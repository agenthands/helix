---
phase: 13
slug: graceful-degradation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-10
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test infrastructure |
| **Quick run command** | `go test ./internal/degrade/... ./internal/kernel/lspool/... -count=1 -short` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/degrade/... ./internal/kernel/lspool/... -count=1 -short`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | DEGRADE-01 | — | Timeout returns structured error, not hang | unit | `go test ./internal/degrade/... -run TestToolClassBudget` | ❌ W0 | ⬜ pending |
| 13-01-02 | 01 | 1 | DEGRADE-02 | — | Middleware enforces deadline per tool class | unit | `go test ./internal/kernel/middleware/... -run TestDeadlineEnforcement` | ❌ W0 | ⬜ pending |
| 13-02-01 | 02 | 1 | DEGRADE-03 | — | ErrCircuitOpen is typed with structured fields | unit | `go test ./internal/kernel/lspool/... -run TestCircuitOpenError` | ❌ W0 | ⬜ pending |
| 13-02-02 | 02 | 1 | DEGRADE-04 | — | Half-open admits exactly one probe | unit | `go test ./internal/kernel/lspool/... -run TestHalfOpenSingleProbe` | ❌ W0 | ⬜ pending |
| 13-02-03 | 02 | 1 | DEGRADE-05 | — | Decorrelated jitter backoff replaces exponential | unit | `go test ./internal/kernel/lspool/... -run TestDecorrelatedJitter` | ❌ W0 | ⬜ pending |
| 13-03-01 | 03 | 2 | DEGRADE-06 | — | Crash restart within budget, repeated crashes trip circuit | integration | `go test ./internal/kernel/lspool/... -run TestCrashRestartBudget` | ❌ W0 | ⬜ pending |
| 13-04-01 | 04 | 2 | DEGRADE-07 | — | SIGTERM drains in-flight, flushes telemetry, exits clean | integration | `go test ./internal/daemon/... -run TestGracefulShutdown` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/degrade/degrade_test.go` — stubs for DEGRADE-01 tool class budgets
- [ ] `internal/kernel/lspool/circuit_test.go` — stubs for DEGRADE-03, DEGRADE-04, DEGRADE-05
- [ ] `internal/daemon/shutdown_test.go` — stubs for DEGRADE-07 graceful shutdown

*Existing test infrastructure covers Go test framework — no new framework install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Deadline observable in Jaeger spans | DEGRADE-01 | Requires running Jaeger collector | Start Jaeger, trigger timeout, verify span shows deadline propagation across forwarder→daemon→kernel→LS |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
