package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/deliveryplanning"
	"well-ambient/internal/kanban"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestScheduleResponseV1CompatibilityFixture(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
	}, "")

	request := httptest.NewRequest(http.MethodGet, "/api/schedule", nil)
	recorder := httptest.NewRecorder()
	srv.handleGetSchedule(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/schedule status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var actual map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&actual); err != nil {
		t.Fatalf("decode actual schedule response: %v", err)
	}
	actual["generated_at"] = "<generated-at>"

	fixtureBytes, err := os.ReadFile("testdata/schedule_response_v1.json")
	if err != nil {
		t.Fatalf("read schedule compatibility fixture: %v", err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatalf("decode schedule compatibility fixture: %v", err)
	}
	if !reflect.DeepEqual(actual, fixture) {
		actualJSON, _ := json.MarshalIndent(actual, "", "  ")
		t.Fatalf("schedule response drifted from v1 fixture:\n%s", actualJSON)
	}
}

func TestServerEndpoints(t *testing.T) {
	// Initialize in-memory SQLite database
	err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Seed Mock User Eddie as Super Admin
	mockUser := userdb.User{
		Username: "Eddie",
		Email:    "eddie@westwell-lab.com",
		Name:     "Eddie",
		Avatar:   "",
	}
	if err := db.DB.Create(&mockUser).Error; err != nil {
		t.Fatalf("Failed to seed mock user: %v", err)
	}

	var superGroup userdb.UserGroup
	if err := db.DB.Where("name = ?", "super_admin").First(&superGroup).Error; err != nil {
		t.Fatalf("Failed to query super_admin group: %v", err)
	}

	membership := userdb.UserGroupMembership{
		UserID:      mockUser.ID,
		UserGroupID: superGroup.ID,
		Scope:       "global",
		ScopeID:     "",
	}
	if err := db.DB.Create(&membership).Error; err != nil {
		t.Fatalf("Failed to bind membership: %v", err)
	}

	token, err := GenerateJWT("Eddie", "Eddie", "mock_wellos_token", "", []string{"super_admin"}, []string{"config:read", "config:write", "users:read", "users:write"})
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Seed some tasks
	task1 := db.TaskTelemetry{
		TaskID:     "task-001",
		Title:      "Test Task 1",
		Repo:       "repo-1",
		Assignee:   "Alice",
		Branch:     "main",
		LastCommit: "feat: initial commit",
		Status:     "backlog",
		LastUpdate: time.Now(),
	}
	task2 := db.TaskTelemetry{
		TaskID:     "task-002",
		Title:      "Test Task 2",
		Repo:       "repo-2",
		Assignee:   "Bob",
		Branch:     "dev",
		LastCommit: "feat: work in progress",
		Status:     "progress",
		LastUpdate: time.Now(),
	}
	if err := db.DB.Create(&task1).Error; err != nil {
		t.Fatalf("Failed to seed task1: %v", err)
	}
	if err := db.DB.Create(&task2).Error; err != nil {
		t.Fatalf("Failed to seed task2: %v", err)
	}

	// Seed some webhook logs
	log1 := db.WebhookLog{
		Event:     "push",
		Payload:   `{"ref":"refs/heads/main"}`,
		CreatedAt: time.Now(),
	}
	log2 := db.WebhookLog{
		Event:     "merge_request",
		Payload:   `{"action":"open"}`,
		CreatedAt: time.Now().Add(-1 * time.Minute),
	}
	if err := db.DB.Create(&log1).Error; err != nil {
		t.Fatalf("Failed to seed log1: %v", err)
	}
	if err := db.DB.Create(&log2).Error; err != nil {
		t.Fatalf("Failed to seed log2: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 8080, Host: "127.0.0.1"},
	}
	srv := NewServer(cfg, "")

	// Test GET /api/tasks
	reqTasks, err := http.NewRequest("GET", "/api/tasks", nil)
	if err != nil {
		t.Fatalf("Failed to create request for tasks: %v", err)
	}
	reqTasks.Header.Set("Authorization", "Bearer "+token)
	rrTasks := httptest.NewRecorder()
	srv.mux.ServeHTTP(rrTasks, reqTasks)

	if status := rrTasks.Code; status != http.StatusOK {
		t.Errorf("GET /api/tasks returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var fetchedTasks []db.TaskTelemetry
	if err := json.NewDecoder(rrTasks.Body).Decode(&fetchedTasks); err != nil {
		t.Fatalf("Failed to decode tasks response: %v", err)
	}

	if len(fetchedTasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(fetchedTasks))
	}

	// Test GET /api/logs
	reqLogs, err := http.NewRequest("GET", "/api/logs", nil)
	if err != nil {
		t.Fatalf("Failed to create request for logs: %v", err)
	}
	reqLogs.Header.Set("Authorization", "Bearer "+token)
	rrLogs := httptest.NewRecorder()
	srv.mux.ServeHTTP(rrLogs, reqLogs)

	if status := rrLogs.Code; status != http.StatusOK {
		t.Errorf("GET /api/logs returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var fetchedLogs []db.WebhookLog
	if err := json.NewDecoder(rrLogs.Body).Decode(&fetchedLogs); err != nil {
		t.Fatalf("Failed to decode logs response: %v", err)
	}

	if len(fetchedLogs) != 2 {
		t.Errorf("Expected 2 logs, got %d", len(fetchedLogs))
	}
	// Verify that logs are sorted by created_at desc (log1 is newer and should be first)
	if fetchedLogs[0].Event != "push" {
		t.Errorf("Expected first log to be 'push' event, got %q", fetchedLogs[0].Event)
	}
}

func TestGetTasksFiltersNonCoreMemberData(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "task-reader@westwell-lab.com", "Task Reader", []string{"dashboard:read"})
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira:   config.JiraConfig{SyncUsers: []string{"Alice"}},
	}
	srv := NewServer(cfg, "")

	tasks := []db.TaskTelemetry{
		{TaskID: "CORE-1", Title: "Core task", Assignee: "Alice", Status: "progress", IssueType: "task", LastUpdate: time.Now()},
		{TaskID: "EXT-1", Title: "External task", Assignee: "Vendor", Status: "progress", IssueType: "task", LastUpdate: time.Now()},
	}
	for _, task := range tasks {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed task %s: %v", task.TaskID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/tasks status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response []db.TaskTelemetry
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode tasks response: %v", err)
	}
	if len(response) != 1 || response[0].TaskID != "CORE-1" {
		t.Fatalf("non-core task should not be returned: %+v", response)
	}

	externalReq := httptest.NewRequest(http.MethodGet, "/api/tasks?assignee=外部协同", nil)
	externalReq.Header.Set("Authorization", "Bearer "+token)
	externalRR := httptest.NewRecorder()
	srv.mux.ServeHTTP(externalRR, externalReq)
	if externalRR.Code != http.StatusOK {
		t.Fatalf("GET /api/tasks?assignee=外部协同 status = %d", externalRR.Code)
	}
	var externalResponse []db.TaskTelemetry
	if err := json.NewDecoder(externalRR.Body).Decode(&externalResponse); err != nil {
		t.Fatalf("decode external response: %v", err)
	}
	if len(externalResponse) != 0 {
		t.Fatalf("external assignee filter should not return non-core data: %+v", externalResponse)
	}
}

func TestGetTasksSupportsRepeatedProjectAndAssigneeFilters(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "multi-filter-reader@westwell-lab.com", "Multi Filter Reader", []string{"dashboard:read"})
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira:   config.JiraConfig{SyncUsers: []string{"Alice", "Bob"}},
	}
	srv := NewServer(cfg, "")

	tasks := []db.TaskTelemetry{
		{TaskID: "HIT-1", Title: "HIT task", Assignee: "Alice", Status: "progress", IssueType: "task", LastUpdate: time.Now()},
		{TaskID: "NS2-2", Title: "NS2 task", Assignee: "Bob", Status: "review", IssueType: "task", LastUpdate: time.Now()},
		{TaskID: "DG-3", Title: "DG task", Assignee: "Alice", Status: "progress", IssueType: "task", LastUpdate: time.Now()},
		{TaskID: "HIT-4", Title: "External task", Assignee: "Vendor", Status: "progress", IssueType: "task", LastUpdate: time.Now()},
	}
	if err := db.DB.Create(&tasks).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks?project=HIT&project=NS2&assignee=Alice&assignee=Bob", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET repeated task filters status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response []db.TaskTelemetry
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode repeated-filter response: %v", err)
	}
	returned := make(map[string]bool, len(response))
	for _, task := range response {
		returned[task.TaskID] = true
	}
	if len(response) != 2 || !returned["HIT-1"] || !returned["NS2-2"] {
		t.Fatalf("repeated filters returned %+v, want HIT-1 and NS2-2", response)
	}
}

func TestGetScheduleFiltersNonCoreMemberData(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "schedule-reader@westwell-lab.com", "Schedule Reader", []string{"demands:read"})
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira:   config.JiraConfig{SyncUsers: []string{"Alice"}},
	}
	srv := NewServer(cfg, "")

	now := time.Now()
	tasks := []db.TaskTelemetry{
		{TaskID: "CORE-DEMAND", Title: "Core demand", Assignee: "Alice", Status: "progress", IssueType: "demand", TaskGroupID: "group-core", LastUpdate: now},
		{TaskID: "EXT-DEMAND", Title: "External demand", Assignee: "Vendor", Status: "progress", IssueType: "demand", LastUpdate: now},
		{TaskID: "EXT-SUBTASK", Title: "External subtask", Assignee: "Vendor", Status: "progress", IssueType: "task", TaskGroupID: "group-core", LastUpdate: now},
	}
	for _, task := range tasks {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed task %s: %v", task.TaskID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/schedule", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/schedule status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response ScheduleResponseDTO
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode schedule response: %v", err)
	}
	if response.Summary.Total != 1 || len(response.Items) != 1 || response.Items[0].DemandID != "CORE-DEMAND" {
		t.Fatalf("non-core demand should not be returned: %+v", response)
	}
	if response.Items[0].SubtaskTotal != 0 {
		t.Fatalf("non-core subtask should not participate in schedule display stats: %+v", response.Items[0])
	}
}

