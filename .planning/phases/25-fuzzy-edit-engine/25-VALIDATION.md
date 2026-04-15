---
phase: 25
slug: fuzzy-edit-engine
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-15
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Sourced from `25-RESEARCH.md` §Validation Architecture. Refine line items
> as plans are written; every task must map to a row in §Per-Task Verification Map.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `github.com/stretchr/testify` (assert + require) |
| **Config file** | none — standard `go test` discovery |
| **Quick run command** | `go test ./internal/fuzzy/...` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~5s package, ~30s full suite |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/fuzzy/... && go vet ./internal/fuzzy/...`
- **After every plan wave:** `go test ./internal/fuzzy/... -race && gofmt -l internal/fuzzy/`
- **Before `/gsd-verify-work`:** `go test ./... && go vet ./... && gofmt -l .` full-repo green
- **Max feedback latency:** ~5 seconds (package-scoped)

---

## Per-Task Verification Map

> Populated by gsd-planner. Every task in `25-*-PLAN.md` MUST have a row here
> with file existence updated to ✅ once the test file is created in Wave 0.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | 0 | FUZZ-01 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_ExactStrategy` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-01 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_WhitespaceStrategy` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-01 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_IndentationStrategy` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-01 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_FailWithDiff` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-01 | — | N/A | unit | `go test ./internal/fuzzy -run TestCascade_StopsOnAmbiguity` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-02 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_StrategyReporting` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-02 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_ScoreTiers` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-03 | — | N/A | unit | `go test ./internal/fuzzy -run TestReflow_AppliesSourcePrefix` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-03 | — | N/A | unit | `go test ./internal/fuzzy -run TestReflow_PreservesEmptyLines` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-03 | — | N/A | unit | `go test ./internal/fuzzy -run TestReflow_TabsAndSpaces` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-07 | — | refuses ambiguous edit | unit | `go test ./internal/fuzzy -run TestAmbiguity_Exact` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-07 | — | structured detail | unit | `go test ./internal/fuzzy -run TestAmbiguity_LineNumberCap` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-07 | — | structured detail | unit | `go test ./internal/fuzzy -run TestAmbiguity_OverflowSuffix` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-07 | — | refuses ambiguous edit | unit | `go test ./internal/fuzzy -run TestAmbiguity_DoesNotCascade` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_TwoSegments` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_MultipleSegments` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_LeadingWhitespace` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_InlineLiteral` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-08 | — | rejects malformed input | unit | `go test ./internal/fuzzy -run TestEllipsis_EmptySegmentRejected` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-08 | — | rejects malformed input | unit | `go test ./internal/fuzzy -run TestEllipsis_ReplacementSegmentMismatch` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_InOrderMatching` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/fuzzy/fuzzy_test.go` — top-level Match() integration tests for FUZZ-01, FUZZ-02
- [ ] `internal/fuzzy/strategies_test.go` — per-strategy units + FUZZ-07 ambiguity + cascade-stop
- [ ] `internal/fuzzy/ellipsis_test.go` — FUZZ-08 segment edge cases
- [ ] `internal/fuzzy/indent_test.go` — FUZZ-03 reflow (tabs/spaces/mixed/empty-line preservation)
- [ ] `internal/fuzzy/diff_test.go` — golden-style assertions on `formatFailureDiff` output
- [ ] No framework install — `testify` already present in `go.mod`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|

*All Phase 25 behaviors have automated verification — engine is pure with no I/O or UI.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
