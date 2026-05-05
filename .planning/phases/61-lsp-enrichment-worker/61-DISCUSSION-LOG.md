# Phase 61: LSP Enrichment Worker - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-05
**Phase:** 61-lsp-enrichment-worker
**Areas discussed:** Queue topology + concurrency, Foreground preemption mechanism, Bulk-update / git-checkout backoff, Enrichment scope for v1

---

## Queue topology + concurrency

### Q1: Phase 61 priority-queue shape — how many priority lanes ship in v1?

| Option | Description | Selected |
|--------|-------------|----------|
| 2 lanes (high / background) | Producer sets one bool: helix_edit / agent-requested = high, everything else = background. Strict priority. Closed enum, additive. | ✓ |
| 3 lanes (interactive_boost / high / background) | Reserves a top lane for files an active foreground tool just touched. Producer for interactive_boost doesn't exist yet. | |
| Single channel + sortable heap | Most flexible, handles full §14.3 ladder later. Heavier code; over-engineering for v1. | |
| Keep single FIFO, no priority | Drop ENRICH-02. Would need roadmap amendment. | |

**User's choice:** 2 lanes (high / background) (Recommended).
**Notes:** Producer maps `helix_edit` + agent-requested → high; watcher / manifest-scan → background. Phase 62 grows lanes when it needs pagerank-bias / public-symbols-first.

### Q2: Concurrency cap — how many enrichment requests run in flight at once?

| Option | Description | Selected |
|--------|-------------|----------|
| 1 global worker | Matches ENRICH-02 default. Cheapest path to "never bury foreground tool calls". | |
| 1 worker per language | Lets gopls + jdtls progress independently. Risks PITFALLS C8 multi-LS contention. | |
| Configurable, default 1 | Ship 1 by default, expose `max_concurrent_workers` for ops to dial up after benchmarks. | ✓ |

**User's choice:** Configurable, default 1, max from config.
**Notes:** New config key `semantic_index.lsp_enrichment.max_concurrent_workers` (default 1). One worker goroutine per cap slot, all draining the shared 2-lane queue. Cap is global across languages — no per-language fan-out in v1.

---

## Foreground preemption mechanism

### Q1: How does interactive MCP traffic preempt in-flight LSP enrichment?

| Option | Description | Selected |
|--------|-------------|----------|
| Tight per-call deadlines + voluntary yield | Worker checks 'foreground-busy' between cascade steps; aborts and marks `partial:"preempted"`. No invasive lspool changes. | ✓ |
| Hard ctx-cancel via lspool callback | Pool cancels in-flight enrichment ctx on contention. Touches RPC layer Phase 56 just stabilized. | |
| No active preemption — share-until-dirty + cap=1 only | No preemption code. Risk: 5s enrichment blocks foreground for up to 5s. | |

**User's choice:** Tight per-call deadlines + voluntary yield (Recommended).
**Notes:** Yield check happens between LSP calls in the §14.4 cascade. On preempt, file marked `partial:true, partial_reason:"preempted"` so next edit / Phase 64 manual refresh re-triggers it.

### Q2: What does the foreground-busy signal look like in the LeaseAcquirer interface?

| Option | Description | Selected |
|--------|-------------|----------|
| `ForegroundBusy(wsKey) bool` method | Two-method LeaseAcquirer interface (also has AcquireLease). Pool tracks "non-enrichment AcquireLease in last yield_check_window_ms". Simple, no goroutines. | ✓ |
| Subscription channel | `OnForegroundActivity(wsKey) <-chan struct{}`. Lower polling overhead but adds goroutine lifecycle. | |
| Reuse lspool queue-depth metric | Worker reads helix_lspool_queue_depth via metrics registry. Couples enrichment to Prometheus collector — brittle. | |

**User's choice:** `ForegroundBusy(wsKey) bool` method (Recommended).
**Notes:** Window is configurable via new key `semantic_index.lsp_enrichment.yield_check_window_ms` (default 200). Pool tracks `lastForegroundLease[wsKey]` under existing pool mutex; "non-enrichment" identified by sessionID NOT starting with `lsp-enrichment:` prefix.

---

## Bulk-update / git-checkout backoff

### Q1: When the coalescer flushes a bulk_update event, what does the LSP enrichment path do?

| Option | Description | Selected |
|--------|-------------|----------|
| Suppress at producer: handler skips Enqueue on bulk_update | Handler marks files `semantic_pending` with `partial_reason='bulk_update_pending'`; no enqueue. Lazy-on-tool-call drives recovery. Matches PITFALLS C8 verbatim. | ✓ |
| Suppress at consumer: worker drops jobs while bulk window active | Queue still flooded but worker drains fast. Wastes work; harder to test deterministically. | |
| Hybrid: synthetic 'workspace_bulk' job | Handler emits one sentinel job; worker scans pending in priority order. More moving parts; defers priority decision to Phase 62. | |

**User's choice:** Suppress at producer: handler skips Enqueue on bulk_update (Recommended).
**Notes:** New `tx.MarkFileSemanticPending(repoID, path, reason)` API on overlay tx. `partial_reason="bulk_update_pending"` added to closed enum (no schema change — column is `TEXT`). Single counter bump per bulk_update event (not per-file).

### Q2: After a bulk_update marks files semantic_pending, what re-triggers enrichment for them?

