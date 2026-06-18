# Bench Metrics (Phase 79)

This document is the human-readable source of truth for the per-task metric
record the bench harness produces. Every metric defined here maps 1:1 to a
nullable field on the Go `evaluators.Metrics` struct (`bench/evaluators/metrics.go`)
and to a property of the `metrics` object in `bench/schema/result.v2.schema.json`.
The snake_case names below are the contract: they are identical across the Go
struct json tags, the JSON schema, and this document.

## Invariants

- **Every metric is nullable, and a missing metric is an explicit JSON `null`,
  never an omission (METRIC-01 / D-06 / D-07).** A grader that cannot compute a
  metric leaves its field `null` and records why in the `metric_errors[]` array
  (`{metric, grader, reason}`).
- **Per-metric failure isolation (D-07).** The coordinator
  (`bench/evaluators/coordinator`) runs all five graders. One grader's failure
  nulls only that grader's metric(s) and appends a `metric_errors` entry; every
  other metric still populates and the row is always emitted and schema-valid.
- **The metrics live per `(task, mode, run_index)`.** The durable result is
  written to `<out>/<task>/<mode>/<run_index>/result.v2.json` alongside the
  single merged `trace.json` for that cell (METRIC-06: one merged trace per cell,
  no re-merge).
- **Scripted-null rule (D-01).** The Go-ToolBench corpus runs a no-LLM scripted
  agent with no provider `usage` block; for those runs the token metrics are
  explicit `null`, never `0` and never synthetically estimated.

## Graders

| Grader | Metrics it produces |
|--------|---------------------|
| `test_runner` | `task_success`, `verified_correctness`, `compile_errors_before`, `compile_errors_after` |
| `patch_validator` | `files_modified`, `edit_locality`, `edit_distance_patch` |
| `regression_checker` | `regression_rate` |
| `token_meter` | `tokens_input`, `tokens_output`, `tokens_input_cached_read`, `tokens_input_cache_write` |
| `tool_trace_analyzer` | `tool_calls`, `wall_time_seconds`, `files_read`, `bytes_read`, `lsp_diagnostics_used`, `semantic_tool_calls`, `retry_count` |

## The metrics

### task_success
Whether the task's success oracle passed. Sourced from the post-patch
`TestOutcome.Passed` gate, which is **exit-code-authoritative** (exit code == 0),
never the count of parsed per-test rows.

### verified_correctness
Whether the stronger verified-correctness oracle passed. Same exit-authoritative
source as `task_success` for the current fixtures.

### compile_errors_before / compile_errors_after
The compile-error count before and after the agent's edits. Derived from the
pre-patch and post-patch outcomes: a non-zero test exit with **no parsed test
rows** is read as a compile failure (count 1); a non-zero exit **with** rows is a
test failure (count 0). `compile_errors_before` is `null` when no pre-patch
outcome was captured.

### tokens_input / tokens_output
Total input (prompt) and output (completion) tokens billed for the run.

**Source-of-truth (METRIC-03 / D-02):** tokens come **exclusively from the
provider response's `usage` block** (the merged trace's `Usage`), never from
Helix's MCP-side tool-call counter or any daemon-side byte tally. The token_meter
reads `usage` when present and emits `null` for all four token columns when it is
absent. The provider-usage assertion is exercised on a real-LLM (`your_agent_full`)
path; the scripted corpus has no `usage` block, so its token metrics are `null`
by design (D-01).

### tokens_input_cached_read / tokens_input_cache_write
FAIR-03 substrate (D-03): input tokens served from a prompt-cache read, and input
tokens written into the prompt cache. Same present-or-null rule as the headline
token columns. A present-and-zero cached count is recorded as `0` (a real
observation), distinct from the scripted `null`.

### tool_calls
Total number of tool calls made during the run. Pinned to the merged trace's
`ToolCallSummary.Total` (METRIC-06); the analyzer never re-merges the trace.

