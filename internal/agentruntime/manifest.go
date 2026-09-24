package agentruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"sort"
	"strings"
)

// CapabilityKind constants
const (
	KindSkill           = "skill"
	KindMCP             = "mcp"
	KindPlugin          = "plugin"
	KindPolicy          = "policy"
	KindModelProfile    = "model_profile"
	KindContextProvider = "context_provider"
)

// LoadLevel constants
const (
	LoadLevelL0 = "L0" // Manifest, ID, permissions, budgets, dependencies
	LoadLevelL1 = "L1" // Guidance, instructions, contract
	LoadLevelL2 = "L2" // Relevant evidence, diff hunks, facts
	LoadLevelL3 = "L3" // Raw blobs, full files, full history (lazy)
)

// DependencyRef defines a declared dependency on another capability.
type DependencyRef struct {
	ID         string `yaml:"id" json:"id"`
	Kind       string `yaml:"kind" json:"kind"`
	Version    string `yaml:"version" json:"version"` // e.g. ">=2", "^1.0", "*"
	IsOptional bool   `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// ResourceDef defines a sliced resource chunk belonging to a capability.
type ResourceDef struct {
	Key           string `yaml:"key" json:"key"`                       // e.g. "review-method"
	LoadLevel     string `yaml:"load_level" json:"load_level"`         // L0, L1, L2, L3
	ContentRef    string `yaml:"content_ref" json:"content_ref"`       // e.g. "blob:sha256:..."
	ContentKind   string `yaml:"content_kind,omitempty" json:"content_kind,omitempty"` // text, json, schema
	Content       string `yaml:"content,omitempty" json:"content,omitempty"`
	TokenEstimate int    `yaml:"token_estimate,omitempty" json:"token_estimate,omitempty"`
}

// BudgetDef defines runtime resource constraints.
type BudgetDef struct {
	InstructionTokens int `yaml:"instruction_tokens" json:"instruction_tokens"`
	EvidenceTokens    int `yaml:"evidence_tokens" json:"evidence_tokens"`
	RawTokens         int `yaml:"raw_tokens" json:"raw_tokens"`
	ToolCalls         int `yaml:"tool_calls" json:"tool_calls"`
	WallTimeSeconds   int `yaml:"wall_time_seconds" json:"wall_time_seconds"`
}

// ValidationDef defines the automated verification suite for the capability.
type ValidationDef struct {
	Suite    string   `yaml:"suite" json:"suite"`
	Required []string `yaml:"required" json:"required"`
}

// ActivationDef defines policy for activating this capability.
type ActivationDef struct {
	Mode           string `yaml:"mode" json:"mode"` // human_approved, canary_required
	CanaryRequired bool   `yaml:"canary_required" json:"canary_required"`
}

// CapabilityManifest is the canonical specification of an agent capability.
type CapabilityManifest struct {
	Schema      string `yaml:"schema" json:"schema"` // "capability-manifest/v1"
	ID          string `yaml:"id" json:"id"`
	Kind        string `yaml:"kind" json:"kind"`
	Version     int    `yaml:"version" json:"version"`
	Digest      string `yaml:"digest,omitempty" json:"digest,omitempty"`
	Owner       string `yaml:"owner,omitempty" json:"owner,omitempty"`
	Sensitivity string `yaml:"sensitivity,omitempty" json:"sensitivity,omitempty"` // public, internal, restricted
	Runtime     struct {
		MinKernel string `yaml:"min_kernel" json:"min_kernel"`
	} `yaml:"runtime" json:"runtime"`
	Scope struct {
		Type string `yaml:"type" json:"type"` // global, tenant, project, repository
		ID   string `yaml:"id,omitempty" json:"id,omitempty"`
	} `yaml:"scope" json:"scope"`
	Triggers struct {
		Intents    []string `yaml:"intents" json:"intents"`
		Conditions []string `yaml:"conditions" json:"conditions"`
	} `yaml:"triggers" json:"triggers"`
	Provides    []string        `yaml:"provides" json:"provides"`
	Requires    []DependencyRef `yaml:"requires" json:"requires"`
	Optional    []DependencyRef `yaml:"optional,omitempty" json:"optional,omitempty"`
	Permissions []string        `yaml:"permissions" json:"permissions"`
	Resources   []ResourceDef   `yaml:"resources" json:"resources"`
	Tools       []string        `yaml:"tools" json:"tools"`
	Budgets     BudgetDef       `yaml:"budgets" json:"budgets"`
	Validation  ValidationDef   `yaml:"validation" json:"validation"`
	Activation  ActivationDef   `yaml:"activation" json:"activation"`
}

// ParseManifest parses a YAML or JSON capability manifest.
func ParseManifest(data []byte) (*CapabilityManifest, error) {
	var m CapabilityManifest
	// Try YAML first (YAML parser also handles JSON seamlessly)
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse capability manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if m.Digest == "" {
		m.Digest = m.ComputeDigest()
	}
	return &m, nil
}

// Validate checks manifest correctness and safety rules.
func (m *CapabilityManifest) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("manifest ID cannot be empty")
	}
	if m.Version <= 0 {
		return fmt.Errorf("manifest version must be positive integer")
	}
	switch m.Kind {
	case KindSkill, KindMCP, KindPlugin, KindPolicy, KindModelProfile, KindContextProvider:
		// Valid
	default:
		return fmt.Errorf("unsupported capability kind: %s", m.Kind)
	}
	if m.Scope.Type == "" {
		m.Scope.Type = "global"
	}
	return nil
}

// ComputeDigest computes a deterministic SHA-256 digest of the manifest content.
func (m *CapabilityManifest) ComputeDigest() string {
	// Normalize structure without Digest field for self-checksumming
	clone := *m
	clone.Digest = ""
	
	// Canonical JSON serialization
	normalizedJSON, err := json.Marshal(clone)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(normalizedJSON)
	return "sha256:" + hex.EncodeToString(hash[:])
}

// MatchIntent checks if the manifest supports the requested intent and conditions.
func (m *CapabilityManifest) MatchIntent(intent string, conditions []string) bool {
	matchedIntent := false
	for _, it := range m.Triggers.Intents {
		if strings.EqualFold(it, intent) || it == "*" {
			matchedIntent = true
			break
		}
	}
	if !matchedIntent {
		return false
	}
	if len(m.Triggers.Conditions) == 0 {
		return true
	}
	condMap := make(map[string]bool)
	for _, c := range conditions {
		condMap[strings.ToLower(c)] = true
	}
	for _, reqCond := range m.Triggers.Conditions {
		if !condMap[strings.ToLower(reqCond)] {
			return false
		}
	}
	return true
}

// CheckPermissionGrant ensures the capability's requested permissions are within granted permissions.
func (m *CapabilityManifest) CheckPermissionGrant(granted []string) (bool, []string) {
	if len(m.Permissions) == 0 {
		return true, nil
	}
	grantMap := make(map[string]bool)
	for _, g := range granted {
		grantMap[strings.ToLower(g)] = true
		if g == "*" {
			return true, nil
		}
	}
	var missing []string
	for _, p := range m.Permissions {
		if !grantMap[strings.ToLower(p)] {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return false, missing
	}
	return true, nil
}
