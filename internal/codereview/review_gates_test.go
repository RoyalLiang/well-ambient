package codereview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"well-ambient/internal/db"
)

func TestIndependentReviewGatesOnPushAndMR(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		mr, commit, repoMR, repoCommit bool
		wantMR, wantCommit             int
	}{
		{"both", true, true, true, true, 1, 2}, {"MR only", true, false, true, true, 1, 0}, {"commit only", false, true, true, true, 0, 2}, {"off", false, false, true, true, 0, 0}, {"repository MR gate", true, true, false, true, 0, 2}, {"repository commit gate", true, true, true, false, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			f.policy(t, db.CodeReviewPolicy{ReviewMRs: &tc.repoMR, ReviewCommits: &tc.repoCommit})
			if _, err := f.s.PatchCommentSettings(ctx, nil, nil, &tc.commit, &tc.mr); err != nil {
				t.Fatal(err)
			}
			head := strings.Repeat("a", 40)
			f.s.HTTP = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				body := `[]`
				version := `{"iid":42,"updated_at":"stamp","diff_refs":{"head_sha":"` + head + `","base_sha":"` + base + `"}}`
				if strings.HasSuffix(r.URL.Path, "/merge_requests") {
					body = "[" + version + "]"
				}
				if strings.HasSuffix(r.URL.Path, "/merge_requests/42") {
					body = version
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			push := []byte(`{"project":{"id":10},"ref":"refs/heads/feature","after":"` + head + `","commits":[{"id":"` + strings.Repeat("1", 40) + `"},{"id":"` + strings.Repeat("2", 40) + `"}]}`)
			if err := f.s.Hook(ctx, "Push Hook", push); err != nil {
				t.Fatal(err)
			}
			mr := []byte(`{"project":{"id":10},"object_attributes":{"iid":42,"state":"opened","updated_at":"stamp","last_commit":{"id":"` + head + `"}}}`)
			if err := f.s.Hook(ctx, "Merge Request Hook", mr); err != nil {
				t.Fatal(err)
			}
			var runs []db.CodeReviewRun
			if err := f.s.DB.Find(&runs).Error; err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			for _, run := range runs {
				counts[run.Kind]++
			}
			if counts["mr"] != tc.wantMR || counts["commit"] != tc.wantCommit {
				t.Fatalf("types=%v, want MR %d commit %d", counts, tc.wantMR, tc.wantCommit)
			}
			if !tc.mr || !tc.repoMR {
				if _, err := f.s.Enqueue(ctx, "10", "mr", "9", ""); err == nil {
					t.Fatal("manual enqueue bypassed MR gate")
				}
			}
			if !tc.commit || !tc.repoCommit {
				if _, err := f.s.Enqueue(ctx, "10", "commit", strings.Repeat("b", 40), ""); err == nil {
					t.Fatal("manual enqueue bypassed commit gate")
				}
			}
		})
	}
}
func TestReviewGatePartialPatchPreservesOtherGates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	off, on := false, true
	settings, err := f.s.PatchCommentSettings(ctx, &off, &on, &on, &off)
	if err != nil {
		t.Fatal(err)
	}
	settings, err = f.s.PatchCommentSettings(ctx, nil, nil, &off, nil)
	if err != nil {
		t.Fatal(err)
	}
	if settings.ReviewMRs || settings.ReviewCommits || settings.SyncCommits || !settings.SyncMRs {
		raw, _ := json.Marshal(settings)
		t.Fatal(string(raw))
	}
}

func TestDisabledReviewCancelsQueuedBeforeAI(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	run, err := f.s.Enqueue(ctx, "10", "commit", head, "")
	if err != nil {
		t.Fatal(err)
	}
	off := false
	if _, err = f.s.PatchCommentSettings(ctx, nil, nil, &off, nil); err != nil {
		t.Fatal(err)
	}
	called := false
	f.s.Generate = func(context.Context, string, string) (string, error) { called = true; return "", nil }
	worked, err := f.s.ProcessOne(ctx)
	if err != nil || !worked {
		t.Fatal(worked, err)
	}
	got, err := f.s.Get(run.ID)
	if err != nil || got.Status != "cancelled" || called {
		t.Fatalf("disabled queued review executed: %s AI=%v err=%v", got.Status, called, err)
	}
}
func TestMRDiscoveryFailureIsNotAcknowledgedAsSuccess(t *testing.T) {
	f := newFixture(t)
	f.s.HTTP = &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
	})}
	if err := f.s.Hook(context.Background(), "Push Hook", []byte(`{"project":{"id":10},"ref":"refs/heads/feature","commits":[]}`)); err == nil {
		t.Fatal("MR discovery failure was silently acknowledged")
	}
}

func TestReviewKindFilterPrecedesResultLimit(t *testing.T) {
	f := newFixture(t)
	for i, kind := range []string{"commit", "mr", "mr"} {
		run := db.CodeReviewRun{Key: "kind-filter-" + string(rune('a'+i)), ProjectID: "10", Kind: kind}
		if err := f.s.DB.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	runs, err := f.s.List(1, "commit")
	if err != nil || len(runs) != 1 || runs[0].Kind != "commit" {
		t.Fatalf("old commit hidden behind MR limit: %+v %v", runs, err)
	}
}
