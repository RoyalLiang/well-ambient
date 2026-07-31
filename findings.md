# Findings & Decisions

## 2026-07-18 - Daily Jira Decision Reminders And Viewport Fit

- The existing `DecisionEvent` already provides an immutable early-meeting audit ledger, but it does not carry a reminder deadline. A dedicated `DailyJiraDecision` record now stores the operational status, assignee, actor, note, decision time, and reminder time while leaving the event ledger unchanged.
- Reminder policy is deliberately low-friction: `escalate` becomes due after 4 hours; `follow_up` and `reassign` become due after 24 hours. Notification computation selects the newest record per Jira, suppresses future and resolved items, and naturally supersedes older decisions.
- Due reviews enter the existing notification SSE as `daily_jira_reminder`, use the decision record ID for a stable per-user dismissal key, and carry the last decision note so the follow-up has context.
- The GET audit projection exposes only the latest operational decision alongside the existing event history. The table shows Jira state plus decision state, while the inspector separates current decision/reminder from the immutable retrospective list.
- The three age scopes, search, result count, last refresh, and refresh action now belong to the table header. Pointer selection works from every cell in a populated row; focusable ARIA grid rows support Enter and Space activation.
- Route height is measured from the component's rendered top to `window.innerHeight - 16px`. The root clips document growth, the table and inspector own their internal scrolling, and stacked layouts divide the same remaining height instead of expanding the browser page.
- Isolated authorized-fixture browser validation used the real `DailyJiraAudit.svelte` component and was deleted afterward. Whole-row click from the assignee cell selected `WA-719`; Enter selected `WA-720`; the 3-day header tab synchronized its row and inspector. No POST request or real Jira/decision data mutation occurred.
- Geometry at 1280x720, 1024x900, 760x1000, and 480x1000 consistently measured 16px beneath the route, zero document horizontal/vertical overflow, bounded pane scrolling, and no unintended table horizontal overflow. Narrow age/search controls and refresh all measure 44px.
- Focused Daily Jira tests and the complete `internal/server` suite pass, including persistence/reminder windows, latest-decision supersession, resolved-item suppression, and per-user reminder dismissal. Svelte check has 0 errors with the same 74 unrelated warnings; production build, `git diff --check`, Impeccable layout detection, and Finesse P0 detection pass.

## 2026-07-18 - Daily Jira Visual Unification

- Authenticated comparison at 2133x903 shows the current Daily Jira route is structurally sound and data-dense, but its floating heading, flush segmented strip, 10px flat panels, and zero-elevation inspector do not match the 16/20px matte workbench language used by the adjacent decision surface.
- At the medium breakpoint the layout stacks into a hard 560px table before the inspector. At the narrow breakpoint it retains a hard 520px table and horizontally scrollable 190px cohort cells; this delays the morning decision form and creates avoidable vertical travel.
- The table's 48px row density, semantic age/status chips, sticky header, row selection, and separate table/inspector scroll owners are correct and should be preserved.
- Three-way review agrees on a targeted preservation redesign: one compact command bar, three separate cohort controls, one aligned table/inspector workbench, consistent Phase 41 material/radii, and content-aware stacked heights.
- Finesse's grain/hero substrate and design-taste's generic Atlassian package guidance are rejected for this slice because the repository already has a committed Svelte product system. The shared Phase 41 tokens and `DecisionDashboard.svelte` are the authoritative visual reference.
- Impeccable's subjective layout assessment found hierarchy/rhythm issues while its mechanical layout detector returned no findings. No arbitrary Tailwind spacing or z-index utilities are present, confirming this is a visual-structure correction rather than a lint-style violation.
- Implementation keeps behavior in place and changes only `DailyJiraAudit.svelte`: 20px structural panels, 16px status controls, a compact command bar, a wider desktop inspector, inset table shell, and a 48px row-derived stacked height.
- Authenticated wide validation at 2133x903 measured a 1108px table panel and 596px inspector with identical 691px heights and the same 324px top edge. The table and inspector remain their own bounded scroll owners.
- At the medium state, a 3-row cohort produces a 340px table panel and places the inspector 16px below it instead of reserving 560px. At effective CSS widths of 750px and 478px, project/recent-activity columns are hidden, table width equals its scroll boundary, and document horizontal overflow is zero.
- Search validation filtered to `ICA-10903` and proved row, Jira link, and inspector title all refer to the same issue; clearing the input restored all three rows without changing business data.
- Final default-width smoke state reopened `每日 Jira`, selected `7 日及以上`, rendered 133 real rows with `ZPU-2769` selected, reported zero horizontal overflow, and had no console errors.
- Final checks pass: Svelte/TypeScript 0 errors, production build, tracked and untracked diff hygiene, Impeccable complete/layout detection, and Finesse P0 detection. The 74 warnings remain in six pre-existing files outside Daily Jira.

## 2026-07-18 - Daily Jira Morning Review

### Requirements
- Add `每日 Jira` as a submenu within the decision surface.
- Audit unresolved Jira for today, 3 days ago, and 7 days ago.
- Optimize the surface for rapid morning-meeting assignment and later review.
- Preserve the independent value of the existing agenda/manual-decision workflow.

### Initial Constraints
- Mandatory UI gate requires Impeccable, design-taste-frontend, and finesse-ui agreement before any frontend edit.
- Existing repository work is heavily dirty; changes must be narrow and preserve unrelated work.
- Prior project baseline is a light, flat, table/workbench-first admin console. Avoid a new hero/cockpit or metric-card surface.
- Exact Jira age semantics, data source, assignment mutation, decision persistence, route ownership, and permissions still require code-backed discovery.

### Research Findings
- `DESIGN.md` requires grouped submenus to live in the global left rail, not inside page content. The right content area should enter business content directly under the breadcrumb/header.
- The current production baseline is the Phase 41 light admin console: compact, table-first, restrained teal accent, flat facts, and one inspector rather than a cockpit/hero.
- `App.svelte` currently models `decision` as one top-level tab guarded by `decision:read`; `DecisionDashboard.svelte` owns Agenda summary data, Jira links, manual intervention, and the decision-event ledger.
- Existing APIs include `/api/agenda/summary`, `/api/agenda/decision`, `/api/strongest-brain/intervention`, and Jira link configuration. Jira synchronization persists local task telemetry and can push assignee overrides back to Jira.
- Navigation already supports submenus for schedule, tasks, and settings through one `NavSubItem` contract. Decision currently has no children or active decision-view state, so the smallest IA extension is a decision submenu with `决策事项` and `每日 Jira`, not a new top-level tab.
- App workspace state and breadcrumbs are view-aware for schedule/tasks; the same pattern can carry a `DecisionView` and preserve `decision:read` as the parent permission.
- `TaskTelemetry` contains stable Jira/task key, title, repo/project, assignee, status, issue type, original `TaskCreatedAt`, `LastUpdate`, due date, and `DecisionLogs`. These fields are sufficient for age audit and traceability without querying Jira live from the browser.
- Agenda currently marks unresolved items older than 7 days as critical and >48 hours without update as stale, but it is risk-derived and filters through the Agenda model. A daily Jira audit needs a dedicated complete unresolved-Jira read rather than reusing only agenda rows.
- Jira assignee overrides already have a synchronization path. The existing manual intervention API and agenda decision handler must be inspected to decide whether daily assignment can reuse one safely or needs a dedicated endpoint.
- The project has no `PRODUCT.md`; Impeccable treats this as a scoped existing-surface change, so the current code and `DESIGN.md` are the product context rather than a blocker.
- The frontend is Svelte 5 with no Atlaskit dependency. The existing semantic admin components/tokens are the committed system and must be reused; design-taste's Atlassian-system suggestion is advisory, not grounds for a framework migration.

### Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Start with route/data/action tracing before UI composition | Age buckets and assignment/history controls must reflect backend truth rather than a frontend-only read model. |
| Add `DecisionView = 'agenda' | 'daily_jira'` beneath the existing decision route | Preserves the `decision:read` parent permission and follows the shell's established submenu pattern without adding a new top-level product area. |
| Define non-overlapping natural-day cohorts: age 0, age 3-6, age 7+ | Matches the requested morning audit checkpoints, prevents duplicate rows across scopes, and avoids hour-boundary drift. Age 1-2 remains outside the requested checkpoints. |
| Add dedicated read/review endpoints under `/api/decision/daily-jira` | Backend truth should own Jira-key filtering, unresolved status, core-member visibility, age boundaries, mutation authorization, Jira sync, and event persistence. |
| Persist `follow_up`, `escalate`, and `reassign` outcomes as `DecisionEvent` plus legacy `DecisionLogs` | Supports structured retrospective history while keeping the existing decision dashboard ledger compatible. |
| Use one flat table/inspector workbench with a scope strip | Faster early-meeting scan and action than three repeated boards; keeps all counts visible without nested cards. |

### Mandatory UI Review Outcome
- Impeccable: global submenu, semantic table selection, inline inspector, complete interaction states, and desktop/1024/narrow authenticated validation.
- design-taste-frontend: preservation redesign, high density, no hero/KPI theater, existing Svelte tokens over Atlaskit migration.
- finesse-ui: product register with low spectacle/high density, feedback-only motion, skeleton/empty/error/success/read-only states.
- Shared ownership: shell navigation -> App view/breadcrumb -> Daily Jira feature state -> backend audit/write contract.

### Implementation Findings
- Added dedicated GET and POST daily Jira contracts with `decision:read` for viewing and the combined `decision:read` + `demands:write` gate for mutations.
- Reassignment updates local telemetry, keeps the Markdown kanban aligned, records both `DecisionEvent` and `DecisionLogs`, and invokes the existing Jira assignee synchronization when Jira is enabled.
- Follow-up and escalation decisions intentionally do not modify `LastUpdate`; reviewing an old Jira must not make its engineering activity look fresh.
- The frontend is isolated in `DailyJiraAudit.svelte`; existing `DecisionDashboard.svelte` remains untouched and retains the agenda/manual-intervention workflow.
- The active decision view follows the existing App/shell view contract and resets workspace scrolling through the shell's route key.
- Isolated authenticated desktop validation confirms the left rail exposes `决策事项 / 每日 Jira`, the active submenu and breadcrumb both switch to `每日 Jira`, and the page defaults to the most urgent non-empty `7 日及以上` cohort.
- The populated fixture proves six unresolved Jira across mutually exclusive today / 3-6 day / 7+ cohorts, a separate 1-2 day watch count, missing-created-time warning, selected-row inspector, write-authorized decision form, Jira deep link, and structured retrospective history without touching real Jira data.
- Browser QA found and fixed a search/detail mismatch: when filtering the active cohort, the inspector now follows the first visible Jira or clears when no match remains.
- Core-member visibility is enforced on both read and review paths. Unassigned Jira remains visible so the morning meeting can dispatch it, while external assignees and out-of-scope reassignment targets stay excluded.


## Current Task: Surface-Aware Text Contrast

- New user feedback establishes a hard surface rule: black/dark ink must not be inherited onto a dark background. Dark product surfaces use cool light primary text plus muted blue-gray secondary text; light surfaces keep the existing dark ink scale.
- The active full-height demand drawer is intentionally a light surface and should not be darkened. The risky path is the legacy `.modal-content` base, which declares `background: #0b1220` without declaring a compatible foreground before the later Phase 41 light override.
- Scope the correction to the modal contrast contract and validate rendered affected states. A repository-wide dark-theme rewrite would touch unrelated legacy panels and is not justified by this feedback.
- Authenticated computed-style inspection identified the visible collision precisely: `AI 解构需求` and `批准审核契约` both render on `rgb(1, 139, 141)` while inheriting near-black `rgb(3, 18, 25)`. The drawer itself is correctly light (`rgb(255,255,255)` with dark ink), and the dark navigation rail already uses light `rgb(212,229,242)` text.
- Use a separate filled-accent contract rather than flipping the general accent token: dark teal filled controls get a darker AA-safe fill, cool off-white ink, and a darker hover fill; outlines, links, progress, and soft accent surfaces keep the existing teal.
- Implementation adds `--wa-accent-fill`, `--wa-accent-fill-hover`, and `--wa-accent-fill-ink`, then applies them to the admin primary action, selected date, shared Button, demand-detail AI action, and delivery primary action without changing light-surface ink.
- Final authenticated computed styles for both reported controls are `rgb(0,111,118)` background and `rgb(246,251,255)` text at 5.70:1 contrast. The light drawer remains 11.99:1 and the dark rail remains 15.37:1.
- Wide and 760×800 screenshots confirm the new light text reads cleanly on the dark teal buttons, full-height drawer geometry remains intact, and horizontal overflow remains zero. The delivery approval button was visually inspected in its scrolled state; console errors are empty.
- Final static gates pass: Svelte/TypeScript check has zero errors, production build and `git diff --check` pass, and finesse detector reports P0=0 after adding the shared spinner's reduced-motion fallback.

## Current Task: Viewport-Height Demand Drawer

- The supplied regression screenshot confirms the previous drawer conversion is still scoped to the flow-board workspace: its top begins below the application header/breadcrumb and its bottom ends at the board boundary, so it reads as an enlarged board panel rather than a true drawer.
- The correction is ownership, not more styling. The detail and linked companion overlays must use viewport-fixed positioning and a global modal layer; the drawer itself must be flush right and span `100dvh` with square corners, while `.detail-body` remains the only long-content scroll owner.
- The complete application shell, including top bar and sidebar, must sit under the same restrained backdrop so visual hierarchy and hit blocking are unambiguous.
- Three-way review agrees on preserving `DemandKanban.svelte` lifecycle ownership, the existing light/teal product language, current dialog semantics, and wide companion behavior. A portal is only warranted if authenticated geometry shows a transformed or contained ancestor constrains `position: fixed`.
- Browser acceptance is geometry-based: drawer and overlay must resolve to the viewport edges at wide and narrow breakpoints, with no horizontal overflow, no shell click-through, and no regression in focus, Escape, linked close, or companion separation.
- First authenticated geometry pass proved that `position: fixed` alone is insufficient: `.workspace-stage` establishes an isolated stacking context below the `z-index: 50` top bar. The drawer rectangle reached the viewport, but hit testing and the screenshot showed the shell top bar still painting above the drawer and hiding its header.
- The planned portal fallback is therefore required. Move only the detail and detail-companion overlay roots to `.functional-console`, where their existing fixed positioning and modal z-index can outrank both top bar and rail without weakening the shell's isolation contract for every other workspace.
- Final authenticated wide geometry is exact: the overlay is `2133.33 × 1037.78`, the drawer is `820 × 1037.78` at `top=0/right=viewport`, the top-left hit belongs to the backdrop, the top-right hit belongs to the drawer header, and document horizontal overflow is `0`.
- The wide AI companion remains `620px` wide with a `16px` drawer gap, no overlap, full-viewport companion overlay, and the same `.functional-console` host.
- At the measured mobile breakpoint (`760 × 800` CSS viewport), both overlay and drawer are exactly `760 × 800` with all four edges flush, both top-corner hit tests owned by the drawer, square corners, one `727px` body scroll owner, and no horizontal overflow.
- Escape closes the portaled drawer, restores focus to the DG-319 demand card, and restores body overflow. Wide backdrop click also closes the drawer and restores body overflow. Browser console errors are empty.
- Final static gates pass: Svelte/TypeScript check has zero errors, production build and `git diff --check` pass, finesse detector reports P0=0, and full-file detector warnings remain legacy rules outside the new portal/fixed/full-height selectors.

## Current Task: Flow Detail Drawer And Unified Task Filters

- The first supplied screenshot shows a centered, near-full-height demand-detail modal floating over a large dimmed board. Its rounded modal shell, internal document workbench, and dashboard shell use competing container languages, which explains the reported lack of visual unity.
- A right-side workspace drawer is the strongest fit for this operational state: demand selection remains spatially connected to the board, the shell hierarchy stays visible, and a fixed drawer header can own close/actions while one body region owns long specification scrolling.
- The second screenshot shows a dense execution workbench with a table on the left and inspector on the right. Their top edges align, but the inspector ends earlier than the visible table and the filter controls mix search input, segmented risk chips, shared dropdowns, and a standalone refresh button in one uneven row.
- The task-filter target is one consistent control vocabulary: every categorical filter uses shared Select, search remains the only text input, refresh remains an action, and all controls share a 40px desktop/44px narrow height and a common baseline.
- The table and inspector should share one desktop workbench height token and independent body overflow only where content requires it; at narrow widths they stack in source order with the inspector following the selected table row context and no horizontal overflow.
- The working tree is heavily modified, including the likely target files. This task must use narrow patches and preserve all unrelated changes.
- Design read: internal operational product UI, calm light console, teal accent, high density, feedback-only motion, targeted preservation redesign.
- Initial responsive contract from the screenshots: right drawer at wide/desktop widths, full-width off-canvas sheet at narrow widths; task toolbar may wrap deliberately at intermediate widths, while the table/inspector stack below the project breakpoint and every touch control reaches 44px.
- `DemandKanban.svelte` owns demand-detail selection, close behavior, body scroll locking, and the linked AI companion host state. Keeping the drawer shell in this owner avoids splitting lifecycle state or risking an orphaned companion.
- The current demand detail is still rendered through the generic `.modal-backdrop/.modal-content` contract, while its content already has a distinct `detail-body`, overview, facts, subtasks, and delivery-control hierarchy. The smallest structural correction is to change only the detail host into an off-canvas dialog and leave create/schedule/confirm modals unchanged.
- `CommitTelemetryPanel.svelte` provides a repository-proven off-canvas interaction reference, but it is telemetry-specific rather than a shared primitive. Reuse its right-edge/backdrop/body-overflow behavior and token language without coupling demand detail to telemetry markup.
- `TaskKanban.svelte` already imports the shared `Select.svelte`; the execution toolbar currently mixes that primitive with a custom segmented risk strip. The user request maps directly to replacing the categorical risk strip with a shared Select while preserving the existing text search, project/owner Selects, and refresh action.
- The active Phase 41 contract defines a restrained light admin shell, one shared component vocabulary, `--wa-control-h`/`--wa-touch-h`, and table-first plus right-inspector composition. No new dependency or design system is appropriate for this targeted pass.
- Three-way review agreement: demand detail becomes a right-edge off-canvas dialog with fixed header and one scrollable body; execution risk becomes a non-searchable shared Select; all categorical controls share one geometry; execution table and inspector share a desktop height/top baseline and release it below the stacking breakpoint.
- Review ownership: `DemandKanban.svelte` owns the drawer and AI-companion lifecycle, `TaskKanban.svelte` owns filtering/selection/panel geometry, and `Select.svelte` remains the only categorical-control behavior owner.
- Review disagreement resolution: do not introduce or retrofit a global Drawer primitive in this dirty-tree pass. Follow the repository telemetry drawer behavior locally because demand detail has unique linked-companion state; revisit extraction only when a second general-purpose consumer exists.
- Validation agreement: demand drawer open/close/backdrop/Escape/focus, long-body scroll and linked AI companion; risk/project/owner selection and empty/populated task states; table-inspector top/bottom geometry; wide desktop, 1440/1180/1024/760/480; 44px narrow controls, zero horizontal overflow, open-menu hit testing, reduced motion, detector, Svelte check, build, and diff hygiene.
- The active detail host is already absolutely scoped to the workspace instead of the browser viewport. Converting its flex alignment from centered to right-edge and its panel from bounded modal height to full workspace height can produce the drawer without changing shell or route ownership.
- The linked AI companion has a desktop side-by-side transform based on a centered 1080px host. The drawer implementation must explicitly cancel the host translation and place the companion to the drawer's left on wide screens; the existing centered overlay fallback below 1500px remains the safer responsive behavior.
- `TaskKanban.svelte` already defines `--task-aligned-panel-height` and applies it to status and personnel workbenches. Execution can reuse that contract rather than introducing another height token, while its table panel becomes a two-row grid and its inspector becomes the matching overflow owner.
- Shared Select supports `searchable={false}`, adaptive up/down placement, keyboard Escape, outside dismissal, compact mode, and the existing 36px control token. It can replace the six-option risk strip without any new control code.
- Implementation converts the detail host to a right-edge 820px workspace drawer with a fixed header, one hidden-rail body scroll owner, focus-on-open, Tab containment, Escape behavior, return focus, reduced-motion fallback, full-width narrow layout, and a wide-screen companion placed to its left.
- Implementation replaces the active execution risk strip with a non-searchable shared Select and reuses the existing aligned panel-height token so the table panel and inspector share top and bottom edges on desktop while releasing the contract below 1180px.
- First static pass succeeds: `git diff --check` is clean and `pnpm -C web check` reports 0 errors. The one newly unused risk-button selector warning was removed before the final check rerun.
- Authenticated Chrome validation confirmed the wide drawer is 820px, owns one `.detail-body` scroll region (`781px` client / `3185px` content in the measured state), traps Tab focus, restores focus on Escape, and has zero document-level horizontal overflow.
- At the measured 760px viewport the detail becomes a workspace-width sheet with one scroll owner and zero horizontal overflow; the task toolbar stacks its search and three dropdown controls without page overflow.
- The first wide companion pass exposed a real flex-basis bug: percentage width was resolved against the already padded remainder and collapsed the AI panel to about 1px. The corrected companion is 620px wide, sits 16px left of the drawer, has zero overlap, and returns focus to `AI 解构需求` after Escape.
- Authenticated task geometry is exact at wide desktop: table panel and inspector are both `617.78px` high with `0px` top and bottom deltas. At 1180px they stack with a 16px gap and equal 480px heights.
- Risk dropdown behavior is verified through the shared Select contract: it exposes one expanded listbox with `需关注 / 全部 / 高风险 / 中风险 / 正常 / 已闭环`, applies the selected value, closes the list, and preserves zero horizontal overflow.
- Final checks pass: Svelte/TypeScript check has 0 errors and 74 existing warnings, production build and `git diff --check` pass, finesse detection has no P0 findings, full-file Impeccable/finesse warnings are documented legacy selectors outside the new drawer/filter/alignment rules, and the authenticated browser console error list is empty.


