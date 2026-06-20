# baseline_rag — Embedder & Chunking Choice

This document is the model-pin and chunking record for the `baseline_rag`
competitive control arm (ABLATE-04 success criterion #2). It is a plain doc, NOT
a resolver `MODE.md` — it carries no frontmatter and is not read by the
`bench/runners` mode resolver. The mode-name → profile resolution still lives in
`bench/runners/baseline_rag/MODE.md`.

The embedding index itself is built and cached by the leaf package
`bench/ragindex` (`cache.go`, `chunk.go`, `embedder.go`, `index.go`).

## Why a documented embedder pin exists

`baseline_rag` is the RAG control arm in the leaderboard. A reviewer's first
pushback on any RAG baseline is "you handicapped it with a weak embedder." This
document pins the embedder explicitly so the arm is honest and reproducible: the
same `(corpus, embedder_model)` always yields the same cached index, and every
`result.v2.json` row records the `embedder_id` that produced it so a CI/offline
row can never be mistaken for a real RAG measurement.

## Embedder backends (selection order)

`bench/ragindex/embedder.go selectEmbedder()` chooses a backend by availability
and returns a distinct `embedder_id` for each:

1. **Primary — OpenAI `text-embedding-3-small`**
   (`embedder_id = openai-text-embedding-3-small`).
   Selected when `OPENAI_API_KEY` is set in the environment. Uses chromem-go's
   built-in `NewEmbeddingFuncDefault()`, which reads `OPENAI_API_KEY` and calls
   the OpenAI embeddings API. The key value is **never** logged or persisted —
   only the `embedder_id` model string is recorded (secret-leak mitigation).
   This is the live/soak baseline.

2. **Fallback — Ollama `nomic-embed-text`**
   (`embedder_id = ollama-nomic-embed-text`).
   Selected when no `OPENAI_API_KEY` is set and a local Ollama daemon is
   reachable. Uses `NewEmbeddingFuncOllama("nomic-embed-text", <base URL>)`. The
   Ollama base URL is **pinned to the default constant** `http://localhost:11434/api`
   and is never accepted as a tool/function argument (SSRF mitigation).

3. **CI / offline — deterministic stub**
   (`embedder_id = stub-deterministic`).
   Selected when neither OpenAI nor Ollama is available (hermetic CI), or when
   `HELIX_RAG_FORCE_STUB` is set. It is a network-free, position-independent
   byte-value histogram of the chunk text, L2-normalized — fully deterministic
   for a fixed input. It exists so the whole `bench/ragindex` package is
   unit-testable with zero network, and its distinct `embedder_id` guarantees a
   CI row is **never mistaken for a real RAG measurement**. It is NOT a quality
   embedder by design.

## Chunking strategy

Implemented in `bench/ragindex/chunk.go` (`func Chunk(content, relPath string)`):

- **Fixed line-window chunker.** Each chunk is a contiguous window of
  **40 lines** with an **8-line overlap** between adjacent windows (step = 32
  lines). The overlap reduces the chance of a relevant symbol being split across
  a window boundary.
- **Stable chunk IDs.** Each chunk's ID is `<rel-path>#<ordinal>` with the
  ordinal starting at `0` (e.g. `pkg/auth.go#0`, `pkg/auth.go#1`). The same
  `(content, rel-path)` input always produces the same chunks and IDs.
- **Empty / whitespace-only files yield zero chunks** (nothing to embed). A file
  whose content fits within a single window yields exactly one chunk.

## Cache layout

The per-corpus index is persisted by chromem-go at:

```
$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/
```

- **`corpus_sha`** (`bench/ragindex/cache.go CorpusSHA`) is a hex sha256 over the
  **sorted** set of `(relative-path, sha256(content))` pairs — it depends only
  on file content, never on mtime or directory-walk order. Changing one byte of
  any file changes the `corpus_sha` and invalidates the cache; an unchanged
  corpus always maps to the same `corpus_sha` and reuses the cached index.
- **Cache root resolution** (`cacheDir()`), in precedence order:
  1. `$HELIX_CACHE_DIR` (used verbatim if set),
  2. `os.UserCacheDir()/helix` (`~/.cache/helix` on Linux,
     `~/Library/Caches/helix` on macOS),
  3. `~/.helix/cache` (last-resort fallback).
- An index built once for a `(corpus_sha, embedder_id)` is reused on a warm
  reopen of the same path with **zero re-embeds**: chromem-go auto-loads the
  gob-encoded documents and embeddings, and the same embedder is re-supplied to
  `GetOrCreateCollection` (the embedding function is not itself persisted).

The `embedder_id` is recorded both in the chromem collection metadata and in
each `baseline_rag` `result.v2.json` row, so an embedder mismatch on reopen is
detectable and every row is provenance-tagged.
