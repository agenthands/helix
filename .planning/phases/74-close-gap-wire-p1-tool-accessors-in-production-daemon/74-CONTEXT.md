# Phase 74: Close gap — wire P1 tool accessors in production daemon - Context

**Gathered:** 2026-05-29
**Status:** Ready for research → planning

<domain>
## Phase Boundary

This phase closes the v1.11 milestone gap surfaced by `.planning/v1.11-MILESTONE-AUDIT.md`:
all 10 Phase 71/72 P1 accessor setters have zero non-test callers in the production
daemon (`internal/daemon/semantic_wiring.go:252-268` wires exactly 8 P0 setters and
logs `setters: 8`). The 6 P1 MCP tools (`explain_symbol_deep`, `find_related_symbols`,
`validate_graph_edge`, `get_cluster_map`, `explain_cluster`, `get_change_impact_graph`)
are registered and visible in `tools/list` but are non-functional against any real
workspace — every handler hits its nil-accessor guard and returns a degraded
`fallback_reason: *_unavailable` envelope.

It is a **production-wiring phase**, not a feature phase. No new tools, no new
handler semantics, no new envelope fields. Deliverables: production accessor
adapters at `internal/daemon/semantic_wiring.go`, `Set*` calls on `b.skill`, an
updated `setters:` log line, a runtime bootstrap test that asserts wired accessors
are non-nil, and a lightweight production-path E2E that drives P1 handlers
against a real `semanticBundle`.

Scope of "which of the 10 accessors are wired" is partially gated on research
(see Decisions D-01 / D-01a). All other decisions are locked.

</domain>

<decisions>
## Implementation Decisions

### Scope of accessor wiring (D-01)
- **D-01 (locked, 6 accessors):** Phase 74 wires the **6 accessors that already
  have production `*Store` query methods** — these are ready to land:
  - `SetSymbolByName` ← thin adapter over `*Store.QuerySymbolByName`
  - `SetExtractorRun` ← thin adapter over `*Store.LatestExtractorRunID` (also closes BLOCKER-2; see D-02)
  - `SetClusterMap` ← thin adapter over `*Store.QueryClusterSummaries`
  - `SetClusterMember` ← thin adapter over `*Store.QueryClusterMembers`
  - `SetClusterPageRank` ← thin adapter over `*Store.QueryNodePageRanks`
  - `SetImpactLookup` ← existing `*integSemanticLookup` (`semantic_wiring.go:782`)
    already satisfies `ImpactLookupAccessor.ExpandFrom` + `Status`; only the
    setter call is missing.
- **D-01a (research-gated, 4 accessors):** The remaining 4 accessors have
  **no production `*Store` query layer** — only test fakes:
  - `TypeChainAccessor.TypeChainForSymbol`
  - `SymbolEdgesAccessor.CallersOf / IncomingEdgesOf / OutgoingEdgesOf`
  - `EdgeEvidenceAccessor.EvidenceForEdge`
  - `ClusterMembershipAccessor.ClusterIDOf`

  The researcher MUST map the `*Store` schema (`internal/semantic/store/`),
  the Phase 71 test fakes that drive these in `tools_*_test.go`, and the
  underlying tables / SQL primitives needed. The researcher's output decides
  whether Phase 74 folds the four queries in (one larger phase, complete
  audit closure) or splits them into a Phase 75 (smaller phases, deferred
  closure of `explain_symbol_deep` / `validate_graph_edge` /
  `find_related_symbols` cluster boost). Handlers already degrade gracefully
  via `fallback_reason` envelopes when these accessors are nil, so deferral
  is safe.

