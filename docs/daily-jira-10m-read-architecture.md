# Keep Daily Jira reads bounded at 10 million rows

Daily Jira now reads from a dedicated projection instead of filtering and grouping the full `task_telemetries` table on every refresh. The online path returns at most 100 items, reads summary counts from a small counter table, and continues with a generation-bound keyset cursor.

This architecture targets millisecond database reads after the projection and its indexes are warm. It does not claim millisecond cold starts, first-time migrations, network delivery, or arbitrary low-selectivity text searches.

## Verify the read path before deployment

Run the focused regression suite:

```sh
GOCACHE=/tmp/well-ambient-daily-jira-gocache \
  go test ./internal/dailyjira ./internal/server \
  -run 'TestReadPage|TestDailyJiraAuditReadsOnlyTheBoundedProjectionPage' \
  -count=1
```

Run the disposable scale benchmark:

```sh
GOCACHE=/tmp/well-ambient-daily-jira-gocache \
  go run ./cmd/daily-jira-read-bench \
  --rows 10000000 \
  --samples 500
```

The benchmark creates a temporary SQLite database, prints progress and latency percentiles, and removes the database when it exits. Add `--keep` only when you need to inspect the generated database.

## Read from a projection, not the write model

The projection separates write-friendly task storage from read-friendly Daily Jira access:

```text
task_telemetries writes
        │
        ├─ SQLite triggers ─> daily_jira_audit_entries
        │                         ├─ ordered page indexes
        │                         ├─ prefix or FTS search index
        │                         └─ generation counter
        │
        └────────────────────> daily_jira_audit_counts

GET /api/decision/daily-jira
        ├─ read small summary counters
        ├─ seek at most 101 projected rows
        ├─ enrich at most 100 task IDs with indexed per-task Top-N windows
        └─ return 100 rows plus the next cursor
```

The deep module boundary is:

```go
ReadPage(scope, bucket, search, cursor, limit) -> page + stable snapshot metadata
```

Callers don't construct projection SQL or decode cursor internals. The reader normalizes scope, caps the page at 100 rows, chooses the available search adapter, reads summary counters, and rejects invalid or stale cursors.

## Use monotonic keyset pagination

Daily Jira keeps its existing business order: overdue items first, then older creation time, older activity time, and task ID. The projection stores `sort_overdue` as `0` for overdue and `1` otherwise, which makes the whole order ascending:

```sql
ORDER BY sort_overdue, created_unix, activity_unix, task_id
```

Continuation uses one row-value comparison:

```sql
WHERE (sort_overdue, created_unix, activity_unix, task_id) > (?, ?, ?, ?)
```

This matters at large offsets. An earlier four-branch `OR` cursor appeared to use the page index, but still walked the ordered prefix. At 100,000 rows its tail-page p95 was about 4.2 ms. The monotonic row-value seek reduced the same tail-page p95 to about 0.13 ms.

The cursor also contains:

- the projection generation;
- a fingerprint of bucket, search text, project scope, and assignee scope;
- a cursor format version.

A projection-relevant write increments the generation; unrelated task fields do not invalidate readers. A continuation from an older generation receives HTTP `409`, so the browser restarts from a completed first page instead of silently skipping or duplicating records.

When the browser has already loaded several pages, an automatic refresh stages pages through the same loaded frontier from one generation. It swaps the completed segment into the UI once. It never merges a new first page with an old tail, so stable scrolling does not trade away snapshot consistency.

## Keep summary queries independent of row count

`daily_jira_audit_counts` stores counts by project, assignee, and bucket. Insert, delete, and bucket-change triggers update these counters transactionally with the projection row.

The request path summarizes the small counter table instead of running `COUNT` or `GROUP BY` across millions of task rows. Separate indexes cover the unscoped bucket summary and assignee-scoped summary. Project-scoped summaries use the counter table primary key.

Decision enrichment is bounded too. Composite indexes lead with `task_id`, then action and descending creation time. Window queries return at most eight audit events and one latest reminder per page task instead of loading an unbounded history into Go and trimming it afterward.

## Move natural-day transitions out of requests

Age buckets and overdue state change when the local calendar day changes. The HTTP reader never performs this rollover.

`startDailyJiraProjectionWorker` runs the rollover in the background once at startup and every minute. Each transaction updates at most 5,000 age candidates and 5,000 due-date candidates through partial transition indexes. It advances the projection generation after each completed batch and records the new rollover day only after the backlog is empty.

