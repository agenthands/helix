# Phase 63: Compaction & Retention - Context

**Gathered:** 2026-05-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 63 ships **idle-debounced compaction** that folds the Phase 60 live
overlay into committed snapshots without losing rows under concurrent writes
or daemon crashes, plus snapshot retention and bounded on-disk growth so
long-lived repos stay performant.

**Phase 63 ships:**

1. **Snapshot-write API** in `internal/semantic/store/snapshot.go` (currently
   an empty doc-comment stub deferred from Phase 59):
   - `BeginSnapshot(ctx, SnapshotMeta) (*Snapshot, error)`
   - `WriteSnapshotFacts(ctx, *Snapshot, merged Facts) error` — typed insert
     helpers for `InsertFiles`/`InsertSymbols`/`InsertReferences`/`InsertEdges`
   - `CommitSnapshot(ctx, *Snapshot, summary SnapshotSummary) error`
   - `AbortSnapshot(ctx, *Snapshot, reason string) error`

2. **Compaction worker** at `internal/semantic/compact/`:
   - Per-workspace dedicated goroutine spawned on `kernel.ActivateWorkspace`,
     joined on shutdown.
   - Owns `time.AfterFunc(compact_after_idle_ms)` reset on every coalescer
     flush via a `LastFlushAt` accessor.
   - Single-tx compaction: snapshot-write + ClearOverlay (where
     `write_epoch <= captured_epoch`) + retention deletes commit atomically
     via DuckDB ACID.
   - Pre-flight overlay-row-count guard; if the projected working set would
     exceed a planner-decided threshold, emit `outcome=partial` and skip.

3. **`CompactionGate.IsReady(ws) (bool, blockedBy)`** in
   `internal/semantic/compact/gate.go` — the SPEC §22.1 quiescence aggregator.
   Composes small read-only accessors registered by each component:
   - coalescer `LastFlushAt(ws)` for `compact_after_idle_ms` gate
   - overlay store `OpenTxCount(ws)` for "no active overlay tx"
   - LSPQueue `Depth(ws)` + `LastEnqueueAt(ws)` for "queue empty OR
     `lsp_compaction_max_wait_ms` elapsed"
   - RankScheduler `IsQuiescent(ws)` for "graph cache version stable"
   - kernel `ActiveEditTxCount(ws)` for "no active edit tx"

4. **`overlay_epoch` CAS read** consuming the Phase 60 D-04 contract:

   ```text
   1. Compaction reads meta.current_epoch → captured_epoch
   2. Compaction reads overlay rows with write_epoch ≤ captured_epoch
   3. Compaction merges those rows into the new snapshot
   4. ClearOverlay deletes rows WHERE write_epoch ≤ captured_epoch
   5. Rows with write_epoch > captured_epoch (committed during compaction)
      remain in the overlay for the next compaction cycle
   ```

5. **Snapshot retention** keeping the last `snapshot_retention=5` snapshots;
   older snapshots dropped in the same compaction tx so retention is
   atomic with snapshot creation (COMPACT-04).

6. **`CHECKPOINT`** issued at every compaction commit to flush the DuckDB
   write-ahead log to disk (COMPACT-03).

7. **Config-gated weekly VACUUM** piggybacked on the per-workspace compactor.
   After a successful compaction commit, the same goroutine checks
   `now() - last_vacuum_at >= vacuum_interval` (default `168h`); if yes AND
   `CompactionGate.IsReady` still holds, VACUUM fires in a separate tx.
   Default off; opt-in via config.

8. **`semantic.live.compact_overlay`** trace span body filled (Phase 60 left
   as no-op stub).

9. **`helix_semantic_compaction_duration_seconds{outcome}`** metric registered
   through `internal/obs/` with closed-enum outcomes:
   `success | partial | skipped_blocked | error`.

10. **New config keys** under `semantic_index.maintenance.*`:
    - `vacuum_enabled` (default `false`)
    - `vacuum_interval` (default `"168h"`)

    All other knobs (`compact_after_idle_ms`, `lsp_compaction_max_wait_ms`,
    `snapshot_retention`) are already wired in `internal/config/defaults.go`
    (Phase 57/60).

11. **Long-repo bench fixture** asserting `.duckdb` file growth is bounded
    across 1000 simulated commits (COMPACT-03).

**Out of scope (deferred):**

