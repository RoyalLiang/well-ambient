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

## [ERR-20260705-001] bundled_soffice_missing_little_cms

**Logged**: 2026-07-05T13:15:45Z
**Priority**: low
**Status**: pending
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
