package runtime

import (
	"encoding/json"
	"testing"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// metricKeys is the canonical set of 19 metric property names (17 METRIC-01/02
// metrics + the 2 FAIR-03 cached-token columns) the metrics object must always
// carry, every one nullable (D-06/D-07).
var metricKeys = []string{
	"task_success", "verified_correctness",
	"tokens_input", "tokens_output", "tokens_input_cached_read", "tokens_input_cache_write",
	"tool_calls", "wall_time_seconds", "files_read", "bytes_read",
	"files_modified", "edit_locality", "regression_rate",
	"lsp_diagnostics_used", "semantic_tool_calls", "edit_distance_patch",
	"retry_count", "compile_errors_before", "compile_errors_after",
}

func iPtr(i int) *int  { return &i }
func bPtr(b bool) *bool { return &b }

// TestResultMetricsRoundTrip (METRIC-01): BuildResult with a populated Metrics +
// MetricErrors emits a metrics object carrying ALL keys (nil metrics serialize
// to JSON null, never omitted), validates against the committed schema, and a
// D-07 row with some nil metrics + a metric_errors entry still validates.
func TestResultMetricsRoundTrip(t *testing.T) {
	base := func() ResultInput {
		return ResultInput{
			TaskID:    "internal-toolbench/IT-go-patch-apply-1",
			Mode:      "your_agent_full",
			Benchmark: "internal-toolbench",
			RunIndex:  0,
			Outcome:   "success",
			TraceRef:  "bench/reports/x/trace.json",
			Fairness:  runners.DefaultContract,
		}
	}

	// (1) Metric-complete: every metric populated.
	t.Run("complete", func(t *testing.T) {
		in := base()
		in.Metrics = evaluators.Metrics{
			TaskSuccess:           bPtr(true),
			VerifiedCorrectness:   bPtr(true),
			TokensInput:           iPtr(1000),
			TokensOutput:          iPtr(200),
			TokensInputCachedRead: iPtr(50),
			TokensInputCacheWrite: iPtr(10),
			ToolCalls:             iPtr(3),
			WallTimeSeconds:       fPtr(2.5),
			FilesRead:             iPtr(2),
			BytesRead:             iPtr(4096),
			FilesModified:         iPtr(1),
			EditLocality:          fPtr(0.5),
			RegressionRate:        fPtr(0),
			LSPDiagnosticsUsed:    iPtr(0),
			SemanticToolCalls:     iPtr(2),
			EditDistancePatch:     iPtr(7),
			RetryCount:            iPtr(0),
			CompileErrorsBefore:   iPtr(0),
			CompileErrorsAfter:    iPtr(0),
		}

		doc, err := BuildResult(in)
		require.NoError(t, err)
		require.NoError(t, Validate(doc), "metric-complete doc must validate")

		var m map[string]any
		require.NoError(t, json.Unmarshal(doc, &m))
		metrics, ok := m["metrics"].(map[string]any)
		require.True(t, ok, "metrics must be a JSON object")
		for _, k := range metricKeys {
			_, present := metrics[k]
			assert.True(t, present, "metrics.%s must be present (never omitted)", k)
		}
	})

	// (2) D-07 row: some nil metrics + a metric_errors entry still validates, and
	// a nil metric serializes to JSON null (not omitted).
	t.Run("d07-nulls", func(t *testing.T) {
		in := base()
		in.Metrics = evaluators.Metrics{
			TaskSuccess: bPtr(true),
			// tokens_input intentionally nil (scripted run) → must be JSON null.
		}
		in.MetricErrors = []evaluators.MetricError{
			{Metric: "tokens_input", Grader: "token_meter", Reason: "no provider usage block (scripted run)"},
		}

		doc, err := BuildResult(in)
		require.NoError(t, err)
		require.NoError(t, Validate(doc), "D-07 row with nil metrics + metric_errors must validate")

		// A nil metric must serialize as an explicit JSON null token, not omitted.
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(doc, &raw))
		metricsRaw, ok := raw["metrics"]
		require.True(t, ok, "metrics object must be present")
		var metrics map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(metricsRaw, &metrics))
		ti, present := metrics["tokens_input"]
		require.True(t, present, "tokens_input key must be present even when nil")
		assert.Equal(t, "null", string(ti), "a nil metric must serialize to JSON null, not be omitted")

		// metric_errors carries the annotation.
		var m map[string]any
		require.NoError(t, json.Unmarshal(doc, &m))
		me, ok := m["metric_errors"].([]any)
		require.True(t, ok, "metric_errors must be a JSON array")
		require.Len(t, me, 1, "exactly one metric_errors entry")
	})
}

func fPtr(f float64) *float64 { return &f }

