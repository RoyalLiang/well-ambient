package decisioncommands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

var errDecisionLeaseLost = errors.New("decision outbox lease was lost")

const (
	ToolVersion = "1"

	ActionReassign   = "reassign"
	ActionReschedule = "reschedule"

	StatePrepared    = "prepared"
	StateAccepted    = "accepted"
	StateQueued      = "queued"
	StateClaimed     = "claimed"
	StateDispatching = "dispatching"
	StateSucceeded   = "succeeded"
	StateFailed      = "failed"
	StateUnknown     = "unknown"
	StatePartial     = "partial"
)

type JiraState struct {
	Assignee    string    `json:"assignee"`
	DueDate     string    `json:"due_date"`
	SecurityID  string    `json:"security_id,omitempty"`
	Security    string    `json:"security,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	EvidenceRef string    `json:"evidence_ref"`
}

type JiraPort interface {
	ReadIssueState(ctx context.Context, binding db.JiraExecutionBinding, issueKey string) (JiraState, error)
	Assign(ctx context.Context, binding db.JiraExecutionBinding, issueKey, assignee string) error
	SetDueDate(ctx context.Context, binding db.JiraExecutionBinding, issueKey, dueDate string) error
}

type ContextRequest struct {
	IssueKey string `json:"issue_key"`
}

type ContextResponse struct {
	IssueKey       string    `json:"issue_key"`
	Project        string    `json:"project"`
	Current        JiraState `json:"current"`
	AllowedActions []string  `json:"allowed_actions"`
	PolicyVersion  int       `json:"policy_version"`
}

type PrepareRequest struct {
	IssueKey string  `json:"issue_key"`
	Assignee *string `json:"assignee,omitempty"`
	DueDate  *string `json:"due_date,omitempty"`
	TTL      int     `json:"ttl_seconds,omitempty"`
}

type Change struct {
	Action    string `json:"action"`
	FromValue string `json:"from_value"`
	ToValue   string `json:"to_value"`
}

type Preconditions struct {
	RemoteUpdatedAt time.Time `json:"remote_updated_at"`
	EvidenceRef     string    `json:"evidence_ref"`
	Values          []Change  `json:"values"`
}

type Plan struct {
	ID            string        `json:"plan_id"`
	SourceID      string        `json:"source_id"`
	IssueKey      string        `json:"issue_key"`
	Project       string        `json:"project"`
	Changes       []Change      `json:"changes"`
	Preconditions Preconditions `json:"preconditions"`
	PolicyVersion int           `json:"policy_version"`
	ExpiresAt     time.Time     `json:"expires_at"`
	Digest        string        `json:"digest"`
	State         string        `json:"state"`
}

type ExecuteRequest struct {
	PlanID         string `json:"plan_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type ActionResult struct {
	ID                   uint       `json:"id"`
	Action               string     `json:"action"`
	DesiredValue         string     `json:"desired_value"`
	ConfirmedRemoteValue string     `json:"confirmed_remote_value,omitempty"`
	State                string     `json:"state"`
	RemoteEvidence       any        `json:"remote_evidence,omitempty"`
	ErrorCode            string     `json:"error_code,omitempty"`
	DispatchedAt         *time.Time `json:"dispatched_at,omitempty"`
	ConfirmedAt          *time.Time `json:"confirmed_at,omitempty"`
}

