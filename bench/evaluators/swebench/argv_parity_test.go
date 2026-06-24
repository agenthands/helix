package swebench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// TestRunArgsParityWithDspyTuneGolden pins the Python SWE-bench grader's argv
// mirror (tools/dspy-tune/grade_swebench.py build_argv, via the shared golden
// tools/dspy-tune/golden/swebench_argv.json) to THIS package's authoritative
// RunArgs. harness.go is the single source of truth; the v2.3 Phase-109 Python
// grader must agree element-for-element.
//
// Break-the-invariant: if RunArgs drifts from the golden (or vice-versa), this
// test goes RED — the parity guard the milestone requires (no silent divergence
// between the Go authority and the Python mirror). This is the swebench analog
// of bench/datasets/aider-polyglot/native_cmd_parity_test.go.
func TestRunArgsParityWithDspyTuneGolden(t *testing.T) {
	raw, err := os.ReadFile(dspyTuneSwebenchGoldenPath(t))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var golden struct {
		Input struct {
			DatasetName     string   `json:"dataset_name"`
			PredictionsPath string   `json:"predictions_path"`
			RunID           string   `json:"run_id"`
			InstanceIDs     []string `json:"instance_ids"`
			MaxWorkers      int      `json:"max_workers"`
			CacheLevel      string   `json:"cache_level"`
		} `json:"input"`
		Argv []string `json:"argv"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}

	got, err := RunArgs(HarnessRun{
		DatasetName:     golden.Input.DatasetName,
		PredictionsPath: golden.Input.PredictionsPath,
		RunID:           golden.Input.RunID,
		InstanceIDs:     golden.Input.InstanceIDs,
		MaxWorkers:      golden.Input.MaxWorkers,
		CacheLevel:      golden.Input.CacheLevel,
	})
	if err != nil {
		t.Fatalf("RunArgs on the golden input errored: %v", err)
	}
	if !reflect.DeepEqual(got, golden.Argv) {
		t.Errorf("argv drift:\n RunArgs=%v\n golden =%v", got, golden.Argv)
	}

	// Anti-vacuity: the golden must carry the load-bearing dataset-org pin and a
	// real instance id, so this test cannot pass by checking an empty shape.
	if golden.Input.DatasetName != "princeton-nlp/SWE-bench_Verified" {
		t.Errorf("golden must pin the dataset org to princeton-nlp/SWE-bench_Verified, got %q", golden.Input.DatasetName)
	}
	if len(golden.Input.InstanceIDs) == 0 || len(golden.Argv) < 12 {
		t.Fatalf("golden is degenerate (instance_ids=%v argv_len=%d) — vacuous-guard", golden.Input.InstanceIDs, len(golden.Argv))
	}
}

func dspyTuneSwebenchGoldenPath(t *testing.T) string {
	t.Helper()
	// This test file lives at bench/evaluators/swebench/; the golden is at
	// tools/dspy-tune/golden/ relative to the repo root (../../../ up from here).
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(repoRoot, "tools", "dspy-tune", "golden", "swebench_argv.json")
}
