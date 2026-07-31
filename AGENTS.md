# AGENTS

Cold start only:

1. Read `./.agents/boot.yaml`.
2. Read `./.agents/core-lite.yaml`.
3. Read `./.agents/state-lite.json`.
4. Read `./.agents/route-presets.jsonl`.

Classify the task, choose the smallest preset, then upgrade only when a trigger in `boot.yaml` says so.

Upgrade map:

- Full router: `./.agents/router.yaml`
- Full core: `./.agents/core.yaml`
- Full state: `./.agents/state.json`
- Memory runtime: `./.agents/runtime/memory.yaml`
- Tool routing: `./.agents/runtime/tool-routing.yaml`
- Final delivery: `./.agents/output/delivery.yaml`

Keep active context to goal, phase, constraints, findings, touched files, validation, and next action.

## Mandatory UI and frontend review gate

Before editing any frontend file or making any UI, interaction, responsive, accessibility, or visual change:

1. Load and apply the project-local `./.agents/skills/impeccable/SKILL.md`.
2. Run a three-way review with Impeccable, `design-taste-frontend` (taste-skill), and `finesse-ui` (finesse-skill).
3. Record the shared direction plus any disagreements in the active task plan before implementation.
4. Do not edit UI/frontend code until the review agrees on hierarchy, component ownership, responsive behavior, and validation scope.
5. After implementation, run Impeccable detection and authenticated browser validation for every affected state and breakpoint.

The gate is mandatory even for small styling fixes. Prefer one clear surface hierarchy, shared components for repeated behavior, and spacing or hairlines over nested cards.

## Matt Pocock skill routing

The `mattpocock-skills@mattpocock` bundle is installed globally under `~/.codex/skills`. Load only the smallest matching skill; do not load the bundle wholesale. Project-local rules and mandatory gates above take precedence.

Use these model-invoked skills automatically when the task clearly matches:

- `diagnosing-bugs`: reproducible bugs, failures, regressions, or performance diagnosis.
- `tdd`: explicitly test-first work, red-green-refactor, or integration-test-driven implementation.
- `prototype`: a disposable experiment is needed to answer a UI, state-model, or logic question.
- `research`: primary-source research that should be captured as a cited repository document.
- `domain-modeling`: domain terminology, ubiquitous language, context boundaries, or ADR maintenance.
- `codebase-design`: module interfaces, seams, testability, or deep-module architecture.
- `code-review`: review of a PR, branch, or diff against a fixed point and an originating specification.
- `resolving-merge-conflicts`: an in-progress merge or rebase has conflicts to resolve.
- `grilling`: only when the user explicitly asks to be grilled or to stress-test a decision.

Treat orchestration skills as user-invoked. Use `ask-matt`, `grill-me`, `grill-with-docs`, `triage`, `improve-codebase-architecture`, `setup-matt-pocock-skills`, `to-spec`, `to-tickets`, `wayfinder`, `implement`, `handoff`, `teach`, or `writing-great-skills` only when the user explicitly requests that flow or names the skill.

Before the first tracker- or domain-dependent Matt Pocock workflow in this repository, run `setup-matt-pocock-skills` with the user’s awareness. Do not silently choose an issue tracker, labels, or documentation location.
