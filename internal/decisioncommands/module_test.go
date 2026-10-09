package decisioncommands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

type fakeJira struct {
	mu                  sync.Mutex
	state               JiraState
	assignCalls         int
	dueDateCalls        int
	dispatchErr         error
	failReadsAfterWrite bool
	readErr             error
	omitIssueIdentity   bool
	afterWriteIssueKey  string
	afterWriteProject   string
	wrote               bool
}

type definiteDispatchError struct{}

func (definiteDispatchError) Error() string         { return "definite rejection" }
func (definiteDispatchError) DefiniteFailure() bool { return true }

func (f *fakeJira) ReadIssueState(_ context.Context, _ db.JiraExecutionBinding, issueKey string) (JiraState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readErr != nil {
		return JiraState{}, f.readErr
	}
	if f.wrote && f.failReadsAfterWrite {
		return JiraState{}, errors.New("verification timeout")
	}
	state := f.state
	if state.IssueKey == "" && !f.omitIssueIdentity {
		state.IssueKey = issueKey
	}
	if f.wrote && f.afterWriteIssueKey != "" {
		state.IssueKey = f.afterWriteIssueKey
		state.ProjectRef = f.afterWriteProject
	}
	return state, nil
}

func (f *fakeJira) Assign(_ context.Context, _ db.JiraExecutionBinding, _ string, assignee string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assignCalls++
	f.wrote = true
	if f.dispatchErr == nil {
		f.state.Assignee = assignee
		f.state.UpdatedAt = f.state.UpdatedAt.Add(time.Second)
		f.state.EvidenceRef = "jira:confirmed"
	}
	return f.dispatchErr
}

func (f *fakeJira) SetDueDate(_ context.Context, _ db.JiraExecutionBinding, _ string, dueDate string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dueDateCalls++
	f.wrote = true
	if f.dispatchErr == nil {
		f.state.DueDate = dueDate
		f.state.UpdatedAt = f.state.UpdatedAt.Add(time.Second)
		f.state.EvidenceRef = "jira:confirmed"
	}
	return f.dispatchErr
}

