---
phase: 79-evaluators-result-schema-metrics-layer
reviewed: 2026-06-18T13:06:36Z
depth: deep
reviewer: gsd-code-reviewer
files_reviewed: 12
files_reviewed_list:
  - bench/schema/result.v2.schema.json
  - bench/schema/result.v2_test.go
  - bench/schema/testdata/result.v2.golden.json
  - bench/evaluators/metrics.go
  - bench/evaluators/coordinator/coordinator.go
  - bench/evaluators/test_runner/test_runner.go
  - bench/evaluators/patch_validator/patch_validator.go
  - bench/evaluators/regression_checker/regression_checker.go
  - bench/evaluators/token_meter/token_meter.go
  - bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go
  - bench/runtime/cell.go
  - bench/runtime/matrix.go
  - bench/runtime/result.go
  - bench/runtime/validate.go
findings:
  blocker: 0
  critical: 0
  high: 0
  medium: 2
  low: 3
  warning: 5
  total: 5
status: issues_found
---

# Phase 79: Code Review Report

**Reviewed:** 2026-06-18T13:06:36Z
**Depth:** deep
**Files Reviewed:** 12 source files (+ schema + golden fixture + tests as context)
**Status:** issues_found (highest severity: MEDIUM)

## Summary

Reviewed the Phase 79 evaluators / result-schema metrics layer under `bench/`: the
nullable `Metrics` contract, the five graders (test_runner, patch_validator,
regression_checker, token_meter, tool_trace_analyzer), the D-07 per-metric-failure
coordinator, the result.v2 schema additive relaxation, and the runtime wiring in
`cell.go` / `matrix.go` / `result.go` / `validate.go`.

Overall the load-bearing correctness rules hold up well under adversarial reading:

- **METRIC-03 token source-of-truth** — `token_meter.MeterTokens` reads *only*
  `mt.Usage.*`, never a tool-call tally or byte counter; the absent-usage path
  returns four explicit nils + a MetricError, never a fabricated 0. The package and
  its tests preserve the present-and-zero vs absent distinction. **However the only
  production caller derives the presence signal from a value threshold (see MD-01).**
- **METRIC-04 edit_locality** — denominator is `git ls-files` (tracked only),
  numerator is `git diff --name-only ∩ tracked ∩ under-repo`; zero-tracked nulls with
  a MetricError; `modified ≤ n` keeps the result in `[0,1]`. Sound.
- **METRIC-05 regression_rate** — denominator is the cached pre-patch passing set;
  a pre-existing failing test is not in that set, so its post-patch failure cannot
  count. Correct.
- **METRIC-06 trace** — `tool_trace_analyzer.Analyze` consumes the already-merged
  `trace.MergedTrace` and pins `tool_calls` to `ToolCallSummary.Total`; it never calls
  `trace.Merge`. Correct.
- **Security (V5 / command injection)** — every `run_index` / task / mode / benchmark /
  language path segment passes `validatePathSegment` (or `validateRunIndexSegment`)
  before any `filepath.Join`; all `git`/`go`/`/bin/sh` invocations use
  `exec.CommandContext` with fixed argv and `cmd.Dir`, never a shell with interpolated
  content. No path-traversal or command-injection surface found.

No BLOCKER or CRITICAL defects. Two MEDIUM findings (one latent correctness bug that
contradicts the documented token contract, one dropped-computable-metric inconsistency)
and three LOW items.

Field-name cross-check (deep pass): every `evaluators.Metrics` json tag matches a
`metrics.*` schema property key 1:1 (19/19); the trace fields read by the graders
(`Usage.InputTokens/OutputTokens/CacheReadTokens/CacheCreationTokens`,
`ToolCallSummary.Total/ByTool`, `Event.Tool/ResultSizeBytes/Kind`,
`KindAPIRetry`, `DurationMs`) all exist in `internal/eval/trace/schema.go` with the
read types. The additive schema relaxation is genuinely minor: the metric-sparse
golden fixture still validates and top-level `tokens_input/tokens_output` are now
`["integer","null"]`.

## Medium

