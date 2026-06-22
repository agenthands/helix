# Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder - Research

**Researched:** 2026-06-21
**Domain:** Go MCP server (standalone), embedding-RAG control arm, vector-DB index builder, bench-harness integration
**Confidence:** HIGH

## Summary

Phase 83 implements the `baseline_rag` competitive control arm. Three sub-deliverables: (1) a **standalone** `cmd/helix-bench-rag` MCP server exposing exactly 4 tools (`rag_search`, `rag_read_chunk`, `grep`, `read_file`) that provably shares no code with the Helix daemon (no `internal/kernel/` or `internal/semantic/` import); (2) a per-corpus **embedding index builder** backed by `chromem-go` (a zero-dependency embeddable Go vector DB) using OpenAI `text-embedding-3-small` primary / Ollama `nomic-embed-text` fallback, cached at `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`; and (3) wiring so a `baseline_rag` ToolBench run produces schema-valid `result.v2.json` rows that record the `embedder_id`, under the **same fairness-contract model snapshot and budget** as `your_agent_full`, with embedding-API calls excluded from the agent's per-task budget.

The codebase is exceptionally well-prepared. Phase 80 left `baseline_rag` as a fail-closed stub short-circuited **by mode name** inside `RunCell` (`bench/runtime/cell.go:429-433`); this phase replaces that short-circuit with a real run path. The fairness contract (`bench/runners/fairness_contract.go`, `DefaultContract`) is a compile-time literal already validated unconditionally in `RunCell`. The `result.v2` builder (`bench/runtime/result.go`) emits a typed doc whose schema (`bench/schema/result.v2.schema.json`) leaves top-level `additionalProperties` **open**, so adding an `embedder_id` open-provenance key is a minor additive change (no v3 bump). The `vet-ablation-leakage` analyzer (`internal/lint/ablationleakage/analyzer.go`) is the exact static import-boundary pattern to extend for the "no kernel/semantic import" gate. The official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk v1.5.0`) is the same one the daemon uses; the standalone server uses `mcpsdk.NewServer` + `mcpsdk.AddTool` + `StdioTransport` directly, **never** importing `internal/mcp` (which transitively pulls kernel/semantic).

**Primary recommendation:** Build `cmd/helix-bench-rag` as a self-contained `package main` that imports only `github.com/modelcontextprotocol/go-sdk/mcp`, `github.com/philippgille/chromem-go`, stdlib, and a new leaf package under `bench/` (e.g. `bench/ragindex/`). Use `chromem-go`'s built-in `NewEmbeddingFuncDefault()` (OpenAI `text-embedding-3-small`) and `NewEmbeddingFuncOllama("nomic-embed-text", ...)` so no direct `openai-go` integration is needed. Wire the `baseline_rag` arm into `RunCell` by replacing the Phase-80 fail-close with a dedicated RAG drive path that spawns `cmd/helix-bench-rag` as the MCP server instead of the Helix daemon.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 4-tool MCP server (`rag_search`/`rag_read_chunk`/`grep`/`read_file`) | Standalone `cmd/helix-bench-rag` (`package main`) | — | Success criterion #1 demands provable isolation from the daemon; it must NOT be a Helix profile |
| Embedding index build + cache | New leaf pkg `bench/ragindex/` | `cmd/helix-bench-rag` (build-on-demand) | Reusable, unit-testable; the cmd server consumes it. MUST also avoid kernel/semantic to stay under the vet gate if imported by the server |
| Per-corpus index cache (`$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`) | `bench/ragindex/` | — | chromem-go `NewPersistentDB(path, compress)` owns disk persistence |
| Fairness contract (model snapshot + budget) | `bench/runners` (`DefaultContract`) | `bench/runtime` cell | Single source of truth already enforced; baseline_rag reuses it verbatim |
| `result.v2.json` row + `embedder_id` | `bench/runtime/result.go` (`BuildResult`) | `bench/schema` | Open provenance key, additive-only minor change |
| Matrix/cell orchestration for baseline_rag | `bench/runtime/cell.go` (`RunCell`) | `bench/runtime/matrix.go` | Replace the Phase-80 fail-close with a real drive leg |
| Import-boundary enforcement | `internal/lint/ablationleakage` + new `cmd/vet-*` | `make vet` | Static, compile-time proof of no kernel/semantic leakage |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/modelcontextprotocol/go-sdk/mcp` | v1.5.0 | MCP server for the 4 RAG tools | `[VERIFIED: go.mod]` Same SDK the Helix daemon uses (`internal/mcp/server.go`) — consistent tool-registration shape |
| `github.com/philippgille/chromem-go` | v0.7.0 | Embeddable Go vector DB: persistent index, query, built-in OpenAI+Ollama embedding funcs | `[VERIFIED: go list -m + pkg.go.dev]` Zero non-stdlib deps (clean go.mod); ships `NewEmbeddingFuncDefault`/`NewEmbeddingFuncOllama` covering BOTH required embedders. Single-binary-constraint friendly |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/openai/openai-go` | v1.12.0 | Direct OpenAI embeddings API | `[VERIFIED: go.mod + vendored cache]` ALREADY in go.mod. Use ONLY if chromem-go's built-in OpenAI func proves insufficient (e.g. need `dimensions` param). `EmbeddingModelTextEmbedding3Small` const confirmed at `embedding.go:121` |
| `github.com/spf13/cobra` | v1.10.2 | `--help` for `cmd/helix-bench-rag` | `[VERIFIED: go.mod]` Success criterion #1 needs `--help`; mirror `cmd/helix-bench/main.go` cobra root |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | result.v2 validate-on-write | `[VERIFIED: go.mod]` Already used by `bench/runtime.Validate` — no change, reuse |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| chromem-go built-in `NewEmbeddingFuncDefault` | Direct `openai-go` `Embeddings.New` | Direct gives `dimensions` control + tighter error surface, but adds code and an explicit client. Built-in func reads `OPENAI_API_KEY` env and is one line. Prefer built-in; keep openai-go as documented fallback |
| chromem-go (persistent) | hand-rolled cosine-similarity over a JSON file | Don't hand-roll — see Don't Hand-Roll table |
| Importing `internal/mcp` | direct `mcpsdk.NewServer` | `internal/mcp` transitively imports kernel+semantic → would FAIL the vet gate. MUST use the SDK directly |

**Installation:**
```bash
go get github.com/philippgille/chromem-go@v0.7.0
# openai-go, mcp go-sdk, cobra, jsonschema already in go.mod
```

**Version verification (done this session):**
- `chromem-go`: `go list -m github.com/philippgille/chromem-go@latest` → `v0.7.0`. `git ls-remote --tags` shows v0.3.0→v0.7.0 (multi-release history, real maintainer philippgille). go.mod has **no `require` block** (zero transitive deps). `[VERIFIED: go list + git ls-remote]`
- `openai-go v1.12.0`: vendored in module cache; `embedding.go:121` defines `EmbeddingModelTextEmbedding3Small`. `[VERIFIED: module cache grep]`

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/philippgille/chromem-go` | Go proxy | v0.3.0→v0.7.0 (multi-year) | popular embeddable vector DB | github.com/philippgille/chromem-go | OK | Approved (verify at human-verify gate) |
| `github.com/openai/openai-go` | Go proxy | already in tree | official OpenAI SDK | github.com/openai/openai-go | OK | Already a dependency |
| `github.com/modelcontextprotocol/go-sdk` | Go proxy | already in tree | official MCP SDK | github.com/modelcontextprotocol/go-sdk | OK | Already a dependency |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

