package server

import "well-ambient/internal/readmodel"

type readContractClass = readmodel.Class

const (
	readClassPage      = readmodel.ClassPage
	readClassAggregate = readmodel.ClassAggregate
	readClassTimeline  = readmodel.ClassTimeline
)

func allPageReadContracts() map[string]readmodel.Contract {
	return map[string]readmodel.Contract{
		"/api/agenda/summary":                   aggregateRead("agenda-summary", readmodel.MaturityPending, "decision.agenda", "evidence.health"),
		"/api/ai/output-trace":                  detailRead("ai-output-trace", readmodel.MaturityBounded, 100, "settings.ai_context"),
		"/api/ai/requirement-clarification":     detailRead("ai-requirement-clarification", readmodel.MaturityBounded, 100, "settings.ai_context"),
		"/api/ai/traces":                        detailRead("ai-traces", readmodel.MaturityBounded, 100, "settings.ai_context"),
		"/api/audit-logs":                       timelineRead("security-audit", readmodel.StrategyTopK, readmodel.MaturityBounded, "settings.audit"),
		"/api/authz/audit-logs":                 timelineRead("authorization-audit", readmodel.StrategyTopK, readmodel.MaturityBounded, "settings.audit", "settings.policies"),
		"/api/authz/policies":                   directoryRead("authorization-policies", readmodel.MaturityPending, "settings.policies"),
		"/api/config":                           singletonRead("runtime-config", readmodel.MaturityVerified, "shell", "settings.gitlab", "settings.feishu", "settings.jira", "settings.performance", "settings.projects", "settings.ai"),
		"/api/config/versions":                  timelineRead("config-versions", readmodel.StrategyTopK, readmodel.MaturityBounded, "settings.gitlab", "settings.feishu", "settings.jira", "settings.performance", "settings.projects", "settings.ai", "settings.audit"),
		"/api/context/documents":                generationPageRead("context-documents", "settings.ai_context"),
		"/api/context/documents/{id}":           detailRead("context-document", readmodel.MaturityBounded, 100, "settings.ai_context"),
		"/api/context/facts":                    generationPageRead("context-facts", "settings.ai_context"),
		"/api/context/pack/replay":              detailRead("context-pack-replay", readmodel.MaturityBounded, 100, "settings.ai_context", "evidence.health"),
		"/api/context/packs/{id}/replay":        detailRead("context-pack-id-replay", readmodel.MaturityBounded, 100, "settings.ai_context", "evidence.health"),
		"/api/corpus-candidates":                generationPageRead("corpus-candidates", "settings.ai_context"),
		"/api/corpus-candidates/{id}/impact":    detailRead("corpus-candidate-impact", readmodel.MaturityBounded, 20, "settings.ai_context"),
		"/api/data-assets/events":               verifiedTimelineRead("data-asset-events", "evidence.health"),
		"/api/data-assets/events/{id}":          detailRead("data-asset-event", readmodel.MaturityVerified, 1, "evidence.health"),
		"/api/data-assets/snapshots/latest":     singletonRead("latest-data-asset-snapshot", readmodel.MaturityVerified, "evidence.health", "kpi.overview"),
		"/api/data-assets/snapshots/{id}":       detailRead("data-asset-snapshot", readmodel.MaturityVerified, 200, "evidence.health", "kpi.overview"),
		"/api/decision/daily-jira":              dailyJiraGenerationPageRead("daily-jira", "decision.daily_jira"),
		"/api/delivery/directory":               directoryRead("delivery-directory", readmodel.MaturityBounded, "schedule.schedule", "schedule.board", "tasks.status", "tasks.execution"),
		"/api/delivery/exceptions":              aggregateRead("delivery-exceptions", readmodel.MaturityPending, "decision.agenda", "schedule.schedule"),
		"/api/delivery/quality":                 aggregateRead("delivery-quality", readmodel.MaturityPending, "schedule.schedule", "evidence.health"),
		"/api/demand-specs":                     pageRead("demand-specs", readmodel.MaturityBounded, "schedule.board", "solutions.catalog"),
		"/api/demands/options":                  directoryRead("demand-options", readmodel.MaturityBounded, "schedule.schedule", "schedule.board"),
		"/api/execution/runs":                   boundedGenerationNestedPageRead("execution-runs", maxExecutionActionsPerRun, "tasks.execution", "schedule.board"),
		"/api/execution/tasks":                  aggregateRead("execution-tasks", readmodel.MaturityPending, "tasks.execution"),
		"/api/gitlab/projects":                  externalDirectoryRead("gitlab-projects", "settings.gitlab"),
		"/api/gitlab/webhooks/status":           singletonRead("gitlab-webhook-status", readmodel.MaturityBounded, "settings.gitlab"),
		"/api/groups":                           directoryRead("user-groups", readmodel.MaturityBounded, "settings.users", "settings.matrix"),
		"/api/jira/link-config":                 singletonRead("jira-link-config", readmodel.MaturityVerified, "shell", "decision.agenda", "decision.daily_jira", "schedule.schedule", "schedule.releases", "schedule.board", "schedule.projects", "tasks.status", "tasks.execution"),
		"/api/kpi/performance":                  aggregateRead("kpi-performance", readmodel.MaturityPending, "kpi.overview"),
		"/api/kpi/report-preview":               aggregateRead("kpi-report-preview", readmodel.MaturityPending, "kpi.overview"),
		"/api/logs":                             timelineRead("webhook-logs", readmodel.StrategyTopK, readmodel.MaturityBounded, "settings.audit"),
		"/api/me":                               singletonRead("current-user", readmodel.MaturityVerified, "shell"),
		"/api/me/decision-table-columns":        singletonRead("decision-table-columns", readmodel.MaturityVerified, "decision.agenda"),
		"/api/me/project-preferences":           singletonRead("project-preferences", readmodel.MaturityVerified, "shell", "schedule.schedule", "schedule.releases", "schedule.board", "schedule.projects", "solutions.catalog", "tasks.status", "tasks.execution"),
		"/api/notifications/sse":                streamRead("notifications", "shell", "decision.agenda", "decision.daily_jira"),
		"/api/performance/explanation":          aggregateRead("performance-explanation", readmodel.MaturityPending, "kpi.calculation", "settings.performance"),
		"/api/performance/snapshots/{id}":       detailRead("performance-snapshot", readmodel.MaturityPending, 200, "kpi.calculation", "settings.performance"),
		"/api/permissions":                      directoryRead("permissions", readmodel.MaturityBounded, "settings.matrix"),
		"/api/projects/config":                  directoryRead("project-config", readmodel.MaturityBounded, "schedule.schedule", "schedule.board", "schedule.projects", "evidence.health", "settings.projects"),
		"/api/projects/scores":                  aggregateRead("project-scores", readmodel.MaturityPending, "schedule.schedule", "schedule.projects", "evidence.health", "settings.projects"),
		"/api/projects/{project_key}/releases":  generationPageRead("project-releases", "schedule.releases"),
		"/api/releases":                         boundedGenerationPageRead("releases", "schedule.releases"),
		"/api/releases/jira-search":             pageRead("jira-release-search", readmodel.MaturityPending, "schedule.releases", "settings.jira"),
		"/api/releases/{id}":                    detailRead("release", readmodel.MaturityVerified, 1, "schedule.releases"),
		"/api/releases/{id}/jira-issues":        boundedGenerationPageRead("release-jira-issues", "schedule.releases"),
		"/api/releases/{id}/snapshot":           aggregateRead("release-snapshot", readmodel.MaturityBounded, "schedule.releases"),
		"/api/requirements/clarification":       detailRead("requirement-clarification", readmodel.MaturityBounded, 100, "settings.ai_context"),
		"/api/review-contracts":                 detailRead("review-contract", readmodel.MaturityBounded, 10, "schedule.board", "tasks.execution"),
		"/api/schedule":                         aggregateRead("schedule", readmodel.MaturityPending, "schedule.schedule", "schedule.board", "schedule.projects"),
		"/api/schedule/risk-calendar":           aggregateRead("schedule-risk-calendar", readmodel.MaturityPending, "schedule.schedule"),
		"/api/solution-catalog":                 pageRead("solution-catalog", readmodel.MaturityPending, "solutions.catalog"),
		"/api/solution-catalog/projects":        directoryRead("solution-catalog-projects", readmodel.MaturityPending, "solutions.catalog"),
		"/api/solution-catalog/{id}":            detailRead("solution-catalog-entry", readmodel.MaturityPending, 100, "solutions.catalog"),
		"/api/solution-prompts":                 pageRead("solution-prompts", readmodel.MaturityPending, "settings.solution_prompts"),
		"/api/solution-standards":               pageRead("solution-standards", readmodel.MaturityPending, "solutions.catalog"),
		"/api/solution-standards/{id}":          detailRead("solution-standard", readmodel.MaturityBounded, 100, "solutions.catalog"),
		"/api/solutions/workspace":              detailRead("solution-workspace", readmodel.MaturityPending, 200, "schedule.board", "solutions.catalog"),
		"/api/status":                           singletonRead("system-status", readmodel.MaturityVerified, "shell", "settings.gitlab", "settings.feishu", "settings.jira", "settings.performance", "settings.projects", "settings.ai"),
		"/api/strongest-brain/ai-traces":        pageRead("strongest-brain-ai-traces", readmodel.MaturityPending, "evidence.health", "settings.ai_context"),
		"/api/strongest-brain/decision-queue":   aggregateRead("decision-queue", readmodel.MaturityPending, "decision.agenda"),
		"/api/strongest-brain/delivery-cockpit": aggregateRead("delivery-cockpit", readmodel.MaturityPending, "decision.agenda", "evidence.health"),
		"/api/strongest-brain/demand-readiness": aggregateRead("demand-readiness", readmodel.MaturityPending, "schedule.schedule", "evidence.health"),
		"/api/strongest-brain/evidence-chain":   detailRead("evidence-chain", readmodel.MaturityPending, 200, "evidence.health", "tasks.execution"),
		"/api/strongest-brain/exceptions":       aggregateRead("strongest-brain-exceptions", readmodel.MaturityPending, "decision.agenda", "evidence.health"),
		"/api/strongest-brain/override-audit":   timelineRead("override-audit", readmodel.StrategyTopK, readmodel.MaturityBounded, "decision.agenda", "schedule.schedule"),
		"/api/strongest-brain/releases":         aggregateRead("release-facts", readmodel.MaturityBounded, "decision.agenda"),
		"/api/strongest-brain/weekly-decisions": aggregateRead("weekly-decisions", readmodel.MaturityPending, "decision.agenda"),
		"/api/task-tracking/assignees":          directoryRead("task-assignees", readmodel.MaturityBounded, "tasks.status", "tasks.execution"),
		"/api/tasks":                            pageRead("legacy-tasks", readmodel.MaturityPending, "schedule.board", "tasks.status", "evidence.health"),
		"/api/tasks/commits":                    timelineRead("task-activity", readmodel.StrategyTopK, readmodel.MaturityBounded, "tasks.execution", "schedule.schedule"),
		"/api/users":                            directoryRead("users", readmodel.MaturityBounded, "schedule.schedule", "schedule.board", "tasks.status", "tasks.execution", "kpi.overview", "settings.users", "settings.matrix"),
		"/api/work-items":                       pageRead("work-items", readmodel.MaturityBounded, "shell", "tasks.status", "schedule.schedule", "schedule.board"),
		"/api/work-items/{id}":                  detailRead("work-item", readmodel.MaturityBounded, 200, "tasks.status", "tasks.execution", "schedule.schedule"),
	}
}

