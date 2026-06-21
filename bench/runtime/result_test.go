package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/languages"
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

// TestEmbedderIDOmittedWhenEmpty (Phase 83 Task 1, ABLATE-04 #3): a build with
// EmbedderID=="" (every honest non-RAG mode) emits NO embedder_id key and still
// passes Validate. embedder_id is an open, additive-minor provenance key —
// additionalProperties is OPEN at the schema's top level, so the key's presence
// or absence is schema-valid either way; omitempty drops it for honest modes.
func TestEmbedderIDOmittedWhenEmpty(t *testing.T) {
	in := ResultInput{
		TaskID:    "internal-toolbench/IT-go-patch-apply-1",
		Mode:      "your_agent_full",
		Benchmark: "internal-toolbench",
		RunIndex:  0,
		Outcome:   "success",
		TraceRef:  "bench/reports/x/trace.json",
		Fairness:  runners.DefaultContract,
		// EmbedderID intentionally left "" — honest non-RAG mode.
	}

	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc), "an EmbedderID=='' build must still validate (additive-minor open key)")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc, &raw))
	_, present := raw["embedder_id"]
	assert.False(t, present,
		"an honest non-RAG mode (EmbedderID=='') must OMIT the embedder_id key (omitempty)")
}

// TestEmbedderIDEmittedWhenSet (Phase 83 Task 1, ABLATE-04 #3): a build with a
// non-empty EmbedderID (the baseline_rag arm) emits the embedder_id key carrying
// the recorded model string and still passes Validate. This is the provenance the
// leaderboard reads to answer "which embedder produced this RAG row" (T-83-03-01).
func TestEmbedderIDEmittedWhenSet(t *testing.T) {
	in := ResultInput{
		TaskID:     "internal-toolbench/IT-go-patch-apply-1",
		Mode:       "baseline_rag",
		Benchmark:  "internal-toolbench",
		RunIndex:   0,
		Outcome:    "success",
		TraceRef:   "bench/reports/x/trace.json",
		Fairness:   runners.DefaultContract,
		EmbedderID: "text-embedding-3-small",
	}

	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc), "a set-EmbedderID build must validate")

	var got struct {
		EmbedderID string `json:"embedder_id"`
	}
	require.NoError(t, json.Unmarshal(doc, &got))
	assert.Equal(t, "text-embedding-3-small", got.EmbedderID,
		"a baseline_rag row must record the selected embedder_id")
}

// TestRunIndexPathSegment (Pitfall 3 / V5): the durable result + trace paths
// carry a <run_index> segment derived from cfg.RunIndex, and the segment is
// routed through validatePathSegment before the join.
func TestRunIndexPathSegment(t *testing.T) {
	const out = "/reports/run"
	const task = "IT-go-patch-apply-1"
	const mode = "your_agent_full"

	t.Run("index-2", func(t *testing.T) {
		resultPath, tracePath, err := cellDurablePaths(out, task, mode, 2)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(out, task, mode, "2", "result.v2.json"), resultPath)
		assert.Equal(t, filepath.Join(out, task, mode, "2", "trace.json"), tracePath)
	})

	t.Run("index-0", func(t *testing.T) {
		resultPath, _, err := cellDurablePaths(out, task, mode, 0)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(out, task, mode, "0", "result.v2.json"), resultPath)
	})

	t.Run("negative-index-rejected", func(t *testing.T) {
		// A negative run index formats to "-1" which validatePathSegment rejects
		// (leading dot/dash is not a clean numeric segment); the guard must fire.
		_, _, err := cellDurablePaths(out, task, mode, -1)
		require.Error(t, err, "a non-clean run_index segment must be rejected by the guard")
	})
}

// recordingRunner is a fixture LanguageRunner that records the order of its
// lifecycle calls, so the pre-patch snapshot ordering can be asserted without a
// real daemon.
type recordingRunner struct {
	calls    *[]string
	outcome  languages.TestOutcome
	runTests func()
}

func (r recordingRunner) Detect(string) bool { return true }
func (r recordingRunner) Setup(context.Context, string) error {
	*r.calls = append(*r.calls, "setup")
	return nil
}
func (r recordingRunner) RunTests(context.Context, string) (languages.TestOutcome, error) {
	*r.calls = append(*r.calls, "runtests")
	if r.runTests != nil {
		r.runTests()
	}
	return r.outcome, nil
}
func (r recordingRunner) Capabilities() []languages.Capability { return nil }