> The `package-legitimacy check` seam was UNAVAILABLE this session (`gsd-tools query` returned SEAM_UNAVAILABLE). chromem-go legitimacy was instead established by: (a) registry resolution via `go list -m`; (b) multi-release git tag history; (c) zero transitive deps; (d) verified real GitHub source repo. Go modules have no postinstall mechanism, so the npm-style postinstall risk does not apply. The planner SHOULD still gate the `go get github.com/philippgille/chromem-go` install behind a `checkpoint:human-verify` task per the package-legitimacy protocol, since the automated seam did not run.

## Architecture Patterns

### System Architecture Diagram

```
                    cmd/helix-bench-rag (standalone package main)
                    ─────────────────────────────────────────────
  bench cell ──spawns──>  MCP server (mcpsdk.NewServer + StdioTransport)
  (RunCell,                     │
   baseline_rag arm)            ├── tool: rag_search(query, k) ─────┐
                                ├── tool: rag_read_chunk(chunk_id)  │
                                ├── tool: grep(pattern, path)       │  (no kernel,
                                └── tool: read_file(path)           │   no semantic)
                                          │                          │
                                          ▼                          │
                              bench/ragindex (leaf pkg) <────────────┘
                              ─────────────────────────
   corpus dir ──corpus_sha──> Index { chromem.NewPersistentDB(
                                  $HELIX_CACHE_DIR/bench-rag-index/<sha>/, compress) }
                                          │
            cache MISS ──build──> chunk files → EmbeddingFunc(ctx, text)
                                          │            │
                                          │     OpenAI text-embedding-3-small (primary)
                                          │     Ollama nomic-embed-text     (fallback)
                                          │
            cache HIT ──load───>  chromem auto-loads gob-encoded docs+embeddings
                                          │
                                          ▼
                              Query(ctx, queryText, k, nil, nil) → []Result
                                          │
                                          ▼
  ── result.v2.json row (BuildResult): + embedder_id provenance key, model_id from DefaultContract
```

