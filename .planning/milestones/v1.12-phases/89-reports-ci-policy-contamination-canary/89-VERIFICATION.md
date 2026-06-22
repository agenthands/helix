---
phase: 89-reports-ci-policy-contamination-canary
verified: 2026-06-21T13:05:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
human_verification:
  - test: "Open a real PR and confirm the bench-quick CI job runs make bench-quick within the 5-minute hard cap with no LLM cost; trigger the bench-full job via workflow_dispatch as a maintainer"
    expected: "bench-quick completes ≤5 min on PR (no provider secret consumed); bench-full only runs on schedule/workflow_dispatch, never on pull_request"
    why_human: "INFRA-04 LIVE CI execution is inspection-gated by design (89-VALIDATION.md Manual-Only table); the YAML STRUCTURE is hermetically proven locally but a real GitHub Actions run cannot be exercised from the verifier sandbox"
---

# Phase 89: Reports, CI Policy & Contamination Canary — Verification Report

**Phase Goal:** The v1.12 publication CAPSTONE — byte-reproducible `helix-bench report --run-id` regenerating all 4 reports; a CI cost-policy split; the real contamination canary (footnote exclusion from headline numbers).
**Verified:** 2026-06-21T13:05:00Z
**Status:** passed (with one inspection-gated INFRA-04 live-CI item routed to human)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | **SC#1 (REPORT-01/02/04):** `report --run-id` regenerates 4 reports; leaderboard shows (mode×benchmark)→pass@1, verified_correctness, cost_per_solved with BCa CIs + non-overlap markers; per_language lists no-coverage langs as `n/a` (8-lang derived set) | ✓ VERIFIED | `report.go` newReportCmd wired (not a stub); leaderboard.golden.md has `verified_correctness`+`cost_per_solved` columns with `[lo, hi]` BCa CIs + em-dash null discipline; per_language.golden.md lists all 8 Tier-1 langs (cpp,csharp,go,java,javascript,python,rust,typescript — matches `bench/languages/` dirs, no `c`) with `n/a` for 5 no-coverage langs; cost_quality scatter x-range (0.0045..0.0150) == leaderboard cost (single-sourced) |
| 2 | **SC#2 (REPORT-03/05):** ablations.md 5 delta tables (full vs no_lsp/no_semantic/no_structured_edit/baseline_plain/baseline_rag) with CI-overlap; cost_quality.md scatter + cost-table valid_until | ✓ VERIFIED | ablations.golden.md has all 5 deltas incl. aggregate-time `full_minus_no_semantic` with `ci_overlap` markers (disjoint / "CI overlap — no X>Y claim"); cost_quality.golden.md + cost_quality_scatter.golden.md render the ASCII scatter (cost vs verified_correctness) and footer cites `cost_table_valid_until: 2027-01-28` |
| 3 | **REPORT-05 LOAD-BEARING:** `report --run-id` regenerates ALL 4 reports BYTE-IDENTICALLY (shared renderAll + zero-RNG determinism) | ✓ VERIFIED | `renderAll(rep, runDir)` is the single shared path emitting all 4 reports, called by both `Aggregate` and `report`. `TestReportByteReproducible` + `TestReportByteReproducibleSameDir` PASS (double-render diff-empty + committed-golden match across all 4; hermetic — no HELIX_BIN/daemon/network). `TestReportEqualsAggregate` PASS (report==aggregate byte-equality). run-id + --out (WR-03) both validated before `filepath.Join` |
| 4 | **INFRA-05 (canary integrity — load-bearing):** a contaminated task is EXCLUDED from ALL headline numbers (pass@1 AND verified_correctness AND ablations AND per_language) + footnoted, fail-safe | ✓ VERIFIED | `cleanRows` now routed through ALL 5 published reduces: reduceLeaderRow:262, reduceVerifiedCorrectness:561 (CR-01), successVectorForMode:594 (WR-01), reduceLanguageRows:338 (WR-02), reduceCostRow:682. reduceCanaryRate (451) deliberately reads ALL rows (MEASUREMENT). **Discrimination proven by revert-and-fail** (see below). Footnote `appendContaminationFootnote` wired into renderAll; production injector `InjectCanaryIfSelected` wired into runtime `runOneCell` (matrix.go:273) |

**Score:** 4/4 truths verified

