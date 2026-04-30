package fuzzy

import (
	"fmt"
	"strings"

	serr "github.com/agenthands/helix/internal/errors"
)

// Match runs the 4-strategy cascade over source and search, returning a
// Result on success or an error on ambiguity / no-match / invalid input.
// The engine is pure -- it performs no I/O. Callers read source, invoke
// Match, inspect the Result, and decide whether to write.
//
// Cascade (CONTEXT.md S2, locked):
//
//  1. Exact          (score 1.0)
//  2. Whitespace     (score 0.95) -- TrimSpace per line
//  3. IndentFlex     (score 0.85) -- TrimLeft(" \t") per line
//  4. Failed         (score 0.0)  -- serr.InvalidArgs with unified-diff-style detail
//
// At any strategy, N > 1 hits returns serr.InvalidArgs immediately and
// does NOT cascade -- add anchor context to disambiguate (CONTEXT.md S4).
func Match(source, search string, opts Options) (*Result, error) {
	if strings.TrimSpace(search) == "" {
		return nil, serr.New(serr.InvalidArgs, "empty search block")
	}

	if opts.AllowEllipsis {
		segments, err := segmentSearch(search)
		if err != nil {
			return nil, err
		}
		if segments != nil {
			return matchSegmented(source, search, segments, opts)
		}
	}

	return matchSingle(source, search, opts)
}

// matchSingle runs the cascade against a single (non-segmented) search block.
func matchSingle(source, search string, opts Options) (*Result, error) {
	sourceLines := splitLines(source)
	searchLines := splitLines(search)
	byteOffsets := lineByteOffsets(sourceLines)

	// Strategy 1: Exact
	hits := sweepExact(sourceLines, searchLines)
	switch len(hits) {
	case 0:
		// fall through
	case 1:
		return buildResult(source, sourceLines, searchLines, byteOffsets, hits[0], StrategyExact, 1.0, opts)
	default:
		return nil, ambiguityError(StrategyExact, hits)
	}

	// Strategy 2: Whitespace-normalized
	hits = sweepWhitespace(sourceLines, searchLines)
	switch len(hits) {
	case 0:
		// fall through
	case 1:
		return buildResult(source, sourceLines, searchLines, byteOffsets, hits[0], StrategyWhitespace, 0.95, opts)
	default:
		return nil, ambiguityError(StrategyWhitespace, hits)
	}

	// Strategy 3: Indentation-flexible
	hits = sweepIndentFlex(sourceLines, searchLines)
	switch len(hits) {
	case 0:
		// fall through
	case 1:
		return buildResult(source, sourceLines, searchLines, byteOffsets, hits[0], StrategyIndentationFlex, 0.85, opts)
	default:
		return nil, ambiguityError(StrategyIndentationFlex, hits)
	}

	// Strategy 4: Failed -- return unified-diff-style payload.
	nearest := findNearestWindow(sourceLines, searchLines)
	return nil, failureError(search, strings.Join(nearest, "\n"))
}

// matchSegmented dispatches each ellipsis segment through the cascade in
// order against a forward-only line-index cursor, ensuring segments
// match in order. FUZZ-08 (CONTEXT.md S3). Replacement must use the
// same segment count (validated here).
//
// Per-segment tier aggregation (FUZZ-02): the returned Result's Strategy
// and Score reflect the WEAKEST tier across all segments (the lowest
// score wins). A mixed exact+indent-flex ellipsis reports
// StrategyIndentationFlex/0.85, not StrategyExact/1.0.
func matchSegmented(source, search string, searchSegments []string, opts Options) (*Result, error) {
	replacementSegments, err := segmentSearch(opts.Replacement)
	if err != nil {
		return nil, err
	}
	if replacementSegments == nil {
		// Replacement has no "..." markers but search does -- counts differ.
		return nil, validateSegmentCounts(len(searchSegments), 1)
	}
	if err := validateSegmentCounts(len(searchSegments), len(replacementSegments)); err != nil {
		return nil, err
	}

	sourceLines := splitLines(source)
	byteOffsets := lineByteOffsets(sourceLines)

	// cursor is an absolute line index into sourceLines. Each segment
	// runs against sourceLines[cursor:] and advances cursor past its
	// matched region.
	cursor := 0
	firstStartLine := -1
	lastEndLine := 0

	// Tier aggregation: track the weakest strategy/score across segments.
	weakestStrategy := StrategyExact
	weakestScore := 1.0

	var matchedTextBuilder strings.Builder
	var replacementBuilder strings.Builder

	for i, seg := range searchSegments {
		segTrim := strings.TrimPrefix(strings.TrimSuffix(seg, "\n"), "\n")
		if segTrim == "" {
			return nil, serr.New(serr.InvalidArgs, "empty ellipsis segment after trim")
		}
		repTrim := strings.TrimPrefix(strings.TrimSuffix(replacementSegments[i], "\n"), "\n")

		subStart, subEnd, subStrategy, subScore, subMatched, subReplacement, subErr :=
			matchSingleLineWindow(sourceLines, cursor, segTrim, repTrim)
		if subErr != nil {
			return nil, subErr
		}

		if firstStartLine < 0 {
			firstStartLine = subStart
		}
		lastEndLine = subEnd
		cursor = subEnd // forward-only: next segment starts after this one

		// Aggregate the weakest tier across segments.
		if subScore < weakestScore {
			weakestScore = subScore
			weakestStrategy = subStrategy
		}

		if i > 0 {
			matchedTextBuilder.WriteString("\n...\n")
			replacementBuilder.WriteString("\n...\n")
		}
		matchedTextBuilder.WriteString(subMatched)
		replacementBuilder.WriteString(subReplacement)
	}

	// Translate line indices to absolute byte offsets in the ORIGINAL
	// source string.
	startByte := byteOffsets[firstStartLine]
	var endByte int
	if lastEndLine >= len(byteOffsets) {
		endByte = len(source)
	} else {
		endByte = byteOffsets[lastEndLine]
		if endByte > 0 {
			endByte-- // trim the '\n' preceding the next line
		}
	}
	if endByte > len(source) {
		endByte = len(source)
	}

	return &Result{
		Strategy:        weakestStrategy,
		Score:           weakestScore,
		StartByte:       startByte,
		EndByte:         endByte,
		MatchedText:     matchedTextBuilder.String(),
		ReplacementText: replacementBuilder.String(),
	}, nil
}

