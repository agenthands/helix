# Phase 65: Existing-Tool Integration (Strangler Fig) — Research

**Researched:** 2026-05-08
**Domain:** MCP-tool dependency-injection seam from existing v1.9 tools (`get_repo_map`, `get_context`, `analyze_blast_radius`, `get_health`) to Phase 60–64 semantic engine; closed-enum `source`/`fallback_reason` envelope contract; graph-first / LSP-validates-critical-edges layering.
**Confidence:** HIGH (every load-bearing claim is verified against the code tree as it stands at Phase 64-VERIFIED state, 2026-05-08).

---

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01: Single `SemanticLookup` interface, one setter per skill.** Mirror the existing `SetEnrichFn` / `SetFallbackDeps` / `SetMetricsSink` pattern at `internal/skill/repomap/skill.go:113-144`.
  - `RepoMapSkill.SetSemanticLookup(lookup)` — for `get_repo_map` and `get_context`.
  - A kernel-resident bridge (likely `internal/kernel/symbols/`) accepts the same lookup for `analyze_blast_radius` via the `skill_adapter.go` precedent (`internal/kernel/health/skill_adapter.go`, `internal/kernel/help/skill_adapter.go`).
  - `internal/kernel/health` already exposes `SemanticStoreProbe` (Phase 57 SC-1) — Phase 65 widens it (or adds a parallel `SemanticIndexAccessor`) to consume the same lookup.

- **D-02: Interface lives in `internal/semantic/integ/`; production adapter in `internal/daemon/semantic_wiring.go`.** New tiny package `internal/semantic/integ/` declares ONLY the `SemanticLookup` interface and value types (`RankedFile`, `Impact`, `Edge`, `SemanticStatus`, `FallbackReason`, `Source`).
  - **vet-noduckdb:** integ MUST NOT import duckdb-go.
  - **vet-nokernel2semantic:** kernel imports integ (interface only), NEVER `internal/semantic/store/` or `internal/semantic/retrieval/`.

  ⚠️ **Constraint conflict surfaced in research — see "Common Pitfalls / Pitfall 1" below.** The current `nokernel2semantic` analyzer (`internal/lint/nokernel2semantic/analyzer.go:34`) blocks ALL `internal/semantic/*` imports from kernel; planner must amend the analyzer with an `internal/semantic/integ/` allowlist OR relocate the integ package outside `internal/semantic/`.

- **D-03: Lookup interface shape (planner refines exact signatures).**
  ```go
  type SemanticLookup interface {
      Available() bool
      RankFiles(ctx, ws) ([]RankedFile, error)
      RankFromSeeds(ctx, ws, seeds []string) ([]RankedFile, error)
      ExpandFrom(ctx, ws, sym SymbolID, depth int) ([]Impact, error)
      ValidateCriticalEdges(ctx, ws, edges []Edge) ([]ValidatedEdge, error)
      Status(ctx, ws) (SemanticStatus, error)
  }
  ```
  All methods are read-only. Errors include sentinels `ErrNoSnapshot`, `ErrIndexBuilding`, `ErrIndexErrored`.

- **D-04: Closed-enum `source` field, three values: `semantic | tree_sitter | fallback`.** Matches SPEC §24 + INTEG-05. Data quality (pending LSP, stale scores) lives on `freshness`; per-node certainty on `confidence`. Three orthogonal signals.

- **D-05: `fallback_reason` closed enum on the envelope when `source=fallback`.** Values: `index_disabled` | `no_snapshot_yet` | `index_building` | `index_error` | `bleve_rebuilding`.

- **D-06: Cold start (semantic enabled, no committed snapshot) → `source=fallback`, NO background indexing kicked.** Honors SPEC §7 + §25 lazy mode. Agents wanting fresh ranking call `index_semantic_graph` explicitly.

- **D-07: Graph-first expansion, LSP validates critical edges (analyze_blast_radius two-pass).** Pass 1: `lookup.ExpandFrom(ctx, ws, sym, depth=2)`. Pass 2: filter critical edges (crosses public-API OR confidence < 0.80) → `lookup.ValidateCriticalEdges`. Validated edges → confidence flips to 1.00; refuted edges → confidence drops to 0.20, `refuted: true`. Reuses Phase 62 confidence ladder verbatim.

- **D-08: Fallback-path confidence cap ≤ 0.6.** Hard contract per ROADMAP success criterion #2. No config knob.

- **D-09: Both Phase 64 carryovers land as Phase 65 Wave 0 prerequisites.**
  - 65-Wave 0: production buildFn — replace empty-Facts at `internal/daemon/semantic_wiring.go:687-742` with real per-language extractor + classifier walk + overlay drain pipeline.
  - 65-Wave 0: WorkspaceKey adapter — replace zero-value return at `internal/daemon/semantic_wiring.go:509-525` with registry lookup.

- **D-10 cross-phase blocker is RESOLVED — Phase 59 EXTRACT-01..05 has shipped** (REQUIREMENTS.md lines 190-194 mark all five Complete; STATE.md shows Phase 59 COMPLETE). Phase 65 unblocked.

### Claude's Discretion

- Exact Go signatures for the `SemanticLookup` interface (D-03 is the contract; planner refines parameter ordering, context-vs-options struct splits, error sentinels).
- Where the kernel-resident `analyze_blast_radius` skill adapter lives (recommend `internal/kernel/symbols/skill_adapter.go` per CONTEXT.md hint).
- Whether `Status` returns one struct or splits per concern.
- `get_health` block name: `semantic_store` (extend) vs `semantic_index` (parallel block) — recommend extending in place; planner confirms after auditing Phase 57 SC-1 callers.
- Goldens fixture layout for `get_repo_map` / `get_context`.
- Singleflight on lookup methods — discretionary per Phase 64 D-02 precedent.

### Deferred Ideas (OUT OF SCOPE)

- Per-tool narrow lookup interfaces (RepomapLookup / BlastRadiusLookup / HealthLookup) — one unified interface for v1.10.
- Constructor-injected lookup via `SkillDeps` — setters ship today.
- `semantic_partial` as a fourth `source` enum value.
- Background-index-on-cold-start.
- Synchronous index up to a budget on cold start.
- Configurable fallback confidence ceiling.
- LSP-first / parallel layering for `analyze_blast_radius`.
- Tools 5-10 from SPEC §23.5-23.10.
- Multi-projection PageRank in the strangler-fig output.
- `get_health` block rename to `semantic_index` (additive evolution preferred).
- Cross-package profile semantics audit.

</user_constraints>

---

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **INTEG-01** | `get_repo_map` consults `repomapSkill.SetSemanticLookup(lookup)` when available; uses persisted graph scores + clusters; falls back on disabled/building/errored. Zero source change to `internal/repomap` engine. | Code Examples §"get_repo_map wiring"; Architecture Patterns §"Setter post-init wiring"; Phase 62 graph_version surface confirmed at `internal/semantic/store/overlay.go:983`; persisted-score read API confirmed via `QueryEffectiveAdjacency`. |
| **INTEG-02** | `get_context` delegates to semantic retrieval engine when available; same fallback contract. | Code Examples §"get_context wiring"; bleve `Engine.QueryBleve` signature at `internal/semantic/retrieval/bleve.go:127`. RankFromSeeds maps to `PersonalizedPageRank` — currently a stub at `internal/daemon/semantic_wiring.go:594-598` (Phase 65 production fill). |
| **INTEG-03** | `analyze_blast_radius` uses semantic graph expansion + LSP validation of critical edges; per-node `confidence` + `evidence`; falls back when semantic disabled. | Code Examples §"analyze_blast_radius two-pass"; existing pure-LSP path at `internal/kernel/symbols/blast.go:21` (`AnalyzeBlastRadius`) becomes Pass 2 primitive. Phase 62 confidence ladder D-12 (62-CONTEXT.md lines 273-284) is the source of confidence values. |
| **INTEG-04** | `get_health` includes a `semantic_index` section with store kind, latest snapshot status, graph_version, overlay_active, pending_lsp_count, last_live_update_ms, last_error. | Code Examples §"get_health extension"; existing SC-1 envelope at `internal/kernel/health/tools.go:155-172` is the additive base. Source-of-truth for each new field listed in Architecture Patterns §"get_health field-source map". |
| **INTEG-05** | Every MCP envelope from semantic-aware tools returns `source: semantic | tree_sitter | fallback`. Index-disabled goldens preserved. | Code Examples §"envelope contract"; closed-enum precedent at `internal/skill/semantic/envelope.go`; existing repomap goldens at `internal/skill/repomap/skill_integration_test.go` lines 141-170. |

</phase_requirements>

---

## Summary

Phase 65 is a **dependency-injection wiring phase**: it threads a single `SemanticLookup` seam (D-01/D-02/D-03) into four already-shipped MCP tools and stamps a closed-enum `source` field (D-04/D-05) on every envelope, with **zero source change to `internal/repomap` engine**. The four tools split across two layers: `get_repo_map` and `get_context` are skill-resident (`internal/skill/repomap/skill.go`, ExecuteTool returning `(string, error)`); `analyze_blast_radius` is kernel-resident (`internal/kernel/symbols/tools.go:560-585`, mcpsdk.AddTool with typed args); `get_health` is also kernel-resident with a daemon-injected probe seam (Phase 57 SC-1 at `internal/kernel/health/tools.go:139-181`). Each wiring style needs a different setter shape but the same `SemanticLookup` value.

The Wave 0 carryovers (CONTEXT.md D-09) are not optional — they are prerequisites for the strangler fig to deliver actual semantic ranking. Production buildFn (65-01) at `internal/daemon/semantic_wiring.go:687-742` ships with an empty-Facts placeholder today; the inline contract at lines 621-686 documents the exact pipeline shape, and `internal/semantic/extract/to_store.go` already exposes the locked `ToStoreFacts` adapter that bridges per-language `[]*ExtractedFile` to `semanticstore.Facts`. WorkspaceKey adapter (65-02) at `internal/daemon/semantic_wiring.go:509-525` returns a zero value today; the daemon's `activeWSKey` registry (`internal/daemon/daemon.go:521`) is the canonical translation source.

The most significant research finding — and the only thing that could surface as a planning-blocker — is the **`nokernel2semantic` analyzer collision** (Common Pitfalls §1). The analyzer at `internal/lint/nokernel2semantic/analyzer.go:34-57` is hard-coded with a no-allowlist substring match, so the kernel-side import of `internal/semantic/integ/` from CONTEXT.md D-02 will fail `make vet` as written. The fix is mechanical (add an allowlist constant or relocate the package), but the planner must include it as an explicit task — not absorb it into another plan.

The `analyze_blast_radius` two-pass algorithm (D-07) reuses Phase 62's confidence ladder verbatim (62-CONTEXT.md D-12, lines 273-284): 1.00 LSP / 0.95 merged / 0.80 ts+local / 0.70 ts-only / 0.60 doc-comment / 0.45 heuristic / 0.20 unknown. The "critical edge" filter is `crossesPublicAPI OR confidence < 0.80`, matching SPEC §24.3 ("LSP validation of critical edges"). The fallback-path confidence cap of 0.6 (D-08) is ROADMAP-mandated; planner does not reopen it.