## Current Task: Event Timeline, Flow Detail, And Execution Filters

- User evidence shows three regressions on the current light admin-console baseline: an inner scrollbar in event mediation records, an unresolved flow-board demand detail panel, and a clipped duplicate owner filter in execution tracking.
- Prior project memory identifies `DecisionDashboard.svelte`, `DemandKanban.svelte`, and the shared `TaskKanban.svelte` task-tracking slice as the first inspection targets.
- Prior validated task-tracking order is metrics -> stage strip -> shared filters -> page-specific table/workbench. This task must keep that flow while merging project and owner filtering into the execution-tracking bar.
- Existing working tree is heavily modified across backend and frontend files. All edits and validation must be target-scoped; no cleanup, revert, or broad formatting is authorized.
- Design read: internal operational product UI, calm light console, teal accent, high density, no decorative motion, targeted evolution only.
- Screenshot 1 is a 2048x925 authenticated flow-board detail state. The demand detail surface occupies almost the full usable viewport height and visibly owns an inner vertical scrollbar at its far-right edge; the AI specification body continues below the fold. This is evidence that the previous detail-panel pass did not establish a viewport-fit/no-competing-scroll contract for this host state.
- Screenshot 2 shows two filter layers in execution tracking: a page-level row containing `项目 / 全部项目` and `负责人 / 全部负责人`, then an execution toolbar containing search, risk/status chips, and another `全部负责人` select. The inner select menu is visually under the table/header layer, so row content remains visible through/over the menu. The target hierarchy is one execution toolbar row containing project, one owner filter, search, state controls, and refresh/count affordances.
- The screenshot makes the owner duplication a structural ownership bug, not a label-only problem: the page-level shared filters and execution-local filters both own the same semantic dimension. Consolidation must leave one source of truth and one interactive control for owner filtering in execution tracking.
- `DecisionDashboard.svelte` already gives `.inspector-content` vertical overflow ownership, while `.ledger-list` adds a second `max-height: 320px; overflow: auto` boundary. The local ledger boundary is the nested scrollbar root cause; mediation records should expand naturally inside the inspector.
- `TaskKanban.svelte` sends `selectedAssignee` as the API `assignee` query and then applies `executionAssigneeFilter` to the same `item.assignee` field. Both option lists come from `coreMembers`, so this is duplicated data and behavior rather than two valid business dimensions.
- Execution tracking will hide the page-level filter strip and move `selectedProject` plus `selectedAssignee` into the active execution toolbar. Other task views retain the existing shared filter surface. The shared `Select.svelte` owns menu keyboard/dismissal behavior, while the toolbar and table receive explicit adjacent stacking levels so the menu is never painted under task rows.
- The demand-detail backdrop is fixed inside the isolated workspace stage while the shell topbar owns a higher sibling stacking layer. The detail height still uses the full dynamic viewport, so its modal can consume space hidden behind the topbar and expose a competing-looking rail. The scoped repair is to calculate desktop detail space below the shell topbar, retain exactly one body scroll owner for long specifications, and hide only that owner's visual rail.
- Responsive contract: the supplied 2048px state keeps the execution controls in one line; intermediate widths may reflow to two rows before the inspector/table become unusably narrow; narrow screens collapse to one column and preserve 44px controls. Long specifications remain wheel, touch, and keyboard scrollable even when the rail is visually hidden.
- Validation scope agreed by all three reviews: authenticated current/all/empty event states; populated long demand detail and its AI companion/close linkage; execution project/owner combinations and open-menu hit testing; wide desktop plus 1440/1280/1024/760 geometry; horizontal overflow, focus, Escape/outside dismissal, console errors, targeted Impeccable detection, Svelte checks, build, and diff hygiene.
- Implementation removes the ledger's local max-height/overflow boundary, keeps the inspector as the only visible scroll owner, and preserves natural current/all/empty expansion.
- Demand detail and its AI companion now share the board's positioned backdrop contract. At 2048x925 the host and companion align within 1px vertically with a 16px gap and zero horizontal overflow; closing the host removes both surfaces. The long body remains scrollable while `scrollbar-width: none` removes the misleading rail.
- Execution tracking now binds its project and owner Select controls directly to `selectedProject` and `selectedAssignee`; the duplicate execution-assignee state and page-level execution filter row are removed. Wide desktop renders search, risk states, project, owner, and refresh in one visual row; responsive widths reflow without horizontal overflow and narrow controls measure 44px.
- Browser hit testing proves the open owner list overlaps the table but `elementFromPoint` resolves to the dropdown option, so table rows cannot paint through or intercept the menu. Combined project plus owner filtering returned the expected single fixture task.
- Final static validation passes with zero Svelte errors (78 existing warnings), a successful production build, clean diff whitespace, and only documented legacy Impeccable findings outside the changed selectors.


## Phase 68: Corpus Approval Information Architecture Refactor

- The screenshot does not show identical candidate records: titles differ, while the same imported document, scope, timestamp, status, and long provenance content repeat around them. The first fix is presentation grouping, not destructive deduplication.
- `CorpusCandidateReview.svelte` currently renders every candidate as a full queue card with status, two-line summary, type, scope, and time. Candidates already carry `context_document_id` and `source_document`, so the UI can group them by immutable document/version without a backend change.
- Selecting a normal candidate eagerly loads the impact endpoint and places `source_markdown` beside the candidate in two equal 500px panes. That comparison is required only after a candidate enters `impact_review`; during ordinary review it duplicates source and proposal content and overwhelms the metadata/action hierarchy.
- The agreed ordinary-review composition is one editable proposal workbench with a compact source-evidence disclosure. The agreed impact-review composition remains current active context versus the locked proposal, with immutable source evidence available separately.
- Queue grouping must preserve all IDs and candidate actions. Shared source title, original filename, version, creation time, and candidate count appear once in a source-group header; row content is reduced to status, title, and the smallest discriminating metadata.
- The existing shared Select and MarkdownWorkbench components already own control behavior and Markdown rendering. This phase should change only orchestration and presentation inside `CorpusCandidateReview.svelte`.
- Implementation groups all eight authenticated fixture candidates into two document batches and renders one filename/version label per batch. Browser counts prove all eight candidate IDs remain reachable; grouping does not filter or deduplicate records.
- Ordinary review now mounts one live Markdown workbench. Structured metadata and immutable source evidence are closed disclosures by default; opening metadata exposes three shared Select controls and zero native `<select>` elements, while opening evidence lazily mounts the read-only source Markdown.
- Candidate titles are content-aware: when the first Markdown heading already equals the candidate title, the detail header suppresses its duplicate; candidates without a matching first heading still retain the header title.
- Impact review still mounts the required two-pane current-context versus proposed-content comparison. The browser-validated high-sensitivity transition changes the queue counters from `7/1` to `6/2`, locks metadata, renders the impact reason, and exposes the separate final publish action.
- The proposed impact pane now renders `draft.content`, matching the value written to `ContextFact.Content`; it no longer consumes the server's display-oriented `after_markdown` wrapper that can repeat title and summary.
- Queue height is viewport-bounded and locally scrollable. Browser geometry passed at desktop, 1024px, 760px, and 480px. At 480px the document has zero horizontal overflow, queue height is 318px, the layout is one column, and actions/disclosures measure 44px.
- Populated, empty, and error states were verified independently. The error state has exactly one alert and zero empty-state blocks; the final clean authenticated run has no console warnings or errors.
- Final validation passes `svelte-check` with 0 errors and only 78 existing warnings in unrelated files, production build, targeted Impeccable detection (`[]`), and diff hygiene.

## Phase 67: Shared Import Select And Live Markdown Composition

- The import scope field is the only control in this form still rendered as a native `<select>`, despite the repository already providing a shared Select with keyboard navigation, outside/Escape dismissal, adaptive overlay placement, errors, disabled state, and compact layout support.
- The smallest ownership fix is to pass the four scope options and current value into shared Select, then keep only the existing `global` side effect in the parent. Search is unnecessary for four fixed options.
- `MarkdownWorkbench.svelte` already owns CodeMirror, Markdown/GFM parsing, sanitized preview, value synchronization, and the edit/split/preview modes. A separate WYSIWYG package or `contenteditable` surface would duplicate ownership and risk lossy HTML-to-Markdown conversion.
- CodeMirror's official `ViewPlugin` and `Decoration` APIs can provide Typora-like WYSIWYM behavior without changing the stored value: inactive lines hide Markdown delimiters and receive semantic typography, while the active line keeps its syntax visible for editing.
- The live mode should become the corpus paste default, but existing source, split, preview/reading, readonly, save, commit, and `change` contracts must remain compatible for other consumers.
- The UI review converged on a targeted preservation pass: no surrounding layout, color, radius, or typography redesign; reuse shared Select and add live composition inside the existing editor only.
- The first Impeccable scan classified a 3px left border on formatted blockquotes as the side-tab anti-pattern. Replace it with a semantic quote widget and restrained row tint rather than suppressing the finding.
- Implementation keeps all Markdown in the existing CodeMirror state and adds only a `live` display mode. Current-line syntax remains editable; inactive-line heading, strong/emphasis, strike, code, link, quote, bullet, and task markers are represented with decorations/widgets that never rewrite the document.
- The import scope control now uses the shared non-searchable Select and retains the existing global-scope cleanup. The import form owns its 44px narrow-screen control token rather than changing every Select consumer globally.
- Browser proof passed shared Select click, Enter, Escape, outside dismissal, option selection, keyboard return to global, and global range-ID disabling. The page contains no native import `<select>`.
- Browser proof also passed immediate Markdown rendering, active-line syntax reveal, raw source preservation, split/read fallbacks, and 1440/1024/760/480 geometry. Every viewport had `scrollWidth === innerWidth`; at 760/480 the Select and all four Markdown mode buttons measured 44px.
- Final browser console and page error lists are empty. Svelte check has 0 errors and only the repository's 78 existing warnings; production build, TypeScript, final Impeccable scan (`[]`), and diff hygiene pass.

## Phase 66: Opaque File Handoff Correction

- The user explicitly corrected the Phase 65 boundary: file uploads belong to the LLM input contract, not to frontend or backend text parsing. This supersedes the earlier `.md/.txt` normalization decision.
- The current frontend violates that boundary by calling `await file.text()`, copying the result into `MarkdownWorkbench`, and posting JSON `content`.
- The current backend violates that boundary by accepting the file-derived `content`, persisting it as the source document, and concatenating `document.Content` into the LLM user prompt.
- The corrected invariant is mechanical and testable: selected `File` stays a `File`; the browser sends multipart; the server reads only opaque bytes plus metadata; the provider adapter creates a file block; uploaded bytes never occur in the text prompt; persistence receives only the LLM extraction document and candidates.
- The UI remains a restrained product workbench. File mode needs metadata/transport status rather than an editor preview; paste mode continues to reuse `MarkdownWorkbench` as the explicit human-authored text path.
- OpenAI developer-docs MCP tools are not available in this session, and the local `codex mcp list` command fails before execution because its packaged binary path is missing. Use official OpenAI-domain web documentation as the narrow fallback and log the exact request schema before implementation.
- Official OpenAI file-input documentation confirms that Responses accepts an `input_file` content part with inline Base64 `file_data` plus `filename`; the app adopts that transport while keeping a conservative 10 MiB local limit below the documented API limit.
- The native Messages adapter deliberately rejects opaque file imports with an actionable Responses-protocol error. It never falls back to local text extraction.
- Completed imports persist only LLM-returned `document_markdown`, summary, and candidates. Failed imports persist metadata/hash/status only; duplicate completed files reuse the existing extraction without another provider call.
- Browser and backend proof agree on the boundary: multipart retains exact selected bytes, the text prompt never contains the sentinel body, file mode never mounts the Markdown editor, and paste mode remains a separate explicit text input.


## Phase 65: Governed Corpus Ingestion And Unified Review Center

- The requested change extends the Phase 64 corpus tabs from presentation cleanup into a governed lifecycle spanning source documents, LLM extraction, candidate review, impact preview, and publication.
- Existing `CorpusCandidate` and `ContextFact` types already provide the promotion seam; the new work must preserve that seam while adding immutable source provenance and a separate publish gate for sensitive/global candidates.
- The repository already has a shared CodeMirror/GFM `MarkdownWorkbench.svelte`; it is the required owner for import/paste content and any Markdown comparison/editing, avoiding a parallel textarea editor.
- Design Read: enterprise configuration workbench for administrators and system-design owners, restrained light-console language, register=product, SOUL=4, SPECTACLE=2, DENSITY=8. Motion is feedback-only.
- The working tree is broadly dirty from prior phases. Phase 65 must use narrow patches and preserve every unrelated modification.
- `ContextDocument` already exists and is auto-migrated, but it currently stores only normalized title/type/scope/source/status/version/content metadata. It lacks original filename/MIME, ingestion status, uploader, source hash lineage, parse failure, and explicit raw-versus-normalized provenance.
- `ContextFact.ContextDocumentID` already provides the document-to-fact link, while `CorpusCandidate` has no source-document link. The smallest compatible model extension is to attach candidates to `ContextDocument` and add review/publication metadata rather than invent a parallel candidate table.
- The production context-pack builder reads only `ContextFact` rows whose status is `active`. This is the authoritative publication boundary and already excludes every unpromoted candidate.
- Current candidate acceptance is one atomic transaction that immediately creates an `active` fact. Sensitive/global governance therefore needs a new intermediate candidate status and a separate publish endpoint; ordinary acceptance can retain the current immediate-promotion behavior.
- Existing permissions are sufficient for the first implementation: `ai_context:read/write/preview` owns documents/import/impact preview, while `corpus_candidate:read/review` owns unified review and publication. No new RBAC code is required unless tests expose an ownership gap.
- The current corpus UI is already split into `资料库 / 候选审核 / 上下文预览`, but `资料库` is fact-first and mounts a long manual form next to the list. Phase 65 should make source documents and import the primary library task, then move direct fact creation behind an explicit advanced disclosure.
- `CorpusCandidateReview.svelte` currently renders compact read-only articles and only sends accepted/rejected. The unified center needs selected-row ownership, editable proposed Markdown/metadata, source provenance, queue state filters, and an impact-review state with a separate publish action.
- `MarkdownWorkbench.svelte` already exposes edit/split/preview, read-only, change/save/commit, custom height, and responsive stacking. It can be reused for import content, manual fact content, candidate editing, and left/right read-only comparison without changing the editor engine.
- The current AI context page already follows the Phase 64 progressive-disclosure tab contract. Preserve that outer task hierarchy rather than adding another nested page title or card grid.
- Three-way UI review converged on a source-first library and selected-candidate workbench. Glass remains limited to the existing outer Settings structure; source rows, metadata, Markdown editors, comparisons, and action bars stay flat with hairlines.
- The initial supported file boundary is text-native `.md`, `.markdown`, and `.txt`, normalized to Markdown while preserving original filename, MIME, hash, uploader, version, and source content. Unsupported binary documents must fail explicitly instead of silently producing unreliable text.
- The impact comparison should compare the currently active matching fact set against the proposed candidate, while showing the immutable source document as provenance. Only `impact_review` candidates expose the final publish action.


## Demand Detail Action Hierarchy And Draft Footer

- The visible purple mismatch came from selector specificity: the legacy `.detail-link-row button` rule outranked the newer single-class AI button rule. Scoped two-class action rules now let the shared admin tokens own the actual rendered colors.
- The top actions now express product hierarchy instead of category decoration: `AI 解构需求` is the teal primary accelerator, while `调整排期` is a neutral secondary utility with the same height, radius, focus language, and restrained hover treatment.
- `撤销草案` is no longer stranded beneath the specification document. It shares the terminal review action row with contract controls, remains visually isolated on the left as a destructive action, and preserves the existing explicit confirmation alert.
- Authenticated DG-319 validation opened and cancelled withdrawal without mutating business data. Desktop and narrow states showed no horizontal overflow, focus/hover states matched tokens, and the console remained error-free.
- Svelte check, production build, diff hygiene, and Phase 61-targeted Impeccable detection pass. Whole-file Impeccable findings remain pre-existing outside the changed ranges.

## Reviewer Multi-select Consecutive Selection Correction

- Phase 59's close-after-toggle behavior was the direct cause of the reported regression. A multi-select needs one continuous selection session, so selecting or deselecting an option now updates the bound values without ending that session.
- The shared capture-phase outside-pointer and Escape lifecycle remains correct. Clicking elsewhere in the modal closes the list even though the modal stops bubbling clicks, while `完成选择` and the chevron remain explicit dismissal paths.
- Removing a selected chip exposed a separate focus boundary: pointerdown focused the remove button and the following value update deleted that focused node, causing `focusout` to close the list. Preventing focus transfer on remove-button pointerdown preserves the open session without suppressing its click action.
- Authenticated DG-319 validation proves consecutive selection, option deselection, chip removal, outside dismissal, Escape, explicit completion, and chevron close. Temporary values were restored and no save/approval action was triggered.
- Desktop and narrow screenshots show the bounded in-flow options remain inside the modal with no horizontal overflow; Svelte check, build, targeted Impeccable detection, and diff hygiene pass.

## Deterministic Multi-select Completion

- The screenshot's apparent outside-click failure was partly a hit-testing problem: the absolute full-width option list covered the following form rows, so clicks intended for required roles or owners were still inside the multi-select.
- Keeping the list open for consecutive selection needs a visible completion affordance. A footer outside the ARIA listbox now owns selected count and `完成选择`.
- Escape previously called `closeDropdown()` and then focused the search input; its `focus` handler immediately reopened the list. Closed-state focus now returns to the chevron.
- Moving the list into normal flow initially exposed a containing-block bug: the absolute chevron used the expanding wrapper for `top: 50%`, moved over `张路路`, and caused apparent click-through. `position: relative` on the fixed trigger keeps the arrow stable.
- Browser geometry proves the open list ends 37px before the next form group, has no overlap or horizontal overflow, and keeps the toggle inside the trigger at both default and 480px widths.
- The authenticated app route was unavailable because the automation session had no login state. The actual shared Svelte component was therefore mounted in a temporary local harness for interaction testing; the harness was deleted after validation.

## Review Select Dismissal And Core-member Boundary

- The close failure was event-boundary specific: the shared controls listened for bubbling `window.click`, while the demand-detail modal intentionally stopped click propagation. Capture-phase `pointerdown` is the stable outside-dismiss contract for both controls.
- Searchable triggers previously exposed only an open path. Making the chevron a real toggle button provides an explicit close path without changing single- versus multi-selection behavior.
- Owner leakage came from merging the full `/api/users` page directory into contract choices. The existing `coreMemberVisibility` service and `/api/demands/options` assignee directory are the authoritative backend/frontend boundary.
- Backend filtering alone is insufficient during a rolling restart because the frontend may still talk to an old server. The UI therefore intersects the combined directory with the core-member list when available and trusts only contract-scoped participants before it arrives.
- A contract-ID-only normalization guard can retain a previously valid but later-filtered owner when directory data arrives asynchronously. Including the normalized option signature in the guard ensures stale non-core values are removed before save.
- Focused lifecycle testing proves a seeded non-core `External Operator` is absent while configured `Reviewer` remains available. Svelte check, build, Impeccable complete/layout detection, and diff hygiene pass.
- Live authenticated browser interaction remains unverified in this run because the browser was locked to a generated connection-error page after the local listener had been down; browser policy prohibited further navigation or DOM actions from that state.

## Health Diagnosis Intervention Modal — Audit Start

