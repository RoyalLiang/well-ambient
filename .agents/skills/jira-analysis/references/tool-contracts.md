# Use the Jira tools

Use the raw MCP names below. A host may expose them as `mcp__<server>__<raw-name>`.

| Tool | Use it for |
| --- | --- |
| `jira_describe_schema` | Published fields, dimensions, metrics, versions, and completeness |
| `jira_search_issues` | Stable, bounded issue pages and drill-down |
| `jira_get_issue` | One issue and optional paged history |
| `jira_aggregate_issues` | Complete-set totals and grouped metrics |

Every response includes policy and data-watermark metadata. A cursor is valid only for the original filters, fields, policy version, and snapshot.
