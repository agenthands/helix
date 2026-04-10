---
phase: 15
slug: benchmark-gate-hardening
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-10
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + benchgate CLI |
| **Config file** | `test/bench/cmd/benchgate/main.go` |
| **Quick run command** | `go build ./test/bench/cmd/benchgate/` |
| **Full suite command** | `go test -bench=. -count=3 -short ./test/bench/...` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go build ./test/bench/cmd/benchgate/`
- **After every plan wave:** Run `go test -bench=. -count=3 -short ./test/bench/...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 1 | BENCH-05 | — | N/A | integration | `grep -v 'warn-only' .github/workflows/bench.yml` | ✅ | ⬜ pending |
| 15-01-02 | 01 | 1 | BENCH-06 | — | N/A | integration | `grep -v 'PLACEHOLDER' test/bench/baselines/v1.1-github-hosted.txt` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CI gate blocks regressing PRs | BENCH-05 | Requires GitHub Actions environment | Push a test PR after baseline committed; verify gate runs without --warn-only |
| Baseline numbers are real CI data | BENCH-06 | Requires capture workflow run on ubuntu-latest | Trigger capture-baseline.yml and verify output format matches benchfmt |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
