# Progress Log

## Session: 2026-07-20 - Task Project/Owner Multi-Select And Surface Rhythm

- **Status:** complete.
- Completed and recorded the mandatory Impeccable, design-taste-frontend, and finesse-ui review before frontend edits. The shared direction keeps one bounded toolbar row, server-correct multi-selection, the existing table/inspector ownership, and scoped transparent secondary surfaces.
- Extended `/api/tasks` and `/api/execution/tasks` to accept repeated or comma-separated Project/Owner values. Values are ORed within each dimension and intersected across dimensions after saved personal project scope and existing core-member visibility.
- Extended the shared `MultiSelect` with an optional compact summary/overlay mode, clear-to-all action, visible Project/Owner ownership labels, keyboard behavior, and touch-safe narrow-screen controls without changing the project-preference default presentation.
- Replaced status, personnel, and execution Project/Owner filters with local multi-select arrays, repeated query serialization, count summaries, and responsive wrapping. Fixed the 1440px project menu clipping by aligning the overlay inside the execution table panel.
- Added the execution-only viewport height budget so its two workbench panes land on the shared shell bottom gutter despite omitting the stage strip. Flattened gray nested fills in task facts, personnel facts, and personnel priority rows while retaining semantic status washes.
- Focused filter tests and the full `internal/db`, `internal/server`, and `internal/agenda` suites pass. Frontend check passes with 0 errors and the existing 72 warnings; production build, diff hygiene, exact-file Impeccable detection, and Finesse P0 detection pass.
- Authenticated isolated browser validation passed at 2048, 1440, 1024, 760, and 480 widths. HIT + NS2 returned 43 execution rows; adding 梁志远 + 朱家聪 returned the correct 3-row intersection; clearing restored 390 rows. Dropdowns are unclipped, narrow controls are 44px, document overflow is zero, and a fresh logged-in execution tab has no console errors.
- Used a copied database with external integrations disabled and left the existing 8080/5173 services untouched. No Jira/demand record was created, updated, assigned, synchronized, or reviewed; both isolated validation services were stopped.

## Session: 2026-07-18 - Daily Jira Decision Reminders And Viewport Fit

- **Status:** complete.
- Completed and recorded the mandatory Impeccable, design-taste-frontend, and finesse-ui review before frontend edits; agreement covers table ownership, whole-row selection, persistent status/reminder ownership, responsive pane behavior, and validation scope.
- Added the `DailyJiraDecision` migration and transactional creation beside `DecisionEvent`. Escalation schedules a 4-hour review; follow-up/reassignment schedule a 24-hour review.
- Extended audit responses with the newest decision state and added dynamic notification-center reminders that ignore superseded/future/resolved decisions and honor `daily_jira_reminder_<decision id>` dismissal keys.
- Moved cohort/search/refresh controls into the table header; made ARIA grid rows clickable from every cell and activatable with Enter/Space; added current decision/due-reminder status to the table and inspector.
- Refit the route to the exact remaining viewport with a 16px bottom gap. Desktop keeps aligned side-by-side panes; medium/narrow uses two bounded rows. Table and inspector remain internal scroll owners and the document does not scroll.
- Restarted the confirmed repository backend so AutoMigrate created `daily_jira_decisions`; the app is running with database configuration version 19 and the existing Jira worker schedule.
- Safe authorized-fixture browser validation of the real component passed pointer/keyboard row selection, header filtering, latest/future/absent decision states, wide/1024/760/480 geometry, 44px narrow controls, zero document overflow, zero unintended table horizontal overflow, and empty console errors. The temporary QA entry was deleted and no decision was submitted.
- Focused and full `internal/server` tests pass. `pnpm -C web check` passes with 0 errors and 74 unrelated existing warnings; production build, diff hygiene, Impeccable layout detection, and Finesse P0 detection pass.
- Reused the existing self-improvement entries for the recurring default Go cache and loopback `httptest` sandbox restrictions, updated their recurrence metadata, and completed validation with `/tmp` GOCACHE plus managed loopback permission.

## Session: 2026-07-18 - Daily Jira Visual Unification

- **Status:** complete.
- Loaded project-local Impeccable plus design-taste-frontend and finesse-ui, including the product, layout, redesign, anti-cheap, and preflight references required by the UI gate.
- Reused the live authenticated Chrome session and inspected Daily Jira with real data at the default wide viewport plus medium/narrow viewport overrides without submitting any decision.
- Compared the surface with the adjacent `决策事项` workbench and confirmed the mismatch is material/radius/hierarchy/responsive spacing, not data ownership or workflow semantics.
- Completed the three-way review and recorded hierarchy, ownership, responsive behavior, validation scope, protection rules, and disagreement resolution before frontend edits.
- Ran the subjective layout assessment before the Impeccable mechanical pre-scan. The detector returned `[]`; no arbitrary Tailwind spacing/z-index patterns exist in the target.
- Implemented the scoped `DailyJiraAudit.svelte` unification: compact matte command bar, independent cohort controls, aligned 20px table/inspector workbench, wider desktop inspector, inset table surface, content-aware stacked height, and secondary-column collapse at narrow widths.
- Svelte/TypeScript checks pass with 0 errors; the 74 existing warnings remain in six unrelated legacy files. Production build, tracked/untracked diff hygiene, Impeccable complete/layout scans, and Finesse P0 scan pass.
- Authenticated screenshot/geometry review passed at 2133x903, medium 3-row stack, CSS 750px, and CSS 478px. Wide panels align exactly; medium table contracts to 340px; narrow tables have no overflow and expose the inspector immediately below.
- Search-to-detail synchronization passed on real `ICA-10903`, then the temporary search was cleared. No decision was submitted and no business data was mutated.
- Final default-width state is left on `每日 Jira / 7 日及以上` with 133 real rows, `ZPU-2769` selected, zero horizontal overflow, and an empty console error list.

## Session: 2026-07-18 - Daily Jira 404 Runtime Recovery

- **Status:** complete.
- Confirmed the live `/api/decision/daily-jira` returned 404 even though the route exists in `internal/server/server.go`.
- Identified port 8080 as a stale `go run cmd/server/main.go` process started on 2026-07-15 from this repository.
- Restarted only that confirmed repository backend using the same command; the database-backed configuration version loaded normally and the Jira worker resumed its existing schedule.
- Verified the direct 8080 route and the Vite-proxied 5173 route both now return 401 without a token, proving the protected Daily Jira route is loaded instead of falling through to 404.
- No source-code change was required for the 404; the fix was runtime reload. The user only needs to refresh the Daily Jira page.

## Session: 2026-07-18 - Daily Jira Morning Review

### Phase 1: Discovery and mandatory review preparation
- **Status:** in_progress
- Started from the user request for a decision-area `每日 Jira` submenu with today / 3-day / 7-day unresolved audits, morning assignment, and retrospective traceability.
- Loaded the repository cold-start route, selected the complex coding preset, and loaded project planning/memory rules.
- Loaded the mandatory Impeccable, design-taste-frontend, finesse-ui, and planning-with-files instructions.
- Recorded the active task, constraints, review gate, phases, and validation scope before implementation.
- Ran Impeccable context discovery for `web`; the project has `DESIGN.md` but no `PRODUCT.md`, which is acceptable for this scoped change.
- Read the product-register guidance and existing Phase 41 design contract, then traced the first decision/Jira route and API touchpoints.
- Confirmed the shell's reusable submenu contract and the App-level view/breadcrumb patterns.
- Confirmed local Jira tasks already retain creation age, last activity, due date, current assignee, unresolved status, and decision logs.
- Logged a discovery-path error for nonexistent `internal/server/task_handlers.go` and switched future inspection to resolved source paths.
- Completed and recorded the mandatory three-way UI review. The gate now permits frontend implementation.
- Fixed the age contract at natural-day cohorts 0, 3-6, and 7+; selected dedicated read/review APIs with structured and legacy audit persistence.
- Implemented `internal/server/daily_jira_handlers.go` and focused tests for cohort boundaries, unresolved/Jira filtering, reassignment persistence, kanban sync, structured history, legacy history, and preservation of stale activity on review-only decisions.
- Added the decision submenu/view wiring in `App.svelte` and `FunctionalAdminShell.svelte`, plus the new responsive `DailyJiraAudit.svelte` table/inspector workbench.
- Focused Go tests passed: `GOCACHE=/tmp/well-ambient-gocache go test ./internal/server -run 'Test(BuildDailyJiraAuditResponse|PostDailyJiraReview)' -count=1`.
- `pnpm -C web check`, `pnpm -C web build`, app TypeScript, and `git diff --check` passed. Svelte reports 74 existing warnings in six legacy files and no remaining warning in `DailyJiraAudit.svelte`.
- The first design-detector sweep correctly exposed pre-existing App-level side stripes/gradient motion debt plus four new pure-white fallbacks; the new fallbacks were corrected, while unrelated legacy debt remains outside this feature slice.
- Both target-only design detectors now pass with no finding in `DailyJiraAudit.svelte`.
- The already-running app had no authenticated in-app-browser session, and its backend did not enable loopback dev auth. Switched to an isolated static build plus fixture-only API so authenticated validation cannot mutate real Jira or historical local data.
- Authenticated desktop DOM validation passed submenu state, breadcrumb ownership, default urgent cohort, all three bucket counts, watch/unclassified indicators, populated table selection, detail facts, decision controls, Jira link, and structured decision history.
- Authenticated 1024 and 760 validation passed responsive stacking, mobile sidebar entry, table scroll ownership, detail form/history placement, and the disabled-to-enabled reassign guard. Browser QA also caught and verified the search/detail selection repair; no decision submission was performed.
- Added server-side core-scope enforcement while preserving unassigned Jira for dispatch, plus backend note/assignee length validation.
- Final focused daily-Jira tests and the complete `internal/server` package suite pass; the full suite required managed loopback permission for existing `httptest` webhook cases. TypeScript, production build, `git diff --check`, and both target design detectors pass; browser console has no error entries.
- Files modified for this task: `internal/server/daily_jira_handlers.go`, `internal/server/daily_jira_handlers_test.go`, `internal/server/server.go`, `web/src/App.svelte`, `web/src/components/prototype/FunctionalAdminShell.svelte`, `web/src/components/DailyJiraAudit.svelte`, plus active plan/findings/progress and error-log records.


## Current Task: Surface-Aware Text Contrast

- 2026-07-16: Added the user-specified dark-surface contrast rule and completed the mandatory Impeccable, design-taste-frontend, and finesse-ui review before frontend edits.
- 2026-07-16: Review resolves in favor of a scoped semantic modal foreground contract plus rendered contrast scanning; the light full-height drawer remains visually unchanged.
- 2026-07-16: Authenticated computed styles confirmed the issue on the two visible primary controls (`rgb(1,139,141)` background with `rgb(3,18,25)` text); light drawer ink and dark rail ink are already correctly paired.
- 2026-07-16: Added dedicated filled-accent background/hover/ink tokens and applied them to the target demand/delivery actions plus shared primary-action and selected-date contracts.
- 2026-07-16: Added the detector-required reduced-motion fallback for the shared Button loading spinner.
- 2026-07-16: Authenticated wide and 760×800 visual checks confirm both target controls use 5.70:1 cool-white-on-dark-teal contrast, drawer/rail surface pairings remain correct, horizontal overflow is zero, and console errors are empty.
- 2026-07-16: Final Svelte/TypeScript check, production build, `git diff --check`, and finesse P0 detector gate pass.
- 2026-07-16: **Status:** complete.

