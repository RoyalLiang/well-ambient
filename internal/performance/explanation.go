package performance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
)

type Explanation struct {
	GeneratedAt     time.Time             `json:"generated_at"`
	ReadOnly        bool                  `json:"read_only"`
	ReadOnlyNotice  string                `json:"read_only_notice"`
	Runtime         RuntimeStatus         `json:"runtime"`
	Formula         FormulaExplanation    `json:"formula"`
	Metrics         []MetricDefinition    `json:"metrics"`
	FactorGroups    []FactorGroup         `json:"factor_groups"`
	LatestRun       *RunExplanation       `json:"latest_run"`
	RecentSnapshots []SnapshotExplanation `json:"recent_snapshots"`
	Evidence        EvidenceStats         `json:"evidence"`
}

type RuntimeStatus struct {
	SchedulerEnabled     bool       `json:"scheduler_enabled"`
	SchedulerStarted     bool       `json:"scheduler_started"`
	CalculationRunning   bool       `json:"calculation_running"`
	IntervalMinutes      float64    `json:"interval_minutes"`
	WindowDays           float64    `json:"assessment_window_days"`
	RetentionDays        float64    `json:"retention_days"`
	FormulaVersion       string     `json:"formula_version"`
	CoverageGate         float64    `json:"evidence_coverage_gate"`
	MinimumSamples       int        `json:"minimum_samples"`
	MinimumExposureDays  int        `json:"minimum_exposure_days"`
	BusyRetryAttempts    int        `json:"busy_retry_attempts"`
	BusyRetryDelayMS     int64      `json:"busy_retry_delay_ms"`
	PublicationMode      string     `json:"publication_mode"`
	DemandMetricsEnabled bool       `json:"demand_metrics_enabled"`
	BugMetricsEnabled    bool       `json:"bug_metrics_enabled"`
	CodeMetricsEnabled   bool       `json:"code_metrics_enabled"`
	JiraHistoryEnabled   bool       `json:"jira_history_enabled"`
	GitDedupeEnabled     bool       `json:"git_dedupe_enabled"`
	LastStartedAt        *time.Time `json:"last_started_at,omitempty"`
	LastCompletedAt      *time.Time `json:"last_completed_at,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
}

type FormulaExplanation struct {
	FinalScore         string       `json:"final_score"`
	DeliveryWeight     string       `json:"delivery_weight"`
	DefectLoss         string       `json:"defect_loss"`
	DefectScore        string       `json:"defect_score"`
	PublicationGate    string       `json:"publication_gate"`
	ResponsibilityRule string       `json:"responsibility_rule"`
	Levels             []ScoreLevel `json:"levels"`
}

type ScoreLevel struct {
	Level string `json:"level"`
	Range string `json:"range"`
}

type MetricDefinition struct {
	Code             string  `json:"code"`
	Dimension        string  `json:"dimension"`
	Name             string  `json:"name"`
	Weight           float64 `json:"weight"`
	Direction        string  `json:"direction"`
	MinimumSamples   int     `json:"minimum_samples"`
	Point2Boundary   float64 `json:"point_2_boundary"`
	Point3Boundary   float64 `json:"point_3_boundary"`
	Point4Boundary   float64 `json:"point_4_boundary"`
	Point5Boundary   float64 `json:"point_5_boundary"`
	Core             bool    `json:"core"`
	Source           string  `json:"source"`
	Rule             string  `json:"rule"`
	RequiredEvidence string  `json:"required_evidence"`
}

type FactorGroup struct {
	Name    string        `json:"name"`
	Purpose string        `json:"purpose"`
	Values  []FactorValue `json:"values"`
}

type FactorValue struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type RunExplanation struct {
	RunID                 string     `json:"run_id"`
	Trigger               string     `json:"trigger"`
	Status                string     `json:"status"`
	FormulaVersion        string     `json:"formula_version"`
	AssessmentWindowStart time.Time  `json:"assessment_window_start"`
	AssessmentWindowEnd   time.Time  `json:"assessment_window_end"`
	InputWatermark        time.Time  `json:"input_watermark"`
	SnapshotCount         int        `json:"snapshot_count"`
	RetentionCutoff       time.Time  `json:"retention_cutoff"`
	ErrorMessage          string     `json:"error_message,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

type SnapshotExplanation struct {
	ID                    uint            `json:"id"`
	RunID                 string          `json:"run_id"`
	SubjectKey            string          `json:"subject_key"`
	FormulaVersion        string          `json:"formula_version"`
	AssessmentWindowStart time.Time       `json:"assessment_window_start"`
	AssessmentWindowEnd   time.Time       `json:"assessment_window_end"`
	InputWatermark        time.Time       `json:"input_watermark"`
	InputDigest           string          `json:"input_digest"`
	DeliveryUnitCount     int             `json:"delivery_unit_count"`
	BugCount              int             `json:"bug_count"`
	DemandDelayCount      int             `json:"demand_delay_count"`
	BugReopenCount        int             `json:"bug_reopen_count"`
	CommitCount           int             `json:"commit_count"`
	DuplicateCommitCount  int             `json:"duplicate_commit_count"`
	EffectiveSampleCount  int             `json:"effective_sample_count"`
	ExposureDays          int             `json:"exposure_days"`
	EvidenceCoverage      float64         `json:"evidence_coverage"`
	ObservedScore         *float64        `json:"observed_score"`
	FinalScore            *float64        `json:"final_score"`
	DeliveryScore         *float64        `json:"delivery_score"`
	QualityScore          *float64        `json:"quality_score"`
	TrendAdjustment       float64         `json:"trend_adjustment"`
	RiskPenalty           float64         `json:"risk_penalty"`
	LeverageBonus         float64         `json:"leverage_bonus"`
	RatingStatus          string          `json:"rating_status"`
	Level                 string          `json:"level"`
	Metrics               json.RawMessage `json:"metrics"`
	Exclusions            json.RawMessage `json:"exclusions"`
	Adjustments           json.RawMessage `json:"adjustments"`
	CreatedAt             time.Time       `json:"created_at"`
}

type SnapshotDetail struct {
	GeneratedAt    time.Time           `json:"generated_at"`
	ReadOnly       bool                `json:"read_only"`
	ReadOnlyNotice string              `json:"read_only_notice"`
	Snapshot       SnapshotExplanation `json:"snapshot"`
	ItemFactors    json.RawMessage     `json:"item_factors"`
	Sources        []SnapshotSource    `json:"sources"`
}

type SnapshotSource struct {
	EvidenceKey         string    `json:"evidence_key"`
	Revision            uint      `json:"revision"`
	EventType           string    `json:"event_type"`
	EvidenceRef         string    `json:"evidence_ref"`
	SourceSystem        string    `json:"source_system"`
	SourceRecordID      string    `json:"source_record_id"`
	WorkItemID          string    `json:"work_item_id"`
	ProjectKey          string    `json:"project_key"`
	Severity            string    `json:"severity,omitempty"`
	EscapeStage         string    `json:"escape_stage,omitempty"`
	ReleaseMethod       string    `json:"release_method,omitempty"`
	RollbackImpact      string    `json:"rollback_impact,omitempty"`
	ReasonCode          string    `json:"reason_code,omitempty"`
	ResponsibilityShare float64   `json:"responsibility_share,omitempty"`
	Outcome             float64   `json:"outcome"`
	Weight              float64   `json:"weight"`
	OccurredAt          time.Time `json:"occurred_at"`
	ObservedAt          time.Time `json:"observed_at"`
	PayloadHash         string    `json:"payload_hash"`
}

type EvidenceStats struct {
	ActiveCount          int                 `json:"active_count"`
	RevisionCount        int64               `json:"revision_count"`
	SourceEventCount     int64               `json:"source_event_count"`
	ByType               []EvidenceTypeCount `json:"by_type"`
	LastObservedAt       *time.Time          `json:"last_observed_at,omitempty"`
	LastSourceObservedAt *time.Time          `json:"last_source_observed_at,omitempty"`
}

type EvidenceTypeCount struct {
	EventType string `json:"event_type"`
	Count     int    `json:"count"`
}

// Explain returns a read-only calculation contract and the latest persisted
// state. Reading it never starts or advances a calculation run.
func (m *Module) Explain(ctx context.Context, snapshotLimit int) (Explanation, error) {
	if m == nil || m.db == nil {
		return Explanation{}, errors.New("performance module database is not configured")
	}
	if snapshotLimit <= 0 || snapshotLimit > 100 {
		snapshotLimit = 30
	}
	now := m.now().UTC()
	settings := m.currentSettings()
	result := Explanation{
		GeneratedAt:     now,
		ReadOnly:        true,
		ReadOnlyNotice:  "本页面只读取已持久化的计算与证据记录，打开或刷新页面不会触发计算。",
		Runtime:         m.runtimeStatus(),
		Formula:         formulaExplanation(),
		Metrics:         metricDefinitions(),
		FactorGroups:    factorGroups(),
		RecentSnapshots: make([]SnapshotExplanation, 0),
		Evidence:        EvidenceStats{ByType: make([]EvidenceTypeCount, 0)},
	}
	err := m.withBusyRetry(ctx, func() error {
		return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			coreMembers, err := loadCoreMemberMatcher(tx, settings.CoreMembers)
			if err != nil {
				return err
			}

			var latest db.PerformanceScoreRun
			if err := tx.Order("created_at DESC, id DESC").First(&latest).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			} else {
				result.LatestRun = runExplanation(latest)
			}

			const snapshotBatchSize = 100
			seenSubjects := make(map[string]struct{})
			for offset := 0; len(result.RecentSnapshots) < snapshotLimit; {
				var snapshots []db.PerformanceScoreSnapshot
				if err := tx.Order("created_at DESC, id DESC").Offset(offset).Limit(snapshotBatchSize).Find(&snapshots).Error; err != nil {
					return err
				}
				for _, snapshot := range snapshots {
					canonical, eligible := coreMembers.canonical(snapshot.SubjectKey)
					if !eligible {
						continue
					}
					if _, seen := seenSubjects[canonical]; seen {
						continue
					}
					seenSubjects[canonical] = struct{}{}
					explanation := snapshotExplanation(snapshot)
					explanation.SubjectKey = canonical
					result.RecentSnapshots = append(result.RecentSnapshots, explanation)
					if len(result.RecentSnapshots) == snapshotLimit {
						break
					}
				}
				offset += len(snapshots)
				if len(snapshots) < snapshotBatchSize {
					break
				}
			}

			var revisionCount int64
			if err := tx.Model(&db.PerformanceEvidenceFact{}).Count(&revisionCount).Error; err != nil {
				return err
			}
			active, err := loadActiveEvidenceFacts(tx, now)
			if err != nil {
				return err
			}
			counts := make(map[string]int)
			var lastObservedAt *time.Time
			for _, fact := range active {
				counts[fact.EventType]++
				if lastObservedAt == nil || fact.ObservedAt.After(*lastObservedAt) {
					observed := fact.ObservedAt
					lastObservedAt = &observed
				}
			}
			for _, definition := range metricEvidenceTypeOrder() {
				result.Evidence.ByType = append(result.Evidence.ByType, EvidenceTypeCount{
					EventType: definition, Count: counts[definition],
				})
			}
			result.Evidence.ActiveCount = len(active)
			result.Evidence.RevisionCount = revisionCount
			result.Evidence.LastObservedAt = lastObservedAt
			if err := tx.Model(&db.PerformanceWorkItemEvent{}).Count(&result.Evidence.SourceEventCount).Error; err != nil {
				return err
			}
			var latestSource db.PerformanceWorkItemEvent
			if err := tx.Order("observed_at DESC, id DESC").First(&latestSource).Error; err == nil {
				observed := latestSource.ObservedAt
				result.Evidence.LastSourceObservedAt = &observed
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return nil
		})
	})
	return result, err
}

