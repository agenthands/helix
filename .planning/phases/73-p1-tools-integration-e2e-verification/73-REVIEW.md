---
phase: 73-p1-tools-integration-e2e-verification
reviewed: 2026-05-21T00:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - internal/profile/modes/edit.yaml
  - internal/profile/modes/read.yaml
  - internal/skill/semantic/export_p1_test.go
  - internal/skill/semantic/p1_e2e_external_test.go
  - internal/skill/semantic/profile_filter_test.go
  - internal/skill/semantic/skill.go
  - internal/skill/semantic/tool_help_test.go
  - internal/skill/semantic/tools_change_impact.go
  - internal/skill/semantic/tools_cluster_map.go
  - internal/skill/semantic/tools_explain_cluster.go
  - internal/skill/semantic/tools_explain_symbol.go
  - internal/skill/semantic/tools_find_related.go
  - internal/skill/semantic/tools_validate_edge.go
  - internal/skill/semantic/wrapper_consistency_test.go
findings:
  critical: 0
  warning: 5
  info: 6
  total: 11
status: issues_found
---

# Phase 73: Code Review Report

**Reviewed:** 2026-05-21T00:00:00Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

Phase 73 exposes 6 P1 MCP tools (`explain_symbol_deep`, `find_related_symbols`,
`validate_graph_edge`, `get_cluster_map`, `explain_cluster`,
`get_change_impact_graph`) to profile filtering by extending `SemanticSkill.Tools()`,
standardizes help-text references, and adds golden / static-gate / E2E tests.

The change set is correctly scoped and `go vet` + `go test ./internal/skill/semantic/...`
both pass. However, the review surfaces several real defects: a node-cap /
edge-cap interaction bug in `get_change_impact_graph` that produces a wrong
`edges_count` and silently drops edges, a test-coverage hole where the E2E
golden test for `get_change_impact_graph` accepts a degenerate empty subgraph
as a pass, a wrong tool-count claim in `SemanticSkill.Description()`, and several
correctness/consistency issues in the cluster-map and explain-cluster handlers
that the new tests do not catch because the proxy implementations are intentionally
degenerate.

No security vulnerabilities or data-loss risks were found — all handlers honour
the documented D-09/D-13 read-only invariant and `validate_graph_edge` /
`find_related_symbols` both perform input validation (closed-enum, path traversal).

## Warnings

### WR-01: `get_change_impact_graph` `edges_count` is wrong and edges are silently dropped when node cap fires

**File:** `internal/skill/semantic/tools_change_impact.go:259-304`
**Issue:** Step 8 computes `edgesCount` by iterating **all** `impacts` (pre-node-cap),
but step 9 (`rawEdges`) only collects edges from `cappedImpacts` (post-node-cap,
truncated at `impactNodeCap`). When `nodesCount > impactNodeCap`, `EdgesCount`
reports edges that belong to dropped nodes and can never appear in `Edges`.
The result then advertises `edges_count` far higher than the edges it actually
returns, and `Truncated` is the only signal the caller gets — there is no way
to tell whether truncation was node-driven or edge-driven. The help text
(line 142) documents `edges_count` as "Pre-cap total edge count", but a caller
cannot reconcile that number with the returned `edges` slice because the slice
was additionally filtered by the node cap, not just the edge cap. This is a
correctness defect in the response contract.

**Fix:** Either (a) compute `edgesCount` over `cappedImpacts` so it matches the
node-cap scope and only reflects the edge cap, or (b) keep the pre-cap total but
add an explicit `edges_dropped_by_node_cap` / separate `truncation_reason` field
so the count is interpretable. Minimal fix for (a):
```go
// 8. Count pre-cap totals over the node-capped set so edges_count is
//    reconcilable with the returned edges slice.
cappedImpacts := impacts
if len(cappedImpacts) > impactNodeCap {
    cappedImpacts = cappedImpacts[:impactNodeCap]
}
nodesCount := len(impacts)
edgesCount := 0
for _, imp := range cappedImpacts {
    for _, e := range imp.Evidence.Edges {
        if allowedInternalKinds == nil {
            edgesCount++
        } else if _, ok := allowedInternalKinds[e.Kind]; ok {
            edgesCount++
        }
    }
}
```

