---
phase: 23-tool-migration
reviewed: 2026-04-15T12:00:00Z
depth: standard
files_reviewed: 32
files_reviewed_list:
  - internal/kernel/diag/actions.go
  - internal/kernel/diag/format.go
  - internal/kernel/edit/delete.go
  - internal/kernel/edit/insert.go
  - internal/kernel/edit/planner.go
  - internal/kernel/edit/rename.go
  - internal/kernel/edit/replace.go
  - internal/kernel/edit/tools.go
  - internal/kernel/edit/treesitter.go
  - internal/kernel/fileops/fileops_test.go
  - internal/kernel/fileops/find.go
  - internal/kernel/fileops/list.go
  - internal/kernel/fileops/read.go
  - internal/kernel/fileops/replace.go
  - internal/kernel/fileops/search.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/fileops/validate.go
  - internal/kernel/fileops/write.go
  - internal/kernel/lspool/circuit_test.go
  - internal/kernel/symbols/hierarchy.go
  - internal/kernel/symbols/overview.go
  - internal/kernel/symbols/retrieval.go
  - internal/kernel/symbols/search.go
  - internal/kernel/symbols/tools.go
  - internal/mcp/middleware.go
  - internal/mcp/server.go
  - internal/mcp/server_test.go
  - internal/mcp/telemetry_middleware_test.go
  - internal/mcp/telemetry_span_test.go
  - internal/profile/skill.go
  - internal/skill/memory/skill.go
  - internal/skill/memory/skill_test.go
findings:
  critical: 0
  warning: 4
  info: 4
  total: 8
status: issues_found
---

# Phase 23: Code Review Report

**Reviewed:** 2026-04-15T12:00:00Z
**Depth:** standard
**Files Reviewed:** 32
**Status:** issues_found

## Summary

The migration from raw `fmt.Errorf` to typed `serr.New()`/`serr.Wrap()` errors is well-executed across the kernel tool packages (fileops, edit, symbols, diag), the memory skill, and the profile skill. Error kinds are generally well-chosen: `InvalidArgs` for user-facing validation, `NotFound` for missing resources, `Internal` for infrastructure failures, `Unsupported` for capability gaps. The `errors.Is`/`errors.As` chain is preserved via `serr.Wrap()` which stores a cause field with `Unwrap()` support.

Key observations:
- **Skill tools** (`memory`, `profile`) consistently use `.WithTool()` for attribution -- good pattern.
- **Kernel tools** (`edit`, `symbols`, `fileops`, `diag`) do NOT use `.WithTool()` -- this is acceptable because they rely on `kernel.WrapToolSpan` for telemetry attribution at the MCP handler layer.
- **No remaining `fmt.Errorf` in MCP-facing paths** of the reviewed files. The `fmt.Errorf` instances in `find.go:58` and `search.go:99` are internal flow-control sentinels ("result limit reached") that are caught and suppressed within the same function -- not exposed to callers.
- **`profile/skill.go` Init()** still uses `fmt.Errorf` (lines 46, 53, 59) for loading embedded profiles/overrides. These are startup-time errors, not MCP-tool-facing, so this is a design choice rather than a bug.

## Warnings

### WR-01: memory read_memory uses Internal for NotFound cause

**File:** `internal/skill/memory/skill.go:193-194`
**Issue:** When `s.store.Read(name)` fails because the memory does not exist, the error is wrapped as `serr.Internal`. If the underlying store returns an os.ErrNotExist or similar, the caller receives `internal` kind instead of `not_found`, making it harder for agents to distinguish "memory does not exist" from "store is broken." The test at `skill_test.go:131-134` and `skill_test.go:181-183` expects errors on missing memories but does not assert the error kind.
**Fix:** Check the cause error and use `serr.NotFound` when the memory does not exist:
```go
content, err := s.store.Read(name)
if err != nil {
    if os.IsNotExist(err) {
        return "", serr.Wrap(serr.NotFound, "memory not found", err).WithTool("read_memory")
    }
    return "", serr.Wrap(serr.Internal, "read operation failed", err).WithTool("read_memory")
}
```

