# Phase 74: Discussion Log

**Gathered:** 2026-05-29
**Purpose:** Human-readable audit trail of the discuss-phase Q&A. Not consumed by downstream agents — they read `74-CONTEXT.md`.

## Domain framing

Phase 74 is the closure phase prescribed by `.planning/v1.11-MILESTONE-AUDIT.md`:
all 10 P1 accessor setters have zero production callers; the 6 P1 MCP tools
return degraded envelopes against any real workspace. Phase 74 wires the
accessors, closes BLOCKER-2 (FreshnessV2), and adds a CI gate so the gap can't
re-open silently.

## Areas selected by user

User selected all four offered gray areas:
1. Accessor adapter sourcing
2. SetExtractorRun source
3. CI gate + E2E shape
4. WR-04 / WR-05 fold-in

## Q1 — Scope of accessor wiring

**Scout finding presented:** Only 6 of the 10 P1 accessors have production
`*Store` query methods (`QuerySymbolByName`, `LatestExtractorRunID`,
`QueryClusterSummaries`, `QueryClusterMembers`, `QueryNodePageRanks`, plus
`ImpactLookup` via existing `integSemanticLookup`). The remaining 4 —
`TypeChain`, `SymbolEdges` (callers/in/out), `EdgeEvidence`,
`ClusterMembership` — have no production query layer; only test fakes inline
in `tools_*_test.go`.

**Options:**
- Wire all 10 (build missing Store SQL in Phase 74)
- Wire the 6 ready, leave 4 unwired (graceful, smallest scope)
- Split: Phase 74 wires 6, Phase 75 builds 4 queries
- Build 4 missing queries first, then decide (researcher-led)

**User selection:** **Build all 4 missing queries first, then decide**

**Notes:**
- Locks D-01: 6 ready accessors are committed scope.
- D-01a: the 4 missing-query accessors are research-gated; gsd-phase-researcher
  must map *Store schema, test fakes, and SQL complexity before scope locks.
- Researcher's table format specified in CONTEXT.md `<specifics>`.

## Q2 — SetExtractorRun source

**Options:**
- Use `*Store.LatestExtractorRunID` directly (thin adapter)
- Derive from IndexRunner's last successful run (cached)
- Have researcher confirm under current snapshot semantics

**User selection:** **Use *Store.LatestExtractorRunID directly**

**Notes:**
- Locks D-02. Matches Phase 71-01 seam choice; `TestLatestExtractorRunID_PopulatedSnapshot`
  proves non-empty under populated snapshots.
- Closes BLOCKER-2 independently of D-01a decision.

## Q3 — CI gate + production E2E shape

**Options:**
- Runtime bootstrap test + lightweight E2E
- Static source-scan + heavy daemon E2E
- Both: source-scan + runtime bootstrap test + light E2E

**User selection:** **Runtime bootstrap test + lightweight E2E**

**Notes:**
- Locks D-03 / D-03a. Runtime non-nil gate catches future deferral
  regressions where a static scan would drift.
- Lightweight E2E uses `buildP1E2EFixture` data shape but drives the
  production `b.skill`, not test-only `Set*` fixtures.
- D-03b: for accessors that may stay nil (D-01a outcome), assert the
  documented `fallback_reason` is emitted — degradation is part of the
  production contract.

## Q4 — WR-04 / WR-05 fold-in

**Options:**
- Defer to a future phase
- Fold WR-04 only (cheap, high-signal)
- Fold both (WR-05 needs new kind-aware accessor)

**User selection:** **Defer to a future phase**

**Notes:**
- Locks D-05. Both are Phase 72 handler bugs, not wiring bugs.
- Captured in CONTEXT.md `<deferred>` for future phase pickup.

## Claude's discretion

- Adapter struct names, file grouping (one file per adapter vs by 71/72 origin) — planner picks.
- Non-nil assertion style (table-driven vs subtests) — planner picks.

## Deferred ideas captured

- WR-04: `get_cluster_map` representative_symbols emit decimal node-IDs
- WR-05: `computeDominantEdgeKinds` permanently empty
- Phase 71 bounded-label metric TODOs (`edge_kind_surface.go:23, 89`)
- Full daemon-boot + MCP-client E2E (heavier than audit closure needs)
- Phase 75 (potential): close 4 missing-query accessors if researcher recommends split

## Scope-creep redirects

None — discussion stayed within audit-defined boundary.
