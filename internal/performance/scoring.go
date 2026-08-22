package performance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"

	"gorm.io/gorm"
)

type subjectEvidence struct {
	SubjectKey string
	Units      []deliveryUnitEvidence
	Bugs       []bugEvidence
	Defects    []attributedDefectEvidence
	Facts      []db.PerformanceEvidenceFact
	Commits    []db.GitCommitLog
}

type deliveryUnitEvidence struct {
	Task     db.TaskTelemetry
	Factor   itemFactor
	Segments []ownershipSegment
}

type ownershipSegment struct {
	SubjectKey string
	Start      time.Time
	End        time.Time
	EndReason  string
	Ref        string
}

type bugEvidence struct {
	Task            db.TaskTelemetry
	Factor          itemFactor
	Segments        []ownershipSegment
	AllSegments     []ownershipSegment
	StatusEvents    []db.PerformanceWorkItemEvent
	RelevantSubject string
}

type attributedDefectEvidence struct {
	Task         db.TaskTelemetry
	OriginTaskID string
	StatusEvents []db.PerformanceWorkItemEvent
}

type subjectCalculation struct {
	SubjectKey           string
	DeliveryUnitCount    int
	BugCount             int
	DemandDelayCount     int
	BugReopenCount       int
	CommitCount          int
	DuplicateCommitCount int
	EffectiveSampleCount int
	ExposureDays         int
	EvidenceCoverage     float64
	ObservedScore        *float64
	FinalScore           *float64
	TrendAdjustment      float64
	RiskPenalty          float64
	LeverageBonus        float64
	RatingStatus         string
	Level                string
	MetricsJSON          string
	ItemFactorsJSON      string
	ExclusionsJSON       string
	AdjustmentsJSON      string
	InputDigest          string
}

type projectFactors struct {
	Priority string
}

func loadSubjectEvidence(tx *gorm.DB, windowStart, watermark time.Time, coreMembers []string) ([]subjectEvidence, error) {
	memberMatcher, err := loadCoreMemberMatcher(tx, coreMembers)
	if err != nil {
		return nil, err
	}
	var tasks []db.TaskTelemetry
	err = tx.Where(`
		(last_update >= ? AND last_update <= ?) OR
		(source_updated_at >= ? AND source_updated_at <= ?) OR
		(task_created_at >= ? AND task_created_at <= ?) OR
		(completed_at >= ? AND completed_at <= ?) OR
		(due_date >= ? AND due_date <= ?)`,
		windowStart, watermark, windowStart, watermark,
		windowStart, watermark, windowStart, watermark,
		windowStart, watermark).
		Order("task_id ASC").Find(&tasks).Error
	if err != nil {
		return nil, err
	}

	var projects []db.ProjectConfig
	if err := tx.Find(&projects).Error; err != nil {
		return nil, err
	}
	projectByKey := make(map[string]projectFactors, len(projects))
	for _, project := range projects {
		projectByKey[deliveryplanning.NormalizeProjectKey(project.ProjectKey)] = projectFactors{
			Priority: project.BasePriority,
		}
	}

	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.TaskID)
	}
	eventsByTask := make(map[string][]db.PerformanceWorkItemEvent)
	if len(taskIDs) > 0 {
		var events []db.PerformanceWorkItemEvent
		if err := tx.Where("work_item_id IN ? AND occurred_at <= ?", taskIDs, watermark).
			Order("work_item_id ASC, occurred_at ASC, id ASC").Find(&events).Error; err != nil {
			return nil, err
		}
		for _, event := range events {
			eventsByTask[event.WorkItemID] = append(eventsByTask[event.WorkItemID], event)
		}
	}

	bySubject := make(map[string]*subjectEvidence)
	ensureSubject := func(subjectKey string) *subjectEvidence {
		subject := bySubject[subjectKey]
		if subject == nil {
			subject = &subjectEvidence{SubjectKey: subjectKey}
			bySubject[subjectKey] = subject
		}
		return subject
	}

	originOwnerByTask := make(map[string]string)
	for _, task := range tasks {
		kind, kindErr := deliveryplanning.NormalizeIssueType(task.IssueType)
		if kindErr != nil || kind != deliveryplanning.WorkItemRequirement {
			continue
		}
		events := eventsByTask[task.TaskID]
		segments := canonicalOwnershipSegments(buildOwnershipSegments(task, events, windowStart, watermark), memberMatcher)
		subjectKey, included := memberMatcher.canonical(task.Assignee)
		if completedWorkItem(task) {
			if owner, ok := ownerAt(segments, *task.CompletedAt); ok {
				subjectKey, included = owner, true
			}
		}
		if !included {
			continue
		}
		originOwnerByTask[strings.TrimSpace(task.TaskID)] = subjectKey
		project := projectByKey[deliveryplanning.NormalizeProjectKey(task.ProjectKey)]
		ensureSubject(subjectKey).Units = append(ensureSubject(subjectKey).Units, deliveryUnitEvidence{
			Task: task, Factor: deliveryFactor(task, project), Segments: segments,
		})
	}

	for _, task := range tasks {
		kind, kindErr := deliveryplanning.NormalizeIssueType(task.IssueType)
		if kindErr != nil || kind != deliveryplanning.WorkItemBug {
			continue
		}
		events := eventsByTask[task.TaskID]

		allSegments := canonicalOwnershipSegments(buildOwnershipSegments(task, events, windowStart, watermark), memberMatcher)
		statusEvents := filterSourceEvents(events, SourceEventStatusChange)
		segmentsBySubject := make(map[string][]ownershipSegment)
		for _, segment := range allSegments {
			segmentsBySubject[segment.SubjectKey] = append(segmentsBySubject[segment.SubjectKey], segment)
		}
		for subjectKey, segments := range segmentsBySubject {
			ensureSubject(subjectKey).Bugs = append(ensureSubject(subjectKey).Bugs, bugEvidence{
				Task: task, Factor: bugFactor(task), Segments: segments,
				AllSegments: allSegments, StatusEvents: statusEvents, RelevantSubject: subjectKey,
			})
		}
		if originOwner := originOwnerByTask[strings.TrimSpace(task.ParentWorkItemID)]; originOwner != "" {
			ensureSubject(originOwner).Defects = append(ensureSubject(originOwner).Defects, attributedDefectEvidence{
				Task: task, OriginTaskID: strings.TrimSpace(task.ParentWorkItemID), StatusEvents: statusEvents,
			})
		}
	}

	formalFacts, err := loadActiveEvidenceFacts(tx, watermark)
	if err != nil {
		return nil, err
	}
	for _, fact := range formalFacts {
		if fact.OccurredAt.Before(windowStart) || fact.OccurredAt.After(watermark) {
			continue
		}
		subjectKey, included := memberMatcher.canonical(fact.SubjectKey)
		if !included {
			continue
		}
		ensureSubject(subjectKey).Facts = append(ensureSubject(subjectKey).Facts, fact)
	}

	var commits []db.GitCommitLog
	if err := tx.Where("action = ? AND created_at >= ? AND created_at <= ? AND TRIM(commit_id) <> ''", "git_push", windowStart, watermark).
		Order("created_at ASC, id ASC").Find(&commits).Error; err != nil {
		return nil, err
	}
	for _, commit := range commits {
		subjectKey, included := memberMatcher.canonical(commit.Author)
		if !included {
			continue
		}
		ensureSubject(subjectKey).Commits = append(ensureSubject(subjectKey).Commits, commit)
	}

	keys := make([]string, 0, len(bySubject))
	for key := range bySubject {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]subjectEvidence, 0, len(keys))
	for _, key := range keys {
		result = append(result, *bySubject[key])
	}
	return result, nil
}

