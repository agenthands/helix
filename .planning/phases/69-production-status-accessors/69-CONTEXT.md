# Phase 69 — Production Status Accessors — CONTEXT

**Date:** 2026-05-14
**Goal (from ROADMAP):** `get_semantic_graph_status` returns real cluster and retrieval status from production engines instead of `{state:"unknown"}` placeholders.
**Requirements:** STATUS-01, STATUS-02, STATUS-03
**Depends on:** Phase 62 (cluster engine), Phase 64 (status tool wiring), Phase 65 (strangler-fig integration)

## Domain

Close the two Phase 64 W1 placeholders in `internal/daemon/semantic_wiring.go`:
- `semSchedulerAdapter.ClusterStatus` (line ~456) — currently returns `{State:"unknown", Reason:"phase-62-clustering-no-status-accessor"}`.
- Retrieval status surface (lines ~441,450 anchor comments) — currently absent from `StatusResult` beyond the `RetrievalPending` bool.

Wire both blocks to real production engines (cluster persistence + bleve), extending the SPEC §23.3 envelope minimally, with race-clean read-path tests and an E2E that proves non-placeholder values.

## Canonical Refs

- `.planning/ROADMAP.md` — Phase 69 success criteria (5 items)
- `.planning/REQUIREMENTS.md` — STATUS-01, STATUS-02, STATUS-03
- `internal/daemon/semantic_wiring.go:407-462` — current placeholder for `ClusterStatus`; companion comment block for retrieval-status absence at 441,450
- `internal/skill/semantic/envelope.go:62-112` — `ClusterStatus`, `StatusResult` (SPEC §23.3) shapes to extend
- `internal/semantic/cluster/persist.go` — `ClusterStore`, `RunClusterDetection`, `toSummaries` → `store.ClusterSummary` rows
- `internal/semantic/retrieval/bleve.go` — `Engine.GetMeta` / `SetMeta`, `Engine.QueryBleve`; bleve doc-count access for indexed_symbols
- `internal/semantic/retrieval/corpus.go` — `MapSymbolToDoc`; one bleve doc per symbol
- `.planning/phases/64-new-mcp-tools/64-07-PLAN.md` + 64-07-SUMMARY.md — Phase 64 P07 15-symbol E2E fixture (TestE2E_IndexThenContext_SymbolCount) to extend
- `.planning/phases/64-new-mcp-tools/64-06-PLAN.md` — Phase 64 P06 status-tool wiring (handler that consumes ClusterStatus + RetrievalStatus)
- `.planning/codebase/CONVENTIONS.md` — D-09 (no Begin/Commit/Abort/Write on read path) and closed-enum envelope invariants

## Decisions

### D1 — Cluster status data source: persisted rows via new *Store accessor

Add `*Store.ClusterStatusForGraphVersion(ctx context.Context, repoID string, graphVersion uint64) (ClusterStatusRow, error)` (or equivalent struct), reading committed `cluster_summary` rows produced by `cluster.RunClusterDetection`. Lock-free read mirroring other Phase 62 effective-graph queries.

State derivation:
- rows exist for current `graph_version` → `"current"`
- rows exist for an older `graph_version` → `"stale"` (Reason names the lag)
- detection in flight (signal TBD by researcher — likely a cluster-scheduler hint or absence of latest-version rows + a fresh write timestamp) → `"building"`
- no rows at all → `"unknown"`

**Why:** Honors D-09 (read-only access through *Store), survives daemon restart, single source-of-truth for downstream consumers. Hybrid options were rejected as more moving parts than the data warrants.

### D2 — Retrieval status envelope: nested `retrieval_status` struct mirroring cluster_status

Extend `StatusResult` (SPEC §23.3) by adding:

```go
type RetrievalStatus struct {
    CorpusVersion  uint64 `json:"corpus_version"`
    IndexedFiles   int64  `json:"indexed_files"`
    IndexedSymbols int64  `json:"indexed_symbols"`
    LastCompactAt  int64  `json:"last_compact_at"` // unix ms
    Reason         string `json:"reason,omitempty"`
}
```

