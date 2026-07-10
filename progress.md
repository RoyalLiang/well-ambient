# Progress Log

## Session: 2026-07-10 — Controlled Autonomous Delivery Rollout

### Phase 0: Baseline and planning
- **Status:** complete
- Loaded the repository cold-start rules, design routing, memory guidance, and `planning-with-files` skill.
- Audited the existing strongest-brain plan, intent API, context-pack deconstruction, archive/import path, GitLab telemetry, and AI MR review.
- Checkpointed all pre-existing uncommitted work as `8f50f00 chore: checkpoint admin console and strongest brain baseline`.
- Began the versioned demand-spec, review-contract, controlled-execution, reviewer-resolution, and corpus-feedback rollout plan.
- Added `docs/controlled-autonomous-delivery-plan.md` with lifecycle, contracts, safety boundaries, phase acceptance, and delivery definition.

### Phase 1: Persistence and state machine
- **Status:** complete
- Next action: inspect DB migration, permission seeding, policy conventions, and GitLab configuration before adding models and transition helpers.
- Confirmed GORM AutoMigrate, incremental permission seeds, existing policy wrapper, `RepoMapping` configuration, and an `httptest`-friendly GitLab HTTP pattern.
- Added the four persistence roots, append-only action log, incremental permissions, and a pure execution-run state machine with tests.
- First targeted Go test attempt was blocked before compilation by the sandboxed default Go build cache; no code diagnostic was produced.
- Policy-compliant rerun passed: `go test ./internal/delivery ./internal/db/...`.

### Phase 2: Human-reviewed executable specification
- **Status:** complete
- Implement draft/create/update/read/freeze behavior and review-contract manage/approve behavior with frozen-version immutability.
- Added authenticated, permission-gated spec and contract routes.
- Added archive hydration so a deconstruction archive can initialize the human-reviewable specification while retaining context-pack provenance.
- Added default review-contract generation, approval checks, freeze readiness gates, immutable frozen versions, and stale-run invalidation.
- Focused lifecycle and readiness-blocker tests pass.

### Phase 3: Reviewer resolution and preflight
- **Status:** complete
- Implement deterministic reviewer resolution, protected-path rules, segregation of duties, validated change-set artifacts, and structured preflight checks.
- Added protected-path glob handling, minimum approvals, author/unavailable exclusion, and structured blocker reasons.
- Added permission-gated execution preflight that verifies frozen spec, approved contract, mapped/configured repository, safe file actions, test policy, and reviewer resolution.
- Pure resolver/change-set tests and the endpoint integration test pass.

### Phase 4: Controlled execution and GitLab adapter
- **Status:** complete
- Add idempotent execution runs/actions and an `httptest`-covered GitLab branch/commit/Draft-MR client.
- Implemented run create/list/start/cancel, append-only actions, safe topic branch naming, GitLab project/reviewer/branch/commit/Draft-MR operations, and evidence projection back to the demand.
- First HTTP contract test attempt compiled, then the sandbox blocked the local `httptest.Server` listener before requests executed.
- Added frozen-spec AI change-set generation with source-file budgets, update-path allowlisting, deletion bans, and completions/responses protocol support.
- Added GitLab Pipeline refresh and test-failure state handling.
- Escalated local-only HTTP contract tests pass, including request shape, reviewer IDs, branch/commit/MR sequence, pipeline result, evidence projection, and idempotency.

### Phase 5: Demand workspace and human verification
- **Status:** complete
- Use the UI design-system skill in the existing light admin-console register.
- Move deconstruction/intent work into the demand workspace and add a compact demand-detail delivery control instead of a new cockpit.
- Added `DemandDeliveryControl.svelte` with spec review, contract approval, freeze, AI generation, execute, MR/CI evidence, cancel and named-owner acceptance actions.
- Moved Deconstructor from evidence to the demand-board workspace and made task import create a reviewable spec draft from its archive.
- Added `execution:accept`, named acceptance-owner enforcement, and MR webhook delivery reconciliation.
- `pnpm --dir web build` passes. `pnpm --dir web check` reports only the eight existing `TaskKanban.svelte` `parentDemand` nullability errors; the new component has no errors.
- Human acceptance and MR reconciliation focused Go tests pass.

### Phase 6: Delivery feedback and governed candidates
- **Status:** complete
- Generate idempotent delivery candidates and require explicit review before creating active context facts.
- Added delivery-case, requirement-pattern and risk-rule candidates with provenance, confidence and scope.
- Added permission-gated list/review APIs; acceptance creates a versioned active Context Fact, rejection creates none, and repeated decisions are idempotent.
- Added a compact candidate-review surface to the existing System Design corpus page and refreshes the fact list after promotion.
- Focused candidate quarantine/promotion tests and frontend production build pass.

### Phase 7: Integration and delivery
- **Status:** complete
- Run full backend regression, final frontend build/check, diff hygiene, update plan/progress/findings, and commit.
- Frontend `pnpm --dir web check` now passes with 0 errors and 65 existing warnings after replacing four unsafe `parentDemand` narrowings with existing safe helpers.
- First full Go regression found two baseline failures: local user omission in demand options and a missing evidence-gap decision item. Both were resolved with narrow compatibility fixes that preserve the configured visibility boundary and raw execution evidence.
- Final `GOCACHE=/tmp/well-ambient-gocache go test ./...` passes across all Go packages.
- Final `pnpm --dir web build` passes; `pnpm --dir web check` passes with 0 errors and 65 pre-existing warnings.
- Final `git diff --check` passes. Background-generated `task_status.md` and `well-ambient.db` changes remain intentionally outside the delivery commit.

## Session: 2026-07-10 - Phase 50 Settings Column Height Synchronization

### Implementation And Validation
- **Status:** complete
- Replaced the independent sticky audit height with a desktop grid-stretch contract driven by the left configuration surface.
- Removed audit diff content from intrinsic grid sizing through an absolutely contained panel, preventing long payloads from inflating both columns.
- Restored normal-flow positioning at the stacked breakpoint and retained the responsive audit height cap.
- Replaced legacy local indigo/dark scrollbar styling with the global light-console scrollbar contract for version and diff regions.
- Browser-checked all five integration pages with audit columns at `1600x900`; every left/right height delta is `0px`.
- Browser-checked the `1200x760` stacked layout; no horizontal overflow and internal diff scrolling remain intact.
- `pnpm --dir web build` and targeted `git diff --check` pass. Existing repository-wide Svelte accessibility warnings remain outside this CSS correction.

### Tool Note
- The browser wrapper does not expose `playwright.setViewportSize`; responsive verification uses the documented browser viewport capability instead.

## Session: 2026-07-10 - Phase 49 Settings Adaptive Layout And Control Consistency

### Audit And Root-Cause Pass
- **Status:** complete
- Confirmed the audit panel stretch/compression conflict, duplicated local breadcrumb ownership, GitLab `slice(0, 5)` truncation, mixed Project select implementations, and competing hover/placeholder rules.
- Rebuilt the real Settings QA entry around `SettingsPanel`; the first reload exposed and then fixed a missing explicit `settingsInspector` declaration in the reactive async-summary update.
- Browser verification also exposed stale breadcrumb counts caused by hidden dependencies inside `integrationDetail()`. The breadcrumb now reads the already-reactive inspector fact instead of invoking the untracked helper directly.

### Tool Notes
- A repository-root `package.json` read failed because this workspace keeps the frontend package at `web/package.json`; subsequent package commands use `pnpm --dir web`.
- The in-app locator wrapper does not expose `hover()`. Hover and open states share the same final CSS lock, so browser validation uses the real clicked/open state and computed backgrounds instead of simulating a hover through page evaluation.

### Implementation And Validation
- **Status:** complete
- Added shared non-Settings breadcrumb rendering through `FunctionalWorkspace`; Settings keeps its child-aware breadcrumb but no longer renders a nested first-item button.
- Moved Settings category context exclusively into the breadcrumb and reduced the large summary header from four facts to three operational facts.
- Rebuilt the version inspector with a 420px desktop width, viewport-bounded sticky height, separately scrollable version/diff regions, formatted JSON, and responsive full-width/narrow modes.
- Raised version retrieval from 12 to 40, removed the page-level eight-version/eight-diff truncation, and kept long collections bounded by internal scrolling.
- Removed GitLab's five-row overview slice and added a 360px repository table scroll boundary with sticky headings.
- Replaced Project's bespoke Project Key dropdown and searchable phase/priority fields with shared Select modes.
- Locked shared/native select open and hover surfaces to the light palette and set shared/global input placeholders to 12px.
- `pnpm --dir web build` passes and target `git diff --check` passes.
- Full `pnpm --dir web check` returns only the existing eight `TaskKanban.svelte` `parentDemand` nullability errors and 65 existing warnings; Phase 49 files add no diagnostics.
- Browser verification covers 1600x900, 1280x720, and 390x844 layouts with no document-level horizontal overflow.

## Session: 2026-07-10 - Phase 48 Unified Settings Main-Content Rebuild

### Scope Correction And Audit
- **Status:** complete
- Confirmed that the latest page-level implementation covered GitLab only.
- Separated shared-shell changes from actual child-page rebuilds.
- Loaded Self-Improving, UI design-system, finesse product-UI/redesign guidance, taste anti-slop constraints, and file-based planning guidance.
- Next action: inventory every Settings route, define the shared workbench contract, then rebuild the remaining child pages.