func deliveryFactor(task db.TaskTelemetry, project projectFactors) itemFactor {
	size := sizePoints(task.EstimateDays)
	demandLevel := demandLevelFactor(task.Priority)
	projectWeight := projectWeightFactor(project.Priority)
	responsibility := 1.0
	weight := round2(clamp(size*demandLevel*projectWeight*responsibility, 0, 10))
	warnings := []string{"responsibility_share_defaulted_to_one"}
	if strings.TrimSpace(task.Priority) == "" {
		warnings = append(warnings, "demand_level_missing_defaulted_to_p2")
	}
	return itemFactor{
		WorkItemID: task.TaskID, Kind: deliveryplanning.WorkItemRequirement,
		ProjectKey: deliveryplanning.NormalizeProjectKey(task.ProjectKey),
		SizePoints: size, PriorityFactor: demandLevel, DemandLevelFactor: demandLevel,
		ProjectFactor: projectWeight, ResponsibilityShare: responsibility,
		DeliveryWeight: weight, ContributionWeight: weight, Warnings: warnings,
	}
}

func bugFactor(task db.TaskTelemetry) itemFactor {
	factor := itemFactor{
		WorkItemID: task.TaskID, Kind: deliveryplanning.WorkItemBug,
		ProjectKey: deliveryplanning.NormalizeProjectKey(task.ProjectKey), FixContributionShare: 1,
		Warnings: []string{"fix_contribution_is_not_defect_responsibility"},
	}
	if severity, ok := severityFactor(firstNonEmpty(task.Severity, task.Priority)); ok {
		factor.SeverityFactor = &severity
	} else {
		factor.Warnings = append(factor.Warnings, "bug_severity_missing")
	}
	return factor
}

func sizePoints(estimateDays float64) float64 {
	switch {
	case estimateDays <= 0:
		return 2
	case estimateDays <= 1:
		return 1
	case estimateDays <= 3:
		return 2
	case estimateDays <= 5:
		return 3
	case estimateDays <= 10:
		return 4
	default:
		return 5
	}
}

