---
phase: 60-live-update-pipeline
reviewed: 2026-05-05T00:00:00Z
depth: standard
files_reviewed: 59
files_reviewed_list:
  - cmd/vet-nokernel2semantic/main.go
  - internal/config/defaults.go
  - internal/config/loader_test.go
  - internal/daemon/daemon.go
  - internal/daemon/live_wiring.go
  - internal/daemon/live_e2e_test.go
  - internal/kernel/edit/notifier_integration_test.go
  - internal/kernel/edit/tools.go
  - internal/kernel/fileops/notifier_integration_test.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/fileops/write.go
  - internal/kernel/kernel.go
  - internal/kernel/notifier_test.go
  - internal/kernel/notifier.go
  - internal/lint/nokernel2semantic/analyzer_test.go
  - internal/lint/nokernel2semantic/analyzer.go
  - internal/lint/nokernel2semantic/realtree_integration_test.go
  - internal/obs/metrics_labels_test.go
  - internal/obs/metrics_test.go
  - internal/obs/metrics.go
  - internal/phasegraph/pipelines/live.go
  - internal/phasegraph/pipelines/pipelines_test.go
  - internal/semantic/config.go
  - internal/semantic/live/classifier_test.go
  - internal/semantic/live/classifier.go
  - internal/semantic/live/coalescer/bulk_test.go
  - internal/semantic/live/coalescer/coalesce_test.go
  - internal/semantic/live/coalescer/coalesce.go
  - internal/semantic/live/coalescer/coalescer_test.go
  - internal/semantic/live/coalescer/coalescer.go
  - internal/semantic/live/handler/handler.go
  - internal/semantic/live/handler/noop_test.go
  - internal/semantic/live/lspqueue/queue_test.go
  - internal/semantic/live/lspqueue/queue.go
  - internal/semantic/live/scanner/manager.go
  - internal/semantic/live/scanner/scanner_integration_test.go
  - internal/semantic/live/scanner/scanner.go
  - internal/semantic/live/scanner/walk.go
  - internal/semantic/live/service/service_test.go
  - internal/semantic/live/service/service.go
  - internal/semantic/live/signal.go
  - internal/semantic/live/watcher/editor_fixtures_test.go
  - internal/semantic/live/watcher/enospc_test.go
  - internal/semantic/live/watcher/enospc.go
  - internal/semantic/live/watcher/manager.go
  - internal/semantic/live/watcher/status.go
  - internal/semantic/live/watcher/watcher_test.go
  - internal/semantic/live/watcher/watcher.go
  - internal/semantic/scheduler/scheduler_incremental_test.go
  - internal/semantic/scheduler/scheduler.go
  - internal/semantic/store/duckdb.go
  - internal/semantic/store/migrations_registry.go
  - internal/semantic/store/migrations_test.go
  - internal/semantic/store/migrations_types.go
  - internal/semantic/store/migrations.go
  - internal/semantic/store/overlay_concurrent_test.go
  - internal/semantic/store/overlay_test.go
  - internal/semantic/store/overlay.go
  - internal/semantic/types.go
  - Makefile
findings:
  critical: 4
  warning: 7
  info: 6
  total: 17
status: issues_found
---

# Phase 60: Code Review Report

**Reviewed:** 2026-05-05T00:00:00Z
**Depth:** standard
**Files Reviewed:** 59
**Status:** issues_found

## Summary

Phase 60 ships the live-update pipeline shape end-to-end (kernel notifier seam, classifier, coalescer, handler, service, watcher, scanner, schema v3, daemon wiring) and the architectural invariants the brief calls out are mostly honoured: the `nokernel2semantic` analyzer is correctly scoped, schema v3 epoch monotonicity is implemented as documented (s.db not tx), `EditNotifier.OnEdit` is wired fire-and-forget at every kernel edit site with the error explicitly discarded, `Coalescer.Enqueue` uses `select+default`, and the scanner walker rejects symlinks via `d.Type()&fs.ModeSymlink`.

However the review surfaced four BLOCKER-level defects that undermine operability, threat-model claims, and the brief's invariants:

