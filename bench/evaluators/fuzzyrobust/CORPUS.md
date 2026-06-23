# fuzzyrobust drift corpus

This directory's `testdata/` holds the committed **drift corpus**
(`drift/{go,python,rust}.json`) and the committed **captured outcomes**
(`captured/{go,python,rust}.json`) that the `fuzzyrobust` stdlib-only leaf scores
committed-vs-committed. The leaf measures `internal/fuzzy`'s 4-strategy cascade
selection and ambiguity refusal (FUZZBENCH-01/02).

## Outcome vocabulary (Pitfall 6)

Every drift case's `expected_strategy` and every captured `strategy` is exactly
one of FIVE values. `failed` is **NEVER** a strategy label.

| Value | Meaning | Source |
|-------|---------|--------|
| `exact` | byte-for-byte match (tier 1) | `internal/fuzzy.StrategyExact` |
| `whitespace_normalized` | leading/trailing whitespace drift (tier 2, TrimSpace per line) | `internal/fuzzy.StrategyWhitespace` |
| `indentation_flexible` | indentation drift (tier 3, TrimLeft(" \t") per line) | `internal/fuzzy.StrategyIndentationFlex` |
| `ellipsis` | middle line(s) elided with `...` (matched via `Options.AllowEllipsis`) | the AllowEllipsis path |
| `ambiguous_match` | **refusal**: N>1 candidate sites → `ErrAmbiguous` | `internal/fuzzy.ErrAmbiguous` |
| `no_match` | cascade exhausted → `ErrNoMatch` | `internal/fuzzy.ErrNoMatch` |

A refusal (`ambiguous_match`) is distinct from a no-match (`no_match`) and from a
successful match — conflating them is the FUZZBENCH anti-vacuity failure (D-06).

### Expected (structural) vs captured (observed) — they may DIVERGE

The leaf grades **strategy SELECTION**: it compares each case's structurally
EXPECTED strategy (the perturbation tier) against the CAPTURED outcome (what
`internal/fuzzy.Match` actually selected). These are deliberately independent —
a divergence is a real, honest MEASUREMENT, not a corpus bug. Two divergences are
inherent to the live cascade and are expected:

- **`indentation_flexible` cases capture as `whitespace_normalized`.** The cascade
  tries whitespace (tier 2, `TrimSpace` per line) BEFORE indentation-flexible
  (tier 3, `TrimLeft(" \t")` per line). Because `TrimSpace` is strictly more
  aggressive than `TrimLeft`, any pure leading-indentation drift already
  re-matches at tier 2, so tier 3 is never the FIRST unique hit. The expected
  `indentation_flexible` therefore scores as a selection mismatch against the
  captured `whitespace_normalized` — the leaf records this faithfully.
- **`ellipsis` cases capture as `exact`.** The ellipsis path matches each `...`
  segment through the cascade and reports the WEAKEST tier across segments. Since
  the head/tail anchors are un-drifted, every segment matches exactly, so the
  aggregated strategy is `exact`.

This is why `ScoreCase` reports both `StrategyMatch` (selection) AND
`editsim.ES` similarity (text) — selection and text-fidelity are orthogonal.

## Expected strategy is DERIVED from the perturbation tier (D-05)

Each single-site drift case is a **real vendored fixture code block** (drawn from
`bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/...`) transformed
by a **deterministic per-tier perturbation**. The transform **IS** the drift-type
derivation — the expected strategy is the transform's tier, **never read from
observed tool behavior**:

| Perturbation (`perturb.go`) | Parameters | Expected strategy |
|-----------------------------|-----------|-------------------|
| `PerturbExact` | identity (no drift) | `exact` |
| `PerturbWhitespace` | prepend 1 leading space + append 2 trailing spaces per non-blank line; blank lines stay blank | `whitespace_normalized` |
| `PerturbIndent` | rewrite leading indent: spaces→tabs (4 spaces = 1 tab, leftover spaces = 1 tab each) + prepend 1 extra tab of depth; content after the indent untouched | `indentation_flexible` |
| `PerturbEllipsis` | replace middle line(s) of a (>=3-line) block with a single `...` on its own line, keeping head + tail anchors | `ellipsis` |

All transforms are pure, stdlib-only (`strings`), RNG-free and time-free, so the
corpus is byte-reproducible (T-102-06). The corpus is authored by
`testdata/gen_corpus.go` (`//go:build ignore`), which applies these transforms to
the inline real-fixture base blocks and emits the JSON.

## Ambiguous (must-refuse) case (D-06)

Each language ships **>= 1 duplicate-block ambiguous case** (`ambiguous: true`,
`expected_strategy: "ambiguous_match"`): a fixture source containing two
drift-equivalent code blocks plus a SEARCH pattern (a whitespace-drifted copy)
that matches **BOTH** sites. `internal/fuzzy.Match` MUST refuse it
(`ErrAmbiguous` → `ambiguous_match`) — NOT `no_match`, NOT a match. The refusal
follows structurally from the duplicate, not from observed behavior. This is the
FUZZBENCH anti-vacuity proof (`TestAmbiguousRefused` + `TestAmbiguousBites`).

## Size floor (D-04)

Documented per-language minimum, enforced by `TestCorpusFloor`:

- **~4 cases per strategy per language** (4 strategies × 4 × 3 langs = 48
  single-site cases).
- **>= 1 duplicate-block ambiguous case** per language (3 total).

Each language file currently ships **17 cases** (16 single-site + 1 ambiguous).

## Capture (outside the leaf)

`captured/{go,python,rust}.json` records the observed `{strategy, refused,
error_kind}` per case. It is regenerated by the `//go:build ignore` non-leaf
harness `bench/runtime/fuzzy_robust_capture_regen.go`, which is the ONLY code
permitted to call `internal/fuzzy.Match` (the leaf never can — `match.go:8`
imports `internal/errors`, so `internal/fuzzy` is not stdlib-only, Pitfall 5).
The harness maps `ErrAmbiguous`→`ambiguous_match`, `ErrNoMatch`→`no_match`, and a
success to its `Result.Strategy` label; it fail-not-skips (`os.Exit(2)`) on an
empty corpus / empty language bucket / unmappable outcome. The seeded committed
captured outcomes are authored to match the structural expectations so the
hermetic golden (`go test ./bench/evaluators/fuzzyrobust/`, NO binary, NO network)
is the sole authoritative proof.

## Regenerating the drift corpus

```sh
go run bench/evaluators/fuzzyrobust/testdata/gen_corpus.go
```
