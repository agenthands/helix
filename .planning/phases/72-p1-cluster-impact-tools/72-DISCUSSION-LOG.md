# Phase 72 — Discussion Log

**Date:** 2026-05-17
**Mode:** discuss (default, batched single-pass — user selected all four gray areas; recommendations accepted as a block)

## Gray areas selected
User multi-selected all four:
1. Cluster ID stability & format
2. Top-N ranking + cluster scoring
3. `get_change_impact_graph` input + shape vs `analyze_blast_radius`
4. Size budgets + confidence cap surfacing

## Q1 — Cluster ID stability
**Options presented:** opaque composite token (recommended) / bare int + implicit current graph / bare int + explicit graph_version param.
**User selected:** opaque composite token.
**Captured as:** D1.

## Q2 — Top-N ranking + scoring
**Options presented:** member count + conductance (recommended) / PageRank-sum + modularity / hybrid size+PageRank tiebreaker.
**User selected:** member count + conductance.
**Captured as:** D2.

## Q3 — `get_change_impact_graph` shape
**Options presented:** seed + depth, pure subgraph (recommended) / + hypothetical-edit metadata / + per-node `would_break`.
**User selected:** seed + depth, pure subgraph.
**Captured as:** D3. Hypothetical-edit metadata + `would_break` flag moved to Deferred Ideas.

## Q4 — Size budgets + confidence cap surfacing
**Options presented:** mirror Phase 71 D2 + cap in both per-edge and envelope (recommended) / caps only / envelope-only.
**User selected:** mirror Phase 71 D2 + cap in both places.
**Captured as:** D4.

## Carried forward from Phase 71 without re-asking
- Seed addressing (D1 → P72 D3 input)
- Edge-kind MCP enum (D3 → P72 D2/D3)
- Freshness envelope shape (D5 → P72 D5)
- Mode tier enforcement at handler entry (D6 → P72 D5)
- Test surface (D7 → P72 D5)

## Deferred ideas captured
- Hypothetical-edit metadata for `get_change_impact_graph`
- LLM-derived cluster labels
- Per-node `would_break` flag
- Cluster ID stability across rebuilds (member-overlap heuristic)
- Modularity / PageRank-sum ranking variants

## Scope creep
None raised.
