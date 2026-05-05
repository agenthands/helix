# Phase 60: Live Update Pipeline - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `60-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-05-05
**Phase:** 60-live-update-pipeline
**Areas discussed:** Coalescer & queueing model, postEditHook wiring (kernel→semantic decoupling), overlay_epoch semantics & storage, Watcher resilience (ENOSPC + scrub)

---

## Coalescer & queueing model

### Pipeline structure

| Option | Description | Selected |
|--------|-------------|----------|
| Single goroutine, sequential dispatch | One per-workspace goroutine: drain fsnotify → time.AfterFunc(debounce_ms) → CoalesceEvents → dispatch sequentially per path. Mirrors internal/memory/watcher.go. | ✓ |
| Single coalescer + bounded worker pool | Coalescer goroutine produces a deduped batch; a small worker pool (e.g. max_parallel_files=4) executes UpdateChangedFile calls concurrently. | |
| Per-file fan-out | Events sharded by path into per-file goroutines that each maintain their own debouncer. | |

**User's choice:** Single goroutine, sequential dispatch
**Notes:** Bulk-update collapse already protects the worst case; per-file parallelism deferred.

### Bulk trigger

| Option | Description | Selected |
|--------|-------------|----------|
| At flush-time after coalesce | After debounce window flushes and coalesce dedupes, if `len(result) > bulk_change_threshold`, emit single `bulk_update`. Matches SPEC §16.2 pseudocode. | ✓ |
| Also trigger on queue-overflow mid-window | If incoming raw events exceed N×threshold during the debounce window, emit `bulk_update` immediately and reset. | |

**User's choice:** At flush-time only
**Notes:** Simpler invariant; one collapse decision point in `CoalesceEvents`. `max_batch_delay_ms` cap (default 1500ms) bounds worst latency.

---

## postEditHook wiring (kernel → semantic decoupling)

### Wiring shape

| Option | Description | Selected |
|--------|-------------|----------|
| Interface in kernel, semantic registers via setter | `kernel.PostEditNotifier` interface (declared in kernel). Daemon constructs semantic impl, calls `kernel.SetPostEditNotifier(impl)`. Mirrors Phase 59 `SetEnrichFn` / `SetActivateCallback`. | ✓ |
| Third-party event broker package | `internal/eventbus/` owned by neither side. Kernel publishes typed events; semantic subscribes at daemon bootstrap. | |

**User's choice:** Interface in kernel, semantic registers via setter
**Notes:** Matches existing daemon DI patterns; no new infrastructure.

### Hook payload

| Option | Description | Selected |
|--------|-------------|----------|
| Workspace + slice of file paths only | Minimal contract: `OnEdit(ctx, workspaceID, paths []string)`. Semantic builds its own `ChangeHelixEdit` events, reads files, re-extracts. | ✓ |
| Workspace + paths + edit kind | `OnEdit(ctx, workspaceID, paths []string, kind EditKind)`. Lets semantic skip re-extraction for trivial cases. | |
| Full SourceChangeEvent slice | Kernel imports the `SourceChangeEvent` type from a shared types package and constructs events itself. | |

**User's choice:** Paths-only — and reframed the entire architecture.
**Notes:** User's reasoning expanded the scope of this decision. Three independent change paths must coexist:

> "Filesystem state is truth. Helix edit hook is early notification. Watcher is coverage for external edits. Manifest scan is recovery if watcher misses events."

Concrete shapes locked by the user:

```go
type EditNotifier interface {
    OnEdit(ctx context.Context, workspaceID WorkspaceID, paths []string) error
}

type WorkspaceChangeSignal struct {
    WorkspaceID WorkspaceID
    Paths       []string
    Source      ChangeSource // helix_edit, fsnotify, manifest_scan, git
    ObservedAt  time.Time
}
```

Classification rules (semantic owns):

```text
path exists, known before, hash changed     → modified
path exists, unknown before                 → created
path missing, known before                  → deleted
old missing + new exists + content lineage  → renamed (best-effort)
many paths changed                          → bulk_update at coalesce time
```

Config keys to add: `live_updates.watcher_enabled`, `live_updates.manifest_scan_enabled`, `live_updates.manifest_scan_interval` (default 10s).

External LLM/IDE/shell edits flow through the same path as Helix edits — passing richer Helix-edit metadata would create asymmetric optimization paths between Helix-origin and external edits, which is fragile.

This reframing folded most of Area 4 (watcher resilience) into the same architectural answer.

---

## overlay_epoch semantics & storage

### Epoch grain

| Option | Description | Selected |
|--------|-------------|----------|
| Per overlay write transaction | Each `BeginOverlayTx` allocates a new epoch atomically (epoch++ under per-workspace lock). Finest grain = strongest CAS guarantee for Phase 63. | ✓ |
| Per coalesced batch | One epoch per batch flush. Coarser; means Phase 63 either compacts a whole batch or none. | |
| Per individual file update within tx | Each file-level upsert tagged with its own epoch even within a shared tx. | |

