package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/deliveryplanning"
	"well-ambient/internal/kanban"
	"well-ambient/internal/performance"
	"well-ambient/internal/solutions"
	"well-ambient/internal/telemetry"

	"gorm.io/gorm"
)

const jiraCompletedKeepAliveWindow = 14 * 24 * time.Hour
const jiraInboundSyncScope = "jira-inbound"
const jiraInboundSyncOverlap = 5 * time.Minute

// startJiraSyncWorker starts a background loop to fetch tasks/bugs from Jira
func (s *Server) startJiraSyncWorker() {
	log.Println("Starting background Jira task synchronization worker...")
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds for quick local testing/responsiveness
	defer ticker.Stop()
	var lastPerformanceHistorySync time.Time
	runCycle := func() {
		s.syncJiraTasks()
		performanceConfig := s.config.PerformanceBrain.Normalized()
		if !performanceConfig.Enabled || !*performanceConfig.JiraHistoryEnabled {
			return
		}
		interval := time.Duration(performanceConfig.IntervalMinutes) * time.Minute
		if !lastPerformanceHistorySync.IsZero() && time.Since(lastPerformanceHistorySync) < interval {
			return
		}
		changed, err := s.syncPerformanceJiraHistory(telemetry.NewJiraClient(&s.config.Jira))
		if err != nil {
			log.Printf("Jira performance history sync failed: %v", err)
			return
		}
		lastPerformanceHistorySync = time.Now()
		if changed && s.performance != nil {
			if _, err := s.performance.RunOnce(context.Background(), "schedule"); err != nil {
				log.Printf("Performance calculation after Jira history sync failed: %v", err)
			}
		}
	}

	// Initial run
	runCycle()

	for range ticker.C {
		runCycle()
	}
}

func (s *Server) syncJiraTasks() {
	if s == nil {
		return
	}
	s.jiraInboundSyncMu.Lock()
	defer s.jiraInboundSyncMu.Unlock()
	if s.config == nil || !s.config.Jira.Enabled || db.DB == nil {
		return
	}

	cycleStartedAt := time.Now()
	checkpoint, checkpointErr := loadJiraInboundSyncState()
	if checkpointErr != nil {
		log.Printf("Jira sync: failed to load inbound checkpoint: %v", checkpointErr)
	}
	checkpoint.Scope = jiraInboundSyncScope
	checkpoint.LastStartedAt = cycleStartedAt
	checkpoint.UpdatedAt = cycleStartedAt
	if err := db.DB.Save(&checkpoint).Error; err != nil {
		log.Printf("Jira sync: failed to persist cycle start: %v", err)
	}

	jql := buildJQL(&s.config.Jira)
	if jql == "" {
		finishJiraInboundCycle(checkpoint, cycleStartedAt, 0, 0, []error{errors.New("Jira inbound sync scope is empty")})
		return
	}
	jc := telemetry.NewJiraClient(&s.config.Jira)

	issues, err := jc.SearchIssues(jql)
	if err != nil {
		log.Printf("Jira sync: failed to search issues: %v", err)
		finishJiraInboundCycle(checkpoint, cycleStartedAt, 0, 0, []error{err})
		return
	}

	log.Printf("Jira sync: retrieved %d issues matching JQL: %s", len(issues), jql)
	mergedIssues := make(map[string]telemetry.JiraIssue, len(issues))
	primaryKeys := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		mergeJiraIssue(mergedIssues, issue)
		primaryKeys[strings.ToUpper(strings.TrimSpace(issue.Key))] = struct{}{}
	}

	var activeLocalTasks []db.TaskTelemetry
	knownLocalKeys := make(map[string]struct{})
	cycleErrors := make([]error, 0)
	if queryErr := db.DB.
		Where("task_id LIKE ? AND (source IS NULL OR TRIM(source) = '' OR LOWER(TRIM(source)) = ?)", "%-%", "jira").
		Find(&activeLocalTasks).Error; queryErr != nil {
		cycleErrors = append(cycleErrors, fmt.Errorf("load local Jira issues: %w", queryErr))
	} else {
		for _, task := range activeLocalTasks {
			knownLocalKeys[strings.ToUpper(strings.TrimSpace(task.TaskID))] = struct{}{}
		}
		for _, reconciliationJQL := range buildJiraReconciliationJQLs(&s.config.Jira, activeLocalTasks, primaryKeys, checkpoint.SuccessfulThrough, cycleStartedAt) {
			reconciliationIssues, searchErr := jc.SearchIssues(reconciliationJQL)
			if searchErr != nil {
				log.Printf("Jira sync reconciliation: failed to search JQL %s: %v", reconciliationJQL, searchErr)
				cycleErrors = append(cycleErrors, fmt.Errorf("search reconciliation JQL: %w", searchErr))
				continue
			}
			for _, issue := range reconciliationIssues {
				if _, known := knownLocalKeys[strings.ToUpper(strings.TrimSpace(issue.Key))]; !known {
					continue
				}
				mergeJiraIssue(mergedIssues, issue)
			}
		}
	}

	orderedIssues := make([]telemetry.JiraIssue, 0, len(mergedIssues))
	for _, issue := range mergedIssues {
		orderedIssues = append(orderedIssues, issue)
	}
	sort.SliceStable(orderedIssues, func(i, j int) bool {
		left := parseOptionalJiraTime(orderedIssues[i].Fields.Updated)
		right := parseOptionalJiraTime(orderedIssues[j].Fields.Updated)
		if left.Equal(right) {
			return orderedIssues[i].Key < orderedIssues[j].Key
		}
		return left.After(right)
	})

	changedCount := 0
	for _, issue := range orderedIssues {
		changed, reconcileErr := s.reconcileJiraIssue(jc, issue)
		if changed {
			changedCount++
			BroadcastTelemetryUpdated(issue.Key)
		}
		if reconcileErr != nil {
			log.Printf("Jira sync: failed to reconcile %s: %v", issue.Key, reconcileErr)
			cycleErrors = append(cycleErrors, reconcileErr)
		}
	}
	finishJiraInboundCycle(checkpoint, cycleStartedAt, len(orderedIssues), changedCount, cycleErrors)
}

func mergeJiraIssue(issues map[string]telemetry.JiraIssue, incoming telemetry.JiraIssue) {
	key := strings.ToUpper(strings.TrimSpace(incoming.Key))
	if key == "" {
		return
	}
	current, exists := issues[key]
	if !exists || parseOptionalJiraTime(incoming.Fields.Updated).After(parseOptionalJiraTime(current.Fields.Updated)) {
		issues[key] = incoming
	}
}

