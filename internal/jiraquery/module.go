package jiraquery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
	"well-ambient/internal/readmodel"
)

const (
	SchemaVersion           = "open-jira/v1"
	MetricDefinitionVersion = "jira-metrics/v1"
	ToolVersion             = "1"
)

var defaultFields = []string{
	"key", "project", "summary", "issue_type", "status", "assignee",
	"priority", "bug_category", "due_date", "created_at", "updated_at",
	"history_complete",
}

type SnapshotMeta struct {
	RequestID               string    `json:"request_id,omitempty"`
	SchemaVersion           string    `json:"schema_version"`
	PolicyVersion           int       `json:"policy_version"`
	MetricDefinitionVersion string    `json:"metric_definition_version"`
	DataAsOf                time.Time `json:"data_as_of"`
	QuerySnapshotRef        string    `json:"query_snapshot_ref"`
	Completeness            float64   `json:"completeness"`
	MissingReasons          []string  `json:"missing_reasons"`
	SourceRefs              []string  `json:"source_refs,omitempty"`
}

type Filters struct {
	Projects      []string   `json:"projects,omitempty"`
	IssueTypes    []string   `json:"issue_types,omitempty"`
	Statuses      []string   `json:"statuses,omitempty"`
	Assignees     []string   `json:"assignees,omitempty"`
	Priorities    []string   `json:"priorities,omitempty"`
	BugCategories []string   `json:"bug_categories,omitempty"`
	UpdatedFrom   *time.Time `json:"updated_from,omitempty"`
	UpdatedUntil  *time.Time `json:"updated_until,omitempty"`
	DueBefore     *time.Time `json:"due_before,omitempty"`
	Unassigned    *bool      `json:"unassigned,omitempty"`
	Overdue       *bool      `json:"overdue,omitempty"`
}

type SearchRequest struct {
	Filters Filters  `json:"filters,omitempty"`
	Fields  []string `json:"fields,omitempty"`
	Cursor  string   `json:"cursor,omitempty"`
	Limit   int      `json:"limit,omitempty"`
}

type SearchResponse struct {
	Items      []map[string]any `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
	Meta       SnapshotMeta     `json:"meta"`
}

type HistoryItem struct {
	ID         uint      `json:"id"`
	HistoryID  string    `json:"history_id"`
	Field      string    `json:"field"`
	FromValue  string    `json:"from_value,omitempty"`
	ToValue    string    `json:"to_value,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type IssueDetail struct {
	Issue      map[string]any `json:"issue"`
	History    []HistoryItem  `json:"history,omitempty"`
	NextCursor string         `json:"next_cursor,omitempty"`
	HasMore    bool           `json:"has_more"`
	Meta       SnapshotMeta   `json:"meta"`
}

type GetRequest struct {
	Key            string   `json:"key"`
	Fields         []string `json:"fields,omitempty"`
	IncludeHistory bool     `json:"include_history,omitempty"`
	HistoryCursor  string   `json:"history_cursor,omitempty"`
	HistoryLimit   int      `json:"history_limit,omitempty"`
}

type AggregateRequest struct {
	Filters    Filters    `json:"filters,omitempty"`
	Metrics    []string   `json:"metrics"`
	GroupBy    []string   `json:"group_by,omitempty"`
	EventFrom  *time.Time `json:"event_from,omitempty"`
	EventUntil *time.Time `json:"event_until,omitempty"`
	TimeBucket string     `json:"time_bucket,omitempty"`
}

type AggregateRow struct {
	Groups  map[string]string  `json:"groups"`
	Metrics map[string]float64 `json:"metrics"`
}

type AggregateResponse struct {
	Rows               []AggregateRow    `json:"rows"`
	MetricCompleteness map[string]string `json:"metric_completeness"`
	TimeZone           string            `json:"time_zone"`
	Meta               SnapshotMeta      `json:"meta"`
}

type SchemaResponse struct {
	Fields     []FieldDefinition  `json:"fields"`
	Dimensions []string           `json:"dimensions"`
	Metrics    []MetricDefinition `json:"metrics"`
	Meta       SnapshotMeta       `json:"meta"`
}

type FieldDefinition struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	TimeSemantic string `json:"time_semantic,omitempty"`
	Description  string `json:"description"`
}

type MetricDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Requires    string `json:"requires,omitempty"`
}

type Module struct {
	db  *gorm.DB
	now func() time.Time
}

func New(database *gorm.DB) *Module {
	return &Module{db: database, now: time.Now}
}