## Current Task: Viewport-Height Demand Drawer

- 2026-07-16: Inspected the new screenshot at original resolution and confirmed the active drawer is bounded by the flow-board workspace instead of the browser viewport.
- 2026-07-16: Reapplied the mandatory Impeccable, design-taste-frontend, and finesse-ui review. The shared direction is a viewport-fixed, square-corner, full-height right drawer with a global backdrop and unchanged single-body scrolling/focus lifecycle.
- 2026-07-16: Recorded component ownership, responsive behavior, disagreement resolution, and exact geometry/browser validation criteria before editing frontend code.
- 2026-07-16: Implemented viewport-fixed `100dvh` geometry and square drawer edges; Svelte check, production build, and diff hygiene pass, with detector output limited to documented legacy findings outside this change.
- 2026-07-16: Authenticated wide/mobile geometry found the drawer rectangle correct but the shell top bar still above it because `.workspace-stage` is an isolated stacking context. Activated the pre-agreed portal fallback instead of weakening the global shell isolation rule.
- 2026-07-16: Portaled only the demand-detail and detail-companion overlay roots to `.functional-console`; the global shell isolation contract remains unchanged.
- 2026-07-16: Final authenticated visual and geometry validation passed at 2133×1038 and 760×800. Drawer/header own the top edge, the global backdrop owns the shell, AI companion keeps a 16px gap, Escape/backdrop/focus/body-scroll lifecycles pass, horizontal overflow is zero, and console errors are empty.
- 2026-07-16: Final Svelte/TypeScript check has zero errors; production build, `git diff --check`, and detector P0 gate pass. Detector findings are documented legacy selectors outside this scoped change.
- 2026-07-16: **Status:** complete.

## Current Task: Flow Detail Drawer And Unified Task Filters

- 2026-07-16: Classified as complex frontend coding with shared-component, responsive, and authenticated browser-visible risk.
- 2026-07-16: Loaded cold-start routing, complex coding workflow, tool/memory routing, planning-with-files, project-local Impeccable, design-taste-frontend, and finesse-ui instructions.
- 2026-07-16: Ran Impeccable context for `web`; `PRODUCT.md` is absent, so this scoped refinement proceeds against the existing code and `DESIGN.md` without initiating a new product brief.
- 2026-07-16: Inspected both supplied screenshots at original resolution and recorded the drawer, unified-filter, panel-alignment, responsive, and scroll-ownership acceptance criteria.
- 2026-07-16: Traced demand-detail lifecycle to `DemandKanban.svelte`, confirmed the current detail still uses the generic centered modal host, and found the existing telemetry off-canvas panel as the closest repository interaction reference.
- 2026-07-16: Traced the task toolbar to `TaskKanban.svelte`; shared Select already owns project/owner dropdown behavior, while the categorical risk filter remains a custom segmented strip.
- 2026-07-16: Completed and recorded the mandatory three-way review. The agreed implementation uses a local state-safe demand drawer, shared Select for all categorical task filters, and one desktop table/inspector height contract with deliberate stacked fallbacks.
- 2026-07-16: Phase 3 implementation started only after hierarchy, component ownership, responsive behavior, accessibility, validation scope, and disagreements were recorded.
- 2026-07-16: Confirmed implementation mechanics: the workspace-scoped detail host can become a right-edge drawer in place, the companion needs a wide-screen left-of-drawer override, and execution can reuse the existing aligned panel-height token.
- 2026-07-16: Implemented the demand drawer, focus/Escape/reduced-motion behavior, wide companion placement, non-searchable shared risk Select, and aligned execution table/inspector height contract.
- 2026-07-16: First `git diff --check` and Svelte check pass with 0 errors; removed the only newly surfaced unused risk-button selector before continuing validation.
- 2026-07-16: Reused the signed-in Chrome localhost session and visually reviewed the real flow-board drawer at 2133x1038 and 760x800, including long-body scroll ownership, Tab containment, Escape/focus return, and zero horizontal overflow.
- 2026-07-16: Browser QA found the first wide AI companion collapsed to about 1px. Replaced the percentage-after-padding width calculation with a bounded flex basis, then revalidated a 620px companion, 16px drawer gap, zero overlap, Escape close, and focus return.
- 2026-07-16: Visually reviewed execution tracking at wide desktop and 760px. Risk is a shared dropdown, table/inspector top and bottom deltas are both 0px at desktop, and the 1180px stacked state keeps a 16px gap with equal panel heights.
- 2026-07-16: Final `pnpm -C web check` passes with 0 errors and 74 existing warnings; `pnpm -C web build` and `git diff --check` pass. Finesse detection reports P0=0; full-file detector warnings are legacy rules outside the changed surfaces.
- 2026-07-16: Final authenticated browser smoke check confirms the semantic demand and AI dialogs, 620px companion geometry, 16px gap, no overlap/overflow, clean overlay close state, and an empty console error list.
- 2026-07-16: **Status:** complete.


## Current Task: Event Timeline, Flow Detail, And Execution Filters

- 2026-07-15: Classified as complex frontend coding with shared-component and browser-visible regression risk.
- 2026-07-15: Loaded cold-start routing, complex coding workflow, memory/tool routing, local Impeccable, design-taste-frontend, finesse-ui, ui-design-system, and planning-with-files instructions.
- 2026-07-15: Ran Impeccable project context for `web`; existing `DESIGN.md` found, `PRODUCT.md` absent, so the scoped fix proceeds against existing code.
- 2026-07-15: Recorded the dirty-tree constraint and created the current task addendum before code inspection or edits.
- 2026-07-15: Inspected both source screenshots at original resolution and translated the visible scrollbar, modal-height, duplicate-filter, and dropdown-stacking symptoms into explicit acceptance criteria.
- 2026-07-15: Logged and resolved one ripgrep leading-hyphen pattern error; subsequent CSS token searches use the option terminator.
- 2026-07-15: Completed the mandatory Impeccable, design-taste-frontend, and finesse-ui reviews. All three agree on one scroll owner, canonical project/owner filters in the execution toolbar, shared Select ownership, deliberate responsive reflow, and authenticated browser coverage.
- 2026-07-15: Resolved the demand-detail review disagreement in favor of a scoped geometry and scrollbar-rail repair that preserves long-content scrolling; a broader progressive-disclosure/shared-modal migration is explicitly deferred.
- 2026-07-15: Phase 3 implementation started only after the shared direction and disagreement resolution were recorded in the active plan.
- 2026-07-15: Removed the mediation ledger's nested overflow boundary; consolidated execution project/owner filtering into the active toolbar using shared Select; and repaired demand-detail/companion workspace geometry plus hidden-rail body scrolling.
- 2026-07-15: `pnpm -C web check` passed with 0 errors and 78 existing warnings; production build and `git diff --check` passed. Targeted Impeccable detection found only legacy rules outside the changed surfaces.
- 2026-07-15: Isolated authenticated browser validation passed decision current/all expansion, demand detail at 2048/1280/760, companion alignment and linked close, execution filter combinations, open-menu hit testing, responsive reflow, 44px narrow controls, and zero horizontal overflow.
- 2026-07-15: **Status:** complete.


## 2026-07-14 - Phase 68 corpus approval information architecture started

- Inspected the supplied authenticated screenshot at original resolution and traced the repeated queue/detail content to `CorpusCandidateReview.svelte`.
- Completed the mandatory Impeccable, design-taste-frontend, and finesse-ui review. All three agree on source-batch grouping, one primary proposal for ordinary review, on-demand source evidence, and full comparison only for impact review.
- Confirmed the current candidate shape already includes document ID and source-document metadata, so the page can remove repeated presentation without dropping records or changing the backend lifecycle.
- Recorded hierarchy, component ownership, responsive behavior, validation scope, and the comparison disagreement resolution in the active Phase 68 plan before frontend implementation.

## 2026-07-14 - Phase 68 corpus approval information architecture complete

- Rebuilt the queue around source-document batches, keeping every candidate row while rendering shared filename/version metadata once per batch.
- Replaced the ordinary two-document comparison with one Typora-like live candidate workbench; moved metadata and immutable source evidence into explicit disclosures and reused shared Select for all classification controls.
- Kept the current-context versus proposed-content comparison only for `impact_review`, preserved the two-stage publication gate, and removed the duplicated impact heading wrapper.
- Added content-aware title suppression, bounded local queue scrolling, single-column responsive collapse, and mutually exclusive error versus successful-empty presentation.
- Extended the authenticated Settings preview with two document batches, ordinary/high-sensitivity candidates, impact responses, lifecycle responses, and explicit populated/empty/error variants.
- Browser validation passed source grouping, ordinary editor, shared Select open/Escape, source evidence disclosure, high-sensitivity transition, impact comparison, empty/error states, and 1440/1024/760/480 geometry. The final clean run has no console warnings or errors.
- `pnpm -C web check` passes with 0 errors and 78 existing unrelated warnings; `pnpm -C web build`, targeted Impeccable detection, and `git diff --check` pass.

## 2026-07-14 - Phase 67 shared Select and live Markdown composition started

- Audited the native scope dropdown, the shared Select API/interaction contract, and the existing CodeMirror Markdown workbench.
- Completed the mandatory Impeccable, design-taste-frontend, and finesse-ui review before editing frontend files.
- Chose a bounded WYSIWYM implementation: canonical Markdown remains unchanged, inactive lines render semantic formatting, and the active line reveals syntax for precise editing.
- Recorded component ownership, responsive behavior, protection rules, browser validation scope, and the rejected lossy `contenteditable` alternative in `task_plan.md`.
- **Status:** in progress

## 2026-07-14 - Phase 67 shared Select and live Markdown composition complete

- Replaced the import form's native scope dropdown with shared Select, including the existing global-scope cleanup, disabled state, adaptive menu, and 44px narrow-screen control sizing.
- Added CodeMirror-native `live` mode to MarkdownWorkbench. Inactive lines immediately render semantic Markdown while the active line exposes syntax; canonical Markdown and existing change/save/commit contracts remain unchanged.
- Made corpus Markdown paste start in `即时排版`; retained explicit `源码 / 分屏 / 阅读` modes for precision editing and compatibility.
- Replaced the detector-flagged blockquote side border with a semantic quote widget and restrained row tint. Final targeted Impeccable detection returns `[]`.
- Svelte check passes with 0 errors and 78 existing warnings; TypeScript, production build, and `git diff --check` pass.
- Isolated authenticated Chrome validation passed Select mouse/keyboard lifecycle, raw Markdown preservation, all live formatting families, fallback modes, zero console/page errors, and zero horizontal overflow at 1440/1024/760/480. Narrow Select/mode controls measure 44px.
- Reviewed screenshots: `output/ui-validation-corpus-live-markdown-1440.png` and `output/ui-validation-corpus-live-markdown-480.png`.
- **Status:** complete

## 2026-07-15 - Phase 72 Markdown, decision ledger, palette, and flow detail redesign complete

