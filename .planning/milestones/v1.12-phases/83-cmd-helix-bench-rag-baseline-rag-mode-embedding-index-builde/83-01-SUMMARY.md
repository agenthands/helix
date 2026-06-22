---
phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde
plan: 01
subsystem: testing
tags: [chromem-go, embeddings, rag, vector-db, bench, corpus-sha, leaf-package]

# Dependency graph
requires:
  - phase: 80
    provides: "baseline_rag fail-closed stub + MODE.md two-key resolver convention this arm replaces"
  - phase: 77
    provides: "bench/runtime result.v2 builder + fairness contract this RAG arm will plug into (Plan 03)"
provides:
  - "bench/ragindex leaf package: deterministic corpus_sha, HELIX_CACHE_DIR cache layout, line-window chunker, embedder selection, chromem-backed Build/Open/Query"
  - "chromem-go v0.7.0 as a direct dependency (embeddable Go vector DB)"
  - "EMBED-CHOICE.md: OpenAI text-embedding-3-small / Ollama nomic-embed-text / stub-deterministic model pin + chunking + cache-layout doc"
affects: [83-02 (cmd/helix-bench-rag server consumes ragindex), 83-03 (cell wiring builds index via ragindex.Open over the cloned task repo)]

# Tech tracking
tech-stack:
  added: [github.com/philippgille/chromem-go v0.7.0]
  patterns:
    - "Leaf bench package (chromem-go + stdlib only) to keep the cmd's no-kernel/no-semantic vet gate true transitively"
    - "Embedder-injectable openWith core so the package is unit-testable with a deterministic stub at zero network"
    - "Content-only corpus_sha (sorted (rel-path, content-sha256) pairs) mirroring result.go fairnessBlock sort discipline"

key-files:
  created:
    - bench/ragindex/cache.go
    - bench/ragindex/cache_test.go
    - bench/ragindex/chunk.go
    - bench/ragindex/chunk_test.go
    - bench/ragindex/embedder.go
    - bench/ragindex/embedder_test.go
    - bench/ragindex/index.go
    - bench/ragindex/index_test.go
    - bench/runners/baseline_rag_agent/EMBED-CHOICE.md
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Stub embedder is a position-independent byte-value histogram (L2-normalized), not a positional sum: shared bytes between a query and a chunk raise cosine similarity, so the no-network reuse/query tests pass while staying deterministic and obviously NOT a real embedder (recorded embedder_id=stub-deterministic, Open Q3)"
  - "Warm-path reuse is detected by coll.Count()>0 after NewPersistentDB auto-loads gob docs+embeddings; on a warm path AddDocuments is skipped so zero re-embeds occur (Pitfall 1: the same embedder is re-supplied to GetOrCreateCollection since chromem never persists the func)"
  - "chunk.go's struct is named Piece (not Chunk) because Go forbids a type and the exported func Chunk sharing a name in one package; the plan's contains:func Chunk is honored by the function"
  - "Empty/whitespace-only files yield ZERO chunks (decided + asserted in TestChunkEmptyFile); a file within one window yields exactly one chunk"
  - "chromem-go promoted to a DIRECT require by a targeted go.mod hand-edit because a full go mod tidy is blocked by a pre-existing, unrelated s2a-go module-graph resolution failure (out of scope, logged to deferred-items.md)"

patterns-established:
  - "Leaf-package discipline proven by go list -deps grep of kernel/semantic/bench-runtime/mcp returning nothing"
  - "Ollama base URL pinned to the default constant, never a function argument (SSRF mitigation T-83-01-02); OPENAI_API_KEY read for availability only, never logged (T-83-01-01)"

requirements-completed: [ABLATE-04]

# Metrics
duration: ~22min
completed: 2026-06-21
---

# Phase 83 Plan 01: ragindex Embedding-Index Builder Summary

