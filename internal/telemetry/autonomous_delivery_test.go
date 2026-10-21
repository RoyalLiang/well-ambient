package telemetry

import (
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/delivery"
)

func TestReconcileAutonomousExecutionMRDeliversAcceptedSuccessfulRun(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	demand := db.TaskTelemetry{TaskID: "DEMAND-MERGE", Title: "Merge", IssueType: "demand", Status: "review", LastUpdate: time.Now()}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	run := db.ExecutionRun{
		RunKey: "merge-run", DemandID: demand.TaskID, TopicBranch: "ai/demand-merge/v1-12345678",
		Status: delivery.RunAcceptancePending, PipelineStatus: "success", AcceptanceState: "accepted",
		MRURL: "https://gitlab.example/mr/8",
	}
	if err := db.DB.Create(&run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}

	reconcileAutonomousExecutionMR(run.TopicBranch, run.MRURL, "merge", "merged")
	if err := db.DB.First(&run, run.ID).Error; err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if run.Status != delivery.RunDelivered || run.MRState != "merged" || run.CompletedAt == nil {
		t.Fatalf("unexpected reconciled run: %+v", run)
	}
	if err := db.DB.First(&demand, "task_id = ?", demand.TaskID).Error; err != nil {
		t.Fatalf("reload demand: %v", err)
	}
	if demand.Status != "done" || demand.CompletedAt == nil {
		t.Fatalf("demand should be delivered: %+v", demand)
	}
}

func TestReconcileAutonomousExecutionMRRejectsChangedOrigin(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatal(err)
	}
	original := config.RepositorySource{InstanceID: "fde", ProjectID: "fde:10", RepositoryKey: "gitlab:https://original.example.test:10"}
	run := db.ExecutionRun{RunKey: "frozen-origin", ProjectRef: "fde:10", DemandID: "FDE-10", TopicBranch: "ai/frozen", Status: delivery.RunAcceptancePending, PipelineStatus: "success", AcceptanceState: "accepted", RepositorySourceJSON: original.JSON()}
	if err := db.DB.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	changed := original
	changed.RepositoryKey = "gitlab:https://changed.example.test:10"
	reconcileAutonomousExecutionMR(run.TopicBranch, "", "merge", "merged", changed)
	if err := db.DB.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if run.MRState == "merged" || run.Status == delivery.RunDelivered {
		t.Fatal("another origin updated frozen execution")
	}
	reconcileAutonomousExecutionMR(run.TopicBranch, "", "merge", "merged", original)
	db.DB.First(&run, run.ID)
	if run.MRState != "merged" {
		t.Fatal("matching origin did not reconcile")
	}
}
