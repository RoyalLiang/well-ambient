package strongestbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/agentruntime"
	"well-ambient/internal/codereview"
	"well-ambient/internal/db"
)

// DimensionSummary aggregates the 7 core dimensions defined in the architecture plan.
type DimensionSummary struct {
	TotalRuns         int     `json:"total_runs"`
	CompletedRuns     int     `json:"completed_runs"`
	PartialRuns       int     `json:"partial_runs"`
	FailedRuns        int     `json:"failed_runs"`
	RetriedRuns       int     `json:"retried_runs"`
	ActiveRuns        int     `json:"active_runs"`
	EvidenceGapRuns   int     `json:"evidence_gap_runs"`
	HighRiskFindings  int     `json:"high_risk_findings"`
	QuestionsCount    int     `json:"questions_count"`
	AvgPromptTokens   float64 `json:"avg_prompt_tokens"`
	AvgDurationMs     float64 `json:"avg_duration_ms"`
	TotalTokensSaved  int     `json:"total_tokens_saved"`
	SecurityRejects   int     `json:"security_rejects"`
	ToolFailureCount  int     `json:"tool_failure_count"`
}

// CapabilityVersionMetric aggregates performance per capability version.
type CapabilityVersionMetric struct {
	CapabilityKey       string  `json:"capability_key"`
	Version             int     `json:"version"`
	Digest              string  `json:"digest"`
	ScopeType           string  `json:"scope_type"`
	ScopeID             string  `json:"scope_id"`
	Status              string  `json:"status"`
	ValidationStatus    string  `json:"validation_status"`
	ActivatedAt         string  `json:"activated_at"`
	TotalRuns           int     `json:"total_runs"`
	CompletedRuns       int     `json:"completed_runs"`
	PartialRuns         int     `json:"partial_runs"`
	FailedRuns          int     `json:"failed_runs"`
	RetriedRuns         int     `json:"retried_runs"`
	EvidenceGapRuns     int     `json:"evidence_gap_runs"`
	HighRiskFindings    int     `json:"high_risk_findings"`
	QuestionsCount      int     `json:"questions_count"`
	AvgTokens           float64 `json:"avg_tokens"`
	AvgDurationMs       float64 `json:"avg_duration_ms"`
	QualityScore        float64 `json:"quality_score"`
	EvidenceScore       float64 `json:"evidence_score"`
	StabilityScore      float64 `json:"stability_score"`
}

// RunExceptionDetail highlights notable partial, failed, or high-risk runs.
type RunExceptionDetail struct {
	RunID            uint   `json:"run_id"`
	RunKey           string `json:"run_key"`
	AgentKind        string `json:"agent_kind"`
	State            string `json:"state"`
	CapabilityKey    string `json:"capability_key"`
	Version          int    `json:"version"`
	HighRiskFindings int    `json:"high_risk_findings"`
	QuestionsCount   int    `json:"questions_count"`
	Error            string `json:"error,omitempty"`
	UpdatedAt        string `json:"updated_at"`
}

// CapabilityIntelligenceReport is the complete 7-dimension governance analysis.
type CapabilityIntelligenceReport struct {
	GeneratedAt      string                    `json:"generated_at"`
	Summary          DimensionSummary          `json:"summary"`
	ActiveSkills     []CapabilityVersionMetric `json:"active_skills"`
	Versions         []CapabilityVersionMetric `json:"versions"`
	Proposals        []db.CapabilityProposal   `json:"proposals"`
	RecentExceptions []RunExceptionDetail      `json:"recent_exceptions"`
	Recommendations  []string                  `json:"recommendations"`
}

// IntelligenceEngine extracts 7-dimension features from AgentRuns, Bindings, and Events.
type IntelligenceEngine struct {
	db       *gorm.DB
	registry *agentruntime.Registry
}

// NewIntelligenceEngine initializes an intelligence engine.
func NewIntelligenceEngine(database *gorm.DB, registry *agentruntime.Registry) *IntelligenceEngine {
	return &IntelligenceEngine{
		db:       database,
		registry: registry,
	}
}

