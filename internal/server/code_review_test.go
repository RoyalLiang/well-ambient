package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"well-ambient/internal/codereview"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

const reviewFixtureHead = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const reviewFixtureBase = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func reviewFixtureServer(t *testing.T) (*Server, string, *httptest.Server) {
	t.Helper()
	setupServerTestDB(t)
	var mu sync.Mutex
	comments := []map[string]any{}
	git := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var out any
		switch {
		case r.Method == "POST":
			var p map[string]string
			_ = json.NewDecoder(r.Body).Decode(&p)
			body := p["body"]
			if body == "" {
				body = p["note"]
			}
			mu.Lock()
			comments = append(comments, map[string]any{"id": 1, "body": body, "note": body})
			mu.Unlock()
			out = map[string]int{"id": 1}
		case strings.HasSuffix(r.URL.Path, "/notes") || strings.HasSuffix(r.URL.Path, "/comments"):
			mu.Lock()
			defer mu.Unlock()
			out = comments
		case strings.Contains(r.URL.Path, "/repository/files/"):
			out = map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("package dispatch\nfunc completeTask(feedback string) bool {\n return feedback == \"\"\n}\n"))}
		case strings.HasSuffix(r.URL.Path, "/diffs") || strings.HasSuffix(r.URL.Path, "/diff"):
			out = []codereview.File{{OldPath: "dispatch/completion.go", NewPath: "dispatch/completion.go", Diff: "@@ -1,4 +1,4 @@\n package dispatch\n func completeTask(feedback string) bool {\n- return feedback == \"completed\"\n+ return feedback == \"\"\n }"}}
		case strings.Contains(r.URL.Path, "/merge_requests/"):
			out = map[string]any{"state": "opened", "title": "修正车辆反馈后的任务完成判定", "author": map[string]any{"name": "Review Fixture"}, "description": "保留重复事件幂等性，明确车辆与任务匹配的完成证据", "web_url": "https://gitlab.example.test/fms/dispatch/-/merge_requests/42", "diff_refs": map[string]string{"head_sha": reviewFixtureHead, "base_sha": reviewFixtureBase}}
		case strings.Contains(r.URL.Path, "/repository/commits/"):
			out = map[string]any{"id": reviewFixtureHead, "title": "修正车辆反馈判定", "parent_ids": []string{reviewFixtureBase}}
		default:
			http.NotFound(w, r)
			return
		}
		emailJSON(w, 200, out)
	}))
	t.Cleanup(git.Close)
	cfg := &config.Config{}
	cfg.GitLab = config.GitLabConfig{Enabled: true, BaseURL: git.URL, APIToken: "fixture-only", Repos: []config.RepoMapping{{ProjectID: "10", Name: "FMS / 调度服务", Path: "fms/dispatch"}, {ProjectID: "20", Name: "FMS / 车辆服务", Path: "fms/vehicle"}}}
	cfg.AI.Enabled = true
	cfg.AI.Model = "fixture-review-model"
	srv := NewServer(cfg, "")
	srv.codeReview.Generate = func(ctx context.Context, system, user string) (string, error) {
		report := codereview.Report{Summary: "任务完成判定把空反馈当作完成证据，可能提前释放车辆资源。建议先保留未决状态，并根据匹配的任务反馈完成转换。", Scenario: "FMS 任务调度", Findings: []codereview.Finding{{Dimension: "robustness", Severity: "high", Title: "空反馈被当作任务完成，可能提前释放车辆", File: "dispatch/completion.go", Line: 3, Evidence: `return feedback == ""`, Impact: "通信中断或反馈延迟时，未完成任务可能被提前结束，车辆资源被重复分配。", Suggestion: "只接受与当前任务和车辆匹配的完成反馈；无反馈保留原状态。", Verification: "增加空反馈、重复反馈、旧任务反馈和反馈乱序的测试。", KnowledgeIDs: []uint{1}}}, Questions: []string{"本次未读取车辆管理服务，跨服务完成确认契约仍需核对。"}}
		for _, d := range codereview.Dimensions {
			analysis := map[string]string{"business": "核对任务完成与车辆反馈的业务不变量，不能以消息缺失替代完成证据。", "robustness": "空反馈路径存在风险，应保留未决状态并支持恢复。", "reusability": "建议将完成证据校验作为明确的领域能力复用，避免多个作业流程各自实现。", "abstraction": "完成判定应表达领域语义，而非把传输层空字符串直接转换为业务完成。", "encapsulation": "资源释放应封装在任务状态转换内，避免调用者绕过证据校验。", "concurrency": "需验证重复反馈和乱序反馈，不应重复释放资源。", "security": "当前变更未涉及权限入口，外部消息来源认证不在本次读取范围。", "performance": "当前判定为常量开销，未发现新增查询或循环。", "testing": "建议覆盖空反馈、过期反馈和重复完成事件。", "delivery": "上线前需核对历史未决任务；通过日志追踪完成依据和任务身份。"}[d]
			report.Assessments = append(report.Assessments, codereview.Assessment{Dimension: d, Analysis: analysis})
		}
		data, _ := json.Marshal(report)
		return string(data), nil
	}
	token := superAdminToken(t, "review-fixture@example.test", "Review Fixture", []string{"dashboard:read", "config:read", "config:write", "ai_context:preview", "solution_prompt:manage"})
	return srv, token, git
}
func reviewRequest(srv *Server, token, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handler.ServeHTTP(w, r)
	return w
}
func TestCodeReviewActionResponseExcludesEvidencePayloads(t *testing.T) {
	payload, err := json.Marshal(codeReviewActionResponse(db.CodeReviewRun{
		ID:           7,
		Status:       "completed",
		PolicyJSON:   `{"rules":"secret policy"}`,
		SnapshotJSON: `{"source":"secret source"}`,
		ReportJSON:   `{"summary":"secret report"}`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"policy_json", "snapshot_json", "report_json", "secret policy", "secret source", "secret report"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("action response leaked %q: %s", forbidden, payload)
		}
	}
}
func TestCodeReviewSyncDisabledReturnsConflict(t *testing.T) {
	srv, token, _ := reviewFixtureServer(t)
	policyJSON, _ := json.Marshal(db.CodeReviewPolicy{
		ProjectID:  "10",
		AutoReview: true,
		Domain:     "general",
		Scenario:   "general",
	})
	reportJSON, _ := json.Marshal(codereview.Report{Summary: "fixture"})
	run := db.CodeReviewRun{
		Key:           "sync-disabled",
		ProjectID:     "10",
		Repo:          "FMS / 调度服务",
		Kind:          "mr",
		Ref:           "42",
		HeadSHA:       reviewFixtureHead,
		BaseSHA:       reviewFixtureBase,
		Status:        "completed",
		Phase:         "评审完成",
		PolicyJSON:    string(policyJSON),
		ReportJSON:    string(reportJSON),
		PublishStatus: "off",
	}
	if err := db.DB.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	w := reviewRequest(srv, token, "POST", fmt.Sprintf("/api/code-reviews/%d/sync", run.ID), "")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "开关已关闭") {
		t.Fatalf("status/body = %d/%s", w.Code, w.Body.String())
	}
}
func TestCodeReviewAPI(t *testing.T) {
	srv, token, _ := reviewFixtureServer(t)
	if w := reviewRequest(srv, "", "GET", "/api/code-reviews", ""); w.Code != 401 {
		t.Fatalf("unauthenticated status=%d", w.Code)
	}
	if w := reviewRequest(srv, token, "PUT", "/api/code-reviews/policy", `{"project_id":"999","domain":"fms","scenario":"dispatch"}`); w.Code != 400 {
		t.Fatal("unconfigured project accepted")
	}
	if w := reviewRequest(srv, token, "PUT", "/api/code-reviews/policy", `{"project_id":"10","domain":"fms","scenario":"dispatch","knowledge_scope":"FMS","sync_mrs":true}`); w.Code != 200 {
		t.Fatalf("policy status %d %s", w.Code, w.Body.String())
	}
	if w := reviewRequest(srv, token, "GET", "/api/code-reviews/repos", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"sync_mrs":true`) {
		t.Fatalf("policy readback status/body = %d/%s", w.Code, w.Body.String())
	}
	if w := reviewRequest(srv, token, "PUT", "/api/code-reviews/policy", `{"project_id":"10","rules":"保留独立字段更新"}`); w.Code != 200 {
		t.Fatalf("partial policy update status/body = %d/%s", w.Code, w.Body.String())
	}
	if w := reviewRequest(srv, token, "GET", "/api/code-reviews/repos", ""); w.Code != 200 ||
		!strings.Contains(w.Body.String(), `"sync_mrs":true`) ||
		!strings.Contains(w.Body.String(), `"rules":"保留独立字段更新"`) {
		t.Fatalf("partial policy update lost fields: %d/%s", w.Code, w.Body.String())
	}
	w := reviewRequest(srv, token, "POST", "/api/code-reviews", `{"project_id":"10","kind":"mr","ref":"42"}`)
	if w.Code != 202 {
		t.Fatalf("enqueue %d %s", w.Code, w.Body.String())
	}
	var run db.CodeReviewRun
	_ = json.Unmarshal(w.Body.Bytes(), &run)
	if _, err := srv.codeReview.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	w = reviewRequest(srv, token, "GET", fmt.Sprintf("/api/code-reviews/%d", run.ID), "")
	if w.Code != 200 {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"partial"`) {
		t.Fatal("missing FMS evidence not reflected")
	}
	w = reviewRequest(srv, token, "POST", fmt.Sprintf("/api/code-reviews/%d/sync", run.ID), "")
	if w.Code != 409 {
		t.Fatal("incomplete review can publish")
	}
	failed := db.CodeReviewRun{Key: "api-failed", ProjectID: "10", Repo: "FMS / 调度服务", Kind: "mr", Ref: "41", Status: "failed", Phase: "评审失败", Error: "fixture failure"}
	if err := db.DB.Create(&failed).Error; err != nil {
		t.Fatal(err)
	}
	w = reviewRequest(srv, token, "POST", fmt.Sprintf("/api/code-reviews/%d/retry", failed.ID), "")
	if w.Code != 202 {
		t.Fatalf("retry status/body = %d/%s", w.Code, w.Body.String())
	}
	var retry db.CodeReviewRun
	if err := json.Unmarshal(w.Body.Bytes(), &retry); err != nil {
		t.Fatal(err)
	}
	if retry.ID == failed.ID || retry.Status != "queued" || retry.Ref != failed.Ref {
		t.Fatalf("unexpected retry response: %+v", retry)
	}
	w = reviewRequest(srv, token, "POST", fmt.Sprintf("/api/code-reviews/%d/retry", failed.ID), "")
	var replayed db.CodeReviewRun
	_ = json.Unmarshal(w.Body.Bytes(), &replayed)
	if w.Code != 202 || replayed.ID != retry.ID {
		t.Fatalf("retry replay status/body = %d/%s", w.Code, w.Body.String())
	}
	req := httptest.NewRequest("GET", "/api/code-reviews/1?repo=unrelated", nil)
	resource := authorizationResourceFromRequest("dashboard:read", req)
	if resource.Scope != "global" {
		t.Fatal("query downgraded authorization scope")
	}
}

