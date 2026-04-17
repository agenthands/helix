# Phase 25: Fuzzy Edit Engine -- Formal Verification

- **Phase:** 25 (Fuzzy Edit Engine)
- **Verified by:** Phase 29 (Formal Verification)
- **Date:** 2026-04-17
- **Test runner:** `go test ./internal/fuzzy/... -count=1 -v` and `go test ./internal/kernel/edit/... -count=1 -run "Fuzzy|fuzzy" -v`

## Summary Table

| Requirement | Status | Tests Cited | Code Location |
|-------------|--------|-------------|---------------|
| FUZZ-01 | PASS | 11 tests (cascade tiers + sweep subtests) | `internal/fuzzy/match.go` `matchSingle`, `internal/fuzzy/strategies.go` |
| FUZZ-02 | PASS | 6 tests (strategy/score reporting) | `internal/fuzzy/types.go` Result struct, `internal/kernel/edit/tools.go:186` |
| FUZZ-03 | PASS | 5 tests (reflow + end-to-end) | `internal/fuzzy/indent.go` `reflow`, `internal/fuzzy/match.go` `buildResult` |
| FUZZ-07 | PASS | 6 tests (ambiguity refusal) | `internal/fuzzy/match.go` `ambiguityError`, `matchSingle` |
| FUZZ-08 | PASS | 10 tests (ellipsis segmentation) | `internal/fuzzy/ellipsis.go` `segmentSearch`, `internal/fuzzy/match.go` `matchSegmented` |

---

## FUZZ-01: 4-Strategy Cascade

**Status:** PASS

### Code Evidence

- **`internal/fuzzy/match.go`** -- `matchSingle` (lines 43-84) implements the cascade:
  1. `sweepExact` (Strategy: exact, Score: 1.0) -- line 49
  2. `sweepWhitespace` (Strategy: whitespace_normalized, Score: 0.95) -- line 60
  3. `sweepIndentFlex` (Strategy: indentation_flexible, Score: 0.85) -- line 71
  4. `failureError` (Strategy: failed, Score: 0.0) -- line 83
- **`internal/fuzzy/strategies.go`** -- three sweep functions:
  - `sweepExact` (lines 11-30): byte-for-byte line equality
  - `sweepWhitespace` (lines 35-54): `strings.TrimSpace` per line
  - `sweepIndentFlex` (lines 59-78): `strings.TrimLeft(" \t")` per line
- **`internal/fuzzy/types.go`** -- Strategy constants: `StrategyExact`, `StrategyWhitespace`, `StrategyIndentationFlex`, `StrategyFailed`
- **`internal/fuzzy/diff.go`** -- `formatFailureDiff` produces unified-diff-style payload for the fail tier

### Test Evidence

All tests pass (`ok github.com/postfix/serena/internal/fuzzy 0.522s`):

| Test | Tier Exercised |
|------|---------------|
| `TestMatch_ExactStrategy` | Exact (tier 1) |
| `TestMatch_WhitespaceStrategy` | Whitespace (tier 2) |
| `TestMatch_IndentationStrategy` | IndentFlex (tier 3) |
| `TestMatch_FailWithDiff` | Fail (tier 4) |
| `TestSweepExact/single_hit` | Exact sweep |
| `TestSweepExact/ambiguous_2_hits` | Exact sweep (multi-hit) |
| `TestSweepExact/no_hit` | Exact sweep (miss) |
| `TestSweepExact/1-line_at_EOF_(off-by-one)` | Exact sweep (boundary) |
| `TestSweepWhitespace/leading_ws_ignored` | Whitespace sweep |
| `TestSweepWhitespace/trailing_ws_ignored` | Whitespace sweep |
| `TestSweepIndentFlex/tab_vs_spaces` | IndentFlex sweep |

---

## FUZZ-02: Strategy Reporting (match_strategy, similarity_score)

**Status:** PASS

### Code Evidence

- **`internal/fuzzy/types.go`** -- `Result` struct (lines 44-59) contains:
  - `Strategy Strategy` -- the tier that produced the match
  - `Score float64` -- discrete tier value: 1.0 / 0.95 / 0.85 / 0.0
- **`internal/kernel/edit/replace.go`** -- `FuzzyMatchInfo` struct (lines 17-20) carries `Strategy` and `Score` from fuzzy result to tool handler
- **`internal/kernel/edit/tools.go`** line 186 -- formats response: `match_strategy: %s\nsimilarity_score: %.2f`
- **`internal/fuzzy/match.go`** -- `buildResult` (line 258) populates `Result{Strategy: strategy, Score: score}`

