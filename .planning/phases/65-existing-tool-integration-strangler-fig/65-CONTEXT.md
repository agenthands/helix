# Phase 65: Existing-Tool Integration (Strangler Fig) - Context

**Gathered:** 2026-05-08
**Status:** Decisions captured. **BLOCKED on Phase 59 (EXTRACT-01..05)** — see D-08 below. Resume planning after Phase 59 ships.

<domain>
## Phase Boundary

Phase 65 wires four already-shipped MCP tools to the Phase 60-64 semantic
engine via dependency injection — with **zero source change to
`internal/repomap` engine** — and an automatic v1.9 fallback whenever the
semantic index is disabled, building, or errored.

**Phase 65 ships:**

1. **`get_repo_map`** — consults `repomapSkill.SetSemanticLookup(lookup)`
   when available; uses persisted graph scores + clusters for ranking;
   falls back to existing tree-sitter + PageRank path on
   `semantic_index.enabled=false` or any unavailability. Index-disabled
   goldens preserved verbatim. (INTEG-01)

2. **`get_context`** — same lookup-via-setter contract as `get_repo_map`;
   delegates to the semantic retrieval engine when available; preserves
   v1.9 dependency-graph behavior on fallback. (INTEG-02)

3. **`analyze_blast_radius`** — graph-first expansion via
   `lookup.ExpandFrom`, then LSP validation of *critical edges* (those
   crossing public-API boundaries OR with confidence < 0.80); per-node
   `confidence` and `evidence`; on fallback drops to LSP-only with
   confidence ≤ 0.6. (INTEG-03)

4. **`get_health`** — extends the existing `semantic_store: {state}`
   envelope (Phase 57 SC-1) into the full `semantic_index` block per SPEC
   §24.5: store kind, latest snapshot status, graph_version, overlay
   active flag, pending LSP count, last live-update latency, last error.
   (INTEG-04)

5. **Closed-enum `source` field** on every semantic-aware MCP envelope:
   `semantic | tree_sitter | fallback`. Operators and goldens detect
   path drift via this field. (INTEG-05, M7 mitigation)

6. **Phase 64 carryover items** — both TODO(phase-65) anchors land as
   Wave 0 prerequisites:
   - **Production buildFn**: replace the empty-Facts placeholder at
     `internal/daemon/semantic_wiring.go:687-742` with a real
     per-language extractor + classifier walk + overlay drain pipeline.
   - **WorkspaceKey adapter**: replace the zero-value return at
     `internal/daemon/semantic_wiring.go:509-525` with a registry lookup
     so retrieval is workspace-scoped (not against `repoRoot=""`).

**Out of scope (deferred to later phases):**

- **Per-language extractors for TS / JS / Python** — Phase 59 EXTRACT-01
  ships these. Phase 65's production buildFn consumes whatever Phase 59
  ships; it does not implement extraction.
- **Agent guardrails** (G-001..G-005) — Phase 66.
- **Evaluation harness** for the strangler-fig integration — Phase 67.
- **Tools 5-10 from SPEC §23.5-23.10** — deferred to v1.10.x.
- **Cluster-shipping algorithms beyond weak-components** — already
  scoped out per REQUIREMENTS.md.
- **Multi-projection PageRank surfacing in `get_repo_map` output** —
  Phase 62 ships only CALL_GRAPH_PAGERANK + REFERENCE_PAGERANK +
  FILE_DEPENDENCY_PAGERANK; Phase 65 surfaces only those.

</domain>

<decisions>
## Implementation Decisions

### Lookup / injection seam

- **D-01: Single `SemanticLookup` interface, one setter per skill.**
  Mirror the existing `SetEnrichFn` / `SetFallbackDeps` /
  `SetMetricsSink` pattern from `internal/skill/repomap/skill.go:115-144`.
  Each skill that needs semantic adds one setter:
  - `RepoMapSkill.SetSemanticLookup(lookup)` — for `get_repo_map` and
    `get_context`.
  - A kernel-resident bridge (likely `internal/kernel/symbols/`) accepts
    the same lookup for `analyze_blast_radius` via the
    `skill_adapter.go` precedent set by `internal/kernel/health/` and
    `internal/kernel/help/`.
  - The kernel-resident `health` package already exposes
    `SemanticStoreProbe` (Phase 57 SC-1) — Phase 65 widens it (or adds
    a parallel `SemanticIndexAccessor`) to consume the same lookup.

  One interface = one mental model. Caddy-style post-init wiring stays
  uniform.