- User reports that the modal's layout and palette have excessive contrast and diverge from the system aesthetic.
- Target direction: calm light admin-console surfaces, cool neutral hierarchy, one restrained risk accent, fewer nested card boundaries, and clearer reading order.
- Screenshot shows the modal is owned by `ProjectHealthTelemetry.svelte`. The current surface combines a pale pink command block, saturated red diagnosis copy/score, blue/green/orange/pink metric bars, lavender status chips, cyan scrollbar, and cool-gray cards; the number of competing hues is the main aesthetic mismatch.
- The header is visually under-structured, while the top intervention block dominates due to its tinted area and oversized white `49.64`. Below it, multiple bordered cards inside section cards create a nested-card effect and make the two columns feel fragmented.
- The modal width is reasonable, but body content is dense and vertically scrollable. The left score/formula card consumes substantial height while the right column continues into intervention content, so section rhythm and cross-column alignment need calibration.
- Markup contains many inline layout styles inside the modal (`phdi-score-row`, formula width, metric row headers), which makes the presentation harder to calibrate responsively and should be replaced with scoped classes.
- The file contains both legacy dark modal/table styles and later light admin-console overrides. The final modal fix must land after the current light overrides or remove conflicting rules, not add another partially competing palette earlier in the file.
- Existing system tokens already provide the correct palette: white/inset surfaces, cool gray text/borders, teal accent, and muted success/warning/danger soft colors.

### Open Questions

- Which component owns the visible modal and whether later CSS overrides are fighting its base styles. **Owner found:** `ProjectHealthTelemetry.svelte`; style conflict still to inspect.
- Whether the two-column body is structurally sound at the screenshot viewport or needs source-order/layout changes. **Resolved:** keep the two-column source order at desktop, tighten the gap, and collapse to one column below 860px.
- Which content blocks must remain visually emphasized for intervention safety. **Resolved:** urgency, overall score state, diagnosis evidence, dimension status, and recommended intervention route; all other surfaces remain neutral.

### Chosen Visual Contract

- Scope all corrections under `.health-diagnosis-modal` so table/workbench styles remain untouched.
- Use a structural header with small context label, a compact intervention summary, and a body-owned scrollbar; remove modal-wide hover contrast.
- Replace the top tinted block's nested white cards with three divided summary cells on one quiet surface.
- Make diagnosis copy neutral with a restrained tone border, remove score glow/text-shadow, and use soft semantic fills only for state chips and result nodes.
- Normalize metric rows to one quiet surface and semantic low-saturation bars; recommendations use neutral rows with a narrow tone rail instead of colored cards.

### Audit Errors

- A CSS-token search omitted the `--` argument separator, so `rg` parsed the pattern as a flag. The token file was still read directly; subsequent searches will use the correct separator.
- The browser's constrained evaluation DOM does not implement `document.createElement`; isolated visual verification must replace the local preview body's markup directly or use existing locators.
- Direct body markup replacement is also blocked because the evaluation DOM exposes `innerHTML` as getter-only. A temporary static page served by the local preview is the appropriate non-auth visual harness.

## Streamed LLM Attachments And Demand File Archive — Initial Correction

- The browser extraction approach is the wrong boundary for this product: it increases the web bundle, cannot reliably preserve document fidelity, and prevents durable original-file provenance.
- Required target flow: `multipart/form-data` upload to `/api/deconstruct` → bounded backend spool/stream → gzip-compressed original storage + database metadata/association → provider-specific file delivery to the LLM → deconstruction result/archive linkage.
- Existing JSON-only deconstruction calls must remain valid; multipart is an additive request form.
- PDF/DOC/DOCX browser dependencies added in the previous pass are candidates for removal once the backend attachment path is in place.
- `/api/deconstruct` currently decodes JSON directly, builds a text-only `messages: [{role, content}]` payload, and calls an OpenAI-compatible endpoint itself. The configured endpoint mode is either `completions` or `responses`.
- `DeconstructArchive` is not created by preview deconstruction; it is created later during task import/archive. Attachment metadata therefore needs a nullable staged association first (demand/context/session key), then archive linkage when import persists the archive.
- The database uses GORM `AutoMigrate`, so a dedicated attachment model can be added without a handwritten migration.
- Official Responses file input supports `input_file` with Base64 `file_data`, Files API `file_id`, or external URL. PDF supplies extracted text plus page images; DOC/DOCX and other rich documents supply extracted text. The accepted rich-document list includes both `.doc` and `.docx`.
- For this local archive flow, inline Base64 `file_data` avoids creating unmanaged remote Files API objects. It is not literal zero-copy streaming at the provider hop because the Responses JSON contract requires Base64, but the browser-to-server hop remains multipart streaming and the original bytes remain preserved locally.
- Native `input_file` is a Responses API contract. Text-only requests preserve the configured endpoint, while attachment requests automatically derive the same provider's `/v1/files` and `/v1/responses` paths. Providers missing those APIs return their actual compatibility error; the server never falls back to local parsing.
- Runtime persistence is currently rooted at the process working directory (`well-ambient.db` in `cmd/server/main.go`), with no storage-dir config. A sibling default such as `data/demand-attachments/` is consistent, but a configurable server attachment directory is safer for deployment and tests.
- Frontend import already receives `archive_id` only after `/api/tasks/import`. The deconstruction response exposes `context_pack_id`, so attachment rows can be created at upload time with `DemandID` + `ContextPackID`, then updated with `DeconstructArchiveID` inside the import transaction.
- The frontend currently sends only `{text}` to `/api/deconstruct` and later sends `demand_id`, `task_group_id`, and `context_pack_id` to `/api/tasks/import`. Multipart should include `text`, `demand_id`, `task_group_id`, and repeated `files` so association data is available immediately.

### Open Architecture Questions

- Does the current OpenAI-compatible provider contract support native file parts, Files API references, or only text messages? **Resolved:** only the configured `responses` path should advertise native attachments; completions remains text-only.
- Which persisted ID is available before and after deconstruction to associate independent uploads safely? **Resolved:** stage by attachment ID + uploader + `context_pack_id`; finalize with `deconstruct_archive_id` during task import.
- What application data directory/config convention already exists for durable local artifacts? **Resolved:** add `server.attachment_dir`, defaulting to `data/demand-attachments`, because current runtime has no artifact directory contract.

### Chosen Backend Contract

- Attachment row: original name, MIME, extension, gzip-relative path, SHA-256, original/compressed sizes, uploader, demand ID, task-group ID, context-pack ID, archive ID, status, timestamps.
- Request: JSON remains supported when no file is present; multipart fields are `text`, `demand_id`, `task_group_id`, and repeated `files`.
- Provider: stream each gzip-decoded original to `/v1/files` with `purpose=user_data`, automatically use `/v1/responses` with `input_file` IDs even when text calls are configured for completions, then best-effort delete remote files.
- Limits: allow the existing document/text formats, max 20 MB per file and max 3 attachments per request.

### Implementation Result

- `DemandAttachment` is auto-migrated and records gzip location, SHA-256, original/compressed size, uploader, demand/task-group/context-pack/archive associations, and lifecycle status.
- Provider upload opens the gzip archive, streams decompressed bytes through `io.Pipe` into a multipart Files API request, references returned IDs in a Responses request, and deletes remote files after completion.
- Frontend retains `File` objects only until submit, sends multipart without setting its own boundary header, and passes returned attachment IDs into task import for transactional archive linking.
- Browser PDF/DOC parsing code and `mammoth`/`pdfjs-dist` dependencies are fully removed; repository search finds no remaining references.
- Focused end-to-end test proves provider receives the original bytes, Responses payload includes the remote file ID, remote cleanup occurs, local metadata is persisted, and import links the attachment to both demand and archive.
- Existing `chat/completions` configuration remains valid for text requests; the focused test now starts from completions configuration and proves attachment requests automatically target `/v1/files` plus `/v1/responses`.
- Final backend suite passes across `internal/server`, `internal/db`, and `internal/config`; frontend production build passes and no browser parser dependency/reference remains.

### Self-Reflection

- The first correction stopped too early at “support these file extensions” and chose a browser parsing implementation without tracing the product's provenance and provider contracts. For requirement evidence, preserve originals and design the persistence/provider boundary before selecting client libraries.

### Discovery Errors

- A repository search command used the shell pattern `.env*`; zsh rejected it before `rg` ran because no file matched. Future searches must quote the pattern or avoid shell globs.
- A search command listed nonexistent `main.go` explicitly, causing an `rg` path error even though the useful results were returned. Future searches should use discovered paths such as `cmd/server/main.go` only.
- Dependency removal initially failed because pnpm selected `.pnpm-store/v10` while current modules are linked from `~/Library/pnpm/store/v10`. Use the existing store explicitly to keep the lockfile operation bounded.
- Initial Go tests were blocked before compilation because the sandbox denied writes to `~/Library/Caches/go-build`; use a workspace-safe `/tmp` cache. Frontend build passes. `pnpm check` still reports the pre-existing `DemandKanban.svelte:1798` union-property error outside this attachment change.
- With `/tmp` cache, the backend compiled and tests advanced until an unrelated GitLab webhook test attempted to bind `httptest` on IPv6 loopback, which the sandbox blocks. Focused attachment tests will use approved loopback access.

## Demand Difficulty And Task Detail Surface — Initial Audit

- Demand scheduling uses a page-local custom difficulty dropdown even though the repository now has a shared Select with outside-click, keyboard, and viewport-aware placement behavior. The selected difficulty is intended to flow through `schedDifficulty` into `/api/tasks/schedule`, where the backend normalizes Low/Medium/High correctly.
- Replace only this custom control with shared Select and keep the existing estimate state/save payload. This removes duplicated open-state and outside-click logic from the affected field.
- The shared Modal is already a light frosted surface, but TaskKanban's slotted detail content still declares dark-theme values such as `#e2e8f0`, `#f1f5f9`, and a dark evidence summary. On the light modal these become unreadable or visually inconsistent.
- Add a final `.task-detail-surface` contract scoped to the modal content, covering labels, values, project/type/status badges, dividers, telemetry action, evidence summary/timeline/empty states, and signal chips without changing task data behavior.

### Implementation And Validation Result

- The custom difficulty trigger/listbox and its duplicated open-state handlers were removed. Shared Select now emits the selected Low/Medium/High value directly into `schedDifficulty`, while the existing schedule payload and manual-estimate source behavior remain unchanged.
- The light task-detail contract is appended after legacy TaskKanban styles and raises contrast for every screenshot-visible region: metadata values, long title, project/bug badges, status, telemetry button, evidence summary, empty-state warning, timeline, and signal chips.
- `pnpm --dir web check` passes with 0 errors and 74 existing warnings; the replacement added no new unused-selector or component diagnostics.
- `pnpm --dir web build`, `git diff --check`, and focused `TestDemandAndKPILogic` pass. The backend test confirms High difficulty persists with `EstimateSource=manual_adjusted`.
- Authenticated browser interaction was not available from the no-login preview, so the two exact logged-in surfaces were not click-tested in-browser this turn.

## Adaptive Overlays And Decision Timeline — Initial Audit

### Corrective Follow-up: Timeline Date Semantics

- User verification exposed that records such as 21:08 appeared under Today at 10:30. The timeline UI was structurally correct but the event contract carried only `HH:mm:ss`, and the first frontend implementation incorrectly supplied today's date.
- Database evidence confirms these are historical records, not timezone-shifted current events: TPY-6597 is `2026-06-19 21:08:05+08:00`, HIT-999 is `2026-07-09 17:54:33+08:00`, HR-4260 is `2026-07-08 17:53:34+08:00`, and ZK-944 is `2026-07-10 17:18:53+08:00`.
- `AutoDecision` now exposes `occurred_at` as the full source timestamp. The old `time` field remains for compatibility, while the frontend uses `occurred_at` for correct date grouping and local-time rendering.
- For a rolling deployment where the frontend briefly talks to an old backend, a time-only value later than the current clock is treated as the previous day instead of a future event; once the backend restarts, the exact source date wins.

### Corrective Follow-up: HR-4202 Reschedule Consistency

- The schedule page and intervention handler already read/write the same `task_telemetries.due_date`; this was not a stale schedule cache or a split-table problem.
- Read-only evidence shows HR-4202 remains `2026-06-25`, and the 10:10 decision event recorded `old_value=2026-06-25` and `new_value=2026-06-25`. The old UI therefore submitted the unchanged date and the backend incorrectly reported success.
- The decision form now consumes DatePicker's explicit change event, shows the exact pending target in the button label, disables submission until the target differs, and verifies the persisted `due_date` returned by the server.
- The server rejects no-op reschedules with HTTP 409, returns the persisted due date on success, and the regression test proves a decision change from June 25 to July 13 is immediately present in the schedule projection.

- The visible management shell is `FunctionalAdminShell.svelte`; its notification popover toggles locally but has no document/window click-outside listener. The legacy `App.svelte` notification branch already has such a listener, so changing only that branch would not fix the reported UI.
- `Select.svelte` and `DatePicker.svelte` are shared across the decision and settings surfaces, but both hard-code `top: calc(100% + 8px)`. Their existing click-outside and Escape behavior can be retained while adding one live placement calculation after render and on viewport scroll/resize.
- The decision page currently renders automatic and manual records as two columns with separate `min-height: 320px` cards. This makes a single manual record occupy a large empty panel and separates events that belong to the same operational chronology.
- Use one content-height panel with a unified event read model, date headers, a continuous timeline rail, distinct automatic/manual markers, preserved Jira/commit links, and a bounded internal list only when event volume requires it.

### Implementation And Validation Result

- The active shell now binds the notification wrapper and closes only when a window click lands outside it; clicks on notification actions remain inside the boundary.
- Shared Select and DatePicker primitives calculate available space after render and on captured scroll/resize, choose the roomier direction when the preferred panel does not fit, and cap internal panel height to the chosen viewport space.
- Automatic records and current-session interventions now map into one reverse-chronological event model, group under Today/Yesterday/full-date labels, retain Jira/commit links, and distinguish automatic/manual events with compact markers.
- Browser QA on the unauthenticated Settings preview confirmed Select opens down with room and adds `drop-up` at the bottom of a 1280x520 viewport; the panel stayed within `187px..363px` and emitted no console errors.
- Notification and decision timeline browser interaction remain login-gated in the preview environment. Static checks and production build cover those paths; no browser-login workaround was attempted.

## KPI Workspace Clean Rebuild — Correction Audit

- The screenshot confirms severe layout failure: the page content occupies only the left half of a wide workspace, the report inspector floats in an unrelated center column, the member table begins hundreds of pixels below the metric row, and the rest of the viewport is empty.
- This was caused by retaining old `.kpi-dashboard` definitions with named areas such as `header`, `focus`, `segments`, `diagnostic`, and `breadth` after those DOM sections were removed. Later CSS attempted to redefine the grid, but earlier `display: contents`, area ownership, sticky report sizing, and responsive rules continued to conflict.
- The prior implementation therefore violated the redesign skill's highest-leverage rule: it changed structure without removing the obsolete layout system. A clean component replacement is lower risk than trying to reason about more than 3,000 lines of accumulated KPI CSS.
- The corrected information hierarchy will be source-order aligned and compact: controls, four health metrics, delivery/risk overview, member capability table with adjacent detail, and evidence. No empty named tracks, sticky center inspector, or page-level masonry.

### Clean Rebuild Result

- `KPIKanban.svelte` was replaced instead of patched. The new file has one script/read-model layer, one source-ordered markup tree, and one style block; no `.kpi-dashboard`, `.report-panel`, `.member-matrix-panel`, legacy named area, or `display: contents` contract remains.
- Wide authenticated measurement: the KPI workspace uses the full 1720px content width; toolbar, metric strip, overview, member panel, and evidence panel share the same x-coordinate and width. Document overflow is zero.
- The first runtime pass found that the risk/meeting list made the overview row 689px tall. The final risk panel is capped at 430px with internal scrolling and independent card heights, moving the member workspace from y=1057 to y=798 on the same viewport.
- Medium-width verification collapses overview and member layouts independently. Narrow verification has zero document overflow; only the 900px data table scrolls within its bounded table shell.
- Real interaction checks passed for weekly to daily switching, team metric refresh, member row selection, personal report generation, evidence scoping, and return to team report. Browser console returned zero errors.
- Self-review lesson: for a heavily iterated component, markup replacement must be paired with deletion of the old layout/CSS system. End-of-file overrides are not a valid structural refactor.

## Main Workspace IA And Metrics Rebuild — Initial Audit

- The shared shell already provides breadcrumb and route identity, so component-local title/description cards repeat context and delay access to the first actionable row.
- The requested Decision, Schedule, Evidence, Task, and KPI removals are explicit authorization to restructure those surfaces; protected behavior is the underlying data and operational actions, not the duplicate presentation wrapper.
- Product UI constraints for this pass: first-screen metrics must be scannable in one row, data numerals use tabular figures, cards exist only where they group a decision or comparison, and child pages must own loading/empty/error states.
- Task Table and Execution Tracking should remain separate information views: the first answers ownership/status/priority, while the second answers delivery evidence and execution state. Their filters and selected-task context should be aligned, but merging both datasets into one dense table would weaken both jobs.
- People Load is the primary task-tracking redesign target and should prioritize capacity, active work, bugs, delay exposure, and concentration risk per person rather than decorative summary tiles.
- KPI reconstruction must first locate the existing daily/weekly report contract and the AI/manual demand score source. If the backend already exposes sufficient task/demand fields, prefer a frontend read model; add or alter API contracts only when a required metric cannot be computed correctly client-side.

### Audit Questions

- Which shell state should own task submenu selection and how is it synchronized with the component's current `taskView` state?
- Which existing records expose demand type, delay/overdue status, owner, progress, risk, and base score without introducing synthetic data?
- Which page-local header wrappers can be removed without also removing useful actions, filters, or loading feedback?

### Implemented Decisions

- Decision now starts with one six-column priority row: weekly demands, weekly bugs, visible items, high risk, medium risk, and automatic records. The duplicate `核心决策流转` command card is gone.
- Schedule and flow views no longer repeat their route title. The new-demand action lives in the schedule filter toolbar and a compact flow control row; authenticated desktop verification confirms the eight schedule controls remain on one row with zero document overflow.
- Evidence search/count/refresh moved into the evidence table header, allowing the `项目介入优先级` intro card to be removed without losing control access.
- Task navigation now belongs to the global rail. Task Table and Execution Tracking remain separate because ownership/status and code/MR evidence are distinct user questions; both share the same project/owner filter surface.
- People Load is now a comparison table ordered by delay and active load, with task/Bug counts, a six-active-item capacity reference, explicit scheduling status, and a selected-person risk inspector.
- KPI uses the existing period/report endpoints for daily and weekly views. The first row now exposes delivery progress, risk exposure, evidence coverage, and delay ratio; the personal matrix exposes task count, Bug count, delay ratio, requirement base score, capability rating, and delivery evidence.
- Requirement base score is derived from AI deconstruction difficulty (High 90, Medium 75, Low 60) or estimate duration fallback; existing `manual_adjusted` scheduling estimates are counted as human revisions. Capability rating combines 60% requirement base score and 40% evidence-backed KPI score.
- Browser testing found two hidden Svelte dependency-order bugs in derived Decision and Task metrics. Both were corrected by declaring the metric arrays and explicitly referencing their reactive inputs; the first loaded render now shows real counts rather than zero or a blank page.
- The first workload-profile assertion also exposed a denominator mismatch: delay ratio excluded demands from task/Bug volume but included overdue demands in the numerator. The final model now computes both numerator and denominator from the same task/Bug population.

## Rounded Surface And Modal Corner Audit — Initial Findings

- The supplied screenshot shows a correctly rounded white modal body but four pale rectangular corner protrusions outside that radius. This points to an unclipped inner/decorative layer or a square background/shadow layer, not to the outer modal radius being absent.
- The visible artifact is strongest at the top-right and bottom corners where a square layer extends beyond the white rounded surface; the fix must clip all modal layers at one authoritative outer container rather than merely increasing `border-radius`.
- The requested visual direction is a flat, calm, rounded management console: outer modals/panels should use the larger container radius, while inputs and inner controls keep a tighter related radius.
- Prior schedule work in `DemandKanban.svelte` is currently uncommitted and belongs to the same user workflow; the corner audit must preserve it while keeping background `task_status.md` and `well-ambient.db` changes untouched.

### Audit Questions

- Which shared tokens/classes own modal, card, panel, table-shell, inspector, and workbench radii?
- Which pseudo-elements, gradients, inset layers, sticky sections, or `overflow: visible` declarations can visually escape a rounded parent?
- Which page-level legacy overrides reintroduce square corners after the shared light-console calibration?

### Confirmed Root Cause

