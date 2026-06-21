# Phase 89: Reports, CI Policy & Contamination Canary - Research

**Researched:** 2026-06-21
**Domain:** Deterministic markdown report rendering (Go), CI cost-policy workflow (GitHub Actions), contamination-canary exclusion wiring
**Confidence:** HIGH (all anchors located and read in-tree; zero external deps; no live run required)

## Summary

Phase 89 is the v1.12 publication capstone. It is **pure logic + static files over EXISTING `result.v2` rows** — no live benchmark run, no network, no new go.mod dependency. Everything it needs already exists in the tree and is hermetically testable against committed goldens. The work is four-fold:

1. **Two new report renderers** (`per_language.md`, `ablations.md`) in `bench/aggregator/` in the EXACT Phase 82 deterministic style (sort-before-emit, fixed precision, em-dash null discipline, atomic temp+rename write, footer block). The `ByLanguage` slice (Phase 85) and the `ablation_deltas` machinery (Phase 80 `bench/runtime/deltas.go`) already produce the substrate; the renderers consume them.
2. **Wire the `helix-bench report --run-id <id>` subcommand** — it is currently a `notYetImplemented("report")` stub at `cmd/helix-bench/main.go:419-426` (already registered, so subcommand count stays 6). `--run-id` resolves to `bench/reports/<run-id>/` (the same run-scoped durable tree `aggregate` consumes — confirmed at `cmd/helix-bench/main.go:251-253`). `report` regenerates ALL 4 reports byte-identically by re-running the aggregator render path over that tree.
3. **CI cost-policy workflow** (`.github/workflows/bench.yml` new) — `make bench-quick` on PR (hermetic, scripted-agent, ≤5-min hard cap via `timeout-minutes`), full `make bench` nightly + maintainer-label-gated. Both targets already exist (Phase 77). Verified by YAML parse/lint test + inspection, NOT a live CI run.
4. **The REAL contamination canary** — the Phase 86 `bench/canary` probe + the aggregator `CanaryPassRate` column already exist. Phase 89 adds (a) a `leaderboard.md` footnote listing contaminated tasks and (b) **exclusion of flagged tasks from headline numbers at aggregate-time** (the `successCount`/reduce path must skip rows where `rowCanary` reports contaminated). A synthetic contaminated-response test trips the flag.