func TestGetExecutionTasksFiltersNonCoreMemberData(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "execution-reader@westwell-lab.com", "Execution Reader", []string{"dashboard:read"})
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira:   config.JiraConfig{SyncUsers: []string{"Bob"}},
	}
	srv := NewServer(cfg, "")

	now := time.Now()
	tasks := []db.TaskTelemetry{
		{TaskID: "DEMAND-CORE", Title: "Core parent demand", Assignee: "Bob", Status: "progress", IssueType: "demand", TaskGroupID: "group-core", LastUpdate: now},
		{TaskID: "EXEC-CORE", Title: "Core execution with hidden external child owner", Source: "jira", Assignee: "Vendor", Status: "progress", IssueType: "task", TaskGroupID: "group-core", LastUpdate: now},
		{TaskID: "EXEC-EXT", Title: "External execution", Source: "jira", Assignee: "Vendor", Status: "progress", IssueType: "task", LastUpdate: now},
	}
	for _, task := range tasks {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed task %s: %v", task.TaskID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/execution/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/execution/tasks status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response ExecutionTasksResponseDTO
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode execution response: %v", err)
	}
	if response.Summary.Total != 1 || len(response.Items) != 1 || response.Items[0].TaskID != "EXEC-CORE" {
		t.Fatalf("non-core execution item should not be returned: %+v", response)
	}
	if response.Items[0].ExecutionAssignee != "" {
		t.Fatalf("non-core child execution assignee should not be returned: %+v", response.Items[0])
	}
}

func TestConfigAPI(t *testing.T) {
	setupServerTestDB(t)

	// Create a temporary file for config.yaml
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9000, Host: "127.0.0.1"},
		GitLab: config.GitLabConfig{BaseURL: "https://example.com"},
	}
	if err := config.SaveConfig(tmpFile.Name(), cfg); err != nil {
		t.Fatalf("Failed to seed bootstrap config file: %v", err)
	}

	srv := NewServer(cfg, tmpFile.Name())

	token, err := func() (string, error) {
		// Ensure user exists in memory DB
		var u userdb.User
		if err := db.DB.Where("username = ?", "Eddie").First(&u).Error; err != nil {
			mockUser := userdb.User{
				Username: "Eddie",
				Email:    "eddie@westwell-lab.com",
				Name:     "Eddie",
				Avatar:   "",
			}
			db.DB.Create(&mockUser)
			var superGroup userdb.UserGroup
			db.DB.Where("name = ?", "super_admin").First(&superGroup)
			membership := userdb.UserGroupMembership{
				UserID:      mockUser.ID,
				UserGroupID: superGroup.ID,
				Scope:       "global",
				ScopeID:     "",
			}
			db.DB.Create(&membership)
		}
		return GenerateJWT("Eddie", "Eddie", "mock_wellos_token", "", []string{"super_admin"}, []string{"config:read", "config:write", "users:read", "users:write"})
	}()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Test GET /api/config
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/config status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var fetchedCfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&fetchedCfg); err != nil {
		t.Fatal(err)
	}
	if fetchedCfg.GitLab.BaseURL != "https://example.com" {
		t.Errorf("Expected GitLab.BaseURL %q, got %q", "https://example.com", fetchedCfg.GitLab.BaseURL)
	}

	// Test POST /api/config
	newCfg := config.Config{
		Server: config.ServerConfig{Port: 9000, Host: "127.0.0.1"},
		GitLab: config.GitLabConfig{BaseURL: "https://new-gitlab.com"},
	}
	body, _ := json.Marshal(newCfg)
	req, err = http.NewRequest("POST", "/api/config", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/config status code: %v", rr.Code)
	}

	// Verify update in memory
	if srv.config.GitLab.BaseURL != "https://new-gitlab.com" {
		t.Errorf("Expected in-memory config update to %q, got %q", "https://new-gitlab.com", srv.config.GitLab.BaseURL)
	}

	// Runtime settings persist in the database; bootstrap YAML remains unchanged.
	savedCfg, err := config.LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read bootstrap config file: %v", err)
	}
	if savedCfg.GitLab.BaseURL != "https://example.com" {
		t.Errorf("Expected bootstrap config to remain %q, got %q", "https://example.com", savedCfg.GitLab.BaseURL)
	}
	var runtimeConfig db.RuntimeConfig
	if err := db.DB.First(&runtimeConfig, runtimeConfigSingletonID).Error; err != nil {
		t.Fatalf("Failed to read current database config: %v", err)
	}
	if !strings.Contains(runtimeConfig.ConfigJSON, `"base_url":"https://new-gitlab.com"`) {
		t.Errorf("Expected database config update, got %s", runtimeConfig.ConfigJSON)
	}

	// Test POST /api/config/test (invalid/unreachable url)
	testReq := ConnectionTestRequest{
		Type: "gitlab",
		GitLab: &config.GitLabConfig{
			BaseURL: "http://invalid-domain-well-ambient-12345.com",
		},
	}
	testBody, _ := json.Marshal(testReq)
	req, err = http.NewRequest("POST", "/api/config/test", bytes.NewBuffer(testBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/config/test status code: %v", rr.Code)
	}

	var testRes ConnectionTestResponse
	if err := json.NewDecoder(rr.Body).Decode(&testRes); err != nil {
		t.Fatal(err)
	}
	if testRes.Success {
		t.Error("Expected connection test to fail for invalid domain")
	}
}

func TestDecryptCryptoJSAES(t *testing.T) {
	passphrase := "django-insecure-()$+l&t333b4ncc0hrw!u!^yd_oja&0qc4n#&xnfcm)5r^n$5k"
	ciphertext := "U2FsdGVkX1/YrFwF4MkCfpE/fcAUsvqDtzgow9841/OfBhstusJu9iRQDfiZERf6"
	expected := "hello-world-12345"

	decrypted, err := DecryptCryptoJSAES(ciphertext, passphrase)
	if err != nil {
		t.Fatalf("Failed to decrypt CryptoJS AES: %v", err)
	}

	if decrypted != expected {
		t.Errorf("Decrypted content mismatch: got %q, want %q", decrypted, expected)
	}
}

func TestGenerateJWTIncludesDepartment(t *testing.T) {
	token, err := GenerateJWT(
		"eddie@westwell-lab.com",
		"Eddie",
		"mock_wellos_token",
		"",
		[]string{"member"},
		[]string{"dashboard:read"},
		"AI Platform",
	)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("Failed to parse JWT: %v", err)
	}

	if claims.Department != "AI Platform" {
		t.Fatalf("Department claim = %q, want %q", claims.Department, "AI Platform")
	}
}

