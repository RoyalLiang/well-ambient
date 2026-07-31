package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProjectPreferenceHandlerTest(t *testing.T) *Server {
	t.Helper()
	previousDB := db.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(&db.UserProjectPreference{}, &db.ProjectConfig{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	db.DB = conn
	t.Cleanup(func() { db.DB = previousDB })

	projects := []db.ProjectConfig{
		{ProjectKey: "HIT", ProjectName: "HIT 香港二期"},
		{ProjectKey: "NS2", ProjectName: "南沙二期"},
		{ProjectKey: "DG", ProjectName: "东莞空港"},
	}
	if err := conn.Create(&projects).Error; err != nil {
		t.Fatalf("seed projects: %v", err)
	}
	return NewServer(&config.Config{Jira: config.JiraConfig{SyncProjects: []string{"HIT", "NS2", "FEL"}}}, "")
}

func TestProjectPreferenceHandlersDefaultSaveResetAndIsolation(t *testing.T) {
	server := setupProjectPreferenceHandlerTest(t)

	get := httptest.NewRequest(http.MethodGet, "/api/me/project-preferences", nil)
	get.Header.Set("x-authenticated-user-id", "alice@example.com")
	getRecorder := httptest.NewRecorder()
	server.handleGetProjectPreferences(getRecorder, get)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("default GET status = %d, body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	var initial projectPreferenceResponse
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode default GET: %v", err)
	}
	if initial.Mode != "all" || len(initial.ProjectKeys) != 0 {
		t.Fatalf("default response = %#v, want all projects", initial)
	}
	if len(initial.Projects) != 3 || initial.Projects[2].ProjectKey != "NS2" {
		t.Fatalf("available projects = %#v, want sync-scoped HIT/FEL/NS2 catalog", initial.Projects)
	}

	put := httptest.NewRequest(http.MethodPut, "/api/me/project-preferences", strings.NewReader(`{"project_keys":["ns2","HIT","hit"]}`))
	put.Header.Set("x-authenticated-user-id", "alice@example.com")
	putRecorder := httptest.NewRecorder()
	server.handleUpdateProjectPreferences(putRecorder, put)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", putRecorder.Code, putRecorder.Body.String())
	}
	var saved projectPreferenceResponse
	if err := json.Unmarshal(putRecorder.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode PUT: %v", err)
	}
	if saved.Mode != "selected" || strings.Join(saved.ProjectKeys, ",") != "HIT,NS2" {
		t.Fatalf("saved response = %#v, want normalized selected keys", saved)
	}

	bobKeys, err := db.LoadUserProjectPreferenceKeys(db.DB, "bob@example.com")
	if err != nil {
		t.Fatalf("load Bob keys: %v", err)
	}
	if len(bobKeys) != 0 {
		t.Fatalf("Bob keys = %#v, want independent all-project default", bobKeys)
	}

	reset := httptest.NewRequest(http.MethodPut, "/api/me/project-preferences", strings.NewReader(`{"project_keys":[]}`))
	reset.Header.Set("x-authenticated-user-id", "alice@example.com")
	resetRecorder := httptest.NewRecorder()
	server.handleUpdateProjectPreferences(resetRecorder, reset)
	if resetRecorder.Code != http.StatusOK {
		t.Fatalf("reset PUT status = %d, body=%s", resetRecorder.Code, resetRecorder.Body.String())
	}
	var resetResponse projectPreferenceResponse
	if err := json.Unmarshal(resetRecorder.Body.Bytes(), &resetResponse); err != nil {
		t.Fatalf("decode reset PUT: %v", err)
	}
	if resetResponse.Mode != "all" || len(resetResponse.ProjectKeys) != 0 {
		t.Fatalf("reset response = %#v, want all projects", resetResponse)
	}
}

func TestProjectPreferenceHandlerRejectsUnknownProjectWithoutReplacingSelection(t *testing.T) {
	server := setupProjectPreferenceHandlerTest(t)
	if err := db.ReplaceUserProjectPreferences(db.DB, "alice@example.com", []string{"HIT"}); err != nil {
		t.Fatalf("seed preference: %v", err)
	}

	request := httptest.NewRequest(http.MethodPut, "/api/me/project-preferences", strings.NewReader(`{"project_keys":["UNKNOWN"]}`))
	request.Header.Set("x-authenticated-user-id", "alice@example.com")
	recorder := httptest.NewRecorder()
	server.handleUpdateProjectPreferences(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown project status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	keys, err := db.LoadUserProjectPreferenceKeys(db.DB, "alice@example.com")
	if err != nil {
		t.Fatalf("reload preference: %v", err)
	}
	if strings.Join(keys, ",") != "HIT" {
		t.Fatalf("keys after rejected update = %#v, want original HIT", keys)
	}
}

func TestProjectPreferenceOptionsIncludeNamedVersionSource(t *testing.T) {
	server := setupProjectPreferenceHandlerTest(t)
	server.config.Jira.BaseURL = "https://jira.example.com"
	server.config.Jira.VersionSources = []config.JiraVersionSource{
		{
			ProjectKey:  "PRJ25024",
			ProjectName: "示例交付项目",
			VersionURL:  "https://jira.example.com/projects/PRJ25024/versions/13622",
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/api/me/project-preferences", nil)
	request.Header.Set("x-authenticated-user-id", "alice@example.com")
	recorder := httptest.NewRecorder()
	server.handleGetProjectPreferences(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body=%s", recorder.Code, recorder.Body.String())
	}

	var response projectPreferenceResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	for _, project := range response.Projects {
		if project.ProjectKey == "PRJ25024" {
			if project.ProjectName != "示例交付项目" {
				t.Fatalf("version source project name = %q, want configured name", project.ProjectName)
			}
			return
		}
	}
	t.Fatalf("version source project missing from options: %#v", response.Projects)
}
