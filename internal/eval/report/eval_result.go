package report

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/agenthands/helix/internal/eval/trace"
)

// EvalResult is the EVAL-01 per-(task, mode) evidence record. It captures
// every measurable dimension of a single eval run and is written as
// result.json under eval/reports/<run-id>/tasks/<task>/<mode>/.
//
// context_precision and context_recall are nullable (*float64) until
// ground-truth labels are available. They serialize as JSON null when unset.
// TODO(post-phase-67): populate from ground-truth label comparison.
type EvalResult struct {
	TaskID  string `json:"task_id"`
	Mode    string `json:"mode"`
	Success bool   `json:"success"`

	// Patch and test signal.
	PatchApplies     bool `json:"patch_applies"`
	TestsPass        bool `json:"tests_pass"`
	DiagnosticsClean bool `json:"diagnostics_clean"`

	// Performance dimensions.
	DurationMs int64 `json:"duration_ms"`
	Tokens     struct {
		Input  int `json:"input"`
		Output int `json:"output"`
	} `json:"tokens"`
	EditCount int `json:"edit_count"`
	// ToolCallsByTool is the per-tool call count from the merged trace.
	// Populated from MergedTrace.ToolCallSummary.ByTool and aggregated into
	// modeAggregate.ToolCallDistribution by buildModeAggregates (WR-04 fix).
	ToolCallsByTool map[string]int `json:"tool_calls_by_tool,omitempty"`

	// Safety signal from Phase 66 guardrail telemetry.
	GuardrailCompliance trace.GuardrailCounts `json:"guardrail_compliance"`

	// Context precision/recall placeholders (EVAL-01 NULLABLE until Phase 68+).
	ContextPrecision *float64 `json:"context_precision"`
	ContextRecall    *float64 `json:"context_recall"`

	// Outcome resolution.
	Outcome       string `json:"outcome"`
	FailureReason string `json:"failure_reason,omitempty"`
}

// WriteResult serialises r to JSON and writes it to path with mode 0600.
// The parent directory must already exist.
func WriteResult(path string, r EvalResult) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("report.WriteResult marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("report.WriteResult write %q: %w", path, err)
	}
	return nil
}
