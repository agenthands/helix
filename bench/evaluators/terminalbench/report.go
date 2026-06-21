package terminalbench

import (
	"encoding/json"
	"fmt"
)

// maxReportBytes caps a single tb results.json body the strict parsers will
// accept (T-88-02-03). The tb JSON is UNTRUSTED — on a live run a hostile or
// runaway report could be arbitrarily large and OOM the parser. 64 MiB
// comfortably covers a real runs/<ts>/results.json (aggregate) or a per-trial
// results.json (both small KB-scale documents in practice) while refusing a
// pathological body. ParseTBResults/ParseTBTrial reject any body larger than this
// BEFORE handing it to encoding/json. A live io.Reader caller should wrap the
// source in io.LimitReader(r, maxReportBytes+1) before ReadAll for the same
// defense at the stream boundary. Copied VERBATIM from
// bench/evaluators/swebench/report.go.
const maxReportBytes = 64 << 20

// TBAggregate mirrors the tb runs/<ts>/results.json aggregate the harness emits
// (A3 [ASSUMED] field names — pinned as the hermetic contract Plan 04's
// human-verify confirms). accuracy/n_resolved/n_unresolved are carried for
// reporting; they are NOT the per-task authoritative gate (that is the per-trial
// is_resolved bool — Pitfall: a count of trial rows is NOT the gate). Unknown
// extra keys are tolerated (forward-compatible).
//
// CITED: deepwiki terminal-bench results schema.
type TBAggregate struct {
	Accuracy    float64 `json:"accuracy"`
	NResolved   int     `json:"n_resolved"`
	NUnresolved int     `json:"n_unresolved"`
}

// TBTrial mirrors the tb per-trial <task_id>/<trial>/results.json the harness
// emits. is_resolved is THE authoritative task_success gate (A3 / Pitfall) — NOT
// a count of any trial rows. task_id is carried for provenance. Unknown extra keys
// are tolerated (forward-compatible).
//
// CITED: deepwiki terminal-bench results schema.
type TBTrial struct {
	TaskID     string `json:"task_id"`
	IsResolved bool   `json:"is_resolved"`
}

// ParseTBResults strictly unmarshals a runs/<ts>/results.json aggregate body into
// TBAggregate. The body is treated as UNTRUSTED: it is size-capped to
// maxReportBytes BEFORE unmarshal (T-88-02-03), and a truncated/garbage/
// type-mismatched body returns an error — never a panic, never a partial silent
// success. An unknown extra key is tolerated (forward-compatible). An empty body
// is an error (a missing report is not a valid empty report).
func ParseTBResults(b []byte) (TBAggregate, error) {
	if err := checkReportSize(b); err != nil {
		return TBAggregate{}, err
	}
	var agg TBAggregate
	if err := json.Unmarshal(b, &agg); err != nil {
		return TBAggregate{}, fmt.Errorf("bench/evaluators/terminalbench: parse results.json: %w", err)
	}
	return agg, nil
}

// ParseTBTrial strictly unmarshals a per-trial results.json body into TBTrial.
// Same untrusted-input discipline as ParseTBResults: size-cap before unmarshal
// (T-88-02-03), error on garbage/type-mismatch (e.g. a non-bool is_resolved),
// tolerate unknown keys, empty body is an error, a literal JSON null is an error
// (a missing/null trial is not a valid empty trial — and must NEVER decode to a
// zero-value is_resolved=false that a caller could mistake for an honest fail).
func ParseTBTrial(b []byte) (TBTrial, error) {
	if err := checkReportSize(b); err != nil {
		return TBTrial{}, err
	}
	// Reject a literal JSON null explicitly: json.Unmarshal of "null" into a struct
	// is a no-op that leaves the zero value WITHOUT error, which would fabricate an
	// is_resolved=false. A null trial is not a valid trial.
	if string(b) == "null" {
		return TBTrial{}, fmt.Errorf("bench/evaluators/terminalbench: parse trial results.json: body is null")
	}
	var trial TBTrial
	if err := json.Unmarshal(b, &trial); err != nil {
		return TBTrial{}, fmt.Errorf("bench/evaluators/terminalbench: parse trial results.json: %w", err)
	}
	return trial, nil
}

// checkReportSize rejects an empty or oversized body before it reaches
// encoding/json, bounding the parse against an untrusted/hostile report
// (T-88-02-03). Copied VERBATIM from bench/evaluators/swebench/report.go.
func checkReportSize(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("bench/evaluators/terminalbench: empty report body")
	}
	if len(b) > maxReportBytes {
		return fmt.Errorf("bench/evaluators/terminalbench: report body %d bytes exceeds %d-byte cap", len(b), maxReportBytes)
	}
	return nil
}