type Operation struct {
	ID        string         `json:"operation_id"`
	PlanID    string         `json:"plan_id"`
	SourceID  string         `json:"source_id"`
	State     string         `json:"state"`
	ErrorCode string         `json:"error_code,omitempty"`
	Actions   []ActionResult `json:"actions"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Module struct {
	db            *gorm.DB
	access        *openaccess.Service
	jira          JiraPort
	now           func() time.Time
	leaseDuration time.Duration
	executeMu     sync.Mutex
}

func New(database *gorm.DB, access *openaccess.Service, jira JiraPort) *Module {
	return &Module{
		db: database, access: access, jira: jira, now: time.Now,
		leaseDuration: 60 * time.Second,
	}
}

func (m *Module) GetContext(
	ctx context.Context,
	identity openaccess.Identity,
	policy openaccess.Policy,
	request ContextRequest,
) (ContextResponse, error) {
	if !policy.AllowsAction("decision.read") {
		return ContextResponse{}, openaccess.NewError("forbidden", "decision reading is not allowed by the active policy")
	}
	issueKey, project, err := normalizeIssueKey(request.IssueKey)
	if err != nil {
		return ContextResponse{}, err
	}
	if !policy.AllowsProject(project) {
		return ContextResponse{}, openaccess.NewError("forbidden", "requested Jira scope is not published")
	}
	if m.jira == nil {
		return ContextResponse{}, openaccess.NewError("upstream_unavailable", "Jira execution adapter is unavailable")
	}
	if _, err := m.access.Source(ctx, identity.SourceID); err != nil {
		return ContextResponse{}, err
	}
	actions := []string{}
	var selectedBinding db.JiraExecutionBinding
	for _, action := range []string{ActionReassign, ActionReschedule} {
		if policyAllowsDecision(policy, action) {
			if binding, bindingErr := m.access.ExecutionBinding(ctx, project, action); bindingErr == nil {
				actions = append(actions, action)
				if selectedBinding.ID == 0 {
					selectedBinding = binding
				}
			}
		}
	}
	if selectedBinding.ID == 0 {
		return ContextResponse{}, openaccess.NewError("forbidden", "no decision action has an active Jira execution binding")
	}
	state, err := m.jira.ReadIssueState(ctx, selectedBinding, issueKey)
	if err != nil {
		return ContextResponse{}, openaccess.NewError("upstream_unavailable", "Jira issue state could not be read")
	}
	if err := authorizeRemoteIssue(policy, state); err != nil {
		return ContextResponse{}, err
	}
	return ContextResponse{
		IssueKey: issueKey, Project: project, Current: state,
		AllowedActions: actions, PolicyVersion: policy.Version,
	}, nil
}

func (m *Module) Prepare(
	ctx context.Context,
	identity openaccess.Identity,
	policy openaccess.Policy,
	request PrepareRequest,
) (Plan, error) {
	if !policy.AllowsAction("decision.prepare") {
		return Plan{}, openaccess.NewError("forbidden", "decision prepare is not allowed by the active policy")
	}
	issueKey, project, err := normalizeIssueKey(request.IssueKey)
	if err != nil {
		return Plan{}, err
	}
	if !policy.AllowsProject(project) {
		return Plan{}, openaccess.NewError("forbidden", "requested Jira scope is not published")
	}
	if _, err := m.access.Source(ctx, identity.SourceID); err != nil {
		return Plan{}, err
	}
	if m.jira == nil {
		return Plan{}, openaccess.NewError("upstream_unavailable", "Jira execution adapter is unavailable")
	}
	requestedActions := requestedActionNames(request)
	bindings := make([]db.JiraExecutionBinding, 0, len(requestedActions))
	for _, action := range requestedActions {
		binding, bindingErr := m.access.ExecutionBinding(ctx, project, action)
		if bindingErr != nil {
			return Plan{}, bindingErr
		}
		bindings = append(bindings, binding)
	}
	if err := compatibleBindings(bindings); err != nil {
		return Plan{}, err
	}
	current, err := m.jira.ReadIssueState(ctx, bindings[0], issueKey)
	if err != nil {
		return Plan{}, openaccess.NewError("upstream_unavailable", "Jira issue state could not be read")
	}
	if err := authorizeRemoteIssue(policy, current); err != nil {
		return Plan{}, err
	}
	changes, err := prepareChanges(policy, current, request)
	if err != nil {
		return Plan{}, err
	}
	for _, change := range changes {
		rule := strings.ToLower(strings.TrimSpace(policy.ApprovalRules[change.Action]))
		if rule != "" && rule != "auto" {
			return Plan{}, openaccess.NewError("forbidden", "the requested action requires a system approval record")
		}
	}
	ttl := time.Duration(request.TTL) * time.Second
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if ttl > time.Hour {
		ttl = time.Hour
	}
	now := m.now().UTC()
	planID, err := openaccess.RandomID("plan")
	if err != nil {
		return Plan{}, fmt.Errorf("generate decision plan id: %w", err)
	}
	preconditions := Preconditions{
		RemoteUpdatedAt: current.UpdatedAt, EvidenceRef: current.EvidenceRef,
		Values: append([]Change(nil), changes...),
	}
	plan := Plan{
		ID: planID, SourceID: identity.SourceID, IssueKey: issueKey, Project: project,
		Changes: changes, Preconditions: preconditions, PolicyVersion: policy.Version,
		ExpiresAt: now.Add(ttl), State: StatePrepared,
	}
	plan.Digest = digest(struct {
		SourceID      string
		IssueKey      string
		Project       string
		Changes       []Change
		Preconditions Preconditions
		PolicyVersion int
		ExpiresAt     time.Time
	}{
		plan.SourceID, plan.IssueKey, plan.Project, plan.Changes,
		plan.Preconditions, plan.PolicyVersion, plan.ExpiresAt,
	})
	changesJSON, _ := json.Marshal(plan.Changes)
	preconditionsJSON, _ := json.Marshal(plan.Preconditions)
	record := db.DecisionPlan{
		ID: plan.ID, SourceID: plan.SourceID, TargetRef: plan.IssueKey, ProjectRef: plan.Project,
		ChangesJSON: string(changesJSON), PreconditionsJSON: string(preconditionsJSON),
		PolicyVersion: plan.PolicyVersion, ExpiresAt: plan.ExpiresAt, Digest: plan.Digest,
		State: StatePrepared, CreatedAt: now, UpdatedAt: now,
	}
	if err := m.db.WithContext(ctx).Create(&record).Error; err != nil {
		return Plan{}, fmt.Errorf("persist decision plan: %w", err)
	}
	return plan, nil
}

func (m *Module) Execute(
	ctx context.Context,
	identity openaccess.Identity,
	policy openaccess.Policy,
	request ExecuteRequest,
) (Operation, error) {
	m.executeMu.Lock()
	defer m.executeMu.Unlock()
	if !policy.AllowsAction("decision.execute") {
		return Operation{}, openaccess.NewError("forbidden", "decision execute is not allowed by the active policy")
	}
	planID := strings.TrimSpace(request.PlanID)
	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if planID == "" || idempotencyKey == "" {
		return Operation{}, openaccess.NewError("invalid_query", "plan_id and idempotency_key are required")
	}
	if len(idempotencyKey) > 160 {
		return Operation{}, openaccess.NewError("invalid_query", "idempotency_key is too long")
	}
	requestDigest := digest(struct {
		PlanID string
	}{planID})
	var operationID string
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing db.OpenOperation
		findErr := tx.Where(
			"source_id = ? AND scope = ? AND idempotency_key = ?",
			identity.SourceID, "decision_execute", idempotencyKey,
		).First(&existing).Error
		if findErr == nil {
			if existing.RequestDigest != requestDigest {
				return openaccess.NewError("idempotency_conflict", "idempotency key is already bound to another request")
			}
			operationID = existing.ID
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		var plan db.DecisionPlan
		if err := tx.Where("id = ? AND source_id = ?", planID, identity.SourceID).First(&plan).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return openaccess.NewError("not_found", "decision plan was not found")
			}
			return err
		}
		if plan.State != StatePrepared {
			return openaccess.NewError("stale_plan", "decision plan has already been accepted or is no longer executable")
		}
		now := m.now().UTC()
		if !plan.ExpiresAt.After(now) {
			return openaccess.NewError("stale_plan", "decision plan has expired")
		}
		var changes []Change
		if err := json.Unmarshal([]byte(plan.ChangesJSON), &changes); err != nil {
			return fmt.Errorf("decode decision plan changes: %w", err)
		}
		if err := authorizeChanges(policy, plan.ProjectRef, changes); err != nil {
			return err
		}
		if _, err := m.access.Source(ctx, identity.SourceID); err != nil {
			return err
		}
		bindings := make(map[string]db.JiraExecutionBinding, len(changes))
		for _, change := range changes {
			binding, bindingErr := m.access.ExecutionBinding(ctx, plan.ProjectRef, change.Action)
			if bindingErr != nil {
				return bindingErr
			}
			bindings[change.Action] = binding
		}
		operationID, findErr = openaccess.RandomID("op")
		if findErr != nil {
			return findErr
		}
		operation := db.OpenOperation{
			ID: operationID, PlanID: plan.ID, SourceID: identity.SourceID,
			KeyID: identity.KeyID,
			Scope: "decision_execute", IdempotencyKey: idempotencyKey,
			RequestDigest: requestDigest, State: StateAccepted, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&operation).Error; err != nil {
			return err
		}
		for position, change := range changes {
			binding := bindings[change.Action]
			action := db.OpenOperationAction{
				OperationID: operation.ID, ActionType: change.Action,
				Position: position, DesiredValue: change.ToValue,
				ConnectorRef: binding.ConnectorRef, ExecutorRef: binding.ExecutorRef,
				BindingVersion: binding.Version,
				State:          StateQueued, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&action).Error; err != nil {
				return err
			}
			outbox := db.OpenOutbox{
				OperationActionID: action.ID, State: StateQueued,
				NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&outbox).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&db.OpenOperation{}).Where("id = ?", operation.ID).
			Updates(map[string]any{"state": StateQueued, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.DecisionPlan{}).Where("id = ?", plan.ID).
			Updates(map[string]any{"state": StateQueued, "updated_at": now}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		delay := 10 * time.Millisecond
		for attempt := 0; attempt < 8; attempt++ {
			recovered, recoveryErr := m.recoverConcurrentExecute(
				ctx, identity.SourceID, planID, idempotencyKey, requestDigest,
			)
			if recoveryErr == nil {
				return recovered, nil
			}
			if openaccess.ErrorCode(recoveryErr) != "not_found" && !isRetryableDBConflict(recoveryErr) {
				return Operation{}, recoveryErr
			}
			if attempt < 7 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return Operation{}, ctx.Err()
				case <-timer.C:
				}
				if delay < 160*time.Millisecond {
					delay *= 2
				}
			}
		}
		return Operation{}, err
	}
	return m.loadOperation(ctx, operationID, identity.SourceID)
}

func isRetryableDBConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"database is locked",
		"database table is locked",
		"serialization failure",
		"could not serialize",
		"deadlock detected",
		"duplicate key",
		"unique constraint",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func (m *Module) recoverConcurrentExecute(
	ctx context.Context,
	sourceID, planID, idempotencyKey, requestDigest string,
) (Operation, error) {
	var existing db.OpenOperation
	err := m.db.WithContext(ctx).Where(
		"source_id = ? AND scope = ? AND idempotency_key = ?",
		sourceID, "decision_execute", idempotencyKey,
	).First(&existing).Error
	if err == nil {
		if existing.RequestDigest != requestDigest {
			return Operation{}, openaccess.NewError("idempotency_conflict", "idempotency key is already bound to another request")
		}
		return m.loadOperation(ctx, existing.ID, sourceID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Operation{}, err
	}
	err = m.db.WithContext(ctx).Where("source_id = ? AND plan_id = ?", sourceID, planID).First(&existing).Error
	if err == nil {
		return Operation{}, openaccess.NewError("stale_plan", "decision plan has already been accepted")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Operation{}, err
	}
	return Operation{}, openaccess.NewError("not_found", "concurrent operation was not found")
}

func (m *Module) GetOperation(ctx context.Context, identity openaccess.Identity, operationID string) (Operation, error) {
	policy, err := m.access.ActivePolicy(ctx)
	if err != nil {
		return Operation{}, err
	}
	if !policy.AllowsAction("decision.read") {
		return Operation{}, openaccess.NewError("forbidden", "decision reading is not allowed by the active policy")
	}
	return m.loadOperation(ctx, strings.TrimSpace(operationID), identity.SourceID)
}

func (m *Module) ProcessNext(ctx context.Context) (bool, error) {
	leaseToken, err := openaccess.RandomID("lease")
	if err != nil {
		return false, err
	}
	now := m.now().UTC()
	var claimed db.OpenOutbox
	recovering := false
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where(
			"state IN ? AND next_attempt_at <= ? AND (leased_until IS NULL OR leased_until < ?)",
			[]string{StateQueued, StateClaimed, StateDispatching}, now, now,
		).Where(
			`NOT EXISTS (
				SELECT 1
				FROM open_operation_actions AS current_action
				JOIN open_operation_actions AS prior_action
				  ON prior_action.operation_id = current_action.operation_id
				 AND prior_action.position < current_action.position
				WHERE current_action.id = open_outboxes.operation_action_id
				  AND prior_action.state <> ?
			)`,
			StateSucceeded,
		).Order("id ASC").First(&claimed)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		if query.Error != nil {
			return query.Error
		}
		recovering = claimed.State == StateDispatching
		nextState := StateClaimed
		if recovering {
			nextState = StateDispatching
		}
		leasedUntil := now.Add(m.leaseDuration)
		result := tx.Model(&db.OpenOutbox{}).
			Where("id = ? AND state = ? AND (leased_until IS NULL OR leased_until < ?)", claimed.ID, claimed.State, now).
			Updates(map[string]any{
				"state": nextState, "lease_token": leaseToken,
				"leased_until": leasedUntil, "attempt": gorm.Expr("attempt + 1"), "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		claimed.State = nextState
		claimed.LeaseToken = leaseToken
		claimed.LeasedUntil = &leasedUntil
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim decision outbox: %w", err)
	}
	if err := m.dispatchClaimed(ctx, claimed, recovering); err != nil {
		return true, err
	}
	return true, nil
}

func (m *Module) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		processed, _ := m.ProcessNext(ctx)
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Module) dispatchClaimed(ctx context.Context, outbox db.OpenOutbox, recovering bool) error {
	var action db.OpenOperationAction
	if err := m.db.WithContext(ctx).First(&action, outbox.OperationActionID).Error; err != nil {
		return m.finishOrphanedClaim(ctx, outbox, "operation_action_missing", "operation action is unavailable")
	}
	var operation db.OpenOperation
	if err := m.db.WithContext(ctx).First(&operation, "id = ?", action.OperationID).Error; err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", "operation_missing", "operation is unavailable", nil)
	}
	var plan db.DecisionPlan
	if err := m.db.WithContext(ctx).First(&plan, "id = ?", operation.PlanID).Error; err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", "plan_missing", "decision plan is unavailable", nil)
	}
	if m.jira == nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", "upstream_unavailable", "Jira execution adapter is unavailable", nil)
	}
	if recovering {
		binding := db.JiraExecutionBinding{
			ProjectRef: plan.ProjectRef, ActionClass: action.ActionType,
			ConnectorRef: action.ConnectorRef, ExecutorRef: action.ExecutorRef,
			Version: action.BindingVersion, Status: openaccess.SourceActive,
		}
		if strings.TrimSpace(binding.ConnectorRef) == "" || strings.TrimSpace(binding.ExecutorRef) == "" {
			var bindingErr error
			binding, bindingErr = m.access.ExecutionBinding(ctx, plan.ProjectRef, action.ActionType)
			if bindingErr != nil {
				return m.finishClaim(ctx, outbox, action, StateUnknown, "", "outcome_unknown", "recovered dispatch binding is unavailable", nil)
			}
		}
		confirmed, verifyErr := m.jira.ReadIssueState(ctx, binding, plan.TargetRef)
		if verifyErr == nil && currentValue(confirmed, action.ActionType) == action.DesiredValue {
			evidence := map[string]any{"after": confirmed, "recovery": "verification_only"}
			return m.finishClaim(ctx, outbox, action, StateSucceeded, action.DesiredValue, "", "", evidence)
		}
		evidence := map[string]any{
			"after": confirmed, "verification_error": errorText(verifyErr),
			"recovery": "verification_only",
		}
		return m.finishClaim(ctx, outbox, action, StateUnknown, "", "outcome_unknown", "recovered dispatch could not be confirmed", evidence)
	}
	if _, err := m.access.Source(ctx, operation.SourceID); err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", openaccess.ErrorCode(err), err.Error(), nil)
	}
	if err := m.access.CredentialActive(ctx, operation.SourceID, operation.KeyID); err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", openaccess.ErrorCode(err), err.Error(), nil)
	}
	policy, err := m.access.ActivePolicy(ctx)
	if err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", openaccess.ErrorCode(err), err.Error(), nil)
	}
	if err := authorizeChanges(policy, plan.ProjectRef, []Change{{Action: action.ActionType, ToValue: action.DesiredValue}}); err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", openaccess.ErrorCode(err), err.Error(), nil)
	}
	if rule := strings.ToLower(strings.TrimSpace(policy.ApprovalRules[action.ActionType])); rule != "" && rule != "auto" {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", "forbidden", "current policy requires approval for this action", nil)
	}
	binding, err := m.access.ExecutionBinding(ctx, plan.ProjectRef, action.ActionType)
	if err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", openaccess.ErrorCode(err), err.Error(), nil)
	}
	var preconditions Preconditions
	if err := json.Unmarshal([]byte(plan.PreconditionsJSON), &preconditions); err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", "invalid_plan", "decision plan preconditions are invalid", nil)
	}
	current, err := m.jira.ReadIssueState(ctx, binding, plan.TargetRef)
	if err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", "upstream_unavailable", "Jira state could not be read before dispatch", nil)
	}
	if err := authorizeRemoteIssue(policy, current); err != nil {
		return m.finishClaim(ctx, outbox, action, StateFailed, "", openaccess.ErrorCode(err), err.Error(), nil)
	}
	expected, ok := preconditionFor(preconditions, action.ActionType)
	if !ok || currentValue(current, action.ActionType) != expected {
		evidence := map[string]any{"before": current}
		return m.finishClaim(ctx, outbox, action, StateFailed, "", "stale_plan", "Jira state changed after the plan was prepared", evidence)
	}
	action.ConnectorRef = binding.ConnectorRef
	action.ExecutorRef = binding.ExecutorRef
	action.BindingVersion = binding.Version
	dispatchedAt := m.now().UTC()
	if err := m.markDispatching(ctx, outbox, action, dispatchedAt); err != nil {
		return err
	}
	dispatchErr := m.dispatch(ctx, binding, plan.TargetRef, action)
	confirmed, verifyErr := m.jira.ReadIssueState(ctx, binding, plan.TargetRef)
	if verifyErr == nil && currentValue(confirmed, action.ActionType) == action.DesiredValue {
		evidence := map[string]any{
			"before": current, "after": confirmed, "dispatch_error": errorText(dispatchErr),
		}
		return m.finishClaim(ctx, outbox, action, StateSucceeded, action.DesiredValue, "", "", evidence)
	}
	if dispatchErr != nil && verifyErr == nil && dispatchDefinitelyFailed(dispatchErr) {
		evidence := map[string]any{
			"before": current, "after": confirmed, "dispatch": "definite_rejection",
		}
		return m.finishClaim(ctx, outbox, action, StateFailed, "", "upstream_rejected", "Jira rejected the requested change", evidence)
	}
	evidence := map[string]any{
		"before": current, "after": confirmed, "dispatch_error": errorText(dispatchErr),
		"verification_error": errorText(verifyErr),
	}
	return m.finishClaim(ctx, outbox, action, StateUnknown, "", "outcome_unknown", "Jira outcome could not be confirmed", evidence)
}

func (m *Module) markDispatching(
	ctx context.Context,
	outbox db.OpenOutbox,
	action db.OpenOperationAction,
	dispatchedAt time.Time,
) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&db.OpenOutbox{}).
			Where("id = ? AND lease_token = ? AND state = ?", outbox.ID, outbox.LeaseToken, StateClaimed).
			Updates(map[string]any{
				"state": StateDispatching, "leased_until": dispatchedAt.Add(m.leaseDuration),
				"updated_at": dispatchedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errDecisionLeaseLost
		}
		result = tx.Model(&db.OpenOperationAction{}).Where("id = ?", action.ID).
			Updates(map[string]any{
				"state": StateDispatching, "dispatched_at": dispatchedAt, "updated_at": dispatchedAt,
				"connector_ref": action.ConnectorRef, "executor_ref": action.ExecutorRef,
				"binding_version": action.BindingVersion,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("operation action is unavailable")
		}
		return nil
	})
}

func (m *Module) dispatch(
	ctx context.Context,
	binding db.JiraExecutionBinding,
	issueKey string,
	action db.OpenOperationAction,
) error {
	switch action.ActionType {
	case ActionReassign:
		return m.jira.Assign(ctx, binding, issueKey, action.DesiredValue)
	case ActionReschedule:
		return m.jira.SetDueDate(ctx, binding, issueKey, action.DesiredValue)
	default:
		return fmt.Errorf("unsupported action %s", action.ActionType)
	}
}

func (m *Module) finishClaim(
	ctx context.Context,
	outbox db.OpenOutbox,
	action db.OpenOperationAction,
	state, confirmed, errorCode, errorMessage string,
	evidence any,
) error {
	now := m.now().UTC()
	evidenceJSON := ""
	if evidence != nil {
		value, _ := json.Marshal(evidence)
		evidenceJSON = string(value)
	}
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		outboxResult := tx.Model(&db.OpenOutbox{}).
			Where("id = ? AND lease_token = ? AND state IN ?", outbox.ID, outbox.LeaseToken, []string{StateClaimed, StateDispatching}).
			Updates(map[string]any{
				"state": state, "lease_token": "", "leased_until": nil,
				"updated_at": now,
			})
		if outboxResult.Error != nil {
			return outboxResult.Error
		}
		if outboxResult.RowsAffected != 1 {
			return errDecisionLeaseLost
		}
		actionUpdates := map[string]any{
			"state": state, "confirmed_remote_value": confirmed,
			"remote_evidence_json": evidenceJSON, "last_error_code": errorCode,
			"last_error": errorMessage, "updated_at": now,
		}
		if state == StateSucceeded {
			actionUpdates["confirmed_at"] = now
		}
		actionResult := tx.Model(&db.OpenOperationAction{}).Where("id = ?", action.ID).Updates(actionUpdates)
		if actionResult.Error != nil {
			return actionResult.Error
		}
		if actionResult.RowsAffected != 1 {
			return fmt.Errorf("operation action is unavailable")
		}
		if state == StateFailed || state == StateUnknown {
			if err := m.failRemainingActions(tx, action, state, now); err != nil {
				return err
			}
		}
		return m.refreshOperationState(tx, action.OperationID, now)
	})
}

func (m *Module) failRemainingActions(
	tx *gorm.DB,
	current db.OpenOperationAction,
	currentState string,
	now time.Time,
) error {
	var remaining []db.OpenOperationAction
	if err := tx.Where(
		"operation_id = ? AND position > ? AND state IN ?",
		current.OperationID, current.Position, []string{StateQueued, StateClaimed},
	).Find(&remaining).Error; err != nil {
		return err
	}
	if len(remaining) == 0 {
		return nil
	}
	actionIDs := make([]uint, 0, len(remaining))
	errorCode := "previous_action_not_confirmed"
	errorMessage := "a previous operation action did not reach confirmed success"
	if currentState == StateUnknown {
		errorMessage = "a previous operation action has an unknown remote outcome"
	}
	for _, action := range remaining {
		actionIDs = append(actionIDs, action.ID)
	}
	if err := tx.Model(&db.OpenOperationAction{}).
		Where("id IN ? AND state IN ?", actionIDs, []string{StateQueued, StateClaimed}).
		Updates(map[string]any{
			"state": StateFailed, "last_error_code": errorCode,
			"last_error": errorMessage, "updated_at": now,
		}).Error; err != nil {
		return err
	}
	return tx.Model(&db.OpenOutbox{}).
		Where("operation_action_id IN ? AND state IN ?", actionIDs, []string{StateQueued, StateClaimed}).
		Updates(map[string]any{
			"state": StateFailed, "lease_token": "", "leased_until": nil,
			"updated_at": now,
		}).Error
}

func (m *Module) finishOrphanedClaim(
	ctx context.Context,
	outbox db.OpenOutbox,
	errorCode, errorMessage string,
) error {
	result := m.db.WithContext(ctx).Model(&db.OpenOutbox{}).
		Where("id = ? AND lease_token = ? AND state IN ?", outbox.ID, outbox.LeaseToken, []string{StateClaimed, StateDispatching}).
		Updates(map[string]any{
			"state": StateFailed, "lease_token": "", "leased_until": nil,
			"updated_at": m.now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errDecisionLeaseLost
	}
	return nil
}

func (m *Module) refreshOperationState(tx *gorm.DB, operationID string, now time.Time) error {
	var actions []db.OpenOperationAction
	if err := tx.Where("operation_id = ?", operationID).Find(&actions).Error; err != nil {
		return err
	}
	state := aggregateState(actions)
	errorCode := ""
	lastError := ""
	for _, action := range actions {
		if action.LastErrorCode != "" {
			errorCode, lastError = action.LastErrorCode, action.LastError
			break
		}
	}
	return tx.Model(&db.OpenOperation{}).Where("id = ?", operationID).
		Updates(map[string]any{
			"state": state, "last_error_code": errorCode,
			"last_error": lastError, "updated_at": now,
		}).Error
}

func (m *Module) loadOperation(ctx context.Context, operationID, sourceID string) (Operation, error) {
	var record db.OpenOperation
	err := m.db.WithContext(ctx).
		Where("id = ? AND source_id = ?", operationID, sourceID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Operation{}, openaccess.NewError("not_found", "operation was not found")
	}
	if err != nil {
		return Operation{}, fmt.Errorf("load operation: %w", err)
	}
	var actions []db.OpenOperationAction
	if err := m.db.WithContext(ctx).Where("operation_id = ?", record.ID).Order("id ASC").Find(&actions).Error; err != nil {
		return Operation{}, fmt.Errorf("load operation actions: %w", err)
	}
	result := Operation{
		ID: record.ID, PlanID: record.PlanID, SourceID: record.SourceID,
		State: record.State, ErrorCode: record.LastErrorCode,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
	for _, action := range actions {
		var evidence any
		if action.RemoteEvidenceJSON != "" {
			_ = json.Unmarshal([]byte(action.RemoteEvidenceJSON), &evidence)
		}
		result.Actions = append(result.Actions, ActionResult{
			ID: action.ID, Action: action.ActionType, DesiredValue: action.DesiredValue,
			ConfirmedRemoteValue: action.ConfirmedRemoteValue, State: action.State,
			RemoteEvidence: evidence, ErrorCode: action.LastErrorCode,
			DispatchedAt: action.DispatchedAt, ConfirmedAt: action.ConfirmedAt,
		})
	}
	return result, nil
}

func prepareChanges(policy openaccess.Policy, current JiraState, request PrepareRequest) ([]Change, error) {
	changes := []Change{}
	if request.Assignee != nil {
		value := strings.TrimSpace(*request.Assignee)
		if !policyAllowsDecision(policy, ActionReassign) {
			return nil, openaccess.NewError("forbidden", "reassignment is not allowed by the active policy")
		}
		if value == strings.TrimSpace(current.Assignee) {
			return nil, openaccess.NewError("invalid_query", "assignee already has the requested value")
		}
		changes = append(changes, Change{Action: ActionReassign, FromValue: current.Assignee, ToValue: value})
	}
	if request.DueDate != nil {
		value := strings.TrimSpace(*request.DueDate)
		if !policyAllowsDecision(policy, ActionReschedule) {
			return nil, openaccess.NewError("forbidden", "rescheduling is not allowed by the active policy")
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return nil, openaccess.NewError("invalid_query", "due_date must use YYYY-MM-DD")
		}
		if value == strings.TrimSpace(current.DueDate) {
			return nil, openaccess.NewError("invalid_query", "due date already has the requested value")
		}
		changes = append(changes, Change{Action: ActionReschedule, FromValue: current.DueDate, ToValue: value})
	}
	if len(changes) == 0 {
		return nil, openaccess.NewError("invalid_query", "prepare requires an assignee or due_date change")
	}
	return changes, nil
}

func authorizeChanges(policy openaccess.Policy, project string, changes []Change) error {
	if !policy.AllowsAction("decision.execute") {
		return openaccess.NewError("forbidden", "decision execute is not allowed by the active policy")
	}
	if !policy.AllowsProject(project) {
		return openaccess.NewError("forbidden", "requested Jira scope is not published")
	}
	for _, change := range changes {
		if !policyAllowsDecision(policy, change.Action) {
			return openaccess.NewError("forbidden", "decision action is not allowed by the active policy")
		}
	}
	return nil
}

func policyAllowsDecision(policy openaccess.Policy, action string) bool {
	return policy.AllowsAction("decision." + action)
}

func requestedActionNames(request PrepareRequest) []string {
	actions := []string{}
	if request.Assignee != nil {
		actions = append(actions, ActionReassign)
	}
	if request.DueDate != nil {
		actions = append(actions, ActionReschedule)
	}
	return actions
}

func compatibleBindings(bindings []db.JiraExecutionBinding) error {
	if len(bindings) == 0 {
		return openaccess.NewError("invalid_query", "prepare requires an assignee or due_date change")
	}
	first := bindings[0]
	for _, binding := range bindings[1:] {
		if binding.ConnectorRef != first.ConnectorRef || binding.ExecutorRef != first.ExecutorRef {
			return openaccess.NewError("forbidden", "decision actions resolve to incompatible Jira execution bindings")
		}
	}
	return nil
}

func authorizeRemoteIssue(policy openaccess.Policy, state JiraState) error {
	if strings.TrimSpace(state.SecurityID) != "" && !policy.AllowsRestrictedJiraIssues() {
		return openaccess.NewError("forbidden", "restricted Jira issues are not published for decision operations")
	}
	return nil
}

func normalizeIssueKey(raw string) (string, string, error) {
	key := strings.ToUpper(strings.TrimSpace(raw))
	parts := strings.SplitN(key, "-", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", openaccess.NewError("invalid_query", "a valid Jira issue key is required")
	}
	return key, parts[0], nil
}

func preconditionFor(preconditions Preconditions, action string) (string, bool) {
	for _, change := range preconditions.Values {
		if change.Action == action {
			return change.FromValue, true
		}
	}
	return "", false
}

func currentValue(state JiraState, action string) string {
	switch action {
	case ActionReassign:
		return strings.TrimSpace(state.Assignee)
	case ActionReschedule:
		return strings.TrimSpace(state.DueDate)
	default:
		return ""
	}
}

func aggregateState(actions []db.OpenOperationAction) string {
	var succeeded, failed, unknown, pending int
	for _, action := range actions {
		switch action.State {
		case StateSucceeded:
			succeeded++
		case StateFailed:
			failed++
		case StateUnknown:
			unknown++
		default:
			pending++
		}
	}
	if unknown > 0 {
		return StateUnknown
	}
	if pending > 0 {
		return StateDispatching
	}
	if succeeded == len(actions) {
		return StateSucceeded
	}
	if succeeded > 0 && failed > 0 {
		return StatePartial
	}
	return StateFailed
}

func digest(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func dispatchDefinitelyFailed(err error) bool {
	type definiteFailure interface {
		DefiniteFailure() bool
	}
	var target definiteFailure
	return errors.As(err, &target) && target.DefiniteFailure()
}
