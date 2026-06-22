# Phase 79: Evaluators & Result-Schema Metrics Layer - Pattern Map

**Mapped:** 2026-06-18
**Files analyzed:** 11 new/modified
**Analogs found:** 11 / 11 (all have a strong in-repo analog)

This phase is ~90% wiring of existing seams plus schema typing. Every new file copies a proven pattern already present in the bench/eval tree. No external packages.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/evaluators/metrics.go` (new) | model | transform | `internal/eval/report/eval_result.go` (nullable `*float64`) + `internal/eval/trace/schema.go` (typed structs) | exact (nullable-struct) |
| `bench/evaluators/evaluators.go` (new, coordinator) | service | transform | `bench/runtime/result.go` `BuildResult` (pure transform) | role-match |
| `bench/evaluators/test_runner/` (new) | service | transform | `bench/languages/go/runner.go` `GoRunner.RunTests` (consumes `TestOutcome`) | exact |
| `bench/evaluators/patch_validator/` (new) | service | file-I/O | `bench/runtime/cell.go` `runVerify` (`exec.CommandContext` + `cmd.Dir`) | role-match |
| `bench/evaluators/token_meter/` (new) | service | transform | `internal/eval/trace/tap.go` `ccUsage` + `schema.go` `Usage` | exact |
| `bench/evaluators/tool_trace_analyzer/` (new) | service | transform | `internal/eval/report` `ComputeContextMetrics` over `MergedTrace` | exact (trace-derived) |
| `bench/evaluators/regression_checker/` (new) | service | transform | `bench/languages/go/runner.go` `GoRunner.RunTests` (run twice) | exact |
| `bench/evaluators/METRICS.md` (new) | doc | — | (none — required output artifact) | n/a |
| `bench/schema/result.v2.schema.json` (modify) | config/schema | — | itself (additive nullable fields, Phase 75 minor-bump policy) | exact |
| `bench/runtime/result.go` (modify) | service | transform | itself (`ResultInput`/`resultDoc`/`BuildResult`/`Validate`) | exact |
| `bench/runtime/{cell.go,matrix.go}` (modify) | service | transform | itself (`RunCell` merge site; `runOneCell` `RunIndex:0`) | exact |

## Pattern Assignments

### `bench/evaluators/metrics.go` (model, nullable struct)

**Analogs:** `internal/eval/report/eval_result.go`, `internal/eval/trace/schema.go`

**Nullable-pointer-marshals-to-null pattern** (`eval_result.go:46-48`):
```go
// Context precision/recall placeholders (EVAL-01 NULLABLE until Phase 68+).
ContextPrecision *float64 `json:"context_precision"`
ContextRecall    *float64 `json:"context_recall"`
```
Replicate this for all 17 metrics — every field a pointer so a nil marshals to JSON `null` (D-06/D-07). Do NOT use `omitempty` (a null must be emitted, never omitted — METRIC-01 acceptance).

**Typed-struct + closed-enum house style** (`schema.go:82-95`):
```go
type Usage struct {
    InputTokens         int `json:"input_tokens"`
    OutputTokens        int `json:"output_tokens"`
    CacheReadTokens     int `json:"cache_read_tokens,omitempty"`
    CacheCreationTokens int `json:"cache_creation_tokens,omitempty"`
}
type ToolCallSummary struct {
    Total  int            `json:"total"`
    ByTool map[string]int `json:"by_tool"`
}
```

**`Metrics` + `MetricError` to build** (snake_case json keys must match the schema property names exactly):
```go
type Metrics struct {
    TaskSuccess         *bool    `json:"task_success"`
    VerifiedCorrectness *bool    `json:"verified_correctness"`
    TokensInput         *int     `json:"tokens_input"`
    TokensOutput        *int     `json:"tokens_output"`
    ToolCalls           *int     `json:"tool_calls"`
    WallTimeSeconds     *float64 `json:"wall_time_seconds"`
    FilesRead           *int     `json:"files_read"`
    BytesRead           *int     `json:"bytes_read"`
    FilesModified       *int     `json:"files_modified"`
    EditLocality        *float64 `json:"edit_locality"`
    RegressionRate      *float64 `json:"regression_rate"`
    LSPDiagnosticsUsed  *int     `json:"lsp_diagnostics_used"`
    SemanticToolCalls   *int     `json:"semantic_tool_calls"`
    EditDistancePatch   *int     `json:"edit_distance_patch"`
    RetryCount          *int     `json:"retry_count"`
    CompileErrorsBefore *int     `json:"compile_errors_before"`
    CompileErrorsAfter  *int     `json:"compile_errors_after"`
}
type MetricError struct {
    Metric string `json:"metric"`
    Grader string `json:"grader"`
    Reason string `json:"reason"`
}
```

---

### `bench/evaluators/test_runner/` & `regression_checker/` (service, transform)

**Analog:** `bench/languages/go/runner.go` `GoRunner.RunTests` + `bench/languages/runner.go` `TestOutcome`

**Exit-code-authoritative gate (Pitfall 4)** (`runner.go:98-103`, `runner.go:47-56`):
```go
return languages.TestOutcome{
    Passed:   exitCode == 0,   // AUTHORITATIVE gate — never infer pass from empty rows
    Tests:    tests,
    Raw:      raw,
    ExitCode: exitCode,
}, nil
```
`task_success`/`verified_correctness` MUST read `TestOutcome.Passed`, never `len(Tests)`. A compile failure gives `Passed=false, Tests=[]`.

**ctx-cancel→infra vs exit→outcome split** (`go/runner.go:78-94`) — reuse for the regression double-run:
```go
if ctx.Err() != nil { return languages.TestOutcome{Raw: raw}, ctx.Err() }
var exitErr *exec.ExitError
if errors.As(err, &exitErr) { exitCode = exitErr.ExitCode() } else { return ..., err }
```

**regression_rate double-run (D-05, METRIC-05):** call `RunTests` twice — pre-patch (on the unmodified clone, BEFORE `driveScript`; this needs a new pre-patch call wired in `RunCell`, see Pitfall 6) and post-patch. Cache the pre-patch *passing set* as the denominator; numerator = cached-passing members that now fail. Key per-test rows on `(Package, Name)` from `languages.TestResult`.

---

### `bench/evaluators/token_meter/` (service, transform)

**Analog:** `internal/eval/trace/tap.go` `ccUsage` parse + `schema.go:82-88` `Usage`

**Source-of-truth = provider `usage`, never MCP counter (METRIC-03/D-02):** read `MergedTrace.Usage` (populated only when a CC `result` event with a usage block was parsed — `tap.go:277-294`). For the scripted agent there is no usage block → emit `null` (D-01), never `0`.

**Pitfall:** `trace.Usage` is a plain `struct{ InputTokens int; ... }` — a genuine absence and a real `0` are indistinguishable from the struct alone. Thread an out-of-band "usage present" signal (agent kind == "claude" AND a `result` event seen) into the grader; null when absent. The METRIC-03 regression test runs against a **captured CC stream-json fixture** under `testdata/` (mirror `bench/runtime/cctap_test.go` / `SynthCCTap`), not a live `claude` CLI.

```go
func meterTokens(mt trace.MergedTrace, usagePresent bool) (in, out *int, e *MetricError) {
    if !usagePresent { // scripted run, D-01/D-02
        return nil, nil, &MetricError{"tokens_input", "token_meter", "no provider usage block (scripted run)"}
    }
    return &mt.Usage.InputTokens, &mt.Usage.OutputTokens, nil
}
```
Cached columns (`tokens_input_cached_read`/`tokens_input_cache_write`) follow the same present-or-null rule (D-03), sourced from `Usage.CacheReadTokens`/`CacheCreationTokens`.

---

### `bench/evaluators/tool_trace_analyzer/` (service, transform)

**Analog:** `internal/eval/report` `ComputeContextMetrics` (derives metrics from `MergedTrace`) + `internal/eval/trace/schema.go:104-158` field shapes

**REUSE the already-merged trace — do NOT re-merge (METRIC-06, Anti-Pattern):** `RunCell` already calls `trace.Merge(...)` at `cell.go:387` and exposes `res.Merged`. The analyzer consumes that `MergedTrace` object; re-merging re-parses the daemon log and risks divergent PID-gate behavior (`tool_calls` would disagree with `trace.json`).

All trace metrics derive from `MergedTrace` fields (`schema.go`):
```go
ToolCalls:         mt.ToolCallSummary.Total           // or len of daemon KindToolCall events
WallTimeSeconds:   float64(mt.DurationMs) / 1000.0
SemanticToolCalls: sum of mt.ToolCallSummary.ByTool over the 10 semantic tool names
// files_read / bytes_read  ← Event.Tool=="read_file"/"list_dir" + Event.ResultSizeBytes
// lsp_diagnostics_used     ← count Event.Tool=="get_diagnostics" (anchor names on registry, not hard-code — A4)
// retry_count              ← count Events where Kind == trace.KindAPIRetry
```
Assert in tests: `tool_calls == mt.ToolCallSummary.Total` and `tap.RejectedForeignPid == 0`.

---

### `bench/evaluators/patch_validator/` (service, file-I/O)

**Analog:** `bench/runtime/cell.go` `runVerify` (`exec.CommandContext` + `cmd.Dir = repoDir`, fixed argv, no shell)

**git-tracked denominator for edit_locality (METRIC-04/D-04):** `git ls-files` = denominator (tracked only, respects `.gitignore`); `git diff --name-only` ∩ tracked = numerator. `edit_locality = 1 − modified/total`. Zero tracked files → null + annotate (undefined denominator). Edge cases to unit-test: root-only edit ≈ 1.0, all-files edit = 0.0 (see Assumption A3 in RESEARCH — fixture must be sized so the assertion holds).

**Command-injection mitigation:** fixed argv via `exec.CommandContext`, `cmd.Dir = repoDir`, never interpolate repo content into a shell. Resolve modified paths under `repoDir` before counting (path-prefix invariant, mirrors `merge.go:122-139`).

---

### `bench/evaluators/evaluators.go` (coordinator, transform)

**Analog:** `bench/runtime/result.go` `BuildResult` (pure transform, no IO)

**Per-metric failure isolation (D-07):** run each grader; on a grader error, leave its metric pointer nil and append a `MetricError`; all other metrics still populate; the `(task, mode, run_index)` row is always emitted and schema-valid. Never whole-row-drop. Recommended seam (RESEARCH Open Q1): assemble one `Metrics` struct + `[]MetricError` and hand them to an extended `BuildResult` — no per-grader fragment files.

---

### `bench/schema/result.v2.schema.json` (modify, additive minor bump)

**Analog:** itself — additive-only = MINOR per the `$comment` policy (line 6); `additionalProperties` stays open; `schema_version` stays the only required field. NOT a v3.

Add a `metrics` object (all 17 fields typed `["<type>", "null"]`) and a `metric_errors` array (`{metric, grader, reason}` items) — exact shape in RESEARCH §Code Examples lines 248-282. **Relax** the existing top-level `tokens_input`/`tokens_output` from `"integer"` to `["integer","null"]` (lowest-risk additive move per Open Q5; keeps old golden fixtures valid). The `metrics` object becomes the canonical home for all 17.

---

### `bench/runtime/result.go` (modify, transform)

**Analog:** itself. Extend `ResultInput` with a `Metrics evaluators.Metrics` + `MetricErrors []evaluators.MetricError`; add the matching fields to `resultDoc` (`result.go:75-89`); assemble them in `BuildResult` (`result.go:102-114`). The **validate-on-write `Validate` gate** (`result.go:144-168`, verbatim jsonschema/v6 compile pattern) is reused unchanged — it now also gates the new fields.

---

### `bench/runtime/{cell.go,matrix.go}` (modify, run_index path)

**Analog:** itself. Two changes (RESEARCH Open Q3 / Pitfall 3):
1. Thread `RunIndex` into the durable path: `cell.go:187-188` currently joins `<OutDir>/<task>/<mode>/result.v2.json` with NO run-index segment; `matrix.go:255` hard-codes `RunIndex: 0`. Add a `<run_index>` segment and stop hard-coding 0.
2. Add a **pre-patch `RunTests`** call before `driveScript` (currently tests run only post-drive at `cell.go:365`) to feed `regression_checker`'s pre-patch snapshot (Pitfall 6 / A5).

## Shared Patterns

### Validate-on-write (jsonschema/v6)
**Source:** `bench/runtime/result.go:144-168` (`Validate`); schema embed at `bench/schema/schema.go:17-23` (`ResultV2SchemaBytes`, `ResultV2SchemaID`).
**Apply to:** every result.v2 write. Reuse the verbatim compile pattern; `jsonschema.UnmarshalJSON` preserves `json.Number` so numeric/format keywords evaluate correctly. Never hand-roll validation.

### Path-segment traversal guard (V5)
**Source:** `bench/runtime/validate.go:23-31` (`validatePathSegment`).
**Apply to:** the new `run_index` path segment BEFORE any `filepath.Join` (T-77-08/T-77-10). Rejects `..`, separators, leading dots.

### ctx-cancel→infra vs exit-code→outcome split
**Source:** `bench/languages/go/runner.go:78-94`; `bench/runtime/cell.go` `runVerify`.
**Apply to:** test_runner, regression_checker, patch_validator (any `exec.CommandContext` grader). A ctx cancellation/timeout is an infra error (return err); a non-zero subprocess exit is a recorded outcome (not an error).

### Nullable-pointer marshals to JSON null
**Source:** `internal/eval/report/eval_result.go:46-48`.
**Apply to:** all 17 metric fields. Pointer + no `omitempty` ⇒ a nil emits `null`, a value emits the value. This is the machine-enforceable form of "missing metrics are explicit nulls, not omissions."

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `bench/evaluators/METRICS.md` | doc | — | Required output artifact (METRIC-04/05 definitions). No code analog; write the `edit_locality` (D-04 git-tracked denominator) and `regression_rate` (D-05 pre/post double-run) definitions, plus the chosen `edit_distance_patch` definition (Open Q4 / A1 — pick a deterministic one, e.g. `git diff --numstat` added+deleted lines). |

## Reuse Boundary (do NOT import)

`internal/eval/{score,judge,report}` are the eval PR-gate lineage and are **byte-identical-locked** (BENCH-01). Copy their *patterns* (nullable pointers, exit-authoritative gating, trace-derived metrics) but do NOT import them into `bench/evaluators/*` — keep bench independent (INFRA-03). Importing `internal/eval/trace` IS fine and expected (shared trace substrate, already imported by `bench/runtime`).

## Metadata

**Analog search scope:** `bench/runtime/`, `bench/schema/`, `bench/languages/`, `internal/eval/{trace,report}/`
**Files read this session:** `bench/runtime/result.go`, `bench/runtime/validate.go`, `bench/runtime/cell.go` (merge site), `bench/runtime/matrix.go` (grep), `bench/schema/result.v2.schema.json`, `bench/schema/schema.go` (grep), `bench/languages/runner.go`, `bench/languages/go/runner.go`, `internal/eval/trace/schema.go`, `internal/eval/report/eval_result.go`
**Pattern extraction date:** 2026-06-18