- `DemandKanban.svelte` gives `.modal-content` a `20px` outer radius, but both `.demand-create-modal` and `.schedule-modal` retain `overflow: visible` so dropdowns and date pickers can escape the modal box.
- The modal header and footer use their own translucent backgrounds without matching top/bottom corner radii. Because the parent does not clip descendants, those square child backgrounds remain visible in all four rounded-corner cutouts exactly as shown in the screenshot.
- Simply changing the modal to `overflow: hidden` would remove the artifact but regress open dropdown/date-picker overlays. The safer repair is to give first/last modal surface sections matching inner radii and reserve clipping for surface primitives that do not host escaping overlays.
- Shared `.wa-admin-card` and `.wa-admin-table-shell` already declare the large radius token, but only table shells clip their content through `overflow: auto`; shared cards lack a consistent `background-clip`/isolation contract and must be audited for square child layers.
- Structural `.wa-admin-section` intentionally has no background or border and should not receive decorative radius merely because of its name; only visible surfaces should be rounded.

### Cross-Page Audit Classification

- The reusable `shared/Modal.svelte` is already safe: its container owns the radius and `overflow: hidden`, so header/body backgrounds cannot escape.
- `ProjectHealthTelemetry.svelte` also places scrolling on the rounded modal container itself; its current overflow behavior clips inner content and does not reproduce the screenshot artifact.
- `CommitTelemetryPanel.svelte` is an intentional edge-to-edge right drawer, not a floating card. Its square viewport-edge corners are part of the sheet pattern and should not be globally rounded like a centered modal.
- Most `2px`–`5px` radii found in production components belong to scrollbar thumbs, progress bars, status markers, small badges, or legacy micro-controls rather than visible outer cards. Raising all of them would create oversized pills and blur hierarchy.
- The many `border-radius: 0 !important` rules in `SettingsPanel.svelte` reset transparent nested legacy wrappers inside already-rounded `section-card` surfaces. They are structural flattening rules, not exposed square cards, so they should remain zero.
- The global audit therefore needs a semantic surface contract rather than a numeric search-and-replace: centered modals and visible cards/panels round and clip their decorative layers; transparent wrappers and viewport-edge drawers remain structural.

### Primary Page Surface Inventory

- The active global scale is already appropriately rounded: `6 / 8 / 10 / 14 / 18px`, with `18px` assigned to outer admin cards, toolbars, and table shells.
- Decision, task, demand, KPI, deconstructor, and Settings production pages consistently opt visible outer panels into `wa-admin-card`, `glass-panel`, `section-card`, or `config-audit-panel` classes. Their final light-console overrides resolve to `14px`–`22px` outer radii.
- KPI's earlier dark `.glass-panel { border-radius: 8px }` is superseded later in the same component by the global `18px` token; it is not an active sharp-corner gap.
- Settings' earlier `12px` surfaces are superseded by Phase 41/46 page rules using `14px`–`22px`, while zero-radius rules remain confined to transparent nested wrappers and flat inner sections.
- The shared design system should be hardened with `background-clip: padding-box` and isolation on visible surface primitives so translucent backgrounds, borders, and pseudo layers respect the declared radius without forcing overflow clipping on interactive containers.

### Implemented Surface Contract

- Shared `wa-panel`, `wa-admin-card`, `wa-admin-toolbar`, and `wa-admin-table-shell` surfaces now use `background-clip: padding-box` and their own isolation context, so translucent backgrounds and borders render inside the declared rounded geometry.
- The reusable shared modal now applies the same surface contract and explicitly rounds its top header layer in addition to retaining parent overflow clipping.
- Demand, schedule, detail, and confirmation modals now share a `20px` outer radius with `19px` inner edge radii on header/footer surfaces. This removes square corner protrusions while leaving dropdown and date-picker overflow available.
- No page-specific mass radius rewrites were needed: the audit found that visible Decision, Task, KPI, Deconstructor, Settings, and telemetry surfaces already use the rounded outer contracts; remaining small/zero radii are intentional inner geometry.

### Authenticated Browser Evidence

- Reused the existing signed-in local management-console page at `localhost:5173`; the latest source changes were already hot-reloaded.
- The new-demand modal computes to `20px` outer radius, `19px 19px 0 0` header radius, padding-box surface clipping, isolated painting, and intentionally visible X/Y overflow.
- Pixel-corner hit testing at all four `1px` corner insets resolves to the modal backdrop, while `10px` insets resolve to the rounded header/footer surfaces. This proves the former square child backgrounds no longer occupy the rounded cutout area.
- A current browser screenshot confirms the pale rectangular protrusions shown in the user screenshot are gone and the shadow now follows a visually continuous rounded silhouette.
- Schedule-page visible outer surfaces compute to `14px`–`20px`; the only `0px` hit is the deliberately flush inner table viewport nested inside a clipped `20px` table panel, so it has no exposed square outer corner.
- Decision-page metrics, table shell, inspector, and log panels compute to `16px`–`20px`, with no visible square outer surface.
- Evidence-page loading surface computes to the global `18px` outer radius; no square outer container is exposed while its data view refreshes.
- Task-page header, metrics, stage selectors, toolbar, table shell, and inspector consistently compute to `10px`. This is the intentionally tighter dense-workbench tier, still rounded on all four corners with no `0px` exposed surface.
- KPI route was observed in its loading state; no exposed card shell existed during that instant, while the component's final CSS audit confirms its populated `glass-panel` surfaces resolve to the global `18px` radius.
- Settings' visible audit surface computes to `8px`, a smaller nested-panel tier with all four corners rounded. Static inspection confirms its primary section cards retain the larger `14px`–`22px` outer tier.

## Controlled Autonomous Delivery Rollout — Initial Findings

- The repository already exposes deterministic intent recognition through `/api/ai/intent`, but the current handler does not yet call the configured LLM enhancement path.
- AI deconstruction already selects and persists a `context_pack_id`, and task import archives the normalized output with a trace.
- Imported AI work is represented as shadow execution tasks linked to a demand through `task_group_id`; this is the right compatibility boundary for autonomous execution.
- GitLab webhooks already record branch/MR evidence and trigger an AI MR review report, but the application does not create branches, commits, MRs, or assign reviewers.
- The new design should extend the existing strongest-brain and policy infrastructure instead of adding a separate cockpit or a second task system.
- Reviewer selection needs a two-stage contract: required roles/candidates/approval rules are frozen with the demand spec, while concrete people are resolved again from the actual changed paths before the MR becomes ready.
- Raw AI output, human edits, test evidence, review outcomes, and acceptance outcomes belong in an immutable execution/case record. Only curated, scoped, versioned candidates may enter the trusted context registry.
- The current worktree contained substantial completed UI and visibility work. It was checkpointed as commit `8f50f00` before this rollout.
- Database schema changes use GORM `AutoMigrate` in `internal/db/db.go`; the new delivery models can be added without a separate migration framework.
- Permission seeding is incremental. Super admins automatically receive every new permission, while admin/member grants must be selected deliberately.
- The existing authorization wrapper already maps permission prefixes to resource types and supports repository-scoped query parameters, so execution routes should reuse `withPermission` rather than invent another authorization layer.
- GitLab API support currently covers project discovery and webhook management through a small authenticated HTTP client. Controlled delivery should extract a reusable API client pattern while keeping tests on `httptest.Server`.
- `RepoMapping` already provides name, path, and project ID. That is sufficient to resolve a configured repository without adding another repository table in the first cut.
- The existing Go 1.22 `ServeMux` path variables are already used for version rollback and can safely support spec freeze and review-contract approval routes.
- Demand-spec lifecycle works cleanly as an extension of `TaskTelemetry`: the demand remains the root, each draft gets a monotonically increasing version, and frozen rows are immutable.
- Creating a new draft can invalidate only unstarted older runs (`pending` and preflight states) without touching executing or completed delivery evidence.
- Reviewer resolution is safest as a pure deterministic function. Protected-path candidates are selected first, then the general candidate pool fills the minimum approval count; the author and unavailable identities are excluded case-insensitively.
- The execution engine can validate explicit structured file actions without interpreting shell text. Relative path normalization, `.git`/parent traversal bans, duplicate detection, and encoding validation form the v1 change-set boundary.
- Preflight should re-resolve reviewers from the actual change paths and persist that resolution on the approved review contract; this preserves the early contract while allowing diff-aware final assignment.
- GitLab's repository commit API is a strong safety boundary for v1: the system submits a validated list of file actions in one commit instead of executing model-authored shell commands.
- Reviewer names must resolve to concrete active GitLab user IDs before branch creation; otherwise the run fails before any write-side effect.
- Run-key idempotency prevents repeated create/start requests from duplicating branches, commits, or MRs. Repeated status refresh remains append-only by using unique action-event keys.
- Automatic tests are represented by the GitLab Pipeline triggered by the pushed topic branch/Draft MR. The system records required test commands and refreshes the authoritative pipeline result instead of claiming local shell execution.
- AI code generation is bounded to supplied source snapshots for updates; it may create safe new paths, cannot delete files, and its JSON output must pass the same deterministic preflight validator before execution.
- Human acceptance and MR merge are separate facts. A run enters `acceptance_pending` after the named acceptance owner approves it, and only the GitLab merge webhook can close it as `delivered` when the pipeline is also successful.
- The existing demand-detail modal is the lowest-coupling UI integration point: it keeps specification, contract, run, Pipeline, MR and acceptance context attached to one demand without adding another cockpit or task surface.
- Moving the existing Deconstructor from the evidence page to the demand board preserves its richer editing workflow; importing reviewed shadow tasks now also creates a versioned spec draft from the archived context pack.
- Delivery feedback needs an explicit quarantine state. `CorpusCandidate.status=pending` is never selected by context-pack assembly; accepting a candidate creates a separate versioned, active Context Fact with archive provenance and a content hash.
- Candidate generation is idempotent by run/type/scope, so both a human-verification path and an MR webhook can safely request feedback generation without duplicates.
- Final regression confirmed the demand-options implementation intentionally exposes only configured core/JQL identities; the stale test was updated to assert that local non-core users and historical task owners remain excluded.
- A completed execution task without commit/MR evidence must produce `evidence_missing` even when its parent demand is independently due soon; parent schedule risk cannot suppress the stricter completion-evidence rule.

## Phase 50 Settings Column Height Synchronization

### Root Cause And Fix
- Phase 49 gave the audit pane its own sticky viewport height, so its bottom edge could not follow the left configuration surface.
- Simply changing the grid to `align-items: stretch` was insufficient: the long GitLab diff then contributed its intrinsic height and expanded both columns to `1984px`.
- The desktop audit panel is now absolutely contained by a stretched, zero-intrinsic-size grid item. The left configuration surface determines the row height, while version/diff overflow remains internal to the audit panel.
- At `max-width: 1280px`, the audit returns to normal flow with a bounded `640px` height so stacked layouts remain readable.
- Version and diff scroll regions now use the same `10px` WebKit geometry, light track, bordered teal thumb, hover color, Firefox `thin` width, and `scrollbar-color` as the global admin tokens.

### Browser Verification
- At `1600x900`, GitLab, Feishu, Jira, Project priority, and AI engine all report a `0px` primary/audit height delta.
- Measured paired heights are GitLab `1206px`, Feishu `552px`, Jira `707px`, Project `535px`, and AI `642px`.
- GitLab long diff content is `1558px` inside a `780px` scroll viewport; Jira's version history is `440px` inside a `209px` scroll viewport.
- At `1200x760`, both stacked columns resolve to `1153px` width, the audit is bounded at `640px`, the panel returns to relative positioning, and document horizontal overflow remains `0px`.

## Phase 49 Settings Adaptive Layout And Control Consistency

### Screenshot Findings
- The audit panel inherits the height of the much taller main configuration content, creating a large blank right column while the actual version list and diff preview are compressed into a narrow upper region.
- Version history already has its own scroll region, but the inspector grid fixes the list/detail rows too aggressively and constrains diff values to short nested scrollers; long arrays become unreadable raw JSON strips.
- The first Settings breadcrumb is a real button inside a globally styled breadcrumb surface, which visually creates a nested pill/card. Settings also owns this breadcrumb locally, so routes outside Settings never receive the same page context.
- GitLab overview explicitly renders `repoList.slice(0, 5)`, even though its count reflects the full configured repository list.
- Project editing mixes a bespoke searchable dropdown with the shared `Select`, so the three fields do not share one trigger, option, hover, and focus language.
- Select and combobox hover rules are duplicated across shared tokens and page-local CSS. Dark legacy hover declarations can still win in some contexts.
- Placeholder typography is independently overridden in several demand/task selection controls, producing sizes that are larger than surrounding control text.

### Implementation Direction
- Give the audit inspector a viewport-bounded sticky height, a bounded scrollable version list, and a flexible diff detail region with wrapped/preformatted values.
- Render one shell-owned breadcrumb/context bar for all primary routes; Settings supplies its subgroup and current child label through the shell contract instead of rendering another local breadcrumb.
- Render every configured GitLab repository and bound the table container with an internal vertical scrollbar.
- Use shared `Select` for Project phase/priority/weight and align the searchable Project Key picker to the same visual tokens.
- Lock light hover/open backgrounds and 12px placeholder typography in shared input/select tokens, then remove stronger page-local exceptions where needed.

### Browser Verification
- The QA entry now mounts the real `SettingsPanel` with deterministic config/version fixtures instead of a hand-authored audit placeholder.
- At 1600 x 900, the Settings grid resolves to `1119px 420px`; the audit inspector is sticky, 720px high, and the diff region scrolls internally (`1558px` content inside a `294px` viewport).
- At 1280px the audit inspector moves below the main content at full width, avoiding a compressed side column.
- At 390px the grid resolves to one 359px column with no document-level horizontal overflow; the audit workspace stays bounded at 620px.
- A Jira fixture with nine visible versions produces a 189px/440px internally scrollable version list on narrow screens.
- A GitLab fixture with twelve configured repositories renders all twelve rows; the repository table is 358px high with 690px scroll content.
- Project phase and priority use the shared non-search Select, while Project Key uses the shared searchable Select. The open trigger and menu compute to white/light surfaces, and the Project Key placeholder computes to 12px.
- The async Settings summary was corrected so breadcrumb meta and header facts now both report `12 个仓库` after config load instead of retaining the initial zero state.

## Phase 48 Unified Settings Main-Content Rebuild

### Correction
- The latest implementation completed a page-level rebuild for `GitLabConfig.svelte` only.
- `SettingsPanel.svelte` grid/audit tuning and `Switch.svelte` token changes affect multiple routes, but are shared infrastructure rather than page-level rebuilds.
- Jira and Project contain earlier substantial local changes; Feishu and AI contain smaller earlier changes. None should be treated as aligned with the new GitLab workbench until audited against the same structure and states.

### Locked Direction
- Product UI, calm precision, light management console, `SPECTACLE=2`, `DENSITY=8`.
- Preserve API contracts, field names/order, save behavior, left-rail IA, and the existing teal accent.
- Prefer flat sections and sparse dividers over nested cards; use labels above controls, visible status language, fixed action placement, and explicit loading/empty/error/disabled states.

### Route Inventory
- Integration/AI child components: GitLab, Feishu, Jira, Project priority, AI engine, and AI context/system-design corpus.
- SettingsPanel-owned routes: KPI, users, permission matrix, authorization policies, and audit.
- The user-visible left rail exposes all 11 routes; therefore visual verification must cover all 11 even when only six use child config components.
- `SettingsPanel.svelte` already owns the common breadcrumb, route summary, main/audit split, and several non-integration page implementations.

### Child Page Audit
- Feishu and Jira still use the legacy `Steps` wizard and full success-screen pattern; both overview pages can report `已就绪` before a health check has actually run.
- Feishu and Jira duplicate overview rows, credentials, summary cards, and action CSS instead of sharing the GitLab workbench language.
- AI engine and system-design corpus share one very large component with two conditional surfaces; engine editing still uses the legacy wizard while the corpus has its own nested registry/editor visual system.
- The existing data/test/save functions are usable and should be preserved; the main work is state semantics, markup hierarchy, and shared presentation.
- Project priority is a list/editor page with useful loading, empty, error, and success behavior, but its markup still uses the old card-header/table/form vocabulary and relies on a long stack of local CSS resets.
- A shared, prefixed workbench stylesheet is lower risk than another round of broad `:global(... !important)` overrides: new markup can opt into the contract while legacy selectors become inactive.

### Shared Workbench Contract
- Root: one full-width `scw-workbench`, no nested page card.
- Overview: compact header with labeled switch, explicit status banner, two-column read grid, optional fact/table section, and one bottom action bar.
- Edit: compact segmented stepper, flat bordered sections, two-column field grid, inline test feedback, and a stable bottom action bar.
- Data pages: title/action toolbar, four-column summary strip, normal-density table, composed skeleton/empty/error states, and inline editor surface.
- Status semantics: `success`, `error`, `incomplete`, `checking`, and `unchecked`; never infer `ready` from the absence of an error.
- Shared `Button`, `TextInput`, `Select`, and `Switch` already expose the required labels, focus treatment, disabled states, and semantic variants; the rebuild should compose these controls rather than duplicate their CSS.
- The workbench stylesheet only needs to own page hierarchy, grids, tables, status surfaces, native textarea/range fields, and action placement.

### AI Surface Findings
- AI engine `isConfigured` currently includes active context-fact count, so an engine with no endpoint can incorrectly enter the configured overview just because the corpus has data. Engine configuration state should depend only on engine fields.
- The engine overview also defaults to `已就绪` without a connection test; it needs the same explicit unchecked/incomplete/error/success model as GitLab, Feishu, and Jira.
- The system-design corpus already has real loading, empty, retry, edit, save, preview, and result states. Its behavior can stay intact while the registry/list/editor shell is flattened into the shared workbench.

### Visual QA Path
- Production preview and the existing development tab both require authentication; no credential submission or auth bypass is appropriate for this task.
- Vite already supports a separate multi-page prototype entry, so a second local-only `settings-preview.html` can mount the real config components with deterministic mock API responses.
- The QA page remains outside production navigation and validates component pixels, overflow, responsive collapse, and edit-state transitions without changing authentication behavior.

### Desktop Visual QA
- The standalone QA entry mounts the real GitLab, Feishu, Jira, Project, AI engine, and system-design components with deterministic fixture responses; it does not duplicate their markup.
- GitLab, Feishu, Jira, Project, and AI engine overview/edit states have no document-level horizontal overflow at 1280 x 720.
- Project keeps the three-row table within its own content width and the edit form resolves to two balanced columns.
- Feishu and Jira keep the four-step editor inside the workbench; their status banners remain explicit instead of reporting a successful connection by default.
- AI engine summary metrics resolve to three equal columns. The two endpoint choices now use an explicit two-column variant so their track count stays deterministic.

### Narrow-Viewport Finding
- At 390px, Feishu, Jira, Project, AI engine, and system-design overview states initially stayed within the document width; Project intentionally scrolls only its table container.
- GitLab exposed a real component defect: an inherited grid min-content track expanded a 329px workbench to 471px. The production component now clamps the root and first-level grids with `minmax(0, 1fr)`, `min-width: 0`, and `max-width: 100%`.
- After the correction, all six integration/AI routes remain within a 390px document width. Feishu's four-step control and Project/GitLab tables keep overflow inside their own bounded containers.

### SettingsPanel-Owned Routes
- KPI, members, permission tree, authorization policies, and security audit are not newly rewritten in Phase 48. They already contain page-specific Phase 46 structures (KPI surface, member table, permission tree, policy builder/explainer, and audit tables) rather than receiving only shell decoration.
- Current static review confirms those five routes continue to use the shared flat section/table/control rules. Phase 48's new page-level implementation scope is the six integration/AI routes.

## Phase 45 Settings Surface System Rebuild

### Requirements
- Rebuild every page under the Settings menu as one coherent product surface.
- Align spacing, typography, controls, states, and responsive layout across all configuration modules.
- Use restrained frosted glass at page-level surfaces only.
- Do not nest capsule-like containers around switches, inputs, or other controls.
- Preserve existing APIs, permissions, configuration behavior, and version rollback flows.

### Initial Visual Findings
- The supplied Jira screenshot has too many independent rounded outlines: page frame, summary panel, metric tiles, field tiles, JQL box, version cards, and inner diff boxes all compete at the same visual level.
- The toggle is visually isolated inside its own rounded halo instead of belonging to a clear section header/action row.
- Configuration facts are rendered as equal cards even when they are simple label-value rows, producing a card-in-card rhythm and wasting vertical space.
- The audit column is narrow but contains nested cards and a second scroll region, making the hierarchy cramped and inconsistent with the main panel.
- Frosted glass should separate page regions, not be applied recursively to every field.
- The previous Phase 45 convergence block explicitly applied glass, borders, and radius to field groups, credentials, registry items, project items, version items, and diff rows; this directly caused the nested-container appearance.
- Integration modules were additionally wrapped by `section-card`, creating a redundant surface around child `wizard`/`project-config-container` roots.

