package edit

import (
	"errors"

	"github.com/postfix/serena/internal/fuzzy"
)

// MetricsSink is the minimal surface internal/kernel/edit needs from the
// observability layer (D-08 invariant: edit never imports internal/obs).
// internal/kernel/fileops also imports edit's MetricsSink to keep one
// definition (D-12). Compile-time assertion lives in
// internal/daemon/wiring_test.go (Plan 53-03).
type MetricsSink interface {
	// EditOutcomeInc records the outcome of an edit-tool invocation.
	// tool must appear in AllowedTools; outcome must be one of the Outcome*
	// constants. Unknown values are dropped at the helper.
	EditOutcomeInc(tool, outcome string)
}

// NoopSink is the zero-allocation default implementation; used when metrics
// are disabled or in tests that do not assert emission.
type NoopSink struct{}

// EditOutcomeInc implements MetricsSink.
func (NoopSink) EditOutcomeInc(string, string) {}

// Compile-time assertion that NoopSink satisfies MetricsSink.
var _ MetricsSink = NoopSink{}

// Outcome label values per Phase 53 D-07. Closed enum: success | fuzzy_applied |
// refused_ambiguous | failed.
const (
	OutcomeSuccess          = "success"
	OutcomeFuzzyApplied     = "fuzzy_applied"
	OutcomeRefusedAmbiguous = "refused_ambiguous"
	OutcomeFailed           = "failed"
)

// AllowedTools is the closed allowlist for the {tool} label on
// serena_edit_outcome_total. This MUST stay in lockstep with the inline
// allowlist in obs.Metrics.EditOutcomeInc — drift is silent (helper-side
// guard drops unknown tools). Plan 53-01 SUMMARY notes this coupling.
//
// Read-only diagnostics tools are intentionally excluded.
var AllowedTools = map[string]bool{
	"replace_symbol_body":  true,
	"insert_before_symbol": true,
	"insert_after_symbol":  true,
	"rename_symbol":        true,
	"safe_delete_symbol":   true,
	"replace_in_file":      true,
	"fuzzy_edit":           true,
	"create_file":          true,
}

// ClassifyOutcome maps a fuzzy.Strategy + error to a closed outcome string.
// Pass strategy="" for tools that do not carry a fuzzy result
// (rename_symbol, insert_*, safe_delete_symbol, create_file).
//
// An ambiguity error (errors.Is(err, fuzzy.ErrAmbiguous)) maps to
// OutcomeRefusedAmbiguous regardless of strategy. Any other non-nil error
// maps to OutcomeFailed. nil error with StrategyExact or "" maps to
// OutcomeSuccess; nil error with whitespace/indentation strategies maps to
// OutcomeFuzzyApplied; nil error with StrategyFailed (defensive) maps to
// OutcomeFailed.
func ClassifyOutcome(strategy fuzzy.Strategy, err error) string {
	if err != nil {
		if errors.Is(err, fuzzy.ErrAmbiguous) {
			return OutcomeRefusedAmbiguous
		}
		return OutcomeFailed
	}
	switch strategy {
	case fuzzy.StrategyExact, "":
		return OutcomeSuccess
	case fuzzy.StrategyWhitespace, fuzzy.StrategyIndentationFlex:
		return OutcomeFuzzyApplied
	case fuzzy.StrategyFailed:
		return OutcomeFailed
	}
	return OutcomeFailed
}
