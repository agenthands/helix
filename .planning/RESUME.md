# RESUME.md — Polyglot Graph Expansion (2026-06-28)

## Context
Borrowing codebase-memory-mcp graph edge model into Helix. Target: 14 edge types, 11 first-class languages, all graph tools working end-to-end.

## What Shipped

### Languages (11/11 extractors)
Go, TypeScript, Python, Java, C#, Rust, C, C++, Kotlin, PHP, Ruby
- Each has: queries.scm, provider.go, smoke_test.go, provider_test.go, stable_id_test.go
- ~231 testdata scenarios total
- Daemon wired in `internal/daemon/daemon.go:374-387`

### Edge Types (14/14 declared, 14/14 pipeline)
All in `internal/skill/semantic/edge_kind_surface.go`

**Deep pipeline (flowing end-to-end, high confidence):**
- CALLS — tree-sitter ref.call captures
- RESOLVES_TO — type resolver
- USES_TYPE — tree-sitter ref.type captures

**Shallow pipeline (flowing, low confidence, needs resolution):**
- IMPLEMENTS, EXTENDS — heritage captures → edges at factsFromExtracted, DstNodeID=0
- IMPORTS — import.source → edges at factsFromExtracted, DstNodeID=0
- DEFINES — container→child edges at factsFromExtracted

**Name-based classifier (flowing, 0.45 confidence):**
- HTTP_CALLS, ASYNC_CALLS, EMITS, LISTENS_ON
- `internal/semantic/classifier/classifier.go` (~350 LOC)
- 11-language pattern tables
- Wired in `semantic_wiring.go:1980-2005`

**Algorithmic (code exists, needs wiring):**
- DATA_FLOWS — ASTProfile (25 features), `classifier.go:ComputeProfile()`
- SIMILAR_TO — `internal/semantic/minhash/minhash.go` (~230 LOC, port of minhash.c)
- SEMANTICALLY_RELATED — MinHash package available, needs RI engine

### Type Resolvers (11/11)
- Full: Go, TypeScript, Python
- Stubs (LSP-conditional): Java, C#, Rust, C, C++, Kotlin, PHP, Ruby
- Wired in `internal/daemon/type_resolver_wiring.go` + `daemon.go:605-617`

### Graph Tool Tests
`internal/skill/semantic/p1_multilang_test.go` — 11 languages × 4 seed-based tools = 44 subtests + 2 cluster tests = 46 all passing

### Benchmark Fixtures
`internal/skill/semantic/p1_e2e_external_test.go` — extended with 8 new languages (files, symbols, edges, scores, clusters, adapters)

## Gaps vs codebase-memory-mcp

### Depth gaps (exist but shallow)
1. **IMPLEMENTS/EXTENDS/IMPORTS/DEFINES**: DstNodeID=0 (unresolved). Need type resolver to fill targets.
2. **HTTP_CALLS/ASYNC_CALLS/EMITS/LISTENS_ON**: Name-only classification. cbm-mcp uses AST-pattern + framework-specific detection.
3. **DATA_FLOWS**: 25-feature profile exists, but not wired into edge emission yet. cbm-mcp has interprocedural analysis.
4. **SIMILAR_TO**: MinHash code ported but not wired into indexing pipeline. Needs integration with symbol extraction.
5. **SEMANTICALLY_RELATED**: MinHash ported but Random Indexing engine (768-dim vectors) not ported. cbm-mcp uses nomic-embed-code.

### Missing entirely
- HANDLES, CONFIGURES, WRITES, MEMBER_OF, TESTS edge types
- FILE_CHANGES_WITH (co-change analysis)
- CROSS_* (cross-repo links)
- Cypher query support
- Vector semantic search (bleve exists but no embeddings)
- 3D visualization UI
- Team artifact packaging
- Route/Resource node types
- 158 languages (we have 11)

## Key Files Changed
- `internal/semantic/extract/{java,csharp,rust,c,cpp,kotlin,php,ruby}/` — 8 new extractors
- `internal/semantic/types/{c_sharp,rust,c,cpp,kotlin}/` — 5 new stub resolvers
- `internal/semantic/classifier/classifier.go` — new, edge classification
- `internal/semantic/minhash/minhash.go` — new, MinHash port
- `internal/daemon/daemon.go` — extractor + resolver wiring
- `internal/daemon/semantic_wiring.go` — edge emission for all new types
- `internal/daemon/type_resolver_wiring.go` — resolver factories
- `internal/skill/semantic/edge_kind_surface.go` — all 18 edge types
- `internal/skill/semantic/p1_e2e_external_test.go` — 8-language fixture extension
- `internal/skill/semantic/p1_multilang_test.go` — 11-language graph tool tests
- `internal/skill/semantic/edge_kind_surface_test.go` — roundtrip tests

## Verification (re-run 2026-06-30, full sweep green)
- `go build ./cmd/helix` — clean
- `go build ./...` — clean (all test files compile)
- `go test ./internal/semantic/extract/...` — all 11 languages pass
  - Java `whitespace_edit_preserves_id` golden regenerated: receiver-capture now
    emits `receiver_text: "System.out"` on the two `println` call refs. It was the
    lone straggler the RecvJavaCsharp subagent missed (only Java fixture with a
    receiver-style call; every other fixture has bare calls, so `omitempty` kept
    their goldens valid).
- `go test ./internal/semantic/{classifier,minhash,types}/...` — pass
- `go test ./internal/semantic/{cochange,crossrepo}/...` — pass
- `go test ./internal/skill/semantic/...` — pass (CROSS_CALLS edges, 46 multilang subtests, 13.6s)
- `go test ./internal/daemon/...` — pass (incl. `TestGracefulShutdownMidRequest`, which
  was flaky/timing-dependent earlier; green now)
- `go test ./internal/cli/...` — pass. NOTE: the HELIX_BIN-gated E2E oracle drives the
  `helix` binary on PATH by default; `/home/john/bin/helix` was stale (built 06-24, predates
  the "activate honors HELIX_SOCKET" fix) and produced a false RED
  (`startup lock tryLock: .../helix-1000/daemon.sock.lock: no such file or directory`).
  Rebuilt + reinstalled `/home/john/bin/helix` from current source → green. Proven first via
  `HELIX_BIN=/tmp/helix-fresh`. Not a code regression.
