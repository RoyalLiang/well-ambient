# Task Plan: Implementation Plan Check and Fix

## Current Task Addendum: Shared Assignee And Project Directory Convergence

### Goal And Exact Symptom

- [x] Restore the task-table assignee dropdown from the accidental three-user RBAC subset to the same configured core-member directory used by the Decision Dashboard.
- [x] Audit every delivery-owner and delivery-project dropdown, remove row-derived and commit/task-prefix-derived candidates, and consume one normalized backend directory.
- [x] Keep page-specific selected values visible without allowing historical, local-only, or non-core values to pollute the reusable directory.

### Reproduction Baseline

- `TaskKanban.svelte` loads `/api/task-tracking/assignees`; that handler queries only users with `user_group_memberships`, so the live dropdown is structurally capped at the three RBAC members.
- `executionAssigneeOptions` adds every visible execution-row owner, including local `Vendor`/`Operator` values, so execution tracking can expose names that the Decision Dashboard does not treat as core members.
- Decision Dashboard and Demand Kanban use `jira.sync_users`, with custom-JQL assignees only as fallback, while Daily Jira derives choices from its current audit buckets. These sources can be complete, partial, or polluted for the same user.
- Project dropdowns currently come from at least four shapes: project config, project preferences, current WorkItem rows, agenda task-key prefixes, and execution facets. This can hide configured projects or display a key/name pair inconsistently.

### Mandatory Three-Way UI Review

- **Impeccable:** product register and earned familiarity. One typed directory owns option identity and labels; each page owns only selection, loading, empty, and error states. Preserve existing `Select`/`MultiSelect`, density, focus behavior, and responsive layout.
- **design-taste-frontend:** dense dashboard/table UI is outside its primary landing-page scope, so apply redesign-preserve only. Keep the current information architecture, tokens, typography, and control geometry; this task changes data ownership, not the visual system. `DESIGN_VARIANCE=3`, `MOTION_INTENSITY=1`, `VISUAL_DENSITY=9`.
- **finesse-ui:** Design Read: research-delivery governance console, restrained and evidence-first, `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`. A shared data contract prevents subtly different standard controls from becoming untrustworthy; motion remains existing state feedback only.

### Shared Direction And Disagreement Resolution

- Shared: backend owns the canonical directory. Assignees use configured Jira core members (`sync_users`, custom JQL only as fallback) and normalize aliases through the user directory; RBAC group membership is authorization, not delivery ownership.
- Shared: projects use the configured Jira/project-preference catalog with normalized key/name pairs. Task IDs, repositories, currently visible rows, and commit evidence never create project options.
- Shared: filters, create/edit forms, reassignment controls, release binding, and project mapping all consume the same identities. A currently selected legacy value may be appended locally for continuity but never enters the shared catalog.
- Impeccable normally avoids modal or visual churn for data defects; finesse similarly prioritizes component consistency. No disagreement remains because the existing controls and geometry stay unchanged.

### Ranked Hypotheses And Red Loops

1. Confirmed: the three-person symptom is caused by `/api/task-tracking/assignees` using RBAC membership instead of configured core members. A focused handler test must expect configured members and reject RBAC-only users.
2. Confirmed: execution tracking facet pollution is caused by merging visible local task owners into the reusable assignee facet. Its regression must keep local rows visible while excluding their owners from the directory.
3. Confirmed: project-option loss and label drift come from rebuilding options from WorkItem rows or agenda task-key prefixes instead of the authoritative project catalog.
4. Confirmed architectural cause: there is no shared typed delivery-directory response, so every page independently implements filtering, alias normalization, fallback, and sorting.

### Component Ownership And Validation Contract

- Backend: a single authenticated read endpoint returns normalized `assignees` and `projects`; the existing demand/task/execution option responses delegate to the same builder where compatibility is required.
- Frontend: Decision Dashboard, Task Table/Execution Tracking, Demand/Schedule forms, Daily Jira reassignment, Release Plan project controls, and Project Mapping names consume the shared response. Project Preferences already uses the same backend catalog and remains compatible.
- Loading failure is fail-safe: existing data stays visible, the current selection is preserved, and no fallback to commit rows, repositories, task prefixes, or arbitrary users occurs.
- Static validation: focused Go regressions, related server suite, Svelte check/build, diff check, and Impeccable/Finesse detection.
- Browser validation: authenticated Decision Dashboard, Task Table, Execution Tracking, Demand/Schedule, Daily Jira, Version Plan, and Project Mapping at desktop and narrow breakpoints; compare exact option sets/labels and verify no overflow or console errors.

### Plan

- [x] Add and run red-capable directory regressions for configured members, RBAC-only users, local execution owners, and authoritative projects.
- [x] Implement the shared backend directory and compatibility delegates.
- [x] Migrate affected frontend controls without changing their visual contract.
- [x] Run static, detector, and authenticated multi-route browser validation.

### Validation Result

- Backend now exposes `/api/delivery/directory`. Its assignees use configured Jira core members (`sync_users` first, custom-JQL fallback) with user-directory alias normalization; its projects use the Jira/version-source project catalog. Legacy task, execution, demand, and Daily Jira option responses delegate to the same builders.
- Decision Dashboard, Task Table, Execution Tracking, Demand/Schedule, Daily Jira reassignment, Version Plan, Deconstructor, and Project Config mapping consume the shared directory. Project Preferences already consumes the same backend project catalog; project rails, role enums, and Jira source-configuration controls were audited and intentionally remain domain-specific.
- Red regressions proved RBAC-only users and visible local Vendor/Operator owners no longer enter reusable candidates, while configured core members and authoritative projects remain available. The complete `internal/server` and `internal/db` suites pass.
- `pnpm --dir web check` passes with 0 errors and existing warnings; the production build, exact-file Impeccable detection (`[]`), Finesse P0 gate, and `git diff --check` pass.
- Authenticated isolated browser validation confirmed the same 10 owner options and 4 project options in Decision Dashboard, Task Table, Execution Tracking, and Demand/Schedule; Version Plan exposed the same 4 projects. At 1440x900 and 390x900, the Task Table menu contained all 10 owners, stayed inside the viewport, had zero document overflow, and the narrow trigger surface measured 44px. A fresh authenticated Task Table tab had no console errors.
- The browser pass exposed an empty-project-config `null.filter` crash; `DemandKanban` now normalizes that payload to `[]`. The post-fix Svelte check/build and detector gates pass. No production database, Jira, GitLab, Feishu, AI, assignment, or review write was executed; the isolated services and temporary database were removed.
- **Status:** complete

---

# 2026-07-31 FZ-2247 双仓库代码轨迹实时回流

## 缺陷契约与当前证据

- [x] 建立同一 Jira 主任务、`task_executor` 与 `crane_manager` 各一条 commit 证据的失败复现。
- [x] 确认 `ExtractTaskID` 把 `FZ-2247` 转成 `fz-2247`，新 Git 证据与 Jira 主记录分裂，轨迹接口的精确查询因此漏数。
- [x] 确认本地 `webhook_logs` 已收到 `task_executor` 的 FZ-2247 Push Hook，但没有任何 `crane_manager` webhook 记录；第二仓库当前尚未进入系统。
- [x] 确认代码轨迹组件打开后只加载一次，应用级通知 SSE 更新不会触发已打开轨迹刷新。
- [x] 修复大小写身份归并、批量 push 中非末条 Jira key 识别、历史大小写证据读取和实时刷新。
- [x] 验证双仓库证据、历史数据兼容、打开态实时刷新、关闭态事件隔离及响应式稳定性。

## 三方 UI 评审结论（编辑前门禁）

- **Impeccable**：交互层应保持单一所有者。`App.svelte` 继续拥有 SSE 连接并发布语义化的 telemetry 更新事件；`CommitTelemetryPanel.svelte` 只在自身打开且事件命中当前任务时刷新，继续独立拥有 skeleton、错误、空态和手动重试。
- **design-taste-frontend**：该技能明确不主导后台数据表，本次仅采用 redesign-preserve。保留现有 Phase 41 浅色管理台、轨迹弹窗、字体、密度、按钮与响应式结构；不增加依赖、装饰、轮询或新视觉主题。
- **finesse-ui**：Design Read 为“研发交付协同工具，证据优先、克制高密度”，`register=product`、`SOUL=4 / SPECTACLE=1 / DENSITY=9`、`hero-engine=none`。实时反馈必须由真实状态事件驱动，不使用装饰性动画，也不允许刷新造成列表清空抖动。
- **共同方向**：应用级 SSE 是唯一实时触发源；组件通过带 `task_id` 的轻量事件精准刷新，保留手动刷新作为容错路径。刷新期间保留现有证据，成功后原子替换，避免可见内容闪空。
- **分歧与取舍**：design-taste-frontend 的适用范围不包含仪表盘，因此不参与视觉创作决策，只提供保留约束；Impeccable 与 finesse-ui 共同决定组件所有权、状态反馈和验证范围。

## 保护边界与验证范围

- 不改变轨迹弹窗几何、层级、关闭路径、视觉 token、任务表/执行追踪表结构和 Jira/GitLab 写入语义。
- 不通过前端轮询补偿 webhook 缺失；`crane_manager` 未入库必须作为集成配置缺口独立暴露。
- 后端聚焦测试覆盖：已存在大写 Jira 主记录、两仓库提交、历史小写日志、同批非末条 commit 含 Jira key。
- 前端静态检查与浏览器覆盖：面板打开后收到命中事件自动刷新，其他任务事件不刷新，关闭后不刷新，手动刷新仍可用，加载时旧证据不闪空。
- 所有运行验证使用本地或隔离数据库，不执行真实 Jira 写回、GitLab 配置修改或生产部署。

## 验证结果

- 后端现在把 `FZ-2247`/`fz-2247` 归并到已有 Jira 主记录，历史大小写日志也能从同一个轨迹接口返回；Git 推送作者只作为证据作者，不再覆盖 Jira 负责人或把任务移出 coremember 可见范围。
- Push Hook 会扫描批次内所有提交。隔离回放中，`crane_manager` 的首条提交含 Jira 号、末条不含 Jira 号，两条都正确归入 `FZ-2247`；`task_executor` 与 `crane_manager` 最终各 2 条提交，且没有生成小写重复任务。
- SSE 新增带 `task_id` 的 `telemetry-updated` 事件；打开的轨迹弹窗只响应当前任务，并在后台刷新时保留旧列表。浏览器中下一条 `task_executor` webhook 无需点击刷新即把 PUSH 从 3 更新为 4。
- 1440×900、760×900、390×844 三个视口均无文档或面板横向溢出，弹窗关闭路径有效，浏览器 warning/error 为 0；Impeccable 返回 `[]`，Finesse P0 为 0。
- `go test ./... -count=1`、`pnpm check`（0 errors，既有告警）、`pnpm build` 与定向 `git diff --check` 均通过。
- 当前数据库审计仅见 `task_executor` 的 FZ-2247 webhook，未见任何 `crane_manager` webhook 投递；代码可正确处理第二仓库，但实际环境仍需在 GitLab 配置中确认/补齐该仓库 webhook。此次未执行外部 GitLab 配置写入或生产部署。
- **Status:** complete locally; production webhook delivery follow-up required

## Current Task Addendum: Execution Evidence Projection, Core-Member Facets, And Release Project Pills

### Goal And Boundaries

- [x] Remove the personnel-load page from the shell, app state, task-tracking view switcher, and rendered branch; legacy `personnel` state falls back to the task table.
- [x] Expand execution tracking from the three explicit execution rows to a work-item evidence lens: one row per real Requirement/Bug with Git evidence, plus explicit execution tasks, without restoring commit-message rows.
- [x] Make the task-table owner dropdown use actual RBAC group members; do not reuse Jira sync/custom-JQL assignee candidates.
- [x] Render the release-plan project value as a compact semantic pill while preserving the existing table density and missing-project warning.

### Mandatory Three-Way UI Review

- **Impeccable:** product register, restrained visual hierarchy, existing components and tokens. Removing a navigation branch must remove its state and fallback paths as well. The release project pill should be a semantic label, not a new decorative component.
- **design-taste-frontend:** this dense product table is outside its primary landing-page scope, so only redesign-preserve applies. Keep the Phase 41 information architecture, typography, density, routes, and control vocabulary; do not introduce a new design system.
- **finesse-ui:** Design Read is a research-delivery control surface, register=product, SOUL 4, SPECTACLE 1, DENSITY 9. Facts, evidence, and filters must remain separate; motion is limited to existing state feedback.

### Shared Direction And Disagreements

- Shared: delete `人员负载` as a complete product surface, not only its visible navigation label.
- Shared: execution tracking is an evidence lens over real work items and explicit execution tasks. Git commits remain timeline evidence and must never become one table row per commit.
- Shared: Core Member options come from users with RBAC group membership. Jira synchronization scope and custom JQL are integration configuration, not UI membership authority.
- Shared: use the existing pill/radius/token language for project ownership. Missing ownership retains its warning tone and remains textually explicit.
- Finesse normally prefers avoiding pills as decoration; here the project value is a compact categorical fact in a dense table, so the semantic use is justified.

### Ranked Hypotheses And Red Loops

1. Confirmed: the execution handler queries only execution kinds. The live database has 12 such rows, 9 are commit-derived and correctly excluded, leaving 3. Meanwhile 36 distinct Jira-shaped Git evidence keys all join to existing Requirement/Bug rows.
2. Confirmed: task owner options call `/api/demands/options`; live `sync_users` is empty, so the handler falls back to custom-JQL assignees instead of the 3 users with RBAC memberships.
3. Confirmed: personnel state is independently declared in `App.svelte`, `FunctionalAdminShell.svelte`, and `TaskKanban.svelte`, so deleting one entry would leave reachable or stale branches.
4. Confirmed: `DeliveryPlan.svelte` renders the bound project as an unclassed span; no data change is needed.

### Validation Contract

- Backend regression: evidence-backed Requirement/Bug rows are projected once, explicit execution tasks remain, and commit-derived telemetry rows remain absent.
- Membership regression: task-tracking owner options contain only distinct users with group membership; an empty RBAC directory remains empty rather than falling back to Jira configuration.
- Frontend static contract: no `personnel` task-view state or visible label remains; old `personnel` route input normalizes to `status`.
- Browser: task owner dropdown contains only RBAC members, execution count exceeds three without commit-message rows, version projects display as pills, and desktop/760/390 layouts remain stable.
- **Status:** complete

### Validation Result

- Backend: focused regressions and the complete `internal/server` + `internal/db` suites pass. The evidence query uses the covering Git evidence index, WorkItems use the task tracking index/primary key, and RBAC members use indexed memberships plus the user primary key.
- Live data boundary: 35 real Requirement/Bug rows currently have Git evidence; 11 match the configured execution-visibility assignees, so the endpoint is no longer structurally capped at the three explicit execution rows.
- Frontend: production build passes. No `personnel` state, label, rendered branch, or related CSS remains in the task surface; execution risk defaults to `全部`, while explicit risk-focus navigation still selects `需关注`.
- Browser fixture at 1440/760/390: task owner options are exactly the three RBAC members; execution tracking displays `6 / 6` and six rows with no commit-derived key; project pills remain visible with `999px` radius; no document-level horizontal overflow or console errors were observed.
- Impeccable reports no finding in `DeliveryPlan.svelte`; changed legacy surfaces retain only their pre-existing P2 pure-white findings. Finesse reports no P0 findings.

---

## Current Task Addendum: Workspace-Stage-Scoped Requirement Dialog

### Goal

Keep the `录入新产品需求` dialog and its create-flow AI companion entirely inside the right-side workspace below the application header. The rail, topbar, and optional degraded-auth banner must remain visually and interactively outside the backdrop.

### Reproduction Baseline

- At `1280x720`, the expanded rail ends at `x=270` and the real `.workspace-stage` is `x=270, y=112.5, width=1010, height=607.5`.
- The current create backdrop is `x=270, y=0, width=1010, height=720`. It passes the rail boundary but covers the `68px` topbar and the `44.5px` local maintenance banner.
- The modal center is `y=360`, while the workspace-stage center is `y=416.25`; therefore the popup is centered against the full right column rather than the actual main-content stage.

### Mandatory Three-Way UI Review

- **Impeccable:** the shell owns both horizontal and vertical workspace bounds. Expose the measured topbar plus optional banner height as one inherited semantic token; keep the overlay fixed so the workspace's internal scroll cannot move it.
- **design-taste-frontend:** use redesign-preservation mode. Do not change copy, controls, colors, typography, dimensions, actions, focus behavior, or responsive information architecture for a boundary correction.
- **finesse-ui:** product-register ownership belongs to the app shell. Scope the token consumption to the new-demand flow and its AI companion, and validate the actual rectangle rather than relying on visual plausibility.

### Agreement And Disagreement Resolution

- Shared agreement: `.workspace-stage` is the source-of-truth geometry. The overlay rectangle must match its visible viewport bounds: rail-right to viewport-right, stage-top to viewport-bottom.
- Shared agreement: a hard-coded `68px` top offset is insufficient because the degraded-auth banner is conditional and can wrap responsively. Measure both shell-owned elements and publish their combined height.
- Shared agreement: retain fixed positioning rather than placing the overlay inside the scrolling `.workspace-frame`; the form and companion must not move with the table scroll.
- Potential disagreement: portalling the overlay into `.workspace-stage` and using `position:absolute` would remove the need for a vertical token. Resolve against it because the mobile workspace becomes content-height/overflow-visible and would center the dialog against the document instead of the visible stage.
- Potential disagreement: change the shared modal backdrop globally. Resolve against it because unrelated decision, schedule, and confirmation flows have different ownership and were not requested.

### Ranked Hypotheses

1. Confirmed: the previous fix changed only the backdrop's left edge while the inherited `position: fixed; top: 0` continued to cover the header.
2. Confirmed design risk: a static topbar token would still miss the optional banner and responsive wrapping.
3. Rejected: z-index alone caused the visual overlap. The measured backdrop rectangle itself begins at `y=0`.
4. Rejected: the failure requires a global shared-modal redesign. The create-demand selectors already provide a safe narrow seam.

### Responsive And Validation Contract

- Expanded desktop: backdrop rectangle equals `.workspace-stage`; rail and header remain outside it.
- Collapsed desktop: the left boundary follows the `94px` rail while the top boundary continues to match the stage.
- Mobile: the rail contributes `0px`; the overlay begins below the measured mobile header/banner and fills only the remaining visible viewport.
- Host and create-flow companion share the same stage rectangle and stay fully visible without document overflow.
- No demand, AI, Jira, or production-database write is allowed during validation.

### Plan

- [x] Reproduce the incorrect backdrop rectangle and record the three-way review.
- [x] Publish the measured workspace-stage top boundary from the shell and consume it in the create-flow overlays.
- [x] Re-run static gates and authenticated browser geometry checks for expanded, collapsed, mobile, and AI companion states.

### Validation Result

- Expanded desktop (`1280x720`, degraded banner visible): `.workspace-stage` and the create backdrop both measure `x=270, y=112.5, width=1010, height=607.5`; the dialog and stage share center `(775, 416.25)`.
- Collapsed desktop with the dialog already open: the header's rail toggle remains clickable; stage and backdrop both update to `x=94, y=112.5, width=1186, height=607.5`, with the dialog recentered at `x=687`.
- Mobile (`760x900`): the backdrop starts below the measured `64px` header and `44.5px` degraded banner at `y=108.5`, ends at the viewport bottom, and preserves the dialog's `14px` horizontal inset with zero document overflow.
- Wide desktop AI companion (`2048x934`): host and companion backdrops both match the stage rectangle `x=270, y=112.5, width=1778, height=821.5`; the panels remain contained with a `16px` gap and zero document overflow.
- Browser console errors are empty. Validation used the isolated database copy with Jira and AI disabled; no demand, AI, or Jira write was submitted.
- `pnpm --dir web check` passes with `0 errors` and the existing `73 warnings in 4 files`; `pnpm --dir web build` and `git diff --check` pass.
- Exact-file Impeccable detection returns `[]`. Finesse reports `p0: 0`; only the files' pre-existing P2 pure-white findings remain.

## Current Task Addendum: Main-Content-Scoped Requirement Dialog

### Goal

Center the `录入新产品需求` dialog against the right-side application content rather than the full browser viewport, while keeping the left navigation outside the visual modal boundary and preserving the complete short-viewport form behavior.

### Design Read

Targeted preservation adjustment for a dense authenticated delivery-governance workbench. Keep the current light table-first product register, compact form density, teal action semantics, and existing dialog dimensions. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; this changes overlay ownership, not the dialog's information architecture or visual language.

### Mandatory Three-Way UI Review

- **Impeccable:** the application shell owns the rail/main-content boundary. Expose that boundary as one inherited semantic offset, then let the requirement backdrop occupy `offset → viewport right`; avoid duplicating rail widths inside page CSS. Keep the dialog centered by the backdrop's flex layout and preserve the existing fixed header/footer plus body scroll owner.
- **design-taste-frontend:** this skill is landing-page oriented, so apply only its redesign-preservation rules. Do not change form fields, copy, typography, tokens, dialog width, actions, or responsive information architecture for a positioning request.
- **finesse-ui:** product register owns the decision. Scope the change to the requirement-create flow and its AI companion instead of globally changing every shared modal. At desktop the active work area is the centering reference; at mobile the off-canvas rail contributes no permanent inset.

### Agreement And Disagreement Resolution

- Shared agreement: `FunctionalAdminShell.svelte` owns a `--wa-main-content-offset` token derived from the expanded/collapsed rail state; the token becomes `0px` when the rail is off canvas.
- Shared agreement: `DemandKanban.svelte` applies the token only to the create-demand backdrop and the create-demand AI companion backdrop. Both use the same horizontal containing block so paired-dialog geometry remains stable.
- Shared agreement: vertical centering, viewport-height containment, dropdown placement, focus/close behavior, form values, and submission behavior remain unchanged.
- Potential disagreement: hard-coding `270px` and `94px` directly in the page is a smaller one-file edit. Resolve against it because rail geometry belongs to the shell and would drift when the shell changes.
- Potential disagreement: changing the shared `Modal.svelte` would make all dialogs follow the same boundary. Resolve against it because the request refers to the new-requirement dialog, and a global portal/overlay change would affect unrelated flows without evidence.

### Responsive And Validation Contract

- Expanded desktop rail: backdrop left edge equals the main-content left edge and dialog horizontal center equals the right content area's horizontal center.
- Collapsed desktop rail: both measurements update to the compact rail boundary without re-opening the dialog.
- `<=860px`: the rail is off canvas, the backdrop returns to the full usable viewport, and the existing 14px/12px responsive inset and single body scroll owner remain intact.
- AI pre-deconstruction companion: host and companion share the same main-content centering reference and do not overlap at wide desktop widths.

### Protection Rules

- Preserve dialog content, actions, date/dropdown behavior, body scroll containment, API writes, toasts, sidebar state, unrelated modals, shared modal behavior, and all unrelated dirty-tree changes.
- Do not submit a demand or invoke a real AI/Jira write during validation.

### Plan

- [x] Inspect shell, rail, modal, and companion ownership and record the three-way review.
- [x] Add one shell-owned main-content offset and consume it in the requirement-create overlays.
- [x] Run frontend checks, exact-file design detection, and browser geometry validation for expanded/collapsed/mobile states.

### Validation Result

- Expanded desktop (`1280x720`): the main content and requirement backdrop both begin at `x=270`; the dialog center and main-content center both equal `775`.
- Collapsed desktop (`1280x720`): the compact rail boundary, main content, and backdrop all begin at `x=94`; the dialog center and main-content center both equal `687`.
- Mobile (`760x900`): the inherited offset becomes `0px`; the backdrop fills the viewport and the dialog retains its existing `14px` horizontal inset with zero document overflow.
- Wide desktop with AI companion (`2048x934`): both backdrops exactly match the `x=270 / width=1778` main-content area; the `680px` host and `525px` companion remain fully visible with a `16px` gap and zero document overflow.
- `pnpm --dir web check`, `pnpm --dir web build`, and `git diff --check` pass. The exact-file Impeccable detector returns `[]`; Finesse reports `p0: 0` and only the files' pre-existing P2 pure-white findings.
- Browser validation used an isolated database copy with Jira and AI disabled. No demand, AI, or Jira write was submitted.

## Current Task Addendum: Requirement Modal, Daily Jira Title, And Release Creation

### Goal

Keep the new-requirement dialog complete at every usable viewport height, show the full selected Jira title in the Daily Jira inspector, and give Version Plan one permission-gated creation path for an independent local release version that may be bound to a project later.

### Design Read

Dense authenticated delivery-governance product for experienced users. Preserve the current light table-first workbench, compact type scale, teal action semantics, and existing information architecture. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; these are containment and lifecycle-entry fixes, not a visual redesign.

### Reproduction Baseline

- New requirement: the dialog is 601px tall at 1280x720, but `.demand-create-modal` overrides the bounded base modal with `overflow: visible !important`; below 649px viewport height its 601px content exceeds the 48px-inset usable area and the form/footer have no owned scroll region.
- Daily Jira: the selected 100-character `NS2-1860` title has `scrollHeight=117px`, `clientHeight=47px`, and `overflow:hidden`, proving that the inherited two-line clamp hides 70px of the title.
- Version Plan: an authenticated manager sees no `创建版本` control; `POST /api/releases` returns `405 Method Not Allowed`, while only the project-scoped compatibility path exists.

### Mandatory Three-Way UI Review

- **Impeccable:** each dialog is one bounded surface with fixed header/footer and one `min-height: 0` body scroll owner. Daily Jira must allocate natural height to the selected title and keep the inspector as the only long-content scroll owner. Release creation must expose validation, loading, success, error, and permission states.
- **design-taste-frontend:** use preservation mode. Keep the existing tokens, density, list/inspector hierarchy, custom date control, and responsive architecture; do not solve clipping by shrinking type or removing fields.
- **finesse-ui:** product register owns the decision. Put `创建版本` beside the existing toolbar actions, use the shared modal and toast vocabulary, and keep Jira association as a subsequent searchable relationship rather than crowding it into creation.

### Agreement And Disagreement Resolution

- Shared agreement: `DemandKanban.svelte` owns its custom create-dialog header/body/footer contract; the body becomes the sole vertical scroll owner and dropdown/date overlays remain locally placed.
- Shared agreement: `DailyJiraAudit.svelte` removes the two-line clamp only in the inspector title, allows full wrapping, and lets the inspector content height/scroll contract absorb long titles without changing table-row density.
- Shared agreement: `DeliveryPlan.svelte` owns the permission-gated create modal; `POST /api/releases` is the canonical page-level write entry and accepts an optional project. The existing project-scoped POST remains as a compatibility route and delegates to the same creation logic.
- Shared agreement: Jira is not linked during release creation. The created local release is selected immediately; the existing searchable Jira association panel remains the explicit next step.
- Potential disagreement: requiring a project would reuse the old endpoint with a smaller diff, but it conflicts with the frozen domain rule that a Release Version can exist before project binding. Resolve in favor of optional project validation at the page-level endpoint.
- Potential disagreement: making the entire requirement dialog scroll is simpler, but it hides actions and creates unstable close/submit access. Resolve in favor of fixed header/footer plus one body scroller.

### Component Ownership And Responsive Contract

- `DemandKanban.svelte`: bounded create-dialog flex chain, scrollable `.form-body`, fixed action footer, short/narrow viewport insets.
- `DailyJiraAudit.svelte`: naturally wrapping inspector heading, minimum-width protection, inspector-owned overflow.
- `DeliveryPlan.svelte`: create action, shared modal form, custom `DatePicker`, submit lifecycle, toast, and immediate selection.
- `release_handlers.go` and `server.go`: optional-project canonical create route with shared validation/persistence; no Jira writeback.
- Desktop: toolbar remains compact, title wraps without overlap, modal stays centered with actions visible.
- Short viewport: dialog remains within `100dvh`; only the form body scrolls.
- Narrow viewport: modal uses the 12px shared inset, actions stack only when needed, controls remain at least 44px high, and no document overflow is introduced.

### Protection Rules

- Preserve all demand fields, assignment behavior, Jira facts/links, decision workflow, release identity fields, searchable Jira association, permissions, routes, toasts, and unrelated dirty-tree changes.
- Do not create or update a real Jira item and do not validate against the live database. Browser mutations use only an isolated copied database.
- Do not merge project binding and Jira association into release identity; neither relationship may overwrite the local version name, status, dates, or source.

### Plan

- [x] Capture failing geometry, clipping, missing-control, and missing-route baselines.
- [x] Complete the mandatory three-way UI review and record ownership/responsive decisions.
- [x] Implement the requirement-modal and Daily Jira title fixes.
- [x] Add canonical version creation with handler and UI regression coverage.
- [x] Run static gates, exact-file design detection, and authenticated browser validation.

### Validation Result

- At 1280x720, the new-requirement dialog is contained at approximately `y=59–661`; its header and footer remain fixed, `.form-body` is the sole vertical scroll owner, and the custom date panel opens upward inside both the form and dialog. Document overflow is zero. The `100dvh` height cap and narrow-screen inset provide the same containment contract at shorter/narrower viewports.
- The Daily Jira `NS2-1860` inspector title changed from `clientHeight=47px / scrollHeight=117px` to `117px / 117px`. The full title wraps without overlapping the facts below, the inspector owns long-content scrolling, and document overflow remains zero.
- In an authenticated browser backed by an isolated database copy, Version Plan successfully created an unbound local version, refreshed and selected it immediately, displayed `未绑定项目`, and emitted the upper-right `版本已创建` toast. Project binding remains optional, while Jira association stays in the existing searchable inspector workflow. No real Jira write was performed.
- `go test ./internal/server -count=1` and `go test ./internal/deliveryplanning ./internal/db -count=1` pass. `pnpm --dir web check` passes with 0 errors and 73 existing warnings in 4 files; `pnpm --dir web build` and `git diff --check` pass.
- Exact-file Impeccable detection on the three affected frontend components returns `[]`. Finesse reports `p0: 0`; Delivery Plan and Daily Jira have no findings, while Demand Kanban retains only pre-existing P2 pure-white findings. Browser console errors are empty.

## Current Task Addendum: Decision Modal Viewport Completeness

### Goal

Keep the Decision Dashboard requirement modal fully composed at short and narrow viewports: the dialog stays inside the usable viewport, its title and actions remain visible, and only the detail content scrolls.

### Design Read

Dense authenticated governance console for experienced delivery managers. Preserve the existing light workbench, compact information hierarchy, teal action semantics, and modal presentation. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; this is a containment and scroll-ownership repair, not a visual redesign.

### Mandatory Three-Way UI Review

- **Impeccable:** the modal is one bounded surface with three owned regions: fixed header, single scrollable body, and fixed footer. Add explicit `min-height: 0` to the flex scroll chain and use the dynamic viewport so mobile browser chrome cannot clip the surface.
- **design-taste-frontend:** use preservation mode because this is dense product UI. Keep typography, colors, facts, fields, controls, and responsive information architecture unchanged; solve the defect at the shared modal boundary instead of shrinking content.
- **finesse-ui:** product register owns the decision. Keep the decision actions persistently visible in a restrained footer separated by one hairline. Do not introduce a nested card, decorative treatment, or second competing scroll owner.

### Agreement And Disagreement Resolution

- Shared agreement: `Modal.svelte` owns viewport containment and the header/body/footer flex contract; `DecisionDashboard.svelte` owns only its business content and action buttons.
- Shared agreement: dialog height is bounded by `100dvh` minus the backdrop inset, the body is the only vertical scroll owner, and the footer participates in the existing focus trap.
- Shared agreement: desktop validation must cover the supplied screenshot proportions plus a 1440x720 short viewport; narrow validation must cover 390x844 with touch-safe controls and no document overflow.
- Potential disagreement: a sticky action row inside the body is a smaller diff, but it remains coupled to body padding and scroll position. Resolve in favor of an optional shared footer slot outside the body scroller.

### Protection Rules

- Preserve every Jira fact, link, intervention field, action, mutation contract, toast behavior, route, typography, color, and unrelated dirty-tree change.
- Do not change the drawer variant, timeline drawer, other modal content, API behavior, or submit a live Jira decision during validation.
- Keep backdrop, Escape, close-button, focus-trap, and focus-return behavior unchanged.

### Plan

- [x] Reproduce the clipped first paint and capture a failing geometry assertion at 1440x720.
- [x] Add a shared fixed footer region and dynamic-viewport containment.
- [x] Move Decision Dashboard actions into the footer without changing business behavior.
- [x] Run static gates, design detectors, and authenticated desktop/short/narrow browser validation.

### Validation Result

- The failing 1440x720 assertion showed a 585px body with 646px of content and the action row beginning at `y=698.6`, below the body bottom at `y=690.2`. After the fix, the body owns the overflow, the footer begins exactly at its bottom, and both actions remain visible inside the dialog at `y=645–679`.
- At the supplied 1920x934 proportions, the full 606px body fits without scrolling, the fixed footer remains visible, the dialog is contained at `y=94.1–839.9`, and the document matches the viewport with no overflow.
- At 390x844, the dialog is contained within a 12px inset, the body has one 361px fallback scroll range, the fixed actions remain visible at `y=775–819`, both action targets are 44px high, and the document has no horizontal or vertical overflow.
- Focusing the final read-only form field scrolls the body to its maximum while leaving the footer fixed. Escape closes the modal and returns focus to the originating `ICA-10231` table row. Browser logs contain no errors, and no decision action was submitted.
- `pnpm --dir web check` passes with 0 errors and 73 existing warnings in 4 files; `pnpm --dir web build` and `git diff --check` pass. Exact-file Impeccable detection returns `[]`; Finesse reports `p0: 0` with no file findings.

## Current Task Addendum: Daily Jira Bottom Ownership And Jira Version-Link Audit

### Goal

Make the Daily Jira table and inspector terminate on the same shell-owned bottom gutter as the adjacent Decision Agenda view, and determine whether Jira release/version URLs such as `/projects/PRJ25024/versions/13622` are currently ingestible.

### Design Read

