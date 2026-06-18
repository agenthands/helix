---
phase: 79-evaluators-result-schema-metrics-layer
fixed_at: 2026-06-18T14:09:52Z
review_path: .planning/phases/79-evaluators-result-schema-metrics-layer/79-REVIEW.md
iteration: 1
findings_in_scope: 10
fixed: 9
skipped: 1
status: partial
---

# Phase 79: Code Review Fix Report

**Fixed at:** 2026-06-18T14:09:52Z
**Source review:** .planning/phases/79-evaluators-result-schema-metrics-layer/79-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope (`fix_scope: all` = medium + warning + low): 10
- Fixed: 9 (MD-01, MD-02, WR-01, WR-02, WR-03, WR-05, LO-01, LO-02, LO-03)
- Skipped: 1 (WR-04 — documented design tradeoff / heavy-coupling risk)

**Gate results (run in the isolated worktree, all green):**
- `go build ./cmd/helix` — PASS (exit 0)
- `go vet ./...` — PASS (exit 0)
- `go test ./bench/...` — PASS (all packages ok)
- `go test ./internal/eval/trace/...` — PASS (touched by MD-01)

(The only build/vet stderr output is the pre-existing, unrelated tree-sitter
Swift `TOKEN_COUNT redefined` C warning, which does not fail the gates.)

All fix commits are scoped to source files only — the ~390 pre-existing unrelated
`.planning/` doc deletions in the working tree were NOT staged (each commit used
explicit `git add <paths>`; never `git add -A`/`.`).

## Fixed Issues

### MD-01: `usagePresent` derived from a token-value threshold (METRIC-03 / D-01/D-03)

**Files modified:** `internal/eval/trace/schema.go`, `internal/eval/trace/tap.go`, `internal/eval/trace/merge.go`, `internal/eval/trace/tap_test.go`, `internal/eval/trace/merge_test.go`, `bench/runtime/cell.go`
**Commit:** eb2f6d97
**Applied fix:** Threaded a true presence boolean end-to-end instead of inferring
one from values. Added `UsagePresent bool` to `CCTapResult` (set at the tap when
`env.Usage != nil`), `MergeInput.CC` carries it into `MergedTrace.UsagePresent`,
and `cell.go` now reads `cfg.Agent == "claude" && merged.UsagePresent`. A real
claude run reporting a genuine all-zero provider usage block is now classified
usage-present (token metrics emitted as pointers-to-0), not silently nulled; the
scripted leg leaves the flag false (preserving explicit null). TDD: added
`TestTapCCStream_AllZeroUsageStillPresent`, `TestTapCCStream_NoUsageBlockAbsent`,
and `TestMergeUsageAbsentSignal` (fail before the plumbing, pass after).

### MD-02: coordinator drops a computable `files_modified` in the zero-tracked path

**Files modified:** `bench/evaluators/coordinator/coordinator.go`, `bench/evaluators/coordinator/coordinator_test.go`
**Commit:** f497d143
**Applied fix:** Assign `m.FilesModified` whenever `EditLocality` returns a
non-nil `modified` count, independent of the locality error (the zero-tracked
case returns a value + a MetricError in one tuple). Additionally, when
`files_modified` is genuinely uncomputable (`modified == nil`, e.g. non-git
repo), emit a dedicated `files_modified` `metric_errors[]` entry so a nulled
metric always carries its own provenance (the live-artifact gap: `files_modified`
was null with NO annotation). TDD: `TestZeroTrackedFilesReportsFilesModified`
(git init, no commit → files_modified reported, edit_locality nulled) and
`TestUncomputableFilesModifiedAnnotated` (non-git dir → files_modified nulled
WITH its own annotation). **Note:** this is a state-handling correctness fix
verified by the two new tests against the exact reviewer scenario; behavior was
confirmed against a non-git / zero-tracked repo dir as the review required.

### WR-01: `regression_rate` treated a skipped post-patch test as a regression

**Files modified:** `bench/languages/runner.go`, `bench/languages/go/runner.go`, `bench/evaluators/regression_checker/regression_checker.go`, `bench/evaluators/regression_checker/regression_checker_test.go`
**Commit:** b7091996
**Applied fix:** Added a tri-state `Skipped bool` field to `TestResult` (set by
the Go runner on a test2json `skip` event) and excluded a deliberately-skipped
post-patch row from the regression numerator. A previously-passing test that is
ABSENT post-patch still counts (genuine coverage loss), distinct from a skip.
Policy is now documented on `RegressionRate`. TDD: added
`post-patch SKIP ... NOT counted → 0.0` and `post-patch ABSENT ... still counts
→ 1/3` sub-tests.

### WR-02: `edit_locality` numerator uses working-tree diff, not diff-against-baseline

