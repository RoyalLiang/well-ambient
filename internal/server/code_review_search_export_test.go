package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/codereview"
	"well-ambient/internal/db"
)

func TestCodeReviewServerSearchBeforeLimitAndPagination(t *testing.T) {
	srv, token, _ := reviewFixtureServer(t)
	old := db.CodeReviewRun{Key: "old-target", ProjectID: "10", Kind: "mr", Ref: "42", Title: "旧评审 100%_literal", HeadSHA: reviewFixtureHead, CreatedAt: time.Now().Add(-time.Hour)}
	if err := db.DB.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 205; i++ {
		if err := db.DB.Create(&db.CodeReviewRun{Key: fmt.Sprintf("search-%d", i), ProjectID: "20", Kind: "commit", Ref: fmt.Sprintf("%040x", i+1), Title: "新提交", SnapshotJSON: "MUST_NOT_LEAK"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.DB.Create(&db.CodeReviewRun{Key: "invisible", ProjectID: "99", Kind: "mr", Ref: "42", Title: "不可见"}).Error; err != nil {
		t.Fatal(err)
	}
	search := func(query string) codereview.RunPage {
		t.Helper()
		response := reviewRequest(srv, token, http.MethodGet, "/api/code-reviews?"+query, "")
		if response.Code != 200 {
			t.Fatalf("search status %d: %s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "MUST_NOT_LEAK") {
			t.Fatal("list leaked frozen evidence")
		}
		var page codereview.RunPage
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	for _, q := range []string{"!42", "MR !42", "100%_literal", reviewFixtureHead[:10]} {
		page := search("q=" + url.QueryEscape(q))
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != old.ID {
			t.Fatalf("query %s: %+v", q, page)
		}
	}
	if page := search("kind=commit&q=" + url.QueryEscape("!42")); page.Total != 0 {
		t.Fatal("type filter was ignored")
	}
	first, second := search("kind=commit&limit=50"), search("kind=commit&limit=50&offset=50")
	if first.Total != 205 || !first.HasMore || len(first.Items) != 50 || len(second.Items) != 50 {
		t.Fatal("invalid pagination")
	}
	ids := map[uint]bool{}
	for _, run := range first.Items {
		ids[run.ID] = true
	}
	for _, run := range second.Items {
		if ids[run.ID] {
			t.Fatal("overlapping pages")
		}
	}
	if page := search("q=" + url.QueryEscape("' OR 1=1 --")); page.Total != 0 {
		t.Fatal("query was not treated literally")
	}
	for _, q := range []string{"kind=unknown", "offset=-1", "limit=201", "q=" + strings.Repeat("a", 201)} {
		if got := reviewRequest(srv, token, http.MethodGet, "/api/code-reviews?"+q, ""); got.Code != 400 {
			t.Fatalf("accepted %s: %d", q, got.Code)
		}
	}
}

func TestCodeReviewMarkdownExportMatchesDetailAndPermissions(t *testing.T) {
	srv, token, _ := reviewFixtureServer(t)
	report := codereview.Report{Summary: "检查完成", Validation: "仅模型与源码验证，未运行测试", Findings: []codereview.Finding{{Priority: "P1", Title: "重复释放", File: "completion.go", Line: 3, Evidence: "release()", Impact: "重复反馈时", Suggestion: "幂等保护", Verification: "建议重放反馈"}}}
	snapshot := codereview.Snapshot{Description: "修正重复释放", Complete: false, Gaps: []string{"未读取外部服务"}, Files: []codereview.File{{NewPath: "completion.go", Diff: "@@ -1 +1 @@\n-release()\n+releaseOnce()", Source: "releaseOnce()"}}}
	b, _ := json.Marshal(report)
	snap, _ := json.Marshal(snapshot)
	run := db.CodeReviewRun{Key: "export-result", ProjectID: "10", Kind: "mr", Ref: "42", Title: "任务完成", Status: "partial", ReportJSON: string(b), SnapshotJSON: string(snap), PolicyJSON: `{"rules":"历史规则"}`}
	if err := db.DB.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/code-reviews/%d", run.ID)
	detail := reviewRequest(srv, token, http.MethodGet, path, "")
	var payload struct {
		Document codereview.ReviewDocument `json:"document"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	export := reviewRequest(srv, token, http.MethodGet, path+"/export", "")
	if artifact := os.Getenv("WELL_REVIEW_EXPORT_ARTIFACT"); artifact != "" {
		if err := os.WriteFile(artifact, export.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if detail.Code != 200 || export.Code != 200 || export.Body.String() != payload.Document.Markdown {
		t.Fatalf("detail and export differ: %d/%d", detail.Code, export.Code)
	}
	if export.Header().Get("Content-Type") != "text/markdown; charset=utf-8" || !strings.Contains(export.Header().Get("Content-Disposition"), fmt.Sprintf("code-review-%d.md", run.ID)) || export.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("incorrect download headers")
	}
	for _, section := range []string{"review", "facts", "code", "knowledge", "rules"} {
		if payload.Document.Sections[section] == "" || !strings.Contains(export.Body.String(), payload.Document.Sections[section]) {
			t.Fatalf("missing section %s", section)
		}
	}
	if !strings.Contains(export.Body.String(), "部分结果") || !strings.Contains(export.Body.String(), "未读取外部服务") {
		t.Fatal("export hid partial coverage")
	}
	reader := seedPolicyTestUser(t, "export-reader", "member", "global", "")
	for _, credential := range []string{"", reader} {
		res := reviewRequest(srv, credential, http.MethodGet, path+"/export", "")
		if res.Code != http.StatusUnauthorized && res.Code != http.StatusForbidden {
			t.Fatalf("unprivileged export %d", res.Code)
		}
		if strings.Contains(res.Body.String(), "历史规则") {
			t.Fatal("export leaked evidence")
		}
	}
	if err := db.DB.Model(&run).Update("status", "queued").Error; err != nil {
		t.Fatal(err)
	}
	if res := reviewRequest(srv, token, http.MethodGet, path+"/export", ""); res.Code != 409 {
		t.Fatal("queued run exported as completed")
	}
	if err := db.DB.Model(&run).Updates(map[string]any{"project_id": "99", "status": "completed"}).Error; err != nil {
		t.Fatal(err)
	}
	if res := reviewRequest(srv, token, http.MethodGet, path+"/export", ""); res.Code != 404 {
		t.Fatal("unconfigured repository exported")
	}
}

func TestCodeReviewDetailKeepsEvidenceWithoutExportableResult(t *testing.T) {
	srv, token, _ := reviewFixtureServer(t)
	stale, _ := json.Marshal(codereview.Report{Summary: "STALE_RESULT_MUST_NOT_APPEAR"})
	snapshot, _ := json.Marshal(codereview.Snapshot{
		Description:       "SAVED_DESCRIPTION",
		Files:             []codereview.File{{NewPath: "saved.go", Source: "saved()", Diff: "SAVED_DIFF"}},
		Knowledge:         []codereview.Knowledge{{ID: 9, Summary: "SAVED_KNOWLEDGE", Content: "SAVED_KNOWLEDGE_CONTENT"}},
		InputOptimization: []codereview.InputOptimization{{Reason: "compressed", InputBytes: 10000, OutputBytes: 5000}},
	})
	for _, tc := range []struct{ status, report string }{
		{"queued", ""}, {"running", "invalid"}, {"failed", string(stale)}, {"cancelled", string(stale)},
		{"completed", "invalid"}, {"partial", "{}"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			run := db.CodeReviewRun{Key: "evidence-" + tc.status, ProjectID: "10", Kind: "mr", Ref: "42", Status: tc.status, ReportJSON: tc.report, SnapshotJSON: string(snapshot), PolicyJSON: `{"rules":"SAVED_POLICY_RULE"}`}
			if err := db.DB.Create(&run).Error; err != nil {
				t.Fatal(err)
			}
			path := fmt.Sprintf("/api/code-reviews/%d", run.ID)
			response := reviewRequest(srv, token, http.MethodGet, path, "")
			var detail struct {
				Document *codereview.ReviewDocument `json:"document"`
			}
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &detail) != nil || detail.Document == nil {
				t.Fatalf("saved evidence unavailable: %d %s", response.Code, response.Body.String())
			}
			if !strings.Contains(detail.Document.Sections["review"], "没有可用的评审结论") || strings.Contains(detail.Document.Sections["review"], "未发现") || strings.Contains(detail.Document.Markdown, "STALE_RESULT_MUST_NOT_APPEAR") {
				t.Fatalf("non-result presented as review conclusion: %s", detail.Document.Sections["review"])
			}
			for section, expected := range map[string][]string{
				"facts": {"SAVED_DESCRIPTION", "Headroom", "已压缩", "10000", "5000"},
				"code":  {"SAVED_DIFF"}, "knowledge": {"SAVED_KNOWLEDGE_CONTENT"}, "rules": {"SAVED_POLICY_RULE"},
			} {
				for _, value := range expected {
					if !strings.Contains(detail.Document.Sections[section], value) {
						t.Errorf("%s evidence lost %q", section, value)
					}
				}
			}
			if exported := reviewRequest(srv, token, http.MethodGet, path+"/export", ""); exported.Code != http.StatusConflict {
				t.Fatalf("non-result exported: %d %s", exported.Code, exported.Body.String())
			}
		})
	}
}
