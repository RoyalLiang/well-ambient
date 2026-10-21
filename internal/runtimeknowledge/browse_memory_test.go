package runtimeknowledge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

func TestBrowseRevalidatesSnapshotSourcesAndCursor(t *testing.T) {
	s, conn, _ := fixture(t)
	ctx := context.Background()
	doc := db.ContextDocument{Title: "车辆资源锁", Content: "锁边界必须核对资源和版本", Scope: "global", Status: "active", IngestionStatus: "completed", Version: 1}
	create(t, conn, &doc)
	if _, err := s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	list, err := s.Browse(ctx, BrowseRequest{Kind: "context_document", Query: "车辆"})
	if err != nil || len(list.Items) != 1 || list.Items[0].Content == "" {
		t.Fatalf("browse: %+v %v", list, err)
	}
	if err := conn.Model(&doc).Update("status", "archived").Error; err != nil {
		t.Fatal(err)
	}
	detail, err := s.Browse(ctx, BrowseRequest{SnapshotID: list.SnapshotID, SourceRef: list.Items[0].SourceRef})
	if err != nil || len(detail.Items) != 1 || detail.Items[0].Content != "" || detail.Items[0].SourceState == "current" {
		t.Fatalf("withdrawn body exposed: %+v %v", detail, err)
	}
	if _, err = s.Browse(ctx, BrowseRequest{SnapshotID: 999}); openaccess.ErrorCode(err) != "conflict" {
		t.Fatalf("expired snapshot: %v", err)
	}
}

func TestDenseMemoryReceiptRevalidatesPermissionsAndChanges(t *testing.T) {
	s, conn, policy := fixture(t)
	ctx := context.Background()
	doc := db.ContextDocument{Title: "资源锁", Content: strings.Repeat("车辆资源版本与fencing证据 ", 120), Scope: "global", Status: "active", IngestionStatus: "completed", Version: 1}
	create(t, conn, &doc)
	if _, err := s.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	req := ContextRequest{Text: "资源锁", Format: "dense", TokenBudget: 4000}
	first, err := s.Context(ctx, policy, req)
	if err != nil || len(first.Evidence) == 0 || !strings.Contains(first.Compact, "@row [ref,scope,claim,title,content]") {
		t.Fatalf("dense: %+v %v", first, err)
	}
	req.KnownMemoryHash = first.MemoryHash
	again, err := s.Context(ctx, policy, req)
	if err != nil || !again.Unchanged || again.Compact != "" || len(again.AuditRefs) != len(first.Evidence) {
		t.Fatalf("reuse: %+v %v", again, err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(again)
	if len(b) >= len(a) {
		t.Fatal("receipt did not reduce wire bytes")
	}
	t.Logf("synthetic response bytes %d -> %d", len(a), len(b))
	if err := conn.Model(&doc).Update("content", "内容已变化").Error; err != nil {
		t.Fatal(err)
	}
	changed, err := s.Context(ctx, policy, req)
	if err != nil || changed.Unchanged || len(changed.Evidence) != 0 {
		t.Fatalf("changed source reused: %+v %v", changed, err)
	}
	policy.Actions = nil
	if _, err := s.Context(ctx, policy, req); openaccess.ErrorCode(err) != "forbidden" {
		t.Fatalf("revoked policy reused: %v", err)
	}
}
