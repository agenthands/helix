# Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard - Pattern Map

**Mapped:** 2026-06-20
**Files analyzed:** 14 (10 created, 1 modified, 3 created-or-moved test/fixture groups)
**Analogs found:** 14 / 14

This phase is additive Go: a new `bench/aggregator/` package (pure functions over a run dir), one
loop edit in `bench/runtime/matrix.go`, a moved/shared cost-table types package, and a new
`helix-bench aggregate` subcommand. Every new file has a strong in-repo analog — the aggregator is
"~150 lines of glue + arithmetic" (RESEARCH §Don't Hand-Roll) layered on proven patterns from
`bench/runtime/deltas.go`, `bench/runtime/cell.go`, and `cmd/helix-bench/validate_cost_table.go`.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/aggregator/aggregate.go` | service (orchestrator) | transform (read→reduce→render) | `bench/runtime/deltas.go` (`ComputeAndWriteDeltas`) | role-match (single→N-run generalization) |
| `bench/aggregator/load.go` | service (I/O + gate) | file-I/O (glob + decode + N-gate) | `bench/runtime/deltas.go` (`loadTaskRows`, group-by-task) | exact (same read+group+null pattern) |
| `bench/aggregator/bootstrap.go` | utility (pure math) | transform (BCa) | `internal/repomap` PageRank ethos + RESEARCH §BCa code examples | role-match (no existing stats analog) |
| `bench/aggregator/passk.go` | utility (pure math) | transform (estimator) | RESEARCH §pass@k product/lgamma forms | role-match (no existing analog) |
| `bench/aggregator/cost.go` | service (loader + join) | file-I/O + transform | `cmd/helix-bench/validate_cost_table.go` (`CostRow`/`CostTable`/`validateCostTable`) | exact (reuse shape + freshness gate) |
| `bench/aggregator/report.go` | service (renderer) | transform (→markdown) | `bench/runtime/deltas.go` write-back + `result.go` `fairnessBlock` sort-before-emit | role-match |
| `bench/cost/` shared types — **Open Q1 RESOLVED: move** | model/config | — | `cmd/helix-bench/validate_cost_table.go:25-107` (move verbatim) | exact (lift to exported pkg) |
| `cmd/helix-bench/aggregate.go` (subcommand) | route (cobra dispatch) | request-response (CLI) | `cmd/helix-bench/main.go:103-158` (`newRunCmd`) + `validate_cost_table.go:113` (`newValidateCostTableCmd`) | exact |
| `bench/runtime/matrix.go` **(MODIFY)** `ExpandMatrix` inner loop | service | — | itself, lines 145-155 (add `runs` axis) | exact (in-place) |
| `bench/aggregator/*_test.go` (5 files) | test | — | `bench/runtime/deltas_test.go` (table-driven, `BuildResult`/`Validate`/`writeModeRow`) | exact |
| `bench/aggregator/testdata/` (fixtures + golden .md) | test fixture | — | `bench/schema/testdata/result.v2.golden.json` + `deltas_test.go` `writeModeRow` | exact |

## Pattern Assignments

### `bench/aggregator/load.go` (service, file-I/O + N-gate)

**Analog:** `bench/runtime/deltas.go` — group-by-task + per-mode decode + nullable-pointer metrics +
explicit-null discipline. Generalize "one row per mode" → "N rows per (task, mode)".

**Group-by-task pattern** (`deltas.go:130-146`):
```go
byTask := make(map[string]map[string]string) // task -> mode -> resultPath
var taskOrder []string                        // deterministic order, not map iteration
for _, oc := range outcomes {
    task, mode := oc.Cell.Task, oc.Cell.Mode
    if oc.Result.ResultPath == "" { continue }
    if _, ok := byTask[task]; !ok { byTask[task] = make(map[string]string); taskOrder = append(taskOrder, task) }
    byTask[task][mode] = oc.Result.ResultPath
}
```
For the aggregator the inner value becomes `map[string][]string` (mode → N run-index paths) and the
source is a **glob of the run dir** (`<runDir>/<task>/<mode>/<run_index>/result.v2.json`) rather than
`[]CellOutcome` (D-01 reads from disk, not from a RunMatrix return).

**Durable path layout to glob** (`cell.go:310-317`, reuse as-is — NO new path code, RESEARCH §Reusable Assets):
```go
resultPath = filepath.Join(outDir, task, mode, strconv.Itoa(runIndex), "result.v2.json")
```

**Per-row decode preserving full doc + nullable metrics** (`deltas.go:234-255`):
```go
var doc map[string]json.RawMessage          // full row preserved verbatim for any write-back
json.Unmarshal(b, &doc)
var rm rowMetrics                            // nullable-pointer subset (never fabricate 0)
if raw, ok := doc["metrics"]; ok { json.Unmarshal(raw, &rm) }
```
**Critical:** the aggregator's `rowMetrics` must mirror `evaluators.Metrics` pointer types
(`metrics.go:19-38`) — `task_success *bool`, `tokens_input *int`, `tokens_output *int`,
`tokens_input_cached_read *int`, `tokens_input_cache_write *int`, `tool_calls *int`,
`files_read *int`, `edit_locality *float64`, … A nil field stays nil (Pitfall 4); it is excluded from
the per-task and across-task vectors, never decoded to 0.

**N-gate / valid-row = decode + `Validate()`** (`deltas_test.go:25` shows `Validate(b)` is the
schema-valid check): a row is "valid" iff it exists, JSON-decodes, AND `runtime.Validate(b) == nil`.

**Fail-closed gate (D-05, NEW logic, fail-closed shape mirrors `validateCostTable`):**
```go
// expectedN comes from --runs / run-manifest, NEVER len(glob) (Pitfall 3).
for _, taskMode := range scope {
    if validRowCount(taskMode) < expectedN {
        deficient = append(deficient, fmt.Sprintf("%s/%s: got %d want %d", task, mode, got, expectedN))
    }
}
if len(deficient) > 0 { return nil, fmt.Errorf("aggregate: insufficient runs: %v", deficient) } // write NOTHING
```

---

### `bench/aggregator/aggregate.go` (service, orchestrator)

**Analog:** `bench/runtime/deltas.go:124-178` (`ComputeAndWriteDeltas`) — the post-run barrier pass
that groups, computes, and writes back. The aggregator is the N-run + BCa-CI generalization (D-03).

**Pure-function signature** (Claude's discretion per D-01; recommended shape from RESEARCH §Structure):
```go
func Aggregate(runDir string, cfg Config) (*Report, error)
// cfg: ExpectedN int; Seed uint64; Iterations int (default/floor 10000); CILevel float64 (0.95);
//      KValues []int ({1, N}); Today time.Time (injected clock for the cost freshness gate)
```

**Two-level reduction (Pattern 1, D-07):** Level 1 reduce N runs of one (task,mode) → one per-task
scalar (mean for continuous, success-rate for boolean, `passAtK(N,c,k)`, mean-USD for cost). Level 2
bootstrap the per-task vector across tasks. A metric nil in ALL N runs of a task → per-task value
ABSENT (deltas.go null discipline, `deltas.go:85-90` `deref` returning `(float64, bool)`):
```go
func deref(p *float64) (float64, bool) { if p == nil { return 0, false }; return *p, true }
```

**Determinism:** record `seed`, `iterations`, `ci_level`, `runs`, cost-table `valid_until` in the
Report footer (D-08); sort everything before emit (mirror `result.go` `fairnessBlock` sort-before-emit).

---

### `bench/aggregator/bootstrap.go` (utility, pure BCa math)

**Analog:** no existing stats code — anchor on the repo's hand-rolled-numerics ethos (~60-LOC
PageRank, CLAUDE.md) and the exact algorithm + verified stdlib primitives in RESEARCH §BCa.

**Stdlib primitives (verified present, go1.26):**
```go
func phiInv(p float64) float64 { return math.Sqrt2 * math.Erfinv(2*p-1) } // Φ⁻¹
func phi(z float64) float64    { return 0.5 * math.Erfc(-z/math.Sqrt2) }   // Φ
```

**Seeded RNG (D-08, `math/rand/v2`):**
```go
rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9E3779B97F4A7C15))
// resample: idx := rng.IntN(len(vals)) for each of len(vals) draws, B times.
```

**BCa endpoint adjustment (Efron & Tibshirani 1993 eq. 14.10; RESEARCH §Code Examples):**
```go
func bcaPercentiles(z0, a, alpha float64) (a1, a2 float64) {
    zlo, zhi := phiInv(alpha/2), phiInv(1-alpha/2)
    adj := func(z float64) float64 {
        den := 1 - a*(z0+z)
        if math.Abs(den) < 1e-12 { return phi(z0 + (z0 + z)) } // Pitfall 6 guard
        return phi(z0 + (z0+z)/den)
    }
    return clamp01(adj(zlo)), clamp01(adj(zhi))
}
```
Implement z0 from `#{θ*_b < θ̂}/B` and `a` from jackknife skewness (RESEARCH §BCa Steps 2-3). Honor
the degenerate matrix: empty vector → null CI; all-identical → point CI `[θ̂,θ̂]`; m==1 → a=0; den≈0 →
percentile fallback. **Must NOT be a percentile bootstrap** (Pitfall 2 — STATS-02 test asserts BCa ≠
percentile on a skewed sample).

---

### `bench/aggregator/passk.go` (utility, pure estimator)

**Analog:** none in-repo — transcribe the LOCKED HumanEval form (D-10, RESEARCH §pass@k).

**PRIMARY (Chen et al. stable product form — the product has c terms, NOT k):**
```go
func passAtK(n, c, k int) float64 {
    if n-c < k { return 1.0 }
    prod := 1.0
    // term count is c (= number of successes), NOT k — looping k times is the
    // classic wrong implementation (yields 0.97348 for (10,3,5) instead of 0.91667).
    for i := n - c + 1; i <= n; i++ { prod *= 1.0 - float64(k)/float64(i) }
    return 1.0 - prod
}
```
This is `1 − Π_{i=n−c+1}^{n}(1 − k/i)`; the product runs over `n − (n−c+1) + 1 = c` terms. Hand-check:
(10,3,5) → i∈{8,9,10} → (1−5/8)(1−5/9)(1−5/10) = 0.375·0.444444·0.5 = 1/12 → 1−1/12 = **0.91667**. ✓

**CROSS-CHECK (lgamma, for the test only):** `passAtKLog = 1 - exp(logBinom(n-c,k) - logBinom(n,k))`
with `logBinom` via `math.Lgamma`. The test computes BOTH and asserts agreement ~1e-12 — and a k-term
loop would break that agreement.

**DO NOT** use `1 − (1−p)^k` (biased — Pitfall 1a; fails STATS-03 at k≥2) and DO NOT loop the product
k times instead of c times (Pitfall 1b — same wrong value 0.97348 for (10,3,5)). Reference values to
assert: `(10,3,5)→0.91667` (the load-bearing k≥2 anchor), `(5,1,1)→0.2`, `(5,2,2)→0.7`, `(n,c,1)→c/n`,
`(n,0,k)→0.0`. Keep at least one k≥2 value — k=1 cannot catch the c-vs-k loop bug.

---

### `bench/aggregator/cost.go` (service, loader + join + rollup)

**Analog:** `cmd/helix-bench/validate_cost_table.go` — reuse `CostRow`/`CostTable` shape, the
`yaml.NewDecoder` + `KnownFields(true)` strict decode, the injected-`today` freshness gate, and the
`stalenessWindowDays = 90` / `dateLayout = "2006-01-02"` constants.

**Loader + strict decode** (`validate_cost_table.go:55-61`):
```go
dec := yaml.NewDecoder(f); dec.KnownFields(true)
var ct CostTable
if err := dec.Decode(&ct); err != nil { return ... }
```

**Join + freshness gate (D-13, fail-closed; RESEARCH §Code Examples + `validate_cost_table.go:86-100`):**
```go
func priceFor(ct CostTable, modelID string, today time.Time) (CostRow, error) {
    for _, r := range ct.Rows {
        if r.ModelID != modelID { continue }
        vu, _ := time.Parse(dateLayout, r.ValidUntil)
        if vu.Before(today.UTC().Truncate(24*time.Hour)) { return CostRow{}, fmt.Errorf("cost: %s past valid_until %s", modelID, r.ValidUntil) }
        lv, _ := time.Parse(dateLayout, r.LastVerified)
        if lv.Before(today.AddDate(0,0,-stalenessWindowDays)) { return CostRow{}, fmt.Errorf("cost: %s last_verified %s >90d stale", modelID, r.LastVerified) }
        return r, nil
    }
    return CostRow{}, fmt.Errorf("cost: no row for model_id %q", modelID) // unknown model = hard error
}
```
The join key is the result's top-level `model_id` (set by `BuildResult` from
`runners.DefaultContract.ModelID`, `result.go:165`).

**USD formula (D-12/D-14):**
```go
usd = (ti*input + tcr*cached + tcw*input /*D-14 cache-write@input rate, TODO column*/ + to*output) / 1_000_000
```
Each term skipped if its token field is nil. Per-task USD for a multi-run cell = **mean USD over the
task's runs** (Open Q3 RESOLVED). cost_per_solved_task = Σ(USD over `task_success==true`) /
count(solved); no solved → null/`—`. Golden test target: 3.555 USD (RESEARCH §COST-02 hand example).

> **Open Q1 RESOLVED — MOVE the cost-table types to an importable `bench/cost` package.** The types
> currently live in `package main` (`cmd/helix-bench`), NOT importable. Plan 82-01 moves
> `CostRow`/`CostTable` + the freshness logic into `bench/cost/cost_table.go` and updates
> `validate_cost_table.go` to import it; the aggregator imports the same package (one parser, one gate).
> See Shared Pattern below.

---

### `bench/aggregator/report.go` (service, markdown renderer)

**Analog:** `deltas.go` write discipline + `result.go` `fairnessBlock` sort-before-emit precedent.

**Atomic write (reuse `cell.go:911` `writeDurable` pattern — temp-file + rename; markdown is NOT
schema-validated, only the optional deltas-with-CIs write-back into result.v2 goes through `Validate`):**
```go
// stage to .tmp-<base>-* in the same dir, then os.Rename — see cell.go:911-937
```

**STATS-04 overlap gate (D-17, NEW; RESEARCH §Report Rendering):**
```go
overlap := lo_a <= hi_b && lo_b <= hi_a // adjacent rows, sort metric → annotate "⚠ CI overlap — no X>Y claim"
```

**FAIR-03 variance warning (D-15):** CV = stddev/mean of per-run USD across a (task,mode)'s N runs >
0.05 → warning row (Open Q2 RESOLVED — CV of per-run USD is the locked statistic). **Determinism:**
sort rows (sort-metric desc, mode tiebreaker), fixed column + footer order (mirror `result.go`
`fairnessBlock`).

---

### `cmd/helix-bench/aggregate.go` (route, cobra subcommand)

**Analog:** `cmd/helix-bench/main.go:103-158` (`newRunCmd`) for flag/RunE shape;
`validate_cost_table.go:113-133` (`newValidateCostTableCmd`) for the thin `RunE` returning the error.

**Subcommand shape:**
```go
func newAggregateCmd() *cobra.Command {
    var runs int; var seed uint64; var iters int
    cmd := &cobra.Command{
        Use:  "aggregate <run_dir>",
        Args: cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            rep, err := aggregator.Aggregate(args[0], aggregator.Config{ExpectedN: runs, Seed: seed, Iterations: iters, Today: time.Now().UTC()})
            if err != nil { return err } // fail-closed: non-zero exit, no reports written
            fmt.Fprintf(cmd.OutOrStdout(), "aggregate: wrote leaderboard.md + cost_quality.md\n")
            return nil
        },
    }
    cmd.Flags().IntVar(&runs, "runs", 3, "expected runs per (task,mode); fail-closed if any cell has fewer")
    // seed, iterations(default 10000), ci-level(0.95) flags...
    return cmd
}
```
Register via `root.AddCommand(newAggregateCmd())` at `main.go:75-79`. Expected-N from `--runs` /
run-manifest, NEVER disk count (D-05/Pitfall 3).

---

### `bench/runtime/matrix.go` — MODIFY `ExpandMatrix` (service)

**Analog:** itself (`matrix.go:145-155`). Add a `runs int` parameter (default 3); the innermost
append emits N cells with `RunIndex 0..runs-1`. ALL downstream path machinery is already RunIndex-aware
(`Cell.RunIndex` doc `matrix.go:35-40`, `runOneCell` threads `c.RunIndex` `matrix.go:266`,
`cellDurablePaths` formats the segment `cell.go:310`, `writeDurable` atomic `cell.go:911`).

**Current (line 150):**
```go
cells = append(cells, Cell{Benchmark: b, Language: l, Mode: m, Task: t})
```
**Change to:**
```go
for r := 0; r < runs; r++ {
    cells = append(cells, Cell{Benchmark: b, Language: l, Mode: m, Task: t, RunIndex: r})
}
```
Thread `--runs N` from `newRunCmd` (`main.go:103`, add an `IntVar`) → `runBenchOpts` →
`ExpandMatrix` call (`main.go:218`). `go build ./cmd/helix-bench` after the signature change.

---

### `bench/aggregator/*_test.go` + `testdata/` (test)

**Analog:** `bench/runtime/deltas_test.go` — table-driven, builds schema-valid fixtures via
`BuildResult(ResultInput{...})` + `Validate`, writes to the `<task>/<mode>/<run_index>/` layout.

**Fixture writer pattern** (`deltas_test.go:16-33` `writeModeRow`, adapt run_index from "0" to 0..N-1):
```go
b, _ := BuildResult(ResultInput{TaskID: task, Mode: mode, Benchmark: "internal-toolbench", RunIndex: r, Outcome: "pass", Metrics: m})
require.NoError(t, Validate(b))
dir := filepath.Join(outDir, task, mode, strconv.Itoa(r)); os.MkdirAll(dir, 0700)
os.WriteFile(filepath.Join(dir, "result.v2.json"), b, 0600)
```
**Golden-file pattern:** `bench/schema/testdata/result.v2.golden.json` — keep synthetic result.v2
fixtures (skewed, overlap, high-variance, golden-cost) + golden `.md` outputs under
`bench/aggregator/testdata/`. **D-01: these are pure unit tests — NO `HELIX_BIN` gate** (unlike
`five_of_six_test.go` / `no_semantic_store_on_test.go`); only an optional E2E smoke needs it.

## Shared Patterns

### Explicit nulls, never fabricated 0 (D-09, Pitfall 4)
**Source:** `bench/runtime/deltas.go:65-90` (`rowMetrics` pointers + `deref` returning `(v, ok)`).
**Apply to:** `load.go`, `aggregate.go`, `bootstrap.go`, `cost.go`. A nil metric → excluded from
vectors → nil-across-all → null CI rendered `—`. Never decode `*int`/`*float64` to 0.

### Atomic durable write (temp-file + rename)
**Source:** `bench/runtime/cell.go:911-937` (`writeDurable`).
**Apply to:** `report.go` (leaderboard.md, cost_quality.md) and any result.v2 write-back. Markdown is
written atomically but NOT schema-validated; result.v2 write-back is re-`Validate()`d first.

### Schema re-validate after write-back
**Source:** `bench/runtime/result.go:204` (`Validate`) + `deltas.go:221-223` (Validate before writeDurable).
**Apply to:** ONLY if the planner chooses the deltas-with-CIs-into-result.v2 path (D-03). Reports are markdown.

### Injected-clock freshness gate (fail-closed)
**Source:** `cmd/helix-bench/validate_cost_table.go:49-107` (injected `today`, 90d window, dateLayout).
**Apply to:** `cost.go` `priceFor` — accept `today time.Time` (from CLI `time.Now().UTC()`, fixed date
in tests) so a `--run-id` regeneration is deterministic. Past `valid_until` / >90d `last_verified` /
unknown model_id → hard error → no reports.

### Sort-before-emit for byte-stable output
**Source:** `bench/runtime/result.go:185` (`fairnessBlock`) + `ExpandMatrix` deterministic ordering
(`matrix.go:107-109`). **Apply to:** `report.go` (rows, columns, footer) and the seeded RNG (D-08) so
`helix-bench aggregate` regenerates byte-identical reports.

### Cobra subcommand wiring
**Source:** `main.go:103-158` (`newRunCmd`) + `validate_cost_table.go:113` + `main.go:75-79`
(`AddCommand`). **Apply to:** `cmd/helix-bench/aggregate.go`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `bench/aggregator/bootstrap.go` (BCa core) | utility | transform | No existing statistics code in-repo; anchor on hand-rolled-numerics ethos + RESEARCH exact algorithm. Stdlib `math.Erfinv`/`Erfc`/`Lgamma` cover the primitives — no third-party dep. |
| `bench/aggregator/passk.go` (estimator) | utility | transform | No existing estimator; transcribe the LOCKED Chen et al. c-term product form (D-10) per RESEARCH. |

(Both are pure math with closed-form acceptance tests; RESEARCH provides verified reference values and
the exact formulas, so "no analog" does not mean "uncertain" — it means the pattern source is the
research doc, not another file.)

## Metadata

**Analog search scope:** `bench/runtime/`, `bench/evaluators/`, `bench/schema/`, `bench/datasets/`,
`bench/runners/`, `cmd/helix-bench/`.
**Files scanned (read):** `deltas.go`, `matrix.go`, `cell.go` (path/write sections), `result.go`
(signatures), `metrics.go`, `validate_cost_table.go`, `main.go` (run subcommand), `deltas_test.go`.
**Pattern extraction date:** 2026-06-20
**Revised:** 2026-06-21 (BLOCKER fix: passk product form corrected to c-term loop `for i := n-c+1; i <= n; i++`; Open Qs 1/2/3 marked RESOLVED in the cost/report/passk sections)
