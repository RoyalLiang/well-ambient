package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func TestKPIPerformanceIncludesProcessRiskFields(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "kpi-admin@westwell-lab.com", "KPI Admin", []string{"kpi:read"})
	seedKPIReportPreviewData(t)

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9110, Host: "127.0.0.1"}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/kpi/performance?period=week", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/kpi/performance failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var res KPIPerformanceResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode KPI performance response: %v", err)
	}

	bob := findUserKPI(t, res.UserKPI, "Bob")
	if bob.TotalCompleted != 1 || bob.DemandsCompleted != 1 {
		t.Fatalf("Bob completion fields mismatch: %+v", bob)
	}
	if bob.OverdueCompleted != 1 {
		t.Fatalf("Bob OverdueCompleted = %d, want 1", bob.OverdueCompleted)
	}
	if bob.ActiveOverdue != 1 {
		t.Fatalf("Bob ActiveOverdue = %d, want 1", bob.ActiveOverdue)
	}
	if bob.ReviewCount != 1 {
		t.Fatalf("Bob ReviewCount = %d, want 1", bob.ReviewCount)
	}
	if bob.MRCount != 2 {
		t.Fatalf("Bob MRCount = %d, want 2", bob.MRCount)
	}
	if bob.AvgCycleDays != 3.5 {
		t.Fatalf("Bob AvgCycleDays = %.1f, want 3.5", bob.AvgCycleDays)
	}
	if len(bob.RiskNotes) < 3 {
		t.Fatalf("Bob RiskNotes should include overdue and review notes: %+v", bob.RiskNotes)
	}
}

func TestKPIReportPreviewReturnsEvidenceBackedSections(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "kpi-admin@westwell-lab.com", "KPI Admin", []string{"kpi:read"})
	seedKPIReportPreviewData(t)

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9111, Host: "127.0.0.1"}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/kpi/report-preview?period=week&type=weekly&user=Bob", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/kpi/report-preview failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var res KPIReportPreviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode report preview response: %v", err)
	}

	if res.Period != "week" || res.Type != "weekly" || res.User != "Bob" {
		t.Fatalf("Report metadata mismatch: %+v", res)
	}
	if res.Overview.TotalCompleted != 1 || res.Overview.ActiveOverdue != 1 || res.Overview.OverdueCompleted != 1 || res.Overview.MRCount != 2 {
		t.Fatalf("Report overview mismatch: %+v", res.Overview)
	}
	if len(res.PersonalSections) != 1 || res.PersonalSections[0].User.Name != "Bob" {
		t.Fatalf("Expected one Bob personal section, got %+v", res.PersonalSections)
	}
	if !containsAll(res.PersonalSections[0].EvidenceIDs, "TASK-KPI-DONE", "TASK-KPI-OVERDUE", "TASK-KPI-REVIEW") {
		t.Fatalf("Personal evidence IDs mismatch: %+v", res.PersonalSections[0].EvidenceIDs)
	}
	if len(res.DepartmentSections) == 0 || res.DepartmentSections[0].Department != "AI Lab" {
		t.Fatalf("Expected AI Lab department section, got %+v", res.DepartmentSections)
	}
	if !reportRisksContain(res.Risks, "TASK-KPI-OVERDUE") {
		t.Fatalf("Expected active overdue risk for TASK-KPI-OVERDUE, got %+v", res.Risks)
	}
	if !meetingFocusContain(res.MeetingFocus, "TASK-KPI-OVERDUE") || !meetingFocusContain(res.MeetingFocus, "TASK-KPI-REVIEW") {
		t.Fatalf("Expected overdue and review meeting focus items, got %+v", res.MeetingFocus)
	}
	if !reportEvidenceContain(res.Evidence, "TASK-KPI-DONE") || !reportEvidenceContain(res.Evidence, "TASK-KPI-OVERDUE") || !reportEvidenceContain(res.Evidence, "TASK-KPI-REVIEW") {
		t.Fatalf("Expected report evidence to include Bob tasks, got %+v", res.Evidence)
	}
}

