# Phase 79: Evaluators & Result-Schema Metrics Layer - Research

**Researched:** 2026-06-18
**Domain:** Go-native benchmark grading layer (metrics extraction, JSON Schema versioning, OTel trace merge reuse, git working-tree diff analysis)
**Confidence:** HIGH (all seams verified by reading live source in this repo via `Read`/`grep`; no external package introduction)

## Summary

Phase 79 is a **net-new grader/metrics layer wired onto an already-complete substrate**. Phases 75/77/78 built every input these evaluators need: a validate-on-write `result.v2.json` builder (`bench/runtime/result.go`), a 2-leg OTel trace merger (`internal/eval/trace.Merge`) with PID-gated taps (`TapDaemonLog`/`TapCCStream`), a structured Go test runner (`bench/languages/go.GoRunner.RunTests` → `languages.TestOutcome`), and a per-cell orchestrator (`bench/runtime/cell.go RunCell`) that already produces the daemon log + agent stream + verify outcome + merged trace. The phase implements **5 grader packages** under `bench/evaluators/*` that transform those artifacts into the **17 normalized metrics**, extends `result.v2.schema.json` to **type all 17 as nullable fields** (a Phase-75 minor bump), and writes `bench/evaluators/METRICS.md`. [VERIFIED: read of all six source files above]

The load-bearing nuance (D-01/D-02) is that the **only corpus that exists today is the scripted Go ToolBench, which has no provider `usage` block** — so `token_meter` emits **explicit `null`** there, and the METRIC-03 source-of-truth regression test must target the wired-not-gating `your_agent_full` (claude) path, which `cell.go` already plumbs (`StartClaude`, `--output-format=stream-json`, `TapCCStream.Usage`). The current `result.v2` builder writes `tokens_input/output` as bare `int` (0 for scripted) and **does not carry rich metrics at all** (they are "absent by construction" per the Phase 77 P02 decision). Phase 79 must change the int token fields to nullable and add a `metrics` object — this is the schema's first move from "metric-sparse" to "metric-complete." [VERIFIED: `bench/runtime/result.go:40-44,75-89`; STATE.md Phase 77 P02 decision]

**Primary recommendation:** Build 5 thin grader packages plus a coordinator. **REUSE** `internal/eval/trace.Merge` + taps verbatim for METRIC-06 (the merger is already called inside `RunCell`; `tool_trace_analyzer` consumes the merged `trace.MergedTrace` rather than re-merging). Add a typed nullable `metrics` object to the schema (minor bump, `additionalProperties` stays open). Have each grader return a `(metric-value, *error)` pair so D-07 per-metric failure isolation nulls one metric without dropping the row. Write each metric **directly into the result.v2 doc via an extended `ResultInput`/`resultDoc`** (no per-grader fragment files — the existing single-doc builder is the natural seam).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Test pass/fail + per-test rows (METRIC-01 `task_success`, `verified_correctness`, METRIC-02 `compile_errors_before/after`) | `bench/evaluators/test_runner` | `bench/languages/go.GoRunner.RunTests` (substrate) | Grader reads the structured `TestOutcome` the LanguageRunner already produces; does not re-shell `go test` itself except for the before/after compile-error counts |
| Diff/locality (METRIC-01 `files_modified`, `edit_locality`, METRIC-02 `edit_distance_patch`) | `bench/evaluators/patch_validator` | git CLI (`git ls-files`, `git diff`) | Pure working-tree analysis in the cell's repo dir; no daemon, no LLM |
| Token accounting (METRIC-01 `tokens_input/output`, FAIR-03 cached columns) | `bench/evaluators/token_meter` | `trace.Usage` from `TapCCStream` / provider `usage` | Reads the merged trace's `Usage`; emits null when no `usage` present (scripted) |
| Tool-call + trace metrics (METRIC-01 `tool_calls`, `wall_time_seconds`, `files_read`, `bytes_read`, `lsp_diagnostics_used`, METRIC-02 `semantic_tool_calls`, `retry_count`; METRIC-06 merged trace) | `bench/evaluators/tool_trace_analyzer` | `internal/eval/trace.Merge` + `MergedTrace` (REUSE) | All these metrics are derivable from the already-merged `trace.MergedTrace` (`ToolCallSummary`, `Events`, `DurationMs`, `Usage`); the merge itself is reused, not rebuilt |
| Regression detection (METRIC-01 `regression_rate`) | `bench/evaluators/regression_checker` | `GoRunner.RunTests` run twice (pre/post patch) | Double-run pre/post the agent's patch against the cached passing set (D-05) |
| Schema typing of all 17 metrics (METRIC-01 acceptance) | `bench/schema/result.v2.schema.json` + `bench/runtime/result.go` builder | jsonschema/v6 validate-on-write | Schema is the contract; the builder is where the metrics object is assembled and validated |
| Coordinator: per-metric isolation + null annotation (D-07) | `bench/evaluators` (top-level or a `coordinator`/`metrics` sub-pkg) | all 5 graders | Owns the "run each grader, catch its error, null+annotate on failure, never drop the row" loop |

## Standard Stack

This phase introduces **no new third-party packages**. Every dependency is already in `go.mod` and in active use by the bench tree. [VERIFIED: `go.mod` grep; imports in `bench/runtime/*.go`]

