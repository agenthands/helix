# Phase 63: Compaction & Retention - Pattern Map

**Mapped:** 2026-05-07
**Files analyzed:** 18 (3 new packages/files + 8 accessor additions + 4 wiring/config + 3 vet/test fixtures)
**Analogs found:** 18 / 18 (every new file has a strong in-tree analog)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/semantic/store/snapshot.go` | service (DB write API) | request-response (BeginTx → Write → Commit) | `internal/semantic/store/overlay.go` (BeginOverlayTx + UpsertOverlayFile + Commit/Rollback) | exact (sister file in same package) |
| `internal/semantic/compact/compactor.go` | service (per-workspace goroutine) | event-driven (OnFlush → AfterFunc → fire) | `internal/semantic/live/coalescer/coalescer.go` | exact (per-workspace + AfterFunc pattern) |
| `internal/semantic/compact/gate.go` | utility (read-only aggregator) | request-response (IsReady probe) | NEW pattern — closest is `RankScheduler.handleAdvance` lock+state read; no pre-existing aggregator | role-novel (constructor injection precedent from Phase 59 D-02) |
| `internal/semantic/compact/accessors.go` | utility (interface declarations) | n/a (type-only) | `internal/semantic/graph/scheduler.go:55-71` (SchedulerStore interface block) | exact (interface-collection idiom) |
| `internal/semantic/compact/vacuum.go` | service (config-gated maintenance op) | request-response (separate tx) | `internal/semantic/store/overlay.go:194` (FlushOverlay no-op stub) + new `Store.Checkpoint`/`Store.Vacuum` helpers | role-match (DB op wrapper + store-layer call) |
| `internal/semantic/compact/compactor_test.go` | test (unit) | n/a | `internal/semantic/live/coalescer/coalescer_test.go` (closed-channel + tight Debounce) | exact (sibling test pattern) |
| `internal/semantic/compact/gate_test.go` | test (unit, one per BlockedReason) | n/a | `internal/semantic/graph/scheduler_test.go` (per-state assertion) | role-match |
| `internal/semantic/compact/kill_test.go` + `testdata/cmd/compact_one/main.go` | test (subprocess integration) | n/a | NEW — no existing subprocess fixture in tree | role-novel (uses stdlib `exec.CommandContext`) |
| `internal/semantic/compact/interleave_test.go` | test (CAS property) | n/a | `internal/semantic/store/overlay_epoch_test.go` (TestOverlayEpochConcurrent under -race) | role-match |
| `internal/semantic/compact/longrepo_bench_test.go` | bench (local-only) | n/a | none in `internal/semantic/`; pattern from `internal/repomap/*_bench_test.go` | role-match |
| `internal/semantic/live/coalescer/coalescer.go` (additive: `LastFlushAt`) | accessor | n/a | self (atomic.Int64 nano stamp; mirrors `Coalescer.drops atomic.Uint64`) | self-extension |
| `internal/semantic/store/overlay.go` (additive: `OverlayTxOpenCount`, `OverlayRowCount`) | accessor | request-response (count query) | `Store.BeginOverlayTx`'s per-workspace `overlayLocks` map (`overlay.go:148-160`) | self-extension (atomic counter on existing per-ws state) |
| `internal/semantic/lspenrich/queue.go` (additive: `LastEnqueueAt`) | accessor | n/a | `LaneQueue.Depth(lane)` already at `queue.go:67-76` | self-extension |
| `internal/semantic/graph/scheduler.go` (additive: `IsQuiescent`) | accessor | n/a | `RankScheduler.timerMu`/`pendingChanged`/`debounceTimer` (`scheduler.go:64-71`) | self-extension |
| `internal/kernel/` (additive: `ActiveEditTxCount`) | accessor | n/a | small new counter; closest is the per-workspace pattern in `lspool` worker counters | role-match |
| `internal/config/defaults.go` (additive: `maintenance.*`) | config | n/a | `defaults.go:127-130` (Phase 62 P02 added `pagerank.repair_debounce_ms` etc. to existing map) | exact (same map literal style) |
| `internal/semantic/config.go` (additive: `MaintenanceConfig` struct + field) | config | n/a | `LiveUpdatesConfig` (`config.go:142-164`) | exact (sibling nested struct) |
| `internal/obs/metrics.go` (additive: `helix_semantic_compaction_duration_seconds`, `helix_semantic_vacuum_duration_seconds` + helpers) | observability | n/a | `LSPEnrichmentDurationVec` registration + `SemanticGraphRepairInc` helper (`metrics.go:313-322, 765-777`) | exact (HistogramVec + drop-on-unknown helper) |
| `internal/daemon/daemon.go` + new `internal/daemon/compact_wiring.go` | wiring | n/a | `internal/daemon/rank_wiring.go` (rankBundle + ensureScheduler) | exact (lazy per-workspace registry) |
| `internal/semantic/store/migrations.go` (optional `migration004` for `last_vacuum_at`) | migration | n/a | `applyMigration003` + `schema3Statements` (`migrations.go:472-535`) | exact (registry-based, ALTER + INSERT version stamp) |
| `cmd/vet-compact-uses-store/main.go` + `internal/lint/compactusesstore/analyzer.go` (OPTIONAL) | vet analyzer | n/a | `cmd/vet-noduckdb/main.go` + `internal/lint/noduckdb/analyzer.go` | exact (singlechecker boundary analyzer) |

---

## Pattern Assignments

### `internal/semantic/store/snapshot.go` (service, DB write API)

**Analog:** `internal/semantic/store/overlay.go`

**Package + imports pattern** (overlay.go:27-35):
```go
package store

import (
    "context"
    "database/sql"
    "fmt"
    "strings"
    "sync"
)
```
Phase 63 snapshot.go MUST live in the SAME `package store` and reuse the same `*Store` receiver — this is what keeps `cmd/vet-noduckdb` happy (only `internal/semantic/store/` may import `duckdb-go`).

**Per-tx handle pattern** (overlay.go:44-58, mirror with `Snapshot` instead of `OverlayTx`):
```go
type OverlayTx struct {
    tx     *sql.Tx
    repoID string
    epoch  uint64
    unlock     func()
    unlockOnce sync.Once
}

func (t *OverlayTx) Epoch() uint64 { return t.epoch }
func (t *OverlayTx) RepoID() string { return t.repoID }
```
Phase 63 mirror:
```go
type Snapshot struct {
    ID         uint64
    RepoID     string
    tx         *sql.Tx
    meta       SnapshotMeta
    createdAt  time.Time
}
```
Note: the per-workspace `unlock` closure pattern from OverlayTx is OPTIONAL for compaction — D-01's single-tx compaction acquires the workspace overlay lock once for the ClearOverlay portion, but `BeginSnapshot` itself does not need the per-ws mutex (snapshot writes don't share state with concurrent overlay writers; the CAS read is what serializes them).

**Begin pattern with nil-guards + error wrapping** (overlay.go:80-141):
```go
func (s *Store) BeginOverlayTx(ctx context.Context, repoID string) (*OverlayTx, error) {
    if s == nil || s.db == nil {
        return nil, fmt.Errorf("BeginOverlayTx: nil store")
    }
    if repoID == "" {
        return nil, fmt.Errorf("BeginOverlayTx: empty repoID")
    }
    // ... ensure-meta-row → atomic counter bump → BeginTx
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        mu.Unlock()
        return nil, fmt.Errorf("BeginOverlayTx: open tx: %w", err)
    }
    return &OverlayTx{tx: tx, repoID: repoID, epoch: epoch, unlock: mu.Unlock}, nil
}
```
Phase 63 `BeginSnapshot` MUST follow this nil-guard + error-wrap discipline. Insert the pending `semantic_snapshots` row inside the tx so a Rollback discards it.

**Commit/Abort pattern** (overlay.go:163-184):
```go
func (t *OverlayTx) Commit() error {
    defer t.releaseLock()
    return t.tx.Commit()
}

func (t *OverlayTx) Rollback() error {
    defer t.releaseLock()
    return t.tx.Rollback()
}
```
Phase 63: `CommitSnapshot` updates `status='committed'` + writes summary BEFORE `tx.Commit()`. `AbortSnapshot` calls `tx.Rollback()` and logs the reason — no explicit DELETE of the pending row needed (rollback handles it).

**SQL parameterization pattern** (overlay.go:106-126, 209-218): every `INSERT`/`UPDATE` uses `?` placeholders bound to the receiver's `repoID`/`epoch`/etc. — never string concat. Phase 63's `WriteSnapshotFacts` MUST use parameterized inserts (per-fact-table batch loop, the same way `MarkSymbolsDeleted` at lines 265-284 iterates per file_id).

**Anti-pattern to avoid:** Do NOT use `Appender` API for `WriteSnapshotFacts` — RESEARCH.md Landmine 6 states the Appender's commit semantics are not bound to the surrounding `*sql.Tx`. Stick with `database/sql` `ExecContext`.

---

### `internal/semantic/compact/compactor.go` (service, per-workspace goroutine)

**Analog:** `internal/semantic/live/coalescer/coalescer.go`

**Struct shape** (coalescer.go:80-93):
```go
type Coalescer struct {
    workspaceID workspace.WorkspaceKey
    cfg         Config
    handler     EventHandler
    logger      Logger
    metrics     MetricsSink
    in          chan live.SourceChangeEvent
    drops       atomic.Uint64

    mu       sync.Mutex
    pending  map[string]live.SourceChangeEvent
    timer    *time.Timer
    maxTimer *time.Timer
}
```
Phase 63 `Compactor`: NO input channel (the trigger is `OnFlush()` plus `AfterFunc` — there is no event stream to drain; the gate is queried on timer fire). One timer instead of two (no max-batch ceiling — the size guard plays that role per RESEARCH.md Pattern 1).

**Constructor with default-fill pattern** (coalescer.go:97-127):
```go
func New(ws workspace.WorkspaceKey, cfg Config, handler EventHandler, logger Logger) *Coalescer {
    def := DefaultConfig()
    if cfg.Debounce <= 0 {
        cfg.Debounce = def.Debounce
    }
    // ...
    if logger == nil {
        logger = noopLogger{}
    }
    metrics := cfg.Metrics
    if metrics == nil {
        metrics = noopMetrics{}
    }
    return &Coalescer{...}
}
```
Phase 63 `compact.NewCompactor`: same idiom — fill zero-value config from `DefaultConfig()`, default nil logger to `slog.Default()`, default nil metrics to a noop sink.

**Run loop pattern** (coalescer.go:153-171):
```go
func (c *Coalescer) Run(ctx context.Context) error {
    flush := c.makeFlush(ctx)
    for {
        select {
        case <-ctx.Done():
            c.mu.Lock()
            if c.timer != nil {
                c.timer.Stop()
            }
            // ...
            c.mu.Unlock()
            return ctx.Err()
        case ev := <-c.in:
            c.accept(ev, flush)
        }
    }
}
```
Phase 63 `Compactor.Run`: simpler — only `<-ctx.Done()` to handle (no event channel). The actual fire path is invoked by `time.AfterFunc` callbacks the goroutine itself never blocks on. Loop body is just `<-ctx.Done()` → stop timer → return ctx.Err().

**Timer-reset pattern under mutex** (coalescer.go:193-202):
```go
if c.timer != nil {
    c.timer.Stop()
}
c.timer = time.AfterFunc(c.cfg.Debounce, flush)
```
Phase 63 `Compactor.OnFlush()` calls EXACTLY this sequence under `c.mu`. Per RESEARCH.md Pitfall 4 the race between `Stop()` and the firing callback is BENIGN because `fire()` re-checks `gate.IsReady` first and a back-to-back fire becomes a no-op (`BlockedOverlayEmpty`).

**Defer-stop on shutdown** (`scheduler.go:128-149` — RankScheduler precedent for the shutdown stop pattern):
```go
func (s *RankScheduler) Run(ctx context.Context) error {
    defer s.stopTimers()
    for { ... }
}

func (s *RankScheduler) stopTimers() {
    s.timerMu.Lock()
    defer s.timerMu.Unlock()
    if s.debounceTimer != nil {
        s.debounceTimer.Stop()
    }
    if s.longIdleTimer != nil {
        s.longIdleTimer.Stop()
    }
}
```
Phase 63 `Compactor.Run` SHOULD use the `defer s.stopTimers()` form (cleaner than the inline coalescer Stop block).

**Outcome metric emission with deferred timer** (RESEARCH.md Example 2):
```go
func (c *Compactor) runCompaction(ctx context.Context) (outcome string) {
    defer func(start time.Time) {
        c.metrics.SemanticCompactionObserve(outcome, time.Since(start))
    }(time.Now())
    // ...
    return "success"
}
```
Mirrors how `Coalescer.makeFlush` emits `SemanticLiveUpdatesInc(kind, "applied"|"error"|...)` per dispatched event.

**Anti-patterns to avoid:**
- Spawning a per-event goroutine (the timer callback runs in its own goroutine; do NOT `go runCompaction()` — that produces concurrent compactions).
- Holding `c.mu` while calling `c.gate.IsReady()` or `runCompaction` (long DB I/O under the timer-reset mutex blocks producer-side `OnFlush()` calls — release before fire).

---

### `internal/semantic/compact/gate.go` (utility, read-only aggregator)

**Analog:** novel pattern, but the closed-enum + constructor-injection idioms come from existing files.

**Closed-enum pattern** (`internal/semantic/lspenrich/queue.go:12-20`):
```go
type Lane string

const (
    LaneHigh       Lane = "high"
    LaneBackground Lane = "background"
)
```
Phase 63 `BlockedReason` MUST follow this exact form so `metrics_labels_test.go`-style cardinality enforcement works:
```go
type BlockedReason string

const (
    BlockedNone            BlockedReason = ""
    BlockedOverlayEmpty    BlockedReason = "overlay_empty"
    BlockedIdleTooShort    BlockedReason = "idle_too_short"
    BlockedEditTxActive    BlockedReason = "edit_tx_active"
    BlockedOverlayTxActive BlockedReason = "overlay_tx_active"
    BlockedLSPPending      BlockedReason = "lsp_pending"
    BlockedRankRepairing   BlockedReason = "rank_repairing"
)
```

**Constructor with injected `now` for test determinism** (Phase 62 RankScheduler precedent — `NewRankScheduler` accepts deps; tests pass tight values):
```go
type CompactionGate struct {
    coalescerAccess CoalescerAccessor
    overlayAccess   OverlayTxAccessor
    overlayRowAccess OverlayRowAccessor
    lspAccess       LSPQueueAccessor
    schedAccess     SchedulerAccessor
    kernelAccess    KernelEditAccessor
    cfg             GateConfig
    now             func() time.Time   // injectable for test determinism
}
```
RESEARCH.md "Anti-Patterns to Avoid" #6: never call `time.Now()` directly inside the gate.

**`IsReady` deterministic-precedence body** (RESEARCH.md Pattern 2):
```go
func (g *CompactionGate) IsReady(ws workspace.WorkspaceKey) (bool, BlockedReason) {
    if g.overlayRowAccess.OverlayRowCount(ws) == 0 {
        return false, BlockedOverlayEmpty
    }
    if g.now().Sub(g.coalescerAccess.LastFlushAt(ws)) < g.cfg.CompactAfterIdle {
        return false, BlockedIdleTooShort
    }
    if g.kernelAccess.ActiveEditTxCount(ws) > 0 {
        return false, BlockedEditTxActive
    }
    if g.overlayAccess.OverlayTxOpenCount(ws) > 0 {
        return false, BlockedOverlayTxActive
    }
    if g.lspAccess.Depth(ws) > 0 &&
       g.now().Sub(g.lspAccess.LastEnqueueAt(ws)) < g.cfg.LSPCompactionMaxWait {
        return false, BlockedLSPPending
    }
    if !g.schedAccess.IsQuiescent(ws) {
        return false, BlockedRankRepairing
    }
    return true, BlockedNone
}
```
Hard invariants from CONTEXT.md D-04: side-effect-free, no I/O, returns FIRST blocking reason (not a bag) for deterministic test assertions. Each accessor returns from in-memory state only.

---

### `internal/semantic/compact/accessors.go` (utility, interface declarations)

**Analog:** `internal/semantic/graph/scheduler_store.go` (or the inline `SchedulerStore` interface in `scheduler.go` if no separate file).

**Interface-collection pattern** (mirror the Phase 62 `SchedulerStore` shape — small read-only methods, named after their question):
```go
// internal/semantic/compact/accessors.go

type CoalescerAccessor interface {
    LastFlushAt(ws workspace.WorkspaceKey) time.Time
}

type OverlayTxAccessor interface {
    OverlayTxOpenCount(ws workspace.WorkspaceKey) int
}

type OverlayRowAccessor interface {
    OverlayRowCount(ctx context.Context, repoID string, capturedEpoch uint64) (int, error)
}

type LSPQueueAccessor interface {
    Depth(ws workspace.WorkspaceKey) int
    LastEnqueueAt(ws workspace.WorkspaceKey) time.Time
}

type SchedulerAccessor interface {
    IsQuiescent(ws workspace.WorkspaceKey) bool
}

type KernelEditAccessor interface {
    ActiveEditTxCount(ws workspace.WorkspaceKey) int
}
```
Each accessor lives in `internal/semantic/compact/` (consumer-side definition) — the producer packages (`coalescer`, `store`, `lspenrich`, `graph`, `kernel`) implement them by adding a method that satisfies the interface. This avoids cross-package coupling: compaction depends on its OWN interface, not on the producer's exported type.

---

### `internal/semantic/live/coalescer/coalescer.go` (additive: `LastFlushAt`)

**Analog:** self — extend the existing `Coalescer` struct with an `atomic.Int64` (Unix nanos) updated in `makeFlush` after the snapshot succeeds.

**Atomic-stamp pattern** (mirror `c.drops atomic.Uint64` at `coalescer.go:87`):
```go
type Coalescer struct {
    // ... existing fields ...
    drops          atomic.Uint64
    lastFlushNanos atomic.Int64    // NEW: Unix nanos of most-recent flush
}

func (c *Coalescer) LastFlushAt() time.Time {
    n := c.lastFlushNanos.Load()
    if n == 0 {
        return time.Time{}
    }
    return time.Unix(0, n)
}
```

**Stamp site** (insert at the top of `makeFlush`'s closure body, after the `len(c.pending) == 0` short-circuit returns — line 207-211):
```go
return func() {
    c.mu.Lock()
    if len(c.pending) == 0 {
        c.mu.Unlock()
        return
    }
    // ... existing snapshot copy ...
    c.lastFlushNanos.Store(time.Now().UnixNano())  // NEW: stamp after snapshot, before mu.Unlock
    c.mu.Unlock()
    // ... existing dispatch loop ...
}
```
Note: the timestamp records "started flushing", not "finished flushing" — Phase 63's gate uses `now - LastFlushAt > compact_after_idle_ms` so the start time gives the correct semantics ("idle since the last flush BEGAN").

---

### `internal/semantic/store/overlay.go` (additive: `OverlayTxOpenCount`, `OverlayRowCount`)

**Analog:** self — add an `atomic.Int32` per workspace via the existing `s.overlayLocks` registry pattern.

**Counter map pattern** (mirror `overlay.go:148-160`'s lazy-install for `s.overlayLocks`):
```go
// In store struct (already has overlayLocks map[string]*sync.Mutex):
overlayTxCounts   map[string]*atomic.Int32  // keyed by repoID; lazily installed
overlayCountsMu   sync.Mutex
```

**Increment site** in `BeginOverlayTx` (insert AFTER line 130 `tx, err := s.db.BeginTx(ctx, nil)` succeeds):
```go
s.overlayTxCountFor(repoID).Add(1)
return &OverlayTx{... unlock: func() { s.overlayTxCountFor(repoID).Add(-1); mu.Unlock() }, ...}, nil
```
Wrap the existing `mu.Unlock` in a closure that decrements first.

**`OverlayTxOpenCount` accessor:**
```go
func (s *Store) OverlayTxOpenCount(ws workspace.WorkspaceKey) int {
    if s == nil {
        return 0
    }
    return int(s.overlayTxCountFor(ws.RepoRoot).Load())
}
```

**`OverlayRowCount` query pattern** (mirror the `MarkSymbolsDeleted` per-table iteration, but as `SELECT COUNT(*)`):
```go
func (s *Store) OverlayRowCount(ctx context.Context, repoID string, capturedEpoch uint64) (int, error) {
    if s == nil || s.db == nil {
        return 0, fmt.Errorf("OverlayRowCount: nil store")
    }
    var total int
    for _, table := range []string{
        "semantic_live_overlay_files",
        "semantic_live_overlay_symbols",
        "semantic_live_overlay_references",
        "semantic_live_overlay_edges",
    } {
        var n int
        q := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE repo_id = ? AND write_epoch <= ?`, table)
        if err := s.db.QueryRowContext(ctx, q, repoID, capturedEpoch).Scan(&n); err != nil {
            return 0, fmt.Errorf("OverlayRowCount(%s): %w", table, err)
        }
        total += n
    }
    return total, nil
}
```
Uses the `idx_overlay_*_write_epoch` indexes added by `applyMigration003` (`migrations.go:527-530`) so the COUNT is index-served, not a full scan.

---

### `internal/semantic/lspenrich/queue.go` (additive: `LastEnqueueAt`)

**Analog:** self — extend `LaneQueue` with `atomic.Int64` Unix nanos; mirror the `LastFlushAt` pattern from the coalescer change.

```go
type LaneQueue struct {
    high           *lspqueue.Queue
    background     *lspqueue.Queue
    lastEnqueueNs  atomic.Int64    // NEW
}

func (q *LaneQueue) EnqueueLane(lane Lane, job lspqueue.RevalidateFileJob) bool {
    var ok bool
    switch lane {
    case LaneHigh:
        ok = q.high.Enqueue(job)
    case LaneBackground:
        ok = q.background.Enqueue(job)
    default:
        return false
    }
    if ok {
        q.lastEnqueueNs.Store(time.Now().UnixNano())
    }
    return ok
}

func (q *LaneQueue) LastEnqueueAt() time.Time {
    n := q.lastEnqueueNs.Load()
    if n == 0 {
        return time.Time{}
    }
    return time.Unix(0, n)
}
```

**`Depth(ws)` aggregate** (the existing `Depth(lane)` is already at `queue.go:67-76`):
```go
// New helper for the workspace-wide total. Phase 63 consumes this directly.
func (q *LaneQueue) DepthAll() int {
    return q.Depth(LaneHigh) + q.Depth(LaneBackground)
}
```
Per-workspace key: today there's ONE `LaneQueue` per daemon (lspenrich.Manager owns it). RESEARCH.md "Existing Component Accessors" line 629 says this aggregate works because the queue is daemon-scoped. If Phase 64+ shards by workspace, the accessor signature `Depth(ws)` will need a workspace map; for Phase 63 the gate's `LSPQueueAccessor.Depth(ws)` is implemented as a thin shim that ignores `ws` and returns `DepthAll()`.

---

### `internal/semantic/graph/scheduler.go` (additive: `IsQuiescent`)

**Analog:** self — surface "no pending changed nodes AND no in-flight repair" from the existing `RankScheduler` state.

**Internal-state pattern** (`scheduler.go:64-71`):
```go
type RankScheduler struct {
    // ...
    timerMu        sync.Mutex
    pendingChanged []NodeID
    debounceTimer  *time.Timer
    longIdleTimer  *time.Timer
    lastSeenGV     uint64
}
```

**Quiescence accessor** (read under `timerMu`):
```go
func (s *RankScheduler) IsQuiescent() bool {
    if s == nil {
        return true
    }
    s.timerMu.Lock()
    defer s.timerMu.Unlock()
    return len(s.pendingChanged) == 0
}
```
**Open question:** does Phase 62's RankScheduler track in-flight `runIncrementalRepair` / `maybeFullRecompute` separately from `pendingChanged`? If `runIncrementalRepair` clears `pendingChanged` at line 181-183 BEFORE doing the work, an `IsQuiescent` check during the repair body would falsely report `true`. RESEARCH.md "Existing Component Accessors" line 631 calls this out: "Track in-flight via a counter or sync.Mutex try-lock pattern."

**Extension required:** add an `atomic.Int32 inFlightCount` on `RankScheduler`; `runIncrementalRepair`/`maybeFullRecompute` increment on entry, decrement in defer. Final accessor:
```go
func (s *RankScheduler) IsQuiescent() bool {
    if s == nil {
        return true
    }
    s.timerMu.Lock()
    pending := len(s.pendingChanged)
    s.timerMu.Unlock()
    return pending == 0 && s.inFlightCount.Load() == 0
}
```

**Per-workspace lookup:** the gate's `SchedulerAccessor.IsQuiescent(ws)` lives on `rankBundle` (in `internal/daemon/rank_wiring.go`), which already keeps the per-repoID map (`subs map[string]*graphpkg.RankScheduler` at `rank_wiring.go:53`). New method:
```go
func (b *rankBundle) IsQuiescent(ws workspace.WorkspaceKey) bool {
    if b == nil {
        return true   // no scheduler → trivially quiescent
    }
    b.mu.Lock()
    s := b.subs[ws.RepoRoot]
    b.mu.Unlock()
    return s.IsQuiescent()  // nil-safe via receiver guard above
}
```

---

### `internal/kernel/` (additive: `ActiveEditTxCount`)

**Analog:** small atomic counter on the kernel handle — increment on entry to each edit tool's Handle, decrement in defer.

**Counter struct** (mirror the `RankScheduler.inFlightCount` pattern just established):
```go
// internal/kernel/edit_tx_count.go (new small file)
type editTxCounters struct {
    mu     sync.Mutex
    counts map[string]*atomic.Int32  // workspace path → count
}

func (k *Kernel) ActiveEditTxCount(ws workspace.WorkspaceKey) int {
    k.editTxMu.Lock()
    c, ok := k.editTxCounts[ws.RepoRoot]
    k.editTxMu.Unlock()
    if !ok {
        return 0
    }
    return int(c.Load())
}

func (k *Kernel) BeginEditTx(ws workspace.WorkspaceKey) (release func()) {
    c := k.editTxCounterFor(ws.RepoRoot)
    c.Add(1)
    return func() { c.Add(-1) }
}
```

**Edit-tool wrap** (insert `defer k.BeginEditTx(ws)()` at the top of every edit tool's `Handle` body — RESEARCH.md "Existing Component Accessors" line 632 enumerates them: `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`, `replace_in_file`, `fuzzy_edit`, `write_file`).

---

### `internal/config/defaults.go` (additive: `maintenance.*`)

**Analog:** `defaults.go:127-130` — Phase 62 P02 added new keys to the existing top-level map literal.

**Phase 62 precedent** (lines 127-130):
```go
// Phase 62 P02: scheduler debounce + full-recompute thresholds (D-08).
"semantic_index.pagerank.repair_debounce_ms":       2000,
"semantic_index.pagerank.full_recompute_idle_ms":   60000,
"semantic_index.pagerank.full_recompute_threshold": float64(0.25),
```

**Phase 63 mirror** (insert immediately after the `live_updates.*` block at line 96 OR as a new block below `pagerank.*`):
```go
// Phase 63: maintenance.* — config-gated VACUUM cadence (D-05).
"semantic_index.maintenance.vacuum_enabled":  false,    // P63 D-05
"semantic_index.maintenance.vacuum_interval": "168h",   // P63 D-05 (1 week)
```
Both keys flow through the standard 4-layer koanf precedence chain.

---

### `internal/semantic/config.go` (additive: `MaintenanceConfig` struct + field)

**Analog:** `LiveUpdatesConfig` (`config.go:142-164`).

**Sibling-struct pattern:**
```go
// MaintenanceConfig holds compaction maintenance settings (P63).
// Field set mirrors SPEC §25.maintenance.* verbatim.
type MaintenanceConfig struct {
    // VacuumEnabled gates the config-gated weekly VACUUM (default false).
    VacuumEnabled bool `koanf:"vacuum_enabled"`
    // VacuumInterval is the human-readable duration between VACUUM runs
    // (default "168h" / 1 week). Parsed via time.ParseDuration in the
    // compactor; values <= 0 disable VACUUM.
    VacuumInterval string `koanf:"vacuum_interval"`
}
```

**Field on `Config`** (insert after `TypeResolution` field at line 69; mirror the alphabetical-or-thematic ordering already present):
```go
// Maintenance configures compaction-time maintenance ops (P63 D-05).
Maintenance MaintenanceConfig `koanf:"maintenance"`
```

---

### `internal/obs/metrics.go` (additive: `helix_semantic_compaction_duration_seconds`, `helix_semantic_vacuum_duration_seconds`)

**Analog:** `LSPEnrichmentDurationVec` registration (lines 313-322) + `SemanticGraphRepairInc` helper (lines 765-777).

**HistogramVec registration pattern** (lines 313-322):
```go
LSPEnrichmentDurationVec: prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "helix_semantic_lsp_enrichment_duration_seconds",
        Help: "LSP enrichment-worker per-file cascade duration in seconds. Phase 61 P03.",
        Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 20.0, 30.0},
    },
    []string{"language"},
),
```

**Phase 63 mirror:**
```go
// Phase 63: compaction + vacuum duration histograms.
SemanticCompactionDurationVec: prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "helix_semantic_compaction_duration_seconds",
        Help: "Compaction cycle duration by outcome (success/partial/skipped_blocked/error). Phase 63 D-01.",
        // 1ms → 60s — partial-skipped is microseconds; success on a small
        // overlay is ~50ms; large compaction is the tail.
        Buckets: []float64{0.001, 0.005, 0.025, 0.1, 0.5, 1.0, 5.0, 15.0, 30.0, 60.0},
    },
    []string{"outcome"},
),
SemanticVacuumDurationVec: prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "helix_semantic_vacuum_duration_seconds",
        Help: "VACUUM cycle duration by outcome (success/skipped/error). Phase 63 D-05.",
        Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 15.0, 60.0, 300.0},
    },
    []string{"outcome"},
),
```

**MustRegister site** (lines 392-422 — add to the existing batch):
```go
m.SemanticCompactionDurationVec,
m.SemanticVacuumDurationVec,
```

**Drop-on-unknown helper** (mirror `SemanticGraphRepairInc` at lines 765-777):
```go
var compactionOutcomes = map[string]struct{}{
    "success":         {},
    "partial":         {},
    "skipped_blocked": {},
    "error":           {},
}

var vacuumOutcomes = map[string]struct{}{
    "success": {},
    "skipped": {},
    "error":   {},
}

func (m *Metrics) SemanticCompactionObserve(outcome string, seconds float64) {
    if seconds < 0 {
        return
    }
    if _, ok := compactionOutcomes[outcome]; !ok {
        return
    }
    m.SemanticCompactionDurationVec.WithLabelValues(outcome).Observe(seconds)
}

func (m *Metrics) SemanticVacuumObserve(outcome string, seconds float64) {
    if seconds < 0 {
        return
    }
    if _, ok := vacuumOutcomes[outcome]; !ok {
        return
    }
    m.SemanticVacuumDurationVec.WithLabelValues(outcome).Observe(seconds)
}
```

**Cardinality test** (extend the existing `metrics_labels_test.go` to assert `outcome` ∈ closed enum, mirroring how `SemanticGraphRepairInc` is tested).

---

### `internal/daemon/daemon.go` + new `internal/daemon/compact_wiring.go` (wiring)

**Analog:** `internal/daemon/rank_wiring.go` (`rankBundle` + `ensureScheduler` lazy registry).

**Bundle struct + lazy-registry pattern** (rank_wiring.go:37-110):
```go
type rankBundle struct {
    cfg     graphpkg.SchedulerConfig
    engine  *graphpkg.Engine
    store   graphpkg.SchedulerStore
    logger  *slog.Logger
    metrics graphpkg.MetricsSink

    notifyCh chan graphpkg.GraphVersionAdvance

    runCtxMu sync.Mutex
    runCtx   context.Context

    mu   sync.Mutex
    subs map[string]*graphpkg.RankScheduler
}
```

**Phase 63 mirror** (`internal/daemon/compact_wiring.go`):
```go
type compactBundle struct {
    cfg     compact.Config
    store   *semanticstore.Store
    gate    *compact.CompactionGate
    logger  *slog.Logger
    metrics compact.MetricsSink

    runCtxMu sync.Mutex
    runCtx   context.Context

    mu       sync.Mutex
    subs     map[string]*compact.Compactor   // repoID → compactor
}

func newCompactBundle(...) *compactBundle { ... }

func (b *compactBundle) Run(ctx context.Context) error {
    if b == nil {
        <-ctx.Done()
        return ctx.Err()
    }
    b.runCtxMu.Lock()
    b.runCtx = ctx
    b.runCtxMu.Unlock()
    <-ctx.Done()
    return ctx.Err()
}

func (b *compactBundle) ensureCompactor(_ context.Context, repoID string) *compact.Compactor {
    // exact mirror of rank_wiring.go:154-185 — lock map, return cached or
    // construct fresh + go c.Run(runCtx)
}

// OnCoalescerFlush is the post-flush hook the live bundle's flush body calls.
func (b *compactBundle) OnCoalescerFlush(ws workspace.WorkspaceKey) {
    if b == nil {
        return
    }
    c := b.ensureCompactor(context.Background(), ws.RepoRoot)
    c.OnFlush()
}
```

**SetActivateCallback hook** (insert into `daemon.go:589-637` after the rank scheduler block at line 629-631):
```go
// Phase 63: per-workspace compactor lifecycle. Mirrors RankScheduler
// ownership (lazy-construct on activation; goroutine joined via daemon
// shutdown ctx).
if compactBundle != nil {
    compactBundle.ensureCompactor(ctx, repoPath)
}
```

**Errgroup goroutine** (insert into `daemon.go:751-777` alongside `d.live.Run`/`d.rank.Run`):
```go
if d.compact != nil {
    g.Go(func() error {
        return d.compact.Run(gctx)
    })
}
```

**Coalescer→compactor flush hook** (modify `internal/semantic/live/coalescer/coalescer.go:makeFlush` OR add a `Coalescer.SetOnFlush(func())` setter):
```go
// New setter on Coalescer (Phase 63 additive):
func (c *Coalescer) SetOnFlush(fn func()) {
    c.onFlush = fn
}

// In makeFlush, after dispatch loop completes:
if c.onFlush != nil {
    c.onFlush()
}
```
The daemon wires it during `buildLiveBundle` (or post-construction) — `coalescer.SetOnFlush(func() { compactBundle.OnCoalescerFlush(ws) })`.

---

### `internal/semantic/store/migrations.go` (optional `applyMigration004` for `last_vacuum_at`)

**Analog:** `applyMigration003` + `schema3Statements` (`migrations.go:472-535`).

**Migration function pattern:**
```go
// applyMigration004 adds the last_vacuum_at column on
// semantic_live_overlay_meta (Phase 63 D-05).
//
// DEFAULT NULL — DuckDB rejects NOT NULL with non-constant DEFAULT
// (see applyMigration003 doc and Landmine 7).
//
// MigrationKind=InPlace — runs at Open time, no reindex.
func applyMigration004(ctx context.Context, db *sql.DB) error {
    stmts := schema4Statements()
    for i, stmt := range stmts {
        if _, err := db.ExecContext(ctx, stmt); err != nil {
            return fmt.Errorf("applyMigration004: stmt %d (%s): %w", i+1, firstLine(stmt), err)
        }
    }
    return nil
}

func schema4Statements() []string {
    return []string{
        `ALTER TABLE semantic_live_overlay_meta ADD COLUMN last_vacuum_at TIMESTAMP DEFAULT NULL`,
        `INSERT INTO semantic_schema_version (version, applied_at) VALUES (4, now())`,
    }
}
```

**Registry entry:** wherever the existing migration registry maps `from→to→fn` (find the `Migration{From: 2, To: 3, Kind: InPlace, Fn: applyMigration003}` style entry — add `{From: 3, To: 4, Kind: InPlace, Fn: applyMigration004}`).

---

### `cmd/vet-compact-uses-store/main.go` + `internal/lint/compactusesstore/analyzer.go` (OPTIONAL)

**Analog:** `cmd/vet-noduckdb/main.go` (11 LOC) + `internal/lint/noduckdb/analyzer.go` (41 LOC).

**Singlechecker shell pattern** (`cmd/vet-noduckdb/main.go`):
```go
package main

import (
    "github.com/agenthands/helix/internal/lint/compactusesstore"
    "golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(compactusesstore.Analyzer) }
```

**Boundary-analyzer pattern** (mirror `internal/lint/noduckdb/analyzer.go:22-41`):
```go
package compactusesstore

import (
    "strings"
    "golang.org/x/tools/go/analysis"
)

const compactPkgPrefix = "github.com/agenthands/helix/internal/semantic/compact"
const allowedStorePrefix = "github.com/agenthands/helix/internal/semantic/store"
const forbiddenImport = "github.com/duckdb/duckdb-go"

var Analyzer = &analysis.Analyzer{
    Name: "compactusesstore",
    Doc:  "fails if internal/semantic/compact imports duckdb-go directly; must go through internal/semantic/store",
    Run: func(pass *analysis.Pass) (interface{}, error) {
        if !strings.HasPrefix(pass.Pkg.Path(), compactPkgPrefix) {
            return nil, nil
        }
        for _, file := range pass.Files {
            for _, imp := range file.Imports {
                path := strings.Trim(imp.Path.Value, `"`)
                if path == forbiddenImport || strings.HasPrefix(path, forbiddenImport+"/") {
                    pass.Reportf(imp.Pos(),
                        "internal/semantic/compact may not import %s directly; route through %s",
                        forbiddenImport, allowedStorePrefix)
                }
            }
        }
        return nil, nil
    },
}
```

**Note:** CONTEXT.md `canonical_refs` calls this OPTIONAL ("if the planner judges it warranted"). Phase 63 already gets DuckDB-isolation from `cmd/vet-noduckdb` — the new analyzer adds belt-and-braces protection specific to the compact→store boundary. RESEARCH.md does not insist on it. Recommendation: ship it for symmetry with `vet-nokernel2semantic`.

---

## Shared Patterns

### Per-workspace ownership (every new goroutine)
**Source:** Phase 60 coalescer + Phase 62 RankScheduler precedent
**Apply to:** `Compactor`, `compactBundle.subs` registry
**Excerpt** (`rank_wiring.go:154-185`):
```go
func (b *rankBundle) ensureScheduler(_ context.Context, repoID string) *graphpkg.RankScheduler {
    b.mu.Lock()
    defer b.mu.Unlock()
    if s, ok := b.subs[repoID]; ok {
        return s
    }
    runCtx := b.runCtx
    if runCtx == nil {
        runCtx = context.Background()
    }
    s := graphpkg.NewRankScheduler(repoID, b.cfg, b.store, b.logger, b.metrics)
    b.subs[repoID] = s
    go func() {
        if err := s.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
            b.logger.Warn("rank scheduler exited", "repo_id", repoID, "err", err)
        }
    }()
    return s
}
```
Phase 63 `compactBundle.ensureCompactor` is the byte-by-byte mirror.

### Constructor injection (no init() / no globals)
**Source:** Phase 59 D-02
**Apply to:** `compact.NewCompactor`, `compact.NewCompactionGate`, all accessor implementations
**Excerpt** (`coalescer.go:97-127`): every dependency passes through the constructor; nil-defaults applied internally. No package-level `init()` registration in the compact package.

### Closed-enum + drop-on-unknown helper (every metric)
**Source:** Phase 57 D-07; Phase 60 D-07; Phase 61 P03; Phase 62 P02
**Apply to:** `SemanticCompactionObserve`, `SemanticVacuumObserve`
**Excerpt** (`metrics.go:765-777`):
```go
var graphRepairOutcomes = map[string]struct{}{
    "applied":           {},
    "frontier_overflow": {},
    "preempted":         {},
    "error":             {},
    "stub_no_data":      {},
}

func (m *Metrics) SemanticGraphRepairInc(outcome string) {
    if _, ok := graphRepairOutcomes[outcome]; !ok {
        return
    }
    m.SemanticGraphRepairVec.WithLabelValues(outcome).Inc()
}
```

### Atomic-stamp accessor (`LastFlushAt`, `LastEnqueueAt`)
**Source:** `Coalescer.drops atomic.Uint64` (`coalescer.go:87`)
**Apply to:** all five new in-memory accessors that publish "most-recent timestamp" or "current count"
**Pattern:**
```go
field atomic.Int64   // Unix nanos OR int counter
```
Read via `Load()`; write via `Store()`/`Add()`. Zero-value (`time.Time{}` or `0`) is the documented "never happened" semantics.

### Error wrapping with method-name prefix
**Source:** `internal/semantic/store/overlay.go` throughout (every error returns `fmt.Errorf("MethodName(arg): %w", err)`)
**Apply to:** every public method on `Store` (snapshot.go, OverlayRowCount), every public method on `Compactor`/`CompactionGate`
**Excerpt** (overlay.go:113-114):
```go
mu.Unlock()
return nil, fmt.Errorf("BeginOverlayTx: ensure meta row for %q: %w", repoID, err)
```

### SQL parameterization (every DDL/DML)
**Source:** `internal/semantic/store/overlay.go` ExecContext sites at lines 106, 117, 209, 237, 273, 295, 384
**Apply to:** every new SQL stmt in snapshot.go and migrations.go
**Pattern:** never string-concat repoID/path/epoch into SQL; always `?` placeholders bound to receiver fields.

### Per-feature defaults test
**Source:** Phase 57 D-09; Phase 59/60 inheritance
**Apply to:** Phase 63 must add `TestLoad_MaintenanceDefaults` next to the existing `TestLoad_*Defaults` tests in `internal/config/config_test.go`
**Pattern (typical sibling test):**
```go
func TestLoad_MaintenanceDefaults(t *testing.T) {
    cfg := loadDefaults(t)
    if cfg.SemanticIndex.Maintenance.VacuumEnabled != false { t.Errorf(...) }
    if cfg.SemanticIndex.Maintenance.VacuumInterval != "168h" { t.Errorf(...) }
}
```

### Single-tx atomicity (compaction body)
**Source:** Phase 60 D-04 (epoch increment); Phase 62 (graph_version single-bump)
**Apply to:** `Compactor.runCompaction` body
**Excerpt** (`overlay.go:80-141`'s structure transposed): one `s.db.BeginTx` call, all writes under the same `*sql.Tx`, defer-rollback on error path, explicit `tx.Commit()` at end. CHECKPOINT issued on `s.db` (NOT the tx) AFTER commit.

---

## No Analog Found

Files with no close match in the codebase (planner should use RESEARCH.md patterns or stdlib precedent):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/semantic/compact/kill_test.go` + `testdata/cmd/compact_one/main.go` | test (subprocess fixture) | n/a | No existing in-tree subprocess crash-test; use stdlib `os/exec` + sentinel-file polling per RESEARCH.md Example 3. |
| `internal/semantic/compact/longrepo_bench_test.go` | bench (local-only) | n/a | No existing semantic-package bench; closest reference is `internal/repomap/*_bench_test.go`. MEMORY.md rule "Benchmarks are local-only — never on CI" applies; document `// local-only; do NOT add to CI` in package comment. |
| `internal/semantic/compact/gate.go` (aggregator state machine) | utility | request-response | Pattern is novel for this codebase but assembly is straightforward — closed enum from `lspenrich.Lane`, constructor-injection from Phase 59 D-02, deterministic-precedence body from RESEARCH.md Pattern 2. |

---

## Metadata

**Analog search scope:**
- `internal/semantic/store/` (snapshot/overlay/migrations sister files)
- `internal/semantic/live/coalescer/` (Phase 60 per-workspace goroutine)
- `internal/semantic/live/lspqueue/` + `internal/semantic/lspenrich/` (Phase 60/61 queue + lane-aware accessor)
- `internal/semantic/graph/` (Phase 62 RankScheduler)
- `internal/daemon/` (rank_wiring, live_wiring, daemon.go bootstrap)
- `internal/config/` (defaults + nested struct)
- `internal/obs/` (HistogramVec + drop-on-unknown helper)
- `cmd/vet-noduckdb/` + `internal/lint/noduckdb/` (boundary analyzer template)

**Files scanned:** 18 distinct analogs read in full or targeted ranges. No re-reads.

**Pattern extraction date:** 2026-05-07
