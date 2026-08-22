package solutioncatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
)

type ComparisonDocument struct {
	Entry             db.SolutionCatalogEntry `json:"entry"`
	DemandDescription string                  `json:"demand_description"`
	Markdown          string                  `json:"markdown"`
}

type ClaimedComparison struct {
	Comparison db.SolutionComparison `json:"comparison"`
	Left       ComparisonDocument    `json:"left"`
	Right      ComparisonDocument    `json:"right"`
}

type RoundOneResult struct {
	Equivalent   bool     `json:"equivalent"`
	Score        float64  `json:"score"`
	Reason       string   `json:"reason"`
	SharedIntent []string `json:"shared_intent"`
	Differences  []string `json:"differences"`
}

type ProjectVariation struct {
	ProjectKey string   `json:"project_key"`
	Items      []string `json:"items"`
}

type RoundTwoResult struct {
	Compatible        bool               `json:"compatible"`
	Standardizable    bool               `json:"standardizable"`
	Score             float64            `json:"score"`
	Summary           string             `json:"summary"`
	CommonCore        []string           `json:"common_core"`
	ProjectVariations []ProjectVariation `json:"project_variations"`
	Conflicts         []string           `json:"conflicts"`
	ProposalTitle     string             `json:"proposal_title"`
	ProposalMarkdown  string             `json:"proposal_markdown"`
}

func (module *Module) ClaimNextComparison(ctx context.Context) (*ClaimedComparison, error) {
	var claimed *ClaimedComparison
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := module.now().UTC()
		staleBefore := now.Add(-30 * time.Minute)
		if err := tx.Model(&db.SolutionComparison{}).
			Where("status = ? AND updated_at < ?", ComparisonRunning, staleBefore).
			Updates(map[string]any{"status": ComparisonQueued, "next_attempt_at": now, "updated_at": now, "last_error": "comparison worker lease expired"}).Error; err != nil {
			return err
		}
		var comparison db.SolutionComparison
		lookup := tx.Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", ComparisonQueued, now).
			Order("recall_score DESC, id ASC").Limit(1).Find(&comparison)
		if lookup.Error != nil || lookup.RowsAffected == 0 {
			return lookup.Error
		}
		update := tx.Model(&db.SolutionComparison{}).Where("id = ? AND status = ?", comparison.ID, ComparisonQueued).
			Updates(map[string]any{"status": ComparisonRunning, "attempt_count": comparison.AttemptCount + 1, "started_at": now, "next_attempt_at": nil, "updated_at": now})
		if update.Error != nil || update.RowsAffected == 0 {
			return update.Error
		}
		comparison.Status, comparison.AttemptCount, comparison.StartedAt, comparison.UpdatedAt = ComparisonRunning, comparison.AttemptCount+1, &now, now
		left, err := module.loadComparisonDocument(tx, comparison.LeftEntryID, comparison.LeftRevisionID)
		if err != nil {
			return err
		}
		right, err := module.loadComparisonDocument(tx, comparison.RightEntryID, comparison.RightRevisionID)
		if err != nil {
			return err
		}
		claimed = &ClaimedComparison{Comparison: comparison, Left: left, Right: right}
		return nil
	})
	return claimed, err
}

func (module *Module) loadComparisonDocument(tx *gorm.DB, entryID, revisionID uint) (ComparisonDocument, error) {
	var entry db.SolutionCatalogEntry
	if err := tx.First(&entry, entryID).Error; err != nil {
		return ComparisonDocument{}, err
	}
	if entry.PublishedRevisionID != revisionID {
		return ComparisonDocument{}, fmt.Errorf("%w: catalog revision changed", ErrConflict)
	}
	var revision db.SolutionRevision
	if err := tx.First(&revision, revisionID).Error; err != nil {
		return ComparisonDocument{}, err
	}
	markdown, err := decodeSolutionRevision(revision)
	if err != nil {
		return ComparisonDocument{}, err
	}
	var demand db.TaskTelemetry
	description := ""
	lookup := tx.Where("task_id = ?", entry.DemandID).Limit(1).Find(&demand)
	if lookup.Error != nil {
		return ComparisonDocument{}, lookup.Error
	}
	if lookup.RowsAffected > 0 {
		description = strings.TrimSpace(demand.Description)
	}
	return ComparisonDocument{Entry: entry, DemandDescription: description, Markdown: markdown}, nil
}

