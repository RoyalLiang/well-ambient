# Agent Rule System

This directory contains the canonical project agent rules.

## Operating Model

1. `AGENTS.md` bootstraps agents that look for a root instruction file.
2. `.agents/boot.yaml`, `.agents/core-lite.yaml`, `.agents/state-lite.json`, and `.agents/route-presets.jsonl` form the cold boot layer.
3. `.agents/router.yaml`, `.agents/core.yaml`, and `.agents/state.json` are full upgrade layers.
4. Domain, workflow, runtime, and output modules are loaded only when their triggers match.
5. `.agents/runtime/memory.yaml` controls automatic compression and long memory.
6. `.agents/output/delivery.yaml` controls final responses.

## Memory Layers

- Active context: current task only.
- `state-lite.json`: tiny resume pointer for cold start.
- `state.json`: full warm memory for current or recent complex work.
- `memory/*.jsonl`: durable long memory records.
- `task_plan.md`, `findings.md`, `progress.md`: task-local memory for complex work.

## Validation

Use `.agents/registry.yaml` as the path index. A stable rule set should satisfy:

- all registry paths exist
- YAML files parse
- JSON files parse
- JSONL memory records parse line by line
- router references do not point to missing required files

Run:

```bash
ruby scripts/validate-agent-rules.rb
```
