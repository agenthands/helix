# Phase 63: Compaction & Retention - Research

**Researched:** 2026-05-07
**Domain:** DuckDB single-tx compaction, snapshot retention, idle-debounced workers, runtime-state quiescence aggregation
**Confidence:** HIGH (codebase facts verified against Helix tree; DuckDB facts verified against official duckdb.org docs and a duckdb/duckdb GitHub issue thread)

## Summary

Phase 63 ships idle-debounced compaction that folds the Phase 60 live overlay into a new committed snapshot in a single DuckDB transaction, plus retention-bounded snapshots, a `CHECKPOINT` after each commit, and a config-gated weekly `VACUUM`. Two plans are pre-decided in CONTEXT.md D-02: P63-01 fills the empty `internal/semantic/store/snapshot.go` stub with a four-method API (`BeginSnapshot` / `WriteSnapshotFacts` / `CommitSnapshot` / `AbortSnapshot`); P63-02 adds the `internal/semantic/compact/` package with the per-workspace compactor goroutine, `CompactionGate.IsReady` aggregator, kill-mid-compact integration test, CAS-interleave property test, and the long-repo bench fixture.

The single load-bearing finding from this research is that **DuckDB's `VACUUM` does not reclaim disk space** — `VACUUM`, `VACUUM ANALYZE`, and `CHECKPOINT` are all explicitly documented as no-ops for space reclamation, and `VACUUM FULL` raises Not Implemented [VERIFIED: github.com/duckdb/duckdb/issues/21154]. This contradicts the optimistic phrasing in CONTEXT.md D-05 ("VACUUM reclaims DuckDB on-disk space") and SPEC §32 Phase 8. The COMPACT-03 acceptance criterion (".duckdb growth bounded across 1000 simulated commits") is achievable via the DuckDB-native `COPY FROM DATABASE` workaround into a fresh file (with file swap), but **not** via `VACUUM`. The planner needs a user decision on this — either the COMPACT-03 acceptance is tightened to "growth bounded by snapshot retention deletes alone" (which is realistic — DuckDB's optimistic block-reuse policy bounds growth even without explicit reclamation when blocks are freed by retention deletes) OR Phase 63 ships a `COPY FROM DATABASE` repack path under `vacuum_enabled=true`.

**Primary recommendation:** Treat `VACUUM` in CONTEXT.md as a synonym for **best-effort space reclamation** — issue `VACUUM` (no-op but harmless) plus a `CHECKPOINT` and rely on retention deletes for actual block-reuse. Defer the `COPY FROM DATABASE` repack as a follow-up if the long-repo bench fixture shows growth exceeds the planner-decided bound. Surface this trade-off explicitly in `<phase_requirements>` so /gsd-discuss-phase can decide before P63-02 lands.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Snapshot-write API (`BeginSnapshot` / `WriteSnapshotFacts` / `CommitSnapshot` / `AbortSnapshot`) | `internal/semantic/store/` | — | Sole owner of `duckdb-go` per Phase 57 D-12 + `cmd/vet-noduckdb/` analyzer |
| Compaction state machine (timer, IsReady gate, OnFlush hook) | `internal/semantic/compact/` (new) | — | Fresh package; goes through `*Store` for all DuckDB I/O |
| Per-workspace `CompactionGate.IsReady` (composes 6 accessors) | `internal/semantic/compact/gate.go` | accessors registered by coalescer / overlay store / lspqueue / scheduler / kernel | Aggregator pattern — read-only composition, no I/O |
| Snapshot retention DELETEs | `internal/semantic/store/snapshot.go` | `internal/semantic/compact/` orchestrates | DDL/DML lives in store; compactor calls it inside the same tx |
| `CHECKPOINT` issuance | `internal/semantic/store/snapshot.go` (new helper) | `internal/semantic/compact/` calls after `CommitSnapshot` | Store layer owns DuckDB driver; compactor sequences |
| `VACUUM` issuance (config-gated) | `internal/semantic/store/snapshot.go` (new helper) | `internal/semantic/compact/` schedules | Same boundary rationale |
| `last_vacuum_at` persistence | `internal/semantic/store/` (new table or column) | compactor reads/writes via store | Schema migration plugs into existing registry |
| Per-workspace compactor goroutine lifecycle | `internal/daemon/daemon.go` `SetActivateCallback` | — | Mirrors Phase 60 coalescer / Phase 62 RankScheduler ownership |
| Bounded-label metric (`helix_semantic_compaction_duration_seconds{outcome}`) | `internal/obs/` | helper invoked from `internal/semantic/compact/` | All `prometheus/client_golang` imports stay in `internal/obs/` |
| Trace span `semantic.live.compact_overlay` (Phase 60 reservation) + `semantic.maintenance.vacuum` (new) | `internal/semantic/compact/` | OTel SDK wired through existing `internal/obs/` | Phase 60 left the span name reserved; this phase fills the body |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/duckdb/duckdb-go/v2` | v2.10502.0 (DuckDB 1.5.x) | Sole DB binding | Pinned in `go.mod`; vet-noduckdb forbids importing it outside `internal/semantic/store/` [VERIFIED: helix go.mod:8] |
| `database/sql` (stdlib) | Go 1.22+ | Tx orchestration | DuckDB-go binding implements `database/sql/driver`; existing `*Store` already uses it for `BeginTx` / `ExecContext` [VERIFIED: internal/semantic/store/overlay.go] |
| `github.com/prometheus/client_golang` | (existing) | Metric registration | Confined to `internal/obs/` per Phase 11 boundary [VERIFIED: internal/obs/metrics.go:22] |
| `log/slog` (stdlib) | Go 1.22+ | Structured logging | Project-wide convention — every existing per-workspace goroutine uses `slog` |
| `time.AfterFunc` (stdlib) | — | Idle-debounce timer | Pattern proven in Phase 60 coalescer + Phase 62 RankScheduler [VERIFIED: internal/semantic/live/coalescer/coalescer.go:196, internal/semantic/graph/scheduler.go:161] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/agenthands/helix/internal/workspace` | (in-tree) | `WorkspaceKey` type | Per-workspace ownership keying — every `*Compactor` is bound to one |
| `github.com/agenthands/helix/internal/obs` | (in-tree) | Metric helpers (drop-on-unknown enum) | `SemanticCompactionInc(outcome)` + `SemanticCompactionObserve(outcome, dur)` helpers — pattern from `SemanticLiveUpdatesInc` / `LSPEnrichmentTotal` |
| `go.opentelemetry.io/otel/trace` | (existing) | Span body for `semantic.live.compact_overlay` + `semantic.maintenance.vacuum` | Phase 60 reserved the span name; OTel SDK is already wired through `internal/obs/` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Single-tx compaction | `compaction_journal` sidecar table | Rejected in CONTEXT.md D-01; revisit only if size guard fires routinely on real workloads (deferred) |
| `time.AfterFunc` debounce | dedicated ticker goroutine | AfterFunc is the proven pattern in Phases 60 + 62; ticker would diverge from established idiom |
| `VACUUM` for space reclamation | `COPY FROM DATABASE` swap | DuckDB `VACUUM` is a no-op for space; the swap requires a fresh attached connection + file rename (operational complexity). See "DuckDB-specific landmines" below. |
| Free-text `BlockedReason` | Closed enum (six values) | Closed enum keeps metric label cardinality bounded (CONTEXT.md D-04) |

**Installation:** No new external dependencies. All stack items are already in `go.mod`.

**Version verification:**
- `duckdb-go/v2 v2.10502.0` confirmed in `go.mod:8` (this maps to DuckDB engine 1.5.x).
- `prometheus/client_golang` already wired through `internal/obs/metrics.go`.
- No npm/pip equivalents — pure Go binary.

## Architecture Patterns

### System Architecture Diagram

