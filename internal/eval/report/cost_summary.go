package report

import (
	"encoding/json"
	"fmt"
	"os"
)

// CostSummary is the EVAL-04 token-only cost aggregation (D-07: no $ conversion).
// SchemaVersion is always "1".
type CostSummary struct {
	SchemaVersion string                `json:"schema_version"`
	ByMode        map[string]ModeCostAgg `json:"by_mode"`
	Overall       struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"overall"`
}

// ModeCostAgg aggregates token counts for one mode.
type ModeCostAgg struct {
	TaskCount        int     `json:"task_count"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	MeanInputTokens  float64 `json:"mean_input_tokens"`
	MeanOutputTokens float64 `json:"mean_output_tokens"`
}

// WriteCostSummary aggregates token counts from results and writes cost_summary.json
// with mode 0600. No dollar conversion is included (D-07 explicit prohibition).
func WriteCostSummary(path string, results []EvalResult) error {
	cs := CostSummary{
		SchemaVersion: "1",
		ByMode:        make(map[string]ModeCostAgg),
	}

	for _, r := range results {
		agg := cs.ByMode[r.Mode]
		agg.TaskCount++
		agg.InputTokens += r.Tokens.Input
		agg.OutputTokens += r.Tokens.Output
		cs.ByMode[r.Mode] = agg

		cs.Overall.InputTokens += r.Tokens.Input
		cs.Overall.OutputTokens += r.Tokens.Output
	}

	// Compute means.
	for mode, agg := range cs.ByMode {
		if agg.TaskCount > 0 {
			agg.MeanInputTokens = float64(agg.InputTokens) / float64(agg.TaskCount)
			agg.MeanOutputTokens = float64(agg.OutputTokens) / float64(agg.TaskCount)
		}
		cs.ByMode[mode] = agg
	}

	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		return fmt.Errorf("report.WriteCostSummary marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("report.WriteCostSummary write %q: %w", path, err)
	}
	return nil
}