func TestExtractDept(t *testing.T) {
	tests := []struct {
		name      string
		in        interface{}
		candidate bool
		want      string
	}{
		{
			name:      "top level department string",
			in:        "AI Lab",
			candidate: true,
			want:      "AI Lab",
		},
		{
			name: "nested department object",
			in: map[string]interface{}{
				"data": map[string]interface{}{
					"user_info": map[string]interface{}{
						"name": "Eddie",
						"department": map[string]interface{}{
							"name": "Platform Engineering",
						},
					},
				},
			},
			want: "Platform Engineering",
		},
		{
			name: "dept keyword nested under array",
			in: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "ignored-person"},
					map[string]interface{}{
						"dept_id_data": map[string]interface{}{
							"label": "Data Intelligence",
						},
					},
				},
			},
			want: "Data Intelligence",
		},
		{
			name: "organization wrapper",
			in: map[string]interface{}{
				"data": map[string]interface{}{
					"profile": map[string]interface{}{
						"organization": map[string]interface{}{
							"label": "Product Intelligence",
						},
					},
				},
			},
			want: "Product Intelligence",
		},
		{
			name: "chinese department key",
			in: map[string]interface{}{
				"data": map[string]interface{}{
					"部门名称": "智能调度部",
				},
			},
			want: "智能调度部",
		},
		{
			name: "department display name inside department context",
			in: map[string]interface{}{
				"data": map[string]interface{}{
					"profile": map[string]interface{}{
						"department": map[string]interface{}{
							"displayName": "Design Systems",
						},
					},
				},
			},
			want: "Design Systems",
		},
		{
			name: "numeric department fallback",
			in: map[string]interface{}{
				"dept_id": "12345",
			},
			want: "12345",
		},
		{
			name: "skip user name outside department context",
			in: map[string]interface{}{
				"data": map[string]interface{}{
					"name":     "Personal Name",
					"username": "person@example.com",
				},
			},
			want: "",
		},
		{
			name: "skip display name outside department context",
			in: map[string]interface{}{
				"data": map[string]interface{}{
					"displayName": "Eddie",
					"avatar":      "https://example.test/avatar.png",
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDept(tt.in)
			if tt.candidate {
				got = extractDeptCandidate(tt.in)
			}
			if got != tt.want {
				t.Fatalf("department extraction = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactSensitiveLoginPayload(t *testing.T) {
	raw := []byte(`{
		"code":0,
		"token":"secret-token",
		"data":{"access_token":"access-secret","profile":{"name":"Eddie","department":"AI Lab"}},
		"items":[{"refresh_token":"refresh-secret","label":"safe"}]
	}`)

	redacted := redactSensitiveLoginPayload(raw)
	for _, leaked := range []string{"secret-token", "access-secret", "refresh-secret"} {
		if strings.Contains(redacted, leaked) {
			t.Fatalf("redacted payload leaked %q: %s", leaked, redacted)
		}
	}
	if !strings.Contains(redacted, "AI Lab") {
		t.Fatalf("redacted payload should preserve non-sensitive fields: %s", redacted)
	}
}

func TestLoginRefreshesAvatarAndDepartmentFromWellOSUserInfo(t *testing.T) {
	setupServerTestDB(t)

	oldLoginDoer := wellOSLoginDoer
	oldUserInfoDoer := wellOSUserInfoDoer
	wellOSLoginDoer = func(client *http.Client, username, password string) (*http.Response, error) {
		if username != "zhiyuan_liang@westwell-lab.com" || password != "password-ok" {
			t.Fatalf("unexpected login credentials: %s / %s", username, password)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"token":"wellos-user-token","name":"旧姓名"}`)),
			Header:     make(http.Header),
		}, nil
	}
	wellOSUserInfoDoer = func(client *http.Client, token string) (*http.Response, error) {
		if token != "wellos-user-token" {
			t.Fatalf("user info token = %q, want wellos-user-token", token)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"code": 0,
				"data": {
					"uid": 1136,
					"email": "zhiyuan_liang@westwell-lab.com",
					"avatar": "https://s1-imfile.feishucdn.com/static-resource/v1/avatar.png",
					"realname": "梁志远",
					"department_name": "研发二部",
					"role_name": "研发工程师"
				},
				"msg": "success"
			}`)),
			Header: make(http.Header),
		}, nil
	}
	t.Cleanup(func() {
		wellOSLoginDoer = oldLoginDoer
		wellOSUserInfoDoer = oldUserInfoDoer
	})

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9103, Host: "127.0.0.1"}}, "")
	body := bytes.NewBufferString(`{"username":"zhiyuan_liang@westwell-lab.com","password":"password-ok"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	user, ok := res["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("login response missing user: %+v", res)
	}
	if user["name"] != "梁志远" {
		t.Fatalf("response user.name = %v, want 梁志远", user["name"])
	}
	if user["department"] != "研发二部" {
		t.Fatalf("response user.department = %v, want 研发二部", user["department"])
	}
	if user["avatar"] != "https://s1-imfile.feishucdn.com/static-resource/v1/avatar.png" {
		t.Fatalf("response user.avatar = %v", user["avatar"])
	}

	token, _ := res["token"].(string)
	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("parse login JWT: %v", err)
	}
	if claims.Name != "梁志远" || claims.Department != "研发二部" || claims.Avatar == "" {
		t.Fatalf("JWT profile mismatch: %+v", claims)
	}

	var saved userdb.User
	if err := db.DB.Where("username = ?", "zhiyuan_liang@westwell-lab.com").First(&saved).Error; err != nil {
		t.Fatalf("saved user not found: %v", err)
	}
	if saved.Name != "梁志远" || saved.Department != "研发二部" || saved.Avatar == "" {
		t.Fatalf("saved user profile mismatch: %+v", saved)
	}
}

func setupServerTestDB(t *testing.T) {
	t.Helper()
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
}

func superAdminToken(t *testing.T, username, name string, permissions []string) string {
	t.Helper()

	mockUser := userdb.User{
		Username: username,
		Email:    username,
		Name:     name,
	}
	if err := db.DB.Create(&mockUser).Error; err != nil {
		t.Fatalf("Failed to seed mock user: %v", err)
	}

	var superGroup userdb.UserGroup
	if err := db.DB.Where("name = ?", "super_admin").First(&superGroup).Error; err != nil {
		t.Fatalf("Failed to query super_admin group: %v", err)
	}

	if err := db.DB.Create(&userdb.UserGroupMembership{
		UserID:      mockUser.ID,
		UserGroupID: superGroup.ID,
		Scope:       "global",
		ScopeID:     "",
	}).Error; err != nil {
		t.Fatalf("Failed to bind membership: %v", err)
	}

	token, err := GenerateJWT(username, name, "mock_wellos_token", "", []string{"super_admin"}, permissions)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	return token
}

func TestGetScheduleBuildsDemandTimeline(t *testing.T) {
	setupServerTestDB(t)

	token := superAdminToken(t, "pm@westwell-lab.com", "PM", []string{"demands:read"})
	cfg := &config.Config{Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"}}
	srv := NewServer(cfg, "")

	now := time.Now()
	overdueDue := now.AddDate(0, 0, -2)
	soonDue := now.AddDate(0, 0, 2)
	staleDue := now.AddDate(0, 0, 10)
	staleUpdate := now.AddDate(0, 0, -5)
	doneAt := now.AddDate(0, 0, -1)

	users := []userdb.User{
		{Username: "alice@westwell-lab.com", Email: "alice@westwell-lab.com", Name: "Alice", Department: "Product"},
		{Username: "bob@westwell-lab.com", Email: "bob@westwell-lab.com", Name: "Bob", Department: "Engineering"},
	}
	for _, user := range users {
		if err := db.DB.Create(&user).Error; err != nil {
			t.Fatalf("seed user %s: %v", user.Username, err)
		}
	}

	tasks := []db.TaskTelemetry{
		{
			TaskID:        "DEMAND-OVERDUE",
			Title:         "Overdue demand",
			Repo:          "platform-core",
			Assignee:      "Alice",
			CreatorDept:   "Product",
			Branch:        "feat/overdue",
			Status:        "progress",
			IssueType:     "demand",
			TaskGroupID:   "group-overdue",
			DueDate:       &overdueDue,
			TaskCreatedAt: now.AddDate(0, 0, -8),
			LastUpdate:    now,
		},
		{
			TaskID:        "DEMAND-SOON",
			Title:         "Due soon demand",
			Repo:          "web",
			Assignee:      "Bob",
			Branch:        "feat/soon",
			Status:        "backlog",
			IssueType:     "demand",
			TaskGroupID:   "group-soon",
			DueDate:       &soonDue,
			TaskCreatedAt: now.AddDate(0, 0, -1),
			LastUpdate:    now,
		},
		{
			TaskID:        "DEMAND-UNSCHEDULED",
			Title:         "Unscheduled demand",
			Assignee:      "Alice",
			Branch:        "-",
			Status:        "backlog",
			IssueType:     "demand",
			TaskCreatedAt: now.AddDate(0, 0, -1),
			LastUpdate:    now,
		},
		{
			TaskID:        "DEMAND-STALE",
			Title:         "Stale demand",
			Assignee:      "Bob",
			Branch:        "feat/stale",
			Status:        "progress",
			IssueType:     "demand",
			DueDate:       &staleDue,
			TaskCreatedAt: now.AddDate(0, 0, -10),
			LastUpdate:    staleUpdate,
		},
		{
			TaskID:        "DEMAND-DONE",
			Title:         "Delivered demand",
			Assignee:      "Bob",
			Branch:        "feat/done",
			Status:        "done",
			IssueType:     "demand",
			DueDate:       &soonDue,
			CompletedAt:   &doneAt,
			TaskCreatedAt: now.AddDate(0, 0, -3),
			LastUpdate:    doneAt,
		},
		{
			TaskID:        "TASK-OVERDUE-1",
			Title:         "Subtask done",
			Assignee:      "Alice",
			Status:        "done",
			IssueType:     "task",
			TaskGroupID:   "group-overdue",
			TaskCreatedAt: now.AddDate(0, 0, -4),
			LastUpdate:    now,
		},
		{
			TaskID:        "TASK-OVERDUE-2",
			Title:         "Subtask active",
			Assignee:      "Alice",
			Status:        "progress",
			IssueType:     "task",
			TaskGroupID:   "group-overdue",
			TaskCreatedAt: now.AddDate(0, 0, -3),
			LastUpdate:    now,
		},
	}

	for _, task := range tasks {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed task %s: %v", task.TaskID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/schedule", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/schedule status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response ScheduleResponseDTO
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode schedule response: %v", err)
	}

	if response.Summary.Total != 5 || response.Summary.Overdue != 1 || response.Summary.DueSoon != 1 || response.Summary.Stale != 1 || response.Summary.Done != 1 {
		t.Fatalf("unexpected schedule summary: %+v", response.Summary)
	}
	if len(response.Items) != 5 {
		t.Fatalf("items length = %d, want 5", len(response.Items))
	}
	if response.Items[0].DemandID != "DEMAND-OVERDUE" || response.Items[0].RiskLevel != "overdue" {
		t.Fatalf("first item should be overdue demand, got %+v", response.Items[0])
	}

	var overdue ScheduleItemDTO
	var unscheduled ScheduleItemDTO
	for _, item := range response.Items {
		if item.DemandID == "DEMAND-OVERDUE" {
			overdue = item
		}
		if item.DemandID == "DEMAND-UNSCHEDULED" {
			unscheduled = item
		}
	}
	if !overdue.Scheduled {
		t.Fatalf("overdue demand should be marked scheduled: %+v", overdue)
	}
	if overdue.SubtaskTotal != 2 || overdue.SubtaskDone != 1 || overdue.SubtaskActive != 1 {
		t.Fatalf("overdue subtask stats mismatch: %+v", overdue)
	}
	if overdue.Department != "Product" {
		t.Fatalf("overdue department = %q, want Product", overdue.Department)
	}
	if unscheduled.Scheduled {
		t.Fatalf("unscheduled demand should not be marked scheduled: %+v", unscheduled)
	}
}

func TestGetExecutionTasksBuildsEvidenceObservability(t *testing.T) {
	setupServerTestDB(t)

	token := superAdminToken(t, "observer@westwell-lab.com", "Observer", []string{"dashboard:read"})
	cfg := &config.Config{Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"}}
	srv := NewServer(cfg, "")

	now := time.Now()
	staleUpdate := now.AddDate(0, 0, -5)
	doneAt := now.AddDate(0, 0, -1)

	users := []userdb.User{
		{Username: "alice.exec@westwell-lab.com", Email: "alice.exec@westwell-lab.com", Name: "Alice", Department: "Engineering"},
		{Username: "bob.exec@westwell-lab.com", Email: "bob.exec@westwell-lab.com", Name: "Bob", Department: "QA"},
	}
	for _, user := range users {
		if err := db.DB.Create(&user).Error; err != nil {
			t.Fatalf("seed user %s: %v", user.Username, err)
		}
	}

	tasks := []db.TaskTelemetry{
		{
			TaskID:        "DEMAND-EXEC",
			Title:         "Execution parent demand",
			Assignee:      "Alice",
			Status:        "progress",
			IssueType:     "demand",
			TaskGroupID:   "group-exec",
			TaskCreatedAt: now.AddDate(0, 0, -7),
			LastUpdate:    now,
		},
		{
			TaskID:        "DEMAND-DONE",
			Title:         "Jira parent demand completed after reassignment",
			Assignee:      "Bob",
			Status:        "done",
			IssueType:     "demand",
			TaskGroupID:   "group-jira-done",
			TaskCreatedAt: now.AddDate(0, 0, -7),
			LastUpdate:    doneAt,
		},
		{
			TaskID:        "JIRA-101",
			Title:         "Bound task with commit",
			Repo:          "platform-core",
			Assignee:      "Alice",
			Branch:        "feat/JIRA-101",
			Status:        "progress",
			IssueType:     "task",
			TaskGroupID:   "group-exec",
			TaskCreatedAt: now.AddDate(0, 0, -2),
			LastUpdate:    now,
		},
		{
			TaskID:        "JIRA-102",
			Title:         "Done without evidence",
			Assignee:      "Bob",
			Status:        "done",
			IssueType:     "task",
			CompletedAt:   &doneAt,
			TaskCreatedAt: now.AddDate(0, 0, -3),
			LastUpdate:    doneAt,
		},
		{
			TaskID:        "JIRA-103",
			Title:         "Merged but Jira still active",
			Assignee:      "Alice",
			Branch:        "feat/JIRA-103",
			Status:        "review",
			IssueType:     "task",
			TaskCreatedAt: now.AddDate(0, 0, -4),
			LastUpdate:    now,
		},
		{
			TaskID:        "JIRA-104",
			Title:         "Stale orphan task",
			Assignee:      "Bob",
			Branch:        "feat/JIRA-104",
			Status:        "progress",
			IssueType:     "task",
			TaskCreatedAt: now.AddDate(0, 0, -8),
			LastUpdate:    staleUpdate,
		},
		{
			TaskID:        "JIRA-106",
			Title:         "Execution task follows completed Jira parent",
			Repo:          "platform-core",
			Assignee:      "Alice",
			Branch:        "feat/JIRA-106",
			Status:        "progress",
			IssueType:     "task",
			TaskGroupID:   "group-jira-done",
			TaskCreatedAt: now.AddDate(0, 0, -2),
			LastUpdate:    now,
		},
		{
			TaskID:        "BUG-105",
			Title:         "Bug with evidence",
			Assignee:      "Bob",
			Branch:        "fix/BUG-105",
			Status:        "progress",
			IssueType:     "bug",
			TaskCreatedAt: now.AddDate(0, 0, -1),
			LastUpdate:    now,
		},
	}
	for _, task := range tasks {
		if err := db.DB.Create(&task).Error; err != nil {
			t.Fatalf("seed task %s: %v", task.TaskID, err)
		}
	}

	logs := []db.GitCommitLog{
		{TaskID: "JIRA-101", Repo: "platform-core", Branch: "feat/JIRA-101", CommitID: "abc123", Message: "JIRA-101 implementation", Author: "Alice", Action: "git_push", CreatedAt: now},
		{TaskID: "JIRA-103", Repo: "platform-core", Branch: "feat/JIRA-103", MrIID: 42, MrURL: "https://gitlab/mr/42", Message: "JIRA-103 MR", Author: "Alice", Action: "mr_merge", CreatedAt: now},
		{TaskID: "JIRA-106", Repo: "platform-core", Branch: "feat/JIRA-106", CommitID: "fed789", Message: "JIRA-106 implementation", Author: "Alice", Action: "git_push", CreatedAt: now},
		{TaskID: "BUG-105", Repo: "platform-core", Branch: "fix/BUG-105", CommitID: "def456", Message: "BUG-105 fix", Author: "Bob", Action: "git_push", CreatedAt: now},
	}
	for _, logRow := range logs {
		if err := db.DB.Create(&logRow).Error; err != nil {
			t.Fatalf("seed git log for %s: %v", logRow.TaskID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/execution/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/execution/tasks status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var response ExecutionTasksResponseDTO
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode execution response: %v", err)
	}

	if response.Summary.Total != 6 || response.Summary.Bound != 3 || response.Summary.Orphan != 3 || response.Summary.HighRisk != 2 {
		t.Fatalf("unexpected execution summary: %+v", response.Summary)
	}

	byID := make(map[string]ExecutionTaskItemDTO)
	for _, item := range response.Items {
		byID[item.TaskID] = item
	}

	if byID["JIRA-101"].ParentDemandID != "DEMAND-EXEC" || byID["JIRA-101"].EvidenceScore == 0 {
		t.Fatalf("bound task evidence mismatch: %+v", byID["JIRA-101"])
	}
	if byID["JIRA-102"].RiskLabel != "完成无证据" || byID["JIRA-102"].RiskLevel != "high" {
		t.Fatalf("done without evidence risk mismatch: %+v", byID["JIRA-102"])
	}
	if byID["JIRA-103"].RiskLabel != "状态不一致" || byID["JIRA-103"].ResultState != "merged_waiting_jira" {
		t.Fatalf("merged mismatch risk mismatch: %+v", byID["JIRA-103"])
	}
	if byID["JIRA-104"].RiskLabel != "推进停滞" {
		t.Fatalf("stale task risk mismatch: %+v", byID["JIRA-104"])
	}
	if byID["JIRA-106"].Assignee != "Bob" || byID["JIRA-106"].ExecutionAssignee != "Alice" || byID["JIRA-106"].JiraAssignee != "Bob" {
		t.Fatalf("Jira parent assignee should drive execution display while preserving execution assignee: %+v", byID["JIRA-106"])
	}
	if byID["JIRA-106"].Status != "done" || byID["JIRA-106"].JiraStatus != "done" || byID["JIRA-106"].ResultState != "jira_done" || byID["JIRA-106"].RiskLevel != "done" {
		t.Fatalf("Jira parent completion should drive execution status/result: %+v", byID["JIRA-106"])
	}
	if byID["BUG-105"].ParentIssueType != deliveryplanning.WorkItemBug || byID["BUG-105"].EvidenceScore == 0 {
		t.Fatalf("evidence-backed Bug must be projected as a real execution row: %+v", byID["BUG-105"])
	}

	filterReq := httptest.NewRequest(http.MethodGet, "/api/execution/tasks?assignee=Bob", nil)
	filterReq.Header.Set("Authorization", "Bearer "+token)
	filterRR := httptest.NewRecorder()
	srv.mux.ServeHTTP(filterRR, filterReq)
	if filterRR.Code != http.StatusOK {
		t.Fatalf("GET /api/execution/tasks?assignee=Bob status = %d, body = %s", filterRR.Code, filterRR.Body.String())
	}
	var filteredResponse ExecutionTasksResponseDTO
	if err := json.NewDecoder(filterRR.Body).Decode(&filteredResponse); err != nil {
		t.Fatalf("decode filtered execution response: %v", err)
	}
	filteredByID := make(map[string]ExecutionTaskItemDTO)
	for _, item := range filteredResponse.Items {
		filteredByID[item.TaskID] = item
	}
	if _, ok := filteredByID["JIRA-106"]; !ok {
		t.Fatalf("effective Jira assignee filter should include JIRA-106 after parent reassignment, got %+v", filteredResponse.Items)
	}
}

func seedLocalUserWithGroup(t *testing.T, username, name, groupName, password string) userdb.User {
	t.Helper()
	passwordHash := ""
	if password != "" {
		var err error
		passwordHash, err = deriveLocalPasswordHash(password)
		if err != nil {
			t.Fatalf("Failed to derive local test password hash: %v", err)
		}
	}

	mockUser := userdb.User{
		Username:          username,
		Email:             username,
		Name:              name,
		Department:        "Offline Ops",
		LocalPasswordHash: passwordHash,
	}
	if err := db.DB.Create(&mockUser).Error; err != nil {
		t.Fatalf("Failed to seed local user: %v", err)
	}

	var group userdb.UserGroup
	if err := db.DB.Where("name = ?", groupName).First(&group).Error; err != nil {
		t.Fatalf("Failed to query %s group: %v", groupName, err)
	}

	if err := db.DB.Create(&userdb.UserGroupMembership{
		UserID:      mockUser.ID,
		UserGroupID: group.ID,
		Scope:       "global",
		ScopeID:     "",
	}).Error; err != nil {
		t.Fatalf("Failed to bind local user membership: %v", err)
	}

	return mockUser
}

func useTempKanbanFile(t *testing.T) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "well-ambient-kanban-test")
	if err != nil {
		t.Fatalf("Failed to create temp kanban dir: %v", err)
	}

	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = tmpDir + "/task_status.md"
	t.Cleanup(func() {
		kanban.KanbanFilePath = oldPath
		os.RemoveAll(tmpDir)
	})
}

func TestGetCurrentUserReturnsDepartment(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "eddie@westwell-lab.com", "Eddie", []string{"dashboard:read"})
	if err := db.DB.Model(&userdb.User{}).
		Where("username = ?", "eddie@westwell-lab.com").
		Update("department", "AI Platform").Error; err != nil {
		t.Fatalf("Failed to update test user department: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9094, Host: "127.0.0.1"}}, "")
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/me failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode /api/me response: %v", err)
	}
	user, ok := res["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("/api/me response missing user object: %+v", res)
	}
	if user["department"] != "AI Platform" {
		t.Fatalf("department = %v, want %q", user["department"], "AI Platform")
	}
}

func TestGetCurrentUserBackfillsDepartmentFromJWT(t *testing.T) {
	setupServerTestDB(t)
	_ = superAdminToken(t, "profile@westwell-lab.com", "Profile User", []string{"dashboard:read"})
	if err := db.DB.Model(&userdb.User{}).
		Where("username = ?", "profile@westwell-lab.com").
		Update("department", "未分配").Error; err != nil {
		t.Fatalf("Failed to seed placeholder department: %v", err)
	}
	token, err := GenerateJWT(
		"profile@westwell-lab.com",
		"Profile User",
		"mock_wellos_token",
		"https://example.com/avatar.png",
		[]string{"super_admin"},
		[]string{"dashboard:read"},
		"User Experience Center",
	)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9097, Host: "127.0.0.1"}}, "")
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/me failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var user userdb.User
	if err := db.DB.Where("username = ?", "profile@westwell-lab.com").First(&user).Error; err != nil {
		t.Fatalf("User not found in DB: %v", err)
	}
	if user.Department != "User Experience Center" {
		t.Fatalf("User Department = %q, want %q", user.Department, "User Experience Center")
	}
	if user.Avatar != "https://example.com/avatar.png" {
		t.Fatalf("User Avatar = %q, want avatar from JWT", user.Avatar)
	}
}

func TestGetCurrentUserRefreshesDepartmentFromJWT(t *testing.T) {
	setupServerTestDB(t)
	_ = superAdminToken(t, "refresh@westwell-lab.com", "Refresh User", []string{"dashboard:read"})
	if err := db.DB.Model(&userdb.User{}).
		Where("username = ?", "refresh@westwell-lab.com").
		Update("department", "Legacy Ops").Error; err != nil {
		t.Fatalf("Failed to seed legacy department: %v", err)
	}
	token, err := GenerateJWT(
		"refresh@westwell-lab.com",
		"Refresh User",
		"mock_wellos_token",
		"",
		[]string{"super_admin"},
		[]string{"dashboard:read"},
		"AI Platform",
	)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9098, Host: "127.0.0.1"}}, "")
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/me failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var user userdb.User
	if err := db.DB.Where("username = ?", "refresh@westwell-lab.com").First(&user).Error; err != nil {
		t.Fatalf("User not found in DB: %v", err)
	}
	if user.Department != "AI Platform" {
		t.Fatalf("User Department = %q, want refreshed department", user.Department)
	}
}

func TestLoginFallsBackForExistingLocalUserWhenWellOSUnavailable(t *testing.T) {
	setupServerTestDB(t)
	seedLocalUserWithGroup(t, "offline@westwell-lab.com", "Offline User", "member", "cached-password")

	oldDoer := wellOSLoginDoer
	wellOSLoginDoer = func(client *http.Client, username, password string) (*http.Response, error) {
		return nil, io.EOF
	}
	t.Cleanup(func() { wellOSLoginDoer = oldDoer })

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9099, Host: "127.0.0.1"}}, "")
	body := bytes.NewBufferString(`{"username":"offline@westwell-lab.com","password":"cached-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("degraded login failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode degraded login response: %v", err)
	}
	if res["degraded"] != true {
		t.Fatalf("degraded flag = %v, want true", res["degraded"])
	}
	token, _ := res["token"].(string)
	if token == "" {
		t.Fatalf("degraded login did not return token: %+v", res)
	}
	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("degraded token invalid: %v", err)
	}
	if claims.UserID != "offline@westwell-lab.com" {
		t.Fatalf("claims.UserID = %q, want offline user", claims.UserID)
	}
	if claims.WellOSToken != "wellos-maintenance-fallback" {
		t.Fatalf("claims.WellOSToken = %q, want maintenance marker", claims.WellOSToken)
	}
}