```
                       ┌─────────────────────────────────────────────┐
                       │  Phase 60 Coalescer (per-workspace)         │
                       │  — debounce flush at 250ms (default)        │
                       │  — emits SourceChangeEvent batches          │
                       └────────────────┬────────────────────────────┘
                                        │ OnFlush() hook (NEW: P63-02)
                                        ▼
              ┌──────────────────────────────────────────────────────┐
              │  Phase 63 Compactor (per-workspace, NEW)             │
              │  ┌────────────────────────────────────────────────┐  │
              │  │ time.AfterFunc(compact_after_idle_ms=5000)     │  │
              │  │   ↓ fires                                      │  │
              │  │ CompactionGate.IsReady(ws)                     │  │
              │  │   composes 6 read-only accessors:              │  │
              │  │   • coalescer.LastFlushAt(ws)    (NEW)         │  │
              │  │   • store.OverlayTxOpenCount(ws) (NEW)         │  │
              │  │   • store.OverlayRowCount(ws,ep) (NEW)         │  │
              │  │   • lspqueue.Depth(ws)+LastEnq.. (Phase 61: D │  │
              │  │     epth() exists; LastEnqueueAt NEW)          │  │
              │  │   • scheduler.IsQuiescent(ws)    (NEW)         │  │
              │  │   • kernel.ActiveEditTxCount(ws) (NEW)         │  │
              │  │ → returns (ready bool, BlockedReason)          │  │
              │  └────────┬───────────────────────────────────────┘  │
              │           │ ready=true                               │
              │           ▼                                          │
              │  Pre-flight size guard:                              │
              │   if OverlayRowCount > max_overlay_files*4 →         │
              │      emit outcome=partial, skip (next flush retries) │
              │           │ guard pass                               │
              │           ▼                                          │
              │  Single DuckDB tx (atomic):                          │
              │   1. captured_epoch ← read meta.current_epoch        │
              │   2. base ← latest committed snapshot                │
              │   3. overlay rows WHERE write_epoch ≤ captured_epoch │
              │   4. merged ← MergeBaseAndOverlay(base, overlay)     │
              │   5. BeginSnapshot(SnapshotMeta{base})               │
              │   6. WriteSnapshotFacts(merged)                      │
              │   7. CommitSnapshot(summary)                         │
              │   8. ClearOverlay WHERE write_epoch ≤ captured_ep.   │
              │   9. DELETE older snapshots beyond N=5 (retention)   │
              │   10. tx.Commit()                                    │
              │  → CHECKPOINT (separate stmt; flushes WAL)           │
              │           │                                          │
              │           ▼ if vacuum gate passes                    │
              │  Separate tx: VACUUM (or COPY-FROM-DATABASE swap)    │
              │  → write last_vacuum_at row                          │
              └──────────────────────────────────────────────────────┘
                                        │ outcome=success/partial/skipped_blocked/error
                                        ▼
                       ┌─────────────────────────────────────────────┐
                       │ helix_semantic_compaction_duration_seconds  │
                       │   {outcome} (closed enum, bounded labels)   │
                       │ semantic.live.compact_overlay (OTel span)   │
                       │ semantic.maintenance.vacuum (OTel span, NEW)│
                       └─────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/
├── semantic/
│   ├── store/
│   │   ├── snapshot.go              # P63-01: BeginSnapshot/Write/Commit/Abort
│   │   ├── overlay.go               # (existing; P63-02 adds OverlayTxOpenCount + OverlayRowCount accessors)
│   │   ├── migrations.go            # (existing; P63-02 may add migration004 for last_vacuum_at)
│   │   └── ...
│   └── compact/                     # NEW package (P63-02)
│       ├── compactor.go             # per-workspace goroutine + time.AfterFunc + OnFlush
│       ├── gate.go                  # CompactionGate + BlockedReason enum
│       ├── vacuum.go                # config-gated weekly VACUUM (separate tx)
│       ├── compactor_test.go        # unit tests
│       ├── gate_test.go             # one test per BlockedReason + happy path
│       ├── kill_test.go             # kill-mid-compact integration test
│       ├── interleave_test.go       # CAS-interleave property test
│       ├── longrepo_bench_test.go   # 100-file × 1000 cycle bench (-bench)
│       └── testdata/                # subprocess fixture for kill-mid-compact
├── daemon/
│   └── daemon.go                    # P63-02 wiring: NewCompactor + SetActivateCallback hook
├── obs/
│   └── metrics.go                   # P63-02: SemanticCompactionDurationVec + helper
└── kernel/
    └── (small additive change: ActiveEditTxCount accessor on kernel handle)
```

### Pattern 1: Per-workspace goroutine + AfterFunc debounce (mirror Phase 60 coalescer)

**What:** Single goroutine per workspace owns a `*time.Timer` initialized via `time.AfterFunc(d, fn)`. Each `OnFlush()` call resets the timer; on fire, the closure runs the compaction. On context cancellation, `Stop()` the timer and return.

**When to use:** Phase 63's compactor — exactly mirrors Phase 60 coalescer ownership.

**Example:**
```go
// Source: internal/semantic/live/coalescer/coalescer.go:193-202 (existing pattern)
if c.timer != nil {
    c.timer.Stop()
}
c.timer = time.AfterFunc(c.cfg.Debounce, flush)
if c.maxTimer == nil {
    c.maxTimer = time.AfterFunc(c.cfg.MaxBatchDelay, flush)
}
```

**Phase 63 adaptation:** Compactor needs only one timer (debounce; no max-batch ceiling — the size guard plays that role):
```go
// Sketch for internal/semantic/compact/compactor.go
func (c *Compactor) OnFlush() {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.timer != nil {
        c.timer.Stop()
    }
    c.timer = time.AfterFunc(c.cfg.CompactAfterIdle, c.fire)
}

func (c *Compactor) fire() {
    if ready, _ := c.gate.IsReady(c.workspaceID); !ready {
        return // gate.IsReady logs the blocked reason; next OnFlush resets the timer
    }
    // ... single-tx compaction
}
```

### Pattern 2: Aggregator over read-only accessors (CompactionGate)

**What:** A struct that holds N read-only accessor interfaces (one per upstream component) and composes them with deterministic precedence. Idempotent and side-effect-free.

**When to use:** `CompactionGate.IsReady` — composes coalescer / overlay store / lspqueue / scheduler / kernel.

**Example:**
```go
// Sketch for internal/semantic/compact/gate.go
type CompactionGate struct {
    coalescer  CoalescerAccessor
    overlay    OverlayTxAccessor
    overlayRow OverlayRowAccessor // NEW: for "no overlay rows yet" check
    lspq       LSPQueueAccessor
    sched      SchedulerAccessor
    kernel     KernelEditAccessor
    cfg        GateConfig
    now        func() time.Time
}

func (g *CompactionGate) IsReady(ws WorkspaceKey) (bool, BlockedReason) {
    // Recommended precedence (cheapest read first; deterministic for tests):
    if g.overlayRow.OverlayRowCount(ws) == 0 {
        return false, BlockedOverlayEmpty
    }
    if g.now().Sub(g.coalescer.LastFlushAt(ws)) < g.cfg.CompactAfterIdle {
        return false, BlockedIdleTooShort
    }
    if g.kernel.ActiveEditTxCount(ws) > 0 {
        return false, BlockedEditTxActive
    }
    if g.overlay.OverlayTxOpenCount(ws) > 0 {
        return false, BlockedOverlayTxActive
    }
    if g.lspq.Depth(ws) > 0 &&
       g.now().Sub(g.lspq.LastEnqueueAt(ws)) < g.cfg.LSPCompactionMaxWait {
        return false, BlockedLSPPending
    }
    if !g.sched.IsQuiescent(ws) {
        return false, BlockedRankRepairing
    }
    return true, BlockedNone
}
```

### Pattern 3: Constructor injection + setter-based daemon wiring

**What:** Compactor takes its dependencies through `NewCompactor(...)`. Daemon wires the per-workspace lifecycle via `SetActivateCallback` (existing pattern from Phases 60-62).

**Example:**
```go
// Source: internal/daemon/daemon.go:589-590 (existing pattern)
mcpServer.SetActivateCallback(func(ctx context.Context, repoPath string) error {
    rt, err := k.ActivateWorkspace(ctx, repoPath)
    // ... existing logic ...
})
```