### Core (all already present)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | Compile + validate `result.v2.schema.json` (Draft 2020-12, offline) | Already the validate-on-write engine in `bench/runtime/validate.go` and `result.v2_test.go` — reuse the verbatim compile pattern [VERIFIED: `go.mod`, `bench/runtime/result.go:144-168`] |
| `encoding/json` (stdlib) | go1.26 | test2json parsing, result doc marshal, CC stream parse | Established pattern across `bench/languages/go/runner.go`, `internal/eval/trace/tap.go` [VERIFIED] |
| `os/exec` (stdlib) | go1.26 | `git ls-files` / `git diff` for patch_validator; `go test` double-run for regression_checker | Same `exec.CommandContext` + `*exec.ExitError` split pattern as `GoRunner.RunTests` and `runVerify` [VERIFIED: `bench/runtime/cell.go:455-474`, `runner.go:70-104`] |
| `internal/eval/trace` | in-repo | `Merge`, `TapDaemonLog`, `TapCCStream`, `MergedTrace`, `Usage`, `ToolCallSummary`, `Event` | The METRIC-06 trace substrate — REUSE (D-discretion resolved below) [VERIFIED: `merge.go`, `tap.go`, `schema.go`] |
| `bench/languages` | in-repo | `LanguageRunner`, `TestOutcome`, `TestResult`, `RunnerFor` | test_runner + regression_checker grade `TestOutcome`; the interface already promises "Phase 79 grades structured output" [VERIFIED: `bench/languages/runner.go:10-13,38-78`] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `git` CLI | 2.47.3 | `git ls-files` (D-04 denominator), `git diff --numstat`/`git diff` (modified-files count, edit_distance_patch) | patch_validator only; the seed fixtures are git-tracked single-module repos [VERIFIED: `git --version`; `git ls-files bench/languages/go/` works] |
| Levenshtein / edit-distance (METRIC-02 `edit_distance_patch`) | — | Compute patch edit distance | **DECISION NEEDED** — could hand-roll a small Levenshtein over diff hunks, or reuse `git diff --numstat` (added+deleted lines) as a coarser proxy. See Open Questions. No external dep recommended. [ASSUMED — no existing edit-distance helper found in repo] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Re-call `trace.Merge` inside `tool_trace_analyzer` | Consume the `MergedTrace` `RunCell` already built (passed into the grader) | Re-merging would double-parse the daemon log + duplicate the PID-gate logic. The merge is already done in `cell.go:387`. **REUSE the merged object, do not re-merge.** [VERIFIED: `cell.go:387-401`] |
| Hand-rolled JSON Schema validation | jsonschema/v6 (already used) | Reuse — never hand-roll (see Don't Hand-Roll) |
| Per-grader fragment files merged by a coordinator | Each grader returns a typed struct; coordinator assembles one `metrics` object into the single `result.v2.json` | The single-doc builder (`BuildResult`) is the established seam; fragment files add filesystem coordination + a merge step for no benefit at single-rep scale. **Recommend direct-into-doc.** (See Open Questions for the planner to confirm.) |

**Installation:** None. `go build ./cmd/helix-bench && go test ./bench/...` covers it.

## Package Legitimacy Audit

> No external packages are introduced by this phase. All dependencies are stdlib, in-repo (`internal/eval/*`, `bench/*`), or already-present-and-in-use (`jsonschema/v6 v6.0.2`).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/santhosh-tekuri/jsonschema/v6` | go.mod (already present) | established | n/a | github.com/santhosh-tekuri/jsonschema | OK (pre-existing) | Approved — already in use by `bench/runtime/validate.go` |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────────────┐
                    │  bench/runtime/cell.go  RunCell  (Phase 77 — EXISTS) │
                    │                                                       │
   per-cell run  →  │  daemon.log ──┐                                       │
   (task,mode,      │  agent stream ┤→ trace.Merge ──→ MergedTrace          │
    run_index)      │  verify/RunTests → TestOutcome  (verifyExit)          │
                    │  repo working tree (git)                              │
                    └────────────────┬──────────────────────────────────────┘
                                     │  artifacts handed to the grader layer
                                     ▼
        ┌──────────────────────── bench/evaluators (Phase 79 — NEW) ───────────────────────┐
        │                                                                                    │
        │  coordinator: for each grader { value, err := grade(); if err { null + annotate } }│
        │                                                                                    │
        │  ┌────────────┐ ┌──────────────┐ ┌────────────┐ ┌────────────────────┐ ┌─────────┐│
        │  │test_runner │ │patch_validator│ │token_meter │ │ tool_trace_analyzer│ │regress. ││
        │  │            │ │               │ │            │ │   (REUSES Merge)   │ │checker  ││
        │  │TestOutcome │ │git ls-files,  │ │MergedTrace │ │ MergedTrace.       │ │RunTests ││
        │  │→ success,  │ │git diff →     │ │.Usage →    │ │ ToolCallSummary,   │ │×2 (pre/ ││
        │  │verified,   │ │files_modified,│ │tokens (or  │ │ Events, DurationMs │ │post) →  ││
        │  │compile_err │ │edit_locality, │ │ NULL if no │ │ → tool_calls,      │ │regress. ││
        │  │before/after│ │edit_distance  │ │ usage)     │ │ files_read, bytes, │ │_rate    ││
        │  └─────┬──────┘ └──────┬────────┘ └─────┬──────┘ │ lsp_diag, semantic,│ └────┬────┘│
        │        │               │                │        │ retry_count        │      │     │
        │        │               │                │        └─────────┬──────────┘      │     │
        │        └───────────────┴────────────────┴──────────────────┴─────────────────┘     │
        │                                     │  17 metric values + per-metric error annots  │
        └─────────────────────────────────────┼────────────────────────────────────────────-┘
                                              ▼
                    ┌─────────────────────────────────────────────────┐
                    │  bench/runtime/result.go  BuildResult  (EXTEND)  │
                    │  assemble `metrics` object (all 17, nullable)    │
                    │  → Validate() against result.v2.schema.json      │
                    │  → write result.v2.json  (one merged trace ref)  │
                    └─────────────────────────────────────────────────┘
```

The reader can trace: a cell run produces 4 artifacts → the coordinator fans them out to 5 graders → each grader emits its metric(s) or a null+annotation → the coordinator assembles a 17-field `metrics` object → the extended `BuildResult` types it into `result.v2.json` and validates it.

### Recommended Project Structure
```
bench/evaluators/
├── METRICS.md                  # METRIC-04/05 definitions (REQUIRED output artifact)
├── evaluators.go               # coordinator: Grade(ctx, GradeInput) (Metrics, []MetricError)
├── metrics.go                  # the typed nullable Metrics struct (17 *T fields) + MetricError
├── test_runner/                # success, verified_correctness, compile_errors_before/after
├── patch_validator/            # files_modified, edit_locality, edit_distance_patch
├── token_meter/                # tokens_input/output (+cached), provider-usage source-of-truth
├── tool_trace_analyzer/        # tool_calls, wall_time, files_read, bytes_read, lsp_diagnostics_used,
│                               #   semantic_tool_calls, retry_count  (REUSES trace.Merge output)
└── regression_checker/         # regression_rate (RunTests pre/post patch)
```

### Pattern 1: Nullable metric with per-metric error isolation (D-07)
**What:** Each metric is a pointer (`*int` / `*float64` / `*bool`) in a typed `Metrics` struct; a nil pointer marshals to JSON `null`. A failed grader records a `MetricError{Metric, Grader, Reason}` and leaves the pointer nil. The row is always emitted.
**When to use:** Every grader.
**Example:**
```go
// Source: pattern derived from internal/eval/report/eval_result.go (*float64 nullable)
//         + bench/languages/go/runner.go (ExitError vs infra-error split)
type Metrics struct {
    TaskSuccess        *bool    `json:"task_success"`
    VerifiedCorrectness *bool   `json:"verified_correctness"`
    TokensInput        *int     `json:"tokens_input"`         // nil → null (scripted, no usage; D-01)
    TokensOutput       *int     `json:"tokens_output"`
    EditLocality       *float64 `json:"edit_locality"`
    RegressionRate     *float64 `json:"regression_rate"`
    // ... all 17, every field a pointer
}

type MetricError struct {
    Metric string `json:"metric"`           // e.g. "tokens_input"
    Grader string `json:"grader"`           // e.g. "token_meter"
    Reason string `json:"reason"`           // e.g. "no provider usage block present"
}
// Coordinator: a grader returning (nil, err) leaves the metric null and appends a MetricError;
// all other metrics still populate; the (task,mode,run_index) row is still written + schema-valid.
```
The existing `internal/eval/report/eval_result.go` already proves the `*float64` "nullable when no ground truth" pattern (`ContextPrecision`, `ContextRecall`). [VERIFIED: `eval_result.go:46-48`]

### Pattern 2: REUSE the merged trace (do not re-merge) — METRIC-06
**What:** `RunCell` already calls `trace.Merge(...)` and returns the `MergedTrace` (`res.Merged`, written to `trace.json` as `trace_ref`). `tool_trace_analyzer` consumes that `MergedTrace` object directly.
**When to use:** All trace-derived metrics.
**Example:**
```go
// Source: bench/runtime/cell.go:387-401 (merge already happens here)
//         internal/eval/trace/schema.go:90-153 (MergedTrace fields)
func analyze(mt trace.MergedTrace) traceMetrics {
    return traceMetrics{
        ToolCalls:        len(filterDaemonToolCalls(mt.Events)), // or mt.ToolCallSummary.Total
        WallTimeSeconds:  float64(mt.DurationMs) / 1000.0,
        SemanticToolCalls: countSemantic(mt.ToolCallSummary.ByTool), // sum the 10 semantic tool names
        // files_read / bytes_read: derive from daemon tool_call events for read_file/list_dir tools
        //   via Event.Tool + Event.ResultSizeBytes / Event.ArgsSummary
        // lsp_diagnostics_used: count tool_call events where Event.Tool == "get_diagnostics" (etc.)
        // retry_count: count Events where Kind == trace.KindAPIRetry
    }
}
```
`MergedTrace` exposes everything needed: `ToolCallSummary.Total`, `ToolCallSummary.ByTool` (per-tool counts), `DurationMs`, `Events` (each carrying `Tool`, `Outcome`, `ResultSizeBytes`, `ArgsSummary`, `Kind`), and `Usage`. [VERIFIED: `internal/eval/trace/schema.go:82-158`]

### Pattern 3: Token source-of-truth = provider `usage`, never MCP counter (METRIC-03 / D-02)
**What:** `tokens_input/output` come from `MergedTrace.Usage` (which is sourced from `TapCCStream`'s parse of the CC `result` event's `usage` block — `input_tokens`, `output_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens`). For the scripted agent there is no `usage` → emit `null` (D-01).
**Example:**
```go
// Source: internal/eval/trace/tap.go:186-292 (ccUsage parse) + schema.go:82-88 (Usage)
func meterTokens(mt trace.MergedTrace, agent string) (*int, *int, *MetricError) {
    // The scripted agent (CI gate) has no provider usage block (D-01/D-02).
    if agent == "scripted" || isZeroUsage(mt.Usage) { // distinguish "absent" from "real 0"
        return nil, nil, &MetricError{"tokens_input", "token_meter", "no provider usage block (scripted run)"}
    }
    return &mt.Usage.InputTokens, &mt.Usage.OutputTokens, nil
}
```
**Pitfall:** `trace.Usage` is a plain struct with `int` fields, so a genuine absence and a real `0` are indistinguishable from the struct alone. The grader MUST decide null-vs-zero from an out-of-band signal (the agent kind / whether a CC `result` event with a usage block was seen). The `CCTapResult` carries `Usage` only when a `result` event was parsed — Phase 79 should thread a "usage present?" boolean (or compare against `agent=="claude"`). [VERIFIED: `tap.go:277-294` populates `Usage` only inside the `result` case]

### Anti-Patterns to Avoid
- **Re-merging the trace inside the grader** — the merge already happened in `RunCell`; double-merging re-parses logs and risks divergent PID-gate behavior. Pass the `MergedTrace`.
- **Sourcing tokens from a daemon-side byte counter** — explicitly forbidden by METRIC-03 / D-02 and the existing `ResultInput` doc comment ("never source these from a daemon-side byte counter"). [VERIFIED: `result.go:40-44`]
- **Emitting `0` for absent tokens** — D-01 forbids it; emit `null`.
- **Whole-row drop on one grader failure** — D-07 forbids it; null+annotate the single metric.
- **A `result.v3` schema** — D-06 + Phase 75 policy: adding nullable fields is a **minor** bump; `additionalProperties` stays open; only `schema_version: "v2"` stays required. [VERIFIED: schema `$comment`, line 6]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON Schema validation | Custom validator | `jsonschema/v6` (verbatim compile pattern in `bench/runtime/validate.go`) | Already the validate-on-write engine; Draft-2020-12 + `json.Number` handling is subtle |
| OTel trace merge / PID gate | A fresh merger | `internal/eval/trace.Merge` + `TapDaemonLog`(pid-gated)/`TapCCStream` | PID gating against cross-cell leakage (T-67-04) is proven green in Phase 78's cross_cell_test; rebuilding re-introduces that risk |
| test2json parsing | Re-parse `go test -json` | `GoRunner.RunTests` → `TestOutcome.Tests` | Already handles the compile-fail-with-zero-rows pitfall and exit-code-authoritative gate (Pitfall 4) |
| git working-tree diff | Walk the FS + diff by hand | `git ls-files` + `git diff`/`git diff --numstat` | Deterministic, respects `.gitignore`, matches D-04's "git-tracked files" denominator exactly |
| result.v2 doc assembly + write | New writer | Extend `BuildResult` / `resultDoc` + `writeDurable` (atomic temp+rename) | The atomic write + validate-on-write gate is already correct and race-safe |

**Key insight:** Phase 79 is ~90% wiring of existing seams plus the schema typing. The genuinely net-new logic is: (1) the `metrics` schema object + nullable typing, (2) `edit_locality`/`edit_distance_patch` via git, (3) `regression_checker`'s pre/post double-run, (4) the per-metric isolation coordinator. Everything else is a transform over artifacts that already exist.

## Runtime State Inventory

> Not a rename/refactor/migration phase. This is additive grader code + an additive (minor) schema change. No stored data, live-service config, OS-registered state, secrets, or build artifacts carry a string that this phase renames.
>
> **The one schema-evolution concern (not runtime state but adjacent):** existing committed golden `result.v2.json` fixtures (`bench/schema/testdata/`, any under `bench/reports/`) were written under the metric-sparse contract. Because the change is additive-only (new optional nullable fields, `additionalProperties` open), **old fixtures stay valid** — verified against the Phase 75 policy. The planner should still add a Wave-0 task to confirm the existing `result.v2_test.go` golden still validates after the schema edit. [VERIFIED: schema `$comment` additive-only policy; `bench/schema/testdata/` exists]

## Common Pitfalls

### Pitfall 1: null vs zero tokens is invisible in `trace.Usage`
**What goes wrong:** `trace.Usage` is `struct{ InputTokens int; ... }` — a real `0` and "no usage block" look identical. Emitting `0` for the scripted corpus violates D-01.
**Why it happens:** The struct can't represent absence; only the surrounding parse context (was a CC `result` event with `usage` seen?) knows.
**How to avoid:** Thread a "usage present" signal (agent kind == "claude" AND a `result` event was parsed) into `token_meter`; null when absent. Add a regression test on the `your_agent_full` path (D-02) and a null-assertion test on the scripted path.
**Warning signs:** A scripted Go ToolBench run shows `tokens_input: 0` instead of `null`. [VERIFIED: `tap.go:277-294`, `schema.go:82-88`]

### Pitfall 2: re-merging the trace double-counts or diverges from the canonical trace
**What goes wrong:** If `tool_trace_analyzer` calls `trace.Merge` again, it can re-tap the daemon log with a different PID assumption and produce a `MergedTrace` that disagrees with the one already written to `trace.json` (the `trace_ref`).
**How to avoid:** Consume `RunCell`'s `res.Merged` (or read the durable `trace.json`). One merge per cell.
**Warning signs:** `tool_calls` metric ≠ `ToolCallSummary.Total` in the cell's `trace.json`. [VERIFIED: merge is at `cell.go:387`; only the daemon leg's `KindToolCall` events are counted, `merge.go:97-99`]

