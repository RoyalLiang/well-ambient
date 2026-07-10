# Controlled Autonomous Delivery Implementation Plan

## 1. Objective

Extend well-ambient from AI-assisted demand analysis into a controlled autonomous delivery system. The system must keep humans at explicit decision gates while allowing AI to perform bounded implementation work: understand and deconstruct a demand, accept human corrections, freeze an executable specification, create isolated branches, commit generated changes, run required checks, open Draft merge requests, resolve reviewers, collect acceptance, and produce knowledge candidates from the completed delivery.

This is an extension of the existing demand, shadow-task, context-pack, authorization, GitLab telemetry, and AI review foundations. It is not a new task system and it must not revive the removed strongest-brain cockpit.

## 2. Target Lifecycle

```text
captured
  -> intent_recognized
  -> clarification_required | deconstruction_draft
  -> human_reviewed
  -> frozen
  -> preflight_ready | preflight_blocked
  -> executing
  -> tests_failed | draft_mr_created
  -> review_pending
  -> acceptance_pending
  -> delivered | rejected | cancelled
  -> corpus_candidate_created
```

The demand remains the commitment object. AI-generated subtasks remain execution/evidence objects linked through `task_group_id`.

## 3. Domain Contracts

### DemandSpecVersion

Immutable versioned snapshot of the executable requirement.

- demand identity and source text
- recognized intent, confidence, facts, inferences, missing context
- user goal, business rules, main and exceptional flows
- permissions, data/API/UI impact, dependencies and risks
- acceptance criteria and test plan
- mapped repositories and normalized execution tasks
- context pack, model, rule version and provenance
- lifecycle status, author, reviewer, frozen by/at

Only the latest frozen version can authorize a new execution run. Editing a frozen version creates a new draft version and invalidates unstarted runs based on the older version.

### ReviewContract

Frozen with the demand spec and re-evaluated against the real diff.

- required review roles
- reviewer candidate identities
- business acceptance owner
- minimum approvals
- protected path rules
- segregation-of-duty rules
- review SLA and escalation owner
- resolution status and reason

The contract determines who must review. A runtime resolver determines the final people using changed paths, repository ownership, availability, and conflict-of-interest rules.

### ExecutionRun and ExecutionAction

`ExecutionRun` is the idempotent orchestration root for one frozen spec version. `ExecutionAction` is its append-only audit trail.

- frozen spec version and task group
- repository, topic branch, base branch and provider
- initiator, service identity and idempotency key
- current state, block reason and timestamps
- commit SHA, MR IID/URL, pipeline state and acceptance state
- ordered actions with request hash, result, external reference and error

### CorpusCandidate

Delivery-derived candidate for future AI context or evaluation data.

- source run/spec/context pack
- candidate type: case, rule, architecture, workflow, estimate, risk, acceptance
- proposed scope and normalized content
- provenance, confidence, sensitivity and expiry
- reviewer disposition: pending, accepted, rejected, superseded
- accepted candidates create or update versioned context facts; pending candidates never enter production context packs

## 4. Safety and Authorization

- `demand_spec:read`, `demand_spec:write`, `demand_spec:freeze`
- `review_contract:manage`, `review_contract:resolve`
- `execution:preflight`, `execution:start`, `execution:cancel`
- `execution:accept` for the named business acceptance owner
- `corpus_candidate:read`, `corpus_candidate:review`

Execution is blocked unless:

1. the demand and frozen spec exist;
2. required information and acceptance criteria pass readiness thresholds;
3. a review contract is approved and resolvable;
4. repository/base branch configuration is allowed;
5. the service identity cannot write to protected branches;
6. required test commands and rollback notes exist for risky changes;
7. no active run already owns the same demand/spec/repository idempotency key.

## 5. GitLab Automation Boundary

The adapter uses the configured GitLab API with least privilege.

