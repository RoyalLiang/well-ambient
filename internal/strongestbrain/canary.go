package strongestbrain

// Canary stages
const (
	CanaryStageShadow = "shadow"
	CanaryStage1Pct   = "1%"
	CanaryStage10Pct  = "10%"
	CanaryStage50Pct  = "50%"
	CanaryStageFull   = "100%"
)

// RollbackThresholds defines the sensitive auto-rollback guards.
type RollbackThresholds struct {
	MaxAllowedFailureRatePct       float64
	MaxAllowedEvidenceGapRatePct   float64
	MaxAllowedP0Regressions        int
	MaxAllowedTokenOverrunPct      float64
	MaxAllowedSecurityViolations   int
}

// DefaultRollbackThresholds returns the production safety baseline.
func DefaultRollbackThresholds() RollbackThresholds {
	return RollbackThresholds{
		MaxAllowedFailureRatePct:     10.0,
		MaxAllowedEvidenceGapRatePct: 20.0,
		MaxAllowedP0Regressions:      0,
		MaxAllowedTokenOverrunPct:    15.0,
		MaxAllowedSecurityViolations: 0,
	}
}

// CanaryEvaluationResult gives the recommendation whether to advance stage or trigger auto-rollback.
type CanaryEvaluationResult struct {
	CurrentStage           string   `json:"current_stage"`
	RecommendedNextStage   string   `json:"recommended_next_stage"`
	CanAdvance             bool     `json:"can_advance"`
	AutoRollbackTriggered  bool     `json:"auto_rollback_triggered"`
	Violations             []string `json:"violations"`
}

// EvaluateCanarySafety evaluates live canary execution metrics against rollback guards.
func EvaluateCanarySafety(stage string, summary DimensionSummary, thresholds RollbackThresholds) CanaryEvaluationResult {
	res := CanaryEvaluationResult{
		CurrentStage: stage,
		Violations:   []string{},
	}

	if summary.TotalRuns == 0 {
		res.CanAdvance = false
		res.RecommendedNextStage = stage
		return res
	}

	failRate := float64(summary.FailedRuns) / float64(summary.TotalRuns) * 100
	if failRate > thresholds.MaxAllowedFailureRatePct {
		res.AutoRollbackTriggered = true
		res.Violations = append(res.Violations, "失败率超标，触发自动回滚保护")
	}

	evidenceGapRate := float64(summary.EvidenceGapRuns) / float64(summary.TotalRuns) * 100
	if evidenceGapRate > thresholds.MaxAllowedEvidenceGapRatePct {
		res.AutoRollbackTriggered = true
		res.Violations = append(res.Violations, "证据缺失率超标，触发自动回滚保护")
	}

	if summary.SecurityRejects > thresholds.MaxAllowedSecurityViolations {
		res.AutoRollbackTriggered = true
		res.Violations = append(res.Violations, "发生非可信数据安全违规事件，立即触发熔断回滚")
	}

	if res.AutoRollbackTriggered {
		res.CanAdvance = false
		res.RecommendedNextStage = "rollback_to_baseline"
		return res
	}

	// Advance stage logic
	res.CanAdvance = true
	switch stage {
	case CanaryStageShadow:
		res.RecommendedNextStage = CanaryStage1Pct
	case CanaryStage1Pct:
		res.RecommendedNextStage = CanaryStage10Pct
	case CanaryStage10Pct:
		res.RecommendedNextStage = CanaryStage50Pct
	case CanaryStage50Pct:
		res.RecommendedNextStage = CanaryStageFull
	default:
		res.RecommendedNextStage = CanaryStageFull
	}

	return res
}
