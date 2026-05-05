# Phase 60: Live Update Pipeline - Context

**Gathered:** 2026-05-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 60 ships the **live update pipeline** that keeps the Phase 57 semantic
overlay synchronized with workspace state. Source changes — from editor saves,
file moves, Helix's own edit tools, and external LLM/IDE/shell writes — flow
into the live overlay deterministically, survive editor atomic-rename save
patterns, and never silently miss events.

**Phase 60 ships:**

1. **Three change-source signals**, all normalized to a single `WorkspaceChangeSignal{WorkspaceID, Paths, Source, ObservedAt}` shape:
   - **`fsnotify` watcher** — directory-level, per-workspace, atomic-rename re-attach for Vim swap-rename, JetBrains `___jb_tmp___`+rename, VS Code atomic save.
   - **`helix_edit` hook** — fast-path notification from kernel edit tools via the new `EditNotifier` interface (D-03).
   - **`manifest_scan`** — periodic content-hash scrub (default 10s) walking the workspace and comparing against `semantic_files.content_hash` (always on; doubles as ENOSPC fallback per D-05).

2. **Per-workspace coalescer + dispatcher** — single goroutine, debounced flush, runs SPEC §16.2 `CoalesceEvents` as a pure function over `[]SourceChangeEvent`, then dispatches sequentially. Bulk-update collapse fires at flush-time only when `len(coalesced) > live_updates.bulk_change_threshold` (default 200).

3. **Path-only → kind classification owned by semantic.** `ClassifyPathChange(ctx, workspaceID, path)` reads filesystem state + compares against `semantic_files.content_hash` and returns one of `ChangeFileCreated | ChangeFileModified | ChangeFileDeleted | ChangeFileRenamed | ChangeHelixEdit | ChangeBulkUpdate`. The classifier is the single owner of change-kind decisions; sources just supply paths.

4. **Overlay writer** filling the empty Phase 57 stub `internal/semantic/store/overlay.go`:
   - `BeginOverlayTx(ctx, repoID) (OverlayTx, error)`
   - `tx.UpsertOverlayFile/Symbol/Reference/Edge(...)`
   - `tx.MarkFileDeleted/MarkSymbolsDeleted/MarkReferencesDeleted/MarkEdgesDeleted(...)`
   - `tx.WriteInvalidations(...)` (no-op stub for Phase 60; Phase 62 fills consumer)
   - `tx.Commit() / tx.Rollback()`
   - `FlushOverlay(ctx)` for periodic/shutdown drain.

5. **Monotonic `overlay_epoch`** per workspace — atomically allocated per write tx (D-04). Persisted via schema migration v2→v3.

6. **Live update handlers** per SPEC §16.3–§16.5: `UpdateChangedFile`, `HandleFileDeleted`, `HandleFileRenamed`. Re-extraction reuses Phase 59's `extract.Registry` (Go / TS+JS / Python first-class; non-first-class languages get `extraction_status="unsupported"` rows).

7. **Phase 59 `ScheduleIncremental(workspaceID, []FileChange) JobID`** body filled — the contract Phase 59 deferred. This is the in-process path the dispatcher invokes.

8. **`pipelines/live.go`** Run-body filled (Phase 57 D-04 left typed-ID stubs).

9. **New config keys** under `semantic_index.live_updates.*`:
   - `watcher_enabled` (default `true`)
   - `manifest_scan_enabled` (default `true`)
   - `manifest_scan_interval` (default `"10s"`)
   - All other keys (`enabled`, `debounce_ms`, `max_batch_delay_ms`, `bulk_change_threshold`, `compact_after_idle_ms`, `lsp_revalidate_after_idle_ms`, `lsp_compaction_max_wait_ms`, `max_overlay_files`, `max_overlay_age`) already defined in SPEC §25 — Phase 60 wires defaults through `internal/config/defaults.go` and `SerenaConfig.SemanticIndex`.

10. **Bounded-label metric** `helix_semantic_live_updates_total{kind, outcome}` with closed-enum labels (Phase 57 D-07 path).

11. **Editor-fixture test suite** asserting LIVE-02 — Vim swap-rename, JetBrains `___jb_tmp___`+rename, VS Code atomic-rename — overlay catches every save.

**Out of scope (deferred):**

- **LSP enrichment worker** — Phase 61. SPEC §16.3's `LSPQueue.Enqueue(RevalidateFileJob{...})` is a typed buffered handoff in Phase 60 (the queue exists, the consumer doesn't).
- **Compaction with `overlay_epoch` CAS** — Phase 63. Phase 60 ships the epoch contract and CAS read points; Phase 63 implements the compactor.
- **Graph cache repair / score invalidation** — Phase 62. SPEC §16.3 `GraphCache.ApplyRepair`, `MarkAffectedScoresAndClusters` are typed no-op stubs.
- **`semantic_graph_status` MCP tool wrapper** — Phase 64.
- **`refresh_semantic_graph` MCP tool** — Phase 64.
- **`get_health` strangler-fig integration** — Phase 65 surfaces watcher status; Phase 60 ships only the health-data accessor on the watcher manager.
- **MCP push notifications** for watcher state changes — no current consumer; deferred to Phase 64+.

</domain>

<decisions>
## Implementation Decisions

### Change sources & normalization (architecture spine)

