package server

import (
	"net/http"
	"strings"

	"well-ambient/internal/db"
)

func queryFilterValues(r *http.Request, key string) []string {
	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, rawValue := range r.URL.Query()[key] {
		for _, candidate := range strings.Split(rawValue, ",") {
			value := strings.TrimSpace(candidate)
			if value == "" || strings.EqualFold(value, "all") {
				continue
			}
			normalized := strings.ToLower(value)
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			values = append(values, value)
		}
	}
	return values
}

func matchesQueryFilter(value string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(filter)) {
			return true
		}
	}
	return false
}

func taskMatchesProjectFilters(projectKey, taskID string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	projectKey = strings.ToUpper(strings.TrimSpace(projectKey))
	if projectKey != "" {
		for _, filter := range filters {
			if projectKey == strings.ToUpper(strings.TrimSpace(filter)) {
				return true
			}
		}
		return false
	}
	taskID = strings.ToUpper(strings.TrimSpace(taskID))
	for _, filter := range filters {
		prefix := strings.ToUpper(strings.TrimSpace(filter))
		if prefix != "" && strings.HasPrefix(taskID, prefix+"-") {
			return true
		}
	}
	return false
}

func filterTaskTelemetriesByQuery(tasks []db.TaskTelemetry, projectFilters, assigneeFilters []string) []db.TaskTelemetry {
	if len(projectFilters) == 0 && len(assigneeFilters) == 0 {
		return tasks
	}
	filtered := make([]db.TaskTelemetry, 0, len(tasks))
	for _, task := range tasks {
		if !taskMatchesProjectFilters(task.ProjectKey, task.TaskID, projectFilters) || !matchesQueryFilter(task.Assignee, assigneeFilters) {
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered
}
