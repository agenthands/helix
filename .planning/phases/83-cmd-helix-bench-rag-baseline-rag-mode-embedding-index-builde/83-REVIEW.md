---
phase: 83-cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 27
files_reviewed_list:
  - bench/ragindex/cache.go
  - bench/ragindex/chunk.go
  - bench/ragindex/embedder.go
  - bench/ragindex/index.go
  - bench/ragindex/cache_test.go
  - bench/ragindex/chunk_test.go
  - bench/ragindex/embedder_test.go
  - bench/ragindex/index_test.go
  - cmd/helix-bench-rag/main.go
  - cmd/helix-bench-rag/server.go
  - cmd/helix-bench-rag/tools.go
  - cmd/helix-bench-rag/main_test.go
  - cmd/helix-bench-rag/leakage_test.go
  - cmd/helix-bench-rag/stub_test.go
  - cmd/vet-bench-rag-leakage/main.go
  - internal/lint/benchragleakage/analyzer.go
  - internal/lint/benchragleakage/analyzer_test.go
  - bench/runtime/result.go
  - bench/runtime/result_test.go
  - bench/runtime/cell.go
  - bench/runtime/rag.go
  - bench/runtime/deltas.go
  - bench/runtime/matrix.go
  - bench/runtime/baseline_rag_test.go
  - bench/runtime/subprocess/ragserver.go
  - bench/runtime/drive.go
  - Makefile
findings:
  critical: 0
  warning: 5
  info: 4
  total: 9
status: issues_found
---

# Phase 83: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 27
**Status:** issues_found

## Summary

Phase 83 adds a standalone, provably-isolated `cmd/helix-bench-rag` MCP server
(chromem-go embeddings over a leaf `bench/ragindex` package), a dual static+dynamic
import-boundary gate, and the `runRAGCell` baseline_rag drive leg in `bench/runtime`.

The high-stakes invariants are met:

- **Isolation** is enforced both statically (`benchragleakage` analyzer wired into
  `make vet`) and dynamically (`leakage_test.go` transitive `go/packages` NeedDeps
  scan). Both use slash-boundary matching, with a lookalike regression test
  (`internal/kernelextra`). `go build` confirms the new packages link cleanly.
- **Path traversal** is blocked by `validatePath` for `grep`/`read_file`/
  `rag_read_chunk` (rejects abs paths, `..` segments, and post-join prefix escape),
  with a dedicated rejection test.
- **Secret handling** is correct: `OPENAI_API_KEY` is read only to decide
  availability and forwarded to the subprocess via env, never logged or echoed; the
  Ollama base URL is a pinned constant (no SSRF arg surface).
- **Budget exclusion** is real: `ragindex.Open` runs before the `start := time.Now()`
  anchor; the spawned server re-opens the warm cache (zero re-embeds); token metrics
  are emitted as explicit null, never a fabricated 0.
- **embedder_id** is recorded on every baseline_rag row, omitempty-dropped for honest
  modes, and round-trip tested.
- **Subprocess lifecycle**: `RAGHandle.Kill` is `sync.Once`-guarded, group-SIGKILLs
  via Setpgid, and observes a single owner `cmd.Wait()` goroutine bounded by a 5s
  timeout — matching the daemon-leg discipline. chromem-go v0.7.0 exposes no
  `Close()`; the persistent DB flushes on write, so there is no leaked DB handle.

No BLOCKER defects were found. The findings below are robustness gaps (warm-reopen
embedder consistency, symlink defense-in-depth, an unbounded read surface, a
goroutine that only reaps at Kill) and quality items.

## Warnings

### WR-01: Warm-reopen embedder can silently diverge from the recorded embedder_id

**File:** `bench/ragindex/embedder.go:50-61`, `bench/runtime/rag.go:101-105`, `bench/runtime/subprocess/ragserver.go:168-175`

**Issue:** The embedder is chosen independently in two processes. The parent
(`runRAGCell`) calls `ragindex.Open` → `selectEmbedder`, builds the cold index, and
records `embedderID := idx.EmbedderID()` onto the row. The spawned server re-runs
`selectEmbedder` on warm reopen. chromem does NOT persist the `EmbeddingFunc`
(documented Pitfall 1), so the *query* embedder is whatever the subprocess selects,
while the *document* vectors on disk were produced by the parent's selection.

`OPENAI_API_KEY` and `HELIX_RAG_FORCE_STUB` are forwarded, keeping those two branches
consistent. The gap is the **Ollama probe**: `ollamaReachable()` is re-evaluated in
the subprocess via a live `net.DialTimeout`. If the parent built with Ollama
(`embedder_id` recorded as `ollama-nomic-embed-text`) but the subprocess momentarily
cannot reach the daemon (timeout, daemon bounce, 250ms dial flake), the server falls
back to the **stub** embedder. Result: doc vectors are Ollama-space, query vectors are
stub-space → cosine scores are meaningless, yet the row still claims
`embedder_id == ollama-nomic-embed-text`. The recorded provenance no longer matches
the embedder that actually answered the query — exactly the "never mistaken for a real
measurement" guarantee the embedder_id field exists to protect.

