## [LRN-20260720-004] correction

**Logged**: 2026-07-20T19:24:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
Bottom-gutter consistency cannot be inferred from a sibling route when the affected page owns a second viewport-height calculation.

### Details
The earlier task-tracking validation proved that the execution panel reached its shell gutter, but the Daily Jira component still calculated `window.innerHeight - rootTop - 16` independently of the shell. The user's screenshot exposed the resulting drift at the Daily Jira route even though a related page had already passed geometry checks.

### Suggested Action
For cross-page bottom-alignment claims, measure the exact affected route and a known-good sibling at the same viewport. Trace every ancestor in the height chain, record root-to-frame-content and root-to-viewport deltas, and reject any child-level `100vh` or `window.innerHeight` subtraction when the shell already owns the viewport.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/DailyJiraAudit.svelte, web/src/components/prototype/FunctionalWorkspace.svelte, task_plan.md
- Tags: bottom-gutter, height-ownership, exact-route-validation, visual-regression
- Pattern-Key: ui.viewport_fit.measure_exact_route
- Recurrence-Count: 1
- First-Seen: 2026-07-20
- Last-Seen: 2026-07-20

### Resolution
- Removed Daily Jira's component-local viewport calculation and resize frame.
- Added Daily Jira to the workspace viewport-fit height chain.
- Authenticated comparison at 2048x925 measured both Daily Jira and Decision Agenda at the same 22px shell gutter; 1024px and 760px checks preserved their responsive gutters with zero document overflow.

---

## [LRN-20260720-003] correction

**Logged**: 2026-07-20T10:35:00+08:00
**Priority**: high
**Status**: in_progress
**Area**: frontend

### Summary
A local surface correction is incomplete when it changes a route-specific background, leaves related controls outside their owning list, or synchronizes detail state without making the selected row visible.

### Details
The prior agenda-only white substrate fixed one screenshot in isolation but broke the shared page-background contract. Task Tracking also kept Project/Owner filters as a separate root-level card and global search updated the inspector without revealing the selected table row. All three symptoms come from ambiguous ownership across shell, list header, and selection behavior.

### Suggested Action
Assign one owner to each concern: the shell owns one flat page substrate and page scrolling; each list header owns its filters and actions; task selection owns both inspector synchronization and a one-shot local row reveal. Validate the complete shell-to-card compositing chain and both visible panels at the user's viewport.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/prototype/FunctionalAdminShell.svelte, web/src/components/TaskKanban.svelte, task_plan.md
- Tags: background-ownership, filter-ownership, visible-selection, local-scroll, visual-regression
- Pattern-Key: ui.task_tracking.surface_selection_ownership
- Recurrence-Count: 1
- First-Seen: 2026-07-20
- Last-Seen: 2026-07-20
- See Also: LRN-20260720-001, LRN-20260720-002, LRN-20260719-003

---

## [LRN-20260720-002] correction

**Logged**: 2026-07-20T09:37:23+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
Fixed-height inspector rows can still look merged when semantic sections use zero grid gap and a nested state surface fills the complete intermediate row.

### Details
The Daily Jira inspector satisfied its fixed-height and no-scroll geometry, but the facts, current-decision summary, and morning-decision form were laid out in adjacent zero-gap rows. The filled intermediate state card did not solve the hierarchy; it made the section feel attached to both neighbors and repeated the nested-card vocabulary already rejected elsewhere.

### Suggested Action
Make semantic section separation part of fixed-height validation. Assign explicit row rhythm at the inspector owner, flatten ordinary state summaries into the parent surface, reserve tinted fills for real semantic alerts, and verify section bounds plus scroll ownership at the user's viewport.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/DailyJiraAudit.svelte, task_plan.md
- Tags: inspector, section-rhythm, fixed-height, nested-card, visual-regression
- Pattern-Key: ui.daily_jira.inspector_section_rhythm
- Recurrence-Count: 1
- First-Seen: 2026-07-20
- Last-Seen: 2026-07-20
- See Also: LRN-20260719-003, LRN-20260719-002

### Resolution
- **Resolved**: 2026-07-20T09:53:00+08:00
- **Notes**: Replaced zero-gap inspector rows with 12px section rhythm, flattened the ordinary decision state to transparent hairlines, rebalanced fixed tracks for two-line titles and facts, and verified zero document/inspector overflow at 1440/1024/760/520 authenticated breakpoints.

---

## [LRN-20260720-001] correction

**Logged**: 2026-07-20T09:23:31+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
A transparent dashboard root can still regress to a gray summary band when the shell substrate shows through translucent cards and their gaps.