### FreshnessV2 / SetExtractorRun (D-02)
- **D-02:** Wire `SetExtractorRun` against a thin adapter over
  `*Store.LatestExtractorRunID(ctx, repoID)`. That is the same source the
  Phase 71-01 test fakes consume; `effective_graph_test.go:1421`
  (`TestLatestExtractorRunID_PopulatedSnapshot`) proves it returns a
  non-empty id derived from the most-recent committed snapshot. **Not** an
  IndexRunner-cached run id — that would introduce in-memory cache
  divergence from `*Store` and re-litigate Phase 71-01's seam choice.
- **D-02a:** This single setter closes BLOCKER-2 from
  `v1.11-MILESTONE-AUDIT.md`. After D-02 lands, `assembleFreshness`
  observes a non-empty `runID` and FreshnessV2 status can reach `current`
  for all 6 P1 tools (the 4 in D-01a included — `SetExtractorRun` is
  independent of TypeChain / SymbolEdges / EdgeEvidence / ClusterMembership
  wiring).

### CI gate + production E2E (D-03)
- **D-03:** Add a runtime bootstrap test (not a static source-scan): the test
  constructs a real `semanticBundle` via the daemon factory path, executes the
  wiring block, and asserts every wired accessor on `b.skill` is **non-nil**
  after construction. This catches future deferral-style regressions
  ("setters: 8" silently going stale) at runtime, where a static scan would
  drift away from compiler reality.
- **D-03a:** Add a lightweight production-path E2E (not a full daemon boot):
  reuse the new bootstrap-test `semanticBundle` plus the Phase 73
  `buildP1E2EFixture` data shape; call each wired P1 handler's
  `Handle*ForTest` against the production `b.skill` (not test-only `Set*`
  fixtures) and assert closed-enum envelopes are populated and
  `fallback_reason` is empty for accessors wired in D-01. **Not** a full
  daemon boot + MCP-client invocation — that is heavier than the audit gap
  requires, may be flaky, and re-tests middleware concerns already covered
  by Phase 73.
- **D-03b:** For the 4 D-01a accessors that may stay nil (depending on
  scope decision from research), the lightweight E2E asserts the
  documented `fallback_reason` is emitted — not silent absence. This makes
  the degradation contract part of the production gate.

### Production wiring location (D-04)
- **D-04:** Extend the existing wiring block at
  `internal/daemon/semantic_wiring.go:252-268`, adding the new `Set*` calls
  alongside the existing 8 P0 calls. Update the log line from
  `"setters", 8` to the new count (`14` if D-01 only; `18` if D-01a also
  lands). **Not** a separate `wireP1Accessors()` helper — the existing
  block is the single source of truth for "what the skill knows about" and
  splitting it muddies the audit-trail.
- **D-04a:** Each new adapter is a small struct with a `repoID` resolution
  helper consistent with the existing accessor pattern
  (`storeAccessor` / `schedulerAccessor` / `queueAccessor`). Use the same
  workspace-key → repoID translation already in `semantic_wiring.go`.

### Tech-debt deferrals (D-05)
- **D-05:** Phase 73 tech-debt items **WR-04** (`get_cluster_map`
  `representative_symbols` emit decimal node-IDs instead of stable symbol
  IDs) and **WR-05** (`computeDominantEdgeKinds` returns empty slice;
  needs a kind-aware `ClusterEdgesAccessor`) are **out of scope** for
  Phase 74. They are pre-existing Phase 72 handler bugs, not wiring gaps.
  Capture in Deferred Ideas. A future phase can address them once
  Phase 74 has stabilized the wiring surface.

### Claude's Discretion
- Exact adapter struct names and file placement (one file per adapter vs
  grouping by Phase 71/72 origin) — planner picks. Consistency with the
  existing `storeAccessor` / `schedulerAccessor` shape is the only
  constraint.
- Exact assertion style for the bootstrap-test non-nil gate — table-driven
  vs explicit subtests. Both work; planner picks for readability.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 74 anchors
- `.planning/v1.11-MILESTONE-AUDIT.md` — the audit that defines this phase.
  Required Remediation section lists the 5-step closure; D-01 / D-02 / D-03 /
  D-04 / D-05 each map back to items in that list.