**Primary recommendation:** Plan exactly the eight-task wave structure CONTEXT.md proposes (Wave 0: 65-01, 65-02; Wave 1: 65-03, 65-04; Wave 2: 65-05, 65-06, 65-07; Wave 3: 65-08), with one addition — a Wave 0 (or absorbed into 65-03) task that amends the `nokernel2semantic` analyzer allowlist before any kernel-side `internal/semantic/integ/` import lands.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| `SemanticLookup` interface declaration | Semantic value-types package (`internal/semantic/integ/`) | — | Types-only, no concrete behavior; importable from both daemon and kernel without dragging in store/retrieval. Locked by D-02. |
| `SemanticLookup` production adapter | Daemon (`internal/daemon/semantic_wiring.go`) | — | The adapter wraps `*semanticstore.Store` + `*retrieval.Engine` + `RankScheduler`; only the daemon assembles those concretions. Mirrors existing `semSessionAdapter` / `semRetrievalAdapter` precedent at `internal/daemon/semantic_wiring.go:498-619`. |
| `get_repo_map` / `get_context` semantic switch | Skill (`internal/skill/repomap/skill.go`) | Kernel (`internal/repomap/`) | Skill owns the dispatcher + setter; engine is untouched per INTEG-01 contract. New code lives in `execGetRepoMap` / `execGetContext` (lines 269-351). |
| `analyze_blast_radius` two-pass orchestration | Kernel (`internal/kernel/symbols/`) | Daemon (lookup wiring) | Pass 2 (LSP validation) is already kernel-resident at `internal/kernel/symbols/blast.go`. Pass 1 (graph expansion) calls into the lookup, so the orchestrator stays kernel-side and the lookup is daemon-injected via a kernel-side setter. New file: `internal/kernel/symbols/skill_adapter.go` (mirrors `internal/kernel/health/skill_adapter.go` pattern) + a small extension to `tools.go:560-585`. |
| `get_health` semantic_index extension | Kernel (`internal/kernel/health/`) | Daemon (lookup wiring) | The existing `RegisterTools` at line 139 already takes a daemon-injected `SemanticStoreProbe`; widening that signature (or adding a `SemanticIndexAccessor` parameter) keeps the kernel free of `internal/semantic` imports per `vet-nokernel2semantic`. |
| Closed-enum `source` / `fallback_reason` constants | Semantic value-types package (`internal/semantic/integ/`) | — | Same package as the interface. Both repomap-skill and kernel-symbols import these constants. |
| Source-of-truth for `graph_version` | Store (`internal/semantic/store/overlay.go:983` `CurrentGraphVersion`) | Phase 62 graph engine (`internal/semantic/graph/apply_repair.go` bumps it) | Read via existing `StoreAccessor.CurrentGraphVersion` — the Phase 64 narrow seam already exists. |
| Source-of-truth for overlay_active | Store (`internal/semantic/store/overlay.go:232` `OverlayHasPendingRows`) | — | Existing surface; Phase 64 already exposes it via `StoreAccessor`. |
| Source-of-truth for pending_lsp_count | LSP queue (`internal/semantic/lspenrich/`) via `QueueAccessor.DepthAll` | — | Existing Phase 64 narrow seam at `internal/skill/semantic/accessors.go:55-63`. |
| Source-of-truth for last_live_update_ms | Live (`internal/semantic/live/handler/`) via `LiveAccessor.LastFlushAt` | — | Existing Phase 64 narrow seam at `internal/skill/semantic/accessors.go:65-74`. |
| Source-of-truth for last_error | New: closed-enum classifier on the daemon-side adapter | — | Mirrors `classifySemanticProbeError` at `internal/kernel/health/tools.go:66-81`; Phase 65 adds the equivalent for retrieval / live / rank errors. |

---

## Standard Stack

### Core (already in tree — no new dependencies)

| Component | Source location | Purpose | Why Standard |
|-----------|-----------------|---------|--------------|
| MCP Go SDK | `github.com/modelcontextprotocol/go-sdk/mcp` | Tool registration via `mcpsdk.AddTool` for typed args | Already used by every kernel tool and the four Phase 64 semantic tools. |
| `internal/skill/repomap` | RepoMapSkill scaffold | Setter pattern host for `SetSemanticLookup` | Phase 53 D-15 setter precedent — `SetEnrichFn`, `SetFallbackDeps`, `SetMetricsSink` all sit in this file. |
| `internal/kernel/symbols/blast.go` | `AnalyzeBlastRadius` LSP primitive | Pass 2 of the two-pass algorithm | Already shipped; SYM-09 contract. |
| `internal/kernel/health/tools.go` | `RegisterTools(server, k, semProbe)` | Extension point for the `semantic_index` block | Phase 57 SC-1 already wires `SemanticStoreProbe`; widening or adding a parallel parameter is additive. |
| `internal/semantic/store` | `*Store` with effective-graph + epoch + graph_version surface | Read-side data source for the lookup adapter | Phase 64 P64-02 already exposes `LatestCommittedSnapshot`, `CurrentGraphVersion`, `OverlayHasPendingRows`, `QueryEffectiveAdjacency`. |
| `internal/semantic/retrieval` | `*Engine` with `QueryBleve`, `*Recoverer` with `RetrievalPending` | RankFromSeeds + retrieval freshness | Phase 64 P64-07 ship. |
| `internal/semantic/extract` | `Provider.Extract` + `Registry` + `ToStoreFacts` | Per-language extraction for the production buildFn (65-01) | Phase 59 ships per-language Go/TS/JS/Python providers; 2026-05-08 update D-06 promoted `ExtractionPipeline` interface specifically for the Phase 65 buildFn call site (`provider.go:32-49`). |
| `internal/semantic/live/classifier.go` | `ClassifyPathChange` | Classifier walk inside the production buildFn | Phase 60 LIVE-01 ship; signature stable. |
| `internal/semantic/graph` | `Engine.ApplyRepair` post-commit hook | graph_version bump observation surface | Phase 62; the Phase 65 lookup adapter only READS `CurrentGraphVersion` — it never calls `ApplyRepair`. |
| `internal/lint/nokernel2semantic` | go/analysis Analyzer | Enforces `internal/kernel/* MUST NOT import internal/semantic/*` | **Will need an allowlist constant for `internal/semantic/integ/` — see Pitfall §1.** |
| `internal/lint/noduckdb` | go/analysis Analyzer | Enforces STORE-06 (duckdb-go only inside `internal/semantic/store/`) | The new `internal/semantic/integ/` package is types-only and does NOT import duckdb-go; existing analyzer is fine as-is. |

### Supporting

| Component | Purpose | When to Use |
|-----------|---------|-------------|
| `golang.org/x/sync/singleflight.Group` | Dedupe concurrent identical calls into the lookup | Optional per CONTEXT.md "Claude's Discretion"; recommend wrapping `ExpandFrom`/`ValidateCriticalEdges` if the same caller-symbol gets queried from concurrent tool calls. Phase 64 D-02 precedent. |
| `golden-file fixture pattern` (existing repomap tests) | Lock index-disabled `get_repo_map`/`get_context` output | Reuse `internal/skill/repomap/skill_integration_test.go:141-170` (`TestGetRepoMap_WithWorkspace`, `TestGetContext_WithWorkspace`); add `_semantic` golden variants for index-on path. |
| `analysistest` | Unit tests for the amended `nokernel2semantic` analyzer | If Wave 0 task amends the analyzer, the existing `internal/lint/nokernel2semantic/analyzer_test.go` style is the template. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `internal/semantic/integ/` package location | Place interface under `internal/integ/` (top-level) | **Solves the `nokernel2semantic` collision without analyzer churn.** CONTEXT.md D-02 specifies `internal/semantic/integ/` — relocating is a planner decision if the analyzer amendment is rejected on review. Recommend amending the analyzer (smaller diff, keeps semantic boundary intact). |
| Extend `health.SemanticStoreProbe` in place | Add a parallel `health.SemanticIndexAccessor` interface | CONTEXT.md "Claude's Discretion" recommends extending in place. Existing JSON envelope is `{"semantic_store": {state, reason}}`; adding fields to that block keeps Phase 57 SC-1 consumers from breaking. Tests at `internal/kernel/health/tools_semantic_test.go:120-128` lock the JSON shape — Phase 65 must add new fields, not rename. |
| Migrate `get_repo_map`/`get_context` to typed-args registration | Keep `ExecuteTool(name, args) (string, error)` and wrap output as JSON | The current skill registration path returns a `string`. Adding the closed-enum `source` field requires the output to BE JSON. Two options: (a) JSON-wrap the existing string under a `"map"` field with siblings `"source"`, `"fallback_reason"`, `"graph_version"`, etc., breaking nothing because the result is still a TextContent block; (b) migrate to typed-args. Recommend (a) for minimal churn — the `ExecuteTool` return value flows through `internal/mcp/server.go:235-250` `AddSkillTool` to a TextContent block, so JSON-as-string is fine. |

**Installation:** No new packages.

**Version verification:** No new external dependencies. All Phase 65 code consumes already-vendored packages at versions already pinned by Phase 57–64.

---

## Architecture Patterns

### System Architecture Diagram

```
                        ┌──────────────────────────────────────────────────┐
   MCP request          │                tools/call dispatch               │
   ────────────────►    │  (LazyInit → Suggest → ProfileFilter → Telemetry)│
                        └──────────┬─────────────────────────┬─────────────┘
                                   │                         │
                  skill route (string)              kernel route (typed args)
                                   │                         │
              ┌────────────────────┴───┐         ┌───────────┴────────────┐
              │   RepoMapSkill         │         │   kernel/symbols       │
              │   .ExecuteTool         │         │   registerAnalyzeBlast │
              │   ┌───────────────┐    │         │     ┌─────────────┐    │
              │   │ get_repo_map  │    │         │     │  Pass 1:    │    │
              │   │ get_context   │    │         │     │  ExpandFrom │    │
              │   └───────┬───────┘    │         │     └──────┬──────┘    │
              └───────────┼────────────┘         │            │           │
                          │                      │     ┌──────▼─────────┐ │
                          │                      │     │  Pass 2:       │ │
                          │  s.semanticLookup    │     │  ValidateEdges │ │
                          │  (atomic.Pointer)    │     │  (LSP, kernel) │ │
                          │                      │     └──────┬─────────┘ │
                          │                      │            │           │
                          │                      └────────────┼───────────┘
                          │                                   │
                          │                                   │
                          │   if Available()                  │
                          ▼                                   ▼
              ┌─────────────────────────────────────────────────────────┐
              │  internal/semantic/integ.SemanticLookup (interface)      │
              │  Available / RankFiles / RankFromSeeds / ExpandFrom /    │
              │  ValidateCriticalEdges / Status                          │
              └────────────────────────┬────────────────────────────────┘
                                       │ production adapter
                                       ▼
              ┌─────────────────────────────────────────────────────────┐
              │  internal/daemon/semantic_wiring.go                      │
              │  integSemanticLookup{store, retrieval, scheduler, ws}    │
              └────┬─────────────────┬──────────────┬───────────────────┘
                   │                 │              │
            ┌──────▼──────┐  ┌───────▼──────┐  ┌───▼──────────┐
            │ *Store      │  │ *Engine      │  │ RankScheduler│
            │ (snapshot+  │  │ (bleve scorch│  │ (Phase 62    │
            │  overlay)   │  │  + RRF)      │  │  PageRank)   │
            └─────────────┘  └──────────────┘  └──────────────┘

   On Available()==false  →  envelope.source = tree_sitter | fallback
                              + fallback_reason (closed enum)
                              + (analyze_blast_radius) confidence ≤ 0.6
                              + v1.9 path runs unchanged

   ─────────────────────────────────────────────────────────────────────

   get_health envelope (extended in place):
              kernel/health.RegisterTools(server, k, probe, indexAccessor?)
                      │
                      ├──→ ComputeSemanticStoreStatus → semantic_store.{state,reason}
                      └──→ NEW: semantic_index.{store, latest_snapshot_status,
                                graph_version, overlay_active,
                                pending_lsp_revalidations,
                                last_live_update_ms, last_error}
```

The diagram emphasizes three separate routes through `SemanticLookup`: skill-side (`RepoMapSkill`), kernel-side (`analyze_blast_radius`), and health-side (`get_health` consuming `Status` for the new `semantic_index` block). The lookup is a single interface; the wiring is three distinct setters because the consumers live in three packages.