func TestLoginDoesNotFallbackForUnknownUserWhenWellOSUnavailable(t *testing.T) {
	setupServerTestDB(t)

	oldDoer := wellOSLoginDoer
	wellOSLoginDoer = func(client *http.Client, username, password string) (*http.Response, error) {
		return nil, fmt.Errorf("tls: failed to verify certificate: x509: certificate signed by unknown authority")
	}
	t.Cleanup(func() { wellOSLoginDoer = oldDoer })

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9100, Host: "127.0.0.1"}}, "")
	body := bytes.NewBufferString(`{"username":"new-user@westwell-lab.com","password":"anything"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("unknown user fallback status = %v, want %v body %s", rr.Code, http.StatusBadGateway, rr.Body.String())
	}
	var response struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode unknown user fallback response: %v body %s", err, rr.Body.String())
	}
	if response.Code != "wellos_unreachable" {
		t.Fatalf("fallback code = %q, want wellos_unreachable", response.Code)
	}
	if strings.Contains(strings.ToLower(response.Message), "maintenance") {
		t.Fatalf("transport failure must not be reported as maintenance: %q", response.Message)
	}
	if !strings.Contains(response.Message, "no verified local session fallback") {
		t.Fatalf("unexpected unknown user fallback response: %q", response.Message)
	}
}

func TestLoginDoesNotFallbackWhenLocalPasswordDoesNotMatch(t *testing.T) {
	setupServerTestDB(t)
	seedLocalUserWithGroup(t, "offline@westwell-lab.com", "Offline User", "member", "cached-password")

	oldDoer := wellOSLoginDoer
	wellOSLoginDoer = func(client *http.Client, username, password string) (*http.Response, error) {
		return nil, io.EOF
	}
	t.Cleanup(func() { wellOSLoginDoer = oldDoer })

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9101, Host: "127.0.0.1"}}, "")
	body := bytes.NewBufferString(`{"username":"offline@westwell-lab.com","password":"wrong-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password fallback status = %v, want %v body %s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "local credential verification failed") {
		t.Fatalf("unexpected wrong password fallback response: %s", rr.Body.String())
	}
}

