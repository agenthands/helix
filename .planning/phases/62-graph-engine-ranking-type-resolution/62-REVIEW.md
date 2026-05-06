---
phase: 62-graph-engine-ranking-type-resolution
reviewed: 2026-05-06T00:00:00Z
depth: standard
files_reviewed: 80
files_reviewed_list:
  - internal/config/defaults.go
  - internal/config/loader_test.go
  - internal/daemon/daemon.go
  - internal/daemon/live_wiring.go
  - internal/daemon/rank_wiring.go
  - internal/daemon/type_resolver_wiring.go
  - internal/graph/options.go
  - internal/graph/pagerank.go
  - internal/graph/pagerank_test.go
  - internal/graph/personalize.go
  - internal/obs/metrics.go
  - internal/obs/metrics_labels_test.go
  - internal/repomap/pagerank.go
  - internal/semantic/cluster/doc.go
  - internal/semantic/cluster/persist.go
  - internal/semantic/cluster/persist_test.go
  - internal/semantic/cluster/weak.go
  - internal/semantic/cluster/weak_test.go
  - internal/semantic/config.go
  - internal/semantic/graph/apply_repair.go
  - internal/semantic/graph/apply_repair_test.go
  - internal/semantic/graph/doc.go
  - internal/semantic/graph/frontier.go
  - internal/semantic/graph/frontier_test.go
  - internal/semantic/graph/full_recompute.go
  - internal/semantic/graph/full_recompute_test.go
  - internal/semantic/graph/invalidations_consumer.go
  - internal/semantic/graph/invalidations_consumer_test.go
  - internal/semantic/graph/metrics.go
  - internal/semantic/graph/ranker.go
  - internal/semantic/graph/ranker_test.go
  - internal/semantic/graph/repair.go
  - internal/semantic/graph/repair_test.go
  - internal/semantic/graph/scheduler.go
  - internal/semantic/graph/scheduler_store.go
  - internal/semantic/graph/scheduler_test.go
  - internal/semantic/graph/status.go
  - internal/semantic/graph/status_test.go
  - internal/semantic/graph/trace.go
  - internal/semantic/graph/util.go
  - internal/semantic/live/handler/handler.go
  - internal/semantic/lspenrich/cascade.go
  - internal/semantic/lspenrich/cascade_integration_test.go
  - internal/semantic/lspenrich/cascade_merge_test.go
  - internal/semantic/lspenrich/cascade_overlay_epoch_test.go
  - internal/semantic/lspenrich/cascade_test.go
  - internal/semantic/lspenrich/integration_acceptance_test.go
  - internal/semantic/lspenrich/stress_test.go
  - internal/semantic/lspenrich/worker_test.go
  - internal/semantic/store/overlay.go
  - internal/semantic/store/overlay_test.go
  - internal/semantic/types/chain.go
  - internal/semantic/types/chain_test.go
  - internal/semantic/types/doc.go
  - internal/semantic/types/emit.go
  - internal/semantic/types/emit_test.go
  - internal/semantic/types/fixpoint.go
  - internal/semantic/types/fixpoint_test.go
  - internal/semantic/types/golang/comment.go
  - internal/semantic/types/golang/resolver.go
  - internal/semantic/types/golang/resolver_test.go
  - internal/semantic/types/golang/scope.go
  - internal/semantic/types/java/stub.go
  - internal/semantic/types/java/stub_test.go
  - internal/semantic/types/ladder.go
  - internal/semantic/types/ladder_test.go
  - internal/semantic/types/php/stub.go
  - internal/semantic/types/php/stub_test.go
  - internal/semantic/types/python/comment.go
  - internal/semantic/types/python/resolver.go
  - internal/semantic/types/python/resolver_test.go
  - internal/semantic/types/python/scope.go
  - internal/semantic/types/resolver.go
  - internal/semantic/types/resolver_test.go
  - internal/semantic/types/ruby/stub.go
  - internal/semantic/types/ruby/stub_test.go
  - internal/semantic/types/typescript/comment.go
  - internal/semantic/types/typescript/resolver.go
  - internal/semantic/types/typescript/resolver_test.go
  - internal/semantic/types/typescript/scope.go
findings:
  critical: 4
  warning: 8
  info: 5
  total: 17
status: issues_found
---

# Phase 62: Code Review Report

**Reviewed:** 2026-05-06
**Depth:** standard
**Files Reviewed:** 80
**Status:** issues_found

## Summary

