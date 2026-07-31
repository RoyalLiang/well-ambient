package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/deliveryplanning"
)

func TestExecutionTasksKeepLocalWorkAndExposeAuthoritativeFacets(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "task-reader@westwell-lab.com", "Task Reader", []string{"dashboard:read"})
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira: config.JiraConfig{
			SyncProjects: []string{"HIT", "NS2"},
			SyncUsers:    []string{"Alice", "Reviewer", "梁志远"},
		},
	}
	srv := NewServer(cfg, "")

	users := []userdb.User{
		{
			Username:   "reviewer@westwell-lab.com",
			Email:      "reviewer@westwell-lab.com",
			Name:       "Reviewer",
			Department: "Quality",
		},
		{
			Username:   "zhiyuan_liang@westwell-lab.com",
			Email:      "zhiyuan_liang@westwell-lab.com",
			Name:       "梁志远",
			Department: "Engineering",
		},
	}
	if err := db.DB.Create(&users).Error; err != nil {
		t.Fatalf("seed configured reviewer: %v", err)
	}

	projects := []db.ProjectConfig{
		{ProjectKey: "HIT", ProjectName: "香港二期"},
		{ProjectKey: "NS2", ProjectName: "南沙二期"},
		{ProjectKey: "WITH", ProjectName: "兼容前缀污染"},
	}
	if err := db.DB.Create(&projects).Error; err != nil {
		t.Fatalf("seed project configs: %v", err)
	}

	now := time.Now()
	tasks := []db.TaskTelemetry{
		{
			TaskID:     "HIT-LOCAL",
			Title:      "本地执行任务",
			Source:     "local",
			ProjectKey: "HIT",
			Assignee:   "Vendor",
			Status:     "progress",
			IssueType:  "task",
			LastUpdate: now,
		},
		{
			TaskID:     "NS2-CORE",
			Title:      "核心成员 Jira 任务",
			Source:     "jira",
			ProjectKey: "NS2",
			Assignee:   "Alice",
			Status:     "progress",
			IssueType:  "task",
			LastUpdate: now,
		},
		{
			TaskID:     "NS2-EXTERNAL",
			Title:      "外部 Jira 任务",
			Source:     "jira",
			ProjectKey: "NS2",
			Assignee:   "Vendor",
			Status:     "progress",
			IssueType:  "task",
			LastUpdate: now,
		},
		{
			TaskID:     "with-5",
			Title:      "无项目归属的历史本地任务",
			Source:     "local",
			Assignee:   "Operator",
			Status:     "progress",
			IssueType:  "task",
			LastUpdate: now,
		},
		{
			TaskID:     "HIT-ALIAS",
			Title:      "Jira 点号账号任务",
			Source:     "local",
			ProjectKey: "HIT",
			Assignee:   "zhiyuan.liang",
			Status:     "progress",
			IssueType:  "task",
			LastUpdate: now,
		},
	}
	if err := db.DB.Create(&tasks).Error; err != nil {
		t.Fatalf("seed execution tasks: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/execution/tasks", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/execution/tasks status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response ExecutionTasksResponseDTO
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode execution tasks response: %v", err)
	}

	itemsByID := make(map[string]ExecutionTaskItemDTO, len(response.Items))
	for _, item := range response.Items {
		itemsByID[item.TaskID] = item
	}
	if _, exists := itemsByID["HIT-LOCAL"]; !exists {
		t.Fatalf("local non-core execution task must remain visible: %+v", response.Items)
	}
	if _, exists := itemsByID["NS2-CORE"]; !exists {
		t.Fatalf("core Jira execution task must remain visible: %+v", response.Items)
	}
	if _, exists := itemsByID["NS2-EXTERNAL"]; exists {
		t.Fatalf("external Jira execution task must follow Jira visibility: %+v", response.Items)
	}
	if got := itemsByID["with-5"].ProjectKey; got != "" {
		t.Fatalf("historical task prefix must not become a project fact, got %q", got)
	}
	if got := itemsByID["HIT-ALIAS"].Assignee; got != "梁志远" {
		t.Fatalf("execution item must resolve separator aliases to the directory display name, got %q", got)
	}

	if len(response.Facets.Projects) != 2 {
		t.Fatalf("project facets must use Jira-scoped authoritative catalog, got %+v", response.Facets.Projects)
	}
	if response.Facets.Projects[0].ProjectKey != "HIT" || response.Facets.Projects[1].ProjectKey != "NS2" {
		t.Fatalf("unexpected authoritative project facets: %+v", response.Facets.Projects)
	}
	assignees := make(map[string]bool, len(response.Facets.Assignees))
	for _, option := range response.Facets.Assignees {
		assignees[option.Value] = true
	}
	if !assignees["Alice"] || !assignees["Reviewer"] || !assignees["梁志远"] {
		t.Fatalf("assignee facets must expose every configured core member, got %+v", response.Facets.Assignees)
	}
	if assignees["Vendor"] || assignees["Operator"] {
		t.Fatalf("visible local-task owners must not pollute the shared core-member directory, got %+v", response.Facets.Assignees)
	}
	if assignees["zhiyuan.liang"] {
		t.Fatalf("assignee facets must not duplicate a resolved directory identity, got %+v", response.Facets.Assignees)
	}
}