### Pitfall 3: run_index path collision (carried over from Phase 77)
**What goes wrong:** `RunCell` writes `<OutDir>/<task>/<mode>/result.v2.json` with **no run_index segment**; `runOneCell` hard-codes `RunIndex: 0`. Two reps with the same OutDir overwrite each other. The Phase 77 IN-05 comment explicitly defers `<task>/<mode>/<run_index>/` to Phase 79.
**Why it happens:** Multi-run repetition (N≥3) is a Phase 82 concern but the *path layout* for it is a Phase 79 deliverable per the cell.go comment ("Threading RunIndex into the path ... is deferred to Phase 79 when repetitions land").
**How to avoid:** **Decide with the planner** whether Phase 79 threads `RunIndex` into the durable path now (the cell.go comment says it should) or stays single-rep. The Phase 79 goal text says "per `(task, mode, run_index)`", which argues for adding the run_index path segment. **Recommend: add the `<run_index>` path segment in Phase 79** and stop hard-coding `RunIndex: 0` in `runOneCell`. [VERIFIED: `cell.go:180-188`, `matrix.go:255`]

### Pitfall 4: compile failure yields zero test rows — never infer pass from absence
**What goes wrong:** A compile error gives a non-zero exit with empty `TestOutcome.Tests`. Counting "0 failing tests → pass" is wrong.
**How to avoid:** `task_success`/`verified_correctness` gate on `TestOutcome.Passed` (exit==0), never on the per-test rows. `compile_errors_before/after` (METRIC-02) is itself derived from "did it compile" — use `go build`/`go vet` exit or the empty-rows-with-nonzero-exit signal. Already documented as Pitfall 4 in the GoRunner. [VERIFIED: `bench/languages/go/runner.go:61-104`; STATE.md "Passed=(exit==0) authoritative gate ... compile-fail → Passed=false Tests=[]"]

