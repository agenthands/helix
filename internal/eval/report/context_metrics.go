// Package report — F-10 context metrics computation.
package report

import (
	"github.com/agenthands/helix/internal/eval/score"
	"github.com/agenthands/helix/internal/eval/trace"
)

// ComputeContextMetrics derives (ContextPrecision, ContextRecall) for one
// (task, mode) run.
//
// Definitions (F-10 resolution per planning context):
//
//	ContextPrecision = relevant_tool_calls / total_tool_calls
//	ContextRecall    = expected_tools_invoked / expected_tools_total
//
// Where "relevant" means: classified as on-task by score.IsRelevant against
// the task's expected_tools.yaml rule set. "Expected" comes from the rules'
// ExpectSequence + ExpectSet via score.Rules.ExpectedToolNames.
//
// Returns (nil, nil) when expected is empty / nil (no ground truth available).
// Returns (precision, recall) populated in [0.0, 1.0] otherwise. When the
// agent fires zero tool calls but expected_tools is non-empty, precision is
// undefined (returned as nil) and recall is 0.0 (no expected tool invoked).
func ComputeContextMetrics(merged trace.MergedTrace, rules score.Rules, expected []string) (*float64, *float64) {
	if len(expected) == 0 {
		return nil, nil
	}

	// ContextRecall: fraction of expected tools that appeared at least once.
	invokedSet := make(map[string]bool, len(merged.ToolCallSummary.ByTool))
	for tool, count := range merged.ToolCallSummary.ByTool {
		if count > 0 {
			invokedSet[tool] = true
		}
	}
	hit := 0
	for _, exp := range expected {
		if invokedSet[exp] {
			hit++
		}
	}
	recall := float64(hit) / float64(len(expected))

	// ContextPrecision: fraction of daemon tool_call events whose tool is
	// classified on-task by score.IsRelevant.
	var total, relevant int
	for _, ev := range merged.Events {
		if ev.Source == "daemon" && ev.Kind == trace.KindToolCall {
			total++
			if score.IsRelevant(ev.Tool, rules) {
				relevant++
			}
		}
	}
	if total == 0 {
		// No tool calls fired → precision undefined; emit recall only.
		return nil, &recall
	}
	precision := float64(relevant) / float64(total)
	return &precision, &recall
}
