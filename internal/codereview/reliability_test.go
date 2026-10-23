package codereview

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"well-ambient/internal/db"
	"well-ambient/internal/headroom"
)

func TestReviewAcceptsFencedJSONWithoutAssessmentBoilerplate(t *testing.T) {
	r := exampleReport()
	r.Assessments = nil
	s := Snapshot{Files: []File{{NewPath: "dispatch.go", Diff: "@@ -1,4 +1,4 @@\n package dispatch\n func Run() error {\n- return err\n+ return nil\n }", Source: "package dispatch\nfunc Run() error {\n return nil\n}"}}}
	parsed, err := parseReport("```json\n"+encode(r)+"\n```", s)
	if err != nil || !parsed.EvidenceComplete || len(parsed.Findings) != 1 {
		t.Fatalf("valid findings rejected for presentation format: %+v %v", parsed, err)
	}
}

func TestReviewMultilineEvidenceIsValidatedAtItsStartingLine(t *testing.T) {
	if evidenceAtLine([]string{"", "return nil", "}"}, 1, "return nil\n}") {
		t.Fatal("blank lines must not shift multiline evidence to a later line")
	}
	r := exampleReport()
	r.Findings[0].Evidence = " return nil\n}"
	s := Snapshot{Files: []File{{NewPath: "dispatch.go", Diff: "@@ -1,4 +1,4 @@\n package dispatch\n func Run() error {\n- return err\n+ return nil\n }", Source: "package dispatch\nfunc Run() error {\n return nil\n}"}}}
	parsed, err := parseReport(encode(r), s)
	if err != nil || !parsed.EvidenceComplete || len(parsed.Findings) != 1 {
		t.Fatalf("valid multiline evidence rejected: %+v %v", parsed, err)
	}
	r.Findings[0].Line = 2
	parsed, err = parseReport(encode(r), s)
	if err != nil || parsed.EvidenceComplete || len(parsed.Findings) != 0 {
		t.Fatal("wrong starting line accepted")
	}
}

func TestLargeReviewRetainsSourcesAndReviewsEveryBatch(t *testing.T) {
	f := newFixture(t)
	original := f.s.HTTP.Transport
	f.s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		var payload any
		switch {
		case strings.HasSuffix(r.URL.Path, "/diffs"):
			files := []File{}
			for i := 0; i < 30; i++ {
				name := fmt.Sprintf("file%d.go", i)
				files = append(files, File{NewPath: name, OldPath: name, Diff: "@@ -1 +1 @@\n-old\n+new"})
			}
			payload = files
		case strings.Contains(r.URL.Path, "/repository/files/"):
			payload = map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("new\n" + strings.Repeat("// context\n", 1500)))}
		default:
			return original.RoundTrip(r)
		}
		b, _ := json.Marshal(payload)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})
	seen := map[string]int{}
	f.s.Generate = func(ctx context.Context, system, input string) (string, error) {
		f.calls++
		var snap Snapshot
		if err := json.NewDecoder(strings.NewReader(input)).Decode(&snap); err != nil {
			return "", err
		}
		for _, file := range snap.Files {
			if file.Source == "" {
				t.Errorf("source discarded: %s", file.NewPath)
			}
			seen[file.NewPath]++
		}
		r := exampleReport()
		r.Findings = nil
		return encode(r), nil
	}
	run := f.review(t, "mr")
	if run.Status != "completed" || len(seen) != 30 || f.calls < 4 {
		t.Fatalf("large review was truncated instead of batched: status=%s files=%d calls=%d", run.Status, len(seen), f.calls)
	}
	for file, count := range seen {
		if count != 2 {
			t.Errorf("%s reviewed %d times; want two passes", file, count)
		}
	}
}

func TestReportBodyOmitsEngineeringAssessments(t *testing.T) {
	r := exampleReport()
	r.Assessments = []Assessment{{Dimension: "robustness", Analysis: "ASSESSMENT_ONLY_SENTINEL"}}
	doc, err := BuildDocument(db.CodeReviewRun{ID: 1, Status: "completed", ReportJSON: encode(r), SnapshotJSON: encode(Snapshot{Complete: true}), PolicyJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(doc.Sections["review"], "触发与影响") > strings.Index(doc.Sections["review"], "验证记录") {
		t.Fatal("findings must precede supporting verification/intent details")
	}
	if strings.Contains(doc.Markdown, "工程质量与适用性") || strings.Contains(doc.Markdown, "ASSESSMENT_ONLY_SENTINEL") {
		t.Fatal("engineering boilerplate leaked into report body/export")
	}
}

func TestReviewRecordsActualHeadroomInvocation(t *testing.T) {
	python := os.Getenv("WELL_AMBIENT_HEADROOM_TEST_PYTHON")
	if python == "" {
		t.Skip("requires installed Headroom Python")
	}
	worker, err := filepath.Abs("../../capabilities/plugins/headroom/worker.py")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WELL_AMBIENT_HEADROOM_PYTHON", python)
	t.Setenv("WELL_AMBIENT_HEADROOM_WORKER", worker)
	f := newFixture(t)
	priorDB := db.DB
	db.DB = f.s.DB
	t.Cleanup(func() { headroom.Default().Close(); db.DB = priorDB })
	if err := f.s.DB.AutoMigrate(&db.Capability{}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DB.Create(&db.Capability{CapabilityKey: headroom.CapabilityKey, Kind: "plugin", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		b, _ := json.Marshal(payload)
		if !strings.Contains(string(b), "SYSTEM_PROTECTED") || !strings.Contains(string(b), "2048") {
			t.Error("optimization lost system/data facts")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"output": []any{map[string]any{"type": "message", "content": []any{map[string]string{"type": "output_text", "text": "ready"}}}}})
	}))
	defer provider.Close()
	f.s.Generate = nil
	f.cfg.AI.BaseURL = provider.URL + "/v1/responses"
	f.cfg.AI.EndpointType = "responses"
	f.cfg.AI.APIToken = "local-test-only"
	rows := []string{}
	for i := 0; i < 200; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":%d,"status":"INFO","component":"review-integration","limit":2048}`, i))
	}
	input := "[" + strings.Join(rows, ",") + "]"
	observations := []InputOptimization{}
	got, err := f.s.generateObserved(context.Background(), f.cfg.AI, "SYSTEM_PROTECTED", input, &observations)
	if err != nil || got != "ready" || len(observations) != 1 || observations[0].Reason != "compressed" || observations[0].OutputBytes >= observations[0].InputBytes {
		t.Fatalf("review did not invoke installed lossless Headroom: output=%q observations=%+v err=%v", got, observations, err)
	}
	t.Logf("review Headroom: %+v", observations[0])
}
