# Follow the decision lifecycle

`decision_prepare` freezes the issue, current remote values, desired values, active policy version, execution binding, expiry, and digest.

`decision_execute` creates a durable Operation, Action, and Outbox entry. The worker rechecks the source, current policy, execution binding, and remote preconditions before dispatch.

The worker reads Jira after dispatch. A local transaction, HTTP `202`, or a sent request isn't success evidence.
