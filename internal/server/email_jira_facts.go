package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"well-ambient/internal/db"
)

type emailJiraFacts struct {
	Owners  map[string][]string
	Updates map[string][]string
}

func appendEmailFact(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// Only source timestamps establish a Jira operation. Ingestion timestamps and
// issue snapshots cannot distinguish a comment from a status change.
func loadEmailJiraFacts(ctx context.Context, tasks []db.TaskTelemetry, start, end time.Time) (emailJiraFacts, error) {
	facts := emailJiraFacts{Owners: map[string][]string{}, Updates: map[string][]string{}}
	hasEvents := db.DB.Migrator().HasTable(&db.PerformanceWorkItemEvent{})
	hasReportChanges := db.DB.Migrator().HasTable(&db.JiraReportChange{})
	hasComments := db.DB.Migrator().HasTable(&db.JiraCommentLog{})
	for offset := 0; offset < len(tasks); offset += 400 {
		limit := offset + 400
		if limit > len(tasks) {
			limit = len(tasks)
		}
		keys := []string{}
		for _, task := range tasks[offset:limit] {
			keys = append(keys, task.TaskID)
			facts.Owners[task.TaskID] = appendEmailFact(facts.Owners[task.TaskID], task.Assignee)
		}
		if hasReportChanges {
			var changes []db.JiraReportChange
			if err := db.DB.WithContext(ctx).Where("task_id IN ?", keys).Order("occurred_at, id").Limit(50001).Find(&changes).Error; err != nil {
				return facts, fmt.Errorf("cannot load Jira report history: %w", err)
			}
			if len(changes) > 50000 {
				return facts, fmt.Errorf("too many Jira report history items")
			}
			for _, change := range changes {
				if !change.OccurredAt.Before(end) {
					continue
				}
				if change.Field == "assignee" {
					for _, value := range []string{change.FromID, change.ToID, change.FromValue, change.ToValue} {
						facts.Owners[change.TaskID] = appendEmailFact(facts.Owners[change.TaskID], value)
					}
				}
				if change.OccurredAt.Before(start) || change.Field == "" {
					continue
				}
				facts.Updates[change.TaskID] = appendEmailFact(facts.Updates[change.TaskID], emailJiraChangeLabel(change.Field, change.FromValue, change.ToValue))
			}
		}
		if hasEvents {
			var events []db.PerformanceWorkItemEvent
			if err := db.DB.WithContext(ctx).Select("work_item_id", "field_name", "from_value", "to_value", "occurred_at").Where("work_item_id IN ? AND source_system = ?", keys, "jira").Order("occurred_at, id").Limit(50001).Find(&events).Error; err != nil {
				return facts, fmt.Errorf("cannot load Jira change facts: %w", err)
			}
			if len(events) > 50000 {
				return facts, fmt.Errorf("too many Jira change facts")
			}
			for _, event := range events {
				if !event.OccurredAt.Before(end) {
					continue
				}
				if event.FieldName == "assignee" {
					facts.Owners[event.WorkItemID] = appendEmailFact(facts.Owners[event.WorkItemID], event.FromValue)
					facts.Owners[event.WorkItemID] = appendEmailFact(facts.Owners[event.WorkItemID], event.ToValue)
				}
				if event.OccurredAt.Before(start) || event.FieldName == "" {
					continue
				}
				label := emailJiraChangeLabel(event.FieldName, event.FromValue, event.ToValue)
				facts.Updates[event.WorkItemID] = appendEmailFact(facts.Updates[event.WorkItemID], label)
			}
		}
		if hasComments {
			var comments []db.JiraCommentLog
			if err := db.DB.WithContext(ctx).Select("task_id", "created_at", "source_updated_at").Where("task_id IN ?", keys).Limit(50001).Find(&comments).Error; err != nil {
				return facts, fmt.Errorf("cannot load Jira comment facts: %w", err)
			}
			if len(comments) > 50000 {
				return facts, fmt.Errorf("too many Jira comment facts")
			}
			for _, c := range comments {
				if !c.CreatedAt.Before(start) && c.CreatedAt.Before(end) {
					facts.Updates[c.TaskID] = appendEmailFact(facts.Updates[c.TaskID], "添加评论")
				}
				if c.SourceUpdatedAt != nil && c.SourceUpdatedAt.After(c.CreatedAt) && !c.SourceUpdatedAt.Before(start) && c.SourceUpdatedAt.Before(end) {
					facts.Updates[c.TaskID] = appendEmailFact(facts.Updates[c.TaskID], "编辑评论")
				}
			}
		}
	}
	for key := range facts.Updates {
		sort.Strings(facts.Updates[key])
	}
	return facts, nil
}

func emailJiraChangeLabel(field, from, to string) string {
	label := map[string]string{"status": "修改状态", "assignee": "转派负责人", "priority": "修改优先级", "duedate": "修改截止日期", "summary": "修改标题", "description": "修改描述"}[field]
	if label == "" {
		label = "修改字段（" + field + "）"
	}
	// Avoid copying long descriptions or other sensitive free text into reports.
	if (field == "status" || field == "assignee" || field == "priority" || field == "duedate") && (from != "" || to != "") {
		label += "：" + from + " → " + to
	}
	return label
}
