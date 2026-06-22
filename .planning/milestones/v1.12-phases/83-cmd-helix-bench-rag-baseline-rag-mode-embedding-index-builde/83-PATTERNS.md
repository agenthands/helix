# Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 14 (8 create, 6 modify)
**Analogs found:** 13 / 14

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/helix-bench-rag/main.go` (new) | cmd-entrypoint | request-response | `cmd/helix-bench/main.go` | role-match (cobra root) |
| `cmd/helix-bench-rag/server.go` (new) | mcp-server | request-response | `internal/mcp/server.go` (used WITHOUT importing) | exact (SDK shape) |
| `cmd/helix-bench-rag/tools.go` (new) | tool-handlers | request-response | `internal/mcp/server.go` registerPingTool (AddTool shape) | role-match |
| `cmd/helix-bench-rag/main_test.go` (new) | test | — | `bench/runtime/cell_test.go` (testify shape) | role-match |
| `cmd/helix-bench-rag/leakage_test.go` (new) | test/vet | transform | `internal/lint/ablationleakage/analyzer.go` | exact (import-boundary) |
| `bench/ragindex/index.go` (new) | service (leaf) | CRUD + file-I/O | chromem-go API (RESEARCH Pattern 2) | role-match (no in-repo analog) |
| `bench/ragindex/chunk.go` (new) | utility | transform | — | NO ANALOG |
| `bench/ragindex/embedder.go` (new) | service | request-response | chromem `EmbeddingFunc` (RESEARCH) | partial |
| `bench/ragindex/cache.go` (new) | utility | file-I/O | `internal/config/loader.go:33-34` (home convention) | partial |
| `bench/ragindex/index_test.go` (new) | test | — | `bench/runtime/cell_test.go` | role-match |
| `bench/runners/baseline_rag_agent/EMBED-CHOICE.md` (new) | doc | — | `bench/runners/your_agent_full/MODE.md` (doc shape) | role-match |
| `bench/runtime/cell.go` (modify) | orchestrator | request-response | self (lines 429-433 + drive leg 511-538) | exact (in-place) |
| `bench/runtime/result.go` (modify) | model/builder | transform | self (Outcome/TraceRef/ModelID open keys) | exact (additive mirror) |
| `bench/runtime/cell_test.go` (modify) + new `baseline_rag_test.go` | test | — | self TestDeferred* (lines 234-285) | exact |
| `bench/runners/baseline_rag/MODE.md` (modify) | doc | — | self (stub → real arm rewrite) | exact |
| `cmd/vet-ablation-leakage` ext OR new cmd (modify/new) | vet-wiring | transform | `cmd/vet-ablation-leakage/main.go` + Makefile:46 | exact |

## Pattern Assignments

### `cmd/helix-bench-rag/server.go` (mcp-server, request-response)

**Analog:** `internal/mcp/server.go` — pattern is REUSED via the SDK directly. CRITICAL: do NOT `import "github.com/agenthands/helix/internal/mcp"` (it transitively links kernel + semantic and fails the vet gate). Import only `mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"`.

**Server construction** (mirror `server.go:81-84`, swap Name/version, no middleware, no kernel):
```go
server := mcpsdk.NewServer(
    &mcpsdk.Implementation{Name: "helix-bench-rag", Version: ver},
    nil,
)
```

**Tool registration** (mirror the `registerPingTool` shape at `server.go:109-110` but use the typed-args generic `mcpsdk.AddTool`, RESEARCH Pattern 1):
```go
mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "rag_search", Description: "..."},
    func(ctx context.Context, req *mcpsdk.CallToolRequest, args RagSearchArgs) (*mcpsdk.CallToolResult, any, error) {
        res, err := idx.Query(ctx, args.Query, args.K, nil, nil)
        ...
        return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: out}}}, nil, nil
    })
```

**Run** (stdio transport): `return server.Run(ctx, &mcpsdk.StdioTransport{})`

**CRITICAL — Pitfall 5:** Register EXACTLY the 4 named tools. Do NOT copy `registerPingTool`/`registerEchoTool`/`registerActivateProjectTool` from `server.go:101-103` — those would push tool-list to >4 and break success criterion #1b.

---

### `cmd/helix-bench-rag/main.go` (cmd-entrypoint, request-response)

**Analog:** `cmd/helix-bench/main.go:1-66`

**Pattern:** cobra root with single `os.Exit` site; subcommands use `RunE` so errors propagate to one exit point.
```go
func main() {
    if err := newRootCmd().Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```
