// Package tool_trace_analyzer is the Phase 79 grader that derives the
// trace-shaped metrics (tool_calls, wall_time_seconds, files_read, bytes_read,
// lsp_diagnostics_used, semantic_tool_calls, retry_count) from an
// already-merged trace.MergedTrace (METRIC-01/02/06).
//
// REUSE, do NOT re-merge (METRIC-06 anti-pattern): RunCell already runs the
// merger and exposes the resulting MergedTrace. This analyzer consumes that
// object as a pure transform; it never invokes the merger again. A second
// merge would re-parse the daemon log and risk divergent PID-gate behavior,
// producing a tool_calls value that disagrees with the durable trace.json.
// Accordingly tool_calls is pinned to mt.ToolCallSummary.Total exactly.
package tool_trace_analyzer

import (
	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/internal/eval/trace"
)

// graderName is stamped into every MetricError this package emits (D-07).
const graderName = "tool_trace_analyzer"

// semanticToolNames is the registry-anchored set of Helix semantic (symbol
// retrieval) tool names used to compute semantic_tool_calls (METRIC-02).
//
// Source of truth: the 9 symbol retrieval tools registered in
// internal/kernel/symbols/ (CLAUDE.md "Layer 1 > internal/kernel/symbols/ — 9
// symbol retrieval tools"). These are the tool-name strings that land in
// trace.MergedTrace.ToolCallSummary.ByTool / Event.Tool — NOT the
// `mcp__smtc__*` client-facing aliases. Keep in sync with the symbols package
// registrations (Assumption A4: anchor on the registry inventory, do not
// hard-code a stale list).
var semanticToolNames = map[string]struct{}{
	"go_to_definition":     {},
	"find_references":      {},
	"find_implementations": {},
	"get_call_hierarchy":   {},
	"get_type_hierarchy":   {},
	"get_hover_info":       {},
	"get_symbol_overview":  {},
	"search_symbols":       {},
	"analyze_blast_radius": {},
}

// diagnosticToolNames is the registry-anchored set of Helix diagnostic tool
// names used to compute lsp_diagnostics_used. Source of truth:
// internal/kernel/diag/ (the `get_diagnostics` tool; CLAUDE.md "Layer 1 >
// internal/kernel/diag/ — 3 diagnostic tools"). Anchored on the registry, not
// hard-coded (Assumption A4).
var diagnosticToolNames = map[string]struct{}{
	"get_diagnostics": {},
}

// readToolNames is the set of file-read / directory-listing tool names whose
// events contribute to files_read / bytes_read. Source of truth:
// internal/kernel/fileops/ (CLAUDE.md "Layer 1 > internal/kernel/fileops/").
var readToolNames = map[string]struct{}{
	"read_file": {},
	"list_dir":  {},
}

// TraceMetrics is the typed bundle of trace-derived metrics this grader
// produces. Every field is a nullable pointer mirroring evaluators.Metrics so a
// per-metric failure nulls one field without dropping the row (D-06/D-07).
type TraceMetrics struct {
	ToolCalls          *int
	WallTimeSeconds    *float64
	FilesRead          *int
	BytesRead          *int
	LSPDiagnosticsUsed *int
	SemanticToolCalls  *int
	RetryCount         *int
}

// Analyze derives the trace-shaped metrics from the already-merged trace. It is
// a pure transform over mt — it never invokes the merger (METRIC-06). Each
// metric that can be computed is returned as a non-nil pointer; a derivation
// that cannot be computed is left nil and annotated with a MetricError (D-07).
func Analyze(mt trace.MergedTrace) (TraceMetrics, []evaluators.MetricError) {
	var tm TraceMetrics
	var errs []evaluators.MetricError

	// tool_calls: pinned to the merged tally — never recomputed via a re-merge
	// (METRIC-06). This guarantees parity with the durable trace.json.
	toolCalls := mt.ToolCallSummary.Total
	tm.ToolCalls = &toolCalls

	// wall_time_seconds: DurationMs / 1000 (DurationMs is millisecond-truncated in
	// merge.go). When the merged span has no captured timing — both StartedAt and
	// EndedAt are the zero time — there is no real duration to report; emit an
	// explicit null + a MetricError rather than a fabricated pointer-to-0, so
	// "no timing captured" is distinct from a genuine sub-second run (WR-03,
	// mirroring the token present-vs-absent discipline). A real run with a captured
	// span shorter than 1ms still reports 0.0 (present-and-zero), which is correct.
	if mt.StartedAt.IsZero() && mt.EndedAt.IsZero() {
		errs = append(errs, evaluators.MetricError{
			Metric: "wall_time_seconds",
			Grader: graderName,
			Reason: "no timing captured (StartedAt/EndedAt unset)",
		})
	} else {
		wall := float64(mt.DurationMs) / 1000.0
		tm.WallTimeSeconds = &wall
	}

	// semantic_tool_calls: sum the ByTool counts over the registry-anchored
	// semantic-tool-name set (METRIC-02).
	semantic := 0
	for tool, n := range mt.ToolCallSummary.ByTool {
		if _, ok := semanticToolNames[tool]; ok {
			semantic += n
		}
	}
	tm.SemanticToolCalls = &semantic

	// files_read / bytes_read / lsp_diagnostics_used: walk the merged events.
	filesRead := 0
	bytesRead := 0
	diagUsed := 0
	retryCount := 0
	for _, ev := range mt.Events {
		if _, ok := readToolNames[ev.Tool]; ok {
			filesRead++
			bytesRead += ev.ResultSizeBytes
		}
		if _, ok := diagnosticToolNames[ev.Tool]; ok {
			diagUsed++
		}
		if ev.Kind == trace.KindAPIRetry {
			retryCount++
		}
	}
	tm.FilesRead = &filesRead
	tm.BytesRead = &bytesRead
	tm.LSPDiagnosticsUsed = &diagUsed
	tm.RetryCount = &retryCount

	return tm, errs
}
