# VERIFIED.md — Multi-Oracle Completion Gate (VERIFIED-03)

This document is the SC#3 acceptance artifact for the multi-oracle completion
gate implemented in `bench/evaluators/completion_gate/`. It records the exact
contract the gate enforces so the documented semantics stay honest. The
`verify-verified-md` Makefile target fails closed (non-zero exit) if any of the
required section headers below is missing.

The gate is the **non-test-bearing sibling producer** of `verified_correctness`
that `bench/evaluators/test_runner/test_runner.go:37-40` anticipates: where
`test_runner` derives `verified_correctness` from a test-suite exit code, this
gate derives it from agreement across three CrossCodeEval completion oracles. It
is an **additive** producer of the existing
`evaluators.Metrics.VerifiedCorrectness *bool` (metrics.go:21) — no
`result.v2.schema.json` field is added.

## Oracles

`verified_correctness = true` requires **ALL THREE** oracles to pass
(all-three-required; one failing oracle drives the composite to an explicit
`false`). The three oracles are the Plan 01 leaf scorers:

| Oracle | Scorer | Pass condition |
|--------|--------|----------------|
| Exact-Match (CM-EM) | `exactmatch.EM(pred, gold) bool` | raw string equality is `true` |
| Edit-Similarity (CM-ES) | `editsim.ES(pred, gold) float64` | `ES >= GateConfig.ESThreshold` |
| Identifier-Match (IM-EM) | `identmatch.Match(pred, gold) (em, f1)` | the `em` (set-equality) bool is `true` |

The composite rule, verbatim from `completion_gate.Grade`:

```
verified_correctness = EM(pred, gold)
    AND ES(pred, gold) >= cfg.ESThreshold
    AND identifier-match(pred, gold)
```

A single passing oracle never masquerades as full verification (T-86-02-02): the
single-oracle-fail test (`TestGrade_EMFails_VerifiedFalse`) asserts
`verified_correctness == false` when only EM fails, even with ES above threshold
and the identifier sets equal.

## Threshold

The edit-similarity threshold is a **per-oracle configurable knob**, not a
hardcoded constant. It is the field `GateConfig.ESThreshold float64` (the minimum
normalized edit similarity, inclusive, in `[0,1]`). The documented default is
`completion_gate.DefaultESThreshold = 0.9`.

Configurability is proven by `TestGrade_ThresholdKnob`: the SAME `(pred, gold)`
pair produces an `ES` value that is below `0.99` but at-or-above `0.5`, so moving
`ESThreshold` between those two values flips the edit-similarity oracle's
pass/fail outcome. The threshold is load-bearing, not cosmetic.

## Abstain

When a completion is **abstained** (low confidence), the gate fails **closed**:
`Grade(pred, gold, cfg, abstain=true)` returns `verified_correctness = &false` —
a real, non-nil `*bool` pointing to `false` — WITHOUT consulting the oracles.

> On abstain (low-confidence completion), the gate emits an explicit
> `verified_correctness=&false` — never a false-positive `true` and never a nil
> drop.

This is the security-relevant invariant (VERIFIED-03 / T-86-02-01): a
false-positive `verified_correctness=true` on a wrong or low-confidence
completion is the worst failure a benchmark can produce, so the gate prefers an
explicit `false` to any risk of a spurious `true`, and it never nil-drops the
metric on this path. The hermetic test `TestGrade_Abstain_ExplicitFalse` asserts
`res.VerifiedCorrectness != nil && *res.VerifiedCorrectness == false`.

**Abstain (`false`) is distinct from could-not-score (`nil`).** Abstain is a
deliberate, verified-false verdict. The reserved "could-not-score" path (for a
future non-total oracle) instead leaves the pointer `nil` and appends a
`MetricError` — "unknown", not "verified false". The present scorers are total on
every input, so the could-not-score path is unused today but the
`GateResult.Errs` field preserves it.

## Tokenizer

The identifier oracle (`identmatch`) tokenizes each completion to identifiers and
removes a language-agnostic keyword set before set comparison. The tokenizer
regex, cited verbatim from `identmatch.go`, is:

```
[A-Za-z_][A-Za-z0-9_]*
```

i.e. a leading ASCII letter or underscore followed by zero or more letters,
digits, or underscores. This is the ASCII identifier rule shared by Python, Java,
TypeScript, and C#; non-ASCII letters act as separators (they are not part of an
identifier token). Tokens are matched, NOT obtained by whitespace splitting (a
whitespace split would over-count punctuation and operators — RESEARCH Pitfall 4).

The language-agnostic **keyword set** removed after tokenization (cited from
`identmatch.go`, the common subset across Python / Java / TypeScript / C#) is:

```
if else elif for while do switch case default break continue return
function func def class interface enum struct extends implements
import from package using namespace new delete try catch finally throw throws
public private protected static final const let var val void
int long short byte char float double bool boolean string
true false null nil none undefined this self super in is as of typeof instanceof
and or not async await yield lambda with pass raise global nonlocal del assert
```

A shared keyword therefore neither helps nor hurts the score: the gate scores
user-chosen NAMES, not language syntax. Empty-vs-empty identifier sets (both
inputs empty, or both keyword-only) are an exact match (`em=true, f1=1.0`);
exactly-one-empty is disjoint (`em=false, f1=0.0`).

## EditSimilarity

The edit-similarity oracle is the CrossCodeEval CM-ES normalized character-level
edit similarity (cited from `editsim.go`):

```
ES(pred, gold) = 1 - levenshtein(pred, gold) / max(len(pred), len(gold))
```

where `len` is the **rune** count (multi-byte unicode counts once) and
`levenshtein` is the standard insert/delete/substitute edit distance over runes.
The result is always within `[0,1]`: `1.0` is identical, `0.0` is maximally
dissimilar. Both-empty short-circuits to `1.0`; exactly-one-empty is `0.0`.

This is explicitly **NOT** git-numstat line distance: `editsim` re-implements
Levenshtein locally and does not reuse
`patch_validator.EditDistancePatch` (the summed added+deleted LINE count from
`git diff --numstat`, an integer over a working tree — a completely different
metric). The `TestES_NumstatDiscriminator` fixture in Plan 01 guards against that
wrong-metric reuse (RESEARCH Pitfall 2), and `editsim` imports stdlib only.

## Proof

The hermetic fixture tests in
`bench/evaluators/completion_gate/gate_test.go` are the SOLE authoritative proof
of this gate — no network, no `HELIX_BIN`, no subprocess. They assert:
all-three-pass → `true`; any-single-oracle-fail → `false`; threshold knob flips
the ES comparison across two `ESThreshold` values; and abstain → explicit non-nil
`false`. Run them with `go test ./bench/evaluators/completion_gate/...`.
