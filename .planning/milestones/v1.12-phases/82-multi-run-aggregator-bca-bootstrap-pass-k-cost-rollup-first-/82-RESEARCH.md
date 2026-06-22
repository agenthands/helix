# Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard - Research

**Researched:** 2026-06-20
**Domain:** Statistical aggregation (BCa bootstrap, pass@k unbiased estimator), cost rollup, markdown report rendering — pure-Go, no external deps
**Confidence:** HIGH (algorithms verified against published sources + stdlib probes; all integration points read from live code)

## Summary

Phase 82 builds `bench/aggregator/` — a pure-function Go package that reads N≥3 `result.v2.json`
rows per `(task, mode)` from `bench/reports/<run_id>/<task>/<mode>/<run_index>/result.v2.json`,
computes BCa bootstrap CIs and HumanEval-unbiased pass@k over per-task aggregates, rolls up
`cost_per_solved_task` against `bench/datasets/cost-table.yaml`, and emits `leaderboard.md` +
`cost_quality.md` into the run dir. Every algorithm here is hand-rollable in well under the repo's
~60-LOC-PageRank budget and needs **zero new dependencies**: Go's `math.Erfinv` (inverse-normal,
verified present) and `math.Lgamma` (log-gamma, verified present) cover the only two "hard" numeric
primitives, and `gopkg.in/yaml.v3` (already a direct dep) covers the cost-table loader.

The single most important correctness fact: **pass@k MUST be the HumanEval unbiased estimator in
the numerically-stable product form** `1 − Π_{i=n−c+1}^{n}(1 − k/i)` evaluated when `n−c ≥ k`
(else 1.0), NOT the naive `1 − (1 − p)^k`. ⚠ The product has exactly **c** terms (c = number of
successes), NOT k — looping k times is the classic wrong implementation (it yields 0.97348 for
(n=10,c=3,k=5) instead of the correct 0.91667). The second: **BCa is bias-correction (z0) +
acceleration (a via jackknife)**, not a percentile bootstrap. Both are exercised by closed-form
acceptance tests (STATS-02/STATS-03) that a naive implementation would fail.

The aggregator is overwhelmingly unit-testable without `HELIX_BIN`: all math, the cost join, the
N-enforcement gate, and report rendering run over synthetic `result.v2.json` fixtures written to a
temp dir. Only an optional end-to-end smoke needs a real run.

**Primary recommendation:** Build five files under `bench/aggregator/` — `load.go` (glob + read +
N-gate), `bootstrap.go` (BCa + seeded RNG), `passk.go` (lgamma log-binomial, stable product form),
`cost.go` (cost-table loader + join + rollup), `report.go` (leaderboard.md + cost_quality.md +
overlap/variance gates) — plus `aggregate.go` as the pure orchestrator. Wire a `helix-bench aggregate
<run_dir>` subcommand. Reuse `deltas.go`'s group-by-task / explicit-null / `writeDurable` /
re-`Validate` discipline; do not delete `deltas.go` this phase (D-03).

## User Constraints (from CONTEXT.md)

### Locked Decisions (D-01 .. D-17 — honor verbatim; do not re-litigate)

- **D-01:** New `bench/aggregator/` package, **pure function over a run directory**
  (`reports/<run_id>/<task>/<mode>/<run_index>/result.v2.json`). Reads rows, never spawns a daemon,
  writes report artifacts back into `reports/<run_id>/`. Bulk is unit-testable WITHOUT `HELIX_BIN`.
- **D-02:** Expose via `helix-bench aggregate <run_dir>` subcommand (mirrors `run` dispatch).
  Optionally auto-invoke at the tail of `RunMatrix` (the `ComputeAndWriteDeltas` call site). The
  subcommand is the primary, re-runnable entry point.
- **D-03:** Supersedes Phase 80's single-run `deltas.go`. Reuse its patterns (group-by-task,
  explicit-null, `writeDurable`, re-`Validate`) generalized to N runs + the full metric set. Do NOT
  delete `deltas.go` this phase unless planning shows it fully subsumed.
- **D-04:** N configurable per-suite, **default 3**. Surface as `--runs N` flag and/or suite-manifest
  field. `ExpandMatrix` emits **N cells per (task, mode)** with `RunIndex 0..N-1`.
- **D-05:** **Fail-closed on insufficient N**: if ANY (task, mode) cell has fewer than the configured
  minimum of *valid* rows, write **no** `reports/` artifacts and return a hard error naming deficient
  cells. Expected N is read from the manifest/flag, **NOT inferred from on-disk file count**.
- **D-06:** **Proper BCa** (not percentile-only, not normal-approx): bias-correction `z0` from the
  proportion of bootstrap replicates below the observed statistic + acceleration `a` via **jackknife**
  over the resampling units.
- **D-07:** **Resampling unit = per-task aggregates.** For each (task, mode) reduce N runs to one
  per-task statistic (mean for continuous; success-rate for boolean), then bootstrap-resample **across
  tasks**. Iterations **default 10,000, floor 10,000**, configurable up.
- **D-08:** **Deterministic, seeded RNG** (seed from config/flag). Record seed + resample count in the
  report so a CI is reproducible.
- **D-09:** **CI level default 95%.** Null discipline: a metric nil across all runs → CI null/omitted
  (never fabricated 0); degenerate all-identical sample → point CI (lo == hi == value), not a crash.
- **D-10:** **LOCK the HumanEval unbiased estimator** `pass@k = 1 − C(n−c, k) / C(n, k)`, `n` = runs
  for the task, `c` = count of runs with `task_success == true`. **DO NOT** use naive `1 − (1 − p)^k`.
  ⚠ Single most important correctness decision in the phase.
- **D-11:** Compute the ratio in **log-space (lgamma / log-binomial)**; guard `k ≤ n`. Report pass@1
  and pass@k (k configurable; defaults `{1, N}`). Per-mode leaderboard pass@k = mean of per-task
  pass@k across tasks, with a BCa CI.
- **D-12:** USD per result = `(tokens_input·input_per_mtok + tokens_input_cached_read·cached_input_per_mtok
  + tokens_output·output_per_mtok) / 1_000_000`, joining `model_id` → cost-table row.
  **`cost_per_solved_task` = Σ(USD over solved tasks) / count(solved)**, "solved" = `task_success == true`.
- **D-13:** Fresh cost helper (YAML loader + lookup-by-`model_id` + multiply/divide). **Honor freshness
  gates** — fail closed if matched row is past `valid_until` or `last_verified` >90 days stale.
- **D-14:** **`cache_write` pricing:** cost-table has no cache-write column. For v1 price
  `tokens_input_cache_write` at the **standard `input_per_mtok` rate** (documented conservative
  approximation, clearly-commented TODO). Do NOT expand the cost-table schema this phase.
- **D-15:** `cost_quality.md` renders cost_per_solved_task per mode × benchmark with BCa CIs, and lands
  the **FAIR-03 > 5 % between-run variance warning**.
- **D-16:** `leaderboard.md` rows = **(mode × benchmark)** over internal ToolBench-Go only; columns =
  headline metrics each with its BCa CI; sort by primary metric (task_success desc).
- **D-17:** **STATS-04 overlap gate:** when a row's BCa CI overlaps a neighbor's CI for a metric,
  annotate the cell (e.g. `⚠ CI overlap — no X>Y claim`) and suppress any directional superiority
  claim. A synthetic-overlap fixture must render the warning.

### Claude's Discretion

- Exact Go package layout under `bench/aggregator/` (`bootstrap.go`, `passk.go`, `cost.go`,
  `leaderboard.go`, `aggregate.go`), function signatures, and markdown table formatting.
- Whether deltas-with-CIs replace the Phase 80 single-run deltas inline or live only in the aggregator
  output — planner's call, guided by D-03.

### Deferred Ideas (OUT OF SCOPE)

- External/public benchmark rows (SWE-bench, Aider Polyglot, Multi-SWE-bench, Terminal-Bench) — adapters
  Phases 84–88; cross-benchmark rendering Phase 89. **This phase is internal-ToolBench-Go-only.**