func TestKPIFiltersNonCoreMemberData(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "kpi-admin@westwell-lab.com", "KPI Admin", []string{"kpi:read"})
	seedKPIReportPreviewData(t)

	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9113, Host: "127.0.0.1"},
		Jira: config.JiraConfig{
			SyncUsers: []string{"Bob"},
		},
	}, "")

	req := httptest.NewRequest(http.MethodGet, "/api/kpi/performance?period=week", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/kpi/performance failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var performance KPIPerformanceResponse
	if err := json.NewDecoder(rr.Body).Decode(&performance); err != nil {
		t.Fatalf("Failed to decode KPI performance response: %v", err)
	}

	if performance.Summary.TotalCompleted != 1 || performance.Summary.TasksCompleted != 0 || performance.Summary.DemandsCompleted != 1 {
		t.Fatalf("KPI summary should only include core member work, got %+v", performance.Summary)
	}
	if len(performance.UserKPI) != 1 || performance.UserKPI[0].Name != "Bob" {
		t.Fatalf("Expected only Bob in KPI users, got %+v", performance.UserKPI)
	}
	for _, dept := range performance.DepartmentKPI {
		if dept.Department != "AI Lab" {
			t.Fatalf("Unexpected non-core department in KPI response: %+v", performance.DepartmentKPI)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/kpi/report-preview?period=week&type=weekly", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/kpi/report-preview failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var preview KPIReportPreviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&preview); err != nil {
		t.Fatalf("Failed to decode KPI report preview response: %v", err)
	}
	if preview.Overview.TotalCompleted != 1 || preview.Overview.PersonalCount != 1 {
		t.Fatalf("Report overview should only include core member data, got %+v", preview.Overview)
	}
	if reportEvidenceContain(preview.Evidence, "TASK-KPI-OTHER") {
		t.Fatalf("Report evidence leaked non-core member task: %+v", preview.Evidence)
	}
}

func TestKPIReportPreviewRejectsUnknownType(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "kpi-admin@westwell-lab.com", "KPI Admin", []string{"kpi:read"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9112, Host: "127.0.0.1"}}, "")

	req := httptest.NewRequest(http.MethodGet, "/api/kpi/report-preview?type=monthly", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Invalid report preview type status = %v, want %v body %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func seedKPIReportPreviewData(t *testing.T) {
	t.Helper()
	now := time.Now()
	bob := userdb.User{
		Username:   "bob@westwell-lab.com",
		Email:      "bob@westwell-lab.com",
		Name:       "Bob",
		Department: "AI Lab",
	}
	if err := db.DB.Create(&bob).Error; err != nil {
		t.Fatalf("Failed to seed Bob: %v", err)
	}

	completedAt := now.Add(-12 * time.Hour)
	completedDue := now.Add(-48 * time.Hour)
	activeDue := now.Add(-24 * time.Hour)
	reviewDue := now.Add(24 * time.Hour)

	tasks := []db.TaskTelemetry{
		{
			TaskID:        "TASK-KPI-DONE",
			Title:         "Deliver reporting digest",
			Repo:          "well-ambient",
			Assignee:      "Bob",
			Status:        "done",
			IssueType:     "demand",
			TaskCreatedAt: now.Add(-4 * 24 * time.Hour),
			LastUpdate:    completedAt,
			CompletedAt:   &completedAt,
			DueDate:       &completedDue,
			MrURL:         "https://gitlab.example.com/well-ambient/-/merge_requests/88",
			TaskGroupID:   "group-kpi-1",
		},
		{
			TaskID:        "TASK-KPI-OVERDUE",
			Title:         "Backfill overdue task owner",
			Repo:          "well-ambient",
			Assignee:      "Bob",
			Status:        "progress",
			IssueType:     "task",
			TaskCreatedAt: now.Add(-3 * 24 * time.Hour),
			LastUpdate:    now.Add(-2 * time.Hour),
			DueDate:       &activeDue,
		},
		{
			TaskID:        "TASK-KPI-REVIEW",
			Title:         "Review KPI evidence renderer",
			Repo:          "well-ambient",
			Assignee:      "Bob",
			Status:        "review",
			IssueType:     "task",
			TaskCreatedAt: now.Add(-2 * 24 * time.Hour),
			LastUpdate:    now.Add(-1 * time.Hour),
			DueDate:       &reviewDue,
			MrIID:         89,
		},
		{
			TaskID:        "TASK-KPI-OTHER",
			Title:         "Other user task",
			Repo:          "well-ambient",
			Assignee:      "Alice",
			Status:        "done",
			IssueType:     "task",
			TaskCreatedAt: now.Add(-2 * 24 * time.Hour),
			LastUpdate:    completedAt,
			CompletedAt:   &completedAt,
		},
	}
	if err := db.DB.Create(&tasks).Error; err != nil {
		t.Fatalf("Failed to seed KPI tasks: %v", err)
	}
}

func findUserKPI(t *testing.T, users []UserKPIDTO, name string) UserKPIDTO {
	t.Helper()
	for _, user := range users {
		if user.Name == name {
			return user
		}
	}
	t.Fatalf("User %q not found in KPI response: %+v", name, users)
	return UserKPIDTO{}
}

func containsAll(items []string, values ...string) bool {
	itemSet := make(map[string]bool, len(items))
	for _, item := range items {
		itemSet[item] = true
	}
	for _, value := range values {
		if !itemSet[value] {
			return false
		}
	}
	return true
}

func reportRisksContain(risks []KPIReportRiskDTO, evidenceID string) bool {
	for _, risk := range risks {
		if containsAll(risk.EvidenceIDs, evidenceID) {
			return true
		}
	}
	return false
}

func meetingFocusContain(focus []KPIReportMeetingFocusDTO, evidenceID string) bool {
	for _, item := range focus {
		if containsAll(item.EvidenceIDs, evidenceID) {
			return true
		}
	}
	return false
}

func reportEvidenceContain(evidence []KPIReportEvidenceDTO, taskID string) bool {
	for _, item := range evidence {
		if strings.EqualFold(item.TaskID, taskID) {
			return true
		}
	}
	return false
}
