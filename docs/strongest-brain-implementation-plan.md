# Strongest Brain Implementation Plan

## Goal

Build two durable foundations for well-ambient:

1. AI demand deconstruction receives a compact, trustworthy, auditable context pack that describes current architecture, workflow, feature boundaries, estimation rules, and relevant delivery history.
2. Authorization evolves from coarse role groups into explainable policy-based access control with finer permissions, resource scopes, and audit-ready decisions.

The implementation must keep context compressed, cacheable, and fast. It must also keep permission decisions understandable to administrators.

## Non-Negotiable Principles

- Context is structured system knowledge, not a larger prompt textarea.
- Permission is `subject + action + resource + scope + condition`, not only a role name.
- Every AI deconstruction should be able to explain which context it used.
- Every authorization denial should be diagnosable by an administrator.
- Existing user work and existing coarse RBAC behavior must remain compatible during migration.

## Current Baseline

- AI prompt context is currently assembled from four AI config fields: architecture, workflow, implemented features, and estimation guidelines.
- AI deconstruction archives already persist model output and estimates, but not the context pack used for a run.
- RBAC currently uses users, user groups, atomic permissions, group permissions, and `global/repo` scoped memberships.
- Route authorization is handled by `withPermission(requiredPermission, handler)`, which queries permission membership directly.
- Settings UI exposes user/group membership and a group permission matrix, but does not explain effective permissions or policy decisions.

## Target Architecture

### AI Context Registry

Core tables:

- `context_documents`: source-level records for system knowledge.
- `context_facts`: normalized fact cards extracted from documents or manual entries.
- `context_chunks`: compressed prompt-ready text with hash and token metadata.
- `context_packs`: one generated context package per deconstruction run or preview.
- `context_pack_items`: join table recording exactly which facts/chunks entered a pack.

Core fields:

- `type`: `architecture`, `workflow`, `feature_boundary`, `estimation_rule`, `glossary`, `risk_rule`, `delivery_history`.
- `scope`: `global`, `repo`, `module`, `demand_type`.
- `scope_id`: empty for global, otherwise repo/module/demand type identifier.
- `source`: `manual`, `config`, `gitlab`, `jira`, `archive`, `doc`.
- `owner`, `status`, `version`, `content_hash`, `summary`, `token_count`, `freshness`, `confidence`.

Context pack assembly:

1. Parse the incoming demand into a scope signature.
2. Select active context facts by scope and simple keyword overlap.
3. Rank by scope match, type priority, freshness, confidence, and text overlap.
4. Fit selected facts into a fixed token budget.
5. Persist the generated pack and item list.
6. Inject only the pack summary into the AI prompt.
7. Store `context_pack_id` on the deconstruction archive.

Compression and cache strategy:

- L0 fixed system prompt remains stable.
- L1 global context has high cache hit rate and changes only when global facts change.
- L2 repo/module context is selected by scope signature.
- L3 delivery history is optional and limited to relevant estimate samples.
- Context pack cache key should include model, prompt template version, context hashes, scope signature, and work-hours setting.

### Policy-Based RBAC

Core tables:

- Existing `users`, `user_groups`, `permissions`, `group_permissions`, `user_group_memberships` remain compatible.
- `authorization_policies`: allow/deny policy records for action/resource/scope.
- `authorization_audit_logs`: records evaluated high-value authorization decisions.

Policy fields:

- `effect`: `allow` or `deny`.
- `subject_type`: `group`, `user`, or `any`.
- `subject_id`: group name, username, or empty.
- `action`: permission/action code.
- `resource_type`: `global`, `config`, `repo`, `project`, `demand`, `user`, `group`.
- `resource_id`: optional resource identifier.
- `scope`: `global`, `repo`, `project`, `demand`, `department`, `owner`.
- `scope_id`: optional scope identifier.
- `condition_json`: reserved for v2 conditions.
- `priority`, `enabled`, `reason`.

Authorization contract:

```go
Authorize(ctx, subject, action, resource) Decision
```

Decision includes:

- `allowed`
- `reason`
- `matched_policy`
- `missing_permission`
- `scope`
- `risk_level`

Compatibility:

- Existing group permissions remain the default allow source.
- Explicit deny policies override group allows.
- `super_admin` remains a break-glass role.
- Existing `withPermission` routes continue working, but delegate to the policy evaluator.

## Implementation Phases

### Phase 1: Plan And Schema

Deliverables:

- This implementation plan.
- DB models and migrations for context registry, context packs, authorization policies, and authorization audit logs.
- Seed fine-grained permissions for AI context and policy management.

Acceptance:

- App starts with auto-migration.
- Existing roles still receive compatible baseline permissions.

### Phase 2: AI Context Registry API

Deliverables:

- List/create/update context facts.
- Preview context pack for demand text.
- Context pack assembly service with deterministic ranking and token budgeting.

Acceptance:

- Admin can manage active global context facts.
- API can return a compact context pack preview for a sample demand.

### Phase 3: AI Deconstruction Integration

Deliverables:

- Deconstruction prompt uses the generated context pack instead of direct config field assembly.
- Import/archive stores `context_pack_id`.
- Legacy AI config fields are mirrored into default context facts when no registry data exists.

Acceptance:

- Deconstruction still works if no context facts were created.
- Archive records the context pack used by each run.

### Phase 4: Policy Evaluator

Deliverables:

- Central authorization evaluator.
- `withPermission` delegates to evaluator.
- Explanation endpoint for current user/action/resource.
- Audit log records denied or high-risk decisions.

Acceptance:

- Existing route tests pass.
- Repo-scoped membership still works.
- Explicit deny policy can block an otherwise allowed action.

### Phase 5: Admin UI

Deliverables:

- AI config page gains a context registry panel with facts, status, scope, and context pack preview.
- Settings security panel gains policy list, effective permission explanation, and high-risk action labeling.

Acceptance:

- UI keeps current dark admin style.
- Empty, loading, success, and error states are visible.
- Admin can understand why a permission exists or is denied.

### Phase 6: Validation And Feedback Loop

Deliverables:

- Unit tests for context pack selection and policy evaluation.
- Full Go regression and frontend production build.
- Task memory update and committed changes.

Acceptance:

- `go test ./...` passes.
- `pnpm --dir web build` passes.
- Final delivery lists validation gaps and known unrelated dirty files.

## First Cut Scope

In the first implementation pass, do not build:

- Embedding search.
- External document ingestion.
- Multi-step approval workflow.
- Full condition expression language.
- Periodic access review.

Instead, ship the v1 contract, persistence, deterministic context selection, prompt integration, policy evaluator, compatibility layer, and usable admin surfaces.

## Ownership Split

- AI Context Backend: DB models, context registry service, context APIs, deconstruction integration, tests.
- RBAC Backend: policy models, evaluator, route integration, seed permissions, tests.
- Frontend Admin UI: context registry management, context preview, policy management, permission explanation.
- Main Integrator: plan document, integration review, conflict resolution, validation, task memory, commit.
