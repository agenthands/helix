---
phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde
fixed_at: 2026-06-21T00:00:00Z
review_path: .planning/phases/83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde/83-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 83: Code Review Fix Report

**Source review:** 83-REVIEW.md
**Iteration:** 1
**Scope:** the 5 Warning findings (WR-01 … WR-05). Info findings (IN-01..IN-04) were out of scope.

**Summary:**
- Findings in scope: 5
- Fixed: 5
- Skipped: 0

Each fix was committed atomically with a `fix(83):` prefix. The isolation
invariant (cmd/helix-bench-rag must not import internal/kernel, internal/semantic,
or internal/mcp) is preserved — the dynamic leakage test and static benchragleakage
analyzer both still pass.

## Fixed Issues

### WR-01: Warm-reopen embedder can silently diverge from the recorded embedder_id

**Files modified:** `bench/ragindex/embedder.go`, `bench/ragindex/index.go`, `bench/ragindex/embedder_test.go`, `bench/runtime/rag.go`, `bench/runtime/subprocess/ragserver.go`
**Commit:** ccc34fee
**Applied fix:** Took the env-pinning (authoritative-selection) approach the review
recommended as its alternative — cleaner than reading chromem's unexported
collection metadata, which has no public accessor in v0.7.0. Added a new
`HELIX_RAG_EMBEDDER` env (`pinnedEmbedderEnv`). `selectEmbedder` now returns an
error and, when the pin is set, deterministically returns exactly that embedder via
a new `embedderFor` helper, **failing closed** if the pinned backend is not usable
in this process (OpenAI without key, Ollama unreachable, unknown id). The parent
`runRAGCell` already computes `embedderID := idx.EmbedderID()` from its cold build;
`StartRAGServer` now takes that `embedderID` and forwards it as
`HELIX_RAG_EMBEDDER`, so the spawned server can no longer re-probe and silently fall
back to the stub while the row still claims `ollama-*`. Added
`TestPinnedEmbedderHonoredOrFailClosed`; updated `TestEmbedderIDsDistinct` for the
new 3-value signature. **Requires human verification of the logic:** the fail-closed
semantics are a deliberate behavior change — a previously-"successful" warm reopen
under a transient Ollama outage will now error out rather than serve mismatched
vectors. This is the intended, honest outcome, but confirm it matches the desired
operational policy (error vs. degrade-and-relabel).

### WR-02: validatePath does not resolve symlinks

**Files modified:** `cmd/helix-bench-rag/tools.go`, `cmd/helix-bench-rag/main_test.go`
**Commit:** 904344f7
**Applied fix:** Added `filepath.EvalSymlinks` on both the corpus root and the
joined path, then re-assert containment against the *resolved* root (apples-to-apples,
which also handles the macOS `/var -> /private/var` temp-dir case). An in-corpus
symlink to an absolute outside path is now rejected; an in-corpus symlink that stays
under root still resolves and reads. Added `TestSymlinkEscapeRejected` covering both.

### WR-03: read_file / rag_search content is unbounded

**Files modified:** `cmd/helix-bench-rag/tools.go`, `cmd/helix-bench-rag/main_test.go`
**Commit:** 0f398c99
**Applied fix:** Added a 256 KiB byte cap with a `\n... [truncated]\n` marker to
`read_file`, a total-rendered-bytes ceiling to `ragSearch`, and clamped `k` to
`maxK = 50` (also addresses IN-01's symmetry concern as a side effect). Added
`TestReadFileByteCap`.

### WR-05: grep treats binary files as text

**Files modified:** `cmd/helix-bench-rag/tools.go`, `cmd/helix-bench-rag/main_test.go`
**Commit:** 8ebbd01c
**Applied fix:** Added a `grep -I`-style NUL-byte sniff over the first 8000 bytes of
each file in the grep walk; files containing a NUL are skipped so raw non-UTF-8 bytes
never reach the TextContent/NDJSON transport. Adjacent text files still match. Added
`TestGrepSkipsBinaryFiles`, which also asserts the output contains no NUL.

### WR-04: readResponses goroutine cannot observe `done` while blocked in ReadString

**Files modified:** `bench/runtime/rag.go`
**Commit:** 6f898af7
**Applied fix:** Changed `driveRAGServer`'s `defer close(done)` to also
`_ = h.Stdout.Close()`, so the blocked `ReadString` returns EOF immediately and the
reader goroutine is reaped when the drive returns rather than at the eventual Kill
(which can be tens of seconds later, across the verify span). Confirmed safe: all
reads complete before the close, and `cmd.Wait` (in StartRAGServer's owner goroutine)
will not double-close an already-closed `os.File`. The gated integration tests
(`TestBaselineRagEmitsRow`, `TestBaselineRagSameContractBudget`) drive this path and
pass.

## Verification

All required gates pass:

- `go build ./...` — clean.
- `go vet ./bench/... ./cmd/helix-bench-rag/... ./internal/lint/benchragleakage/...` — clean.
- `go test ./bench/ragindex/... ./cmd/helix-bench-rag/... ./internal/lint/benchragleakage/...` — pass.
- `make build` + `HELIX_BIN=… HELIX_BENCH_RAG_BIN=… go test ./bench/runtime/...` — pass;
  the HELIX_BIN-gated `TestBaselineRag*` cells genuinely RAN (not skipped) and passed.

---

_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
