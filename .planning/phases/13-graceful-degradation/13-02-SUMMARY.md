---
phase: 13-graceful-degradation
plan: 02
subsystem: mcp-middleware, lspool-circuit
tags: [deadline-propagation, circuit-breaker, typed-errors, decorrelated-jitter, graceful-degradation]
dependency_graph:
  requires: [degrade-package, tool-class-map, budget-lookup, degradation-config]
  provides: [deadline-middleware, typed-circuit-error, jitter-backoff, restart-budget, single-probe-halfopen]
  affects: [internal/mcp, internal/kernel/lspool, internal/daemon]
tech_stack:
  added: [math/rand/v2, sync/atomic]
  patterns: [budget-func-injection, decorrelated-jitter, atomic-cas-probe, typed-error-is-bridge]
key_files:
  created:
    - internal/kernel/lspool/circuit_err.go
    - internal/kernel/lspool/circuit_test.go
    - internal/mcp/middleware_deadline_test.go
  modified:
    - internal/mcp/middleware.go
    - internal/kernel/lspool/circuit.go
    - internal/kernel/lspool/pool.go
    - internal/daemon/daemon.go
    - internal/mcp/telemetry_middleware_test.go
    - internal/mcp/telemetry_span_test.go
    - internal/kernel/lspool/pool_test.go
    - internal/kernel/lspool/metrics_test.go
decisions:
  - "BudgetFunc type used instead of *config.DegradationConfig to avoid import cycle (mcp -> config -> profile -> mcp)"
  - "ErrCircuitOpen sentinel kept in pool.go; CircuitOpenError.Is() bridges for errors.Is compatibility"
  - "restartBudget defaults to 3 in NewCircuitBreaker when <= 0"
metrics:
  duration_seconds: 482
  completed: "2026-04-10"
  tasks_completed: 2
  tasks_total: 2
  test_count: 12
  test_pass: 12
---

# Phase 13 Plan 02: Deadline Propagation & Circuit Breaker Enhancement Summary

Deadline-aware TelemetryMiddleware with per-class timeout budgets via BudgetFunc injection, plus circuit breaker enhanced with decorrelated jitter, restart budget, typed CircuitOpenError, and single-probe half-open via atomic CAS.

## What Was Built

### internal/mcp/middleware.go
- `BudgetFunc` type: `func(toolName string) time.Duration` -- avoids import cycle by injecting budget lookup as a closure
- Deadline injection in `TelemetryMiddleware`: wraps `context.WithTimeout` per tool class BEFORE the tracing span, so timeout covers both span and handler
- `InstallMiddleware` updated to accept `BudgetFunc` parameter
- Nil `BudgetFunc` disables deadline injection (backward compatible)

### internal/mcp/middleware_deadline_test.go
- 5 tests: SlowHandler (completes within budget), Timeout (exceeds budget), ConfigOverride (1s override), NonToolCall (no deadline on tools/list), ZeroBudget (unmapped tool gets ClassRead default)

### internal/kernel/lspool/circuit_err.go
- `CircuitOpenError` struct with `Language`, `BackoffRemaining`, `Failures`, `RetryAfter` fields
- `Error()` includes language and failure count
- `Is(target)` returns true for `ErrCircuitOpen` sentinel -- backward compatibility bridge

### internal/kernel/lspool/circuit.go
- `restartBudget` field: max consecutive failures before permanent open (D-05)
- `probing` atomic.Bool: single-probe half-open guard (D-06)
- `RecordFailure()`: decorrelated jitter (D-04) replacing pure exponential backoff
- `CanAttempt()`: restart budget check + atomic CAS probe admission
- `CircuitOpenErr()`: creates typed error from current circuit state

### internal/kernel/lspool/pool.go
- `PoolConfig.RestartBudget` field wired through to `NewCircuitBreaker`
- `AcquireLease` and `PromoteToDirty` return `cb.CircuitOpenErr()` instead of `fmt.Errorf` wrapping

### internal/daemon/daemon.go
- `cfg.Degradation.RestartBudget` wired into `PoolConfig`
- `degrade.BudgetFor` wrapped as `BudgetFunc` closure and passed to `InstallMiddleware`

## Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Failing deadline tests | 1b7b4075 | internal/mcp/middleware_deadline_test.go |
| 1 (GREEN) | Wire deadline propagation | bb80919b | internal/mcp/middleware.go, middleware_deadline_test.go, telemetry_middleware_test.go, telemetry_span_test.go, daemon/daemon.go |
| 2 (RED) | Failing circuit breaker tests | ad7d596a | internal/kernel/lspool/circuit_test.go |
| 2 (GREEN) | Enhance circuit breaker | 237149c3 | circuit_err.go, circuit.go, circuit_test.go, pool.go, pool_test.go, metrics_test.go, daemon.go |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] BudgetFunc type instead of *config.DegradationConfig parameter**
- **Found during:** Task 1 (TDD GREEN)
- **Issue:** Importing `internal/config` from `internal/mcp/middleware.go` creates an import cycle: mcp -> config -> profile -> mcp (config/loader.go imports profile, profile/skill.go imports mcp)
- **Fix:** Introduced `BudgetFunc` type (`func(toolName string) time.Duration`) in middleware.go. daemon.go creates the closure wrapping `degrade.BudgetFor` with the config. This preserves the same behavior without the circular dependency.
- **Files modified:** internal/mcp/middleware.go, internal/daemon/daemon.go
- **Commit:** bb80919b

**2. [Rule 1 - Bug] Updated existing tests for deterministic backoff assertions**
- **Found during:** Task 2 (TDD GREEN)
- **Issue:** Existing `TestCircuitBreaker_RecordFailure_ExponentialBackoff` asserted exact backoff values (1s, 2s, 4s) which are no longer deterministic with jitter. `TestCircuitBreaker_NeverFullyBreaks` used restart budget 3 with 10 failures, hitting permanent open.
- **Fix:** Changed assertions to range-based (>= base, <= maxBackoff). Updated budget parameters for tests that need more headroom.
- **Files modified:** internal/kernel/lspool/pool_test.go
- **Commit:** 237149c3

## Threat Surface

T-13-03 mitigated: `context.WithTimeout` injected per tool class in TelemetryMiddleware. Slow LS cannot hang the server beyond the class budget.

T-13-04 mitigated: Restart budget (default 3) prevents CPU burn from restart loops. After budget exhaustion, `CanAttempt()` returns false permanently until `RecordSuccess` resets.

T-13-05 mitigated: Single-probe via `atomic.Bool.CompareAndSwap` admits exactly one request in half-open state, preventing thundering herd on recovery.

T-13-06 accepted: CircuitOpenError includes language name and retry time -- not sensitive, needed for client retry logic.

T-13-07 accepted: Client-imposed shorter deadline wins (context.WithTimeout takes minimum); server budget is a maximum.

## Self-Check: PASSED

All 7 files found. All 4 commits verified.