- `.planning/ROADMAP.md` — Phase 74 entry (sparse: goal "To be planned",
  Requirements TBD, Depends on: Phase 73). Phase 74 inherits its goal from
  the audit, not the roadmap stub.

### Production wiring (the file Phase 74 edits)
- `internal/daemon/semantic_wiring.go:252-268` — the existing 8-setter block
  that Phase 74 extends. Pattern: `b.skill.Set*(b.<thing>Accessor())`.
- `internal/daemon/semantic_wiring.go:782` — `integSemanticLookup` struct
  (already implements `ImpactLookupAccessor.ExpandFrom` + `Status`; just
  needs the `SetImpactLookup` call).
- `internal/daemon/integ_lookup_export.go` — `NewIntegSemanticLookupForTest`
  test factory; production constructor pattern lives here.

### Accessor seams (declared, ready to consume)
- `internal/skill/semantic/accessors.go:189-412` — all 10 P1 accessor
  interfaces (and row types: `TypeChainRow`, `SymbolEdgeRow`,
  `EdgeEvidenceRow`, `EvidenceRange`, `ClusterSummaryRow`,
  `ClusterMemberRow`).
- `internal/skill/semantic/skill.go:150-237` — the 10 `Set*` methods Phase 74
  must call.

### Underlying `*Store` query layer
- `internal/semantic/store/effective_graph.go:595` — `QuerySymbolByName`
- `internal/semantic/store/effective_graph.go:650` — `LatestExtractorRunID`
- `internal/semantic/store/effective_graph.go:810` — `QueryClusterSummaries`
- `internal/semantic/store/effective_graph.go:855` — `QueryClusterMembers`
- `internal/semantic/store/effective_graph.go:908` — `QueryNodePageRanks`
- `internal/semantic/store/effective_graph_test.go:1421` —
  `TestLatestExtractorRunID_PopulatedSnapshot` proves D-02's source is
  populated under realistic data.

### Inherited tool contracts (locked — do NOT re-litigate)
- `.planning/phases/71-p1-single-symbol-read-tools/71-CONTEXT.md` — Phase 71
  accessor seams, envelope contract, FreshnessV2 shape.
- `.planning/phases/72-p1-cluster-impact-tools/72-CONTEXT.md` — Phase 72
  cluster/impact accessor seams, cluster_id codec.
- `.planning/phases/73-p1-tools-integration-e2e-verification/73-CONTEXT.md` —
  Phase 73 establishes the in-tree static gate pattern; D-03 picks a
  runtime gate alongside it, not replacing it.
- `.planning/phases/73-p1-tools-integration-e2e-verification/73-VERIFICATION.md` —
  proves test-fixture wiring works; Phase 74's E2E must drive the
  production wiring path instead.

### Handlers (read to confirm nil-guard / fallback_reason contract)
- `internal/skill/semantic/tools_explain_symbol.go`
- `internal/skill/semantic/tools_find_related.go`
- `internal/skill/semantic/tools_validate_edge.go`
- `internal/skill/semantic/tools_cluster_map.go`
- `internal/skill/semantic/tools_explain_cluster.go`
- `internal/skill/semantic/tools_change_impact.go`

### Build / vet gates
- `vet-nokernel2semantic`, `vet-noduckdb` — must stay green.
- Race-clean under `go test -race -count=1`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `b.storeAccessor()` / `b.schedulerAccessor()` / `b.queueAccessor()` etc.
  in `semantic_wiring.go` — the established adapter-construction pattern.
  New P1 adapters follow the same shape: small struct holding `b`, methods
  resolving workspace → repoID and delegating to `*Store`.
- `*integSemanticLookup` (`semantic_wiring.go:782`) — ALREADY implements
  the `ImpactLookupAccessor` method set (`ExpandFrom`, `Status`). Phase 74
  only needs to call `b.skill.SetImpactLookup(b.lookupAccessor)` where
  `b.lookupAccessor` is the existing struct.
