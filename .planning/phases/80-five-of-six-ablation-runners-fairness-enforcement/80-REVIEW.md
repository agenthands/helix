---
phase: 80-five-of-six-ablation-runners-fairness-enforcement
reviewed: 2026-06-19T10:12:55Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - bench/BENCH.md
  - bench/runners/baseline_plain/MODE.md
  - bench/runners/baseline_rag/MODE.md
  - bench/runners/contract_test.go
  - bench/runners/mode_resolver_test.go
  - bench/runners/no_lsp/MODE.md
  - bench/runners/no_structured_edit/MODE.md
  - bench/runners/your_agent_no_semantic/MODE.md
  - bench/runtime/cell.go
  - bench/runtime/cell_test.go
  - bench/runtime/deltas.go
  - bench/runtime/deltas_test.go
  - bench/runtime/five_of_six_test.go
  - bench/runtime/matrix.go
  - bench/runtime/result.go
  - bench/runtime/result_test.go
  - bench/schema/result.v2.schema.json
  - cmd/helix-bench/main.go
  - cmd/helix-bench/run_cmd_test.go
findings:
  critical: 0
  warning: 5
  info: 4
  total: 9
status: issues_found
---

# Phase 80: Code Review Report

**Reviewed:** 2026-06-19T10:12:55Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Reviewed the five-of-six ablation matrix wiring: the new MODE.md arms, the
mode→profile resolver tests, the single-cell orchestrator (`cell.go`), the
matrix dispatcher (`matrix.go`), the 3-delta post-matrix pass (`deltas.go`),
the result.v2 builder + schema, and the `helix-bench run` subcommand.

The two intended asymmetric deferrals (`your_agent_no_semantic` emitting a
`guarantee_pending_phase_81` row; `baseline_rag` fail-closing with a distinct
`Deferred` outcome and no row) are implemented as specified and were not
flagged. The fairness gate is fail-closed (`Validate()` returns non-nil → RunCell
refuses to run), the path-traversal guards run before every join, and the delta
arithmetic is correct with proper null-skip semantics.

No BLOCKER-class correctness, security, or data-loss defects were found. The
findings below are quality/robustness defects: a now-stale durable-layout test
and a dead path assertion that disagree with the run-index-keyed layout RunCell
actually writes, a non-deterministic map iteration in `fairnessBlock` that
threatens the project's byte-reproducibility guarantee once overrides exist, and
several stale comments that misdescribe the current code.

## Warnings

### WR-01: `Sandbox.ResultPath`/`MergedTracePath` and `TestCellLayout` assert a durable layout RunCell no longer produces

**File:** `bench/runtime/sandbox/sandbox.go:59-69`, `bench/runtime/cell_test.go:39-55`
**Issue:** RunCell writes durable artifacts via `cellDurablePaths`, which threads
a `<run_index>` segment: `<out>/<task>/<mode>/<run_index>/result.v2.json`
(cell.go:182-189, 265). But `Sandbox.ResultPath`/`MergedTracePath` still return
the *old* run-index-less layout `<out>/<task>/<mode>/result.v2.json`. A grep
confirms these two sandbox helpers are dead in production — nothing outside the
test calls them. `TestCellLayout` (cell_test.go:39-55) asserts the layout via
those dead helpers, so it green-lights a path that RunCell never writes,
providing false confidence that the durable layout is correct. The test cannot
catch a regression in the real `cellDurablePaths` layout.
**Fix:** Either delete the unused `Sandbox.ResultPath`/`MergedTracePath` helpers
and rewrite `TestCellLayout` to assert against `cellDurablePaths(outDir, task,
mode, 0)`, or update both helpers to take a `runIndex` and emit the
`<run_index>` segment so they match production:
```go
func (s *Sandbox) ResultPath(task, mode string, runIndex int) string {
    return filepath.Join(s.outDir, task, mode, strconv.Itoa(runIndex), "result.v2.json")
}
```

### WR-02: `run_cmd_test.go` best-effort result assertion points at the wrong (run-index-less) path

**File:** `cmd/helix-bench/run_cmd_test.go:78-83`
**Issue:** `TestRunSubcommandWiresThemAll` stats
`<runOutDir>/IT-go-patch-apply-1/your_agent_full/result.v2.json` — without the
`/0/` run-index segment that RunCell actually writes
(`.../your_agent_full/0/result.v2.json`). The `os.Stat` therefore always misses
even when a helix binary is present and a real row was written, so the
"durable result.v2.json present" branch is dead code. The companion
`TestRunSubcommandWiresDeltaPass` (line 132) uses the correct `/0/` path,
confirming the discrepancy. The assertion silently logs "not written / wiring
still asserted" regardless of outcome, so it can never detect a broken durable
write.
**Fix:** Add the run-index segment to match the real layout:
```go
resultPath := filepath.Join(runOutDir, "IT-go-patch-apply-1", "your_agent_full", "0", "result.v2.json")
```

### WR-03: `fairnessBlock` iterates a map, producing non-deterministic `fairness.overrides[]` ordering