### MD-01: `usagePresent` derived from a token-value threshold defeats the present-and-zero contract (METRIC-03 / D-01/D-02/D-03)

**File:** `bench/runtime/cell.go:497`
**Issue:**
```go
usagePresent := cfg.Agent == "claude" && (merged.Usage.InputTokens > 0 || merged.Usage.OutputTokens > 0)
```
This is the only production caller of `token_meter.MeterTokens`. The token_meter
package documents `usagePresent` as a *presence* signal — "agent kind == claude AND a
CC `result` event with a usage block was parsed" — explicitly *out-of-band* precisely
because `trace.Usage` is a plain value struct in which a genuine absence and a real 0
are indistinguishable (token_meter.go:9-17). The whole point of D-03 / the
`TestScriptedNullTokens` "present-and-zero must be a non-nil pointer to 0" assertion is
to keep a real `usage{input_tokens:0}` distinct from "no usage block".

Deriving the flag from `InputTokens > 0 || OutputTokens > 0` reintroduces exactly the
value-vs-presence conflation the grader is designed to defend against. A real claude
run whose provider `usage` block reports `input_tokens: 0` / `output_tokens: 0` (or one
whose only non-zero fields are the cache columns) is misclassified as usage-*absent*,
nulling all four token metrics — including `tokens_input_cached_read` /
`tokens_input_cache_write`, which could legitimately be non-zero with zero fresh
input/output.

Not currently triggerable: the bench runtime never populates `merged.Usage` — the
scripted CC leg sets `Usage: trace.Usage{}` (cctap.go:83) and the claude leg discards
the agent result (WR-03 in cell.go), so `merge.go:63` always copies a zero Usage. The
flag is therefore always `false` today. But it is wired wrong and will misfire the
moment the claude leg is finished and starts carrying real usage — silently and only on
the degenerate-but-real zero-token case, which is the hardest to notice.

**Fix:** Thread a true presence boolean instead of inferring one from values. The
cleanest seam is to carry an explicit "usage block was parsed" flag from the CC tap
(e.g. a `bool` on `trace.CCTapResult` / `MergedTrace` set when `env.Usage != nil` at
tap.go:285) and pass *that* into the coordinator:
```go
// merged.UsagePresent set by the tap when a CC result event carried a usage block.
usagePresent := cfg.Agent == "claude" && merged.UsagePresent
```
Until that plumbing exists, at minimum stop using a value threshold as a presence
proxy and document the temporary limitation; a value-of-0 must not be read as "absent".

### MD-02: coordinator drops a computable `files_modified` in the zero-tracked-files case, contradicting the grader contract

**File:** `bench/evaluators/coordinator/coordinator.go:90-96`
**Issue:**
```go
loc, modified, locErr := patch_validator.EditLocality(ctx, in.RepoDir)
if locErr != nil {
    errs = append(errs, *locErr)
} else {
    m.EditLocality = loc
    m.FilesModified = modified
}
```
`EditLocality` deliberately returns a non-nil `modified` pointer *together with* a
MetricError when the repo has zero tracked files (patch_validator.go:77-83), and its
doc comment promises "files_modified is still reported in that case" (patch_validator.go:41).
The coordinator only assigns `m.FilesModified` in the no-error branch, so on the
zero-tracked path the computable `files_modified` count is silently discarded and
`metrics.files_modified` is emitted as null even though the grader produced a value.

This is the inverse of the D-07 intent: D-07 nulls *only* the metric a grader could not
compute. Here a separate, fully-computed metric (`files_modified`) is nulled as
collateral because it shares a return tuple with the failed metric (`edit_locality`).

The patch_validator test that exercises the zero-tracked path discards the `modified`
return (`patch_validator_test.go:93` uses `_`), so no test catches this drift.

**Fix:** Assign `files_modified` whenever it is non-nil, independent of the locality
error:
```go
loc, modified, locErr := patch_validator.EditLocality(ctx, in.RepoDir)
if modified != nil {
    m.FilesModified = modified
}
if locErr != nil {
    errs = append(errs, *locErr)
} else {
    m.EditLocality = loc
}
```