### WR-02: E2E golden test for `get_change_impact_graph` accepts a degenerate empty result as PASS

**File:** `internal/skill/semantic/p1_e2e_external_test.go:783-815`
**Issue:** `TestP1E2E_GetChangeImpactGraph` only asserts the envelope is valid
and that `fallback_reason != "impact_lookup_unavailable"`. It never asserts
`nodes` or `edges` is non-empty. The seed `p1SeedGoServeHTTP` has a
`ServeHTTP → handle` CALLS edge in the fixture (`edges` slice, line 196), so a
correct impact expansion at depth 2 must return at least one node/edge. As
written the test passes even if `ExpandFrom` returns an empty `[]integ.Impact`
— which is exactly the kind of regression an E2E test is supposed to catch.
Compare with `TestP1E2E_GetClusterMap` (line 753-754) which *does* assert
`total_clusters > 0`. This is an adversarial-review concern: the test validates
"a response came back" rather than "the right response came back".

**Fix:** Add a non-degeneracy assertion, e.g.:
```go
nodes, _ := body["nodes"].([]interface{})
require.Greater(t, len(nodes), 0,
    "get_change_impact_graph: seeded ServeHTTP->handle edge must yield >0 impact nodes")
```
If `ExpandFrom` legitimately cannot expand from this seed against the real
`*Store` (e.g. because the integ lookup needs a different edge kind than the
fixture's `"call_graph"`), that is itself a fixture bug worth surfacing now
rather than hiding behind a permissive assertion.

### WR-03: `SemanticSkill.Description()` claims 10 tools but the skill now exposes 16

**File:** `internal/skill/semantic/skill.go:75-77` (also doc comments at lines 1-3, 22-24, 312)
**Issue:** Phase 73-01 appends 6 P1 tools to `Tools()` (now returns 16 entries),
but `Description()` still returns `"Semantic graph indexing and retrieval (10 tools)"`.
The package doc comment (lines 1-3) and the `SemanticSkill` struct doc (lines 22-24)
also still say "10 MCP tools" / "four MCP tools". `Description()` is user-facing
(surfaced through the skill registry), so this is a wrong fact shipped to the
agent, not just a stale comment. The `Tools()` doc comment at line 312 also still
says "10 MCP tool definitions" while the body now returns 16.

**Fix:** Update `Description()` to `"Semantic graph indexing and retrieval (16 tools)"`
and correct the three doc comments. Consider deriving the count from `len(s.Tools())`
to prevent future drift.

### WR-04: `get_cluster_map` `members_preview` / `representative_symbols` are decimal node-ID strings, not symbol IDs — contract violation

**File:** `internal/skill/semantic/tools_cluster_map.go:260-294, 365-389`
**Issue:** The response shape documents `members_preview` and
`representative_symbols` as `[]string` of `symbol_id` strings (struct comments
lines 52-57, help text lines 100-101). The handler instead populates them with
`nodeIDToSymbolIDString(r.nodeID)` which returns the **decimal node ID** (e.g.
`"1001"`), not a stable `symbol_id` such as `"repo/src/svc.go::ServeHTTP"`.
Worse, the `nodeIDs` passed to `QueryNodePageRanks` is `[]uint64{row.ClusterIntID}`
— the cluster's integer ID used as a proxy node — so even the ranking input is
semantically wrong. The inline comments (lines 256-293) openly acknowledge this
is a "proxy approach" deferred to a later wave, but Phase 73 ships these tools to
the profile surface and adds an E2E test (`TestP1E2E_GetClusterMap`) that asserts
only `total_clusters > 0` and never inspects `members_preview` content — so the
incorrect field values are now reachable by real agents with no test guarding the
contract.

**Fix:** Either resolve node IDs to real `symbol_id` strings via
`ClusterMemberAccessor` (which is wired in `explain_cluster` and available here),
or — if the proxy is a deliberate phase-staged decision — change the JSON field
docs/help text to state the values are opaque pending the cluster-member wiring,
and add an explicit test asserting the documented-vs-actual shape so the gap is
tracked. Shipping a tool whose documented `symbol_id` field returns a bare
integer is a contract defect.

### WR-05: `computeDominantEdgeKinds` always returns an empty slice — `dominant_edge_kinds` is dead output

**File:** `internal/skill/semantic/tools_cluster_map.go:335-363`
**Issue:** `computeDominantEdgeKinds` unconditionally returns an empty
`[]EdgeKindSurface{}` (all three parameters are discarded via `_ = ...`). The
expensive `QueryEffectiveAdjacency` call at lines 237-248 (explicitly flagged
`O(E)` by the OQ-3 TODO) produces `adjOut`, which is then passed only to this
function that ignores it. So `get_cluster_map` pays the full `O(E)` adjacency
scan on every call to feed a function that always returns nothing, and every
`ClusterSummaryEntry.DominantEdgeKinds` is empty. The handler comment at
lines 344-358 documents this as intentional ("the adjacency does not encode
kind") — but then the `QueryEffectiveAdjacency` call is pure wasted work and
`dominant_edge_kinds` is a permanently-empty field that the help text (line 102)
advertises as populated.

**Fix:** If kind data is genuinely unavailable until a `ClusterEdgesAccessor`
exists, delete the `QueryEffectiveAdjacency` call and the `adjOut` plumbing from
`get_cluster_map` so it does not pay `O(E)` for nothing, and mark
`dominant_edge_kinds` as "not yet populated" in the help text. Keep the field in
the struct for forward compatibility but stop doing the work that produces no
value. (Note: `explain_cluster`'s `computeClusterMetrics` has the same
empty-`dominantKinds` return at lines 356-392 — same remediation applies there.)

## Info

### IN-01: `explain_cluster` passes `limit=0` to `QueryClusterMembers` then truncates manually — relies on undocumented accessor behavior

**File:** `internal/skill/semantic/tools_explain_cluster.go:235, 246-249`
**Issue:** Step 6 calls `QueryClusterMembers(..., 0)` with a literal `0` limit and
the comment at the handler header (line 169) says "with limit". The handler then
records `memberCount` and truncates `memberRows` to `limit` itself (lines 244-249).
This works only if the store treats `limit == 0` as "no limit" (unbounded). That
contract is not asserted anywhere in this phase's tests, and if a future store
revision treats `0` as "return zero rows", `explain_cluster` silently returns an
empty member list with `member_count == 0`. The fixed `0` is effectively a magic
number standing in for "unbounded".

**Fix:** Either pass an explicit large bound (e.g. `explainClusterMaxMembers`)
or add a named const `clusterMembersUnbounded = 0` with a comment pinning the
store contract, and add a store-level test asserting `limit == 0` means unbounded.

### IN-02: `errAs` helper in `tools_explain_cluster.go` is effectively dead — both branches return identically

**File:** `internal/skill/semantic/tools_explain_cluster.go:194-201, 397-406`
**Issue:** In `handleExplainCluster` the `decodeClusterID` error handling does:
```go
var sErr *serr.Error
if ok := errAs(err, &sErr); ok {
    return errorResult(err.Error())
}
return errorResult(err.Error())
```
Both the `if` branch and the fall-through return `errorResult(err.Error())` — the
`errAs` call, the `sErr` variable, and the branch are pure dead code. `sErr` is
never read. This is harmless today but is misleading: a reader assumes the
`*serr.Error` path does something distinct. `go vet` does not flag it because
`sErr` is "used" by being passed to `errAs`.

**Fix:** Collapse to `return errorResult(err.Error())` and delete the unused
`errAs` helper (it has no other caller in the package — verify with grep before
removal).

### IN-03: `explain_cluster` `members_returned` is set from post-sort `len(members)`, not the cap — correct but fragile

**File:** `internal/skill/semantic/tools_explain_cluster.go:268-281, 335`
**Issue:** `MembersReturned: len(members)` happens to equal the capped count
because `members` is built one-to-one from the already-truncated `memberRows`.
This is correct, but the relationship is implicit — a future edit that filters
`members` after construction (e.g. dropping zero-PageRank entries) would silently
desynchronize `members_returned` from `len(Members)`. The field name promises
"count included in this response" which should always be `len(result.Members)`.

**Fix:** Set `MembersReturned: len(members)` immediately adjacent to assigning
`Members: members`, or compute it as `len(result.Members)` after struct
assembly, so the invariant is locally obvious.

### IN-04: `intToDecimalString` reinvents `strconv.FormatUint` to avoid a non-existent import cycle

**File:** `internal/skill/semantic/tools_cluster_map.go:374-389`
**Issue:** The comment justifies the hand-rolled uint64→string conversion as
avoiding "import cycle risk" with `strconv`. `strconv` is a leaf standard-library
package and cannot create an import cycle with anything. The function is correct
but is unnecessary reinvention of `strconv.FormatUint(n, 10)`, adding ~12 lines
of buffer arithmetic that a reviewer must verify by hand. (This is partly moot if
WR-04 is fixed by removing the proxy path entirely.)

**Fix:** Replace `intToDecimalString` / `nodeIDToSymbolIDString` internals with
`strconv.FormatUint(nodeID, 10)`.

### IN-05: `wrapper_consistency_test.go` `readAll` duplicates `os.ReadFile` / `io.ReadAll`

**File:** `internal/skill/semantic/wrapper_consistency_test.go:135-144`
**Issue:** `readAll` scans the file line-by-line with `bufio.Scanner` and
re-joins with `\n` purely to get the file content as a string. `os.ReadFile`
returns `[]byte` directly and is simpler and faster. The line-by-line
reconstruction also normalizes away a trailing-newline difference (appends `\n`
after every line including the last), which is harmless for a `strings.Contains`
gate but is a subtle behavior the reader must reason about.

**Fix:** Replace the `readAll` body with `b, err := os.ReadFile("register.go")`
and use `string(b)`; delete the helper.

### IN-06: `p1ValidFallbackReasons` includes `cluster_member_unavailable` which no handler emits, and omits `evidence_lookup_lagging` / `evidence_lookup_unavailable` / `stale_cluster_id`

**File:** `internal/skill/semantic/p1_e2e_external_test.go:575-584`
**Issue:** The closed-enum allow-set used by `assertP1Envelope` lists
`cluster_member_unavailable`, but no reviewed handler emits that string
(`explain_cluster` degrades silently to an empty member list with no fallback
reason; `get_cluster_map` emits `cluster_map_unavailable`). Conversely the set
is missing three reasons that the reviewed handlers *do* emit:
`validate_graph_edge` emits `evidence_lookup_lagging`
(`tools_validate_edge.go:489`) and `evidence_lookup_unavailable`
(`tools_validate_edge.go:383, 400`), and `explain_cluster` emits
`stale_cluster_id` (`tools_explain_cluster.go:218`). Because the E2E tests only
exercise happy-path inputs, none of these gaps fail today — but the allow-set is
advertised (file header lines 24-28) as "the union of all declared
fallback_reason values across the 6 P1 tools", which it is not. A future
negative-path test added against this helper would spuriously fail on a
legitimate `evidence_lookup_lagging` response.

**Fix:** Reconcile `p1ValidFallbackReasons` with the actual closed enums:
remove `cluster_member_unavailable` (or grep-confirm a handler that emits it),
and add `evidence_lookup_lagging`, `evidence_lookup_unavailable`, and
`stale_cluster_id`.

---

_Reviewed: 2026-05-21T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
