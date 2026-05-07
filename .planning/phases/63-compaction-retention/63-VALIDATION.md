---
phase: 63
slug: compaction-retention
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-07
---

# Phase 63 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.22+) |
| **Config file** | none — Go's built-in test discovery |
| **Quick run command** | `go test ./internal/semantic/store/... ./internal/semantic/compact/...` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~60 seconds (quick) / ~5 minutes (full) |

---

## Sampling Rate

- **After every task commit:** Run quick run command (package-scoped)
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

> Populated by the planner during plan generation. Each task in PLAN.md gets a row mapping
> task_id → requirement → test type → automated command. The planner derives these from
> the acceptance criteria in CONTEXT.md and RESEARCH.md.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | 01 | 1 | COMPACT-01..05 | — | TBD | TBD | TBD | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

> Populated by planner. Expected new test files include:
- [ ] `internal/semantic/store/snapshot_test.go` — unit tests for Begin/Write/Commit/Abort
- [ ] `internal/semantic/compact/compactor_test.go` — compactor goroutine tests
- [ ] `internal/semantic/compact/gate_test.go` — `BlockedReason` per-condition unit tests
- [ ] `internal/semantic/compact/compactor_kill_test.go` — kill-mid-compact subprocess fixture (COMPACT-05)
- [ ] `internal/semantic/compact/cas_property_test.go` — interleave-overlay-writes property test (COMPACT-02)
- [ ] `internal/semantic/compact/longrepo_bench_test.go` — long-repo growth bound bench (COMPACT-03)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (none expected) | — | — | — |

*All phase behaviors targeted for automated verification — kill-mid-compact uses subprocess fixture; long-repo bench is `go test -bench=Long`.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
