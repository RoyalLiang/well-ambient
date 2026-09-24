package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func setupProjectCatalogTest(t *testing.T) (*Server, string) {
	t.Helper()
	previousDB := db.DB
	setupServerTestDB(t)
	conn := db.DB
	t.Cleanup(func() {
		sqlDB, err := conn.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		db.DB = previousDB
	})
	username := "catalog-reader@example.test"
	if err := conn.Create(&userdb.User{Username: username, Name: "Catalog Reader"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&userdb.AuthorizationPolicy{
		Effect: "allow", SubjectType: "user", SubjectID: username, Action: "config:read",
		ResourceType: "config", Scope: "global", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	token, err := GenerateJWT(username, "Catalog Reader", "", "", []string{"member"}, []string{"config:read"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &config.Config{}, mux: http.NewServeMux()}
	s.routes()
	return s, token
}

func projectCatalogGET(t *testing.T, s *Server, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	s.mux.ServeHTTP(recorder, request)
	return recorder
}

func readProjectCatalog(t *testing.T, s *Server, token string) []projectPreferenceOption {
	t.Helper()
	response := projectCatalogGET(t, s, token, "/api/projects/catalog")
	if response.Code != http.StatusOK {
		t.Fatalf("catalog GET status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("catalog must not be cached: Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	var result struct {
		Projects []projectPreferenceOption `json:"projects"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if result.Projects == nil {
		t.Fatal("catalog projects must be an array, including when empty")
	}
	return result.Projects
}

func TestProjectCatalogMergesSettingsSourcesWithoutCreatingMappings(t *testing.T) {
	s, token := setupProjectCatalogTest(t)
	s.config.Jira.SyncProjects = []string{" old ", "OLD", "", "  "}
	s.config.Jira.VersionSources = []config.JiraVersionSource{
		{ProjectKey: " ver ", ProjectName: " Version project "},
		{ProjectKey: " OLD ", ProjectName: "Source must not replace manual name"},
		{ProjectKey: "saved", ProjectName: "Saved source name"},
		{ProjectKey: "keyonly", ProjectName: "Descriptive source name"},
		{ProjectKey: " ", ProjectName: "Not a project"},
	}
	mappings := []db.ProjectConfig{
		{ProjectKey: " old ", ProjectName: " Manual name ", GitReposJSON: `["manual-repo"]`},
		{ProjectKey: "SAVED", ProjectName: ""},
		{ProjectKey: "KEYONLY", ProjectName: "KEYONLY"},
		{ProjectKey: "ARCHIVE", ProjectName: "Retained mapping"},
	}
	if err := db.DB.Create(&mappings).Error; err != nil {
		t.Fatal(err)
	}
	tasks := []db.TaskTelemetry{
		{TaskID: "WRONG-1", ProjectKey: " new ", Source: "jira", ExternalKey: "OTHER-1", Repo: "New Jira project (NEW)"},
		{TaskID: "NEW-2", ProjectKey: "NEW", Source: "jira"},
		{TaskID: "MISLEAD-3", Source: "jira", ExternalKey: " ext-3 "},
		{TaskID: " legacy-4 ", ProjectKey: "   ", Source: "jira", Repo: "Legacy Jira project (LEGACY)"},
		{TaskID: "OLD-10", ProjectKey: "OLD", Source: "jira", Repo: "Old Jira project (OLD)"},
		{TaskID: "VER-10", ProjectKey: "VER", Source: "jira", Repo: "Jira version project (VER)"},
		{TaskID: "BADNAME-10", ProjectKey: "BADNAME", Source: "jira", Repo: "Wrong Jira name (OTHER)"},
		{TaskID: "LOCAL-5", Source: "local"},
		{TaskID: "GIT-6", Source: "git"},
		{TaskID: "UNKNOWN-7"},
		{TaskID: "LOCAL-8", Source: "local", ExternalKey: "FAKE-8", ProjectKey: "FAKE"},
		{TaskID: "not-a-jira-key", Source: "jira"},
	}
	if err := db.DB.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	before := projectCatalogGET(t, s, token, "/api/projects/config").Body.String()
	got := readProjectCatalog(t, s, token)
	want := []projectPreferenceOption{
		{ProjectKey: "ARCHIVE", ProjectName: "Retained mapping"},
		{ProjectKey: "BADNAME", ProjectName: "BADNAME"},
		{ProjectKey: "EXT", ProjectName: "EXT"},
		{ProjectKey: "KEYONLY", ProjectName: "Descriptive source name"},
		{ProjectKey: "LEGACY", ProjectName: "Legacy Jira project"},
		{ProjectKey: "NEW", ProjectName: "New Jira project"},
		{ProjectKey: "OLD", ProjectName: "Manual name"},
		{ProjectKey: "SAVED", ProjectName: "Saved source name"},
		{ProjectKey: "VER", ProjectName: "Version project"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog = %#v, want %#v", got, want)
	}
	if again := readProjectCatalog(t, s, token); !reflect.DeepEqual(again, want) {
		t.Fatalf("repeated catalog = %#v, want stable order %#v", again, want)
	}
	after := projectCatalogGET(t, s, token, "/api/projects/config").Body.String()
	if before != after {
		t.Fatalf("catalog GET changed mappings: before=%s after=%s", before, after)
	}
}

func TestProjectCatalogReadsNewSyncAndVersionSourcesOnEveryGET(t *testing.T) {
	s, token := setupProjectCatalogTest(t)
	if got := readProjectCatalog(t, s, token); len(got) != 0 {
		t.Fatalf("empty catalog = %#v", got)
	}
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "ARRIVED-1", ProjectKey: "ARRIVED", Source: "jira",
	}).Error; err != nil {
		t.Fatal(err)
	}
	s.config.Jira.VersionSources = []config.JiraVersionSource{
		{ProjectKey: "NEXT", ProjectName: "Next release"},
	}
	want := []projectPreferenceOption{
		{ProjectKey: "ARRIVED", ProjectName: "ARRIVED"},
		{ProjectKey: "NEXT", ProjectName: "Next release"},
	}
	if got := readProjectCatalog(t, s, token); !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog after sync/config update = %#v, want %#v", got, want)
	}
}

func TestProjectCatalogRequiresConfigReadPermission(t *testing.T) {
	s, token := setupProjectCatalogTest(t)
	if response := projectCatalogGET(t, s, "", "/api/projects/catalog"); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", response.Code)
	}
	if response := projectCatalogGET(t, s, token, "/api/projects/catalog"); response.Code != http.StatusOK {
		t.Fatalf("config reader status = %d, body=%s", response.Code, response.Body.String())
	}
	if err := db.DB.Model(&userdb.AuthorizationPolicy{}).
		Where("subject_id = ?", "catalog-reader@example.test").
		Update("action", "config:write").Error; err != nil {
		t.Fatal(err)
	}
	if response := projectCatalogGET(t, s, token, "/api/projects/catalog"); response.Code != http.StatusForbidden {
		t.Fatalf("writer without read permission status = %d, want 403", response.Code)
	}
}

func TestProjectCatalogCannotTurnGlobalSettingsIntoRepoScopedRead(t *testing.T) {
	s, _ := setupProjectCatalogTest(t)
	token := seedPolicyTestUser(t, "repo-catalog-admin@example.test", "admin", "repo", "platform-core")
	for _, query := range []string{"", "?repo=platform-core", "?project_id=platform-core"} {
		response := projectCatalogGET(t, s, token, "/api/projects/catalog"+query)
		if response.Code != http.StatusForbidden {
			t.Fatalf("repo-scoped catalog GET %q status = %d, want 403", query, response.Code)
		}
	}
}

func TestProjectConfigsReturnsSavedProjectsOutsideOldSyncScope(t *testing.T) {
	s, token := setupProjectCatalogTest(t)
	s.config.Jira.SyncProjects = []string{"OLD"}
	mappings := []db.ProjectConfig{
		{ProjectKey: "OLD", ProjectName: "Old project"},
		{ProjectKey: "NEW", ProjectName: "New synced project", GitReposJSON: `["new-repo"]`},
	}
	if err := db.DB.Create(&mappings).Error; err != nil {
		t.Fatal(err)
	}
	response := projectCatalogGET(t, s, token, "/api/projects/config")
	if response.Code != http.StatusOK {
		t.Fatalf("mapping GET status = %d, body=%s", response.Code, response.Body.String())
	}
	var got []db.ProjectConfig
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, mappings) {
		t.Fatalf("saved mappings = %#v, want all persisted mappings %#v", got, mappings)
	}
}

func TestProjectConfigSavePreservesManualNameThroughScoringAndReadback(t *testing.T) {
	s, token := setupProjectCatalogTest(t)
	if err := db.DB.Create(&userdb.AuthorizationPolicy{
		Effect: "allow", SubjectType: "user", SubjectID: "catalog-reader@example.test",
		Action: "config:write", ResourceType: "config", Scope: "global", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID: "NEWJIRA-1", ProjectKey: "NEWJIRA", Source: "jira",
		Repo: "New delivery (NEWJIRA)", Status: "progress",
	}).Error; err != nil {
		t.Fatal(err)
	}

	// Both descriptive and legacy-looking names can be explicit user choices.
	for _, name := range []string{
		"\u65b0\u540c\u6b65\u4ea4\u4ed8\u9879\u76ee",
		"NEWJIRA\u9879\u76ee",
	} {
		body, err := json.Marshal(map[string]any{
			"project_key": "NEWJIRA", "project_name": name,
			"git_repos_json": `["delivery-repo"]`, "base_score": 60, "base_score_weight": 0.1,
		})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/projects/config", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		saved := httptest.NewRecorder()
		s.mux.ServeHTTP(saved, request)
		if saved.Code != http.StatusOK {
			t.Fatalf("save mapping status = %d, body=%s", saved.Code, saved.Body.String())
		}

		readback := projectCatalogGET(t, s, token, "/api/projects/config")
		var mappings []db.ProjectConfig
		if err := json.Unmarshal(readback.Body.Bytes(), &mappings); err != nil {
			t.Fatal(err)
		}
		if len(mappings) != 1 || mappings[0].ProjectName != name ||
			mappings[0].GitReposJSON != `["delivery-repo"]` {
			t.Fatalf("saved mapping after scoring = %#v, want manual name %q and chosen repo", mappings, name)
		}

		scored := projectCatalogGET(t, s, token, "/api/projects/scores")
		var scores []db.ProjectScore
		if err := json.Unmarshal(scored.Body.Bytes(), &scores); err != nil {
			t.Fatal(err)
		}
		if len(scores) != 1 || scores[0].ProjectName != name {
			t.Fatalf("score names = %#v, want manual name %q", scores, name)
		}
		if got := readProjectCatalog(t, s, token); !reflect.DeepEqual(got, []projectPreferenceOption{
			{ProjectKey: "NEWJIRA", ProjectName: name},
		}) {
			t.Fatalf("catalog after rescoring = %#v, want manual name %q", got, name)
		}
		if after := projectCatalogGET(t, s, token, "/api/projects/config"); after.Body.String() != readback.Body.String() {
			t.Fatalf("score GET changed mappings: before=%s after=%s", readback.Body.String(), after.Body.String())
		}
	}
}