### wall_time_seconds
Wall-clock duration of the run in seconds, derived from the merged trace's
`DurationMs`.

### files_read / bytes_read
Number of files read and total bytes read during the run, summed from the
read/list tool events in the merged trace (`ResultSizeBytes`).

### files_modified
The count of git-tracked files under the task repo subtree that the agent
changed. This is the numerator of `edit_locality` (see below).

### edit_locality (METRIC-04)

**Definition:** `edit_locality = 1 − (modified_files / total_files_in_repo_subtree)`.

- **Denominator `total_files_in_repo_subtree` (D-04):** the set of **git-tracked
  files** under the task repo subtree, enumerated via `git ls-files`. Generated,
  vendored, and **untracked** files are excluded from the denominator — this makes
  the metric deterministic and reproducible across machines and reflects the real
  source surface rather than build artifacts.
- **Numerator `modified_files`:** the count of those tracked files the agent
  changed, computed as `git diff --name-only` ∩ tracked-files, restricted to paths
  under the repo dir.
- **Edge cases:**
  - **Root-only / single-file edit →** the formula approaches `1.0` as the repo
    grows (one modified file out of N tracked files; for a one-file repo it is
    `0.0` by the literal formula — a single edit in a single-file repo touched the
    entire tracked surface). The "concentrated edit ≈ high locality" reading
    (Assumption A3) holds for any repo with more than one tracked file.
  - **All-files edit → `0.0`** (every tracked file modified ⇒ `1 − 1 = 0`).
  - **Zero tracked files →** `edit_locality` (and `files_modified`) is `null` with
    a `patch_validator` `metric_errors` entry; there is no meaningful denominator.

### regression_rate (METRIC-05)

**Definition:** `regression_rate = failing_pre-existing_tests_post_patch /
passing_pre-existing_tests_pre_patch`.

- **Pre/post double-run (D-05):** the `regression_checker` runs the fixture's
  full pre-existing test set **twice** — once **pre-patch** (the cached passing
  set, keyed on `(Package, Name)`, is the denominator) and once **post-patch**.
  The harness captures the pre-patch snapshot **before** driving the agent's edit
  (a `RunTests` call before the drive step in `RunCell`), so the denominator
  reflects the repo as it stood before the patch.
- **Numerator:** members of the cached pre-patch **passing** set that are no
  longer passing post-patch (failing **or** absent — a disappeared test is no
  longer passing).
- **Pre-existing failures do NOT count.** A test that was already failing
  pre-patch is excluded from both numerator and denominator, so a patch is never
  blamed for a regression that predates it (Pitfall 6).
- **Empty passing set →** `regression_rate` is `null` with a
  `regression_checker` `metric_errors` entry (no denominator to divide by).
- **Cost note (D-05):** the double-run is cheap for the tiny single-module Go
  ToolBench fixtures. A targeted/cached strategy for SWE-bench-scale external
  repos is deferred to Phase 87.

### lsp_diagnostics_used
Number of LSP diagnostics consulted during the run, counted from `get_diagnostics`
tool events in the merged trace.

### semantic_tool_calls
Number of semantic (SMTC) tool calls, summed over the merged trace's per-tool
tally for the semantic tool-name set (the 9 `internal/kernel/symbols` tools —
the strings that actually land in the trace, not the `mcp__smtc__*` client
aliases).

### edit_distance_patch

**Definition (resolved Open Q4):** the **sum of added + deleted lines reported by
`git diff --numstat`** across the working tree. Binary or malformed `--numstat`
lines contribute `0`.

This is the deterministic, in-house edit-distance definition: it requires no
reference patch and no external diff library, and is reproducible from the git
working tree alone. It is **not** a Levenshtein/string edit distance against a
gold patch — it is the line-level churn of the agent's diff.

### retry_count
Number of API retries during the run, counted from `KindAPIRetry` events in the
merged trace.