// matchSingleLineWindow runs the 4-strategy cascade against
// sourceLines[cursor:] for the given search text, returning absolute
// line indices (startLine, endLine) plus the strategy/score/matched
// text/replacement text. Used by matchSegmented to avoid reconstructing
// substring windows.
//
// endLine is exclusive (one past the last matched line) so the caller
// can use it directly as the next segment's cursor.
func matchSingleLineWindow(sourceLines []string, cursor int, search, replacement string) (int, int, Strategy, float64, string, string, error) {
	window := sourceLines[cursor:]
	searchLines := splitLines(search)

	runStrategy := func(hits []int, strategy Strategy, score float64) (int, int, Strategy, float64, string, string, error, bool) {
		switch len(hits) {
		case 0:
			return 0, 0, "", 0, "", "", nil, false
		case 1:
			relStart := hits[0]
			absStart := cursor + relStart
			absEnd := absStart + len(searchLines)
			matchedRegion := sourceLines[absStart:absEnd]
			matchedText := strings.Join(matchedRegion, "\n")
			replacementText := reflow(searchLines, matchedRegion, replacement)
			return absStart, absEnd, strategy, score, matchedText, replacementText, nil, true
		default:
			// Translate to absolute line numbers; ambiguityError will +1 them.
			abs := make([]int, len(hits))
			for i, h := range hits {
				abs[i] = cursor + h
			}
			return 0, 0, "", 0, "", "", ambiguityError(strategy, abs), true
		}
	}

	if s, e, st, sc, mt, rt, err, done := runStrategy(sweepExact(window, searchLines), StrategyExact, 1.0); done {
		return s, e, st, sc, mt, rt, err
	}
	if s, e, st, sc, mt, rt, err, done := runStrategy(sweepWhitespace(window, searchLines), StrategyWhitespace, 0.95); done {
		return s, e, st, sc, mt, rt, err
	}
	if s, e, st, sc, mt, rt, err, done := runStrategy(sweepIndentFlex(window, searchLines), StrategyIndentationFlex, 0.85); done {
		return s, e, st, sc, mt, rt, err
	}
	nearest := findNearestWindow(window, searchLines)
	return 0, 0, "", 0, "", "", failureError(search, strings.Join(nearest, "\n"))
}

// buildResult assembles a Result from a successful sweep hit. lineIdx is
// the starting line index in sourceLines. The byte span runs from the
// start of lineIdx to the start of (lineIdx + len(searchLines)) -- or
// len(source) if the match reaches EOF.
func buildResult(source string, sourceLines, searchLines []string, byteOffsets []int, lineIdx int, strategy Strategy, score float64, opts Options) (*Result, error) {
	startByte := byteOffsets[lineIdx]
	endLine := lineIdx + len(searchLines)
	var endByte int
	if endLine >= len(byteOffsets) {
		endByte = len(source)
	} else {
		endByte = byteOffsets[endLine]
		if endByte > 0 {
			endByte-- // trim the trailing '\n' that belongs to the last matched line
		}
	}
	if endByte > len(source) {
		endByte = len(source)
	}
	if startByte > endByte {
		startByte = endByte
	}

	matchedRegion := sourceLines[lineIdx:endLine]
	matchedText := source[startByte:endByte]
	replacementText := reflow(searchLines, matchedRegion, opts.Replacement)

	return &Result{
		Strategy:        strategy,
		Score:           score,
		StartByte:       startByte,
		EndByte:         endByte,
		MatchedText:     matchedText,
		ReplacementText: replacementText,
	}, nil
}

// ambiguityError builds the FUZZ-07 error payload using formatAmbiguity.
// Hit indices from the sweep are 0-indexed line numbers; we translate to
// 1-indexed before formatting (CONTEXT.md S4 mandate).
func ambiguityError(strategy Strategy, hits []int) error {
	oneIndexed := make([]int, len(hits))
	for i, h := range hits {
		oneIndexed[i] = h + 1
	}
	msg := formatAmbiguity(oneIndexed)
	return serr.New(serr.InvalidArgs, msg).
		WithDetail(fmt.Sprintf("strategy=%s count=%d", strategy, len(hits)))
}

// failureError builds the FUZZ-01 fail-tier error payload using formatFailureDiff.
func failureError(search, nearest string) error {
	return serr.New(serr.InvalidArgs, "no fuzzy match found").
		WithDetail(formatFailureDiff(search, nearest))
}
