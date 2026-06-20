# Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard - Context

**Gathered:** 2026-06-20
**Status:** Ready for planning

> **Generated in `--auto` mode** (auto_advance chain from Phase 81). All gray areas were
> auto-selected and each decision below took the recommended option. Review before planning;
> any decision can be overridden in `/gsd-plan-phase 82`.

<domain>
## Phase Boundary

Deliver `bench/aggregator/` — the first externally-publishable bench artifact. It consumes the
per-`(task, mode, run_index)` `result.v2.json` rows already produced by the Phase 77–80 matrix
runner, aggregates **N ≥ 3 runs per (task, mode)**, and emits:

- **BCa (bias-corrected accelerated) bootstrap confidence intervals** (≥ 10,000 resamples) for
  every aggregated metric on every leaderboard row (STATS-02).
- **`pass@1` / `pass@k`** via the HumanEval unbiased closed-form (STATS-03).
- **`cost_per_solved_task`** rolled up from each result's provider `usage` block against the
  Phase 75 cost table (COST-02), rendered in `cost_quality.md` with BCa CIs (COST-03).
- **`leaderboard.md`** — the first leaderboard, from **internal ToolBench-Go data only**.
- **N-enforcement** at the matrix runner (emit N cells per cell) and a fail-closed aggregator
  that refuses to write `reports/` if any cell has fewer than the configured minimum (STATS-01).
- **CI-overlap honesty gate** (STATS-04): flag any leaderboard cell whose BCa CI overlaps a
  neighbor; never render an "X > Y" claim without non-overlapping CIs.

**Requirements:** STATS-01, STATS-02, STATS-03, STATS-04, COST-02, COST-03.

**Explicitly NOT in this phase** (scope anchor — belongs to later phases):
- External/public benchmarks (SWE-bench, Aider Polyglot, Terminal-Bench, etc.) — leaderboard is
  internal-ToolBench-Go-only here. Cross-benchmark reporting is the Reports phase (89).
- The `baseline_rag` arm / real RAG data (Phase 83).
- The full reports surface beyond `leaderboard.md` + `cost_quality.md` (Phase 89).
</domain>

<decisions>
## Implementation Decisions

### Aggregator shape & invocation
- **D-01:** Build a **new `bench/aggregator/` package** that is a **pure function over a run
  directory** (`reports/<run_id>/<task>/<mode>/<run_index>/result.v2.json`). It reads result.v2
  rows, never spawns a daemon, and writes report artifacts back into `reports/<run_id>/`. This
  makes the bulk of the aggregator **unit-testable WITHOUT `HELIX_BIN`** (only an end-to-end
  smoke needs real run data) — a deliberate departure from the daemon-gated `bench/runtime/`
  tests. *(auto-selected: recommended — decoupled, testable, supersedes the inline deltas path.)*