func TestLoginUsesDevAuthForLoopbackWhenEnabled(t *testing.T) {
	setupServerTestDB(t)
	t.Setenv("WELL_AMBIENT_DEV_AUTH", "1")

	oldDoer := wellOSLoginDoer
	wellOSLoginDoer = func(client *http.Client, username, password string) (*http.Response, error) {
		return nil, io.EOF
	}
	t.Cleanup(func() { wellOSLoginDoer = oldDoer })

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9102, Host: "127.0.0.1"}}, "")
	body := bytes.NewBufferString(`{"username":"dev@westwell-lab.com","password":"anything"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	req.RemoteAddr = "127.0.0.1:54321"
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("dev auth login failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode dev auth response: %v", err)
	}
	if res["degraded_reason"] != "local_dev_auth" {
		t.Fatalf("degraded_reason = %v, want local_dev_auth", res["degraded_reason"])
	}
	token, _ := res["token"].(string)
	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("dev auth token invalid: %v", err)
	}
	if claims.WellOSToken != "dev-local-auth" {
		t.Fatalf("claims.WellOSToken = %q, want dev-local-auth", claims.WellOSToken)
	}
}

func TestLoginDoesNotUseDevAuthForNonLoopback(t *testing.T) {
	setupServerTestDB(t)
	t.Setenv("WELL_AMBIENT_DEV_AUTH", "1")

	oldDoer := wellOSLoginDoer
	wellOSLoginDoer = func(client *http.Client, username, password string) (*http.Response, error) {
		return nil, io.EOF
	}
	t.Cleanup(func() { wellOSLoginDoer = oldDoer })

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9103, Host: "127.0.0.1"}}, "")
	body := bytes.NewBufferString(`{"username":"remote@westwell-lab.com","password":"anything"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	req.RemoteAddr = "10.1.2.3:54321"
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("non-loopback dev auth status = %v, want %v body %s", rr.Code, http.StatusBadGateway, rr.Body.String())
	}
}

func TestCreateDemandUsesAuthenticatedDepartmentFallback(t *testing.T) {
	setupServerTestDB(t)
	useTempKanbanFile(t)
	_ = superAdminToken(t, "pm@westwell-lab.com", "PM", []string{"demands:write"})
	if err := db.DB.Model(&userdb.User{}).
		Where("username = ?", "pm@westwell-lab.com").
		Update("department", "未分配").Error; err != nil {
		t.Fatalf("Failed to seed placeholder department: %v", err)
	}

	token, err := GenerateJWT(
		"pm@westwell-lab.com",
		"PM",
		"mock_wellos_token",
		"",
		[]string{"super_admin"},
		[]string{"demands:write"},
		"Product Ops",
	)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9095, Host: "127.0.0.1"}}, "")
	payload := map[string]string{
		"title":       "Department claim demand",
		"description": "Verify creator department from authenticated profile.",
		"assignee":    "Bob",
		"due_date":    "2026-06-30",
		"repo":        "platform-core",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/demands", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/demands failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode demand response: %v", err)
	}
	taskID, _ := res["task_id"].(string)

	var task db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		t.Fatalf("Demand task not found in DB: %v", err)
	}
	if task.CreatorDept != "Product Ops" {
		t.Fatalf("CreatorDept = %q, want %q", task.CreatorDept, "Product Ops")
	}

	var user userdb.User
	if err := db.DB.Where("username = ?", "pm@westwell-lab.com").First(&user).Error; err != nil {
		t.Fatalf("User not found in DB: %v", err)
	}
	if user.Department != "Product Ops" {
		t.Fatalf("User Department = %q, want %q", user.Department, "Product Ops")
	}
}

func TestGetDemandOptionsBuildsFormCandidates(t *testing.T) {
	setupServerTestDB(t)

	token := superAdminToken(t, "pm-options@westwell-lab.com", "PM Options", []string{"demands:write"})
	extraUsers := []userdb.User{
		{Username: "alice.options@westwell-lab.com", Email: "alice.options@westwell-lab.com", Name: "Alice Options", Department: "Product"},
		{Username: "bob.options@westwell-lab.com", Email: "bob.options@westwell-lab.com", Name: "Bob Options", Department: "Engineering"},
	}
	for _, u := range extraUsers {
		if err := db.DB.Create(&u).Error; err != nil {
			t.Fatalf("seed user %s: %v", u.Username, err)
		}
	}
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID:     "DEMAND-OPTIONS",
		Title:      "Existing options source",
		Repo:       "legacy-web",
		Assignee:   "Task Owner",
		Status:     "backlog",
		IssueType:  "demand",
		LastUpdate: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed task telemetry: %v", err)
	}
	if err := db.DB.Create(&db.ProjectConfig{
		ProjectKey:   "AMB",
		ProjectName:  "Ambient Control",
		BasePriority: "P1",
		ProjectPhase: "交付",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed project config: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9097, Host: "127.0.0.1"},
		GitLab: config.GitLabConfig{Repos: []config.RepoMapping{
			{Name: "platform-core", Path: "group/platform-core", ProjectID: "42"},
		}},
		Jira: config.JiraConfig{
			SyncProjects: []string{"CFG"},
			SyncUsers:    []string{"Jira Owner"},
			CustomJQL:    `project in (OPS, "APP-X") AND assignee in ("JQL Owner", middle.q)`,
		},
	}
	srv := NewServer(cfg, "")

	req := httptest.NewRequest(http.MethodGet, "/api/demands/options", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/demands/options failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var response DemandOptionsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode demand options response: %v", err)
	}

	for _, want := range []string{"Jira Owner"} {
		if !stringSliceContains(response.Assignees, want) {
			t.Fatalf("assignees missing %q: %+v", want, response.Assignees)
		}
	}
	for _, notWant := range []string{"Alice Options", "Bob Options", "Task Owner", "JQL Owner", "middle.q"} {
		if stringSliceContains(response.Assignees, notWant) {
			t.Fatalf("non-core assignee should not be offered %q: %+v", notWant, response.Assignees)
		}
	}
	for _, want := range []string{"AMB", "Ambient Control", "CFG", "OPS", "APP-X"} {
		if !stringSliceContains(response.Projects, want) {
			t.Fatalf("projects missing %q: %+v", want, response.Projects)
		}
	}
	for _, notWant := range []string{"platform-core", "legacy-web", "group/platform-core", "42"} {
		if stringSliceContains(response.Projects, notWant) {
			t.Fatalf("projects should not include repo candidate %q: %+v", notWant, response.Projects)
		}
	}
}

