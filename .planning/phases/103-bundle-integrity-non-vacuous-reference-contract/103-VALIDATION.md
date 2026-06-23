---
phase: 103
slug: bundle-integrity-non-vacuous-reference-contract
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-24
---

# Phase 103 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) + `github.com/stretchr/testify` (`require`/`assert`) |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./internal/cli/ -run 'InstallSkill|Bundle|Reference' -count=1` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/cli/ -run 'InstallSkill|Bundle|Reference' -count=1`
- **After every plan wave:** `go test ./... && go vet ./...`
- **Before `/gsd-verify-work`:** full suite green + `go run ./cmd/helix-refgen -check` exit 0 + `go test ./test/oracle/adopt/... -count=1` green.
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 103-bundle-closedset | 01 | 1 | BUNDLE-01 | install writes EXACTLY {SKILL.md, reference.md} | unit | `go test ./internal/cli/ -run TestInstallSkillInstallsExactlyBundle -count=1` | ❌ W0 | ⬜ pending |
| 103-uninstall-symmetry | 01 | 1 | BUNDLE-01 | uninstall filtered identically; idempotent | unit | `go test ./internal/cli/ -run 'TestInstallSkillIdempotent|Uninstall' -count=1` | ✅ extend | ⬜ pending |
| 103-issue-moved | 01 | 1 | BUNDLE-01 | SKILL-ISSUE.md no longer embedded/installed | unit | `go test ./internal/cli/ -run TestInstallSkillInstallsExactlyBundle -count=1` + `go build ./cmd/helix` | ❌ W0 | ⬜ pending |
| 103-atomicity | 01 | 1 | BUNDLE-01 | atomicity + withinSkillRoot containment preserved | unit | `go test ./internal/cli/ -run 'NoTempLeftover|Containment' -count=1` | ✅ keep green | ⬜ pending |
| 103-contract-count | 01 | 1 | BUNDLE-02 | exact count == len(VerbToolNames())==50 | unit | `go test ./internal/cli/ -run TestReferenceCoversEveryVerb -count=1` | ✅ exists | ⬜ pending |
| 103-contract-discrim | 01 | 1 | BUNDLE-02 | RED on a known-absent verb (revert + fabricated-absent) | unit (anti-vacuity) | `go test ./internal/cli/ -run 'TestReferenceCompletenessRevertFails' -count=1` | ✅/❌ W0 | ⬜ pending |
| 103-repro | 01 | 1 | invariant | reference.md byte-reproducible | gate | `go run ./cmd/helix-refgen -check` (exit 0) | ✅ green | ⬜ pending |
| 103-adopt-anchor | 01 | 1 | invariant | StripDecisionMatrix / adopt scorecard unaffected | unit | `go test ./test/oracle/adopt/... -count=1` | ✅ green | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cli/skill_bundle_test.go` (new) OR extend `internal/cli/skill_test.go` — closed-set `TestInstallSkillInstallsExactlyBundle` for BUNDLE-01 (RED-first: write before the allowlist + before moving SKILL-ISSUE.md so it fails on the 3-file install).
- [ ] `internal/cli/reference_contract_test.go` — confirm-and-seal BUNDLE-02 (exact count already asserts 50; revert-and-fail proof exists at lines 82-104); add an explicit fabricated-absent-verb discriminator only if the existing real-verb-drop proof is judged insufficient (RED-first).
- [ ] No framework install needed — `testing` + `testify` already present.

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
