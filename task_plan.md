# Task Plan: Implementation Plan Check and Fix

## Goal
Inspect the repository against the provided implementation plan and apply the needed fixes that fit the current codebase, including the follow-up fixes for frontend skill rules, department display, demand scheduling date styling, and AI deconstruction binding.

## Current Task Addendum: GitLab Webhook Ensure/Status MVP
- [x] Inspect existing GitLab/Jira integration and route patterns
- [x] Add GitLab project webhook ensure/status backend capability
- [x] Register protected API routes
- [x] Add httptest-backed GitLab API regression tests
- [x] Run targeted Go validation

## Current Phase
Complete

## Phases

### Phase 1: Requirements & Discovery
- [x] Read repository boot instructions
- [x] Read the provided implementation plan
- [x] Locate actual backend and frontend implementation points
- **Status:** complete

### Phase 2: Planning & Structure
- [x] Map plan items to current files and symbols
- [x] Identify gaps where the plan references stale paths or APIs
- **Status:** complete

### Phase 3: Implementation
- [x] Apply backend fixes
- [x] Apply frontend fixes
- [x] Add focused tests where possible
- **Status:** complete

### Phase 4: Testing & Verification
- [x] Run targeted Go tests
- [x] Run frontend build
- [x] Fix validation failures
- **Status:** complete

### Phase 5: Delivery
- [x] Update task memory
- [x] Summarize changed files and validation
- **Status:** complete

### Phase 6: Follow-up Fixes
- [x] Write the frontend UI skill rule requiring `taste-skill`
- [x] Record fallback to `frontend-design` because `taste-skill` is not currently available
- [x] Fix department display by strengthening WellOS extraction, JWT claims, `/api/me`, and frontend profile refresh
- [x] Deepen AI demand scheduling/deconstruction binding by reusing a demand's existing `task_group_id`
- [x] Replace the native-looking demand due-date control with a styled date input shell
- [x] Add focused tests and rerun full validation
- **Status:** complete

### Phase 7: Latest Follow-up Fixes
- [x] Use the requested `design-taste-frontend` guidance for touched UI
- [x] Restyle delete/archive confirmation modals to match the internal dark system
- [x] Fix demand creator department fallback after relogin, including stale `未分配` placeholders
- [x] Keep new-demand and schedule deadline controls visually custom while preserving native date picking
- [x] Add regression tests and rerun Go/build validation
- **Status:** complete

### Phase 8: Brain Binding, Profile, Repo, and Calendar Follow-up
- [x] Use the requested `design-taste-frontend` guidance for this UI repair set
- [x] Bind demand scheduling and AI deconstruction through stable `brain-{demand_id}` task groups
- [x] Replace native new-demand and schedule date inputs with a custom calendar control
- [x] Stop defaulting new demand cards to the first code repository
- [x] Add a readonly personal profile surface inside the avatar dropdown for OS-owned profile data
- [x] Backfill missing or placeholder department/avatar/name values from authenticated claims through `/api/me`
- [x] Add regression tests and rerun full Go validation
- **Status:** complete

### Phase 9: WellOS Maintenance Login Degradation
- [x] Add guarded local fallback for existing users when WellOS login is unreachable
- [x] Require cached local password verification from a prior successful WellOS login before degraded login
- [x] Deny fallback for unknown users and users without local permission snapshots
- [x] Mark degraded sessions and audit degraded logins
- [x] Add frontend maintenance banner for degraded sessions
- [x] Add opt-in loopback-only local development auth for OS outages
- [x] Add regression tests and rerun Go/build validation
- **Status:** complete

### Phase 10: System Extension Landing
- [x] Convert the product/architecture recommendations into implementation tracks
- [x] Land GitLab/Jira integration extensions through worker A
- [x] Land KPI report preview and personal performance analysis through worker B
- [x] Land AI demand deconstruction analysis extensions through worker C
- [x] Integrate worker changes, resolve conflicts, and run full validation
- [x] Update findings/progress and summarize the delivered extension surface
- **Status:** complete

