package codereview

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"well-ambient/internal/db"
)

func registerReviewMethod(t *testing.T, f *fixture, status string) uint {
	t.Helper()
	if err := f.s.DB.AutoMigrate(&db.Capability{}, &db.CapabilityVersion{}, &db.CapabilityResource{}); err != nil {
		t.Fatal(err)
	}
	capability := db.Capability{CapabilityKey: "code_review", Kind: "skill", Status: status}
	if err := f.s.DB.Create(&capability).Error; err != nil {
		t.Fatal(err)
	}
	version := db.CapabilityVersion{CapabilityID: capability.ID, Version: 3, ScopeType: "global", Status: "active", ValidationStatus: "passed", ManifestJSON: "{}", ContentDigest: "fixture-digest"}
	if err := f.s.DB.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	resource := db.CapabilityResource{CapabilityVersionID: version.ID, ResourceKey: "review-method", ContentKind: "text", ContentHash: digest("ENABLED_MERGE_REVIEW_RULE"), Content: "ENABLED_MERGE_REVIEW_RULE"}
	if err := f.s.DB.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	return version.ID
}

func TestCodeReviewBothPassesUseFrozenEnabledMergeReview(t *testing.T) {
	f := newFixture(t)
	versionID := registerReviewMethod(t, f, "active")
	original := f.s.Generate
	var prompts []string
	f.s.Generate = func(ctx context.Context, system, input string) (string, error) {
		prompts = append(prompts, system)
		return original(ctx, system, input)
	}
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "42", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.DB.Model(&db.CapabilityResource{}).Where("capability_version_id = ?", versionID).Update("content", "LATER_RULE_MUST_NOT_REPLACE_FROZEN").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected analysis and counter-review, got %d", len(prompts))
	}
	for i, prompt := range prompts {
		if !strings.Contains(prompt, "ENABLED_MERGE_REVIEW_RULE") || !strings.Contains(prompt, "review_framework.md") || !strings.Contains(prompt, "report_templates.md") || strings.Contains(prompt, "LATER_RULE_MUST_NOT_REPLACE_FROZEN") {
			t.Fatal("actual model pass did not use the frozen enabled merge-review bundle")
		}
		if !strings.Contains(prompt, `"priority":"P1"`) {
			t.Errorf("pass %d does not request merge-review priority", i)
		}
	}
	run, err = f.s.Get(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal([]byte(run.ReportJSON), &report); err != nil {
		t.Fatal(err)
	}
	if report.ReviewMethod == nil || report.ReviewMethod.Name != "merge-review" || report.ReviewMethod.VersionID != versionID || report.ReviewMethod.Hash == "" {
		t.Fatalf("report lacks actual method provenance: %+v", report)
	}
}

func TestMergeReviewPriorityAndMethodAppearInPublishedReport(t *testing.T) {
	f := newFixture(t)
	snapshot, err := f.s.git().Snapshot(context.Background(), "10", "mr", "42")
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.Replace(encode(exampleReport()), `"severity":"medium"`, `"severity":"medium","priority":"P2"`, 1)
	report, err := parseReport(raw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encode(report), `"priority":"P2"`) {
		t.Fatal("merge-review priority disappeared during evidence validation")
	}
	report.ReviewMethod = &ReviewMethod{Name: "merge-review", Version: 3, Hash: strings.Repeat("a", 64)}
	body := commentBody(db.CodeReviewRun{HeadSHA: head}, report)
	if !strings.Contains(body, "merge-review") || !strings.Contains(body, "P2") {
		t.Fatal("published report omitted actual method or priority")
	}
}

func TestCodeReviewCannotBypassDisabledMergeReview(t *testing.T) {
	f := newFixture(t)
	registerReviewMethod(t, f, "disabled")
	if _, err := f.s.Enqueue(context.Background(), "10", "mr", "42", ""); err == nil {
		t.Fatal("disabled skill silently fell back to default review")
	}
}

func TestCodeReviewRejectsCorruptEnabledSkillResource(t *testing.T) {
	f := newFixture(t)
	versionID := registerReviewMethod(t, f, "active")
	if err := f.s.DB.Model(&db.CapabilityResource{}).Where("capability_version_id = ?", versionID).Update("content", "CHANGED_WITHOUT_UPDATING_HASH").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Enqueue(context.Background(), "10", "mr", "42", ""); err == nil {
		t.Fatal("corrupt enabled skill resource was accepted")
	}
}

func TestCodeReviewRejectsUnverifiableEnabledSkillResource(t *testing.T) {
	for _, hash := range []string{"", "unknown-reference", "sha256:short"} {
		t.Run(hash, func(t *testing.T) {
			f := newFixture(t)
			versionID := registerReviewMethod(t, f, "active")
			if err := f.s.DB.Model(&db.CapabilityResource{}).Where("capability_version_id = ?", versionID).Update("content_hash", hash).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := f.s.Enqueue(context.Background(), "10", "mr", "42", ""); err == nil {
				t.Fatal("unverifiable skill resource was accepted")
			}
		})
	}
}

func TestCodeReviewQueuedRunHonorsSkillDisable(t *testing.T) {
	f := newFixture(t)
	registerReviewMethod(t, f, "active")
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "42", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.DB.Model(&db.Capability{}).Where("capability_key = ?", "code_review").Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	called := false
	f.s.Generate = func(context.Context, string, string) (string, error) { called = true; return "", nil }
	f.s.ProcessOne(context.Background())
	run, err = f.s.Get(run.ID)
	if err != nil || run.Status != "failed" || called {
		t.Fatalf("disabled skill executed: run=%+v called=%v err=%v", run, called, err)
	}
}