- **D-01: Three change sources, single normalized signal type.**
  Filesystem state is truth. Helix's own edit hook is a fast notification path,
  not the source of truth. Watcher catches external edits (LLM-via-other-tool,
  IDE writes, shell, git checkout). Manifest scan is the correctness recovery
  path that runs unconditionally.

  All three sources produce:

  ```go
  // internal/semantic/live/signal.go (or sibling)
  type ChangeSource string
  const (
      ChangeSourceHelixEdit    ChangeSource = "helix_edit"
      ChangeSourceFsnotify     ChangeSource = "fsnotify"
      ChangeSourceManifestScan ChangeSource = "manifest_scan"
  )

  type WorkspaceChangeSignal struct {
      WorkspaceID workspace.WorkspaceKey
      Paths       []string
      Source      ChangeSource
      ObservedAt  time.Time
  }
  ```

  Semantic owns classification and event construction:

  ```go
  func (s *liveService) OnWorkspaceChanged(ctx context.Context, sig WorkspaceChangeSignal) error {
      paths := NormalizeAndDeduplicate(sig.Paths) // resolve to repo-relative, dedupe
      for _, path := range paths {
          kind := s.ClassifyPathChange(ctx, sig.WorkspaceID, path)
          s.liveQueue.Enqueue(SourceChangeEvent{
              RepoID:     s.ResolveRepoID(sig.WorkspaceID),
              Kind:       kind,
              Path:       path,
              Source:     string(sig.Source),
              ObservedAt: sig.ObservedAt,
          })
      }
      return nil
  }
  ```

  Classification rules (read filesystem + compare against
  `semantic_files.content_hash`):

  ```text
  path exists, known before, hash changed     → ChangeFileModified
  path exists, unknown before                 → ChangeFileCreated
  path missing, known before                  → ChangeFileDeleted
  old missing + new exists + content lineage  → ChangeFileRenamed (best-effort)
  source = helix_edit                         → ChangeHelixEdit
                                                (subkind in metadata; otherwise
                                                fall through to modified/created)
  many paths in same flush > threshold        → collapses to ChangeBulkUpdate at
                                                CoalesceEvents time, NOT per-path
  ```

  **Hard invariants:**
  - All change sources MUST normalize to `WorkspaceChangeSignal` before
    entering semantic. No source-specific shortcuts that bypass classification.
  - `ClassifyPathChange` is the SINGLE site that decides the `Kind` field
    of `SourceChangeEvent`. Sources MUST NOT pre-classify.
  - The watcher and manifest scanner do not import each other; they're
    independent producers feeding the same downstream.

- **D-02: Per-workspace single-goroutine coalescer with sequential dispatch.**

  ```go
  // internal/semantic/live/coalescer.go (or sibling)
  type Coalescer struct {
      workspaceID workspace.WorkspaceKey
      events      chan SourceChangeEvent
      timer       *time.Timer       // time.AfterFunc(debounce_ms)
      pending     map[string]SourceChangeEvent
      handler     EventHandler
      cfg         CoalescerConfig
      logger      *slog.Logger
  }
  ```

  Mirrors the proven pattern in `internal/memory/watcher.go` (debounce timer +
  per-flush dispatch). One per-workspace goroutine drains `events`, accumulates
  into `pending`, fires `time.AfterFunc(cfg.DebounceMs)` to flush. On flush:

  1. Snapshot `pending`, reset map.
  2. Call SPEC §16.2 `CoalesceEvents([]SourceChangeEvent) []SourceChangeEvent`
     (pure function — same package so unit-testable in isolation).
  3. If `len(result) > cfg.BulkChangeThreshold` (default 200), emit a single
     `SourceChangeEvent{Kind: ChangeBulkUpdate}` instead of the per-path slice.
  4. Sequential dispatch — each event goes to `UpdateChangedFile` /
     `HandleFileDeleted` / `HandleFileRenamed` / `HandleBulkUpdate` per kind.
     Errors logged with structured slog; per-event failures do NOT abort the
     batch (other files continue).

  **Bulk collapse fires at flush-time only.** No mid-window overflow path —
  the `time.AfterFunc(max_batch_delay_ms)` cap (default 1500ms) bounds worst
  latency under continuous storms. Simpler invariant: one collapse decision
  point, in `CoalesceEvents`.

  **Hard invariants:**
  - One goroutine per workspace. No per-file fan-out (cross-path coalescing
    requires single-loop ownership).
  - Sequential dispatch — no worker pool. `bulk_update` collapse already
    bounds the worst case; per-file parallelism is a Phase 61+ optimization
    if benchmarks demand it.
  - `CoalesceEvents` is pure (input slice → output slice); no side effects.
    Unit-testable without spinning up the watcher.

### Kernel→semantic decoupling

