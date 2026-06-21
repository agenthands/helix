package multiswebench

import (
	"testing"
)

func ptrInt(i int) *int { return &i }

// TestIngest proves the load-bearing wiring: a parsed per-instance report → a
// bench/runtime.ResultInput with task_success ← the report's `resolved` bool
// (Pitfall 5, authoritative — NEVER a count of test rows), the harness
// container_id + exit_code carried verbatim (exit_code 0 PRESERVED, Pitfall 2),
// and Language STAMPED FROM THE CELL ARGUMENT (Pitfall 1: Multi-SWE has NO
// per-instance language JSON field). A missing instance is an ingest error with a
// zero-value ResultInput — never fabricated success.
func TestIngest(t *testing.T) {
	const iid = "golang__go-1001"

	mkReport := func(resolved bool) InstanceReport {
		return InstanceReport{
			iid: InstanceEval{
				Org:      "golang",
				Repo:     "go",
				Number:   1001,
				Resolved: resolved,
			},
		}
	}

	t.Run("resolved true -> task_success true, language stamped from cell arg", func(t *testing.T) {
		in, err := Ingest(mkReport(true), iid, "go", "ctr-abc123", ptrInt(0))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.Benchmark != "multi-swe-bench" {
			t.Errorf("benchmark = %q, want multi-swe-bench", in.Benchmark)
		}
		if in.TaskID != iid {
			t.Errorf("task_id = %q, want %q", in.TaskID, iid)
		}
		if in.Metrics.TaskSuccess == nil || *in.Metrics.TaskSuccess != true {
			t.Errorf("task_success = %v, want true (from resolved)", in.Metrics.TaskSuccess)
		}
		if in.Metrics.VerifiedCorrectness != nil {
			t.Errorf("verified_correctness = %v, want nil", in.Metrics.VerifiedCorrectness)
		}
		if in.Language != "go" {
			t.Errorf("language = %q, want go (stamped from the cell arg, NOT a JSON field)", in.Language)
		}
		if in.ContainerID != "ctr-abc123" {
			t.Errorf("container_id = %q, want ctr-abc123", in.ContainerID)
		}
		if in.ExitCode == nil || *in.ExitCode != 0 {
			t.Errorf("exit_code = %v, want ptr(0) PRESERVED (Pitfall 2)", in.ExitCode)
		}
	})

	t.Run("resolved false -> task_success false", func(t *testing.T) {
		in, err := Ingest(mkReport(false), iid, "go", "ctr-xyz", ptrInt(1))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.Metrics.TaskSuccess == nil || *in.Metrics.TaskSuccess != false {
			t.Errorf("task_success = %v, want false (from resolved)", in.Metrics.TaskSuccess)
		}
		if in.ExitCode == nil || *in.ExitCode != 1 {
			t.Errorf("exit_code = %v, want ptr(1)", in.ExitCode)
		}
	})

	t.Run("nil exit_code carried through as nil", func(t *testing.T) {
		in, err := Ingest(mkReport(true), iid, "java", "", nil)
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.ExitCode != nil {
			t.Errorf("exit_code = %v, want nil (no exit captured)", in.ExitCode)
		}
		if in.ContainerID != "" {
			t.Errorf("container_id = %q, want empty", in.ContainerID)
		}
		if in.Language != "java" {
			t.Errorf("language = %q, want java", in.Language)
		}
	})

	t.Run("missing instance -> error, never fabricated success", func(t *testing.T) {
		in, err := Ingest(mkReport(true), "golang__go-99999", "go", "ctr", ptrInt(0))
		if err == nil {
			t.Fatalf("Ingest with missing instance: want error, got nil (in=%+v)", in)
		}
		if in.Metrics.TaskSuccess != nil {
			t.Errorf("on error task_success must be nil, got %v (no fabricated success)", in.Metrics.TaskSuccess)
		}
		if in.Language != "" {
			t.Errorf("on error the ResultInput must be the zero value, got language %q", in.Language)
		}
	})

	t.Run("task_success and verified are distinct pointers", func(t *testing.T) {
		in, err := Ingest(mkReport(true), iid, "go", "ctr", ptrInt(0))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.Metrics.TaskSuccess != nil && in.Metrics.VerifiedCorrectness != nil &&
			in.Metrics.TaskSuccess == in.Metrics.VerifiedCorrectness {
			t.Error("task_success and verified_correctness must be distinct pointers")
		}
	})
}
