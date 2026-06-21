package multiswebench

import (
	"encoding/json"
	"fmt"
)

// maxReportBytes caps a single harness report body the strict parsers will
// accept (T-88-01-04). The harness JSON is UNTRUSTED — on a live run a hostile or
// runaway report could be arbitrarily large and OOM the parser. 64 MiB comfortably
// covers a real final_report.json or a per-instance report (both small KB-scale
// documents in practice) while refusing a pathological body. ParseFinalReport/
// ParseInstanceReport reject any body larger than this BEFORE handing it to
// encoding/json. A live io.Reader caller should wrap the source in
// io.LimitReader(r, maxReportBytes+1) before ReadAll for the same defense at the
// stream boundary. Copied VERBATIM from bench/evaluators/swebench/report.go.
const maxReportBytes = 64 << 20

// FinalReport mirrors the Multi-SWE-bench final_report.json the harness emits,
// which follows the SWE-bench make_run_report lineage (A2, Pitfall 5): the
// resolved/unresolved id sets are the AUTHORITATIVE task_success gate — NOT a
// count of per-instance test rows. total_instances/resolved_instances/
// unresolved_instances are carried for reporting; schema_version is the harness's
// own report schema version (an int), distinct from result.v2. Unknown extra keys
// are tolerated (forward-compatible).
//
// CITED: swebench reporting.py make_run_report; multi-swe-bench README
// final_report.json.
type FinalReport struct {
	TotalInstances      int      `json:"total_instances"`
	ResolvedInstances   int      `json:"resolved_instances"`
	UnresolvedInstances int      `json:"unresolved_instances"`
	ResolvedIDs         []string `json:"resolved_ids"`
	UnresolvedIDs       []string `json:"unresolved_ids"`
	SchemaVersion       int      `json:"schema_version"`
}

// InstanceEval is the per-instance evaluation record the harness writes under the
// instance id key in the per-instance report. `resolved` is THE authoritative
// task_success gate (Pitfall 5 / A2) — NOT a count of test rows. org/repo/number/
// instance_id are carried for provenance; Multi-SWE-bench has NO per-instance
// `language` field (Pitfall 1), so language is NOT read here — it is stamped from
// the cell argument in Ingest.
type InstanceEval struct {
	Org        string `json:"org"`
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	InstanceID string `json:"instance_id"`
	Resolved   bool   `json:"resolved"`
}

// InstanceReport is the per-instance report shape: a map keyed by instance id →
// its InstanceEval. Mirrors the upstream contract; it typically holds one key per
// per-language report file.
type InstanceReport map[string]InstanceEval

// ParseFinalReport strictly unmarshals a final_report.json body into FinalReport.
// The body is treated as UNTRUSTED: it is size-capped to maxReportBytes BEFORE
// unmarshal (T-88-01-04), and a truncated/garbage/type-mismatched body returns an
// error — never a panic, never a partial silent success. An unknown extra key is
// tolerated (forward-compatible). An empty body is an error (a missing report is
// not a valid empty report).
func ParseFinalReport(b []byte) (FinalReport, error) {
	if err := checkReportSize(b); err != nil {
		return FinalReport{}, err
	}
	var fr FinalReport
	if err := json.Unmarshal(b, &fr); err != nil {
		return FinalReport{}, fmt.Errorf("bench/evaluators/multiswebench: parse final report: %w", err)
	}
	return fr, nil
}

// ParseInstanceReport strictly unmarshals a per-instance report body into an
// InstanceReport map. Same untrusted-input discipline as ParseFinalReport:
// size-cap before unmarshal (T-88-01-04), error on garbage/type-mismatch (e.g. a
// non-bool resolved), tolerate unknown keys, empty body is an error, a literal
// JSON null is an error.
func ParseInstanceReport(b []byte) (InstanceReport, error) {
	if err := checkReportSize(b); err != nil {
		return nil, err
	}
	var rep InstanceReport
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, fmt.Errorf("bench/evaluators/multiswebench: parse instance report: %w", err)
	}
	if rep == nil {
		return nil, fmt.Errorf("bench/evaluators/multiswebench: parse instance report: body is null")
	}
	return rep, nil
}

// checkReportSize rejects an empty or oversized body before it reaches
// encoding/json, bounding the parse against an untrusted/hostile report
// (T-88-01-04). Copied VERBATIM from bench/evaluators/swebench/report.go.
func checkReportSize(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("bench/evaluators/multiswebench: empty report body")
	}
	if len(b) > maxReportBytes {
		return fmt.Errorf("bench/evaluators/multiswebench: report body %d bytes exceeds %d-byte cap", len(b), maxReportBytes)
	}
	return nil
}