### Error Log
- `ProjectConfig.svelte` large patch failed on attempt 1 because a malformed deletion marker made the expected context invalid. No file changes were applied; retry strategy changed to small exact-context patches.
- A read attempted `web/src/prototype.ts`, but the repository uses `web/src/prototype-entry.ts`; the correct entry was read on the next inspection.
- In-app preview screenshot capture closed the active tab target after all DOM and layout measurements had completed. The tab binding was reacquired for the final static check; no application error or console failure accompanied the tool-side closure.
- The final attempt to keep Vite Preview running on `127.0.0.1:4175` was blocked by sandbox `EPERM`; the required escalation was then rejected because the current Codex usage limit prevented approval. No workaround was attempted. The built `dist/settings-preview.html` artifact and completed browser QA remain valid, but no persistent review URL is available at handoff.

### Shared Contract And Child Pages
- **Status:** complete
- Added `web/src/styles/settings-config-workbench.css` and loaded it from `SettingsPanel.svelte`.
- Migrated Feishu, Jira, Project priority, AI engine, and system-design corpus markup to the shared workbench hierarchy.
- Replaced false ready/connected defaults with explicit unchecked, incomplete, checking, error, and success states.
- Corrected AI engine configuration detection so context facts alone do not mark the model engine configured.
- Removed obsolete page-local style blocks and retained only component-specific rules; subsequent production builds pass without target-page unused-selector warnings.
- Added `settings-preview.html`, a local-only Vite entry that mounts the real six config components behind deterministic API fixtures for visual QA without weakening production authentication.
- Desktop browser review now covers GitLab, Feishu, Jira, Project priority, and AI engine overview/edit states with no document-level horizontal overflow.

### Responsive QA And Final Validation
- **Status:** complete
- System-design list/editor and context-pack result states pass desktop overflow checks.
- First 390px pass found and corrected GitLab grid min-content overflow; the other five child pages stayed within the document width, with Project table overflow isolated to its own scroll container.
- GitLab, Feishu, Jira, Project, and AI narrow edit states now pass document-width checks; Feishu intentionally keeps its four-step control internally scrollable.
- `pnpm --dir web build` passes. Full `pnpm --dir web check` remains blocked only by eight existing `TaskKanban.svelte` nullability errors; no target Settings file reports an error.
- Target-file `git diff --check` passes. Route coverage and the built local QA entry are recorded; a persistent preview process could not be retained because sandbox escalation was unavailable.


## Session: 2026-07-10 - Phase 45 Settings Surface System Rebuild

### Phase 1: Audit and shared system
- **Status:** complete
- **Started:** 2026-07-10
- Actions taken:
  - Loaded the UI design system, existing-project redesign, finesse product UI, and file-planning guidance.
  - Reconfirmed the light admin-console baseline and the user's preference for restrained glass.
  - Recorded the explicit ban on nested container/capsule/control composition.
  - Removed redundant `section-card` wrappers from GitLab, Feishu, Jira, project, AI engine, and AI context routes.
  - Added the Phase 46 flat workbench contract: one glass surface per page column, definition-list summaries, flat form grids, flat audit rows, and restrained semantic states.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/src/components/SettingsPanel.svelte`

### Phase 2: Browser visual verification and correction
- **Status:** complete
- Actions taken:
  - Production build passed after the structural and visual-system change.
  - Diff hygiene passed for the touched files.
  - Ran in-app Playwright inspection against an isolated local API fixture across all 11 Settings routes.
  - Fixed version-detail compression, rounded version/summary rows, context option wrapping, project table overflow, and permission branch card nesting.
  - Added shared scroll reset so configuration edit modes and menu route changes always open at the workbench top.
  - Rebuilt the <=860px shell as an off-canvas navigation drawer and verified open/close state at 760px.
  - Removed the isolated API fixture and restored the normal Vite proxy after visual verification.
- Files created/modified:
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/config/GitLabConfig.svelte`
  - `web/src/components/config/FeishuConfig.svelte`
  - `web/src/components/config/JiraConfig.svelte`
  - `web/src/components/config/AIConfig.svelte`
  - `web/src/components/config/ProjectConfig.svelte`
  - `web/src/lib/settings-ui.ts`


## Session: 2026-07-07

### Phase 40: High-Fidelity Admin Prototype Reset
- **Status:** complete
- **Started:** 2026-07-07
- Actions taken:
  - Re-ran the UI review with `finesse-skill`, `taste-skill`, `ui-skill`, and the image-first workflow because prior coded passes still did not meet the visual bar.
  - Generated a high-fidelity reference image for a light, flat, frosted-glass management console and copied it to `output/modern-admin-reference-phase40.png`.
  - Added `web/prototype.html` as an unauthenticated standalone Vite entry so the prototype can be reviewed without logging into the main app.
  - Added `web/src/prototype-entry.ts` and `web/src/components/prototype/StandaloneAdminPrototype.svelte`.
  - Reframed the prototype around a dark left rail, clean top command bar, large focus card, metric stack, risk/evidence task table, and right-side inspector.
  - Updated `web/vite.config.ts` so both `index.html` and `prototype.html` are emitted in production builds.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/vite.config.ts`
  - `web/prototype.html`
  - `web/src/prototype-entry.ts`
  - `web/src/components/prototype/StandaloneAdminPrototype.svelte`
  - `output/modern-admin-reference-phase40.png`
- Validation:
  - `pnpm build` in `web` passed and emitted `dist/prototype.html`.
  - `git diff --check` passed.
  - `pnpm check` still reports the three known non-prototype TypeScript errors in `TaskKanban.svelte` and `ProjectConfig.svelte`; no prototype-specific errors were reported.
  - Browser screenshot verification at `http://127.0.0.1:5174/prototype.html` confirmed the standalone prototype renders the left rail, focus card, task table, and inspector.

## Session: 2026-07-06

### Phase 37: Modern Admin UI Prototype
- **Status:** complete
- **Started:** 2026-07-06
- Actions taken:
  - Loaded planning-with-files, `finesse-skill` product UI guidance, and `ui-skill`.
  - Set the design read to product register, restrained glass, evidence-first, SPECTACLE 2, DENSITY 9.
  - Recorded the prototype protection rules: do not revive the rejected decision queue, delivery cockpit, or project health telemetry surfaces.
  - Started with a coded prototype and design-token substrate rather than rewriting existing business pages.
  - Added `modern-admin-tokens.css` with restrained acrylic, semantic status colors, density spacing, focus rings, and reduced-motion behavior.
  - Added a static coded `ModernAdminPrototype` surface covering exception command, weekly decisions, schedule governance, evidence replay, and override preflight.
  - Exposed the prototype as a `config:read`-gated `UI 原型` tab in `App.svelte`.
  - Started Vite dev server for local review; sandboxed bind failed, approved local bind succeeded and Vite selected port 5174 because 5173 was occupied.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/src/main.ts`
  - `web/src/App.svelte`
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/ModernAdminPrototype.svelte`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Local preview is running at `http://127.0.0.1:5174/`.
  - Browser page-text check confirmed the app is reachable but the in-app browser is on the login screen, so visual prototype review requires logging in and opening the `UI 原型` tab.

## Session: 2026-07-05

### Phase 31: Hide Project Health Telemetry
- **Status:** complete
- **Started:** 2026-07-05
- Actions taken:
  - Accepted the product judgment that the visible project telemetry panel still does not prove its practical operator value.
  - Removed `ProjectHealthTelemetry` from the evidence observatory render tree.
  - Removed the now-unused `ProjectHealthTelemetry` import from `App.svelte`.
  - Kept `ProjectHealthTelemetry.svelte` in the codebase so the scoring foundation can be reworked or deleted later with a clearer product contract.
- Files modified:
  - `web/src/App.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `git diff --check` passed.
  - `pnpm build` in `web` passed with existing Svelte warnings in other components.

### Phase 30: Project Health Telemetry Triage And Manual Intervention
- **Status:** complete
- **Started:** 2026-07-05
- Actions taken:
  - Applied `finesse-skill` guidance as a restrained product-dashboard refactor.
  - Rebuilt the expanded project health panel around an exception command board: top risk focus, red/yellow/green counts, average health, and priority intervention cards.
  - Derived each project's health level, weakest dimension, evidence gap, decision question, and manual intervention direction from existing SH/EQ/CE/SI/PHDI fields.
  - Replaced raw table columns with compact evidence micro-cells, weakest-dimension diagnosis, and a dedicated manual intervention column.
  - Reworked the details modal into an intervention dossier with urgency, decision question, weakest evidence, human action, source route, and diagnostic evidence.
  - Removed new `ProjectHealthTelemetry.svelte` build warnings by changing the modal close path and deleting unused CSS.
- Files modified:
  - `web/src/components/ProjectHealthTelemetry.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `git diff --check` passed.
  - `pnpm build` in `web` passed; remaining Svelte warnings are existing warnings in other components, not in `ProjectHealthTelemetry.svelte`.

## Session: 2026-07-04

### Phase 29: Red-Zone First Paint Filtering
- **Status:** complete
- **Started:** 2026-07-04
- Actions taken:
  - Loaded the global `ui-skill` before changing the red-zone UI behavior.
  - Confirmed the screenshot showed lower-case Git branch/commit-derived IDs such as `revert-4`, `with-5`, and `middleq-...` being rendered as story cards during the first paint.
  - Changed dashboard bootstrap so config/member filtering finishes before the first agenda fetch populates visible red-zone cards.
  - Added a stable Jira issue-key guard for red-zone agenda items, excluding internal `TASK-*`/`DEMAND-*` and lower-case Git branch slices from the decision cards.
  - Left the existing auto-decision telemetry feed untouched because the reported issue was the red-zone card grid.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/src/components/DecisionDashboard.svelte`
- Validation:
  - `git diff --check` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector/chunk-size warnings.
  - Running Vite preview received HMR for `DecisionDashboard.svelte`.

### Phase 28: Schedule And Evidence UI Refinement
- **Status:** complete
- **Started:** 2026-07-04
- Actions taken:
  - Loaded the global `ui-skill` before changing the touched UI surfaces.
  - Reworked the red-zone diagnostic card header so story/bug type and Jira number sit on one line.
  - Locked red-zone card rows to a stable equal-height grid so upper and lower cards render consistently.
  - Rebuilt the schedule risk calendar as an operational control panel with a primary focus line, refresh action, risk filter chips, clearer bucket hierarchy, and denser event rows.
  - Moved the demand deconstruction engine from the evidence observatory into the schedule governance page, keeping the evidence observatory focused on health telemetry and task/evidence collaboration.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `web/src/App.svelte`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/DemandKanban.svelte`
