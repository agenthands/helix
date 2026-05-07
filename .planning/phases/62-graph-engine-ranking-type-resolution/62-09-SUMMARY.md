---
phase: 62-graph-engine-ranking-type-resolution
plan: 09
subsystem: live-handler / graph-repair
tags: [handler, post-commit, file-fact-diff, cross-phase-bridge, gap-closure, observability, tdd]
gap_closure: true
requires:
  - "internal/semantic/graph (FileFactDiff, GraphRepair, ComputeGraphRepair, GraphEdge, SymbolDiff)"
  - "internal/semantic/live/handler.OverlayWriter / OverlayTx (existing tx span)"
  - "internal/semantic/live/handler.RankApplier (Phase 62 P02 hook surface)"
provides:
  - "internal/semantic/live/handler.FileFactDiffRecorder (exported tx-scoped recorder seam)"
  - "Handler.emptyDiffOnce (per-workspace once-gated INFO log)"
  - "ROADMAP cross-phase obligation linking Phase 60 P04 + future Phase 62 type-resolver retrofit to the recorder seam"
affects:
  - "Future Phase 60 P04: full FileFact upsert MUST populate the recorder via RecordSymbol{Removed,Changed,Added}"
  - "Any future Phase 62 type-resolver live-edge retrofit: MUST populate the recorder via RecordEdge{Added,Removed}"
tech-stack:
  added:
    - "log/slog handler test plumbing (recordingSlogHandler) reused for once-INFO assertions"
  patterns:
    - "Per-workspace sync.Once gate via sync.Map[repoID]*sync.Once"
    - "Nil-receiver-safe record methods (matches existing handler nil-safe pattern)"
    - "Test-only seam via export_test.go (production field stays unexported)"
key-files:
  created:
    - "internal/semantic/live/handler/recorder_test.go (3 new tests)"
    - "internal/semantic/live/handler/export_test.go (SetPopulateRecorderForTest seam)"
  modified:
    - "internal/semantic/live/handler/handler.go (FileFactDiffRecorder type + Handler fields + updateChangedFileWithKind rewrite)"
    - ".planning/ROADMAP.md (cross-phase notes under Phase 60 + Phase 62)"
decisions:
  - "Path B+ (structural hybrid): build the recorder seam now + observability + ROADMAP note. Path A (retrofit type-resolver edge emission into the live tx) was rejected because Phase 62 P05's dispatcher is not invoked from any handler-tx callsite today; building a live-edge retrofit would have pre-implemented future Phase 62 work outside this plan's scope."
  - "Recorder method names mirror SymbolDiff bit-flag terminology (RecordSymbolChanged carrying SignatureChanged/ExportedChanged/etc.) so populator call sites read clearly."
  - "populateRecorderForTest is unexported on the Handler struct; the only writer is SetPopulateRecorderForTest in export_test.go. Production paths leave the field nil."
  - "Empty-diff log fires INFO (not WARN) — it documents the deferred state, not a failure. Once-gated per workspace per Handler instance via sync.Map[repoID]*sync.Once."
  - "D-06 single BumpGraphVersion call site preserved: production tally is exactly 1 (apply_repair.go); the recorder seam only feeds the existing ApplyRepair pipeline."
metrics:
  duration: "~30 minutes"
  tasks: 2
  files_changed: 4
  completed: 2026-05-07
---

# Phase 62 Plan 09: Gap Closure for Verification Truth #22 (FileFactDiffRecorder Seam)

Closed 62-VERIFICATION.md gap truth #22 (BLOCKER) — handler `UpdateChangedFile` fired ApplyRepair on an unconditionally empty FileFactDiff in production — by introducing the exported `FileFactDiffRecorder` seam future populators MUST write through, plus per-workspace once-INFO observability that surfaces the empty-diff short-circuit to operators.

## Tasks Completed

### Task 1 — RED: failing tests for recorder seam + empty-diff once-INFO (commit `3fba57f9`)

Added three tests to `internal/semantic/live/handler/recorder_test.go` plus `export_test.go` exposing `SetPopulateRecorderForTest`:

- **`TestUpdateChangedFile_RecorderSeamExists`** — pins the `FileFactDiffRecorder` shape: 5 record methods (one per `FileFactDiff` slice variant), `Snapshot`, `IsEmpty`. Compile-time signature assertions plus a runtime assertion that `RecordSymbolChanged` flows through `Snapshot`.
- **`TestUpdateChangedFile_PopulatedDiffFiresApplyRepair`** — locks the contract future Phase 60 P04 / type-resolver retrofit populators write through: when the recorder is populated with a graph-changing `SymbolDiff{ExportedChanged: true}`, `ApplyRepair` fires exactly once with a non-empty `GraphRepair` and `DirtyNodes` is non-empty.
- **`TestUpdateChangedFile_EmptyDiffShortCircuits_OnceInfo`** — locks today's production behaviour: empty recorder → `ApplyRepair` NOT called → once-INFO log fires exactly once per workspace per Handler instance. Drives `repo-A` twice (1 log) then `repo-B` once (2 logs total).