### Details
The Decision Agenda metrics and status-strip parents remained transparent, but the agenda route inherited the gray-blue `workspace-frame` background. Semi-transparent card fills and 12px gaps exposed that substrate across the screenshot's complete summary region. Checking only the page component's background was therefore insufficient; the rendered color comes from the complete shell-to-card compositing chain.

### Suggested Action
For route-specific surface corrections, inspect computed backgrounds through every ancestor and pseudo-element. Keep the shell as the substrate owner, use a route-specific flat surface when required, and validate both card interiors and inter-card gaps at the user's viewport. Add an isolation check proving sibling routes retain their own background.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/prototype/FunctionalAdminShell.svelte, web/src/components/DecisionDashboard.svelte, task_plan.md
- Tags: computed-background, shell-ownership, compositing, visual-regression
- Pattern-Key: ui.shell.substrate_compositing
- Recurrence-Count: 1
- First-Seen: 2026-07-20
- Last-Seen: 2026-07-20
- See Also: LRN-20260719-003

### Resolution
- **Resolved**: 2026-07-20T09:29:00+08:00
- **Notes**: Added an agenda-only frame class and assigned the existing flat surface token at the shell owner. Authenticated computed-style checks covered the full ancestor chain, four responsive widths, zero overflow, Daily Jira isolation, and clean console output.

---

## [LRN-20260718-001] best_practice

**Logged**: 2026-07-18T16:05:00+08:00
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
Adding a Go route is not live until the long-running `go run` backend is restarted and the actual proxied response code is verified.

### Details
The Daily Jira frontend and route source were correct, but the local backend had been running from a July 15 Go build. Vite served the new UI while `/api/decision/daily-jira` still reached the stale router and returned 404. Restarting the repository backend changed both direct 8080 and Vite-proxied 5173 requests to the expected protected-route 401 response.

### Suggested Action
After changing server routes, verify the live endpoint without a token: 404 means stale/missing routing, while 401 confirms the protected route is loaded. Restart only the confirmed repository backend process, then repeat both direct and proxy checks.

### Metadata
- Source: user_feedback
- Related Files: internal/server/server.go, internal/server/daily_jira_handlers.go, web/src/components/DailyJiraAudit.svelte
- Tags: go-run, stale-process, route-reload, vite-proxy, runtime-verification
- Pattern-Key: harden.runtime_route_reload
- Recurrence-Count: 1
- First-Seen: 2026-07-18
- Last-Seen: 2026-07-18

### Resolution
- Restarted `go run cmd/server/main.go` from the repository root.
- Confirmed `GET /api/decision/daily-jira` returns 401 instead of 404 through both ports 8080 and 5173.

---

## [LRN-20260719-003] correction

**Logged**: 2026-07-19T13:03:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
Balancing a full-height drawer must not be solved by wrapping each information group in another bordered card.

### Details
The previous correction removed the large lower dead zone and restored top spacing, but retained a bordered overview card, six inset fact capsules, four bordered narrative cards, and a bordered intervention card inside an already elevated drawer. The resulting geometry was balanced while the visual hierarchy became fragmented and over-componentized. The project baseline explicitly prefers one workbench surface with divider-based grouping over card-in-card rhythm.

### Suggested Action
Treat the drawer as the only elevated surface. Keep semantic sections for accessibility, but group facts and narratives with typography, whitespace, and sparse hairlines. Reserve separate surfaces only for truly independent interaction contexts, not every content label.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/DecisionDashboard.svelte, task_plan.md
- Tags: nested-cards, drawer, visual-hierarchy, flattening
- Pattern-Key: ui.drawer.single_surface
- Recurrence-Count: 1
- First-Seen: 2026-07-19
- Last-Seen: 2026-07-19
- See Also: LRN-20260719-002

### Resolution
- Removed all borders, radii, and filled backgrounds from the overview, fact cells, narrative sections, and intervention section inside the drawer.
- Preserved one cross-divider content matrix and one top rule for the operation area while keeping the existing full-height geometry and responsive scroll ownership.
- Authenticated computed-style checks confirmed transparent zero-radius inner sections at every tested desktop and narrow breakpoint.

---

## [LRN-20260711-001] correction

**Logged**: 2026-07-11T11:18:00+08:00
**Priority**: high
**Status**: pending
**Area**: frontend

### Summary
A flow-board scroll fix must preserve a definite visible board height in the real shell; `height: 0` with flex growth is unsafe when any ancestor height is only intrinsic.

### Details
The previous change made the board a zero-height flex item. In the authenticated shell the height chain was not definite at every level, so the lanes disappeared even though the CSS compiled successfully. The AI deconstruction reuse also exposed the entire workbench in a modal instead of adapting content and layout to the launching context.