1. The `helix_semantic_live_updates_total` counter — the central operability gate the brief explicitly calls out (invariant #4: "the `helix_semantic_live_updates_total{kind,outcome=dropped}` counter records drops") — has zero call sites in the live pipeline. The metric, its helper, its drop-on-unknown enforcement, and three unit tests all exist, but neither the coalescer (drop site), the handler (applied/error site), nor the watcher (queue full) ever invokes `metrics.SemanticLiveUpdatesInc`. Operators cannot observe drops, applies, errors, or no-ops. Phase 60-05B's verification gate cannot be satisfied.

2. The watcher does NOT honor invariant #6 ("both watcher and scanner must skip symlinks"). The scanner's `Walk` correctly tests `d.Type()&fs.ModeSymlink`; the watcher's `handleEvent` and `addRecursive` have no symlink check anywhere. A `Create` on a symlinked subdirectory is `os.Stat`'d (not `Lstat`), follows the link, and `fw.Add()`s the target — outside the workspace. T-60-04-03 mitigation is incomplete; only the classifier's defense-in-depth Lstat catches the resulting events.

3. `BuildLiveUpdatePhases` (the Phase 60 D-06 phase-graph validator that is supposed to fail bootstrap when a wired component is missing) is never invoked from `internal/daemon/daemon.go`. The 9-phase constructor exists, has full tests asserting it errors on missing components, but no callsite in the daemon. A regression that drops `kernel.SetEditNotifier` or `scheduler.SetIncrementalHandler` will NOT be caught — the validator the plan promised is dead wiring.

4. `liveBundle.lspQueue` is constructed (`lspqueue.New(1024)`) but never enqueued into and never exposed. The queue declared by Phase 60 P04 has no producer in this phase. Combined with the missing metric helper, an operator inspecting `helix_semantic_live_lspqueue_depth` would see a permanently-zero value with no path to non-zero — and Phase 61's worker would consume an empty channel forever.

The 7 warnings cover finding-classification mis-emission, schema-version readback ordering reliance, time.AfterFunc race, an opaque non-monotone fall-through in the merge table, and four duplication / test-helper concerns. Six info items capture minor improvements.

## Critical Issues

### CR-01: helix_semantic_live_updates_total counter is never emitted

**File:** `internal/semantic/live/coalescer/coalescer.go:107-118`, `internal/semantic/live/coalescer/coalescer.go:181-213`, `internal/semantic/live/handler/handler.go:98-177`, `internal/semantic/live/watcher/watcher.go:255-285`, `internal/obs/metrics.go:436-466`
**Issue:** The `Metrics.SemanticLiveUpdatesInc(kind, outcome)` helper is declared, drop-tested, cardinality-tested, and label-allowlisted, but no production code path calls it. `grep -rn "SemanticLiveUpdatesInc" internal/` returns only the helper definition and its three unit tests in `internal/obs/`; no hits in `internal/semantic/live/...` or `internal/daemon/...`. Specific gaps:

- `Coalescer.Enqueue` (coalescer.go:113) drops on full queue but only logs and increments an atomic counter — no `outcome="dropped"` emission.
- `Coalescer.flush` dispatch loop (coalescer.go:205-211) never emits `outcome="applied"` on success or `outcome="error"` on `Dispatch` error.
- `Coalescer.flush` empty-batch short-circuit (coalescer.go:202-204) never emits `outcome="no_op"`.
- `Handler.Dispatch` and its branch helpers (handler.go:98-177) never emit anything for any kind.
- `workspaceWatcher.run` (watcher.go:151-189) drops events under ENOSPC and ignored-dir filters with no metric.

The brief explicitly names this as invariant #4 ("the `helix_semantic_live_updates_total{kind,outcome=dropped}` counter records drops"). The 60-05B SUMMARY presumably claims D-07 done, but `helix_semantic_live_updates_total` will be permanently zero in production. Operators cannot observe pipeline health; the `helix_semantic_live_updates_total{kind,outcome="dropped"}` SLO the brief implies cannot be alerted on.

**Fix:** Wire emission at every drop / apply / error site. Suggested seams:
```go
// internal/semantic/live/coalescer/coalescer.go around line 113
func (c *Coalescer) Enqueue(ev live.SourceChangeEvent) {
    select {
    case c.in <- ev:
    default:
        c.drops.Add(1)
        c.metrics.SemanticLiveUpdatesInc(string(ev.Kind), "dropped") // NEW
        c.logger.Warn("coalescer queue full; dropped event", ...)
    }
}

// internal/semantic/live/coalescer/coalescer.go around line 206
for _, ev := range merged {
    if err := c.handler.Dispatch(ctx, ev); err != nil {
        c.metrics.SemanticLiveUpdatesInc(string(ev.Kind), "error") // NEW
        c.logger.Warn("coalescer: dispatch error", ...)
    } else {
        c.metrics.SemanticLiveUpdatesInc(string(ev.Kind), "applied") // NEW
    }
}
```
Add a `Metrics` field to `Coalescer`/`Handler` (currently the `obs.Metrics` is plumbed into `buildLiveBundle` at `live_wiring.go:300` as `_ = metrics`, so the wiring already has the handle — just thread it through `coalescer.Config` / `handler.New`). Add an integration test that drives a full queue and asserts the dropped counter advances.

### CR-02: Watcher does not reject symlinks (invariant #6 violation)

**File:** `internal/semantic/live/watcher/watcher.go:105-131` (`addRecursive`), `internal/semantic/live/watcher/watcher.go:196-231` (`handleEvent`)
**Issue:** The brief's invariant #6 states "both watcher and scanner must skip symlinks (T-60-04-03 / T-60-05b-01)". The scanner correctly enforces this in `Walk` (scanner/walk.go:61: `if d.Type()&fs.ModeSymlink != 0 { return nil }`). The watcher does NOT.

- `addRecursive` (line 105) walks with `filepath.WalkDir` and only checks `d.IsDir()`. WalkDir reports the file's own type — a symlinked directory is reported with `fs.ModeSymlink|fs.ModeDir` bits — but the watcher does not test the symlink bit before calling `fw.Add(path)`. A symlinked directory under the workspace root will be added to fsnotify, watching wherever it points (potentially outside the workspace).
- `handleEvent` (line 226-230) processes Create events with `os.Stat` (not `os.Lstat`). When a `Create` event lands on a path that turns out to be a symlink to a directory, `os.Stat(ev.Name)` follows the link and returns the target's `info.IsDir() == true`, then `fw.Add(ev.Name)` registers the symlink (which is the link itself in fsnotify, but new files inside the linked target then surface).

Defense-in-depth at `live.ClassifyPathChange` (classifier.go:76-79) catches the symlink at classification time — but the watcher already pumped a real path event into the pending set, the directory might already be under watch, and a non-defensive operator could trust the watcher's own filtering.

**Fix:** Mirror the scanner's check inside `addRecursive` and `handleEvent`:
```go
// In addRecursive, before fw.Add(path):
if d.Type()&fs.ModeSymlink != 0 {
    return nil // drop symlink dirs at enumeration time
}

// In handleEvent, replace os.Stat with os.Lstat:
if info, err := os.Lstat(ev.Name); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
    _ = w.fw.Add(ev.Name)
}
```
Add a unit test that creates `wsroot/link → /tmp/elsewhere` and asserts `fw.Add` is not invoked on the link.

### CR-03: BuildLiveUpdatePhases (Phase 60 D-06 validator) is never invoked from the daemon

**File:** `internal/phasegraph/pipelines/live.go:116-183`, `internal/daemon/daemon.go` (entire file), `internal/daemon/live_wiring.go` (entire file)
**Issue:** Phase 60-05B introduced `BuildLiveUpdatePhases(LiveUpdateComponents)` as the wiring-validator that fails bootstrap when a required component (kernel.SetEditNotifier, OverlayStore, IncrementalHandler, lspqueue) is missing. Tests in `pipelines_test.go:65-123` assert the constructor errors on nil components. But `grep -rn "BuildLiveUpdatePhases\|LiveUpdateComponents" internal/daemon/ cmd/` returns ZERO matches — the daemon does NOT invoke `BuildLiveUpdatePhases` and does not build a `LiveUpdateComponents`. The validator is dead code.

A future regression that drops `k.SetEditNotifier(liveService)` from `live_wiring.go:180` (or that drops `sched.SetIncrementalHandler(h)` from `live_wiring.go:150`) will NOT be caught at bootstrap. The Phase 60 D-06 contract ("a missing component fails the bootstrap rather than silently dropping events" — pipelines/live.go:107-110) is unmet.

**Fix:** Invoke the validator from `Daemon.Run` (or from `buildLiveBundle`) before the daemon starts serving:
```go
// In live_wiring.go after the bundle is fully wired (after step 6 around line 207):
if err := runPhaseGraph(pipelines.BuildLiveUpdatePhases(pipelines.LiveUpdateComponents{
    EditNotifier:         liveService,
    OverlayStore:         store,
    IncrementalScheduler: h,
    LSPRevalidationQueue: bundle.lspQueue,
})); err != nil {
    logger.Error("live-update phase-graph validation failed", "err", err)
    return nil // or fail-fast — depends on D-07 policy
}
```
If the intentional choice is "ship the constructor today, wire the runner in v1.11", document it explicitly in 60-05b-SUMMARY.md and remove the verification gate's dead language ("`grep -v ... | grep -c 'noopRun'` returns 0 inside this constructor"). The current state is "wired enough to test, never invoked in production".

### CR-04: lspQueue is constructed but has no producer (Phase 60 P04 producer-side missing)

**File:** `internal/daemon/live_wiring.go:183` (`bundle := &liveBundle{service: liveService, lspQueue: lspqueue.New(1024)}`), `internal/semantic/live/lspqueue/queue.go:42-49` (Enqueue), `internal/semantic/live/handler/handler.go` (entire file)
**Issue:** `lspqueue.Queue.Enqueue` returns `bool` and is documented (queue.go:5-8) as "Phase 60 P04 declares the queue and exposes Enqueue (producer-only); Phase 61 wires a worker goroutine to drain via Channel()". But `grep -rn "lspQueue\|LSPRevalidationQueue\|\.Enqueue(" internal/semantic/live/` finds zero `lspQueue.Enqueue` callers. The `liveBundle.lspQueue` field is set but never read (the bundle struct has no accessor exposing it; the daemon stores the bundle but never reaches into the field).

The consequence is twofold:
1. The Phase 60 producer-side promise from `queue.go:5-8` ("Phase 60 P04 declares the queue and exposes Enqueue (producer-only)") is unfulfilled — Enqueue exists but no producer calls it.
2. When Phase 61's worker lands and reads from `q.Channel()`, the channel will be permanently empty because no upstream code ever feeds it.

If the design intent is "every successful overlay write enqueues a revalidation job", the handler's `UpdateChangedFile` (handler.go:115-129) is the natural seam — and it's missing.

**Fix:** Either delete the queue construction from `buildLiveBundle` until Phase 61 (and remove the `LSPRevalidationQueue any` field from `LiveUpdateComponents`), OR wire the producer side in this phase. If the latter, add an `Enqueue` call after `tx.Commit()` succeeds in `Handler.UpdateChangedFile`:
```go
// internal/semantic/live/handler/handler.go around line 128
if err := tx.Commit(); err != nil {
    return fmt.Errorf("UpdateChangedFile: commit: %w", err)
}
if h.LSPQueue != nil {
    h.LSPQueue.Enqueue(lspqueue.RevalidateFileJob{RepoID: repoID, Path: path})
}
return nil
```
Document the choice in 60-05b-SUMMARY.md. Either way, the current half-wired state ("queue exists, validator requires it non-nil, no producer feeds it, no consumer drains it") is the worst of both worlds.

## Warnings

### WR-01: replace_in_file no-workspace path emits outcome=internal but returns NoWorkspace error

**File:** `internal/kernel/fileops/tools.go:386-389`, also `tools.go:480-482` (fuzzy_edit), `tools.go:243-247` (create_file paths via `noWorkspaceError()`)
**Issue:** When `root == ""`, the handler sets `outcome = "internal"` on the deferred metric emit, but returns `noWorkspaceError()` which produces an `serr.NoWorkspace` error string. The `EditOutcomeInc` enum has no `no_workspace` bucket (deliberately, per `ClassifyEditError` Q-3 doc in `edit/tools.go:65-69`), so the outcome label is forced to `internal`. An operator looking at `helix_edit_outcome_total{tool=replace_in_file,outcome=internal}` will see the no-workspace condition and the kernel-imploded condition collapsed into one bucket — exactly the under-classification that Q-3 acknowledges as TODO(v1.3, WR-06). Unrelated to Phase 60 directly, but the 60-03 wiring inherited the conflation.
**Fix:** Documented under-classification — accept as v1.3 backlog. If Phase 60 wants to surface no-workspace cleanly, add `case "no_workspace":` to `EditOutcomeInc` and emit `outcome = "no_workspace"` from these branches. Otherwise leave a `// TODO(v1.3, WR-06)` marker at each of the four affected branches.

### WR-02: classifyExisting reads schema_version with LIMIT 1 / no ORDER BY (test brittleness)

**File:** `internal/semantic/store/duckdb.go:177` (`SELECT version FROM semantic_schema_version LIMIT 1`), `internal/semantic/store/migrations_test.go:204-241` (`TestMigration_ForwardIncompatible`)
**Issue:** `classifyExisting` uses `LIMIT 1` without `ORDER BY`. The test `TestMigration_ForwardIncompatible` injects a `version=99` row alongside `v1, v2, v3` rows and relies on DuckDB returning the rows in insertion order so that the LIMIT-1 read returns v=2 (≤ CurrentSchemaVersion=3, so `classifyExisting` returns clean), then `runMigrations` does `max(version)=99` and trips the forward-incompat guard. If DuckDB ever changes its scan order (HEAP rebalancing, row-group ordering), the test could non-deterministically read v=99 first and the test path becomes "classifyExisting flagged forward-incompat → quarantine" instead of "registry rejected forward-incompat". Behaviour-preserving for the user (both paths surface an error), but the test asserts on the registry-side error message and would silently change which assertion runs.
**Fix:** Either add `ORDER BY version` to make `classifyExisting` deterministic (and read `max(version)` so semantics are unambiguous), or change the test to use a fresh DB with only `v=99` injected. The former is cheap and aligns with `runMigrations.readSchemaVersion` (which already uses `max(version)`).

### WR-03: Coalescer.makeFlush time.AfterFunc may fire after Coalescer.Run exits

**File:** `internal/semantic/live/coalescer/coalescer.go:129-147` (Run) and 172-178 (timer creation)
**Issue:** `Run`'s ctx-cancellation branch (lines 134-141) Stops the timer and maxTimer under the mutex. But `time.AfterFunc` schedules `flush` on the runtime timer goroutine, and there's a window where `accept` (running on the input-channel goroutine, holding the mutex briefly to update pending) creates a fresh `time.AfterFunc(c.cfg.Debounce, flush)` AFTER ctx has been cancelled but BEFORE the next iteration of the Run select picks up `ctx.Done`. That AfterFunc will fire on the runtime timer goroutine after `Run` returns, calling `flush` against a cancelled ctx — which inside `Dispatch` may panic if the handler held a now-nil store reference, or at minimum produce a phantom dispatch with `ctx.Err() = context.Canceled`. The current handler doesn't crash on cancelled ctx, but defensive operators should not rely on that.
**Fix:** Hold the mutex across the entire ctx-Done shutdown sequence, OR use `sync.Once` to gate `flush` on a `closed` flag. Simpler: have `flush` short-circuit on `ctx.Err() != nil`:
```go
func (c *Coalescer) makeFlush(ctx context.Context) func() {
    return func() {
        if ctx.Err() != nil {
            return
        }
        // ... existing body
    }
}
```

### WR-04: MergeChange last-write-wins fallback is opaque and silently swallows non-monotone events

**File:** `internal/semantic/live/coalescer/coalesce.go:88-119`
**Issue:** The `default:` branch (line 115-117) returns `cur, true` for any unmapped (prev, cur) combination. This includes meaningful sequences the table doesn't enumerate:
- `deleted+modified` (the file came back via off-watcher route, then was re-edited) → returns `modified` (loses the deleted→modified resurrection signal)
- `rename+rename` (renamed twice) → returns the second rename (loses chain)
- `rename+deleted` (renamed then deleted) → returns deleted on the new path key but the old-path tombstone never fires
The doc comment SAYS "any other combo → last-write-wins (cur replaces prev)" but the merge-key is `OldPath→Path` for renames, so a `rename(a→b)` followed by `delete(b)` keys the rename under `a→b` and the delete under `b` — they don't actually meet in MergeChange. The "default" branch is reachable mostly when both events have the same path key but kinds the table doesn't pair (e.g., `deleted+modified`), and the table-based reader will assume it's exhaustive when it is not.
**Fix:** Either enumerate the missing pairs in the switch (preferred — the SPEC §16.2 should be exhaustive), or add a `t.Errorf("unexpected (%s,%s) combo reached fallback")` in tests that drives every (prev, cur) pair to surface gaps. Add a doc-comment table mapping each unhandled pair to its fallback semantic.

### WR-05: Watcher.run defers fw.Close while Manager.Stop also calls fw.Close — potential double-close path

**File:** `internal/semantic/live/watcher/watcher.go:151-154` (`defer w.fw.Close()` in run), `internal/semantic/live/watcher/watcher.go:316-324` (`close()` called from Manager.Stop), `internal/semantic/live/watcher/manager.go:144-152`
**Issue:** Two paths close `fw`:
1. `Manager.Stop` → `ww.close()` → `_ = w.fw.Close()`
2. `workspaceWatcher.run` → `defer func() { _ = w.fw.Close() }()`

When Stop is called, it closes fw, which causes the `<-w.fw.Events` channel to close, which causes `run` to return, which fires the deferred `fw.Close` again. fsnotify returns `ErrClosed` on the second call which is discarded. The doc comment at line 152 acknowledges this as idempotent. However, `close()` does NOT cancel the context, and the defer in `run` stops the per-workspace timer separately from `close()`. If timer.Stop in `close()` races the timer.Stop in run's ctx.Done branch (lines 162-165), one of them will see a nil timer (the `close()` path nil's the timer at line 320 but the run loop holds its own pointer that may already have been Stopped). Not a panic, but the timer-stopping bookkeeping is duplicated and racy.
**Fix:** Single owner. Either Manager.Stop only cancels the ctx (let run drive the cleanup via `defer`), or remove the `defer fw.Close()` from run (let Manager.Stop drive it). The latter is cleaner because Manager already has the lifecycle responsibility.