- `baseline_rag` arm — real RAG data Phase 83.
- Precise `cache_write_per_mtok` cost column — follow-up (D-14 approximates at input rate).
- Full fairness-warning reporting surface beyond `cost_quality.md` — Phase 89.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STATS-01 | Default N≥3 runs/task/mode; configurable per-suite; matrix runner enforces; schema validates `runs` length ≥ N | §N≥3 Enforcement: `ExpandMatrix` emits N cells; aggregator fail-closes on count-of-valid-rows < expected-N read from manifest/flag (NOT disk count) |
| STATS-02 | BCa bootstrap CIs over per-task aggregates, every metric every row, ≥10,000 iters; closed-form acceptance + width sanity | §BCa Bootstrap: exact z0 + jackknife-a formulas, degenerate handling, `math.Erfinv`, closed-form test design |
| STATS-03 | pass@1 / pass@k per HumanEval `1 − C(n−c,k)/C(n,k)`; unit test vs published reference values | §pass@k: stable product form + lgamma form + verified reference values (n=10,c=3,pass@5=0.91667 and 3 hand-computed anchors) |
| STATS-04 | Flag any cell whose BCa CI overlaps a neighbor; no "X>Y" without non-overlap; synthetic-overlap rendered | §Report Rendering: interval-overlap predicate, neighbor pairing, warning annotation, synthetic fixture |
| COST-02 | `cost_per_solved_task` = Σ(USD over solved)/count(solved); matches hand-computed example | §Cost Rollup: USD formula, model_id join, solved-only filter, worked hand example for the golden test |
| COST-03 | `cost_quality.md` shows cost_per_solved_task per mode × benchmark with BCa CIs; renders for sample run | §Report Rendering: cost_quality.md layout + per-task USD as the resampling unit for the cost CI |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Glob + read N result.v2 rows | `bench/aggregator/` (pure I/O over run dir) | `bench/runtime` (path layout `cellDurablePaths`) | D-01: aggregator owns reading; reuses the matrix's `<task>/<mode>/<run_index>/` layout, no new path code |
| N≥3 enforcement (gate) | `bench/aggregator/` (fail-closed on valid-row count) | `bench/runtime/matrix.go` (`ExpandMatrix` emits N cells) | D-04/D-05: producing N runs is matrix's job; refusing on deficiency is the aggregator's |
| BCa bootstrap / pass@k / cost math | `bench/aggregator/` (pure functions) | `math` stdlib (`Erfinv`, `Lgamma`) | D-06/D-10/D-12: stats are pure, deterministic, dependency-free |
| Cost-table load + freshness gate | `bench/aggregator/cost.go` (new helper) | `bench/runners` (`DefaultContract.ModelID` join key) | D-13: cost helper is fresh; the join key is the existing fairness contract's ModelID |
| Report rendering + overlap/variance gates | `bench/aggregator/report.go` | — | D-15/D-16/D-17: markdown emission + honesty gates are presentation, owned here |
| Subcommand dispatch | `cmd/helix-bench` (`aggregate`) | `bench/aggregator` (the callable) | D-02: thin cobra wrapper over the pure package |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `math` (stdlib) | go1.26 | `math.Erfinv` (Φ⁻¹ for BCa), `math.Lgamma` (log-binomial for pass@k), `math.Erf`/`math.Sqrt` (Φ for BCa endpoints) | Repo no-heavy-deps ethos; both primitives **verified present** in the toolchain (probe below) |
| `math/rand/v2` (stdlib) | go1.26 | Seeded deterministic bootstrap resampling (D-08) | `rand/v2` gives a clean `*rand.Rand` from `rand.NewPCG(seed, seed2)` — reproducible, fast, no global state |
| `gopkg.in/yaml.v3` | v3.0.1 (existing direct dep) | Load `cost-table.yaml` (D-13) | Already the cost-table loader's decoder in `cmd/helix-bench/validate_cost_table.go`; reuse the exact `CostRow`/`CostTable` shape |
| `encoding/json` (stdlib) | go1.26 | Read `result.v2.json` rows (decode into nullable-pointer structs) | Matches `deltas.go` `rowMetrics` decode pattern; preserves explicit nulls |

**Verification probe (run this session):**
```
Erfinv(0.9)= 1.1630871536766738   Lgamma(6)= 4.787491742782046   go version go1.26.0
```
`[VERIFIED: go run probe]` Both `math.Erfinv` and `math.Lgamma` exist and return correct values; no third-party stats package is needed.

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/santhosh-tekuri/jsonschema/v6` | existing | Re-`Validate()` any result.v2 row written back | Only if the aggregator writes back into result.v2 rows (D-03 deltas-with-CIs path); the *reports* (leaderboard.md/cost_quality.md) are markdown, not schema-validated |
| `github.com/spf13/cobra` | v1.9.1 (existing) | `helix-bench aggregate` subcommand | D-02 dispatch wiring in `cmd/helix-bench` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `math.Erfinv` for Φ⁻¹ | Hand-rolled Acklam / Beasley-Springer-Moro rational approximation | UNNECESSARY — stdlib `Erfinv` gives `Φ⁻¹(p) = √2·Erfinv(2p−1)` exactly; the hand-roll only adds approximation error and LOC. **Recommend `math.Erfinv`.** |
| `math.Lgamma` log-binomial | Stable product form `1 − Π_{i=n−c+1}^{n}(1 − k/i)` | Both correct. The **product form is the canonical Chen et al. choice** and avoids even lgamma rounding for small k; lgamma generalizes cleanly. **Recommend the product form as primary, lgamma as the cross-check in the test** (compute both, assert agreement). |
| `math/rand/v2` | `math/rand` (v1) with `rand.New(rand.NewSource(seed))` | v1 works but v2 is the current idiom and its PCG source is explicitly reproducible; either satisfies D-08. Pick v2. |

**Installation:** No new modules. All four core libs are stdlib or existing direct deps.

**Version verification:**
```
gopkg.in/yaml.v3 v3.0.1          # go.mod direct dep — confirmed present
github.com/spf13/cobra v1.9.1    # existing
math.Erfinv / math.Lgamma        # stdlib go1.26 — probed, present
```
`[VERIFIED: go.mod + go run]`

## Package Legitimacy Audit

> No external packages are installed in this phase. Every dependency is Go stdlib or an existing
> direct dependency already vetted in prior phases.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `math`, `math/rand/v2`, `encoding/json` | Go stdlib | — | — | golang.org | OK | Approved (stdlib) |
| `gopkg.in/yaml.v3` | Go modules (existing) | mature | high | github.com/go-yaml/yaml | OK | Approved (already a direct dep) |
| `github.com/spf13/cobra` | Go modules (existing) | mature | high | github.com/spf13/cobra | OK | Approved (already a direct dep) |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
helix-bench aggregate <run_dir>            RunMatrix tail (optional auto-invoke, D-02)
        │                                          │
        └──────────────┬───────────────────────────┘
                       ▼
            ┌────────────────────────┐
            │  aggregate.go (pure)    │  expectedN (from --runs flag / suite manifest)
            └────────────────────────┘
                       │
        ┌──────────────┼───────────────────────────────┐
        ▼              ▼                                 ▼
  load.go         cost.go (load cost-table.yaml)   (config: seed, iters=10000, ciLevel=0.95)
  glob run dir         │ freshness gate (valid_until/last_verified) → FAIL CLOSED
  read N rows/cell     │
        │              │
        ▼              │
  ┌─────────────────┐  │
  │ N-GATE (D-05)   │  │   for each (task,mode) cell: count VALID rows
  │ valid<expectedN │  │   < expectedN  →  HARD ERROR, write NOTHING
  │  → hard error   │  │
  └─────────────────┘  │
        │ (all cells OK)│
        ▼              ▼
  per-(task,mode) aggregates ────────► per-task statistic vector (one value/task)
   (mean continuous, success-rate bool, pass@k via passk.go, USD via cost.go)
        │
        ▼
  bootstrap.go: BCa over the per-task vector (resample ACROSS tasks)
   z0 = Φ⁻¹(#(boot < θ̂)/B)   a = jackknife skewness   endpoints α1,α2 = Φ(BCa-adjusted)
        │
        ▼
  report.go ──► leaderboard.md   (mode×bench rows, metric ± [lo,hi], sort task_success desc,
        │                          STATS-04 overlap gate annotations)
        └─────► cost_quality.md  (cost_per_solved_task ± [lo,hi], FAIR-03 >5% variance warning)
                       │
                       ▼
            written into reports/<run_id>/   (atomic writeDurable; markdown, not schema-validated)
```