- Validation:
  - `git diff --check` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector/chunk-size warnings.

### Phase 27: Remove Delivery Cockpit And Continue Phase APIs
- **Status:** complete
- **Started:** 2026-07-04
- Actions taken:
  - Accepted the product judgment that the visible strongest-brain delivery cockpit still does not prove its value.
  - Loaded the global `ui-skill` before touching UI code.
  - Removed `StrongestBrainDeliveryCockpit` from the decision dashboard and deleted the frontend component.
  - Preserved backend strongest-brain read models so later phase-specific pages can reuse evidence, exception, weekly decision, AI trace, override, and authz data.
  - Added `GET /api/strongest-brain/exceptions` as the Phase 2 exception-center API with mode, summary, evidence, close requirements, owners, deadlines, severity, and evidence completeness.
  - Added `GET /api/strongest-brain/weekly-decisions` as the Phase 3 decision-meeting API with mode, summary, decision cards, options, owners, evidence refs, and decision debt.
  - Refactored strongest-brain decision snapshot construction so decision queue, exception center, and weekly decision center share one read model.
- Files created/modified:
  - `.agents/domains/coding.yaml`
  - `.agents/memory/project.jsonl`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/server.go`
  - `internal/server/strongest_brain_handlers.go`
  - `internal/server/strongest_brain_handlers_test.go`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/StrongestBrainDeliveryCockpit.svelte` deleted
- Validation:
  - `git diff --check` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestStrongestBrain(ExceptionAndWeeklyDecisionCenters|DecisionQueue|EvidenceChain|DeliveryCockpit)' -count=1` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector/chunk-size warnings.

### Phase 26: Strongest Brain Delivery Transformation Rollout
- **Status:** complete
- **Started:** 2026-07-04
- Actions taken:
  - Committed the existing workspace first as baseline `adcc981 chore: capture strongest brain baseline`.
  - Loaded complex-task planning/execution guidance and scoped frontend taste guidance to a dense internal delivery cockpit.
  - Spawned three subagents for backend evidence/exception/weekly-decision work, AI trace/readiness/context-pack work, and frontend cockpit integration.
  - Added a delivery cockpit read model over schedule, execution, decision, AI trace, override, and authorization evidence.
  - Extended strongest-brain evidence chain and decision queue read models with missing links, chain status, evidence completeness, exception summary, and weekly decision cards.
  - Added AI output trace, requirement clarification, and context-pack replay read models over existing deconstruction archive and context pack data.
  - Mounted a compact `StrongestBrainDeliveryCockpit` above the decision dashboard without restoring the rejected old decision-queue surface.
  - Registered protected APIs for delivery cockpit, demand readiness, override audit, AI traces, output trace, clarification, and context-pack replay.
  - Kept generated runtime changes in `task_status.md` and `well-ambient.db` out of the implementation commit.
- Files created/modified:
  - `.agents/state.json`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/ai_handlers.go`
  - `internal/server/ai_trace_helpers.go`
  - `internal/server/ai_trace_helpers_test.go`
  - `internal/server/server.go`
  - `internal/server/strongest_brain_delivery_handlers.go`
  - `internal/server/strongest_brain_handlers.go`
  - `internal/server/strongest_brain_handlers_test.go`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/StrongestBrainDeliveryCockpit.svelte`
- Validation:
  - `git diff --check` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(StrongestBrainDeliveryCockpit|StrongestBrainDecisionQueue|StrongestBrainEvidenceChain|AIIntentSummary|AIOutputTraceReadModelReplaysArchiveContextPack|RequirementClarificationReadModelClassifiesQuestions|ImportTasksAutoArchivesContextPackAndTrace|ImportTasksArchivesContextPackID|ParseDeconstructResponseContent)' -count=1` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/... -count=1` passed after approved non-sandbox rerun because sandboxed `httptest` loopback binding is blocked.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector/chunk-size warnings.

## Session: 2026-07-03

### Phase 25: Remove Decision Queue Surface
- **Status:** complete
- **Started:** 2026-07-03
- Actions taken:
  - Treated the latest feedback as a removal request rather than another UI explanation pass.
  - Removed the visible strongest-brain decision queue section from the decision dashboard.
  - Removed frontend `/api/strongest-brain/decision-queue` fetching and the 15-second queue polling path.
  - Removed queue-only interfaces, state, derived helpers, handlers, CSS, and responsive selector remnants.
  - Preserved the existing actionable Agenda metrics, filters, task selection, AI plan import, and manual intervention workflow.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
  - `web/src/components/DecisionDashboard.svelte`
- Validation:
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector and chunk-size warnings.

### Phase 24: Decision Queue Meaning
- **Status:** complete
- **Started:** 2026-07-03
- Actions taken:
  - Treated the user feedback as a cockpit usefulness gap in the Phase 22 strongest-brain decision queue.
  - Loaded AGENTS cold-start context, planning/execution guidance, and the required frontend taste guidance.
  - Inspected `DecisionDashboard.svelte` and the strongest-brain decision queue API projection.
  - Preserved the backend API contract and carried backend item `source` through frontend normalization.
  - Added a selected decision focus panel showing the current decision kind, idle cost, suggested action, handling entry, and source counts.
  - Added per-card queue source, decision type, no-action cost, and clearer handling entry labels.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
  - `web/src/components/DecisionDashboard.svelte`
- Validation:
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector and chunk-size warnings.

### Phase 23: Schedule Governance Risk Calendar
- **Status:** complete
- **Started:** 2026-07-03
- Actions taken:
  - Resumed after Phase 22 implementation commit `2e9ea85`.
  - Loaded AGENTS cold-start context, planning/execution skills, and continuation memory.
  - Selected master-plan Phase 4 as the next executable slice: schedule governance risk calendar.
  - Spawned backend worker Fermat for `/api/schedule/risk-calendar`.
  - Spawned frontend worker James for the demand schedule-view risk calendar surface.
  - Spawned QA explorer Archimedes for schedule/decision/notification contract risk review.
  - Confirmed current dirty `task_status.md` and `well-ambient.db` are outside the intended Phase 23 scope.
  - Closed the subagents after timeout/shutdown and integrated the partial frontend work that appeared in the shared worktree.
  - Added `/api/schedule/risk-calendar` as an additive read model over existing schedule risk rules.
  - Added week/month risk buckets, calendar events, and summary counts for overdue, due soon, stale, and missing schedule.
  - Added a compact risk calendar panel to the demand schedule view with fallback from current schedule rows if the API is unavailable.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/server.go`
  - `internal/server/schedule_risk_calendar_handlers.go`
  - `internal/server/schedule_risk_calendar_handlers_test.go`
  - `web/src/components/DemandKanban.svelte`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestGetScheduleRiskCalendar|TestGetScheduleBuildsDemandTimeline' -count=1` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector and chunk-size warnings.
  - `git diff --check` passed.

## Session: 2026-07-02

### Phase 22: Strongest Brain Master Plan Landing, Intent Recognition, And Summary
- **Status:** complete
- **Started:** 2026-07-02
- Actions taken:
  - Committed all existing workspace changes before new implementation: `2a7890c chore: checkpoint current workspace changes`.
  - Loaded required planning and execution skills.
  - Loaded `design-taste-frontend` and scoped it to a dense internal dark cockpit repair, not a marketing redesign.
  - Spawned backend worker Sagan for strongest-brain read models, decision queue, evidence chain, and AI intent/summary API.
  - Spawned frontend worker Hilbert for visible strongest-brain cockpit and AI intent/summary interaction.
  - Spawned QA explorer Averroes for integration risk and validation recommendations.
  - Backend worker loaded AGENTS cold-start files, selected the `coding.complex` preset, and recorded backend-only scope.
  - Backend worker confirmed no frontend edits should be made and parallel-agent changes must not be reverted.
  - Added Phase 3.5 to the master plan for AI intent recognition and summarization.
  - Added `/api/strongest-brain/decision-queue`, `/api/strongest-brain/evidence-chain`, `/api/ai/intent`, `/api/ai/intent-summary`, and `/api/ai/assistant/summary`.
  - Reused schedule and execution read models to build a v1 strongest-brain decision queue with evidence digests.
  - Added a deterministic AI intent/summary fallback so the conversation UI works when AI providers are unavailable.
  - Added the visible strongest-brain decision queue to the decision cockpit and a transparent AI intent/summary panel to the deconstructor.
  - Closed the QA subagent after integrating its findings.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `docs/strongest-brain-phased-master-plan.md`
  - `internal/server/server.go`
  - `internal/server/strongest_brain_handlers.go`
  - `internal/server/strongest_brain_handlers_test.go`
  - `web/src/App.svelte`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/Deconstructor.svelte`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestStrongestBrain|TestAIIntentSummary' -count=1` passed.
  - `pnpm build` in `web` passed with existing Svelte a11y/unused-selector and chunk-size warnings.
  - `git diff --check` passed.
  - Full `go test ./internal/server -count=1` remains environment-limited in the sandbox because existing `httptest.NewServer` tests cannot bind `[::1]:0`; escalation was unavailable due account usage limit.

## Session: 2026-06-23

