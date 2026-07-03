package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestGetScheduleRiskCalendarBuildsBuckets(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "calendar@westwell-lab.com", "Calendar Admin", []string{"demands:read"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")

	now := time.Now()
	overdueDue := now.AddDate(0, 0, -2)
	soonDue := now.AddDate(0, 0, 2)
	staleDue := now.AddDate(0, 0, 12)
	staleUpdate := now.AddDate(0, 0, -5)
	tasks := []db.TaskTelemetry{
		{
			TaskID:        "CAL-101",
			Title:         "逾期需求",
			Assignee:      "Alice",
			CreatorDept:   "Product",
			Branch:        "feature/calendar-late",
			Status:        "progress",
			IssueType:     "demand",
			TaskGroupID:   "calendar-group-1",
			DueDate:       &overdueDue,
			TaskCreatedAt: now.AddDate(0, 0, -8),
			LastUpdate:    now,
		},
		{
			TaskID:        "CAL-102",
			Title:         "临期需求",
			Assignee:      "Bob",
			Branch:        "feature/calendar-soon",
			Status:        "progress",
			IssueType:     "demand",
			TaskGroupID:   "calendar-group-2",
			DueDate:       &soonDue,
			TaskCreatedAt: now.AddDate(0, 0, -3),
			LastUpdate:    now,
		},
		{
			TaskID:        "CAL-103",
			Title:         "待排期需求",
			Assignee:      "Alice",
			Status:        "backlog",
			IssueType:     "demand",
			TaskCreatedAt: now.AddDate(0, 0, -1),
			LastUpdate:    now,
		},
		{
			TaskID:        "CAL-104",
			Title:         "推进滞后需求",
			Assignee:      "Bob",
			Branch:        "feature/calendar-stale",
			Status:        "progress",
			IssueType:     "demand",
			TaskGroupID:   "calendar-group-4",
			DueDate:       &staleDue,
			TaskCreatedAt: now.AddDate(0, 0, -12),
			LastUpdate:    staleUpdate,
		},
	}
	for _, task := range tasks {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed %s: %v", task.TaskID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/schedule/risk-calendar", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response ScheduleRiskCalendarResponseDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Summary.Total != 4 || response.Summary.Overdue != 1 || response.Summary.DueSoon != 1 || response.Summary.Stale != 1 || response.Summary.MissingSchedule != 1 {
		t.Fatalf("unexpected summary: %+v", response.Summary)
	}
	if len(response.Weeks) == 0 || len(response.Months) == 0 {
		t.Fatalf("expected week and month buckets, got weeks=%d months=%d", len(response.Weeks), len(response.Months))
	}
	if response.Events[0].RiskType != "overdue" || response.Events[0].RiskLevel != "critical" {
		t.Fatalf("first event should be overdue critical, got %+v", response.Events[0])
	}
}
