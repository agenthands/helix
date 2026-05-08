# Phase 65: Existing-Tool Integration (Strangler Fig) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-08
**Phase:** 65-existing-tool-integration-strangler-fig
**Areas discussed:** Lookup/injection seam shape, Fallback trigger taxonomy + source enum, analyze_blast_radius semantic + LSP layering, Carryover from Phase 64

---

## Lookup / injection seam shape

### Q1: How should the semantic lookup be injected into the four existing tools?

| Option | Description | Selected |
|--------|-------------|----------|
| Single `SemanticLookup` setter on each skill | Mirror SetEnrichFn / SetFallbackDeps pattern; one interface (RankFiles, RankFromSeeds, ExpandFrom, ValidateCriticalEdges, Status, Available) wired into repomap, kernel/symbols, and kernel/health. One mental model; matches Caddy-style post-init. | ✓ |
| Per-tool narrow interfaces | Each skill gets its own narrow interface; smaller surfaces, easier to fake. Cost: 3 wiring points, 3 adapters over the same engine. | |
| Constructor injection (no setter) | Receive lookup via SkillDeps. Cleanest deps graph but breaks Caddy-style init() pattern; SkillDeps grows; existing skills need constructor changes vs. one setter add. | |

**User's choice:** Single `SemanticLookup` setter on each skill (Recommended).

### Q2: Where should the SemanticLookup interface and its production adapter live?

| Option | Description | Selected |
|--------|-------------|----------|
| Interface in `internal/semantic/integ/`, adapter in `internal/daemon/` | New tiny package declares only the interface + value types; production adapter lives next to semanticBundle. Respects vet-nokernel2semantic boundary. | ✓ |
| Interface in `internal/skill/integ/` shared types package | Co-locate alongside skills. Risk: kernel/symbols is kernel-resident, not skill-resident; misleading name. | |
| Interface inline in `internal/semantic/` root | Co-locate with Bundle / RetrievalAccessor / etc. Risk: blurs the layer between accessor primitives and integration-seam interfaces. | |

**User's choice:** Interface in `internal/semantic/integ/`, adapter in `internal/daemon/` (Recommended).

---

## Fallback trigger taxonomy + source enum

### Q3: What closed-enum values should the `source` field carry on every semantic-aware tool envelope?

| Option | Description | Selected |
|--------|-------------|----------|
| Three values: semantic / tree_sitter / fallback | Match SPEC §24 / INTEG-05 verbatim. Quality signals stay on `freshness` and `confidence`. | ✓ |
| Four values: add `semantic_partial` | Richer path drift signal for callers that gate on quality. Cost: conflates path with quality; deviates from SPEC wording. | |
| Two values: semantic / fallback (collapse tree_sitter) | Simpler enum but loses the distinction between "index disabled by config" and "index enabled but unavailable". | |

**User's choice:** Three values: semantic / tree_sitter / fallback (Recommended).

### Q4: When semantic is enabled but no committed snapshot exists yet, what should `get_repo_map` / `get_context` do?

| Option | Description | Selected |
|--------|-------------|----------|
| Fall back to tree_sitter, emit `source: fallback` | Honor SPEC §7 "no blocking full index on normal tool use unless requested" + SPEC §25 lazy-init doctrine. Agents call index_semantic_graph explicitly. | ✓ |
| Trigger background index, serve fallback this call | Zero-config UX. Cost: surprise multi-second work; can mask config issues; first-call CPU spike invisible to operator. Conflicts with explicit lazy-init. | |
| Block synchronously up to a small budget (~500ms) | Best-effort fresh data. Cost: violates "no blocking full index"; budget tuning rabbit hole. | |

**User's choice:** Fall back to tree_sitter, emit `source: fallback` (Recommended).

---

## analyze_blast_radius semantic + LSP layering

### Q5: How should `analyze_blast_radius` layer the semantic graph and LSP when both are available?

| Option | Description | Selected |
|--------|-------------|----------|
| Graph-first expansion, LSP validates critical edges | Persisted graph expansion (cheap), then LSP validates only edges crossing public API or with confidence < 0.80. Matches SPEC §24.3 ordering. | ✓ |
| LSP-first core, graph adds transitive expansion | LSP precise core + graph extends. Cost: pay full LSP latency before anything; defeats persistent-graph-as-fast-path argument. | |
| Parallel both, union with conflict resolution | Highest recall. Cost: 2x LSP load even when graph would suffice; complex conflict resolution; harder determinism. | |

**User's choice:** Graph-first expansion, LSP validates critical edges (Recommended).

### Q6: How should the per-node `confidence` ceiling be set on the FALLBACK path?

