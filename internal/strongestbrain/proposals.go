package strongestbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/db"
)

// Proposal types
const (
	ProposalTypeResolverTune       = "resolver_tune"
	ProposalTypeBudgetRealloc      = "budget_realloc"
	ProposalTypeSkillResourceSlice = "skill_resource_slice"
	ProposalTypeToolReorder        = "tool_reorder"
	ProposalTypeModelSwitch        = "model_switch"
)

// Proposal statuses
const (
	ProposalStatusPendingReview = "pending_review"
	ProposalStatusApproved      = "approved"
	ProposalStatusRejected      = "rejected"
	ProposalStatusCanaryActive  = "canary_active"
	ProposalStatusApplied       = "applied"
)

// ProposalGenerator generates candidate optimization proposals based on intelligence signals.
type ProposalGenerator struct {
	db *gorm.DB
}

// NewProposalGenerator initializes a proposal generator.
func NewProposalGenerator(database *gorm.DB) *ProposalGenerator {
	return &ProposalGenerator{db: database}
}

// GenerateProposals analyzes dimension metrics and creates candidate proposals for human review.
func (pg *ProposalGenerator) GenerateProposals(ctx context.Context, report *CapabilityIntelligenceReport) ([]db.CapabilityProposal, error) {
	var created []db.CapabilityProposal
	summary := report.Summary

	// Rule 1: High Partial rate or Evidence Gaps -> Skill Resource Slicing & Context Expansion
	if summary.TotalRuns >= 3 && (summary.PartialRuns*5 >= summary.TotalRuns || summary.EvidenceGapRuns*4 >= summary.TotalRuns) {
		changes := map[string]any{
			"resource_slicing": map[string]any{
				"strategy":    "split_l1_and_l2",
				"l1_focus":    "output_contract_and_core_assertions",
				"l2_evidence": "diff_hunks_and_system_contracts",
			},
			"token_reserve_increase_pct": 20,
		}
		changesBytes, _ := json.Marshal(changes)
		baselineBytes, _ := json.Marshal(map[string]any{
			"partial_rate":      float64(summary.PartialRuns) / float64(summary.TotalRuns),
			"evidence_gap_rate": float64(summary.EvidenceGapRuns) / float64(summary.TotalRuns),
		})
		canaryScopeBytes, _ := json.Marshal(map[string]any{
			"initial_traffic_pct": 10,
			"target_projects":     []string{"*"},
			"duration_hours":      24,
		})
		rollbackBytes, _ := json.Marshal(map[string]any{
			"max_allowed_partial_increase_pct": 5,
			"max_allowed_evidence_gap_pct":     10,
		})

		proposal := db.CapabilityProposal{
			Title:                "细化核心审查技能（code_review）L1/L2资源切片与证据上下文扩展",
			ProposalType:         ProposalTypeSkillResourceSlice,
			Status:               ProposalStatusPendingReview,
			TargetCapabilityKey:  "code_review",
			ChangesJSON:          string(changesBytes),
			BaselineMetricsJSON:  string(baselineBytes),
			CanaryScopeJSON:      string(canaryScopeBytes),
			RollbackTriggersJSON: string(rollbackBytes),
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}
		if err := pg.saveProposalIfNew(ctx, &proposal); err == nil && proposal.ID > 0 {
			created = append(created, proposal)
		}
	}

	// Rule 2: Token Budget Optimization -> Compact ACP & Budget Allocation
	if summary.TotalRuns >= 2 && summary.AvgPromptTokens > 8000 {
		changes := map[string]any{
			"context_protocol": "acp/1",
			"budget_allocation": map[string]int{
				"instruction_tokens": 1500,
				"evidence_tokens":    6000,
				"raw_tokens":         0,
			},
			"lazy_load_l3": true,
		}
		changesBytes, _ := json.Marshal(changes)
		baselineBytes, _ := json.Marshal(map[string]any{
			"avg_prompt_tokens": summary.AvgPromptTokens,
		})
		canaryScopeBytes, _ := json.Marshal(map[string]any{
			"initial_traffic_pct": 5,
			"target_projects":     []string{"test-repos"},
		})
		rollbackBytes, _ := json.Marshal(map[string]any{
			"token_cost_overrun_pct": 10,
			"evidence_loss_allowed":  0,
		})

		proposal := db.CapabilityProposal{
			Title:                "启用全量 ACP/1 紧凑上下文呈现并重新划分 Token 预算（目标降幅 ≥ 30%）",
			ProposalType:         ProposalTypeBudgetRealloc,
			Status:               ProposalStatusPendingReview,
			TargetCapabilityKey:  "code_review",
			ChangesJSON:          string(changesBytes),
			BaselineMetricsJSON:  string(baselineBytes),
			CanaryScopeJSON:      string(canaryScopeBytes),
			RollbackTriggersJSON: string(rollbackBytes),
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}
		if err := pg.saveProposalIfNew(ctx, &proposal); err == nil && proposal.ID > 0 {
			created = append(created, proposal)
		}
	}

	// Rule 3: High Retry Rate -> Resolver Condition Fine-tuning
	if summary.TotalRuns >= 3 && summary.RetriedRuns*10 >= summary.TotalRuns {
		changes := map[string]any{
			"resolver_tuning": map[string]any{
				"condition_check": "require_non_empty_diff",
				"pre_warm_mcp":    true,
				"timeout_seconds": 180,
			},
		}
		changesBytes, _ := json.Marshal(changes)
		baselineBytes, _ := json.Marshal(map[string]any{
			"retry_rate": float64(summary.RetriedRuns) / float64(summary.TotalRuns),
		})
		canaryScopeBytes, _ := json.Marshal(map[string]any{
			"initial_traffic_pct": 10,
		})
		rollbackBytes, _ := json.Marshal(map[string]any{
			"retry_rate_threshold": 0.05,
		})

		proposal := db.CapabilityProposal{
			Title:                "优化能力解析器触发条件与预热探活，降低人工重试重评率",
			ProposalType:         ProposalTypeResolverTune,
			Status:               ProposalStatusPendingReview,
			TargetCapabilityKey:  "code_review",
			ChangesJSON:          string(changesBytes),
			BaselineMetricsJSON:  string(baselineBytes),
			CanaryScopeJSON:      string(canaryScopeBytes),
			RollbackTriggersJSON: string(rollbackBytes),
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}
		if err := pg.saveProposalIfNew(ctx, &proposal); err == nil && proposal.ID > 0 {
			created = append(created, proposal)
		}
	}

	return created, nil
}