### Technical Direction
| Decision | Rationale |
|----------|-----------|
| Use a flat section + field-row model | Removes nested capsules and makes scanning/editing faster |
| Keep one page-level glass surface per column | Preserves depth without gray overlays or card nesting |
| Use a single teal accent and cool neutral scale | Matches the existing shell while avoiding purple/gray drift |
| Keep version history as a compact inspector | Audit remains available without competing with configuration work |
| Keep legacy root class during migration and add a later Phase 46 contract | Prevents Svelte unused-selector noise while the new contract wins by cascade order; visual behavior is owned by Phase 46 selectors |

### Browser Verification Findings
- GitLab, Feishu, Jira, AI engine, and project summaries render as flat label-value lists inside one primary glass surface.
- Jira and AI edit modes render field wrappers with transparent backgrounds, zero radius, and zero outer borders; only actual controls retain control styling.
- Version history is a flat selectable list with the selected version detail below it; it no longer compresses detail into a narrow nested column.
- The context library uses a flat list/editor split with one divider; option sets are borderless grids without an outer capsule.
- Member, permission, policy, audit, and KPI routes contain no dark surface drift; the permission branch card and decorative side stripe were removed.
- At 1280px the project table receives full width and the audit inspector moves below, eliminating horizontal table scroll.
- At 760px the global navigation is an off-canvas drawer and the configuration page remains visible behind it.

### Pre-flight Evidence
- One teal accent is locked across configuration routes; status colors are semantic exceptions only.
- Glass is limited to breadcrumb, page header, primary work surface, and version inspector.
- Gradient text was removed from GitLab, Feishu, AI, and project success/header states.
- Browser metrics found no content overflow at 1024px or 760px and no decorated field wrappers.


## Requirements
- Phase 41 correction: user paused the rollout because the current implementation drifted away from the original prototype style and data-chain intent.
- Phase 41 execution model: recalibrate first, then use subagents; one common agent owns shared components/data contracts, and one page agent owns each page slice.
- Phase 41 visual baseline: `output/functional-admin-console-reference.png` is the primary target because it uses `well-ambient`; `functional-admin-console-reference-alt.png` and `modern-admin-reference-phase40.png` can inform layout but must not reintroduce `最强大脑`.
- Phase 41 protection rule: page agents must not invent new page-local visual systems; they must consume the shared prototype-aligned contract from the common agent.
- Current Phase 40 feedback: prior prototype passes are still not beautiful or comfortable enough; use `finesse-skill`, `taste-skill`, and `ui-skill` together, generate a visual prototype first, then implement the chosen direction.
- Phase 40 output requirement: the prototype must be directly visible without login so visual quality can be judged before migrating existing authenticated business pages.
- Current Phase 37 requirement: execute the recommended UI redesign implementation order by starting with audit, coded prototype, componentized design substrate, safe app-shell exposure, and frontend validation.
- Phase 37 UI protection rules: do not revive the rejected visible decision queue, do not revive the visible delivery cockpit, do not remount project health telemetry, and do not rewrite existing business pages until the prototype proves a concrete operator workflow.
- Phase 37 design read: `研发协同管理台 · restrained glass + evidence-first · register=product · SPECTACLE=2 · DENSITY=9`.
- Current rollout requirement: commit all uncommitted code, then use subagents to start implementing every phase from `docs/strongest-brain-delivery-transformation-plan.md`.
- New baseline commit before rollout: `adcc981 chore: capture strongest brain baseline`.
- Current worker split: Worker A owns backend evidence/exception/weekly decision read-model helpers, Worker B owns AI trace/readiness/context-pack backend helpers, Worker C owns frontend delivery cockpit integration.
- Phase 26 rollout is now implemented as a cross-phase delivery cockpit plus supporting read models, not as a revived visible decision queue.
- Check and repair the repository according to `/Users/eddie/.gemini/antigravity/brain/ffc6d8a3-38c0-4a9b-a458-9b51beeff27b/implementation_plan.md`.
- Planned areas: confirmation modal styling, WellOS department extraction, select dropdown truncation, AI deconstruction to demand association, demand card shadow task progress, targeted tests/builds.
- Follow-up requirements: require `taste-skill` for frontend UI work, explain/deepen demand scheduling and AI deconstruction binding, fix user department display, and fix the native-looking new-demand due-date style.
- Current extension requirement: generate a concrete landing plan from the architecture recommendations and use self-agents to land GitLab/Jira integration, KPI report preview, and AI deconstruction expansion in bounded implementation tracks.
- Current task addendum: compact the work-summary effort/AI-evaluation controls, optimize the config version audit/rollback surface, lock background scroll under modals, repair assignee/Jira/detail interactions across decision and demand boards, and make demand/task/bug status transitions feel seamless with strongest-brain recommendation guidance.
- Current backend worker scope: implement the strongest-brain v1 backend read-model loop for EvidenceChain, DecisionQueueItem, schedule governance reuse, and AI intent recognition/summary, limited to backend server/db/telemetry files and focused tests.
- Current Phase 22 requirement: based on `docs/strongest-brain-phased-master-plan.md`, add AI intent recognition and summarization to the plan and implementation, checkpoint all existing code first, then use subagents to land the complete strongest-brain plan as a v1 closed loop.
- Current Phase 23 continuation: proceed into master-plan Phase 4 by adding a schedule-governance risk calendar read model and compact schedule-view surface, while preserving the existing `/api/schedule` contract.
- Current Phase 24 feedback: the visible "决策队列" does not yet communicate its display meaning; improve the cockpit so it shows why an item matters, what decision is needed, what happens if it is ignored, and where the operator should act.
- Current Phase 25 feedback: the user still judges the visible "决策队列" as not useful; remove that visible surface instead of continuing to iterate on it.
- Current Phase 27 feedback: the visible strongest-brain delivery cockpit still does not prove its existence value; temporarily remove it and continue other phases through smaller, independent APIs.
- Current Phase 28 feedback: refine the visible schedule/evidence UI by fixing red-zone card height and Jira-line density, upgrading the schedule risk calendar's interaction quality, and moving demand deconstruction from evidence observation to schedule governance.
- Current Phase 29 bug: refreshing the decision dashboard briefly renders Git branch/commit-derived cards in the red-zone grid before config/member filtering removes them.
- Current Phase 30 direction: optimize project health telemetry into an exception triage surface and add explicit manual intervention directions for red/yellow projects.
- Current Phase 31 feedback: the visible project telemetry panel still does not show enough practical value; hide it from the evidence observatory for now.
- Current Phase 34 feedback: KPI page clicks cause visible jumping, and the dashboard needs more depth and breadth beyond the first score/matrix refactor.
- Current Phase 35 bug: "Jira Task 开发结果追踪" showed stale child execution assignee/status when the bound Jira Task had already been completed or reassigned.

