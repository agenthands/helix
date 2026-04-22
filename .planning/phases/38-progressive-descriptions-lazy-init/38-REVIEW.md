---
phase: 38-progressive-descriptions-lazy-init
reviewed: 2026-04-22T12:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - internal/mcp/registry.go
  - internal/mcp/middleware.go
  - internal/mcp/lazy_init.go
  - internal/mcp/lazy_init_test.go
  - internal/mcp/server.go
  - internal/kernel/help/help.go
  - internal/kernel/help/tools.go
  - internal/kernel/help/help_test.go
  - internal/kernel/symbols/tools.go
  - internal/kernel/edit/tools.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/diag/tools.go
  - internal/kernel/health/tools.go
  - internal/skill/memory/skill.go
  - internal/skill/workflow/skill.go
  - internal/skill/repomap/skill.go
  - internal/profile/skill.go
  - internal/daemon/daemon.go
  - test/bench/tools_descriptions_test.go
  - test/bench/main_test.go
  - test/bench/tools_manifest_test.go
findings:
  critical: 0
  warning: 3
  info: 3
  total: 6
status: issues_found
---

# Phase 38: Code Review Report

**Reviewed:** 2026-04-22T12:00:00Z
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

## Summary

Phase 38 introduces progressive tool descriptions (BriefDescription for tools/list, HelpText for get_tool_help), lazy workspace initialization middleware, and a new help tool. The implementation is generally well-structured with good test coverage for the lazy init middleware and help formatting. All tool registrations consistently provide BriefDescription and HelpText. The middleware ordering (LIFO chain: lazy init last-installed = first-executed) is correctly documented and implemented.

Three warnings were found: a race condition in the lazy init middleware where a failed sync.Once becomes permanently stuck, a data race on `nextCalled` in test code, and the `toolSchemas` slice in SerenaMCPServer lacking synchronization. Three info items note minor code quality observations.

## Warnings

### WR-01: Lazy init sync.Once is permanently poisoned on activation failure

**File:** `internal/mcp/lazy_init.go:82-98`
**Issue:** When `activateFn` returns an error, the `sync.Once` for that path is consumed and will never fire again. The `initErr` is captured inside the `Do` closure, but on subsequent calls `once.Do` is a no-op, so `initErr` remains at its zero value (nil). This means the second call after a failed activation will skip activation entirely and proceed to `next()` as if the workspace is active -- but `isActiveFn()` still returns false, so it enters the activation branch again, gets the already-consumed Once, calls `Do` (which does nothing), sees `initErr == nil`, and falls through to `next()` with no workspace active.

The net effect: the first call gets a proper error response, but all subsequent calls silently proceed without an active workspace, likely causing downstream tool failures with confusing error messages.

**Fix:** Replace `sync.Once` with a pattern that allows retrying on failure, such as storing both the Once and the error, and resetting the Once on failure:

```go
type initState struct {
    once sync.Once
    err  error
}

// In the middleware handler, after once.Do:
if initErr != nil {
    // Reset so next call retries activation
    m.mu.Lock()
    delete(m.initOnce, root)
    m.mu.Unlock()
    // return error result...
}
```

### WR-02: Data race on `nextCalled` variable in lazy init test

**File:** `internal/mcp/lazy_init_test.go:44-73`
**Issue:** The variable `nextCalled` is written inside the `next` closure (line 46) and read on lines 60, 65, and 73 without synchronization. While this test currently runs sequentially, the pattern is fragile. More importantly, the concurrent test at line 105 discards both result and error from the handler (`_, _ = handler(...)`), which means it would not catch a panic or unexpected error in the concurrent path.

**Fix:** Use `atomic.Int32` for `nextCalled` or protect with a mutex. For the concurrent test, at minimum check for errors:

```go
nextCalled := atomic.Int32{}
next := func(...) (mcpsdk.Result, error) {
    nextCalled.Add(1)
    return &mcpsdk.CallToolResult{}, nil
}
```

### WR-03: toolSchemas slice in SerenaMCPServer has no synchronization

**File:** `internal/mcp/server.go:173-174`
**Issue:** The `toolSchemas` field is a `[]*mcpsdk.Tool` slice that is appended to in `AddTool`, `AddToolWithMeta`, `AddSkillTool`, and the built-in registration methods, then read in `CollectToolSchemas`. While in practice all writes happen during daemon initialization (single goroutine) and reads happen after initialization, there is no formal synchronization. If `AddTool` were ever called concurrently with `CollectToolSchemas` (or with another `AddTool`), this would be a data race. The `ToolRegistry` correctly uses `sync.RWMutex` but `toolSchemas` does not benefit from that lock.

**Fix:** Either document that `AddTool`/`AddToolWithMeta`/`AddSkillTool` must only be called during single-threaded initialization (and add a comment to that effect), or protect the slice with the same mutex pattern used by `ToolRegistry`.

## Info

### IN-01: Linear scan in get_tool_help for schema lookup

**File:** `internal/kernel/help/tools.go:47-50`
**Issue:** The schema lookup iterates over all tool schemas to find a match by name. With 43 tools this is negligible, but a map would be more idiomatic.

**Fix:** Consider pre-building a `map[string]*mcpsdk.Tool` from `CollectToolSchemas()` at registration time, or accept the linear scan given the small tool count.

### IN-02: Unused outcomeEnum values declared but not wired

**File:** `internal/mcp/middleware.go:61-65`
**Issue:** `outcomeInvalidArgs`, `outcomeNotFound`, and `outcomeLSCrash` are declared with TODO comments for v1.3 but are only referenced in the `outcomeEnum` slice for CI assertions. The variables themselves are technically dead code.

**Fix:** This is intentional per the comments (pre-declared for dashboard vocabulary). No action required, but consider using `_ = outcomeInvalidArgs` or similar to suppress lint warnings if they arise.

### IN-03: Duplicated helper functions across tool packages

**File:** Multiple files (`symbols/tools.go:99-114`, `edit/tools.go:77-92`, `fileops/tools.go:178-193`, `diag/tools.go:93-108`, `health/tools.go:109-125`, `help/tools.go:74-90`)
**Issue:** `textResult()` and `errorResult()` helper functions are duplicated identically in 6 packages. While each is package-private and Go conventions accept this pattern, the duplication is notable.

**Fix:** Consider extracting to a shared internal package (e.g., `internal/mcp/result`) if the codebase continues to grow. Acceptable as-is given Go's preference for small, focused packages.

---

_Reviewed: 2026-04-22T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