For `--help` (criterion #1a) cobra provides it automatically once a root `cobra.Command` with `Use`/`Short` exists. The server-run is the root's `RunE` (or a `serve` subcommand).

---

### `cmd/helix-bench-rag/leakage_test.go` (test/vet, transform)

**Analog:** `internal/lint/ablationleakage/analyzer.go:38-46,165-178` (the import-boundary discipline) AND `cmd/vet-ablation-leakage/main.go` (the singlechecker wrapper).

**Forbidden-prefix set** (mirror `analyzer.go:42-45` shape, new prefixes per RESEARCH Pattern 3):
```go
forbidden := []string{
    "github.com/agenthands/helix/internal/kernel",
    "github.com/agenthands/helix/internal/semantic",
}
```

**Match discipline — copy verbatim from `analyzer.go:170`** (exact-OR-slash-boundary; a bare `HasPrefix` over-flags lookalikes like `internal/semantic/storehouse`):
```go
if path == forbidden || strings.HasPrefix(path, forbidden+"/") { ... }
```

**Transitive requirement (Pitfall 2):** an in-package test must load the TRANSITIVE import set via `golang.org/x/tools/go/packages` with `NeedImports|NeedDeps` (a `bench/` helper could re-introduce the edge transitively). Iterate `pkg.Imports` recursively, not just direct `file.Imports`.

**Static vet gate option:** extend the existing analyzer with a second `checkedPkgPrefix` + forbidden set, OR add a new `cmd/vet-bench-rag-leakage` singlechecker. The singlechecker wrapper is trivially copied from `cmd/vet-ablation-leakage/main.go`:
```go
func main() { singlechecker.Main(<analyzer>.Analyzer) }
```
Then add a `$(GO) vet -vettool=$(VETTOOL_...) ./...` line in the Makefile `vet:` target alongside `Makefile:46`.

---

### `bench/runtime/result.go` (model/builder, transform) — ADD `embedder_id`

**Analog:** SELF — the `Outcome`/`TraceRef`/`ModelID` open-provenance keys are the exact mirror to follow. Schema top-level `additionalProperties` is OPEN (confirmed `result.v2.schema.json:6` versioning comment), so this is additive-minor, NO v3 bump.

Three edits, each mirroring the existing open keys:
1. `ResultInput` (after line 55, alongside `Outcome string`/`TraceRef string`): add `EmbedderID string`
2. `resultDoc` (after line 115, alongside `ModelID string \`json:"model_id"\``): add
   ```go
   EmbedderID string `json:"embedder_id,omitempty"`
   ```
   Use `omitempty` so honest non-RAG modes (which leave it `""`) drop the key; only `baseline_rag` sets it.
3. `BuildResult` (in the `doc := resultDoc{...}` literal at lines 154-169, alongside `ModelID: in.Fairness.ModelID`): add `EmbedderID: in.EmbedderID,`

**Determinism note (Pitfall 4):** mirror `fairnessBlock`'s sort discipline (result.go:178+) — any map-derived output must be sorted for byte-stable sha256 reproducibility.

---

### `bench/runtime/cell.go` (orchestrator) — REPLACE fail-close at lines 429-433

**Analog:** SELF.

**Current short-circuit (cell.go:429-433)** detects `baseline_rag` BY MODE NAME right after the unconditional fairness gate (cell.go:417-419) and BEFORE `benchsandbox.New`:
```go
if cfg.Mode == "baseline_rag" {
    res.Deferred = true
    res.DeferredReason = "baseline_rag: real RAG arm deferred to Phase 83 (ABLATE-04)"
    return res, nil
}
```

**Replace with a real RAG drive leg.** Keep detection BY MODE NAME (anti-pattern: do NOT add a MODE.md frontmatter key — the resolver is strict two-key with `KnownFields(true)`).

**Budget-exclusion (Pitfall 3 / criterion #4):** build/load the index OUT-OF-BAND, mirroring how `prePatchSnapshot` runs BEFORE `driveScript` at `cell.go:528`. The embedding build must NOT be inside the timed agent span (the `start := time.Now()` anchor at cell.go:491). The agent's `rag_search` then only queries an already-built index. Embedder tokens must NOT enter `tokens_input`/`tokens_output`.

**Fairness identity (criterion #4):** the `runners.DefaultContract.Validate()` gate at cell.go:417-419 already runs unconditionally; baseline_rag reuses the same `DefaultContract` verbatim (same `ModelID` snapshot + budget as `your_agent_full`). The drive leg spawns `cmd/helix-bench-rag` as the MCP server INSTEAD of the Helix daemon (`subprocess.StartDaemon` at cell.go:501), reusing the sandbox/clone/socket scaffolding (cell.go:435-518).

---

### `bench/runtime/cell_test.go` + new `bench/runtime/baseline_rag_test.go` (test)

**Analog:** SELF — `TestDeferred...` at cell_test.go:234-285.

The existing tests assert `res.Deferred == true`, `DeferredReason` contains "Phase 83" (line 255), and NO `result.v2.json` on disk (line 261). These MUST FLIP: the real arm now emits a schema-valid row with a non-empty `embedder_id`. Re-scope `TestDeferred*` (or replace with `TestBaselineRagEmitsRow` / `TestBaselineRagSameContractBudget`). Reuse the same `Cell{Benchmark, Mode: "baseline_rag", Task}` construction (cell_test.go:245,270) and the `require`/`assert` testify shape. These cell tests are HELIX_BIN-gated — per MEMORY.md `go test ./...` is FALSE-GREEN without `HELIX_BIN` set.

---

### `bench/ragindex/index.go` (service leaf, CRUD + file-I/O)

**Analog:** No in-repo vector-store analog (NO ANALOG — use RESEARCH Pattern 2). MUST be a LEAF package: import only `chromem-go` + stdlib (Pitfall 2).
```go
db, err := chromem.NewPersistentDB(filepath.Join(cacheDir, "bench-rag-index", corpusSHA), true)
ef := chromem.NewEmbeddingFuncDefault() // OpenAI text-embedding-3-small; reads OPENAI_API_KEY
coll, err := db.GetOrCreateCollection("corpus", map[string]string{"embedder": embedderID}, ef)
// cache MISS: coll.AddDocuments(ctx, docs, runtime.NumCPU())
// query:      results, err := coll.Query(ctx, queryText, k, nil, nil)
```
**Pitfall 1:** `EmbeddingFunc` is NOT persisted — always re-supply the SAME embedder on reopen; record `embedder_id` in collection metadata AND the result row.

---

### `bench/ragindex/cache.go` (utility, file-I/O)

**Analog:** `internal/config/loader.go:33-34` (the `~/.helix` home convention). `$HELIX_CACHE_DIR` is a NEW env-var (no prior usage). Use the RESEARCH `cacheDir()` fallback: `HELIX_CACHE_DIR` → `os.UserCacheDir()/helix` → `~/.helix/cache`.

**`CorpusSHA` determinism (Pitfall 4):** hash SORTED `(rel-path, content-sha256)` pairs only — never mtime, never walk order. Mirror `result.go:183` sort-for-byte-stability discipline.

---

### `bench/runners/baseline_rag/MODE.md` (modify) + `bench/runners/baseline_rag_agent/EMBED-CHOICE.md` (new)

**Analog:** `bench/runners/your_agent_full/MODE.md` (frontmatter + body doc shape). Keep `baseline_rag/MODE.md` two-key frontmatter (`mode`, `profile`) UNCHANGED structurally; rewrite the body from "fail-closed stub / deferred to Phase 83" to the real arm. `EMBED-CHOICE.md` documents: OpenAI `text-embedding-3-small` primary, Ollama `nomic-embed-text` fallback, chunking strategy (criterion #2).

## Shared Patterns

### Import-boundary discipline
**Source:** `internal/lint/ablationleakage/analyzer.go:170`
**Apply to:** `cmd/helix-bench-rag/leakage_test.go`, any new vet cmd
```go
if path == forbidden || strings.HasPrefix(path, forbidden+"/") { ... }
```
Exact-OR-slash-boundary — never bare `HasPrefix`.

### Open provenance key (additive-minor)
**Source:** `bench/runtime/result.go:53-55,113-115,163-165` (Outcome/TraceRef/ModelID)
**Apply to:** the new `embedder_id` key — `omitempty` on the doc field, threaded through `ResultInput` → `resultDoc` → `BuildResult`.

### Fairness contract reuse
**Source:** `bench/runners/fairness_contract.go:84-85` (`DefaultContract`, `ModelID: "claude-sonnet-4-5-20250929"`), gated at `bench/runtime/cell.go:417-419`
**Apply to:** baseline_rag drive leg — reuse `DefaultContract` verbatim; never a separate config (criterion #4).

### Singlechecker vet wrapper + Makefile wiring
**Source:** `cmd/vet-ablation-leakage/main.go` + `Makefile:24,46`
**Apply to:** the no-kernel/semantic static gate if implemented as a vet tool.

### Path-traversal validation (V5)
**Source:** `bench/runtime/cell.go:299`, `bench/runners/mode_resolver.go:54` (`validatePathSegment`/`validateModeName` discipline)
**Apply to:** `read_file`/`grep`/`rag_read_chunk` path args — clean + confine to corpus root; reject `..`, absolute paths, separators.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `bench/ragindex/chunk.go` | utility | transform | No chunking strategy exists in the tree; greenfield, follow EMBED-CHOICE.md doc |
| `bench/ragindex/index.go` (partial) | service | CRUD | No prior vector-store; chromem-go is a new dep — use RESEARCH Pattern 2, not an in-repo analog |

## Metadata

**Analog search scope:** `cmd/`, `internal/mcp/`, `internal/lint/ablationleakage/`, `bench/runtime/`, `bench/runners/`, `bench/schema/`, `Makefile`
**Files scanned:** 11
**Pattern extraction date:** 2026-06-21
