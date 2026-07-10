# Task Plan: Implementation Plan Check and Fix

## Current Task Addendum: Phase 50 Settings Column Height Synchronization
- [x] Remove the audit pane's independent desktop viewport height
- [x] Make the left configuration surface determine the shared desktop grid-row height
- [x] Keep version history and diff content inside bounded internal scroll regions
- [x] Match version-audit scrollbar width, track, thumb, border, and hover color to global tokens
- [x] Verify every integration page with an audit column plus the stacked responsive breakpoint

### Phase 50 Protection Rules
- The audit content must not contribute its long diff payload to the grid row's intrinsic height.
- Desktop columns must share an exact bottom edge; stacked layouts retain a bounded audit height.
- Scrollbars must reuse the global light-console treatment rather than legacy indigo/dark rules.

## Current Task Addendum: Phase 49 Settings Adaptive Layout And Control Consistency
- [x] Rebuild the configuration version inspector so history and diff preview remain readable without stretching to main-content height
- [x] Move breadcrumb context into the shared application shell and remove the duplicated Settings breadcrumb treatment
- [x] Remove GitLab overview repository truncation and bound long repository/version collections with internal scrolling
- [x] Normalize Project editor selects and shared Select hover/open states to the light admin visual contract
- [x] Normalize placeholder typography across shared inputs, comboboxes, and demand/task selection controls
- [x] Verify desktop/narrow layouts, repository counts, dropdown states, and production build/diff hygiene

### Phase 49 Protection Rules
- Preserve API contracts, save/rollback behavior, route permissions, and configured values.
- Collection height limits must create an internal scroll boundary; they must not hide records or truncate API data.
- Breadcrumbs are page context, not a nested button/card hierarchy. Every primary route should expose the same shared context bar.
- Native and custom selects must keep light neutral hover/open surfaces and readable placeholder sizing.

## Current Task Addendum: Phase 48 Unified Settings Main-Content Rebuild
- [x] Correct the scope report: the latest page-level rebuild covered GitLab only
- [x] Reconfirm the approved product-UI direction and preserve APIs, fields, navigation, and brand accent
- [x] Inventory every Settings route and classify page-level versus shared-shell work
- [x] Establish one reusable configuration-workbench contract for overview, edit, status, tables, and actions
- [x] Rebuild Feishu, Jira, project priority, AI engine, and system design content to the same contract
- [x] Verify the six rebuilt routes at desktop and narrow widths; inspect the five SettingsPanel-owned routes against their existing Phase 46 page structures
- [x] Run frontend build, targeted checks, diff hygiene, and browser visual review

### Phase 48 Scope Rule
- A page counts as rebuilt only when its own content structure and states use the shared workbench contract.
- Shared shell, audit-column, or token changes alone do not count as rebuilding a configuration page.
- Do not report completion until every listed route has been inspected and verified.

### Phase 48 Coverage Result
- Page-level rebuild in this phase: GitLab, Feishu, Jira, Project priority, AI engine, and System Design corpus.
- Existing page-level Phase 46 structures, re-audited but not newly rewritten in this phase: KPI, members, permission tree, authorization policies, and security audit.
- Browser validation: six rebuilt routes at 1280px and 390px, including overview/edit modes and bounded table/stepper overflow.

## Current Task Addendum: Phase 45 Settings Surface System Rebuild
- [x] Re-read repository boot, memory, and the current Settings visual baseline
- [x] Set the design read: product register, calm precision, spectacle 2, density 8
- [x] Audit every Settings child page and identify nested-container/capsule anti-patterns
- [x] Establish one shared Settings page, section, field, control, and action-bar system
- [x] Migrate GitLab, Feishu, Jira, project mapping, AI, context, authorization, and audit surfaces
- [x] Verify summary/edit/version states at desktop and narrow viewports
- [x] Run frontend build, diff hygiene, and visual pre-flight checks

### Phase 45 Validation Notes
- Browser inspection covered all 11 Settings routes plus Jira and AI edit modes.
- Desktop, 1024px, and 760px layouts have no configuration-content horizontal overflow.
- Browser-computed field wrapper audit returned zero decorated or nested field surfaces.
- The mobile shell keeps content visible and exposes Settings subnavigation through an off-canvas drawer.