func TestGetDemandOptionsUsesCustomJQLAssigneesAsFallback(t *testing.T) {
	setupServerTestDB(t)

	token := superAdminToken(t, "pm-jql-fallback@westwell-lab.com", "PM JQL Fallback", []string{"demands:write"})
	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9107, Host: "127.0.0.1"},
		Jira: config.JiraConfig{
			CustomJQL: `project = OPS AND assignee in ("JQL Owner", middle.q)`,
		},
	}, "")

	req := httptest.NewRequest(http.MethodGet, "/api/demands/options", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/demands/options failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var response DemandOptionsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode demand options response: %v", err)
	}
	for _, want := range []string{"JQL Owner", "middle.q"} {
		if !stringSliceContains(response.Assignees, want) {
			t.Fatalf("fallback assignees missing %q: %+v", want, response.Assignees)
		}
	}
}

func TestGetDemandOptionsAllowsAuthenticatedDemandReader(t *testing.T) {
	setupServerTestDB(t)

	token, err := GenerateJWT("reader@westwell-lab.com", "Demand Reader", "mock_wellos_token", "", []string{"member"}, []string{"demands:read"})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if err := db.DB.Create(&userdb.User{
		Username: "reader-candidate@westwell-lab.com",
		Email:    "reader-candidate@westwell-lab.com",
		Name:     "Reader Candidate",
	}).Error; err != nil {
		t.Fatalf("seed candidate user: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9108, Host: "127.0.0.1"}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/demands/options", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/demands/options status = %v body %s", rr.Code, rr.Body.String())
	}

	var response DemandOptionsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode demand options response: %v", err)
	}
	if !stringSliceContains(response.Assignees, "Reader Candidate") {
		t.Fatalf("expected authenticated reader to receive assignee candidates, got %+v", response.Assignees)
	}
}

func TestGetJiraLinkConfigDoesNotRequireConfigRead(t *testing.T) {
	setupServerTestDB(t)

	token, err := GenerateJWT("demand-reader@westwell-lab.com", "Demand Reader", "mock_wellos_token", "", []string{"member"}, []string{"demands:read"})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9109, Host: "127.0.0.1"},
		Jira: config.JiraConfig{
			Enabled: true,
			BaseURL: "https://jira.example.com/",
		},
	}, "")

	req := httptest.NewRequest(http.MethodGet, "/api/jira/link-config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/jira/link-config status = %v body %s", rr.Code, rr.Body.String())
	}

	var response jiraLinkConfigResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode Jira link config response: %v", err)
	}
	if !response.Enabled || response.BaseURL != "https://jira.example.com" {
		t.Fatalf("unexpected Jira link config: %+v", response)
	}
}

func TestReassignDemandUpdatesDBAndKanban(t *testing.T) {
	setupServerTestDB(t)
	useTempKanbanFile(t)
	token := superAdminToken(t, "pm-reassign@westwell-lab.com", "PM Reassign", []string{"demands:write"})
	now := time.Now()

	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-REASSIGN",
		Title:         "Reassignable demand",
		Repo:          "platform-core",
		Assignee:      "Alice",
		Branch:        "-",
		LastCommit:    "-",
		Status:        "backlog",
		IssueType:     "demand",
		TaskCreatedAt: now,
		LastUpdate:    now,
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	if err := kanban.SyncTaskToKanban(&demand); err != nil {
		t.Fatalf("seed kanban: %v", err)
	}

	payload := map[string]string{
		"task_id":  "DEMAND-REASSIGN",
		"assignee": "Bob",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/demands/reassign", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9104, Host: "127.0.0.1"}}, "")
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/demands/reassign failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var updated db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "DEMAND-REASSIGN").First(&updated).Error; err != nil {
		t.Fatalf("updated demand not found: %v", err)
	}
	if updated.Assignee != "Bob" {
		t.Fatalf("Assignee = %q, want Bob", updated.Assignee)
	}

	kanbanContent, err := os.ReadFile(kanban.KanbanFilePath)
	if err != nil {
		t.Fatalf("read kanban file: %v", err)
	}
	if !strings.Contains(string(kanbanContent), "| DEMAND-REASSIGN | Reassignable demand | platform-core | Bob | - | - |") {
		t.Fatalf("kanban file did not include reassigned owner:\n%s", string(kanbanContent))
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestCreateDemandUsesRequestDepartmentFallback(t *testing.T) {
	setupServerTestDB(t)
	useTempKanbanFile(t)
	token := superAdminToken(t, "requester@westwell-lab.com", "Requester", []string{"demands:write"})

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9096, Host: "127.0.0.1"}}, "")
	payload := map[string]string{
		"title":        "Department body demand",
		"description":  "Verify creator department from request fallback.",
		"assignee":     "Bob",
		"due_date":     "2026-07-01",
		"repo":         "platform-core",
		"creator_dept": "Delivery Office",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/demands", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/demands failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode demand response: %v", err)
	}
	taskID, _ := res["task_id"].(string)

	var task db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		t.Fatalf("Demand task not found in DB: %v", err)
	}
	if task.CreatorDept != "Delivery Office" {
		t.Fatalf("CreatorDept = %q, want %q", task.CreatorDept, "Delivery Office")
	}

	var user userdb.User
	if err := db.DB.Where("username = ?", "requester@westwell-lab.com").First(&user).Error; err != nil {
		t.Fatalf("User not found in DB: %v", err)
	}
	if user.Department != "Delivery Office" {
		t.Fatalf("User Department = %q, want %q", user.Department, "Delivery Office")
	}
}

func TestImportTasksLinksDemand(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "Eddie", "Eddie", []string{"demands:write"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9091, Host: "127.0.0.1"}}, "")

	tmpDir, err := os.MkdirTemp("", "import-tasks-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = tmpDir + "/task_status.md"
	defer func() { kanban.KanbanFilePath = oldPath }()

	now := time.Now()
	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-900",
		Title:         "Demand to link",
		Repo:          "platform-core",
		Assignee:      "Bob",
		Status:        "backlog",
		IssueType:     "demand",
		TaskCreatedAt: now,
		LastUpdate:    now,
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("Failed to seed demand: %v", err)
	}

	payload := map[string]interface{}{
		"task_group_id": "group-import-001",
		"demand_id":     "DEMAND-900",
		"tasks": []map[string]interface{}{
			{
				"id":          "task-901",
				"repo":        "platform-core",
				"title":       "Build linked shadow task",
				"assignee":    "Bob (推荐)",
				"period_days": 3,
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/tasks/import", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/tasks/import failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var linkedDemand db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "DEMAND-900").First(&linkedDemand).Error; err != nil {
		t.Fatalf("Linked demand not found: %v", err)
	}
	if linkedDemand.TaskGroupID != "group-import-001" {
		t.Fatalf("Demand TaskGroupID = %q, want %q", linkedDemand.TaskGroupID, "group-import-001")
	}

	var shadowTask db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "task-901").First(&shadowTask).Error; err != nil {
		t.Fatalf("Imported shadow task not found: %v", err)
	}
	if shadowTask.TaskGroupID != "group-import-001" || shadowTask.Assignee != "Bob" || shadowTask.Status != "backlog" {
		t.Fatalf("Imported shadow task fields mismatch: %+v", shadowTask)
	}

	kanbanContent, err := os.ReadFile(kanban.KanbanFilePath)
	if err != nil {
		t.Fatalf("Expected kanban file to be written: %v", err)
	}
	if !strings.Contains(string(kanbanContent), "DEMAND-900") || !strings.Contains(string(kanbanContent), "task-901") {
		t.Fatalf("Kanban file does not include linked demand and imported task:\n%s", string(kanbanContent))
	}
}

func TestImportTasksReusesLinkedDemandTaskGroup(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "Eddie", "Eddie", []string{"demands:write"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9093, Host: "127.0.0.1"}}, "")

	tmpDir, err := os.MkdirTemp("", "import-tasks-reuse-group-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = tmpDir + "/task_status.md"
	defer func() { kanban.KanbanFilePath = oldPath }()

	now := time.Now()
	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-901",
		Title:         "Demand with existing brain group",
		Repo:          "platform-core",
		Assignee:      "Bob",
		Status:        "progress",
		IssueType:     "demand",
		TaskCreatedAt: now,
		LastUpdate:    now,
		TaskGroupID:   "group-existing-901",
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("Failed to seed demand: %v", err)
	}

	payload := map[string]interface{}{
		"demand_id": "DEMAND-901",
		"tasks": []map[string]interface{}{
			{
				"id":       "task-902",
				"repo":     "platform-core",
				"title":    "Build another linked shadow task",
				"assignee": "Bob",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/tasks/import", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/tasks/import failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var shadowTask db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "task-902").First(&shadowTask).Error; err != nil {
		t.Fatalf("Imported shadow task not found: %v", err)
	}
	if shadowTask.TaskGroupID != "group-existing-901" {
		t.Fatalf("Imported shadow task TaskGroupID = %q, want %q", shadowTask.TaskGroupID, "group-existing-901")
	}
}

func TestImportTasksCreatesStableBrainGroupForDemand(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "Eddie", "Eddie", []string{"demands:write"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9098, Host: "127.0.0.1"}}, "")

	tmpDir, err := os.MkdirTemp("", "import-tasks-stable-group-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	oldPath := kanban.KanbanFilePath
	kanban.KanbanFilePath = tmpDir + "/task_status.md"
	defer func() { kanban.KanbanFilePath = oldPath }()

	now := time.Now()
	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-777",
		Title:         "Demand without existing brain group",
		Repo:          "-",
		Assignee:      "Bob",
		Status:        "backlog",
		IssueType:     "demand",
		TaskCreatedAt: now,
		LastUpdate:    now,
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("Failed to seed demand: %v", err)
	}

	payload := map[string]interface{}{
		"demand_id": "DEMAND-777",
		"tasks": []map[string]interface{}{
			{
				"id":       "task-777",
				"repo":     "platform-core",
				"title":    "Stable linked shadow task",
				"assignee": "Bob",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/tasks/import", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/tasks/import failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var linkedDemand db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "DEMAND-777").First(&linkedDemand).Error; err != nil {
		t.Fatalf("Linked demand not found: %v", err)
	}
	if linkedDemand.TaskGroupID != "brain-demand-777" {
		t.Fatalf("Demand TaskGroupID = %q, want %q", linkedDemand.TaskGroupID, "brain-demand-777")
	}

	var shadowTask db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "task-777").First(&shadowTask).Error; err != nil {
		t.Fatalf("Imported shadow task not found: %v", err)
	}
	if shadowTask.TaskGroupID != "brain-demand-777" {
		t.Fatalf("Shadow TaskGroupID = %q, want %q", shadowTask.TaskGroupID, "brain-demand-777")
	}
}

