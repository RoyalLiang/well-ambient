package openaccess

import (
	"context"
	"fmt"
	"sort"
	"time"

	"well-ambient/internal/db"
)

type InvocationMetric struct {
	ToolName    string         `json:"tool_name"`
	Total       int            `json:"total"`
	Succeeded   int            `json:"succeeded"`
	Failed      int            `json:"failed"`
	AverageMS   float64        `json:"average_ms"`
	ErrorCodes  map[string]int `json:"error_codes"`
	SuccessRate float64        `json:"success_rate"`
}

type OperationMetric struct {
	Total              int            `json:"total"`
	States             map[string]int `json:"states"`
	ConfirmedRate      float64        `json:"confirmed_rate"`
	UnknownCount       int            `json:"unknown_count"`
	OldestUnknownSince *time.Time     `json:"oldest_unknown_since,omitempty"`
}

type IntelligenceReport struct {
	GeneratedAt     time.Time          `json:"generated_at"`
	WindowStart     time.Time          `json:"window_start"`
	InvocationTotal int                `json:"invocation_total"`
	Invocations     []InvocationMetric `json:"invocations"`
	Operations      OperationMetric    `json:"operations"`
	Recommendations []string           `json:"recommendations"`
}

func (s *Service) Intelligence(ctx context.Context, window time.Duration) (IntelligenceReport, error) {
	if window <= 0 {
		window = 30 * 24 * time.Hour
	}
	now := s.now().UTC()
	since := now.Add(-window)
	var invocations []db.CapabilityInvocation
	if err := s.db.WithContext(ctx).
		Where("created_at >= ?", since).
		Order("created_at ASC, id ASC").
		Limit(10000).
		Find(&invocations).Error; err != nil {
		return IntelligenceReport{}, fmt.Errorf("load capability invocations: %w", err)
	}
	type invocationAggregate struct {
		metric      InvocationMetric
		durationSum int64
	}
	aggregates := map[string]*invocationAggregate{}
	for _, invocation := range invocations {
		aggregate := aggregates[invocation.ToolName]
		if aggregate == nil {
			aggregate = &invocationAggregate{metric: InvocationMetric{
				ToolName: invocation.ToolName, ErrorCodes: map[string]int{},
			}}
			aggregates[invocation.ToolName] = aggregate
		}
		aggregate.metric.Total++
		aggregate.durationSum += invocation.DurationMS
		if invocation.Outcome == "succeeded" {
			aggregate.metric.Succeeded++
		} else {
			aggregate.metric.Failed++
			if invocation.ErrorCode != "" {
				aggregate.metric.ErrorCodes[invocation.ErrorCode]++
			}
		}
	}
	toolNames := make([]string, 0, len(aggregates))
	for name := range aggregates {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)
	metrics := make([]InvocationMetric, 0, len(toolNames))
	for _, name := range toolNames {
		aggregate := aggregates[name]
		if aggregate.metric.Total > 0 {
			aggregate.metric.AverageMS = float64(aggregate.durationSum) / float64(aggregate.metric.Total)
			aggregate.metric.SuccessRate = float64(aggregate.metric.Succeeded) / float64(aggregate.metric.Total)
		}
		metrics = append(metrics, aggregate.metric)
	}

	var operations []db.OpenOperation
	if err := s.db.WithContext(ctx).
		Where("created_at >= ?", since).
		Order("created_at ASC, id ASC").
		Limit(10000).
		Find(&operations).Error; err != nil {
		return IntelligenceReport{}, fmt.Errorf("load open operations: %w", err)
	}
	operationMetric := OperationMetric{States: map[string]int{}}
	confirmed := 0
	for _, operation := range operations {
		operationMetric.Total++
		operationMetric.States[operation.State]++
		switch operation.State {
		case "succeeded":
			confirmed++
		case "unknown":
			operationMetric.UnknownCount++
			if operationMetric.OldestUnknownSince == nil || operation.UpdatedAt.Before(*operationMetric.OldestUnknownSince) {
				value := operation.UpdatedAt
				operationMetric.OldestUnknownSince = &value
			}
		}
	}
	if operationMetric.Total > 0 {
		operationMetric.ConfirmedRate = float64(confirmed) / float64(operationMetric.Total)
	}
	recommendations := buildIntelligenceRecommendations(metrics, operationMetric, now)
	return IntelligenceReport{
		GeneratedAt: now, WindowStart: since, InvocationTotal: len(invocations),
		Invocations: metrics, Operations: operationMetric, Recommendations: recommendations,
	}, nil
}

func buildIntelligenceRecommendations(
	metrics []InvocationMetric,
	operations OperationMetric,
	now time.Time,
) []string {
	recommendations := []string{}
	if operations.UnknownCount > 0 {
		recommendation := "优先核验 unknown Operation；不要自动重发 Jira 写操作。"
		if operations.OldestUnknownSince != nil {
			recommendation = fmt.Sprintf(
				"优先核验 %d 个 unknown Operation；最早一条已持续 %s，不要自动重发 Jira 写操作。",
				operations.UnknownCount,
				now.Sub(*operations.OldestUnknownSince).Round(time.Minute),
			)
		}
		recommendations = append(recommendations, recommendation)
	}
	for _, metric := range metrics {
		if metric.Total >= 5 && metric.SuccessRate < 0.9 {
			recommendations = append(recommendations, fmt.Sprintf(
				"工具 %s 成功率为 %.1f%%；先按错误码聚类并用保存样本离线复核契约或 Skill。",
				metric.ToolName, metric.SuccessRate*100,
			))
		}
		if metric.ErrorCodes["invalid_query"] >= 3 {
			recommendations = append(recommendations, fmt.Sprintf(
				"工具 %s 的 invalid_query 较多；检查 Skill 示例、字段发现和查询预算说明。",
				metric.ToolName,
			))
		}
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "当前窗口没有需要自动升级权限或写策略的证据；继续采集调用事实。")
	}
	return recommendations
}