// ComputeReport calculates the comprehensive 7-dimension capability intelligence report.
func (ie *IntelligenceEngine) ComputeReport(ctx context.Context, limit int) (*CapabilityIntelligenceReport, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	now := time.Now()

	// 1. Fetch active capability versions
	var activeVersions []db.CapabilityVersion
	_ = ie.db.WithContext(ctx).
		Where("status = ?", "active").
		Order("scope_type ASC, scope_id ASC, version DESC").
		Find(&activeVersions).Error

	capIDToKey := make(map[uint]string)
	var caps []db.Capability
	if err := ie.db.WithContext(ctx).Find(&caps).Error; err == nil {
		for _, c := range caps {
			capIDToKey[c.ID] = c.CapabilityKey
		}
	}

	report := &CapabilityIntelligenceReport{
		GeneratedAt:      now.Format(time.RFC3339),
		Summary:          DimensionSummary{},
		ActiveSkills:     []CapabilityVersionMetric{},
		Versions:         []CapabilityVersionMetric{},
		Proposals:        []db.CapabilityProposal{},
		RecentExceptions: []RunExceptionDetail{},
		Recommendations:  []string{},
	}

	for _, v := range activeVersions {
		actTime := ""
		if v.ActivatedAt != nil {
			actTime = v.ActivatedAt.Format(time.RFC3339)
		}
		capKey := capIDToKey[v.CapabilityID]
		if capKey == "" {
			capKey = fmt.Sprintf("cap-%d", v.CapabilityID)
		}
		report.ActiveSkills = append(report.ActiveSkills, CapabilityVersionMetric{
			CapabilityKey:    capKey,
			Version:          v.Version,
			Digest:           v.ContentDigest,
			ScopeType:        v.ScopeType,
			ScopeID:          v.ScopeID,
			Status:           v.Status,
			ValidationStatus: v.ValidationStatus,
			ActivatedAt:      actTime,
		})
	}

	// 2. Fetch AgentRuns (new architecture)
	var agentRuns []db.AgentRun
	_ = ie.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&agentRuns).Error

	// Also fetch legacy CodeReviewRuns for complete backward compatibility
	var legacyReviewRuns []db.CodeReviewRun
	_ = ie.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&legacyReviewRuns).Error

	metricsMap := make(map[string]*CapabilityVersionMetric)
	getMetric := func(key string, ver int, digest string) *CapabilityVersionMetric {
		metricKey := fmt.Sprintf("%s:%d", key, ver)
		if m, exists := metricsMap[metricKey]; exists {
			return m
		}
		m := &CapabilityVersionMetric{
			CapabilityKey: key,
			Version:       ver,
			Digest:        digest,
		}
		metricsMap[metricKey] = m
		return m
	}

	totalPromptTokens := 0
	totalDuration := int64(0)

	// Process AgentRuns
	for _, run := range agentRuns {
		report.Summary.TotalRuns++
		switch run.State {
		case "completed":
			report.Summary.CompletedRuns++
		case "partial":
			report.Summary.PartialRuns++
		case "failed":
			report.Summary.FailedRuns++
		case "queued", "resolving", "loading", "running", "verifying":
			report.Summary.ActiveRuns++
		}
		totalPromptTokens += run.PromptTokens
		totalDuration += run.DurationMs

		// Query capability bindings for this run
		var bindings []db.RunCapabilityBinding
		_ = ie.db.WithContext(ctx).Where("run_id = ?", run.ID).Find(&bindings).Error

		for _, b := range bindings {
			m := getMetric(b.CapabilityKey, b.Version, b.Digest)
			m.TotalRuns++
			switch run.State {
			case "completed":
				m.CompletedRuns++
			case "partial":
				m.PartialRuns++
			case "failed":
				m.FailedRuns++
			}
		}

		if run.State == "partial" || run.State == "failed" {
			if len(report.RecentExceptions) < 20 {
				capKey := "unknown"
				capVer := 1
				if len(bindings) > 0 {
					capKey = bindings[0].CapabilityKey
					capVer = bindings[0].Version
				}
				report.RecentExceptions = append(report.RecentExceptions, RunExceptionDetail{
					RunID:         run.ID,
					RunKey:        run.RunKey,
					AgentKind:     run.AgentKind,
					State:         run.State,
					CapabilityKey: capKey,
					Version:       capVer,
					Error:         run.Error,
					UpdatedAt:     run.UpdatedAt.Format(time.RFC3339),
				})
			}
		}
	}

	// Process Legacy CodeReviewRuns
	for _, run := range legacyReviewRuns {
		report.Summary.TotalRuns++
		switch run.Status {
		case "completed":
			report.Summary.CompletedRuns++
		case "partial":
			report.Summary.PartialRuns++
		case "failed":
			report.Summary.FailedRuns++
		case "queued", "running":
			report.Summary.ActiveRuns++
		}
		if run.RetryOfID > 0 {
			report.Summary.RetriedRuns++
		}

		capName := run.SkillName
		if capName == "" {
			capName = "code_review"
		}
		m := getMetric(capName, run.SkillVersion, run.SkillHash)
		m.TotalRuns++
		switch run.Status {
		case "completed":
			m.CompletedRuns++
		case "partial":
			m.PartialRuns++
		case "failed":
			m.FailedRuns++
		}
		if run.RetryOfID > 0 {
			m.RetriedRuns++
		}

		var reportData codereview.Report
		if run.ReportJSON != "" && json.Unmarshal([]byte(run.ReportJSON), &reportData) == nil {
			high := 0
			for _, f := range reportData.Findings {
				if f.Severity == "high" {
					high++
				}
			}
			qCount := len(reportData.Questions)
			m.HighRiskFindings += high
			m.QuestionsCount += qCount
			report.Summary.HighRiskFindings += high
			report.Summary.QuestionsCount += qCount

			if !reportData.EvidenceComplete || qCount > 0 {
				m.EvidenceGapRuns++
				report.Summary.EvidenceGapRuns++
			}

			if (run.Status == "partial" || run.Status == "failed" || high > 0) && len(report.RecentExceptions) < 20 {
				report.RecentExceptions = append(report.RecentExceptions, RunExceptionDetail{
					RunID:            run.ID,
					RunKey:           fmt.Sprintf("review-%d", run.ID),
					AgentKind:        "code_review",
					State:            run.Status,
					CapabilityKey:    capName,
					Version:          run.SkillVersion,
					HighRiskFindings: high,
					QuestionsCount:   qCount,
					Error:            run.Error,
					UpdatedAt:        run.UpdatedAt.Format(time.RFC3339),
				})
			}
		} else if run.Status == "partial" || run.Status == "failed" {
			m.EvidenceGapRuns++
			report.Summary.EvidenceGapRuns++
		}
	}

	// 3. Count Security Rejects and Tool Failures from Events
	var secRejects int64
	_ = ie.db.WithContext(ctx).Model(&db.AgentRunEvent{}).
		Where("event_type = ?", agentruntime.EventValidationFailed).
		Count(&secRejects).Error
	report.Summary.SecurityRejects = int(secRejects)

	if report.Summary.TotalRuns > 0 {
		report.Summary.AvgPromptTokens = float64(totalPromptTokens) / float64(report.Summary.TotalRuns)
		report.Summary.AvgDurationMs = float64(totalDuration) / float64(report.Summary.TotalRuns)
	}

	// 4. Calculate Scores for Version Metrics
	for _, m := range metricsMap {
		if m.TotalRuns > 0 {
			m.QualityScore = float64(m.CompletedRuns) / float64(m.TotalRuns) * 100
			m.StabilityScore = float64(m.TotalRuns-m.FailedRuns-m.RetriedRuns) / float64(m.TotalRuns) * 100
			if m.TotalRuns > m.EvidenceGapRuns {
				m.EvidenceScore = float64(m.TotalRuns-m.EvidenceGapRuns) / float64(m.TotalRuns) * 100
			} else {
				m.EvidenceScore = 0
			}
		}
		report.Versions = append(report.Versions, *m)
	}
	sort.Slice(report.Versions, func(i, j int) bool {
		if report.Versions[i].TotalRuns != report.Versions[j].TotalRuns {
			return report.Versions[i].TotalRuns > report.Versions[j].TotalRuns
		}
		return report.Versions[i].Version > report.Versions[j].Version
	})

	// 5. Query proposals
	var proposals []db.CapabilityProposal
	_ = ie.db.WithContext(ctx).Order("created_at DESC").Limit(10).Find(&proposals).Error
	report.Proposals = proposals

	// 6. Generate Governance Recommendations
	report.Recommendations = ie.buildRecommendations(report.Summary, len(report.ActiveSkills))

	return report, nil
}

