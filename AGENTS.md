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

Frontend UI rule:

- Any change that touches frontend UI, visual design, interaction, layout, or user-facing styling must use `taste-skill` before editing. If `taste-skill` is unavailable in the current Codex skill list, explicitly record that fallback and use `frontend-design` as the temporary substitute.
