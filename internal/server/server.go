package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"well-ambient/internal/agenda"
	"well-ambient/internal/agentruntime"
	"well-ambient/internal/codereview"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"
	"well-ambient/internal/performance"
	"well-ambient/internal/readmodel"
	"well-ambient/internal/server/authz"
	"well-ambient/internal/solutioncatalog"
	"well-ambient/internal/solutions"
	"well-ambient/internal/strongestbrain"
	"well-ambient/internal/telemetry"
)

var buildInfo = struct {
	Version   string
	Commit    string
	BuildTime string
}{Version: "dev", Commit: "unknown", BuildTime: "unknown"}

func SetBuildInfo(version, commit, buildTime string) {
	if strings.TrimSpace(version) != "" {
		buildInfo.Version = strings.TrimSpace(version)
	}
	if strings.TrimSpace(commit) != "" {
		buildInfo.Commit = strings.TrimSpace(commit)
	}
	if strings.TrimSpace(buildTime) != "" {
		buildInfo.BuildTime = strings.TrimSpace(buildTime)
	}
}

// Server encapsulates the HTTP server logic
type Server struct {
	codeReview          *codereview.Service
	emailConfigMu       sync.RWMutex
	emailConfigSnapshot *config.Config
	emailWorkerWakeup   chan struct{}
	emailLLM            func(context.Context, string, string) (string, error)
	emailSender         func(context.Context, config.SMTPConfig, []string, string, string, string) error
	emailConfluenceSync func(context.Context, config.ConfluenceSyncConfig, *emailReport) (string, error)
	config              *config.Config
	configPath          string
	mux                 *http.ServeMux
	handler             http.Handler
	readRegistry        *readmodel.Registry
	jiraAssigneeSync    func(issueKey, assignee string)
	jiraDueDateSync     func(issueKey, dueDate string)
	jiraCommentSync     func(issueKey, comment string)
	jiraDailyReviewSync func(issueKey string, assignee *string, comment string) error
	jiraReleaseList     func(ctx context.Context, projectKey string) ([]deliveryplanning.ExternalRelease, error)
	jiraInboundSyncMu   sync.Mutex
	solutionLLM         func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	solutions           *solutions.Module
	solutionCatalog     *solutioncatalog.Module
	performance         *performance.Module
	capabilityRegistry  *agentruntime.Registry
	agentTrace          *agentruntime.TraceCollector
	legacySkillAdapter  *agentruntime.LegacySkillAdapter
	strongestBrain      *strongestbrain.Service
	streamingCtx        context.Context
	stopStreaming       context.CancelFunc
}

func (s *Server) ensureAgentRuntime() {
	if db.DB == nil {
		return
	}
	if s.capabilityRegistry == nil {
		s.capabilityRegistry = agentruntime.NewRegistry(db.DB)
	}
	if s.agentTrace == nil {
		s.agentTrace = agentruntime.NewTraceCollector(db.DB)
	}
	if s.legacySkillAdapter == nil {
		s.legacySkillAdapter = agentruntime.NewLegacySkillAdapter(db.DB, s.capabilityRegistry)
		_ = s.legacySkillAdapter.EnsureDefaultCapabilities(context.Background())
	}
	if s.strongestBrain == nil {
		s.strongestBrain = strongestbrain.NewService(db.DB, s.capabilityRegistry)
	}
}