**Two real gaps the planner MUST close (not in any prior phase):**
- **`verified_correctness` leaderboard column is NOT rendered today.** `rowMetrics.VerifiedCorrectness` is decoded in `load.go:32` but has **no reduce function and no column** in `report.go`. SC#1 (REPORT-01) explicitly requires `(mode × benchmark) → pass@1, verified_correctness, cost_per_solved`. The planner must add a `reduceVerifiedCorrectness` (mirroring `reduceCanaryRate`'s pooled-rate path) and a leaderboard column. This regenerates the leaderboard golden.
- **Adding ANY column / footnote / canary exclusion regenerates `leaderboard.golden.md`** (and possibly `cost_quality.golden.md`). The current goldens were deliberately frozen byte-for-byte across Phases 85/86/87 by populating-but-not-rendering the additive columns. Phase 89 is the phase that finally renders them, so the golden churn is expected and correct — but every prior test that asserts "additive column perturbs nothing" (e.g. `TestSwebenchColumnsGoldenStable`, `TestAggregateCanaryAbsentIsEmDash`'s golden guard) must be updated in lockstep.

**Primary recommendation:** Implement all four report renderers as pure functions in `bench/aggregator/report.go` (reusing `renderFooter`, `fmtCI`, `writeReport`, sort helpers verbatim), add the missing `verified_correctness` reduce + the canary exclusion + footnote in `aggregate.go`, wire `report --run-id` as a thin cobra RunE resolving `<out>/<run-id>/` and calling the shared render path, author one new `.github/workflows/bench.yml`, and prove all of it with committed golden `.md` fixtures + a YAML-parse test. Regenerate the goldens with `-update` and review the diff.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Render 4 report `.md` files | `bench/aggregator/` (pure render layer) | — | `report.go` already owns leaderboard/cost render; per_language/ablations belong beside them for shared sort/footer/atomic-write helpers |
| `report --run-id` CLI surface | `cmd/helix-bench/` (cobra RunE) | `bench/aggregator` (Aggregate/render) | The stub lives here; thin RunE wraps the pure orchestrator (mirrors `aggregate.go` exactly) |
| Canary contamination flag (per row) | `bench/canary/` (pure probe) | `bench/aggregator` (score-time reduce) | `IsContaminated` is a leaf primitive; exclusion+footnote is an aggregate-time policy |
| Headline exclusion of contaminated tasks | `bench/aggregator/aggregate.go` (reduce path) | — | Exclusion must happen where `successCount`/`reduceLeaderRow` build the headline vectors |
| Ablation delta substrate | `bench/runtime/deltas.go` (single-run write-back) | `bench/aggregator` (multi-run BCa render) | deltas.go writes `ablation_deltas` per-row; the report re-reduces across runs with BCa+overlap |
| CI cost-policy gating | `.github/workflows/bench.yml` (static YAML) | `Makefile` (`bench-quick`/`bench`) | Targets exist; the workflow is the policy wrapper |

## Standard Stack

This phase introduces **ZERO new dependencies**. Everything is stdlib + existing in-tree packages.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `fmt`/`sort`/`strings`/`math` | go 1.25.1 (module); toolchain go1.26.0 | report rendering, deterministic sort | `report.go` already uses exactly these; no new imports needed |
| Go stdlib `os`/`path/filepath` | — | atomic temp+rename write, run-dir resolution | `writeReport` (report.go:362) is the pattern to reuse verbatim |
| `github.com/spf13/cobra` | v1.9.1 (in tree) | `report` subcommand RunE | The `report` stub + `aggregate.go` already use it |
| `bench/aggregator` (in-tree) | — | `Aggregate`, `Report`, `LeaderRow`, `ByLanguage`, render helpers | The render layer this phase extends |
| `bench/canary` (in-tree) | — | `Sentinel`, `IsContaminated`, `DocKeyCompletion`, `DocKeyContaminated`, `InjectPrompt` | The contamination probe (Phase 86) |
| `bench/runtime` (in-tree) | — | `deltas.go` `ablation_deltas`, `SwebenchRawResolvedKey`/`SwebenchRescoredVerifiedKey` consts | Delta + swebench-key substrate |
| `bench/cost` (in-tree) | — | `CostTable`, `LoadCostTable`, `PriceFor`, `valid_until` | cost_quality.md scatter + valid_until citation |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | in tree | golden-diff assertions | All hermetic report tests (`assert.Equal(want, got)`) |
| `gopkg.in/yaml.v3` or stdlib | check before use | YAML parse-lint test for the new workflow | The INFRA-04 workflow-file validity test |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| ASCII scatter in `cost_quality.md` | SVG embed | REPORT-04 acceptance says "ASCII / svg" — ASCII is dependency-free, byte-deterministic, GitHub-renderable in a fenced block. Recommend ASCII (bucketed cost-vs-correctness grid). SVG would need fixed coordinate rounding for byte-stability; not worth it. |
| New `report` orchestrator | Extend `Aggregate` to render all 4 | `Aggregate` currently renders only 2. Recommend factoring the render+write step into a shared `renderAll(rep, runDir)` that BOTH `aggregate` and `report` call, so the 4-report set is byte-identical regardless of entry point (REPORT-05). |

**Installation:** None — no `go get`. Confirm no new deps with `go list -deps ./bench/aggregator/... | sort -u` after edits (the Phase 82/85/86 leaf-discipline tests already guard this).

**Version verification:** Module is `go 1.25.1` (go.mod), local toolchain `go1.26.0`. No registry packages added, so no `npm view`/`pip index` step applies. The package-legitimacy gate is **N/A** for this phase (zero external packages installed).

## Package Legitimacy Audit

**Not applicable — this phase installs ZERO external packages.** All code reuses in-tree packages (`bench/aggregator`, `bench/canary`, `bench/runtime`, `bench/cost`) and Go stdlib. No `go get`, no go.mod change. The Phase 82/83/86 leaf-import vet gates (`go list -deps` cleanliness) already enforce that no new dependency sneaks in.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REPORT-01 | `leaderboard.md`: `(mode × benchmark) → pass@1, verified_correctness, cost_per_solved` with BCa CIs + non-overlap markers | `renderLeaderboard` (report.go:223) ALREADY emits pass@1 + CIs + STATS-04 overlap gate. **GAP: must ADD a `verified_correctness` column** — `rowMetrics.VerifiedCorrectness` (load.go:32) is decoded but unreduced/unrendered. Add `reduceVerifiedCorrectness` (mirror `reduceCanaryRate` pooled-rate). `cost_per_solved` lives in cost_quality.md today; REPORT-01 wants it on the leaderboard too — surface the `CostRow.CostPerSolved` value as a leaderboard column. |
| REPORT-02 | `per_language.md`: per-language slice; no-coverage language = `n/a`, not omitted | `rep.ByLanguage []LanguageRow` (Phase 85, report.go:114-118 + reduceLanguageRows aggregate.go:255) already produces `{Language, PassRate, N}`. **New renderer** `renderPerLanguage` iterates a FIXED list of all 8 Tier-1 languages; a language absent from `ByLanguage` renders `n/a` (NOT skipped). |
| REPORT-03 | `ablations.md`: `full vs {no_lsp,no_semantic,no_structured_edit,baseline_plain,baseline_rag}` delta tables with CI overlap analysis | `deltaComparisons` (deltas.go:52-61) names 4 of the 5 (`baseline_plain`, `no_lsp`, `no_structured_edit`, `baseline_rag`). **GAP: `no_semantic` is a deliberate NON-operand in deltas.go** (it is the partial arm). The report must compute its delta at AGGREGATE time (full mode's BCa CI vs no_semantic mode's BCa CI, with `ciOverlap` analysis) rather than relying on the single-run `ablation_deltas` write-back. Recommend: `renderAblations` reduces each (full, other) pair across runs via the existing `bca`+`ciOverlap` and renders a delta + overlap marker per comparison. |
| REPORT-04 | `cost_quality.md`: cost-per-solved scatter (cost vs verified_correctness) + cites cost-table `valid_until` | `renderCostQuality` (report.go:302) ALREADY emits cost_per_solved + FAIR-03 variance + `cost_table_valid_until` in the footer. **Extend** with an ASCII scatter (bucketed cost on one axis, verified_correctness on the other). The `valid_until` citation already exists via `costTableValidUntil` (aggregate.go:624). |
| REPORT-05 | `helix-bench report --run-id <id>` regenerates all 4 byte-identically (`diff` empty) | Wire the `notYetImplemented("report")` stub (main.go:419). `--run-id` → `bench/reports/<id>/` (run-scoped tree, confirmed main.go:251-253). RunE calls the shared `renderAll` over that tree. Byte-reproducibility proven by a hermetic golden test: render twice, assert identical, AND diff against committed 4 goldens. |
| INFRA-04 | CI policy: `make bench-quick` on PR (ToolBench Go-only, ≤5min, no LLM cost); full `make bench` nightly/maintainer-gated; documented budget | New `.github/workflows/bench.yml`. `bench-quick` already hermetic+scripted (Makefile:137-146, no API key, D-01). PR job: `make bench-quick` with `timeout-minutes: 5`. Nightly/dispatch job: `make bench` gated on a maintainer label (`if: contains(github.event.pull_request.labels.*.name, 'run-full-bench')` or `schedule:`/`workflow_dispatch`). Budget documented in `bench/BENCH.md`. |
| INFRA-05 | Contamination canary: known-novel pattern in select tasks; synthetic contaminated-response trips flag; flagged tasks in `leaderboard.md` footnote + excluded from headline | `bench/canary` (`Sentinel`/`InjectPrompt`/`IsContaminated`) + `rowCanary`/`reduceCanaryRate` exist. **Add:** (a) aggregate-time exclusion — `successCount` and the metric reduces must SKIP rows where `rowCanary` reports contaminated; (b) a `leaderboard.md` footnote section listing contaminated `(task, mode)` cells; (c) wire `InjectPrompt` into select tasks (currently it has NO production caller). Synthetic test: a row with `completion` echoing `Sentinel` is excluded + footnoted. |

## Architecture Patterns

### System Architecture Diagram

```
                       helix-bench report --run-id <id>
                                   │
                                   ▼
                    resolve <out>/<run-id>/  (default bench/reports/<id>/)
                                   │
                                   ▼
              ┌─────────────────────────────────────────────┐
              │  bench/aggregator.Aggregate(runDir, cfg)     │
              │   (PURE; ONE seeded PCG RNG; fail-closed)    │
              │                                              │
              │  Load(runDir, expectedN)  ──► []Row per      │
              │     (fail-closed N-gate)      (task,mode)    │
              │                                              │
              │  ── EXCLUSION (NEW, INFRA-05) ──             │
              │  drop rows where rowCanary()==contaminated   │
              │  from the HEADLINE vectors; remember them    │
              │  for the footnote                            │
              │                                              │
              │  Level-1 reduce (per-task scalar)            │
              │  Level-2 BCa CI (across-task, seeded RNG)    │
              │   + reduceVerifiedCorrectness (NEW)          │
              │   + reduceCanaryRate / reduceSwebenchScores  │
              │   + reduceLanguageRows (ByLanguage)          │
              │                                              │
              │  Report{Leaderboard, Cost, ByLanguage,       │
              │         Footer, contaminated[]}              │
              └──────────────────────┬───────────────────────┘
                                     │ renderAll(rep, runDir)  (NEW shared step)
            ┌────────────┬───────────┼────────────┬───────────────┐
            ▼            ▼           ▼            ▼                ▼
   leaderboard.md  per_language.md ablations.md cost_quality.md  (footnote
   (+verified_     (8 langs;       (5 deltas;   (+ASCII scatter;  in
    correctness;    n/a for        full vs each; valid_until)     leaderboard.md)
    +contam         no-coverage)   ciOverlap)
    footnote)
            └──────── all 4 written atomically (temp+rename) ────────┘
                                     │
                                     ▼
              byte-identical on re-run  (golden .md fixtures = proof)
```

File-to-implementation mapping is in the Component Responsibilities below, not in the diagram.

### Component Responsibilities

| File | Change | Responsibility |
|------|--------|----------------|
| `bench/aggregator/report.go` | EXTEND | Add `renderPerLanguage`, `renderAblations`, ASCII scatter in `renderCostQuality`, `verified_correctness` + `cost_per_solved` leaderboard columns, contamination footnote. Reuse `fmtCI`/`renderFooter`/`writeReport`/sort helpers verbatim. |
| `bench/aggregator/aggregate.go` | EXTEND | Add `reduceVerifiedCorrectness`; canary EXCLUSION in `successCount`/reduce path; collect contaminated `(task,mode)` for the footnote; factor render+write into `renderAll(rep, runDir)`. |
| `cmd/helix-bench/main.go` (or new `report.go`) | WIRE | Replace `notYetImplemented("report")` with a RunE resolving `<out>/<run-id>/` and calling `renderAll`. Mirror `cmd/helix-bench/aggregate.go` flag/clock/fail-closed pattern. |
| `.github/workflows/bench.yml` | NEW | PR job `make bench-quick` (`timeout-minutes: 5`); nightly/dispatch job `make bench` (maintainer-gated). |
| `bench/aggregator/testdata/*.golden.md` | REGEN | 4 goldens (leaderboard regen for new cols/footnote; per_language new; ablations new; cost_quality regen for scatter). |
| `bench/runners/.../task` injection point | WIRE | Call `canary.InjectPrompt` into SELECT tasks (currently no production caller). |
| `bench/BENCH.md` | DOC | Document the CI cost budget + the canary policy. |

### Pattern 1: Deterministic render (Phase 82 discipline — REUSE EXACTLY)
**What:** Sort-before-emit, fixed `%.4f` precision, em-dash for null CI, atomic temp+rename, fixed-order footer. No RNG in the render layer; all bootstrap RNG is the ONE seeded PCG threaded through `bca()` in the reduce path.
**When to use:** Every new renderer (`per_language.md`, `ablations.md`) and the scatter.
**Example:**
```go
// Source: bench/aggregator/report.go:186-217 (in-tree)
func fmtCI(c ciValue) string {
	if !c.OK {
		return emDash // "—" — NEVER a fabricated 0
	}
	return fmt.Sprintf("%.4f [%.4f, %.4f]", c.Point, c.Lo, c.Hi)
}
// sortLeaderRows: sort.SliceStable by point DESC, mode name tiebreaker;
// null sorts last via sortKey()==-Inf. Keeps emit order total + deterministic.
```

### Pattern 2: Pooled-rate additive column (for verified_correctness — MIRROR canary)
**What:** A flat pooled `trueCount/total` as a degenerate `[point,point]` ciValue that consumes ZERO RNG (so it can never perturb the locked BCa determinism contract). Null total → `OK:false` → em-dash.
**When to use:** The NEW `reduceVerifiedCorrectness`.
**Example:**
```go
// Source: bench/aggregator/aggregate.go:413-422 (pooledRate) — reuse this helper
func pooledRate(trueCount, total int) ciValue {
	if total == 0 { return ciValue{OK: false} } // em-dash, never fabricated 0
	rate := float64(trueCount) / float64(total)
	return ciValue{Point: rate, Lo: rate, Hi: rate, OK: true}
}
// reduceVerifiedCorrectness: iterate loaded.Rows(task,mode), read
// r.Metrics.VerifiedCorrectness (*bool, nil-skipped), feed pooledRate.
```

### Pattern 3: Aggregate-time exclusion (INFRA-05)
**What:** Before building the headline vectors, drop rows where `rowCanary(r)` reports `contaminated==true`. The excluded set is collected per `(task,mode)` and rendered as a `leaderboard.md` footnote. Exclusion is at AGGREGATE time (the natural choke point — `successCount`/`reduceLeaderRow` see every row).
**Where:** `aggregate.go` `successCount` (line 532) and each metric reduce — wrap row iteration with a contaminated-skip; OR filter `loaded.Rows(task,mode)` once at the top of `reduceLeaderRow`.
**Example:**
```go
// NEW — mirrors rowCanary (aggregate.go:305) which already exists:
func cleanRows(rows []Row) (clean []Row, contaminated []Row) {
	for _, r := range rows {
		if c, present := rowCanary(r); present && c {
			contaminated = append(contaminated, r)
			continue
		}
		clean = append(clean, r)
	}
	return
}
// reduceLeaderRow uses cleanRows() for the headline; the contaminated slice
// feeds the footnote. CanaryPassRate stays computed over ALL rows (it MEASURES
// contamination; the headline EXCLUDES it).
```

### Anti-Patterns to Avoid
- **Rendering with any RNG.** All randomness is the single seeded PCG in the reduce path (aggregate.go:98). A renderer that touches RNG breaks byte-reproducibility (REPORT-05).
- **Fabricating 0 for a null metric.** Always em-dash (`emDash`, report.go:33). The whole codebase enforces this (Pitfall 4 in every prior phase).
- **`diff`-empty proven only by a live run.** REPORT-05 acceptance MUST be a hermetic golden test (render twice + diff committed goldens). A live CI run is NEVER the sole proof.
- **Deriving the canary Sentinel anywhere but `bench/canary.Sentinel`.** It is the single source of truth (canary.go:37, explicitly forward-compatible for Phase 89).
- **Computing `no_semantic` ablation from `ablation_deltas`.** deltas.go deliberately excludes it (it is the partial arm). Compute it at aggregate-time from the full vs no_semantic BCa CIs.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CI interval overlap test | New overlap logic | `ciOverlap(a, b)` (report.go:151) | Already the STATS-04 predicate; ablations.md reuses it |
| Atomic report write | `os.WriteFile` | `writeReport` (report.go:362) | temp+rename; concurrent reader never sees a torn file |
| Cost pricing / freshness gate | New cost join | `cost.PriceFor` + `costTableValidUntil` | Fail-closed freshness gate (D-13) already built |
| Contamination detection | New string match | `canary.IsContaminated` | Single source of truth; has teeth (Phase 86) |
| Per-language pooled rate | New reduce | `reduceLanguageRows` → `rep.ByLanguage` | Phase 85 already computes it correctly |
| Delta machinery | New delta math | `deltas.go` `ablation_deltas` (single-run) + `bca` (multi-run) | 4/5 comparisons already named |
| Footer / determinism | New footer | `renderFooter` (report.go:347) | Fixed field order, byte-stable |

**Key insight:** Phase 89 is 90% wiring + rendering of substrate that Phases 82/85/86/87 deliberately built ahead of time (populated-but-unrendered columns, the `ByLanguage` slice, the `ablation_deltas` keys, the canary probe). The ONLY genuinely new logic is: the `verified_correctness` reduce, the two new renderers, the ASCII scatter, the canary exclusion+footnote, and the YAML workflow. Resist re-implementing any reduced metric.

## Runtime State Inventory

> Phase 89 is greenfield-render over EXISTING `result.v2` files — no rename/refactor/migration. The only "state" is the committed golden fixtures.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `bench/aggregator/testdata/{leaderboard,cost_quality}.golden.md` + cost-table goldens. result.v2 rows are read-only inputs. | REGEN the 2 existing goldens (new columns/scatter); ADD 2 new goldens (per_language, ablations). Review diff. |
| Live service config | None — no daemon, no external service in the render path. | None — verified: render path is `bench/aggregator` pure code. |
| OS-registered state | None. | None. |
| Secrets/env vars | `bench-quick` uses NO API key (scripted agent, D-01). The new workflow needs no secret for the PR job. | The nightly `make bench` job, if it ever runs a real model, would need a provider key — but the milestone rule is "benchmarks local-only"; recommend the nightly job ALSO run scripted/hermetic or be `workflow_dispatch`-only with the key as a gated secret. |
| Build artifacts | `make bench-quick` builds `./cmd/helix` to `$(BINARY)`; the workflow must build it first (Makefile:138 already does). | None beyond what the Makefile target already does. |

## Common Pitfalls

### Pitfall 1: Golden churn breaks prior "additive-perturbs-nothing" tests
**What goes wrong:** Rendering `verified_correctness`/canary footnote changes `leaderboard.golden.md`; tests like `TestSwebenchColumnsGoldenStable` (aggregate_swebench_test.go:148) and the canary golden guard assert the OLD bytes.
**Why it happens:** Phases 85/86/87 froze the goldens by NOT rendering their additive columns. Phase 89 is the phase that renders them.
**How to avoid:** Regenerate goldens with `-update`, then UPDATE the assertions in those prior tests (their intent shifts from "column is invisible" to "column renders correctly"). Review the diff line-by-line — a single stray precision change is a determinism bug.
**Warning signs:** A golden diff that touches MORE than the new columns/footnote → an unintended reorder or precision drift.

### Pitfall 2: `no_semantic` missing from the ablation deltas
**What goes wrong:** `ablations.md` shows 4 deltas, not 5 — `full vs no_semantic` is blank.
**Why it happens:** `deltas.go` `requiredModes` (line 67) and `operandModes` (line 71) deliberately EXCLUDE `no_semantic` (the partial arm). The single-run `ablation_deltas` object never has it.
**How to avoid:** Compute `full vs no_semantic` at AGGREGATE time from the two modes' BCa CIs (both are present in `loaded`), with `ciOverlap`. Do NOT add `no_semantic` to deltas.go's operand set.
**Warning signs:** `ablations.md` renderer reading only `ablation_deltas` instead of re-reducing across modes.

### Pitfall 3: `cost_per_solved` on the leaderboard double-sources the point
**What goes wrong:** REPORT-01 wants `cost_per_solved` on the LEADERBOARD; it currently lives only in `cost_quality.md` (CostRow). Naively re-computing it risks a value that disagrees with cost_quality.md.
**Why it happens:** Two reduce paths for the same number.
**How to avoid:** Build BOTH the leaderboard and cost rows from the SAME `reduceCostRow` output — surface `CostRow.CostPerSolved` into the matching `LeaderRow` by `(mode,benchmark)` key. One source, two renders (IN-02 discipline: aggregate.go:456-471 already sources the headline from `costPerSolvedTask`).
**Warning signs:** Leaderboard cost ≠ cost_quality.md cost for the same row.

### Pitfall 4: ASCII scatter not byte-stable
**What goes wrong:** `cost_quality.md` golden flickers run-to-run.
**Why it happens:** Floating-point axis bucketing without fixed rounding, or map iteration order.
**How to avoid:** Bucket cost/correctness into a fixed grid with `%.4f`-style fixed rounding; sort all rows before plotting; render the grid row-by-row deterministically. Same discipline as `fmtCI`.
**Warning signs:** `TestDeterministic`-style double-render diff is non-empty for cost_quality.md.

### Pitfall 5: `report --run-id` resolves the wrong directory
**What goes wrong:** `report` writes/reads a different tree than `run`/`aggregate`.
**Why it happens:** `run` writes to `<out>/<run-id>/` (main.go:253, default `bench/reports/<id>/`). `aggregate` takes the run_dir POSITIONALLY. `report --run-id` must join `--out` (default `bench/reports`) with `--run-id`.
**How to avoid:** Resolve `runDir := filepath.Join(out, runID)`; validate `runID` as a path segment (reuse `validateRunIndexSegment`-style guard, cell.go:322, against `..`/leading-dot). Then call the shared render path. Confirm `report` and `aggregate` produce byte-identical leaderboard.md over the same tree.
**Warning signs:** `report` and `aggregate` leaderboard.md differ for the same run-id.

## Code Examples

### Resolve --run-id and render (the report subcommand body)
```go
// NEW cmd/helix-bench/report.go — mirrors aggregate.go:39-97
func newReportCmd() *cobra.Command {
	var (out, runID string; runs int; seed uint64; iters int; ciLevel float64; costTable string)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Regenerate all 4 bench reports byte-identically from a run-id",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runID == "" { return fmt.Errorf("report: --run-id is required") }
			// Pitfall 5: validate the segment, then join under --out.
			runDir := filepath.Join(out, runID) // bench/reports/<id>/
			cfg := aggregator.Config{ExpectedN: runs, Seed: seed, Iterations: iters,
				CILevel: ciLevel, CostTablePath: costTable, Today: time.Now().UTC()}
			if _, err := aggregator.Aggregate(runDir, cfg); err != nil { return err } // fail-closed
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "bench/reports", "durable output root (joined with --run-id)")
	cmd.Flags().StringVar(&runID, "run-id", "", "run identifier (the <out>/<run-id>/ tree to render)")
	cmd.Flags().IntVar(&runs, "runs", 3, "expected runs per (task,mode); fail-closed if fewer")
	cmd.Flags().Uint64Var(&seed, "seed", 42, "bootstrap RNG seed (fixed for byte-determinism)")
	cmd.Flags().IntVar(&iters, "iterations", 10000, "BCa replicate count")
	cmd.Flags().Float64Var(&ciLevel, "ci-level", 0.95, "confidence level")
	cmd.Flags().StringVar(&costTable, "cost-table", "bench/datasets/cost-table.yaml", "cost-table YAML")
	return cmd
}
```
Note: if `Aggregate` is extended to render all 4 reports (recommended), `report` and `aggregate` both emit the full set. Keep `report` as the publication-facing entry point.

### per_language.md: n/a for no-coverage languages (REPORT-02)
```go
// NEW renderPerLanguage — the 8 Tier-1 languages are a FIXED list; absent = "n/a".
var tier1Languages = []string{"c", "cpp", "csharp", "go", "java", "javascript", "python", "rust", "typescript"}
func renderPerLanguage(byLang []LanguageRow, footer Footer) string {
	have := map[string]LanguageRow{}
	for _, r := range byLang { have[r.Language] = r }
	var b strings.Builder
	b.WriteString("# Per-Language\n\n| language | pass_rate | n |\n| --- | --- | --- |\n")
	for _, lang := range tier1Languages { // FIXED order, deterministic
		if r, ok := have[lang]; ok {
			fmt.Fprintf(&b, "| %s | %.4f | %d |\n", lang, r.PassRate, r.N)
		} else {
			fmt.Fprintf(&b, "| %s | n/a | n/a |\n", lang) // REPORT-02: NOT omitted
		}
	}
	b.WriteString("\n"); b.WriteString(renderFooter(footer)); return b.String()
}
```
(Confirm the exact 8 Tier-1 language IDs against `bench/languages` / the ADAPTER coverage; the matrix above is the 9-language registry slice — trim/confirm the canonical "8 Tier-1" set during planning. **[ASSUMED — verify against bench/languages naming]**)

### YAML workflow (INFRA-04) — PR job with hard cap
```yaml
# .github/workflows/bench.yml  (NEW)
name: bench
on:
  pull_request: { branches: [main] }
  schedule: [{ cron: "0 6 * * *" }]   # nightly full bench
  workflow_dispatch:
permissions: { contents: read }
jobs:
  bench-quick:                          # PR cost gate (INFRA-04)
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    timeout-minutes: 5                  # HARD 5-min cap (no LLM cost; scripted agent)
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25.x', cache: true }
      - run: make bench-quick           # hermetic, no API key (Makefile:137)
  bench-full:                           # nightly OR maintainer-gated
    if: github.event_name == 'schedule' || github.event_name == 'workflow_dispatch'
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25.x', cache: true }
      - run: make bench                 # full matrix; documented budget in bench/BENCH.md
```
(Maintainer-label gating alternative: add a third job `if: contains(github.event.pull_request.labels.*.name, 'run-full-bench')`. Confirm the project's preferred gate — schedule vs label — during planning; both satisfy INFRA-04's "nightly or on-demand, gated on a maintainer".)

## State of the Art

| Old Approach (pre-89) | Current Approach (89) | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Additive columns populated but NOT rendered (goldens frozen) | Render `verified_correctness` + canary footnote | Phase 89 | leaderboard golden regenerates (expected) |
| 2 reports (leaderboard, cost_quality) | 4 reports + `report --run-id` | Phase 89 | per_language + ablations join the set |
| Canary measured (CanaryPassRate) but headline includes contaminated rows | Headline EXCLUDES contaminated rows + footnote | Phase 89 | INFRA-05 closes Pitfall 1 of the milestone |
| `report` = `notYetImplemented` stub | `report --run-id` wired | Phase 89 | REPORT-05 byte-reproducibility |

**Deprecated/outdated:** Nothing removed. All prior substrate is consumed, not replaced.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `github.com/stretchr/testify` (in tree) |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./bench/aggregator/... ./cmd/helix-bench/... -count=1` |
| Full suite command | `go test ./... -count=1 && make vet` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REPORT-01 | leaderboard has pass@1 + verified_correctness + cost_per_solved + CIs + overlap markers | golden | `go test ./bench/aggregator/ -run TestLeaderboard -x` | ❌ Wave 0 (extend report_test.go + regen golden) |
| REPORT-02 | per_language lists all 8 langs; no-coverage = `n/a` | golden | `go test ./bench/aggregator/ -run TestPerLanguage -x` | ❌ Wave 0 (new test + new golden) |
| REPORT-03 | 5 ablation deltas incl. no_semantic; CI overlap | golden | `go test ./bench/aggregator/ -run TestAblations -x` | ❌ Wave 0 (new test + new golden) |
| REPORT-04 | cost_quality scatter (ASCII) + valid_until cited | golden | `go test ./bench/aggregator/ -run TestCostQuality -x` | ❌ Wave 0 (extend + regen golden) |
| REPORT-05 | all 4 byte-identical on re-render (diff empty) | hermetic golden / double-render | `go test ./bench/aggregator/ -run TestReportByteReproducible -x` AND `go test ./cmd/helix-bench/ -run TestReportSubcommand -x` | ❌ Wave 0 |
| INFRA-04 | workflow file exists + is valid YAML; PR job has 5-min cap | YAML parse / lint | `go test ./.github/... -run TestBenchWorkflowValid` (or a small parse test in a test pkg) | ❌ Wave 0 |
| INFRA-05 | contaminated row excluded from headline + footnoted; synthetic test trips flag | unit + golden | `go test ./bench/aggregator/ -run TestCanaryExclusion -x` | ❌ Wave 0 (extend aggregate_canary_test.go) |

### Sampling Rate
- **Per task commit:** `go test ./bench/aggregator/... ./cmd/helix-bench/... -count=1`
- **Per wave merge:** `go test ./... -count=1 && make vet`
- **Phase gate:** Full suite green + all 4 goldens byte-stable before `/gsd-verify-work`.

### The byte-reproducibility test (REPORT-05) — the load-bearing hermetic proof
```go
func TestReportByteReproducible(t *testing.T) {
	dir := goldenFixture(t)                 // committed multi-run tree pattern (aggregate_test.go:231)
	_, err := aggregator.Aggregate(dir, aggConfig(3)); require.NoError(t, err)
	first := map[string][]byte{}
	for _, n := range []string{"leaderboard.md","per_language.md","ablations.md","cost_quality.md"} {
		b, _ := os.ReadFile(filepath.Join(dir, n)); first[n] = b
	}
	_, err = aggregator.Aggregate(dir, aggConfig(3)); require.NoError(t, err) // re-render
	for _, n := range []string{"leaderboard.md","per_language.md","ablations.md","cost_quality.md"} {
		b, _ := os.ReadFile(filepath.Join(dir, n))
		assert.Equal(t, string(first[n]), string(b), "%s must be byte-identical on re-render (REPORT-05)", n)
		// AND diff against the committed golden:
		want, _ := os.ReadFile(filepath.Join("testdata", n[:len(n)-3]+".golden.md"))
		assert.Equal(t, string(want), string(b), "%s drifted from committed golden", n)
	}
}
```
The CI workflow YAML is verified by inspection + a YAML-parse test, NEVER by a live CI run. The byte-reproducibility (`diff` empty) is ALWAYS a hermetic golden test, never proven solely by a live `report` invocation.

### Wave 0 Gaps
- [ ] `bench/aggregator/report_test.go` — extend for verified_correctness + cost_per_solved columns
- [ ] `bench/aggregator/per_language_test.go` + `testdata/per_language.golden.md` — new (REPORT-02)
- [ ] `bench/aggregator/ablations_test.go` + `testdata/ablations.golden.md` — new (REPORT-03, incl. no_semantic at aggregate-time)
- [ ] `bench/aggregator/aggregate_canary_test.go` — extend for exclusion + footnote (INFRA-05)
- [ ] `cmd/helix-bench/report_test.go` — `--run-id` resolution + report==aggregate byte-equality
- [ ] `testdata/leaderboard.golden.md` + `cost_quality.golden.md` — REGEN (new cols/scatter/footnote)
- [ ] A YAML-parse test for `.github/workflows/bench.yml` (INFRA-04 acceptance)
- [ ] Update prior "additive-perturbs-nothing" golden assertions (Pitfall 1)

## Security Domain

> `security_enforcement` is not a code-security surface for this phase (pure render + static YAML, Go-native repo). No new attack surface beyond path handling.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | `--run-id` MUST be validated as a single path segment (reject `..`, leading-dot, absolute) before `filepath.Join` — reuse the `validateRunIndexSegment` pattern (cell.go:322) / the `isValidRunID` `[A-Za-z0-9_-]+` guard already in `bench/evaluators/swebench/harness.go:97`. |
| V6 Cryptography | no | No crypto in the render path. (cosign signing in CI is the EXISTING bench-mirror.yml concern, untouched here.) |
| Others (V2/V3/V4) | no | No auth/session/access-control surface. |

### Known Threat Patterns
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via `--run-id` | Tampering | Validate run-id as a `[A-Za-z0-9_-]+` segment before `filepath.Join` (reuse `isValidRunID`) |
| Workflow secret leakage in CI | Information disclosure | PR `bench-quick` job uses NO secret (scripted agent, no API key); full job (if it ever uses a key) gates the secret behind `workflow_dispatch`/maintainer |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test/render | ✓ | go1.26.0 (module go 1.25.1) | — |
| `make` | bench-quick/bench targets | ✓ (Makefile present) | — | — |
| `./cmd/helix` build | bench-quick (daemon) | ✓ (builds in-target, Makefile:138) | — | — |
| Network / API key | NOT required this phase | n/a | — | Scripted agent, hermetic goldens — no live run |
| GitHub Actions | INFRA-04 workflow | n/a (CI-side; verified by YAML-parse + inspection) | — | YAML-parse test is the local proof |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None — the phase is fully hermetic.

## Open Questions (RESOLVED)

1. **Is the `report` subcommand a stub or wired?** — **RESOLVED:** Stub. `cmd/helix-bench/main.go:419-426` `newReportCmd()` has `RunE: notYetImplemented("report")`. It is ALREADY registered (root.AddCommand at main.go:101) so the subcommand count stays 6 (main_test.go:35 asserts 6) — **no count-assertion change needed.** Recommendation: replace the RunE body; keep registration as-is.

2. **What does `--run-id` resolve to?** — **RESOLVED:** `<out>/<run-id>/` where `--out` defaults to `bench/reports` and `--run-id` defaults to a UTC timestamp `20060102T150405Z` (main.go:178-181, 251-253). The `run` subcommand writes the durable tree to `<out>/<run-id>/<task>/<mode>/<run_index>/result.v2.json`, which is EXACTLY the tree `aggregate` (and therefore `report`) consumes. Recommendation: `report --run-id <id>` resolves `filepath.Join("bench/reports", id)` and calls the shared render path. Validate `<id>` as a path segment.

3. **Is the canary exclusion at aggregate-time?** — **RESOLVED:** Yes. `rowCanary` (aggregate.go:305) already derives the per-row flag at score time; `reduceCanaryRate` (aggregate.go:326) measures the pass-rate over ALL rows. The headline EXCLUSION (skip contaminated rows in `successCount`/the metric reduces) belongs in the same aggregate path (the only choke point that sees every row). `CanaryPassRate` MEASURES contamination; the leaderboard headline EXCLUDES it + footnotes the contaminated `(task,mode)` cells. Recommendation: add a `cleanRows`/`contaminatedRows` split at the top of `reduceLeaderRow`.

4. **Does `verified_correctness` already render?** — **RESOLVED: NO.** `rowMetrics.VerifiedCorrectness` is decoded (load.go:32) but has NO reduce function and NO leaderboard column. Phase 87's `RescoredScore` is the SWE-bench-specific UTBoost-rescored rate, NOT the generic `metrics.verified_correctness`. REPORT-01 requires the generic column. Recommendation: add `reduceVerifiedCorrectness` (pooledRate over `r.Metrics.VerifiedCorrectness`) + a leaderboard column. This regenerates the golden.

5. **Is `InjectPrompt` wired into any task today?** — **RESOLVED: NO production caller.** `canary.InjectPrompt` (canary.go:58) is referenced only by the canary package + shared-const sites (`bench/runtime/result.go:32`, `bench/evaluators/swebench/rescore.go:56` reference the doc-key const, not InjectPrompt). INFRA-05 says "emit a known-novel pattern in SELECT tasks" — the planner must add the injection point (a select-subset hook in the task/prompt path) so a contaminated model would echo the sentinel. The `completion` doc key is then read at score time by the existing `rowCanary`. Recommendation: a minimal deterministic "inject into every Kth select task" hook; keep it pure + testable.

6. **`no_semantic` ablation delta source?** — **RESOLVED:** Compute at AGGREGATE time (full vs no_semantic BCa CIs + `ciOverlap`), NOT from `deltas.go` `ablation_deltas` (which deliberately excludes `no_semantic` as the partial arm — deltas.go:26,38). All 5 modes are present in `loaded`; the ablations renderer re-reduces each `(full, other)` pair.

7. **`cost_per_solved` on the leaderboard vs cost_quality.md double-source?** — **RESOLVED:** Single-source it. Surface `CostRow.CostPerSolved` (already the IN-02-honest point from `costPerSolvedTask`, aggregate.go:463) into the matching `LeaderRow`. One reduce, two renders.

## Sources

### Primary (HIGH confidence — read in-tree this session)
- `bench/aggregator/report.go` (386 lines) — render layer, fmtCI/sort/overlap/footer/writeReport, additive column structs
- `bench/aggregator/aggregate.go` (632 lines) — pure orchestrator, reduceCanaryRate/reduceSwebenchScores/reduceLanguageRows/pooledRate, rowCanary, successCount
- `bench/aggregator/load.go` (lines 1-90) — Row/Loaded/rowMetrics (VerifiedCorrectness decoded-but-unrendered)
- `bench/canary/canary.go` (75 lines) — Sentinel/InjectPrompt/IsContaminated/DocKeyCompletion/DocKeyContaminated
- `bench/runtime/deltas.go` (317 lines) — ablation_deltas, deltaComparisons (no_semantic excluded)
- `cmd/helix-bench/main.go` (report stub at 419, run-id/out at 178-181/251-253), `cmd/helix-bench/aggregate.go` (CLI pattern), `cmd/helix-bench/main_test.go` (6-subcommand assertion)
- `Makefile:128-146` (bench / bench-quick targets), `.github/workflows/{bench-mirror.yml,go-test.yml}` (CI conventions, timeout-minutes, SHA-pinned actions)
- `bench/aggregator/testdata/leaderboard.golden.md` + `aggregate_test.go`/`aggregate_canary_test.go`/`aggregate_swebench_test.go` (golden discipline, `-update` flag, goldenFixture)
- `.planning/REQUIREMENTS.md` (REPORT-01..05, INFRA-04/05 acceptance), `.planning/STATE.md` (Phase 82/85/86/87 decisions), `89-CONTEXT.md` (SC#1-4)

### Secondary (MEDIUM confidence)
- The exact "8 Tier-1 languages" canonical ID set — inferred from the 9-language registry slice; verify against `bench/languages` naming during planning.

### Tertiary (LOW confidence)
- None.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The "8 Tier-1 languages" for per_language.md map to a fixed list resembling `{c,cpp,csharp,go,java,javascript,python,rust,typescript}` | REPORT-02 / Code Examples | LOW — verify the canonical IDs against `bench/languages`; wrong IDs = wrong `n/a` rows, caught by the per_language golden test |
| A2 | The maintainer-gate for full bench is `schedule` + `workflow_dispatch` (vs a label) | INFRA-04 | LOW — both satisfy "nightly or on-demand, gated on a maintainer"; confirm project preference |
| A3 | ASCII (not SVG) scatter is acceptable for REPORT-04 | REPORT-04 | LOW — acceptance says "ASCII / svg"; ASCII is byte-deterministic and dependency-free |
| A4 | `report` should call the SAME render path as `aggregate` (factor into `renderAll`) so both emit byte-identical output | REPORT-05 | LOW — strongly implied by "byte-identical"; the alternative (separate render) risks drift |

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new deps; all anchors read in-tree.
- Architecture: HIGH — substrate (ByLanguage, ablation_deltas, canary, additive columns) confirmed present; gaps (verified_correctness reduce, exclusion, footnote, InjectPrompt caller) precisely located.
- Pitfalls: HIGH — golden-churn, no_semantic exclusion, double-source cost, scatter determinism, run-id resolution all grounded in read code.

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable; pure in-tree render phase, no fast-moving external surface)

## RESEARCH COMPLETE

**Phase:** 89 - Reports, CI Policy & Contamination Canary

**Confidence:** HIGH

### Key Findings
- `report` is a `notYetImplemented` stub already registered (count stays 6); `--run-id` resolves to `bench/reports/<id>/` — the same tree `aggregate` consumes.
- **GAP:** `verified_correctness` is decoded (load.go:32) but NOT reduced or rendered — REPORT-01 requires it; add `reduceVerifiedCorrectness` (pooledRate) + a column.
- Canary substrate (Sentinel/IsContaminated/rowCanary/CanaryPassRate) exists; INFRA-05 needs aggregate-time EXCLUSION of contaminated rows + a leaderboard footnote, and a production `InjectPrompt` caller (none exists today).
- `ablations.md` `full vs no_semantic` must be computed at aggregate-time (deltas.go deliberately excludes no_semantic); reuse `bca`+`ciOverlap`.
- Rendering the additive columns/footnote REGENERATES the frozen goldens (expected); prior "additive-perturbs-nothing" tests must be updated in lockstep.
- Zero new deps; phase is fully hermetic — byte-reproducibility is a golden test, CI YAML is a parse/lint test, never a live run.

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | All in-tree; no external packages |
| Architecture | HIGH | Substrate + gaps located precisely in read code |
| Pitfalls | HIGH | Each grounded in a specific file:line |

### Open Questions
All 7 resolved inline (see Open Questions section) — verified_correctness gap, run-id resolution, exclusion locus, no_semantic source, InjectPrompt wiring, cost double-source, report/aggregate render unification.

### Ready for Planning
Research complete. The planner can map REPORT-01..05 + INFRA-04/05 to concrete deliverables, sequence the golden regeneration carefully (Pitfall 1), and structure waves around: (1) the two new renderers + verified_correctness reduce, (2) the canary exclusion+footnote+InjectPrompt, (3) the `report --run-id` wiring + byte-reproducibility test, (4) the CI workflow + YAML-parse test.
