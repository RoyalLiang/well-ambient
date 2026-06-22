package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/kanban"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
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

func TestConfigAPI(t *testing.T) {
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

	// Verify write to disk
	savedCfg, err := config.LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read saved config file: %v", err)
	}
	if savedCfg.GitLab.BaseURL != "https://new-gitlab.com" {
		t.Errorf("Expected saved config update to %q, got %q", "https://new-gitlab.com", savedCfg.GitLab.BaseURL)
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
	for _, item := range response.Items {
		if item.DemandID == "DEMAND-OVERDUE" {
			overdue = item
			break
		}
	}
	if overdue.SubtaskTotal != 2 || overdue.SubtaskDone != 1 || overdue.SubtaskActive != 1 {
		t.Fatalf("overdue subtask stats mismatch: %+v", overdue)
	}
	if overdue.Department != "Product" {
		t.Fatalf("overdue department = %q, want Product", overdue.Department)
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
		return nil, io.EOF
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
	if !strings.Contains(rr.Body.String(), "no local session fallback") {
		t.Fatalf("unexpected unknown user fallback response: %s", rr.Body.String())
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
	schedReq := map[string]string{
		"task_id":       taskID,
		"branch":        "feat/demand-test-01",
		"due_date":      "2026-06-29",
		"status":        "progress",
		"task_group_id": "group-test-001",
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