- Reduced the shared Markdown toolbar to one default `即时排版` mode while preserving explicit advanced edit and read-only preview lifecycles.
- Hid visible option-list rails across shared Select/MultiSelect and compatible listboxes without removing overflow scrolling or keyboard behavior.
- Replaced the detached full-width decision timeline with a contextual current/all evidence ledger inside the strongest-brain inspector, including empty, automatic, manual, actor, task, time, and commit states.
- Introduced the supplied color library through semantic product tokens with contrast-safe foregrounds and restrained state usage.
- Rebuilt the flow-board demand detail into a wider bounded workbench with flat facts, adjacent actions, one body scroll owner, natural-height Markdown preview, responsive one-column narrow layout, and 44px narrow actions.
- Authenticated safe-fixture browser validation covered decision current/all/empty states, long hidden-rail dropdown overflow, selection/detail ordering, populated detail, Markdown preview/edit, and 1440/1024/760/480 geometry. A later reconnect was blocked by the browser URL policy and was not bypassed.
- Final checks pass: `pnpm -C web check` reports 0 errors and 78 existing warnings in 6 files; production build and `git diff --check` pass. Full-file detectors report only documented legacy findings outside the Phase 72 selectors.
- **Status:** complete

## 2026-07-15 - Phase 72 interface system refinement started

- Reloaded the project-local Impeccable, design-taste-frontend, finesse-ui, project UI-system, and planning-with-files guidance before frontend edits.
- Inspected the supplied flow-board detail screenshot and recorded the nested-scroll, narrow-column, repeated-card, and first-screen hierarchy defects.
- Established the initial product-register direction and semantic mapping for the requested source palette; implementation remains blocked on completing the mandatory isolated layout assessments and target-file audit.
- Completed the mandatory isolated subjective layout assessment and mechanical pre-scan, then recorded the four-skill agreement, disagreement resolution, component ownership, responsive contract, protection rules, and validation scope in `task_plan.md` before frontend edits.
- **Status:** in_progress

## Session: 2026-07-13 - Responses-first LLM Transport And Unified Draft Actions

- **Status:** complete with a safety-scoped browser-streaming validation exception
- Added a unified provider client for OpenAI/Sub2API Responses and native Claude Messages, including sync and SSE parsing, auth headers, file IDs, and legacy Chat Completions URL migration.
- Migrated demand specs, demand deconstruction, autonomous execution, telemetry review, and AI connection tests away from active Chat Completions payloads and `choices[].message` parsing.
- Converted `/api/deconstruct` to provider SSE plus browser NDJSON and updated both Svelte consumers to read the streamed completion through a shared parser.
- Replaced the Completions configuration choice with Responses API and Claude Messages, and moved `撤销草案` directly before `保存契约` in one right-aligned footer group.
- Go package tests, Svelte check, production build, structural footer assertion, and diff hygiene pass. Authenticated browser validation covered both protocol-choice states without saving or calling a provider.
- Full authenticated draft replay was stopped because the isolated copy of historical data activated background delay-alert handling; the safety reviewer rejected a restart to prevent possible external notifications.

## Session: 2026-07-13 - Demand Detail Action Hierarchy And Draft Footer

- **Status:** complete
- Moved `撤销草案` into the bottom contract action row, left-separated from save/approve progression actions, while preserving the no-contract fallback and existing confirmation flow.
- Replaced the detail action pair's inherited purple treatment with shared teal-primary and neutral-secondary semantics.
- Authenticated DG-319 testing passed desktop and narrow layout, hover, keyboard focus, withdrawal open/cancel, action adjacency, and horizontal-overflow checks without saving or mutating business data.
- `pnpm --dir web check`, production build, `git diff --check`, and Phase 61-targeted Impeccable detection pass; browser console errors are empty.

## Session: 2026-07-13 - Deterministic Multi-select Completion

- **Status:** complete
- Reproduced the user's screenshot failure and confirmed the open candidate list covered required roles and owner controls, making their clicks hit reviewer options instead of outside-dismiss logic.
- Reworked the shared multi-select so its bounded options participate in form flow, added selected-count plus `完成选择`, and preserved consecutive multi-selection.
- Fixed Escape reopening, removed the parent container's unconditional open handler, and anchored the chevron to the trigger so it cannot drift into options when the list expands.
- Real-component browser testing passed select, complete, chevron close, outside click, Escape, desktop geometry, and 480px responsive behavior without saving business data.
- Svelte check reports 0 errors; production build, targeted Impeccable complete/layout scans, and diff hygiene pass.

## Session: 2026-07-12 - Review Select Dismissal And Core-member Boundary

- **Status:** complete with an authenticated-browser validation exception
- Repaired `Select` and `MultiSelect` at the shared primitive: capture-phase outside pointer, focus exit, Escape, and an explicit chevron toggle now close either list even inside modal content that stops bubbling clicks.
- Preserved deliberate selection semantics: a single owner selection closes immediately; reviewer multi-select remains open only so multiple people can be selected consecutively.
- Filtered review participants through backend `coreMemberVisibility` and filtered the old-backend frontend compatibility directory through `/api/demands/options`.
- Added directory-signature normalization so a late core-member response clears stale non-core reviewer/owner values before save.
- Focused Go tests, `pnpm --dir web check` (0 errors), production build, targeted Impeccable complete/layout scans, and `git diff --check` pass.
- Authenticated browser interaction was attempted after restarting Vite, but the in-app browser stayed locked to the earlier localhost connection-error page by browser security policy; no workaround or alternate browser surface was used.

## Session: 2026-07-12 - Demand Detail AI Workbench And Executable Spec Draft

- **Status:** complete

- Implemented draft-only withdrawal and atomic cleanup of the matching review contract; frozen specifications remain immutable.
- Expanded AI deconstruction and draft persistence with business rules, main/exception flows, permission rules, data impact, API impact, UI impact, dependencies, risks, acceptance criteria, test plan, repositories, and task breakdown.
- Unified create, detail, and schedule companion ownership; host close now closes the companion and desktop geometry uses the real host width.
- Fixed the adjacent detail-to-schedule transition by capturing the demand before clearing detail state.
- Focused demand-spec tests, `pnpm -C web check`, `pnpm -C web build`, TypeScript compilation, and `git diff --check` passed.
- Authenticated browser validation at 2133x902 measured exact group centering, a 16px panel gap, about 1px host/companion height delta, and zero horizontal overflow for detail and schedule pairs.
- Existing draft `DG-319` displayed the implementation blueprint and guarded withdrawal confirmation; final deletion was intentionally not executed.

## Session: 2026-07-11 - Health Diagnosis Intervention Modal Calibration

### Phase 1: Visual and code audit
- **Status:** complete
- Loaded the existing-project redesign, UI audit, and file-planning workflows.
- Recorded the constraint to reduce contrast without weakening genuine risk/intervention signals.
- Inspected the supplied screenshot and located the modal markup in `ProjectHealthTelemetry.svelte`.
- Captured the visual root cause: six competing accent families plus nested bordered cards, not simply “red is too bright.”
- Traced the modal markup and confirmed it mixes inline layout styles with legacy dark rules and later light overrides.
- Logged a harmless CSS-token search error caused by a missing `rg --` separator.
- Next action: inspect the final active modal override block, then define and implement one scoped modal contract.

### Phase 2: Visual contract
- **Status:** complete
- Kept desktop two-column information architecture and defined a single-column breakpoint below 860px.
- Chose system cool neutrals and teal as the base, with muted semantic accents only where risk meaning requires them.
- Chose body-owned scrolling, reduced nested-card borders, and scoped styles to avoid changing project tables.

### Phase 3: Implementation
- **Status:** in_progress
- Next action: add the modal scope/context header, remove inline layout declarations, add metric tone classes, and implement the final scoped style block.
- Implemented the scoped modal contract and passed production build plus target-component static checks.
- First isolated browser harness attempt failed because `document.createElement` is unavailable in the constrained evaluation surface; logged and switched to direct body markup replacement.
- Direct body replacement was also rejected by the read-only evaluation DOM. Stopped retrying DOM mutation and moved the harness to a temporary preview-served HTML file.

## Session: 2026-07-11 - Streamed LLM Attachments And Demand File Archive

### Phase 1: Architecture correction
- **Status:** complete
- Applied file-planning, self-correction, and existing admin UI workflows.
- Recorded the corrected boundary: original files belong in backend multipart handling, compressed local storage, durable demand/deconstruction association, and provider-aware LLM delivery.
- Confirmed the current deconstruction handler is JSON-only and sends text-only OpenAI-compatible payloads; GORM AutoMigrate is the schema path.
- Verified the official Responses `input_file` contract and accepted `.pdf`, `.doc`, and `.docx` formats. Chose inline Base64 file data for provider delivery and multipart streaming for browser-to-server upload.
- Traced the post-preview import transaction and selected `context_pack_id` as the stable staging link, with archive linkage finalized during `/api/tasks/import`.
- Logged a second discovery-path error caused by naming a nonexistent root `main.go`; no implementation action depended on that failed path.
- Logged and corrected a failed discovery command caused by an unmatched zsh glob.
- Next action: trace provider request types, deconstruction archive creation timing, database migrations, and existing data-directory conventions before changing contracts.

### Phase 2: Contract definition
- **Status:** complete
- Selected streamed Files API upload plus Responses `input_file`, with remote cleanup and local gzip retention.
- Defined staged context-pack association followed by import-time archive association.
- Defined additive multipart fields, bounded file counts/sizes, storage configuration, and attachment metadata.

### Phase 3: Backend implementation
- **Status:** complete
- Next action: add model/migration, storage/provider helpers, handler integration, and import linkage.
- Added the attachment model/migration, gzip storage and multipart reader, streamed Files API uploader, Responses file references, remote cleanup, and import-time archive linking.
- Dependency cleanup hit a pnpm store-location mismatch; logged it and will retry with the already-linked store path.
- Frontend production build and diff hygiene pass; full Svelte check remains blocked by the existing `DemandKanban.svelte:1798` type error.
- Initial Go tests did not reach compilation because the default build cache is outside the sandbox; next run will use `/tmp/well-ambient-gocache`.
- The cache-safe backend run compiled successfully but the broad server package was stopped by an unrelated loopback-binding test; focused new tests remain the validation target.

### Phase 4: Frontend streaming conversion
- **Status:** complete
- Replaced local parsing with up to three retained `File` objects and multipart submission.
- Added attachment IDs to the deconstruction/import state bridge and updated upload copy to describe compressed archival plus LLM parsing.
- Removed browser parsing libraries from package metadata and lockfile.

### Phase 5: Focused validation
- **Status:** complete
- Added and passed a provider-loopback end-to-end test covering upload bytes, file ID use, remote deletion, gzip metadata, demand association, and archive association.
- Updated the test to begin with completions configuration and confirmed attachment calls auto-route to Files + Responses.
- Complete backend suite passes; frontend production build and repository-wide diff hygiene pass.
- `pnpm check` remains independently blocked by the pre-existing `DemandKanban.svelte:1798` union-property error; the production build validates the changed frontend path.

## Session: 2026-07-11 - Demand Difficulty And Task Detail Surface

### Audit
- **Status:** complete
- Applied the UI design-system and file-planning workflows to the two screenshot-scoped issues.
- Traced difficulty from the custom dropdown to the schedule request and confirmed backend normalization accepts the existing option values.
- Traced the task detail mismatch to legacy dark colors inside TaskKanban content rendered in the shared light Modal.