func demandLevelFactor(priority string) float64 {
	switch strings.ToUpper(strings.TrimSpace(priority)) {
	case "P0", "BLOCKER", "CRITICAL":
		return 1.40
	case "P1", "HIGH", "MAJOR", "高":
		return 1.20
	case "P3", "LOW", "TRIVIAL", "低":
		return 0.80
	default:
		return 1
	}
}

func projectWeightFactor(priority string) float64 {
	switch strings.ToUpper(strings.TrimSpace(priority)) {
	case "P0", "CRITICAL":
		return 1.30
	case "P1", "HIGH", "高":
		return 1.15
	case "P3", "LOW", "低":
		return 0.85
	default:
		return 1
	}
}

// priorityFactor remains a compatibility alias for historical callers and
// means project weight, not Jira demand level, in persisted pre-v6 callers.
func priorityFactor(priority string) float64 { return projectWeightFactor(priority) }

func bugLoss(severity, escapeStage string, defectResponsibilityShare float64) (float64, bool) {
	severityValue, severityOK := severityFactor(severity)
	escapeValue, escapeOK := escapeFactor(escapeStage)
	responsibility, responsibilityOK := responsibilityFactor(defectResponsibilityShare)
	if !severityOK || !escapeOK || !responsibilityOK {
		return 0, false
	}
	return round2(severityValue * escapeValue * responsibility), true
}

func severityFactor(severity string) (float64, bool) {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "S1", "P0", "BLOCKER", "CRITICAL":
		return 8, true
	case "S2", "P1", "MAJOR", "HIGH":
		return 5, true
	case "S3", "P2", "MINOR", "MEDIUM":
		return 3, true
	case "S4", "P3", "TRIVIAL", "LOW":
		return 1, true
	default:
		return 0, false
	}
}

func escapeFactor(stage string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "development", "dev", "开发":
		return 0.60, true
	case "test", "qa", "测试", "integration", "集成", "集成/测试":
		return 1, true
	case "staging", "uat", "预发布", "预发", "uat/预发":
		return 1.20, true
	case "production", "prod", "生产":
		return 1.50, true
	default:
		return 0, false
	}
}