func TestExecutionTasksExcludeCommitDerivedRowsAndExposeParentKind(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "execution-boundary@westwell-lab.com", "Execution Boundary", []string{"dashboard:read"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"}}, "")

	now := time.Now()
	rows := []db.TaskTelemetry{
		{
			TaskID: "HIT-901", Title: "车辆到位异常", Source: "jira", ExternalKey: "HIT-901",
			ProjectKey: "HIT", IssueType: "bug", Status: "progress", TaskGroupID: "group-bug", LastUpdate: now,
		},
		{
			TaskID: "EXEC-901", Title: "修复到位状态机", Source: "local", ProjectKey: "HIT",
			IssueType: "task", Status: "progress", TaskGroupID: "group-bug", ParentWorkItemID: "HIT-901", LastUpdate: now,
		},
		{
			TaskID: "middleq-20260731", Title: "Merge branch 'feature' into develop", Source: "git",
			IssueType: "task", Status: "progress", Repo: "task_executor", Branch: "feature/middleq-20260731",
			LastCommit: "Merge branch 'feature' into develop", LastUpdate: now,
		},
		{
			TaskID: "legacy-commit-7", Title: "fix: legacy commit row",
			IssueType: "task", Status: "progress", Repo: "task_executor", Branch: "fix/legacy-commit-7",
			LastCommit: "fix: legacy commit row\n\nbody", LastUpdate: now,
		},
	}
	if err := db.DB.Create(&rows).Error; err != nil {
		t.Fatalf("seed execution boundary rows: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/execution/tasks", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/execution/tasks status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response ExecutionTasksResponseDTO
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode execution tasks response: %v", err)
	}
	itemsByID := make(map[string]ExecutionTaskItemDTO, len(response.Items))
	for _, item := range response.Items {
		itemsByID[item.TaskID] = item
	}
	if _, exists := itemsByID["middleq-20260731"]; exists {
		t.Fatalf("source=git evidence row must not become an execution task: %+v", response.Items)
	}
	if _, exists := itemsByID["legacy-commit-7"]; exists {
		t.Fatalf("legacy commit-derived telemetry row must not become an execution task: %+v", response.Items)
	}
	item, exists := itemsByID["EXEC-901"]
	if !exists {
		t.Fatalf("real local execution task must remain visible: %+v", response.Items)
	}
	if item.ParentIssueType != deliveryplanning.WorkItemBug {
		t.Fatalf("parent issue type = %q, want %q", item.ParentIssueType, deliveryplanning.WorkItemBug)
	}
}

func TestExecutionTasksProjectEvidenceBackedWorkItemsWithoutCommitRows(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "execution-evidence@westwell-lab.com", "Execution Evidence", []string{"dashboard:read"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"}}, "")

	now := time.Now()
	rows := []db.TaskTelemetry{
		{
			TaskID: "HIT-1101", Title: "车辆到位异常", Source: "jira", ExternalKey: "HIT-1101",
			ProjectKey: "HIT", Assignee: "Alice", IssueType: "bug", Status: "progress", LastUpdate: now,
		},
		{
			TaskID: "NS2-2202", Title: "优化任务派发", Source: "jira", ExternalKey: "NS2-2202",
			ProjectKey: "NS2", Assignee: "Reviewer", IssueType: "requirement", Status: "review", LastUpdate: now,
		},
		{
			TaskID: "EXEC-3303", Title: "补充回归测试", Source: "local",
			ProjectKey: "HIT", Assignee: "Alice", IssueType: "task", Status: "progress", LastUpdate: now,
		},
		{
			TaskID: "middleq-20260731", Title: "fix: raw commit row", Source: "git",
			IssueType: "task", Status: "progress", Repo: "task_executor", Branch: "feature/middleq-20260731",
			LastCommit: "fix: raw commit row", LastUpdate: now,
		},
	}
	if err := db.DB.Create(&rows).Error; err != nil {
		t.Fatalf("seed evidence projection rows: %v", err)
	}
	logs := []db.GitCommitLog{
		{TaskID: "HIT-1101", Repo: "task_executor", Branch: "fix/HIT-1101", CommitID: "bug-commit", Message: "fix bug", Action: "git_push", CreatedAt: now},
		{TaskID: "NS2-2202", Repo: "task_executor", Branch: "feat/NS2-2202", CommitID: "task-commit", Message: "build task", Action: "git_push", CreatedAt: now},
		{TaskID: "middleq-20260731", Repo: "task_executor", Branch: "feature/middleq-20260731", CommitID: "raw-commit", Message: "raw commit", Action: "git_push", CreatedAt: now},
	}
	if err := db.DB.Create(&logs).Error; err != nil {
		t.Fatalf("seed execution evidence: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/execution/tasks", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/execution/tasks status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response ExecutionTasksResponseDTO
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode execution tasks response: %v", err)
	}
	itemsByID := make(map[string]ExecutionTaskItemDTO, len(response.Items))
	for _, item := range response.Items {
		itemsByID[item.TaskID] = item
	}
	for _, taskID := range []string{"HIT-1101", "NS2-2202", "EXEC-3303"} {
		if _, exists := itemsByID[taskID]; !exists {
			t.Fatalf("execution tracking missing %s: %+v", taskID, response.Items)
		}
	}
	if _, exists := itemsByID["middleq-20260731"]; exists {
		t.Fatalf("raw commit telemetry must not become a tracking row: %+v", response.Items)
	}
	if got := itemsByID["HIT-1101"].ParentIssueType; got != deliveryplanning.WorkItemBug {
		t.Fatalf("bug evidence projection parent kind = %q, want %q", got, deliveryplanning.WorkItemBug)
	}
	if got := itemsByID["NS2-2202"].ParentIssueType; got != deliveryplanning.WorkItemRequirement {
		t.Fatalf("requirement evidence projection parent kind = %q, want %q", got, deliveryplanning.WorkItemRequirement)
	}
}
