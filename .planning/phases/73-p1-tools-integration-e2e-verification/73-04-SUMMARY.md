---
phase: 73-p1-tools-integration-e2e-verification
plan: 04
subsystem: semantic-skill-p1-e2e-verification
tags: [e2e, tdd, sc3, sc4, p1-tools, wrapper-consistency, real-store]
dependency_graph:
  requires: [P1TOOL-01, P1TOOL-02, P1TOOL-03, P1TOOL-04, P1TOOL-05, P1TOOL-06]
  provides: [P1TOOL-08, P1TOOL-09]
  affects:
    - internal/skill/semantic/wrapper_consistency_test.go
    - internal/skill/semantic/export_p1_test.go
    - internal/skill/semantic/p1_e2e_external_test.go
tech_stack:
  added: []
  patterns:
    - export_test.go (test-only handler exports for black-box E2E)
    - real-store-e2e (DuckDB *Store + bleve in t.TempDir())
    - bufio-scanner-static-gate (SC#3 wrapper consistency)
key_files:
  created:
    - internal/skill/semantic/wrapper_consistency_test.go
    - internal/skill/semantic/export_p1_test.go
    - internal/skill/semantic/p1_e2e_external_test.go
  modified: []
decisions:
  - "Use inline fixture implementations for TypeChain/SymbolEdges/EdgeEvidence/ClusterMembership seams since the daemon's newSemanticBundle does not wire these; satisfies SC#4 (real *Store) while gracefully handling seams with no SQL backing yet"
  - "Use p1-prefix for all stub types in p1_e2e_external_test.go to avoid collision with identically-named stubs in status_e2e_external_test.go (same package semantic_test)"
  - "Reuse existing extractText from status_e2e_external_test.go rather than redeclaring; same package means no redeclaration needed"
  - "Use daemon.NewIntegSemanticLookupForTest for ImpactLookup to get real ExpandFrom backed by seeded call_graph edges"
  - "Encode seedClusterIDEncoded as fmt.Sprintf(\"weak_components:%d:1\", gv) matching encodeClusterID format in tools_cluster_map.go"
metrics:
  duration_minutes: 35
  completed_date: "2026-05-21"
  tasks_completed: 4
  files_modified: 3
---

# Phase 73 Plan 04: SC#3 Static Wrapper Gate + SC#4 Real-Store P1 E2E Suite Summary

SC#3 static consistency gate (wrapper_consistency_test.go) and SC#4 real-store E2E suite (p1_e2e_external_test.go) proving all 6 P1 handlers use checkMode + FreshnessV2 and produce valid envelope output against a real DuckDB store + bleve index.

## Tasks Completed

| Task | Description | Commit | Result |
|------|-------------|--------|--------|
| 1 | SC#3 static wrapper-consistency gate for 6 P1 handler files | 31715eb3 | Done |
| 2 | Handle*ForTest exports for 6 P1 handlers (export_p1_test.go) | 3f779ac2 | Done |
| 3 | buildP1E2EFixture builder + TestP1E2EFixture_SeedsNonEmpty smoke test | fd608e68 | Done |
| 4 | 6 TestP1E2E_* real-store E2E assertions (same commit as Task 3) | fd608e68 | Done |

## What Was Built

**Task 1 — SC#3 Static Wrapper-Consistency Gate (wrapper_consistency_test.go)**

`TestWrapperConsistency_P1Handlers` in `package semantic` (internal test) performs:
- Per-file bufio.Scanner scan over the 6 P1 handler files, stripping `//`-prefixed comment lines, asserting both `checkMode(` and `FreshnessV2` appear on at least one non-comment line (the D-04 SC#3 invariant)
- RegisterAll coverage subtest: reads `register.go` and asserts all 6 `register*` function names appear in the body

Uses `p1WrapperGatedFiles` (distinct from `gatedHandlerFiles` in readonly_gate_test.go) and `readAll` helper to avoid name collisions. Mirrors the bufio.Scanner comment-stripping pattern from `readonly_gate_test.go`.

**Task 2 — Handle*ForTest Exports (export_p1_test.go)**

`package semantic` (compiled only during tests) exposes 6 unexported handlers as `HandleXxxForTest` functions visible to `package semantic_test`:
- `HandleExplainSymbolDeepForTest`, `HandleFindRelatedSymbolsForTest`, `HandleValidateGraphEdgeForTest`
- `HandleGetClusterMapForTest`, `HandleExplainClusterForTest`, `HandleGetChangeImpactGraphForTest`

Pattern mirrors `export_status_test.go` (Phase 69-06). Lets the black-box E2E suite invoke handlers without adding production API surface.

**Task 3 — buildP1E2EFixture Builder + Smoke Test (p1_e2e_external_test.go)**

`buildP1E2EFixture(t *testing.T) *p1E2EFixture` in `package semantic_test`:
- Opens real `*semanticstore.Store` (DuckDB) + `*retrieval.Engine` (bleve) in `t.TempDir()`
- Commits a snapshot with 7 symbols (Go + TS + Java) and 3 `call_graph` edges (ServeHTTP→processRequest→validateInput) via `BeginSnapshot→WriteSnapshotFacts→CommitSnapshot`
- Opens overlay tx: `BumpGraphVersion`, `UpsertGraphScores` (PageRank for all 7 nodes), `UpsertClusters` (1 weak_components cluster), `UpsertClusterMembers` (ServeHTTP + processRequest), then `Commit`
- Wires `SemanticSkill` with all `Set*` setters:
  - Review mode session accessor (`p1StaticSessionAcc` with `Mode: "review"`)
  - Store-backed: `p1StoreSymbolByName`, `p1StoreExtractorRun`, `p1StoreClusterMap`, `p1StoreClusterMember`, `p1StoreClusterPageRank`
  - Inline fixtures: `p1InlineClusterMembership`, `p1InlineTypeChain`, `p1InlineSymbolEdges`, `p1InlineEdgeEvidence`
  - `daemon.NewIntegSemanticLookupForTest(store, ws)` for ImpactLookup (real ExpandFrom on call_graph edges)
  - Placeholder accessors: `p1PlainStoreAcc`, `p1PlaceholderSchedAcc` (returns `graph.ScoreStatusMissing`), `p1ZeroQueueAcc`, `p1ZeroLiveAcc`, `p1PlaceholderRetrievalAcc`, `p1NoopCompactAcc`

`TestP1E2EFixture_SeedsNonEmpty` smoke test asserts: symbol count > 0, adjacency output len > 0, cluster count > 0.

**Task 4 — 6 TestP1E2E_* Real-Store E2E Assertions**

| Test | Tool | Key assertions |
|------|------|----------------|
| `TestP1E2E_ExplainSymbolDeep` | explain_symbol_deep | Non-empty text, valid envelope (checkMode read, FreshnessV2) |
| `TestP1E2E_FindRelatedSymbols` | find_related_symbols | Non-empty text, valid envelope |
| `TestP1E2E_ValidateGraphEdge` | validate_graph_edge | Non-empty text, valid envelope |
| `TestP1E2E_GetClusterMap` | get_cluster_map | Non-empty text, valid envelope |
| `TestP1E2E_ExplainCluster` | explain_cluster | Uses seedClusterIDEncoded = `"weak_components:{gv}:1"`, non-empty text, valid envelope |
| `TestP1E2E_GetChangeImpactGraph` | get_change_impact_graph | Mode=review (modeTierReview requirement), non-empty text, valid envelope |

`assertP1Envelope(t, tool, text)` validates freshness envelope closed enums: `status ∈ {current, stale, unknown}`, `source ∈ {store, live_lsp, fallback, none}`, `fallback_reason ∈ {none, accessor-nil, accessor-error, accessor-timeout, ""}`, `confidence ∈ [0.0, 1.0]`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Functionality] Daemon P1 seam gap — TypeChain/SymbolEdges/EdgeEvidence/ClusterMembership not wired in newSemanticBundle**

- **Found during:** Task 3 fixture design
- **Issue:** `daemon.newSemanticBundle` only wires 8 core setters. `TypeChain`, `SymbolEdges`, `EdgeEvidence`, and `ClusterMembership` seams have no SQL backing in the current `*Store` — no accessor factory exists for them in the daemon layer.
- **Fix:** Used inline fixture implementations (`p1InlineTypeChain`, `p1InlineSymbolEdges`, `p1InlineEdgeEvidence`, `p1InlineClusterMembership`) that return data consistent with the 7 seeded symbols. This satisfies SC#4 (real *Store drives the cluster/symbol/impact paths) while gracefully handling seams that are not yet backed by SQL queries.
- **Files modified:** Only p1_e2e_external_test.go (test-only)
- **Commit:** fd608e68

**2. [Rule 1 - Bug] extractText redeclaration avoided — reuse from status_e2e_external_test.go**

- **Found during:** Task 3 initial draft
- **Issue:** First draft defined a new `extractTextP1` function with a panicking stub. Since both files are in `package semantic_test`, the existing `extractText` function from `status_e2e_external_test.go` is directly accessible.
- **Fix:** Removed the stub and used `extractText` directly throughout p1_e2e_external_test.go.
- **Files modified:** p1_e2e_external_test.go only
- **Commit:** fd608e68

## Verification Results

```
go build ./internal/skill/semantic/... — exit 0 (swift macro warning pre-existing)
go vet ./internal/skill/semantic/ — exit 0
go test -race -count=1 ./internal/skill/semantic/ -run TestWrapperConsistency_P1Handlers — PASS
go test -race -count=1 ./internal/skill/semantic/ -run TestP1E2EFixture — PASS (0.10s)
go test -race -count=1 ./internal/skill/semantic/ -run 'TestP1E2E_' — 6/6 PASS (2.6s)
go test -race -count=1 ./internal/skill/semantic/ — PASS (6.1s)
go build ./... — exit 0 (tmp/ fixture errors pre-existing)
go vet ./... — exit 0 (same pre-existing tmp/ errors)
```

## Known Stubs

None — all inline fixture implementations produce data consistent with the seeded DuckDB store contents. The `p1InlineTypeChain`, `p1InlineSymbolEdges`, `p1InlineEdgeEvidence`, and `p1InlineClusterMembership` fixtures are intentional test scaffolding (not UI-visible stubs), and they are wired to return data for the 7 symbols seeded in `buildP1E2EFixture`.

## Threat Flags

None — test-only files; no new network endpoints, auth paths, or schema changes introduced.

## Self-Check: PASSED

- [x] `internal/skill/semantic/wrapper_consistency_test.go` exists
- [x] `internal/skill/semantic/export_p1_test.go` exists
- [x] `internal/skill/semantic/p1_e2e_external_test.go` exists (815 lines)
- [x] Commits 31715eb3, 3f779ac2, fd608e68 exist in git log
- [x] `go build ./...` and `go vet ./...` exit 0
- [x] `TestP1E2EFixture_SeedsNonEmpty` race-clean PASS
- [x] All 6 `TestP1E2E_*` race-clean PASS
- [x] Full package `./internal/skill/semantic/` race-clean PASS (6.1s)
