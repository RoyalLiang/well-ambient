package jiraissuesync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"well-ambient/internal/db"
)

type workspaceFixture struct {
	mu               sync.Mutex
	server           *httptest.Server
	s                *Workspace
	issues           map[string]gitIssue
	comments         []string
	closed           bool
	merged           map[string]bool
	pipelineSHA      string
	pipelineStatus   string
	commentFail      bool
	transitionFail   bool
	ambiguousIssue   bool
	creates          int
	transitions      int
	posts            int
	aiCalls          int
	pipelineSource   string
	jiraDescription  string
	honorDone        bool
	readbackFailures int
	mergeSHA         string
}

func workspaceTest(t *testing.T) *workspaceFixture {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:workspace-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.AutoMigrate(&db.JiraIssueWorkspace{}, &db.JiraIssueRun{}, &db.JiraIssueLink{}); err != nil {
		t.Fatal(err)
	}
	sql, _ := conn.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	f := &workspaceFixture{issues: map[string]gitIssue{}, merged: map[string]bool{"42": true, "43": true}, pipelineSHA: strings.Repeat("a", 40), pipelineStatus: "success", mergeSHA: strings.Repeat("a", 40), pipelineSource: "push", jiraDescription: "verify dispatch"}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	f.s = &Workspace{DB: conn, Key: []byte(strings.Repeat("k", 32)), Client: f.server.Client()}
	f.s.AI = func(ctx context.Context, system, user string) (string, error) {
		f.aiCalls++
		if !strings.Contains(user, "dispatch vehicles") || !strings.Contains(user, strings.Repeat("a", 40)) {
			t.Error("AI did not receive pinned source")
		}
		return `{"proposals":[{"project":"NS2","repo_ids":["42","43"],"task_keys":["NS2-1"],"reason":"dispatch vehicles source","evidence":[{"repo_id":"42","path":"README.md","sha":"` + strings.Repeat("a", 40) + `"},{"repo_id":"43","path":"README.md","sha":"` + strings.Repeat("a", 40) + `"}]}]}`, nil
	}
	return f
}
func (f *workspaceFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	p := r.URL.Path
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if strings.HasPrefix(p, "/api/v4/") {
		if r.Header.Get("PRIVATE-TOKEN") != "dedicated-gl" {
			http.Error(w, "bad token", 403)
			return
		}
	} else if strings.HasPrefix(p, "/rest/api/2/") {
		if r.Header.Get("Authorization") != "Bearer dedicated-jira" {
			http.Error(w, "bad token", 403)
			return
		}
	}
	repo := func(id int64) Repo {
		return Repo{ID: id, Path: fmt.Sprintf("ops/repo%d", id), DefaultBranch: "main", WebURL: f.server.URL + fmt.Sprintf("/ops/repo%d", id), IssuesEnabled: true}
	}
	if p == "/api/v4/groups/ops/projects" {
		write([]Repo{repo(42), repo(43), {ID: 99, Path: "other/shared", DefaultBranch: "main", IssuesEnabled: true}})
		return
	}
	if p == "/rest/api/2/project" {
		write([]Project{{Key: "NS2", Name: "Terminal"}, {Key: "WA", Name: "Platform"}})
		return
	}
	if p == "/rest/api/2/search" {
		var query struct {
			JQL string `json:"jql"`
		}
		_ = json.NewDecoder(r.Body).Decode(&query)
		if f.honorDone && f.closed && !strings.Contains(query.JQL, "key in") {
			write(map[string]any{"startAt": 0, "total": 0, "issues": []any{}})
			return
		}

		write(map[string]any{"startAt": 0, "total": 1, "issues": []any{map[string]any{"key": "NS2-1", "fields": map[string]any{"summary": "dispatch vehicles", "description": f.jiraDescription, "status": map[string]any{"name": "In progress", "statusCategory": map[string]string{"key": map[bool]string{true: "done", false: "indeterminate"}[f.closed]}}}}}})
		return
	}
	if strings.Contains(p, "/repository/commits/") {
		write(map[string]string{"id": strings.Repeat("a", 40)})
		return
	}
	if strings.HasSuffix(p, "/repository/tree") {
		write([]map[string]string{{"type": "blob", "path": "README.md"}, {"type": "blob", "path": "secret.go"}})
		return
	}
	if strings.Contains(p, "/repository/files/") {
		if r.URL.Query().Get("ref") != strings.Repeat("a", 40) {
			http.Error(w, "unpinned", 400)
			return
		}
		write(map[string]string{"encoding": "base64", "content": "ZGlzcGF0Y2ggdmVoaWNsZXM="})
		return
	}
	for _, id := range []string{"42", "43"} {
		prefix := "/api/v4/projects/" + id
		if p == prefix {
			n := int64(42)
			if id == "43" {
				n = 43
			}
			write(repo(n))
			return
		}
		if p == prefix+"/issues" {
			if r.Method == "POST" {
				var payload struct{ Title, Description string }
				_ = json.NewDecoder(r.Body).Decode(&payload)
				f.creates++
				n := int64(42)
				if id == "43" {
					n = 43
				}
				issue := gitIssue{ID: n + 100, IID: 1, ProjectID: n, Description: payload.Description, WebURL: f.server.URL + "/" + id + "/issues/1", State: "opened"}
				f.issues[id] = issue
				if f.ambiguousIssue {
					http.Error(w, "lost acknowledgement dedicated-gl", 502)
					return
				}
				write(issue)
				return
			}
			if issue, ok := f.issues[id]; ok {
				write([]gitIssue{issue})
			} else {
				write([]gitIssue{})
			}
			return
		}
		if p == prefix+"/issues/1" {
			issue := f.issues[id]
			if r.Method == "PUT" {
				var payload struct{ Description string }
				_ = json.NewDecoder(r.Body).Decode(&payload)
				issue.Description = payload.Description
				f.issues[id] = issue
			}
			write(issue)
			return
		}
		if p == prefix+"/issues/1/closed_by" {
			if f.merged[id] {
				n := int64(42)
				if id == "43" {
					n = 43
				}
				write([]mergeRequest{{IID: 7, ProjectID: n, State: "merged"}})
			} else {
				write([]mergeRequest{})
			}
			return
		}
		if p == prefix+"/merge_requests/7" {
			n := int64(42)
			if id == "43" {
				n = 43
			}
			now := time.Now()
			write(mergeRequest{IID: 7, ProjectID: n, State: "merged", TargetBranch: "main", MergeCommitSHA: f.mergeSHA, MergedAt: &now, WebURL: f.server.URL + "/" + id + "/merge_requests/7"})
			return
		}
		if p == prefix+"/merge_requests/7/closes_issues" {
			write([]gitIssue{f.issues[id]})
			return
		}
		if p == prefix+"/pipelines" {
			if r.URL.Query().Get("sha") != f.mergeSHA || r.URL.Query().Get("ref") != "main" {
				http.Error(w, "wrong proof query", 400)
				return
			}
			write([]pipeline{{Source: f.pipelineSource, ID: 51, SHA: f.pipelineSHA, Ref: "main", Status: f.pipelineStatus, WebURL: f.server.URL + "/" + id + "/pipelines/51"}})
			return
		}
	}
	if p == "/rest/api/2/issue/NS2-1/comment" {
		if r.Method == "POST" {
			f.posts++
			var payload struct{ Body string }
			_ = json.NewDecoder(r.Body).Decode(&payload)
			f.comments = append(f.comments, payload.Body)
			if f.commentFail {
				http.Error(w, "lost acknowledgement dedicated-jira", 502)
				return
			}
			write(map[string]string{"id": "1"})
			return
		}
		rows := []map[string]string{}
		for _, v := range f.comments {
			rows = append(rows, map[string]string{"body": v})
		}
		write(map[string]any{"startAt": 0, "total": len(rows), "comments": rows})
		return
	}
	if p == "/rest/api/2/issue/NS2-1/transitions" {
		if r.Method == "POST" {
			f.transitions++
			if !f.transitionFail {
				f.closed = true
			}
			if f.transitionFail {
				http.Error(w, "workflow denied", 400)
				return
			}
			w.WriteHeader(204)
			return
		}
		write(map[string]any{"transitions": []any{map[string]any{"id": "31", "to": map[string]any{"statusCategory": map[string]string{"key": "done"}}}}})
		return
	}
	if p == "/rest/api/2/issue/NS2-1" {
		if f.closed && f.readbackFailures > 0 {
			f.readbackFailures--
			http.Error(w, "lost readback", 502)
			return
		}

		key := "indeterminate"
		if f.closed {
			key = "done"
		}
		write(map[string]any{"key": "NS2-1", "fields": map[string]any{"status": map[string]any{"statusCategory": map[string]string{"key": key}}}})
		return
	}
	http.Error(w, "unexpected route "+p, 404)
}
func (f *workspaceFixture) setup(t *testing.T) WorkspaceView {
	t.Helper()
	ctx := context.Background()
	v, err := f.s.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := v.Settings
	c.GroupURL = f.server.URL + "/ops"
	c.JiraURL = f.server.URL
	c.GitLabToken = "dedicated-gl"
	c.JiraToken = "dedicated-jira"
	v, err = f.s.Save(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	f.run(t, v.Settings.Version, "discover")
	v, _ = f.s.View(ctx)
	c = v.Settings
	c.Mappings = []Mapping{{Project: "NS2", RepoIDs: []string{"42", "43"}, TaskKeys: []string{"NS2-1"}}}
	v, err = f.s.Save(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func (f *workspaceFixture) run(t *testing.T, version int, kind string) RunView {
	t.Helper()
	run, lease, err := f.s.Begin(context.Background(), version, kind)
	if err != nil {
		t.Fatal(err)
	}
	f.s.Execute(context.Background(), run, lease, []string{"42", "43"})
	v, err := f.s.View(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return v.Runs[0]
}
func TestWorkspaceMultiRepoEvidenceAndIdempotency(t *testing.T) {
	f := workspaceTest(t)
	v := f.setup(t)
	if len(v.Catalog.Repositories) != 2 {
		t.Fatal("shared repo escaped namespace")
	}
	run := f.run(t, v.Settings.Version, "analyze")
	if run.Status != "succeeded" || len(run.Proposals) != 1 || f.aiCalls != 1 {
		t.Fatalf("analysis %+v", run)
	}
	if run.GroupURL != f.server.URL+"/ops" || run.JiraURL != f.server.URL || run.AnalysisCatalog == nil || len(run.AnalysisCatalog.Repositories) != 2 || len(run.TaskDetails) == 0 {
		t.Fatalf("analysis lost immutable source or Jira details: %+v", run)
	}
	run = f.run(t, v.Settings.Version, "sync")
	if run.Status != "succeeded" || !f.closed || f.posts != 1 || f.creates != 2 || f.transitions != 1 {
		t.Fatalf("sync %+v closed=%v", run, f.closed)
	}
	if len(run.Items[0].Repositories) != 2 || !run.Items[0].Commented || !run.Items[0].Closed {
		t.Fatal("missing granular proof")
	}
	run = f.run(t, v.Settings.Version, "sync")
	if run.Status != "succeeded" || f.posts != 1 || f.creates != 2 || f.transitions != 1 {
		t.Fatalf("duplicate side effect %+v", run)
	}

}
func TestWorkspaceBlocksIncompleteEvidence(t *testing.T) {
	for _, mode := range []string{"one-unmerged", "wrong-sha", "failed-pipeline"} {
		t.Run(mode, func(t *testing.T) {
			f := workspaceTest(t)
			v := f.setup(t)
			switch mode {
			case "one-unmerged":
				f.merged["43"] = false
			case "wrong-sha":
				f.pipelineSHA = strings.Repeat("b", 40)
			case "failed-pipeline":
				f.pipelineStatus = "failed"
			}
			run := f.run(t, v.Settings.Version, "sync")
			if f.closed || f.posts != 0 || f.transitions != 0 || run.Items[0].Closed {
				t.Fatalf("closed without proof: %+v", run)
			}
		})
	}
}
func TestWorkspaceReconcilesAmbiguousWritesAndPartialFailure(t *testing.T) {
	for _, mode := range []string{"issue", "comment", "transition"} {
		t.Run(mode, func(t *testing.T) {
			f := workspaceTest(t)
			v := f.setup(t)
			switch mode {
			case "issue":
				f.ambiguousIssue = true
			case "comment":
				f.commentFail = true
			case "transition":
				f.transitionFail = true
			}
			first := f.run(t, v.Settings.Version, "sync")
			if first.Status != "failed" || f.closed {
				t.Fatalf("unexpected first run %+v", first)
			}
			f.ambiguousIssue = false
			f.commentFail = false
			f.transitionFail = false
			second := f.run(t, v.Settings.Version, "sync")
			if second.Status != "succeeded" || !f.closed || f.creates != 2 || f.posts != 1 {
				t.Fatalf("failed reconciliation: %+v creates=%d posts=%d", second, f.creates, f.posts)
			}
		})
	}
}
func TestWorkspaceCredentialsVersionLeaseAndCancellation(t *testing.T) {
	f := workspaceTest(t)
	v := f.setup(t)
	ctx := context.Background()
	var row db.JiraIssueWorkspace
	f.s.DB.First(&row, 1)
	if strings.Contains(row.Credentials, "dedicated") || row.Credentials == "" {
		t.Fatal("plaintext credentials")
	}
	serialized := marshal(v)
	if strings.Contains(serialized, "dedicated") {
		t.Fatal("GET exposed credentials")
	}
	c := v.Settings
	c.GroupURL += "/other"
	if _, err := f.s.Save(ctx, c); err == nil {
		t.Fatal("forwarded old credentials")
	}
	c = v.Settings
	c.Version--
	if _, err := f.s.Save(ctx, c); err == nil {
		t.Fatal("stale save")
	}
	run, lease, err := f.s.Begin(ctx, v.Settings.Version, "sync")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.s.Begin(ctx, v.Settings.Version, "discover"); err == nil {
		t.Fatal("concurrent run accepted")
	}
	if _, err = f.s.Save(ctx, v.Settings); err == nil {
		t.Fatal("changed active config")
	}
	if err = f.s.Cancel(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	f.s.Execute(ctx, run, lease, nil)
	if f.creates != 0 || f.posts != 0 {
		t.Fatal("cancelled run mutated remote")
	}
}
func TestWorkspaceRejectsAIInventedEvidence(t *testing.T) {
	f := workspaceTest(t)
	v := f.setup(t)
	f.s.AI = func(context.Context, string, string) (string, error) {
		return `{"proposals":[{"project":"NS2","repo_ids":["99"],"task_keys":["NS2-1"],"reason":"guess","evidence":[{"repo_id":"99","path":"README.md","sha":"bad"}]}]}`, nil
	}
	r := f.run(t, v.Settings.Version, "analyze")
	if r.Status != "failed" || len(r.Proposals) > 0 || f.creates != 0 {
		t.Fatalf("accepted invented source: %+v", r)
	}
}
func TestWorkspaceRejectsRedirectAndMissingKey(t *testing.T) {
	f := workspaceTest(t)
	v, _ := f.s.View(context.Background())
	c := v.Settings
	c.GroupURL = f.server.URL + "/ops"
	c.JiraURL = f.server.URL
	c.GitLabToken = "secret"
	f.s.Key = nil
	if _, err := f.s.Save(context.Background(), c); err == nil {
		t.Fatal("saved without encryption key")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("credential forwarded to redirect target") }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer redirect.Close()
	c.GroupURL = redirect.URL + "/ops"
	c.JiraURL = redirect.URL
	remote, err := newRemote(c, credentials{GitLab: "secret"}, redirect.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = remote.request(context.Background(), false, "GET", "/project", nil, nil); err == nil {
		t.Fatal("followed credential redirect")
	}
}

func TestWorkspaceKnownIdentityAndOldCancellation(t *testing.T) {
	f := workspaceTest(t)
	f.merged["43"] = false
	v := f.setup(t)
	f.run(t, v.Settings.Version, "sync")
	f.mu.Lock()
	for id, issue := range f.issues {
		issue.Description = "user edited away marker"
		f.issues[id] = issue
	}
	f.mu.Unlock()
	run := f.run(t, v.Settings.Version, "sync")
	if run.Status != "failed" || f.creates != 2 {
		t.Fatal("lost marker created a duplicate")
	}
	ctx := context.Background()
	old, _, err := f.s.Begin(ctx, v.Settings.Version, "sync")
	if err != nil {
		t.Fatal(err)
	}
	f.s.DB.Model(&db.JiraIssueWorkspace{}).Where("id = 1").Update("lease_until", time.Now().UTC().Add(-time.Minute))
	current, lease, err := f.s.Begin(ctx, v.Settings.Version, "discover")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.Cancel(ctx, old.ID); err == nil {
		t.Fatal("old run cancelled new lease")
	}
	f.s.Execute(ctx, current, lease, nil)
	view, _ := f.s.View(ctx)
	if view.Runs[0].Status != "succeeded" {
		t.Fatal("new run was cancelled")
	}
}

func TestWorkspaceRecoversDoneReadbackAndNoOp(t *testing.T) {
	f := workspaceTest(t)
	v := f.setup(t)
	c := v.Settings
	c.Mappings[0].TaskKeys = nil
	v, err := f.s.Save(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	f.honorDone = true
	f.readbackFailures = 1
	first := f.run(t, v.Settings.Version, "sync")
	if first.Status != "failed" || !f.closed || first.Items[0].Closed {
		t.Fatalf("expected unconfirmed done: %+v", first)
	}
	second := f.run(t, v.Settings.Version, "sync")
	if second.Status != "succeeded" || len(second.Items) != 1 || !second.Items[0].Closed || f.posts != 1 {
		t.Fatalf("failed done reconciliation: %+v", second)
	}
	third := f.run(t, v.Settings.Version, "sync")
	if third.Status != "succeeded" || len(third.Items) != 0 {
		t.Fatalf("empty active scope should succeed: %+v", third)
	}
}
func TestWorkspaceReservedMarkersAndPipelineSource(t *testing.T) {
	f := workspaceTest(t)
	v := f.setup(t)
	f.pipelineSource = "schedule"
	f.jiraDescription = "sample <!-- /well-ambient:jira-issue --> content"
	f.run(t, v.Settings.Version, "sync")
	if f.closed || f.posts != 0 {
		t.Fatal("untrusted pipeline closed Jira")
	}
	before := f.issues["42"].Description
	f.run(t, v.Settings.Version, "sync")
	if f.issues["42"].Description != before {
		t.Fatal("reserved source marker broke managed block idempotency")
	}
	f.pipelineSource = "push"
	f.run(t, v.Settings.Version, "sync")
	if !f.closed {
		t.Fatal("valid push evidence rejected")
	}
}
func TestWorkspaceRejectsAlteredCommentEvidence(t *testing.T) {
	f := workspaceTest(t)
	v := f.setup(t)
	f.transitionFail = true
	f.run(t, v.Settings.Version, "sync")
	f.transitionFail = false
	body := f.comments[0]
	marker := body[strings.Index(body, "[well-ambient:verified:"):]
	f.comments[0] = marker
	run := f.run(t, v.Settings.Version, "sync")
	if run.Status != "failed" || f.closed || f.posts != 1 {
		t.Fatalf("altered comment accepted: %+v", run)
	}
}

func TestWorkspaceReopenRequiresNewCodeAndRecovers(t *testing.T) {
	f := workspaceTest(t)
	v := f.setup(t)
	c := v.Settings
	c.Mappings[0].TaskKeys = nil
	v, err := f.s.Save(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	f.honorDone = true
	f.run(t, v.Settings.Version, "sync")
	f.closed = false
	old := f.run(t, v.Settings.Version, "sync")
	if old.Status != "failed" || f.closed || f.transitions != 1 {
		t.Fatal("reused old evidence to close reopened Jira")
	}
	f.mergeSHA = strings.Repeat("b", 40)
	f.pipelineSHA = f.mergeSHA
	f.readbackFailures = 1
	uncertain := f.run(t, v.Settings.Version, "sync")
	if uncertain.Status != "failed" || !f.closed || uncertain.Items[0].Closed {
		t.Fatalf("expected lost acknowledgement: %+v", uncertain)
	}
	confirmed := f.run(t, v.Settings.Version, "sync")
	if confirmed.Status != "succeeded" || !confirmed.Items[0].Closed || f.posts != 2 {
		t.Fatalf("reopened task not reconciled: %+v", confirmed)
	}
}

func TestWorkspaceCancelsInFlightAI(t *testing.T) {
	f := workspaceTest(t)
	v := f.setup(t)
	started := make(chan struct{})
	finished := make(chan struct{})
	f.s.AI = func(ctx context.Context, _, _ string) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}
	run, lease, err := f.s.Begin(context.Background(), v.Settings.Version, "analyze")
	if err != nil {
		t.Fatal(err)
	}
	go func() { f.s.Execute(context.Background(), run, lease, []string{"42"}); close(finished) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("AI never started")
	}
	if err = f.s.Cancel(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not interrupt in-flight AI")
	}
	view, _ := f.s.View(context.Background())
	if view.Busy || view.Runs[0].Status != "cancelled" || f.creates != 0 {
		t.Fatal("cancelled operation did not clean up")
	}
}