- **MCP tool wrappers** (`refresh_semantic_graph`, `index_semantic_graph`,
  `get_semantic_graph_status`) — Phase 64. Phase 63 ships the underlying
  compaction trigger and snapshot-write API; Phase 64 wraps them as MCP.
- **`get_health` integration surfacing compaction status** — Phase 65
  strangler-fig. Phase 63 ships only a `Status()` accessor on the per-
  workspace compactor.
- **Strangler-fig integration of `get_repo_map` / `get_context` over
  committed snapshots** — Phase 65.
- **Real effective-graph queries** (`QueryEffectiveAdjacency`,
  `CountStaleScoreRows`, `MarkAllScoreRowsStale`) — Phase 64. Phase 63
  does not consume these.

</domain>

<decisions>
## Implementation Decisions

### Crash-recovery mechanism

- **D-01: Single-tx compaction with pre-flight overlay-size guard.**
  One DuckDB transaction wraps:
  1. `BeginSnapshot` (allocate snapshot_id, base = latest committed snapshot).
  2. `WriteSnapshotFacts` (typed inserts for files/symbols/references/edges
     produced by `MergeBaseAndOverlay`).
  3. `CommitSnapshot` (flip status to committed, write summary row).
  4. `ClearOverlay` (`DELETE FROM semantic_live_overlay_* WHERE repo_id=? AND
     write_epoch <= ?`).
  5. Snapshot retention deletes (drop snapshots beyond the most-recent 5).

  Crash mid-tx → DuckDB rollback restores the prior committed snapshot
  AND the entire overlay AND retention rows. The kill-mid-compact
  integration test (COMPACT-05 acceptance) asserts this.

  **Pre-flight size guard:** before opening the tx, count overlay rows:

  ```go
  if rows, _ := s.OverlayRowCount(ctx, repoID, capturedEpoch); rows > guard {
      // emit metric outcome=partial; skip this cycle
      // CompactionGate next idle window will retry
      return ErrOverlayTooLarge
  }
  ```

  Threshold value: planner's call. Suggestion: `max_overlay_files * 4`
  (the symbols/refs/edges expansion factor) so a workspace honoring
  `max_overlay_files=1000` produces at most ~4k rows per fact table —
  comfortably under DuckDB's `memory_limit: 1GiB` default.

  **Hard invariants:**
  - Compaction NEVER deletes overlay rows with `write_epoch > captured_epoch`.
    Rows committed during compaction (the per-tx `current_epoch` increments
    independently of compaction) survive for the next pass — Phase 60 D-04
    contract.
  - Snapshot creation and ClearOverlay MUST be in the same DuckDB tx.
    Splitting them re-opens the COMPACT-02 race.
  - Retention deletes (drop snapshots > 5) MUST be in the same tx. If
    retention fails, compaction fails; rollback restores the older snapshot
    that was about to be evicted.
  - On `outcome=partial` (size guard hit): no rows deleted from overlay,
    no snapshot committed. The next idle window retries.

### Snapshot-write API scope

- **D-02: Two plans inside Phase 63 — API first, compactor second.**

  **P63-01: Snapshot-write API.**
  Fills `internal/semantic/store/snapshot.go` with:
  ```go
  func (s *Store) BeginSnapshot(ctx, SnapshotMeta) (*Snapshot, error)
  func (s *Store) WriteSnapshotFacts(ctx, *Snapshot, Facts) error
  func (s *Store) CommitSnapshot(ctx, *Snapshot, SnapshotSummary) error
  func (s *Store) AbortSnapshot(ctx, *Snapshot, reason string) error
  ```
  Plus a synthetic "fake compactor" test fixture that exercises the API
  end-to-end without depending on the real overlay merge logic. Unit tests
  cover BeginSnapshot → WriteSnapshotFacts → CommitSnapshot,
  BeginSnapshot → AbortSnapshot, and the retention-delete pattern.

  **P63-02: Compactor + retention + VACUUM.**
  Adds `internal/semantic/compact/` (compactor goroutine, `gate.go` with
  `CompactionGate.IsReady`, integration with the snapshot-write API from
  P63-01). Includes the kill-mid-compact integration test, the
  `interleave overlay writes with compaction` property test, and the
  long-repo bench fixture.

  **Rationale:** the snapshot-write API is reviewable as a clean foundation
  PR before compaction logic enters the picture. Risk reduction: the
  compactor PR is purely about compaction state-machine logic, not about
  snapshot insert correctness.

