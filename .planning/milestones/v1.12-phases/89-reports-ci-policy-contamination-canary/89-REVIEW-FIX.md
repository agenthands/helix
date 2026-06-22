---
phase: 89-reports-ci-policy-contamination-canary
fixed_at: 2026-06-21T09:07:29Z
review_path: .planning/phases/89-reports-ci-policy-contamination-canary/89-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 89: Code Review Fix Report

**Fixed at:** 2026-06-21T09:07:29Z
**Source review:** .planning/phases/89-reports-ci-policy-contamination-canary/89-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (1 BLOCKER + 3 Warnings)
- Fixed: 4
- Skipped: 0

Root cause (single, as named in the fix brief): the canary `cleanRows` exclusion
(INFRA-05 integrity invariant — every published/headline number computed over the
CLEAN subset, only `CanaryPassRate` over all rows) was wired into only
`reduceLeaderRow` and `reduceCostRow`. The Phase 89 additive reduces that landed
alongside it (`reduceVerifiedCorrectness`, `successVectorForMode`,
`reduceLanguageRows`) bypassed it, so a contaminated row's verdict silently inflated
published headline / ablation / per-language numbers. WR-03 is a separate
path-traversal gap in the operator `report` command.

## Fixed Issues

### CR-01 (BLOCKER): verified_correctness headline column counted contaminated rows

**Files modified:** `bench/aggregator/aggregate.go`, `bench/aggregator/aggregate_canary_test.go`
**Commit:** d82a153e
**Applied fix:** Routed `reduceVerifiedCorrectness` through the same `cleanRows` split
the other headline reduces use, with the per-cell skip on an empty cell. The
cost-vs-verified scatter y-axis is fixed automatically — `buildScatterPoints` reads
the already-reduced `LeaderRow.VerifiedCorrectness`, so excluding contaminated rows
from the reduce excludes them from the scatter too (no separate edit needed).

**Test strengthening (the load-bearing part of CR-01):** the prior
`TestCanaryExclusionFromHeadline` fixture gave clean and dirty rows the SAME verdict
(`verified_correctness=true` on every row), so pooled-clean (1.0) == pooled-all (1.0)
and the assertion never discriminated. Replaced it with a DISCRIMINATING fixture: two
clean tasks ({unsolved+verified=false, solved+verified=true} → 0.5) plus one
fully-contaminated solved+verified task that, if counted, pushes pass@1 /
verified_correctness / per_language from 0.50 up to 0.667. Verified by reverting the
fix: the strengthened test then fails with `0.5 vs 0.667` (difference 0.1667),
proving the regression is genuinely caught.

**IN-03 (footnote text):** resolves automatically — `report.go:368` now legitimately
claims verified_correctness is excluded from the headline. No edit required.

> Logic-bearing exclusion change verified by an explicit revert-and-fail check, not
> just by syntax/build — the discriminating fixture is the proof.

### WR-01: ablation task_success deltas counted contaminated rows

**Files modified:** `bench/aggregator/aggregate.go`
**Commit:** 31f43351
**Applied fix:** Applied the `cleanRows` split inside `successVectorForMode` before
`successCount`. `present` is still derived from PRE-filter row existence so a
fully-contaminated mode renders an em-dash (Present propagation) rather than vanishing
from the ablation comparison. Asserted by the new `TestCanaryExclusionFromAblations`,
which uses the REAL ablation operand modes (`your_agent_full` / `your_agent_no_lsp`)
and asserts the FullCI point is 0.5 (clean {0,1}) not 0.667 (with the dirty task).

### WR-02: per-language pass-rate counted contaminated rows

**Files modified:** `bench/aggregator/aggregate.go`
**Commit:** 6c7de989
**Applied fix:** Applied the `cleanRows` split per cell in `reduceLanguageRows` before
bucketing rows by language. `reduceCanaryRate` (the MEASUREMENT) deliberately still
reads ALL rows. Asserted by the strengthened `TestCanaryExclusionFromHeadline`
per_language block (clean pooled 2/4 = 0.5, n = 4 clean rows only).

### WR-03: `report --run-id` resolved `--out` with zero validation

**Files modified:** `cmd/helix-bench/report.go`, `cmd/helix-bench/report_test.go`
**Commit:** a97050de
**Applied fix:** Added `isValidOut`. `--out` remains an operator-trusted root
(absolute paths allowed), but a cleaned path that is bare `..` or leads with a `..`
segment is REFUSED before `filepath.Join(out, runID)`, closing the
`--out ../../x` relocation vector. Narrowed the doc comment to state what is actually
enforced (it previously over-claimed containment for the un-validated `--out` half).
`TestReportOutValidation` asserts the SPECIFIC `invalid --out` error, so it
discriminates the guard from an incidental fail-closed `Aggregate` error on a missing
directory (verified by revert-and-fail).

## Skipped / Out of Scope

- **IN-01 (action-pin convention):** out of scope per the fix brief. bench.yml
  tag-pins (`actions/checkout@v4`, `actions/setup-go@v5`), which matches the
  established convention for the non-release CI lane (`go-test.yml`, `codeql.yml`,
  `codespell.yml` all tag-pin). No change.
- **IN-02 (`reduceSwebenchScores` latent unrendered bypass):** out of scope per the
  fix brief — **TRACKED LATENT ITEM**. `reduceSwebenchScores` still pools over raw
  `loaded.Rows` with no `cleanRows` split. There is NO published-number leak today
  because its columns (RawScore/RescoredScore) are not rendered into any report. The
  moment a future phase renders them, the same contamination leak reappears. Apply the
  same `cleanRows` split pattern (CR-01/WR-01/WR-02) when that render lands.

## Verification

All required gates pass:

- `go build ./...` — PASS
- `go vet ./bench/... ./cmd/helix-bench/...` — PASS
- `go test -count=1 ./bench/aggregator/... ./cmd/helix-bench/... ./bench/runtime/...` — PASS
- Load-bearing: `go test ./bench/aggregator/ -run
  'ByteReproducible|Determ|GoldenStable|CanaryExclusion|Contaminat|Verified|Ablation|PerLanguage'`
  — PASS (TestReportByteReproducible, TestDeterministic, all *Golden / *ByteStable green)
- `make vet` (full gate incl. custom ablation-leakage / bench-rag-leakage vet tools) — PASS

Determinism / byte-reproducibility intact: `cleanRows` consumes no RNG and is applied
inside the existing sort-before-emit cells, so it does not perturb the locked
metric-reduction order or the bootstrap draws. The golden fixtures contain no
contaminated rows, so the canary exclusion is a no-op on them — no golden
regeneration was required and all golden tests stayed green.

---

_Fixed: 2026-06-21T09:07:29Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
