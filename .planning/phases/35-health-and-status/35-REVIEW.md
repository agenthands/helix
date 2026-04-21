---
phase: 35-health-and-status
reviewed: 2026-04-21T14:30:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/kernel/lspool/health.go
  - internal/kernel/lspool/health_test.go
  - internal/kernel/lspool/worker.go
  - internal/kernel/lspool/circuit.go
  - internal/kernel/kernel.go
  - internal/kernel/health/tools.go
  - internal/kernel/health/tools_test.go
  - internal/daemon/daemon.go
  - internal/cli/status.go
  - internal/cli/status_output.go
  - internal/cli/status_test.go
  - internal/cli/root.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 35: Code Review Report

**Reviewed:** 2026-04-21T14:30:00Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

The health and status feature is well-structured with clear separation of concerns: data layer in `lspool/health.go`, MCP tool in `health/tools.go`, gRPC RPC in `daemon.go`, and CLI in `cli/status.go`. The snapshot approach (copy under RLock, classify after unlock) is correct. Test coverage is solid with good edge case handling.

Three warnings were identified: `os.Exit` calls inside a cobra `RunE` handler bypass deferred cleanup, `FilterReport` can leave empty workspace shells in the output, and `HasFailures` does not consider `half_open` circuits as failures despite the CLI guidance suggesting it should for consistency. Two info items note minor code quality improvements.

## Warnings

### WR-01: os.Exit in cobra RunE bypasses deferred cleanup and testability

**File:** `internal/cli/status.go:48,55,110`
**Issue:** The `runStatus` function is a cobra `RunE` handler, but calls `os.Exit(1)` in three places. This bypasses any deferred functions (including `conn.Close()` on line 67 and `cancel()` on line 72), prevents cobra from running its post-run hooks, and makes the function untestable since `os.Exit` terminates the process. The two early exits (lines 48, 55) also skip the gRPC connection setup, so cleanup is not an issue there, but the exit on line 110 occurs after `defer conn.Close()` is registered.
**Fix:** Return a sentinel error or use cobra's built-in exit code support. For the "daemon not running" case, return an error that the caller can translate to an exit code. For the failure exit on line 110, refactor so `RunE` returns an error and let the caller decide the exit code:
```go
// Define a typed error for exit code control
var errDaemonNotRunning = fmt.Errorf("daemon not running")
var errUnhealthyWorkers = fmt.Errorf("unhealthy workers detected")

// In runStatus, replace os.Exit calls:
if _, err := os.Stat(socketPath); os.IsNotExist(err) {
    return errDaemonNotRunning
}
// ...
if printer.HasFailures(&report) {
    return errUnhealthyWorkers
}
```
Then in the root command or a wrapper, map these errors to exit codes.

### WR-02: FilterReport leaves empty workspace entries after filtering

**File:** `internal/kernel/health/tools.go:62-84`
**Issue:** When `FilterReport` filters workers and circuits in non-verbose mode, workspaces where all workers were healthy and all circuits were closed end up with empty `Workers` and `Circuits` slices but remain in the `Workspaces` array. For the MCP tool JSON output, this produces workspace objects with empty arrays, which is noisy. For the CLI, these empty workspaces still render a "Workspace: /path" header with no content underneath.
**Fix:** Add a post-filter step to remove empty workspaces:
```go
// After the filtering loop, remove empty workspaces.
var nonEmpty []lspool.WorkspaceHealth
for _, ws := range report.Workspaces {
    if len(ws.Workers) > 0 || len(ws.Circuits) > 0 {
        nonEmpty = append(nonEmpty, ws)
    }
}
report.Workspaces = nonEmpty
```

### WR-03: HasFailures inconsistency with half_open circuit state

**File:** `internal/cli/status_output.go:97-111`
**Issue:** `HasFailures` only checks for `State == "open"` circuits, not `"half_open"`. However, `classifyWorker` in `health.go:189` marks a worker as `"degraded"` when the circuit is half-open, which means the system is partially impaired. Meanwhile, `HasFailures` returns false for half_open circuits paired with degraded workers. If the intent of the exit code 1 is "any non-healthy state," half_open should be included. If the intent is "only hard failures," the current behavior is correct but should be documented with a comment explaining the deliberate exclusion.
**Fix:** Either include half_open or add a clarifying comment:
```go
// Option A: Include half_open as a failure signal
for _, c := range ws.Circuits {
    if c.State == "open" || c.State == "half_open" {
        return true
    }
}

// Option B: Document the deliberate exclusion
// half_open circuits are excluded: they indicate a recovery probe is
// in progress, not a permanent failure. Exit code 1 is reserved for
// states that require operator attention.
```

## Info

### IN-01: Unused Languages field in WorkspaceHealth from pool snapshot

**File:** `internal/kernel/lspool/health.go:31`
**Issue:** `WorkspaceHealth.Languages` is declared in the struct but never populated by `HealthSnapshot()`. It is only filled later by `kernel.HealthStatus()` (kernel.go:132-138). This means if anyone calls `pool.HealthSnapshot()` directly, `Languages` will always be nil. The field arguably belongs at the kernel level rather than the pool level, since the pool has no language detection awareness.
**Fix:** No immediate code change needed. Consider adding a comment on the `Languages` field:
```go
// Languages is populated by kernel.HealthStatus(), not by pool.HealthSnapshot().
Languages []string `json:"languages"`
```

### IN-02: Double gRPC call when --json is used without --verbose

**File:** `internal/cli/status.go:84-101`
**Issue:** When `--json` is passed without `--verbose`, the code makes an initial gRPC call with `verbose=false`, then immediately makes a second call with `verbose=true` to get the full data for JSON output. This is a minor inefficiency -- the code could simply always pass `verbose=true` when `--json` is set, avoiding the redundant first call.
**Fix:**
```go
// At the top of runStatus, adjust verbose for JSON mode:
if jsonOut {
    verbose = true
}
```

---

_Reviewed: 2026-04-21T14:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