func TestImportTasksArchivesEstimates(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "Eddie", "Eddie", []string{"demands:write"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9099, Host: "127.0.0.1"}}, "")
	useTempKanbanFile(t)

	now := time.Now()
	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-880",
		Title:         "Demand with estimates",
		Repo:          "-",
		Assignee:      "Bob",
		Status:        "backlog",
		IssueType:     "demand",
		TaskCreatedAt: now,
		LastUpdate:    now,
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("Failed to seed demand: %v", err)
	}

	payload := map[string]interface{}{
		"demand_id":   "DEMAND-880",
		"input_text":  "需要建设 AI 估算归档能力",
		"mappedRepos": []string{"platform-core"},
		"analysis": map[string]interface{}{
			"completeness_score":      90,
			"overall_estimated_days":  5.5,
			"overall_estimated_hours": 44,
			"overall_difficulty":      "High",
			"estimate_basis":          "后端归档和前端展示可并行，后端为关键路径",
			"missing_info":            []string{},
			"risks":                   []string{},
			"dependencies":            []string{},
			"acceptance_criteria":     []string{},
			"schedule_notes":          []string{},
			"meeting_questions":       []string{},
			"confidence":              0.82,
		},
		"tasks": []map[string]interface{}{
			{
				"id":              "task-880",
				"repo":            "platform-core",
				"title":           "Persist estimate archive",
				"assignee":        "Bob",
				"priority":        "High",
				"complexity":      "High",
				"difficulty":      "High",
				"estimated_days":  3.5,
				"estimated_hours": 28,
				"estimate_basis":  "新增归档表和导入链路",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/tasks/import", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/tasks/import failed: got %v body %s", rr.Code, rr.Body.String())
	}

	var archive db.DeconstructArchive
	if err := db.DB.Where("demand_id = ?", "DEMAND-880").First(&archive).Error; err != nil {
		t.Fatalf("Deconstruct archive not found: %v", err)
	}
	if archive.OverallEstimateDays != 5.5 || archive.OverallDifficulty != "High" || archive.TaskGroupID != "brain-demand-880" {
		t.Fatalf("Archive fields mismatch: %+v", archive)
	}
	if !strings.Contains(archive.InputText, "估算归档") || !strings.Contains(archive.TasksJSON, "Persist estimate archive") {
		t.Fatalf("Archive did not preserve input/tasks snapshot: %+v", archive)
	}

	var shadowTask db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "task-880").First(&shadowTask).Error; err != nil {
		t.Fatalf("Imported shadow task not found: %v", err)
	}
	if shadowTask.EstimateDays != 3.5 || shadowTask.EstimateHours != 28 || shadowTask.Difficulty != "High" {
		t.Fatalf("Shadow task estimate fields mismatch: %+v", shadowTask)
	}
	if shadowTask.EstimateSource != "ai_deconstruct" || shadowTask.EstimateArchiveID != archive.ID {
		t.Fatalf("Shadow task archive linkage mismatch: %+v archive=%d", shadowTask, archive.ID)
	}
}