File-to-implementation mapping is in Component Responsibilities (the table above), not the diagram.

### Recommended Project Structure
```
cmd/helix-bench-rag/
├── main.go             # cobra root + --help; wires MCP server, stdio Run
├── server.go           # mcpsdk.NewServer + 4 AddTool registrations (rag_search, rag_read_chunk, grep, read_file)
├── tools.go            # tool handlers (query index, read chunk, grep, read_file)
├── leakage_test.go     # go/packages import-set test: assert NO internal/kernel, internal/semantic
└── helptest / main_test.go  # --help works; tool-list == exactly 4

bench/ragindex/
├── index.go            # Build/Open over chromem.NewPersistentDB; CorpusSHA; cache layout
├── chunk.go            # chunking strategy (documented in EMBED-CHOICE.md)
├── embedder.go         # selectEmbedder(): OpenAI primary, Ollama fallback; returns embedder_id
└── cache.go            # $HELIX_CACHE_DIR resolution + <corpus_sha> path

bench/runners/baseline_rag_agent/
└── EMBED-CHOICE.md     # model pin + chunking strategy doc (success criterion #2)

bench/runtime/
├── cell.go             # REPLACE fail-close (line 429) with baseline_rag drive leg
└── result.go           # add embedder_id open-provenance key to ResultInput/resultDoc

bench/runtime/subprocess/
└── ragserver.go        # StartRAGServer: spawn cmd/helix-bench-rag over the per-cell socket (sibling of daemon.go's StartDaemon)
```

### Pattern 1: Standalone MCP server (no internal/mcp import)
**What:** Construct the SDK server directly so the binary links only the SDK + chromem-go, never the daemon's tool surface.
**When to use:** The `cmd/helix-bench-rag` entrypoint.
**Example:**
```go
// Source: internal/mcp/server.go:81-90,192 (pattern), used WITHOUT importing internal/mcp
import mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "helix-bench-rag", Version: ver}, nil)
mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "rag_search", Description: "..."},
    func(ctx context.Context, req *mcpsdk.CallToolRequest, args RagSearchArgs) (*mcpsdk.CallToolResult, any, error) {
        res, err := idx.Query(ctx, args.Query, args.K, nil, nil)
        // ... marshal res into TextContent
        return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: out}}}, nil, nil
    })
// ...register the other 3 tools...
return server.Run(ctx, &mcpsdk.StdioTransport{})
```

### Pattern 2: chromem-go persistent per-corpus index
**What:** One persistent DB per `(corpus, embedder_model)` keyed by corpus SHA.
**When to use:** `bench/ragindex/index.go`.
**Example:**
```go
// Source: pkg.go.dev/github.com/philippgille/chromem-go (v0.7.0)
db, err := chromem.NewPersistentDB(filepath.Join(cacheDir, "bench-rag-index", corpusSHA), true /*gzip*/)
// EmbeddingFunc is NOT persisted — MUST be re-supplied on reopen.
ef := chromem.NewEmbeddingFuncDefault() // OpenAI text-embedding-3-small; reads OPENAI_API_KEY
// fallback: chromem.NewEmbeddingFuncOllama("nomic-embed-text", "http://localhost:11434/api")
coll, err := db.GetOrCreateCollection("corpus", map[string]string{"embedder": embedderID}, ef)
// build path (cache MISS): coll.AddDocuments(ctx, docs, runtime.NumCPU())
// query path: results, err := coll.Query(ctx, queryText, k, nil, nil)
```
chromem-go auto-loads gob-encoded docs+embeddings on reopen of the same path, so a cache HIT skips re-embedding. `Document{ID, Metadata, Embedding, Content}`; `EmbeddingFunc = func(ctx, text) ([]float32, error)`.

