package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/eval/report"
)

// TestWriteCostSummary verifies that WriteCostSummary produces correct per-mode
// and overall token totals without any dollar conversion (D-07).
func TestWriteCostSummary(t *testing.T) {
	results := []report.EvalResult{
		{TaskID: "t1", Mode: "baseline", Tokens: struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		}{Input: 100, Output: 50}},
		{TaskID: "t2", Mode: "baseline", Tokens: struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		}{Input: 200, Output: 80}},
		{TaskID: "t1", Mode: "native", Tokens: struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		}{Input: 150, Output: 60}},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "cost_summary.json")

	if err := report.WriteCostSummary(path, results); err != nil {
		t.Fatalf("WriteCostSummary: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat cost_summary.json: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode: got %v, want 0600", info.Mode().Perm())
	}

	var cs report.CostSummary
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &cs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cs.Overall.InputTokens != 450 {
		t.Errorf("Overall.InputTokens: got %d, want 450", cs.Overall.InputTokens)
	}
	if cs.Overall.OutputTokens != 190 {
		t.Errorf("Overall.OutputTokens: got %d, want 190", cs.Overall.OutputTokens)
	}

	baselineAgg := cs.ByMode["baseline"]
	if baselineAgg.TaskCount != 2 {
		t.Errorf("baseline TaskCount: got %d, want 2", baselineAgg.TaskCount)
	}
	if baselineAgg.InputTokens != 300 {
		t.Errorf("baseline InputTokens: got %d, want 300", baselineAgg.InputTokens)
	}

	nativeAgg := cs.ByMode["native"]
	if nativeAgg.TaskCount != 1 {
		t.Errorf("native TaskCount: got %d, want 1", nativeAgg.TaskCount)
	}

	// No dollar fields should be present.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, badField := range []string{"cost_usd", "price", "dollars"} {
		if _, ok := raw[badField]; ok {
			t.Errorf("dollar field %q must not be present in cost_summary (D-07)", badField)
		}
	}
}
