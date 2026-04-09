---
phase: 06-test-harness-go-dogfooding
reviewed: 2026-04-08T12:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - internal/daemon/daemon.go
  - test/integration/harness.go
  - test/integration/helpers.go
  - test/integration/harness_test.go
  - test/integration/a_doc.go
  - test/integration/symbols_test.go
  - test/integration/fileops_test.go
  - test/integration/diag_test.go
  - test/integration/memory_test.go
  - test/integration/workflow_test.go
  - test/integration/profile_test.go
  - test/integration/smoke_http_test.go
  - testdata/fixtures/go/main.go
  - testdata/fixtures/go/pkg/greeter.go
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 6: Code Review Report

**Reviewed:** 2026-04-08T12:00:00Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

This review covers the integration test harness, all test files for the 6 MCP tool categories (symbols, fileops, diagnostics, memory, workflow, profile), HTTP smoke tests, Go test fixtures, and the daemon bootstrap code. The test harness is well-structured with clean separation of concerns, good use of `t.Helper()`, proper `t.Cleanup` registration, and solid behavioral assertion patterns for LS-dependent tests. The daemon code is solid with proper errgroup lifecycle management.

Four warnings were identified: an unchecked error in daemon bootstrap, a busy-wait spin loop in a test, duplicate LS readiness polling logic that should be extracted, and a goroutine leak path in the test harness. Three info items note minor improvements.

## Warnings

### WR-01: Unchecked error from os.UserHomeDir

**File:** `internal/daemon/daemon.go:78`
**Issue:** `homeDir, _ := os.UserHomeDir()` discards the error. If this fails (e.g., in a containerized environment without HOME set), `homeDir` is empty and downstream paths like `filepath.Join("", ".serena", "bin")` and `filepath.Join("", ".serena")` silently resolve to relative paths. This could cause the installer to write binaries into unexpected locations and profile resolution to fail with a confusing error.
**Fix:**
```go
homeDir, err := os.UserHomeDir()
if err != nil {
    return nil, fmt.Errorf("determining home directory: %w", err)
}
```

### WR-02: Busy-wait spin loop in TestHTTPSmoke_WithLS

**File:** `test/integration/smoke_http_test.go:56-71`
**Issue:** The LS readiness poll uses a `for !ready` loop with `default` case and `time.Sleep` instead of a ticker. The `default` case in the select means the `ctx.Done()` check only fires on the exact iteration when context expires -- between sleeps, the goroutine busy-polls through `default` before sleeping. This is functionally correct but wastes CPU during the poll. More importantly, the `select` with `default` plus `time.Sleep` is an anti-pattern in Go that can miss context cancellation during the sleep.
**Fix:** Use a ticker-based pattern consistent with the other LS readiness polls in the codebase:
```go
ticker := time.NewTicker(500 * time.Millisecond)
defer ticker.Stop()
for {
    select {
    case <-ctx.Done():
        t.Skip("gopls did not become ready within timeout")
    case <-ticker.C:
        res, err := td.Session.CallTool(ctx, &mcp.CallToolParams{...})
        if err == nil && !res.IsError && textContent(res) != "" && textContent(res) != "(no results)" {
            ready = true
            goto done
        }
    }
}
done:
```

### WR-03: Duplicate LS readiness polling logic

**File:** `test/integration/symbols_test.go:45-70`, `test/integration/diag_test.go:26-51`, `test/integration/smoke_http_test.go:53-71`
**Issue:** Three test files contain nearly identical LS readiness polling code (poll search_symbols, check for non-error and non-empty result). This duplicated logic is a maintenance burden -- any fix (like WR-02) must be applied in three places. The harness already has `WaitForLS` but it calls `t.Fatalf` on timeout, which is why these tests inline their own version.
**Fix:** Add a `TryWaitForLS` helper to harness.go that returns a boolean instead of fataling:
```go
// TryWaitForLS polls for LS readiness, returning true if ready, false on timeout.
func TryWaitForLS(t *testing.T, session *mcp.ClientSession, timeout time.Duration) bool {
    t.Helper()
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return false
        case <-ticker.C:
            result, err := session.CallTool(ctx, &mcp.CallToolParams{
                Name: "search_symbols", Arguments: map[string]any{"query": "main"},
            })
            if err == nil && !result.IsError {
                text := textContent(result)
                if text != "" && text != "(no results)" {
                    t.Logf("LS ready: %s", text[:min(len(text), 80)])
                    return true
                }
            }
        }
    }
}
```

### WR-04: Kernel goroutine leak path in StartTestDaemon

**File:** `test/integration/harness.go:116-118`
**Issue:** `d.KernelInstance().Run(ctx)` is started in a goroutine but its return value is discarded with `_ =`. If the kernel exits with an error before the test completes (e.g., pool initialization failure), the test continues obliviously with a dead kernel. Additionally, if `Stop()` is called but `Run` does not return promptly, the goroutine leaks until the process exits. While acceptable for tests, the error suppression can mask real failures during test development.
**Fix:** Capture the error and log it so kernel failures are visible in test output:
```go
go func() {
    if err := d.KernelInstance().Run(ctx); err != nil && ctx.Err() == nil {
        t.Errorf("kernel.Run exited unexpectedly: %v", err)
    }
}()
```

## Info

### IN-01: Duplicate skill.InitAll invocation

**File:** `test/integration/harness.go:104`, `internal/daemon/daemon.go:126`
**Issue:** `StartTestDaemon` calls `skill.InitAll(deps)` with test-specific temp dirs, then `daemon.New` calls `skill.InitAll` again with production dirs. If `InitAll` is not idempotent (or the second call overwrites the first), memory tools may write to the production global dir instead of the test temp dir. This works today because the test deps are set first and the daemon's `InitAll` logs a warning, but the double-init is fragile.
**Fix:** Consider adding an option to `daemon.New` to skip skill init when skills are pre-initialized by the test harness, or verify that `InitAll` is documented as idempotent-safe.

### IN-02: Magic line/column numbers in symbol tests

**File:** `test/integration/symbols_test.go:76-79`, `test/integration/symbols_test.go:92-93`, and others
**Issue:** Line and column numbers like `6`, `1`, `10`, `5` are hardcoded without comments explaining which symbol they refer to in the fixture. The comment on line 75 helps but is easy to miss. If fixtures change, these magic numbers silently break tests.
**Fix:** Define named constants or use a lookup table at the top of the test:
```go
const (
    helperCallLine   = 6  // line in main.go where Helper() is called
    helperDefLine    = 10 // line in main.go where func Helper() is defined
    greeterIfaceLine = 5  // line in pkg/greeter.go where Greeter interface is defined
)
```

### IN-03: Test fixture unused import

**File:** `testdata/fixtures/go/pkg/greeter.go:3`
**Issue:** The `fmt` import is used, so this is fine. However, the `NewGreeter` function (line 19) is not called from the main fixture (`testdata/fixtures/go/main.go`), meaning call hierarchy tests targeting `NewGreeter` may not find callers. This is not a bug but a potential gap in test coverage for the `get_call_hierarchy` tool.
**Fix:** Consider adding a call to `pkg.NewGreeter()` from `main.go` to provide a cross-file call hierarchy edge for testing.

---

_Reviewed: 2026-04-08T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