func (pg *ProposalGenerator) saveProposalIfNew(ctx context.Context, p *db.CapabilityProposal) error {
	var count int64
	pg.db.WithContext(ctx).Model(&db.CapabilityProposal{}).
		Where("target_capability_key = ? AND proposal_type = ? AND status = ?",
			p.TargetCapabilityKey, p.ProposalType, ProposalStatusPendingReview).
		Count(&count)
	if count > 0 {
		return nil // Don't duplicate active pending proposals
	}
	return pg.db.WithContext(ctx).Create(p).Error
}

// ReviewProposal reviews a proposal (approve or reject).
func (pg *ProposalGenerator) ReviewProposal(ctx context.Context, proposalID uint, decision string, reviewer string, notes string) (*db.CapabilityProposal, error) {
	if decision != ProposalStatusApproved && decision != ProposalStatusRejected {
		return nil, fmt.Errorf("invalid decision %q: must be 'approved' or 'rejected'", decision)
	}
	if reviewer == "" {
		reviewer = "admin"
	}

	var proposal db.CapabilityProposal
	if err := pg.db.WithContext(ctx).First(&proposal, proposalID).Error; err != nil {
		return nil, fmt.Errorf("proposal %d not found: %w", proposalID, err)
	}

	if proposal.Status != ProposalStatusPendingReview {
		return nil, fmt.Errorf("proposal is in %s state, cannot be reviewed again", proposal.Status)
	}

	now := time.Now()
	proposal.Status = decision
	proposal.ReviewDecision = decision
	proposal.ReviewedBy = reviewer
	proposal.ReviewedAt = &now
	proposal.ReviewNotes = notes
	proposal.UpdatedAt = now

	if err := pg.db.WithContext(ctx).Save(&proposal).Error; err != nil {
		return nil, fmt.Errorf("failed to save proposal review: %w", err)
	}

	return &proposal, nil
}