func (m *Module) RunSnapshotCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		_ = m.db.WithContext(ctx).Where("expires_at <= ?", m.now().UTC()).Delete(&db.OpenQuerySnapshot{}).Error
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Module) DescribeSchema(ctx context.Context, policy openaccess.Policy, requestID string) (SchemaResponse, error) {
	if !policy.AllowsAction("jira.read") {
		return SchemaResponse{}, openaccess.NewError("forbidden", "Jira reading is not allowed by the active policy")
	}
	if !policy.JiraCachePublicationVerified() {
		return SchemaResponse{}, openaccess.NewError("forbidden", "Jira cache publication has not been verified")
	}
	if len(policy.AllowedProjects) == 0 {
		return SchemaResponse{}, openaccess.NewError("forbidden", "Jira query has no published project scope")
	}
	var rows []struct {
		Complete bool
		Count    int64
	}
	if err := m.db.WithContext(ctx).Model(&db.TaskTelemetry{}).
		Select("jira_history_complete AS complete, COUNT(*) AS count").
		Where("LOWER(source) = ? AND UPPER(project_key) IN ?", "jira", upper(policy.AllowedProjects)).
		Group("jira_history_complete").
		Scan(&rows).Error; err != nil {
		return SchemaResponse{}, fmt.Errorf("describe Jira schema: %w", err)
	}
	var total, complete int64
	for _, row := range rows {
		total += row.Count
		if row.Complete {
			complete += row.Count
		}
	}
	dataAsOf, err := m.dataAsOf(ctx, policy.AllowedProjects)
	if err != nil {
		return SchemaResponse{}, err
	}
	meta := buildMeta(requestID, policy, dataAsOf, complete, total, "schema", nil)
	fields := []FieldDefinition{
		{Name: "key", Type: "string", Description: "Jira issue key"},
		{Name: "project", Type: "string", Description: "authoritative Jira project key"},
		{Name: "summary", Type: "string", Description: "current issue summary"},
		{Name: "issue_type", Type: "string", Description: "normalized issue type"},
		{Name: "status", Type: "string", Description: "current synchronized status"},
		{Name: "assignee", Type: "string", Description: "current synchronized assignee"},
		{Name: "priority", Type: "string", Description: "current Jira priority"},
		{Name: "bug_category", Type: "string", Description: "configured authoritative bug category field"},
		{Name: "due_date", Type: "date", TimeSemantic: "current", Description: "current due date"},
		{Name: "created_at", Type: "timestamp", TimeSemantic: "source", Description: "Jira issue creation time"},
		{Name: "updated_at", Type: "timestamp", TimeSemantic: "source", Description: "last synchronized Jira update time"},
		{Name: "history_complete", Type: "boolean", Description: "whether synchronized changelog is complete"},
	}
	published := make([]FieldDefinition, 0, len(fields))
	for _, field := range fields {
		if policy.AllowsFields("jira_describe_schema", []string{field.Name}) {
			published = append(published, field)
		}
	}
	dimensions := []string{}
	for _, dimension := range []string{"project", "issue_type", "status", "assignee", "priority", "bug_category"} {
		if policy.AllowsFields("jira_describe_schema", []string{dimension}) {
			dimensions = append(dimensions, dimension)
		}
	}
	metrics := []MetricDefinition{}
	for _, metric := range []MetricDefinition{
		{Name: "issue_count", Description: "distinct current issues"},
		{Name: "unassigned_count", Description: "current issues without an assignee"},
		{Name: "overdue_count", Description: "unresolved current issues whose due date is before the query time"},
		{Name: "history_complete_count", Description: "issues with complete synchronized changelog"},
		{Name: "history_incomplete_count", Description: "issues without complete synchronized changelog"},
		{Name: "created_issue_count", Description: "issues created in the half-open event interval"},
		{Name: "resolved_event_count", Description: "status changes into a resolved state", Requires: "complete Jira history for full results"},
		{Name: "resolved_issue_count", Description: "distinct issues resolved in the event interval", Requires: "complete Jira history for full results"},
		{Name: "reopened_event_count", Description: "status changes from resolved back to unresolved", Requires: "complete Jira history for full results"},
		{Name: "assignee_change_event_count", Description: "assignee change events in the event interval", Requires: "complete Jira history for full results"},
		{Name: "assignee_change_issue_count", Description: "distinct issues with an assignee change", Requires: "complete Jira history for full results"},
		{Name: "status_duration_hours", Description: "complete-history time spent in each status", Requires: "group_by=status"},
	} {
		if policy.AllowsFields("jira_describe_schema", metricFields(metric.Name)) {
			metrics = append(metrics, metric)
		}
	}
	return SchemaResponse{
		Fields: published, Dimensions: dimensions, Metrics: metrics,
		Meta: meta,
	}, nil
}

