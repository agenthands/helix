# Phase 63: Compaction & Retention - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-07
**Phase:** 63-compaction-retention
**Areas discussed:** Crash-recovery mechanism, Snapshot-write API scope, Compaction trigger + RankScheduler coordination, VACUUM cadence implementation

---

## Crash-recovery mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Single transaction | One DuckDB tx wraps snapshot write + ClearOverlay + retention. Simplest invariant; relies on DuckDB ACID. Acceptable for dev-workspace scale (snapshot_retention=5, max_overlay_files=1000, idle-debounce gates large overlays). Add a hard size pre-check that bails to 'partial' status if the overlay row count exceeds a guard threshold. | ✓ |
| compaction_journal sidecar | Add semantic_compaction_journal table tracking (workspace_id, snapshot_id, captured_epoch, phase, started_at). Each compaction phase commits independently. Daemon startup scans for in-progress journal rows and finishes-or-rolls-back. Buys unbounded compaction size at cost of more state-machine code. | |
| Hybrid: single-tx with journal fallback | Default path is single-tx. If pre-check shows tx working set would exceed a guard threshold (configurable, default e.g. 10k overlay rows), fall back to journaled multi-tx mode. Captures the simple-case win but keeps escape hatch for big overlays. More code than (1), less than full (2). | |

**User's choice:** Single transaction (Recommended)
**Notes:** REQUIREMENTS COMPACT-05 explicitly leaves "single tx OR journal" open. The single-tx + size-guard path matches the actual workload (developer workspaces with snapshot_retention=5 and max_overlay_files=1000) and avoids the journal state-machine complexity. Pre-flight overlay-size guard emits `outcome=partial` if the projected working set would blow past DuckDB's `memory_limit: 1GiB`. Threshold value deferred to planner (suggestion: max_overlay_files * 4).

---

## Snapshot-write API scope

| Option | Description | Selected |
|--------|-------------|----------|
| Two plans inside Phase 63 | P63-01 ships the snapshot-write API in internal/semantic/store/snapshot.go with full unit tests against a synthetic 'fake compactor' fixture (matches SPEC §22 shape). P63-02 ships the actual compactor on top. Snapshot API is reviewable as a clean foundation; compactor PR is purely about compaction logic. | ✓ |
| Single Phase 63 plan, API + compactor together | One large plan adds snapshot writers and the compactor in lockstep. Tighter coupling between API design and first real consumer. Less PR ceremony but a bigger blast radius per commit. | |
| Insert a new phase (e.g. 62.1) before 63 | Snapshot-write API ships as its own short phase before Phase 63 begins. Maximum separation; matches the precedent of 59.1 inserted between 59 and 60. Adds a phase to the v1.10 milestone count. | |

**User's choice:** Two plans inside Phase 63 (Recommended)
**Notes:** Discovery during scout: `internal/semantic/store/snapshot.go` is currently empty (just a doc-comment placeholder); Phase 59 deferred BeginSnapshot/WriteSnapshotFacts/CommitSnapshot/AbortSnapshot to "P59 implementation" but they never landed. Compaction can't ship without them, so Phase 63 owns the API. Two-plan split optimizes for review surface — P63-01 lands a clean snapshot-write API; P63-02 lands compaction logic on top.

---

## Compaction trigger + RankScheduler coordination

### Question 1 — Where does the compaction trigger goroutine live?

| Option | Description | Selected |
|--------|-------------|----------|
| Per-workspace dedicated goroutine | Spawned on kernel.ActivateWorkspace, joined on shutdown. Owns time.AfterFunc(compact_after_idle_ms) reset on every coalescer flush. Mirrors Phase 60 ownership pattern (coalescer, watcher, manifest scanner). | ✓ |
| Single global goroutine, round-robin workspaces | One goroutine ticks periodically and checks gates per active workspace. Simpler bootstrap; breaks the per-workspace ownership pattern Phase 60 established. | |
| Piggyback on coalescer post-flush | The Phase 60 coalescer goroutine schedules the compactor via a longer timer after each flush. Tightest coupling; turns coalescer into a multi-purpose object. | |

**User's choice:** Per-workspace dedicated goroutine (Recommended)
**Notes:** Mirrors Phase 60 D-02 / D-05 ownership exactly — coalescer, watcher, manifest scanner are all per-workspace. Spawned on kernel.ActivateWorkspace, joined on deactivation/shutdown. Coalescer flush calls compactor.OnFlush() which resets the AfterFunc timer.

### Question 2 — How does the compactor read the six SPEC §22.1 gates?

| Option | Description | Selected |
|--------|-------------|----------|
| Single aggregate CompactionGate.IsReady accessor | internal/semantic/compact/gate.go composes small read-only accessors registered by each component (coalescer LastFlushAt, overlay OpenTxCount, LSPQueue.Depth, RankScheduler.IsQuiescent). Returns (ready bool, blockedBy string) for metric/log emission. Each component ships its accessor; the gate composes. | ✓ |
| Compactor inlines each component check | No aggregator; the compactor imports coalescer, overlay store, lspqueue, scheduler packages directly and ANDs the conditions inline. Less indirection; harder to test gate composition in isolation. | |