func calculateSubject(evidence subjectEvidence, settings Settings, windowStart, watermark time.Time) (subjectCalculation, error) {
	metrics := make([]metricResult, 0, len(v6MetricRules))
	sampleRefs := make(map[string]struct{})
	completion := make([]weightedEvidence, 0)
	onTime := make([]weightedEvidence, 0)
	flow := make([]weightedEvidence, 0)
	qualityExposureWeight := 0.0
	qualitySampleRefs := make([]string, 0)
	qualityOrigins := make(map[string]struct{})
	completedDemandCount := 0
	delayCount := 0
	qualityCutoff := watermark.Add(-time.Duration(settings.MinimumExposureDays) * 24 * time.Hour)

	for _, unit := range evidence.Units {
		weight := unit.Factor.ContributionWeight
		completed := completedWorkItem(unit.Task)
		completedInWindow := completed && !unit.Task.CompletedAt.Before(windowStart) && !unit.Task.CompletedAt.After(watermark)
		dueEligible := unit.Task.DueDate != nil && !unit.Task.DueDate.Before(windowStart) && !unit.Task.DueDate.After(watermark)
		if !completedInWindow && !dueEligible {
			continue
		}
		ref := workItemEvidenceRef(unit.Task, "completion")
		completion = append(completion, weightedEvidence{ref: ref, weight: weight, value: boolRatio(completed)})
		sampleRefs[ref] = struct{}{}
		if completedInWindow {
			completedDemandCount++
			if !unit.Task.CompletedAt.After(qualityCutoff) {
				qualityExposureWeight += weight
				qualityOrigins[strings.TrimSpace(unit.Task.TaskID)] = struct{}{}
				qualitySampleRefs = append(qualitySampleRefs, workItemEvidenceRef(unit.Task, "quality_exposure"))
			}
		}
		if dueEligible {
			passed := completed && !unit.Task.CompletedAt.After(*unit.Task.DueDate)
			onTime = append(onTime, weightedEvidence{ref: workItemEvidenceRef(unit.Task, "due"), weight: weight, value: boolRatio(passed)})
			if !passed {
				delayCount++
			}
		}
		if completedInWindow {
			plannedHours := unit.Task.EstimateHours
			if plannedHours <= 0 && unit.Task.EstimateDays > 0 {
				plannedHours = unit.Task.EstimateDays * 8
			}
			actualHours := responsibilityHours(unit.Segments, evidence.SubjectKey, windowStart, *unit.Task.CompletedAt)
			if plannedHours > 0 && actualHours > 0 {
				flow = append(flow, weightedEvidence{ref: workItemEvidenceRef(unit.Task, "flow"), weight: weight, value: clamp(plannedHours/actualHours, 0, 1)})
			}
		}
	}

	reopenCount := 0
	seenReopens := make(map[string]struct{})
	countReopen := func(event db.PerformanceWorkItemEvent) {
		if !jiraStatusDone(event.FromValue) || jiraStatusDone(event.ToValue) {
			return
		}
		ref := sourceEventRef(event)
		if _, exists := seenReopens[ref]; exists {
			return
		}
		seenReopens[ref] = struct{}{}
		reopenCount++
	}
	for _, bug := range evidence.Bugs {
		for _, event := range bug.StatusEvents {
			countReopen(event)
		}
	}
	for _, defect := range evidence.Defects {
		for _, event := range defect.StatusEvents {
			countReopen(event)
		}
	}

	defectRule := mustMetricRule(metricDefectDensity)
	defectRefs := make([]string, 0)
	defectLossTotal := 0.0
	formalFactors := make([]itemFactor, 0)
	formalDefectWorkItems := make(map[string]struct{})
	qualityEvidenceConfirmed := false
	qualityConfirmationRefs := make([]string, 0)
	for _, fact := range evidence.Facts {
		if fact.EventType == evidenceDefectExposure && fact.Outcome > 0 {
			qualityEvidenceConfirmed = true
			qualityConfirmationRefs = append(qualityConfirmationRefs, formalEvidenceRef(fact))
		}
		if fact.EventType != evidenceDefectAttribution {
			continue
		}
		loss, ok := bugLoss(fact.Severity, fact.EscapeStage, fact.ResponsibilityShare)
		if !ok {
			continue
		}
		weightedLoss := round2(loss * fact.Weight)
		severity, _ := severityFactor(fact.Severity)
		escape, _ := escapeFactor(fact.EscapeStage)
		responsibility := fact.ResponsibilityShare
		defectRefs = append(defectRefs, formalEvidenceRef(fact))
		formalDefectWorkItems[strings.TrimSpace(fact.WorkItemID)] = struct{}{}
		defectLossTotal += weightedLoss
		formalFactors = append(formalFactors, itemFactor{
			WorkItemID: fact.WorkItemID, Kind: evidenceDefectAttribution,
			ProjectKey: fact.ProjectKey, ContributionWeight: fact.Weight,
			SeverityFactor: &severity, EscapeFactor: &escape,
			DefectResponsibilityShare: &responsibility, BugLoss: &weightedLoss,
		})
	}
	jiraFactors := make([]itemFactor, 0)
	jiraQualityDerived := false
	for _, defect := range evidence.Defects {
		if _, exposed := qualityOrigins[defect.OriginTaskID]; !exposed {
			continue
		}
		if _, overridden := formalDefectWorkItems[strings.TrimSpace(defect.Task.TaskID)]; overridden {
			continue
		}
		severity, ok := severityFactor(firstNonEmpty(defect.Task.Severity, defect.Task.Priority))
		if !ok {
			continue
		}
		escape := 1.0
		responsibility := 1.0
		closure := defectClosureMultiplier(defect.Task, defect.StatusEvents, watermark)
		loss := round2(severity * escape * responsibility * closure)
		ref := workItemEvidenceRef(defect.Task, "origin_defect")
		defectRefs = append(defectRefs, ref)
		defectLossTotal += loss
		jiraQualityDerived = true
		jiraFactors = append(jiraFactors, itemFactor{
			WorkItemID: defect.Task.TaskID, OriginWorkItemID: defect.OriginTaskID,
			Kind: evidenceDefectAttribution, ProjectKey: defect.Task.ProjectKey,
			SeverityFactor: &severity, EscapeFactor: &escape,
			DefectResponsibilityShare: &responsibility, ClosureMultiplier: &closure, BugLoss: &loss,
			Warnings: []string{"jira_origin_requirement_attribution", "escape_stage_missing_defaulted_to_test", "quality_point_capped_at_four"},
		})
	}
	defectMetric := metricUnavailableWithEvidence(defectRule, qualitySampleRefs, "没有达到质量判定所需的已完成需求暴露量")
	if qualityExposureWeight >= 10 && len(qualitySampleRefs) > 0 {
		defectMetric = metricFromRatio(defectRule, defectLossTotal/qualityExposureWeight, qualitySampleRefs)
		defectMetric.EvidenceRefs = append(defectMetric.EvidenceRefs, defectRefs...)
		defectMetric.EvidenceRefs = append(defectMetric.EvidenceRefs, qualityConfirmationRefs...)
		defectMetric.Numerator = floatPointer(round2(defectLossTotal))
		defectMetric.Denominator = floatPointer(round2(qualityExposureWeight))
		defectMetric.RawUnit = "加权缺陷损失/充分暴露的完成需求权重"
		if jiraQualityDerived || !qualityEvidenceConfirmed {
			defectMetric = capMetricPoint(defectMetric, 4, "Jira 缺少完整逃逸阶段或缺陷覆盖确认，工程质量最高按 4 档计入")
		}
	}

	dedupedCommits := dedupeCommits(evidence.Commits)
	validFingerprintRefs := make([]string, 0)
	duplicateCount := 0
	seenFingerprints := make(map[string]string)
	for _, commit := range dedupedCommits {
		fingerprint := strings.TrimSpace(commit.ContentFingerprint)
		if fingerprint == "" {
			continue
		}
		ref := gitEvidenceRef(commit)
		validFingerprintRefs = append(validFingerprintRefs, ref)
		if strings.TrimSpace(commit.DuplicateOfCommit) != "" {
			duplicateCount++
			continue
		}
		key := strings.ToLower(strings.TrimSpace(commit.Repo)) + ":" + fingerprint
		if _, exists := seenFingerprints[key]; exists {
			duplicateCount++
		} else {
			seenFingerprints[key] = commit.CommitID
		}
	}

	demandCompletion := metricFromEvidence(mustMetricRule(metricDemandCompletion), completion)
	demandCompletion = withEvidenceFraction(demandCompletion, completion, "完成需求权重/应完成需求权重")
	predictability := predictabilityMetric(onTime, flow)

	if !settings.EnableDemandMetrics {
		demandCompletion = metricUnavailableForRule(mustMetricRule(metricDemandCompletion), "需求交付指标已由配置关闭")
		predictability = metricUnavailableForRule(mustMetricRule(metricDemandOnTime), "交付可预测性指标已由配置关闭")
	}
	if !settings.EnableBugMetrics {
		defectMetric = metricUnavailableForRule(defectRule, "Bug 质量指标已由配置关闭")
	}

	metrics = append(metrics, demandCompletion, predictability, defectMetric)
	adjustments := codeRiskAdjustments(settings.EnableCodeMetrics, validFingerprintRefs, duplicateCount, len(dedupedCommits), completedDemandCount)
	riskPenalty := 0.0
	for _, adjustment := range adjustments {
		if adjustment.Value < 0 {
			riskPenalty += -adjustment.Value
		}
	}
	riskPenalty = clamp(riskPenalty, 0, 10)
	qualifiedWeight := 0.0
	qualifiedWeightedScore := 0.0
	exclusions := make([]string, 0)
	for _, metric := range metrics {
		if !metric.Available || metric.Score == nil || !metric.SampleQualified {
			exclusions = append(exclusions, metric.Code+": "+metric.Reason)
			continue
		}
		qualifiedWeight += metric.Weight
		qualifiedWeightedScore += *metric.WeightedPoints
	}
	coverage := round2(qualifiedWeight)
	var observed *float64
	if qualifiedWeight > 0 {
		observed = floatPointer(round2(clamp(qualifiedWeightedScore-riskPenalty, 0, 100)))
	}
	ratingStatus := "insufficient_evidence"
	var finalScore *float64
	level := ""
	exposureDays := calculateExposureDays(evidence, windowStart, watermark)
	coreMetricsQualified := true
	for _, metric := range metrics {
		rule := mustMetricRule(metric.Code)
		if rule.Core && (!metric.Available || !metric.SampleQualified) {
			coreMetricsQualified = false
			exclusions = append(exclusions, metric.Code+": 核心指标未达到正式样本门槛")
		}
	}
	if qualifiedWeight < settings.CoverageGate {
		exclusions = append(exclusions, "GATE: 可用指标权重低于判定表覆盖率门槛")
	}
	if len(sampleRefs) < settings.MinimumSamples {
		exclusions = append(exclusions, "GATE: 去重后的有效样本低于判定表全局门槛")
	}
	if exposureDays < settings.MinimumExposureDays {
		exclusions = append(exclusions, "GATE: 可验证暴露天数低于判定表门槛")
	}
	formalEligible := observed != nil && qualifiedWeight >= settings.CoverageGate && len(sampleRefs) >= settings.MinimumSamples && exposureDays >= settings.MinimumExposureDays && coreMetricsQualified
	if settings.PublicationMode == publicationModeShadow {
		ratingStatus = "shadow"
		exclusions = append(exclusions, "GATE: 当前配置为影子模式，只生成参考分和审计快照")
	} else if formalEligible {
		value := round2(clamp(*observed, 0, 100))
		finalScore = &value
		level = levelFor(value)
		ratingStatus = "formal"
	}

	factors := make([]itemFactor, 0, len(evidence.Units)+len(evidence.Bugs)+len(formalFactors)+len(jiraFactors))
	for _, unit := range evidence.Units {
		factors = append(factors, unit.Factor)
	}
	for _, bug := range evidence.Bugs {
		factor := bug.Factor
		for _, segment := range bug.Segments {
			factor.OwnershipSegmentHours += round2(segment.End.Sub(segment.Start).Hours())
		}
		factors = append(factors, factor)
	}
	factors = append(factors, formalFactors...)
	factors = append(factors, jiraFactors...)
	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		return subjectCalculation{}, err
	}
	factorsJSON, err := json.Marshal(factors)
	if err != nil {
		return subjectCalculation{}, err
	}
	exclusionsJSON, err := json.Marshal(exclusions)
	if err != nil {
		return subjectCalculation{}, err
	}
	adjustmentsJSONBytes, err := json.Marshal(adjustments)
	if err != nil {
		return subjectCalculation{}, err
	}
	adjustmentsJSON := string(adjustmentsJSONBytes)
	digestInput, err := json.Marshal(struct {
		SubjectKey string
		Metrics    []metricResult
		Factors    []itemFactor
		Facts      []db.PerformanceEvidenceFact
		Commits    []db.GitCommitLog
	}{evidence.SubjectKey, metrics, factors, evidence.Facts, dedupedCommits})
	if err != nil {
		return subjectCalculation{}, err
	}
	digest := sha256.Sum256(digestInput)
	return subjectCalculation{
		SubjectKey: evidence.SubjectKey, DeliveryUnitCount: len(evidence.Units), BugCount: distinctSubjectBugCount(evidence),
		DemandDelayCount: delayCount, BugReopenCount: reopenCount,
		CommitCount: len(dedupedCommits), DuplicateCommitCount: duplicateCount,
		EffectiveSampleCount: len(sampleRefs), ExposureDays: exposureDays, EvidenceCoverage: coverage,
		ObservedScore: observed, FinalScore: finalScore, RiskPenalty: round2(riskPenalty), RatingStatus: ratingStatus, Level: level,
		MetricsJSON: string(metricsJSON), ItemFactorsJSON: string(factorsJSON),
		ExclusionsJSON: string(exclusionsJSON), AdjustmentsJSON: adjustmentsJSON,
		InputDigest: hex.EncodeToString(digest[:]),
	}, nil
}