**File:** `bench/runtime/result.go:180-190`
**Issue:** `fairnessBlock` ranges over `fc.Overrides` (a
`map[string]ModeOverride`) and appends to the `overrides` slice in Go's
randomized map-iteration order. With ≥2 overrides, two builds of the same
contract emit `fairness.overrides[]` in different orders, so the result.v2.json
bytes are not reproducible. This directly undercuts the project's
byte-reproducibility guarantee (CLAUDE.md "Reproducibility … byte-identical
sha256s") and the schema's stated reproducibility intent. It is latent today only
because `DefaultContract.Overrides` is empty and `TestResultV2ValidWithOverrides`
exercises a single entry (order-insensitive), so no test catches it.
**Fix:** Sort the projected overrides by mode before returning:
```go
overrides := make([]resultOverride, 0, len(fc.Overrides))
for mode, ov := range fc.Overrides {
    overrides = append(overrides, resultOverride{Mode: mode, WaiverReason: ov.WaiverReason, ApprovedBy: ov.ApprovedBy})
}
sort.Slice(overrides, func(i, j int) bool { return overrides[i].Mode < overrides[j].Mode })
return resultFairness{Overrides: overrides}
```

### WR-04: Stale `writeDurable` comment claims paths are *not* run-index keyed, contradicting the actual layout

**File:** `bench/runtime/cell.go:708-717`
**Issue:** The `writeDurable` doc comment states "the durable artifact path is
keyed only by (task, mode) with no run-index segment (see the IN-05 comment
above), so two cells sharing an OutDir drive concurrent writes to the SAME path."
This is false: `cellDurablePaths` (cell.go:182-189) keys the path by `(task,
mode, run_index)`, and `Cell.RunIndex` is threaded through (cell.go:265,
matrix.go:266). Under the current single-rep `ExpandMatrix` (RunIndex always 0),
distinct `(task, mode)` cells already write distinct dirs, so the "concurrent
writes to the SAME path" scenario the comment justifies cannot occur. The
atomic temp-file+rename is still harmless, but the comment misdescribes the
concurrency model and will mislead a future maintainer reasoning about parallel
write safety. Note `TestCellGoStaleComments` (cell_test.go:304-314) guards
exactly one stale phrase ("absolute per-cell store path") — this stale comment is
a second instance the guard does not cover.
**Fix:** Rewrite the comment to reflect run-index keying, e.g. "the durable path
is keyed by (task, mode, run_index) so distinct cells never target the same path;
the atomic temp-file+rename is defensive against a future repetition axis or a
re-run overwriting an existing row, ensuring a concurrent reader never sees a
torn file."

### WR-05: `discoverTasks` comment overstates the consequence of a leading-dot task dir

**File:** `cmd/helix-bench/main.go:303-312`
**Issue:** The leading-dot skip comment asserts that "a leading-dot id is
rejected by ExpandMatrix's validateMatrixID, which would hard-fail the entire
expansion before any legitimate task runs." `validatePathSegment` does reject a
leading dot, and `ExpandMatrix` returns on the first bad id (matrix.go:139-143),
so a single `.DS_Store`-style dir surfacing into the task set would abort the
*whole* run — exactly the failure mode the comment warns about. The skip itself
is correct and defends against this; the concern is that the prose presents the
abort as a hypothetical ("would") when it is the actual behavior, and there is
no test asserting that a leading-dot dir in the dataset tree is skipped rather
than aborting the run. A regression that dropped the `!strings.HasPrefix(...,
".")` filter would silently convert a benign `.git` dir into a full-run abort
with no test to catch it.
**Fix:** Add a unit test for `discoverTasks` that seeds a `.hidden` dir alongside
a real task dir and asserts the hidden dir is skipped and the real task is
returned, locking the filter in place.

## Info

### IN-01: `claudeMaxToolCalls` magic constant with no override path

**File:** `bench/runtime/cell.go:48-50`
**Issue:** `claudeMaxToolCalls = 20` bounds the claude `--max-turns`. It is a
hard-coded const with no flag/config override; a multi-edit task driven by the
claude path would silently truncate at 20 turns. Acceptable for the
wired-not-gating claude path this phase, but worth surfacing as configurable when
the claude arm is finished in a later phase.
**Fix:** When the claude path is completed, thread the bound from
`CellConfig`/a run flag rather than a package const.

### IN-02: `deriveStoreOptIn` is a thin wrapper kept only for a test

**File:** `bench/runtime/matrix.go:320-324`
**Issue:** `deriveStoreOptIn` exists solely to preserve a standalone predicate
"used by store_isolation_test.go" (per its own comment); production
`runOneCell` calls `meta.storeOptIn()` directly. It re-reads `task.json` a second
time (via `readTaskMeta`) for the test's benefit. Minor dead-ish production
surface.
**Fix:** Consider having the isolation test call `readTaskMeta(seedDir).storeOptIn()`
directly and removing the wrapper, or document it as test-only with a clearer
name.

### IN-03: `contract_test.go` model_id assertion is structurally tautological

**File:** `bench/runners/contract_test.go:82-84`
**Issue:** Inside the per-mode loop the test compares `DefaultContract.ModelID`
against `wantModelID`, where `wantModelID := DefaultContract.ModelID` — i.e. it
compares the package-level value to itself, so the per-mode assertion can never
fail on drift. This is acknowledged in the file's own prose (there is no per-mode
ModelID override path), so it is an intentional scope decision per the phase
context, not a bug. Flagged only so a future reviewer adding a per-mode ModelID
path knows this loop currently asserts nothing mode-specific.
**Fix:** No change required this phase; when/if a per-mode ModelID projection is
added, replace the tautology with an assertion over the *projected* per-mode
value.

### IN-04: `iPtr`/`bPtr`/`fPtr` helper duplication across test files

**File:** `bench/runtime/result_test.go:28-29,123`, `bench/runtime/deltas_test.go:15-16`
**Issue:** `intPtr`/`floatPtr` (deltas_test.go) and `iPtr`/`bPtr`/`fPtr`
(result_test.go) are near-duplicate pointer-builder helpers in the same package
with divergent names. Minor maintainability smell; harmless.
**Fix:** Consolidate into one set of pointer helpers in a shared test file in the
`runtime` package.

---

_Reviewed: 2026-06-19T10:12:55Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
