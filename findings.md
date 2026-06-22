# Findings & Decisions

## Requirements
- Check and repair the repository according to `/Users/eddie/.gemini/antigravity/brain/ffc6d8a3-38c0-4a9b-a458-9b51beeff27b/implementation_plan.md`.
- Planned areas: confirmation modal styling, WellOS department extraction, select dropdown truncation, AI deconstruction to demand association, demand card shadow task progress, targeted tests/builds.
- Follow-up requirements: require `taste-skill` for frontend UI work, explain/deepen demand scheduling and AI deconstruction binding, fix user department display, and fix the native-looking new-demand due-date style.
- Current extension requirement: generate a concrete landing plan from the architecture recommendations and use self-agents to land GitLab/Jira integration, KPI report preview, and AI deconstruction expansion in bounded implementation tracks.

## Research Findings
- Git status shows the repository contents are currently untracked; avoid treating that as disposable state.
- Follow-up `find` showed `internal/server` and `cmd/server` do exist, but were not returned by the initial `rg --files` listing, likely due ignore rules.
- `DemandKanban.svelte` already contains demand/subtask filtering, card progress rendering, and confirm modal classes.
- `Deconstructor.svelte` already fetches active demands and posts `demand_id` to `/api/tasks/import`.
- `DecisionDashboard.svelte` already uses `width: max-content; min-width: 145px; max-width: 240px;` on `.custom-select-options`; `.custom-option` still has `overflow: hidden`, so long labels may still clip inside the widened menu.
- `handleLogin` already read WellOS response bytes before unmarshalling, but the direct raw log would expose auth tokens. It now logs a recursively redacted payload.
- `handleImportTasks` already accepted `demand_id`; it now rejects missing demand IDs instead of silently succeeding, preserves existing task metadata, and syncs imported tasks to markdown after commit.
- `ProcessWebhookEvent` previously rebuilt `TaskTelemetry` without `task_group_id`, `due_date`, creator, and decision-log fields; it now preserves those fields so AI task-group linkage survives Git telemetry.
- `DemandKanban.svelte` now filters archived demands/subtasks out of the active demand board.
- `TaskKanban.svelte` had a Svelte 5 compile error from a duplicate nested `{@const parentDemand...}` in the Done lane; the duplicate was removed.
- Earlier follow-up work recorded a fallback path for sessions where `taste-skill` is unavailable. In the latest request, the provided `design-taste-frontend` skill was read and applied to the touched demand UI.
- Department display was fragile because the frontend relied on localStorage/login response only. Department now travels through WellOS extraction, DB persistence, JWT claim, `/api/me`, and frontend refresh.
- Demand scheduling and AI deconstruction are now linked by the durable `task_group_id`: when a demand already has a task group, future AI imports reuse it.
- The new-demand due-date field now uses a styled date input shell with a custom calendar affordance and an invisible native picker hit target.
- The latest UI follow-up used the requested `design-taste-frontend` direction as an internal-dashboard repair, keeping the existing dark operational system instead of introducing landing-page styling.
- New demand creator departments could still remain hidden when the user row already contained placeholder values such as `未分配`. Those placeholders are now treated as missing, allowing JWT login department data or the frontend `creator_dept` fallback to overwrite them.
- The delete/archive dialog no longer inherits the old glass panel styling; it uses a flatter system modal with explicit target ID, compact metadata, and action-specific destructive/archive buttons.
- The latest follow-up replaces native date inputs entirely in touched demand forms with a custom calendar component, so browser-native date styling no longer leaks into the new-demand or schedule dialogs.
- Demand scheduling and AI deconstruction now share a stable generated task group when a demand has no explicit group: `brain-{demand_id}`. This lets schedule decisions and imported AI shadow tasks extend the same demand graph.
- New product demands should not auto-inherit the first GitLab repository. The repo field now stays empty on create, and demand cards show AI mapping state until shadow tasks or downstream development work establish repository context.
- A readonly personal profile surface is appropriate because OS profile data is authoritative and not editable in this app. It should live in the avatar dropdown rather than consume a main navigation tab, while `/api/me` repairs stale placeholder department/avatar fields from authenticated claims.
- WellOS department parsing must not accept generic top-level `displayName` as a department, because that can be the user's display name. Generic labels are now accepted only inside a known department context.
- When WellOS is in planned maintenance, the safe fallback is not password bypass for everyone. Only locally known users with existing group/permission snapshots and a matching local cached password hash from a prior successful WellOS login can receive a degraded session, and that session must be marked and audited.
- The app should visibly disclose degraded authentication, because OS-owned profile data may be stale until WellOS comes back.
- Local development has a separate need from production degraded auth. The development bypass must be opt-in via `WELL_AMBIENT_DEV_AUTH=1`, restricted to loopback requests, and visibly marked as `local_dev_auth`.
- GitLab webhook management belongs with the existing config handlers because `GET /api/gitlab/projects` already lives there and uses the `config:write` permission boundary.
- `internal/server/*` is ignored by `.gitignore` via the `server` pattern, so edits there do not appear in normal `git status`/`git diff`; verify with direct file reads and tests.
- GitLab API error responses should not be proxied back to callers because self-hosted GitLab can echo request details. Returning status-only messages avoids leaking `api_token` or `secret_token`.
- Existing Jira sync is already wired through `ProcessWebhookEvent -> SyncStatusToJira`; this MVP leaves Jira client behavior unchanged and focuses on automatically installing the GitLab webhook that feeds that path.
- GitLab project browsing already exists in the configuration panel and backend; the practical next step is webhook ensure/status automation over selected mapped repositories.
- KPI should move from raw completion ranking to factual performance previews: delivery count, cycle/risk indicators, report-preview narrative, and evidence links.
- AI deconstruction should move from task generation to a demand operation map: completeness, missing information, risks, dependencies, acceptance criteria, schedule notes, meeting questions, and confidence.
- The Override card had native-feeling assignee/deadline controls because it still used a plain text input and raw `input[type=date]`; replacing them with the existing dark-system custom control pattern fixes the mismatch while preserving the date input interaction.
- The red-zone filter can be presented as project filtering while continuing to use the existing `repo` JSON field from agenda telemetry; this avoids a backend/API rename for a label-level UX request.
- Native `<select>` styling in the KPI report preview is better replaced with a button-backed custom dropdown because browser option popovers cannot be consistently themed across platforms.
- Jira custom JQL summary overflow is a CSS containment issue: summary values need `min-width: 0`, right-aligned wrapping, and `overflow-wrap: anywhere` for long query strings.
- AI deconstruction already had editable `period_days` in the frontend, but the backend prompt did not require the model to output estimate fields. This made the visible schedule default rather than an auditable model estimate.
- Estimate accuracy evaluation needs both the original deconstruction run and task-level normalized estimates. A `DeconstructArchive` run record plus `TaskTelemetry` estimate fields keeps run-level and task-level evidence joinable by `task_group_id` and archive ID.
- Webhook/MR telemetry updates must preserve estimate fields, otherwise completion events would erase the baseline needed to compare estimated versus actual cycle time.
- The "strongest brain" product direction is to make daily/weekly coordination exception-driven: the system should assemble facts from demand decomposition, GitLab/Jira telemetry, KPI reports, and decision logs, then ask humans only for blocked decisions, missing context, and risk tradeoffs.
- AI deconstruction estimate quality depends on configured system context. The model now receives architecture, workflow, implemented capabilities, estimation guidelines, and work-hours-per-day from AI settings instead of guessing the current product map.
- Deconstructor task effort should be edited in hours. Day values remain stored as derived scheduling compatibility data, but the operator-facing control and prompt now treat hours as primary.
- Keeping AI analysis below the deconstruction button leaves the right result panel dedicated to project mapping and shadow-task cards, which better matches the user's review workflow.
- The strongest-brain landing model should turn status meetings into exception review: fact collectors ingest Jira/GitLab/AI archives/KPI data, the diff layer compares goal-plan-actual-risk, and humans only see direction decisions, resource tradeoffs, and override interventions.
- `全局领航能力汇总.xlsx` 第一个 sheet `功能列表` contains only the path-planning module capability catalog: 39 features across standard functions, extensions, advanced exploration, and customization.
- The path-planning sheet shows 20 durable capabilities, 7 initially validated, 3 development-complete, 3 in progress, 5 pending development, and 1 unmarked; 25 are configurable, 12 non-configurable, and 2 unclear.
- A long module capability catalog should be archived in a dedicated AI context document, with only a compact summary injected into `ai.project_architecture`, `ai.delivery_workflow`, `ai.implemented_features`, and `ai.estimation_guidelines`.
- Module prompt context must explicitly state scope boundaries so AI does not confuse path-planning business capabilities with well-ambient's collaboration-system capabilities.
- The path-planning/module-image direction was later identified as mis-scoped for “AI 上下文配置中心”: the center should configure system design, product capabilities, and delivery/process context for demand deconstruction, not behave as a targeted module capability feature.
- AI context should now be maintained as structured facts rather than four large visible text areas. The old AI config fields remain a backend fallback only, while the UI path is fact cards plus pack preview.
- A context pack must be persisted and linked to every deconstruction archive so future estimate review can reconstruct which system facts the model saw.
- The first Context Registry retrieval path is deterministic scope/keyword scoring with token budgets. This is intentionally simpler than embeddings so cache keys, tests, and audit behavior are stable before adding vector recall.
- Policy RBAC is now a layer over the existing permission matrix: explicit deny policies override legacy group grants, allow policies can grant exceptions, and global super_admin remains break-glass access.
- Authorization failures and high-risk grants should be logged as decisions with subject, action, resource, scope, matched policy, missing permission, reason, and risk level.
- Settings now treats AI engine configuration and AI context facts as separate admin tabs. This prevents the context registry from being perceived as part of provider/model setup.
- The visual permission matrix should not be maintained as a fixed table. The new Settings permission view fetches the live permission catalog from `/api/permissions`, derives branches by permission namespace, and renders vertical permission nodes so future seeded permissions appear without adding columns by hand.
- Policy authorization is easier to operate when it follows the sentence model "who can do what, where, and why". Templates, segmented subject/scope controls, permission action chips, and a priority stepper reduce the raw native form surface.
- The latest demand creation issue is a source aggregation problem: the modal needs candidates from users, Jira config/JQL, historical tasks, current user, and configured projects, not only `/api/users`.
- In this deployment Jira `Task` is a demand, not an execution-only task. The code should map Jira Task/Story/Feature/Epic-like issue types to `issue_type = demand`, while Bug/Defect-like issue types remain `bug`.
- Execution tracking remains useful after that change, but its scope becomes AI shadow tasks, manually split execution tasks, and Bug development evidence, not Jira Task demand intake.

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Locate actual symbols with `rg` before editing | Current repo structure differs from the plan. |
| Use `find` for ignored server files | `rg --files` missed backend files that are relevant to this task. |
| Redact login debug output | Keeps observability without violating token/secret handling. |
| Preserve task metadata during import/webhook updates | Prevents AI demand-task relationships and due dates from being erased by later state transitions. |
| Use a current-user profile endpoint | Fixes stale or missing frontend department display after refresh without granting admin user-list access. |
| Reuse selected demand task groups | Deepens and extends the AI decomposition graph around the demand rather than creating separate groups per run. |
| Generate `brain-{demand_id}` when a selected demand has no task group | Scheduling and AI deconstruction need a durable relationship before repo-level tasks exist. |
| Add readonly profile in the avatar dropdown instead of editable personal settings | User name, avatar, and department are OS-owned values, so this app should display and refresh them near the user trigger, not mutate them or consume a main tab. |
| Stop showing Git repo as default product-demand metadata | A repository belongs to mapped implementation tasks, not to the initial product-demand card before AI/repo mapping. |
| Use local access snapshots plus cached password verification during WellOS maintenance fallback | This preserves availability for known users without allowing unknown accounts or wrong passwords to bypass OS authentication. |
| Mark degraded JWTs and audit degraded logins | Operators need a visible trail that the session came from maintenance fallback rather than normal OS auth. |
| Keep local development auth separate from production maintenance fallback | Developers can continue debugging during OS downtime without weakening deployed authentication. |
| Keep GitLab webhook ensure/status under `config:write` | It mutates external GitLab project configuration and uses integration credentials, matching the existing GitLab project-selection API boundary. |
| Match existing hooks by normalized webhook URL | A hook for the same callback should be updated in place so token/events drift is repaired without duplicate callbacks. |
| Do not include GitLab response bodies in per-project errors | Prevents accidental token/secret exposure while still reporting enough status for operators to diagnose. |
| Use self-agents for disjoint implementation tracks | The requested work naturally splits across integration, KPI/reporting, and AI deconstruction modules with minimal file overlap. |
| Prefer structured previews before LLM-generated prose for reports | The system should make work facts auditable before adding summarization polish. |
| Preserve existing dark cockpit styling for control polish | The requested fixes are usability refinements inside an internal dashboard, not a redesign or marketing-page treatment. |
| Use custom button/dropdown shells for report and Override controls | This keeps the visual system consistent and avoids platform-native select/date surfaces leaking into the UI. |
| Persist every imported deconstruction run as an archive | Estimate accuracy is a longitudinal problem; retaining only current task fields would lose the original model rationale and overall estimate. |
| Normalize missing subtask estimates from overall estimate and difficulty weights | LLM outputs can omit optional fields; deterministic fallback keeps downstream due dates, UI display, and archives usable. |
| Configure AI project context instead of relying on implicit model knowledge | Architecture, workflow, implemented capabilities, and estimation rules must be first-class prompt inputs to reduce estimate drift. |
| Make hours the primary deconstruction estimate unit | Operators tune task effort in hours; day values remain derived compatibility data for due-date and archive logic. |
| Store long domain capability catalogs as archived context documents and inject compact summaries into config | Long feature inventories are better versioned/audited in docs, while runtime prompt fields should stay concise enough for stable AI behavior. |
| Remove the module-image AI context center rather than renaming it | Its upload/version/toggle model made the context center behave like a targeted feature; the remaining generic AI context fields better match the intended system-design and workflow calibration purpose. |
| Build AI context as a structured fact registry | Demand deconstruction needs typed facts, scope, freshness, confidence, token counts, hashes, and persisted pack assembly to keep prompts compact and auditable. |
| Keep legacy AI context fields as fallback only | Existing deployments should not lose prompt context, but future context should be curated through facts and packs rather than a broad text configuration center. |
| Layer policy authorization over existing RBAC | This preserves current group-permission compatibility while enabling subject/action/resource/scope decisions and explicit deny overrides. |
| Log denied and high-risk authorization decisions | Permission problems should be explainable without guessing which group, policy, scope, or missing permission caused the result. |
| Source permission tree data from the backend permission catalog | Permission growth should be driven by seeded `permissions` data rather than a hard-coded frontend matrix. |
| Keep AI context facts in their own Settings tab | The context registry is an input layer for demand deconstruction, while AI engine config is provider/model connectivity. |
| Replace policy raw selects with guided controls | Admins need fewer fragile fields and more visible policy intent before saving allow/deny rules. |
| Map Jira Task-like issue types to demands | The team's Jira Task items are real requirements that need scheduling, iteration planning, and delivery tracking. |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| Initial discovery suggested missing backend server | Confirmed the files exist via `find`; proceed with direct reads. |
| Frontend build failed on `TaskKanban.svelte` const placement | Removed the duplicate const from inside the card markup. |
| Svelte build reports existing a11y warnings | Build succeeds; warnings remain outside this plan's blocking scope. |
| Earlier session lacked `taste-skill` availability | Wrote the required rule and recorded `frontend-design` as the fallback for future unavailable sessions; latest demand UI fixes used the requested `design-taste-frontend` skill. |
| `apply_patch` failed due formatted context drift | Re-read snippets, patched with smaller context, and logged the tooling lesson. |
| `svelte-check` still fails after this UI polish | Remaining type errors are pre-existing and outside the touched files: `TaskKanban` nullable values, `Button size` props in config/settings panels, `Switch helperText`, and `App.svelte` header indexing. Production build passes. |
| Full server test suite cannot run inside the current sandbox | Existing tests using `httptest.NewServer` fail with `listen tcp6 [::1]:0: bind: operation not permitted`; targeted non-listener permission catalog test passes. |
| `svelte-check` still fails after Settings/RBAC polish | Remaining errors are pre-existing in `TaskKanban.svelte` and `App.svelte`; touched Settings/AI/Button files no longer contribute errors, and production build passes. |

## Resources
- Source plan: `/Users/eddie/.gemini/antigravity/brain/ffc6d8a3-38c0-4a9b-a458-9b51beeff27b/implementation_plan.md`