func allPageReadDatasets() []readmodel.Dataset {
	return []readmodel.Dataset{
		{Name: "task_facts", Tables: []string{
			"task_telemetries", "git_commit_logs", "jira_comment_logs", "work_item_release_links", "release_versions", "project_configs",
		}},
		{Name: "performance_facts", Tables: []string{
			"performance_score_runs", "performance_score_snapshots", "performance_evidence_facts", "performance_work_item_events", "performance_audit_events",
		}},
		{Name: "solution_facts", Tables: []string{
			"solution_assets", "solution_revisions", "solution_source_refs", "solution_polish_jobs", "solution_catalog_entries", "solution_catalog_search_tokens",
			"solution_comparisons", "solution_standards", "solution_standard_revisions", "solution_standardization_proposals",
		}},
		{Name: "context_facts", Tables: []string{"context_facts"}},
		{Name: "context_corpus", Tables: []string{"context_documents", "corpus_candidates"}},
		{Name: "execution_runs", Tables: []string{"execution_runs", "execution_actions"}},
		{Name: "release_facts", Tables: []string{"release_versions", "work_item_release_links", "release_jira_links"}},
		{Name: "security_facts", Tables: []string{"audit_logs", "authorization_audit_logs", "authorization_policies", "config_versions"}},
		{Name: "identity_facts", Tables: []string{"users", "user_groups", "user_group_memberships", "permissions", "group_permissions", "project_configs"}},
	}
}