- **D-02: Interface lives in `internal/semantic/integ/`; production
  adapter in `internal/daemon/semantic_wiring.go`.** New tiny package
  `internal/semantic/integ/` declares ONLY:
  - `type SemanticLookup interface { ... }`
  - Value types: `RankedFile`, `Impact`, `Edge`, `SemanticStatus`,
    `FallbackReason`, `Source`.
  - The closed-enum `source` constants and `freshness` aliases
    (re-exporting Phase 64's enum).

  No imports from `internal/kernel` or `internal/skill/*` — keeps the
  vet-nokernel2semantic boundary clean and lets every consumer import
  the integ package without dragging in store/retrieval concretions.

  Production adapter (`integSemanticLookup` or similar) wraps
  `*semanticstore.Store` + `*retrieval.Engine` + `RankScheduler` and
  lives next to `semanticBundle` in
  `internal/daemon/semantic_wiring.go`.

- **D-03: Lookup interface shape (Wave 1 contract — planner refines
  exact signatures).**

  ```go
  type SemanticLookup interface {
      Available() bool                              // semantic_index.enabled gate
      RankFiles(ctx, ws) ([]RankedFile, error)      // get_repo_map
      RankFromSeeds(ctx, ws, seeds []string)        // get_context
          ([]RankedFile, error)
      ExpandFrom(ctx, ws, sym SymbolID, depth int)  // analyze_blast_radius pass 1
          ([]Impact, error)
      ValidateCriticalEdges(ctx, ws, edges []Edge)  // analyze_blast_radius pass 2
          ([]ValidatedEdge, error)
      Status(ctx, ws) (SemanticStatus, error)       // get_health
  }
  ```

  All methods are read-only — they NEVER trigger snapshot writes (read+
  tier doctrine carried from Phase 64 D-09). Errors include sentinels
  for `ErrNoSnapshot`, `ErrIndexBuilding`, `ErrIndexErrored` so callers
  can map them cleanly to `source=fallback` + a `fallback_reason`.

### Source-field envelope contract

- **D-04: Closed-enum `source` field, three values:
  `semantic | tree_sitter | fallback`.** Matches SPEC §24 and
  INTEG-05 phrasing exactly:
  - `semantic` — ranking sourced from the persisted graph + clusters
    (semantic enabled AND lookup served the call).
  - `tree_sitter` — index disabled by config (`semantic_index.enabled=false`);
    the v1.9 path ran. Goldens cover this branch.
  - `fallback` — index enabled but unavailable for this call (no
    snapshot yet, building, errored, etc.); v1.9 path ran with a
    `fallback_reason` in the envelope.

  Data quality (pending LSP, stale scores, partial enrichment) lives on
  the existing `freshness` field, NOT on `source`. Per-node certainty
  for `analyze_blast_radius` lives on `confidence`. Three orthogonal
  signals, not one overloaded enum.

- **D-05: `fallback_reason` closed enum on the envelope when
  `source=fallback`.** Values: `index_disabled` (defensive — should be
  `tree_sitter` not `fallback`), `no_snapshot_yet`, `index_building`,
  `index_error`, `bleve_rebuilding`. Operators and goldens get a
  precise diagnostic without the fallback path leaking raw error text
  (WR-NEW-01 doctrine carried from `internal/kernel/health/tools.go`).

- **D-06: Cold start (semantic enabled, no committed snapshot) →
  `source=fallback`, NO background indexing kicked.** Honors SPEC §7
  acceptance "no blocking full index on normal tool use unless
  requested" and SPEC §25 `indexing.mode=lazy` /
  `auto_index_on_activate=false`. Agents that want fresh semantic
  ranking call `index_semantic_graph` explicitly. This phase never
  silently triggers a multi-second index from a foreground tool call.

### `analyze_blast_radius` semantic + LSP layering

- **D-07: Graph-first expansion, LSP validates critical edges.**
  Two-pass algorithm:

  **Pass 1 — graph expansion (cheap, persisted, deterministic):**
  ```go
  impacts := lookup.ExpandFrom(ctx, ws, sym, depth=2)
  // Each Impact carries: node_id, edge_kind, confidence (Phase 62
  // ladder: 1.00 LSP / 0.95 merged / 0.80 ts+local / 0.70 ts-only /
  // 0.45 heuristic), evidence (edges, ranks).
  ```

  **Pass 2 — LSP validation of critical edges only:**
  ```go
  critical := filter(impacts, e => e.crossesPublicAPI || e.confidence < 0.80)
  validated := lookup.ValidateCriticalEdges(ctx, ws, critical)
  // Validated edges → confidence flips to 1.00.
  // Refuted edges (LSP says edge does not exist live) → confidence
  // drops to 0.20, marked `refuted: true`.
  ```

  **"Critical edge" definition:** an edge is critical when EITHER
  (a) it crosses a public-API boundary (caller in package A ↔ callee
  exported from package B), OR (b) the edge's confidence is below 0.80
  (the Phase 62 ts+local threshold). All other edges trust the
  persisted graph signal.

  Reuses Phase 62 confidence ladder verbatim — no new mapping. Matches
  SPEC §24.3 ordering ("persistent graph expansion / live overlay
  state / LSP validation of critical edges").

- **D-08: Fallback-path confidence cap ≤ 0.6.** When semantic is
  unavailable (any `source=fallback` or `source=tree_sitter` envelope),
  per-node `confidence` is hard-capped at 0.6 — even though LSP is
  technically ground truth (1.00). Rationale: without graph
  corroboration we cannot surface evidence beyond direct references.
  Honors ROADMAP success criterion #2 verbatim: "on fallback,
  confidence drops to ≤ 0.6 and the result envelope says so."

  Envelope on the fallback path:
  ```json
  {
    "source": "fallback",
    "fallback_reason": "no_snapshot_yet",
    "per_node": [
      { "node_id": "...", "confidence": 0.6,
        "evidence": { "lsp_locations": [...] }, "lsp_validated": true }
    ]
  }
  ```

### Carryover items from Phase 64 → Phase 65 wave structure

- **D-09: Both Phase 64 carryovers land as Phase 65 Wave 0
  prerequisites.** Both items exist in service of the strangler-fig
  integration; Phase 65 is their natural home.

  - **65-Wave 0 / 65-01-PLAN: production buildFn** — replace the
    empty-Facts commit at `internal/daemon/semantic_wiring.go:687-742`
    with a real pipeline: per-language fact extraction (consumes
    Phase 59 EXTRACT-01..05) + classifier walk (Phase 60 LIVE-01) +
    overlay drain → store.WriteSnapshotFacts. The pipeline contract is
    documented inline at `semantic_wiring.go:680-686`.

  - **65-Wave 0 / 65-02-PLAN: WorkspaceKey adapter** — replace
    `semSessionAdapter.Workspace` zero-value return at
    `semantic_wiring.go:509-525` with a registry lookup that resolves
    `*mcp.SessionInfo` (which carries the hashed workspace key string)
    back to the canonical `workspace.WorkspaceKey` struct held by the
    daemon's `activeWSKey` registry. Without this, retrieval queries
    against `repoRoot=""` and Phase 65's strangler-fig integration
    inherits zero ranking signal.

### Cross-phase blocker

- **D-10: Phase 65 BLOCKED on Phase 59 (EXTRACT-01..05).** Phase 65's
  Wave 0 buildFn consumes per-language extractors. Phase 59 is sitting
  in REQUIREMENTS.md as Pending. Sequence shift agreed:

  ```
  next: /gsd-discuss-phase 59  (full Phase 59: EXTRACT-01..05)
  then: /gsd-plan-phase 59 → /gsd-execute-phase 59 → /gsd-verify-phase 59
  then: resume /gsd-plan-phase 65 with extractors as a prerequisite
  ```

  This CONTEXT.md captures the Phase 65 decisions reached today; they
  remain valid until Phase 59 ships. Phase 64 carryover TODO(phase-65)
  anchors stay in code unchanged. Update STATE.md to reflect the shift.

### Wave plan (proposed; planner refines after Phase 59 ships)

```
Wave 0 (carryover prerequisites — depends on Phase 59 EXTRACT-01)
  - 65-01-PLAN: production buildFn — real Facts pipeline
  - 65-02-PLAN: WorkspaceKey adapter — registry lookup, not zero value

Wave 1 (lookup seam + envelope contract)
  - 65-03-PLAN: SemanticLookup interface in internal/semantic/integ/
                + production adapter in internal/daemon/
  - 65-04-PLAN: source-field envelope contract + closed enum tests
                across all four tools

Wave 2 (per-tool integration)
  - 65-05-PLAN: get_repo_map + get_context wired via SetSemanticLookup
  - 65-06-PLAN: analyze_blast_radius graph-first + LSP validation
  - 65-07-PLAN: get_health semantic_index block expansion (INTEG-04)

Wave 3 (acceptance closure)
  - 65-08-PLAN: index-disabled goldens preserved + integration tests
                exercising every {source × fallback_reason} pair
```

### Locked from prior phases (NOT re-discussed)

- **All five profiles see all four tools** — Phase 64 D-14 (symmetric
  matrix). Phase 65 changes nothing in profile YAMLs; the four tools
  remain visible in `tools/list` for every profile. SPEC §30.2 mode
  gating (read+ for `get_repo_map` / `get_context` / `get_health` /
  `analyze_blast_radius`) is enforced inside each tool, not in
  middleware.
- **`refresh` overlay-only / `index --mode=incremental` snapshot-
  commit boundary** — Phase 64 D-09/D-10. The strangler-fig path is
  read-only with respect to committed state; lookup methods never call
  the Phase 63 compactor or `BeginSnapshot` directly.
- **Stable-key tiebreak ordering** — Phase 62 CR-03 (sort-before-
  iterate). `get_repo_map` and `get_context` deterministic ranking
  iteration uses the same tiebreak: score → graph_version →
  symbol_id / file_path.
- **ProfileFilterMiddleware unchanged** — `BriefDescriptions()`
  pass-through happens automatically; Phase 65 adds nothing to
  middleware order. CLAUDE.md "Middleware Execution Order (LIFO)"
  invariant preserved.
- **`get_tool_help` registry** — the four tools already have help
  metadata registered (Phase 64 TOOL-05). Phase 65 only adjusts
  descriptions to mention the new `source` field; help registration
  is not a separate plan.
- **Stable-bleve recovery procedure** — Phase 64 D-08 specifies
  rebuild-from-snapshot when `bleve` segment is missing or stale.
  Phase 65 surfaces this as `fallback_reason=bleve_rebuilding` on
  affected envelopes; the rebuild itself is Phase 64 wiring.

### Claude's Discretion (no user input needed)

- **Exact Go signatures for the SemanticLookup interface** — the
  shape in D-03 is the contract; planner refines parameter ordering,
  context-vs-options struct splits, error sentinels.
- **Where the kernel-resident `analyze_blast_radius` skill adapter
  lives** — `internal/kernel/symbols/skill_adapter.go` (new) following
  the `internal/kernel/health/skill_adapter.go` /
  `internal/kernel/help/skill_adapter.go` precedent.
- **Whether `Status` returns one struct or splits per concern** —
  `SemanticStatus` carries the SPEC §24.5 fields; planner decides
  whether to inline accessors per field or one bag struct.
- **`get_health` block name: `semantic_store` (extend) vs
  `semantic_index` (parallel block)** — recommend extending the
  existing `semantic_store` block in place to add the new fields,
  keeping the field-add additive (no rename, no breaking change to
  Phase 57 SC-1 consumers). Planner confirms after reviewing the
  current envelope's downstream consumers.
- **Goldens fixture layout** — index-disabled goldens for
  `get_repo_map` / `get_context` reuse the existing fixtures from the
  pre-Phase-65 codebase; semantic-on goldens are new fixtures. Planner
  picks fixture filenames + repo layout.
- **Singleflight on lookup methods** — if planner judges concurrent
  identical calls warrant deduplication (e.g., two simultaneous
  `analyze_blast_radius` calls on the same symbol), wrap with
  `golang.org/x/sync/singleflight.Group` per Phase 64 D-02 precedent.
  Otherwise, lookup methods are stateless reads against the same
  underlying engine and concurrency is fine.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements (load-bearing)

- `.planning/REQUIREMENTS.md` lines 76-82 — INTEG-01..05 phrasing is
  the contract. Each requirement maps to acceptance criterion #1..#4
  on the ROADMAP.
- `.planning/ROADMAP.md` lines 194-204 — parent ROADMAP Phase 65 block
  (Goal, Depends on, Requirements, 4 Success Criteria, Carryover from
  Phase 64).
- `.planning/milestones/v1.10-ROADMAP.md` lines 165-174 — milestone
  detail file (mirrors the parent ROADMAP entry).

### Specification (source of truth for shapes and rules)

- `SPEC-DRAFT.md` §24 (Integration With Existing Tools) — §24.1
  `get_repo_map`, §24.2 `get_context`, §24.3 `analyze_blast_radius`,
  §24.4 Edit Tools (LIVE-07 — already shipped), §24.5 `get_health`
  (full `semantic_index` block shape).
- `SPEC-DRAFT.md` §7 acceptance — "Existing tools preserve fallback
  behavior. No regression when semantic index disabled. No blocking
  full index on normal tool use unless requested."
- `SPEC-DRAFT.md` §17 (Graph Cache and Repair) — `analyze_blast_radius`
  consumes the repair rules; lookup methods never trigger ApplyRepair.
- `SPEC-DRAFT.md` §18 (PageRank and Ranking) — `get_repo_map` and
  `get_context` consume persisted PageRank scores.
- `SPEC-DRAFT.md` §25 (Configuration) — `semantic_index.enabled` is
  the master gate; Phase 65 adds NO new config keys (RRF weights stay
  Go-internal per Phase 64 D-07).
- `SPEC-DRAFT.md` §26.2 (Stale and Pending States) — closed enum for
  the `freshness` field.
- `SPEC-DRAFT.md` §30.2 (MCP Mode Gating) — all four tools are read+.

### Phase 60-64 lock-down (must not regress)

- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md` — D-04
  overlay_epoch CAS contract; lookup methods consume the live overlay
  through `*Store.QueryEffectiveAdjacency`.
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md`
  — RankScheduler + ApplyRepair + graph_version single-bump;
  confidence ladder used by analyze_blast_radius D-07.
- `.planning/phases/63-compaction-retention/63-CONTEXT.md` — snapshot-
  write API consumed by Phase 65 Wave 0 production buildFn (65-01).
  Compactor stays untouched by lookup methods (read+ doctrine).
- `.planning/phases/64-new-mcp-tools/64-CONTEXT.md` — Phase 64 ships
  the four NEW tools that Phase 65 doesn't touch; D-09 (refresh
  overlay-only), D-14 (all profiles see all tools) are the locked
  invariants Phase 65 inherits.
- `.planning/phases/64-new-mcp-tools/VERIFICATION.md` lines 110-115 —
  the two TODO(phase-65) anchors and their inline contract docs.

### Phase 59 dependency (BLOCKER)

- `.planning/REQUIREMENTS.md` lines 32-36 — EXTRACT-01..05 contract
  (typed symbols, references, imports, edges for Go / TS / JS /
  Python; stable symbol IDs per SPEC §11.1; LSP merge per SPEC §11.2).
- `SPEC-DRAFT.md` §11 (Stable Symbol Identity, LSP merge) — the
  contract Phase 65's production buildFn consumes.

### Architectural invariants (CLAUDE.md)

- `CLAUDE.md` "Middleware Execution Order (LIFO)" — Phase 65 does NOT
  add or reorder middleware; profile/mode gating is unchanged.
- `CLAUDE.md` "Tool Registration" — kernel tools use
  `RegisterTools(server *mcp.SerenaMCPServer, ...)`; skill tools use
  `ToolProvider.Tools()`. Phase 65's lookup setter follows the
  established `SetEnrichFn` / `SetFallbackDeps` precedent.
- `CLAUDE.md` "Skill System" — Caddy-style `init()` registration;
  daemon imports skills via blank imports in
  `internal/daemon/imports.go`.
- `CLAUDE.md` "Code intelligence: SMTC-first tool routing" — agents
  building Phase 65 use SMTC tools for the cross-package navigation,
  not Grep.

### Pattern templates (must mirror)

- `internal/skill/repomap/skill.go:113-144` — `SetEnrichFn` /
  `SetFallbackDeps` / `SetMetricsSink` setter pattern. Phase 65's
  `SetSemanticLookup` follows this exactly.
- `internal/kernel/health/skill_adapter.go` and
  `internal/kernel/help/skill_adapter.go` — kernel-resident tool
  wrapped as a skill adapter when the tool needs kernel state.
  `analyze_blast_radius` adapter follows this.
- `internal/kernel/health/tools.go:139-181` — `RegisterTools` accepts
  the daemon-supplied `SemanticStoreProbe` seam (SC-1). Phase 65
  widens this signature (or adds a parallel parameter) for the
  fuller `semantic_index` block.
- `internal/daemon/semantic_wiring.go` — Phase 64's `semanticBundle`
  + `semSessionAdapter` + `semStoreAdapter` precedent for the Phase
  65 production adapter. New types live next to these.

### Validation tooling

- `cmd/vet-noduckdb/` — Phase 65 `internal/semantic/integ/` package
  MUST NOT import `duckdb-go`; lookup methods go through
  `internal/semantic/store/`.
- `cmd/vet-nokernel2semantic` — kernel does NOT import
  `internal/semantic`. The new `internal/kernel/symbols/skill_adapter.go`
  imports `internal/semantic/integ` (interface only), NOT
  `internal/semantic/store/` or `internal/semantic/retrieval/`.

### Reference test artifacts (Phase 64)

- `internal/skill/semantic/integration_test.go` —
  `TestE2E_IndexThenContext_SymbolCount` (15-symbol fixture) is the
  current closure of W3 at the test layer. Phase 65 Wave 0 grows this
  fixture once production buildFn ships real Facts.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`internal/skill/repomap/skill.go`** — full skill scaffold: TagCache,
  TagExtractor, TreeRenderer, ElisionRenderer, FileGraph,
  `SetEnrichFn`, `SetFallbackDeps`, `SetMetricsSink`. Phase 65 adds
  `SetSemanticLookup` alongside — no engine source changes.
- **`internal/kernel/symbols/tools.go:560-585`
  (`registerAnalyzeBlastRadius`)** — current pure-LSP implementation
  via `find_references` + call/type hierarchy. Phase 65 wraps this with
  a graph-first pre-pass; the existing `AnalyzeBlastRadius` core stays
  as the LSP-validation primitive (Pass 2).
- **`internal/kernel/health/tools.go:139-181`** — `get_health` already
  wires a `SemanticStoreProbe` (Phase 57 SC-1) and surfaces a
  `semantic_store: {state: disabled|ready|unhealthy, reason}` block.
  Phase 65 extends this in place rather than adding a parallel block —
  fields ADD, no rename, additive evolution.
- **`internal/daemon/semantic_wiring.go:308-525`** — `semSessionAdapter`,
  `semStoreAdapter`, `semSchedulerAdapter`, `semQueueAdapter`,
  `semRetrievalAdapter` precedents. The Phase 65 production lookup
  adapter joins this set (`integSemanticLookup`).
- **`internal/semantic/retrieval/`** — bleve + RRF engine wired in
  Phase 64; Phase 65's lookup `RankFromSeeds` / `RankFiles` calls into
  this with the workspace-scoped engine handle.
- **`internal/graph/`** — Phase 62 PageRank + clustering + persisted
  scores. Lookup `RankFiles` reads scores via the existing read API
  (no recomputation in the hot path).
- **Phase 62 ApplyRepair post-commit hook** — `graph_version` bumps
  observable to lookup callers; `Status` surfaces the bumped version.
- **`internal/mcp/middleware.go` `ProfileFilterMiddleware` and
  `BriefDescriptions()`** — already filters `tools/list` by active
  profile. Phase 65 adds nothing here; descriptions update for the
  four tools to mention the new `source` field is purely cosmetic.
- **`mcpsdk.AddTool` + `kernel.WrapToolSpan` + `errorResult` /
  `textResult`** — established result-shape helpers in
  `internal/kernel/symbols/tools.go` and
  `internal/kernel/health/tools.go`. Phase 65 reuses these unchanged.

### Established Patterns

- **Caddy-style `init()` registration** — `internal/daemon/imports.go`
  blank-imports each skill package; Phase 65 adds nothing here (no new
  skills land — `internal/semantic/integ/` is a types-only package).
- **Setter post-init wiring for cross-package deps** — `SetEnrichFn` /
  `SetFallbackDeps` / `SetMetricsSink` (repomap), `SetActivateCallback`
  (kernel/workspace), `SetSessionFn` (semanticBundle Phase 64). Phase
  65 `SetSemanticLookup` is the next entry in this family.
- **Closed-enum reasons in MCP envelopes (WR-NEW-01)** — see
  `internal/kernel/health/tools.go:50-81` (`SemanticReasonProbeTimeout`
  etc.). Phase 65's `fallback_reason` follows this discipline; raw
  error text never leaks into the MCP envelope.
- **Bounded-label metrics with closed enum** — Phase 57 D-07 / Phase
  62 truth #21. Phase 65 emits e.g. `helix_mcp_tool_source_total{tool,
  source, fallback_reason}` with the new closed enum.
- **Sort-before-iterate doctrine** (Phase 62 CR-03) — deterministic
  result order for `RankFiles` / `RankFromSeeds`.
- **Golden-fixture preservation** — `internal/skill/repomap/`'s
  existing tests (e.g., `skill_integration_test.go`) lock the
  index-disabled output. Phase 65 must not move these — it adds
  `_semantic` variants of the goldens for the index-on path.

### Integration Points

- **`internal/daemon/daemon.go`** — Phase 65 adds (in order):
  - Construct the production `integSemanticLookup` adapter alongside
    the Phase 64 `semanticBundle`.
  - After kernel + skill init, call `repomapSkill.SetSemanticLookup(lookup)`
    and the kernel-symbols-side equivalent.
  - Widen `health.RegisterTools` signature (or pass an additional
    `SemanticIndexAccessor`) to surface the full SPEC §24.5 block.
- **`internal/skill/repomap/skill.go`** — adds one setter, one struct
  field, one consult-on-execute branch in `execGetRepoMap` /
  `execGetContext`; the tree-walk and rendering paths are unchanged.
- **`internal/kernel/symbols/`** — adds `skill_adapter.go` (new) and a
  small wrapper around the existing `AnalyzeBlastRadius` to layer the
  graph-first pre-pass; the LSP-only path stays as-is.
- **`internal/kernel/health/`** — extends `SemanticStoreProbe` (or
  introduces a wider `SemanticIndexAccessor`) to surface
  graph_version, overlay_active, pending_lsp_count,
  last_live_update_ms, last_error.

### Constraints

- Phase 65 must NOT touch middleware order
  (CLAUDE.md "Middleware Execution Order (LIFO)").
- Phase 65 must NOT modify `internal/repomap` engine source — INTEG-01
  contract. All wiring goes through `internal/skill/repomap/skill.go`
  setters.
- Phase 65 must NOT import `duckdb-go` outside
  `internal/semantic/store/` (Phase 57 D-12 + `cmd/vet-noduckdb/`).
- Phase 65 must NOT trigger snapshot writes from any lookup method —
  read+ tier; Phase 64 D-09/D-10 boundary preserved.
- Phase 65 must NOT regress `tools/list` profile filtering for the
  four tools — they are already visible in every profile (Phase 64
  D-14).
- Phase 65 must NOT auto-trigger background indexing on cold start —
  D-06 above; honors SPEC §7 "no blocking full index on normal tool
  use unless requested."
- Phase 65 must NOT proceed before Phase 59 EXTRACT-01..05 ships —
  D-10 above; production buildFn (Wave 0) requires per-language
  extractors.

</code_context>

<specifics>
## Specific Ideas

- **The setter pattern is the right shape because the codebase already
  has three precedents for it.** `SetEnrichFn` (LSP enrichment),
  `SetFallbackDeps` (LSP documentSymbol fallback), `SetMetricsSink`
  (per-extractor latency observation) all live on
  `internal/skill/repomap/skill.go`. `SetSemanticLookup` is the
  fourth in the family — exactly the kind of optional, post-init,
  daemon-wired dependency these setters serve.

- **Source enum stays at three values to match SPEC verbatim and to
  keep orthogonal signals orthogonal.** `source` answers "which code
  path ran"; `freshness` answers "how fresh is the data"; `confidence`
  (analyze_blast_radius) answers "how sure are we about this node".
  Adding `semantic_partial` would conflate path with quality.

- **Graph-first / LSP-validates-critical-edges respects what each
  signal does best.** The persisted graph is fast and broad; LSP is
  slow and precise. Walking the graph cheaply, then paying LSP only
  on the edges that actually matter, gives us the recall of the
  semantic graph without the latency of an LSP call per edge.

- **The fallback confidence cap of 0.6 is a contract, not an opinion.**
  ROADMAP success criterion #2 spells it out. A config knob would
  invite premature tuning; a higher cap would mute the strangler-fig's
  most important signal — that the agent is operating on degraded data.

- **Both Phase 64 carryovers belong in Phase 65 because they exist to
  enable Phase 65.** The empty-Facts placeholder buildFn was shipped
  knowing Phase 65 would replace it with the real pipeline. The
  zero-value WorkspaceKey adapter was shipped knowing the strangler-fig
  integration would need workspace-scoped retrieval. Splitting them
  into a Phase 64.5 micro-phase is bookkeeping churn.

- **Sequence shift to Phase 59 first is strictly better than a Go-only
  hack in Phase 65.** A Go-only extractor inside Phase 65 would
  duplicate logic Phase 59 will eventually own; cause confidence-ladder
  inconsistency between Go (Phase 65 inline) and TS/JS/Python (Phase 59
  proper); and create rework when Phase 59 ships. Stop, run Phase 59,
  return.

</specifics>

<deferred>
## Deferred Ideas

- **Per-tool narrow lookup interfaces (RepomapLookup, BlastRadiusLookup,
  HealthLookup).** Phase 65 ships one unified `SemanticLookup`
  interface; if it grows unwieldy, split later. Today the methods all
  share workspace + engine access patterns; one interface is fine.
- **Constructor-injected lookup via SkillDeps.** Setters work today
  and match the existing pattern. Constructor injection is a Phase
  65+ refactor if SkillDeps grows enough to warrant it.
- **`semantic_partial` as a fourth `source` enum value.** Quality
  signals stay on `freshness` and `confidence`. Add a value here
  only if Phase 67 eval shows agents need richer path-vs-quality
  signal in one field.
- **Background-index-on-cold-start.** Aggressive UX; today we honor
  the lazy-init doctrine. Reconsider if telemetry shows agents
  frequently calling `get_repo_map` before `index_semantic_graph`
  and getting confused by the fallback.
- **Synchronous index up to a budget on cold start.** Same
  reasoning — defer until eval proves it's needed.
- **Configurable fallback confidence ceiling
  (`semantic_index.fallback_confidence_ceiling`).** Today hard-coded
  at 0.6 per ROADMAP success criterion. If Phase 67 eval shows
  operators want to tune, add a config key then.
- **LSP-first / parallel layering for `analyze_blast_radius`.**
  Graph-first wins on cost; parallel wins on recall. Phase 67 eval
  is the natural place to compare.
- **Tools 5-10 from SPEC §23.5-23.10** (`explain_symbol_deep`,
  `find_related_symbols`, `get_cluster_map`, `explain_cluster`,
  `get_change_impact_graph`, `validate_graph_edge`) — deferred to
  v1.10.x.
- **Multi-projection PageRank in the strangler-fig output.** Phase
  62 ships only CALL_GRAPH / REFERENCE / FILE_DEPENDENCY; Phase 65
  surfaces only what Phase 62 ships. Multi-projection blending in
  the strangler-fig path is a future enhancement.
- **`get_health` `semantic_store` block rename to `semantic_index`.**
  Tempting for symmetry with SPEC §24.5 wording. Today we add fields
  in place to avoid breaking Phase 57 SC-1 consumers; rename is a
  v1.11 cleanup if anyone cares.
- **Cross-package profile semantics audit (carried from Phase 64
  deferred list).** Still worth doing in v1.11.

### Reviewed Todos (not folded)

None — `gsd-sdk query todo.match-phase 65` was not run as part of this
discussion (todo cross-reference deferred until Phase 65 actually
plans, after Phase 59 ships).

</deferred>

---

*Phase: 65-existing-tool-integration-strangler-fig*
*Context gathered: 2026-05-08*
*BLOCKED: Phase 59 (EXTRACT-01..05) must ship before Phase 65 planning resumes.*
