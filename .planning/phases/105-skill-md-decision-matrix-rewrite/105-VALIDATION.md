---
phase: 105
slug: skill-md-decision-matrix-rewrite
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-24
---

# Phase 105 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./internal/cli/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~60–120 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/cli/...` (the SKILL.md drift + anti-vacuity guards)
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite green; `helix-refgen --check` still byte-clean (SKILL.md is hand-authored but lives in the bundle dir — confirm no generator regression)
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 105-01-01 | 01 | 0 | SKILL-01/02/03 | — | N/A | unit (RED) | `go test ./internal/cli/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky — refined by the planner.*

---

## Wave 0 Requirements

- [ ] Extend the existing `TestSkillVerbMembershipDrift` cross-check and ADD break-the-invariant guards in the SKILL.md test file: a fabricated decision-matrix row that (a) mixes a QUERY and an ACTION verb, or (b) carries a `—` "Not this" placeholder, must be REJECTED (assert-RED). Plus: assert every indexed-graph verb row carries the `index-semantic-graph` prerequisite note, and assert the `## Decision matrix` `StripDecisionMatrix` anchor + SKILL-04 description size cap hold.

*Refined by the planner against the RESEARCH.md Validation Architecture section.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification (SKILL.md↔VerbToolNames cross-check, anchor preservation, size cap, QUERY/ACTION-split + no-`—`-placeholder guards).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