func (s *Server) streamingContext() context.Context {
	if s == nil || s.streamingCtx == nil {
		return context.Background()
	}
	return s.streamingCtx
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, configPath string) *Server {
	catalogConfig := cfg.SolutionCatalog.Normalized()
	solutionModule := solutions.New(db.DB)
	if err := solutionModule.MigrateLegacyInitialDrafts(context.Background()); err != nil {
		log.Printf("Solution lifecycle migration failed: %v", err)
	}
	streamingCtx, stopStreaming := context.WithCancel(context.Background())
	s := &Server{
		config:            cfg,
		configPath:        configPath,
		mux:               http.NewServeMux(),
		solutions:         solutionModule,
		streamingCtx:      streamingCtx,
		stopStreaming:     stopStreaming,
		emailWorkerWakeup: make(chan struct{}, 1),
		solutionCatalog: solutioncatalog.New(db.DB, solutioncatalog.Settings{
			Interval:        time.Duration(catalogConfig.IntervalMinutes) * time.Minute,
			CandidateLimit:  catalogConfig.CandidateLimit,
			RecallThreshold: catalogConfig.RecallThreshold,
		}),
	}
	s.performance = performance.New(db.DB, s.performanceSettings())
	s.jiraAssigneeSync = s.syncAssigneeToJira
	s.jiraDueDateSync = s.syncDueDateToJira
	s.jiraCommentSync = s.syncDecisionCommentToJira
	s.jiraDailyReviewSync = func(issueKey string, assignee *string, comment string) error {
		return telemetry.NewJiraClient(&s.config.Jira).UpdateIssueWithComment(issueKey, assignee, comment)
	}
	s.jiraReleaseList = func(ctx context.Context, projectKey string) ([]deliveryplanning.ExternalRelease, error) {
		return telemetry.NewJiraClient(&s.config.Jira).ListProjectReleases(ctx, projectKey)
	}
	s.solutionLLM = func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
		return queryServerLLMContext(ctx, s.config, systemPrompt, userPrompt)
	}
	s.setEmailConfig(*cfg)
	s.codeReview = s.newCodeReviewService()
	s.ensureAgentRuntime()
	s.routes()
	if db.DB != nil {
		databaseConfig, configErr := cfg.Database.Resolve()
		if configErr != nil {
			panic(fmt.Sprintf("resolve database configuration: %v", configErr))
		}
		if databaseConfig.AutoMigrate {
			if err := MigrateReadModels(db.DB); err != nil {
				panic(fmt.Sprintf("initialize all-page read generations: %v", err))
			}
		} else if err := verifyReadModels(db.DB); err != nil {
			panic(fmt.Sprintf("verify all-page read generations: %v", err))
		}
	}
	readRegistry, err := readmodel.NewRegistry(allPageReadContracts())
	if err != nil {
		panic(fmt.Sprintf("invalid all-page read contracts: %v", err))
	}
	s.readRegistry = readRegistry
	s.handler = readRegistry.Wrap(s.mux)

	// Bind the telemetry notification broadcast callback to avoid import cycles
	telemetry.OnNotificationBroadcast = BroadcastNotifications
	telemetry.OnTelemetryBroadcast = BroadcastTelemetryUpdated

	return s
}

func (s *Server) performanceSettings() performance.Settings {
	performanceConfig := s.config.PerformanceBrain.Normalized()
	return performance.Settings{
		Enabled:             performanceConfig.Enabled,
		CoreMembers:         s.configuredKPICoreMembers(),
		Interval:            time.Duration(performanceConfig.IntervalMinutes) * time.Minute,
		Window:              time.Duration(performanceConfig.AssessmentWindowDays) * 24 * time.Hour,
		Retention:           time.Duration(performanceConfig.RetentionDays) * 24 * time.Hour,
		FormulaVersion:      performanceConfig.FormulaVersion,
		CoverageGate:        performanceConfig.EvidenceCoverageGate,
		MinimumSamples:      performanceConfig.MinimumSamples,
		MinimumExposureDays: performanceConfig.MinimumExposureDays,
		BusyRetries:         performanceConfig.BusyRetryAttempts,
		BusyRetryDelay:      time.Duration(performanceConfig.BusyRetryDelayMS) * time.Millisecond,
		PublicationMode:     performanceConfig.PublicationMode,
		EnableDemandMetrics: *performanceConfig.DemandMetricsEnabled,
		EnableBugMetrics:    *performanceConfig.BugMetricsEnabled,
		EnableCodeMetrics:   *performanceConfig.CodeMetricsEnabled,
		JiraHistoryEnabled:  *performanceConfig.JiraHistoryEnabled,
		GitDedupeEnabled:    *performanceConfig.GitDedupeEnabled,
		FeatureFlagsSet:     true,
	}
}