- **D-03: `EditNotifier` interface in kernel, semantic registers via setter.**

  ```go
  // internal/kernel/notifier.go (new file)
  type EditNotifier interface {
      OnEdit(ctx context.Context, workspaceID workspace.WorkspaceKey, paths []string) error
  }
  ```

  The interface lives in `internal/kernel/` so kernel can call it without
  importing semantic. Daemon bootstrap (after Phase 57 step 6b + Phase 59
  semantic store/scheduler open) constructs the semantic-side implementation
  and calls `kernel.SetEditNotifier(notifier)` — exactly mirrors the
  `SetEnrichFn` / `SetActivateCallback` pattern already in
  `internal/daemon/daemon.go`.

  Edit tools call the hook **fire-and-forget**:

  ```go
  // example: internal/kernel/edit/replace.go (sketch)
  func (h *replaceHandler) Handle(...) (...) {
      // ... do the edit ...
      if n := h.kernel.EditNotifier(); n != nil {
          // synchronous call — semantic impl MUST return immediately
          // (just enqueues into the per-workspace coalescer queue).
          _ = n.OnEdit(ctx, h.kernel.ActiveWorkspace(), []string{h.relPath})
      }
      return result, nil
  }
  ```

  The semantic-side impl:

  ```go
  func (l *liveService) OnEdit(ctx context.Context, ws workspace.WorkspaceKey, paths []string) error {
      // Normalize, build WorkspaceChangeSignal, enqueue into per-workspace coalescer.
      // MUST return immediately — no extraction, no I/O, no blocking.
      l.signal(ctx, WorkspaceChangeSignal{
          WorkspaceID: ws,
          Paths:       paths,
          Source:      ChangeSourceHelixEdit,
          ObservedAt:  l.now(),
      })
      return nil
  }
  ```

  **Wired into 8 entry points** (LIVE-07 enumerates 6 + 2):
  - `internal/kernel/edit/`: `replace_symbol_body`, `insert_before_symbol`,
    `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`.
  - `internal/kernel/fileops/`: `replace_in_file`, `fuzzy_edit`, `write_file`.

  (LIVE-07 lists 6 edit tools + `replace_in_file` + `fuzzy_edit` = 8.
  `write_file` is included for completeness — the user's "filesystem state is
  truth" framing under D-01 means every Helix-originated write needs the fast
  signal even when it's not a symbol-aware edit.)

  **Hard invariants:**
  - `internal/kernel/` MUST NOT import `internal/semantic/...`. Verified by
    build-time greppable check (Phase 60 may add a small `vet` analyzer or
    integration test; the existing `cmd/vet-noduckdb/` pattern is the
    template if a new analyzer is needed).
  - `EditNotifier.OnEdit` MUST return without blocking on extraction or I/O.
    The semantic-side impl is a queue enqueue + return.
  - `OnEdit` errors are non-fatal — edit tools log and continue. The
    fast-path signal is an optimization; the watcher + manifest scan provide
    correctness coverage.

### Overlay epoch & schema

- **D-04: Per-tx `overlay_epoch` allocation, schema migration v2→v3.**

  Each `BeginOverlayTx` atomically increments the per-workspace `current_epoch`
  under a per-workspace lock and stamps every row written in that tx with
  `write_epoch = current_epoch`. No-op coalesced batches (where `CoalesceEvents`
  returns an empty slice or every event resolves to a no-op) DO NOT open a
  transaction → no epoch consumed.

  **Schema migration v2→v3** flows through Phase 57 D-02's migration registry
  (`internal/semantic/store/migrations.go`). The migration:

  ```sql
  -- Add monotonic epoch to overlay metadata
  ALTER TABLE semantic_live_overlay_meta
    ADD COLUMN current_epoch UBIGINT NOT NULL DEFAULT 0;

  -- Stamp every overlay row with the epoch under which it was written
  ALTER TABLE semantic_live_overlay_files
    ADD COLUMN write_epoch UBIGINT NOT NULL DEFAULT 0;
  ALTER TABLE semantic_live_overlay_symbols
    ADD COLUMN write_epoch UBIGINT NOT NULL DEFAULT 0;
  ALTER TABLE semantic_live_overlay_references
    ADD COLUMN write_epoch UBIGINT NOT NULL DEFAULT 0;
  ALTER TABLE semantic_live_overlay_edges
    ADD COLUMN write_epoch UBIGINT NOT NULL DEFAULT 0;

  CREATE INDEX idx_overlay_files_write_epoch
    ON semantic_live_overlay_files(repo_id, write_epoch);
  CREATE INDEX idx_overlay_symbols_write_epoch
    ON semantic_live_overlay_symbols(repo_id, write_epoch);
  CREATE INDEX idx_overlay_references_write_epoch
    ON semantic_live_overlay_references(repo_id, write_epoch);
  CREATE INDEX idx_overlay_edges_write_epoch
    ON semantic_live_overlay_edges(repo_id, write_epoch);
  ```

  **Phase 63 CAS contract** (Phase 60 documents and ships the read points;
  Phase 63 implements the compactor):

  ```text
  1. Compaction reads meta.current_epoch → captured_epoch
  2. Compaction reads overlay rows with write_epoch ≤ captured_epoch
  3. Compaction merges those rows into the snapshot
  4. ClearOverlay deletes rows WHERE write_epoch ≤ captured_epoch
  5. Rows with write_epoch > captured_epoch (committed during compaction)
     remain in the overlay for the next compaction cycle
  ```

  **Hard invariants:**
  - `current_epoch` is monotonically non-decreasing per workspace (no
    rollbacks on tx abort — the increment commits independently of row
    writes; if the tx rolls back, that epoch is just unused).
  - Concurrent `BeginOverlayTx` calls on the same workspace serialize on
    the per-workspace lock; the registry of locks lives in the overlay
    writer, keyed by `repo_id`.
  - Cross-workspace `BeginOverlayTx` calls do NOT serialize against each
    other.
  - Empty coalesced flushes do not advance epoch — keeps the sequence
    dense and the `overlay_files` table count meaningful for Phase 64
    status surfaces.

### Watcher resilience

- **D-05: `slog.Warn` + `get_health` for ENOSPC; reuse `semantic_files.content_hash` for manifest scan.**

  **Linux inotify `ENOSPC`:**
  - Detect at `fsnotify.NewWatcher()` or `Add()` failure with `syscall.ENOSPC`.
  - Emit one structured `slog.Warn` with workspace, error, remediation hint
    (`fs.inotify.max_user_watches` sysctl).
  - Watcher manager exposes `Status() WatcherStatus` with fields
    `Active bool`, `Reason string`, `LastError error`, `RemediationHint string`.
  - Phase 65 `get_health` integration surfaces this as
    `watcher_status="unavailable"`, `reason="inotify_enospc"`.
  - `SemanticIndexState` STAYS `ready` — manifest scan keeps the overlay
    correct, just at higher latency. Promoting to `stale` would lie about
    correctness.

  **Manifest scanner:**
  - Walks the workspace per Phase 59 D-04 file-discovery rules
    (`SPEC-DRAFT.md` §13.2 exclusions: `.git/`, `node_modules/`, `vendor/`,
    `dist/`, `build/`, `target/`, `coverage/`, files above
    `indexing.max_file_size`).
  - For each file: hash with the same function Phase 59 uses (xxhash64
    via `github.com/cespare/xxhash/v2` per Phase 59 D-canonical-refs).
  - Compare against the effective `semantic_files.content_hash` (Phase 59
    populates this column).
  - Mismatches → enqueue as `WorkspaceChangeSignal{Source: manifest_scan}`
    → classifier decides modified/created/deleted.
  - Files in `semantic_files` but missing from disk → deleted.
  - Files on disk but not in `semantic_files` → created.
  - Default cadence: `live_updates.manifest_scan_interval = "10s"`.
  - Always on (not gated on watcher state) — ENOSPC and watcher misses are
    invisible to the scanner.
  - **Walk concurrency:** reuse Phase 59's `extraction.max_parallel_files`
    default (4) for hashing parallelism. Tunable via config if benchmarks
    demand it.

  **Hard invariants:**
  - Manifest scan does not write directly to overlay — it goes through the
    same `WorkspaceChangeSignal` → classifier → coalescer pipeline as the
    watcher and helix_edit hook.
  - Manifest scan does NOT touch `semantic_watcher_manifest` (no such
    table). It reads `semantic_files.content_hash` only. Single source of
    truth.
  - Manifest scan respects `live_updates.enabled = false` — when disabled
    in config, NEITHER watcher nor scanner runs (the live-updates feature
    is off entirely). When `live_updates.enabled = true`, both run by
    default; either can be individually disabled via `watcher_enabled` /
    `manifest_scan_enabled` for tests or constrained environments.

### Pipeline integration

- **D-06: Phase 60 fills `phasegraph.pipelines/live.go` Run bodies.**
  Phase 57 D-04 left typed phase ID constants and `Requires`/`Provides`
  declarations in `internal/phasegraph/pipelines/live.go` with empty
  `Run`/`Validate` bodies. Phase 60 fills these to wire the live-update
  pipeline into the daemon's bootstrap-time phase graph. The shape (typed
  IDs, dependency edges) does not change — only `Run`/`Validate`/`Shutdown`
  bodies.

