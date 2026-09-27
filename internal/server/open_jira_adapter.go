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
	assignee := ""
	if issue.Fields.Assignee != nil {
		assignee = strings.TrimSpace(issue.Fields.Assignee.Name)
	}
	updatedAt := parseOptionalJiraTime(issue.Fields.Updated)
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	state := decisioncommands.JiraState{
		Assignee: assignee, DueDate: strings.TrimSpace(issue.Fields.DueDate),
		UpdatedAt:   updatedAt,
		EvidenceRef: "jira-issue:" + strings.ToUpper(strings.TrimSpace(issueKey)) + "@" + updatedAt.UTC().Format(time.RFC3339Nano),
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