### Compaction trigger ownership + RankScheduler coordination

- **D-03: Per-workspace dedicated compactor goroutine.**

  ```go
  // internal/semantic/compact/compactor.go (new)
  type Compactor struct {
      workspaceID workspace.WorkspaceKey
      gate        *CompactionGate
      store       *store.Store
      lspq        LSPQueueAccessor
      coalescer   CoalescerAccessor
      scheduler   SchedulerAccessor
      kernel      KernelAccessor
      timer       *time.Timer  // time.AfterFunc(compact_after_idle_ms)
      cfg         CompactorConfig
      logger      *slog.Logger
  }
  ```

  Spawned on `kernel.ActivateWorkspace`, joined on workspace deactivation
  and on daemon shutdown. Mirrors Phase 60 ownership pattern exactly:
  coalescer, watcher, manifest scanner are all per-workspace.

  Trigger flow:
  1. Coalescer flush completes → calls `compactor.OnFlush()`.
  2. `OnFlush` resets `time.AfterFunc(cfg.CompactAfterIdleMS)`.
  3. Timer fires → goroutine calls `gate.IsReady(ws)`.
  4. If ready → run compaction (D-01); else → log `blockedBy` reason as
     debug, no retry (next coalescer flush will reset the timer).

  **Hard invariants:**
  - One compactor goroutine per workspace. No global compactor.
  - Compactor MUST NOT hold the per-workspace overlay tx mutex while waiting
    for `CompactionGate.IsReady` — the gate is read-only.
  - Compactor MUST be torn down when its workspace deactivates; otherwise
    a stale workspace could fire a tx against a closed handle.

- **D-04: Single aggregate `CompactionGate.IsReady` accessor.**

  ```go
  // internal/semantic/compact/gate.go (new)
  type CompactionGate struct {
      coalescerAccess CoalescerAccessor   // LastFlushAt(ws) time.Time
      overlayAccess   OverlayTxAccessor   // OpenTxCount(ws) int
      lspAccess       LSPQueueAccessor    // Depth(ws) int, LastEnqueueAt(ws) time.Time
      schedAccess     SchedulerAccessor   // IsQuiescent(ws) bool
      kernelAccess    KernelEditAccessor  // ActiveEditTxCount(ws) int
      cfg             GateConfig
      now             func() time.Time
  }

  type BlockedReason string
  const (
      BlockedNone           BlockedReason = ""
      BlockedOverlayEmpty   BlockedReason = "overlay_empty"
      BlockedIdleTooShort   BlockedReason = "idle_too_short"
      BlockedEditTxActive   BlockedReason = "edit_tx_active"
      BlockedOverlayTxActive BlockedReason = "overlay_tx_active"
      BlockedLSPPending     BlockedReason = "lsp_pending"
      BlockedRankRepairing  BlockedReason = "rank_repairing"
  )

  func (g *CompactionGate) IsReady(ws workspace.WorkspaceKey) (bool, BlockedReason)
  ```

  Each component ships a small read-only accessor (constructor injection,
  Phase 59 D-02). The gate composes them with deterministic ordering for
  testability (returns the FIRST blocking reason, not a bag).

  **Hard invariants:**
  - `IsReady` MUST be idempotent and side-effect-free. Repeated calls under
    the same state return the same answer.
  - `IsReady` MUST NOT block on I/O. Each accessor returns from in-memory
    state (counters, atomics, last-event timestamps).
  - `BlockedReason` is a closed enum (no free-text) so `outcome=skipped_blocked`
    metric label cardinality stays bounded.

  **New accessors required on existing components** (small additive
  changes; Phase 63 adds them):
  - `coalescer.LastFlushAt(ws) time.Time` — already implicit (the timer is
    a `time.AfterFunc`); expose the timestamp.
  - `store.OverlayTxOpenCount(ws) int` — atomic counter incremented in
    `BeginOverlayTx`, decremented on Commit/Rollback.
  - `lspqueue.Depth(ws) int` + `lspqueue.LastEnqueueAt(ws) time.Time` —
    Phase 61 has the queue; surface both.
  - `scheduler.IsQuiescent(ws) bool` — Phase 62-03 has RankScheduler with
    drop-on-full; expose "no in-flight repair AND no queued repair".
  - `kernel.ActiveEditTxCount(ws) int` — kernel edit tools are short-lived;
    a counter under the kernel handle's mutex suffices.