func singletonRead(id string, maturity readmodel.Maturity, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassSingleton, readmodel.StrategyCurrent, maturity, 1, 1, 0, 512<<10, "one current row", surfaces)
}

func directoryRead(id string, maturity readmodel.Maturity, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassDirectory, readmodel.StrategyCachedDirectory, maturity, 200, 5000, 0, 2<<20, "version or update watermark", surfaces)
}

func externalDirectoryRead(id string, surfaces ...string) readmodel.Contract {
	contract := directoryRead(id, readmodel.MaturityPending, surfaces...)
	contract.TargetQueryP95MS = 500
	contract.TargetHandlerP95MS = 1000
	return contract
}

func detailRead(id string, maturity readmodel.Maturity, nestedMax int, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassDetail, readmodel.StrategyBoundedDetail, maturity, 1, 1, nestedMax, 4<<20, "entity revision plus bounded nested collections", surfaces)
}

func pageRead(id string, maturity readmodel.Maturity, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassPage, readmodel.StrategyKeyset, maturity, 50, 100, 0, 2<<20, "scope fingerprint plus fixed high watermark", surfaces)
}

func nestedPageRead(id string, maturity readmodel.Maturity, maxNestedRows int, surfaces ...string) readmodel.Contract {
	contract := pageRead(id, maturity, surfaces...)
	contract.MaxNestedRows = maxNestedRows
	return contract
}