This keeps the first request after midnight from inheriting a potentially large update. During rollover, each response still observes one committed projection generation.

## Select the search adapter at runtime

The default Go SQLite driver does not include FTS5. The migration records the available adapter in `daily_jira_audit_state.search_mode`, and the API exposes it as `page.search_mode`.

The two modes are:

| Mode | Behavior | Intended use |
| --- | --- | --- |
| `prefix` | Uses five bucket-leading B-tree indexes, unions matching task IDs, then reads the ordered page | Default builds and selective task, title, project, assignee, or status prefixes |
| `fts5_trigram` | Uses the external-content trigram FTS table and projection triggers | Three-or-more-character contains search |

Build the server with FTS5 when contains search is required at large scale:

```sh
go build -tags sqlite_fts5 -o well-ambient-server ./cmd/server
```

The focused module suite runs in both modes:

```sh
go test ./internal/dailyjira -count=1
go test -tags sqlite_fts5 ./internal/dailyjira -count=1
```

No local index can guarantee millisecond latency for a prefix that matches millions of rows while also preserving the business sort. Treat such queries as a separate search-product requirement. At that point, route the existing search adapter boundary to a dedicated search service with top-K retrieval instead of weakening the page-read contract.

## Understand the measured performance envelope

The following local benchmark ran on 2026-08-21 with Go 1.24.1 on Darwin arm64. It used 10,000,000 synthetic projection rows, a page limit of 100, a 512 MiB SQLite page cache, a 1 GiB memory map, warm reads, and 500 samples per scenario.

| Scenario | p50 | p95 | p99 | max |
| --- | ---: | ---: | ---: | ---: |
| Reader first page, including summary | 0.585 ms | 0.824 ms | 1.031 ms | 3.642 ms |
| Reader cursor page, including summary | 0.600 ms | 0.845 ms | 0.913 ms | 6.334 ms |
| Selective Jira key search | 0.279 ms | 0.367 ms | 0.435 ms | 0.614 ms |
| Selective title-prefix search | 0.279 ms | 0.525 ms | 1.201 ms | 1.617 ms |
| Raw keyset page near midpoint | 0.098 ms | 0.115 ms | 0.214 ms | 0.377 ms |
| Raw keyset page near tail | 0.098 ms | 0.112 ms | 0.194 ms | 0.350 ms |

The generated database was about 6.25 GB. Loading rows took 26.8 seconds, and building the final index set took 69.8 seconds. These are one-time preparation costs, not online query latency. Real size and migration time depend on title length, cardinality, storage, journal settings, and whether FTS5 is enabled.

The benchmark proves that the local warm read module is depth-independent at 10 million rows. It does not prove HTTP p95 under production concurrency. Capture API timing, SQLite busy time, response size, and browser long tasks after deploying the new binary.

## Roll out the projection without hiding migration cost

The application migration backfills the projection when `backfilled = 0`. It bulk-materializes rows before creating the secondary indexes, avoiding per-row maintenance of the final index set. It also reinstalls the module-owned triggers transactionally, so an existing trigger name cannot preserve an older SQL definition. At 10 million rows, don't first run that backfill in an unplanned restart window.

Before rollout:

1. Take a recoverable SQLite backup.
2. Rehearse migration against a production-sized copy.
3. Measure database growth and free disk space with the intended FTS build mode.
4. Schedule the first materialization as a migration step or maintenance window.
5. Start the new binary only after the projection state and index plans pass verification.

Code rollback does not require deleting the projection tables. The previous binary can ignore them while the triggers continue to keep them current. Drop projection tables or triggers only as a separate, reviewed database cleanup after rollback is stable.

## Monitor the boundaries that can still break the target

Track these signals separately:

- projection generation and rollover backlog;
- query p50, p95, and p99 by bucket, search mode, and scope;
- SQLite busy and lock duration;
- response rows and bytes, which must stay bounded;
- stale-cursor `409` rate;
- first-time migration and index-build duration;
- browser mounted-row count, DOM node count, and long tasks;
- automatic refresh success, last completed refresh time, and visible scroll stability.

If page latency rises with cursor depth, inspect the row-value seek and its composite index first. If summary latency rises with task-row count, verify that the request is still reading `daily_jira_audit_counts` rather than grouping `daily_jira_audit_entries` or `task_telemetries`.