**Fix:** Pass the parent-selected embedder identity to the server explicitly rather
than re-deriving it, and make the server fail-closed on mismatch. Read back the
collection's persisted `embedder` metadata in `openWith` and refuse to serve if the
freshly-selected `embedderID` disagrees:

```go
// in openWith, after GetOrCreateCollection on a warm (Count()>0) path:
if persisted := coll.Metadata()["embedder"]; persisted != "" && persisted != embedderID {
    return nil, fmt.Errorf(
        "ragindex: embedder mismatch on warm reopen: index built with %q, this process selected %q",
        persisted, embedderID)
}
```

Alternatively forward a pinned `HELIX_RAG_EMBEDDER=<embedder_id>` env from the parent
so the subprocess selects deterministically instead of re-probing Ollama.

### WR-02: validatePath does not resolve symlinks (defense-in-depth gap on direct invocation)

**File:** `cmd/helix-bench-rag/tools.go:34-60`

**Issue:** `validatePath` performs a lexical `filepath.Clean` + `filepath.Join` +
prefix containment check but never calls `filepath.EvalSymlinks`. A symlink *inside*
the corpus root that targets an absolute path outside it (e.g. `link -> /etc/passwd`)
passes every lexical check — `joined` is still under `rootAbs` — and `os.ReadFile`
then follows the link out of the sandbox. The same applies to `grep` walking a
directory that contains such a link.

Through the bench harness this is **not currently exploitable**: `Sandbox.CloneRepo`
(`internal/eval/sandbox/sandbox.go:133-136`) skips symlinks during the clone, so the
corpus working copy is symlink-free. But `cmd/helix-bench-rag` is a standalone binary
that accepts an arbitrary `--corpus`; a direct invocation (or a future caller that
clones differently) over a tree containing symlinks would leak file contents outside
root. The containment check should not rely on an upstream caller's cloning policy.

**Fix:** Resolve symlinks and re-assert containment on the resolved path:

```go
resolved, err := filepath.EvalSymlinks(joined)
if err != nil {
    return "", fmt.Errorf("resolve path: %w", err)
}
if resolved != rootAbs && !strings.HasPrefix(resolved, rootAbs+string(os.PathSeparator)) {
    return "", fmt.Errorf("path escapes corpus root after symlink resolution: %q", rel)
}
return resolved, nil
```

(Resolve `rootAbs` with `EvalSymlinks` too so the prefix comparison is apples-to-apples.)

### WR-03: read_file / rag_search content is unbounded — a single tool call can flood the MCP transport

**File:** `cmd/helix-bench-rag/tools.go:189-199` (`readFile`), `cmd/helix-bench-rag/tools.go:62-89` (`ragSearch`)

**Issue:** `grep` deliberately caps output at `maxGrepMatches = 200` "so a pathological
pattern cannot flood the MCP transport" (tools.go:19-21), but `read_file` returns
`string(b)` for an arbitrarily large file with no size cap, and `ragSearch` concatenates
full chunk contents for up to `k` hits with no total-bytes ceiling. The threat the grep
cap addresses (a single tool call materializing an unbounded response onto the stdio
JSON-RPC pipe) applies equally here — a multi-megabyte file read, or a large `k`, will
serialize the whole payload into one `TextContent`. This is an inconsistency in the
stated transport-protection discipline, and on a corpus with large generated files it
risks an oversized frame.

**Fix:** Apply a byte cap mirroring the grep discipline. For `read_file`, truncate with
a marker:

```go
const maxReadFileBytes = 256 * 1024
if len(b) > maxReadFileBytes {
    return string(b[:maxReadFileBytes]) + "\n... [truncated]\n", nil
}
```

For `ragSearch`, also bound `k` (e.g. cap at a small max) so a caller cannot request an
arbitrarily large concatenation.

### WR-04: readResponses goroutine cannot observe `done` while blocked in ReadString — only reaped at Kill

**File:** `bench/runtime/rag.go:286-290`, `bench/runtime/drive.go:204-222`

**Issue:** `driveRAGServer` starts `go readResponses(h.Stdout, respCh, done)` and
`defer close(done)`. `readResponses` only selects on `done` *after* it has parsed a
line (drive.go:209-216); while it is parked inside `reader.ReadString('\n')` waiting
for the next byte, it cannot react to `done` being closed. So when `driveRAGServer`
returns, `done` closes but the reader stays blocked on the still-open stdout pipe. In
`runRAGCell` the server is not killed until *after* `prePatchSnapshot`-equivalent verify
work — i.e. after `RunTests`/`runVerify`, which can run for tens of seconds. During that
whole window the reader goroutine is alive and pinned to the live process's stdout.