func buildJiraReconciliationJQLs(cfg *config.JiraConfig, localTasks []db.TaskTelemetry, primaryKeys map[string]struct{}, successfulThrough, now time.Time) []string {
	configuredProjectKeys := config.JiraProjectKeys(cfg)
	customJQLProjectKeys := extractJIRAProjects(cfg.CustomJQL)
	configuredProjectKeys = normalizeJIRAScopeValues(append(configuredProjectKeys, customJQLProjectKeys...), true)
	restrictProjects := len(cfg.SyncProjects) > 0 || len(customJQLProjectKeys) > 0 ||
		(len(cfg.VersionSources) > 0 && strings.TrimSpace(cfg.CustomJQL) == "")

	since := successfulThrough.Add(-jiraInboundSyncOverlap)
	if successfulThrough.IsZero() {
		since = now.Add(-jiraCompletedKeepAliveWindow)
	} else if since.After(now) {
		since = now.Add(-jiraInboundSyncOverlap)
	}
	if len(configuredProjectKeys) > 0 {
		quotedProjects := make([]string, 0, len(configuredProjectKeys))
		for _, projectKey := range configuredProjectKeys {
			quotedProjects = append(quotedProjects, fmt.Sprintf("%q", projectKey))
		}
		return []string{fmt.Sprintf(
			`project in (%s) AND updated >= %q`,
			strings.Join(quotedProjects, ", "),
			since.In(time.Local).Format("2006-01-02 15:04"),
		)}
	}

	keys := make([]string, 0, len(localTasks))
	seen := make(map[string]struct{}, len(localTasks))
	for _, task := range localTasks {
		if !shouldIncludeInJiraKeepAlive(task, now) {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(task.TaskID))
		projectKey, valid := splitJiraIssueKey(key)
		if !valid {
			continue
		}
		if _, exists := primaryKeys[key]; exists {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		if restrictProjects {
			allowed := false
			for _, configuredProjectKey := range configuredProjectKeys {
				if strings.EqualFold(configuredProjectKey, projectKey) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		seen[key] = struct{}{}
		keys = append(keys, fmt.Sprintf("%q", key))
	}
	sort.Strings(keys)

	sinceClause := fmt.Sprintf(` AND updated >= %q`, since.In(time.Local).Format("2006-01-02 15:04"))

	const batchSize = 50
	queries := make([]string, 0, (len(keys)+batchSize-1)/batchSize)
	for start := 0; start < len(keys); start += batchSize {
		end := start + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		queries = append(queries, fmt.Sprintf("key in (%s)%s", strings.Join(keys[start:end], ", "), sinceClause))
	}
	return queries
}

func splitJiraIssueKey(issueKey string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(issueKey), "-")
	if len(parts) != 2 || len(parts[0]) < 2 || len(parts[0]) > 10 || parts[1] == "" {
		return "", false
	}
	for index, char := range parts[0] {
		if index == 0 && (char < 'A' || char > 'Z') {
			return "", false
		}
		if char < 'A' || char > 'Z' {
			if char < '0' || char > '9' {
				return "", false
			}
		}
	}
	for _, char := range parts[1] {
		if char < '0' || char > '9' {
			return "", false
		}
	}
	return parts[0], true
}

func (s *Server) reconcileJiraIssue(jc *telemetry.JiraClient, issue telemetry.JiraIssue) (bool, error) {
	issue.Key = strings.TrimSpace(issue.Key)
	if issue.Key == "" {
		return false, nil
	}
	assigneeName := jiraIssueAssigneeName(issue)
	reporterName, reporterUser := jiraIssueReporterIdentity(issue)
	if issue.Fields.Assignee != nil {
		s.syncJiraUserToLocal(issue.Fields.Assignee.Name, issue.Fields.Assignee.DisplayName, issue.Fields.Assignee.EmailAddress)
	}
	jiraMappedStatus := mapJiraStatus(issue.Fields.Status.Name)
	issueType := mapJiraIssueType(issue.Fields.IssueType.Name)
	createdTime := parseJiraTime(issue.Fields.Created)
	sourceUpdatedAt := parseOptionalJiraTime(issue.Fields.Updated)
	projectKey := deliveryplanning.NormalizeProjectKey(issue.Fields.Project.Key)
	changed := false

	var existing db.TaskTelemetry
	loadErr := db.DB.Where("task_id = ?", issue.Key).First(&existing).Error
	switch {
	case errors.Is(loadErr, gorm.ErrRecordNotFound):
		existing = db.TaskTelemetry{
			TaskID: issue.Key, ProjectKey: projectKey, Source: "jira", ExternalKey: issue.Key,
			PlanningState: deliveryplanning.PlanningReady, Title: issue.Fields.Summary,
			Repo: "-", Assignee: assigneeName, JiraReporter: reporterName, JiraReporterUser: reporterUser,
			Branch: "-", LastCommit: "-",
			Status: jiraMappedStatus, IssueType: issueType, TaskCreatedAt: createdTime,
			LastUpdate: time.Now(), SourceUpdatedAt: sourceUpdatedAt,
		}
		applyJiraPerformanceFields(&existing, issue)
		if createErr := db.DB.Create(&existing).Error; createErr != nil {
			return false, fmt.Errorf("create Jira issue %s: %w", issue.Key, createErr)
		}
		changed = true
		log.Printf("Jira sync: created new local task %s with status %s", issue.Key, jiraMappedStatus)
		if kanbanErr := kanban.SyncTaskToKanban(&existing); kanbanErr != nil {
			log.Printf("Jira sync: failed to write task %s to kanban markdown: %v", issue.Key, kanbanErr)
		}
	case loadErr != nil:
		return false, fmt.Errorf("load Jira issue %s: %w", issue.Key, loadErr)
	default:
		hasChanges := false
		if existing.Title != issue.Fields.Summary {
			existing.Title = issue.Fields.Summary
			hasChanges = true
		}
		if existing.Assignee != assigneeName {
			if shouldPreserveLocalAssignee(existing, assigneeName, jiraMappedStatus, sourceUpdatedAt, time.Now()) {
				log.Printf("Jira sync: preserving local assignee override for %s (%s), ignoring Jira assignee %s", issue.Key, existing.Assignee, assigneeName)
			} else {
				existing.Assignee = assigneeName
				hasChanges = true
			}
		}
		if existing.JiraReporter != reporterName {
			existing.JiraReporter = reporterName
			hasChanges = true
		}
		if existing.JiraReporterUser != reporterUser {
			existing.JiraReporterUser = reporterUser
			hasChanges = true
		}
		if existing.IssueType != issueType {
			existing.IssueType = issueType
			hasChanges = true
		}
		if existing.ProjectKey == "" && projectKey != "" {
			existing.ProjectKey = projectKey
			hasChanges = true
		}
		if existing.Source == "" {
			existing.Source = "jira"
			hasChanges = true
		}
		if existing.ExternalKey == "" {
			existing.ExternalKey = issue.Key
			hasChanges = true
		}
		if existing.PlanningState == "" {
			existing.PlanningState = deliveryplanning.PlanningReady
			hasChanges = true
		}
		if existing.TaskCreatedAt.Unix() != createdTime.Unix() {
			existing.TaskCreatedAt = createdTime
			hasChanges = true
		}
		if !sourceUpdatedAt.IsZero() && !existing.SourceUpdatedAt.Equal(sourceUpdatedAt) {
			existing.SourceUpdatedAt = sourceUpdatedAt
			hasChanges = true
		}
		if applyJiraPerformanceFields(&existing, issue) {
			hasChanges = true
		}
		jiraOwnsStatus := strings.EqualFold(strings.TrimSpace(existing.Source), "jira")
		if existing.Status != jiraMappedStatus && (jiraOwnsStatus || time.Since(existing.LastUpdate) > 15*time.Second) {
			log.Printf("Jira sync: status of %s changed on Jira from %s -> %s, updating locally", issue.Key, existing.Status, jiraMappedStatus)
			existing.Status = jiraMappedStatus
			hasChanges = true
		}
		if hasChanges {
			existing.LastUpdate = time.Now()
			if saveErr := db.DB.Save(&existing).Error; saveErr != nil {
				return false, fmt.Errorf("save Jira issue %s: %w", issue.Key, saveErr)
			}
			changed = true
			if kanbanErr := kanban.SyncTaskToKanban(&existing); kanbanErr != nil {
				log.Printf("Jira sync: failed to write updated task %s to kanban markdown: %v", issue.Key, kanbanErr)
			}
		}
	}

	var reconcileErrors []error
	if _, eventErr := s.appendJiraPerformanceEvents(issue); eventErr != nil {
		reconcileErrors = append(reconcileErrors, fmt.Errorf("append Jira performance events for %s: %w", issue.Key, eventErr))
	}
	s.reconcileJiraIssueVersions(issue)
	commentsChanged, commentsErr := s.syncJiraCommentsForIssue(jc, issue.Key, sourceUpdatedAt)
	changed = changed || commentsChanged
	if commentsErr != nil {
		reconcileErrors = append(reconcileErrors, commentsErr)
	}
	if len(reconcileErrors) > 0 {
		return changed, errors.Join(reconcileErrors...)
	}
	return changed, nil
}

func loadJiraInboundSyncState() (db.JiraInboundSyncState, error) {
	var state db.JiraInboundSyncState
	err := db.DB.Where("scope = ?", jiraInboundSyncScope).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.JiraInboundSyncState{Scope: jiraInboundSyncScope}, nil
	}
	return state, err
}

func finishJiraInboundCycle(state db.JiraInboundSyncState, startedAt time.Time, issueCount, changedCount int, cycleErrors []error) {
	now := time.Now()
	state.Scope = jiraInboundSyncScope
	state.LastStartedAt = startedAt
	state.LastIssueCount = issueCount
	state.LastChangedCount = changedCount
	state.UpdatedAt = now
	if len(cycleErrors) == 0 {
		state.SuccessfulThrough = startedAt
		state.LastSucceededAt = now
		state.LastError = ""
	} else {
		message := errors.Join(cycleErrors...).Error()
		if len(message) > 4000 {
			message = message[:4000]
		}
		state.LastError = message
	}
	if err := db.DB.Save(&state).Error; err != nil {
		log.Printf("Jira sync: failed to persist cycle result: %v", err)
	}
}

// syncPerformanceJiraHistory imports resolved and already-due Jira issues for
// the configured assessment window. It deliberately does not reuse the
// active-work JQL or synchronize comments: personnel scoring needs both the
// achieved numerator and the due-eligible denominator.
func (s *Server) syncPerformanceJiraHistory(jc *telemetry.JiraClient) (bool, error) {
	if s == nil || s.config == nil || jc == nil || db.DB == nil {
		return false, nil
	}
	jql := buildPerformanceJiraHistoryJQL(s.config, s.configuredKPICoreMembers())
	if jql == "" {
		return false, nil
	}
	issues, err := jc.SearchIssues(jql)
	if err != nil {
		return false, err
	}
	log.Printf("Jira performance history sync: retrieved %d resolved-or-due issues", len(issues))

	changed := false
	now := time.Now()
	for _, issue := range issues {
		if parseOptionalJiraTime(issue.Fields.ResolutionDate).IsZero() && parseOptionalJiraDate(issue.Fields.DueDate) == nil {
			continue
		}
		eventsChanged, eventErr := s.appendJiraPerformanceEvents(issue)
		if eventErr != nil {
			return changed, fmt.Errorf("append historical Jira events for %s: %w", issue.Key, eventErr)
		}
		changed = changed || eventsChanged
		assigneeName := jiraIssueAssigneeName(issue)
		if issue.Fields.Assignee != nil {
			s.syncJiraUserToLocal(issue.Fields.Assignee.Name, issue.Fields.Assignee.DisplayName, issue.Fields.Assignee.EmailAddress)
		}
		projectKey := deliveryplanning.NormalizeProjectKey(issue.Fields.Project.Key)
		sourceUpdatedAt := parseOptionalJiraTime(issue.Fields.Updated)

		var task db.TaskTelemetry
		err := db.DB.Where("task_id = ?", issue.Key).First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			task = db.TaskTelemetry{
				TaskID: issue.Key, ProjectKey: projectKey, Source: "jira", ExternalKey: issue.Key,
				PlanningState: deliveryplanning.PlanningReady, Title: issue.Fields.Summary,
				Repo: "-", Assignee: assigneeName, Branch: "-", LastCommit: "-",
				Status: mapJiraStatus(issue.Fields.Status.Name), IssueType: mapJiraIssueType(issue.Fields.IssueType.Name),
				TaskCreatedAt: parseJiraTime(issue.Fields.Created), LastUpdate: now, SourceUpdatedAt: sourceUpdatedAt,
			}
			applyJiraPerformanceFields(&task, issue)
			if err := db.DB.Create(&task).Error; err != nil {
				return changed, fmt.Errorf("create historical Jira issue %s: %w", issue.Key, err)
			}
			changed = true
			continue
		}
		if err != nil {
			return changed, fmt.Errorf("load historical Jira issue %s: %w", issue.Key, err)
		}

		taskChanged := false
		for current, incoming := range map[*string]string{
			&task.Title: issue.Fields.Summary, &task.Assignee: assigneeName,
			&task.ProjectKey: projectKey, &task.Source: "jira", &task.ExternalKey: issue.Key,
		} {
			if *current != incoming {
				*current = incoming
				taskChanged = true
			}
		}
		status := mapJiraStatus(issue.Fields.Status.Name)
		if task.Status != status {
			task.Status = status
			taskChanged = true
		}
		issueType := mapJiraIssueType(issue.Fields.IssueType.Name)
		if task.IssueType != issueType {
			task.IssueType = issueType
			taskChanged = true
		}
		if task.PlanningState == "" {
			task.PlanningState = deliveryplanning.PlanningReady
			taskChanged = true
		}
		if !sourceUpdatedAt.IsZero() && !task.SourceUpdatedAt.Equal(sourceUpdatedAt) {
			task.SourceUpdatedAt = sourceUpdatedAt
			taskChanged = true
		}
		if applyJiraPerformanceFields(&task, issue) {
			taskChanged = true
		}
		if !taskChanged {
			continue
		}
		task.LastUpdate = now
		if err := db.DB.Save(&task).Error; err != nil {
			return changed, fmt.Errorf("save historical Jira issue %s: %w", issue.Key, err)
		}
		changed = true
	}
	return changed, nil
}

