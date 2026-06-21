---
phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde
verified: 2026-06-21T00:44:43Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: "Initial verification (no prior VERIFICATION.md). Code review 83-REVIEW.md found 0 blockers / 5 warnings; all 5 fixed (83-REVIEW-FIX.md); fixes confirmed non-regressive against all 4 success criteria."
---

# Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder Verification Report

**Phase Goal:** The RAG baseline is a competent grep + embedding-RAG control arm, not a strawman — a standalone `cmd/helix-bench-rag` MCP server with exactly 4 fixed tools (rag_search, rag_read_chunk, grep, read_file), backed by chromem-go + OpenAI text-embedding-3-small (Ollama nomic-embed-text offline fallback). baseline_rag reuses the fairness-contract model+budget; embedding-API calls are NOT charged to the agent per-task budget.
**Verified:** 2026-06-21T00:44:43Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (the four ROADMAP success criteria)

| # | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | `cmd/helix-bench-rag --help` works; tool-list = exactly 4 tools; vet/test gate asserts no transitive import from internal/kernel or internal/semantic | ✓ VERIFIED | `go build -o /tmp/hbr ./cmd/helix-bench-rag` OK; `--help` exit 0 with usage. `go list -deps ./cmd/helix-bench-rag \| grep internal/(kernel\|semantic\|mcp)` = EMPTY. `TestHelp`, `TestToolListIsExactlyFour`, `TestNoKernelSemanticImport` (go/packages NeedDeps transitive BFS) all PASS. Static analyzer `benchragleakage` (forbidden = internal/kernel + internal/semantic, slash-boundary match) wired into `make vet` (Makefile:47,54,71); `go vet -vettool=…` exit 0. **Live binary over MCP stdio returned exactly 4 tools: [grep, rag_read_chunk, rag_search, read_file].** ToolNames() count is tracked at AddTool time from each `mcpsdk.Tool.Name` — cannot diverge from real registrations. |
| 2 | Per-corpus index built once per (corpus, embedder_model), cached at `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`; EMBED-CHOICE.md documents the model pin + chunking | ✓ VERIFIED | `IndexPath` = `filepath.Join(cacheDir(), "bench-rag-index", sha)` (cache.go:84-90); cacheDir precedence HELIX_CACHE_DIR > UserCacheDir/helix > ~/.helix/cache. `CorpusSHA` content-deterministic (sorted (rel-path, sha256) pairs, mtime/order independent). `TestCachePathAndReuse` proves WARM reopen = **zero re-embeds** via an atomic call-counter (warmCalls==0 asserted). `TestCorpusSHADeterministic/TestIndexPathLayout/TestCacheDirPrecedence/TestQueryReturnsRelevantChunk` PASS. EMBED-CHOICE.md (90 lines) documents OpenAI text-embedding-3-small primary, Ollama nomic-embed-text fallback, stub-deterministic CI fallback, 40-line/8-overlap chunker, `<rel>#<ord>` IDs, and the cache layout. |
| 3 | A baseline_rag ToolBench-Go run produces schema-valid result.v2.json rows with embedder_id recorded in every row | ✓ VERIFIED | `embedder_id` threaded ResultInput → resultDoc (`json:"embedder_id,omitempty"`) → BuildResult (result.go:57,129,180). Pure tests `TestEmbedderIDOmittedWhenEmpty/EmittedWhenSet` PASS. **HELIX_BIN + HELIX_BENCH_RAG_BIN-gated `TestBaselineRagEmitsRow` RAN (0.24s, not skipped) and PASSES**: it reads the on-disk result.v2.json, runs `Validate(b)` (schema), and asserts `mode==baseline_rag` + non-empty `embedder_id`. `five_of_six` smoke + full gated `./bench/runtime/...` suite green (34s). |
| 4 | Same-model-same-budget invariant: DefaultContract reused verbatim; index build OUT-OF-BAND (excluded from per-task budget); documented in BENCH.md | ✓ VERIFIED | rag.go:101 `ragindex.Open` runs BEFORE rag.go:120 `start := time.Now()`; server spawn + agent drive after the anchor. Result built with `Fairness: runners.DefaultContract` verbatim (rag.go:235) + `EmbedderID` (rag.go:236). **Gated `TestBaselineRagSameContractBudget` RAN and PASSES**: asserts `model_id == DefaultContract.ModelID` AND `tokens_input/tokens_output == null` (embedding cost not charged). BENCH.md:88-99 documents DefaultContract verbatim reuse, out-of-band build, tokens NOT charged, embedder_id on every row; MODE.md rewritten to the real 4-tool arm ("fail-closed stub" prose removed). |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `bench/ragindex/cache.go` | CorpusSHA + cacheDir + IndexPath layout | ✓ VERIFIED | content-deterministic SHA; `bench-rag-index/<sha>` path; tests green |
| `bench/ragindex/chunk.go` | deterministic chunker | ✓ VERIFIED | 40-line/8-overlap windows, `<rel>#<ord>` IDs; `func Chunk` present |
| `bench/ragindex/embedder.go` | selectEmbedder + distinct embedder_id + fail-closed pin (WR-01) | ✓ VERIFIED | OpenAI/Ollama/stub IDs distinct; `HELIX_RAG_EMBEDDER` pin fail-closed; `TestPinnedEmbedderHonoredOrFailClosed` PASS; API key never logged |
| `bench/ragindex/index.go` | chromem Build/Open/Query; warm reuse | ✓ VERIFIED | `chromem.NewPersistentDB`; zero-re-embed warm path proven by counting stub |
| `bench/runners/baseline_rag_agent/EMBED-CHOICE.md` | model pin + chunking + cache doc | ✓ VERIFIED | names all 3 embedders, chunking, cache layout |
| `cmd/helix-bench-rag/{main,server,tools}.go` | cobra root + 4 AddTool + handlers | ✓ VERIFIED | --help exit 0; exactly 4 tools; validatePath (+symlink resolve WR-02, byte caps WR-03, binary skip WR-05) |
| `cmd/helix-bench-rag/leakage_test.go` | transitive import test | ✓ VERIFIED | go/packages NeedDeps BFS; PASS |
| `internal/lint/benchragleakage/analyzer.go` | static import-prefix gate | ✓ VERIFIED | forbidden = kernel+semantic, slash-boundary; analysistest green (leaky flagged, clean/sibling silent) |
| `cmd/vet-bench-rag-leakage/main.go` | singlechecker wrapper | ✓ VERIFIED | builds; `go vet -vettool` exit 0; wired into `make vet` |
| `bench/runtime/result.go` | embedder_id open key | ✓ VERIFIED | omitempty additive-minor; schema v2 unchanged |
| `bench/runtime/rag.go` | runRAGCell real drive leg | ✓ VERIFIED | out-of-band build before timed span; DefaultContract reuse |
| `bench/runtime/subprocess/ragserver.go` | StartRAGServer | ✓ VERIFIED | RAGHandle Pid()/Kill(); forwards HELIX_RAG_EMBEDDER pin (WR-01); stdout closed on drive return (WR-04) |
| `bench/runtime/deltas.go` | baseline_rag operand | ✓ VERIFIED | `full_minus_baseline_rag` optional operand; required modes unchanged |
| `bench/runners/baseline_rag/MODE.md` + `bench/BENCH.md` | real-arm + budget-exclusion docs | ✓ VERIFIED | real 4-tool arm; budget exclusion + embedder_id provenance documented |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| cmd/helix-bench-rag/server.go | go-sdk/mcp | NewServer + 4×AddTool + StdioTransport (NOT internal/mcp) | ✓ WIRED |
| cmd/helix-bench-rag/tools.go | bench/ragindex | idx.Query / Chunk reconstruction | ✓ WIRED |
| bench/runtime/rag.go | subprocess.StartRAGServer | spawns cmd/helix-bench-rag over stdio | ✓ WIRED |
| bench/runtime/rag.go | bench/ragindex | out-of-band ragindex.Open before time.Now() | ✓ WIRED |
| bench/runtime/result.go | result.v2 doc | embedder_id omitempty key | ✓ WIRED |
| Makefile | cmd/vet-bench-rag-leakage | go vet -vettool in `vet:` target | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| --help works | `helix-bench-rag --help` | exit 0, usage printed | ✓ PASS |
| Exactly 4 tools (live MCP) | initialize + tools/list over stdio | `[grep, rag_read_chunk, rag_search, read_file]` count=4 | ✓ PASS |
| No forbidden transitive import | `go list -deps … \| grep internal/(kernel\|semantic\|mcp)` | empty | ✓ PASS |
| Static vet gate | `go vet -vettool=vet-bench-rag-leakage ./cmd/helix-bench-rag/...` | exit 0 | ✓ PASS |
| Gated baseline_rag row emits schema-valid + embedder_id | `HELIX_BIN=… HELIX_BENCH_RAG_BIN=… go test -run TestBaselineRagEmitsRow` | RAN 0.24s, PASS | ✓ PASS |
| Same contract + budget exclusion | `… -run TestBaselineRagSameContractBudget` | RAN 0.24s, PASS (model_id match, null tokens) | ✓ PASS |
| Full gated bench suite | `HELIX_BIN=… HELIX_BENCH_RAG_BIN=… go test ./bench/runtime/...` | ok 34.281s | ✓ PASS |
| go vet touched packages | `go vet ./bench/... ./cmd/helix-bench-rag/... …` | exit 0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| ABLATE-04 | 83-01/02/03 | RAG baseline as a real grep+embedding control arm (4-tool isolated server, per-corpus index, embedder_id provenance, same-model-same-budget) | ✓ SATISFIED | All 4 success criteria verified above |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | No TBD/FIXME/XXX in any phase source file | — | none |
| (multiple) | — | `return nil` matches are WalkDir-callback / error-guard control flow, NOT stub returns | ℹ️ Info | none — verified by inspection |
| go.mod | — | Full-tree `go mod tidy` deferred due to pre-existing, unrelated s2a-go module-graph failure (deferred-items.md) | ℹ️ Info | Out of scope; chromem-go v0.7.0 is a working direct require, `go build ./...` clean. Not goal-blocking. |

