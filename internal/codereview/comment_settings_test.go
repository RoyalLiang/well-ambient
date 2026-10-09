package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestCommentSettingsDefaultsAndIndependentRepositoryPolicy(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncCommits: true, SyncMRs: true, Rules: "keep repository rule"})
	settings, err := f.s.CommentSettings(context.Background())
	if err != nil || !settings.SyncCommits || !settings.SyncMRs {
		t.Fatalf("default global settings=%+v err=%v", settings, err)
	}
	disabled := false
	settings, err = f.s.PatchCommentSettings(context.Background(), &disabled, nil)
	if err != nil || settings.SyncCommits || !settings.SyncMRs {
		t.Fatalf("partial update settings=%+v err=%v", settings, err)
	}
	settings, err = f.s.CommentSettings(context.Background())
	if err != nil || settings.SyncCommits || !settings.SyncMRs {
		t.Fatalf("persisted settings=%+v err=%v", settings, err)
	}
	p, err := f.s.Policy("10")
	if err != nil || !p.SyncCommits || !p.SyncMRs || p.Rules != "keep repository rule" {
		t.Fatalf("global update rewrote repository policy=%+v err=%v", p, err)
	}
	p, err = f.s.Policy("unconfigured")
	if err != nil || p.SyncCommits || p.SyncMRs {
		t.Fatalf("repository defaults must remain disabled: %+v err=%v", p, err)
	}
}

func TestCommentSettingsConcurrentPartialUpdatesPreserveBothFields(t *testing.T) {
	f := newFixture(t)
	disabled := false
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for _, isCommit := range []bool{true, false} {
		wg.Add(1)
		go func(commit bool) {
			defer wg.Done()
			var err error
			if commit {
				_, err = f.s.PatchCommentSettings(context.Background(), &disabled, nil)
			} else {
				_, err = f.s.PatchCommentSettings(context.Background(), nil, &disabled)
			}
			errors <- err
		}(isCommit)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	settings, err := f.s.CommentSettings(context.Background())
	if err != nil || settings.SyncCommits || settings.SyncMRs {
		t.Fatalf("concurrent partial update lost a field: %+v err=%v", settings, err)
	}
}

func TestGlobalCommentGateBlocksCompletedAndQueuedReviews(t *testing.T) {
	for _, kind := range []string{"commit", "mr"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			f.policy(t, db.CodeReviewPolicy{SyncCommits: true, SyncMRs: true})
			run := f.review(t, kind)
			ref := "8"
			if kind == "commit" {
				ref = head
			}
			queued, err := f.s.Enqueue(context.Background(), "10", kind, ref, "queued-before-global-disabled")
			if err != nil || queued.PublishStatus != "waiting_review" {
				t.Fatalf("queue should initially allow comments: %+v err=%v", queued, err)
			}
			disabled := false
			if _, err := f.s.PatchCommentSettings(context.Background(), &disabled, &disabled); err != nil {
				t.Fatal(err)
			}
			if err := f.s.Publish(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			run, err = f.s.Get(run.ID)
			if err != nil || run.PublishStatus != "off" || f.posts != 0 {
				t.Fatalf("closed global gate published completed review: %+v posts=%d err=%v", run, f.posts, err)
			}
			if _, err := f.s.ProcessOne(context.Background()); err != nil {
				t.Fatal(err)
			}
			queued, err = f.s.Get(queued.ID)
			if err != nil || queued.PublishStatus != "off" {
				t.Fatalf("completed queued review bypassed global gate: %+v err=%v", queued, err)
			}
		})
	}
}

func TestGlobalCommentGateRecheckedBeforeRemoteWrite(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	baseTransport := f.s.HTTP.Transport
	changed := false
	f.s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if !changed && strings.HasSuffix(r.URL.Path, "/merge_requests/7") {
			changed = true
			disabled := false
			if _, err := f.s.PatchCommentSettings(context.Background(), nil, &disabled); err != nil {
				return nil, err
			}
		}
		return baseTransport.RoundTrip(r)
	})
	if err := f.s.Publish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	run, err := f.s.Get(run.ID)
	if !changed || err != nil || run.PublishStatus != "off" || f.posts != 0 {
		t.Fatalf("final gate not enforced: changed=%t status=%s posts=%d err=%v", changed, run.PublishStatus, f.posts, err)
	}
}

func TestSystemCorpusIncludesEnabledCrossRepositoryProjectAndModuleFacts(t *testing.T) {
	f := newFixture(t)
	for _, fact := range []db.ContextFact{
		{Scope: "global", Status: "active", Content: "global completion rule"},
		{Scope: "repo", ScopeID: "other-repository", Status: "active", Content: "cross repository completion rule"},
		{Scope: "project", ScopeID: "other-project", Status: "active", Content: "project completion rule"},
		{Scope: "module", ScopeID: "shared-module", Status: "active", Content: "module completion rule"},
		{Scope: "repo", ScopeID: "disabled-repository", Status: "disabled", Content: "disabled completion rule"},
		{Scope: "global", Status: "archived", Content: "archived completion rule"},
	} {
		if err := f.s.DB.Create(&fact).Error; err != nil {
			t.Fatal(err)
		}
	}
	f.policy(t, db.CodeReviewPolicy{Domain: "fms", Scenario: "dispatch", KnowledgeScope: "OLD-NARROW-SCOPE"})
	run := f.review(t, "mr")
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(run.SnapshotJSON), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Knowledge) != 4 || strings.Contains(run.SnapshotJSON, "disabled completion") || strings.Contains(run.SnapshotJSON, "archived completion") {
		t.Fatalf("system corpus selection=%+v", snapshot.Knowledge)
	}
	var p db.CodeReviewPolicy
	if err := json.Unmarshal([]byte(run.PolicyJSON), &p); err != nil {
		t.Fatal(err)
	}
	if p.Domain != "general" || p.Scenario != "general" || p.KnowledgeScope != SystemCorpusScope {
		t.Fatalf("legacy fields narrowed review policy=%+v", p)
	}
}