### Implementation And Validation
- **Status:** complete
- Replaced the bespoke difficulty dropdown with shared Select and removed its redundant local open/close state and dark menu CSS.
- Added the `task-detail-surface` light palette contract after all legacy TaskKanban rules, including narrow-screen layout behavior.
- Frontend check passes with 0 errors and 74 existing warnings; production build and diff hygiene pass.
- Focused demand scheduling test passes and confirms selected difficulty persistence.
- Browser interaction remains unverified because both target surfaces require a real authenticated session unavailable to the standalone preview.

## Session: 2026-07-11 - Adaptive Overlays And Decision Timeline

### Corrective Follow-up: Historical Event Dates
- **Status:** complete
- User screenshot showed future-looking times under Today; traced the issue to the backend stripping dates with `Format("15:04:05")` and the frontend defaulting time-only records to today.
- Added `occurred_at` to automatic decision records, populated it from task `LastUpdate` and fallback event timestamps, and retained `time` for compatibility.
- Updated the timeline to prefer the full timestamp and added a safe old-backend rollover rule.
- Verified the reported task IDs against the read-only SQLite database; their dates span June 19 through July 10 rather than today.
- Focused Agenda tests, frontend check/build, and diff hygiene pass.

### Corrective Follow-up: HR-4202 Reschedule Consistency
- **Status:** complete
- Traced the intervention and schedule endpoints to the same TaskTelemetry row and inspected HR-4202 plus its latest decision event read-only.
- Prevented unchanged due dates from being presented or recorded as successful adjustments.
- Added explicit frontend target capture, target-date button copy, server response verification, and a persisted due-date response.
- Added and passed an intervention-to-schedule projection regression for June 25 to July 13 plus a no-op rejection assertion.
- First test-fixture patch matched an earlier `now := time.Now()` in the same file; moved the fixture using the exact test-function boundary and reran successfully.
- Focused server test, frontend check/build, JSON/diff hygiene pass.

### Audit
- **Status:** complete
- Read the repository cold-start rules, complex coding preset, UI design-system skill, file-planning skill, and failure-handling skill.
- Inspected the supplied screenshots and traced each reported behavior to the active shell, shared Select/DatePicker primitives, and `DecisionDashboard.svelte` split log region.
- Confirmed the worktree is already dirty; changes will remain limited to the targeted UI components plus these existing planning files.
- First combined planning-file patch failed because `progress.md` starts with `# Progress Log`, not `# Progress`; re-read the live file and applied this smaller exact-context patch.

### Implementation And Validation
- **Status:** complete
- Added outside-click notification dismissal to the active functional shell.
- Added shared viewport-aware up/down placement and bounded overflow to Select and DatePicker.
- Replaced split automatic/manual log cards with one content-height, date-grouped decision event timeline.
- `pnpm --dir web check` passes with 0 errors and 74 existing warnings.
- `pnpm --dir web build` and `git diff --check` pass.
- Browser-tested shared Select down/up behavior at 1280x520 with zero console errors; authenticated notification/timeline browser testing was unavailable from the no-login preview.
- Initial preview command failed at the repository root because only `web/package.json` exists; retrying with `pnpm --dir web preview` was correct. The sandbox then blocked local binding, and the approved local preview command succeeded.

## Session: 2026-07-10 — KPI Workspace Clean Rebuild

### Phase 1: Failure audit
- **Status:** complete
- Loaded the UI audit, product-dashboard, redesign, and self-improvement workflows because the user rejected the prior KPI result.
- Inspected the supplied screenshot and confirmed the failure is structural, not a spacing issue.
- Identified conflicting legacy grid-area, display-contents, sticky panel, and responsive rules as the root cause.
- Next action: replace `KPIKanban.svelte` entirely with one source-order-aligned component and one scoped style system.

### Phase 2: Clean data and read-model layer
- **Status:** complete
- Replaced the entire legacy KPI component while preserving the performance and report-preview APIs.
- Kept real daily/weekly summaries, member workload, Bug count, delay ratio, requirement base score, capability rating, risks, meetings, and evidence.
- Added backward-safe frontend fallbacks for a currently running backend that has not yet restarted with the newest KPI DTO fields.

### Phase 3: Single-flow workspace
- **Status:** complete
- Implemented one vertical flow: period toolbar, four metrics, delivery/risk overview, people capability workbench, and evidence grid.
- Used local overview and member grids only; each collapses independently without named page-level areas or sticky page inspectors.
- Capped the risk reader at 430px after live measurement exposed unnecessary row stretching.

### Phase 4: Validation
- **Status:** complete
- `pnpm --dir web check` passes with 0 errors and 74 non-KPI warnings, down from 94 because the obsolete KPI CSS was deleted.
- `pnpm --dir web build` and `git diff --check` pass.
- Authenticated browser verification passed at the default 2133px-class viewport, 900px override, and 390px override with zero document-level horizontal overflow.
- Runtime interactions passed: 日报/周报, member selection, personal report, evidence scoping, and team-report restoration. Browser console has zero errors.
- Final skill pre-flight: no fake data, no marketing hero, no decorative motion, no legacy grid tracks, explicit focus states, 44px member controls, reduced-motion skeleton fallback, and bounded table/risk scrolling.

## Session: 2026-07-10 — Main Workspace IA And Metrics Rebuild

### Phase 1: Audit and contracts
- **Status:** complete
- Loaded the UI design-system, product-dashboard refinement, existing-project redesign, and file-planning workflows.
- Locked the product design read to calm precision, low spectacle, and high information density.
- Declared shell-owned page identity, real-data-only metrics, and preservation of existing behavior as protection rules.
- Next action: inspect App shell subnavigation, the five target components, and relevant KPI/scoring API contracts before applying structural edits.

### Phase 2: Decision, Schedule, and Evidence hierarchy
- **Status:** complete
- Removed the three duplicate intro/title cards and preserved their actions by moving controls into the first real data surface.
- Rebuilt Decision's top row around six priority statistics and kept it responsive at three, two, and one columns.
- Authenticated browser verification confirms the Schedule toolbar stays single-line at desktop and all three routes have zero document-level horizontal overflow.

### Phase 3: Task information architecture and People Load
- **Status:** complete
- Added shell-owned Task Table, People Load, and Execution Tracking submenus with child-aware breadcrumbs and scroll reset keys.
- Removed the in-page task title/view-switch card and replaced it with one shared filter/context toolbar.
- Rebuilt People Load as a sortable capacity table plus selected-person risk inspector; authenticated data showed 13 members and no page overflow.
- Kept Task Table and Execution Tracking separate while aligning their project/owner filter language.

### Phase 4: Daily/weekly metrics and capability model
- **Status:** complete
- Removed the KPI hero card and the redundant score/distribution dashboard stack.
- Reorganized the page into period controls, multidimensional health metrics, report evidence, personal capability matrix, and department facts.
- Extended the KPI response with total task/Bug counts, delay ratio, derived requirement base score, scored-item count, and manual-revision count.
- Added a focused unit test covering AI difficulty and manual-estimate base-score paths.

### Phase 5: Validation and pre-flight
- **Status:** complete
- `pnpm --dir web check` passes with 0 errors; the 94 warnings are legacy accessibility/unused-selector noise, including selectors made obsolete by intentionally removed cards.
- Focused KPI/server tests pass; production build and `git diff --check` pass.
- The first sandboxed full Go run reached the existing local-listener test and was blocked from binding `httptest` on `[::1]`; the approved local-listener rerun completed with `go test ./...` passing across every package.
- Added and passed an aggregation assertion for personal task/Bug totals, same-population delay ratio, base-score fallback, AI difficulty scoring, and manual estimate scoring.
- Authenticated Chrome validation covered Decision, Schedule, Evidence, all three Task submenus, KPI weekly/daily switching, and 390px-class narrow layout. All target routes have zero document-level horizontal overflow; the KPI data table scrolls only inside its bounded table shell on narrow screens.
- Anti-cheap/pre-flight outcome: the redesign removes decorative intro cards, uses one existing accent/token system, shows only real API metrics, adds no motion or fake data, and keeps product density high with explicit mobile collapse rules.

## Session: 2026-07-10 — Rounded Surface And Modal Corner Audit

### Phase 1: Audit
- **Status:** complete
- Loaded the UI design-system, existing-project redesign, and file-planning workflows.
- Inspected the supplied screenshot and recorded the likely clipped-layer/root-radius mismatch.
- Next action: inventory shared tokens and every modal/card/panel/container selector before choosing the smallest shared fix.
- Confirmed the screenshot artifact comes from square translucent header/footer backgrounds escaping a `20px` modal whose overflow is intentionally visible for dropdown/date-picker overlays.
- Rejected a blanket `overflow: hidden` modal fix because it would clip interactive overlays; the repair will radius the modal's edge sections and strengthen shared visible-surface primitives separately.
- Audited the shared modal, project-health modal, telemetry drawer, Settings structural resets, and low-radius declarations. Shared modal primitives are already clipped; the screenshot bug is local to the demand/schedule modal family, while global work should target visible surface classes only.
- Completed the production-page inventory across decision, demand, task, KPI, deconstructor, telemetry, shared modal, and Settings surfaces. Outer page containers already converge on the `18px` token or explicit `14px`–`22px` radii.

### Phase 2: Shared and modal repair
- **Status:** complete
- Add a non-clipping rounded-surface contract to shared visible primitives, then give demand/schedule modal edge sections matching inner radii so overlay menus remain usable.
- Hardened global visible surfaces with padding-box background clipping and isolated paint contexts.
- Rounded the shared modal header layer and the demand modal family's header/footer/detail edge layers without clipping escaping dropdown/date-picker overlays.

### Phase 3: Page-specific gap closure
- **Status:** complete
- Confirmed active outer surfaces across Decision, Demand, Task, KPI, Deconstructor, Settings, shared modal, project health, and telemetry already resolve to the rounded design-system contract.
- Preserved intentional small radii for micro-controls and zero radii for transparent structural wrappers and the viewport-edge telemetry drawer.

### Phase 4: Validation
- **Status:** complete
- `pnpm --dir web check` passes with 0 errors and the existing 65 warnings.
- `pnpm --dir web build` and `git diff --check` pass.
- Authenticated browser inspection confirms the new-demand modal's four 1px corner points expose the backdrop, not square header/footer layers; the rendered screenshot has a continuous rounded silhouette.
- Authenticated browser audits pass for Schedule and Decision: exposed surface radii are `14px`–`20px`; the Schedule table's `0px` inner viewport remains safely nested inside a clipped `20px` parent.
- Authenticated browser audits pass for Evidence and Task: the evidence surface is `18px`; the dense task workbench consistently uses a tighter but fully rounded `10px` tier.
- Authenticated main-route audit also covered KPI's loading state and Settings' visible audit panel; no exposed square card/container was found. Static final-rule inspection covers KPI populated panels and Settings child routes.
- Restored the claimed browser tab to its original Schedule page with the new-demand modal open, then released browser control.
- Final rerun: `pnpm --dir web check` passes with 0 errors and 65 existing warnings; `pnpm --dir web build` and `git diff --check` pass.

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

## 2026-07-11 Phase 47 Start