The 5 code-review Warnings (WR-01..WR-05) were all fixed (83-REVIEW-FIX.md) and confirmed non-regressive: pinned fail-closed warm-reopen embedder (`TestPinnedEmbedderHonoredOrFailClosed`), symlink-resolving validatePath (`TestSymlinkEscapeRejected`), byte caps (`TestReadFileByteCap`), binary-file skip in grep (`TestGrepSkipsBinaryFiles`), and stdout-close goroutine reaping. The 4 Info findings (IN-01..IN-04) were explicitly out of scope and are non-blocking robustness/quality items.

### Human Verification Required

None. The one Manual-Only verification in 83-VALIDATION.md (live OpenAI/Ollama embedder reachability) is explicitly out of CI scope — the deterministic stub embedder makes the entire path hermetically testable, and live-embedder runs are a soak-time activity, not a phase-goal gate. All four success criteria are fully verifiable with automated checks, which pass.

### Gaps Summary

No gaps. All four ROADMAP success criteria for Phase 83 are observably TRUE in the live codebase:

1. The standalone `cmd/helix-bench-rag` builds, `--help` works, serves EXACTLY 4 tools (confirmed both by unit test and by a live MCP stdio round-trip), and is provably isolated from `internal/kernel`/`internal/semantic` both dynamically (transitive NeedDeps test) and statically (`make vet` analyzer).
2. The per-corpus chromem-go index is built once per (content-deterministic corpus_sha, embedder_id), cached under `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`, with warm reuse proven to do zero re-embeds; EMBED-CHOICE.md documents the model pin and chunking.
3. A `baseline_rag` cell emits a schema-valid `result.v2.json` row with a non-empty `embedder_id` — verified by a HELIX_BIN-gated test that actually RAN (not the MEMORY false-green skip path) and validated the on-disk row.
4. The same-model-same-budget invariant holds: `runners.DefaultContract` is reused verbatim, the embedding index is built out-of-band before the timed span (token metrics null), and BENCH.md/MODE.md document the budget exclusion.

The MEMORY false-green hazard was specifically defended against: both `helix` (via `make build`) and `helix-bench-rag` were built and `HELIX_BIN`+`HELIX_BENCH_RAG_BIN` were set, so the gated cell tests genuinely executed (visible non-zero runtimes), not skipped.

---

_Verified: 2026-06-21T00:44:43Z_
_Verifier: Claude (gsd-verifier)_