func (ie *IntelligenceEngine) buildRecommendations(summary DimensionSummary, activeSkills int) []string {
	var recs []string
	if activeSkills == 0 {
		recs = append(recs, "严重警告：未检测到激活的核心技能版本，请优先在 Registry 中激活全局核心能力。")
	}
	if summary.TotalRuns == 0 {
		return append(recs, "暂无执行运行样本；建议先执行基线试运行或触发回放以积累治理数据。")
	}
	if summary.PartialRuns*5 >= summary.TotalRuns {
		recs = append(recs, "部分完成（Partial）比例偏高（≥20%）：建议由最强大脑生成提案，细化 Skill 资源切片并扩充 Context Provider 检索深度。")
	}
	if summary.FailedRuns*10 >= summary.TotalRuns {
		recs = append(recs, "运行失败率偏高（≥10%）：请检查模型端点连通性、输出结构校验及 MCP 插件健康度。")
	}
	if summary.EvidenceGapRuns*4 >= summary.TotalRuns {
		recs = append(recs, "证据完整度缺口显著（≥25%）：建议强化代码审查中的 Diff Hunk 关联与知识图谱引用，避免规则猜测。")
	}
	if summary.RetriedRuns*10 >= summary.TotalRuns {
		recs = append(recs, "人工重试重评率偏高（≥10%）：建议触发针对候选版本的隔离 Replay 评测，验证后再做版本晋级。")
	}
	if summary.SecurityRejects > 0 {
		recs = append(recs, fmt.Sprintf("安全门禁已拦截 %d 次未授权或格式违规操作：内核边界防护生效，继续保持严格非可信数据隔离。", summary.SecurityRejects))
	}
	if len(recs) == 0 {
		recs = append(recs, "当前运行时执行平稳，7维指标均处于安全水线之内；保持版本冻结，可继续通过 Replay 闭环探索成本与 Token 压缩优化。")
	}
	return recs
}