| Option | Description | Selected |
|--------|-------------|----------|
| Hard cap ≤ 0.6 on fallback path | Honor ROADMAP success criterion #2 verbatim; envelope carries `fallback_reason`. Caller signal: confidence > 0.6 ⟺ graph-corroborated. | ✓ |
| Use Phase 62 confidence ladder unchanged on both paths | One mental model. Cost: deviates from ROADMAP SC; conflates LSP evidence with graph corroboration. | |
| Tunable ceiling via config | Operator opt-in to higher confidence. Cost: another knob agents tune blindly; SC is explicit; premature without eval data. | |

**User's choice:** Hard cap ≤ 0.6 on fallback path (Recommended).

---

## Carryover from Phase 64 — production buildFn + WorkspaceKey

### Q7: How should the two TODO(phase-65) carryover items be scoped within Phase 65?

| Option | Description | Selected |
|--------|-------------|----------|
| Both carryovers in Phase 65, planned as their own waves | (a) production buildFn = Wave 0 TDD; (b) WorkspaceKey adapter = Wave 0. Both unblock end-to-end strangler-fig. | ✓ |
| Production buildFn in 65, WorkspaceKey deferred to 66 | Smaller phase. Cost: get_semantic_context already in production ranks against repoRoot=""; erodes signal quality of Phase 65 output. | |
| Both carryovers split into pre-Phase-65 micro-phase | Each phase has single thesis. Cost: extra ROADMAP churn; carryovers exist to enable strangler-fig. | |

**User's choice:** Both carryovers in Phase 65, planned as their own waves (Recommended).

### Q8: EXTRACT-01 (Phase 59) is still pending. How do we handle this dependency?

| Option | Description | Selected |
|--------|-------------|----------|
| Ship Go-only minimal extractor inside Phase 65 buildFn | Wave 0 ships Go extraction only; Phase 59 ships TS/JS/Python later. De-risks Phase 65; doesn't block on Phase 59. | |
| Block Phase 65 on Phase 59 EXTRACT-01 completing first | Surface dependency; clean dependency graph; one source of truth for extraction. Cost: stalls strangler-fig until Phase 59 lands. | ✓ |
| Use tree-sitter raw queries inline in buildFn | No Phase 59 dep; works for any language with grammar. Cost: duplicates Phase 59 logic; inconsistent confidence; tech debt. | |

**User's choice:** Block Phase 65 on Phase 59 EXTRACT-01 completing first.

### Q9 (clarifying): Stop Phase 65 vs absorb Phase 59 vs reconsider?

| Option | Description | Selected |
|--------|-------------|----------|
| Stop Phase 65, run Phase 59 next, return to Phase 65 | Honor original ROADMAP sequence. Phase 59 runs as own discuss → plan → execute. Phase 65 resumes after. | ✓ |
| Fold Phase 59 EXTRACT-01..05 into Phase 65 as Wave -1 | Single phase delivers strangler-fig including extractors. Cost: phase scope balloons; mixes primitives with integration; messy traceability. | |
| Reconsider — Go-only minimal extractor | Re-pick option 1 from Q8. Cost: duplicates Phase 59 logic; potential rework. | |

**User's choice:** Stop Phase 65, run Phase 59 next, return to Phase 65 (Recommended).

**Notes:** This is the load-bearing decision of the discussion. The Phase 65 CONTEXT.md captures the decisions reached today; they remain valid until Phase 59 ships. STATE.md updated to reflect the sequence shift.

---

## Claude's Discretion

The following items were left to planner judgment, captured under
"Claude's Discretion" in CONTEXT.md:

- Exact Go signatures for the SemanticLookup interface (shape locked, signatures to be refined)
- Where the kernel-resident `analyze_blast_radius` skill adapter lives (recommend `internal/kernel/symbols/skill_adapter.go` per `health` / `help` precedent)
- Whether `Status` returns one struct or splits per concern
- `get_health` block name: extend `semantic_store` in place vs. add parallel `semantic_index` block (recommend in-place extension)
- Goldens fixture layout (reuse existing for index-disabled; new fixtures for semantic-on)
- Singleflight on lookup methods (only if planner judges concurrent identical calls warrant deduplication)

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` section. Highlights:

- Per-tool narrow lookup interfaces (split if unified gets unwieldy)
- Constructor-injected lookup via SkillDeps (Phase 65+ refactor)
- `semantic_partial` as fourth `source` enum value (only if Phase 67 eval shows need)
- Background-index-on-cold-start / synchronous-budget index (defer until telemetry justifies)
- Configurable fallback confidence ceiling (Phase 67 eval signal)
- LSP-first / parallel layering for analyze_blast_radius (Phase 67 eval comparison)
- Tools 5-10 from SPEC §23.5-23.10 (deferred to v1.10.x)
- Multi-projection PageRank in strangler-fig output (Phase 62 ships only 3 projections)
- `get_health` `semantic_store` → `semantic_index` rename (v1.11 cleanup)
- Cross-package profile semantics audit (carried from Phase 64 deferred list)