## Research Findings
- Phase 41 primary visual baseline is a management console with dark left rail, light mist workspace, white frosted cards, compact metric cards, table-first main surface, right inspector, low-saturation shadows, teal/green accent, and brand text `well-ambient`.
- The later dark/glass per-page edits drifted because production pages were allowed to define local visual systems rather than consuming one shared prototype contract.
- Current `DESIGN.md` still describes the older dark-tech cockpit direction; it conflicts with the Phase 41 prototype-aligned target and needs an explicit override or update.
- Phase 41 common contract now supersedes the old dark cockpit contract: production pages should consume `web/src/lib/admin-console/contract.ts`, `docs/admin-ui-calibration.md`, and the `.wa-admin-*` token classes rather than inventing local visual systems.
- Decision, Schedule, Task, Evidence, KPI, and Settings now have production-page Phase 41 shells while preserving their existing API/data chains.
- The Settings worker stalled without output; the main thread completed `SettingsPanel.svelte` with a container-level adapter and shell to avoid blocking the page rollout.
- `SettingsPanel.svelte` still embeds legacy config child components; those are intentionally preserved for functionality and can be handled as independent child-component polish later.
- Settings IA correction: multi-area settings pages should use grouped submenus in the left nav, and the right work area should be one unified frame with breadcrumb navigation above the actual content.
- Settings rail correction: grouped Settings submenus must belong to the global `FunctionalAdminShell` rail/menu, not to `SettingsPanel.svelte`'s main content frame.
- Phase 44 correction: UI-related work now defaults to `ui-skill`, then falls back to `finesse-skill` and `taste-skill`; this replaces the older finesse-first/taste-only assumptions in local task memory.
- Phase 44 main-content rule: production pages should not render a module hero inside `FunctionalWorkspace` by default. The topbar is the page identity layer; content should begin with real metrics, compact controls, tables, and inspectors.
- Phase 44 visual correction: migrated page-level headers should behave like low-height toolbars, not landing-page or cockpit hero blocks; explanatory copy is hidden where it competes with the operational content.
- The previous Settings right side mixed hero, metrics, category table, module content, and inspector at one level; that made the information hierarchy unclear and conflicted with the requested management-console pattern.
- Phase 41 validation passes for TypeScript app build and Vite production build; remaining frontend warnings are existing repository-wide warnings, mostly in `DemandKanban.svelte`, shared modal/switch, and config child components.
- Phase 40 skill review result: `taste-skill` is not the primary dashboard implementation skill, but its image-first and anti-generic rules are the right correction for a visual-quality failure; `ui-skill` should govern component hierarchy, and `finesse-skill` should remain the product UI audit guard.
- The generated reference image validates a stronger visual direction than Phase 39: light workspace, dark translucent rail, flat information surfaces, restrained frosted glass, comfortable whitespace, task table, and right inspector.
- Keeping the new prototype at `/prototype.html` removes the previous visual-review blocker where the app landed on the login page before the `UI 原型` tab could be inspected.
- Vite needs explicit multi-page build inputs for `prototype.html`; otherwise the dev server can serve the file but production build only emits `index.html`.
- Phase 37 should be treated as product UI, not a brand/landing surface: density, tables, command rows, evidence panels, and clear interaction states matter more than hero spectacle.
- The current frontend has several large Svelte components with local style systems (`DemandKanban`, `SettingsPanel`, `TaskKanban`, `Deconstructor`, `DecisionDashboard`, `KPIKanban`), so a safe redesign starts by adding shared tokens/prototype surfaces rather than editing all pages at once.
- `App.svelte` currently uses top tabs for decision, schedule, evidence, and settings; the lowest-risk prototype insertion point is an additional permission-gated preview tab rather than replacing existing pages.
- The existing visual direction overuses emoji labels, gradients, and local glass panels; Phase 37 should replace those patterns in the prototype with restrained semantic status, compact command bars, fixed-density rows, and purposeful acrylic only on shell/overlays.
- Git status shows the repository contents are currently untracked; avoid treating that as disposable state.
- Phase 22 baseline checkpoint commit created before new implementation: `2a7890c chore: checkpoint current workspace changes`.
- Phase 22 uses three subagents with disjoint scopes: backend strongest-brain/AI intent API, frontend cockpit/AI conversation UI, and QA integration review.
- The active frontend design read is an internal研发协同 dark cockpit, not a marketing or landing surface; density should remain operational and UI copy should be plain, auditable, and concise.
- Phase 22 landed a v1 strongest-brain loop: master plan addendum, decision queue API, evidence chain API, deterministic AI intent/summary API, cockpit decision queue UI, and transparent deconstructor conversation panel.
- Full server package validation is still sandbox-limited because existing GitLab webhook tests bind a local `httptest` listener; targeted Phase 22 handler tests pass without listener binding.
- Phase 23 should reuse `buildScheduleResponse` and `resolveScheduleRisk` instead of creating a second risk taxonomy; the new calendar is a read-model projection over existing schedule facts.
- Current dirty `task_status.md` and `well-ambient.db` are not part of the Phase 23 scope and should remain unstaged unless explicitly requested.
- Phase 23 subagents timed out and were closed; frontend partial work appeared in the shared worktree and was integrated by the main agent, while backend/API/test work was completed locally.
- The Phase 23 risk calendar now returns additive week/month buckets plus event summaries, while the schedule table still consumes the existing `/api/schedule` contract.
- The decision queue API already returns source-level items from schedule, execution, and context read models, but the frontend normalized every API item as `strongest_brain`, hiding the item's origin and making the queue feel like a generic list.
- The existing queue cards had problem/evidence/action text, but no explicit current focus, no idle-cost language, and no clear handling entry for items that do not map to the old Agenda intervention panel.
- After adding meaning language, the user still found the strongest-brain decision queue surface useless. The better product move is subtraction: remove the surface and leave the actionable Agenda metrics/filter/intervention flow as the decision dashboard's main workflow.
- Removing the frontend queue fetch also stops the decision dashboard from polling `/api/strongest-brain/decision-queue` every 15 seconds when the visible queue is gone.
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
- WellOS login no longer provides the authoritative avatar and department fields directly. After login succeeds, the backend must call `GET /api/user/info?token={token}` and use `data.avatar`, `data.realname`, and `data.department_name`.
- Demand schedule rows must distinguish workflow status from schedule completeness. `backlog` can mean a demand is waiting or planned, but it is not proof of schedule unless the row has both a development branch and a due date.
- The schedule table should keep schedule status, effort estimate, delivery evidence, risk, and update time in separate columns. Mixing branch/repo placeholders into the schedule column makes unscheduled work look scheduled.
- AI effort estimation can reuse the existing `/api/deconstruct` contract for the scheduling MVP. The schedule modal should treat it as an optional estimate baseline and persist the selected overall hours/days/difficulty only when the user confirms scheduling.
- Current code already contains partial fixes for this task: `modalScrollLock.ts`, scroll locking in shared Modal, DemandKanban and SettingsPanel manual modal locks, compact schedule effort controls, demand assignee editing UI, and a page-filtered config-version audit panel.
- Demand and Task boards currently fetch Jira base URL through `/api/config`, which requires `config:read`; normal demand/task users can therefore lose the Jira direct-link affordance even when Jira is configured.
- Demand assignee editing can fail for creators/assignees who are allowed to call `/api/demands/reassign` but cannot call `/api/demands/options`, because that route currently requires `demands:write` and the dropdown then only contains the current assignee.
- Agenda decision reassignment writes `TaskTelemetry`, decision logs, and kanban markdown, but Jira sync later unconditionally restores the Jira assignee. This explains the user-visible "success but no effect" behavior after a background sync.
- The config version audit panel now filters by section, but selection can still point at a version from another section and the two-column diff layout consumes too much height on integration pages.
- Final Phase 21 fix: expose only Jira `base_url` through authenticated `/api/jira/link-config`, allow authenticated demand users to load assignee/project options, preserve recent local assignee override logs from Jira sync for 24 hours, compact the schedule estimate strip, make config-version audit section-specific and lower height, and use card-level detail fallback on demand cards.
- Current Phase 22 backend constraint: this worker should not modify frontend files and must not revert changes made by parallel agents.
- Worker B Phase 26: `db.DeconstructArchive` already stores `context_pack_id`, `input_text`, deconstruction output JSON, confidence, completeness, and archive timestamps; `db.ContextPack` stores model, prompt template version, token budget/count, summary, hash, and item count.
- Worker B Phase 26: `/api/deconstruct` currently assembles and persists a context pack and returns `context_pack_id`, but it does not create a deconstruction archive because import/demand linkage happens later.
- Worker B Phase 26: before this slice, `/api/tasks/import` archived deconstruction output with `context_pack_id` and auto-built an `import_archive` context pack only when the request omitted one but included `input_text`; the MVP now also derives an input snapshot from imported tasks and returns archive/trace metadata.
- Phase 26 backend now exposes a consolidated strongest-brain delivery cockpit that aggregates evidence health, exception-only progress signals, weekly decision questions, schedule risks, AI trace metadata, override audit events, authorization explanation summaries, and concrete entry points.
- Phase 26 evidence read models now distinguish completed-without-evidence, merged-but-not-completed, scheduled-but-stale, due-soon, overdue, missing links, chain status, and evidence completeness.
- Phase 26 AI trace work keeps every generated recommendation replayable by `archive_id`, `context_pack_id`, prompt/source version metadata, confidence, completeness, selected context facts, and deterministic missing-question buckets.
- Phase 26 frontend initially mounted a delivery cockpit above the existing decision dashboard, but Phase 27 removes that visible surface because the user still does not see its value.
- Backend aggregate read models remain available for reuse, but user-facing strongest-brain work should proceed as focused phase-specific pages or APIs rather than a single all-in-one cockpit.
- Phase 2 can now consume `/api/strongest-brain/exceptions`, which returns an exception-only response with summary counts, severity, owner, deadline, evidence refs, missing links, close requirements, chain status, and evidence completeness.
- Phase 3 can now consume `/api/strongest-brain/weekly-decisions`, which returns decision-meeting mode data with weekly decision cards, options, evidence refs, owners, deadlines, decision debt, and must-decide counts.
- The strongest-brain decision queue remains a backend compatibility/read-model source, but the user-facing direction should not be to repackage it as a generic queue or cockpit.
- The first modern-admin prototype still read like a static concept board; the next iteration needs component boundaries, real operator controls, and a dense table-first workflow to satisfy the management-console requirement.
- For this admin UI, glass should be limited to shell and inspector separation. The main work surface should stay flat, scan-friendly, and table-oriented.
- The prototype should prove the exception-to-mediation loop before migrating existing production pages: command table, preflight drawer, evidence chain, workflow path, and weekly decision queue are the minimum useful slice.
- The componentized prototype became more usable but still lacked a memorable visual center; a bolder product UI needs a first-screen theatre that makes the selected issue, risk, evidence gap, and action intent visually unavoidable.
- Product UI can be bold without becoming a landing page when spectacle is bound to real state: ambient grids, acrylic edge depth, status signal glow, and a selected-issue theatre should point to operational facts, not decoration.
- The red-zone diagnostic cards need fixed row sizing and a compact identity header because variable title/project rows make the upper/lower card heights feel uneven.
- The schedule risk calendar should behave like a governance command surface: show the current focus, give direct risk filter actions with counts, keep refresh visible, and make each time bucket scan-friendly.
- Evidence observatory should focus on facts, links, health telemetry, and collaboration evidence. Demand deconstruction produces readiness, estimates, subtasks, and schedule inputs, so it belongs in the schedule governance page.
- The red-zone refresh flicker is caused by agenda data loading before config-driven core-member filtering settles; lower-case branch slices such as `revert-4` and `with-5` can pass the default in-memory member list for one paint.
- The red-zone decision grid should only render stable Jira issue keys. Git branch names and internal shadow-task prefixes can still exist in telemetry, but they should not appear as story cards.
- Project health telemetry should answer "which project needs human intervention now" before showing raw score math. The primary surface now ranks red/yellow projects ahead of green projects and exposes the top exception as the command focus.
- The four health dimensions map naturally to four manual intervention directions: schedule truth confirmation, evidence-chain repair, ownership/load mediation, and quality/regression triage.
- The first health-telemetry optimization can stay frontend-only because `/api/projects/scores` already provides enough SH/EQ/CE/SI inputs to derive health level, weakest dimension, evidence gap, and decision question.
- The detail modal is more useful as an intervention dossier than a metric explanation page: it now opens with urgency, weakest dimension, evidence gap, and a human action before showing formula details.
- Project avatars and priority cards should carry the same red/yellow/green state language as PHDI. This avoids another decorative identity treatment and keeps the page visually tied to operational severity.
- Project health telemetry remains conceptually useful only if it has a concrete operator workflow. Until that workflow is proven, the evidence observatory should not spend first-screen attention on the panel.
- Hiding the project telemetry mount is safer than deleting the component because the scoring and diagnosis work can either be reworked into a sharper workflow or removed in a later cleanup with less urgency.
- KPI 研发绩效大盘 should treat daily and weekly reports as first-class review grains, not as one option in a generic time-range selector.
- The existing KPI API already exposes enough fields for a first explainable score: completed tasks, demands, bugs, MR count, review count, cycle days, active overdue, and overdue completed.
- A member matrix is more useful than ranking cards for this workflow because operators need to compare task/Bug volume, code evidence, cycle time, and risk side by side.
- The score should be shown with its component breakdown so it remains an auditable evaluation aid instead of an opaque performance verdict.
- KPI aggregation previously ignored the coreMember boundary, so non-core assignees could still affect summary totals and report evidence even if other dashboards filtered them.
- `jira.sync_users` is the clearest coreMember source; custom JQL `assignee in (...)` remains a useful fallback for deployments that encode the team in JQL only.
- Core member filtering must happen before KPI aggregation, otherwise user rows may be hidden while totals, departments, risks, or evidence still leak non-core work.
- Core member matching needs aliases because Jira usernames, WellOS/local display names, and email prefixes can refer to the same person.
- KPI click jumping came from two UI behaviors: report-mode refreshes could replace the whole mounted dashboard with the full loading state, and active button feedback used `transform: scale(...)`.
- The remaining daily/weekly jump came from a second layer of layout shift: the sync indicator was still conditionally inserted into the header, and the report/member sections could resize substantially when daily and weekly data had different row counts.
- A useful KPI second layer should diagnose team shape, not just add more totals: delivery concentration, evidence coverage, risk load, review pressure, Bug share, and score spread explain whether the score is healthy.
- KPI breadth should show distribution across work type, score bands, departments, and cycle time so managers can compare team balance without manually reading every row.
- Execution tracking rows already knew their parent demand through `task_group_id`, but owner/status/result calculations ignored the parent Jira Task demand facts.
- In this deployment Jira Task is the demand主线, so its current Jira assignee/status must drive execution tracking display for bound rows; child execution task assignee remains useful only as secondary execution evidence.
- Filtering execution rows by assignee before DTO construction can lose rows after Jira reassignment, because SQL only sees the stale child execution assignee.

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Add a coded prototype before full UI migration | The current pages are business-critical and large; a prototype can validate IA, density, visual language, and component vocabulary without breaking existing workflows. |
| Gate the prototype as a preview surface instead of replacing the current dashboard | The user asked to start implementation, but previous broad cockpit surfaces were rejected; a preview lets us evaluate the new management-console direction safely. |
| Use restrained glass as a material layer, not decorative glassmorphism everywhere | Product UI needs scannability and contrast; acrylic should separate shell, panels, and overlays rather than turn every card into a glossy object. |
| Keep the prototype focused on exception, schedule, evidence, and override workflows | These map to the strongest-brain closed loop and avoid reviving generic queues or low-value health dashboards. |
| Refactor the prototype into local components before touching production pages | The first prototype's weakness was structure as much as visuals; Shell, Metric, Badge, Table, and Inspector components make the desired admin vocabulary explicit and reusable. |
| Make the command table the primary prototype surface | Operators need to compare risk, owner, stage, due date, evidence, and next action in one scan; card-first layouts are too slow for this workflow. |
| Keep the right-side drawer as mediation preflight rather than generic detail | The ultimate value is shortening human intervention, so the selected item should expose reason, close condition, evidence state, route, audit trail, and direct actions. |
| Add a first-screen Theatre component for the bolder prototype pass | The user asked for beauty and boldness; the Theatre gives the product UI a memorable visual center while still carrying selected issue facts and actions. |
| Raise spectacle through substrate and data-state visualization, not through a marketing hero | The page is still an admin console, so boldness should come from acrylic depth, ambient grid, status signals, and operational hierarchy rather than decorative copy or unrelated imagery. |
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
| Keep user-info refresh best-effort after successful auth | A temporary user-info failure should not reject a valid WellOS login, but successful responses must refresh DB, JWT, and frontend payload profile fields. |
| Use row-level `scheduled` for schedule table truth | A boolean contract avoids frontends and future pages reinterpreting `backlog` as “已排期” without branch and due-date evidence. |
| Keep AI schedule estimation optional | AI configuration failures should not block a normal schedule edit; the estimate is useful context and persisted telemetry, not a mandatory gate. |
| Preserve local manual assignee overrides from Jira sync for 24 hours | A decision-panel reassignment should visibly take effect immediately, while still allowing Jira to become authoritative again after the handoff window if no matching Jira update arrives. |
| End local assignee override protection when Jira reports `done` | Completion is a stronger Jira fact than an active handoff window; the execution tracking board must not keep a stale local owner after Jira has finished and reassigned the issue. |
| Keep recent completed Jira issues in correction sync | The production custom JQL excludes `done`, so completed issues need a bounded key-based keep-alive window to capture post-completion owner changes and reopen facts. |
| Expose Jira link metadata through a non-secret authenticated endpoint | Demand/task users need to open Jira issues, but should not require full `config:read` access or receive integration tokens. |
| Build the strongest-brain queue as an aggregation layer over schedule and execution read models | This lands a useful v1 loop without duplicating risk logic already covered by `/api/schedule` and `/api/execution/tasks`. |
| Use deterministic AI intent/summary fallback first | The conversation surface must remain explainable and usable when AI providers are disabled, misconfigured, or slow. |
| Fetch Jira link config before full config in the decision cockpit | Decision users need issue links, but should not depend on `config:read` to render the queue. |
| Implement risk calendar as additive endpoint | `/api/schedule` is already consumed by the table and tests; a new `/api/schedule/risk-calendar` avoids breaking existing filters and virtualized row rendering. |
| Keep risk calendar as a read-model projection | Phase 4 needs visible schedule governance, but risk rules should still come from the existing schedule resolver until a unified event ledger lands. |
| Let the frontend fall back to current schedule rows | Operators should still see a useful risk calendar if the new endpoint is temporarily unavailable or deployed behind the frontend. |
| Treat the decision queue as a cockpit projection rather than a new backend contract | The fastest useful fix is to reveal existing queue source, decision kind, idle cost, and entry state in the frontend without changing `/api/strongest-brain/decision-queue`. |
| Keep non-Agenda queue items visible but honest about their entry | Schedule/execution/context items may not be actionable in the old intervention form, so the UI now labels them as source-located work instead of pretending every card can be handled by the same control. |
| Remove the strongest-brain decision queue only from the frontend dashboard for now | The user rejected the visible surface; leaving backend APIs intact avoids unnecessary churn to Phase 22 contracts/tests while removing the useless UI and polling. |
| Worker B Phase 26 uses `DeconstructArchive + ContextPack` as the AI trace MVP source of truth | The existing schema already stores archive output, input snapshot, confidence, context pack metadata, prompt template version, and selected pack items, so a read model avoids new migrations during the parallel rollout. |
| Worker B Phase 26 leaves trace/clarification route registration to the main agent | `server.go` is being edited by another worker; helper handlers are implemented but not registered to avoid route ownership conflicts. |
| Worker B Phase 26 derives missing-question tiers deterministically from deconstruction analysis | `missing_info` becomes must-answer, dependencies become can-defer, and meeting questions are classified by readiness/keywords so tests stay stable without external AI calls. |
| Implement the strongest-brain "all phases" request as one evidence cockpit plus focused read APIs | The pain points are connected by the same proof chain, so the first landing slice should consolidate status, decisions, readiness, schedule risk, override audit, traceability, and permission explanation rather than scatter new pages. |
| Keep the old decision queue hidden | The user explicitly rejected that visible surface; the new cockpit surfaces only exception and decision prompts with evidence and entry points. |
| Keep AI trace/readiness deterministic for MVP tests | Deterministic read models make replay, audit, and missing-question behavior stable without depending on external model calls. |
| Remove the visible delivery cockpit for now | The user still does not see its existence value; continuing to defend the surface would add UI noise, so only the reusable backend foundations stay. |
| Split Phase 2 and Phase 3 into independent APIs | Exception handling and weekly decisions have clearer product contracts than a broad cockpit, and future UI can be purpose-built around each phase. |
| Reuse one strongest-brain decision snapshot for queue, exceptions, and weekly decisions | Shared evidence/risk logic prevents three endpoints from drifting on severity, owners, missing links, and evidence completeness. |
| Move demand deconstruction into schedule governance | The deconstruction output feeds scope clarity, estimate confidence, missing questions, and schedule readiness rather than evidence observation. |
| Make the risk calendar filter row count-first | Operators decide by severity/count first, so all risk chips expose totals and switch the calendar view directly. |
| Gate red-zone agenda rendering on config readiness | Prevents the first paint from using the broad default member list before `/api/config` has had a chance to narrow the visible scope. |
| Require stable Jira issue keys for red-zone cards | Lower-case Git branch slices and internal generated task IDs are telemetry evidence, not decision-board story cards. |
| Treat project health telemetry as an exception triage surface | The panel should reduce daily progress chasing by ranking projects that need human action, not by making operators compare raw score columns. |
| Derive manual intervention from the weakest health dimension | This gives every red/yellow project one concrete action direction while preserving the existing scoring API and avoiding premature backend expansion. |
| Keep the health telemetry refactor frontend-only for the first slice | Existing score fields already support the intervention model; backend changes should wait until interventions need persistence, ownership assignment, audit, or rollback. |
| Remove decorative identity gradients from project health rows | Severity color carries operational meaning here, so project identity should not compete with red/yellow/green health status. |
| Hide the project telemetry panel until its workflow value is proven | A visible dashboard module should either drive an operator action or stay out of the way; the evidence observatory keeps the task/code evidence surface while telemetry remains dormant. |
| Preserve the telemetry component instead of deleting it immediately | The user asked to hide it temporarily, so the safer move is to unmount the surface and leave the component available for redesign or later cleanup. |
| Make KPI daily/weekly mode the primary control | The stated product goal is clear report granularity, so the UI should start from review cadence instead of raw period ranges. |
| Derive the KPI score client-side for this slice | Existing backend fields are sufficient for a transparent first model; backend persistence can wait until the score formula needs audit history, calibration, or permissions. |
| Use output, evidence, flow, and risk as the score dimensions | These dimensions map cleanly to the available data and avoid evaluating people from a single task-count metric. |
| Replace member ranking cards with a matrix table | Performance review needs scan-friendly comparison across multiple dimensions, not variable-height decorative cards. |
| Keep department mix separate from personal score | Department totals explain delivery distribution, but should not silently add or subtract from an individual's score. |
| Apply coreMember filtering inside KPI backend endpoints | Filtering only in the UI would leave summary, department, risk, and evidence data polluted by non-core assignees. |
| Disable KPI coreMember filtering when no member source is configured | Empty local/test deployments should preserve existing all-data behavior instead of returning an apparently broken empty dashboard. |
| Resolve core members by display name, username, email prefix, and first token | Jira and local profile identity fields vary, so strict string equality would incorrectly remove legitimate core members. |
| Reuse custom JQL assignees only as fallback | Explicit `jira.sync_users` is a clearer team boundary and matches existing frontend precedence. |
| Enforce coreMember visibility at API boundaries | Non-coreMember data should not be returned and then hidden in the UI; task, schedule, execution, and options APIs should apply the same configured member boundary before encoding. |
| Keep KPI as the only place that may use wider context internally | KPI can later use broader data for calculation context, but response evidence and visible user rows must remain constrained to included coreMember users. |
| Keep KPI mounted during post-load refresh | Background data refresh should not resize or remount the dashboard; inline sync feedback preserves spatial stability. |
| Replace click scale animation with color/border feedback | Internal dashboard controls should acknowledge clicks without changing element dimensions or causing perceived page jump. |
| Reserve fixed KPI refresh and matrix geometry | Daily/weekly switching changes data volume, so sync feedback, dynamic copy, report preview, and member matrix need stable slots or internal scrolling. |
| Add KPI depth and breadth as diagnostic panels | The next useful layer is explanatory structure around concentration, evidence, risk, review, quality, and distribution, not another decorative ranking card. |
| Let parent Jira Task facts drive bound execution rows | The execution tracking table is labeled as Jira Task result tracking, so the displayed owner/status should follow the bound Jira demand主线 rather than stale child execution fields. |
| Preserve child execution assignee separately | The old execution assignee may explain historical code activity, but it should not be mistaken for the current Jira owner. |
| Apply execution assignee/search filters after DTO construction | Effective owner/search fields include parent Jira facts, so pre-DT0 SQL filtering against child assignee can hide correctly reassigned rows. |
| Let Jira completion override local assignee protection | The sync worker's 24-hour manual override window can be useful during active triage, but it must stop once Jira returns a completed status. |
| Treat FZ-2220 as a completed-fact refresh gap, not a completion-flow gap | The local row is already `done`; the stale field is assignee, caused by completed issues exiting keep-alive while the main Jira JQL excludes `done`. |
| Redact secondary execution assignees that are non-core | A core-owned Jira row can still have an old child execution assignee; that helper field must not leak a non-core name in the tracking table. |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| Browser DOM snapshot check failed while verifying the local app | Switched to a lightweight page-text read; confirmed the Vite app is reachable but the in-app browser is currently on the login screen, so visual prototype verification requires logging in. |
| Browser verification of the componentized prototype was blocked by login | The app loaded at `http://127.0.0.1:5174/`, but the in-app browser stayed on the login screen and browser security policy rejected `javascript:` navigation for setting local preview auth. |
| Phase 38 `pnpm check` still fails outside the prototype files | Remaining errors are existing non-prototype issues in `TaskKanban.svelte` and `ProjectConfig.svelte`; production build and diff hygiene pass. |
| Phase 39 browser visual verification still needs a real login | The preview is behind the existing authenticated app shell; build-level validation passes, but visual review of `UI 原型` must happen after login. |
| Sandboxed Vite dev server could not bind `127.0.0.1:5173` | Re-ran the same dev server command with approved local bind permissions; Vite started on `http://127.0.0.1:5174/` because 5173 was already in use. |
| Initial discovery suggested missing backend server | Confirmed the files exist via `find`; proceed with direct reads. |
| Frontend build failed on `TaskKanban.svelte` const placement | Removed the duplicate const from inside the card markup. |
| Svelte build reports existing a11y warnings | Build succeeds; warnings remain outside this plan's blocking scope. |
| Earlier session lacked `taste-skill` availability | Wrote the required rule and recorded `frontend-design` as the fallback for future unavailable sessions; latest demand UI fixes used the requested `design-taste-frontend` skill. |
| `apply_patch` failed due formatted context drift | Re-read snippets, patched with smaller context, and logged the tooling lesson. |
| `svelte-check` still fails after this UI polish | Remaining type errors are pre-existing and outside the touched files: `TaskKanban` nullable values, `Button size` props in config/settings panels, `Switch helperText`, and `App.svelte` header indexing. Production build passes. |
| Full server test suite cannot run inside the current sandbox | Existing tests using `httptest.NewServer` fail with `listen tcp6 [::1]:0: bind: operation not permitted`; targeted non-listener permission catalog test passes. |
| `svelte-check` still fails after Settings/RBAC polish | Remaining errors are pre-existing in `TaskKanban.svelte` and `App.svelte`; touched Settings/AI/Button files no longer contribute errors, and production build passes. |
| Phase 22 full server package test cannot be re-run outside the sandbox in this session | The required escalation was rejected due account usage limit; targeted Phase 22 Go tests, frontend build, and `git diff --check` passed. |
| Phase 23 subagents did not return final reports before timeout | Closed all three agents, integrated the shared-worktree frontend partial, and completed backend/API/tests in the main thread. |
| Decision queue display meaning was unclear to the user | Added a current focus panel plus per-card source, decision type, no-action cost, and handling-entry labels. |
| The added decision queue meaning still did not satisfy the user's usefulness threshold | Removed the queue surface and its frontend polling/state/style code. |
| Worker-owned helper names conflicted during integration | Kept the richer shared strongest-brain helpers in `strongest_brain_handlers.go`, renamed delivery-only helpers, and removed the duplicate temporary helper file. |
| Full internal Go validation failed inside the sandbox because `httptest` could not bind loopback | Reran the same `go test ./internal/... -count=1` with approved non-sandbox execution and it passed. |
| `task_status.md` and `well-ambient.db` changed during validation/runtime side effects | Kept those generated/runtime changes out of the Phase 26 implementation commit instead of mixing operational data with source changes. |
| The delivery cockpit UI did not communicate its value even after adding purpose text and action strips | Removed the frontend component and shifted implementation effort to smaller phase APIs that can be evaluated independently. |
| Phase 32 `pnpm check` failed outside the KPI component | Treated the 3 existing type errors in `TaskKanban.svelte` and `ProjectConfig.svelte` as validation blockers outside this KPI refactor; production build and diff hygiene passed. |
| KPI report preview leaked non-core evidence when no user filter was selected | Moved coreMember filtering ahead of performance/report builders so evidence, risks, meeting focus, and overview all consume the same filtered task set. |
| KPI page jumped on report-mode clicks | Preserved the mounted dashboard after first load, added inline sync state, blocked duplicate refresh clicks, and removed active-state scale transforms. |
| KPI still jumped after the first fix | Kept the sync indicator mounted with hidden visibility, fixed dynamic text heights, gave report/member panels stable minimum heights, and capped the member matrix with internal scroll plus sticky header. |
| Jira execution tracking showed stale负责人/status | Added effective Jira assignee/status projection, secondary execution-assignee display, post-DTO filtering, and regression coverage for reassigned completed parent Jira Tasks. |
| Jira sync could preserve a stale owner after completion | Passed Jira mapped status into the local override guard and added a completed-status regression so `done` issues refresh from Jira immediately. |
| FZ-2220 stayed on an old assignee after completion | Changed keep-alive from active-only to active plus recently completed Jira keys, with tests for the 14-day completed correction window. |
| Non-coreMember rows could still be returned outside KPI | Added a shared visibility filter, removed external collaboration dropdown entries, and added tests proving task, schedule, and execution APIs suppress non-core data. |

## Resources
- Source plan: `/Users/eddie/.gemini/antigravity/brain/ffc6d8a3-38c0-4a9b-a458-9b51beeff27b/implementation_plan.md`

## 2026-07-11 Flow Board and AI Deconstruction Findings

- `.flow-dashboard .lane-cards` already declares `overflow-y: auto`, but the board and lanes override their shared height token to `auto`; without a bounded ancestor height, the lane never becomes a scroll container and is clipped by the shell.
- The gray strip between the filter card and lanes is the light page background showing through a transparent, gapped flow root. A single subtle white flow surface can connect the regions without introducing another heavy card.
- The demand-detail modal closes from an unconditional backdrop click handler. A target/currentTarget boundary is safer for nested async controls and prevents AI actions from dismissing detail state.
- `DemandDeliveryControl.svelte` uses invalid `font: ... inherit` shorthand for fields and buttons, so browsers discard the declaration and inherit inconsistent typography.
- `Deconstructor.svelte` is mounted by `App.svelte` only in the flow-board view. The product model should be one demand-intelligence workbench launched with seeded text from capture, schedule, or flow context.

## 2026-07-11 Phase 48 Correction Findings

- A zero-height flex child only grows reliably when the complete ancestor chain has a definite height. The authenticated flow shell did not satisfy that condition, so the safe contract is a viewport-clamped board height plus internal lane scrolling.
- “可关联需求” is not a passive metric: selecting one reuses that demand's existing task group, while the unbound option creates independent AI tasks. The selector therefore belongs before generation, not below the result fold.
- The full deconstructor includes file import, intent conversation, prompt disclosure, editable evidence table, and deep analysis. A contextual modal needs only association, source text, generation state, compact task suggestions, and the final synchronization checks.
- The settings project list is served by `/api/projects/config`; mixing `/api/demands/options`, telemetry scores, and historical task data created extra values. New-demand project choices now consume only the same filtered project-config response.
- A side-by-side AI companion is viable above 1500px; narrower viewports need an overlay fallback to preserve usable field widths.

## 2026-07-11 Phase 49 Browser Findings

- At 2048x925 the new-demand dialog measured `680x599.27` and the AI companion measured `620x597`; their vertical edges differ by about 2px and the horizontal gap is 16px.
- The combined dialog group center is exactly `1024px`, matching the viewport center, and neither the document nor the AI panel has horizontal overflow.
- Both surfaces resolve to the same light-console surface color `rgba(255,255,255,0.94)`, border `rgba(255,255,255,0.76)`, and primary text color `rgb(41,56,71)`.
- The compact AI panel visibly separates “需求标题” from “需求内容 / 规格说明”; the deconstruction request serializes them as `【需求标题】` and `【需求内容】` sections.
- At the default 1292x903 viewport the responsive fallback presents the AI panel as a centered overlay with no horizontal overflow, preserving readable field widths.

## 2026-07-11 Phase 51 Demand Association Search Findings

- The association selector already had a bounded option menu, so the lowest-risk interaction is a sticky search row inside that menu rather than a second modal or global search surface.
- Filtering now matches demand ID, title, owner, repository, and project name without mutating the source demand list.
- Authenticated browser validation filtered 101 demands to the single HR-4202 result, then selected it and confirmed the dropdown closed with the association value retained.
- The open-menu state had no horizontal overflow and the browser console reported no errors.
- Visual evidence is saved at `output/ui-validation-demand-association-search.png`.
# Health Diagnosis Intervention Modal Calibration

- `ProjectHealthTelemetry.svelte` owned both the modal structure and a large legacy high-contrast style block; a final `.health-diagnosis-modal` scope was required to avoid changing the surrounding project-health workbench.
- The visual mismatch came from a full pink intervention surface, saturated score/diagnostic text, four competing metric colors, lavender badges, and repeated inset-card borders.
- The revised contract uses a white workbench surface, restrained neutral sections, muted semantic rails/chips, one consistent type hierarchy, and a body-owned scrollbar.
- Browser harness verification at 1280×720 measured a 920×680 modal with 20px vertical safety space, no document horizontal overflow, two columns of approximately 455px/396px, and `overflow-y: auto` confined to the modal body (`610px` client / `668px` scroll height).
- The authenticated project-health route remains login-gated in preview. The current in-app browser facade also cannot resize the viewport, so the 860px/620px single-column rules were verified from compiled/source CSS rather than reported as live narrow-screen testing.
# Task Panels Height And Internal Scroll Alignment