Targeted preservation fix for a dense authenticated operations console. Keep the Phase 41 light product register, system sans, current table density, 20px panel radius, teal semantics, and existing decision workflow. `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; motion remains feedback-only.

### Mandatory Three-Way UI Review

- **Impeccable:** the screenshot shows a containment contract problem, not a need for another padding or shadow tweak. The decision shell already owns viewport height, bottom gutter, and overflow. Daily Jira must consume that remaining block through the same explicit `height: 100% / min-height: 0` chain as Decision Agenda, with table and inspector remaining the only internal scroll owners.
- **design-taste-frontend:** use redesign-preserve mode. Do not change the navigation, typography, palette, table rows, inspector content, or responsive information architecture. Remove the page-local viewport math instead of compensating it with another magic subtraction.
- **finesse-ui:** product register owns the decision. Keep the two aligned workbench surfaces and one locked shell gutter. The highest-leverage fix is component ownership: shell computes available space once; Daily Jira fills it; no document/body scroll and no decorative bottom band.

### Agreement And Disagreement Resolution

- Shared agreement: `FunctionalAdminShell` owns the viewport and shell gutter; `FunctionalWorkspace` owns the remaining content track; `DailyJiraAudit` owns only its grid and internal table/inspector scrolling.
- Shared agreement: add Daily Jira to the existing decision viewport-fit selector and replace the JavaScript `window.innerHeight - top - 16` calculation with `height: 100%`.
- Shared agreement: desktop validation must compare Daily Jira and Decision Agenda bottom rectangles in the same viewport and scroll position; narrow validation must keep the existing stacked layout and internal table-scroll path.
- Potential disagreement: reducing the component height by another fixed number could mimic the screenshot at one resolution. Resolve against it because it duplicates shell padding and will drift when the breadcrumb wraps or browser scale changes.

### Protection Rules

- Preserve Jira data, decision records, routes, cohort semantics, table row height, inspector fields, shell gutter tokens, and unrelated dirty-tree changes.
- Do not create, update, assign, synchronize, or review any Jira/demand record during validation.
- The Jira version-link question is an evidence-backed capability audit. Do not add a new ingestion workflow unless the user explicitly asks for it after the current behavior and extension path are clear.

### Plan

- [x] Reproduce and measure the Daily Jira bottom ownership against the decision shell.
- [x] Connect Daily Jira to the existing viewport-fit height chain and remove page-local viewport math.
- [x] Trace Jira URL/JQL synchronization and confirm current version-link support boundaries.
- [x] Run frontend checks, design detectors, and authenticated desktop/narrow browser validation without business mutation.

### Validation Result

- Daily Jira now consumes the shell-owned viewport track through `FunctionalWorkspace`; its component-local `window.innerHeight - top - 16` calculation and resize frame are removed.
- At the supplied 2048x925 viewport, Daily Jira and Decision Agenda both end at `y=903`, exactly 22px above the viewport bottom and flush with the workspace content edge. Document overflow is zero, both Daily Jira panels end together, and the Jira table shell remains the sole long-list scroll owner.
- The same ownership holds at 1024x900 and 760x900: the root is flush with the workspace content edge, preserves the responsive 14px/10px shell gutter, and creates no document overflow.
- The authenticated Jira version page identifies version `13622` as `ReeWell-1.1`, reports 21 current-version issues, and exposes the issue-navigator JQL `project = 11900 AND fixVersion = 13622`. The current application can retrieve that issue set through `custom_jql`, but it does not parse the release-page URL or resolve a version URL automatically.
- No Jira sync, demand mutation, assignment, or review action was executed. Validation used an isolated copy of the database with Jira, GitLab, Feishu, and AI disabled; the temporary runtime was stopped.
- `pnpm check` passes with 0 errors and 71 existing warnings in 5 files; `pnpm build` and `git diff --check` pass. Exact-file Impeccable detection returns `[]`; Finesse reports `p0: 0` and no file findings. Authenticated browser console errors are empty.

## Current Task Addendum: Task Filter Multi-Select And Surface Rhythm

### Goal

Make the task-tracking Project and Owner filters fully readable and multi-selectable, align the execution workbench bottom inset with the other task views, and remove remaining gray card-within-card fills from secondary task modules.

### Design Read

Preservation redesign for an authenticated, high-density delivery console. Keep the existing Phase 41 light workbench, system sans, teal semantic accent, compact table rows, and current information architecture. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; motion remains feedback-only.

### Mandatory Three-Way UI Review

- **Impeccable:** replace the duplicated single-value task controls with the shared accessible `MultiSelect`; empty selection means all, selected values remain visible through a bounded summary, and the dropdown must escape table scrolling/clipping while preserving keyboard, focus, empty, and disabled states. Give the execution view its own viewport-height budget because it intentionally omits the stage strip. Flatten gray inner fact blocks rather than adding a new parent card.
- **design-taste-frontend:** apply preservation guidance only because the target is dense product UI. Keep current tokens, typography, row density, routes, and copy. Use one familiar component vocabulary, one radius scale, and explicit 760px/mobile collapse. Add no dependency, theme, decorative motion, or broad visual rewrite.
- **finesse-ui:** product register owns the decision. Use standard multi-select behavior with checkmarks and a clear selected-count summary; keep table and inspector as the only strong workbench surfaces; secondary facts, summaries, and submodules use transparency, spacing, and hairlines instead of gray nested cards. Touch targets remain at least 44px on narrow screens.

### Agreement And Disagreement Resolution

- Shared agreement: `TaskKanban.svelte` owns both local multi-select values and query serialization; the task and execution endpoints accept repeated `project` and `assignee` parameters so multi-select filtering remains server-correct and never broadens the saved personal project scope.
- Shared agreement: the filter control stays one row tall and reports `全部项目` / `全部负责人`, the selected label, or `已选 N 项`; individual selections remain visible in the open list rather than expanding the toolbar into multiple chip rows.
- Shared agreement: execution, status, and personnel keep their current workbench ownership and scroll regions. Only the execution panel height receives the missing-stage-strip offset so every view lands on the shell bottom gutter.
- Shared agreement: flatten only nested gray fills inside task inspector facts, personnel summary cells, and execution secondary facts; semantic warning/danger/success washes remain because they convey state.
- Potential disagreement: rendering selected chips exposes every value, but in a dense toolbar it causes variable height and repeats the clipping problem. Resolve in favor of a fixed-height count summary plus checked options in the dropdown.
- Potential disagreement: a global `--wa-surface-inset` token change would remove more gray at once, but it would alter unrelated pages and control states. Resolve with scoped TaskKanban overrides only.

### Component Ownership And Responsive Contract

- `web/src/components/shared/MultiSelect.svelte`: optional compact summary/overlay presentation while preserving the existing profile-preference behavior by default.
- `web/src/components/TaskKanban.svelte`: project/owner arrays, filter summaries, repeated query params, aligned panel height, and scoped secondary-surface flattening.
- `internal/server/server.go` and `internal/server/execution_handlers.go`: repeated query filtering with existing personal-project and core-member intersections intact.
- Desktop: search keeps the widest column; risk, Project, Owner, and refresh stay fully readable; dropdowns are bounded and unclipped.
- 1024/760: controls wrap into explicit grid rows; at <=760px they become one column with 44px controls. Workbench panels become content-height under the existing single-column breakpoint.

### Protection Rules

- Preserve task/Jira data, status/risk meanings, project preferences, core-member visibility, routes, table columns, row density, inspector content, and unrelated dirty-tree work.
- Empty local filter arrays mean all visible projects or all visible core owners. Personal project preferences remain the upper-bound scope.
- Do not create, update, assign, sync, or review Jira/demand records during validation.
- Do not change the global surface token, main navigation, shell gutter, or browser/document scroll owner.

### Plan

- [x] Add server-correct repeated Project/Owner filtering and focused tests.
- [x] Add bounded shared multi-select summary presentation and replace task filters.
- [x] Align execution bottom inset and flatten scoped gray inner surfaces.
- [x] Run checks/build/detectors plus authenticated desktop/1024/760/480 browser validation.

### Validation Result

- Repeated and comma-separated Project/Owner query parsing is covered by unit tests; project values use union semantics, owner values use union semantics, and the two dimensions intersect after the authenticated user's saved project scope.
- Full `internal/db`, `internal/server`, and `internal/agenda` Go suites pass. `pnpm --dir web check` passes with 0 errors and the existing 72-warning baseline; the production build and `git diff --check` pass.
- Exact-file Impeccable detection returns `[]`. Finesse reports no P0 findings in either changed frontend file; the remaining side-stripe matches are existing semantic timeline tracks.
- Authenticated isolated validation selected HIT plus NS2 and reduced execution rows from 390 to 43; adding 梁志远 plus 朱家聪 reduced the intersection to 3, and every result matched both selected dimensions. Clearing both filters restored 390 rows.
- At 2048px the execution toolbar stays on one row and the workbench lands about 12px above the shell bottom. At 1024, 760, and 480px the controls reflow without document overflow; narrow controls and dropdown actions meet the 44px touch target.
- The task inspector fact groups, personnel summary facts, and personnel priority rows now compute to transparent inner surfaces with hairline separation; semantic status fills remain intact.
- Fresh authenticated navigation to the execution view produced no console errors. Validation used a copied database with external integrations disabled; no demand/Jira record was created, updated, assigned, synchronized, or reviewed, and both isolated services were stopped afterward.

## Current Task Addendum: Unified Workspace Surface And Task List Ownership

### Goal
Restore one shared page substrate across the admin console, make a global-search task selection visibly selected in the left task table, and move Project/Owner filters into the task list header instead of a standalone root-level card.

### Design Read
Dense enterprise execution console for experienced delivery managers. Preserve the existing light workbench, compact rows, teal selection semantics, and current information architecture. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; this is a targeted ownership correction, not a theme redesign.

### Three-way Review
- **Impeccable:** the shell must own one flat page substrate and vertical page scrolling; the task list must own its title, result count, filters, actions, visible selection, and local row reveal. Remove the standalone filter surface and do not add another wrapper.
- **design-taste-frontend:** preserve the product UI and design tokens, flatten the hierarchy, and recover list prominence by integrating the filters instead of shrinking 46px data rows or adding decorative surfaces.
- **finesse-ui:** treat this as a targeted redesign of component ownership. Keep the selected state obvious but restrained, keep interactions immediate and feedback-only, and make responsive changes inside the list toolbar without changing business workflows.

### Agreement And Disagreement Resolution
- Shared agreement: `FunctionalAdminShell` owns the page background; `TaskKanban` remains transparent at page level; real cards/tables retain their existing surface tokens.
- Shared agreement: status and personnel filters belong inside their respective list header containers. Execution already follows this ownership and remains unchanged.
- Shared agreement: global search selection is complete only when the inspector and a visible highlighted row agree. Row reveal is one-shot; it uses the table's local vertical owner when the table is bounded, otherwise the existing `.workspace-frame` owner at narrow breakpoints, never `document/body`.
- Potential disagreement: retain the shell's subtle top gradient or enforce an exact uniform substrate. The user's explicit request that the main background and inter-card gaps match resolves this in favor of a single flat `--wa-bg-page` fill.
- Potential disagreement: at 1440px force every toolbar onto one line or allow an internal second line. Density and legibility resolve this in favor of an adaptive toolbar: one container, with internal wrapping when width requires it.

### Component Ownership
- `FunctionalAdminShell.svelte`: one page background and existing workspace scroll policy; remove the agenda-only white substrate.
- `TaskKanban.svelte`: task filters, status/personnel list toolbars, selected-row semantics, and one-shot local row reveal.
- Global search shell: navigation and selection event only; no business-data mutation and no document/body scrolling.

### Responsive Contract
- 1440: list/inspector remain two columns; filters sit in the list header and may use a compact internal row.
- 1024: existing single-column workbench remains; Project/Owner controls stay two columns inside the list header.
- 760 and 520: title, filters, and actions stack within the same list header; controls use touch-safe height; table keeps its own horizontal boundary and the workspace keeps vertical ownership.

### Validation Scope
- [ ] Pre-change authenticated computed-style audit for Task Tracking, Decision Agenda, and Daily Jira substrate/gap colors. **Blocked:** local loopback bind was denied and the scoped approval stream disconnected.
- [ ] Search `FEL2WD-1532`; confirm right detail, visible left row, `aria-selected=true`, selected styling, and local table/workspace-owner scroll without document movement. **Source contract complete; authenticated interaction pending.**
- [ ] Validate status, personnel, and execution views at 1440/1024/760/520; verify filter dropdowns are unclipped and no standalone filter row remains. **Responsive source review complete; authenticated pixels pending.**
- [ ] Confirm Decision Agenda and Daily Jira now expose the same `--wa-bg-page` substrate between cards while card interiors retain their own surfaces. **Single-owner source contract complete; computed pixels pending.**
- [x] Run exact-file Impeccable layout detection, frontend check/build, Finesse detection, and final diff audit.
- [ ] Run authenticated console checks after a safe local listener is explicitly approved.

### Protection Rules
- Do not change task data, Jira decisions, APIs, routes, metric meanings, table row density, or inspector content.
- Do not add a page-level background to `TaskKanban`, a new filter card, or browser/document scrolling.
- Do not let the one-shot reveal repeat on periodic refresh or ordinary filter changes.

## Superseded Addendum: Decision Agenda Gray Substrate Regression

> Superseded by **Unified Workspace Surface And Task List Ownership** above. The user's later cross-page comparison invalidated the agenda-only white substrate; the shared shell background is now the governing decision.

### Goal
Remove the gray substrate visible across the Decision Agenda summary region in the user's screenshot without adding another wrapper card, changing metric geometry, or affecting Daily Jira.

### Design Read
Morning-triage product UI for experienced delivery managers. Preserve the Phase 41 light workbench, compact 4-by-2 summary hierarchy, and teal semantics. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; targeted surface regression fix only.

### Live Audit
- `.decision-metrics`, `.decision-stage-strip`, `.decision-admin`, `.workspace-content`, and `.functional-workspace` all compute to transparent backgrounds with no shadow or pseudo-element fill.
- The gray is the agenda route's `workspace-frame` substrate (`rgb(238, 244, 247)` plus a translucent white top gradient) showing through the 12px card gaps and the cards' translucent fills.
- The route currently has zero document overflow and stable `68px / 48px / remainder` tracks; the complaint is surface ownership, not layout geometry.

### Protection Rules
- Preserve all metric values, labels, status filters, card/chip dimensions, 12px gaps, table height, routes, APIs, focus behavior, and responsive breakpoints.
- Preserve the shell-owned background contract; do not paint `.decision-admin` or introduce a summary wrapper/card.
- Keep Daily Jira and all non-agenda routes unchanged. Do not submit any live Jira mutation during validation.

### Mandatory Three-Way UI Review
- [x] Impeccable: the shell owns page substrate. Give only the Decision Agenda frame one flat opaque work surface; do not add a bordered, rounded, or shadowed summary container. Cards remain the information units, while the frame supplies continuity through their gaps.
- [x] design-taste-frontend: use preservation mode because this is dense product UI. Make a single color correction at the route boundary; keep the established card grid, copy, typography, and interaction model unchanged.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`. Replace the composited gray with the existing flat surface token at the frame owner. No new component, gradient, decoration, motion, or second accent.
- Shared hierarchy and ownership: `FunctionalAdminShell.svelte` owns the Decision Agenda route substrate through a dedicated frame class. `DecisionDashboard.svelte` continues to own metrics, filters, table, and drawers without gaining an outer visual frame.
- Responsive behavior: the correction is color-only at every breakpoint. Existing wide 4-column and narrow responsive grids, viewport-fit height chain, internal table scrolling, and mobile touch behavior remain unchanged.
- Validation scope: live computed background/class ownership; populated agenda at current user viewport plus 1440/1024/760/520 widths; no document/workspace overflow; unchanged metric/stage/table rectangles; Daily Jira background unchanged; console review; `pnpm check`, production build, both design detectors, and diff hygiene.
- Disagreement and resolution: painting each card opaque would still leave gray gutters, while painting `.decision-admin` would violate shell ownership and recreate a page-level inner frame. Apply the flat surface only to the shell's agenda-specific workspace frame.

### Phases
- [x] Phase 1 - inspect screenshot and authenticated computed styles, then complete the renewed three-way review
- [x] Phase 2 - add the agenda-specific shell surface without changing component geometry
- [x] Phase 3 - run static, responsive, route-isolation, and console validation

### Current Phase
Superseded - the agenda-only `--wa-surface-flat` decision described above was removed after the user identified cross-page background drift. Its historical validation remains evidence for the prior state only and must not be treated as the current surface contract.

## Current Task Addendum: Flatten Requirement Drawer Hierarchy

### Goal
Remove the excessive nested-card appearance from the requirement detail drawer while preserving its information, visual balance, stable title behavior, viewport ownership, and decision workflow.

### Design Read
Morning-triage product UI for experienced delivery managers. Preserve the Phase 41 light workbench and teal semantics. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=8`; targeted hierarchy flattening, not a new component system.

### Existing-Surface Audit
- The drawer is already the primary elevated surface, but the overview, six fact cells, four narrative sections, and intervention form each add their own border, radius, or fill.
- Repeated white cards and inset fact capsules make every block appear equally important, weaken the title-to-content hierarchy, and recreate the configuration-center card-in-card pattern that the project contract explicitly rejects.
- The useful two-column content proportion and full-height balance are sound; the problem is component chrome, not information architecture or data density.

### Protection Rules
- Preserve every fact, description, recommendation, flow item, intervention record, field, action, status, link, API call, focus behavior, and responsive access path.
- Preserve the 148px-class header, full-height drawer, `1.3fr / 0.7fr` content weighting, proportional narrative rows, 18px desktop bottom inset, hidden-rail fallback scrolling, and viewport-locked document.
- Do not change the global shell, table, timeline drawer, typography family, accent palette, routes, backend contracts, or submit a live Jira decision.

### Mandatory Three-Way UI Review
- [x] Impeccable: the drawer is the one card. Remove internal section borders, radii, white fills, and fact capsules. Use headings, whitespace, and a small number of hairlines to group related content. Nested cards are always wrong.
- [x] design-taste-frontend: this is dense product UI outside the skill's main marketing scope, so use preservation mode. Keep the current data hierarchy and controls; apply the high-density rule that generic card containers are banned and plain layout should carry the information.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=8`. Convert the four narrative cards into one editorial matrix with one vertical and one horizontal divider. Keep artificial decoration at zero and let the bottom operation area read through a single top rule.
- Shared hierarchy and ownership: `DecisionDashboard.svelte` keeps one elevated drawer, one flat overview region, one flat narrative matrix, and one decision region. Fact values are direct definition-list cells without individual surfaces. Narrative sections retain semantic `<section>` ownership but lose card chrome.
- Responsive behavior: desktop/tablet preserve the asymmetric 2x2 matrix. At `<=460px`, the matrix becomes one column and replaces cross-grid borders with sparse horizontal separators. Short-height and narrow layouts retain the existing intrinsic-flow hidden-rail fallback.
- Validation scope: card-chrome count and computed border/background/radius audit; screenshot record plus a long-title record; 1440/1024/760/520/390 widths and 650px height; no document overflow; no content overlap; form actions contained; close/backdrop/Escape/focus return; `pnpm check`, production build, Impeccable/Finesse detection, diff hygiene, and authenticated console review without live mutations.
- Disagreement and resolution: a tinted intervention background would improve immediate grouping but would create another rectangular card. Use only a top hairline plus spacing. The primary buttons already provide sufficient action emphasis.

### Phases
- [x] Phase 1 - audit the current chrome and complete the renewed three-way review
- [x] Phase 2 - flatten facts, narrative sections, and intervention grouping while preserving geometry
- [x] Phase 3 - run design/static gates and authenticated responsive validation

### Current Phase
Complete - the requirement drawer is now the only elevated card. The overview and all six fact cells have transparent backgrounds, zero radius, and no individual borders; the four narrative sections share one asymmetric matrix with one horizontal and one vertical hairline; the intervention area uses only a top rule. Authenticated checks at 1440/1024/760 found zero document and drawer-body scroll delta, no overlap, contained actions, and an 18px bottom inset. The 520 layout remains fully visible, while 390px and 1024x650 retain hidden-rail fallback scrolling with every action reachable and no document overflow. The long Jira title remains two lines in a 148px header. Close button, backdrop, Escape, and focus return pass; the current console is clean. `pnpm check` passes with 0 errors and the unchanged 72-warning baseline, the production build and both design detectors pass, and `git diff --check` is clean. No live Jira decision was submitted.

## Current Task Addendum: Requirement Drawer Visual Balance Correction

### Goal
Correct the requirement drawer shown in the user's screenshot: restore comfortable spacing in the upper information hierarchy, eliminate the large unused lower region, and preserve the no-browser-scroll contract through intentional full-height composition rather than mechanical compression.

### Design Read
Internal delivery-governance product UI used during morning triage. Preserve the light teal workbench and familiar admin controls. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=8`; targeted visual-balance correction, not a new design system.

### Screenshot Audit
- The header, fact cells, narrative cards, and intervention form are packed into the upper portion with uniformly tight padding, making the title and section hierarchy feel compressed.
- The drawer body uses start-aligned intrinsic tracks, so its content stops well above the viewport bottom and leaves a large white dead zone that reads as unfinished rather than intentionally calm.
- The narrative grid contains uneven content depth; the short intervention-record card creates a second local void while the flow summary carries most of the text.
- Passing zero-scroll geometry did not validate visual balance. The previous review should have rejected the final screenshot.

### Protection Rules
- Preserve every Jira fact, description, recommendation, flow item, intervention record, decision field/action, route, API, row activation, and accessible drawer behavior.
- Preserve the existing type family, teal semantic accent, radius/control vocabulary, full-height drawer, hidden browser-edge rail, and viewport-locked document.
- Do not submit any live decision mutation during validation.

### Mandatory Three-Way UI Review
- [x] Impeccable: restore a comfortable 148px-class header and 4px-grid padding rhythm. Replace start-packed body flow with three owned tracks: overview, elastic narrative grid, and intervention. The final section must land near the viewport bottom with a deliberate bottom inset, not a dead zone.
- [x] design-taste-frontend: this dashboard remains outside the skill's marketing focus, so apply preservation mode only. Reduce density from extreme to readable high-density, keep the current brand/system, and solve the screenshot through spacing, hierarchy, and proportion instead of decorative treatment or a second component library.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=8`. Use full-height composition: comfortable title/meta, stable overview, elastic two-row detail matrix, bottom-anchored decision surface. Give the lower narrative row more space than the upper row because the flow summary contains materially more content.
- Shared hierarchy and ownership: `DecisionDashboard.svelte` owns `148px header + body(auto overview / minmax detail / auto intervention)`. The body owns only fallback short-screen scrolling. Detail cards stretch within deliberate `0.85fr / 1.15fr` rows; empty intervention history centers within its assigned card so space reads as an empty state rather than accidental whitespace.
- Responsive behavior: at desktop/tablet 900px-class heights the drawer fills the viewport with balanced spacing and zero scroll delta. Below 640px or on short heights, tracks return to intrinsic flow and retain hidden-rail scrolling so no content is clipped.
- Validation scope: the exact screenshot record and a long-title record; top/bottom occupied ratio and section rectangles at the user's real viewport; 1440/1024/760/520 widths; 900px-class and 650px short heights; no document overflow; no visible rail; no card/form overlap; close/backdrop/Escape/focus return; final rendered screenshot review, check/build, both design detectors, diff hygiene, and console review.
- Disagreement and resolution: stretching every intrinsic card equally would only move the blank area inside the cards. Allocate elastic height at the narrative-grid level, use unequal row ratios matching content depth, and center only the genuine empty-history state. The user-visible density becomes comfortable without inventing filler.

### Root Cause
- The previous correction optimized scroll delta first, reducing the header to 132px and compacting every block while leaving `.requirement-drawer-body` start-aligned.
- The content therefore occupied only the upper portion of the full-height drawer; the remaining viewport height was rendered as an unowned white region.
- Detector and geometry passes were insufficient because the final screenshot's visual mass and bottom occupancy were not explicit gates.

### Phases
- [x] Phase 1 - inspect the user screenshot, record the correction, and complete the renewed three-way review
- [x] Phase 2 - implement balanced header/body tracks, comfortable spacing, proportional detail rows, and intentional empty state
- [x] Phase 3 - run screenshot-led geometry, interaction, static, and design validation

### Current Phase
Complete - the drawer now uses a 148px-class title header, a comfortable overview, a `1.3fr / 0.7fr` narrative column split with `0.85fr / 1.15fr` rows, a centered genuine empty-history state, and an intervention surface that finishes with an 18px bottom inset. Authenticated checks at 1440/1024/760 widths and 900px-class heights found zero drawer-body or document scroll delta, no card/form overlap, contained actions, and a 44px close target. At 520/390 widths and a 650px short height, the hidden-rail drawer remains the sole fallback scroll owner while the document stays viewport-locked and all content remains reachable. The long-title record wraps to two lines within its fixed 147px header without moving or overlapping the body. The user screenshot was the visual-balance audit source; automated final pixel capture was blocked by missing macOS Accessibility/Screen Recording permission, so the post-edit gate used the authenticated rendered DOM, measured rectangles, interactions, and responsive containment rather than claiming an unavailable screenshot. `pnpm check` passes with 0 errors and the unchanged 72-warning baseline, the production build and both design detectors pass, `git diff --check` is clean, and the current authenticated browser console reports no errors. No live decision mutation was submitted.

## Current Task Addendum: Compact Requirement Drawer Without Browser-Edge Scrollbar

### Goal
Compress the Decision Agenda requirement drawer so a normal desktop viewport shows the complete detail surface without a scrollbar at the browser edge, while preserving every fact, section, decision field, action, keyboard behavior, and short-viewport access path.

### Design Read
Internal delivery-governance product UI for rapid meeting triage. Preserve the existing light high-density workbench and teal semantic vocabulary. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; targeted preservation redesign with no decorative motion.

### Protection Rules
- Preserve the full drawer information set, Agenda/Jira data, intervention inputs and actions, API behavior, row activation, focus trap/return, Escape/backdrop/button close, and existing responsive full-width drawer contract.
- Do not change routes, labels, the accent, typography family, backend contracts, list geometry, or the independent event timeline drawer.
- Keep scrolling available by wheel, touch, and keyboard on genuinely short viewports even when the scrollbar rail is visually hidden.
- Do not submit a live reassignment or due-date mutation during validation.

### Mandatory Three-Way UI Review
- [x] Impeccable: the browser document must remain locked to the viewport. Compress the fixed drawer header, card padding, grid gaps, and form rows; remove the visible scrollbar rail without disabling the drawer body's short-viewport overflow. Retain 44px close target, focus states, and all dialog semantics.
- [x] design-taste-frontend: this dense admin surface is outside the skill's primary marketing scope, so use preservation mode only. Keep the established light teal console, component vocabulary, copy, information architecture, and behavior; solve the complaint through spacing and rhythm rather than a visual-system rewrite.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`. Use a 4-column compact fact grid with the project fact spanning two columns, two-column content summaries with intrinsic heights, and a balanced 2x2 intervention form. Avoid forced equal-height whitespace and nested scroll chrome.
- Shared hierarchy and ownership: `DecisionDashboard.svelte` remains the sole owner of drawer state and content. The header is fixed and compact; the drawer body remains the only fallback scroll owner. The document, shell, and workspace frame stay non-scrollable on the decision route.
- Responsive behavior: desktop and tablet should fit the full drawer at ordinary 900px-class heights without a visible rail. Below 640px the drawer remains edge-to-edge, the facts use two columns, and short screens keep invisible-rail scrolling so content is never clipped.
- Validation scope: authenticated current and long-title records; document/drawer/body geometry at desktop plus 1180/1024/760/520 widths; no visible scrollbar rail; no clipped facts, summaries, inputs, actions, or alerts; wheel/keyboard access at a deliberately short height; close button/backdrop/Escape/focus return; Svelte check/build, design detectors, diff hygiene, and browser console review.
- Disagreement and resolution: forcing `overflow: hidden` on the drawer body would remove the rail but make short-screen content unreachable. Keep `overflow-y: auto`, remove the rail chrome, and compress the ordinary-height layout until its scroll delta is zero.

### Root Cause
- The browser document is already viewport-locked at `968/968px`; the apparent browser scrollbar is the drawer body's right-edge rail (`814px` client height versus `866px` scroll height).
- A 154px header, 195px fact block, stretched 199px summary row, and three-row 196px intervention form consume more vertical space than their information requires.
- `scrollbar-gutter: stable` reserves a persistent edge rail, making the nested scroll owner visually indistinguishable from a browser scrollbar.

### Phases
- [x] Phase 1 - inspect authenticated drawer geometry and complete the mandatory three-way review
- [x] Phase 2 - implement compact drawer spacing, intrinsic sections, balanced form rows, and hidden rail
- [x] Phase 3 - run static/design gates and authenticated responsive/short-viewport validation

### Current Phase
Complete - the drawer header is reduced from 154px to 132px, the fact block from 195px to 144px, and the intervention form from three rows/196px to a balanced two-row/128px layout. Intrinsic section heights remove forced blank card space, the 4-column fact layout gives the project value two columns, and the drawer body hides its edge rail while retaining `overflow-y: auto`. Authenticated long-title checks showed no header/meta overlap. Exact 1440/1024/760/520x900 validation reported zero document and drawer-body scroll delta, a 44px close target, visible contained actions, and no horizontal overflow. At 390x900 and 1024x650, the document remained viewport-locked while the invisible-rail drawer preserved content access; a real wheel action reached the 130px maximum scroll and exposed the action row. Escape and backdrop close still return focus to the triggering row. `pnpm check` passes with 0 errors and the unchanged 72-warning baseline, the production build passes, both design detectors return no findings, `git diff --check` is clean, and the browser console reports no errors. No live decision mutation was submitted.

## Current Task Addendum: Decision Dashboard Detail Drawer And Viewport Compression

### Goal
Replace the persistent requirement-detail column on the Decision Dashboard with a polished row-triggered right drawer, reduce and align the top metrics, and fit the list workbench into the remaining browser viewport without a page-level vertical scrollbar.

### Design Read
Internal delivery-governance product UI for fast meeting triage. Preserve the Phase 41 light table-first console and restrained teal semantics. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; targeted preservation redesign with feedback-only drawer motion.

### Protection Rules
- Preserve routes, labels, Agenda/Jira data, visibility rules, filters, selection state, decision APIs, event center, and the existing semantic token/component vocabulary.
- Do not change the primary font, accent, global navigation, or backend contracts.
- Limit implementation to the Decision Dashboard plus the shell/workspace height ownership required to remove the outer scrollbar.
- Do not submit a live reassignment or due-date change during validation.

### Mandatory Three-Way UI Review
- [x] Impeccable: remove the fixed inspector column and let the full-width table own the primary workbench. A whole-row pointer or keyboard activation opens one page-level right drawer with backdrop, Escape close, focus trap, and focus return. The drawer header stays fixed; its body is the only detail scroll owner. The route frame itself must not scroll.
- [x] design-taste-frontend: dense product UI is outside the skill's primary marketing scope, so apply preservation mode only. Keep the light console, teal focus/selection, typography, routes, controls, row density, and API behavior. Do not add Atlaskit, a second card vocabulary, decorative gradients, or spectacle motion.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`. Reduce duplicated six-card metrics to four aligned compact cards (`可见事项 / 高风险 / 中风险 / 自动记录`) because requirement/bug totals already exist in the status strip. Give the remaining height to one full-width table and use progressive disclosure for the details and intervention form.
- Shared hierarchy and ownership: compact four-card metrics -> compact four-way status strip -> full-width list workbench. `DecisionDashboard.svelte` owns selected item, row activation, drawer state, focus behavior, and drawer content. `FunctionalAdminShell.svelte` owns decision-route frame scrolling. `FunctionalWorkspace.svelte` owns breadcrumb plus remaining-height content allocation. The global `DecisionEventCenter` remains independent.
- Responsive behavior: wide layouts keep four metrics and four status controls in one row; medium layouts use two compact rows while preserving list height from the remaining viewport. The detail drawer is 560px on wide screens, bounded to the viewport, and edge-to-edge below 640px. The table may scroll internally; the drawer body may scroll internally; neither the browser nor the workspace frame may scroll vertically.
- Validation scope: authenticated populated state; short and long titles; whole-row pointer plus Enter/Space; drawer header/body geometry, Jira link, facts, description, advice, flow, history, and intervention form; close button/backdrop/Escape/focus return; 2133/1440/1180/1024/760/520 viewports; zero document/workspace-frame vertical overflow and zero document horizontal overflow; list remains internally scrollable; Svelte check/build, diff hygiene, Impeccable/Finesse detection, and current browser console review.
- Disagreement and resolution: forcing every long detail section into a fixed no-scroll drawer would clip real content or create empty gaps. Keep only the drawer shell and header fixed, and allow exactly one scroll owner in the drawer body. Removing the requirement/bug metrics is intentional de-duplication, not information loss, because the adjacent status strip continues to expose both counts and filters.

### Root Cause
- Six 143px metric cards plus a 70px status strip consume 229px before the list, while the fixed 793px two-column workbench extends 278px below the 905px viewport.
- The shell keeps the Decision Agenda workspace frame at `overflow-y: scroll`, producing a 300px frame scroll range even though the table already owns its own overflow.
- The persistent 716px inspector column cuts the table width to 988px and makes the page height depend on an oversized shared panel-height calculation instead of the remaining viewport.

### Phases
- [x] Phase 1 - inspect the authenticated desktop geometry and complete the mandatory three-way review
- [x] Phase 2 - implement compact metrics, viewport ownership, full-width list, and accessible detail drawer
- [x] Phase 3 - run static/design gates and authenticated interaction/responsive validation

### Current Phase
Complete - the Decision Agenda now uses four compact metrics in a 68px row, a 48px four-way status strip, and a full-width list that consumes the bounded remainder of the route frame. The persistent inspector is removed; pointer, Enter, or Space activation on a row opens a full-height right drawer with a fixed two-line header, one detail-body scroll owner, Jira/fact/content/intervention sections, backdrop/button/Escape close, focus trap, and focus return. Authenticated validation at 2133/1440/1180/1024/760/520 widths found zero document or workspace-frame overflow, preserved internal table scrolling, stable long-title geometry, and a 44px mobile close target. Daily Jira retained its existing viewport-fit and single-scroll-owner contract. `pnpm check` passes with 0 errors and the unchanged 72-warning baseline, the production build and both design detectors pass for the changed dashboard/workspace surfaces, `git diff --check` is clean, and the current browser console reports no errors. No live decision mutation was submitted during validation.

## Current Task Addendum: Workspace-Wide Decision Event Notification

### Goal
Make the latest decision-event notification visible in every breadcrumb-enabled workspace panel, keep its existing inline visual contract, and let the same click open the full-height reverse-chronological event drawer without depending on the Decision Dashboard route being mounted.

### Mandatory Three-Way UI Review
- [x] Impeccable: one compact event signal belongs to the shared breadcrumb surface, not to any route body. The application layer owns event loading and drawer state so switching panels cannot clear the notification or break its action. Keep the existing keyboard-operable row, semantic dot, truncation, focus return, Escape close, and single drawer scroll owner.
- [x] design-taste-frontend: this is dense product UI outside the skill's primary marketing scope, so use preservation mode only. Keep the current light workbench, teal/blue semantics, typography, route labels, responsive breadcrumb schema, and notification styling. Do not introduce another component library, card, ticker animation, or page-level visual redesign.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`. Treat the event as a global operational signal with exactly one presentation per workspace. Keep feedback-only motion and the existing `type / Jira / message / time / detail` schema; the drawer remains the progressive-disclosure surface for the complete ledger.
- Shared hierarchy and ownership: `App.svelte` owns the latest-event summary and open request. A workspace-level event timeline component owns polling, event normalization, full-height drawer state, and focus behavior. `FunctionalWorkspace.svelte` remains the only notification presenter. Route components no longer determine whether the notification exists.
- Responsive behavior: every breadcrumb-enabled route uses the same existing three-zone desktop row and full-width secondary row below the current breakpoint. The drawer remains a bounded right panel on desktop and edge-to-edge on narrow screens. Daily Jira must retain its viewport fit after the notification appears.
- Validation scope: authenticated decision agenda, Daily Jira, schedule, evidence, tasks, and KPI routes; identical latest event across routes; direct drawer opening from a non-decision route with no route jump; newest-first timeline; backdrop/button/Escape close and focus return; 1024/760/520 geometry; zero document horizontal overflow; Daily Jira zero browser-level vertical overflow; Svelte check/build, Impeccable/Finesse detection, and browser console review.
- Disagreement and resolution: navigating to the Decision Dashboard before opening the drawer would reuse the existing route-owned implementation but would break the user's current context. The shared event center therefore owns the drawer at workspace level. Keep the legacy Decision Dashboard timeline code untouched in this narrow fix to minimize regression risk, but stop using it as the shared notification source.

### Phases
- [x] Phase 1 - trace current route gating, event loading, unmount clearing, and drawer ownership
- [x] Phase 2 - complete and record the mandatory three-way review before frontend edits
- [x] Phase 3 - implement the workspace-level event source and drawer bridge
- [x] Phase 4 - run static/design gates and authenticated cross-route responsive validation

### Current Phase
Complete - `DecisionEventCenter.svelte` now remains mounted at workspace level, loads automatic flow events plus persisted override-audit events, publishes one shared latest summary, polls every 15 seconds, and owns the full-height drawer. `App.svelte` passes that summary to every workspace, including Settings, and no longer relies on the Decision Dashboard component for notification or drawer availability. Authenticated checks confirmed the same latest `MDL-1375` event on Decision, Daily Jira, Schedule, Evidence, Tasks, KPI, and Settings. Opening it from Settings preserved the current route, showed 48 events newest-first, filled the viewport, and closed by button, backdrop, or Escape with focus return. 1024/760/520 checks found zero document horizontal overflow; the 520px notification retained a 44px touch target, and Daily Jira retained zero document vertical overflow, hidden frame overflow, and a 16px bottom gap. `pnpm check` passes with the unchanged 72-warning baseline, production build passes, both post-edit design detectors return no findings for the shared notification and event center, and `git diff --check` is clean.

## Current Task Addendum: Daily Jira Single-Viewport Fixed Inspector

### Goal
Remove the right-edge workspace/browser scrollbar from Daily Jira, remove every vertical scrollbar from the Jira inspector, and keep every inspector block on a stable track so short and long Jira titles cannot move the fact, decision, form, or history sections.

### Mandatory Three-Way UI Review
- [x] Impeccable: the shell must explicitly mark Daily Jira as a viewport-fitted route; the page root then consumes only the remaining frame height. The Jira table remains the sole vertical scroll owner. The inspector, form, and history tracks use `overflow: hidden`, fixed grid ownership, line clamps, and ellipsis instead of nested scroll containers.
- [x] design-taste-frontend: this is dense product UI outside the skill's primary marketing scope, so use preservation mode only. Keep the current navigation, light teal visual language, data density, table selection, inputs, API behavior, and component vocabulary. Fix height and overflow ownership without introducing a new card system or decorative treatment.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`. On wide screens retain five vertically aligned inspector tracks. Below the workbench breakpoint, use the inspector's horizontal width to arrange the same five blocks in a bounded two-dimensional grid instead of creating another scroll region.
- Shared hierarchy and ownership: `FunctionalAdminShell.svelte` owns route-level frame scrolling and disables it only for Daily Jira and the existing schedule board. `DailyJiraAudit.svelte` owns the viewport height, table scrolling, fixed inspector tracks, title clamping, and responsive inspector grid. The table is the only local vertical scroll owner; browser, workspace frame, inspector, decision form, and history never scroll vertically.
- Responsive behavior: desktop uses five fixed vertical tracks sized to the available inspector height. Tablet uses a three-column, two-row inspector grid; narrow mobile uses a two-column, three-row grid and gives more of the bounded workbench height to the inspector. Optional history depth is clipped within its stable preview track while the total record count stays visible.
- Validation scope: authenticated Daily Jira at the current desktop viewport plus 1180/1024/760/520/480 checks; zero document and workspace-frame vertical overflow; zero inspector/form/history vertical overflow; left table remains scrollable; long/short Jira title switching preserves every inspector block top and height; feedback state still fits; pointer and keyboard row selection remain intact; Svelte check/build, Impeccable detection, and browser console review.
- Disagreement and resolution: showing an unlimited decision history inside a fixed-height inspector conflicts with the explicit no-scroll requirement. Keep the inspector as a latest-history preview with its total count visible; do not reintroduce a scrollbar. Full history remains a separate progressive-disclosure concern rather than a hidden nested scroll owner.

### Root Cause
- The shell forces `.workspace-frame { overflow-y: scroll; }` for Daily Jira, so a browser-edge rail is always rendered and the workspace's minimum height adds 56px of overflow even though the Jira surface already measures itself to the viewport.
- The inspector's fixed rows total 782px before its 32px padding, but the live panel is only 727px high. `overflow: auto` on the inspector plus `overflow-y: auto` on both the decision form and history creates three nested vertical scroll owners.
- The Daily Jira root always declares two grid rows, and the feedback variant declares three, despite rendering only one or two children. These phantom tracks and gaps waste bounded height.

### Phases
- [x] Phase 1 - inspect live geometry and complete the mandatory three-way review
- [x] Phase 2 - implement route-level viewport fitting and fixed inspector tracks
- [x] Phase 3 - run static, detector, authenticated, and responsive validation

### Current Phase
Complete - Daily Jira now marks its shell frame as viewport-fitted, uses one real grid row without feedback and two real rows with feedback, and keeps the table as the only vertical scroll owner. The inspector uses five stable desktop tracks, a three-column tablet grid, and a two-column mobile grid; the inspector, decision form, and history all report hidden or visible overflow without scrollable excess. Authenticated validation at desktop plus exact 1180/1024/760/520/480 viewports showed zero document overflow, zero inspector/form/history scroll delta, and a still-scrollable Jira table. Switching between `JTG-9848` and the much longer `ICA-10648` produced zero top/height delta for all five inspector blocks. The reassign state also fits with two selects at 480px. `pnpm check` has 0 errors and the unchanged 72-warning baseline, production build passes, Impeccable reports no findings, Finesse reports none in Daily Jira, and the current browser console is clean.

## Current Task Addendum: Inline Event Notification Styling

### Goal
Remove the nested rectangular-card appearance from the latest decision-event notification while keeping the breadcrumb strip, full-row activation, event semantics, drawer behavior, and responsive layout intact.

### Mandatory Three-Way UI Review
- [x] Impeccable: present the latest event as one inline notification rail inside the existing breadcrumb surface. Remove the child border, fill, and inset shadow; keep a semantic status dot, truncation, a visible keyboard focus treatment, and the existing full-row button behavior.
- [x] design-taste-frontend: this remains dense product UI, so preserve the Phase 41 typography, teal/blue semantic colors, compact height, route context, and drawer interaction. Use direct typography and spacing rather than another card or pill system.
- [x] finesse-ui: use `dot / event kind / Jira / title / time / detail arrow` as a lightweight centered ticker. Let hover change ink and nudge the arrow only; do not introduce a container outline, rounded rectangle, gradient, or decorative motion.
- Shared hierarchy and ownership: `FunctionalWorkspace.svelte` continues to own the breadcrumb notification presentation; `App.svelte` and `DecisionDashboard.svelte` data and drawer contracts remain unchanged.
- Responsive behavior: wide and tablet layouts keep the centered notification schema; narrow layouts retain a 44px transparent touch target and hide secondary label/kind/time fields without adding a box.
- Validation scope: no child border/background/box-shadow in default or hover states; automatic/manual semantic dot colors; whole-row pointer and keyboard activation; title truncation; desktop/1024/760/480 horizontal containment; drawer open/close unchanged; Svelte check/build and Impeccable detection.
- Disagreement and resolution: removing all container chrome reduces conventional button affordance. Preserve clickability with pointer cursor, a colored detail label and arrow, hover ink change, and a two-line focus underline instead of restoring a rectangle.

### Phases
- [x] Phase 1 - inspect and complete the mandatory three-way review
- [x] Phase 2 - implement the inline notification style
- [x] Phase 3 - run static, detector, and responsive interaction validation

### Current Phase
Complete - the breadcrumb notification is now one transparent inline rail with no child border, fill, radius, or shadow. Automatic and manual events retain distinct semantic dots, the real latest-event row opens the existing full-height drawer, and pointer/focus affordances remain visible without restoring a box. Authenticated desktop validation plus isolated 1024/760/480 checks confirmed centered geometry, 44px narrow touch height, truncation, zero horizontal overflow, and clean current-page console output. `pnpm check`, production build, diff check, and post-edit Impeccable detection all pass; the 72 Svelte warnings remain the pre-existing baseline in five unrelated files.

## Current Task Addendum: Daily Jira Stable Inspector And Decision Event Drawer