func predictabilityMetric(onTime, flow []weightedEvidence) metricResult {
	rule := mustMetricRule(metricDemandOnTime)
	if len(onTime) == 0 {
		return metricUnavailableForRule(rule, "没有带到期日的需求，无法判定交付可预测性")
	}
	onTimeRatio, _ := weightedEvidenceRatio(onTime)
	refs := evidenceRefs(onTime)
	ratio := onTimeRatio
	degraded := len(flow) == 0
	if !degraded {
		flowRatio, _ := weightedEvidenceRatio(flow)
		ratio = 0.7*onTimeRatio + 0.3*flowRatio
		refs = uniqueStrings(append(refs, evidenceRefs(flow)...))
	}
	metric := metricFromRatio(rule, ratio, refs)
	metric.Numerator = floatPointer(round4(ratio))
	metric.Denominator = floatPointer(1)
	metric.RawUnit = "70%按期率+30%计划周期兑现率"
	if degraded {
		metric = capMetricPoint(metric, 4, "缺少原始估算或负责人责任周期，仅按到期兑现率计算，最高按 4 档计入")
	}
	return metric
}

func weightedEvidenceRatio(evidence []weightedEvidence) (float64, float64) {
	numerator, denominator := 0.0, 0.0
	for _, item := range evidence {
		if item.weight <= 0 {
			continue
		}
		denominator += item.weight
		numerator += item.weight * clamp(item.value, 0, 1)
	}
	if denominator == 0 {
		return 0, 0
	}
	return numerator / denominator, denominator
}