### Recommended Project Structure

```
internal/
├── semantic/
│   └── integ/                   # NEW (D-02) — types-only package, no duckdb-go, no kernel/ imports
│       ├── doc.go               # Package contract + closed-enum doctrine
│       ├── lookup.go            # SemanticLookup interface + error sentinels
│       ├── source.go            # Source closed enum + FallbackReason closed enum + tests
│       └── status.go            # SemanticStatus + RankedFile + Impact + Edge + ValidatedEdge
├── daemon/
│   └── semantic_wiring.go       # MODIFIED — add integSemanticLookup adapter alongside existing semStoreAdapter et al.
│                                #            replace makeProductionBuildFn empty-Facts (65-01)
│                                #            replace semSessionAdapter.Workspace zero-value (65-02)
│                                #            wire SetSemanticLookup on RepoMapSkill + kernel-symbols-side adapter
├── skill/
│   └── repomap/
│       └── skill.go             # MODIFIED — add SetSemanticLookup setter; lookup-consult branches in execGetRepoMap / execGetContext;
│                                #            envelope-wrap output (string → JSON containing source/fallback_reason/graph_version + existing tree text)
├── kernel/
│   ├── symbols/
│   │   ├── skill_adapter.go     # NEW (CONTEXT.md "Claude's Discretion") — kernel-resident skill_adapter.go mirroring health/help precedent
│   │   ├── blast_semantic.go    # NEW — two-pass orchestrator wrapping existing AnalyzeBlastRadius
│   │   └── tools.go             # MODIFIED — registerAnalyzeBlastRadius accepts the lookup; threads it into blast_semantic.go
│   └── health/
│       └── tools.go             # MODIFIED — widen RegisterTools signature OR add SemanticIndexAccessor parameter
└── lint/
    └── nokernel2semantic/
        └── analyzer.go          # MODIFIED — add ALLOWLIST const + check (or planner relocates integ package)

internal/skill/repomap/
├── skill_integration_test.go    # MODIFIED — existing TestGetRepoMap_WithWorkspace / TestGetContext_WithWorkspace serve as the index-disabled goldens (preserved per INTEG-01); new _semantic counterparts added
└── skill_test.go                # MODIFIED — wider source/fallback_reason matrix tests
```

### Pattern 1: Setter post-init wiring (for cross-package optional deps)

**What:** A skill or kernel package exposes a `SetX(...)` method that the daemon calls after construction. The skill defaults to a "lookup not wired" path (`s.semanticLookup == nil` → fall back to v1.9 behavior) so tests and degraded modes work without daemon involvement.

**When to use:** Every Phase 65 cross-package dependency. Mirrors the four existing precedents at `internal/skill/repomap/skill.go`:

- `SetEnrichFn(fn func(graph *repomap.FileGraph))` — line 115
- `SetFallbackDeps(deps *FallbackDeps)` — line 123
- `SetRegistry(registry *treesitter.GrammarRegistry)` — line 162
- `SetMetricsSink(sink repomap.MetricsSink)` — line 137

**Example (skill side, lock-protected, atomic-pointer-safe):**

```go
// SetSemanticLookup wires the optional semantic-index lookup. nil-safe
// argument is normalized to a no-op stub so the dispatcher's emission
// sites never need nil-checks. Mirrors SetMetricsSink pattern at
// internal/skill/repomap/skill.go:137.
//
// Source: internal/skill/repomap/skill.go:137 (existing pattern)
func (s *RepoMapSkill) SetSemanticLookup(lookup integ.SemanticLookup) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.semanticLookup = lookup
}

func (s *RepoMapSkill) lookup() integ.SemanticLookup {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.semanticLookup == nil {
        return integ.NoopLookup{} // Available()==false; never errors
    }
    return s.semanticLookup
}
```

### Pattern 2: Daemon-side adapter alongside `semanticBundle` (production lookup)

**What:** The daemon owns the only place where `*semanticstore.Store`, `*retrieval.Engine`, and `RankScheduler` are concretely available. The `integSemanticLookup` adapter wraps those concretions and presents the `SemanticLookup` interface to consumers.

**When to use:** Exactly Phase 65's `internal/daemon/semantic_wiring.go` — joins the existing family of adapters (`semStoreAdapter`, `semSchedulerAdapter`, `semQueueAdapter`, `semLiveAdapter`, `semCompactorAdapter`, `semSessionAdapter`, `semRetrievalAdapter`).

**Example (production adapter sketch, uses existing surfaces):**

```go
// integSemanticLookup adapts the daemon's semanticBundle to the
// integ.SemanticLookup interface. Read-only — never triggers snapshot
// writes (read+ tier per Phase 64 D-09 / Phase 65 D-03).
//
// Source: internal/daemon/semantic_wiring.go:530-619 (semRetrievalAdapter precedent)
type integSemanticLookup struct {
    bundle    *semanticBundle
    store     *semanticstore.Store
    retrieval *semRetrievalAdapter
    rank      *rankBundle
    enabledFn func() bool // closes over cfg.SemanticIndex.Enabled
}

func (l *integSemanticLookup) Available() bool {
    if l == nil || !l.enabledFn() {
        return false
    }
    return l.store != nil && l.store.Available()
}

func (l *integSemanticLookup) Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error) {
    if !l.Available() {
        return integ.SemanticStatus{State: integ.StatusDisabled}, nil
    }
    repoID := ws.Hash() // see Pattern 3 below
    snap, _ := l.store.LatestCommittedSnapshot(ctx, repoID)
    gv, _ := l.store.CurrentGraphVersion(ctx, repoID)
    overlay := l.store.OverlayHasPendingRows(repoID)
    return integ.SemanticStatus{
        State:               integ.StatusReady, // closed enum
        Store:               "duckdb",
        LatestSnapshotID:    snap,
        GraphVersion:        gv,
        OverlayActive:       overlay,
        PendingLSP:          l.bundle.queue.DepthAll(ws), // existing
        LastLiveUpdateMs:    l.bundle.live.LastFlushAt(ws),
        LastErrorReason:     l.bundle.lastErrorReason(),  // closed enum
    }, nil
}
```

### Pattern 3: Workspace-key resolution from `*mcp.SessionInfo` (65-02)

**What:** The current `semSessionAdapter.Workspace(ctx)` at `internal/daemon/semantic_wiring.go:509-525` returns a zero-value `workspace.WorkspaceKey`. The daemon's authoritative workspace key lives in the `activeWSKey` variable captured by the `SetActivateCallback` closure (`internal/daemon/daemon.go:521`, `694-754`).

**When to use:** Wave 0 / 65-02 — replace the zero return with a registry lookup. Two options:

(a) Capture a `func() workspace.WorkspaceKey` closure (the existing `wsKeyFn` at `internal/daemon/daemon.go:523`) and pass it through `newSemanticBundle` constructor. **Recommend.** Mirrors how `wsKeyFn` is already passed to `symbols.RegisterTools` (`internal/daemon/daemon.go:526`) and `edit.RegisterTools` (line 527).

(b) Look up `*mcp.SessionInfo.WorkspaceKey` (a hashed string per `internal/mcp/session.go:29`) and reverse it through a registry. **Reject.** The hash is `sha256.Sum256(...)[:8]` (see `internal/workspace/key.go:22-25`); reversal requires storing every observed `WorkspaceKey` — needless complexity.

**Example (option (a), simplest closure pass-through):**

```go
// Replace the empty Workspace() at semantic_wiring.go:509-525.
//
// Source: internal/daemon/daemon.go:523 (existing wsKeyFn closure)
type semSessionAdapter struct {
    getSession func(ctx context.Context) *mcp.SessionInfo
    wsKeyFn    func() workspace.WorkspaceKey // NEW (65-02)
}

func (a *semSessionAdapter) Workspace(ctx context.Context) workspace.WorkspaceKey {
    if a == nil || a.wsKeyFn == nil {
        return workspace.WorkspaceKey{}
    }
    return a.wsKeyFn() // returns activeWSKey from daemon.go:521
}
```

The closure is per-daemon (single active workspace per daemon today); when multi-workspace lands the closure resolves session→workspace via a real registry. CONTEXT.md keeps this scope bounded.

### Pattern 4: Production buildFn pipeline (65-01)

**What:** Replace the empty-`Facts` commit at `internal/daemon/semantic_wiring.go:687-742` with the documented inline pipeline. The full contract is at lines 621-686 of the same file — the planner copies the inline plan directly into a Plan markdown.

**When to use:** Wave 0 / 65-01 — mode=full and mode=incremental.

**Pipeline shape (verified against existing surfaces, NOT speculative):**

```go
func (b *semanticBundle) makeProductionBuildFn() semantic.RunnerBuildFn {
    return func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st semantic.BuildState) (semantic.IndexResult, error) {
        repoID := ws.Hash()
        startedAt := time.Now()

        // 1. Resolve base snapshot for incremental mode.
        var baseSnapshotID uint64
        if mode == "incremental" {
            latest, err := b.store.LatestCommittedSnapshot(ctx, repoID)
            if err != nil {
                return semantic.IndexResult{}, fmt.Errorf("buildFn: LatestCommittedSnapshot: %w", err)
            }
            baseSnapshotID = latest
        }

        // 2. Walk the workspace OR drain overlay per mode.
        //    - mode=full: filepath.WalkDir from ws.RepoRoot, classify each
        //      via live.ClassifyPathChange (semantic_wiring.go:621-639 pipeline doc).
        //    - mode=incremental: drain b.live.OverlayPending(ws) (Phase 60 surface).
        var extracted []*extract.ExtractedFile
        for path := range walkOrDrain(b, ws, mode) {
            kind, ok, err := live.ClassifyPathChange(ctx, semantic.RepoID(repoID), path, b.store, nil, live.ChangeSourceWatcher)
            if err != nil || !ok || kind == live.ChangeFileDeleted {
                continue // skipped per classifier rules
            }
            lang := langFromExt(path) // existing helper in repomap or extract
            provider, ok := b.extractRegistry.Provider(lang)
            if !ok {
                continue // language not first-class; skip (Phase 59 emits partial:true)
            }
            source, err := os.ReadFile(path)
            if err != nil {
                continue
            }
            ef, err := provider.Extract(ctx, source, extract.SourceFile{Path: path, Language: lang})
            if err != nil {
                continue
            }
            extracted = append(extracted, ef)
        }

        // 3. Convert per-language facts to store wire format (locked adapter).
        //    Source: internal/semantic/extract/to_store.go:58 (Phase 65 unblock adapter, D-08).
        facts := extract.ToStoreFacts(extracted)

        // 4. Snapshot lifecycle (verbatim contract from semantic_wiring.go:643-662).
        snap, err := b.store.BeginSnapshot(ctx, semanticstore.SnapshotMeta{
            RepoID:         repoID,
            BaseSnapshotID: baseSnapshotID,
        })
        if err != nil {
            return semantic.IndexResult{}, fmt.Errorf("buildFn: BeginSnapshot: %w", err)
        }
        committed := false
        defer func() {
            if !committed {
                _ = b.store.AbortSnapshot(context.Background(), snap, "buildFn: not committed")
            }
        }()
        st.SetSnapshotID(snap.ID)

        if err := b.store.WriteSnapshotFacts(ctx, snap, facts); err != nil {
            return semantic.IndexResult{}, fmt.Errorf("buildFn: WriteSnapshotFacts: %w", err)
        }

        summary := semanticstore.SnapshotSummary{DurationMs: time.Since(startedAt).Milliseconds()}
        if err := b.store.CommitSnapshot(ctx, snap, summary); err != nil {
            return semantic.IndexResult{}, fmt.Errorf("buildFn: CommitSnapshot: %w", err)
        }
        committed = true

        // 5. Post-commit ApplyRepair fires automatically via Phase 60 hook
        //    (live.handler → rank.engine wired in daemon.go:372). No explicit
        //    OnNewSnapshot call required — semantic_wiring.go:664-666.
        return semantic.IndexResult{
            SnapshotID:   snap.ID,
            FilesIndexed: int64(len(extracted)),
            FilesReused:  0,
            DurationMs:   time.Since(startedAt).Milliseconds(),
        }, nil
    }
}
```