It is not a permanent leak (the goroutine returns on EOF once `h.Kill()` closes the
process), but it is a longer-lived goroutine than the comments imply ("Drop rather than
leak" suggests prompt exit), and it holds `h.Stdout` open across the verify span. If a
future refactor moves the drive into a long-lived loop or removes the Kill, this becomes
a genuine leak.

**Fix:** Close `h.Stdout` when the drive returns so the blocked `ReadString` unblocks
with EOF immediately, instead of waiting for the eventual Kill:

```go
defer func() {
    close(done)
    _ = h.Stdout.Close() // unblock readResponses' ReadString now, not at Kill
}()
```

(Confirm `RAGHandle.Stdout` closing before `cmd.Wait` is safe — for an `os/exec`
`StdoutPipe` the caller may close it; `Wait` will not double-close.)

### WR-05: grep treats binary files as text and emits raw bytes onto the JSON-RPC transport

**File:** `cmd/helix-bench-rag/tools.go:141-157`

**Issue:** `grep`'s directory walk (`filepath.WalkDir` over `abs`) calls `search(p)` on
**every** non-directory file, including binaries, images, and (since the corpus is a
cloned repo) any committed `.so`/`.png`/`.pdf`. `search` does
`strings.Split(string(b), "\n")` and `re.MatchString(line)` on raw bytes, then emits
matching "lines" verbatim into the result. A binary file that happens to contain the
pattern bytes will inject arbitrary non-UTF-8 bytes (including NULs and control chars)
into a `TextContent` field that is then JSON-marshaled onto the stdio transport. At best
this produces garbage matches; at worst it corrupts the NDJSON framing the driver parses
in `readResponses`/`parseRPCLine`.

**Fix:** Skip non-text files. A cheap heuristic is a NUL-byte sniff on the first chunk:

```go
b, readErr := os.ReadFile(p)
if readErr != nil { return readErr }
if bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 {
    return nil // binary file — skip
}
```

(Or restrict the walk to a known text-extension allowlist.)

## Info

### IN-01: rag_search exposes no upper bound on `k`; defaultK is applied only for k<=0

**File:** `cmd/helix-bench-rag/tools.go:67-69`, `bench/ragindex/index.go:126-136`

**Issue:** `ragSearch` substitutes `defaultK` only when `k <= 0`; a caller-supplied huge
`k` is passed straight to `Index.Query`, which clamps to the document count. Functionally
safe (no crash), but it pairs with WR-03 to allow a large concatenated response. Consider
clamping `k` to a documented maximum for symmetry with `maxGrepMatches`.

### IN-02: CorpusSHA and buildDocuments walk and hash/embed the entire tree, including `.git`

**File:** `bench/ragindex/cache.go:49-78`, `bench/ragindex/index.go:92-123`

**Issue:** Both walkers descend into every file under `root`, including the cloned
repo's `.git/` objects and any vendored/generated content. This inflates the corpus SHA
surface (any `.git` index churn flips the cache key — though clones are deterministic
here) and embeds non-source blobs into the k-NN index, diluting retrieval quality and
wasting cold-build embedding cost. The bench clone may or may not include `.git`; if it
does, RAG results will surface git internals. Consider skipping `.git/` and other
non-source directories (or honoring a simple ignore list) in both walkers so the index
and SHA reflect source content only.

### IN-03: handler tool bodies (grep/readChunk/readFile/ragSearch beyond path-traversal) lack direct unit tests

**File:** `cmd/helix-bench-rag/tools.go`, `cmd/helix-bench-rag/main_test.go`

**Issue:** `TestPathTraversalRejected` covers the validator and `TestToolListIsExactlyFour`
covers registration, but the happy-path behavior of `grep` (match formatting, the
`maxGrepMatches` cap, directory walk, `fs.SkipAll`), `readChunk` (ordinal-in-range,
malformed id, reconstruction parity with the index), and `ragSearch` (multi-hit
rendering, the `---` separator, empty-results "no results") is exercised only
indirectly through the HELIX_BIN-gated integration test, which SKIPs in a plain
`go test ./...` (per project MEMORY: bench tests are false-green without HELIX_BIN).
Add hermetic unit tests for these handlers using the existing `stubIndex` and a temp
corpus so the core tool logic is covered without the binary.

### IN-04: comment in computeTaskDeltas describes a fall-through that is dead for required modes

**File:** `bench/runtime/deltas.go:204-230`

**Issue:** When `cmp.optional` is false and the row is absent, the code does not `continue`
— it falls through to `other := otherRow.metrics` with a zero-value `loadedRow`. The
comment correctly asserts a required operand "can never be absent here" (gated by
`firstMissingMode`), so this path is dead in practice and produces no wrong output today.
But the structure is fragile: if `requiredModes`/`deltaComparisons` ever drift so a
non-optional comparison references a non-required mode, this silently emits an all-`{}`
delta (every metric skipped) instead of failing. Make the invariant explicit by guarding
the required-but-absent case:

```go
if !present {
    if cmp.optional {
        continue
    }
    // required operand missing — should be impossible (firstMissingMode gated it)
    panic(fmt.Sprintf("computeTaskDeltas: required operand %q absent", cmp.other))
}
```

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
