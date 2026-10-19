package server

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/telemetry"
)

// syncCurrentJiraFacts has an independent cursor and never reads comments or
// runs scoring. Historical/retry workers retain ownership of older coverage.
func (s *Server) syncCurrentJiraFacts() {
	if s == nil {
		return
	}
	s.jiraCurrentSyncMu.Lock()
	defer s.jiraCurrentSyncMu.Unlock()
	if s.config == nil || !s.config.Jira.Enabled || db.DB == nil {
		return
	}
	started := time.Now()
	state := db.JiraInboundSyncState{Scope: jiraCurrentSyncScope}
	if err := db.DB.Where("scope = ?", jiraCurrentSyncScope).Limit(1).Find(&state).Error; err != nil {
		log.Printf("Jira current facts: failed to read cursor: %v", err)
		return
	}
	// The first current-facts window is bounded. The existing reconciliation
	// worker still covers history, including any pre-start changes outside it.
	// Keep the initial cycle's anchor across failed attempts, so startup outages
	// cannot slide the first window past changes that still need retrying.
	cycleStartedAt := started
	since := state.SuccessfulThrough
	if since.IsZero() {
		if !state.LastStartedAt.IsZero() {
			cycleStartedAt = state.LastStartedAt
		}
		since = cycleStartedAt
	}
	state.LastStartedAt, state.UpdatedAt = cycleStartedAt, started
	if err := db.DB.Save(&state).Error; err != nil {
		log.Printf("Jira current facts: failed to record start: %v", err)
		return
	}
	var tasks []db.TaskTelemetry
	if err := db.DB.Select("task_id", "project_key", "source", "status", "last_update", "completed_at").
		Where("task_id LIKE ? AND (source IS NULL OR TRIM(source) = '' OR LOWER(TRIM(source)) = ?)", "%-%", "jira").
		Find(&tasks).Error; err != nil {
		finishJiraInboundCycle(state, cycleStartedAt, 0, 0, []error{err})
		return
	}
	known := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		known[strings.ToUpper(strings.TrimSpace(task.TaskID))] = struct{}{}
	}
	queries := buildJiraReconciliationJQLs(&s.config.Jira, tasks, nil, since, started)
	if len(queries) == 0 {
		finishJiraInboundCycle(state, cycleStartedAt, 0, 0, []error{errors.New("no configured or known Jira scope for current facts")})
		return
	}
	client := telemetry.NewJiraClient(&s.config.Jira)
	issues := make(map[string]telemetry.JiraIssue)
	var failures []error
	for _, query := range queries {
		result, err := client.SearchIssues(query)
		if err != nil {
			failures = append(failures, fmt.Errorf("read current Jira changes: %w", err))
			continue
		}
		for _, issue := range result {
			if _, ok := known[strings.ToUpper(strings.TrimSpace(issue.Key))]; ok {
				mergeJiraIssue(issues, issue)
			}
		}
	}
	changed := 0
	for _, issue := range issues {
		updated, err := s.reconcileJiraIssueFacts(issue)
		if updated {
			changed++
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	finishJiraInboundCycle(state, cycleStartedAt, len(issues), changed, failures)
	log.Printf("Jira current facts: issues=%d changed=%d duration=%s errors=%d", len(issues), changed, time.Since(started).Round(time.Millisecond), len(failures))
}