| Option | Description | Selected |
|--------|-------------|----------|
| Next per-file edit + manual refresh tool only | Phase 61 ships zero auto-recovery. Next helix_edit / fsnotify event re-enqueues that file; Phase 64 `refresh_semantic_graph(paths=...)` drains on demand. | ✓ |
| Lazy on tool call — read tools enqueue pending files | Reads heal pending state. Adds coupling from semantic-aware read tools — wiring point doesn't fully exist in Phase 61. | |
| Background drain at low priority | Worker picks pending in PageRank order when high lane empty + foreground-busy false. Re-introduces the flood C8 warns against. | |

**User's choice:** Next per-file edit + manual refresh tool only (Recommended).
**Notes:** Trade-off explicitly accepted: untouched files in a checked-out branch stay pending indefinitely until queried. Phase 64+ read tools that surface pending state will make this visible to the user.

---

## Enrichment scope for v1

### Q1: Which LSP calls does the Phase 61 cascade actually issue and persist?

| Option | Description | Selected |
|--------|-------------|----------|
| Full SPEC §14.4 cascade | All 6 capabilities: documentSymbols, hover, callHierarchy (depth=2), typeHierarchy (depth=2), implementations, find-refs + go-to-def on refs. Phase 62 consumes immediately. | ✓ |
| Symbols + diagnostics only — defer all edges | documentSymbols + hover + diagnostics. Phase 62 extends cascade. ENRICH-04 budget exhaustion gets exercised less. | |
| Symbols + diagnostics + RESOLVES_TO only | Adds go-to-definition on refs. Skips call/type hierarchy + implementations. Middle ground. | |

**User's choice:** Full SPEC §14.4 cascade (Recommended).
**Notes:** Edges persist with `validation_state="validated"`, `confidence=1.0`, `source="lsp.<call>"`. Cascade order is fixed in v1; per-language `MethodNotFound` results in slog.Debug + continue (no per-language opt-out config knob).

### Q2: Which languages does the Phase 61 enrichment worker run against?

| Option | Description | Selected |
|--------|-------------|----------|
| All languages with LS readiness gate satisfied | Honors WaitUntilJavaReady (jdtls), experimental/serverStatus.quiescent (rust-analyzer), AcquireLease success otherwise. Surface bounded by Phase 59 extraction tier × LS readiness × LS installed. | ✓ |
| Limit to Phase 59 first-class only (Go, TS+JS, Python) | Hardcoded allowlist. Contradicts ENRICH-03 (cites both readiness gates as in-scope). Pushes Java + Rust users to Phase 62+. | |
| Configurable allowlist, default first-class | Compromise; adds config knob with no clear consumer. | |

**User's choice:** All languages with LS readiness gate satisfied (Recommended).
**Notes:** On readiness-gate timeout (jdtls cold start exceeds `timeout_per_file`), file marked `semantic_pending` with `partial_reason="lsp_not_ready"` and re-queued via standard re-trigger paths. Same recovery as bulk_update. Per-file error log suppressed; one summary log per worker startup.

---

## Claude's Discretion

- **Package layout:** `internal/semantic/lspenrich/` (matches SPEC §3 line 344). 2-lane queue extension may live as `internal/semantic/live/lspqueue/` (extending existing) or `internal/semantic/lspenrich/queue.go` — planner picks.
- **Session-ID scheme:** `"lsp-enrichment:<workspace-key>:<language>"` per long-lived clean lease. One lease per (workspace, language) pair, held for the worker's lifetime, released on workspace deactivation.
- **Per-LSP-call failure handling:** `MethodNotFound` → `slog.Debug` once per (language, method) and continue. Generic JSON-RPC error → `slog.Warn`, commit partial, continue. Whole-job failure → mark `semantic_pending`, `partial_reason="lsp_unavailable"`, no re-enqueue.
- **Metrics:** `helix_semantic_lsp_enrichment_total{language, outcome}`, `..._duration_seconds{language}`, `..._errors_total{language, outcome}`, `..._lane_depth{lane}`, `..._bulk_suppressed_total`. All closed-enum bounded labels.
- **Trace spans:** `semantic.lsp_enrich_file` (root) + child spans per cascade step.
- **Worker `Status()` accessor (D-09):** read-only struct for Phase 65 `get_health` integration.
- **Stress fixture:** small Go module + Java Maven project under `internal/semantic/lspenrich/testdata/stress/`.
- **Plan layout:** suggested 4 plans (interface + producer rewiring; worker engine + cascade + budget; metrics + bootstrap + Status; ENRICH-05 stress + integration tests + REQ check-off). Planner may collapse or split.

## Deferred Ideas

- Per-language fan-out (revisit if benchmarks demand)
- PageRank-bias prioritization (Phase 62)
- Background drainer for `bulk_update_pending` files (explicitly out — PITFALLS C8)
- `refresh_semantic_graph` MCP tool (Phase 64)
- `get_semantic_graph_status` MCP tool (Phase 64)
- `get_health` enrichment integration (Phase 65)
- Hard ctx-cancel preemption (revisit if voluntary yield insufficient)
- Subscription-channel foreground-busy signal (alternative to bool method)
- Single sortable heap queue (revisit if priority surface grows past 4 lanes)
- Receipts / guardrails on enrichment-derived edges (Phase 66)
- Adaptive priority promotion for `semantic_pending` files (Phase 62+)
- Cross-workspace enrichment coordination (out of scope; workspaces are independent)