func generationPageRead(id string, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassPage, readmodel.StrategyGenerationKeyset, readmodel.MaturityVerified, 50, 100, 0, 2<<20, "projection generation plus scope fingerprint", surfaces)
}

func dailyJiraGenerationPageRead(id string, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassPage, readmodel.StrategyGenerationKeyset, readmodel.MaturityVerified, 100, 100, 0, 2<<20, "projection generation plus scope fingerprint", surfaces)
}

func boundedGenerationPageRead(id string, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassPage, readmodel.StrategyGenerationKeyset, readmodel.MaturityBounded, 50, 100, 0, 2<<20, "projection generation plus scope fingerprint", surfaces)
}

func boundedGenerationNestedPageRead(id string, maxNestedRows int, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassPage, readmodel.StrategyGenerationKeyset, readmodel.MaturityBounded, 50, 100, maxNestedRows, 2<<20, "projection generation plus scope fingerprint", surfaces)
}

func aggregateRead(id string, maturity readmodel.Maturity, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassAggregate, readmodel.StrategyProjection, maturity, 200, 5000, 0, 4<<20, "completed projection generation or as-of watermark", surfaces)
}

func timelineRead(id string, strategy readmodel.Strategy, maturity readmodel.Maturity, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassTimeline, strategy, maturity, 100, 200, 0, 2<<20, "descending event key plus fixed high watermark", surfaces)
}

func verifiedTimelineRead(id string, surfaces ...string) readmodel.Contract {
	return timelineRead(id, readmodel.StrategyKeyset, readmodel.MaturityVerified, surfaces...)
}

func streamRead(id string, surfaces ...string) readmodel.Contract {
	return baseRead(id, readmodel.ClassStream, readmodel.StrategyStream, readmodel.MaturityVerified, 1, 1, 0, 256<<10, "monotonic event identity with reconnect semantics", surfaces)
}

func baseRead(
	id string,
	class readmodel.Class,
	strategy readmodel.Strategy,
	maturity readmodel.Maturity,
	defaultRows int,
	maxRows int,
	maxNestedRows int,
	maxResponseBytes int64,
	snapshot string,
	surfaces []string,
) readmodel.Contract {
	return readmodel.Contract{
		ID: id, Class: class, TargetStrategy: strategy, Maturity: maturity,
		Surfaces: surfaces, DefaultRows: defaultRows, MaxRows: maxRows,
		MaxNestedRows: maxNestedRows, MaxResponseBytes: maxResponseBytes,
		TargetQueryP95MS: 20, TargetHandlerP95MS: 50, Snapshot: snapshot,
	}
}