### Goal
Reduce Daily Jira timing state to a visible color-only mark, stabilize every right-inspector block against variable Jira title length, restore outer workspace scrolling while the pointer is over the Decision Dashboard inspector, and move the decision-event ledger into a centered latest-event breadcrumb summary plus an on-demand full-height reverse-chronological drawer.

### Design Read
Internal delivery-governance product UI for morning triage and decision traceability. Preserve the Phase 41 light, table-first console. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; targeted preservation redesign with feedback-only motion.

### Mandatory Three-Way UI Review
- [x] Impeccable: keep the visible Jira timing cell to one semantic color dot, but retain a screen-reader-only state label; lock inspector header/fact/decision/form/history tracks so changing Jira titles cannot move downstream content; make the workspace frame the Decision Dashboard scroll owner; use a keyboard-operable breadcrumb summary and an accessible full-height drawer with focus return, Escape close, backdrop close, and one timeline scroll owner.
- [x] design-taste-frontend: this is dense dashboard/product UI outside the skill's primary marketing scope, so apply preservation rules only. Keep routes, labels, APIs, data adapters, teal accent, type family, table density, selection behavior, and existing control components; do not introduce Atlaskit, a new card system, or decorative motion.
- [x] finesse-ui: use one fixed-schema latest-event strip in the center of the breadcrumb bar (`最新事件 / 类型 / Jira / 标题 / 时间 / 查看详情`), then progressively disclose the complete visible ledger in a right-edge viewport drawer sorted newest first. Remove the ledger from the inspector and release its nested overflow so wheel input chains directly to the workspace.
- Shared hierarchy and ownership: `DailyJiraAudit.svelte` owns the color-only timing dot and stable inspector tracks. `DecisionDashboard.svelte` owns event ordering, latest-event publication, drawer state, focus behavior, and the complete timeline. `App.svelte` bridges the latest summary and drawer-open request. `FunctionalWorkspace.svelte` owns the breadcrumb-center presentation. Backend APIs, visibility filtering, decision mutation, and 15-second refresh remain unchanged.
- Responsive behavior: the breadcrumb is a three-zone row on wide screens and places the event summary on its own full-width row below 980px. The event drawer is a bounded right panel on desktop and edge-to-edge on narrow screens. Daily Jira uses explicit stable inspector tracks on desktop with breakpoint-specific taller facts/form tracks where the inspector becomes narrow; the inspector remains its single local scroll owner.
- Validation scope: color-only overdue/due-soon/healthy cells with no visible timing text; long/short Jira title switching with identical downstream block Y coordinates; Decision Dashboard wheel chaining over the inspector; latest-event refresh projection; drawer backdrop, close button, Escape, focus return, newest-first ordering, empty state, Jira/commit links; 1440/1024/760/480 geometry and zero document horizontal overflow; Svelte check/build, Impeccable/Finesse detection, and browser console review without live mutations.
- Disagreements and resolution: color alone is normally insufficient status communication, but the user explicitly requested no visible state/age copy. Honor the visible contract and preserve the status through `role="img"` plus `aria-label`, without a hover tooltip. Absolute-positioning a child summary over the breadcrumb would be brittle; use explicit parent/child data ownership instead. Retaining selected/all tabs in the drawer would conflict with “所有事件记录”; the drawer always shows all currently visible events in descending order.

### Phases
- [x] Phase 1 - inspect timing, inspector, breadcrumb, event, drawer, and scroll ownership contracts
- [x] Phase 2 - complete and record the mandatory three-way review before frontend edits
- [x] Phase 3 - implement color-only timing, stable Jira inspector tracks, breadcrumb event summary, drawer, and scroll ownership fix
- [x] Phase 4 - run static/design checks and authenticated/isolated browser validation across states and breakpoints
- [x] Phase 5 - record evidence and deliver

### Current Phase
Complete - overdue/due-soon/healthy cells render red/amber/green 11px dots with empty visible cell text and screen-reader labels. Switching between the long and short Jira fixtures produced zero top/height deltas across all five inspector tracks. Wheel input over the Decision Dashboard inspector advanced the workspace frame while both former inspector scroll positions remained zero. The authenticated live page exposed the newest event in the breadcrumb; isolated live-component validation proved the click-open drawer is fixed to the full viewport, contains every visible event newest-first, closes by backdrop/button/Escape, returns focus, stays closed across navigation remounts, and has no console or horizontal-overflow errors at desktop/1024/760/480 widths. `pnpm check` reports 0 errors, production build passes, and the post-edit Impeccable scan reports no new findings in the three affected components.

## Current Task Addendum: Daily Jira Timing Badge Clarification

### Goal
Replace the ambiguous first-column age/overdue stack with one compact, single-line timing badge that separates creation age from deadline health and uses the requested red/yellow/green status vocabulary.

### Mandatory Three-Way UI Review
- [x] Impeccable: rename the column from “年龄” to “时效”, keep the creation age as compact context, and pair every color with a visible status word. Use one inline badge rather than a colored age pill plus a second wrapped overdue label.
- [x] design-taste-frontend: apply preservation rules only. Keep the existing table density, widths, typography, selection behavior, and semantic tokens; do not redesign the workbench or add new decoration.
- [x] finesse-ui: use one scan-friendly status unit with a 7px semantic dot, short label, and tabular age. Prevent wrapping at every breakpoint and reserve red for overdue, amber for due within three natural days, and green for healthy.
- Shared direction: `DailyJiraAudit.svelte` owns display-only timing classification using the backend's generated timestamp, due date, and overdue fact. The backend age cohorts and persisted data remain unchanged. The badge reads `圆点 状态 · 创建时长`, exposes the full deadline meaning through `title`, and stays on one line.
- Responsive behavior: widen the desktop timing column enough for the inline badge, then use deliberate 112px/104px timing widths at 760/520 while preserving the existing hidden secondary columns and zero table/document overflow.
- Validation scope: overdue, due-today/three-day, healthy/no-deadline states; today and multi-day age copy; no wrapped timing badges; 1280/760/480 table geometry; keyboard/row selection unchanged; Svelte check/build, Impeccable/Finesse detection, and browser console review.
- Disagreement and resolution: treating the three age cohorts themselves as red/yellow/green would conflate creation age with deadline health. Use explicit `due_date` health for color/status and retain `age_days` only as factual “创建多久” context.

### Phases
- [x] Phase 1 - inspect and agree on timing semantics and responsive ownership
- [x] Phase 2 - implement the single-line timing badge
- [x] Phase 3 - validate semantic states, breakpoints, and static/design gates

### Current Phase
Complete - the first column now renders one non-wrapping deadline-health badge with factual creation age. Red overdue, amber due within three natural days, and green healthy states were verified at 1280/760/480 with zero document overflow, the existing 16px bottom gap, keyboard row selection, clean console output, passing check/build, and clean Impeccable/Finesse detection.

## Current Task Addendum: Daily Jira Decision Reminders And Viewport Fit

### Goal
Move all Jira audit filters into the table-owned header, make every populated row the selection target for the inspector, persist an operational decision record alongside the immutable audit event, generate status-based overdue reminders, and keep the entire page inside the browser viewport with a deliberate bottom gap.

### Mandatory Three-Way UI Review
- [x] Impeccable: the table owns cohort/search/refresh controls; a populated grid row owns pointer and keyboard selection with visible focus; the inspector owns decision state and history; the route root fits the remaining viewport while the table and inspector are the only vertical scroll owners.
- [x] design-taste-frontend: this dense admin workflow remains outside the skill's marketing-page focus, so apply preservation rules only. Keep the current light workbench, teal focus/selection, route hierarchy, type/radius vocabulary, and feedback-only motion; do not add cards, a new design system, or ornamental state graphics.
- [x] finesse-ui: product register, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`. Collapse the detached cohort cards into one compact table command header, expose the latest decision/reminder as operational status rather than decoration, and use one bounded viewport workspace with internal pane scrolling.
- Shared hierarchy: compact route header -> optional feedback -> viewport-fitted table/inspector workbench. Inside the table panel: age scope -> search/refresh -> result metadata -> sticky columns -> whole-row selection. Inside the inspector: Jira facts -> latest decision/reminder -> decision form -> immutable decision history.
- Component ownership: `DailyJiraAudit.svelte` owns table filters, selection, pane geometry, decision/reminder presentation, and local feedback; the shell continues to own navigation and the browser viewport; `daily_jira_handlers.go` owns validation, status-to-reminder policy, transactionality, and response projection; `db.DailyJiraDecision` stores operational decision/reminder facts; `DecisionEvent` remains the audit ledger; the existing notification SSE owns delivery and per-user dismissal.
- Responsive behavior: desktop retains aligned left table/right inspector panes. At the existing workbench breakpoint the panes become two bounded rows inside the same remaining viewport; neither pane expands the document. At narrow widths secondary columns collapse, controls reach touch size, and both panes keep their own internal scrolling. The route leaves 16px beneath the workbench at every measured breakpoint.
- Validation scope: latest-decision projection, reminder policy and supersession, resolved-Jira suppression, notification dismissal key, row pointer/keyboard selection, filter placement, populated/empty/error/success/read-only source contracts, wide/1024/760/480 geometry, pane scroll owners, zero browser-level vertical/horizontal overflow, static checks/build, Impeccable/Finesse detection, and authenticated browser inspection without submitting a live decision.
- Disagreements and resolution: a mobile drawer would maximize inspector height but would change the established inline review workflow and add overlay ownership. Keep the inspector inline and split the available height between two internally scrollable panes. A configurable reminder time would add meeting friction and an additional validation contract; use explicit status defaults instead: escalation after 4 hours, follow-up and reassignment after 24 hours. A new decision supersedes the prior reminder, and a resolved Jira no longer emits it.

### Phases
- [x] Phase 1 - inspect the current UI, decision-event persistence, database models, and notification delivery
- [x] Phase 2 - complete and record the mandatory three-way review before frontend edits
- [x] Phase 3 - implement decision records, reminders, table-owned controls, row activation, and viewport fit
- [x] Phase 4 - run focused backend tests, frontend checks/build, detectors, and authenticated breakpoint validation
- [x] Phase 5 - record evidence and deliver

### Current Phase
Complete - cohort/search/refresh controls are table-owned; every populated row is pointer- and keyboard-selectable; the inspector exposes the latest persistent decision and its review deadline; notification SSE emits only the latest unresolved due decision and honors per-user dismissal. Wide/1024/760/480 validation measured a fixed 16px bottom gap, zero document-level horizontal or vertical overflow, intentional table/inspector internal scroll ownership, 44px narrow controls, and an empty console error list without submitting a decision.

## Current Task Addendum: Daily Jira Visual Unification

### Goal
Bring the existing `每日 Jira` audit surface back into the Phase 41 light admin-console system without changing its data, permissions, age cohorts, decision actions, or retrospective history.

### Design Read
Internal morning-governance product UI for operations staff. Preserve the restrained teal accent and dense table-first workflow. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; targeted preservation redesign rather than a new visual system.

### Protection Rules
- Keep the current `今日 / 3 日 / 7 日` cohort semantics, search, selection, Jira links, decision form, permission boundary, and history behavior unchanged.
- Preserve the global shell, navigation labels, type family, semantic color tokens, and the existing decision/agenda page.
- Limit frontend implementation to `DailyJiraAudit.svelte` unless validation proves a shared-shell defect.
- Use one scroll owner for the table and one for the inspector on wide screens; release fixed-height ownership when the layout stacks.
- Do not add a hero, decorative KPI theater, a second accent, or nested cards.

### Mandatory Three-Way UI Review
- [x] Impeccable: the existing structure is correct, but the visual hierarchy is too flat and the responsive stack is too tall. Use a compact command surface, three independently scannable cohort controls, one aligned table/inspector workbench, 4px-grid spacing, and natural-height stacked panels. Keep focus, loading, empty, error, success, read-only, and reduced-motion behavior.
- [x] design-taste-frontend: apply only its redesign audit and anti-slop rules because it declares dense dashboards out of scope. Preserve-mode dials are `VARIANCE=3`, `MOTION=2`, `DENSITY=9`; retain the repository component system rather than introduce Atlaskit or a new design library.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`. Align radii, translucent borders, control vocabulary, numeric typography, and table/inspector material with `DecisionDashboard.svelte`; omit its brand-register grain, giant typography, and spectacle substrate.
- Shared direction: keep shell/App/component ownership unchanged. Inside `DailyJiraAudit.svelte`, convert the loose title/actions into one low command bar, render the three cohorts as separate 16px-radius status controls using the existing semantic colors, give the table and inspector the same 20px outer workbench radius/material, increase the inspector's usable desktop width, and keep row density at 48px. Desktop remains two-column; medium/narrow layouts stack, but the table height follows visible content within a capped scroll boundary so the inspector begins immediately after the list rather than after an artificial blank block.
- Disagreements and resolution: Finesse's generic premium substrate asks for grain and stronger layered decoration, while the project contract explicitly reserves restrained matte glass for structural boundaries; the Phase 41 contract wins. The current `DecisionDashboard` uses more metric cards than this workflow needs, so visual unity comes from tokens, radii, material, toolbar, status controls, and aligned workbench geometry rather than copying all six metrics. Impeccable's layout command asks for isolated sub-agent assessments, but delegation is unavailable under the active collaboration policy; the required subjective assessment ran first in the primary context, followed by the detector pre-scan, and this fallback is recorded.

### Current Assessment
- Spacing: the 12px base is consistent, but nearly every group uses the same gap, so the page lacks tight-versus-generous rhythm.
- Hierarchy: the floating header, flush three-way tab strip, table toolbar, and panels all carry similar visual weight; the first action path is not obvious under a squint test.
- Structure: the table/inspector topology is correct and avoids repeated boards, but the 1180px breakpoint drops to a fixed 560px table and pushes the inspector down even when only a few rows are present.
- Density: 48px rows fit the operational use case. The desktop inspector is usable but visually flatter than the adjacent decision workbench; narrow screens waste vertical space and can hide the decision form below the fold.
- Mechanical pre-scan: `detect.mjs --scope layout` returns `[]`; no arbitrary Tailwind spacing or z-index utilities exist. The issues are visual/structural rather than detector-recognizable violations.

### Phases
- [x] Phase 1 - load the three required UI skills and project validation contract
- [x] Phase 2 - inspect the authenticated desktop, medium, and narrow states and complete the review before editing
- [x] Phase 3 - implement the scoped visual and responsive layout unification
- [x] Phase 4 - run Impeccable/finesse detection, type/build/diff checks, and authenticated desktop/1024/760 validation
- [x] Phase 5 - record evidence and deliver

### Current Phase
Complete - Daily Jira now uses the Phase 41 command/status/workbench vocabulary; wide table and inspector align, stacked table height follows visible rows, CSS 760/480 layouts remove secondary columns without document overflow, search-detail synchronization remains intact, and all static/design/authenticated browser gates pass.

### Validation Scope
- Populated wide state with real Jira data: command bar, cohort selection, selected row, table/inspector top and bottom geometry, and inspector scroll ownership.
- Medium and narrow authenticated states: no horizontal document overflow, explicit cohort/control reflow, bounded table scrolling, and inspector starts directly below the table.
- Search-to-selection synchronization, Jira deep link visibility, read-only/write-authorized form rendering, and no decision submission during validation.
- Loading/empty/error/success source contracts remain intact; focus and reduced-motion rules stay present.

## Current Task Addendum: Daily Jira Morning Review

### Goal
Add a `每日 Jira` submenu under the decision area that audits unresolved Jira items from today, at least 3 days old, and at least 7 days old, then supports rapid morning-meeting assignment and later decision traceability.

### Protection Rules
- Preserve the established light, table-first admin-console baseline and the existing decision/agenda workflows.
- Treat the three age windows as operational audit scopes, not decorative KPI cards.
- Reuse current Jira data, assignee, project, status, permission, and navigation contracts where they already exist; do not invent parallel ownership semantics.
- Work narrowly in the existing dirty tree and do not revert unrelated user changes.
- Do not edit frontend code until the mandatory three-way review records agreement on hierarchy, ownership, responsive behavior, accessibility, and validation.

### Mandatory Three-Way UI Review
- [x] Impeccable: place the submenu in the global rail; use one age-scope strip, a semantic row-selectable table, and one inline inspector; retain keyboard/focus semantics and validate loading, empty, populated, error, read-only, and successful mutation states.
- [x] design-taste-frontend: treat this as a preservation redesign with high density and low motion; keep the existing Phase 41 tokens/component vocabulary, avoid metric-card theater, and do not migrate the Svelte app to Atlaskit solely because the data originates in Jira.
- [x] finesse-ui: `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; optimize the first viewport for count scanning, row selection, rapid assignment, explicit feedback, and an audit ledger rather than visual spectacle.
- Shared direction: the shell owns `决策事项 / 每日 Jira` navigation; `App.svelte` owns the active decision view and breadcrumbs; a new `DailyJiraAudit.svelte` owns fetch/filter/selection/action UI; the backend owns stable Jira-key filtering, non-overlapping natural-day buckets, visibility, write authorization, Jira reassignment sync, and decision-event persistence. The surface is one flat workbench: compact scope buttons with counts, dense table, inline inspector, one scroll owner per pane, and responsive stacking below the desktop workbench breakpoint.
- Disagreements and resolution: design-taste's generic Jira-product mapping suggests Atlaskit, while the project contract forbids a second component system; preserve the committed Svelte admin system. A three-column age board would expose all buckets simultaneously but repeat the same row UI and reduce morning scan density; use one visible count strip with a single active table. Impeccable favors a generic reusable inspector primitive, but this workflow's decision/event ownership is specific; keep it in the new feature component and reuse only shared controls.

### Design Read
Internal morning-governance product UI with a restrained, precise, traceable language. Preserve the light table-first console and teal semantic accent. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; targeted preservation feature.

### Phases
- [x] Phase 1 - trace decision navigation, Jira data/actions, backend contracts, and authenticated route state
- [x] Phase 2 - complete and record the three-way review before frontend edits
- [x] Phase 3 - implement the smallest complete backend/frontend slice
- [x] Phase 4 - run focused tests, Impeccable detection, and authenticated browser validation across affected states and breakpoints
- [x] Phase 5 - compress evidence and deliver

### Current Phase
Complete - Daily Jira navigation, age cohorts, morning decisions, visibility boundaries, retrospective history, responsive UI, static gates, complete server tests, and authenticated isolated browser validation pass.

### Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Discovery command named nonexistent `internal/server/task_handlers.go` | 1 | Use `rg --files internal/server` and inspect the actual task/execution handler paths before the next targeted read. |
| First detector sweep exited non-zero on legacy `App.svelte` side stripes, gradient text, and missing reduced-motion fallback; finesse also found four pure-white fallbacks in the new component | 1 | Preserve unrelated legacy styling in the dirty App, replace all new pure-white fallbacks with the existing cool off-white surface, and rerun both detectors narrowly on the new component while recording legacy findings as baseline debt. |
| Authenticated search filtered the table to `INFRA-881` while the inspector still showed filtered-out `TOS-318` | 1 | Add reactive visible-selection reconciliation; revalidated that the filtered row and inspector now match. |
| The complete server suite could not open its existing loopback `httptest` listener inside the sandbox | 1 | Reran the exact full package test with managed loopback permission; the suite passed. |

### Validation Scope
- Data correctness for unresolved Jira membership in today / 3-day / 7-day audit scopes, including boundary dates and duplicate avoidance.
- Fast assignment/decision actions, permission/error behavior, Jira deep-linking, refresh state, and decision-history visibility.
- Authenticated desktop, 1024px, and narrow/mobile states; loading, empty, populated, error, and action feedback.
- Targeted backend/frontend tests, Svelte check/build, diff hygiene, Impeccable detector, browser console and geometry review.

## Current Task Addendum: Surface-Aware Text Contrast

### Goal
Make foreground color follow surface luminance on the demand-detail workflow: dark surfaces use cool light text and muted blue-gray secondary text, while the white drawer keeps the established dark ink scale.

### Mandatory Three-Way UI Review
- [x] Impeccable: bind foregrounds to semantic surface ownership rather than applying a global text override; preserve focus, disabled, helper, and status contrast.
- [x] design-taste-frontend: keep the existing light drawer and teal accent; on any retained dark modal fallback, use a restrained cool off-white/blue-gray ink pair instead of black or pure white.
- [x] finesse-ui: enforce one surface and one compatible ink scale; validate rendered dark-background text rather than inferring contrast from token names.
- Shared direction: retain the light full-height drawer. Correct the legacy dark modal base so unscoped/inherited text cannot become black, then explicitly keep the Phase 41 light modal/drawer on the dark ink tokens. Do not turn this into a global dark-theme rewrite.
- Disagreement and resolution: a global theme-aware text system would be more comprehensive, but it would exceed the reported surface and risk unrelated legacy panels. Resolve with a scoped modal contrast contract plus authenticated rendered-state scanning.

### Phases
- [x] Phase 1 - record the surface/foreground contrast contract
- [x] Phase 2 - inspect rendered affected states for dark-background/dark-text collisions
- [x] Phase 3 - implement the smallest semantic foreground correction
- [x] Phase 4 - run static, detector, and authenticated visual/contrast validation

### Current Phase
Complete - filled accent controls use the dedicated dark teal/cool-white contrast contract; rendered wide/mobile states, static gates, reduced-motion fallback, and browser console review pass.

## Current Task Addendum: Viewport-Height Demand Drawer

### Goal
Correct the flow-board demand drawer so it is owned by the application viewport rather than the board workspace: it must open flush against the right edge, span the full visible height, dim and block the complete shell, and preserve the linked AI companion and existing detail interactions.

### Design Read
Targeted preservation refinement for the existing light delivery-governance console. Preserve the teal accent, compact type scale, flat facts, and single-scroll-body detail hierarchy. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`.

### Protection Rules
- Work narrowly in the dirty tree and preserve all unrelated changes.
- Keep demand selection, close/Escape/Tab/focus-return behavior, AI actions, scheduling, Markdown editing, and linked AI-companion lifecycle intact.
- The viewport overlay owns the full-screen modal layer; the drawer body remains the only long-content scroll owner.
- Do not introduce another card language, decorative treatment, or new component-system migration.

### Mandatory Three-Way UI Review
- [x] Impeccable: promote overlay ownership to a viewport-fixed modal layer, retain semantic dialog/focus behavior, and validate exact geometry plus shell hit blocking.
- [x] design-taste-frontend: preserve the existing visual language and remove the residual floating-card cue; use a square, structural full-height edge instead of rounded modal corners.
- [x] finesse-ui: treat this as a product drawer rather than a page panel; use `100dvh`, one left hairline/shadow, one scroll body, and keep the AI companion on the same fixed overlay layer.
- Shared direction: `DemandKanban.svelte` continues to own the demand and AI-companion lifecycle, but both detail overlay hosts become viewport-fixed. The demand drawer is flush right with top, bottom, and right edges at the viewport boundary; the global backdrop covers the top bar, sidebar, and board; narrow screens use the same full-height edge-to-edge contract.
- Disagreement and resolution: retaining the previous left-side corner radius would soften the panel, but it would continue to read as a floating modal. Resolve in favor of square corners and a restrained left boundary so the surface reads as application structure. Authenticated geometry proved the shell's isolated workspace stacking context still paints the top bar above a fixed descendant, so the pre-agreed fallback is active: portal only the two detail overlay roots to `.functional-console`, leaving the global shell isolation unchanged.

### Phases
- [x] Phase 1 - inspect the supplied regression screenshot and current overlay ownership
- [x] Phase 2 - complete and record the mandatory three-way review before frontend edits
- [x] Phase 3 - implement viewport-fixed full-height drawer and companion geometry
- [x] Phase 4 - run detector/static/build checks and authenticated browser validation at wide and narrow breakpoints
- [x] Phase 5 - record final evidence and deliver the scoped result

### Current Phase
Complete - the drawer and detail companion are application-level fixed overlays; exact wide/mobile geometry, global hit blocking, linked interactions, static gates, and visual review pass.

### Validation Scope
- Wide and narrow authenticated flow-board states: overlay and drawer `top=0`, `bottom=innerHeight`, `height=innerHeight`, `right=innerWidth`, with zero document horizontal overflow.
- Global shell is visibly dimmed and cannot receive pointer input while open; drawer header remains fixed and the body is the only long-content scroll owner.
- Backdrop close, close button, Escape, Tab containment, and focus return remain intact.
- Wide AI companion is separated from the drawer without overlap; closing the host closes the companion.
- Run Impeccable/finesse detection, Svelte check, production build, diff hygiene, and browser console review.

## Current Task Addendum: Flow Detail Drawer And Unified Task Filters

### Goal
Replace the visually detached flow-board demand modal with a workspace-aligned drawer, then unify task-panel filters on the shared dropdown vocabulary while aligning the task table and detail inspector as one workbench.

### Design Read
Internal delivery-governance product UI for daily operators, with a restrained, precise, table-first light-console language. Preserve the existing teal accent and compact type scale. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; targeted preservation redesign.

### Protection Rules
- Work incrementally on the dirty tree without reverting or rewriting unrelated user changes.
- Preserve demand selection, demand detail actions, AI companion behavior, task selection, Jira navigation, current filter semantics, evidence actions, and authenticated permissions.
- Keep the established light console, semantic color tokens, type family, route/navigation labels, and shared Select interaction contract.
- Do not add decorative glass, new accent colors, nested cards, or a second scrolling owner inside either workbench.

### Mandatory Three-Way UI Review
- [x] Impeccable: hierarchy, component ownership, responsive behavior, accessibility, validation scope
- [x] design-taste-frontend: preservation audit, density, shape/color consistency, anti-slop preflight
- [x] finesse-ui: product-register drawer, filter, table/inspector system, interaction states
- Shared direction: `DemandKanban.svelte` keeps ownership of selection, scroll lock, close behavior, and AI-companion linkage while the demand-detail host becomes a right-edge off-canvas dialog with a fixed header and one scrollable body. `TaskKanban.svelte` keeps filter/data ownership, shared `Select.svelte` owns every categorical filter including a non-searchable risk dropdown, and text search remains the only text input. The execution table and inspector share one desktop height/top baseline with independent content overflow, then release the equal-height contract when the workbench stacks. Preserve the Phase 41 light shell, existing teal accent, 4px spacing grid, and established radius/control tokens.
- Disagreements and resolution: Impeccable preferred a reusable generic Drawer primitive, while finesse-ui favored the smallest state-safe product change and design-taste-frontend warned against expanding a dense admin change into a new component-system migration. Because the repository has only a telemetry-specific drawer and demand detail coordinates a unique AI companion, this pass keeps the shell in `DemandKanban.svelte` but follows the proven off-canvas behavior. Impeccable also favored labeled controls; the toolbar cannot afford persistent labels at current density, so accessible names plus explicit selected values are retained. Risk uses a non-searchable Select because six stable options do not justify search.

### Phases
- [x] Phase 1 - inspect screenshots, active demand/task components, shared primitives, tokens, and current geometry
- [x] Phase 2 - complete and record the mandatory three-way review before frontend edits
- [x] Phase 3 - implement the demand drawer and unified aligned task workbench
- [x] Phase 4 - run targeted detector/static/build checks and authenticated browser validation for affected states and breakpoints
- [x] Phase 5 - compress evidence and deliver the scoped change summary

### Current Phase
Complete - demand detail is a responsive right drawer with a stable companion state, execution risk uses shared Select, desktop table/inspector geometry aligns exactly, responsive stacking is deliberate, and all static/browser gates pass.

### Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Process listing was blocked by the local sandbox | 1 | Reused `lsof` and browser health checks to identify the already-running Vite/API stack; did not repeat the blocked `ps` path |
| Chrome's current session API does not expose `tabs.claim`, and `tabs.get` only sees agent-session tabs | 2 | Read the live API signature, opened a new Chrome-session tab, and reused the browser's signed-in localhost session without inspecting credentials or storage |
| The current Tab Playwright facade does not expose `reload` | 1 | Used the documented top-level `Tab.reload()` method |
| Locator-scoped geometry evaluation timed out for the companion although the node was present | 2 | Used the documented page-level read-only evaluation surface and verified the exact panel/drawer rectangles there |

## Current Task Addendum: Event Timeline, Flow Detail, And Execution Filters

### Goal
Remove the unintended scrollbar from event mediation records, repair the schedule flow-board detail panel in its real authenticated state, and consolidate duplicated project/owner filters into one single-line execution-tracking filter bar whose dropdowns are never clipped.

### Design Read
Operational delivery-governance admin UI for internal managers, with a calm, precise, table-first product language. Preserve the established light console and teal accent. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; targeted redesign, not a visual rewrite.

### Protection Rules
- Work on the dirty tree without reverting or rewriting unrelated user changes.
- Preserve task selection, Jira navigation, project and owner filtering semantics, demand detail actions, AI workbench behavior, and authenticated route permissions.
- The shell/page owns scrolling; event records and flow-board lanes/details must not introduce competing scrollbars unless content genuinely cannot fit at the validated breakpoint.
- Consolidated execution filters must remain one desktop row, use one owner control instead of duplicate owner/business-law controls, and collapse deliberately at narrower breakpoints without clipping menus.

### Mandatory Three-Way UI Review
- [x] Impeccable: hierarchy, component ownership, responsive behavior, accessibility, validation scope
- [x] design-taste-frontend: targeted-evolution audit, density, shape/color consistency, anti-slop preflight
- [x] finesse-ui: product-register component/system review and redesign audit
- Shared direction: keep one scroll owner per surface; let the decision inspector own event-record overflow, keep demand-detail header/actions stable while the long body remains operable, and make execution tracking own one project filter plus one canonical owner filter by reusing the shared Select primitive. The execution toolbar sits above its table stacking context, stays one row at the supplied wide-desktop state, and reflows deliberately below that width.
- Disagreements and resolution: Impeccable and design-taste preferred a broader demand-detail progressive-disclosure/shared-modal migration, while finesse-ui warned that removing all long-content scrolling would make the specification unusable. This targeted pass keeps the existing detail component and its single body scroll owner, removes only the visible scrollbar rail, and repairs usable viewport/topbar geometry; the broader information-architecture migration is deferred because it exceeds the reported regression.

### Phases
- [x] Phase 1 - inspect screenshots, active components, current CSS/overflow/filter state, and prior fixes
- [x] Phase 2 - complete and record the mandatory three-way review before frontend edits
- [x] Phase 3 - implement event-record, flow-detail, and consolidated execution-filter fixes
- [x] Phase 4 - run targeted detector/static/build checks and authenticated browser validation at affected states and breakpoints
- [x] Phase 5 - compress final evidence and deliver the scoped change summary

### Current Phase
Complete - mediation records expand under the inspector's single scroll owner, the flow detail and companion remain inside the usable workspace at wide and narrow breakpoints, and execution tracking owns one project plus one owner filter with an unclipped shared Select menu.

### Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| `rg` parsed a CSS token pattern beginning with `--z` as an option | 1 | Use `rg -- '<pattern>'` for leading-hyphen patterns and log the reusable tool gotcha in `.learnings/ERRORS.md` |
| Source inspection assumed stale `web/src/lib/components` paths | 1 | Resolve current component paths with `rg --files` before reading |
| Final browser geometry check used a stale toolbar selector | 1 | Resolve the active class from source and null-check the target before measurement |


## Current Task Addendum: Demand Detail AI Workbench And Executable Spec Draft

### Goal
Polish the demand-detail delivery controls, make draft creation reversible, unify demand-detail and schedule AI deconstruction into the centered two-panel workbench, and deepen AI output into an implementation-ready specification.

### Protection Rules
- Preserve the existing light admin-console baseline and all current demand, schedule, deconstruction, review, freeze, and execution permissions.
- Draft withdrawal is allowed only for an unfrozen draft and must remove its draft review contract atomically; frozen specifications and execution evidence remain immutable.
- Closing a host demand-detail or schedule panel must also close its AI companion; a companion may never remain orphaned.
- Keep the existing safe change-set boundary: AI may only create files or update explicitly supplied source files, and CI/human review remain mandatory.
- Work incrementally on the dirty tree without reverting unrelated user changes.

### Phases
- [x] Phase 1 - trace current modal state, specification lifecycle, and AI deconstruction schema
- [x] Phase 2 - implement draft withdrawal and implementation-ready AI specification fields
- [x] Phase 3 - unify companion-panel state, centering, host close behavior, and demand-detail control styling/copy
- [x] Phase 4 - run focused backend/frontend checks and browser-visible validation where available

### Current Phase
Complete - draft withdrawal is guarded, AI specifications include implementation boundaries, all three AI companions share one centered host contract, and host close behavior cannot orphan the right panel.

### Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Initial broad `rg` included missing `src` and `apps` roots and produced a noisy truncated result | 1 | Narrow subsequent reads to exact server and Svelte files with line-numbered ranges |
| First delivery-control style patch assumed a 760px breakpoint, but the current file uses 900px | 1 | Re-read the live style tail and apply smaller exact-context changes against the 900px block |
| Focused Go test tried to use the sandbox-blocked default cache under `~/Library/Caches/go-build` | 1 | Re-run with `GOCACHE=/tmp/well-ambient-gocache` |
| A second local API process could not bind port 8080 because the existing development API already owned it | 1 | Reuse the already-running authenticated development stack for browser validation and stop only the preview process started by this task |

## Current Task Addendum: Compact Task Rows, Inspector Pills, And Filter Layering

### Goal
Reduce Jira Task row density, render inspector facts as compact capsules with icon-only task types, and ensure the top project/owner dropdowns render above following dashboard sections without clipping.

### Protection Rules
- Preserve Jira navigation, row selection, filters, task details, evidence actions, and accessible task-type names.
- Replace visible Bug/Task text with semantic SVG icons while retaining `aria-label` and tooltip text.
- Remove secondary row metadata rather than hiding primary Jira key, title, owner, status, risk, evidence state, sync time, or action.
- Fix dropdown stacking at the owning filter surface; do not globally raise every select menu.

### Phases
- [x] Phase 1 — inspect screenshot and trace active row, inspector, and dropdown structures
- [x] Phase 2 — implement compact markup, icon treatment, capsule facts, and scoped layering
- [x] Phase 3 — run build/diff checks and browser visual/interaction validation

### Current Phase
Complete — status rows are single-line and compact, inspector facts use wrapping capsules, task types are icon-only with accessible names, and the top filter menus render above following cards without clipping.

### Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|


## Current Task Addendum: Task Panels Height And Internal Scroll Alignment

### Goal
Align the personnel load table with its detail inspector, and make the Jira Task execution-row panel a stable-height workbench whose table scrolls internally while matching the right inspector height.

### Protection Rules
- Keep table headers, row selection, task-detail actions, filters, and responsive stacking behavior unchanged.
- Use one shared desktop panel-height contract per two-column workbench; do not introduce page-level horizontal overflow.
- Keep toolbar/header regions fixed and assign vertical scrolling only to the data body/shell.
- Release fixed heights when the layout collapses to one column.

### Phases
- [x] Phase 1 — locate active markup and current grid/overflow contracts
- [x] Phase 2 — implement shared stretch and fixed-height panel contracts
- [x] Phase 3 — validate build, diff hygiene, and browser-visible dimensions/scroll ownership

### Current Phase
Complete — both desktop workbenches use the same responsive fixed-height contract, left and right panels align exactly, table headers remain fixed, and row overflow is owned by the inner table shells.

### Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| `rg` placed `--glob` after path arguments, so ripgrep interpreted it as a path | 1 | Keep ripgrep options before search roots and use the follow-up searches only on explicit source paths |
| Source inspection paths were still prefixed with `web/` while the command was running inside `web/` | 1 | Keep build commands rooted in `web/`, but run repository-relative source inspection from the repository root |
| A combined preview command repeated repository-root paths from inside `web/`, and the sandbox denied binding port 4175 | 1 | Separate root-level inspection from preview startup and request the already-scoped Vite preview permission for the local port |


## Current Task Addendum: Health Diagnosis Intervention Modal Calibration

### Goal
Rebuild the health diagnosis and intervention modal into the established calm light admin-console visual system, reducing harsh contrast while preserving risk legibility and all diagnostic actions/data.

### Protection Rules
- Keep diagnosis calculations, intervention content, attributes, scrolling, and close behavior unchanged.
- Use the existing neutral/cyan management-console palette; reserve muted red for genuine intervention risk only.
- Remove nested high-contrast cards and competing accent colors before changing information structure.
- Validate the real modal at desktop and constrained viewport sizes without introducing page-level overflow.

### Phases
- [x] Phase 1 — inspect screenshot, locate active component/styles, and identify inherited conflicts
- [x] Phase 2 — define calmer hierarchy, palette, spacing, and responsive layout contract
- [x] Phase 3 — implement scoped markup/style calibration
- [x] Phase 4 — run static/build checks and browser visual verification

### Current Phase
Complete — the modal now uses the calm light workbench hierarchy, restrained semantic tones, content-owned scrolling, and responsive single-column fallbacks; build, diff hygiene, and desktop browser verification pass.

### Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| `rg` treated a token pattern beginning with `--wa-` as a command flag | 1 | Use `rg -- '<pattern>'` for CSS custom-property searches |
| Browser evaluation surface did not expose `document.createElement` for the isolated modal harness | 1 | Reuse the existing body element and assign the harness markup directly instead of creating nodes imperatively |
| Browser evaluation DOM also exposes `innerHTML` as read-only | 2 | Stop mutating the constrained DOM; serve a temporary static harness through the existing local preview instead |
| The in-app browser Playwright facade does not expose `setViewportSize` | 1 | Verify the desktop viewport in-browser and inspect the explicit 860px/620px responsive rules statically; record the constrained-viewport limitation instead of claiming a live resize |
| Final `pnpm build` was first invoked from the repository root, which has no package manifest | 1 | Run frontend package commands from `web/`; keep repository-root checks limited to git and source inspection |

## Current Task Addendum: Streamed LLM Attachments And Demand File Archive

### Goal
Replace browser-side document parsing with multipart streaming to the backend, pass uploaded files through the configured LLM request path, and persist gzip-compressed originals with a durable demand/deconstruction association.

### Protection Rules
- Preserve the current JSON-only `/api/deconstruct` contract for callers without attachments.
- Do not keep PDF/DOC parsing libraries in the browser bundle.
- Enforce bounded upload size and allowed MIME/extensions before persistence or provider calls.
- Store files under an application-owned data directory with generated names; never trust client paths.
- Associate attachments with a demand when selected and with the resulting deconstruction archive/context for independent deconstruction.

### Phases
- [x] Phase 1 — trace LLM provider, deconstruction archive, database migration, and storage configuration contracts
- [x] Phase 2 — define attachment schema, gzip storage service, multipart request contract, and provider file strategy
- [x] Phase 3 — implement backend persistence/association and LLM attachment delivery
- [x] Phase 4 — replace frontend parsing with streamed multipart upload and remove parser dependencies
- [x] Phase 5 — add focused tests and run backend/frontend validation

### Current Phase
Complete — original attachments stream through Files + Responses, gzip locally, and retain demand/context/archive provenance; backend suite, frontend build, and diff hygiene pass.

### Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Previous implementation parsed files in the browser instead of preserving and sending originals | 1 | Replace the architecture end-to-end; remove browser parsers after backend path is verified |
| zsh rejected an unmatched `.env*` search glob | 1 | Quote glob arguments or search explicit directories/files |
| `rg` was given nonexistent root `main.go` | 1 | Search only discovered paths (`cmd/server/main.go`, `internal/`) |
| `pnpm remove` selected the workspace-local store while `node_modules` was linked to the user store | 1 | Retry with the existing explicit store path; do not reinstall or alter unrelated packages |
| Go tests could not write the default macOS user cache under sandboxing | 1 | Re-run with `GOCACHE=/tmp/well-ambient-gocache` |
| Full server suite reached an unrelated `httptest` case that cannot bind a local port in the sandbox | 1 | Add and run focused attachment tests with approved local-loopback access; keep unrelated suite failure separate |

## Current Task Addendum: Demand Difficulty And Task Detail Surface

### Goal
Replace the unreliable bespoke difficulty dropdown in demand scheduling with the shared Select interaction contract, and align the task-detail modal content with the established light admin-console palette.

### Phases
- [x] Phase 1 — trace difficulty state/save flow and audit task-detail style inheritance
- [x] Phase 2 — replace difficulty selection and normalize saved-value feedback
- [x] Phase 3 — scope a complete light task-detail surface palette
- [x] Phase 4 — run frontend checks/build, diff hygiene, and targeted interaction/visual verification

### Current Phase
Complete — difficulty uses shared Select and task details use a scoped light surface; static, build, persistence, and diff checks pass.

## Current Task Addendum: Adaptive Overlays And Decision Timeline

### Corrective Follow-up: Preserve Event Dates
- [x] Trace future-looking timeline times to the API contract
- [x] Add full `occurred_at` timestamps while retaining legacy `time`
- [x] Make the frontend prefer full timestamps and safely infer yesterday for future-looking legacy time-only values
- [x] Verify screenshot event records against the local database and run backend/frontend regression

### Corrective Follow-up: Decision Reschedule Consistency
- [x] Trace HR-4202 from the intervention request through TaskTelemetry and the schedule projection
- [x] Confirm the recorded intervention was a no-op (`2026-06-25` to `2026-06-25`)
- [x] Make the date picker emit an explicit target value and expose that value on the submit button
- [x] Reject unchanged reschedule requests and return the persisted due date for client verification
- [x] Add a cross-boundary regression from intervention to schedule projection

### Goal
Fix notification dismissal, make shared select/date overlays choose an upward or downward placement from live viewport space, and replace the oversized split decision logs with one date-grouped event timeline in the main decision panel.

### Protection Rules
- Preserve notification actions, profile behavior, API contracts, agenda polling, filters, and intervention submission.
- Solve overlay placement in shared primitives so all consuming pages inherit the behavior; keep click-outside and Escape dismissal.
- Merge automatic and manual events visually without changing their underlying sources or evidence links.
- Preserve unrelated uncommitted KPI, schedule, backend, database, and task-status work.

### Phases
- [x] Phase 1 — inspect screenshots, active shell, shared overlay primitives, and decision-log structure
- [x] Phase 2 — implement click-outside dismissal and viewport-aware overlay placement
- [x] Phase 3 — implement one date-grouped automatic/manual decision timeline
- [x] Phase 4 — run frontend checks/build, diff hygiene, and browser interaction/layout verification

### Current Phase
Complete — shared overlays are viewport-aware, the active notification popover dismisses on outside click, and decision events share one compact date-grouped timeline.

### Validation Contract
- `pnpm --dir web check`
- `pnpm --dir web build`
- `git diff --check`
- Browser verify notification outside-click dismissal, select/date upward placement near the viewport bottom, normal downward placement when space allows, and compact date-grouped timeline rendering.

## Current Task Addendum: KPI Workspace Clean Rebuild

### Goal
Replace the broken KPI page with one coherent, source-order-aligned product workspace. Eliminate every legacy grid-area/layout override and rebuild daily/weekly health, risk, personal workload, delay, requirement base score, capability rating, and evidence review from the existing APIs.

### Root Cause And Protection Rules
- Root cause: the previous iteration changed markup but retained several generations of `.kpi-dashboard` grid-area and responsive overrides. Removed regions still reserved visual tracks, while the report panel kept old sticky/column behavior.
- Delete the legacy KPI component presentation and CSS completely; do not append another override layer.
- Preserve `/api/kpi/performance`, `/api/kpi/report-preview`, permission behavior, daily/weekly switching, real evidence, and the new backend workload/base-score fields.
- One DOM reading order must equal one visual reading order. Any two-column section owns its own local grid and collapses independently.
- Preserve unrelated uncommitted work and never modify background-generated `task_status.md` or `well-ambient.db`.

### Phases
- [x] Phase 1 — screenshot/root-cause audit and protection rules
- [x] Phase 2 — replace KPI component with a clean data/read-model layer
- [x] Phase 3 — implement the new single-flow KPI workspace and responsive system
- [x] Phase 4 — run frontend/backend validation and authenticated desktop/narrow browser QA

### Current Phase
Complete — the legacy KPI layout is gone and the clean rebuild passes static, build, interaction, wide-screen, medium, and narrow-browser verification.

## Current Task Addendum: Main Workspace Information Architecture And Metrics Rebuild

### Goal
Remove the duplicate page-title cards immediately below the shared breadcrumb, consolidate each workspace's first-screen actions and signals, split task tracking into shell-owned submenus, and rebuild metrics around daily/weekly delivery health plus person-level workload, delay, bugs, and ability rating.

### Design Read
Product delivery governance · calm precision · register=product · SOUL=4 · SPECTACLE=2 · DENSITY=9.

### Protection Rules
- Preserve the shared light-console shell, current brand accent, authentication/authorization, API contracts, filters, drill-downs, demand creation, and delivery-control workflows.
- The shared shell owns breadcrumb and page identity; child workspaces start with actionable data or controls, not another title/description card.
- Keep task list and execution trace as distinct submenus because they answer different questions; share filter/state language so the user does not re-learn the page.
- Use real task, demand, risk, evidence, report, and member data. Any derived ability score must expose its calculation inputs and allow the existing/manual base-score path to remain authoritative.
- Preserve prior uncommitted schedule/modal work and do not modify background-generated `task_status.md` or `well-ambient.db`.

### Phases
- [x] Phase 1 — audit shell subnavigation, page-local headers, data contracts, and score sources
- [x] Phase 2 — flatten Decision, Schedule, and Evidence first-screen hierarchy
- [x] Phase 3 — move Task Table, People Load, and Execution Tracking into shell submenus; rebuild People Load
- [x] Phase 4 — rebuild KPI daily/weekly and personal capability surfaces using real data
- [x] Phase 5 — run frontend checks/build, diff hygiene, and authenticated visual verification

### Current Phase
Complete — all five workspaces pass static, focused backend, production-build, desktop browser, and narrow-layout verification.

### Validation Contract
- `pnpm --dir web check`
- `pnpm --dir web build`
- `git diff --check`
- Browser-verify all affected routes at desktop and narrow widths, including submenu state, no duplicate intro card, no unexpected horizontal overflow, and readable loading/empty/error states.

## Current Task Addendum: Rounded Surface And Modal Corner Audit

### Goal
Remove the sharp rectangular corner artifacts around the new-demand modal and establish a consistent rounded-surface contract for modals, cards, panels, table shells, inspectors, and configuration containers across the admin application.

### Protection Rules
- Preserve the existing calm light-console palette, content density, responsive behavior, and all business interactions.
- Fix shared surface primitives first; add local overrides only where legacy specificity or pseudo-elements still break rounded corners.
- Rounded containers must clip their own background, border, decorative layers, and inner sections so square children cannot escape the parent radius.
- Keep the prior uncommitted schedule-table/date-control work in `DemandKanban.svelte`; do not stage or modify background-generated `task_status.md` or `well-ambient.db`.

### Phases
- [x] Phase 1 — inventory modal/card/container selectors, pseudo-elements, shadows, overflow, and radius tokens across all primary pages
- [x] Phase 2 — repair shared rounded-surface primitives and the new-demand modal corner artifact
- [x] Phase 3 — close page-specific gaps without flattening intentional nested control radii
- [x] Phase 4 — run frontend check/build, diff hygiene, and authenticated-browser-safe visual verification where available

### Current Phase
Complete — shared and page-level rounded surfaces pass static, build, and authenticated browser verification.

### Validation Contract
- `pnpm --dir web check`
- `pnpm --dir web build`
- `git diff --check`
- Verify that modal/card/panel backgrounds and decorative layers are clipped to their declared radius and no horizontal/vertical layout regressions are introduced.

## Current Task Addendum: Controlled Autonomous Delivery Rollout

### Goal
Turn the existing intent recognition, AI deconstruction, context-pack archive, GitLab telemetry, and AI MR review foundations into a controlled autonomous delivery loop: human-freeze a versioned demand specification, execute only through policy gates, create isolated branches/commits/Draft MRs, resolve reviewers from a review contract plus the real diff, require CI and human acceptance, and feed delivery outcomes back as auditable corpus candidates.

### Protection Rules
- Never write directly to a protected/default branch; all automated work uses a dedicated topic branch and Draft MR.
- AI output is never promoted directly into the trusted knowledge registry; it first becomes a versioned candidate with provenance and human disposition.
- A demand must have a frozen specification and approved review contract before execution can start.
- Author, code reviewer, and business acceptance owner remain distinct identities; self-review cannot satisfy an approval requirement.
- Every transition and external side effect must be idempotent, auditable, permission-checked, and recoverable.
- Existing deconstruction, task import, schedule, telemetry, RBAC, and Settings behavior must remain compatible.

### Phases
- [x] Phase 0 — checkpoint the dirty baseline and write the delivery plan
- [x] Phase 1 — add domain models, permissions, state machine, and migrations
- [x] Phase 2 — add demand-spec draft/edit/freeze APIs and review-contract APIs
- [x] Phase 3 — add reviewer resolution and execution preflight policy
- [x] Phase 4 — add controlled execution runs, GitLab branch/commit/Draft-MR adapter, and audit actions
- [x] Phase 5 — connect demand/deconstruction UI to human review, freeze, execute, and verification states
- [x] Phase 6 — add delivery feedback and knowledge-candidate review loop
- [x] Phase 7 — run focused backend/frontend regression, close gaps, update docs, and commit the completed rollout

### Current Phase
Complete — all controlled autonomous delivery phases passed final validation and are included in the delivery commit.

### Validation Contract
- Focused Go tests for every new state transition, policy gate, reviewer rule, and GitLab request.
- `go test ./...` at final integration.
- `pnpm --dir web build` at each UI boundary and final integration.
- `git diff --check` before each phase commit and final delivery.

### Errors Encountered
| Error | Attempt | Resolution |
|---|---:|---|
| Go tests could not write the default macOS build cache under `~/Library/Caches/go-build` | 1 | Treat as an environment-only sandbox failure; use `/tmp/well-ambient-gocache` for subsequent tests and request a policy-compliant rerun of the failed validation. |
| New `compactStrings` helper collided with an existing strongest-brain helper that accepts a limit argument | 2 | Renamed the helper, then replaced one missed call in default reviewer-candidate construction; existing shared helper remains unchanged. |
| Phase 4 `httptest.Server` could not bind `[::1]:0` inside the sandbox | 1 | Treat as a local-listener sandbox restriction and rerun the focused HTTP contract tests with approved escalation; no real network service is contacted. |
| Draft MR execution succeeded externally but demand evidence projection used nonexistent `mr_iid` column | 1 | Use the existing schema's `mr_i_id` column and make evidence projection transactional/error-returning instead of silently ignoring failures. |
| Combined state/audit patch missed current gofmt-adjusted context | 1 | Re-read exact snippets and split the update into narrow patches; no partial changes were applied. |
| AI change-set generator referenced telemetry package's private `queryLLM` | 1 | Add a server-local protocol-compatible LLM client and keep package boundaries explicit. |
| Final `go test ./...` exposed two baseline regressions in demand-option visibility and strongest-brain evidence-missing queue rules | 1 | Reproduce each focused test, inspect current shared filtering/read-model logic, apply minimal compatibility fixes, then rerun full regression. |

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

---

# 2026-07-31 执行追踪与任务表性能、数据边界和轨迹弹窗

## 目标与保护边界

- [x] 执行追踪只展示真实 Jira/本地执行任务；Git commit、Push、MR 只能作为执行证据，不得生成主表行。
- [x] 任务表继续只展示 Requirement/Bug Work Item，并把列表查询从逐行补数改为有界批量查询。
- [x] 执行追踪与任务表不再每 15 秒同时全量刷新；后台刷新不得重入，也不得让非当前视图阻塞当前操作。
- [x] 执行追踪的 Bug/Task 标记统一使用任务表同款结构；轨迹标识与消息单行省略，完整文本通过标题可达。
- [x] 轨迹从右侧抽屉改为共享 `Modal` 的 `wide` 弹窗，沿用决策面板的遮罩、层级、焦点、滚动与关闭语义。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：主表保持单一行结构，类型、编号、标题和状态形成固定阅读顺序；轨迹是临时深挖证据，使用共享宽弹窗且只保留一个滚动所有者。长分支、仓库、hash 与消息不得撑高时间线，统一单行省略并保留完整 `title`。
- **design-taste-frontend**：该技能不主导高密度后台数据表，仅采用 redesign-preserve。保留 Phase 41 浅色研发治理工作台、现有 token、字体和列密度，不引入营销页层级、装饰性视觉或新依赖。
- **finesse-ui**：Design Read 为研发交付控制台，`register=product`、`SOUL 4 / SPECTACLE 1 / DENSITY 8`。主表是事实层，commit/MR 是证据层；两者必须在视觉和数据模型上分开。弹窗内容使用稳定摘要 + 受控时间线，不用嵌套卡片堆叠制造层级。

### 分歧与取舍

- 产品工作台通常可用抽屉保持主表上下文，但用户明确要求参考决策面板改为弹窗；因此任务跟踪入口使用共享 `Modal size="wide"`，排期页的内联轨迹和其他现有调用保持不变。
- 不通过更小字号或强制压缩全部证据来实现“一行”；采用 `white-space: nowrap + ellipsis + title`，窄屏仍允许摘要区重排，但轨迹文本不参与纵向膨胀。
- 不把所有性能问题归因于索引。现有类型表达式索引已命中；本轮优先修复 Work Item N+1、commit 派生行、全量并发轮询和重复渲染，再用查询计划确认索引剩余价值。

## 组件所有权、响应式与验证范围

- `internal/deliveryplanning/service.go`：拥有 Work Item 批量快照装配，列表查询次数必须与行数无关。
- `internal/server/execution_handlers.go` 与 `internal/telemetry/handler.go`：拥有执行任务/纯 Git 证据边界和服务端筛选顺序。
- `TaskKanban.svelte`：拥有当前视图加载、过滤结果渲染、Bug/Task 同构标记和轨迹弹窗调用，不复制弹窗实现。
- `CommitTelemetryPanel.svelte`：拥有轨迹内容；新增 modal presentation 并复用共享 `Modal`，drawer/inline 兼容调用不变。
- 桌面：主表与检查器层级不变，宽弹窗最大 960px；窄屏：弹窗遵循共享 24px/12px 安全边距，摘要可两列，时间线保持单行省略且无文档级横向溢出。
- 回归：查询次数、commit 派生行、Jira/local 执行任务保留、Work Item 类型边界、定时刷新去重、弹窗焦点/关闭/滚动、1440/760/390 登录态浏览器几何与筛选响应。

## 当前诊断与红灯

- 真实库有 685 条 Work Item。当前 `QueryPlan` 对每行分别查询 release link/release 和最新同步操作，完整加载产生约 1,370 次附加 SQL，是任务表首要慢因。
- 任务跟踪类型表达式索引已经命中；Git 证据也已有 `(task_id, created_at DESC)` 复合索引。问题不是单纯缺索引，排序形状和逐行补数仍会产生临时排序/N+1。
- 执行追踪当前把 Git webhook 自动生成、标题直接来自 commit message 的 `issue_type=task` 记录当成执行任务；实际库中可见 `middleq-*`、`actual-*`、`revert-*` 等纯 commit 行。
- 前端每 15 秒同时全量刷新 Work Item、core member 目录和执行任务；请求可重入，且当前视图无关的数据也会抢占查询与渲染。

## 实施与验证结果

- Work Item 快照由逐行查询改为批量装配；40 行查询计数从红灯的 96 次降为固定不超过 6 次，完整列表不再按行追加 SQL。
- 执行目录在服务端排除 `source=git/gitlab` 与兼容旧数据的 commit 派生记录；Git webhook 新建记录显式标记 `source=git`，更新既有执行任务时保留 source、项目、外部编号、父任务、规划状态与 revision。
- 前端只加载当前任务视图，60 秒可见态刷新且禁止请求重入；执行风险筛选改为显式响应式依赖，420 条本地夹具筛选到精确 1 条已在浏览器验证。
- 执行追踪复用任务表的 B/T 类型结构，类型来自父 Work Item；代码轨迹改为共享 `Modal size="wide"`，18 条长轨迹在 1440、760、390 三档均保持单行省略和弹窗内滚动。
- 现有任务类型表达式索引与 Git `(task_id, created_at DESC)` 索引已通过查询计划确认命中；反复卡顿主因是 N+1、全视图轮询、commit 数据越界和无效筛选渲染，不是缺少索引。本轮不新增重复索引。
- `go test ./internal/deliveryplanning ./internal/telemetry`、聚焦 server catalog 测试与完整 `internal/server` 套件通过；`pnpm --dir web check` 为 0 error（80 条既有 warning），生产构建通过。Impeccable 无目标发现，Finesse 无 P0；目标组件仅报告 TaskKanban 中既存的纯黑白中性色 P2。
- 隔离浏览器夹具覆盖 685 条 Work Item、420 条执行任务、Bug/Task 展示、精确搜索、轨迹弹窗和三档响应式；控制台无 error，验证入口及服务已清理。
- **Status:** complete

---

# 2026-07-31 任务表事实源校正、搜索回车定位与详情统一

## 目标与保护边界

- [x] 全局搜索输入需求或 Bug 后按 Enter，选择精确匹配项；无精确匹配时选择当前排序首项，并进入任务表定位高亮对应行。
- [x] “任务表”只展示权限范围内的 Work Item（需求 / Bug），不再把 Git 提交派生的 Execution Task 当作任务表主数据。
- [x] “执行追踪”和“人员负载”继续使用 Execution Task 及其证据目录，不把 Work Item 与执行证据重新混成一个读模型。
- [x] 任务表的项目与负责人筛选由可见 Work Item 及权威项目目录生成；切换任务视图时不遗留另一个领域的无效筛选值。
- [x] 修复任务表右侧检查器的文本重叠、强制裁切和隐藏列表项，并把任务详情统一为决策看板的共享 `Modal`、事实网格和分区结构。
- [x] 保留现有 Phase 41 浅色后台、导航、权限、Jira 跳转和脏工作区中的其他改动；不新增视觉系统、依赖或远程写入。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：任务表必须先校正信息架构，再调整表格。主表按“事项身份、类型、项目、负责人、状态、目标版本、最后同步”组织；右栏是随行检查器，应独立滚动且完整展示事实和正文，不能靠截断或隐藏内容维持高度。
- **design-taste-frontend**：该技能不主导高密度后台产品页，仅采用 redesign-preserve。保留 Phase 41 的字体、色彩、控件和表格语言，不引入营销层级、英雄区、渐变或新设计系统。
- **finesse-ui**：Design Read 为“工程交付控制台 · 克制、表格优先、证据清晰”，`register=product`、`SPECTACLE=2`、`DENSITY=8`。详情使用共享弹窗标题、稳定的事实网格和少量语义分区；动效只服务于搜索定位反馈。

### 分歧与取舍

- 上一轮把任务表、执行追踪、人员负载统一到 `/api/execution/tasks`，与本轮用户确认的领域语义冲突。本轮以项目领域模型为准：任务表回到 `/api/work-items`，执行追踪与人员负载保留 `/api/execution/tasks`。
- 不把任务详情复制成决策看板的业务字段；只统一弹窗容器、标题层级、事实网格、分区和响应式节奏，内容仍是 Work Item 自身事实。
- 不通过缩小字体、三行截断或隐藏第三条以后列表项解决右栏高度。桌面右栏独立滚动，窄屏跟随主表自然堆叠，动态文本完整可达。
- 不改动共享 `Modal` 或全局 token，以免影响决策看板等已验证页面；由 `TaskKanban.svelte` 使用同一共享组件并收敛局部结构。

## 组件所有权、响应式与验证范围

- `FunctionalAdminShell.svelte`：拥有全局搜索、Enter 提交、精确 / 首项选择与任务表导航；搜索事实源改为 `/api/work-items`。
- `TaskKanban.svelte`：状态视图拥有 Work Item 表格、筛选、选择、检查器和详情；执行 / 人员视图继续拥有 Execution Task 数据与筛选。
- `/api/work-items`：需求 / Bug、项目、负责人、版本、计划与同步事实的权威读取入口；`/api/execution/tasks`：执行任务、代码证据、风险与人员负载入口。
- `Modal.svelte`：继续拥有焦点、遮罩、关闭与尺寸语义；任务详情仅复用决策看板采用的普通 `wide` 弹窗模式，不修改共享实现。
- `>=1281px`：主表 + 辅检查器，检查器内容独立纵向滚动，事实两列；`761–1280px`：表格在上、检查器在下；`<=760px`：事实单列、分区单列、表格横向受控滚动且页面无横向溢出。
- 红灯：Enter 当前只执行查询而不选中；任务表当前请求 `/api/execution/tasks`；右栏最终桌面规则强制 `overflow:hidden`、正文三行截断并隐藏第三条以后列表项。
- 回归：前端检查与构建、Impeccable / Finesse 检测；登录态浏览器覆盖需求与 Bug 搜索 Enter、精确 / 首项定位、项目 / 负责人下拉、任务表 / 执行 / 人员数据边界、长文本右栏、详情弹窗，以及桌面和窄屏断点。

## 实施与验证结果

- 全局搜索已改用 `/api/work-items`，Enter 会选择编号 / 标题精确匹配项，否则选择当前首项，并导航到任务表滚动定位高亮行。
- 状态视图已改为 Work Item 表格，项目和负责人筛选来自可见事项与权威项目目录；执行追踪和人员负载继续使用 Execution Task，视图切换会清理跨领域筛选。
- 右侧检查器改为完整正文、完整列表和独立纵向滚动；任务详情已复用决策看板的 `wide` 共享弹窗、事实网格与分区结构。
- 隔离登录态浏览器已覆盖桌面与 390px 窄屏：需求 / Bug 回车定位、Work Item / Git 执行边界、项目 / 负责人选项、右栏长文本和弹窗响应式均通过，页面无横向溢出且控制台无错误。
- 前端检查为 0 error，生产构建、Impeccable / Finesse 检测及目标差异空白检查均通过；验证仅使用本地只读夹具，没有调用外部集成或写入业务数据。
- **Status:** complete

---

# 2026-07-31 版本计划多 Jira 关联、下拉与版本标识纠偏

## 目标与保护边界

- [x] 解释并移除“全部项目”后的无意义清除叉号，只在真实选中非空值时提供清除动作。
- [x] 修复共享项目下拉在弹窗和右侧检查器滚动容器内被裁切、长项目名显示不全的问题。
- [x] 创建版本的校验失败、服务失败和创建成功均进入右上角全局 Toast；字段错误同时保留就地说明。
- [x] 版本只关联本地已同步的多条 Jira 事项，不创建、不选择、不写回 Jira 版本；移除重复的 Jira 项目下拉框。
- [x] 右侧检查器支持按所绑项目搜索、批量选择并原子加入 Jira 事项，也支持查看和移出已有事项。
- [x] 项目看板卡片存在主目标版本时直接显示版本名称。
- [x] 复用并强化现有 `work_item_release_links`，用复合索引支撑版本计数、候选查询和卡片版本投影；不破坏或删除历史表。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：这是 Phase 41 高密度产品工作台的定向修复。版本事实表继续承担扫描，右侧检查器继续承担关系编辑；创建弹窗只负责版本字段。下拉必须逃离 `overflow` 裁切上下文，成功与失败使用共享 `aria-live` Toast，字段错误留在字段附近。所有列表需覆盖加载、空、错误、只读和冲突状态。
- **design-taste-frontend**：该技能明确不主导 dense product UI，因此只采用 redesign-preserve 约束。保留现有路由、青绿色令牌、系统字阶、表格结构、右侧检查器和共享组件，不引入营销页布局、Hero、装饰性动效或第二套组件库。
- **finesse-ui**：Design Read 为“研发交付治理产品工作台，清晰、可信、高密度”，`register=product`，`SOUL 4 / SPECTACLE 1 / DENSITY 9`。版本与 Jira 的关系应使用标准批量选择和明确状态，不使用 Jira 版本目录或重复的项目选择；动画只用于下拉、Toast 和提交反馈。

### 分歧与取舍

- finesse 通用 substrate 建议纹理和更强材质，但与 `DESIGN.md` 的 Phase 41 平整、表格优先契约冲突。本次以仓库契约为准，不增加纹理、渐变、玻璃卡片或新阴影层。
- design-taste 建议密集后台采用成熟组件系统；仓库已经有共享 `Select`、`MultiSelect`、`Modal`、`ToastHost` 和完整 tokens，继续修复这套系统，不再引入 Atlaskit 等第二套依赖。
- “版本关联 Jira”按本地业务归属解释，不等同于 Jira `fixVersion`。版本的所属项目是唯一范围来源，Jira 事项的项目从已同步事实和 Jira Key 派生，因此删除 Jira 项目下拉框。
- 关系数据继续使用 `work_item_release_links` 的主目标关系，避免再建一张重复多对多表。只在外部 Jira 版本身份实际变化时生成 Jira 版本写回 outbox；本地版本关联不得清空或修改 Jira `fixVersion`。

## 组件所有权、响应式与验证范围

- `internal/db/delivery_planning.go`：定义版本事实和版本到工作项关系的复合索引；保留历史 `release_jira_links` 表但新流程不再读取或写入。
- `internal/deliveryplanning`：继续拥有项目一致性、主目标版本、CAS revision、事件和外部写回边界。
- `release_handlers.go`：提供版本列表中的 Jira 数量、同项目 Jira 候选、原子批量关联和单项移出；不要求 Jira 项目或 Jira 版本。
- `schedule_handlers.go`：一次批量查询投影主目标版本，避免项目看板逐卡查询。
- `Select.svelte`：共享清除语义与 portal 下拉所有者；项目页面不自行计算浮层位置。
- `DeliveryPlan.svelte`：版本页所有者，负责项目绑定、候选搜索、批量选择、关系列表和 Toast。
- `ProjectBoard.svelte`：只消费后端版本投影并显示版本标识，不解释或修改版本关系。
- 桌面保持表格加检查器；低于 1100px 变单列，低于 760px 保持 44px 触控目标。下拉在弹窗、检查器及滚动后均不可被裁切或越出视口。
- 红灯测试覆盖：空值项目筛选无清除按钮、批量关联两条 Jira、跨项目和已占用冲突、原子回滚、本地版本不生成 Jira 版本 outbox、项目看板版本投影、所需复合索引存在。
- 实现后执行定向与全量 Go 测试、`pnpm --dir web check`、生产构建、Impeccable detector、Finesse anti-cheap/preflight，并在认证隔离浏览器验证桌面与 760/390 窄屏。

## 交付结果

- [x] 新增版本 Jira 候选、批量关联、单项移出 API，并在版本列表返回关联事项数量。
- [x] 创建版本成功和失败均使用右上角 Toast；字段级错误继续就地呈现。
- [x] 共享 Select 使用 body portal 和视口约束定位，空值“全部项目”不再显示清除叉号。
- [x] 右侧检查器移除 Jira 项目/Jira 版本选择，按版本所属项目搜索并批量关联多条 Jira 事项。
- [x] 项目看板一次批量投影主目标版本并在 Jira 卡片显示版本名称。
- [x] 输出 ADR 与表结构、索引、查询和迁移设计；历史 `release_jira_links` 仅保留兼容，不作为新流程数据源。
- [x] 全量 Go 测试、前端检查/构建、设计检测、差异检查和隔离认证浏览器验证通过。
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
| Phase 52 reused an expired browser binding | 1 | Reinitialized the browser runtime and selected a fresh Chrome binding before continuing authenticated validation. |
| Phase 52 error-log patch used stale context | 1 | Re-read the current error table and appended with exact local context. |

## Phase 47: Flow Board Scroll and Reusable AI Deconstruction

- [x] Trace the flow-board height chain, demand-detail modal lifecycle, and current deconstruction ownership
- [x] Give the four flow lanes a bounded viewport with independent scrolling and a continuous light surface
- [x] Harden demand-detail backdrop handling and normalize the controlled-delivery panel typography/layout
- [x] Expose one AI deconstruction workbench from flow details, schedule inspection, and new-demand capture
- [x] Run targeted frontend validation and record the delivered behavior

## Phase 48: Flow Visibility Regression and Contextual AI Panel

- [x] Replace the indefinite zero-height flex board with an explicit viewport-clamped lane region
- [x] Move demand association to the top of the compact AI workflow and explain its task-group effect
- [x] Reduce AI modal content to input, association, generated task summary, and synchronization checks
- [x] Make new-demand AI pre-deconstruction a responsive right-side companion panel
- [x] Source new-demand project choices exclusively from the project configuration endpoint
- [x] Prevent horizontal resizing of the requirement description textarea
- [x] Re-run frontend build, TypeScript, and diff hygiene validation

## Phase 49: AI Companion Alignment and UI Delivery Gate

- [x] Measure the new-demand modal and bind the AI companion to the same rendered height
- [x] Center both desktop dialogs as one group with a 16px gap
- [x] Separate demand title and description in the compact UI
- [x] Send labeled title/content sections to the deconstruction LLM endpoint
- [x] Add mandatory authenticated browser simulation and screenshot review to delivery rules
- [x] Authenticate in the real app and capture responsive and 2048x925 validation screenshots
- [x] Verify color, spacing, alignment, overflow, field separation, and responsive fallback

## Phase 51: Demand Association Inline Search

- [x] Add in-place filtering to both compact demand-association dropdowns
- [x] Search by demand ID, title, owner, repository, or project
- [x] Show live result counts and an explicit empty state
- [x] Reset the query after selection, dismissal, or reopening
- [x] Verify filtering and selection in the authenticated browser
- [x] Capture and review the affected UI state by screenshot

## Phase 52: Inspector Density, Global Search, and Login Field Polish

- [x] Audit the schedule inspector, shell search, and login field ownership without overwriting unrelated dirty-tree work
- [x] Convert repeated inspector facts into compact wrapping capsules while preserving title, progress, and acceptance hierarchy
- [x] Implement usable global search submission and result navigation in the authenticated shell
- [x] Restyle the email prefix/suffix login control as one continuous accessible field group
- [x] Run frontend build, TypeScript, diff hygiene, authenticated browser interaction, and screenshot review
- **Status:** complete

## Phase 53: Streamed AI Specification Draft And Markdown Preview

- [x] Trace the current draft-generation request, provider call, persistence boundary, and result rendering path
- [x] Define a streaming contract that keeps the HTTP connection active and separates progress/content/final/error events
- [x] Implement backend incremental delivery without losing the final persisted draft or existing non-stream callers
- [x] Replace the textarea matrix with a safe, unified Markdown preview and deliberate streaming/ready/error states
- [x] Run focused backend/frontend checks plus authenticated browser interaction and screenshot review
- **Status:** complete — streamed persistence, Markdown reading/editing, UTF-8 delivery, desktop/narrow responsive behavior, and authenticated browser visual acceptance all pass

### Protection Rules

- Preserve the existing draft review, edit, freeze, withdrawal, and execution lifecycle; streaming changes transport and presentation, not authorization.
- Never report success until the final draft is persisted and its identifier/version is returned.
- Abort cleanly on client disconnect while distinguishing browser cancellation from provider or persistence failure.
- Keep the current calm light admin-console baseline and render LLM structure semantically instead of flattening it into fixed fields.
- Browser validation must use a loopback-only configuration with every external integration disabled; development login must not contact WellOS before issuing the local session.

### Errors Encountered

| Error | Attempt | Resolution |
|-------|---------|------------|
| Combined validation ran repository-root Go paths from `web/`, so Go files were not found | 1 | Split backend validation at repository root from frontend package validation under `web/`; do not repeat the mixed-working-directory command |
| Stream regression could not start `httptest` because the sandbox denied loopback binding | 1 | Re-run only the focused SSE regression with approved loopback permission and the repository `/tmp` Go cache |
| SSE test fixture split Chinese text at arbitrary byte offsets and produced invalid UTF-8 replacement characters | 1 | Split simulated provider chunks by Unicode runes so each SSE JSON event is valid, matching real provider framing |
| Markdown delimiter look-behind emitted a safe byte count that could still bisect a multibyte Chinese rune | 2 | Clamp each outbound Markdown chunk to the nearest valid UTF-8 prefix boundary before writing the NDJSON event |
| Local port probe assigned to zsh's readonly `status` special variable | 1 | Rename the loop variable to `http_code` before probing the development stack again |
| Safe temporary YAML was superseded by active configuration version 19 from the main database, restoring `0.0.0.0` and Jira sync | 1 | Do not start another server against the main database; identify and reuse an existing project listener only if it is already safe and user-owned |
| Browser metrics probe had an unmatched closing parenthesis | 1 | Replaced it with a shorter read-only DOM projection and reused the successful result |
| Markdown screenshot showed `_暂无内容_` literally because only asterisk emphasis was supported | 1 | Add safe underscore-emphasis parsing and recheck the live preview before acceptance |
| Final task-memory patch used stale validation-list context and did not apply | 1 | Re-read the live state tail and update task, findings, progress, and state with smaller exact-context patches |

## Phase 54: Impeccable Review Gate And Shared Markdown Workbench

- [x] Install `pbakaus/impeccable` into both the user-wide and project skill directories without vendoring repository noise
- [x] Make Impeccable, taste-skill, and finesse-ui a mandatory pre-execution review gate for every UI/frontend change
- [x] Replace the nested specification cards with one flat document workbench and a single legitimate editor boundary
- [x] Extract an editor-grade shared Markdown component with edit, split, and preview modes
- [x] Reuse the shared component for streamed output and persisted draft editing while preserving the governed specification lifecycle
- [x] Run dependency, frontend, Impeccable, and authenticated browser validation at desktop and narrow widths
- **Status:** complete

### Phase 54 Design Review

- **Impeccable:** remove nested containers, use hierarchy and hairlines instead of repeated rounded panels, and extract repeated Markdown behavior into one reusable component.
- **taste-skill:** use a proven editor engine with its native interaction model; do not simulate an editor with a styled textarea.
- **finesse-ui:** product register, high information density, one workbench boundary, restrained motion, and no decorative dashboard treatment.

### Phase 54 Errors Encountered

| Error | Attempt | Resolution |
|-------|---------|------------|
| Python skill download failed because the local CA chain could not verify GitHub | 1 | Reused the official installer with its `--method git` transport; both scoped installations succeeded without disabling TLS validation. |
| The first narrow split view kept the 460px single-mode height and clipped its two 280px panes | 1 | Added a breakpoint-specific 700px split workspace with two bounded equal rows; measured scroll height now matches client height. |
| State inspection assumed `warm_memory` was an array | 1 | Read the schema first and updated the actual object fields with exact-context patches. |

## Phase 55: Inline Markdown Editing And Review Contract Layout

- [x] Make persisted Markdown preview-only by default with no visible mode controls
- [x] Enter CodeMirror on preview double-click and save/exit when focus moves to blank space outside the workbench
- [x] Preserve a keyboard-accessible edit entry without restoring a persistent edit button
- [x] Recompose the review contract into balanced reviewer and policy groups
- [x] Widen the demand-detail modal to a restrained 960px desktop maximum while preserving narrow breakpoints and AI companion geometry
- [x] Run Impeccable, type/build, authenticated interaction, responsive, and screenshot validation
- **Status:** complete

### Phase 55 Three-Way Review

- **Impeccable layout assessment:** remove the persistent mode switch; current equal three-column review grid is visually unbalanced; use 4pt-aligned 8/12/16/24/32 spacing and a structural responsive collapse.
- **Impeccable mechanical scan:** layout detector returned `[]`; historic files contain broad non-4pt spacing, so remediation stays scoped to selectors touched in this phase rather than rewriting the whole dashboard.
- **taste-skill:** keep CodeMirror as the edit engine; direct-manipulation preview is the primary state, with a discoverable text hint and keyboard fallback.
- **finesse-ui:** use one document surface, a two-group review workbench, right-aligned approval actions, and a wider but still contextual 960px modal.
- **Resolved tradeoff:** the isolated layout assessment proposed 1040px and a visible edit button; the user explicitly rejected visible edit controls, so the implementation uses 960px plus a screen-reader/keyboard entry that appears only on focus.

### Phase 55 Validation Exceptions

- A full-file Impeccable scan of `DemandKanban.svelte` reports old side-accent borders, gradient text, and width transitions in unrelated dashboard regions. They predate this phase and are not accepted as new work; the two changed UI components pass a complete scan with `[]`, and all three target files pass `--scope layout` with `[]`.
- The first two isolated layout-review agents stalled before returning; both were interrupted and the same bounded checklist completed successfully in a fresh no-context agent.

## Phase 56: Flat Review Contract And Directory-backed Reviewers

- [x] Trace reviewer defaults, user-directory payloads, saved value semantics, and the existing shared select contract
- [x] Remove the duplicate structured execution-field presentation while preserving its backend payload compatibility
- [x] Add a reusable in-place searchable multi-select and reuse the shared single-select for responsible owners
- [x] Replace stale Jira sync-user reviewer defaults with demand/task-derived candidates and real directory-backed form values
- [x] Recompose the review contract as one flat responsive form without nested fieldsets or card-on-card grouping
- [x] Run focused backend/frontend checks, Impeccable scans, authenticated interaction tests, and screenshot review
- **Status:** complete

### Phase 56 Three-Way Review

- **Impeccable:** remove the redundant raw execution-field UI; eliminate nested review fieldsets; use one consistent form hierarchy and shared primitives.
- **taste-skill:** bind people controls to the real user directory, keep stored reviewer identifiers compatible with the resolver, and expose email/department as searchable context rather than free text.
- **finesse-ui:** use a flat product-form register: candidate reviewers span the row, responsible owners use searchable single selects, policy controls remain compact, and status/actions stay visually subordinate to the document.
- **Resolved tradeoff:** create a dedicated shared multi-select instead of expanding the existing single-select API, so current consumers keep their behavior and the new multi-value interaction remains explicit and testable.

### Phase 56 Validation And Errors

- Focused review lifecycle/default/participant tests pass; `pnpm --dir web check` reports 0 errors and 78 pre-existing warnings; production build, `git diff --check`, targeted Impeccable scan, and layout-scope scan pass.
- Authenticated DG-319 validation confirmed no raw structured-field surface, no stale `Eddie Antigravity`, searchable department/email matching, two simultaneous reviewer selections, and directory-backed defaults for both responsible-owner selects.
- Screenshot: `output/ui-validation-review-contract-directory-selects.png`.

| Error | Attempt | Resolution |
|-------|---------|------------|
| `gofmt` was first invoked from `web/`, so the repository-root Go path was not found | 1 | Run Go formatting and tests from the repository root; keep frontend commands on `pnpm --dir web` |
| A later diagnostic `pnpm check` was invoked from the repository root without `--dir web` | 1 | Re-run the authoritative check with `pnpm --dir web check`; it passes with 0 errors |
| The live Vite page retained a compile overlay from an intermediate HMR state | 1 | Reload the authenticated tab after the production build, then repeat the complete DG-319 flow |
| The participant-directory test assumed only the seeded Reviewer, but the auth helper also registers the author | 1 | Assert the target participant by identity fields instead of assuming an incorrect total directory size |
| Browser locator helpers `inputValue` and `scrollIntoViewIfNeeded` are not exposed by this browser facade | 1 | Use the semantic DOM snapshot for values and a harmless locator click to bring the target section into view |

