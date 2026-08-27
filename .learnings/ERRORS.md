## [ERR-20260814-007] chrome-console-api-assumption

**Logged**: 2026-08-14T17:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: testing

### Summary
The final browser pass assumed the claimed Chrome tab exposed a `console.find` helper even though this plugin session only advertised viewport control.

### Error
```text
Cannot read properties of undefined (reading 'find')
Capability is not available: console
```

### Resolution
- **Resolved**: 2026-08-14T17:21:00+08:00
- **Notes**: Queried `chrome.capabilities.list()` and limited the final browser assertions to the available viewport, DOM, geometry, and screenshot evidence. Future Chrome sessions must not assume a console capability that is not advertised.

---

## [ERR-20260827-018] release-contract-fixture-escaped-input

**Logged**: 2026-08-27T14:08:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

调整 release.env 的 shell 转义断言时误改了环境变量输入夹具。

### Error

```text
release notes contained automated\ release\ batch instead of automated release batch
```

### Context

- 相同文本在测试中同时出现在输入和期望数组，补丁匹配了第一个位置。
- 生成器正确保留了输入，因此反斜杠进入了批次说明。

### Suggested Fix

分别锚定输入数组与输出断言：输入保持普通空格，只有 `release.env` 断言检查 Bash `%q` 转义。

### Metadata

- Reproducible: yes
- Related Files: cmd/server/release_artifact_contract_test.go

### Resolution

- **Resolved**: 2026-08-27T14:09:00+08:00
- **Notes**: 恢复普通输入，并只在 release.env 期望值中保留反斜杠。

---

## [ERR-20260827-017] apply-patch-delete-add-same-file

**Logged**: 2026-08-27T14:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: infra

### Summary

`apply_patch` 拒绝在同一个补丁中对 Makefile 同时执行 Delete File 和 Add File。

### Error

```text
apply_patch verification failed: invalid patch: multiple operations target Makefile
```

### Context

- 尝试用整文件删除再新增的方式重写 Makefile。
- 工具要求同一文件在一个补丁中只能有一种操作。

### Suggested Fix

整文件替换使用单个 Update File 补丁；需要新增同名文件时拆成两个独立补丁。

### Metadata

- Reproducible: yes
- Related Files: Makefile

### Resolution

- **Resolved**: 2026-08-27T14:01:00+08:00
- **Notes**: 改为一个 Update File 补丁，不再组合 Delete/Add。

---

## [ERR-20260824-001] browser-page-proxy-scrolltop-setter

**Logged**: 2026-08-24T22:08:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The browser page proxy exposed `scrollTop` as getter-only during authenticated Daily Jira lazy-load validation.

### Error
```text
TypeError: Cannot set property scrollTop of [object Object] which has only a getter
```

### Context
- The failure occurred inside browser page evaluation while trying to scroll `#daily-jira-table-content` to its bottom.
- The page itself remained healthy and no Jira synchronization or write action was triggered.

### Suggested Fix
Use the element's native `scrollTo()` method (or a supported locator scroll action) instead of assigning through the proxy property.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DailyJiraAudit.svelte

### Resolution
- **Resolved**: 2026-08-24T22:09:00+08:00
- **Notes**: Replaced direct property assignment with the element's native `scrollTo()` method. The authenticated list then lazy-loaded from a 5305px to a 9612px virtual extent while keeping 23 DOM data rows.

---

## [ERR-20260821-004] daily-jira-trigger-template-argument-mismatch

**Logged**: 2026-08-21T19:08:00+08:00
**Priority**: low
**Status**: resolved
**Area**: database-migration

### Summary
The Daily Jira update-trigger template added a fifth `%s` placeholder for the projection insert but initially passed only four arguments.

### Error
```text
internal/dailyjira/migration.go:253:3: fmt.Sprintf format %s reads arg #5, but call has 4 args
```

### Suggested Fix
After changing generated SQL templates, run `gofmt` and the focused package test immediately so Go's format-string vet check validates placeholder parity before broader testing.

### Resolution
- Passed `insertEntry` as the fifth template argument.
- Re-ran the focused Daily Jira migration and read-model tests.

---

## [ERR-20260814-001] spreadsheet-formula-description-text

**Logged**: 2026-08-14T10:30:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary
Two formula-description cells beginning with `=` were interpreted as executable formulas and produced `#NAME?` during workbook verification.

### Error
```text
责任划分定级!C40 = #NAME?
责任划分定级!C41 = #NAME?
```

### Context
- Added a responsibility-grading sheet to the existing technical-team assessment workbook.
- The cells were intended to display human-readable formula descriptions, not calculate values.

### Suggested Fix
Prefix literal formula-description strings with a single quote before writing them as values, and keep the final formula-error scan mandatory.

### Metadata
- Reproducible: yes
- Related Files: outputs/019fdb47-81b1-7940-9f2d-5098adb30344/研发考核标准-责任划分定级版.xlsx

### Resolution
- **Resolved**: 2026-08-14T10:31:00+08:00
- **Notes**: Updated the builder to write the two descriptions as literal text and reran validation.

---

## [ERR-20260814-008] browser-locator-focus-api-mismatch

**Logged**: 2026-08-14T11:13:01+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The browser validation attempted an unsupported `locator.focus()` method while checking the Daily Jira link focus state.

### Error
```text
dailyAuthTab.playwright.locator(...).focus is not a function
dailyAuthTab.playwright.screenshot is not a function
```

### Context
- The authenticated page and Jira link were already available and valid; only the assumed browser-client locator method was unsupported.
- No external Jira navigation, form input, or business write occurred.

### Suggested Fix
Use the browser client's supported keyboard interaction for focus movement, or prove focus styling through the source contract without invoking an unsupported locator method. Capture images with `tab.screenshot(...)`, not `tab.playwright.screenshot(...)`.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DailyJiraAudit.svelte

### Resolution
- **Resolved**: 2026-08-14T11:13:01+08:00
- **Notes**: Stopped retrying the unsupported focus method, retained source-contract focus verification, and used `tab.screenshot(...)` for runtime images while continuing DOM, geometry, and link-attribute checks.

---

## [ERR-20260814-005] viewport-resize-with-locked-modal-scroll

**Logged**: 2026-08-14T16:13:00+08:00
**Priority**: low
**Status**: resolved
**Area**: testing

### Summary
Resizing from the mobile breakpoint while the modal held a body scroll lock preserved the mobile page offset, so the desktop workspace-scoped dialog was sampled offscreen and one reopen wait timed out.

### Error
```text
Playwright selector deadline exceeded while waiting for the 900px dialog after a coordinate scroll targeted the outer page.
```

### Resolution
- **Resolved**: 2026-08-14T16:15:00+08:00
- **Notes**: Closed the dialog before each viewport change, reopened from the semantic Edit button, and validated each breakpoint as an independent user state.

---

## [ERR-20260814-004] hmr-closed-modal-before-geometry-sample

**Logged**: 2026-08-14T16:05:00+08:00
**Priority**: low
**Status**: resolved
**Area**: testing

### Summary
The first post-HMR geometry sample assumed the previously open dialog still existed and passed a missing workbench node to `getComputedStyle`.

### Error
```text
TypeError: getComputedStyle expects an Element
```

### Resolution
- **Resolved**: 2026-08-14T16:06:00+08:00
- **Notes**: Checked dialog presence after HMR, confirmed the app had cleanly reset to preview, then reopened the draft editor before sampling.

---

## [ERR-20260814-003] unmatched-zsh-config-glob

**Logged**: 2026-08-14T15:53:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
A read-only config scan used an unmatched `.env*` glob under zsh, which aborted that scan segment.

### Error
```text
zsh: no matches found: .env*
```

### Resolution
- **Resolved**: 2026-08-14T15:54:00+08:00
- **Notes**: Used the live process open-file list and a read-only SQLite query instead; future optional globs should be expanded with `rg --files` first. A later literal `rg` pattern containing backticks repeated the same shell-interpolation class, so literal search patterns must also be single-quoted.

---

## [ERR-20260814-002] markdown-header-contract-overfit

**Logged**: 2026-08-14T15:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first green run failed because the new source contract required an inline template expression even though the component correctly computed the same condition reactively.

