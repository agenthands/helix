---
mode: baseline_rag
profile: baseline
---

# baseline_rag

The **retrieval-only RAG control arm** (ABLATE-04). Instead of the Helix daemon,
this mode drives the agent against a **standalone** `cmd/helix-bench-rag` MCP
server that exposes a fixed **four-tool** retrieval surface over a per-corpus
embedding index:

| Tool | Purpose |
|------|---------|
| `rag_search` | semantic k-NN search over the corpus embedding index |
| `rag_read_chunk` | return one chunk's full content by id |
| `grep` | regex search confined to the corpus root |
| `read_file` | read a corpus-root-relative file |

The `profile: baseline` frontmatter is **structurally required** (the resolver is
strict two-key with `KnownFields(true)`) but is **not** used to filter a Helix
tool surface here — `cmd/helix-bench-rag` is not the Helix daemon and has no Helix
profile. The arm is detected **by mode name** in the cell wiring (not by a
frontmatter marker — that keeps the two-key resolver change-free). The server
shares **no** code with the Helix daemon: it never links `internal/kernel`,
`internal/semantic`, or `internal/mcp` (enforced by `make vet`'s
`benchragleakage` analyzer and a transitive import-boundary test).

## Embedding index (chromem-go)

The per-corpus embedding index is built by the `bench/ragindex` leaf package over
the **cloned task repo working copy** (keyed by a content-deterministic
`corpus_sha`), backed by `chromem-go`. Embedder selection is OpenAI
`text-embedding-3-small` → Ollama `nomic-embed-text` → a deterministic
hermetic-CI **stub** (`stub-deterministic`); each backend records a **distinct
`embedder_id`** so a CI row is never mistaken for a real RAG measurement. The
pinned models, chunking strategy, and `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`
cache layout are documented in
[`../baseline_rag_agent/EMBED-CHOICE.md`](../baseline_rag_agent/EMBED-CHOICE.md).

## Same-contract, same-budget (criterion #4)

baseline_rag reuses `runners.DefaultContract` **verbatim** — the SAME model
snapshot (`claude-sonnet-4-5-20250929`) and per-task budget as `your_agent_full`,
never a separate config. Crucially, the embedding index is built/loaded
**out-of-band** — BEFORE the timed agent span — so the embedding-API calls are
**NOT charged** to the agent's per-task `tokens_input` / `tokens_output` budget
(only the agent's `rag_search` queries over the already-built index run inside the
timed span). Every baseline_rag result row records its `embedder_id`, and the arm
participates as a delta operand (`full_minus_baseline_rag`).