## Phase 57: Select Dismissal And Core-member Ownership Boundary

- [x] Reproduce the shared single/multi-select dismissal paths and trace the responsible-person data adapters
- [x] Complete the mandatory Impeccable + taste-skill + finesse-ui review before frontend edits
- [x] Make both shared selects close on outside pointer, Escape, focus exit, and explicit chevron toggle
- [x] Restrict reviewer and owner choices to the shared core-member boundary while preserving valid saved values
- [x] Add focused backend regression coverage for the core-member participant boundary and compile-check both shared selector contracts
- [x] Run Svelte/build, Impeccable, diff hygiene, and attempt authenticated browser/responsive validation
- **Status:** complete — implementation and automated gates pass; authenticated browser interaction was attempted but the in-app browser remained locked to its prior localhost connection-error page by browser security policy

### Phase 57 Three-Way Review

- **Impeccable harden:** window-level bubbling `click` listeners are not reliable inside modal content that stops propagation; dismissal must use capture-phase outside-pointer handling, keyboard Escape, and focus-exit recovery.
- **taste-skill:** this is product UI outside the skill's primary marketing scope; apply only its form-state completeness, standard affordance, and regression gates. Do not restyle the established admin surface.
- **finesse-ui:** keep product register at SPECTACLE 1 / DENSITY 8; repair the shared primitive once, preserve the flat form hierarchy, and use the established core-member adapter instead of full-directory filtering in each control.
- **Shared direction:** one interaction contract for `Select` and `MultiSelect`; an explicit chevron button toggles open/closed, selection closes single-select but intentionally keeps multi-select open, and all people controls consume the same core-member directory.
- **Protection rules:** do not alter saved contract identifiers, review approval semantics, Markdown editing, modal geometry, or unrelated dropdown consumers; do not persist browser-test selections.

### Phase 57 Root Cause

- Both selectors listen for outside `click` on `window` in the bubbling phase, but the demand-detail modal stops click propagation. Clicks on modal blank space never reach the listener.
- The searchable trigger only calls `openDropdown`; its chevron is non-interactive, so a second trigger click cannot close the list.
- The review form merges `/api/users` and contract participants without applying the existing core-member visibility boundary, allowing non-core people into the single-owner controls.

### Phase 57 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| A diagnostic `rg` used an unmatched `.env*` zsh glob | 1 | Keep follow-up searches on explicit repository paths or quote optional glob patterns |
| The sandbox denied binding Vite to `127.0.0.1:5175` with `EPERM` | 1 | Restarted the local frontend with the narrowly approved `pnpm --dir web dev` command |
| The in-app browser tab had already navigated to Chrome's generated connection-error `data:` page while the server was down; browser security policy then blocked all further navigation and DOM interaction | 1 | Did not attempt a workaround or alternate browser surface; completed Go, Svelte, production-build, Impeccable, responsive-source, and diff validation and recorded the live-browser exception explicitly |

### Phase 57 Validation Result

- Focused Go lifecycle test passes and proves `External Operator` is excluded while the configured core `Reviewer` remains available.
- `pnpm --dir web check` passes with 0 errors and 78 existing warnings; `pnpm --dir web build`, targeted Impeccable complete/layout scans, and `git diff --check` pass.
- Both selectors now share capture-phase outside-pointer, focus-exit, Escape, and explicit chevron-close behavior. Single-select closes after selection; multi-select remains open only for deliberate consecutive selection.
- The review directory is filtered at both the backend participant projection and frontend compatibility adapter. A directory-signature normalization pass clears stale non-core owners when the core directory arrives after the contract payload.

## Phase 58: Deterministic Multi-select Completion

- [x] Inspect the user's screenshot and reproduce the visual obstruction path in the shared multi-select implementation
- [x] Complete the mandatory Impeccable + taste-skill + finesse-ui review before editing frontend code
- [x] Separate the listbox from its completion controls and add an explicit, always-visible close action
- [x] Harden chevron event ordering and positioning so closing cannot be undone or hit an option underneath
- [x] Update reviewer guidance copy to match the final interaction contract
- [x] Run Svelte/build, Impeccable detection, and real-component browser validation for open/select/complete/outside/Escape states
- **Status:** complete — all automated and isolated real-component interaction gates pass; the browser session did not inherit the user's authenticated application state, so the same component was validated in a temporary local harness that was removed afterward

### Phase 58 Three-Way Review

- **Impeccable harden/product:** the screenshot shows a lifecycle failure, not a color problem. The overlay remains active after the user's selection task and covers the following controls. Completion must be explicit, keyboard reachable, and visually attached to the list.
- **taste-skill:** this dense product form is outside taste-skill's primary marketing scope. Apply only its redesign-preservation, form contrast, focus-state, cleanup, and pre-flight constraints; preserve the established Phase 41 visual system.
- **finesse-ui redesign/product:** keep SPECTACLE 1 and DENSITY 8, repair only the shared primitive, preserve the flat review form, and use a bounded option viewport plus a stable completion footer instead of restyling the surrounding contract section.
- **Shared direction:** keep consecutive multi-selection, but render the bounded option region in normal form flow so it pushes later fields down instead of covering them; add a dedicated `完成选择` action; use pointerdown only to preserve focus and block the parent trigger, then perform the single authoritative toggle on click after pointer release; outside pointer, focus exit, and Escape remain valid close paths.
- **Protection rules:** do not change reviewer values, save/approval semantics, field order, modal width, core-member filtering, or the shared single-select.
- **Validation scope:** reviewer candidates and required roles; open, select, complete, chevron close, outside click, Escape, keyboard focus, desktop and narrow modal widths.

### Phase 58 Screenshot Finding

- The option overlay spans the full review-form width and remains above the next rows after three reviewers have been selected. Pointer attempts on covered fields land on option rows, so outside-click cannot help the user until the overlay itself provides a reliable completion control.

### Phase 58 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| Toggling open during chevron `pointerdown` changed inline layout before the same pointer sequence finished; the following click hit a newly rendered option and selected `张路路` | 1 | Keep pointerdown non-mutating and use it only for focus/propagation control; toggle once on the later click event |
| After moving the list into normal flow, the absolutely positioned chevron still used the expanding wrapper as its containing block, so `top: 50%` moved it into the option list over `张路路` | 2 | Make the fixed-height trigger the chevron's positioning context with `position: relative` |

### Phase 58 Validation Result

- Actual `MultiSelect.svelte` was mounted with three preselected core members, six directory options, and representative following form fields. No API save or contract mutation occurred.
- Consecutive selection kept the list open and updated `已选 4 项`; `完成选择` closed it and restored focus to the chevron.
- Chevron close, outside click on the next field, and Escape each reduced visible listboxes to `0`.
- Open-state geometry measured `overlap=false`, a 37px separation from the following form group, chevron fully inside the trigger, and no horizontal document overflow at desktop and 480px.
- `pnpm --dir web check` passes with 0 errors and 78 existing warnings; production build, targeted Impeccable complete/layout scans, and `git diff --check` pass.
- Visual evidence: `output/ui-validation-multiselect-completion-480.png`.

## Phase 59: Unified Select Lifecycle And Close-on-select Contract

- [x] Verify that the live 5175 Vite server is serving the latest `MultiSelect.svelte` rather than a stale bundle
- [x] Audit whether single and multi selects share one component or duplicate their dismissal lifecycle
- [x] Complete the mandatory Impeccable + taste-skill + finesse-ui review before editing frontend code
- [x] Extract one shared outside-pointer/Escape lifecycle used by both `Select` and `MultiSelect`
- [x] Change multi-select option toggles to close immediately after every selection or removal
- [x] Update reviewer guidance copy and remove any wording that promises an always-open consecutive-selection flow
- [x] Run Svelte/build, Impeccable detection, and real-component browser validation with both single and multi selects
- **Status:** complete

### Phase 59 Three-Way Review

- **Impeccable harden/product:** the live module is current, so this is a contract mismatch rather than cache drift. User control wins: selecting an option must complete the immediate interaction and close the surface; outside pointer and Escape are fallback exits, not the primary completion path.
- **taste-skill:** dense admin forms remain outside its primary scope. Preserve the Phase 41 visual system and standard select affordances; do not merge distinct single/multi value contracts into a prop-heavy visual monolith.
- **finesse-ui redesign/product:** keep two thin public primitives (`Select`, `MultiSelect`) for clear type semantics, but extract their duplicated dismissal listeners into one shared lifecycle module and validate both through the same state matrix.
- **Shared direction:** multi-select closes after every option toggle and restores focus to its trigger; users reopen to add another candidate. Both public components consume one shared outside-pointer/Escape controller with identical cleanup behavior.
- **Protection rules:** do not change selected values, directory filtering, field order, save/approval behavior, modal geometry, or unrelated Select consumers.
- **Validation scope:** multi select closes after select and deselect; both types close by chevron, outside click, and Escape; listeners are removed on destroy; desktop and 480px layouts remain stable.

### Phase 59 Architecture Answer

- Before this phase, the project had two shared components, `Select.svelte` and `MultiSelect.svelte`, but each implemented its own window listeners and close lifecycle. It was public-component reuse, not a unified dropdown foundation.

### Phase 59 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| Clicking the multi-select search input opened the in-flow list during `focus`, before the pointer sequence completed; the browser selected `张路路` and closed instead of only opening | 1 | Do not mutate list layout on focus; open after the completed input click, while retaining input and ArrowDown open paths |

### Phase 59 Validation Result

- The Vite server on `127.0.0.1:5175` was confirmed to serve the edited `MultiSelect.svelte`, ruling out a stale-bundle explanation.
- An actual-component harness covered `Select.svelte` and `MultiSelect.svelte`: multi-select closed after both select and deselect; single-select closed after selection; both closed by chevron, outside pointer, and Escape with zero remaining listboxes.
- The focus/click regression was reproduced and fixed: focusing a combobox no longer mutates the in-flow layout before the originating pointer sequence completes.
- `pnpm --dir web check` passes with 0 errors and 78 existing warnings; production build, complete and layout-scoped Impeccable detection (`[]`), and `git diff --check` pass.
- The temporary harness and its invalid narrow-layout screenshot were removed. The authenticated production route was not mutated during validation; the automation browser exposed the login surface rather than the user's existing app session.

## Phase 60: Reviewer Multi-select Consecutive Selection Contract

- [x] Reproduce the close-after-every-toggle behavior in the shared `MultiSelect.svelte`
- [x] Complete and record the mandatory Impeccable + taste-skill + finesse-ui review before editing frontend code
- [x] Keep the multi-select open after selecting, deselecting, or removing a value while it is expanded
- [x] Update reviewer guidance copy to describe consecutive selection and outside-click completion
- [x] Validate option toggles, outside pointer, Escape, explicit completion, chevron close, and responsive layout
- [x] Run Svelte/build, Impeccable detection, authenticated browser validation, and diff hygiene
- **Status:** complete

### Phase 60 Three-Way Review

- **Impeccable harden/product:** a multi-select is one continuous selection session. Selecting or removing an option must update the value without ending that session; outside pointer, Escape, the chevron, and `完成选择` remain clear dismissal paths.
- **taste-skill:** this dense admin form is outside taste-skill's primary design scope, so preserve the current light-console styling and component ownership. Apply only its interaction-state, form-label, accessibility, and redesign-scope constraints.
- **finesse-ui redesign/product:** keep the shared `MultiSelect` as the single owner of multi-value behavior and preserve the shared outside-pointer/Escape lifecycle. Do not change `Select` or introduce a reviewer-only fork.
- **Shared direction:** option selection, deselection, and chip removal keep the list open; clicking outside the component closes it. Escape, `完成选择`, and the chevron also close it. The value update remains immediate and searchable consecutive selection remains available.
- **Disagreement resolved:** Phase 59 intentionally made multi-select behave like single-select by closing after every toggle. The user's current correction supersedes that contract because repeated reopen cycles violate normal multi-select operation; shared dismissal infrastructure stays, only the multi-value completion semantics change.
- **Protection rules:** do not change reviewer values, core-member filtering, save/approval behavior, field order, modal geometry, `Select.svelte`, or unrelated consumers.
- **Validation scope:** reviewer candidates and required roles; select, deselect, chip removal, search continuity, outside pointer, Escape, `完成选择`, chevron, desktop and narrow widths, authenticated demand detail.

### Phase 60 Validation Result

- Authenticated `http://localhost:5173/` DG-319 demand detail verified the real shared component without saving or approving the review contract.
- Selecting 梁志远 and 白凌云 consecutively kept one listbox visible and advanced the footer from `已选 1 项` to `已选 2 项`; clicking the form outside closed it while preserving both temporary selections.
- Deselecting an option and removing an expanded-state chip both kept the listbox visible. The chip path required a pointerdown focus guard because removing the focused button otherwise triggered focus-exit dismissal.
- Escape, `完成选择`, and the chevron each closed the reviewer list. All temporary reviewer changes were reversed before releasing the authenticated tab, and browser console errors were empty.
- Desktop and narrow screenshots show the in-flow list remains inside the detail modal with no horizontal overflow: `output/ui-validation-reviewer-multiselect-continuous-desktop.png` and `output/ui-validation-reviewer-multiselect-continuous-narrow.png`.
- `pnpm --dir web check` passes with 0 errors and 78 existing warnings; production build, targeted Impeccable detection (`[]`), and `git diff --check` pass.

### Phase 60 Pre-flight Self-grade

1. **Interaction contract:** option selection stays open, proven by one visible listbox after each of two clicks and footer counts 1 then 2.
2. **Dismissal contract:** outside pointer, Escape, `完成选择`, and chevron each produced zero visible listboxes and `aria-expanded=false`.
3. **Scope:** only `MultiSelect.svelte` interaction ownership and the reviewer helper sentence changed; `Select.svelte`, values, filtering, save behavior, and modal layout remain untouched.
4. **Responsive fit:** authenticated desktop and narrow screenshots show the in-flow list inside the modal and DOM checks report no horizontal overflow.
5. **Product-register honesty:** SPECTACLE remains 1 and no decorative visual changes were introduced; feedback is immediate, familiar, keyboard-reachable, and aligned with the existing light-console component vocabulary.

## Phase 61: Demand Detail Action Hierarchy And Draft Footer

- [x] Inspect the authenticated demand-detail action hierarchy and existing shared admin action tokens
- [x] Complete and record the mandatory Impeccable + taste-skill + finesse-ui review before editing frontend code
- [x] Move `撤销草案` from the specification body into the bottom contract action row without changing its confirmation flow
- [x] Replace the legacy purple detail-action styling with one accent primary and one neutral secondary action
- [x] Validate draft, confirmation, permission, hover/focus, desktop, and narrow states in the authenticated demand detail
- [x] Run Svelte/build, Impeccable detection, and diff hygiene
- **Status:** complete

### Phase 61 Three-Way Review

- **Impeccable product/layout:** actions that affect the same draft workflow belong in one terminal action region. Keep the destructive action visually separated on the left and progression actions on the right; confirmation stays explicit and adjacent to that footer. Use semantic color only, not the inherited purple decoration.
- **taste-skill redesign constraints:** preserve the established light admin-console palette, existing control height, radius, focus ring, and restrained visual density. Do not add icons, gradients, glow, or a new button vocabulary for a two-button correction.
- **finesse-ui redesign/product:** treat `AI 解构需求` as the primary accelerator and `调整排期` as a neutral secondary utility. Reuse the shared `wa-admin-action` component classes, then add only the scoped specificity required to prevent the legacy detail-row rule from overriding tokens.
- **Shared direction:** `撤销草案` moves beside the bottom workflow buttons, aligned left and separated from save/approve/freeze actions aligned right. The top pair becomes teal primary plus neutral secondary, with consistent height, radius, hover, focus, and responsive wrapping.
- **Disagreement resolved:** making both top actions equally teal would remove their hierarchy; making both neutral would understate the AI workbench entry. Primary/secondary treatment preserves equal availability while making the recommended action clear.
- **Component ownership:** `DemandDeliveryControl.svelte` owns draft workflow actions and withdrawal confirmation; `DemandKanban.svelte` owns the demand-detail header action group; global tokens remain unchanged.
- **Protection rules:** do not change permissions, API calls, draft/contract state transitions, confirmation copy, modal width, scheduling behavior, AI companion behavior, or unrelated button consumers.
- **Responsive behavior:** the bottom action row may wrap below 900px while preserving destructive-before-progressive reading order; the top pair wraps without full-width stretching or horizontal overflow.
- **Validation scope:** authenticated draft detail at desktop and narrow widths; top action default/hover/focus; withdrawal prompt open/cancel without confirming mutation; save/approve adjacency; zero horizontal overflow and zero console errors.

### Phase 61 Validation Result

- Authenticated DG-319 showed `AI 解构需求` as the token teal primary action (`rgb(0, 143, 150)`) and `调整排期` as the neutral secondary action; both measured 38px high with 10px radii. AI hover resolved to the stronger teal and keyboard Tab focus produced the shared teal outline on scheduling.
- The draft footer contained `撤销草案` on the left and `保存契约` plus `批准审核契约` on the right in one action row. Opening withdrawal displayed the existing alert and both confirmation choices; `继续编辑` closed it, and no withdrawal, save, approval, AI, or scheduling mutation was submitted.
- Desktop validation ran at the existing 2133x902 viewport. The responsive override requested 760x900 and the Chrome surface reported an actual 844x1000 viewport; both top and footer groups remained inside the 960px modal contract with no document horizontal overflow.
- Browser console errors were empty. The temporary viewport override was reset and the claimed application tab was returned to the flow board with the detail modal closed.
- `pnpm --dir web check` passes with 0 errors and the existing repository warnings. Production build and `git diff --check` pass.
- Full Impeccable detection still reports 15 historical findings in the large `DemandKanban.svelte`; filtering the detector output to every Phase 61 markup/style hunk returns zero targeted findings.

## Phase 62: Responses-first LLM Transport And Unified Draft Actions

- [x] Trace every Chat Completions/Responses request path and confirm the current provider-to-browser streaming boundaries
- [x] Review Sub2API, OpenAI Responses streaming, and Anthropic Messages streaming contracts
- [x] Complete and record the mandatory Impeccable + taste-skill + finesse-ui review before frontend edits
- [x] Add one Responses-first provider client with an explicit Claude Messages adapter and shared sync/stream parsing
- [x] Migrate demand specs, deconstruction, autonomous execution, telemetry review, and connection tests off direct Chat Completions payloads
- [x] Stream demand-deconstruction progress and completion to the frontend over NDJSON
- [x] Update AI configuration from Chat Completions/Responses selection to Responses/Claude Messages protocol selection
- [x] Place `撤销草案` immediately before `保存契约` in the right-aligned footer button group
- [x] Run focused Go tests, frontend checks/build, Impeccable detection, authenticated browser UI validation, local streaming validation, and diff hygiene
- **Status:** complete with a safety-scoped browser-streaming validation exception

### Phase 62 Provider Architecture

- **Default protocol:** OpenAI, Sub2API, and OpenAI-compatible gateways use `POST /v1/responses`; legacy empty or `completions` configuration is normalized to Responses instead of retaining Chat Completions behavior.
- **Native Claude protocol:** explicit `endpoint_type: messages` uses `POST /v1/messages`, `x-api-key`, `anthropic-version`, system prompt plus user messages, and Anthropic `content_block_delta` streaming. Claude models routed through Sub2API's `/v1/responses` remain on the Responses adapter.
- **Responses request contract:** send only supported fields needed by this product: `model`, `instructions`, structured `input`, optional `input_file` parts, and `stream`. Avoid forwarding gateway/client metadata such as `user` that Sub2API reports as incompatible with upstream Responses models.
- **Responses stream contract:** consume `response.output_text.delta`; accumulate deltas as the authoritative text and use the completed response only when a provider emitted no text deltas. Provider errors become typed transport errors.
- **Browser transport:** keep provider protocol details server-side. Browser-facing generation uses application NDJSON events (`status`, `provider_delta`/`markdown_delta`, `complete`, `error`) over a flushed response body and is consumed with `ReadableStream.getReader()`.
- **Compatibility boundary:** Chat Completions request generation and `choices[].message` parsing are removed from active paths; the persisted `endpoint_type` field remains only as a configuration compatibility slot for `responses` or `messages`.

### Phase 62 Three-Way UI Review

- **Impeccable product/layout:** destructive and progression actions for the same draft belong to one right-aligned terminal group. Order them by consequence and flow: `撤销草案`, `保存契约`, `批准审核契约`; preserve the explicit confirmation before deletion.
- **taste-skill redesign constraints:** maintain the existing light admin-console controls and semantic danger/primary colors. Do not add separators, icons, gradients, or a second footer zone; adjacency and restrained color are enough.
- **finesse-ui redesign/product:** protocol configuration should describe real provider behavior, not expose an obsolete Chat Completions fork. Keep a compact two-option control for Responses and native Claude Messages, with Responses selected for migrated legacy values.
- **Shared direction:** move withdrawal from the far-left slot into the existing right action cluster directly before save. In AI settings, replace `Completions` with `Claude Messages`; keep the rest of the configuration form hierarchy and responsive behavior unchanged.
- **Disagreement resolved:** a far-left destructive action created separation but broke the user's desired visual unity. Semantic danger styling and confirmation provide sufficient protection inside one compact action group.
- **Component ownership:** `internal/llm` owns provider payloads, auth headers, response parsing, and streaming; server handlers own application NDJSON/persistence; `Deconstructor.svelte` owns deconstruction stream presentation; `AIConfig.svelte` owns protocol choice; `DemandDeliveryControl.svelte` owns draft footer ordering.
- **Protection rules:** do not change authorization, prompt content, persistence transactions, attachment provenance, reviewer permissions, confirmation copy, scheduling semantics, modal geometry, or unrelated admin controls.
- **Responsive behavior:** protocol choice remains two compact controls; draft actions wrap as one right-aligned group below available width while preserving source order.
- **Validation scope:** OpenAI/Sub2API Responses sync and stream; native Claude Messages sync and stream; legacy completions config normalization; attachment file IDs; provider error frames; frontend first status/delta/complete states; draft confirmation cancel; desktop and narrow authenticated layouts.

### Phase 62 Validation Result

- `internal/llm` tests prove legacy Chat Completions URLs normalize to `/v1/responses`, Responses uses `instructions` plus structured `input`, file IDs become `input_file`, and streaming accumulates `response.output_text.delta`. Claude tests prove `/v1/messages`, `x-api-key`, `anthropic-version`, synchronous content parsing, and `content_block_delta` streaming.
- Server tests prove demand-spec streaming and demand deconstruction both consume provider SSE and emit browser NDJSON; the deconstruction stream exposes status, two provider deltas, and a typed complete result. Config, telemetry, LLM, and server packages all pass with loopback-enabled tests.
- `Deconstructor.svelte` and the schedule estimator now request `application/x-ndjson` and share one `ReadableStream` parser. Raw partial JSON is not rendered; the UI presents bounded status copy and only commits the typed `complete.result`.
- Authenticated browser validation in an isolated local session confirmed the AI settings show `Responses API（推荐）` and `Claude Messages（直连）`, with the Claude selection switching the endpoint placeholder to `/v1/messages`. No provider request or configuration save was submitted.
- The authenticated draft-detail replay was stopped after the isolated backend unexpectedly activated delay-alert background processing on copied historical data; restarting was rejected as an external-notification risk. The footer instead passed a source-order assertion (`撤销草案` < `保存契约` < `批准审核契约`), Svelte type checking, production build, and prior authenticated DG-319 layout baseline.
- Impeccable reported only historical whole-file findings in legacy `Deconstructor`/`DemandKanban` CSS, outside the Phase 62 stream/status and footer changes. `git diff --check` passes.

## Phase 63: Configuration Center Unified Glass Workbench

- [x] Load cold-start routing, task memory, project Impeccable, design-taste-frontend, finesse-ui, planning-with-files, browser, and failure-recovery guidance
- [x] Audit the current authenticated Settings routes and record the mandatory three-way review before UI edits
- [x] Remove the Settings-only KPI route while preserving top-level KPI and KPI permission governance
- [x] Centralize Settings route metadata, permissions, labels, and fallback order
- [x] Rebuild the Settings page context, workbench, version inspector, and remaining security surfaces around one shared visual system
- [x] Normalize GitLab and shared `scw-*` configuration pages without changing business workflows
- [x] Add responsive, reduced-motion, reduced-transparency, focus, loading, empty, error, success, and disabled coverage
- [x] Run type/build/diff checks, Impeccable detection, and authenticated route/breakpoint browser validation
- **Status:** complete

### Phase 63 Three-Way Review

- **Design Read:** engineering delivery governance admin · calm, precise, restrained · register=product · SOUL=5 · SPECTACLE=1 · DENSITY=7 · engine=none.
- **Impeccable:** use `compact context -> single workbench -> flat fields/tables -> action bar`; SettingsPanel owns orchestration, shell owns navigation, domain components own data and mutations. Do not add another decorative Phase skin; reduce duplicate identity, fixed-height inspector sync, side stripes, nested cards, and control drift.
- **design-taste-frontend:** this dense product UI is outside taste-skill's primary marketing scope, so apply only its redesign audit, theme/color/shape locks, card restraint, and explicit responsive collapse. Preserve the light console and existing Svelte component system.
- **finesse-ui:** product register with high density and no spectacle. Use one page shell, compact header, optional 380-400px audit utility rail, shared status/form/action/table vocabulary, and feedback-only motion.
- **Shared hierarchy:** shell -> compact translucent context/header -> one translucent workbench boundary -> transparent field rows or bounded tables -> action bar. No page hero and no repeated page title inside child components.
- **Shared component ownership:** centralize Settings section metadata and permissions; App owns route availability/fallback, FunctionalAdminShell owns nested navigation/drawer, SettingsPanel owns page composition and audit inspector, config children own only domain state and mutations.
- **Shared responsive behavior:** wide content uses `minmax(0, 1fr) + 380-400px`; at medium width the inspector flows below at natural height; below 760px use one column, 44px controls, full-width actions, and one explicit table overflow boundary. Only the shell workspace scrolls vertically except code, versions, and wide tables.
- **Shared validation scope:** all 10 remaining Settings sections; integration overview/edit/health and version empty/diff states; project/AI/context loading/empty/error/success; security tables, dialogs, dropdowns, and permission actions; authenticated desktop/1024/760/480 checks; config-only, context-only, users-only, and KPI-only permission profiles.
- **Disagreement resolved:** the user explicitly wants glass while both product skills reject decorative glass. Glass is therefore limited to page chrome, the main workbench, the version inspector, and overlays; inner content stays flat with hairlines.
- **Disagreement resolved:** card alignment means aligned outer panel tops, grids, title baselines, and action bars. It does not justify equal-height content or card-inside-card layouts.
- **Protection rules:** preserve top-level KPI, `kpi:read` entries in permission/policy surfaces, all API requests, field order, saves/tests/rollbacks, RBAC behavior, active teal accent, and unrelated dirty-tree work.

### Phase 63 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| `apply_patch` could not find an older findings anchor after the browser audit | 1 | Re-read the file tail and appended using the exact current final line instead of repeating the stale anchor |
| A targeted KPI scan referenced `web/src/components/FunctionalAdminShell.svelte`, but the component lives under `components/prototype` | 1 | Used `rg --files` to resolve the authoritative path and reran the scan successfully |
| The Chrome locator wrapper does not expose Playwright `boundingBox()` | 1 | Revalidated the switch through its accessible role plus a refreshed 480px screenshot instead of relying on an unsupported method |

### Phase 63 Validation Result

- One shared `settings-sections.ts` now owns the 10 remaining Settings sections, permission gates, labels, summaries, and fallback order. The Settings-only KPI entry is absent from App routing, shell subnavigation, Settings rendering, and fallback logic; the top-level KPI route and `kpi:read` policy semantics remain intact.
- The authenticated configuration center now uses one compact glass context header and one aligned workbench vocabulary across integration, AI, user, permission, policy, and audit pages. Glass is structural; inner rows, tables, forms, and permission branches stay flat with hairlines.
- Authenticated route audit opened all 10 remaining pages and confirmed one active page region and heading per route, no Settings KPI entry, and no load-failure state.
- Visual review passed at the existing 2133x902 viewport, 1024x900, 760x900, and 480x900. Wide layouts keep the workbench/audit rail aligned; medium and narrow layouts flow the audit below; 480px facts and fields stack without horizontal clipping. A mobile switch-size regression discovered during review was corrected and rechecked after a clean reload.
- GitLab overview and the non-mutating edit wizard were both reviewed; no save, health check, webhook install, rollback, membership, permission, or policy mutation was submitted. Browser console errors were zero.
- `pnpm -C web exec tsc -p tsconfig.app.json --pretty false`, `pnpm -C web check` (0 errors, 78 existing warnings in unrelated files), `pnpm -C web build`, targeted Impeccable detection, and `git diff --check` pass.

## Phase 64: Configuration Panel Alignment And Dense-Page Normalization

- [x] Reload project UI, planning, browser, and failure-recovery guidance
- [x] Reproduce wide-panel alignment and abnormal-height defects in the authenticated configuration center
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review
- [x] Remove height coupling and establish one shared top/bottom alignment contract for workbench plus audit rail
- [x] Normalize the system design corpus page into one clear workbench hierarchy
- [x] Normalize policy authorization into aligned builder, coverage, and decision regions
- [x] Validate all configuration routes and affected overview/edit/loading/empty/error states at desktop, 1024px, 760px, and 480px
- [x] Run TypeScript, Svelte check, production build, Impeccable detection, console review, and diff hygiene
- **Status:** complete

### Phase 64 Validation Result

- All 10 authenticated Settings routes render the expected page heading with no load-failure state and zero document horizontal overflow. GitLab, Feishu, Jira, Project, and AI engine main/audit panes share a 0px desktop top delta.
- GitLab keeps natural primary height while its desktop audit inspector is sticky and viewport-bounded. At exact 1024/760 widths the inspector flows below as a 680/700px region; its 1559px diff content scrolls locally instead of stretching the panel to 1978px.
- System corpus exposes exactly one of `资料库 / 候选审核 / 上下文预览`. The wide library grid shares one top baseline; it collapses to one column at 760/480. Candidate queues paginate at 10 items, and empty/preview states remain compact.
- Policy authorization exposes exactly one of `策略列表 / 新建策略 / 授权解释 / 授权审计`. Action inventories and the 40-row audit are bounded, allow/deny semantics are restored, and mobile tabs plus the priority control measure 44px.
- Members, permission tree, and security audit now use named, bounded data regions with sticky table headers where applicable. Their former 4536/4532/5982px panels reduce to 739/934/739px desktop workbenches while preserving every record through local scrolling.
- Fresh authenticated console validation across permission and member pages reports zero errors. No save, review, health check, rollback, permission, membership, policy, preview, or other mutation was submitted.
- `pnpm -C web exec tsc -p tsconfig.app.json --pretty false`, `pnpm -C web check` (0 errors, 78 existing warnings), `pnpm -C web build`, targeted Impeccable detection (`[]`), and `git diff --check` pass.

### Phase 64 Design Read And Protection Rules

- **Design Read:** engineering delivery governance admin · calm, precise, restrained · register=product · SOUL=5 · SPECTACLE=1 · DENSITY=8 · engine=none.
- **Anti-default:** reject equal-height cards and nested glass as a cosmetic answer to alignment. Alignment must come from shared grid tracks, natural-height content, explicit overflow ownership, and consistent section headers/action rows.
- **Protection rules:** preserve the Phase 41 light console, teal accent, all route labels, APIs, permissions, saves/tests/rollbacks, corpus decisions, policy evaluation behavior, and unrelated dirty-tree work.
- **Scope boundary:** target Settings layout ownership, system-corpus composition, policy authorization composition, and shared configuration CSS. Do not restructure global navigation or business workflows.
- **Validation scope:** all 10 remaining Settings routes, with focused screenshot review for GitLab alignment, system corpus, policy authorization, and any route previously affected by abnormal panel height.

### Phase 64 Three-Way Review

- **Impeccable:** the panel defect is a layout-ownership conflict, not a missing cosmetic override. `SettingsPanel.svelte` still mounts the Phase 41/46/49 root classes, so older absolute positioning, `height: 100%`, fixed heights, and hidden overflow compete with the unified natural-height workbench. The outer Settings grid should own only columns and the shared top baseline; the main pane stays natural height, while only the version inspector may be sticky and viewport-bounded.
- **design-taste-frontend:** preserve the restrained light console and repair the two page-local compositions. The corpus list/editor should share a deliberate grid track and collapse to one column; policy content should stop placing the wide seven-column table and audit log in equal half-width panels. Restore green/red semantic decision states instead of allowing the general teal active rule to flatten them.
- **finesse-ui:** do not add another Phase skin. Remove the legacy phase classes from the live root, keep one structural glass workbench, and use progressive disclosure for dense workflows. The corpus becomes `资料库 / 候选审核 / 上下文预览`; policy authorization becomes `策略列表 / 新建策略 / 授权解释 / 授权审计`, so only one task surface owns vertical space at a time.
- **Shared direction:** use one page context, one tabbed task switcher, one natural-height task panel, and explicit local overflow only for source lists, code, and wide tables. Outer panel tops align through one grid row; bottoms follow content rather than artificial equal heights. Inner panels stay flat with hairlines and shared tokens.
- **Disagreement resolved:** taste-skill favored the smallest CSS-only correction, while Impeccable and finesse-ui favored full progressive disclosure. Because the user specifically reports visual disorder and abnormal page length on two multi-workflow pages, use tabs as a bounded presentation refactor while preserving every existing field, API, action, result, and source order inside its task panel.
- **Component ownership:** `SettingsPanel.svelte` owns Settings orchestration and policy task tabs; `AIConfig.svelte` owns corpus task tabs; `CorpusCandidateReview.svelte` owns bounded candidate presentation; `settings-config-workbench.css` owns the shared `scw-*` surface vocabulary. No backend or route ownership changes.
- **Responsive behavior:** wide Settings keeps `minmax(0, 1fr)` plus the existing inspector rail with a shared top baseline; at 1120px the inspector flows below. Corpus uses an aligned list/editor split on wide screens and one column at 960px; all task tabs wrap without horizontal overflow. At 760px controls remain at least 44px and wide tables keep one named local scroll boundary.
- **Accessibility:** tabs use `tablist`, `tab`, `tabpanel`, `aria-selected`, and labelled panel relationships; policy choice buttons expose pressed state, the enabled control exposes switch semantics, and decision/error states retain semantic text in addition to color.
- **Validation scope:** authenticated GitLab overview/edit alignment; corpus library/candidate/preview loading, empty, error, and populated states; policy list/create/explain/audit states; desktop, 1024px, 760px, and 480px; keyboard focus, local overflow ownership, page height, console errors, TypeScript/Svelte/build, Impeccable detection, and diff hygiene.

### Phase 64 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| The prior Chrome tab binding was unavailable after the previous task finalized it (`settingsTab is not defined`) | 1 | Discard the stale binding and claim a fresh existing authenticated Chrome tab through the persistent browser connection |
| The first corpus inspection assumed both components lived directly under `web/src/components` | 1 | Resolve the authoritative files with `rg --files`; both live under `web/src/components/config` |
| Removing the legacy Phase root classes fixed runtime conflicts but increased `svelte-check` from the repository warning baseline to 522 unused-selector warnings | 1 | Restore the compatibility class names and give the unified root one stable ID that explicitly owns panel geometry and policy semantics above legacy skins |
| Narrow-view corpus validation assumed an off-canvas sidebar button had changed routes, then attempted to select a tab that was not mounted | 2 | Change target routes at desktop width, then apply the narrow viewport; guard all state-specific DOM nodes before measuring |
| Authenticated permission-route console showed `Cannot read properties of null (reading 'filter')` for a user with runtime-null memberships | 1 | Normalize non-array memberships to an empty list for both coverage counting and member-table rendering |
| Chrome `tabs.new({ url })` created `about:blank` instead of navigating | 1 | Create the tab, then call its documented `goto(url)` method explicitly before validation |
| Final handoff tried to force the original tab to `策略列表`, but that tab no longer exposed the requested tab locator | 1 | Stop changing the user's final route and finalize the already authenticated Settings tab in its current state |

## Phase 65: Governed Corpus Ingestion And Unified Review Center

- [x] Audit current corpus models, review promotion, LLM request paths, Markdown workbench, permissions, and authenticated UI states
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review before frontend edits
- [x] Add source-document persistence, version/provenance fields, ingestion APIs, and LLM-to-`CorpusCandidate` extraction
- [x] Extend candidate governance for normal auto-publish versus sensitive/global impact-preview publication
- [x] Rebuild the corpus surface around source documents, Markdown import, advanced manual entry, unified review, and impact comparison
- [x] Add focused backend/frontend tests and validate build, detector, authenticated states, responsive breakpoints, and diff hygiene
- **Status:** complete

### Phase 65 Goal

Turn the system design corpus into a governed document-ingestion workflow: preserve original Markdown sources, let the configured LLM generate traceable candidate facts, review and edit them in one center, publish ordinary facts immediately after approval, and require a separate impact-comparison publish step for sensitive or global architecture facts.

### Phase 65 Protection Rules

- Preserve the existing Phase 41 light Settings workbench, teal accent, current `ContextFact` and context-pack behavior, permissions, manual fact entry, candidate review history, and unrelated dirty-tree work.
- Original source content is immutable/versioned and remains distinguishable from LLM rewrites or supplements; no LLM output may silently overwrite source evidence.
- Pending, rejected, and impact-review candidates must never be selected by the production context-pack builder.
- Ordinary candidates may become active only through an authorized review. Sensitive/global candidates require an explicit second publish action after impact preview.
- Reuse `MarkdownWorkbench.svelte` for Markdown input and comparison; do not introduce a second editor engine or raw textarea-based Markdown workflow.
- Keep document, candidate, and fact mutations atomic where their lifecycle crosses database entities, and keep the current accepted/rejected review contract backward-compatible where practical.

### Phase 65 Validation Contract

- Focused Go tests for source import, LLM extraction, candidate decisions, sensitive/global publish gating, and context-pack exclusion
- `pnpm -C web exec tsc -p tsconfig.app.json --pretty false`
- `pnpm -C web check`
- `pnpm -C web build`
- Targeted Impeccable detection and `git diff --check`
- Authenticated browser validation for source list/import, Markdown edit/preview, candidate review/edit/reject, ordinary publish, sensitive impact comparison/publish, loading/empty/error/success, and desktop/1024/760/480 layouts

### Phase 65 Three-Way Review

