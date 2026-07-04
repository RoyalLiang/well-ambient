# Reusable Agent Rules

This package is a clean backup of the agent rule runtime.

To use it in another project, copy `AGENTS.md`, `.agents/`, `scripts/validate-agent-rules.py`, and `.pre-commit-config.yaml` into that project. Merge `gitignore-agent-rules.snippet` into the project `.gitignore`.

Validate after copying:

```bash
python3 -B scripts/validate-agent-rules.py
```

The `agent-rules-latest` package resets active runtime state and long-memory records so it can be reused safely. Timestamped snapshots preserve the source project state as a reference.
