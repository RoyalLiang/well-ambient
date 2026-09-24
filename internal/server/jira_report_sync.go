package server

import (
	"fmt"
	"strings"

	"gorm.io/gorm/clause"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/telemetry"
)

func applyJiraReportFields(task *db.TaskTelemetry, issue telemetry.JiraIssue) bool {
	complete := issue.HistoryAvailable && issue.Changelog.StartAt == 0 && len(issue.Changelog.Histories) >= issue.Changelog.Total
	category, fieldID := issue.BugCategory, issue.BugCategoryFieldID
	if !issue.BugCategoryAvailable {
		category, fieldID = "", ""
	}
	changed := task.JiraBugCategory != category || task.JiraBugCategoryFieldID != fieldID || task.JiraHistoryComplete != complete
	task.JiraBugCategory, task.JiraBugCategoryFieldID, task.JiraHistoryComplete = category, fieldID, complete
	return changed
}

func persistJiraReportChanges(issue telemetry.JiraIssue) error {
	var rows []db.JiraReportChange
	for _, history := range issue.Changelog.Histories {
		at := parseOptionalJiraTime(history.Created)
		if at.IsZero() {
			return fmt.Errorf("Jira history %s has no valid source timestamp", history.ID)
		}
		for i, item := range history.Items {
			id := strings.TrimSpace(history.ID)
			if id == "" {
				return fmt.Errorf("Jira history has no source id")
			}
			field := strings.ToLower(strings.TrimSpace(item.FieldID))
			if field == "" {
				field = strings.ToLower(strings.TrimSpace(item.Field))
			}
			rows = append(rows, db.JiraReportChange{TaskID: issue.Key, HistoryID: id, ItemIndex: i, Field: field, FromValue: item.FromString, ToValue: item.ToString, FromID: item.From, ToID: item.To, OccurredAt: at})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return db.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&rows, 200).Error
}

// Supplement the active-work scopes: status/current-assignee filters must not
// hide recent Jira issues transferred away from a core member.
func buildEmailHistoricalJiraScope(cfg config.Config) string {
	settings := cfg.DailyJiraEmail.Normalized()
	if !settings.Enabled && len(settings.ProjectGroups) == 0 {
		return ""
	}
	members := (&Server{config: &cfg}).configuredKPICoreMembers()
	members = normalizeJIRAScopeValues(members, false)
	if len(members) == 0 {
		return ""
	}
	projects := config.JiraProjectKeys(&cfg.Jira)
	projects = append(projects, extractJIRAProjects(cfg.Jira.CustomJQL)...)
	for _, group := range settings.ProjectGroups {
		projects = append(projects, group.Projects...)
	}
	projects = normalizeJIRAScopeValues(projects, true)
	parts := []string{fmt.Sprintf("assignee WAS IN (%s)", quoteJIRAValues(members)), "(updated >= -8d OR created >= -4d)"}
	// One extra day covers report timezone versus Jira server timezone boundaries.
	if len(projects) > 0 {
		parts = append(parts, fmt.Sprintf("project IN (%s)", quoteJIRAValues(projects)))
	}
	return strings.Join(parts, " AND ")
}
