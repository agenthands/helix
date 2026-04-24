---
phase: 47
slug: bug-rust-analyzer-rename
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-24
---

# Phase 47 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Detailed architecture: see `47-RESEARCH.md` §Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none |
| **Quick run command** | `go test ./internal/kernel/edit/... ./internal/kernel/lspool/... -run 'Rename|Quirk' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~45 seconds (full), ~3 seconds (quick) |

---

## Sampling Rate

- **After every task commit:** Run quick run command
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** `go vet ./...` + full suite must be green
- **Max feedback latency:** 45 seconds

---

## Per-Task Verification Map

Populated by planner — one row per task. Minimum coverage:

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 47-01-XX | 01 | 1 | BUG-02 | — | RCA capture (wire trace) documented | doc | `test -s .planning/phases/47-bug-rust-analyzer-rename/47-RCA.md` | ❌ W0 | ⬜ pending |
| 47-02-XX | 02 | 2 | BUG-02 | — | `RenameOverrider` interface + `RustAnalyzerAdapter.RenameOverride` | unit | `go test ./internal/kernel/edit/... -run 'TestRenameOverrider' -count=1` | ❌ W0 | ⬜ pending |
| 47-02-XX | 02 | 2 | BUG-02 | — | Native-rename path still reachable when override declines | unit | `go test ./internal/kernel/edit/... -run 'TestRenameDispatch' -count=1` | ❌ W0 | ⬜ pending |
| 47-03-XX | 03 | 3 | BUG-02 | — | Rust rename integration (unskipped) with `strategy` assertion | integration | `go test ./test/integration/... -run 'TestEdit_RustFixture/rename' -count=1` | ✅ (unskip) | ⬜ pending |
| 47-03-XX | 03 | 3 | BUG-02 | — | Hover / references / find_implementations unchanged | integration | `go test ./test/integration/... -run 'TestSymbols_RustFixture' -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/kernel/edit/rename_override_test.go` — unit tests for `RenameOverrider` dispatch
- [ ] `internal/kernel/lspool/quirks_test.go` (extend) — `RustAnalyzerAdapter` implements `RenameOverride` (structural check)
- [ ] `.planning/phases/47-bug-rust-analyzer-rename/47-RCA.md` — wire-trace capture artifact (D-02)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| USAGE.md Troubleshooting entry exists and points to D-05 limits | BUG-02 SC#2 | Human-readable doc review | Read USAGE.md §Troubleshooting, confirm Rust rename entry references `rust-client-side` strategy and BUG-DEFER-02 |
| Metric counter emits on both strategy paths | BUG-02 | Requires running daemon + tool invocation | Start daemon, run rename twice (one native path, one override), inspect metrics endpoint |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 45s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