- Read the repository routing instructions and selected the complex coding workflow.
- Applied the UI design-system and file-based planning skills for this cross-surface frontend fix.
- Confirmed the flow scroll failure is a broken bounded-height chain rather than a missing overflow declaration.
- Confirmed AI deconstruction is currently flow-only because `App.svelte` conditionally mounts it only for the board view.
- Confirmed the demand AI-spec typography drift comes partly from invalid CSS font shorthand, and the detail backdrop close condition needs hardening.

## 2026-07-11 Phase 47 Complete

- Made the flow root a bounded flex surface, set the board to consume the remaining height, and made every lane a real independent scroll container.
- Replaced the gray gap with one restrained translucent white surface behind the filter and lane region.
- Hardened demand-detail dismissal so only a direct backdrop click closes it; nested AI actions preserve the detail modal.
- Corrected controlled-delivery field/button typography and tightened its cards, spacing, radii, and textarea heights to the light admin-console baseline.
- Moved AI deconstruction ownership into `DemandKanban` and exposed seeded workbench entry points from new-demand capture, schedule inspection/edit, the flow toolbar, and demand detail.
- Removed the board-only standalone deconstructor mount from `App.svelte` so there is one reusable capability instead of a view-specific page appendage.
- Validation passed: `pnpm build`, `pnpm exec tsc -p tsconfig.app.json --pretty false`, and `git diff --check`.
- Browser preview reached the application login page, but no authenticated preview session was available, so protected-route visual interaction remains unverified in this run.

## 2026-07-11 Phase 48 Complete

- Replaced the disappearing `height: 0` flow board with a `560px–850px` viewport-clamped board and preserved independent lane scrolling.
- Added a compact AI mode that removes the metric strip, document upload, intent console, prompt contract, empty result table, and wide editable evidence table from contextual dialogs.
- Promoted demand association above the input and explained that it controls whether generated tasks reuse a demand task group or remain independent.
- Added compact generated-task rows and kept synchronization analysis below them only after a result exists.
- Made `AI 预解构` open as a right-side companion to the new-demand modal on wide screens, with a centered adaptive fallback on narrower screens.
- Changed new-demand project options to use only `/api/projects/config`, matching the settings project source and canonical project keys/names.
- Disabled horizontal resizing for requirement description textareas.
- Validation passed: `pnpm build`, `pnpm exec tsc -p tsconfig.app.json --pretty false`, and `git diff --check`; existing repository-wide Svelte accessibility/unused-selector warnings remain non-blocking.

## 2026-07-11 Phase 49 Complete

- Updated `.agents/output/delivery.yaml` so every UI change requires authenticated browser simulation and screenshot-based visual review; authentication blockers now require waiting for user input rather than claiming completion.
- Bound the AI companion height to the rendered new-demand modal height and centered the two-dialog desktop group with a 16px gap.
- Split title and description into distinct compact fields and changed `/api/deconstruct` input to labeled `【需求标题】` / `【需求内容】` sections.
- Authenticated browser validation passed at the default responsive viewport and 2048x925.
- Wide-screen measurements: create `680x599.27`, AI `620x597`, group center `1024/1024`, gap `16px`, no horizontal overflow.
- Screenshot review confirmed consistent surface/border/text colors, aligned top/bottom edges, readable information hierarchy, and a centered narrow-screen fallback.
- Saved validation screenshots to `output/ui-validation-demand-ai-responsive.png` and `output/ui-validation-demand-ai-wide.png`.
- `pnpm build`, TypeScript compilation, and `git diff --check` passed; existing repository-wide Svelte warnings remain non-blocking.

## 2026-07-11 Phase 51 Complete

- Added an inline search field to both compact demand-association dropdown render paths in `Deconstructor.svelte`.
- Added real-time matching across demand ID, title, owner, repository, and project, plus a live filtered/total count and explicit no-results state.
- Reset the search query after selection, unbound selection, outside dismissal, and reopening so stale filters do not surprise the next interaction.
- Authenticated browser validation filtered `101` demands to `HR-4202`, selected the result, confirmed the menu closed and the value remained selected, and found no console errors or horizontal overflow.
- Saved and reviewed `output/ui-validation-demand-association-search.png`.
- `pnpm build`, TypeScript compilation, and `git diff --check` passed; existing repository-wide Svelte warnings remain non-blocking.
# Health Diagnosis Intervention Modal Calibration

- Added a dedicated modal scope and clearer title hierarchy without changing diagnosis data or close behavior.
- Reworked the intervention brief, diagnostic evidence, PHDI formula, project facts, metric rows, and recommendations into the light admin-console palette.
- Preserved red/amber/green only as low-saturation semantic states; removed glow, loud multi-color scores, and nested high-contrast surfaces.
- Added responsive one-column layout and compact mobile treatment while keeping scrolling inside the modal body.
- Validation: `pnpm build` passed; targeted `pnpm check` only surfaced the pre-existing `DemandKanban.svelte` union error and an existing unused selector warning; `git diff --check` passed.
- Desktop browser harness passed at 1280×720 with no page overflow and correct modal-owned scrolling. Dynamic narrow-viewport verification was unavailable because the browser facade has no viewport resize method.
# Task Panels Height And Internal Scroll Alignment

- Located the active personnel and Jira Task status layouts and their final light-admin CSS overrides.
- Confirmed this is a layout/overflow ownership defect, not a data rendering issue.
- Added scoped status/personnel modifiers, a shared responsive desktop height, equal-height grid tracks, fixed toolbar rows, and internal table/inspector scrolling.
- `pnpm build` passed with only existing Svelte warnings; `git diff --check` passed.
- Browser harness confirmed exact 480px/480px alignment for both workbenches and verified internal table scrolling with no page overflow.
# Compact Task Rows, Inspector Pills, And Filter Layering

- Located the exact status-row, inspector fact, and top filter dropdown markup in `TaskKanban.svelte`.
- Replaced visible task-type text with accessible SVG icons, compacted status rows to one primary line, converted inspector facts to wrapping capsules, and raised the owning filter surface above subsequent cards.
- `pnpm build` and `git diff --check` pass; build output contains only existing warnings outside this change.
- Browser harness verified row density, title width, icon accessibility, five capsule facts, and an unobstructed open dropdown over the metric grid.

## 2026-07-12 Phase 52 Start

- Applied the UI design-system and file-based planning workflows for the three-part frontend refinement.
- Reviewed the supplied schedule screenshot and recorded the inspector density, inactive global search, and segmented login-field issues.
- Preserved the existing dirty worktree and limited planned edits to the owning frontend components.
- Confirmed ownership: `DemandKanban.svelte` renders the inspector, `FunctionalAdminShell.svelte` owns the inert search input, and `App.svelte` owns the login field group.
- Implemented a real shell search flow with delayed input search, explicit submit, bounded results, empty/error states, and route-aware result actions.
- Replaced the inspector's ruled fact grid with wrapping pills and restyled the login email suffix as an intentional capsule inside one continuous focus surface.
- TypeScript, production build, and diff hygiene passed; the first build exposed two new search ARIA warnings, which were corrected with an explicit combobox contract and option selection state before browser review.
- Authenticated browser validation confirmed live search results and route navigation, eight compact inspector pills, no horizontal overflow, and no console errors at `2133x902`.
- Isolated unauthenticated validation confirmed the login prefix/suffix control at `1280x720` without changing or logging out the user's active Chrome session.
- Added result-ID handoff from the shell to schedule/task components so selecting a global result can clear conflicting filters and focus the exact record after route navigation.

## 2026-07-12 Phase 52 Complete

- Delivered the compact schedule-inspector pill layout, functional global search with exact result focus, and continuous login email field group.
- Final validation passed: TypeScript, production build, `git diff --check`, authenticated search/route/focus interaction at `2133x902`, and isolated login screenshot review at `1280x720`.
- Final browser regression selected non-first-row `HR-4202` in both the table and inspector with no console errors or document horizontal overflow.
# 2026-07-12 - Demand detail AI workbench and executable spec draft

- Loaded the repository cold-start modules and classified the work as `coding.complex`, medium risk.
- Applied UI skill order `ui-design-system -> finesse-ui -> design-taste-frontend`; selected product register, SOUL 4, SPECTACLE 2, DENSITY 7.
- Inspected the dirty tree and preserved existing uncommitted demand-modal, responsive AI, and broader project changes.
- Traced the demand spec lifecycle, deconstruction prompt/schema, delivery controls, and companion modal geometry.
- Confirmed three root causes: no draft-withdrawal API, standalone presentation defaults for detail/schedule, and incomplete AI implementation-spec output.
- Implemented draft-only withdrawal, expanded the AI implementation-spec schema, added the editable implementation blueprint, and unified create/schedule/detail companion ownership.
- TypeScript compile and `git diff --check` passed; focused Go tests require the repository-standard `/tmp` cache because the sandbox blocks the default macOS Go cache.

# 2026-07-12 - Streamed AI specification draft and Markdown preview

- Applied the repository `coding.complex` preset, file-based planning workflow, and UI design-system audit workflow.
- Reviewed the supplied screenshot and recorded the current textarea-matrix, nested-scroll, weak-hierarchy, and Markdown-flattening problems.
- Preserved the existing dirty worktree; planned changes are limited to the AI specification generation/transport/rendering path plus task memory.
- **Status:** discovery in progress
- Implemented the first backend/frontend slice: NDJSON events, provider token streaming, heartbeat frames, transaction-gated completion, persisted Markdown, semantic preview, explicit edit mode, and safe local Markdown rendering.
- The first combined validation command used the wrong working directory for Go paths; frontend production build still completed successfully with only existing repository warnings. Logged the error and split the next validation by package root.
- Added a provider-level SSE regression with deliberately split Markdown/JSON delimiters; its first run was blocked before execution by sandbox loopback restrictions, so focused approved-loopback validation is required.
- Corrected UTF-8 chunk boundaries after the SSE regression exposed multibyte Chinese characters being split by delimiter look-behind; the focused stream/spec lifecycle suite now passes.
- TypeScript, production build, focused Go lifecycle/stream/login tests, and diff hygiene pass.
- Browser validation reached the actual local login route at `127.0.0.1:5175`, but the long-running API does not allow the disposable local account. The login page is handed off for the user to authenticate; Markdown preview/edit and screenshot review remain pending.
- **Status:** implementation and automated validation complete; authenticated visual acceptance pending
- Authenticated browser validation resumed on the real DG-319 flow-board detail modal at 1280×720. The preview has 14 semantic headings, 6 lists, zero textareas, no horizontal document overflow, and modal-owned vertical scrolling.
- Screenshot review found underscore-style Markdown emphasis leaking as literal syntax; added safe `_text_` rendering before continuing visual acceptance.
- Rechecked the live preview after the emphasis fix: `_暂无内容_` no longer appears literally and eight `<em>` nodes render safely.
- Verified the Preview/Edit switch against DG-319 without saving or mutating business data; edit mode exposes every governed field and preview mode restores the semantic document.
- Authenticated screenshots passed at 1280×720 desktop and 760×900 narrow viewport with no document horizontal overflow or browser console errors.
- Final focused Go stream/spec/login tests, TypeScript compilation, production build, and diff hygiene pass. Existing unrelated Svelte warnings remain unchanged.
- **Status:** complete

# 2026-07-12 - Impeccable review gate and shared Markdown workbench

