package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/trace"
)

// TestEvalResultJSONShape verifies that EvalResult marshals to exactly the
// EVAL-01 field names and types. context_precision and context_recall must
// serialize as null when the pointer is nil.
func TestEvalResultJSONShape(t *testing.T) {
	r := report.EvalResult{
		TaskID:           "task-001",
		Mode:             "baseline",
		Success:          true,
		PatchApplies:     true,
		TestsPass:        true,
		DiagnosticsClean: true,
		DurationMs:       1234,
		Tokens: struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		}{Input: 100, Output: 200},
		EditCount: 3,
		GuardrailCompliance: trace.GuardrailCounts{
			Warned:         1,
			Blocked:        0,
			ReceiptsIssued: 2,
		},
		ContextPrecision: nil,
		ContextRecall:    nil,
		Outcome:          "success",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// Verify required EVAL-01 fields exist.
	required := []string{
		"task_id", "mode", "success", "patch_applies", "tests_pass",
		"diagnostics_clean", "duration_ms", "tokens", "edit_count",
		"guardrail_compliance", "context_precision", "context_recall", "outcome",
	}
	for _, field := range required {
		if _, ok := m[field]; !ok {
			t.Errorf("missing required field %q in marshaled EvalResult", field)
		}
	}

	// context_precision and context_recall must be JSON null when pointer is nil.
	if v, ok := m["context_precision"]; !ok || v != nil {
		t.Errorf("context_precision should be null when pointer is nil, got: %v", v)
	}
	if v, ok := m["context_recall"]; !ok || v != nil {
		t.Errorf("context_recall should be null when pointer is nil, got: %v", v)
	}

	// Verify task_id value round-trips.
	if m["task_id"] != "task-001" {
		t.Errorf("task_id: got %v, want task-001", m["task_id"])
	}
}

// TestWriteResultJSON verifies that WriteResult creates a 0600 file under the
// given path containing valid JSON for the EvalResult.
func TestWriteResultJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")

	r := report.EvalResult{
		TaskID:  "task-002",
		Mode:    "native",
		Success: false,
		Outcome: "failed",
	}

	if err := report.WriteResult(path, r); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	// Verify the file exists and has mode 0600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat result.json: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("result.json mode: got %v, want 0600", info.Mode().Perm())
	}

	// Verify it's valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result.json: %v", err)
	}
	var got report.EvalResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal result.json: %v", err)
	}
	if got.TaskID != "task-002" {
		t.Errorf("TaskID: got %q, want task-002", got.TaskID)
	}
	if got.Mode != "native" {
		t.Errorf("Mode: got %q, want native", got.Mode)
	}
	if got.Success {
		t.Errorf("Success should be false")
	}
}
