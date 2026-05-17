# Phase 72 — P1 Cluster & Impact Tools — CONTEXT

**Date:** 2026-05-17
**Phase:** 72 (p1-cluster-impact-tools)
**Status:** context captured

## Domain

Three new MCP tools answer workspace-level structural questions on top of the v1.10 semantic graph + cluster engine:

- `get_cluster_map` (`read+`) — workspace-level weak-component overview: count, size distribution, top-N clusters with member counts, representative symbols, dominant edge kinds.
- `explain_cluster` (`read+`) — per-cluster detail consumed from `get_cluster_map`: full member list, per-member rank, cohesion/separation scores, dominant entry-point symbols.
- `get_change_impact_graph` (`review+`) — pre-edit blast-radius **subgraph** (nodes + edges + edge kinds), distinct in shape from the existing file-level `AnalyzeBlastRadius`.

Phase 72 is the second wave of the P1 tool set; Phase 71 shipped the three single-symbol read tools (`explain_symbol_deep`, `find_related_symbols`, `validate_graph_edge`) and locked the skill envelope, freshness shape, edge-kind enum, mode-check pattern, and test surface. Phase 72 mirrors that pattern — no new daemon-bootstrap special-casing.

## Canonical Refs

### Phase 72 anchors
- `.planning/ROADMAP.md` — Phase 72 entry (Goal, Success Criteria, Depends on: 62/69/71)
- `.planning/REQUIREMENTS.md` — P1TOOL-03, P1TOOL-04, P1TOOL-05
- `.planning/phases/71-p1-single-symbol-read-tools/71-CONTEXT.md` — **MUST read**; D1/D2/D3/D5/D6/D7 are inherited unchanged
- `.planning/phases/71-p1-single-symbol-read-tools/71-VERIFICATION.md` — verified envelope/mode-check/test patterns to mirror

### Skill envelope + mode-check pattern (mirror Phase 71)
- `internal/skill/semantic/` — Phase 71 envelope.go + mode_check.go + accessors.go pattern
- `internal/skill/semantic/handlers_p1.go` (Phase 71) — handler shape to clone for the three new tools
- `internal/profile/` — `read+` / `review+` mode tier matrix

### Cluster engine (Phase 62 + 63)
- `internal/semantic/cluster/weak.go` — `WeakComponents(nodes, edges) []Cluster`
- `internal/semantic/cluster/persist.go` — `ClusterStore`, `ClusterTx`, `RunClusterDetection(ctx, repoID, projection, ...)`
- `internal/semantic/store/effective_graph.go` — `ClusterSummary{ID int, MemberCount int}`, `ClusterMemberRow{ClusterID, NodeID}`, `UpsertClusters`, `UpsertClusterMembers` (scoped by `(projection, graph_version)`)
- `internal/skill/semantic/accessors.go` — Phase 69 `ClusterStatus` accessor; Phase 72 must reuse, not duplicate

### Blast-radius existing surface (do NOT duplicate)
- `internal/kernel/symbols/blast.go` — `AnalyzeBlastRadius(ctx, lease, uri, line, col) (*BlastRadius, error)` returns **file-level summary** with critical-path filtering
- `internal/kernel/symbols/blast_radius_strangler.go` — `analyzeBlastRadiusViaLookup`, `capConfidences`, `formatBlastRadiusEnvelope` — Phase 72's `get_change_impact_graph` reuses `integ.SemanticLookup` and the cap helper but emits a **subgraph**, not a file rollup

### Status / freshness
- Phase 69 `RetrievalStatus` + `ClusterStatus` envelopes — drive freshness on all three tools
- `FreshnessV2` envelope from Phase 71 D5 — `{graph_version, snapshot_id, extractor_run_id}` carried verbatim

### Edge-kind surface
- Phase 71 D3 — closed MCP-surface enum + `internal_kind` field for `get_change_impact_graph` edges (same surface as `validate_graph_edge`)

## Decisions

### D1 — `cluster_id` is an opaque composite token bound to `(projection, graph_version)`

`get_cluster_map` emits `cluster_id` as the opaque string `"{projection}:{graph_version}:{id}"` (e.g., `"weak_components:42:7"`). `explain_cluster` decodes the token and **refuses with a structured `stale_cluster_id` error** (pointing back to `get_cluster_map`) if `graph_version` is no longer current for the workspace.