**A self-contained `bench/ragindex` leaf package — content-deterministic corpus_sha, `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/` cache, 40-line/8-overlap chunker, OpenAI→Ollama→stub embedder selection, and chromem-go-backed cold-build / warm-reuse / k-NN Query — plus the EMBED-CHOICE.md model pin.**

## Performance

- **Duration:** ~22 min
- **Started:** 2026-06-21
- **Completed:** 2026-06-21
- **Tasks:** 4 (1 checkpoint orchestrator-approved, 2 TDD, 1 auto)
- **Files modified:** 11 (9 created, go.mod/go.sum)

## Accomplishments
- `chromem-go v0.7.0` added and pinned as a direct dependency (zero transitive deps; package-legitimacy gate orchestrator-approved).
- `bench/ragindex` is a true LEAF: `go list -deps` shows NO `internal/kernel`, `internal/semantic`, `bench/runtime`, or `internal/mcp` import — the contract Plan 02's vet gate relies on transitively.
- `CorpusSHA` is content-deterministic and mtime/walk-order-independent (sorted `(rel-path, content-sha256)` pairs); a one-byte content change flips it.
- Cold `Open` chunks + embeds every corpus file; a warm reopen of the same path reuses persisted gob docs+embeddings with **zero re-embeds** (proven by a counting-stub call count of 0 on the warm path).
- `selectEmbedder` returns a DISTINCT `embedder_id` per backend and never logs the API key; the stub makes the whole package unit-testable with no network.
- `EMBED-CHOICE.md` documents the three-backend model pin, the chunking strategy, and the cache layout (ABLATE-04 criterion #2, doc half).

## Task Commits

1. **Task 1: chromem-go dependency (checkpoint, orchestrator-approved)** — `5300a7ee` (chore)
2. **Task 2: corpus_sha + cache dir + chunker** — `552c34b2` (test RED) → `6990b2f5` (feat GREEN)
3. **Task 3: embedder selection + chromem index** — `ae447e89` (test RED) → `16873280` (feat GREEN)
4. **Task 4: EMBED-CHOICE.md** — `6a4ae1a0` (docs)

## Files Created/Modified
- `bench/ragindex/cache.go` — `CorpusSHA`, `cacheDir` (HELIX_CACHE_DIR > UserCacheDir/helix > ~/.helix/cache), `IndexPath`.
- `bench/ragindex/chunk.go` — `Chunk(content, relPath) []Piece`; 40-line windows, 8-line overlap, `<rel>#<ord>` IDs.
- `bench/ragindex/embedder.go` — `selectEmbedder`, `stubEmbedder`, Ollama reachability probe; embedder_id constants.
- `bench/ragindex/index.go` — `Open`/`openWith` over `chromem.NewPersistentDB` + `GetOrCreateCollection` + `AddDocuments`; `Query`, `EmbedderID`, `Count`.
- `bench/ragindex/{cache,chunk,embedder,index}_test.go` — determinism, precedence, layout, distinct IDs, stub determinism, warm-path zero-re-embed, query relevance.
- `bench/runners/baseline_rag_agent/EMBED-CHOICE.md` — model pin + chunking + cache-layout doc.
- `go.mod`, `go.sum` — chromem-go v0.7.0 direct require.

## Decisions Made
See `key-decisions` frontmatter. Headline: byte-value-histogram stub (deterministic, no-network, semantically faithful enough for the reuse/query tests yet plainly not a real embedder); `coll.Count()>0` warm-path gate; chunk struct renamed `Piece` to free the `Chunk` function name; chromem promoted to direct require by hand-edit (tidy blocked by unrelated s2a-go).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `go mod tidy` blocked by pre-existing unrelated s2a-go module-graph failure**
- **Found during:** Task 3 (after `go get chromem-go`)
- **Issue:** `go mod tidy` aborts resolving `github.com/google/s2a-go` (transitive via the sigstore→google.golang.org/api chain) — entirely unrelated to chromem-go. Left chromem-go marked `// indirect`.
- **Fix:** Targeted `go.mod` hand-edit promoting `github.com/philippgille/chromem-go v0.7.0` into the direct `require` block and removing the `// indirect` line. Verified `go list -m` → v0.7.0 and `go build` of touched packages.
- **Files modified:** go.mod
- **Verification:** `go list -m github.com/philippgille/chromem-go` → `v0.7.0`; `go build ./bench/ragindex/... ./cmd/helix`; `go test ./bench/ragindex/` green.
- **Committed in:** `16873280` (Task 3 commit). Out-of-scope s2a-go fix logged to `deferred-items.md`.

**2. [Design clarification] chunk type renamed `Chunk`→`Piece`**
- **Found during:** Task 2 (chunk.go GREEN)
- **Issue:** The plan's `contains: "func Chunk"` and the tests both want `Chunk(...)` as a function, but a Go package cannot have a type and a function with the same name.
- **Fix:** Named the result struct `Piece`; kept `func Chunk(content, relPath string) []Piece`. Tests reference fields (`.ID`, `.Content`, `.RelPath`), not the type name, so this is transparent.
- **Files modified:** bench/ragindex/chunk.go
- **Verification:** `TestChunk*` green.
- **Committed in:** `6990b2f5` (Task 2 commit).

---

**Total deviations:** 2 (1 Rule-3 blocking, 1 design clarification)
**Impact on plan:** Both necessary to compile/ship; no scope creep. The s2a-go issue is pre-existing and explicitly out of scope.

## Threat Model Compliance
- **T-83-01-01 (secret leak):** `OPENAI_API_KEY` is read only for availability; only `embedder_id` model strings are recorded — verified by inspection of embedder.go.
- **T-83-01-02 (SSRF):** Ollama base URL pinned to the `ollamaBaseURL` constant `http://localhost:11434/api`, never an argument.
- **T-83-01-03 (path tampering):** `corpus_sha` is a hex sha256 with no separators, joined under the resolved cacheDir; cannot escape the cache root.
- **T-83-SC (package legitimacy):** Task 1 checkpoint resolved by orchestrator (chromem-go v0.7.0 verified on proxy.golang.org + local module cache).

## Known Stubs
- `stub-deterministic` embedder (`bench/ragindex/embedder.go`) is an INTENTIONAL, documented stub — it is the hermetic-CI/offline fallback recorded with a distinct `embedder_id` so a CI row is never mistaken for a real RAG measurement (Open Q3). It is not a defect; live/soak runs use OpenAI→Ollama. Documented in EMBED-CHOICE.md.

## Issues Encountered
- Initial `TestQueryReturnsRelevantChunk` failed: the first stub (positional byte sum) made a short query cluster away from its source chunk. Switched the stub to a position-independent byte-value histogram so shared bytes raise cosine similarity; test now green. (Resolved within Task 3, before the GREEN commit.)

## User Setup Required
`OPENAI_API_KEY` is required ONLY for live/soak embedder runs (Plan 03), NOT for CI/unit tests (the stub embedder needs no network). See the plan `user_setup` block and EMBED-CHOICE.md. No setup is needed to run `go test ./bench/ragindex/...`.

## Next Phase Readiness
- `bench/ragindex` is ready to be consumed by Plan 02 (`cmd/helix-bench-rag` server: `rag_search`→`Index.Query`, `rag_read_chunk`) and Plan 03 (cell builds the index out-of-band via `ragindex.Open(repoDir)` over the cloned task repo, excluded from the agent budget).
- Leaf invariant holds, so importing `bench/ragindex` into the standalone server will not break the no-kernel/no-semantic vet gate.
- Open: full-tree `go mod tidy` deferred until the pre-existing s2a-go module-graph issue is resolved (deferred-items.md).

## Self-Check: PASSED

All 10 created files exist on disk; all 7 commits (5300a7ee, 552c34b2, 6990b2f5, ae447e89, 16873280, 6a4ae1a0, dd12c804) are present in git history.

---
*Phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde*
*Completed: 2026-06-21*