func buildPerformanceJiraHistoryJQL(cfg *config.Config, coreMembers []string) string {
	if cfg == nil || !cfg.Jira.Enabled || !cfg.PerformanceBrain.Normalized().Enabled || !*cfg.PerformanceBrain.Normalized().JiraHistoryEnabled {
		return ""
	}
	projects := config.JiraProjectKeys(&cfg.Jira)
	projects = append(projects, extractJIRAProjects(cfg.Jira.CustomJQL)...)
	projects = normalizeJIRAScopeValues(projects, true)
	coreMembers = normalizeJIRAScopeValues(coreMembers, false)
	if len(projects) == 0 || len(coreMembers) == 0 {
		return ""
	}
	issueTypes := extractJIRAIssueTypes(cfg.Jira.CustomJQL)
	if len(issueTypes) == 0 {
		issueTypes = []string{"Bug", "Task"}
	}
	issueTypes = normalizeJIRAScopeValues(issueTypes, false)
	windowDays := cfg.PerformanceBrain.Normalized().AssessmentWindowDays
	bugIssueTypes := make([]string, 0)
	for _, issueType := range issueTypes {
		if mapJiraIssueType(issueType) == deliveryplanning.WorkItemBug {
			bugIssueTypes = append(bugIssueTypes, issueType)
		}
	}
	if len(bugIssueTypes) == 0 {
		bugIssueTypes = []string{"Bug"}
	}
	return fmt.Sprintf(
		"project in (%s) AND issuetype in (%s) AND ((assignee in (%s) AND ((statusCategory = Done AND resolutiondate >= -%dd) OR (due >= -%dd AND due <= now()))) OR (issuetype in (%s) AND (created >= -%dd OR updated >= -%dd OR resolutiondate >= -%dd)))",
		quoteJIRAValues(projects), quoteJIRAValues(issueTypes), quoteJIRAValues(coreMembers), windowDays, windowDays,
		quoteJIRAValues(bugIssueTypes), windowDays, windowDays, windowDays,
	)
}