### Error
```text
Expected `{#if showToolbar && (showDocumentMeta || toolbarModes.length > 1)}` but the implementation used `showWorkbenchToolbar`.
```

### Resolution
- **Resolved**: 2026-08-14T15:36:00+08:00
- **Notes**: Assert the reactive behavior boundary and its template use separately, avoiding an implementation-shape false negative. The same overfit class recurred when a layout contract fixed CSS declaration order; it was corrected by extracting the selector block and asserting each required property independently.

### Metadata
- Recurrence-Count: 2
- Last-Seen: 2026-08-14

---

## [ERR-20260814-001] local-service-probe-address-and-query-quoting

**Logged**: 2026-08-14T10:05:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The initial local UI probe assumed an IPv4-reachable listener and left a URL query string unquoted in zsh.

### Error
```text
curl: (7) Failed to connect to 127.0.0.1 port 5173
zsh: no matches found: http://127.0.0.1:8080/api/performance/explanation?snapshot_limit=1
```

### Resolution
- Treat `lsof` listener presence and HTTP reachability as separate checks.
- Quote URLs containing query parameters and use the authenticated in-app browser for the required UI validation surface.

---

## [ERR-20260721-023] frontend-validation-run-from-repository-root

**Logged**: 2026-07-21T15:24:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The final frontend check and build were first launched from the repository root, which has no pnpm importer manifest.

### Error
```text
ERR_PNPM_NO_IMPORTER_MANIFEST_FOUND No package.json was found in /Users/eddie/Workspace/well-ambient
```

### Context
- Operation: rerun the final Svelte check and production build after the responsive height fixes.
- The frontend package lives under `web/`, so the root working directory was invalid for these commands.

### Suggested Fix
Run frontend package commands with `workdir=/Users/eddie/Workspace/well-ambient/web`.

### Metadata
- Reproducible: yes
- Related Files: web/package.json
- Recurrence-Count: 4
- Last-Seen: 2026-07-22

### Resolution
- **Resolved**: 2026-07-21T15:25:00+08:00
- **Notes**: Re-ran both commands from `web/`.

---

## [ERR-20260813-006] legacy-evidence-api-fixture

**Logged**: 2026-08-13T16:12:00+08:00
**Priority**: low
**Status**: resolved
**Area**: testing

### Summary
The full server suite found an older rollback-evidence API fixture that did not supply the v4-required release method.

### Error
```text
append evidence = 422: release_method must match v4 full, canary, or hotfix
```

### Resolution
- **Resolved**: 2026-08-13T16:13:00+08:00
- **Notes**: Added the scorecard-defined full-release fact to the fixture; product validation remains strict.

---

## [ERR-20260813-005] v4-closeout-compile-types

**Logged**: 2026-08-13T16:05:00+08:00
**Priority**: low
**Status**: resolved
**Area**: testing

### Summary
The first post-gofmt compile caught one stale import and a `json.RawMessage` passed directly to `strings.Contains` in the new de-duplication assertion.

### Error
```text
internal/performance/module_test.go: cannot use json.RawMessage as string
internal/performance/types.go: "strings" imported and not used
```

### Resolution
- **Resolved**: 2026-08-13T16:06:00+08:00
- **Notes**: Removed the stale import and made the test's raw JSON conversion explicit.

---

## [ERR-20260813-008] browser-evaluate-fetch-unavailable

**Logged**: 2026-08-13T14:36:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The browser read-only evaluation sandbox did not expose `fetch` while checking the final performance API state.

### Error
```text
TypeError: fetch is not a function
```

### Context
- The application page itself continued to fetch and render the same endpoint normally.
- No product API request failed.

### Suggested Fix
Use the page-rendered DOM state or the browser network capability instead of assuming native `fetch` exists inside the evaluation sandbox.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/PerformanceCalculationGuide.svelte

### Resolution
- **Resolved**: 2026-08-13T14:36:00+08:00
- **Notes**: Reloaded the app and verified the final 12:53:47 startup run through the rendered calculation status.

---

## [ERR-20260813-007] performance-snapshot-locator-not-unique

**Logged**: 2026-08-13T14:27:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
An authenticated browser click used only the member name, but append-only scoring correctly exposed the same member in two calculation runs.

### Error
```text
strict mode violation: member detail locator resolved to 2 elements
```

### Context
- Read-only interaction with recent immutable performance snapshots.
- The duplicate accessible name represented distinct run timestamps, not a duplicated row bug.

### Suggested Fix
Select the latest matching snapshot explicitly or scope the locator to its run row.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/PerformanceCalculationGuide.svelte

### Resolution
- **Resolved**: 2026-08-13T14:27:00+08:00
- **Notes**: Selected the first, newest snapshot and completed the detail validation.

---

## [ERR-20260813-006] in-app-browser-localhost-client-block

**Logged**: 2026-08-13T14:05:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The in-app browser client blocked navigation to the local development origin even though the application was already available in Chrome.

### Error
```text
Localhost navigation was blocked by the in-app browser client.
```

### Context
- Authenticated validation of the local personnel-performance page at `http://localhost:5173/`.
- No product request or data mutation failed.

### Suggested Fix
Use the connected Chrome capability for local authenticated development when the in-app browser client rejects localhost.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/PerformanceCalculationGuide.svelte

### Resolution
- **Resolved**: 2026-08-13T14:05:00+08:00
- **Notes**: Switched to the already authenticated Chrome session without reading or entering credentials.

---

## [ERR-20260813-005] frontend-check-preexisting-nullability

**Logged**: 2026-08-13T13:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
The full frontend type gate is blocked by nine pre-existing nullability errors in an unrelated solution workspace component.

### Error
```text
SolutionWorkspace.svelte: 'workspace' is possibly 'null'
svelte-check found 9 errors and 91 warnings in 6 files
```

### Context
- The changed performance files produced no type errors.
- Three new scroll-region accessibility warnings were identified for immediate cleanup.

### Suggested Fix
Keep unrelated dirty work untouched, remove warnings introduced by this task, then validate production build and target diagnostics separately.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/SolutionWorkspace.svelte, web/src/components/PerformanceCalculationGuide.svelte

### Resolution
- **Resolved**: 2026-08-13T13:35:00+08:00
- **Notes**: Scoped validation to task-owned files and recorded the repository-wide gate as pre-existing.

---

## [ERR-20260813-004] go-test-httptest-port-sandbox

**Logged**: 2026-08-13T13:15:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The full server test package could not bind an `httptest` loopback port inside the restricted sandbox.

### Error
```text
httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
```

### Context
- `internal/performance` completed successfully.
- `internal/server` reached an unrelated GitLab webhook test that creates a local HTTP test server.

### Suggested Fix
Rerun the same read-only Go test command with managed local-network test permission.

### Metadata
- Reproducible: yes
- Related Files: internal/server/config_handlers_gitlab_webhook_test.go

### Resolution
- **Resolved**: 2026-08-13T13:15:00+08:00
- **Notes**: Escalated the unchanged test command for loopback-only test execution.

---

## [ERR-20260813-003] findings-context-mismatch

**Logged**: 2026-08-13T13:05:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: docs

### Summary
An `apply_patch` append used a summarized sentence that did not exactly match the current findings file.

### Error
```text
apply_patch verification failed: Failed to find expected lines
```

### Context
- Attempted to append task findings using a context line reconstructed from the compacted turn summary.
- No product file was changed.

### Suggested Fix
Read the bounded file tail after compaction and patch against the exact current heading or append at EOF.

### Metadata
- Reproducible: yes
- Related Files: findings.md, progress.md
- Recurrence-Count: 5
- Last-Seen: 2026-08-25T00:00:00+08:00

### Resolution
- **Resolved**: 2026-08-13T13:05:00+08:00
- **Notes**: Re-read each target file independently and applied bounded patches against exact live content. Two further 2026-08-25 attempts reused a cross-file context reconstructed from truncated output and failed without partial writes; recovery again split the update into file-local bounded hunks. Future planning updates must not combine targets unless every anchor was read verbatim in the immediately preceding output.

---

## [ERR-20260813-001] browser-readonly-evaluate-parsefloat

**Logged**: 2026-08-13T09:38:52+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The browser read-only evaluation environment did not expose `parseFloat` as a callable global while measuring the task-tracking bottom inset.

### Error
```text
TypeError: parseFloat is not a function
```

### Context
- Operation: authenticated Chrome geometry measurement after the TaskKanban height fix.
- The failure occurred only in the validation script; it did not affect application code, data, or browser state.

### Suggested Fix
Use direct numeric conversion from computed CSS values in browser read-only evaluations instead of relying on `parseFloat`.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/TaskKanban.svelte
- Recurrence-Count: 2
- Last-Seen: 2026-08-25T00:00:00+08:00

### Resolution
- **Resolved**: 2026-08-13T09:38:52+08:00
- **Notes**: Replaced the parsing helper in the next measurement with direct numeric conversion.

---

## [ERR-20260813-002] browser-responsive-selector-assumption

**Logged**: 2026-08-13T09:40:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The first 760px geometry script assumed the desktop execution workbench selector would always exist and passed a missing node to `getComputedStyle`.

### Error
```text
TypeError: getComputedStyle expects an Element
```

### Context
- Operation: authenticated Chrome responsive validation at the 760px breakpoint.
- The script failed before any business action; it only changed the browser viewport.

### Suggested Fix
Inspect the breakpoint DOM first and make geometry helpers return null for missing optional containers.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/TaskKanban.svelte

### Resolution
- **Resolved**: 2026-08-13T09:40:00+08:00
- **Notes**: Switched the responsive check to null-safe discovery before measuring the active layout.

---

## [ERR-20260812-007] llm-timeout-client-transport-comparison

**Logged**: 2026-08-12T23:25:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first custom-client timeout regression compared an interface containing a function-valued RoundTripper, which is not comparable in Go.

### Error
```text
panic: runtime error: comparing uncomparable type llm.roundTripFunc
```

### Context
- Command: `GOCACHE=/tmp/well-ambient-gocache go test ./internal/llm -count=1`
- The product client had already cleared the injected overall timeout correctly; only the transport-preservation assertion panicked.

### Suggested Fix
Use a pointer-valued `*http.Transport` when an interface identity comparison is required; function-valued adapters should only be invoked, not compared.

### Metadata
- Reproducible: yes
- Related Files: internal/llm/client_test.go

### Resolution
- **Resolved**: 2026-08-12T23:26:00+08:00
- **Notes**: Replaced the function-valued transport fixture with a comparable transport pointer before rerunning the regression.

---

## [ERR-20260812-005] performance-browser-snapshot-locator

**Logged**: 2026-08-12T19:45:00+08:00
**Priority**: low
**Status**: resolved
**Area**: browser-validation

### Summary
Two browser assertions assumed a unique formal snapshot row while the silent runner was actively appending snapshots.

### Error
```text
Playwright selector deadline exceeded
strict mode violation: row locator resolved to 2 elements
```

### Context
- The first assertion ran immediately after restarting the isolated server with newly seeded evidence.
- By the clean-session assertion, two scheduled snapshots for Alice existed, so a role/name locator was no longer unique.

### Suggested Fix
Inspect the page after an unexpected wait, then treat append-only snapshot tables as ordered collections: assert the count and inspect the first latest row instead of assuming one row forever.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/PerformanceCalculationGuide.svelte, internal/performance/module.go

### Resolution
- **Resolved**: 2026-08-12T19:47:00+08:00
- **Notes**: Confirmed formal snapshot generation, read the newest row, and preserved the append-only expectation in the validation evidence.

---

## [ERR-20260812-004] performance-plan-checkpoint-context

**Logged**: 2026-08-12T19:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary
A combined progress checkpoint assumed task-plan wording from a draft summary instead of matching the current persisted section.

### Error
```text
apply_patch verification failed: Failed to find expected lines in task_plan.md
```

### Context
- The product and test files were not touched by the failed patch.
- The task plan contains several adjacent August 12 workstreams, so exact section text matters.

### Suggested Fix
Use `rg -n -C` to locate the task-specific heading and patch only the exact current checklist wording.

### Metadata
- Reproducible: yes
- Related Files: task_plan.md, progress.md

### Resolution
- **Resolved**: 2026-08-12T19:21:00+08:00
- **Notes**: Located the personnel-performance section and applied a scoped checkpoint.

---

## [ERR-20260812-003] performance-evidence-test-migration

**Logged**: 2026-08-12T19:10:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first focused test after adding the formal evidence ledger migrated the production schema but omitted the new table from the package's isolated in-memory test database.

### Error
```text
no such table: performance_evidence_facts
```

### Context
- Command: `GOCACHE=/tmp/well-ambient-gocache go test ./internal/performance ./internal/config`
- The failure happened when the scorer read active formal evidence and when retention deleted expired evidence.

### Suggested Fix
Treat the package test migration list as part of the schema seam and add every new performance-owned table to both production AutoMigrate and the isolated test helper in the same patch.

### Metadata
- Reproducible: yes
- Related Files: internal/db/db.go, internal/performance/module_test.go

### Resolution
- **Resolved**: 2026-08-12T19:11:00+08:00
- **Notes**: Added `PerformanceEvidenceFact` to the in-memory migration before rerunning focused tests.

---

## [ERR-20260812-002] stale-performance-workbook-path

**Logged**: 2026-08-12T14:10:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary
The workbook reader targeted a superseded output filename and failed before importing the current assessment standard.

### Error
```text
ENOENT: no such file or directory, open '.../技术团队成员考核标准-v1.xlsx'
```

### Context
- Attempted a read-only inspection for concrete assessment items.
- The current output file is `研发考核标准.xlsx` in the same output directory.

### Suggested Fix
Resolve the newest intended workbook from the output directory before generating a reader script, and keep the filename in one script constant.

### Metadata
- Reproducible: yes
- Related Files: outputs/019fdb47-81b1-7940-9f2d-5098adb30344/研发考核标准.xlsx

### Resolution
- **Resolved**: 2026-08-12T14:11:00+08:00
- **Notes**: Located the current workbook by modification time and switched the reader input to it.

---

## [ERR-20260731-030] stale-sse-browser-harness

**Logged**: 2026-07-31T23:59:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first realtime telemetry observation reused a browser EventSource across a rebuilt backend, so persisted evidence was present but the open panel did not receive the new named SSE event.

### Error
```text
Expected the open FZ-2247 trajectory to add crane_manager evidence automatically; the panel remained at PUSH 1 until its manual refresh.
```

### Context
- The isolated backend had been stopped, rebuilt, and restarted after the browser had already opened its SSE connection.
- SQLite confirmed both repository records were persisted correctly.
- Reloading the authenticated app created a fresh EventSource; a subsequent webhook changed the open panel from PUSH 3 to PUSH 4 without any UI refresh action.

### Suggested Fix
When validating realtime behavior after rebuilding or restarting the isolated backend, reload the authenticated app before the assertion and prove persistence separately from delivery.

### Metadata
- Reproducible: yes
- Related Files: web/src/App.svelte, web/src/components/CommitTelemetryPanel.svelte, internal/server/notification_handlers.go
- See Also: ERR-20260731-025, ERR-20260731-026, ERR-20260731-027

### Resolution
- **Resolved**: 2026-07-31T23:59:00+08:00
- **Notes**: Recreated the browser SSE connection and observed the next FZ-2247 webhook appear live; the panel showed both task_executor and crane_manager with no console errors.

---

## [ERR-20260721-022] copied-db-overrode-isolated-config

**Logged**: 2026-07-21T15:00:00+08:00
**Priority**: high
**Status**: resolved
**Area**: tests

### Summary
A copied historical database loaded its persisted configuration after startup and overrode the Jira-disabled validation YAML.

### Error
```text
Loaded configuration from database version 19
Starting well-ambient server on 0.0.0.0:8080
Starting background Jira task synchronization worker...
```

### Context
- Operation: start an isolated authenticated UI backend using a copied database for long-data layout validation.
- The YAML disabled all external integrations and selected port 18080, but `config_versions` in the copied database had higher runtime precedence.
- The sandbox denied the unexpected 8080 bind, so no validation service remained running.

### Suggested Fix
When reusing historical data for isolated UI validation, remove persisted `config_versions` from the temporary copy before startup, then verify the startup log shows the intended loopback address and no external worker.

### Metadata
- Reproducible: yes
- Related Files: internal/config/config.go, internal/server/server.go, well-ambient.db

### Resolution
- **Resolved**: 2026-07-21T15:00:00+08:00
- **Notes**: Cleared only the temporary copy's `config_versions` table and retained the Jira-disabled YAML as the sole runtime configuration.

---

## [ERR-20260721-021] parallel-validation-setup-dependency

**Logged**: 2026-07-21T14:55:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
An isolated validation database copy raced the temporary directory creation because dependent setup commands were launched in parallel.

### Error
```text
cp: /private/tmp/well-layout-validation/well-ambient.db: No such file or directory
```

### Context
- Operation: prepare a copied SQLite database and locally built server for authenticated UI validation.
- The directory creation, server build, and database copy were submitted in the same parallel batch even though the copy depended on the directory.

### Suggested Fix
Create the validation directory first, then parallelize only the independent server build and database copy.

### Metadata
- Reproducible: yes
- Related Files: well-ambient.db

### Resolution
- **Resolved**: 2026-07-21T14:55:00+08:00
- **Notes**: Re-ran the database copy after confirming the temporary directory and built server existed.

---

## [ERR-20260720-026] isolated-ui-validation-hit-existing-8080-service

**Logged**: 2026-07-20T22:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: browser-validation

### Summary
The first isolated full-app validation attempts reached an existing service on port 8080, so local development login returned 401 instead of reaching the fixture backend.

### Error
```text
login status 401; fixture backend received no request
```

### Context
- Static Vite preview did not proxy the login API.
- The normal Vite development proxy targets `localhost:8080`, which was already occupied.
- A direct `::1` host value was also invalid because the server formats addresses without IPv6 brackets.

### Resolution
Started a fresh fixture backend on `127.0.0.1:18080` and a temporary Vite validation config proxying `/api` to that port. Authenticated browser validation then passed, and the temporary repo config was removed.

### Metadata
- Reproducible: yes
- Related Files: web/vite.config.ts, internal/server/server.go

---

## [ERR-20260721-020] responsive-submenu-click-after-auto-collapse

**Logged**: 2026-07-21T14:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
Responsive browser validation timed out when clicking a secondary navigation item after its primary item auto-collapsed the mobile rail.

### Error
```text
Playwright selector deadline exceeded while clicking the flow-board submenu
```

### Context
- Operation: validate the flow board after applying a sub-860px viewport override.
- The primary navigation click succeeded, but responsive navigation closed the rail before the secondary click.

### Suggested Fix
After responsive primary navigation, capture a fresh DOM snapshot and reopen the rail before locating and clicking the submenu.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/prototype/FunctionalAdminShell.svelte
- Recurrence-Count: 2
- Last-Seen: 2026-07-21

### Resolution
- **Resolved**: 2026-07-21T14:20:00+08:00
- **Notes**: Continued validation using the responsive rail toggle before the submenu click.

---

## [ERR-20260720-025] version-source-test-omitted-jira-base-url

**Logged**: 2026-07-20T17:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: backend-test

### Summary
The named version-source preference test supplied a release URL but omitted the Jira base URL, so the source was correctly excluded as invalid.

### Error
```text
version source project missing from options
```

### Context
- Operation: verify that version-only projects and configured names flow into user project preferences.
- Version source validity intentionally depends on parsing against the configured Jira base URL.

### Resolution
Added the matching fixture base URL and reran the server package tests.

### Metadata
- Reproducible: yes
- Related Files: internal/server/project_preference_handlers_test.go, internal/config/jira_versions.go

---

## [ERR-20260720-024] project-preference-test-used-wrong-server-fields

**Logged**: 2026-07-20T17:30:00+08:00
**Priority**: low
**Status**: resolved
**Area**: backend-test

### Summary
A new project-preference test referenced guessed `Server.mu` and `Server.cfg` fields, so the test package did not compile.

### Error
```text
server.mu undefined
server.cfg undefined
```

### Context
- Operation: add coverage proving a named Jira version source appears in user project preferences.
- The actual server configuration field is `Server.config` and this test does not require concurrent mutation.

### Resolution
Changed the test fixture to set `server.config.Jira.VersionSources` directly, then reran the server package tests.

### Metadata
- Reproducible: yes
- Related Files: internal/server/project_preference_handlers_test.go, internal/server/server.go

---

## [ERR-20260720-023] browser-binding-was-finalized

**Logged**: 2026-07-20T18:42:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first follow-up browser call assumed a prior task's finalized browser variable was still available in the current execution context.

### Error
```text
browser is not defined
```

### Context
- The previous validation had already finalized its controlled tabs.
- The current task needed a new authenticated geometry pass for Daily Jira and a read-only inspection of an already-open Jira version page.

### Suggested Fix
At the start of each browser task, reuse a live browser binding only when it is actually present; otherwise initialize the browser runtime once, name the session, discover visible user tabs, and claim exact returned tab objects.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DailyJiraAudit.svelte

---

## [ERR-20260731-021] authenticated-preview-not-running

**Logged**: 2026-07-31T16:38:57+08:00
**Priority**: low
**Status**: pending
**Area**: frontend

### Summary
The expected local authenticated frontend preview was not available for the initial browser reproduction.

### Error
```text
curl: (7) Failed to connect to 127.0.0.1 port 5173
```

### Context
- Operation: probe the previously used local Vite URL before building browser red-light assertions.
- The current thread has no attached terminal session and no process was listening on the expected port.

### Suggested Fix
Use the repository's isolated database and local authentication bootstrap to start a fresh backend/frontend pair, then run the browser reproduction against that bounded environment.

### Metadata
- Reproducible: yes
- Related Files: web/vite.config.ts

---

## [ERR-20260731-020] shell-backticks-in-read-only-rg-query

**Logged**: 2026-07-31T11:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
A read-only `rg` query used Markdown backticks inside a double-quoted shell command, so zsh attempted command substitution.

### Error
```text
zsh:1: command not found: Modal
```

### Context
- Operation: locate the shared-modal addendum in `task_plan.md`.
- No file mutation or application process was involved.

### Suggested Fix
Use a single-quoted search pattern or remove Markdown delimiters from shell arguments.

### Metadata
- Reproducible: yes
- Related Files: task_plan.md

### Resolution
- **Resolved**: 2026-07-31T11:20:00+08:00
- **Notes**: Replaced the query with a single-quoted literal pattern and retained the repository command-escaping rule for later checks.

---

---

## [ERR-20260730-001] broad-context-free-conditional-replacement

**Logged**: 2026-07-30T15:41:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
A compatibility guard intended for the release creation helper matched the first identical project-key condition in the file and broke the release list build.

### Error
```
internal/server/release_handlers.go:49:25: undefined: scopedProjectKey
```

### Context
- Operation: preserve the existing project-scoped release creation behavior while validating optional project binding on the new page-level endpoint.
- The patch matched `if projectKey != ""` without including the target function context, so it modified `handleListReleases` instead of `createLocalRelease`.

### Suggested Fix
Use function-level context for repeated conditionals, inspect both the intended function and the first file occurrence after patching, and run the focused compile test immediately.

### Metadata
- Reproducible: yes
- Related Files: internal/server/release_handlers.go

### Resolution
- **Resolved**: 2026-07-30T15:42:00+08:00
- **Notes**: Restored the list filter, applied the scoped-route guard inside `createLocalRelease`, and reran the focused and full server test gates.

---

## [ERR-20260729-001] shell-search-pattern-used-unescaped-backtick

**Logged**: 2026-07-29T00:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
A combined ripgrep command embedded a backtick pattern inside a double-quoted shell command and failed during parsing.

### Error
```text
zsh:3: unmatched "
```

### Context
- Operation: inspect frontend API calls while auditing overlapping strongest-brain pages.
- The search pattern mixed shell double quotes with a literal Svelte template backtick.

### Suggested Fix
Split the search into simple single-quoted patterns or omit the backtick-specific branch when ordinary fetch patterns are sufficient.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/prototype/FunctionalAdminShell.svelte

### Resolution
- **Resolved**: 2026-07-29T00:01:00+08:00
- **Notes**: Re-ran the inspection with a simpler pattern and obtained the required navigation and API ownership evidence.

---

## [ERR-20260726-001] skill-installer-python-ca-chain

**Logged**: 2026-07-26T12:38:49+08:00
**Priority**: medium
**Status**: resolved
**Area**: infra

### Summary
The global skill installer could not download a public GitHub archive because the local Python runtime could not validate the TLS certificate chain.

### Error
```text
ssl.SSLCertVerificationError: [SSL: CERTIFICATE_VERIFY_FAILED] certificate verify failed: unable to get local issuer certificate
```

### Context
- Command: `install-skill-from-github.py --repo mattpocock/skills ...`
- The same repository was reachable through Git over HTTPS.
- Disabling TLS verification would weaken the installation boundary.

### Suggested Fix
Use the installer's supported `--method git` fallback so Git performs the authenticated TLS transport and sparse checkout.

### Metadata
- Reproducible: yes
- Related Files: /Users/eddie/.codex/skills/.system/skill-installer/scripts/install-skill-from-github.py

### Resolution
- **Resolved**: 2026-07-26T12:38:49+08:00
- **Notes**: Retried with the supported Git method instead of bypassing certificate validation.

---

## [ERR-20260724-001] go-build-default-cache-denied

**Logged**: 2026-07-24T01:24:00+08:00
**Priority**: low
**Status**: resolved
**Area**: infra

### Summary
The isolated browser-validation backend build reused the macOS Go cache and was denied by the workspace sandbox.

### Error
```text
open /Users/eddie/Library/Caches/go-build/...: operation not permitted
```

### Context
- Operation attempted: build a temporary local backend for authenticated UI validation.
- The output binary and validation database were already scoped to `/tmp`; only the implicit Go cache escaped that boundary.

### Suggested Fix
Set `GOCACHE=/tmp/well-ambient-gocache` on every local Go build or test command in this workspace.

### Metadata
- Reproducible: yes
- Related Files: cmd/server/main.go
- See Also: ERR-20260713-010, ERR-20260715-001, ERR-20260720-013

### Resolution
- **Resolved**: 2026-07-24T01:24:00+08:00
- **Notes**: Retried the temporary backend build with the established `/tmp` Go cache.

---

## [ERR-20260720-020] go-build-cache-sandbox-denied

**Logged**: 2026-07-20T19:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first Jira version-source test command used the default macOS Go build cache, which the workspace sandbox cannot read.

### Error
```
open /Users/eddie/Library/Caches/go-build/...: operation not permitted
```

### Context
- Operation: run focused config, server, and telemetry Go tests after adding Jira version-link parsing.
- The config package completed, while packages sharing the denied compiled artifact failed during setup.

### Suggested Fix
Set `GOCACHE=/tmp/well-ambient-gocache` for repository Go validation inside the sandbox.

### Metadata
- Reproducible: yes
- Related Files: internal/config/jira_versions.go, internal/server/jira_worker.go
- Recurrence-Count: 2
- Last-Seen: 2026-07-23

### Resolution
- **Resolved**: 2026-07-20T19:36:00+08:00
- **Notes**: Switched the focused rerun to the established `/tmp` Go cache path. The same sandbox denial recurred on 2026-07-23 and was handled with that cache path.

---

## [ERR-20260720-021] broad-config-example-read-exposed-secret-field

**Logged**: 2026-07-20T19:36:00+08:00
**Priority**: high
**Status**: resolved
**Area**: config

### Summary
A broad Jira block inspection included an existing credential field even though only safe schema keys were needed.

### Error
```
The command printed the Jira api_token line from config.example.yaml.
```

### Context
- Operation: locate the example Jira block before deciding whether to document `version_sources`.
- A prior project learning already required narrow safe-field inspection for configuration files.

### Suggested Fix
Do not read configuration-value blocks. Inspect type definitions and exact safe key names only, and skip example-file edits when their context would expose credentials.

### Metadata
- Reproducible: yes
- Related Files: config.example.yaml, internal/config/config.go
- See Also: ERR-20260720-010
- Recurrence-Count: 2
- Last-Seen: 2026-07-21

### Resolution
- **Resolved**: 2026-07-20T19:37:00+08:00
- **Notes**: Stopped reading or editing the example config and kept implementation/documentation in typed code and UI surfaces only.

---

## [ERR-20260720-022] server-tests-loopback-denied

**Logged**: 2026-07-20T22:16:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The sandboxed full server suite could compile but could not open the IPv6 loopback listener required by existing `httptest.NewServer` cases.

### Error
```
httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
```

### Context
- Operation: run the full `internal/server` suite after Jira version-source changes.
- Failure occurred in an existing GitLab webhook HTTP test before the changed Jira tests completed.

### Suggested Fix
Run the bounded `go test ./internal/server` command with loopback permission after it compiles in the sandbox.

### Metadata
- Reproducible: yes
- Related Files: internal/server/config_handlers_gitlab_webhook_test.go
- Recurrence-Count: 2
- Last-Seen: 2026-07-30

### Resolution
- **Resolved**: 2026-07-20T22:17:00+08:00
- **Notes**: Reran the full server suite with loopback permission; it passed. The same sandbox-only failure recurred on 2026-07-30 during delivery-baseline validation and the bounded escalated rerun passed.

### Resolution
- **Resolved**: 2026-07-20T18:45:00+08:00
- **Notes**: Reinitialized the browser connection, claimed the exact local-app and Jira-version tabs returned by discovery, and completed the read-only checks.

---

## [ERR-20260720-010] chrome-evaluate-dom-is-read-only

**Logged**: 2026-07-20T09:51:10+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome evaluation surface exposes DOM nodes as read-only objects, so it cannot host an ephemeral overdue-state fixture.

### Error
```
TypeError: Cannot set property className of [object Object] which has only a getter
```

### Context
- The live Daily Jira dataset currently contains no `latest_decision` rows, so a temporary two-column due state was considered for visual-only verification.
- The operation was intentionally local and non-persistent, but this browser surface disallows DOM mutation.

### Suggested Fix
Do not bypass the read-only browser contract. Validate the available live state in the browser, validate the due selector and two-column source contract statically, and defer real due-state pixels until such a Jira exists in authenticated data.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DailyJiraAudit.svelte

### Resolution
- **Resolved**: 2026-07-20T09:52:00+08:00
- **Notes**: Stopped attempting DOM mutation, preserved live business data, and limited browser claims to the available no-decision state.

---

## [ERR-20260720-020] isolated-task-filter-loopback-bind-denied

**Logged**: 2026-07-20T16:28:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tests

### Summary
The isolated current-source backend and Vite validation servers could not bind their loopback ports inside the default sandbox.

### Error
```text
listen tcp 127.0.0.1:18081: bind: operation not permitted
Error: listen EPERM: operation not permitted 127.0.0.1:5176
```

### Context
- Attempted to start a copied-database backend with every external integration disabled on port 18081 and a temporary Vite proxy on port 5176.
- Existing user-owned services on ports 8080 and 5173 were intentionally left untouched.

### Suggested Fix
Rerun only the two loopback-only validation services with managed escalation, finish authenticated read-only browser checks, then stop both isolated sessions.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/TaskKanban.svelte, web/src/components/shared/MultiSelect.svelte
- See Also: ERR-20260720-014

### Resolution
- **Resolved**: 2026-07-20T18:16:00+08:00
- **Notes**: Started only the copied-database backend and temporary Vite proxy with managed loopback permission, completed authenticated read-only validation, stopped both sessions, and confirmed the original 8080/5173 services were not replaced.

---

## [ERR-20260720-021] browser-clear-filter-footer-became-stale

**Logged**: 2026-07-20T16:38:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The browser check tried to read the open multi-select footer after clicking its conditional clear button, but the clear action removed that button and focus-out closed the dropdown.

### Error
```text
Playwright selector deadline exceeded
```

### Context
- The `全部项目` action correctly cleared the project values.
- The follow-up locator assumed the dropdown would remain open even though the clicked control disappeared after the state change.

### Suggested Fix
After clicking a conditional control that removes itself, capture a fresh DOM snapshot and verify the closed-control summary plus filtered table state instead of reusing a footer locator.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/shared/MultiSelect.svelte
- See Also: ERR-20260720-019

### Resolution
- **Resolved**: 2026-07-20T16:39:00+08:00
- **Notes**: A fresh snapshot showed `全部项目`, the project dropdown closed, and the remaining two-owner filter returned 35 rows as expected.
- See Also: ERR-20260720-009

---

## [ERR-20260720-022] browser-clean-tab-api-assumptions

**Logged**: 2026-07-20T18:12:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The clean-console browser check first assumed unsupported `tabs.open` and `playwright.domcontentloaded` helpers.

### Error
```text
browser.tabs.open is not a function
cleanTab2.playwright.domcontentloaded is not a function
```

### Context
- A fresh tab was needed so pre-authentication 403 logs would not be confused with post-login component errors.
- The browser binding supports `tabs.new()`, `tab.goto()`, snapshots, locators, and `dev.logs()`, but not the two assumed helpers.

### Suggested Fix
Create the tab with `tabs.new()`, navigate with `tab.goto()`, wait briefly for application hydration, then inspect the DOM and tab-scoped error log.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/TaskKanban.svelte

### Resolution
- **Resolved**: 2026-07-20T18:13:00+08:00
- **Notes**: A fresh authenticated tab navigated through Task Tracking to Execution Tracking and returned an empty tab-scoped console error list. The 2026-08-25 recurrence was the same casing mistake (`goTo` instead of supported `goto`); no navigation occurred before correction.

---

## [ERR-20260720-009] chrome-evaluate-classlist-method-unavailable

**Logged**: 2026-07-20T09:50:10+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome evaluation wrapper also exposed `classList` without callable DOMTokenList methods.

### Error
```
TypeError: s.classList.add is not a function
```

### Context
- A temporary, non-persistent overdue state was being composed only to validate two-column geometry and semantic tint.
- Read-only DOM properties and direct property assignment work reliably in this wrapper; several prototype convenience methods do not.

### Suggested Fix
For ephemeral browser fixtures in this runtime, prefer direct `className` and `innerHTML` assignment, always restore the original values, and avoid DOM prototype helpers.

### Metadata
- Reproducible: unknown
- Related Files: web/src/components/DailyJiraAudit.svelte
- See Also: ERR-20260720-006, ERR-20260720-008

### Resolution
- **Resolved**: 2026-07-20T09:51:00+08:00
- **Notes**: Switched the temporary state fixture to direct properties and restored the original DOM immediately after measurement.

---

## [ERR-20260720-008] chrome-evaluate-parsefloat-shadowed

**Logged**: 2026-07-20T09:49:20+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome evaluation sandbox exposed `parseFloat` as a non-callable binding during title-line measurement.

### Error
```
TypeError: parseFloat is not a function
```

### Context
- The selected long-title Jira was already verified correctly.
- Only the derived line-count calculation failed; raw title height and computed line-height remained available.

### Suggested Fix
Return raw geometry and computed CSS strings from the browser, then infer the line count outside the page evaluation instead of calling the shadowed global.

### Metadata
- Reproducible: unknown
- Related Files: web/src/components/DailyJiraAudit.svelte
- See Also: ERR-20260720-007

### Resolution
- **Resolved**: 2026-07-20T09:50:00+08:00
- **Notes**: Removed the in-page numeric parsing and completed the measurement using raw bounding-box height plus computed line-height.

---

## [ERR-20260720-007] chrome-evaluate-mouseevent-constructor-unavailable

**Logged**: 2026-07-20T09:48:10+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome evaluation wrapper did not expose `MouseEvent` as a constructible browser global for synthetic row activation.

### Error
```
TypeError: MouseEvent is not a constructor
```

### Context
- Tried explicit event dispatch after the target row's `click()` method was unavailable.
- The target identifier was then corrected to a Jira row confirmed present in the current 135-row bucket.

### Suggested Fix
Return to the Playwright locator for the confirmed-present Jira key; the earlier locator timeout was caused by requesting a row absent from the current bucket, not by the row being offscreen.

### Metadata
- Reproducible: unknown
- Related Files: web/src/components/DailyJiraAudit.svelte
- See Also: ERR-20260720-005, ERR-20260720-006

### Resolution
- **Resolved**: 2026-07-20T09:49:00+08:00
- **Notes**: Used the locator only after confirming the exact Jira key exists in the current rendered bucket, then verified selection through the inspector.

---

## [ERR-20260720-006] chrome-row-click-method-unavailable

**Logged**: 2026-07-20T09:47:30+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The browser evaluation found the target table row, but its convenience `click()` method was unavailable in this runtime.

### Error
```
TypeError: row.click is not a function
```

### Context
- Followed the fallback from ERR-20260720-005 to activate a rendered Jira row without scrolling or business mutation.
- The returned row supports event dispatch but not the HTMLElement convenience method in this evaluation wrapper.

### Suggested Fix
Dispatch a bubbling, cancelable `MouseEvent('click')` on the located row and confirm selection through the inspector key before measuring.

### Metadata
- Reproducible: unknown
- Related Files: web/src/components/DailyJiraAudit.svelte
- See Also: ERR-20260720-005

### Resolution
- **Resolved**: 2026-07-20T09:48:00+08:00
- **Notes**: Used explicit mouse-event dispatch and verified the selected Jira key in the inspector before continuing.

---

## [ERR-20260720-005] chrome-offscreen-jira-row-locator-timeout

**Logged**: 2026-07-20T09:46:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The text-filtered Playwright locator timed out while activating an offscreen Jira row that was already present in the rendered table.

### Error
```
Playwright selector deadline exceeded
waiting on click for selector tbody tr >> internal:has-text="NS2-1779"i
```

### Context
- Final authenticated validation needed a two-line Jira title without changing business data.
- The table renders a large locally scrollable row set, and the wrapper locator did not resolve the offscreen match before its selector deadline.

### Suggested Fix
Use an in-page exact text lookup over the already-rendered rows and dispatch the existing row click handler, then verify the selected inspector title before measuring.

### Metadata
- Reproducible: unknown
- Related Files: web/src/components/DailyJiraAudit.svelte
- See Also: ERR-20260720-002, ERR-20260720-004

### Resolution
- **Resolved**: 2026-07-20T09:47:00+08:00
- **Notes**: Switched to a bounded DOM row lookup for the non-mutating selection state and continued the same authenticated validation.

---

## [ERR-20260720-004] daily-jira-geometry-missing-scroll-selector

**Logged**: 2026-07-20T09:39:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first Daily Jira geometry probe assumed a `.table-scroll` selector that does not exist in the rendered component.

### Error
```
TypeError: Cannot read properties of null (reading 'scrollHeight')
```

### Context
- Measured inspector section bounds and scroll ownership before the visual fix.
- The missing optional list selector caused the whole evaluation to abort.

### Suggested Fix
Probe candidate overflow selectors from the component source and guard every optional element before reading geometry.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DailyJiraAudit.svelte
- See Also: ERR-20260720-003

### Resolution
- **Resolved**: 2026-07-20T09:39:30+08:00
- **Notes**: Re-ran the measurement with null-safe element geometry and enumerated actual scroll owners instead of assuming the list class.

---

## [ERR-20260720-003] chrome-stale-claimed-tab-handle

**Logged**: 2026-07-20T09:37:23+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The persisted Chrome tab object no longer owned its previously claimed tab even though the tab remained open.

### Error
```
Tab not found: 733581466. Existing tabs: none
```

### Context
- Attempted to reuse the prior turn's `grayFixTab` handle for authenticated Daily Jira geometry inspection.
- `chrome.user.openTabs()` still listed the application tab, but the claimed handle had expired.

### Suggested Fix
Refresh the open-tab list and reclaim the existing tab by id before continuing; do not open a duplicate authenticated application tab.

### Metadata
- Reproducible: unknown
- Related Files: web/src/components/DailyJiraAudit.svelte
- See Also: ERR-20260720-002

### Resolution
- **Resolved**: 2026-07-20T09:38:00+08:00
- **Notes**: Reclaimed tab `733581466` from the current Chrome tab list and continued on the existing authenticated session.

---

## [ERR-20260720-002] chrome-role-locator-hidden-responsive-nav

**Logged**: 2026-07-20T09:27:12+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The role-based Daily Jira navigation locator timed out while the responsive shell did not expose the target as a visible role match.

### Error
```text
Playwright selector deadline exceeded
waiting on click for selector internal:role=button[name=/每日 Jira/]
```

### Context
- Operation attempted: route-isolation check at the calibrated 1024px viewport.
- The exact navigation item was present, but the role query did not produce a clickable visible target in that responsive shell state.

### Suggested Fix
Use the established visible button locator filtered by rendered text after returning to a wide viewport, then verify route state through computed frame class and background.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/prototype/FunctionalAdminShell.svelte
- See Also: ERR-20260719-030

### Resolution
- **Resolved**: 2026-07-20T09:27:12+08:00
- **Notes**: Switched to `locator('button').filter({hasText:'每日 Jira'})` at 1440px, verified the agenda-only class was absent and Daily Jira retained the original gray substrate, then returned to Decision Agenda.

---

## [ERR-20260720-001] rg-leading-hyphen-pattern

**Logged**: 2026-07-20T09:27:12+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Ripgrep parsed a CSS custom-property search pattern as an option because the pattern began with two hyphens.

### Error
```text
rg: unrecognized flag --wa-surface-flat
```

### Context
- Operation attempted: locate the existing `--wa-surface-flat` token definition.

### Suggested Fix
Insert `--` before any ripgrep pattern that starts with a hyphen and place glob options before positional paths.

### Metadata
- Reproducible: yes
- Related Files: web/src/styles/modern-admin-tokens.css

### Resolution
- **Resolved**: 2026-07-20T09:27:12+08:00
- **Notes**: Re-ran the search with `rg -n -- "--wa-surface-flat"` and confirmed the project token definition.

---

## [ERR-20260620-001] apply_patch_context

**Logged**: 2026-06-20T12:46:00+08:00
**Priority**: low
**Status**: pending
**Area**: tooling

### Summary
Patch context drifted after `gofmt`, causing an `apply_patch` verification failure.

### Error
```text
apply_patch verification failed: Failed to find expected lines in internal/server/ai_handlers.go
```

### Context
- Operation attempted: follow-up patch after formatting edited Go files.
- The patch used a larger pre-format context block, while the current file had been formatted.

### Suggested Fix
Use smaller context blocks around the exact current file excerpt after formatters run.

### Metadata
- Reproducible: yes
- Related Files: internal/server/ai_handlers.go

---

## [ERR-20260718-012] frontend-check-run-from-repository-root

**Logged**: 2026-07-18T20:21:10+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The frontend check was first run from the repository root, while this repository keeps its pnpm package manifest under `web/`.

### Error
```text
ERR_PNPM_NO_IMPORTER_MANIFEST_FOUND No package.json was found in /Users/eddie/Workspace/well-ambient
```

### Context
- Attempted `pnpm check` from the repository root after editing Svelte components.
- `web/package.json` contains the actual `check` script.

### Suggested Fix
Run frontend package scripts with `workdir=/Users/eddie/Workspace/well-ambient/web`.

### Metadata
- Reproducible: yes
- Related Files: web/package.json

### Resolution
- **Resolved**: 2026-07-18T20:21:10+08:00
- **Notes**: Located the package manifest and continued validation from `web/`.

---

## [ERR-20260718-004] zsh-empty-glob-in-readonly-search

**Logged**: 2026-07-18T14:28:00+08:00
**Priority**: low
**Status**: resolved
**Area**: config

### Summary
A repository credential search used the unmatched shell glob `.env*`, so zsh stopped the command before `rg` ran.

### Error
```text
zsh:1: no matches found: .env*
```

### Context
- The command was read-only and made no file changes.
- The search combined fixed paths with an optional dotenv glob under zsh's default `nomatch` behavior.

### Suggested Fix
Discover optional dotenv files with `rg --files -g '.env*'` or search the repository using `rg` include globs instead of passing an unmatched shell glob.

### Metadata
- Reproducible: yes
- Related Files: .env.example

### Resolution
- **Resolved**: 2026-07-18T14:28:00+08:00
- **Notes**: Replaced the shell-expanded path with repository-level `rg` include/exclude globs.

---

## [ERR-20260718-005] unmatched-shell-quote-in-source-search

**Logged**: 2026-07-18T14:31:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
A combined source inspection command embedded backticks and both quote styles in one `rg` expression, leaving zsh with an unmatched quote.

### Error
```text
zsh:1: unmatched "
```

### Context
- The command was read-only and stopped before either source inspection ran.
- The intended follow-up was to inspect the login response contract and startup API calls.

### Suggested Fix
Split source inspection and fetch-call discovery into simple commands with single-purpose quoting.

### Metadata
- Reproducible: yes
- Related Files: web/src/App.svelte

### Resolution
- **Resolved**: 2026-07-18T14:31:00+08:00
- **Notes**: Replaced the combined command with separate `sed` and fixed-string `rg` calls.

---

## [ERR-20260718-006] stale-duplicate-input-patch

**Logged**: 2026-07-18T14:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
A narrow patch attempted to remove a duplicate search input seen in a combined source dump, but a direct source re-read showed only one input node.

### Error
```text
apply_patch verification failed: Failed to find expected lines
```

### Suggested Fix
Re-read the exact source hunk before patching when combined command output may repeat adjacent lines.

### Metadata
- Reproducible: no
- Related Files: web/src/components/DailyJiraAudit.svelte

### Resolution
- **Resolved**: 2026-07-18T14:35:00+08:00
- **Notes**: Direct `sed` and `rg` checks confirmed exactly one search input; no code change was needed.

---

## [ERR-20260718-007] sandbox-localhost-bind-denied

**Logged**: 2026-07-18T14:36:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The sandbox denied binding the isolated browser-validation fixture server to `127.0.0.1:5174`.

### Error
```text
Error: listen EPERM: operation not permitted 127.0.0.1:5174
```

### Suggested Fix
Rerun the exact narrow localhost fixture-server command with managed escalation rather than retrying other ports.

### Metadata
- Reproducible: yes
- Related Files: /tmp/well-ambient-daily-jira-fixture.mjs
- Recurrence-Count: 3
- Last-Seen: 2026-07-24

### Resolution
- **Resolved**: 2026-07-18T14:36:00+08:00
- **Notes**: Managed approval allowed the fixture-only runs; the latest recurrence used the isolated copied database, disabled every external integration, and started Vite on the same validated local port.

---

## [ERR-20260718-008] malformed-multi-file-apply-patch

**Logged**: 2026-07-18T14:37:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary
A combined planning/error-log patch left an empty hunk marker before the next file section, so the patch parser rejected it.

### Error
```text
apply_patch verification failed: invalid hunk
```

### Suggested Fix
Use one complete patch per file when updating several task-state files programmatically.

### Metadata
- Reproducible: yes
- Related Files: findings.md, progress.md, .learnings/ERRORS.md

### Resolution
- **Resolved**: 2026-07-18T14:37:00+08:00
- **Notes**: Reissued three valid file-scoped patches.

---

## [ERR-20260718-009] unsupported-browser-locator-scroll-method

**Logged**: 2026-07-18T14:39:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The in-app browser locator does not expose upstream Playwright `scrollIntoViewIfNeeded`.

### Error
```text
dailyInspector.scrollIntoViewIfNeeded is not a function
```

### Suggested Fix
Use the documented page-level `tab.dom_cua.scroll({x, y})` interface and verify with a fresh screenshot.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DailyJiraAudit.svelte

### Resolution
- **Resolved**: 2026-07-18T14:39:00+08:00
- **Notes**: Used documented DOM scrolling and visually verified the stacked inspector.

---

## [ERR-20260718-010] full-server-tests-loopback-bind-denied

**Logged**: 2026-07-18T14:43:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The complete server suite includes GitLab webhook tests that open an `httptest` loopback listener, which the sandbox denied.

### Error
```text
httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
```

### Suggested Fix
Rerun the exact server test command with managed loopback escalation after the focused sandbox-safe tests pass.

### Metadata
- Reproducible: yes
- Related Files: internal/server/config_handlers_gitlab_webhook_test.go
- Recurrence-Count: 3
- Last-Seen: 2026-08-12

### Resolution
- **Resolved**: 2026-07-18T14:44:00+08:00
- **Notes**: The complete `internal/server` suite passes under managed loopback approval; the 2026-08-12 solution-catalog run hit the same environment boundary after focused sandbox-safe tests passed.

---

## [ERR-20260718-011] sandbox-local-runtime-control-denied

**Logged**: 2026-07-18T16:03:00+08:00
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary
The sandbox blocked localhost curl access, process inspection, and terminating the confirmed stale backend process.

### Error
```text
curl: (7) Failed to connect to 127.0.0.1 port 8080
zsh:kill: kill failed: operation not permitted
```

### Suggested Fix
Use managed escalation for the exact localhost diagnostic and confirmed process-control commands; do not guess alternate ports or terminate unrelated processes.

### Metadata
- Reproducible: yes
- Related Files: internal/server/server.go

### Resolution
- **Resolved**: 2026-07-18T16:04:00+08:00
- **Notes**: Managed access confirmed the 404, identified the repository `go run` parent/child, restarted it, and verified 401 through direct and proxied routes.

---

## [ERR-20260718-001] chrome-hidden-wait-cdp-short-timeout

**Logged**: 2026-07-18T14:14:55+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Chrome's locator wait for the deconstruction loading label hit a short CDP evaluation timeout even though the LLM request was still running.

### Error
```text
Timed out after 3000ms waiting for selector internal:text="解构分析中"s: Timed out after 1ms waiting for CDP command Runtime.evaluate.
```

### Context
- Waited for the live AI deconstruction label to become hidden after submitting a harmless synthetic request.
- The requested locator timeout was longer, but the Chrome control layer failed its underlying evaluation earlier.

### Suggested Fix
Poll a narrowly scoped result/status container in separate browser calls and treat this control-layer timeout independently from the in-flight application request.

### Metadata
- Reproducible: unknown
- Related Files: web/src/components/Deconstructor.svelte

### Resolution
- **Resolved**: 2026-07-18T14:14:55+08:00
- **Notes**: Continued with short, targeted status reads without resubmitting the deconstruction request.

---

## [ERR-20260715-005] stale-component-subdirectory-assumption

**Logged**: 2026-07-15T17:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
A parallel source read assumed the components lived under `web/src/lib/components`, but this checkout keeps them under `web/src/components`.

### Error
```text
sed: web/src/lib/components/TaskKanban.svelte: No such file or directory
```

### Suggested Fix
Resolve component paths with `rg --files` before parallel line-range reads when prior notes omit or may stale the exact directory.

### Resolution
- **Resolved**: 2026-07-15T17:21:00+08:00
- **Notes**: Located the current files under `web/src/components` and stopped using the invalid paths.

---

## [ERR-20260715-006] stale-browser-toolbar-selector

**Logged**: 2026-07-15T18:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The final browser geometry check queried an obsolete execution-toolbar class and then dereferenced the missing element.

### Error
```text
TypeError: Cannot read properties of null (reading 'querySelectorAll')
```

### Suggested Fix
Resolve the active component class from the current source before the measurement and null-check optional DOM targets.

### Resolution
- **Resolved**: 2026-07-15T18:36:00+08:00
- **Notes**: Switched the check to the current `.phase41-execution-controls` surface.

---

## [ERR-20260714-006] markdown-workbench-css-block-closure

**Logged**: 2026-07-14T16:34:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
The first Svelte check after adding live Markdown mode found a missing closing brace in the existing visually-hidden preview access rule.

### Error
```text
web/src/components/shared/MarkdownWorkbench.svelte:588:1 Error: } expected (css)
```

### Context
- The patch removed an apparently duplicated brace without first matching it to the preceding multi-line selector block.
- TypeScript passed because the error was confined to the Svelte style parser.

### Suggested Fix
Inspect the numbered CSS block around any apparently redundant brace before removing it, then run `pnpm -C web check` immediately after structural Svelte edits.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/shared/MarkdownWorkbench.svelte

### Resolution
- **Resolved**: 2026-07-14T16:35:00+08:00
- **Notes**: Restored the missing block closure and scheduled the complete Svelte check for an immediate rerun.

---

## [ERR-20260714-007] svelte-constructor-parameter-property

**Logged**: 2026-07-14T16:37:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
Svelte's current preprocessing configuration rejected a TypeScript accessibility modifier on a constructor parameter.

### Error
```text
TypeScript language features like accessibility modifiers on constructor parameters are not natively supported
```

### Context
- `MarkdownTaskWidget` used `constructor(private checked: boolean)`.
- Plain typed class fields are supported and preserve identical runtime behavior.

### Suggested Fix
Use an explicit class field plus assignment in Svelte component scripts unless the repository enables full script preprocessing.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/shared/MarkdownWorkbench.svelte

### Resolution
- **Resolved**: 2026-07-14T16:38:00+08:00
- **Notes**: Replaced the parameter property with `checked: boolean` and a constructor assignment.

---

## [ERR-20260714-008] browser-runtime-process-conflict

**Logged**: 2026-07-14T16:40:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tests

### Summary
The in-app browser bootstrap again failed before session creation because the client attempted to redefine the protected runtime `process` property.

### Error
```text
Cannot redefine property: process
```

### Suggested Fix
After one real bootstrap attempt and the required Browser skill read, use the repository's established system-Chrome plus isolated local fixture fallback instead of retrying the same runtime conflict.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/config/CorpusSourceLibrary.svelte, web/src/components/shared/MarkdownWorkbench.svelte

### Resolution
- **Resolved**: 2026-07-14T16:41:00+08:00
- **Notes**: Switched to the bundled Playwright runtime and installed Chrome without touching production data.

---

## [ERR-20260714-009] browser-validation-ambiguous-scope-label

**Logged**: 2026-07-14T16:43:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first validation run used a strict `适用范围` text locator that matched both the import form and the advanced manual form.

### Error
```text
strict mode violation: getByText('适用范围', { exact: true }) resolved to 2 elements
```

### Suggested Fix
Anchor browser assertions to the unique shared component ID when repeated form labels are intentionally present on the page.

### Metadata
- Reproducible: yes
- Related Files: /tmp/well-ambient-phase67-browser.mjs

### Resolution
- **Resolved**: 2026-07-14T16:44:00+08:00
- **Notes**: Replaced the broad text wait with `#corpus-import-scope`.

---

## [ERR-20260713-005] apply-patch-binary-cleanup

**Logged**: 2026-07-13T00:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
`apply_patch` could not delete a temporary PNG validation artifact because it only accepts UTF-8 text input.

### Error
```text
apply_patch verification failed: invalid utf-8 sequence
```

### Resolution
Use a scoped filesystem removal only for the binary artifact created by this task; continue using `apply_patch` for source and text-file edits.

---

## [ERR-20260713-004] multiselect-focus-layout-click-through

**Logged**: 2026-07-13T10:02:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
Opening the in-flow multi-select during input focus changed layout before the originating click completed and allowed an option to be selected accidentally.

### Error
```text
Clicking the candidate combobox added 张路路 and left no visible listbox.
```

### Resolution
Focus no longer opens either shared combobox. Multi-select opens after the completed input click, while text input and ArrowDown retain explicit open behavior.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/shared/MultiSelect.svelte, web/src/components/shared/Select.svelte

---

## [ERR-20260713-003] process-inspection-sandbox

**Logged**: 2026-07-13T10:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: infra

### Summary
The sandbox blocked `ps` while inspecting the process serving port 5175.

### Error
```text
zsh: operation not permitted: ps
```

### Resolution
Used read-only `lsof` instead and confirmed PID 4420 serves from `/Users/eddie/Workspace/well-ambient/web`; an approved loopback curl then confirmed the live module contains the latest implementation.

### Metadata
- Reproducible: yes
- Related Files: task_plan.md

---

## [ERR-20260713-001] inline-dropdown-pointerdown-toggle

**Logged**: 2026-07-13T09:42:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary
Opening an inline-expanding dropdown exposed two hit-test defects: premature layout mutation and a chevron positioned relative to the expanding wrapper instead of the fixed trigger.

### Error
```text
Expected 3 selected reviewers after opening; browser snapshot showed 4 and added 张路路.
```

### Context
- Actual `MultiSelect.svelte` mounted in the temporary browser validation harness.
- Chevron pointerdown both prevented default and opened the in-flow option region.

### Resolution
Pointerdown now only prevents focus transfer and parent propagation. The parent trigger no longer forces every click back through `openDropdown`, the click toggle is deferred until the pointer sequence finishes, and the trigger itself is now the chevron's positioning context so the control cannot move over list options.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/shared/MultiSelect.svelte

---

## [ERR-20260713-002] apply-patch-stale-progress-context

**Logged**: 2026-07-13T09:49:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary
A combined final-state patch used an outdated top-of-file context for `progress.md` and was rejected atomically.

### Resolution
Re-read the current task, progress, findings, and learning sections, then applied smaller exact-context patches successfully.

### Metadata
- Reproducible: yes
- Related Files: task_plan.md, progress.md, findings.md, .learnings/LEARNINGS.md

---

## [ERR-20260712-005] local-browser-error-page-lock

**Logged**: 2026-07-12T23:30:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: frontend-validation

### Summary
The local Vite listener was initially stopped, and the in-app browser navigated to Chrome's generated connection-error `data:` page. After the server restarted, browser security policy blocked navigation and DOM interaction from that error page.

### Resolution
Did not bypass the browser policy or switch browser surfaces. Completed the focused Go regression, Svelte type check, production build, Impeccable complete/layout scans, responsive source review, and diff hygiene; recorded authenticated live interaction as an explicit validation exception.

---

## [ERR-20260705-001] bundled_soffice_missing_little_cms

**Logged**: 2026-07-05T13:15:45Z
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
Bundled `soffice` failed during DOCX-to-PDF rendering because its LibreOffice runtime referenced a missing Homebrew `little-cms2` dynamic library.

### Error
```text
dyld: Library not loaded: /opt/homebrew/opt/little-cms2/lib/liblcms2.2.dylib
Referenced from: .../dependencies/native/libreoffice-headless/libreoffice/LibreOfficeDev.app/Contents/Frameworks/libvcllo.dylib
Reason: tried: '/opt/homebrew/opt/little-cms2/lib/liblcms2.2.dylib' ... (no such file)
```

### Context
- Operation attempted: convert `output/doc/个人简介内容_润色版.docx` to PDF for visual QA.
- Runtime binary: `/Users/eddie/.cache/codex-runtimes/codex-primary-runtime/dependencies/bin/soffice`.
- System `/Applications/LibreOffice.app/Contents/MacOS/soffice` was not installed.

### Suggested Fix
For DOCX visual QA in this workspace, try `textutil`, `qlmanage`, or install/fix LibreOffice dependencies before relying on the bundled `soffice`.

### Metadata
- Reproducible: yes
- Related Files: output/doc/个人简介内容_润色版.docx

---

## [ERR-20260620-002] apply_patch_context

**Logged**: 2026-06-20T17:37:00+08:00
**Priority**: low
**Status**: pending
**Area**: tooling

### Summary
Patch context drifted while updating task memory, causing an `apply_patch` verification failure.

### Error
```text
apply_patch verification failed: Failed to find expected lines in task_plan.md
```

### Context
- Operation attempted: mark Phase 11 complete and append a decision row.
- The plan file already had nearby edits from prior sessions, so the larger context did not match exactly.

### Suggested Fix
Before patching long-lived task memory files, read the exact current section and patch only the smallest stable block.

### Metadata
- Reproducible: yes
- Related Files: task_plan.md
- See Also: ERR-20260620-001

---

## [ERR-20260620-003] go_test_httptest_loopback

**Logged**: 2026-06-20T18:04:00+08:00
**Priority**: low
**Status**: pending
**Area**: tests

### Summary
Full Go tests can fail inside the sandbox when `httptest.NewServer` needs a loopback listener.

### Error
```text
panic: httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
```

### Context
- Operation attempted: `GOCACHE=/tmp/well-ambient-gocache go test ./... -count=1`.
- Targeted non-listener server tests passed; full suite failed only when GitLab webhook tests started an `httptest` server.

### Suggested Fix
Use `/tmp` Go cache and request loopback permission for full Go validation that exercises `httptest.NewServer`.

### Metadata
- Reproducible: yes
- Related Files: internal/server/config_handlers_gitlab_webhook_test.go

---

## [ERR-20260620-004] spreadsheet_node_repl_meta

**Logged**: 2026-06-20T20:18:00+08:00
**Priority**: low
**Status**: pending
**Area**: tooling

### Summary
`node_repl` failed to import/read a workbook due missing sandbox metadata, while bundled Node plus `@oai/artifact-tool` worked.

### Error
```text
Mcp error: -32602: js: codex/sandbox-state-meta: missing field `sandboxPolicy`
```

### Context
- Operation attempted: use `mcp__node_repl.js` to import `/Users/eddie/Workspace/well-ambient/全局领航能力汇总.xlsx` with `@oai/artifact-tool`.
- Fallback used `/Users/eddie/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node` with a temporary script in `/private/tmp/well-ambient-sheet` and a symlinked bundled `node_modules`.

### Suggested Fix
For spreadsheet extraction in this desktop runtime, prefer bundled Node + temporary `.mjs` script when `node_repl` reports sandbox metadata errors.

### Metadata
- Reproducible: unknown
- Related Files: 全局领航能力汇总.xlsx

---

## [ERR-20260623-001] tracked_task_memory_overwrite

**Logged**: 2026-06-23T00:00:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tooling

### Summary
Task-local planning files in this repository are tracked long-lived memory files; replacing them with fresh planning templates erased prior task history in the worktree.

### Error
```text
git restore -- task_plan.md findings.md progress.md
fatal: Unable to create '/Users/eddie/Workspace/well-ambient/.git/index.lock': Operation not permitted
```

### Context
- Operation attempted: create fresh `task_plan.md`, `findings.md`, and `progress.md` for a complex UI/backend task.
- These files already existed in git and contained long-running project memory, so adding fresh templates was the wrong approach.
- A sandboxed `git restore` could not write the git index lock; elevated restore was used only for those three files.

### Suggested Fix
Before using planning-with-files in this repository, check whether `task_plan.md`, `findings.md`, and `progress.md` are tracked. If they are tracked, append a small new phase/update instead of replacing the file.

### Metadata
- Reproducible: yes
- Related Files: task_plan.md, findings.md, progress.md

### Resolution
- **Resolved**: 2026-06-23T00:00:00+08:00
- **Notes**: Restored the three files to HEAD and continued implementation without staging the accidental overwrite.

---
# [ERR-20260711-001] frontend-path-assumption

**Logged**: 2026-07-11T10:55:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
Frontend component inspection initially used `web/src/lib` instead of the repository's `web/src/components` path.

### Error
```
sed: web/src/lib/DemandKanban.svelte: No such file or directory
```

### Context
- Read-only inspection command for flow board and AI deconstruction components.
- The repository stores these Svelte components under `web/src/components`.

### Suggested Fix
Resolve component paths with `rg --files` before assuming a source subdirectory.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DemandKanban.svelte

---

# [ERR-20260712-002] skill-installer-python-ca-chain

**Logged**: 2026-07-12T22:10:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tooling

### Summary
The GitHub skill installer's default Python download transport failed local certificate-chain verification.

### Error
```text
ssl.SSLCertVerificationError: certificate verify failed: unable to get local issuer certificate
```

### Resolution
Re-ran the same official installer with `--method git`. Both scoped Impeccable installs completed without disabling certificate verification or copying the full repository.

---

# [ERR-20260712-003] warm-memory-shape-assumption

**Logged**: 2026-07-12T22:28:00+08:00
**Priority**: low
**Status**: resolved
**Area**: task-state

### Summary
A read-only state probe assumed `warm_memory` was an array and called `slice()`, but the project schema stores it as an object.

### Resolution
Inspected the live schema first, then used exact object fields for the final state update.

---

# [ERR-20260712-004] impeccable-broad-scan-scope

**Logged**: 2026-07-12T22:46:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend-validation

### Summary
A complete Impeccable scan included the entire legacy `DemandKanban.svelte` file and returned unrelated historical warnings outside the detail-modal change.

### Resolution
Kept those findings as explicit existing debt, ran the complete detector on the two changed UI components, and ran the layout-scoped detector across all three target files. Both scoped gates returned `[]` without broadening the requested change.

---
## [ERR-20260712-003] shell_glob

**Logged**: 2026-07-12T23:23:00+08:00
**Priority**: low
**Status**: resolved
**Area**: infra

### Summary
An optional `.env*` path glob aborted a combined ripgrep diagnostic under zsh.

### Error
```
zsh: no matches found: .env*
```

### Context
- Searched optional environment files alongside explicit source paths.
- zsh expanded the unmatched glob before ripgrep could run.

### Suggested Fix
Quote optional globs or omit them and search only discovered explicit paths.

### Metadata
- Reproducible: yes
- Related Files: task_plan.md

### Resolution
- **Resolved**: 2026-07-12T23:23:00+08:00
- **Commit/PR**: none
- **Notes**: Continued with explicit source paths and no repeated wildcard invocation.

---

## [ERR-20260713-006] browser-local-server-unavailable

**Logged**: 2026-07-13T11:43:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Authenticated browser validation initially targeted a stopped local Vite server.

### Error
```text
net::ERR_CONNECTION_REFUSED at http://127.0.0.1:5175/
```

### Context
- Browser validation reused the prior Phase 59 development URL without first verifying the listener.
- The source and production build were valid; only the local validation server was absent.

### Suggested Fix
Check the local listener or start the scoped Vite development server before opening the validation tab.

### Metadata
- Reproducible: yes
- Related Files: web/package.json

### Resolution
- **Resolved**: 2026-07-13T11:44:00+08:00
- **Commit/PR**: none
- **Notes**: Started the local-only Vite server and reused the existing browser binding.

---

## [ERR-20260713-007] multiselect-chip-removal-focus-exit

**Logged**: 2026-07-13T11:52:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: frontend

### Summary
Removing a selected chip while the multi-select was open still closed the list through focus exit.

### Error
```text
After clicking “移除 白凌云”, the selected chips were empty but visible listboxes changed from 1 to 0.
```

### Context
- The direct `finishSelection()` call had already been removed.
- Pointerdown focused the chip's remove button; the subsequent value update removed that focused button from the DOM, producing a focusout with no in-component related target.

### Suggested Fix
Prevent the remove button's pointerdown from moving focus, while preserving its click action and stopping propagation inside the component.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/shared/MultiSelect.svelte

### Resolution
- **Resolved**: 2026-07-13T11:53:00+08:00
- **Commit/PR**: none
- **Notes**: Added a non-mutating pointerdown guard to the chip remove button; browser validation covers the open-state removal path.

---

## [ERR-20260713-008] browser-cross-locator-filter

**Logged**: 2026-07-13T12:42:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Chrome validation could not compose one locator into another locator's `filter({ has })` option.

### Error
```text
Cannot read private member #e from an object whose class did not declare it
```

### Context
- Tried to identify the DG-319 demand card by combining a card locator with a nested Jira-link locator.
- The browser control layer rejected the cross-locator object even though both locators belonged to the same tab.

### Suggested Fix
After refreshing the DOM snapshot, prefer an exact unique visible-text locator or one self-contained stable CSS selector instead of passing locator instances across `filter({ has })`.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DemandKanban.svelte

### Resolution
- **Resolved**: 2026-07-13T12:43:00+08:00
- **Commit/PR**: none
- **Notes**: Re-snapshotted, confirmed the exact demand title was unique, and opened the detail through that supported locator.

---

## [ERR-20260713-009] zsh-unmatched-source-glob

**Logged**: 2026-07-13T13:17:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
Source discovery failed because zsh expanded an unmatched root `*.go`/`*.yml` glob before ripgrep ran.

### Error
```text
zsh:1: no matches found: *.go
```

### Suggested Fix
Discover files with `rg --files` first or quote optional globs so the shell cannot reject an empty match.

### Resolution
- **Resolved**: 2026-07-13T13:17:00+08:00
- **Notes**: Continued with explicit paths and `rg --files`; no wildcard retry.

---

## [ERR-20260713-010] go-build-default-cache-denied

**Logged**: 2026-07-13T13:18:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The browser-validation server build used Go's default cache outside the writable sandbox.

### Error
```text
open /Users/eddie/Library/Caches/go-build/...: operation not permitted
```

### Suggested Fix
Use `GOCACHE=/tmp/well-ambient-gocache` for every Go build/test command in this workspace.

### Metadata
- Reproducible: yes
- Related Files: internal/server/daily_jira_handlers_test.go
- Recurrence-Count: 2
- Last-Seen: 2026-07-18

### Resolution
- **Resolved**: 2026-07-13T13:18:00+08:00
- **Notes**: Rebuilt successfully with the workspace-standard `/tmp` cache.

---

## [ERR-20260713-011] isolated-browser-historical-background-actions

**Logged**: 2026-07-13T13:29:00+08:00
**Priority**: high
**Status**: resolved
**Area**: tests

### Summary
An isolated backend using a copied historical database activated delay-alert background processing even though Jira, GitLab, Feishu, and AI were disabled in the temporary config.

### Error
```text
Email extension hook: Send delay alert email to ...
```

### Suggested Fix
Use a purpose-built fixture database with only the required user, demand, draft, and contract rows for browser validation. Do not reuse a production-like historical database for isolated full-app runs.

### Resolution
- **Resolved**: 2026-07-13T13:29:00+08:00
- **Notes**: Stopped the server, accepted the safety rejection on restart, and completed validation with authenticated non-provider UI checks, loopback protocol tests, and a structural footer-order assertion.

---

## [ERR-20260713-012] finalized-chrome-tab-binding-reuse

**Logged**: 2026-07-13T23:16:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
A Chrome tab binding from the completed Settings validation was no longer defined after the tab had been finalized.

### Error
```text
settingsTab is not defined
```

### Context
- Attempted to reuse the prior task's `settingsTab` binding for a new Settings refinement pass.
- The persistent Chrome connection remains valid; only the finalized tab binding is stale.

### Suggested Fix
After a task finalizes browser tabs, obtain a fresh tab from the existing browser binding on the next validation pass instead of assuming the old page variable survives.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/SettingsPanel.svelte

### Resolution
- **Resolved**: 2026-07-13T23:16:00+08:00
- **Commit/PR**: none
- **Notes**: The next browser step will claim a fresh authenticated tab through the existing Chrome binding.

---

## [ERR-20260713-013] config-component-path-assumption

**Logged**: 2026-07-13T23:34:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The first Phase 64 source inspection assumed the corpus configuration components lived directly under `web/src/components`.

### Error
```text
nl: web/src/components/CorpusCandidateReview.svelte: No such file or directory
rg: web/src/components/AIConfig.svelte: No such file or directory
```

### Suggested Fix
Resolve component locations with `rg --files web/src` before opening guessed paths in this repository.

### Resolution
- **Resolved**: 2026-07-13T23:34:00+08:00
- **Notes**: Located both files under `web/src/components/config` and resumed inspection there.

---

## [ERR-20260713-014] settings-phase-class-warning-explosion

**Logged**: 2026-07-13T23:48:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: frontend

### Summary
Removing the legacy Settings Phase classes from the live root correctly disabled obsolete height rules, but caused Svelte to report hundreds of newly unused legacy selectors.

### Error
```text
svelte-check found 0 errors and 522 warnings in 9 files
```

### Suggested Fix
Until the historical CSS blocks are removed in a dedicated cleanup, keep their compatibility class names and use one stable unified-root selector to own current layout geometry above them.

### Resolution
- **Resolved**: 2026-07-13T23:48:00+08:00
- **Notes**: Restored Phase compatibility classes, added `#settings-unified-root`, and scoped the authoritative panel geometry plus policy semantic overrides to that root.

---

## [ERR-20260714-001] responsive-metric-assumed-active-tab

**Logged**: 2026-07-14T00:03:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first narrow corpus metric assumed the off-canvas sidebar button had changed routes and then treated the missing library grid as an active-tab mismatch. A second tab click proved the target route had not mounted at all.

### Error
```text
TypeError: getComputedStyle expects an Element
Playwright selector deadline exceeded
waiting on click for selector internal:role=tab[name=/^资料库/]
```

### Suggested Fix
Switch routes while the desktop sidebar is visibly available, then apply the requested narrow viewport. Select the exact task tab and null-check optional DOM targets before measuring.

### Resolution
- **Resolved**: 2026-07-14T00:03:00+08:00
- **Notes**: Responsive route validation now follows desktop navigation first, viewport override second, with guarded state metrics.

---

## [ERR-20260714-002] null-memberships-permission-route

**Logged**: 2026-07-14T00:16:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: frontend

### Summary
Authenticated permission-route validation exposed a user record whose runtime `memberships` value was null even though the frontend type declared an array.

### Error
```text
TypeError: Cannot read properties of null (reading 'filter')
at groupMemberCount (SettingsPanel.svelte)
```

### Suggested Fix
Normalize API collection fields at the read boundary before filtering or iterating them.

### Resolution
- **Resolved**: 2026-07-14T00:16:00+08:00
- **Notes**: Added `membershipsForUser()` and reused it in coverage counting plus member-table rendering.

---

## [ERR-20260714-003] chrome-new-tab-url-ignored

**Logged**: 2026-07-14T00:19:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome client accepted `tabs.new({ url })` but created an `about:blank` tab in this runtime.

### Error
```text
url: about:blank
```

### Suggested Fix
Create the tab first and then call `tab.goto(url)` explicitly.

### Resolution
- **Resolved**: 2026-07-14T00:19:00+08:00
- **Notes**: Explicit `goto('http://localhost:5173/')` loaded the authenticated app successfully.

---

## [ERR-20260714-004] final-route-forcing-timeout

**Logged**: 2026-07-14T00:23:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The final browser handoff tried to force the original validation tab onto the policy list after resetting the viewport, but the expected tab locator was not mounted in that tab state.

### Error
```text
Playwright selector deadline exceeded
waiting on click for selector internal:role=tab[name=/^策略列表/]
```

### Suggested Fix
Do not force a cosmetic final route after validation is complete; preserve the authenticated tab's current state and finalize it directly.

### Resolution
- **Resolved**: 2026-07-14T00:23:00+08:00
- **Notes**: Finalized the existing authenticated Settings tab without another route mutation.

---

## [ERR-20260714-005] context-import-invalid-test-fixtures

**Logged**: 2026-07-14T15:36:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first full Go run failed two new context-import boundary tests because the JSON fixture encoded literal backslash-n text and the missing-file fixture supplied an invalid empty multipart stream.

### Error
```text
manual Markdown source contained literal \\n sequences
read multipart body: multipart: NextPart: EOF
```

### Suggested Fix
Represent JSON newline escapes once inside the raw fixture string, and build a valid multipart form containing metadata but no file when testing the missing-file branch.

### Resolution
- **Resolved**: 2026-07-14T15:36:00+08:00
- **Notes**: Corrected both fixtures without changing the ingestion implementation and scheduled the full suite for an immediate rerun.

---

## [ERR-20260715-001] go-build-cache-permission

**Logged**: 2026-07-15T16:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first Go validation attempt could not write the default user build cache in the managed workspace.

### Error
```text
permission denied while writing the default Go build cache
```

### Suggested Fix
Use an isolated writable cache under `/tmp` for repository validation instead of retrying the same default cache.

### Resolution
- **Resolved**: 2026-07-15T16:01:00+08:00
- **Notes**: Subsequent Go-related validation uses `GOCACHE=/tmp/well-ambient-gocache`.

---

## [ERR-20260715-002] unsafe-live-backend-validation-escalation

**Logged**: 2026-07-15T16:18:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tests

### Summary
Starting the real backend for UI validation would have loaded live credentials, listened broadly, and enabled development authentication.

### Error
```text
escalation rejected: live credentials + 0.0.0.0 listener + development authentication
```

### Suggested Fix
Validate the built frontend against an isolated same-origin fixture server bound only to `127.0.0.1` with synthetic authenticated data.

### Resolution
- **Resolved**: 2026-07-15T16:20:00+08:00
- **Notes**: No real backend or live data was used; all browser mutations stayed inside the disposable local fixture.

---

## [ERR-20260715-003] browser-local-url-policy-reconnect

**Logged**: 2026-07-15T16:40:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
After an already-successful browser validation session reset, the replacement tab entered a local connection-error page and the browser URL policy refused programmatic reconnection to the fixture.

### Error
```text
Browser Use rejected the local navigation due to URL policy.
```

### Suggested Fix
Do not bypass the browser policy. Retain the successful measurements already collected and complete validation with current build, type, diff, detector, and source-contract checks.

### Resolution
- **Resolved**: 2026-07-15T16:41:00+08:00
- **Notes**: The browser session was finalized and viewport overrides were reset without further navigation attempts.

---

## [ERR-20260715-004] ripgrep-leading-hyphen-pattern

**Logged**: 2026-07-15T17:05:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
A CSS token search pattern beginning with `--` was parsed by ripgrep as a command flag.

### Error
```text
rg: unrecognized flag --z|z-index|overlay|dropdown|popover
```

### Context
- Attempted a combined CSS token and z-index search while diagnosing dropdown stacking.
- The pattern began with `--z`, so `rg` treated it as an option instead of a pattern.

### Suggested Fix
Terminate options before any pattern that can begin with a hyphen: `rg -- '<pattern>' <paths>`.

### Metadata
- Reproducible: yes
- Related Files: web/src/styles/modern-admin-tokens.css

### Resolution
- **Resolved**: 2026-07-15T17:06:00+08:00
- **Notes**: Continue with `rg -- '<pattern>'`; do not retry the failing form.

---

## [ERR-20260718-002] server-test-concurrent-source-gap

**Logged**: 2026-07-18T14:16:20+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first elevated server test compile observed route registrations before the concurrently edited Jira daily-audit handler file was present.

### Error
```text
internal/server/server.go:66:87: s.handleGetDailyJiraAudit undefined
internal/server/server.go:67:129: s.handlePostDailyJiraReview undefined
```

### Context
- The worktree already contained unrelated active edits.
- `internal/server/daily_jira_handlers.go` appeared immediately after the failed compile and defined both handlers.

### Suggested Fix
When a dirty worktree is changing concurrently, re-check the exact missing symbols and rerun the narrow test once the referenced file is present; do not patch unrelated in-progress code.

### Metadata
- Reproducible: no
- Related Files: internal/server/server.go, internal/server/daily_jira_handlers.go

### Resolution
- **Resolved**: 2026-07-18T14:17:10+08:00
- **Notes**: Reran the focused deconstruction server tests after the handler file appeared; all selected tests passed.

---

## [ERR-20260718-003] context-pack-schema-column-assumption

**Logged**: 2026-07-18T14:19:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
A read-only diagnostic query used `budget_tokens`, while the current `context_packs` table names the column `token_budget`.

### Error
```text
no such column: budget_tokens
```

### Suggested Fix
Inspect the current SQLite table schema before querying task-generated diagnostic artifacts.

### Metadata
- Reproducible: yes
- Related Files: well-ambient.db

### Resolution
- **Resolved**: 2026-07-18T14:19:00+08:00
- **Notes**: Reissued the read-only query with the actual `token_budget` column.

---

## [ERR-20260718-013] local-vite-sandbox-bind

**Logged**: 2026-07-18T20:22:30+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The local Vite validation server could not bind a loopback port inside the restricted sandbox.

### Error
```text
Error: listen EPERM: operation not permitted 127.0.0.1:4173
```

### Suggested Fix
When browser-visible local validation requires a loopback server, retry the narrowly scoped `pnpm dev` command with the managed local-server approval.

### Metadata
- Reproducible: yes
- Related Files: web/package.json

### Resolution
- **Resolved**: 2026-07-18T20:23:25+08:00
- **Notes**: Started the same loopback-only server under the approved prefix, completed validation, and stopped it afterward.

---

## [ERR-20260718-014] persistent-browser-locator-name-collision

**Logged**: 2026-07-18T20:28:40+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
A reused browser-control variable name resolved to a locator from an earlier session and targeted the wrong tab label.

### Error
```text
Timed out waiting for a stale tab locator instead of the visible "当天 1" tab.
```

### Suggested Fix
Use task-specific fresh binding names in the persistent browser kernel and take a fresh DOM snapshot after any locator timeout before rebuilding the locator.

### Metadata
- Reproducible: no
- Related Files: web/src/components/DailyJiraAudit.svelte

### Resolution
- **Resolved**: 2026-07-18T20:29:10+08:00
- **Notes**: Re-snapshotted the page, used a unique locator binding, and verified the green healthy state successfully.

---

## [ERR-20260718-015] chrome-browser-client-owner-methods

**Logged**: 2026-07-18T21:08:30+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Chrome session naming and user-tab claiming were called on the wrong browser-client owners.

### Error
```text
agent.setSessionName is not a function
chromeValidation.tabs.claim is not a function
```

### Context
- The persistent Chrome browser binding was still valid after the previous task finalized its controlled tabs.
- The attempted calls treated session naming as an `agent` method and user-tab claiming as a `tabs` method.

### Suggested Fix
Call `browserBinding.nameSession(name)` for the session label and `browserBinding.user.claimTab(tabId)` for an existing user tab. Use `browserBinding.tabs.finalize(...)` only for end-of-task cleanup.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DailyJiraAudit.svelte
- See Also: ERR-20260718-014

### Resolution
- **Resolved**: 2026-07-18T21:09:00+08:00
- **Notes**: Claimed the authenticated well-ambient tab through `chromeValidation.user.claimTab`, completed all geometry checks, and retained the user-owned tab during finalization.

---

## [ERR-20260718-016] frontend-check-workdir

**Logged**: 2026-07-18T22:04:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The frontend type check was started from the repository root even though the pnpm importer manifest lives under `web/`.

### Error
```text
ERR_PNPM_NO_IMPORTER_MANIFEST_FOUND No package.json was found in /Users/eddie/Workspace/well-ambient
```

### Context
- Command attempted: `pnpm check`
- The repository is not a root pnpm workspace; frontend scripts are owned by `web/package.json`.

### Suggested Fix
Run frontend scripts with `workdir=/Users/eddie/Workspace/well-ambient/web` or use `pnpm --dir web <script>`.

### Metadata
- Reproducible: yes
- Related Files: web/package.json

### Resolution
- **Resolved**: 2026-07-18T22:04:20+08:00
- **Notes**: Subsequent frontend checks were routed through the `web/` importer.

---

## [ERR-20260718-017] duplicate-drawer-close-label

**Logged**: 2026-07-18T22:14:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The event drawer backdrop and close button shared the same accessible name, so a strict role locator matched both during keyboard validation.

### Error
```text
strict mode violation: getByRole('button', { name: '关闭事件记录' }) resolved to 2 elements
```

### Context
- The backdrop is intentionally a full-viewport button and the header has a separate close button.
- Both controls originally exposed `aria-label="关闭事件记录"`.

### Suggested Fix
Give the backdrop a distinct accessible name while keeping the visible close action unchanged.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionEventCenter.svelte

### Resolution
- **Resolved**: 2026-07-18T22:14:20+08:00
- **Notes**: Renamed the backdrop to `关闭事件记录背景层`; the header close button retains `关闭事件记录`.

---

## [ERR-20260718-018] collapsed-sidebar-submenu-locator

**Logged**: 2026-07-18T22:19:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The narrow-screen browser check tried to click the Daily Jira submenu while the responsive sidebar kept that submenu collapsed.

### Error
```text
Playwright selector deadline exceeded waiting on getByRole('button', { name: /每日 Jira 决策看板/ })
```

### Context
- Viewport width was 520px.
- The top-level Decision Dashboard route changed successfully, but the child menu was not interactable in the collapsed navigation state.

### Suggested Fix
Take a fresh DOM snapshot at the narrow breakpoint and use the visible menu-expansion control before selecting Daily Jira.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/prototype/FunctionalAdminShell.svelte

### Resolution
- **Resolved**: 2026-07-18T22:21:00+08:00
- **Notes**: Opened the mobile rail before each navigation level, reached Daily Jira through the visible submenu, and completed the 520px viewport check.

---

## [ERR-20260718-019] chrome-console-method-name

**Logged**: 2026-07-18T22:23:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome browser-client Playwright facade does not expose a `consoleMessages()` method.

### Error
```text
eventTab.playwright.consoleMessages is not a function
```

### Context
- DOM, locator, keyboard, and evaluation APIs were working normally.
- The failure was limited to choosing the wrong diagnostic method name.

### Suggested Fix
Inspect the installed browser-client API definition and use its supported console or page-error reader.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionEventCenter.svelte

### Resolution
- **Resolved**: 2026-07-18T22:31:00+08:00
- **Notes**: The installed Chrome facade does not expose the attempted method. Runtime validation instead completed through successful route, drawer, keyboard, focus, and geometry interactions, together with passing static checks and design detectors.

---

## [ERR-20260718-020] nonextensible-page-window

**Logged**: 2026-07-18T22:26:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome validation environment prevents adding custom properties to the page `window` object.

### Error
```text
TypeError: Cannot add property __eventValidationLogs, object is not extensible
```

### Context
- A temporary property was intended to hold runtime error messages during route replays.
- Page evaluation itself remained available.

### Suggested Fix
Store temporary diagnostic output in a hidden DOM element and keep event-handler state inside a closure instead of extending `window`.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionEventCenter.svelte

### Resolution
- **Resolved**: 2026-07-18T22:26:20+08:00
- **Notes**: Switched the temporary runtime log collector to a hidden DOM node.

---

## [ERR-20260718-021] browser-evaluate-dom-construction

**Logged**: 2026-07-18T22:28:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The controlled Chrome evaluation facade does not expose `document.createElement` as a callable DOM method.

### Error
```text
TypeError: document.createElement is not a function
```

### Context
- The attempted hidden-node collector followed the fallback from ERR-20260718-020.
- Existing DOM querying and element attribute APIs remained available.

### Suggested Fix
Use a temporary data attribute on the existing document root instead of creating a new node.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionEventCenter.svelte
- See Also: ERR-20260718-020

### Resolution
- **Resolved**: 2026-07-18T22:28:20+08:00
- **Notes**: Routed temporary diagnostic output through an attribute on `document.documentElement`.

---

## [ERR-20260718-022] browser-evaluate-dom-mutation

**Logged**: 2026-07-18T22:29:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The controlled Chrome evaluation facade exposes DOM state for reading but not root-element mutation methods such as `setAttribute`.

### Error
```text
TypeError: document.documentElement.setAttribute is not a function
```

### Context
- This was the second fallback for storing temporary console diagnostics inside the page.
- All requested product interactions and read-only geometry evaluations still worked.

### Suggested Fix
Do not inject a page-side log collector through this facade. Use the browser client's native log command when available, or rely on clean runtime interactions plus static checks and record the diagnostic limitation.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionEventCenter.svelte
- See Also: ERR-20260718-019, ERR-20260718-020, ERR-20260718-021

### Resolution
- **Resolved**: 2026-07-18T22:29:20+08:00
- **Notes**: Stopped attempting page mutation and retained the successful interaction, type, build, and detector evidence.

---

## [ERR-20260718-023] chrome-tab-attach-api-drift

**Logged**: 2026-07-18T23:08:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The current Chrome browser facade can discover user tabs but does not expose the previously assumed `tabs.attach` method.

### Error
```text
chrome.tabs.attach is not a function
```

### Context
- The authenticated `http://localhost:5173/` tab was returned by `chrome.user.openTabs()`.
- The stale bound tab needed to be replaced without losing the user's login state.

### Suggested Fix
Inspect the current browser facade methods and use its supported existing-tab activation or binding API instead of assuming `tabs.attach`.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionDashboard.svelte

### Resolution
- **Resolved**: 2026-07-18T23:10:00+08:00
- **Notes**: Used `chrome.user.openTabs()` and passed the exact returned tab object to `chrome.user.claimTab(tab)`, preserving the authenticated page.

---

## [ERR-20260718-024] pnpm-check-root-without-manifest

**Logged**: 2026-07-18T23:22:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The frontend check was first invoked from the repository root, which does not own a package manifest.

### Error
```text
ERR_PNPM_NO_IMPORTER_MANIFEST_FOUND No package.json was found in /Users/eddie/Workspace/well-ambient
```

### Context
- The only package manifest for the Svelte frontend is `web/package.json`.
- No implementation files were changed by the failed command.

### Suggested Fix
Run pnpm frontend scripts with `workdir=/Users/eddie/Workspace/well-ambient/web`.

### Metadata
- Reproducible: yes
- Related Files: web/package.json

### Resolution
- **Resolved**: 2026-07-18T23:22:30+08:00
- **Notes**: Located `web/package.json` and moved all subsequent frontend checks to the `web` package directory.

---

## [ERR-20260718-025] chrome-locator-focus-method

**Logged**: 2026-07-18T23:27:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The current Chrome locator facade supports `press()` but does not expose a Playwright-style `focus()` method.

### Error
```text
longAgendaRow.focus is not a function
```

### Context
- The row keyboard-opening path needed validation without triggering any live mutation.
- Browser API documentation lists locator `press(value, options)` as the supported keyboard method.

### Suggested Fix
Call `locator.press('Space')` or `locator.press('Enter')` directly instead of a separate `focus()` call.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionDashboard.svelte

### Resolution
- **Resolved**: 2026-07-18T23:27:20+08:00
- **Notes**: Switched the keyboard validation path to the documented locator `press()` method.

---
## [ERR-20260719-026] chrome-viewport-requires-integer-dimensions

**Logged**: 2026-07-19T10:24:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome viewport calibration call rejected a fractional width while validating responsive drawer geometry.

### Error
```
Expected integer, received float
path: width
```

### Context
- Operation: `viewportControl.set()` during 1440/1024/760/520 responsive validation.
- Input included `width: 921.6` to compensate for the browser's 90% zoom calibration.
- The viewport API schema accepts integers only.

### Suggested Fix
Round every calibrated width and height before calling `viewportControl.set()`, then verify the actual CSS viewport returned by the page.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionDashboard.svelte
- See Also: ERR-20260718-023

### Resolution
- **Resolved**: 2026-07-19T10:29:00+08:00
- **Notes**: Rounded calibrated viewport dimensions to integers; the 1440/1024/760/520 responsive geometry sweep completed successfully.

---
## [ERR-20260719-027] chrome-cua-scroll-parameter-names

**Logged**: 2026-07-19T10:31:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome computer-action scroll call rejected Playwright-style `deltaY` parameters during short-viewport drawer validation.

### Error
```
cua.scroll requires x, y, scrollX, and scrollY
```

### Context
- Operation: wheel-scroll the hidden-rail requirement drawer at a 1024x650 CSS viewport.
- Attempted input used `deltaY: 420`.
- This browser facade requires explicit `scrollX` and `scrollY` fields.

### Suggested Fix
Call `cua.scroll({ x, y, scrollX: 0, scrollY: amount })` and verify the target container's `scrollTop` changes.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionDashboard.svelte
- See Also: ERR-20260718-025
- Recurrence-Count: 2
- Last-Seen: 2026-07-19

### Resolution
- **Resolved**: 2026-07-19T10:32:00+08:00
- **Notes**: Re-ran with `scrollX` and `scrollY`; the hidden-rail drawer reached its 130px maximum scroll and exposed the action row while the document remained viewport-locked.
- **Follow-up**: The flattened-drawer check again confirmed that Chrome scrolling requires all four fields: `x`, `y`, `scrollX`, and `scrollY`; the 1024x650 drawer reached its 113px maximum while the document stayed locked.

---
## [ERR-20260719-028] browser-evaluate-parsefloat-shadowing

**Logged**: 2026-07-19T11:08:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
An authenticated drawer geometry check failed because the page evaluation context did not expose the expected global `parseFloat` function.

### Error
```
TypeError: parseFloat is not a function
```

### Context
- Operation: derive the rendered line count for a long requirement title from its computed line height.
- The evaluation called unqualified `parseFloat(...)` inside the page context.
- All prior drawer measurements completed; only the derived line-count probe failed.

### Suggested Fix
Use `Number.parseFloat(getComputedStyle(element).lineHeight)` in browser evaluation code and rerun the complete interaction cleanup sequence.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionDashboard.svelte
- See Also: ERR-20260718-025

### Resolution
- **Resolved**: 2026-07-19T11:09:00+08:00
- **Notes**: Re-ran the long-title geometry check with `Number.parseFloat`; the two-line title, fixed 147px header, card containment, decision-surface separation, and viewport-locked document all passed.

---

## [ERR-20260719-029] browser-page-evaluate-method-location

**Logged**: 2026-07-19T12:44:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The final browser cleanup probe first called `evaluate` on the Chrome page wrapper instead of its Playwright capability.

### Error
```
chromeValidation.evaluate is not a function
```

### Context
- Operation: restore the real viewport and confirm final document geometry, drawer state, focus return, and console errors.
- The Chrome wrapper exposes page evaluation through `chromeValidation.playwright.evaluate(...)`.
- The viewport reset completed before the unsupported method call.

### Suggested Fix
Use the wrapper's `playwright` capability for DOM evaluation and keep `dev.logs(...)` on the wrapper's development capability.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionDashboard.svelte

### Resolution
- **Resolved**: 2026-07-19T12:44:00+08:00
- **Notes**: Re-ran with `chromeValidation.playwright.evaluate(...)`; the real 2133x906 document matched the viewport exactly, both drawers were closed, focus had returned to the table row, and the error console was empty.

---

## [ERR-20260719-030] chrome-role-row-transient-timeout

**Logged**: 2026-07-19T13:21:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first role-and-accessible-name row lookup timed out while reopening the authenticated requirement drawer.

### Error
```
Timed out after 3000ms waiting for selector internal:role=row[name=/ZPU-2769/]
```

### Context
- Operation: reopen the screenshot Jira row after reclaiming the existing authenticated Chrome tab.
- The table contained 134 rows and the target text was present; only the role lookup timed out during a short CDP evaluation window.

### Suggested Fix
For this virtualized table, locate `tbody tr` and filter by the unique Jira text before clicking.

### Metadata
- Reproducible: no
- Related Files: web/src/components/DecisionDashboard.svelte

### Resolution
- **Resolved**: 2026-07-19T13:22:00+08:00
- **Notes**: `locator('tbody tr').filter({ hasText: 'ZPU-2769' })` returned exactly one row and opened the drawer successfully.

---

## [ERR-20260719-031] chrome-locator-hover-unsupported

**Logged**: 2026-07-19T13:26:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The Chrome locator facade does not expose an upstream Playwright `hover()` method.

### Error
```
flatBodyLocator.hover is not a function
```

### Context
- Operation: position the pointer over the hidden-rail drawer body before short-viewport wheel validation.
- The drawer geometry had already provided a safe interior point for direct computer-action scrolling.

### Suggested Fix
Pass an explicit interior `x` and `y` point with `scrollX` and `scrollY` to the Chrome computer-action scroll call.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DecisionDashboard.svelte
- See Also: ERR-20260718-009, ERR-20260719-027

### Resolution
- **Resolved**: 2026-07-19T13:27:00+08:00
- **Notes**: Direct scrolling at the drawer-body coordinates reached the 113px maximum and made the action row visible without moving the document.

---
## [ERR-20260720-017] impeccable-target-scan-expanded-to-repository

**Logged**: 2026-07-20T10:45:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Appending `.` to an Impeccable target scan expanded the detector across the repository and returned unrelated legacy warnings.

### Error
```
The result included App.svelte, DemandKanban.svelte, detector fixtures, and output prototypes outside the two affected files.
```

### Context
- Operation: final UI anti-pattern detection for TaskKanban and FunctionalAdminShell.

### Suggested Fix
Pass only the exact affected paths for a scoped final scan. Use a separate repository-wide audit only when the task explicitly includes legacy cleanup.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/TaskKanban.svelte, web/src/components/prototype/FunctionalAdminShell.svelte

### Resolution
- **Resolved**: 2026-07-20T10:46:00+08:00
- **Notes**: Re-ran with only the two affected files. The remaining three warnings are pre-existing legacy sections outside this change; the dedicated layout scan is clean.

---

## [ERR-20260720-016] finesse-reference-path-was-one-level-deeper

**Logged**: 2026-07-20T10:43:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The first Finesse preflight read omitted the `references/` directory and failed to find the required files.

### Error
```
sed: /Users/eddie/.codex/skills/finesse-skill/anti-cheap.md: No such file or directory
sed: /Users/eddie/.codex/skills/finesse-skill/preflight.md: No such file or directory
```

### Context
- Operation: mandatory anti-cheap and preflight review before final delivery.

### Suggested Fix
Resolve referenced paths relative to the skill entrypoint and verify them with `rg --files` before reading.

### Metadata
- Reproducible: yes
- Related Files: /Users/eddie/.codex/skills/finesse-skill/SKILL.md

### Resolution
- **Resolved**: 2026-07-20T10:44:00+08:00
- **Notes**: Located and read both files from `finesse-skill/references/`.

---

## [ERR-20260720-015] copied-db-versioned-config-overrode-safe-startup-config

**Logged**: 2026-07-20T10:37:00+08:00
**Priority**: high
**Status**: resolved
**Area**: tests

### Summary
The validation server loaded the newest versioned configuration from the copied database and overrode the external safe config, briefly starting the Jira worker before the sandbox denied the port bind.

### Error
```
Loaded configuration from database version 19
Starting background Jira task synchronization worker...
```

### Context
- Operation: start an isolated authenticated UI validation server from `/tmp` with every external integration disabled.
- The database was a disposable `/tmp` copy, never the workspace database, and the process exited immediately when the local bind failed.

### Suggested Fix
When validating from a copied database, sanitize the newest `config_versions.config_json` in the copy as well as supplying an external config. Confirm the effective startup log before opening the browser.

### Metadata
- Reproducible: yes
- Related Files: internal/server/config_version_handlers.go, internal/server/server.go

### Resolution
- **Resolved**: 2026-07-20T10:38:00+08:00
- **Notes**: Replaced only the copied database's latest configuration with an integration-disabled local profile. The workspace database and live business data were not modified.

---

## [ERR-20260720-014] loopback-bind-and-approval-channel-unavailable

**Logged**: 2026-07-20T10:37:00+08:00
**Priority**: medium
**Status**: pending
**Area**: tests

### Summary
Both backend and Vite loopback binds were denied inside the sandbox, and the scoped escalation request could not be reviewed because the approval stream disconnected.

### Error
```
listen tcp 127.0.0.1:8080: bind: operation not permitted
listen EPERM: operation not permitted 127.0.0.1:5173
Automatic approval review failed: stream disconnected before completion
```

### Context
- Operation: run authenticated browser validation against an isolated `/tmp` database copy with all external integrations disabled.
- Static checks and production build can continue, but live breakpoint, computed-style, and interaction evidence cannot be produced without an authorized local listener.

### Suggested Fix
Retry only after explicit user approval or a restored approval channel; do not switch to the workspace database or another unsafe runtime path.

### Metadata
- Reproducible: environment-dependent
- Related Files: web/vite.config.ts, tmp/ui-validation-config.yaml

---

## [ERR-20260720-013] go-build-default-cache-denied

**Logged**: 2026-07-20T10:36:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The first isolated validation-server build attempted to use the macOS user Go cache, which is outside the writable sandbox.

### Error
```
open /Users/eddie/Library/Caches/go-build/...: operation not permitted
```

### Context
- Operation: compile the current backend to `/tmp` so validation would not run a stale checked-in binary.

### Suggested Fix
Set `GOCACHE=/tmp/well-ambient-gocache` for local validation builds in this workspace.

### Metadata
- Reproducible: yes
- Related Files: cmd/server/main.go
- Recurrence-Count: 2
- Last-Seen: 2026-07-31T23:19:00+08:00

### Resolution
- **Resolved**: 2026-07-20T10:36:30+08:00
- **Notes**: Rebuilt successfully with the Go cache redirected to `/tmp`.

---

## [ERR-20260720-012] broad-config-read-exposed-checked-in-secrets

**Logged**: 2026-07-20T10:43:00+08:00
**Priority**: high
**Status**: resolved
**Area**: tooling

### Summary
A broad configuration preview returned checked-in integration credentials that were irrelevant to the UI validation task.

### Error
```
The requested line range included credential fields from config.example.yaml and config_test.yaml.
```

### Context
- Operation: inspect only the local server port and integration enablement before authenticated browser validation.
- The command requested complete configuration sections instead of narrowly matching safe structural fields.

### Suggested Fix
Never dump project configuration files when only host, port, or boolean feature state is needed. Use a narrow parser or exact safe-field match that excludes tokens, secrets, usernames, and URLs carrying credentials.

### Metadata
- Reproducible: yes
- Related Files: config.example.yaml, config_test.yaml
- Recurrence-Count: 2
- Last-Seen: 2026-07-31T23:18:00+08:00

### Resolution
- **Resolved**: 2026-07-20T10:44:00+08:00
- **Notes**: Stopped reading the supplied configs and created a separate minimal validation config with every external integration disabled.

---

## [ERR-20260720-011] sandbox-process-list-denied

**Logged**: 2026-07-20T10:41:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The sandbox denied a broad process-list command used to locate an existing local preview server.

### Error
```
zsh: operation not permitted: ps
```

### Context
- Operation: determine whether the authenticated development server and frontend preview were still running.
- The later fixed-port HTTP probes established that no local service was listening.

### Suggested Fix
Probe the repository's configured local ports directly instead of enumerating all host processes.

### Metadata
- Reproducible: yes
- Related Files: web/vite.config.ts

### Resolution
- **Resolved**: 2026-07-20T10:42:00+08:00
- **Notes**: Replaced process enumeration with bounded HTTP checks on the expected backend and frontend ports.

---
## [ERR-20260720-018] project-preference-snapshot-scope-not-forwarded

**Logged**: 2026-07-20T16:20:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
The first focused Go test build failed because the new project scope reached the strongest-brain snapshot loader but was not forwarded into the read-model builder.

### Error
```
internal/server/strongest_brain_handlers.go:592:61: undefined: projectKeys
```

### Context
- Operation: run focused project-preference tests after adding server-side scope filtering.
- `buildStrongestBrainDecisionSnapshot` accepted `projectKeys`, while `buildStrongestBrainDecisionReadModel` still had its original signature and referenced a name outside its scope.

### Suggested Fix
Thread the normalized scope through every layer that adds task-derived records, including weak-semantic evidence decisions, and retain variadic compatibility for existing direct unit-test callers.

### Metadata
- Reproducible: yes
- Related Files: internal/server/strongest_brain_handlers.go

### Resolution
- **Resolved**: 2026-07-20T16:23:00+08:00
- **Notes**: Forwarded the normalized scope through the variadic read-model builder and reran the focused db/server/agenda tests successfully.

---

## [ERR-20260720-019] browser-validation-guessed-daily-jira-heading

**Logged**: 2026-07-20T14:48:00+08:00
**Priority**: low
**Status**: resolved
**Area**: browser-validation

### Summary
The authenticated browser check clicked the correct Daily Jira navigation item but waited for a guessed heading that the page does not render.

### Error
```text
Playwright selector deadline exceeded while waiting for "每日 Jira 风险审计"
```

### Context
- The navigation state had already changed successfully.
- The wait target came from an assumption instead of the post-click DOM snapshot.

### Resolution
Captured a fresh DOM snapshot, anchored validation to the actual `7 日及以上 Jira 列表` structure, and verified that all 19 visible rows belonged only to HIT or NS2.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DailyJiraAudit.svelte

---

## [ERR-20260731-022] shell-expanded-static-contract-pattern

**Logged**: 2026-07-31T16:52:43+08:00
**Priority**: low
**Status**: resolved
**Area**: validation

### Summary
A Node static-contract check embedded JavaScript template literals in a double-quoted shell command, so zsh interpreted the backticks and `?` pattern before Node received the script.

### Error
```text
zsh:1: bad pattern: /api/work-items?search=
```

### Resolution
Rewrote the validation command with a single-quoted JavaScript program and double-quoted JavaScript strings. The same four contracts then passed.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/prototype/FunctionalAdminShell.svelte, web/src/components/TaskKanban.svelte

---

## [ERR-20260731-023] broad-example-config-inspection-exposed-secret-shaped-values

**Logged**: 2026-07-31T16:55:30+08:00
**Priority**: high
**Status**: resolved
**Area**: credential-hygiene

### Summary
A broad inspection of `config.example.yaml` returned credential-shaped example values that were not needed for the UI validation.

### Resolution
Stopped reading configuration contents and switched browser validation to a local fixture-only API. Future startup discovery should inspect config schemas or targeted non-secret keys, never whole configuration files.

### Metadata
- Reproducible: yes
- Related Files: config.example.yaml

---

## [ERR-20260731-024] local-validation-command-recovery

**Logged**: 2026-07-31T17:04:12+08:00
**Priority**: low
**Status**: resolved
**Area**: validation

### Summary
Three local-only validation steps needed correction: the sandbox denied the first Vite bind, one search command had an unmatched shell quote, and one broad multi-hunk patch no longer matched the edited date section.

### Resolution
Reran only the local preview and fixture services with scoped approval, replaced the malformed search with a literal-safe command, and split the date-format edit into small context-anchored patches. No external integration or business write was enabled.

### Metadata
- Reproducible: environment-dependent
- Related Files: web/src/components/TaskKanban.svelte, web/src/components/prototype/FunctionalAdminShell.svelte
- Recurrence-Count: 3
- Last-Seen: 2026-07-31T20:08:00+08:00

---

## [ERR-20260731-025] core-member-browser-harness-recovery

**Logged**: 2026-07-31T17:24:00+08:00
**Priority**: low
**Status**: resolved
**Area**: validation

### Summary
The first combined patch used stale task-plan context, browser validation initially guessed unsupported viewport and finalization call shapes, and the full server suite hit a sandbox-denied `httptest` listener.

### Error
```text
apply_patch verification failed
setViewportSize is not a function
browser.tabs.finalize expects an options object
panic: httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
```

### Resolution
Split the patch into file-local hunks anchored to current content, kept the data-boundary validation at the unaffected desktop breakpoint, finalized the browser with `{ tabIds: [...] }`, and reran the server suite with scoped approval so its existing temporary-listener test could execute. The local read-only fixture and preview were stopped after validation.

### Metadata
- Reproducible: yes
- Related Files: task_plan.md, internal/server/server_test.go, web/src/components/TaskKanban.svelte

---

## [ERR-20260731-026] execution-tracking-validation-api-recovery

**Logged**: 2026-07-31T18:04:00+08:00
**Priority**: low
**Status**: resolved
**Area**: validation

### Summary
The execution-tracking validation initially used unsupported browser viewport and geometry calls, a broad patch missed current context, and the sandbox denied an existing server test listener.

### Error
```text
setViewportSize is not a function
boundingBox is not a function
apply_patch verification failed
httptest: failed to listen on a port: operation not permitted
```

### Context
- Breakpoint validation was required for the trajectory modal.
- The browser runtime exposes viewport overrides through the browser capability, not the tab Playwright subset.
- The server suite contains an existing `httptest` listener that needs scoped execution outside the filesystem sandbox.

### Suggested Fix
Read the viewport capability documentation before responsive checks, use screenshots plus supported locator reads for geometry, split patches into current file-local hunks, and rerun listener-dependent tests with the narrowest approved command.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/TaskKanban.svelte, web/src/components/CommitTelemetryPanel.svelte, internal/server/execution_task_catalog_test.go
- See Also: ERR-20260731-025

### Resolution
- **Resolved**: 2026-07-31T18:04:00+08:00
- **Notes**: Used the browser viewport capability for 760/390 validation, verified the modal visually, split edits into scoped patches, and completed the focused server suite with scoped approval.

---

## [ERR-20260731-027] full-server-suite-sandbox-listener

**Logged**: 2026-07-31T19:40:30+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The focused execution and task-directory tests passed, while the complete server suite again reached an unrelated `httptest` case that cannot bind an IPv6 loopback listener inside the sandbox.

### Error
```text
panic: httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
```

### Context
- Command: `GOCACHE=/tmp/well-ambient-go-cache go test ./internal/server ./internal/db -count=1`
- The failure occurs in `TestEnsureGitLabWebhooksUpdatesExistingHook`, after the changed execution/task-directory tests have passed.

### Suggested Fix
Rerun the same bounded Go test command with local loopback-listener permission; keep the focused no-listener regression suite as the sandbox-safe validation loop.

### Metadata
- Reproducible: yes
- Related Files: internal/server/config_handlers_gitlab_webhook_test.go
- See Also: ERR-20260731-025, ERR-20260731-026

### Resolution
- **Resolved**: 2026-07-31T19:42:00+08:00
- **Notes**: Reran the bounded suite with local loopback-listener permission. Updated the pre-existing execution summary test to the new evidence-backed WorkItem projection contract; the complete server and database suites then passed.

---

## [ERR-20260731-028] demand-directory-compatibility-patch-syntax

**Logged**: 2026-07-31T23:08:00+08:00
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary
A deletion hunk that replaced demand-option assignee collection left two stale closing braces, so gofmt stopped before the focused directory tests could run.

### Error
```text
internal/server/demand_handlers.go:61:2: expected declaration, found '}'
```

### Suggested Fix
Inspect the complete edited function boundary immediately after a multi-block deletion, then run gofmt before combining it with tests.

### Metadata
- Reproducible: yes
- Related Files: internal/server/demand_handlers.go
- See Also: ERR-20260731-024

### Resolution
- **Resolved**: 2026-07-31T23:09:00+08:00
- **Notes**: Removed the two stale braces and reran formatting plus the same focused regression command.

---

## [ERR-20260731-029] nullable-project-config-payload

**Logged**: 2026-07-31T23:22:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: frontend

### Summary
The empty project-config endpoint returned JSON `null`, which replaced the schedule page's array state and broke the next reactive `.filter()`.

### Error
```text
TypeError: Cannot read properties of null (reading 'filter')
```

### Context
- Authenticated isolated browser validation navigated from the schedule surface to Version Plan with an empty project-config table.
- The authoritative delivery project directory remained populated, but the unrelated editable project-config request assigned `null` directly to `projectConfigs`.

### Suggested Fix
Normalize list-shaped API payloads at the client boundary with `Array.isArray(payload) ? payload : []` before assigning reactive array state.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DemandKanban.svelte

### Resolution
- **Resolved**: 2026-07-31T23:28:00+08:00
- **Notes**: Added array normalization and repeated frontend check/build plus clean authenticated route validation.

---

## [ERR-20260812-001] teams-v2-module-path

**Logged**: 2026-08-12T12:05:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary
The first scorecard-skill read used root-level module paths even though `SKILL.md` routes them through `modules/`.

### Error
```text
sed: /Users/eddie/.codex/skills/teams-v2/core-lite.md: No such file or directory
```

### Context
- Attempted to load the teams-v2 cold-start files before mapping the executable performance workbook to the current system.
- The entrypoint itself was read successfully and showed the correct relative locations.

### Suggested Fix
Resolve every referenced path relative to the directory containing `SKILL.md`; use `modules/core-lite.md`, `modules/route-presets.md`, and `modules/scorecard-lite.md`.

### Metadata
- Reproducible: yes
- Related Files: /Users/eddie/.codex/skills/teams-v2/SKILL.md

### Resolution
- **Resolved**: 2026-08-12T12:06:00+08:00
- **Notes**: Reloaded the cold-start files from their documented `modules/` locations and continued the repository-backed audit.

---

## [ERR-20260812-002] performance-full-go-test

**Logged**: 2026-08-12T18:02:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tests

### Summary
The first full Go run mixed sandbox-blocked `httptest` listeners with a new SQLite polling test that could observe a transient table lock.

### Error
```text
httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
database table is locked: performance_score_runs
```

### Context
- Command: `GOCACHE=/tmp/well-ambient-go-build go test ./... -count=1`
- The listener failures come from existing LLM/server tests under the restricted sandbox.
- The SQLite lock came from the new background-runner test polling while its worker transaction was writing.

### Suggested Fix
Serialize the in-memory SQLite test connection, give every repeated test invocation a unique shared-cache DSN, rerun the new package repeatedly, then rerun the full suite with the existing local-listener permission needed by repository tests.

### Metadata
- Reproducible: yes
- Related Files: internal/performance/module_test.go, internal/llm/client_test.go, internal/server/config_handlers_gitlab_webhook_test.go

### Resolution
- **Resolved**: 2026-08-12T18:05:00+08:00
- **Notes**: Serialized each in-memory SQLite test database, added a unique DSN per repeated invocation, passed the performance package ten consecutive times, and reran the full suite with local-listener permission.

---

## [ERR-20260812-006] performance-cleanup-port-check

**Logged**: 2026-08-12T18:43:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The first isolated-runtime cleanup check passed two TCP inclusion filters with the same `LISTEN` state to one `lsof` invocation, which this `lsof` version rejects.

### Error
```text
lsof: duplicate TCP inclusion: LISTEN
```

### Context
- The performance backend and Vite sessions had already received Ctrl-C.
- This was a read-only verification command and did not affect implementation or runtime data.

### Suggested Fix
Check each explicit port in a separate `lsof -iTCP:<port> -sTCP:LISTEN` invocation.

### Metadata
- Reproducible: yes
- Related Files: none

### Resolution
- **Resolved**: 2026-08-12T18:43:00+08:00
- **Notes**: Rechecked ports 18083 and 4178 independently; neither had a listener.

---

## [ERR-20260812-007] solution-catalog-sqlite-datetime-aggregate

**Logged**: 2026-08-12T23:56:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
SQLite returns `MAX(datetime-column)` as a string, so scanning the catalog project aggregate directly into `time.Time` failed.

### Error
```text
sql: Scan error on column index 2, name "last_synced": unsupported Scan, storing driver.Value type string into type *time.Time
```

### Context
- Command: focused solution catalog and handler Go tests.
- The failure was isolated to `ListProjects`; catalog projection, search scope, detail scope, comparison, and standardization tests had passed.

### Suggested Fix
Aggregate `unixepoch(synced_at)` into an integer and convert it explicitly with `time.Unix` at the module boundary.

### Metadata
- Reproducible: yes
- Related Files: internal/solutioncatalog/query.go, internal/server/solution_catalog_handlers_test.go

### Resolution
- **Resolved**: 2026-08-12T23:57:00+08:00
- **Notes**: Replaced the driver-dependent datetime aggregate scan with Unix seconds plus explicit UTC conversion and retained the handler regression.

---
## [ERR-20260813-001] performance-runtime-process-inspection

**Logged**: 2026-08-13T12:25:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The restricted shell blocked `ps` and loopback `curl` while diagnosing the live performance runner.

### Error
```text
zsh: operation not permitted: ps
curl: (7) Failed to connect to 127.0.0.1 port 8080
```

### Context
- Read-only inspection of the already-running local development process and `/health` endpoint.
- `lsof` independently confirmed PID 9978 was listening on port 8080.

### Suggested Fix
Repeat the same read-only process and loopback probes with the managed local-runtime permission.

### Metadata
- Reproducible: yes
- Related Files: cmd/server/main.go, internal/server/server.go

### Resolution
- **Resolved**: 2026-08-13T12:25:00+08:00
- **Notes**: Continued with an explicitly approved read-only local-runtime probe.

---

## [ERR-20260813-002] zsh-empty-vite-glob

**Logged**: 2026-08-13T12:25:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
An optional Vite-config search used a shell glob that had no matches under zsh.

### Error
```text
zsh: no matches found: vite.config.*
```

### Context
- Read-only search for the frontend proxy target.
- No product command or file mutation failed.

### Suggested Fix
Resolve candidates with `rg --files` first and pass only existing paths to `rg`.

### Metadata
- Reproducible: yes
- Related Files: web/package.json

### Resolution
- **Resolved**: 2026-08-13T12:25:00+08:00
- **Notes**: Replaced the glob with an `rg --files` candidate lookup.

---

## [ERR-20260813-003] overbroad-config-example-read

**Logged**: 2026-08-13T15:05:00+08:00
**Priority**: high
**Status**: resolved
**Area**: tooling

### Summary
A configuration inspection printed unrelated sensitive integration fields because the requested line range extended beyond the performance block.

### Error
```text
The read-only command used a broad line range instead of selecting only performance_brain keys.
```

### Context
- The task only required v4.0 performance configuration defaults.
- No credential was modified, copied into source, memory, or the response.

### Suggested Fix
For configuration containing possible credentials, use `rg` with exact safe key names or a structured parser that emits an allowlist; never print a broad line range.

### Metadata
- Reproducible: yes
- Related Files: config.yaml, config.example.yaml

### Resolution
- **Resolved**: 2026-08-13T15:05:00+08:00
- **Notes**: Stopped broad configuration reads and restricted subsequent checks to the performance key allowlist.

---

## [ERR-20260813-004] unavailable-tsx-test-runner

**Logged**: 2026-08-13T15:32:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The focused frontend contract test was first invoked through `pnpm exec tsx`, but this workspace does not install `tsx`.

### Error
```text
Command "tsx" not found
```

### Context
- The target is a Node test file that only uses syntax supported by the repository's current Node runtime.
- The failed runner prevented the chained Svelte check from starting.

### Suggested Fix
Use Node 22's explicit TypeScript stripping flag with the native test runner, and run type/Svelte checks as separate commands so one runner error cannot hide later validation.

### Metadata
- Reproducible: yes
- Related Files: web/tests/performance-governance-contract.test.ts, web/package.json

### Resolution
- **Resolved**: 2026-08-13T15:32:00+08:00
- **Notes**: Verified `node --experimental-strip-types --test tests/performance-governance-contract.test.ts`; all three focused contracts pass. Plain `node --test` is insufficient for `.ts` in this runtime.

---

## [ERR-20260813-005] sandbox-httptest-loopback

**Logged**: 2026-08-13T14:56:00+08:00
**Priority**: low
**Status**: resolved
**Area**: testing

### Summary
The combined backend regression was first run in the restricted sandbox, where an unrelated server test could not bind an ephemeral localhost port.

### Error
```text
httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
```

### Context
- `internal/performance` and `internal/config` had already passed in the same invocation.
- The failure occurred in `TestEnsureGitLabWebhooksUpdatesExistingHook`, before the server suite could complete.

### Suggested Fix
When a repository server suite contains `httptest.NewServer`, rerun the exact test command with narrowly approved local-loopback permission after the sandbox failure.

### Metadata
- Reproducible: yes
- Related Files: internal/server/config_handlers_gitlab_webhook_test.go

### Resolution
- **Resolved**: 2026-08-13T14:57:00+08:00
- **Notes**: Reran the unchanged command with local-loopback permission; performance, config, and server suites all passed.

---

## [ERR-20260813-006] browser-validation-api-assumptions

**Logged**: 2026-08-13T15:18:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
Browser validation initially assumed a unique historical snapshot, a scriptable `window.resizeTo`, and an array-shaped tab-finalization argument.

### Error
```text
strict mode violation: getByRole('button', { name: '查看 刘子翔 的评分详情' }) resolved to 2 elements
TypeError: window.resizeTo is not a function
browser.tabs.finalize expects an options object
```

### Context
- The persisted snapshot table intentionally shows more than one run, so member labels repeat.
- The in-app browser viewport is fixed and does not expose `window.resizeTo`.
- Browser cleanup uses `tabs.finalize({})`.

### Suggested Fix
Scope member detail checks to the first/latest row, measure the actual browser viewport, verify narrow-screen rules statically when viewport emulation is unavailable, and finalize with an options object.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/PerformanceCalculationGuide.svelte, web/src/components/shared/Modal.svelte

### Resolution
- **Resolved**: 2026-08-13T15:18:00+08:00
- **Notes**: Validated the latest snapshots at the available 1280x720 viewport, checked the shared mobile rules, and finalized both in-app and claimed Chrome tabs.

---

## [ERR-20260813-007] performance-run-query-schema-assumption

**Logged**: 2026-08-13T15:19:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The first read-only audit query assumed a `started_at` column that the run table does not contain.

### Error
```text
no such column: started_at
```

### Context
The persisted run table uses `created_at` for start time and `completed_at` for completion time.

### Suggested Fix
Inspect `.schema` before composing an ad hoc SQLite evidence query.

### Metadata
- Reproducible: yes
- Related Files: internal/performance/models.go

### Resolution
- **Resolved**: 2026-08-13T15:19:00+08:00
- **Notes**: Re-ran the evidence query with `created_at`; the latest v4.1 run, 14 snapshots, and all audit event counts were confirmed.

---

## [ERR-20260813-008] unrelated-svelte-check-errors

**Logged**: 2026-08-13T15:20:00+08:00
**Priority**: medium
**Status**: unresolved
**Area**: frontend

### Summary
The repository-wide Svelte check remains blocked by pre-existing nullable-value errors outside the personnel-performance surface.

### Error
```text
SolutionWorkspace.svelte: 9 errors and repository-wide existing warnings
```

### Context
- The affected performance components build successfully.
- The focused performance governance contract passes.
- The unrelated dirty file was not changed as part of this fix.

### Suggested Fix
Repair the `SolutionWorkspace.svelte` nullable-value errors in a separately scoped task, then rerun the repository-wide checker.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/SolutionWorkspace.svelte

---

## [ERR-20260813-009] conditional-named-slot-placement

**Logged**: 2026-08-13T23:55:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: frontend

### Summary
Svelte rejected the solution editor footer because the named slot element was nested inside an `{#if}` block instead of being a direct child of `Modal`.

### Error
```text
Element with a slot='...' attribute must be a child of a component or a descendant of a custom element
SolutionWorkspace.svelte:363:12
```

### Context
- Command: `pnpm check`
- The editor Modal conditionally rendered both its body and `slot="footer"` under `workspace?.working`.
- Existing repository warnings are unrelated; this new error is local to the edited component.

### Suggested Fix
Keep the named footer slot as a direct `Modal` child and move the workspace condition inside the slotted element.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/SolutionWorkspace.svelte

### Resolution
- **Resolved**: 2026-08-14T00:02:00+08:00
- **Notes**: Moved the footer slot to be a direct `Modal` child and nested the workspace condition inside it; `pnpm check` returned zero errors.

---

## [ERR-20260813-010] empty-temporary-go-module-cache

**Logged**: 2026-08-13T23:54:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The isolated browser-validation server build pointed both Go caches at new empty temporary directories, forcing a dependency download that the restricted network could not complete.

### Error
```text
dial tcp: lookup goproxy.cn: no such host
```

### Context
- The temporary build only needed an isolated build cache; the machine already had the required module versions in its standard module cache.
- No application source or dependency version was at fault.

### Suggested Fix
Keep `GOCACHE` isolated for generated build artifacts, but reuse the existing read-only `GOMODCACHE` when all pinned modules are already present.

### Metadata
- Reproducible: yes
- Related Files: go.mod

### Resolution
- **Resolved**: 2026-08-13T23:55:00+08:00
- **Notes**: Rebuilt with the temporary `GOCACHE` and the existing module cache; the validation server compiled successfully without network access.

---
## [ERR-20260814-006] chrome-tabs-claim-api-mismatch

**Logged**: 2026-08-14T17:10:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tooling

### Summary
The browser validation attempted an unsupported `chrome.tabs.claim` method when reconnecting to an existing user tab.

### Error
```text
chrome.tabs.claim is not a function
chrome.tabs.open is not a function
```

### Context
- The Chrome binding and logged-in localhost tab were visible, but the current browser-client API does not expose `tabs.claim`.
- No page navigation, input, save, publish, or business mutation occurred.

### Suggested Fix
Use the documented supported tab-opening or user-tab connection path for this browser-client version; do not retry `tabs.claim`.

### Metadata
- Reproducible: yes
- Related Files: none

### Resolution
- **Resolved**: 2026-08-14T17:16:00+08:00
- **Notes**: Read the current browser-client API, named the session, passed the exact fresh `openTabs()` object to `chrome.user.claimTab(...)`, and connected successfully. Current methods are `browser.user.claimTab(...)` and `browser.tabs.new()`.

---

## [ERR-20260825-001] browser-evaluate-bare-history-global

**Logged**: 2026-08-25T00:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The browser page-evaluation sandbox did not expose the bare `history` global while preparing a query-only fixture mode switch.

### Error
```text
TypeError: Cannot read properties of undefined (reading 'replaceState')
```

### Context
- The evaluation attempted `history.replaceState(...)` before any product button was clicked.
- The authenticated page, completed AI result, and backend state were unchanged.

### Suggested Fix
Do not depend on history mutation, proxy property assignment, or root-element attribute methods in this page sandbox. Drive temporary fixture modes through an actual visible form input and the normal UI interaction path.

### Metadata
- Reproducible: yes
- Related Files: web/src/lib/deconstruct-retention-fixture.ts

### Resolution
- **Resolved**: 2026-08-25T00:00:00+08:00
- **Notes**: Bare `history`, `window.history`, direct dataset assignment, and root `setAttribute` were unavailable. Switched the temporary slow-stream mode to a visible input marker consumed only by the development fixture, then continued without navigation or business writes.

---

## [ERR-20260825-002] browser-locator-after-parent-hmr

**Logged**: 2026-08-25T00:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The browser attempted to click the AI generate button after a parent-component HMR had already closed the modal.

### Error
```text
Playwright selector deadline exceeded: no matches for button "生成解构"
```

### Context
- A temporary fixture and parent callback were edited during browser validation.
- A fresh DOM snapshot showed the schedule page healthy and the modal closed; no hidden node or business action was invoked.

### Suggested Fix
After any parent-component HMR, take a fresh DOM snapshot and reopen the target state before reusing locators.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/DemandKanban.svelte

### Resolution
- **Resolved**: 2026-08-25T00:00:00+08:00
- **Notes**: Discarded the stale locator plan, confirmed the visible schedule page, and rebuilt the fixture state through normal UI actions.

---

## [ERR-20260825-003] browser-confirm-click-without-state-guard

**Logged**: 2026-08-25T00:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The validation captured that a close confirmation was absent but still attempted to click its confirm button in the same script.

### Error
```text
Playwright selector deadline exceeded: no matches for button "确认停止并关闭"
```

### Context
- Wrapper `fill()` had not propagated the slow-fixture marker through the Svelte input binding, so the run completed immediately and idle close removed the modal.
- No confirmation existed and no real AI request or business write occurred.

### Suggested Fix
Verify the rendered running state before clicking confirmation. For a two-run lifecycle fixture, use a deterministic module-local call sequence instead of depending on UI text propagation to choose the second response mode.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/Deconstructor.svelte

### Resolution
- **Resolved**: 2026-08-25T00:00:00+08:00
- **Notes**: Split the flow into guarded stages. Keyboard input read back correctly in the DOM but did not deterministically select the response branch, so the final fixture uses first-call complete / second-call slow sequencing.

---

## [ERR-20260825-004] browser-task-view-combined-click-snapshot-timeout

**Logged**: 2026-08-25T12:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The authenticated task-page baseline combined a navigation click, wait, and large DOM snapshot in one browser call and hit the short CDP evaluation timeout.

### Error
```text
Timed out after 3000ms waiting for CDP command Runtime.evaluate.
```

### Context
- Operation: switch from execution tracking to the task table and immediately serialize an 8.5k DOM snapshot.
- The preceding page was healthy and the click did not submit or mutate business data.

### Suggested Fix
Split navigation and state inspection into separate calls, then measure only the owning containers needed for geometry validation.

### Metadata
- Reproducible: no
- Related Files: web/src/components/TaskKanban.svelte
- See Also: ERR-20260719-030

### Resolution
- **Resolved**: 2026-08-25T12:21:00+08:00
- **Notes**: Continued with separate click, compact snapshot, and targeted geometry reads.

---

## [ERR-20260825-005] stale-settings-component-path

**Logged**: 2026-08-25T17:12:35+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
The configuration-inspector revision initially targeted a stale nested component path instead of the repository's actual SettingsPanel location.

### Error
```text
rg: web/src/lib/components/admin/SettingsPanel.svelte: No such file or directory
```

### Context
- The attempted path came from an outdated component-layout assumption.
- The current repository indexes the owner as `web/src/components/SettingsPanel.svelte`.

### Suggested Fix
Resolve target files with `rg --files` before using remembered component paths in a dirty, evolving workspace.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/SettingsPanel.svelte

### Resolution
- **Resolved**: 2026-08-25T17:12:35+08:00
- **Notes**: Re-indexed the workspace and continued against the actual SettingsPanel owner.

---

## [ERR-20260825-006] stale-chrome-tab-handle

**Logged**: 2026-08-25T17:50:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The preserved Chrome tab handle no longer referred to a live tab when responsive validation resumed.

### Error
```text
No tab with id: 733588617.
```

### Context
- The previous authenticated tab had closed between validation phases.
- The Chrome session reported no claimable live tabs; a fresh tab opened at the login page because the prior login had expired.

### Suggested Fix
List live Chrome tabs before reusing a persisted handle, then use the repository's read-only preview when authentication is unavailable.

### Metadata
- Reproducible: no
- Related Files: web/settings-preview.html, web/src/settings-preview-entry.ts

### Resolution
- **Resolved**: 2026-08-25T17:53:00+08:00
- **Notes**: Opened the checked-in read-only Settings preview, completed desktop/tablet/phone validation without business writes, reset the viewport, and closed the validation tab.

---

## [ERR-20260825-007] node-repl-metric-expression-parenthesis

**Logged**: 2026-08-25T17:54:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Two dense one-line browser metric expressions contained an extra closing parenthesis.

### Error
```text
Unexpected token: ')'
```

### Context
- Both expressions combined navigation, viewport changes, evaluation, and output serialization in one line.
- No page state or business data was mutated.

### Suggested Fix
Assign browser evaluation results to a named variable, then emit that variable in a separate statement.

### Metadata
- Reproducible: yes
- Related Files: web/src/components/SettingsPanel.svelte
- See Also: ERR-20260825-004

### Resolution
- **Resolved**: 2026-08-25T17:55:00+08:00
- **Notes**: Split the expressions into named metric assignments; all three responsive measurements completed.

---
## [ERR-20260827-001] docs-write-shared-style-guide-missing

**Logged**: 2026-08-27T00:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary

`docs-write` 引用的共享风格指南未安装在声明的相对路径。

### Error

```text
sed: /Users/eddie/.agents/skills/_shared/metabase-style-guide.md: No such file or directory
```

### Context

- 读取 `/Users/eddie/.agents/skills/docs-write/SKILL.md` 后按其相对引用加载共享文件。
- 技能正文已包含完成本任务所需的核心写作规则，因此实现不受阻。

### Suggested Fix

补齐技能包中的 `_shared/metabase-style-guide.md`，或移除失效引用并将必要规则保留在 `SKILL.md`。

### Metadata

- Reproducible: yes
- Related Files: /Users/eddie/.agents/skills/docs-write/SKILL.md

### Resolution

- **Resolved**: 2026-08-27T00:00:00+08:00
- **Notes**: 不再重复读取缺失路径，按已加载的技能正文继续。

---

## [ERR-20260827-002] setup-token-main-patch-context-drift

**Logged**: 2026-08-27T00:05:00+08:00
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary

令牌模块、校验和启动入口的组合补丁因 `cmd/server/main.go` 现有文案与快照不同而校验失败。

### Error

```text
apply_patch verification failed: Failed to find expected lines in cmd/server/main.go
```

### Context

- 工作树已有同一区域的用户/任务改动。
- `apply_patch` 原子失败，确认 `setup_token.go` 未创建，其他文件也未被部分修改。

### Suggested Fix

重新读取目标小范围，按单文件、精确当前上下文拆分补丁。

### Metadata

- Reproducible: no
- Related Files: cmd/server/main.go, internal/server/setup_server.go

### Resolution

- **Resolved**: 2026-08-27T00:06:00+08:00
- **Notes**: 已采用小块补丁，保留现有启动文案和工作树改动。

---

## [ERR-20260827-003] setup-token-deployment-patch-context-drift

**Logged**: 2026-08-27T00:15:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary

部署、示例和文档组合补丁两次因中文现有文案与快照不同而原子失败。

### Error

```text
apply_patch verification failed: Failed to find expected lines in deploy/.env.production.example
apply_patch verification failed: Failed to find expected lines in docs/deployment-linux-postgres.md
```

### Context

- 代码、Compose、部署脚本和文档都在同一组合补丁中，任一文案漂移会阻止全部文件更新。
- 两次失败均为原子失败，没有半应用。

### Suggested Fix

对脏工作树中的多文件变更按文件拆分；文档优先锚定稳定代码块或标题，不依赖整句自然语言。

### Metadata

- Reproducible: no
- Related Files: deploy/.env.production.example, docs/deployment-linux-postgres.md

### Resolution

- **Resolved**: 2026-08-27T00:18:00+08:00
- **Notes**: 已拆成单文件补丁，并在稳定部署命令代码块后新增独立令牌小节。

---

## [ERR-20260827-004] prettier-not-installed

**Logged**: 2026-08-27T00:25:00+08:00
**Priority**: low
**Status**: resolved
**Area**: docs

### Summary

`docs-write` 要求的 Prettier 不在当前项目依赖中。

### Error

```text
ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL Command "prettier" not found
```

### Context

- 尝试格式化新增 TypeScript 合同和 Markdown 部署说明。
- 命令在任何后续测试执行前退出，没有产生文件改动。

### Suggested Fix

若项目决定统一采用 Prettier，应将其加入开发依赖并提供仓库级格式化脚本；本任务不临时下载依赖。

### Metadata

- Reproducible: yes
- Related Files: web/package.json, docs/deployment-linux-postgres.md

### Resolution

- **Resolved**: 2026-08-27T00:26:00+08:00
- **Notes**: 改用仓库现有 TypeScript/Svelte 检查、Node 合同测试和 Markdown 人工结构检查。

---

## [ERR-20260827-005] docker-cli-not-installed

**Logged**: 2026-08-27T00:30:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: infra

### Summary

本地环境没有 Docker CLI，无法执行 `docker compose config --quiet`。

### Error

```text
zsh: command not found: docker
```

### Context

- Compose 校验与 Go/Node 测试使用 `&&` 串联，因此命令在测试前停止。
- 没有启动或修改任何容器。

### Suggested Fix

在无 Docker 环境中先用本地 YAML 解析器和部署合同验证结构，并把真实 Compose 启动保留为带 Docker 的交付环境门禁。

### Metadata

- Reproducible: yes
- Related Files: compose.yaml

### Resolution

- **Resolved**: 2026-08-27T00:31:00+08:00
- **Notes**: 改用 Ruby YAML 解析，Go 与 Node 测试拆分执行。

---

## [ERR-20260827-006] ruby-psych-aliases-keyword-unsupported

**Logged**: 2026-08-27T00:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: infra

### Summary

系统 Ruby 2.6 的 Psych 不支持 `YAML.load_file(..., aliases: true)` 关键字参数。

### Error

```text
unknown keyword: aliases (ArgumentError)
```

### Context

- 这是无 Docker 环境下的 Compose YAML 替代语法检查。
- 同批 bash、Go 和 Node 校验均通过。

### Suggested Fix

Ruby 2.6 使用 `YAML.load(File.read(path))` 的兼容调用；不要重复较新 Psych API。

### Metadata

- Reproducible: yes
- Related Files: compose.yaml

### Resolution

- **Resolved**: 2026-08-27T00:36:00+08:00
- **Notes**: 改用 Ruby 2.6 兼容 API。

---

## [ERR-20260827-007] full-go-tests-sandbox-loopback-denied

**Logged**: 2026-08-27T12:23:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

全量 Go 测试中的既有 `httptest.NewServer` 无法在沙箱内监听 IPv6 loopback。

### Error

```text
httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted
```

### Context

- `internal/llm` 和 `internal/server` 的既有 HTTP 测试触发；本任务定向令牌测试已通过。
- 同批 `go vet ./...`、前端检查和生产构建通过。

### Suggested Fix

按审批流程仅为全量测试开放本地 loopback，使用相同命令重跑，不修改测试逻辑。

### Metadata

- Reproducible: yes
- Related Files: internal/llm/client_test.go, internal/server/config_handlers_gitlab_webhook_test.go

### Resolution

- **Resolved**: 2026-08-27T12:28:00+08:00
- **Notes**: 在获批的本地 loopback 权限下原命令全量通过。

---

## [ERR-20260827-008] setup-token-smoke-sandbox-loopback-denied

**Logged**: 2026-08-27T12:30:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

隔离 setup 进程需要获批权限监听 18198，沙箱内 curl 也不能跨该边界访问它。

### Error

```text
curl: (7) Failed to connect to 127.0.0.1 port 18198
```

### Context

- 获批后的测试进程仍在运行，自动令牌文件已出现并通过 `0600`、65 字节和 64 位十六进制格式检查。
- 现有本地 18197 服务未停止或改令牌。

### Suggested Fix

在与监听进程相同的受控权限边界内请求 `/ready`，随后发送 SIGINT 检查文件清理。

### Metadata

- Reproducible: yes
- Related Files: cmd/server/main.go, internal/server/setup_token.go

### Resolution

- **Resolved**: 2026-08-27T12:31:00+08:00
- **Notes**: 同权限 `/ready` 返回 `SETUP`；SIGINT 后进程退出码 0，生成文件已删除。

---
## [ERR-20260827-009] ambient-local-tab-not-claimable

**Logged**: 2026-08-27T12:45:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

环境提示存在 5175 标签，但当前 in-app browser 的可接管标签列表没有该 URL。

### Error

```text
Local 5175 tab is not open
```

### Context

- 浏览器连接成功，但 `browser.user.openTabs()` 未返回匹配标签。
- 未关闭、刷新或修改用户标签。

### Suggested Fix

本地只读诊断可在同一浏览器新建临时标签访问目标 URL；不要假定 ambient tab 一定可接管。

### Metadata

- Reproducible: unknown
- Related Files: web/vite.config.ts

### Resolution

- **Resolved**: 2026-08-27T12:46:00+08:00
- **Notes**: 改为创建临时 5175 标签，继续同一反馈循环。

---
## [ERR-20260827-018] migration-diagnosis-local-tooling

**Logged**: 2026-08-27T18:45:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tests

### Summary
迁移诊断中多项本地工具默认路径不可用，且一次 `rg` 命令错误包含 shell 反引号。

### Error

```text
sqlite3: unable to open database file
headroom retrieve: approval review deadline
go test: go-build cache operation not permitted
pgrep: Cannot get process list
socat: command not found
zsh: command not found: gorm
```

### Resolution
- SQLite 改用获批的 `mode=ro&immutable=1` URI。
- Go 命令统一使用 `/tmp/well-ambient-gocache`。
- 进程树改用精确 PID 的获批 `ps`/`lsof`；无需 `socat`。
- 后续 shell 搜索禁止反引号模式，使用不含命令替换字符的 `rg` 表达式。

---

## [ERR-20260827-017] migration-plan-multi-file-context-drift

**Logged**: 2026-08-27T13:35:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
迁移诊断计划的跨文件原子补丁因 `progress.md` 标题层级与预期不同而失败。

### Error

```text
apply_patch verification failed: Failed to find expected lines in progress.md
```

### Resolution
先用 `rg` 确认各文件标题层级，再逐文件应用补丁；没有产生半更新。

---

## [ERR-20260827-010] local-vite-port-not-listening

**Logged**: 2026-08-27T12:48:00+08:00
**Priority**: high
**Status**: resolved
**Area**: frontend

### Summary

用户报告本地没有引导时，5175 实际未监听，浏览器连接被拒绝。

### Error

```text
net::ERR_CONNECTION_REFUSED
```

### Context

- 使用同一 in-app browser 新建临时标签直连 `http://127.0.0.1:5175/`。
- 页面尚未进入 Svelte setup gate，不能把问题归因为 UI 分支。

### Suggested Fix

先恢复并验证 Vite 监听，再检查 `/api/setup/status` 代理与 setup 后端；把两者纳入本地一键启动合同。

### Metadata

- Reproducible: yes
- Related Files: web/vite.config.ts, package.json, Makefile

---
## [ERR-20260827-011] local-process-inspection-sandbox-denied

**Logged**: 2026-08-27T12:52:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

沙箱内无法读取 8080 进程命令，也不能连接由外部权限边界启动的本地服务。

### Error

```text
ps: operation not permitted
curl: (7) Failed to connect to 127.0.0.1 port 8080
```

### Context

- `lsof` 已确认 PID 84469 监听 8080。
- 本地 `config.yaml` 的非秘密字段显示数据库 driver 为空、端口 8080。

### Suggested Fix

若仍需辨认 8080 进程/API，使用受控只读权限检查；不要据沙箱 curl 失败判断外部进程不存在。

### Metadata

- Reproducible: yes
- Related Files: config.yaml, web/vite.config.ts

---

## [ERR-20260827-012] local-dev-findings-patch-context-drift

**Logged**: 2026-08-27T12:53:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

组合记录补丁因 `Vite 与` 中的空格上下文不一致而原子失败。

### Error

```text
apply_patch verification failed: Failed to find expected lines in progress.md
```

### Context

- 没有产生半应用。

### Suggested Fix

用 `rg` 读取精确当前行后再拆分补丁。

### Metadata

- Reproducible: no
- Related Files: progress.md

### Resolution

- **Resolved**: 2026-08-27T12:54:00+08:00
- **Notes**: 已按当前文本完成记录。

---
## [ERR-20260827-013] vite-dev-sandbox-listen-denied

**Logged**: 2026-08-27T12:58:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

Vite 无法在默认沙箱内监听 127.0.0.1:5175。

### Error

```text
local development server exited while binding 127.0.0.1:5175
```

### Context

- 命令未留下运行进程。

### Suggested Fix

用受控本地 loopback 权限重跑同一 dev 命令。

### Metadata

- Reproducible: yes
- Related Files: web/vite.config.ts

---
## [ERR-20260827-014] browser-local-url-policy-block

**Logged**: 2026-08-27T13:02:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tests

### Summary

浏览器控制策略阻止自动刷新本地 5175 页面。

### Error

```text
browser URL policy blocks this action
```

### Context

- Vite 已在 5175 监听。
- 不使用其他浏览器、CDP 或间接浏览器控制绕过。

### Suggested Fix

改用只读本地 HTTP 探针验证 HTML 和 setup API；用户可在已打开页面手动刷新。

### Metadata

- Reproducible: yes
- Related Files: web/vite.config.ts

### Resolution

- **Resolved**: 2026-08-27T13:03:00+08:00
- **Notes**: 停止浏览器自动化，采用更安全的本地 curl 反馈循环。

---

## [ERR-20260827-015] post-fix-ps-sandbox-denied

**Logged**: 2026-08-27T13:18:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
修复后再次用 `ps` 读取启动参数时仍被沙箱拒绝。

### Error

```text
zsh:1: operation not permitted: ps
```

### Resolution
改用 `lsof` 确认监听进程与临时运行目录，并用同源 HTTP 验证实际行为；没有扩大权限。

---

## [ERR-20260827-016] completion-record-patch-context-drift

**Logged**: 2026-08-27T13:22:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
跨多个记录文件的完成状态补丁因 `findings.md` 上下文漂移而原子失败。

### Error

```text
apply_patch verification failed: Failed to find expected lines in findings.md
```

### Resolution
拆分为逐文件补丁，并用稳定标题作为上下文；没有产生半应用。

---
## [ERR-20260827-019] git-add-sandbox-index-lock

**Logged**: 2026-08-27T14:38:43Z
**Priority**: low
**Status**: resolved
**Area**: config

### Summary

`git add -A` could not create `.git/index.lock` under the workspace-write sandbox.

### Error

```text
fatal: Unable to create '/Users/eddie/Workspace/well-ambient/.git/index.lock': Operation not permitted
```

### Context

- Command: `git add -A`
- The repository had no pre-existing `.git/index.lock`; the sandbox allowed worktree edits but exposed `.git` read-only.

### Suggested Fix

Retry the same narrowly scoped Git command with the approved `git add` escalation instead of changing repository state or removing lock files.

### Metadata

- Reproducible: yes
- Related Files: `.git/index`
- See Also: ERR-20260827-007

### Resolution

- **Resolved**: 2026-08-27T14:38:43Z
- **Notes**: Retried through the scoped Git staging approval path.

---
## [ERR-20260827-020] deployment-contract-suite-hit-concurrent-dockerfile-change

**Logged**: 2026-08-27T15:07:29Z
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

The broad database setup contract suite failed on an unrelated Dockerfile base-image assertion while diagnosing setup mode.

### Error

```text
production image stages keep build toolchains out of both runtimes: expected FROM debian:bookworm-slim AS server
```

### Context

- Command: `node --experimental-strip-types --test tests/database-setup-contract.test.ts`
- The exact setup-mode contracts passed; a concurrent uncommitted Dockerfile optimization changed the runtime base image after the previous commit.
- The failure did not exercise the reported `Database setup mode is active` behavior.

### Suggested Fix

Use `--test-name-pattern` for the setup/deployment feedback loop, and review the separate Dockerfile change with its own image contract before accepting it.

### Metadata

- Reproducible: yes
- Related Files: `Dockerfile`, `web/tests/database-setup-contract.test.ts`
- See Also: ERR-20260827-019

### Resolution

- **Resolved**: 2026-08-27T15:07:29Z
- **Notes**: The focused setup/Compose contract passed 2/2 and the Go apply/restart tests passed.

---
## [ERR-20260827-021] combined-contract-and-plan-patch-context-mismatch

**Logged**: 2026-08-27T15:17:39Z
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary

A combined patch for the healthcheck regression contract and task plan failed because the task-plan context had drifted.

### Error

```text
apply_patch verification failed: Failed to find expected lines in task_plan.md
```

### Context

- The patch attempted to update two files atomically.
- No partial changes were applied.

### Suggested Fix

Patch the exact test seam separately and append a self-contained task-plan section without relying on an older neighboring heading.

### Metadata

- Reproducible: no
- Related Files: `cmd/server/release_artifact_contract_test.go`, `task_plan.md`
- See Also: ERR-20260827-017

### Resolution

- **Resolved**: 2026-08-27T15:17:39Z
- **Notes**: Split into exact single-file patches.

---