func TestLegacyNarrowReportCannotPublishAsSystemCorpusReview(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run := f.review(t, "mr")
	legacy := db.CodeReviewPolicy{ProjectID: "10", Domain: "fms", Scenario: "dispatch", KnowledgeScope: "FMS", SyncMRs: true}
	run.PolicyJSON = encode(legacy)
	if err := f.s.DB.Model(&run).Update("policy_json", run.PolicyJSON).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.Publish(context.Background(), run); err == nil || f.posts != 0 {
		t.Fatalf("legacy narrow report was published, posts=%d err=%v", f.posts, err)
	}
}

func TestSystemCorpusRanksRelevantOlderFactsBeforeCandidateCap(t *testing.T) {
	for _, unrelatedCount := range []int{20, 260} {
		t.Run(fmt.Sprint(unrelatedCount), func(t *testing.T) {
			f := newFixture(t)
			older := time.Now().Add(-24 * time.Hour)
			global := db.ContextFact{Scope: "global", Status: "active", Content: "criticalflow shared constraint", UpdatedAt: older}
			crossRepo := db.ContextFact{Scope: "repo", ScopeID: "other-repository", Status: "active", Content: "criticalflow cross repository contract", UpdatedAt: older}
			for _, fact := range []*db.ContextFact{&global, &crossRepo} {
				if err := f.s.DB.Create(fact).Error; err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < unrelatedCount; i++ {
				fact := db.ContextFact{Scope: "repo", ScopeID: "10", Status: "active", Content: "unrelated repository material"}
				if err := f.s.DB.Create(&fact).Error; err != nil {
					t.Fatal(err)
				}
			}
			facts, err := f.s.knowledge(context.Background(), SystemCorpusPolicy(db.CodeReviewPolicy{}), config.RepoMapping{ProjectID: "10"}, Snapshot{Title: "criticalflow"})
			if err != nil {
				t.Fatal(err)
			}
			if len(facts) > 20 || len(facts) < 2 || facts[0].ID != crossRepo.ID || facts[1].ID != global.ID {
				t.Fatalf("%d unrelated repo facts crowded out relevant old evidence: %+v", unrelatedCount, facts)
			}
			if facts[0].Scope != "repo:other-repository" || facts[1].Scope != "global:" {
				t.Fatalf("cross repository provenance was lost: %+v", facts[:2])
			}
		})
	}
}

func TestSystemCorpusKeepsContentAndCandidateBudgets(t *testing.T) {
	f := newFixture(t)
	oversized := db.ContextFact{Scope: "global", Status: "active", Content: "criticalflow " + strings.Repeat("x", 24000)}
	if err := f.s.DB.Create(&oversized).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 220; i++ {
		fact := db.ContextFact{Scope: "module", ScopeID: "shared", Status: "active", Content: "criticalflow " + strings.Repeat("x", 5000)}
		if err := f.s.DB.Create(&fact).Error; err != nil {
			t.Fatal(err)
		}
	}
	candidates, err := f.s.systemCorpusCandidates(context.Background(), []string{"criticalflow"})
	if err != nil || len(candidates) != 200 {
		t.Fatalf("candidate cap=%d err=%v", len(candidates), err)
	}
	facts, err := f.s.knowledge(context.Background(), SystemCorpusPolicy(db.CodeReviewPolicy{}), config.RepoMapping{}, Snapshot{Title: "criticalflow"})
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, fact := range facts {
		n := len(fact.Summary) + len(fact.Content)
		if fact.ID == oversized.ID || n > 24000 {
			t.Fatalf("oversized fact entered corpus: id=%d bytes=%d", fact.ID, n)
		}
		total += n
	}
	if len(facts) == 0 || len(facts) > 20 || total > 36000 {
		t.Fatalf("selection exceeded budgets: facts=%d bytes=%d", len(facts), total)
	}
}

func TestLegacyQueuedReviewFreezesActualEffectivePolicy(t *testing.T) {
	f := newFixture(t)
	f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
	run, err := f.s.Enqueue(context.Background(), "10", "mr", "7", "legacy-queued")
	if err != nil {
		t.Fatal(err)
	}
	legacy := db.CodeReviewPolicy{ProjectID: "10", Domain: "fms", Scenario: "dispatch", KnowledgeScope: "FMS", SyncMRs: true}
	if err := f.s.DB.Model(&run).Update("policy_json", encode(legacy)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	run, err = f.s.Get(run.ID)
	if err != nil || run.Status != "completed" || run.PublishStatus != "pending" {
		t.Fatalf("legacy queued review failed: %+v err=%v", run, err)
	}
	var p db.CodeReviewPolicy
	if err := json.Unmarshal([]byte(run.PolicyJSON), &p); err != nil {
		t.Fatal(err)
	}
	if p.Domain != "general" || p.Scenario != "general" || p.KnowledgeScope != SystemCorpusScope {
		t.Fatalf("new evidence retained incorrect old scope snapshot: %+v", p)
	}
	if err := f.s.Publish(context.Background(), run); err != nil || f.posts != 1 {
		t.Fatalf("newly assessed system report could not publish: posts=%d err=%v", f.posts, err)
	}
}
