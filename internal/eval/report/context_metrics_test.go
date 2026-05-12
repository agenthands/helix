package report_test

import (
	"testing"

	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/score"
	"github.com/agenthands/helix/internal/eval/trace"
)

func TestComputeContextMetrics_NoExpected(t *testing.T) {
	merged := trace.MergedTrace{}
	prec, rec := report.ComputeContextMetrics(merged, score.Rules{}, nil)
	if prec != nil || rec != nil {
		t.Errorf("expected (nil, nil), got (%v, %v)", prec, rec)
	}
}

func TestComputeContextMetrics_ExpectedNoTap(t *testing.T) {
	rules := score.Rules{
		ExpectSet: []score.SetRule{{Tools: []string{"find_references", "goto_definition"}}},
	}
	expected := rules.ExpectedToolNames()
	merged := trace.MergedTrace{} // no events, no summary
	prec, rec := report.ComputeContextMetrics(merged, rules, expected)
	if prec != nil {
		t.Errorf("precision should be nil when no tool calls fired, got %v", *prec)
	}
	if rec == nil || *rec != 0.0 {
		t.Errorf("recall should be 0.0 when no expected tools invoked, got %v", rec)
	}
}

func TestComputeContextMetrics_FullOverlap(t *testing.T) {
	rules := score.Rules{
		ExpectSet: []score.SetRule{{Tools: []string{"find_references", "goto_definition"}}},
	}
	expected := rules.ExpectedToolNames()
	merged := trace.MergedTrace{
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "find_references"},
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "goto_definition"},
		},
		ToolCallSummary: trace.ToolCallSummary{
			Total:  2,
			ByTool: map[string]int{"find_references": 1, "goto_definition": 1},
		},
	}
	prec, rec := report.ComputeContextMetrics(merged, rules, expected)
	if prec == nil || *prec != 1.0 {
		t.Errorf("precision = %v, want 1.0", prec)
	}
	if rec == nil || *rec != 1.0 {
		t.Errorf("recall = %v, want 1.0", rec)
	}
}

func TestComputeContextMetrics_Partial(t *testing.T) {
	rules := score.Rules{
		ExpectSet: []score.SetRule{{Tools: []string{"find_references", "goto_definition"}}},
	}
	expected := rules.ExpectedToolNames()
	merged := trace.MergedTrace{
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "find_references"}, // relevant
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "read_file"},       // not relevant
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "read_file"},       // not relevant
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "read_file"},       // not relevant
		},
		ToolCallSummary: trace.ToolCallSummary{
			Total:  4,
			ByTool: map[string]int{"find_references": 1, "read_file": 3},
		},
	}
	prec, rec := report.ComputeContextMetrics(merged, rules, expected)
	if prec == nil || *prec != 0.25 {
		t.Errorf("precision = %v, want 0.25", prec)
	}
	if rec == nil || *rec != 0.5 {
		t.Errorf("recall = %v, want 0.5", rec)
	}
}
