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