### Phase 21: Board Cohesion, Audit Panel, And Modal Lock Polish
- **Status:** complete
- **Started:** 2026-06-23
- Actions taken:
  - Resumed after context compaction and reread the active planning files.
  - Read the required `design-taste-frontend` guidance and applied it as an internal-dashboard repair constraint.
  - Added the current six-part task addendum to `task_plan.md` and `findings.md`.
  - Added a non-secret Jira link-config endpoint so demand/task users can open Jira without `config:read`.
  - Allowed authenticated demand users to load reassignment candidates and protected recent manual assignee overrides from Jira sync rollback.
  - Compacted the schedule effort summary, made difficulty a dropdown, and reduced the config-version audit panel height with section-specific filtering.
  - Confirmed modal scroll locking covers shared Modal, Demand modals, and Settings modals.
  - Made demand cards keyboard/click accessible for details while preserving Jira/direct action click targets.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/config_handlers.go`
  - `internal/server/server.go`
  - `internal/server/jira_worker.go`
  - `internal/server/jira_worker_test.go`
  - `internal/server/server_test.go`
  - `internal/agenda/handlers.go`
  - `internal/agenda/handlers_test.go`
  - `internal/server/demand_handlers.go`
  - `web/src/lib/modalScrollLock.ts`
  - `web/src/components/shared/Modal.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/TaskKanban.svelte`
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/DecisionDashboard.svelte`

## Latest Validation After Phase 21
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Targeted reassignment/Jira metadata tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server ./internal/agenda -run 'TestShouldPreserveLocalAssignee|TestGetDemandOptions|TestGetJiraLinkConfig|TestPostAgendaDecisionReassignSyncsKanban' -count=1` | targeted backend behavior passes | passed | pass |
| Frontend production build | `pnpm build` in `web` | Vite build succeeds | passed with existing Svelte a11y warnings | pass |

## Latest Error Log
| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-06-23 | Svelte build failed because `{@const detailSubtasks...}` was not an immediate child of an allowed block | 1 | Replaced the inline const with a reactive `detailSubtasks` value before rerunning the build successfully |

## Session: 2026-06-20

### GitLab/Jira Integration Worker A: Webhook Ensure/Status MVP
- **Status:** complete
- Actions taken:
  - Loaded required AGENTS cold-start files and selected the complex coding preset.
  - Inspected `internal/config/config.go`, `internal/server/config_handlers.go`, `internal/server/server.go`, and `internal/telemetry/jira_client.go`.
  - Added protected GitLab webhook installation and inspection handlers.
  - Registered `POST /api/gitlab/webhooks/ensure` and `GET /api/gitlab/webhooks/status` behind `config:write`.
  - Implemented GitLab API list/create/update project hook calls with push and merge request events enabled.
  - Kept GitLab API token and webhook secret out of response bodies and returned status-only GitLab API errors.
  - Added httptest-backed regression coverage for update, create, status, URL defaulting, event flags, and response secret redaction.
- Files modified:
  - `internal/server/config_handlers.go`
  - `internal/server/server.go`
  - `internal/server/config_handlers_gitlab_webhook_test.go`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestEnsureGitLabWebhooks|TestGetGitLabWebhookStatus|TestResolveGitLabWebhookURL' -count=1 -v` passed after rerun with loopback bind permission.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -count=1` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/telemetry -count=1` passed.
- Notes:
  - A sandboxed first attempt at the httptest suite failed because the sandbox blocked local listener binding; the same tests passed with approved loopback access.
  - `internal/server/*` is ignored by `.gitignore`, so normal `git status` does not show those edited files.

### Phase 1: Requirements & Discovery
- **Status:** complete
- **Started:** 2026-06-20
- Actions taken:
  - Loaded required AGENTS cold-start files.
  - Loaded `executing-plans`, `planning-with-files`, and `frontend-design` skill guidance.
  - Read the external implementation plan.
  - Initial file search missed planned backend paths.
  - Located ignored backend server files with `find`.
  - Mapped existing frontend implementation against the plan.
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/notification_handlers.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/server_test.go`
  - `internal/telemetry/handler.go`
  - `internal/telemetry/handler_test.go`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/TaskKanban.svelte`
  - `web/dist/*` (generated by `pnpm build`)

### Phase 2: Planning & Structure
- **Status:** complete
- Actions taken:
  - Mapped the plan's backend items to `internal/server/notification_handlers.go` and `internal/server/ai_handlers.go`.
  - Mapped frontend items to `DemandKanban.svelte`, `DecisionDashboard.svelte`, `Deconstructor.svelte`, and related `TaskKanban.svelte` parent-demand display.

### Phase 3: Implementation
- **Status:** complete
- Actions taken:
  - Added top-level department candidate extraction and recursive redaction for WellOS login response logs.
  - Tightened department recursion so non-department `name` fields are not treated as departments.
  - Made `/api/tasks/import` preserve existing metadata, reject unknown `demand_id`, link demands, and sync imported shadow tasks to markdown.
  - Preserved `task_group_id`, due date, creator, and decision logs during GitLab webhook telemetry updates.
  - Filtered archived demand data from `DemandKanban`.
  - Let `DecisionDashboard` custom-select options wrap instead of clipping.
  - Removed invalid duplicate nested `{@const}` in `TaskKanban`.

### Phase 4: Testing & Verification
- **Status:** complete
- Actions taken:
  - Ran targeted Go tests.
  - Ran full Go tests.
  - Ran frontend production build.
  - Noted existing Svelte a11y warnings that do not block build.

### Phase 6: Follow-up Fixes
- **Status:** complete
- Actions taken:
  - Added a project rule requiring `taste-skill` for frontend UI changes, with `frontend-design` as fallback when unavailable.
  - Added department to JWT claims and auth headers.
  - Added `GET /api/me` so the frontend can refresh the current user's department/profile without admin permissions.
  - Expanded WellOS department extraction to handle organization/unit/Chinese keys and department IDs.
  - Updated the app shell to recover department, permissions, and role from JWT and `/api/me`.
  - Updated AI deconstruction import so a selected demand with an existing task group keeps that group for new shadow tasks.
  - Updated the Deconstructor demand dropdown to use semantic button elements for the touched UI.
  - Replaced demand schedule date inputs with a styled shell and custom calendar icon while keeping native date picking behavior.
  - Added focused tests for JWT department claim, `/api/me`, department extraction, and task-group reuse.
- Files created/modified:
  - `AGENTS.md`
  - `.agents/domains/coding.yaml`
  - `.learnings/ERRORS.md`
  - `internal/server/jwt.go`
  - `internal/server/server.go`
  - `internal/server/user_handlers.go`
  - `internal/server/notification_handlers.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/App.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/dist/*` (generated by `pnpm build`)

## Test Results
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Targeted Go tests | `go test ./internal/server ./internal/telemetry -v -count=1` | server and telemetry pass | passed | pass |
| Full Go tests | `go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend build | `pnpm build` in `web` | production build succeeds | passed with existing a11y warnings | pass |
| Full Go tests after follow-up | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend build after follow-up | `pnpm build` in `web` | production build succeeds | passed with remaining existing a11y warnings outside the Deconstructor dropdown touched area | pass |
| Phase 45 frontend build | `pnpm -C web build` | production build succeeds | passed; existing DemandKanban and Switch warnings remain | pass |
| Phase 45 TypeScript | `pnpm -C web exec tsc -p tsconfig.app.json --pretty false` | no type errors | passed | pass |
| Phase 45 browser route sweep | all 11 Settings routes | no dark drift, content overflow, or nested field surfaces | passed with isolated local fixture | pass |
| Phase 45 responsive workbench | 1024x768 and 760x900 | content remains visible and navigation usable | passed; drawer verified open and closed | pass |

### Phase 7: Modal, Department, and Date Follow-up
- **Status:** complete
- Actions taken:
  - Applied the requested `design-taste-frontend` skill direction as a constrained internal-dashboard UI repair.
  - Reworked the delete/archive confirmation dialog into a flatter dark system modal with explicit target metadata and action-specific color.
  - Removed the native visual surface from new-demand and schedule date fields by adding a custom display layer over the native date picker hit target.
  - Passed the current user's department from `App.svelte` into `DemandKanban.svelte` and sent it as `creator_dept` when creating a demand.
  - Strengthened backend creator department resolution to fall back from DB to JWT-auth header to request body.
  - Treated `未分配` and `无部门` as placeholders rather than valid departments, so a refreshed login department can overwrite stale placeholder data.
  - Added focused server tests for JWT department fallback, request-body fallback, user table backfill, and placeholder overwrite.
- Files modified:
  - `internal/server/demand_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/App.svelte`
  - `web/src/components/DemandKanban.svelte`

### Phase 8: Brain Binding, Profile, Repo, and Calendar Follow-up
- **Status:** complete
- Actions taken:
  - Applied `design-taste-frontend` as the active UI constraint for this frontend change set.
  - Moved the readonly personal profile out of the main tab navigation and into the avatar dropdown so users can inspect OS-returned name, avatar, role, permissions, and department at the trigger point.
  - Updated `/api/me` to backfill stale or missing user name, avatar, and department from authenticated JWT headers, including placeholder department values.
  - Passed authenticated avatar through middleware headers so the profile endpoint can repair missing avatar data.
  - Tightened WellOS department extraction so generic display-name fields are only accepted inside department context.
  - Linked demand scheduling and AI decomposition through a stable `brain-{demand_id}` task group when no explicit group exists.
  - Updated AI task imports so selected demands without an existing group reuse the same stable brain group across scheduling and deconstruction.
  - Removed the new-demand repo auto-default from the first GitLab repo; new product demands now submit an empty repo and cards show AI mapping state instead.
  - Replaced native date inputs in new-demand and schedule forms with a custom in-system calendar component.
  - Added focused regression tests for `/api/me` backfill, stable brain-group imports, and department parsing edge cases.
- Files created/modified:
  - `internal/server/server.go`
  - `internal/server/user_handlers.go`
  - `internal/server/notification_handlers.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/App.svelte`
  - `web/src/components/ProfilePanel.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/Deconstructor.svelte`

## Latest Validation Update
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Full Go tests after brain/profile/calendar follow-up | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend production build after brain/profile/calendar follow-up | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |

### Phase 9: WellOS Maintenance Login Degradation
- **Status:** complete
- Actions taken:
  - Added a guarded local fallback for WellOS login outages.
  - Allowed degraded login only for users that already exist locally, have a local user-group/permission snapshot, and pass local cached-password verification from a prior successful WellOS login.
  - Denied degraded login for unknown users or users without local access permissions.
  - Issued degraded JWT sessions with a `wellos-maintenance-fallback` marker instead of a real WellOS token.
  - Recorded degraded logins as `user_login_degraded` audit events.
  - Kept the hardened WellOS request path using HTTP/1.1, no keep-alive reuse, browser-like headers, and one retry for transient EOF/reset errors.
  - Added a frontend maintenance banner when the backend returns `degraded: true`.
  - Added regression tests for allowed degraded login, unknown-user denial, and wrong-password denial.
  - Added an explicit local-development auth switch for OS outages: `WELL_AMBIENT_DEV_AUTH=1`.
  - Restricted dev auth to loopback requests only, creates/updates a local developer user, binds `super_admin`, and marks the session with `local_dev_auth`.
- Files modified:
  - `internal/db/user/user.go`
  - `internal/server/notification_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/App.svelte`

## Latest Validation After Maintenance Fallback
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Degraded login server tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestLogin(FallsBack|DoesNotFallback)' -count=1 -v` | existing local user succeeds, unknown user fails | passed | pass |
| Full Go tests after degraded login | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend production build after degraded login banner | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Dev auth server tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestLogin(FallsBack|DoesNotFallback|UsesDevAuth)' -count=1 -v` | dev auth works only on loopback when enabled | passed | pass |