func extractJIRAIssueTypes(jql string) []string {
	inMatch := regexp.MustCompile(`(?i)(?:issuetype|type)\s+in\s*\(([^)]*)\)`).FindStringSubmatch(jql)
	if len(inMatch) >= 2 {
		return splitJIRAListValues(inMatch[1])
	}
	eqMatch := regexp.MustCompile(`(?i)(?:issuetype|type)\s*=\s*("[^"]+"|'[^']+'|[A-Za-z0-9_.-]+)`).FindStringSubmatch(jql)
	if len(eqMatch) >= 2 {
		value := strings.TrimSpace(strings.Trim(eqMatch[1], `"'`))
		if value != "" {
			return []string{value}
		}
	}
	return nil
}

func normalizeJIRAScopeValues(values []string, uppercase bool) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if uppercase {
			value = strings.ToUpper(value)
		}
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func quoteJIRAValues(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ReplaceAll(value, `\`, `\\`)
		value = strings.ReplaceAll(value, `"`, `\"`)
		quoted = append(quoted, `"`+value+`"`)
	}
	return strings.Join(quoted, ", ")
}

func jiraIssueAssigneeName(issue telemetry.JiraIssue) string {
	if issue.Fields.Assignee == nil {
		return "未指派"
	}
	for _, value := range []string{issue.Fields.Assignee.DisplayName, issue.Fields.Assignee.Name, issue.Fields.Assignee.EmailAddress} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "未指派"
}

func jiraIssueReporterIdentity(issue telemetry.JiraIssue) (string, string) {
	if issue.Fields.Reporter == nil {
		return "", ""
	}
	displayName := strings.TrimSpace(issue.Fields.Reporter.DisplayName)
	username := strings.TrimSpace(issue.Fields.Reporter.Name)
	if displayName == "" {
		displayName = username
	}
	if displayName == "" {
		displayName = strings.TrimSpace(issue.Fields.Reporter.EmailAddress)
	}
	if username == "" {
		username = strings.TrimSpace(issue.Fields.Reporter.EmailAddress)
	}
	return displayName, username
}

