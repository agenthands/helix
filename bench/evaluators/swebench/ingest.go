package swebench

import (
	"fmt"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runtime"
)

// benchmarkName is the PINNED benchmark identifier stamped onto every SWE-bench
// result row (ADAPTER-SWE-01). It is the value the Plan 04 aggregator slices the
// raw-vs-rescored column on.
const benchmarkName = "swe-bench-verified"

// Ingest is the load-bearing transform from a parsed harness per-instance report
// into a bench/runtime.ResultInput — a PURE function over JSON the harness already
// produced (no subprocess, mirroring test_runner's pure-transform-over-outcome
// discipline). It looks up instanceID in the report and maps:
//
//   - task_success ← the canonical report's `resolved` bool (Pitfall 1: the
//     AUTHORITATIVE gate; NEVER len(tests_status rows)). Assigned to a fresh local
//     then &local so it is a DISTINCT pointer — never aliased with
//     VerifiedCorrectness, which Plan 03's 3-condition gate sets independently
//     (the anticipated VERIFIED-01 divergence; test_runner.go:37-40 anticipates it).
//   - container_id / exit_code ← carried verbatim into the Plan 01 additive open
//     keys. exit_code is a *int so a literal 0 ("ran clean") is PRESERVED, not
//     dropped (Pitfall 2); a nil stays nil ("no exit captured").
//   - Benchmark = "swe-bench-verified", TaskID = instanceID.
//
// VerifiedCorrectness is DELIBERATELY left nil: Plan 03's gate is the sole
// producer of verified_correctness. Ingest sets ONLY Metrics.TaskSuccess.
//
// A missing instance (instanceID not in report) is an INGEST ERROR — never a
// fabricated success: the returned ResultInput is the zero value (TaskSuccess nil)
// so a caller that ignores the error cannot mistake it for a pass.
func Ingest(report InstanceReport, instanceID, containerID string, exitCode *int) (runtime.ResultInput, error) {
	eval, ok := report[instanceID]
	if !ok {
		return runtime.ResultInput{}, fmt.Errorf("bench/evaluators/swebench: ingest: instance %q not present in report (could-not-ingest)", instanceID)
	}

	// Distinct local → distinct pointer. Never alias with verified_correctness.
	taskSuccess := eval.Resolved

	in := runtime.ResultInput{
		Benchmark:   benchmarkName,
		TaskID:      instanceID,
		ContainerID: containerID,
		ExitCode:    exitCode,
		// Only TaskSuccess is set; VerifiedCorrectness stays nil (Plan 03 owns it).
		// Every other Metrics field is the nullable zero (nil) — an explicit null in
		// the eventual result.v2, never a fabricated value.
		Metrics: evaluators.Metrics{TaskSuccess: &taskSuccess},
	}
	return in, nil
}