**Verification:** Every signature above is checked against:
- `extract.Provider.Extract`: `internal/semantic/extract/provider.go:48`
- `extract.ToStoreFacts`: `internal/semantic/extract/to_store.go:58`
- `live.ClassifyPathChange`: `internal/semantic/live/classifier.go:51`
- `*Store.LatestCommittedSnapshot`: `internal/semantic/store/effective_graph.go:208`
- `*Store.BeginSnapshot/WriteSnapshotFacts/CommitSnapshot/AbortSnapshot`: contract documented inline at `internal/daemon/semantic_wiring.go:668-674`.

**Acceptable error / partial-coverage behavior:** per-file extraction errors are non-fatal (continue the walk, log at debug). Per-file `ExtractedFile.Partial=true` is preserved through `ToStoreFacts` (verified — see the file-disposition contract at `to_store.go:18-37`). Total extraction failure (zero `extracted` files) still commits an empty snapshot — that matches the Phase 64 behavior and the rank/retrieval engines tolerate it (snapshot.go:309-407 documents the empty-Facts no-op path on `WriteSnapshotFacts`).

### Pattern 5: Closed-enum source/fallback_reason envelope

**What:** `source` ∈ `{semantic, tree_sitter, fallback}` and `fallback_reason` ∈ `{index_disabled, no_snapshot_yet, index_building, index_error, bleve_rebuilding}` (D-04, D-05). The integ package owns these constants. Every consumer of the lookup checks `Available()` then maps lookup errors to `fallback_reason` values via a mini-classifier.

**When to use:** Every envelope from the four tools.

**Example (mini-classifier mirrors `classifySemanticProbeError` at `internal/kernel/health/tools.go:66-81`):**

```go
// classifyLookupErr maps a lookup error onto the closed FallbackReason enum.
//
// Source: internal/kernel/health/tools.go:66-81 (existing closed-enum precedent)
func classifyLookupErr(err error, lookup integ.SemanticLookup) integ.FallbackReason {
    if !lookup.Available() {
        return integ.FallbackReasonIndexDisabled
    }
    if errors.Is(err, integ.ErrNoSnapshot) {
        return integ.FallbackReasonNoSnapshotYet
    }
    if errors.Is(err, integ.ErrIndexBuilding) {
        return integ.FallbackReasonIndexBuilding
    }
    if errors.Is(err, integ.ErrBleveRebuilding) {
        return integ.FallbackReasonBleveRebuilding
    }
    return integ.FallbackReasonIndexError
}
```

### Anti-Patterns to Avoid

- **Auto-trigger background indexing on cold start.** Forbidden by D-06 + SPEC §7. The lookup MUST return `ErrNoSnapshot` and let the caller stamp `source=fallback, fallback_reason=no_snapshot_yet`. Watch for any code path that calls `index_semantic_graph` from inside a lookup method.
- **Letting raw error text escape into the envelope.** WR-NEW-01 doctrine — every `fallback_reason` value MUST be a closed-enum constant, never a `fmt.Sprintf("...%v", err)`. Operators get raw text via `slog.Warn`; agents get the enum.
- **Calling `BeginSnapshot` / `WriteSnapshotFacts` / `CommitSnapshot` / `OnFlush` / `BumpGraphVersion` from any lookup method.** Read+ tier per D-03; Phase 64 D-09 invariant. The `vet`-grep canary at `internal/skill/semantic/tools_refresh.go` (Phase 64 P64-05) is the precedent — reuse the same grep approach for Phase 65 lookup methods.
- **Adding the `source` field as an inline prefix string in the existing repomap text output.** That breaks structured parsing and makes goldens fragile. JSON-wrap the existing string into an envelope (CONTEXT.md "Claude's Discretion" / Alternatives Considered).
- **Reordering MCP middleware.** CLAUDE.md "Middleware Execution Order (LIFO)" invariant. Phase 65 adds NO new middleware; profile/mode gating stays exactly as Phase 64 left it.
- **Importing `internal/semantic/store/` or `internal/semantic/retrieval/` from `internal/kernel/`.** Kernel imports `internal/semantic/integ/` only (interface package). The `vet-nokernel2semantic` analyzer enforces this — see Pitfall §1 for the analyzer amendment requirement.
- **Importing `duckdb-go` from `internal/semantic/integ/`.** The integ package is types-only — no store concretions. `vet-noduckdb` enforces this.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Workspace activation registry | A new daemon-side registry | The existing `activeWSKey` closure in `internal/daemon/daemon.go:521` + `wsKeyFn` at line 523 | Already canonical; multi-workspace expansion is a future-multi-workspace problem (deferred per `internal/daemon/semantic_wiring.go:559`). |
| Per-language fact extraction | A Phase 65 inline extractor | `extract.Provider.Extract` + `extract.ToStoreFacts` (`internal/semantic/extract/`) | Phase 59 ships per-language Go/TS/JS/Python providers with stable IDs; ToStoreFacts is the locked Phase 65 unblock adapter. |
| File classification (created/modified/deleted/skipped) | A re-implementation in the buildFn | `live.ClassifyPathChange(ctx, repoID, path, store, hasher, source)` | Phase 60 owns the single source of classification kind. Symlink rejection (T-60-04-03) is already there. |
| Closed-enum reason classifier | An ad-hoc switch on error message strings | The `classifySemanticProbeError` precedent at `internal/kernel/health/tools.go:66-81` | WR-NEW-01 doctrine; raw error text NEVER leaks. |
| PageRank scoring | An inline call to `internal/repomap.RankFiles` for the semantic path | Read persisted scores via the lookup's `RankFiles` (which reads `semantic_graph_scores` rows persisted by Phase 62 RankScheduler) | Phase 62 already computed and persisted scores; recomputing in the hot path is wrong. The fallback path uses `internal/repomap.FileGraph.RankFiles` unchanged (INTEG-01 contract). |
| LSP-side blast radius (Pass 2) | A new LSP wrapper | Existing `kernel/symbols.AnalyzeBlastRadius` at `internal/kernel/symbols/blast.go:21` | Already shipped; SYM-09 contract; Pass 2 is just "call this on the critical-edge subset". |
| Bleve-backed text retrieval | A second text engine | Existing `*retrieval.Engine.QueryBleve` at `internal/semantic/retrieval/bleve.go:127` | Phase 64 P64-07 ships it with weighted RRF + dual-store recovery. |
| Snapshot status / overlay state surface | A new accessor | `StoreAccessor` interface at `internal/skill/semantic/accessors.go:18-33` (`LatestCommittedSnapshot`, `CurrentGraphVersion`, `OverlayHasPendingRows`, `QueryEffectiveAdjacency`) | All four existing accessors expose exactly the SPEC §24.5 fields the `get_health` extension needs. |
| Pending-LSP count / last-flush surface | A new accessor | `QueueAccessor.DepthAll` + `LiveAccessor.LastFlushAt` (existing in `internal/skill/semantic/accessors.go:55-74`) | Phase 64 already exposes both. |

**Key insight:** The strangler-fig is mostly composition — every concrete capability the four tools need is already shipped by Phase 57–64. Phase 65's value is the wiring, the closed-enum envelope, and the two-pass orchestrator. Hand-rolling any of the building blocks duplicates work and breaks the read+ doctrine.

---

## Runtime State Inventory

(Phase 65 is a wiring + envelope phase; no rename/refactor. Skipped per researcher checklist.)

---

## Common Pitfalls

### Pitfall 1: `nokernel2semantic` analyzer blocks `internal/semantic/integ/` import from kernel

**What goes wrong:** CONTEXT.md D-02 specifies that the kernel-resident `analyze_blast_radius` skill_adapter imports `internal/semantic/integ/` (the new types-only package). The current `nokernel2semantic` analyzer at `internal/lint/nokernel2semantic/analyzer.go:34-57` uses a hard-coded substring match:

```go
const checkedPkgPrefix = "github.com/agenthands/helix/internal/kernel"
const forbiddenImportPrefix = "github.com/agenthands/helix/internal/semantic"
// ... if strings.HasPrefix(path, forbiddenImportPrefix) { ... Reportf(...) ... }
```

There is **no allowlist**. The analyzer will fail `make vet` and the `TestAnalyzer_RealTreeIsClean` integration test as soon as the kernel-side adapter compiles.

**Why it happens:** The analyzer was written for Phase 60 LIVE-07 invariant #1, before Phase 65's strangler-fig integration was a concrete plan. The substring check is correct for "no kernel-to-store/retrieval/graph" but too coarse for "no kernel-to-anything-under-semantic-except-the-types-only-integ-package".

**How to avoid:** Two acceptable resolutions; planner picks one.

(a) **Amend the analyzer (recommended).** Add an allowlist constant:

```go
const allowedSemanticIntegPath = "github.com/agenthands/helix/internal/semantic/integ"
// ... if strings.HasPrefix(path, forbiddenImportPrefix) && !strings.HasPrefix(path, allowedSemanticIntegPath) { ... }
```

Plus an `analysistest` fixture under `testdata/src/kernel-imports-integ/` confirming the allowlisted import passes. Diff is small (~10 lines) and keeps the architectural intent ("kernel can import the types-only seam, never the concretions").

(b) **Relocate the integ package.** Place the interface under `internal/integ/semantic/` (a top-level integ package). No analyzer change needed. CONTEXT.md D-02 names `internal/semantic/integ/` as the location; relocating is a planner decision worth at least flagging.

**Warning signs:** `go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./internal/kernel/...` reports `internal/kernel/* must not import internal/semantic/* (got import "github.com/agenthands/helix/internal/semantic/integ" in github.com/agenthands/helix/internal/kernel/symbols)`. CI step at `.github/workflows/go-test.yml` may not run `make vet` (the realtree integration test build-tag-gates `integration`); local `make test` will surface it.

**Decision support:** Recommend (a). The integ package is conceptually part of the semantic boundary, just at the seam — moving it out of `internal/semantic/` muddies the "everything semantic lives under one root" mental model.

### Pitfall 2: `get_repo_map` / `get_context` skill output is a `string`, not a struct

**What goes wrong:** INTEG-05 requires every envelope to carry `source` + (when `source=fallback`) `fallback_reason`. But `RepoMapSkill.ExecuteTool` returns `(string, error)` (`internal/skill/repomap/skill.go:257-265`), and `internal/mcp/server.go:235-250 AddSkillTool` wraps that string into a TextContent block as-is. There is no struct envelope today.

**Why it happens:** The repomap skill predates Phase 64's typed-args registration pattern. The Phase 64 four tools use `mcpsdk.AddTool` directly (`internal/skill/semantic/register.go`), bypassing the legacy `ExecuteTool` route.

**How to avoid:** JSON-wrap the existing string output. The dispatcher returns `string` containing JSON like:

```json
{
  "source": "semantic",
  "graph_version": 184,
  "freshness": "fresh",
  "tree": "<existing budgeted tree text>"
}
```

Or on fallback:

```json
{
  "source": "fallback",
  "fallback_reason": "no_snapshot_yet",
  "tree": "<existing budgeted tree text>"
}
```

Index-disabled goldens at `internal/skill/repomap/skill_integration_test.go:141-170` will need updating to expect the JSON-wrapped form — this is a **golden change**. CONTEXT.md "index-disabled goldens preserved verbatim" (D-04) means the **tree content is preserved verbatim**, not the wire format.

