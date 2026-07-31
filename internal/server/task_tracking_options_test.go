package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func TestTaskTrackingAssigneeOptionsUseConfiguredCoreMembersNotRBACMemberships(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "task-options-admin@westwell-lab.com", "Task Options Admin", []string{"delivery:read"})

	rbacOnly := userdb.User{Username: "rbac@westwell-lab.com", Email: "rbac@westwell-lab.com", Name: "RBAC Only", Department: "Operations"}
	configuredAlias := userdb.User{Username: "zhiyuan_liang@westwell-lab.com", Email: "zhiyuan_liang@westwell-lab.com", Name: "梁志远", Department: "Engineering"}
	external := userdb.User{Username: "external@westwell-lab.com", Email: "external@westwell-lab.com", Name: "External Candidate", Department: "Vendor"}
	if err := db.DB.Create(&rbacOnly).Error; err != nil {
		t.Fatalf("seed RBAC-only member: %v", err)
	}
	if err := db.DB.Create(&configuredAlias).Error; err != nil {
		t.Fatalf("seed configured alias member: %v", err)
	}
	if err := db.DB.Create(&external).Error; err != nil {
		t.Fatalf("seed external candidate: %v", err)
	}
	var memberGroup userdb.UserGroup
	if err := db.DB.Where("name = ?", "member").First(&memberGroup).Error; err != nil {
		t.Fatalf("load member group: %v", err)
	}
	if err := db.DB.Create(&userdb.UserGroupMembership{UserID: rbacOnly.ID, UserGroupID: memberGroup.ID, Scope: "global"}).Error; err != nil {
		t.Fatalf("seed RBAC-only membership: %v", err)
	}

	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira: config.JiraConfig{
			SyncUsers: []string{"Configured Owner", "zhiyuan.liang"},
			CustomJQL: `assignee in ("JQL Candidate")`,
		},
	}, "")
	request := httptest.NewRequest(http.MethodGet, "/api/task-tracking/assignees", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/task-tracking/assignees status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Assignees []ExecutionAssigneeOptionDTO `json:"assignees"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode task tracking assignees: %v", err)
	}
	values := make(map[string]bool, len(response.Assignees))
	for _, option := range response.Assignees {
		values[option.Value] = true
	}
	for _, want := range []string{"Configured Owner", "梁志远"} {
		if !values[want] {
			t.Fatalf("configured core member %q missing from options: %+v", want, response.Assignees)
		}
	}
	for _, notWant := range []string{"RBAC Only", "External Candidate", "zhiyuan.liang", "JQL Candidate"} {
		if values[notWant] {
			t.Fatalf("non-directory candidate %q leaked into task options: %+v", notWant, response.Assignees)
		}
	}
}

func TestDeliveryDirectoryUsesAuthoritativeProjectCatalog(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "directory-admin@westwell-lab.com", "Directory Admin", []string{"delivery:read"})
	if err := db.DB.Create(&[]db.ProjectConfig{
		{ProjectKey: "HIT", ProjectName: "香港二期"},
		{ProjectKey: "LOCAL", ProjectName: "非 Jira 本地项目"},
	}).Error; err != nil {
		t.Fatalf("seed project directory: %v", err)
	}

	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9090, Host: "127.0.0.1"},
		Jira: config.JiraConfig{
			SyncProjects: []string{"HIT", "NS2"},
			SyncUsers:    []string{"Configured Owner"},
		},
	}, "")
	request := httptest.NewRequest(http.MethodGet, "/api/delivery/directory", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	srv.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/delivery/directory status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response DeliveryDirectoryResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode delivery directory: %v", err)
	}
	if len(response.Projects) != 2 {
		t.Fatalf("project directory must follow configured Jira catalog: %+v", response.Projects)
	}
	projects := make(map[string]string, len(response.Projects))
	for _, project := range response.Projects {
		projects[project.ProjectKey] = project.ProjectName
	}
	if projects["HIT"] != "香港二期" || projects["NS2"] != "NS2" {
		t.Fatalf("project key/name pairs are not normalized: %+v", response.Projects)
	}
	if _, exists := projects["LOCAL"]; exists {
		t.Fatalf("local config outside the Jira catalog leaked into directory: %+v", response.Projects)
	}
}