**User's choice:** Per write transaction
**Notes:** Aligns with SPEC §16.3 (one tx per `UpdateChangedFile`).

### Epoch storage

| Option | Description | Selected |
|--------|-------------|----------|
| ALTER `semantic_live_overlay_meta` + ADD column on each fact table | Schema migration v2→v3 (Phase 60 owns it). `meta.current_epoch` + `write_epoch` on overlay_files/symbols/references/edges. Phase 63 CAS uses these. | ✓ |
| Sidecar table `semantic_live_overlay_epochs` | New table mapping (repo_id, epoch) → (started_at, committed_at, file_count). | |
| In-memory only, persisted on flush | Counter held in process; persisted to meta only at compaction handoff. | |

**User's choice:** ALTER existing tables
**Notes:** Single source of truth; Phase 63 CAS reads existing structure.

### No-op batch behavior

| Option | Description | Selected |
|--------|-------------|----------|
| No — only advance on actual writes | If `CoalesceEvents` returns empty or all events are no-ops, no transaction opens, no epoch consumed. | ✓ |
| Yes — advance on every flush attempt | Even no-op flushes bump epoch. | |

**User's choice:** Only advance on actual writes
**Notes:** Keeps epoch sequence dense and meaningful; compaction isn't fooled into thinking work happened.

---

## Watcher resilience (ENOSPC + manifest scan)

> Note: Area 2's reframe by the user established that manifest scan is always on (default 10s) and is the correctness recovery path. ENOSPC is just "watcher off, scanner unchanged." This pre-answered the architectural questions; only sub-questions remained.

### ENOSPC surface

| Option | Description | Selected |
|--------|-------------|----------|
| `slog.Warn` + `get_health` degraded flag | Log structured warning at ENOSPC detection. `get_health` (Phase 65) surfaces `watcher_status="unavailable"`, reason, remediation hint. SemanticIndexState stays "ready". | ✓ |
| Promote SemanticIndexState to "stale" flag | Watcher death promotes per-workspace state to "stale" until manual recovery. | |
| MCP notification to active sessions + log | Push JSON-RPC notification to connected MCP clients. | |

**User's choice:** `slog.Warn` + `get_health` degraded flag
**Notes:** Manifest scan keeps the overlay correct, so promoting state to "stale" would lie about correctness. MCP push notifications deferred (no current consumer).

### Manifest source

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse `semantic_files.content_hash` column | Phase 59 already populates `content_hash` on every indexed file. Scanner walks workspace, hashes each file with same `Hash(content)`, compares vs effective `semantic_files.content_hash`. | ✓ |
| Maintain separate watcher manifest in `semantic_watcher_manifest` table | New table indexed by path → last_observed_hash. Decouples watcher state from extraction state. | |

**User's choice:** Reuse `semantic_files.content_hash`
**Notes:** No new state to maintain; single source of truth. If query patterns ever justify a sidecar table, it's an additive optimization.

---

## Claude's Discretion

The following implementation details were not asked of the user (captured in CONTEXT.md `<decisions>` Claude's Discretion subsection):

- fsnotify library reuse (`github.com/fsnotify/fsnotify` already at `internal/memory/watcher.go`).
- Concrete package layout under `internal/semantic/live/{watcher,scanner,...}/`.
- Trace span names per SPEC §28.2 (`semantic.live.coalesce`, `semantic.live.update_file`, etc.).
- Plan layout (suggested 4 plans; planner may collapse or split).
- fsnotify Op-flag → path-list translation inside the watcher.
- JetBrains `___jb_tmp___` rename re-attach detection.
- `LSPQueue.Enqueue` as typed buffered handoff (Phase 61 fills consumer).
- `GraphCache.ApplyRepair` and `MarkAffectedScoresAndClusters` as typed no-op stubs (Phase 62 fills bodies).
- Hash function reuse (xxhash64 from Phase 59).
- Per-workspace mutex granularity for epoch allocation.
- Manifest scan walk concurrency = 4 (matches Phase 59 `extraction.max_parallel_files`).
- `ChangeFileRenamed` detection as best-effort with delete+create fallback.
- Bounded-label outcomes enum: `applied | no_op | error | dropped`.

## Deferred Ideas

Captured in CONTEXT.md `<deferred>`:

- MCP push notifications for watcher state changes (Phase 64+).
- Per-file fan-out / worker-pool dispatch (revisit if benchmarks demand).
- `semantic_watcher_manifest` standalone table (additive optimization).
- `refresh_semantic_graph` MCP tool (Phase 64).
- `get_semantic_graph_status` MCP tool wrapper (Phase 64).
- `get_health` watcher status integration (Phase 65 strangler-fig).
- Cross-workspace event coordination.
- fsnotify replacement (only if quirks force it).
- Detecting renames across content edits.
- Generic "edit kind" subtyping for `ChangeHelixEdit`.
- Adaptive `manifest_scan_interval`.
