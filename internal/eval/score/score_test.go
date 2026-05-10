package score_test

import (
	"time"
	"testing"

	"github.com/agenthands/helix/internal/eval/score"
	"github.com/agenthands/helix/internal/eval/trace"
)

// makeEvent creates a daemon-side tool_call Event for testing.
func makeEvent(tool string) trace.Event {
	return trace.Event{
		T:           time.Now(),
		Source:      "daemon",
		Kind:        trace.KindToolCall,
		Tool:        tool,
		Outcome:     "success",
		ArgsSummary: "",
	}
}

// makeEventWithArgs creates a daemon-side tool_call Event with args_summary.
func makeEventWithArgs(tool, argsSummary string) trace.Event {
	return trace.Event{
		T:           time.Now(),
		Source:      "daemon",
		Kind:        trace.KindToolCall,
		Tool:        tool,
		Outcome:     "success",
		ArgsSummary: argsSummary,
	}
}

// makeTrace builds a MergedTrace from a list of events.
func makeTrace(events ...trace.Event) trace.MergedTrace {
	return trace.MergedTrace{
		SchemaVersion: "1",
		TaskID:        "test-task",
		Mode:          "semantic",
		Events:        events,
	}
}

// TestScoreExpectSequenceMatches verifies that a trace with [find_references,
// rename_symbol] matches an expect_sequence rule and earns +1.
func TestScoreExpectSequenceMatches(t *testing.T) {
	rules := score.Rules{
		ExpectSequence: []score.SequenceRule{
			{
				ID:    "rename-after-references",
				Score: 1,
				Pattern: []score.PatternStep{
					{Tool: "find_references", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
					{Tool: "rename_symbol", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
				},
			},
		},
	}
	tr := makeTrace(makeEvent("find_references"), makeEvent("rename_symbol"))
	s := score.Apply(tr, rules)

	if s.Total != 1 {
		t.Errorf("Total = %d; want 1", s.Total)
	}
	if len(s.Deltas) != 1 {
		t.Fatalf("Deltas len = %d; want 1", len(s.Deltas))
	}
	if s.Deltas[0].RuleID != "rename-after-references" {
		t.Errorf("Deltas[0].RuleID = %q; want rename-after-references", s.Deltas[0].RuleID)
	}
	if s.Deltas[0].Score != 1 {
		t.Errorf("Deltas[0].Score = %d; want 1", s.Deltas[0].Score)
	}
}

// TestScoreExpectSequenceWithIntervening verifies that intervening events do
// not break a sequence match.
func TestScoreExpectSequenceWithIntervening(t *testing.T) {
	rules := score.Rules{
		ExpectSequence: []score.SequenceRule{
			{
				ID:    "rename-after-references",
				Score: 1,
				Pattern: []score.PatternStep{
					{Tool: "find_references", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
					{Tool: "rename_symbol", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
				},
			},
		},
	}
	// Intervening event: get_definition between find_references and rename_symbol.
	tr := makeTrace(
		makeEvent("find_references"),
		makeEvent("get_definition"),
		makeEvent("rename_symbol"),
	)
	s := score.Apply(tr, rules)

	if s.Total != 1 {
		t.Errorf("Total = %d; want 1 (intervening events should be ignored)", s.Total)
	}
}

// TestScoreExpectSequenceArgsMatch verifies that args_match requires a
// substring match on args_summary.
func TestScoreExpectSequenceArgsMatch(t *testing.T) {
	rules := score.Rules{
		ExpectSequence: []score.SequenceRule{
			{
				ID:    "rename-authMiddleware",
				Score: 1,
				Pattern: []score.PatternStep{
					{
						Tool:           "find_references",
						ArgsMatch:      map[string]string{"symbol": "AuthMiddleware"},
						ArgsMatchRegex: nil,
					},
					{Tool: "rename_symbol", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
				},
			},
		},
	}

	t.Run("match_present", func(t *testing.T) {
		tr := makeTrace(
			makeEventWithArgs("find_references", `{"symbol":"AuthMiddleware"}`),
			makeEvent("rename_symbol"),
		)
		s := score.Apply(tr, rules)
		if s.Total != 1 {
			t.Errorf("Total = %d; want 1 when args_match is satisfied", s.Total)
		}
	})

	t.Run("match_absent", func(t *testing.T) {
		tr := makeTrace(
			makeEventWithArgs("find_references", `{"symbol":"OtherFunc"}`),
			makeEvent("rename_symbol"),
		)
		s := score.Apply(tr, rules)
		if s.Total != 0 {
			t.Errorf("Total = %d; want 0 when args_match substring is not found", s.Total)
		}
	})
}

// TestScoreForbidSequenceMatches verifies that a forbidden sequence earns -1.
func TestScoreForbidSequenceMatches(t *testing.T) {
	rules := score.Rules{
		ForbidSequence: []score.SequenceRule{
			{
				ID:    "rename-by-grep",
				Score: -1,
				Pattern: []score.PatternStep{
					{Tool: "search_for_pattern", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
					{Tool: "replace_in_file", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
				},
			},
		},
	}
	tr := makeTrace(makeEvent("search_for_pattern"), makeEvent("replace_in_file"))
	s := score.Apply(tr, rules)

	if s.Total != -1 {
		t.Errorf("Total = %d; want -1", s.Total)
	}
	if len(s.Deltas) != 1 {
		t.Fatalf("Deltas len = %d; want 1", len(s.Deltas))
	}
	if s.Deltas[0].Score != -1 {
		t.Errorf("Deltas[0].Score = %d; want -1", s.Deltas[0].Score)
	}
}

// TestScoreForbidSetRequirePriorMissing verifies that delete_file without
// prior find_references earns -1.
func TestScoreForbidSetRequirePriorMissing(t *testing.T) {
	rules := score.Rules{
		ForbidSet: []score.ForbidSetRule{
			{
				ID:    "delete-without-references",
				Score: -1,
				When:  score.ForbidWhen{ToolUsed: "delete_file"},
				RequirePrior: score.ForbidRequire{
					AnyOf: []string{"find_references", "analyze_blast_radius"},
				},
			},
		},
	}
	// Only delete_file — no prior find_references or analyze_blast_radius.
	tr := makeTrace(makeEvent("delete_file"))
	s := score.Apply(tr, rules)

	if s.Total != -1 {
		t.Errorf("Total = %d; want -1 when prior is missing", s.Total)
	}
}

// TestScoreForbidSetRequirePriorPresent verifies that find_references before
// delete_file does NOT trigger the negative score.
func TestScoreForbidSetRequirePriorPresent(t *testing.T) {
	rules := score.Rules{
		ForbidSet: []score.ForbidSetRule{
			{
				ID:    "delete-without-references",
				Score: -1,
				When:  score.ForbidWhen{ToolUsed: "delete_file"},
				RequirePrior: score.ForbidRequire{
					AnyOf: []string{"find_references", "analyze_blast_radius"},
				},
			},
		},
	}
	tr := makeTrace(makeEvent("find_references"), makeEvent("delete_file"))
	s := score.Apply(tr, rules)

	if s.Total != 0 {
		t.Errorf("Total = %d; want 0 when prior is present", s.Total)
	}
}

// TestScoreReceiptsRule verifies that safe_delete_symbol with
// receipts:["abc","def"] in args_summary earns +1.
func TestScoreReceiptsRule(t *testing.T) {
	rules := score.Rules{
		Receipts: []score.ReceiptRule{
			{
				ID:      "safe-delete-with-receipts",
				Score:   1,
				When:    score.ReceiptWhen{ToolUsed: "safe_delete_symbol"},
				Require: score.ReceiptRequire{ReceiptsNonEmpty: true},
			},
		},
	}

	t.Run("receipts_present", func(t *testing.T) {
		tr := makeTrace(makeEventWithArgs("safe_delete_symbol", `receipts:["abc","def"]`))
		s := score.Apply(tr, rules)
		if s.Total != 1 {
			t.Errorf("Total = %d; want 1 when receipts present", s.Total)
		}
	})

	t.Run("receipts_empty", func(t *testing.T) {
		tr := makeTrace(makeEventWithArgs("safe_delete_symbol", `receipts:[]`))
		s := score.Apply(tr, rules)
		if s.Total != 0 {
			t.Errorf("Total = %d; want 0 when receipts empty", s.Total)
		}
	})

	t.Run("tool_not_fired", func(t *testing.T) {
		tr := makeTrace(makeEvent("find_references"))
		s := score.Apply(tr, rules)
		if s.Total != 0 {
			t.Errorf("Total = %d; want 0 when tool not fired", s.Total)
		}
	})
}

// TestScoreExpectSetTools verifies that a set rule earns +1 when both tools
// fire (any order).
func TestScoreExpectSetTools(t *testing.T) {
	rules := score.Rules{
		ExpectSet: []score.SetRule{
			{
				ID:    "verified-after-edit",
				Score: 1,
				Tools: []string{"rename_symbol", "verify_edit"},
			},
		},
	}

	t.Run("both_tools_present", func(t *testing.T) {
		tr := makeTrace(makeEvent("rename_symbol"), makeEvent("verify_edit"))
		s := score.Apply(tr, rules)
		if s.Total != 1 {
			t.Errorf("Total = %d; want 1", s.Total)
		}
	})

	t.Run("only_one_tool", func(t *testing.T) {
		tr := makeTrace(makeEvent("rename_symbol"))
		s := score.Apply(tr, rules)
		if s.Total != 0 {
			t.Errorf("Total = %d; want 0 when only one tool present", s.Total)
		}
	})

	t.Run("reversed_order", func(t *testing.T) {
		tr := makeTrace(makeEvent("verify_edit"), makeEvent("rename_symbol"))
		s := score.Apply(tr, rules)
		if s.Total != 1 {
			t.Errorf("Total = %d; want 1 for reversed order", s.Total)
		}
	})
}

// TestScoreIsDeterministic verifies that Apply returns identical Score for
// identical input.
func TestScoreIsDeterministic(t *testing.T) {
	rules := score.Rules{
		ExpectSequence: []score.SequenceRule{
			{
				ID:    "rename-after-references",
				Score: 1,
				Pattern: []score.PatternStep{
					{Tool: "find_references", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
					{Tool: "rename_symbol", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
				},
			},
		},
		ForbidSequence: []score.SequenceRule{
			{
				ID:    "rename-by-grep",
				Score: -1,
				Pattern: []score.PatternStep{
					{Tool: "search_for_pattern", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
					{Tool: "replace_in_file", ArgsMatch: map[string]string{}, ArgsMatchRegex: nil},
				},
			},
		},
	}
	tr := makeTrace(makeEvent("find_references"), makeEvent("rename_symbol"))

	s1 := score.Apply(tr, rules)
	s2 := score.Apply(tr, rules)

	if s1.Total != s2.Total {
		t.Errorf("Apply not deterministic: %d != %d", s1.Total, s2.Total)
	}
	if len(s1.Deltas) != len(s2.Deltas) {
		t.Errorf("Deltas len not deterministic: %d != %d", len(s1.Deltas), len(s2.Deltas))
	}
}
