package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"well-ambient/internal/agenda"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/server/authz"
	"well-ambient/internal/telemetry"
)

// Server encapsulates the HTTP server logic
type Server struct {
	config     *config.Config
	configPath string
	mux        *http.ServeMux
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, configPath string) *Server {
	s := &Server{
		config:     cfg,
		configPath: configPath,
		mux:        http.NewServeMux(),
	}
	s.routes()

	// Bind the telemetry notification broadcast callback to avoid import cycles
	telemetry.OnNotificationBroadcast = BroadcastNotifications

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
	s.mux.HandleFunc("GET /api/tasks", s.withAuth(s.handleGetTasks))
	s.mux.HandleFunc("GET /api/tasks/commits", s.withAuth(s.handleGetTaskCommits))
	s.mux.HandleFunc("GET /api/execution/tasks", s.withPermission("dashboard:read", s.handleGetExecutionTasks))
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
	s.mux.HandleFunc("GET /api/schedule", s.withPermission("demands:read", s.handleGetSchedule))
	s.mux.HandleFunc("POST /api/tasks/schedule", s.withAuth(s.handleScheduleTask))

	// Protected Project Configs & Brain Scores
	s.mux.HandleFunc("GET /api/projects/config", s.withAuth(s.handleGetProjectConfigs))
	s.mux.HandleFunc("POST /api/projects/config", s.withPermission("config:write", s.handleSaveProjectConfig))
	s.mux.HandleFunc("GET /api/projects/scores", s.withAuth(s.handleGetProjectScores))
	s.mux.HandleFunc("POST /api/projects/scores/calculate", s.withPermission("config:write", s.handleCalculateProjectScores))

	// Protected AI Deconstructor API
	s.mux.HandleFunc("POST /api/deconstruct", s.withAuth(s.handleDeconstruct))
	s.mux.HandleFunc("POST /api/tasks/import", s.withAuth(s.handleImportTasks))
	s.mux.HandleFunc("GET /api/context/facts", s.withPermission("ai_context:read", s.handleListContextFacts))
	s.mux.HandleFunc("POST /api/context/facts", s.withPermission("ai_context:write", s.handleSaveContextFact))
	s.mux.HandleFunc("PUT /api/context/facts", s.withPermission("ai_context:write", s.handleSaveContextFact))
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
	case "demands":
		return "demand"
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

// handleGetTasks returns all task telemetries as JSON
func (s *Server) handleGetTasks(w http.ResponseWriter, r *http.Request) {
	var tasks []db.TaskTelemetry
	if err := db.DB.Find(&tasks).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query tasks: %v", err), http.StatusInternalServerError)
		return
	}

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

// handleGetTaskCommits returns all git commit/MR logs associated with a task_id
func (s *Server) handleGetTaskCommits(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "Bad Request: missing task_id query parameter", http.StatusBadRequest)
		return
	}

	var logs []db.GitCommitLog
	if err := db.DB.Where("task_id = ?", taskID).Order("created_at desc").Find(&logs).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query git logs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		log.Printf("Error encoding task commits: %v", err)
	}
}