### Phase 10: System Extension Landing
- **Status:** complete
- Actions taken:
  - Converted the extension recommendations into three bounded implementation tracks.
  - Spawned worker A for GitLab/Jira integration and GitLab webhook ensure/status backend APIs.
  - Spawned worker B for KPI personal performance analysis and daily/weekly report preview.
  - Spawned worker C for AI demand deconstruction analysis fields and frontend display.
  - Applied `design-taste-frontend` as the active UI constraint for touched dashboard UI: internal dark cockpit, low motion, high density, no marketing-style sections.
  - Added GitLab config-panel actions for webhook status inspection and install/update after a successful config save.
  - Integrated worker changes and validated Go plus frontend build.
- Files modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `internal/server/config_handlers.go`
  - `internal/server/config_handlers_gitlab_webhook_test.go`
  - `internal/server/server.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/ai_handlers_test.go`
  - `internal/server/kpi_handlers.go`
  - `internal/server/kpi_handlers_test.go`
  - `web/src/components/config/GitLabConfig.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/KPIKanban.svelte`

## Latest Validation After Phase 10
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Targeted extension tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestEnsureGitLabWebhooks|TestGetGitLabWebhookStatus|TestResolveGitLabWebhookURL|TestParseDeconstructResponseContent|TestKPIPerformanceIncludesProcessRiskFields|TestKPIReportPreview' -count=1 -v` | GitLab webhook, AI analysis, and KPI report tests pass | passed with approved loopback bind for httptest | pass |
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed with approved loopback bind for httptest | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |

## Latest Validation
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Full Go tests after modal/department/date follow-up | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed | pass |
| Frontend production build after modal/department/date follow-up | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Frontend type check | `pnpm --dir web check` | no type errors | failed on pre-existing TS errors outside this change set | known issue |

### Phase 11: Dashboard UI Control Polish
- **Status:** complete
- Actions taken:
  - Applied `design-taste-frontend` as a constrained internal dashboard UI repair.
  - Replaced the Override assignee text input with a dark-system custom dropdown populated from the selected item, AI suggestion, core members, and agenda assignees.
  - Replaced the Override deadline native visual surface with a custom dark display shell backed by a transparent clickable native date input.
  - Renamed the red-zone diagnostic filter from code repository language to project filtering while preserving the existing `repo` API field.
  - Replaced the KPI report preview team-view native select with a custom dark dropdown and outside-click dismissal.
  - Added wrapping constraints for Jira custom JQL summary values so long JQL strings remain inside the summary card.
- Files modified:
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/KPIKanban.svelte`
  - `web/src/components/config/JiraConfig.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`

## Latest Validation After Dashboard UI Control Polish
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Frontend type check | `pnpm --dir web check` | no type errors | failed on pre-existing TS errors outside this change set | known issue |

### Phase 12: AI Deconstruction Estimation & Meeting Replacement
- **Status:** complete
- Actions taken:
  - Extended the AI deconstruction prompt and JSON contract with overall estimated days/hours, overall difficulty, estimate basis, and subtask estimated days/hours/difficulty.
  - Added deterministic normalization so missing task estimates are distributed from the overall estimate using difficulty weights.
  - Added `DeconstructArchive` to persist each imported deconstruction run with input text, mapped repos, task snapshot, analysis snapshot, overall estimate, difficulty, confidence, and task group.
  - Added task-level estimate telemetry fields: estimate days, estimate hours, difficulty, source, and archive ID.
  - Preserved estimate telemetry through GitLab/MR webhook state updates.
  - Updated the Deconstructor UI to show overall estimate/difficulty and editable subtask estimate/difficulty while keeping the existing dark cockpit style.
  - Sent input text, analysis, mapped repos, mock state, and task estimates during one-click import so archives are complete.
  - Added regression tests for estimate normalization and import archive persistence.
- Files modified:
  - `internal/db/db.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/ai_handlers_test.go`
  - `internal/server/server_test.go`
  - `internal/telemetry/handler.go`
  - `web/src/components/Deconstructor.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.learnings/ERRORS.md`

## Latest Validation After Phase 12
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Estimate/import focused tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestParseDeconstructResponseContent|TestImportTasks(LinksDemand|ReusesLinkedDemandTaskGroup|CreatesStableBrainGroupForDemand|ArchivesEstimates|RejectsUnknownDemand)' -count=1 -v` | estimate parsing and archive import pass | passed | pass |
| Telemetry preservation tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/telemetry -run TestProcessWebhook -count=1 -v` | webhook tests pass | passed | pass |
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed with approved loopback access for httptest | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Frontend type check | `pnpm --dir web check` | no type errors | failed on pre-existing TS errors outside this change set | known issue |

### Phase 13: AI Project Context & Hour-Based Deconstruction
- **Status:** complete
- Actions taken:
  - Added AI settings fields for project architecture, delivery workflow, implemented capabilities, estimation guidelines, and default work-hours-per-day.
  - Injected those context fields into the deconstruction prompt, with a fallback well-ambient system summary when fields are empty.
  - Made `estimated_hours` and `overall_estimated_hours` the primary prompt/UI estimate fields while preserving `estimated_days` as derived scheduling/archive compatibility data.
  - Added configured work-hours normalization so non-8-hour day settings remain consistent in parsed AI output.
  - Moved the AI analysis panel to the left input column directly under the deconstruction button, keeping the right panel focused on project mapping and shadow task cards.
  - Updated Deconstructor subtask effort controls from day options to hour options.
  - Removed the local `Switch helperText` type error from `AIConfig.svelte`.
- Files modified:
  - `internal/config/config.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/ai_handlers_test.go`
  - `web/src/components/config/AIConfig.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/IntegrationPanel.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`

## Latest Validation After Phase 13
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Focused AI deconstruction tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestParseDeconstructResponseContent|TestImportTasksArchivesEstimates' -count=1 -v` | parse/import estimate tests pass | passed | pass |
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | passed with approved loopback access for httptest | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Frontend type check | `pnpm --dir web check` | no type errors | failed on 8 pre-existing TS errors outside this change set; no `AIConfig` type error remains | known issue |

### Phase 14: Path Planning Module Prompt Context
- **Status:** complete
- Actions taken:
  - Read the uploaded workbook `全局领航能力汇总.xlsx`, first sheet `功能列表`.
  - Extracted 39 path-planning module capabilities with category, status, configurability, site/project coverage, and validation notes.
  - Created `docs/ai-context/path-planning-module.md` as the archived source-of-truth summary for AI use.
  - Populated `config.example.yaml` AI context fields with a compact prompt-ready summary that references the archive document.
  - Preserved module scope boundaries: this catalog is path-planning business context, not the full well-ambient collaboration architecture.
  - Validated config parsing and focused AI deconstruction tests.
- Files modified:
  - `config.example.yaml`
  - `docs/ai-context/path-planning-module.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
  - `.learnings/ERRORS.md`

## Latest Validation After Phase 14
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Config package tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/config -run Test -count=1 -v` | config tests pass and YAML remains load-compatible | passed | pass |
| Focused AI deconstruction tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestParseDeconstructResponseContent|TestImportTasksArchivesEstimates' -count=1 -v` | parse/import estimate tests pass | passed | pass |

### Phase 15: Remove Mis-scoped AI Context Center
- **Status:** complete
- Actions taken:
  - Removed the `AIContextCenter` import/render from AI settings while keeping the generic project-context and estimation fields.
  - Deleted the AI context profile upload/list/toggle backend, tests, and database model.
  - Removed `/api/ai-context/*` route registration and stopped appending enabled module profiles to the deconstruction prompt.
  - Cleared path-planning-specific context values from `config.example.yaml`.
  - Deleted the path-planning AI context archive document because it represented the mis-scoped targeted module direction.
  - Updated task memory to mark the module-image direction as superseded by a general system-context configuration model.