### WR-06: Scanner uses sync.Mutex around `changed` slice but the walk is single-threaded

**File:** `internal/semantic/live/scanner/scanner.go:130-152`
**Issue:** `scanOnce` declares `var mu sync.Mutex` and locks it around `changed = append(...)` inside the `Walk` callback, but `Walk` itself (walk.go:50 `filepath.WalkDir`) is single-threaded and `cfg.MaxParallelFiles` (scanner.go:81) is documented as "reserved for a future tunable; the current impl is sequential." The mutex is dead synchronization — extra cost, false signal that concurrent extension is supported.
**Fix:** Drop the mutex in the current sequential implementation, OR (preferable) implement the parallel-fanout the config field promises so the lock pays for itself.

### WR-07: noopMetricsLogger comment doesn't match noopLogAdapter type name

**File:** `internal/daemon/live_wiring.go:108-113`
**Issue:** The doc-comment block above the type says "noopMetricsLogger" but the actual type is named `noopLogAdapter`. A reviewer searching for "noopMetricsLogger" in IDE goto-symbol gets nothing. Trivial defect but it makes grep-driven code archaeology fail.
**Fix:** Rename the comment header to `// noopLogAdapter` or rename the type to match the comment. Choose one.

## Info

### IN-01: `_ = metrics` in buildLiveBundle is the dropping-ground for CR-01

