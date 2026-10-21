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
	"well-ambient/internal/telemetry"
)

func TestGitLabConnectionSecretsBindToIDAndDestination(t *testing.T) {
	current := config.Config{GitLab: config.GitLabConfig{BaseURL: "https://devgit.example.test", APIToken: "dev-secret", Secret: "dev-hook", Connections: []config.GitLabConnection{{ID: "fde", BaseURL: "https://fde.example.test", APIToken: "fde-secret", Secret: "fde-hook"}}}}
	next := current
	next.GitLab.Connections = append([]config.GitLabConnection(nil), current.GitLab.Connections...)
	redactConfiguredSecrets(&next)
	encoded, _ := json.Marshal(next)
	if strings.Contains(string(encoded), "fde-secret") || current.GitLab.Connections[0].APIToken != "fde-secret" {
		t.Fatal("redaction leaked or mutated live credentials")
	}
	mergeConfiguredSecrets(&next, current)
	if next.GitLab.APIToken != "dev-secret" || next.GitLab.Connections[0].APIToken != "fde-secret" {
		t.Fatal("instance credentials were not restored independently")
	}
	next.GitLab.Connections[0].BaseURL = "https://other.example.test"
	next.GitLab.Connections[0].APIToken = configuredSecretPlaceholder
	next.GitLab.Connections[0].Secret = configuredSecretPlaceholder
	mergeConfiguredSecrets(&next, current)
	if next.GitLab.Connections[0].APIToken != "" || next.GitLab.Connections[0].Secret != "" {
		t.Fatal("credentials inherited after destination change")
	}
	req := ConnectionTestRequest{InstanceID: "fde", GitLab: &config.GitLabConfig{BaseURL: "https://fde.example.test", APIToken: configuredSecretPlaceholder}}
	mergeConnectionTestSecrets(&req, current)
	if req.GitLab.APIToken != "fde-secret" {
		t.Fatal("health check reused default credentials")
	}
}

func TestExecutionEvidenceKeepsFrozenSourceAndWebhookIdentity(t *testing.T) {
	setupServerTestDB(t)
	source := config.RepositorySource{InstanceID: "fde", ProjectID: "fde:10", RepositoryKey: "gitlab:https://fde.example.test:10", FullPath: "project-code/original/service", Name: "service"}
	task := db.TaskTelemetry{TaskID: "FDE-10", Title: "Source evidence"}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	run := db.ExecutionRun{DemandID: task.TaskID, ProjectRef: "fde:10", Repo: "service", TopicBranch: "ai/frozen", ServiceIdentity: "fixture", RepositorySourceJSON: source.JSON()}
	if err := (&Server{}).recordExecutionEvidence(run, gitLabCommit{ID: "same-sha"}); err != nil {
		t.Fatal(err)
	}
	var logs []db.GitCommitLog
	db.DB.Find(&logs)
	if len(logs) != 3 {
		t.Fatalf("evidence rows=%d", len(logs))
	}
	for _, row := range logs {
		if row.RepositoryKey != source.RepositoryKey || row.RepositorySourceJSON != source.JSON() || row.Repo != "fde / project-code/original/service" {
			t.Fatal("frozen evidence source missing")
		}
	}
	inserted, err := telemetry.PersistGitPushEvidence(db.DB, db.GitCommitLog{Repo: "fde / project-code/renamed/service", CommitID: "same-sha", Action: "git_push", RepositoryKey: source.RepositoryKey, RepositorySourceJSON: source.JSON()})
	if err != nil || inserted {
		t.Fatalf("webhook duplicated active delivery evidence: inserted=%v err=%v", inserted, err)
	}
}

func TestStrongestBrainKeepsRepositorySourceSnapshots(t *testing.T) {
	logs := []db.GitCommitLog{{TaskID: "NS2-1", Repo: "fde / project-code/a/service", RepositoryKey: "fde:10", RepositorySourceJSON: `{"name":"original-a"}`, CommitID: "same", Action: "git_push", Branch: "main", CreatedAt: time.Now()}, {TaskID: "NS2-1", Repo: "fde / project-code/b/service", RepositoryKey: "fde:11", RepositorySourceJSON: `{"name":"original-b"}`, CommitID: "same", Action: "git_push", Branch: "main", CreatedAt: time.Now()}}
	digest := buildEvidenceDigest(logs)
	if len(digest) != 1 || len(digest[0].RepositorySources) != 2 || !strings.Contains(strings.Join(digest[0].Signals, " "), "project-code/a/service") {
		t.Fatal("brain digest lost source names or snapshots")
	}
	if strongestBrainLogEvidenceRef(logs[0]) == strongestBrainLogEvidenceRef(logs[1]) {
		t.Fatal("brain evidence references collided")
	}
}

func TestDefaultWebhookManagementRejectsForeignMappings(t *testing.T) {
	for _, repo := range []config.RepoMapping{{ProjectID: "fde:10", InstanceID: "fde", RemoteProjectID: "10"}, {ProjectID: "fde:10"}} {
		if _, err := scopedWebhookRepos(config.GitLabConfig{}, []config.RepoMapping{repo}); err == nil {
			t.Fatal("default connection accepted foreign repository")
		}
	}
	if _, err := scopedWebhookRepos(config.GitLabConfig{}, []config.RepoMapping{{ProjectID: "10"}, {Path: "core/service"}}); err != nil {
		t.Fatal(err)
	}
}

func TestScopedWebhookRejectsWrongInstanceBeforeWrites(t *testing.T) {
	setupServerTestDB(t)
	cfg := config.Config{GitLab: config.GitLabConfig{Secret: "dev-hook", Connections: []config.GitLabConnection{{ID: "fde", Enabled: true, BaseURL: "https://fde.example.test", Secret: "fde-hook", RootGroup: "project-code"}}, Repos: []config.RepoMapping{{Name: "service", Path: "project-code/a/service", InstanceID: "fde", RemoteProjectID: "10", ProjectID: "fde:10"}}}}
	s := NewServer(&cfg, "unused")
	for _, test := range []struct{ secret, body string }{{"dev-hook", `{"project":{"id":10,"web_url":"https://fde.example.test/project-code/a/service"}}`}, {"fde-hook", `{"project":{"id":10,"web_url":"https://devgit.example.test/project-code/a/service"}}`}, {"fde-hook", `{"project":{"id":11,"web_url":"https://fde.example.test/project-code/a/service"}}`}} {
		r := httptest.NewRequest(http.MethodPost, "/api/webhook/gitlab/fde", strings.NewReader(test.body))
		r.Header.Set("X-Gitlab-Token", test.secret)
		r.Header.Set("X-Gitlab-Event", "Push Hook")
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatalf("invalid source accepted: %d", w.Code)
		}
	}
	var count int64
	db.DB.Model(&db.WebhookLog{}).Count(&count)
	if count != 0 {
		t.Fatal("invalid source wrote a webhook record")
	}
}
