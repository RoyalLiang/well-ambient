# Reuse the idempotency key

Choose one stable key for the business attempt, such as a workflow run ID plus step ID.

- Same source, key, and plan: returns the original Operation.
- Same source and key with another plan: returns `idempotency_conflict`.
- Another key for an already accepted plan: returns `stale_plan`.

Don't generate a new key to escape `unknown`.