## Goal
Inspect the repository against the provided implementation plan and apply the needed fixes that fit the current codebase, including the follow-up fixes for frontend skill rules, department display, demand scheduling date styling, and AI deconstruction binding.

## Current Task Addendum: Phase 40 High-Fidelity Admin Prototype Reset
- [x] Re-run frontend skill review with `finesse-skill`, `taste-skill`, `ui-skill`, and image-first guidance
- [x] Generate and inspect a high-fidelity visual reference before coding
- [x] Move prototype review to an unauthenticated standalone Vite page
- [x] Build a light, flat, frosted-glass admin prototype with dark rail, focal work area, task table, and right inspector
- [x] Configure Vite multi-page build output for `prototype.html`
- [x] Run frontend build, diff hygiene, and browser screenshot verification

## Current Task Addendum: Phase 41 Prototype-Aligned Multi-Agent Refactor
- [x] Pause the drifted page-by-page styling direction after user feedback
- [x] Recalibrate the visual target against `output/functional-admin-console-reference.png`
- [x] Create a shared UI/data contract before page agents edit production surfaces
- [x] Spawn one common worker for shared components, tokens, and data adapters
- [x] Spawn one worker per page with disjoint ownership and strict prototype alignment
- [x] Integrate workers in order: common contract first, then page slices
- [x] Run visual/static validation against prototype contract and frontend checks

## Current Task Addendum: Phase 42 Settings IA Correction
- [x] Convert flat Settings navigation groups into collapsible submenus
- [x] Replace the right-side mixed hero/metrics/category/inspector stack with a unified content frame
- [x] Add breadcrumb navigation above the actual Settings content
- [x] Preserve existing config, KPI, RBAC, authorization, and audit data flows
- [x] Run TypeScript, production build, and diff hygiene validation

## Current Task Addendum: Phase 43 Settings Rail Navigation Correction
- [x] Move Settings grouped submenus out of the Settings content area and into the global left rail/menu
- [x] Add Shell-level Settings submenu permission filtering, active-section highlight, and section navigation callback
- [x] Keep SettingsPanel as a single content frame with breadcrumb above actual business content
- [x] Preserve existing config, KPI, RBAC, authorization, and audit data flows
- [x] Run TypeScript, production build, and diff hygiene validation

## Current Task Addendum: Phase 44 Main Content Prototype Alignment
- [x] Change the frontend UI default skill rule to `ui-skill -> finesse-skill -> taste-skill`
- [x] Flatten `FunctionalWorkspace` so production pages do not render the old module hero by default
- [x] Tighten `FunctionalAdminShell` main content spacing, topbar, and light mist background to match the generated reference
- [x] Normalize migrated page headers into compact content toolbars instead of large descriptive hero blocks
- [x] Run TypeScript, production build, and diff hygiene validation

## Current Task Addendum: Phase 37 Modern Admin UI Prototype
- [x] Load planning-with-files, `finesse-skill` product UI guidance, and `ui-skill`
- [x] Set design read: product register, restrained glass, evidence-first, density 9
- [x] Audit current frontend surface and document highest-leverage UI risks
- [x] Add a reusable design-token substrate for the modern admin prototype
- [x] Build a coded modern admin prototype around exception, schedule, evidence, and override workflows
- [x] Expose the prototype safely without reviving rejected decision queue, delivery cockpit, or project health surfaces
- [x] Run frontend build and record validation

## Current Task Addendum: Strongest Brain Delivery Transformation Rollout
- [x] Commit all uncommitted workspace changes as a baseline before implementation
- [x] Load complex-task planning/execution guidance and frontend design constraints
- [x] Spawn subagents for backend evidence/exception, AI trace/readiness, and frontend cockpit slices
- [x] Inventory existing strongest-brain, context, policy, schedule, and dashboard surfaces
- [x] Integrate backend delivery cockpit/read-model APIs across evidence, exceptions, weekly decisions, AI trace, override, and authz explanation
- [x] Integrate frontend strongest-brain delivery cockpit without reviving the rejected old decision-queue surface
- [x] Add focused tests for new read-model contracts
- [x] Run targeted Go validation and frontend build
- [x] Update task memory and commit implementation changes

