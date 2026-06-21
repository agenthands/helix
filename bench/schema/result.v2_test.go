package schema_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

const (
	schemaPath = "result.v2.schema.json"
	goldenPath = "testdata/result.v2.golden.json"
)

// compileResultV2Schema compiles bench/schema/result.v2.schema.json under
// Draft 2020-12, offline (the schema is added as a local resource — no network
// fetch). Mirrors the validator analog in test/oracle/contract/schema_test.go.
func compileResultV2Schema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	schemaBytes, err := os.ReadFile(schemaPath)
	require.NoError(t, err, "read %s", schemaPath)

	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)

	// Decode the schema document with jsonschema.UnmarshalJSON (Pitfall 2:
	// preserves json.Number so numeric/format keywords evaluate correctly).
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	require.NoError(t, err, "unmarshal schema doc")
	require.NoError(t, c.AddResource(schemaPath, doc), "add schema resource")

	sch, err := c.Compile(schemaPath)
	require.NoError(t, err, "compile %s", schemaPath)
	return sch
}

// loadGoldenInstance reads and decodes the golden fixture as a validation
// instance via jsonschema.UnmarshalJSON (NOT encoding/json — Pitfall 2).
func loadGoldenInstance(t *testing.T) any {
	t.Helper()
	fixture, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "read %s", goldenPath)
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(fixture))
	require.NoError(t, err, "unmarshal golden instance")
	return inst
}

// TestResultV2GoldenValidates asserts the canonical golden result.v2.json
// fixture validates against result.v2.schema.json under Draft 2020-12.
func TestResultV2GoldenValidates(t *testing.T) {
	sch := compileResultV2Schema(t)
	inst := loadGoldenInstance(t)
	require.NoError(t, sch.Validate(inst), "golden result.v2.json must validate against the schema")
}

// TestResultV2SchemaIsValidDraft2020 asserts the schema document itself is a
// valid Draft 2020-12 schema by validating it against the bundled meta-schema
// (resolved offline).
func TestResultV2SchemaIsValidDraft2020(t *testing.T) {
	schemaBytes, err := os.ReadFile(schemaPath)
	require.NoError(t, err, "read %s", schemaPath)

	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	meta, err := c.Compile("https://json-schema.org/draft/2020-12/schema")
	require.NoError(t, err, "compile Draft 2020-12 meta-schema")

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	require.NoError(t, err, "unmarshal schema doc")
	require.NoError(t, meta.Validate(doc), "result.v2.schema.json is not a valid Draft 2020-12 schema")
}

// TestResultV2RequiresSchemaVersion asserts schema_version is required: an
// instance with that key removed must fail validation.
func TestResultV2RequiresSchemaVersion(t *testing.T) {
	sch := compileResultV2Schema(t)
	inst := loadGoldenInstance(t)

	m, ok := inst.(map[string]any)
	require.True(t, ok, "golden instance must decode to a JSON object")
	delete(m, "schema_version")

	require.Error(t, sch.Validate(m), "instance missing schema_version must NOT validate")
}

// fullMetricsDoc returns a result.v2 instance whose metrics object carries
// every metric + cached-token column set to an in-range value, plus a
// metric_errors[] array. Built as a raw JSON literal then decoded with
// jsonschema.UnmarshalJSON (Pitfall 2: preserve json.Number).
func decodeInstance(t *testing.T, raw string) any {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(raw)))
	require.NoError(t, err, "unmarshal instance literal")
	return inst
}

// TestResultV2Metrics covers the additive metrics object, the metric_errors[]
// array, nullable typing, range constraints, and the additive-only regression
// (the pre-existing golden fixture, which has no metrics object, still
// validates after the schema edit).
func TestResultV2Metrics(t *testing.T) {
	sch := compileResultV2Schema(t)

	t.Run("fully-populated metrics object validates", func(t *testing.T) {
		inst := decodeInstance(t, `{
			"schema_version": "v2",
			"metrics": {
				"task_success": true,
				"verified_correctness": false,
				"tokens_input": 18432,
				"tokens_output": 2048,
				"tokens_input_cached_read": 16384,
				"tokens_input_cache_write": 1024,
				"tool_calls": 12,
				"wall_time_seconds": 42.5,
				"files_read": 7,
				"bytes_read": 91230,
				"files_modified": 3,
				"edit_locality": 0.85,
				"regression_rate": 0.0,
				"lsp_diagnostics_used": 4,
				"semantic_tool_calls": 9,
				"edit_distance_patch": 220,
				"retry_count": 1,
				"compile_errors_before": 2,
				"compile_errors_after": 0
			}
		}`)
		require.NoError(t, sch.Validate(inst), "fully-populated metrics object must validate")
	})

	t.Run("null metric values validate (nullable typing)", func(t *testing.T) {
		inst := decodeInstance(t, `{
			"schema_version": "v2",
			"metrics": {
				"tokens_input": null,
				"edit_locality": null,
				"task_success": null,
				"compile_errors_after": null
			}
		}`)
		require.NoError(t, sch.Validate(inst), "null metric values must validate (nullable typing)")
	})

	t.Run("metric_errors array validates", func(t *testing.T) {
		inst := decodeInstance(t, `{
			"schema_version": "v2",
			"metric_errors": [
				{"metric": "tokens_input", "grader": "token_meter", "reason": "no provider usage block"},
				{"metric": "edit_locality", "grader": "tool_trace_analyzer", "reason": "no diff to score"}
			]
		}`)
		require.NoError(t, sch.Validate(inst), "metric_errors array must validate")
	})

	t.Run("out-of-range edit_locality is rejected", func(t *testing.T) {
		inst := decodeInstance(t, `{
			"schema_version": "v2",
			"metrics": {"edit_locality": 1.5}
		}`)
		require.Error(t, sch.Validate(inst), "edit_locality > 1 must be rejected (maximum: 1)")
	})

	t.Run("negative tokens_input is rejected", func(t *testing.T) {
		inst := decodeInstance(t, `{
			"schema_version": "v2",
			"metrics": {"tokens_input": -1}
		}`)
		require.Error(t, sch.Validate(inst), "negative tokens_input must be rejected (minimum: 0)")
	})

	t.Run("additive-only: pre-existing golden fixture (no metrics object) still validates", func(t *testing.T) {
		inst := loadGoldenInstance(t)
		require.NoError(t, sch.Validate(inst), "metric-sparse golden fixture must still validate after the additive schema edit")
	})
}

// TestResultV2AdditiveFieldStaysValid asserts the additive-only=minor contract
// (D-03): adding a brand-new unknown optional field keeps the instance valid
// because additionalProperties is left open at the top level.
func TestResultV2AdditiveFieldStaysValid(t *testing.T) {
	sch := compileResultV2Schema(t)
	inst := loadGoldenInstance(t)

	m, ok := inst.(map[string]any)
	require.True(t, ok, "golden instance must decode to a JSON object")
	m["future_metric_xyz"] = "a field added by a later phase without a schema bump"

	require.NoError(t, sch.Validate(m), "an extra optional field must keep the instance valid (additive-only=minor)")
}