### Pattern 3: Static import-boundary vet check
**What:** A `go/analysis` analyzer (or a `go/packages`-based test) that fails if `cmd/helix-bench-rag` (or `bench/ragindex`) imports `internal/kernel/...` or `internal/semantic/...`.
**When to use:** Success criterion #1 vet test.
**Example:** Extend `internal/lint/ablationleakage` with a second `checkedPkgPrefix` + `forbiddenImportPrefixes` set, OR add a focused in-package test in `cmd/helix-bench-rag` using `golang.org/x/tools/go/packages` to load `Imports` and assert the forbidden prefixes are absent. The existing analyzer's exact-OR-slash-boundary match (`pkgPath == prefix || HasPrefix(pkgPath, prefix+"/")`) is the correct discipline to avoid over-flagging lookalikes.
```go
// Source: internal/lint/ablationleakage/analyzer.go:42-46 (forbiddenImportPrefixes shape)
forbidden := []string{
    "github.com/agenthands/helix/internal/kernel",
    "github.com/agenthands/helix/internal/semantic",
}
```

### Anti-Patterns to Avoid
- **Importing `internal/mcp` for convenience:** it transitively links `internal/kernel` + `internal/semantic` → the vet gate fails and success criterion #1 is violated. Use `mcpsdk` directly.
- **Re-embedding on every run:** chromem-go persists embeddings; check the cache dir and reuse. Charging re-embeds to the agent budget would also break criterion #4.
- **Charging embedding-API calls to the per-task budget:** criterion #4 explicitly excludes them. Index building happens out-of-band (before/around the agent run), and the embedder's token usage must NOT enter the result row's `tokens_input`/`tokens_output` budget accounting.
- **A new MODE.md frontmatter key:** the resolver is strict two-key (`mode`, `profile`) with `KnownFields(true)`. Keep `baseline_rag/MODE.md` change minimal; detection stays by mode name in `RunCell` (the Phase-80 pattern), not a frontmatter marker.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Vector store + similarity search | cosine-sim over a JSON blob | `chromem-go` `NewPersistentDB` + `Query` | Handles gob persistence, concurrency, normalization, metadata filters; zero deps |
| OpenAI embedding HTTP calls | hand-rolled `net/http` to `/v1/embeddings` | `chromem.NewEmbeddingFuncDefault()` | One line; reads `OPENAI_API_KEY`; retries/normalization built in |
| Ollama embedding HTTP calls | hand-rolled POST to `/api/embed` | `chromem.NewEmbeddingFuncOllama(model, baseURL)` | Matches chromem's `EmbeddingFunc` contract directly |
| MCP protocol framing | custom JSON-RPC server | `mcpsdk.NewServer` + `StdioTransport` | Same SDK the daemon ships; spec-correct |
| result.v2 schema validation | manual field checks | `bench/runtime.Validate` (reuse) | Already embedded + tested |
| Fairness model/budget pin | a new config | `runners.DefaultContract` | Single source of truth; criterion #4 demands identity with `your_agent_full` |
| Per-cell RAG-server subprocess spawn | re-roll exec/socket plumbing in cell.go | a sibling `subprocess/ragserver.go` `StartRAGServer` (mirrors `daemon.go` `StartDaemon`) | The bench sandbox already owns socket/clone/cleanup; the RAG leg reuses it like the daemon leg does |

**Key insight:** chromem-go was chosen precisely because it collapses the entire embedding-RAG stack (vector store + persistence + OpenAI + Ollama embedders) into one zero-dependency library, which respects Helix's single-binary constraint and keeps the standalone server's import set tiny — directly serving the no-leakage vet gate.

## Runtime State Inventory

> Greenfield-ish phase (new cmd + new leaf pkg + additive wiring). The one piece of pre-existing runtime state is the Phase-80 fail-close stub that this phase REPLACES.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | Per-corpus chromem index is NEW state, written under `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/` (gob files). No pre-existing store to migrate. | Code only — create cache dir on first build; key by corpus_sha so a corpus change invalidates cleanly |
| Live service config | None — no external service config embeds a renamed string. OpenAI/Ollama endpoints are read from env/defaults, not committed config. | None |
| OS-registered state | None | None — verified: no Task Scheduler / systemd / pm2 registration touches baseline_rag |
| Secrets/env vars | `OPENAI_API_KEY` (read by chromem default embedder); new `$HELIX_CACHE_DIR` convention introduced this phase (no prior usage in tree — verified by grep). Ollama base URL defaults to `http://localhost:11434/api`. | Document in EMBED-CHOICE.md + BENCH.md; resolve `$HELIX_CACHE_DIR` with a fallback (see Open Questions) |
| Build artifacts | New `cmd/helix-bench-rag` binary; new `go.sum` entries for chromem-go. | `go mod tidy` after `go get`; ensure `make build` / release matrix still builds (single-mode CGO=1) |

