package telemetry

import (
	"testing"
	"time"

	"well-ambient/internal/db"
)

func TestProjectScorerUsesAuthoritativeProjectFactsWithoutCreatingPrefixes(t *testing.T) {
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init test database: %v", err)
	}
	if err := db.DB.Create(&db.ProjectConfig{
		ProjectKey:      "HIT",
		ProjectName:     "香港二期",
		BasePriority:    "P1",
		BaseScore:       60,
		BaseScoreWeight: 0.1,
	}).Error; err != nil {
		t.Fatalf("seed project config: %v", err)
	}
	tasks := []db.TaskTelemetry{
		{
			TaskID:     "EXEC-LOCAL",
			ProjectKey: "HIT",
			IssueType:  "task",
			Status:     "progress",
			Assignee:   "Alice",
			LastUpdate: time.Now(),
		},
		{
			TaskID:     "with-5",
			IssueType:  "task",
			Status:     "progress",
			Assignee:   "Bob",
			LastUpdate: time.Now(),
		},
	}
	if err := db.DB.Create(&tasks).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}

	scores, err := CalculateAndSaveScores()
	if err != nil {
		t.Fatalf("calculate scores: %v", err)
	}
	if len(scores) != 1 || scores[0].ProjectKey != "HIT" {
		t.Fatalf("scores must follow persisted project facts, got %+v", scores)
	}

	var configs []db.ProjectConfig
	if err := db.DB.Order("project_key").Find(&configs).Error; err != nil {
		t.Fatalf("query project configs: %v", err)
	}
	if len(configs) != 1 || configs[0].ProjectKey != "HIT" {
		t.Fatalf("scoring must not create project configs from task prefixes, got %+v", configs)
	}
}
