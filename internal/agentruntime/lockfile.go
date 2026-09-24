package agentruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// RefInfo provides a versioned reference with content digest.
type RefInfo struct {
	ID      string `json:"id,omitempty"`
	Version any    `json:"version,omitempty"` // string or int
	Digest  string `json:"digest"`
}

// BoundCapability records an exact capability version locked for an AgentRun.
type BoundCapability struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Version         int      `json:"version"`
	Digest          string   `json:"digest"`
	SelectionReason string   `json:"selection_reason"`
	LoadLevels      []string `json:"load_levels,omitempty"` // e.g. ["L0", "L1", "L2"]
}

// PermissionGrantInfo records the immutable authorization snapshot.
type PermissionGrantInfo struct {
	ID      string   `json:"id"`
	Digest  string   `json:"digest"`
	Actions []string `json:"actions"`
}

// LockfileBudgets defines the frozen runtime boundaries.
type LockfileBudgets struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ToolCalls       int `json:"tool_calls"`
	WallTimeSeconds int `json:"wall_time_seconds"`
}

// RunLockfile records the complete, immutable dependency graph and authorization boundary of an AgentRun.
type RunLockfile struct {
	Schema          string              `json:"schema"` // "agent-run-lockfile/v1"
	RunID           string              `json:"run_id"`
	AgentKind       string              `json:"agent_kind"`
	Kernel          RefInfo             `json:"kernel"`
	ModelProfile    RefInfo             `json:"model_profile"`
	Capabilities    []BoundCapability   `json:"capabilities"`
	ContextPack     RefInfo             `json:"context_pack"`
	PermissionGrant PermissionGrantInfo `json:"permission_grant"`
	Budgets         LockfileBudgets     `json:"budgets"`
	Resolver        RefInfo             `json:"resolver"`
}

// ComputeHash calculates the immutable SHA-256 content address of the lockfile.
func (l *RunLockfile) ComputeHash() (string, error) {
	if l == nil {
		return "", fmt.Errorf("cannot compute hash for nil lockfile")
	}
	bytes, err := json.Marshal(l)
	if err != nil {
		return "", fmt.Errorf("failed to marshal lockfile: %w", err)
	}
	h := sha256.Sum256(bytes)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// VerifyHash verifies whether the lockfile matches an expected content address.
func (l *RunLockfile) VerifyHash(expectedHash string) bool {
	h, err := l.ComputeHash()
	if err != nil {
		return false
	}
	return h == expectedHash
}
