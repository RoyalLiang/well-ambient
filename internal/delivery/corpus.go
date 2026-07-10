package delivery

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
)

// EnsureCorpusCandidates creates governed, idempotent candidates from a
// completed or rejected run. It never creates active context facts directly.
func EnsureCorpusCandidates(tx *gorm.DB, run db.ExecutionRun) ([]db.CorpusCandidate, error) {
	if tx == nil || run.ID == 0 {
		return nil, fmt.Errorf("database and execution run are required")
	}
	var spec db.DemandSpecVersion
	if err := tx.First(&spec, run.DemandSpecVersionID).Error; err != nil {
		return nil, err
	}
	scope := "repo"
	scopeID := strings.TrimSpace(run.Repo)
	if scopeID == "" {
		scope = "global"
	}

	provenance, _ := json.Marshal(map[string]interface{}{
		"execution_run_id":       run.ID,
		"demand_spec_version_id": spec.ID,
		"demand_id":              run.DemandID,
		"context_pack_id":        spec.ContextPackID,
		"pipeline_status":        run.PipelineStatus,
		"acceptance_state":       run.AcceptanceState,
		"mr_url":                 run.MRURL,
	})
	caseContent, _ := json.Marshal(map[string]interface{}{
		"goal":                spec.UserGoal,
		"summary":             spec.Summary,
		"acceptance_criteria": json.RawMessage(defaultJSONArray(spec.AcceptanceCriteriaJSON)),
		"test_plan":           json.RawMessage(defaultJSONArray(spec.TestPlanJSON)),
		"tasks":               json.RawMessage(defaultJSONArray(spec.TasksJSON)),
		"outcome":             run.Status,
		"pipeline":            run.PipelineStatus,
	})
	candidates := []db.CorpusCandidate{
		{
			ExecutionRunID: run.ID, DemandSpecVersionID: spec.ID, ContextPackID: spec.ContextPackID,
			CandidateType: "delivery_case", Scope: scope, ScopeID: scopeID,
			Title: fmt.Sprintf("%s 交付案例", run.DemandID), Summary: firstText(spec.Summary, spec.UserGoal, run.DemandID),
			Content: string(caseContent), ProvenanceJSON: string(provenance), Confidence: candidateConfidence(run),
			Sensitivity: "internal", Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			ExecutionRunID: run.ID, DemandSpecVersionID: spec.ID, ContextPackID: spec.ContextPackID,
			CandidateType: "requirement_pattern", Scope: scope, ScopeID: scopeID,
			Title: fmt.Sprintf("%s 验收与测试模式", run.DemandID), Summary: "冻结需求规格中的验收标准与测试口径",
			Content: string(mustJSON(map[string]json.RawMessage{
				"acceptance_criteria": json.RawMessage(defaultJSONArray(spec.AcceptanceCriteriaJSON)),
				"test_plan":           json.RawMessage(defaultJSONArray(spec.TestPlanJSON)),
			})), ProvenanceJSON: string(provenance), Confidence: candidateConfidence(run),
			Sensitivity: "internal", Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	if strings.TrimSpace(spec.RisksJSON) != "" && spec.RisksJSON != "[]" {
		candidates = append(candidates, db.CorpusCandidate{
			ExecutionRunID: run.ID, DemandSpecVersionID: spec.ID, ContextPackID: spec.ContextPackID,
			CandidateType: "risk_rule", Scope: scope, ScopeID: scopeID,
			Title: fmt.Sprintf("%s 风险复盘", run.DemandID), Summary: "需求风险与实际交付结果的复盘候选",
			Content:        string(mustJSON(map[string]interface{}{"risks": json.RawMessage(defaultJSONArray(spec.RisksJSON)), "outcome": run.Status, "block_reason": run.BlockReason})),
			ProvenanceJSON: string(provenance), Confidence: candidateConfidence(run), Sensitivity: "internal",
			Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		})
	}

	for index := range candidates {
		candidate := &candidates[index]
		var existing db.CorpusCandidate
		err := tx.Where("execution_run_id = ? AND candidate_type = ? AND scope = ? AND scope_id = ?", run.ID, candidate.CandidateType, candidate.Scope, candidate.ScopeID).First(&existing).Error
		if err == nil {
			*candidate = existing
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		if err := tx.Create(candidate).Error; err != nil {
			return nil, err
		}
	}
	return candidates, nil
}

func defaultJSONArray(raw string) []byte {
	raw = strings.TrimSpace(raw)
	if raw == "" || !json.Valid([]byte(raw)) {
		return []byte("[]")
	}
	return []byte(raw)
}

func mustJSON(value interface{}) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "交付复盘候选"
}

func candidateConfidence(run db.ExecutionRun) float64 {
	if run.Status == RunDelivered && run.PipelineStatus == "success" && run.AcceptanceState == "accepted" {
		return 0.95
	}
	if run.Status == RunRejected {
		return 0.72
	}
	return 0.65
}
