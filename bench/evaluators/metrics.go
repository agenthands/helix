// Package evaluators owns the typed nullable metric contract every Phase 79
// grader produces against and the runtime marshals into result.v2.json.
//
// Metrics is a pure data contract: every field is a pointer so a nil marshals
// to an explicit JSON null (never omitted). This is the machine form of
// METRIC-01's acceptance — "missing metrics are explicit nulls, not omissions"
// — and D-06/D-07 (typed nullable fields + per-metric error annotation). The
// snake_case json tags here MUST match the property names of the metrics object
// in bench/schema/result.v2.schema.json verbatim; that key parity is the
// contract every Wave-2 grader and Wave-3 wiring consumes.
package evaluators

// Metrics is the typed nullable per-task metric record (D-06). It carries the
// 17 metrics from METRIC-01/02 plus the 2 FAIR-03 cached-token columns (D-03,
// TokensInputCachedRead/TokensInputCacheWrite). Every field is a pointer and
// NONE is elided when empty: a nil pointer marshals to JSON null so a metric a
// grader could not compute is recorded as an explicit null, never silently
// dropped.
type Metrics struct {
	TaskSuccess           *bool    `json:"task_success"`
	VerifiedCorrectness   *bool    `json:"verified_correctness"`
	TokensInput           *int     `json:"tokens_input"`
	TokensOutput          *int     `json:"tokens_output"`
	TokensInputCachedRead *int     `json:"tokens_input_cached_read"`
	TokensInputCacheWrite *int     `json:"tokens_input_cache_write"`
	ToolCalls             *int     `json:"tool_calls"`
	WallTimeSeconds       *float64 `json:"wall_time_seconds"`
	FilesRead             *int     `json:"files_read"`
	BytesRead             *int     `json:"bytes_read"`
	FilesModified         *int     `json:"files_modified"`
	EditLocality          *float64 `json:"edit_locality"`
	RegressionRate        *float64 `json:"regression_rate"`
	LSPDiagnosticsUsed    *int     `json:"lsp_diagnostics_used"`
	SemanticToolCalls     *int     `json:"semantic_tool_calls"`
	EditDistancePatch     *int     `json:"edit_distance_patch"`
	RetryCount            *int     `json:"retry_count"`
	CompileErrorsBefore   *int     `json:"compile_errors_before"`
	CompileErrorsAfter    *int     `json:"compile_errors_after"`
}

// MetricError is a per-metric grader-failure annotation (D-07). When a grader
// cannot produce a value it leaves the Metrics field nil and records why here,
// naming the offending metric and the grader that failed. The schema's
// metric_errors[] array accepts these objects.
type MetricError struct {
	Metric string `json:"metric"`
	Grader string `json:"grader"`
	Reason string `json:"reason"`
}