func (m *Module) Search(ctx context.Context, policy openaccess.Policy, requestID string, request SearchRequest) (SearchResponse, error) {
	if !policy.AllowsAction("jira.read") {
		return SearchResponse{}, openaccess.NewError("forbidden", "Jira reading is not allowed by the active policy")
	}
	projects, err := authorizedProjects(policy, request.Filters.Projects)
	if err != nil {
		return SearchResponse{}, err
	}
	fields, err := publishedFields(policy, "jira_search_issues", request.Fields, filterFields(request.Filters)...)
	if err != nil {
		return SearchResponse{}, err
	}
	limit := request.Limit
	if limit <= 0 {
		limit = min(50, policy.QueryLimits.MaxPageSize)
	}
	if limit > policy.QueryLimits.MaxPageSize {
		limit = policy.QueryLimits.MaxPageSize
	}
	scope := struct {
		PolicyVersion int
		Projects      []string
		Filters       Filters
		Fields        []string
	}{policy.Version, projects, normalizeFilters(request.Filters), fields}
	codec, err := readmodel.NewCursorCodec("open-jira-search/v1", scope)
	if err != nil {
		return SearchResponse{}, err
	}
	position := searchPosition{}
	var snapshot db.OpenQuerySnapshot
	var payload searchSnapshotPayload
	if request.Cursor != "" {
		cursorWatermark, decodeErr := codec.Decode(request.Cursor, &position)
		if decodeErr != nil {
			return SearchResponse{}, openaccess.NewError("invalid_query", "cursor is invalid for this query")
		}
		if position.SnapshotID == "" || position.Offset < 0 {
			return SearchResponse{}, openaccess.NewError("invalid_query", "cursor position is invalid")
		}
		if _, parseErr := time.Parse(time.RFC3339Nano, cursorWatermark); parseErr != nil {
			return SearchResponse{}, openaccess.NewError("invalid_query", "cursor watermark is invalid")
		}
		err = m.db.WithContext(ctx).
			Where(
				"id = ? AND contract = ? AND policy_version = ? AND scope_digest = ? AND expires_at > ?",
				position.SnapshotID, "open-jira-search/v1", policy.Version, digestValue(scope), m.now().UTC(),
			).
			First(&snapshot).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SearchResponse{}, openaccess.NewError("invalid_query", "query snapshot has expired or is unavailable")
		}
		if err != nil {
			return SearchResponse{}, fmt.Errorf("load Jira query snapshot: %w", err)
		}
		if err := json.Unmarshal([]byte(snapshot.PayloadJSON), &payload); err != nil {
			return SearchResponse{}, fmt.Errorf("decode Jira query snapshot: %w", err)
		}
	} else {
		dataAsOf, watermarkErr := m.dataAsOf(ctx, projects)
		if watermarkErr != nil {
			return SearchResponse{}, watermarkErr
		}
		var tasks []db.TaskTelemetry
		if err := m.filteredQuery(ctx, projects, request.Filters).
			Where("source_updated_at <= ?", dataAsOf).
			Order("source_updated_at DESC, task_id ASC").
			Limit(policy.QueryLimits.MaxScanRows + 1).
			Find(&tasks).Error; err != nil {
			return SearchResponse{}, fmt.Errorf("search Jira issues: %w", err)
		}
		if len(tasks) > policy.QueryLimits.MaxScanRows {
			return SearchResponse{}, openaccess.NewError("invalid_query", "query exceeds the configured snapshot budget")
		}
		complete := int64(0)
		for _, task := range tasks {
			if task.JiraHistoryComplete {
				complete++
			}
			payload.Items = append(payload.Items, projectIssue(task, fields))
			payload.SourceRefs = append(payload.SourceRefs, issueSourceRef(task))
		}
		payload.Complete = complete
		payload.Total = int64(len(tasks))
		normalized, err := json.Marshal(payload)
		if err != nil {
			return SearchResponse{}, err
		}
		if err := json.Unmarshal(normalized, &payload); err != nil {
			return SearchResponse{}, err
		}
		snapshotID, err := openaccess.RandomID("jira_snapshot")
		if err != nil {
			return SearchResponse{}, err
		}
		now := m.now().UTC()
		snapshot = db.OpenQuerySnapshot{
			ID: snapshotID, Contract: "open-jira-search/v1", PolicyVersion: policy.Version,
			ScopeDigest: digestValue(scope), DataAsOf: dataAsOf.UTC(),
			PayloadJSON: string(normalized), Completeness: completeness(payload.Complete, payload.Total),
			MissingJSON: `[]`, ExpiresAt: now.Add(30 * time.Minute), CreatedAt: now,
		}
		if snapshot.Completeness < 1 {
			snapshot.MissingJSON = `["one or more issues have incomplete Jira changelog history"]`
		}
		if err := m.db.WithContext(ctx).Create(&snapshot).Error; err != nil {
			return SearchResponse{}, fmt.Errorf("persist Jira query snapshot: %w", err)
		}
		_ = m.db.WithContext(ctx).Where("expires_at <= ?", now).Delete(&db.OpenQuerySnapshot{}).Error
		position = searchPosition{SnapshotID: snapshot.ID, Offset: 0}
	}
	if position.Offset > len(payload.Items) {
		return SearchResponse{}, openaccess.NewError("invalid_query", "cursor offset is outside the query snapshot")
	}
	end := min(len(payload.Items), position.Offset+limit)
	items := payload.Items[position.Offset:end]
	refs := payload.SourceRefs[position.Offset:end]
	hasMore := end < len(payload.Items)
	var nextCursor string
	if hasMore {
		nextCursor, err = codec.Encode(
			snapshot.DataAsOf.UTC().Format(time.RFC3339Nano),
			searchPosition{SnapshotID: snapshot.ID, Offset: end},
		)
		if err != nil {
			return SearchResponse{}, err
		}
	}
	meta := buildMeta(requestID, policy, snapshot.DataAsOf, payload.Complete, payload.Total, scope, refs)
	meta.QuerySnapshotRef = "jira-query:" + snapshot.ID
	return SearchResponse{Items: items, NextCursor: nextCursor, HasMore: hasMore, Meta: meta}, nil
}

func (m *Module) Get(ctx context.Context, policy openaccess.Policy, requestID string, request GetRequest) (IssueDetail, error) {
	if !policy.AllowsAction("jira.read") {
		return IssueDetail{}, openaccess.NewError("forbidden", "Jira reading is not allowed by the active policy")
	}
	if !policy.JiraCachePublicationVerified() {
		return IssueDetail{}, openaccess.NewError("forbidden", "Jira cache publication has not been verified")
	}
	if len(policy.AllowedProjects) == 0 {
		return IssueDetail{}, openaccess.NewError("forbidden", "Jira query has no published project scope")
	}
	key := strings.ToUpper(strings.TrimSpace(request.Key))
	if key == "" {
		return IssueDetail{}, openaccess.NewError("invalid_query", "issue key is required")
	}
	extraFields := []string{}
	if request.IncludeHistory {
		extraFields = append(extraFields, "history")
	}
	fields, err := publishedFields(policy, "jira_get_issue", request.Fields, extraFields...)
	if err != nil {
		return IssueDetail{}, err
	}
	var task db.TaskTelemetry
	err = m.db.WithContext(ctx).
		Where("LOWER(source) = ? AND task_id = ? AND UPPER(project_key) IN ?", "jira", key, upper(policy.AllowedProjects)).
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IssueDetail{}, openaccess.NewError("not_found", "issue was not found")
	}
	if err != nil {
		return IssueDetail{}, fmt.Errorf("load Jira issue: %w", err)
	}
	response := IssueDetail{Issue: projectIssue(task, fields)}
	var refs []string
	refs = append(refs, issueSourceRef(task))
	dataAsOf := task.SourceUpdatedAt
	if dataAsOf.IsZero() {
		dataAsOf = task.LastUpdate
	}
	if request.IncludeHistory {
		historyFields := normalizeList(policy.FieldRules["jira_history"], false)
		if len(historyFields) == 0 {
			return IssueDetail{}, openaccess.NewError("forbidden", "Jira history fields are not published")
		}
		limit := request.HistoryLimit
		if limit <= 0 {
			limit = min(100, policy.QueryLimits.MaxPageSize)
		}
		if limit > policy.QueryLimits.MaxPageSize {
			limit = policy.QueryLimits.MaxPageSize
		}
		scope := struct {
			PolicyVersion int
			Key           string
		}{policy.Version, key}
		codec, codecErr := readmodel.NewCursorCodec("open-jira-history/v1", scope)
		if codecErr != nil {
			return IssueDetail{}, codecErr
		}
		position := historyPosition{}
		if request.HistoryCursor != "" {
			if _, decodeErr := codec.Decode(request.HistoryCursor, &position); decodeErr != nil {
				return IssueDetail{}, openaccess.NewError("invalid_query", "history cursor is invalid")
			}
		}
		query := m.db.WithContext(ctx).Where("task_id = ? AND LOWER(field) IN ?", key, historyFields)
		if position.ID > 0 {
			query = query.Where(
				"(occurred_at < ?) OR (occurred_at = ? AND id < ?)",
				position.OccurredAt, position.OccurredAt, position.ID,
			)
		}
		var changes []db.JiraReportChange
		if err := query.Order("occurred_at DESC, id DESC").Limit(limit + 1).Find(&changes).Error; err != nil {
			return IssueDetail{}, fmt.Errorf("load Jira issue history: %w", err)
		}
		response.HasMore = len(changes) > limit
		if response.HasMore {
			changes = changes[:limit]
		}
		for _, change := range changes {
			response.History = append(response.History, HistoryItem{
				ID: change.ID, HistoryID: change.HistoryID, Field: change.Field,
				FromValue: change.FromValue, ToValue: change.ToValue, OccurredAt: change.OccurredAt,
			})
			refs = append(refs, fmt.Sprintf("jira-change:%s:%s:%d", key, change.HistoryID, change.ItemIndex))
		}
		if response.HasMore && len(changes) > 0 {
			last := changes[len(changes)-1]
			response.NextCursor, err = codec.Encode(
				dataAsOf.UTC().Format(time.RFC3339Nano),
				historyPosition{OccurredAt: last.OccurredAt, ID: last.ID},
			)
			if err != nil {
				return IssueDetail{}, err
			}
		}
	}
	complete := int64(0)
	if task.JiraHistoryComplete {
		complete = 1
	}
	response.Meta = buildMeta(requestID, policy, dataAsOf, complete, 1, request, refs)
	return response, nil
}