**Pre-existing behavior this phase changes:** `bench/runtime/cell.go:429-433` short-circuits `baseline_rag` to `Deferred=true` with reason "deferred to Phase 83". This phase MUST replace that branch with a real drive leg AND update the associated tests (`bench/runtime/cell_test.go:234-285` `TestDeferred...` assert Deferred/NO-row — these will need to flip to assert a real row, or be re-scoped). Also `bench/runtime/deltas.go:18-29,136` currently excludes baseline_rag as a non-operand; once it emits rows, decide whether it becomes a delta operand (likely yes — it is the control arm). The Phase-80 stub references in `bench/BENCH.md:77-82` and `bench/runners/baseline_rag/MODE.md` must be rewritten from "fail-closed stub" to the real arm.

## Common Pitfalls

### Pitfall 1: EmbeddingFunc is not persisted
**What goes wrong:** Reopening a `NewPersistentDB` without supplying the same `EmbeddingFunc` makes queries fail or silently re-embed with a different model.
**Why it happens:** Functions cannot be gob-serialized; chromem persists docs+embeddings but not the func.
**How to avoid:** Always pass the same embedder to `GetOrCreateCollection` on reopen. Record `embedder_id` in collection metadata AND in the result row so a mismatch is detectable.
**Warning signs:** Query results change between runs on an unchanged corpus.

### Pitfall 2: Transitive import leakage through a "helper"
**What goes wrong:** Importing any `bench/` package that itself imports `internal/kernel` or `internal/semantic` re-introduces the forbidden edge transitively, even though `cmd/helix-bench-rag` never names it directly.
**Why it happens:** `bench/runtime`, `bench/languages`, `bench/evaluators/coordinator` all import daemon-side packages (e.g. `cell.go` imports `internal/eval/runner`, `internal/eval/sandbox`).
**How to avoid:** Keep `bench/ragindex/` a LEAF package depending only on chromem-go + stdlib. The vet test must check the **transitive** import set (`go/packages` with `NeedImports|NeedDeps`), not just direct imports.
**Warning signs:** The leakage test passes on direct imports but the binary is large / links semantic symbols.