- `*Store.LatestExtractorRunID` — production method exists; tested under
  populated and empty snapshots. D-02's adapter is a 5-line wrapper.
- `buildP1E2EFixture(t)` (`p1_e2e_external_test.go`) — populated workspace
  builder; D-03a reuses its data shape but drives the production skill
  instance, not test-only `Set*` calls.

### Established Patterns
- "Setters block" is the canonical wiring location — single block, single
  log line counting setters. Splitting it has been resisted across phases
  64 / 69 / 70.
- Narrow `*Accessor` interfaces, declared in `internal/skill/semantic/accessors.go`,
  with production adapters in `internal/daemon/`. Black-box wiring tests
  use `package semantic_test` to avoid import cycles (Phase 73 D-04a).
- Graceful degradation via `fallback_reason: "*_unavailable"` is the
  contract when an accessor is nil. Phase 74's D-03b makes this contract
  observable in the production E2E.

### Integration Points (and gaps)
- **Production gap:** 4 accessors have no `*Store` query layer — only
  test fakes inline in `tools_*_test.go`. Researcher MUST inventory the
  fakes, map to underlying schema, and decide whether to fold or defer
  (D-01a).
- **Workspace → repoID:** semantic_wiring.go has a stable translation
  already used by the 8 P0 adapters. P1 adapters reuse it; no new
  translation logic.
- **`SemanticSkill.Tools()`:** returns 10 entries today (4 P0 + 6 P1) —
  ToolProvider surface is correct; Phase 74 changes nothing about
  registration, only about data backing.

</code_context>

<specifics>
## Specific Ideas

- The non-nil bootstrap test (D-03) should NOT use reflection over the
  skill struct; instead, expose a method or test export
  (`*SemanticSkill.WiredAccessorsForTest()` or similar) returning a typed
  struct of bools, so the test is grep-able and stable under refactors.
  Pattern mirrors Phase 73's `Handle*ForTest` export approach.
- The lightweight E2E (D-03a) MUST cover at least one tool per direction
  partition for `SymbolEdgesAccessor` if D-01a folds — callers / incoming /
  outgoing are independent partitions and a single-direction test risks
  silently regressing the other two.
- Researcher (D-01a): produce a table per missing accessor with columns
  `(method, fake-impl location, *Store schema gap, estimated SQL
  complexity, freshness contract)`. Use this to recommend fold vs defer.

</specifics>

<deferred>
## Deferred Ideas

- **WR-04** (Phase 73 tech debt): `get_cluster_map`
  `representative_symbols` / `members_preview` emit decimal node-ID strings
  instead of stable symbol IDs (`tools_cluster_map.go:260-294, 365-389`).
  Likely a small fix once `QueryClusterMembers` results are properly
  threaded — but a Phase 72 handler bug, not a wiring bug. Future phase.
- **WR-05** (Phase 73 tech debt): `computeDominantEdgeKinds` permanently
  returns empty (`tools_cluster_map.go:335-363`). Requires a new kind-aware
  `ClusterEdgesAccessor` that surfaces edge labels alongside weights.
  Future phase.
- Phase 71 deferred bounded-label metric instrumentation
  (`edge_kind_surface.go:23, 89`) — still gated on `obs.Metrics` seam
  being wired into the semantic skill package. Future phase.
- Full daemon-boot + MCP-client E2E for P1 tools — useful for confidence,
  but heavier than Phase 74's audit-closure need. Future testing phase
  (or part of a release-gate harness) if flake exposure justifies.
- Closing the 4 missing-query accessors (D-01a) as a separate Phase 75 if
  the researcher's report recommends a split rather than a fold.

</deferred>

---

*Phase: 74-close-gap-wire-p1-tool-accessors-in-production-daemon*
*Context gathered: 2026-05-29*