func (m *Module) Aggregate(ctx context.Context, policy openaccess.Policy, requestID string, request AggregateRequest) (AggregateResponse, error) {
	if !policy.AllowsAction("jira.read") {
		return AggregateResponse{}, openaccess.NewError("forbidden", "Jira reading is not allowed by the active policy")
	}
	projects, err := authorizedProjects(policy, request.Filters.Projects)
	if err != nil {
		return AggregateResponse{}, err
	}
	metrics, err := normalizeMetrics(request.Metrics)
	if err != nil {
		return AggregateResponse{}, err
	}
	groups, err := normalizeGroups(request.GroupBy, policy.QueryLimits.MaxGroupBy)
	if err != nil {
		return AggregateResponse{}, err
	}
	requiredFields := append(filterFields(request.Filters), groups...)
	for _, metric := range metrics {
		requiredFields = append(requiredFields, metricFields(metric)...)
	}
	if !policy.AllowsFields("jira_aggregate_issues", normalizeList(requiredFields, false)) {
		return AggregateResponse{}, openaccess.NewError("forbidden", "one or more aggregate fields are not published")
	}
	hasHistorical, hasCurrent := false, false
	for _, metric := range metrics {
		if isHistoricalMetric(metric) {
			hasHistorical = true
		} else {
			hasCurrent = true
		}
	}
	if hasHistorical && hasCurrent {
		return AggregateResponse{}, openaccess.NewError("invalid_query", "current and historical metrics must be queried separately")
	}
	if hasHistorical {
		return m.aggregateHistorical(ctx, policy, requestID, request, projects, metrics, groups)
	}
	var tasks []db.TaskTelemetry
	if err := m.filteredQuery(ctx, projects, request.Filters).
		Order("task_id ASC").
		Limit(policy.QueryLimits.MaxScanRows + 1).
		Find(&tasks).Error; err != nil {
		return AggregateResponse{}, fmt.Errorf("aggregate Jira issues: %w", err)
	}
	if len(tasks) > policy.QueryLimits.MaxScanRows {
		return AggregateResponse{}, openaccess.NewError("invalid_query", "query exceeds the configured aggregation budget")
	}
	now := m.now().UTC()
	type aggregate struct {
		groups  map[string]string
		metrics map[string]float64
	}
	buckets := map[string]*aggregate{}
	complete := int64(0)
	dataAsOf := time.Time{}
	for _, task := range tasks {
		if task.JiraHistoryComplete {
			complete++
		}
		if task.SourceUpdatedAt.After(dataAsOf) {
			dataAsOf = task.SourceUpdatedAt
		}
		groupValues := make(map[string]string, len(groups))
		keyParts := make([]string, 0, len(groups))
		for _, group := range groups {
			value := groupValue(task, group)
			groupValues[group] = value
			keyParts = append(keyParts, group+"="+value)
		}
		key := strings.Join(keyParts, "\x00")
		bucket := buckets[key]
		if bucket == nil {
			bucket = &aggregate{groups: groupValues, metrics: make(map[string]float64)}
			for _, metric := range metrics {
				bucket.metrics[metric] = 0
			}
			buckets[key] = bucket
		}
		for _, metric := range metrics {
			switch metric {
			case "issue_count":
				bucket.metrics[metric]++
			case "unassigned_count":
				if isUnassigned(task.Assignee) {
					bucket.metrics[metric]++
				}
			case "overdue_count":
				if isOverdue(task, now) {
					bucket.metrics[metric]++
				}
			case "history_complete_count":
				if task.JiraHistoryComplete {
					bucket.metrics[metric]++
				}
			case "history_incomplete_count":
				if !task.JiraHistoryComplete {
					bucket.metrics[metric]++
				}
			}
		}
	}
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]AggregateRow, 0, len(keys))
	for _, key := range keys {
		bucket := buckets[key]
		rows = append(rows, AggregateRow{Groups: bucket.groups, Metrics: bucket.metrics})
	}
	if dataAsOf.IsZero() {
		dataAsOf, err = m.dataAsOf(ctx, projects)
		if err != nil {
			return AggregateResponse{}, err
		}
	}
	meta := buildMeta(requestID, policy, dataAsOf, complete, int64(len(tasks)), request, nil)
	metricCompleteness := make(map[string]string, len(metrics))
	for _, metric := range metrics {
		metricCompleteness[metric] = "complete"
	}
	return AggregateResponse{
		Rows: rows, MetricCompleteness: metricCompleteness, TimeZone: "UTC", Meta: meta,
	}, nil
}

type historyAggregateBucket struct {
	groups  map[string]string
	metrics map[string]float64
}