func evidenceRefs(evidence []weightedEvidence) []string {
	refs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		if item.weight > 0 && strings.TrimSpace(item.ref) != "" {
			refs = append(refs, item.ref)
		}
	}
	return uniqueStrings(refs)
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func capMetricPoint(metric metricResult, maximum int, reason string) metricResult {
	if metric.PointLevel == nil || *metric.PointLevel <= maximum {
		if reason != "" && metric.Reason == "" {
			metric.Reason = reason
		}
		return metric
	}
	point := maximum
	score := float64(maximum)
	weighted := round2(score * 20 * metric.Weight)
	metric.PointLevel = &point
	metric.Score = &score
	metric.WeightedPoints = &weighted
	metric.Reason = reason
	return metric
}

func defectClosureMultiplier(task db.TaskTelemetry, events []db.PerformanceWorkItemEvent, watermark time.Time) float64 {
	reopens := 0
	for _, event := range events {
		if jiraStatusDone(event.FromValue) && !jiraStatusDone(event.ToValue) {
			reopens++
		}
	}
	overdueDays := 0.0
	if task.DueDate != nil {
		end := watermark
		if completedWorkItem(task) {
			end = *task.CompletedAt
		}
		if end.After(*task.DueDate) {
			overdueDays = end.Sub(*task.DueDate).Hours() / 24
		}
	}
	if reopens >= 2 || overdueDays > 7 {
		return 1.5
	}
	if reopens == 1 || overdueDays > 0 {
		return 1.2
	}
	return 1
}

func codeRiskAdjustments(enabled bool, validFingerprintRefs []string, duplicateCount, commitCount, completedDemandCount int) []adjustmentResult {
	result := make([]adjustmentResult, 0, 2)
	qualified := enabled && len(validFingerprintRefs) >= 10 && completedDemandCount >= 3
	duplicateRatio := 0.0
	if len(validFingerprintRefs) > 0 {
		duplicateRatio = float64(duplicateCount) / float64(len(validFingerprintRefs))
	}
	density := 0.0
	if completedDemandCount > 0 {
		density = float64(commitCount) / float64(completedDemandCount)
	}
	duplicatePenalty := 0.0
	densityPenalty := 0.0
	reason := "至少需要 10 个稳定指纹 Commit 和 3 个已完成需求，当前风险信号不参与扣分"
	if !enabled {
		reason = "代码风险扣分已由配置关闭"
	}
	if qualified {
		duplicatePenalty = duplicateChangePenalty(duplicateRatio)
		densityPenalty = commitDensityPenalty(density)
		reason = "达到代码风险样本门槛"
	}
	duplicateNumerator, duplicateDenominator := float64(duplicateCount), float64(len(validFingerprintRefs))
	densityNumerator, densityDenominator := float64(commitCount), float64(completedDemandCount)
	result = append(result, adjustmentResult{
		Type: "code_risk", Name: "重复变更率", ReasonCode: "DUPLICATE_CHANGE_RATE", Value: penaltyAdjustmentValue(duplicatePenalty),
		RawRatio: &duplicateRatio, Numerator: &duplicateNumerator, Denominator: &duplicateDenominator,
		RawUnit: "重复指纹 Commit/稳定指纹 Commit", EvidenceRefs: validFingerprintRefs, Reason: reason,
	})
	result = append(result, adjustmentResult{
		Type: "code_risk", Name: "Commit 密度风险", ReasonCode: "COMMIT_DENSITY", Value: penaltyAdjustmentValue(densityPenalty),
		RawRatio: &density, Numerator: &densityNumerator, Denominator: &densityDenominator,
		RawUnit: "Commit/已完成需求", EvidenceRefs: validFingerprintRefs, Reason: reason,
	})
	return result
}

func penaltyAdjustmentValue(penalty float64) float64 {
	if penalty == 0 {
		return 0
	}
	return -penalty
}

func duplicateChangePenalty(ratio float64) float64 {
	switch {
	case ratio <= 0.03:
		return 0
	case ratio <= 0.10:
		return 1
	case ratio <= 0.20:
		return 3
	case ratio <= 0.35:
		return 5
	default:
		return 6
	}
}

func commitDensityPenalty(density float64) float64 {
	switch {
	case density <= 8:
		return 0
	case density <= 10:
		return 1
	case density <= 12:
		return 2
	case density <= 14:
		return 3
	default:
		return 4
	}
}

func buildOwnershipSegments(task db.TaskTelemetry, events []db.PerformanceWorkItemEvent, windowStart, watermark time.Time) []ownershipSegment {
	assigneeEvents := filterSourceEvents(events, SourceEventAssigneeChange)
	start := task.TaskCreatedAt
	if start.IsZero() {
		start = windowStart
	}
	owner := strings.TrimSpace(task.Assignee)
	if len(assigneeEvents) > 0 && strings.TrimSpace(assigneeEvents[0].FromValue) != "" {
		owner = strings.TrimSpace(assigneeEvents[0].FromValue)
	}
	end := watermark
	endReason := "watermark"
	if completedWorkItem(task) && task.CompletedAt.Before(end) {
		end = *task.CompletedAt
		endReason = "completed"
	}
	segments := make([]ownershipSegment, 0, len(assigneeEvents)+1)
	cursor := start
	for _, event := range assigneeEvents {
		if event.OccurredAt.Before(cursor) {
			owner = strings.TrimSpace(event.ToValue)
			continue
		}
		if event.OccurredAt.After(end) {
			break
		}
		if owner != "" && event.OccurredAt.After(cursor) {
			segments = append(segments, ownershipSegment{SubjectKey: owner, Start: cursor, End: event.OccurredAt, EndReason: "transfer", Ref: sourceEventRef(event)})
		}
		owner = strings.TrimSpace(event.ToValue)
		cursor = event.OccurredAt
	}
	if owner != "" && end.After(cursor) {
		segments = append(segments, ownershipSegment{SubjectKey: owner, Start: cursor, End: end, EndReason: endReason, Ref: "jira:" + task.TaskID + ":owner_segment"})
	}
	result := make([]ownershipSegment, 0, len(segments))
	for _, segment := range segments {
		if segment.End.Before(windowStart) || segment.Start.After(watermark) {
			continue
		}
		if segment.Start.Before(windowStart) {
			segment.Start = windowStart
		}
		if segment.End.After(watermark) {
			segment.End = watermark
		}
		result = append(result, segment)
	}
	return result
}

func canonicalOwnershipSegments(segments []ownershipSegment, matcher coreMemberMatcher) []ownershipSegment {
	result := make([]ownershipSegment, 0, len(segments))
	for _, segment := range segments {
		subjectKey, included := matcher.canonical(segment.SubjectKey)
		if !included {
			continue
		}
		segment.SubjectKey = subjectKey
		result = append(result, segment)
	}
	return result
}

