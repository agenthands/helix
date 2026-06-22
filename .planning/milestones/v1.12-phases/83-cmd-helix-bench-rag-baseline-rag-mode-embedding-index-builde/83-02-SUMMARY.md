---
phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde
plan: 02
subsystem: bench
tags: [mcp-sdk, standalone-server, baseline-rag, import-boundary, vet-analyzer, go-analysis, ablate-04]

# Dependency graph
requires:
  - phase: 83
    plan: 01
    provides: "bench/ragindex leaf package (Index.Query, ragindex.Chunk, Open) the standalone server consumes"
provides:
  - "cmd/helix-bench-rag: standalone package-main MCP server exposing EXACTLY 4 tools (rag_search, rag_read_chunk, grep, read_file) over bench/ragindex, built via mcpsdk directly (never internal/mcp)"
  - "internal/lint/benchragleakage: go/analysis import-prefix analyzer forbidding internal/kernel + internal/semantic on the cmd/helix-bench-rag namespace"
  - "cmd/vet-bench-rag-leakage: singlechecker wrapper wired into make vet"
  - "Dynamic transitive (go/packages NeedDeps) import-boundary test proving the binary shares no daemon code"
affects: [83-03 (cell drive leg spawns cmd/helix-bench-rag as the MCP server instead of the Helix daemon)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Standalone MCP server via mcpsdk.NewServer + AddTool + StdioTransport, NEVER internal/mcp (which transitively links kernel+semantic)"
    - "Self-tracked tool-name set (benchServer.ToolNames) since the SDK exposes no public registered-tools accessor — backs the exactly-4 contract"
    - "querier interface (not concrete *ragindex.Index) so tool handlers are unit-testable with a no-network stub"
    - "Dual import-boundary gate: dynamic (go/packages NeedDeps BFS) + static (go/analysis singlechecker in make vet), exact-OR-slash-boundary match"
    - "validatePath confines all FS access to the corpus root: rejects absolute paths, .. segments, and post-Join escapes via exact-OR-separator containment"

key-files:
  created:
    - cmd/helix-bench-rag/main.go
    - cmd/helix-bench-rag/server.go
    - cmd/helix-bench-rag/tools.go
    - cmd/helix-bench-rag/main_test.go
    - cmd/helix-bench-rag/stub_test.go
    - cmd/helix-bench-rag/leakage_test.go
    - internal/lint/benchragleakage/analyzer.go
    - internal/lint/benchragleakage/analyzer_test.go
    - internal/lint/benchragleakage/testdata/src/github.com/agenthands/helix/cmd/helix-bench-rag/clean/clean.go
    - internal/lint/benchragleakage/testdata/src/github.com/agenthands/helix/cmd/helix-bench-rag/leaky/leaky.go
    - internal/lint/benchragleakage/testdata/src/github.com/agenthands/helix/cmd/helix-bench-rag/sibling/sibling.go
    - internal/lint/benchragleakage/testdata/src/github.com/agenthands/helix/internal/kernel/kernel.go
    - internal/lint/benchragleakage/testdata/src/github.com/agenthands/helix/internal/kernelextra/kernelextra.go
    - cmd/vet-bench-rag-leakage/main.go
  modified:
    - Makefile

key-decisions:
  - "stub_test.go (not main_test.go) holds the deterministic no-network querier stub; idx field is a querier interface so tool handlers are exercised with zero chromem/network — the cmd tests are fully hermetic (no HELIX_BIN, no API key)"
  - "rag_read_chunk reconstructs the chunk from disk via ragindex.Chunk(slash-normalized relPath) at the parsed ordinal rather than a chromem read-by-id — the Index exposes no read-by-id and the windower is deterministic, so ordinals line up with rag_search chunk_ids exactly"
  - "Tool-list assertion reads benchServer.ToolNames() (a self-tracked slice appended at AddTool time) because the go-sdk mcp.Server has no public list-registered-tools accessor"
  - "Analyzer is a PURE import-prefix gate (dropped all ablationleakage ChooseSource call-site machinery); internal/mcp is NOT listed explicitly because it transitively links both internal/kernel and internal/semantic, so any path through it trips the existing prefixes"
  - "Testdata uses the full-import-path GOPATH layout (testdata/src/github.com/agenthands/helix/...) matching the ablationleakage analog, with stand-in internal/kernel + internal/kernelextra packages so analysistest can resolve the forbidden import and the slash-boundary lookalike"

requirements-completed: [ABLATE-04]

# Metrics
duration: ~30min
completed: 2026-06-21
---

# Phase 83 Plan 02: cmd/helix-bench-rag Standalone 4-Tool MCP Server + Import-Boundary Gate Summary

**A provably-isolated `package main` MCP server exposing EXACTLY four tools (`rag_search`, `rag_read_chunk`, `grep`, `read_file`) over the Plan 01 `bench/ragindex` leaf — built via `mcpsdk.NewServer`+`AddTool`+`StdioTransport` directly (never `internal/mcp`) — backed by a dual import-boundary gate: a dynamic transitive `go/packages` NeedDeps test plus a static `go/analysis` singlechecker (`vet-bench-rag-leakage`) wired into `make vet`, both forbidding `internal/kernel` and `internal/semantic`.**

## Performance
- **Duration:** ~30 min
- **Started:** 2026-06-21
- **Completed:** 2026-06-21
- **Tasks:** 2 (both TDD)
- **Files modified:** 14 (13 created, Makefile)

## Accomplishments
- `cmd/helix-bench-rag` is a standalone `package main` MCP server: `mcpsdk.NewServer(&Implementation{Name:"helix-bench-rag"})` + exactly four `mcpsdk.AddTool` registrations + `StdioTransport` Run; `--help` works (cobra root, single `os.Exit`).
- `go list -deps ./cmd/helix-bench-rag` contains NO `internal/kernel`, `internal/semantic`, OR `internal/mcp` — the isolation invariant (ABLATE-04 #1c) holds.
- Tool surface is EXACTLY 4 (`{grep, rag_read_chunk, rag_search, read_file}`) — `TestToolListIsExactlyFour` asserts the name set; no `ping`/`echo`/`activate_project` copied (Pitfall 5).
- All three FS-touching tools (`read_file`, `grep`, `rag_read_chunk`) reject `..`, absolute paths, and post-Join escapes via `validatePath` — proven by `TestPathTraversalRejected` (T-83-02-01).
- The import boundary is gated TWICE: dynamically by `TestNoKernelSemanticImport` (go/packages `NeedDeps` BFS over the full transitive graph — a direct-only check would be a false pass) and statically by the `benchragleakage` analyzer in `make vet`.
- The analyzer uses the verbatim exact-OR-slash-boundary match (`path == forbidden || HasPrefix(path, forbidden+"/")`); a `sibling` fixture importing `internal/kernelextra` proves a bare-`HasPrefix` regression would be caught.

## Task Commits
1. **Task 1: standalone 4-tool MCP server + --help + tool-count + path-traversal** — `738914d1` (test RED) → `72d2d841` (feat GREEN)
2. **Task 2: transitive import-boundary test + static vet analyzer + make vet wiring** — `52701111` (test RED) → `348eade5` (feat GREEN)

## Files Created/Modified
- `cmd/helix-bench-rag/main.go` — cobra root via `newRootCmd().Execute()`; single `os.Exit` site.
- `cmd/helix-bench-rag/server.go` — `buildServer(querier, root)` → `benchServer` (mcpsdk server + tracked tool names); `newRootCmd` with `--corpus` flag, opens index out-of-band and serves over stdio; the four typed-arg AddTool registrations.
- `cmd/helix-bench-rag/tools.go` — `handlers` (idx + root); `validatePath` (corpus-root confinement); `ragSearch`/`readChunk`/`grep`/`readFile`.
- `cmd/helix-bench-rag/main_test.go` — `TestHelp`, `TestToolListIsExactlyFour`, `TestPathTraversalRejected`.
- `cmd/helix-bench-rag/stub_test.go` — deterministic no-network `querier` stub.
- `cmd/helix-bench-rag/leakage_test.go` — `TestNoKernelSemanticImport` (go/packages NeedDeps transitive BFS).
- `internal/lint/benchragleakage/analyzer.go` — pure import-prefix analyzer; `checkedPkgPrefix=cmd/helix-bench-rag`, forbidden={internal/kernel, internal/semantic}.
- `internal/lint/benchragleakage/analyzer_test.go` — analysistest over leaky(flagged)/clean/sibling.
- `internal/lint/benchragleakage/testdata/...` — 5 fixtures (clean, leaky, sibling + stand-in kernel/kernelextra).
- `cmd/vet-bench-rag-leakage/main.go` — `singlechecker.Main(benchragleakage.Analyzer)`.
- `Makefile` — `VETTOOL_BENCH_RAG_LEAKAGE` var + install rule + `vet:` prerequisite + recipe line.

## Decisions Made
See `key-decisions` frontmatter. Headline: hermetic `querier`-interface stub (no chromem/network in cmd tests); `rag_read_chunk` reconstructs chunks via the deterministic `ragindex.Chunk` windower so ordinals match `rag_search` chunk_ids; self-tracked `ToolNames` slice backs the exactly-4 assertion (the SDK has no public tool-list accessor); `internal/mcp` is implicitly forbidden via the two transitive prefixes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Split the index `querier` out behind an interface + a separate stub file**
- **Found during:** Task 1 (GREEN)
- **Issue:** The plan's tests call `buildServer`/`handlers` with a stub index, but `*ragindex.Index` requires a live chromem DB (network/embedder), defeating a hermetic cmd test. The plan named the stub helper `newStubIndex()` without specifying its home or the seam.
- **Fix:** Introduced a narrow `querier` interface (`Query(ctx, q, k)`) as the `handlers.idx`/`buildServer` parameter type, and placed `newStubIndex()` in a dedicated `stub_test.go`. The production `newRootCmd` still passes the concrete `*ragindex.Index`. Zero production-behavior change; purely a testability seam mirroring Plan 01's `openWith` embedder-injection discipline.
- **Files modified:** cmd/helix-bench-rag/server.go, cmd/helix-bench-rag/tools.go, cmd/helix-bench-rag/stub_test.go
- **Verification:** `go test ./cmd/helix-bench-rag/` green; `go vet` clean.
- **Committed in:** `72d2d841` (Task 1 GREEN).

**2. [Design clarification] rag_search `Query` signature is 3-arg, not the 5-arg shown in PATTERNS**
- **Found during:** Task 1 (GREEN)
- **Issue:** PATTERNS.md sketched `idx.Query(ctx, query, k, nil, nil)` (mirroring chromem's `coll.Query`), but the Plan 01 public API is `(*Index).Query(ctx, queryText, k)` — the two trailing chromem filter args are encapsulated inside ragindex.
- **Fix:** Called the actual 3-arg `Query`. No functional impact (the encapsulated filters were always nil).
- **Files modified:** cmd/helix-bench-rag/tools.go
- **Committed in:** `72d2d841` (Task 1 GREEN).

---

**Total deviations:** 2 (1 Rule-3 testability seam, 1 design clarification). Neither expands scope; both are required to compile a hermetic, correctly-typed server.

## Threat Model Compliance
- **T-83-02-01 (path tampering):** `validatePath` rejects absolute paths, `..` segments, and any post-`filepath.Join` result outside the corpus root (exact-OR-separator containment). All three FS tools route through it. Proven by `TestPathTraversalRejected` over `../etc/passwd`, `/etc/passwd`, and `a/../../etc/passwd`.
- **T-83-02-02 (kernel/semantic leakage):** Dual gate — `TestNoKernelSemanticImport` (dynamic transitive NeedDeps) + `benchragleakage` analyzer in `make vet` (static). `go list -deps` confirms CLEAN.
- **T-83-02-03 (tool-surface spoofing):** EXACTLY 4 tools registered; `TestToolListIsExactlyFour` rejects any drift.

## TDD Gate Compliance
Both tasks followed RED→GREEN. Git log shows the gate commits in order:
- Task 1: `738914d1` test(RED) → `72d2d841` feat(GREEN).
- Task 2: `52701111` test(RED) → `348eade5` feat(GREEN).
Note: `TestNoKernelSemanticImport` (the dynamic boundary guard, added in Task 2 RED) PASSED immediately because Task 1 already produced a clean binary — this is a guard/regression test, not a behavior-driving RED, so no fail-then-pass flip is expected for it. The analyzer's analysistest fixtures provided the genuine green→red flip (leaky flagged, clean/sibling silent).

## Known Stubs
- `cmd/helix-bench-rag/stub_test.go` `stubIndex` is a TEST-ONLY deterministic querier; it never ships in the binary. The production path uses the real `*ragindex.Index`. Not a defect.

## Next Phase Readiness
- Plan 03's cell drive leg can spawn `cmd/helix-bench-rag --corpus <cloned-repo>` as the MCP server in place of the Helix daemon; the index is built out-of-band (Plan 01 `ragindex.Open`), excluded from the timed agent budget.
- The dual import-boundary gate guarantees the control arm stays daemon-code-free as Plan 03 wires it in; any future edit that re-introduces a kernel/semantic edge fails both `go test ./cmd/helix-bench-rag/...` and `make vet`.

## Self-Check: PASSED

All 13 created files exist on disk; both feat commits (`72d2d841`, `348eade5`) and both test commits (`738914d1`, `52701111`) are present in git history. `go build`, `go vet`, and `go test` of `./cmd/helix-bench-rag/...` and `./internal/lint/benchragleakage/...` all pass; `--help` exits 0; `go list -deps` is CLEAN of internal/kernel|semantic|mcp.

---
*Phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde*
*Completed: 2026-06-21*
