---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-
verified: 2026-06-21T02:30:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: initial verification (this is the first 82-VERIFICATION.md; the 82-REVIEW.md was a code review, not a goal-backward verification)
---

# Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard — Verification Report

**Phase Goal:** First externally-publishable bench artifact — `bench/aggregator/` consumes N≥3 runs per (task,mode), computes BCa bootstrap CIs (≥10,000 resamples) + pass@k (HumanEval closed-form), rolls up cost_per_solved_task against the Phase 75 cost table, and emits leaderboard.md + cost_quality.md from internal ToolBench-Go data only.
**Verified:** 2026-06-21
**Status:** passed
**Re-verification:** No — initial goal-backward verification.

## Goal Achievement

### Observable Truths (the 4 ROADMAP Success Criteria)

| # | Truth (Success Criterion) | Status | Evidence |
|---|---------------------------|--------|----------|
| 1 | (STATS-01) N≥3 enforced by matrix runner (ExpandMatrix emits N cells + `--runs`); aggregator REFUSES reports/ if any cell < min (fail-closed); missing/empty run dir is also an error | VERIFIED | `bench/runtime/matrix.go:117-161` ExpandMatrix(...,runs) emits `runs` cells with `RunIndex: r` (0..runs-1); `cmd/helix-bench/main.go` threads `--runs`. `bench/aggregator/load.go:115-153` N-gate: deficient cells → hard error, writes nothing; `load.go:127-130` zero-discovery (empty/missing/non-numeric runDir) → hard error (CR-01 fix). Tests RUN+PASS: TestNGate (4 subtests), TestLoadZeroDiscoveryFailsClosed (empty/non_existent/only_non_numeric), TestAggregateEmptyRunDirFailsClosed, TestAggregateNonExistentRunDirFailsClosed, TestAggregateCmdFailClosed (E2E). expectedN read from arg not `len(glob)` (Pitfall 3). |
| 2 | (STATS-02/04) Proper BCa CIs (z0 + jackknife accel, ≥10,000 resamples, seeded/deterministic) for every metric; closed-form unit test; reports flag CI-overlap and suppress unjustified X>Y | VERIFIED | `bench/aggregator/bootstrap.go`: real z0 via `phiInv(p0)` of fraction-below-θ̂ (line 107), jackknife acceleration `jackknifeAccel` (eq. 14.15, line 153), injected `*rand.Rand` (D-08), WR-01 ordering guard `if lo>hi {swap}` (125-127), degenerate handling (empty→null, all-identical/m==1→point). Tests B=10000. STATS-04 overlap gate in `report.go:195-220` ("CI overlap warnings (STATS-04)"). Tests RUN+PASS: TestBCa (incl BCa_differs_from_percentile_on_skew — the fake-BCa discriminator, BCa_contains_true_mean, determinism_same_seed, 3 degenerate cases), TestBCaIntervalOrderingGuard, TestBCaPercentilesCanInvert, TestOverlapGate (3 subtests incl non-overlap renders NO warning). |
| 3 | (STATS-03) pass@1 and pass@k via UNBIASED closed-form 1−C(n−c,k)/C(n,k) as c-term product (NOT naive 1−(1−p)^k); reference PassAtK(10,3,5)==0.91667 | VERIFIED | `bench/aggregator/passk.go:46-56` product runs `i = n-c+1 .. n` = exactly **c** terms (comment line 49-52 explicitly warns against k-term loop), `prod *= 1 - k/i`. WR-02 k>n guard returns NaN unless c==n (never fabricates 1.0). Tests RUN+PASS: TestPassAtK/anchor_10_3_5 (0.91667), TestPassAtKAntiNaive (explicitly asserts result ≠ 0.83193 naive form), TestPassAtKFormAgreement (product==lgamma ~1e-12), TestPassAtKDomainGuard, pass@1==c/n identity subtests. |
| 4 | (COST-02/03) cost_per_solved_task = Σ(usage-derived USD over solved)/count(solved) joined to Phase 75 cost-table by model_id, matches hand-computed example; cost_quality.md renders per mode×benchmark with BCa CIs + FAIR-03 CV>0.05 warning | VERIFIED | `bench/aggregator/cost.go:28-73`: perResultUSD (D-12/D-14 formula, cache-write at input rate w/ TODO, nil-token excluded never $0), costPerSolvedTask (Σ/count, no-solved→null). Join via `cost.PriceFor` (aggregate.go:281) by model_id (cost-table.yaml:19 model_id key) with D-13 freshness gate (bench/cost/cost_table.go:83-119). WR-03: cost-table LOAD failure → hard error (aggregate.go:88-91). cost_quality.md render + FAIR-03 CV gate `cvThreshold=0.05` (report.go:29, 266-273). Tests RUN+PASS: TestCostPerSolved/golden_3.555_USD (hand-computed (4.50+2.61)/2), TestCostFreshnessGate (3 fail-closed subtests), TestCostQualityRender (high-variance renders FAIR-03 warning, low-variance does not, null→em-dash), TestAggregateCmdBadCostTableFailsClosed (E2E). |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `bench/cost/cost_table.go` | Exported CostRow/CostTable/ValidateCostTable/PriceFor/LoadCostTable + freshness consts | VERIFIED | Cost-table types MOVED here (single source of truth); ValidateCostTable + PriceFor + freshness gate present. 8 cost tests pass. |
| `bench/runtime/matrix.go` | ExpandMatrix runs axis emitting N RunIndex cells | VERIFIED | `ExpandMatrix(...,runs int)`, inner loop `for r:=0;r<runs;r++` emits `RunIndex: r`; runs<1→1 defensive. |
| `bench/aggregator/bootstrap.go` | BCaInterval + phi/phiInv | VERIFIED | 222 lines; genuine z0+jackknife BCa, not percentile. |
| `bench/aggregator/passk.go` | PassAtK c-term product + logBinom cross-check | VERIFIED | c-term product form; logBinom via Lgamma for cross-check. |
| `bench/aggregator/cost.go` | perResultUSD + costPerSolvedTask reusing bench/cost.PriceFor | VERIFIED | Delegates freshness gate to bench/cost; nil discipline correct. |
| `bench/aggregator/load.go` | Load(runDir,expectedN) + deficiency gate + nullable rowMetrics | VERIFIED | N-gate + zero-discovery fail-closed; rowMetrics mirrors evaluators.Metrics pointer types. |
| `bench/aggregator/aggregate.go` | Aggregate orchestrator + two-level reduce | VERIFIED | Pure orchestrator wires Load→BCa/PassAtK/perResultUSD→render; one seeded RNG; WR-03 cost-load fail-closed. |
| `bench/aggregator/report.go` | renderLeaderboard + renderCostQuality + overlap + CV + atomic write | VERIFIED | os.Rename atomic write; em-dash null CI; dynamic pass@%d header; STATS-04 + FAIR-03 sections. |
| `cmd/helix-bench/aggregate.go` | newAggregateCmd cobra subcommand | VERIFIED | RunE→aggregator.Aggregate; flags --runs/--seed/--iterations/--ci-level; Today=time.Now().UTC(); error propagated. |
| `cmd/helix-bench/main.go` | root.AddCommand(newAggregateCmd()) | VERIFIED | main.go:88 registered. |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| cmd/helix-bench/validate_cost_table.go | bench/cost | cost.ValidateCostTable | WIRED (validate_cost_table.go:7 import, :44 call) |
| cmd/helix-bench/main.go | runtime.ExpandMatrix | --runs threaded | WIRED |
| bootstrap.go | math.Erfinv / math/rand/v2 | phiInv + NewPCG | WIRED |
| cost.go | bench/cost.PriceFor | model_id join + freshness | WIRED (aggregate.go:281) |
| load.go | runtime.Validate | per-row schema re-validate | WIRED (load.go:227) |
| aggregate.go | BCaInterval/PassAtK/perResultUSD/Load | orchestrator | WIRED |
| report.go | os.Rename atomic write | temp+rename | WIRED (report.go:334) |
| cmd/helix-bench/aggregate.go | aggregator.Aggregate | RunE | WIRED (aggregate.go:74) |
| cmd/helix-bench/main.go | newAggregateCmd | AddCommand | WIRED (main.go:88) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Pure correctness suite (BCa, pass@k, cost, N-gate, overlap, variance, determinism) RUNS+PASS without HELIX_BIN | `go test ./bench/aggregator/... ./bench/cost/... -count=1 -v` | all PASS (TestBCa, TestPassAtK/anchor 0.91667, TestPassAtKAntiNaive, TestNGate, TestLoadZeroDiscoveryFailsClosed, TestOverlapGate, TestCostQualityRender, TestCostPerSolved golden, TestDeterministic, TestAggregateEndToEnd) | PASS |
| Binary builds | `go build -o helix ./cmd/helix` | BUILD_OK | PASS |
| Aggregate subcommand E2E RUNS (not SKIP) | `HELIX_BIN=.../helix go test ./cmd/helix-bench/ -run 'Aggregate' -count=1 -v` | 3/3 PASS (FailClosed, BadCostTableFailsClosed, Sufficient) — RAN, not skipped | PASS |
| Vet incl ablation-leakage | `make vet` | exit 0 (all 6 vet tools clean) | PASS |
| Full standard suite (no HELIX_BIN) | `go test ./...` | exit 0, no failures | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| STATS-01 | 82-01, 82-05, 82-07 | N≥3 enforced; matrix emits N; aggregator fail-closed | SATISFIED | ExpandMatrix N-cells + N-gate + zero-discovery guard; TestNGate, TestLoadZeroDiscoveryFailsClosed, E2E FailClosed |
| STATS-02 | 82-02, 82-06 | BCa CIs ≥10k over per-task aggregates, every metric | SATISFIED | BCaInterval z0+jackknife B=10000; TestBCa incl fake-BCa discriminator |
| STATS-03 | 82-03, 82-06 | pass@1/pass@k unbiased closed-form | SATISFIED | c-term product; TestPassAtK anchor 0.91667 + anti-naive |
| STATS-04 | 82-06 | Overlap gate, suppress X>Y | SATISFIED | report.go overlap section; TestOverlapGate (overlap warns, non-overlap silent) |
| COST-02 | 82-04, 82-06 | cost_per_solved_task formula joined to cost-table | SATISFIED | perResultUSD/costPerSolvedTask + PriceFor join; TestCostPerSolved golden 3.555 |
| COST-03 | 82-06, 82-07 | cost_quality.md per mode×benchmark with BCa CIs + FAIR-03 variance | SATISFIED | renderCostQuality + CV gate; TestCostQualityRender |

