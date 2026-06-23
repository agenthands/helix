# Phase 102: RepoMap-Quality + Fuzzy-Robustness Evals + Baselines - Pattern Map

**Mapped:** 2026-06-23
**Files analyzed:** 14 new/modified
**Analogs found:** 14 / 14 (every new file mirrors a verified Phase 100 / leaf-evaluator analog)

## Two Non-Obvious Contracts (read before planning)

1. **The leaf-import self-test is REQUIRED.** `vet-ablation-leakage`
   (`internal/lint/ablationleakage/analyzer.go`, `checkedPkgPrefix = ".../bench/runners"`)
   does **NOT** gate `bench/evaluators/*`. Nothing will fail the build if `repomapeval`
   or `fuzzyrobust` imports `internal/repomap` / `internal/fuzzy`. Stdlib-only is enforced
   by **convention (mirror editsim's zero-import discipline) + a package-level
   `TestLeafImports` self-test** that asserts the leaf's import set is stdlib (+ `editsim`
   for fuzzyrobust) only. Do NOT plan a task that relies on the analyzer catching this.

2. **Captured artifacts are produced OUTSIDE the leaf, scored committed-vs-committed.**
   The leaf NEVER dials the daemon and NEVER calls `fuzzy.Match`/`internal/repomap`.
   A `//go:build ignore` HELIX_BIN-gated regenerator (in `bench/runtime/`) captures the
   ranking / strategy as committed JSON; the stdlib leaf scores **committed gold vs
   committed captured**. `internal/fuzzy` is not even leaf-importable — `match.go:8`
   imports `internal/errors`, so it is not stdlib-only.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/evaluators/repomapeval/repomapeval.go` | evaluator-leaf | transform (metric math) | `bench/evaluators/editsim/editsim.go` | exact (leaf shape) |
| `bench/evaluators/repomapeval/rankers.go` | evaluator-leaf | transform | `bench/aggregator/bootstrap.go` (PCG) | role-match |
| `bench/evaluators/repomapeval/corpus.go` | loader | file-I/O (read committed JSON) | `bench/datasets/aider-polyglot/loader.go` | role-match |
| `bench/evaluators/repomapeval/repomapeval_test.go` | test (hermetic golden) | request-response | `editsim_test.go` + `aider_edit_baseline_test.go` | exact |
| `bench/evaluators/repomapeval/discriminator_test.go` | test (anti-vacuity) | transform | `editsim_test.go` `TestES_NumstatDiscriminator` | exact |
| `bench/evaluators/repomapeval/testdata/` | corpus data | file-I/O | `bench/reports/aider-edit-baseline/` (committed-data idiom) | role-match |
| `bench/evaluators/fuzzyrobust/fuzzyrobust.go` | evaluator-leaf | transform (scoring via `editsim.ES`) | `bench/evaluators/editsim/editsim.go` | exact |
| `bench/evaluators/fuzzyrobust/perturb.go` | utility | transform (deterministic perturbation) | (net-new; structural, mirror editsim purity) | partial |
| `bench/evaluators/fuzzyrobust/corpus.go` | loader | file-I/O | `bench/datasets/aider-polyglot/loader.go` | role-match |
| `bench/evaluators/fuzzyrobust/fuzzyrobust_test.go` | test (hermetic golden) | request-response | `editsim_test.go` | exact |
| `bench/evaluators/fuzzyrobust/ambiguous_test.go` | test (anti-vacuity) | request-response | `editsim_test.go` `TestES_NumstatDiscriminator` | exact |
| `bench/aggregator/repomap_eval_baseline.go` + `fuzzy_robust_baseline.go` | renderer | transform (sort-before-emit) | `bench/aggregator/aider_edit_baseline.go` | exact |
| `bench/aggregator/repomap_eval_baseline_test.go` + `fuzzy_robust_baseline_test.go` | test (byte-repro) | request-response | `aider_edit_baseline_test.go` | exact |
| `bench/runtime/repomap_eval_capture_regen.go` + `fuzzy_robust_capture_regen.go` | regenerator | event-driven (HELIX_BIN-gated capture) | `bench/runtime/aider_edit_baseline_regen.go` | exact |
| `.gitignore` (allowlist) + `Makefile` (regen targets) | config | — | `.gitignore:273-276` + `Makefile:160` (`bench-aider-edit`) | exact |

## Pattern Assignments

### `bench/evaluators/repomapeval/repomapeval.go` + `fuzzyrobust/fuzzyrobust.go` (evaluator-leaf, transform)

**Analog:** `bench/evaluators/editsim/editsim.go`

**Stdlib-only leaf shape** — package doc states the leaf boundary and the "wrong-metric"
guard explicitly; imports are stdlib-only (no import block at all in editsim.go). Mirror
this discipline; the metric math (`nDCGAtK`, `recallAtK`, `mrr`, `budgetFitRatio` from
RESEARCH Code Examples) goes here, pure-function, total-on-every-input. fuzzyrobust adds
exactly ONE in-repo import: `editsim.ES`.

```go
// editsim.go:30-46 — the pure total-function leaf signature to mirror
func ES(pred, gold string) float64 {
    pr := []rune(pred); gr := []rune(gold)
    if len(pr) == 0 && len(gr) == 0 { return 1.0 }
    ...
}
```

Use `editsim.ES(matchedText, expectedText)` in fuzzyrobust per D-05; do NOT re-implement
edit similarity (Don't-Hand-Roll).

**nDCG tie-break determinism (Pitfall 4):** any ordering the metric or baseline depends on
must use `sort.SliceStable` or a total-order comparator (rank, then `file:symbol`
lexicographic). nDCG convention: binary gain `rel_i ∈ {0,1}`, discount `1/log2(i+2)`
(0-indexed loop), `nDCG=0` when `IDCG=0`.

---

### `bench/evaluators/repomapeval/rankers.go` (evaluator-leaf, transform — discriminators)

**Analog:** `bench/aggregator/bootstrap.go` (PCG seeding), excerpt in RESEARCH:

```go
// reversed = forward reversed; seeded-random = rand.NewPCG(seed,seed2).Shuffle
func seededRandomRanker(forward []string, seed, seed2 uint64) []string {
    out := append([]string(nil), forward...)
    rng := rand.New(rand.NewPCG(seed, seed2))
    rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
    return out
}
```

Use `math/rand/v2` `rand.NewPCG` (NOT `time`-seeded `math/rand`) for byte-identical output.

---

### `bench/evaluators/*/corpus.go` (loader, file-I/O)

**Analog:** `bench/datasets/aider-polyglot/loader.go` (`LoadExercise` / `validatePathSegment`)

**Path-segment validation BEFORE any `filepath.Join`** (loader.go:81-89, V5 control). The
corpus loader reads committed gold (`file:symbol` IDs) + committed captured ranking from
`testdata/` via `encoding/json`. Gold ground truth is authored from `.meta/config.json`
`files.solution` + `.meta/example.*` reference solutions — reuse `aiderpolyglot.LoadExercise`
(loader.go:133) for that authoring, never re-parse config ad hoc. Enforce the documented
per-language size floor (`~10 tasks/lang`; fuzzy `~4/strategy/lang`) in a `TestCorpusFloor`.

**Symbol-identity convention (Pitfall 3):** pin `file:symbol` identity ONCE
(e.g. `relpath:SymbolName`, receiver-qualified for methods); gold authoring and the
capture-step parser MUST normalize to the SAME convention or recall is spuriously 0.

---

### `bench/aggregator/repomap_eval_baseline.go` + `fuzzy_robust_baseline.go` (renderer, transform)

**Analog:** `bench/aggregator/aider_edit_baseline.go`

**Sort-before-emit + fail-closed renderer** (aider_edit_baseline.go:40-71, 96-100):

```go
func RenderAiderEditBaseline(resultV2 []byte) ([]byte, error) {
    var row aiderEditBaselineRow
    if err := json.Unmarshal(resultV2, &row); err != nil { return nil, fmt.Errorf("...: %w", err) }
    if row.EditFormatApplied == nil {  // FAIL-CLOSED on a missing load-bearing key
        return nil, fmt.Errorf("...: missing edit_format_applied (fail-closed)")
    }
    type kv struct{ k, v string }
    metricRows := []kv{ {"...", ...} }
    sort.Slice(metricRows, func(i, j int) bool { return metricRows[i].k < metricRows[j].k })
    // ... pure string-builder, deterministic metrics ONLY: no latency/timestamp/abs-path ...
}
```

Decode ONLY deterministic fields into the row struct so latency/timestamp can never leak.
RepoMap baseline headline = nDCG@10 (D-07); fail-closed if it's missing. Fuzzy baseline
carries per-case strategy-match + the ambiguous-refused assertion.

---

### `bench/aggregator/*_baseline_test.go` (test, byte-reproducibility)

**Analog:** `bench/aggregator/aider_edit_baseline_test.go`

Three tests to mirror exactly (aider_edit_baseline_test.go:48-118):
- `Test*BaselineResultValid` — committed `result.v2.json` present + schema-valid + carries
  the load-bearing key (`readBaselineResult` fail-closed helper, lines 37-44).
- `Test*BaselineByteReproducible` — render TWICE from the same committed bytes, assert
  byte-identical, AND assert match against committed golden (lines 71-89).
- `Test*BaselineAntiVacuity` — STRIP the load-bearing metric → assertion fails + renderer
  fail-closes (lines 94-118). This is the BASELINE-02 "stripped metric fails" test.

`const baselineDir = "../reports/repomap-eval-baseline"` (mirror line 32).

---

### `bench/evaluators/*/discriminator_test.go` + `ambiguous_test.go` (test, anti-vacuity / assert-RED)

**Analog:** `editsim_test.go` `TestES_NumstatDiscriminator` (lines 59-78) — the
deliberate "wrong-metric / break-the-invariant must turn RED" idiom.

**RepoMap (`TestDiscriminator`, D-09):** forward must beat BOTH reversed AND seeded-random
on nDCG@10 by a committed absolute margin (≈0.2), AND prove the discriminator BITES — both
rev and rnd individually FAIL the margin gate:

```go
fwd := nDCG10(forwardRanking, gold)
rev := nDCG10(reversedRanker(forwardRanking), gold)
rnd := nDCG10(seededRandomRanker(forwardRanking, seed, seed2), gold)
require.GreaterOrEqual(t, fwd-max(rev,rnd), margin)   // margin committed
require.Less(t, rev, fwd) ; require.Less(t, rnd, fwd) // discriminator bites (non-vacuous)
```

**Fuzzy (`TestAmbiguousRefused`, D-06):** the duplicate-block case asserts the captured
outcome == `"ambiguous_match"` (a refusal), NOT `"no_match"` and NOT a match — Pitfall 6.

---

### `bench/runtime/repomap_eval_capture_regen.go` + `fuzzy_robust_capture_regen.go` (regenerator, event-driven)

**Analog:** `bench/runtime/aider_edit_baseline_regen.go`

```go
//go:build ignore
package main
func main() {
    helixBin := os.Getenv("HELIX_BIN")
    if helixBin == "" {
        fmt.Fprintln(os.Stderr, "HELIX_BIN must be set ...")
        os.Exit(2)   // FAIL-NOT-SKIP when invoked as the regenerator
    }
    // dial warm daemon, capture, render via aggregator, WriteFile committed bytes
}
```

**RepoMap capture (Pitfall 2):** `get-repo-map` returns `{"tree": treeText}` envelope
(`internal/skill/repomap/skill.go:428`), NOT a symbol list. The regenerator PARSES the
rendered tree into an ordered `file:symbol` list (parse contract: file order = PageRank
prefix appearance order; symbol order = in-file elided-def appearance order) and commits
the PRE-PARSED JSON. The leaf never parses tree text. Capture both `get-repo-map` (uniform)
and `get-context` (personalized PageRank seeded from `files.solution`).

**Fuzzy capture (Pitfall 5):** a non-leaf harness here calls `internal/fuzzy.Match`
(NOT the leaf) and records `{strategy, refused, error_kind}` per case, mapping the two
sentinels (`match.go:18,25`): `ErrAmbiguous`→`"ambiguous_match"`, `ErrNoMatch`→`"no_match"`.
Strategy labels are the 4 `types.go:14-28` values; `StrategyFailed` is never emitted as a
strategy label.

---

### `.gitignore` + `Makefile` (config)

**Analog:** `.gitignore:273-276` allowlist + `Makefile` `bench-aider-edit` target (line 160).

Add allowlist entries mirroring lines 275-276:
```
!/bench/reports/repomap-eval-baseline/
!/bench/reports/repomap-eval-baseline/**
!/bench/reports/fuzzy-robust-baseline/
!/bench/reports/fuzzy-robust-baseline/**
```

Add Makefile targets mirroring `bench-aider-edit` (Makefile:160-172): build helix, then
`HELIX_BIN="$(CURDIR)/$(BINARY)" $(GO) run bench/runtime/repomap_eval_capture_regen.go`
(and the fuzzy regen). Local-only; the committed bytes are the CI contract.

## Shared Patterns

### Stdlib-only leaf discipline + self-test
**Source:** `bench/evaluators/editsim/editsim.go` (zero non-stdlib imports)
**Apply to:** `repomapeval.go`, `fuzzyrobust.go` (+ `editsim` only), `rankers.go`, `perturb.go`, `corpus.go`
A `TestLeafImports` self-test is MANDATORY because `vet-ablation-leakage` does not gate
these packages (Contract 1 above).

### Fail-closed on a missing load-bearing key
**Source:** `aider_edit_baseline.go:45-47`, `aider_edit_baseline_test.go:37-44,57-59`
**Apply to:** every renderer + every committed-result test. Missing metric / empty file /
absent key is a HARD error, never a zero-value pass.

### Determinism: sort-before-emit + PCG seeding + double-render diff
**Source:** `aider_edit_baseline.go:71` (`sort.Slice`), `bootstrap.go` (`rand.NewPCG`),
`aider_edit_baseline_test.go:71-89` (double-render)
**Apply to:** all baselines + the seeded-random ranker. Commit deterministic metrics only.

### Path-segment validation before filepath.Join
**Source:** `bench/datasets/aider-polyglot/loader.go:81-89`
**Apply to:** every corpus/exercise loader (V5 control; reuse `LoadExercise`).

### editsim.ES reuse (fuzzy similarity)
**Source:** `bench/evaluators/editsim/editsim.go:33`
**Apply to:** `fuzzyrobust.go` — D-05 mandates reuse, do not re-implement.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `bench/evaluators/fuzzyrobust/perturb.go` | utility | transform | No existing per-tier perturbation helper; net-new. Mirror editsim's pure-stdlib-function discipline (no I/O). RESEARCH Code Examples sketches `perturbWhitespace`/`perturbIndent`/`perturbEllipsis`; expected strategy is the transform's tier (structural, never observed). |

## Metadata

**Analog search scope:** `bench/evaluators/{editsim,exactmatch,token_meter}`, `bench/aggregator/`, `bench/runtime/`, `bench/datasets/aider-polyglot/`, `internal/fuzzy/`, `.gitignore`, `Makefile`
**Files scanned (read):** 8 (editsim.go, editsim_test.go, aider_edit_baseline.go, aider_edit_baseline_test.go, aider_edit_baseline_regen.go, loader.go, fuzzy/types.go, fuzzy/match.go) + .gitignore/Makefile slices
**Pattern extraction date:** 2026-06-23