- **Impeccable:** keep one clear task hierarchy and make provenance visible at the decision point. The source library should use a bounded source list beside one Markdown import/preview workbench; the review center should use one selected candidate rather than stacked review cards. Every LLM supplement must be labelled, source-linked, editable, and excluded from production until governance completes.
- **design-taste-frontend:** this surface is outside taste-skill's primary marketing scope, so defer to the existing product system. Preserve the Phase 41 light console, fixed product typography, teal accent, 8-10px radius vocabulary, explicit responsive collapse, and anti-slop constraints. Do not add a new visual system, decorative glass, equal-card grid, or animated AI spectacle.
- **finesse-ui:** register=product, SOUL=4, SPECTACLE=2, DENSITY=8. Use progressive disclosure: source documents and import are the default library task; direct fact editing becomes an advanced disclosure; the unified review center uses a compact queue plus a single workbench, with motion limited to loading and state feedback.
- **Shared hierarchy:** existing corpus context toolbar -> `资料库 / 统一审核 / 上下文预览` -> one active task panel. Library panel owns source list and import editor. Review panel owns queue, candidate metadata/content editing, source evidence, and conditional impact comparison. Preview remains the production-context verification surface.
- **Component ownership:** `AIConfig.svelte` owns the three top-level corpus tasks and advanced manual-fact disclosure; a new source-library component owns document import/list/version viewing; `CorpusCandidateReview.svelte` owns unified review and conditional publication; `MarkdownWorkbench.svelte` remains the sole Markdown editor/renderer; backend handlers own lifecycle transitions and source/fact provenance.
- **Responsive behavior:** at wide widths source/review panels use a bounded 340-380px queue plus `minmax(0, 1fr)` workbench. At <=960px they stack in source-first order. Impact comparison is two equal `minmax(0, 1fr)` panes on wide screens and one column below 900px. At <=760px tabs and actions keep 44px targets, workbench heights reduce without page-level horizontal overflow, and only source/candidate queues plus Markdown panes own local scrolling.
- **Validation scope:** authenticated source loading/empty/error/import success, file-to-Markdown loading, document selection/version metadata, advanced manual create/edit, pending candidate edit/reject/ordinary accept, sensitive/global first approval, impact before/after comparison, final publish, context fact refresh, and context-pack selection. Cover desktop, 1024px, 760px, 480px, keyboard focus, reduced motion/transparency, console errors, TypeScript/Svelte/build, focused Go tests, detector, and diff hygiene.
- **Disagreement resolved:** taste-skill favored the smallest preservation pass because dense admin workflows are out of its primary scope; Impeccable and finesse favored source-first progressive disclosure. The user's six explicit lifecycle requirements require the larger workflow refactor, but it remains bounded to the existing corpus route and shared Settings visual vocabulary.
- **Disagreement resolved:** the original request mentions file/Markdown import, while the first backend slice can normalize supported text files to Markdown. Accept `.md`, `.markdown`, and `.txt` now, preserve filename/MIME/hash, and reject unsupported binary formats rather than pretending to parse them. PDF/DOCX remain a later ingestion adapter, not a hidden lossy path.

### Phase 65 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| Combined skill reads exceeded the tool output budget and were truncated | 1 | Re-read each required skill and product reference in bounded line ranges before implementation |
| The focused LLM-ingestion test could not bind the sandbox-blocked `httptest` IPv6 loopback listener | 1 | Re-run the same focused test with approved local-loopback execution; do not weaken or remove the real provider-contract test |
| The first production-build command ran from the repository root, which has no package manifest | 1 | Route frontend commands through `pnpm -C web ...`; the corrected production build passed |
| Chrome browser-client initialization attempted to redefine a protected runtime `process` property, and Computer Use state discovery then stalled | 2 | Follow the plugin recovery path, terminate the stalled call, and use the bundled Playwright runtime with local Chrome against the same authenticated fixture stack |
| The bundled Playwright package referenced browser artifacts that were not installed | 2 | Launch the already-installed system Chrome executable instead of downloading or installing new software |
| Vite's `localhost:8080` proxy resolved to an unrelated IPv6 listener instead of the isolated IPv4 fixture backend | 1 | Intercept only the validation browser's `/api` requests and fulfill them from `127.0.0.1:8080`; leave project configuration and existing services untouched |
| Closing the validation browser while an SSE route was still in flight produced a harness-only `TargetClosedError` | 1 | Call `page.unrouteAll({ behavior: 'ignoreErrors' })` before closing the fixture context; the next three authenticated runs exited cleanly |

## Phase 66: Opaque File Handoff To LLM

- [x] Re-read the current document import, LLM transport, source persistence, and upload UI boundaries
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review before frontend edits
- [x] Verify the current Responses file-input contract against official OpenAI documentation
- [x] Add one protocol-owned file-input request path in `internal/llm`
- [x] Replace backend document-body parsing/prompt concatenation with multipart envelope validation and opaque file handoff
- [x] Persist only the structured LLM extraction as the managed document/candidate result
- [x] Remove browser `File.text()` and render explicit file-handoff versus Markdown-paste states
- [x] Add transport-boundary tests proving uploaded bytes reach the LLM adapter unchanged
- [x] Run focused Go tests, frontend checks/build, detector, authenticated browser state/breakpoint validation, and diff hygiene
- **Status:** complete

### Phase 66 Goal

Correct the Phase 65 ingestion boundary: an uploaded file is an opaque payload that the frontend sends as multipart data and the backend forwards directly through the configured LLM protocol. Neither browser nor backend may decode, normalize, convert, or concatenate the file body into a text prompt. The backend may validate metadata/size, hash opaque bytes, and persist metadata, but document Markdown, summaries, anchors, and candidates must come from the structured LLM result.

### Phase 66 Architecture Contract

- `CorpusSourceLibrary.svelte` owns file selection metadata and multipart submission; it must never call `File.text()`, `FileReader.readAsText()`, or place file bytes in `MarkdownWorkbench`.
- `MarkdownWorkbench.svelte` remains the manual paste/advanced text entry path. File handoff and pasted Markdown are mutually exclusive source modes in one import workbench.
- `context_document_handlers.go` owns authorization, multipart envelope validation, opaque byte limits/hash, ingestion lifecycle, and persistence of LLM output. It must not infer file text, convert formats, or append uploaded bytes to prompts.
- `internal/llm` owns provider-specific file blocks. Responses receives an `input_file` part; native provider adapters must either transport the file using their documented file block or return an explicit unsupported-protocol error, never silently fall back to backend parsing.
- The extraction schema must return the normalized source document Markdown plus summary and candidate array. `ContextDocument.Content` is therefore LLM-produced normalized Markdown, not an uploaded raw body.
- A failed LLM call may leave only metadata, opaque hash, and failure status. No uploaded file body is persisted by this workflow.
- Raw-byte hashing and multipart decoding are transport operations, not content parsing; they are allowed only for integrity, deduplication, and size enforcement.

### Phase 66 Three-Way Review

- **Design Read:** enterprise corpus ingestion workbench for administrators · restrained, explicit, task-first · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=8 · engine=none.
- **Impeccable:** preserve the one-workbench hierarchy and make the active input mode unambiguous. A selected file is represented by filename, MIME, size, remove/replace control, and a plain explanation that it is sent directly to the LLM; its content must not appear in the editor. Manual paste keeps the shared Markdown editor and full label/error/focus states.
- **design-taste-frontend:** this admin workflow is outside taste-skill's main scope. Apply only preservation, theme/shape/color locks, copy audit, accessibility, and explicit single-column collapse. Do not introduce decorative AI visuals or another component vocabulary.
- **finesse-ui:** product register, high density, feedback-only motion. Treat file handoff and Markdown paste as progressive-disclosure source modes with complete default, selected, submitting, success, failure, and disabled states; keep metadata flat and avoid nested cards.
- **Shared direction:** retain `资料库 / 统一审核 / 上下文预览`; change only the import workbench. Use one source-mode switch, one metadata form, one mode-owned input surface, and one terminal action row. File mode shows transport metadata; paste mode shows `MarkdownWorkbench`.
- **Disagreement resolved:** Phase 65 allowed browser/backend text normalization for `.md/.txt`; the user's correction supersedes that decision. Even text-native files remain opaque and go to the LLM file-input block. The Markdown editor applies only to content intentionally pasted or manually revised by a person.
- **Component ownership:** frontend owns mode/selection only; server owns transport envelope and ingestion lifecycle; `internal/llm` owns file encoding/protocol payload; the LLM owns content extraction; persistence owns only the returned structured result.
- **Responsive behavior:** source mode controls and file metadata wrap without horizontal overflow; at <=760px controls remain at least 44px and actions become full width. No new fixed height is introduced.
- **Validation scope:** Responses payload contains exact uploaded bytes in `input_file`; no backend prompt contains those bytes; unsupported provider behavior is explicit; multipart missing/oversize/duplicate/failure/success; paste-mode compatibility; authenticated file selected/submitting/success/failure and paste edit/preview at desktop, 1024px, 760px, and 480px.

### Phase 66 Protection Rules

- Preserve permissions, candidate review/publication gates, source-to-candidate provenance, context-pack active-fact boundary, advanced manual fact entry, Settings layout, teal accent, and unrelated dirty-tree work.
- Do not save uploaded file bytes in the database or filesystem, and do not submit a live external LLM request during validation.
- Preserve backward compatibility for intentional Markdown paste clients where practical, but never interpret a file upload as pasted text.

### Phase 66 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| Combined required-skill reads exceeded the output budget | 1 | Re-read the missing skill ranges and product references in separate bounded calls before the review |
| `codex mcp list` failed because the packaged CLI binary path does not exist | 1 | Record the toolchain failure and use the official OpenAI-domain documentation fallback required by the OpenAI docs skill; do not retry the same broken binary path |
| Focused Go tests attempted to use the sandbox-blocked user build cache | 1 | Re-run with isolated `GOCACHE` and `GOMODCACHE` paths under `/tmp`, matching the repository's established validation path |
| The fresh `/tmp` module cache tried to download already-installed dependencies, but sandbox DNS blocked `goproxy.cn` | 1 | Keep only `GOCACHE` isolated and reuse the existing read-only default module cache; avoid a network download |
| Browser control bootstrap failed with `Cannot redefine property: process` before an agent session was created, including after a clean runtime reset | 2 | Record the plugin failure and use the repository's established isolated Playwright plus installed Chrome fallback against local authenticated fixtures; do not touch production data or call a live LLM |
| The first full Go run exposed literal `\\n` data in the manual-Markdown fixture and an invalid empty multipart fixture | 1 | Encode real JSON newlines and construct a valid metadata-only multipart body before asserting the missing-file branch; rerun the focused and full suites |
| The first browser harness run treated Chrome's expected failed-resource console entry for the intentional 502 import fixture as an unexpected application error | 1 | Assert exactly one expected 502 network entry and continue to require zero other console/page errors; rerun every state and breakpoint |

## Phase 67: Shared Import Select And Live Markdown Composition

- [x] Audit the import scope control, shared Select contract, and current MarkdownWorkbench modes
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review before frontend edits
- [x] Replace the native import scope control with the shared Select component
- [x] Add a Typora-like live Markdown mode inside the existing CodeMirror workbench
- [x] Make corpus Markdown import default to live composition while preserving source, split, and reading modes
- [x] Run TypeScript/Svelte/build checks, Impeccable detection, authenticated browser state/breakpoint validation, and diff hygiene
- **Status:** complete

### Phase 67 Goal

Remove the last browser-native control from the document-import form and make Markdown editing immediately legible without a preview action. The editor must keep canonical Markdown as its only stored value while formatting inactive lines in place and revealing syntax on the active editing line.

### Phase 67 Three-Way Review

- **Design Read:** enterprise corpus-import workbench for administrators · restrained light console · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=8 · engine=none.
- **Impeccable:** `CorpusSourceLibrary.svelte` should own only the selected scope value and option data; the shared `Select.svelte` must own combobox visuals, keyboard interaction, placement, focus, disabled state, and narrow-screen target sizing. Markdown remains one editor surface rather than a source editor plus a second live editor.
- **design-taste-frontend:** dashboards and code editors are outside its primary marketing scope, so preserve information architecture, typography, teal accent, light-console palette, and radius system. Extend CodeMirror through its official decoration/theme APIs instead of introducing a `contenteditable` editor or a new visual vocabulary.
- **finesse-ui:** treat this as a targeted product-UI correction, not a page redesign. The import form reuses the shared primitive; Markdown becomes a live-composition surface with feedback-only state changes and no decorative animation.
- **Shared hierarchy:** import metadata -> one mode-owned input surface -> terminal import action. The scope field uses the existing shared Select. Markdown paste opens in `即时排版`; `源码 / 分屏 / 阅读` remain explicit fallback views but are no longer required to see formatted content.
- **Component ownership:** `CorpusSourceLibrary.svelte` owns scope state, options, and the global-scope side effect; `Select.svelte` owns dropdown behavior and styling; `MarkdownWorkbench.svelte` owns canonical Markdown, CodeMirror decorations, live/source/split/read view selection, and change events.
- **Live-editing contract:** the active line exposes raw Markdown syntax for precise editing. Inactive lines hide presentation markers and render heading, emphasis, inline code, links, quotations, lists, and tasks in place. Switching to source must reveal the unchanged raw Markdown; no HTML-to-Markdown conversion is permitted.
- **Responsive behavior:** the shared Select keeps its adaptive overlay and 44px narrow target; the Markdown toolbar may wrap but must not create document-level horizontal overflow. The existing single-column import collapse remains unchanged.
- **Validation scope:** dropdown open/select, keyboard navigation, Escape/outside close, disabled/global side effect at desktop and 480px; live heading/emphasis/code/link/quote/list/task formatting, active-line syntax reveal, raw-Markdown preservation, source/split/read fallbacks, typing updates, and no console errors at 1440/1024/760/480.
- **Disagreement resolved:** a true HTML `contenteditable` surface would look more WYSIWYG but introduces lossy bidirectional conversion and a second editor ownership model. The selected CodeMirror WYSIWYM approach delivers Typora-like immediate formatting while preserving Markdown exactly.

### Phase 67 Protection Rules

- Preserve the Phase 66 opaque-file boundary, multipart submission, import metadata, scope semantics, permissions, candidate lifecycle, and unrelated dirty-tree work.
- Do not add a Markdown editor dependency or change stored/API content from Markdown to HTML.
- Do not redesign the surrounding corpus library, change the established light-console palette, or remove the explicit source/split/read fallbacks.

### Phase 67 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| The first finesse redesign reference read used a nonexistent `references/redesign.md` path | 1 | Read the actual `references/redesign-mode.md` file completely and use its targeted-redesign contract |
| The first Svelte check found the visually-hidden preview button CSS block missing its closing brace after the live-mode patch | 1 | Restore the exact block closure, log the failure through the self-improvement workflow, and rerun the full frontend check |
| The second Svelte check rejected a TypeScript constructor parameter property in the CodeMirror task widget | 1 | Replace `private checked` with an ordinary typed class field compatible with the repository's Svelte preprocessing configuration |
| In-app browser bootstrap repeated the known `Cannot redefine property: process` runtime conflict before creating a session | 1 | Stop retrying the same integration failure and use the established system-Chrome plus isolated local fixture fallback |
| The sandbox denied the first local Vite preview bind on `127.0.0.1:4173` | 1 | Re-run the same bounded local preview command with the required loopback permission |
| The first Playwright validation used a strict text locator that matched both the import scope label and the advanced manual scope label | 1 | Wait on the unique shared-control ID `#corpus-import-scope` and rerun the unchanged interaction assertions |

## Phase 68: Corpus Approval Information Architecture Refactor

- [x] Audit the screenshot, candidate queue, metadata form, source evidence, proposal editor, and impact-review states
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review before frontend edits
- [x] Group the review queue by source document and remove repeated row-level source metadata
- [x] Make one candidate proposal the primary ordinary-review surface and move immutable source evidence behind disclosure
- [x] Preserve a true before/after comparison only for high-sensitivity or global-architecture impact review
- [x] Validate queue grouping, decision states, responsive breakpoints, keyboard/focus, detector, checks/build, and diff hygiene
- **Status:** complete

### Phase 68 Goal

Turn the visually repetitive unified-review page into a source-batch review workbench. Shared document/version/status metadata appears once per source group, each candidate row carries only the information needed to navigate, and ordinary review focuses on one editable candidate instead of rendering two competing full documents. No candidate record is deleted or silently hidden.

### Phase 68 Three-Way Review

- **Design Read:** enterprise corpus-governance approval workbench · calm, precise, evidence-first · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=8 · engine=none.
- **Screenshot audit:** the queue repeats status, type, scope, timestamp, source identity, and two-line summaries for every candidate from the same document. The detail pane then repeats source title/locator in metadata and renders source plus proposal as two equal 500px workbenches. Decision target, provenance, editable content, and publication gate therefore compete at the same visual weight.
- **Impeccable:** use one clear task surface and progressive disclosure. Group candidates by immutable source document/version; render shared provenance once in a group header; make compact candidate rows scannable; keep the proposed candidate as the primary review body; expose source evidence on demand; retain explicit loading, empty, error, selected, focus, disabled, and decision states.
- **design-taste-frontend:** this dense admin/code-editor surface is outside the skill's primary marketing scope, so apply preservation rules only. Keep the established light console, teal accent, typography, radius scale, route hierarchy, shared controls, and Markdown engine. Do not add a new palette, decorative glass layer, card grid, font, or motion system.
- **finesse-ui:** use a targeted product redesign with high density and feedback-only state changes. Progressive disclosure should separate routine approval from forensic evidence, while the final action row remains stable and unambiguous.
- **Shared hierarchy:** queue summary and source batches -> selected candidate decision header -> compact editable metadata -> one proposed Markdown workbench -> collapsed source evidence -> review note and terminal action bar. In `impact_review`, replace the ordinary body with the required current-context versus proposed-candidate comparison and retain immutable source evidence as a separate disclosure.
- **Component ownership:** `CorpusCandidateReview.svelte` owns source grouping, selected candidate state, disclosure, lifecycle-specific composition, and actions. `MarkdownWorkbench.svelte` continues to own editing/rendering. Backend candidates remain the source of truth; this phase changes presentation, not candidate persistence or lifecycle semantics.
- **Responsive behavior:** wide layout uses a bounded 300-340px grouped queue plus one fluid workbench. At 960px the queue stacks above detail and remains locally bounded. At 760px metadata and actions become one column with 44px targets and no document-level horizontal overflow.
- **Validation scope:** multiple source batches and many candidates, source metadata rendered once per group, candidate selection, ordinary editable proposal, source disclosure, reject, ordinary accept/publish, sensitive/global transition, impact comparison/final publish, loading/empty/error, 1440/1024/760/480, keyboard focus, no console errors, TypeScript/Svelte/build, Impeccable detection, and diff hygiene.
- **Disagreement resolved:** a full side-by-side comparison is valuable for high-sensitivity/global impact confirmation, but creates redundant noise during ordinary first-pass review. Use one proposal plus on-demand source evidence for ordinary review, and preserve full comparison only in the second-stage impact state.

### Phase 68 Protection Rules

- Preserve every candidate returned by the API; grouping must never deduplicate, filter, merge, or suppress records.
- Preserve permissions, review/publish endpoints, normal versus impact-review gates, immutable provenance, Markdown canonical value, and unrelated dirty-tree work.
- Reuse existing Button, Select, and MarkdownWorkbench components; do not add a frontend dependency or change backend schemas unless runtime evidence proves presentation grouping is insufficient.

### Phase 68 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| The first Vite preview bind failed with sandbox `EPERM` on `127.0.0.1:4173` | 1 | Re-run the same bounded local-only preview with the required loopback approval; preserve the established validation path |
| The first source-library preview fixture used `/api/context-documents`, while the live component correctly calls `/api/context/documents` | 1 | Correct the fixture route, rebuild, reload, and confirm the authenticated library and review states without an authorization leak |
| The first impact fixture exposed a duplicated candidate heading because `after_markdown` prepended title and summary to content that already contained the title | 1 | Render the exact `draft.content` that is persisted to `ContextFact.Content`; keep title and summary in the surrounding governance metadata |
| A failed queue request displayed both the error alert and the successful-empty state | 1 | Gate the empty state on `!error`, so failure and successful emptiness have one unambiguous meaning each |

## Phase 69: Transient Corpus Decision Toasts

- [x] Audit the candidate-review success channel and existing shared feedback primitives
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review before frontend edits
- [x] Add one application-level upper-right toast host with timed and manual dismissal
- [x] Route every corpus decision confirmation out of the review container and into the toast host
- [x] Preserve contextual validation and network errors inside the review workbench
- [x] Run checks/build, Impeccable detection, authenticated browser state/breakpoint validation, and diff hygiene
- **Status:** complete

### Phase 69 Goal

Treat short-lived corpus decision confirmations as global operation feedback rather than review content. Success notices such as “影响审核已确认，候选已正式发布。” must appear in the upper-right notification layer, dismiss automatically, remain manually dismissible, and never occupy space inside the unified-review container.

### Phase 69 Three-Way Review

- **Design Read:** enterprise corpus-governance workbench for administrators · restrained light console · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=8 · motion=feedback only.
- **Impeccable:** separate feedback by recovery value. Validation and request failures remain contextual with `role="alert"`; completed decisions move to one global `aria-live="polite"` toast region. The toast must not steal focus, must expose a close action, and must honor reduced motion and narrow safe-area insets.
- **design-taste-frontend:** dense admin UI is outside the skill's primary marketing scope, so apply preservation rules only. Keep the established light theme, teal semantic accent, type/radius vocabulary, copy, and route hierarchy. Do not introduce a decorative notification system, new palette, or dependency.
- **finesse-ui:** product success uses a 3-5 second toast. Motion is justified only as action feedback, limited to opacity/translation, and disabled for reduced-motion users. Reuse the existing shared `Alert.svelte` presentation instead of creating a second alert vocabulary.
- **Shared hierarchy:** candidate queue and workbench remain unchanged. Contextual error -> review container. Completed decision -> application overlay layer. The latest notices may stack up to a small bounded count without shifting page layout.
- **Component ownership:** a shared toast store owns queueing, duration, and dismissal; a shared toast host owns the upper-right overlay, responsive bounds, accessibility, and transition; `CorpusCandidateReview.svelte` only publishes success events. Production `App.svelte` and the isolated settings preview each mount one host.
- **Responsive behavior:** desktop uses a bounded 420px upper-right column; narrow screens use equal 12px side insets and full available width. Close targets remain at least 44px on touch layouts, with no horizontal overflow.
- **Validation scope:** reject, ordinary publish, sensitive first approval, impact publish, automatic dismissal, manual dismissal, absence of in-container success markup, persistence of contextual errors, reduced motion, and 1440/1024/760/480 layouts with no console errors.
- **Disagreement resolved:** a candidate-local fixed toast would minimize file count but would duplicate behavior and risk being trapped by workbench stacking contexts. Use one shared application-level host so transient feedback has a single owner and predictable viewport placement.

### Phase 69 Protection Rules

- Preserve candidate lifecycle semantics, endpoints, permissions, queue selection, impact-preview requirements, contextual error copy, and unrelated dirty-tree work.
- Do not convert recoverable errors into transient toasts, and do not persist success copy after navigation or reload.
- Do not add a dependency, change the global palette, or modify the persistent notification inbox in the top bar.

### Phase 69 Errors

| Error | Attempt | Resolution |
|-------|---------|------------|
| The first finesse reference read used the skill root instead of its `references/` directory | 1 | Locate the actual skill tree, read `references/product-ui.md` and `references/redesign-mode.md`, and keep the correction in the active plan |
| The first SettingsPanel inspection used the obsolete `components/config/SettingsPanel.svelte` path | 1 | Resolve the live file with `rg --files` and inspect `web/src/components/SettingsPanel.svelte` before choosing the toast host boundary |
| The sandbox denied the first local Vite preview bind on `127.0.0.1:4173` | 1 | Re-run the same bounded local preview with approved loopback access, matching the repository validation path |
| The broad Impeccable scan found legacy side-tab and gradient-text warnings in unrelated `App.svelte` CSS | 1 | Preserve unrelated dirty-tree UI, run the required targeted scan on the changed notification/review surfaces, and record its clean `[]` result separately |
| The first attempt to create an in-app browser tab used an unsupported `browser.tabs.open` method | 1 | Read the selected browser's complete API documentation and use `browser.tabs.new()` followed by `tab.goto()` |
| The first selector-driven auto-dismiss wait hit the browser runtime's shorter selector deadline | 1 | Take a fresh snapshot, then verify the known 4.5-second timer with one bounded 4.8-second wait followed immediately by a specific zero-toast assertion |

## Phase 70: Impact Review Reading Flow And Panel Alignment

- [x] Audit the nested impact-comparison columns and the outer queue/workbench height contract
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review before frontend edits
- [x] Convert impact comparison from side-by-side panes into one ordered reading flow
- [x] Give the desktop queue and workbench one shared bounded height and explicit scroll ownership
- [x] Preserve natural stacked flow and touch targets below the existing responsive breakpoint
- [x] Run checks/build, Impeccable detection, authenticated browser state/breakpoint validation, and diff hygiene
- **Status:** complete

### Phase 70 Goal

Make the second-stage impact review read as one coherent decision flow instead of another nested two-column screen. Keep the desktop queue/detail relationship, but make both outer panels start and end on the same visual baseline with predictable internal scrolling; on narrower screens, return to natural document flow.

### Phase 70 Three-Way Review

- **Design Read:** enterprise corpus-governance approval workbench for administrators · calm, precise, sequential · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=8 · motion=feedback only.
- **Current-state audit:** the desktop screen contains two column systems at once. The outer queue/detail grid is a valid navigation-to-task relationship, but `comparison-grid` creates a second side-by-side reading decision inside the detail pane. The queue list alone is capped at `min(70vh, 680px)`, while the workbench grows with content, so their visible bottoms cannot align.
- **Impeccable:** preserve one clear hierarchy. Impact confirmation is serial: understand the currently active context, then inspect the proposed result, then record a decision. Render those comparison panes vertically. Give the outer queue and workbench one shared bounded desktop height, keep their headers/actions stable, and assign scrolling to the queue list and workbench body rather than stretching the page with blank space.
- **design-taste-frontend:** this dense admin/code-editor surface is outside its primary marketing scope. Apply preservation rules only: retain the light console, teal accent, typography, radius vocabulary, route/IA, and explicit mobile collapse. Add no decorative cards, palette, font, imagery, or motion system.
- **finesse-ui:** use a targeted product redesign, not a page rebuild. The two outer columns remain because they express queue navigation and selected work; the inner comparison becomes one ordered high-density flow. Use one shared height token and named overflow regions, with feedback-only interaction.
- **Shared hierarchy:** grouped review queue -> selected-candidate header -> scrollable review body -> terminal action row. In impact review, body order is impact notice -> current active context -> proposed result -> immutable evidence -> review note.
- **Component ownership:** `CorpusCandidateReview.svelte` owns the outer panel geometry, scroll boundaries, lifecycle composition, and comparison ordering. `MarkdownWorkbench.svelte` continues to own Markdown rendering and its own read/edit semantics; endpoints and lifecycle state remain unchanged.
- **Responsive behavior:** above 960px both outer panels use one bounded viewport-relative height and align top/bottom. At 960px and below the panels stack, fixed height is removed, the queue stays locally capped, the workbench returns to natural height, and comparison remains single-column. At 760px actions and form controls keep 44px targets with no document-level horizontal overflow.
- **Validation scope:** ordinary and impact candidates at 1440px and 1024px; impact pane ordering; queue/workbench top and bottom deltas; queue-list/workbench-body scroll ownership; stacked behavior at 760px and 480px; source disclosure, actions, focus, no horizontal overflow, and no console errors.
- **Disagreement resolved:** Phase 68 retained side-by-side comparison for impact review. The user's correction supersedes that layout choice: the evidentiary comparison remains complete, but changes to a vertical before/after reading sequence. Equal height is achieved with a shared bounded desktop panel, not by adding blank padding or letting the queue grow to the full document height.

### Phase 70 Protection Rules

- Preserve the outer desktop queue/detail information architecture, candidate grouping, selection, permissions, endpoints, normal versus impact publication gates, immutable provenance, Markdown values, and transient toast behavior.
- Do not introduce a new component library, change the palette or global shell, add decorative motion, or modify unrelated dirty-tree work.
- Do not force fixed panel heights on stacked narrow layouts or hide content; all bounded desktop content must remain reachable by keyboard and scrolling.

### Phase 70 Corrections And Errors

| Error or correction | Attempt | Resolution |
|-------|---------|------------|
| The Phase 68 impact state still used an inner side-by-side comparison and independently capped only the left queue | 1 | Treat the impact decision as a vertical reading sequence and replace unrelated height rules with one shared desktop panel contract before implementation |
| A combined design-taste skill read exceeded the tool output budget | 1 | Re-read the required ranges in bounded chunks and use only its preservation guidance because it explicitly defers dense admin/code-editor UI to product patterns |
| The first Phase 70 planning patch included an empty update hunk | 1 | Remove the invalid empty hunk and apply the task-plan append as one exact patch before editing frontend code |
| The sandbox denied the first local Vite preview bind on `127.0.0.1:4173` | 1 | Re-run the same bounded loopback-only preview with the required permission and continue with the established authenticated fixture path |
| The in-app browser does not support `networkidle` for `waitForLoadState` | 1 | Use a fresh DOM snapshot as the authoritative post-navigation readiness signal instead of repeating the unsupported wait state |
| The first 1440px in-app screenshot timed out in `Page.captureScreenshot` | 1 | Continue with DOM geometry and console validation, then retry a smaller responsive viewport capture instead of repeating the same full-width request |
| Applying the Chrome viewport override while a second validation window was selected left the claimed target tab at 480px | 1 | Close the extra tab, reapply the override to the intended claimed tab, and verify the browser-reported 1024x900 content viewport before measuring geometry |
| A final Chrome console re-check used the in-app browser-only `tab.console.find` surface | 1 | Keep the already completed Chrome console inspection as the authoritative result; do not repeat an unsupported cross-browser API |

## Phase 71: Workspace-Bounded Toast Positioning

- [x] Audit the toast mount, containing block, z-index, and main-content height contract
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review before frontend edits
- [x] Move the production and preview toast hosts into their main-content stages
- [x] Bound toast width and height to the stage while preserving auto/manual dismissal and responsive close targets
- [x] Run checks/build, Impeccable detection, authenticated browser state/breakpoint validation, and diff hygiene
- **Status:** complete

### Phase 71 Goal

Keep transient operation feedback visible at the upper-right of the active workspace without allowing it to enter the global Header, cover notification/profile controls, or extend beyond the main-content viewport. Preserve the existing toast queue, copy, duration, accessibility, and feedback styling.

### Phase 71 Three-Way Review

- **Design Read:** enterprise configuration and corpus-governance console for administrators · calm, precise, task-local feedback · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=8 · motion=feedback only.
- **Current-state audit:** `App.svelte` mounts `ToastHost` before the authentication/shell branches, while `.toast-region` is `position: fixed` at viewport top/right with fallback `z-index: 1400`. Its containing block is the viewport, so the 420px notice stack deterministically overlaps the 68px desktop or 64px mobile Header and its user controls. The isolated settings preview repeats the same ownership error above its 58px topbar.
- **Impeccable layout assessment:** the root cause is component ownership and positioning reference, not a smaller `top` offset. `FunctionalAdminShell` must own a `workspace-stage` after the Header and optional maintenance banner; that stage becomes the containing block and clipping boundary while `.workspace-frame` remains the only page scroll owner. Toast height must be capped by stage pixels, not only by the three-notice queue limit.
- **Impeccable mechanical scan:** the layout detector returned `[]`, but source geometry still proves the bug. Project spacing tokens are `4/8/12/16/20/24/32/40/48px`; Toast spacing is on-scale. The scan also found no definition for `--wa-layer-toast`, so the global fallback `1400` currently outranks Header `50`, popovers `80`, backdrop `120`, and mobile rail `130`.
- **design-taste-frontend:** this dense admin surface is outside its primary marketing scope. Apply preserve-redesign rules only: keep the established light console, teal accent, Alert vocabulary, Header, IA, radius/type system, copy, and feedback motion. Do not redesign the toast card or introduce new dependencies.
- **finesse-ui:** use a targeted product redesign. The global store continues to own queueing and timers, but the Shell owns the visual overlay layer. Establish a local stacking context with a low stage-local layer; cap width/height against the stage; preserve responsive 44px close targets and reduced-motion behavior.
- **Shared hierarchy:** global Header and profile/notification controls -> optional maintenance banner -> workspace stage -> workspace content + stage-local toast overlay. The toast is visually global to the active task, not global to the browser viewport.
- **Component ownership:** `FunctionalAdminShell.svelte` owns the production `ToastHost` mount and workspace stage; `SettingsConfigPreview.svelte` owns an equivalent preview content stage; `ToastHost.svelte` owns stage-relative overlay geometry; `App.svelte` no longer mounts an unconditional viewport host. `FunctionalWorkspace.svelte`, toast store, Alert, and settings page components remain unchanged.
- **Responsive behavior:** desktop notices align to the workspace gutter below Header. At `<=860px` the stage still occupies the remaining shell height and uses its compact gutter. At `<=760px` notices fill the available stage width with safe side insets and retain 44px close targets. Maximum block size always equals stage height minus its insets.
- **Validation scope:** authenticated settings toast at desktop, collapsed rail, maintenance-banner state, 1024px, 860px, 760px, 480px, and minimum supported width; assert toast top is at or below stage top, toast bottom never exceeds stage bottom, Header/profile intersection is zero, workspace horizontal overflow is zero, close/auto-dismiss still work, and console has no new errors.
- **Disagreement resolved:** Phase 69 described the host as application-level and mounted it above the Shell. Keep application-level feedback semantics and the shared store, but move visual containment into the Shell because viewport-level positioning conflicts with the established `Header + workspace` ownership contract.

### Phase 71 Protection Rules

- Preserve toast text, three-notice bound, 4.5-second default timer, manual close, `aria-live`, Alert presentation, reduced-motion/reduced-transparency handling, candidate lifecycle, endpoints, and unrelated dirty-tree work.
- Do not hardcode a Header-height offset, because the maintenance banner and responsive Header change the workspace start. Do not add another scroll owner for page content.
- Do not change Header, profile, notification inbox, navigation, route structure, palette, or global shell styling beyond the new workspace-stage containment contract.

### Phase 71 Corrections And Errors

| Error or correction | Attempt | Resolution |
|-------|---------|------------|
| Phase 69 mounted the shared host at the viewport root, so its fixed 1400 layer covered Header controls | 1 | Keep the shared store but move the visual host into a stage-local containing block below Header and cap it to that block |
| The first planning-skill path used the Codex skills root, but this installation lives under `.agents/skills` | 1 | Read `/Users/eddie/.agents/skills/planning-with-files/SKILL.md` and continue with the existing project planning files |
| The Impeccable layout and finesse reference files were first combined into one read that truncated the middle | 1 | Re-read the required product and redesign references separately to EOF before deciding or editing |
| The first multi-file planning append expected a Phase 70 status line in `findings.md`, where that progress-only line does not exist | 1 | Re-read each file tail and apply exact per-file append contexts instead of reusing one anchor across the planning files |
| The browser's guarded page evaluator rejects dynamic module loading and DOM script creation | 1 | Trigger toast through the existing candidate-review UI instead of injecting application modules or page scripts |
| The first toast wait used the unsupported `waitForSelector`, then a locator wait outlived the 4.5-second toast | 1 | Re-trigger a fresh ordinary publish and use one bounded 120ms wait followed immediately by geometry inspection |
| The first preview-stage geometry pass showed the toast correctly bounded but already above the viewport because the preview document itself had scrolled | 1 | Make the isolated preview mirror production ownership: fixed-height page, fixed topbar/tabs, and a scrolling content host inside the bounded stage |
| Opening a second Chrome validation tab created another default-width window instead of inheriting the selected 760px viewport | 1 | Close the extra tab and navigate the already measured 760px claimed tab to the preview route before triggering the real toast |
| The selected browser tab does not expose the previously assumed `console.find` API | 1 | Do not claim a console-log sweep; rely on successful interactions/build checks and report this narrow runtime-observability gap honestly |
| A final 480px screenshot tried the ordinary-publish action after the fixture queue had advanced to a different lifecycle state | 1 | Keep the already captured 480px geometry and dismissal evidence; do not repeat an action that no longer exists in the current fixture state |

## Phase 72: Markdown, Decision Timeline, Palette, And Flow-Board Detail Redesign

- [x] Locate the live Markdown, dropdown, decision-timeline, palette-token, and flow-board detail owners
- [x] Complete the isolated Impeccable layout assessment and mechanical pre-scan
- [x] Record the full Impeccable + design-taste-frontend + finesse-ui + project UI-system agreement before frontend edits
- [x] Make the Markdown workbench default to the single immediate-layout mode and hide scrolling dropdown rails
- [x] Recompose the decision-event timeline around decision evidence and recency
- [x] Map the requested source palette to restrained semantic product roles
- [x] Rebuild the flow-board demand detail into a bounded, responsive, single-scroll-owner workbench
- [x] Run checks/build, Impeccable and Finesse scans, authenticated browser state/breakpoint validation, contrast checks, and diff hygiene
- **Status:** complete

### Phase 72 Goal

Remove redundant view controls and visible inner scroll rails, strengthen the strongest-brain decision history as an evidence ledger, introduce the requested colors through one restrained semantic token system, and turn the flow-board click detail from a narrow nested-scroll document into a bounded task workbench with clear summary, actions, specification, and progressive disclosure.

### Phase 72 Initial Audit

- **Design Read:** high-density enterprise planning and decision-governance console for operators · precise, composed, evidence-first · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=8 · motion=feedback only.
- **Screenshot finding:** the flow-board detail is a centered narrow column inside a wide backdrop, has both a modal scrollbar and a large Markdown-region scrollbar, repeats bordered boxes for simple metadata, exposes the empty shadow-task region at full width, and makes the specification dominate the first screen before the task context and actions have established a stable reading order.
- **Palette intent:** use `#018b8d` as the dominant product accent, reserve `#6ecc54` for success, `#d34947`/`#c8161d` for danger or blocking states, `#eb5c20` for warning, `#002fa7`/`#0d3a69` for information and evidence, and `#71e2d1` for low-chroma accent surfaces. Keep `#470125` and `#492d22` for rare semantic categories only when their meaning is explicit. Do not display all colors simultaneously.
- **Protection rules:** preserve task data, actions, lifecycle semantics, endpoints, route/navigation labels, Markdown content and autosave behavior, Header/shell ownership, existing accessibility hooks, and unrelated dirty-tree work. No new design-system dependency and no decorative animation.

### Phase 72 Corrections And Errors

| Error or correction | Attempt | Resolution |
|-------|---------|------------|
| A combined multi-skill read exceeded the tool output budget and truncated the design-taste content | 1 | Re-read the design-taste skill in bounded ranges to EOF before auditing or editing frontend code |
| The first palette token search began with `--wa-`, so `rg` parsed the pattern as an option | 1 | Re-run the search with `rg -n -- '<pattern>'` and keep the other successful component-owner discoveries |

### Phase 72 Mechanical Pre-Scan

- Impeccable layout detector: exit `0`, raw result `[]`.
- Arbitrary Tailwind spacing/z-index scan: no matches.
- Strict CSS numeric pass: 324 pre-existing spacing declarations outside the documented scale and 36 explicit z-index declarations across the seven broad target files.
- Deliberate exception: do not turn this scoped redesign into a full-file legacy spacing or stacking rewrite. All new/directly changed layout uses semantic spacing tokens and existing overlay ownership; unrelated legacy declarations remain preserved and are covered by browser regression checks.

### Phase 72 Mandatory UI Review Agreement

