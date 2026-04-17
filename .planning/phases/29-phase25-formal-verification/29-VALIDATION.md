---
phase: 29
slug: phase25-formal-verification
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-17
---

# Phase 29 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (built-in) |
| **Config file** | go.mod |
| **Quick run command** | `go test ./internal/fuzzy/... -count=1` |
| **Full suite command** | `go test ./internal/fuzzy/... ./internal/kernel/edit/... -count=1 -run "Fuzzy\|fuzzy"` |
| **Estimated runtime** | ~1 second |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/fuzzy/... -count=1`
- **After every plan wave:** Run `go test ./internal/fuzzy/... ./internal/kernel/edit/... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 1 second

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 29-01-01 | 01 | 1 | FUZZ-01 | T-29-01 | N/A (docs only) | integration | `go test ./internal/fuzzy/... -run "Sweep\|Cascade" -v` | ✅ | ✅ green |
| 29-01-01 | 01 | 1 | FUZZ-02 | T-29-01 | N/A (docs only) | integration | `go test ./internal/fuzzy/... -run "StrategyReporting\|ScoreTier" -v` | ✅ | ✅ green |
| 29-01-01 | 01 | 1 | FUZZ-03 | T-29-01 | N/A (docs only) | integration | `go test ./internal/fuzzy/... -run "Reflow" -v` | ✅ | ✅ green |
| 29-01-01 | 01 | 1 | FUZZ-07 | T-29-01 | N/A (docs only) | integration | `go test ./internal/fuzzy/... -run "Ambiguity" -v` | ✅ | ✅ green |
| 29-01-01 | 01 | 1 | FUZZ-08 | T-29-01 | N/A (docs only) | integration | `go test ./internal/fuzzy/... -run "Ellipsis" -v` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. 45 tests across `internal/fuzzy/` (40) and `internal/kernel/edit/` (5) provide full automated coverage.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 1s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-17