**Why:** `ClusterSummary.ID` is an integer scoped by `(projection, graph_version)` in `internal/semantic/store/effective_graph.go`. A bare int captured before a graph rebuild silently points at a different cluster after rebuild — a correctness bug masquerading as a stable handle. The composite token makes the binding visible and enforceable at the handler boundary.

**Implementation note:** the freshness envelope already carries `graph_version`; the stale-id check is a single equality comparison at handler entry. Error reason is a closed enum value alongside Phase 71's existing reasons.

### D2 — Ranking by member count; cohesion by conductance; PageRank for in-cluster rank

- `get_cluster_map` top-N: sort by `MemberCount` desc (already materialized in `ClusterSummary`; cheap; intuitive for agents).
- `explain_cluster` per-member rank: PageRank (the existing graph engine score from Phase 62).
- Cohesion: intra-edge density (intra_edges / max_possible_intra_edges).
- Separation: conductance (edges_leaving_cluster / (2 × intra_edges + edges_leaving)).
- Representative symbols per cluster: top-3 by PageRank within cluster.
- Dominant edge kinds per cluster: top-3 by occurrence count among intra-cluster edges (surfaced via the Phase 71 D3 MCP edge-kind enum).
- Dominant entry-point symbols (`explain_cluster`): members with `is_exported = true` or otherwise flagged as entry points by `find_entry_points` semantics, sorted by PageRank.

**Why:** member-count ranking is what agents reach for first when scanning a workspace; conductance is the standard, well-defined community-detection separation metric and is computable from `intra_edges` + `edges_leaving` already available from the cluster store. PageRank reuses Phase 62 rather than introducing a new score.

### D3 — `get_change_impact_graph` input + output shape

**Input (closed schema):**
- Seed identity: `symbol_id` OR `(file_path, symbol_name)` tuple (same shape as Phase 71 D1).
- `max_depth` (int, default 2, max 5).
- `edge_kinds` (optional `[]string` filter using the Phase 71 D3 MCP edge-kind enum).

**Output:**
```
{
  "nodes": [
    {"symbol_id": "...", "qualified_name": "...", "package": "...", "pagerank": 0.123}
  ],
  "edges": [
    {"from": "<symbol_id>", "to": "<symbol_id>", "edge_kind": "<mcp_enum>", "internal_kind": "<internal>", "confidence": 0.92}
  ],
  "truncated": false,
  "reached_depth": 2,
  "nodes_count": 47,
  "edges_count": 113
}
```

Plus the standard envelope (`FreshnessV2`, source, mode_tier, `confidence_cap` per D4).

**Differentiation from `AnalyzeBlastRadius`:** `AnalyzeBlastRadius` (`internal/kernel/symbols/blast.go`) returns a file-level rollup with critical-path filtering, intended for review/risk summaries. `get_change_impact_graph` returns a pure subgraph (nodes + edges + edge kinds) intended for agents that want to **traverse** or visualize impact, not a rolled-up risk score. No `would_break` flags, no critical-path filtering, no file aggregation — that stays in the existing tool.

**Why:** keeps the new tool a clean composable primitive; avoids duplicating `AnalyzeBlastRadius` semantics; defers hypothetical-edit metadata (signature change, rename, delete) to a follow-up phase (see Deferred Ideas) so the Phase 72 contract is small and verifiable.

### D4 — Size budgets + confidence cap surfacing (mirror Phase 71 D2)

**Per-section caps with `_count` totals (no pagination — same as Phase 71 D2):**

| Tool | Field | Default | Max |
|---|---|---|---|
| `get_cluster_map` | `top_n` clusters | 20 | 100 |
| `get_cluster_map` | members preview per cluster | 5 | (fixed) |
| `explain_cluster` | members | 200 | 1000 |
| `get_change_impact_graph` | nodes | 200 | (hard cap) |
| `get_change_impact_graph` | edges | 500 | (hard cap) |
| `get_change_impact_graph` | depth | 2 | 5 |

Every truncated section reports a `<section>_count` total so agents see what they're missing.

