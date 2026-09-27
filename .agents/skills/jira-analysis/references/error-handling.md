# Handle Jira tool errors

- `unauthenticated`: stop and repair the integration credential.
- `forbidden`: don't probe neighboring projects or fields.
- `invalid_query`: narrow the time range, grouping, fields, or result size.
- `rate_limited`: wait before retrying. Rotated keys share the source quota.
- `upstream_unavailable`: report that the source couldn't be verified.

Never replace an error or missing history with a generated fact.
