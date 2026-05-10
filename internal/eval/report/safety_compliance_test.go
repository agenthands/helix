package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/trace"
)

// TestWriteSafetyCompliance verifies per-mode guardrail counts.
func TestWriteSafetyCompliance(t *testing.T) {
	results := []report.EvalResult{
		{Mode: "baseline", GuardrailCompliance: trace.GuardrailCounts{Warned: 1, Blocked: 0, ReceiptsIssued: 2}},
		{Mode: "baseline", GuardrailCompliance: trace.GuardrailCounts{Warned: 0, Blocked: 1, ReceiptsIssued: 1}},
		{Mode: "semantic_guarded", GuardrailCompliance: trace.GuardrailCounts{Warned: 3, Blocked: 2, ReceiptsIssued: 5}},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "safety_compliance.json")

	if err := report.WriteSafetyCompliance(path, results); err != nil {
		t.Fatalf("WriteSafetyCompliance: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode: got %v, want 0600", info.Mode().Perm())
	}

	var sc report.SafetyCompliance
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &sc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	base := sc.ByMode["baseline"]
	if base.Warned != 1 {
		t.Errorf("baseline Warned: got %d, want 1", base.Warned)
	}
	if base.Blocked != 1 {
		t.Errorf("baseline Blocked: got %d, want 1", base.Blocked)
	}
	if base.ReceiptsIssued != 3 {
		t.Errorf("baseline ReceiptsIssued: got %d, want 3", base.ReceiptsIssued)
	}

	guarded := sc.ByMode["semantic_guarded"]
	if guarded.Warned != 3 {
		t.Errorf("semantic_guarded Warned: got %d, want 3", guarded.Warned)
	}
}