- Files modified:
  - `config.example.yaml`
  - `internal/db/db.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/server.go`
  - `web/src/components/config/AIConfig.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Files deleted:
  - `docs/ai-context/path-planning-module.md`
  - `internal/server/ai_context_handlers.go`
  - `internal/server/ai_context_handlers_test.go`
  - `web/src/components/config/AIContextCenter.svelte`

## Latest Validation After Phase 15
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1` | all Go packages pass | sandbox loopback bind failed, then passed with approved loopback access | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings | pass |

### Phase 16: Strongest Brain Context Registry And Policy RBAC
- **Status:** complete
- Actions taken:
  - Wrote the implementation plan to `docs/strongest-brain-implementation-plan.md`.
  - Added AI Context Registry data models for documents, facts, chunks, packs, and pack items.
  - Added context fact list/create/update APIs plus deterministic context-pack preview and persistence.
  - Rewired AI deconstruction to use a persisted context pack before prompt assembly and return `context_pack_id`.
  - Linked imported deconstruction archives to `context_pack_id`, with import-time pack creation fallback.
  - Added policy authorization models, fine-grained permission seeds, deny/allow evaluator, decision audit logging, and `withPermission` compatibility.
  - Added policy list/save, authorization explain, and authorization audit APIs.
  - Reworked AI settings UI so structured facts replace the old visible free-text context center; legacy fields remain backend fallback only.
  - Added strategy/RBAC UI for policy creation, decision explanation, policy list, and audit logs.
- Files modified:
  - `docs/strongest-brain-implementation-plan.md`
  - `internal/db/db.go`
  - `internal/db/user/init.go`
  - `internal/db/user/user.go`
  - `internal/server/ai_handlers.go`
  - `internal/server/authz/policy.go`
  - `internal/server/authz_handlers.go`
  - `internal/server/context_handlers.go`
  - `internal/server/context_handlers_test.go`
  - `internal/server/policy_authorization_test.go`
  - `internal/server/server.go`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/config/AIConfig.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`

## Latest Validation After Phase 16
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Focused context/auth tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(Context|Authorization|WithPermission|Authorize|ImportTasksArchives|ParseDeconstruct)'` | new context, archive, and auth paths pass | passed | pass |
| Full Go regression | `GOCACHE=/tmp/well-ambient-gocache go test ./...` | all Go packages pass | sandbox loopback bind failed, then passed with approved loopback access | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte a11y warnings in untouched DemandKanban/TaskKanban/shared controls | pass |

### Phase 17: AI Engine Tab, Permission Tree, and Policy Workbench Polish
- **Status:** complete
- Actions taken:
  - Applied `design-taste-frontend`/taste-skill guidance as a restrained internal admin-console refactor.
  - Split the AI settings surface into an independent `AI 引擎配置` tab and a separate `上下文事实` tab backed by the same `AIConfig` component view modes.
  - Replaced native-looking AI context fact selects with button-backed choice grids and styled the number/range/text controls to match the dark product system.
  - Added `GET /api/permissions` so the frontend can use the live seeded permission catalog rather than a stale hard-coded matrix.
  - Rebuilt the visual permission matrix as a namespace-derived permission tree with per-group coverage toggles and responsive vertical layout.
  - Reworked policy authorization from raw selects/checkbox/number inputs into quick-start templates, segmented subject/scope controls, permission action chips, a state toggle, and a priority stepper.
  - Added `Button.size` support because existing config/settings call sites already used `size="small"`.
  - Added focused backend coverage for the permission catalog handler.
- Files modified:
  - `internal/server/server.go`
  - `internal/server/user_handlers.go`
  - `internal/server/policy_authorization_test.go`
  - `web/src/components/SettingsPanel.svelte`
  - `web/src/components/config/AIConfig.svelte`
  - `web/src/components/shared/Button.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
- Validation:
  - `pnpm build` in `web` passed with existing Svelte accessibility warnings in unrelated/touched-old areas.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run TestHandleListPermissionsReturnsSeededCatalog` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server/authz ./internal/agenda ./internal/config ./internal/kanban ./internal/telemetry` passed.
  - Full `go test ./...` and `go test ./internal/server` remain blocked by sandbox loopback restrictions from existing `httptest.NewServer` tests.
  - `pnpm check` still fails on pre-existing `TaskKanban.svelte` and `App.svelte` TypeScript errors; touched Settings/AI/Button files no longer add errors.

## Latest Validation After Phase 17
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Frontend production build | `pnpm build` in `web` | production build succeeds | passed with existing Svelte a11y warnings | pass |
| Permission catalog handler test | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run TestHandleListPermissionsReturnsSeededCatalog` | `/api/permissions` handler returns seeded permissions | passed | pass |
| Non-listener package tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server/authz ./internal/agenda ./internal/config ./internal/kanban ./internal/telemetry` | packages pass without local listener use | passed | pass |
| Full server tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server` | server tests pass | blocked by sandbox loopback bind in existing `httptest.NewServer` tests | blocked |
| Frontend type check | `pnpm check` in `web` | type check passes | failed on pre-existing `TaskKanban.svelte` and `App.svelte` errors outside touched Settings/AI/Button files | blocked |

### Phase 18: Demand Creation Sources and Jira Task Demand Model
- **Status:** complete
- Actions taken:
  - Applied `design-taste-frontend` guidance as an internal dense product UI repair.
  - Added the `/api/demands/options` form metadata endpoint so demand creation can load assignees and projects from users, Jira config/JQL, historical tasks, and configured repositories.
  - Added a project selector to the new-demand modal and kept “暂不指定项目” as the safe default for oral requirements.
  - Replaced div-backed dropdown options with button-backed controls and removed blur/scale animation from the modal backdrop to reduce typing-time visual jitter.
  - Changed Jira issue type mapping so Jira Task/Story/Feature/Epic-like items enter `issue_type = demand`; Bug/Defect-like items remain `bug`.
  - Added `docs/demand-bug-lifecycle-model.md` and updated execution/schedule docs to match the new demand/bug/execution split.
- Files modified:
  - `internal/server/demand_handlers.go`
  - `internal/server/jira_worker.go`
  - `internal/server/jira_worker_test.go`
  - `internal/server/server.go`
  - `internal/server/server_test.go`
  - `web/src/components/DemandKanban.svelte`
  - `docs/demand-bug-lifecycle-model.md`
  - `docs/execution-task-observability-mvp.md`
  - `docs/schedule-table-mvp.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`

## Latest Validation After Phase 18
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Demand/Jira focused Go tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(MapJiraStatus|MapJiraIssueType|BuildJQL|ParseJiraTime|GetDemandOptionsBuildsFormCandidates|CreateDemandUsesAuthenticatedDepartmentFallback|CreateDemandUsesRequestDepartmentFallback)' -count=1 -v` | Jira mapping, demand options, and create-demand fallbacks pass | passed | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing Svelte accessibility warnings | pass |
| Frontend type check | `pnpm --dir web check` | type check passes | passed with existing warnings, 0 errors | pass |

### Phase 19: WellOS User Info Profile Refresh
- **Status:** complete
- Actions taken:
  - Added `GET https://wellos.westwell-lab.com/api/user/info?token={token}` support after normal WellOS login succeeds.
  - Used returned `realname`, `avatar`, `email`, and `department_name` to refresh local user records.
  - Generated JWT claims and login response payloads from the refreshed avatar and department values.
  - Left maintenance fallback and local development auth on local cached profile data only.
  - Added a focused regression test for login profile refresh through the new WellOS user-info response.
- Files modified:
  - `internal/server/notification_handlers.go`
  - `internal/server/server_test.go`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`

## Latest Validation After Phase 19
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| WellOS profile refresh tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestLoginRefreshesAvatarAndDepartmentFromWellOSUserInfo|TestGetCurrentUser(ReturnsDepartment|BackfillsDepartmentFromJWT|RefreshesDepartmentFromJWT)|TestGenerateJWTIncludesDepartment' -count=1 -v` | login response, DB, and JWT use user-info avatar and department | passed | pass |

### Phase 20: Demand Schedule Table and Effort Estimate Polish
- **Status:** complete
- Actions taken:
  - Added row-level `scheduled` to the schedule DTO so UI truth is not inferred from workflow status.
  - Extended schedule saving to persist optional AI estimate hours, days, and difficulty.
  - Split the schedule table into schedule, effort, delivery evidence, risk, and update time columns.
  - Removed branch/repo placeholder text from the schedule column and added styled internal scrollbars.
  - Reworked the scheduling modal into an opaque dark-system surface with a layout-stable custom calendar and optional AI effort panel.
  - Updated `docs/schedule-table-mvp.md` with the schedule truth and AI estimate contract.
- Files modified:
  - `internal/server/demand_handlers.go`
  - `internal/server/schedule_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/components/DemandKanban.svelte`
  - `docs/schedule-table-mvp.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`

## Latest Validation After Phase 20
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Schedule focused Go tests | `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestGetScheduleBuildsDemandTimeline|TestDemandAndKPILogic' -count=1 -v` | schedule DTO and schedule save estimate fields pass | passed | pass |
| Frontend type check | `pnpm --dir web check` | Svelte/TS has 0 errors | passed with existing warnings | pass |
| Frontend production build | `pnpm --dir web build` | production build succeeds | passed with existing warnings | pass |
| Diff hygiene | `git diff --check` | no whitespace errors | passed | pass |