func decisionFixture(t *testing.T) (*Module, *openaccess.Service, *fakeJira, openaccess.Identity, openaccess.Policy) {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(
		&db.IntegrationSource{}, &db.IntegrationCredential{}, &db.IntegrationPolicyVersion{},
		&db.JiraExecutionBinding{}, &db.DecisionPlan{}, &db.OpenOperation{},
		&db.OpenOperationAction{}, &db.OpenOutbox{},
	); err != nil {
		t.Fatal(err)
	}
	access := openaccess.New(conn)
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	if _, err := access.CreateSource(context.Background(), "agent-host", "Agent Host", "", "standard"); err != nil {
		t.Fatal(err)
	}
	credential, err := access.IssueCredential(context.Background(), "agent-host", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := access.ActivatePolicy(context.Background(), openaccess.PolicySpec{
		AllowedProjects: []string{"WA"},
		Actions: []string{
			"decision.read", "decision.prepare", "decision.execute",
			"decision.reassign", "decision.reschedule",
		},
		ApprovalRules: map[string]string{
			ActionReassign: "auto", ActionReschedule: "auto",
		},
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{ActionReassign, ActionReschedule} {
		if err := access.UpsertExecutionBinding(context.Background(), db.JiraExecutionBinding{
			ProjectRef: "WA", ActionClass: action, ConnectorRef: "jira-primary",
			ExecutorRef: "jira-service", Status: openaccess.SourceActive,
		}); err != nil {
			t.Fatal(err)
		}
	}
	jira := &fakeJira{state: JiraState{
		ProjectRef: "WA",
		Assignee:   "Alice", DueDate: "2026-09-30", UpdatedAt: now, EvidenceRef: "jira:before",
	}}
	module := New(conn, access, jira)
	module.now = func() time.Time { return now }
	identity := openaccess.Identity{SourceID: "agent-host", KeyID: credential.KeyID, QuotaProfile: "standard"}
	return module, access, jira, identity, policy
}

func TestPrepareExecuteAndConfirmRemoteOutcome(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	dueDate := "2026-10-02"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{
		IssueKey: "wa-1", Assignee: &assignee, DueDate: &dueDate,
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if plan.State != StatePrepared || len(plan.Changes) != 2 || plan.Digest == "" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{
		PlanID: plan.ID, IdempotencyKey: "decision-1",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if operation.State != StateQueued || len(operation.Actions) != 2 {
		t.Fatalf("unexpected queued operation: %+v", operation)
	}
	replayed, err := module.Execute(ctx, identity, policy, ExecuteRequest{
		PlanID: plan.ID, IdempotencyKey: "decision-1",
	})
	if err != nil || replayed.ID != operation.ID {
		t.Fatalf("idempotent replay = %+v, %v", replayed, err)
	}
	if _, err := module.Execute(ctx, identity, policy, ExecuteRequest{
		PlanID: plan.ID, IdempotencyKey: "decision-2",
	}); openaccess.ErrorCode(err) != "stale_plan" {
		t.Fatalf("second operation for plan error = %v", err)
	}

	for range 2 {
		processed, processErr := module.ProcessNext(ctx)
		if processErr != nil || !processed {
			t.Fatalf("process next = %v, %v", processed, processErr)
		}
	}
	completed, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != StateSucceeded || jira.assignCalls != 1 || jira.dueDateCalls != 1 {
		t.Fatalf("completed operation = %+v calls=%d/%d", completed, jira.assignCalls, jira.dueDateCalls)
	}
	for _, action := range completed.Actions {
		if action.State != StateSucceeded || action.ConfirmedRemoteValue != action.DesiredValue || action.ConfirmedAt == nil {
			t.Fatalf("unconfirmed action: %+v", action)
		}
	}
}

func TestConcurrentSameIdempotencyKeyReturnsOneOperation(t *testing.T) {
	module, _, _, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-101", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		operation Operation
		err       error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			operation, executeErr := module.Execute(ctx, identity, policy, ExecuteRequest{
				PlanID: plan.ID, IdempotencyKey: "concurrent-same-key",
			})
			results <- result{operation: operation, err: executeErr}
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent execute errors = %v / %v", first.err, second.err)
	}
	if first.operation.ID == "" || first.operation.ID != second.operation.ID {
		t.Fatalf("concurrent operations = %q / %q", first.operation.ID, second.operation.ID)
	}
	var count int64
	if err := module.db.Model(&db.OpenOperation{}).Where("plan_id = ?", plan.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("operation count = %d, want 1", count)
	}
}

func TestStalePreconditionStopsBeforeDispatch(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-2", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "stale"})
	if err != nil {
		t.Fatal(err)
	}
	jira.state.Assignee = "Carol"
	processed, err := module.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("process stale = %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateFailed || result.ErrorCode != "stale_plan" || jira.assignCalls != 0 {
		t.Fatalf("stale operation = %+v calls=%d", result, jira.assignCalls)
	}
}

func TestRemoteTimeoutBecomesUnknownAndIsNotBlindlyRetried(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-3", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	jira.dispatchErr = errors.New("request timeout")
	jira.failReadsAfterWrite = true
	processed, err := module.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("process unknown = %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateUnknown || jira.assignCalls != 1 {
		t.Fatalf("unknown operation = %+v calls=%d", result, jira.assignCalls)
	}
	assertOperationRedacted(t, result, "visibility_unavailable", "Bob", plan.ID, "request timeout", "verification timeout")
	processed, err = module.ProcessNext(ctx)
	if err != nil || processed || jira.assignCalls != 1 {
		t.Fatalf("unknown action was retried: processed=%v err=%v calls=%d", processed, err, jira.assignCalls)
	}
}

func TestDefiniteJiraRejectionBecomesFailedWhenRemoteValueIsUnchanged(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-42", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "definite-reject"})
	if err != nil {
		t.Fatal(err)
	}
	jira.dispatchErr = definiteDispatchError{}
	if processed, err := module.ProcessNext(ctx); err != nil || !processed {
		t.Fatalf("process rejection = %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateFailed || result.ErrorCode != "upstream_rejected" || jira.assignCalls != 1 {
		t.Fatalf("rejected operation = %+v calls=%d", result, jira.assignCalls)
	}
}

func TestSourceDisableAndPolicyTighteningBlockQueuedAction(t *testing.T) {
	module, access, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	dueDate := "2026-10-05"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-4", DueDate: &dueDate})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "disabled"})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.SetSourceStatus(ctx, identity.SourceID, openaccess.SourceDisabled); err != nil {
		t.Fatal(err)
	}
	if processed, err := module.ProcessNext(ctx); err != nil || !processed {
		t.Fatalf("process disabled = %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateFailed || result.ErrorCode != "forbidden" || jira.dueDateCalls != 0 {
		t.Fatalf("disabled operation = %+v calls=%d", result, jira.dueDateCalls)
	}
}

func TestApprovalPolicyTighteningBlocksQueuedAction(t *testing.T) {
	module, access, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-43", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "approval-tightening"})
	if err != nil {
		t.Fatal(err)
	}
	tightened := policy.PolicySpec
	tightened.ApprovalRules = map[string]string{
		ActionReassign:   "manual",
		ActionReschedule: "auto",
	}
	if _, err := access.ActivatePolicy(ctx, tightened, "admin"); err != nil {
		t.Fatal(err)
	}
	if processed, err := module.ProcessNext(ctx); err != nil || !processed {
		t.Fatalf("process tightened approval = %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateFailed || result.ErrorCode != "forbidden" || jira.assignCalls != 0 {
		t.Fatalf("approval-tightened operation = %+v calls=%d", result, jira.assignCalls)
	}
}

func TestRetryableDBConflictClassification(t *testing.T) {
	for _, message := range []string{
		"database table is locked",
		"could not serialize access due to concurrent update",
		"duplicate key value violates unique constraint",
	} {
		if !isRetryableDBConflict(errors.New(message)) {
			t.Fatalf("error %q was not classified as retryable", message)
		}
	}
	if isRetryableDBConflict(errors.New("permission denied")) {
		t.Fatal("permission error was classified as retryable")
	}
}

func TestRevokedExecutionCredentialBlocksQueuedAction(t *testing.T) {
	module, access, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-44", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "revoked-key"})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.RevokeCredential(ctx, identity.KeyID); err != nil {
		t.Fatal(err)
	}
	if processed, err := module.ProcessNext(ctx); err != nil || !processed {
		t.Fatalf("process revoked key = %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateFailed || result.ErrorCode != "forbidden" || jira.assignCalls != 0 {
		t.Fatalf("revoked-key operation = %+v calls=%d", result, jira.assignCalls)
	}
}

func TestRestrictedJiraIssueIsDeniedByDefault(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	jira.state.SecurityID = "10001"
	jira.state.Security = "Restricted"
	assignee := "Bob"
	if _, err := module.Prepare(context.Background(), identity, policy, PrepareRequest{
		IssueKey: "WA-45", Assignee: &assignee,
	}); openaccess.ErrorCode(err) != "forbidden" {
		t.Fatalf("restricted issue error = %v, want forbidden", err)
	}
}

func TestOperationReadHidesEvidenceAfterProjectPublicationIsRemoved(t *testing.T) {
	module, access, _, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	request := ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "removed-project"}
	operation, err := module.Execute(ctx, identity, policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil || len(before.Actions) != 1 || before.Actions[0].ConfirmedRemoteValue != "Bob" {
		t.Fatalf("published operation = %+v, %v", before, err)
	}
	tightened := policy.PolicySpec
	tightened.AllowedProjects = []string{"OTHER"}
	policy, err = access.ActivatePolicy(ctx, tightened, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if result, err := module.GetOperation(ctx, identity, operation.ID); openaccess.ErrorCode(err) != "not_found" || len(result.Actions) != 0 {
		t.Fatalf("removed project operation leaked = %+v, %v", result, err)
	}
	if result, err := module.Execute(ctx, identity, policy, request); openaccess.ErrorCode(err) != "not_found" || len(result.Actions) != 0 {
		t.Fatalf("idempotent replay leaked removed project operation = %+v, %v", result, err)
	}
}

func TestOperationReadRechecksRestrictedIssuePublication(t *testing.T) {
	for _, completed := range []bool{false, true} {
		t.Run(fmt.Sprintf("completed=%t", completed), func(t *testing.T) {
			module, access, jira, identity, policy := decisionFixture(t)
			ctx := context.Background()
			spec := policy.PolicySpec
			spec.DataRules = map[string]string{"jira_restricted_issues": "allow"}
			policy, err := access.ActivatePolicy(ctx, spec, "admin")
			if err != nil {
				t.Fatal(err)
			}
			jira.state.SecurityID = "10001"
			jira.state.Security = "Restricted"
			assignee := "Bob"
			plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
			if err != nil {
				t.Fatal(err)
			}
			operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "restricted-publication"})
			if err != nil {
				t.Fatal(err)
			}
			if completed {
				if _, err := module.ProcessNext(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := module.GetOperation(ctx, identity, operation.ID); err != nil {
				t.Fatalf("explicitly published restricted operation: %v", err)
			}
			jira.readErr = errors.New("current issue identity is unavailable")
			published, err := module.GetOperation(ctx, identity, operation.ID)
			if err != nil || published.ID != operation.ID {
				t.Fatalf("operation status unavailable: %+v, %v", published, err)
			}
			assertOperationRedacted(t, published, "visibility_unavailable", assignee, plan.ID)
			jira.readErr = nil
			spec.DataRules = map[string]string{"jira_restricted_issues": "deny"}
			if _, err := access.ActivatePolicy(ctx, spec, "admin"); err != nil {
				t.Fatal(err)
			}
			result, err := module.GetOperation(ctx, identity, operation.ID)
			if err != nil || result.ID != operation.ID {
				t.Fatalf("restricted operation status unavailable = %+v, %v", result, err)
			}
			assertOperationRedacted(t, result, "restricted_issue", "Bob", "10001", plan.ID)
		})
	}
}

func TestOperationReadFailsClosedWithoutLegacyVisibilityEvidence(t *testing.T) {
	module, _, _, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "legacy-visibility"})
	if err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{"remote_updated_at": plan.Preconditions.RemoteUpdatedAt, "evidence_ref": plan.Preconditions.EvidenceRef, "values": plan.Preconditions.Values}
	legacyJSON, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := module.db.Model(&db.DecisionPlan{}).Where("id = ?", plan.ID).Update("preconditions_json", string(legacyJSON)).Error; err != nil {
		t.Fatal(err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil || result.ID != operation.ID {
		t.Fatalf("legacy operation status unavailable = %+v, %v", result, err)
	}
	assertOperationRedacted(t, result, "stored_visibility_unverified", "Bob", plan.ID)
	if _, err := module.ProcessNext(ctx); err != nil {
		t.Fatalf("publication read gate affected worker: %v", err)
	}
	result, err = module.GetOperation(ctx, identity, operation.ID)
	if err != nil || result.State != StateSucceeded || len(result.Actions) != 1 || result.Actions[0].ConfirmedRemoteValue != "Bob" {
		t.Fatalf("verified legacy operation = %+v, %v", result, err)
	}
}

func TestOperationReadHidesIssueRestrictedBeforeDispatch(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "restricted-before-dispatch"})
	if err != nil {
		t.Fatal(err)
	}
	jira.state.SecurityID = "10001"
	jira.state.Security = "Restricted"
	if _, err := module.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	if jira.assignCalls != 0 {
		t.Fatal("restricted issue was dispatched")
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil || result.State != StateFailed {
		t.Fatalf("restricted failed operation status unavailable = %+v, %v", result, err)
	}
	assertOperationRedacted(t, result, "restricted_issue", "Bob", "10001", plan.ID)
}

func TestOperationReadRechecksVisibilityAfterConfirmation(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Private Person"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	request := ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "restricted-after-confirmation"}
	operation, err := module.Execute(ctx, identity, policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	jira.state.SecurityID = "10001"
	jira.state.Security = "Private Security"
	for _, replay := range []bool{false, true} {
		var result Operation
		if replay {
			result, err = module.Execute(ctx, identity, policy, request)
		} else {
			result, err = module.GetOperation(ctx, identity, operation.ID)
		}
		if err != nil || result.State != StateSucceeded || result.ID != operation.ID {
			t.Fatalf("redacted status remains available, replay=%t: %+v, %v", replay, result, err)
		}
		assertOperationRedacted(t, result, "restricted_issue", "Private Person", "Private Security", "WA-1", plan.ID)
	}
}

func TestOperationVisibilityFailurePreservesStatusWithoutPrivateEvidence(t *testing.T) {
	for _, completed := range []bool{false, true} {
		t.Run(fmt.Sprintf("completed=%t", completed), func(t *testing.T) {
			module, _, jira, identity, policy := decisionFixture(t)
			ctx := context.Background()
			assignee := "Private Person"
			plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
			if err != nil {
				t.Fatal(err)
			}
			request := ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "visibility-unavailable"}
			if !completed {
				jira.readErr = errors.New("private upstream error: WA-1")
			}
			operation, err := module.Execute(ctx, identity, policy, request)
			if err != nil {
				t.Fatal(err)
			}
			if completed {
				if _, err := module.ProcessNext(ctx); err != nil {
					t.Fatal(err)
				}
				jira.readErr = errors.New("private upstream error: WA-1")
			} else {
				assertOperationRedacted(t, operation, "visibility_unavailable", "Private Person", "WA-1", plan.ID, "private upstream error")
			}
			result, err := module.GetOperation(ctx, identity, operation.ID)
			if err != nil || result.ID != operation.ID || result.CreatedAt.IsZero() || result.UpdatedAt.IsZero() || len(result.Actions) != 1 {
				t.Fatalf("status unavailable: %+v, %v", result, err)
			}
			wantState := StateQueued
			if completed {
				wantState = StateSucceeded
			}
			if result.State != wantState || result.Actions[0].State != wantState {
				t.Fatalf("redaction changed recovery status: %+v", result)
			}
			assertOperationRedacted(t, result, "visibility_unavailable", "Private Person", "WA-1", plan.ID, "private upstream error")
		})
	}
}

func assertOperationRedacted(t *testing.T, operation Operation, reason string, privateValues ...string) {
	t.Helper()
	encoded, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	if document["publication_status"] != "redacted" || document["missing_reason"] != reason {
		t.Fatalf("missing explicit publication status: %s", encoded)
	}
	for _, field := range []string{"plan_id", "remote_evidence", "desired_value", "confirmed_remote_value", "error_code"} {
		if strings.Contains(string(encoded), `"`+field+`"`) {
			t.Fatalf("redacted result contains %s: %s", field, encoded)
		}
	}
	for _, value := range privateValues {
		if strings.Contains(string(encoded), value) {
			t.Fatalf("redacted result contains private value %q: %s", value, encoded)
		}
	}
}

func TestDecisionRejectsMovedOrUnverifiableIssueIdentity(t *testing.T) {
	for _, identityCase := range []struct{ issueKey, project string }{
		{"OTHER-1", "OTHER"}, {"WA-2", "WA"}, {"WA-1", "OTHER"}, {"", "WA"}, {"WA-1", ""},
	} {
		t.Run(identityCase.issueKey+"/"+identityCase.project, func(t *testing.T) {
			module, _, jira, identity, policy := decisionFixture(t)
			jira.state.IssueKey = identityCase.issueKey
			jira.state.ProjectRef = identityCase.project
			jira.omitIssueIdentity = identityCase.issueKey == ""
			ctx := context.Background()
			if result, err := module.GetContext(ctx, identity, policy, ContextRequest{IssueKey: "WA-1"}); openaccess.ErrorCode(err) != "forbidden" || result.Current.Assignee != "" {
				t.Fatalf("context published mismatched issue: %+v, %v", result, err)
			}
			assignee := "Bob"
			if result, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee}); openaccess.ErrorCode(err) != "forbidden" || result.ID != "" {
				t.Fatalf("prepared mismatched issue: %+v, %v", result, err)
			}
		})
	}
}

func TestMovedIssueIsBlockedBeforeDispatch(t *testing.T) {
	module, access, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	spec := policy.PolicySpec
	spec.DataRules = map[string]string{"jira_restricted_issues": "allow"}
	policy, err := access.ActivatePolicy(ctx, spec, "admin")
	if err != nil {
		t.Fatal(err)
	}
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	jira.state.IssueKey, jira.state.ProjectRef = "OTHER-1", "OTHER"
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "moved-before-dispatch"})
	if err != nil {
		t.Fatal(err)
	}
	assertOperationRedacted(t, operation, "issue_scope_unverified", "Bob", plan.ID)
	if _, err := module.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	var stored db.OpenOperation
	if err := module.db.First(&stored, "id = ?", operation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.State != StateFailed || jira.assignCalls != 0 {
		t.Fatalf("moved issue was written: state=%s calls=%d", stored.State, jira.assignCalls)
	}
}

func TestMovedIssueCannotConfirmDispatchOrRecovery(t *testing.T) {
	for _, recovering := range []bool{false, true} {
		t.Run(fmt.Sprintf("recovering=%t", recovering), func(t *testing.T) {
			module, access, jira, identity, policy := decisionFixture(t)
			ctx := context.Background()
			spec := policy.PolicySpec
			spec.DataRules = map[string]string{"jira_restricted_issues": "allow"}
			policy, err := access.ActivatePolicy(ctx, spec, "admin")
			if err != nil {
				t.Fatal(err)
			}
			assignee := "Bob"
			plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
			if err != nil {
				t.Fatal(err)
			}
			request := ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "moved-confirmation"}
			operation, err := module.Execute(ctx, identity, policy, request)
			if err != nil {
				t.Fatal(err)
			}
			if recovering {
				if err := module.db.Model(&db.OpenOperationAction{}).Where("operation_id = ?", operation.ID).Update("state", StateDispatching).Error; err != nil {
					t.Fatal(err)
				}
				if err := module.db.Model(&db.OpenOutbox{}).Where("operation_action_id = ?", operation.Actions[0].ID).Updates(map[string]any{"state": StateDispatching, "lease_token": "expired-worker", "leased_until": module.now().Add(-time.Minute)}).Error; err != nil {
					t.Fatal(err)
				}
				jira.state.Assignee = assignee
				jira.state.IssueKey, jira.state.ProjectRef = "OTHER-1", "OTHER"
			} else {
				jira.afterWriteIssueKey, jira.afterWriteProject = "OTHER-1", "OTHER"
			}
			if _, err := module.ProcessNext(ctx); err != nil {
				t.Fatal(err)
			}
			var stored db.OpenOperation
			if err := module.db.First(&stored, "id = ?", operation.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.State != StateUnknown {
				t.Fatalf("wrong issue identity confirmed: %+v", stored)
			}
			result, err := module.GetOperation(ctx, identity, operation.ID)
			if err != nil || result.State != StateUnknown {
				t.Fatalf("unknown status unavailable: %+v, %v", result, err)
			}
			assertOperationRedacted(t, result, "issue_scope_unverified", "Bob", "OTHER-1", plan.ID)
			result, err = module.Execute(ctx, identity, policy, request)
			if err != nil {
				t.Fatal(err)
			}
			assertOperationRedacted(t, result, "issue_scope_unverified", "Bob", "OTHER-1", plan.ID)
			if recovering && jira.assignCalls != 0 {
				t.Fatal("verification recovery resent a write")
			}
		})
	}
}

func TestCompletedMovedIssueRedactsPublishedEvidence(t *testing.T) {
	module, access, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	spec := policy.PolicySpec
	spec.DataRules = map[string]string{"jira_restricted_issues": "allow"}
	policy, err := access.ActivatePolicy(ctx, spec, "admin")
	if err != nil {
		t.Fatal(err)
	}
	assignee := "Private Person"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-1", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	request := ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "moved-after-confirmation"}
	operation, err := module.Execute(ctx, identity, policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	jira.state.IssueKey, jira.state.ProjectRef = "OTHER-1", "OTHER"
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil || result.State != StateSucceeded {
		t.Fatalf("status unavailable: %+v, %v", result, err)
	}
	assertOperationRedacted(t, result, "issue_scope_unverified", "Private Person", "WA-1", plan.ID)
	result, err = module.Execute(ctx, identity, policy, request)
	if err != nil {
		t.Fatal(err)
	}
	assertOperationRedacted(t, result, "issue_scope_unverified", "Private Person", "WA-1", plan.ID)
}

func TestFailedFirstActionStopsLaterActions(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	dueDate := "2026-10-10"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{
		IssueKey: "WA-46", Assignee: &assignee, DueDate: &dueDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{
		PlanID: plan.ID, IdempotencyKey: "fail-stop",
	})
	if err != nil {
		t.Fatal(err)
	}
	jira.state.Assignee = "Carol"
	if processed, err := module.ProcessNext(ctx); err != nil || !processed {
		t.Fatalf("process first action = %v, %v", processed, err)
	}
	if processed, err := module.ProcessNext(ctx); err != nil || processed {
		t.Fatalf("later action should not be claimable: %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateFailed || jira.assignCalls != 0 || jira.dueDateCalls != 0 ||
		len(result.Actions) != 2 || result.Actions[1].ErrorCode != "previous_action_not_confirmed" {
		t.Fatalf("fail-stop operation = %+v calls=%d/%d", result, jira.assignCalls, jira.dueDateCalls)
	}
}

func TestExpiredDispatchLeaseRecoversByVerificationWithoutResend(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-5", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "recovery"})
	if err != nil {
		t.Fatal(err)
	}
	var action db.OpenOperationAction
	if err := module.db.First(&action, "operation_id = ?", operation.ID).Error; err != nil {
		t.Fatal(err)
	}
	expired := module.now().Add(-time.Minute)
	dispatchedAt := module.now().Add(-2 * time.Minute)
	if err := module.db.Model(&db.OpenOperationAction{}).Where("id = ?", action.ID).
		Updates(map[string]any{"state": StateDispatching, "dispatched_at": dispatchedAt}).Error; err != nil {
		t.Fatal(err)
	}
	if err := module.db.Model(&db.OpenOutbox{}).Where("operation_action_id = ?", action.ID).
		Updates(map[string]any{
			"state": StateDispatching, "lease_token": "dead-worker", "leased_until": expired,
		}).Error; err != nil {
		t.Fatal(err)
	}
	jira.state.Assignee = "Bob"
	jira.state.EvidenceRef = "jira:already-confirmed"
	processed, err := module.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("recover process = %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateSucceeded || jira.assignCalls != 0 ||
		result.Actions[0].ConfirmedRemoteValue != "Bob" {
		t.Fatalf("recovered operation = %+v calls=%d", result, jira.assignCalls)
	}
}

func TestExpiredClaimBeforeDispatchCanBeSafelyRetried(t *testing.T) {
	module, _, jira, identity, policy := decisionFixture(t)
	ctx := context.Background()
	dueDate := "2026-10-08"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-6", DueDate: &dueDate})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "claimed-retry"})
	if err != nil {
		t.Fatal(err)
	}
	var action db.OpenOperationAction
	if err := module.db.First(&action, "operation_id = ?", operation.ID).Error; err != nil {
		t.Fatal(err)
	}
	expired := module.now().Add(-time.Minute)
	if err := module.db.Model(&db.OpenOutbox{}).Where("operation_action_id = ?", action.ID).
		Updates(map[string]any{
			"state": StateClaimed, "lease_token": "dead-before-dispatch", "leased_until": expired,
		}).Error; err != nil {
		t.Fatal(err)
	}
	processed, err := module.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("retry claimed process = %v, %v", processed, err)
	}
	result, err := module.GetOperation(ctx, identity, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateSucceeded || jira.dueDateCalls != 1 {
		t.Fatalf("retried claimed operation = %+v calls=%d", result, jira.dueDateCalls)
	}
}

func TestFinishClaimRejectsStaleLeaseOwner(t *testing.T) {
	module, _, _, identity, policy := decisionFixture(t)
	ctx := context.Background()
	assignee := "Bob"
	plan, err := module.Prepare(ctx, identity, policy, PrepareRequest{IssueKey: "WA-7", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}
	operation, err := module.Execute(ctx, identity, policy, ExecuteRequest{PlanID: plan.ID, IdempotencyKey: "lease-fence"})
	if err != nil {
		t.Fatal(err)
	}
	var action db.OpenOperationAction
	if err := module.db.First(&action, "operation_id = ?", operation.ID).Error; err != nil {
		t.Fatal(err)
	}
	var outbox db.OpenOutbox
	if err := module.db.First(&outbox, "operation_action_id = ?", action.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := module.db.Model(&db.OpenOutbox{}).Where("id = ?", outbox.ID).
		Updates(map[string]any{
			"state": StateClaimed, "lease_token": "new-owner",
			"leased_until": module.now().Add(time.Minute),
		}).Error; err != nil {
		t.Fatal(err)
	}
	outbox.LeaseToken = "old-owner"
	if err := module.finishClaim(ctx, outbox, action, StateSucceeded, "Bob", "", "", nil); !errors.Is(err, errDecisionLeaseLost) {
		t.Fatalf("stale lease finish error = %v, want errDecisionLeaseLost", err)
	}
	var unchanged db.OpenOperationAction
	if err := module.db.First(&unchanged, action.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.State != StateQueued || unchanged.ConfirmedRemoteValue != "" {
		t.Fatalf("stale owner changed action: %+v", unchanged)
	}
}