func (m *Module) aggregateHistorical(
	ctx context.Context,
	policy openaccess.Policy,
	requestID string,
	request AggregateRequest,
	projects, metrics, groups []string,
) (AggregateResponse, error) {
	if request.EventFrom == nil || request.EventUntil == nil {
		return AggregateResponse{}, openaccess.NewError("invalid_query", "historical metrics require event_from and event_until")
	}
	from, until := request.EventFrom.UTC(), request.EventUntil.UTC()
	if !from.Before(until) {
		return AggregateResponse{}, openaccess.NewError("invalid_query", "event_from must be before event_until")
	}
	if until.Sub(from) > 366*24*time.Hour {
		return AggregateResponse{}, openaccess.NewError("invalid_query", "historical metric interval exceeds 366 days")
	}
	timeBucket := strings.ToLower(strings.TrimSpace(request.TimeBucket))
	if timeBucket != "" && timeBucket != "day" && timeBucket != "week" {
		return AggregateResponse{}, openaccess.NewError("invalid_query", "time_bucket must be day or week")
	}
	if contains(metrics, "status_duration_hours") && !contains(groups, "status") {
		return AggregateResponse{}, openaccess.NewError("invalid_query", "status_duration_hours requires group_by=status")
	}
	var tasks []db.TaskTelemetry
	if err := m.filteredQuery(ctx, projects, request.Filters).
		Order("task_id ASC").
		Limit(policy.QueryLimits.MaxScanRows + 1).
		Find(&tasks).Error; err != nil {
		return AggregateResponse{}, fmt.Errorf("load Jira historical metric scope: %w", err)
	}
	if len(tasks) > policy.QueryLimits.MaxScanRows {
		return AggregateResponse{}, openaccess.NewError("invalid_query", "historical query exceeds the configured issue budget")
	}
	taskByID := make(map[string]db.TaskTelemetry, len(tasks))
	taskIDs := make([]string, 0, len(tasks))
	complete := int64(0)
	dataAsOf := time.Time{}
	for _, task := range tasks {
		taskByID[task.TaskID] = task
		taskIDs = append(taskIDs, task.TaskID)
		if task.JiraHistoryComplete {
			complete++
		}
		if task.SourceUpdatedAt.After(dataAsOf) {
			dataAsOf = task.SourceUpdatedAt
		}
	}
	buckets := map[string]*historyAggregateBucket{}
	distinct := map[string]map[string]map[string]struct{}{}
	ensureBucket := func(groupValues map[string]string) *historyAggregateBucket {
		key := aggregateGroupKey(groupValues)
		bucket := buckets[key]
		if bucket == nil {
			bucket = &historyAggregateBucket{
				groups: groupValues, metrics: make(map[string]float64, len(metrics)),
			}
			for _, metric := range metrics {
				bucket.metrics[metric] = 0
			}
			buckets[key] = bucket
		}
		return bucket
	}
	addDistinct := func(groupValues map[string]string, metric, taskID string) {
		key := aggregateGroupKey(groupValues)
		if distinct[key] == nil {
			distinct[key] = map[string]map[string]struct{}{}
		}
		if distinct[key][metric] == nil {
			distinct[key][metric] = map[string]struct{}{}
		}
		distinct[key][metric][taskID] = struct{}{}
	}
	if len(groups) == 0 && timeBucket == "" {
		ensureBucket(map[string]string{})
	}

	if contains(metrics, "created_issue_count") {
		for _, task := range tasks {
			if task.TaskCreatedAt.Before(from) || !task.TaskCreatedAt.Before(until) {
				continue
			}
			groupValues := historicalGroups(task, nil, groups)
			addTimeBucket(groupValues, task.TaskCreatedAt, timeBucket)
			ensureBucket(groupValues).metrics["created_issue_count"]++
		}
	}

	historyMetrics := containsAny(metrics,
		"resolved_event_count", "resolved_issue_count", "reopened_event_count",
		"assignee_change_event_count", "assignee_change_issue_count",
	)
	sourceRefs := []string{}
	if historyMetrics && len(taskIDs) > 0 {
		var changes []db.JiraReportChange
		if err := m.db.WithContext(ctx).
			Where(
				"task_id IN ? AND occurred_at >= ? AND occurred_at < ? AND LOWER(field) IN ?",
				taskIDs, from, until, []string{"status", "assignee"},
			).
			Order("occurred_at ASC, id ASC").
			Limit(policy.QueryLimits.MaxScanRows + 1).
			Find(&changes).Error; err != nil {
			return AggregateResponse{}, fmt.Errorf("load Jira historical events: %w", err)
		}
		if len(changes) > policy.QueryLimits.MaxScanRows {
			return AggregateResponse{}, openaccess.NewError("invalid_query", "historical query exceeds the configured event budget")
		}
		for _, change := range changes {
			task, ok := taskByID[change.TaskID]
			if !ok {
				continue
			}
			groupValues := historicalGroups(task, &change, groups)
			addTimeBucket(groupValues, change.OccurredAt, timeBucket)
			bucket := ensureBucket(groupValues)
			field := strings.ToLower(strings.TrimSpace(change.Field))
			switch field {
			case "status":
				if isResolvedStatus(change.ToValue) {
					if contains(metrics, "resolved_event_count") {
						bucket.metrics["resolved_event_count"]++
					}
					if contains(metrics, "resolved_issue_count") {
						addDistinct(groupValues, "resolved_issue_count", change.TaskID)
					}
				}
				if isResolvedStatus(change.FromValue) && !isResolvedStatus(change.ToValue) &&
					contains(metrics, "reopened_event_count") {
					bucket.metrics["reopened_event_count"]++
				}
			case "assignee":
				if contains(metrics, "assignee_change_event_count") {
					bucket.metrics["assignee_change_event_count"]++
				}
				if contains(metrics, "assignee_change_issue_count") {
					addDistinct(groupValues, "assignee_change_issue_count", change.TaskID)
				}
			}
			if len(sourceRefs) < 500 {
				sourceRefs = append(sourceRefs, fmt.Sprintf(
					"jira-change:%s:%s:%d", change.TaskID, change.HistoryID, change.ItemIndex,
				))
			}
		}
	}

	if contains(metrics, "status_duration_hours") && len(taskIDs) > 0 {
		if err := m.addStatusDurations(
			ctx, tasks, taskIDs, groups, timeBucket, from, until,
			policy.QueryLimits.MaxScanRows, ensureBucket,
		); err != nil {
			return AggregateResponse{}, err
		}
	}
	for key, metricSets := range distinct {
		bucket := buckets[key]
		for metric, issueSet := range metricSets {
			bucket.metrics[metric] = float64(len(issueSet))
		}
	}
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]AggregateRow, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, AggregateRow{Groups: buckets[key].groups, Metrics: buckets[key].metrics})
	}
	historyDependent := false
	metricCompleteness := make(map[string]string, len(metrics))
	for _, metric := range metrics {
		if metric == "created_issue_count" {
			metricCompleteness[metric] = "complete"
			continue
		}
		historyDependent = true
		if complete == int64(len(tasks)) {
			metricCompleteness[metric] = "complete"
		} else {
			metricCompleteness[metric] = "partial"
		}
	}
	metaComplete := int64(len(tasks))
	if historyDependent {
		metaComplete = complete
	}
	if dataAsOf.IsZero() {
		dataAsOf = m.now().UTC()
	}
	meta := buildMeta(requestID, policy, dataAsOf, metaComplete, int64(len(tasks)), request, sourceRefs)
	return AggregateResponse{
		Rows: rows, MetricCompleteness: metricCompleteness, TimeZone: "UTC", Meta: meta,
	}, nil
}

