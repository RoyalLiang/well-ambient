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

func TestAutomaticMRVersionDeduplication(t *testing.T) {
	f := newFixture(t)
	no := false
	f.policy(t, db.CodeReviewPolicy{ReviewCommits: &no})
	original := f.s.HTTP.Transport
	sha := head
	f.s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v4/projects/10/merge_requests/7" {
			data, _ := json.Marshal(map[string]any{"iid": 7, "sha": sha, "state": "opened", "diff_refs": map[string]string{"head_sha": sha, "base_sha": base}})
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
		}
		if r.URL.Path == "/api/v4/projects/10/merge_requests" {
			data, _ := json.Marshal([]map[string]any{{"iid": 7, "sha": sha, "updated_at": "2026-10-24T02:49:12Z", "diff_refs": map[string]string{"head_sha": sha, "base_sha": base}}})
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
		}
		return original.RoundTrip(r)
	})
	hook := func(event, body string) {
		t.Helper()
		if err := f.s.Hook(context.Background(), event, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	push := func() { hook("Push Hook", `{"project":{"id":10},"ref":"refs/heads/feature/test","after":"`+sha+`"}`) }
	mr := func(updated string) {
		hook("Merge Request Hook", `{"project":{"id":10},"object_attributes":{"iid":7,"state":"opened","updated_at":"`+updated+`","last_commit":{"id":"`+sha+`"}}}`)
	}
	push()
	mr("2026-10-24T02:49:13Z")
	mr("2026-10-24T02:50:00Z")
	push()
	var count int64
	f.s.DB.Model(&db.CodeReviewRun{}).Count(&count)
	if count != 1 {
		t.Fatalf("same MR/head through push, MR hook and metadata replay created %d reviews; want 1", count)
	}
	sha = strings.Repeat("c", 40)
	mr("2026-10-24T03:00:00Z")
	push()
	f.s.DB.Model(&db.CodeReviewRun{}).Count(&count)
	if count != 2 {
		t.Fatalf("new head must create exactly one new review, got %d", count)
	}
}

func TestAutomaticMRVersionReusesLegacyRun(t *testing.T) {
	f := newFixture(t)
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", head+":old-updated-at")
	if err != nil {
		t.Fatal(err)
	}
	f.s.DB.Model(&run).Updates(map[string]any{"head_sha": head, "base_sha": base, "status": "partial"})
	body := `{"project":{"id":10},"object_attributes":{"iid":7,"state":"opened","updated_at":"new-updated-at","last_commit":{"id":"` + head + `"}}}`
	if err = f.s.Hook(context.Background(), "Merge Request Hook", []byte(body)); err != nil {
		t.Fatal(err)
	}
	var count int64
	f.s.DB.Model(&db.CodeReviewRun{}).Count(&count)
	if count != 1 {
		t.Fatalf("legacy reviewed version was enqueued again: %d", count)
	}
}

func TestAutomaticMRVersionSkipsStaleQueuedHead(t *testing.T) {
	f := newFixture(t)
	old, err := f.s.enqueueMRVersion(context.Background(), "10", "7", mrCodeVersion{Head: strings.Repeat("c", 40), Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.s.DB.First(&old, old.ID)
	if old.Status != "cancelled" || f.calls != 0 {
		t.Fatalf("stale version was reviewed: status=%s calls=%d", old.Status, f.calls)
	}
	var current db.CodeReviewRun
	if err = f.s.DB.Where("status = ? AND head_sha = ?", "queued", head).First(&current).Error; err != nil {
		t.Fatal(err)
	}
}
func TestReviewSearchCollapsesOnlyIdenticalMRVersions(t *testing.T) {
	f := newFixture(t)
	for _, run := range []db.CodeReviewRun{
		{RepositorySourceJSON: `{"repository_key":"source-one"}`, Key: "a", ProjectID: "10", Kind: "mr", Ref: "7", HeadSHA: head, BaseSHA: base, Status: "partial"},
		{RepositorySourceJSON: `{"repository_key":"source-one"}`, Key: "b", ProjectID: "10", Kind: "mr", Ref: "7", HeadSHA: head, BaseSHA: base, Status: "completed"},
		{Key: "c", ProjectID: "10", Kind: "mr", Ref: "8", HeadSHA: head, BaseSHA: base},
		{Key: "d", ProjectID: "10", Kind: "mr", Ref: "7", HeadSHA: head, BaseSHA: strings.Repeat("c", 40)},
		{Key: "e", ProjectID: "10", Kind: "mr", Ref: "7"},
	} {
		if err := f.s.DB.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.s.Search(context.Background(), RunFilter{Query: "MR !7", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 1 || !page.HasMore {
		t.Fatalf("wrong unique-version pagination: %+v", page)
	}
	var history int64
	f.s.DB.Model(&db.CodeReviewRun{}).Count(&history)
	if history != 5 {
		t.Fatal("audit history was removed")
	}
}

func TestAutomaticMRVersionTracksBaseAndRevivesOnlySkippedVersions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := mrCodeVersion{Head: head, Base: base}
	first, err := f.s.enqueueMRVersion(ctx, "10", "7", a)
	if err != nil {
		t.Fatal(err)
	}
	// The same head on a different target baseline is a different reviewed diff.
	b := mrCodeVersion{Head: head, Base: strings.Repeat("d", 40)}
	second, err := f.s.enqueueMRVersion(ctx, "10", "7", b)
	if err != nil || first.ID == second.ID {
		t.Fatalf("retarget diff reused old review: %v", err)
	}
	f.s.DB.Model(&first).Updates(map[string]any{"status": "cancelled", "phase": skippedMRVersionPhase})
	revived, err := f.s.enqueueMRVersion(ctx, "10", "7", a)
	if err != nil || revived.ID != first.ID || revived.Status != "queued" {
		t.Fatalf("skipped A did not recover after B->A: %+v %v", revived, err)
	}
	if err = f.s.Cancel(first.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, err := f.s.enqueueMRVersion(ctx, "10", "7", a)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("user cancellation was overridden: %+v %v", cancelled, err)
	}
}
