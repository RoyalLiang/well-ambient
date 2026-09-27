---
name: board-decision
description: Prepare and execute governed single-issue Jira reassignment or rescheduling plans with idempotency, precondition checks, and remote confirmation.
---

# Board Decision

Use this skill only for a single Jira issue reassignment or due-date change.

## Execute a decision

1. Call `decision_get_context` to read current remote state and allowed actions.
2. Call `decision_prepare` with the exact desired assignee or due date. Treat the returned plan as reviewable intent, not an executed fact.
3. Check the target, before and after values, policy version, expiry, and evidence reference.
4. Call `decision_execute` with only `plan_id` and a stable `idempotency_key`. Don't add or alter changes during execute.
5. Poll `decision_get_operation` until it reaches `succeeded`, `failed`, `partial`, or `unknown`.

## Interpret operation states

- `accepted`, `queued`, or `dispatching`: the change isn't confirmed.
- `succeeded`: Jira was read after dispatch and matched the desired value.
- `failed`: no successful effect was confirmed. Follow the error code.
- `unknown`: stop. Don't retry with a new key or claim success.
- `partial`: report each action separately.

## Keep the safety boundary

Don't send `approved=true`, actor, source, Jira credentials, or extra change parameters. The server owns approvals, identity, execution bindings, policy, and remote verification.

Read [the lifecycle](references/lifecycle.md), [idempotency rules](references/idempotency.md), and [error handling](references/error-handling.md) when needed.
