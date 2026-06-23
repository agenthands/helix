# Phase 102: RepoMap-Quality + Fuzzy-Robustness Evals + Baselines - Research

**Researched:** 2026-06-23
**Domain:** Go stdlib-only bench evaluators (IR ranking metrics + fuzzy-match robustness), committed byte-reproducible baselines
**Confidence:** HIGH (all findings verified against the live codebase via Read/Grep; zero external deps introduced)

## Summary

Phase 102 adds two stdlib-only leaf evaluators under `bench/evaluators/` that **measure existing Helix subsystems** — `internal/repomap` (PageRank ranking + token-budget fit) and `internal/fuzzy` (4-strategy cascade + ambiguity refusal) — against corpora authored from **task ground truth** (the vendored exercism `.meta/example.go` reference solutions + `.meta/config.json` `files.solution`), never from tool output. Each evaluator ships an **anti-vacuity discriminator** (reversed + seeded-random rankers for RepoMap; a must-be-refused duplicate-block case for fuzzy) and a **committed byte-reproducible baseline** mirroring the Phase 100 `aider_edit_baseline` pattern exactly.

The single most important architectural finding: **the `vet-ablation-leakage` analyzer (`internal/lint/ablationleakage`) does NOT police `bench/evaluators/*` imports** — its import-boundary check is scoped to `bench/runners/*` only (`checkedPkgPrefix = "github.com/agenthands/helix/bench/runners"`). So "stdlib-only leaf, respecting `vet-ablation-leakage`" is a **discipline convention** the existing leaves already follow (editsim/exactmatch/token_meter import only stdlib), not a compile-time gate against the new packages. The plan must enforce stdlib-only by *convention + a self-test*, and CANNOT import `internal/repomap` or `internal/fuzzy` (the latter is not even stdlib-only — it imports `internal/errors`). Both subsystems' outputs must therefore be consumed **as committed data**, captured by a separate `HELIX_BIN`-gated regenerator that lives OUTSIDE the leaf (mirroring `bench/runtime/aider_edit_baseline_regen.go`).

