# Keep Jira metric meanings separate

- `issue_count` counts distinct issues in the current projection.
- `unassigned_count` counts current issues without an assignee.
- `overdue_count` counts unresolved current issues whose due date is before the query time.
- `history_complete_count` and `history_incomplete_count` describe synchronized changelog coverage.
- `created_issue_count` counts issues created in `[event_from, event_until)`.
- `resolved_event_count` counts every transition into a resolved state.
- `resolved_issue_count` counts distinct issues with at least one such transition.
- `reopened_event_count` counts transitions from resolved back to unresolved.
- `assignee_change_event_count` counts assignment changes; `assignee_change_issue_count` counts distinct affected issues.
- `status_duration_hours` sums complete-history time by status and requires `group_by=status`.

Status, assignee, priority, project, issue type, and bug category are current-state dimensions. The bug category comes from the configured authoritative Jira field.

Use half-open time ranges: include `event_from`, exclude `event_until`. Historical metrics use UTC and support `time_bucket=day|week`. Don't mix current and historical metrics in one request.
