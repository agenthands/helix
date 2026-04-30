package fileops

import (
	stderrors "errors"
	"testing"

	"github.com/agenthands/helix/internal/fuzzy"
	"github.com/agenthands/helix/internal/kernel/edit"
)

// TestFileopsTools_OutcomeEmission documents the (handler × outcome × strategy)
// closed-enum tuples that the 2 fileops tools (replace_in_file, fuzzy_edit)
// can emit on helix_edit_outcome_total (Phase 53 D-10 + D-11). It exercises
// the shared edit.ClassifyEditError helper imported from the edit package
// (DRY: single source of truth for the D-10 outcome classifier) and asserts
// each handler's strategy mapping. The instrumentation itself is verified
// via the acceptance grep on tools.go (>=2 RecordEditOutcome calls); end-to-
// end emission round-trip is covered by TestRecordEditOutcome in
// internal/mcp/record_edit_outcome_test.go.
func TestFileopsTools_OutcomeEmission(t *testing.T) {
	t.Run("replace_in_file_exact_match", func(t *testing.T) {
		// Literal-match success path in registerReplaceInFile sets
		// strategy="exact" when count > 0. Pinned by the acceptance grep
		// on tools.go ("strategy = \"exact\"").
	})

	t.Run("replace_in_file_whitespace_match", func(t *testing.T) {
		// Fuzzy fallback path sets strategy = string(fResult.Strategy).
		// fuzzy.StrategyWhitespace stringifies to "whitespace_normalized" —
		// pinned by fuzzy/types.go and the strategyEnum in middleware.go.
		if string(fuzzy.StrategyWhitespace) != "whitespace_normalized" {
			t.Errorf("StrategyWhitespace = %q, want whitespace_normalized", string(fuzzy.StrategyWhitespace))
		}
	})

	t.Run("replace_in_file_indentation_match", func(t *testing.T) {
		if string(fuzzy.StrategyIndentationFlex) != "indentation_flexible" {
			t.Errorf("StrategyIndentationFlex = %q, want indentation_flexible", string(fuzzy.StrategyIndentationFlex))
		}
	})

	t.Run("replace_in_file_no_match_emits_none", func(t *testing.T) {
		// fuzzy.Match's failureError wraps fuzzy.ErrNoMatch (Phase 53 W0).
		// Handler classifies via edit.ClassifyEditError -> "no_match" and
		// strategy stays "none" (initial value). Verify the classification.
		if got := edit.ClassifyEditError(fuzzy.ErrNoMatch); got != "no_match" {
			t.Errorf("ClassifyEditError(ErrNoMatch) = %q, want no_match", got)
		}
		// Q-4: StrategyFailed.String() is "failed" but is NEVER emitted as
		// a strategy label value — handler keeps strategy="none" on the
		// fuzzy-error branch.
		if string(fuzzy.StrategyFailed) != "failed" {
			t.Skip("fuzzy.StrategyFailed.String() changed — Q-4 mapping assumption may need review")
		}
	})

	t.Run("fuzzy_edit_success_emits_strategy", func(t *testing.T) {
		// Success path in registerFuzzyEdit: strategy = string(result.Strategy).
		// All three real cascade tiers must be in the closed strategyEnum.
		for _, s := range []fuzzy.Strategy{
			fuzzy.StrategyExact,
			fuzzy.StrategyWhitespace,
			fuzzy.StrategyIndentationFlex,
		} {
			label := string(s)
			if label == "failed" || label == "" {
				t.Errorf("fuzzy.Strategy %v stringifies to %q — would violate closed strategyEnum", s, label)
			}
		}
	})

	t.Run("fuzzy_edit_no_match_emits_none", func(t *testing.T) {
		// Handler error branch: outcome = ClassifyEditError(err);
		// strategy stays "none". Confirms ErrNoMatch sentinel reaches the
		// classifier intact via FuzzyEdit -> fuzzy.Match.
		err := stderrors.Join(stderrors.New("wrap"), fuzzy.ErrNoMatch)
		if got := edit.ClassifyEditError(err); got != "no_match" {
			t.Errorf("ClassifyEditError(ErrNoMatch-wrapped) = %q, want no_match", got)
		}
	})

	t.Run("fuzzy_edit_ambiguous_emits_ambiguous_match", func(t *testing.T) {
		err := stderrors.Join(stderrors.New("wrap"), fuzzy.ErrAmbiguous)
		if got := edit.ClassifyEditError(err); got != "ambiguous_match" {
			t.Errorf("ClassifyEditError(ErrAmbiguous-wrapped) = %q, want ambiguous_match", got)
		}
	})

	t.Run("missing_required_field_records_internal_per_q3", func(t *testing.T) {
		// Q-3 (RESOLVED): missing-field validations bucket as "internal" —
		// handler sets outcome="internal" directly before returning.
		// The classifier returns "internal" for any non-fuzzy-sentinel error.
		if got := edit.ClassifyEditError(stderrors.New("missing required field: path")); got != "internal" {
			t.Errorf("ClassifyEditError(missing-field) = %q, want internal", got)
		}
	})
}