**Primary recommendation:** Build each evaluator as a pure stdlib package that scores **committed gold corpus** against a **committed captured ranking/strategy artifact** (committed-vs-committed). A separate `//go:build ignore` regenerator (in `bench/runtime/` or a `cmd/` script) dials the warm daemon `HELIX_BIN`-gated to refresh the captured artifact. The hermetic golden sibling test (`go test ./bench/...`, no binary, no network) reads both committed files and is the sole authoritative proof. This reconciles "stdlib-only leaf" + "no kernel import" + "needs the tool's ranking" exactly as Phase 100 did.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Ranking-quality metrics (recall@k/MRR/nDCG) | `bench/evaluators/repomapeval` (leaf, stdlib) | — | Pure scoring of committed gold vs committed ranking; no I/O on the kernel |
| Token-budget-fit ratio | `bench/evaluators/repomapeval` (leaf) | — | Gold-retention-within-budget computed from committed rendered map text |
| Fuzzy strategy/refusal scoring | `bench/evaluators/fuzzyrobust` (leaf, stdlib + `editsim.ES`) | — | Scores committed expected-strategy vs committed captured strategy |
| Live ranking capture (`get-repo-map`/`get-context`) | `bench/runtime` regenerator (`//go:build ignore`) | warm daemon via `HELIX_BIN` | Dials gRPC `StreamMCP`; lives OUTSIDE the leaf (Phase 100 precedent) |
| Live fuzzy strategy capture | `bench/runtime` regenerator OR a thin in-repo non-leaf harness | `internal/fuzzy.Match` | Capture-as-data; the leaf never imports `internal/fuzzy` |
| Deterministic baseline render | `bench/aggregator` (`RenderRepoMapEvalBaseline` / `RenderFuzzyRobustBaseline`) | — | Mirror `RenderAiderEditBaseline`: sort-before-emit, no latency/timestamp |
| Corpus authoring (gold + drift) | committed JSON/txt corpus files | `.meta/example.go` + `.meta/config.json` | Authored once from ground truth, frozen, reviewed |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go standard library | go.mod toolchain (Go 1.22+, uses `math/rand/v2` PCG in aggregator) | All metric math, JSON corpus I/O, perturbation transforms | `[VERIFIED: codebase]` Leaf-evaluator discipline is stdlib-only; editsim/exactmatch/token_meter import zero non-stdlib packages |
| `bench/evaluators/editsim` | in-repo (`editsim.ES`) | Normalized CM-ES edit similarity for `fuzzyrobust` | `[VERIFIED: codebase]` CONTEXT.md D-05 mandates reuse; `editsim.go:33` `ES(pred, gold string) float64` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `math/rand/v2` (`rand.NewPCG`) | stdlib | Seeded random ranker discriminator | `[VERIFIED: codebase]` aggregator/bootstrap.go uses `rand.New(rand.NewPCG(seed, seed2))` for byte-identical seeded output |
| `encoding/json` | stdlib | Corpus + captured-artifact serialization | Corpus on-disk format (Claude's discretion: JSON recommended for parity with result.v2) |
| `sort` | stdlib | sort-before-emit determinism, tie-breaking in rankings | `[VERIFIED: codebase]` aggregator/aider_edit_baseline.go sorts metric rows before emit |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Committed-vs-committed scoring | Leaf shells out to `helix` at test time | REJECTED — violates fail-not-skip determinism + couples the hermetic golden to a binary. Phase 100 precedent is committed artifact + separate regenerator |
| Importing `internal/fuzzy` in the leaf | — | IMPOSSIBLE — `internal/fuzzy/match.go:8` imports `internal/errors` (serr), so it is NOT stdlib-only; the leaf must consume captured strategy data |

**Installation:** No new packages. `[VERIFIED: codebase]` STATE.md v2.1 constraint: "ZERO new Go deps."

## Package Legitimacy Audit

> Not applicable — this phase installs **no external packages**. All code uses the Go standard library plus the in-repo `bench/evaluators/editsim` package. No npm/PyPI/crates dependency is added (STATE.md: "ZERO new Go deps").

## Architecture Patterns

### System Architecture Diagram

```
                  AUTHORING (once, committed, reviewed)
  .meta/config.json (files.solution) ─┐
  .meta/example.go (reference soln) ──┼──> [author gold symbols] ──> gold corpus (committed JSON)
  vendored fixture code blocks ───────┴──> [per-tier perturbation] ──> drift corpus (committed JSON)

                  CAPTURE (HELIX_BIN-gated, separate regenerator, OUTSIDE the leaf)
  HELIX_BIN daemon ──get-repo-map/get-context──> rendered "tree" text ──parse order──> captured ranking (committed)
  internal/fuzzy.Match (via non-leaf harness) ──> observed strategy/refusal ──────────> captured strategy (committed)

                  SCORE (stdlib-only leaf, go test ./bench/..., NO binary, NO network)
  gold corpus ─────────┐
  captured ranking ────┼──> repomapeval ──> recall@10 / MRR / nDCG@10 / budget-fit ratio
                       │         │
  reversed ranker ─────┤         └──> DISCRIMINATOR: forward nDCG@10 − max(rev,rand) ≥ margin (≈0.2) else FAIL
  seeded-random ranker ┘
  drift corpus ────────┐
  captured strategy ───┼──> fuzzyrobust ──> per-case strategy-match + editsim.ES + ambiguous-refused assertion
  duplicate-block case ┘

                  BASELINE (committed, byte-reproducible)
  scores ──> RenderRepoMapEvalBaseline / RenderFuzzyRobustBaseline (sort-before-emit, deterministic-metrics-only)
         ──> bench/reports/repomap-eval-baseline/{result.v2.json,BENCH-RESULTS.md} (gitignore-allowlisted)
```

### Recommended Project Structure
```
bench/evaluators/
├── repomapeval/
│   ├── repomapeval.go        # stdlib-only: metric math (recall@k, MRR, nDCG@10, budget-fit)
│   ├── rankers.go            # reversed + seeded-random discriminator rankers (stdlib)
│   ├── corpus.go             # committed gold-corpus + captured-ranking loader (encoding/json)
│   ├── repomapeval_test.go   # hermetic golden: scores committed-vs-committed, no binary
│   ├── discriminator_test.go # ANTI-VACUITY: reversed/random MUST fail the gold corpus
│   └── testdata/             # committed gold corpus + captured ranking fixtures
├── fuzzyrobust/
│   ├── fuzzyrobust.go        # stdlib + editsim.ES: per-case strategy/refusal scoring
│   ├── perturb.go            # deterministic per-tier perturbation transforms (corpus authoring helper)
│   ├── corpus.go             # committed drift-corpus + captured-strategy loader
│   ├── fuzzyrobust_test.go   # hermetic golden
│   ├── ambiguous_test.go     # ANTI-VACUITY: duplicate-block case MUST be refused
│   └── testdata/
bench/aggregator/
├── repomap_eval_baseline.go       # RenderRepoMapEvalBaseline (mirror aider_edit_baseline.go)
├── repomap_eval_baseline_test.go  # byte-reproducible double-render + anti-vacuity (mirror)
├── fuzzy_robust_baseline.go
└── fuzzy_robust_baseline_test.go
bench/runtime/
├── repomap_eval_capture_regen.go  # //go:build ignore: HELIX_BIN-gated live ranking capture
└── fuzzy_robust_capture_regen.go  # //go:build ignore: strategy capture
bench/reports/
├── repomap-eval-baseline/   # gitignore-allowlisted (mirror aider-edit-baseline/)
└── fuzzy-robust-baseline/
```

### Pattern 1: Committed-vs-committed scoring with a separate HELIX_BIN regenerator
**What:** The leaf scores two committed files (gold + captured). A `//go:build ignore` regenerator dials the daemon to refresh the captured file.
**When to use:** Always — this is the locked Phase 100 determinism contract.
**Example:**
```go
// Source: bench/runtime/aider_edit_baseline_regen.go (Phase 100, VERIFIED)
//go:build ignore
package main
func main() {
    helixBin := os.Getenv("HELIX_BIN")
    if helixBin == "" {
        fmt.Fprintln(os.Stderr, "HELIX_BIN must be set ...")
        os.Exit(2) // fail-not-skip when invoked as the regenerator
    }
    // ... dial daemon, capture, render via aggregator, write committed bytes ...
}
```

### Pattern 2: Deterministic baseline renderer (sort-before-emit, no latency)
**What:** A pure `Render*Baseline([]byte) ([]byte, error)` that restates only deterministic metrics, sorting every list before emit so map-iteration order can never drift the bytes.
**Example:**
```go
// Source: bench/aggregator/aider_edit_baseline.go:64-71 (VERIFIED)
type kv struct{ k, v string }
metricRows := []kv{ {"nDCG@10", ...}, {"recall@10", ...}, {"MRR", ...} }
sort.Slice(metricRows, func(i, j int) bool { return metricRows[i].k < metricRows[j].k })
// fail-CLOSED on a missing load-bearing key, never a silent drop:
if row.HeadlineMetric == nil { return nil, fmt.Errorf("...: missing nDCG@10 (fail-closed)") }
```

### Pattern 3: Anti-vacuity discriminator test (assert-RED)
**What:** A deliberate break-the-invariant test that MUST turn the corpus RED, proving the metric grades.
**Example (RepoMap, per D-09):**
```go
// reversed ranker = gold-ranking reversed; seeded-random = rand.NewPCG(seed,seed2) shuffle
fwd := nDCG10(forwardRanking, gold)
rev := nDCG10(reverse(forwardRanking), gold)
rnd := nDCG10(seededShuffle(forwardRanking, seed), gold)
require.GreaterOrEqual(t, fwd-max(rev,rnd), margin) // margin ≈ 0.2, committed
// AND prove the discriminator BITES: both rev and rnd individually FAIL the margin gate.
```
**Example (fuzzy, per D-06):** the duplicate-block corpus case asserts `errors.Is(capturedErr, ErrAmbiguous)` / the captured strategy is `"ambiguous_match"` — a refusal, NOT a match.

### Anti-Patterns to Avoid
- **Leaf shells out to `helix` / clones at test time:** breaks hermetic golden determinism. Capture lives in the regenerator.
- **Importing `internal/fuzzy` or `internal/repomap` into the leaf:** the former is not stdlib-only (imports `internal/errors`); the latter pulls tree-sitter/SQLite. Capture-as-data only.
- **Deriving gold from `get-repo-map` output:** self-confirming. Gold MUST come from `.meta/example.go` symbols (D-02).
- **Green-path-only discriminator test:** a gate with only a happy-path test is presumed broken (v1.12 vacuous-pass CRITICAL).
- **Emitting latency/timestamp/absolute-path into the committed baseline:** non-reproducible; excluded by construction (Phase 100 contract).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Edit similarity for fuzzy drift | A new Levenshtein/ratio | `editsim.ES` | `[VERIFIED]` D-05 + editsim.go already provides rune-level normalized CM-ES; re-impl risks the wrong-metric pitfall its own discriminator test guards |
| Seeded deterministic RNG | `time`-seeded `math/rand` | `rand.New(rand.NewPCG(seed, seed2))` | `[VERIFIED]` aggregator/bootstrap.go uses PCG for byte-identical resamples |
| Baseline rendering plumbing | New report writer | Mirror `RenderAiderEditBaseline` + `bench/reports/*` gitignore allowlist | `[VERIFIED]` Phase 100 already solved byte-reproducible committed baselines |
| Exercise/config loading | Re-parse `.meta/config.json` ad hoc | `aiderpolyglot.LoadExercise(dir, language)` (returns `Config.Files.Solution`) | `[VERIFIED]` loader.go:133; stdlib-only accessor already exists |
| Fuzzy 4-strategy semantics | Re-implement the cascade | Capture `internal/fuzzy.Match` output; derive *expected* strategy structurally from the perturbation | `[VERIFIED]` measure-don't-port (STATE.md); the transform IS the derivation (D-05) |

**Key insight:** Phase 102 is a **measurement** phase. Nearly every primitive (edit similarity, seeded RNG, baseline rendering, exercise loading, the subsystems under test) already exists. The net-new code is metric math + corpus authoring + discriminators.

## Common Pitfalls

### Pitfall 1: Assuming `vet-ablation-leakage` enforces the evaluator leaf boundary
**What goes wrong:** Planning a task that relies on the analyzer to catch a `internal/repomap` import from `repomapeval`.
**Why it happens:** CONTEXT.md says "respecting `vet-ablation-leakage`", which reads like enforcement.
**Reality:** `[VERIFIED: internal/lint/ablationleakage/analyzer.go:38]` `checkedPkgPrefix = ".../bench/runners"` — the import check ONLY fires on `bench/runners/*`, NOT `bench/evaluators/*`. The doc comment even states the forbidden edge is deliberately NOT `→ internal/fuzzy`.
**How to avoid:** Enforce stdlib-only by (a) convention (mirror editsim's zero-import discipline) and (b) a package-level self-test that asserts the leaf's import set is stdlib + `editsim` only (e.g. a `go/packages` or hardcoded-import-list test). Do NOT claim the existing analyzer gates it.
**Warning signs:** A plan task says "the analyzer will fail the build if X imports internal/repomap" — it won't, for evaluator packages.

### Pitfall 2: `get-repo-map` returns rendered TREE TEXT, not a structured symbol list
**What goes wrong:** Expecting a JSON array of ranked `file:symbol` IDs from the tool.
**Why it happens:** "ranking quality" implies a ranked list.
**Reality:** `[VERIFIED: internal/skill/repomap/skill.go:428]` the tool returns `MarshalEnvelope(env, {"tree": treeText})` — JSON `{"source","fallback_reason","graph_version","freshness","tree"}` where `tree` is the directory-tree text from `TreeRenderer.RenderBudgeted` (`render.go`). File order within the rendered prefix follows PageRank (the `ranked[:mid]` prefix), and symbols appear as elided defs under each file. `get-context` uses personalized PageRank (`RankFromSeeds`).
**How to avoid:** The captured-ranking artifact must be **derived by parsing the rendered tree text** into an ordered `file:symbol` list at capture time (in the regenerator), then committed. The leaf scores against that committed parsed order — it never parses tree text itself if the captured artifact is pre-parsed JSON. Decide the parse contract (file order = appearance order in tree; symbol order = elided-def appearance order) and document it.
**Warning signs:** A task that asks the leaf to parse `"tree"` markdown — push that into the capture step.

### Pitfall 3: Symbol-identity mismatch between gold and captured ranking
**What goes wrong:** Gold `file:symbol` IDs don't string-match what the rendered map emits, so recall is spuriously 0.
**Why it happens:** `.meta/example.go` symbol names (authored gold) vs. the elided-def text rendered by `elide.go` may differ in casing/qualification/receiver.
**How to avoid:** Pin the `file:symbol` identity convention ONCE (e.g. `relpath:FuncName` or `relpath:Receiver.Method`), author gold to it, and make the capture-step parser normalize to the SAME convention. Add a corpus-authoring assertion that every gold ID for an exercise is parseable. Author gold from `.meta/example.go` (the reference solution that actually defines the symbols), keyed to the `files.solution` relpath.
**Warning signs:** recall@10 == 0 for a non-broken ranker on a real exercise.

### Pitfall 4: nDCG tie-breaking non-determinism
**What goes wrong:** Equal-relevance items in the ranking sort in map-iteration order, drifting the committed baseline bytes.
**Why it happens:** Go map iteration is randomized; `sort.Slice` is not stable.
**How to avoid:** Use `sort.SliceStable` or a total-order comparator (rank, then `file:symbol` lexicographic) for any ordering the metric or baseline depends on. The gain/discount convention for nDCG@10: binary relevance gain `rel_i ∈ {0,1}`, discount `1/log2(i+1)` for 1-indexed position `i`, `DCG = Σ rel_i/log2(i+1)`, `IDCG` = DCG of the ideal (all gold first), `nDCG = DCG/IDCG` (define `nDCG=0` when `IDCG=0`, i.e. no gold in top-k).
**Warning signs:** `TestByteReproducible` double-render diff is non-empty intermittently.

### Pitfall 5: `internal/fuzzy.Match` is not directly leaf-importable
**What goes wrong:** Planning `fuzzyrobust` to call `fuzzy.Match` directly.
**Why it happens:** It looks like a pure function.
**Reality:** `[VERIFIED: internal/fuzzy/match.go:8]` it imports `internal/errors` (serr) and `internal/fuzzy` pulls the whole package. The leaf must consume **captured** strategy/refusal data. The capture harness (non-leaf, e.g. in `bench/runtime`) calls `fuzzy.Match` and records `{strategy, refused, error_kind}` per case.
**How to avoid:** The expected strategy is derived structurally from the perturbation (D-05); the captured strategy is recorded by the non-leaf harness; the leaf only compares the two committed values + applies `editsim.ES` to the matched-vs-expected text.
**Warning signs:** `fuzzyrobust` import list contains `internal/fuzzy`.

### Pitfall 6: Refusal representation ambiguity
**What goes wrong:** A refusal (ambiguous) is conflated with a no-match (failed) or a successful match.
**Reality:** `[VERIFIED: internal/fuzzy/match.go:18-25]` two distinct sentinels: `ErrAmbiguous` (N>1 candidates → refusal) and `ErrNoMatch` (cascade exhausted). Phase 53 classifier maps `ErrAmbiguous`→`outcome="ambiguous_match"`, `ErrNoMatch`→`outcome="no_match"`/`strategy="none"`. `StrategyFailed` ("failed") is never emitted as a strategy label.
**How to avoid:** The captured artifact must record outcome as one of `{exact, whitespace_normalized, indentation_flexible, ambiguous_match, no_match}`. The duplicate-block must-refuse case asserts captured outcome == `ambiguous_match` (D-06), NOT `no_match`.

## Code Examples

### nDCG@10 (binary relevance, deterministic tie-break)
```go
// Source: standard IR nDCG (CITED: en.wikipedia.org/wiki/Discounted_cumulative_gain),
// adapted to stdlib + sort-before-emit determinism.
func nDCGAtK(rankedIDs []string, gold map[string]bool, k int) float64 {
    dcg := 0.0
    for i := 0; i < k && i < len(rankedIDs); i++ {
        if gold[rankedIDs[i]] {
            dcg += 1.0 / math.Log2(float64(i+2)) // 1-indexed pos i+1 → log2((i+1)+1)
        }
    }
    // IDCG: all min(len(gold),k) relevant items at the top.
    ideal := len(gold); if ideal > k { ideal = k }
    idcg := 0.0
    for i := 0; i < ideal; i++ { idcg += 1.0 / math.Log2(float64(i+2)) }
    if idcg == 0 { return 0 }
    return dcg / idcg
}
```

### recall@k and MRR
```go
func recallAtK(rankedIDs []string, gold map[string]bool, k int) float64 {
    if len(gold) == 0 { return 0 }
    hit := 0
    for i := 0; i < k && i < len(rankedIDs); i++ { if gold[rankedIDs[i]] { hit++ } }
    return float64(hit) / float64(len(gold))
}
func mrr(rankedIDs []string, gold map[string]bool) float64 {
    for i, id := range rankedIDs { if gold[id] { return 1.0 / float64(i+1) } }
    return 0
}
```

### Gold-retention-within-budget ratio (D-08)
```go
// fraction of gold symbols that survive into the rendered map within the token budget.
func budgetFitRatio(renderedSymbolIDs []string, gold map[string]bool) float64 {
    if len(gold) == 0 { return 0 }
    inMap := map[string]bool{}
    for _, id := range renderedSymbolIDs { if gold[id] { inMap[id] = true } }
    return float64(len(inMap)) / float64(len(gold))
}
```

### Seeded-random + reversed discriminator rankers (D-09)
```go
// Source: rand.NewPCG pattern from bench/aggregator/bootstrap.go (VERIFIED)
func reversedRanker(forward []string) []string {
    out := make([]string, len(forward))
    for i, id := range forward { out[len(forward)-1-i] = id }
    return out
}
func seededRandomRanker(forward []string, seed, seed2 uint64) []string {
    out := append([]string(nil), forward...)
    rng := rand.New(rand.NewPCG(seed, seed2))
    rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
    return out
}
```

### Deterministic per-tier perturbation transforms (D-05, corpus authoring)
```go
// Each transform takes a real fixture code block and produces a drifted search block
// whose EXPECTED strategy is the transform's tier (structural derivation, not observed).
//   exact:                 identity (no drift)               -> StrategyExact
//   whitespace_normalized: add trailing/leading spaces/line  -> StrategyWhitespace
//   indentation_flexible:  reindent (spaces<->tabs / depth)  -> StrategyIndentationFlex
//   ellipsis:              elide middle line(s) with "..."    -> matched via AllowEllipsis
func perturbWhitespace(block string) string { /* add "  " trailing per line */ }
func perturbIndent(block string) string     { /* swap leading spaces<->tab / shift depth */ }
func perturbEllipsis(block string) string   { /* replace middle lines with "\n...\n" */ }
// exact = block unchanged.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `math/rand` global seed | `math/rand/v2` `rand.NewPCG(seed,seed2)` | Phase 100 aggregator | Byte-identical seeded output across runs (required for committed baseline) |
| Bench reports fully gitignored | Allowlisted committed baseline dir | Phase 100 (`!/bench/reports/aider-edit-baseline/**`) | Committed baseline tree is the CI contract |
| Latency in committed metrics | Deterministic-metrics-only | Phase 100 | Latency → local `bench-micro`; never committed |

**Deprecated/outdated:** None relevant — this phase is additive.

## Runtime State Inventory

> Not a rename/refactor/migration phase — this is a greenfield-additive evaluator phase. No stored data, live-service config, OS-registered state, secrets, or build artifacts carry a renamed string. **None — verified by phase scope (new packages + committed corpus/baseline only).**

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All packages + tests | ✓ | go.mod (1.22+) | — |
| `helix` binary (`HELIX_BIN`) | Live ranking capture (regenerator only) | conditional | `/tmp/helix-v2.1-test` may exist; `make build` rebuilds | **Hermetic golden runs binary-free** — the live leg is regenerator-only |
| Network/clone | None | n/a | — | Corpus authored from already-vendored fixtures (offline) |
| Vendored fixtures (py/go/rust) | Corpus authoring | ✓ | `bench/datasets/aider-polyglot/fixtures/` | — |

**Missing dependencies with no fallback:** None — `go test ./bench/...` (hermetic golden) needs no binary and no network.
**Missing dependencies with fallback:** `HELIX_BIN` for the regenerator only; absence does not block the authoritative test (it blocks only on-demand artifact regen). **Planning constraint:** if regenerating the captured ranking, a built `helix` is required (`make build` then `HELIX_BIN="$(pwd)/helix"`); surface this as a regen prerequisite, never a test prerequisite.

## Validation Architecture

> `workflow.nyquist_validation` not found explicitly false in config — section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify` (`require`/`assert`) `[VERIFIED: aider_edit_baseline_test.go imports]` |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./bench/evaluators/repomapeval/ ./bench/evaluators/fuzzyrobust/` |
| Full suite command | `go test ./...` (then `go vet ./...`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REPOEVAL-01 | leaf computes recall@k/MRR/nDCG + budget-fit, stdlib-only | unit | `go test ./bench/evaluators/repomapeval/ -run TestMetrics` | ❌ Wave 0 |
| REPOEVAL-01 | leaf imports stdlib + editsim only | unit (self-test) | `go test ./bench/evaluators/repomapeval/ -run TestLeafImports` | ❌ Wave 0 |
| REPOEVAL-02 | gold corpus from `.meta/example.go`, size floor enforced | unit | `go test ./bench/evaluators/repomapeval/ -run TestCorpusFloor` | ❌ Wave 0 |
| REPOEVAL-02 | reversed AND random ranker MUST fail the gold corpus by margin | unit (anti-vacuity) | `go test ./bench/evaluators/repomapeval/ -run TestDiscriminator` | ❌ Wave 0 |
| FUZZBENCH-01 | leaf scores 4-strategy selection + refusal via editsim.ES | unit | `go test ./bench/evaluators/fuzzyrobust/ -run TestStrategy` | ❌ Wave 0 |
| FUZZBENCH-02 | drift corpus per-tier, expected strategy from drift type, size floor | unit | `go test ./bench/evaluators/fuzzyrobust/ -run TestCorpusFloor` | ❌ Wave 0 |
| FUZZBENCH-02 | duplicate-block ambiguous case MUST be refused | unit (anti-vacuity) | `go test ./bench/evaluators/fuzzyrobust/ -run TestAmbiguousRefused` | ❌ Wave 0 |
| BASELINE-02 | committed baseline present, schema-ok, byte-reproducible double-render | unit | `go test ./bench/aggregator/ -run TestRepoMapEvalBaseline` | ❌ Wave 0 |
| BASELINE-02 | baseline anti-vacuity (stripped metric fails the assertion) | unit | `go test ./bench/aggregator/ -run TestRepoMapEvalBaselineAntiVacuity` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/evaluators/repomapeval/ ./bench/evaluators/fuzzyrobust/ ./bench/aggregator/`
- **Per wave merge:** `go test ./... && go vet ./...`
- **Phase gate:** Full suite green; hermetic golden tests pass with NO `HELIX_BIN` and NO network.

### Wave 0 Gaps
- [ ] `bench/evaluators/repomapeval/repomapeval_test.go` — covers REPOEVAL-01 (metrics)
- [ ] `bench/evaluators/repomapeval/discriminator_test.go` — covers REPOEVAL-02 (anti-vacuity)
- [ ] `bench/evaluators/repomapeval/testdata/` — committed gold corpus + captured ranking fixtures
- [ ] `bench/evaluators/fuzzyrobust/fuzzyrobust_test.go` — covers FUZZBENCH-01
- [ ] `bench/evaluators/fuzzyrobust/ambiguous_test.go` — covers FUZZBENCH-02 (must-refuse)
- [ ] `bench/aggregator/repomap_eval_baseline_test.go` + `fuzzy_robust_baseline_test.go` — covers BASELINE-02 (mirror `aider_edit_baseline_test.go`)
- [ ] `.gitignore` allowlist entries for `bench/reports/repomap-eval-baseline/` + `fuzzy-robust-baseline/` (mirror lines 275-276)
- [ ] `Makefile` regen targets (mirror `bench-aider-edit` at line 160)
- [ ] Leaf import self-test (since `vet-ablation-leakage` does NOT gate `bench/evaluators/*`)

## Security Domain

> `security_enforcement` default-enabled. This phase adds **bench evaluators + committed test corpora** — no auth, no network surface, no untrusted input at runtime (corpus is authored + committed). The ASVS-relevant concern is **input handling of fixture paths** (already mitigated by the vendored, trusted, committed corpus).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | n/a (no auth surface) |
| V3 Session Management | no | n/a |
| V4 Access Control | no | n/a |
| V5 Input Validation | yes (low) | Path-segment validation when loading exercises — reuse `aiderpolyglot.LoadExercise` which already calls `validatePathSegment` `[VERIFIED: loader.go:81]` |
| V6 Cryptography | no | n/a (seeded RNG is for determinism, not security; never use for secrets) |

### Known Threat Patterns for Go bench evaluators
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via corpus/exercise name | Tampering | `validatePathSegment` before any `filepath.Join` (already in loader) |
| Non-determinism leaking into committed bytes | Tampering (silent drift) | sort-before-emit + seeded PCG + double-render diff test |
| Vacuous-pass gate (discriminator never bites) | Repudiation | Mandatory assert-RED anti-vacuity test (v1.12 CRITICAL) |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | nDCG uses **binary** relevance (gold ∈ {0,1}), discount `1/log2(pos+1)` | Code Examples | LOW — D-07 says "nDCG@10"; binary gain is the natural fit for a gold set. If graded relevance is wanted, the formula generalizes trivially. Confirm convention in plan. |
| A2 | The captured ranking is **pre-parsed into ordered `file:symbol` JSON** at capture time (regenerator), so the leaf never parses tree text | Pitfall 2 / Architecture | MEDIUM — alternative is the leaf parses committed raw tree text. Either works; pre-parsed keeps the leaf simpler and the parse contract testable in the capture harness. Decide in plan. |
| A3 | `file:symbol` identity = `relpath:SymbolName` (receiver-qualified for methods) | Pitfall 3 | MEDIUM — exact convention is Claude's discretion; must be consistent between gold authoring and capture parser or recall is spuriously 0. |
| A4 | Corpus on-disk format = JSON (parity with result.v2) | Standard Stack | LOW — explicitly Claude's discretion (D-discretion). JSON recommended; txt also acceptable. |
| A5 | The fuzzy capture harness lives in `bench/runtime` (non-leaf) and calls `fuzzy.Match` | Pitfall 5 | LOW — any non-leaf location works; `bench/runtime` mirrors the Phase 100 regenerator home. |
| A6 | Discriminator margin ≈ 0.2 absolute on nDCG@10 | D-09 | LOW — CONTEXT.md says "≥0.2 is a guide, not a hard mandate"; committed value is Claude's discretion. Pick after seeing real forward-vs-reversed spread on the authored corpus. |

## Open Questions

1. **Does the parsed `get-repo-map` tree expose symbol-level order reliably, or only file-level?**
   - What we know: `render.go` renders files (PageRank-ordered prefix) with elided symbol defs nested under each. File order is PageRank; symbol order within a file is elided-def appearance order.
   - What's unclear: whether elided defs preserve a stable rank-meaningful order *within* a file, or just source order. D-01 wants symbol-level gold.
   - Recommendation: At capture time, derive symbol order as (file PageRank rank, then in-file def appearance order). Document this as the ranking contract. If within-file order isn't rank-meaningful, symbol-level nDCG still discriminates via *file*-level ordering carrying the symbols — the reversed-file-order ranker will still tank nDCG. Validate the discriminator margin actually bites on the authored corpus during the plan's anti-vacuity task.

2. **Do `get-context` (personalized PageRank) and `get-repo-map` (uniform) both need a corpus, or is one sufficient for REPOEVAL-01?**
   - What we know: REPOEVAL-01 names both verbs. `get-context` requires seed files.
   - Recommendation: Author gold once; capture BOTH rankings (uniform + seeded-from-`files.solution`) into the committed artifact; score both. Seed `get-context` from the `files.solution` stub path. If authoring cost is high, plan may scope REPOEVAL-01 to `get-repo-map` as the headline with `get-context` as a reported secondary — confirm with the requirement text in plan.

## Sources

### Primary (HIGH confidence — verified in-repo this session)
- `internal/lint/ablationleakage/analyzer.go` — leaf-boundary scope (`bench/runners` only; NOT evaluators)
- `bench/evaluators/editsim/editsim.go` + `editsim_test.go` — `ES` signature + discriminator-test pattern
- `bench/aggregator/aider_edit_baseline.go` + `_test.go` — deterministic baseline render + byte-reproducible + anti-vacuity pattern
- `bench/runtime/aider_edit_baseline_regen.go` — `//go:build ignore` HELIX_BIN-gated regenerator pattern
- `internal/skill/repomap/skill.go:368-441` — `get_repo_map` returns `{"tree":...}` envelope, not a symbol list
- `internal/repomap/render.go` — `RenderBudgeted` PageRank-prefix tree rendering
- `internal/fuzzy/match.go` + `types.go` + `strategies.go` — `internal/errors` import (not stdlib), `ErrAmbiguous`/`ErrNoMatch` sentinels, 4 strategy labels
- `bench/datasets/aider-polyglot/loader.go` — `LoadExercise` + `Config.Files.Solution`
- `bench/datasets/aider-polyglot/fixtures/{go,python,rust}/exercises/practice/*/.meta/config.json` + `example.go` — gold ground-truth source
- `.gitignore:271-276` + `Makefile:160` — committed-baseline allowlist + regen-target precedent

### Secondary (MEDIUM confidence)
- `bench/aggregator/bootstrap.go` — `rand.NewPCG(seed,seed2)` seeded determinism

### Tertiary (LOW confidence)
- IR metric formulas (nDCG/recall/MRR) — standard definitions `[CITED: en.wikipedia.org/wiki/Discounted_cumulative_gain]`; conventions confirmed-by-construction, not version-sensitive

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new deps; all primitives verified in-repo
- Architecture (committed-vs-committed + separate regenerator): HIGH — direct Phase 100 precedent read end-to-end
- Pitfalls: HIGH — each grounded in a specific verified file/line (analyzer scope, tree-text output, fuzzy non-stdlib import, sentinel semantics)
- Metric formulas: MEDIUM — standard IR definitions; relevance-gain convention (A1) is the one open choice

**Research date:** 2026-06-23
**Valid until:** 2026-07-23 (stable — internal codebase, additive phase, no fast-moving external deps)