// TestResultV2Valid covers D-04: the result.v2 builder emits a schema-valid,
// provenance-complete / metric-sparse doc with fairness sourced from
// DefaultContract, and validate-on-write rejects a malformed doc.
func TestResultV2Valid(t *testing.T) {
	in := ResultInput{
		SchemaVersion: "", // builder must default to "v2"
		TaskID:        "internal-toolbench/IT-go-patch-apply-1",
		Mode:          "your_agent_full",
		Benchmark:     "internal-toolbench",
		RunIndex:      0,
		Outcome:       "success",
		TraceRef:      "bench/reports/20260617T120000Z/IT-go-patch-apply-1/your_agent_full/trace.json",
		Fairness:      runners.DefaultContract,
		// Scripted path: zero tokens, not fabricated (Pitfall 6).
		TokensInput:  0,
		TokensOutput: 0,
	}

	doc, err := BuildResult(in)
	require.NoError(t, err, "BuildResult must succeed")

	// (1) The built doc validates against bench/schema/result.v2.schema.json.
	require.NoError(t, Validate(doc), "built result doc must validate against the schema")

	// Decode for field-level assertions.
	var m map[string]any
	require.NoError(t, json.Unmarshal(doc, &m), "built doc must be valid JSON")

	// (2) Provenance-complete: schema_version=="v2", task_id, mode, benchmark,
	// and an outcome key are present.
	assert.Equal(t, "v2", m["schema_version"], "schema_version must be v2")
	assert.Equal(t, "internal-toolbench/IT-go-patch-apply-1", m["task_id"])
	assert.Equal(t, "your_agent_full", m["mode"])
	assert.Equal(t, "internal-toolbench", m["benchmark"])
	assert.Equal(t, "success", m["outcome"], "outcome must be carried as an open prop")
	assert.Equal(t, in.TraceRef, m["trace_ref"], "trace_ref must be carried as an open prop")
	assert.Equal(t, runners.DefaultContract.ModelID, m["model_id"], "model_id sourced from DefaultContract")

	// (3) Metric-sparse: rich metrics (Phase 79) MUST be absent.
	_, hasEditLocality := m["edit_locality"]
	_, hasRegression := m["regression_rate"]
	_, hasPassK := m["pass@k"]
	_, hasPassAtK := m["pass_at_k"]
	assert.False(t, hasEditLocality, "edit_locality must be absent (Phase 79)")
	assert.False(t, hasRegression, "regression_rate must be absent (Phase 79)")
	assert.False(t, hasPassK, "pass@k must be absent (Phase 79)")
	assert.False(t, hasPassAtK, "pass_at_k must be absent (Phase 79)")

	// (4) tokens are 0 for the scripted path.
	assert.EqualValues(t, 0, m["tokens_input"], "scripted tokens_input must be 0")
	assert.EqualValues(t, 0, m["tokens_output"], "scripted tokens_output must be 0")

	// (5) fairness.overrides is an empty array (not null) when DefaultContract
	// has no overrides.
	fairness, ok := m["fairness"].(map[string]any)
	require.True(t, ok, "fairness block must be a JSON object")
	overrides, ok := fairness["overrides"]
	require.True(t, ok, "fairness.overrides key must be present")
	require.NotNil(t, overrides, "fairness.overrides must not be null when empty")
	arr, ok := overrides.([]any)
	require.True(t, ok, "fairness.overrides must be a JSON array")
	assert.Empty(t, arr, "fairness.overrides must be empty when DefaultContract has no overrides")
}

// TestResultV2ValidRejectsMalformed proves validate-on-write actually rejects a
// doc the schema forbids (schema_version of the wrong type), not rubber-stamps.
func TestResultV2ValidRejectsMalformed(t *testing.T) {
	// schema_version is a const "v2"; a non-"v2"/non-string value must fail.
	bad := []byte(`{"schema_version": 123}`)
	require.Error(t, Validate(bad), "a doc with a non-string schema_version must fail Validate")

	bad2 := []byte(`{"schema_version": "v1"}`)
	require.Error(t, Validate(bad2), "a doc with schema_version != v2 must fail Validate")

	// A doc missing schema_version entirely must also fail (it is required).
	bad3 := []byte(`{"task_id": "x"}`)
	require.Error(t, Validate(bad3), "a doc missing schema_version must fail Validate")
}

// TestResultV2ValidWithOverrides asserts fairness.overrides carries waiver
// entries when the contract has overrides (forward-compat for Phase 80).
func TestResultV2ValidWithOverrides(t *testing.T) {
	mt := 16384
	fc := runners.DefaultContract
	fc.Overrides = map[string]runners.ModeOverride{
		"your_agent_full": {
			MaxTokens:    &mt,
			WaiverReason: "needs a bigger ceiling for multi-file diffs",
			ApprovedBy:   "maintainer:test",
		},
	}
	in := ResultInput{
		TaskID:    "internal-toolbench/IT-go-patch-apply-1",
		Mode:      "your_agent_full",
		Benchmark: "internal-toolbench",
		Outcome:   "success",
		TraceRef:  "trace.json",
		Fairness:  fc,
	}
	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc), "doc with an override must still validate")

	var m map[string]any
	require.NoError(t, json.Unmarshal(doc, &m))
	fairness := m["fairness"].(map[string]any)
	arr := fairness["overrides"].([]any)
	require.Len(t, arr, 1, "one override expected")
	ov := arr[0].(map[string]any)
	assert.Equal(t, "your_agent_full", ov["mode"])
	assert.Equal(t, "needs a bigger ceiling for multi-file diffs", ov["waiver_reason"])
	assert.Equal(t, "maintainer:test", ov["approved_by"])
}