// Opt-in isolated app with a real authenticated UI and local-only GitLab/LLM fixtures.
func TestCodeReviewBrowserFixture(t *testing.T) {
	if os.Getenv("WELL_CODE_REVIEW_BROWSER_FIXTURE") != "1" {
		t.Skip("opt-in browser fixture")
	}
	srv, token, _ := reviewFixtureServer(t)
	fact := db.ContextFact{ID: 1, Scope: "project", ScopeID: "FMS", Status: "active", Version: 3, Summary: "任务完成必须具备匹配的车辆反馈证据", Content: "测试知识：任务与车辆身份匹配后才确认完成。没有反馈不代表成功。重复反馈不得重复释放资源。", Source: "fixture", ContentHash: "fixture-fms-rule"}
	if err := db.DB.Create(&fact).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.CodeReviewPolicy{ProjectID: "10", Domain: "fms", Scenario: "dispatch", KnowledgeScope: "FMS"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, entry := range []db.GitCommitLog{{Repo: "FMS / 调度服务", Author: "Review Fixture", Action: "git_push", CommitID: reviewFixtureHead, Message: "修正车辆反馈与完成判定", Branch: "feature/task-evidence"}, {Repo: "FMS / 调度服务", Author: "Review Fixture", Action: "mr_open", MrIID: 42, Message: "统一完成证据与任务状态转换", Branch: "feature/task-evidence"}} {
		if err := db.DB.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
	}
	err := srv.codeReview.Hook(context.Background(), "Merge Request Hook", []byte(`{"project":{"id":10},"object_attributes":{"iid":42,"state":"opened","last_commit":{"id":"`+reviewFixtureHead+`"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var run db.CodeReviewRun
	if err = db.DB.First(&run).Error; err != nil {
		t.Fatal(err)
	}

	if _, err = srv.codeReview.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	run.ID = 0
	run.Key = "fixture-failed"
	run.Status = "failed"
	run.Phase = "评审失败"
	run.Error = "模型服务暂时不可用，请重新评审"
	run.Ref = "41"
	if err = db.DB.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if os.Getenv("WELL_CODE_REVIEW_SCROLL_FIXTURE") == "1" {
		var completed db.CodeReviewRun
		if err := db.DB.Where("status = ?", "completed").First(&completed).Error; err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 36; i++ {
			item := completed
			item.ID = 0
			item.Key = fmt.Sprintf("scroll-fixture-%d", i)
			item.Ref = fmt.Sprintf("%d", 100+i)
			item.Author = "Review Fixture"
			if i%3 == 0 {
				item.ProjectID = "20"
				item.Repo = "FMS / 车辆服务"
			}
			item.Title = fmt.Sprintf("%02d · 验证长列表：任务状态一致性、反馈恢复与跨服务资源释放边界", i+1)
			if err := db.DB.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	root, _ := filepath.Abs("../../web/dist")
	mux := http.NewServeMux()
	mux.Handle("/api/", srv.handler)
	mux.Handle("/", http.FileServer(http.Dir(root)))
	mux.HandleFunc("GET /__fixture/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>localStorage.setItem('jwt_token',%q);localStorage.setItem('current_user_name','Review Fixture');localStorage.setItem('current_user_email','review-fixture@example.test');localStorage.setItem('current_user_role','super_admin');localStorage.setItem('current_user_permissions','["dashboard:read","config:read","config:write","ai_context:preview","solution_prompt:manage"]');location.replace('/');</script>`, token)
	})
	group := userdb.UserGroup{Name: "review-fixture-reader"}
	db.DB.Create(&group)
	for _, code := range []string{"dashboard:read", "ai_context:preview"} {
		var permission userdb.Permission
		db.DB.Where("code = ?", code).First(&permission)
		db.DB.Create(&userdb.GroupPermission{UserGroupID: group.ID, PermissionID: permission.ID})
	}
	readerToken := seedPolicyTestUser(t, "review-reader@example.test", group.Name, "global", "")
	mux.HandleFunc("GET /__fixture/reader-login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>localStorage.setItem('jwt_token',%q);localStorage.setItem('current_user_name','Review Reader');localStorage.setItem('current_user_email','review-reader@example.test');localStorage.setItem('current_user_role','member');localStorage.setItem('current_user_permissions','["dashboard:read","ai_context:preview"]');location.replace('/');</script>`, readerToken)
	})
	stop := make(chan struct{})
	var once sync.Once
	mux.HandleFunc("POST /__fixture/stop", func(w http.ResponseWriter, r *http.Request) { once.Do(func() { close(stop) }); w.WriteHeader(204) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.startCodeReviewWorker(ctx)
	web := httptest.NewServer(mux)
	defer web.Close()
	os.MkdirAll("../../outputs", 0755)
	data, _ := json.Marshal(map[string]string{"url": web.URL})
	if err = os.WriteFile("../../outputs/code-review-fixture.json", data, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stop:
	case <-time.After(45 * time.Minute):
	}
}

func TestCodeReviewTargetsAndKnowledgeAuthorization(t *testing.T) {
	srv, token, _ := reviewFixtureServer(t)
	for _, log := range []db.GitCommitLog{{Repo: "FMS / 调度服务", Action: "git_push", CommitID: reviewFixtureHead, Author: "member", Message: "first"}, {Repo: "FMS / 调度服务", Action: "git_push", CommitID: reviewFixtureHead, Author: "member", Message: "duplicate"}, {Repo: "FMS / 调度服务", Action: "mr_open", MrIID: 7, Message: "MR"}, {Repo: "FMS / 调度服务", Action: "ai_review", Message: "old AI summary"}, {Repo: "other", Action: "git_push", CommitID: reviewFixtureBase, Message: "unrelated"}} {
		if err := db.DB.Create(&log).Error; err != nil {
			t.Fatal(err)
		}
	}
	w := reviewRequest(srv, token, "GET", "/api/code-reviews/targets?project_id=10", "")
	if w.Code != 200 {
		t.Fatalf("targets status=%d", w.Code)
	}
	var result struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Items) != 2 || strings.Contains(w.Body.String(), "unrelated") || strings.Contains(w.Body.String(), "old AI") {
		t.Fatal("targets are not scoped/deduplicated")
	}
	group := userdb.UserGroup{Name: "review-list-reader"}
	db.DB.Create(&group)
	var permission userdb.Permission
	db.DB.Where("code = ?", "dashboard:read").First(&permission)
	if permission.ID == 0 {
		t.Fatal("missing dashboard permission")
	}
	db.DB.Create(&userdb.GroupPermission{UserGroupID: group.ID, PermissionID: permission.ID})
	reader := seedPolicyTestUser(t, "limited-review@example.test", group.Name, "global", "")
	if w := reviewRequest(srv, reader, "GET", "/api/code-reviews", ""); w.Code != 200 {
		t.Fatalf("reader cannot see queue %d", w.Code)
	}
	if w := reviewRequest(srv, reader, "GET", "/api/code-reviews/1", ""); w.Code != 403 {
		t.Fatalf("knowledge/source exposed without ai_context permission %d", w.Code)
	}
	if w := reviewRequest(srv, reader, "POST", "/api/code-reviews", `{"project_id":"10","kind":"mr","ref":"7"}`); w.Code != 403 {
		t.Fatal("read-only user can enqueue")
	}
	if w := reviewRequest(srv, reader, "PUT", "/api/code-reviews/policy", `{"project_id":"10","domain":"general","scenario":"general","sync_mrs":true}`); w.Code != 403 {
		t.Fatal("read-only user can enable external comments")
	}
	cfg := srv.currentEmailConfig()
	cfg.GitLab.Repos = append(cfg.GitLab.Repos, config.RepoMapping{ProjectID: "30", Name: "FMS / 调度服务"})
	srv.setEmailConfig(cfg)
	w = reviewRequest(srv, token, "GET", "/api/code-reviews/targets?project_id=10", "")
	if !strings.Contains(w.Body.String(), "重复映射") {
		t.Fatal("ambiguous name mapping not blocked")
	}
}