// TestRunCellPrePatchOrder asserts the pre-patch snapshot seam runs the
// registered runner's Setup+RunTests and returns its outcome, so RunCell can
// capture a pre-patch passing set BEFORE driving the agent's edit (D-05/Pitfall 6).
func TestRunCellPrePatchOrder(t *testing.T) {
	var calls []string
	rr := recordingRunner{
		calls: &calls,
		outcome: languages.TestOutcome{
			Passed: true,
			Tests:  []languages.TestResult{{Package: "p", Name: "TestSeed", Passed: true}},
		},
	}

	out, err := prePatchSnapshot(context.Background(), rr, t.TempDir())
	require.NoError(t, err, "pre-patch snapshot must run cleanly")
	require.NotNil(t, out, "a registered runner must yield a pre-patch outcome")
	assert.True(t, out.Passed, "pre-patch outcome must reflect the runner's result")
	assert.Equal(t, []string{"setup", "runtests"}, calls,
		"pre-patch snapshot must run Setup then RunTests")

	// A nil runner (no structured runner registered) yields no snapshot.
	out, err = prePatchSnapshot(context.Background(), nil, t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, out, "no runner -> no pre-patch outcome (verify.sh fallback path)")
}

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
		// Caller explicitly sets a genuine 0 (e.g. a real run that billed 0 tokens):
		// an explicit non-nil pointer is honored as-is and emits 0 (LO-01: only a
		// NIL top-level pointer defaults from the canonical metrics value).
		TokensInput:  iPtr(0),
		TokensOutput: iPtr(0),
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

	// (4) an explicitly-set 0 is honored at the top level.
	assert.EqualValues(t, 0, m["tokens_input"], "explicit tokens_input must be 0")
	assert.EqualValues(t, 0, m["tokens_output"], "explicit tokens_output must be 0")

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

// TestResultV2TopLevelTokensAgreeWithMetrics covers LO-01: when the caller leaves
// the top-level token pointers nil (the wired path), the builder defaults them
// from the canonical metrics values so the two homes agree. A scripted run with
// metrics.tokens_input == null must emit top-level tokens_input == null too — not
// a fabricated 0.
func TestResultV2TopLevelTokensAgreeWithMetrics(t *testing.T) {
	t.Run("scripted: both homes null", func(t *testing.T) {
		in := ResultInput{
			TaskID:    "t",
			Mode:      "your_agent_full",
			Benchmark: "internal-toolbench",
			Outcome:   "success",
			TraceRef:  "trace.json",
			Fairness:  runners.DefaultContract,
			// TokensInput/Output left nil (wired path); Metrics tokens are nil.
		}
		doc, err := BuildResult(in)
		require.NoError(t, err)
		require.NoError(t, Validate(doc), "null top-level tokens must still validate")

		var m map[string]any
		require.NoError(t, json.Unmarshal(doc, &m))
		ti, present := m["tokens_input"]
		require.True(t, present, "tokens_input key must be present")
		assert.Nil(t, ti, "top-level tokens_input must be null when metrics.tokens_input is null (LO-01)")
		to, present := m["tokens_output"]
		require.True(t, present, "tokens_output key must be present")
		assert.Nil(t, to, "top-level tokens_output must be null when metrics.tokens_output is null (LO-01)")
	})

	t.Run("usage present: both homes carry the value", func(t *testing.T) {
		in := ResultInput{
			TaskID:    "t",
			Mode:      "your_agent_full",
			Benchmark: "internal-toolbench",
			Outcome:   "success",
			TraceRef:  "trace.json",
			Fairness:  runners.DefaultContract,
			Metrics: evaluators.Metrics{
				TokensInput:  iPtr(1234),
				TokensOutput: iPtr(567),
			},
		}
		doc, err := BuildResult(in)
		require.NoError(t, err)
		require.NoError(t, Validate(doc))

		var m map[string]any
		require.NoError(t, json.Unmarshal(doc, &m))
		assert.EqualValues(t, 1234, m["tokens_input"], "top-level mirrors metrics.tokens_input")
		assert.EqualValues(t, 567, m["tokens_output"], "top-level mirrors metrics.tokens_output")
	})
}

// TestAblationStatusProjected covers D-03: a row built with a non-empty
// AblationStatus (the deferred no_semantic marker) projects that exact value
// under the open provenance key `ablation_status`, and the row still passes the
// validate-on-write gate (top-level additionalProperties is OPEN — additive).
func TestAblationStatusProjected(t *testing.T) {
	in := ResultInput{
		TaskID:         "internal-toolbench/IT-go-patch-apply-1",
		Mode:           "your_agent_no_semantic",
		Benchmark:      "internal-toolbench",
		RunIndex:       0,
		Outcome:        "success",
		TraceRef:       "trace.json",
		Fairness:       runners.DefaultContract,
		AblationStatus: "guarantee_pending_phase_81",
	}
	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc), "an ablation_status-marked row must still validate (open additionalProperties)")

	var m map[string]any
	require.NoError(t, json.Unmarshal(doc, &m))
	got, present := m["ablation_status"]
	require.True(t, present, "ablation_status key must be present when set")
	assert.Equal(t, "guarantee_pending_phase_81", got,
		"ablation_status must round-trip the exact deferred-marker value (D-03)")
}

