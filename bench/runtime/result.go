// Package runtime implements the bench cell orchestration glue that wires the
// reused Phase 67 internal/eval machinery (sandbox, subprocess daemon, trace
// tap + merge) into a working `helix-bench run`. This file owns the
// result.v2.json builder (D-04): a pure transform from a ResultInput into a
// schema-valid, provenance-complete / metric-sparse result document, plus a
// validate-on-write gate (T-77-04) that refuses to emit a doc the committed
// schema rejects.
package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runners"
	benchschema "github.com/agenthands/helix/bench/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// resultSchemaVersion is the pinned result-contract version (D-04). It is the
// schema's only required field and is a const "v2" in the schema.
const resultSchemaVersion = "v2"

// ResultInput is the full set of provenance the cell orchestrator (Plan 03)
// hands the builder. Rich metrics (edit_locality, regression_rate, pass@k) are
// deliberately ABSENT here — Phase 79 owns them (D-04).
type ResultInput struct {
	// SchemaVersion is normally left empty; the builder defaults it to "v2".
	// A caller may set it, but a non-"v2" value will fail validate-on-write.
	SchemaVersion string

	// Named schema props.
	TaskID    string
	Mode      string
	Benchmark string
	RunIndex  int

	// Headline tokens (LEGACY top-level fields). These are deprecated in favor of
	// the canonical metrics.tokens_input / metrics.tokens_output (METRIC-03). A nil
	// pointer means "no token data" and is emitted as JSON null — never a
	// fabricated 0 (Pitfall 6 / METRIC-03 boundary), never sourced from a
	// daemon-side byte counter. The builder defaults these from the canonical
	// metrics values when the caller leaves them nil, so the two homes agree
	// (LO-01): a scripted run nulls BOTH rather than 0 at top-level and null in
	// metrics.
	TokensInput  *int
	TokensOutput *int

	// Open provenance props (not named in the schema; valid as additional
	// properties under the additive-only=minor contract — Pitfall 2 / Open Q3).
	// Stable snake_case keys chosen here: outcome, trace_ref, model_id.
	Outcome  string // pass/fail outcome derived from the merged trace / VerifyExitCode
	TraceRef string // durable merged-trace JSON path

	// Fairness is the compile-time contract the run executed under; the builder
	// projects its ModelID into model_id and its Overrides into fairness.overrides.
	Fairness runners.FairnessContract

	// AblationStatus is the D-03 machine-checkable deferral marker projected into
	// the open provenance key `ablation_status` (omitempty). "" for honest modes;
	// "guarantee_pending_phase_81" for the no_semantic arm whose kernel
	// disable_semantic_subsystem guarantee lands in Phase 81. The Phase 82
	// aggregator reads it to tell a partial no_semantic row from a clean one. SET
	// by the cell wiring in Plan 03 (this plan delivers the field + schema doc).
	AblationStatus string

	// Metrics is the canonical Phase 79 nullable metric record (D-06/METRIC-01),
	// assembled by the coordinator. Every field is a pointer; a nil marshals to an
	// explicit JSON null (never omitted), so a metric a grader could not compute is
	// recorded as null rather than dropped. The zero value is an all-null metrics
	// object, which still validates (every schema property is nullable).
	Metrics evaluators.Metrics
	// MetricErrors carries the D-07 per-metric grader-failure annotations. It is
	// omitted from the doc when empty (omitempty on resultDoc.MetricErrors).
	MetricErrors []evaluators.MetricError
}

// resultFairness is the schema's fairness block. overrides is always a non-nil
// slice so it marshals as `[]` (an empty array), never `null`, when there are
// no overrides (D-04 / schema result.v2.schema.json:55-76).
type resultFairness struct {
	Overrides []resultOverride `json:"overrides"`
}

// resultOverride mirrors the schema's fairness.overrides[] item shape.
type resultOverride struct {
	Mode         string `json:"mode"`
	WaiverReason string `json:"waiver_reason"`
	ApprovedBy   string `json:"approved_by"`
}