**Confidence cap (success criterion #5):** when results fall back to a degraded path (e.g., type-resolver tier 3), apply cap = 0.6 in **both** places:
- Per-edge `confidence` field is clamped (reuse `internal/kernel/symbols/blast_radius_strangler.go:capConfidences` helper).
- Envelope carries `confidence_cap: {value: 0.6, reason: "type_resolver_tier_3"}` so the systemic degradation signal is visible even when an agent only reads envelope-level metadata.

`reason` is a closed enum (extends Phase 71's existing reason set; new value added in Phase 72: `type_resolver_tier_3`).

### D5 — Inherited from Phase 71 (unchanged)

The following Phase 71 decisions apply verbatim to Phase 72 and must NOT be re-litigated:

- **D1 (Phase 71)** — Seed addressing: `symbol_id` OR `(file_path, symbol_name)` tuple. Used by `get_change_impact_graph`.
- **D3 (Phase 71)** — Edge-kind surface: closed MCP enum + `internal_kind` field. Used by `get_cluster_map` (dominant edge kinds) and `get_change_impact_graph` (edge entries).
- **D5 (Phase 71)** — Freshness envelope: `{graph_version, snapshot_id, extractor_run_id}`. Required on all three tool responses.
- **D6 (Phase 71)** — Read-only invariant + mode tier enforcement at handler entry. `get_cluster_map`/`explain_cluster` enforce `read+`; `get_change_impact_graph` enforces `review+`. Snapshot-write canary applies.
- **D7 (Phase 71)** — Test surface: per-handler unit + cross-tool integration + in-tree static read-only gate; `-race -count=1` clean.

## Implementation Notes (for researcher / planner)

- **Reuse, don't fork.** All three tools live in `internal/skill/semantic/` alongside Phase 71 handlers. No new daemon-bootstrap wiring (P1TOOL-08 invariant).
- **Cluster data reads** go through the existing `ClusterStore` / `RunClusterDetection` API — no direct SQL.
- **Cohesion/conductance** computation: prefer doing it at cluster-detection time (`internal/semantic/cluster/`) and persisting alongside `ClusterSummary`, then reading. If that's too invasive for Phase 72, compute on demand in the handler from the persisted members + edges, but document the cost.
- **`get_change_impact_graph`** uses `integ.SemanticLookup` (Phase 65 seam, also used by `analyzeBlastRadiusViaLookup`) for symbol resolution + edge traversal. Confidence cap helper already exists at `internal/kernel/symbols/blast_radius_strangler.go:capConfidences` — extract or reuse, do not copy.
- **Stale `cluster_id`** returns a structured error with `next_action: "call get_cluster_map"` — agents must be able to recover programmatically.
- **Cross-tool integration test** (Phase 71 D7 pattern): `get_cluster_map` → pick top cluster → `explain_cluster` → seed `get_change_impact_graph` from a representative symbol → assert envelope agreement (same `graph_version`).
- **Type-resolver tier detection** — the path that signals "tier 3" lives in the type resolver from Phase 62; planner must identify the exact predicate and wire `confidence_cap` from it.

## Deferred Ideas

- **`get_change_impact_graph` hypothetical-edit metadata** (`edit_kind: signature_change | rename | delete`). Useful but expands contract + test surface. Backlog for a future tool extension phase.
- **Cluster naming/labelling** beyond "representative symbols" (e.g., LLM-derived cluster summaries). Out of scope; that's a separate tool family.
- **`get_change_impact_graph` per-node `would_break` flag**. Duplicates `AnalyzeBlastRadius` semantics; not adding.
- **Cluster ID stability across graph rebuilds** (e.g., heuristic re-matching by member overlap). Could later let agents cache cluster identities across snapshots. Out of scope — D1's stale-id error gives agents an explicit refresh path instead.
- **Modularity / PageRank-sum ranking variants**. Member-count + conductance is enough for v1; revisit if agents prove otherwise.

## Spec Lock

No SPEC.md for Phase 72 (success criteria live in ROADMAP.md). Locked constraints from ROADMAP.md Success Criteria 1–5:

1. `get_cluster_map` returns workspace-level weak-component overview with the listed fields. — addressed by D2.
2. `explain_cluster` returns full member list, per-member ranking, cohesion/separation, dominant entry-points. — addressed by D1 + D2.
3. `get_change_impact_graph` returns a graph subgraph (not file-level summary). — addressed by D3.
4. Mode tier enforcement (`review+` / `read+`) + profile filtering. — inherited D5 (Phase 71 D6).
5. Confidence cap on degraded path + freshness envelope on all responses. — addressed by D4 + inherited D5 (Phase 71 D5).