- Installed Impeccable 3.9.1 into `~/.agents/skills/impeccable` and `./.agents/skills/impeccable`; scoped installer output contained no repository metadata, sidecars, or cache files to clean.
- Replaced the old frontend skill fallback with a mandatory Impeccable + taste-skill + finesse-ui review gate in `AGENTS.md`, `.agents/domains/coding.yaml`, `DESIGN.md`, and project routing memory.
- Added the shared CodeMirror/GFM `MarkdownWorkbench.svelte`, migrated live AI output and persisted specification editing, and removed the page-local Markdown renderer.
- Flattened the delivery surface to spacing and hairlines while keeping one bordered editor workbench and collapsing structured execution fields behind progressive disclosure.
- Corrected the first narrow split-height defect discovered during authenticated browser QA.
- Final validation passed: `pnpm check` (0 errors), `pnpm build`, `git diff --check`, Impeccable detector (`[]`), desktop edit/split/preview interaction, 760x900 responsive checks, and zero browser console errors.
- **Status:** complete

## 2026-07-14 - Phase 65 governed corpus ingestion and review center

- Loaded the mandatory project-local Impeccable skill, design-taste-frontend, finesse-ui, and file-planning guidance.
- Classified the surface as a dense product-register Settings workbench and locked a restrained, low-motion, high-density design read.
- Started a narrow audit of the existing corpus promotion seam, LLM path, source persistence, permissions, and shared Markdown editor before any frontend edit.
- Confirmed that the repository already migrates `ContextDocument`, links facts to documents, filters production context to active facts, and promotes accepted candidates transactionally. The implementation can extend these seams instead of replacing them.
- Audited the current AIConfig, candidate queue, and shared Markdown workbench. The target composition is source-first library, one selected-candidate review workbench, and advanced manual fact editing within the existing three-task corpus shell.
- Completed and recorded the mandatory Impeccable, design-taste-frontend, and finesse-ui review before any frontend edit. Shared direction and disagreements are now locked in the active Phase 65 plan.
- Added the source-document model extensions, import/list/archive routes, LLM extraction, review-mode classifier, impact preview, and final publish endpoint. Legacy focused tests pass; the new mock-provider test needs local-loopback permission because sandboxed `httptest` cannot bind `[::1]`.
- **Status:** in progress

# 2026-07-13 - Reviewer multi-select consecutive selection correction

- Restored the expected multi-select session: reviewer selection, option deselection, and expanded-state chip removal now keep the option list open.
- Preserved capture-phase outside-pointer, focus-exit, Escape, explicit completion, and chevron dismissal; updated reviewer guidance to describe consecutive selection and outside-click completion.
- Added a pointerdown focus guard for chip removal after authenticated testing exposed that deleting the focused remove button otherwise triggered focus-exit closure.
- Authenticated DG-319 testing covered desktop and narrow layouts, consecutive selection, outside dismissal, option deselection, chip removal, Escape, `完成选择`, and chevron close with no browser console errors or horizontal overflow.
- Restored all temporary reviewer values before releasing the browser tab; no save or approval action was triggered.
- Final Svelte check passes with 0 errors and 78 existing warnings; production build, targeted Impeccable detection, and diff hygiene pass.
- **Status:** complete

# 2026-07-12 - Inline Markdown editing and review contract layout

- Completed the mandatory Impeccable + taste-skill + finesse-ui review, including isolated layout assessment and mechanical scan.
- Removed visible Markdown mode/save controls from the demand specification while preserving the shared workbench's reusable API.
- Added double-click editing, outside-click temporary persistence, failure-safe edit retention, and a focus-only keyboard entry.
- Rebuilt the review contract into two flat semantic groups with responsive one-column fallback and restrained summary metadata.
- Unified demand-detail and AI companion width ownership at 960px desktop maximum.
- Final validation passed: Svelte check with 0 errors, production build, targeted and layout-scope Impeccable scans (`[]`), `git diff --check`, authenticated DG-319 double-click/save/exit flow, 1280x720 and 760x900 layout metrics, screenshots, and zero browser console errors.
- **Status:** complete

# 2026-07-12 - Flat review contract and directory-backed reviewers

- Removed the duplicate “结构化执行字段” presentation while retaining the governed payload compatibility behind the Markdown document.
- Added the shared `MultiSelect.svelte` primitive with inline search, removable selections, keyboard handling, adaptive placement, and accessible multi-select semantics.
- Rebuilt the review contract as one flat responsive form: multi-select reviewers/roles, shared searchable single-select owners, compact approval count, and a restrained segregation note.
- Stopped preloading Jira `sync_users` as reviewers; default candidates now come only from implementation-task assignees and exclude the specification author.
- Added a permission-scoped participant directory to the review-contract response and retained the already-loaded page directory as a development/backward-compatible source.
- Final validation passed: focused Go tests, Svelte check with 0 errors, production build, diff hygiene, targeted/full layout Impeccable scans (`[]`), and authenticated DG-319 search/multi-select/default-owner/screenshot review.
- **Status:** complete
## 2026-07-13 — Phase 59 select lifecycle correction

- Confirmed the live Vite module was current, so the remaining report was an interaction-contract bug rather than stale frontend output.
- Kept `Select.svelte` and `MultiSelect.svelte` as typed shared primitives and extracted their duplicated outside-pointer/Escape handling into `web/src/components/shared/selectLifecycle.ts`.
- Changed multi reviewer selection and deselection to close immediately and restore focus; removed focus-triggered opening that could move the in-flow list underneath the originating click.
- Updated reviewer helper copy to explain the close/reopen workflow.
- Actual-component browser checks covered selection, deselection, chevron, outside pointer, and Escape for both select modes. Svelte check, production build, complete/layout Impeccable scans, and diff hygiene pass.

## 2026-07-13 — Phase 63 configuration center unified glass workbench

- Removed the Settings-only KPI route and submenu while preserving the top-level KPI page and KPI permission/policy resources.
- Added one shared Settings route registry and routed App access/fallback, shell navigation, and SettingsPanel metadata through it.
- Rebuilt the page context, primary workbench, audit inspector, configuration forms/tables, security surfaces, and modals around one restrained glass-and-hairline system with reduced-motion and reduced-transparency fallbacks.
- Normalized GitLab and the existing shared configuration workbenches without changing their fetch, save, test, rollback, permission, or mutation contracts.
- Authenticated browser QA covered all 10 Settings pages, GitLab overview/edit state, and 2133x902, 1024x900, 760x900, and 480x900 responsive layouts. Fixed the mobile switch inflation found during the first 480px review.
- Final validation passes: TypeScript, Svelte check with 0 errors, production build, targeted Impeccable detection, diff hygiene, and zero browser console errors.
- **Status:** complete

## 2026-07-13 — Phase 64 configuration alignment and dense-page normalization

- Completed the mandatory Impeccable + taste-skill + finesse-ui review and recorded the progressive-disclosure decision before frontend edits.
- Reassigned Settings geometry to one stable unified root so legacy Phase height, stretch, absolute-position, and overflow rules cannot make the audit rail or primary panel abnormally tall.
- Rebuilt policy authorization as four task tabs and system corpus as three task tabs while preserving every existing field, API, permission, result, and mutation path.
- Bounded policy tables, policy action vocabularies, audit history, and corpus candidate presentation; candidate review now uses the shared visual tokens and ten-item pagination.
- TypeScript, Svelte check (0 errors, 78 existing warnings), production build, Impeccable detection (`[]`), and diff hygiene pass. Authenticated browser validation is in progress; the first 2133x902 policy pass has zero horizontal overflow and no document vertical overflow.
- Authenticated browser validation now covers all 10 routes plus every corpus/policy task at desktop and exact 1024/760/480 viewports. GitLab's responsive audit rail is capped to 680/700px, long members/permission/audit datasets are bounded, and every measured route has zero document horizontal overflow.
- A fresh authenticated tab verified the runtime-null memberships guard across permission and member pages with zero console errors. No mutation was submitted.
- Final validation passes: TypeScript, Svelte check with 0 errors and 78 existing warnings, production build, targeted Impeccable detection (`[]`), and `git diff --check`.
- **Status:** complete

## 2026-07-14 - Phase 65 governed corpus ingestion and unified review center complete

- Added versioned raw-document persistence, list/detail/import/archive APIs, Markdown/text normalization, configured-LLM extraction, provenance labels, failure retention, and candidate/document publication counts.
- Extended corpus governance with editable pending candidates, ordinary immediate publication, high-sensitive/global/AI-suggestion impact staging, recorded preview actor/time, preview invalidation on re-review, and immutable final publication of the previewed candidate.
- Rebuilt the corpus route around raw-document management, shared Markdown import/preview, advanced manual fact maintenance, a unified queue/workbench review center, and side-by-side current-versus-proposed impact review.
- Added regression coverage for import extraction, pending exclusion, ordinary publication, pending rejection, impact bypass rejection, recorded preview, final publication, source linkage, document counts, and archive-time withdrawal of unpublished candidates.
- Final checks pass: `go test ./... -count=1`, `pnpm -C web check` (0 errors, 78 existing warnings), `pnpm -C web build`, Impeccable detector `[]`, Finesse detector with zero findings, and target diff hygiene.
- Authenticated fixture-browser validation passed the complete workflow at 1440/1024/760/480 widths with zero horizontal overflow and zero console errors; validation evidence is saved under `output/ui-validation-corpus-*.png`.
- **Status:** complete
## 2026-07-14 - Phase 66 opaque file handoff correction started

- Re-loaded the required project Impeccable, design-taste-frontend, finesse-ui, OpenAI docs, planning, and self-correction guidance.
- Confirmed the current violation on both sides of the boundary: browser `File.text()` plus backend persisted/prompt-concatenated file-derived content.
- Recorded the three-way UI direction, provider/file ownership, persistence boundary, protection rules, validation scope, and the superseded Phase 65 normalization decision in `task_plan.md` before frontend edits.
- The current phase will keep direct Markdown paste as a separate advanced text path while making uploaded files opaque end-to-end.
- The first focused Go test attempt was blocked by the default user cache permissions; TypeScript passed. Validation now uses isolated `/tmp` Go caches instead of repeating the failing command.

## 2026-07-14 - Phase 66 opaque file handoff correction complete

- Added provider-owned opaque file inputs in `internal/llm`: Responses emits `input_file` with filename and inline Base64 data; native Messages returns an explicit unsupported-protocol error instead of parsing locally.
- Replaced file-derived JSON content with multipart transport. The server now limits, validates, hashes, and forwards raw bytes but never decodes or concatenates them into the prompt.
- Changed managed document persistence to use only LLM-returned `document_markdown`, summary, and candidates. Failed uploads retain metadata/hash/failure status with an empty document body, and duplicate completed uploads reuse the existing extraction.
- Reworked the source workbench into mutually exclusive file and Markdown modes. File mode keeps only the selected `File` and transport metadata; Markdown mode reuses `MarkdownWorkbench` for intentional human text.
- Added regression coverage for exact opaque-byte transport, prompt isolation, unsupported protocols, multipart success/missing/oversize/failure/deduplication, LLM-only persistence, and manual Markdown compatibility.
- Final automated checks pass: `go test ./... -count=1`, TypeScript, `pnpm -C web check` with 0 errors, production build, Impeccable detection `[]`, and `git diff --check`.
- Isolated authenticated Chrome validation passed empty, file-selected, submitting, success, failure, Markdown-edit, and Markdown-preview states at 1440/1024/760/480. All widths had zero horizontal overflow; desktop columns shared the same top edge; narrow controls measured 44px; the intentional 502 fixture was the only expected network error and there were zero unexpected console/page errors.
- Reviewed screenshots saved as `output/ui-validation-corpus-opaque-file-desktop.png`, `output/ui-validation-corpus-opaque-file-1024.png`, `output/ui-validation-corpus-opaque-file-760.png`, and `output/ui-validation-corpus-opaque-markdown-480.png`.
- Finesse pre-flight self-grade: the surface remains product-register, preserves one teal accent and the existing type/radius system, introduces no banned decorative patterns, uses explicit action copy, keeps labels above inputs and 44px narrow controls, and honestly claims no spectacle engine.
- **Status:** complete

