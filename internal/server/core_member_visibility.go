package server

import (
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type coreMemberVisibility struct {
	filter    kpiCoreMemberFilter
	directory kpiUserDirectory
}

func (s *Server) loadCoreMemberVisibility() (coreMemberVisibility, []userdb.User, error) {
	var users []userdb.User
	if err := db.DB.Find(&users).Error; err != nil {
		return coreMemberVisibility{}, nil, err
	}
	filter := s.buildKPICoreMemberFilter(users)
	return coreMemberVisibility{
		filter:    filter,
		directory: newKPIUserDirectory(users),
	}, users, nil
}

func (v coreMemberVisibility) includesAssignee(assignee string) bool {
	return v.filter.includesAssignee(assignee, v.directory)
}

func (v coreMemberVisibility) filterTasks(tasks []db.TaskTelemetry) []db.TaskTelemetry {
	if !v.filter.enabled {
		return tasks
	}
	filtered := make([]db.TaskTelemetry, 0, len(tasks))
	for _, task := range tasks {
		if v.includesAssignee(task.Assignee) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func (v coreMemberVisibility) filterExecutionItems(items []ExecutionTaskItemDTO) []ExecutionTaskItemDTO {
	if !v.filter.enabled {
		return items
	}
	filtered := make([]ExecutionTaskItemDTO, 0, len(items))
	for _, item := range items {
		if !v.includesAssignee(item.Assignee) {
			continue
		}
		if item.ExecutionAssignee != "" && !v.includesAssignee(item.ExecutionAssignee) {
			item.ExecutionAssignee = ""
		}
		if item.JiraAssignee != "" && !v.includesAssignee(item.JiraAssignee) {
			item.JiraAssignee = ""
		}
		filtered = append(filtered, item)
	}
	return filtered
}