RED state confirmed via `go test`:
```
internal/semantic/live/handler/export_test.go:13:54: undefined: FileFactDiffRecorder
internal/semantic/live/handler/export_test.go:17:4: h.populateRecorderForTest undefined
```

### Task 2 — GREEN: recorder type + handler wiring + once-INFO + ROADMAP note (commits `9148aa33` + this commit)

**`internal/semantic/live/handler/handler.go` changes:**

1. Added `"sync"` import.
2. New exported `FileFactDiffRecorder` type with five nil-receiver-safe record methods (`RecordSymbolRemoved/Changed/Added`, `RecordEdgeAdded/Removed`), `Snapshot()` returning `graphpkg.FileFactDiff`, and `IsEmpty()`. Tx-scoped concurrency contract documented (single-goroutine ownership matching `graphpkg.nodeSet` contract).
3. `Handler` struct gained `emptyDiffOnces sync.Map` (key: `repoID` string, value `*sync.Once`) and the unexported `populateRecorderForTest func(*FileFactDiffRecorder)` test seam.
4. New `(*Handler).emptyDiffOnce(repoID, fn)` helper using `LoadOrStore` to fire `fn` exactly once per repoID per Handler instance.
5. Rewrote `updateChangedFileWithKind`: constructs `recorder := &FileFactDiffRecorder{}` after `BeginOverlayTx` succeeds, invokes `h.populateRecorderForTest(recorder)` if non-nil (production: nil), commits the tx, then branches on `recorder.IsEmpty()`:
   - **Empty path:** invokes `h.emptyDiffOnce(string(repoID), func() { h.Logger.Info(...) })` with attrs `repo_id`, `phase_dependency=60-P04`, `see=62-VERIFICATION.md truth #22; closure 62-09-PLAN.md`.
   - **Non-empty path:** snapshots → `graphpkg.ComputeGraphRepair(diff)` → `ApplyRepair` if `!repair.IsEmpty()` (existing logic preserved).
6. Removed the dead `var diff graphpkg.FileFactDiff` declaration; the cross-phase TODOs migrated to load-bearing comment block at the recorder hook site (`TODO(phase-60-p04)` and `TODO(phase-62-future)` anchors retained).

**`.planning/ROADMAP.md` changes:** see ORCHESTRATOR ACTION REQUIRED section below for the exact edits to re-apply on main.

**Test results:**

| Suite | Command | Result |
| ----- | ------- | ------ |
| New 3 tests, count=10 | `go test ./internal/semantic/live/handler/... -count=10 -run TestUpdateChangedFile_` | ok 0.955s |
| Handler suite, race | `go test ./internal/semantic/live/handler/... -race -count=1` | ok 2.227s |
| Graph regression | `go test ./internal/semantic/graph/... -count=1` | ok 0.577s |
| Whole-internal vet | `go vet ./internal/...` | clean |

D-06 single-bump invariant preserved — `grep -rE "BumpGraphVersion" internal/semantic/graph/ --include='*.go' \| grep -v _test.go` shows exactly one production call site (`apply_repair.go: tx.BumpGraphVersion(ctx)`); the rest are interface declaration + doc-comment references.

## Behavioral Summary (the truth #22 closure)

**Before (production):**
```go
var diff graphpkg.FileFactDiff   // unconditionally empty
if h.rankApplier != nil {
    repair := graphpkg.ComputeGraphRepair(diff)   // empty → empty repair
    if !repair.IsEmpty() {                          // always false
        h.rankApplier.ApplyRepair(...)             // dead path in production
    }
}
```
Effect: `ApplyRepair` never fired in production for `UpdateChangedFile`. `graph_version` advanced only via the (currently absent) bulk-update path. Truth #22 BLOCKER.