### CR-01 / WR-01 / WR-02 / WR-03 Fix Discrimination (revert-and-fail proof)

The review found the prior exclusion tests were VACUOUS (clean==dirty verdict masked the leak). I independently proved each fix is now discriminating by reverting it and re-running the test:

| Finding | Fix locus | Revert-and-fail result |
| --- | --- | --- |
| CR-01 (verified_correctness counted contaminated) | reduceVerifiedCorrectness:561 cleanRows | Reverted → `TestCanaryExclusionFromHeadline` FAILS `0.5 vs 0.667` on the verified_correctness assertion (diff 0.1667) |
| WR-01 (ablations counted contaminated) | successVectorForMode:594 cleanRows | Reverted → `TestCanaryExclusionFromAblations` FAILS `0.5 vs 0.667` on FullCI |
| WR-02 (per_language counted contaminated) | reduceLanguageRows:338 cleanRows | Reverted → `TestCanaryExclusionFromHeadline` FAILS on per_language pass_rate `0.5 vs 0.667` AND n count |
| WR-03 (`report --out` unvalidated traversal) | report.go isValidOut + RunE guard | `TestReportOutValidation` PASS (asserts the SPECIFIC invalid-`--out` error, not an incidental Aggregate fail) |

Source was restored to HEAD after each revert (`git diff --quiet bench/aggregator/aggregate.go` → CLEAN). CanaryPassRate correctly asserts 4/6=0.667 (still measures all rows). The fixes are real, not narrative.

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `bench/aggregator/aggregate.go` | reduceVerifiedCorrectness + cleanRows in all 5 reduces + renderAll | ✓ VERIFIED | All 5 published reduces route cleanRows; renderAll emits 4 reports |
| `bench/aggregator/report.go` | renderPerLanguage, renderAblations, scatter, footnote, verified/cost columns | ✓ VERIFIED | tier1Languages (8), appendContaminationFootnote, renderScatter present |
| `bench/runtime/prompt_inject.go` | deterministic InjectCanaryIfSelected (every Kth, FNV K=7) | ✓ VERIFIED | Pure FNV-1a selector; references canary.InjectPrompt only (single-source) |
| `bench/runtime/matrix.go` | production caller wiring | ✓ VERIFIED | matrix.go:273 wires injector into the claude dispatch branch of runOneCell |
| `cmd/helix-bench/report.go` | report --run-id RunE, validated, fail-closed | ✓ VERIFIED | isValidRunID + isValidOut before filepath.Join; funnels through Aggregate→renderAll |
| `bench/aggregator/byte_reproducible_test.go` | double-render + golden proof, all 4 | ✓ VERIFIED | TestReportByteReproducible + SameDir PASS |
| `.github/workflows/bench.yml` | PR bench-quick 5-min cap + gated full | ✓ VERIFIED | least-priv perms, 5-min cap, no judge ref, schedule/dispatch-gated full |
| `bench/ci_workflow_test.go` | hermetic YAML-parse proof | ✓ VERIFIED | TestBenchWorkflow PASS |
| `bench/BENCH.md` | ## CI Cost Policy + canary policy | ✓ VERIFIED | heading present (count=1) |
| Committed goldens (leaderboard/per_language/ablations/cost_quality/scatter) | byte-stable fixtures | ✓ VERIFIED | All match; full aggregator suite green |

### Key Link Verification