func responsibilityHours(segments []ownershipSegment, subject string, windowStart, completedAt time.Time) float64 {
	totalCalendarHours := 0.0
	for _, segment := range segments {
		if segment.End.Before(windowStart) || segment.Start.After(completedAt) {
			continue
		}
		start := segment.Start
		end := segment.End
		if start.Before(windowStart) {
			start = windowStart
		}
		if end.After(completedAt) {
			end = completedAt
		}
		if strings.EqualFold(strings.TrimSpace(segment.SubjectKey), strings.TrimSpace(subject)) {
			totalCalendarHours += end.Sub(start).Hours()
		}
	}
	return totalCalendarHours * 8 / 24
}

func filterSourceEvents(events []db.PerformanceWorkItemEvent, eventType string) []db.PerformanceWorkItemEvent {
	result := make([]db.PerformanceWorkItemEvent, 0)
	for _, event := range events {
		if event.EventType == eventType {
			result = append(result, event)
		}
	}
	return result
}

func ownerAt(segments []ownershipSegment, at time.Time) (string, bool) {
	for index := len(segments) - 1; index >= 0; index-- {
		segment := segments[index]
		if !at.Before(segment.Start) && !at.After(segment.End) {
			return segment.SubjectKey, true
		}
	}
	return "", false
}

func jiraStatusDone(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "done", "closed", "resolved", "completed", "完成", "已解决", "已关闭":
		return true
	default:
		return false
	}
}

func withEvidenceFraction(metric metricResult, evidence []weightedEvidence, unit string) metricResult {
	if !metric.Available {
		return metric
	}
	numerator := 0.0
	denominator := 0.0
	for _, item := range evidence {
		if item.weight <= 0 {
			continue
		}
		denominator += item.weight
		numerator += item.weight * clamp(item.value, 0, 1)
	}
	metric.Numerator = floatPointer(round2(numerator))
	metric.Denominator = floatPointer(round2(denominator))
	metric.RawUnit = unit
	return metric
}

func dedupeCommits(commits []db.GitCommitLog) []db.GitCommitLog {
	result := make([]db.GitCommitLog, 0, len(commits))
	seen := make(map[string]struct{}, len(commits))
	for _, commit := range commits {
		key := gitCommitDedupeKey(commit)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, commit)
	}
	return result
}

func gitCommitDedupeKey(commit db.GitCommitLog) string {
	return strings.ToLower(strings.TrimSpace(commit.Repo)) + ":" + strings.ToLower(strings.TrimSpace(commit.CommitID))
}

func gitEvidenceRef(commit db.GitCommitLog) string {
	return "git:" + strings.TrimSpace(commit.Repo) + ":" + strings.TrimSpace(commit.CommitID)
}

func sourceEventRef(event db.PerformanceWorkItemEvent) string {
	return event.SourceSystem + ":" + event.SourceEventID
}

func distinctSubjectBugCount(evidence subjectEvidence) int {
	seen := make(map[string]struct{})
	for _, bug := range evidence.Bugs {
		seen[bug.Task.TaskID] = struct{}{}
	}
	for _, defect := range evidence.Defects {
		seen[defect.Task.TaskID] = struct{}{}
	}
	return len(seen)
}

func workItemEvidenceRef(task db.TaskTelemetry, suffix string) string {
	prefix := strings.ToLower(strings.TrimSpace(task.Source))
	if prefix == "" {
		prefix = "task"
	}
	return prefix + ":" + task.TaskID + ":" + suffix
}

func calculateExposureDays(evidence subjectEvidence, windowStart, watermark time.Time) int {
	earliest := watermark
	found := false
	consider := func(candidate time.Time) {
		if candidate.IsZero() || candidate.After(watermark) {
			return
		}
		if candidate.Before(windowStart) {
			candidate = windowStart
		}
		if !found || candidate.Before(earliest) {
			earliest = candidate
			found = true
		}
	}
	for _, unit := range evidence.Units {
		consider(unit.Task.TaskCreatedAt)
	}
	for _, bug := range evidence.Bugs {
		consider(bug.Task.TaskCreatedAt)
	}
	for _, fact := range evidence.Facts {
		consider(fact.OccurredAt)
	}
	for _, commit := range evidence.Commits {
		consider(commit.CreatedAt)
	}
	if !found {
		return 0
	}
	return int(watermark.Sub(earliest).Hours() / 24)
}

func completedWorkItem(task db.TaskTelemetry) bool {
	return strings.EqualFold(strings.TrimSpace(task.Status), "done") && task.CompletedAt != nil
}

func formalEvidenceRef(fact db.PerformanceEvidenceFact) string {
	if strings.TrimSpace(fact.EvidenceRef) != "" {
		return fact.EvidenceRef
	}
	return fact.EvidenceKey
}

func boolRatio(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func floatPointer(value float64) *float64 { return &value }

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
