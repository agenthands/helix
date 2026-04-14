---
phase: 19-protocol-contract-oracles
reviewed: 2026-04-11T12:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - test/oracle/protocol/handshake_test.go
  - test/oracle/protocol/tools_list_test.go
  - test/oracle/protocol/session_isolation_test.go
  - test/oracle/protocol/reconnect_test.go
  - test/oracle/contract/schema_test.go
  - test/oracle/contract/selectability_test.go
  - test/oracle/contract/golden_test.go
  - test/oracle/contract/errors_test.go
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 19: Code Review Report

**Reviewed:** 2026-04-11T12:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Eight integration test files implementing protocol oracle and contract oracle test suites for MCP tool verification. The code is well-structured overall: proper use of build tags, test helpers, golden file patterns, and harness abstractions. The tests cover handshake validation, session isolation, reconnect resilience, schema validation, tool selectability, golden output capture, and error contract verification.

Key concerns: a potential data race from subtests modifying a shared map, a runner resource leak in `listAllTools`, and fragile assumptions in session isolation assertions.

## Warnings

### WR-01: Data race in TestToolsList_UniqueNames -- subtest writes to shared map

**File:** `test/oracle/protocol/tools_list_test.go:28-34`
**Issue:** The `seen` map is declared in the parent test and mutated inside `t.Run` subtests. When `t.Run` subtests execute in parallel (or when `-count` > 1 triggers concurrent subtest scheduling), writing to the shared `seen` map without synchronization is a data race. Even without explicit `t.Parallel()`, the Go test runner documentation notes that subtests from `t.Run` can interleave with other tests in the same goroutine, but more critically, if someone later adds `t.Parallel()` to these subtests (a common refactor), this becomes a hard crash. The pattern of mutating outer-scope state from within `t.Run` is a known footgun.
**Fix:** Either move the duplicate check outside the subtest loop, or protect the map:
```go
seen := make(map[string]bool, len(result.Tools))
for _, tool := range result.Tools {
    require.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
    seen[tool.Name] = true
}
```
This removes the `t.Run` wrapper (subtests here only add per-tool naming, not real isolation). Alternatively, keep subtests but make `seen` checks after collecting all names.

### WR-02: Runner leak in schema_test.go listAllTools -- caller ignores returned runner

**File:** `test/oracle/contract/schema_test.go:47`
**Issue:** `listAllTools` returns both `([]*mcp.Tool, *Runner)` but callers at lines 47, 63, 79, 104 all discard the runner with `tools, _ := listAllTools(t)`. While the runner is cleaned up via `t.Cleanup` (registered in `StartRunner`), the `defer runner.Stop()` pattern used everywhere else is bypassed. The runner's `Stop()` is registered via `tb.Cleanup(r.Stop)` inside `StartRunner`, so this is safe. However, the function signature returning `*Runner` suggests the caller should manage it. The return value is misleading and the second return should be removed since all callers ignore it.
**Fix:** Simplify `listAllTools` to not return the runner, since `t.Cleanup` already handles lifecycle:
```go
func listAllTools(t *testing.T) []*mcp.Tool {
    t.Helper()
    runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
    _ = runner // kept alive by t.Cleanup
    // ...
    return result.Tools
}
```

### WR-03: Fragile assertion -- tool count inequality may break with profile changes

**File:** `test/oracle/protocol/session_isolation_test.go:121-123`
**Issue:** `TestSessionIsolation_ToolListIndependence` asserts `require.NotEqual(t, len(toolsA), len(toolsB))` to prove read and edit modes resolve tool lists independently. This assertion is fragile: if a future profile change happens to give read and edit modes the same number of tools (even with different tool sets), this test fails with a misleading error about isolation being broken. The test proves "different count" but the contract is "independent resolution."
**Fix:** Assert on specific tool presence/absence (as `TestSessionIsolation_ModeRestrictions` already does) or compare the actual tool name sets rather than just counts:
```go
require.NotEqual(t, toolsA, toolsB,
    "read mode and edit mode should have different tool sets")
```

### WR-04: normalizeResponse regex may over-match fixture paths

**File:** `test/oracle/contract/golden_test.go:30`
**Issue:** The regex `/[^\s"]+/testdata/fixtures/` uses a greedy character class that matches any non-whitespace, non-quote character. If output contains multiple path-like segments on the same line, the regex could match unintended prefixes. More importantly, this regex does not anchor to a path start (e.g., `/` prefix), so it could match partial strings like `notapath/testdata/fixtures/`. The current regex does require a leading `/` via the literal `/` before `[^\s"]+`, so the actual risk is low, but the greedy match could consume too much if paths appear adjacent.
**Fix:** Consider using a non-greedy quantifier or more specific path characters:
```go
text = regexp.MustCompile(`/[^\s"]+?/testdata/fixtures/`).ReplaceAllString(text, "<FIXTURE_ROOT>/")
```

## Info

### IN-01: Compiled regexes in normalizeResponse should be package-level vars

**File:** `test/oracle/contract/golden_test.go:24-41`
**Issue:** Five `regexp.MustCompile` calls execute on every invocation of `normalizeResponse`. Since these are constant patterns, compiling them once as package-level variables avoids redundant work and follows Go convention.
**Fix:** Extract to package-level:
```go
var (
    reFixturePath = regexp.MustCompile(`/[^\s"]+/testdata/fixtures/`)
    reTmpPath     = regexp.MustCompile(`(?:/tmp|/var/folders)/[^\s"]+`)
    // ...
)
```

### IN-02: selectability_test.go uses assert (soft) while other files use require (hard)

**File:** `test/oracle/contract/selectability_test.go:70-71`
**Issue:** Selectability tests use `assert.True` / `assert.Less` / `assert.NotContains` (which log failures but continue) while other test files consistently use `require` (which stop on first failure). This is likely intentional (selectability is advisory, not blocking), but the inconsistency is worth noting. The `TestSelectability_UniqueDescriptions` at line 83 uses `t.Errorf` directly, which is yet another pattern.
**Fix:** No change needed if the soft-fail behavior is intentional for selectability tests. Consider adding a brief comment at the top of the file documenting this design choice.

### IN-03: TODO comment for future error categories

**File:** `test/oracle/contract/errors_test.go:3`
**Issue:** `TODO(phase-20)` comment documents planned expansion for timeout, circuit_open, and unsupported error categories. This is properly scoped with a phase reference -- no action needed now, noting for tracking.
**Fix:** None required. The TODO is well-scoped.

---

_Reviewed: 2026-04-11T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