// TestAblationStatusOmittedWhenEmpty covers D-03: an honest mode (empty
// AblationStatus) omits the ablation_status key entirely (omitempty), and the
// row still validates.
func TestAblationStatusOmittedWhenEmpty(t *testing.T) {
	in := ResultInput{
		TaskID:    "internal-toolbench/IT-go-patch-apply-1",
		Mode:      "your_agent_full",
		Benchmark: "internal-toolbench",
		RunIndex:  0,
		Outcome:   "success",
		TraceRef:  "trace.json",
		Fairness:  runners.DefaultContract,
		// AblationStatus left empty (honest mode).
	}
	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc), "an honest-mode row must validate")

	var m map[string]any
	require.NoError(t, json.Unmarshal(doc, &m))
	_, present := m["ablation_status"]
	assert.False(t, present, "ablation_status must be omitted when empty (omitempty, honest mode)")
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

// TestLanguageEmittedWhenSet (Phase 85 Task 1, ADAPTER-AIDER-01): a build with a
// non-empty Language (the per-language runner path threading Cell.Language) emits
// the `language` key carrying the recorded language string and still passes
// Validate. This is the provenance the aggregator slices on to answer SC#1's
// "Python pass-rate" question. Mirrors the EmbedderID open-additive-key precedent.
func TestLanguageEmittedWhenSet(t *testing.T) {
	in := ResultInput{
		TaskID:    "aider-polyglot/exercism-python-1",
		Mode:      "your_agent_full",
		Benchmark: "aider-polyglot",
		RunIndex:  0,
		Outcome:   "success",
		TraceRef:  "bench/reports/x/trace.json",
		Fairness:  runners.DefaultContract,
		Language:  "python",
	}

	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc), "a set-Language build must validate")

	var got struct {
		Language string `json:"language"`
	}
	require.NoError(t, json.Unmarshal(doc, &got))
	assert.Equal(t, "python", got.Language,
		"a row built with Language set must record the language provenance key")
}

// TestLanguageOmittedWhenEmpty (Phase 85 Task 1, ADAPTER-AIDER-01): a build with
// Language=="" emits NO `language` key and still passes Validate. `language` is an
// open, additive-minor provenance key (additionalProperties is OPEN at the top
// level), so its presence or absence is schema-valid either way; omitempty drops
// it. This is the backward-compat guarantee: old result.v2 artifacts written
// before the language axis existed (Language unset) still validate.
func TestLanguageOmittedWhenEmpty(t *testing.T) {
	in := ResultInput{
		TaskID:    "internal-toolbench/IT-go-patch-apply-1",
		Mode:      "your_agent_full",
		Benchmark: "internal-toolbench",
		RunIndex:  0,
		Outcome:   "success",
		TraceRef:  "bench/reports/x/trace.json",
		Fairness:  runners.DefaultContract,
		// Language intentionally left "" — a pre-language artifact / non-runner path.
	}

	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc),
		"a Language=='' build must still validate (additive-minor open key, backward compatible)")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc, &raw))
	_, present := raw["language"]
	assert.False(t, present,
		"a row with Language=='' must OMIT the language key (omitempty) so old artifacts stay byte-compatible")
}

// TestLanguageBackwardCompatValidate (Phase 85 Task 1, T-85-01-03): a hand-rolled
// result.v2 fixture with NO `language` key (exactly what every pre-Phase-85
// artifact looks like) still passes Validate. This pins the schema honesty
// guarantee: adding the optional `language` property must NOT make the field
// required and must NOT close additionalProperties.
func TestLanguageBackwardCompatValidate(t *testing.T) {
	// Minimal pre-language result.v2: required schema_version + the named props a
	// real row carries, with NO language key.
	fixture := []byte(`{
  "schema_version": "v2",
  "task_id": "internal-toolbench/IT-go-patch-apply-1",
  "mode": "your_agent_full",
  "benchmark": "internal-toolbench",
  "run_index": 0,
  "tokens_input": null,
  "tokens_output": null,
  "fairness": {"overrides": []},
  "outcome": "success",
  "trace_ref": "bench/reports/x/trace.json",
  "model_id": "claude-sonnet-4-5-20250929",
  "metrics": {"task_success": true}
}`)
	require.NoError(t, Validate(fixture),
		"a pre-language result.v2 artifact (no `language` key) must still validate")
}