## Error Log
| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-07-04 | Worker B targeted `go test ./internal/server` could not compile because concurrent strongest-brain symbols were undefined: `enrichStrongestBrainDecisionItems`, `dedupeStrongestBrainDecisionItems`, `buildStrongestBrainWeeklyDecisions`, `buildStrongestBrainDecisionSummary`, `buildStrongestBrainExceptionSummary` | 1 | Left unrelated strongest-brain files untouched; recorded validation blocker and kept Worker B changes scoped to AI/context handlers and tests. |
| 2026-06-20 | `rg --files` did not surface `internal/server` | 1 | Switched to `find` and targeted file reads. |
| 2026-06-20 | Go tests could not write default Go cache in sandbox | 1 | Re-ran with approved elevated `go test`. |
| 2026-06-20 | `pnpm build` failed on invalid nested `{@const}` in `TaskKanban.svelte` | 1 | Removed duplicate nested const from Done lane. |
| 2026-06-20 | `apply_patch` context did not match after `gofmt` | 1 | Re-read formatted snippets, patched with smaller context, and logged to `.learnings/ERRORS.md`. |
| 2026-06-20 | `apply_patch` context did not match while updating `task_plan.md` | 1 | Re-read the exact current section and applied a smaller-context patch. |

### Phase 26: Worker B AI Trace And Requirement Clarification Backend Slice
- **Status:** complete
- Actions taken:
  - Added AI output trace, Context Pack replay, and requirement clarification read-model helpers/handlers without registering new routes in `server.go`.
  - Extended `/api/deconstruct` response with a `trace` object when context pack replay succeeds.
  - Made `/api/tasks/import` auto-build an `import_archive` context pack from imported task snapshots when no explicit `context_pack_id`/`input_text` is supplied.
  - Made deconstruction archive creation require a non-zero `context_pack_id` and return `archive_id`, `context_pack_id`, `imported_count`, and `trace` from import.
  - Added focused tests for trace replay, missing-question tiering/readiness, and import auto-archive trace output.
- Validation:
  - `git diff --check` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(AIOutputTraceReadModelReplaysArchiveContextPack|RequirementClarificationReadModelClassifiesQuestions|ImportTasksAutoArchivesContextPackAndTrace|ImportTasksArchivesContextPackID|ParseDeconstructResponseContent)' -count=1 -v` passed.

## 5-Question Reboot Check
| Question | Answer |
|----------|--------|
| Where am I? | Complete |
| Where am I going? | Deliver concise final summary |
| What's the goal? | Temporarily hide the project health telemetry panel because its practical operator value is still unclear. |
| What have I learned? | A dashboard module should stay visible only when it drives a concrete operator action; telemetry without a proven workflow should not consume the evidence observatory. |
| What have I done? | Removed the `ProjectHealthTelemetry` mount from `App.svelte`, recorded Phase 31 in task memory, and confirmed production build plus diff hygiene. |

### Phase 32: KPI R&D Performance Dashboard Refactor
- **Status:** complete
- Actions taken:
  - Applied `finesse-skill` product UI direction to rebuild the KPI dashboard as a dense evaluation cockpit rather than a loose ranking board.
  - Promoted daily/weekly report granularity to the top-level control and synchronized it with the backend `day`/`week` period values.
  - Added a report brief panel that shows current report window, generation time, delivery totals, task/demand/Bug split, active overdue count, and MR count.
  - Added a team average score panel and an explainable score model: output contribution, code evidence, flow efficiency, and risk health.
  - Replaced the old member ranking cards with a member matrix showing KPI score, task/demand/Bug counts, MR/review/cycle evidence, active/overdue risk, and score breakdown.
  - Reworked the department view into compact department mix cards with proportional delivery bars and task/demand/Bug distribution.
  - Removed stale KPI leaderboard CSS and added responsive styles for the new dashboard structure.
- Files modified:
  - `web/src/components/KPIKanban.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - `.agents/state.json` parsed successfully.
  - `pnpm check` remains blocked by 3 non-KPI errors: `TaskKanban.svelte` references missing `TaskResponse.id`, and `ProjectConfig.svelte` assigns numbers where strings are expected.
  - Existing repository-wide Svelte a11y and unused CSS warnings remain outside the KPI component.

### Phase 33: KPI Core Member Data Boundary
- **Status:** complete
- Actions taken:
  - Added a server-side core member filter to the KPI endpoints so non-core member data is removed before KPI aggregation and report preview generation.
  - Derived core members from `jira.sync_users`; when absent, fell back to assignees parsed from custom JQL `assignee in (...)`.
  - Kept the filter disabled when no core member source is configured to avoid clearing KPI data in unconfigured test/local deployments.
  - Matched core members by display name, username, email prefix, and first token so Jira usernames and local profile names stay aligned.
  - Ensured KPI summary, user rows, department rows, report overview, personal sections, risks, meeting focus, and evidence are all based on the filtered task/user set.
  - Added a regression test proving a non-core member task is excluded from KPI performance and report preview evidence.
- Files modified:
  - `internal/server/kpi_handlers.go`
  - `internal/server/kpi_handlers_test.go`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestKPI(PerformanceIncludesProcessRiskFields|ReportPreviewReturnsEvidenceBackedSections|FiltersNonCoreMemberData|ReportPreviewRejectsUnknownType)' -count=1 -v` passed.

### Phase 34: KPI Interaction Stability And Analysis Depth
- **Status:** complete
- Actions taken:
  - Applied `finesse-skill` product UI guidance for the KPI follow-up as a dense internal performance dashboard, not a decorative redesign.
  - Fixed click/refresh jumping by preserving the already-loaded KPI dashboard while the daily/weekly mode refreshes data.
  - Added an inline sync indicator for background refreshes so the page does not swap to a full loading state after first load.
  - Prevented repeated daily/weekly clicks while a refresh is in flight and removed active-state scale transforms from KPI controls.
  - Followed up on the remaining daily/weekly layout jump by making the sync indicator a fixed reserved slot, locking dynamic copy height, and giving the report/member matrix panels stable minimum heights.
  - Capped the member matrix as an internal scroll area with a sticky header so weekly rows do not stretch the whole page and daily rows do not collapse it.
  - Added a performance depth diagnosis panel for delivery concentration, evidence coverage, risk load, review pressure, Bug share, and score spread.
  - Added a breadth distribution panel for task/demand/Bug mix, score bands, department coverage, and average cycle time.
- Files modified:
  - `web/src/components/KPIKanban.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Existing repository-wide Svelte warnings remain outside the KPI interaction/depth pass.

### Phase 35: Jira Execution Tracking Fact Alignment
- **Status:** complete
- Actions taken:
  - Traced the "Jira Task 开发结果追踪" view from `TaskKanban.svelte` through `/api/execution/tasks` to `execution_handlers.go`.
  - Confirmed bound execution rows were displaying the child execution task's assignee/status, even when the parent Jira Task demand had already been completed or reassigned in Jira.
  - Added explicit `execution_assignee`, `jira_assignee`, and `jira_status` fields to the execution tracking DTO.
  - Made the bound Jira Task demand drive the effective `assignee`, `status`, result state, risk state, last update, and assignee/search filters.
  - Kept the child execution assignee visible as secondary table evidence so historical execution ownership is not lost.
  - Updated Jira sync so a completed Jira status terminates the 24-hour local manual assignee protection window.
  - Updated Jira keep-alive correction sync to continue probing recently completed Jira Task/Bug keys for 14 days, allowing post-completion owner changes and reopen facts to refresh even when the main custom JQL excludes `done`.
  - Added a regression case where the parent Jira Task is `done` and reassigned to Bob while the child execution task still says Alice/progress.
- Files modified:
  - `internal/server/execution_handlers.go`
  - `internal/server/jira_worker.go`
  - `internal/server/server_test.go`
  - `internal/server/jira_worker_test.go`
  - `web/src/components/TaskKanban.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestShouldPreserveLocalAssignee|TestGetExecutionTasksBuildsEvidenceObservability' -count=1 -v` passed.
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestShouldPreserveLocalAssignee|TestShouldIncludeInJiraKeepAlive' -count=1 -v` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Existing repository-wide Svelte warnings remain outside the execution tracking fact-alignment fix.

### Phase 36: Core Member Visibility Boundary
- **Status:** complete
- Actions taken:
  - Added a shared coreMember visibility helper that reuses the configured KPI member source: `jira.sync_users` first, custom JQL assignees as fallback, disabled when no member source is configured.
  - Applied the visibility boundary to `/api/tasks`, `/api/schedule`, `/api/execution/tasks`, and `/api/demands/options`.
  - Made `/api/schedule` filter both demand rows and subtask stats so non-coreMember-owned subtasks do not affect the visible schedule projection.
  - Made `/api/execution/tasks` filter by effective current owner and redact non-core `execution_assignee` / `jira_assignee` secondary fields before encoding.
  - Added a KPI report evidence guard so report evidence remains tied to included coreMember users.
  - Removed the visible `外部协同` assignee entries and shortcut branch from `TaskKanban.svelte` and `DemandKanban.svelte`.
- Files modified:
  - `internal/server/core_member_visibility.go`
  - `internal/server/server.go`
  - `internal/server/schedule_handlers.go`
  - `internal/server/execution_handlers.go`
  - `internal/server/demand_handlers.go`
  - `internal/server/kpi_handlers.go`
  - `internal/server/server_test.go`
  - `web/src/components/TaskKanban.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'TestGetTasksFiltersNonCoreMemberData|TestGetScheduleFiltersNonCoreMemberData|TestGetExecutionTasksFiltersNonCoreMemberData|TestKPIFiltersNonCoreMemberData|TestGetExecutionTasksBuildsEvidenceObservability' -count=1 -v` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Existing repository-wide Svelte warnings remain outside this data-boundary fix.