### Pitfall 5: edit_locality denominator must be git-tracked, not a filesystem walk
**What goes wrong:** Walking the repo dir counts `.git/`, build artifacts, and untracked scratch — inflating the denominator and skewing `edit_locality`.
**How to avoid:** D-04 — denominator = `git ls-files` under the task subtree (tracked only); numerator = count of those tracked files the agent modified (`git diff --name-only` intersected with tracked). Unit-test the edge cases: root-only edit → `1 − 1/N` (≈1.0 for large N; the success criterion says "root-only = 1.0" — clarify whether that means a single-file denominator or the formula limit), all-files edit → `1 − N/N = 0.0`. [VERIFIED: D-04 in CONTEXT.md; `git ls-files` works in repo]

### Pitfall 6: regression double-run must cache the **pre-patch passing set** as the denominator
**What goes wrong:** Computing `regression_rate` against the full test set (not the pre-patch *passing* subset) miscounts tests that were already failing before the agent touched anything.
**How to avoid:** D-05 — run the fixture's full pre-existing test set **pre-patch**, cache the passing set as the denominator (`passing_pre-existing_tests_pre_patch`); run again post-patch; numerator = members of the cached passing set that now fail. The Go fixtures are tiny single-module repos, so the double-run cost is negligible. **Sequencing concern:** the pre-patch run must happen on the *unmodified clone* before the agent's edits — this means `regression_checker` needs a pre-patch test snapshot, which `RunCell` does not currently capture (it only runs tests post-patch). The planner must add a pre-patch `RunTests` call (on the freshly-cloned repo, before `driveScript`). [VERIFIED: `cell.go` runs tests only at step 6, post-drive; D-05 in CONTEXT.md]

