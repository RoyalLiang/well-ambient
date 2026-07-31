package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/agenda"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"
	"well-ambient/internal/server/authz"
	"well-ambient/internal/telemetry"
)

// Server encapsulates the HTTP server logic
type Server struct {
	config           *config.Config
	configPath       string
	mux              *http.ServeMux
	jiraAssigneeSync func(issueKey, assignee string)
	jiraDueDateSync  func(issueKey, dueDate string)
	jiraCommentSync  func(issueKey, comment string)
	jiraReleaseList  func(ctx context.Context, projectKey string) ([]deliveryplanning.ExternalRelease, error)
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, configPath string) *Server {
	s := &Server{
		config:     cfg,
		configPath: configPath,
		mux:        http.NewServeMux(),
	}
	s.jiraAssigneeSync = s.syncAssigneeToJira
	s.jiraDueDateSync = s.syncDueDateToJira
	s.jiraCommentSync = s.syncDecisionCommentToJira
	s.jiraReleaseList = func(ctx context.Context, projectKey string) ([]deliveryplanning.ExternalRelease, error) {
		return telemetry.NewJiraClient(&s.config.Jira).ListProjectReleases(ctx, projectKey)
	}
	s.routes()

	// Bind the telemetry notification broadcast callback to avoid import cycles
	telemetry.OnNotificationBroadcast = BroadcastNotifications
	telemetry.OnTelemetryBroadcast = BroadcastTelemetryUpdated

	return s
}

