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

	_ "github.com/mattn/go-sqlite3"
)

type BaselineTask struct {
	TaskID      string
	IssueType   string
	Repo        string
	TaskGroupID string
	ProjectKey  string
}

type BaselineProject struct {
	ProjectKey   string
	Repositories []string
}

type BaselineIssueTypeCount struct {
	IssueType        string `json:"issue_type"`
	Total            int    `json:"total"`
	WithTaskGroup    int    `json:"with_task_group"`
	WithoutTaskGroup int    `json:"without_task_group"`
}

type BaselineUnresolvedTask struct {
	TaskID    string `json:"task_id"`
	IssueType string `json:"issue_type"`
	Repo      string `json:"repo,omitempty"`
	Reason    string `json:"reason"`
}

type BaselineProjectResolution struct {
	Explicit      int `json:"explicit"`
	IDPrefix      int `json:"id_prefix"`
	UniqueRepo    int `json:"unique_repo"`
	AmbiguousRepo int `json:"ambiguous_repo"`
	Unresolved    int `json:"unresolved"`
}

type BaselineReport struct {
	TotalTasks          int                       `json:"total_tasks"`
	ConfiguredProjects  int                       `json:"configured_projects"`
	SchemaHasProjectKey bool                      `json:"schema_has_project_key"`
	IssueTypes          []BaselineIssueTypeCount  `json:"issue_types"`
	WithTaskGroup       int                       `json:"with_task_group"`
	WithoutTaskGroup    int                       `json:"without_task_group"`
	ProjectResolution   BaselineProjectResolution `json:"project_resolution"`
	UnresolvedTasks     []BaselineUnresolvedTask  `json:"unresolved_tasks"`
}

func AnalyzeBaseline(tasks []BaselineTask, projects []BaselineProject, schemaHasProjectKey bool) BaselineReport {
	projectKeys := make(map[string]struct{}, len(projects))
	repoProjects := make(map[string][]string)
	for _, project := range projects {
		key := normalizeProjectKey(project.ProjectKey)
		if key == "" {
			continue
		}
		projectKeys[key] = struct{}{}
		for _, repository := range project.Repositories {
			repo := normalizeRepository(repository)
			if repo == "" {
				continue
			}
			repoProjects[repo] = appendUnique(repoProjects[repo], key)
		}
	}

	report := BaselineReport{
		TotalTasks:          len(tasks),
		ConfiguredProjects:  len(projectKeys),
		SchemaHasProjectKey: schemaHasProjectKey,
	}
	issueCounts := make(map[string]*BaselineIssueTypeCount)

	for _, task := range tasks {
		issueType := normalizeIssueType(task.IssueType)
		count := issueCounts[issueType]
		if count == nil {
			count = &BaselineIssueTypeCount{IssueType: issueType}
			issueCounts[issueType] = count
		}
		count.Total++
		if strings.TrimSpace(task.TaskGroupID) == "" || strings.TrimSpace(task.TaskGroupID) == "-" {
			count.WithoutTaskGroup++
			report.WithoutTaskGroup++
		} else {
			count.WithTaskGroup++
			report.WithTaskGroup++
		}

		explicit := normalizeProjectKey(task.ProjectKey)
		if explicit != "" {
			if _, ok := projectKeys[explicit]; ok {
				report.ProjectResolution.Explicit++
				continue
			}
			report.ProjectResolution.Unresolved++
			report.UnresolvedTasks = append(report.UnresolvedTasks, BaselineUnresolvedTask{
				TaskID: task.TaskID, IssueType: issueType, Repo: task.Repo,
				Reason: fmt.Sprintf("explicit project %q is not in the project catalog", explicit),
			})
			continue
		}

		prefix := projectPrefix(task.TaskID)
		if _, ok := projectKeys[prefix]; ok {
			report.ProjectResolution.IDPrefix++
			continue
		}

		repoMatches := repoProjects[normalizeRepository(task.Repo)]
		switch len(repoMatches) {
		case 1:
			report.ProjectResolution.UniqueRepo++
		case 0:
			report.ProjectResolution.Unresolved++
			report.UnresolvedTasks = append(report.UnresolvedTasks, BaselineUnresolvedTask{
				TaskID: task.TaskID, IssueType: issueType, Repo: task.Repo,
				Reason: "no explicit project, known ID prefix, or unique repository mapping",
			})
		default:
			report.ProjectResolution.AmbiguousRepo++
			report.UnresolvedTasks = append(report.UnresolvedTasks, BaselineUnresolvedTask{
				TaskID: task.TaskID, IssueType: issueType, Repo: task.Repo,
				Reason: fmt.Sprintf("repository maps to multiple projects: %s", strings.Join(repoMatches, ", ")),
			})
		}
	}

	report.IssueTypes = make([]BaselineIssueTypeCount, 0, len(issueCounts))
	for _, count := range issueCounts {
		report.IssueTypes = append(report.IssueTypes, *count)
	}
	sort.Slice(report.IssueTypes, func(i, j int) bool {
		return report.IssueTypes[i].IssueType < report.IssueTypes[j].IssueType
	})
	sort.Slice(report.UnresolvedTasks, func(i, j int) bool {
		return report.UnresolvedTasks[i].TaskID < report.UnresolvedTasks[j].TaskID
	})
	return report
}

