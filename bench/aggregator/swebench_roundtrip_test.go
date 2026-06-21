package aggregator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/evaluators/swebench"
	"github.com/agenthands/helix/bench/runtime"
)

// loadInstanceEval reads a swebench testdata report fixture (relative to the
// swebench package's testdata dir) and returns the single InstanceEval under iid.
func loadInstanceEval(t *testing.T, name, iid string) swebench.InstanceEval {
	t.Helper()
	p := filepath.Join("..", "evaluators", "swebench", "testdata", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	rep, err := swebench.ParseInstanceReport(b)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	ev, ok := rep[iid]
	if !ok {
		t.Fatalf("instance %q not in %s", iid, name)
	}
	return ev
}

// ★ TestRescore_ApplyToRow_RoundTrip closes the producer↔reader wiring gap: it
// stamps a result row via swebench.Rescore(...).ApplyToRow (the SC#2-shaped
// raw=true/rescored=false divergence), BuildResults the row, marshals it, parses
// it back into an aggregator Row, and drives THIS package's rowSwebenchScores
// reader over it — asserting raw present=true (true) AND rescored present=true
// (false). The producer and reader exercise the SAME pinned bench/runtime key
// consts, so a LIVE SWE-bench divergence can never silently em-dash. No
// hand-injected doc key.
func TestRescore_ApplyToRow_RoundTrip(t *testing.T) {
	const iid = "sympy__sympy-20590"
	canonical := loadInstanceEval(t, "report.buggy_canonical.json", iid)
	augmented := loadInstanceEval(t, "report.buggy_utboost.json", iid)

	in := runtime.ResultInput{Benchmark: "swe-bench-verified", TaskID: iid, Mode: "your_agent"}
	swebench.Rescore(canonical, &augmented).ApplyToRow(&in)

	b, err := runtime.BuildResult(in)
	if err != nil {
		t.Fatalf("BuildResult: %v", err)
	}
	// The stamped doc must still pass the committed schema (additive-open-key).
	if err := runtime.Validate(b); err != nil {
		t.Fatalf("stamped row failed schema validation: %v", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal row: %v", err)
	}
	row := Row{Doc: doc}

	raw, rescored, present := rowSwebenchScores(row)
	if !present {
		t.Fatal("rowSwebenchScores present=false, want true (a stamped LIVE row must read present)")
	}
	if raw == nil || *raw != true {
		t.Errorf("raw = %v, want true (swebench_raw_resolved)", raw)
	}
	if rescored == nil || *rescored != false {
		t.Errorf("rescored = %v, want false (swebench_rescored_verified)", rescored)
	}
}
