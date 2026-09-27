# Handle decision errors

- `forbidden`: policy, source, or execution binding no longer allows the action.
- `stale_plan`: prepare again from fresh context. Don't reuse old desired assumptions.
- `idempotency_conflict`: recover the original operation or choose the correct business attempt.
- `upstream_unavailable`: no dispatch confirmation is available.
- `outcome_unknown`: stop automatic retries and escalate for verification.
