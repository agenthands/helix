package swebench

import (
	"encoding/json"
	"fmt"
)

// maxReportBytes caps a single harness report body the strict parsers will
// accept (T-87-02-01). The harness JSON is UNTRUSTED — on a live run a hostile
// or runaway report could be arbitrarily large and OOM the parser. 64 MiB
// comfortably covers a real {model}.{run_id}.json or a per-instance report.json
// (both are small KB-scale documents in practice) while refusing a pathological
// body. ParseRunReport/ParseInstanceReport reject any body larger than this BEFORE
// handing it to encoding/json, so the cap bounds the parse the same way
// crosscodeeval's io.LimitReader bounds an untrusted parquet download
// (bench/datasets/crosscodeeval/fetch.go maxParquetBytes). A live io.Reader caller
// should wrap the source in io.LimitReader(r, maxReportBytes+1) before ReadAll for
// the same defense at the stream boundary.
const maxReportBytes = 64 << 20

// TestList is one cell of a tests_status row: the success/failure name lists the
// harness records for a single test bucket. Both are plain []string of test ids.
type TestList struct {
	Success []string `json:"success"`
	Failure []string `json:"failure"`
}

// TestsStatus mirrors the upstream per-instance tests_status object
// (CITED: swebench/harness/grading.py get_eval_report): the four named buckets,
// each a {success, failure} pair of test-id lists. FAIL_TO_PASS are the canonical
// issue tests (the gate's condition (a)); PASS_TO_PASS are the pre-existing tests
// (any failure ⇒ a regress, condition (c)); a non-empty PASS_TO_FAIL.failure is a
// clear regress. FAIL_TO_FAIL is carried for completeness.
type TestsStatus struct {
	FailToPass TestList `json:"FAIL_TO_PASS"`
	PassToPass TestList `json:"PASS_TO_PASS"`
	FailToFail TestList `json:"FAIL_TO_FAIL"`
	PassToFail TestList `json:"PASS_TO_FAIL"`
}

// InstanceEval is the per-instance evaluation record the harness writes under the
// instance id key in report.json. `resolved` is THE authoritative task_success
// gate (Pitfall 1 / A4) — NOT a count of tests_status rows.
type InstanceEval struct {
	PatchIsNone              bool        `json:"patch_is_None"`
	PatchExists              bool        `json:"patch_exists"`
	PatchSuccessfullyApplied bool        `json:"patch_successfully_applied"`
	Resolved                 bool        `json:"resolved"`
	TestsStatus              TestsStatus `json:"tests_status"`
}

// InstanceReport is the per-instance report.json shape: a map keyed by instance
// id → its InstanceEval. The harness writes one such file per instance under
// logs/run_evaluation/<run_id>/<model>/<instance_id>/report.json (A3); it
// typically holds a single key but the map shape mirrors the upstream contract
// exactly.
type InstanceReport map[string]InstanceEval

// RunReport mirrors the final run-report {model}.{run_id}.json the harness emits
// (CITED: swebench/harness/reporting.py make_run_report). The Docker-only
// unstopped_* / unremoved_* keys parse when present (a Docker client ran) and are
// simply absent otherwise; they are not load-bearing for ingestion. schema_version
// is the harness's own report schema version (an int), distinct from result.v2.
type RunReport struct {
	TotalInstances      int      `json:"total_instances"`
	SubmittedInstances  int      `json:"submitted_instances"`
	CompletedInstances  int      `json:"completed_instances"`
	ResolvedInstances   int      `json:"resolved_instances"`
	UnresolvedInstances int      `json:"unresolved_instances"`
	EmptyPatchInstances int      `json:"empty_patch_instances"`
	ErrorInstances      int      `json:"error_instances"`
	CompletedIDs        []string `json:"completed_ids"`
	IncompleteIDs       []string `json:"incomplete_ids"`
	EmptyPatchIDs       []string `json:"empty_patch_ids"`
	SubmittedIDs        []string `json:"submitted_ids"`
	ResolvedIDs         []string `json:"resolved_ids"`
	UnresolvedIDs       []string `json:"unresolved_ids"`
	ErrorIDs            []string `json:"error_ids"`
	SchemaVersion       int      `json:"schema_version"`

	// Docker-only fields (present only when a Docker client ran). Parsed when
	// present, absent otherwise — never required.
	UnstoppedInstances int      `json:"unstopped_instances"`
	UnstoppedContainers []string `json:"unstopped_containers"`
	UnremovedImages     []string `json:"unremoved_images"`
}

// ParseRunReport strictly unmarshals a {model}.{run_id}.json body into RunReport.
// The body is treated as UNTRUSTED: it is size-capped to maxReportBytes BEFORE
// unmarshal (T-87-02-01), and a truncated/garbage/type-mismatched body returns an
// error — never a panic, never a partial silent success. An unknown extra key is
// tolerated (forward-compatible: the harness may add report fields). An empty body
// is an error (a missing report is not a valid empty report).
func ParseRunReport(b []byte) (RunReport, error) {
	if err := checkReportSize(b); err != nil {
		return RunReport{}, err
	}
	var rr RunReport
	if err := json.Unmarshal(b, &rr); err != nil {
		return RunReport{}, fmt.Errorf("bench/evaluators/swebench: parse run-report: %w", err)
	}
	return rr, nil
}

// ParseInstanceReport strictly unmarshals a per-instance report.json body into an
// InstanceReport map. Same untrusted-input discipline as ParseRunReport: size-cap
// before unmarshal (T-87-02-01), error on garbage/type-mismatch (e.g. a non-bool
// resolved), tolerate unknown keys, empty body is an error.
func ParseInstanceReport(b []byte) (InstanceReport, error) {
	if err := checkReportSize(b); err != nil {
		return nil, err
	}
	var rep InstanceReport
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, fmt.Errorf("bench/evaluators/swebench: parse instance report: %w", err)
	}
	if rep == nil {
		// A literal JSON `null` unmarshals into a nil map without error; that is
		// not a valid report (a missing/null report is not an empty report).
		return nil, fmt.Errorf("bench/evaluators/swebench: parse instance report: body is null")
	}
	return rep, nil
}

// checkReportSize rejects an empty or oversized body before it reaches
// encoding/json, bounding the parse against an untrusted/hostile report
// (T-87-02-01). It is the in-memory mirror of the io.LimitReader cap a live
// stream caller applies at the source.
func checkReportSize(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("bench/evaluators/swebench: empty report body")
	}
	if len(b) > maxReportBytes {
		return fmt.Errorf("bench/evaluators/swebench: report body %d bytes exceeds %d-byte cap", len(b), maxReportBytes)
	}
	return nil
}