### VACUUM cadence

- **D-05: VACUUM piggybacked on the per-workspace compactor; default off.**

  After a successful compaction commit, the same goroutine checks:

  ```go
  if cfg.VacuumEnabled &&
      now.Sub(lastVacuumAt) >= cfg.VacuumInterval &&
      gate.IsReady(ws).ready {
      runVacuum(ctx, ws)  // separate tx; CHECKPOINT after
      writeLastVacuumAt(ctx, ws, now)
  }
  ```

  `last_vacuum_at` storage: a row in `semantic_meta` keyed by
  `(repo_id, key='last_vacuum_at')` with value as RFC3339 string. If
  `semantic_meta` doesn't already accept arbitrary `(repo_id, key)` rows,
  add a small schema migration in Phase 57's registry pattern. Default
  config:

  ```yaml
  semantic_index:
    maintenance:
      vacuum_enabled: false
      vacuum_interval: "168h"   # 1 week
  ```

  **Hard invariants:**
  - VACUUM MUST run in its own tx (DuckDB blocks all reads/writes during
    VACUUM; wrapping it inside the compaction tx would extend the lock
    window from "compaction duration" to "compaction + VACUUM").
  - VACUUM MUST re-check `gate.IsReady` immediately before firing — the
    quiescence window may have closed between compaction commit and the
    VACUUM check.
  - VACUUM MUST emit a separate trace span (`semantic.maintenance.vacuum`)
    and metric label so operators can distinguish compaction time from
    VACUUM time.
  - Default-off keeps fresh installs from hitting a multi-second VACUUM
    surprise in week 2; production deployments and the long-repo bench
    fixture explicitly opt in.

### Acceptance criteria (must hold at end of phase)

1. **Snapshot API (P63-01):** all four methods land in
   `internal/semantic/store/snapshot.go`; unit tests exercise begin → write
   → commit, begin → abort, and retention-delete patterns against the
   synthetic fake compactor fixture.
2. **Single-tx compaction (P63-02):** kill-mid-compact integration test
   confirms overlay+snapshot consistent on restart (COMPACT-05). Test fires
   `runtime.Goexit()` (or process kill on a subprocess fixture) at every
   tx phase and asserts the resulting state.
3. **CAS contract (P63-02):** property test interleaves overlay writes with
   compaction; rows committed during compaction (`write_epoch >
   captured_epoch`) MUST survive ClearOverlay (COMPACT-02).
4. **Pre-flight size guard (P63-02):** integration test forces an overlay
   above the threshold; assert `outcome=partial`, no rows deleted, no
   snapshot committed; next idle window after overlay shrinks below
   threshold runs successfully.
5. **`CompactionGate.IsReady` (P63-02):** unit tests cover each
   `BlockedReason` in isolation; the `BlockedNone` path is gated only when
   ALL six conditions hold.
6. **Snapshot retention (P63-02):** integration test runs 7 compactions;
   asserts only the most-recent 5 snapshots remain; older snapshots deleted
   in the same compaction tx (COMPACT-04).
7. **CHECKPOINT (P63-02):** integration test asserts the WAL is flushed
   after each compaction commit (file-size or `pragma wal_status` check;
   DuckDB equivalent).
8. **Long-repo bench fixture (P63-02):** synthetic 100-file fixture runs
   1000 random-edit cycles with compaction enabled; asserts `.duckdb` file
   size stays under a planner-decided bound (COMPACT-03).
9. **VACUUM piggyback (P63-02):** integration test sets
   `vacuum_enabled=true`, `vacuum_interval="1ms"`, simulates one
   compaction cycle, asserts VACUUM fires in a separate tx; with
   `vacuum_enabled=false`, VACUUM never fires regardless of elapsed time.
10. **Metric/span (P63-02):**
    `helix_semantic_compaction_duration_seconds{outcome}` emits with
    closed-enum labels; `semantic.live.compact_overlay` span body filled
    (Phase 60 left as no-op stub).
11. **COMPACT-01..05 marked Done** in `.planning/REQUIREMENTS.md` after
    Phase 63 close.

### Claude's Discretion (no user input needed)

- **Concrete package layout:** `internal/semantic/compact/` for compactor
  + gate; `internal/semantic/store/snapshot.go` filled. Sub-packages
  (`internal/semantic/compact/gate/`) are at the planner's call.