// routes sets up API routing
// routes sets up API routing
func (s *Server) routes() {
	// Status and Health Check (No Auth)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)

	// Auth APIs (No Auth)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)

	// GitLab Webhook Entry Point (No Auth)
	s.mux.HandleFunc("POST /api/webhook/gitlab", s.handleGitLabWebhook)

	// Protected Task and Log Queries
	s.mux.HandleFunc("GET /api/tasks", s.withAuth(s.withLegacyDeliveryAPI(legacyTasksReadEndpoint, s.handleGetTasks)))
	s.mux.HandleFunc("GET /api/tasks/commits", s.withAuth(s.handleGetTaskCommits))
	s.mux.HandleFunc("GET /api/execution/tasks", s.withPermission("dashboard:read", s.handleGetExecutionTasks))
	s.mux.HandleFunc("GET /api/delivery/directory", s.withAuth(s.handleGetDeliveryDirectory))
	s.mux.HandleFunc("GET /api/task-tracking/assignees", s.withPermission("delivery:read", s.handleGetTaskTrackingAssignees))
	s.mux.HandleFunc("GET /api/strongest-brain/evidence-chain", s.withPermission("dashboard:read", s.handleGetStrongestBrainEvidenceChain))
	s.mux.HandleFunc("GET /api/strongest-brain/delivery-cockpit", s.withPermission("decision:read", s.handleGetStrongestBrainDeliveryCockpit))
	s.mux.HandleFunc("GET /api/strongest-brain/decision-queue", s.withPermission("decision:read", s.handleGetStrongestBrainDecisionQueue))
	s.mux.HandleFunc("GET /api/strongest-brain/exceptions", s.withPermission("decision:read", s.handleGetStrongestBrainExceptions))
	s.mux.HandleFunc("GET /api/strongest-brain/weekly-decisions", s.withPermission("decision:read", s.handleGetStrongestBrainWeeklyDecisions))
	s.mux.HandleFunc("GET /api/strongest-brain/demand-readiness", s.withPermission("demands:read", s.handleGetStrongestBrainDemandReadiness))
	s.mux.HandleFunc("GET /api/strongest-brain/override-audit", s.withPermission("decision:read", s.handleGetStrongestBrainOverrideAudit))
	s.mux.HandleFunc("GET /api/strongest-brain/ai-traces", s.withPermission("ai_context:read", s.handleGetStrongestBrainAITraces))
	s.mux.HandleFunc("POST /api/strongest-brain/intervention", s.withPermission("demands:write", s.handleStrongestBrainIntervention))
	s.mux.HandleFunc("GET /api/decision/daily-jira", s.withPermission("decision:read", s.handleGetDailyJiraAudit))
	s.mux.HandleFunc("POST /api/decision/daily-jira/review", s.withPermission("decision:read", s.withPermission("demands:write", s.handlePostDailyJiraReview)))
	s.mux.HandleFunc("GET /api/logs", s.withAuth(s.handleGetLogs))

	// Protected Config APIs
	s.mux.HandleFunc("GET /api/config", s.withPermission("config:read", s.handleGetConfig))
	s.mux.HandleFunc("GET /api/jira/link-config", s.withAuth(s.handleGetJiraLinkConfig))
	s.mux.HandleFunc("POST /api/config", s.withPermission("config:write", s.handleSaveConfig))
	s.mux.HandleFunc("POST /api/config/test", s.withPermission("config:write", s.handleTestConnection))
	s.mux.HandleFunc("GET /api/config/versions", s.withPermission("config:read", s.handleListConfigVersions))
	s.mux.HandleFunc("POST /api/config/versions/{id}/rollback", s.withPermission("config:write", s.handleRollbackConfigVersion))
	s.mux.HandleFunc("GET /api/gitlab/projects", s.withPermission("config:write", s.handleGetGitLabProjects))
	s.mux.HandleFunc("POST /api/gitlab/webhooks/ensure", s.withPermission("config:write", s.handleEnsureGitLabWebhooks))
	s.mux.HandleFunc("GET /api/gitlab/webhooks/status", s.withPermission("config:write", s.handleGetGitLabWebhookStatus))

	// Protected User, Groups, and RBAC APIs
	s.mux.HandleFunc("GET /api/me", s.withAuth(s.handleGetCurrentUser))
	s.mux.HandleFunc("GET /api/me/project-preferences", s.withAuth(s.handleGetProjectPreferences))
	s.mux.HandleFunc("PUT /api/me/project-preferences", s.withAuth(s.handleUpdateProjectPreferences))
	s.mux.HandleFunc("GET /api/me/decision-table-columns", s.withAuth(s.handleGetDecisionTableColumns))
	s.mux.HandleFunc("PUT /api/me/decision-table-columns", s.withAuth(s.handleUpdateDecisionTableColumns))
	s.mux.HandleFunc("GET /api/users", s.withPermission("users:read", s.handleGetUsers))
	s.mux.HandleFunc("POST /api/users/groups", s.withPermission("users:write", s.handleUpdateUserGroups))
	s.mux.HandleFunc("POST /api/users/transfer-admin", s.withPermission("users:transfer_super_admin", s.handleTransferAdmin))

	s.mux.HandleFunc("GET /api/groups", s.withPermission("users:read", s.handleGetGroups))
	s.mux.HandleFunc("GET /api/permissions", s.withPermission("users:read", s.handleListPermissions))
	s.mux.HandleFunc("POST /api/groups", s.withPermission("users:write", s.handleCreateGroup))
	s.mux.HandleFunc("DELETE /api/groups/{name}", s.withPermission("users:write", s.handleDeleteGroup))
	s.mux.HandleFunc("POST /api/groups/permissions", s.withPermission("users:write", s.handleSaveGroupPermissions))

	s.mux.HandleFunc("GET /api/audit-logs", s.withPermission("users:read", s.handleGetAuditLogs))
	s.mux.HandleFunc("POST /api/users/department", s.withPermission("users:write", s.handleUpdateUserDepartment))

	// Protected KPI Performance API
	s.mux.HandleFunc("GET /api/kpi/performance", s.withPermission("kpi:read", s.handleGetKPIPerformance))
	s.mux.HandleFunc("GET /api/kpi/report-preview", s.withPermission("kpi:read", s.handleGetKPIReportPreview))

	// Protected Demand & Schedule APIs
	s.mux.HandleFunc("GET /api/demands/options", s.withAuth(s.handleGetDemandOptions))
	s.mux.HandleFunc("POST /api/demands", s.withPermission("demands:write", s.handleCreateDemand))
	s.mux.HandleFunc("DELETE /api/demands", s.withAuth(s.handleDeleteDemand))
	s.mux.HandleFunc("POST /api/demands/archive", s.withAuth(s.handleArchiveDemand))
	s.mux.HandleFunc("POST /api/demands/reassign", s.withAuth(s.handleReassignDemand))
	s.mux.HandleFunc("GET /api/schedule", s.withPermission("demands:read", s.withLegacyDeliveryAPI(legacyScheduleReadEndpoint, s.handleGetSchedule)))
	s.mux.HandleFunc("GET /api/schedule/risk-calendar", s.withPermission("demands:read", s.handleGetScheduleRiskCalendar))
	s.mux.HandleFunc("POST /api/tasks/schedule", s.withAuth(s.withLegacyDeliveryAPI(legacyScheduleWriteEndpoint, s.handleScheduleTask)))
	s.mux.HandleFunc("GET /api/demand-specs", s.withPermission("demand_spec:read", s.handleListDemandSpecs))
	s.mux.HandleFunc("POST /api/demand-specs", s.withPermission("demand_spec:write", s.handleSaveDemandSpec))
	s.mux.HandleFunc("POST /api/demand-specs/ai-stream", s.withPermission("demand_spec:write", s.handleStreamAIDemandSpec))
	s.mux.HandleFunc("DELETE /api/demand-specs/{id}", s.withPermission("demand_spec:write", s.handleDeleteDemandSpecDraft))
	s.mux.HandleFunc("POST /api/demand-specs/{id}/freeze", s.withPermission("demand_spec:freeze", s.handleFreezeDemandSpec))
	s.mux.HandleFunc("GET /api/review-contracts", s.withPermission("demand_spec:read", s.handleGetReviewContract))
	s.mux.HandleFunc("POST /api/review-contracts", s.withPermission("review_contract:manage", s.handleSaveReviewContract))
	s.mux.HandleFunc("POST /api/review-contracts/{id}/approve", s.withPermission("review_contract:manage", s.handleApproveReviewContract))
	s.mux.HandleFunc("POST /api/execution/preflight", s.withPermission("execution:preflight", s.handleExecutionPreflight))
	s.mux.HandleFunc("POST /api/execution/change-set/generate", s.withPermission("execution:preflight", s.handleGenerateExecutionChangeSet))
	s.mux.HandleFunc("GET /api/execution/runs", s.withPermission("demand_spec:read", s.handleListExecutionRuns))
	s.mux.HandleFunc("POST /api/execution/runs", s.withPermission("execution:preflight", s.handleCreateExecutionRun))
	s.mux.HandleFunc("POST /api/execution/runs/{id}/start", s.withPermission("execution:start", s.handleStartExecutionRun))
	s.mux.HandleFunc("POST /api/execution/runs/{id}/cancel", s.withPermission("execution:cancel", s.handleCancelExecutionRun))
	s.mux.HandleFunc("POST /api/execution/runs/{id}/refresh", s.withPermission("execution:preflight", s.handleRefreshExecutionRun))
	s.mux.HandleFunc("POST /api/execution/runs/{id}/verify", s.withPermission("execution:accept", s.handleVerifyExecutionRun))
	s.mux.HandleFunc("GET /api/corpus-candidates", s.withPermission("corpus_candidate:read", s.handleListCorpusCandidates))
	s.mux.HandleFunc("POST /api/corpus-candidates/{id}/review", s.withPermission("corpus_candidate:review", s.handleReviewCorpusCandidate))
	s.mux.HandleFunc("GET /api/corpus-candidates/{id}/impact", s.withPermission("corpus_candidate:read", s.withPermission("ai_context:preview", s.handlePreviewCorpusCandidateImpact)))
	s.mux.HandleFunc("POST /api/corpus-candidates/{id}/publish", s.withPermission("corpus_candidate:review", s.withPermission("ai_context:preview", s.handlePublishCorpusCandidate)))

	// Protected Project Configs & Brain Scores
	s.mux.HandleFunc("GET /api/projects/config", s.withAuth(s.handleGetProjectConfigs))
	s.mux.HandleFunc("POST /api/projects/config", s.withPermission("config:write", s.handleSaveProjectConfig))
	s.mux.HandleFunc("GET /api/projects/scores", s.withAuth(s.handleGetProjectScores))
	s.mux.HandleFunc("POST /api/projects/scores/calculate", s.withPermission("config:write", s.handleCalculateProjectScores))
	s.mux.HandleFunc("GET /api/projects/{project_key}/releases", s.withPermission("delivery:read", s.handleListProjectReleases))
	s.mux.HandleFunc("POST /api/projects/{project_key}/releases/sync", s.withPermission("release:manage", s.handleSyncProjectReleases))
	s.mux.HandleFunc("POST /api/projects/{project_key}/releases", s.withPermission("release:manage", s.handleCreateProjectRelease))
	s.mux.HandleFunc("GET /api/releases", s.withPermission("delivery:read", s.handleListReleases))
	s.mux.HandleFunc("POST /api/releases", s.withPermission("release:manage", s.handleCreateRelease))
	s.mux.HandleFunc("GET /api/releases/jira-search", s.withPermission("release:manage", s.handleSearchJiraReleases))
	s.mux.HandleFunc("GET /api/releases/{id}", s.withPermission("delivery:read", s.handleGetRelease))
	s.mux.HandleFunc("PATCH /api/releases/{id}", s.withPermission("release:manage", s.handlePatchRelease))
	s.mux.HandleFunc("GET /api/releases/{id}/jira-issues", s.withPermission("delivery:read", s.handleListReleaseJiraIssues))
	s.mux.HandleFunc("POST /api/releases/{id}/jira-issues/bulk", s.withPermission("release:manage", s.handleBulkAddReleaseJiraIssues))
	s.mux.HandleFunc("DELETE /api/releases/{id}/jira-issues/{work_item_id}", s.withPermission("release:manage", s.handleDeleteReleaseJiraIssue))
	s.mux.HandleFunc("PUT /api/releases/{id}/jira-link", s.withPermission("release:manage", s.handlePutReleaseJiraLink))
	s.mux.HandleFunc("DELETE /api/releases/{id}/jira-link", s.withPermission("release:manage", s.handleDeleteReleaseJiraLink))
	s.mux.HandleFunc("GET /api/releases/{id}/snapshot", s.withPermission("delivery:read", s.handleGetReleaseSnapshot))
	s.mux.HandleFunc("GET /api/work-items", s.withPermission("delivery:read", s.handleListWorkItems))
	s.mux.HandleFunc("GET /api/work-items/{id}", s.withPermission("delivery:read", s.handleGetWorkItem))
	s.mux.HandleFunc("PATCH /api/work-items/{id}/planning", s.withPermission("delivery:plan", s.handlePatchWorkItemPlanning))
	s.mux.HandleFunc("POST /api/work-items/bulk-planning", s.withPermission("delivery:plan", s.handleBulkWorkItemPlanning))
	s.mux.HandleFunc("GET /api/delivery/exceptions", s.withPermission("decision:read", s.handleGetDeliveryExceptions))
	s.mux.HandleFunc("GET /api/delivery/quality", s.withPermission("decision:read", s.handleGetDeliveryQuality))

	// Protected AI Deconstructor API
	s.mux.HandleFunc("POST /api/deconstruct", s.withAuth(s.handleDeconstruct))
	s.mux.HandleFunc("POST /api/ai/intent", s.withAuth(s.handleAIIntentSummary))
	s.mux.HandleFunc("POST /api/ai/intent-summary", s.withAuth(s.handleAIIntentSummary))
	s.mux.HandleFunc("POST /api/ai/assistant/summary", s.withAuth(s.handleAIIntentSummary))
	s.mux.HandleFunc("GET /api/ai/output-trace", s.withPermission("ai_context:read", s.handleGetAIOutputTrace))
	s.mux.HandleFunc("GET /api/ai/traces", s.withPermission("ai_context:read", s.handleGetAIOutputTrace))
	s.mux.HandleFunc("GET /api/ai/requirement-clarification", s.withPermission("ai_context:read", s.handleGetRequirementClarification))
	s.mux.HandleFunc("GET /api/requirements/clarification", s.withPermission("ai_context:read", s.handleGetRequirementClarification))
	s.mux.HandleFunc("GET /api/context/pack/replay", s.withPermission("ai_context:read", s.handleGetContextPackReplay))
	s.mux.HandleFunc("GET /api/context/packs/{id}/replay", s.withPermission("ai_context:read", s.handleGetContextPackReplay))
	s.mux.HandleFunc("POST /api/tasks/import", s.withAuth(s.handleImportTasks))
	s.mux.HandleFunc("GET /api/context/facts", s.withPermission("ai_context:read", s.handleListContextFacts))
	s.mux.HandleFunc("POST /api/context/facts", s.withPermission("ai_context:write", s.handleSaveContextFact))
	s.mux.HandleFunc("PUT /api/context/facts", s.withPermission("ai_context:write", s.handleSaveContextFact))
	s.mux.HandleFunc("GET /api/context/documents", s.withPermission("ai_context:read", s.handleListContextDocuments))
	s.mux.HandleFunc("GET /api/context/documents/{id}", s.withPermission("ai_context:read", s.handleGetContextDocument))
	s.mux.HandleFunc("POST /api/context/documents/import", s.withPermission("ai_context:write", s.handleImportContextDocument))
	s.mux.HandleFunc("POST /api/context/documents/{id}/archive", s.withPermission("ai_context:write", s.handleArchiveContextDocument))
	s.mux.HandleFunc("POST /api/context/pack/preview", s.withPermission("ai_context:preview", s.handlePreviewContextPack))

	// Protected Policy Authorization APIs
	s.mux.HandleFunc("GET /api/authz/policies", s.withPermission("policies:read", s.handleListAuthorizationPolicies))
	s.mux.HandleFunc("POST /api/authz/policies", s.withPermission("policies:write", s.handleSaveAuthorizationPolicy))
	s.mux.HandleFunc("POST /api/authz/explain", s.withPermission("policies:read", s.handleExplainAuthorization))
	s.mux.HandleFunc("GET /api/authz/audit-logs", s.withPermission("authorization_audit:read", s.handleListAuthorizationAuditLogs))

	// Protected Notification SSE API
	s.mux.HandleFunc("GET /api/notifications/sse", s.withAuth(s.handleNotificationsSSE))
	s.mux.HandleFunc("POST /api/notifications/read", s.withAuth(s.handleMarkNotificationRead))

	// Protected Phase 4: Agenda and Decision APIs
	s.mux.HandleFunc("GET /api/agenda/summary", s.withAuth(agenda.HandleGetAgendaSummary))
	s.mux.HandleFunc("POST /api/agenda/decision", s.withAuth(agenda.HandlePostAgendaDecision))
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header or URL query parameter
		authHeader := r.Header.Get("Authorization")
		tokenStr := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			tokenStr = r.URL.Query().Get("token")
		}

		if tokenStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized", "message":"Missing authorization token"}`))
			return
		}

		claims, err := ParseJWT(tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(fmt.Sprintf(`{"error":"unauthorized", "message":"Invalid or expired token: %v"}`, err)))
			return
		}

		// Inject user info into headers
		r.Header.Set("x-authenticated-user-id", claims.UserID)
		r.Header.Set("x-authenticated-user-name", claims.Name)
		r.Header.Set("x-authenticated-user-avatar", claims.Avatar)
		r.Header.Set("x-authenticated-user-department", claims.Department)

		next(w, r)
	}
}

