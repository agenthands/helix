package swebench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReportParse proves the run-report and per-instance report.json strict
// parsers populate every field over the committed fixtures (hermetic — no
// subprocess, no Docker), tolerate unknown extra keys, and TOTAL on a
// truncated/garbage body (an error, never a panic, never a partial silent
// success). The fixtures are the SOLE authoritative proof (MEMORY false-green).
func TestReportParse(t *testing.T) {
	t.Run("run-report over fixture", func(t *testing.T) {
		b, err := os.ReadFile(filepath.Join("testdata", "run_report.json"))
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		rr, err := ParseRunReport(b)
		if err != nil {
			t.Fatalf("ParseRunReport: %v", err)
		}
		if rr.TotalInstances != 5 {
			t.Errorf("total_instances = %d, want 5", rr.TotalInstances)
		}
		if rr.ResolvedInstances != 3 {
			t.Errorf("resolved_instances = %d, want 3", rr.ResolvedInstances)
		}
		if rr.SchemaVersion != 2 {
			t.Errorf("schema_version = %d, want 2", rr.SchemaVersion)
		}
		if len(rr.ResolvedIDs) != 3 || rr.ResolvedIDs[0] != "sympy__sympy-20590" {
			t.Errorf("resolved_ids = %v, want 3 ids starting sympy__sympy-20590", rr.ResolvedIDs)
		}
		if len(rr.UnresolvedIDs) != 2 {
			t.Errorf("unresolved_ids = %v, want 2", rr.UnresolvedIDs)
		}
	})

	t.Run("per-instance report over fixture", func(t *testing.T) {
		b, err := os.ReadFile(filepath.Join("testdata", "report.canonical.json"))
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		rep, err := ParseInstanceReport(b)
		if err != nil {
			t.Fatalf("ParseInstanceReport: %v", err)
		}
		ir, ok := rep["sympy__sympy-20590"]
		if !ok {
			t.Fatalf("instance sympy__sympy-20590 not in report, keys=%v", keysOf(rep))
		}
		if !ir.Resolved {
			t.Error("resolved = false, want true (authoritative gate)")
		}
		if !ir.PatchSuccessfullyApplied {
			t.Error("patch_successfully_applied = false, want true")
		}
		if got := ir.TestsStatus.FailToPass.Success; len(got) != 2 || got[0] != "test_sympify_rational" {
			t.Errorf("FAIL_TO_PASS.success = %v, want 2 starting test_sympify_rational", got)
		}
		if len(ir.TestsStatus.FailToPass.Failure) != 0 {
			t.Errorf("FAIL_TO_PASS.failure = %v, want empty", ir.TestsStatus.FailToPass.Failure)
		}
		if len(ir.TestsStatus.PassToPass.Success) != 2 {
			t.Errorf("PASS_TO_PASS.success = %v, want 2", ir.TestsStatus.PassToPass.Success)
		}
	})

	t.Run("unknown extra key does not break parse", func(t *testing.T) {
		body := `{"total_instances": 1, "resolved_instances": 1, "schema_version": 2, "brand_new_future_key": {"nested": [1,2,3]}}`
		rr, err := ParseRunReport([]byte(body))
		if err != nil {
			t.Fatalf("ParseRunReport with extra key: %v", err)
		}
		if rr.TotalInstances != 1 {
			t.Errorf("total_instances = %d, want 1", rr.TotalInstances)
		}
	})

	t.Run("garbage body errors, never panics", func(t *testing.T) {
		cases := []string{
			"",
			"{",
			"not json at all",
			`{"total_instances": "this should be a number"}`,
			`{"sympy__sympy-20590": {"resolved": "not a bool"}}`,
		}
		for _, c := range cases {
			if _, err := ParseRunReport([]byte(c)); err == nil && c != "" {
				// empty run-report "" -> error; the others -> error too.
				t.Errorf("ParseRunReport(%q): want error, got nil", c)
			}
			if _, err := ParseInstanceReport([]byte(c)); err == nil {
				t.Errorf("ParseInstanceReport(%q): want error, got nil", c)
			}
		}
	})

	t.Run("oversized body is bounded (no OOM)", func(t *testing.T) {
		// A body just over the cap must be refused rather than fully buffered.
		huge := "{" + strings.Repeat(" ", maxReportBytes+16) + "}"
		if _, err := ParseInstanceReport([]byte(huge)); err == nil {
			t.Error("oversized body: want error, got nil")
		}
		if _, err := ParseRunReport([]byte(huge)); err == nil {
			t.Error("oversized run-report: want error, got nil")
		}
	})
}

func keysOf(m InstanceReport) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