- **D-07: Phase 60 fills Phase 59's `ScheduleIncremental(workspaceID, []FileChange) JobID`.**
  Phase 59 D-04 shipped the `ExtractionScheduler` interface with
  `ScheduleIncremental` documented as "body landed in Phase 60". Phase 60
  implements this method on the existing scheduler — it accepts a slice of
  `FileChange` (path + kind + content hash) and feeds them through the same
  re-extraction path the live handlers use (`UpdateChangedFile`,
  `HandleFileDeleted`, `HandleFileRenamed`).

  This is the in-process API the dispatcher (D-02) calls into. External
  callers (CLI subcommands, Phase 64 MCP tools) get the same path.

### Acceptance criteria (must hold at end of phase)

1. `internal/kernel/` does not import `internal/semantic/...` — verified by
   greppable check (or new vet analyzer if the planner judges it warranted).
2. `EditNotifier.OnEdit` returns within `O(microseconds)` — verified by an
   integration test that wires a slow semantic impl and asserts the kernel
   edit tool returns before extraction completes.
3. The editor-fixture test suite covers Vim swap-rename, JetBrains
   `___jb_tmp___`+rename, VS Code atomic-rename — overlay catches the change
   for each editor; test asserts the post-save `semantic_live_overlay_files`
   row exists with correct content hash.
4. `CoalesceEvents` is a pure function with unit tests covering each merge
   rule from SPEC §16.2 (`modified+modified→modified`, `created+deleted→no-op`,
   `created+modified→created`, `modified+deleted→deleted`,
   `deleted+created→modified`, `rename`, `> threshold → bulk_update`).
5. `bulk_update` collapse fires when raw event count exceeds
   `live_updates.bulk_change_threshold` (default 200) AT FLUSH TIME ONLY.
   Asserted by an integration test that fires 250 events within the debounce
   window and asserts a single `bulk_update` event reaches the dispatcher.
6. Schema version stamps move from `2` (Phase 59) to `3` (Phase 60); the
   migration succeeds on a clean store, on a Phase-57-populated store, and
   on a Phase-59-populated store.