// withPermission wraps a handler with RBAC permission check, supporting project-scoped checks
func (s *Server) withPermission(requiredPermission string, next http.HandlerFunc) http.HandlerFunc {
	return s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.Header.Get("x-authenticated-user-id")
		if userIDStr == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","message":"Missing user identity"}`))
			return
		}

		decision := authz.NewEvaluator(db.DB).Authorize(
			r.Context(),
			authz.Subject{Username: userIDStr},
			requiredPermission,
			authorizationResourceFromRequest(requiredPermission, r),
		)
		if !decision.Allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			if decision.MissingPermission == authz.MissingSubject {
				w.Write([]byte(`{"error":"forbidden","message":"User not registered"}`))
				return
			}
			w.Write([]byte(`{"error":"forbidden","message":"You do not have permission to perform this action"}`))
			return
		}

		next(w, r)
	})
}

func authorizationResourceFromRequest(requiredPermission string, r *http.Request) authz.Resource {
	repoName := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repoName == "" {
		repoName = strings.TrimSpace(r.URL.Query().Get("project_id"))
	}

	resource := authz.Resource{
		Type:        resourceTypeForPermission(requiredPermission),
		Scope:       "global",
		RequestPath: r.URL.Path,
		IPAddress:   r.RemoteAddr,
	}
	if repoName != "" {
		resource.Type = "repo"
		resource.ID = repoName
		resource.Scope = "repo"
		resource.ScopeID = repoName
	}
	return resource
}

