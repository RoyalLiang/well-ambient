package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/decisioncommands"
	"well-ambient/internal/telemetry"
)

type openJiraDecisionAdapter struct {
	server *Server
}

func (a openJiraDecisionAdapter) ReadIssueState(
	ctx context.Context,
	binding db.JiraExecutionBinding,
	issueKey string,
) (decisioncommands.JiraState, error) {
	if err := validateOpenJiraBinding(binding); err != nil {
		return decisioncommands.JiraState{}, err
	}
	issue, err := telemetry.NewJiraClient(&a.server.config.Jira).GetIssue(ctx, issueKey)
	if err != nil {
		return decisioncommands.JiraState{}, err
	}
	return jiraStateFromIssue(issue)
}

func jiraStateFromIssue(issue telemetry.JiraIssue) (decisioncommands.JiraState, error) {
	issueKey := strings.ToUpper(strings.TrimSpace(issue.Key))
	project := strings.ToUpper(strings.TrimSpace(issue.Fields.Project.Key))
	parts := strings.SplitN(issueKey, "-", 2)
	if len(parts) != 2 || parts[0] != project || parts[1] == "" || project == "" {
		return decisioncommands.JiraState{}, fmt.Errorf("Jira issue identity and project are unavailable or inconsistent")
	}
	assignee := ""
	if issue.Fields.Assignee != nil {
		assignee = strings.TrimSpace(issue.Fields.Assignee.Name)
	}
	updatedAt := parseOptionalJiraTime(issue.Fields.Updated)
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	state := decisioncommands.JiraState{
		IssueKey: issueKey, ProjectRef: project,
		Assignee: assignee, DueDate: strings.TrimSpace(issue.Fields.DueDate),
		UpdatedAt:   updatedAt,
		EvidenceRef: "jira-issue:" + issueKey + "@" + updatedAt.UTC().Format(time.RFC3339Nano),
	}
	if issue.Fields.Security != nil {
		state.SecurityID = strings.TrimSpace(issue.Fields.Security.ID)
		state.Security = strings.TrimSpace(issue.Fields.Security.Name)
	}
	return state, nil
}

func (a openJiraDecisionAdapter) Assign(
	ctx context.Context,
	binding db.JiraExecutionBinding,
	issueKey, assignee string,
) error {
	if err := validateOpenJiraBinding(binding); err != nil {
		return err
	}
	return telemetry.NewJiraClient(&a.server.config.Jira).UpdateAssigneeContext(ctx, issueKey, assignee)
}

func (a openJiraDecisionAdapter) SetDueDate(
	ctx context.Context,
	binding db.JiraExecutionBinding,
	issueKey, dueDate string,
) error {
	if err := validateOpenJiraBinding(binding); err != nil {
		return err
	}
	return telemetry.NewJiraClient(&a.server.config.Jira).UpdateDueDateContext(ctx, issueKey, dueDate)
}

func validateOpenJiraBinding(binding db.JiraExecutionBinding) error {
	if binding.ConnectorRef != "jira-primary" || binding.ExecutorRef != "jira-service" {
		return fmt.Errorf("unsupported Jira execution binding")
	}
	return nil
}
