# Phase 89: Reports, CI Policy & Contamination Canary - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 9 (5 modify, 4 create) + 6 golden/test fixtures
**Analogs found:** 9 / 9 (every deliverable extends in-tree substrate; zero greenfield logic)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/aggregator/report.go` (MODIFY: `renderPerLanguage`, `renderAblations`, scatter, verified_correctness + cost_per_solved cols, contam footnote) | renderer (pure) | transform | itself — `renderLeaderboard`/`renderCostQuality`/`fmtCI`/`renderFooter`/`writeReport` | exact (self-extend) |
| `bench/aggregator/aggregate.go` (MODIFY: `reduceVerifiedCorrectness`, `cleanRows` exclusion, contam collection, `renderAll`) | orchestrator (pure) | transform/reduce | itself — `reduceCanaryRate`/`reduceSwebenchScores`/`pooledRate`/`rowCanary` | exact (self-extend) |
| `cmd/helix-bench/report.go` (NEW, or replace `newReportCmd` body in main.go:419) | CLI (cobra RunE) | request-response | `cmd/helix-bench/aggregate.go` `newAggregateCmd` | exact (sibling subcommand) |
| `.github/workflows/bench.yml` (NEW) | config (CI) | event-driven | `.github/workflows/go-test.yml` + `release.yml` | role-match |
| `bench/<task injection point>` (WIRE `canary.InjectPrompt`) | utility wiring | transform | `bench/aggregator/aggregate.go:305` `rowCanary` (consumer side) | partial (no producer analog) |
| `bench/aggregator/testdata/{leaderboard,cost_quality}.golden.md` (REGEN) | test fixture | — | existing goldens | exact |
| `bench/aggregator/testdata/{per_language,ablations}.golden.md` (NEW) | test fixture | — | existing goldens | exact |
| `bench/aggregator/{per_language,ablations}_test.go` (NEW), `report_test.go`/`aggregate_canary_test.go` (EXTEND) | test | — | `aggregate_test.go` `goldenFixture`/`TestAggregateEndToEnd` | exact |
| `cmd/helix-bench/report_test.go` (NEW) | test | — | `aggregate_test.go` golden pattern | role-match |
| `bench/BENCH.md` (DOC: cost budget + canary policy) | doc | — | — | n/a |

## Pattern Assignments

### `bench/aggregator/report.go` (renderer, transform)

**Analog:** itself. Every new renderer reuses these verbatim helpers — no new imports.

**Null + precision discipline** (`report.go:31-43`, `186-192`): `const emDash = "—"`; `ciValue{Point,Lo,Hi,OK}`; `fmtCI` returns `emDash` when `!c.OK`, else `fmt.Sprintf("%.4f [%.4f, %.4f]", ...)`. NEVER fabricate 0. Every new column/cell goes through `fmtCI`; the ASCII scatter buckets must use the same `%.4f` fixed rounding (Pitfall 4).

**Atomic write** (`report.go:362-386`): `writeReport(runDir, name, content)` — temp+rename. The NEW `per_language.md`/`ablations.md` MUST write through this, not `os.WriteFile` (Don't-Hand-Roll).

**Footer** (`report.go:347-356`): `renderFooter(f Footer)` — fixed field order incl. `cost_table_valid_until`. Every new report appends `renderFooter(footer)`. `cost_quality.md` already cites `valid_until` here (REPORT-04's citation requirement is already met; only the scatter is new).

**Sort-before-emit** (`report.go:197-217`, `276-296`): `sortLeaderRows` (point DESC, mode tiebreaker, null→-Inf via `sortKey`); `sortCostRows` (cost ASC, null→+Inf). `renderAblations` and the scatter MUST sort their rows before emit.

**Overlap predicate** (`report.go:151-156`): `ciOverlap(a, b ciValue)` — the STATS-04 reusable predicate. `renderAblations` reuses this for the `full vs <other>` CI-overlap markers (Don't-Hand-Roll; do NOT write new overlap logic). See the rendered usage at `report.go:261-272` `overlapWarnings`.

**Leaderboard header to EXTEND** (`report.go:223-256`): `renderLeaderboard` currently emits 10 columns. ADD `verified_correctness` + `cost_per_solved` columns to the header (`report.go:231`), the separator (`:232`), and the `fmt.Fprintf` row (`:234-238`) — and the contamination footnote section after the overlap-warning block (`:244-251` is the structural template for an appended section). `LeaderRow` struct (`report.go:46-87`) already carries `CanaryPassRate`, `RawScore`, `RescoredScore`; ADD a `VerifiedCorrectness ciValue` and a `CostPerSolved ciValue` field beside them (Pitfall 3: source `CostPerSolved` from the cost row, do NOT recompute).

**per_language.md renderer** (NEW — mirror `renderCostQuality:302-327` structure): iterate a FIXED `tier1Languages` list; a language absent from `rep.ByLanguage` (`LanguageRow{Language,PassRate,N}`, `report.go:114-118`) renders `n/a`, NOT omitted (REPORT-02). **VERIFY the canonical Tier-1 ID set against `bench/languages` before hardcoding** (Assumption A1; the research example list is unverified).

### `bench/aggregator/aggregate.go` (orchestrator, reduce)

**Analog:** itself. Three named precedents the new code mirrors EXACTLY.

**Pooled-rate additive reduce** (`aggregate.go:413-422` `pooledRate`; `326-348` `reduceCanaryRate`; `388-411` `reduceSwebenchScores`): the NEW `reduceVerifiedCorrectness` reads `r.Metrics.VerifiedCorrectness` (already decoded, `load.go:32`, a `*bool`), feeds `pooledRate(trueCount, total)` — nil-skipped, zero total → `ciValue{OK:false}`. It consumes NO RNG so it cannot perturb the locked BCa determinism contract. Assign it in the per-mode loop AFTER the determinism-locked `reduceLeaderRow` (mirror the `CanaryPassRate`/`RawScore` assignments at `aggregate.go:124,133`).

**Per-row canary flag** (`aggregate.go:305-315` `rowCanary`): already derives `(contaminated, present)` from the `completion` doc key via `canary.IsContaminated`. The NEW `cleanRows(rows) (clean, contaminated []Row)` wraps this — filter at the TOP of `reduceLeaderRow` (`aggregate.go:180`) and `reduceCostRow` (`:428`) so headline vectors EXCLUDE contaminated rows (INFRA-05). `reduceCanaryRate` stays over ALL rows (it MEASURES contamination). Collect the contaminated `(task,mode)` set on the `Report` for the footnote (Pitfall 1).

**successCount** (`aggregate.go:532-543`): the boolean scalar both `reduceLeaderRow` and `reduceCostRow` (the "solved" gate, `:447-448`) call — exclusion must apply BEFORE this counts (feed it `clean` rows).

**Render+write step to factor** (`aggregate.go:147-156`): currently inline (renders 2 reports). Factor into a shared `renderAll(rep, runDir)` that emits all 4 + footnote, called by BOTH `Aggregate` and the new `report` RunE (Assumption A4, REPORT-05 byte-identity). The single seeded RNG threads at `aggregate.go:98` `rand.New(rand.NewPCG(cfg.Seed, ...))` — NEVER add RNG to the render layer (anti-pattern).

**no_semantic ablation** (compute at aggregate-time, NOT from deltas.go): `bench/runtime/deltas.go:52-61` `deltaComparisons` names only 4 (`baseline_plain`, `no_lsp`, `no_structured_edit`, `baseline_rag`); `no_semantic` is a deliberate non-operand (`deltas.go:26-27,67,71`). `renderAblations` re-reduces each `(full, other)` BCa-CI pair (the `bca` helper, `aggregate.go:592-605`) across runs and applies `ciOverlap` (Pitfall 2). Both modes are present in `loaded`.

### `cmd/helix-bench/report.go` (CLI, request-response)

**Analog:** `cmd/helix-bench/aggregate.go` `newAggregateCmd` (full file, 1-97).

Mirror EXACTLY: `aggregator.Config{ExpectedN: runs, Seed, Iterations, CILevel, CostTablePath, Today: time.Now().UTC()}` (`aggregate.go:62-73`); inject the clock at the CLI boundary, never inside the pure aggregator; propagate the `Aggregate` error verbatim via `RunE` for fail-closed non-zero exit (`:74-78`); same flag defaults (`--runs 3`, `--seed 42`, `--iterations 10000`, `--ci-level 0.95`, `--cost-table bench/datasets/cost-table.yaml` — consts `aggregate.go:27,32`).

**Difference vs aggregate:** `aggregate` takes `runDir` POSITIONALLY (`ExactArgs(1)`); `report` takes `--run-id` + `--out` (default `bench/reports`, matching the `run` subcommand's durable tree `main.go:178,253`) and resolves `runDir := filepath.Join(out, runID)` (Pitfall 5). The current stub is `main.go:419-426` `newReportCmd` (`RunE: notYetImplemented("report")`) — already registered, so subcommand count stays 6 (do NOT touch registration). Replace the RunE body.

**Path-segment validation** (V5, Pitfall 5): validate `runID` as `[A-Za-z0-9_-]+` before `filepath.Join`. Reuse the pattern at `bench/evaluators/swebench/harness.go:92-97` `isValidRunID` (rejects `..`/leading-dot) or `bench/runtime/cell.go:322-329` `validateRunIndexSegment`.

### `.github/workflows/bench.yml` (config, event-driven)

**Analog:** `.github/workflows/go-test.yml` (triggers, `permissions: {contents: read}`, `actions/checkout@v4`, `actions/setup-go@v5` with `go-version: '1.25.x'`+`cache:true`, `timeout-minutes`). `release.yml` for SHA-pinned-action convention.

- PR job `bench-quick`: `if: github.event_name == 'pull_request'`, `timeout-minutes: 5` (HARD cap), single step `run: make bench-quick` (hermetic, NO API key — Makefile:137-146, scripted agent). NB: `make bench-quick` builds `helix` then runs the smoke; it pins `--tasks=IT-go-patch-apply-1 --agent=scripted`.
- Full job `bench-full`: `if: github.event_name == 'schedule' || workflow_dispatch` (nightly cron + on-demand maintainer gate, A2), `run: make bench` (Makefile:128-129).
- Honor the EVAL-07 "forbid judge in CI" grep gate convention (`go-test.yml:133-143`) — keep `tool_behavior_judge` out of the new file.
- Verified by a YAML-parse/lint test (INFRA-04), NEVER a live CI run.

### Task injection point (`canary.InjectPrompt` production caller)

**Analog:** consumer side only — `aggregate.go:305` `rowCanary` reads the `completion` key that a contaminated model would echo. `InjectPrompt` (`bench/canary/canary.go:58-63`) has NO production caller today (research Open Q5). Add a deterministic "inject into every Kth select task" hook in the prompt/task path; keep it pure + byte-stable (same input→same output, per the canary.go:56 doc). Sentinel is the single source of truth (`canary.go:37`) — reference it, never re-derive (anti-pattern).

## Shared Patterns

### Determinism (zero-RNG-in-render)
**Source:** `bench/aggregator/aggregate.go:98` (single seeded PCG), `report.go:22-25` (sort-before-emit + fixed precision + atomic write)
**Apply to:** ALL new renderers, the scatter, `reduceVerifiedCorrectness`, `renderAll`. Additive reduces use `pooledRate` (no RNG). REPORT-05 byte-identity depends on this.

### Em-dash null discipline
**Source:** `report.go:31-43,187-192` (`emDash`, `fmtCI`); `pooledRate` zero-total → `OK:false`
**Apply to:** every new cell — verified_correctness, cost_per_solved, scatter, ablation deltas, per-language `n/a`.

### Atomic report write + footer
**Source:** `report.go:362-386` (`writeReport`), `:347-356` (`renderFooter`)
**Apply to:** all 4 reports via the shared `renderAll`.

### Golden-diff test harness
**Source:** `bench/aggregator/aggregate_test.go:228-274` (`goldenFixture`, `TestAggregateEndToEnd`, `-update` flag at `:22`)
**Apply to:** new `per_language_test.go`/`ablations_test.go`, the byte-reproducible double-render test (REPORT-05), and `report_test.go` (report==aggregate equality). Pitfall 1: regenerate the 2 existing goldens with `-update` and UPDATE prior "additive-perturbs-nothing" assertions (`aggregate_swebench_test.go:148` `TestSwebenchColumnsGoldenStable`, `aggregate_canary_test.go:122` `TestAggregateCanaryAbsentIsEmDash`) in lockstep.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `canary.InjectPrompt` production caller | utility wiring | transform | No existing producer; only the consumer (`rowCanary`) exists. Use the injection-point guidance above + the deterministic-purity discipline. |
| `bench/BENCH.md` cost-budget/canary-policy doc | doc | — | Documentation; no code analog. |

## Metadata

**Analog search scope:** `bench/aggregator/`, `bench/canary/`, `bench/runtime/`, `cmd/helix-bench/`, `.github/workflows/`, `Makefile`, `bench/evaluators/swebench/`
**Files scanned:** 10 read + targeted greps
**Pattern extraction date:** 2026-06-21
