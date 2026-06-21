package multiswebench

import (
	"fmt"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runtime"
)

// benchmarkName is the PINNED benchmark identifier stamped onto every
// Multi-SWE-bench result row (ADAPTER-MULTI-01). It is the value the aggregator
// slices the benchmark column on.
const benchmarkName = "multi-swe-bench"

// Ingest is the load-bearing transform from a parsed per-instance report into a
// bench/runtime.ResultInput — a PURE function over JSON the harness already
// produced (no subprocess). It looks up instanceID in the report and maps:
//
//   - task_success ← the report's `resolved` bool (Pitfall 5: the AUTHORITATIVE
//     gate; NEVER a count of test rows). Assigned to a fresh local then &local so
//     it is a DISTINCT pointer — never aliased with VerifiedCorrectness.
//   - language ← the `lang` ARGUMENT, NOT a JSON field (Pitfall 1: Multi-SWE-bench
//     has no per-instance language field; language is the dataset-file directory,
//     stamped by the cell orchestrator and carried verbatim here). This is what
//     flows through the existing aggregator reduceLanguageRows to yield per-
//     language pass-rate (SC#1).
//   - container_id / exit_code ← carried verbatim into the additive open keys.
//     exit_code is a *int so a literal 0 ("ran clean") is PRESERVED, not dropped
//     (Pitfall 2); a nil stays nil ("no exit captured").
//   - Benchmark = "multi-swe-bench", TaskID = instanceID.
//
// VerifiedCorrectness is DELIBERATELY left nil. A missing instance (instanceID not
// in report) is an INGEST ERROR — never a fabricated success: the returned
// ResultInput is the zero value (TaskSuccess nil) so a caller that ignores the
// error cannot mistake it for a pass.
func Ingest(report InstanceReport, instanceID, lang, containerID string, exitCode *int) (runtime.ResultInput, error) {
	eval, ok := report[instanceID]
	if !ok {
		return runtime.ResultInput{}, fmt.Errorf("bench/evaluators/multiswebench: ingest: instance %q not present in report (could-not-ingest)", instanceID)
	}

	// Distinct local → distinct pointer. Never alias with verified_correctness.
	taskSuccess := eval.Resolved

	in := runtime.ResultInput{
		Benchmark:   benchmarkName,
		TaskID:      instanceID,
		Language:    lang, // Pitfall 1: from the cell arg, NEVER a JSON field.
		ContainerID: containerID,
		ExitCode:    exitCode,
		// Only TaskSuccess is set; every other Metrics field is the nullable zero
		// (nil) — an explicit null in the eventual result.v2, never fabricated.
		Metrics: evaluators.Metrics{TaskSuccess: &taskSuccess},
	}
	return in, nil
}
