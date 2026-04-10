---
phase: 13
slug: graceful-degradation
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-10
validated: 2026-04-10
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
| 13-01-01 | 01 | 1 | DEGRADE-01 | T-13-01, T-13-02 | Timeout returns structured error, not hang | unit | `go test ./internal/degrade/... -run TestToolClassMap -count=1 -v && go test ./internal/degrade/... -run TestBudgetFor -count=1 -v` | ✅ internal/degrade/budget_test.go | ✅ green |
| 13-01-02 | 01 | 1 | DEGRADE-02 | T-13-03 | Middleware enforces deadline per tool class | unit | `go test ./internal/mcp/... -run TestDeadlinePropagation -count=1 -v` | ✅ internal/mcp/middleware_deadline_test.go | ✅ green |
| 13-02-01 | 02 | 2 | DEGRADE-03 | T-13-06 | ErrCircuitOpen is typed with structured fields | unit | `go test ./internal/kernel/lspool/... -run TestCircuitOpenError -count=1 -v` | ✅ internal/kernel/lspool/circuit_test.go | ✅ green |
| 13-02-02 | 02 | 2 | DEGRADE-04 | T-13-05 | Half-open admits exactly one probe | unit | `go test ./internal/kernel/lspool/... -run TestSingleProbeHalfOpen -count=1 -v` | ✅ internal/kernel/lspool/circuit_test.go | ✅ green |
| 13-02-03 | 02 | 2 | DEGRADE-05 | — | Decorrelated jitter backoff replaces exponential | unit | `go test ./internal/kernel/lspool/... -run TestDecorrelatedJitter -count=1 -v` | ✅ internal/kernel/lspool/circuit_test.go | ✅ green |
| 13-03-01 | 03 | 3 | DEGRADE-06 | T-13-04 | Crash restart within budget, repeated crashes trip circuit | unit | `go test ./internal/kernel/lspool/... -run TestRestartBudget -count=1 -v` | ✅ internal/kernel/lspool/circuit_test.go | ✅ green |
| 13-03-02 | 03 | 3 | DEGRADE-07 | T-13-09 | SIGTERM drains in-flight, flushes telemetry, exits clean | integration | `go test ./internal/daemon/... -run TestGracefulShutdown -count=1 -v -timeout=30s` | ✅ internal/daemon/shutdown_graceful_test.go | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/degrade/budget_test.go` — 10 tests for tool class map and budget lookup (DEGRADE-01)
- [x] `internal/kernel/lspool/circuit_test.go` — 7 tests for typed error, jitter, restart budget, probe (DEGRADE-03/04/05/06)
- [x] `internal/mcp/middleware_deadline_test.go` — 5 tests for deadline propagation (DEGRADE-02)
- [x] `internal/daemon/shutdown_graceful_test.go` — 2 tests for graceful shutdown (DEGRADE-07)

*Existing test infrastructure covers Go test framework — no new framework install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Deadline observable in Jaeger spans | DEGRADE-01 | Requires running Jaeger collector | Start Jaeger, trigger timeout, verify span shows deadline propagation across forwarder→daemon→kernel→LS |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved

---

## Validation Audit 2026-04-10

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Total tests | 24 |
| Tests passing | 24 |