// resultDoc is the typed result.v2 document. Named schema props use the schema's
// snake_case keys; the open provenance props (outcome/trace_ref/model_id) ride
// alongside them as additional properties (valid because the schema leaves
// top-level additionalProperties OPEN). Rich metrics are intentionally not
// fields here, so they are absent from the emitted doc (D-04).
type resultDoc struct {
	SchemaVersion string `json:"schema_version"`
	TaskID        string `json:"task_id"`
	Mode          string `json:"mode"`
	Benchmark     string `json:"benchmark"`
	RunIndex      int    `json:"run_index"`
	// TokensInput/Output are LEGACY top-level mirrors of metrics.tokens_input /
	// metrics.tokens_output. Pointers so a nil emits JSON null (schema relaxed to
	// ["integer","null"]), keeping the two homes in agreement (LO-01).
	TokensInput  *int           `json:"tokens_input"`
	TokensOutput *int           `json:"tokens_output"`
	Fairness     resultFairness `json:"fairness"`

	// Open provenance props (Open Question 3 — stable snake_case keys).
	Outcome  string `json:"outcome"`
	TraceRef string `json:"trace_ref"`
	ModelID  string `json:"model_id"`

	// AblationStatus is the D-03 deferral marker. WITH omitempty so honest modes
	// emit nothing; only the no_semantic arm carries "guarantee_pending_phase_81".
	AblationStatus string `json:"ablation_status,omitempty"`

	// Phase 79 canonical metric record (METRIC-01/D-06). Metrics has NO omitempty:
	// the object (and every nullable field within it) is always emitted so a
	// missing metric is an explicit JSON null, never an omission (D-07).
	// MetricErrors is omitempty — an empty annotations slice is simply absent.
	Metrics      evaluators.Metrics       `json:"metrics"`
	MetricErrors []evaluators.MetricError `json:"metric_errors,omitempty"`
}

// BuildResult transforms a ResultInput into the canonical result.v2 JSON bytes.
// It is a pure function (no IO): the cell orchestrator validates the bytes with
// Validate before writing them to the durable out dir. schema_version defaults
// to "v2"; fairness.overrides is sourced from the contract (empty array when the
// contract has none).
func BuildResult(in ResultInput) ([]byte, error) {
	sv := in.SchemaVersion
	if sv == "" {
		sv = resultSchemaVersion
	}

	// LO-01: keep the legacy top-level token fields in agreement with the
	// canonical metrics.* home. When the caller leaves the top-level pointers nil
	// (the wired path — cell.go never sets them), default them from the canonical
	// metrics values, so a scripted run with metrics.tokens_input == null also
	// emits top-level tokens_input == null rather than a fabricated 0.
	tokensInput := in.TokensInput
	if tokensInput == nil {
		tokensInput = in.Metrics.TokensInput
	}
	tokensOutput := in.TokensOutput
	if tokensOutput == nil {
		tokensOutput = in.Metrics.TokensOutput
	}

	doc := resultDoc{
		SchemaVersion:  sv,
		TaskID:         in.TaskID,
		Mode:           in.Mode,
		Benchmark:      in.Benchmark,
		RunIndex:       in.RunIndex,
		TokensInput:    tokensInput,
		TokensOutput:   tokensOutput,
		Fairness:       fairnessBlock(in.Fairness),
		Outcome:        in.Outcome,
		TraceRef:       in.TraceRef,
		ModelID:        in.Fairness.ModelID,
		AblationStatus: in.AblationStatus,
		Metrics:        in.Metrics,
		MetricErrors:   in.MetricErrors,
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("bench/runtime: marshal result.v2: %w", err)
	}
	return b, nil
}

// fairnessBlock projects a FairnessContract's overrides into the schema's
// fairness block. The slice is always non-nil so it marshals as `[]` (not
// `null`) when there are no overrides (D-04).
func fairnessBlock(fc runners.FairnessContract) resultFairness {
	overrides := make([]resultOverride, 0, len(fc.Overrides))
	for mode, ov := range fc.Overrides {
		overrides = append(overrides, resultOverride{
			Mode:         mode,
			WaiverReason: ov.WaiverReason,
			ApprovedBy:   ov.ApprovedBy,
		})
	}
	return resultFairness{Overrides: overrides}
}

// Validate is the validate-on-write gate (T-77-04). It compiles the embedded
// result.v2.schema.json under Draft 2020-12 (offline) and validates resultBytes
// against it, returning a non-nil error for any doc the schema rejects. The
// compile pattern is the verbatim jsonschema/v6 pattern from
// bench/schema/result.v2_test.go (UnmarshalJSON preserves json.Number so
// numeric/format keywords evaluate correctly).
func Validate(resultBytes []byte) error {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)

	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(benchschema.ResultV2SchemaBytes))
	if err != nil {
		return fmt.Errorf("bench/runtime: unmarshal embedded result.v2 schema: %w", err)
	}
	if err := c.AddResource(benchschema.ResultV2SchemaID, schemaDoc); err != nil {
		return fmt.Errorf("bench/runtime: add result.v2 schema resource: %w", err)
	}
	sch, err := c.Compile(benchschema.ResultV2SchemaID)
	if err != nil {
		return fmt.Errorf("bench/runtime: compile result.v2 schema: %w", err)
	}

	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(resultBytes))
	if err != nil {
		return fmt.Errorf("bench/runtime: unmarshal result doc for validation: %w", err)
	}
	if err := sch.Validate(inst); err != nil {
		return fmt.Errorf("bench/runtime: result.v2 doc failed schema validation: %w", err)
	}
	return nil
}
