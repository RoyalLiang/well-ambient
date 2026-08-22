package performance

import (
	"fmt"
	"strings"
)

type metricDirection string

const (
	directionHigherIsBetter metricDirection = "higher_is_better"
	directionLowerIsBetter  metricDirection = "lower_is_better"
)

type metricRule struct {
	Code             string
	Dimension        string
	Name             string
	Weight           float64
	Direction        metricDirection
	MinimumSamples   int
	Point2Boundary   float64
	Point3Boundary   float64
	Point4Boundary   float64
	Point5Boundary   float64
	Core             bool
	Source           string
	Formula          string
	RequiredEvidence string
}

var v6MetricRules = []metricRule{
	{metricDemandCompletion, "交付结果", "需求加权完成率", 0.35, directionHigherIsBetter, 5, 0.50, 0.70, 0.85, 0.95, true, "Jira resolution / due_date", "Σ周期内已完成需求权重 ÷ Σ周期内完成或到期需求权重", "完成时间或到期日、需求等级、项目等级和当前完成状态"},
	{metricDemandOnTime, "交付可预测性", "交付可预测性", 0.20, directionHigherIsBetter, 3, 0.55, 0.75, 0.90, 0.97, true, "Jira resolutiondate / due_date / original estimate / assignee history", "70% × 加权按期率 + 30% × 计划周期兑现率", "完成时间、到期日、原始估算、负责人责任周期和需求权重"},
	{metricDefectDensity, "工程质量", "责任加权缺陷损失率", 0.45, directionLowerIsBetter, 5, 0.90, 0.60, 0.35, 0.15, true, "Jira parent/issue links + performance_evidence_facts.defect_attribution", "Σ严重度 × 逃逸阶段 × 归责份额 × 闭环系数 ÷ Σ已完成且充分暴露的需求权重", "缺陷来源需求、需求完成负责人、严重度、逃逸阶段、重开/延期和至少 30 天质量暴露"},
}

var v4AdjustmentValues = map[string]float64{
	"TREND_DOWN_SUSTAINED":    -5,
	"TREND_DOWN":              -3,
	"TREND_STABLE":            0,
	"TREND_UP":                3,
	"TREND_UP_SUSTAINED":      5,
	"TRUST_GOVERNANCE_BREACH": -10,
	"TRUST_MAJOR_AVOIDABLE":   -8,
	"TRUST_HIDDEN_RISK":       -5,
	"TRUST_REPEAT_FAILURE":    -3,
	"LEV_TEAM_REUSE":          3,
	"LEV_CROSS_PROJECT":       5,
	"LEV_ORG":                 8,
	"LEV_STRATEGIC":           10,
}

func adjustmentValue(reasonCode string) (float64, bool) {
	value, ok := v4AdjustmentValues[reasonCode]
	return value, ok
}

func releaseMethodFactor(method string) (float64, bool) {
	switch normalizeFactorToken(method) {
	case "全量发布", "全量", "full":
		return 1, true
	case "灰度发布", "灰度", "canary":
		return 0.70, true
	case "热修复", "hotfix":
		return 0.80, true
	default:
		return 0, false
	}
}

func rollbackImpactFactor(impact string) (float64, bool) {
	switch normalizeFactorToken(impact) {
	case "全量", "full":
		return 1.50, true
	case "部分", "partial":
		return 1, true
	case "配置/单节点", "配置", "单节点", "config", "single-node", "single_node":
		return 0.50, true
	default:
		return 0, false
	}
}

func responsibilityFactor(value float64) (float64, bool) {
	switch value {
	case 0, 0.5, 1:
		return value, true
	default:
		return 0, false
	}
}

func normalizeFactorToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func metricRuleFor(code string) (metricRule, bool) {
	for _, rule := range v6MetricRules {
		if rule.Code == code {
			return rule, true
		}
	}
	return metricRule{}, false
}

func mustMetricRule(code string) metricRule {
	rule, ok := metricRuleFor(code)
	if !ok {
		panic("missing performance metric rule: " + code)
	}
	return rule
}

func scoreForRatio(rule metricRule, ratio float64) float64 {
	ratio = clamp(ratio, 0, 1)
	point := 1
	if rule.Direction == directionLowerIsBetter {
		switch {
		case ratio <= rule.Point5Boundary:
			point = 5
		case ratio <= rule.Point4Boundary:
			point = 4
		case ratio <= rule.Point3Boundary:
			point = 3
		case ratio <= rule.Point2Boundary:
			point = 2
		}
	} else {
		switch {
		case ratio >= rule.Point5Boundary:
			point = 5
		case ratio >= rule.Point4Boundary:
			point = 4
		case ratio >= rule.Point3Boundary:
			point = 3
		case ratio >= rule.Point2Boundary:
			point = 2
		}
	}
	return float64(point)
}

func metricFromEvidence(rule metricRule, evidence []weightedEvidence) metricResult {
	totalWeight := 0.0
	weightedValue := 0.0
	refs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		if item.weight <= 0 {
			continue
		}
		totalWeight += item.weight
		value := item.value
		if rule.Direction == directionHigherIsBetter {
			value = clamp(value, 0, 1)
		} else if value < 0 {
			value = 0
		}
		weightedValue += item.weight * value
		refs = append(refs, item.ref)
	}
	if len(refs) == 0 || totalWeight == 0 {
		return metricUnavailableWithEvidence(rule, refs, "当前考核周期没有满足该指标口径的有效样本")
	}
	return metricFromRatio(rule, weightedValue/totalWeight, refs)
}

func metricFromRatio(rule metricRule, ratio float64, refs []string) metricResult {
	if rule.Direction == directionHigherIsBetter {
		ratio = clamp(ratio, 0, 1)
	} else if ratio < 0 {
		ratio = 0
	}
	ratio = round4(ratio)
	score := scoreForRatio(rule, ratio)
	pointLevel := int(score)
	weightedPoints := round2(score * 20 * rule.Weight)
	result := metricResult{
		Code: rule.Code, Name: rule.Name, Weight: rule.Weight, Available: true,
		SampleQualified: len(refs) >= rule.MinimumSamples,
		Direction:       string(rule.Direction), MinimumSamples: rule.MinimumSamples,
		RawRatio: &ratio, PointLevel: &pointLevel, Score: &score, WeightedPoints: &weightedPoints,
		EvidenceCount: len(refs), EvidenceRefs: refs,
	}
	if !result.SampleQualified {
		result.Reason = fmt.Sprintf("有效样本 %d 个，低于判定表要求的 %d 个；仅形成参考分，不计入正式评分", len(refs), rule.MinimumSamples)
	}
	return result
}

func metricUnavailableForRule(rule metricRule, reason string) metricResult {
	return metricUnavailableWithEvidence(rule, nil, reason)
}

func metricUnavailableWithEvidence(rule metricRule, refs []string, reason string) metricResult {
	return metricResult{
		Code: rule.Code, Name: rule.Name, Weight: rule.Weight, Available: false,
		Direction: string(rule.Direction), MinimumSamples: rule.MinimumSamples,
		EvidenceCount: len(refs), EvidenceRefs: refs, Reason: reason,
	}
}