## 2026-07-14 - Phase 69 transient corpus decision toasts complete

- Completed the mandatory project-local Impeccable, design-taste-frontend, and finesse-ui review before frontend edits. The shared decision is global transient success plus contextual recoverable errors, with product-register styling and feedback-only motion.
- Added `web/src/lib/toast.ts` for a bounded three-notice queue, 4.5-second default dismissal, timer cleanup, and manual dismissal.
- Added `web/src/components/shared/ToastHost.svelte`, reusing `Alert.svelte` for the existing visual vocabulary and adding upper-right safe-area placement, reduced-motion handling, reduced-transparency fallback, responsive width, and 44px narrow close targets.
- Mounted one host in the production `App.svelte` tree and one in the isolated settings preview tree; updated the shared alert close-label contract for accessible Chinese copy.
- Removed the candidate review's local success state, in-container success markup, and success CSS. Reject, ordinary publish, impact-stage approval, and final impact publish now publish to the shared toast queue; contextual errors remain unchanged.
- Final automated validation passes: `pnpm -C web check` with zero errors and existing unrelated warnings, production build, targeted Impeccable detector `[]`, and target `git diff --check`.
- Isolated authenticated browser validation covered ordinary publish, exact final impact-publish copy, automatic dismissal, manual dismissal, contextual 503 error, and 1440/1024/760/480 layouts. Every viewport had zero horizontal overflow and the browser console had zero warnings/errors.
- **Status:** complete

## 2026-07-15 - Phase 70 impact review alignment complete

- Loaded the mandatory project-local Impeccable guidance and completed the design-taste-frontend plus finesse-ui product/redesign review before frontend edits.
- Recorded the shared direction, the superseded Phase 68 side-by-side decision, protection rules, responsive contract, validation scope, and correction log in `task_plan.md`.
- Converted the impact comparison to one vertical reading flow and introduced a shared desktop panel height with explicit queue-list/workbench-body scrolling. Narrow layouts keep natural height.
- `pnpm -C web check` passes with 0 errors and 78 existing unrelated warnings; the production Vite build passes.
- Targeted Impeccable detection returns `[]`, finesse detection returns zero findings, and target diff hygiene passes.
- At the 1440px impact-review pass, the queue and workbench both measure 670px high with identical y=179 and bottom=849; their top and bottom deltas are exactly 0px.
- The impact comparison computes to one 950px column. “当前生效上下文” and “拟发布结果” share x=414 and appear at y=430 and y=830 respectively, confirming the ordered vertical flow.
- The queue list is the left scroll owner (`625/701px`) and the workbench body is the right scroll owner (`530/1150px`); the header and 61px action row remain outside the body scroll. Document horizontal overflow is zero.
- The browser's 1024 override produced a fresh 1269x720 content viewport. At that medium desktop size, impact queue/workbench both measure 520px high with 0px top/bottom delta, comparison remains one 790px column, and document overflow stays zero.
- Visual screenshot review confirms two aligned outer bordered panels, a stable right action footer, and only the current-context comparison leg visible at the top of the right scroll body; the proposed result follows below rather than competing beside it.
- Switching back to an ordinary candidate keeps the same 520px outer heights and 0px top/bottom delta, renders exactly one proposal region and zero comparison panes, and preserves right-body scrolling without changing the workflow.
- Chrome's exact 760 override reports a 749x900 content viewport. The queue and workbench stack at the same x=35, the queue returns to a natural 366px block with a 320px local list, and the workbench returns to natural 1500px height with `overflow-y: visible`.
- The two impact panes remain one 649px column at y=970 and y=1370, document overflow is zero, and all three terminal action buttons measure 44px. Screenshot review confirms a clean queue-first/workbench-second vertical composition.
- At exact 480x900, the queue and workbench stack at x=27 with equal 415px widths; both impact panes share x=42 and 385px width, the three action buttons measure 44x385px, source evidence expands in place, and document horizontal overflow remains zero.
- At exact 1024x900, the desktop contract is active: queue and workbench both start at y=564 and end at y=1204 with identical 640px heights. Queue-list and workbench-body own their respective scrolling, both impact panes share x=414 and 534px width, and document horizontal overflow is zero.
- Authenticated browser coverage now includes impact and ordinary candidates, desktop and stacked layouts, source disclosure, scroll ownership, action sizing, and zero unexpected console warnings/errors. The correction is validated against actual rectangles rather than CSS declarations alone.
- Self-review lesson: distinguish the valid outer queue/detail relationship from a redundant inner comparison grid, then verify both information order and rendered top/bottom geometry at the affected breakpoints.
- **Status:** complete

## 2026-07-15 - Phase 71 workspace-bounded toast positioning complete

- Reloaded the mandatory project-local Impeccable, design-taste-frontend, finesse-ui, planning-with-files, and self-correction guidance before frontend edits.
- Completed isolated layout judgment and mechanical pre-scan. Both confirm that the toast queue/store is valid while the viewport-root host and global fixed layer are not.
- Recorded the shared component ownership, stage geometry, z-index, responsive, protection, and browser validation contracts in `task_plan.md` before implementation.
- Removed the unconditional root host from `App.svelte`, mounted the production host inside a new `FunctionalAdminShell.workspace-stage`, and converted `ToastHost` from viewport-fixed/global-1400 to stage-absolute/local-10 with stage-relative width and max height.
- Updated the isolated settings preview to use the same fixed-header, bounded-stage, internally scrolling content contract so test toasts remain visible when the review workbench is scrolled.
- `pnpm -C web check` passes with 0 errors and the same 78 existing warnings; the production Vite build passes. Impeccable layout detection returns `[]`; targeted ToastHost/preview Impeccable and Finesse scans are clean. Broader scans only report known legacy App/Shell findings outside this positioning change.
- Browser shell geometry at 1280x720: Header is y=0..68, workspace stage is y=68..720, and the empty host anchor is y=80, exactly 12px below the stage start. Header/toast and profile/toast intersection areas are both zero; document overflow is zero.
- A real ordinary-candidate publish in the isolated preview rendered the 58px toast at y=142, exactly 12px inside the y=130..688 content stage. Topbar/toast intersection and page overflow are zero, with 488px remaining below the notice.
- At exact 760x900, the production shell Header is y=0..64 and the stage is y=64..900. The host anchor begins 8px inside the stage, spans only the available workspace width, and has zero Header/profile intersection and zero document overflow.
- A real 760px publish renders a 70px toast at y=138 inside the y=130..868 stage, leaving 660px below it. The notice uses the available 708px stage width and the close target is 44x44px.
- At exact 480x900, a real publish renders the notice at y=153 inside the y=145..890 stage with 667px remaining below, zero topbar intersection, zero page/document overflow, and a 44x44px close target.
- The 480px toast auto-dismisses after the existing timer and manual close removes a fresh notice immediately. Collapsing the desktop rail from 270px to 94px leaves the Header/stage boundary and 12px toast inset intact with zero Header/profile intersection.
- Browser geometry and screenshots show the notice belongs to the review workspace rather than the Header. The selected browser surface did not expose a console-log API, so no new console-log claim is made; every exercised interaction completed without a page-control exception after the documented harness corrections.
- Final `pnpm -C web check` passes with 0 errors and 78 existing warnings; production build, targeted Impeccable, Impeccable layout scan, targeted Finesse, and target `git diff --check` all pass.
- Self-reflection: the prior implementation correctly separated transient success from panel content but chose the browser viewport as the overlay owner. Future shell-level overlays must first identify the intended visual boundary and establish their containing block there before selecting `fixed`, `absolute`, or z-index values.
- **Status:** complete
# 2026-07-18 - Backend LLM deconstruction availability audit complete

- Confirmed effective configuration: AI enabled, token present and redacted, Pixel provider, `gpt-5.5`, Responses API.
- Confirmed real gateway health through the authenticated configuration center.
- Completed one non-synced synthetic pre-deconstruction and received three structured tasks, 76% completeness, 72% confidence, and a 16-hour overall estimate.
- Closed the AI companion and cancelled the host new-demand form; no actual demand or task sync was submitted.
- Focused Go configuration, provider-client, deconstruction, attachment-stream, and demand-spec stream tests pass.
- Residual configuration debt: the database snapshot still stores legacy `endpoint_type: completions`; runtime normalization makes it safe today, but a future explicit save should persist `responses`.

# 2026-07-18 - Daily Jira timing badge clarification complete

- Completed the mandatory Impeccable, design-taste-frontend, and finesse-ui review before editing the Daily Jira surface.
- Renamed the first table column and related empty/unclassified copy from age wording to timing wording without changing backend cohorts or reminder policy.
- Replaced the wrapped age/overdue stack with one accessible, non-wrapping dot badge: red overdue, amber due within three natural days, and green healthy, followed by factual creation age.
- `pnpm check` passes with 0 errors and 79 existing warnings; production build, Impeccable layout detection, Finesse detection, and `git diff --check` pass.
- Real-component browser validation passed at 1280/760/480 with all semantic states, today/multi-day copy, keyboard row selection, zero browser-level overflow, a fixed 16px bottom gap, and no console errors. The isolated read-only fixture and local validation server were removed after use.
- **Status:** complete

# 2026-07-20 - Per-user project preferences complete

- Added per-user project preference persistence plus authenticated `GET`/`PUT /api/me/project-preferences`; no saved rows means all projects, while selected mode stores normalized project keys.
- Applied the preference scope to demand, Jira/task, schedule, risk, execution, agenda, strongest-brain, notification, and deconstruction/archive read paths, including direct-record access guards.
- Added an inline preference editor to the profile popover with shared multi-select behavior, explicit all/selected modes, empty-selection protection, and page refresh events for the affected demand/Jira views.
- Full Go suites pass for `internal/db`, `internal/server`, and `internal/agenda`; `pnpm --dir web check` passes with 0 errors and 72 pre-existing warnings; the production web build passes; exact affected-file Impeccable detection returns `[]`; `git diff --check` passes.
- Authenticated isolated browser validation covered default-all, multi-project save, filtered agenda and Daily Jira results, restore-all, and 480px responsive behavior. Agenda results changed from 150 to 6 for HIT + NS2, and every visible Daily Jira row used HIT or NS2.
- Fixed the mobile profile popover discovered during validation; at 480px it now stays within the viewport at x=12..468. The test user was restored to all projects, isolated services were stopped, and no demand/Jira record was created or synchronized.
- **Status:** complete

# 2026-07-20 - Daily Jira bottom ownership and Jira version-link audit complete

