// Package fuzzy implements a pure line-based fuzzy text matcher with a
// 4-strategy cascade (exact, whitespace-normalized, indentation-flexible,
// fail-with-diff), ellipsis segmentation, and Aider-style indentation
// reflow. It performs no I/O — callers read source, invoke Match, and
// decide whether to write.
package fuzzy

// Strategy enumerates the match strategies in the 4-tier cascade.
// The string values are part of the public contract: they are reported
// back to agents in tool responses (FUZZ-02) and MUST NOT change without
// a coordinated update to downstream consumers.
type Strategy string

const (
	// StrategyExact is byte-for-byte line equality. Score tier: 1.0.
	StrategyExact Strategy = "exact"
	// StrategyWhitespace trims leading/trailing whitespace per line before
	// comparing. Internal whitespace inside a line still must match.
	// Score tier: 0.95.
	StrategyWhitespace Strategy = "whitespace_normalized"
	// StrategyIndentationFlex strips leading " \t" per line before comparing.
	// Internal whitespace inside a line still must match. Score tier: 0.85.
	StrategyIndentationFlex Strategy = "indentation_flexible"
	// StrategyFailed indicates no strategy produced a unique match; Match
	// returns serr.InvalidArgs with a unified-diff-style payload.
	// Score tier: 0.0.
	StrategyFailed Strategy = "failed"
)

// Options configures a single call to Match. Zero-value is valid:
// Replacement="" means the caller is probing for a match without
// substituting; AllowEllipsis=false disables "..." segmentation.
type Options struct {
	// Replacement is the text the engine will reflow onto the matched
	// region's indentation and return via Result.ReplacementText.
	Replacement string
	// AllowEllipsis opts into "..."-on-own-line segmentation (FUZZ-08).
	// When false, "..." lines are treated as literal text.
	AllowEllipsis bool
}

// Result reports a successful match. All byte offsets are relative to
// the `source` string passed to Match (not file offsets).
type Result struct {
	// Strategy is the tier that produced the match.
	Strategy Strategy
	// Score is the discrete tier value: 1.0 / 0.95 / 0.85 / 0.0.
	Score float64
	// StartByte is the inclusive byte offset of the matched region in source.
	StartByte int
	// EndByte is the exclusive byte offset of the matched region in source.
	EndByte int
	// MatchedText is source[StartByte:EndByte] — the original region.
	MatchedText string
	// ReplacementText is Options.Replacement after common-prefix dedent and
	// reapplication of the matched region's leading whitespace. Empty
	// replacement lines are preserved as empty (no trailing whitespace).
	ReplacementText string
}