7. Per-tx `overlay_epoch` advancement: a stress test that fires N
   concurrent `BeginOverlayTx` calls on the same workspace asserts each tx
   gets a unique `write_epoch` and rows tagged accordingly.
8. No-op coalesced batches do not advance epoch — asserted by injecting a
   `created+deleted` pair (which `CoalesceEvents` resolves to no-op) and
   confirming `meta.current_epoch` is unchanged after the flush.
9. Linux `ENOSPC` simulation: `fsnotify.Add` returning `syscall.ENOSPC`
   triggers exactly one `slog.Warn`; `Status()` returns
   `Active=false, Reason="inotify_enospc"`; manifest scan continues to
   produce events.
10. Manifest scan detects watcher misses: a test that disables the watcher,
    edits files externally, and asserts the scanner produces the right
    `WorkspaceChangeSignal` within `2 × manifest_scan_interval` (one cycle
    plus debounce slack).
11. `helix_semantic_live_updates_total{kind, outcome}` metric emits with
    the closed-enum labels `kind ∈ {file_created, file_modified, file_deleted,
    file_renamed, helix_edit, bulk_update}` and `outcome ∈ {applied,
    no_op, error, dropped}`. Bounded labels registered through the existing
    `internal/obs/` path.
12. The 8 kernel edit/fileops tools each emit an `EditNotifier.OnEdit` call
    on success — verified by per-tool integration tests with a mock notifier
    asserting the call.
13. `LIVE-01` through `LIVE-07` requirements all marked `Done` in
    `.planning/REQUIREMENTS.md` after Phase 60 close.

### Claude's Discretion (no user input needed)

- **fsnotify library:** reuse `github.com/fsnotify/fsnotify` already used by
  `internal/memory/watcher.go`. No new dependency.
- **Concrete package layout:** `internal/semantic/live/` as the umbrella for
  watcher, coalescer, manifest scanner, classifier, dispatcher, and overlay
  writer. Sub-packages permitted (`internal/semantic/live/watcher/`,
  `internal/semantic/live/scanner/`, etc.) at the planner's call. The
  overlay writer fills `internal/semantic/store/overlay.go` (the existing
  Phase 57 stub) and may grow into `internal/semantic/store/overlay/` if
  the planner judges it warrants its own subdirectory — the constraint is
  it stays inside `internal/semantic/store/` so the existing
  `cmd/vet-noduckdb/` analyzer protects the duckdb-go boundary.
- **Trace span names** per SPEC §28.2: `semantic.live.coalesce`,
  `semantic.live.update_file`, `semantic.live.graph_repair` (no-op stub
  span; Phase 62 fills body), `semantic.live.revalidate_file` (no-op stub
  span; Phase 61 fills body), `semantic.live.compact_overlay` (no-op stub
  span; Phase 63 fills body).
- **Plan layout:** the planner decides wave structure. Suggested 4 plans:
  P01 schema migration v2→v3 + overlay writer + epoch contract; P02
  watcher + manifest scanner + classifier + coalescer; P03 EditNotifier
  interface + 8 tool wiring + ScheduleIncremental fill-in; P04
  editor-fixture LIVE-02 test suite + integration tests + metric wiring.
  Planner may collapse or split.
- **fsnotify event-to-signal translation:** the watcher's translation of
  `fsnotify.Op` flags (Create | Write | Remove | Rename | Chmod) to a path
  list is the watcher's internal concern. The output crossing the
  watcher→classifier boundary is always a flat `[]string` of paths, per
  D-01.
- **Atomic-rename detection:** for JetBrains' `___jb_tmp___` pattern,
  detect by suffix and re-attach the watcher to the renamed-to path. For
  VS Code atomic save, fsnotify's Rename event already triggers re-attach
  via the directory-level watch. Implementation detail; tests in (3) above
  validate.
- **`LSPQueue.Enqueue` from SPEC §16.3:** typed buffered handoff in
  Phase 60 — the channel/queue exists and accepts `RevalidateFileJob`
  items, but no consumer goroutine drains it. Phase 61 wires the
  enrichment worker to drain.
- **`GraphCache.ApplyRepair`, `MarkAffectedScoresAndClusters` from SPEC
  §16.3:** typed no-op stubs; Phase 62 fills bodies. Phase 60 calls them
  with the computed `repair` so Phase 62 has the data flow already wired.
- **Hash function for manifest scan:** reuse Phase 59's xxhash64
  implementation. Same canonical hash means file content equality is
  symmetrically detectable across extract and scan.
- **Lock granularity:** per-workspace mutex for `BeginOverlayTx` epoch
  allocation. No global lock. Workspaces are independent.
- **Manifest scan walk concurrency:** default 4 (matches Phase 59
  `extraction.max_parallel_files`). Tunable via config if benchmarks
  demand.
- **`ChangeFileRenamed` detection:** best-effort. fsnotify provides Rename
  events; manifest scan can detect rename via "old missing + new exists +
  same content hash" heuristic. If not detected, falls back to
  `delete + create` per SPEC §16.5 fallback path (which is correct, just
  loses identity preservation).
- **Bounded-label outcomes:** `applied | no_op | error | dropped`.
  `dropped` covers events discarded due to ENOSPC, overflow protection,
  or workspace shutdown.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements (load-bearing)

