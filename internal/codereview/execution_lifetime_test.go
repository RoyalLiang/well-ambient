package codereview

import (
	"context"
	"errors"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestReviewExecutionContextKeepsCallerLifetime(t *testing.T) {
	for _, bounded := range []bool{false, true} {
		name := "no-parent-deadline"
		if bounded {
			name = "inherit-parent-deadline"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			parent := context.Background()
			if bounded {
				var cancel context.CancelFunc
				parent, cancel = context.WithTimeout(parent, time.Hour)
				defer cancel()
			}
			original := f.s.Generate
			calls := 0
			f.s.Generate = func(ctx context.Context, system, user string) (string, error) {
				got, has := ctx.Deadline()
				want, wants := parent.Deadline()
				if has != wants || (has && !got.Equal(want)) {
					t.Errorf("review changed caller deadline: got %v/%v want %v/%v", got, has, want, wants)
				}
				calls++
				return original(ctx, system, user)
			}
			if _, err := f.s.Enqueue(parent, "10", "mr", "7", ""); err != nil {
				t.Fatal(err)
			}
			if _, err := f.s.ProcessOne(parent); err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("review passes=%d, want 2", calls)
			}
		})
	}
}

func TestReviewParentCancellationAndDeadlineStopGeneration(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "cancel"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			var ctx context.Context
			var cancel context.CancelFunc
			if deadline {
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()
			run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "")
			if err != nil {
				t.Fatal(err)
			}
			entered := false
			f.s.Generate = func(call context.Context, _, _ string) (string, error) {
				entered = true
				if !deadline {
					cancel()
				}
				<-call.Done()
				return "", call.Err()
			}
			_, err = f.s.ProcessOne(ctx)
			want := context.Canceled
			if deadline {
				want = context.DeadlineExceeded
			}
			if !entered || !errors.Is(err, want) {
				t.Fatalf("generation did not retain caller cancellation: entered=%v err=%v", entered, err)
			}
			saved, err := f.s.Get(run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.ReportJSON != "" || saved.Status != "failed" {
				t.Fatalf("interrupted generation produced result: %+v", saved)
			}
		})
	}
}

func waitForExecutionState(t *testing.T, s *Service, id uint, condition func(db.CodeReviewRun) bool) db.CodeReviewRun {
	t.Helper()
	limit := time.NewTimer(2 * time.Second)
	defer limit.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		row, err := s.Get(id)
		if err == nil && condition(row) {
			return row
		}
		select {
		case <-limit.C:
			t.Fatalf("run %d did not reach expected state", id)
		case <-tick.C:
		}
	}
}

func TestReviewHeartbeatProtectsLiveRunAndStillRecoversOrphans(t *testing.T) {
	f := newFixture(t)
	f.s.reviewPollInterval = time.Millisecond
	f.s.reviewHeartbeatInterval = 5 * time.Millisecond
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "")
	if err != nil {
		t.Fatal(err)
	}
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	original := f.s.Generate
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
			t.Error("review did not stop")
		}
	})
	f.s.Generate = func(call context.Context, system, user string) (string, error) {
		calls++
		if calls == 1 {
			close(entered)
			select {
			case <-release:
			case <-call.Done():
				return "", call.Err()
			}
		}
		return original(call, system, user)
	}
	go func() { defer close(finished); _, err := f.s.ProcessOne(ctx); result <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("model not entered")
	}
	old := time.Now().Add(-16 * time.Minute)
	if err := f.s.DB.Model(&db.CodeReviewRun{}).Where("id = ?", run.ID).UpdateColumn("updated_at", old).Error; err != nil {
		t.Fatal(err)
	}
	waitForExecutionState(t, f.s, run.ID, func(row db.CodeReviewRun) bool {
		return row.Status == "running" && row.UpdatedAt.After(old.Add(time.Minute))
	})
	orphan := db.CodeReviewRun{Key: "lifetime-orphan", ProjectID: "10", Status: "running", UpdatedAt: old}
	if err := f.s.DB.Create(&orphan).Error; err != nil {
		t.Fatal(err)
	}
	if processed, err := f.s.ProcessOne(context.Background()); processed || err != nil {
		t.Fatalf("unexpected second work: %v %v", processed, err)
	}
	live, err := f.s.Get(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	dead, err := f.s.Get(orphan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live.Status != "running" || dead.Status != "failed" {
		t.Fatalf("live=%s orphan=%s", live.Status, dead.Status)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	live, err = f.s.Get(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live.Status != "completed" {
		t.Fatalf("live run did not complete: %s", live.Status)
	}
}

func TestReviewMonitorStopsGenerationAfterLosingRunningState(t *testing.T) {
	f := newFixture(t)
	f.s.reviewPollInterval = time.Millisecond
	f.s.reviewHeartbeatInterval = time.Millisecond
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "")
	if err != nil {
		t.Fatal(err)
	}
	f.s.Generate = func(ctx context.Context, _, _ string) (string, error) {
		if err := f.s.DB.Model(&db.CodeReviewRun{}).Where("id = ?", run.ID).Updates(map[string]any{"status": "failed", "phase": "recovered elsewhere"}).Error; err != nil {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
			return "", errors.New("monitor did not cancel generation")
		}
	}
	if _, err := f.s.ProcessOne(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost state did not cancel generation: %v", err)
	}
	saved, err := f.s.Get(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "failed" || saved.Phase != "recovered elsewhere" || saved.ReportJSON != "" {
		t.Fatal("monitor overwrote terminal run")
	}
}