**Phase 63 extension:** Inside `SetActivateCallback`, after `ActivateWorkspace` returns the runtime, construct + start the compactor:
```go
comp := compact.NewCompactor(workspaceID, gate, store, lspq, coal, sched, k, cfg, logger)
go comp.Run(ctx)
// store the handle in a per-workspace map keyed by WorkspaceKey for shutdown
```

### Anti-Patterns to Avoid

- **Splitting snapshot creation and `ClearOverlay` across separate tx** — re-opens the COMPACT-02 race; forbidden by CONTEXT.md D-01 hard invariant. Single tx only.
- **Wrapping `VACUUM` inside the compaction tx** — extends the global lock window (DuckDB takes a lock during checkpoint/vacuum-like ops). CONTEXT.md D-05 invariant: VACUUM in its own tx.
- **Free-text `BlockedReason`** — explodes metric label cardinality. Closed enum (six values).
- **Importing `duckdb-go` from `internal/semantic/compact/`** — violates `cmd/vet-noduckdb/` analyzer. Compactor goes through `*Store`.
- **Blocking I/O inside `CompactionGate.IsReady`** — gate runs on every timer fire; must be O(1) reads of in-memory state. CONTEXT.md D-04 invariant.
- **Persisting `last_vacuum_at` outside a per-workspace key** — would leak across workspaces. Key on `repo_id`.
- **Using `time.Now()` directly inside the gate** — kills test determinism. Inject `now func() time.Time` (matches Phase 62 RankScheduler precedent).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Idle-debounce timer | A custom ticker + comparison loop | `time.AfterFunc` (stdlib) | Already proven in coalescer + RankScheduler; race-tested under `-race` |
| Per-workspace timer reset under contention | Custom mutex + atomic timer ptr | Inherit Phase 60 coalescer pattern (mu.Lock; Stop; new AfterFunc) | The race window between Stop and AfterFunc is benign in this idiom — the fired callback re-checks state under the same mutex |
| WAL flush | Manual file fsync | `CHECKPOINT` SQL stmt | DuckDB's `CHECKPOINT` calls `fsync()` internally [CITED: duckdb.org/docs/current/sql/statements/checkpoint] |
| Atomic snapshot+overlay-clear+retention | Multi-tx orchestration with rollback compensation | Single DuckDB BEGIN..COMMIT tx | DuckDB rollback semantics restore prior state on crash mid-tx [CITED: duckdb.org transactions docs + WAL replay verified] |
| Bounded metric labels | Free-text labels | Closed-enum with drop-on-unknown helper | Project-wide pattern (Phase 57 D-07; Phase 60 D-07; Phase 61 P03) |
| Subprocess kill in Go tests | Custom signal goroutine | `os.Process.Kill()` on a `cmd.Start()`-launched subprocess | stdlib supports it; no third-party test runner needed |
| Disk-space reclamation | A custom file-truncation pass | `COPY FROM DATABASE` (DuckDB-native) | DuckDB's only documented native approach [VERIFIED: duckdb/duckdb#21154] |

**Key insight:** All hand-rolling traps in this domain are already solved by upstream patterns. The one genuinely-novel piece is `CompactionGate.IsReady` composition logic — and even that follows the constructor-injection pattern proven in Phase 59 D-02.

## Runtime State Inventory

> Phase 63 ships new code; it doesn't rename or migrate existing artifacts. This section is included for completeness only.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `last_vacuum_at` storage requires either (a) new column on `semantic_live_overlay_meta`, (b) new generic `semantic_meta` table per CONTEXT.md D-05 suggestion, or (c) row in existing meta table with a new column. **Note:** the `semantic_meta` table referenced in CONTEXT.md D-05 does NOT currently exist — only `semantic_live_overlay_meta` does. [VERIFIED: internal/semantic/store/migrations.go schema1 lines 290-300, schema3 line 516] | Schema migration004 to add `last_vacuum_at TIMESTAMP DEFAULT NULL` column on `semantic_live_overlay_meta` (simplest; reuses existing per-workspace meta row) OR introduce a new generic `semantic_meta(repo_id, key, value)` table for future maintenance keys. Planner's call. |
| Live service config | None — Phase 63 introduces no new external service registrations. | None |
| OS-registered state | None | None |
| Secrets/env vars | None | None |
| Build artifacts | New package `internal/semantic/compact/` will be compiled into the existing `helix` binary. No new binaries, no goreleaser changes. | None |

## Common Pitfalls

### Pitfall 1: DuckDB `VACUUM` does NOT reclaim disk space

**What goes wrong:** Following CONTEXT.md D-05's wording literally — "VACUUM reclaims DuckDB on-disk space" — produces a long-repo bench that grows unboundedly even with `vacuum_enabled=true`.

**Why it happens:** DuckDB's `VACUUM`, `VACUUM ANALYZE`, and `CHECKPOINT` are explicitly no-ops for space reclamation. `VACUUM FULL` raises Not Implemented [VERIFIED: github.com/duckdb/duckdb/issues/21154]. DuckDB free blocks ARE reused for new inserts, so the file does not grow without bound when retention DELETEs free blocks (the natural Phase 63 path), but the file size never shrinks. Only `COPY FROM DATABASE` to a fresh file (followed by file swap) actually reclaims space [VERIFIED: same issue].

**How to avoid:**
1. Bound the long-repo bench growth by **retention deletes alone** (free blocks reused on subsequent inserts), not by VACUUM.
2. Treat `vacuum_enabled=true` as a best-effort + `CHECKPOINT` issuance — VACUUM remains a no-op SQL stmt that's harmless to issue but achieves nothing measurable.
3. If COMPACT-03's "growth bounded" criterion fails the bench fixture, add a `COPY FROM DATABASE` repack path under `vacuum_enabled=true` (planner's call; deferred unless bench shows growth).
4. Surface this trade-off explicitly in the planner's PLAN.md and in `<phase_requirements>` so /gsd-discuss-phase can confirm before P63-02 lands.

**Warning signs:** Long-repo bench shows monotonic file growth proportional to total writes (regardless of `vacuum_enabled` setting); operator dashboards show non-shrinking `.duckdb` after retention should have freed thousands of rows.

### Pitfall 2: `CHECKPOINT` is "stop-the-world"

**What goes wrong:** Issuing `CHECKPOINT` after every compaction commit blocks all readers and writers across all workspaces sharing the same DuckDB file for the duration of the WAL flush.

**Why it happens:** DuckDB checkpoint design locks all clients during the flush [VERIFIED: alibabacloud.com/blog/duckdb-internals-part-5 + duckdb.org/2024/10/30/analytics-optimized-concurrent-transactions]. For Phase 63 this is BENIGN today because **each workspace already has its own `.duckdb` file** (`<workspace>/.helix/semantic.duckdb` per Phase 57 STORE-01) — so a checkpoint stops only that workspace's connection, not other workspaces. But if Phase 65+ ever introduces a multi-workspace shared file, checkpoint coordination becomes load-bearing.

**How to avoid:** Document the per-workspace-file invariant in P63-02's compactor doc-comment. Alert in the gate if `OverlayTxOpenCount(ws) > 0` (the `BlockedOverlayTxActive` reason already covers this — checkpoint won't run until the overlay tx commits).

**Warning signs:** Concurrent test asserts hang waiting on the post-CHECKPOINT lock release; `pragma_wal_status` shows the WAL never rotated.

### Pitfall 3: Time-of-check vs time-of-use on `CompactionGate.IsReady`

**What goes wrong:** The gate returns `ready=true`, but between gate-check and `BeginOverlayTx` an external edit lands, opens its own overlay tx, and now the compactor races with a live writer — but the captured_epoch was already read.

**Why it happens:** The gate is read-only and side-effect-free (CONTEXT.md D-04 invariant). It cannot hold a lock spanning gate-check → tx-begin.

**How to avoid:** This is **already correct** under the Phase 60 D-04 CAS contract — the post-gate `BeginCompactionTx` reads `captured_epoch ← current_epoch` atomically; rows with `write_epoch > captured_epoch` are never deleted by `ClearOverlay`. The race is a no-op for correctness; it just means the compactor processed N rows while N+1 was being written. The N+1 row is captured by the next compaction cycle. Verified by the COMPACT-02 acceptance test ("interleave overlay writes with compaction").