func resourceTypeForPermission(permission string) string {
	switch strings.SplitN(permission, ":", 2)[0] {
	case "config":
		return "config"
	case "users":
		return "user"
	case "demands", "demand_spec", "review_contract", "execution":
		return "demand"
	case "corpus_candidate":
		return "config"
	case "ai_context":
		return "config"
	case "policies", "authorization_audit":
		return "global"
	default:
		return "global"
	}
}

// Start runs the server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	log.Printf("Starting well-ambient server on %s", addr)

	// Start background Jira sync worker
	go s.startJiraSyncWorker()

	return http.ListenAndServe(addr, s.mux)
}

// handleHealth checks system readiness
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleStatus returns current telemetry summary (mock for MVP skeleton)
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"online", "version":"0.1.0", "telemetry":{"active_hooks":0}}`))
}

// handleGitLabWebhook processes incoming GitLab events
func (s *Server) handleGitLabWebhook(w http.ResponseWriter, r *http.Request) {
	telemetry.HandleWebhook(s.config, w, r)
}

// handleGetTasks returns task telemetries as JSON with query filters
func (s *Server) handleGetTasks(w http.ResponseWriter, r *http.Request) {
	projectFilters := queryFilterValues(r, "project")
	assigneeFilters := queryFilterValues(r, "assignee")
	visibility, _, err := s.loadCoreMemberVisibility()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query users for core member visibility: %v", err), http.StatusInternalServerError)
		return
	}

	tx := db.DB.Where("status != ?", "archived")
	tx, err = applyRequestProjectScope(tx, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}

	var tasks []db.TaskTelemetry
	if err := tx.Find(&tasks).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query tasks: %v", err), http.StatusInternalServerError)
		return
	}
	tasks = visibility.filterTasks(tasks)
	tasks = filterTaskTelemetriesByQuery(tasks, projectFilters, assigneeFilters)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		log.Printf("Error encoding tasks: %v", err)
	}
}

// handleGetLogs returns the 50 most recent webhook logs as JSON
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	var logs []db.WebhookLog
	if err := db.DB.Order("created_at desc").Limit(50).Find(&logs).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query webhook logs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		log.Printf("Error encoding webhook logs: %v", err)
	}
}

type TelemetryActivityDTO struct {
	TaskID    string    `json:"task_id"`
	Repo      string    `json:"repo"`
	Branch    string    `json:"branch"`
	CommitID  string    `json:"commit_id,omitempty"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	MrIID     int       `json:"mr_iid,omitempty"`
	MrURL     string    `json:"mr_url,omitempty"`
	Action    string    `json:"action"` // git_push, mr_open, mr_merge, mr_close, jira_comment
	CreatedAt time.Time `json:"created_at"`
}