## Code Examples

### Extending the result schema (additive nullable `metrics` object)
```jsonc
// Source: extend bench/schema/result.v2.schema.json (additive-only = minor, $comment policy)
// Add under "properties": (schema_version stays the ONLY required field)
"metrics": {
  "type": "object",
  "description": "All 17 normalized metrics (METRIC-01/02). Every metric is nullable: a missing metric is an explicit JSON null (failed grader, D-07), never an omission.",
  "properties": {
    "task_success":          { "type": ["boolean", "null"] },
    "verified_correctness":  { "type": ["boolean", "null"] },
    "tokens_input":          { "type": ["integer", "null"], "minimum": 0 },
    "tokens_output":         { "type": ["integer", "null"], "minimum": 0 },
    "tool_calls":            { "type": ["integer", "null"], "minimum": 0 },
    "wall_time_seconds":     { "type": ["number",  "null"], "minimum": 0 },
    "files_read":            { "type": ["integer", "null"], "minimum": 0 },
    "bytes_read":            { "type": ["integer", "null"], "minimum": 0 },
    "files_modified":        { "type": ["integer", "null"], "minimum": 0 },
    "edit_locality":         { "type": ["number",  "null"], "minimum": 0, "maximum": 1 },
    "regression_rate":       { "type": ["number",  "null"], "minimum": 0 },
    "lsp_diagnostics_used":  { "type": ["integer", "null"], "minimum": 0 },
    "semantic_tool_calls":   { "type": ["integer", "null"], "minimum": 0 },
    "edit_distance_patch":   { "type": ["integer", "null"], "minimum": 0 },
    "retry_count":           { "type": ["integer", "null"], "minimum": 0 },
    "compile_errors_before": { "type": ["integer", "null"], "minimum": 0 },
    "compile_errors_after":  { "type": ["integer", "null"], "minimum": 0 }
  }
},
"metric_errors": {
  "type": "array",
  "description": "D-07 per-metric failure annotations. Each entry records a grader that failed and which metric it left null.",
  "items": {
    "type": "object",
    "properties": {
      "metric": { "type": "string" },
      "grader": { "type": "string" },
      "reason": { "type": "string" }
    }
  }
}
```
Note: the existing top-level `tokens_input`/`tokens_output` (currently typed `integer`, min 0) should be **relaxed to nullable** (or the canonical token home moved into `metrics`). The planner must decide whether to keep the top-level token fields (back-compat) AND add them under `metrics`, or migrate. Because `additionalProperties` is open and old fixtures only set the top-level ones, **relaxing the top-level fields to `["integer","null"]` is the lowest-risk additive move**; the `metrics` object becomes the canonical home for all 17. [VERIFIED: current schema lines 31-50]

### git-based edit_locality (METRIC-04 / D-04)
```go
// Source: pattern from bench/runtime/cell.go runVerify (exec.CommandContext + repoDir cwd)
func editLocality(ctx context.Context, repoDir string) (*float64, *int, error) {
    tracked, err := gitLines(ctx, repoDir, "ls-files")           // denominator: tracked files
    if err != nil { return nil, nil, err }
    modified, err := gitLines(ctx, repoDir, "diff", "--name-only") // numerator: changed tracked files
    if err != nil { return nil, nil, err }
    n := len(tracked)
    m := len(intersectTracked(modified, tracked))
    if n == 0 { return nil, &m, nil } // undefined denominator → null locality, annotate
    loc := 1.0 - float64(m)/float64(n)
    return &loc, &m, nil
}
```

