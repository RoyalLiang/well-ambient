package delivery

import "strings"

const (
	SpecDraft  = "draft"
	SpecFrozen = "frozen"

	ReviewDraft    = "draft"
	ReviewApproved = "approved"

	RunPending           = "pending"
	RunPreflightBlocked  = "preflight_blocked"
	RunPreflightReady    = "preflight_ready"
	RunExecuting         = "executing"
	RunExecutionFailed   = "execution_failed"
	RunTestsFailed       = "tests_failed"
	RunDraftMRCreated    = "draft_mr_created"
	RunReviewPending     = "review_pending"
	RunAcceptancePending = "acceptance_pending"
	RunDelivered         = "delivered"
	RunRejected          = "rejected"
	RunCancelled         = "cancelled"
)

var runTransitions = map[string]map[string]bool{
	RunPending: {
		RunPreflightBlocked: true,
		RunPreflightReady:   true,
		RunCancelled:        true,
	},
	RunPreflightBlocked: {
		RunPreflightReady: true,
		RunCancelled:      true,
	},
	RunPreflightReady: {
		RunExecuting: true,
		RunCancelled: true,
	},
	RunExecuting: {
		RunExecutionFailed: true,
		RunTestsFailed:     true,
		RunDraftMRCreated:  true,
		RunCancelled:       true,
	},
	RunTestsFailed: {
		RunExecuting:     true,
		RunReviewPending: true,
		RunCancelled:     true,
	},
	RunExecutionFailed: {
		RunCancelled: true,
	},
	RunDraftMRCreated: {
		RunReviewPending: true,
		RunCancelled:     true,
	},
	RunReviewPending: {
		RunAcceptancePending: true,
		RunTestsFailed:       true,
		RunRejected:          true,
		RunCancelled:         true,
	},
	RunAcceptancePending: {
		RunDelivered: true,
		RunRejected:  true,
		RunCancelled: true,
	},
}

func NormalizeState(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func CanTransitionRun(from, to string) bool {
	from = NormalizeState(from)
	to = NormalizeState(to)
	if from == to {
		return true
	}
	return runTransitions[from][to]
}

func IsTerminalRun(status string) bool {
	switch NormalizeState(status) {
	case RunDelivered, RunRejected, RunCancelled:
		return true
	default:
		return false
	}
}
