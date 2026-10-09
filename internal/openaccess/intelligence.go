package openaccess

import (
	"context"
	"fmt"
	"sort"
	"time"

	"well-ambient/internal/db"
)

type InvocationMetric struct {
	ToolName     string         `json:"tool_name"`
	Total        int            `json:"total"`
	Succeeded    int            `json:"succeeded"`
	Failed       int            `json:"failed"`
	StartedCount int            `json:"started_count"`
	AverageMS    float64        `json:"average_ms"`
	ErrorCodes   map[string]int `json:"error_codes"`
	SuccessRate  float64        `json:"success_rate"`
}

type OperationMetric struct {
	Total              int            `json:"total"`
	States             map[string]int `json:"states"`
	ConfirmedRate      float64        `json:"confirmed_rate"`
	UnknownCount       int            `json:"unknown_count"`
	OldestUnknownSince *time.Time     `json:"oldest_unknown_since,omitempty"`
	// Outstanding risks include unresolved operations created before the reporting window.
	OutstandingUnknownCount           int        `json:"outstanding_unknown_count"`
	OldestOutstandingUnknownUpdatedAt *time.Time `json:"oldest_outstanding_unknown_updated_at,omitempty"`
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
	var invocations []struct {
		ToolName   string
		Outcome    string
		ErrorCode  string
		Total      int
		DurationMS int64
	}
	if err := s.db.WithContext(ctx).Model(&db.CapabilityInvocation{}).
		Select("tool_name, outcome, error_code, COUNT(*) AS total, COALESCE(SUM(duration_ms), 0) AS duration_ms").
		Where("created_at >= ? AND created_at <= ?", since, now).
		Group("tool_name, outcome, error_code").
		Scan(&invocations).Error; err != nil {
		return IntelligenceReport{}, fmt.Errorf("aggregate capability invocations: %w", err)
	}
	type invocationAggregate struct {
		metric      InvocationMetric
		durationSum int64
	}
	aggregates := map[string]*invocationAggregate{}
	invocationTotal := 0
	for _, invocation := range invocations {
		aggregate := aggregates[invocation.ToolName]
		if aggregate == nil {
			aggregate = &invocationAggregate{metric: InvocationMetric{
				ToolName: invocation.ToolName, ErrorCodes: map[string]int{},
			}}
			aggregates[invocation.ToolName] = aggregate
		}
		invocationTotal += invocation.Total
		aggregate.metric.Total += invocation.Total
		if invocation.Outcome == "started" {
			aggregate.metric.StartedCount += invocation.Total
			continue
		}
		aggregate.durationSum += invocation.DurationMS
		if invocation.Outcome == "succeeded" {
			aggregate.metric.Succeeded += invocation.Total
		} else {
			aggregate.metric.Failed += invocation.Total
			if invocation.ErrorCode != "" {
				aggregate.metric.ErrorCodes[invocation.ErrorCode] += invocation.Total
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
		completed := aggregate.metric.Total - aggregate.metric.StartedCount
		if completed > 0 {
			aggregate.metric.AverageMS = float64(aggregate.durationSum) / float64(completed)
			aggregate.metric.SuccessRate = float64(aggregate.metric.Succeeded) / float64(completed)
		}
		metrics = append(metrics, aggregate.metric)
	}

	var operations []struct {
		State string
		Total int
	}
	if err := s.db.WithContext(ctx).Model(&db.OpenOperation{}).
		Select("state, COUNT(*) AS total").
		Where("created_at >= ? AND created_at <= ?", since, now).
		Group("state").
		Scan(&operations).Error; err != nil {
		return IntelligenceReport{}, fmt.Errorf("aggregate open operations: %w", err)
	}
	operationMetric := OperationMetric{States: map[string]int{}}
	confirmed := 0
	for _, operation := range operations {
		operationMetric.Total += operation.Total
		operationMetric.States[operation.State] += operation.Total
		switch operation.State {
		case "succeeded":
			confirmed += operation.Total
		case "unknown":
			operationMetric.UnknownCount += operation.Total
		}
	}
	if operationMetric.UnknownCount > 0 {
		var oldest db.OpenOperation
		if err := s.db.WithContext(ctx).
			Where("created_at >= ? AND created_at <= ? AND state = ?", since, now, "unknown").
			Order("updated_at ASC, id ASC").Limit(1).Find(&oldest).Error; err != nil {
			return IntelligenceReport{}, fmt.Errorf("load oldest unknown operation: %w", err)
		}
		if oldest.ID != "" {
			operationMetric.OldestUnknownSince = &oldest.UpdatedAt
		}
	}
	if operationMetric.Total > 0 {
		operationMetric.ConfirmedRate = float64(confirmed) / float64(operationMetric.Total)
	}
	var outstandingUnknownCount int64
	if err := s.db.WithContext(ctx).Model(&db.OpenOperation{}).
		Where("state = ?", "unknown").Count(&outstandingUnknownCount).Error; err != nil {
		return IntelligenceReport{}, fmt.Errorf("count outstanding unknown operations: %w", err)
	}
	operationMetric.OutstandingUnknownCount = int(outstandingUnknownCount)
	if outstandingUnknownCount > 0 {
		var oldest db.OpenOperation
		if err := s.db.WithContext(ctx).Where("state = ?", "unknown").
			Order("updated_at ASC, id ASC").Limit(1).Find(&oldest).Error; err != nil {
			return IntelligenceReport{}, fmt.Errorf("load oldest outstanding unknown operation: %w", err)
		}
		if oldest.ID != "" {
			operationMetric.OldestOutstandingUnknownUpdatedAt = &oldest.UpdatedAt
		}
	}
	recommendations := buildIntelligenceRecommendations(metrics, operationMetric, now)
	return IntelligenceReport{
		GeneratedAt: now, WindowStart: since, InvocationTotal: invocationTotal,
		Invocations: metrics, Operations: operationMetric, Recommendations: recommendations,
	}, nil
}

func buildIntelligenceRecommendations(
	metrics []InvocationMetric,
	operations OperationMetric,
	now time.Time,
) []string {
	recommendations := []string{}
	if operations.OutstandingUnknownCount > 0 {
		recommendation := "优先核验 unknown Operation；不要自动重发 Jira 写操作。"
		if operations.OldestOutstandingUnknownUpdatedAt != nil {
			recommendation = fmt.Sprintf(
				"优先核验 %d 个 unknown Operation；最早更新时间距今 %s，不要自动重发 Jira 写操作。",
				operations.OutstandingUnknownCount,
				now.Sub(*operations.OldestOutstandingUnknownUpdatedAt).Round(time.Minute),
			)
		}
		recommendations = append(recommendations, recommendation)
	}
	for _, metric := range metrics {
		if metric.StartedCount > 0 {
			recommendations = append(recommendations, fmt.Sprintf(
				"工具 %s 有 %d 个审计未结束调用；核验请求状态及审计完成记录，不要将其视为业务成功或自动重发写操作。",
				metric.ToolName, metric.StartedCount,
			))
		}
		if metric.Total-metric.StartedCount >= 5 && metric.SuccessRate < 0.9 {
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