func ReadSQLiteBaseline(ctx context.Context, databasePath string) (BaselineReport, error) {
	absolutePath, err := filepath.Abs(databasePath)
	if err != nil {
		return BaselineReport{}, fmt.Errorf("resolve database path: %w", err)
	}
	dsn := (&url.URL{
		Scheme:   "file",
		Path:     absolutePath,
		RawQuery: "mode=ro&_query_only=1",
	}).String()
	database, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return BaselineReport{}, fmt.Errorf("open database read-only: %w", err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	if err := database.PingContext(ctx); err != nil {
		return BaselineReport{}, fmt.Errorf("ping database read-only: %w", err)
	}

	schemaHasProjectKey, err := tableHasColumn(ctx, database, "task_telemetries", "project_key")
	if err != nil {
		return BaselineReport{}, err
	}
	projectSelect := "''"
	if schemaHasProjectKey {
		projectSelect = "coalesce(project_key, '')"
	}
	rows, err := database.QueryContext(ctx, fmt.Sprintf(`
		SELECT task_id, coalesce(issue_type, ''), coalesce(repo, ''),
		       coalesce(task_group_id, ''), %s
		FROM task_telemetries
		ORDER BY task_id`, projectSelect))
	if err != nil {
		return BaselineReport{}, fmt.Errorf("query task baseline: %w", err)
	}
	var tasks []BaselineTask
	for rows.Next() {
		var task BaselineTask
		if err := rows.Scan(&task.TaskID, &task.IssueType, &task.Repo, &task.TaskGroupID, &task.ProjectKey); err != nil {
			rows.Close()
			return BaselineReport{}, fmt.Errorf("scan task baseline: %w", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Close(); err != nil {
		return BaselineReport{}, fmt.Errorf("close task baseline rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BaselineReport{}, fmt.Errorf("iterate task baseline: %w", err)
	}

	projectRows, err := database.QueryContext(ctx, `
		SELECT coalesce(project_key, ''), coalesce(git_repos_json, '[]')
		FROM project_configs
		ORDER BY project_key`)
	if err != nil {
		return BaselineReport{}, fmt.Errorf("query project baseline: %w", err)
	}
	var projects []BaselineProject
	for projectRows.Next() {
		var key, repositoriesJSON string
		if err := projectRows.Scan(&key, &repositoriesJSON); err != nil {
			projectRows.Close()
			return BaselineReport{}, fmt.Errorf("scan project baseline: %w", err)
		}
		var repositories []string
		if err := json.Unmarshal([]byte(repositoriesJSON), &repositories); err != nil {
			projectRows.Close()
			return BaselineReport{}, fmt.Errorf("decode repositories for project %q: %w", key, err)
		}
		projects = append(projects, BaselineProject{ProjectKey: key, Repositories: repositories})
	}
	if err := projectRows.Close(); err != nil {
		return BaselineReport{}, fmt.Errorf("close project baseline rows: %w", err)
	}
	if err := projectRows.Err(); err != nil {
		return BaselineReport{}, fmt.Errorf("iterate project baseline: %w", err)
	}

	return AnalyzeBaseline(tasks, projects, schemaHasProjectKey), nil
}

func tableHasColumn(ctx context.Context, database *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := database.QueryContext(ctx, "PRAGMA table_info("+tableName+")")
	if err != nil {
		return false, fmt.Errorf("inspect %s schema: %w", tableName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan %s schema: %w", tableName, err)
		}
		if strings.EqualFold(name, columnName) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate %s schema: %w", tableName, err)
	}
	return false, nil
}

func normalizeIssueType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	return value
}

func normalizeProjectKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeRepository(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func projectPrefix(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	index := strings.Index(taskID, "-")
	if index <= 0 {
		return ""
	}
	return normalizeProjectKey(taskID[:index])
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}
