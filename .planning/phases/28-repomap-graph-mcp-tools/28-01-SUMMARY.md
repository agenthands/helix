---
phase: 28-repomap-graph-mcp-tools
plan: 01
subsystem: repomap
tags: [graph, pagerank, lsp-enrichment, tag-cache]
dependency_graph:
  requires: [27-01, 27-02, 27-03, 27-04]
  provides: [FileGraph, BuildGraph, PageRank, RankFiles, AllFiles, EnrichFromLSP, Location, RankedFile]
  affects: [28-02, 28-03]
tech_stack:
  added: []
  patterns: [power-iteration-pagerank, personalized-teleportation, sqrt-edge-weighting, self-loop-isolated-defs]
key_files:
  created:
    - internal/repomap/graph.go
    - internal/repomap/pagerank.go
    - internal/repomap/pagerank_test.go
    - internal/repomap/graph_test.go
  modified:
    - internal/repomap/cache.go
decisions:
  - "Hand-rolled PageRank (~60 LOC) avoids third-party dependency; supports personalization"
  - "Version counter on TagCache enables dirty-flag graph rebuild caching"
  - "Location type decouples repomap from gen package for LSP enrichment"
metrics:
  duration: 331s
  completed: "2026-04-17T13:46:18Z"
  tasks_completed: 3
  tasks_total: 3
  files_created: 4
  files_modified: 1
---

# Phase 28 Plan 01: FileGraph + PageRank + LSP Enrichment Summary

Cross-file reference graph built from SQLite-cached tags with PageRank ranking (uniform + personalized) and LSP enrichment hook.

## One-liner

File-level reference graph with sqrt-weighted edges, power-iteration PageRank supporting personalized teleportation, and additive LSP enrichment -- zero new dependencies.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | TagCache.AllFiles + FileGraph + BuildGraph + EnrichFromLSP | e8a39bcd | cache.go, graph.go |
| 2 | PageRank with personalization support (TDD) | 36184004 | pagerank.go, pagerank_test.go |
| 3 | Graph building and LSP enrichment tests (TDD) | 49fa9086 | graph_test.go |

## Implementation Details

### TagCache Extensions (cache.go)
- `AllFiles() (map[string][]Tag, error)` -- bulk query of all cached files and tags
- `Version() int64` -- monotonic counter tracking cache mutations (store, invalidate, clear)
- Version field added to TagCache struct, incremented in storeTags, InvalidateFile, Clear

### FileGraph (graph.go)
- `FileGraph` struct with `Edges map[string]map[string]float64`, `Files map[string]bool`, `sync.RWMutex`
- `BuildGraph(cache *TagCache)` -- constructs graph from cached tags:
  - Indexes defs (symbol -> []file) and refs (symbol -> file -> count)
  - Cross-file edges weighted by `math.Sqrt(refCount)` per D-03
  - Self-loops (weight 0.1) for definitions without cross-file references per aider pattern
  - Same-file refs skipped
- `EnrichFromLSP(sourceFile, refs []Location)` -- additive edges from LSP references per D-10/D-11
- `Location{URI, Line}` -- minimal type avoiding gen package import
- `RankedFile{Path, Score}` -- exported type for rendering layer
- `NodeCount()`, `EdgeCount()` -- query methods for tests

### PageRank (pagerank.go)
- `PageRank(damping, epsilon, maxIter, personalization)` -- power iteration with:
  - Uniform teleportation when personalization is nil
  - Personalized teleportation with seed normalization and small baseline for non-seed nodes
  - Dangling node rank redistribution (uniform across all nodes)
  - maxIter=100 cap per T-28-02 DoS mitigation
  - Convergence check via sum of absolute differences < epsilon
- `RankFiles(damping, personalization)` -- convenience wrapper returning sorted `[]RankedFile`

## Test Coverage

- 6 PageRank tests: uniform chain, personalized, dangling nodes, empty graph, single-node, sorted output
- 5 BuildGraph tests: cross-file edges, weight by ref count, isolated definition, skip same-file refs, empty cache
- 2 EnrichFromLSP tests: cross-file edge addition, same-file skip
- 2 TagCache tests: AllFiles bulk retrieval, Version counter increments

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Dangling node test setup**
- **Found during:** Task 2 (TDD GREEN phase)
- **Issue:** Test used `addEdge("C", "C", 0.0)` which registered C in `outWeight` map, preventing it from being treated as dangling
- **Fix:** Changed to `g.Files["C"] = true` without edges
- **Files modified:** pagerank_test.go
- **Commit:** 36184004

## Known Stubs

None -- all functionality is fully implemented and tested.

## TDD Gate Compliance

- RED gate: b8e46583 (test commit for PageRank tests)
- GREEN gate: 36184004 (PageRank implementation + test fix)
- Task 3 tests passed immediately against Task 1 implementation (expected -- tests validate prior task's code)