## Current Task Addendum: Remove Delivery Cockpit And Continue Phase APIs
- [x] Load the global `ui-skill` for the UI-related removal
- [x] Remove the visible strongest-brain delivery cockpit from the decision dashboard
- [x] Delete the unused frontend cockpit component so the rejected surface is not shipped
- [x] Keep backend evidence/read-model foundations available for later phase-specific pages
- [x] Add Phase 2 exception-center API as an independent endpoint
- [x] Add Phase 3 weekly-decision-center API as an independent endpoint
- [x] Add focused tests for exception and weekly decision contracts
- [x] Run targeted Go validation and frontend build

## Current Task Addendum: Phase 28 Schedule And Evidence UI Refinement
- [x] Load the global `ui-skill` for the UI-related refinement
- [x] Make red-zone diagnostic cards equal-height across the upper/lower card rows
- [x] Place story/bug type and Jira number on the same card header line
- [x] Upgrade the schedule risk calendar layout with a clearer command row, filter chips, and bucket hierarchy
- [x] Move the demand deconstruction engine from the evidence observatory into the schedule governance page
- [x] Run frontend validation

## Current Task Addendum: Phase 29 Red-Zone First Paint Filtering
- [x] Load the global `ui-skill` for the UI-related refresh bug
- [x] Diagnose the first-paint race between agenda loading and config/member filtering
- [x] Gate agenda rendering until config filtering is ready
- [x] Filter red-zone cards to stable Jira issue keys so Git branch slices do not appear as story cards
- [x] Run frontend validation

## Current Task Addendum: Phase 30 Project Health Telemetry Triage
- [x] Load global `finesse-skill` for the UI-related dashboard refactor
- [x] Reframe project health telemetry from a passive score table into an exception triage surface
- [x] Derive health level, weakest dimension, evidence gap, decision question, and manual intervention direction from existing scores
- [x] Add top-risk focus, priority intervention cards, and red/yellow/green summary counts
- [x] Replace raw metric columns with evidence micro-cells, weakest-dimension diagnosis, and manual intervention direction
- [x] Rework the project detail modal into a health diagnosis and intervention plan
- [x] Run frontend validation and diff hygiene

## Current Task Addendum: Phase 31 Hide Project Health Telemetry
- [x] Accept that the visible project telemetry panel still does not prove practical value
- [x] Hide the panel from the evidence observatory without deleting the component or backend score foundations
- [x] Keep the evidence observatory focused on task/code evidence already used by operators
- [x] Run frontend validation and diff hygiene

## Current Task Addendum: Board Cohesion, Audit Panel, And Modal Lock Polish
- [x] Inspect demand/task/bug/decision board components and shared state flow
- [x] Compact the effort summary and AI evaluation controls
- [x] Reduce and restyle the config version audit/rollback surface
- [x] Add or centralize modal background scroll locking
- [x] Fix assignee changes, Jira links, and detail actions on demand/decision boards
- [x] Improve seamless requirement/task/bug status transition feedback and strongest-brain guidance
- [x] Run focused validation, then stage and commit intentional changes

## Current Task Addendum: Strongest Brain Master Plan Landing, Intent Recognition, And Summary
- [x] Commit all existing workspace changes as a baseline before new implementation
- [x] Load planning, execution, and frontend taste guidance
- [x] Update the master plan with AI intent recognition and summarization direction
- [x] Use subagents for backend, frontend, and QA/integration tracks
- [x] Implement strongest-brain v1 read models, decision queue, evidence chain, and AI intent/summary API
- [x] Implement the visible strongest-brain cockpit and AI intent/summary interaction surface
- [x] Integrate subagent work, resolve conflicts, and run focused validation
- [x] Update task memory and commit intentional changes

### Backend Strongest-Brain Worker Scope
- [x] Read master plan, server handlers, and related DB models
- [x] Implement backend-only EvidenceChain and DecisionQueueItem read APIs
- [x] Reuse existing schedule governance rows where possible
- [x] Add deterministic AI intent recognition and summary v1 API with optional configured AI call
- [x] Add focused backend tests and run targeted validation
- [x] Return worker output for main rollout integration and commit

