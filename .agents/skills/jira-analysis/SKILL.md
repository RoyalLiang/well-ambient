---
name: jira-analysis
description: Analyze published Jira facts with server-side search and aggregation while preserving metric definitions, snapshot time, evidence, and history gaps.
---

# Jira Analysis

Use this skill for Jira inventory, distribution, ownership, priority, due-date, data-quality, and drill-down questions.

## Run the analysis

1. Call `jira_describe_schema` before using an unfamiliar field, dimension, or metric.
2. State whether the question is about current inventory or historical events. Don't treat current state as an event count.
3. Use `jira_aggregate_issues` for totals and distributions. Don't total a page returned by `jira_search_issues`.
4. Use `jira_search_issues` or `jira_get_issue` to inspect the issues behind a result.
5. Keep the returned `query_snapshot_ref`, `data_as_of`, policy version, metric version, and completeness with the conclusion.
6. Cite issue or change references for each material explanation.

## Stop instead of guessing

Stop or narrow the request when the tool returns `forbidden`, `invalid_query`, `rate_limited`, or `upstream_unavailable`.

When `completeness` is below `1`, explain the missing history. Don't convert unavailable historical metrics into zero.

## Report the result

Separate facts from interpretation. Include:

- the metric and grouping definition;
- the authorized scope and data cutoff;
- the result;
- evidence or drill-down references;
- missing data and limits.

Read [tool contracts](references/tool-contracts.md), [metric semantics](references/metrics.md), and [error handling](references/error-handling.md) when needed.
