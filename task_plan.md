# Task Plan: Implementation Plan Check and Fix

## Goal
Inspect the repository against the provided implementation plan and apply the needed fixes that fit the current codebase, including the follow-up fixes for frontend skill rules, department display, demand scheduling date styling, and AI deconstruction binding.

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

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Initial file index missed `internal/server` | 1 | Used `find` and targeted reads to locate ignored backend files. |
| `go test` could not write the default Go build cache under sandbox | 1 | Re-ran with user-approved elevated test command. |
| Frontend build failed on invalid `{@const}` placement in `TaskKanban.svelte` | 1 | Removed the duplicate nested const in the Done lane. |
| `apply_patch` context failed after `gofmt` | 1 | Re-read current snippets, applied smaller-context patch, and recorded the tooling lesson. |
