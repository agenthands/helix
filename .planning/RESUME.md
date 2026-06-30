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

**Resolved pipeline (DstNodeID filled for in-repo targets, 2026-06-30):**
- IMPLEMENTS, EXTENDS — heritage captures → edges at factsFromExtracted; DstNodeID
  resolved via batch name index (`resolvePendingDst`), conf 0.70 when in-repo, 0 external
- IMPORTS — import named-symbols → resolved to in-repo def (conf 0.60); bare module imports stay 0
- DEFINES — container→child edges at factsFromExtracted (always resolved)

**Name-based classifier (flowing, callee resolved when in-repo):**
- HTTP_CALLS, ASYNC_CALLS, EMITS, LISTENS_ON
- `internal/semantic/classifier/classifier.go` (~350 LOC)
- 11-language pattern tables
- Wired in `semantic_wiring.go`; callee NAME resolved to in-repo def (conf 0.55) else 0

**Algorithmic (WIRED end-to-end 2026-06-30):**
- DATA_FLOWS — ASTProfile (25 features) computed per function body at extraction time
  (all 11 providers via `extract.FingerprintBody`), emitted as structural-profile
  similarity in `dataFlowsEdges` (cosine ≥ 0.985, lang-scoped buckets). Structural-shape
  similarity, NOT interprocedural taint flow. Conf 0.35.
- SIMILAR_TO — MinHash signature per function body (all 11 providers), emitted via LSH
  near-clone detection in `similarToEdges` (Jaccard ≥ 0.95). Conf 0.45.
- SEMANTICALLY_RELATED — MinHash package available, needs RI engine (still unwired)

### Type Resolvers (11/11)
- Full: Go, TypeScript, Python
- Stubs (LSP-conditional): Java, C#, Rust, C, C++, Kotlin, PHP, Ruby
- Wired in `internal/daemon/type_resolver_wiring.go` + `daemon.go:605-617`

### Graph Tool Tests
`internal/skill/semantic/p1_multilang_test.go` — 11 languages × 4 seed-based tools = 44 subtests + 2 cluster tests = 46 all passing

### Benchmark Fixtures
`internal/skill/semantic/p1_e2e_external_test.go` — extended with 8 new languages (files, symbols, edges, scores, clusters, adapters)

## Gaps vs codebase-memory-mcp

> **Re-audited against HEAD 2026-06-30.** The pre-2026-06-30 "Missing entirely"
> list was stale: 6 edge kinds + Route/Resource nodes it called missing are in
> fact emitted/present at HEAD (see CORRECTION below). Every claim here is keyed
> to a verified emission site in `internal/daemon/semantic_wiring.go` /
> `semantic_similarity_edges.go` or a declaration in `internal/semantic/extract`.

### Depth gaps (exist but shallow)
1. **IMPLEMENTS/EXTENDS/IMPORTS**: ~~DstNodeID=0~~ **RESOLVED 2026-06-30.** Targets
   now resolved against the batch name→node index in `factsFromExtracted`
   (`resolvePendingDst`, `internal/daemon/semantic_similarity_edges.go`) — the
   same mechanism TESTS/HANDLES already use. The type-resolver path RESUME
   pointed at is a Phase-57 stub (`QueryEffectiveEdges` returns empty), so it was
   never going to fill these. In-repo targets get a real DstNodeID + raised
   confidence (heritage 0.70, named imports 0.60, classifier callees 0.55);
   out-of-repo targets (stdlib/external) correctly stay DstNodeID=0 at base
   confidence. DEFINES was already resolved (never DstNodeID=0 — RESUME was stale).
   Latent bug also fixed: snapshot edges never got an `EdgeID` (PK is
   `(snapshot_id, edge_id)`), so any ≥2-edge snapshot would have collided in
   production — now stamped dense 1-based in the buildFn before WriteSnapshotFacts.
2. **HTTP_CALLS/ASYNC_CALLS/EMITS/LISTENS_ON**: callee NAME now resolves to its
   in-repo definition when present (the in-repo emitter/handler case); external
   callees stay DstNodeID=0. Framework-specific AST detection still name-only.
3. **STRUCTURAL_TWIN** (the structural-shape edge, formerly mis-named DATA_FLOWS):
   **WIRED 2026-06-30; RENAMED 2026-06-30 (v2.8 Phase 124, B0-a).**
   `classifier.ComputeProfile` computed per function body at extraction time (all
   11 providers via `extract.FingerprintBody`), carried on
   `extract.SymbolFact.Profile`, emitted as structural-profile-similarity edges in
   `structuralTwinEdges` (`EdgeKind: "STRUCTURAL_TWIN"`, Source `ast_profile`,
   cosine ≥ 0.985, language-scoped quantized buckets, per-node fan-out cap,
   confidence 0.35). This is STRUCTURAL-PROFILE similarity (control-flow /
   expression shape), a distinct signal from SIMILAR_TO (MinHash token-clone).
   The **`DATA_FLOWS`/`data_flows` surface kind is now RESERVED** (declared +
   validatable, producerless) for true interprocedural arg-to-param flow — the
   v2.8 Workstream B FEATURE phases (def-use → arg→param → propagation), NOT yet
   roadmapped. That is the remaining genuine depth gap.
4. **SIMILAR_TO**: **WIRED 2026-06-30.** MinHash signatures computed per function
   body at extraction time (all 11 providers), carried on
   `extract.SymbolFact.MinHash`, emitted via LSH near-clone detection in
   `similarToEdges` (Jaccard ≥ 0.95, per-node fan-out cap). Confidence 0.45.
