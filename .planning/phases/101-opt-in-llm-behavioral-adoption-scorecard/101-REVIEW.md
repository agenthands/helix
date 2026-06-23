---
phase: 101-opt-in-llm-behavioral-adoption-scorecard
reviewed: 2026-06-23T00:00:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - test/oracle/adopt/scorecard.go
  - test/oracle/adopt/scorecard_test.go
  - test/oracle/adopt/testdata/transcripts/intact-01.json
  - test/oracle/adopt/testdata/transcripts/intact-02.json
  - test/oracle/adopt/testdata/transcripts/intact-03.json
  - test/oracle/adopt/testdata/transcripts/intact-04.json
  - test/oracle/adopt/testdata/transcripts/intact-05.json
  - test/oracle/adopt/testdata/transcripts/intact-06.json
  - test/oracle/adopt/testdata/transcripts/sabotaged-01.json
  - test/oracle/adopt/testdata/transcripts/sabotaged-02.json
  - test/oracle/adopt/testdata/transcripts/sabotaged-03.json
  - test/oracle/adopt/testdata/transcripts/sabotaged-04.json
  - test/oracle/adopt/testdata/transcripts/sabotaged-05.json
  - test/oracle/adopt/testdata/transcripts/sabotaged-06.json
  - test/oracle/judge/adoption_exemplar_test.go
  - test/oracle/judge/aggregate.go
  - test/oracle/judge/rubric.go
  - test/oracle/llm/adoption_scorecard_test.go
  - test/oracle/llm/skill_trigger_test.go
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
---

# Phase 101: Code Review Report

**Reviewed:** 2026-06-23
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Reviewed the Phase 101 opt-in LLM behavioral adoption scorecard: the hermetic
build-tag-free scorer (`test/oracle/adopt`), its committed fixtures, the
`llmjudge`-tagged judge rubric/aggregator, and the `llm`-tagged live legs.

The core hermetic scorer (`scorecard.go`) is well-constructed and adversarially
defensible. I verified every claim that mattered:

- `go test ./test/oracle/adopt/` passes hermetically (no tag, no key, no network).
- `StripDecisionMatrix` correctly handles the real embedded SKILL.md, where
  `## Decision matrix` is the *last* `## ` heading — it hits the `j < 0` branch
  and returns `skill[:i]`, materially shortening the body (confirmed against
  `internal/cli/skills/helix/SKILL.md`, 93 lines, single `## ` heading at line 35).
  `TestSabotageNonNoop` would correctly turn RED if that heading were renamed.
- `ClassifyChoice` is prefix-keyed and lowercased; `chose` and `fellBack` are
  mutually exclusive (helix-prefixed vs shell-tool-prefixed), so no response
  double-counts. The `FirstCommand` decoration-stripping handles inline fences,
  fenced blocks, `$ `/`> ` prompts, and uppercase. Verified via a standalone driver.
- The revert-and-fail margin is real: intact fixtures all classify as `helix`
  choices (choice_rate 1.0), sabotaged all classify as fallbacks (choice_rate 0.0),
  drop 1.0 >= MaterialDrop 0.4.
- `go vet -tags llmjudge ./test/oracle/judge/` and `go vet -tags llm ./test/oracle/llm/`
  both pass; `gofmt -l` is clean across all reviewed `.go` files.

No BLOCKER-class defects (no security issues, no crashes, no data loss, no vacuity
hole). Two WARNINGs: a real non-determinism defect in the judge aggregator's
`WorstDimensions` ordering, and a comment/behavior mismatch in the negative-exemplar
test that mislabels a `soft_fail` as a "FAILING verdict." Three INFO items cover
fixture realism and minor doc drift.

## Warnings

### WR-01: `WorstDimensions` ordering is non-deterministic on tied averages

**File:** `test/oracle/judge/aggregate.go:80-88`
**Issue:** `WorstDimensions` is populated by ranging over the `report.DimensionAvgs`
map — Go map iteration order is randomized — and then sorted with `sort.Slice`
(not stable) keyed *only* on the average value:

```go
for dim, avg := range report.DimensionAvgs {   // random iteration order
    if avg < 0.7 {
        report.WorstDimensions = append(report.WorstDimensions, dim)
    }
}
sort.Slice(report.WorstDimensions, func(i, j int) bool {
    return report.DimensionAvgs[...[i]] < report.DimensionAvgs[...[j]] // value-only comparator
})
```

When two or more sub-0.7 dimensions have the **same** average (a common case —
e.g. several dimensions all averaging 0.5), the comparator returns false for both
orderings, so `sort.Slice` leaves them in their (already randomized) input order.
The resulting `WorstDimensions` slice is therefore non-deterministic across runs
for tied dimensions. This report is serialized to `testdata/aggregate.json` via
`WriteAggregate`, so the instability surfaces as spurious diff churn and makes the
artifact non-reproducible. No test covers `WorstDimensions` ordering, so the defect
is uncaught.

