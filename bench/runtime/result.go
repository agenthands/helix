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
	"sort"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runners"
	benchschema "github.com/agenthands/helix/bench/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// resultSchemaVersion is the pinned result-contract version (D-04). It is the
// schema's only required field and is a const "v2" in the schema.
const resultSchemaVersion = "v2"

// SwebenchRawResolvedKey and SwebenchRescoredVerifiedKey are the two PINNED
// snake_case open-provenance result-row doc KEY NAMES (Phase 87, VERIFIED-02)
// stamped onto the result doc as OPEN keys by the SWE-bench rescore producer
// (Plan 03 rescore.go ApplyToRow) and read at score time by the aggregator
// (Plan 04 rowSwebenchScores). They are declared ONCE here — the single shared
// home both the Plan 03 producer and the Plan 04 reader import — mirroring the
// bench/canary.DocKeyCompletion key-name precedent so the writer and reader can
// never drift on the spelling. These are key NAMES only: NO schema property is
// added, `required` is unchanged, and additionalProperties stays OPEN at the
// schema top level (no v3 bump). The values carried under these keys are the
// raw-upstream `report.resolved` verdict and the UTBoost-rescored
// verified_correctness verdict respectively, reported side-by-side by the
// aggregator (raw-vs-rescored column).
const (
	SwebenchRawResolvedKey      = "swebench_raw_resolved"
	SwebenchRescoredVerifiedKey = "swebench_rescored_verified"
)

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

	// EmbedderID is the Phase 83 (ABLATE-04 #3) open-provenance key recording which
	// embedding model produced a baseline_rag RAG row (e.g. "text-embedding-3-small",
	// "nomic-embed-text", or "stub-deterministic"). It mirrors the Outcome/TraceRef
	// open keys exactly. Honest non-RAG modes leave it "" so the omitempty doc field
	// drops the key — only baseline_rag sets it. The recorded string answers a
	// "weak embedder" pushback with the exact model used (T-83-03-01).
	EmbedderID string

	// Language is the Phase 85 (ADAPTER-AIDER-01) open-provenance key recording the
	// language axis of this cell, populated from Cell.Language at the cell.go
	// BuildResult call site. It mirrors the EmbedderID open key exactly: additive-
	// minor, omitempty, schema_version stays "v2", additionalProperties stays OPEN.
	// A row written before the language axis existed (or a non-per-language path)
	// leaves it "" so the omitempty doc field drops the key and old artifacts stay
	// byte-compatible. The aggregator slices per-language pass-rate on it (SC#1).
	Language string

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

	// ContainerID is the Phase 87 (ADAPTER-SWE-01) open-provenance key recording
	// the Docker container/image identity the SWE-bench harness ran the instance
	// in, projected into the open key `container_id` (omitempty). It mirrors the
	// EmbedderID/Language additive-minor discipline EXACTLY: additive-minor,
	// omitempty, schema_version stays "v2", additionalProperties stays OPEN, NOT
	// added to required. Honest non-SWE-bench rows leave it "" so the omitempty
	// doc field drops the key and old artifacts stay byte-compatible; only the
	// SWE-bench ingestion path (Plan 02) populates it. Passed through verbatim —
	// no top-level default/fallback.
	ContainerID string

	// ExitCode is the Phase 87 (ADAPTER-SWE-01) open-provenance key recording the
	// SWE-bench harness subprocess exit code, projected into the open key
	// `exit_code` (omitempty). CRITICAL (Pitfall 2): it is a *int NOT an int — a
	// literal 0 is a real "ran clean" exit code that a value-type omitempty would
	// wrongly drop; nil = "no exit code captured" (drops the key). Mirrors the
	// EmbedderID/Language additive-minor discipline: additive-minor, omitempty,
	// schema_version stays "v2", additionalProperties stays OPEN, NOT added to
	// required. Only the SWE-bench ingestion path (Plan 02) populates it. Passed
	// through verbatim — no top-level default/fallback.
	ExitCode *int

	// SwebenchRawResolved / SwebenchRescoredVerified are the Phase 87 (VERIFIED-02)
	// open-provenance keys the SWE-bench rescore producer (Plan 03 rescore.go
	// ApplyToRow) STAMPS onto the row under the PINNED SwebenchRawResolvedKey /
	// SwebenchRescoredVerifiedKey names so the Plan 04 aggregator (rowSwebenchScores)
	// reads them back present=true on a LIVE row. They are *bool (NOT bool) WITH
	// omitempty so a nil drops the key but a literal false (the SC#2 buggy-patch
	// rescored verdict, the WHOLE POINT) is PRESERVED — a value-type omitempty would
	// wrongly drop a load-bearing false (Pitfall 2). They mirror the EmbedderID/
	// Language/ContainerID additive-minor discipline EXACTLY: additive-minor,
	// omitempty, schema_version stays "v2", additionalProperties stays OPEN, NOT
	// added to required. Only the SWE-bench rescore path populates them; honest non-
	// SWE-bench rows leave both nil so old artifacts stay byte-compatible.
	SwebenchRawResolved      *bool
	SwebenchRescoredVerified *bool

	// EditFormatApplied is the Phase 100 (EDITBENCH-03) open-provenance key the
	// polyglot-edit (aider_edit) cell STAMPS onto the row to record whether the
	// deterministic EDIT-verb agent actually applied the reference edit format to
	// the solution stub (true) or could not (false). It is a *bool (NOT bool) WITH
	// omitempty so a nil drops the key but a literal false — the load-bearing "edit
	// format NOT applied" verdict, the WHOLE POINT — is PRESERVED; a value-type
	// omitempty would wrongly drop a load-bearing false (Pitfall 2). It mirrors the
	// SwebenchRawResolved additive-minor discipline EXACTLY: additive-minor,
	// omitempty, schema_version stays "v2", additionalProperties stays OPEN, NOT
	// added to required. Only the aider_edit path populates it; honest non-edit-bench
	// rows leave it nil so old artifacts stay byte-compatible.
	EditFormatApplied *bool

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

	// EmbedderID is the Phase 83 (ABLATE-04 #3) embedder-provenance key. WITH
	// omitempty so honest non-RAG modes (EmbedderID=="") emit nothing; only the
	// baseline_rag arm carries the selected embedding model string. additive-minor
	// open key — additionalProperties is OPEN at the schema top level, no v3 bump.
	EmbedderID string `json:"embedder_id,omitempty"`

	// Language is the Phase 85 (ADAPTER-AIDER-01) language-axis provenance key. WITH
	// omitempty so a pre-language artifact / non-per-language path (Language=="")
	// emits nothing and stays byte-compatible; the per-language runner path carries
	// the cell's language string. additive-minor open key — additionalProperties is
	// OPEN at the schema top level, no v3 bump.
	Language string `json:"language,omitempty"`

	// AblationStatus is the D-03 deferral marker. WITH omitempty so honest modes
	// emit nothing; only the no_semantic arm carries "guarantee_pending_phase_81".
	AblationStatus string `json:"ablation_status,omitempty"`

	// ContainerID is the Phase 87 (ADAPTER-SWE-01) SWE-bench container-provenance
	// key. WITH omitempty so non-SWE-bench rows (ContainerID=="") emit nothing and
	// stay byte-compatible; only the SWE-bench ingestion path carries the harness
	// container/image id. additive-minor open key — additionalProperties is OPEN
	// at the schema top level, no v3 bump.
	ContainerID string `json:"container_id,omitempty"`

	// ExitCode is the Phase 87 (ADAPTER-SWE-01) SWE-bench harness exit-code key.
	// *int (NOT int) WITH omitempty so a nil drops the key but a literal 0 (a real
	// "ran clean" exit) is PRESERVED — a value-type omitempty would wrongly drop a
	// clean zero (Pitfall 2). additive-minor open key — additionalProperties is
	// OPEN at the schema top level, no v3 bump.
	ExitCode *int `json:"exit_code,omitempty"`

	// SwebenchRawResolved / SwebenchRescoredVerified are the Phase 87 (VERIFIED-02)
	// SWE-bench raw-vs-rescored open keys. *bool (NOT bool) WITH omitempty so a nil
	// drops the key but a literal false (the SC#2 rescored verdict) is PRESERVED — a
	// value-type omitempty would wrongly drop a load-bearing false (Pitfall 2). The
	// json tags are the PINNED SwebenchRawResolvedKey / SwebenchRescoredVerifiedKey
	// const VALUES verbatim, so producer (ApplyToRow) and reader (aggregator
	// rowSwebenchScores) read the same spelling. additive-minor open keys —
	// additionalProperties is OPEN at the schema top level, no v3 bump.
	SwebenchRawResolved      *bool `json:"swebench_raw_resolved,omitempty"`
	SwebenchRescoredVerified *bool `json:"swebench_rescored_verified,omitempty"`

	// EditFormatApplied is the Phase 100 (EDITBENCH-03) polyglot-edit open key.
	// *bool (NOT bool) WITH omitempty so a nil drops the key but a literal false
	// (the "edit format NOT applied" verdict) is PRESERVED — a value-type omitempty
	// would wrongly drop a load-bearing false (Pitfall 2). additive-minor open key —
	// additionalProperties is OPEN at the schema top level, no v3 bump.
	EditFormatApplied *bool `json:"edit_format_applied,omitempty"`

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
		EmbedderID:     in.EmbedderID,
		Language:       in.Language,
		AblationStatus: in.AblationStatus,
		ContainerID:    in.ContainerID,
		ExitCode:       in.ExitCode,

		SwebenchRawResolved:      in.SwebenchRawResolved,
		SwebenchRescoredVerified: in.SwebenchRescoredVerified,

		EditFormatApplied: in.EditFormatApplied,

		Metrics: in.Metrics,
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
// `null`) when there are no overrides (D-04). The projected overrides are sorted
// by mode before returning: fc.Overrides is a map, and ranging it in Go's
// randomized iteration order would emit fairness.overrides[] in a different
// order across builds, breaking the repo's byte-identical-sha256 reproducibility
// guarantee once ≥2 overrides exist.
func fairnessBlock(fc runners.FairnessContract) resultFairness {
	overrides := make([]resultOverride, 0, len(fc.Overrides))
	for mode, ov := range fc.Overrides {
		overrides = append(overrides, resultOverride{
			Mode:         mode,
			WaiverReason: ov.WaiverReason,
			ApprovedBy:   ov.ApprovedBy,
		})
	}
	sort.Slice(overrides, func(i, j int) bool { return overrides[i].Mode < overrides[j].Mode })
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