## Warnings

### WR-01: `regression_rate` treats a skipped post-patch test as a regression

**File:** `bench/evaluators/regression_checker/regression_checker.go:61-71` (with `bench/languages/go/runner.go:148-154`)
**Issue:** `parseTest2JSON` maps `skip` → `Passed: false`. A test that passed pre-patch
but is `skip`-ped post-patch therefore has `postPassed[k] == false` and is counted as a
regression. A deliberate `t.Skip()` introduced by the agent (or a build-tag/env change
that skips a previously-running test) is not obviously a "newly-broken pre-existing
test" — it inflates `regression_rate`. This is a defensible modeling choice, but it is
undocumented and the regression_checker doc frames the numerator as "now failing OR
absent", omitting the skip case.
**Fix:** Decide and document the policy. If a skip should not count as a regression,
track a tri-state (pass/fail/skip) for post-patch rows and exclude skips from the
numerator; otherwise add a comment to regression_checker.go and the Go runner stating
that skip is intentionally treated as not-passing for regression purposes.

### WR-02: `edit_locality` numerator uses working-tree diff, not diff-against-baseline

**File:** `bench/evaluators/patch_validator/patch_validator.go:56`
**Issue:** `git diff --name-only` (and `--numstat`) report only *unstaged* working-tree
changes against the index. If a driven agent stages its edits (`git add`) — or a future
language runner's Setup stages files — the diff goes empty and `edit_locality` reports
1.0 / `edit_distance_patch` reports 0 despite real modifications. The metric silently
under-counts. For the current scripted Go fixtures (which edit in place and never stage)
this is fine, but the grader is presented as a general patch metric.
**Fix:** If the intent is "all changes since the seed clone", diff against the recorded
baseline commit (`git diff --name-only HEAD` or a captured pre-patch ref) rather than the
unstaged working tree, so staged edits are still counted. At minimum document that the
numerator is unstaged-only and relies on the driver never staging.

### WR-03: `wall_time_seconds` truncates to float seconds and can report 0.0 for sub-millisecond / unset spans

**File:** `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go:88`
**Issue:** `wall := float64(mt.DurationMs) / 1000.0`. `DurationMs` is computed in
`merge.go` as `int64(EndedAt.Sub(StartedAt) / time.Millisecond)` — integer-truncated.
A merged trace assembled from a zero/unset span (e.g. a synthesized or test trace with
`StartedAt == EndedAt`) yields `DurationMs == 0` and thus `wall_time_seconds == 0.0`,
which is emitted as a present pointer-to-0 rather than null. Distinguishing "ran for 0s"
from "no timing captured" is the same present-vs-absent hazard the token path is careful
about; here it is silently collapsed to 0. Low blast radius (timing is informational),
but worth a note.
**Fix:** Acceptable as-is for real runs; if the present-vs-absent distinction matters for
downstream stats, null `wall_time_seconds` when `EndedAt`/`StartedAt` are zero, and
document that `DurationMs` is millisecond-truncated.

### WR-04: `semantic_tool_calls` / `lsp_diagnostics_used` / read-tool sets are hand-maintained string allowlists prone to silent drift