No orphaned requirements: REQUIREMENTS.md maps STATS-01..04, COST-02, COST-03 to Phase 82; all are claimed by plans and verified. FAIR-03's variance-gate (CV>0.05 in cost_quality.md) is the in-scope slice for this phase per the documented phase split, and is implemented + tested.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| bench/aggregator/cost.go | 41 | `TODO(D-14)` cache_write_per_mtok column follow-up | ℹ️ Info | Line references the locked decision D-14 (conservative input-rate approximation is the documented v1 design, precise column deferred per CONTEXT deferred list). Not unresolved debt — it is a design-locked approximation with a tracked follow-up scope. Does not block goal. |

No `TBD`/`FIXME`/`XXX` markers, no placeholder/stub returns, no fabricated-empty data paths in the phase's modified files. The single `TODO(D-14)` is a design-locked, CONTEXT-deferred approximation (not a broken/incomplete implementation), so it does not trip the debt-marker blocker gate.

### Human Verification Required

None. The phase deliverable is pure-Go statistical/cost code with deterministic, self-checking unit tests (closed-form anchors: pass@k 0.91667, cost 3.555 USD, BCa-vs-percentile divergence, overlap/variance render fixtures). No visual/real-time/external-service behavior requires human judgment. The published markdown artifacts are byte-determinism-tested against golden files.

