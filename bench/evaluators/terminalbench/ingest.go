package terminalbench

import (
	"fmt"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runtime"
)

// benchmarkName is the PINNED benchmark identifier stamped onto every
// Terminal-Bench result row (ADAPTER-TERM-01). It is the value the aggregator
// slices the benchmark column on.
const benchmarkName = "terminal-bench"

// Ingest is the load-bearing transform from a parsed per-trial result into a
// bench/runtime.ResultInput — a PURE function over JSON tb already produced (no
// subprocess, no Docker). It maps:
//
//   - task_success ← the per-trial `is_resolved` bool (the AUTHORITATIVE gate;
//     NEVER a count of trial rows — Pitfall). Assigned to a fresh local then
//     &local so it is a DISTINCT pointer — never aliased with VerifiedCorrectness.
//   - container_id / exit_code ← carried verbatim into the additive open keys.
//     exit_code is a *int so a literal 0 ("ran clean") is PRESERVED, not dropped
//     (Pitfall 2); a nil stays nil ("no exit captured").
//   - Benchmark = "terminal-bench", TaskID = trial.TaskID.
//
// VerifiedCorrectness is DELIBERATELY left nil. An empty/zero TaskID is an INGEST
// ERROR — never a fabricated success: the returned ResultInput is the zero value
// (TaskSuccess nil) so a caller that ignores the error cannot mistake it for a
// pass. The SC#2 container-isolation invariant is owned by tb's per-task
// DockerComposeManager; this function NEVER touches Docker — the adapter imports
// no bench/container (asserted by the import-level gate in ingest_test.go).
func Ingest(trial TBTrial, containerID string, exitCode *int) (runtime.ResultInput, error) {
	if trial.TaskID == "" {
		return runtime.ResultInput{}, fmt.Errorf("bench/evaluators/terminalbench: ingest: trial has an empty task_id (could-not-ingest)")
	}

	// Distinct local → distinct pointer. Never alias with verified_correctness.
	taskSuccess := trial.IsResolved

	in := runtime.ResultInput{
		Benchmark:   benchmarkName,
		TaskID:      trial.TaskID,
		ContainerID: containerID,
		ExitCode:    exitCode,
		// Only TaskSuccess is set; VerifiedCorrectness stays nil. Every other Metrics
		// field is the nullable zero (nil) — an explicit null in the eventual
		// result.v2, never a fabricated value.
		Metrics: evaluators.Metrics{TaskSuccess: &taskSuccess},
	}
	return in, nil
}