### Suggested Action
Use an explicit viewport-clamped board height, verify actual shell behavior, and provide a compact contextual deconstruction surface with a side-panel variant for new-demand capture.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/DemandKanban.svelte, web/src/components/Deconstructor.svelte
- Tags: flow-board, flex-height, contextual-modal, ui-regression

---

## [LRN-20260713-001] correction

**Logged**: 2026-07-13T00:00:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
Capture-phase outside dismissal is not sufficient when a multi-select overlay covers the next form rows and the intended workflow keeps the list open after every selection.

### Details
The previous fixes treated continuous multi-selection as a reason to keep the list open after option toggles. Even after removing visual overlap and adding explicit completion controls, the live interaction still violates the user's expectation that an option choice completes and closes the dropdown. The shared single and multi components also duplicate window dismissal listeners, so their behavior can drift.

### Suggested Action
Close multi-select after every option toggle, restore focus to its trigger, and extract outside-pointer/Escape listener ownership into one shared lifecycle used by both single and multi select primitives.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/shared/MultiSelect.svelte, web/src/components/DemandDeliveryControl.svelte
- Tags: multiselect, overlay, completion, event-ordering, visual-obstruction
- See Also: LRN-20260712-002
- Pattern-Key: harden.multiselect_completion_boundary
- Recurrence-Count: 3
- First-Seen: 2026-07-12
- Last-Seen: 2026-07-13

### Resolution

- Multi-select now closes and restores trigger focus after every option select or deselect.
- `Select` and `MultiSelect` now share `selectLifecycle.ts` for capture-phase outside-pointer and Escape ownership and cleanup.
- Opening on focus was removed after real-component validation exposed layout click-through; deliberate click, input, ArrowDown, and chevron entry paths remain.

---

## [LRN-20260712-002] correction

**Logged**: 2026-07-12T23:20:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
Shared dropdown dismissal and people-option filtering must be validated inside the real modal propagation boundary and against the project's core-member source.

### Details
The first review-contract selector pass used bubbling `window.click` listeners. The demand-detail modal stops click propagation, so clicking modal blank space could not close either dropdown. The searchable triggers also only reopened the list and exposed no explicit close toggle. Separately, merging the full `/api/users` directory into owner selects bypassed the established core-member visibility boundary.

### Suggested Action
Use capture-phase outside-pointer handling plus Escape, focus-exit, and a real chevron toggle in both shared selectors. Build review people options by joining the user directory to `/api/demands/options` core-member identities, and filter the backend participant projection through `coreMemberVisibility`.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/shared/Select.svelte, web/src/components/shared/MultiSelect.svelte, web/src/components/DemandDeliveryControl.svelte, internal/server/autonomous_delivery_handlers.go
- Tags: dropdown-dismissal, event-propagation, core-member, review-contract
- Pattern-Key: harden.dropdown_capture_boundary
- Recurrence-Count: 1
- First-Seen: 2026-07-12
- Last-Seen: 2026-07-12

### Resolution

- Replaced bubbling outside-click listeners with capture-phase `pointerdown` plus focus-exit and Escape recovery in both shared selectors.
- Added an explicit toggle button to searchable chevrons; single selection closes immediately while multi-selection stays open only to support consecutive choices.
- Filtered backend review participants through `coreMemberVisibility`, filtered the frontend compatibility directory through `/api/demands/options`, and re-normalized saved owners whenever the core-directory signature changes.
- Focused Go tests, Svelte checks, production build, Impeccable detection, and diff hygiene pass.

---

## [LRN-20260713-002] correction

**Logged**: 2026-07-13T11:43:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
A multi-select selection session must remain open across option toggles and close on an explicit or outside dismissal action.

### Details
Phase 59 copied single-select completion semantics into `MultiSelect`: every option selection or removal closed the list and forced the operator to reopen it. The user clarified that this violates normal multi-select operation. The prior overlay obstruction was already solved by keeping the option region in flow, so it is safe and preferable to preserve one continuous multi-selection session.

### Suggested Action
Keep `MultiSelect` open after selection, deselection, and chip removal. Continue to close through capture-phase outside pointer, focus exit, Escape, the chevron, or the explicit `完成选择` action. Do not change `Select` semantics.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/shared/MultiSelect.svelte, web/src/components/DemandDeliveryControl.svelte
- Tags: multiselect, interaction-contract, consecutive-selection, outside-dismissal
- See Also: LRN-20260713-001, LRN-20260712-002
- Pattern-Key: harden.multiselect_completion_boundary
- Recurrence-Count: 4
- First-Seen: 2026-07-12
- Last-Seen: 2026-07-13

### Resolution