// routes sets up API routing
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/code-reviews/repos", s.withPermission("dashboard:read", s.handleCodeReviewRepos))
	s.mux.HandleFunc("GET /api/code-reviews/targets", s.withPermission("dashboard:read", s.handleCodeReviewTargets))
	s.mux.HandleFunc("PUT /api/code-reviews/policy", s.withPermission("config:write", s.handleSaveCodeReviewPolicy))
	s.mux.HandleFunc("GET /api/code-reviews", s.withPermission("dashboard:read", s.handleListCodeReviews))
	s.mux.HandleFunc("POST /api/code-reviews", s.withPermission("config:write", s.handleCreateCodeReview))
	s.mux.HandleFunc("GET /api/code-reviews/{id}", s.withPermission("dashboard:read", s.withPermission("ai_context:preview", s.handleGetCodeReview)))
	s.mux.HandleFunc("POST /api/code-reviews/{id}/cancel", s.withPermission("config:write", s.handleCancelCodeReview))
	s.mux.HandleFunc("POST /api/code-reviews/{id}/retry", s.withPermission("config:write", s.handleRetryCodeReview))
	s.mux.HandleFunc("POST /api/code-reviews/{id}/sync", s.withPermission("config:write", s.handleSyncCodeReview))
	// Status and Health Check (No Auth)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleHealth)
	s.mux.HandleFunc("GET /live", s.handleLiveness)
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
	s.mux.HandleFunc("GET /api/performance/explanation", s.withAuth(s.withGlobalSuperAdmin(s.handleGetPerformanceExplanation)))
	s.mux.HandleFunc("GET /api/performance/snapshots/{id}", s.withAuth(s.withGlobalSuperAdmin(s.handleGetPerformanceSnapshot)))
	s.mux.HandleFunc("POST /api/performance/evidence", s.withAuth(s.withGlobalSuperAdmin(s.handleAppendPerformanceEvidence)))
	s.mux.HandleFunc("GET /api/strongest-brain/releases", s.withPermission("decision:read", s.handleGetStrongestBrainReleases))
	s.mux.HandleFunc("GET /api/strongest-brain/delivery-cockpit", s.withPermission("decision:read", s.handleGetStrongestBrainDeliveryCockpit))
	s.mux.HandleFunc("GET /api/strongest-brain/decision-queue", s.withPermission("decision:read", s.handleGetStrongestBrainDecisionQueue))
	s.mux.HandleFunc("GET /api/strongest-brain/exceptions", s.withPermission("decision:read", s.handleGetStrongestBrainExceptions))
	s.mux.HandleFunc("GET /api/strongest-brain/weekly-decisions", s.withPermission("decision:read", s.handleGetStrongestBrainWeeklyDecisions))
	s.mux.HandleFunc("GET /api/strongest-brain/demand-readiness", s.withPermission("demands:read", s.handleGetStrongestBrainDemandReadiness))
	s.mux.HandleFunc("GET /api/strongest-brain/override-audit", s.withPermission("decision:read", s.handleGetStrongestBrainOverrideAudit))
	s.mux.HandleFunc("GET /api/strongest-brain/ai-traces", s.withPermission("ai_context:read", s.handleGetStrongestBrainAITraces))
	s.mux.HandleFunc("GET /api/strongest-brain/review-intelligence", s.withPermission("decision:read", s.handleGetStrongestBrainReviewIntelligence))
	s.mux.HandleFunc("POST /api/strongest-brain/intervention", s.withPermission("demands:write", s.handleStrongestBrainIntervention))
	s.mux.HandleFunc("GET /api/decision/daily-jira", s.withPermission("decision:read", s.handleGetDailyJiraAudit))
	s.mux.HandleFunc("POST /api/decision/daily-jira/sync", s.withPermission("decision:read", s.handlePostDailyJiraSync))
	s.mux.HandleFunc("POST /api/decision/daily-jira/review", s.withPermission("decision:read", s.withPermission("demands:write", s.handlePostDailyJiraReview)))
	s.mux.HandleFunc("GET /api/logs", s.withAuth(s.handleGetLogs))
	s.mux.HandleFunc("GET /api/data-assets/events", s.withPermission("data_asset:read", s.handleListDataAssetEvents))
	s.mux.HandleFunc("GET /api/data-assets/events/{id}", s.withPermission("data_asset:read", s.handleGetDataAssetEvent))
	s.mux.HandleFunc("GET /api/data-assets/snapshots/latest", s.withPermission("data_asset:read", s.handleGetLatestDataAssetSnapshot))
	s.mux.HandleFunc("GET /api/data-assets/snapshots/{id}", s.withPermission("data_asset:read", s.handleGetDataAssetSnapshot))

	// Protected Config APIs
	s.mux.HandleFunc("GET /api/config", s.withPermission("config:read", s.handleGetConfig))
	s.mux.HandleFunc("GET /api/jira/link-config", s.withAuth(s.handleGetJiraLinkConfig))
	s.mux.HandleFunc("POST /api/config", s.withPermission("config:write", s.handleSaveConfig))
	s.mux.HandleFunc("POST /api/config/test", s.withPermission("config:write", s.handleTestConnection))
	s.mux.HandleFunc("POST /api/daily-jira-email/template", s.withPermission("config:write", s.handleGenerateEmailTemplate))
	s.mux.HandleFunc("POST /api/daily-jira-email/preview", s.withPermission("config:write", s.handlePreviewEmail))
	s.mux.HandleFunc("POST /api/daily-jira-email/send", s.withPermission("config:write", s.handleSendEmail))
	s.mux.HandleFunc("GET /api/daily-jira-email/candidate-owners", s.withPermission("config:read", s.handleGetEmailCandidateOwners))
	s.mux.HandleFunc("GET /api/daily-jira-email/runs", s.withPermission("config:read", s.handleEmailRuns))
	s.mux.HandleFunc("GET /api/daily-jira-email/templates", s.withPermission("config:read", s.handleListEmailTemplates))
	s.mux.HandleFunc("POST /api/daily-jira-email/templates", s.withPermission("config:write", s.handleCreateEmailTemplates))
	s.mux.HandleFunc("DELETE /api/daily-jira-email/templates/{id}", s.withPermission("config:write", s.handleDeleteEmailTemplate))
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
	s.mux.HandleFunc("GET /api/solutions/workspace", s.withPermission("solution:read", withSolutionCompression(s.handleGetSolutionWorkspace)))
	s.mux.HandleFunc("POST /api/solutions/draft", s.withPermission("solution:write", withSolutionCompression(s.handleSaveSolutionDraft)))
	s.mux.HandleFunc("POST /api/solutions/fork", s.withPermission("solution:write", withSolutionCompression(s.handleForkSolutionDraft)))
	s.mux.HandleFunc("POST /api/solutions/polish", s.withPermission("solution:write", withSolutionCompression(s.handleRequestSolutionPolish)))
	s.mux.HandleFunc("POST /api/solutions/jobs/{id}/retry", s.withPermission("solution:write", withSolutionCompression(s.handleRetrySolutionJob)))
	s.mux.HandleFunc("POST /api/solutions/candidates/{id}/apply", s.withPermission("solution:write", withSolutionCompression(s.handleApplySolutionCandidate)))
	s.mux.HandleFunc("POST /api/solutions/publish", s.withPermission("solution:publish", withSolutionCompression(s.handlePublishSolution)))
	s.mux.HandleFunc("GET /api/solution-catalog", s.withPermission("solution:read", withSolutionCompression(s.handleListSolutionCatalog)))
	s.mux.HandleFunc("GET /api/solution-catalog/projects", s.withPermission("solution:read", s.handleListSolutionCatalogProjects))
	s.mux.HandleFunc("GET /api/solution-catalog/{id}", s.withPermission("solution:read", withSolutionCompression(s.handleGetSolutionCatalogEntry)))
	s.mux.HandleFunc("GET /api/solution-standards", s.withPermission("solution:read", withSolutionCompression(s.handleListSolutionStandards)))
	s.mux.HandleFunc("GET /api/solution-standards/{id}", s.withPermission("solution:read", withSolutionCompression(s.handleGetSolutionStandard)))
	s.mux.HandleFunc("POST /api/solution-catalog/reconcile", s.withPermission("solution:publish", s.withGlobalSuperAdmin(s.handleReconcileSolutionCatalog)))
	s.mux.HandleFunc("POST /api/solution-catalog/proposals/{id}/review", s.withPermission("solution:publish", withSolutionCompression(s.handleReviewSolutionProposal)))
	s.mux.HandleFunc("GET /api/solution-prompts", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleListSolutionPrompts)))
	s.mux.HandleFunc("POST /api/solution-prompts", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleSaveSolutionPrompt)))
	s.mux.HandleFunc("POST /api/solution-prompts/test", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleTestSolutionPrompt)))
	s.mux.HandleFunc("POST /api/solution-prompts/{id}/test", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleTestStoredCodeReviewSkill)))
	s.mux.HandleFunc("POST /api/solution-prompts/{id}/activate", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleActivateSolutionPrompt)))
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
	s.mux.HandleFunc("GET /api/projects/catalog", s.withPermission("config:read", s.handleGetProjectCatalog))
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
	s.mux.HandleFunc("POST /api/releases/{id}/publish", s.withPermission("release:manage", s.handlePublishRelease))
	s.mux.HandleFunc("POST /api/releases/{id}/archive", s.withPermission("release:manage", s.handleArchiveRelease))
	s.mux.HandleFunc("POST /api/releases/{id}/discard", s.withPermission("release:manage", s.handleDiscardRelease))
	s.mux.HandleFunc("DELETE /api/releases/{id}", s.withPermission("release:manage", s.handleDeleteRelease))
	s.mux.HandleFunc("GET /api/releases/{id}/jira-issues", s.withPermission("delivery:read", s.handleListReleaseJiraIssues))
	s.mux.HandleFunc("POST /api/releases/{id}/jira-issues/bulk", s.withPermission("release:manage", s.handleBulkAddReleaseJiraIssues))
	s.mux.HandleFunc("DELETE /api/releases/{id}/jira-issues/{work_item_id}", s.withPermission("release:manage", s.handleDeleteReleaseJiraIssue))
	s.mux.HandleFunc("PUT /api/releases/{id}/jira-link", s.withPermission("release:manage", s.handlePutReleaseJiraLink))
	s.mux.HandleFunc("DELETE /api/releases/{id}/jira-link", s.withPermission("release:manage", s.handleDeleteReleaseJiraLink))
	s.mux.HandleFunc("GET /api/releases/{id}/snapshot", s.withPermission("delivery:read", s.handleGetReleaseSnapshot))
	s.mux.HandleFunc("GET /api/work-items", s.withPermission("delivery:read", s.handleListWorkItems))
	s.mux.HandleFunc("GET /api/work-items/{id}", s.withPermission("delivery:read", s.handleGetWorkItem))
	s.mux.HandleFunc("PATCH /api/work-items/{id}/planning", s.withPermission("delivery:plan", s.handlePatchWorkItemPlanning))
	s.mux.HandleFunc("POST /api/work-items/{id}/complete", s.withAuth(s.handleCompleteWorkItem))
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

	// Protected Agent Runtime & Capability Registry APIs
	s.mux.HandleFunc("GET /api/agent-runtime/capabilities", s.withPermission("ai_context:read", s.handleListAgentCapabilities))
	s.mux.HandleFunc("GET /api/agent-runtime/capabilities/{id}", s.withPermission("ai_context:read", s.handleGetAgentCapabilityDetail))
	s.mux.HandleFunc("POST /api/agent-runtime/capabilities", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleRegisterAgentCapability)))
	s.mux.HandleFunc("PATCH /api/agent-runtime/capabilities/{id}/status", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleUpdateAgentCapabilityStatus)))
	s.mux.HandleFunc("DELETE /api/agent-runtime/capabilities/{id}", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleUninstallAgentCapability)))
	s.mux.HandleFunc("POST /api/agent-runtime/capabilities/{id}/reinstall", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleReinstallAgentCapability)))
	s.mux.HandleFunc("POST /api/agent-runtime/capabilities/{id}/upgrade", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleUpgradeAgentCapability)))
	s.mux.HandleFunc("POST /api/agent-runtime/capabilities/remote-install", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleRemoteInstallAgentCapability)))
	s.mux.HandleFunc("POST /api/agent-runtime/capabilities/{id}/activate", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleActivateAgentCapabilityVersion)))
	s.mux.HandleFunc("POST /api/agent-runtime/resolve/preview", s.withPermission("ai_context:preview", s.handlePreviewCapabilityResolve))
	s.mux.HandleFunc("GET /api/agent-runtime/runs", s.withPermission("ai_context:read", s.handleListAgentRuns))
	s.mux.HandleFunc("GET /api/agent-runtime/runs/{id}/lockfile", s.withPermission("ai_context:read", s.handleGetAgentRunLockfile))
	s.mux.HandleFunc("GET /api/agent-runtime/runs/{id}/trace", s.withPermission("ai_context:read", s.handleGetAgentRunTrace))
	s.mux.HandleFunc("POST /api/agent-runtime/replays", s.withPermission("solution_prompt:manage", s.withGlobalSuperAdmin(s.handleRunAgentRuntimeReplay)))
	s.mux.HandleFunc("GET /api/agent-runtime/replays/{id}", s.withPermission("ai_context:read", s.handleGetAgentRuntimeReplay))

	// Protected Strongest Brain Capability Intelligence & Proposals
	s.mux.HandleFunc("GET /api/strongest-brain/capability-intelligence", s.withPermission("decision:read", s.handleGetStrongestBrainCapabilityIntelligence))
	s.mux.HandleFunc("GET /api/strongest-brain/capability-proposals", s.withPermission("decision:read", s.handleListStrongestBrainCapabilityProposals))
	s.mux.HandleFunc("POST /api/strongest-brain/capability-proposals/{id}/review", s.withPermission("decision:read", s.withGlobalSuperAdmin(s.handleReviewStrongestBrainCapabilityProposal)))
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
	// Review responses contain source code and knowledge across configured repositories.
	// Query parameters must not downgrade this global permission check to one repository.
	if strings.HasPrefix(r.URL.Path, "/api/code-reviews") {
		return resource
	}
	// Runtime settings (including SMTP credentials) are always global resources.
	if strings.HasPrefix(requiredPermission, "config:") && (r.URL.Path == "/api/config" || strings.HasPrefix(r.URL.Path, "/api/config/") || r.URL.Path == "/api/projects/catalog" || strings.HasPrefix(r.URL.Path, "/api/daily-jira-email/")) {
		return resource
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
	case "demands", "demand_spec", "review_contract", "execution", "solution":
		return "demand"
	case "solution_prompt":
		return "config"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()
	return s.Serve(ctx)
}

