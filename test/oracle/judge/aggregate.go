//go:build llmjudge

package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agenthands/helix/test/harness"
)

// AggregateReport summarizes judge scores across all transcripts (D-09).
type AggregateReport struct {
	TotalTranscripts int                `json:"total_transcripts"`
	PassCount        int                `json:"pass_count"`
	SoftFailCount    int                `json:"soft_fail_count"`
	FailCount        int                `json:"fail_count"`
	DimensionAvgs    map[string]float64 `json:"dimension_averages"`
	WorstPerformers  []string           `json:"worst_performers"` // scenario IDs with verdict=fail
	WorstDimensions  []string           `json:"worst_dimensions"` // dimensions with average below 0.7
	SelfJudged       bool               `json:"self_judged"`
}

// Aggregate computes an aggregate report from a slice of scores.
func Aggregate(scores []*Score) *AggregateReport {
	report := &AggregateReport{
		TotalTranscripts: len(scores),
		DimensionAvgs:    make(map[string]float64),
	}

	if len(scores) == 0 {
		return report
	}

	// Accumulate sums per dimension.
	sums := map[string]float64{
		"tool_choice":           0,
		"description_use":       0,
		"output_interpretation": 0,
		"uncertainty_handling":  0,
		"polyglot_reasoning":    0,
		"adoption":              0,
	}

	for _, s := range scores {
		switch s.Verdict {
		case "pass":
			report.PassCount++
		case "soft_fail":
			report.SoftFailCount++
		case "fail":
			report.FailCount++
			report.WorstPerformers = append(report.WorstPerformers, s.ScenarioID)
		}

		sums["tool_choice"] += s.ToolChoice
		sums["description_use"] += s.DescriptionUse
		sums["output_interpretation"] += s.OutputInterpretation
		sums["uncertainty_handling"] += s.UncertaintyHandling
		sums["polyglot_reasoning"] += s.PolyglotReasoning
		sums["adoption"] += s.Adoption

		if s.SelfJudged {
			report.SelfJudged = true
		}
	}

	// Compute averages and find worst dimensions.
	n := float64(len(scores))
	for dim, sum := range sums {
		avg := sum / n
		report.DimensionAvgs[dim] = avg
	}

	// Find dimensions with average below 0.7, sorted ascending.
	for dim, avg := range report.DimensionAvgs {
		if avg < 0.7 {
			report.WorstDimensions = append(report.WorstDimensions, dim)
		}
	}
	sort.Slice(report.WorstDimensions, func(i, j int) bool {
		a, b := report.WorstDimensions[i], report.WorstDimensions[j]
		if report.DimensionAvgs[a] != report.DimensionAvgs[b] {
			return report.DimensionAvgs[a] < report.DimensionAvgs[b]
		}
		return a < b // stable, name-keyed tiebreak on tied averages
	})

	sort.Strings(report.WorstPerformers)

	return report
}

// WriteAggregate marshals the report to indented JSON and writes it to
// test/oracle/judge/testdata/aggregate.json.
func WriteAggregate(t *testing.T, report *AggregateReport) {
	t.Helper()

	dir := filepath.Join(harness.ProjectRoot(), "test", "oracle", "judge", "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating aggregate dir: %v", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshaling aggregate report: %v", err)
	}

	path := filepath.Join(dir, "aggregate.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing aggregate %s: %v", path, err)
	}
	t.Logf("aggregate report written: %s", path)

	// Also log a formatted summary.
	var sb strings.Builder
	sb.WriteString("\n=== Judge Aggregate Summary ===\n")
	fmt.Fprintf(&sb, "Total: %d | Pass: %d | Soft-fail: %d | Fail: %d\n",
		report.TotalTranscripts, report.PassCount, report.SoftFailCount, report.FailCount)
	if report.SelfJudged {
		sb.WriteString("WARNING: Self-judged run (subject == judge model)\n")
	}
	sb.WriteString("\nDimension Averages:\n")
	dims := []string{"tool_choice", "description_use", "output_interpretation", "uncertainty_handling", "polyglot_reasoning", "adoption"}
	for _, d := range dims {
		fmt.Fprintf(&sb, "  %-25s %.2f\n", d, report.DimensionAvgs[d])
	}
	if len(report.WorstDimensions) > 0 {
		fmt.Fprintf(&sb, "\nWorst Dimensions (avg < 0.7): %s\n", strings.Join(report.WorstDimensions, ", "))
	}
	if len(report.WorstPerformers) > 0 {
		fmt.Fprintf(&sb, "Worst Performers: %s\n", strings.Join(report.WorstPerformers, ", "))
	}
	t.Log(sb.String())
}