- Removed close-and-refocus calls from option toggles and expanded-state chip removal.
- Retained the shared capture-phase outside-pointer and Escape lifecycle plus explicit completion and chevron dismissal.
- Updated reviewer guidance to describe consecutive selection and outside-click completion.

---
## [LRN-20260719-002] correction

**Logged**: 2026-07-19T10:46:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
A zero-scroll geometry pass is not a valid UI review when the resulting drawer has an over-compressed top half and a large unused lower half.

### Details
The requirement drawer was mechanically compacted until its scroll delta reached zero. Although type, build, detector, and geometry checks passed, the user-provided screenshot showed a visibly unbalanced composition: headings, facts, summaries, and the intervention form were packed into the upper portion while roughly one third of the full-height drawer remained empty. The mandatory three-way review must evaluate the final rendered composition, not only rules and measurements.

### Suggested Action
For full-height drawers, make vertical space distribution an explicit review gate. Verify the occupied-content ratio, top/bottom visual mass, section hierarchy, control comfort, and final screenshot at the user's real viewport. Reject any layout that merely eliminates scrolling by shrinking content while leaving a large dead zone.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/DecisionDashboard.svelte, task_plan.md
- Tags: drawer, visual-balance, three-way-review, screenshot-validation
- Pattern-Key: ui.drawer.balance_before_scroll
- Recurrence-Count: 1
- First-Seen: 2026-07-19
- Last-Seen: 2026-07-19
- See Also: LRN-20260718-001

### Resolution
- Restored a comfortable 148px-class header, assigned elastic height to the narrative matrix, and anchored the intervention surface with an 18px bottom inset.
- Rejected and corrected one intermediate card-overflow state before delivery.
- Authenticated responsive checks now include content containment, section overlap, title wrapping, scroll ownership, and full-document geometry in addition to static detectors.

---

## [LRN-20260731-003] correction

**Logged**: 2026-07-31T17:24:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
Task Table owner options must come from the core-member candidate directory, not from assignees observed on visible Work Items.

### Details
The Work Item table correctly preserved real owner facts, but the filter dropdown derived its options from those same rows. That made external collaborators valid selectable candidates. The user clarified that row truth and candidate authorization are separate boundaries. The backend also merged custom-JQL assignees even when `jira.sync_users` already defined the core-member set.

### Suggested Action
Use `/api/demands/options` as the shared core-member candidate directory. Keep Work Item owners visible in rows, but never promote them into selection options. Apply `jira.sync_users` as the primary configured member source and use custom-JQL assignees only when it is empty.

### Metadata
- Source: user_feedback
- Related Files: web/src/components/TaskKanban.svelte, internal/server/demand_handlers.go, internal/server/server_test.go
- Tags: core-member, candidate-directory, work-item, authorization-boundary
- Pattern-Key: domain.candidate_directory_not_observed_facts
- Recurrence-Count: 1
- First-Seen: 2026-07-31
- Last-Seen: 2026-07-31

### Resolution
- Replaced Work Item-derived owner facets with the shared core-member directory.
- Corrected demand options to use the same `sync_users`-first, custom-JQL-fallback rule as the central visibility filter.
- Locked both boundaries with backend and frontend regression checks plus an isolated authenticated browser fixture.

---

## [LRN-20260731-004] correction

**Logged**: 2026-07-31T20:18:00+08:00
**Priority**: high
**Status**: resolved
**Area**: full-stack

### Summary
RBAC group membership is an authorization boundary, not the delivery owner directory.

### Details
Using RBAC core-group memberships for Task Table owner options reduced the list to three people. The Decision Dashboard contract is authoritative: delivery owner candidates come from configured Jira core members, with `sync_users` primary and custom-JQL assignees only as a fallback. Project candidates come from the shared Jira/version-source project catalog. Row owners, commit authors, repository names, and authorization groups must not be promoted into reusable dropdown options.

### Suggested Action
Expose one typed delivery directory endpoint and make every delivery owner/project dropdown consume it. Keep legacy endpoints as compatibility adapters and preserve currently selected historical values only locally where edit continuity requires it.

### Metadata
- Source: user_feedback
- Related Files: internal/server/delivery_directory.go, internal/server/task_tracking_options.go, web/src/lib/delivery-directory.ts, web/src/components/TaskKanban.svelte
- Tags: delivery-directory, core-member, rbac, project-catalog, dropdown
- Pattern-Key: domain.delivery_directory_not_authorization_group
- Recurrence-Count: 1
- First-Seen: 2026-07-31
- Last-Seen: 2026-07-31

### Resolution
- Added `/api/delivery/directory` as the common owner/project option source.
- Migrated delivery filters and forms away from visible-row, Git, repository, hardcoded, and RBAC-derived candidates.
- Added backend contract tests for configured-member and authoritative-project behavior.

---