## Current Task Addendum: Phase 23 Schedule Governance Risk Calendar
- [x] Restore current plan context and classify the continuation as `coding.complex`
- [x] Choose Phase 4 risk calendar as the next executable slice after Phase 22
- [x] Spawn backend, frontend, and QA/integration subagents with disjoint scopes
- [x] Add `/api/schedule/risk-calendar` read model without changing `/api/schedule`
- [x] Add a compact risk calendar surface to the demand schedule view
- [x] Integrate subagent work, run focused validation, and record limits
- [x] Commit intentional Phase 23 changes

## Current Task Addendum: Phase 24 Decision Queue Meaning
- [x] Treat the feedback as a strongest-brain cockpit usefulness gap
- [x] Preserve the existing dark operational dashboard style
- [x] Keep backend queue contracts unchanged and improve the frontend read model projection
- [x] Surface queue source, current decision focus, decision kind, idle cost, and handling entry
- [x] Verify the frontend production build
- [x] Update task memory and commit intentional changes

## Current Task Addendum: Phase 25 Remove Decision Queue Surface
- [x] Accept the product judgment that the visible strongest-brain decision queue remains low-value
- [x] Remove the decision queue section from the decision dashboard
- [x] Remove the frontend strongest-brain queue fetch and 15s polling
- [x] Remove queue-only types, derived state, handlers, styles, and responsive rules
- [x] Keep the existing actionable Agenda metrics, filters, and manual intervention workflow
- [x] Verify frontend production build and commit intentional changes

## Current Task Addendum: GitLab Webhook Ensure/Status MVP
- [x] Inspect existing GitLab/Jira integration and route patterns
- [x] Add GitLab project webhook ensure/status backend capability
- [x] Register protected API routes
- [x] Add httptest-backed GitLab API regression tests
- [x] Run targeted Go validation

## Current Task Addendum: Demand Schedule Table Polish
- [x] Fix native-looking schedule table scrollbars
- [x] Make the demand scheduling modal opaque and layout-stable
- [x] Add optional AI effort estimation to the scheduling flow
- [x] Split schedule, delivery evidence, and update time into clear table columns

## Current Task Addendum: Phase 34 KPI Interaction Stability And Analysis Depth
- [x] Apply `finesse-skill` product UI guidance for the KPI dashboard follow-up
- [x] Fix click/refresh jumping by keeping the loaded dashboard mounted during data refresh
- [x] Remove active-state scale transforms from KPI controls so clicks do not resize the layout
- [x] Reserve fixed slots for sync feedback and dynamic KPI panels so daily/weekly switching does not change page geometry
- [x] Add depth diagnostics for delivery concentration, evidence coverage, risk load, review pressure, Bug share, and score spread
- [x] Add breadth distribution for issue mix, score bands, department coverage, and average cycle time
- [x] Run frontend production build and diff hygiene

## Current Task Addendum: Phase 35 Jira Execution Tracking Fact Alignment
- [x] Trace the "Jira Task 开发结果追踪" view from `TaskKanban.svelte` to `/api/execution/tasks`
- [x] Confirm execution rows were using child execution task assignee/status even when the bound Jira Task demand had changed
- [x] Make bound Jira Task demand assignee/status drive execution tracking display and result state
- [x] Preserve child execution assignee as secondary evidence instead of overwriting it
- [x] Move execution assignee/search filtering after DTO construction so filters use effective Jira facts
- [x] Add regression coverage for Jira completion plus reassignment
- [x] Run focused backend and frontend validation

## Current Phase
Phase 37: Modern Admin UI Prototype

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

### Phase 18: Demand Creation Sources And Jira Task Demand Model
- [x] Keep the new-demand modal stable while typing and add a project selector
- [x] Expand demand assignee/project candidates beyond the current logged-in user
- [x] Treat Jira `Task` as a schedulable demand while keeping `Bug` in the defect flow
- [x] Update the demand/bug lifecycle design documentation
- [x] Run focused backend and frontend validation
- **Status:** complete

### Phase 19: WellOS User Info Profile Refresh
- [x] Call the WellOS user-info endpoint after successful login
- [x] Use `avatar`, `realname`, and `department_name` to update local user profile and JWT claims
- [x] Preserve degraded login behavior without calling external profile APIs
- [x] Add focused login/profile regression coverage
- **Status:** complete