**File:** `internal/daemon/live_wiring.go:209`
**Issue:** The `metrics *obs.Metrics` parameter to `buildLiveBundle` is plumbed all the way through, then explicitly discarded with `_ = metrics // metrics is reserved for future per-bundle wiring (counters live on *obs.Metrics)`. This is the single seam where the metric helper from CR-01 should be threaded into the coalescer and handler. The current discard makes the gap visible but unfixed.
**Fix:** Once CR-01 is addressed, replace this discard with `coalescer.Config{..., Metrics: metrics}` plumbing.

### IN-02: TestEditTools_OnEditNotCalledOnError uses brittle string-walking heuristic

**File:** `internal/kernel/edit/notifier_integration_test.go:217-257`, `internal/kernel/fileops/notifier_integration_test.go:177-208`
**Issue:** The test scans the source file with `strings.Index(rest, "_ = n.OnEdit(ctx,")` and asserts the next `return textResult(` lies before the next `}))` — a coarse heuristic that breaks if any non-success-branch contains both substrings or if the closure shape evolves. A future refactor that wraps the OnEdit in a helper or moves it outside the closure will silently pass this test while breaking the wiring contract.
**Fix:** Convert to a real handler-driven integration test that mocks the OnEdit hook and asserts:
1. OnEdit fires exactly once on the success path.
2. OnEdit does NOT fire on each named error path (missing path, missing symbol_name, etc.).
The existing live_e2e_test.go already shows the pattern; expand it to cover error branches.