- **Design Read:** enterprise planning, governance, and strongest-brain workbench for operators · evidence-first, restrained, and task-oriented · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=8 · motion=feedback only. `design-taste-frontend` is outside its dashboard focus, so its contribution is preservation/audit discipline rather than a marketing aesthetic.
- **Impeccable:** reduce simultaneous choices and nested containers. Markdown exposes one default live mode; dropdown lists keep scrolling but hide the rail; demand detail gets one stable header plus one body scroll owner; repeated fact cards become a flat definition grid; color is semantic and restricted. New spacing uses the existing 4/8/12/16/20/24/32/48px tokens.
- **design-taste-frontend:** treat this as preserve-redesign. Keep routes, labels, copy voice, system sans, IA, accessibility, analytics-facing actions, and the established light console. Do not add imagery, a new library, a new font, decorative glass, gradients, motion, or a second design system. The dense dashboard defers to product UI patterns.
- **finesse-ui:** targeted product redesign with SPECTACLE=1 and DENSITY=8. Use progressive disclosure, familiar controls, one action variant per intent, explicit loading/empty/error states, stable modal ownership, 44px narrow targets, and feedback-only transitions. No nested modal and no page-level animation.
- **project UI system:** keep one sans family, fixed product type sizes, 4px rhythm, 10px control/14px panel radius vocabulary, restrained hairlines, and semantic tokens. The supplied colors are a source library, not a rainbow requirement.
- **Shared hierarchy:** Markdown document identity -> one current mode -> content; dropdown trigger -> scrollable hidden-rail listbox; strongest-brain metrics -> stage filters -> selection list left -> selected-item inspector right -> contextual decision evidence -> intervention; demand detail header -> summary/facts/actions -> shadow-task disclosure -> governed delivery stages.
- **Component ownership:** `MarkdownWorkbench.svelte` owns visible mode options and live/edit/preview rendering; `Select.svelte` and `MultiSelect.svelte` own listbox scrolling; `modern-admin-tokens.css` owns semantic color roles; `DecisionDashboard.svelte` owns current/all event scope and the contextual ledger; `DemandKanban.svelte` owns modal geometry, summary, shadow tasks, and the single page scroll; `DemandDeliveryControl.svelte` owns specification/review/execution lifecycle; its Markdown child only owns editor/document scrolling while actively editing.
- **Markdown contract:** default `mode='live'` and `availableModes=['live']`. An explicitly supplied current mode that is outside the default list renders as the sole visible mode, so read-only `preview` and advanced `edit` remain coherent. Consumers may explicitly opt into more modes; `showToolbar=false` remains the correct inline-document path.
- **Decision timeline contract:** replace the decorative global full-width date axis with a compact evidence ledger inside the selected-item inspector before intervention. Default scope is the selected task; a two-option scope control preserves the complete global archive. Rows prioritize outcome/title and message, then actor/time/task/commit evidence. Latest event may receive a restrained surface tint, with no zigzag connector or pill overload.
- **Demand detail contract:** `.detail-modal` owns bounded size and clips; Header is a stable first row; `.detail-body` is the only page-level vertical scroll owner. Shadow-task list loses its own cap/scroll. Specification preview expands naturally inside the detail body; the Markdown editor may own a bounded editor scroll only during active source editing.
- **Responsive behavior:** at 1440px the decision list is left and inspector right; the detail host remains companion-compatible and uses a wider but bounded desktop workbench. At 1024px decision list precedes inspector in stacked flow and detail facts reduce to two columns. At 760px and 480px detail becomes near-full-screen with one-column facts/actions and 44px controls; the ledger becomes a single-column evidence list; no document-level horizontal overflow. Existing companion overlay fallback below 1500px remains.
- **Palette mapping:** primary/selection/focus `#018b8d`; success `#6ecc54` with dark ink; warning `#eb5c20` with dark ink; blocking danger `#c8161d` with white and softer danger accents from `#d34947`; information/evidence `#002fa7` and deep info `#0d3a69`; soft accent surface `#71e2d1`; `#470125` and `#492d22` remain rare named category primitives, not default UI accents. Contrast-unsafe white-on-teal, white-on-green, white-on-orange, and white-on-`#d34947` are prohibited.
- **Validation scope:** default/import/manual/read-only/inline-edit Markdown states; long Select/MultiSelect lists with wheel, keyboard, and hidden rails; decision current/all/empty ledgers and list/inspector order at 1440/1024/760/480; demand detail empty/populated subtasks, no-spec/spec-preview/spec-edit, AI companion, close/backdrop/actions, single scroll owner, 44px targets, no horizontal overflow; palette contrast and semantic state consistency; checks/build, targeted detectors, Finesse scan, diff hygiene, and authenticated browser validation.
- **Disagreement resolved:** the isolated layout assessment considered the global timeline's old bottom placement directionally valid because its data is global. The user's correction and the main product review outweigh that default: transform it into a contextual evidence ledger inside the inspector, while preserving an explicit “全部记录” scope so no audit data is lost. The same assessment also caught the reversed desktop/medium selection-detail order; restore selection-first causality.

### Phase 72 Protection Rules

- Preserve every endpoint, permission, mutation, autosave/commit contract, candidate/document/demand/spec lifecycle, task selection behavior, Jira/GitLab link destination, AI companion ownership, header/shell ownership, route/nav label, accessibility attribute, and unrelated dirty-tree change.
- Do not normalize the seven large files mechanically, add a dependency, invent new data, hide reachable content, remove global decision history, or turn every supplied color into a visible accent.
- Do not edit frontend files until this agreement is recorded. This gate is now satisfied; implementation may begin.

### Phase 72 Implementation And Validation

- `MarkdownWorkbench.svelte` now defaults to `live` and one available mode. Its toolbar derives from the permitted/current mode, so the normal import/review path displays only `即时排版`; explicit advanced `edit` and read-only `preview` consumers remain single-mode and keep their existing lifecycle.
- Shared `Select` and `MultiSelect` option containers retain bounded `overflow-y: auto`, overscroll containment, and keyboard ownership while hiding Firefox and WebKit scrollbar rails. A 31-option authenticated fixture measured `scrollHeight=1238`, `clientHeight=268`, `scrollbar-width:none`, and WebKit scrollbar `display:none`.
- The strongest-brain history is now a selected-item evidence ledger inside the inspector, before manual intervention. It defaults to the current task, preserves an explicit all-records scope, exposes automatic/manual totals, uses flat evidence rows, and has a dedicated empty state. Browser checks confirmed current `2`, all `3`, and zero-event states.
- Desktop strongest-brain geometry keeps selection before detail and aligns both panels to the same bounded height; the 1024px breakpoint stacks selection before inspector and removes the fixed height. The old full-width timeline is absent, and checked desktop/tablet states have zero horizontal overflow.
- The requested color library is mapped once in `modern-admin-tokens.css`: teal primary, green success, orange warning, red danger, navy information, and restrained soft/rare category primitives. Browser-computed primary, success, danger, and information colors matched the supplied values. Dark accent ink is used where white would fail normal-text contrast.
- Flow-board demand detail is a 1080px bounded workbench with a stable route/title header, flat semantic definition grid, summary-adjacent actions, compact shadow-task treatment, and `.detail-body` as the sole page-level vertical scroll owner. The subtask list and Markdown preview no longer create nested page scroll regions; active source editing retains its intentional editor scroll.
- Browser geometry validated the populated detail at 1440, 1024, 760, and 480 widths: desktop modal `1080x860`, tablet modal `976x852`, no document horizontal overflow, one-column narrow facts/actions, transparent fact rows, and only `.detail-body` as the modal scroll owner. The final narrow action rule statically enforces 44px minimum targets and the latest build contains that rule.
- `pnpm -C web check` passes with `0 errors and 78 warnings in 6 files`; `pnpm -C web build` and `git diff --check` pass. Full-file Impeccable/Finesse scans still report legacy side stripes, gradient text, raw white fallbacks, and historic motion elsewhere in the large dashboard files; none intersects the Phase 72 ledger, detail-workbench, Markdown-mode, dropdown-rail, or semantic-token selectors. The new work adds no decorative motion, and existing delivery streaming motion already has a reduced-motion fallback.
- A final browser reconnection attempt was stopped by the browser URL safety policy after the already-successful state/breakpoint runs. No bypass was attempted; final verification used the retained browser measurements plus current build/type/diff and source-contract checks.
# 2026-07-18 - Backend LLM deconstruction availability audit

- [x] Trace effective AI configuration, protocol normalization, provider client, backend deconstruction stream, and browser consumer.
- [x] Verify the latest database-backed configuration without exposing credentials.
- [x] Run the authenticated AI health check against the configured model endpoint.
- [x] Run a harmless synthetic new-demand pre-deconstruction without syncing tasks or assigning a demand.
- [x] Run focused configuration, LLM client, deconstruction, attachment-stream, and demand-spec stream tests.

Outcome: current Pixel / `gpt-5.5` configuration is available through the effective Responses protocol. The full `/api/deconstruct` NDJSON path returned and parsed three task suggestions. Persisted `endpoint_type: completions` remains a legacy value normalized at runtime; it should be saved as `responses` during a future explicit configuration update.

## Current Task Addendum: Daily Jira Inspector Section Separation

- [x] Inspect the supplied screenshot and the live right-inspector grid ownership
- [x] Complete and record the mandatory Impeccable + design-taste-frontend + finesse-ui review before frontend edits
- [x] Separate overview, current-decision state, and morning-decision entry without adding another card
- [x] Preserve fixed section heights, the existing scroll owners, and the no-browser-scroll contract
- [x] Run Impeccable/Finesse detection, checks/build, and authenticated browser validation at affected breakpoints
- **Status:** complete

### Goal

Correct the Daily Jira inspector where the fact overview, current-decision state, and morning-decision form read as one vertically joined module. Restore a clear three-stage reading rhythm while keeping the inspector fixed-height and free of internal scrolling.

### Three-Way Review

- **Design Read:** enterprise morning-triage workbench · restrained light console · register=product · SOUL=4 · SPECTACLE=1 · DENSITY=9 · motion=feedback only.
- **Current-state audit:** `audit-inspector` owns five explicit rows but uses `gap: 0`; the 82px `decision-state` fills its complete row with an inset gray card, so its top and bottom edges sit directly against the fact and form rows. The semantic blocks exist in DOM, but the rendered vertical rhythm does not expose their ownership.
- **Impeccable:** keep one inspector surface and three semantic stages. Use intentional row spacing and sparse hairlines so the state summary is visually independent; do not add a wrapper, nested surface, shadow, or decorative fill. Fixed row contracts must be checked against actual content rather than inferred from compilation.
- **design-taste-frontend:** this dense admin surface is outside the skill's primary marketing scope, so apply preservation guidance only. Retain the existing light theme, teal accent, typography, radii, Jira information architecture, and interaction behavior. Prefer whitespace over another container.
- **finesse-ui:** targeted product redesign only. The inspector remains the component owner; state uses flat hierarchy, 4px-grid spacing, fixed data typography, and semantic danger tint only when a reminder is actually due. Spectacle and decorative motion remain absent.
- **Shared hierarchy:** issue title and facts -> current decision/reminder state -> editable morning-decision form -> audit history. Each transition must be readable without relying on a new card background.
- **Component ownership:** `DailyJiraAudit.svelte` owns the inspector grid rows, section separation, and responsive remapping. Shared Select/Button components, API calls, Jira links, list selection, and decision persistence remain unchanged.
- **Responsive behavior:** desktop keeps fixed-height rows with explicit vertical spacing; the existing <=1180px three-column inspector and <=520px compact remapping keep their named grid areas and receive only the minimum separator adjustment needed for the same hierarchy. No breakpoint may introduce document or inspector scrolling.
- **Validation scope:** selected Jira without a decision, selected Jira with a decision/reminder when available, short and two-line titles, 1440/1024/760/520 widths, visual gaps and section bounds, fixed-height containment, document/inspector/list scroll ownership, keyboard focus, console errors, checks/build, and targeted detectors.
- **Disagreement resolved:** retaining the gray rounded state card would make its boundary explicit, but repeats the nested-surface vocabulary the user already rejected. Flatten the ordinary state into the inspector, separate it with row rhythm and a single hairline treatment, and reserve the semantic danger wash only for an overdue reminder.

### Protection Rules

- Preserve Daily Jira filters, row-click selection, timing dots, issue facts, decision options, reminder semantics, audit history, endpoints, permissions, and all unrelated dirty-tree work.
- Do not add a new component, dependency, card, browser scroll owner, or inspector-local scrollbar.
- Keep the list as the only intentional vertical scroll region in the desktop two-panel workbench.

### Implementation And Validation

- The desktop inspector now uses explicit 12px row rhythm. The fixed title, fact, decision-state, and history tracks were rebalanced to `92 / 124 / 74 / elastic / 128px`, taking space from the previously loose form track without changing total panel height.
- The ordinary current-decision state is transparent, square, and shadowless with sparse top/bottom hairlines; the no-decision copy spans both columns. The existing `.due` state alone keeps its semantic danger wash.
- The <=1180px remap aligns the header and current state on one 92px top track with a shared bottom boundary. The <=520px layout gives facts 148px and compacts only the form gap/textarea, so both list and detail stay fully contained.
- Authenticated geometry at 1440, 1024, 760, and 520 CSS px found zero document overflow, zero horizontal document overflow, zero inspector overflow, zero section overlap, and only the Jira table shell as the intentional scroll owner. Desktop section gaps measured 12px; the tested two-line NS2-268 title measured 43.2px against 21.6px line-height with zero header clipping.
- The live dataset exposed no `latest_decision` rows. No business record was created for testing; the available no-decision state was browser-validated, while the unchanged two-column/due branches were covered by source, type, build, and detector checks.
- `pnpm -C web check`, `pnpm -C web build`, and `git diff --check` pass. Existing warnings remain outside `DailyJiraAudit.svelte`. Impeccable detection returns `[]`, Finesse reports `p0: 0` with no file findings, and the authenticated Chrome console contains no errors.
# Current Task Addendum: Per-User Project Preferences

## Goal

Allow every authenticated user to keep the default all-project view or select multiple projects, then apply that preference to demand and Jira-derived read models across the console.

## Mandatory Three-Way UI Review