func applyJiraPerformanceFields(task *db.TaskTelemetry, issue telemetry.JiraIssue) bool {
	if task == nil {
		return false
	}
	changed := false
	originWorkItemID := jiraOriginRequirementID(issue)
	if task.ParentWorkItemID != originWorkItemID {
		task.ParentWorkItemID = originWorkItemID
		changed = true
	}
	priority := strings.TrimSpace(issue.Fields.Priority.Name)
	if task.Priority != priority {
		task.Priority = priority
		changed = true
	}
	if task.Severity != priority {
		task.Severity = priority
		changed = true
	}
	if resolution := parseOptionalJiraTime(issue.Fields.ResolutionDate); !resolution.IsZero() && strings.EqualFold(mapJiraStatus(issue.Fields.Status.Name), "done") {
		if task.CompletedAt == nil || !task.CompletedAt.Equal(resolution) {
			task.CompletedAt = &resolution
			changed = true
		}
	}
	if dueDate := parseOptionalJiraDate(issue.Fields.DueDate); dueDate != nil {
		if task.DueDate == nil || !task.DueDate.Equal(*dueDate) {
			task.DueDate = dueDate
			changed = true
		}
	}
	estimateSeconds := issue.Fields.TimeOriginalEstimate
	if estimateSeconds <= 0 {
		estimateSeconds = issue.Fields.TimeTracking.OriginalEstimateSeconds
	}
	if estimateSeconds > 0 && (task.EstimateSource == "" || task.EstimateSource == "jira") {
		hours := float64(estimateSeconds) / 3600
		days := hours / 8
		if task.EstimateHours != hours || task.EstimateDays != days || task.EstimateSource != "jira" {
			task.EstimateHours = hours
			task.EstimateDays = days
			task.EstimateSource = "jira"
			changed = true
		}
	}
	return changed
}

func jiraOriginRequirementID(issue telemetry.JiraIssue) string {
	if mapJiraIssueType(issue.Fields.IssueType.Name) != deliveryplanning.WorkItemBug {
		return ""
	}
	if issue.Fields.Parent != nil {
		if key := strings.TrimSpace(issue.Fields.Parent.Key); key != "" {
			return key
		}
	}
	candidates := make([]string, 0)
	appendCandidate := func(reference *telemetry.JiraIssueReference) {
		if reference == nil || mapJiraIssueType(reference.Fields.IssueType.Name) != deliveryplanning.WorkItemRequirement {
			return
		}
		key := strings.TrimSpace(reference.Key)
		if key != "" {
			candidates = append(candidates, key)
		}
	}
	for _, link := range issue.Fields.IssueLinks {
		appendCandidate(link.InwardIssue)
		appendCandidate(link.OutwardIssue)
	}
	candidates = normalizeJIRAScopeValues(candidates, true)
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

func (s *Server) appendJiraPerformanceEvents(issue telemetry.JiraIssue) (bool, error) {
	if s == nil || s.performance == nil || s.config == nil || !s.config.PerformanceBrain.Normalized().Enabled {
		return false, nil
	}
	issueType := mapJiraIssueType(issue.Fields.IssueType.Name)
	projectKey := deliveryplanning.NormalizeProjectKey(issue.Fields.Project.Key)
	legacyObservedState := map[string]any{
		"status": issue.Fields.Status.Name, "assignee": jiraIssueAssigneeName(issue),
		"priority": issue.Fields.Priority.Name, "due_date": issue.Fields.DueDate,
		"resolution_date": issue.Fields.ResolutionDate, "issue_type": issueType,
	}
	observedState := make(map[string]any, len(legacyObservedState)+1)
	for key, value := range legacyObservedState {
		observedState[key] = value
	}
	observedState["parent_work_item_id"] = jiraOriginRequirementID(issue)
	updatedAt := parseOptionalJiraTime(issue.Fields.Updated)
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	changed := false
	snapshotID := strings.TrimSpace(issue.Fields.Updated)
	if snapshotID == "" {
		snapshotID = updatedAt.UTC().Format(time.RFC3339Nano)
	}
	snapshotCommand := performance.SourceEventCommand{
		DedupeKey:  "jira:" + issue.Key + ":snapshot:" + snapshotID,
		WorkItemID: issue.Key, ProjectKey: projectKey, IssueType: issueType,
		EventType: performance.SourceEventIssueSnapshot, OccurredAt: updatedAt,
		SourceSystem: "jira", SourceEventID: issue.Key + ":" + snapshotID,
		Actor: "jira-sync", Payload: observedState,
	}
	result, err := s.performance.AppendSourceEvent(context.Background(), snapshotCommand)
	if errors.Is(err, performance.ErrSourceEventConflict) {
		// Snapshot payload v1 did not include parent_work_item_id. Retrying the
		// exact legacy payload turns an old immutable row into a valid replay,
		// while any unrelated payload conflict remains an error.
		snapshotCommand.Payload = legacyObservedState
		result, err = s.performance.AppendSourceEvent(context.Background(), snapshotCommand)
	}
	if err != nil {
		return false, err
	}
	changed = !result.Replayed
	for _, history := range issue.Changelog.Histories {
		occurredAt := parseOptionalJiraTime(history.Created)
		if occurredAt.IsZero() {
			continue
		}
		actor := strings.TrimSpace(history.Author.DisplayName)
		if actor == "" {
			actor = strings.TrimSpace(history.Author.Name)
		}
		for index, item := range history.Items {
			field := strings.ToLower(strings.TrimSpace(item.Field))
			eventType := ""
			switch field {
			case "assignee":
				eventType = performance.SourceEventAssigneeChange
			case "status":
				eventType = performance.SourceEventStatusChange
			case "duedate", "due date":
				eventType = performance.SourceEventDueDateChange
			case "priority", "severity":
				eventType = performance.SourceEventPriorityChange
			default:
				continue
			}
			sourceEventID := fmt.Sprintf("%s:%s:%d", issue.Key, history.ID, index)
			appended, appendErr := s.performance.AppendSourceEventAllowingActorDrift(context.Background(), performance.SourceEventCommand{
				DedupeKey: "jira:" + sourceEventID, WorkItemID: issue.Key,
				ProjectKey: projectKey, IssueType: issueType, EventType: eventType,
				FieldName: field, FromValue: item.FromString, ToValue: item.ToString,
				Actor: actor, OccurredAt: occurredAt, SourceSystem: "jira",
				SourceEventID: sourceEventID, Payload: map[string]any{
					"field_id": item.FieldID, "jira_history_id": history.ID,
				},
			})
			if appendErr != nil {
				return changed, appendErr
			}
			changed = changed || !appended.Replayed
		}
	}
	return changed, nil
}

func parseOptionalJiraDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil
	}
	return &parsed
}

func formatJiraProjectLabel(key, name string) string {
	key = strings.TrimSpace(key)
	name = strings.TrimSpace(name)
	if key == "" {
		return name
	}
	if name == "" || strings.EqualFold(name, key) {
		return key
	}
	return fmt.Sprintf("%s (%s)", name, key)
}

