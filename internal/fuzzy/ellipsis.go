package fuzzy

import (
	"fmt"
	"regexp"
	"strings"

	serr "github.com/postfix/serena/internal/errors"
)

// dotsRe matches a "..." marker on its own line with optional leading
// spaces/tabs. (?m) enables multiline so ^ and $ anchor per-line. The
// pattern is anchored at both ends with a bounded character class --
// zero backtracking surface (ReDoS-safe, see T-25-02-01).
var dotsRe = regexp.MustCompile(`(?m)^[ \t]*\.\.\.$`)

// segmentSearch splits a search block on "..." markers that appear on
// their own line. Returns (nil, nil) when the block contains no such
// marker -- caller should then use the whole-block path. Returns an
// InvalidArgs error if any segment is empty (contains only whitespace).
func segmentSearch(search string) ([]string, error) {
	segments := dotsRe.Split(search, -1)
	if len(segments) == 1 {
		return nil, nil // no marker found
	}
	for i, seg := range segments {
		if strings.TrimSpace(seg) == "" {
			return nil, serr.New(serr.InvalidArgs,
				"empty ellipsis segment").
				WithDetail(fmt.Sprintf("segment %d of %d is empty; every segment must contain at least one non-empty anchor line", i+1, len(segments)))
		}
	}
	return segments, nil
}

// validateSegmentCounts ensures the search block and the replacement
// block have matching segment counts when ellipsis markers are used.
// Callers pass the counts from two independent segmentSearch calls.
func validateSegmentCounts(searchCount, replacementCount int) error {
	if searchCount != replacementCount {
		return serr.New(serr.InvalidArgs,
			"ellipsis segment count mismatch").
			WithDetail(fmt.Sprintf("search has %d segments, replacement has %d; both blocks must use the same number of '...' markers", searchCount, replacementCount))
	}
	return nil
}
