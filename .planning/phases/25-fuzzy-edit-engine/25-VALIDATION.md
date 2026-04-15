---
phase: 25
slug: fuzzy-edit-engine
status: ready
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-15
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Sourced from `25-RESEARCH.md` §Validation Architecture. Tests are co-located
> with implementation via inline TDD (RED step in the same task that ships GREEN),
> so there is no separate Wave 0 plan — every test file is created inside its
> owning plan's first task.

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

> Every row maps to a concrete plan + task. Tests are written inside the
> same task that produces the implementation (TDD RED→GREEN inline), so
> File Exists is marked `✅ inline` rather than needing a separate Wave 0.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 25-06.1 | 25-06 | 3 | FUZZ-01 | T-25-06-01 | N/A | unit | `go test ./internal/fuzzy -run TestMatch_ExactStrategy` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-01 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_WhitespaceStrategy` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-01 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_IndentationStrategy` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-01 | T-25-06-03 | surfaces nearest-window diff | unit | `go test ./internal/fuzzy -run TestMatch_FailWithDiff` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-01 | — | halts cascade on ambiguity | unit | `go test ./internal/fuzzy -run TestCascade_StopsOnAmbiguity` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-02 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_StrategyReporting` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-02 | — | N/A | unit | `go test ./internal/fuzzy -run TestMatch_ScoreTiers` | ✅ inline | ⬜ pending |
| 25-04.1 | 25-04 | 2 | FUZZ-03 | T-25-04-01 | N/A | unit | `go test ./internal/fuzzy -run TestReflow_AppliesSourcePrefix` | ✅ inline | ⬜ pending |
| 25-04.1 | 25-04 | 2 | FUZZ-03 | T-25-04-02 | no trailing ws on empty lines | unit | `go test ./internal/fuzzy -run TestReflow_PreservesEmptyLines` | ✅ inline | ⬜ pending |
| 25-04.1 | 25-04 | 2 | FUZZ-03 | — | N/A | unit | `go test ./internal/fuzzy -run TestReflow_TabsAndSpaces` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-07 | — | refuses ambiguous edit | unit | `go test ./internal/fuzzy -run TestAmbiguity_Exact` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-07 | — | structured detail | unit | `go test ./internal/fuzzy -run TestAmbiguity_LineNumberCap` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-07 | — | structured detail | unit | `go test ./internal/fuzzy -run TestAmbiguity_OverflowSuffix` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-07 | — | refuses ambiguous edit | unit | `go test ./internal/fuzzy -run TestAmbiguity_DoesNotCascade` | ✅ inline | ⬜ pending |
| 25-05.1 | 25-05 | 2 | FUZZ-07 | T-25-05-03 | 5-hit DoS cap | unit | `go test ./internal/fuzzy -run TestFormatAmbiguity` | ✅ inline | ⬜ pending |
| 25-05.1 | 25-05 | 2 | FUZZ-01 | T-25-05-01 | diff payload shape | unit | `go test ./internal/fuzzy -run TestFormatFailureDiff` | ✅ inline | ⬜ pending |
| 25-05.1 | 25-05 | 2 | FUZZ-01 | T-25-05-02 | nearest-window bounds | unit | `go test ./internal/fuzzy -run TestFindNearestWindow_PicksBestMatchCount` | ✅ inline | ⬜ pending |
| 25-02.1 | 25-02 | 1 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_TwoSegments` | ✅ inline | ⬜ pending |
| 25-02.1 | 25-02 | 1 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_MultipleSegments` | ✅ inline | ⬜ pending |
| 25-02.1 | 25-02 | 1 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_LeadingWhitespace` | ✅ inline | ⬜ pending |
| 25-02.1 | 25-02 | 1 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_InlineLiteral` | ✅ inline | ⬜ pending |
| 25-02.1 | 25-02 | 1 | FUZZ-08 | — | rejects malformed input | unit | `go test ./internal/fuzzy -run TestEllipsis_EmptySegmentRejected` | ✅ inline | ⬜ pending |
| 25-02.1 | 25-02 | 1 | FUZZ-08 | — | rejects malformed input | unit | `go test ./internal/fuzzy -run TestEllipsis_ReplacementSegmentMismatch` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-08 | — | N/A | unit | `go test ./internal/fuzzy -run TestEllipsis_InOrderMatching` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-02 | — | tier aggregation across segments | unit | `go test ./internal/fuzzy -run TestEllipsis_MixedTierAggregationReportsWeakest` | ✅ inline | ⬜ pending |
| 25-06.1 | 25-06 | 3 | FUZZ-08 | T-25-06-01 | no trailing-newline byte drift | unit | `go test ./internal/fuzzy -run TestEllipsis_ByteOffsetsWithTrailingNewline` | ✅ inline | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Phase 25 uses **inline TDD** — each plan's first task writes its own test file as the
RED step of the red→green cycle. There is no separate Wave 0 plan. The test files
below are produced inside the listed owning plan and are ready to run as soon as
that plan completes.

- [x] `internal/fuzzy/ellipsis_test.go` — owned by Plan 25-02, task 1 (FUZZ-08 edge cases)
- [x] `internal/fuzzy/strategies_test.go` — owned by Plan 25-03, task 1 (per-strategy sweeps)
- [x] `internal/fuzzy/indent_test.go` — owned by Plan 25-04, task 1 (FUZZ-03 reflow)
- [x] `internal/fuzzy/diff_test.go` — owned by Plan 25-05, task 1 (golden-string formatters)
- [x] `internal/fuzzy/fuzzy_test.go` — owned by Plan 25-06, task 1 (top-level Match integration)
- [x] No framework install — `testify` already present in `go.mod`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|

*All Phase 25 behaviors have automated verification — engine is pure with no I/O or UI.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (inline-TDD: owning plan provides the RED file)
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready
