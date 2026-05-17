# Phase 71: P1 Single-Symbol Read Tools — Research

**Researched:** 2026-05-17
**Domain:** MCP tool surface over the v1.10 semantic graph + retrieval engine
**Confidence:** HIGH (all seams probed in-tree against current `main`)

## Summary

Phase 71 adds three new `read+` MCP tools (`explain_symbol_deep`, `find_related_symbols`, `validate_graph_edge`) to `internal/skill/semantic/`, mirroring the Phase 64 P0 four-tool pattern exactly: each tool gets its own `tools_<name>.go` with a typed-args struct, a const help string, a `register<Name>` function called from `RegisterAll`, and a `handle<Name>` method on `*SemanticSkill`. All shared scaffolding (envelope.go, accessors.go, mode_check.go, skill.go) is already in place.

The two heavy lifts are:
1. **A new `(file_path, symbol_name) → SymbolID` resolver.** `integ.SemanticLookup` currently exposes `SymbolID(file, line, col)` — coordinate-based, not name-based. There is no `QuerySymbolByName` on `*Store`. A new accessor + thin wrapper in `internal/skill/semantic/accessors.go` is REQUIRED.
2. **Three new freshness fields** (`snapshot_id`, `extractor_run_id`, `as_of_unix_ms`) on the response envelope. `SemanticStatus.LatestSnapshotID` and `GraphVersion` exist; the other two do NOT. `as_of_unix_ms` is trivial (server clock); `extractor_run_id` needs a new accessor on `*Store`.