**File:** `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go:32-59`
**Issue:** `semanticToolNames` (9 entries), `diagnosticToolNames` (1 entry), and
`readToolNames` (2 entries) are hard-coded string sets that must stay in lockstep with
the kernel tool registrations in `internal/kernel/{symbols,diag,fileops}`. The comments
claim "anchor on the registry, do not hard-code a stale list" (A4), but the code does
exactly hard-code a list — there is no compile-time or test-time assertion tying these
sets to the live registry. If a symbol tool is renamed/added (the registry is 53 tools
and documented as auto-generated), these metrics silently under-count with no failure.
This is the recurring drift class already burned the team once (see MEMORY: "helix tool
docs drift"). Not a correctness bug today, but a maintenance trap.
**Fix:** Add a test that cross-checks these sets against the actual registry inventory
(e.g. iterate the registered tool names for the symbols/diag/fileops packages and assert
the allowlists are a subset / fail on an unrecognized-but-expected name), so a rename
breaks a test instead of silently zeroing the metric.

### WR-05: `readToolNames` includes `list_dir`, inflating `files_read` and adding directory-listing bytes to `bytes_read`

**File:** `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go:56-59,107-110`
**Issue:** `files_read` is incremented for both `read_file` and `list_dir`, and
`bytes_read += ev.ResultSizeBytes` for both. A directory listing is not a file read; its
result bytes are a directory enumeration, not file content. METRIC naming
(`files_read` / `bytes_read`) implies file content reads. Counting `list_dir` toward
`files_read` over-counts the "files read" metric and pollutes `bytes_read` with listing
payload sizes. This may be intentional ("read-ish operations"), but it is not what the
metric name claims.
**Fix:** Either drop `list_dir` from `readToolNames` (count only `read_file` toward
files_read/bytes_read), or split it into a distinct counter; document the chosen
semantics in METRICS.md.

## Low

### LO-01: dual-home `tokens_input`/`tokens_output` can disagree within one document

**File:** `bench/runtime/result.go:42-44,92-93` + `bench/runtime/cell.go:514-524`
**Issue:** A result doc carries top-level `tokens_input`/`tokens_output` (always written,
defaulting to int 0 because RunCell never sets `ResultInput.TokensInput`) *and*
`metrics.tokens_input`/`metrics.tokens_output` (null for scripted runs). So a scripted
result emits `"tokens_input": 0` at top level and `"metrics": {"tokens_input": null}` —
two homes that disagree (0 vs null) for the same quantity in the same document. The
schema permits both (top-level relaxed to nullable, metrics canonical), and the
relaxation comment frames metrics as canonical, but a naive consumer reading the
top-level field gets a fabricated 0 for a run with no token data — the exact "never
fabricate 0" hazard METRIC-03 fights, just at the legacy top-level field.
**Fix:** Since the top-level columns are now nullable, prefer leaving them null when no
token data exists (make `ResultInput.TokensInput/Output` pointers, or copy the metrics
nulls into them) so the two homes agree; or document loudly that the top-level columns
are deprecated and consumers must read `metrics.*`.

### LO-02: `git`/`go` PATH-resolution failure surfaces as a per-metric MetricError rather than an infra error

**File:** `bench/evaluators/patch_validator/patch_validator.go:139-152`
**Issue:** `gitLines` returns any non-cancellation `cmd.Output()` error (including "git
not found on PATH" or "not a git repo") as a generic error, which the coordinator
records as a per-metric MetricError and continues (D-07). A missing `git` binary is an
infrastructure failure that will null *both* patch metrics on *every* cell, silently,
rather than failing the run loudly. The doc acknowledges this is intentional
("surface it as a MetricError rather than silently undercount"), so this is a design
note, not a bug — but environment-class failures (git absent) are arguably distinct from
metric-class failures (this repo has no diff) and conflating them hides a broken host.
**Fix:** Optionally distinguish `exec.ErrNotFound` / non-repo errors from a clean
non-zero exit and treat the former as infra (or at least emit a distinct, greppable
reason string), so an operator can tell "no git on host" from "no changes to score".

### LO-03: `EditDistancePatch` silently swallows malformed numstat lines as binary files

**File:** `bench/evaluators/patch_validator/patch_validator.go:111-115`
**Issue:** `if aerr != nil || derr != nil { continue }` lumps two cases together: a
genuine binary file (numstat `-\t-\t<path>`, correctly contributing 0) and a malformed /
unexpected numstat line (a parse failure that ought never happen). Both are silently
skipped. If git's numstat format ever shifts or an unexpected line appears, the edit
distance silently under-counts with no signal. Low risk (numstat format is stable), but
the conflation means a real parse anomaly is indistinguishable from an expected binary
entry.
**Fix:** Special-case the binary sentinel explicitly (`fields[0] == "-" && fields[1] ==
"-"` → contribute 0) and treat any *other* non-integer field as a parse anomaly worth a
MetricError or at least a logged warning, rather than silently dropping it.

---

_Reviewed: 2026-06-18T13:06:36Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
