# Check report applicability

The report applies to its recorded `base_sha` and `head_sha`.

- `matches_current_head`: the supplied current head matched the report.
- `outdated`: the supplied current head differed.
- `not_checked`: no current head was supplied.

Always include `run_status`, `coverage_gaps`, and `evidence_complete`. Don't use the newest timestamp as a substitute for a SHA match.