### Phase 11: Dashboard UI Control Polish
- [x] Use the requested `design-taste-frontend` guidance for touched UI
- [x] Replace native-looking Override assignee/deadline controls with dark-system controls
- [x] Rename the red-zone repository filter to project filtering while preserving the existing `repo` API field
- [x] Replace the native report preview team select with a dark-system custom dropdown
- [x] Fix Jira custom JQL summary overflow
- [x] Run focused frontend validation
- **Status:** complete

### Phase 12: AI Deconstruction Estimation & Meeting Replacement
- [x] Add overall and subtask estimate fields to the AI deconstruction contract
- [x] Persist deconstruction run archives for later estimate accuracy validation
- [x] Preserve task estimate telemetry through imports and webhook state changes
- [x] Display editable estimate and difficulty data in the Deconstructor cockpit
- [x] Add focused regression tests and rerun frontend/backend validation
- [x] Summarize the strongest-brain product strategy for reducing daily/weekly meeting toil
- **Status:** complete

### Phase 13: AI Project Context & Hour-Based Deconstruction
- [x] Add AI project-context configuration for architecture, workflow, implemented capabilities, and estimation guidelines
- [x] Inject configured project context into the deconstruction prompt so estimates use the current system map
- [x] Treat hours as the primary deconstruction estimate unit while retaining day conversion for scheduling/archive compatibility
- [x] Move AI analysis below the deconstruction button so the shadow-task result panel stays focused on task cards
- [x] Add configured-hours regression coverage and rerun frontend/backend validation
- [x] Summarize the strongest-brain landing model for replacing status collection with exception-driven decisions
- **Status:** complete

### Phase 14: Path Planning Module Prompt Context
- [x] Read `全局领航能力汇总.xlsx` first sheet `功能列表`
- [x] Extract path-planning capability counts, status distribution, configurability, and key feature groups
- [x] Create a dedicated AI context archive for the path-planning module
- [x] Populate AI project-context fields in `config.example.yaml` with a compact prompt-ready summary
- [x] Validate YAML/config loading and focused AI deconstruction tests
- [x] Summarize strongest-brain guidance for configuring and using module-specific prompt context
- **Status:** complete

### Phase 15: Remove Mis-scoped AI Context Center
- [x] Remove the module-image AI context center UI from AI settings
- [x] Delete `/api/ai-context/*` routes, handlers, tests, and database model
- [x] Stop injecting enabled module context profiles into deconstruction prompts
- [x] Clear path-planning-specific context from `config.example.yaml`
- [x] Delete the path-planning AI context archive document
- [x] Validate Go regression and frontend production build
- [x] Commit only intentional source/config/docs/memory changes
- **Status:** complete

### Phase 16: Strongest Brain Context Registry And Policy RBAC
- [x] Write the implementation plan to `docs/strongest-brain-implementation-plan.md`
- [x] Spawn parallel workers for AI context backend, RBAC backend, and frontend admin UI
- [x] Integrate AI Context Registry schema, APIs, pack assembly, and deconstruction archive link
- [x] Integrate policy authorization schema, evaluator, compatibility layer, and tests
- [x] Integrate AI context and policy management UI surfaces
- [x] Run full Go and frontend validation
- [x] Update task memory and commit intentional changes
- **Status:** complete

### Phase 17: AI Engine Tab, Permission Tree, And Policy Workbench Polish
- [x] Split AI settings into independent AI engine and context fact tabs
- [x] Replace native-looking AI context controls with button-backed choices and custom range/input styling
- [x] Add a live `/api/permissions` catalog endpoint
- [x] Replace the hard-coded visual permission matrix with a namespace-derived permission tree
- [x] Rework policy authorization into templates, segmented controls, action chips, and a priority stepper
- [x] Add focused permission-catalog handler coverage
- [x] Run frontend build and targeted backend validation
- **Status:** complete