### WR-02: memory delete/rename use Internal for NotFound cause

**File:** `internal/skill/memory/skill.go:241-242,275-276`
**Issue:** Same pattern as WR-01. `s.store.Delete(name)` and `s.store.Rename(oldName, newName)` wrap all errors as `serr.Internal`. If the target memory does not exist, agents cannot programmatically distinguish a missing resource from an infrastructure failure.
**Fix:** Apply the same os.IsNotExist check before wrapping, or propagate typed errors from the store layer.

### WR-03: edit package uses Internal for file-not-found scenarios

**File:** `internal/kernel/edit/delete.go:54-55`, `internal/kernel/edit/insert.go:27-28`, `internal/kernel/edit/replace.go:30-31`
**Issue:** `os.ReadFile(filePath)` failures are wrapped as `serr.Internal`. If the file does not exist (the most likely failure mode), this should be `serr.NotFound`. The current code sends "internal: read file" to agents when the file path is simply wrong.
**Fix:** Check `os.IsNotExist(err)` before wrapping:
```go
source, err := os.ReadFile(filePath)
if err != nil {
    if os.IsNotExist(err) {
        return nil, serr.New(serr.NotFound, "file not found").WithDetail(filePath)
    }
    return nil, serr.Wrap(serr.Internal, "read file", err).WithDetail(filePath)
}
```

### WR-04: diag format.go uses Internal for file read failure

**File:** `internal/kernel/diag/format.go:68-69`
**Issue:** `ApplyFormatEdits` wraps `os.ReadFile` failure as `serr.Internal`. Same pattern as WR-03 -- a non-existent file path would be misclassified.
**Fix:** Same os.IsNotExist check as WR-03.

## Info

### IN-01: find.go and search.go use fmt.Errorf for flow control sentinel

**File:** `internal/kernel/fileops/find.go:58`, `internal/kernel/fileops/search.go:99`
**Issue:** `fmt.Errorf("result limit reached")` is used as a walk-termination sentinel, matched by string comparison on lines 65 and 104 respectively. This is fragile -- a typo in either location would silently break the limit check. Consider using a package-level sentinel error variable.
**Fix:**
```go
var errResultLimit = errors.New("result limit reached")
// Then use: return errResultLimit
// And check: if err != nil && !errors.Is(err, errResultLimit) {
```

### IN-02: profile/skill.go Init uses raw fmt.Errorf

**File:** `internal/profile/skill.go:46,53,59`
**Issue:** Three `fmt.Errorf` calls remain in the `Init()` method. These are startup-time errors (not MCP-tool-facing), but for consistency with the rest of the codebase, they could use `serr.Wrap(serr.Internal, ...)`. This is a low-priority consistency item since Init errors are handled at daemon boot, not by MCP clients.
**Fix:** Replace with `serr.Wrap(serr.Internal, "loading embedded profiles", err)` etc.

### IN-03: Unused import of "fmt" retained in replace.go after migration

**File:** `internal/kernel/edit/replace.go:5`
**Issue:** The `fmt` package is imported and used only on line 49 for `fmt.Sprintf` in the invalid byte range error. This is fine -- not an unused import. However, this is the only remaining `fmt.Sprintf` inside an `serr.New()` call in the edit package; the message could be cleaner using `.WithDetail()` instead.
**Fix:** Optional -- restructure to:
```go
return serr.New(serr.Internal, "invalid byte range for file").
    WithDetail(fmt.Sprintf("[%d:%d] len=%d", startByte, endByte, len(source)))
```

### IN-04: edit/tools.go uses fmt.Sprintf in Sprintf-style for tool output strings

**File:** `internal/kernel/edit/tools.go:6,129-131`
**Issue:** `fmt` is imported for output formatting (user-facing text results), not for error construction. This is correct usage -- just noting that `fmt.Sprintf` calls on lines 129, 169, 204, etc. are for MCP tool result text, not error paths. No action needed.
**Fix:** None required.

---

_Reviewed: 2026-04-15T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