### Phase 20: Demand Schedule Table And Effort Estimate Polish
- [x] Add row-level `scheduled` to `/api/schedule` so the UI does not infer real scheduling from `backlog`
- [x] Persist optional AI estimate hours, days, and difficulty through `/api/tasks/schedule`
- [x] Split schedule table columns into schedule, effort, delivery evidence, and update time
- [x] Restyle schedule table scrollbars and the scheduling modal/date picker surface
- [x] Update the schedule MVP documentation and add focused regression assertions
- **Status:** complete

### Phase 21: Board Cohesion, Audit Panel, And Modal Lock Polish
- [x] Use the required `design-taste-frontend` guidance as a constrained internal-dashboard repair
- [x] Locate the current work summary, audit/rollback, decision board, demand board, and modal implementations
- [x] Implement compact effort-summary controls and custom difficulty select
- [x] Make config version audit page-specific, compact, and non-native-scrollbar styled
- [x] Ensure open modals/dialogs lock background scrolling across pages
- [x] Fix assignee mutation propagation and demand board action click targets
- [x] Add cohesive status-transition UX and strongest-brain recommendation dimensions
- [x] Run focused frontend/backend validation and commit intentional changes
- **Status:** complete

### Phase 22: Strongest Brain Master Plan Landing, Intent Recognition, And Summary
- [x] Create a checkpoint commit for all existing workspace changes
- [x] Read `planning-with-files`, `executing-plans`, and `design-taste-frontend`
- [x] Spawn backend, frontend, and QA/integration subagents with disjoint scopes
- [x] Add AI intent recognition and summarization to the master plan
- [x] Add backend strongest-brain read models and API endpoints
- [x] Add deterministic AI intent/summary fallback with optional configured-AI path
- [x] Add frontend strongest-brain cockpit and transparent AI intent/summary conversation panel
- [x] Add focused tests and run validation
- [x] Commit intentional Phase 22 changes
- **Status:** complete

### Phase 23: Schedule Governance Risk Calendar
- [x] Restore planning context and avoid unrelated dirty runtime files
- [x] Read Phase 4 from the strongest-brain master plan
- [x] Spawn backend, frontend, and QA/integration subagents
- [x] Implement schedule risk calendar API over existing schedule risk rules
- [x] Implement weekly/monthly risk calendar strip in the schedule workbench
- [x] Add focused tests/build validation
- [x] Update task memory and commit intentional changes
- **Status:** complete

### Phase 24: Decision Queue Meaning
- [x] Read current decision queue UI and strongest-brain API projection
- [x] Preserve API compatibility while carrying backend `source` into the UI
- [x] Add a selected decision focus panel with idle-cost and handling-entry language
- [x] Add per-card decision kind, queue source, and no-action cost
- [x] Run frontend production build
- [x] Update memory files and commit intentional changes
- **Status:** complete

### Phase 25: Remove Decision Queue Surface
- [x] Treat the latest user feedback as a removal request, not another iteration
- [x] Remove visible strongest-brain decision queue markup from `DecisionDashboard.svelte`
- [x] Remove frontend queue fetch, polling, queue-only state, and derived helpers
- [x] Remove queue-only CSS and responsive remnants
- [x] Run frontend production build
- [x] Update memory files and commit intentional changes
- **Status:** complete

### Phase 26: Worker B AI Trace And Requirement Clarification Backend Slice
- [x] Follow AGENTS cold start and classify as `coding.complex`
- [x] Read strongest-brain delivery plan and current AI/context archive handlers
- [x] Add AI output trace/context-pack replay read model without route registration
- [x] Add requirement clarification read model
- [x] Return archive/context trace fields from import/deconstruct responses where available
- [x] Add focused non-external-AI tests
- [x] Run targeted validation after parallel strongest-brain compile blockers are resolved
- **Status:** complete

### Phase 27: Remove Rejected Delivery Cockpit And Keep Focused APIs
- [x] Remove the visible strongest-brain delivery cockpit surface after the user rejected its product value
- [x] Keep backend evidence, exception, weekly-decision, AI trace, override, and permission explanation foundations reusable
- [x] Split exception handling and weekly decision review into focused read APIs instead of one broad cockpit
- [x] Preserve the rule that future strongest-brain UI must prove a concrete operator workflow before becoming visible
- **Status:** complete