The retrieval entrypoint is essentially solved: `retrieval.Fuse(text, graph, cfg, gvLookup)` already implements RRF with the Phase 62 sort-before-iterate tiebreak, and `RetrievalAccessor.PersonalizedPageRank` already accepts a seed list. `find_related_symbols` reuses the exact code path `get_semantic_context` uses today, with a single-symbol anchor list. Cluster co-membership boost has no current implementation in retrieval — it must be added (or, planner's call: deferred to Phase 72).

The edge-kind vocabulary is informal: only `"RESOLVES_TO"` is consistently emitted by name; `"CALLS"` / `"USES_TYPE"` appear in comments and tests but are extractor-driven strings, not a closed Go enum. The mapping table is essentially a research/planner-decided fixed set keyed off live SQL surveys plus extractor source-reading.

**Primary recommendation:** Implement as five plans — (1) seam accessors (QuerySymbolByName + ExtractorRunID + ClusterMembershipReference), (2) edge-kind surface enum + mapper, (3) `explain_symbol_deep` tool, (4) `find_related_symbols` tool, (5) `validate_graph_edge` tool + cross-tool integration test + fixture extension. Each later plan is independently mergeable once (1) and (2) land.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Seed resolution `(file, name) → SymbolID` | API / skill-semantic | Database (new `*Store` accessor) | `accessors.go` wraps; new `QuerySymbolByName` on `*Store` does the SQL |
| Tool registration / MCP wiring | API / skill-semantic | — | `register.go::RegisterAll` |
| `read+` mode enforcement | API / skill-semantic | — | `mode_check.go::checkMode(snap, modeTierRead)` at handler entry |
| Type-chain lookup | Database / `*Store` | API (resolver) | `integ.SemanticLookup.ExpandFrom` returns Impact rows; type ladder lives in `internal/semantic/types/` |
| Incoming / outgoing edge enumeration | Database / `*Store` | API (mapper) | `StoreAccessor.QueryEffectiveAdjacency` already returns (out, in) adjacency for a projection |
| Caller enumeration | Database / `*Store` | API | Use `QueryEffectiveAdjacency(projection="call_graph")` reverse-indexed |
| RRF fusion | API / retrieval | — | `retrieval.Fuse` already implements weighted RRF |
| Personalized PageRank from seed | API / retrieval | — | `RetrievalAccessor.PersonalizedPageRank(ctx, repoID, anchors)` |
| Cluster co-membership boost | API / skill-semantic | Database | Read `semantic_cluster_members`; weight the seed's cluster mates; mix into Fuse output post-hoc |
| Edge-claim evidence assembly | API / skill-semantic | — | New `edge_evidence.go`: pulls LSP citations from edge.Source, AST citations from extractor metadata, type-resolver tier from RESOLVES_TO edges |
| Freshness envelope | API / skill-semantic | Database | `accessors.go` reads `graph_version`, `snapshot_id`, new `extractor_run_id`, `time.Now().UnixMilli()` |

## Standard Stack

### Core (already in use; no new deps)
| Library / Package | Version | Purpose | Why Standard |
|---|---|---|---|
| `github.com/modelcontextprotocol/go-sdk/mcp` | (pinned in go.mod) | MCP tool registration via `mcpsdk.AddTool` | Official Go SDK, already used by all four Phase 64 tools |
| `go.opentelemetry.io/otel/trace` | (pinned) | `kernel.WrapToolSpan` for per-tool spans | Phase 64 tools all use this pattern |
| `internal/semantic/integ` | in-tree | `SemanticLookup` seam (read-only) | Phase 65 D-03; production wiring in daemon |
| `internal/semantic/retrieval` | in-tree | `Fuse`, `TextRank`/`GraphRank` types, RRF config | Phase 64 P07 retrieval engine |
| `internal/semantic/graph` | in-tree | `EdgeKind`, repair shapes | Phase 62 graph engine |
| `internal/semantic/store` | in-tree | `*Store` SQL accessors | Phase 60 store |
| `internal/kernel` | in-tree | `WrapToolSpan` | Tracing wrapper |
| `internal/guardrails` | in-tree | `IssueReceiptOnSuccess(ctx, Class…)` | Phase 66 receipt issuance; `tools_context.go` already uses this |
| `internal/errors` | in-tree | `serr.New(…)` structured errors | mode_check.go already uses this |

**No new external dependencies.** Phase 71 is purely additive Go in existing packages.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|---|---|---|
| New `QuerySymbolByName` on `*Store` | `*Store.QuerySymbolByLocation` after re-parsing the file to find the name | Adds parse coupling and is slower; one new SQL helper is cleaner |
| Per-tool freshness assembly | Shared helper in `accessors.go` | Helper preferred — three call sites would otherwise drift |
| Hand-rolled cluster-boost fusion | Defer cluster boost to Phase 72 | If cluster engine integration is non-trivial, defer; ROADMAP allows it (succ.crit. #2 says "via PageRank-from-seeds + cluster co-membership + RRF fusion" — cluster co-membership IS in scope for Phase 71) |

## Package Legitimacy Audit

> No new external packages — Phase 71 is in-tree Go only. Audit not applicable.

| Package | Registry | Disposition |
|---|---|---|
| (none) | — | — |

## Architecture Patterns

### System Architecture Diagram

```
                          MCP request (tools/call)
                                   │
                                   ▼
                ┌────────────── Middleware stack ──────────────┐
                │  LazyInit → Suggestion → ProfileFilter →     │
                │  Telemetry  (LIFO; established Phase 64)     │
                └─────────────────────┬────────────────────────┘
                                      ▼
   ┌──────────────────────────────────────────────────────────────────┐
   │   handle{ExplainSymbolDeep,FindRelatedSymbols,ValidateGraphEdge} │
   │                                                                  │
   │   1. checkMode(snap, modeTierRead)        [mode_check.go]        │
   │   2. resolveSeed(args) → SymbolID + ResolutionEnum               │
   │        ├─ symbol_id variant: passthrough                         │
   │        └─ (file,name) variant: NEW accessors.go wrapper          │
   │           → NEW *Store.QuerySymbolByName(repo, path, name)       │
   │              returns []SymbolID; len==1=exact, >1=ambiguous,     │
   │              0=not_found                                         │
   │   3. (per-tool work, see below)                                  │
   │   4. assembleFreshnessEnvelope() [accessors.go helper]           │
   │      ├─ store.CurrentGraphVersion                                │
   │      ├─ store.LatestCommittedSnapshot                            │
   │      ├─ NEW store.LatestExtractorRunID(repo)                     │
   │      ├─ time.Now().UnixMilli()                                   │
   │      └─ Phase69 status = current|stale|unknown                   │
   │   5. guardrails.IssueReceiptOnSuccess  (Phase 66)                │
   └────────────────────┬────────────┬────────────┬────────────────────┘
                        │            │            │
        explain_symbol_deep  find_related   validate_graph_edge
                        │            │            │
       ┌────────────────┘            │            └─────────────────┐
       ▼                             ▼                              ▼
 ExpandFrom (type chain)     PersonalizedPageRank       For seed edge:
 QueryEffectiveAdjacency     (anchors=[seedSymbolID])    1. Probe edge row in
  (out + in edges for sym)   QueryBleve (anchor-only)       semantic_edges
 callers = adj[in] over     retrieval.Fuse(...)             (RESOLVES_TO/CALLS)
  call_graph projection     + cluster co-membership      2. Source field has
 cluster row from           boost (NEW: read              "lsp.go.text_doc…" →
  semantic_cluster_members  semantic_cluster_members)     LSP evidence
                            paths filter (post-Fuse)     3. Lookup type-resolver
                                                             tier via ladder
                                                          4. Cap conf ≤ 0.6 if
                                                             degraded (TYPES-04)
       │                             │                              │
       └──────── EdgeKindMapper.MapInternalKind ──────┐              │
       (internal → surface: calls/references/         │              │
        implements/extends/has_type/uses_type/        │              │
        contains/other)                               │              │
                                                      ▼              ▼
                                          response envelope (JSON over MCP)
```

### Recommended File Layout

```
internal/skill/semantic/
├── accessors.go                      # EXTEND: add SymbolByNameAccessor, ExtractorRunAccessor, ClusterMembershipAccessor
├── envelope.go                       # EXTEND: add Seed envelope (resolution, ambiguous_candidates) + FreshnessV2 with snapshot_id/extractor_run_id/as_of_unix_ms/status
├── register.go                       # EXTEND: RegisterAll calls 3 new register*() funcs
├── edge_kind_surface.go              # NEW: closed enum + MapInternalKind() function
├── tools_explain_symbol.go           # NEW: handler + args + help const
├── tools_explain_symbol_test.go      # NEW: table-driven unit tests
├── tools_find_related.go             # NEW
├── tools_find_related_test.go        # NEW
├── tools_validate_edge.go            # NEW
├── tools_validate_edge_test.go       # NEW
├── seed_resolve.go                   # NEW: resolveSeed() shared helper for the 3 tools
├── seed_resolve_test.go              # NEW
└── populated_graph_fixture_test.go   # NEW or rename: shared multi-language graph fixture builder (referenced by all 3 tool tests + integration_test.go)
```

### Pattern 1: Tool Registration (mirror tools_context.go:110-124)
```go
// Source: internal/skill/semantic/tools_context.go:110-124 (verified in-tree)
func registerExplainSymbolDeep(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "explain_symbol_deep",
        Description: "Deep explanation of one symbol: type chain + edges + callers + cluster (read+).",
    }, kernel.WrapToolSpan(tracer, "explain_symbol_deep",
        func(ctx context.Context, req *mcpsdk.CallToolRequest, args ExplainSymbolDeepArgs) (*mcpsdk.CallToolResult, any, error) {
            return s.handleExplainSymbolDeep(ctx, args), nil, nil
        }))
    server.Registry().Register(&mcp.ToolDef{
        Name:             "explain_symbol_deep",
        Description:      "Deep explanation of one symbol (read+).",
        BriefDescription: "Deep symbol explanation",
        HelpText:         explainSymbolDeepHelp,
    })
}
```

Then in `register.go::RegisterAll`:
```go
registerExplainSymbolDeep(server, s, tracer)
registerFindRelatedSymbols(server, s, tracer)
registerValidateGraphEdge(server, s, tracer)
```

### Pattern 2: Handler Entry (mirror tools_context.go:151-163)
```go
func (s *SemanticSkill) handleExplainSymbolDeep(ctx context.Context, args ExplainSymbolDeepArgs) *mcpsdk.CallToolResult {
    snap := s.sessionSnapshot(ctx)
    if err := checkMode(snap, modeTierRead); err != nil {
        return errorResult(err.Error())
    }
    ws := s.workspaceKey(ctx)
    seed, resolution, err := s.resolveSeed(ctx, ws, args.Seed)  // NEW helper
    if err != nil { return errorResult(err.Error()) }
    if resolution == ResolutionNotFound {
        return jsonResult(buildNotFoundEnvelope(seed, ws))
    }
    // ... per-tool work ...
}
```

### Pattern 3: D-09 Read-Only Grep-Gate (mirror tools_refresh.go:15-23 + tools_refresh_test.go:24-43)
Each new handler file MUST carry the same header comment as `tools_refresh.go:15-23` (read-only invariant). Each test file MUST install a `recorderStoreAccessor` whose `BeginSnapshot`/`CommitSnapshot`/`AbortSnapshot`/`WriteSnapshotFacts` methods call `t.Fatal` — see `tools_refresh_test.go:116-123` for the exact pattern. CI grep-gate extension: add the three new handler filenames to the existing grep-gate list (researcher could not locate the grep-gate runner config — likely lives in `.github/workflows/` or `scripts/check-read-only.sh`; planner must locate during 71-01 or 71-02).

### Anti-Patterns to Avoid
- **Editing `skill.go`** beyond the existing "FINAL at end-of-W0" stub-deletion contract. The Phase 64 plan locked it; Phase 71 adds tools without re-touching shared scaffolding (the `accessors.go` interface additions and `register.go` body extension are the only allowed mutations to shared files).
- **Resolving seeds through LSP positions.** D1 rejects this; the graph is the source of truth.
- **Direct edge-kind passthrough.** All edge_kind outputs MUST go through `MapInternalKind` so the surface enum stays closed.
- **Computing confidence inline.** Use the `internal/semantic/types.CapCommentConfidence` and `ConfidenceForEvidence` helpers; do not re-implement the ladder.
- **Treating empty cluster membership as an error.** Most symbols are in cluster `0` or unassigned; emit `{cluster_id: 0, size: 0}` not a refusal.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---|---|---|---|
| Rank fusion | Custom RRF | `internal/semantic/retrieval.Fuse(text, graph, cfg, gvLookup)` | Already implements weighted RRF + 3-key tiebreak; tested under 10-run determinism |
| Personalized PageRank | Custom traversal | `RetrievalAccessor.PersonalizedPageRank(ctx, repoID, anchors)` | Backed by Phase 62 graph; already wired |
| Symbol fact lookup | Direct SQL | `*Store.QueryEffectiveSymbol` (existing) + new `QuerySymbolByName` | Lock-free read accessors per Phase 69 D1 |
| Type ladder confidence | Inline math | `types.ConfidenceForEvidence(kind)` + `types.CapCommentConfidence(c, kind)` | TYPES-03 / TYPES-04 caps are centralized |
| MCP tool tracing | Bare handler | `kernel.WrapToolSpan(tracer, "tool_name", handler)` | All Phase 64 tools use this |
| Receipt issuance | Inline | `guardrails.IssueReceiptOnSuccess(ctx, ClassContextGathered, scope)` | Phase 66 D-01 GUARD-03 contract |
| Path traversal validation | Inline | `validatePaths(args.Files, ws.RepoRoot)` (existing helper in handler_helpers.go) | Test infra already validates against `..` and absolute paths |

**Key insight:** The Phase 64 P0 wave already commoditized every shared concern (envelope, tracing, mode-check, receipt, validation). Phase 71 is mostly tool-specific business logic plus three thin accessor additions.

## Runtime State Inventory

Phase 71 is greenfield (additive tools, no rename / refactor / migration). This section is intentionally omitted.

## Common Pitfalls

### Pitfall 1: Seed resolution race with overlay writes
**What goes wrong:** `QuerySymbolByName` reads `semantic_symbols` at the latest committed snapshot. Between resolution and the per-tool read (e.g., `QueryEffectiveAdjacency`), a new snapshot may commit, leaving the resolved `SymbolID` referencing a stale snapshot's symbol that may no longer exist in the effective graph.
**Why it happens:** No snapshot pinning across multi-step reads.
**How to avoid:** Re-read `LatestCommittedSnapshot` at envelope-assembly time and stamp the response with that snapshot_id. If the seed `SymbolID` was resolved against an older snapshot but the effective-graph reads return no rows, emit `resolution: "not_found"` with `fallback_reason: "symbol_evicted"`. Mirror the `RetrievalPending`-early-return pattern from `tools_context.go:212-224`.
**Warning signs:** Concurrent invocation tests (D7 #5) under `-race` show occasional empty edge lists for seeds that resolved successfully.

### Pitfall 2: `RESOLVES_TO` edges dominating the edge-kind surface
**What goes wrong:** Every type-resolver pass writes `RESOLVES_TO` edges (verified: `internal/semantic/types/{golang,typescript,java,python}/resolver.go` all emit this kind exclusively). If the surface mapping table omits a dedicated bucket, `explain_symbol_deep` reports almost every edge as `other` or `has_type`.
**Why it happens:** The internal vocabulary is type-resolver-biased; `CALLS` / `USES_TYPE` are emitted by extractor passes elsewhere.
**How to avoid:** Map `RESOLVES_TO` → `has_type` in the surface enum. Confirmed by reading the four resolver files: `RESOLVES_TO` always denotes a "this symbol has this type" relationship.
**Warning signs:** Mapping-coverage test (D7 #3 round-trip) shows all symbols' edges collapsing to `other`.

### Pitfall 3: Cluster co-membership over the wrong graph_version
**What goes wrong:** `semantic_cluster_members` is keyed on `(repo_id, graph_version, cluster_id, node_id)`. If `find_related_symbols` queries cluster membership against a graph_version different from the one PageRank ran against, the boost is silently zero or skewed.
**Why it happens:** PageRank reads from the effective graph (snapshot ⊕ overlay); cluster rows are only written for committed snapshots.
**How to avoid:** Use `LatestCommittedSnapshot`'s `graph_version` for both PageRank seed expansion AND the cluster-members read. When `OverlayHasPendingRows == true`, set `fallback_reason: "overlay_active_cluster_boost_skipped"` and emit related symbols without cluster boost.
**Warning signs:** `find_related_symbols` returns the same ranking with/without cluster boost.

### Pitfall 4: Mapping table breakage when extractor adds a new internal kind
**What goes wrong:** A future extractor pass emits `INVOKES_VIRTUAL` (or similar). `MapInternalKind` returns `other` silently; agents start receiving uninformative classifications.
**Why it happens:** No build-time check that the mapping table is complete.
**How to avoid:** Emit a bounded-label metric `mcp_edge_kind_surface_other_total{internal_kind=…}` (per D3) so unmapped kinds are visible on dashboards. Document this in `edge_kind_surface.go`. Cannot be a build-time check — internal kinds are strings, not a typed enum.
**Warning signs:** Production metric shows non-zero `other` rate for an `internal_kind` that should map.

### Pitfall 5: TYPES-04 cap interaction with the 3-source confidence sum
**What goes wrong:** D4 proposes LSP=0.4, AST=0.3, type_resolver=0.3 (scaled by tier). With all three present and an LSP-confirmed tier-1 type, the sum is 1.0. But if any contribution is "degraded" (type_resolver tier 6/7), the sum must cap at 0.6 (TYPES-04), which conflicts with the additive model.
**Why it happens:** Two competing rules: additive contribution vs. tier-cap.
**How to avoid:** Compute the raw additive score first. If `evidence_status != "complete"` OR any `type_resolver` tier is `tier6_heuristic` or `tier7_unknown`, clamp final confidence to `min(rawScore, 0.6)` using `types.CapCommentConfidence`. Per-source `confidence_contribution` in the evidence array reflects the unclamped pre-sum value; the envelope's top-level `confidence` is the post-cap value. Document this asymmetry in the help text.
**Warning signs:** `validate_graph_edge` returns confidence > 0.6 for an edge whose evidence is all heuristic.

### Pitfall 6: Concurrent map writes in cluster-membership boost
**What goes wrong:** Reading `semantic_cluster_members` and merging boosts into the Fuse output uses a map keyed on `SymbolID`. If implemented naively across goroutines (per-symbol fan-out), the map race-detector trips.
**Why it happens:** Tempting to parallelize per-candidate boosts.
**How to avoid:** Single goroutine, single pass over `FusedCandidate` output. Boost factor is a deterministic multiplier on Score; ordering preserved by re-sorting via the same 3-key tiebreak `retrieval.Fuse` uses internally (or extend `Fuse` itself with an optional `clusterBoost` parameter).

## Code Examples

### Resolving a seed (NEW helper)
```go
// Source: NEW, modeled on tools_context.go validation pattern
// File: internal/skill/semantic/seed_resolve.go

type SeedInput struct {
    SymbolID   string `json:"symbol_id,omitempty"   jsonschema:"graph-internal stable key"`
    FilePath   string `json:"file_path,omitempty"   jsonschema:"workspace-relative path; pair with symbol_name"`
    SymbolName string `json:"symbol_name,omitempty" jsonschema:"qualified symbol name; pair with file_path"`
}

type Resolution string
const (
    ResolutionExact     Resolution = "exact"
    ResolutionAmbiguous Resolution = "ambiguous"
    ResolutionNotFound  Resolution = "not_found"
)

type ResolvedSeed struct {
    SymbolID            integ.SymbolID
    Resolution          Resolution
    AmbiguousCandidates []integ.SymbolID // cap 5
}

func (s *SemanticSkill) resolveSeed(ctx context.Context, ws workspace.WorkspaceKey, in SeedInput) (ResolvedSeed, error) {
    if in.SymbolID != "" {
        return ResolvedSeed{SymbolID: integ.SymbolID(in.SymbolID), Resolution: ResolutionExact}, nil
    }
    if in.FilePath == "" || in.SymbolName == "" {
        return ResolvedSeed{}, fmt.Errorf("seed requires symbol_id OR (file_path AND symbol_name)")
    }
    // NEW: s.symbolByName accessor
    candidates, err := s.symbolByName.QuerySymbolByName(ctx, ws.Hash(), in.FilePath, in.SymbolName)
    if err != nil { return ResolvedSeed{}, err }
    switch len(candidates) {
    case 0:
        return ResolvedSeed{Resolution: ResolutionNotFound}, nil
    case 1:
        return ResolvedSeed{SymbolID: candidates[0], Resolution: ResolutionExact}, nil
    default:
        if len(candidates) > 5 { candidates = candidates[:5] }
        return ResolvedSeed{
            SymbolID:            candidates[0],
            Resolution:          ResolutionAmbiguous,
            AmbiguousCandidates: candidates,
        }, nil
    }
}
```

### Fusion entrypoint (mirror tools_context.go:226-264)
```go
// find_related_symbols reuses get_semantic_context's fusion verbatim with
// the seed as a single-symbol anchor:
anchors := []string{string(seed.SymbolID)}
textRanks, _ := s.retrieval.QueryBleve("", anchors)        // empty task → graph-only bias
graphRanks, _ := s.retrieval.PersonalizedPageRank(ctx, repoID, anchors)
// translate types (see tools_context.go:249-256), then:
fused := retrieval.Fuse(text, graph, retrieval.DefaultRRFConfig(), gvLookup)
// post-fuse: apply cluster co-membership boost (NEW), then paths filter, then top-k clamp
```

### Edge kind surface mapping (NEW)
```go
// Source: NEW; file: internal/skill/semantic/edge_kind_surface.go
package semantic

type EdgeKindSurface string
const (
    EdgeKindCalls       EdgeKindSurface = "calls"
    EdgeKindReferences  EdgeKindSurface = "references"
    EdgeKindImplements  EdgeKindSurface = "implements"
    EdgeKindExtends     EdgeKindSurface = "extends"
    EdgeKindHasType     EdgeKindSurface = "has_type"
    EdgeKindUsesType    EdgeKindSurface = "uses_type"
    EdgeKindContains    EdgeKindSurface = "contains"
    EdgeKindOther       EdgeKindSurface = "other"
)

// MapInternalKind translates an internal EdgeKind string to a surface enum.
// One-way mapping; unmapped kinds yield EdgeKindOther.
func MapInternalKind(internal string) EdgeKindSurface {
    switch internal {
    case "CALLS":        return EdgeKindCalls
    case "REFERENCES":   return EdgeKindReferences
    case "IMPLEMENTS":   return EdgeKindImplements
    case "EXTENDS":      return EdgeKindExtends
    case "RESOLVES_TO":  return EdgeKindHasType  // Pitfall 2 — RESOLVES_TO is the resolver's "has-type" edge
    case "USES_TYPE":    return EdgeKindUsesType
    case "CONTAINS":     return EdgeKindContains
    case "DEFINED_IN":   return EdgeKindContains // synonym in some extractors
    case "IMPORTS":      return EdgeKindReferences
    default:             return EdgeKindOther
    }
}
```

### Freshness envelope assembly (NEW helper)
```go
// File: internal/skill/semantic/accessors.go (extension)

type FreshnessV2 struct {
    GraphVersion    uint64 `json:"graph_version"`
    SnapshotID      uint64 `json:"snapshot_id"`
    ExtractorRunID  string `json:"extractor_run_id"`
    AsOfUnixMs      int64  `json:"as_of_unix_ms"`
    Status          string `json:"status"` // current|stale|unknown — Phase 69 D1
}

func (s *SemanticSkill) assembleFreshness(ctx context.Context, ws workspace.WorkspaceKey) FreshnessV2 {
    repoID := ws.Hash()
    gv, _   := s.store.CurrentGraphVersion(ctx, repoID)
    snap, _ := s.store.LatestCommittedSnapshot(ctx, repoID)
    runID := ""
    if s.extractorRun != nil { // NEW accessor
        runID, _ = s.extractorRun.LatestExtractorRunID(ctx, repoID)
    }
    status := "current"
    if s.store.OverlayHasPendingRows(repoID) { status = "stale" }
    if snap == 0 { status = "unknown" }
    return FreshnessV2{
        GraphVersion:   gv,
        SnapshotID:     snap,
        ExtractorRunID: runID,
        AsOfUnixMs:     time.Now().UnixMilli(),
        Status:         status,
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|---|---|---|---|
| LSP-position-keyed lookups | Stable `SymbolID` keys | Phase 59 EXTRACT-02 | All semantic reads address symbols by stable ID, not file:line:col |
| Bespoke fusion per tool | `retrieval.Fuse` with RRF | Phase 64 P07 | Single, tested entrypoint with deterministic tiebreak |
| Unbounded confidence | 7-tier ladder + caps | Phase 62 TYPES-01..04 | Comment-derived ≤ 0.60; degraded paths ≤ 0.60 |
| Snapshot-mutating reads | Lock-free `*Store` accessors | Phase 69 D1 | Read tools never serialize against the snapshot-write path |
| Single freshness bool | Closed-enum 4-state Freshness | Phase 64 envelope | `fresh` / `stale` / `structurally_fresh_semantically_pending` / `overlay_active` already in `envelope.go` |
| `state: unknown` placeholders | Real cluster + retrieval status | Phase 69 | Phase 71 envelopes reuse Phase 69 derivations |

**Deprecated/outdated:**
- LSP-position seed addressing for new MCP tools (rejected in D1).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|---|---|---|
| A1 | `RESOLVES_TO` should map to `has_type` (not `uses_type`) on the surface enum | Code Examples / Pitfall 2 | If wrong, agents misread type-resolver edges as use-relationships; correctable by table edit |
| A2 | A new `QuerySymbolByName(repoID, path, name) []SymbolID` accessor is the cleanest add for D1 seed resolution | Standard Stack | If existing accessor was missed, plan 71-01 becomes a no-op stub; researcher checked `effective_graph.go` exhaustively and only `QuerySymbolByLocation` + `QuerySymbolPath` exist |
| A3 | Cluster co-membership boost is part of Phase 71 scope (ROADMAP succ.crit. #2) and is implementable as a post-Fuse multiplier read from `semantic_cluster_members` | Architecture | If the cluster engine surface is harder than a SQL read, planner may defer to Phase 72 with explicit `fallback_reason: "cluster_boost_pending"` |
| A4 | `extractor_run_id` does not currently exist anywhere in the codebase and must be added as a new accessor + a new column or schema row | Summary / Pitfall 1 | If a stamp already exists in `semantic_files` or `semantic_snapshots`, the accessor is a thin select; researcher grepped and found no hit |
| A5 | Phase 69 D1 three-state status enum is `{current, stale, unknown}` matching the cluster_status enum in `envelope.go:73-74` | Code Examples | Re-confirmed via 69-CONTEXT.md grep — A5 is HIGH confidence |
| A6 | The "Phase 64 P07 populated-graph fixture" referenced in CONTEXT.md is NOT a multi-language graph fixture — it is the bleve recovery harness. A new multi-language fixture must be built fresh in Phase 71 | Architecture / D7 | If a fixture is hidden under a different name, planner discovers it during 71-04 |
| A7 | `vet-nokernel2semantic` build target enforces the kernel↔semantic boundary; Phase 71 stays semantic-side and does not breach it | Architecture / CONTEXT.md | Verified by reading CONTEXT.md; no kernel imports needed for any of the three tools |

## Open Questions (RESOLVED)

1. **Where is the CI grep-gate runner config?**
   **RESOLVED:** 71-01 Task 2 probes for an external runner (`.github/workflows/`, `scripts/`); 71-05 Task 3 branches on the finding — Branch A extends the external runner, Branch B adds an in-tree Go test mirroring `tools_refresh_test.go:116-135` recorder gate.
   **Original investigation:**
   - What we know: `tools_refresh.go` carries a header comment ("INVARIANT (D-09 / D-13): … no Begin/Commit/Abort/Write …") and `tools_refresh_test.go` installs a recorder accessor that fails on those methods. CONTEXT.md says "CI grep-gate on the three new handler files (mirroring `tools_refresh.go`'s gate)."
   - What's unclear: The actual grep step is not in `internal/skill/semantic/`. It is likely in `.github/workflows/*.yml` or `Makefile`/`scripts/`. Researcher did not exhaustively probe CI configs.
   - Recommendation: Plan 71-01 (seam plan) includes a 30-min "locate grep-gate; document file + line" task before the first new handler lands.

2. **Should cluster co-membership boost be part of Phase 71 or deferred to Phase 72?**
   **RESOLVED:** 71-01 declares `ClusterMembershipAccessor` narrow interface; 71-04 wires the fusion integration. If accessor cost is non-trivial during execution, 71-04 keeps the boost disabled (returns 0 contribution) with `fallback_reason: "cluster_boost_unavailable"` and the interface still ships in 71-01.
   **Original investigation:**
   - What we know: ROADMAP succ.crit. #2 lists it; cluster engine (`semantic_cluster_members`) exists in Phase 62/69.
   - What's unclear: Whether reading the cluster_id of a single symbol is a one-row SELECT or requires the cluster engine's runtime accessor. The `SchedulerAccessor.ClusterStatus(repoID)` exists but is bucket-status, not per-symbol membership.
   - Recommendation: Add a new `ClusterMembershipAccessor.ClusterIDOf(repoID, symbolID) (clusterID uint64, size int, err error)` in plan 71-01. If hard, defer the boost and emit `fallback_reason: "cluster_boost_unavailable"` per A3.

3. **How should `validate_graph_edge` derive AST citations?**
   **RESOLVED:** 71-05 Task 1 first step investigates the extractor surface; if AST metadata is absent (no range/file on edges), the `ast` source contributes only via internal-kind presence and `evidence_status: "partial"` is returned. Outcome documented in plan 71-05 SUMMARY.
   **Original investigation:**
   - What we know: LSP citations live on `Edge.Source = "lsp.go.text_document_definition"` etc. (verified in `internal/semantic/types/golang/resolver.go:60`). Type-resolver citations come from the ladder (`EvidenceLSP`/`EvidenceAnnotation`/`EvidenceConstructor`/`EvidenceAssignment`/`EvidenceComment`/`EvidenceHeuristic`/`EvidenceUnknown` — that's the 7-tier ladder).
   - What's unclear: AST citations have no dedicated `Source` prefix today. They may need to be synthesized from extractor metadata (the `tree_sitter_kind` and `range` would come from the extractor's per-symbol record).
   - Recommendation: Plan 71-05 (`validate_graph_edge` plan) drafts the AST citation shape and verifies extractor output carries the needed metadata. If it doesn't, AST citations become `evidence_kind: "ast"` with `tree_sitter_kind: ""` and `evidence_status: "partial"`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|---|---|---|---|---|
| Go toolchain (host CC) | All builds | Required | go.mod-pinned | — |
| `go vet`, `go test -race` | Validation | Built-in | — | — |

No external dependencies. Phase 71 is pure Go in existing packages.

## Validation Architecture

> Phase 71 honors `workflow.nyquist_validation` (assumed enabled — config absent).

### Test Framework
| Property | Value |
|---|---|
| Framework | Go `testing` + `-race` |
| Config file | None — Go convention (`*_test.go`) |
| Quick run command | `go test ./internal/skill/semantic/ -run <pattern> -race` |
| Full suite command | `go test ./internal/skill/semantic/... ./internal/semantic/... -race -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|---|---|---|---|---|
| P1TOOL-01 | `explain_symbol_deep` returns type chain + edges + callers + cluster + envelope | unit + integration | `go test ./internal/skill/semantic/ -run TestExplainSymbolDeep -race` | ❌ Wave 0 |
| P1TOOL-02 | `find_related_symbols` ranks via PageRank+cluster+RRF; honors paths strict-subset | unit + integration | `go test ./internal/skill/semantic/ -run TestFindRelatedSymbols -race` | ❌ Wave 0 |
| P1TOOL-06 | `validate_graph_edge` returns confidence + evidence array | unit | `go test ./internal/skill/semantic/ -run TestValidateGraphEdge -race` | ❌ Wave 0 |
| Phase 71 succ.crit. #1 | Works for Go AND TypeScript AND Java | integration | `go test ./internal/skill/semantic/ -run TestThreeTools_PopulatedGraph_MultiLang -race` | ❌ Wave 0 — needs fixture |
| Phase 71 succ.crit. #4 | `read+` mode enforced at handler entry | unit | `go test ./internal/skill/semantic/ -run TestThreeTools_ModeEnforcement` | ❌ Wave 0 |
| Phase 71 succ.crit. #5 | Closed-enum envelope fields present | unit + integration | `go test ./internal/skill/semantic/ -run TestThreeTools_EnvelopeShape -race` | ❌ Wave 0 |
| D7 #5 | Race-clean under concurrent invocation | integration | `go test ./internal/skill/semantic/ -run TestThreeTools_Concurrent -race -count=10` | ❌ Wave 0 |
| D3 | Edge-kind surface round-trip (all known mappings + unmapped → other) | unit | `go test ./internal/skill/semantic/ -run TestEdgeKindSurface_RoundTrip` | ❌ Wave 0 |
| D4 | Confidence cap at 0.6 on degraded TYPES-04 paths | unit | `go test ./internal/skill/semantic/ -run TestValidateGraphEdge_TYPES04Cap` | ❌ Wave 0 |
| D1 | Seed resolution exact/ambiguous/not_found | unit | `go test ./internal/skill/semantic/ -run TestSeedResolve` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go vet ./internal/skill/semantic/ && go test ./internal/skill/semantic/ -race -count=1` (~5s)
- **Per wave merge:** `go test ./internal/skill/semantic/... ./internal/semantic/... -race -count=1` (~30s)
- **Phase gate:** `go test ./... -race -count=1` green; `go vet ./...` clean; CI grep-gate green for `tools_explain_symbol.go`, `tools_find_related.go`, `tools_validate_edge.go`.

### Wave 0 Gaps
- [ ] `internal/skill/semantic/tools_explain_symbol_test.go` — covers P1TOOL-01
- [ ] `internal/skill/semantic/tools_find_related_test.go` — covers P1TOOL-02
- [ ] `internal/skill/semantic/tools_validate_edge_test.go` — covers P1TOOL-06
- [ ] `internal/skill/semantic/seed_resolve_test.go` — covers D1
- [ ] `internal/skill/semantic/edge_kind_surface_test.go` — covers D3
- [ ] `internal/skill/semantic/populated_graph_fixture_test.go` (or extension to an existing fixture file) — multi-language fixture with at least one Go + one TS + one Java symbol and a handful of CALLS / RESOLVES_TO / USES_TYPE edges
- [ ] `internal/skill/semantic/integration_test.go` — extend with `TestThreeTools_*` cases for cross-tool consistency

No framework install needed — Go `testing` ships with the toolchain.

## Sources

### Primary (HIGH confidence — read directly in this session)
- `internal/skill/semantic/envelope.go` — Freshness/IndexStatus/ClusterStatus/RetrievalStatus/CommonEnvelope shapes
- `internal/skill/semantic/mode_check.go` — `checkMode(snap, modeTierRead|Review|Admin)` pattern
- `internal/skill/semantic/accessors.go` — full set of narrow accessor interfaces (StoreAccessor, SchedulerAccessor, RetrievalAccessor, etc.)
- `internal/skill/semantic/register.go` — `RegisterAll` pattern + per-tool register helpers
- `internal/skill/semantic/tools_context.go` (lines 1-320) — closest analog to all three new tools
- `internal/skill/semantic/tools_refresh.go` (header + grep-gate doc) — D-09 read-only invariant pattern
- `internal/skill/semantic/tools_refresh_test.go:116-123` — recorder-style D-09 test gating
- `internal/skill/semantic/skill.go` — `SemanticSkill` struct + post-init setters
- `internal/semantic/integ/lookup.go` — full `SemanticLookup` interface contract; confirms `SymbolID` takes (path, line, col), NOT (path, name)
- `internal/semantic/integ/status.go` — `SemanticStatus` struct: has GraphVersion + LatestSnapshotID; lacks `extractor_run_id` and `as_of_unix_ms`
- `internal/semantic/retrieval/rrf.go` — `Fuse` signature, RRF formula, 3-key tiebreak
- `internal/semantic/store/effective_graph.go` (lines 480-590) — `QuerySymbolPath`, `QuerySymbolByLocation`; NO `QuerySymbolByName`
- `internal/semantic/types/ladder.go` — 7-tier confidence ladder constants
- `internal/semantic/types/golang/resolver.go:45-80` — `RESOLVES_TO` edge emission + LSP source string format
- `internal/semantic/graph/repair.go:13` — `EdgeKind` field is `string` with informal `e.g.,` doc comment
- `internal/semantic/store/overlay.go:792-870` — `semantic_cluster_members` row shape and SQL
- `.planning/phases/64-new-mcp-tools/64-07-PLAN.md` — confirms "Phase 64 P07" is the bleve recovery plan, not a multi-language fixture
- `.planning/phases/69-production-status-accessors/69-CONTEXT.md` — three-state cluster status enum

### Secondary (MEDIUM confidence)
- `.planning/REQUIREMENTS.md` lines 40-50 + 99-107 — P1TOOL requirement text and phase ownership
- `.planning/ROADMAP.md` lines 157-251 — Phase 71 success criteria

### Tertiary (LOW confidence — flagged for plan-time confirmation)
- CI grep-gate runner location (Open Question 1)
- Cluster-engine per-symbol membership accessor cost (Open Question 2)
- AST citation metadata availability on edges (Open Question 3)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every package and seam was opened and read in this session
- Architecture: HIGH — file layout mirrors Phase 64 exactly with three additive handlers
- Pitfalls: HIGH — Pitfall 1 (race), 2 (RESOLVES_TO dominance), 3 (cluster gv mismatch), 5 (TYPES-04 vs additive) are derived from reading the actual code
- Test surface: HIGH — Go-idiomatic, no new framework
- Open question resolutions: MEDIUM — three flagged for plan-time confirmation

**Research date:** 2026-05-17
**Valid until:** 2026-06-17 (stable in-tree code; valid as long as Phase 64 envelope contract holds and Phase 69 ships D1 status enum)

## RESEARCH COMPLETE

**Phase:** 71 — p1-single-symbol-read-tools
**Confidence:** HIGH

### Key Findings
- **No `(file, name) → SymbolID` resolver exists today.** `integ.SemanticLookup.SymbolID` takes `(file, line, col)`; `*Store` has `QuerySymbolByLocation` and `QuerySymbolPath` but no `QuerySymbolByName`. Plan 71-01 must add this accessor.
- **`extractor_run_id` does not exist anywhere.** Must be added (new column or in-memory monotonic counter on `*Store`) and exposed via a new accessor. `as_of_unix_ms` is trivial (`time.Now().UnixMilli()`).
- **Retrieval fusion is essentially solved.** `retrieval.Fuse` + `PersonalizedPageRank` already handle the find_related ranking path; cluster co-membership boost is the only new fusion piece.
- **Edge-kind vocabulary is informal.** Only `RESOLVES_TO` is consistently emitted by name; `CALLS` / `USES_TYPE` appear in doc-comments. Surface mapping is a planner-decided fixed table with a bounded-label metric for unmapped kinds.
- **The "Phase 64 P07 populated-graph fixture" is a misnomer** — that plan is the bleve recovery work, not a multi-language fixture. A multi-language graph fixture must be built fresh in Phase 71.

### File Created
`.planning/phases/71-p1-single-symbol-read-tools/71-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|---|---|---|
| Standard Stack | HIGH | All packages opened and read |
| Architecture | HIGH | Phase 64 P0 pattern verified line-by-line |
| Pitfalls | HIGH | All five derived from actual code, not training-data guess |
| Seed resolution | HIGH | Interface contract read in full; confirms NEW accessor is needed |
| Edge-kind mapping | MEDIUM | Confirmed informal vocabulary; mapping table is planner judgment |
| Test fixture | HIGH | Confirmed no existing multi-language fixture |

### Open Questions
1. CI grep-gate runner location — locate in plan 71-01.
2. Cluster co-membership boost: per-symbol accessor cost, in scope for 71 or deferred to 72.
3. AST citation metadata availability on edges for `validate_graph_edge`.

### Ready for Planning
Research complete. Planner can now structure 71 as 5 plans:
- **71-01** — Seam additions: `QuerySymbolByName` on `*Store`, `LatestExtractorRunID` accessor, `ClusterMembershipAccessor.ClusterIDOf`, new `SymbolByNameAccessor` / `ExtractorRunAccessor` / `ClusterMembershipAccessor` interfaces in `accessors.go`, post-init setters in `skill.go`.
- **71-02** — Edge-kind surface enum + mapper + envelope V2 (Freshness with snapshot_id/extractor_run_id/as_of_unix_ms/status).
- **71-03** — `explain_symbol_deep` (handler + register + tests + help const).
- **71-04** — `find_related_symbols` (handler + register + tests + cluster-boost wiring) + multi-language populated-graph fixture (consumed by 71-03/04/05 tests).
- **71-05** — `validate_graph_edge` (handler + register + tests) + cross-tool integration test + grep-gate extension for the three new handler files.