- Both reported surfaces are in `web/src/components/TaskKanban.svelte`.
- The personnel grid explicitly uses `align-items: start`, so the table card and inspector size independently; neither has a shared height contract.
- The Jira Task status workbench also uses `align-items: start`. Its left section has a toolbar plus table shell, but the shell has no bounded height or overflow ownership, so row count expands the page while the inspector remains content-height/sticky.
- The correct desktop contract is equal-height grid tracks, column cards/panels as `min-height: 0` flex/grid containers, and overflow on the table shell only. At the existing 1180px collapse breakpoint, fixed heights should be released.
- The shared admin table already provides sticky headers and `overflow: auto`, so the fix only needs to bound the shell through its parent grid rather than duplicate table behavior.
- Browser verification at 1280×720 measured both status panels at 480px with a 0px height delta. The Jira Task shell measured 367px client height versus 2259px scroll height and retained `overflow-y: auto`.
- The personnel table and inspector also both measured 480px with a 0px height delta. Its table shell measured 401px client height versus 921px scroll height; no document-level horizontal overflow was introduced.
# Compact Task Rows, Inspector Pills, And Filter Layering

- The status table repeats project, parent Demand, repository, active-day, and evidence-progress metadata below the primary row content; these secondary elements are the source of the excessive row height.
- Task type is currently plain `Bug`/`Task` text in the table and `Bug 缺陷`/`Task 任务` in the generic inspector fact grid.
- The inspector facts are rendered as five independent two-line cards. They can be converted safely to a wrapping capsule list because the underlying `AdminInspectorRecord` contract does not need to change.
- The project/owner dropdowns live inside `.phase41-filter-surface`; its stacking context is not raised above the metric grid, while the absolute menu itself only has a local z-index.
- `.wa-admin-card` uses `isolation: isolate`, so raising only `.custom-select-options` cannot place it above later sibling cards; the filter surface itself must be positioned and assigned the higher z-index.
- Browser verification measured compact task rows at about 55px and confirmed there are no title, owner, or evidence secondary-line nodes in the status rows.
- All five inspector facts render in a wrapping flex capsule group; the sampled capsule height is 30px. The task-type icon is 24px, has no visible text, and retains `aria-label="缺陷"` or `aria-label="任务"`.
- The open project menu overlaps the following metric cards while remaining the top hit-tested element (`custom-option`), proving the filter surface stacking fix works. No document-level horizontal overflow was introduced.

# 2026-07-12 Phase 52 Screenshot Findings

- The schedule inspector repeats eight label/value facts in a two-column ruled grid, consuming most of the panel height before progress and acceptance criteria.
- Status, owner, dates, project, goal, stage, estimate, and task-group identifiers are secondary facts and can become wrapping capsules; title, progress, and acceptance criteria should remain the primary reading path.
- The shell global search is visually present but the user reports no effective search behavior, so the implementation must trace keyboard submission, result state, and route navigation rather than only restyle the input.
- The login email prefix and fixed domain are visually split by a hard inner boundary; they should read as one continuous field group with a shared focus ring and softer separator.
- `FunctionalAdminShell.svelte` owns the global search, but its input is currently unbound and has no submit handler, result state, API request, or navigation behavior.
- `App.svelte` owns the login prefix field and already wraps prefix and suffix together; the remaining issue is the suffix treatment and autofill/background seam, not form semantics.
- `DemandKanban.svelte` builds the schedule inspector as eight generic facts and renders them in a two-column ruled grid; the data contract can stay unchanged while the render becomes a wrapping capsule list.
- Authenticated search for `ZPU-2769` returned one demand result with title, owner, status, and route; selecting it opened the schedule workbench and produced no console errors.
- At `2133x902`, the schedule inspector renders all eight facts as wrapping 30px pills in a 67px block with no document horizontal overflow.
- At `1280x720`, the unauthenticated login preview renders the combined email control at 49px high with a transparent prefix input and a 32px rounded suffix capsule; the screenshot shows one continuous field surface without the former hard seam.
- A non-first-row regression with `HR-4202` confirmed exact focus handoff: the schedule filter, selected table row, and inspector all moved to `HR-4202`, with no browser console errors.
# 2026-07-12 - Demand detail AI workbench and executable spec draft

- `DemandDeliveryControl.svelte` can create/update/freeze a demand spec but the server exposes no draft-withdrawal route; a real undo requires a backend transaction, not UI-only state clearing.
- Draft creation also creates a `ReviewContract`, so withdrawal must delete the draft contract and spec together and reject non-draft specs.
- `DemandKanban.svelte` opens the new-demand AI panel with `presentation='companion'`, but demand-detail and schedule call the same function with its default standalone modal presentation.
- Host close handlers currently clear only their own state, leaving `showDeconstructorModal` untouched; this is the orphaned right-panel bug.
- The current companion geometry is hard-coded for the 680px create-demand host. Schedule is 720px and detail is 760px, so a shared host-width variable is needed for a centered pair.
- `DemandSpecVersion` already persists business rules, main/exception flows, permission rules, data/API/UI impact, dependencies, risks, acceptance criteria, test plan, mapped repositories, and tasks.
- `DeconstructAnalysis` currently generates only completeness, estimates, missing info, risks, dependencies, acceptance criteria, schedule notes, and meeting questions. The reserved implementation fields are never populated, which weakens downstream code generation.
- The safe execution boundary is already appropriate: a frozen spec feeds code generation, updates are restricted to supplied source snapshots, delete actions are rejected, and CI plus human review remain required.
- Authenticated browser validation exposed an adjacent detail-to-schedule bug: the click handler called `closeDemandDetails()` before reading `detailDemand`, so it always cleared the reference and failed to open schedule. Capture the demand first, then switch panels.

# 2026-07-12 - Streamed AI specification draft and Markdown preview

- The reported flow waits synchronously for a slow LLM result, so an intermediary can time out even when the provider eventually succeeds. The replacement must emit bytes early and periodically, then send a distinct persisted final event.
- The supplied screenshot shows one long modal composed of many uniform textarea fields. This flattens headings, lists, code, emphasis, and tables into body-text boxes, creates multiple nested scrollbars, and weakens the reading path.
- The result surface should have one document reading column with Markdown typography, a compact status/header rail, and explicit generating/complete/error states. Editing can remain a separate intentional mode rather than making every field look permanently editable.
- Visual baseline: calm white workbench, teal accent used sparingly, neutral dividers, restrained status chips, one body-owned scrollbar, and responsive single-column behavior.
- The existing development-auth path required both an explicit environment flag and a loopback request, but still contacted WellOS first. Short-circuiting only under those two guards prevents external authentication traffic during local browser validation without changing production behavior.
- Authenticated DG-319 validation at 1280×720 rendered 14 semantic Markdown headings and 6 lists with zero result textareas, no document horizontal overflow, and one modal-owned vertical scrollbar (`3210px` scroll height / `670px` client height).
- The explicit Preview/Edit switch preserves the readable Markdown result as the default while keeping all governed fields available for manual correction and save.
- At 760×900 the detail modal measured 732px wide with 14px side safety space; the Markdown article measured 611px and its scroll/client widths matched, while the document remained exactly 760px wide.
- Browser screenshot review confirmed the calm white/cyan workbench palette, clear heading rhythm, restrained status chips, responsive stacked toolbar, and no competing nested result scrollbars. Browser console errors: 0.
- Validation screenshots: `output/ui-validation-ai-spec-markdown-desktop.png`, `output/ui-validation-ai-spec-markdown-body.png`, `output/ui-validation-ai-spec-edit-mode.png`, and `output/ui-validation-ai-spec-markdown-760.png`.

# 2026-07-12 - Impeccable review gate and shared Markdown workbench

- The remaining card nesting was structural, not a Markdown renderer problem: `delivery-control > delivery-stage > markdown-preview/spec-editor` retained three rounded, bordered surfaces after the previous content refactor.
- The project already has `web/src/components/shared`, so the Markdown editor belongs there rather than in a page-local design-system fork.
- Impeccable, taste-skill, and finesse-ui agreed on one legitimate tool boundary, a proven editor engine, flat stage hierarchy, progressive disclosure for governed fields, and authenticated responsive verification.
- `MarkdownWorkbench.svelte` now owns CodeMirror editing, GFM rendering, DOMPurify sanitization, edit/split/preview modes, read-only behavior, line/character status, save shortcut, and streaming presentation.
- The stream preview and persisted specification use the same shared component; the legacy hand-written `web/src/lib/markdown.ts` renderer is deleted.
- Authenticated DG-319 validation measured a 460px desktop editor with 83 lines / 1136 characters, equal 351px split panes, zero nested workbenches, and transparent 0px-radius delivery stages separated only by 1px hairlines.
- At 760x900, split mode expands to 700px with two 282px panes and no clipping; preview mode remains 460px and the workbench, toolbar, and modal all have matching scroll/client widths.
- The Impeccable detector returned `[]`, browser console errors were zero, and no business data was saved or mutated during interaction testing.

# 2026-07-12 - Inline Markdown editing and review contract layout

- Persistent edit/split/preview controls competed with the document even though preview is the dominant state; the demand-spec surface now renders only the Markdown document until direct interaction requests editing.
- `MarkdownWorkbench.svelte` now supports an inline-edit contract: double-click enters CodeMirror, an outside pointer commits changed content, unchanged content exits locally, save failure remains in edit mode, and a 1px keyboard entry becomes visible only on focus.
- Authenticated DG-319 validation changed the source from 1136 to 1137 characters with a trailing space, clicked outside, received `Markdown 草案已临时保存。`, and returned to preview; payload trimming preserved the actual Markdown body.
- The old review contract used three equal columns even though the third column contained three stacked policy fields. The replacement uses reviewer and policy fieldsets in a 1.08/.92 grid, a 24px divider rhythm, compact status/SLA metadata, and right-aligned approval actions.
- Desktop detail width is now one coherent 960px contract instead of competing 640px and 760px declarations; the AI companion host variable uses the same 960px geometry.
- At 1280x720, the detail modal measured 960px, the Markdown workbench 903px, and review columns about 475/404px. At 760x900, the modal remained 732px, the review layout collapsed to one 675px column, and modal/workbench scroll widths matched client widths.
- Targeted Impeccable and layout scans returned `[]`; the broad `DemandKanban.svelte` warnings belong to unrelated legacy regions and were recorded as explicit existing debt.

# 2026-07-12 - Flat review contract and directory-backed reviewers

- The incorrect `Eddie Antigravity` reviewer came from `Jira.SyncUsers` being copied wholesale into every new review contract; that configuration is a sync scope, not an approval decision.
- Reviewer candidates should represent an explicit contract selection. The only defensible automatic candidates are actual implementation-task assignees, with the specification author removed by default.
- Responsible-person controls require a contextual directory even when the reviewer manager lacks global `users:read`; returning the minimal participant projection with the readable review contract keeps the permission boundary aligned with the workflow.
- The page already loads the user directory for common admin roles. Merging it with the contract-scoped directory makes HMR/old-backend development sessions resilient while the shared selector still consumes one de-duplicated adapter shape.
- Authenticated DG-319 review showed 20+ searchable real users with name, department, and email context. Department filtering reduced the list to exact matches, and two candidates remained selected simultaneously without saving.
- The final review surface has one form hierarchy and no nested fieldsets/cards. The Markdown document flows directly into the review section, and the removed raw structured matrix no longer appears in the DOM.
## Responses-first LLM Transport And Browser Streaming

- The pre-existing demand-spec editor already streamed provider SSE into browser NDJSON, but `/api/deconstruct` still buffered a full provider response and both deconstruction consumers used `res.json()`. Streaming was therefore partial, not system-wide.
- Active Chat Completions construction was duplicated across the server client, demand deconstruction, telemetry review, and connection test. One `internal/llm` client now owns protocol normalization, payloads, authentication, synchronous parsing, and SSE parsing.
- OpenAI, Sub2API, and compatible gateways use Responses by default. The persisted `completions` value is a migration input only; explicit native Anthropic uses Claude Messages. Attachment file IDs remain a Responses-only feature and fail explicitly for native Messages.
- The browser protocol remains provider-neutral NDJSON. Demand deconstruction now emits status, provider delta, parse, complete, and error events; UI consumers display progress without rendering incomplete JSON.
- `撤销草案` now shares one right-aligned progression group with `保存契约` and `批准审核契约`, preserving semantic danger styling and confirmation while removing the isolated far-left action zone.
- Authenticated UI validation proved the protocol-choice states. A copied historical database was not safe for repeated full-app validation because background delay-alert processing activated even with external integrations disabled; fixture-only databases are required for future isolated browser runs.

# 2026-07-13 - Configuration center full visual refactor audit

- The requested change is a product-register redesign of the authenticated configuration center, not a new marketing surface. Preserve the Phase 41 light console, real API behavior, permissions, route labels outside the removed Settings KPI entry, and the existing teal accent.
- The current Settings family has one shared `scw-*` workbench for Feishu, Jira, Project, and AI, but GitLab still owns a separate legacy vocabulary and `SettingsPanel.svelte` carries roughly 4,900 lines of accumulated Phase 41/46/49 style overrides. That layered ownership is the primary source of cross-page drift.
- Authenticated 2133x902 inspection covered Feishu, GitLab, and Members. Feishu and GitLab use the same outer header and audit-column concept, but internal headings, status strips, tables, and action alignment differ; Members drops the audit column and exposes the older RBAC table styling.
- The outer page currently stacks breadcrumb, a second title/facts card, then the workbench. This spends vertical space on duplicate identity and makes the configuration content begin too low. The redesign should merge page identity, status, and facts into one compact translucent command header.
- Glass should communicate page chrome and persistent workbench boundaries only. Field rows, tables, and permission branches should remain flat or use hairlines so the interface does not become nested glass cards.
- KPI remains a valid top-level `度量洞察` route, but the `KPI 绩效大盘` Settings child is redundant. Remove it from Settings types, permissions/fallback routing, shell subnavigation, Settings metadata, and Settings rendering without deleting KPI permissions or the top-level KPI page.
- Browser inspection was read-only: no configuration save, health check, rollback, membership mutation, or authorization action was triggered.
- The implemented route registry has 10 Settings sections and no KPI section; App fallback, shell submenu, page metadata, access checks, and Settings rendering now consume that registry instead of maintaining divergent local arrays.
- The unified visual contract is a translucent context header plus one primary workbench and optional audit rail. Inner configuration facts, forms, security branches, and tables use transparent surfaces or hairline boundaries so the explicit glass request does not reintroduce nested-card depth.
- Authenticated validation opened GitLab, Feishu, Jira, Project priority, AI engine, system corpus, members, permission matrix, policy authorization, and audit log. All exposed the expected page region/heading, none exposed a KPI Settings item, and none showed a load failure.
- Screenshot review at 2133x902, 1024x900, 760x900, and 480x900 confirmed aligned outer panels, responsive inspector flow, readable tables, consistent teal status color, and no visible horizontal overflow. The first 480px pass exposed a generic 44px touch rule inflating switches; excluding `role=switch` restored the intended 44x24 control.
- The GitLab edit wizard was entered without modifying fields and reviewed at 760x900; navigating away restored the read-only overview. Browser console errors remained zero and no business mutation was issued.

# 2026-07-13 - Configuration panel alignment follow-up audit

- The authenticated policy page reproduces the user's complaint at 2133x902: template cards sit above a dense two-column editor/explainer, both columns expand full permission-chip inventories, and each column introduces its own inner scrollbar. Density comes from simultaneous exposure rather than task-focused grouping.
- `SettingsPanel.svelte` still mounts `phase41-settings phase46-settings phase49-settings settings-unified`. The legacy selectors carry higher specificity and `!important`, so earlier `stretch`, absolute-position, fixed-height, and overflow rules can beat the new natural-height/sticky contract.
- The three-way review agrees that Settings layout ownership must converge on one final contract: shared grid controls only columns and top baseline; primary content uses natural page height; the version inspector is the only sticky side rail; only version list, diff/code, and explicit wide-table containers own local scrolling.
- System corpus drift is page-internal. `AIConfig.svelte` renders the source library/editor, candidate review, and context preview in one continuous page. The source list has a bounded height while the editor grows naturally, and the unbounded candidate article list can extend the page indefinitely.
- Policy drift is page-internal. Template choices, policy editor, authorization explainer, policy table, and authorization audit are all visible at once. The complete permission vocabulary is expanded twice, and the lower 7-column table is constrained to half width beside another internally scrolling region.
- Impeccable and finesse prefer explicit tabs/progressive disclosure for the corpus and policy task families; taste recommends the lower-risk first slice: fix corpus list/editor stretch, top-align policy editor/explainer, make policy table plus audit full-width stacked, restore semantic allow/deny colors, and remove duplicate inner page identities.
- Shared direction for this follow-up: stop legacy root phase classes from owning geometry; use natural-height outer layout; normalize the two named pages into clear task groups; preserve APIs, permissions, field values/order, mutations, and route labels. The first implementation removed the classes outright, but that exposed hundreds of dormant CSS warnings, so the stable solution keeps compatibility class names while a single `#settings-unified-root` owns live geometry above them.
- The implemented policy surface now defaults to one compact strategy-list task with `策略列表 / 新建策略 / 授权解释 / 授权审计` tabs. At 2133x902 the primary workbench is 337px high, the page needs no vertical scroll, and document horizontal overflow is exactly zero; the previous editor, explainer, table, and 40 audit rows no longer expand simultaneously.
- Policy table and audit history are explicit named scroll regions, action vocabularies are bounded, semantic allow/deny colors are restored, and all choice/switch states expose pressed or checked semantics without changing their mutations.
- Authenticated policy state sweep confirms exactly one `tabpanel` for each of the four tabs. The create form exposes 27 existing actions inside a bounded chooser and 6 pressed-state controls; the explain form exposes 2 pressed states; the 40-row audit is capped at 523px with local scrolling; all four states keep document horizontal overflow at zero.
- Authenticated corpus library validation shows one active task panel and a deliberate 630px / 1029px source-editor split. Both columns start at the same 399px baseline and resolve to the same 743px grid track; the page has zero horizontal overflow and no duplicate internal page title.
- The corpus candidate tab resolves to a compact 171px empty state when no work is pending; populated queues are capped at ten articles per page by component-owned pagination. The preview tab resolves to one 293px task panel with a single textarea, one preview action, and one calm waiting state; neither task adds horizontal overflow or performs a request during validation.
- GitLab desktop validation proves the repaired two-panel contract: both panes start at 260px with a 0px top delta; the 1201px primary pane stays natural height while the sticky audit pane is independently capped at 790px. The first medium-width pass exposed a remaining 1978px audit expansion caused by an unconstrained diff row, so the responsive audit owner is now capped to roughly one viewport and its version/diff regions retain local scrolling.
- Responsive GitLab checks requested 1024, 760, and 480 widths; Chrome reported actual content viewports of 1138, 844, and 533px. At 844px the formerly 1978px audit panel is now exactly 700px, with its 1559px diff content scrolling inside a 281px region. At 533px the primary and audit panels are one column, the audit stays 700px, version rows are bounded, and document horizontal overflow remains zero.
- At the 533px policy pass, every task remains the only visible panel: the action vocabularies scroll inside 219px/179px regions, the 40-row audit scrolls inside 560px, and priority control height is 44px. The pass exposed 34px policy tabs from a legacy important rule; the unified mobile selector now restores all four tabs to 44px.
- The full desktop route sweep found no load failures, no horizontal overflow, and a 0px top delta on GitLab, Feishu, Jira, Project, and AI engine audit rails. It also exposed content-driven 4536px member, 4532px permission-tree, and 5982px security-audit surfaces; these directories are now bounded to a 720px data workbench with sticky table headers and named scrolling regions instead of stretching the entire outer glass panel.
- Post-fix desktop measurements reduce member and audit primary panels to 739px and permission matrix to 934px; their data regions are 614px high and retain the full 4410/4339/5856px datasets via local scrolling. At exact 760 and 480 viewports, all three pages keep zero document overflow; the 480 member/audit tables own their necessary horizontal scroll while the permission tree stays within its 405px region.
- Exact responsive validation covered corpus task panels at 1024/760 and policy task panels at 1024/760/480. Every state mounts exactly one `tabpanel`, mobile controls are 44px, tab strips do not overflow, and every page keeps document horizontal overflow at zero. A fresh authenticated tab reports zero console errors after the null-memberships normalization.

# 2026-07-14 - Governed corpus ingestion and unified review center

