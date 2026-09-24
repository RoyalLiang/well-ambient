package strongestbrain

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
)

// Service provides the unified Strongest Brain evolution and governance engine.
type Service struct {
	db           *gorm.DB
	registry     *agentruntime.Registry
	intelligence *IntelligenceEngine
	proposals    *ProposalGenerator
	replay       *ReplayEngine
}

// NewService instantiates the unified Strongest Brain service.
func NewService(database *gorm.DB, registry *agentruntime.Registry) *Service {
	return &Service{
		db:           database,
		registry:     registry,
		intelligence: NewIntelligenceEngine(database, registry),
		proposals:    NewProposalGenerator(database),
		replay:       NewReplayEngine(database, registry),
	}
}

// GetIntelligence computes the 7-dimension governance report and triggers candidate proposals.
func (s *Service) GetIntelligence(ctx context.Context, limit int) (*CapabilityIntelligenceReport, error) {
	report, err := s.intelligence.ComputeReport(ctx, limit)
	if err != nil {
		return nil, err
	}

	// Proactively generate optimization proposals if threshold anomalies are detected
	_, _ = s.proposals.GenerateProposals(ctx, report)

	// Refresh proposals list in report
	var proposals []db.CapabilityProposal
	_ = s.db.WithContext(ctx).Order("created_at DESC").Limit(20).Find(&proposals).Error
	report.Proposals = proposals

	return report, nil
}

// ListProposals lists recent optimization proposals.
func (s *Service) ListProposals(ctx context.Context, status string) ([]db.CapabilityProposal, error) {
	query := s.db.WithContext(ctx).Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var list []db.CapabilityProposal
	if err := query.Find(&list).Error; err != nil {
		return nil, fmt.Errorf("failed to query proposals: %w", err)
	}
	return list, nil
}

// ReviewProposal performs human-in-the-loop signoff on an optimization proposal.
func (s *Service) ReviewProposal(ctx context.Context, proposalID uint, decision string, reviewer string, notes string) (*db.CapabilityProposal, error) {
	return s.proposals.ReviewProposal(ctx, proposalID, decision, reviewer, notes)
}

// RunReplay executes an offline evaluation between candidate and baseline.
func (s *Service) RunReplay(ctx context.Context, cmd ReplayCommand) (*db.CapabilityEvaluation, error) {
	return s.replay.Evaluate(ctx, cmd)
}

// GetReplay retrieves a past evaluation by ID.
func (s *Service) GetReplay(ctx context.Context, evaluationID uint) (*db.CapabilityEvaluation, error) {
	var eval db.CapabilityEvaluation
	if err := s.db.WithContext(ctx).First(&eval, evaluationID).Error; err != nil {
		return nil, fmt.Errorf("evaluation %d not found: %w", evaluationID, err)
	}
	return &eval, nil
}