func (m *Module) addStatusDurations(
	ctx context.Context,
	tasks []db.TaskTelemetry,
	taskIDs, groups []string,
	timeBucket string,
	from, until time.Time,
	maxRows int,
	ensureBucket func(map[string]string) *historyAggregateBucket,
) error {
	var changes []db.JiraReportChange
	if err := m.db.WithContext(ctx).
		Where("task_id IN ? AND occurred_at < ? AND LOWER(field) = ?", taskIDs, until, "status").
		Order("task_id ASC, occurred_at ASC, id ASC").
		Limit(maxRows + 1).
		Find(&changes).Error; err != nil {
		return fmt.Errorf("load Jira status durations: %w", err)
	}
	if len(changes) > maxRows {
		return openaccess.NewError("invalid_query", "status duration query exceeds the configured history budget")
	}
	byTask := map[string][]db.JiraReportChange{}
	for _, change := range changes {
		byTask[change.TaskID] = append(byTask[change.TaskID], change)
	}
	for _, task := range tasks {
		if !task.JiraHistoryComplete {
			continue
		}
		statusChanges := byTask[task.TaskID]
		if len(statusChanges) == 0 {
			start := maxTime(task.TaskCreatedAt, from)
			m.addStatusDurationSegment(task, task.Status, groups, timeBucket, start, until, ensureBucket)
			continue
		}
		status := statusChanges[0].FromValue
		cursor := task.TaskCreatedAt
		if cursor.IsZero() {
			cursor = statusChanges[0].OccurredAt
		}
		for _, change := range statusChanges {
			m.addStatusDurationSegment(task, status, groups, timeBucket, maxTime(cursor, from), minTime(change.OccurredAt, until), ensureBucket)
			status = change.ToValue
			cursor = change.OccurredAt
		}
		m.addStatusDurationSegment(task, status, groups, timeBucket, maxTime(cursor, from), until, ensureBucket)
	}
	return nil
}

func (m *Module) addStatusDurationSegment(
	task db.TaskTelemetry,
	status string,
	groups []string,
	timeBucket string,
	start, end time.Time,
	ensureBucket func(map[string]string) *historyAggregateBucket,
) {
	if !start.Before(end) {
		return
	}
	for start.Before(end) {
		segmentEnd := end
		if boundary := nextBucketBoundary(start, timeBucket); !boundary.IsZero() && boundary.Before(segmentEnd) {
			segmentEnd = boundary
		}
		groupValues := historicalGroups(task, nil, groups)
		if contains(groups, "status") {
			groupValues["status"] = emptyLabel(status)
		}
		addTimeBucket(groupValues, start, timeBucket)
		ensureBucket(groupValues).metrics["status_duration_hours"] += segmentEnd.Sub(start).Hours()
		start = segmentEnd
	}
}

func isHistoricalMetric(metric string) bool {
	switch metric {
	case "created_issue_count", "resolved_event_count", "resolved_issue_count",
		"reopened_event_count", "assignee_change_event_count",
		"assignee_change_issue_count", "status_duration_hours":
		return true
	default:
		return false
	}
}

func historicalGroups(task db.TaskTelemetry, change *db.JiraReportChange, groups []string) map[string]string {
	values := make(map[string]string, len(groups)+1)
	for _, group := range groups {
		value := groupValue(task, group)
		if change != nil {
			field := strings.ToLower(strings.TrimSpace(change.Field))
			if group == "status" && field == "status" {
				value = emptyLabel(change.ToValue)
			}
			if group == "assignee" && field == "assignee" {
				value = emptyLabel(change.ToValue)
			}
		}
		values[group] = value
	}
	return values
}

func addTimeBucket(groups map[string]string, at time.Time, mode string) {
	if mode == "" {
		return
	}
	groups["time_bucket"] = bucketStart(at.UTC(), mode).Format("2006-01-02")
}

func bucketStart(at time.Time, mode string) time.Time {
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	if mode == "week" {
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	}
	return day
}

func nextBucketBoundary(at time.Time, mode string) time.Time {
	switch mode {
	case "day":
		return bucketStart(at, mode).AddDate(0, 0, 1)
	case "week":
		return bucketStart(at, mode).AddDate(0, 0, 7)
	default:
		return time.Time{}
	}
}

func aggregateGroupKey(groups map[string]string) string {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+groups[key])
	}
	return strings.Join(parts, "\x00")
}

func isResolvedStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "done", "closed", "resolved", "completed", "archived", "已完成", "已关闭":
		return true
	default:
		return false
	}
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func containsAny(values []string, candidates ...string) bool {
	for _, candidate := range candidates {
		if contains(values, candidate) {
			return true
		}
	}
	return false
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

type searchPosition struct {
	SnapshotID string `json:"snapshot_id"`
	Offset     int    `json:"offset"`
}

type searchSnapshotPayload struct {
	Items      []map[string]any `json:"items"`
	SourceRefs []string         `json:"source_refs"`
	Complete   int64            `json:"complete"`
	Total      int64            `json:"total"`
}

type historyPosition struct {
	OccurredAt time.Time `json:"occurred_at"`
	ID         uint      `json:"id"`
}

func (m *Module) filteredQuery(ctx context.Context, projects []string, filters Filters) *gorm.DB {
	query := m.db.WithContext(ctx).Model(&db.TaskTelemetry{}).
		Where("LOWER(source) = ? AND UPPER(project_key) IN ?", "jira", upper(projects))
	if values := normalizeList(filters.IssueTypes, false); len(values) > 0 {
		query = query.Where("LOWER(issue_type) IN ?", lower(values))
	}
	if values := normalizeList(filters.Statuses, false); len(values) > 0 {
		query = query.Where("LOWER(status) IN ?", lower(values))
	}
	if values := normalizeList(filters.Assignees, false); len(values) > 0 {
		query = query.Where("LOWER(assignee) IN ?", lower(values))
	}
	if values := normalizeList(filters.Priorities, false); len(values) > 0 {
		query = query.Where("LOWER(priority) IN ?", lower(values))
	}
	if values := normalizeList(filters.BugCategories, false); len(values) > 0 {
		query = query.Where("LOWER(jira_bug_category) IN ?", lower(values))
	}
	if filters.UpdatedFrom != nil {
		query = query.Where("source_updated_at >= ?", filters.UpdatedFrom.UTC())
	}
	if filters.UpdatedUntil != nil {
		query = query.Where("source_updated_at < ?", filters.UpdatedUntil.UTC())
	}
	if filters.DueBefore != nil {
		query = query.Where("due_date < ?", filters.DueBefore.UTC())
	}
	if filters.Unassigned != nil {
		unassigned := "(assignee IS NULL OR TRIM(assignee) = '' OR LOWER(TRIM(assignee)) IN ('unassigned','未指派','-'))"
		if *filters.Unassigned {
			query = query.Where(unassigned)
		} else {
			query = query.Where("NOT " + unassigned)
		}
	}
	if filters.Overdue != nil {
		clause := "due_date IS NOT NULL AND due_date < ? AND LOWER(TRIM(status)) NOT IN ?"
		done := []string{"done", "closed", "resolved", "completed", "archived", "已完成", "已关闭"}
		if *filters.Overdue {
			query = query.Where(clause, m.now().UTC(), done)
		} else {
			query = query.Where("NOT ("+clause+")", m.now().UTC(), done)
		}
	}
	return query
}

func (m *Module) dataAsOf(ctx context.Context, projects []string) (time.Time, error) {
	var task db.TaskTelemetry
	err := m.db.WithContext(ctx).
		Select("source_updated_at").
		Where("LOWER(source) = ? AND UPPER(project_key) IN ?", "jira", upper(projects)).
		Order("source_updated_at DESC").
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return m.now().UTC(), nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("read Jira data watermark: %w", err)
	}
	if task.SourceUpdatedAt.IsZero() {
		return m.now().UTC(), nil
	}
	return task.SourceUpdatedAt.UTC(), nil
}

func (m *Module) completenessForQuery(query *gorm.DB) (int64, int64, error) {
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, 0, fmt.Errorf("count Jira query: %w", err)
	}
	var complete int64
	if err := query.Where("jira_history_complete = ?", true).Count(&complete).Error; err != nil {
		return 0, 0, fmt.Errorf("count complete Jira history: %w", err)
	}
	return complete, total, nil
}

func completeness(complete, total int64) float64 {
	if total <= 0 {
		return 1
	}
	return float64(complete) / float64(total)
}