| From | To | Via | Status |
| --- | --- | --- | --- |
| reduceVerifiedCorrectness | cleanRows | aggregate.go:561 split before pooling | ✓ WIRED |
| successVectorForMode | cleanRows | aggregate.go:594 split, present pre-filter | ✓ WIRED |
| reduceLanguageRows | cleanRows | aggregate.go:338 split before bucketing | ✓ WIRED |
| buildScatterPoints | LeaderRow.VerifiedCorrectness | reads already-cleaned reduce (scatter inherits exclusion) | ✓ WIRED |
| InjectCanaryIfSelected | canary.InjectPrompt/Sentinel | single-source, never re-derived | ✓ WIRED |
| runOneCell | InjectCanaryIfSelected | matrix.go:273 production dispatch | ✓ WIRED |
| report RunE | aggregator.Aggregate / renderAll | filepath.Join after dual validation | ✓ WIRED |
| Aggregate + report | renderAll | one render path, two entry points | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| All phase-89 packages build+test | `go build ./...` + `go test -count=1 ./bench/aggregator/ ./cmd/helix-bench/ ./bench/runtime/ ./bench/canary/ ./bench/` | all ok | ✓ PASS |
| Byte-reproducibility (REPORT-05) | `go test ./bench/aggregator/ -run ByteReproducible` | TestReportByteReproducible + SameDir PASS | ✓ PASS |
| Canary exclusion discriminates (INFRA-05) | revert cleanRows → re-run CanaryExclusion tests | FAIL 0.5 vs 0.667 (CR-01/WR-01/WR-02) | ✓ PASS |
| Full custom-analyzer gate | `make vet` | exit 0 (incl. ablation/rag-leakage analyzers) | ✓ PASS |
| Hermetic CI YAML parse (INFRA-04) | `go test ./bench/ -run TestBenchWorkflow` | PASS | ✓ PASS |
| Injector determinism | `go test ./bench/runtime/ -run InjectCanary` | 3/3 PASS | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
| --- | --- | --- | --- |
| REPORT-01 | 89-01 | ✓ SATISFIED | leaderboard verified_correctness + cost_per_solved columns w/ BCa CIs |
| REPORT-02 | 89-01 | ✓ SATISFIED | per_language 8 Tier-1 langs, n/a rows |
| REPORT-03 | 89-01 | ✓ SATISFIED | ablations 5 deltas incl. aggregate-time no_semantic + CI-overlap |
| REPORT-04 | 89-01 | ✓ SATISFIED | cost_quality ASCII scatter + valid_until citation |
| REPORT-05 | 89-03 | ✓ SATISFIED | shared renderAll + hermetic byte-reproducibility (4 reports) + report==aggregate |
| INFRA-04 | 89-04 | ✓ SATISFIED (structure) / ? live-CI human-gated | bench.yml + hermetic parse test; live run inspection-gated |
| INFRA-05 | 89-02 | ✓ SATISFIED | cleanRows exclusion across ALL headline numbers + footnote + production injector; discrimination proven |

No orphaned requirements — all 7 Phase-89 REQ-IDs in REQUIREMENTS.md are claimed by a plan.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| bench/aggregator/report.go | 78 | `"TBD"` substring | ℹ️ Info | Inside a comment that explicitly negates it ("the REAL producer — NOT 'TBD'"). Not a debt marker. |

No blocker debt markers. The `notYetImplemented("report")` stub was removed (report subcommand fully implemented).

### Human Verification Required

#### 1. INFRA-04 live CI execution (inspection-gated by design)

**Test:** Open a real PR and confirm `bench-quick` runs `make bench-quick` within the 5-minute hard cap with no LLM cost; trigger `bench-full` via `workflow_dispatch` as a maintainer.
**Expected:** bench-quick ≤5 min on PR with no provider secret; bench-full only on schedule/workflow_dispatch, never on pull_request.
**Why human:** Per 89-VALIDATION.md the live GitHub Actions run is explicitly inspection-gated (verified post-merge). The YAML STRUCTURE is hermetically proven locally (`TestBenchWorkflow` PASS); a live runner cannot be exercised from the verifier sandbox. This is the single honestly-deferred item — it does NOT block goal achievement (the per-brief criterion: `passed` if YAML is hermetically parse-tested AND the live run honestly inspection-gated — both hold).

### Gaps Summary

No gaps. All 4 must-haves are VERIFIED against the live codebase. The load-bearing claims (REPORT-05 byte-reproducibility, INFRA-05 canary integrity) are hermetically proven: byte-reproducibility by a no-network double-render+golden test, and canary exclusion by independent revert-and-fail discrimination on each of CR-01/WR-01/WR-02 (each reverts to the exact `0.5 vs 0.667` inflation the canary exists to prevent). The 1 CRITICAL + 3 warnings from 89-REVIEW are confirmed FIXED in source without regressing determinism (`make vet` + full aggregator suite green; git tree clean post-revert). The only non-automated item (INFRA-04 live CI run) is honestly inspection-gated and routed to human per the phase brief, which explicitly permits `passed` for that item under hermetic structural proof.

---

_Verified: 2026-06-21T13:05:00Z_
_Verifier: Claude (gsd-verifier)_
