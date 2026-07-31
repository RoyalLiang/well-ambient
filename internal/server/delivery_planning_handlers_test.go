package server

import (
	"bytes"
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
	"well-ambient/internal/deliveryplanning"
)

func TestVersionPlanListsExistingReleaseAndSupportsBulkJiraIssueAssociation(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "release-owner@example.com", "Release Owner", []string{"delivery:read", "release:manage"})
	if err := db.DB.Create(&db.ProjectConfig{ProjectKey: "HIT", ProjectName: "香港二期"}).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	release := db.ReleaseVersion{
		ProjectKey: "HIT",
		Source:     "local",
		ExternalID: "local-20",
		Name:       "Harbor 2.0",
		Status:     deliveryplanning.ReleasePlanned,
	}
	if err := db.DB.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	issues := []db.TaskTelemetry{
		{
			TaskID:        "HIT-101",
			ExternalKey:   "HIT-101",
			Source:        "jira",
			ProjectKey:    "HIT",
			IssueType:     "requirement",
			Title:         "Prepare crane telemetry",
			Status:        "backlog",
			PlanningState: deliveryplanning.PlanningReady,
		},
		{
			TaskID:        "HIT-102",
			ExternalKey:   "HIT-102",
			Source:        "jira",
			ProjectKey:    "HIT",
			IssueType:     "bug",
			Title:         "Repair release dashboard",
			Status:        "progress",
			PlanningState: deliveryplanning.PlanningReady,
		},
	}
	if err := db.DB.Create(&issues).Error; err != nil {
		t.Fatalf("create Jira issues: %v", err)
	}

	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled: true,
		BaseURL: "https://jira.example.com",
	}}, "")

	doRequest := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, request)
		return recorder
	}

	listRecorder := doRequest(http.MethodGet, "/api/releases?q=harbor", "")
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("release list status = %d, body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload struct {
		Items []struct {
			Release        db.ReleaseVersion `json:"release"`
			JiraIssueCount int64             `json:"jira_issue_count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode release list: %v", err)
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].Release.ID != release.ID || listPayload.Items[0].JiraIssueCount != 0 {
		t.Fatalf("unexpected release list: %#v", listPayload.Items)
	}

	candidateRecorder := doRequest(
		http.MethodGet,
		"/api/releases/"+jsonNumber(release.ID)+"/jira-issues?scope=candidates&q=telemetry",
		"",
	)
	if candidateRecorder.Code != http.StatusOK {
		t.Fatalf("Jira issue candidate status = %d, body=%s", candidateRecorder.Code, candidateRecorder.Body.String())
	}
	var candidatePayload struct {
		Items []struct {
			WorkItemID string `json:"work_item_id"`
			JiraKey    string `json:"jira_key"`
			JiraURL    string `json:"jira_url"`
			Linked     bool   `json:"linked"`
		} `json:"items"`
	}
	if err := json.Unmarshal(candidateRecorder.Body.Bytes(), &candidatePayload); err != nil {
		t.Fatalf("decode Jira candidates: %v", err)
	}
	if len(candidatePayload.Items) != 1 ||
		candidatePayload.Items[0].WorkItemID != "HIT-101" ||
		candidatePayload.Items[0].JiraKey != "HIT-101" ||
		candidatePayload.Items[0].JiraURL != "https://jira.example.com/browse/HIT-101" ||
		candidatePayload.Items[0].Linked {
		t.Fatalf("unexpected Jira candidates: %#v", candidatePayload.Items)
	}
	bulkRecorder := doRequest(
		http.MethodPost,
		"/api/releases/"+jsonNumber(release.ID)+"/jira-issues/bulk",
		`{"work_item_ids":["HIT-101","HIT-102"],"reason":"纳入 Harbor 2.0 发布范围"}`,
	)
	if bulkRecorder.Code != http.StatusOK {
		t.Fatalf("bulk Jira association status = %d, body=%s", bulkRecorder.Code, bulkRecorder.Body.String())
	}
	var linkCount, outboxCount int64
	if err := db.DB.Model(&db.WorkItemReleaseLink{}).
		Where("release_version_id = ? AND relation = ? AND active = ? AND is_primary = ?",
			release.ID, deliveryplanning.ReleaseTargetFix, true, true).
		Count(&linkCount).Error; err != nil {
		t.Fatalf("count Jira associations: %v", err)
	}
	if err := db.DB.Model(&db.WorkItemSyncOperation{}).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count Jira version outbox: %v", err)
	}
	if linkCount != 2 || outboxCount != 0 {
		t.Fatalf("association/outbox count = %d/%d, want 2/0", linkCount, outboxCount)
	}

	listRecorder = doRequest(http.MethodGet, "/api/releases?q=harbor", "")
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode associated release list: %v", err)
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].Release.ProjectKey != "HIT" ||
		listPayload.Items[0].JiraIssueCount != 2 {
		t.Fatalf("associated release = %#v", listPayload.Items)
	}

	linkedRecorder := doRequest(
		http.MethodGet,
		"/api/releases/"+jsonNumber(release.ID)+"/jira-issues?scope=linked",
		"",
	)
	if linkedRecorder.Code != http.StatusOK {
		t.Fatalf("linked Jira issue status = %d, body=%s", linkedRecorder.Code, linkedRecorder.Body.String())
	}
	var linkedPayload struct {
		Items []struct {
			WorkItemID string `json:"work_item_id"`
			Linked     bool   `json:"linked"`
		} `json:"items"`
	}
	if err := json.Unmarshal(linkedRecorder.Body.Bytes(), &linkedPayload); err != nil {
		t.Fatalf("decode linked Jira issues: %v", err)
	}
	if len(linkedPayload.Items) != 2 || !linkedPayload.Items[0].Linked || !linkedPayload.Items[1].Linked {
		t.Fatalf("linked Jira issues = %#v", linkedPayload.Items)
	}
}

func TestBulkJiraIssueAssociationRejectsCrossProjectAtomically(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "release-owner@example.com", "Release Owner", []string{"delivery:read", "release:manage"})
	release := db.ReleaseVersion{
		ProjectKey: "HIT",
		Source:     "local",
		ExternalID: "local-atomic",
		Name:       "Harbor atomic",
		Status:     deliveryplanning.ReleasePlanned,
	}
	if err := db.DB.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	issues := []db.TaskTelemetry{
		{TaskID: "HIT-201", ExternalKey: "HIT-201", Source: "jira", ProjectKey: "HIT", IssueType: "requirement", PlanningState: deliveryplanning.PlanningReady},
		{TaskID: "OTHER-201", ExternalKey: "OTHER-201", Source: "jira", ProjectKey: "OTHER", IssueType: "requirement", PlanningState: deliveryplanning.PlanningReady},
	}
	if err := db.DB.Create(&issues).Error; err != nil {
		t.Fatalf("create Jira issues: %v", err)
	}
	server := NewServer(&config.Config{}, "")
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/releases/"+jsonNumber(release.ID)+"/jira-issues/bulk",
		strings.NewReader(`{"work_item_ids":["HIT-201","OTHER-201"],"reason":"atomic scope check"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "release_project_mismatch") {
		t.Fatalf("cross-project status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
	var count int64
	if err := db.DB.Model(&db.WorkItemReleaseLink{}).Where("release_version_id = ?", release.ID).Count(&count).Error; err != nil {
		t.Fatalf("count rolled back associations: %v", err)
	}
	if count != 0 {
		t.Fatalf("cross-project batch left %d associations, want 0", count)
	}
}

func TestScheduleProjectsPrimaryReleaseForProjectBoardCards(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "board-reader@example.com", "Board Reader", []string{"demands:read"})
	release := db.ReleaseVersion{
		ProjectKey: "HIT",
		Source:     "local",
		ExternalID: "local-board",
		Name:       "FMS 5.4.0",
		Status:     deliveryplanning.ReleasePlanned,
	}
	if err := db.DB.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	issue := db.TaskTelemetry{
		TaskID:        "HIT-301",
		ExternalKey:   "HIT-301",
		Source:        "jira",
		ProjectKey:    "HIT",
		IssueType:     "requirement",
		Title:         "Show release on project card",
		Status:        "progress",
		PlanningState: deliveryplanning.PlanningReady,
		LastUpdate:    time.Now(),
	}
	if err := db.DB.Create(&issue).Error; err != nil {
		t.Fatalf("create Jira issue: %v", err)
	}
	link := db.WorkItemReleaseLink{
		WorkItemID:       issue.TaskID,
		ReleaseVersionID: release.ID,
		Relation:         deliveryplanning.ReleaseTargetFix,
		IsPrimary:        true,
		Active:           true,
		Source:           "manual",
	}
	if err := db.DB.Create(&link).Error; err != nil {
		t.Fatalf("create release association: %v", err)
	}

	server := NewServer(&config.Config{}, "")
	request := httptest.NewRequest(http.MethodGet, "/api/schedule", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("schedule status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items []struct {
			DemandID          string `json:"demand_id"`
			TargetReleaseID   uint   `json:"target_release_id"`
			TargetReleaseName string `json:"target_release_name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode schedule response: %v", err)
	}
	if len(payload.Items) != 1 ||
		payload.Items[0].TargetReleaseID != release.ID ||
		payload.Items[0].TargetReleaseName != release.Name {
		t.Fatalf("schedule release projection = %#v, want %d/%q", payload.Items, release.ID, release.Name)
	}
}

func TestCreateReleaseSupportsUnboundAndCatalogBoundVersions(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "release-creator@example.com", "Release Creator", []string{"delivery:read", "release:manage"})
	if err := db.DB.Create(&db.ProjectConfig{ProjectKey: "HIT", ProjectName: "香港二期"}).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	server := NewServer(&config.Config{}, "")

	createRelease := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/api/releases", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, request)
		return recorder
	}

	unboundRecorder := createRelease(`{
		"name":"Harbor 3.0",
		"description":"Independent release fact",
		"status":"planned",
		"start_date":"2026-08-01",
		"release_date":"2026-08-31"
	}`)
	if unboundRecorder.Code != http.StatusCreated {
		t.Fatalf("unbound create status = %d, body=%s", unboundRecorder.Code, unboundRecorder.Body.String())
	}
	var unbound db.ReleaseVersion
	if err := json.Unmarshal(unboundRecorder.Body.Bytes(), &unbound); err != nil {
		t.Fatalf("decode unbound release: %v", err)
	}
	if unbound.ID == 0 || unbound.ProjectKey != "" || unbound.Source != "local" || unbound.Name != "Harbor 3.0" {
		t.Fatalf("unexpected unbound release: %#v", unbound)
	}

	boundRecorder := createRelease(`{"name":"Harbor 3.1","project_key":"hit"}`)
	if boundRecorder.Code != http.StatusCreated {
		t.Fatalf("bound create status = %d, body=%s", boundRecorder.Code, boundRecorder.Body.String())
	}
	var bound db.ReleaseVersion
	if err := json.Unmarshal(boundRecorder.Body.Bytes(), &bound); err != nil {
		t.Fatalf("decode bound release: %v", err)
	}
	if bound.ProjectKey != "HIT" || bound.Source != "local" || bound.Name != "Harbor 3.1" {
		t.Fatalf("unexpected bound release: %#v", bound)
	}

	scopedRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/HIT/releases",
		strings.NewReader(`{"name":"Harbor 3.2"}`),
	)
	scopedRequest.Header.Set("Authorization", "Bearer "+token)
	scopedRequest.Header.Set("Content-Type", "application/json")
	scopedRecorder := httptest.NewRecorder()
	server.mux.ServeHTTP(scopedRecorder, scopedRequest)
	if scopedRecorder.Code != http.StatusCreated {
		t.Fatalf("project-scoped compatibility status = %d, body=%s", scopedRecorder.Code, scopedRecorder.Body.String())
	}
	var scoped db.ReleaseVersion
	if err := json.Unmarshal(scopedRecorder.Body.Bytes(), &scoped); err != nil {
		t.Fatalf("decode project-scoped release: %v", err)
	}
	if scoped.ProjectKey != "HIT" || scoped.Name != "Harbor 3.2" {
		t.Fatalf("unexpected project-scoped release: %#v", scoped)
	}

	unknownRecorder := createRelease(`{"name":"Unknown project release","project_key":"missing"}`)
	if unknownRecorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(unknownRecorder.Body.String(), "unknown_project") {
		t.Fatalf("unknown project status/body = %d/%s", unknownRecorder.Code, unknownRecorder.Body.String())
	}
}

func TestReleaseCatalogSyncIsFeatureGatedAndIdempotent(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{Jira: config.JiraConfig{
		Enabled:               true,
		VersionCatalogEnabled: true,
	}}, "")
	server.jiraReleaseList = func(ctx context.Context, projectKey string) ([]deliveryplanning.ExternalRelease, error) {
		return []deliveryplanning.ExternalRelease{{
			ProjectKey: projectKey,
			ExternalID: "13622",
			Name:       "ReeWell-1.1",
			Status:     deliveryplanning.ReleasePlanned,
		}}, nil
	}

	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/projects/PRJ25024/releases/sync", nil)
		request.SetPathValue("project_key", "PRJ25024")
		recorder := httptest.NewRecorder()
		server.handleSyncProjectReleases(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("sync attempt %d status = %d, body=%s", attempt, recorder.Code, recorder.Body.String())
		}
	}
	var count int64
	db.DB.Model(&db.ReleaseVersion{}).Count(&count)
	if count != 1 {
		t.Fatalf("idempotent release count = %d, want 1", count)
	}

	server.config.Jira.VersionCatalogEnabled = false
	request := httptest.NewRequest(http.MethodPost, "/api/projects/PRJ25024/releases/sync", nil)
	request.SetPathValue("project_key", "PRJ25024")
	recorder := httptest.NewRecorder()
	server.handleSyncProjectReleases(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("disabled catalog status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestPatchWorkItemPlanningReturnsActionableConflictsAndAudit(t *testing.T) {
	setupServerTestDB(t)
	release := db.ReleaseVersion{
		ProjectKey: "HIT",
		Source:     "jira",
		ExternalID: "12",
		Name:       "1.2",
		Status:     deliveryplanning.ReleasePlanned,
	}
	if err := db.DB.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	task := db.TaskTelemetry{
		TaskID:        "HIT-401",
		IssueType:     "requirement",
		ProjectKey:    "HIT",
		Source:        "jira",
		ExternalKey:   "HIT-401",
		PlanningState: deliveryplanning.PlanningReady,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	server := NewServer(&config.Config{}, "")
	body := []byte(`{
		"expected_revision":0,
		"primary_target_release_id":` + jsonNumber(release.ID) + `,
		"planning_state":"committed",
		"assignee":"Owner",
		"due_date":"2026-08-15",
		"reason":"纳入 1.2 发布范围"
	}`)
	request := httptest.NewRequest(http.MethodPatch, "/api/work-items/HIT-401/planning", bytes.NewReader(body))
	request.SetPathValue("id", task.TaskID)
	request.Header.Set("x-authenticated-user-id", "pm@example.com")
	recorder := httptest.NewRecorder()
	server.handlePatchWorkItemPlanning(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("planning status = %d, body=%s", recorder.Code, recorder.Body.String())
	}

	var updated db.TaskTelemetry
	if err := db.DB.First(&updated, "task_id = ?", task.TaskID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if updated.Revision != 1 || updated.PlanningState != deliveryplanning.PlanningCommitted || updated.DueDate == nil {
		t.Fatalf("unexpected updated task: %+v", updated)
	}
	var eventCount, outboxCount int64
	db.DB.Model(&db.WorkItemEvent{}).Where("work_item_id = ?", task.TaskID).Count(&eventCount)
	db.DB.Model(&db.WorkItemSyncOperation{}).Where("work_item_id = ?", task.TaskID).Count(&outboxCount)
	if eventCount != 1 || outboxCount != 1 {
		t.Fatalf("event/outbox count = %d/%d, want 1/1", eventCount, outboxCount)
	}

	request = httptest.NewRequest(http.MethodPatch, "/api/work-items/HIT-401/planning", bytes.NewReader(body))
	request.SetPathValue("id", task.TaskID)
	request.Header.Set("x-authenticated-user-id", "pm@example.com")
	recorder = httptest.NewRecorder()
	server.handlePatchWorkItemPlanning(recorder, request)
	if recorder.Code != http.StatusConflict || !bytes.Contains(recorder.Body.Bytes(), []byte("revision_conflict")) {
		t.Fatalf("stale revision status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
}

func TestBulkPlanningIsAtomicAndDecodesPlanningFields(t *testing.T) {
	setupServerTestDB(t)
	release := db.ReleaseVersion{
		ProjectKey: "HIT",
		Source:     "jira",
		ExternalID: "bulk-12",
		Name:       "1.2",
		Status:     deliveryplanning.ReleasePlanned,
	}
	if err := db.DB.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	tasks := []db.TaskTelemetry{
		{TaskID: "HIT-501", IssueType: "requirement", ProjectKey: "HIT", PlanningState: deliveryplanning.PlanningReady},
		{TaskID: "HIT-502", IssueType: "requirement", ProjectKey: "HIT", PlanningState: deliveryplanning.PlanningReady},
	}
	if err := db.DB.Create(&tasks).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	server := NewServer(&config.Config{}, "")

	requestBody := func(secondRevision uint) []byte {
		return []byte(`{"items":[
			{"work_item_id":"HIT-501","expected_revision":0,"primary_target_release_id":` + jsonNumber(release.ID) + `,"reason":"批量归入 1.2"},
			{"work_item_id":"HIT-502","expected_revision":` + strconv.FormatUint(uint64(secondRevision), 10) + `,"primary_target_release_id":` + jsonNumber(release.ID) + `,"reason":"批量归入 1.2"}
		]}`)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/work-items/bulk-planning", bytes.NewReader(requestBody(99)))
	request.Header.Set("x-authenticated-user-id", "pm@example.com")
	recorder := httptest.NewRecorder()
	server.handleBulkWorkItemPlanning(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("failed bulk status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	for _, taskID := range []string{"HIT-501", "HIT-502"} {
		var task db.TaskTelemetry
		if err := db.DB.First(&task, "task_id = ?", taskID).Error; err != nil {
			t.Fatalf("reload %s: %v", taskID, err)
		}
		if task.Revision != 0 {
			t.Fatalf("%s revision = %d after failed batch, want 0", taskID, task.Revision)
		}
	}

	request = httptest.NewRequest(http.MethodPost, "/api/work-items/bulk-planning", bytes.NewReader(requestBody(0)))
	request.Header.Set("x-authenticated-user-id", "pm@example.com")
	recorder = httptest.NewRecorder()
	server.handleBulkWorkItemPlanning(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("successful bulk status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	for _, taskID := range []string{"HIT-501", "HIT-502"} {
		var task db.TaskTelemetry
		if err := db.DB.First(&task, "task_id = ?", taskID).Error; err != nil {
			t.Fatalf("reload %s: %v", taskID, err)
		}
		if task.Revision != 1 {
			t.Fatalf("%s revision = %d after successful batch, want 1", taskID, task.Revision)
		}
	}
}

func TestReleaseStatusCannotReopen(t *testing.T) {
	setupServerTestDB(t)
	release := db.ReleaseVersion{
		ProjectKey: "HIT",
		Source:     "local",
		ExternalID: "local-1",
		Name:       "1.0",
		Status:     deliveryplanning.ReleaseArchived,
	}
	if err := db.DB.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}
	server := NewServer(&config.Config{}, "")
	request := httptest.NewRequest(http.MethodPatch, "/api/releases/1", bytes.NewBufferString(`{"status":"planned"}`))
	request.SetPathValue("id", jsonNumber(release.ID))
	recorder := httptest.NewRecorder()
	server.handlePatchRelease(recorder, request)
	if recorder.Code != http.StatusConflict || !bytes.Contains(recorder.Body.Bytes(), []byte("invalid_release_transition")) {
		t.Fatalf("reopen status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
}

func TestDeliveryPermissionsPreserveLegacyRoleCompatibility(t *testing.T) {
	setupServerTestDB(t)
	assertGroupPermission := func(group, permission string) {
		t.Helper()
		var count int64
		if err := db.DB.Table("group_permissions").
			Joins("JOIN user_groups ON user_groups.id = group_permissions.user_group_id").
			Joins("JOIN permissions ON permissions.id = group_permissions.permission_id").
			Where("user_groups.name = ? AND permissions.code = ?", group, permission).
			Count(&count).Error; err != nil {
			t.Fatalf("query %s/%s: %v", group, permission, err)
		}
		if count != 1 {
			t.Fatalf("%s permission %s count = %d, want 1", group, permission, count)
		}
	}
	assertGroupPermission("member", "delivery:read")
	assertGroupPermission("admin", "delivery:plan")
	assertGroupPermission("admin", "release:manage")

	custom := userdb.UserGroup{Name: "legacy_pm", DisplayName: "Legacy PM"}
	if err := db.DB.Create(&custom).Error; err != nil {
		t.Fatalf("create custom group: %v", err)
	}
	var legacy, delivery userdb.Permission
	if err := db.DB.Where("code = ?", "demands:write").First(&legacy).Error; err != nil {
		t.Fatalf("load legacy permission: %v", err)
	}
	if err := db.DB.Where("code = ?", "delivery:plan").First(&delivery).Error; err != nil {
		t.Fatalf("load delivery permission: %v", err)
	}
	if err := db.DB.Create(&userdb.GroupPermission{UserGroupID: custom.ID, PermissionID: legacy.ID}).Error; err != nil {
		t.Fatalf("grant legacy permission: %v", err)
	}
	if err := userdb.InitializeSeeds(db.DB); err != nil {
		t.Fatalf("rerun incremental seeds: %v", err)
	}
	var count int64
	db.DB.Model(&userdb.GroupPermission{}).
		Where("user_group_id = ? AND permission_id = ?", custom.ID, delivery.ID).
		Count(&count)
	if count != 1 {
		t.Fatalf("legacy custom group delivery compatibility count = %d, want 1", count)
	}
}

func jsonNumber(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