- Raw source documents need their own lifecycle, provenance, version, hash, uploader, and ingestion state; a `ContextFact` alone cannot preserve immutable evidence or represent failed extraction.
- LLM extraction is safest when it returns bounded atomic candidates with explicit `source_fact`, `source_rewrite`, or `ai_suggestion` evidence kinds. AI-only supplements and high/global architecture rules must remain outside production context until a separate impact gate completes.
- The final publish endpoint must use the exact persisted candidate that was previewed. Candidate metadata/content is frozen in the impact stage, re-review invalidates prior preview metadata, and the backend rejects publish requests without a recorded impact preview.
- Archiving a raw source must also withdraw its unpublished candidates atomically; otherwise an invalidated source could still publish later. Accepted candidates and their active facts remain untouched.
- The source-first layout resolves the previous mixed corpus page: `资料库` owns raw-document import/version viewing, `统一审核` owns a queue plus one review workbench, manual `ContextFact` entry is an advanced disclosure, and `上下文预览` remains the production verification task.
- Authenticated isolated-browser validation exercised real Markdown import through a mock Responses provider, ordinary acceptance, sensitive/global first approval, impact comparison, final publication, and source publication counts without touching the workspace database.
- Exact 1440, 1024, 760, and 480px checks found zero document or component horizontal overflow. Desktop Markdown comparisons had 0-2px top/height deltas, narrow comparisons stacked in source/current-first order, browser console errors were zero, and both published candidates retained their source-document link.

# 2026-07-14 - Corpus decision feedback correction

- `CorpusCandidateReview.svelte` treated four completed decisions as persistent panel content through one local `success` state. Those messages shifted the review layout even though they required no follow-up action.
- The existing shared `Alert.svelte` already owns the project's success presentation, but the repository had no reusable application-level toast queue or overlay host. `Deconstructor.svelte` contains a page-local bottom-right toast that is not an appropriate shared owner for Settings feedback.
- The correct feedback taxonomy is recovery-based: validation and request failures remain in the review container with `role="alert"`; reject, ordinary publish, impact-stage approval, and final impact publication use a non-blocking `aria-live="polite"` upper-right toast.
- Production and the isolated settings preview require separate host mounts because they are different Svelte entry trees. One shared store keeps the queue and duration contract identical without coupling the candidate component to either shell.
- Browser validation confirmed the exact final message `影响审核已确认，候选已正式发布。` in the upper-right overlay at 16px top with a bounded 420px desktop width, while `.review-message.success` remained absent.
- The 4.5-second timer removed the toast after the bounded 4.8-second check, manual close removed it immediately, and a fixture 503 remained inside the review container as `role="alert"` with no toast and no false empty state.
- At 1440/1024/760/480 widths the notification remained fixed, within the viewport, and caused no document horizontal overflow. The 760/480 layouts use safe side insets and a 44x44 close target; browser console errors and warnings were zero.

# 2026-07-15 - Impact review flow and panel alignment correction

- The apparent “still two-column” problem is nested: the outer queue/detail grid is required for navigation and is explicitly referenced by the height complaint, while the inner `comparison-grid` is the redundant second two-column system. The impact comparison should therefore become a vertical current-context-first sequence without removing the outer queue.
- The height mismatch comes from unrelated sizing rules: only `.candidate-queue-list` had `max-height: min(70vh, 680px)`, while `.review-workbench` remained content-sized. `align-items` cannot align their bottoms when the right side contains two fixed-height Markdown workbenches plus metadata and actions.
- The stable desktop contract is one shared viewport-relative panel height on both outer columns. Queue header and workbench header/action row remain stable; the queue list and workbench body are the named scroll owners. Below 960px the fixed height is removed and the existing stacked flow returns.
- `MarkdownWorkbench` uses the `minHeight` prop as an actual fixed component height and already owns document scrolling. The vertical impact panes are reduced from 420px to 360px to keep each comparison leg legible without recreating an abnormally tall review body.
- The visual change stays inside `CorpusCandidateReview.svelte`; no lifecycle, API, permission, grouping, Markdown value, or toast behavior changes.

# 2026-07-15 - Workspace-bounded toast positioning correction

- The toast overlap is structural: `App.svelte` mounts `ToastHost` above every authentication/shell branch and `ToastHost.svelte` fixes it to viewport coordinates. Changing `top` would only encode one Header height and fail when the maintenance banner or responsive Header changes.
- The Shell already has the correct ownership boundary. `.console-main` orders Header, optional maintenance banner, and `.workspace-frame`; introducing one relative `workspace-stage` after the first two lets the toast inherit the actual remaining content rectangle without measuring Header pixels.
- `.workspace-frame` must remain the sole page scroll owner. The new stage wraps it, establishes `position: relative`, `isolation: isolate`, and `overflow: hidden`; the toast is absolute inside the stage, while the workspace continues to scroll independently.
- Toast pixels need both a queue bound and a geometric bound. The existing maximum of three notices remains, while `max-block-size: calc(100% - 2 * gutter)` prevents long messages from extending below the workspace.
- The global `--wa-layer-toast` token is not defined, so the current fallback 1400 exceeds Header and popover layers. Stage-local containment allows a low local layer and makes escape into Header impossible even if notice content grows.
- The isolated settings preview has a separate Svelte tree and needs the same ownership pattern around `preview-settings-host`; `settings-preview-entry.ts` remains a bootstrap-only file.
- The two isolated Impeccable assessments agreed on component ownership and containment. The subjective audit caught the Header/workspace hierarchy and long-message height risk; the detector returned clean `[]` and separately confirmed that this bug is outside its mechanical rule coverage.
- Browser interaction exposed a preview-only follow-up: placing the host in a natural-height stage bounds it geometrically but lets the whole stage scroll offscreen. The production Shell does not have this problem because its workspace stage occupies the remaining viewport. The preview now mirrors that contract with a fixed-height page, fixed topbar/tabs, and one internally scrolling settings host.
- Final correction rule: feedback scope and visual containment are separate decisions. A store may be application-global while its overlay is workspace-local; mounting both globally is not required and can violate shell hierarchy.

# 2026-07-15 - Phase 72 initial visual and design-system audit

- The attached flow-board detail has three competing vertical owners: the centered overlay document, the modal-level rail, and a second Markdown rail. The primary defect is information architecture and scroll ownership, not merely colors or padding.
- The visible metadata is simple fact content but is presented as six equal nested cards, which flattens hierarchy. A compact definition grid or summary strip can preserve every value with less visual noise.
- The empty shadow-task section takes the same width and visual weight as populated content. It should become a compact status row or a collapsed secondary section until it contains actionable records.
- The Markdown specification currently behaves like a second page embedded in the modal. The detail workbench should own the only vertical scroll, while the Markdown region expands naturally inside it and defaults to the single `即时排版` presentation.
- Product-register color must remain restrained. The supplied ten-color library is a source palette, not a requirement to show ten accents. A stable semantic mapping with teal primary, green success, orange warning, red danger, and navy information protects scanability and state meaning.
- The strongest-brain decision timeline should be treated as a chronological evidence ledger: decision/outcome first, actor and time aligned consistently, supporting evidence available progressively, latest meaningful event visually current, and no decorative zigzag timeline.
- Live ownership is now located: `MarkdownWorkbench.svelte` renders the four mode controls; `Select.svelte` and `MultiSelect.svelte` own scrollable option lists; `DecisionDashboard.svelte` owns the named decision-event timeline; `DemandKanban.svelte` owns the click-through detail modal and composes `DemandDeliveryControl.svelte` for the AI specification.
- The flow-board detail is not a duplicate page: `DemandKanban.svelte` owns task context and actions, while `DemandDeliveryControl.svelte` owns the AI-spec lifecycle. The redesign must preserve that ownership and remove nested scroll through composition rather than copying spec markup into the modal.
- `MarkdownWorkbench.svelte` currently defaults to `mode='preview'` and always renders four mode buttons when its toolbar is enabled. Live callers already use `live`, while readonly evidence explicitly uses `preview`; the safe component contract is a default single live mode plus an opt-in list of additional modes, with an explicit current mode retained for readonly consumers.
- Both shared dropdowns already have the correct scroll behavior. `Select.svelte .select-options` and `MultiSelect.svelte .multi-select-options` use bounded `overflow:auto`; only the visible rails need hiding with cross-browser scrollbar rules. Scrolling, keyboard navigation, overscroll containment, and max-height must stay intact.
- `modern-admin-tokens.css` is the semantic source of truth. The global token values currently use teal `#008f96`, green `#04966f`, amber `#d88700`, red `#dd4b3e`, and blue `#256bd8`. Updating those semantic primitives is safer and more consistent than scattering the supplied hex values through individual components.
- Many component fallbacks still embed the old values. The token update will govern normal application rendering; only touched shared and redesigned components should receive updated fallbacks to avoid expanding this phase into an unrelated whole-repo mechanical rewrite.
- `DecisionDashboard.svelte` places the global decision timeline after the entire dynamically-height-matched table/inspector workbench, so it is visually buried and detached from the selected decision context. Its current nested date column, connector line, marker, time column, kind pill, title, task pill, commit pill, and body create too many equal-weight signals.
- The main decision page already has the correct primary order: metrics, state filters, selection table, selected-item inspector. The timeline should become an evidence ledger associated with the selected record, while an unselected state can show the latest filtered events. This preserves global discoverability without keeping a second full-width dashboard below the main task.
- The demand detail composes the right owners but exposes them in one long flex column. `DemandKanban.svelte` provides header, title, six facts, actions, shadow tasks, then a complete `DemandDeliveryControl.svelte`; the modal and Markdown workbench each scroll. The new structure should keep a sticky summary/action header and one detail-body scroll owner, with the delivery component using an embedded/natural-height presentation.
- `DemandDeliveryControl.svelte` initializes specification Markdown as `preview`, hides its toolbar, and enters edit by double-click. It should keep that lifecycle. The shared Markdown default change must not force this explicit inline-edit consumer into live mode; instead the component should allow the parent’s explicit mode while the toolbar's default visible controls remain restrained.
- The details companion contract is width-coupled through `--ai-host-width` and `companionHostHeight`. Any wider detail workbench must update both host declarations and preserve the existing 16px host/companion gap; below 1500px the companion already becomes a centered overlay, so the base detail width must still fit independently.
- Contrast measurements constrain the palette mapping: `#6ecc54` and `#71e2d1` require dark ink; `#eb5c20` also requires dark ink; `#002fa7`, `#470125`, `#492d22`, and `#0d3a69` support white; `#c8161d` supports white; `#d34947` and `#018b8d` narrowly fail 4.5:1 with white at normal text size. Therefore teal/red library colors should serve accents, borders, icons, or soft surfaces unless paired with a separately verified dark ink or darker semantic strong tone.
- The isolated Impeccable mechanical layout detector returned `[]`, and arbitrary Tailwind spacing/z-index patterns were absent. Its strict CSS number pass found 324 legacy non-scale spacing declarations and 36 explicit z-index values across the seven large target files. This phase will not mechanically normalize entire legacy files; new and directly changed layout values must use existing tokens, and untouched declarations are a deliberate preservation exception.
- Every current `MarkdownWorkbench` consumer already supplies a mode explicitly. Editable corpus surfaces use `live`, manual advanced entry uses `edit`, and evidence/read-only surfaces use `preview`; the delivery spec binds an explicit preview/edit lifecycle. Changing the component default to `live` is forward-safe, while a default single-mode toolbar will render the consumer's explicit current mode when it is not live.
- The working tree is heavily modified and the shared Markdown/MultiSelect files are currently untracked from the prior corpus implementation. Phase 72 must patch only the named surface contracts and never normalize or restore unrelated user work.
- Existing browser captures confirm the detail defect is not caused by the supplied screenshot scaling: at the prior 1280-wide validation the modal header and summary are readable, but the six repeated fact cards, full empty shadow-task band, delivery header, and a second bordered Markdown viewport still consume nearly the entire visible height before the document body is reached.
- The previous companion screenshot also confirms the project already uses a centered host/companion ownership pattern. The detail redesign should preserve that interaction and improve the host surface itself rather than replacing the companion with a nested tab or a second modal.
- The isolated subjective assessment agreed on single detail-body scrolling, flat fact presentation, one default Markdown mode, restrained palette roles, and selection-first responsive causality. It argued for retaining the global timeline below the main workbench; the final review instead makes the ledger contextual and adds an explicit all-records scope, satisfying the user's location correction without deleting audit coverage.

# 2026-07-20 - Jira version sources and decision bottom substrate

- Jira release-page URLs are stable query references rather than issue URLs. The safe application boundary is same-origin parsing of `/projects/{projectKey}/versions/{numericVersionID}` followed by Jira search JQL `(project = "KEY" AND fixVersion = ID)`; page HTML scraping is unnecessary.
- Version-source JQL must remain additive to the ordinary or custom branch. Otherwise a release outside the ordinary project list—or a custom JQL result set—would silently disappear despite being explicitly configured.
- Version projects are part of the effective project catalog, not just the Jira worker. Their configured number/name now feeds demand options, personal project preferences, project configuration/scores, and telemetry score scope.
- The reported decision-page strip was shell-owned: `.workspace-frame` painted the gray page substrate and applied four-sided padding while the decision roots remained transparent. Setting only the decision route's bottom inset to zero lets the page/card root reach the viewport without changing card backgrounds or unrelated routes.
- Authenticated fixture geometry proved the ownership change: at 2048x924 the decision frame and both Decision Agenda/Daily Jira roots end at y=924 with `padding-bottom: 0`; at 760x900 they end at y=900 with no horizontal document overflow.
- Browser configuration validation recognized the supplied link as `PRJ25024 / 13622`, offered the parsed project key before the project name was entered, and displayed the completed valid state without saving or connecting to Jira.

# 2026-07-15 - Phase 72 implementation findings

- The strongest-brain timeline works better as evidence attached to the selected decision than as a second dashboard below the selection/detail workbench. Keeping `当前 / 全部` scope preserves audit completeness without weakening causality.
- Hiding a dropdown rail is safe only when the list remains the same scroll owner. The final shared rule changes scrollbar presentation only; measured overflow, option count, focus ownership, and list height remain intact.
- `#018b8d`, `#6ecc54`, `#eb5c20`, and `#d34947` are not safe default white-text fills for normal text. The semantic token layer therefore separates source color from readable foreground instead of applying one universal white label treatment.
- The detail modal's excess height was caused by multiple scroll owners and repeated card chrome, not by one incorrect `max-height`. Bounded modal geometry plus a single scrolling body and natural-height specification preview resolves both the visual and interaction defect.
- Full-file aesthetic detectors remain noisy on the heavily evolved dashboard sources. The relevant Phase 72 selectors are clean; remaining findings point to older risk-calendar, board, and delivery-policy styles and should be handled as a separate legacy cleanup rather than folded into this scoped refactor.

# 2026-07-18 - Daily Jira timing badge clarification

- Creation age and deadline health are separate facts. The audit buckets and `age_days` remain unchanged, while the first-column status is derived from `due_date` plus the backend `generated_at` audit timestamp.
- The compact contract is `圆点 + 逾期/临期/健康 + · + 今天/N天`. Red means already overdue, amber means due today through three natural days, and green means later or without a due date; every color also has visible text and a full accessible description.
- Date-only Jira deadlines are normalized as calendar dates before comparison so `YYYY-MM-DD` values do not shift across browser time zones.
- The first column is 126px on wide screens, 112px at 760px, and 104px at 520px. Browser measurement at 1280/760/480 found 24px-high badges, `white-space: nowrap`, equal client/scroll widths, zero document overflow, and the existing 16px bottom gap.
- Browser checks also confirmed `健康 · 今天`, exact three-day `临期`, red overdue, green no-deadline, whole-row Enter selection, read-only safety with no POST, and an empty console error list.
# 2026-07-18 - Backend LLM deconstruction availability

- Latest database configuration enables AI, has a non-empty credential, targets `https://sub2.congmingai.com`, and selects `gpt-5.5`.
- Persisted `endpoint_type` is `completions`; `AIConfig.Protocol()` deliberately normalizes every non-Messages value to `responses`, so the real endpoint is `/v1/responses`.
- The authenticated health check passed with the message that `gpt-5.5` is reachable through Responses.
- A harmless synthetic request exercised `/api/deconstruct`, provider streaming, NDJSON browser transport, structured JSON parsing, and UI normalization; it produced three task suggestions with 76% completeness.
- No demand, shadow task, or deconstruct archive was created because neither task synchronization nor demand assignment was executed. The run persisted context pack `24` (329 tokens, two items) as a diagnostic context artifact.
- `go test ./internal/config ./internal/llm` and the focused `internal/server` deconstruction/streaming tests passed.

# 2026-07-20 - Daily Jira bottom ownership and Jira version-link audit

- The bottom mismatch was a duplicated ownership defect. `FunctionalAdminShell` already fixes the viewport and owns the responsive gutter, while `DailyJiraAudit` independently subtracted its DOM top and a fixed 16px from `window.innerHeight`.
- `FunctionalWorkspace` explicitly connected only `.decision-admin` to the viewport-fit content track. Adding `.daily-jira` to the same direct-child contract and using `height: 100%` removes the second calculation without changing panel styling, row density, or responsive information order.
- At 2048x925, the frame ends at 925, its bottom padding is 22px, and both Daily Jira panels end at 903. Decision Agenda produces the same 903px root bottom. The exact equality demonstrates component ownership rather than a viewport-specific visual compensation.
- The authenticated Jira release page is not an issue URL; it is a release/version view. Its authoritative “在问题导航器中查看” link uses `project = 11900 AND fixVersion = 13622`, proving the version's issue set is queryable through Jira search.
- The current client exposes paginated `/rest/api/2/search`, and the worker gives `custom_jql` precedence over project/user/status-generated JQL. Therefore the 21 matching issues are ingestible today by configuring that JQL, but pasting `/projects/PRJ25024/versions/13622` itself is not understood because no release-URL parser or version resolver exists.
# 2026-07-31 - Local release to Jira work-item association

- “全部项目”后的叉号来自共享 Select 把空字符串选项也当成可清除选择；清除动作现在只在真实非空值存在时渲染。
- 创建弹窗和检查器下拉显示不全的根因是绝对定位菜单仍受滚动容器裁切；菜单现已 portal 到 body，并按触发器和视口计算上下方向、宽度与边界。
- 本地版本的项目已经唯一决定 Jira 候选范围，因此 Jira 项目下拉是重复且可能产生冲突的输入，已从右侧检查器移除。
- 版本与 Jira 事项继续以 `work_item_release_links` 表达 N:M 关系；本地关联不会创建 Jira 版本，也不会写回或清空 Jira `fixVersion`。
- 批量关联在一个事务内校验 Jira 来源、同项目和主目标版本冲突，并通过 CAS 更新关系；任何一条冲突都会整体回滚。
- 版本列表的 Jira 数量、候选检索和项目看板版本标识均使用批量/聚合查询，避免逐行 N+1。
- 旧 `release_jira_links` 表和兼容接口暂不删除；新 UI 不再把它当作事实来源，后续删除仍需经过兼容观测门禁。
- 隔离认证浏览器验证了创建成功/失败 Toast、完整长项目名、批量关联两条 Jira、关联列表、项目看板版本标识和 744px 无横向溢出；未触碰生产数据或真实 Jira。

# 2026-07-31 - Task Table Work Item convergence

- 全局搜索原先只更新结果列表，Enter 没有稳定执行“选择 + 导航 + 定位”；搜索入口现以 Work Item 为事实源，优先编号 / 标题精确匹配，否则选择排序首项。
- 任务表错误显示 Git 提交的根因是状态、执行、人员三个视图共用了 `/api/execution/tasks`。状态视图现使用 `/api/work-items`，执行与人员仍保留 Execution Task 证据边界。
- 状态视图的项目和负责人选项必须来自可见 Work Item 及权威项目目录，不能复用 Git 执行任务的提交者和仓库目录；跨视图筛选也必须在切换时清理。
- 右侧文字重叠来自桌面覆盖规则同时使用 `overflow: hidden`、正文行截断和隐藏后续列表项。检查器现独立纵向滚动，正文和列表完整可达。
- 任务详情与决策看板统一的是弹窗容器、事实网格、分区和响应式节奏，不复制决策业务字段，也不修改共享 `Modal`。
- 隔离登录态浏览器在 1280×720 和 390×844 验证了需求 / Bug 回车定位、数据边界、筛选目录、长文本检查器、详情弹窗和横向滚动归属；未进行外部请求或业务写入。

# 2026-07-31 - Task Table core-member candidate boundary

- Work Item 行的当前负责人是事实数据，不等于允许用户筛选或重新指派的候选目录；外部协同负责人可以留在表格中，但不能因此进入负责人下拉。
- 状态视图此前直接对可见 Work Item 的 `assignee` 去重，导致所有历史/外部负责人都成为候选；现改为复用 `/api/demands/options` 的 core member 目录。
- 后端候选接口此前在 `jira.sync_users` 已配置时仍无条件合并 custom JQL assignees，与中央可见性规则不一致。统一规则是 `sync_users` 优先，只有其为空时才从 JQL 回退。
- 执行追踪和人员负载仍使用 Execution Task facets；本次只收窄任务表状态视图的候选授权边界，不隐藏 Work Item，也不篡改行内负责人。
- 隔离登录态浏览器证明：外部负责人行仍可见，负责人下拉只显示核心成员。
