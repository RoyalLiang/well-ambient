package codereview

import (
	"strings"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestDocumentConclusionAndPriorityPresentation(t *testing.T) {
	report := exampleReport()
	report.EvidenceComplete = true
	report.Summary = "上线结论：证据不足。第一批摘要。\n\n上线结论：证据不足。第二批摘要。"
	for _, p := range []string{"P0", "P1", "P2", "P3", "P4", "P5"} {
		f := report.Findings[0]
		f.Priority = p
		f.Title = "问题" + p
		report.Findings = append(report.Findings, f)
	}
	run := db.CodeReviewRun{Status: "completed", Kind: "mr", Ref: "255", Repo: "fde / project-code/fuzhou/fms/gui_server", CreatedAt: time.Date(2026, 10, 23, 17, 0, 0, 0, time.UTC), ReportJSON: encode(report), SnapshotJSON: encode(Snapshot{Complete: true, SourceBranch: "feature/FZ-2661"})}
	doc, err := BuildDocument(run)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(doc.Sections["review"], "**评审结论：存在阻塞问题") || !strings.HasPrefix(doc.Markdown, "**评审结论：") {
		t.Fatal("conclusion must be the first independent bold line")
	}
	if strings.Contains(doc.Sections["review"], "第一批摘要") || strings.Contains(doc.Sections["review"], "F1") {
		t.Fatal("raw batch summaries and internal finding IDs leaked into body")
	}
	for _, p := range []string{"P0", "P1", "P2", "P3", "P4", "P5"} {
		if !strings.Contains(doc.Sections["review"], "["+p+"]") || !strings.Contains(doc.Sections["review"], "问题"+p) {
			t.Fatalf("missing priority %s", p)
		}
	}
	if doc.Filename != "gui_server_feature-FZ-2661_mr255_2026-10-24.md" {
		t.Fatal(doc.Filename)
	}
	if !strings.Contains(doc.Sections["facts"], "第一批摘要") {
		t.Fatal("historical summary evidence was lost")
	}
}
func TestDocumentNeverUpgradesPartialConclusion(t *testing.T) {
	report := exampleReport()
	report.Findings = nil
	report.EvidenceComplete = true
	for _, tc := range []struct {
		status   string
		complete bool
	}{{"partial", true}, {"completed", false}} {
		doc, _ := BuildDocument(db.CodeReviewRun{Status: tc.status, ReportJSON: encode(report), SnapshotJSON: encode(Snapshot{Complete: tc.complete})})
		if !strings.HasPrefix(doc.Sections["review"], "**评审结论：证据或覆盖不足") {
			t.Fatal(doc.Sections["review"])
		}
	}
}
func TestFilenameUntrustedSegments(t *testing.T) {
	run := db.CodeReviewRun{Kind: "mr", Ref: "7", Repo: "a/b", SnapshotJSON: encode(Snapshot{SourceBranch: "../fix/evil\r\n\x00\u202e" + strings.Repeat("长", 100)})}
	filename := reviewFilename(run)
	if strings.ContainsAny(filename, "/\\\r\n\x00") || strings.Contains(filename, "\u202e") || len(filename) > 230 {
		t.Fatal(filename)
	}
}

func TestDocumentStartsWithCompactIssueIndexAndKeepsFullExplanations(t *testing.T) {
	report := exampleReport()
	report.Findings[0].Title = "foo\n\n![unexpected](https://example.test/image.png)\n\n"
	snapshot := Snapshot{Head: head, URL: "https://gitlab.example.test/fms/dispatch/-/merge_requests/7"}
	doc, _ := BuildDocument(db.CodeReviewRun{Status: "completed", ReportJSON: encode(report), SnapshotJSON: encode(snapshot)})
	body := doc.Sections["review"]
	index := strings.Index(body, "<table class=\"cr-findings-overview\">")
	details := strings.Index(body, "## 问题详情")
	if index < 0 || details < index || strings.Contains(body, "<details") {
		t.Fatal("expected a compact index followed by full explanations")
	}
	if strings.Contains(body, "\n\n![unexpected]") || strings.Contains(body, "<img") {
		t.Fatal("untrusted title escaped its text field")
	}
	if !strings.Contains(body, "<th scope=\"col\">来源</th>") || !strings.Contains(body, "/-/blob/"+head+"/dispatch.go#L3") {
		t.Fatal("missing frozen, clickable source line")
	}
	for _, label := range []string{"触发与影响", "修改建议", "验证建议（未执行）"} {
		if !strings.Contains(body[details:], label) {
			t.Fatal("explanation lost: " + label)
		}
	}
}
func TestFindingSourceLinksRejectUntrustedRootsAndTraversal(t *testing.T) {
	snapshot := Snapshot{Head: head, URL: "https://user:secret@example.test/org/repo/-/merge_requests/7"}
	if findingSourceURL(db.CodeReviewRun{}, snapshot, Finding{File: "a.go", Line: 1}) != "" {
		t.Fatal("credentialed source link")
	}
	snapshot.URL = "https://example.test/org/repo/-/merge_requests/7"
	if findingSourceURL(db.CodeReviewRun{}, snapshot, Finding{File: "../a.go", Line: 1}) != "" {
		t.Fatal("source path traversal")
	}
	target := findingSourceURL(db.CodeReviewRun{}, snapshot, Finding{File: "src/a b.go", Line: 12})
	if target != "https://example.test/org/repo/-/blob/"+head+"/src/a%20b.go#L12" {
		t.Fatal(target)
	}
}