Phase 62 ships a generic deterministic PageRank engine, graph_version
advance machinery, a per-workspace RankScheduler, weak-component
clustering, and a tiered type-resolution layer. Determinism scaffolding
(sort-before-iterate, hex-digest goldens) is consistently applied at the
engine and frontier layers, the per-workspace mutex contract from Phase
60 is correctly preserved, and the `internal/graph/` engine respects its
"stdlib-only, no project imports" invariant.

However, the review found four BLOCKER-class concurrency / correctness
defects:

1. The `runIncrementalRepair` post-commit timer-reset path holds the
   per-workspace overlay lock across calls back into the store
   (`CountStaleScoreRows`) — a latent self-deadlock waiting for
   `rankStoreAdapter.CountStaleScoreRows` to acquire the same lock.
2. `runIncrementalRepair` and `maybeFullRecompute` close over the
   activation-time `ctx` when arming `time.AfterFunc` callbacks; under
   timer fire the callbacks ignore `s.in`-side cancellation and run
   without honoring scheduler shutdown ordering.
3. The Python / TypeScript / Go `guessFromName` helpers iterate a
   `map[string]string` directly. Any future overlap of suffix keys (or
   a refactor that adds one) silently breaks the GRAPH-01 / TYPES-04
   determinism contract that the phase otherwise enforces religiously
   ("Every node-keyed iteration in this package MUST funnel through a
   sorted slice" — `internal/semantic/graph/util.go`).
4. `RankScheduler.runIncrementalRepair` writes score rows stamped with
   `s.lastSeenGV` without re-checking `CurrentGraphVersion` after
   acquiring the workspace lock and without a preemption mark — the
   companion path `RunFullRecompute` enforces D-10 preemption, but the
   incremental path does not, so a competing `ApplyRepair` between
   `QueryEffectiveAdjacency` and `Commit` quietly persists rows under
   a stale gv that will read back as `stale` immediately.

Plus eight WARNING-class issues (test fakes with race-prone field
access, dead code in the `time` import, mutated input slice in the
cascade, and several latent contract gaps) and five lower-severity
items.

## Critical Issues

### CR-01: RankScheduler holds workspace lock across CountStaleScoreRows callback

**File:** `internal/semantic/graph/scheduler.go:237-294`
**Issue:** `runIncrementalRepair` acquires `release := s.store.LockWorkspace(s.repoID)` at line 237 and `defer release()` is the last `defer` (innermost in LIFO order). The function then calls `tx.Commit()` at line 271, sets `committed = true`, and at line 282 invokes `s.store.CountStaleScoreRows(ctx, s.repoID, s.cfg.Projection)` while the workspace lock is STILL held (defer runs only at function return). Production `rankStoreAdapter.CountStaleScoreRows` is a stub today (`internal/daemon/rank_wiring.go:264-268`), so the deadlock is dormant — but the contract on `LockWorkspace` mirrors the same per-workspace lock that `BeginRepairTx` / `BeginOverlayTx` acquires. The moment Phase 64 wires a real `CountStaleScoreRows` that opens its own tx for the COUNT query (or any future read path that takes the same lock for MVCC consistency), this becomes a hard deadlock against itself.

The same pattern repeats in `maybeFullRecompute` at line 305 (CountStaleScoreRows BEFORE LockWorkspace via `RunFullRecompute`, OK) but if a subsequent change reorders or wraps the call in any locking, the same hazard reopens.

**Fix:** Release the workspace lock immediately after `tx.Commit()`, BEFORE the post-commit stale-fraction read. Either restructure to drop `defer release()` in favor of an explicit `release()` after commit, or move the stale-fraction probe into a helper that runs without the workspace lock held. Document on `SchedulerStore.CountStaleScoreRows` that it MUST NOT take the workspace lock.

```go
// scheduler.go:237 — explicit release after commit
release := s.store.LockWorkspace(s.repoID)
// ... tx body, Commit ...
release()
// Stale-fraction probe runs lock-free.
stale, total, err := s.store.CountStaleScoreRows(...)
```

### CR-02: Timer callbacks capture activation-time ctx and survive scheduler shutdown ordering

**File:** `internal/semantic/graph/scheduler.go:161-168, 286-294, 314-321`
**Issue:** `handleAdvance` (called from `Run`'s ctx-driven select loop) installs `time.AfterFunc(s.cfg.Debounce, func() { s.runIncrementalRepair(ctx) })`. The captured `ctx` is the one the loop received; that's nominally OK. But:

1. `stopTimers()` (line 140-149) calls `s.debounceTimer.Stop()` / `s.longIdleTimer.Stop()`. Per Go's `time.Timer.Stop` contract, Stop returns false if the timer's callback is already executing or has already run — it does NOT block the in-flight callback or cancel it. So if Run returns due to ctx.Done while a debounce timer is firing concurrently, the goroutine running `runIncrementalRepair` may still call into the store, open `BeginRepairTx`, and write a partial result AFTER scheduler shutdown completed. The errgroup expects scheduler.Run's return to mean "fully quiesced".
2. `runIncrementalRepair` re-arms `s.longIdleTimer = time.AfterFunc(s.cfg.FullRecomputeIdle, func() { s.maybeFullRecompute(ctx) })` at line 289. But by the time this callback fires (default 60s later), the captured `ctx` may have been cancelled. The first line of `maybeFullRecompute` does check `ctx.Err()` (line 302), good — but the deferred re-arm inside `maybeFullRecompute` (lines 314-321) also captures the same potentially-stale ctx and unconditionally re-arms a new AfterFunc (line 317-319). On shutdown this leaks a timer with a captured-cancelled-ctx callback that fires once and exits via the ctx.Err check; not catastrophic, but the cleanup contract is muddy.

**Fix:** Make timer callbacks honor a shutdown-aware ctx. Either replace `time.AfterFunc` with a select-driven timer goroutine that participates in the same closed-channel lifecycle as `Run`, or wait on in-flight timer callbacks via a `sync.WaitGroup` before `Run` returns. Concrete pattern:

```go
type RankScheduler struct {
    // ...existing fields...
    activeWG sync.WaitGroup // tracks in-flight AfterFunc callbacks
}

func (s *RankScheduler) handleAdvance(ctx context.Context, adv GraphVersionAdvance) {
    // ...
    s.timerMu.Lock()
    s.debounceTimer = time.AfterFunc(s.cfg.Debounce, func() {
        s.activeWG.Add(1)
        defer s.activeWG.Done()
        s.runIncrementalRepair(ctx)
    })
    // ...
}

func (s *RankScheduler) Run(ctx context.Context) error {
    defer s.activeWG.Wait() // block until in-flight callbacks drain
    defer s.stopTimers()
    // ...existing select loop...
}
```

### CR-03: guessFromName iterates an unsorted map (latent determinism break)

**File:** `internal/semantic/types/golang/resolver.go:259-280`
**File:** `internal/semantic/types/python/resolver.go:198-221`
**File:** `internal/semantic/types/typescript/resolver.go:193-211`
**Issue:** All three per-language `guessFromName` helpers iterate a literal `suffixMap := map[string]string{...}` with `for short, long := range suffixMap`. Today the suffixes ("Repo", "Svc", "Mgr", "Ctrl", "Cfg", "Conf", "Hdlr", "Conn") happen not to overlap as suffixes of any common identifier, so a single iteration deterministically picks at most one match per name. But the moment any future suffix is added that shares a tail with another (or a refactor inadvertently introduces overlap), Go's randomized map iteration order will return different `long` forms for the same input across runs / processes — silently breaking the GRAPH-01 byte-equal determinism contract that Phase 62 spent significant effort establishing (`internal/semantic/graph/util.go` — "Every node-keyed iteration in this package MUST funnel through a sorted slice"; `internal/graph/pagerank.go` D-03 sort-before-iterate).

The phase otherwise enforces this religiously (e.g., `frontier.go:48-64`, `pagerank.go:36-37`, `weak.go:90-94`). The three `guessFromName` helpers are the only places the rule is broken.

**Fix:** Replace the map iteration with sorted-key iteration:

```go
// golang/resolver.go:255 (and similarly for python + typescript)
func guessFromName(name string) string {
    if name == "" {
        return ""
    }
    type rule struct{ short, long string }
    // sorted by short ascending — deterministic match-first regardless of
    // future additions that may alias one suffix as another's tail.
    rules := []rule{
        {"Cfg", "Config"}, {"Conf", "Configuration"}, {"Conn", "Connection"},
        {"Ctrl", "Controller"}, {"Hdlr", "Handler"}, {"Mgr", "Manager"},
        {"Repo", "Repository"}, {"Svc", "Service"},
    }
    for _, r := range rules {
        if strings.HasSuffix(name, r.short) && len(name) > len(r.short) {
            prefix := strings.TrimSuffix(name, r.short)
            if prefix == "" { return r.long }
            return strings.ToUpper(prefix[:1]) + prefix[1:] + r.long
        }
    }
    return ""
}
```

### CR-04: Incremental repair writes score rows under stale gv without preemption check

**File:** `internal/semantic/graph/scheduler.go:176-296`
**Issue:** `runIncrementalRepair` reads `out, in` adjacency at line 188 (BEFORE `LockWorkspace`), runs PageRank at lines 231-235, then `LockWorkspace` at 237 and writes scores stamped with `s.lastSeenGV` (line 259). There is NO re-read of `CurrentGraphVersion` after acquiring the lock, and NO preemption marker. If a competing `ApplyRepair` lands on the same workspace between `QueryEffectiveAdjacency` (line 188) and `BeginRepairTx` (line 240), the workspace lock serializes the two writes correctly — but the scheduler's incremental write commits with `GraphVersion=s.lastSeenGV` which is now strictly less than the post-commit current gv. Readers will compute `ScoreStatusStale` for every row that just got written (per `computeScoreStatus` in status.go).

The companion path `RunFullRecompute` correctly handles this case: lines 114-117 re-read `CurrentGraphVersion` mid-tx and stamp `ScoreStatusApproximate` if `endGV > startGV` (D-10). The incremental path does NOT mirror this defense — it assumes its short window between adjacency read and tx commit is preemption-free, which the per-workspace lock does NOT guarantee for the read.

The acceptance test `TestRankScheduler_DebounceCoalesces` does not exercise this path because the fake's `BumpGraphVersion` only fires from inside `BeginRepairTx`, and the test does not interleave a competing ApplyRepair.

**Fix:** Mirror `RunFullRecompute`'s preemption check inside `runIncrementalRepair`. Read `CurrentGraphVersion` after `BeginRepairTx`, compare against `s.lastSeenGV`, and stamp `ScoreStatusApproximate` (or short-circuit with a `preempted` outcome metric) if it advanced. Concretely, replace the row-build loop near line 254:

```go
currentGV, err := s.store.CurrentGraphVersion(ctx, s.repoID)
if err != nil { /* metricInc("error"); return */ }
status := ScoreStatusExact
if currentGV > s.lastSeenGV {
    status = ScoreStatusApproximate
}
rows := make([]ScoreRow, 0, len(frontier))
for _, n := range frontier {
    rows = append(rows, ScoreRow{
        NodeID: n, Score: scores[n],
        GraphVersion: currentGV,           // <-- write under the actual current gv
        Status:       string(status),
    })
}
```

## Warnings

### WR-01: prioritizeSymbols mutates input slice after UpsertSymbols already wrote it

**File:** `internal/semantic/lspenrich/cascade.go:332, 349, 497-504`
**Issue:** `cascade.Run` calls `tx.UpsertSymbols(ctx, job.Path, syms)` at line 332, then `prioritized := prioritizeSymbols(syms, budget)` at line 349 which iterates `for i := range syms { syms[i].Name = strings.TrimSpace(syms[i].Name) }` — mutating the SAME backing array `tx.UpsertSymbols` was just handed. Whether the store has finished consuming `syms` synchronously is implementation-dependent (today's store is in-process so it likely has, but the contract is implicit). At minimum this is a confusing data-flow ordering: persisted names may differ from the per-symbol names used in subsequent cascade steps.

**Fix:** Trim names BEFORE `tx.UpsertSymbols`, or copy the slice in `prioritizeSymbols` before mutating. Cleanest: trim in `documentSymbol` adapter before the cascade ever sees them.

### WR-02: full_recompute.go retains time import via `_ = time.Now()` placeholder

**File:** `internal/semantic/graph/full_recompute.go:14, 165`
**Issue:** Line 165 reads `_ = time.Now() // hold the time import for future metric integration`. This is dead code with a comment promising future use; the `time` import on line 16 only exists because of this no-op call. If a future change drops the placeholder without adding a real timer, the import lingers. Small thing, but the discipline elsewhere in the phase is not to keep "for future" hooks that don't compile-check anything useful.

**Fix:** Remove the `time` import + the `_ = time.Now()` line until a real metric timer is added. The duration metric `SemanticGraphPagerankObserve` is already wired through `MetricsSink` (apply_repair.go:75) and would be the natural emission site.

### WR-03: traceApplyRepair declared but never invoked

**File:** `internal/semantic/graph/trace.go:1-15`
**Issue:** `traceApplyRepair(ctx, repoID)` is defined as a no-op span helper but `apply_repair.go` does not call it. The doc comment promises "Phase 62 P02 ships the seam without otel wiring; P03 may swap in a real opentelemetry.SpanFromContext call" — P03 has shipped per the phase plans, and the call site is still absent. Either the helper is dead or the wiring forgot to land.

**Fix:** Either wire `traceApplyRepair(ctx, repoID)` at the top of `Engine.ApplyRepair` (the natural call site, with `defer end(err)` for span finalization) or delete trace.go until a real tracer is imported.

### WR-04: rank_wiring.go has dead `_ = fmt.Sprintf` import-keepalive

**File:** `internal/daemon/rank_wiring.go:23, 344`
**Issue:** Line 344 reads `var _ = fmt.Sprintf // keeps unused fmt import out of the build; remove once any error formatting lands.` The file imports `fmt` (line 23) solely to keep this compile-time keepalive happy. If the comment's contingency ("once any error formatting lands") is the actual contract, the file currently has zero error formatting and should drop the import.

**Fix:** Remove the unused `fmt` import and the keepalive sentinel. If a future revision wants `fmt.Errorf`, add the import then.

### WR-05: rankStoreAdapter implements SchedulerStore as no-op stubs returning success

**File:** `internal/daemon/rank_wiring.go:251-273`
**Issue:** `rankStoreAdapter.QueryEffectiveGraph`, `QueryEffectiveAdjacency`, `CountStaleScoreRows`, and `MarkAllScoreRowsStale` all return empty results and `nil` error today (Phase 64 follow-up TODOs). This is documented, BUT the consequence is that every `runIncrementalRepair` call in production today computes an empty frontier and writes zero score rows — silently. There is no observability signal (warning log, INFO with hint) that the rank surface is currently empty due to a deferred dependency. Operators monitoring `helix_semantic_graph_repair_total{outcome="applied"}` will see clean metrics that mask "no data" semantics.

**Fix:** At each stub method, log once at slog.Warn with a "Phase 64 follow-up — returning empty result" message keyed to a `sync.Once`, OR record a distinct metric (`outcome="stub_no_data"` or similar) so dashboards reveal the gap. Alternatively, gate the whole `rankBundle` construction behind a config feature-flag that defaults to off until Phase 64 lands.

### WR-06: ConfigForRunFullRecompute pre-deletes scores even when nodes is empty

**File:** `internal/semantic/graph/full_recompute.go:97-99`
**Issue:** `RunFullRecompute` runs `tx.DeleteScoresForProjection(ctx, projection)` at line 97 unconditionally — even when `len(nodes) == 0` (e.g., a freshly-bootstrapped workspace with no symbols yet, or the production stub returning empty). The function then writes zero rows and commits. This means every fresh bootstrap full-recompute deletes any scores the operator might have hand-seeded for testing, AND every full recompute against a stub-empty-graph wipes the score table on every long-idle fire. With the rank surface stubbed to empty (WR-05 above), any pre-existing score rows from a prior process get dropped on the first long-idle fire after restart.

**Fix:** Short-circuit when `len(nodes) == 0`: skip the delete + upsert, commit a no-op tx, return `(false, nil)`. The intent of `DeleteScoresForProjection` is "replace the prior generation with the new generation atomically", which is meaningless when the new generation is empty.

```go
// full_recompute.go:97
if len(nodes) == 0 {
    return false, nil
}
if err := tx.DeleteScoresForProjection(...); err != nil { ... }
```

### WR-07: nodeSet.add called concurrently is unsafe; no doc disclaimer

**File:** `internal/semantic/graph/repair.go:141-156`
**Issue:** `nodeSet` is used inside `ComputeGraphRepair` on a per-call local stack so concurrency is fine TODAY. But `nodeSet` is exposed as a package-level type (used by `apply_repair.go:223-228` inside the `endpoints` accumulator over `repair.RemovedEdges`) and has no doc comment indicating it is single-goroutine. A future caller who shares one `nodeSet` across goroutines (e.g., a parallel diff merge) gets a data race on `s.seen` and `s.order`.

**Fix:** Add a one-line doc comment to `nodeSet`: "Not safe for concurrent use; callers MUST own the value." Same for `slice()` / `add()`.

### WR-08: sortedNodeIDsSlice has unreachable code branch

**File:** `internal/semantic/graph/full_recompute.go:171-187`
**Issue:** `sortedNodeIDsSlice` builds a dedup map, calls `sortedNodeIDs(tmp)`, then has:
```go
if len(sorted) == len(out) {
    return sorted
}
return sorted // dedup-safe path
```
Both branches return the same `sorted` value — the conditional is dead. Comment claims "dedup-safe path" for the second branch but neither branch differs in observable behavior. Either the dedup behavior is intended (and the unused `out` should be removed) or the dedup is unintended (and the `if len ==` was supposed to fall through to a different return).

**Fix:** Simplify to `return sortedNodeIDs(tmp)`. Remove the unused `out` variable.

## Info

### IN-01: nodeIDsToUint64 documents NodeID as a uint64 alias but converts anyway

**File:** `internal/semantic/graph/apply_repair.go:307-316`
**Issue:** `nodeIDsToUint64` allocates a new uint64 slice for every call. Comment says "the conversion is a no-op but the copy preserves caller-provided slice ownership." Since `NodeID = uint64` (type alias, line 7 in repair.go), the slice could be passed via `unsafe.Slice` without copy — but the explicit copy is defensive and correct. Just noisy.

**Fix:** Keep as-is; could optimize later if benchmarks show it matters.

### IN-02: graph.Options.Personalize uses map[any]float64 to dodge type parameters

**File:** `internal/graph/options.go:24-29`
**Issue:** `Options.Personalize map[any]float64` exists so the struct stays type-parameter-free. The `any` keys are then asserted to `T` at PageRank time. This works but pushes type errors from compile to runtime; callers passing a wrong key type get silently-skipped seeds with no error. The current callers (repomap, scheduler) all pass strings or NodeIDs correctly, but a future caller could pass an int and get unexpected uniform-teleport behavior.

**Fix:** Document on `Personalize` that "keys NOT type-assertable to T are silently dropped — use type-aware constructors at the call site." Or add a debug-build assertion.

### IN-03: cluster persist_test.go RoundTrip test runs orchestrator twice for capture

**File:** `internal/semantic/cluster/persist_test.go:122-177`
**Issue:** `TestRunClusterDetection_RoundTrip` runs `RunClusterDetection` once with `fs`, then runs it AGAIN with a `capturingFakeStore` to capture tx state. The first run's success/error state is checked but the captured-tx assertions only validate the second run. This is fragile (dual-run masks any test that breaks on state-shared side effects).

**Fix:** Restructure `fakeStore.BeginOverlayTx` to record its returned tx on the store struct (`f.lastTx = newTx; return newTx, nil`), so a single run can be inspected. Avoids the double-run pattern entirely.

### IN-04: edgeIDForTriple masks high bit; comment understates collision risk

**File:** `internal/semantic/store/overlay.go:728-759`
**Issue:** FNV-1a 64-bit hash masked to 63 bits via `& 0x7FFFFFFFFFFFFFFF` because duckdb-go rejects high-bit-set uint64. Comment claims "collisions across distinct triples are vanishingly improbable for the per-workspace cardinality (< 10M edges)". For 10M edges in 2^63 space, birthday-paradox collision probability is ~10M^2/2^64 ≈ 5.4e-6 per workspace — small, but not "vanishingly small". For a fleet of 100k workspaces this is ~50% chance of at least one workspace having a collision. The collision wouldn't corrupt data (UPSERT just merges two distinct triples into one row, losing the second), but it is a silent data-loss path.

**Fix:** Either (a) document the fleet-level math accurately, (b) add a CHECK constraint or post-write verification, or (c) widen to a 128-bit hash and store as two columns. Optional for v1; flag for v1.10.x audit.

### IN-05: PageRank single-node branch has dead-code `_ = w` after assignment

**File:** `internal/graph/pagerank.go:39-52`
**Issue:** Single-node fast path:
```go
if w, ok := opts.Personalize[any(sorted[0])]; ok && w > 0 {
    score = 1.0
    _ = w // re-normalization on a single-node graph is trivially 1.0
}
```
The variable `w` is read in the `ok && w > 0` guard then explicitly silenced via `_ = w`. The `if` body unconditionally sets `score = 1.0` regardless of `w`'s value (already 1.0 from line 43). The whole block could be `if _, ok := opts.Personalize[...]; ok { /* score stays 1.0 */ }`. Or just delete the entire `if` since `score = 1.0` was already assigned.

**Fix:** Simplify to `out[sorted[0]] = 1.0` — the personalize check has no observable effect on a single-node graph.

---

_Reviewed: 2026-05-06_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
