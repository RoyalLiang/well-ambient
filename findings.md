# Findings & Decisions

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