// handleGetTaskCommits returns all git commit/MR logs associated with a task_id
func (s *Server) handleGetTaskCommits(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "Bad Request: missing task_id query parameter", http.StatusBadRequest)
		return
	}
	allowed, err := requestCanAccessTask(r, taskID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, fmt.Sprintf("Task %s not found", taskID), http.StatusNotFound)
		return
	}

	taskIDVariants := []string{taskID}
	for _, variant := range []string{strings.ToUpper(taskID), strings.ToLower(taskID)} {
		if !slices.Contains(taskIDVariants, variant) {
			taskIDVariants = append(taskIDVariants, variant)
		}
	}

	var gitLogs []db.GitCommitLog
	_ = db.DB.Where("task_id IN ?", taskIDVariants).Find(&gitLogs)

	var jiraComments []db.JiraCommentLog
	_ = db.DB.Where("task_id IN ?", taskIDVariants).Find(&jiraComments)

	// 合并为 TelemetryActivityDTO
	activities := []TelemetryActivityDTO{}
	for _, gl := range gitLogs {
		activities = append(activities, TelemetryActivityDTO{
			TaskID:    taskID,
			Repo:      gl.Repo,
			Branch:    gl.Branch,
			CommitID:  gl.CommitID,
			Message:   gl.Message,
			Author:    gl.Author,
			MrIID:     gl.MrIID,
			MrURL:     gl.MrURL,
			Action:    gl.Action,
			CreatedAt: gl.CreatedAt,
		})
	}
	for _, jc := range jiraComments {
		activities = append(activities, TelemetryActivityDTO{
			TaskID:    taskID,
			Repo:      "Jira",
			Branch:    "-",
			Message:   jc.Body,
			Author:    jc.Author,
			Action:    "jira_comment",
			CreatedAt: jc.CreatedAt,
		})
	}

	// 倒序排序
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].CreatedAt.After(activities[j].CreatedAt)
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activities); err != nil {
		log.Printf("Error encoding task commits: %v", err)
	}
}