// mapJiraStatus maps general Jira issue statuses to our 4 Kanban stages
func mapJiraStatus(jiraStatus string) string {
	s := strings.ToLower(strings.TrimSpace(jiraStatus))
	switch s {
	case "to do", "backlog", "open", "reopened", "new", "todo":
		return "backlog"
	case "in progress", "progress", "active", "doing":
		return "progress"
	case "in review", "review", "under review", "qa", "testing":
		return "review"
	case "done", "closed", "resolved", "completed":
		return "done"
	default:
		return "backlog"
	}
}

// mapJiraIssueType converts Jira issue types into the app's planning model.
// In this deployment Jira Task/Story-like items are real demands; AI-generated
// shadow work remains `task` when it enters through the deconstruction import.
func mapJiraIssueType(jiraIssueType string) string {
	s := strings.ToLower(strings.TrimSpace(jiraIssueType))
	switch s {
	case "bug", "缺陷", "故障", "defect":
		return "bug"
	case "task", "任务", "story", "故事", "requirement", "需求", "feature", "epic":
		return "requirement"
	default:
		return "requirement"
	}
}

func (s *Server) reconcileJiraIssueVersions(issue telemetry.JiraIssue) {
	if s == nil || s.config == nil || !s.config.Jira.VersionCatalogEnabled || db.DB == nil {
		return
	}
	state, err := telemetry.JiraIssueVersionState(issue)
	if err != nil {
		log.Printf("Jira sync: failed to normalize version facts for %s: %v", issue.Key, err)
		return
	}
	if _, err := deliveryplanning.NewService(db.DB).ReconcileExternalIssue(context.Background(), state); err != nil {
		log.Printf("Jira sync: failed to reconcile version facts for %s: %v", issue.Key, err)
	}
}

func shouldPreserveLocalAssignee(task db.TaskTelemetry, incomingAssignee string, incomingStatus string, incomingUpdatedAt time.Time, now time.Time) bool {
	current := strings.TrimSpace(task.Assignee)
	incoming := strings.TrimSpace(incomingAssignee)
	if current == "" || strings.EqualFold(current, incoming) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(incomingStatus), "done") {
		return false
	}
	if !incomingUpdatedAt.IsZero() && incomingUpdatedAt.After(task.LastUpdate) {
		return false
	}
	if task.LastUpdate.IsZero() || now.Sub(task.LastUpdate) > 24*time.Hour {
		return false
	}

	logs := strings.TrimSpace(task.DecisionLogs)
	if logs == "" {
		return false
	}

	return strings.Contains(logs, "转派") ||
		strings.Contains(logs, "调整需求负责人") ||
		strings.Contains(logs, "调停干预")
}

func shouldIncludeInJiraKeepAlive(task db.TaskTelemetry, now time.Time) bool {
	if !strings.EqualFold(strings.TrimSpace(task.Status), "done") {
		return true
	}
	if task.LastUpdate.IsZero() {
		return false
	}
	return now.Sub(task.LastUpdate) <= jiraCompletedKeepAliveWindow
}

// buildJQL constructs a JQL query string from the Jira sync configuration
func buildJQL(cfg *config.JiraConfig) string {
	var ordinaryJQL string
	if strings.TrimSpace(cfg.CustomJQL) != "" {
		ordinaryJQL = strings.TrimSpace(cfg.CustomJQL)
	} else {
		var parts []string

		// Filter by projects
		if len(cfg.SyncProjects) > 0 {
			var quotedProjects []string
			for _, p := range cfg.SyncProjects {
				if p = strings.TrimSpace(p); p != "" {
					quotedProjects = append(quotedProjects, fmt.Sprintf("%q", p))
				}
			}
			if len(quotedProjects) > 0 {
				parts = append(parts, fmt.Sprintf("project in (%s)", strings.Join(quotedProjects, ", ")))
			}
		}

		// Filter by assignees
		if len(cfg.SyncUsers) > 0 {
			var quotedUsers []string
			for _, u := range cfg.SyncUsers {
				if u = strings.TrimSpace(u); u != "" {
					quotedUsers = append(quotedUsers, fmt.Sprintf("%q", u))
				}
			}
			if len(quotedUsers) > 0 {
				parts = append(parts, fmt.Sprintf("assignee in (%s)", strings.Join(quotedUsers, ", ")))
			}
		}

		// Filter by status/progress
		if len(cfg.SyncStatuses) > 0 {
			var quotedStatuses []string
			for _, st := range cfg.SyncStatuses {
				if st = strings.TrimSpace(st); st != "" {
					quotedStatuses = append(quotedStatuses, fmt.Sprintf("%q", st))
				}
			}
			if len(quotedStatuses) > 0 {
				parts = append(parts, fmt.Sprintf("status in (%s)", strings.Join(quotedStatuses, ", ")))
			}
		}

		ordinaryJQL = strings.Join(parts, " AND ")
	}

	versionJQL := buildJiraVersionJQL(cfg)
	switch {
	case ordinaryJQL != "" && versionJQL != "":
		return fmt.Sprintf("(%s) OR %s", ordinaryJQL, versionJQL)
	case versionJQL != "":
		return versionJQL
	default:
		return ordinaryJQL
	}
}

func buildJiraVersionJQL(cfg *config.JiraConfig) string {
	refs := config.JiraVersionReferences(cfg)
	clauses := make([]string, 0, len(refs))
	for _, ref := range refs {
		clauses = append(clauses, fmt.Sprintf(`(project = %q AND fixVersion = %s)`, ref.ProjectKey, ref.VersionID))
	}
	return strings.Join(clauses, " OR ")
}

// parseJiraTime parses standard Jira timestamps into time.Time
func parseJiraTime(timeStr string) time.Time {
	if parsed := parseOptionalJiraTime(timeStr); !parsed.IsZero() {
		return parsed
	}
	return time.Now()
}

func parseOptionalJiraTime(timeStr string) time.Time {
	if strings.TrimSpace(timeStr) == "" {
		return time.Time{}
	}
	// Try RFC3339 format
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t
	}
	// Try Jira standard format: "2006-01-02T15:04:05.000-0700"
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", timeStr); err == nil {
		return t
	}
	return time.Time{}
}

