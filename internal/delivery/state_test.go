package delivery

import "testing"

func TestRunStateTransitions(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{RunPending, RunPreflightReady, true},
		{RunPending, RunExecuting, false},
		{RunPreflightBlocked, RunPreflightReady, true},
		{RunPreflightReady, RunExecuting, true},
		{RunExecuting, RunDraftMRCreated, true},
		{RunDraftMRCreated, RunReviewPending, true},
		{RunReviewPending, RunAcceptancePending, true},
		{RunAcceptancePending, RunDelivered, true},
		{RunDelivered, RunExecuting, false},
		{" EXECUTING ", "executing", true},
	}

	for _, tt := range tests {
		if got := CanTransitionRun(tt.from, tt.to); got != tt.want {
			t.Errorf("CanTransitionRun(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestTerminalRunStates(t *testing.T) {
	for _, state := range []string{RunDelivered, RunRejected, RunCancelled} {
		if !IsTerminalRun(state) {
			t.Errorf("%s should be terminal", state)
		}
	}
	if IsTerminalRun(RunExecuting) {
		t.Fatal("executing should not be terminal")
	}
}
