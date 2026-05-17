# Phase 71 — P1 Single-Symbol Read Tools — CONTEXT

**Date:** 2026-05-17
**Goal (from ROADMAP):** Three new `read+` MCP tools answer agent questions about a single seed symbol — deep explanation, related symbols, and edge validation — backed by the v1.10 semantic graph + `integ.SemanticLookup`.
**Requirements:** P1TOOL-01, P1TOOL-02, P1TOOL-06
**Depends on:** Phase 62 (graph engine, ranking, type resolution), Phase 65 (`integ.SemanticLookup` seam), Phase 68 (precise diffs so freshness envelope is meaningful), Phase 69 (real status surfaces)

## Domain

Phase 71 ships the first three of the six v1.11 P1 MCP tools — the **single-symbol read wave**:

1. **`explain_symbol_deep`** — given one seed, returns type chain (with ladder tier + resolved confidence), incoming + outgoing edges (classified by kind), callers, cluster membership reference, and the freshness envelope.
2. **`find_related_symbols`** — given one seed, returns up to `k` ranked sibling symbols via PageRank-from-seeds + cluster co-membership + RRF fusion.
3. **`validate_graph_edge`** — given an edge claim `(from, to, edge_kind)`, returns a confidence score + ordered evidence path (LSP / AST / type-resolver citations).

All three are `read+` mode tier, wrapped in `internal/skill/semantic/` per the Phase 64 P0 pattern (envelope.go + mode_check.go + accessors.go), and read-only on the graph. Profile/mode gating across the 5 × 4 matrix is **deferred to Phase 73**; Phase 71 only enforces `read+` at handler entry.

**Phase 71 ships:**

1. **Three handler files** in `internal/skill/semantic/` following Phase 64's pattern: `tools_explain_symbol.go`, `tools_find_related.go`, `tools_validate_edge.go`.
2. **Seed resolution accessor** on `integ.SemanticLookup` (or a thin wrapper) that turns `(file_path, symbol_name)` into a `graph.SymbolID` with closed-enum `resolution ∈ {exact, ambiguous, not_found}`. May already exist from Phase 65 — researcher to confirm.
3. **Edge-kind surface enum** — lowercase closed enum in `internal/skill/semantic/` with an internal→surface mapping table.
4. **Per-tool response envelope structs** carrying v1.10 closed-enum freshness/source/fallback_reason, plus per-section count totals.
5. **Unit + integration tests** for each tool against a populated graph fixture (reuse / extend the Phase 64 P07 builder). Profile/mode matrix tests land in Phase 73.

**Out of scope (deferred):**

- The cluster wave (`get_cluster_map`, `explain_cluster`, `get_change_impact_graph`) — Phase 72.
- Profile-filter golden tests across the 5 × 4 matrix — Phase 73.
- `get_tool_help` parameter doc extraction for the new tools — Phase 73 (P1TOOL-07).
- LSP-position addressing (`file:line:character`) — rejected; the graph is the source of truth for P1. Agents pivot from `goto_definition` via name resolution if needed.
- Pagination over caller / edge lists — explicit defer to a future phase if usage shows it matters.
- Per-call weight knobs for `find_related_symbols` ranking — RRF fusion is weight-free; opening tuning surface now is premature.

## Canonical Refs

**Downstream agents MUST read these before planning or implementing.**

### Phase 71 anchors
- `.planning/ROADMAP.md` — Phase 71 success criteria (5 items)
- `.planning/REQUIREMENTS.md` — P1TOOL-01, P1TOOL-02, P1TOOL-06 (and the P1TOOL-07/08/09 cross-phase obligations landing in 73)

### Skill envelope + mode-check pattern (Phase 64 P0 — the template to mirror)
- `internal/skill/semantic/envelope.go` — closed-enum freshness/source/fallback_reason
- `internal/skill/semantic/mode_check.go` — handler-entry mode tier enforcement
- `internal/skill/semantic/accessors.go` — graph/status accessor wiring
- `internal/skill/semantic/tools_context.go` — closest existing analog for "symbol-anchored read tool"; reuse its symbol_id input shape and clamp pattern
- `internal/skill/semantic/tools_status.go` — closest existing analog for "envelope-rich read tool"

### Graph + ranking + type resolution
- `internal/semantic/graph/repair.go:13` — internal `EdgeKind` strings (CALLS / RESOLVES_TO / USES_TYPE / …)
- `internal/semantic/graph/apply_repair.go` — edge insertion shape (source-of-truth for internal_kind values)
- `internal/semantic/types/golang/resolver.go:45` — 7-tier ladder + evidence enum (reused by `validate_graph_edge` for type-resolver citations)
- `internal/retrieval/` — PageRank, TextRank, RRF (Phase 62 — the fusion entrypoint that `find_related_symbols` consumes)

