package db

import "testing"

func TestInitDBCreatesTaskTrackingReadIndexes(t *testing.T) {
	if err := InitDB(":memory:"); err != nil {
		t.Fatalf("init database: %v", err)
	}
	for _, indexName := range []string{
		"idx_task_tracking_kind_status_project_owner",
		"idx_task_tracking_parent_group",
		"idx_git_evidence_task_created",
	} {
		var count int64
		if err := DB.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?",
			indexName,
		).Scan(&count).Error; err != nil {
			t.Fatalf("query index %s: %v", indexName, err)
		}
		if count != 1 {
			t.Fatalf("expected task-tracking index %s", indexName)
		}
	}
}
