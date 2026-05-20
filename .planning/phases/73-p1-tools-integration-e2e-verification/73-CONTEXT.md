# Phase 73: P1 Tools Integration & E2E Verification - Context

**Gathered:** 2026-05-20
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase makes the 6 P1 MCP tools — `explain_symbol_deep`, `find_related_symbols`,
`validate_graph_edge` (Phase 71) and `get_cluster_map`, `explain_cluster`,
`get_change_impact_graph` (Phase 72) — **provably integrated**: profile/mode gated
across the full 5 profiles × 4 modes matrix, uniformly wrapped in the semantic
skill, documented via `get_tool_help`, and verified end-to-end against a real
populated workspace.

It is a **verification and integration phase**, not a feature phase. No new tools,
no new handler behavior, no new accessors. The deliverables are: golden tests,
help-doc standardization, a static consistency gate, and a real-store E2E suite.
All tool semantics are locked by Phase 71 and Phase 72 CONTEXT.md.

</domain>

<decisions>
## Implementation Decisions

### Profile-filter golden test (SC#1)
- **D-01:** Extend the existing inline table-driven pattern in
  `internal/skill/semantic/profile_filter_test.go` to cover the 6 P1 tools across
  the full 5×4 matrix. Add a `p1ToolNames` slice and assert visibility per
  `(profile, mode)` cell using the existing `hasAll` / `hasNone` helpers and
  `resolveForProfileMode`. **No** `testdata/` snapshot files, **no** single
  committed golden JSON — consistency with the P0 test (`semanticToolNames`)
  matters more, and Go-readable diffs survive matrix changes better than JSON
  blobs.

### E2E populated-workspace fixture (SC#4)
- **D-02:** Build one shared real-store fixture — `buildP1E2EFixture(t)` — in
  `package semantic_test`, extending the existing `productionAdapterFixture`
  (`production_adapter_e2e_test.go`). It provides a real `*Store` + bleve in a
  tempdir, populated with the multi-language symbol/edge/cluster set all 6 tools
  need. All 6 tool E2E tests reuse this single builder. **Not** per-tool tempdir
  setup (duplicates snapshot/bleve wiring 6× and risks drift); **not** mutating
  `productionAdapterFixture` in place (that would couple Phase 65 tests to
  Phase 73 data). This honors the established "DO NOT inline-copy this fixture"
  norm from `populated_graph_fixture_test.go`.
- **D-02a:** SC#4 explicitly demands a **real `*Store` + bleve + tempdir** — the
  existing `populated_graph_fixture_test.go` is an in-memory mock and does NOT
  satisfy SC#4. The mock fixture stays for unit tests; the E2E suite uses the new
  real-store builder.

### get_tool_help (SC#2)
- **D-03:** Standardize all 6 P1 `*Help` consts to a fixed section template
  (Usage Examples / Parameters / Returns) so `get_tool_help` output is uniform
  across the P1 tool set, then add coverage tests asserting `get_tool_help`
  returns non-empty parameter documentation for each of the 6 tools. Param docs
  are extracted via `jsonschema.For[T]` from the typed args, exactly as Phase 64
  P0 tools do — that mechanism is locked by SC#2 and is not re-litigated.
- **D-03a:** The 6 P1 tools already carry hand-written `*Help` consts
  (`explainSymbolDeepHelp`, `getClusterMapHelp`, etc.) used in their `register*`
  functions. Phase 73 normalizes their section layout; it does not invent new
  help content from scratch.

### Skill-wrapper consistency enforcement (SC#3)
- **D-04:** Add a new in-tree static gate — `wrapper_consistency_test.go` — in the
  same style as the existing `readonly_gate_test.go`: a source-scan test over all
  6 P1 handler files asserting each (a) calls the mode-check helper, (b) emits a
  `FreshnessV2` envelope, and (c) is registered via `RegisterAll`. The repo has
  **no external CI linter** (established by 71-01), so the standing in-tree test
  is the enforcement mechanism — it makes the SC#3 invariant resistant to future
  refactors. **Not** a manual VERIFICATION.md checklist (catches nothing on
  future drift).
- **D-04a:** The existing `readonly_gate_test.go` `gatedHandlerFiles` list
  **already covers all 6 P1 handlers** (Phase 72 extended it). No further
  extension of the read-only gate is needed — Phase 73 only adds the new
  wrapper-consistency gate alongside it.

### Claude's Discretion
- Exact section headings/wording of the standardized `*Help` template (D-03) —
  planner picks, as long as all 6 are identical in structure.
- The precise multi-language symbol/edge/cluster shape of `buildP1E2EFixture`
  (D-02) — must be rich enough that every one of the 6 tools returns a
  non-degenerate result with all closed-enum envelope fields populated.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 73 anchors
- `.planning/ROADMAP.md` — Phase 73 entry (Goal, 5 Success Criteria, Depends on: 71, 72)
- `.planning/REQUIREMENTS.md` — P1TOOL-07 (profile/mode gating + get_tool_help),
  P1TOOL-08 (semantic-skill wrapping, zero bootstrap special-casing),
  P1TOOL-09 (E2E verification, closed-enum envelope fields)