func TestImportTasksRejectsUnknownDemand(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "Eddie", "Eddie", []string{"demands:write"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9092, Host: "127.0.0.1"}}, "")

	payload := map[string]interface{}{
		"task_group_id": "group-import-404",
		"demand_id":     "DEMAND-404",
		"tasks": []map[string]interface{}{
			{
				"id":       "task-404",
				"repo":     "platform-core",
				"title":    "Should rollback",
				"assignee": "Bob",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/tasks/import", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("POST /api/tasks/import status = %v, want %v body %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}

	var count int64
	db.DB.Model(&db.TaskTelemetry{}).Where("task_id = ?", "task-404").Count(&count)
	if count != 0 {
		t.Fatalf("Expected transaction rollback for imported task, found %d records", count)
	}
}

func TestDemandAndKPILogic(t *testing.T) {
	setupServerTestDB(t)

	// Setup a clean memory DB context (TestServerEndpoints might have set it up, but let's make sure it has groups/perms)
	var count int64
	db.DB.Model(&userdb.UserGroup{}).Count(&count)
	if count == 0 {
		userdb.InitializeSeeds(db.DB)
	}

	// Create test users
	userBob := userdb.User{
		Username:   "bob@westwell-lab.com",
		Email:      "bob@westwell-lab.com",
		Name:       "Bob",
		Department: "R&D",
	}
	db.DB.Create(&userBob)

	// Eddie is super admin (already created in TestServerEndpoints, let's query/create)
	var userEddie userdb.User
	if err := db.DB.Where("username = ?", "Eddie").First(&userEddie).Error; err != nil {
		userEddie = userdb.User{
			Username:   "Eddie",
			Email:      "eddie@westwell-lab.com",
			Name:       "Eddie",
			Department: "Management",
		}
		db.DB.Create(&userEddie)
	} else {
		userEddie.Department = "Management"
		db.DB.Save(&userEddie)
	}

	// Bind group and perms to Eddie
	var superGroup userdb.UserGroup
	db.DB.Where("name = ?", "super_admin").First(&superGroup)
	var memberMembership userdb.UserGroupMembership
	if err := db.DB.Where("user_id = ? AND user_group_id = ?", userEddie.ID, superGroup.ID).First(&memberMembership).Error; err != nil {
		db.DB.Create(&userdb.UserGroupMembership{
			UserID:      userEddie.ID,
			UserGroupID: superGroup.ID,
			Scope:       "global",
		})
	}

	// Generate super_admin token
	token, err := GenerateJWT("Eddie", "Eddie", "mock_wellos_token", "", []string{"super_admin"}, []string{"kpi:read", "demands:write", "users:write"})
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
	}
	srv := NewServer(cfg, "")

	// 1. Test POST /api/users/department
	deptReq := map[string]string{
		"username":   "bob@westwell-lab.com",
		"department": "AI Lab",
	}
	deptBody, _ := json.Marshal(deptReq)
	req, _ := http.NewRequest("POST", "/api/users/department", bytes.NewBuffer(deptBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/users/department failed: got %v", rr.Code)
	}

	// Verify Bob's department updated
	var updatedBob userdb.User
	db.DB.Where("username = ?", "bob@westwell-lab.com").First(&updatedBob)
	if updatedBob.Department != "AI Lab" {
		t.Errorf("Expected Bob department to be 'AI Lab', got '%s'", updatedBob.Department)
	}

	// 2. Test POST /api/demands
	demandReq := map[string]string{
		"title":       "Test new requirement from PM",
		"description": "This is a detailed specification description.",
		"assignee":    "Bob",
		"due_date":    "2026-06-30",
		"repo":        "platform-core",
	}
	demandBody, _ := json.Marshal(demandReq)
	req, _ = http.NewRequest("POST", "/api/demands", bytes.NewBuffer(demandBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/demands failed: got %v", rr.Code)
	}

	var demandRes map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&demandRes)
	taskID := demandRes["task_id"].(string)

	if !strings.HasPrefix(taskID, "DEMAND-") {
		t.Errorf("Expected generated task_id prefix 'DEMAND-', got '%s'", taskID)
	}

	// Verify task in DB
	var task db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		t.Fatalf("Demand task not found in DB: %v", err)
	}
	if task.Title != "Test new requirement from PM" || task.IssueType != "demand" || task.Status != "backlog" {
		t.Errorf("Demand task fields mismatch: %+v", task)
	}

	// Verify notification generated
	var notif db.Notification
	if err := db.DB.Where("task_id = ? AND type = ?", taskID, "demand_assigned").First(&notif).Error; err != nil {
		t.Errorf("Assignment notification not created in DB: %v", err)
	}

	// 3. Test POST /api/tasks/schedule
	// Bob signs in and schedules the task
	bobToken, _ := GenerateJWT("bob@westwell-lab.com", "Bob", "mock_bob_token", "", []string{"member"}, []string{})
	schedReq := map[string]interface{}{
		"task_id":        taskID,
		"branch":         "feat/demand-test-01",
		"due_date":       "2026-06-29",
		"status":         "progress",
		"task_group_id":  "group-test-001",
		"estimate_hours": 18.4,
		"estimate_days":  2.3,
		"difficulty":     "High",
	}
	schedBody, _ := json.Marshal(schedReq)
	req, _ = http.NewRequest("POST", "/api/tasks/schedule", bytes.NewBuffer(schedBody))
	req.Header.Set("Authorization", "Bearer "+bobToken)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/tasks/schedule failed: got %v", rr.Code)
	}

	// Verify task scheduled
	var scheduledTask db.TaskTelemetry
	db.DB.Where("task_id = ?", taskID).First(&scheduledTask)
	if scheduledTask.Branch != "feat/demand-test-01" || scheduledTask.Status != "progress" || scheduledTask.TaskGroupID != "group-test-001" {
		t.Errorf("Expected scheduled task changes, got %+v", scheduledTask)
	}
	if scheduledTask.EstimateHours != 18.4 || scheduledTask.EstimateDays != 2.3 || scheduledTask.Difficulty != "High" || scheduledTask.EstimateSource != "manual_adjusted" {
		t.Errorf("Expected manual estimate fields to persist, got %+v", scheduledTask)
	}

	// Verify notification dismissed
	var notifState db.UserNotificationState
	notifKey := fmt.Sprintf("demand_assigned_%d", notif.ID)
	if err := db.DB.Where("notification_key = ? AND user_id = ?", notifKey, "bob@westwell-lab.com").First(&notifState).Error; err != nil {
		t.Errorf("Notification state not marked as read: %v", err)
	}
	if notifState.Status != "dismissed" {
		t.Errorf("Expected dismissed notification, got status '%s'", notifState.Status)
	}

	// 4. Test KPI performance data calculation
	// Move task to done so it gets counted in KPI
	scheduledTask.Status = "done"
	now := time.Now()
	scheduledTask.CompletedAt = &now
	db.DB.Save(&scheduledTask)

	req, _ = http.NewRequest("GET", "/api/kpi/performance?period=week", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/kpi/performance failed: got %v", rr.Code)
	}

	var kpiRes KPIPerformanceResponse
	json.NewDecoder(rr.Body).Decode(&kpiRes)

	if kpiRes.Summary.DemandsCompleted != 1 || kpiRes.Summary.TotalCompleted != 1 {
		t.Errorf("KPI Summary count mismatch: %+v", kpiRes.Summary)
	}

	// Bob should be top 1 R&D member
	foundBob := false
	for _, uKPI := range kpiRes.UserKPI {
		if uKPI.Name == "Bob" {
			foundBob = true
			if uKPI.DemandsCompleted != 1 || uKPI.TotalCompleted != 1 || uKPI.Department != "AI Lab" {
				t.Errorf("Bob's KPI detail mismatch: %+v", uKPI)
			}
			break
		}
	}
	if !foundBob {
		t.Error("Bob not found in UserKPI list")
	}

	// AI Lab department should be top 1
	foundDept := false
	for _, dKPI := range kpiRes.DepartmentKPI {
		if dKPI.Department == "AI Lab" {
			foundDept = true
			if dKPI.DemandsCompleted != 1 || dKPI.TotalCompleted != 1 {
				t.Errorf("AI Lab department's KPI detail mismatch: %+v", dKPI)
			}
			break
		}
	}
	if !foundDept {
		t.Error("AI Lab department not found in DepartmentKPI list")
	}

	// 5. Test POST /api/demands/archive
	// Create another demand to archive
	archiveReq := map[string]string{
		"title":       "Task to archive",
		"description": "Archive test",
		"assignee":    "Bob",
		"due_date":    "2026-06-30",
	}
	archiveBody, _ := json.Marshal(archiveReq)
	req, _ = http.NewRequest("POST", "/api/demands", bytes.NewBuffer(archiveBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	var archiveRes map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&archiveRes)
	archiveTaskID := archiveRes["task_id"].(string)

	// Verify creator & creatorDept were automatically captured in handleCreateDemand
	var archiveTask db.TaskTelemetry
	db.DB.Where("task_id = ?", archiveTaskID).First(&archiveTask)
	if archiveTask.Creator != "Eddie" || archiveTask.CreatorDept != "Management" {
		t.Errorf("Expected Creator 'Eddie' and CreatorDept 'Management', got creator='%s' dept='%s'", archiveTask.Creator, archiveTask.CreatorDept)
	}

	// Now archive the task
	archivePostReq := map[string]string{
		"task_id": archiveTaskID,
	}
	archivePostBody, _ := json.Marshal(archivePostReq)
	req, _ = http.NewRequest("POST", "/api/demands/archive", bytes.NewBuffer(archivePostBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/demands/archive failed: got %v", rr.Code)
	}

	// Verify it is archived in DB
	db.DB.Where("task_id = ?", archiveTaskID).First(&archiveTask)
	if archiveTask.Status != "archived" {
		t.Errorf("Expected status to be 'archived', got '%s'", archiveTask.Status)
	}

	// 6. Test DELETE /api/demands
	// Create another demand to delete
	deleteReq := map[string]string{
		"title":       "Task to delete",
		"description": "Delete test",
		"assignee":    "Bob",
		"due_date":    "2026-06-30",
	}
	deleteBody, _ := json.Marshal(deleteReq)
	req, _ = http.NewRequest("POST", "/api/demands", bytes.NewBuffer(deleteBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	var deleteRes map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&deleteRes)
	deleteTaskID := deleteRes["task_id"].(string)

	// Now delete it
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("/api/demands?task_id=%s", deleteTaskID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("DELETE /api/demands failed: got %v", rr.Code)
	}

	// Verify task is deleted from DB
	var deletedTask db.TaskTelemetry
	errDeleteSearch := db.DB.Where("task_id = ?", deleteTaskID).First(&deletedTask).Error
	if errDeleteSearch == nil {
		t.Errorf("Expected task %s to be deleted from DB, but still exists", deleteTaskID)
	}
}

func TestJiraSyncUserToLocalAndAssignee(t *testing.T) {
	setupServerTestDB(t)

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira: config.JiraConfig{
			Enabled: true,
		},
	}
	srv := NewServer(cfg, "")

	// 1. 测试从 Jira 同步用户到本地用户表
	// 初始状态下不存在该用户
	var user userdb.User
	err := db.DB.Where("username = ?", "lulu.zhang").First(&user).Error
	if err == nil {
		t.Fatalf("Expected user lulu.zhang not to exist in DB initially")
	}

	// 执行自动同步方法
	srv.syncJiraUserToLocal("lulu.zhang", "张露露", "lulu.zhang@westwell-lab.com")

	// 验证用户在 DB 中被正确创建
	err = db.DB.Where("username = ?", "lulu.zhang").First(&user).Error
	if err != nil {
		t.Fatalf("Expected user lulu.zhang to be created, but got error: %v", err)
	}
	if user.Name != "张露露" || user.Email != "lulu.zhang@westwell-lab.com" {
		t.Errorf("Unexpected user fields: %+v", user)
	}

	// 2. 测试重新同步已存在用户且有名字更新的情况
	srv.syncJiraUserToLocal("lulu.zhang", "张露露_new", "lulu.zhang@westwell-lab.com")
	err = db.DB.Where("username = ?", "lulu.zhang").First(&user).Error
	if err != nil {
		t.Fatalf("Failed to fetch user after update: %v", err)
	}
	if user.Name != "张露露_new" {
		t.Errorf("Expected user name to be '张露露_new', got '%s'", user.Name)
	}
}

func TestGetTaskCommitsCombinesGitAndJiraLogs(t *testing.T) {
	setupServerTestDB(t)

	cfg := &config.Config{Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"}}
	srv := NewServer(cfg, "")

	taskID := "TEST-101"
	now := time.Now()

	// 1. 种子数据：GitCommitLog
	gitLog := db.GitCommitLog{
		TaskID:    taskID,
		Repo:      "core",
		Branch:    "feat/TEST-101",
		CommitID:  "git-sha-123",
		Message:   "git commit message",
		Author:    "Alice",
		Action:    "git_push",
		CreatedAt: now.Add(-10 * time.Minute),
	}
	db.DB.Create(&gitLog)

	// 2. 种子数据：JiraCommentLog
	jiraComment := db.JiraCommentLog{
		TaskID:    taskID,
		CommentID: "comment-id-456",
		Author:    "Bob (Jira)",
		Body:      "jira comment body",
		CreatedAt: now, // 更加新鲜，应该排在前面
	}
	db.DB.Create(&jiraComment)

	// 3. 构造请求调用 handleGetTaskCommits
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/commits?task_id="+taskID, nil)
	rr := httptest.NewRecorder()
	srv.handleGetTaskCommits(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Unexpected status: got %d", rr.Code)
	}

	var activities []TelemetryActivityDTO
	if err := json.NewDecoder(rr.Body).Decode(&activities); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(activities) != 2 {
		t.Fatalf("Expected 2 activities, got %d", len(activities))
	}

	// 验证排序：Jira 评论更加新鲜，应位于 activities[0]
	if activities[0].Action != "jira_comment" || activities[0].Author != "Bob (Jira)" {
		t.Errorf("Expected first item to be Jira comment, got %+v", activities[0])
	}
	if activities[1].Action != "git_push" || activities[1].CommitID != "git-sha-123" {
		t.Errorf("Expected second item to be Git push, got %+v", activities[1])
	}
}

func TestGetTaskCommitsIncludesHistoricalCaseVariantsAcrossRepositories(t *testing.T) {
	setupServerTestDB(t)

	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"}}, "")
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "FZ-2247", ExternalKey: "FZ-2247", Source: "jira", IssueType: "requirement", Title: "吊具检测驶离保护",
	}).Error; err != nil {
		t.Fatalf("seed work item: %v", err)
	}
	logs := []db.GitCommitLog{
		{TaskID: "fz-2247", Repo: "task_executor", CommitID: "task-executor-sha", Message: "feat: FZ-2247 task executor", Action: "git_push", CreatedAt: time.Now().Add(-time.Minute)},
		{TaskID: "FZ-2247", Repo: "crane_manager", CommitID: "crane-manager-sha", Message: "feat: FZ-2247 crane manager", Action: "git_push", CreatedAt: time.Now()},
	}
	if err := db.DB.Create(&logs).Error; err != nil {
		t.Fatalf("seed multi-repository evidence: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/commits?task_id=FZ-2247", nil)
	rr := httptest.NewRecorder()
	srv.handleGetTaskCommits(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET task commits status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var activities []TelemetryActivityDTO
	if err := json.NewDecoder(rr.Body).Decode(&activities); err != nil {
		t.Fatalf("decode activities: %v", err)
	}
	if len(activities) != 2 {
		t.Fatalf("activity count = %d, want 2: %+v", len(activities), activities)
	}
	if activities[0].Repo != "crane_manager" || activities[1].Repo != "task_executor" {
		t.Fatalf("activity repositories = [%s, %s], want [crane_manager, task_executor]", activities[0].Repo, activities[1].Repo)
	}
}
