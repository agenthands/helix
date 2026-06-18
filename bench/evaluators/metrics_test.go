package evaluators

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// allMetricKeys is the complete snake_case key set the Metrics struct must
// marshal — the 17 metrics from METRIC-01/02 plus the 2 FAIR-03 cached-token
// columns (D-03). Every key MUST appear in the marshaled output even when its
// value is nil (explicit JSON null, never an omission — METRIC-01 acceptance).
var allMetricKeys = []string{
	"task_success",
	"verified_correctness",
	"tokens_input",
	"tokens_output",
	"tokens_input_cached_read",
	"tokens_input_cache_write",
	"tool_calls",
	"wall_time_seconds",
	"files_read",
	"bytes_read",
	"files_modified",
	"edit_locality",
	"regression_rate",
	"lsp_diagnostics_used",
	"semantic_tool_calls",
	"edit_distance_patch",
	"retry_count",
	"compile_errors_before",
	"compile_errors_after",
}

// TestMetricsNullableMarshal proves the nullable contract:
//   - a zero-value Metrics{} marshals every metric key as explicit JSON null
//     (present, NOT omitted),
//   - a Metrics with a couple of fields set marshals those to their value while
//     the rest stay null,
//   - every json tag is snake_case and the marshaled object's key set is exactly
//     the 17 metrics + 2 cached-token columns.
func TestMetricsNullableMarshal(t *testing.T) {
	t.Run("zero value emits explicit nulls, never omissions", func(t *testing.T) {
		data, err := json.Marshal(Metrics{})
		require.NoError(t, err, "marshal zero-value Metrics")

		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &m), "decode marshaled Metrics")

		// Spot-check the keys the behavior spec names explicitly.
		for _, k := range []string{"task_success", "tokens_input", "edit_locality", "regression_rate", "compile_errors_after"} {
			raw, ok := m[k]
			require.Truef(t, ok, "key %q must be present in marshaled zero-value Metrics", k)
			require.Equalf(t, "null", string(raw), "nil pointer for %q must marshal to JSON null", k)
		}
	})

	t.Run("set fields marshal to value, the rest stay null", func(t *testing.T) {
		loc := 1.0
		ok := true
		data, err := json.Marshal(Metrics{EditLocality: &loc, TaskSuccess: &ok})
		require.NoError(t, err, "marshal partially-populated Metrics")

		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &m), "decode marshaled Metrics")

		require.Equal(t, "1", string(m["edit_locality"]), "set edit_locality must marshal to its value")
		require.Equal(t, "true", string(m["task_success"]), "set task_success must marshal to true")
		require.Equal(t, "null", string(m["tokens_input"]), "unset tokens_input must stay null")
		require.Equal(t, "null", string(m["regression_rate"]), "unset regression_rate must stay null")
	})

	t.Run("json keys are exactly the 17 metrics + 2 cached columns, snake_case", func(t *testing.T) {
		data, err := json.Marshal(Metrics{})
		require.NoError(t, err, "marshal zero-value Metrics")

		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &m), "decode marshaled Metrics")

		require.Len(t, m, len(allMetricKeys), "Metrics must marshal exactly %d keys", len(allMetricKeys))
		for _, k := range allMetricKeys {
			_, ok := m[k]
			require.Truef(t, ok, "expected snake_case key %q in marshaled Metrics", k)
		}
	})
}

// TestMetricErrorShape proves a MetricError marshals to the {metric, grader,
// reason} shape the schema's metric_errors[] items expect.
func TestMetricErrorShape(t *testing.T) {
	data, err := json.Marshal(MetricError{
		Metric: "tokens_input",
		Grader: "token_meter",
		Reason: "no provider usage block",
	})
	require.NoError(t, err, "marshal MetricError")

	var m map[string]string
	require.NoError(t, json.Unmarshal(data, &m), "decode marshaled MetricError")

	require.Equal(t, "tokens_input", m["metric"])
	require.Equal(t, "token_meter", m["grader"])
	require.Equal(t, "no provider usage block", m["reason"])
	require.Len(t, m, 3, "MetricError must marshal exactly metric/grader/reason")
}
