# Findings & Decisions

## Requirements
- Check and repair the repository according to `/Users/eddie/.gemini/antigravity/brain/ffc6d8a3-38c0-4a9b-a458-9b51beeff27b/implementation_plan.md`.
- Planned areas: confirmation modal styling, WellOS department extraction, select dropdown truncation, AI deconstruction to demand association, demand card shadow task progress, targeted tests/builds.
- Follow-up requirements: require `taste-skill` for frontend UI work, explain/deepen demand scheduling and AI deconstruction binding, fix user department display, and fix the native-looking new-demand due-date style.

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

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| Initial discovery suggested missing backend server | Confirmed the files exist via `find`; proceed with direct reads. |
| Frontend build failed on `TaskKanban.svelte` const placement | Removed the duplicate const from inside the card markup. |
| Svelte build reports existing a11y warnings | Build succeeds; warnings remain outside this plan's blocking scope. |
| Earlier session lacked `taste-skill` availability | Wrote the required rule and recorded `frontend-design` as the fallback for future unavailable sessions; latest demand UI fixes used the requested `design-taste-frontend` skill. |
| `apply_patch` failed due formatted context drift | Re-read snippets, patched with smaller context, and logged the tooling lesson. |

## Resources
- Source plan: `/Users/eddie/.gemini/antigravity/brain/ffc6d8a3-38c0-4a9b-a458-9b51beeff27b/implementation_plan.md`