### Phase 30: Project Health Telemetry Triage And Manual Intervention
- [x] Apply `finesse-skill` as the default UI skill for the dashboard refactor
- [x] Reframe the project health panel from a passive score table into an exception triage surface
- [x] Derive red/yellow/green health levels, weakest dimension, evidence gap, decision question, and manual intervention direction from existing scores
- [x] Add a priority intervention strip so the highest-risk project has a clear next human action
- [x] Replace the wide metric table with evidence micro-cells, weakest-dimension diagnosis, and manual intervention columns
- [x] Rework the project detail modal into a health diagnosis and intervention plan view
- [x] Remove new ProjectHealthTelemetry build warnings and run production build validation
- **Status:** complete

### Phase 31: Hide Project Health Telemetry
- [x] Remove the `ProjectHealthTelemetry` mount from the evidence observatory
- [x] Remove the now-unused frontend import from `App.svelte`
- [x] Preserve the component file for a future value-defined redesign or deletion pass
- [x] Run frontend production build and diff hygiene
- **Status:** complete

### Phase 32: KPI R&D Performance Dashboard Refactor
- [x] Apply `finesse-skill` product UI guidance for the KPI dashboard redesign
- [x] Make daily/weekly report granularity the primary control and bind it to the API period contract
- [x] Replace the old summary/ranking layout with a report brief, team average score, report preview, member matrix, and department mix
- [x] Derive explainable member scores from output, code evidence, flow efficiency, and risk health using existing KPI fields
- [x] Show each member's linked task, demand, Bug, MR, review, cycle, overdue, and score breakdown dimensions in one dense table
- [x] Remove stale KPI leaderboard CSS and add layout-stable responsive styles for the refactored surface
- [x] Run frontend production build validation
- **Status:** complete

### Phase 33: KPI Core Member Data Boundary
- [x] Locate existing coreMember conventions in decision, task, and demand dashboards
- [x] Add backend KPI filtering based on `jira.sync_users`, falling back to `assignee in (...)` from custom JQL
- [x] Filter KPI completed tasks, active tasks, users, summaries, departments, report sections, risks, meeting focus, and evidence before aggregation
- [x] Support display name, username, email prefix, and first-token matching for core member identity resolution
- [x] Preserve existing behavior when no core members are configured
- [x] Add regression coverage proving non-core member work does not leak into KPI performance or report preview
- **Status:** complete

### Phase 34: KPI Interaction Stability And Analysis Depth
- [x] Keep the KPI dashboard mounted after the first successful load and show refresh state inline
- [x] Prevent repeated report-mode clicks while a KPI refresh is already running
- [x] Replace click scale feedback with stable border/background feedback
- [x] Keep sync indicators, report preview, and member matrix height stable across daily/weekly data changes
- [x] Add a depth diagnosis panel that explains dependency, evidence, risk, review, quality, and score variance
- [x] Add a breadth map that shows task/demand/Bug mix, score-band distribution, department coverage, and average cycle
- [x] Run production build validation and diff hygiene
- **Status:** complete

### Phase 35: Jira Execution Tracking Fact Alignment
- [x] Keep Jira Task demand as the authoritative owner/status source for bound execution rows
- [x] Add `execution_assignee`, `jira_assignee`, and `jira_status` to execution tracking DTOs
- [x] Use effective Jira assignee/status for table display, summary counts, risk/result state, and assignee filtering
- [x] Show old child execution assignee as a secondary note in the tracking table
- [x] Stop preserving local assignee overrides when Jira reports the issue as completed
- [x] Keep recently completed Jira Task/Bug keys in correction sync so post-completion assignee changes refresh
- [x] Add tests proving reassigned/completed Jira parent facts are reflected in execution tracking
- [x] Run focused Go test, frontend build, and diff hygiene
- **Status:** complete

### Phase 36: Core Member Visibility Boundary
- [x] Add a shared coreMember visibility helper based on `jira.sync_users` with custom JQL assignee fallback
- [x] Filter `/api/tasks` so non-coreMember task rows are not returned
- [x] Filter `/api/schedule` demands and schedule subtask stats to coreMember-owned data
- [x] Filter `/api/execution/tasks` by effective Jira/current owner and redact non-core secondary assignee fields
- [x] Keep KPI report evidence constrained to included coreMember users even if KPI internals use broader context later
- [x] Remove visible `外部协同` task/schedule dropdown entries from the affected frontend surfaces
- [x] Add regression tests for task, schedule, and execution visibility boundaries
- [x] Run focused Go tests, frontend build, and diff hygiene
- **Status:** complete

