---
phase: 80-five-of-six-ablation-runners-fairness-enforcement
fixed_at: 2026-06-19T00:00:00Z
review_path: .planning/phases/80-five-of-six-ablation-runners-fairness-enforcement/80-REVIEW.md
iteration: 1
findings_in_scope: 9
fixed: 7
skipped: 2
status: partial
---

# Phase 80: Code Review Fix Report

**Fixed at:** 2026-06-19
**Source review:** .planning/phases/80-five-of-six-ablation-runners-fairness-enforcement/80-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 9
- Fixed: 7 (WR-01, WR-02, WR-03, WR-04, WR-05, IN-02, IN-04)
- Skipped: 2 (IN-01, IN-03 — both intentional / out-of-scope by design)

**Verification (post-fix, full tree):**
- `go build ./cmd/helix` — PASS
- `go vet ./...` — PASS
- `go test ./bench/... ./cmd/helix-bench/...` (HELIX_BIN set) — PASS
- Stray `./helix` build artifact removed.

## Fixed Issues

### WR-01: `Sandbox.ResultPath`/`MergedTracePath` and `TestCellLayout` assert a durable layout RunCell no longer produces

**Files modified:** `bench/runtime/sandbox/sandbox.go`, `bench/runtime/cell_test.go`
**Commit:** 0ac7e9a7
**Applied fix:** Chose option (a). Verified via grep that `Sandbox.ResultPath`/`MergedTracePath` were called only by `TestCellLayout` (no production callers), then deleted both dead helpers (and the now-unused `path/filepath` import in sandbox.go) and rewrote `TestCellLayout` to assert against the production `cellDurablePaths(outDir, task, mode, 0)`, so the test now guards the real `<out>/<task>/<mode>/<run_index>/{result.v2.json,trace.json}` layout. Updated the package doc comment to reflect that durable paths are computed by `cellDurablePaths`.

### WR-02: `run_cmd_test.go` best-effort result assertion points at the wrong (run-index-less) path

**Files modified:** `cmd/helix-bench/run_cmd_test.go`
**Commit:** c95b1ae4
**Applied fix:** Added the `"0"` run-index segment to the `os.Stat` target at line 78 so it becomes `<runOutDir>/IT-go-patch-apply-1/your_agent_full/0/result.v2.json`, matching the correct sibling path in `TestRunSubcommandWiresDeltaPass`.

### WR-03: `fairnessBlock` iterates a map, producing non-deterministic `fairness.overrides[]` ordering

**Files modified:** `bench/runtime/result.go`
**Commit:** a3a7d731
**Applied fix:** Added `sort` import and a `sort.Slice(overrides, ... Mode < Mode)` call before returning from `fairnessBlock`, so projected overrides are emitted in deterministic mode order. This protects the repo's byte-identical-sha256 reproducibility guarantee once ≥2 overrides exist. Updated the doc comment to explain the sort rationale.

### WR-04: Stale `writeDurable` comment claims paths are *not* run-index keyed

**Files modified:** `bench/runtime/cell.go`
**Commit:** 8d39cd7c
**Applied fix:** Comment-only. Rewrote the `writeDurable` doc comment to state the path is keyed by `(task, mode, run_index)` via `cellDurablePaths` (so distinct cells never target the same path under single-rep ExpandMatrix), and reframed the atomic temp-file+rename as defense for a future repetition axis / re-run overwrite. No code behavior changed. Confirmed the change does not introduce the phrase guarded by `TestCellGoStaleComments`.

### WR-05: `discoverTasks` comment overstates the consequence of a leading-dot task dir

**Files modified:** `cmd/helix-bench/main.go`, `cmd/helix-bench/discover_tasks_test.go` (new)
**Commit:** 2b4e53ec
**Applied fix:** Added `TestDiscoverTasksSkipsHiddenDirs`, which seeds a `.hidden` dir alongside a real task dir under a temp datasets tree and asserts `discoverTasks` returns only the real task — locking the leading-dot filter against a regression that would convert a benign dir into a full-run abort. Also softened the overstated "would hard-fail" prose to describe the actual abort behavior the filter prevents. The filter behavior itself was not changed.

### IN-02: `deriveStoreOptIn` is a thin wrapper kept only for a test

**Files modified:** `bench/runtime/matrix.go`, `bench/runtime/store_isolation_test.go`
**Commit:** 204f8993
**Applied fix:** Removed the test-only `deriveStoreOptIn` wrapper and changed the single caller (`store_isolation_test.go`) to call `readTaskMeta(seedDir).storeOptIn()` directly (both are in package `runtime`). Updated the one comment reference in the test from `deriveStoreOptIn` to `meta.storeOptIn`. Verified the isolation test still compiles; it skips at runtime without a helix binary, which is unrelated to this change.

### IN-04: `iPtr`/`bPtr`/`fPtr` helper duplication across test files

**Files modified:** `bench/runtime/ptr_helpers_test.go` (new), `bench/runtime/result_test.go`, `bench/runtime/deltas_test.go`
**Commit:** 1d4320ff
**Applied fix:** Created `ptr_helpers_test.go` with the single canonical `iPtr`/`bPtr`/`fPtr` set, removed the scattered definitions from `result_test.go` and the divergent `intPtr`/`floatPtr` from `deltas_test.go`, and rewrote all `intPtr`/`floatPtr` call sites in `deltas_test.go` to `iPtr`/`fPtr`. Pure test-only cleanup; all call sites compile and the runtime test suite passes.

## Skipped Issues

### IN-01: `claudeMaxToolCalls` magic constant with no override path

**File:** `bench/runtime/cell.go:48-50`
**Reason:** skipped-by-design — out of scope per fix guardrails. The fix is explicitly deferred to "when the claude arm is completed" (a later phase); threading a config/flag now would be scope creep into the deferred claude path. The const is left as-is.
**Original issue:** `claudeMaxToolCalls = 20` is a hard-coded const bounding claude `--max-turns` with no flag/config override.

### IN-03: `contract_test.go` model_id assertion is structurally tautological

**File:** `bench/runners/contract_test.go:82-84`
**Reason:** skipped-by-design — the review itself states "No change required this phase." This is the intended Open-Q1 scope-A design (the live claude argv projects only `model_id` today; there is no per-mode ModelID path). The `model_id` assertion is left exactly as-is.
**Original issue:** The per-mode loop compares `DefaultContract.ModelID` against `wantModelID := DefaultContract.ModelID` (the package-level value to itself), so the per-mode assertion can never fail on drift — acknowledged as an intentional scope decision.

---

_Fixed: 2026-06-19_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