### IN-03: SemanticLiveUpdatesInc allows kind="bulk_update" but no producer emits that kind today

**File:** `internal/semantic/live/coalescer/coalesce.go:64-70`, `internal/semantic/live/handler/handler.go:106-107`
**Issue:** The coalescer synthesizes `ChangeBulkUpdate` with `Source: changeSourceCoalescerInternal()`, the handler dispatches it via `HandleBulkUpdate`. Phase 60 ships the wiring but the test `TestBulkUpdateCollapse` only exercises the threshold collapse — no integration test confirms the synthetic event survives the dispatch chain.
**Fix:** Once CR-01 lands, add an integration test that floods the coalescer with >threshold events, asserts the handler sees a single `bulk_update` event, and asserts `helix_semantic_live_updates_total{kind=bulk_update,outcome=applied}` advances by 1.

### IN-04: ChangeFileRenamed has no producer in this phase

**File:** `internal/semantic/live/signal.go:80`, `internal/semantic/live/handler/handler.go:104-105`, `internal/semantic/live/coalescer/coalesce.go:111-114`
**Issue:** The `ChangeFileRenamed` enum value, the `OldPath` field on `SourceChangeEvent`, the rename branch of `MergeChange`, and `Handler.HandleFileRenamed` are all wired — but no producer (classifier, watcher, scanner) ever emits a rename. The fsnotify watcher fires Create+Remove pairs without correlating them into a rename. So the rename branches are exercised only by direct unit tests; no end-to-end path reaches them.
**Fix:** Either add a TODO marking "rename detection lands in 60-XX" and document the dead paths, or implement rename correlation in the watcher (combine Create+Remove with same content_hash within debounce window). The rename merge-key (`OldPath→Path`) is non-trivial enough that leaving it untested in the e2e flow risks bit-rot.