**Warning signs:** Existing goldens locked to a plain-text tree start failing with `unexpected character` if a downstream test parses them as plain text. Plan a single mechanical golden-update task as part of 65-04.

**Alternative:** Migrate `get_repo_map` / `get_context` to typed-args (`mcpsdk.AddTool[GetRepoMapArgs, GetRepoMapResult]`). Larger diff, risks breaking the `TestRepoMapSkill_*` test family at `internal/skill/repomap/skill_test.go`. **Recommend the JSON-wrap approach.**

### Pitfall 3: `SemanticIndex.Enabled=false` defensively classified as `index_disabled` (fallback) when CONTEXT.md says it should be `tree_sitter`

**What goes wrong:** D-04 says: when `semantic_index.enabled=false`, the v1.9 path runs and the envelope says `source=tree_sitter`. But the obvious classifier — "Available() returned false → fallback + fallback_reason=index_disabled" — produces `source=fallback`, not `source=tree_sitter`. Operators reading the envelope cannot distinguish "config disabled" from "config enabled but errored."

**Why it happens:** D-04 is the locked semantics; D-05 lists `index_disabled` as a "defensive" `fallback_reason` because in a correctly-implemented Phase 65, the source=disabled path takes the `tree_sitter` envelope value, not `fallback`.

**How to avoid:** The classifier checks the config gate FIRST, BEFORE checking lookup availability:

```go
func chooseSource(cfg *config.Config, lookup integ.SemanticLookup, err error) (integ.Source, integ.FallbackReason) {
    if !cfg.SemanticIndex.Enabled {
        return integ.SourceTreeSitter, "" // by config
    }
    if !lookup.Available() || err != nil {
        return integ.SourceFallback, classifyLookupErr(err, lookup)
    }
    return integ.SourceSemantic, ""
}
```

The `index_disabled` `fallback_reason` value remains in the closed enum as a defensive value for any race between config-toggle and lookup-construction; in steady state, it should never appear.

**Warning signs:** Index-disabled integration test asserts `source=fallback` instead of `source=tree_sitter`. The goldens will catch this immediately if the test-matrix in 65-08 includes it.

### Pitfall 4: Phase 57 SC-1 envelope shape regression on `get_health` extension

**What goes wrong:** `internal/kernel/health/tools_semantic_test.go:120-128` asserts the JSON shape `"semantic_store":{"state":"ready"}`. Phase 65 extending the `semantic_store` block (vs. adding a parallel `semantic_index` block) must keep that field name and embed the new fields at the same level OR a sub-level — but if the planner picks "rename to `semantic_index`", the existing test fails and downstream consumers (operators, dashboards) break.

**Why it happens:** SPEC §24.5 names the section `semantic_index`; the Phase 57 SC-1 envelope is `semantic_store`. Choosing between SPEC fidelity and backward compat is a one-line difference in the marshalled envelope.

**How to avoid:** Keep the existing `semantic_store: {state, reason}` block and ADD the new fields at the same level OR add a sibling `semantic_index` block. CONTEXT.md "Claude's Discretion" recommends the former (extend in place); recommend the latter only if the planner finds the field set diverges enough that one block becomes confusing. Whatever the planner picks, the existing JSON-shape test must continue to pass — Phase 65 is additive evolution.

**Warning signs:** `TestSemanticStoreStatus_JSONShape` fails. CI surfaces it; local `go test ./internal/kernel/health/...` catches it.

### Pitfall 5: Singleflight on lookup methods masks the fallback path

**What goes wrong:** If the planner wraps `lookup.ExpandFrom` with `singleflight.Group`, and the first concurrent caller errors with `ErrIndexBuilding`, all joined callers get the same error — but they may be racing with a snapshot commit, so the second caller could have succeeded if it had run independently.

**Why it happens:** Singleflight short-circuits identical concurrent calls onto a single goroutine. Phase 64 D-02 uses this for `index_semantic_graph` to dedupe expensive build calls — that's correct because the build is genuinely shared work. For lookups, the dedup is less valuable and the joined-error semantics confuse the fallback envelope.

**How to avoid:** **Skip singleflight on lookup methods.** Lookups are stateless reads against the shared engines; concurrent calls produce independent results. CONTEXT.md "Claude's Discretion" leaves this open; the recommendation here is to NOT wrap. If a future profiling shows hot-path duplication on `ExpandFrom`, revisit.

**Warning signs:** Two concurrent `analyze_blast_radius` calls on the same symbol both get `source=fallback, fallback_reason=index_building` even though the snapshot completed mid-second-call.

### Pitfall 6: Confidence flip on refuted edges leaks into Pass 1 output

**What goes wrong:** D-07 says refuted edges drop to confidence 0.20 with `refuted: true`. If the orchestrator mutates the Pass 1 result in-place when applying Pass 2's verdict, but then calls Pass 1 again for a sibling tool call, the in-place mutation persists across calls.

**Why it happens:** Easy mistake when the Pass 1 result is a slice the lookup returned by reference. The lookup may be reusing internal buffers.

**How to avoid:** Always copy the Pass 1 result before applying Pass 2 verdicts. The integ.Impact slice is small (depth=2 means dozens, not thousands).

**Warning signs:** Two sequential `analyze_blast_radius` calls on the same symbol return different confidence values for unchanged edges.

---

## Code Examples

### Example 1: get_repo_map wired to the lookup with semantic-on / fallback envelope

```go
// Source: derives from internal/skill/repomap/skill.go:269-287 (existing execGetRepoMap)
//         + internal/skill/semantic/tools_status.go (closed-enum freshness selection precedent)

func (s *RepoMapSkill) execGetRepoMap(args map[string]interface{}) (string, error) {
    budget := extractTokenBudget(args, defaultRepoMapBudget)
    ctx := context.Background() // or thread through a typed-args migration

    lookup := s.lookup() // returns NoopLookup{} when not wired

    // 1. Choose source per D-04/D-06.
    var (
        source         integ.Source
        fallbackReason integ.FallbackReason
        graphVersion   uint64
        tree           string
    )

    if !lookup.Available() {
        // semantic_index disabled by config → source=tree_sitter, run v1.9 unchanged.
        source = integ.SourceTreeSitter
        if err := s.ensureGraph(); err != nil {
            return "", serr.Wrap(serr.Internal, "building reference graph", err).WithTool("get_repo_map")
        }
        ranked := s.graph.RankFiles(0.85, nil)
        tree = s.renderer.RenderBudgeted(ranked, budget)
    } else {
        ws := s.workspaceKey() // see Pattern 3 — closure threaded through Init
        ranked, err := lookup.RankFiles(ctx, ws)
        if err != nil {
            // Lookup error → source=fallback + closed-enum reason; v1.9 path runs.
            source = integ.SourceFallback
            fallbackReason = classifyLookupErr(err, lookup)
            if perr := s.ensureGraph(); perr != nil {
                return "", serr.Wrap(serr.Internal, "building reference graph", perr).WithTool("get_repo_map")
            }
            ranked := s.graph.RankFiles(0.85, nil)
            tree = s.renderer.RenderBudgeted(ranked, budget)
        } else {
            // Semantic-on path. Persisted scores from Phase 62 PageRank.
            source = integ.SourceSemantic
            status, _ := lookup.Status(ctx, ws)
            graphVersion = status.GraphVersion
            // Render the ranked output through the EXISTING tree renderer
            // (no engine change). RankedFile maps onto the existing
            // s.renderer expected shape via a small adapter.
            tree = s.renderer.RenderBudgeted(adaptRankedFiles(ranked), budget)
        }
    }

    // 2. JSON-wrap the envelope (Pitfall §2).
    return marshalEnvelope(integ.RepoMapEnvelope{
        Source:         source,
        FallbackReason: fallbackReason, // empty when source != fallback
        GraphVersion:   graphVersion,   // 0 when source != semantic
        Freshness:      computeFreshness(lookup, ctx),
        Tree:           tree,
    }), nil
}
```

### Example 2: analyze_blast_radius two-pass (D-07)

```go
// Source: extends internal/kernel/symbols/tools.go:560-585 (existing pure-LSP path).
//         New file: internal/kernel/symbols/blast_semantic.go.

func registerAnalyzeBlastRadius(server *mcp.SerenaMCPServer, k *kernel.Kernel,
    wsKeyFn func() workspace.WorkspaceKey, lookupFn func() integ.SemanticLookup,
    tracer trace.Tracer,
) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "analyze_blast_radius",
        Description: "Analyze the blast radius (impact) of changing a symbol",
    }, kernel.WrapToolSpan(tracer, "analyze_blast_radius",
        func(ctx context.Context, req *mcpsdk.CallToolRequest, args BlastRadiusArgs) (*mcpsdk.CallToolResult, any, error) {
            // ... existing arg validation + lease acquisition ...
            lookup := lookupFn()
            if lookup == nil || !lookup.Available() {
                // Fallback path: pure LSP with confidence cap ≤ 0.6 (D-08).
                br, err := AnalyzeBlastRadius(ctx, lease, uri, lspLine, lspCol)
                if err != nil {
                    return errorResult(err.Error()), nil, nil
                }
                return textResult(formatBlastRadiusEnvelope(br, integ.SourceFallback,
                    integ.FallbackReasonIndexDisabled, /*confidenceCap=*/ 0.6)), nil, nil
            }

            // Pass 1: graph expansion.
            ws := wsKeyFn()
            sym := symbolIDFromArgs(args) // helper; SymbolID type per integ
            impacts, err := lookup.ExpandFrom(ctx, ws, sym, /*depth=*/ 2)
            if err != nil {
                br, lspErr := AnalyzeBlastRadius(ctx, lease, uri, lspLine, lspCol)
                if lspErr != nil {
                    return errorResult(lspErr.Error()), nil, nil
                }
                return textResult(formatBlastRadiusEnvelope(br, integ.SourceFallback,
                    classifyLookupErr(err, lookup), 0.6)), nil, nil
            }

            // Pass 2: filter critical edges, validate via LSP.
            critical := filterCritical(impacts) // crossesPublicAPI || confidence < 0.80
            validated, vErr := lookup.ValidateCriticalEdges(ctx, ws, edgesOf(critical))
            if vErr == nil {
                applyValidationVerdicts(impacts, validated) // flip 1.00 / 0.20 + refuted:true
            }
            // (vErr non-nil: leave Pass 1 confidences untouched; LSP unavailable
            // is non-fatal — log at debug.)

            return textResult(formatBlastRadiusEnvelopeFromImpacts(impacts, integ.SourceSemantic, "", 0.0 /* no cap */)), nil, nil
        }))
    server.Registry().Register(&mcp.ToolDef{Name: "analyze_blast_radius", /* ... */})
}
```

### Example 3: get_health extension (additive, in-place)

```go
// Source: extends internal/kernel/health/tools.go:139-181 (existing SC-1 path).

func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel,
    semProbe SemanticStoreProbe, semIndex SemanticIndexAccessor, // NEW arg (D-01 widening)
) {
    // ... existing tracer + AddTool prelude ...
    semStatus := ComputeSemanticStoreStatus(ctx, semProbe)

    // NEW: extension fields per SPEC §24.5.
    semIdx := ComputeSemanticIndexBlock(ctx, semIndex) // uses SemanticIndexAccessor.Status
    envelope := struct {
        *lspool.HealthReport
        SemanticStore SemanticStoreStatus `json:"semantic_store"`
        SemanticIndex SemanticIndexBlock  `json:"semantic_index,omitempty"` // only present when accessor wired
    }{
        HealthReport:  report,
        SemanticStore: semStatus,
        SemanticIndex: semIdx,
    }
    // ... existing JSON marshal + textResult ...
}

// SemanticIndexBlock mirrors SPEC §24.5 verbatim. Closed-enum status states.
type SemanticIndexBlock struct {
    Enabled                  bool   `json:"enabled"`
    Store                    string `json:"store"`                          // "duckdb" today
    LatestSnapshotStatus     string `json:"latest_snapshot_status"`         // "ready" | "building" | "missing"
    GraphVersion             uint64 `json:"graph_version"`
    OverlayActive            bool   `json:"overlay_active"`
    PendingLSPRevalidations  int    `json:"pending_lsp_revalidations"`
    LastLiveUpdateMs         int64  `json:"last_live_update_ms"`
    LastError                string `json:"last_error,omitempty"`           // closed-enum reason; never raw
}
```