### Phase 37: Modern Admin UI Prototype
- [x] Record the Phase 37 product UI design read and protection rules
- [x] Add global modern-admin design tokens for restrained glass, semantic color, density, focus, and motion behavior
- [x] Add a static coded prototype for exception command, weekly decisions, schedule governance, evidence replay, and override preflight
- [x] Gate the prototype behind `config:read` as a preview tab in the existing app shell
- [x] Run `pnpm build` and `git diff --check`
- [x] Start Vite dev server for browser preview on the first available local port
- **Status:** complete

### Phase 38: Finesse Componentized Admin Prototype Refactor
- [x] Re-apply `finesse-skill` as a product UI redesign rather than a static concept board
- [x] Flatten the modern-admin token substrate into a restrained charcoal/acrylic management-console system
- [x] Split the prototype into local Shell, Metric, Badge, Table, and Inspector components
- [x] Rebuild the prototype around a dense command table, command toolbar, status metrics, weekly decision queue, workflow path, and right-side mediation preflight drawer
- [x] Preserve the `config:read` gated `UI 原型` entry instead of replacing production pages
- [x] Run frontend production build, `git diff --check`, and `pnpm check` diagnostics
- [x] Attempt in-app browser verification and record the login boundary
- **Status:** complete

### Phase 39: Bold Beauty Admin Prototype Pass
- [x] Route the user feedback as a `finesse-skill` bolder iteration with product UI boundaries
- [x] Upgrade modern-admin tokens from restrained charcoal to editorial control-room acrylic with stronger cyan focus, deeper substrate, richer grain, and stronger depth
- [x] Add `PrototypeTheatre` as the first-screen visual center for selected issue focus, signals, owner, due date, and decisive actions
- [x] Rework `PrototypeShell` into a stronger ambient stage with a bolder rail, brand mark, and active lane treatment
- [x] Refine Metric, Badge, Table, and Inspector components for sharper hierarchy, glow discipline, richer borders, and more elegant micro-interactions
- [x] Keep production pages, auth gates, and backend untouched
- [x] Run frontend production build, filtered `pnpm check` diagnostics, and diff hygiene
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
| Treat Jira Task as a demand in this deployment | The team uses Jira Task to record real product and development requirements, so Jira Task must enter demand scheduling rather than execution-only tracking. |
| Fetch WellOS avatar and department from `/api/user/info` | The login response no longer carries authoritative avatar and department fields; profile refresh must use the token-backed user-info endpoint. |
| Treat `backlog` as workflow state, not proof of schedule | A demand is only truly scheduled when it has both a development branch and a due date; delivery evidence and update time should live in separate columns. |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| ProjectConfig large patch context mismatch | 1 | Split the edit into small exact-context patches after re-reading the current markup |
| Vite preview `listen EPERM` in sandbox | 1 | Restarted the local preview with the approved server permission. |
| Exploratory `rg` commands used unmatched zsh globs | 1 | Switched to `rg --glob` and explicit paths. |
| Browser metric probe used unavailable `parseFloat` | 1 | Replaced it with numeric string normalization in the next read-only probe. |
| KPI mock response caused a browser-only shape error | 1 | Classified as isolated fixture noise; production build and KPI source were not changed. |
| Initial file index missed `internal/server` | 1 | Used `find` and targeted reads to locate ignored backend files. |
| `go test` could not write the default Go build cache under sandbox | 1 | Re-ran with user-approved elevated test command. |
| Frontend build failed on invalid `{@const}` placement in `TaskKanban.svelte` | 1 | Removed the duplicate nested const in the Done lane. |
| `apply_patch` context failed after `gofmt` | 1 | Re-read current snippets, applied smaller-context patch, and recorded the tooling lesson. |
| `apply_patch` context failed while updating task memory | 1 | Re-read the current task plan tail and applied a smaller exact-context patch. |
| `apply_patch` context failed while appending Phase 12 task memory | 1 | Re-read current `task_plan.md` tail and applied an exact smaller-context patch. |
| Full Go tests failed in sandbox on `httptest.NewServer` loopback bind | 1 | Re-ran with `/tmp` Go cache and approved loopback access; full suite passed. |
