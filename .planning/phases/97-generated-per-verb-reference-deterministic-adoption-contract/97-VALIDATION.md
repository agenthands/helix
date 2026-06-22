---
phase: 97
slug: generated-per-verb-reference-deterministic-adoption-contract
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-22
---

# Phase 97 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib testing) |
| **Config file** | none — Go toolchain; gates wired via Makefile |
| **Quick run command** | `go test ./internal/cli/... ./cmd/helix-refgen/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~60–120 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/cli/... ./cmd/helix-refgen/...`
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite + `helix-refgen --check` + `make verify-reference` (or equivalent) green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner fills) | — | — | REF-01/02/03, ADOPT-01 | — | N/A | unit | `go test ./internal/cli/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/helix-refgen/main_test.go` — generator + `--check` drift-gate tests (REF-01, REF-03)
- [ ] `internal/cli/skill_test.go` additions — `embed.FS` bundle install + SKILL-04 idle-cost still on SKILL.md only (REF-02)
- [ ] `internal/cli/nudge_test.go` / adoption-contract test — completeness sourced from `VerbToolNames()` + per-shape nudge golden keyed on the specific verb (ADOPT-01)
- [ ] Anti-vacuity revert-and-fail tests — delete-a-verb→RED, break-a-mapping→RED

*Existing infrastructure (go test, the `runNudgeCapture`/`parseAdvisory` harness, the `TestHelixSymbolicTools_NoDrift` drift-gate pattern) covers most phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (none) | — | — | All phase behaviors have automated verification. |

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