A reader traces the primary use case: invoke → load N rows → enforce N → reduce to per-task stats →
BCa-bootstrap each metric → render two markdown reports with honesty gates.

### Recommended Project Structure
```
bench/aggregator/
├── aggregate.go     # pure orchestrator: Aggregate(runDir, Config) (*Report, error)
├── load.go          # glob <task>/<mode>/<run_index>/result.v2.json, decode nullable rows, N-gate
├── bootstrap.go     # BCa: percentile(), jackknifeAccel(), biasCorrect(), BCaInterval()
├── passk.go         # passAtK(n, c, k) stable product form + logBinom cross-check
├── cost.go          # CostTable loader (yaml), lookup-by-model_id, freshness gate, perResultUSD()
├── report.go        # renderLeaderboard(), renderCostQuality(), overlapWarn(), varianceWarn()
└── *_test.go        # pure unit tests over synthetic result.v2 fixtures (NO HELIX_BIN)
```

### Pattern 1: Per-task reduction THEN bootstrap-across-tasks (D-07)

**What:** Two-level aggregation. Level 1: for each (task, mode), reduce its N run rows to ONE
per-task statistic. Level 2: the leaderboard-row CI bootstraps the *vector of per-task statistics*
(one element per task), resampling tasks with replacement.

**When to use:** Every metric on every leaderboard row, including pass@k and cost.

**Why this unit:** STATS-02 says "over per-task aggregates" explicitly — the task is the exchangeable
sampling unit, not the individual run. Bootstrapping individual runs would conflate within-task
variance with between-task variance and understate the CI.