// syncAssigneeToJira updates the assignee in Jira. It runs asynchronously to avoid blocking the API response.
func (s *Server) syncAssigneeToJira(issueKey string, localAssignee string) {
	if !s.config.Jira.Enabled {
		return
	}
	log.Printf("Jira sync: attempting to sync assignee override for %s -> %s", issueKey, localAssignee)
	jc := telemetry.NewJiraClient(&s.config.Jira)
	jiraUser, resolveErr := resolveJiraAssigneeUsername(localAssignee)
	if resolveErr != nil {
		log.Printf("Jira sync WARNING: %v", resolveErr)
		return
	}
	err := jc.UpdateAssignee(issueKey, jiraUser)
	if err != nil {
		log.Printf("Jira sync: failed to sync assignee for %s to Jira: %v", issueKey, err)
	} else {
		log.Printf("Jira sync: successfully synced assignee for %s to Jira (%s)", issueKey, jiraUser)
	}
}

func resolveJiraAssigneeUsername(localAssignee string) (string, error) {
	localAssignee = strings.TrimSpace(localAssignee)
	if localAssignee == "" || localAssignee == "未指派" || localAssignee == "-" || strings.EqualFold(localAssignee, "Unassigned") {
		return "", nil
	}

	var user userdb.User
	if err := db.DB.Where("name = ? OR username = ? OR email = ?", localAssignee, localAssignee, localAssignee).First(&user).Error; err == nil {
		jiraUser := strings.TrimSpace(user.Username)
		if idx := strings.Index(jiraUser, "@"); idx > 0 {
			jiraUser = jiraUser[:idx]
		}
		if jiraUser == "" {
			return "", fmt.Errorf("cannot map local assignee %q to a Jira username", localAssignee)
		}
		return jiraUser, nil
	}
	for _, char := range localAssignee {
		if char >= 0x4e00 && char <= 0x9fff {
			return "", fmt.Errorf("cannot map local assignee %q to a valid Jira username", localAssignee)
		}
	}
	return localAssignee, nil
}

func (s *Server) syncDueDateToJira(issueKey, dueDate string) {
	if !s.config.Jira.Enabled || strings.TrimSpace(dueDate) == "" {
		return
	}
	log.Printf("Jira sync: attempting to sync due date for %s -> %s", issueKey, dueDate)
	jc := telemetry.NewJiraClient(&s.config.Jira)
	if err := jc.UpdateDueDate(issueKey, dueDate); err != nil {
		log.Printf("Jira sync: failed to sync due date for %s to Jira: %v", issueKey, err)
		return
	}
	log.Printf("Jira sync: successfully synced due date for %s to Jira (%s)", issueKey, dueDate)
}

func (s *Server) syncDecisionCommentToJira(issueKey, comment string) {
	comment = strings.TrimSpace(comment)
	if !s.config.Jira.Enabled || comment == "" {
		return
	}
	log.Printf("Jira sync: attempting to add decision comment for %s", issueKey)
	jc := telemetry.NewJiraClient(&s.config.Jira)
	if err := jc.AddComment(issueKey, comment); err != nil {
		log.Printf("Jira sync: failed to add decision comment for %s: %v", issueKey, err)
		return
	}
	log.Printf("Jira sync: successfully added decision comment for %s", issueKey)
}