- `.planning/REQUIREMENTS.md` §LIVE — LIVE-01 through LIVE-07 phrasing is
  the contract. Each requirement has its acceptance test (D-acceptance #1–#13).
- `.planning/milestones/v1.10-ROADMAP.md` Phase 60 block (lines 99–109) —
  Goal, Depends on, Requirements, 5 Success Criteria.

### Specification (source of truth for shapes and rules)

- `SPEC-DRAFT.md` §9.11 — Live Overlay Tables. Phase 60's schema migration
  v2→v3 extends `semantic_live_overlay_meta` (`current_epoch`) and the
  four overlay fact tables (`write_epoch`).
- `SPEC-DRAFT.md` §16 — Live Update Pipeline:
  - §16.1 — `SourceChangeKind` enum + `SourceChangeEvent` struct (the wire
    type the coalescer consumes).
  - §16.2 — Coalesce + debounce rules. `CoalesceEvents` is implemented as
    a pure function exactly per the pseudocode.
  - §16.3 — `UpdateChangedFile` flow. Phase 60 implements; the
    `LSPQueue.Enqueue`, `GraphCache.ApplyRepair`,
    `MarkAffectedScoresAndClusters` calls are typed no-op stubs (see
    Claude's Discretion).
  - §16.4 — `HandleFileDeleted` flow.
  - §16.5 — `HandleFileRenamed` flow with content-hash lineage detection.
- `SPEC-DRAFT.md` §25 — `semantic_index.live_updates.*` config keys.
  Phase 60 adds the three new keys (`watcher_enabled`,
  `manifest_scan_enabled`, `manifest_scan_interval`); the rest are
  already declared in SPEC and just need defaults wired through
  `internal/config/defaults.go`.
- `SPEC-DRAFT.md` §27.2 — Watcher misses; Phase 60 mitigations (manifest
  scan = required, content hash check on semantic query, manual refresh
  via Phase 64 tool).
- `SPEC-DRAFT.md` §28.1/§28.2 — `helix_semantic_live_updates_total` metric
  + `semantic.live.*` trace spans.
- `SPEC-DRAFT.md` §29.5 — Rapid Edit Storm — coalesce + bulk_update is the
  documented mitigation; Phase 60 ships both.
- `SPEC-DRAFT.md` §32 Phase 2 — "Live Overlay Updates" deliverables and
  acceptance criteria (matches Phase 60 Success Criteria).

### Phase 57 lock-down (must not regress)

- `.planning/phases/57-semantic-store-foundation-pipeline-dag-library/57-CONTEXT.md`
  — D-02 (explicit migration registry — Phase 60's v2→v3 migration MUST
  flow through this), D-04 (typed phase ID stubs in `pipelines/live.go`
  Phase 60 fills), D-07 (bounded-label metric registration path).
- `internal/semantic/store/overlay.go` — the empty Phase 57 stub Phase 60
  fills with the `BeginOverlayTx` API.
- `internal/semantic/store/migrations.go` — current schema (v1 from
  Phase 57, v2 from Phase 59). Phase 60's migration v2→v3 plugs into the
  existing `Migration{From, To, Kind}` registry.
- `internal/semantic/store/duckdb.go` — fact-store open path. Phase 60
  does not change open semantics, just adds tables.

### Phase 59 lock-down (must not regress)

- `.planning/phases/59-tree-sitter-extraction-stable-symbol-ids/59-CONTEXT.md`
  — D-01 (no `internal/repomap` import inside `internal/semantic/extract/`;
  Phase 60's classifier and re-extraction path inherit this), D-02
  (constructor injection — Phase 60 follows the same pattern for watcher
  manager, coalescer, scanner, overlay writer), D-04 (`ExtractionScheduler`
  interface — Phase 60 fills the `ScheduleIncremental` body), D-05
  (partial extraction model — Phase 60's re-extraction uses the same
  `extraction_status` taxonomy).
- `internal/semantic/types.go` — typed identifiers Phase 60 may extend
  (e.g., `WorkspaceChangeSignal`, `EditKind`).

### Architectural invariants

- `CLAUDE.md` "Middleware Execution Order (LIFO)" — Phase 60 does not
  touch middleware; live-update plumbing lives below the MCP layer.
- `internal/treesitter/registry_cgo.go` — Phase 60's re-extraction reuses
  the singleton `*treesitter.GrammarRegistry` injected at daemon
  bootstrap (BUG-04 / EXTRACT-05 invariant).
- `internal/daemon/daemon.go` — Phase 60 wiring lands after Phase 59's
  scheduler construction; new bootstrap calls:
  `kernel.SetEditNotifier(semanticLive)`, watcher manager `Start(ws)` per
  workspace activation, manifest scanner kick on first `RequireReady`.

### Pattern templates (must mirror)

- `internal/memory/watcher.go` — fsnotify watcher pattern reference
  (debounce timer, recursive add, per-event dispatch). Phase 60 mirrors
  the structure for the workspace-source watcher.
- `internal/daemon/daemon.go` `SetEnrichFn` / `SetActivateCallback` —
  Phase 60's `kernel.SetEditNotifier(...)` follows the same setter
  pattern.
- `internal/kernel/edit/tools.go:131` `RegisterTools(...)` — Phase 60
  threads notifier access through the same kernel handle these tools
  already receive.
- `internal/kernel/fileops/tools.go:169` `RegisterTools(...)` — same
  pattern for `replace_in_file`, `fuzzy_edit`, `write_file`.
- `internal/repomap/extractor.go` — example file walker with ignore rules
  (Phase 59 D-04 reused these). The manifest scanner mirrors the walker
  shape but does NOT import repomap (Phase 59 D-01 invariant cascades).

### Validation tooling

- `cmd/vet-noduckdb/` — Phase 57 vet-tool. Phase 60's
  `internal/semantic/live/...` packages MUST NOT import `duckdb-go`
  directly — they go through `internal/semantic/store/`.

### Test fixtures (must seed)

- `SPEC-DRAFT.md` §31.3 — Live Update Golden Example. Use as the spine
  for the integration test asserting "single-file edit → effective graph
  updates without full reindex".
- New: editor-fixture testdata under
  `internal/semantic/live/testdata/editors/` for Vim swap-rename,
  JetBrains `___jb_tmp___`+rename, VS Code atomic-rename. Each fixture
  is a small Go program / shell script that performs the editor's save
  dance against a target file; the test runs it under the watcher and
  asserts the overlay catches the change.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`internal/memory/watcher.go`** — fsnotify-based watcher with debounce
  + recursive add. Closest existing pattern; Phase 60's watcher mirrors
  the structure (per-workspace goroutine, `time.AfterFunc(debounce)`,
  pending-event map).
- **`*treesitter.GrammarRegistry`** (Phase 49 BUG-04 singleton) — re-used
  for re-extraction in `UpdateChangedFile`. Daemon bootstrap injects;
  Phase 60 does NOT create a second registry.
- **Phase 59 `extract.Registry`** — re-extraction path. Live update
  handlers call the same provider per language Phase 59's initial
  extraction uses.
- **Phase 59 `ExtractionScheduler`** — Phase 60 fills the
  `ScheduleIncremental(workspaceID, []FileChange) JobID` body the
  interface exposes; same scheduler instance.
- **Phase 57 `internal/semantic/store/`** — fact store. Phase 60 fills
  the empty `overlay.go` stub with the `BeginOverlayTx` API and adds the
  v2→v3 migration to the existing migration registry.
- **Phase 57 `internal/phasegraph/pipelines/live.go`** — typed phase ID
  stubs Phase 60 fills.
- **`internal/obs/`** — bounded-label metric registration; Phase 60
  registers `helix_semantic_live_updates_total` through this path.
- **`internal/config/`** — koanf 4-layer precedence; new
  `live_updates.*` defaults added to `defaults.go` and the
  `SerenaConfig.SemanticIndex.LiveUpdates` struct field (already
  reserved in Phase 57 P02/P03).
- **`github.com/cespare/xxhash/v2`** — already in dependency graph;
  manifest scanner reuses Phase 59's hash function.
- **`internal/kernel/edit/`** + **`internal/kernel/fileops/`** — 8 entry
  points for the `EditNotifier.OnEdit` call. Existing tool registration
  functions (`RegisterTools`) already accept the kernel handle Phase 60
  threads the notifier through.

### Established Patterns

- **Constructor injection** (Phase 59 D-02). Phase 60's watcher manager,
  coalescer, manifest scanner, overlay writer all constructed with their
  dependencies passed in — no `init()` registration, no blank imports.
- **Setter-style cross-package wiring** (`SetEnrichFn`,
  `SetActivateCallback` in `internal/daemon/daemon.go`). Phase 60's
  `kernel.SetEditNotifier(...)` follows this exact pattern.
- **Bounded-label metrics with closed enum** (Phase 57 D-07). `kind` and
  `outcome` labels on the new live-updates counter both come from closed
  enums.
- **Per-feature defaults test** (Phase 57 D-09, Phase 59 inheritance).
  Phase 60 adds `TestLoad_LiveUpdatesDefaults` for the three new keys.
- **Schema migration via registry** (Phase 57 D-02). Phase 60's v2→v3
  migration is one entry in the registry, with up/down SQL and an
  in-place `Kind`.

### Integration Points

- **Daemon bootstrap (`internal/daemon/daemon.go`)** — after Phase 59's
  scheduler construction, Phase 60 adds:
  - Construct `live.Service` (overlay writer, coalescer factory,
    classifier, EditNotifier impl).
  - Call `kernel.SetEditNotifier(liveService)`.
  - Construct `live.WatcherManager` (per-workspace watcher pool).
  - Construct `live.Scanner` (per-workspace manifest scanner pool).
  - On `kernel.ActivateWorkspace`: register the workspace with the
    watcher manager, scanner, and coalescer; pass into the scheduler so
    `ScheduleIncremental` reaches the right per-workspace queue.
  - Register `helix_semantic_live_updates_total` via `internal/obs/`.
- **`SerenaConfig.SemanticIndex.LiveUpdates`** field — already reserved
  in Phase 57. Phase 60 populates the three new koanf keys via
  `internal/config/defaults.go` and the per-feature defaults test.
- **`internal/kernel/edit/tools.go`** + **`internal/kernel/fileops/tools.go`**
  — each of the 8 tool handlers gets a single-line `OnEdit` call inserted
  at the success path. The kernel handle they already hold provides
  access to the notifier.

### Constraints

- Phase 60 must NOT touch middleware order (CLAUDE.md "Middleware Execution
  Order (LIFO)"). Live-update plumbing is below the MCP layer.
- Phase 60 must NOT introduce a second `GrammarRegistry`
  (BUG-04 / EXTRACT-05). Re-extraction reuses the daemon-injected
  singleton.
- Phase 60 must NOT import `internal/semantic/...` from `internal/kernel/`
  (LIVE-07). The `EditNotifier` interface lives in `internal/kernel/`;
  the implementation lives in `internal/semantic/live/`.
- Phase 60 must NOT import `internal/repomap/` from
  `internal/semantic/...` (Phase 59 D-01 cascade). The manifest scanner
  walker is structurally similar to repomap's walker but is net-new code.
- Phase 60 must NOT import `duckdb-go` outside `internal/semantic/store/`
  (Phase 57 D-12 + `cmd/vet-noduckdb/` analyzer). Live packages go
  through the store API.
- Phase 60 must NOT pre-classify change kinds at the source
  (D-01 invariant). All sources produce `WorkspaceChangeSignal` with
  paths only; semantic owns classification.
- Phase 60 must NOT block kernel edit tools on extraction
  (D-03 invariant). `EditNotifier.OnEdit` returns immediately after
  enqueueing.

</code_context>

<specifics>
## Specific Ideas

- **The user's "filesystem state is truth" framing** is the spine. The
  three change sources (`helix_edit`, `fsnotify`, `manifest_scan`) are
  redundant by design — any one of them missing must not cause data loss.
  The redundancy is the correctness story; per-source latency is the
  user-experience story.
- **Helix edit hook is an optimization, not the source of truth.**
  External LLM/IDE/shell writes hit the same path via fsnotify or
  manifest scan. This means Helix edits and external edits look identical
  to semantic — same classifier, same coalescer, same handlers.
- **Manifest scan is required, not best-effort** (LIVE-03 phrasing). It
  doubles as the ENOSPC fallback (D-05) and the watcher-miss recovery
  path (SPEC §27.2). Default cadence 10s is small enough to be invisible
  to most workflows but large enough to not thrash the disk on big repos.
- **`bulk_update` collapse keeps git-checkout storms tractable.** A
  10k-file checkout produces one `bulk_update` event that schedules an
  incremental re-walk, not 10k overlay writes. The scheduler's existing
  initial-walk priority order (Phase 59 D-04) governs the re-walk.
- **`overlay_epoch` is one of the load-bearing mechanisms for Phase 63.**
  Phase 60's contract is small but precise: per-tx allocation, monotone,
  stamped on every fact row. Phase 63 reads it under CAS during
  compaction. Getting the contract wrong here = silent overlay row loss
  in Phase 63.
- **Editor-fixture tests are a hard gate.** LIVE-02 explicitly requires
  Vim, JetBrains, VS Code save patterns are caught. Skipping these tests
  fails the phase regardless of unit-test coverage.

</specifics>

<deferred>
## Deferred Ideas

- **MCP push notifications for watcher state changes** — JSON-RPC
  notification to active sessions on watcher health transitions.
  Deferred because no current MCP client subscribes; revisit when
  Phase 64 lands the new MCP tools.
- **Per-file fan-out / worker-pool dispatch** — D-02 chose sequential
  dispatch. If Phase 61+ benchmarks show the dispatcher is the
  bottleneck under realistic edit storms, revisit. The `bulk_update`
  collapse is the first-line defense; a worker pool is the second.
- **`semantic_watcher_manifest` standalone table** — D-05 reuses
  `semantic_files.content_hash`. If the planner discovers a query
  pattern where joining against `semantic_files` is too expensive (e.g.
  scanner startup cost on a million-file workspace), a sidecar table
  with just (path, hash) becomes the optimization. Not now.
- **`refresh_semantic_graph` MCP tool** — Phase 64. Phase 60 ships only
  the underlying `ScheduleIncremental` path; Phase 64 wraps as MCP.
- **`get_semantic_graph_status` MCP tool wrapper** — Phase 64. Phase 60
  ships the `Status()` data accessor on the watcher manager and live
  service; Phase 64 wraps.
- **`get_health` watcher status integration** — Phase 65 strangler-fig.
  Phase 60 ships the `WatcherStatus` data accessor; Phase 65 wires it
  into the existing `get_health` tool.
- **Cross-workspace event coordination** — Phase 60 treats workspaces as
  independent. Coalescer is per-workspace, locks are per-workspace,
  epochs are per-workspace. Multi-workspace coordination (e.g., a single
  git submodule across two workspaces) is out of scope.
- **fsnotify replacement** — fsnotify works on Linux/macOS/Windows but
  has known quirks (no recursive watches on Linux, kqueue limits on
  macOS). Phase 60 ships with fsnotify; if a quirk forces a rewrite,
  it's a separate phase.
- **Detecting renames across content edits** — SPEC §16.5 falls back to
  `delete + create` if content hash changed during the rename. Detecting
  "rename + small edit" (e.g., a developer touch-up after `git mv`) is
  not in scope; the fallback is correct, just loses identity preservation.
- **Generic "edit kind" subtyping for ChangeHelixEdit** — D-03 hook
  payload is paths-only. If the user later wants per-edit-kind metrics
  (e.g., `rename_symbol` vs `replace_in_file`), it's a small additive
  extension to `WorkspaceChangeSignal`. Not now.
- **Adaptive `manifest_scan_interval`** — fixed 10s by default. Future
  phase could back-off when watcher is healthy and shorten when watcher
  is degraded. Not now.

</deferred>

---

*Phase: 60-live-update-pipeline*
*Context gathered: 2026-05-05*