**Fix:** Add a deterministic tiebreaker on the dimension name:

```go
sort.Slice(report.WorstDimensions, func(i, j int) bool {
    a, b := report.WorstDimensions[i], report.WorstDimensions[j]
    if report.DimensionAvgs[a] != report.DimensionAvgs[b] {
        return report.DimensionAvgs[a] < report.DimensionAvgs[b]
    }
    return a < b // stable, name-keyed tiebreak
})
```

`WorstPerformers` is already safe (`sort.Strings`), so only this slice needs the fix.

### WR-02: Negative-exemplar test header claims a "FAILING verdict" but asserts `soft_fail`

**File:** `test/oracle/judge/adoption_exemplar_test.go:12-18, 31-33`
**Issue:** The doc comment states the adoption=0.0 grep response "flows through
`ComputeVerdict` to a FAILING verdict ... proving the rubric can demonstrably FAIL
rather than being a trivially-always-pass gate." But the assertion is:

```go
require.Equal(t, "soft_fail", grepResponse.Verdict, ...)
```

A single zero is `soft_fail`, not `fail`, per `ComputeVerdict` (`rubric.go:98`).
`soft_fail` is a non-pass verdict, so the anti-vacuity intent is still met by the
second `doubleZero` case (which is a true `fail`). However the header's
"FAILING verdict" phrasing for the *single*-zero case is inaccurate and could
mislead a future maintainer into "fixing" the assertion to `fail`, which would
then turn RED for the wrong reason. If ROADMAP SC#2 actually requires a single
grep response to produce a hard `fail` (not merely non-pass), then the rubric
thresholds — not the test — are wrong and a single adoption=0.0 should be weighted
to escalate. Clarify which is intended.

**Fix:** Reword the header comment to say "a non-pass (`soft_fail`) verdict" for the
single-zero case, OR — if SC#2 requires a hard fail on a lone grep — change
`ComputeVerdict` so an adoption=0.0 is treated as escalating (e.g. count adoption
as a mandatory dimension), and update the assertion to match. Do not silently flip
the assertion to `fail` against the current threshold logic.

## Info

### IN-01: `intact-04` fixture uses positional args that diverge from the documented CLI syntax

**File:** `test/oracle/adopt/testdata/transcripts/intact-04.json:7`
**Issue:** The fixture response is `helix rename-symbol FormatToolList RenderToolList`,
but the embedded SKILL.md decision matrix documents `helix rename-symbol --new-name=...`.
Classification is unaffected (only the `helix ` prefix is load-bearing), but the
fixture is presented as a realistic transcript and the positional form does not match
the real verb's flag contract.

**Fix:** Align the fixture with the documented form, e.g.
`helix rename-symbol --symbol FormatToolList --new-name RenderToolList`, so the
committed transcripts model real invocations.

### IN-02: Complementarity (`ChoiceRate + FallbackRate == 1.0`) is a fixture property, not an enforced invariant

**File:** `test/oracle/adopt/scorecard.go:38-47, 124-146`
**Issue:** `ScorecardResult`'s doc and `TestComplementary`/`TestSabotagedSkillRevertAndFail`
assert the two rates sum to 1.0, but `Scorecard` does not enforce it. A response that
is neither a `helix ` choice nor a fallback prefix (prose, or any non-listed first
command) classifies as `(false, false)` and silently lowers *both* rates so they sum
to < 1.0. The current committed fixtures happen to be clean, so the invariant holds
today, but a future fixture (or live transcript fed through the same scorer) could
break it without any guard in the scorer itself.

**Fix:** Optional — if complementarity is a contract, surface an `Unclassified` count
on `ScorecardResult` and have `Scorecard` populate it, so a non-complementary bucket
is observable rather than silently absorbed. At minimum, document that the 1.0 sum is
contingent on every response being classifiable.

### IN-03: `fallbackPrefixes` is broader than the rubric/skill's stated fallback set

**File:** `test/oracle/adopt/scorecard.go:72`
**Issue:** `fallbackPrefixes = {"grep ", "sed ", "cat ", "find ", "rg ", "ls "}`
includes `rg ` and `ls `, but the judge rubric anchor (`rubric.go:145`) and the
SKILL.md baseline enumerate only `grep, sed, cat, find`. The wider set is defensible
(rg and ls are equally "standard tools the skill displaces"), but the hermetic
classifier and the LLM-judge rubric now use *different* fallback vocabularies, so the
two adoption signals are not measuring exactly the same thing. The trailing-space
keying is correct (`ls ` won't match `lsp`).

**Fix:** Either add `rg`/`ls` to the rubric anchor's negative-exemplar list so both
surfaces agree, or document why the hermetic classifier intentionally uses a stricter
superset than the judge prompt.

---

_Reviewed: 2026-06-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