- Removed Daily Jira's duplicate viewport-height calculation and connected the component to the same shell/workspace height chain used by Decision Agenda.
- Authenticated 2048x925 comparison measured both routes at the same 22px bottom gutter; 1024x900 and 760x900 preserved 14px and 10px responsive gutters with zero document overflow.
- The Jira release page for `PRJ25024 / 13622` exposes 21 issues through `project = 11900 AND fixVersion = 13622`. Existing `custom_jql` plus `SearchIssues` can ingest that issue set, but the application does not currently parse release-page URLs automatically.
- No Jira sync or business-record mutation was performed. Frontend check/build, diff hygiene, exact-file Impeccable/Finesse scans, and authenticated console validation pass.
- **Status:** complete

# 2026-07-20 - Jira version sources and decision bottom substrate complete

- Added typed Jira version sources with project number, project name, and version-page URL; server-side normalization enforces the configured Jira origin/base path, numeric version ID, project-key agreement, uniqueness, and canonical storage.
- Jira synchronization now unions ordinary/custom JQL with version clauses such as `(project = "PRJ25024" AND fixVersion = 13622)` and includes valid version projects in keep-alive scope and project catalogs.
- Added the responsive Settings editor with add/remove rows, immediate parsing, parsed-key adoption, invalid/duplicate feedback, overview, and confirmation summary. Existing Jira connection and custom-JQL behavior remains intact, with release sources explicitly additive.
- Removed only the decision route's shell bottom inset. Decision Agenda and Daily Jira cards now meet the browser bottom consistently without painting over business surfaces or changing unrelated page gutters.
- Go config/telemetry and focused server tests pass; frontend check/build, Impeccable complete/layout scans, Finesse P0 gate, and `git diff --check` pass.
- Authenticated isolated browser validation passed the supplied version URL and both decision pages at 2048x924 and 760x900 with zero horizontal overflow and no console errors. No save, Jira sync, or business-data mutation was performed; all temporary services were stopped.
- **Status:** complete

# 2026-07-29 - Project board, Jira reverse sync, and decision table complete

- Added the Schedule Governance project board with saved-project scope, priority fallback, a horizontally scrollable selector, and Todo/In Progress/Done lanes with expandable details.
- Added Jira due-date reverse synchronization after successful schedule or decision rescheduling and Jira comment creation from non-empty decision conclusions.
- Added per-user persisted decision-table columns, consistent Bug/Task markers, and uniform status-cell row backgrounds.
- Go package and focused regression tests pass. Frontend check has 0 errors and 73 existing warnings; production build, exact-file Impeccable/Finesse detection, and diff hygiene pass.
- Authenticated isolated browser validation passed at 1440×900 and 390×844, including project switching, lane recomputation, detail expansion, preference persistence across reload, responsive scroll ownership, 44px narrow controls, matching status backgrounds, and an empty console error list.
- Jira, GitLab, and AI writes were disabled during validation. No production data was changed; temporary services and database files were removed.
- **Status:** complete

# 2026-07-30 - Delivery domain convergence implementation complete

- Froze the delivery domain vocabulary and SQLite baseline before adding project, release, release-link, event, sync-operation, outbox, parent-work-item, and revision facts.
- Added a single delivery-planning service and routed new work-item planning APIs plus the legacy schedule mutation adapter through its project/release gates, CAS checks, audit reason, atomic event, and outbox rules.
- Added Jira release reconciliation, ambiguity handling, manual-primary preservation, migration dry-run tooling, compatibility counters, and `/api/delivery/quality` deletion-gate metrics.
- Added the version-planning workbench, kept Project Board read-only, separated its card detail action from a persistent Jira anchor, exposed owners in collapsed cards, and restricted Task Tracking to execution tasks with optional parent context.
- Full Go tests pass; frontend check has zero errors; production build, diff hygiene, and exact-file Impeccable detection pass. Finesse has zero P0 findings and no findings in the two new workbench surfaces.
- Authenticated isolated browser validation passed Project Board pointer/keyboard/Jira/owner behavior at desktop and narrow widths, Version Plan selection stability and internal scrolling, and execution-only Task Tracking. No real Jira navigation or business mutation was performed.
- Phase 8 physical deletion remains gated because the compatibility counters have not yet observed one stable release cycle with zero calls. The compatibility routes are marked deprecated and measurable, but not unsafely removed.
- **Status:** implementation complete; compatibility deletion pending observation gate
# 2026-07-31 - Version Plan multi-Jira association complete

- Fixed the empty “all projects” Select clear affordance and portaled shared dropdowns to the body so modal/inspector overflow no longer clips long project options.
- Routed version creation success and failure through the upper-right Toast while retaining field-local validation.
- Replaced Jira release-version selection with project-scoped Jira work-item search, atomic multi-select association, linked-item listing, and individual removal.
- Added release Jira-item counts and Project Board primary-release projection without per-row queries; linked Jira cards now show the local version name.
- Kept `work_item_release_links` as the single relationship source, added candidate/count/projection indexes, blocked conflicting project changes, and prevented local associations from producing Jira fixVersion outbox writes.
- Added the local release ownership ADR and a database schema/index/query design document. Legacy `release_jira_links` remains compatibility-only.
- Full `go test ./...` passed. `pnpm check` passed with 0 errors and 82 pre-existing warnings; production build passed; exact affected-file Impeccable detection returned `[]`.
- Authenticated isolated browser validation passed the desktop workflow and 744px responsive state, including both creation Toast paths, long dropdown options, two-item batch association, linked-item removal surface, and Project Board version badges. No production or real Jira data was changed.
- **Status:** complete

# 2026-07-31 - Version Plan batch-selection height stabilization complete

- Reproduced the exact cumulative-growth defect in the authenticated Version Plan inspector: 12 selected Jira items grew the selector from 38px to 422px and the inspector scroll content from 556px to 940px; one long item already grew the trigger to 70px.
- Enabled the shared MultiSelect summary presentation only for the batch Jira call site. The selector now retains a fixed control height and displays a count such as “已选 30 项”, while the overlay continues to own complete candidate display, search, clear, and continuous selection.
- Authenticated browser validation passed at 1440×900 and 390×900. Selecting 30 of 48 candidates kept the trigger at 36px, preserved inspector height, kept the overlay inside the viewport, produced no horizontal overflow, and left browser error logs empty.
- `pnpm check` passes with 0 errors and 79 existing warnings; production build, exact DeliveryPlan Impeccable/Finesse detection, and targeted whitespace checks pass.
- Validation used the isolated local database with Jira, GitLab, Feishu, and AI disabled. No batch-association request or production write was submitted.
- **Status:** complete

# 2026-07-31 - Schedule inspector and task-directory convergence complete

- Rebalanced the Schedule Board into a fluid table plus a 420–560px content-owned inspector, with independent body scrolling, clearer fact/edit/action/risk hierarchy, complete tab keyboard semantics, and responsive 3+2 / 2+2+1 layouts.
- Made execution assignee facets merge the authorized user directory, configured members, and visible local-task owners. Jira aliases now resolve through the shared identity directory, while unmapped local owners remain available.
- Kept project facets on the authoritative user-scoped project catalog. Task Table, Execution Tracking, and Personnel Load now expose identical project and owner choices in the authenticated browser fixture.
- Full Go tests passed. Frontend check passed with 0 errors and 79 existing warnings; production build, exact whitespace checks, Impeccable detection, and the Finesse P0 gate passed.
- Authenticated browser validation passed at 1440×900, 1024×900, 760×900, and 390×900 with internal table/inspector scrolling, no overlap or horizontal overflow, stable multi-select geometry, keyboard tab switching, and an empty clean-page console log.
- Validation used integrations-disabled isolated SQLite data. No schedule save or production Jira/database write was performed.
- **Status:** complete

# 2026-07-31 - Task Table Work Item convergence complete

- Rewired global demand/Bug search to Work Items and made Enter select the exact ID/title match or the first ranked result before navigating and scrolling to the highlighted row.
- Restored the status Task Table to the Work Item read model with correct project/owner facets while retaining Execution Tasks for execution tracking and personnel load.
- Removed inspector clamping/hidden-list behavior, added independent scrolling, and rebuilt task detail with the same shared `wide` Modal hierarchy used by Decision Dashboard.
- Authenticated fixture browser validation passed at 1280×720 and 390×844 with correct Work Item/Git boundaries, filters, long-content reachability, responsive modal geometry, zero document overflow, and no console errors.
- Frontend check, production build, targeted Impeccable/Finesse detection, static contracts, and diff hygiene pass. Validation was local and read-only.
- **Status:** complete

# 2026-07-31 - Task Table core-member candidate boundary complete

- Replaced Work Item-derived owner options with the shared core-member directory while preserving real owner facts in table rows.
- Aligned `/api/demands/options` with the central member rule: configured `sync_users` wins and custom-JQL assignees are fallback-only.
- Added backend regressions for primary/fallback behavior and a frontend source contract preventing future Work Item-derived candidate leakage.
- Isolated authenticated browser validation kept an external assignee visible in its row while exposing only the configured core member in the owner dropdown.
- Internal server tests, frontend check/build, Impeccable/Finesse gates, and diff hygiene pass; no external integration or business write was used.
- **Status:** complete

# 2026-07-31 - Shared delivery owner and project directory complete

- Corrected the three-person Task Table regression by separating RBAC authorization from delivery ownership. One authenticated `/api/delivery/directory` now returns configured Jira core members and the authoritative Jira/version-source project catalog.
- Migrated Decision Dashboard, Task Table, Execution Tracking, Demand/Schedule forms, Daily Jira reassignment, Version Plan, Deconstructor, and Project Config mapping to the shared directory. Project Preferences was confirmed to already use the same backend catalog.
- Added regressions proving RBAC-only users, visible local execution owners, task IDs, repositories, and commit evidence cannot pollute reusable owner/project candidates.
- Complete server/database tests, frontend check/build, Impeccable detection, Finesse P0 gate, and diff hygiene pass.
- Authenticated isolated browser validation confirmed matching 10-person and 4-project sets across the main delivery pages. Desktop and 390px Task Table checks showed an unclipped 10-option menu, zero document overflow, a 44px narrow trigger, and an error-free fresh task route.
- Fixed an additional empty-project-config `null` payload crash found during the browser audit; external integrations remained disabled and the isolated services/database were removed.
- **Status:** complete

# 2026-07-31 - FZ-2247 dual-repository realtime trajectory repair complete

- Canonicalized Jira issue identity across GitLab evidence so lowercase parser output and historical lowercase logs resolve to the existing `FZ-2247` Work Item instead of creating or hiding a Git-derived duplicate.
- Scanned every commit in a push batch, preserved the canonical Jira owner when Git evidence arrives, and kept both `task_executor` and `crane_manager` evidence under one trajectory even when only a non-final commit carries the Jira key.
- Added task-scoped SSE telemetry updates and open-panel background refresh with stale-request protection and no list flicker.
- Full Go tests, frontend check/build, detector gates, whitespace checks, and authenticated browser validation passed. The live panel advanced from PUSH 3 to PUSH 4 without manual refresh and rendered both repositories at desktop, tablet, and mobile widths without overflow.
- Production/runtime evidence still shows no `crane_manager` webhook delivery. No external GitLab mutation or deployment was performed; webhook installation/status remains the required environment follow-up.
- **Status:** local repair and validation complete