**After (production, no populators wired yet):**
```go
recorder := &FileFactDiffRecorder{}
// ... tx.Commit ...
if h.rankApplier != nil {
    if recorder.IsEmpty() {
        h.emptyDiffOnce(repoID, func() {
            h.Logger.Info("live FileFactDiff is empty; ApplyRepair short-circuited ...")
        })
    } else { /* snapshot → ComputeGraphRepair → ApplyRepair */ }
}
```
Effect: ApplyRepair still does not fire (matching today's production reality), BUT operators see a single INFO log per workspace surfacing the deferred state. Phase 60 P04 + any future Phase 62 type-resolver retrofit just call `recorder.RecordSymbolChanged(...)` / `recorder.RecordEdgeAdded(...)` from inside the tx span and the populated path takes over automatically.

## Spec for Future Populator Phases

**Phase 60 P04 (full FileFact upsert):**
- Within `OverlayTx.UpsertOverlayFile` (or its successor full-FileFact variant), compute `SymbolDiff` against the prior file fact.
- For each removed symbol: `recorder.RecordSymbolRemoved(graphpkg.SymbolDiff{NodeID: …})`.
- For each changed symbol: `recorder.RecordSymbolChanged(graphpkg.SymbolDiff{NodeID: …, SignatureChanged|ExportedChanged|KindChanged|StableKeyChanged|BodyOnlyChanged})`.
- For each added symbol: `recorder.RecordSymbolAdded(graphpkg.SymbolDiff{NodeID: …})`.
- Plumb the recorder into the `OverlayTx` surface OR pass it as a third argument to `UpsertOverlayFile` (TBD by P04 design).
- **Acceptance:** P04's tests should remove or update the `EmptyDiffShortCircuits_OnceInfo` test in this plan, since the empty path will no longer be the production reality.

**Future Phase 62 type-resolver live-edge retrofit:**
- After symbol diff is computed in P04, invoke `dispatcher.ResolveChain(file)` and feed the resulting `[]GraphEdge`s through `recorder.RecordEdgeAdded(...)` / `recorder.RecordEdgeRemoved(...)`.
- Required only when "ApplyRepair fires on graph-changing edits in production" becomes the acceptance criterion; until then, P04 alone is sufficient to close truth #22's main assertion.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Doc accuracy] FileFactDiff.IsEmpty() does not exist**
- **Found during:** Task 2 GREEN write-up of `FileFactDiffRecorder.IsEmpty()` doc.
- **Issue:** The plan's `<interfaces>` block claimed `IsEmpty` had "identical semantics to graphpkg.FileFactDiff.IsEmpty after Snapshot." Verified `internal/semantic/graph/repair.go` and `FileFactDiff` has no `IsEmpty` method (only `GraphRepair.IsEmpty` exists).
- **Fix:** Adjusted the `IsEmpty` doc on `FileFactDiffRecorder` to reference the downstream short-circuit chain (`ComputeGraphRepair` → empty `GraphRepair` → D-06 short-circuit) rather than a non-existent method.
- **Files modified:** `internal/semantic/live/handler/handler.go` (doc only).
- **Commit:** included in `9148aa33`.

**2. [Rule 3 — Build dep] Test seam needed for unexported field**
- **Issue:** The plan called for an unexported `populateRecorderForTest` field on `Handler` written-to from `handler_test.go` (different package: `handler_test`). Cross-package tests cannot write unexported fields.
- **Fix:** Created `internal/semantic/live/handler/export_test.go` (in-package test file, not shipped in production builds) with `SetPopulateRecorderForTest(*Handler, func(*FileFactDiffRecorder))` as the package-internal setter. The test file (`recorder_test.go`) calls `handler.SetPopulateRecorderForTest(h, populator)`.
- **Files modified:** `internal/semantic/live/handler/export_test.go` (new), `internal/semantic/live/handler/recorder_test.go` (uses the setter).
- **Commit:** included in `3fba57f9` (RED).

**3. [Rule 3 — Build dep] Logger interface mismatch with slog.Handler**
- **Issue:** The plan's empty-diff test sketch wrote `h.Logger = slog.New(rh)` directly. `Handler.Logger` is the package-internal `Logger` interface (`Warn(msg string, args ...any)` / `Info(msg string, args ...any)`), not `*slog.Logger`. Direct assignment fails.
- **Fix:** Added a small `slogLoggerAdapter` in `recorder_test.go` wrapping `*slog.Logger` to satisfy the handler's `Logger` interface. Threaded it through `newRecorderTestHandler(t, applier, populator, slogHandler)`.
- **Files modified:** `internal/semantic/live/handler/recorder_test.go` (test plumbing only).
- **Commit:** included in `3fba57f9` (RED).

### Auth Gates

None.

### Architectural Decisions Surfaced

None — Path B+ (structural hybrid: seam + observability + ROADMAP) was the planned approach and remained the executed approach.

## Self-Check: PASSED

**Files:**
- `internal/semantic/live/handler/handler.go` — FOUND, modified (FileFactDiffRecorder + handler wiring + once-INFO).
- `internal/semantic/live/handler/recorder_test.go` — FOUND (3 new tests).
- `internal/semantic/live/handler/export_test.go` — FOUND (SetPopulateRecorderForTest seam).
- `.planning/ROADMAP.md` — FOUND, modified (cross-phase notes at lines 145 and 161).
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-09-SUMMARY.md` — FOUND (this file).

**Commits (verified via `git log --oneline`):**
- `3fba57f9` — `test(62-09): add failing tests for FileFactDiffRecorder seam + empty-diff once-INFO (truth #22)` — FOUND
- `9148aa33` — `feat(62-09): add FileFactDiffRecorder seam + empty-diff once-INFO observability (truth #22)` — FOUND
- (this commit) — `docs(62-09): cross-phase carry-forward note + plan summary` — pending

**Acceptance grep checks:**
- `grep -nE "type FileFactDiffRecorder struct" internal/semantic/live/handler/handler.go` → 1 line (line 58). PASS.
- `grep -nE "var diff graphpkg.FileFactDiff" internal/semantic/live/handler/handler.go` → 0 lines (dead pattern eliminated). PASS.
- `grep -cE "TODO\(phase-60-p04\)|TODO\(phase-62-future\)" internal/semantic/live/handler/handler.go` → 2. PASS.
- `grep -nE "62-09 closure|FileFactDiffRecorder" .planning/ROADMAP.md` → 2 lines. PASS.
- D-06 single BumpGraphVersion call site preserved (only `apply_repair.go: tx.BumpGraphVersion(ctx)` is a real call). PASS.

## ORCHESTRATOR ACTION REQUIRED

The post-wave merge step in `execute-phase.md` restores `.planning/ROADMAP.md` from a backup (orchestrator-owned file protection at #1756), so the worktree's ROADMAP edits are not preserved on main. The orchestrator MUST re-apply the following two edits on main after merge.

### Edit 1 — Phase 60 entry (cross-phase obligation note)

**File:** `.planning/ROADMAP.md`

**Locator (anchor that must remain unchanged):**
```
- [x] Phase 60: Live Update Pipeline (0/6 plans) (completed 2026-05-05)
  Plans:
```

**Insert this NEW line BETWEEN the existing two anchor lines (i.e., immediately after `- [x] Phase 60: Live Update Pipeline (0/6 plans) (completed 2026-05-05)` and immediately before `  Plans:`):**

```
  > **Phase 60 P04 obligation (from 62-09 closure):** the full FileFact upsert MUST record SymbolDiff entries via `internal/semantic/live/handler.FileFactDiffRecorder.RecordSymbol*` so Phase 62's post-commit `ApplyRepair` hook fires productively in production. Until P04 lands, `handler.UpdateChangedFile` emits a once-INFO log per workspace surfacing the empty-diff short-circuit (62-VERIFICATION.md truth #22).
```

**Final shape after edit:**
```
- [x] Phase 60: Live Update Pipeline (0/6 plans) (completed 2026-05-05)
  > **Phase 60 P04 obligation (from 62-09 closure):** the full FileFact upsert MUST record SymbolDiff entries via `internal/semantic/live/handler.FileFactDiffRecorder.RecordSymbol*` so Phase 62's post-commit `ApplyRepair` hook fires productively in production. Until P04 lands, `handler.UpdateChangedFile` emits a once-INFO log per workspace surfacing the empty-diff short-circuit (62-VERIFICATION.md truth #22).
  Plans:
```

### Edit 2 — Phase 62 entry (cross-phase carry-forward note)

**File:** `.planning/ROADMAP.md`

**Locator (anchor that must remain unchanged):**
```
- [x] Phase 62: Graph Engine, Ranking & Type Resolution (5/5 plans)
  Plans:
```

**Insert this NEW line BETWEEN the existing two anchor lines:**

```
  > **Cross-phase carry-forward (62-09 closure):** the live handler now threads a `FileFactDiffRecorder` through every overlay tx; populators in Phase 60 P04 (full FileFact upsert) and any future Phase 62 type-resolver live-edge retrofit MUST write through `internal/semantic/live/handler.FileFactDiffRecorder` so the post-commit `ApplyRepair` hook surfaces graph-changing edits in production. Until those populators land, the empty-diff once-INFO log per workspace at `helix.live.handler` surfaces the gap. Phase 60 entry above carries the populator obligation.
```

**Final shape after edit:**
```
- [x] Phase 62: Graph Engine, Ranking & Type Resolution (5/5 plans)
  > **Cross-phase carry-forward (62-09 closure):** the live handler now threads a `FileFactDiffRecorder` through every overlay tx; populators in Phase 60 P04 (full FileFact upsert) and any future Phase 62 type-resolver live-edge retrofit MUST write through `internal/semantic/live/handler.FileFactDiffRecorder` so the post-commit `ApplyRepair` hook surfaces graph-changing edits in production. Until those populators land, the empty-diff once-INFO log per workspace at `helix.live.handler` surfaces the gap. Phase 60 entry above carries the populator obligation.
  Plans:
```

### Sanity check after re-apply

After applying both edits on main, the orchestrator can verify with:
```bash
grep -nE "62-09 closure|FileFactDiffRecorder" .planning/ROADMAP.md
```
Expected: exactly 2 matching lines (one under Phase 60, one under Phase 62).