### regression_checker pre/post double-run (METRIC-05 / D-05)
```go
// Source: bench/languages/go.GoRunner.RunTests (run it twice)
//   PRE-patch run MUST be on the unmodified clone, BEFORE driveScript (new step in RunCell).
prePass := passingSet(prePatchOutcome.Tests)               // cache denominator
postFail := 0
for _, t := range postPatchOutcome.Tests {
    if !t.Passed && prePass[key(t)] { postFail++ }          // regressed members of the cached set
}
var rate *float64
if len(prePass) == 0 { rate = nil } else { r := float64(postFail)/float64(len(prePass)); rate = &r }
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `result.v2` is metric-sparse (only 4 token fields + provenance; rich metrics "absent by construction") | `result.v2` carries a typed nullable `metrics` object with all 17 | Phase 79 (this phase) | The schema moves from substrate to metric-complete; the typed `resultDoc` struct in `result.go` gains a `Metrics` field |
| `tokens_input/output` typed as bare `int`, written `0` for scripted | nullable; `null` when no provider `usage` (D-01/D-02) | Phase 79 | METRIC-03 source-of-truth enforced; honest scripted corpus |
| Single-rep durable path `<task>/<mode>/` (RunIndex hard-coded 0) | `<task>/<mode>/<run_index>/` (recommended) | Phase 79 (per cell.go IN-05 deferral) | Unblocks Phase 82 multi-run; stop hard-coding `RunIndex: 0` |

**Deprecated/outdated:** none — this is the first metrics layer; no prior grader code to deprecate. `internal/eval/{score,judge,report}` are the *eval* (PR-gate smoke) lineage and are **explicitly NOT to be modified** (BENCH-01 byte-identical rule). They are reference patterns only, not building blocks to import into `bench/evaluators` (see Open Questions Q2).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `edit_distance_patch` (METRIC-02) is best computed via `git diff --numstat` (added+deleted lines) or a small hand-rolled Levenshtein over hunks — no external dep | Standard Stack / Supporting | If a specific edit-distance algorithm (e.g. Myers char-level) is required for comparability with external benchmarks, the proxy is wrong. METRIC-02 only requires "test records all 17 metrics", so any deterministic definition documented in METRICS.md likely suffices — confirm the intended definition with the planner. |
| A2 | Each grader writes directly into one `result.v2.json` via an extended `BuildResult` (no per-grader fragment files) | Architecture Patterns / Alternatives | If the planner prefers fragment files (e.g. for partial-failure durability or parallel grader writes), the coordinator shape changes. The discretion item in CONTEXT.md leaves this open. |
| A3 | "root-only edit = 1.0" in the METRIC-04 acceptance means the formula limit `1 − 1/N` for a single modified file in an N-file repo (→ 1.0 as N grows), not literally 1.0 for any N | Common Pitfalls / Pitfall 5 | If the acceptance literally requires `edit_locality == 1.0` for a single-file edit, the formula `1 − modified/total` cannot yield exactly 1.0 unless total→∞. The unit test fixture must be sized so the assertion holds (e.g. assert `> 0.9`), or the definition reinterpreted. Resolve against METRIC-04 exact wording. |
| A4 | `lsp_diagnostics_used` and `semantic_tool_calls` are counted by matching `Event.Tool` against the known diagnostic/semantic tool-name sets from the registry | Pattern 2 | If a tool is renamed or the semantic set is mis-enumerated, the counts drift. Anchor the name sets on the README tool inventory / registry (the 10 semantic tools, the diagnostic tools) rather than hard-coding. The quick-task 260617-t7x history shows the 10 semantic tools were recently re-synced — use that list. |
| A5 | The pre-patch regression snapshot requires a new `RunTests` call in `RunCell` before `driveScript` | Pitfall 6 | If the planner instead snapshots tests in the grader from a pristine re-clone, the cell.go change is unnecessary. Either works; confirm the integration point. |

## Open Questions (RESOLVED)

1. **Where do graders write — direct-into-doc vs per-grader fragments?**
   - What we know: `BuildResult`/`resultDoc` is the established single-doc seam; `writeDurable` is atomic.
   - What's unclear: CONTEXT.md leaves this to the planner (discretion item).
   - Recommendation: direct-into-doc via an extended `ResultInput.Metrics` + `resultDoc.Metrics`; coordinator assembles the struct, `BuildResult` marshals it, `Validate` gates it. Fragments add filesystem coordination for no benefit at single-rep scale.
   - **RESOLVED:** → direct-into-doc. 79-04 Task 2a wires `evaluators.Metrics` into `ResultInput.Metrics`/`resultDoc.Metrics`; `BuildResult` assembles and the verbatim `Validate` gate accepts it (no per-grader fragment files). Matches CONTEXT.md discretion item.

2. **Are `internal/eval/{score,judge,report}` reusable, or net-new?**
   - What we know: BENCH-01 mandates `eval/` stays **byte-identical** (only a one-paragraph EVAL.md pointer permitted). `report.ComputeContextMetrics` and `eval_result.go`'s `*float64` nullable pattern are good *templates*. `score.Apply` operates on `MergedTrace` daemon tool-call events — conceptually adjacent to `tool_trace_analyzer`.
   - What's unclear: whether `bench/evaluators` may *import* from `internal/eval/*` (read-only) or must be wholly net-new.
   - Recommendation: **Net-new packages under `bench/evaluators/*`**, copying the proven *patterns* (nullable pointers, exit-code-authoritative gating) but not importing `internal/eval/score|judge|report` (those are the eval PR-gate lineage; keeping bench independent honors the eval↔bench separation in INFRA-03). Importing `internal/eval/trace` is fine and expected (it is the shared trace substrate, already imported by `bench/runtime`).
   - **RESOLVED:** → net-new `bench/evaluators/*` packages (reflected across 79-02 + 79-03 graders and the 79-04 coordinator). `internal/eval/{score,judge,report}` are NOT imported (BENCH-01 eval↔bench separation); only `internal/eval/trace` is imported (the shared trace substrate, per tool_trace_analyzer in 79-03).

3. **Does Phase 79 add the `<run_index>` durable-path segment now?**
   - What we know: `cell.go` IN-05 comment defers it to Phase 79; the goal text says "per `(task, mode, run_index)`".
   - Recommendation: yes — add the segment and stop hard-coding `RunIndex: 0` in `matrix.go runOneCell`. Coordinate with Phase 82 (multi-run) expectations.
   - **RESOLVED:** → yes. 79-04 Task 2b adds the `<run_index>` durable-path segment in `cell.go` (guarded by `validatePathSegment`) and drops the hard-coded `RunIndex: 0` in `matrix.go runOneCell`.

4. **Exact `edit_distance_patch` definition** — see Assumption A1. Pick a deterministic definition, document it in METRICS.md.
   - **RESOLVED:** → `edit_distance_patch` = `git diff --numstat` added + deleted lines (the deterministic in-house definition). Implemented in 79-02 (patch_validator) and documented in 79-04 `bench/evaluators/METRICS.md`.

5. **Top-level token fields: relax-to-nullable vs migrate into `metrics`** — see the schema code example. Recommend relaxing the existing top-level `tokens_input/output` to `["integer","null"]` and making the `metrics` object the canonical home, to keep old fixtures valid.
   - **RESOLVED:** → relax-to-nullable. 79-01 Task 2 relaxes the existing top-level `tokens_input`/`tokens_output` from `"integer"` to `["integer","null"]` and makes the `metrics` object the canonical home (keeps old golden fixtures valid; additive-only minor bump).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | test_runner, regression_checker (`go test`/`go build`), all builds | ✓ | go1.26.0 | — |
| `git` CLI | patch_validator (`git ls-files`, `git diff`) | ✓ | 2.47.3 | none needed; fixtures are git repos |
| `jsonschema/v6` | schema validate-on-write | ✓ | v6.0.2 (in go.mod) | — |
| `claude` CLI | METRIC-03 source-of-truth regression test (`your_agent_full` path) | ✗ (not on this host; wired-not-gating per D-01) | — | The regression test can run against a **recorded CC `result` stream fixture** (a captured `--output-format=stream-json` with a `usage` block) fed to `TapCCStream`, asserting the token source — no live claude needed in CI |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** `claude` CLI — the METRIC-03 assertion is best implemented against a **captured CC stream-json fixture** with a real `usage` block (validating `TapCCStream.Usage` → `token_meter` → `result.v2`), keeping CI cost-free and deterministic. This mirrors the existing `SynthCCTap`/`cctap_test.go` fixture approach. [VERIFIED: `bench/runtime/cctap_test.go` exists; `tap.go:186-294` parses the usage block]

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ `jsonschema/v6` for schema assertions) |
| Config file | none — `go test ./bench/...` |
| Quick run command | `go test ./bench/evaluators/... ./bench/schema/... -count=1` |
| Full suite command | `go vet ./... && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| METRIC-01 | All 12 base metrics present (nullable, not omitted); schema validates | unit | `go test ./bench/schema/... -run TestResultV2Metrics -x` | ❌ Wave 0 |
| METRIC-01/D-07 | One failed grader → that metric `null` + `metric_errors` entry, row still emitted + schema-valid | unit | `go test ./bench/evaluators/... -run TestPerMetricIsolation -x` | ❌ Wave 0 |
| METRIC-02 | All 5 extended metrics recorded on a real task | integration | `go test ./bench/evaluators/... -run TestAllSeventeenMetrics -x` | ❌ Wave 0 |
| METRIC-03 | `tokens_input/output` sourced from provider `usage`, NOT MCP counter; cached columns separate | unit (fixture stream) | `go test ./bench/evaluators/token_meter/... -run TestProviderUsageSourceOfTruth -x` | ❌ Wave 0 |
| METRIC-03/D-01 | scripted run → tokens `null`, never 0 | unit | `go test ./bench/evaluators/token_meter/... -run TestScriptedNullTokens -x` | ❌ Wave 0 |
| METRIC-04 | `edit_locality` edge cases: root-only ≈ 1.0, all-files ≈ 0.0 (git-tracked denominator) | unit | `go test ./bench/evaluators/patch_validator/... -run TestEditLocality -x` | ❌ Wave 0 |
| METRIC-05 | `regression_rate` on a synthetic regression case (pre/post double-run, cached passing set) | unit | `go test ./bench/evaluators/regression_checker/... -run TestRegressionRate -x` | ❌ Wave 0 |
| METRIC-06 | Single merged trace per (task,mode,run_index); continuity root→LSP leaves; no orphan/foreign-PID spans | integration | `go test ./bench/evaluators/tool_trace_analyzer/... -run TestTraceMergeContinuity -x` (reuses `trace.Merge`; assert `RejectedForeignPid==0`, `ToolCallSummary.Total==tool_calls`) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/evaluators/... ./bench/schema/... -count=1` (sub-second; pure transforms over fixtures)
- **Per wave merge:** `go vet ./... && go test ./bench/...`
- **Phase gate:** Full suite green + `make bench-quick` (the ≤90s hermetic Go-ToolBench E2E smoke) produces a metric-complete `result.v2.json` before `/gsd-verify-work`.

### Edge cases to assert (per the success criteria)
- METRIC-04: root-only edit (single tracked file modified in an N-file repo) and all-files edit (every tracked file modified → 0.0); empty/zero-tracked-files repo → null + annotation (undefined denominator).
- METRIC-05: synthetic fixture where the agent's patch breaks a previously-passing test (rate > 0) AND a clean patch (rate == 0) AND a fixture with a pre-existing failing test (must NOT count toward the rate — it was not in the cached passing set).
- METRIC-03: (a) a captured CC stream fixture with a `usage` block → token_meter reads it; (b) the same task via scripted agent → `null`; (c) assert the value does NOT come from any daemon/MCP-side counter (negative test: a daemon-side byte total present in the trace must be ignored).
- METRIC-06: assert exactly one merged trace per cell; `RejectedForeignPid == 0`; no orphan spans (every CC tool_use correlates); `tool_calls` metric == `MergedTrace.ToolCallSummary.Total`.

### Wave 0 Gaps
- [ ] `bench/evaluators/metrics.go` — the typed nullable `Metrics` struct + `MetricError` (shared test fixtures)
- [ ] `bench/evaluators/evaluators_test.go` — per-metric isolation (D-07) + all-17-present (METRIC-01/02)
- [ ] `bench/evaluators/token_meter/token_meter_test.go` — provider-usage source-of-truth + scripted-null (needs a captured CC stream-json fixture under `testdata/`)
- [ ] `bench/evaluators/patch_validator/patch_validator_test.go` — edit_locality edge cases (needs tiny git-repo fixtures, or `git init` in a temp dir)
- [ ] `bench/evaluators/regression_checker/regression_checker_test.go` — pre/post double-run synthetic regression
- [ ] `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer_test.go` — trace metrics over a `MergedTrace` fixture (reuse the `trace` test helpers' shape)
- [ ] `bench/schema/result.v2_test.go` — extend: assert the new `metrics`/`metric_errors` validate AND that the **existing golden fixture still validates** (additive-only proof)
- [ ] Framework install: none (stdlib `testing` already in use)

## Security Domain

> `security_enforcement` is not disabled in config. This phase is a Go-native grader layer with no auth/session/network surface. SMTC has no security capability for Go, so `java-security` is not applicable here (per CLAUDE.md).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | Path-segment validation before any `filepath.Join` (the existing `validatePathSegment`/`validateCellKey` in `bench/runtime/validate.go`) — any new run_index path segment MUST go through it (T-77-08/T-77-10). [VERIFIED: `bench/runtime/validate.go:23-31`] |
| V6 Cryptography | no | — |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via task/mode/run_index segment in durable path | Tampering | Reuse `validatePathSegment` for the new `run_index` segment before joining (it currently validates task/mode/benchmark/language). [VERIFIED: `validate.go`] |
| Command injection via `git`/`go` args derived from repo content | Tampering | `exec.CommandContext` with fixed argv (no shell); `cmd.Dir = repoDir`; never interpolate untrusted strings into a shell — matches the existing `RunTests`/`runVerify` pattern. [VERIFIED: `runner.go:71`, `cell.go:459`] |
| Patch path escaping the repo root (agent writes outside subtree) | Tampering | Already enforced by `trace.Merge`'s path-prefix invariant (T-67-02) on `PatchPaths` vs `RepoRoot`; patch_validator should likewise resolve modified paths under `repoDir` before counting. [VERIFIED: `merge.go:122-139`] |

## Sources

### Primary (HIGH confidence — read live in this repo this session)
- `internal/eval/trace/merge.go` — `Merge(MergeInput) (MergedTrace, error)`, tool-call summary, path-prefix invariant, outcome resolution
- `internal/eval/trace/schema.go` — `MergedTrace`, `Event`, `Usage`, `ToolCallSummary`, `GuardrailCounts`, event-kind/outcome enums
- `internal/eval/trace/tap.go` — `TapDaemonLog(path, expectedPid)`, `TapCCStream(path)`, `ccUsage` parse (provider usage block)
- `bench/runtime/cell.go` — `RunCell`, the full per-cell spine; merge call site; run_index path deferral (IN-05)
- `bench/runtime/result.go` — `BuildResult`/`Validate`/`resultDoc`/`ResultInput`; the metric-sparse current contract + token boundary comment
- `bench/runtime/matrix.go` — `ExpandMatrix`/`RunMatrix`/`runOneCell` (RunIndex:0 hard-coded)
- `bench/runtime/validate.go` — `validatePathSegment` (V5 path-traversal guard)
- `bench/schema/result.v2.schema.json` + `bench/schema/schema.go` — current contract (4 typed token fields, open additionalProperties, additive-only=minor policy)
- `bench/languages/runner.go` + `bench/languages/go/runner.go` — `LanguageRunner`, `TestOutcome`, `TestResult`, `GoRunner.RunTests` (exit-authoritative gate, Pitfall 4)
- `internal/eval/score/score.go`, `internal/eval/report/{context_metrics,eval_result}.go` — pattern templates (nullable `*float64`, trace-event-derived metrics)
- `.planning/{REQUIREMENTS.md, STATE.md, ROADMAP.md, milestones/v1.12-ROADMAP.md}` + `79-CONTEXT.md` — requirements, locked decisions, phase goal/criteria

### Secondary (MEDIUM confidence)
- STATE.md Decisions log — Phase 75/77/78 schema + isolation decisions cross-referenced for the additive-only policy and the metric-sparse-by-construction history

### Tertiary (LOW confidence)
- none (no external/WebSearch sources were needed; the phase introduces no new third-party packages)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages; all deps verified present and in active use
- Architecture: HIGH — every seam (Merge, taps, TestOutcome, BuildResult, RunCell) read directly; reuse-vs-rebuild resolved by source evidence
- Pitfalls: HIGH — each pitfall is anchored to a specific line/decision (null-vs-0 in Usage, run_index path deferral, compile-fail rows, git-tracked denominator, pre-patch snapshot)
- Open questions: MEDIUM — 5 genuine discretion points (write location, eval reuse boundary, run_index path, edit_distance definition, token field migration) left for the planner with strong recommendations

**Research date:** 2026-06-18
**Valid until:** 2026-07-18 (stable — internal repo seams, no fast-moving external deps)