### Test Evidence

| Test | What It Verifies |
|------|-----------------|
| `TestMatch_StrategyReporting/exact` | Strategy=exact reported |
| `TestMatch_StrategyReporting/whitespace` | Strategy=whitespace_normalized reported |
| `TestMatch_StrategyReporting/indent-diff-hits-whitespace` | Strategy=whitespace_normalized for indent diff |
| `TestMatch_ScoreTiers` | Score values: 1.0 (exact), 0.95 (whitespace), 0.85 (indent-flex) |
| `TestReplaceBodyFuzzy_ExactMatchWithinBody` | FuzzyMatchInfo.Strategy propagated through edit layer |
| `TestReplaceBodyFuzzy_WhitespaceNormalized` | FuzzyMatchInfo.Strategy=whitespace_normalized in edit layer |

---

## FUZZ-03: Indentation Preservation via Reflow

**Status:** PASS

### Code Evidence

- **`internal/fuzzy/indent.go`** -- full reflow pipeline:
  - `commonLeadingPrefix` (lines 14-33): finds longest common whitespace prefix across non-empty lines
  - `dedent` (lines 62-76): strips common prefix from replacement lines
  - `reapplyPrefix` (lines 81-91): prepends source indentation to non-empty lines
  - `reflow` (lines 105-120): orchestrates the full transform: searchPrefix -> dedent -> sourcePrefix -> reapply
- **`internal/fuzzy/match.go`** -- `buildResult` (line 256) calls `reflow(searchLines, matchedRegion, opts.Replacement)` to produce `ReplacementText`

### Test Evidence

| Test | What It Verifies |
|------|-----------------|
| `TestReflow_AppliesSourcePrefix` | Source indentation applied to dedented replacement |
| `TestReflow_TabsAndSpaces` | Mixed tab/space indentation handled correctly |
| `TestReflow_PreservesEmptyLines` | Empty lines not padded with trailing whitespace |
| `TestMatch_ReplacementText_Reflow` | End-to-end: Match returns reflowed ReplacementText |
| `TestReplaceBodyFuzzy_WhitespaceNormalized` | Full stack: edit layer applies reflow through fuzzy match |

Supporting unit tests for reflow internals:
- `TestCommonLeadingPrefix/*` (7 subtests)
- `TestDedent`
- `TestReapplyPrefix_PreservesEmptyLines`

---

## FUZZ-07: Ambiguity Refusal

**Status:** PASS

### Code Evidence

- **`internal/fuzzy/match.go`** -- `matchSingle` (lines 43-84):
  - Each cascade tier checks `len(hits)` via switch: `case 0:` falls through, `case 1:` returns result, `default:` returns `ambiguityError` immediately
  - The `default` branch does NOT fall through to the next tier -- it returns, preventing cascade past ambiguity
- **`internal/fuzzy/match.go`** -- `ambiguityError` (lines 271-279): builds `serr.InvalidArgs` with 1-indexed line numbers and strategy detail
- **`internal/fuzzy/diff.go`** -- `formatAmbiguity` formats hit locations with overflow suffix for >5 hits

### Test Evidence

| Test | What It Verifies |
|------|-----------------|
| `TestCascade_StopsOnAmbiguity` | Cascade halts at ambiguous tier (no fallthrough) |
| `TestAmbiguity_Exact` | Exact tier returns error on 2+ hits |
| `TestAmbiguity_LineNumberCap` | Line numbers capped at 5 in error message |
| `TestAmbiguity_OverflowSuffix` | "and N more" suffix for >5 hits |
| `TestAmbiguity_DoesNotCascade` | Whitespace tier ambiguity does not cascade to indent-flex |
| `TestSweepExact/ambiguous_2_hits` | sweepExact returns 2 hits for duplicated content |

Supporting formatting tests:
- `TestFormatAmbiguity/2_hits`
- `TestFormatAmbiguity/exactly_5_hits_(no_overflow_suffix)`
- `TestFormatAmbiguity/6_hits_(and_1_more)`
- `TestFormatAmbiguity/10_hits_(and_5_more)`

---

## FUZZ-08: Ellipsis/Placeholder Support

**Status:** PASS

### Code Evidence

- **`internal/fuzzy/ellipsis.go`**:
  - `dotsRe` (line 15): regex `(?m)^[ \t]*\.\.\.$` detects "..." on own line with optional leading whitespace
  - `segmentSearch` (lines 21-34): splits search on "..." markers, rejects empty segments
  - `validateSegmentCounts` (lines 39-46): ensures search and replacement have matching segment counts