- **Pre-flight overlay row-count threshold:** suggestion
  `max_overlay_files * 4`. Planner may revise after benching.
- **`BlockedReason` enum exact names:** planner's call as long as the
  closed-enum cardinality is preserved (no free-text).
- **`semantic_meta` schema for `last_vacuum_at`:** prefer a generic
  `(repo_id, key, value)` shape so future maintenance keys (e.g.
  `last_full_reindex_at`) reuse the same table without new schema
  migrations.
- **Bench fixture shape:** synthetic 100-file Go workspace, 1000 cycles
  of "edit a random file → coalescer flush → compaction". Specific size
  bound deferred to bench result; suggestion: `growth_factor < 2.0x` of
  initial committed snapshot size after 1000 cycles.
- **Pre-existing component accessors:** if `coalescer.LastFlushAt`,
  `lspqueue.Depth`, `lspqueue.LastEnqueueAt`, `scheduler.IsQuiescent`,
  `kernel.ActiveEditTxCount`, `store.OverlayTxOpenCount` don't exist
  yet, P63-02 adds them — they're small read-only additions.
- **Trace span names:** `semantic.live.compact_overlay` (Phase 60
  reservation), `semantic.maintenance.vacuum` (new).
- **Bounded-label outcomes:** `success | partial | skipped_blocked |
  error`. The fifth-bucket case (compaction tried but DuckDB returned
  an error) maps to `error`; metric description must call out that
  `error` is non-recoverable for that idle window — next coalescer
  flush resets the timer.
- **Pipeline integration:** Phase 63 likely fills the `Run` body of a
  pipeline phase if Phase 57 reserved one for compaction. If not, the
  compactor goroutine bootstraps directly from `internal/daemon/daemon.go`
  alongside the live-update spine (Phase 60 D-06 pattern).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements (load-bearing)