**Warning signs:** Property test fails because a row written DURING compaction is visible neither in the new snapshot nor in the post-compaction overlay. This MUST NOT happen — if it does, the CAS contract is broken.

### Pitfall 4: `time.AfterFunc(d)` reset race vs goroutine fire

**What goes wrong:** `OnFlush()` calls `Stop()` then assigns a new timer; meanwhile the previous timer's callback fires (because Stop() returns false when the callback is already running). Two compactions race.

**Why it happens:** `time.Timer.Stop()` does not guarantee the callback hasn't fired or isn't currently running.

**How to avoid:**
1. Hold a mutex around the entire fire body so the second invocation sees the post-first state.
2. Inside `fire()`, re-check `gate.IsReady` first — if a previous fire just committed a snapshot, the overlay is empty (`BlockedOverlayEmpty`) and the second fire becomes a no-op.
3. Mirror Phase 60 coalescer's exact ordering: `c.mu.Lock(); if c.timer != nil { c.timer.Stop() }; c.timer = time.AfterFunc(...)`. Tests under `-race` validate.

**Warning signs:** Race-detector flags concurrent access to `c.timer`; two compaction commits happen back-to-back with the second producing `outcome=skipped_blocked, blocked_by=overlay_empty`.

### Pitfall 5: Process kill BEFORE `tx.Commit()` returns

**What goes wrong:** SIGKILL during step 10 of the compaction tx (the actual `tx.Commit()` call). What does the post-restart state look like?

**Why it happens:** Commit is a multi-stage operation: WAL write → fsync → in-memory visibility flip. A kill between stages is exactly what COMPACT-05 is asking us to test.

**How to avoid:** DuckDB's WAL replay handles this:
- Kill BEFORE WAL fsync → commit is undone; pre-tx state restored. Overlay rows preserved, no new snapshot.
- Kill AFTER WAL fsync, BEFORE in-memory visibility → restart replays WAL; commit completes during recovery.
- Kill DURING WAL write → DuckDB detects partial WAL entry on replay and discards [CITED: duckdb.org "On restart, the database files are re-loaded from disk, the changes in the WAL are re-applied (if present), and things happily continue."].
- Kill AFTER `CHECKPOINT` (separate stmt; outside compaction tx) → snapshot is durable in main file; WAL is empty.