**Example (Go, illustrative — signatures are Claude's discretion):**
```go
// Source: derived from D-07 + bench/runtime/deltas.go null discipline
// Level 1: reduce N runs of one (task,mode) to a single per-task value.
//   - continuous metric (tokens_input, edit_locality, ...): mean over non-nil runs
//   - boolean metric (task_success): success-rate = #true / N
//   - pass@k: passAtK(n=N, c=#true, k)  (already a per-task scalar)
//   - cost: per-task USD = mean USD over the task's runs (or Σ — see Cost Rollup)
// A metric nil in ALL N runs → per-task value is ABSENT (nil), excluded from the
// across-task vector (never fabricated 0 — deltas.go discipline).

// Level 2: bootstrap the per-task vector across tasks.
vals := perTaskStatistics(cell, metric) // []float64, one per task with a non-nil value
ci := BCaInterval(vals, statMean, 10000, 0.95, rng)
```

### Pattern 2: Deterministic seeded resampling (D-08)
**What:** One `*rand.Rand` seeded from config, threaded into every bootstrap call.
**Why:** Byte-stable reports (repo reproducibility discipline). Record seed + iteration count in
the report footer so any CI regenerates.
**Example:**
```go
// Source: math/rand/v2 stdlib
rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9E3779B97F4A7C15))
// resample: idx := rng.IntN(len(vals)) for each of len(vals) draws, B times.
```

### Pattern 3: Reuse deltas.go write discipline, generalized to N (D-03)
**What:** Group-by-task, explicit-null skip, `writeDurable()` atomic temp-file+rename, re-`Validate()`
after any result.v2 write-back. `bench/runtime/deltas.go:124` (`ComputeAndWriteDeltas`) and
`bench/runtime/cell.go:911` (`writeDurable`) are the proven templates.
**Note:** The *reports* (leaderboard.md/cost_quality.md) are markdown — they use `writeDurable` for
atomicity but are NOT schema-validated. Only the deltas-with-CIs write-BACK into result.v2 rows (if
the planner chooses that path per D-03) goes through `Validate()`.

### Anti-Patterns to Avoid
- **Naive pass@k `1 − (1 − p̂)^k`:** biased; fails STATS-03 against published values. (See Pitfall 1.)
- **c-vs-k loop-count bug (product looping k times not c times):** the product `1 − Π_{i=n−c+1}^{n}(1 − k/i)`
  has exactly **c** terms; a `for i:=0;i<k;i++` loop computes the wrong product (0.97348 not 0.91667 for
  (10,3,5)). (See Pitfall 1.)
- **Percentile bootstrap labeled "BCa":** skips z0 + a; wrong on skewed metrics. (Pitfall 2.)
- **Counting on-disk files as N:** hides a partial matrix. Compare valid-row count to *expected* N
  from the manifest/flag. (Pitfall 3.)
- **Fabricating 0 for a nil metric:** skews the bootstrap and the mean; a nil-across-all-runs metric
  yields a null CI. (Pitfall 4.)
- **Unseeded `rand`:** non-reproducible reports. (Pitfall 5.)
- **`a` division-by-zero on an all-identical sample:** guard before the BCa adjustment. (Pitfall 6.)

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Inverse normal CDF Φ⁻¹ | Acklam / Beasley-Springer-Moro rational approximation | `math.Erfinv`: `Φ⁻¹(p) = math.Sqrt2 * math.Erfinv(2*p − 1)` | Stdlib is exact to machine precision; the approximation only adds error + LOC. **Verified present.** |
| Normal CDF Φ (for BCa endpoints) | Series/continued-fraction expansion | `Φ(z) = 0.5 * math.Erfc(−z / math.Sqrt2)` | Stdlib `Erfc`; exact, one line |
| Binomial coefficient for pass@k | Big-int factorials / `C(n,k)` directly | Stable product form (primary) + `math.Lgamma` log-binomial (cross-check) | Direct factorials overflow for n>20; the product/log forms are the published numerically-stable route |
| Cost-table YAML parsing | Custom parser | `gopkg.in/yaml.v3` with `dec.KnownFields(true)` | The existing `validate_cost_table.go` already does exactly this; copy the `CostRow`/`CostTable` shape |
| Atomic durable write | `os.WriteFile` | `bench/runtime.writeDurable` pattern (temp+rename) | Avoids torn files a concurrent reader could observe (`cell.go:899-937` rationale) |

**Key insight:** The repo already ships `math.Erfinv`, `math.Lgamma`, a working YAML cost-table
loader, and an atomic-write helper. The phase's "new" code is ~150 lines of glue and arithmetic, not
a numerics library.

## BCa Bootstrap — Exact Algorithm (STATS-02, D-06/D-07/D-09)

Let `x = [x_1, ..., x_m]` be the **per-task statistic vector** for one (metric, leaderboard-row),
`θ̂ = s(x)` the observed statistic (typically the mean), `B ≥ 10000` resamples, `α` the two-tail level
(0.05 for a 95% CI → `α/2 = 0.025`).

### Step 1 — Bootstrap replicates
For `b = 1..B`: draw `m` indices uniformly with replacement from `0..m−1` (seeded RNG), form the
resample, compute `θ*_b = s(resample)`. Collect `θ* = [θ*_1, ..., θ*_B]`, then **sort** it.

### Step 2 — Bias correction z0
```
p0 = (#{ θ*_b < θ̂ }) / B
z0 = Φ⁻¹(p0) = math.Sqrt2 * math.Erfinv(2*p0 − 1)
```
`[CITED: Efron & Tibshirani, "An Introduction to the Bootstrap" (1993), §14.3]`
**Degenerate guard (Pitfall 6):** if `p0 == 0` or `p0 == 1`, `Φ⁻¹` is ±∞. Treat an all-identical
bootstrap distribution (every `θ*_b == θ̂`) as a **point CI** `[θ̂, θ̂]` and return early.

### Step 3 — Acceleration a (jackknife)
Compute leave-one-out jackknife replicates `θ̂_(i) = s(x without element i)` for `i = 1..m`.
Let `θ̄ = mean(θ̂_(i))`. Then:
```
num = Σ_i (θ̄ − θ̂_(i))^3
den = 6 * ( Σ_i (θ̄ − θ̂_(i))^2 )^(3/2)
a   = num / den          // skewness of the jackknife distribution
```
`[CITED: Efron & Tibshirani (1993), eq. 14.15]`
**Degenerate guard:** if `den == 0` (all jackknife replicates identical → no spread, e.g. m==1 or all
x identical), set `a = 0` (reduces BCa to bias-corrected percentile; with z0 also 0 it is the plain
percentile). Do NOT divide by zero.

### Step 4 — BCa-adjusted percentiles
```
zα1 = Φ⁻¹(α/2)          // e.g. Φ⁻¹(0.025) ≈ −1.959964
zα2 = Φ⁻¹(1 − α/2)      // e.g. Φ⁻¹(0.975) ≈ +1.959964

α1 = Φ( z0 + (z0 + zα1) / (1 − a*(z0 + zα1)) )
α2 = Φ( z0 + (z0 + zα2) / (1 − a*(z0 + zα2)) )

where Φ(z) = 0.5 * math.Erfc(−z / math.Sqrt2)
```
`[CITED: Efron & Tibshirani (1993), eq. 14.10]`

### Step 5 — Read endpoints off the sorted bootstrap distribution
```
lo = quantile(sorted θ*, α1)
hi = quantile(sorted θ*, α2)
```
Use a deterministic quantile rule (e.g. nearest-rank `idx = clamp(round(α·(B−1)), 0, B−1)`, or linear
interpolation — pick ONE and lock it; document it for byte-stable reproduction). Clamp `α1, α2` into
`[0,1]` defensively (extreme a/z0 can push them out).

### Degenerate / edge-case matrix (D-09)

| Condition | Handling |
|-----------|----------|
| metric nil across ALL runs of ALL tasks → empty per-task vector | CI is **null/omitted**; metric cell renders `—` (never fabricated 0) |
| all per-task values identical (zero variance) | point CI `[θ̂, θ̂]`; z0/a guards trigger; no crash |
| m == 1 (one task) | jackknife undefined → `a = 0`; CI degenerates to `[θ̂, θ̂]` (and flag low-N in report) |
| `p0 == 0` or `1` | point CI (Step 2 guard) |
| `1 − a*(z0+zα)` == 0 | clamp denominator away from 0 (e.g. `if abs < 1e-12 → use percentile fallback for that endpoint`) |

### Closed-form acceptance test design (STATS-02)

The acceptance is "unit tests against a closed-form known distribution; CI width sanity-checked." Two
complementary tests:

1. **Coverage / location on a known distribution.** Draw a fixed synthetic per-task vector from a
   distribution with a known mean (e.g. a seeded sample whose true mean is known, OR a small
   exponential-shaped vector to exercise skew where BCa ≠ percentile). With a fixed RNG seed and
   B=10000, assert the BCa interval **contains the true parameter** and that for the skewed input the
   **BCa endpoints differ from the plain-percentile endpoints** (proving z0/a actually fired — a
   percentile bootstrap masquerading as BCa would produce identical endpoints and FAIL this assertion).
2. **CI-width sanity.** Assert `hi > lo` for a non-degenerate sample; assert `hi == lo == θ̂` for an
   all-identical sample (degenerate path); assert the width shrinks as m grows (more tasks → tighter
   CI) on a fixed-distribution synthetic.
3. **Determinism.** Same seed + same input ⇒ byte-identical `[lo, hi]` across two calls (locks D-08).

> A robust, citable anchor for test 1: the BCa interval for a **symmetric** sample (z0≈0, a≈0) should
> closely match the percentile interval; the divergence on a deliberately skewed sample is the
> discriminating signal. This is exactly what a naive percentile implementation cannot reproduce.

## pass@k — HumanEval Unbiased Estimator (STATS-03, D-10/D-11)

### The locked formula
```
pass@k = 1 − C(n−c, k) / C(n, k)
```
`n` = number of runs for the task (= N), `c` = `#{ runs with metrics.task_success == true }`,
`k` = the k value (defaults `{1, N}`). `[CITED: Chen et al. 2021, "Evaluating Large Language Models
Trained on Code", arXiv:2107.03374, §2.1]`

### Numerically-stable PRIMARY implementation (the canonical Chen et al. product form)
```go
// Source: Chen et al. 2021 reference numpy estimator, transcribed to Go.
// pass@k for a single task with n samples, c correct, target k.
//
// CRITICAL: the product runs over i = n-c+1 .. n, which is exactly c terms
// (c = number of successes), NOT k. Looping k times is the classic WRONG
// implementation — it yields 0.97348 for (n=10,c=3,k=5) instead of 0.91667.
func passAtK(n, c, k int) float64 {
    if k > n {
        // k > available samples is undefined; guard per D-11. Caller should not
        // request k>n; treat as the n-c<k branch (return 1.0) or clamp upstream.
    }
    if n-c < k {
        return 1.0 // fewer than k incorrect samples → at least one of any k is correct
    }
    prod := 1.0
    // 1 - C(n-c,k)/C(n,k) computed as 1 - Π_{i=n-c+1}^{n} (1 - k/i).
    // The loop runs i = n-c+1 .. n  →  c terms (= number of successes), NOT k.
    for i := n - c + 1; i <= n; i++ {
        prod *= 1.0 - float64(k)/float64(i)
    }
    return 1.0 - prod
}
```
This is algebraically the HumanEval unbiased estimator: `C(n-c,k)/C(n,k) = Π_{i=n-c+1}^{n} (i-k)/i =
Π_{i=n-c+1}^{n} (1 - k/i)`, a product of `n - (n-c+1) + 1 = c` terms. It never overflows and needs no
factorials.

> **Worked verification (the load-bearing k≥2 anchor):** (n=10, c=3, k=5).
> `n-c = 7 ≥ k = 5`, so the loop runs i ∈ {8, 9, 10} (3 = c terms):
> `(1 − 5/8)(1 − 5/9)(1 − 5/10) = 0.375 · 0.444444 · 0.5 = 0.0833333 = 1/12`, so
> `pass@k = 1 − 1/12 = 11/12 = 0.91667`. ✓ A k-term loop (i = 8..12, 5 terms) would instead give
> 0.97348 — the bug this anchor catches.

### lgamma log-binomial CROSS-CHECK (D-11, for the test)
```go
// logBinom returns ln C(a,b) via lgamma; -inf for b>a or b<0.
func logBinom(a, b int) float64 {
    if b < 0 || b > a { return math.Inf(-1) }
    la, _ := math.Lgamma(float64(a) + 1)
    lb, _ := math.Lgamma(float64(b) + 1)
    lab, _ := math.Lgamma(float64(a-b) + 1)
    return la - lb - lab
}
// passAtKLog = 1 - exp(logBinom(n-c,k) - logBinom(n,k)), with n-c<k → 1.0.
```
This implements the correct closed form and AGREES with the c-term product form. Compute BOTH in the
test and assert they agree to ~1e-12. Two independent derivations agreeing is strong evidence of
correctness (and a k-term product loop would break the agreement).

### Verified reference values to assert against (STATS-03)

| n | c | k | pass@k | Source |
|---|---|---|--------|--------|
| 10 | 3 | 5 | **0.91667** (= 11/12) | `[VERIFIED: leehanchung.github.io pass@k worked example + hand-computed (1−5/8)(1−5/9)(1−5/10)=1/12]` — matches both forms; the k≥2 anti-naive AND anti-k-term-loop anchor |
| 5 | 1 | 1 | **0.2** (= c/n when k=1; pass@1 = mean success) | `[VERIFIED: hand-computed]` 1 − C(4,1)/C(5,1) = 1 − 4/5 |
| 5 | 2 | 2 | **0.7** (= 1 − C(3,2)/C(5,2) = 1 − 3/10) | `[VERIFIED: hand-computed]` |
| n | c | 1 | **c/n** (pass@1 reduces to the success rate) | `[VERIFIED: algebra]` 1 − C(n−c,1)/C(n,1) = 1 − (n−c)/n = c/n |
| n | 0 | k | **0.0** (no correct samples) | `[VERIFIED]` C(n,k)/C(n,k) = 1 |
| n | c≥n−k+1 | k | **1.0** (n−c<k branch) | `[VERIFIED]` early-return |

> **pass@1 == success-rate** is a load-bearing identity: it means the same per-task reduction that
> feeds the boolean `task_success` leaderboard column (success-rate) IS pass@1. Use it as a sanity
> assertion linking the two code paths. (Note: k=1 alone cannot catch the c-vs-k loop bug — at k=1 the
> loop runs once either way; the (10,3,5)→0.91667 anchor is what catches it.)

### Leaderboard pass@k (D-11)
Per-mode leaderboard `pass@k` = **mean of per-task pass@k across tasks**, with a BCa CI computed by
treating the per-task pass@k values as the across-task resampling vector (Pattern 1). Defaults: report
`pass@1` and `pass@N` (k = the configured N).

## N≥3 Enforcement (STATS-01, D-04/D-05)

### Producing N cells — `ExpandMatrix` change
Today `ExpandMatrix` (`bench/runtime/matrix.go:110-156`) emits exactly one `Cell` per
(benchmark, language, mode, task) with `RunIndex` left 0. The cell layer ALREADY threads `RunIndex`
into the durable path (`Cell.RunIndex` → `runOneCell` → `RunCell` → `cellDurablePaths`, which formats
`<task>/<mode>/<run_index>/` and V5-validates the segment via `validateRunIndexSegment`). So the ONLY
change needed is the innermost loop:

```go
// Source: bench/runtime/matrix.go:146-154 (extend the inner append)
// Add a `runs int` parameter (default 3). For each (b,l,m,t), emit `runs` cells
// with RunIndex 0..runs-1. Distinct RunIndex → distinct durable dir (no overwrite,
// per cell.go:899 writeDurable rationale + Cell.RunIndex doc).
for r := 0; r < runs; r++ {
    cells = append(cells, Cell{Benchmark: b, Language: l, Mode: m, Task: t, RunIndex: r})
}
```
Thread `--runs N` from the `run` subcommand (`cmd/helix-bench/main.go:103`, default 3) into
`ExpandMatrix`. No new path machinery (the layout, the dispatcher's per-cell goroutine, and the
atomic write are all RunIndex-aware already).

### Expected-N source (NOT disk count) — the fail-closed gate
- **Expected N** comes from the `--runs` flag (aggregate subcommand) and/or a suite-manifest field —
  **never** from "however many files are on disk" (Pitfall 3). A run-level manifest written by
  `helix-bench run` (e.g. `reports/<run_id>/run-manifest.json` carrying `{runs: N, seed, modes,
  benchmarks}`) is the clean source; the aggregate subcommand reads it (with `--runs` as override).
- **Valid row** = a `result.v2.json` that exists, decodes, and passes `Validate()` (schema-valid).
  A deferred cell (baseline_rag) writes no row by construction and is excluded from the relevant
  modes; an errored cell may leave no row.
- **Gate:** for every (task, mode) in scope, if `#valid_rows < expectedN`, append the cell to a
  deficiency list. If the list is non-empty, write **NO** report artifacts and return a hard error
  enumerating each deficient `(task, mode): got X want N`. This mirrors the fail-closed shape of the
  fairness gate (`cell.go:417`) and the cost-table validator (`validate_cost_table.go`).

> Schema note (D-05 "assert `runs` array length ≥ N where a `runs` array is materialized"): the
> current schema stores ONE row per file (`run_index` is a scalar), NOT a `runs[]` array inside one
> file. So the length check is over the *count of valid row files* per (task,mode), not an in-document
> array. If a future plan materializes a `runs[]` array, the same ≥N assertion applies in-document.
> The aggregator should treat "N valid row files" as the operative invariant this phase.

## Cost Rollup (COST-02/COST-03, D-12/D-13/D-14)

### Where the token counts come from
The result.v2 schema does **NOT** carry a raw provider `usage` block. The cost helper reads tokens
from the canonical `metrics` object (`bench/schema/result.v2.schema.json:90-109`,
`evaluators.Metrics`):
- `metrics.tokens_input` (`*int`)
- `metrics.tokens_output` (`*int`)
- `metrics.tokens_input_cached_read` (`*int`)
- `metrics.tokens_input_cache_write` (`*int`)

The model key is the top-level open prop `model_id` (set by `BuildResult` from
`runners.DefaultContract.ModelID`, `result.go:165`). **Null discipline:** if a token field is nil, do
NOT fabricate 0 for cost — a result whose token metrics are all nil has **no computable cost** and is
excluded from the cost rollup (its task contributes no USD; if ALL of a task's runs are token-null the
task is absent from the cost CI vector). *(Caveat: the scripted CI corpus is usage-absent → token
metrics are explicit null. Cost rollup therefore only produces non-null numbers on `--agent=claude`
runs that carry a provider usage block. The leaderboard's cost column will render `—` for a
scripted-only run; that is correct, not a bug.)*

### USD-per-result formula (D-12/D-14)
```
usd = ( tokens_input            * input_per_mtok
      + tokens_input_cached_read * cached_input_per_mtok
      + tokens_input_cache_write * input_per_mtok          // D-14 v1 approximation — TODO cache_write column
      + tokens_output           * output_per_mtok
      ) / 1_000_000
```
Each term is **skipped if its token field is nil** (not treated as 0 in a way that hides missing data;
arithmetically a skipped term is +0, but the *presence* gate above decides whether the whole result
has a cost at all). The `cache_write` term is the documented conservative approximation (D-14): price
cache-write tokens at the **standard `input_per_mtok`** rate with a clearly-commented `// TODO(D-14):
add cache_write_per_mtok column; v1 prices cache-write at input rate (conservative)`.

> **Confirmation of D-14 approximation:** real Anthropic prompt-caching prices cache *writes* at
> ~1.25× the base input rate (5-minute cache) and cache *reads* at ~0.1× — so pricing cache-write at
> 1.0× input is an **under**-estimate (conservative for cost, as D-14 claims). `[ASSUMED]` (provider
> pricing structure from training knowledge — flag A1; the *direction* of the approximation matters
> for the "conservative" label, verify against current Anthropic pricing before publishing headline
> cost numbers). The v1 code path is correct regardless; only the comment's "conservative" claim
> depends on this.

### cost_per_solved_task (COST-02)
```
cost_per_solved_task(mode) = Σ_{tasks with task_success==true} USD(task) / count(task_success==true)
```
"solved" = `metrics.task_success == true`. Per-task USD for a multi-run cell = **mean USD over that
task's N runs** (so the per-task value is comparable across tasks with the same N). A task with no
solved run contributes neither to the numerator nor the denominator. If no task is solved,
`cost_per_solved_task` is **null/omitted** (division by zero → `—`, not NaN).

### Cost CI (COST-03)
The cost_quality.md CI is a BCa interval over the **per-task USD vector** (one mean-USD per solved
task), via Pattern 1. Same `BCaInterval` call as every other metric.

### Cost-table loader + freshness gate (D-13)
Reuse the EXACT shape from `cmd/helix-bench/validate_cost_table.go:25-42` (`CostRow`, `CostTable`)
but in a reusable home. **The types currently live in `package main` (cmd/helix-bench) — they are not
importable by `bench/aggregator/`.** Two options for the planner:
- **(preferred)** Move `CostRow`/`CostTable` + the `validateCostTable` freshness logic into a new
  exported `bench/cost` package, and have BOTH `validate_cost_table.go` and the aggregator import it
  (DRY — one freshness gate, one parser).
- (fallback) Duplicate the small struct in `bench/aggregator/cost.go` (acceptable but risks drift; the
  freshness constants `stalenessWindowDays = 90` and `dateLayout = "2006-01-02"` must match).

**Freshness gate at aggregate time (fail-closed, D-13):** when joining a result's `model_id` to a
cost row, fail closed if the matched row's `valid_until` is in the past OR `last_verified` is >90 days
stale, using an injected `today` (`time.Now().UTC()` from the CLI; a fixed date in tests — mirrors
`FairnessContract.DeprecationGate` and `validateCostTable`'s injected-clock discipline). A `model_id`
with no matching cost row is also a hard error (cannot price an unknown model). Per the project's
reproducibility ethos, prefer injecting `today` so a `--run-id` regeneration is deterministic.

### COST-02 hand-computed golden example (for the test)
Construct a fixture: model `claude-sonnet-4-5-20250929` (cost-table:
input=3.0, output=15.0, cached=0.30 USD/MTok). Two solved tasks:

| task | tokens_input | cached_read | cache_write | tokens_output | USD |
|------|-------------|-------------|-------------|---------------|-----|
| T1 | 1,000,000 | 0 | 0 | 100,000 | 3.0 + 0 + 0 + 1.5 = **4.50** |
| T2 | 500,000 | 200,000 | 100,000 | 50,000 | 1.5 + 0.06 + 0.30 + 0.75 = **2.61** |

`cost_per_solved_task = (4.50 + 2.61) / 2 = ` **3.555 USD**. Hard-code this in the COST-02 golden test
(`[VERIFIED: hand-computed against D-12/D-14 formula + committed cost-table.yaml rates]`). Note T2's
cache_write term `100,000 * 3.0 / 1e6 = 0.30` uses the input rate (D-14).

## Report Rendering (D-15/D-16/D-17, STATS-04/COST-03)

### leaderboard.md (D-16)
- **Rows:** (mode × benchmark). This phase: benchmark is always `internal-toolbench`, so rows are
  one-per-mode in practice. Keep the (mode × benchmark) key so Phase 89 can add rows.
- **Columns:** headline metrics each as `value [lo, hi]` (BCa CI). Recommended set (from
  `evaluators.Metrics`): `task_success` (success-rate), `pass@1`, `pass@N`, `tokens_input`,
  `tokens_output`, `tool_calls`, `files_read`, `edit_locality`. A null-CI metric renders `—`.
- **Sort:** by primary metric `task_success` descending (D-16).
- **Footer:** record `seed`, `bootstrap_iterations` (10000), `ci_level` (0.95), `runs` (N), and the
  cost-table `valid_until` (reproducibility + provenance).

### STATS-04 overlap gate (D-17)
For each metric column, walk **adjacent rows in the sorted order** (the "neighbors"). Two CIs
`[lo_a, hi_a]` and `[lo_b, hi_b]` **overlap** iff:
```
overlap := lo_a <= hi_b && lo_b <= hi_a   // standard interval-intersection test
```
When a higher-ranked row's CI overlaps the next row's CI for the *sort metric*, annotate the cell
(e.g. append `⚠ CI overlap — no X>Y claim`) and **suppress any directional superiority statement**
for that pair. Render the annotation inline in the table or as a footnote keyed to the row pair.
**Acceptance fixture:** craft two synthetic modes whose `task_success` CIs deliberately overlap
(e.g. mode A `0.70 [0.55, 0.85]`, mode B `0.60 [0.45, 0.75]` → overlap because `0.55 ≤ 0.75`), assert
the rendered markdown contains the overlap warning; and a non-overlapping pair (A `0.90 [0.85,0.95]`,
B `0.50 [0.40,0.60]`) asserts NO warning.

### cost_quality.md (COST-03 + FAIR-03 variance, D-15)
- **Rows:** (mode × benchmark) with `cost_per_solved_task value [lo, hi]` (BCa CI over per-task USD).
- **FAIR-03 > 5 % between-run variance warning (D-15):** for each (task, mode) compute between-run
  variance of the cost-relevant usage (the N runs' token totals / USD). The FAIR-03 acceptance is
  "a synthetic high-variance trace produces a fairness warning in cost_quality.md." Define the
  detector as: **coefficient of variation** (`stddev / mean`) of per-run USD (or per-run total tokens)
  across the N runs of a (task, mode) > 0.05 → emit a fairness warning row/footnote naming the cell.
  This consumes STATS-01's N≥3 (you need ≥2 runs for variance; N≥3 makes it meaningful). Document the
  exact statistic chosen (CV of per-run USD is the recommended, cost-aligned choice).
  **Acceptance fixture:** synthesize a (task, mode) with N=3 runs whose token totals vary >5% (CV>0.05)
  → assert the warning renders; and a low-variance cell → assert no warning.
- **Provenance:** cite the cost-table `valid_until` in the report (COST-04 pattern, parallel to D-16
  footer).

### Markdown determinism
Sort EVERYTHING (rows by sort-metric then mode name as tiebreaker; columns fixed order; footer fields
fixed order). `fairnessBlock` (`result.go:185`) already establishes the "sort before emit for
byte-stable output" precedent — follow it so `helix-bench aggregate` regenerates byte-identical
reports (REPORT-05 forward-compat).

## Common Pitfalls

### Pitfall 1: Naive or mis-looped pass@k
**What goes wrong (1a — naive):** Using `1 − (1 − c/n)^k`. **Why:** it is a biased estimator (treats
k draws as independent with replacement; the unbiased estimator is without-replacement combinatorial).
**What goes wrong (1b — c-vs-k loop count):** writing the product as `for i:=0;i<k;i++ { prod *= 1 -
k/(n-c+1+i) }` (looping **k** times) instead of `for i:=n-c+1;i<=n;i++ { prod *= 1 - k/i }` (looping
**c** times). The correct product `1 − Π_{i=n−c+1}^{n}(1 − k/i)` has exactly **c** terms (c = number
of successes), NOT k. The k-term loop yields 0.97348 for (10,3,5); the correct c-term loop yields
0.91667.
**How to avoid:** the locked `1 − C(n−c,k)/C(n,k)` c-term product form (D-10). Add a code comment
"term count is c (= number of successes), NOT k — looping k times is the classic wrong implementation."
**Warning sign:** pass@1 from the naive form (and from a k-term loop) still equals c/n (so a pass@1-only
test would NOT catch either bug) — the divergence appears at k≥2. The STATS-03 test MUST include a k≥2
reference value (e.g. n=10,c=3,k=5 → 0.91667; naive gives `1 − 0.7^5 = 0.83193`, a k-term loop gives
0.97348, both clearly different) AND assert product-form ≡ lgamma-form agreement (a k-term loop breaks
the agreement).

### Pitfall 2: Percentile bootstrap masquerading as BCa
**What goes wrong:** skipping z0 + a and just reading the `α/2` and `1−α/2` percentiles.
**Why:** "it looks like a CI." **How to avoid:** implement z0 (Step 2) and jackknife a (Step 3);
the STATS-02 test asserts BCa endpoints DIFFER from percentile endpoints on a skewed sample.
**Warning sign:** identical output to a percentile bootstrap on skewed data.

### Pitfall 3: Counting on-disk files as N
**What goes wrong:** "I found 2 rows, N=2, proceed" hides that the matrix should have produced 3.
**Why:** disk count == produced count, not expected count. **How to avoid:** read expected N from the
manifest/flag; gate `#valid < expectedN` (D-05). **Warning sign:** an aggregator that never errors on
a partial run.

### Pitfall 4: Fabricating 0 for nil metrics
**What goes wrong:** decoding a nil metric as 0 and feeding it to the mean/bootstrap.
**Why:** Go zero-value of `*int` deref or a sloppy default. **How to avoid:** keep `*int`/`*float64`
pointers (`deltas.go` `rowMetrics` pattern); exclude nil from the per-task and across-task vectors;
nil-across-all → null CI. **Warning sign:** a metric that should be null shows 0 with a tight CI.

### Pitfall 5: Non-deterministic bootstrap
**What goes wrong:** unseeded `rand` → different CI every run. **Why:** global `math/rand` or
`time.Now()` seed. **How to avoid:** seed from config (D-08), thread one `*rand.Rand`, record the seed
in the report. **Warning sign:** two `aggregate` runs over the same dir produce different CIs.

### Pitfall 6: BCa division-by-zero / ±∞ on degenerate samples
**What goes wrong:** `den == 0` in the jackknife a, or `Φ⁻¹(0)`/`Φ⁻¹(1)` = ±∞, or
`1 − a*(z0+zα) == 0`. **Why:** all-identical sample, m==1, or extreme a/z0. **How to avoid:** the
guards in the degenerate matrix (a=0 on zero spread; point CI on identical bootstrap; clamp the
denominator and α1/α2). **Warning sign:** NaN/Inf in a rendered CI.

### Pitfall 7: cache_write priced wrong / silently
**What goes wrong:** pricing cache-write at the cached-READ rate (0.30) instead of input (3.0), or
dropping it. **Why:** the cost-table has no cache-write column. **How to avoid:** D-14 — input rate,
clearly-commented TODO. **Warning sign:** cost numbers that ignore cache-write tokens entirely.

## Runtime State Inventory

> This is a greenfield additive phase (new `bench/aggregator/` package + one `ExpandMatrix` loop edit +
> one new subcommand). It is NOT a rename/refactor/migration. The five runtime-state categories are
> answered for completeness.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — the aggregator only READS `result.v2.json` rows and WRITES new markdown reports; it does not store keyed/queryable state. | None |
| Live service config | None — pure-function package, no daemon, no external service (D-01). | None |
| OS-registered state | None — no OS-level registrations. | None |
| Secrets/env vars | None new — no API keys (no LLM calls in the aggregator). `HELIX_BIN` is only for the optional E2E smoke. | None |
| Build artifacts | New package compiles into `cmd/helix-bench`; no stale artifacts. The `--runs` flag change is additive. | `go build ./cmd/helix-bench` after the `ExpandMatrix` signature change |

**Nothing found in categories 1–4:** verified by reading `deltas.go`, `matrix.go`, `cell.go`,
`result.go` — the data plane is read-result-rows / write-markdown only.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 80 single-run `ComputeAndWriteDeltas` (3 fixed deltas, 5-metric subset, 4 real modes) | Phase 82 multi-run aggregator (N runs, full metric set, BCa CIs, pass@k, cost) | This phase (D-03) | Aggregator supersedes deltas but does NOT delete it this phase; both coexist until Phase 89 consolidation |
| Hand-rolled Φ⁻¹ (Acklam/BSM) common in older Go stats code | `math.Erfinv` (stdlib) | Go ≥1.10 has `Erfinv` | No third-party stats dep; exact inverse-normal |
| Naive `1−(1−p)^k` pass@k (pre-2021) | HumanEval unbiased `1−C(n−c,k)/C(n,k)` | Chen et al. 2021 | The locked, citable, testable estimator (D-10) |

**Deprecated/outdated:**
- Naive pass@k: biased, fails published reference values. Never use (D-10).
- Percentile-only bootstrap as a stand-in for BCa: wrong on skew (D-06).

## Code Examples

### Φ and Φ⁻¹ from stdlib (BCa primitives)
```go
// Source: Go math stdlib (verified: math.Erfinv present, go1.26)
import "math"

func phiInv(p float64) float64 { return math.Sqrt2 * math.Erfinv(2*p-1) } // Φ⁻¹
func phi(z float64) float64    { return 0.5 * math.Erfc(-z/math.Sqrt2) }   // Φ
```

### BCa endpoint adjustment (the heart of D-06)
```go
// Source: Efron & Tibshirani (1993) eq. 14.10; transcribed.
func bcaPercentiles(z0, a, alpha float64) (alpha1, alpha2 float64) {
    zlo, zhi := phiInv(alpha/2), phiInv(1-alpha/2)
    adj := func(z float64) float64 {
        den := 1 - a*(z0+z)
        if math.Abs(den) < 1e-12 { return phi(z0 + (z0 + z)) } // guard (Pitfall 6)
        return phi(z0 + (z0+z)/den)
    }
    return clamp01(adj(zlo)), clamp01(adj(zhi))
}
```

### Cost-table join + freshness gate (D-13)
```go
// Source: cmd/helix-bench/validate_cost_table.go shape, reused.
func priceFor(ct CostTable, modelID string, today time.Time) (CostRow, error) {
    for _, r := range ct.Rows {
        if r.ModelID == modelID {
            vu, _ := time.Parse("2006-01-02", r.ValidUntil)
            if vu.Before(today.UTC().Truncate(24*time.Hour)) {
                return CostRow{}, fmt.Errorf("cost: model %s past valid_until %s", modelID, r.ValidUntil)
            }
            lv, _ := time.Parse("2006-01-02", r.LastVerified)
            if lv.Before(today.AddDate(0,0,-90)) {
                return CostRow{}, fmt.Errorf("cost: model %s last_verified %s >90d stale", modelID, r.LastVerified)
            }
            return r, nil
        }
    }
    return CostRow{}, fmt.Errorf("cost: no cost-table row for model_id %q", modelID)
}
```

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Anthropic prices cache-WRITE above input rate (so D-14's input-rate approximation is *conservative*) | Cost Rollup §D-14 | LOW for code (v1 path is correct either way); affects only the "conservative" wording in the comment/report. Verify against current Anthropic pricing before publishing headline cost numbers. |
| A2 | FAIR-03 variance detector statistic = CV (stddev/mean) of per-run USD > 0.05 | Report Rendering | MEDIUM — D-15 says ">5% between-run variance" but does not fix the exact statistic. CV-of-USD is a defensible, cost-aligned reading; the planner/discuss-phase may pin a different definition (e.g. CV of total tokens, or relative range). Confirm before locking the acceptance fixture. |
| A3 | Per-task USD for a multi-run cell = mean USD over the task's runs (vs sum) | Cost Rollup | MEDIUM — mean keeps tasks comparable; sum would scale with N. Recommend mean; confirm with planner. cost_per_solved_task itself is unambiguous (Σ over solved / count solved, D-12). |
| A4 | Expected-N source is a run-manifest written by `helix-bench run` (+ `--runs` override) | N≥3 Enforcement | LOW — D-05 mandates "from manifest/flag, not disk"; whether the manifest is a new file or the flag alone is the planner's call. Either satisfies D-05 as long as it is NOT disk-count. |
| A5 | Quantile rule for reading bootstrap endpoints (nearest-rank vs linear interp) | BCa §Step 5 | LOW — both are valid; pick ONE and lock for byte-stable reproduction. Does not affect coverage materially at B=10000. |

**Note:** No `[ASSUMED]` package names — all dependencies are stdlib or pre-existing direct deps,
so no `checkpoint:human-verify` install gate is needed.

## Open Questions (RESOLVED)

1. **Move vs duplicate the cost-table types? — RESOLVED: MOVE to an importable `bench/cost` package.**
   - What we know: `CostRow`/`CostTable`/`validateCostTable` live in `cmd/helix-bench` `package main`,
     not importable by `bench/aggregator`.
   - **Resolution:** MOVE `CostRow`/`CostTable` + the freshness logic into a new exported `bench/cost`
     package (Plan 82-01 owns this — `files_modified` includes `bench/cost/cost_table.go` and updates
     `cmd/helix-bench/validate_cost_table.go` to import it). Both `validate-cost-table` and the
     aggregator share one parser and one freshness gate (DRY). Low-risk refactor.

2. **FAIR-03 variance statistic exact definition (see A2)? — RESOLVED: CV of per-run USD > 0.05.**
   - What we know: ">5% between-run variance" is the requirement; acceptance is a synthetic
     high-variance trace producing a warning.
   - **Resolution:** FAIR-03 variance = **coefficient of variation (stddev/mean) of per-run USD across
     the N runs of a (task,mode), threshold > 0.05** → emit a fairness warning. The cost-quality plan
     encodes this statistic and its acceptance fixture (high-variance CV>0.05 warns; low-variance does
     not).

3. **Per-task USD aggregation for a multi-run cell (see A3)? — RESOLVED: mean USD over the task's runs.**
   - What we know: D-12 fixes `cost_per_solved_task = Σ(USD over solved)/count(solved)`; the per-task
     reduction for a multi-run cell was open.
   - **Resolution:** per-task USD = **mean USD over the task's N runs** (keeps tasks comparable across
     equal N; sum would scale with N). The cost plan and the COST-02 golden test use this reduction.

> (Note: the earlier "auto-invoke aggregator at RunMatrix tail" item is covered by D-02's "optionally"
> — the subcommand is the primary, re-runnable, testable entry point this phase; the tail auto-invoke
> is a non-blocking follow-up and is not required for any Phase 82 acceptance.)

## Environment Availability

> The aggregator is pure Go over the local filesystem. No external tools/services are required for the
> unit-testable core (D-01). Only the optional end-to-end smoke needs a real run.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | building + all unit tests | ✓ | go1.26.0 | — |
| `math.Erfinv` / `math.Lgamma` | BCa + pass@k | ✓ | stdlib go1.26 | none needed (verified present) |
| `gopkg.in/yaml.v3` | cost-table loader | ✓ | v3.0.1 (direct dep) | — |
| `HELIX_BIN` | optional E2E smoke only | n/a for unit tests | — | unit tests over synthetic fixtures (the primary test surface, D-01) |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** `HELIX_BIN` (only the optional smoke; all core tests are
HELIX_BIN-free per D-01).

## Validation Architecture

> nyquist_validation assumed enabled (no `workflow.nyquist_validation: false` found in config scope).

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (table-driven) |
| Config file | none — `go test` convention |
| Quick run command | `go test ./bench/aggregator/...` |
| Full suite command | `go test ./... && go vet ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STATS-01 | N-gate fail-closes when a cell has < expectedN valid rows | unit | `go test ./bench/aggregator/ -run TestNGate` | ❌ Wave 0 |
| STATS-02 | BCa CI: closed-form coverage + width sanity + BCa≠percentile on skew + determinism | unit | `go test ./bench/aggregator/ -run TestBCa` | ❌ Wave 0 |
| STATS-03 | pass@k matches published + hand-computed reference values; product form == lgamma form | unit | `go test ./bench/aggregator/ -run TestPassAtK` | ❌ Wave 0 |
| STATS-04 | overlap warning rendered for synthetic-overlap fixture, absent for non-overlap | unit | `go test ./bench/aggregator/ -run TestOverlapGate` | ❌ Wave 0 |
| COST-02 | cost_per_solved_task == hand-computed 3.555 for the golden fixture | unit | `go test ./bench/aggregator/ -run TestCostPerSolved` | ❌ Wave 0 |
| COST-03 | cost_quality.md renders cost+CI + FAIR-03 variance warning for sample run | unit | `go test ./bench/aggregator/ -run TestCostQualityRender` | ❌ Wave 0 |
| D-04 | ExpandMatrix emits N cells with RunIndex 0..N-1 | unit | `go test ./bench/runtime/ -run TestExpandMatrixRuns` | ❌ Wave 0 |
| D-08 | same seed ⇒ byte-identical reports | unit | `go test ./bench/aggregator/ -run TestDeterministic` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/aggregator/...`
- **Per wave merge:** `go test ./... && go vet ./...`
- **Phase gate:** full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `bench/aggregator/bootstrap_test.go` — STATS-02 (closed-form, width, BCa≠percentile, determinism)
- [ ] `bench/aggregator/passk_test.go` — STATS-03 (reference values + form agreement)
- [ ] `bench/aggregator/cost_test.go` — COST-02 (golden hand-computed) + freshness-gate fail-closed
- [ ] `bench/aggregator/report_test.go` — STATS-04 overlap + FAIR-03 variance + COST-03 render + determinism
- [ ] `bench/aggregator/load_test.go` — STATS-01 N-gate (deficient cell → hard error, no reports written)
- [ ] `bench/aggregator/testdata/` — synthetic `result.v2.json` fixtures (skewed, overlap, high-variance, golden-cost) + golden `.md` outputs
- [ ] `bench/runtime/matrix_test.go` extension — D-04 N-cell expansion
- [ ] Framework install: none — Go stdlib `testing` already in use

## Project Constraints (from CLAUDE.md)

- **Go single binary, CGO=1, no heavy deps** → this phase adds ZERO new deps (stdlib + existing yaml/cobra).
  The hand-rolled-algorithm ethos (~60-LOC PageRank) is honored: BCa ≈ 40 LOC, pass@k ≈ 15 LOC.
- **`go vet ./...` and `go test ./...` before completing any Go task** → both in the test map; the phase
  gate runs both.
- **GSD workflow enforcement** → file edits go through the GSD execute-phase flow.
- **SMTC-first tool routing** → for code navigation during implementation, prefer `mcp__smtc__*` over
  grep/Read (Go is first-class). No effect on the aggregator's runtime behavior.
- **modernc.org/sqlite (CGO-free) for FTS5** → not used by the aggregator (no DB); noted for context only.

## Sources

### Primary (HIGH confidence)
- Go `math` stdlib — `math.Erfinv`, `math.Lgamma`, `math.Erfc`, `math.Sqrt2` — verified by `go run` probe this session (`Erfinv(0.9)=1.163…`, `Lgamma(6)=4.787…`, go1.26.0).
- Live codebase reads: `bench/runtime/{deltas.go,matrix.go,cell.go,result.go}`, `bench/evaluators/metrics.go`, `bench/schema/result.v2.schema.json`, `bench/runners/fairness_contract.go`, `bench/datasets/cost-table.yaml`, `cmd/helix-bench/{main.go,validate_cost_table.go}` — field names, signatures, and path layout taken directly.
- Chen et al. 2021, "Evaluating Large Language Models Trained on Code" (arXiv:2107.03374) §2.1 — pass@k unbiased estimator + numerically-stable product form.
- Efron & Tibshirani (1993), "An Introduction to the Bootstrap" §14.3 / eqs. 14.10, 14.15 — BCa z0, jackknife acceleration, percentile adjustment.

### Secondary (MEDIUM confidence)
- leehanchung.github.io "Statistics for AI/ML Part 4: pass@k and Unbiased Estimator" — worked reference value n=10,c=3,pass@5 = 0.91667 (cross-checked against the c-term product form: (1−5/8)(1−5/9)(1−5/10)=1/12 → 1−1/12=11/12).

### Tertiary (LOW confidence)
- Anthropic prompt-caching pricing structure (cache-write ~1.25× input, cache-read ~0.1×) — training knowledge, flagged A1; affects only the "conservative" label on the D-14 approximation, not the code path.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all deps stdlib/existing, `Erfinv`/`Lgamma` probe-verified.
- BCa algorithm: HIGH — formulas from the standard reference; degenerate cases enumerated; test design specified.
- pass@k: HIGH — locked formula + stable c-term product form + multiple verified/hand-computed reference values (incl. the (10,3,5)→0.91667 k≥2 anchor recomputed by hand).
- Cost rollup: HIGH for formula/join (read from live schema + cost-table); MEDIUM on per-task USD aggregation choice (A3, now resolved to mean) and FAIR-03 variance statistic (A2, now resolved to CV of per-run USD).
- Integration points: HIGH — read directly from `matrix.go`/`cell.go`/`result.go`/`metrics.go`.
- Pitfalls: HIGH — each tied to a concrete acceptance test that catches it.

**Research date:** 2026-06-20
**Revised:** 2026-06-21 (BLOCKER fix: pass@k product form corrected to c-term loop `Π_{i=n−c+1}^{n}(1−k/i)`; Open Questions marked RESOLVED)
**Valid until:** 2026-07-20 (stable — pure-Go algorithms + locked decisions; the only volatile item is A1 provider pricing wording)