- `.planning/REQUIREMENTS.md` §COMPACT (lines 83-89) — COMPACT-01 through
  COMPACT-05 phrasing is the contract. Each requirement maps to an
  acceptance test (D-acceptance #1–#11 above).
- `.planning/milestones/v1.10-ROADMAP.md` Phase 63 block (lines 143-152)
  — Goal, Depends on (Phase 60, Phase 62), Requirements, 4 Success
  Criteria.

### Specification (source of truth for shapes and rules)

- `SPEC-DRAFT.md` §22 (Overlay Compaction) — §22.1 trigger conditions,
  §22.2 pseudocode (the shape Phase 63 implements). The pseudocode's
  `Store.GetLatestSnapshot`, `Store.LoadOverlay`, `Store.BeginSnapshot`,
  `Store.WriteSnapshotFacts`, `Store.CommitSnapshot`,
  `Store.AbortSnapshot`, `Store.ClearOverlay` all map to D-01/D-02
  decisions above.
- `SPEC-DRAFT.md` §9.11 — Live Overlay Tables. Phase 63's CAS read uses
  the `write_epoch` column added by Phase 60.
- `SPEC-DRAFT.md` §25 — `semantic_index.live_updates.compact_after_idle_ms`,
  `lsp_compaction_max_wait_ms`, `indexing.snapshot_retention` (already
  wired in `internal/config/defaults.go`). Phase 63 adds
  `maintenance.vacuum_enabled` and `maintenance.vacuum_interval`.
- `SPEC-DRAFT.md` §28.1 — `helix_semantic_compaction_duration_seconds{outcome}`
  metric.
- `SPEC-DRAFT.md` §28.2 — `semantic.live.compact_overlay` trace span
  (Phase 60 stub; Phase 63 fills body). Phase 63 adds
  `semantic.maintenance.vacuum`.
- `SPEC-DRAFT.md` §32 Phase 8 — "Compaction and Retention" deliverables
  and acceptance criteria.

### Phase 60 lock-down (CAS contract — must not regress)

- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md`
  — D-04 (overlay_epoch CAS contract — Phase 63 reads under CAS), D-02
  (per-workspace coalescer pattern — Phase 63 mirrors for compactor),
  D-05 (per-workspace ownership — compactor follows the same lifecycle
  as watcher manager and scanner).
- `internal/semantic/store/overlay.go` — D-04 epoch contract code (lines
  16–25 document Phase 63's CAS read; lines 187–194 reserve `FlushOverlay`
  for cooperative drain semantics if Phase 63 needs them).
- `internal/semantic/store/snapshot.go` — currently empty stub Phase 63
  fills (P63-01).
- `internal/semantic/store/migrations.go` — current schema. Phase 63's
  optional `last_vacuum_at` migration plugs into the existing
  `Migration{From, To, Kind}` registry.
- `internal/semantic/store/duckdb.go` — fact-store open path; Phase 63
  does not change open semantics.

### Phase 62 lock-down (RankScheduler quiescence)

- `.planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md`
  — RankScheduler debounce + drop-on-full design. Phase 63 adds an
  `IsQuiescent(ws) bool` accessor consumed by `CompactionGate.IsReady`.
- `internal/semantic/rank/scheduler.go` — RankScheduler implementation;
  Phase 63 surfaces "no in-flight repair AND no queued repair" as the
  quiescence answer.

### Phase 61 lock-down (LSPQueue accessor)

- `internal/semantic/lspqueue/` (or wherever Phase 61 lands the queue) —
  Phase 63 adds `Depth(ws) int` and `LastEnqueueAt(ws) time.Time`
  accessors for the gate.

### Architectural invariants

- `CLAUDE.md` "Middleware Execution Order (LIFO)" — Phase 63 does NOT
  touch middleware; compaction lives below the MCP layer.
- `internal/daemon/daemon.go` — Phase 63 wiring lands after Phase 60's
  watcher/scanner construction; new bootstrap calls:
  `compactor := compact.NewCompactor(...)`, attached to workspace
  activation alongside watcher/scanner.

### Pattern templates (must mirror)

- `internal/semantic/live/coalescer.go` (Phase 60) — per-workspace
  goroutine + `time.AfterFunc` timer pattern. Phase 63's compactor
  mirrors the structure.
- `internal/daemon/daemon.go` `SetEnrichFn` / `SetActivateCallback` /
  `SetEditNotifier` — Phase 63's compactor wiring follows the same
  setter/per-workspace activation pattern.
- `internal/obs/` — bounded-label metric registration; Phase 63
  registers `helix_semantic_compaction_duration_seconds` through this
  path.
- `internal/config/defaults.go` (Phase 60 P05B) — Phase 63 adds
  `maintenance.vacuum_enabled` (false) and `maintenance.vacuum_interval`
  ("168h") via the same defaults table + per-feature defaults test.

### Validation tooling

- `cmd/vet-noduckdb/` — Phase 63's `internal/semantic/compact/` package
  MUST NOT import `duckdb-go` directly; it goes through
  `internal/semantic/store/`. Vet analyzer enforces.
- (potential new) `cmd/vet-compact-uses-store/` — if the planner judges
  it warranted, a small additional analyzer can pin the
  `compact → store` boundary the way `vet-nokernel2semantic` pins
  `kernel → semantic`. Not required but consistent with the project's
  precedent.

### Test fixtures (must seed)

- New: `internal/semantic/compact/testdata/` for the kill-mid-compact
  fixture (subprocess-launched daemon that runs one compaction cycle and
  is killed at each tx phase boundary).
- New: `internal/semantic/compact/longrepo_bench_test.go` — synthetic
  100-file workspace, 1000 random-edit cycles, asserts bounded `.duckdb`
  growth.
- New: `internal/semantic/compact/gate_test.go` — unit tests for each
  `BlockedReason` in isolation, plus the `BlockedNone` happy path.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`internal/semantic/store/`** (Phase 57 + Phase 60) — fact store with
  overlay write API and `current_epoch`/`write_epoch` schema. Phase 63
  fills the empty `snapshot.go` and adds the compaction read/clear paths
  via new methods on `*Store`.
- **Phase 60 `internal/semantic/live/coalescer.go`** — per-workspace
  goroutine + `time.AfterFunc(debounce_ms)` pattern. Phase 63's compactor
  mirrors this structure with `time.AfterFunc(compact_after_idle_ms)`.
- **`internal/obs/`** — bounded-label metric registration; Phase 63
  registers `helix_semantic_compaction_duration_seconds` and
  `helix_semantic_vacuum_duration_seconds` through this path.
- **`internal/config/`** — koanf 4-layer precedence; new
  `maintenance.*` keys added to `defaults.go` and the
  `SerenaConfig.SemanticIndex.Maintenance` struct field.
- **DuckDB ACID** — single-tx compaction relies on this; rollback
  semantics on crash mid-tx are the entire correctness story for
  COMPACT-05.
- **`internal/semantic/store/migrations_registry.go`** — schema
  migration registry; Phase 63 may plug in an optional migration if
  `semantic_meta` doesn't already accept arbitrary `(repo_id, key)`
  rows for `last_vacuum_at`.
- **Phase 62 `RankScheduler`** — Phase 63 adds an `IsQuiescent`
  accessor; the scheduler's existing internal state (in-flight count,
  queue depth) is surfaced read-only.
- **Phase 61 `LSPQueue`** — Phase 63 adds `Depth` and `LastEnqueueAt`
  accessors; the queue already tracks both internally.

### Established Patterns

- **Constructor injection** (Phase 59 D-02). Phase 63's compactor and
  gate are constructed with their dependency accessors — no `init()`
  registration, no blank imports.
- **Setter-style cross-package wiring** (`SetEnrichFn`,
  `SetActivateCallback`, `SetEditNotifier`). Phase 63's compactor
  registration follows this pattern from `internal/daemon/daemon.go`.
- **Bounded-label metrics with closed enum** (Phase 57 D-07). `outcome`
  on `helix_semantic_compaction_duration_seconds` is a closed enum;
  `BlockedReason` (used as a metric label on `outcome=skipped_blocked`)
  is also a closed enum.
- **Per-feature defaults test** (Phase 57 D-09, Phase 59/60 inheritance).
  Phase 63 adds `TestLoad_MaintenanceDefaults` for the two new keys.
- **Schema migration via registry** (Phase 57 D-02). If `last_vacuum_at`
  needs a new column or table, Phase 63's migration is one entry in the
  registry with up/down SQL.
- **Per-workspace ownership** (Phase 60 D-02 / D-05). Phase 63's
  compactor is per-workspace, spawned on `kernel.ActivateWorkspace`,
  joined on deactivation/shutdown.
- **Single-tx atomicity** (Phase 60 D-04 epoch increment, Phase 62
  graph_version single-bump). Phase 63 extends the pattern: compaction
  is one tx, retention is in the same tx.

### Integration Points

- **Daemon bootstrap (`internal/daemon/daemon.go`)** — Phase 63 adds:
  - Construct `compact.NewCompactor(...)` per active workspace.
  - Register `helix_semantic_compaction_duration_seconds` and
    `helix_semantic_vacuum_duration_seconds` via `internal/obs/`.
  - On `kernel.ActivateWorkspace`: spawn the compactor goroutine; on
    deactivation: join.
- **`SerenaConfig.SemanticIndex.Maintenance`** field — new koanf struct
  for the two `maintenance.*` keys. Defaults wired through
  `internal/config/defaults.go`.
- **Phase 60 coalescer post-flush hook** — Phase 60's coalescer flush
  goroutine gets a `compactor.OnFlush()` call inserted at the success
  path. The compactor handle is held by the same per-workspace
  registry the coalescer uses.
- **Phase 62 RankScheduler** — Phase 63 adds an `IsQuiescent(ws)`
  accessor as a small additive change to the scheduler interface.
- **Phase 61 LSPQueue** — Phase 63 adds `Depth(ws)` and
  `LastEnqueueAt(ws)` accessors.

### Constraints

- Phase 63 must NOT touch middleware order (CLAUDE.md "Middleware
  Execution Order (LIFO)"). Compaction is below the MCP layer.
- Phase 63 must NOT import `duckdb-go` outside `internal/semantic/store/`
  (Phase 57 D-12 + `cmd/vet-noduckdb/` analyzer). The compactor goes
  through `*Store`.
- Phase 63 must NOT delete overlay rows with `write_epoch >
  captured_epoch` (Phase 60 D-04 CAS contract). Single most load-bearing
  invariant for Phase 63.
- Phase 63 must NOT split snapshot creation and `ClearOverlay` across
  separate transactions (re-opens COMPACT-02 race).
- Phase 63 must NOT block on I/O inside `CompactionGate.IsReady` (must
  be in-memory state only, side-effect-free).
- Phase 63 must NOT run VACUUM inside the compaction tx (extends the
  global lock window from "compaction" to "compaction + VACUUM").
- Phase 63 must NOT default `vacuum_enabled` to true (REQUIREMENTS
  COMPACT-03 phrasing is "config-gated"; default-off respects that).

</code_context>

<specifics>
## Specific Ideas

- **The user's "filesystem state is truth" framing carries forward.**
  Phase 63's compactor is downstream of Phase 60's classifier; it never
  reads the filesystem directly. Overlay rows are the only input.
- **Single-tx is the simplest correct invariant.** REQUIREMENTS
  COMPACT-05 explicitly leaves "single tx OR journal" open; the size
  guard preserves the simplicity at the cost of a `partial` outcome that
  retries naturally on the next idle window.
- **Two-plan split (D-02) optimizes for review surface, not phase
  count.** P63-01 ships a clean snapshot-write API; P63-02 ships
  compaction logic on top. Each plan has a self-contained verification
  story.
- **`CompactionGate.IsReady` is the single point where Phase 60, Phase
  61, Phase 62, and the kernel's edit lifecycle all meet.** Get the
  ordering right (read-only accessors, deterministic blocked-reason
  precedence) and the gate becomes the natural extension point for
  future preconditions (e.g. Phase 65 strangler-fig may want
  "snapshot in use by foreground tool" as a new blocker).
- **VACUUM default-off respects the user-experience promise.** A
  user who enables `semantic_index.enabled=true` shouldn't get a
  surprise multi-second pause in week 2. Production deployments and
  the long-repo bench fixture explicitly opt in.
- **The kill-mid-compact integration test is a hard gate.**
  COMPACT-05 explicitly requires it. Skipping fails the phase
  regardless of unit-test coverage.

</specifics>

<deferred>
## Deferred Ideas

- **`compaction_journal` sidecar table (alternative crash-recovery
  mechanism).** Considered and rejected for D-01 in favor of single-tx
  with size guard. If benchmarks reveal the size guard fires routinely
  on real workloads — i.e. the `partial` outcome becomes the common
  case — revisit and add the journal in a follow-up phase.
- **Adaptive `compact_after_idle_ms`.** Fixed 5000ms by default. A
  future phase could shorten it on high-churn workspaces and lengthen
  it on idle ones. Not now.
- **Per-snapshot retention policies.** Phase 63 keeps the last N=5
  snapshots uniformly. A future phase could keep "the last 5 daily
  snapshots + 4 weekly + 12 monthly" if long-term history becomes
  relevant. Not now.
- **`get_semantic_graph_status` MCP tool wrapper.** Phase 64. Phase 63
  ships only the `Status()` accessor on the per-workspace compactor.
- **`refresh_semantic_graph` MCP tool.** Phase 64. Phase 63 ships only
  the underlying `ScheduleIncremental` path (Phase 60 wired) plus the
  compaction trigger.
- **`get_health` integration surfacing compaction status.** Phase 65
  strangler-fig. Phase 63 ships only the data accessor; Phase 65 wires
  it.
- **MCP push notifications for compaction state changes.** No current
  consumer; deferred to Phase 64+ if needed.
- **Multi-workspace VACUUM serialization.** Phase 63 treats workspaces
  as independent for VACUUM (each per-workspace compactor checks
  independently). If a future phase finds VACUUM contention across
  workspaces under high load, a daemon-level VACUUM scheduler can
  coordinate. Not now.
- **Compaction concurrency across workspaces.** Phase 63 allows
  per-workspace compactors to run concurrently (no global compaction
  lock). If DuckDB contention surfaces, a global semaphore can be
  added. Not now.
- **Snapshot diff / time-travel queries.** SPEC §22.2 keeps
  `BaseSnapshotID` on every committed snapshot — sets up future
  diff/time-travel queries but Phase 63 doesn't expose them.
- **Online compaction (without idle window).** Phase 63 requires
  quiescence. If the daemon is genuinely never idle (continuous edit
  storms), compaction never fires. The bulk_update collapse and
  manifest_scan_interval together bound this in practice; if a
  pathological case emerges, a future phase could add a "force
  compaction every N edits regardless" knob.
- **Cross-workspace shared snapshots.** A monorepo with submodules
  spanning multiple workspaces could in principle share snapshots.
  Out of scope.

### Reviewed Todos (not folded)

None — no todos matched Phase 63 scope per the cross-reference step.

</deferred>

---

*Phase: 63-compaction-retention*
*Context gathered: 2026-05-07*
