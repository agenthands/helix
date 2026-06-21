package swebench

import (
	"testing"
)

func ptrInt(i int) *int { return &i }

// TestIngest proves the load-bearing wiring: a parsed harness report → a
// bench/runtime.ResultInput with task_success ← report.resolved (Pitfall 1,
// authoritative — NEVER a tests_status row count), the harness container_id +
// exit_code carried into the Plan 01 additive keys (exit_code 0 PRESERVED), and
// VerifiedCorrectness LEFT NIL (Plan 03's 3-condition gate is the sole producer
// of verified_correctness — the anticipated independence split).
func TestIngest(t *testing.T) {
	const iid = "sympy__sympy-20590"

	mkReport := func(resolved bool) InstanceReport {
		return InstanceReport{
			iid: InstanceEval{
				PatchSuccessfullyApplied: true,
				Resolved:                 resolved,
				TestsStatus: TestsStatus{
					FailToPass: TestList{Success: []string{"test_a"}},
					PassToPass: TestList{Success: []string{"test_b"}},
				},
			},
		}
	}

	t.Run("resolved true -> task_success true, verified nil", func(t *testing.T) {
		in, err := Ingest(mkReport(true), iid, "ctr-abc123", ptrInt(0))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.Benchmark != "swe-bench-verified" {
			t.Errorf("benchmark = %q, want swe-bench-verified", in.Benchmark)
		}
		if in.TaskID != iid {
			t.Errorf("task_id = %q, want %q", in.TaskID, iid)
		}
		if in.Metrics.TaskSuccess == nil || *in.Metrics.TaskSuccess != true {
			t.Errorf("task_success = %v, want true (from resolved)", in.Metrics.TaskSuccess)
		}
		if in.Metrics.VerifiedCorrectness != nil {
			t.Errorf("verified_correctness = %v, want nil (Plan 03's gate is sole producer)", in.Metrics.VerifiedCorrectness)
		}
		if in.ContainerID != "ctr-abc123" {
			t.Errorf("container_id = %q, want ctr-abc123", in.ContainerID)
		}
		if in.ExitCode == nil || *in.ExitCode != 0 {
			t.Errorf("exit_code = %v, want ptr(0) PRESERVED (Pitfall 2)", in.ExitCode)
		}
	})

	t.Run("resolved false -> task_success false", func(t *testing.T) {
		in, err := Ingest(mkReport(false), iid, "ctr-xyz", ptrInt(1))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.Metrics.TaskSuccess == nil || *in.Metrics.TaskSuccess != false {
			t.Errorf("task_success = %v, want false (from resolved)", in.Metrics.TaskSuccess)
		}
		if in.Metrics.VerifiedCorrectness != nil {
			t.Errorf("verified_correctness = %v, want nil", in.Metrics.VerifiedCorrectness)
		}
		if in.ExitCode == nil || *in.ExitCode != 1 {
			t.Errorf("exit_code = %v, want ptr(1)", in.ExitCode)
		}
	})

	t.Run("nil exit_code carried through as nil", func(t *testing.T) {
		in, err := Ingest(mkReport(true), iid, "", nil)
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.ExitCode != nil {
			t.Errorf("exit_code = %v, want nil (no exit captured)", in.ExitCode)
		}
		if in.ContainerID != "" {
			t.Errorf("container_id = %q, want empty", in.ContainerID)
		}
	})

	t.Run("missing instance -> error, never fabricated success", func(t *testing.T) {
		in, err := Ingest(mkReport(true), "django__django-99999", "ctr", ptrInt(0))
		if err == nil {
			t.Fatalf("Ingest with missing instance: want error, got nil (in=%+v)", in)
		}
		if in.Metrics.TaskSuccess != nil {
			t.Errorf("on error task_success must be nil, got %v (no fabricated success)", in.Metrics.TaskSuccess)
		}
	})

	t.Run("task_success and verified are distinct pointers", func(t *testing.T) {
		in, err := Ingest(mkReport(true), iid, "ctr", ptrInt(0))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		// task_success is set; verified is nil. They must not alias.
		if in.Metrics.TaskSuccess != nil && in.Metrics.VerifiedCorrectness != nil &&
			in.Metrics.TaskSuccess == in.Metrics.VerifiedCorrectness {
			t.Error("task_success and verified_correctness must be distinct pointers")
		}
	})
}