// Serve runs the HTTP server and all background workers until the context is
// cancelled. It gives Linux service managers and containers a bounded graceful
// shutdown path instead of abruptly abandoning in-flight work.
func (s *Server) Serve(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	log.Printf("Starting well-ambient server on %s", addr)
	workerContext, stopWorkers := context.WithCancel(ctx)
	emailWorkerDone := s.startEmailWorker(workerContext)

	// Keep the latency-sensitive Jira projection independent from heavy history replay.
	s.startJiraSyncWorkers(workerContext)
	go s.startSolutionWorker(workerContext)
	go s.startCodeReviewWorker(workerContext)
	go s.startDailyJiraProjectionWorker(workerContext)
	catalogStarted := s.solutionCatalog != nil && s.solutionCatalog.Start(workerContext)
	if s.performance != nil {
		s.performance.Start(workerContext)
	}
	defer func() {
		stopWorkers()
		<-emailWorkerDone
		if s.performance != nil {
			s.performance.Stop()
		}
		if catalogStarted {
			s.solutionCatalog.Stop()
		}
	}()

	handler := s.handler
	if handler == nil {
		handler = s.mux
	}
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serverError := make(chan error, 1)
	go func() {
		serverError <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Println("收到关闭信号 (Ctrl+C / SIGTERM)，正在关闭 SSE/流式长连接，保留缓冲期等待在途普通写请求排空...")
		if s.stopStreaming != nil {
			s.stopStreaming()
		}
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			log.Printf("在途普通写请求排空超时或失败，强制关闭连接: %v", err)
			_ = httpServer.Close()
			return fmt.Errorf("graceful HTTP shutdown: %w", err)
		}
		log.Println("在途普通写请求已全部排空，HTTP 服务已成功退出。")
		err := <-serverError
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// handleLiveness proves only that the process and HTTP event loop are alive.
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleHealth checks database readiness. Existing /health clients keep their
// path while /ready makes the readiness intent explicit for new deployments.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := db.Ping(ctx); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable", "dependency": "database"})
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

type jiraInboundStatus struct {
	Enabled           bool       `json:"enabled"`
	State             string     `json:"state"`
	SuccessfulThrough *time.Time `json:"successful_through,omitempty"`
	LastStartedAt     *time.Time `json:"last_started_at,omitempty"`
	LastSucceededAt   *time.Time `json:"last_succeeded_at,omitempty"`
	LastIssueCount    int        `json:"last_issue_count"`
	LastChangedCount  int        `json:"last_changed_count"`
	HasError          bool       `json:"has_error"`
}

func optionalStatusTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func (s *Server) jiraInboundStatus(now time.Time) jiraInboundStatus {
	status := jiraInboundStatus{State: "disabled"}
	if s == nil || s.config == nil || !s.config.Jira.Enabled {
		return status
	}
	status.Enabled = true
	status.State = "pending"
	if db.DB == nil {
		status.State = "unavailable"
		status.HasError = true
		return status
	}

	var checkpoint db.JiraInboundSyncState
	if err := db.DB.Where("scope = ?", jiraInboundSyncScope).First(&checkpoint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status
		}
		status.State = "unavailable"
		status.HasError = true
		return status
	}

	status.SuccessfulThrough = optionalStatusTime(checkpoint.SuccessfulThrough)
	status.LastStartedAt = optionalStatusTime(checkpoint.LastStartedAt)
	status.LastSucceededAt = optionalStatusTime(checkpoint.LastSucceededAt)
	status.LastIssueCount = checkpoint.LastIssueCount
	status.LastChangedCount = checkpoint.LastChangedCount
	status.HasError = strings.TrimSpace(checkpoint.LastError) != ""

	switch {
	case status.HasError:
		status.State = "error"
	case checkpoint.LastStartedAt.IsZero():
		status.State = "pending"
	case checkpoint.LastStartedAt.After(checkpoint.LastSucceededAt):
		status.State = "syncing"
		if now.Sub(checkpoint.LastStartedAt) > jiraInboundStaleAfter {
			status.State = "stale"
		}
	case checkpoint.LastSucceededAt.IsZero():
		status.State = "pending"
	case now.Sub(checkpoint.LastSucceededAt) > jiraInboundStaleAfter:
		status.State = "stale"
	default:
		status.State = "healthy"
	}
	return status
}