Embedded in `StatusResult` as `RetrievalStatus RetrievalStatus` (new field). Existing `RetrievalPending bool` stays at the top level — it is a freshness gate, not retrieval-corpus state, and Phase 64 consumers depend on it.

**Why:** Symmetry with `ClusterStatus`; future additive fields land cleanly without touching unrelated top-level fields; no breaking change to Phase 64 envelope.

### D3 — Counter + version provenance: bleve is source-of-truth

| Field | Source | Writer |
|---|---|---|
| `indexed_files` | `bleve.Index.DocCount()` aggregated to file granularity (count distinct file_path field in bleve mapping), OR a `bleve_files` meta key updated per batch — researcher picks the cheaper path | IndexRunner (after upsert batch) |
| `indexed_symbols` | `bleve.Index.DocCount()` directly (one doc per symbol per `MapSymbolToDoc`) | IndexRunner |
| `corpus_version` | bleve meta key `"corpus_version"` via `Engine.SetMeta`/`GetMeta` | IndexRunner writes after each successful upsert with the current graph_version |
| `last_compact_at` | bleve meta key `"last_compact_at"` (unix ms) | Compactor writes after a successful compaction |

All single-writer. Read path is pure `GetMeta` + `DocCount` — no migration, no new store columns.

**Why:** bleve owns the corpus, so the corpus-state metadata lives next to the data. Avoids new SQLite columns + migration. Obs-metric option was rejected because obs is intentionally lossy across restarts.

### D4 — ClusterStatus additive fields + E2E fixture extension

**Struct extension** — additive only, preserves existing JSON keys:

```go
type ClusterStatus struct {
    State       string `json:"state"`                 // unchanged
    Reason      string `json:"reason,omitempty"`      // unchanged
    ComputedAt  int64  `json:"computed_at,omitempty"` // NEW — unix ms from cluster_summary row
    MemberCount int    `json:"member_count,omitempty"`// NEW — total members across all clusters for this (repo, graph_version)
}
```

**E2E fixture** — extend Phase 64 P07 15-symbol bleve fixture: populate the same workspace's `ClusterStore` with 2–3 `cluster_summary` rows + matching `cluster_members` rows at fixture-build time, then assert non-placeholder values across both `cluster_status` and `retrieval_status` in a single integration test.

**Why:** Additive fields preserve closed-enum stability and Phase 64 consumer compatibility. Re-using the P07 fixture avoids parallel fixture maintenance and keeps the "populated workspace" definition consistent.

## Implementation Notes (for researcher / planner)

- Three placeholder comments at `semantic_wiring.go:408, 441, 450` MUST be removed once the real accessors land (success criterion #5).
- D-09 invariant is enforced via the existing `vet-nokernel2semantic` + read-path grep audit pattern from Phase 64. The new `ClusterStatusForGraphVersion` accessor MUST land with race-clean read-path tests (success criterion #3).
- Cluster "building" state needs a concrete signal. Two candidates for research to nail down:
  1. Cluster scheduler exposes an "in-flight" boolean (preferred — same pattern as `IsQuiescent`).
  2. Absence of latest-graph-version rows combined with a recent prior write (heuristic, slower).
- `RetrievalStatus.Reason` is populated when state is degraded (e.g., bleve meta absent → reason `"corpus_version-uninitialized"`). Closed-enum reason values: TBD by planner.
- Phase 64 P06 status handler (`tools_status.go`) already composes the response — only the adapter outputs need to change.
- The compactor must learn to write `bleve.SetMeta("last_compact_at", ...)`. Plan should locate the existing compactor success path and add a single write there.

## Deferred Ideas

- Per-projection cluster status (instead of one global block) — would help when multiple ranking projections exist; out of scope for STATUS-01.
- Historical cluster status (last-N versions) for observability dashboards — separate phase.
- Streaming retrieval-status delta over MCP for long-running indexers — UX add-on, not in scope.

## Spec Lock

No SPEC.md present for Phase 69. Requirements come from REQUIREMENTS.md (STATUS-01/02/03) and ROADMAP success criteria.