### Phase 38: Finesse Componentized Admin Prototype Refactor
- **Status:** complete
- Actions taken:
  - Re-read and applied `finesse-skill` product UI guidance after the first prototype was rejected as too static and not aligned with the requested management-console feel.
  - Reworked `modern-admin-tokens.css` from a more decorative deep-blue glass layer into a flatter charcoal/acrylic token substrate with semantic status colors, stable control sizing, and quieter surfaces.
  - Split the prototype into local reusable pieces: `PrototypeShell`, `PrototypeMetric`, `PrototypeBadge`, `PrototypeTable`, and `PrototypeInspector`.
  - Replaced the single-file concept-board prototype with a dense operational UI: left lane navigation, command header, search/filter/density toolbar, metric strip, exception table, weekly decision queue, selected workflow path, and right-side mediation preflight drawer.
  - Kept the preview behind the existing `config:read` gated `UI 原型` tab instead of replacing current production pages.
  - Attempted browser verification at `http://127.0.0.1:5174/`; the app is reachable, but the in-app browser remained on the login page and browser policy blocked local script navigation for setting a preview auth state.
- Files modified:
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/ModernAdminPrototype.svelte`
  - `web/src/components/prototype/PrototypeShell.svelte`
  - `web/src/components/prototype/PrototypeMetric.svelte`
  - `web/src/components/prototype/PrototypeBadge.svelte`
  - `web/src/components/prototype/PrototypeTable.svelte`
  - `web/src/components/prototype/PrototypeInspector.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - `pnpm check` still fails on 3 pre-existing non-prototype TypeScript errors: `TaskKanban.svelte` references missing `TaskResponse.id`, and `ProjectConfig.svelte` binds numeric values where string values are expected.
  - Existing repository-wide Svelte a11y and unused CSS warnings remain outside the prototype refactor.

### Phase 39: Bold Beauty Admin Prototype Pass
- **Status:** complete
- Actions taken:
  - Treated the feedback "美即正义" as a `finesse-skill` bolder iteration while keeping the scope inside the preview prototype.
  - Pushed the visual direction to an editorial control-room aesthetic: deeper substrate, stronger cyan focus, richer grain, acrylic edges, ambient grid, and more decisive contrast.
  - Added `PrototypeTheatre.svelte` as the new first-screen visual center, combining selected issue focus, risk badge, owner/due/wait/stage facts, signal counts, and direct mediation actions.
  - Reworked `PrototypeShell.svelte` so the rail and app stage feel intentional rather than merely functional.
  - Refined `PrototypeMetric`, `PrototypeBadge`, `PrototypeTable`, and `PrototypeInspector` with stronger hierarchy, richer borders, more elegant status glow, sharper selected-row feedback, and a more instrument-like preflight drawer.
  - Left production pages, auth behavior, backend routes, and real data flows untouched.
- Files modified:
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/ModernAdminPrototype.svelte`
  - `web/src/components/prototype/PrototypeTheatre.svelte`
  - `web/src/components/prototype/PrototypeShell.svelte`
  - `web/src/components/prototype/PrototypeMetric.svelte`
  - `web/src/components/prototype/PrototypeBadge.svelte`
  - `web/src/components/prototype/PrototypeTable.svelte`
  - `web/src/components/prototype/PrototypeInspector.svelte`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Filtered `pnpm check` still reports only the 3 known non-prototype TypeScript errors in `TaskKanban.svelte` and `ProjectConfig.svelte`.
  - Browser visual verification still requires logging in to the existing app and opening `UI 原型`.

### Phase 41: Prototype-Aligned Multi-Agent Refactor Calibration
- **Status:** complete
- Actions taken:
  - Paused further page implementation after user feedback that the current style drifted away from the original prototype and data-chain intent.
  - Re-read AGENTS cold-start files, active state, memory hits, and frontend design skills.
  - Reconfirmed the primary visual baseline as `output/functional-admin-console-reference.png`: `well-ambient` brand, dark left rail, light main workspace, table-first middle surface, right inspector, restrained frosted cards.
  - Marked `output/functional-admin-console-reference-alt.png` and `output/modern-admin-reference-phase40.png` as layout references only because they still show `最强大脑`.
  - Spawned a common worker agent to own shared Phase 41 design/data contract work before page agents edit production surfaces.
  - Integrated the common worker output: `DESIGN.md`, `docs/admin-ui-calibration.md`, `modern-admin-tokens.css`, `FunctionalAdminShell.svelte`, `FunctionalWorkspace.svelte`, and `web/src/lib/admin-console/contract.ts`.
  - Integrated Decision, Schedule, Task, Evidence, and KPI page workers with disjoint file ownership and Phase 41 metric/table/inspector vocabulary.
  - Preserved the existing production data chains: decision agenda/intervention APIs, task/schedule/execution APIs, evidence/deconstruction APIs, KPI performance/report preview APIs, and settings/config/RBAC/audit APIs.
  - Closed the stalled Settings worker and completed `SettingsPanel.svelte` in the main thread with a Phase 41 container shell, real adapter metrics, category table, and right inspector while leaving config child components functional.
  - Confirmed no `最强大脑`, `Strongest Brain`, or `StrongestBrain` strings in the migrated page targets.
  - Removed remaining old emoji markers from `SettingsPanel.svelte` page/control copy.
- Files modified in this slice:
  - `DESIGN.md`
  - `docs/admin-ui-calibration.md`
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/lib/admin-console/contract.ts`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/prototype/FunctionalWorkspace.svelte`
  - `web/src/components/DecisionDashboard.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/TaskKanban.svelte`
  - `web/src/components/ProjectHealthTelemetry.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/KPIKanban.svelte`
  - `web/src/components/SettingsPanel.svelte`
- Validation:
  - `pnpm exec tsc -p tsconfig.app.json --pretty false` in `web` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check -- web/src/components/SettingsPanel.svelte` passed.
  - Targeted migrated-page brand scan passed.
  - Existing repository-wide Svelte warnings remain, mainly in `DemandKanban.svelte`, shared modal/switch, and config child components; Settings no longer contributes new warnings.

### Phase 42: Settings IA Correction
- **Status:** complete
- Actions taken:
  - Converted the Settings left navigation from flat grouped labels into real collapsible submenus with `aria-expanded`.
  - Replaced the mixed right-side stack of hero, metrics, category table, module content, and inspector with a unified right content frame.
  - Added a breadcrumb bar above the Settings content area: `管理台配置 / group / current section`.
  - Kept the actual business content below the breadcrumb in one content shell so GitLab, Feishu, Jira, Projects, AI, KPI, RBAC, policy, and audit flows render consistently.
  - Preserved existing API/data flows and child config components; this was a container/IA correction only.
- Files modified:
  - `web/src/components/SettingsPanel.svelte`
  - `DESIGN.md`
  - `docs/admin-ui-calibration.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm exec tsc -p tsconfig.app.json --pretty false` in `web` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check -- web/src/components/SettingsPanel.svelte` passed.
  - Build output has no new `SettingsPanel.svelte` warnings; remaining warnings are existing repository-wide warnings outside this IA correction.

### Phase 43: Settings Rail Navigation Correction
- **Status:** complete
- Actions taken:
  - Moved Settings grouped submenus out of the Settings content frame and into the global `FunctionalAdminShell` left rail.
  - Added Shell-level Settings submenu permission filtering, active section highlight, and callback-driven section navigation.
  - Removed the internal `SettingsPanel.svelte` sidebar markup, sidebar state, resize observer, and sidebar-specific CSS.
  - Kept `SettingsPanel.svelte` as a single breadcrumb-plus-content frame so the right content area remains unified.
  - Preserved existing config, KPI, RBAC, authorization, and audit data flows; this was a navigation ownership correction.
- Files modified:
  - `web/src/App.svelte`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/SettingsPanel.svelte`
  - `DESIGN.md`
  - `docs/admin-ui-calibration.md`
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
  - `.agents/state.json`
- Validation:
  - `pnpm exec tsc -p tsconfig.app.json --pretty false` in `web` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Targeted `SettingsPanel.svelte` scan found no internal sidebar/menu selectors or grouped-navigation helpers.
  - Build output has no new `SettingsPanel.svelte` warnings; remaining warnings are existing repository-wide warnings outside this correction.

### Phase 44: Main Content Prototype Alignment
- **Status:** complete
- Actions taken:
  - Updated the local frontend UI rule so UI work defaults to `ui-skill`, then falls back to `finesse-skill` and `taste-skill`.
  - Flattened `FunctionalWorkspace.svelte` so production pages do not render the old module hero by default.
  - Tightened `FunctionalAdminShell.svelte` main content spacing, topbar sizing, search width, light background, and card shadow tokens toward `output/functional-admin-console-reference.png`.
  - Converted the migrated Demand, Task, KPI, Project Health, Deconstructor, and Settings page-level headers into compact workbench toolbars instead of explanatory hero blocks.
  - Preserved existing page data flows and child panels; this pass changes visual hierarchy only.
- Files modified:
  - `.agents/domains/coding.yaml`
  - `.agents/memory/project.jsonl`
  - `.agents/state.json`
  - `DESIGN.md`
  - `docs/admin-ui-calibration.md`
  - `web/src/styles/modern-admin-tokens.css`
  - `web/src/components/prototype/FunctionalAdminShell.svelte`
  - `web/src/components/prototype/FunctionalWorkspace.svelte`
  - `web/src/components/DemandKanban.svelte`
  - `web/src/components/TaskKanban.svelte`
  - `web/src/components/KPIKanban.svelte`
  - `web/src/components/ProjectHealthTelemetry.svelte`
  - `web/src/components/Deconstructor.svelte`
  - `web/src/components/SettingsPanel.svelte`
- Validation:
  - `pnpm exec tsc -p tsconfig.app.json --pretty false` in `web` passed.
  - `pnpm build` in `web` passed.
  - `git diff --check` passed.
  - Targeted `showContextBar` scan confirmed no production page enables the optional workspace context hero.
  - Build still reports existing repository-wide Svelte warnings in `DemandKanban.svelte`, shared modal/switch, and config child components.