// ExplainSnapshot returns the persisted inputs and source references for one
// immutable score snapshot. It does not advance or recreate a calculation run.
func (m *Module) ExplainSnapshot(ctx context.Context, snapshotID uint) (SnapshotDetail, error) {
	if m == nil || m.db == nil {
		return SnapshotDetail{}, errors.New("performance module database is not configured")
	}
	if snapshotID == 0 {
		return SnapshotDetail{}, gorm.ErrRecordNotFound
	}
	result := SnapshotDetail{
		GeneratedAt:    m.now().UTC(),
		ReadOnly:       true,
		ReadOnlyNotice: "详情只读取该次已持久化快照及其输入水位内的来源，不会触发重新计算。",
		Sources:        make([]SnapshotSource, 0),
	}
	err := m.withBusyRetry(ctx, func() error {
		return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var snapshot db.PerformanceScoreSnapshot
			if err := tx.First(&snapshot, snapshotID).Error; err != nil {
				return err
			}
			coreMembers, err := loadCoreMemberMatcher(tx, m.currentSettings().CoreMembers)
			if err != nil {
				return err
			}
			canonical, eligible := coreMembers.canonical(snapshot.SubjectKey)
			if !eligible {
				return gorm.ErrRecordNotFound
			}
			result.Snapshot = snapshotExplanation(snapshot)
			result.Snapshot.SubjectKey = canonical
			result.ItemFactors = rawJSON(snapshot.ItemFactorsJSON)

			var metrics []metricResult
			if err := json.Unmarshal([]byte(snapshot.MetricsJSON), &metrics); err != nil {
				return err
			}
			includedRefs := make(map[string]struct{})
			for _, metric := range metrics {
				for _, ref := range metric.EvidenceRefs {
					includedRefs[ref] = struct{}{}
				}
			}
			var adjustments []adjustmentResult
			if strings.TrimSpace(snapshot.AdjustmentsJSON) != "" {
				if err := json.Unmarshal([]byte(snapshot.AdjustmentsJSON), &adjustments); err != nil {
					return err
				}
			}
			for _, adjustment := range adjustments {
				if adjustment.EvidenceRef != "" {
					includedRefs[adjustment.EvidenceRef] = struct{}{}
				}
				for _, ref := range adjustment.EvidenceRefs {
					includedRefs[ref] = struct{}{}
				}
			}
			facts, err := loadActiveEvidenceFacts(tx, snapshot.InputWatermark)
			if err != nil {
				return err
			}
			for _, fact := range facts {
				ref := formalEvidenceRef(fact)
				if _, included := includedRefs[ref]; !included {
					continue
				}
				result.Sources = append(result.Sources, snapshotSource(fact, ref))
			}
			return nil
		})
	})
	return result, err
}

