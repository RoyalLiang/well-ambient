package deliveryplanning

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

type MigrationChange struct {
	TaskID                 string `json:"task_id"`
	IssueTypeBefore        string `json:"issue_type_before"`
	IssueTypeAfter         string `json:"issue_type_after"`
	ProjectKeyBefore       string `json:"project_key_before,omitempty"`
	ProjectKeyAfter        string `json:"project_key_after,omitempty"`
	ProjectResolution      string `json:"project_resolution,omitempty"`
	ParentWorkItemIDBefore string `json:"parent_work_item_id_before,omitempty"`
	ParentWorkItemIDAfter  string `json:"parent_work_item_id_after,omitempty"`
	ParentResolution       string `json:"parent_resolution,omitempty"`
	SourceBefore           string `json:"source_before,omitempty"`
	SourceAfter            string `json:"source_after,omitempty"`
	ExternalKeyBefore      string `json:"external_key_before,omitempty"`
	ExternalKeyAfter       string `json:"external_key_after,omitempty"`
}

type MigrationUnresolved struct {
	TaskID string `json:"task_id"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type MigrationReport struct {
	DryRun                 bool                  `json:"dry_run"`
	TotalBefore            int                   `json:"total_before"`
	TotalAfter             int                   `json:"total_after"`
	ChangeCount            int                   `json:"change_count"`
	UnresolvedProjectCount int                   `json:"unresolved_project_count"`
	OrphanExecutionCount   int                   `json:"orphan_execution_count"`
	Changes                []MigrationChange     `json:"changes"`
	Unresolved             []MigrationUnresolved `json:"unresolved"`
}

func PlanMigration(tasks []db.TaskTelemetry, projects []db.ProjectConfig) MigrationReport {
	projectKeys := make(map[string]struct{}, len(projects))
	repoProjects := make(map[string][]string)
	for _, project := range projects {
		key := NormalizeProjectKey(project.ProjectKey)
		if key == "" {
			continue
		}
		projectKeys[key] = struct{}{}
		var repositories []string
		_ = json.Unmarshal([]byte(project.GitReposJSON), &repositories)
		for _, repository := range repositories {
			repository = normalizeRepository(repository)
			if repository != "" {
				repoProjects[repository] = appendUnique(repoProjects[repository], key)
			}
		}
	}

	workItemsByGroup := make(map[string][]string)
	workItemProjects := make(map[string]string)
	for _, task := range tasks {
		kind, err := normalizeMigrationKind(task, projectKeys)
		if err != nil || kind == ExecutionTask {
			continue
		}
		projectKey, _ := resolveMigrationProject(task, projectKeys, repoProjects)
		workItemProjects[task.TaskID] = projectKey
		if group := normalizedMigrationGroup(task.TaskGroupID); group != "" {
			workItemsByGroup[group] = appendUnique(workItemsByGroup[group], task.TaskID)
		}
	}

	report := MigrationReport{DryRun: true, TotalBefore: len(tasks), TotalAfter: len(tasks)}
	for _, task := range tasks {
		change := MigrationChange{
			TaskID:                 task.TaskID,
			IssueTypeBefore:        task.IssueType,
			ProjectKeyBefore:       task.ProjectKey,
			ParentWorkItemIDBefore: task.ParentWorkItemID,
			SourceBefore:           task.Source,
			ExternalKeyBefore:      task.ExternalKey,
		}
		kind, err := normalizeMigrationKind(task, projectKeys)
		if err != nil {
			report.Unresolved = append(report.Unresolved, MigrationUnresolved{
				TaskID: task.TaskID,
				Type:   "unsupported_issue_type",
				Reason: err.Error(),
			})
			continue
		}
		change.IssueTypeAfter = kind

		projectKey, resolution := resolveMigrationProject(task, projectKeys, repoProjects)
		change.ProjectKeyAfter = projectKey
		change.ProjectResolution = resolution

		if kind == ExecutionTask {
			parentID := strings.TrimSpace(task.ParentWorkItemID)
			parentResolution := "explicit"
			if parentID == "" {
				group := normalizedMigrationGroup(task.TaskGroupID)
				candidates := workItemsByGroup[group]
				if len(candidates) == 1 {
					parentID = candidates[0]
					parentResolution = "unique_task_group"
				} else {
					parentResolution = "orphan"
					report.OrphanExecutionCount++
					reason := "execution task has no explicit parent or unique task-group work item"
					if len(candidates) > 1 {
						reason = "task group maps to multiple work items: " + strings.Join(candidates, ", ")
					}
					report.Unresolved = append(report.Unresolved, MigrationUnresolved{
						TaskID: task.TaskID,
						Type:   "parent_work_item",
						Reason: reason,
					})
				}
			}
			change.ParentWorkItemIDAfter = parentID
			change.ParentResolution = parentResolution
			if change.ProjectKeyAfter == "" && parentID != "" {
				if inherited := workItemProjects[parentID]; inherited != "" {
					change.ProjectKeyAfter = inherited
					change.ProjectResolution = "parent_work_item"
				}
			}
		}
		if change.ProjectKeyAfter == "" {
			report.UnresolvedProjectCount++
			report.Unresolved = append(report.Unresolved, MigrationUnresolved{
				TaskID: task.TaskID,
				Type:   "project",
				Reason: "no explicit project, parent work-item project, catalog ID prefix, or unique repository mapping",
			})
		}
		change.SourceAfter, change.ExternalKeyAfter = migrationIdentity(task, change.ProjectKeyAfter, kind)

		if migrationChangeNeeded(change) {
			report.Changes = append(report.Changes, change)
		}
	}
	report.ChangeCount = len(report.Changes)
	sort.Slice(report.Changes, func(i, j int) bool { return report.Changes[i].TaskID < report.Changes[j].TaskID })
	sort.Slice(report.Unresolved, func(i, j int) bool {
		if report.Unresolved[i].TaskID != report.Unresolved[j].TaskID {
			return report.Unresolved[i].TaskID < report.Unresolved[j].TaskID
		}
		return report.Unresolved[i].Type < report.Unresolved[j].Type
	})
	return report
}

func LoadMigrationReport(ctx context.Context, conn *gorm.DB) (MigrationReport, error) {
	if conn == nil {
		return MigrationReport{}, fmt.Errorf("database is not initialized")
	}
	var tasks []db.TaskTelemetry
	if err := conn.WithContext(ctx).Order("task_id ASC").Find(&tasks).Error; err != nil {
		return MigrationReport{}, err
	}
	var projects []db.ProjectConfig
	if err := conn.WithContext(ctx).Order("project_key ASC").Find(&projects).Error; err != nil {
		return MigrationReport{}, err
	}
	return PlanMigration(tasks, projects), nil
}

func ReadSQLiteMigrationReport(ctx context.Context, databasePath string) (MigrationReport, error) {
	absolutePath, err := filepath.Abs(databasePath)
	if err != nil {
		return MigrationReport{}, err
	}
	dsn := (&url.URL{Scheme: "file", Path: absolutePath, RawQuery: "mode=ro&_query_only=1"}).String()
	database, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return MigrationReport{}, err
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	if err := database.PingContext(ctx); err != nil {
		return MigrationReport{}, err
	}
	optionalColumns := []string{"project_key", "source", "external_key", "parent_work_item_id"}
	selectExpressions := make(map[string]string, len(optionalColumns))
	for _, column := range optionalColumns {
		exists, err := tableHasColumn(ctx, database, "task_telemetries", column)
		if err != nil {
			return MigrationReport{}, err
		}
		selectExpressions[column] = "''"
		if exists {
			selectExpressions[column] = "coalesce(" + column + ", '')"
		}
	}
	rows, err := database.QueryContext(ctx, fmt.Sprintf(`
		SELECT task_id, coalesce(issue_type, ''), coalesce(repo, ''),
		       coalesce(task_group_id, ''), %s, %s, %s, %s
		FROM task_telemetries
		ORDER BY task_id`,
		selectExpressions["project_key"],
		selectExpressions["source"],
		selectExpressions["external_key"],
		selectExpressions["parent_work_item_id"],
	))
	if err != nil {
		return MigrationReport{}, err
	}
	var tasks []db.TaskTelemetry
	for rows.Next() {
		var task db.TaskTelemetry
		if err := rows.Scan(
			&task.TaskID,
			&task.IssueType,
			&task.Repo,
			&task.TaskGroupID,
			&task.ProjectKey,
			&task.Source,
			&task.ExternalKey,
			&task.ParentWorkItemID,
		); err != nil {
			rows.Close()
			return MigrationReport{}, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Close(); err != nil {
		return MigrationReport{}, err
	}
	if err := rows.Err(); err != nil {
		return MigrationReport{}, err
	}

	projectRows, err := database.QueryContext(ctx, `
		SELECT coalesce(project_key, ''), coalesce(project_name, ''), coalesce(git_repos_json, '[]')
		FROM project_configs
		ORDER BY project_key`)
	if err != nil {
		return MigrationReport{}, err
	}
	var projects []db.ProjectConfig
	for projectRows.Next() {
		var project db.ProjectConfig
		if err := projectRows.Scan(&project.ProjectKey, &project.ProjectName, &project.GitReposJSON); err != nil {
			projectRows.Close()
			return MigrationReport{}, err
		}
		projects = append(projects, project)
	}
	if err := projectRows.Close(); err != nil {
		return MigrationReport{}, err
	}
	if err := projectRows.Err(); err != nil {
		return MigrationReport{}, err
	}
	return PlanMigration(tasks, projects), nil
}

func ApplyMigration(ctx context.Context, conn *gorm.DB, report MigrationReport) (MigrationReport, error) {
	if conn == nil {
		return MigrationReport{}, fmt.Errorf("database is not initialized")
	}
	err := conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, change := range report.Changes {
			updates := map[string]any{
				"issue_type":          change.IssueTypeAfter,
				"project_key":         change.ProjectKeyAfter,
				"parent_work_item_id": change.ParentWorkItemIDAfter,
				"source":              change.SourceAfter,
				"external_key":        change.ExternalKeyAfter,
			}
			if err := tx.Model(&db.TaskTelemetry{}).Where("task_id = ?", change.TaskID).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return MigrationReport{}, err
	}
	applied, err := LoadMigrationReport(ctx, conn)
	if err != nil {
		return MigrationReport{}, err
	}
	applied.DryRun = false
	applied.TotalBefore = report.TotalBefore
	if applied.TotalAfter != report.TotalBefore {
		return MigrationReport{}, fmt.Errorf("task conservation failed: before=%d after=%d", report.TotalBefore, applied.TotalAfter)
	}
	return applied, nil
}

func resolveMigrationProject(task db.TaskTelemetry, projectKeys map[string]struct{}, repoProjects map[string][]string) (string, string) {
	if explicit := NormalizeProjectKey(task.ProjectKey); explicit != "" {
		if _, exists := projectKeys[explicit]; exists {
			return explicit, "explicit"
		}
		return "", "unknown_explicit"
	}
	if prefix := projectPrefix(task.TaskID); prefix != "" {
		if _, exists := projectKeys[prefix]; exists {
			return prefix, "task_id_prefix"
		}
	}
	matches := repoProjects[normalizeRepository(task.Repo)]
	if len(matches) == 1 {
		return matches[0], "unique_repository"
	}
	if len(matches) > 1 {
		return "", "ambiguous_repository"
	}
	return "", "unresolved"
}

func migrationIdentity(task db.TaskTelemetry, projectKey, kind string) (string, string) {
	if source := strings.ToLower(strings.TrimSpace(task.Source)); source != "" {
		return source, strings.TrimSpace(task.ExternalKey)
	}
	taskID := strings.TrimSpace(task.TaskID)
	prefix := projectPrefix(taskID)
	suffix := strings.TrimPrefix(strings.ToUpper(taskID), prefix+"-")
	if kind != ExecutionTask && projectKey != "" && prefix == projectKey && allDigits(suffix) && taskID == strings.ToUpper(taskID) {
		return "jira", taskID
	}
	return "local", strings.TrimSpace(task.ExternalKey)
}

func normalizeMigrationKind(task db.TaskTelemetry, projectKeys map[string]struct{}) (string, error) {
	kind, err := NormalizeIssueType(task.IssueType)
	if err != nil || kind != ExecutionTask {
		return kind, err
	}
	if source := strings.ToLower(strings.TrimSpace(task.Source)); source != "" {
		if source == "jira" {
			return WorkItemRequirement, nil
		}
		return ExecutionTask, nil
	}
	taskID := strings.TrimSpace(task.TaskID)
	prefix := projectPrefix(taskID)
	suffix := strings.TrimPrefix(strings.ToUpper(taskID), prefix+"-")
	_, knownProject := projectKeys[prefix]
	if knownProject && taskID == strings.ToUpper(taskID) && allDigits(suffix) {
		return WorkItemRequirement, nil
	}
	return ExecutionTask, nil
}

func migrationChangeNeeded(change MigrationChange) bool {
	return strings.TrimSpace(change.IssueTypeBefore) != change.IssueTypeAfter ||
		NormalizeProjectKey(change.ProjectKeyBefore) != change.ProjectKeyAfter ||
		strings.TrimSpace(change.ParentWorkItemIDBefore) != change.ParentWorkItemIDAfter ||
		strings.ToLower(strings.TrimSpace(change.SourceBefore)) != change.SourceAfter ||
		strings.TrimSpace(change.ExternalKeyBefore) != change.ExternalKeyAfter
}

func normalizedMigrationGroup(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return ""
	}
	return value
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