### IN-05: storeFileHashLookup always returns ("", false, nil) makes ChangeFileModified unreachable

**File:** `internal/daemon/live_wiring.go:82-90`
**Issue:** The production adapter `storeFileHashLookup.EffectiveContentHash` is hard-coded to return `("", false, nil)` (TODO(60-D-05) marker). This means `ClassifyPathChange` (classifier.go:86-95) ALWAYS returns `ChangeFileCreated` for fsnotify/scan events on existing files, never `ChangeFileModified` and never `ChangeFileDeleted`. The classifier's three-way decision is collapsed to two outcomes (created OR no-op). The doc comment acknowledges this as "label-only concern" but the live e2e test (live_e2e_test.go:163-166) still asserts `write_epoch >= 1` only — it doesn't pin which Kind reaches the handler, so the bug is silent.

Combined with CR-01 (no metric emission), the operator-facing impact is:
- Every file modification will appear in logs as "file_created" if logged.
- Once CR-01 is fixed, `helix_semantic_live_updates_total{kind="file_modified"}` will always be zero in production.
**Fix:** Document the limitation explicitly in the daemon log on bundle wiring. Add a regression test that drives a known file through the full pipeline and asserts the dispatched event's Kind. When Phase 62 lands `EffectiveContentHash`, this becomes a real fix.

### IN-06: scannerStoreLookup always returns empty map — same dead-classification pattern

**File:** `internal/daemon/live_wiring.go:98-106`
**Issue:** Mirror of IN-05 on the scanner side. `KnownFiles` returns an empty map, so the scanner's diff loop (scanner.go:147-153) treats every walked path as "not in known set, emit changed" — the scanner will emit a full-workspace "changed" set on EVERY scan cycle (default 10s) because nothing is ever known. Coupled with the bulk-collapse threshold (default 200), every scan in a workspace with >200 files synthesizes a single ChangeBulkUpdate, every 10s, forever.
**Fix:** Same as IN-05 — once Phase 59 snapshot writer populates rows, the scanner's KnownFiles adapter can read real data. Until then, document the runaway-bulk-update behaviour in 60-VALIDATION.md and consider a feature flag to disable the scanner in workspaces above N files (the existing `manifest_scan_enabled: true` default amplifies this).

---

_Reviewed: 2026-05-05_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