func (module *Module) CompleteRoundOne(ctx context.Context, id uint, promptVersion string, result RoundOneResult) error {
	result.Score = normalizedScore(result.Score)
	encoded, err := marshalAndEncode(result)
	if err != nil {
		return err
	}
	return module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var comparison db.SolutionComparison
		if err := tx.First(&comparison, id).Error; err != nil {
			return err
		}
		if comparison.Status != ComparisonRunning || comparison.Stage != ComparisonStageRoundOne {
			return ErrConflict
		}
		now := module.now().UTC()
		verdict := "not_equivalent"
		status := ComparisonDismissed
		stage := ComparisonStageRoundOne
		var completedAt *time.Time
		if result.Equivalent {
			verdict = "equivalent"
			status = ComparisonQueued
			stage = ComparisonStageRoundTwo
		} else {
			completedAt = &now
		}
		return tx.Model(&comparison).Updates(map[string]any{
			"status": status, "stage": stage, "round_one_verdict": verdict, "round_one_score": result.Score,
			"round_one_prompt_version": strings.TrimSpace(promptVersion),
			"round_one_payload":        encoded.stored, "round_one_encoding": encoded.encoding, "round_one_hash": encoded.hash,
			"round_one_bytes": encoded.rawBytes, "round_one_stored_bytes": len(encoded.stored),
			"last_error": "", "next_attempt_at": nil, "completed_at": completedAt, "updated_at": now,
		}).Error
	})
}

func (module *Module) CompleteRoundTwo(ctx context.Context, id uint, promptVersion string, result RoundTwoResult) error {
	result.Score = normalizedScore(result.Score)
	encoded, err := marshalAndEncode(result)
	if err != nil {
		return err
	}
	return module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var comparison db.SolutionComparison
		if err := tx.First(&comparison, id).Error; err != nil {
			return err
		}
		if comparison.Status != ComparisonRunning || comparison.Stage != ComparisonStageRoundTwo || comparison.RoundOneVerdict != "equivalent" {
			return ErrConflict
		}
		now := module.now().UTC()
		verdict := "incompatible"
		status := ComparisonIncompatible
		if result.Compatible {
			verdict = "compatible"
			status = ComparisonCompleted
		}
		if result.Standardizable && strings.TrimSpace(result.ProposalMarkdown) != "" {
			proposalContent, err := encodePayload([]byte(strings.TrimSpace(result.ProposalMarkdown)))
			if err != nil {
				return err
			}
			proposal := db.SolutionStandardizationProposal{
				ComparisonID: comparison.ID, Status: ProposalPending,
				Title: strings.TrimSpace(result.ProposalTitle), Summary: strings.TrimSpace(result.Summary),
				Content: proposalContent.stored, ContentEncoding: proposalContent.encoding, ContentHash: proposalContent.hash,
				ContentBytes: proposalContent.rawBytes, StoredBytes: len(proposalContent.stored),
				CreatedBy: "solution-catalog-agent", CreatedAt: now, UpdatedAt: now,
			}
			if proposal.Title == "" {
				proposal.Title = "待复核标准方案"
			}
			if err := tx.Create(&proposal).Error; err != nil {
				return err
			}
			status = ComparisonNeedsReview
		}
		return tx.Model(&comparison).Updates(map[string]any{
			"status": status, "round_two_verdict": verdict, "round_two_score": result.Score,
			"round_two_prompt_version": strings.TrimSpace(promptVersion),
			"round_two_payload":        encoded.stored, "round_two_encoding": encoded.encoding, "round_two_hash": encoded.hash,
			"round_two_bytes": encoded.rawBytes, "round_two_stored_bytes": len(encoded.stored),
			"last_error": "", "next_attempt_at": nil, "completed_at": now, "updated_at": now,
		}).Error
	})
}

func (module *Module) FailComparison(ctx context.Context, id uint, cause error) error {
	return module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var comparison db.SolutionComparison
		if err := tx.First(&comparison, id).Error; err != nil {
			return err
		}
		if comparison.Status != ComparisonRunning {
			return nil
		}
		now := module.now().UTC()
		status := ComparisonFailed
		var next *time.Time
		if comparison.AttemptCount < 3 {
			status = ComparisonQueued
			retry := now.Add(time.Duration(comparison.AttemptCount*comparison.AttemptCount) * time.Minute)
			next = &retry
		}
		message := "comparison failed"
		if cause != nil {
			message = cause.Error()
		}
		return tx.Model(&comparison).Updates(map[string]any{
			"status": status, "last_error": message, "next_attempt_at": next, "updated_at": now,
		}).Error
	})
}

func normalizedScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

type ReviewProposalCommand struct {
	ProposalID   uint
	Action       string
	ReviewNote   string
	Actor        string
	StandardID   uint
	ProjectScope []string
}