### Inherited tool contracts (locked — do NOT re-litigate)
- `.planning/phases/71-p1-single-symbol-read-tools/71-CONTEXT.md` — envelope,
  freshness shape (`FreshnessV2`), edge-kind enum, mode-check pattern, test surface
- `.planning/phases/71-p1-single-symbol-read-tools/71-VERIFICATION.md` — verified
  patterns to mirror
- `.planning/phases/72-p1-cluster-impact-tools/72-CONTEXT.md` — cluster_id codec,
  cluster/impact tool shapes, confidence-cap, D5 inheritance table
- `.planning/phases/72-p1-cluster-impact-tools/72-VERIFICATION.md` — Phase 72 ship gate

### Profile / mode matrix
- `internal/profile/` — 5 profiles × 4 modes; `read+` / `review+` tiers
- `internal/profile/modes/{read,edit,review,admin}.yaml` — per-mode tool exclusions
- `internal/skill/semantic/profile_filter_test.go` — existing P0 golden test to extend (D-01)

### Skill wrapping + tool registration
- `internal/skill/semantic/skill.go` — `SemanticSkill.Tools()` ToolProvider list
  (currently returns only the 4 P0 tools — see Integration Points below)
- `internal/skill/semantic/register.go` — `RegisterAll` registers all 10 tools
  (4 P0 + 6 P1) via `register*` functions
- `internal/skill/semantic/envelope.go`, `mode_check.go`, `accessors.go` — the
  v1.10 wrapping pattern SC#3 requires uniform compliance with
- `internal/skill/semantic/tools_{explain_symbol,find_related,validate_edge,cluster_map,explain_cluster,change_impact}.go`
  — the 6 P1 handler files (each carries a `*Help` const)

### Static gates + E2E fixtures
- `internal/skill/semantic/readonly_gate_test.go` — existing in-tree static gate;
  `gatedHandlerFiles` already lists all 6 P1 handlers. Style template for the new
  wrapper-consistency gate (D-04)
- `internal/skill/semantic/production_adapter_e2e_test.go` — `productionAdapterFixture`
  (real `*Store` + bleve + tempdir) to extend into `buildP1E2EFixture` (D-02)
- `internal/skill/semantic/status_e2e_external_test.go` — Phase 69 real-store E2E
  pattern (`package semantic_test`, daemon factory accessors)
- `internal/skill/semantic/populated_graph_fixture_test.go` — in-memory MOCK
  fixture; used for unit tests, NOT sufficient for SC#4 (see D-02a)
- `internal/skill/semantic/integration_test.go` — existing cross-tool integration tests

### Build / vet gates (SC#5)
- `vet-nokernel2semantic` and `vet-noduckdb` — must stay green
- Race-clean under `go test -race -count=1`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `profile_filter_test.go` — `resolveForProfileMode`, `hasAll`, `hasNone`,
  `semanticToolNames`, `readPlusOnlyTools`: directly extensible for the 6 P1 tools.
- `productionAdapterFixture` (`production_adapter_e2e_test.go`) — real `*Store` +
  bleve + tempdir setup; the seed for `buildP1E2EFixture`.
- `readonly_gate_test.go` — line-by-line comment-stripping source scanner; copy
  its structure for `wrapper_consistency_test.go`.
- Each P1 handler already has a `*Help` const and `jsonschema.For[T]`-typed args —
  Phase 73 standardizes, it does not author from zero.

### Established Patterns
- In-tree static gates (not external CI linters) are the repo's enforcement
  mechanism for cross-file invariants (71-01 established no CI runner exists).
- "DO NOT inline-copy this fixture" — shared fixture builders, invoked per test,
  are the norm (`buildPopulatedGraphFixture`).
- Black-box `package semantic_test` is required for any test that imports
  `internal/daemon` (production-adapter constructors) to avoid an import cycle.

### Integration Points
- **`SemanticSkill.Tools()` (`skill.go:325`) currently returns only the 4 P0
  tools** — the 6 P1 tools are registered directly with the server via
  `RegisterAll`/`register*` but are NOT in the `Tools()` ToolProvider list.
  Profile filtering (`skill.ResolveTools`) operates on the ToolProvider surface.
  Planner/researcher MUST determine whether the 6 P1 tools need to be added to
  `Tools()` for SC#1 profile-filter coverage to be meaningful, or whether they
  flow through a different resolution path. This is the most likely real gap
  blocking SC#1.

</code_context>

<specifics>
## Specific Ideas

- Golden test must assert **no leakage across modes** — e.g., a `review+` tool
  must not surface in `read` mode for any profile (mirror the existing
  `TestProfileFilter_ReadMode_IndexExcluded` style).
- E2E suite must assert the closed-enum envelope fields `freshness`, `source`,
  `fallback_reason`, `confidence` are present AND within their declared enums for
  each of the 6 tools — enum membership, not just non-nil.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. Phase 73 is verification/integration
only; any new tool behavior, new accessors, or hypothetical-edit metadata remain
deferred per the Phase 72 CONTEXT.md "Deferred Ideas" section.

</deferred>

---

*Phase: 73-p1-tools-integration-e2e-verification*
*Context gathered: 2026-05-20*
