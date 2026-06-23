---
phase: 101-opt-in-llm-behavioral-adoption-scorecard
fixed_at: 2026-06-23T00:00:00Z
review_path: .planning/phases/101-opt-in-llm-behavioral-adoption-scorecard/101-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 101: Code Review Fix Report

**Fixed at:** 2026-06-23
**Source review:** .planning/phases/101-opt-in-llm-behavioral-adoption-scorecard/101-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5
- Fixed: 5
- Skipped: 0

## Fixed Issues

### WR-01: `WorstDimensions` ordering is non-deterministic on tied averages

**Files modified:** `test/oracle/judge/aggregate.go`
**Commit:** 2b045abd
**Applied fix:** Added a name-keyed tiebreaker to the `sort.Slice` comparator in
`Aggregate`. When two sub-0.7 dimensions share the same average, the comparator
now falls back to lexicographic ordering on the dimension name, making
`WorstDimensions` (and the serialized `aggregate.json`) reproducible across runs.

### WR-02: Negative-exemplar test header claimed a "FAILING verdict" but asserts `soft_fail`

**Files modified:** `test/oracle/judge/adoption_exemplar_test.go`
**Commit:** e7aac5ec
**Applied fix:** Reworded the `TestAdoptionNegativeExemplarVerdict` doc comment to
state that a lone adoption=0.0 yields a non-pass `soft_fail` (one-zero rule), and
that the second `doubleZero` case escalates to a hard `fail`. This matches the
current `ComputeVerdict` threshold logic and removes the misleading "FAILING
verdict" phrasing for the single-zero case. Comment-only change; the assertions
were already correct against the threshold logic, so no test behavior changed.
The reviewer's SC#2-hard-fail alternative was intentionally NOT taken — flipping
the assertion or escalating the threshold would change verdict semantics without
a roadmap mandate; the clarifying reword is the safe resolution.

### IN-01: `intact-04` fixture used positional args diverging from documented CLI syntax

**Files modified:** `test/oracle/adopt/testdata/transcripts/intact-04.json`
**Commit:** 0abb6ece
**Applied fix:** Replaced the positional `helix rename-symbol FormatToolList
RenderToolList` response with the real verb's flag contract:
`helix rename-symbol --path internal/cli/setup_output.go --line 42 --column 6
--new-name RenderToolList`. The reviewer's suggested `--symbol` flag does not
exist on the real `rename-symbol` verb (verbs_gen.go uses `--path --line --column
--new-name`), so the fixture was aligned to the actual flag contract rather than
the suggested-but-nonexistent form. Classification is unchanged (still keys on the
`helix ` prefix; hermetic adopt test passes).

### IN-02: Complementarity was a fixture property, not an observable invariant

**Files modified:** `test/oracle/adopt/scorecard.go`
**Commit:** a334e334
**Applied fix:** Added an `Unclassified` int field to `ScorecardResult` and
populated it in `Scorecard`: responses that are neither a helix choice nor a
fallback prefix are now counted, making a `ChoiceRate + FallbackRate < 1.0`
bucket observable rather than silently absorbed. Updated the struct doc comment
to state the 1.0-sum is contingent on every response being classifiable. The
per-bucket classification was refactored from two independent `if`s to a
priority `switch` (chose > fellBack > unclassified); the review confirmed `chose`
and `fellBack` are mutually exclusive, so existing counts are unchanged. Hermetic
adopt test passes.

### IN-03: `fallbackPrefixes` was broader than the rubric/skill's stated fallback set

**Files modified:** `test/oracle/judge/rubric.go`
**Commit:** 55813bc1
**Applied fix:** Added `rg` and `ls` to the rubric `adoption` anchor's
negative-exemplar list so the LLM-judge prompt and the hermetic classifier
(`fallbackPrefixes = {grep, sed, cat, find, rg, ls}`) enumerate the same fallback
vocabulary. No test asserts the exact anchor text, and the `RubricPrompt`
`Contains "grep"` assertion in the exemplar test still holds; `go vet -tags
llmjudge` passes.

---

_Fixed: 2026-06-23_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
