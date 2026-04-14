---
phase: 18-harness-extraction-foundation
reviewed: 2026-04-11T12:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - test/harness/doc.go
  - test/harness/fixture.go
  - test/harness/golden.go
  - test/harness/runner.go
  - test/harness/runner_test.go
  - test/harness/tools.go
  - test/oracle/contract/doc.go
  - test/oracle/judge/doc.go
  - test/oracle/llm/doc.go
  - test/oracle/protocol/doc.go
  - test/oracle/protocol/smoke_test.go
  - test/oracle/scenario/doc.go
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
---

# Phase 18: Code Review Report

**Reviewed:** 2026-04-11T12:00:00Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

This phase introduces the shared test harness (`test/harness/`) and oracle package scaffolding (`test/oracle/`). The code is well-structured: build tags are consistent, doc.go files establish clear package boundaries, fixture and golden file utilities are clean, and the Runner abstraction neatly wraps daemon + MCP client lifecycle.

Two warnings relate to missing timeouts in tool helper functions and a silently discarded kernel error. Three info items cover minor redundancies and code duplication.

## Warnings

### WR-01: Tool helpers use context.Background() with no timeout

**File:** `test/harness/tools.go:16`
**Issue:** `CallTool`, `CallToolExpectError`, and `ListSessionTools` all use `context.Background()` which has no deadline. If a tool call hangs (e.g., LS deadlock), the test will block indefinitely rather than failing with a clear timeout message. This is especially risky in CI where hung tests consume runners.
**Fix:** Accept a context parameter or derive a context with a reasonable timeout:
```go
func CallTool(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
    tb.Helper()
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    result, err := session.CallTool(ctx, &mcp.CallToolParams{
        Name:      name,
        Arguments: args,
    })
    require.NoError(tb, err, "tool %s call error", name)
    require.False(tb, result.IsError, "tool %s failed: %s", name, TextContent(result))
    return result
}
```
Apply the same pattern to `CallToolExpectError` (line 39) and `ListSessionTools` (line 52).

### WR-02: Kernel goroutine error silently discarded

**File:** `test/harness/runner.go:131-133`
**Issue:** The kernel `Run` goroutine discards its error with `_ = d.KernelInstance().Run(ctx)`. If the kernel fails (e.g., pool initialization error), tests will see mysterious failures with no indication that the kernel died. This makes debugging flaky integration tests harder.
**Fix:** Log or record the error so test failures are diagnosable:
```go
go func() {
    if err := d.KernelInstance().Run(ctx); err != nil && ctx.Err() == nil {
        // Only log if not caused by context cancellation (normal shutdown).
        tb.Errorf("kernel.Run exited with error: %v", err)
    }
}()
```
Note: `tb.Errorf` is safe to call from goroutines in Go 1.24+. For older versions, use `tb.Logf` or a channel.

## Info

### IN-01: Redundant defer runner.Stop() in smoke tests

**File:** `test/oracle/protocol/smoke_test.go:19`
**Issue:** `defer runner.Stop()` is called explicitly, but `StartRunner` already registers `tb.Cleanup(r.Stop)` at `runner.go:163`. The double-cancel is safe (context cancel is idempotent) but unnecessary and could confuse future contributors.
**Fix:** Remove the explicit `defer runner.Stop()` since Cleanup handles it:
```go
func TestSmokeImportHarness(t *testing.T) {
    runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
    // Stop is handled by t.Cleanup registered in StartRunner.
    tools := harness.ListSessionTools(t, runner.Session)
```

### IN-02: Duplicate golden.go between harness and integration

**File:** `test/harness/golden.go:17`
**Issue:** The `flag.Bool("update", ...)` declaration and `AssertTools`/`AssertGolden` functions are duplicated between `test/harness/golden.go` and `test/integration/golden.go`. While safe today (no cross-imports, separate test binaries), this duplication means bug fixes must be applied in two places. The doc.go explicitly forbids harness from importing integration, which is correct -- the migration path would be for integration tests to eventually import harness.
**Fix:** When integration tests are migrated to use the harness package (as the phase plan suggests), remove `test/integration/golden.go` and have integration tests import `harness.GoldenStore` and `harness.AssertGolden` instead.

### IN-03: skill.InitAll called without idempotency guard

**File:** `test/harness/runner.go:119`
**Issue:** `skill.InitAll(deps)` is called on every `StartRunner` invocation. The underlying `InitAll` re-initializes all skills unconditionally. If multiple tests in the same package call `StartRunner` (even sequentially), skills are re-initialized with different temp dirs each time. This works correctly today but is fragile -- if a skill ever retains state across Init calls or registers global side effects, tests could interfere with each other.
**Fix:** No immediate code change needed. Document the expectation that `Skill.Init()` must be idempotent-safe, or add a `sync.Once` guard in `StartRunner` if skills are confirmed to be process-global singletons.

---

_Reviewed: 2026-04-11T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
