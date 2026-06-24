package aiderpolyglot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// TestNativeCmdParityWithDspyTuneGolden pins the Python task-success grader's
// per-language native-test-command mirror (tools/dspy-tune/grade_aider.py, via
// the shared golden tools/dspy-tune/golden/aider_native_cmd.json) to THIS
// package's authoritative nativeTestCommand. loader.go is the single source of
// truth; the v2.3 Phase-108 Python grader must agree case-for-case.
//
// Break-the-invariant: if loader.go's command for any language drifts from the
// golden (or vice-versa), this test goes RED — the parity guard the milestone
// requires (no silent divergence between the Go authority and the Python mirror).
func TestNativeCmdParityWithDspyTuneGolden(t *testing.T) {
	goldenPath := dspyTuneGoldenPath(t)
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}
	var golden map[string]json.RawMessage
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}

	// Every non-comment entry in the golden must equal nativeTestCommand(lang).
	checked := 0
	for lang, rawArgv := range golden {
		if lang == "_comment" {
			continue
		}
		var want []string
		if err := json.Unmarshal(rawArgv, &want); err != nil {
			t.Fatalf("golden[%q] is not a string array: %v", lang, err)
		}
		got, err := NativeTestCommand(lang)
		if err != nil {
			t.Fatalf("NativeTestCommand(%q) errored but golden lists it: %v", lang, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("native cmd drift for %q: loader.go=%v golden=%v", lang, got, want)
		}
		checked++
	}

	// Anti-vacuity: the golden must actually cover the known languages, so this
	// test cannot pass by checking nothing. Rust in particular must carry the
	// load-bearing --include-ignored.
	if checked < 6 {
		t.Fatalf("golden covered only %d languages; expected the full 6-language map (vacuous-guard)", checked)
	}
	rustWant := []string{"cargo", "test", "--", "--include-ignored"}
	var rustGolden []string
	if err := json.Unmarshal(golden["rust"], &rustGolden); err != nil {
		t.Fatalf("golden rust not an array: %v", err)
	}
	if !reflect.DeepEqual(rustGolden, rustWant) {
		t.Errorf("golden rust=%v must keep the load-bearing --include-ignored %v", rustGolden, rustWant)
	}
}

func dspyTuneGoldenPath(t *testing.T) string {
	t.Helper()
	// This test file lives at bench/datasets/aider-polyglot/; the golden is at
	// tools/dspy-tune/golden/ relative to the repo root (../../../ up from here).
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(repoRoot, "tools", "dspy-tune", "golden", "aider_native_cmd.json")
}