- **`internal/fuzzy/match.go`**:
  - `Match` (lines 29-37): dispatches to `matchSegmented` when `AllowEllipsis=true` and `segmentSearch` returns non-nil segments
  - `matchSegmented` (lines 95-182): processes each segment through `matchSingleLineWindow` with forward-only cursor, aggregates weakest tier across segments

### Test Evidence

| Test | What It Verifies |
|------|-----------------|
| `TestEllipsis_TwoSegments` | Basic 2-segment matching with "..." separator |
| `TestEllipsis_MultipleSegments` | 3+ segment matching |
| `TestEllipsis_LeadingWhitespace` | "..." with leading whitespace still recognized as marker |
| `TestEllipsis_InlineLiteral` | Inline "..." (not on own line) NOT treated as marker |
| `TestEllipsis_EmptySegmentRejected/empty_leading` | Empty leading segment rejected |
| `TestEllipsis_EmptySegmentRejected/empty_trailing` | Empty trailing segment rejected |
| `TestEllipsis_EmptySegmentRejected/empty_middle` | Empty middle segment rejected |
| `TestEllipsis_ReplacementSegmentMismatch` | Mismatched segment counts rejected |
| `TestEllipsis_NoMarkerReturnsNil` | No "..." returns nil (whole-block path) |
| `TestEllipsis_InOrderMatching` | Segments match in forward-only order |
| `TestEllipsis_MixedTierAggregationReportsWeakest` | Weakest strategy/score across segments reported |
| `TestEllipsis_ByteOffsetsWithTrailingNewline` | Byte offsets correct with trailing newlines |

---

## Test Execution Output

### `go test ./internal/fuzzy/... -count=1 -v`

```
PASS
ok  github.com/postfix/serena/internal/fuzzy    0.522s
```

All 44 tests/subtests passed:
- TestFormatAmbiguity (4 subtests)
- TestFormatFailureDiff
- TestFormatFailureDiff_SingleLine
- TestFindNearestWindow_PicksBestMatchCount
- TestFindNearestWindow_NoMatchReturnsFirstWindow
- TestFindNearestWindow_SourceShorterThanSearch
- TestFormatFailureDiff_EmptyNearest
- TestEllipsis_TwoSegments
- TestEllipsis_MultipleSegments
- TestEllipsis_LeadingWhitespace
- TestEllipsis_InlineLiteral
- TestEllipsis_EmptySegmentRejected (3 subtests)
- TestEllipsis_ReplacementSegmentMismatch
- TestEllipsis_NoMarkerReturnsNil
- TestMatch_ExactStrategy
- TestMatch_WhitespaceStrategy
- TestMatch_IndentationStrategy
- TestMatch_FailWithDiff
- TestCascade_StopsOnAmbiguity
- TestMatch_StrategyReporting (3 subtests)
- TestMatch_ScoreTiers
- TestAmbiguity_Exact
- TestAmbiguity_LineNumberCap
- TestAmbiguity_OverflowSuffix
- TestAmbiguity_DoesNotCascade
- TestEllipsis_InOrderMatching
- TestEllipsis_MixedTierAggregationReportsWeakest
- TestEllipsis_ByteOffsetsWithTrailingNewline
- TestMatch_EmptySearch
- TestMatch_ReplacementText_Reflow
- TestMatch_ByteOffsets
- TestCommonLeadingPrefix (7 subtests)
- TestDedent
- TestReapplyPrefix_PreservesEmptyLines
- TestReflow_AppliesSourcePrefix
- TestReflow_TabsAndSpaces
- TestReflow_PreservesEmptyLines
- TestSweepExact (7 subtests)
- TestSweepWhitespace (4 subtests)
- TestSweepIndentFlex (4 subtests)

### `go test ./internal/kernel/edit/... -count=1 -run "Fuzzy|fuzzy" -v`

```
PASS
ok  github.com/postfix/serena/internal/kernel/edit    0.635s
```

All 5 edit-layer fuzzy tests passed:
- TestReplaceBodyFuzzy_ExactMatchWithinBody
- TestReplaceBodyFuzzy_WhitespaceNormalized
- TestReplaceBodyFuzzy_NoSearchBody_FullReplace
- TestReplaceBodyFuzzy_NoMatchInBody
- TestReplaceBodyFuzzy_OffsetTranslation
