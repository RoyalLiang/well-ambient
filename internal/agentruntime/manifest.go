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
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
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

// GenerateSkillMarkdown converts a CapabilityManifest and its loaded resource slices into a standard SKILL.md document.
func (m *CapabilityManifest) GenerateSkillMarkdown(resources []ResourceDef) string {
	var sb strings.Builder

	// 1. YAML Frontmatter
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", m.ID))
	if m.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", m.Description))
	} else {
		sb.WriteString(fmt.Sprintf("description: Agent capability %s (%s)\n", m.ID, m.Kind))
	}
	sb.WriteString(fmt.Sprintf("kind: %s\n", m.Kind))
	sb.WriteString(fmt.Sprintf("version: %d\n", m.Version))
	if m.Owner != "" {
		sb.WriteString(fmt.Sprintf("owner: %s\n", m.Owner))
	}
	if len(m.Permissions) > 0 {
		sb.WriteString("permissions:\n")
		for _, p := range m.Permissions {
			sb.WriteString(fmt.Sprintf("  - %s\n", p))
		}
	}
	if len(m.Tools) > 0 {
		sb.WriteString("tools:\n")
		for _, t := range m.Tools {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
	}
	if len(m.Requires) > 0 {
		sb.WriteString("requires:\n")
		for _, r := range m.Requires {
			sb.WriteString(fmt.Sprintf("  - id: %s\n    kind: %s\n    version: \"%s\"\n", r.ID, r.Kind, r.Version))
		}
	}
	sb.WriteString("---\n\n")

	// 2. Title & Overview
	title := m.ID
	if m.ID == "code_review" {
		title = "Code Review (代码审查与合规治理技能)"
	} else if m.ID == "gitlab.snapshot" {
		title = "GitLab Snapshot (代码快照提取插件)"
	} else if m.ID == "knowledge.search" {
		title = "Knowledge Search (领域知识检索提供方)"
	}
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	if m.Description != "" {
		sb.WriteString(fmt.Sprintf("> %s\n\n", m.Description))
	}

	// 3. Hierarchy & Included Components
	if len(m.Requires) > 0 {
		sb.WriteString("## 包含的微内核能力组件 (Included Components)\n\n")
		sb.WriteString("本技能通过微内核声明式组装以下依赖组件与能力插件：\n\n")
		for _, req := range m.Requires {
			compDesc := "下属依赖切片"
			if req.ID == "gitlab.snapshot" {
				compDesc = "负责拉取合并请求 (MR) 的完整 Diff、Commit 变更日志与分支元数据"
			} else if req.ID == "knowledge.search" {
				compDesc = "负责检索系统领域知识、业务术语与代码架构合规准则"
			}
			sb.WriteString(fmt.Sprintf("- **`%s`** (`%s` 版本: `%s`)：%s\n", req.ID, req.Kind, req.Version, compDesc))
		}
		sb.WriteString("\n")
	}

	// 4. Intent Triggers
	if len(m.Triggers.Intents) > 0 {
		sb.WriteString("## 触发意图与适用场景 (Triggers & Intents)\n\n")
		sb.WriteString("当求解器识别到以下任务意图时，将自动调度本能力：\n\n")
		for _, intent := range m.Triggers.Intents {
			sb.WriteString(fmt.Sprintf("- 🎯 `%s`\n", intent))
		}
		sb.WriteString("\n")
	}

	// 5. Tools & Permissions
	sb.WriteString("## 工具权限与执行契约 (Tools & Permissions)\n\n")
	if len(m.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("- **受权工具**: `%s`\n", strings.Join(m.Tools, "`, `")))
	}
	if len(m.Permissions) > 0 {
		sb.WriteString(fmt.Sprintf("- **所需权限**: `%s`\n", strings.Join(m.Permissions, "`, `")))
	}
	if m.Budgets.InstructionTokens > 0 || m.Budgets.EvidenceTokens > 0 {
		sb.WriteString(fmt.Sprintf("- **资源预算**: 指令 %d tokens，证据 %d tokens，最多允许 %d 次工具调用，超时 %d 秒\n",
			m.Budgets.InstructionTokens, m.Budgets.EvidenceTokens, m.Budgets.ToolCalls, m.Budgets.WallTimeSeconds))
	}
	sb.WriteString("\n")

	// 6. Resources & Instructions
	sb.WriteString("## 核心指令与资源切片 (Resource Slices)\n\n")
	if len(resources) == 0 {
		sb.WriteString("*暂无加载的具体指令切片。*\n")
	} else {
		for _, res := range resources {
			sb.WriteString(fmt.Sprintf("### 切片: `%s` (%s)\n\n", res.Key, res.LoadLevel))
			if res.ContentKind == "json" {
				sb.WriteString("```json\n")
				sb.WriteString(strings.TrimSpace(res.Content))
				sb.WriteString("\n```\n\n")
			} else {
				sb.WriteString("```markdown\n")
				sb.WriteString(strings.TrimSpace(res.Content))
				sb.WriteString("\n```\n\n")
			}
		}
	}

	return sb.String()
}