The `SemanticIndexAccessor` interface lives in `internal/kernel/health/tools.go` next to `SemanticStoreProbe`. The daemon-side adapter wraps `integ.SemanticLookup.Status()` — kernel never imports `internal/semantic/store/` or `internal/semantic/retrieval/`.

### Example 4: integ package skeleton (D-02)

```go
// Path: internal/semantic/integ/lookup.go
// vet-noduckdb: this package MUST NOT import duckdb-go.
// vet-nokernel2semantic: kernel imports THIS package only (interface + value types).
package integ

import (
    "context"
    "errors"

    "github.com/agenthands/helix/internal/workspace"
)

// SemanticLookup is the read+ seam from existing MCP tools to the Phase 60-64
// semantic engine. All methods are read-only — they NEVER trigger snapshot
// writes (Phase 64 D-09 doctrine).
type SemanticLookup interface {
    Available() bool
    RankFiles(ctx context.Context, ws workspace.WorkspaceKey) ([]RankedFile, error)
    RankFromSeeds(ctx context.Context, ws workspace.WorkspaceKey, seeds []string) ([]RankedFile, error)
    ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID, depth int) ([]Impact, error)
    ValidateCriticalEdges(ctx context.Context, ws workspace.WorkspaceKey, edges []Edge) ([]ValidatedEdge, error)
    Status(ctx context.Context, ws workspace.WorkspaceKey) (SemanticStatus, error)
}

// Closed enum: source.
type Source string

const (
    SourceSemantic   Source = "semantic"
    SourceTreeSitter Source = "tree_sitter"
    SourceFallback   Source = "fallback"
)

// Closed enum: fallback_reason (only meaningful when Source==SourceFallback).
type FallbackReason string

const (
    FallbackReasonIndexDisabled    FallbackReason = "index_disabled"     // defensive (steady state should be SourceTreeSitter)
    FallbackReasonNoSnapshotYet    FallbackReason = "no_snapshot_yet"
    FallbackReasonIndexBuilding    FallbackReason = "index_building"
    FallbackReasonIndexError       FallbackReason = "index_error"
    FallbackReasonBleveRebuilding  FallbackReason = "bleve_rebuilding"
)

// Error sentinels for the closed-enum classifier.
var (
    ErrNoSnapshot      = errors.New("integ: no committed snapshot yet")
    ErrIndexBuilding   = errors.New("integ: index build in progress")
    ErrIndexErrored    = errors.New("integ: index in error state")
    ErrBleveRebuilding = errors.New("integ: bleve segment rebuilding")
)

// NoopLookup is the default lookup before SetSemanticLookup wires the production
// adapter. Available() returns false; all other methods return ErrIndexErrored.
type NoopLookup struct{}

func (NoopLookup) Available() bool { return false }
// ... other methods return ErrIndexErrored ...
```

---

## State of the Art

(Phase 65 is internal-tool wiring; "state of the art" is Phase 64 D-09 read+ doctrine and Phase 62 confidence ladder. Both are project-internal and locked.)

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `get_repo_map` returns plain text | JSON envelope wrapping the same text + closed-enum `source`/`fallback_reason`/`graph_version` | Phase 65 | Index-disabled goldens need a one-time mechanical update from plain-text to JSON-wrapped (Pitfall §2). |
| `analyze_blast_radius` is pure LSP | Two-pass: Phase 62 graph expansion → LSP validation of critical edges | Phase 65 D-07 | Existing `AnalyzeBlastRadius` becomes the Pass 2 primitive; fallback path uses it unchanged with confidence cap ≤ 0.6. |
| `get_health` carries `semantic_store: {state, reason}` only | Adds full `semantic_index` block per SPEC §24.5 | Phase 65 INTEG-04 | Additive — Phase 57 SC-1 consumers see the existing `semantic_store` block unchanged. |
| Lookup-side workspace resolution returns zero-value | Closure-based registry pass-through (Pattern 3) | Phase 65 65-02 | Multi-workspace expansion deferred (single-workspace daemon today). |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `RepoMapSkill.ExecuteTool` JSON-wrap approach won't break MCP-client-side parsing of `get_repo_map` / `get_context` results. | Pitfall §2 | LOW — MCP TextContent is opaque to the SDK; consumers parse it themselves. If a downstream client hard-coded plain-text parsing, that client breaks. The four agent profiles ship without such parsers. [ASSUMED — based on knowledge of existing MCP client behavior in Claude Code / Codex / OpenCode; not verified by reading every client.] |
| A2 | `*Store.OverlayHasPendingRows(repoID)` and `*Store.CurrentGraphVersion(ctx, repoID)` reads do not block on the overlay write mutex. | Pattern 2 | LOW — `OverlayHasPendingRows` documented at `internal/semantic/store/overlay.go:223-232` as in-memory atomic read; `CurrentGraphVersion` at line 983 reads from a per-workspace counter. [ASSUMED — read-side concurrency invariants are stated in Phase 60 D-04 contract; not stress-tested for Phase 65 read+ tier specifically.] |
| A3 | The JSON-shape regression test at `internal/kernel/health/tools_semantic_test.go:120-128` uses substring matching that tolerates added fields. | Pitfall §4 | LOW — the test asserts `strings.Contains(...)` for the existing fields; adding fields is additive. **VERIFIED** by reading the test code. |
| A4 | Phase 62 RankScheduler persists per-projection PageRank scores with `graph_version` stamping that the lookup can read directly without recomputation. | Don't Hand-Roll | MEDIUM — Phase 62 D-07 + D-10 specify this contract; the `semantic_graph_scores` table is keyed `(repo_id, graph_version, node_id, projection)` per `internal/semantic/store/effective_graph.go:22`. **VERIFIED** by inspecting the comment header. The actual read API the lookup will use (`RankFiles` ↔ score-row select) is not yet exposed as a public method on `*Store`; Phase 65 may need to add a thin reader. Flag for planner. |
| A5 | Bleve recovery state (`*retrieval.Recoverer.RetrievalPending`) maps cleanly to `fallback_reason=bleve_rebuilding`. | Pattern 5 | LOW — Phase 64 already exposes this via `RetrievalAccessor.RetrievalPending` (`internal/skill/semantic/accessors.go:104-107`); reusing it for the closed-enum mapping is mechanical. **VERIFIED.** |
| A6 | The amended `nokernel2semantic` analyzer's allowlist will not introduce unintended escape hatches (e.g., a future `internal/semantic/integ/store/` import slipping through `strings.HasPrefix` because `integ` is its prefix). | Pitfall §1 fix (a) | LOW — exact-match the allowed prefix at `internal/semantic/integ` AND require the import to be either equal to that prefix or have a slash boundary after it. Test it with `analysistest`. [ASSUMED — recommend the planner adds an exact-or-slash-boundary check, not just a HasPrefix.] |

---

## Open Questions

1. **How does `RankedFile` map onto `internal/repomap.FileGraph.RankFiles` output for the existing `TreeRenderer`?**
   - What we know: `TreeRenderer.RenderBudgeted` at `internal/skill/repomap/skill.go:281` accepts the v1.9 ranked-file shape produced by `internal/repomap.FileGraph.RankFiles`.
   - What's unclear: whether the integ-package `RankedFile` (from persisted Phase 62 scores) needs an explicit adapter or whether the renderer can accept a duck-typed input.
   - Recommendation: Plan a small adapter (`adaptRankedFiles`) in `internal/skill/repomap/skill.go` that converts integ-shape to renderer-shape. Keep it tiny — INTEG-01 forbids touching the engine, and the renderer is engine-resident.

2. **Does the `*Store` expose a public `RankFiles`/`RankFromSeeds` reader for the Phase 65 lookup, or does Phase 65 need to add one?**
   - What we know: Phase 62 persists scores under `semantic_graph_scores`; `*Store.QueryEffectiveAdjacency` is exposed; rank engine internals are NOT publicly readable.
   - What's unclear: whether the Phase 65 lookup adapter at `integSemanticLookup.RankFiles` reads scores via a (yet-to-be-named) public method on `*Store` OR via `*RankScheduler` directly.
   - Recommendation: Plan a small `*Store.QueryRankedFiles(ctx, repoID, projection, limit) ([]RankedFile, error)` in the same wave as 65-03, OR thread the existing `*RankScheduler` / `rankBundle.QueryScores(...)` (if it exists) through the production adapter. Researcher could not verify which without reading more of `internal/graph/`/`internal/semantic/graph/` than the time budget allowed; planner confirms during Wave 1.

3. **`analyze_blast_radius` — what's the canonical `SymbolID` translation between LSP file:line:col args and the integ-side stable symbol identity?**
   - What we know: SPEC §11.1 defines stable symbol IDs (LSP identity → package/module path + qualified name + kind + signature hash → file path fallback). Phase 59 EXTRACT-02 ships this contract.
   - What's unclear: the Phase 65 kernel-side adapter receives `(line, col, path)` from `BlastRadiusArgs` and must translate to a `SymbolID` to pass to `lookup.ExpandFrom`. The translation likely lives in extract or store, but the kernel can't import either (`vet-nokernel2semantic`).
   - Recommendation: Add a `SymbolID(ctx, ws, path, line, col) (SymbolID, error)` method to the integ `SemanticLookup` interface, OR translate inside the daemon-side adapter and surface the `SymbolID` through the lookup bridge. **Flag for planner — this is a missing piece in CONTEXT.md D-03 that will surface in 65-06.**

4. **Should the production adapter cache `Status()` between calls or compute every time?**
   - What we know: `Status()` is called from `get_health` (low-frequency), but also potentially from the per-call source classifier on every `get_repo_map`/`get_context` to compute `freshness`.
   - What's unclear: whether the per-call cost (a few atomic reads + a `sql` query for `LatestCommittedSnapshot`) is hot enough to warrant caching.
   - Recommendation: No cache for v1.10. Profile in Phase 67 eval; revisit in v1.10.x if a hot-path emerges.

---

## Environment Availability

(Phase 65 is internal-tool wiring with no external dependencies beyond Go and the already-vendored package set. Skipped per researcher checklist.)

---

## Validation Architecture