### Seed resolution
- `internal/integ/` (Phase 65) — `SemanticLookup` seam; this is the (file, name) → SymbolID resolver. Researcher must confirm whether the existing seam returns the closed-enum `resolution` field or whether a thin wrapper is needed in `internal/skill/semantic/accessors.go`.

### Prior phase patterns
- `.planning/phases/64-new-mcp-tools/64-CONTEXT.md` — original P0 tool wave; envelope conventions established
- `.planning/phases/64-new-mcp-tools/64-07-PLAN.md` + `64-07-SUMMARY.md` — populated-graph fixture builder to extend
- `.planning/phases/69-production-status-accessors/69-CONTEXT.md` — D1 lock-free `*Store` accessor pattern; Phase 71 read accessors mirror this
- `.planning/phases/70-incremental-refresh-overlay-drain/70-CONTEXT.md` — D2 paths-as-strict-subset semantics (`find_related_symbols.paths` mirrors this)
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md` — TYPES-04 confidence cap on degraded ladder paths

### Boundary invariants
- `.planning/codebase/CONVENTIONS.md` — closed-enum bounded labels; D-09 read-only on graph reads
- Build target `vet-nokernel2semantic` — kernel ↔ semantic boundary; all three tools live semantic-side

## Decisions

### D1 — Seed addressing: `symbol_id` OR `(file_path, symbol_name)` tuple

All three tools accept exactly two input variants for the seed (oneOf JSON schema):

- **`symbol_id`** — graph-internal stable key; same shape `get_context` already takes.
- **`file_path` + `symbol_name`** — resolved via `integ.SemanticLookup` (Phase 65).

`validate_graph_edge` takes two seeds (`from` and `to`), each independently using either variant.

**Envelope carries closed-enum `resolution`** per seed:

```
resolution ∈ {exact, ambiguous, not_found}
```

- `exact` — single graph symbol matched (or symbol_id was used directly).
- `ambiguous` — multiple symbols matched the (file, name) tuple; tool returns the highest-confidence match and lists alternatives in `ambiguous_candidates` (cap 5).
- `not_found` — no match; tool returns an error envelope with `fallback_reason: "symbol_not_found"`.

**Why:**
- Reuses Phase 64's `symbol_id` input convention (zero new vocabulary for that variant).
- (file, name) tuple removes the mandatory discovery hop for the common "I know the function name" case without forcing the agent through `get_context` first.
- LSP-position addressing rejected — would couple P1 to LSP warm state and undermine the graph-backed framing; `goto_definition` already serves that use case.

**Researcher must confirm:** whether `integ.SemanticLookup` already returns the three-state `resolution` enum, or whether a thin wrapper is needed in `internal/skill/semantic/accessors.go`.

### D2 — Response size budgeting: per-section caps + count totals (no pagination)

Each tool applies bounded per-section caps; envelope reports `*_count_total` vs `*_count_returned` so agents can detect truncation.

**`explain_symbol_deep` caps:**
- `max_callers = 50`
- `max_edges_per_direction = 100` (separate caps for incoming, outgoing)
- Type chain — no cap (closed structure; depth bounded by language semantics)
- Cluster membership returned as `{cluster_id, size}` reference — full member list comes from Phase 72's `explain_cluster`

**`find_related_symbols` caps:**
- `k` (top-N): default `20`, clamped `[1, 100]`
- No weight knobs in Phase 71 — RRF fusion is rank-based and weight-free; future phases can add `pagerank_weight` if usage demands it
- `paths` filter acts on **result set** (return only related symbols whose definition file is under one of the paths). The seed itself is NOT constrained by `paths`. Mirrors Phase 70 D2 paths-as-strict-subset precedent.

**`validate_graph_edge`:**
- Evidence cap: 10 citations (see D4)
- No list-shaped output otherwise; one confidence + one evidence array

**Why per-section caps over unified `max_tokens` budget:**
- Predictable, deterministic, easy to test exhaustively.
- No section-priority policy to document and defend ("which gets trimmed first when budget is tight").
- Agents that need exhaustive results can opt-in later when pagination lands (non-breaking add).

**Rejected:** unified `max_tokens` budget (added policy without clear benefit at this stage). Cursor pagination (state surface not justified by current agent use cases; can be added non-breakingly later).

### D3 — Edge-kind surface: closed MCP-surface enum + `internal_kind` field

`explain_symbol_deep` and `find_related_symbols` (when reporting edges in result rationale) classify edges using a **lowercase closed enum** decoupled from internal graph evolution:

```
edge_kind ∈ {calls, references, implements, extends, has_type, uses_type, contains, other}
```

- Mapping table from internal enum (`internal/semantic/graph/repair.go:13` — CALLS / RESOLVES_TO / USES_TYPE / etc.) lives in `internal/skill/semantic/` (new file: `edge_kind_surface.go`).
- Each emitted edge carries `internal_kind` (raw internal string) for debugging and to make the mapping table auditable.
- Unmapped internal kinds → `edge_kind: "other"` with `internal_kind` populated. A bounded-label metric records unmapped internal kinds so the mapping table can grow as new kinds are added internally.

**Why:**
- Matches v1.10 envelope convention (freshness, source, fallback_reason are all lowercase closed enums).
- Decouples MCP surface stability from internal graph refactors. Renaming `CALLS` → `INVOKES` internally doesn't break agent prompts hardcoded against `edge_kind: "calls"`.
- `internal_kind` field gives debuggers full fidelity without forcing the agent to learn the internal vocabulary.

**Rejected:** exposing internal `EdgeKind` strings directly (couples MCP contract to internal refactors); freeform edge_kind strings (defeats closed-enum guarantees that downstream agents rely on).

### D4 — `validate_graph_edge` evidence shape: flat ordered list, lenient

Evidence is a flat ordered list of citations, sorted by `confidence_contribution` descending:

```json
"evidence": [
  {
    "source": "lsp",
    "lsp_method": "callHierarchy/incomingCalls",
    "file": "...",
    "range": {"start": {...}, "end": {...}},
    "confidence_contribution": 0.4
  },
  {
    "source": "ast",
    "tree_sitter_kind": "call_expression",
    "file": "...",
    "range": {...},
    "confidence_contribution": 0.3
  },
  {
    "source": "type_resolver",
    "tier": "tier2_annotation",
    "evidence_kind": "annotation",
    "file": "...",
    "range": {...},
    "confidence_contribution": 0.3
  }
]
```

- **`source` closed enum:** `{lsp, ast, type_resolver}`.
- **`tier` field** populated only when `source == type_resolver`; reuses the existing 7-tier evidence enum from `internal/semantic/types/` (tier1_lsp / tier2_annotation / tier3_constructor / tier4_assignment / tier5_godoc / tier6_heuristic / tier7_unknown).
- **Cap:** 10 citations; envelope reports `evidence_count_total` vs `evidence_count_returned`.
- **`evidence_status` closed enum:** `{complete, partial, none}` — populated independently of confidence.

**Lenient stance:** non-zero confidence with empty evidence is allowed when the graph attests the edge but citation retrieval lags. Envelope flags this via `evidence_status: "none"` and applies the Phase 62 TYPES-04 confidence cap (max 0.6 on degraded paths).

**Why flat over tree:**
- Easy to render in agent UIs and stream-parse.
- Reuses existing closed enums (no new vocabulary).
- Multi-hop derivation chains are rare for edge evidence — edges are usually directly attested; the rare type-resolver tier-4 assignment-flow chains lose minor expressiveness in flat form, an acceptable tradeoff.

**Why lenient:**
- Matches Phase 62 TYPES-04 — degraded paths return capped results, not refusals.
- Strict stance would punish real graph data when citation lookup lags or is unimplemented for an edge kind.

### D5 — Freshness envelope: graph_version + snapshot_id + extractor_run_id

All three tools carry a freshness envelope mirroring v1.10 conventions:

```json
"freshness": {
  "graph_version": "v3.7.42",
  "snapshot_id": "snap_abc123",
  "extractor_run_id": "run_xyz789",
  "as_of_unix_ms": 1715760000000,
  "status": "current"
}
```

- `status ∈ {current, stale, unknown}` — Phase 69 D1 three-state envelope reused verbatim.
- `source ∈ {graph, type_resolver_ladder, ast_fallback}` — closed-enum, identifies which subsystem produced the result.
- `fallback_reason` — populated when degraded; closed-enum reusing Phase 62/64 vocabulary.

**Why three identifiers (graph_version, snapshot_id, extractor_run_id):**
- `graph_version` — the committed graph generation for cache invalidation.
- `snapshot_id` — pinpoints the exact `*Store` snapshot the read happened against (Phase 69 accessor pattern).
- `extractor_run_id` — last extraction pass that touched the seed's defining file; matters for "did my recent edit land in this answer?" agent reasoning.

**Researcher confirms:** the exact shape of Phase 69's status envelope and whether `extractor_run_id` is already exposed by an existing accessor or needs a new read accessor.

### D6 — Read-only invariant + mode tier enforcement

All three handlers MUST:

- Enforce `read+` mode tier at handler entry via `mode_check.go` (mirror Phase 64 pattern).
- Use only lock-free `*Store` read accessors and graph/retrieval read entrypoints. No writes, no overlay tx, no snapshot commits. Honors D-09.
- Apply the Phase 62 TYPES-04 confidence cap (≤ 0.6) when any computation falls back to a degraded path (e.g., type-resolver tier 6/7, AST-only edge evidence). Envelope's `fallback_reason` carries the closed-enum reason.

A CI grep-gate on the three new handler files (mirroring `tools_refresh.go`'s gate) enforces absence of `Begin/Commit/Abort/Write` snapshot tokens.

### D7 — Test surface

Per success criterion #5 (race-clean under concurrent invocation):

1. **`internal/skill/semantic/tools_explain_symbol_test.go`** — table-driven:
   - Happy path: known seed in Go fixture, asserts type chain + caller count + edge classifications.
   - Resolution variants: exact / ambiguous / not_found.
   - Truncation: hot symbol with > 50 callers / > 100 edges → asserts envelope counts.
   - Degraded path: type-resolver tier-6 hit → asserts confidence cap + `fallback_reason`.

2. **`internal/skill/semantic/tools_find_related_test.go`** — table-driven:
   - Default k=20; clamping at [1, 100].
   - `paths` filter: results all under filter, seed outside filter is allowed.
   - Empty result (isolated seed) → returns empty list, not an error.

3. **`internal/skill/semantic/tools_validate_edge_test.go`** — table-driven:
   - Edge present with full evidence (`complete`).
   - Edge present with no citations available (`none`, capped confidence).
   - Edge absent (low confidence + appropriate `fallback_reason`).
   - Internal-kind round-trip through the surface enum (all known mappings + an unmapped fixture → `other`).

4. **Integration test** in `internal/skill/semantic/integration_test.go` (or sibling) — run all three tools against the populated Phase 64 P07 graph fixture, assert envelope shape + cross-tool consistency (e.g., `explain_symbol_deep`'s incoming-edges count agrees with `validate_graph_edge` confirmations for sampled edges).

5. **Race testing:** all tests run under `go test -race`. Concurrent invocation test spins N goroutines hitting each tool against the same seed; asserts identical responses (idempotency) and no race detector hits.

**Why semantic-skill location:** matches Phase 64 P0 test layout; co-locates with the handlers they cover.

## Implementation Notes (for researcher / planner)

- Phase 65's `integ.SemanticLookup` may already expose the resolution enum; if not, the wrapper lives in `internal/skill/semantic/accessors.go` and reuses `integ.SemanticLookup` for the underlying name→symbol mapping.
- Edge-kind surface mapping table: start with the kinds currently visible in `internal/semantic/graph/repair.go` and `apply_repair.go`. New file `internal/skill/semantic/edge_kind_surface.go` with the mapping + a `MapInternalKind` function. Mapping is one-way (internal → surface); reverse is not needed because tools don't accept surface-enum input.
- `find_related_symbols` consumes the existing PageRank-from-seeds + RRF fusion entrypoint in `internal/retrieval/` (same one `get_context` uses); cluster co-membership boost comes from the Phase 62 cluster engine. Researcher confirms whether a new fusion helper is needed or whether the existing retrieval surface already accepts the seed-symbol input shape.
- `validate_graph_edge` confidence formula needs a planner decision: simple "sum of contributions clamped to [0,1]" vs weighted by source priority. Researcher proposes; planner ratifies. Default proposal: each source contributes up to 0.4 (LSP) / 0.3 (AST) / 0.3 (type_resolver, scaled by tier). All three present → 1.0; any single present → at most 0.4.
- Per success criterion #1 (Go / TypeScript / Java), test fixtures must include at least one symbol per language. Reuse Phase 64 P07 builder; if it's Go-only today, extending it to TS/Java is part of Phase 71 (small extraction).
- `extractor_run_id` exposure: Phase 69's status accessor may already carry this; if not, a new read accessor on `*Store` is the cleanest add (mirrors Phase 70 D2 since-epoch accessor pattern).
- All three handlers register through `init()` (Caddy-style) in `internal/skill/semantic/register.go`. No daemon-bootstrap special-casing (P1TOOL-08).

## Deferred Ideas

- **Pagination cursors for callers / edges** — defer until usage shows truncation bites real workflows.
- **Per-call weight knobs for `find_related_symbols`** — defer until a concrete tuning need emerges; RRF is weight-free by construction.
- **LSP-position addressing** for seeds — rejected for P1; `goto_definition` covers position-based discovery, which then yields a symbol_id.
- **Tree-of-derivation evidence** for `validate_graph_edge` — defer; flat list is sufficient for known evidence shapes.
- **Streaming progressive responses** over MCP — UX add-on, not in scope.
- **Edge-kind surface enum bidirectional mapping** (surface → internal) — defer until a tool accepts surface-enum input (none in Phase 71/72 do).

## Spec Lock

No SPEC.md present for Phase 71. Requirements come from `.planning/REQUIREMENTS.md` (P1TOOL-01, P1TOOL-02, P1TOOL-06) and ROADMAP success criteria.

---

*Phase: 71-p1-single-symbol-read-tools*
*Context gathered: 2026-05-17 via /gsd-discuss-phase*
