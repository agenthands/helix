---
phase: 22-error-taxonomy
reviewed: 2026-04-15T12:00:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - internal/errors/errors.go
  - internal/errors/errors_test.go
  - internal/errors/kinds.go
  - internal/kernel/lspool/circuit_err.go
  - internal/kernel/lspool/circuit_test.go
  - internal/kernel/lspool/circuit.go
  - internal/kernel/lspool/pool.go
  - internal/mcp/errors.go
  - internal/mcp/server_test.go
findings:
  critical: 1
  warning: 4
  info: 3
  total: 8
status: issues_found
---

# Phase 22: Code Review Report

**Reviewed:** 2026-04-15T12:00:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

The error taxonomy implementation is well-structured: the `internal/errors` package provides a clean Kind-based error type with sentinel matching, builder pattern, and JSON serialization. The circuit breaker integration in `lspool` correctly migrates to the new `serr.Error` type. The MCP errors bridge layer provides backward-compatible re-exports.

Key concerns: a race condition in the pool's `spawnWorkerLocked` method where the mutex is released during worker start, a missing default for `RestartBudget` in `DefaultPoolConfig`, and mutation safety on exported sentinel errors.

## Critical Issues

### CR-01: Race condition in spawnWorkerLocked due to mutex release mid-operation

**File:** `internal/kernel/lspool/pool.go:273-276`
**Issue:** `spawnWorkerLocked` unlocks `p.mu` before calling `worker.Start(startCtx)` and re-locks after. During this window, another goroutine can call `AcquireLease` or `PromoteToDirty`, which also acquire `p.mu`, and may spawn additional workers or modify `p.workers`/`p.leases` maps concurrently. The `len(p.workers) >= p.config.MaxWorkers` check at line 133 may have already passed before the unlock, so a concurrent caller could also pass it, leading to exceeding MaxWorkers. Additionally, the `p.nextID` counter at line 243 could produce duplicate IDs if two goroutines interleave between the increment and map insertion.
**Fix:** Track "in-flight spawns" with a counter incremented before unlocking, so the MaxWorkers check accounts for pending spawns. Alternatively, restructure to avoid unlocking the mutex:
```go
// Option A: Account for in-flight spawns in the MaxWorkers check.
// Before p.mu.Unlock():
p.inFlightSpawns++
p.mu.Unlock()
err := worker.Start(startCtx)
p.mu.Lock()
p.inFlightSpawns--

// And in the MaxWorkers check:
if len(p.workers) + p.inFlightSpawns >= p.config.MaxWorkers {
    return nil, ErrMaxWorkersReached
}
```

## Warnings

### WR-01: DefaultPoolConfig omits RestartBudget, resulting in zero-value default

**File:** `internal/kernel/lspool/pool.go:26-34`
**Issue:** `DefaultPoolConfig()` does not set `RestartBudget`. The field comment says "default 3" but the returned struct has `RestartBudget: 0`. When `NewCircuitBreaker` receives 0, it defaults to 3 internally (line 37-39 of circuit.go), so this works by accident. However, if a caller inspects `DefaultPoolConfig().RestartBudget` they see 0, not 3, which is misleading and fragile.
**Fix:**
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

### WR-02: Exported sentinel errors are mutable and can be corrupted

**File:** `internal/errors/kinds.go:27-34`
**Issue:** The sentinel errors (`ErrNotFound`, `ErrInvalidArgs`, etc.) are exported `var` pointers to `*Error`. Any caller can mutate them (e.g., `serr.ErrNotFound.Message = "hacked"`), which would corrupt the sentinel for all subsequent `errors.Is` callers. Similarly, `internal/mcp/errors.go:12` creates `ErrSessionExpired` as a mutable `var` that could be mutated.
**Fix:** This is a known Go limitation with pointer sentinels. Document the contract explicitly that sentinels must not be mutated, or consider making `Kind` the sentinel and using a function-based matching approach. At minimum, add a comment:
```go
// Sentinel errors for use with errors.Is. DO NOT mutate these values.
// Each sentinel carries only a Kind; Error.Is() compares Kind values.
```

### WR-03: probing atomic accessed outside mutex creates subtle ordering concern

**File:** `internal/kernel/lspool/circuit.go:70,99,120`
**Issue:** `cb.probing` is an `atomic.Bool` accessed both inside the mutex (`RecordFailure` line 70, `RecordSuccess` line 99) and inside the mutex but via CAS in `CanAttempt` line 120. The atomic is redundant since all three methods hold `cb.mu`. If the intent was lock-free half-open probing (the CAS at line 120), then `CanAttempt` should not hold the mutex for that operation. As-is, the CAS provides no benefit since the mutex already serializes access, and the mixed locking pattern (mutex + atomic on same field) makes the concurrency contract unclear.
**Fix:** Either remove the atomic and use a plain `bool` (since the mutex already protects all access), or redesign `CanAttempt` to use the atomic without the mutex for the probe path. Using a plain bool is simpler:
```go
type CircuitBreaker struct {
    // ...
    probing bool // guarded by mu
}
```

### WR-04: Floating-point equality comparison for circuit state

**File:** `internal/kernel/lspool/circuit.go:54`
**Issue:** `setStateLocked` compares `cb.state == next` using `float64` equality. While the current values (0, 1, 2) are exactly representable in IEEE 754, using `float64` for an enum is unconventional in Go and invites future bugs if intermediate values are added. The type choice appears driven by the metrics gauge API (`LSPoolCircuitStateSet` takes `float64`), but the internal state should use a proper enum type.
**Fix:** Use an `int` or typed constant for the internal state and convert to `float64` only at the metrics boundary:
```go
type circuitState int
const (
    stateClosed   circuitState = iota
    stateHalfOpen
    stateOpen
)
// In setStateLocked, convert: cb.sink.LSPoolCircuitStateSet(cb.language, float64(next))
```

## Info

### IN-01: Custom string search in test when stdlib is available

**File:** `internal/kernel/lspool/circuit_test.go:165-176`
**Issue:** The test file implements custom `containsStr` and `contains` helper functions that duplicate `strings.Contains` from the standard library. The `strings` package is not imported.
**Fix:** Replace with `strings.Contains`:
```go
import "strings"
// Then use: strings.Contains(msg, "circuit_open")
```

### IN-02: Unused variable in TestDecorrelatedJitter

**File:** `internal/kernel/lspool/circuit_test.go:63,71`
**Issue:** The variable `cb` is created at line 63 but never used meaningfully -- only a blank identifier assignment at line 71 (`_ = cb`). This is dead code.
**Fix:** Remove lines 63 and 71, or use `cb` in the test if it was meant to accumulate failures across iterations.

### IN-03: ErrorDetail has redundant MarshalJSON that adds no behavior

**File:** `internal/mcp/errors.go:28-31`
**Issue:** `ErrorDetail.MarshalJSON` creates a type alias and marshals through it, but this is only necessary to prevent infinite recursion when the type implements `json.Marshaler`. Since the method itself is the only `MarshalJSON` on the type, removing it would let `encoding/json` marshal the struct using default field-based encoding with identical results. The method is a no-op wrapper.
**Fix:** Remove the `MarshalJSON` method entirely. The struct tags (`json:"code"`, etc.) will produce the same output via default marshaling.

---

_Reviewed: 2026-04-15T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
