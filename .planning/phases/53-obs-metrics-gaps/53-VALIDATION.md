---
phase: 53
slug: obs-metrics-gaps
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-26
---

# Phase 53 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — uses standard `go test` |
| **Quick run command** | `go test ./internal/obs/... ./internal/kernel/lspool/... ./internal/repomap/... ./internal/kernel/edit/... ./internal/daemon/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~30s quick / ~90s full |

---

## Sampling Rate

- **After every task commit:** Run quick command for the touched package(s)
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 53-01-01 | 01 | 1 | OBS-03 | — | N/A | unit | `go test ./internal/obs/ -run TestMetricsRegistration` | ❌ W0 | ⬜ pending |
| 53-01-02 | 01 | 1 | OBS-03 | — | Bounded label cardinality (no unbounded series) | unit | `go test ./internal/obs/ -run TestCardinalityBounded` | ❌ W0 | ⬜ pending |
| 53-01-03 | 01 | 1 | OBS-03 | — | Zero-alloc when noop | unit | `go test ./internal/obs/ -run TestNoopZeroAlloc -benchmem` | ❌ W0 | ⬜ pending |
| 53-02-01 | 02 | 2 | OBS-03 | — | N/A | unit | `go test ./internal/kernel/lspool/ -run TestCacheHitMissEmission` | ❌ W0 | ⬜ pending |
| 53-02-02 | 02 | 2 | OBS-03 | — | N/A | unit | `go test ./internal/repomap/ -run TestRepoMapMetricsEmission` | ❌ W0 | ⬜ pending |
| 53-02-03 | 02 | 2 | OBS-03 | — | N/A | unit | `go test ./internal/kernel/ -run TestSessionLifecycleEmission` | ❌ W0 | ⬜ pending |
| 53-02-04 | 02 | 2 | OBS-03 | — | N/A | unit | `go test ./internal/kernel/edit/ -run TestEditOutcomeEmission` | ❌ W0 | ⬜ pending |
| 53-03-01 | 03 | 3 | OBS-03 | — | N/A | unit | `go test ./internal/daemon/ -run TestWiringAssertions` | ✅ | ⬜ pending |
| 53-03-02 | 03 | 3 | OBS-03 | — | N/A | manual | grep `USAGE.md` for new metric names | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/obs/metrics_test.go` — extend with cardinality and zero-alloc benchmarks for new series
- [ ] `internal/kernel/lspool/metrics_test.go` — emission tests for hit/miss branches
- [ ] `internal/repomap/metrics_test.go` — new file, cache hit + extract duration emission
- [ ] `internal/kernel/edit/metrics_test.go` — new file, outcome enum emission across all edit tools
- [ ] `internal/kernel/session_lifecycle_test.go` — new file (or extend existing) for activate/deactivate/timeout/shutdown phases

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `USAGE.md` Observability section documents each new metric | OBS-03 SC#3 | Documentation is human-readable text, not code under test | Read USAGE.md Observability section; verify each of the 5 new series is named with its labels and a one-line semantic |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
