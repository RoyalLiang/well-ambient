package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"well-ambient/internal/db"
	"well-ambient/internal/jiraissuesync"
)

// Synthetic integrations: this fixture never contacts production Jira or GitLab.
func configureJiraIssueBrowserFixture(t *testing.T, s *Server) {
	t.Helper()
	t.Setenv("WELL_JIRA_ISSUE_SYNC_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("f", 32))))
	var mu sync.Mutex
	issues := map[string]map[string]any{}
	comments := []map[string]string{}
	closed := false
	mode := "success"
	var remote *httptest.Server
	remote = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		send := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		p := r.URL.Path
		if p == "/__scenario" {
			mode = r.URL.Query().Get("mode")
			closed = false
			send(map[string]string{"mode": mode})
			return
		}
		if mode == "bad-credentials" {
			http.Error(w, "synthetic permission denied", 403)
			return
		}
		repo := func(id int) map[string]any {
			return map[string]any{"id": id, "path_with_namespace": fmt.Sprintf("ops/%s", map[int]string{42: "terminal", 43: "fleet"}[id]), "default_branch": "main", "issues_enabled": true, "web_url": remote.URL + fmt.Sprintf("/projects/%d", id)}
		}
		switch p {
		case "/api/v4/groups/ops/projects":
			if total, err := strconv.Atoi(os.Getenv("WELL_JIRA_SYNC_FIXTURE_CATALOG_SIZE")); err == nil && total >= 0 && total <= 5001 {
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if page < 1 {
					page = 1
				}
				start := (page - 1) * 100
				end := min(start+100, total)
				items := []any{}
				for i := start; i < end; i++ {
					if i < 2 {
						items = append(items, repo(42+i))
					} else {
						items = append(items, map[string]any{"id": 42 + i, "path_with_namespace": fmt.Sprintf("ops/repository-%04d", i+1), "default_branch": "main", "issues_enabled": true})
					}
				}
				if end < total {
					w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
				}
				send(items)
			} else {
				send([]any{repo(42), repo(43)})
			}
			return
		case "/rest/api/2/project":
			send([]map[string]string{{"key": "NS2", "name": "南沙二期"}, {"key": "WA", "name": "Well Ambient"}})
			return
		case "/rest/api/2/search":
			send(map[string]any{"startAt": 0, "total": 1, "issues": []any{map[string]any{"key": "NS2-1", "fields": map[string]any{"summary": "车辆反馈与资源锁验证", "description": "隔离验收任务：核对车辆反馈与资源锁。", "status": map[string]string{"name": "处理中"}}}}})
			return
		}
		for _, id := range []int{42, 43} {
			prefix := fmt.Sprintf("/api/v4/projects/%d", id)
			key := fmt.Sprint(id)
			switch p {
			case prefix:
				send(repo(id))
				return
			case prefix + "/repository/commits/main":
				send(map[string]string{"id": strings.Repeat("a", 40)})
				return
			case prefix + "/repository/tree":
				send([]map[string]string{{"path": "README.md", "type": "blob"}, {"path": "src/dispatch.ts", "type": "blob"}})
				return
			case prefix + "/issues":
				if r.Method == "POST" {
					var data map[string]any
					_ = json.NewDecoder(r.Body).Decode(&data)
					issues[key] = map[string]any{"id": id + 100, "iid": 1, "project_id": id, "description": data["description"], "web_url": remote.URL + prefix + "/issues/1"}
					send(issues[key])
				} else if v, ok := issues[key]; ok {
					send([]any{v})
				} else {
					send([]any{})
				}
				return
			case prefix + "/issues/1":
				if r.Method == "PUT" {
					var data map[string]any
					_ = json.NewDecoder(r.Body).Decode(&data)
					issues[key]["description"] = data["description"]
				}
				send(issues[key])
				return
			case prefix + "/issues/1/closed_by":
				if mode == "partial" && id == 43 {
					send([]any{})
					return
				}
				send([]map[string]any{{"iid": 7, "project_id": id, "state": "merged"}})
				return
			case prefix + "/merge_requests/7":
				send(map[string]any{"iid": 7, "project_id": id, "state": "merged", "merged_at": time.Now().UTC(), "target_branch": "main", "merge_commit_sha": strings.Repeat("a", 40), "web_url": remote.URL + prefix + "/merge_requests/7"})
				return
			case prefix + "/merge_requests/7/closes_issues":
				send([]any{issues[key]})
				return
			case prefix + "/pipelines":
				send([]map[string]any{{"id": 51, "source": "push", "sha": strings.Repeat("a", 40), "ref": "main", "status": map[bool]string{true: "failed", false: "success"}[mode == "failed-validation"], "web_url": remote.URL + prefix + "/pipelines/51"}})
				return
			}
			if strings.HasPrefix(p, prefix+"/repository/files/") {
				send(map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("车辆反馈与资源锁验证 dispatch vehicles locking NS2"))})
				return
			}
		}
		switch p {
		case "/rest/api/2/issue/NS2-1/comment":
			if r.Method == "POST" {
				var v map[string]string
				_ = json.NewDecoder(r.Body).Decode(&v)
				comments = append(comments, v)
				send(map[string]string{"id": "1"})
			} else {
				send(map[string]any{"startAt": 0, "total": len(comments), "comments": comments})
			}
			return
		case "/rest/api/2/issue/NS2-1/transitions":
			if r.Method == "POST" {
				if mode == "transition-failed" {
					http.Error(w, "synthetic workflow denied", 400)
					return
				}
				closed = true
				w.WriteHeader(204)
			} else {
				send(map[string]any{"transitions": []any{map[string]any{"id": "31", "to": map[string]any{"statusCategory": map[string]string{"key": "done"}}}}})
			}
			return
		case "/rest/api/2/issue/NS2-1":
			category := "indeterminate"
			if closed {
				category = "done"
			}
			send(map[string]any{"fields": map[string]any{"status": map[string]any{"statusCategory": map[string]string{"key": category}}}})
			return
		}
		http.Error(w, "synthetic route unavailable", 404)
	}))
	t.Cleanup(remote.Close)
	s.jiraIssueAnalyze = func(ctx context.Context, system, user string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(12 * time.Second):
		}
		return `{"proposals":[{"project":"NS2","repo_ids":["42","43"],"task_keys":["NS2-1"],"reason":"两个仓库的调度与车辆反馈代码共同覆盖资源锁任务；仅检查本次抽样文件。","evidence":[{"repo_id":"42","path":"src/dispatch.ts","sha":"` + strings.Repeat("a", 40) + `"},{"repo_id":"43","path":"README.md","sha":"` + strings.Repeat("a", 40) + `"}]}]}`, nil
	}
	if os.Getenv("WELL_JIRA_SYNC_FIXTURE_SEED") == "1" {
		svc := s.jiraIssueWorkspace()
		view, err := svc.View(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		c := view.Settings
		c.GroupURL = remote.URL + "/ops"
		c.JiraURL = remote.URL
		c.GitLabToken = "synthetic-gitlab"
		c.JiraToken = "synthetic-jira"
		view, err = svc.Save(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		if os.Getenv("WELL_REVIEW_MAPPING_BROWSER_FIXTURE") == "1" {
			for i, kind := range []string{"mr", "commit", "mr", "commit"} {
				ref := "7"
				if kind == "commit" {
					ref = strings.Repeat(strconv.Itoa(i+1), 40)
				}
				run := db.CodeReviewRun{Key: "review-browser-" + strconv.Itoa(i), ProjectID: "42", Repo: "ops/terminal", RepositorySourceJSON: `{"namespace_name":"港口项目","namespace_path":"ops","full_path":"ops/terminal"}`, Kind: kind, Ref: ref, Author: []string{"Fixture Liang", "Fixture Ted"}[i%2], Title: []string{"校验任务映射与证据", "优化仓库分页读取"}[i%2], HeadSHA: strings.Repeat("a", 40), Status: "completed", Phase: "评审完成", PublishStatus: "off", ReportJSON: `{"summary":"隔离数据：评审详情展示验证","findings":[],"assessments":[],"questions":[]}`, CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute)}
				if err := s.codeReview.DB.Create(&run).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
		if os.Getenv("WELL_JIRA_SYNC_FIXTURE_CONFIG_ONLY") == "1" {
			t.Logf("Jira Issue synthetic group=%s/ops Jira=%s", remote.URL, remote.URL)
			return
		}
		run, lease, err := svc.Begin(context.Background(), view.Settings.Version, "discover")
		if err != nil {
			t.Fatal(err)
		}
		svc.Execute(context.Background(), run, lease, nil)
		view, _ = svc.View(context.Background())
		c = view.Settings
		c.Mappings = []jiraissuesync.Mapping{{Project: "NS2", RepoIDs: []string{"42", "43"}, TaskKeys: []string{"NS2-1"}}, {Project: "WA", RepoIDs: []string{"43"}}}
		if _, err = svc.Save(context.Background(), c); err != nil {
			t.Fatal(err)
		}
	}
	// Public synthetic endpoint only; no token is seeded or borrowed from global config.
	t.Logf("Jira Issue synthetic group=%s/ops Jira=%s (use any synthetic tokens)", remote.URL, remote.URL)
}