- **D-02:** Expose it via a `helix-bench aggregate <run_dir>` subcommand (mirrors the existing
  `helix-bench run` dispatch). Optionally also auto-invoke the aggregator at the end of
  `RunMatrix` (the same call site where Phase 80's `ComputeAndWriteDeltas` runs today) — but the
  subcommand is the primary, re-runnable entry point. *(recommended.)*
- **D-03:** This aggregator **supersedes** Phase 80's single-run `bench/runtime/deltas.go`
  (`ComputeAndWriteDeltas`, 3 fixed deltas over a 5-metric subset). Reuse its proven patterns —
  group-by-task, explicit-null handling (never fabricate 0), `writeDurable()` atomic writes,
  re-`Validate()` after any result.v2 write-back — but generalize to N runs and the full metric
  set. Do not delete deltas.go in this phase unless planning shows it is fully subsumed; prefer
  to have the aggregator own multi-run deltas-with-CIs and leave the single-run path until the
  reports phase consolidates.

### N ≥ 3 runs (STATS-01)
- **D-04:** **N is configurable per-suite, default 3.** Surface it as a run flag
  (`--runs N`, default 3) and/or a suite-manifest field; `ExpandMatrix` emits **N cells per
  (task, mode)** with `RunIndex 0..N-1` (the `Cell.RunIndex` field, `cellDurablePaths`, and the
  concurrent dispatch already support this — no new path machinery).
- **D-05:** The aggregator is **fail-closed on insufficient N**: if ANY (task, mode) cell has
  fewer than the configured minimum of *valid* result.v2 rows, it writes **no** `reports/`
  artifacts and returns a hard error naming the deficient cells. The expected N is read from the
  run manifest/flag (do not infer N from however many files happen to be on disk). Schema-level:
  assert/validate `runs` array length ≥ N where a `runs` array is materialized.

### BCa bootstrap (STATS-02)
- **D-06:** Implement a **proper BCa** interval (not percentile-only, not normal-approx):
  bias-correction `z0` from the proportion of bootstrap replicates below the observed statistic,
  plus **acceleration `a` via jackknife** over the resampling units. *(recommended — the
  acceptance test is against a closed-form known distribution, which a naive percentile bootstrap
  would fail on skew.)*
- **D-07:** **Resampling unit = per-task aggregates** (STATS-02: "over per-task aggregates"). For
  each (task, mode), reduce its N runs to one per-task statistic (e.g. mean for continuous
  metrics; success-rate for boolean), then bootstrap-resample **across tasks** to get the
  leaderboard-row CI. Bootstrap iterations **default 10,000, floor 10,000** (configurable up).
- **D-08:** **Deterministic, seeded RNG** (seed from config/flag) so reports are reproducible —
  consistent with the repo's reproducibility discipline (CLAUDE.md build/repro emphasis). Record
  the seed and resample count in the report so a CI is reproducible.
- **D-09:** **CI level default 95%.** Edge cases mirror deltas.go null discipline: a metric that
  is nil across all runs → CI is **null/omitted** (never fabricated 0); a degenerate
  all-identical sample → point CI (lo == hi == value), not a crash. CIs are computed per metric
  per leaderboard row.

### pass@k (STATS-03)
- **D-10:** **LOCK the HumanEval unbiased estimator** `pass@k = 1 − C(n−c, k) / C(n, k)`, where
  `n` = runs for the task, `c` = count of runs with `task_success == true`. **DO NOT** use the
  naive `1 − (1 − p)^k` — it is a biased estimator and would fail the STATS-03 acceptance test
  against published HumanEval reference values. ⚠ This is the single most important correctness
  decision in the phase; the researcher/planner must heed it.
- **D-11:** Compute the combinatorial ratio in **log-space (lgamma / log-binomial)** to avoid
  overflow / precision loss for larger `n`; guard `k ≤ n`. Report **pass@1 and pass@k** (k
  configurable; sensible defaults `{1, N}`). The per-mode leaderboard pass@k = mean of per-task
  pass@k across tasks, with a BCa CI (per D-06/D-07).

### Cost rollup (COST-02 / COST-03)
- **D-12:** USD per result = `(tokens_input·input_per_mtok + tokens_input_cached_read·cached_input_per_mtok
  + tokens_output·output_per_mtok) / 1_000_000`, joining the result's `model_id` to its row in
  `bench/datasets/cost-table.yaml`. **`cost_per_solved_task` = Σ(USD over solved tasks) /
  count(solved)** where "solved" = `task_success == true` (COST-02).
- **D-13:** Write a fresh cost helper (YAML loader for `cost-table.yaml` + a lookup-by-`model_id`
  + the multiply/divide). **Honor the cost table's freshness gates** — fail closed if the matched
  row is past `valid_until` or `last_verified` is stale (>90 days), consistent with the FAIR-02
  deprecation discipline already in the fairness contract.
- **D-14:** **`cache_write` token pricing:** `cost-table.yaml` currently has only
  `input_per_mtok` / `output_per_mtok` / `cached_input_per_mtok` (no cache-write column). For v1,
  price `tokens_input_cache_write` at the **standard `input_per_mtok` rate** as a documented
  conservative approximation, and leave a clearly-commented TODO. A precise
  `cache_write_per_mtok` column is a follow-up (do not expand the cost-table schema in this
  phase). *(recommended — keeps scope tight; flag for research to confirm the approximation.)*
- **D-15:** `cost_quality.md` renders **cost_per_solved_task per mode × benchmark with BCa CIs**
  (COST-03), and also lands the **FAIR-03 > 5 % between-run variance warning** (the variance
  *gate* consumes STATS-01's N≥3 here; the broader fairness-warning rendering surface is Phase
  89, but the cost_quality.md warning is in scope this phase per the FAIR-03 phase split).

### Leaderboard & honesty gate (STATS-04)
- **D-16:** `leaderboard.md` rows = **(mode × benchmark)** over internal ToolBench-Go only;
  columns = the headline metrics (task_success / pass@k, tokens_input/output, tool_calls,
  files_read, edit_locality, …) each shown **with its BCa CI**. Sort by the primary metric
  (task_success desc).
- **D-17:** **STATS-04 overlap gate:** when a row's BCa CI overlaps a neighboring row's CI for a
  metric, annotate the cell (e.g. `⚠ CI overlap — no X>Y claim`) and suppress any directional
  superiority claim for that pair. A synthetic-overlap fixture must render the warning (the
  acceptance test).

### Claude's Discretion
- Exact Go package layout under `bench/aggregator/` (e.g. `bootstrap.go`, `passk.go`, `cost.go`,
  `leaderboard.go`, `aggregate.go`), function signatures, and markdown table formatting are left
  to the planner/executor, provided the decisions above hold.
- Whether deltas-with-CIs replace the Phase 80 single-run deltas inline or live only in the
  aggregator output — planner's call, guided by D-03.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner) MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` § "Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard" — goal + 4 success criteria.
- `.planning/REQUIREMENTS.md` — STATS-01 (line 71), STATS-02 (72), STATS-03 (73), STATS-04 (74), COST-02 (79), COST-03 (80); FAIR-03 (28, the variance-gate phase split that lands its detector here).

### Result schema & metrics (what the aggregator reads)
- `bench/schema/result.v2.schema.json` — the result contract (open additive schema; `metrics` object, `usage`/token fields, `model_id`, `outcome`, `run_index`, `ablation_deltas`/`ablation_status`).
- `bench/schema/schema.go` — `ResultV2SchemaBytes` go:embed (single source of truth).
- `bench/runtime/result.go` — `BuildResult(ResultInput)` (build), `Validate(bytes)` (re-validate every write-back), types `ResultInput` / `resultDoc` / `resultFairness` / `resultOverride`.
- `bench/evaluators/metrics.go` — `evaluators.Metrics` (the 19 nullable pointer fields the aggregator computes CIs over) + `MetricError`.
- `bench/evaluators/coordinator/coordinator.go` — `Grade()` (how metrics are produced; D-07 per-metric null isolation).

### Run layout & matrix (what produces the N runs)
- `bench/runtime/matrix.go` — `Cell.RunIndex`, `ExpandMatrix`, `RunMatrix`, `runOneCell` (extend to emit N cells per (task, mode)).
- `bench/runtime/cell.go` — `cellDurablePaths()` + `validateRunIndexSegment()` (the `<task>/<mode>/<run_index>/` durable path; reuse as-is).
- `bench/runtime/deltas.go` — `ComputeAndWriteDeltas` (Phase 80 single-run precursor; reuse group-by-task + null + atomic-write patterns, generalize to N runs).

### Cost inputs
- `bench/datasets/cost-table.yaml` — per-(provider, model_id) USD rates (`input_per_mtok`, `output_per_mtok`, `cached_input_per_mtok`) + freshness gates (`valid_until`, `last_verified`, `deprecation_at`).
- `bench/runners/fairness_contract.go` — `DefaultContract.ModelID` (must equal the cost-table row's `model_id`; the join key).

### External reference (formula, not a repo file)
- HumanEval pass@k unbiased estimator: Chen et al. 2021, "Evaluating Large Language Models Trained on Code" — `pass@k = 1 − C(n−k+ ... )` → use `1 − C(n−c,k)/C(n,k)`. Lock per D-10; the unit test validates against published reference values.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Durable path layout** (`cell.go:cellDurablePaths` + `validateRunIndexSegment`) — already
  yields `<out>/<task>/<mode>/<run_index>/`; the aggregator globs this tree. No new path code.
- **Matrix N-run support** (`matrix.go`: `Cell.RunIndex`, concurrent `dispatch`) — already lands
  distinct RunIndex cells in distinct dirs without overwrite; only `ExpandMatrix` needs to emit N
  cells per (task, mode).
- **result.v2 read/validate** (`result.go`: `Validate`, schema in `bench/schema/`) — reuse for
  reading rows and re-validating any aggregated row written back.
- **deltas.go patterns** — group-by-task, explicit-null handling, `writeDurable()` atomic write,
  re-validate-after-write. Direct analog for the aggregator's write path.
- **Metrics contract** (`evaluators.Metrics`, 19 nullable fields) — the exact set of metrics to
  aggregate; null fields propagate to null CIs.
- **Cost inputs** — `cost-table.yaml` (rates + freshness gates) + `fairness_contract.go`
  (`ModelID` join key).

### Established Patterns
- **Explicit nulls, never fabricated 0s** (D-06 metrics; deltas null-skip) — the aggregator MUST
  preserve this in CIs and cost rollups (a nil metric → null CI, not a 0 that skews the bootstrap).
- **HELIX_BIN-gated integration tests** (`five_of_six_test.go`, `no_semantic_store_on_test.go`,
  `daemon_tap_integration_test.go`) — but note D-01: most aggregator tests are pure-Go unit tests
  over synthetic result.v2 fixtures and need NO `HELIX_BIN`; only an end-to-end smoke does.
- **Golden-file + embedded-schema validation** (`bench/schema/testdata/result.v2.golden.json`,
  Draft 2020-12) — pattern for the aggregator's report/golden tests.
- **Atomic durable writes + re-validate** — every result.v2 write-back re-runs `Validate()`.

### Integration Points
- **Read:** N × `result.v2.json` per (task, mode) from `reports/<run_id>/<task>/<mode>/<run_index>/`.
- **Cost join:** result `model_id` → `cost-table.yaml` row.
- **Write:** `leaderboard.md` + `cost_quality.md` into `reports/<run_id>/` (reports are
  gitignored generated artifacts; `.gitkeep` root).
- **Supersedes:** the Phase 80 `ComputeAndWriteDeltas` call site at the tail of `RunMatrix`.

### Known correctness pitfalls to carry into planning
- **pass@k estimator (D-10):** must be the HumanEval *unbiased* form `1 − C(n−c,k)/C(n,k)`, not
  `1 − (1−p)^k`. The scout's first-pass description used the naive form — it is WRONG for STATS-03.
- **BCa, not percentile bootstrap (D-06):** the acceptance test is a closed-form distribution;
  skip bias-correction/acceleration and the CI will be off on skewed metrics.
- **N read from manifest, not disk count (D-05):** counting files on disk hides a partial matrix;
  the deficiency gate must compare against the *expected* N.
</code_context>

<specifics>
## Specific Ideas

- The leaderboard is the project's headline-claim vehicle: *"Same model + same budget — with
  Helix the agent solves more tasks, with fewer tokens, fewer files read, fewer destructive
  edits"* (PROJECT.md milestone goal). The STATS-04 overlap gate exists precisely to keep that
  claim honest — no superiority statement survives overlapping CIs.
- Reproducibility matters here as elsewhere in the repo: seed the bootstrap and record seed +
  resample count in the report so a leaderboard row's CI can be regenerated byte-stably.
</specifics>

<deferred>
## Deferred Ideas

- **External/public benchmark rows** on the leaderboard (SWE-bench Verified, Aider Polyglot,
  Multi-SWE-bench, Terminal-Bench) — their adapters land in Phases 84–88; cross-benchmark
  rendering is the Reports phase (89). This phase is internal-ToolBench-Go-only.
- **`baseline_rag` arm** in the leaderboard — real RAG data lands in Phase 83.
- **Precise `cache_write_per_mtok` cost column** — Phase 82 approximates cache-write at the input
  rate (D-14); a dedicated column is a follow-up.
- **The full fairness-warning reporting surface** beyond `cost_quality.md` — Phase 89.

None of these block Phase 82; discussion stayed within the aggregator scope.
</deferred>

---

*Phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard*
*Context gathered: 2026-06-20*