func digestValue(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func authorizedProjects(policy openaccess.Policy, requested []string) ([]string, error) {
	if !policy.JiraCachePublicationVerified() {
		return nil, openaccess.NewError("forbidden", "Jira cache publication has not been verified")
	}
	if len(policy.AllowedProjects) == 0 {
		return nil, openaccess.NewError("forbidden", "Jira query has no published project scope")
	}
	if len(requested) == 0 {
		return upper(policy.AllowedProjects), nil
	}
	result := normalizeList(requested, true)
	for _, project := range result {
		if !policy.AllowsProject(project) {
			return nil, openaccess.NewError("forbidden", "requested Jira scope is not published")
		}
	}
	return result, nil
}

func publishedFields(policy openaccess.Policy, capability string, requested []string, extra ...string) ([]string, error) {
	fields := normalizeList(requested, false)
	if len(fields) == 0 {
		fields = append([]string(nil), defaultFields...)
	}
	required := append(append([]string(nil), fields...), extra...)
	if !policy.AllowsFields(capability, normalizeList(required, false)) {
		return nil, openaccess.NewError("forbidden", "one or more requested fields are not published")
	}
	return fields, nil
}

func filterFields(filters Filters) []string {
	fields := []string{}
	if len(filters.Projects) > 0 {
		fields = append(fields, "project")
	}
	if len(filters.IssueTypes) > 0 {
		fields = append(fields, "issue_type")
	}
	if len(filters.Statuses) > 0 {
		fields = append(fields, "status")
	}
	if len(filters.Assignees) > 0 || filters.Unassigned != nil {
		fields = append(fields, "assignee")
	}
	if len(filters.Priorities) > 0 {
		fields = append(fields, "priority")
	}
	if len(filters.BugCategories) > 0 {
		fields = append(fields, "bug_category")
	}
	if filters.UpdatedFrom != nil || filters.UpdatedUntil != nil {
		fields = append(fields, "updated_at")
	}
	if filters.DueBefore != nil {
		fields = append(fields, "due_date")
	}
	if filters.Overdue != nil {
		fields = append(fields, "due_date", "status")
	}
	return normalizeList(fields, false)
}

func metricFields(metric string) []string {
	switch metric {
	case "issue_count":
		return []string{"key"}
	case "unassigned_count":
		return []string{"assignee"}
	case "overdue_count":
		return []string{"due_date", "status"}
	case "history_complete_count", "history_incomplete_count":
		return []string{"history_complete"}
	case "created_issue_count":
		return []string{"key", "created_at"}
	case "resolved_event_count", "resolved_issue_count", "reopened_event_count", "status_duration_hours":
		return []string{"key", "history", "status"}
	case "assignee_change_event_count", "assignee_change_issue_count":
		return []string{"key", "history", "assignee"}
	default:
		return nil
	}
}

func normalizeMetrics(metrics []string) ([]string, error) {
	if len(metrics) == 0 {
		metrics = []string{"issue_count"}
	}
	allowed := map[string]bool{
		"issue_count": true, "unassigned_count": true, "overdue_count": true,
		"history_complete_count": true, "history_incomplete_count": true,
		"created_issue_count": true, "resolved_event_count": true, "resolved_issue_count": true,
		"reopened_event_count": true, "assignee_change_event_count": true,
		"assignee_change_issue_count": true, "status_duration_hours": true,
	}
	result := normalizeList(metrics, false)
	for _, metric := range result {
		if !allowed[metric] {
			return nil, openaccess.NewError("invalid_query", "unsupported Jira metric: "+metric)
		}
	}
	return result, nil
}

func normalizeGroups(groups []string, max int) ([]string, error) {
	result := normalizeList(groups, false)
	if len(result) > max {
		return nil, openaccess.NewError("invalid_query", "too many Jira grouping dimensions")
	}
	allowed := map[string]bool{
		"project": true, "issue_type": true, "status": true,
		"assignee": true, "priority": true, "bug_category": true,
	}
	for _, group := range result {
		if !allowed[group] {
			return nil, openaccess.NewError("invalid_query", "unsupported Jira grouping dimension: "+group)
		}
	}
	return result, nil
}

func normalizeFilters(filters Filters) Filters {
	filters.Projects = normalizeList(filters.Projects, true)
	filters.IssueTypes = normalizeList(filters.IssueTypes, false)
	filters.Statuses = normalizeList(filters.Statuses, false)
	filters.Assignees = normalizeList(filters.Assignees, false)
	filters.Priorities = normalizeList(filters.Priorities, false)
	filters.BugCategories = normalizeList(filters.BugCategories, false)
	return filters
}

func normalizeList(values []string, uppercase bool) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if uppercase {
			value = strings.ToUpper(value)
		} else {
			value = strings.ToLower(value)
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func upper(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strings.ToUpper(strings.TrimSpace(value)))
	}
	return result
}

func lower(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strings.ToLower(strings.TrimSpace(value)))
	}
	return result
}

func projectIssue(task db.TaskTelemetry, fields []string) map[string]any {
	item := make(map[string]any, len(fields))
	for _, field := range fields {
		switch field {
		case "key":
			item[field] = task.TaskID
		case "project":
			item[field] = task.ProjectKey
		case "summary":
			item[field] = task.Title
		case "description":
			item[field] = task.Description
		case "issue_type":
			item[field] = task.IssueType
		case "status":
			item[field] = task.Status
		case "assignee":
			item[field] = task.Assignee
		case "reporter":
			item[field] = task.JiraReporter
		case "priority":
			item[field] = task.Priority
		case "severity":
			item[field] = task.Severity
		case "bug_category":
			item[field] = task.JiraBugCategory
		case "due_date":
			item[field] = task.DueDate
		case "created_at":
			item[field] = task.TaskCreatedAt
		case "updated_at":
			item[field] = task.SourceUpdatedAt
		case "history_complete":
			item[field] = task.JiraHistoryComplete
		case "parent_key":
			item[field] = task.ParentWorkItemID
		}
	}
	return item
}

func groupValue(task db.TaskTelemetry, group string) string {
	switch group {
	case "project":
		return task.ProjectKey
	case "issue_type":
		return emptyLabel(task.IssueType)
	case "status":
		return emptyLabel(task.Status)
	case "assignee":
		return emptyLabel(task.Assignee)
	case "priority":
		return emptyLabel(task.Priority)
	case "bug_category":
		return emptyLabel(task.JiraBugCategory)
	default:
		return ""
	}
}

func emptyLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(missing)"
	}
	return strings.TrimSpace(value)
}

func isUnassigned(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || value == "unassigned" || value == "未指派" || value == "-"
}

func isOverdue(task db.TaskTelemetry, now time.Time) bool {
	if task.DueDate == nil || !task.DueDate.Before(now) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(task.Status)) {
	case "done", "closed", "resolved", "completed", "archived", "已完成", "已关闭":
		return false
	default:
		return true
	}
}

func issueSourceRef(task db.TaskTelemetry) string {
	value := task.SourceUpdatedAt
	if value.IsZero() {
		value = task.LastUpdate
	}
	return fmt.Sprintf("jira-issue:%s@%s", task.TaskID, value.UTC().Format(time.RFC3339Nano))
}

func buildMeta(
	requestID string,
	policy openaccess.Policy,
	dataAsOf time.Time,
	complete, total int64,
	query any,
	refs []string,
) SnapshotMeta {
	completeness := 1.0
	missing := []string{}
	if total > 0 {
		completeness = float64(complete) / float64(total)
	}
	if completeness < 1 {
		missing = append(missing, "one or more issues have incomplete Jira changelog history")
	}
	payload, _ := json.Marshal(struct {
		PolicyVersion int
		DataAsOf      time.Time
		Query         any
	}{policy.Version, dataAsOf.UTC(), query})
	sum := sha256.Sum256(payload)
	if refs == nil {
		refs = []string{}
	}
	return SnapshotMeta{
		RequestID: requestID, SchemaVersion: SchemaVersion, PolicyVersion: policy.Version,
		MetricDefinitionVersion: MetricDefinitionVersion, DataAsOf: dataAsOf.UTC(),
		QuerySnapshotRef: "jira-query:" + hex.EncodeToString(sum[:]),
		Completeness:     completeness, MissingReasons: missing, SourceRefs: refs,
	}
}