### Deferred / Out-of-Scope (informational)

- **`TestRunSubcommandWiresDeltaPass`** (cmd/helix-bench/run_cmd_test.go:93) is **pre-existing Phase 80-05 debt**, NOT a Phase 82 gap. Confirmed: the last commit touching this file is `c95b1ae4 fix(80)` (a Phase 80 commit); no Phase 82 commit touched run_cmd_test.go or runBench. The test asserts `runBench` writes `ablation_deltas` (a Phase-80 delta-pass write-back feature whose GREEN never landed); it FAILS under HELIX_BIN and SKIPs without it, identically on pre-Phase-82 commits. Per the project test gate and deferred-items.md, Phase 82 is NOT failed on it. HELIX_BIN runs for Phase 82 were correctly scoped to `-run 'Aggregate'`, which passes 3/3.

### Gaps Summary

No gaps. All 4 ROADMAP success criteria are observably true in the codebase, backed by primary pure-Go unit tests that RUN+PASS without HELIX_BIN and an HELIX_BIN-gated subcommand E2E that RUNS (not skips) and passes. The statistical core is correct and adversarially tested: pass@k is the unbiased c-term product (anti-naive + form-agreement guards), BCa is a genuine z0+jackknife interval (fake-BCa discriminator test present, WR-01 ordering guard), the N-gate is fail-closed including the zero-discovery hole (CR-01 fix), and the cost rollup joins to the Phase 75 cost table with a fail-closed freshness gate (WR-03 fix at the aggregator boundary). All 7 code-review findings (1 critical + 3 warning + 3 info) are fixed and carry regression tests. `make vet` (incl ablation-leakage) and the full standard suite both exit 0.

---

_Verified: 2026-06-21_
_Verifier: Claude (gsd-verifier)_