> nyquist_validation is enabled in `.planning/config.json` (workflow.nyquist_validation: true). This section is REQUIRED.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + `testify` (already vendored at `internal/skill/repomap/skill_integration_test.go:10-11`) |
| Config file | `go.mod` (no separate config) |
| Quick run command | `go test ./internal/skill/repomap/... ./internal/kernel/health/... ./internal/kernel/symbols/... ./internal/semantic/integ/... -count=1 -race` |
| Full suite command | `go test ./... -count=1 -race` (run by `make test`) |
| Vet command | `make vet` (runs the singlecheckers including the amended `nokernel2semantic`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| INTEG-01 | `get_repo_map` semantic-on returns ranked output sourced from persisted graph scores | integration | `go test ./internal/skill/repomap/ -run "TestGetRepoMap_Semantic" -v -count=1` | ❌ Wave 1 (65-04 / 65-05) |
| INTEG-01 | `get_repo_map` index-disabled goldens preserved | integration (golden) | `go test ./internal/skill/repomap/ -run "TestGetRepoMap_WithWorkspace" -v -count=1` | ✅ exists; needs JSON-wrap update |
| INTEG-02 | `get_context` semantic-on delegates to retrieval engine | integration | `go test ./internal/skill/repomap/ -run "TestGetContext_Semantic" -v -count=1` | ❌ Wave 1 (65-04 / 65-05) |
| INTEG-02 | `get_context` index-disabled goldens preserved | integration (golden) | `go test ./internal/skill/repomap/ -run "TestGetContext_WithWorkspace" -v -count=1` | ✅ exists; needs JSON-wrap update |
| INTEG-03 | `analyze_blast_radius` semantic-on returns confidence + evidence | integration | `go test ./internal/kernel/symbols/ -run "TestAnalyzeBlastRadius_Semantic" -v -count=1` | ❌ Wave 2 (65-06) |
| INTEG-03 | `analyze_blast_radius` fallback caps confidence ≤ 0.6 | unit | `go test ./internal/kernel/symbols/ -run "TestAnalyzeBlastRadius_FallbackConfidenceCap" -v -count=1` | ❌ Wave 2 (65-06) |
| INTEG-03 | Pass 2 LSP validation flips confidence on critical edges | unit | `go test ./internal/kernel/symbols/ -run "TestAnalyzeBlastRadius_ValidateCriticalEdges" -v -count=1` | ❌ Wave 2 (65-06) |
| INTEG-04 | `get_health` includes `semantic_index` block with all SPEC §24.5 fields | integration | `go test ./internal/kernel/health/ -run "TestGetHealth_SemanticIndexBlock" -v -count=1` | ❌ Wave 2 (65-07) |
| INTEG-04 | Phase 57 SC-1 envelope shape preserved (existing `semantic_store` block additive) | unit | `go test ./internal/kernel/health/ -run "TestSemanticStoreStatus_JSONShape" -v -count=1` | ✅ exists |
| INTEG-05 | Closed-enum `source` field on every envelope | unit | `go test ./internal/semantic/integ/ -run "TestSource_ClosedEnum" -v -count=1` | ❌ Wave 1 (65-03) |
| INTEG-05 | Closed-enum `fallback_reason` field on every fallback envelope | unit | `go test ./internal/semantic/integ/ -run "TestFallbackReason_ClosedEnum" -v -count=1` | ❌ Wave 1 (65-03) |
| INTEG-05 | Index-disabled (`tree_sitter`) source classification distinct from `fallback` | integration | `go test ./internal/skill/repomap/ -run "TestEnvelope_IndexDisabledIsTreeSitter" -v -count=1` | ❌ Wave 3 (65-08) |
| 65-Wave 0 | Production buildFn writes real Facts (not empty) | integration | `go test ./internal/skill/semantic/ -run "TestE2E_IndexThenContext_SymbolCount" -v -count=1` | ✅ exists; will pass with non-empty count once 65-01 ships |
| 65-Wave 0 | WorkspaceKey adapter returns non-zero key after activation | unit | `go test ./internal/daemon/ -run "TestSemSessionAdapter_WorkspaceResolved" -v -count=1` | ❌ Wave 0 (65-02) |
| Architecture | `internal/kernel/* MUST NOT import internal/semantic/store/`, `internal/semantic/retrieval/`, `internal/semantic/graph/` | vet | `make vet` (runs `vet-nokernel2semantic` with allowlist for `integ/`) | ✅ vet binary exists; analyzer needs amendment (Pitfall §1) |
| Architecture | `internal/semantic/integ/` MUST NOT import duckdb-go | vet | `make vet` (runs `vet-noduckdb`) | ✅ analyzer exists; covers the new path |

### Sampling Rate

- **Per task commit:** `go test ./<changed-package>/... -count=1 -race` + `go vet ./...`
- **Per wave merge:** `go test ./internal/skill/repomap/... ./internal/kernel/... ./internal/semantic/integ/... ./internal/daemon/... ./internal/lint/nokernel2semantic/... -count=1 -race` + `make vet`
- **Phase gate:** `make test` (full suite green) + `make vet` (all singlecheckers pass) before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/semantic/integ/lookup_test.go` — closed-enum `Source` and `FallbackReason` exhaustive tests (covers INTEG-05 unit layer)
- [ ] `internal/semantic/integ/source_test.go` — verify enum strings round-trip via JSON (matches WR-NEW-01 doctrine)
- [ ] `internal/lint/nokernel2semantic/testdata/src/kernel-imports-integ-allowed/main.go` — fixture for the allowlist check
- [ ] `internal/lint/nokernel2semantic/testdata/src/kernel-imports-store-still-forbidden/main.go` — fixture verifying the broader forbiddance is preserved
- [ ] `internal/daemon/semantic_wiring_buildfn_test.go` — production buildFn integration test with a small Go-only fixture (3 files, ~15 symbols, mirrors `TestE2E_IndexThenContext_SymbolCount` shape but exercises the real provider path)
- [ ] `internal/daemon/semantic_wiring_workspace_test.go` — WorkspaceKey adapter test asserting non-zero return after `SetActivateCallback` fires
- [ ] No new framework install needed — `testify` and `go test` are already used.

### {source × fallback_reason} Test Matrix (Wave 3 / 65-08)

The acceptance closure plan must exercise every `(source, fallback_reason)` pair across all four tools. Matrix:

| # | Source | Fallback Reason | Tool | Setup | Expected envelope shape |
|---|--------|-----------------|------|-------|-------------------------|
| 1 | `semantic` | `""` | get_repo_map | semantic enabled, snapshot committed, lookup ranks files | `{source:"semantic", graph_version:>0, freshness:"fresh", tree:"..."}` |
| 2 | `semantic` | `""` | get_context | as #1 + seeds | `{source:"semantic", graph_version:>0, freshness:"fresh", ranked_symbols:[...]}` |
| 3 | `semantic` | `""` | analyze_blast_radius | semantic enabled + symbol exists + Pass 2 validates critical edges | `{source:"semantic", confidence per node ∈ [0.20, 1.00], evidence:[...]}` |
| 4 | `semantic` | `""` | get_health | semantic enabled, snapshot committed | `{semantic_store:{state:"ready"}, semantic_index:{enabled:true, store:"duckdb", latest_snapshot_status:"ready", graph_version:>0, ...}}` |
| 5 | `tree_sitter` | `""` | get_repo_map | `semantic_index.enabled=false` (config) | `{source:"tree_sitter", tree:"..."}` (existing golden text preserved verbatim) |
| 6 | `tree_sitter` | `""` | get_context | as #5 + seeds | `{source:"tree_sitter", ...existing golden...}` |
| 7 | `tree_sitter` | `""` | analyze_blast_radius | as #5 | `{source:"tree_sitter", per_node:[{confidence:≤0.6, lsp_validated:true}, ...]}` |
| 8 | `tree_sitter` | `""` | get_health | as #5 | `{semantic_store:{state:"disabled"}}` (no `semantic_index` block OR `semantic_index:{enabled:false}`) |
| 9 | `fallback` | `no_snapshot_yet` | get_repo_map | semantic enabled, no snapshot committed (cold start), lookup returns ErrNoSnapshot | `{source:"fallback", fallback_reason:"no_snapshot_yet", tree:"..."}` |
| 10 | `fallback` | `no_snapshot_yet` | get_context | as #9 + seeds | `{source:"fallback", fallback_reason:"no_snapshot_yet", ...}` |
| 11 | `fallback` | `no_snapshot_yet` | analyze_blast_radius | as #9 | `{source:"fallback", fallback_reason:"no_snapshot_yet", per_node:[{confidence:≤0.6, lsp_validated:true}]}` |
| 12 | `fallback` | `index_building` | get_repo_map | concurrent index_semantic_graph in flight; lookup returns ErrIndexBuilding | `{source:"fallback", fallback_reason:"index_building", tree:"..."}` |
| 13 | `fallback` | `index_building` | get_context | as #12 | `{source:"fallback", fallback_reason:"index_building", ...}` |
| 14 | `fallback` | `index_building` | analyze_blast_radius | as #12 | `{source:"fallback", fallback_reason:"index_building", per_node:[{confidence:≤0.6, ...}]}` |
| 15 | `fallback` | `index_error` | get_repo_map | snapshot exists but corrupted; lookup returns ErrIndexErrored | `{source:"fallback", fallback_reason:"index_error", tree:"..."}` |
| 16 | `fallback` | `index_error` | get_health | as #15 | `{semantic_index:{last_error:"index_error", ...}}` (closed enum, NEVER raw text) |
| 17 | `fallback` | `bleve_rebuilding` | get_context | bleve segment recovering; `RetrievalPending=true` | `{source:"fallback", fallback_reason:"bleve_rebuilding", ...}` |
| 18 | `fallback` | `bleve_rebuilding` | get_health | as #17 | `{semantic_index:{last_error:"bleve_rebuilding", ...}}` |
| 19 | `fallback` | `index_disabled` (defensive) | get_repo_map | edge case: `semantic_index.enabled=true` at boot but lookup constructed before `Available()` flipped — should NOT happen in steady state but the enum value is reserved for it | `{source:"fallback", fallback_reason:"index_disabled", tree:"..."}` |

**Coverage targets:**
- Every (source, fallback_reason) cell in the matrix has at least one test.
- Each of the four tools has at least one row covering each `source` value.
- The 5×3 enumeration of `fallback_reason` × {get_repo_map, get_context, analyze_blast_radius} (15 cells) is exercised at least once.
- `get_health` covers each `last_error` closed-enum value via the same setup as the other three tools.

**Test harness reuse:** mirror the `newE2EHarness` pattern at `internal/skill/semantic/integration_test.go:438-440` (15-symbol fixture) — same harness shape, four tools, parameterized by `(source, fallback_reason)` setup function.

---

## Security Domain

> security_enforcement is not explicitly disabled in `.planning/config.json`. This section is REQUIRED.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | no | MCP transport handles session identity; Phase 65 adds nothing. |
| V3 Session Management | no | Existing `*mcp.SessionInfo` flows unchanged. |
| V4 Access Control | yes | SPEC §30.2 mode gating: all four tools are read+ — enforced inside each tool, not in middleware. Phase 65 must NOT bypass mode checks (verified: existing `mode_check.go` pattern in `internal/skill/semantic/` is untouched by Phase 65; `analyze_blast_radius` lives in kernel where mode is checked at the lease-acquisition boundary). |
| V5 Input Validation | yes | `get_context` already validates `files` for path traversal at `internal/skill/repomap/skill.go:312-321` (T-28-05) — Phase 65 preserves that path verbatim and applies the same validation when seeds are passed to `RankFromSeeds`. |
| V6 Cryptography | no | Phase 65 adds no crypto operations; `WorkspaceKey.Hash()` is sha256 from existing code. |

### Known Threat Patterns for Helix Phase 65

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via `get_context` files / lookup seeds | Tampering | Existing `..` rejection + `filepath.IsAbs` outside-root check at `skill.go:312-321`. Phase 65 propagates the same validation to `RankFromSeeds` seeds before calling the lookup. |
| Path drift between v1.9 path and semantic path (M7 mitigation) | Tampering | The closed-enum `source` field IS the mitigation — operators detect drift by golden-diffing semantic-on vs index-disabled outputs. The golden matrix in Validation Architecture §"{source × fallback_reason} Test Matrix" enforces this. |
| Background indexing kicked on cold start (DoS via tool call triggering full reindex) | DoS | D-06 forbids it. Lookup MUST return `ErrNoSnapshot` and let the caller stamp `source=fallback`. Test #9 in the matrix locks this — assert no `index_semantic_graph` invocation during the cold-start `get_repo_map` call. |
| Snapshot writes leaking through lookup methods (read+ tier breach) | EoP | The `vet`-grep canary used in Phase 64 P64-05 (`grep -E "(BeginSnapshot\|CommitSnapshot\|AbortSnapshot\|WriteSnapshotFacts\|OnFlush\|BumpGraphVersion)" internal/semantic/integ/`) must run clean for Phase 65 too. Add to the verifier checklist. |
| Profile filter regression on the four tools (Phase 64 D-14 invariant) | EoP | Existing 4 profile-filter tests at `internal/skill/semantic/profile_filter_test.go` cover Phase 64 tools; Phase 65 must NOT change which profile sees which tool. Add a Phase 65 version asserting all four (`get_repo_map`, `get_context`, `analyze_blast_radius`, `get_health`) remain visible across all five profiles after the SetSemanticLookup setter is wired. |
| Middleware order regression | EoP | Phase 65 adds NO new middleware; the LIFO order at daemon.go steps 14/14b/14c/14d is unchanged. Verifier asserts via `TestMiddlewareOrder_*` (existing). |
| `duckdb-go` leak into integ package (`vet-noduckdb`) | EoP | The `vet-noduckdb` analyzer at `cmd/vet-noduckdb/` already enforces this; Phase 65 integ package is types-only. |
| Kernel imports of `internal/semantic/store` or `internal/semantic/retrieval` (`vet-nokernel2semantic`) | EoP | The amended `vet-nokernel2semantic` allowlist permits ONLY `internal/semantic/integ/`; `internal/semantic/store/`, `retrieval/`, `graph/`, etc. remain forbidden. Pitfall §1 fix (a) preserves this. |
| Raw error text leakage into MCP envelope (info disclosure) | Info Disclosure | Closed-enum `fallback_reason` and `last_error` fields per WR-NEW-01. The `classifyLookupErr` helper (Pattern 5) is the mandatory chokepoint. Verifier asserts no `fmt.Sprintf("%v", err)` calls produce envelope content. |

---

## Project Constraints (from CLAUDE.md)

- **`go vet ./...` and `go test` MUST run clean before completing any Go task.** Phase 65 verifier checklist includes both.
- **`make vet` runs additional singlecheckers** (`vet-nokernel2semantic`, `vet-noduckdb`, `vet-compact-uses-store`, `vet-nosemantic2kernel`) — Phase 65 must keep all four green; the amended `nokernel2semantic` analyzer is the only one Phase 65 touches.
- **SMTC-first tool routing** (CLAUDE.md "Code intelligence: SMTC-first tool routing") — agents implementing Phase 65 use `mcp__smtc__*` for cross-package navigation (`goto_definition`, `find_references`, `get_callers`), not `Grep`. SMTC list_capabilities was checked; no `java-security` capability applies (Helix is Go-native).
- **GSD workflow enforcement** — Phase 65 implementation lands through `/gsd:execute-phase`. Tasks land via worktrees per `.planning/config.json` `use_worktrees: true`.
- **TDD mode is enabled** (`.planning/config.json` `tdd_mode: true`) — RED-first gates required for every task in waves 1–3. Wave 0 carryovers (65-01, 65-02) MAY be RED-then-GREEN since they have known existing inline contracts and locked adapter functions.
- **Phase 59 EXTRACT-01..05 has SHIPPED** (REQUIREMENTS.md lines 190-194 + STATE.md). The CONTEXT.md "BLOCKED on Phase 59" header is stale; planning resumes immediately.
- **Phase 65 must NOT modify `internal/repomap` engine source** (INTEG-01 contract; CONTEXT.md "Constraints" #2). All wiring goes through `internal/skill/repomap/skill.go` setters.

---

## Sources

### Primary (HIGH confidence)

- `internal/daemon/semantic_wiring.go` (1–756) — entire production wiring for Phase 64 semantic tools; production buildFn TODO anchors at lines 680, 723; WorkspaceKey adapter zero-return at lines 509-525; inline production-pipeline contract at lines 621-686.
- `internal/skill/repomap/skill.go` (1–538) — full RepoMapSkill scaffold; existing setter precedents at lines 113-144; `execGetRepoMap` / `execGetContext` at lines 269-351.
- `internal/kernel/symbols/tools.go` (lines 540-613) + `internal/kernel/symbols/blast.go` — existing pure-LSP `analyze_blast_radius` implementation; SYM-09 contract.
- `internal/kernel/health/tools.go` (lines 1–242) + `internal/kernel/health/skill_adapter.go` — Phase 57 SC-1 envelope and skill adapter pattern; closed-enum reasons at lines 50-55; `ComputeSemanticStoreStatus` at lines 93-113.
- `internal/lint/nokernel2semantic/analyzer.go` (1–58) — the analyzer that requires amendment (Pitfall §1).
- `internal/semantic/extract/provider.go` (1–85) + `internal/semantic/extract/to_store.go` (1–80) — locked Phase 65 unblock adapter and the `Provider.Extract` interface promotion (D-06, 2026-05-08).
- `internal/semantic/live/classifier.go` (1–96) — `ClassifyPathChange` for the production buildFn walk.
- `internal/semantic/store/effective_graph.go` (lines 22, 85, 155, 186, 208, 242) + `overlay.go` (lines 232, 983, 1019) — read-side accessors the lookup adapter consumes.
- `internal/semantic/retrieval/bleve.go` (lines 24-181) + `internal/semantic/retrieval/recovery.go` (lines 32-70) — `Engine.QueryBleve` + `Recoverer.RetrievalPending` surfaces.
- `internal/skill/semantic/accessors.go` (1–144) + `internal/skill/semantic/envelope.go` (1–149) — Phase 64 narrow accessor seams + closed-enum precedent the Phase 65 integ package mirrors.
- `internal/skill/semantic/integration_test.go` (lines 401–520) — 15-symbol fixture and harness pattern for Phase 65 acceptance tests.
- `internal/skill/repomap/skill_integration_test.go` (lines 1–80, 141–170) — existing index-disabled goldens that must be preserved through 65-04 JSON-wrap migration.
- `internal/daemon/daemon.go` (lines 510-754) — daemon bootstrap including `activeWSKey`, `wsKeyFn`, `SetActivateCallback`, semanticBundle wiring.
- `internal/workspace/key.go` (1–25) + `internal/mcp/session.go` (1–106) — `WorkspaceKey` struct + `SessionInfo.WorkspaceKey` (hashed string).
- `SPEC-DRAFT.md` §7, §11, §17, §18, §24 (lines 2419-2504), §25 (lines 2506-2606), §26.2, §30.2 — the source of truth for envelope shapes and confidence ladder.
- `.planning/REQUIREMENTS.md` (lines 76-82) — INTEG-01..05 contract.
- `.planning/ROADMAP.md` (lines 194-204) — Phase 65 success criteria.
- `.planning/phases/65-existing-tool-integration-strangler-fig/65-CONTEXT.md` (entire) — locked decisions D-01..D-10.
- `.planning/phases/64-new-mcp-tools/64-CONTEXT.md` (lines 163-205) — D-09 read+ doctrine, D-14 all-profiles-see-all-tools.
- `.planning/phases/64-new-mcp-tools/VERIFICATION.md` (lines 110-115) — the two TODO(phase-65) anchors.
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md` (lines 86-180, 273-284) — Phase 62 confidence ladder + RankScheduler + ApplyRepair.
- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md` (lines 240-310) — D-04 `overlay_epoch` CAS contract.
- `.planning/STATE.md` (lines 30-39) — Phase 59 / 60 / 64 verification state.

### Secondary (MEDIUM confidence)

- CONTEXT.md cross-references to `internal/skill/semantic/tools_refresh.go` (Phase 64 D-09/D-13 grep canary) — pattern verified by reading lines 90-94 of the VERIFICATION.md grep results; canary command quoted but the test code itself was not read.

### Tertiary (LOW confidence)

(None. All claims are verified against on-disk code.)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every package exists in the tree at the cited file/line.
- Architecture (setter pattern, daemon adapter, two-pass blast radius): HIGH — all three patterns have shipped precedents in the codebase.
- Pitfalls (especially §1 the analyzer collision): HIGH — verified by reading `internal/lint/nokernel2semantic/analyzer.go` and confirming no allowlist exists.
- Code examples: HIGH for signatures (every call signature checked); MEDIUM for the production buildFn full body (the inline pipeline contract is documented but the precise `walkOrDrain`/`langFromExt`/`adaptRankedFiles` helper names are placeholders the planner refines).

**Research date:** 2026-05-08
**Valid until:** 2026-06-08 (30 days; codebase state is stable post-Phase 64 verification, no major refactors expected during planning).

---

## RESEARCH COMPLETE

### Key Findings

1. **Phase 59 unblocked Phase 65.** STATE.md and REQUIREMENTS.md confirm EXTRACT-01..05 SHIPPED; CONTEXT.md "BLOCKED on Phase 59" header is stale. Planning resumes immediately. The locked `extract.ToStoreFacts` adapter at `internal/semantic/extract/to_store.go:58` is exactly the bridge the production buildFn needs.

2. **`nokernel2semantic` analyzer collision (Pitfall §1) is the only research-surfaced blocker requiring a planner decision.** The current analyzer at `internal/lint/nokernel2semantic/analyzer.go:34-57` blocks ALL `internal/semantic/*` imports from kernel; CONTEXT.md D-02 needs the kernel-side `analyze_blast_radius` adapter to import `internal/semantic/integ/`. Recommended fix: amend the analyzer with an exact-prefix allowlist for `internal/semantic/integ` (~10-line diff + analysistest fixture). Alternative: relocate integ outside `internal/semantic/`. Surface this as an explicit Wave 0 task (or absorb into 65-03).

3. **`get_repo_map` / `get_context` envelope contract requires a JSON-wrap migration of the existing string output (Pitfall §2).** Existing index-disabled goldens at `internal/skill/repomap/skill_integration_test.go:141-170` will need a one-time mechanical update; the underlying tree text is preserved verbatim per INTEG-01.

4. **Every concrete capability the four tools need is already shipped — Phase 65 is wiring + envelope + two-pass orchestrator.** `*Store` exposes `LatestCommittedSnapshot`, `CurrentGraphVersion`, `OverlayHasPendingRows`, `QueryEffectiveAdjacency`. `*retrieval.Engine` exposes `QueryBleve`. `*Recoverer` exposes `RetrievalPending`. `extract.Provider.Extract` + `ToStoreFacts` close the buildFn pipeline. `kernel/symbols.AnalyzeBlastRadius` becomes the Pass 2 LSP primitive unchanged. The Phase 64 narrow-accessor seams at `internal/skill/semantic/accessors.go` are the template for the new `SemanticLookup` interface.

5. **SymbolID translation for `analyze_blast_radius` (Open Question 3) is the one missing piece in CONTEXT.md D-03 — planner adds a method to the integ interface or threads translation through the daemon adapter.** Surface during 65-06 planning.

### File Created

`.planning/phases/65-existing-tool-integration-strangler-fig/65-RESEARCH.md`

### Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | Every cited file/line was read directly; no speculation about availability. |
| Architecture | HIGH | Three setter precedents (`SetEnrichFn`, `SetFallbackDeps`, `SetMetricsSink`) + Phase 64 narrow-accessor pattern + `health/skill_adapter.go` precedent all verified. |
| Pitfalls | HIGH | Each pitfall traces to a specific file/line/test. The `nokernel2semantic` collision (Pitfall §1) is the load-bearing finding. |

### Open Questions

1. Public read API on `*Store` for ranked files (vs threading `*RankScheduler` through). Plan a small reader in Wave 1 if absent.
2. SymbolID translation seam for `analyze_blast_radius` (kernel-side) — likely needs a method addition to the integ `SemanticLookup` interface.
3. `Status()` caching strategy — recommend no cache for v1.10; profile in Phase 67.

### Ready for Planning

Research complete. Planner can create PLAN.md files for the eight-task wave structure (Wave 0: 65-01, 65-02, 65-00-vet [analyzer amendment]; Wave 1: 65-03, 65-04; Wave 2: 65-05, 65-06, 65-07; Wave 3: 65-08).
