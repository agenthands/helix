package terminalbench

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseTBResults (Task 2, T-88-02-03 / A3): ParseTBResults over
// testdata/results.json yields accuracy/n_resolved/n_unresolved; an unknown extra
// key is tolerated. An empty body, an oversized body, and a type-mismatched
// accuracy field each error (bound BEFORE unmarshal).
func TestParseTBResults(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "results.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	agg, err := ParseTBResults(b)
	if err != nil {
		t.Fatalf("ParseTBResults on valid fixture: %v", err)
	}
	if agg.Accuracy != 0.6 {
		t.Errorf("accuracy = %v, want 0.6", agg.Accuracy)
	}
	if agg.NResolved != 3 {
		t.Errorf("n_resolved = %d, want 3", agg.NResolved)
	}
	if agg.NUnresolved != 2 {
		t.Errorf("n_unresolved = %d, want 2", agg.NUnresolved)
	}

	// Empty body is an error.
	if _, err := ParseTBResults(nil); err == nil {
		t.Error("ParseTBResults(nil) must error (a missing report is not an empty report)")
	}
	// Oversized body is an error (bound BEFORE unmarshal).
	big := make([]byte, maxReportBytes+1)
	if _, err := ParseTBResults(big); err == nil {
		t.Error("ParseTBResults over the size cap must error before unmarshal")
	}
	// Type-mismatched field is an error (accuracy as a string).
	if _, err := ParseTBResults([]byte(`{"accuracy":"high"}`)); err == nil {
		t.Error("ParseTBResults with a type-mismatched accuracy must error")
	}
}

// TestParseTBTrial (Task 2, A3): ParseTBTrial over testdata/trial_results.json
// yields the per-trial task_id + is_resolved bool; a literal null body errors; an
// empty body errors; a non-bool is_resolved errors; unknown keys tolerated.
func TestParseTBTrial(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "trial_results.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	trial, err := ParseTBTrial(b)
	if err != nil {
		t.Fatalf("ParseTBTrial on valid fixture: %v", err)
	}
	if trial.TaskID != "hello-world" {
		t.Errorf("task_id = %q, want hello-world", trial.TaskID)
	}
	if !trial.IsResolved {
		t.Errorf("is_resolved = %v, want true", trial.IsResolved)
	}

	// Empty body is an error.
	if _, err := ParseTBTrial(nil); err == nil {
		t.Error("ParseTBTrial(nil) must error")
	}
	// Null body is an error.
	if _, err := ParseTBTrial([]byte("null")); err == nil {
		t.Error("ParseTBTrial(null) must error")
	}
	// Oversized body is an error.
	big := make([]byte, maxReportBytes+1)
	if _, err := ParseTBTrial(big); err == nil {
		t.Error("ParseTBTrial over the size cap must error before unmarshal")
	}
	// Non-bool is_resolved is an error.
	if _, err := ParseTBTrial([]byte(`{"task_id":"x","is_resolved":"yes"}`)); err == nil {
		t.Error("ParseTBTrial with a non-bool is_resolved must error")
	}
}