// handleStatus returns current process and inbound synchronization health.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	payload := struct {
		Status        string                  `json:"status"`
		Version       string                  `json:"version"`
		Commit        string                  `json:"commit"`
		BuildTime     string                  `json:"build_time"`
		Telemetry     map[string]int          `json:"telemetry"`
		JiraSync      jiraInboundStatus       `json:"jira_sync"`
		ReadContracts readmodel.Inventory     `json:"read_contracts"`
		ReadPaths     []readmodel.Observation `json:"read_paths"`
	}{
		Status:        "online",
		Version:       buildInfo.Version,
		Commit:        buildInfo.Commit,
		BuildTime:     buildInfo.BuildTime,
		Telemetry:     map[string]int{"active_hooks": 0},
		JiraSync:      s.jiraInboundStatus(time.Now()),
		ReadContracts: s.readContractInventory(strings.EqualFold(r.URL.Query().Get("read_contracts"), "full")),
		ReadPaths:     s.readPathObservations(),
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Status response encode failed: %v", err)
	}
}

func (s *Server) readContractInventory(includeDeclarations bool) readmodel.Inventory {
	if s == nil || s.readRegistry == nil {
		return readmodel.Inventory{}
	}
	return s.readRegistry.Inventory(includeDeclarations)
}

func (s *Server) readPathObservations() []readmodel.Observation {
	if s == nil || s.readRegistry == nil {
		return []readmodel.Observation{}
	}
	return s.readRegistry.Snapshot()
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

const maxTaskActivityRows = 100

// handleGetTaskCommits returns the most recent bounded git/Jira activity window
// associated with a task_id. The response remains an array for legacy clients;
// the hard cap prevents one old task from materializing an unbounded timeline.
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

	var gitLogs []db.GitCommitLog
	if err := db.DB.WithContext(r.Context()).
		Where("LOWER(task_id) = LOWER(?)", taskID).
		Order("created_at DESC, id DESC").
		Limit(maxTaskActivityRows).
		Find(&gitLogs).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query Git task activity: %v", err), http.StatusInternalServerError)
		return
	}

	var jiraComments []db.JiraCommentLog
	if err := db.DB.WithContext(r.Context()).
		Where("LOWER(task_id) = LOWER(?) AND current = ?", taskID, true).
		Order("created_at DESC, id DESC").
		Limit(maxTaskActivityRows).
		Find(&jiraComments).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query Jira task activity: %v", err), http.StatusInternalServerError)
		return
	}

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
	if len(activities) > maxTaskActivityRows {
		activities = activities[:maxTaskActivityRows]
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activities); err != nil {
		log.Printf("Error encoding task commits: %v", err)
	}
}
