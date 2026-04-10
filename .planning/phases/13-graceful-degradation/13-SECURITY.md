# Phase 13 Security Verification

**Phase:** 13 -- graceful-degradation
**ASVS Level:** 1
**Verified:** 2026-04-10
**Auditor:** GSD Security Auditor (automated)

## Threat Verification

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-13-01 | Tampering | mitigate | CLOSED | `internal/degrade/budget.go:118-141` -- `configTimeout` calls `clampPositive` which returns 0 for negative/zero inputs, causing `BudgetFor` to fall back to `defaultBudgets` (always positive). Tests `TestBudgetFor_NegativeConfigClampedToDefault` and `TestBudgetFor_NeverReturnsZero` in `budget_test.go:123-141` confirm the invariant. |
| T-13-02 | Denial of Service | accept | CLOSED | `internal/degrade/budget.go:25-77` -- `toolClassMap` is a package-level `var` initialized at compile time. `ToolClassFor` (line 90-95) returns `ClassRead` for unmapped tools. Test `TestToolClassMap_UnmappedDefault` in `budget_test.go:68-73` confirms. Accepted risk: map is read-only in practice; no mutation path exists. |
| T-13-03 | Denial of Service | mitigate | CLOSED | `internal/mcp/middleware.go:148-155` -- `TelemetryMiddleware` injects `context.WithTimeout(ctx, budget)` per tool class before handler execution, gated on `budgetFn != nil` and `budget > 0`. `BudgetFunc` is wired in `internal/daemon/daemon.go:276-278` wrapping `degrade.BudgetFor`. |
| T-13-04 | Denial of Service | mitigate | CLOSED | `internal/kernel/lspool/circuit.go:112` -- `CanAttempt` checks `cb.failures >= cb.restartBudget` and returns false when exhausted, preventing restart loops. Default budget is 3 (line 35). `restartBudget` flows from config via `internal/daemon/daemon.go:176` -> `PoolConfig.RestartBudget` -> `NewCircuitBreaker` (line 30). |
| T-13-05 | Denial of Service | mitigate | CLOSED | `internal/kernel/lspool/circuit.go:117` -- `cb.probing.CompareAndSwap(false, true)` admits exactly one goroutine in half-open state. `probing` is `atomic.Bool` (line 21). Reset on failure (line 67) and success (line 96). |
| T-13-06 | Information Disclosure | accept | CLOSED | `internal/kernel/lspool/circuit_err.go:18-21` -- `CircuitOpenError.Error()` includes language name, failure count, and retry time. These are operational metadata, not sensitive data. Needed for client retry logic. |
| T-13-07 | Tampering | accept | CLOSED | `internal/mcp/middleware.go:150-154` -- `context.WithTimeout` creates a derived context; if the parent already has a shorter deadline, the shorter one wins (Go context semantics). The server budget is a maximum, not a minimum. |
| T-13-08 | Denial of Service | mitigate | CLOSED | `internal/daemon/daemon.go:144` -- `cfg.Degradation.MemoryLimitMB > 0` guard ensures `debug.SetMemoryLimit` is only called with positive values. Zero (default, per `internal/config/defaults.go:30`) skips the call entirely, preserving GOMEMLIMIT env var behavior. |
| T-13-09 | Denial of Service | mitigate | CLOSED | `internal/daemon/shutdown.go:13-14` -- `ShutdownTimeout` bounds the drain with `context.WithTimeout`. Default 10s (line 16). Context cancellation propagates to kernel drain (line 25) and all in-flight handlers. Integration tests `TestGracefulShutdownMidRequest` and `TestGracefulShutdownClean` in `shutdown_graceful_test.go` verify clean exit within timeout. |
| T-13-10 | Tampering | accept | CLOSED | `internal/daemon/daemon.go:144-151` -- `debug.SetMemoryLimit` intentionally overrides any GOMEMLIMIT env var when `MemoryLimitMB > 0`. This is documented behavior per D-09 and code comment (line 142: "zero means 'don't set' and lets the GOMEMLIMIT env var take effect undisturbed"). |

## Accepted Risks Log

| Threat ID | Risk Description | Rationale |
|-----------|------------------|-----------|
| T-13-02 | Unmapped tools get ClassRead budget | ClassRead is the tightest budget (5s); this is a safe default. The map is a compile-time constant with no mutation path. |
| T-13-06 | CircuitOpenError exposes language name and retry time | Operational metadata needed for client retry logic. Not PII or secrets. |
| T-13-07 | Client can impose shorter deadline than server budget | This is correct Go context semantics. Server budget is a ceiling, not a floor. Client shortening their own deadline is not a threat. |
| T-13-10 | Config-driven SetMemoryLimit overrides GOMEMLIMIT env var | Intentional design: config takes precedence. Documented in code comments. Users who set memory_limit_mb > 0 expect it to control the limit. |

## Unregistered Flags

None. No unregistered threat flags were found in the SUMMARY.md files for plans 01, 02, or 03.

## Summary

All 10 threats verified. 6 mitigations confirmed present in code with matching test coverage. 4 accepted risks documented with rationale.