func (module *Module) ReviewProposal(ctx context.Context, command ReviewProposalCommand) (StandardizationProposalView, *StandardDetail, error) {
	command.Action = strings.ToLower(strings.TrimSpace(command.Action))
	command.Actor = strings.TrimSpace(command.Actor)
	if command.ProposalID == 0 || (command.Action != "accept" && command.Action != "reject") || command.Actor == "" {
		return StandardizationProposalView{}, nil, ErrInvalid
	}
	var result StandardizationProposalView
	var standardDetail *StandardDetail
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var proposal db.SolutionStandardizationProposal
		if err := tx.First(&proposal, command.ProposalID).Error; err != nil {
			return ErrNotFound
		}
		if proposal.Status != ProposalPending {
			return ErrConflict
		}
		allowed, err := module.proposalAllowed(tx, proposal, command.ProjectScope)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrNotFound
		}
		raw, err := decodePayload(proposal.Content, proposal.ContentEncoding, proposal.ContentHash)
		if err != nil {
			return err
		}
		now := module.now().UTC()
		if command.Action == "reject" {
			if err := tx.Model(&proposal).Updates(map[string]any{
				"status": ProposalRejected, "reviewed_by": command.Actor, "review_note": strings.TrimSpace(command.ReviewNote),
				"reviewed_at": now, "updated_at": now,
			}).Error; err != nil {
				return err
			}
			proposal.Status, proposal.ReviewedBy, proposal.ReviewNote, proposal.ReviewedAt, proposal.UpdatedAt = ProposalRejected, command.Actor, strings.TrimSpace(command.ReviewNote), &now, now
			result = StandardizationProposalView{Proposal: proposal, Markdown: string(raw)}
			return nil
		}

		standard := db.SolutionStandard{}
		if command.StandardID != 0 {
			if err := tx.Where("id = ? AND status = ?", command.StandardID, StandardActive).First(&standard).Error; err != nil {
				return ErrNotFound
			}
		} else {
			standard = db.SolutionStandard{
				Status: StandardActive, Title: proposal.Title, Summary: proposal.Summary,
				CreatedBy: command.Actor, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&standard).Error; err != nil {
				return err
			}
		}
		revision := db.SolutionStandardRevision{
			StandardID: standard.ID, Version: standard.Sequence + 1, ParentRevisionID: standard.CurrentRevisionID,
			ProposalID: proposal.ID, Content: append([]byte(nil), proposal.Content...), ContentEncoding: proposal.ContentEncoding,
			ContentHash: proposal.ContentHash, ContentBytes: proposal.ContentBytes, StoredBytes: proposal.StoredBytes,
			CreatedBy: command.Actor, CreatedAt: now,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		if err := tx.Model(&standard).Updates(map[string]any{
			"sequence": revision.Version, "current_revision_id": revision.ID,
			"title": proposal.Title, "summary": proposal.Summary, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&proposal).Updates(map[string]any{
			"status": ProposalAccepted, "standard_id": standard.ID, "reviewed_by": command.Actor,
			"review_note": strings.TrimSpace(command.ReviewNote), "reviewed_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		standard.Sequence, standard.CurrentRevisionID, standard.Title, standard.Summary, standard.UpdatedAt = revision.Version, revision.ID, proposal.Title, proposal.Summary, now
		proposal.Status, proposal.StandardID, proposal.ReviewedBy, proposal.ReviewNote, proposal.ReviewedAt, proposal.UpdatedAt = ProposalAccepted, standard.ID, command.Actor, strings.TrimSpace(command.ReviewNote), &now, now
		projects, _, err := module.standardProjectsWithTx(tx, proposal, command.ProjectScope)
		if err != nil {
			return err
		}
		result = StandardizationProposalView{Proposal: proposal, Markdown: string(raw)}
		standardDetail = &StandardDetail{Standard: standard, Revision: revision, Markdown: string(raw), Projects: projects}
		return nil
	})
	return result, standardDetail, err
}

func (module *Module) proposalAllowed(tx *gorm.DB, proposal db.SolutionStandardizationProposal, scope []string) (bool, error) {
	_, allowed, err := module.standardProjectsWithTx(tx, proposal, scope)
	return allowed, err
}

func (module *Module) standardProjectsWithTx(tx *gorm.DB, proposal db.SolutionStandardizationProposal, scope []string) ([]string, bool, error) {
	var comparison db.SolutionComparison
	if err := tx.First(&comparison, proposal.ComparisonID).Error; err != nil {
		return nil, false, err
	}
	var entries []db.SolutionCatalogEntry
	if err := tx.Where("id IN ?", []uint{comparison.LeftEntryID, comparison.RightEntryID}).Find(&entries).Error; err != nil {
		return nil, false, err
	}
	if len(entries) != 2 {
		return nil, false, ErrIntegrity
	}
	projects := db.NormalizeProjectKeys([]string{entries[0].ProjectKey, entries[1].ProjectKey})
	for _, project := range projects {
		if !allowedProject(project, scope) {
			return nil, false, nil
		}
	}
	return projects, true, nil
}

func comparisonJSON(raw string, target any) error {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, "```") {
		lines := strings.Split(clean, "\n")
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
			clean = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	if start, end := strings.Index(clean, "{"), strings.LastIndex(clean, "}"); start >= 0 && end > start {
		clean = clean[start : end+1]
	}
	if err := json.Unmarshal([]byte(clean), target); err != nil {
		return fmt.Errorf("decode comparison JSON: %w", err)
	}
	return nil
}

func ParseRoundOne(raw string) (RoundOneResult, error) {
	var result RoundOneResult
	if err := comparisonJSON(raw, &result); err != nil {
		return RoundOneResult{}, err
	}
	result.Score = normalizedScore(result.Score)
	return result, nil
}

func ParseRoundTwo(raw string) (RoundTwoResult, error) {
	var result RoundTwoResult
	if err := comparisonJSON(raw, &result); err != nil {
		return RoundTwoResult{}, err
	}
	result.Score = normalizedScore(result.Score)
	return result, nil
}
