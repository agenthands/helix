---
phase: 13-graceful-degradation
reviewed: 2026-04-10T12:00:00Z
depth: standard
files_reviewed: 18
files_reviewed_list:
  - internal/config/config.go
  - internal/config/defaults.go
  - internal/daemon/daemon.go
  - internal/daemon/shutdown_graceful_test.go
  - internal/degrade/budget.go
  - internal/degrade/budget_test.go
  - internal/kernel/lspool/circuit.go
  - internal/kernel/lspool/circuit_err.go
  - internal/kernel/lspool/circuit_test.go
  - internal/kernel/lspool/metrics_test.go
  - internal/kernel/lspool/pool.go
  - internal/kernel/lspool/pool_test.go
  - internal/mcp/middleware.go
  - internal/mcp/middleware_deadline_test.go
  - internal/mcp/telemetry_middleware_test.go
  - internal/mcp/telemetry_span_test.go
  - test/bench/metrics_bench_test.go
  - test/bench/tracing_bench_test.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 13: Code Review Report

**Reviewed:** 2026-04-10T12:00:00Z
**Depth:** standard
**Files Reviewed:** 18
**Status:** issues_found

## Summary

Phase 13 introduces graceful degradation: per-tool-class timeout budgets injected via middleware, a circuit breaker with decorrelated jitter backoff and restart budget, a `DegradationConfig` struct with sane defaults, and integration into the daemon bootstrap. The code is well-structured, thoroughly tested, and follows project conventions. However, there is one critical concurrency issue in the worker pool's spawn path, plus several lower-severity items.

## Critical Issues

### CR-01: spawnWorkerLocked drops and re-acquires mutex, allowing MaxWorkers to be exceeded

**File:** `internal/kernel/lspool/pool.go:275-277`
**Issue:** `spawnWorkerLocked` (which is documented as "Must be called with p.mu held") temporarily releases `p.mu` around `worker.Start()` (lines 275-277). While the lock is released, concurrent `AcquireLease` or `PromoteToDirty` callers can pass the `len(p.workers) >= p.config.MaxWorkers` check and also enter `spawnWorkerLocked`, because the count has not yet been incremented. When all goroutines re-acquire the lock and insert their workers into the map, the pool can exceed `MaxWorkers`. This also means the freshly allocated `nextID` may collide or the worker map state may have changed in ways the caller does not expect.
**Fix:** Reserve a slot in the workers map before releasing the lock, or use a separate semaphore to enforce MaxWorkers. For example:
```go
// Before unlocking, reserve the slot with a placeholder:
p.nextID++
id := fmt.Sprintf("w-%s-%d", wsKey.Language, p.nextID)
p.workers[id] = nil // placeholder reserves the slot

p.mu.Unlock()
err := worker.Start(startCtx)
p.mu.Lock()

if err != nil {
    delete(p.workers, id) // release reservation
    return nil, err
}
p.workers[id] = worker
```

## Warnings

### WR-01: DefaultPoolConfig omits RestartBudget, resulting in zero value

**File:** `internal/kernel/lspool/pool.go:27-34`
**Issue:** `DefaultPoolConfig()` does not set `RestartBudget`, so it defaults to `0`. While `NewCircuitBreaker` clamps `restartBudget <= 0` to `3`, and daemon.go explicitly passes `cfg.Degradation.RestartBudget` into `PoolConfig`, any code path that uses `DefaultPoolConfig()` directly (line 179 in daemon.go when `poolCfg.BaseTTL == 0`) will silently get `RestartBudget: 0` in the PoolConfig struct. The circuit breaker constructor compensates, but the PoolConfig is inconsistent with the documented default of 3.
**Fix:** Add `RestartBudget: 3` to `DefaultPoolConfig()`:
```go
func DefaultPoolConfig() PoolConfig {
    return PoolConfig{
        BaseTTL:               300,
        CeilingTTL:            3600,
        MaxWorkers:            10,
        RSSHardCapMB:          2048,
        PressureCheckInterval: 10,
        RestartBudget:         3,
    }
}
```

### WR-02: stopAll called with nil context in test, fragile if Worker.Stop behavior changes

**File:** `internal/kernel/lspool/metrics_test.go:174`
**Issue:** `p.stopAll(nil)` passes a nil context. This works today because the fake workers have `process == nil` so `w.process.Stop(ctx)` is never reached. However, if `Worker.Stop` is ever modified to use the context directly (e.g., for logging, deadline propagation), this will panic with a nil context. The Go convention strongly discourages passing nil contexts.
**Fix:** Pass `context.Background()` instead of `nil`:
```go
p.stopAll(context.Background())
```

### WR-03: context.Canceled classified as "internal" outcome rather than dedicated bucket

**File:** `internal/mcp/middleware.go:83-100`
**Issue:** `classifyOutcome` checks for `context.DeadlineExceeded` and `lspool.ErrCircuitOpen` but not `context.Canceled`. When a client disconnects mid-request, the context is canceled and the error is classified as `outcomeInternal`. This mixes legitimate client disconnects (which are expected) with actual internal errors, making the metric less useful for alerting. Client cancellations could dominate the "internal" bucket and hide real errors.
**Fix:** Add a `context.Canceled` check before the generic error fallback, or add a dedicated `"canceled"` outcome value:
```go
if errors.Is(err, context.DeadlineExceeded) {
    return outcomeTimeout
}
if errors.Is(err, context.Canceled) {
    return outcomeTimeout // or a new "canceled" bucket
}
if errors.Is(err, lspool.ErrCircuitOpen) {
    return outcomeCircuitOpen
}
return outcomeInternal
```

## Info

### IN-01: Unused variable in circuit_test.go

**File:** `internal/kernel/lspool/circuit_test.go:63-71`
**Issue:** In `TestDecorrelatedJitter`, a `CircuitBreaker` named `cb` is created on line 63 and never used (line 71: `_ = cb`). This appears to be leftover from a refactor where the inner loop was changed to create fresh `cb2` instances.
**Fix:** Remove the unused `cb` variable:
```go
// Remove lines 63 and 71:
// cb := NewCircuitBreaker("go", maxBackoff, 10, NoopSink{})
// _ = cb
```

### IN-02: Custom containsStr/contains helpers in circuit_test.go duplicate strings.Contains

**File:** `internal/kernel/lspool/circuit_test.go:150-161`
**Issue:** `containsStr` and `contains` are hand-rolled substring search functions. The standard library `strings.Contains` does the same thing more clearly and is already used elsewhere in the codebase via testify.
**Fix:** Replace with `strings.Contains`:
```go
import "strings"

// Replace containsStr(msg, "go") with:
strings.Contains(msg, "go")
```

---

_Reviewed: 2026-04-10T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
