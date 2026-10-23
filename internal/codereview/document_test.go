package codereview

import (
	"encoding/json"
	"strings"
	"testing"
	"well-ambient/internal/db"
)

func TestHistoricalDocumentUsesFrozenRulesAndQualifiesKnowledge(t *testing.T) {
	report := exampleReport()
	report.Findings[0].KnowledgeIDs = []uint{9}
	snap := Snapshot{Files: []File{{NewPath: "dispatch.go", Diff: "```\nmalicious fence\n```"}}, Knowledge: []Knowledge{{ID: 9, Version: 3, Summary: "规则", Content: "</details>\n```", Hash: "original"}, {ID: 10, Summary: "背景"}}}
	run := db.CodeReviewRun{ID: 7, Status: "completed", ReportJSON: encode(report), SnapshotJSON: encode(snap), PolicyJSON: `{"rules":"frozen-only"}`}
	doc, err := BuildDocument(run)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"运行时规则快照未保存", "frozen-only", "问题引用：F1", "检索到的背景资料", "P2", "验证建议（未执行）"} {
		if !strings.Contains(doc.Markdown, s) {
			t.Errorf("missing %s", s)
		}
	}
	if strings.Contains(doc.Markdown, DefaultReviewRules[0]) {
		t.Fatal("current runtime rules attributed to historical result")
	}
	if !strings.Contains(doc.Markdown, "````diff\n```\nmalicious fence") {
		t.Fatal("code fence escaped into report")
	}
}

func TestIntentChecksRejectFabricatedDeclarationAndCode(t *testing.T) {
	report := exampleReport()
	snap := Snapshot{Title: "修正错误处理", Files: []File{{NewPath: "dispatch.go", Source: "package dispatch\nfunc run() {\nreturn nil\n}", Diff: "@@ -3 +3 @@\n-return err\n+return nil"}}}
	report.IntentChecks = []IntentCheck{{Declaration: "修正错误处理", Status: "有代码依据", Analysis: "检查返回路径", Code: []CodeReference{{File: "dispatch.go", Line: 3}}}}
	valid, err := parseReport(encode(report), snap)
	if err != nil || len(valid.IntentChecks) != 1 {
		t.Fatalf("valid intent rejected: %v", err)
	}
	for _, check := range []IntentCheck{{Declaration: "不存在的需求", Status: "有代码依据", Analysis: "声称实现"}, {Declaration: "修正错误处理", Status: "有代码依据", Analysis: "声称实现", Code: []CodeReference{{File: "missing.go", Line: 3}}}} {
		report.IntentChecks = []IntentCheck{check}
		parsed, err := parseReport(encode(report), snap)
		if err != nil {
			t.Fatal(err)
		}
		if len(parsed.IntentChecks) != 0 || parsed.EvidenceComplete {
			t.Fatal("fabricated intent accepted")
		}
	}
	var contract Snapshot
	f := newFixture(t)
	run := f.review(t, "mr")
	if err := json.Unmarshal([]byte(run.SnapshotJSON), &contract); err != nil {
		t.Fatal(err)
	}
	if contract.EvidenceContract == nil || contract.EvidenceContract.Version != RuntimePromptVersion || len(contract.EvidenceContract.PromptHash) != 64 {
		t.Fatal("actual runtime rules not frozen")
	}
}

func TestCodeFenceHandlesLongUntrustedRuns(t *testing.T) {
	value := strings.Repeat("`", 160000)
	fenced := mdCode(value, "text")
	if !strings.HasPrefix(fenced, strings.Repeat("`", 160001)+"text\n") || !strings.HasSuffix(fenced, "\n"+strings.Repeat("`", 160001)) {
		t.Fatal("unsafe code fence")
	}
}

func TestDocumentPreservesLegacyPriority(t *testing.T) {
	for _, tc := range []struct{ severity, group string }{
		{"P0", "阻塞问题（P0 / P1）"},
		{"P1", "阻塞问题（P0 / P1）"},
		{"P2", "改进建议（P2）"},
		{"P3", "细节建议（P3）"},
	} {
		t.Run(tc.severity, func(t *testing.T) {
			report := exampleReport()
			report.Findings[0].Severity = tc.severity
			report.Findings[0].Priority = ""
			doc, err := BuildDocument(db.CodeReviewRun{Status: "completed", ReportJSON: encode(report)})
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range []string{"### " + tc.group, "#### F1 · " + tc.severity + " ·"} {
				if !strings.Contains(doc.Sections["review"], expected) {
					t.Fatalf("legacy priority %s changed: %s", tc.severity, doc.Sections["review"])
				}
			}
		})
	}
}

func TestDocumentKeepsEvidenceWithoutReviewConclusion(t *testing.T) {
	stale := exampleReport()
	stale.Summary = "STALE_RESULT_MUST_NOT_APPEAR"
	snapshot := Snapshot{
		Description:       "SAVED_DESCRIPTION",
		Files:             []File{{NewPath: "saved.go", Source: "saved()", Diff: "SAVED_DIFF"}},
		Knowledge:         []Knowledge{{ID: 9, Summary: "SAVED_KNOWLEDGE", Content: "SAVED_KNOWLEDGE_CONTENT"}},
		InputOptimization: []InputOptimization{{Reason: "compressed", InputBytes: 10000, OutputBytes: 5000}},
		EvidenceContract:  &EvidenceContract{Version: "SAVED_RUNTIME", Rules: []string{"SAVED-RUNTIME-RULE"}},
	}
	for _, tc := range []struct{ status, report string }{
		{"queued", ""}, {"running", "invalid"}, {"failed", encode(stale)}, {"cancelled", encode(stale)},
		{"completed", "invalid"}, {"partial", "{}"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			doc, err := BuildDocument(db.CodeReviewRun{Status: tc.status, ReportJSON: tc.report, SnapshotJSON: encode(snapshot), PolicyJSON: `{"rules":"SAVED_POLICY_RULE"}`})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(doc.Sections["review"], "没有可用的评审结论") || strings.Contains(doc.Sections["review"], "未发现") || strings.Contains(doc.Markdown, "STALE_RESULT_MUST_NOT_APPEAR") {
				t.Fatalf("missing or stale result presented as a conclusion: %s", doc.Sections["review"])
			}
			for section, expected := range map[string][]string{
				"facts":     {"SAVED_DESCRIPTION", "Headroom", "已压缩", "10000", "5000"},
				"code":      {"SAVED_DIFF"},
				"knowledge": {"SAVED_KNOWLEDGE", "SAVED_KNOWLEDGE_CONTENT"},
				"rules":     {"SAVED-RUNTIME-RULE", "SAVED_POLICY_RULE"},
			} {
				for _, value := range expected {
					if !strings.Contains(doc.Sections[section], value) {
						t.Errorf("%s evidence lost %q: %s", section, value, doc.Sections[section])
					}
				}
			}
		})
	}
}