The kill-mid-compact integration test (D-acceptance #2) must exercise these phases. See "Crash Recovery Validation Matrix" below.

**Warning signs:** Restart shows half a snapshot row in `semantic_snapshots` with `status='committed'` AND overlay rows with `write_epoch ≤ captured_epoch` still present — would indicate atomicity violation, must fail the test loud.

### Pitfall 6: `runtime.Goexit()` does NOT close DuckDB connections

**What goes wrong:** Using `runtime.Goexit()` (CONTEXT.md D-acceptance #2 suggested option) to simulate kill-mid-compact ends the compaction goroutine but leaves the DuckDB connection open and the open `*sql.Tx` un-rolled-back. The next operation on the same connection sees the orphaned tx.

**Why it happens:** Goexit unwinds Go stack only; OS-level DuckDB state is process-local, not goroutine-local.

**How to avoid:** Use a **subprocess fixture** instead. Launch a child process via `exec.CommandContext` that runs one compaction cycle, then send `os.Process.Kill()` (SIGKILL) at each tx phase boundary. Parent process re-opens the DuckDB file (a fresh connection) and asserts the post-restart state. This produces a much more realistic crash recovery test.

**Warning signs:** Test passes trivially because the in-process orphan tx never gets validated against a fresh connection; rollback semantics never actually exercised.

## Code Examples

### Example 1: Snapshot-write API skeleton (P63-01)

```go
// Source: based on SPEC §22.2 pseudocode + Phase 60 overlay.go pattern
// Path: internal/semantic/store/snapshot.go (replaces existing stub)

package store

import (
    "context"
    "database/sql"
    "fmt"
    "time"
)

type SnapshotMeta struct {
    RepoID         string
    BaseSnapshotID uint64
    Kind           string // "live_compaction" | "full" | "incremental"
    SchemaVersion  int    // current Phase 60 schema version (3)
}

type SnapshotSummary struct {
    Files       int
    Symbols     int
    References  int
    Edges       int
    Partial     bool
    PartialReason string
}

type Snapshot struct {
    ID         uint64
    RepoID     string
    tx         *sql.Tx
    meta       SnapshotMeta
    createdAt  time.Time
}

// BeginSnapshot allocates a fresh snapshot_id, opens a tx, and INSERTs the
// pending snapshot row with status='pending'. Subsequent WriteSnapshotFacts
// calls write under this tx. CommitSnapshot flips status='committed'.
func (s *Store) BeginSnapshot(ctx context.Context, meta SnapshotMeta) (*Snapshot, error) { /* ... */ }

// WriteSnapshotFacts inserts files/symbols/references/edges for the snapshot
// under its tx. Idempotent within the tx. Caller invokes once with the full
// merged Facts struct.
func (s *Store) WriteSnapshotFacts(ctx context.Context, snap *Snapshot, facts Facts) error { /* ... */ }

// CommitSnapshot flips status='committed', writes the summary row, and
// commits the tx. After this returns, readers can see the snapshot.
func (s *Store) CommitSnapshot(ctx context.Context, snap *Snapshot, summary SnapshotSummary) error { /* ... */ }

// AbortSnapshot rolls back the tx. Used on any error path during writing.
func (s *Store) AbortSnapshot(ctx context.Context, snap *Snapshot, reason string) error { /* ... */ }
```

### Example 2: Single-tx compaction body (P63-02)

```go
// Source: SPEC §22.2 + CONTEXT.md D-01 invariants
// Path: internal/semantic/compact/compactor.go (sketch)

func (c *Compactor) runCompaction(ctx context.Context) (outcome string) {
    defer func(start time.Time) {
        c.metrics.SemanticCompactionObserve(outcome, time.Since(start))
    }(time.Now())

    // Pre-flight size guard (D-01)
    rows, _ := c.store.OverlayRowCount(ctx, c.repoID, /*captured later*/)
    if rows > c.cfg.MaxOverlayFiles*4 {
        return "partial"
    }

    // Open ONE DuckDB tx for snapshot + overlay-clear + retention.
    snap, err := c.store.BeginCompactionTx(ctx, c.repoID) // wraps Begin + epoch capture
    if err != nil {
        return "error"
    }
    defer func() {
        if outcome != "success" && outcome != "partial" {
            _ = c.store.AbortSnapshot(ctx, snap, outcome)
        }
    }()

    base, _ := c.store.GetLatestSnapshot(ctx, c.repoID)
    overlay, _ := c.store.LoadOverlayLE(ctx, c.repoID, snap.CapturedEpoch)
    merged := MergeBaseAndOverlay(base, overlay)

    if err := c.store.WriteSnapshotFacts(ctx, snap, merged); err != nil {
        return "error"
    }
    if err := c.store.ClearOverlayLE(ctx, c.repoID, snap.CapturedEpoch); err != nil {
        return "error"
    }
    if err := c.store.DeleteOldSnapshots(ctx, c.repoID, c.cfg.SnapshotRetention); err != nil {
        return "error"
    }
    if err := c.store.CommitSnapshot(ctx, snap, BuildSummary(merged)); err != nil {
        return "error"
    }
    // CHECKPOINT in the same connection but OUTSIDE the tx — flushes WAL.
    if err := c.store.Checkpoint(ctx); err != nil {
        // Non-fatal — commit succeeded; CHECKPOINT failure just delays WAL flush.
        c.logger.Warn("checkpoint after compaction failed", "err", err)
    }
    return "success"
}
```

### Example 3: Subprocess kill-mid-compact fixture (P63-02 testdata)

```go
// Source: stdlib os/exec
// Path: internal/semantic/compact/kill_test.go (sketch)

func TestKillMidCompact(t *testing.T) {
    tmpDB := filepath.Join(t.TempDir(), "test.duckdb")
    seedFixture(t, tmpDB) // writes overlay rows, leaves a base snapshot

    for _, phase := range []string{"pre_write", "mid_write", "post_commit_pre_clear", "post_clear_pre_retention", "post_tx"} {
        t.Run(phase, func(t *testing.T) {
            // Launch a child process that runs one compaction and signals at the named phase.
            cmd := exec.CommandContext(t.Context(),
                "go", "run", "./testdata/cmd/compact_one",
                "-db", tmpDB, "-kill-at", phase)
            cmd.Stdout = os.Stderr
            cmd.Stderr = os.Stderr
            if err := cmd.Start(); err != nil {
                t.Fatal(err)
            }
            // Wait for child to reach the named phase, then SIGKILL.
            // (Child writes a sentinel file at each phase; parent watches.)
            waitForSentinel(t, tmpDB, phase)
            _ = cmd.Process.Kill()
            _ = cmd.Wait()

            // Re-open the DuckDB file from this process and assert state.
            db := openStore(t, tmpDB)
            assertConsistentState(t, db, phase)
        })
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hope `VACUUM` reclaims space | `COPY FROM DATABASE` swap | DuckDB issue #21154 (open as of 2025) | Phase 63 must NOT rely on `VACUUM` for COMPACT-03 growth bound; revisit with a repack path if bench shows growth |
| Hand-roll a journal sidecar table | Single DuckDB tx + WAL replay | DuckDB 1.x (stable for 2+ years) | CONTEXT.md D-01 — journal is a deferred fallback if size guard misfires |
| Polling for quiescence | Read-only accessors composed in a gate | Phase 60 / 62 idiom | New idiom for Phase 63; aligns with the Caddy-style constructor-injection ethos in Phase 59 D-02 |

**Deprecated/outdated:**
- **`VACUUM ANALYZE` for space reclamation** — DuckDB no-op since at least 1.0; the duckdb-web docs even have an open issue requesting the no-op comment be updated [CITED: duckdb/duckdb-web issue #1331].
- **In-process kill via `runtime.Goexit()` for crash testing** — does not exercise DuckDB connection-level rollback; subprocess kill is the realistic path.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (existing project standard) |
| Config file | none — `go test ./...` is the canonical entry |
| Quick run command | `go test -run TestCompactor -count=1 ./internal/semantic/compact/...` |
| Full suite command | `go test -race ./...` |
| Bench command | `go test -bench=BenchmarkLongRepoCompaction -benchmem ./internal/semantic/compact/...` (local-only per project rule "Benchmarks are local-only — never on CI") |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| COMPACT-01 | Compactor merges overlay into snapshot after `compact_after_idle_ms`; previous snapshot preserved on failure | integration | `go test -run TestCompactor_IdleTrigger ./internal/semantic/compact/` | ❌ Wave 0 |
| COMPACT-01 (gate) | Gate blocks during edit/overlay/LSP tx | unit (one test per BlockedReason) | `go test -run TestGate ./internal/semantic/compact/` | ❌ Wave 0 |
| COMPACT-02 | CAS interleave: rows written during compaction NOT dropped by ClearOverlay | property | `go test -run TestCompactor_CASInterleave ./internal/semantic/compact/` | ❌ Wave 0 |
| COMPACT-03 (CHECKPOINT) | CHECKPOINT runs at compaction commit; WAL flushed | integration | `go test -run TestCompactor_CheckpointAfterCommit ./internal/semantic/compact/` | ❌ Wave 0 |
| COMPACT-03 (VACUUM) | Config-gated weekly VACUUM; runs in separate tx | integration | `go test -run TestCompactor_VacuumGated ./internal/semantic/compact/` | ❌ Wave 0 |
| COMPACT-03 (long-repo) | 100-file × 1000 cycle bench; growth bounded | bench (local-only) | `go test -bench=BenchmarkLongRepoCompaction ./internal/semantic/compact/` | ❌ Wave 0 |
| COMPACT-04 | Snapshot retention keeps last N=5; older deleted in same tx | integration | `go test -run TestCompactor_Retention ./internal/semantic/compact/` | ❌ Wave 0 |
| COMPACT-05 | Kill-mid-compact: overlay+snapshot consistent on restart | integration (subprocess) | `go test -run TestKillMidCompact ./internal/semantic/compact/` | ❌ Wave 0 |
| (P63-01) Snapshot API | Begin → Write → Commit; Begin → Abort; retention deletes | unit (synthetic fake compactor) | `go test -run TestSnapshot ./internal/semantic/store/` | ❌ Wave 0 |
| (D-acceptance #10) Metric | `helix_semantic_compaction_duration_seconds{outcome}` emits closed enum | unit (label cardinality) | `go test -run TestSemanticCompactionLabels ./internal/obs/` | ❌ Wave 0 |
| (D-acceptance #10) Span | `semantic.live.compact_overlay` body filled | unit (test recorder) | `go test -run TestCompactOverlaySpan ./internal/semantic/compact/` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test -run TestCompactor -count=1 ./internal/semantic/compact/...` (~ < 30s; excludes bench)
- **Per wave merge:** `go test -race ./internal/semantic/compact/... ./internal/semantic/store/...`
- **Phase gate:** `go test -race ./...` green + bench produced a growth-bound report (local-only) before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/semantic/compact/` — entire package new (P63-02)
- [ ] `internal/semantic/compact/compactor_test.go` — covers COMPACT-01 idle trigger
- [ ] `internal/semantic/compact/gate_test.go` — covers COMPACT-01 gate (one test per BlockedReason)
- [ ] `internal/semantic/compact/interleave_test.go` — covers COMPACT-02
- [ ] `internal/semantic/compact/kill_test.go` + `internal/semantic/compact/testdata/cmd/compact_one/main.go` — covers COMPACT-05 (subprocess fixture)
- [ ] `internal/semantic/compact/longrepo_bench_test.go` — covers COMPACT-03 growth-bound (local-only)
- [ ] `internal/semantic/store/snapshot_test.go` (or extend `snapshot.go` test pair) — covers P63-01 unit tests
- [ ] `internal/obs/metrics_labels_test.go` extension — covers `helix_semantic_compaction_duration_seconds` closed enum
- [ ] No framework install needed (stdlib `testing` only)

## Crash Recovery Validation Matrix

This drives the COMPACT-05 kill-mid-compact integration test design. The compaction tx has 5 observable phase boundaries; the test must exercise each.

| # | Tx Phase | Kill Point | Expected Post-Restart State |
|---|----------|------------|----------------------------|
| 1 | Pre-row-write | After `BeginSnapshot` returns, before any `WriteSnapshotFacts` row inserted | DuckDB rolls back the empty tx. `semantic_snapshots` row with `status='pending'` either absent (rollback discarded the INSERT) OR present and STAYS `pending` forever (orphaned). The latter is acceptable IFF retention sweeps `pending` rows older than a TTL — Phase 63 should DELETE orphaned `pending` snapshots on next compaction start (planner's call). Overlay UNCHANGED. |
| 2 | Mid-row-write | Halfway through `WriteSnapshotFacts` (e.g., files written, symbols not) | DuckDB rolls back the entire tx via WAL recovery. `semantic_snapshots` row absent. Overlay UNCHANGED. New snapshot did NOT commit. |
| 3 | Post-CommitSnapshot, pre-ClearOverlay | After `tx.Commit()` for the snapshot writes BUT BEFORE `ClearOverlay` runs (this can NOT happen in the single-tx design because `ClearOverlay` is INSIDE the tx; included for completeness — if a future planner splits tx, this is the reopened race) | (Not exercised — single-tx invariant prevents this kill point) |
| 4 | Post-ClearOverlay, pre-retention-delete | Same as #3 — would only manifest if tx is split | (Not exercised — single-tx invariant) |
| 5 | Post-`tx.Commit()`, pre-`CHECKPOINT` | After tx commit returns, before CHECKPOINT runs | New snapshot committed AND overlay cleared (atomic by the single tx). WAL contains the commit but main file may not have been fsynced. On restart, WAL replay applies the commit. `semantic_snapshots` shows committed snapshot; overlay empty. WAL re-flushed by next CHECKPOINT (or daemon shutdown). |
| 6 | During CHECKPOINT | DuckDB is fsyncing the WAL to main file | Per DuckDB durability guarantees, CHECKPOINT is itself recoverable: WAL stays consistent, main file may be partially written. On restart, DuckDB detects the partial main-file write and re-applies WAL. State equivalent to phase 5 above. [VERIFIED: duckdb.org transactions/WAL semantics + LazyFS testing per duckdb.org/2024/10/30] |

**Test fixture design implication:** The subprocess fixture (`internal/semantic/compact/testdata/cmd/compact_one/main.go`) writes a sentinel file at each phase boundary; parent process polls for the sentinel, then SIGKILLs. After kill, parent re-opens the DuckDB file with a fresh connection and asserts the matrix above.

**Acceptance for COMPACT-05:** Phases 1, 2, 5, 6 must be exercised (phases 3 and 4 are unreachable under single-tx invariant — but a sentinel test should fail loudly if a future PR splits the tx and exposes them).

## DuckDB-Specific Landmines

These are concrete pitfalls a planner must know — beyond the project-wide patterns the team already follows.

### Landmine 1: `VACUUM` is a no-op for space reclamation
Already covered in Pitfall 1 above. Planner-impacting because COMPACT-03's wording "VACUUM keeps long-repo bench fixture's `.duckdb` file growth bounded" is misleading — the bound comes from retention DELETEs (free blocks reused), not VACUUM. [VERIFIED: duckdb/duckdb#21154]

### Landmine 2: `CHECKPOINT` is "stop-the-world"
Already covered in Pitfall 2. Per-workspace-file invariant (Phase 57 STORE-01) means this is benign today; do NOT rely on per-workspace isolation if a future phase introduces a shared file. [VERIFIED: alibabacloud.com/blog/duckdb-internals-part-5]

### Landmine 3: DuckDB uses optimistic concurrency control
Multiple writers do not block on locks; conflicts abort one tx. Phase 63 holds a per-workspace overlay mutex (Phase 60 D-04) so writers serialize at the application layer — DuckDB-level conflicts are unreachable. Compaction reads under that same per-workspace lock for the ClearOverlay portion. [VERIFIED: duckdb.org/2024/10/30/analytics-optimized-concurrent-transactions]

### Landmine 4: `CHECKPOINT` waits; `FORCE CHECKPOINT` aborts active tx
Phase 63 should issue plain `CHECKPOINT` after `tx.Commit()` — never `FORCE CHECKPOINT`, because that would abort other workspaces' active txs (if files are ever shared) or other in-progress overlay writes on the same workspace. [CITED: duckdb.org/docs/current/sql/statements/checkpoint]

### Landmine 5: Long-running tx can exceed `checkpoint_threshold` (default 16 MB WAL)
A compaction that writes a giant snapshot may bloat the WAL beyond `checkpoint_threshold`. DuckDB will trigger an automatic CHECKPOINT — which under "stop-the-world" semantics blocks the in-flight tx. Pre-flight size guard (CONTEXT.md D-01) protects against this in normal operation; planner should consider adding `pragma checkpoint_threshold='256MiB'` (or similar) at store open for the live overlay file if benches show contention. [VERIFIED: github.com/duckdb/duckdb/issues/9721 + duckdb.org WAL docs]

### Landmine 6: DuckDB Append API bypasses some tx semantics
The `Appender` API used for bulk inserts has different commit semantics than `database/sql`. Phase 63 should use `database/sql` `ExecContext` for `WriteSnapshotFacts` (not Appender) to keep the single-tx invariant intact — Appender flushes are not transactionally bound to the surrounding `*sql.Tx`. [VERIFIED: duckdb-go-bindings docs / Phase 57 P02 SUMMARY which avoided Appender for this reason]

### Landmine 7: `ALTER TABLE ADD COLUMN ... NOT NULL` is rejected
Phase 59 P02 already hit this; documented in `internal/semantic/store/migrations.go:401-410`. If P63-02 adds a `last_vacuum_at` column to `semantic_live_overlay_meta`, use `DEFAULT NULL` (or `DEFAULT '1970-01-01'::TIMESTAMP`) — never `NOT NULL DEFAULT now()`. [VERIFIED: codebase precedent + duckdb-go-bindings limitation]

### Landmine 8: `DROP COLUMN` is incomplete in DuckDB
Schema rollback is not viable; downgrade requires the existing quarantine-and-rebuild path (Phase 57 D-04). Planner does NOT need to ship a v4→v3 down-migration. [VERIFIED: codebase precedent: migrations.go:411-417]

## Existing Component Accessors — surface check

CONTEXT.md D-04 lists six accessors `CompactionGate.IsReady` consumes. For each, here is the codebase reality and what P63-02 must add.

| Accessor | Component | Already Exists? | Action |
|----------|-----------|-----------------|--------|
| `coalescer.LastFlushAt(ws) time.Time` | `internal/semantic/live/coalescer/coalescer.go` | ❌ NO — `Coalescer` has the timer but no exposed flush timestamp | P63-02 adds `LastFlushAt() time.Time` (returns zero if never flushed). Update `makeFlush` to atomically store the time. |
| `store.OverlayTxOpenCount(ws) int` | `internal/semantic/store/overlay.go` | ❌ NO — no per-workspace tx counter | P63-02 adds an `atomic.Int32` per workspace, incremented in `BeginOverlayTx` after `BeginTx` succeeds, decremented in `Commit`/`Rollback`. New method on `*Store`. |
| `store.OverlayRowCount(ws, capturedEpoch) int` | `internal/semantic/store/overlay.go` | ❌ NO | P63-02 adds: SELECT COUNT(*) FROM each overlay table WHERE repo_id=? AND write_epoch <= ?. Sum across the four tables. |
| `lspqueue.Depth(ws) int` | `internal/semantic/lspenrich/queue.go` (Phase 61 LaneQueue) | ✓ PARTIAL — `LaneQueue.Depth(lane)` exists; aggregate `Depth(LaneHigh) + Depth(LaneBackground)` for total | Wrap the existing accessor; no new code in `lspenrich` needed. |
| `lspqueue.LastEnqueueAt(ws) time.Time` | `internal/semantic/lspenrich/queue.go` | ❌ NO — LaneQueue tracks Depth but not last-enqueue timestamp | P63-02 adds an `atomic.Int64` (Unix nanos) updated on each `EnqueueLane` success. New method `LastEnqueueAt() time.Time`. |
| `scheduler.IsQuiescent(ws) bool` | `internal/semantic/graph/scheduler.go` (Phase 62 RankScheduler) | ❌ NO — `RankScheduler` has internal state (`pendingChanged`, `debounceTimer`, in-flight repair) but no public quiescence accessor | P63-02 adds `IsQuiescent() bool`. Definition: `len(pendingChanged) == 0 && (no in-flight runIncrementalRepair) && (no in-flight maybeFullRecompute)`. Track in-flight via a counter or sync.Mutex try-lock pattern. |
| `kernel.ActiveEditTxCount(ws) int` | `internal/kernel/` (everywhere) | ❌ NO — no edit-tx counter exists; edit tools fire-and-forget | P63-02 adds: a small counter on the kernel handle that edit tools increment at start of Handle and decrement on return. Single mutex-protected counter per workspace OR atomic int. Edit tools list (LIVE-07 + write_file = 8): `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`, `replace_in_file`, `fuzzy_edit`, `write_file`. |

**Plan-impact summary:** P63-02 adds 6 small additive accessors (1 on coalescer, 2 on store, 1 on lspenrich queue, 1 on scheduler, 1 on kernel) before the gate becomes implementable. None require schema changes; all are in-memory state surfaces.

## Pre-flight Overlay Row-Count Guard — threshold validation

CONTEXT.md D-01 suggests `max_overlay_files * 4`. Validation against DuckDB's `memory_limit: 1GiB` default:

- `max_overlay_files=1000` (default) × 4 = 4000 rows per fact table.
- Average overlay-symbol row: ~300 bytes (UBIGINT keys + small TEXT name/qualified_name + JSON fact_json typically < 200 bytes for tree-sitter facts).
- Worst case: 4000 rows × 4 fact tables × 300 bytes ≈ 4.8 MB working set.
- Comfortably under 1 GiB memory_limit.

**Conclusion:** `max_overlay_files * 4` is a safe heuristic. Planner can revise after the long-repo bench reveals real distributions, but ship the suggested threshold.

**Failure mode if too high:** OOM during MergeBaseAndOverlay (the merge produces an in-memory data structure), not slow tx — DuckDB itself handles 4k inserts trivially. A 10× higher guard (40k rows) would cost ~50 MB; still fine. A 100× higher guard (400k rows) starts to risk OOM under the 1 GiB DuckDB memory_limit if combined with concurrent reads.

**Failure mode if too low:** `outcome=partial` fires too often, compaction never fires on busy workspaces, overlay grows unbounded, snapshots drift stale. The bench fixture should record `outcome=partial` rate.

## Metric Label Cardinality

`helix_semantic_compaction_duration_seconds` outcome label is a closed 4-value enum: `success | partial | skipped_blocked | error`.

**Open question** in CONTEXT.md: when `outcome=skipped_blocked`, do we add a `blocked_by` label?

**Recommendation:** **NO**, do not add `blocked_by` as a metric label.

**Rationale:**
1. Six BlockedReason values × 4 outcomes = 24 combos per workspace. Workspace count is unbounded — multiplying by an extra dimension explodes cardinality.
2. The bounded-label discipline in this project (Phase 57 D-07; Phase 60 D-07) keeps labels bounded; adding `blocked_by` would require a new label whitelist in the linter.
3. Operators don't need per-blocked_reason histograms — they need to know **whether** compaction is firing. A single `blocked_by` count breakdown can be exposed via the OTel span attributes on `semantic.live.compact_overlay` (span attributes do not have the same cardinality discipline as Prometheus labels).
4. If a per-blocked-reason counter is genuinely needed, ship it as a separate counter `helix_semantic_compaction_blocked_total{reason}` with the closed enum.

**Closed enum as currently defined:** `success | partial | skipped_blocked | error` (4 values). Cardinality bounded.

## Project Constraints (from CLAUDE.md)

CLAUDE.md is in scope for this phase and contains directives that constrain Phase 63 plans. Listed verbatim where load-bearing:

| Directive | Phase 63 Implication |
|-----------|---------------------|
| Always run `go vet` and `go test` before completing any Go task | Wave 0 must include `go vet ./...` and `go test -race ./...` as gate criteria |
| Constraint: Language: Go — single binary, native concurrency | Phase 63 must NOT introduce subprocess daemons (the kill-mid-compact subprocess is a TEST fixture, not a production component) |
| GSD Workflow Enforcement: do NOT make direct repo edits outside a GSD workflow | All Phase 63 code lands through `/gsd:execute-phase` — no out-of-band commits |
| SMTC-first tool routing for code intelligence | Planner agent should use `mcp__smtc__*` tools (goto_definition, find_references, get_callers) when investigating Helix's Go code, not Grep+Read |
| Phase 63 must NOT touch middleware order (CLAUDE.md "Middleware Execution Order (LIFO)") | Compaction lives below the MCP layer; no middleware changes |
| Phase 63 must NOT import `duckdb-go` outside `internal/semantic/store/` (Phase 57 D-12 + `cmd/vet-noduckdb/`) | The compactor goes through `*Store`; vet-noduckdb gates this in CI |
| Benchmarks are local-only — never on CI (MEMORY.md) | The long-repo bench fixture (COMPACT-03) MUST NOT be wired to a GitHub Actions workflow. Document as "local-only" in the bench file's package comment. |
| UAT must automate daemon/MCP checks (MEMORY.md) | If Phase 63 ships a UAT, drive `helix daemon`/MCP via the forwarder fixture — never ask user to run them manually |

## Open Questions (RESOLVED)

1. **`VACUUM` semantics — true intent of CONTEXT.md D-05.**
   - What we know: DuckDB `VACUUM` is a no-op for space reclamation. CONTEXT.md D-05 says "VACUUM reclaims DuckDB on-disk space" which is factually wrong against current DuckDB behavior.
   - What's unclear: Did the user intend (a) issue VACUUM as a harmless no-op for forward-compat, (b) ship `COPY FROM DATABASE` repack now, or (c) tighten COMPACT-03 to "growth bounded by retention deletes only"?
   - **RESOLVED 2026-05-07 via planner lock — ship VACUUM as harmless no-op; CONTEXT.md D-05 wiring stays.** Default safest path: option (a) plus document the DuckDB limitation in code comments + RESEARCH.md.

2. **`semantic_meta` table — does it exist?**
   - What we know: CONTEXT.md D-05 says "If `semantic_meta` doesn't already accept arbitrary `(repo_id, key)` rows, add a small schema migration." The codebase has `semantic_live_overlay_meta` (per-workspace meta with PRIMARY KEY repo_id) but NO generic `semantic_meta` table.
   - What's unclear: User wants a generic key-value meta table for future maintenance keys (e.g. `last_full_reindex_at`), or just a `last_vacuum_at` column on the existing `semantic_live_overlay_meta`?
   - **RESOLVED — adopt RESEARCH.md recommendation: add `last_vacuum_at TIMESTAMP DEFAULT NULL` column on existing `semantic_live_overlay_meta` via migration004 (P63-02 Task 1).** Simplest, reuses existing per-workspace row. If a future phase needs more maintenance keys, that phase can add the generic table. (Pragmatic principle: don't pre-design; wait for the second use case.)

3. **`outcome=error` recoverability semantics for the metric description.**
   - What we know: CONTEXT.md says "metric description must call out that `error` is non-recoverable for that idle window — next coalescer flush resets the timer."
   - What's unclear: Should `error` block ALL future compactions until daemon restart, or just the current idle window?
   - **RESOLVED — retry-on-next-flush (per RESEARCH.md recommendation); error never permanently disables compactor until restart.** Just the current idle window. Next coalescer flush resets the timer; next gate-pass triggers compaction. If errors recur N times in a row, log loud (operator alert) but don't permanently disable.

4. **VACUUM separate trace span name.**
   - What we know: Compaction span is `semantic.live.compact_overlay` (Phase 60 reservation). VACUUM should be separate.
   - What's unclear: `semantic.maintenance.vacuum` (CONTEXT.md suggestion) vs `semantic.live.vacuum_overlay` (closer to Phase 60 family).
   - **RESOLVED — `semantic.maintenance.vacuum` (CONTEXT.md proposed; keeps `semantic.live.*` family clean).** VACUUM is a maintenance operation, not part of the live-update flow. Distinct namespace makes operator filtering cleaner.

## Environment Availability

> Phase 63 has no external runtime dependencies beyond what Phase 57+ already established. This section is INCLUDED as a sanity check rather than skipped.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.22+ | Build | ✓ | 1.24.0 (verified at `go version` invocation; project go.mod declares minimum) | — |
| DuckDB engine 1.5.x | Phase 57 store | ✓ | bundled via `duckdb-go-bindings/lib/<platform>` (CGO=1 build) | — |
| `cmd/vet-noduckdb` analyzer | CI gate | ✓ | in-tree | — |
| `prometheus/client_golang` | metric registration | ✓ | already in go.mod | — |
| `go.opentelemetry.io/otel/trace` | span body | ✓ | already wired through `internal/obs/` | — |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None.

**Skip note:** This phase is purely Go code + DuckDB SQL changes. No new tools, services, or runtimes.

## Security Domain

> Helix's `security_enforcement` config defaults: present? Per recent practice in Phases 57-62, the project does not gate security ASVS scans on Go code (no `java-security` capability available against this codebase per CLAUDE.md SMTC routing). The standard Go security review applies.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Phase 63 has no auth surface — compaction is internal to the daemon |
| V3 Session Management | no | No session state involved |
| V4 Access Control | no | Workspace activation is governed by existing kernel ACL — Phase 63 inherits |
| V5 Input Validation | yes | `BlockedReason` closed enum + outcome closed enum + reject unknown values at metric helper boundary (existing project pattern) |
| V6 Cryptography | no | No crypto operations |
| V8 Data Protection | yes (low) | DuckDB file path traversal — covered by Phase 57 P05 hardening (CR-01); Phase 63 adds no new file-path inputs |

### Known Threat Patterns for Go + DuckDB

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via `repo_id` | Tampering | Parameterized queries — already pattern-locked in `internal/semantic/store/overlay.go` (every `ExecContext` uses `?` placeholders); Phase 63 inherits |
| Free-text metric label cardinality explosion | Denial-of-service (memory) | Closed enum + drop-on-unknown helper (existing pattern in `internal/obs/`) |
| Long-running compaction tx blocks readers | Denial-of-service | Pre-flight size guard (CONTEXT.md D-01) caps tx size; per-workspace file isolation prevents cross-workspace blocking |
| Path traversal via workspace.WorkspaceKey | Tampering | Already validated upstream in `internal/workspace/` (Phase 57 hardening pass); Phase 63 receives validated keys, never raw paths |

## Sources

### Primary (HIGH confidence)

- **Helix codebase** (verified via Read on these files, 2026-05-07):
  - `internal/semantic/store/overlay.go` — D-04 epoch contract code lines 16-25, 187-194 [VERIFIED]
  - `internal/semantic/store/snapshot.go` — empty stub Phase 63 fills [VERIFIED]
  - `internal/semantic/store/migrations.go` — current schema (16 tables; `semantic_live_overlay_meta` exists, `semantic_meta` does NOT) [VERIFIED]
  - `internal/semantic/live/coalescer/coalescer.go` — per-workspace goroutine + `time.AfterFunc` pattern [VERIFIED]
  - `internal/semantic/graph/scheduler.go` — RankScheduler shape; no IsQuiescent yet [VERIFIED]
  - `internal/semantic/lspenrich/queue.go` — LaneQueue.Depth(lane) exists, LastEnqueueAt does NOT [VERIFIED]
  - `internal/daemon/daemon.go` — SetActivateCallback / SetEditNotifier wiring patterns [VERIFIED]
  - `internal/config/defaults.go` — `compact_after_idle_ms`, `lsp_compaction_max_wait_ms`, `snapshot_retention` already wired [VERIFIED]
  - `internal/obs/metrics.go` — bounded-label registration pattern + CounterVec/HistogramVec idiom [VERIFIED]
  - `cmd/vet-noduckdb/main.go` + `internal/lint/noduckdb/` — analyzer scope [VERIFIED]
  - `go.mod` — duckdb-go/v2 v2.10502.0 (DuckDB 1.5.x) [VERIFIED]

- **DuckDB official docs** (web fetch / web search 2026-05-07):
  - [CHECKPOINT statement](https://duckdb.org/docs/current/sql/statements/checkpoint) — semantics, blocking behavior
  - [Transaction Management](https://duckdb.org/docs/current/sql/statements/transactions.html) — snapshot isolation, atomicity
  - [Reclaiming Space (operations manual)](https://duckdb.org/docs/stable/operations_manual/footprint_of_duckdb/reclaiming_space) — VACUUM/CHECKPOINT no-op for space; COPY FROM DATABASE recommended [CITED]
  - [Analytics-Optimized Concurrent Transactions (Oct 2024)](https://duckdb.org/2024/10/30/analytics-optimized-concurrent-transactions) — optimistic concurrency, WAL replay [CITED]
  - [DuckDB Internals Part 5: Transaction Lifecycle (Alibaba Cloud)](https://www.alibabacloud.com/blog/duckdb-internals---part-5-the-transaction-lifecycle_602860) — internal commit machinery [CITED]

### Secondary (MEDIUM confidence)

- [duckdb/duckdb#21154](https://github.com/duckdb/duckdb/issues/21154) — VACUUM FULL not implemented; COPY FROM DATABASE workaround [CITED]
- [duckdb/duckdb#9721](https://github.com/duckdb/duckdb/issues/9721) — checkpoint_threshold parameter behavior [CITED]
- [duckdb/duckdb#17778](https://github.com/duckdb/duckdb/issues/17778) — duckdb file size growth diagnosis [CITED]
- [MotherDuck VACUUM doc](https://motherduck.com/docs/sql-reference/duckdb-sql-reference/duckdb-statements/vacuum/) — confirms no-op behavior [CITED]
- [duckdb/duckdb-web#1331](https://github.com/duckdb/duckdb-web/issues/1331) — open issue: VACUUM docs need updating [CITED]

### Tertiary (LOW confidence)

- None — every claim in this research is backed by either a codebase Read or an official DuckDB source.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `max_overlay_files * 4` is a safe pre-flight guard threshold | Pre-flight Overlay Row-Count Guard | Too low → `outcome=partial` fires often; too high → OOM under 1 GiB DuckDB memory_limit. Bench fixture should validate. |
| A2 | The `semantic_meta` table mentioned in CONTEXT.md D-05 was a misremembered name for `semantic_live_overlay_meta` (which IS the canonical per-workspace meta table) | Open Question #2 | If user actually wants a NEW generic meta table, P63-02 must add migration004 with a schema-and-data spec. Surface in discuss-phase. |
| A3 | Per-workspace DuckDB file isolation (Phase 57 STORE-01) holds — CHECKPOINT only locks one workspace's clients | Pitfall 2 / Landmine 2 | If a future phase introduces a shared file, CHECKPOINT becomes a global stop-the-world. Verified for Phase 63 against current Phase 57 invariant. |
| A4 | DuckDB's WAL replay correctly handles partial WAL writes on SIGKILL | Pitfall 5 / Crash Recovery Matrix | LazyFS testing (duckdb.org Oct 2024 article) backs this; if integration test reveals otherwise, fall back to `compaction_journal` sidecar (CONTEXT.md deferred). |
| A5 | `outcome=error` should retry on next coalescer flush (not permanently disable) | Open Question #3 | If user wants permanent-disable-until-restart semantics, planner must add a circuit-breaker. Safest default: retry-on-next-flush. |
| A6 | Adding `blocked_by` as a metric label would explode cardinality unacceptably | Metric Label Cardinality | If operators genuinely need per-reason-blocked breakdown, they will ask; current closed enum stays at 4 values. |
| A7 | The compaction span name `semantic.live.compact_overlay` (Phase 60 reservation) is the right home for span body, not a new `semantic.compact.run` namespace | Open Question #4 | Phase 60 reserved this name; Phase 63 fills the body. If naming preference shifts, planner aligns with Phase 60's choice. |
| A8 | Subprocess kill (not `runtime.Goexit()`) is the right fixture for kill-mid-compact COMPACT-05 test | Pitfall 6 / Crash Recovery Matrix | `runtime.Goexit()` in-process leaves DuckDB connection orphaned without exercising rollback. Subprocess is realistic. CONTEXT.md hints "(or process kill on a subprocess fixture)" — this research strongly recommends subprocess. |

**If this table is empty:** N/A — contains 8 assumptions that the planner should confirm or treat as defaults. A1, A2, A3, A4 are most load-bearing.

## Metadata

**Confidence breakdown:**

- **Standard stack:** HIGH — every library is already in `go.mod`; versions verified.
- **Architecture patterns:** HIGH — every pattern (per-workspace goroutine, AfterFunc debounce, constructor injection, setter-based daemon wiring, closed-enum metrics, single-tx atomicity) is already in use elsewhere in Helix and verified via codebase Read.
- **DuckDB semantics:** HIGH — verified against official duckdb.org documentation and 5 GitHub issues; all critical claims (VACUUM no-op, CHECKPOINT stop-the-world, WAL replay correctness, optimistic concurrency) cross-verified across at least 2 sources.
- **Existing accessor inventory:** HIGH — direct grep + Read on every named source file. Six accessors confirmed missing; one (`LaneQueue.Depth`) confirmed present.
- **Pitfalls:** HIGH — most are codebase precedents (Phase 57/59/60/62 already hit them) or DuckDB-documented limitations.
- **Validation architecture:** MEDIUM — test framework + commands are direct from project standards. Per-test naming and structure are recommendations the planner may adjust.

**Research date:** 2026-05-07
**Valid until:** 2026-06-07 (30 days; DuckDB 1.5.x is stable; the fast-moving piece is the duckdb VACUUM-FULL issue resolution — re-check duckdb/duckdb#21154 status)