- **Design Read:** existing enterprise delivery console for authenticated project members; preserve the Phase 41 light, table-first product language. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=8`; motion is feedback only.
- **Impeccable:** personal project scope belongs to the user profile, not the administrator-only configuration center. Use familiar radio choices for `全部项目` and `指定项目`, reuse the shared accessible multi-select, expose loading/error/success/disabled states, and make the empty-selection rule explicit.
- **design-taste-frontend:** this is dense product UI outside the skill's primary marketing scope, so apply preservation guidance only. Keep the current system sans, teal semantic focus/selection, 4px rhythm, established controls, routes, and copy voice. Add no dependency, imagery, decorative motion, or second design system.
- **finesse-ui:** keep the profile popover as the entry point and use progressive disclosure for the editor. Do not introduce a modal for one preference. Server-side scope owns visibility; the UI only edits and summarizes the preference. Use standard controls, one primary save action, one secondary cancel action, and no decorative card nesting.
- **Shared hierarchy and ownership:** profile identity -> current project scope summary -> inline preference editor -> logout. `ProjectPreferences.svelte` owns preference loading, mode selection, multi-select, validation, and save feedback. `FunctionalAdminShell.svelte` owns placement in the authenticated profile popover. Backend preference helpers own normalization, persistence, and task/Jira query scoping. Data surfaces only react to a shared refresh event after a successful save.
- **Responsive behavior:** profile popover remains bounded to the viewport; the preference section is one column at every width; project options use the shared hidden-rail scroll owner; controls are at least 44px on narrow screens; long project names truncate in options and wrap in the selected summary without horizontal page overflow.
- **Validation scope:** default-all user, selected multi-project user, clearing back to all, invalid/unknown project rejection, authenticated isolation between two users, task/schedule/execution/daily-Jira/agenda filtering, saved-state refresh, desktop and 760/480 profile states, keyboard selection, loading/error/success feedback, checks/build, detector scans, and authenticated browser validation without mutating demand or Jira business records.
- **Disagreement resolved:** a dedicated configuration-center page would provide more room, but ordinary members may not have configuration permissions and project scope is personal rather than administrative. Keep the entry in the profile popover and progressively disclose one inline editor; if the project list grows beyond the bounded selector's capacity, the shared selector remains its own scroll owner rather than expanding the popover into a new page.

## Protection Rules

- Empty preference rows mean `全部项目` for backward compatibility and default behavior.
- Validate selected keys against the currently available Jira project catalog and normalize them case-insensitively before persistence.
- Intersect personal scope with existing route permissions, core-member visibility, and any page-local project filter; never broaden access.
- Preserve all Jira sync configuration, project priority configuration, task mutations, routes, labels, and unrelated dirty-tree work.
- Do not create or update a demand, Jira issue, agenda decision, or daily-Jira review during validation.

## Plan

- [x] Add normalized per-user project preference persistence and authenticated GET/PUT endpoints.
- [x] Apply the saved scope to demand and Jira-derived read models and guard direct task/Jira detail or decision paths.
- [x] Add the profile-owned preference editor and refresh affected surfaces after save.
- [x] Run focused backend tests, frontend checks/build, design detectors, and authenticated browser validation.

## Validation Result

- Go: full `internal/db`, `internal/server`, and `internal/agenda` suites pass with loopback-enabled `httptest` support.
- Frontend: `pnpm --dir web check` passes with 0 errors and the existing 72-warning baseline; production build passes.
- Detection: exact-file Impeccable scan for the new preference component and shell integration returns no findings.
- Browser: isolated authenticated user starts at all projects; empty selected mode disables save; HIT + NS2 save reduces agenda from 150 to 6 and every visible Daily Jira row has only HIT/NS2 prefixes; saving all restores the original range.
- Responsive: desktop profile state passes; at 480x900 the popover is bounded to `left=12`, `right=468`, controls are 44px or taller, and browser console errors remain empty. The validation user was restored to all projects and the isolated no-integration runtime was stopped.

# Current Task Addendum: Structural Workspace-Scoped Demand Overlay

## Goal

Make the new-demand dialog and its create-flow AI companion belong to the right-side workspace below the header, so their mask never covers or alters the sidebar, top header, or maintenance banner.

## Mandatory UI Review Agreement

- **Design Read:** existing high-density enterprise operations console; targeted preservation fix. `register=product`, `SOUL=4`, `SPECTACLE=1`, `DENSITY=9`; motion remains state feedback only.
- **Current-state diagnosis:** the previous fix still renders the affected backdrops as viewport-fixed layers and only simulates the workspace boundary with measured top/left variables. Even when the rectangles line up, ownership remains global and can drift across shell variants. The schedule root can also become its own positioned container, so a CSS-only switch to `position: absolute` is not sufficient by itself.
- **Impeccable:** the shell must own overlay geometry. Move the affected backdrop nodes into `.workspace-stage`, the existing positioned and isolated workspace container, then use `position: absolute; inset: 0` on desktop. The header, maintenance banner, and navigation rail stay outside that stacking context and remain fully visible and interactive.
- **design-taste-frontend:** this dense product workflow is outside the skill's primary marketing scope, so only its preserve-redesign rules apply. Keep the existing route, dialog hierarchy, copy, fields, color, radius, spacing, focus behavior, and close paths unchanged.
- **finesse-ui:** use a narrow product-register correction, not a modal redesign. The mask owns only the task surface; mobile keeps a fixed remainder-of-viewport boundary because `.workspace-stage` intentionally becomes auto-height there. No new visual effect, dependency, or decorative motion is introduced.
- **Project UI system:** the existing Phase 41 light admin tokens, custom controls, 4px rhythm, and shell z-index vocabulary remain authoritative. `FunctionalAdminShell.svelte` continues to own the workspace boundary; `DemandKanban.svelte` only portals the create-flow overlays to that boundary.
- **Hierarchy:** sidebar and header stay normal -> optional maintenance banner stays normal -> workspace content is masked -> dialog is centered inside that masked region.
- **Component ownership:** `.workspace-stage` is the desktop containing block; the create-demand backdrop and create companion are its children while open. The measured top token remains only for the mobile fixed fallback. Other confirmation, schedule, and detail overlays retain their current behavior.
- **Responsive behavior:** desktop widths above 860px use structural absolute containment and automatically follow expanded/collapsed rail geometry. At 860px and below, the backdrop stays fixed from the measured workspace top to the viewport bottom with `left: 0`, so long document height cannot move the dialog off-screen.
- **Validation scope:** verify the actual `offsetParent`, computed position, and matching rectangles for expanded and collapsed desktop rail; prove header, banner, and rail do not intersect the mask; verify mobile fixed geometry, create AI companion containment at wide desktop, dialog overflow, keyboard close/focus behavior, console errors, Svelte check/build, targeted detectors, and diff hygiene.
- **Disagreement resolved:** retaining viewport-fixed geometry on desktop can visually match one shell snapshot but does not satisfy the user's ownership requirement. Pure descendant absolute positioning can bind to `.flow-dashboard` instead of the shell. A scoped portal into `.workspace-stage` is therefore the stable structural solution. The old overlay-positioning offset is removed; a renamed `--wa-workspace-inline-start` token remains only for deterministic wide companion sizing because percentage values resolve against different boxes in `width` and `transform`.

## Protection Rules

- Preserve dialog content, field order, labels, submission behavior, AI behavior, routes, business data, and unrelated dirty-tree work.
- Do not create or submit a demand, Jira issue, schedule mutation, or AI draft during validation.
- Do not change global overlays that were not named in this complaint.

## Plan

- [x] Reproduce the structural defect from source ownership and inspect all positioned ancestors.
- [x] Complete the mandatory Impeccable, design-taste-frontend, and finesse-ui review before frontend edits.
- [x] Portal the create-flow overlays into `.workspace-stage` and replace desktop coordinate simulation with structural containment.
- [x] Remove the obsolete overlay-positioning compensation while retaining the mobile top boundary and a companion-width-only inline-start token.
- [x] Run checks, build, detectors, authenticated browser geometry validation, and diff hygiene.

## Validation Result

- Desktop `1280x720`, expanded rail: `.workspace-stage` and `.demand-create-backdrop` both measure `x=270, y=112.5, width=1010, height=607.5`; the backdrop is an absolute child of `.workspace-stage`. Header ends at `y=68`, maintenance banner ends at `y=112.5`, rail ends at `x=270`, and all three are structurally outside the mask.
- The header menu remains clickable while the dialog is open. After collapsing the rail, stage and backdrop both update to `x=94, y=112.5, width=1186, height=607.5`; the dialog recenters without a manual overlay offset. The top-level authenticated browser console is empty.
- Mobile `760x900`: the portaled backdrop uses the fixed fallback at `x=0, y=108.5, width=760, height=791.5`; the dialog stays within `14px` horizontal insets and the document has zero horizontal or vertical overflow while open.
- Wide `2048x934`: the create and companion backdrops are both absolute children of the stage and match `x=270, y=112.5, width=1778, height=821.5`. The 680px host and 525px companion remain contained with an exact `16px` gap and zero document overflow.
- `pnpm --dir web check` passes with `0 errors` and the existing `73 warnings in 4 files`; production build and `git diff --check` pass.
- Exact-file Impeccable layout detection returns no findings. Finesse reports `p0: 0`; only the files' pre-existing P2 pure-white findings remain.
- Validation used the isolated local database with Jira and AI disabled. No demand, Jira issue, schedule mutation, or AI draft was submitted, and the temporary responsive fixture was deleted.

# Current Task Addendum: Jira Version Sources And Decision Bottom Substrate

## Goal

Allow administrators to configure Jira release-page sources with a project number and name, parse each link into a safe `project + fixVersion` query, and remove the exposed gray shell strip below every decision page without changing business-card structure.

## Mandatory UI Review Agreement

- **Design Read:** existing high-density enterprise operations console for project administrators and morning-triage operators; restrained light teal product language. `register=product`, `SOUL=4`, `SPECTACLE=2`, `DENSITY=8`; motion is feedback only.
- **Current-state audit:** Jira currently supports ordinary project/member/status filters or one overriding custom JQL, but has no typed version source. The decision screenshot's full-width bottom gray strip is outside the cards: `.workspace-frame` owns a gray page background plus four-sided shell padding, while `FunctionalWorkspace`, `DecisionDashboard`, and `DailyJiraAudit` are intentionally transparent.
- **Impeccable layout assessment:** keep shell as the sole background/gutter/viewport owner. Remove the decision route's bottom gutter at `.workspace-frame.viewport-fit-frame`; do not add a compensating background, negative margin, or wrapper in workspace/business pages. Keep Jira version mappings as one flat, hairline-separated configuration section with labels, inline parse/error state, and no nested decorative cards.
- **Impeccable mechanical pre-scan:** `detect.mjs --scope layout` returned `[]` across the shell, workspace, decision, Daily Jira, Jira config, and token targets. Tailwind arbitrary spacing/z-index patterns had no matches. A clean scan is only the floor; source geometry still proves the exposed bottom gutter.
- **design-taste-frontend:** dense dashboards and settings are outside its primary marketing scope, so apply preserve-redesign guidance only. Retain the existing route/IA, system sans, teal accent, fixed product type, 4px rhythm, copy voice, labels, and accessibility. Add no dependency, palette, font, imagery, ornamental card, or motion system.
- **finesse-ui:** use a targeted product redesign. Standard labels and inputs remain familiar; the list editor exposes default, focus, parsed, invalid, disabled/saving, add, and remove states. Release sources are additive data scope, not a second Jira connection workflow. The shell fix stays route-scoped and must not change scroll ownership on unrelated pages.
- **project UI system:** existing Phase 41 tokens, 10px field groups, 14px primary panels, hairline separators, one accent, fixed 12/13/16px product type, and 4/8/12/16/24px rhythm remain authoritative. Native labeled inputs may compose the repeated row; no new component library is justified.
- **Shared hierarchy:** Jira connection -> ordinary synchronization scope -> optional release-version sources -> advanced custom JQL -> connection test -> configuration summary. Decision hierarchy remains breadcrumb -> decision content; the content now terminates at the browser edge rather than above an artificial gray footer-like strip.
- **Component ownership:** backend config owns version-source validation/canonicalization; the Jira worker owns additive JQL construction and keep-alive project scope; `JiraConfig.svelte` owns editing and immediate parse feedback; `SettingsPanel.svelte` only transports the typed config. `FunctionalAdminShell.svelte` owns the decision route bottom inset. `FunctionalWorkspace`, `DecisionDashboard`, and `DailyJiraAudit` remain transparent render-only children.
- **Responsive behavior:** version-source rows use three desktop fields and collapse to one column below the existing Settings breakpoint; remove stays a 44px target on narrow screens; long URLs wrap/clip inside their own field without page overflow. Decision pages keep the existing viewport-fit desktop contract, but the bottom inset becomes zero at every active decision breakpoint so no gray strip is exposed. Broader short-viewport/narrow-page fit changes are deferred because they are a distinct issue from the reported bottom strip.
- **Validation scope:** parser unit tests for exact/relative/context-path/wrong-origin/malformed/mismatch/duplicate links; JQL union and project-scope tests; configuration save rejection; Svelte type/check/build; Jira settings empty/one/multiple/invalid/summary states at desktop/760/480; decision agenda and Daily Jira bottom geometry at desktop/1024/760, no gray strip, no document overflow, correct scroll owners, keyboard focus, console errors, targeted Impeccable/Finesse scans, and diff hygiene. No Jira issue, demand, decision, or review record may be created.
- **Disagreement resolved:** the layout assessment also found a separate narrow/short-viewport density risk and gray-white gradients inside some decision panels. The supplied red outline identifies the external full-width shell strip, so this pass fixes only its real owner. It does not flatten valid table/inspector surfaces or expand into a responsive-height redesign. Version sources are additive to ordinary/custom JQL because otherwise a configured release outside the ordinary project list would never be fetched; the UI states this explicitly.

## Protection Rules

- Preserve credentials, ordinary Jira filters, custom JQL semantics within its ordinary branch, issue pagination, comments, status mapping, task mutation behavior, routes, labels, and unrelated dirty-tree work.
- Accept only HTTP(S) release links on the configured Jira origin and base path with `/projects/{projectKey}/versions/{numericVersionID}`; never fetch or scrape arbitrary version-page HTML.
- A version-source project number must match the link, project name is required for operator clarity, duplicate project/version pairs are rejected, and stored URLs are canonicalized without query or fragment.
- Do not modify real Jira configuration or business data during validation.

## Plan

- [x] Audit the existing Jira config/search path and decision shell geometry.
- [x] Complete isolated Impeccable layout assessment and mechanical pre-scan.
- [x] Record Impeccable + design-taste-frontend + finesse-ui + project UI-system agreement before frontend edits.
- [x] Implement parser, validation, project catalog integration, and additive version JQL.
- [x] Add the responsive Jira version-source editor and summary.
- [x] Remove the decision route's exposed bottom shell gutter.
- [x] Run targeted tests, checks/build, detectors, authenticated browser validation, and diff hygiene.

## Validation Result

- Parser coverage passes for the supplied `PRJ25024 / 13622` URL, relative/context-path URLs, wrong origins/routes/IDs, canonicalization, mismatch, duplicate, and project-scope union behavior.
- Jira worker coverage passes for version-only, ordinary-plus-version, custom-JQL-plus-version, invalid-source, and keep-alive project-scope behavior. Configuration save rejects invalid sources before applying them.
- The version-source project number/name flows into demand options, project preferences, project configuration/scores, and telemetry scoring rather than remaining a worker-only setting.
- `pnpm --dir web check` passes with 0 errors and 71 pre-existing warnings in unrelated files; production build and `git diff --check` pass.
- Exact-file Impeccable complete/layout detection returned `[]`. Finesse returned P0=0; its seven shell pure-white findings are pre-existing and outside the bottom-inset change.
- Browser configuration validation at desktop and 760px parsed the supplied URL immediately, exposed `使用 PRJ25024`, and confirmed `已识别 PRJ25024，版本 ID 13622`; the narrow form had zero horizontal overflow. Save was not clicked.
- Authenticated fixture validation at 2048x924 and 760x900 covered both Decision Agenda and Daily Jira. In every state the viewport-fit frame and page root ended exactly at the viewport bottom with `padding-bottom: 0`; narrow states had zero document horizontal overflow and the console error list was empty.
- Validation used a fresh temporary SQLite database with Jira disabled. No Jira issue, demand, decision, review, or production configuration was created or changed; all isolated services were stopped.
- **Status:** complete
# 2026-07-29 排期治理项目看板、Jira 反向同步与决策事项表

## 目标

- 在“排期治理”下新增“项目看板”：按项目切换，三列展示待办、处理中、完成。
- 决策面板和排期面板提交排期后，把负责人和到期日同步到 Jira；把决策结论描述同步为 Jira 评论。
- 决策事项表区分 Bug / Task，支持按用户持久化显示列，并统一状态列与其余列的背景。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：沿用现有 Phase 41 轻量管理台与流转看板的信息密度。应用壳继续负责视口与滚动边界；项目看板只拥有“项目横向选择轨 + 三个同级泳道”。选择轨必须同时支持鼠标、触摸、键盘和窄屏横向滚动。加载、空数据、错误与项目偏好变化都是一等状态。
- **design-taste-frontend**：这是现有高密度业务产品的增量功能，不引入新的视觉主题。保持现有字阶、令牌、圆角与表格语法；新增菜单是唯一的信息架构变化，避免额外 Hero、嵌套卡片和装饰性动效。
- **finesse-ui**：界面寄存器为产品工作台，SOUL 4 / SPECTACLE 1 / DENSITY 9。项目切换器紧凑、可扫描，三列保持平权；动效仅用于选择与反馈，窄屏点击目标不小于 44px。列配置复用共享 MultiSelect，并由后端按用户持久化。

### 分歧与取舍

- finesse 的通用视觉 DNA 可使用纹理等品牌手法，但与本项目 Phase 41 的浅色表格优先契约冲突；以仓库契约为准，不增加纹理或新底材。
- 不把项目维度继续塞进已超过一万行的 `DemandKanban.svelte`；新建职责单一的 `ProjectBoard.svelte`，让既有排期/流转状态和滚动行为不受影响。
- “关注项目”以现有用户项目偏好为唯一数据边界：选择模式只展示已关注项目；全项目模式按项目基础优先级排序。这样与后端权限交集一致，不制造可选但无数据的项目。
- 列配置使用列表工具栏中的共享弹出式多选，不增加模态框；事项编号列固定显示，避免用户配置后失去主标识与导航入口。

## 组件所有权与响应式

- `FunctionalAdminShell.svelte`：只负责新增“项目看板”子菜单与激活态。
- `App.svelte`：只负责视图路由和新看板挂载。
- `ProjectBoard.svelte`：负责项目偏好、项目优先级、排期数据、项目选择与三列映射；宽屏三列，窄屏保持单一受控滚动面，不产生页面级双滚动。
- `DecisionDashboard.svelte`：负责事项类型标识、列选择 UI 与状态列背景统一；列偏好读写由 `/api/me/decision-table-columns` 负责。
- Jira 写入只在本地事务成功后异步触发；排期到期日与决策评论均不进入数据库事务，失败必须可观测且不得回滚已成功的本地操作。

## 验证范围

- Go 单测覆盖 Jira 到期日/评论请求契约、用户列偏好校验与持久化、排期和决策干预的同步触发。
- 前端构建与静态检查；Impeccable 和 Finesse detector 检查所有变更的前端文件。
- 使用隔离数据库和禁用真实外部写入的本地服务做认证浏览器验证：项目看板加载/空态/切换、三列与横向选择；决策表类型标识、列配置持久化、状态列背景；覆盖桌面与窄屏。

## 完成与验证

- 新增独立项目看板、横向项目选择轨和待办/处理中/完成三列；有关注项目时仅展示关注范围，否则按项目优先级回退排序。
- 排期与决策改期均在本地保存成功后反向同步 Jira 到期日；非空决策结论同步为 Jira 评论。
- 决策事项表加入统一 Bug/Task 标识、按用户持久化列配置，并让状态单元格继承整行背景。
- `go test ./internal/telemetry ./internal/db ./internal/server` 与最终定向回归均通过；`pnpm check` 为 0 error / 73 个既有 warning，`pnpm build` 通过。
- Impeccable 最终扫描返回 `[]`；新增 `ProjectBoard.svelte` 与改动后的 `DecisionDashboard.svelte` 的 Finesse 扫描无告警。
- 认证浏览器在 1440×900 与 390×844 验证项目切换、泳道数据、详情展开、列配置刷新持久化、状态背景、44px 窄屏控件和受控横向滚动；页面无横向溢出，控制台无错误。
- 浏览器验证使用隔离数据库并禁用 Jira/GitLab/AI 外部写入；未对真实 Jira 或生产数据执行写操作，临时服务和数据已清理。
- **Status:** complete

---

# 2026-07-30 项目、版本、交付项与页面收敛实施

## 执行顺序

- [x] Phase 0：冻结 `CONTEXT.md` 领域词汇、生成只读数据基线和旧接口兼容 fixture。
- [x] Phase 1-2：增加项目、版本、版本关系、事件、同步操作和父交付项事实，完成 dry-run 迁移与差异报告。
- [x] Phase 3-4：接入 Jira 版本事实、多版本冲突、revision/CAS、事件/outbox，并把旧排期写接口收口为统一 planning adapter。
- [x] Phase 5：交付计划成为项目、版本、负责人、截止日的唯一写入口；项目看板成为只读流转 Lens。
- [x] Phase 6-7：执行页只保留执行任务，决策与洞察收敛为共享目录和多个 Lens。
- [ ] Phase 8：只删除已经通过调用观测和 deletion test 证明无调用的兼容路径。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：按现有 Phase 41 产品工作台执行 harden，而不是重做视觉系统。页面壳继续拥有视口和滚动边界；交付计划拥有表格/流转/版本视图与右侧检查器。项目看板卡片的收起态必须直接显示负责人，主内容必须是鼠标和键盘都可操作的详情入口，Jira 深链接必须始终可见且与展开操作分离。加载、空数据、网络错误、权限只读、revision 冲突和 Jira 链接缺失均是一等状态。
- **design-taste-frontend**：该技能声明 dense product UI 不是其主适用面，因此不引入其营销页布局或动效；只采用 redesign-preserve 约束。保留路由、浅色令牌、字阶、圆角、导航标签和既有可访问性，不重排信息架构，不增加 Hero、装饰性动效、嵌套卡片或新的视觉主题。
- **finesse-ui**：Design Read 为“研发交付治理产品工作台，清晰、可信、高密度”，`register=product`，`SOUL 4 / SPECTACLE 1 / DENSITY 9`。表格/列表是事实主面，检查器是唯一规划编辑面；卡片、按钮和链接使用标准 affordance，状态变化不靠颜色单独表达，窄屏触控目标不小于 44px。

### 分歧与取舍

- 既有项目卡片把点击定义为原地展开，但用户把“内容可点击”理解为进入事项详情。保留展开以维持三列浏览效率，同时让卡片主区承担明确详情入口，并把 Jira 链接常驻为独立点击区域；禁止把 `<a>` 嵌套在 `<button>` 内。
- finesse 的通用 substrate 建议可加入纹理；它与仓库 Phase 41 平整、表格优先的产品契约冲突，以 `DESIGN.md` 为准，不增加纹理、霓虹、渐变或额外玻璃层。
- design-taste 建议 dense product UI 使用成熟产品组件系统；本仓库已经有共享 Select、MultiSelect、DatePicker、Toast 和 Phase 41 tokens，继续复用现有系统，不再引入第二套依赖。

## 层级、组件所有权与响应式

- `internal/deliveryplanning`：唯一拥有类型归一化、项目/版本门禁、revision、事件和 outbox 规则。
- `DeliveryPlan.svelte`：交付计划页面所有者，拥有表格/流转/版本视图、选择稳定性、批量操作和右侧规划检查器。
- `ProjectBoard.svelte`：只读项目流转 Lens；显示负责人、规划状态和常驻 Jira 链接，通过共享选择状态打开交付计划详情，不保存独立规划规则。
- `DemandKanban.svelte`：迁移期兼容 wrapper，不再新增规划规则。
- `TaskKanban.svelte`：只读显示父交付项、项目和主目标版本；项目/版本编辑跳转到交付计划。
- `FunctionalAdminShell.svelte` 与 `App.svelte`：只负责导航、路由和共享选择 handoff，不解释业务事实。
- 宽屏使用表格/三泳道与右侧检查器，且只有预期的内部滚动所有者；低于 860px 转单列，低于 760px 保持 44px 触控目标和单一受控横向滚动边界；所有断点禁止文档级横向溢出。

## 验证范围

- 缺陷红灯覆盖：卡片主内容可由点击和 Enter/Space 打开、负责人在收起态可见、Jira 深链接常驻且 URL 正确。
- Go 领域/数据库/HTTP/Jira adapter 测试覆盖计划中的门禁、唯一性、事务、冲突、幂等和兼容响应。
- 前端执行 `check`、生产构建、Impeccable detector、Finesse anti-cheap/preflight。
- 使用隔离数据库和关闭真实 Jira 写回的认证浏览器，验证 2048×924、1440×900、1280×800、760×900、480×900；覆盖交付计划各状态、看板详情与 Jira 跳转、键盘、焦点、滚动所有权、选择稳定性和控制台错误。

---

## 实施与验证结果

- Phase 0 冻结了交付领域术语、历史 SQLite 只读基线、迁移统计和旧排期响应 fixture；迁移工具默认 dry-run，只有显式提供备份并选择 apply 才允许写入。
- 新增项目、版本、目标/影响版本关系、规划事件、同步操作、outbox、父交付项和 revision 事实；领域服务统一执行项目/版本门禁、Bug 影响版本规则、CAS 冲突、原因/操作者审计及原子批量规划。
- Jira adapter 已支持版本目录读取与幂等对账；自动来源仅在唯一候选时设主版本，多候选进入待澄清，人工确认的主版本不会被自动漂移覆盖。
- `/api/work-items`、单项规划、原子批量、例外、版本快照和版本同步接口已接入；旧排期写接口降级为统一 planning service 的兼容 adapter，不再拥有独立业务规则。
- 新增“版本计划”页面作为项目、主目标版本、影响版本、负责人、截止日和规划状态的写入口。项目看板保持只读 Lens，卡片主区支持鼠标、Enter 和 Space 展开，负责人在收起态可见，Jira 链接为常驻独立锚点。
- 执行页只消费 execution task 读模型，不再把 Demand/Bug 父项混入任务总数；有父交付项时才显示“打开交付计划”，孤立任务不提供无效跳转。决策与洞察通过共享入口组件收敛为独立 Lens。
- 兼容 GET `/api/tasks`、GET `/api/schedule` 和 POST `/api/tasks/schedule` 现在返回弃用标识并按端点计数；`GET /api/delivery/quality` 汇总未归项目、未归版本、多版本歧义、孤立执行任务、同步失败、revision 冲突和兼容调用。
- Phase 8 的物理删除保持关闭：当前迁移期页面仍存在真实兼容调用，且尚未经历计划要求的一个稳定发布周期。删除门禁必须先看到调用计数归零并通过 deletion test；本次不以“代码已转接”冒充“兼容路径可安全删除”。
- 全量 `go test ./...` 通过。`pnpm check` 通过且为 0 error，生产构建和 `git diff --check` 通过。精确变更文件 Impeccable 检测返回 `[]`；Finesse 为 P0=0，新建的交付计划和项目看板无 findings，既有大文件只保留历史纯白色 P2。
- 隔离认证浏览器验证：项目看板 101 张卡片均可展开并有独立 Jira 链接，负责人直接可见；桌面和 760/480 宽度均无文档级横向溢出，窄屏 Jira 目标为 44px。版本计划加载 500 条事实，刷新后保持选中项，桌面表格/检查器和 480px 内部横向滚动归属正确。任务页只显示 9 条 execution Task，孤立项不再出现无效规划按钮。
- 验证数据库为 SQLite 备份，Jira、GitLab、飞书和 AI 外部集成关闭；没有执行迁移 apply、Jira 跳转、规划保存或任何真实外部写入。
- **Status:** implementation complete; Phase 8 deletion gate awaiting one stable release cycle

---

# 2026-07-30 版本事实、Jira 关联与人工调停反馈纠偏

## 目标与执行顺序

- [x] 先把“版本计划”纠正为现有发布版本的事实列表，不再以交付项表格冒充版本计划。
- [x] 版本允许绑定项目，并通过可搜索的 Jira 版本目录建立显式关联；本地版本身份与 Jira 外部身份分开保存。
- [x] 项目看板的横向项目开关只显示项目全称，不再重复显示项目简称。
- [x] 人工调停使用独立的会议备注字段；仅当会议备注非空时把原文同步为 Jira 评论，禁止注入默认评论。
- [x] 把人工调停详情抽屉改为标准弹窗，保存成功或失败统一通过右上角 Toast 反馈。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：版本是页面主对象，使用“版本事实表 + 单一版本检查器”的高密度结构；项目绑定与 Jira 关联属于版本编辑能力。Jira 搜索必须完整覆盖输入、加载、结果、空结果、错误和已关联状态。人工调停是一次聚焦提交，应使用项目共享弹窗并让成功、失败反馈进入全局 Toast。
- **design-taste-frontend**：保持 Phase 41 浅色、表格优先的产品系统，不引入新主题、Hero、装饰性动画或第二套组件库。项目切换器移除简称后只保留全称和必要的关注状态，减少重复标签。
- **finesse-ui**：Design Read 为研发治理产品工作台，`SOUL 4 / SPECTACLE 1 / DENSITY 9`。版本表承担事实扫描，右侧检查器承担绑定编辑；人工调停弹窗只容纳完成决策所需的事实、负责人/日期和会议备注，Toast 作为跨页面一致的提交反馈。

### 分歧与取舍

- finesse 的通用产品建议倾向用抽屉承载编辑，但本次调停是一项有明确提交边界的短事务，且用户要求统一弹窗与右上角反馈，因此采用居中宽弹窗；弹窗内容保持单一滚动，不复制页面级导航。
- Jira 版本不会再作为本地发布版本直接写入同一身份行。版本计划保留本地版本为主事实，通过独立关联记录引用 Jira 项目和 Jira 版本，避免同步覆盖本地版本名称、状态和时间窗。
- “版本可绑定项目”表示版本在建档/迁移阶段可以暂时未绑定；进入发布承诺前仍需绑定项目。页面把“未绑定项目”和“未关联 Jira”作为显式待治理状态，而不是静默缺省。

## 组件所有权与验证范围

- `internal/db` 与 `internal/deliveryplanning`：拥有发布版本事实和 Jira 关联记录的唯一存储语义。
- `release_handlers.go`：提供全局版本列表、项目绑定、Jira 搜索、关联和解除关联；Jira 搜索只读，不创建版本事实。
- `DeliveryPlan.svelte`：只展示现有版本，负责筛选、项目绑定和可搜索 Jira 关联；不再承载工作项规划写入。
- `ProjectBoard.svelte`：仅移除项目简称，不改变关注优先级、三泳道和选择状态。
- `DecisionDashboard.svelte`：使用共享 `Modal` 和 `showToast`；会议备注原文作为独立字段提交，空值不产生 Jira 评论。
- Go 测试覆盖关联唯一性、搜索过滤、项目绑定和空/非空会议备注；前端执行类型检查、构建、检测器和认证浏览器桌面/窄屏验证。

## 实施与验证结果

- 全量 `go test ./... -count=1` 通过；版本目录/项目绑定/Jira 搜索关联以及会议备注空值、非空原文同步的定向测试通过。
- `pnpm --dir web check` 为 0 error，`pnpm --dir web build` 和 `git diff --check` 通过；精确变更文件 Impeccable 检测返回 `[]`，Finesse 为 P0=0。
- 隔离认证浏览器验证确认：版本页只展示现有版本；项目绑定成功后指标同步更新；`2.1` 搜索只返回匹配的 Jira 版本并可建立关联；项目滑轨只显示“香港二期”全称，卡片负责人和 Jira 链接仍可见、可操作。
- 人工调停空会议备注落库为空且未触发 Jira 评论；填写“会议确认：等待接口联调，周五复核。”后，事件账本保存同一原文，弹窗关闭并在右上角显示“人工调停已保存，会议备注已提交 Jira 同步”。
- 390px 验证无文档级横向溢出：版本表将横向滚动限制在表格容器，人工调停弹窗宽 342px、左右各 24px，正文为单一滚动区；弹窗初始焦点位于关闭按钮，关闭后焦点返回事项行，浏览器错误日志为空。
- 浏览器与后端均使用临时 SQLite、Mock Jira 和本地端口；未连接或写入生产 Jira/数据库。
- **Status:** complete

---

# 2026-07-31 共享弹窗遮罩仅覆盖右侧内容区纠偏

## 目标

- [x] 用真实共享弹窗复现侧边栏与 Header 被遮罩的问题，并建立可重复的几何/命中断言。
- [x] 把共享 `Modal` 的遮罩挂载到右侧 `.workspace-stage`，而不是 `document.body`。
- [x] 保留无工作台容器时的视口级回退，不改变弹窗内容、提交、关闭、焦点和业务状态。
- [x] 让同一断言在桌面展开/收起侧栏及 760/390 窄屏的共享 dialog 状态转绿。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：这是产品 Shell 的层级归属缺陷，不是透明度问题。共享弹窗必须以 `.workspace-stage` 为桌面 containing block，遮罩、模糊和指针命中均不得越过工作区边界；无工作台容器时保留现有全视口回退。
- **design-taste-frontend**：该技能不负责高密度产品 UI，仅应用 redesign-preserve 约束。保留现有浅色令牌、Modal 内容、字号、圆角、路由和交互语义，不引入新的视觉语言或依赖。
- **finesse-ui**：Design Read 为研发治理产品工作台，`register=product`、`SOUL 4 / SPECTACLE 1 / DENSITY 9`。只修共享 overlay 所有权；dialog 在右侧内容区居中，drawer 继续复用同一工作区边界，动画只表达状态。

### 分歧与取舍

- 仅给某个页面弹窗增加偏移会继续遗漏 `DecisionDashboard`、`TaskKanban` 和 `DeliveryPlan` 的共享 `Modal`；统一修复共享组件，页面调用方不再各自计算 Header/侧栏尺寸。
- 桌面端使用结构性的 `workspace-stage + absolute inset: 0`；移动端因工作区允许自然高度，使用从 `--wa-main-content-top` 到视口底部的 fixed 回退。两者都保证 Header 不被遮罩。
- 共享组件仍保留 `body + fixed` 的无 Shell 回退，避免独立预览或未来非工作台调用失去遮罩。

## 红灯证据与保护规则

- 修复前真实共享“创建版本”弹窗的遮罩父节点是 `body`，计算样式为 `position: fixed`，矩形为 `x=0, y=0, width=1280, height=720`。
- 修复前遮罩与侧栏重叠 `194400px²`、与 Header 重叠 `68680px²`，两处中心点的 `elementFromPoint` 都命中共享遮罩；回归断言 `pass=false`。
- 保护弹窗标题、内容、footer、关闭路径、Escape/Tab 行为、body scroll lock、提交逻辑、Toast 和所有业务数据；验证期间未提交版本、调停或任务写入。

## 实施与验证结果

- 根因是共享 `Modal.svelte` 仍使用 `portalToBody + position: fixed + 100vw/100dvh`。第一次仅用 `classList` 标记作用域时，Svelte 将对应 scoped CSS 视为不可达并从产物中裁掉；改成 Svelte 可追踪的 `class:is-workspace-scoped` 后规则真实进入构建产物。
- 1280×720 展开侧栏时，遮罩与工作区均为 `x=270, y=112.5, width=1010, height=607.5`，计算定位为 `absolute`；侧栏/Header 重叠均为 0，中心点分别命中侧栏品牌区与 Header，而不是遮罩。
- 侧栏收起后，遮罩和工作区同时更新为 `x=94, width=1186`，仍与侧栏/Header 零重叠。弹窗打开时 Header 用户菜单可展开，侧栏导航可切换页面。
- 760×900 时遮罩为 `x=0, y=108.5, width=760, height=791.5`；390×900 时为 `x=0, y=127, width=390, height=773`。两种窄屏均从 Header/维护条下方开始，弹窗完整位于遮罩内，文档宽度分别保持 760/390，无横向溢出。
- `pnpm --dir web check` 通过，结果为 0 error 和既有 73 warnings；生产构建、`git diff --check` 通过。Impeccable detector 返回 `[]`，Finesse 为 `p0: 0` 且共享 Modal 无 findings；认证浏览器错误日志为空。
- 验证使用关闭 Jira、GitLab、飞书和 AI 的隔离 SQLite 服务，没有提交版本、Jira、调停或任务写入。
- **Status:** complete

---

# 2026-07-31 共享弹窗毛玻璃半透明调整

## 目标与保护边界

- [x] 将共享弹窗遮罩从偏实的复合底色调整为明确的半透明毛玻璃层。
- [x] 保持弹窗正文为高可读实色，不改变标题、内容、footer、关闭路径、焦点和业务状态。
- [x] 保持遮罩只覆盖 `.workspace-stage`，Header 与侧边栏继续正常显示和交互。
- [x] 完成桌面与窄屏视觉、几何及构建验证。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：毛玻璃仅属于用于聚焦的遮罩层。降低遮罩底色的不透明度，让底层内容轮廓可辨；弹窗容器继续保持高不透明度，保证表单和正文对比度。
- **design-taste-frontend**：该技能不主导高密度后台界面，仅采用 redesign-preserve 约束。保留现有浅色产品体系、组件尺寸、交互和工作区定位，不引入装饰性玻璃卡片或新依赖。
- **finesse-ui**：Design Read 为研发治理产品工作台，`register=product`、`SOUL 4 / SPECTACLE 1 / DENSITY 9`。采用单层冷色半透明遮罩配合适度 blur；drawer 使用更轻的遮罩，避免玻璃效果成为装饰主体。

### 分歧与取舍

- “半透明”不通过整个组件的 `opacity` 实现，否则会同时降低弹窗文字和控件对比度；只调整 backdrop 的 alpha。
- 当前浅色叠层会把主内容洗成近似不透明灰色，因此移除复合浅色底，改用单一半透明中性色；保留 `backdrop-filter` 和无滤镜时仍可辨识弹窗层级的底色。
- 不改变上一轮已经验证的 `.workspace-stage` 挂载和响应式边界，避免视觉调整重新影响 Header 与侧边栏。

## 验证范围

- 共享 `Modal.svelte` 的普通 dialog 与 drawer 遮罩。
- 桌面及 390px 窄屏；检查遮罩计算色、透明度、范围、命中关系和弹窗内容可读性。
- `pnpm --dir web check`、生产构建、`git diff --check`、Impeccable/Finesse 检测和浏览器错误日志。

## 实施与验证结果

- 普通 dialog 遮罩改为单层 `rgba(24, 38, 51, 0.28)`，毛玻璃为 `blur(16px) saturate(112%)`；drawer 使用更轻的 `rgba(24, 38, 51, 0.18)`，弹窗容器自身底色未变。
- 1280×720 浏览器计算样式与源码一致，遮罩仍以 `.workspace-stage` 为父节点，矩形为 `x=270, y=112.5, width=1010, height=607.5`；与侧栏、Header 重叠均为 0，弹窗完整位于遮罩内。
- 390×900 时遮罩为 `x=0, y=127, width=390, height=773`，计算色和 blur 与桌面一致；Header 重叠为 0，弹窗为 `x=12, width=366`，文档宽度保持 390。
- 视觉截图确认底层内容轮廓可辨、弹窗正文仍保持清晰实色。关闭弹窗后焦点返回“创建版本”，浏览器错误日志为空。
- `pnpm --dir web check`、生产构建和 `git diff --check` 通过；Impeccable detector 返回 `[]`，Finesse 为 `p0: 0` 且共享 Modal 无 findings。
- 浏览器使用本地调试会话；后端代理目标 `127.0.0.1:18083` 当时不可用，因此仅验证共享弹窗视觉、响应式和交互，不声称业务数据加载通过，也未提交任何业务写入。
- **Status:** complete

---

# 2026-07-31 任务跟踪全菜单事实链统一与最强大脑式重构

## 目标与事实边界

- [x] 流转看板统一读取服务端排期事实，排期确认后原地刷新并正确进入已排期；需求、缺陷等 Work Item 使用同一口径，不再只统计 `demand`。
- [x] 项目看板在 Jira 状态变化后自动刷新，同时保留当前项目、展开项和用户操作上下文。
- [x] 证据健康总表改为“异常优先的事实表 + 单一检查器”，删除无决策价值的重复卡片和装饰层。
- [x] 任务表、执行追踪、人员负载共享同一执行任务读取模型、权威项目目录和稳定多选控件。
- [x] 执行任务不因“非核心成员”被误删；项目偏好只控制展示范围，不承担任务归属和权限语义。
- [x] 人员负载只展示可解释的任务、风险和证据计数，不再使用没有容量基准的伪百分比。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：把六个问题视为一个产品工作台的信息架构和组件所有权缺陷。页面只能消费对应领域的唯一事实源；筛选、加载、空态、错误和刷新状态必须稳定且可见。高密度页面采用一条摘要带、一个主表/泳道和一个上下文检查器，避免重复卡片与嵌套容器。
- **design-taste-frontend**：高密度后台不属于该技能的主设计范围，因此只采用 redesign-preserve 审计约束。保留既有浅色治理后台、导航、路由、令牌和交互语言，不引入营销页结构、新字体、新依赖或脱离产品体系的视觉主题。
- **finesse-ui**：Design Read 为“研发交付治理后台 · 证据驱动、表格优先、单一事实源”，`register=product`、`SOUL=4 / SPECTACLE=1 / DENSITY=9`。用真实字段、对齐表格、克制层级和反馈型动效表达状态；重复行为由共享组件承担，页面不得自行实现浮层定位或项目目录。

### 分歧与取舍

- finesse 的品牌纹理和强 substrate 建议与当前 Phase 41 浅色、表格优先基线冲突，本轮不增加纹理、渐变或装饰动效，只保留紧凑层级和清晰状态。
- 不为“最强大脑式”增加新的导航层或大屏驾驶舱；它在本产品中定义为事实统一、异常优先、可追溯和低操作摩擦。
- 证据健康、执行追踪和人员负载不共享同一视觉组件，但共享筛选目录、稳定浮层和刷新生命周期；领域数据不在前端互相推导。

## 组件所有权

- `/api/schedule`：Work Item 排期事实、泳道分类和排期汇总的唯一读取模型；前端不再以 `issue_type === demand` 推导总量。
- `/api/execution/tasks`：任务表、执行追踪和人员负载的唯一执行任务读取模型，同时返回当前用户范围内的权威项目和负责人筛选目录。
- `/api/projects/scores`：证据健康事实；页面只负责排序、筛选和检查器呈现，不重新计算分数。
- `MultiSelect.svelte`：多选浮层定位、视口约束、焦点和连续选择的唯一所有者；业务页只绑定值。
- `ProjectBoard.svelte`：Jira 只读投影的轮询、窗口恢复刷新和并发请求防回退的生命周期所有者。

## 回归与验证范围

- Go 回归覆盖：排期写入后的服务端泳道事实、非核心本地执行任务可见、项目偏好范围、项目/负责人筛选目录和无伪项目键。
- 前端检查：共享多选不因选项变化产生页面位移；三个任务视图不重复请求、不在筛选时重置数据；项目看板自动刷新但保留选择态。
- 认证浏览器覆盖：桌面和窄屏下的流转看板排期前后、Jira 投影刷新、证据健康正常/异常/空态、任务表、多选连续操作、执行追踪和人员负载。
- 验证使用隔离数据库和 Mock Jira；不连接或写入生产 Jira、生产数据库。

## 当前取证

- 本地库共有 690 条活跃工作项，其中 `demand` 只有 31 条（待办 2、进行中 0、完成 29）；`requirement` 有 130 条、`bug` 有 517 条。现有流转看板仅保留 `issue_type === demand`，直接造成截图中的 2 / 0 / 0 / 17。
- 本地库有 12 条执行任务，但接口按核心成员再次过滤后只剩 3 条；任务表显示三条并非真实任务总量。
- 12 条执行任务的 `project_key` 和父工作项字段均为空，兼容逻辑从任务 ID 前缀推断项目，产生 `MIDDLEQ`、`ACTUAL`、`WITH` 等伪项目甚至人名式选项。
- `TaskKanban` 虽复用同一个多选组件，却仍从当前响应反推项目目录，并在每次多选变化时重新请求；绝对定位浮层随内容重排，形成鼠标下的页面跳动。
## 实施与验证结果

- `/api/schedule` 和流转看板统一使用规范化 Work Item 类型与服务端 `scheduled` 事实。隔离登录态浏览器中，确认 `HIT-101` 排期后无需刷新，待排期从 1 变 0，已排期从 2 变 3，并出现右上角成功 Toast。
- 项目看板增加 15 秒静默轮询、窗口聚焦/恢复和 Jira 同步事件刷新，并用请求序号阻止旧响应回退。隔离库把 `NS2-201` 从 backlog 更新为 progress 后，页面自动从待办移到处理中，当前项目仍保持“南沙二期”。
- 执行任务接口返回权威项目和负责人 facets。任务表、执行追踪、人员负载只请求同一执行任务读模型；本地任务不再被 Jira 核心成员过滤，外部 Jira 任务仍按同步成员范围隐藏，任务 ID 前缀不再生成伪项目。
- 共享 `MultiSelect` 把浮层 portal 到 `body` 并按触发器和视口固定定位。桌面连续选择项目/负责人时 `scrollY`、触发器和浮层坐标均不变；760px 下浮层位于视口左右边界内，选择后页面滚动位置不变。
- 证据健康总表只保留一条决策摘要、一个异常优先事实表和一个检查器；健康区间由综合分与最弱维度共同判定，避免“质量 50% 高危”仍显示稳定。隔离数据识别出 2 个红区项目，红区筛选只显示对应 2 行。
- 人员负载改为总任务、活跃、缺证据、高风险、涉及项目和行动状态；不再显示无容量基准的伪百分比或伪 Bug 数。隔离页面正确显示 4 名成员、4 条执行任务和 4 个项目范围。
- 数据层新增 Work Item 类型/状态/项目/负责人复合索引、父项/任务组索引和代码证据时间倒序索引；项目评分器只接受已配置或 Jira 同步目录中的权威项目，不再落库 `WITH`、`MIDDLEQ` 等任务编号前缀。
- `GOCACHE=/tmp/well-ambient-go-cache go test ./... -count=1` 全量通过；`pnpm --dir web check` 为 0 error、79 个既有 warning，生产构建和 `git diff --check` 通过。Impeccable 精确文件检测返回 `[]`；Finesse 为 P0=0，仅报告既有大文件中的纯白色 P2。
- 登录态浏览器在 1440×900 和 760×900 检查了流转排期即时移栏、项目 Jira 投影自动刷新、任务表完整目录、项目/负责人连续多选、执行追踪、人员负载和证据健康红区；两种宽度均无文档级横向溢出，浏览器错误日志为空。
- 验证使用关闭 Jira、GitLab、飞书和 AI 的隔离 SQLite 服务；没有连接或写入生产 Jira、生产数据库。
- **Status:** complete

---

# 2026-07-31 版本计划批量 Jira 下拉完整显示

## 目标与保护边界

- [x] 复现批量 Jira 下拉被右侧版本检查器裁切，并建立视口边界与滚动可达性断言。
- [x] 让版本计划复用共享 `MultiSelect` 的 portal、向上/向下自适应和内部滚动能力。
- [x] 保持 Jira 候选范围、搜索、连续多选、禁用态和批量关联写入语义不变。
- [x] 完成桌面与窄屏登录态浏览器验证，以及前端检查、构建和 UI 检测。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：下拉属于 overlay，不应作为右侧检查器滚动内容继续参与排版。浮层必须逃逸 `overflow-y: auto`，按触发器与视口计算方向和最大高度，并把长列表滚动限定在选项区。
- **design-taste-frontend**：该技能不主导高密度后台产品页，仅采用 redesign-preserve 约束。保留 Phase 41 浅色表格工作台、现有字号、色彩、圆角、文案和领域流程，不增加新组件库或无关视觉重构。
- **finesse-ui**：Design Read 为研发治理产品工作台，`register=product`、`SOUL 4 / SPECTACLE 1 / DENSITY 9`。版本页只映射 Jira 候选；共享多选组件拥有浮层几何、焦点、连续选择与滚动状态。

### 分歧与取舍

- taste-skill 明确不负责 dashboard/data table 产品 UI，因此不采用其营销页布局与动效建议；本次只使用其保留现有设计系统和避免无关重构的约束。
- 不为版本面板单独复制一套 Jira 下拉 CSS，也不调整检查器高度。优先接通已经具备 portal 和视口定位能力的共享多选模式。
- 长 Jira 标题允许在选项内自然换行；完整显示依靠受控选项区滚动，而不是无限增高浮层或扩大页面滚动范围。

## 组件所有权与验证范围

- `DeliveryPlan.svelte`：负责启用共享 overlay 模式和映射 Jira 候选，不拥有浮层坐标。
- `MultiSelect.svelte`：继续作为 portal、上下翻转、视口边距、最大高度、焦点和连续选择的唯一所有者。
- 浏览器回归检查：浮层挂载到 `body`，矩形完整位于视口内；靠近底部时向上展开；首末候选可通过内部滚动访问；选择前后页面与检查器滚动位置不跳动；桌面和窄屏无文档级横向溢出。

## 当前诊断

- 版本计划已经使用共享 `MultiSelect`，但该调用没有开启 `overlay`。因此候选列表以普通文档流元素渲染在 `.inspector-body { overflow-y: auto; }` 内，截图中的底部候选被检查器和视口共同裁切。

## 实施与验证结果

- `DeliveryPlan.svelte` 的批量 Jira 多选启用共享 `overlay` 模式；未修改候选接口、项目继承、搜索、禁用态、选择值或批量关联请求。
- 修复前 1440×900 红灯：浮层为 `position: relative`，父节点是 `.multi-select-wrapper`，底部为 `953.6px`，超过 `900px` 视口并超过 `871px` 检查器底部。
- 修复后 1440×900：浮层 portal 到 `body`、定位为 `fixed`，因下方空间不足自动向上展开，矩形为 `x=1087.8, y=380.6, width=309.2, height=261`，四边均位于视口安全边距内。
- 48 条 Jira 候选的选项区保持内部滚动；滚动到 `scrollTop=3556.5` 后最后一项完整可见。选择该项后文档滚动、检查器滚动和选项滚动位置均未变化。
- 760×900 与 390×900 均通过 portal、视口边界和横向溢出断言；390px 下浮层宽 312px、选项区高 192px，页面横向溢出为 0。浏览器错误日志为空。
- `pnpm check`、`pnpm build` 和精确 `git diff --check` 通过；Impeccable 检测返回 `[]`，Finesse 检测为 `p0: 0` 且目标文件无 findings。检查仍输出仓库其他大组件的既有 Svelte warning，本次文件未新增错误或告警。
- 浏览器使用关闭 Jira、GitLab、飞书和 AI 的临时数据库副本；只执行本地候选选择，没有提交批量关联或任何生产写入。
- **Status:** complete

---

# 2026-07-31 版本计划批量多选高度稳定

## 目标与保护边界

- [x] 复现 Jira 选择数量增加后触发器与右侧检查器持续增高，并建立固定高度断言。
- [x] 批量 Jira 选择改为固定高度摘要，不再把每个已选事项逐条回填到触发器。
- [x] 保持候选范围、搜索、连续多选、清除、禁用态、浮层视口约束和批量关联语义不变。
- [x] 完成桌面与窄屏登录态浏览器验证，以及前端检查、构建和 UI 检测。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：批量选择器的主控件必须是稳定锚点。已选数量不应改变控件或页面几何；完整事项仍在浮层内查看，收起时只显示一个可读摘要。
- **design-taste-frontend**：该技能不主导高密度后台产品页，仅采用 redesign-preserve 约束。保留 Phase 41 浅色治理工作台、既有字段顺序、色彩、字号和交互语言，不引入新视觉系统。
- **finesse-ui**：Design Read 为研发治理产品工作台，`register=product`、`SOUL 4 / SPECTACLE 1 / DENSITY 9`。批量输入采用固定高度、数量摘要和受控 overlay，避免选择反馈引发累计布局位移。

### 分歧与取舍

- 不在共享组件中全局启用摘要模式；其他业务场景可能仍需要直接查看和移除少量标签。由 `DeliveryPlan.svelte` 为批量 Jira 场景显式选择摘要呈现。
- 不限制可选 Jira 数量，也不截断候选数据来换取布局稳定；完整选择状态仍由共享组件管理，关闭浮层后只压缩其视觉表达。
- 保留上一轮已验证的 `overlay` 模式。摘要负责稳定触发器高度，overlay 负责稳定浮层边界，两者分别解决不同层级的问题。

## 组件所有权与验证范围

- `DeliveryPlan.svelte`：声明批量 Jira 选择使用摘要模式并绑定选择值。
- `MultiSelect.svelte`：继续拥有固定高度摘要、候选浮层、内部滚动、搜索、清除、焦点和连续选择。
- 浏览器回归检查：选择 0、1、多条 Jira 时触发器高度不变；文档高度和检查器滚动范围不随已选数量累计增长；浮层仍在视口内，桌面和窄屏无横向溢出。

## 当前诊断

- 批量 Jira 调用沿用了默认标签模式。已选的长 Jira 标题逐条渲染为 `.selection-chip`，而触发器允许 `flex-wrap: wrap`，因此选择数量越多，控件和检查器滚动内容越高。
- 1440×900 红灯中，0 条时触发器为 38px、检查器滚动内容为 556px；12 条时分别增长到 422px 和 940px，“批量关联”被向下推移 314px。
- 清空后保持浮层打开，检查器滚动内容立即恢复到 556px；只选择一条长 Jira 就使触发器增到 70px，排除 overlay 定位并把最小复现缩到一个选择。

## 实施与验证结果

- `DeliveryPlan.svelte` 为该批量 Jira 调用显式启用共享 `summaryMode`；没有修改共享默认值、候选数据、搜索、禁用态、清除、关联请求或 Jira 领域语义。
- 1440×900 下连续选择 12 条后，触发器稳定为 36px、检查器滚动内容稳定为 554px、文档高度保持 900px；浮层继续完整位于视口安全边距内。
- 390×900 下继续选择到 30 条，触发器仍为 36px、检查器高度仍为 608px，收起后显示“已选 30 项”；浮层为 `x=28..340, y=389.1..640.1`，页面横向溢出为 0。
- 桌面收起态和窄屏展开态截图均已检查，层级、间距、对齐、长标题换行、操作区和内部滚动正常；浏览器错误日志为空。
- `pnpm check` 通过（0 error、79 个既有 warning），`pnpm build` 通过；Impeccable 返回 `[]`，Finesse 为 `p0: 0` 且目标文件无 findings，目标文件和计划文件的空白检查通过。
- 验证使用关闭 Jira、GitLab、飞书和 AI 的隔离数据库；只改变本地选择状态，没有点击批量关联，也没有生产写入。
- **Status:** complete

---

# 2026-07-31 排期检查器重排与任务跟踪目录一致性

## 目标与保护边界

- [x] 排期看板右侧检查器恢复“主表、辅检查器”的视觉层级，内容高度和滚动由检查器自身承担。
- [x] 排期编辑按记录身份、事实、编辑主任务、保存、验收与风险组织；长内容、失败提示和窄屏不得被裁切。
- [x] 任务表、执行追踪、人员负载继续共享一个执行任务读取模型，负责人目录合并权限内成员与可见任务负责人。
- [x] 项目目录保持权威 Jira/项目配置与个人项目范围，不从任务 ID 或当前可见行反推。
- [x] 完成服务端回归、前端检查/构建、UI 检测与登录态多断点浏览器验证。

## 三方 UI 评审结论（编辑前门禁）

### 共同方向

- **Impeccable**：当前主要缺陷是右栏在超宽屏占约 44% 且与长表强制等高，上半区又通过 30px 控件、单行裁切和连续五等分网格硬塞内容。桌面应使用流动主表 + `clamp(420px, 34vw, 560px)` 辅检查器并顶部对齐；检查器拆成固定 header/tabs 与可滚动 body，按 4/8/12/16/24px 建立紧松节奏。
- **design-taste-frontend**：该技能不主导数据表型产品 UI，仅采用 redesign-preserve。保留现有 Phase 41 浅色治理后台、导航、字段、状态颜色和组件库；不引入营销页层级、新字体、新依赖或新视觉主题。
- **finesse-ui**：Design Read 为“研发交付治理工作台 · 事实优先、紧凑清晰”，`register=product`、`SOUL 4 / SPECTACLE 1 / DENSITY 9`、`hero-engine=none`。记录身份和排期编辑是主任务，保存是唯一 primary action；AI 操作、验收和风险是次级组，不与主操作同权。

### 分歧与取舍

- Impeccable 建议把检查器抽成独立 `ScheduleInspector.svelte`。本轮工作树已有大量未提交领域改造，为避免扩大回归面，先在 `DemandKanban.svelte` 收敛最终语义结构与级联；组件拆分作为后续深模块整理，不阻塞这次可见问题。
- 不用继续缩小字号、控件和文本行数来换取“首屏无滚动”。桌面保持产品密度，但动态错误、长标题、验收项和风险内容可自然增高，并仅由 inspector body 滚动。
- 机械 detector 返回空数组，但扫描发现大量 3/5/6/7/9/10/11/14px 微差值。最终生效的检查器规则统一回现有 4pt token scale；不为历史已被覆盖的旧声明做无关全文件清理。

## 组件所有权与响应式

- `DemandKanban.svelte`：继续拥有需求选择、排期草稿、保存和 AI 回调；本轮只重排右侧检查器 markup 与最终样式，不改变写入语义。
- `CommitTelemetryPanel.svelte`：继续拥有代码轨迹内容；排期检查器只提供 tab 容器，不复制轨迹数据或刷新逻辑。
- `/api/execution/tasks.facets`：三个任务跟踪页面的唯一筛选目录。项目使用当前用户范围内的权威配置；负责人合并配置/成员目录与可见的本地执行负责人。
- `TaskKanban.svelte`：三个视图继续消费同一份 facets；不得为单个子页再建局部项目/负责人数组。
- `>=1281px`：主表流动，检查器宽 420–560px、顶部对齐、内容高度驱动。
- `761–1280px`：保持 table-first 单列，检查器紧随表格；事实与表单保持可读列数。
- `<=760px`：事实两列、字段单列、保存全宽、风险筛选可换行或受控滚动，触控目标至少 44px，页面无横向溢出。

## 当前诊断与红灯

- 服务端测试已构造已配置且可见、但暂无执行任务的 `Reviewer`。修复前 `/api/execution/tasks.facets.assignees` 只有 `Alice / Operator / Vendor`，缺少 `Reviewer`，证明负责人 facets 错把当前执行行当成完整目录。
- 项目 facets 已由 `availableProjectPreferenceOptions` 和个人项目范围生成，不再使用任务 ID 前缀；本轮保留该权威边界并做页面/API 对照验证。
- 前端三个子页都读取 `executionAssigneeFacets`、`allProjects`，未发现独立接口分叉；负责人差异的首要根因在服务端目录合成，而不是三个页面各自请求。

## 验证范围

- Go：负责人目录包含无执行任务的已配置成员，同时保留可见本地执行负责人；项目配置、个人范围和无伪项目键回归保持通过。
- 前端：三个任务视图的项目/负责人选项集合一致；切换视图、多选和刷新不重置目录或引起页面跳动。
- 浏览器：排期检查器覆盖排期设置/代码轨迹、长标题、保存错误/空态可达性；1440、1024、760、390 宽度及短高度检查唯一滚动归属、宽度上限、无横向溢出和键盘 tab 语义。
- 所有浏览器验证使用关闭 Jira、GitLab、飞书和 AI 的隔离 SQLite；不保存排期、不写生产系统。

## 实施与验证结果

- 排期检查器在桌面改为 `minmax(580px, 1fr) + clamp(420px, 34vw, 560px)` 的主表/辅栏关系并顶部对齐；检查器由内容决定高度，排期与轨迹 body 各自滚动，不再被长表强制拉伸。
- 排期事实调整为桌面 3+2、窄屏 2+2+1；编辑字段在桌面两列、窄屏单列。AI 辅助操作与唯一主操作“保存排期”分组，风险日历在窄屏改为 3+2，所有关键触控控件至少 44px。
- 排期设置/代码轨迹标签补齐 tablist、roving tabindex、方向键/Home/End 和 tabpanel 关联；浏览器已验证键盘切换后焦点、选中项及面板标签一致。
- 执行任务负责人 facets 合并权限内用户目录、配置成员和当前可见本地任务负责人；`zhiyuan.liang` 等 Jira 登录名通过共享目录规范化为中文显示名，且保留没有目录映射的本地负责人。
- 项目 facets 继续使用当前用户范围内的权威项目配置。浏览器确认任务表、执行追踪、人员负载三页的 5 个项目选项完全一致，11 个负责人选项也完全一致；负责人连续多选时触发器、浮层、页面高度和滚动位置均未跳动。
- 登录态浏览器在 1440×900、1024×900、760×900 和 390×900 检查了排期设置、代码轨迹、内部滚动、操作区、风险区和响应式堆叠；各断点文档级横向溢出为 0，760/390 下主表内部滚动且与检查器保持 16px 间距。
- 新开干净浏览器页重新进入排期看板后控制台 error/warning 为空；验证期间未保存排期，也未连接或写入生产 Jira、生产数据库。
- `GOCACHE=/tmp/well-ambient-go-cache go test ./... -count=1` 全量通过；`pnpm --dir web check` 为 0 error、79 个既有 warning；生产构建和精确 `git diff --check` 通过。
- Impeccable 精确文件检测返回 `[]`；Finesse 为 P0=0，仅报告该历史大文件中的既有纯白色 P2，不属于本次检查器规则。
- **Status:** complete

---

# 2026-07-31 任务表负责人 core member 边界纠偏

## 缺陷契约

- [x] 任务表负责人下拉只能显示 core member 目录，不得因为某条 Work Item 当前由外部协同人员负责而把该人员加入可选候选。
- [x] `jira.sync_users` 非空时是 core member 的唯一配置来源；自定义 JQL 中的负责人仅在 `sync_users` 为空时作为回退。
- [x] Work Item 行仍可如实显示当前负责人，不因候选目录收窄而隐藏事项或篡改事实。
- [x] 执行追踪与人员负载继续使用后端已过滤的 Execution Task facets，不与状态视图的候选目录混用。

## 三方 UI 评审结论

- **Impeccable**：这是可执行候选权限边界，不是展示事实边界；下拉必须只提供合法候选，行内事实保持原样。
- **design-taste-frontend**：该技能不主导后台数据表，仅采用 redesign-preserve；不改变现有下拉外观、密度、层级或响应式结构。
- **finesse-ui**：Design Read 为“研发交付控制台，克制、事实优先，register=product，SPECTACLE=1，DENSITY=8”；只修正数据源，不增加控件或视觉噪声。
- **共同方向**：复用 `/api/demands/options` 的 core member 目录，并在后端收敛 `sync_users` 优先、JQL 回退的单一规则。
- **无分歧项**：三方均不建议从 Work Item 历史负责人反推可选人员，也不建议通过前端黑名单补丁隐藏个别人名。

## 验证范围

- 后端回归测试覆盖 `sync_users` 优先、JQL 回退、普通数据库用户和 Work Item 外部负责人不得泄漏。
- 前端静态契约覆盖任务表读取 core member 目录且不再从 Work Item 列表派生负责人候选。
- 登录态浏览器用隔离夹具同时注入 core member、JQL 外部人员和 Work Item 外部负责人，验证下拉只显示 core member，行内仍显示真实负责人。
- 前端检查、生产构建、Impeccable / Finesse 检测及差异卫生。

## 实施与验证结果

- 任务表状态视图已停止从 Work Item 行负责人派生候选，改为读取 `/api/demands/options` 的 core member 目录；目录刷新时会移除已经失效的筛选值。
- 后端候选目录已复用中央 `configuredKPICoreMembers` 规则：`sync_users` 非空时不再合并自定义 JQL 负责人，只有配置为空时才使用 JQL 回退。
- 后端回归覆盖了普通用户、Work Item 外部负责人、JQL 外部负责人不得泄漏，并保留 JQL 回退场景。
- 隔离登录态浏览器中，任务表行继续显示 `外部协同乙` 的真实负责人事实，负责人下拉唯一候选为 `核心成员甲`。
- `go test ./internal/server`、前端检查、生产构建、静态契约、Impeccable、Finesse P0 与差异卫生均通过；本地夹具和预览已停止并清理。
- **Status:** complete