1. Resolve project and default/base branch.
2. Create `ai/<demand-id>/<task-or-run-slug>` from the base ref.
3. Apply only explicitly generated file actions through the repository commit API.
4. Create a Draft MR with spec, context, tests and trace references.
5. Assign resolved reviewers and labels.
6. Read pipeline/MR state through webhook evidence and explicit status refresh.

The server never pushes directly to a default/protected branch and never auto-merges in this rollout. A failed or cancelled run keeps its audit history and may close its Draft MR, but destructive branch cleanup requires an explicit authorized action.

## 6. Phase Plan

### Phase 0 — Plan and checkpoint

- checkpoint existing user work
- write this plan and persistent execution records
- confirm existing extension points and compatibility constraints

Acceptance: clean baseline commit and an auditable phase plan.

### Phase 1 — Persistence and state machine

- add models and migrations for demand specs, review contracts, execution runs/actions and corpus candidates
- seed permissions
- implement normalization and allowed-transition helpers

Acceptance: migration succeeds; focused model/state tests pass; existing data is unaffected.

### Phase 2 — Human-reviewed executable specification

- create draft/read/update/freeze APIs
- allow deconstruction output to initialize a draft while preserving original trace/context references
- add review-contract create/update/approve APIs
- invalidate stale execution eligibility when a new draft version is created

Acceptance: only authorized users can freeze; frozen versions are immutable; audit provenance is complete.

### Phase 3 — Reviewer resolution and preflight

- resolve final reviewers from contract, changed paths and repository rules
- enforce author/reviewer separation and minimum approvals
- return explicit preflight checks and blockers

Acceptance: deterministic resolution tests cover normal, protected-path, unavailable-candidate and self-review cases.

### Phase 4 — Controlled execution and GitLab adapter

- create/start/cancel execution runs
- add GitLab project, branch, commit and Draft-MR operations behind an interface
- record every external request/result as an execution action
- attach reviewer resolution and test evidence to the Draft MR

Acceptance: mocked GitLab tests prove branch/commit/MR request shape, idempotency, failure states and protected-branch safety.

The generated change set is an explicit input artifact in v1. AI generation may produce that artifact, but the execution engine only accepts validated path/content actions and never executes arbitrary shell text from a model.

### Phase 5 — Demand workspace integration

- add specification/review/execution state to demand details
- expose human edit/freeze, preflight, start/cancel and verification actions
- display blockers, tests, MR, reviewers and audit trail without adding a separate cockpit

Acceptance: existing demand/schedule flows remain usable and the production frontend build passes.

### Phase 6 — Feedback and knowledge candidates

- create candidates from final spec, human corrections, estimates, test/review failures and acceptance outcome
- add list/review endpoints and a compact Settings review surface
- accepted candidates enter the existing context registry as versioned facts

Acceptance: rejected/pending candidates never affect context packs; accepted candidates preserve provenance.

### Phase 7 — Integration and delivery

- run focused and full backend tests
- run frontend build and targeted type checks
- verify diff hygiene and update all planning records
- commit the completed rollout

Acceptance: all phase criteria pass or any external-environment-only gap is explicitly documented with reproducible evidence.

## 7. Out of Scope

- automatic merge to protected branches
- deployment to production
- executing arbitrary model-generated shell commands
- training a model directly from raw production conversations
- bypassing GitLab approval or CODEOWNERS controls
- silently promoting AI output to trusted system knowledge

## 8. Delivery Definition

The rollout is complete when a demand can move from a human-edited frozen specification through deterministic preflight into an auditable GitLab Draft MR with reviewer assignment and evidence, and its final outcome can create a governed knowledge candidate without contaminating the trusted context registry.

## 9. Implementation Status

All phases are implemented. Final backend regression, frontend production build, frontend static checks, and diff-hygiene validation pass. The resulting workflow intentionally stops at a Draft MR and requires both GitLab evidence and named human acceptance before delivery can be reconciled as complete.
