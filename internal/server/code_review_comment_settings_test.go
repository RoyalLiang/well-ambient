package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"well-ambient/internal/codereview"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func reviewCommentSettingsServer(t *testing.T) (*Server, string) {
	t.Helper()
	setupServerTestDB(t)
	cfg := &config.Config{}
	cfg.GitLab = config.GitLabConfig{
		Enabled: true, BaseURL: "https://gitlab.example.test", APIToken: "fixture-only",
		Repos: []config.RepoMapping{{ProjectID: "10", Name: "fixture repository", Path: "fixture/repository"}},
	}
	srv := NewServer(cfg, "")
	token := superAdminToken(t, "review-settings@example.test", "Review Settings", []string{"dashboard:read", "config:write"})
	return srv, token
}

func TestCodeReviewCommentSettingsAPIPartialUpdateAndRepositoryDTO(t *testing.T) {
	srv, token := reviewCommentSettingsServer(t)
	w := reviewRequest(srv, token, "GET", "/api/code-reviews/comment-settings", "")
	var settings codereview.CommentSettings
	if err := json.Unmarshal(w.Body.Bytes(), &settings); err != nil || w.Code != http.StatusOK || !settings.SyncCommits || !settings.SyncMRs {
		t.Fatalf("default settings response=%d/%s err=%v", w.Code, w.Body.String(), err)
	}
	for _, body := range []string{`{"sync_commits":false}`, `{"sync_mrs":false}`} {
		w = reviewRequest(srv, token, "PATCH", "/api/code-reviews/comment-settings", body)
		if w.Code != http.StatusOK {
			t.Fatalf("partial settings response=%d/%s", w.Code, w.Body.String())
		}
	}
	w = reviewRequest(srv, token, "GET", "/api/code-reviews/repos", "")
	var response struct {
		CommentSettings codereview.CommentSettings `json:"comment_settings"`
		Repos           []struct {
			Policy db.CodeReviewPolicy `json:"policy"`
		} `json:"repos"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != http.StatusOK {
		t.Fatalf("repository response=%d/%s err=%v", w.Code, w.Body.String(), err)
	}
	if response.CommentSettings.SyncCommits || response.CommentSettings.SyncMRs || len(response.Repos) != 1 {
		t.Fatalf("repos omitted current global settings: %s", w.Body.String())
	}
	p := response.Repos[0].Policy
	if p.SyncCommits || p.SyncMRs || p.Domain != "general" || p.Scenario != "general" || p.KnowledgeScope != codereview.SystemCorpusScope {
		t.Fatalf("repository defaults or fixed corpus policy changed: %+v", p)
	}
	for _, body := range []string{`{}`, `{"sync_mrs":"enabled"}`, `{"sync_commits":null}`} {
		if w := reviewRequest(srv, token, "PATCH", "/api/code-reviews/comment-settings", body); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid settings accepted: body=%s status=%d", body, w.Code)
		}
	}
}

func TestCodeReviewCommentSettingsRequiresGlobalConfigPermission(t *testing.T) {
	srv, _ := reviewCommentSettingsServer(t)
	group := userdb.UserGroup{Name: "review-settings-reader"}
	if err := db.DB.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	var permission userdb.Permission
	if err := db.DB.Where("code = ?", "dashboard:read").First(&permission).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&userdb.GroupPermission{UserGroupID: group.ID, PermissionID: permission.ID}).Error; err != nil {
		t.Fatal(err)
	}
	reader := seedPolicyTestUser(t, "review-settings-reader@example.test", group.Name, "global", "")
	if w := reviewRequest(srv, reader, "GET", "/api/code-reviews/comment-settings", ""); w.Code != http.StatusOK {
		t.Fatalf("dashboard reader cannot read gates: %d/%s", w.Code, w.Body.String())
	}
	if w := reviewRequest(srv, reader, "PATCH", "/api/code-reviews/comment-settings?repo=fixture", `{"sync_mrs":true}`); w.Code != http.StatusForbidden {
		t.Fatalf("reader changed global external comments: %d/%s", w.Code, w.Body.String())
	}
}

func TestCodeReviewManualSyncReportsClosedGlobalGate(t *testing.T) {
	srv, token := reviewCommentSettingsServer(t)
	p := codereview.SystemCorpusPolicy(db.CodeReviewPolicy{ProjectID: "10", SyncCommits: true, SyncMRs: true})
	if err := db.DB.Save(&p).Error; err != nil {
		t.Fatal(err)
	}
	policyJSON, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if w := reviewRequest(srv, token, "PATCH", "/api/code-reviews/comment-settings", `{"sync_commits":false,"sync_mrs":false}`); w.Code != http.StatusOK {
		t.Fatalf("close global gates: %d/%s", w.Code, w.Body.String())
	}
	for _, kind := range []string{"mr", "commit"} {
		run := db.CodeReviewRun{
			Key: "closed-global-" + kind, ProjectID: "10", Repo: "fixture repository", Kind: kind,
			Ref: "42", HeadSHA: reviewFixtureHead, BaseSHA: reviewFixtureBase,
			Status: "completed", PublishStatus: "pending", PolicyJSON: string(policyJSON), ReportJSON: `{"summary":"fixture"}`,
		}
		if err := db.DB.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
		w := reviewRequest(srv, token, "POST", fmt.Sprintf("/api/code-reviews/%d/sync", run.ID), "")
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "全局") || !strings.Contains(w.Body.String(), "已关闭") {
			t.Fatalf("manual %s sync did not explain global gate: %d/%s", kind, w.Code, w.Body.String())
		}
	}
}

func TestCodeReviewPolicySaveClearsLegacyHiddenScope(t *testing.T) {
	srv, token := reviewCommentSettingsServer(t)
	w := reviewRequest(srv, token, "PUT", "/api/code-reviews/policy", `{"project_id":"10","domain":"fms","scenario":"dispatch","knowledge_scope":"legacy-project","rules":"shared architecture"}`)
	var p db.CodeReviewPolicy
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || w.Code != http.StatusOK {
		t.Fatalf("save legacy client policy: %d/%s err=%v", w.Code, w.Body.String(), err)
	}
	if p.Domain != "general" || p.Scenario != "general" || p.KnowledgeScope != codereview.SystemCorpusScope || p.Rules != "shared architecture" {
		t.Fatalf("hidden legacy fields still apply: %+v", p)
	}
	var stored db.CodeReviewPolicy
	if err := db.DB.First(&stored, "project_id = ?", "10").Error; err != nil {
		t.Fatal(err)
	}
	if stored.Domain != p.Domain || stored.Scenario != p.Scenario || stored.KnowledgeScope != p.KnowledgeScope {
		t.Fatalf("saved policy differs from fixed runtime contract: %+v", stored)
	}
}

// Match the published PostgreSQL schema rather than deriving it from the Go
// model: AutoMigrate would conceal acronym-to-column naming regressions.
func TestCodeReviewControlsPersistAgainstPublishedSchema(t *testing.T) {
	srv, token := reviewCommentSettingsServer(t)
	usePublishedReviewPolicySchema(t)
	fields := []string{"sync_commits", "sync_mrs", "review_commits", "review_mrs"}
	for _, enabled := range []bool{false, true} {
		for _, field := range fields {
			body := fmt.Sprintf(`{"%s":%t}`, field, enabled)
			w := reviewRequest(srv, token, "PATCH", "/api/code-reviews/comment-settings", body)
			if w.Code != http.StatusOK {
				t.Fatalf("global %s=%t save=%d/%s", field, enabled, w.Code, w.Body.String())
			}
			w = reviewRequest(srv, token, "GET", "/api/code-reviews/comment-settings", "")
			var saved map[string]bool
			if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || w.Code != http.StatusOK || saved[field] != enabled {
				t.Fatalf("global %s=%t persisted=%d/%s err=%v", field, enabled, w.Code, w.Body.String(), err)
			}
		}
	}
	w := reviewRequest(srv, token, "PUT", "/api/code-reviews/policy", `{"project_id":"10","rules":"keep repository rules"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("initial repository rules=%d/%s", w.Code, w.Body.String())
	}
	for _, enabled := range []bool{true, false} {
		for _, field := range fields {
			body := fmt.Sprintf(`{"project_id":"10","%s":%t}`, field, enabled)
			w = reviewRequest(srv, token, "PUT", "/api/code-reviews/policy", body)
			if w.Code != http.StatusOK {
				t.Fatalf("repository %s=%t save=%d/%s", field, enabled, w.Code, w.Body.String())
			}
			w = reviewRequest(srv, token, "GET", "/api/code-reviews/repos", "")
			var saved struct {
				Repos []struct {
					Policy map[string]any `json:"policy"`
				} `json:"repos"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || w.Code != http.StatusOK || len(saved.Repos) != 1 || saved.Repos[0].Policy[field] != enabled || saved.Repos[0].Policy["rules"] != "keep repository rules" {
				t.Fatalf("repository %s=%t persisted=%d/%s err=%v", field, enabled, w.Code, w.Body.String(), err)
			}
		}
	}
}

func usePublishedReviewPolicySchema(t *testing.T) {
	t.Helper()
	for _, query := range []string{
		`DROP TABLE code_review_policies`,
		`CREATE TABLE code_review_policies (
		 project_id varchar(160) PRIMARY KEY, auto_review boolean,
		 sync_commits boolean, sync_m_rs boolean, domain varchar(32),
		 scenario varchar(64), knowledge_scope varchar(160), rules text,
		 updated_at datetime, review_commits boolean, review_mrs boolean)`,
	} {
		if err := db.DB.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
}