func (m *Module) runtimeStatus() RuntimeStatus {
	settings := m.currentSettings()
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return RuntimeStatus{
		SchedulerEnabled: settings.Enabled, SchedulerStarted: m.started, CalculationRunning: m.running,
		IntervalMinutes: settings.Interval.Minutes(), WindowDays: settings.Window.Hours() / 24,
		RetentionDays: settings.Retention.Hours() / 24, FormulaVersion: settings.FormulaVersion,
		CoverageGate: settings.CoverageGate, MinimumSamples: settings.MinimumSamples,
		MinimumExposureDays: settings.MinimumExposureDays,
		BusyRetryAttempts:   settings.BusyRetries, BusyRetryDelayMS: settings.BusyRetryDelay.Milliseconds(),
		PublicationMode: settings.PublicationMode, DemandMetricsEnabled: settings.EnableDemandMetrics,
		BugMetricsEnabled: settings.EnableBugMetrics, CodeMetricsEnabled: settings.EnableCodeMetrics,
		JiraHistoryEnabled: settings.JiraHistoryEnabled, GitDedupeEnabled: settings.GitDedupeEnabled,
		LastStartedAt: cloneTime(m.lastStartedAt), LastCompletedAt: cloneTime(m.lastCompletedAt), LastError: m.lastError,
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func formulaExplanation() FormulaExplanation {
	return FormulaExplanation{
		FinalScore:         "总分 = 交付结果 35 分 + 交付可预测性 20 分 + 工程质量 45 分 - 代码风险扣分（最多 10 分）。每项先按判定边界映射为 1 到 5 档，再乘以固定权重；缺项不做二次归一，也不会把局部满档放大为 100 分",
		DeliveryWeight:     "规模点 × 需求等级系数 × 项目权重系数 × 责任份额，单需求权重封顶 10；复杂度、阶段和成员角色不再重复放大",
		DefectLoss:         "严重度系数 × 逃逸阶段系数 × 缺陷责任份额 × 闭环系数；闭环系数由延期与重开情况映射为 1 / 1.2 / 1.5",
		DefectScore:        "责任加权缺陷损失 ÷ 已完成且充分暴露的需求权重；Jira 缺少逃逸阶段或缺陷覆盖确认时最高按 4 档计入",
		PublicationGate:    "D01、D02、B01 三项核心计算均达到样本门槛，达标权重不低于 70%，去重需求不少于 5，质量暴露不少于 30 天，且配置为 formal 时才发布正式分",
		ResponsibilityRule: "Bug 修复贡献与缺陷责任分开；优先采用正式归责，其次按 Jira parent/唯一需求链接归到来源需求 Done 时负责人，修复人不会自动成为缺陷责任人。",
		Levels:             []ScoreLevel{{Level: "显著超出预期", Range: "85 到 100"}, {Level: "超出预期", Range: "75 到 84.99"}, {Level: "达到预期", Range: "60 到 74.99"}, {Level: "部分达到预期", Range: "45 到 59.99"}, {Level: "未达到预期", Range: "0 到 44.99"}},
	}
}

func metricDefinitions() []MetricDefinition {
	definitions := make([]MetricDefinition, 0, len(v6MetricRules))
	for _, rule := range v6MetricRules {
		definitions = append(definitions, MetricDefinition{
			Code: rule.Code, Dimension: rule.Dimension, Name: rule.Name, Weight: rule.Weight,
			Direction: string(rule.Direction), MinimumSamples: rule.MinimumSamples,
			Point2Boundary: rule.Point2Boundary, Point3Boundary: rule.Point3Boundary,
			Point4Boundary: rule.Point4Boundary, Point5Boundary: rule.Point5Boundary, Core: rule.Core,
			Source: rule.Source, Rule: rule.Formula, RequiredEvidence: rule.RequiredEvidence,
		})
	}
	return definitions
}

func factorGroups() []FactorGroup {
	return []FactorGroup{
		{Name: "需求规模点", Purpose: "由估算工期映射", Values: []FactorValue{{Label: "不超过 1 天", Value: 1}, {Label: "1 到 3 天", Value: 2}, {Label: "3 到 5 天", Value: 3}, {Label: "5 到 10 天", Value: 4}, {Label: "超过 10 天", Value: 5}}},
		{Name: "需求等级系数", Purpose: "体现单需求优先等级", Values: []FactorValue{{Label: "P0", Value: 1.40}, {Label: "P1", Value: 1.20}, {Label: "P2 / 未设置", Value: 1}, {Label: "P3", Value: 0.80}}},
		{Name: "项目权重系数", Purpose: "体现项目组合优先程度", Values: []FactorValue{{Label: "P0", Value: 1.30}, {Label: "P1", Value: 1.15}, {Label: "P2 / 未设置", Value: 1}, {Label: "P3", Value: 0.85}}},
		{Name: "Bug 严重度系数", Purpose: "用于来源需求质量损失", Values: []FactorValue{{Label: "S1", Value: 8}, {Label: "S2", Value: 5}, {Label: "S3", Value: 3}, {Label: "S4", Value: 1}}},
		{Name: "Bug 发现阶段系数", Purpose: "正式证据优先；Jira 缺失时使用测试阶段中性值并限制最高档", Values: []FactorValue{{Label: "生产", Value: 1.50}, {Label: "UAT / 预发", Value: 1.20}, {Label: "集成 / 测试", Value: 1}, {Label: "开发", Value: 0.60}}},
		{Name: "责任归因系数", Purpose: "不可归因或有争议的事实不得进入个人负向", Values: []FactorValue{{Label: "已确认", Value: 1}, {Label: "共同责任", Value: 0.50}, {Label: "不可归因 / 有争议", Value: 0}}},
		{Name: "Bug 闭环系数", Purpose: "用延期和重开反映缺陷闭环成本", Values: []FactorValue{{Label: "按期且未重开", Value: 1}, {Label: "延期或重开 1 次", Value: 1.20}, {Label: "严重延期或重开至少 2 次", Value: 1.50}}},
	}
}

func metricEvidenceTypeOrder() []string {
	return []string{evidenceDefectExposure, evidenceDefectAttribution}
}

func runExplanation(run db.PerformanceScoreRun) *RunExplanation {
	return &RunExplanation{
		RunID: run.RunID, Trigger: run.Trigger, Status: run.Status, FormulaVersion: run.FormulaVersion,
		AssessmentWindowStart: run.AssessmentWindowStart, AssessmentWindowEnd: run.AssessmentWindowEnd,
		InputWatermark: run.InputWatermark, SnapshotCount: run.SnapshotCount,
		RetentionCutoff: run.RetentionCutoff, ErrorMessage: run.ErrorMessage,
		CompletedAt: run.CompletedAt, CreatedAt: run.CreatedAt,
	}
}

func snapshotExplanation(snapshot db.PerformanceScoreSnapshot) SnapshotExplanation {
	deliveryScore, qualityScore := snapshotComponentScores(snapshot.FormulaVersion, snapshot.MetricsJSON)
	return SnapshotExplanation{
		ID: snapshot.ID, RunID: snapshot.RunID, SubjectKey: snapshot.SubjectKey,
		FormulaVersion:        snapshot.FormulaVersion,
		AssessmentWindowStart: snapshot.AssessmentWindowStart, AssessmentWindowEnd: snapshot.AssessmentWindowEnd,
		InputWatermark: snapshot.InputWatermark, InputDigest: snapshot.InputDigest,
		DeliveryUnitCount: snapshot.DeliveryUnitCount, BugCount: snapshot.BugCount,
		DemandDelayCount: snapshot.DemandDelayCount, BugReopenCount: snapshot.BugReopenCount,
		CommitCount: snapshot.CommitCount, DuplicateCommitCount: snapshot.DuplicateCommitCount,
		EffectiveSampleCount: snapshot.EffectiveSampleCount, EvidenceCoverage: snapshot.EvidenceCoverage,
		ExposureDays:  snapshot.ExposureDays,
		ObservedScore: snapshot.ObservedScore, FinalScore: snapshot.FinalScore,
		DeliveryScore: deliveryScore, QualityScore: qualityScore,
		TrendAdjustment: snapshot.TrendAdjustment, RiskPenalty: snapshot.RiskPenalty, LeverageBonus: snapshot.LeverageBonus,
		RatingStatus: snapshot.RatingStatus, Level: snapshot.Level,
		Metrics: rawJSON(snapshot.MetricsJSON), Exclusions: rawJSON(snapshot.ExclusionsJSON), Adjustments: rawJSON(snapshot.AdjustmentsJSON), CreatedAt: snapshot.CreatedAt,
	}
}

func snapshotComponentScores(formulaVersion, metricsJSON string) (*float64, *float64) {
	// Component totals are only comparable inside the current three-metric
	// contract. Historical snapshots remain readable, but must not be projected
	// into the v6 delivery/quality split.
	if formulaVersion != defaultFormulaVersion {
		return nil, nil
	}
	var metrics []metricResult
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		return nil, nil
	}
	delivery, quality := 0.0, 0.0
	deliveryAvailable, qualityAvailable := false, false
	for _, metric := range metrics {
		if !metric.Available || !metric.SampleQualified || metric.WeightedPoints == nil {
			continue
		}
		switch metric.Code {
		case metricDemandCompletion, metricDemandOnTime:
			delivery += *metric.WeightedPoints
			deliveryAvailable = true
		case metricDefectDensity:
			quality += *metric.WeightedPoints
			qualityAvailable = true
		}
	}
	var deliveryResult, qualityResult *float64
	if deliveryAvailable {
		deliveryResult = floatPointer(round2(delivery))
	}
	if qualityAvailable {
		qualityResult = floatPointer(round2(quality))
	}
	return deliveryResult, qualityResult
}

func snapshotSource(fact db.PerformanceEvidenceFact, ref string) SnapshotSource {
	return SnapshotSource{
		EvidenceKey: fact.EvidenceKey, Revision: fact.Revision, EventType: fact.EventType,
		EvidenceRef: ref, SourceSystem: fact.SourceSystem, SourceRecordID: fact.SourceRecordID,
		WorkItemID: fact.WorkItemID, ProjectKey: fact.ProjectKey,
		Severity: fact.Severity, EscapeStage: fact.EscapeStage,
		ReleaseMethod: fact.ReleaseMethod, RollbackImpact: fact.RollbackImpact, ReasonCode: fact.ReasonCode,
		ResponsibilityShare: fact.ResponsibilityShare, Outcome: fact.Outcome, Weight: fact.Weight,
		OccurredAt: fact.OccurredAt, ObservedAt: fact.ObservedAt, PayloadHash: fact.PayloadHash,
	}
}

func rawJSON(value string) json.RawMessage {
	if json.Valid([]byte(value)) {
		return json.RawMessage(value)
	}
	return json.RawMessage("[]")
}