## Key Questions
1. Which current files contain the server handlers named in the plan?
2. Does the current data model support linking demands to AI task groups?
3. Are the planned frontend controls already partially implemented?

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| Treat the plan as a target, not a literal file path list | The initial `rg --files` index missed ignored backend files, so path discovery needed a fallback. |
| Continue with ignored files discovered by `find` | `rg --files` did not list `internal/server`, but `find` revealed the backend server files are present. |
| Redact WellOS login response logs | The implementation plan requested raw response visibility, but auth tokens and secrets must not be written to logs. |
| Preserve existing telemetry link fields on webhook update | AI shadow-task linkage would otherwise be lost when GitLab webhooks overwrite `TaskTelemetry`. |
| Fail imports with unknown `demand_id` | A selected product-demand association should not silently import unlinked tasks. |
| Store department in JWT and expose `/api/me` | Department display should recover after reload even if localStorage was stale or empty. |
| Reuse an existing demand task group during AI import | Repeated AI decompositions should extend the same demand graph instead of creating detached groups. |
| Keep `frontend-design` as a documented fallback only when `taste-skill` is unavailable | The latest demand UI follow-up used the requested `design-taste-frontend` skill; fallback language remains only for future unavailable sessions. |
| Treat `未分配` and `无部门` as missing department values | Placeholder department data should not block a real department from a refreshed login/JWT or request body. |
| Generate stable `brain-{demand_id}` groups for ungrouped demands | The schedule record and AI shadow-task graph need the same durable join key. |
| Add readonly personal info in the avatar dropdown rather than editable settings or a main tab | Name, avatar, and department come from OS and should be displayed/refreshed by this app, not edited locally or promoted to primary navigation. |
| Remove repo auto-default from product demand creation | Product demands can exist before repo mapping; repository display belongs to AI/development task context. |
| Allow only known local users with matching cached password hashes during WellOS maintenance fallback | Availability improves during planned OS downtime without letting unknown users or wrong passwords bypass central authentication. |
| Gate local development auth behind `WELL_AMBIENT_DEV_AUTH=1` and loopback-only requests | Local debugging can continue during OS outages while production remains protected. |
| Land extension work as three bounded tracks | GitLab/Jira integration, KPI reporting, and AI demand analysis can progress independently while the main thread keeps integration and validation coherent. |
| Keep the extension MVP factual and auditable | The system goal is to reduce status meetings by deriving work facts from Jira/GitLab/AI/report data, not by generating unverifiable prose. |
| Keep UI control polish local to the affected dashboard/config components | The request was about native-looking controls and overflow, so broader layout, backend, and data-model changes would add unnecessary risk. |
| Treat AI estimates as audit data, not display-only UI metadata | Future accuracy evaluation needs the original deconstruction run, normalized overall estimate, subtask estimates, and actual completion telemetry to remain joinable. |
| Configure AI project context instead of relying on implicit model knowledge | Architecture, workflow, implemented capabilities, and estimation rules must be first-class prompt inputs to reduce estimate drift. |
| Make hours the primary deconstruction estimate unit | Operators tune task effort in hours; day values remain derived compatibility data for due-date and archive logic. |
| Store long domain capability catalogs as archived context documents and inject compact summaries into config | Long feature inventories are better versioned/audited in docs, while runtime prompt fields should stay concise enough for stable AI behavior. |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Initial file index missed `internal/server` | 1 | Used `find` and targeted reads to locate ignored backend files. |
| `go test` could not write the default Go build cache under sandbox | 1 | Re-ran with user-approved elevated test command. |
| Frontend build failed on invalid `{@const}` placement in `TaskKanban.svelte` | 1 | Removed the duplicate nested const in the Done lane. |
| `apply_patch` context failed after `gofmt` | 1 | Re-read current snippets, applied smaller-context patch, and recorded the tooling lesson. |
| `apply_patch` context failed while updating task memory | 1 | Re-read the current task plan tail and applied a smaller exact-context patch. |
| `apply_patch` context failed while appending Phase 12 task memory | 1 | Re-read current `task_plan.md` tail and applied an exact smaller-context patch. |
| Full Go tests failed in sandbox on `httptest.NewServer` loopback bind | 1 | Re-ran with `/tmp` Go cache and approved loopback access; full suite passed. |