### Pitfall 3: Embedding cost charged to the agent budget
**What goes wrong:** If index-building runs inside the timed/budgeted agent leg, embedding tokens inflate `tokens_input` and the same-budget invariant (criterion #4) breaks.
**Why it happens:** Naively building the index lazily on first `rag_search` call during the agent run.
**How to avoid:** Build/load the index **before** driving the agent (out-of-band, like `prePatchSnapshot` runs before `driveScript` in `cell.go:528`). The agent's `rag_search` then only queries an already-built index. Document the exclusion in `BENCH.md`.
**Warning signs:** baseline_rag rows show higher `tokens_input` than `your_agent_full` for comparable tasks.

### Pitfall 4: Corpus SHA instability
**What goes wrong:** Non-deterministic corpus hashing (e.g. directory walk order, timestamps) produces a different `<corpus_sha>` each run → cache never hits → re-embeds every time.
**Why it happens:** `filepath.Walk` order or including mtime in the hash.
**How to avoid:** Hash the SORTED set of (relative-path, content-sha256) pairs only — never mtime. Mirror the repo's existing determinism discipline (e.g. `fairnessBlock` sorts overrides at `result.go:183` for byte-stable output).
**Warning signs:** Cache dir grows with a new `<sha>` subdir on every invocation.

### Pitfall 5: Tool count drift breaks criterion #1
**What goes wrong:** A diagnostic/ping tool sneaks in, making tool-list return 5 not 4.
**Why it happens:** Copying the daemon's `registerPingTool`/`registerEchoTool` scaffolding.
**How to avoid:** Register EXACTLY the 4 named tools; add a `main_test.go` asserting `ListTools` returns exactly `{rag_search, rag_read_chunk, grep, read_file}`. Do NOT copy the diagnostic tools from `internal/mcp/server.go`.
**Warning signs:** The tool-count test fails or `--help`/tool-list shows ping/echo.

## Code Examples

### Resolve `$HELIX_CACHE_DIR` with fallback
```go
// New convention this phase (no prior HELIX_CACHE_DIR usage in tree — verified by grep).
// Mirror the repo's ~/.helix home convention (internal/config/loader.go:33-34).
func cacheDir() string {
    if d := os.Getenv("HELIX_CACHE_DIR"); d != "" {
        return d
    }
    if d, err := os.UserCacheDir(); err == nil { // ~/.cache on Linux, ~/Library/Caches on macOS
        return filepath.Join(d, "helix")
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".helix", "cache")
}
```

### Deterministic corpus SHA
```go
// Sorted (path, content-hash) pairs only — never mtime (Pitfall 4).
func CorpusSHA(root string) (string, error) {
    var entries []string
    _ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() { return err }
        rel, _ := filepath.Rel(root, p)
        b, e := os.ReadFile(p); if e != nil { return e }
        sum := sha256.Sum256(b)
        entries = append(entries, rel+":"+hex.EncodeToString(sum[:]))
        return nil
    })
    sort.Strings(entries)
    h := sha256.Sum256([]byte(strings.Join(entries, "\n")))
    return hex.EncodeToString(h[:]), nil
}
```

### Adding embedder_id to result.v2 (additive, minor)
```go
// bench/runtime/result.go — add to ResultInput and resultDoc; schema additionalProperties is OPEN.
// ResultInput: add `EmbedderID string`
// resultDoc:   add `EmbedderID string `json:"embedder_id,omitempty"``
// BuildResult: doc.EmbedderID = in.EmbedderID
// Honest non-RAG modes leave it "" (omitempty drops the key); baseline_rag sets it.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| baseline_rag = fail-closed stub, no row | real RAG arm emitting schema-valid rows | Phase 83 (this) | Control arm becomes a real competitor in the leaderboard |
| OpenAI `text-embedding-ada-002` | `text-embedding-3-small` (cheaper, better) | OpenAI v3 embeddings (2024) | chromem `EmbeddingModelOpenAI3Small` / default; the pinned primary |
| Ollama `/api/embeddings` (legacy, single) | `/api/embed` (batch `input`) | Ollama 0.1.x → current | chromem's `NewEmbeddingFuncOllama` handles the endpoint; pass base URL ending in `/api` |

**Deprecated/outdated:**
- `text-embedding-ada-002`: superseded by `-3-small`/`-3-large`; do not pin it.
- Ollama legacy `/api/embeddings` single-input route: the newer `/api/embed` supports batching; chromem abstracts this.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | chromem-go's built-in `NewEmbeddingFuncDefault`/`NewEmbeddingFuncOllama` cover both required embedders, so `openai-go` need not be imported directly | Standard Stack | LOW — openai-go is already in go.mod as a documented fallback; switching is local |
| A2 | Ollama `nomic-embed-text` is reachable at `http://localhost:11434/api` in the offline-fallback path | Patterns | MEDIUM — offline test environments may lack Ollama; fallback must degrade gracefully (skip/flag, not crash). Confirm at human-verify |
| A3 | `$HELIX_CACHE_DIR` is a NEW env-var convention (no prior usage) with a sensible `os.UserCacheDir()` fallback | Code Examples | LOW — grep confirmed no prior usage; the success criterion names the path literally |
| A4 | Adding `embedder_id` as an open top-level provenance key is schema-valid (additive minor, no v3) | Code Examples | LOW — schema `additionalProperties` is open at top level (verified in schema JSON) |
| A5 | Embedding-index build runs out-of-band (before the agent drive leg), excluded from per-task budget | Pitfalls | MEDIUM — wiring must ensure the build is not inside the timed agent span; verify in cell wiring |
| A6 | chromem-go's `Query` is sufficient for `rag_search` (k-NN by text); no need for `dimensions` tuning | Stack | LOW — default 1536-dim text-embedding-3-small is standard; openai-go fallback gives `dimensions` if needed |

**Non-empty:** these assumptions need confirmation at the planner's `checkpoint:human-verify` (especially A2 Ollama availability and A5 budget exclusion).

## Open Questions (RESOLVED)

1. **Where does the corpus live for `corpus_sha`?** — **RESOLVED: per-repo (cloned task-repo working copy).** The index is built over the cloned task repo's source tree (hash → `corpus_sha`), per cell's repo working copy — NOT a per-language corpus spanning all tasks. This keeps the RAG arm honest (it sees the same repo the daemon arm edits) and naturally caches when the same repo recurs. Plan 03 builds the index via `ragindex.Open(repoDir)` over the cloned working copy.
   - What we knew: Go ToolBench tasks are under `bench/datasets/internal-toolbench/go/<task>/` (each a clonable repo). A "corpus" for RAG is the set of source files an agent searches. Success criterion #2 says "once per `(corpus, embedder_model)`" — the chosen reading treats one task repo as one corpus, so the `(corpus, embedder_model)` cache key is `(corpus_sha, embedder_id)`.

2. **Does `baseline_rag` become a delta operand once it emits rows?** — **RESOLVED: YES.** `baseline_rag` is added as a delta operand in `bench/runtime/deltas.go` (Plan 03, Task 3) now that it emits real rows — it is the headline control arm and the leaderboard wants `full vs baseline_rag` deltas this phase. The Phase-80 non-operand exclusion (`deltas.go:18-29,128-136,257`) is revised accordingly.
   - What we knew: `bench/runtime/deltas.go` previously excluded it as a non-operand stub; Phase 82's aggregator expectations confirm the control arm participates in deltas.

3. **How should the offline (no OPENAI_API_KEY, no Ollama) path behave?** — **RESOLVED: deterministic stub embedder, recorded as a distinct `embedder_id`.** When neither OpenAI nor Ollama is reachable (hermetic CI), a deterministic hash-based pseudo-embedding embedder is selected and recorded as `embedder_id == "stub-deterministic"` (Plan 01, `bench/ragindex/embedder.go selectEmbedder()`), so a CI row is never mistaken for a real RAG measurement. Real embedder runs (OpenAI primary → Ollama fallback) are the soak/local path; all three record a distinct `embedder_id`.
   - What we knew: CI is hermetic; embedding APIs are network calls. The fallback chain is OpenAI → Ollama → `stub-deterministic`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `github.com/philippgille/chromem-go` | index builder + server | ✓ (resolves) | v0.7.0 | none needed |
| `github.com/openai/openai-go` | optional direct embeddings | ✓ (in go.mod) | v1.12.0 | chromem built-in func |
| OpenAI API (`OPENAI_API_KEY`) | primary embedder | ✗ at research time (env not set) | — | Ollama `nomic-embed-text` |
| Ollama daemon (`localhost:11434`) | offline fallback embedder | ✗ (not probed reachable) | — | deterministic stub embedder for CI (Open Q3) |
| MCP go-sdk | 4-tool server | ✓ (in go.mod) | v1.5.0 | none |

**Missing dependencies with no fallback:** none block the build; the BUILD path is fully local. Only the LIVE embedding run needs OpenAI or Ollama, which have a documented fallback chain (OpenAI → Ollama → stub for CI).

**Missing dependencies with fallback:**
- OpenAI API key → Ollama → deterministic stub (CI). All three record a distinct `embedder_id`.

## Validation Architecture

> `workflow.nyquist_validation: true` in config → section REQUIRED.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ `github.com/stretchr/testify` already used across bench) |
| Config file | none (Go convention) |
| Quick run command | `go test ./cmd/helix-bench-rag/... ./bench/ragindex/...` |
| Full suite command | `go test ./...` then `HELIX_BIN="$(pwd)/helix" go test ./bench/runtime/...` (HELIX_BIN-gated cells) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ABLATE-04 (#1a) | `cmd/helix-bench-rag --help` works | smoke | `go test ./cmd/helix-bench-rag/ -run TestHelp` | ❌ Wave 0 |
| ABLATE-04 (#1b) | tool-list returns EXACTLY 4 tools | unit | `go test ./cmd/helix-bench-rag/ -run TestToolListIsExactlyFour` | ❌ Wave 0 |
| ABLATE-04 (#1c) | no import from `internal/kernel`/`internal/semantic` (transitive) | unit/vet | `go test ./cmd/helix-bench-rag/ -run TestNoKernelSemanticImport` (go/packages) + `make vet` | ❌ Wave 0 |
| #2 | index built once per `(corpus, embedder_model)`, cached at `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/` | unit | `go test ./bench/ragindex/ -run TestCachePathAndReuse` | ❌ Wave 0 |
| #2 | `EMBED-CHOICE.md` documents model pin + chunking | doc/test | `go test ./bench/runners/... -run TestEmbedChoiceDocExists` (or a file-presence assert) | ❌ Wave 0 |
| #3 | baseline_rag run produces schema-valid `result.v2.json` with `embedder_id` in every row | integration | `HELIX_BIN=... go test ./bench/runtime/ -run TestBaselineRagEmitsRow` | ❌ Wave 0 (replaces TestDeferred*) |
| #4 | baseline_rag uses identical `DefaultContract.ModelID` + budget as `your_agent_full`; embedding calls excluded from budget | unit | `go test ./bench/runtime/ -run TestBaselineRagSameContractBudget` | ❌ Wave 0 |
| determinism | corpus_sha stable across runs (no mtime) | unit | `go test ./bench/ragindex/ -run TestCorpusSHADeterministic` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./cmd/helix-bench-rag/... ./bench/ragindex/...` (pure, fast, no HELIX_BIN)
- **Per wave merge:** `go vet ./...` (includes the leakage analyzer) + `go test ./...`
- **Phase gate:** `HELIX_BIN="$(pwd)/helix" go test ./bench/runtime/...` green (the baseline_rag cell row test) + full suite before `/gsd-verify-work`. NOTE per MEMORY.md: `go test ./...` is FALSE-GREEN for bench cells without `HELIX_BIN` set — always re-run touched `bench/runtime` with `HELIX_BIN`.

### Wave 0 Gaps
- [ ] `cmd/helix-bench-rag/main_test.go` — `--help` works; tool-list == exactly 4 (ABLATE-04 #1a/#1b)
- [ ] `cmd/helix-bench-rag/leakage_test.go` — transitive import-set excludes kernel/semantic (#1c)
- [ ] `bench/ragindex/index_test.go` — cache path/reuse + corpus_sha determinism (#2)
- [ ] `bench/runtime/baseline_rag_test.go` — replaces/extends `cell_test.go` TestDeferred* to assert a real schema-valid row with `embedder_id` (#3)
- [ ] `bench/runtime/...` — same-contract/budget assertion + embedding-cost-exclusion (#4)
- [ ] New `cmd/vet-ablation-leakage` extension OR new analyzer wiring in `make vet` for the cmd boundary (#1c static)
- [ ] Framework install: none — Go testing + testify already present

## Security Domain

> `security_enforcement` not explicitly false in config → section included. This is a Go-native, network-light bench tool; the SMTC `java-security` capability does NOT apply (CLAUDE.md: this repo has no security capability).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | no user auth surface |
| V3 Session Management | no | stdio MCP, no sessions |
| V4 Access Control | no | local bench tool |
| V5 Input Validation | yes | Validate path args to `read_file`/`grep`/`rag_read_chunk` against path traversal — reuse the repo's `validatePathSegment`/`validateTaskID` discipline (`bench/runtime/cell.go:299`, `mode_resolver.go:54`) |
| V6 Cryptography | no | sha256 used only as a content fingerprint (non-security), `crypto/sha256` stdlib |

### Known Threat Patterns for Go MCP tool

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via `read_file`/`grep` path arg | Tampering/Info-disclosure | Clean + confine to the corpus root before any FS access (mirror `validateModeName`/`validatePathSegment`); reject `..`, absolute paths, separators |
| Secret leakage (`OPENAI_API_KEY` in logs/result rows) | Info-disclosure | Never log the key; record only the `embedder_id` model string, never the key |
| SSRF via Ollama base URL | SSRF | Pin the Ollama base URL to a config/default; do not accept it as a tool argument |

## Sources

### Primary (HIGH confidence)
- `internal/mcp/server.go`, `internal/lint/ablationleakage/analyzer.go`, `bench/runtime/cell.go`, `bench/runtime/result.go`, `bench/schema/result.v2.schema.json`, `bench/runners/fairness_contract.go`, `bench/runners/mode_resolver.go`, `bench/runners/baseline_rag/MODE.md`, `cmd/helix-bench/main.go` — read this session
- `go.mod` (verified versions), `go list -m github.com/philippgille/chromem-go@latest` → v0.7.0, `git ls-remote --tags` (tag history), module cache `openai-go@v1.12.0/embedding.go`
- `pkg.go.dev/github.com/philippgille/chromem-go` (v0.7.0) — API signatures via WebFetch
- `.planning/REQUIREMENTS.md` (ABLATE-04, FAIR-01/02/03), `.planning/STATE.md` (Phase 80/81/82 decisions)

### Secondary (MEDIUM confidence)
- chromem-go README via WebFetch (persistence + embedding-func behavior)

### Tertiary (LOW confidence)
- Ollama `/api/embed` shape — WebSearch was UNAVAILABLE this session; endpoint behavior is abstracted by chromem's `NewEmbeddingFuncOllama` so the exact wire shape is not load-bearing. Confirm Ollama reachability at the human-verify gate (A2).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified against registry/go.mod; chromem-go API confirmed via pkg.go.dev
- Architecture: HIGH — codebase patterns (MCP server, fairness contract, result builder, vet analyzer, fail-close stub) all read directly this session
- Pitfalls: HIGH — derived from the actual cell wiring, schema openness, and chromem persistence semantics
- External embedding APIs: MEDIUM — chromem abstracts them; live reachability (OpenAI/Ollama) unverified (network), documented fallback chain

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable Go libs; chromem-go v0.7.0 is the current release)
</content>
</invoke>