**Files modified:** `bench/evaluators/patch_validator/patch_validator.go`
**Commit:** 5c1e6fe1
**Applied fix:** Documentation-only (one of the review's accepted fix options:
"At minimum document that the numerator is unstaged-only"). The working-tree-diff
baseline is documented as an intentional design choice in the phase SUMMARYs for
the scripted in-place fixtures, so changing the diff baseline to `git diff HEAD`
would alter locked metric semantics and was judged out of scope. Added a BASELINE
note to the package doc making the unstaged-only assumption and its future
migration condition explicit. No behavioral change.

### WR-03: `wall_time_seconds` reported 0.0 for an unset/zero span

**Files modified:** `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go`, `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer_test.go`, `bench/evaluators/coordinator/coordinator_test.go`
**Commit:** 850aae6b
**Applied fix:** Null `wall_time_seconds` with a MetricError when both
`StartedAt` and `EndedAt` are the zero time (no timing captured), distinguishing
it from a genuine sub-second run (which still reports present-and-zero because the
span WAS recorded). Updated the two fixtures that set `DurationMs` without a span
to carry a real span. TDD: `TestWallTimeNullWhenSpanUnset` and
`TestWallTimeZeroForCapturedSubMillisecondSpan`.

### WR-05: `readToolNames` included `list_dir`, inflating `files_read`/`bytes_read`

**Files modified:** `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go`, `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer_test.go`, `bench/evaluators/METRICS.md`
**Commit:** 7847d18f
**Applied fix:** Dropped `list_dir` from `readToolNames` so `files_read` /
`bytes_read` count only file-content reads (`read_file`), matching the metric
names. Updated the analyzer test to assert `list_dir` does not contribute
(files_read 2→1, bytes_read 350→100) and documented the chosen semantics in
METRICS.md.

### LO-01: dual-home `tokens_input`/`tokens_output` could disagree (0 vs null)

**Files modified:** `bench/runtime/result.go`, `bench/runtime/result_test.go`
**Commit:** 7d5fdb8a
**Applied fix:** Made the legacy top-level `tokens_input`/`tokens_output` fields
`*int` (schema already relaxed to `["integer","null"]`) and defaulted a nil
top-level pointer from the canonical `metrics.*` value in `BuildResult`, so a
scripted run nulls BOTH homes rather than emitting top-level `0` against
`metrics.tokens_input: null`. An explicitly-set 0 is still honored. `cell.go`
needed no change (it never set these). TDD:
`TestResultV2TopLevelTokensAgreeWithMetrics` (scripted → both null; usage present
→ both carry the value); updated the existing scripted test to use `iPtr(0)`.

### LO-02: git PATH-resolution failure indistinguishable from a clean non-zero exit

**Files modified:** `bench/evaluators/patch_validator/patch_validator.go`
**Commit:** 10d9bf7d
**Applied fix:** Detect `exec.ErrNotFound` in `gitLines` and wrap it with a
greppable `git-unavailable:` prefix, so an operator can distinguish "no git on
host" (environment-class failure that nulls both patch metrics on every cell)
from a normal per-metric miss. Diagnostic-only change (no behavioral branch
beyond the reason string); existing tests remain green.

### LO-03: `EditDistancePatch` swallowed malformed numstat lines as binary files

**Files modified:** `bench/evaluators/patch_validator/patch_validator.go`, `bench/evaluators/patch_validator/patch_validator_test.go`
**Commit:** 2de765bf
**Applied fix:** Special-case the binary sentinel (`"-\t-"` → contributes 0,
silent) and surface any OTHER non-integer numstat field as an
`edit_distance_patch` MetricError instead of silently dropping it. Extracted the
parse loop into a pure `sumNumstat` helper for testability. TDD: `TestSumNumstat`
covers text-sum, binary-sentinel-zero, and malformed-line-errors cases.

## Skipped Issues

### WR-04: `semantic_tool_calls` / `lsp_diagnostics_used` / read-tool allowlists are hand-maintained

**File:** `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go:32-59`
**Reason:** skipped — design tradeoff, not a correctness bug, with material
fix-side risk. The review's suggested fix is a test that cross-checks the
allowlists against the live kernel tool registry. Achieving that requires
importing `internal/kernel/{symbols,diag,fileops}` (which pull the full
tree-sitter + LSP machinery) into a lightweight bench grader test, risking an
import cycle and a heavy/slow test dependency for a metric that is not wrong
today. The prompt's judgment guidance explicitly lists hand-maintained allowlists
as a "known docs-drift class" to skip if not low-risk. The allowlists remain
correct for the current 53-tool registry; the existing in-code comments already
flag the keep-in-sync obligation. Deferred to a dedicated registry-introspection
task if/when the drift risk materializes.
**Original issue:** The semantic/diagnostic/read tool-name sets are hard-coded
string maps that must stay in lockstep with the kernel tool registrations; a
rename/addition would silently under-count with no test failure.

---

_Fixed: 2026-06-18T14:09:52Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