**User's choice:** Single aggregate CompactionGate.IsReady accessor (Recommended)
**Notes:** Each component (coalescer, overlay store, LSPQueue, RankScheduler, kernel) ships a small read-only accessor; the gate composes them with deterministic ordering. BlockedReason is a closed enum (overlay_empty, idle_too_short, edit_tx_active, overlay_tx_active, lsp_pending, rank_repairing) so the metric label cardinality stays bounded. Phase 63 adds new accessors on Phase 60/61/62 components as small additive changes.

---

## VACUUM cadence implementation

### Question 1 — How should VACUUM be scheduled?

| Option | Description | Selected |
|--------|-------------|----------|
| Piggyback on per-workspace compactor | After each successful compaction commit the same goroutine checks now() - last_vacuum_at >= vacuum_interval (default 168h). If yes AND CompactionGate still holds, VACUUM fires in a separate tx. Reuses the quiescence gate; last_vacuum_at lives in semantic_meta. Smallest amount of new code. | ✓ |
| Dedicated daemon-level ticker | Separate hourly time.Ticker iterates workspaces and fires VACUUM when due + quiescent. Decouples maintenance from compaction at the cost of a new long-lived goroutine and duplicated quiescence-check plumbing. | |
| Startup-only check | Daemon start checks each workspace; schedules VACUUM if due. Simple; skips weekly cadence entirely on long-lived daemons (production helix daemons may run days/weeks without restart). | |

**User's choice:** Piggyback on per-workspace compactor (Recommended)
**Notes:** Reuses the per-workspace ownership and the CompactionGate.IsReady aggregator already established in Question 1 of Area 3. VACUUM runs in its own tx (DuckDB blocks reads/writes during VACUUM; nesting it inside the compaction tx would double the lock window). last_vacuum_at lives in semantic_meta keyed by (repo_id, key='last_vacuum_at') with RFC3339 string value.

### Question 2 — Default state for VACUUM?

| Option | Description | Selected |
|--------|-------------|----------|
| Default off, opt-in | semantic_index.maintenance.vacuum_enabled: false default. Matches the 'config-gated' phrasing in REQUIREMENTS COMPACT-03; benches/long-running production deployments opt in. Avoids surprise multi-second VACUUM on fresh installs in week 2. | ✓ |
| Default on, opt-out | Maintenance is good hygiene; users with constrained environments opt out. Trades a hidden week-2 surprise for fewer 'why is my .duckdb 8GB' bug reports. | |

**User's choice:** Default off, opt-in (Recommended)
**Notes:** Matches the literal "config-gated weekly VACUUM" phrasing in REQUIREMENTS COMPACT-03. Production deployments and the long-repo bench fixture opt in explicitly. New config keys: semantic_index.maintenance.vacuum_enabled (default false), semantic_index.maintenance.vacuum_interval (default "168h").

---

## Claude's Discretion

The user did not redirect any specific items to Claude's discretion during this discussion. Items naturally falling under Claude's discretion (planner-level implementation details) are itemized in CONTEXT.md `<decisions>` "Claude's Discretion" subsection:

- Concrete package layout (`internal/semantic/compact/` for compactor + gate)
- Pre-flight overlay row-count threshold value (suggestion: max_overlay_files * 4)
- Exact `BlockedReason` enum names (closed-enum invariant preserved)
- `semantic_meta` schema shape for `last_vacuum_at` (prefer generic `(repo_id, key, value)`)
- Bench fixture specifics (synthetic 100-file workspace, 1000 cycles; specific size bound deferred to bench result)
- Whether new pre-existing component accessors need a vet analyzer

## Deferred Ideas

- **compaction_journal sidecar** — alternative crash-recovery mechanism. Revisit if benchmarks show the size guard fires routinely.
- **Adaptive compact_after_idle_ms** — currently fixed 5000ms default.
- **Per-snapshot retention policies** — currently uniform last N=5; future phase could add daily/weekly/monthly tiers.
- **MCP tool wrappers** (`get_semantic_graph_status`, `refresh_semantic_graph`) — Phase 64.
- **`get_health` compaction-status integration** — Phase 65 strangler-fig.
- **MCP push notifications for compaction state** — Phase 64+ if any client subscribes.
- **Multi-workspace VACUUM serialization** — defer until contention is observed.
- **Global compaction lock across workspaces** — defer until DuckDB contention surfaces.
- **Snapshot diff / time-travel queries** — `BaseSnapshotID` is preserved on every committed snapshot; queries are out of scope.
- **Online compaction without idle window** — defer; bulk_update collapse and manifest_scan_interval bound the pathological case.
- **Cross-workspace shared snapshots** — out of scope.
