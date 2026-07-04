package telemetry

import (
	"testing"
	"well-ambient/internal/db"
)

func TestSemanticLinkTrustReasonRejectsWeakProjectMismatch(t *testing.T) {
	oldDB := db.DB
	t.Cleanup(func() { db.DB = oldDB })
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	task := db.TaskTelemetry{
		TaskID:   "NS2-1692",
		Title:    "南沙二期配置中心点位",
		Repo:     "PRJ25151-南沙二期码头Q-Chassis运营20套 (NS2)",
		Branch:   "-",
		Assignee: "梁志远",
		Status:   "progress",
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	ok, reason := semanticLinkTrustReason("NS2-1692", "baiyun_dev", "task_executor", "haoliang.jiang")
	if ok {
		t.Fatalf("expected weak semantic link to be rejected")
	}
	if reason == "" {
		t.Fatalf("expected rejection reason")
	}
}

func TestSemanticLinkTrustReasonAcceptsStrongEvidence(t *testing.T) {
	oldDB := db.DB
	t.Cleanup(func() { db.DB = oldDB })
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	task := db.TaskTelemetry{
		TaskID:   "TASK-777",
		Title:    "Backend linked task",
		Repo:     "backend-core",
		Branch:   "feature/linked-work",
		Assignee: "Eddie",
		Status:   "progress",
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	ok, reason := semanticLinkTrustReason("TASK-777", "feature/linked-work", "backend-core", "Eddie")
	if !ok {
		t.Fatalf("expected strong semantic link to be accepted, reason=%s", reason)
	}
}