5. **SEMANTICALLY_RELATED**: **WIRED 2026-06-30 (v2.8 Workstream A, Phases 121-123).**
   Now emitted via Random Indexing (cbm-mcp's own mechanism), NOT the unported
   neural embedder. New leaf engine `internal/semantic/relatedidx/` projects a
   body's identifier+comment VOCABULARY into a 256-dim int32 context vector
   (deterministic: integer accumulation, order-independent, N-shuffle proven);
   a curated stop-list (`stopwords.go`) drops boilerplate so the edge reflects
   domain vocabulary not plumbing. Computed per body via `extract.FingerprintBody`
   (all 11 providers, on `SymbolFact.ContextVec`), emitted in `semanticallyRelatedEdges`
   (`semantic_similarity_edges.go`) via SimHash LSH (16×4 bands) + exact-cosine
   recheck ≥ 0.55, fan-out capped. ORTHOGONAL to SIMILAR_TO by construction
   (MinHash erases vocabulary/keeps structure; RI keeps vocabulary/erases
   structure) — proven on real bodies: same-struct/disjoint-vocab Jaccard 1.0 /
   cosine 0.0; diff-struct/shared-vocab Jaccard 0.0 / cosine 0.80. Selectivity
   mutation-confirmed: removing the stop-list makes boilerplate-saturated
   unrelated bodies link (cosine 0.69 > 0.55) → guard goes RED. Readable via
   `helix explain-symbol-deep` (no kind filter). The Random-Indexing engine cbm-mcp
   uses (nomic-embed-code, 768-dim neural vectors) is still NOT ported — RI is the
   single-binary-safe parity path; bleve remains BM25 text-only.

### Missing entirely (verified against HEAD audit 2026-06-30)

Genuinely absent at HEAD (post-v2.8 Workstream A + Phase 124):
- **True interprocedural DATA_FLOWS** — the `DATA_FLOWS`/`data_flows` surface kind is now RESERVED but has NO producer: real arg-to-param flow (def-use → arg→param → propagation) is unbuilt. The structural-shape edge that used to squat on this name is now correctly `STRUCTURAL_TWIN` (see depth gap #3). This is v2.8 Workstream B's FEATURE phases — B0-a (the rename) is done; the feature build is not yet roadmapped.
- **Cypher / openCypher query subset** — no query language over the edge store.
- **Vector semantic search** — bleve is BM25 text-only; no embedding model, no vector index, no vector dependency in `go.mod`.
- **3D visualization UI.**
- **Team artifact packaging** — Helix has DuckDB snapshots but no cbm-mcp-style compressed shareable repo artifact.
- **158-language coverage** — Helix ships 11 first-class extractors.

CORRECTION — the following were listed "missing entirely" in the pre-2026-06-30
note but ARE emitted/present at HEAD (verified emission sites):
- **HANDLES** — `semantic_wiring.go:2382` (handler→route) + classifier path.
- **CONFIGURES / WRITES** — `classifier.ClassifyCall` → emitted at `semantic_wiring.go:2186`→`2198`.
- **MEMBER_OF** — `semantic_wiring.go:2306`.
- **TESTS** — `semantic_wiring.go:2361`.
- **FILE_CHANGES_WITH** — git co-change via `cochange.Mine`, `semantic_wiring.go:1571`.
- **CROSS_IMPORTS / CROSS_CALLS** — `crossrepo` resolver, `semantic_wiring.go:2140` / `2171`.
- **Route / Resource node types** — `extract.KindRoute`/`KindResource` (`fact.go:26-27`); `RouteFact`/`ResourceFact` promoted to synthetic nodes (`fact.go:201-220`).

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

### Depth work (2026-06-30) — DstNodeID resolution + DATA_FLOWS/SIMILAR_TO wiring
- `internal/semantic/extract/fact.go` — `SymbolFact.MinHash` + `.Profile` fingerprint fields
- `internal/semantic/extract/fingerprint.go` — new; `FingerprintBody` + `IsFingerprintableKind`
- `internal/semantic/extract/{golang,rust,java,csharp,c,cpp,kotlin,php,ruby,python,typescript}/provider.go`
  — all 11 providers compute fingerprints on function/method bodies (Python/TS ascend
  from name node to declaration; the other 9 use their existing `body` node)
- `internal/daemon/semantic_similarity_edges.go` — new; `resolvePendingDst`,
  `similarToEdges`, `dataFlowsEdges`, `lastNameSegment`, `importTargetCandidates`
- `internal/daemon/semantic_wiring.go` — `factsFromExtracted` records pending dst +
  collects fingerprints + emits SIMILAR_TO/DATA_FLOWS; buildFn stamps dense EdgeIDs
- `internal/daemon/semantic_similarity_edges_test.go`, `semantic_similarity_e2e_test.go` — new tests

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
- **Depth work 2026-06-30:** `go build ./...` clean, `go vet` clean.
  `go test ./internal/semantic/extract/... ./internal/semantic/{classifier,minhash,types}/...
  ./internal/daemon/... ./internal/skill/semantic/...` — 24 pkg ok, 0 fail.
  New: 9 helper unit tests + 2 real-Go-provider e2e tests (clone→SIMILAR_TO+DATA_FLOWS,
  embedded-struct→resolved IMPLEMENTS). Fixed latent snapshot-edge PK collision
  (edges never got an EdgeID; surfaced once a ≥2-edge snapshot was emitted).
- `go test ./internal/cli/...` — pass. NOTE: the HELIX_BIN-gated E2E oracle drives the
  `helix` binary on PATH by default; `/home/john/bin/helix` was stale (built 06-24, predates
  the "activate honors HELIX_SOCKET" fix) and produced a false RED
  (`startup lock tryLock: .../helix-1000/daemon.sock.lock: no such file or directory`).
  Rebuilt + reinstalled `/home/john/bin/helix` from current source → green. Proven first via
  `HELIX_BIN=/tmp/helix-fresh`. Not a code regression.