// syncJiraUserToLocal automatically upserts the user info retrieved from Jira into the local users table.
func (s *Server) syncJiraUserToLocal(username, displayName, email string) {
	if username == "" && email == "" {
		return
	}

	var user userdb.User
	// 尝试通过 Username 或 Email 查询
	err := db.DB.Where("username = ? OR email = ?", username, email).First(&user).Error
	if err != nil {
		// 未找到，新建用户映射
		dbEmail := email
		if dbEmail == "" {
			dbEmail = username + "@westwell-lab.com"
		}
		dbUsername := username
		if dbUsername == "" {
			dbUsername = email
		}
		dbName := displayName
		if dbName == "" {
			dbName = username
		}

		newUser := userdb.User{
			Username:   dbUsername,
			Email:      dbEmail,
			Name:       dbName,
			Department: "未分配",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		if err := db.DB.Create(&newUser).Error; err != nil {
			log.Printf("Jira sync user: failed to create user mapping for %s: %v", username, err)
		} else {
			log.Printf("Jira sync user: automatically created local user mapping: %s (%s)", dbName, dbUsername)
		}
	} else {
		// 已存在，如果显示名有变化则同步更新
		if displayName != "" && user.Name != displayName {
			user.Name = displayName
			user.UpdatedAt = time.Now()
			if err := db.DB.Save(&user).Error; err == nil {
				log.Printf("Jira sync user: updated local user name mapping for %s: %s -> %s", username, user.Name, displayName)
			}
		}
	}
}

// syncJiraComments pulls comments for a Jira issue and saves them locally.
func (s *Server) syncJiraComments(jc *telemetry.JiraClient, issueKey string) {
	if _, err := s.syncJiraCommentsForIssue(jc, issueKey, time.Time{}); err != nil {
		log.Printf("Jira sync comments: failed to reconcile comments for %s: %v", issueKey, err)
	}
}

func (s *Server) syncJiraCommentsForIssue(jc *telemetry.JiraClient, issueKey string, issueUpdatedAt time.Time) (bool, error) {
	if jc == nil || issueKey == "" {
		return false, nil
	}

	var syncState db.JiraIssueSyncState
	stateLookup := db.DB.Where("task_id = ?", issueKey).Limit(1).Find(&syncState)
	if stateLookup.Error != nil {
		return false, fmt.Errorf("load Jira comment sync state for %s: %w", issueKey, stateLookup.Error)
	}
	if stateLookup.RowsAffected > 0 && !issueUpdatedAt.IsZero() && !syncState.LastSucceededAt.IsZero() &&
		!syncState.SourceUpdatedAt.Before(issueUpdatedAt) && strings.TrimSpace(syncState.LastError) == "" {
		return false, nil
	}

	comments, err := jc.GetComments(issueKey)
	if err != nil {
		recordJiraIssueSyncFailure(syncState, issueKey, err)
		return false, fmt.Errorf("fetch Jira comments for %s: %w", issueKey, err)
	}

	seenCommentIDs := make([]string, 0, len(comments))
	seenCommentIDSet := make(map[string]struct{}, len(comments))
	hasEligibleSolutionSource := false
	changed := false
	projectionErrors := make([]error, 0)
	for _, c := range comments {
		seenCommentIDs = append(seenCommentIDs, c.ID)
		seenCommentIDSet[c.ID] = struct{}{}
		authorName := strings.TrimSpace(c.Author.DisplayName)
		if authorName == "" {
			authorName = "Unknown"
		}
		commentUpdatedAt := optionalJiraTimePointer(c.Updated)
		var existing db.JiraCommentLog
		commentErr := db.DB.Where("comment_id = ?", c.ID).First(&existing).Error
		if errors.Is(commentErr, gorm.ErrRecordNotFound) {
			createdTime := parseJiraTime(c.Created)
			newComment := db.JiraCommentLog{
				TaskID:          issueKey,
				CommentID:       c.ID,
				Author:          authorName,
				Body:            c.Body,
				Current:         true,
				CreatedAt:       createdTime,
				SourceUpdatedAt: commentUpdatedAt,
			}
			if createErr := db.DB.Create(&newComment).Error; createErr != nil {
				projectionErrors = append(projectionErrors, fmt.Errorf("save Jira comment %s for %s: %w", c.ID, issueKey, createErr))
			} else {
				changed = true
			}
		} else if commentErr != nil {
			projectionErrors = append(projectionErrors, fmt.Errorf("load Jira comment %s for %s: %w", c.ID, issueKey, commentErr))
		} else {
			commentChanged := existing.TaskID != issueKey || existing.Body != c.Body || existing.Author != authorName ||
				!existing.Current || !optionalTimesEqual(existing.SourceUpdatedAt, commentUpdatedAt)
			if commentChanged {
				existing.TaskID = issueKey
				existing.Body = c.Body
				existing.Author = authorName
				existing.Current = true
				existing.SourceUpdatedAt = commentUpdatedAt
				if saveErr := db.DB.Save(&existing).Error; saveErr != nil {
					projectionErrors = append(projectionErrors, fmt.Errorf("refresh Jira comment %s for %s: %w", c.ID, issueKey, saveErr))
				} else {
					changed = true
				}
			}
		}

		marker, eligible := jiraSolutionCommentMarker(c.Body, c.Author.DisplayName)
		if strings.Contains(c.Body, "<!-- WELL_AMBIENT_SOLUTION_LINK:") {
			eligible = false
			marker = "machine_link"
		}
		_, observeErr := s.solutions.ObserveSource(context.Background(), solutions.ObserveSourceCommand{
			DemandID: issueKey, SourceSystem: "jira", ExternalID: c.ID,
			Author: c.Author.DisplayName, Marker: marker, Eligible: eligible, Body: c.Body,
			SourceCreatedAt: optionalJiraTimePointer(c.Created), SourceUpdatedAt: optionalJiraTimePointer(c.Updated), Actor: "jira-sync",
		})
		if observeErr != nil {
			projectionErrors = append(projectionErrors, fmt.Errorf("persist solution source %s for %s: %w", c.ID, issueKey, observeErr))
			continue
		}
		if eligible {
			hasEligibleSolutionSource = true
		}
	}

	var currentComments []db.JiraCommentLog
	if currentErr := db.DB.Where("task_id = ? AND current = ?", issueKey, true).Find(&currentComments).Error; currentErr != nil {
		projectionErrors = append(projectionErrors, fmt.Errorf("load current Jira comments for %s: %w", issueKey, currentErr))
	} else {
		for index := range currentComments {
			if _, exists := seenCommentIDSet[currentComments[index].CommentID]; exists {
				continue
			}
			currentComments[index].Current = false
			if saveErr := db.DB.Save(&currentComments[index]).Error; saveErr != nil {
				projectionErrors = append(projectionErrors, fmt.Errorf("retire removed Jira comment %s for %s: %w", currentComments[index].CommentID, issueKey, saveErr))
			} else {
				changed = true
			}
		}
	}
	if reconcileErr := s.solutions.ReconcileSourceSet(context.Background(), issueKey, "jira", seenCommentIDs); reconcileErr != nil {
		projectionErrors = append(projectionErrors, fmt.Errorf("reconcile removed Jira comments for %s: %w", issueKey, reconcileErr))
	}
	if hasEligibleSolutionSource {
		var demand db.TaskTelemetry
		if demandErr := db.DB.Where("task_id = ?", issueKey).First(&demand).Error; demandErr != nil {
			projectionErrors = append(projectionErrors, fmt.Errorf("load Jira demand %s for solution draft: %w", issueKey, demandErr))
		} else if _, _, draftErr := s.solutions.RequestInitialDraft(context.Background(), solutions.RequestInitialDraftCommand{
			DemandID: issueKey, ProjectKey: demand.ProjectKey, Title: demand.Title,
			Markdown: solutionInitialMarkdown(demand), RequestedBy: "jira-sync",
		}); draftErr != nil && !errors.Is(draftErr, solutions.ErrConflict) {
			projectionErrors = append(projectionErrors, fmt.Errorf("queue initial solution draft for %s: %w", issueKey, draftErr))
		}
	}

	if len(projectionErrors) > 0 {
		joined := errors.Join(projectionErrors...)
		recordJiraIssueSyncFailure(syncState, issueKey, joined)
		return changed, joined
	}
	now := time.Now()
	syncState.TaskID = issueKey
	if !issueUpdatedAt.IsZero() {
		syncState.SourceUpdatedAt = issueUpdatedAt
	}
	syncState.LastSucceededAt = now
	syncState.LastError = ""
	syncState.UpdatedAt = now
	if saveErr := db.DB.Save(&syncState).Error; saveErr != nil {
		return changed, fmt.Errorf("save Jira comment sync state for %s: %w", issueKey, saveErr)
	}
	return changed, nil
}

func optionalTimesEqual(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func recordJiraIssueSyncFailure(state db.JiraIssueSyncState, issueKey string, syncErr error) {
	state.TaskID = issueKey
	state.LastError = syncErr.Error()
	if len(state.LastError) > 4000 {
		state.LastError = state.LastError[:4000]
	}
	state.UpdatedAt = time.Now()
	if saveErr := db.DB.Save(&state).Error; saveErr != nil {
		log.Printf("Jira sync comments: failed to persist error state for %s: %v", issueKey, saveErr)
	}
}

func jiraSolutionCommentMarker(body, author string) (string, bool) {
	trimmed := strings.TrimSpace(body)
	markers := []string{"[方案]", "【方案】", "# 方案", "## 方案", "<!-- WELL_AMBIENT_SOLUTION -->"}
	for _, marker := range markers {
		if strings.HasPrefix(trimmed, marker) || strings.Contains(trimmed, "\n"+marker) {
			return marker, true
		}
	}
	switch strings.ToLower(strings.TrimSpace(author)) {
	case "jira公用-解决方案", "jira公用账号-解决方案", "jira公用账户-解决方案":
		return "author:jira公用-解决方案", true
	}
	return "", false
}

func optionalJiraTimePointer(value string) *time.Time {
	parsed := parseOptionalJiraTime(value)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}
