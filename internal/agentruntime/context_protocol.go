package agentruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ContextFact represents a structured business rule, policy, or requirement.
type ContextFact struct {
	ID        string `json:"id"`        // F1, F2
	Type      string `json:"type"`      // rule, policy, constraint, requirement
	Statement string `json:"statement"` // fact summary
	SourceRef string `json:"source_ref"` // reference ID e.g. K12
}

// ContextChange represents a modified file or changed section.
type ContextChange struct {
	ID        string `json:"id"`        // C1, C2
	Path      string `json:"path"`      // file path
	Lines     string `json:"lines"`     // line range e.g. "38:51"
	Status    string `json:"status"`    // modified, added, deleted
}

// ContextEvidence represents verified evidence linking a claim/fact to code changes.
type ContextEvidence struct {
	ID           string `json:"id"`            // E1, E2
	TargetChange string `json:"target_change"` // C1:44
	Snippet      string `json:"snippet"`       // actual code line / quotation
}

// ContextOpenQuestion represents an unresolved uncertainty or missing context item.
type ContextOpenQuestion struct {
	ID       string `json:"id"`       // Q1, Q2
	Question string `json:"question"` // what needs confirmation
}

// ContextRef represents an external knowledge or specification reference.
type ContextRef struct {
	ID     string `json:"id"`     // K12, B1
	Target string `json:"target"` // knowledge:182@v3, blob:sha256:...
}

// ContextGraph represents the complete canonical context assembled for an AgentRun.
type ContextGraph struct {
	RunID             string                `json:"run_id"`
	AgentKind         string                `json:"agent_kind"`
	Goal              string                `json:"goal"`
	ScopeRepo         string                `json:"scope_repo"`
	ScopeMR           string                `json:"scope_mr"`
	ScopeHeadSHA      string                `json:"scope_head_sha"`
	BoundCapabilities []string              `json:"bound_capabilities"` // e.g. ["merge-review@3", "gitlab.snapshot@2"]
	InstructionBudget int                   `json:"instruction_budget"`
	EvidenceBudget    int                   `json:"evidence_budget"`
	RawBudget         int                   `json:"raw_budget"`
	Facts             []ContextFact         `json:"facts"`
	Changes           []ContextChange       `json:"changes"`
	Evidence          []ContextEvidence     `json:"evidence"`
	OpenQuestions     []ContextOpenQuestion `json:"open_questions"`
	Refs              []ContextRef          `json:"refs"`
}

// ComputeHash computes deterministic SHA-256 fingerprint of the ContextGraph.
func (g *ContextGraph) ComputeHash() string {
	raw := g.RenderCompactView()
	h := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(h[:])
}

// RenderCompactView renders the ContextGraph in the high-density ACP/1 format.
// This deterministic compact view reduces LLM token overhead by >= 30% compared to raw JSON.
func (g *ContextGraph) RenderCompactView() string {
	var b strings.Builder
	b.WriteString("@schema acp/1\n")
	b.WriteString(fmt.Sprintf("@run %s kind=%s goal=%s\n", g.RunID, g.AgentKind, g.Goal))
	if g.ScopeRepo != "" || g.ScopeMR != "" || g.ScopeHeadSHA != "" {
		b.WriteString(fmt.Sprintf("@scope repo=%s mr=%s head=%s\n", g.ScopeRepo, g.ScopeMR, g.ScopeHeadSHA))
	}
	if len(g.BoundCapabilities) > 0 {
		b.WriteString(fmt.Sprintf("@caps %s\n", strings.Join(g.BoundCapabilities, " ")))
	}
	b.WriteString(fmt.Sprintf("@budget instruction=%d evidence=%d raw=%d\n\n",
		g.InstructionBudget, g.EvidenceBudget, g.RawBudget))

	if len(g.Facts) > 0 {
		b.WriteString("@facts\n")
		for _, f := range g.Facts {
			b.WriteString(fmt.Sprintf("%s|%s|%s|%s\n", f.ID, f.Type, f.Statement, f.SourceRef))
		}
		b.WriteString("\n")
	}

	if len(g.Changes) > 0 {
		b.WriteString("@changes\n")
		for _, c := range g.Changes {
			b.WriteString(fmt.Sprintf("%s|%s|%s|%s\n", c.ID, c.Path, c.Lines, c.Status))
		}
		b.WriteString("\n")
	}

	if len(g.Evidence) > 0 {
		b.WriteString("@evidence\n")
		for _, e := range g.Evidence {
			b.WriteString(fmt.Sprintf("%s|%s|%s\n", e.ID, e.TargetChange, e.Snippet))
		}
		b.WriteString("\n")
	}

	if len(g.OpenQuestions) > 0 {
		b.WriteString("@open\n")
		for _, q := range g.OpenQuestions {
			b.WriteString(fmt.Sprintf("%s|%s\n", q.ID, q.Question))
		}
		b.WriteString("\n")
	}

	if len(g.Refs) > 0 {
		b.WriteString("@refs\n")
		for _, r := range g.Refs {
			b.WriteString(fmt.Sprintf("%s|%s\n", r.ID, r.Target))
		}
	}

	return strings.TrimSpace(b.String())
}

// EstimateTokens provides a deterministic token estimation for text.
func EstimateTokens(text string) int {
	// Standard heuristic: 1 token ~= 4 english/code characters, or ~= 1.5 CJK characters
	cjkCount := 0
	otherCount := 0
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			cjkCount++
		} else {
			otherCount++
		}
	}
	tokens := (otherCount + 3) / 4 + (cjkCount*2 + 2) / 3
	if tokens < 1 && len(text) > 0 {
		tokens = 1
	}
	return tokens
}

// RenderDelta generates a compact delta format against a base context pack hash.
func (g *ContextGraph) RenderDelta(baseHash string, addedFacts []ContextFact, removedQuestions []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("@base %s\n@delta\n", baseHash))
	for _, f := range addedFacts {
		b.WriteString(fmt.Sprintf("+%s|%s|%s|%s\n", f.ID, f.Type, f.Statement, f.SourceRef))
	}
	for _, qID := range removedQuestions {
		b.WriteString(fmt.Sprintf("-%s\n", qID))
	}
	return strings.TrimSpace(b.String())
}

// ApplyDelta applies incremental updates to a base ContextGraph.
func (g *ContextGraph) ApplyDelta(expectedBaseHash string, delta string) error {
	lines := strings.Split(delta, "\n")
	if len(lines) == 0 {
		return nil
	}
	var baseHeader string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@base ") {
			baseHeader = strings.TrimPrefix(line, "@base ")
			break
		}
	}
	if baseHeader != "" && baseHeader != expectedBaseHash {
		return fmt.Errorf("context delta base hash mismatch: expected %s, got %s", expectedBaseHash, baseHeader)
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "@") {
			continue
		}
		if strings.HasPrefix(line, "+F") {
			parts := strings.Split(strings.TrimPrefix(line, "+"), "|")
			if len(parts) >= 4 {
				g.Facts = append(g.Facts, ContextFact{
					ID:        parts[0],
					Type:      parts[1],
					Statement: parts[2],
					SourceRef: parts[3],
				})
			}
		} else if strings.HasPrefix(line, "-Q") {
			targetID := strings.TrimPrefix(line, "-")
			filtered := make([]ContextOpenQuestion, 0, len(g.OpenQuestions))
			for _, q := range g.OpenQuestions {
				if q.ID != targetID {
					filtered = append(filtered, q)
				}
			}
			g.OpenQuestions = filtered
		}
	}
	return nil
}
