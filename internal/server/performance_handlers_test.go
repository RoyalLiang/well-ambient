package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func TestGetConfigReportsTheAuditedPerformanceContract(t *testing.T) {
	srv := &Server{config: &config.Config{PerformanceBrain: config.PerformanceBrainConfig{
		Enabled: true, FormulaVersion: "v4.0", EvidenceCoverageGate: 0.95,
	}}}
	request := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	response := httptest.NewRecorder()
	srv.handleGetConfig(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("get config = %d: %s", response.Code, response.Body.String())
	}
	var payload config.Config
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	performance := payload.PerformanceBrain
	if !performance.Enabled || performance.FormulaVersion != "v6.0" || performance.PublicationMode != "shadow" || performance.EvidenceCoverageGate != 0.70 || performance.MinimumSamples != 5 || performance.MinimumExposureDays != 30 ||
		!*performance.DemandMetricsEnabled || !*performance.BugMetricsEnabled || !*performance.CodeMetricsEnabled || !*performance.JiraHistoryEnabled || !*performance.GitDedupeEnabled {
		t.Fatalf("GET /api/config exposed stale or mutable performance gates: %+v", performance)
	}
}

func TestPerformanceExplanationAndEvidenceRequireGlobalSuperAdmin(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{}, "")
	superToken := superAdminToken(t, "performance-owner@example.com", "Performance Owner", []string{"kpi:read"})
	memberToken := performanceMemberToken(t, "performance-reader@example.com", []string{"kpi:read"})

	for _, endpoint := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/performance/explanation"},
		{method: http.MethodGet, path: "/api/performance/snapshots/1"},
		{method: http.MethodPost, path: "/api/performance/evidence", body: `{}`},
	} {
		request := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))
		request.Header.Set("Authorization", "Bearer "+memberToken)
		response := httptest.NewRecorder()
		srv.mux.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s %s for member = %d, want 403", endpoint.method, endpoint.path, response.Code)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/performance/explanation", nil)
	request.Header.Set("Authorization", "Bearer "+superToken)
	response := httptest.NewRecorder()
	srv.mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("superadmin explanation = %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		ReadOnly bool `json:"read_only"`
		Metrics  []struct {
			Code      string  `json:"code"`
			Dimension string  `json:"dimension"`
			Weight    float64 `json:"weight"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.ReadOnly || len(payload.Metrics) != 3 ||
		payload.Metrics[0].Code != "D01" || payload.Metrics[0].Dimension != "交付结果" || payload.Metrics[0].Weight != 0.35 ||
		payload.Metrics[1].Code != "D02" || payload.Metrics[1].Dimension != "交付可预测性" || payload.Metrics[1].Weight != 0.20 ||
		payload.Metrics[2].Code != "B01" || payload.Metrics[2].Dimension != "工程质量" || payload.Metrics[2].Weight != 0.45 {
		t.Fatalf("unexpected explanation payload: %+v", payload)
	}
}

func TestPerformanceSnapshotDetailReturnsPersistedReadOnlyData(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{
		Jira:             config.JiraConfig{SyncUsers: []string{"Alice"}},
		PerformanceBrain: config.PerformanceBrainConfig{Enabled: true},
	}, "")
	now := time.Now().UTC()
	completedAt := now.Add(-time.Hour)
	dueAt := now
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "WA-DETAIL", Assignee: "Alice", IssueType: "requirement", Status: "done",
		TaskCreatedAt: now.Add(-24 * time.Hour), LastUpdate: now,
		CompletedAt: &completedAt, DueDate: &dueAt, EstimateDays: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	run, err := srv.performance.RunOnce(context.Background(), "run_once")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot db.PerformanceScoreSnapshot
	if err := db.DB.Where("run_id = ?", run.RunID).First(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	token := superAdminToken(t, "performance-detail@example.com", "Performance Detail", nil)
	request := httptest.NewRequest(http.MethodGet, "/api/performance/snapshots/"+strconv.FormatUint(uint64(snapshot.ID), 10), nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	srv.mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("snapshot detail = %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		ReadOnly bool `json:"read_only"`
		Snapshot struct {
			ID         uint   `json:"id"`
			SubjectKey string `json:"subject_key"`
		} `json:"snapshot"`
		ItemFactors []any `json:"item_factors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.ReadOnly || payload.Snapshot.ID != snapshot.ID || payload.Snapshot.SubjectKey != "Alice" || len(payload.ItemFactors) != 1 {
		t.Fatalf("unexpected snapshot detail: %+v", payload)
	}
}

func TestPerformanceEvidenceEndpointAppendsAuditButDoesNotTriggerCalculation(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{}, "")
	token := superAdminToken(t, "performance-auditor@example.com", "Performance Auditor", nil)
	body := `{
		"evidence_key":"jira:WA-401:rollback",
		"event_type":"change_rollback_outcome",
		"subject_key":"Alice",
		"work_item_id":"WA-401",
		"project_key":"WA",
		"release_method":"全量发布",
		"outcome":1,
		"occurred_at":"2026-08-12T08:00:00Z",
		"source_system":"jira",
		"source_record_id":"WA-401:deploy-7",
		"payload":{"reviewed":true}
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/performance/evidence", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	srv.mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("append evidence = %d: %s", response.Code, response.Body.String())
	}
	var fact db.PerformanceEvidenceFact
	if err := db.DB.Where("evidence_key = ?", "jira:WA-401:rollback").First(&fact).Error; err != nil {
		t.Fatal(err)
	}
	if fact.CreatedBy != "Performance Auditor" || fact.Revision != 1 || fact.Action != "observe" {
		t.Fatalf("unexpected evidence fact: %+v", fact)
	}
	var runCount int64
	if err := db.DB.Model(&db.PerformanceScoreRun{}).Count(&runCount).Error; err != nil {
		t.Fatal(err)
	}
	if runCount != 0 {
		t.Fatalf("evidence API triggered %d calculation runs", runCount)
	}
	var auditCount int64
	if err := db.DB.Model(&db.PerformanceAuditEvent{}).Where("event_type = ?", "evidence_recorded").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("evidence audit count = %d, want 1", auditCount)
	}
}

func performanceMemberToken(t *testing.T, username string, permissions []string) string {
	t.Helper()
	group := userdb.UserGroup{Name: "performance_member_" + strings.ReplaceAll(username, "@", "_"), DisplayName: "Performance Member"}
	if err := db.DB.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	user := userdb.User{Username: username, Email: username, Name: "Performance Reader"}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&userdb.UserGroupMembership{UserID: user.ID, UserGroupID: group.ID, Scope: "global", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	for _, code := range permissions {
		var permission userdb.Permission
		if err := db.DB.Where("code = ?", code).First(&permission).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.DB.Create(&userdb.GroupPermission{UserGroupID: group.ID, PermissionID: permission.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	token, err := GenerateJWT(username, "Performance Reader", "mock_wellos_token", "", []string{group.Name}, permissions)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
