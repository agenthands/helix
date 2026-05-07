# Phase 64: New MCP Tools (P0 set of 4) - Research

**Researched:** 2026-05-07
**Domain:** MCP tool surface over Phase 60-63 semantic engine; bleve FTS retrieval; mode-tier gating
**Confidence:** HIGH for integration surfaces (verified by source read), MEDIUM for bleve dependency (verified version + license, indexing throughput unmeasured), LOW for the specific recovery procedure shape (no existing analog in repo)

## Summary

Phase 64 ships four MCP skill tools (`index_semantic_graph`, `refresh_semantic_graph`, `get_semantic_graph_status`, `get_semantic_context`) registered through the existing Caddy-style `init()` pattern under `internal/skill/semantic/`, wired to the Phase 62/63 store + scheduler + compactor surfaces via constructor injection from `internal/daemon/daemon.go`. Every architectural seam these tools need already exists: `*store.Store` has `BeginSnapshot/CommitSnapshot/AbortSnapshot/CurrentGraphVersion/QueryEffective*`; `*lspenrich.LaneQueue` has `DepthAll/LastEnqueueAt`; `*graph.RankScheduler` has `IsQuiescent`; the compaction gate accessors expose `LastFlushAt`. The four tools are **wrappers**, not new engines.

Three pieces of new infrastructure must be built: (1) a `singleflight.Group` keyed by `(workspace, mode)` for `index_semantic_graph`'s concurrent-call coordination (D-02), (2) the **effective-graph queries** `QueryEffectiveAdjacency / CountStaleScoreRows / MarkAllScoreRowsStale` on `*Store` — the `graph.SchedulerStore` interface already declares them and test fakes already exist, but **no production implementation lives in `internal/semantic/store/` yet** (this is Phase 64's deferred-from-63 work), and (3) a bleve full-text index per workspace with a daemon-restart recovery procedure that compares `bleve segment metadata` against `semantic_meta.latest_snapshot_id`. **Per-tool mode-tier enforcement is also a new pattern in this codebase** — no existing tool today rejects a call based on `session.Mode`; Phase 64 introduces this discipline and must establish the error envelope shape that Phase 66's `GuardrailMiddleware` will later piggy-back on.

**Primary recommendation:** Land the four tools as a single skill package `internal/skill/semantic/` with one `init()` registration (mirrors `internal/skill/repomap/skill.go`); have the daemon construct the bleve index handle + the singleflight group + a per-workspace `IndexRunner` in a new `internal/daemon/semantic_wiring.go` (mirrors `compact_wiring.go`); add the effective-graph store methods on `*Store` in a new `internal/semantic/store/effective_graph.go` consuming the existing P60 D-04 CAS contract via `BeginOverlayTx`'s read-side counterpart.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| MCP tool registration & schema | Skill package (`internal/skill/semantic/`) | Daemon bootstrap (blank import) | Mirrors `internal/skill/repomap/`; daemon owns central registration. |
| Mode-tier check (review+/admin) | Tool handler body | Session (`*mcp.SessionInfo`) | Per CONTEXT.md "must NOT enforce in middleware" — needed inside handler so error envelope can carry actionable metadata. |
| Sync-with-timeout dispatch | Tool handler + `singleflight.Group` | TelemetryMiddleware `BudgetFunc` | Existing per-tool deadline injection caps the timeout; singleflight joins concurrent callers. |
| Snapshot writes (incremental) | `*store.Store.{Begin,Write,Commit,Abort}Snapshot` | Phase 63 compactor (idle-debounced, separate path) | Index tool calls the snapshot-write API directly; Phase 63 compactor remains the idle-trigger path. |
| Live overlay drain (refresh) | Phase 60 coalescer + handler | `*lspenrich.LaneQueue` (wait_for_lsp) | Refresh signals through existing classifier+coalescer; no new infrastructure. |
| Effective-graph queries | `*store.Store` (new methods) | Phase 60 D-04 CAS | Already declared on `graph.SchedulerStore` interface; test fakes exist; production impl missing. |
| Status aggregation | Tool handler | Live service / RankScheduler / LaneQueue / CompactionGate accessors | All accessors exist; tool handler composes JSON response. |
| Retrieval ranking (bleve+RRF) | New package `internal/semantic/retrieval/` | Phase 62 PageRank scores via `*Store` | New code; bleve handle managed per workspace; RRF is pure Go. |
| Bleve dual-store recovery | Daemon activate callback | `*store.Store` snapshot read | Compare bleve segment metadata vs `semantic_meta.latest_snapshot_id`; rebuild via goroutine. |

## Standard Stack

### Core (already in go.mod)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/modelcontextprotocol/go-sdk` | v1.5.0 | MCP server, `AddTool`, `CallToolResult` | [VERIFIED: go.mod:14] Already used by every Helix tool; new tools must use this SDK shape. |
| `golang.org/x/sync` | v0.20.0 | `singleflight.Group` for D-02 concurrent-index join | [VERIFIED: go.mod:51 + grep `internal/semantic/lspenrich/manager.go:15` already imports `golang.org/x/sync/singleflight`] Pattern precedent in codebase. |
| `github.com/duckdb/duckdb-go/v2` | v2.10502.0 | Backing store for snapshot/overlay reads | [VERIFIED: go.mod:6] Confined to `internal/semantic/store/` per `cmd/vet-noduckdb`. |

### New (must be added)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/blevesearch/bleve/v2` | v2.4.4 (latest as of Dec 2024) | Pure-Go full-text index for `get_semantic_context` task-string matching | [VERIFIED: pkg.go.dev — Apache-2.0 license, scorch backend default] [CITED: github.com/blevesearch/bleve/tree/v2.4.4]. CONTEXT.md D-05 + D-08 commit to bleve. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| bleve | DuckDB FTS5 (already in stack) | Smaller binary; SQLite FTS-style query language; would couple FTS reads to overlay tx semantics. CONTEXT.md D-08 instructs researcher to flag fallback only on hard blocker. **Verdict below in §"bleve verdict".** |
| `singleflight.Group` keyed `(ws, mode)` | Per-workspace mutex + lease counter | singleflight is the codebase idiom (see `internal/semantic/lspenrich/manager.go:87 acquireFlight singleflight.Group`); rejected alternative pre-empted. |
| Multi-projection PageRank in retrieval | Single CALL_GRAPH_PAGERANK projection | CONTEXT.md "deferred ideas" defers multi-projection blending; Phase 64 reads a single projection. |

**Installation:**
```bash
# Verify version before pinning:
go list -m -versions github.com/blevesearch/bleve/v2 | tail -5
# Pin once verified:
go get github.com/blevesearch/bleve/v2@v2.4.4
```

**Version verification status:**
- `golang.org/x/sync@v0.20.0` — already pinned, used by `lspenrich/manager.go`. [VERIFIED: go.mod]
- `bleve/v2@v2.4.4` — [CITED: github.com/blevesearch/bleve/tree/v2.4.4 Dec 17 release]. Planner MUST run `go list -m -versions github.com/blevesearch/bleve/v2` before pinning to capture any v2.4.5+ that landed since.

### bleve verdict (per CONTEXT.md D-08 researcher gate)

| Gate | Status | Notes |
|------|--------|-------|
| Version (v2.x scorch backend) | PASS | [VERIFIED: pkg.go.dev v2.4.4] Scorch is the default in v2.0.0+ ("New() to create a scorch index using zap v15"). |
| License (Apache-2.0) | PASS | [VERIFIED: pkg.go.dev/github.com/blevesearch/bleve/v2 — "Apache License, Version 2.0"]. |
| Dependency size / binary growth | UNMEASURED — flag for plan-time | [ASSUMED] bleve v2 has 40 imports per pkg.go.dev. Empirical binary growth requires `go build` before/after — Phase 64 plan MUST include a Wave-0 sizing measurement task. If growth >50MB, fall back to DuckDB FTS5 per D-08. |
| Indexing throughput on 50k-symbol fixture | UNMEASURED — flag for plan-time | [ASSUMED] No baseline exists in repo. Plan MUST include a benchmark task. If bleve >5x slower than DuckDB FTS5, fall back per D-08. |
| Crash-recovery story | NEW DESIGN — see §"Bleve recovery procedure" below | No analog in codebase; plan must specify exact mechanism. |

**Recommendation:** Proceed with bleve under D-08 conditional gate. Plan emits a Wave-0 sizing/throughput benchmark; if it busts the thresholds the planner reroutes to DuckDB FTS5 inside `*Store`.

## Architecture Patterns

### System Architecture Diagram

```
┌──────────────────┐  tools/call  ┌───────────────────────────────────────┐
│  Agent / IDE     │─────────────▶│ MCP Server (mcpsdk + Helix middleware)│
└──────────────────┘              │   LazyInit → Suggest → ProfileFilter  │
                                  │            → Telemetry → handler      │
                                  └────────────────┬──────────────────────┘
                                                   │ args: typed struct
                                                   ▼
              ┌────────────────────────────────────────────────────────────┐
              │  internal/skill/semantic/  (NEW — Phase 64)                │
              │  ├── index_semantic_graph    [review+/admin]               │
              │  ├── refresh_semantic_graph  [read+]                       │
              │  ├── get_semantic_graph_status [read+]                     │
              │  └── get_semantic_context    [read+]                       │
              │  Mode-tier check FIRST in each handler.                    │
              └─────────┬─────────┬──────────────────┬──────────┬──────────┘
                        │         │                  │          │
       ┌────────────────┘         │                  │          └─────────────┐
       │                          │                  │                        │
       ▼                          ▼                  ▼                        ▼
 ┌──────────────┐  ┌──────────────────────┐  ┌────────────────────┐   ┌──────────────────┐
 │ singleflight │  │ Live coalescer drain │  │ Status aggregator  │   │ Retrieval engine │
 │ Group        │  │ (live.Service)       │  │  - LatestSnapshot* │   │  ┌────────────┐  │
 │ key=(ws,mode)│  │  + LSPQueue.DepthAll │  │  - GraphVersion    │   │  │ bleve idx  │  │
 │              │  │  + ApplyRepair hook  │  │  - IsQuiescent     │   │  │  (per-ws)  │  │
 └──────┬───────┘  └─────────┬────────────┘  │  - LastFlushAt     │   │  └─────┬──────┘  │
        │                    │               │  - DepthAll        │   │        │         │
        ▼                    ▼               └─────────┬──────────┘   │  ┌─────▼──────┐  │
 ┌────────────────┐   ┌───────────────┐                │              │  │ PageRank   │  │
 │ index runner   │   │ overlay/lspq  │                │              │  │ (persisted)│  │
 │ → BeginSnapshot│   │ writes        │                │              │  └─────┬──────┘  │
 │ → WriteFacts   │   │ → ApplyRepair │                │              │        │         │
 │ → CommitSnapsh │   │ → graph_ver++ │                │              │  ┌─────▼──────┐  │
 │ (background    │   └───────────────┘                │              │  │ RRF fusion │  │
 │  on timeout)   │                                    │              │  └─────┬──────┘  │
 └────────┬───────┘                                    │              └────────┼─────────┘
          │                                            │                       │
          └─────────────┬──────────────────────────────┴───────────────────────┘
                        ▼
           ┌────────────────────────────────────────┐
           │ *store.Store                           │
           │  • BeginSnapshot/Write/Commit/Abort    │
           │  • CurrentGraphVersion                 │
           │  • QueryEffective{Files,Symbols,Refs,  │
           │                   Edges,Adjacency NEW} │
           │  • semantic_meta.latest_snapshot_id    │
           │    (NEW accessor: LatestCommittedSnap) │
           │  • CountStaleScoreRows NEW             │
           │  • MarkAllScoreRowsStale NEW           │
           └────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/skill/semantic/                 # NEW — four MCP tools
├── skill.go                              # init(), ToolProvider impl, dependency setters (Caddy-style)
├── tools_index.go                        # index_semantic_graph handler + args struct
├── tools_refresh.go                      # refresh_semantic_graph handler
├── tools_status.go                       # get_semantic_graph_status handler
├── tools_context.go                      # get_semantic_context handler
├── mode_check.go                         # checkMode(snap, requiredTier) → error helper
├── envelope.go                           # freshness/graph_version envelope shared by all 4 tools
├── runner.go                             # IndexRunner: singleflight + background-build coordinator
└── *_test.go                             # table-driven handler tests; mode-violation envelope tests

internal/semantic/retrieval/             # NEW — bleve + RRF retrieval
├── bleve.go                              # bleve index open/close/upsert/query
├── corpus.go                             # symbol fact → bleve document mapping (D-06 fields)
├── rrf.go                                # weighted RRF score fusion (D-07)
├── recovery.go                           # daemon-start mismatch detection + rebuild goroutine
├── config.go                             # internal RRFConfig (Go-internal constants per D-07)
└── *_test.go                             # determinism harness, RRF property tests

internal/semantic/store/effective_graph.go  # NEW — production impl of SchedulerStore methods
                                             # (QueryEffectiveAdjacency, CountStaleScoreRows,
                                             #  MarkAllScoreRowsStale, LatestCommittedSnapshot)

internal/daemon/semantic_wiring.go        # NEW — mirrors compact_wiring.go
                                             # bleve handle per workspace, IndexRunner construction,
                                             # SetActivateCallback hook for bleve recovery probe

internal/daemon/imports.go                # MODIFY — add blank import for skill/semantic
internal/profile/profiles/*.yaml          # MODIFY — add 4 tool names to all 5 profile skill blocks
                                             # OR D-14: skill subset in claude-code/codex/etc. is "semantic"
                                             # — add new "semantic" skill identifier and emit it from skill.go
```

### Pattern 1: Skill registration with Caddy-style init()

**What:** All four tools live in one skill package; daemon discovers via blank import.

**When to use:** Every Helix MCP tool. No exceptions in v1.10 scope.

**Example (mirror of `internal/skill/repomap/skill.go:62-95`):**
```go
// internal/skill/semantic/skill.go
package semantic

import (
    "github.com/agenthands/helix/internal/mcp"
    "github.com/agenthands/helix/internal/skill"
)

type SemanticSkill struct {
    store     storeAccessor   // narrow seam, *semantic/store.Store adapter
    scheduler schedulerAccessor // *graph.RankScheduler accessor
    queue     queueAccessor   // *lspenrich.LaneQueue accessor
    live      liveAccessor    // *live/service.Service accessor
    runner    *IndexRunner    // singleflight + background-build coordinator
    retrieval *retrieval.Engine // bleve + RRF
    logger    *slog.Logger
}

func init() { skill.Register(&SemanticSkill{}) }

func (s *SemanticSkill) Name() string { return "semantic" }
func (s *SemanticSkill) Description() string { return "Semantic graph indexing and retrieval" }
func (s *SemanticSkill) Init(deps skill.SkillDeps) error { /* logger only — daemon wires the rest */ }

// SetStore / SetScheduler / SetQueue / SetLive / SetRunner / SetRetrieval
// are post-init setters called by daemon (mirror SetEnrichFn pattern from
// internal/skill/repomap/skill.go:115).

func (s *SemanticSkill) Tools() []*mcp.ToolDef {
    return []*mcp.ToolDef{
        {Name: "index_semantic_graph", Description: "...", BriefDescription: "...", HelpText: indexHelp},
        {Name: "refresh_semantic_graph", Description: "...", BriefDescription: "...", HelpText: refreshHelp},
        {Name: "get_semantic_graph_status", Description: "...", BriefDescription: "...", HelpText: statusHelp},
        {Name: "get_semantic_context", Description: "...", BriefDescription: "...", HelpText: contextHelp},
    }
}
```

**Source for the pattern:** `internal/skill/repomap/skill.go` (lines 62-145, 192-254). RegisterFn is nil because the daemon-side `RegisterTools` call uses `mcpsdk.AddTool` with the typed args generic. **Open question for planner:** repomap's tools currently have nil `RegisterFn` and a separate `ExecuteTool(name, args)` dispatch — but kernel tools (e.g., `internal/kernel/symbols/tools.go:326-351`) wire `mcpsdk.AddTool` directly with typed-arg generics. **Phase 64 should follow the kernel pattern (typed args + AddTool) because the four tools have rich input schemas and `get_tool_help` extracts param docs from the schema** (see §"get_tool_help wiring" below).

### Pattern 2: Typed-arg `mcpsdk.AddTool` with kernel.WrapToolSpan

**What:** Each tool gets a `RegisterTools(server *mcp.SerenaMCPServer, deps...)` function called from daemon; uses `mcpsdk.AddTool` generic with a typed args struct so json-schema is auto-derived and `get_tool_help` works out of the box.

**When to use:** Whenever a tool has more than trivial inputs. All four Phase 64 tools qualify.

**Example (mirror of `internal/kernel/symbols/tools.go:326-351`):**
```go
// internal/skill/semantic/tools_index.go
type IndexSemanticGraphArgs struct {
    Mode          string `json:"mode,omitempty"           jsonschema:"auto, full, incremental, or refresh"`
    MaxDurationMs int    `json:"max_duration_ms,omitempty" jsonschema:"per-call timeout (default 120000)"`
    Paths         []string `json:"paths,omitempty"        jsonschema:"optional path filter"`
}

func registerIndexSemanticGraph(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "index_semantic_graph",
        Description: "Build or refresh a committed semantic snapshot.",
    }, kernel.WrapToolSpan(tracer, "index_semantic_graph",
        func(ctx context.Context, req *mcpsdk.CallToolRequest, args IndexSemanticGraphArgs) (*mcpsdk.CallToolResult, any, error) {
            // 1. Mode-tier check (NEW PATTERN)
            if err := checkMode(s.session(ctx), modeTierReview); err != nil {
                return errorResult(err.Error()), nil, nil
            }
            // 2. Resolve mode=auto
            mode := args.Mode
            if mode == "" || mode == "auto" {
                mode = s.runner.ResolveAuto(ctx, s.workspaceKey())
            }
            // 3. singleflight join
            result, err := s.runner.Run(ctx, s.workspaceKey(), mode, args.MaxDurationMs)
            if err != nil { return errorResult(err.Error()), nil, nil }
            return jsonResult(result), nil, nil
        }))
    server.Registry().Register(&mcp.ToolDef{
        Name: "index_semantic_graph",
        Description: "...",
        BriefDescription: "...",
        HelpText: indexHelp,
    })
}
```

### Pattern 3: Mode-tier enforcement inside the handler (NEW)

**What:** Check `session.Snapshot().Mode` against the tool's required mode tier; return a structured error envelope on violation. This is **net-new in this codebase** — no existing tool does it (CONTEXT.md "constraints" makes this explicit).

**Example:**
```go
// internal/skill/semantic/mode_check.go
type modeTier int
const (
    modeTierRead modeTier = iota   // any session mode
    modeTierReview                  // session must be in review or admin
    modeTierAdmin                   // session must be in admin
)

func checkMode(snap mcp.SessionSnapshot, required modeTier) error {
    cur := snap.Mode
    switch required {
    case modeTierRead:
        return nil
    case modeTierReview:
        if cur == "review" || cur == "admin" { return nil }
    case modeTierAdmin:
        if cur == "admin" { return nil }
    }
    return serr.New(serr.PermissionDenied,
        fmt.Sprintf("tool requires mode review+ or admin; current mode is %q", cur)).
        WithDetail(fmt.Sprintf("call switch_mode(target_mode=%q) to elevate", "review"))
}
```

**Source for session access:** `internal/mcp/middleware.go:340-345` shows `getSession(ctx).Snapshot()` returning `Profile`, `Mode`, `Language` consistently. The skill receives a `getSession` accessor identical to the middleware's via daemon post-init wiring.

### Pattern 4: singleflight.Group for concurrent dispatch (D-02)

**Source for the pattern:** `internal/semantic/lspenrich/manager.go:15` already imports `golang.org/x/sync/singleflight`; line 87 declares `acquireFlight singleflight.Group`. Phase 64 mirrors this idiom.

**Sketch (`internal/skill/semantic/runner.go`):**
```go
type IndexRunner struct {
    sf      singleflight.Group       // key="<repoRoot>|<mode>"
    inFlight sync.Map                  // key=repoRoot → *buildState
    store    *semantic_store.Store
    live     *live_service.Service
    timeout  time.Duration            // default per-call ceiling
}

type buildState struct {
    snapshotID  uint64
    startedAt   time.Time
    mode        string
    cancel      context.CancelFunc
    done        chan struct{}
}

func (r *IndexRunner) Run(ctx context.Context, ws workspace.WorkspaceKey, mode string, maxMs int) (Result, error) {
    key := ws.RepoRoot + "|" + mode
    // sync-with-timeout: per CONTEXT.md D-01, time-bounded inside the call
    deadline := time.Duration(maxMs) * time.Millisecond
    if deadline == 0 || deadline > r.timeout { deadline = r.timeout }
    callCtx, cancel := context.WithTimeout(ctx, deadline)
    defer cancel()

    type out struct { res Result; err error }
    ch := r.sf.DoChan(key, func() (any, error) {
        // bgCtx is a context.Background-derived context kept alive past the
        // call so a timed-out caller still observes the eventual commit via
        // get_semantic_graph_status (D-04).
        bgCtx, bgCancel := context.WithCancel(context.Background())
        // ... record bgCancel in r.inFlight for shutdown cancellation
        return r.runInternal(bgCtx, ws, mode)
    })

    select {
    case v := <-ch:
        if v.Err != nil { return Result{}, v.Err }
        return v.Val.(Result), nil
    case <-callCtx.Done():
        // Timeout: read in-flight state, return partial=true status=building
        st, _ := r.inFlight.Load(ws.RepoRoot)
        return Result{
            Partial: true,
            Status: "building",
            SnapshotID: st.(*buildState).snapshotID,
            FilesIndexed: 0, // TODO: wire progress accessor
            FilesReused: 0,
            Freshness: "stale",
            DurationMs: int(deadline.Milliseconds()),
        }, nil
    }
}
```

**Critical invariant:** `singleflight.DoChan` returns the *same* result to all waiters. The second concurrent `index_semantic_graph` call attaches and observes the same `snapshot_id` when the build commits, satisfying CONTEXT.md acceptance test #1.

### Anti-Patterns to Avoid

- **Mode check in middleware:** CONTEXT.md "constraints" forbids it explicitly. Mode enforcement lives in the handler so the error envelope can include "current mode" and "required mode" hints.
- **Calling `compactor.OnFlush` from refresh:** D-13 hard invariant — refresh is read+ and must not touch the compactor. Refresh signals through `live.Service.OnWorkspaceChanged` only.
- **New `semantic_index.*` config keys for RRF weights:** D-07 specifies Go-internal constants. Defer config exposure to Phase 67 evaluation harness.
- **Hand-rolling FTS:** CONTEXT.md D-05/D-08 commits to bleve (or DuckDB FTS5 fallback). No custom inverted-index code.
- **Duplicate goroutines for indexing:** singleflight is mandatory; second caller MUST attach not start a parallel build.
- **`mcpsdk.AddTool` without typed args:** kills the `get_tool_help` parameter introspection (which reads `tool.InputSchema` from `server.CollectToolSchemas()` per `internal/kernel/help/tools.go:50-55`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Concurrent index calls dedup | Per-workspace mutex + lease counter | `golang.org/x/sync/singleflight.Group` | Already in go.mod; codebase precedent in `lspenrich/manager.go:87`; correct semantics for "first call returns shared result". |
| Per-tool deadline injection | Context.WithTimeout in handler | TelemetryMiddleware `BudgetFunc` | Already wraps every tool call (`internal/mcp/middleware.go:307-313`). Phase 64 honors `max_duration_ms` by routing through this. |
| FTS index over symbol facts | Custom inverted index | `github.com/blevesearch/bleve/v2` | Per CONTEXT.md D-05; pure-Go, deterministic, scorch backend. |
| Score fusion for hybrid retrieval | Custom rank-blend | Weighted RRF formula (D-07) | Industry-standard; one-page implementation; defaults `K=60, w_text=1.0, w_graph=1.0`. |
| Snapshot tx mechanics | Reach into `s.db` directly | `*store.Store.{Begin,Write,Commit,Abort}Snapshot` | P63-01 closed this; `cmd/vet-noduckdb` enforces. |
| Live overlay drain | Custom event pipe | `live.Service.OnWorkspaceChanged` | Existing fire-and-forget API; classifier+coalescer already wired. |
| LSP-pending detection | Re-query DuckDB for pending rows | `LaneQueue.DepthAll() / LastEnqueueAt()` | O(1) atomic reads (`internal/semantic/lspenrich/queue.go:74-92`). |
| Workspace activation hook | Custom watcher | `mcpServer.SetActivateCallback` | Already used by repomap (`internal/skill/repomap/skill.go:100-106`), rank, compact wiring. |
| `get_tool_help` param docs | Custom doc-gen | `internal/kernel/help.ExtractParamDocs` | Reads `tool.InputSchema` automatically. Just register the tool with typed args + `HelpText` and it works. |

**Key insight:** Phase 64 is a wiring/composition phase. Almost every primitive it needs already exists; the temptation to "make it special" must be resisted. The two genuinely new pieces are bleve+RRF retrieval and per-tool mode-tier enforcement.

## Runtime State Inventory

> Phase 64 is a greenfield wiring phase. Code/config-only changes; no rename, no migration. Section omitted per template guidance.

## Common Pitfalls

### Pitfall 1: bleve segment metadata drifts from snapshot id on daemon restart
**What goes wrong:** Daemon crashes mid-`get_semantic_context`. On restart, the bleve index file is older than the latest committed `semantic_meta.latest_snapshot_id`. Retrieval returns ranked results that reference symbols deleted in the latest snapshot.
**Why it happens:** bleve and DuckDB are NOT transactionally bound (CONTEXT.md D-08 explicitly calls this out).
**How to avoid:** On workspace activation, compare bleve index meta vs `semantic_meta.latest_snapshot_id`; if mismatched, gate `get_semantic_context` on rebuild completion. Recommended response shape: return the request with `freshness=stale, retrieval_pending=true, partial=true` until rebuild finishes (see §"Bleve recovery procedure").
**Warning signs:** `get_semantic_context` returns symbol IDs that fail `QueryEffectiveSymbols`. Property-style test: pre-populate bleve, delete the snapshot, restart, assert recovery procedure fires.

### Pitfall 2: concurrent `mode=full` rebuilds clobber each other's snapshots
**What goes wrong:** Without singleflight, two `index_semantic_graph(mode=full)` calls both call `BeginSnapshot`; both succeed (different snapshot IDs); both write facts; both commit. Last commit wins; first one becomes orphaned `committed` row.
**Why it happens:** `BeginSnapshot` does NOT serialize on workspace key (it's per-tx).
**How to avoid:** singleflight key MUST be `(workspace, mode)` per D-02; second caller attaches to the in-flight build and receives the same `snapshot_id`. Acceptance test #1 must use `t.Parallel()` to fire two concurrent calls.
**Warning signs:** `SELECT COUNT(*) FROM semantic_snapshots WHERE repo_id=? AND status='committed' AND committed_at > ?` returns >1 in the test window.

### Pitfall 3: `get_semantic_context` returns non-deterministic order under map iteration
**What goes wrong:** Go map iteration is randomized; iterating over a `map[NodeID]float64` of fused scores yields different orderings each call, breaking the "byte-identical results" determinism harness (acceptance test #8).
**Why it happens:** `score → graph_version → symbol_id` tiebreak (CONTEXT.md "Claude's Discretion") must be enforced via sort, not relied on from map iteration.
**How to avoid:** Materialize results into a `[]Candidate` slice; `sort.SliceStable` by (score desc, graph_version desc, symbol_id asc). This is the Phase 62 CR-03 "sort-before-iterate" doctrine — locked in `62-CONTEXT.md`. Determinism harness asserts byte-identical JSON across 10 runs.
**Warning signs:** Identical query inputs produce different result orders across consecutive calls in a tight loop.

### Pitfall 4: refresh-with-paths leaves remaining queue items pending forever
**What goes wrong:** D-11 commits to strict-subset semantics — refresh with `paths=["a.go"]` processes only `a.go`. If subsequent calls always specify `paths`, the rest of the live queue never drains.
**Why it happens:** Refresh's classifier+coalescer flush is path-bounded by design.
**How to avoid:** Document explicitly in `helpText` for `refresh_semantic_graph`. The natural Phase 60 coalescer flush (driven by `flush_after_idle_ms`) drains the remainder; agents that need everything drained call `refresh_semantic_graph` without `paths`. Determinism: a passing test must assert that `refresh(paths=[X])` does NOT touch queue items for `Y`.

### Pitfall 5: `wait_for_lsp:true` blocks past the TelemetryMiddleware deadline
**What goes wrong:** `BudgetFunc` injects a per-tool deadline (`internal/mcp/middleware.go:307-313`); if `max_wait_ms=10000` exceeds the per-tool budget, the underlying ctx fires DeadlineExceeded mid-block. Tool returns `outcome=timeout` instead of the SPEC §23.2 envelope.
**Why it happens:** Two competing deadlines — the middleware's class-based budget and the request's `max_wait_ms`.
**How to avoid:** `max_wait_ms` MUST be capped at the lesser of `(request_value, budget_for_refresh_tool)`; planner ensures `BudgetFunc` returns ≥3000ms for refresh, OR refresh is in the "long" budget class. Plan to add a budget mapping for the four new tools.
**Warning signs:** `helix_tool_calls_total{tool="refresh_semantic_graph", outcome="timeout"}` rises with `wait_for_lsp:true` in the request.

### Pitfall 6: per-workspace mode enforcement vs `single_project=true` profiles
**What goes wrong:** `claude-code` profile has `single_project: true` and `default_mode: edit`. A user calling `index_semantic_graph` (review+) gets a mode-violation error and must `switch_mode(target_mode="review")` first. If `allowed_mode_transitions` blocks `edit→review` (it doesn't in current YAMLs, but planner must verify), the error envelope's "elevate to review" hint is misleading.
**Why it happens:** Mode hint in error doesn't introspect transition graph.
**How to avoid:** `mode_check.go` reads `profile.AllowedModeTransitions[currentMode]` to determine whether the suggested elevation is reachable; if not, the hint reads "this profile cannot reach review mode; use a different profile." Verify all 5 profile YAMLs allow the transition before committing.
**Warning signs:** Users report "switch_mode rejected" errors after seeing the elevation hint.

### Pitfall 7: `freshness=structurally_fresh_semantically_pending` requires <1ms
**What goes wrong:** CONTEXT.md acceptance test #3 requires foreground-tool budget after a single edit. If `get_semantic_context` recomputes PageRank on the hot path or reopens bleve segments, it busts this.
**Why it happens:** Confusing "build the index" with "read the index." All four tools are READ paths against pre-computed state; only `index_semantic_graph` writes.
**How to avoid:** `get_semantic_context` reads persisted PageRank scores (`semantic_pagerank_scores` table) — never recomputes. bleve segment is opened once per workspace, kept warm. Overlay-active and pending-LSP signals are O(1) atomic reads (`OverlayHasPendingRows`, `LaneQueue.DepthAll`).
**Warning signs:** `helix_tool_duration_seconds{tool="get_semantic_context"}` p50 >5ms.

## Code Examples

Verified patterns from official sources / repo:

### Skill registration (mirror)
```go
// Source: internal/skill/repomap/skill.go:62-65 + 192-198
func init() { skill.Register(&RepoMapSkill{}) }

func (s *RepoMapSkill) Tools() []*mcp.ToolDef {
    return []*mcp.ToolDef{
        s.getRepoMapTool(),
        s.getContextTool(),
    }
}
```

### Kernel-style typed-args registration with help text
```go
// Source: internal/kernel/symbols/tools.go:326-351
func registerGoToDefinition(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "go_to_definition",
        Description: "Go to the definition of a symbol at a given position",
    }, kernel.WrapToolSpan(tracer, "go_to_definition", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GoToDefinitionArgs) (*mcpsdk.CallToolResult, any, error) {
        // … handler body
    }))
    server.Registry().Register(&mcp.ToolDef{
        Name: "go_to_definition", Description: "...",
        BriefDescription: "Jump to where a symbol is defined", HelpText: goToDefinitionHelp,
    })
}
```

### singleflight idiom
```go
// Source: internal/semantic/lspenrich/manager.go:15, 87 (already in repo)
import "golang.org/x/sync/singleflight"
type Manager struct { acquireFlight singleflight.Group }
// Phase 64 IndexRunner mirrors this with key=(workspace, mode).
```

### Snapshot write API (consume from index_semantic_graph)
```go
// Source: internal/semantic/store/snapshot.go:242, 315, 408
snap, err := store.BeginSnapshot(ctx, store.SnapshotMeta{
    RepoID: repoID, BaseSnapshotID: prevID, CapturedEpoch: epoch,
})
if err != nil { return err }
defer func() { if !committed { _ = store.AbortSnapshot(ctx, snap, "build failed") } }()
if err := store.WriteSnapshotFacts(ctx, snap, facts); err != nil { return err }
if err := store.CommitSnapshot(ctx, snap, summary); err != nil { return err }
```

### Status accessors (consume from get_semantic_graph_status)
```go
// Sources:
// - graph version:    internal/semantic/store/overlay.go:983 (CurrentGraphVersion)
// - LSP pending:      internal/semantic/lspenrich/queue.go:87 (DepthAll)
// - last live update: internal/semantic/live/service/service.go:79 (LastFlushAt)
// - rank quiescent:   internal/semantic/graph/scheduler.go:413 (IsQuiescent)
// - overlay active:   internal/semantic/store/overlay.go:232 (OverlayHasPendingRows)
// - score status:     internal/semantic/graph/status.go:6-24 (ScoreStatus enum)
```

### get_tool_help auto-extraction
```go
// Source: internal/kernel/help/help.go:21-67
// Once a tool registers via mcpsdk.AddTool with a typed args struct AND
// server.Registry().Register(&mcp.ToolDef{HelpText: ...}), get_tool_help
// works automatically — ExtractParamDocs reads the InputSchema from
// server.CollectToolSchemas(). Phase 64 needs zero special wiring for TOOL-05.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hand-rolled FTS over symbol names | bleve scorch backend | bleve v2.0 (Apr 2022) | Pure-Go, BM25 default, deterministic. Phase 64 commits per CONTEXT.md D-05. |
| Per-tool middleware mode check | In-handler tier check | Phase 64 (this phase) | Better error UX (envelope carries actionable hints). Pattern propagates to Phase 66 guardrails. |
| Sync wait for full reindex | Sync-with-timeout + background continuation | Phase 64 D-01 | Caller doesn't lose progress on timeout; polls status. |

**Deprecated/outdated:**
- bleve v1 (`github.com/blevesearch/bleve` without `/v2`): superseded; v2 is the actively maintained line.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TOOL-01 | `index_semantic_graph` (review+/admin): builds/refreshes committed snapshot in auto/full/incremental/refresh; SPEC §23.1 envelope; partial-state on timeout. | §"singleflight.Group D-02"; snapshot-write API at `internal/semantic/store/snapshot.go:242-461`; sync-with-timeout via TelemetryMiddleware BudgetFunc (`internal/mcp/middleware.go:307-313`). |
| TOOL-02 | `refresh_semantic_graph` (read+): drains live changes; `wait_for_lsp` + `paths` filters; never commits a snapshot. | `live.Service.OnWorkspaceChanged` at `internal/semantic/live/service/service.go:184`; LSP wait via `LaneQueue.DepthAll/LastEnqueueAt` at `internal/semantic/lspenrich/queue.go:74-92`. |
| TOOL-03 | `get_semantic_graph_status` (read+): SPEC §23.3 envelope; latest snapshot id, graph_version, overlay state, pending LSP, freshness, score_status, cluster_status, last-live-update latency. | All accessors verified to exist (see §"Status accessors" code example). NEW: `LatestCommittedSnapshot(repoID)` accessor needed on `*Store` — currently no production accessor; tests query `semantic_snapshots` directly via `s.db` (see `snapshot_test.go:74-141`). |
| TOOL-04 | `get_semantic_context` (read+): bleve+RRF; envelope carries `freshness_mode`, `graph_version`, `overlay_active`, `freshness`, `pending_lsp_files`, per-candidate `evidence` + `confidence`. | §"bleve verdict"; RRF formula in §"Pattern 4"; determinism via sort-before-iterate (Phase 62 CR-03). |
| TOOL-05 | All four tools respect SPEC §30.2 mode gating; `tools/list` filters; `get_tool_help` returns parameter docs. | Profile YAMLs at `internal/profile/profiles/*.yaml`; `ProfileFilterMiddleware` already filters (`internal/mcp/middleware.go:429-496`); `get_tool_help` works automatically with typed-args registration (`internal/kernel/help/help.go:21-67`). |

## Bleve recovery procedure (Phase 64-specific design)

CONTEXT.md "Claude's Discretion" defers exact mechanism to planner; this section pins the recommended shape.

**On workspace activation (in `daemon.go` `SetActivateCallback`, ordered AFTER live.startWorkspace + rank.ensureScheduler + compactBndl.ensureCompactor):**
1. Read `semantic_meta.latest_snapshot_id` from `*Store` (NEW accessor `LatestCommittedSnapshot(ctx, repoID) → uint64`).
2. Read bleve `IndexMeta.last_indexed_snapshot_id` from the bleve segment dir's metadata file (`bleve/index_meta.json` — design choice; or via bleve's IndexAlias + custom kvstore field).
3. If `bleve.last_indexed_snapshot_id == store.latest_snapshot_id` → ready, no action.
4. If bleve missing OR `bleve.last_indexed_snapshot_id < store.latest_snapshot_id`:
   - Spawn rebuild goroutine attached to daemon errgroup.
   - Until rebuild completes, `get_semantic_context` returns `freshness=stale, retrieval_pending=true, partial=true` (envelope adds a top-level `retrieval_pending` field; agents poll via `get_semantic_graph_status` which surfaces a `retrieval` block).
5. If bleve `last_indexed_snapshot_id > store.latest_snapshot_id` (impossible normally, but post-rollback could occur): **fail-fast log** + treat as missing; rebuild from scratch.

**Rebuild procedure (goroutine):**
- Open bleve index in write mode.
- Iterate `semantic_symbols` table at the latest committed snapshot (`SELECT … WHERE snapshot_id = latest_id`).
- Bulk-index symbol facts using `bleve.NewBatch()` (D-06: name + docstring + path + comment window).
- On commit, persist `IndexMeta.last_indexed_snapshot_id = latest_id`.
- Notify the SemanticSkill so `retrieval_pending` flips to false.

**Property-test acceptance (planner adds Wave 0 fixture):**
1. Pre-populate bleve at snapshot N. Commit snapshot N+1 via `BeginSnapshot/Commit`. Restart daemon. Assert `get_semantic_context` returns `retrieval_pending=true` for the first few seconds, then `false` after rebuild.
2. Delete bleve dir entirely. Restart daemon. Assert rebuild completes and serves results.

## Effective-graph queries (deferred-from-63 to Phase 64)

CONTEXT.md "Phase 64 ships" item 6: `QueryEffectiveAdjacency`, `CountStaleScoreRows`, `MarkAllScoreRowsStale` on `*Store`.

**Status verified:**
- Interface declared at `internal/semantic/graph/scheduler_store.go:18-62` (lines 43, 56, 61).
- Test fakes already exist in `internal/semantic/graph/scheduler_test.go:72-89` and `internal/semantic/graph/full_recompute_test.go:74-247`.
- Production implementation **does not exist** — `internal/semantic/store/duckdb.go` has only `QueryEffectiveFiles/Symbols/References/Edges` (lines 483-511, all returning Schema-1 empty stubs).

**What Phase 64 must add (in `internal/semantic/store/effective_graph.go`):**

```go
// Method receiver pattern: on *Store, mirroring the existing QueryEffective* shape.

// QueryEffectiveAdjacency returns (out, in) adjacency maps for the (repo, projection)
// effective graph (snapshot ⊕ overlay − tombstones). Reads under no lock; consumes the
// Phase 60 D-04 CAS contract (write_epoch <= captured_epoch from caller's read tx).
//
// The implementation queries semantic_edges (snapshot side) UNION semantic_live_overlay_edges
// (overlay side, NOT tombstoned) WHERE repo_id=? AND projection=?, then pivots into the two
// adjacency maps.
func (s *Store) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
    out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) { /* implementation per scheduler_store.go contract */ }

// CountStaleScoreRows: SELECT COUNT(*) FROM semantic_pagerank_scores GROUPed by status='stale'
// or graph_version < current. LOCK-FREE per scheduler_store.go:48-56 contract.
func (s *Store) CountStaleScoreRows(ctx context.Context, repoID, projection string) (
    stale, total int, err error,
) { /* implementation */ }

// MarkAllScoreRowsStale: UPDATE semantic_pagerank_scores SET status='stale' WHERE repo_id=? AND projection=?.
func (s *Store) MarkAllScoreRowsStale(ctx context.Context, repoID, projection string) error { /* impl */ }
```

**Daemon wiring:** the existing `rankStoreAdapter` in `internal/daemon/rank_wiring.go:9-32` already declares it adapts `*store.Store` to `graph.SchedulerStore`. Once these three methods exist on `*Store`, the adapter delegates verbatim; no daemon change.

**Cross-cutting concerns:** the `cmd/vet-noduckdb` analyzer (Phase 57) requires duckdb-go imports stay in `internal/semantic/store/` — the new file is in the right place.

## Open Questions

1. **Single skill package vs four packages?**
   - What we know: `internal/skill/repomap/skill.go` ships two tools in one package; `internal/kernel/symbols/tools.go` ships nine tools in one package. The codebase is already comfortable with multi-tool packages.
   - What's unclear: CONTEXT.md "Claude's Discretion" leaves it to planner; Phase 65 strangler-fig will likely add tools that overlap conceptually with semantic.
   - Recommendation: ONE package `internal/skill/semantic/` with separate files per tool (`tools_index.go`, etc.). Phase 65 lands in the same package; if it grows past ~6 tools, split later.

2. **Where does the `IndexRunner`'s background-build progress live?**
   - What we know: D-04 timeout response carries `files_indexed`, `files_reused` "so far"; the runner must expose a progress accessor.
   - What's unclear: Atomic counters on `*buildState`? Or read from `semantic_snapshots` row counts mid-write?
   - Recommendation: atomic counters on `*buildState` (one per file processed). Concrete shape locked at plan time; researcher flags this for explicit task in plan.

3. **`status=building` field placement in response envelope?**
   - What we know: CONTEXT.md "Claude's Discretion" recommends a top-level `status` field with closed enum `committed | building | failed`.
   - What's unclear: Whether SPEC §23.1 envelope already names this field something else.
   - Recommendation: planner reads SPEC-DRAFT.md §23.1 verbatim to confirm field name; if SPEC is silent, use `status` per CONTEXT.md recommendation.

4. **Profile YAML registration mechanism: skill name vs tool list?**
   - What we know: Existing profiles use `skills:` (e.g., `repomap`, `health`) — adding the four tools should mean adding a new `semantic` skill identifier and listing it in each profile YAML's `skills:` block. D-14 says all 5 profiles get all 4 tools.
   - What's unclear: Whether a single `semantic` skill identifier or one per tool is preferred. Codebase has 1:1 (skill→multiple tools) precedent (`repomap`, `symbol-retrieval`).
   - Recommendation: ONE `semantic` skill identifier added to all 5 profile YAMLs and 4 mode YAMLs (`read`/`edit`/`review`/`admin`). Mirrors `repomap` and `symbol-retrieval`.

5. **Mode YAML enforcement vs handler enforcement double-check?**
   - What we know: Existing `read.yaml` excludes editing tools by listing them in `exclude_tools`; `ProfileFilterMiddleware` filters `tools/list` by `AllowedTools`.
   - What's unclear: If `read.yaml` should `exclude_tools: [index_semantic_graph]` (so the tool doesn't appear in `tools/list` for read mode) AND the handler should reject when called anyway. Belt-and-braces.
   - Recommendation: BOTH layers — `tools/list` doesn't show `index_semantic_graph` in read mode; if the agent calls it directly anyway (cached schema), the handler returns the structured mode-violation envelope. Acceptance test #5 covers handler-side; profile filter test covers tools/list-side.

6. **bleve recovery: block vs return-stale?**
   - What we know: CONTEXT.md "Claude's Discretion" recommends "block with `freshness=stale, retrieval_pending=true`"; says agent calls during rebuild should not crash.
   - What's unclear: Whether the rebuild blocks the request mid-call (synchronous wait) or returns immediately with the pending flag (agent retries).
   - Recommendation: Return immediately with `retrieval_pending=true`; agents poll `get_semantic_graph_status` (which exposes the rebuild progress). Aligns with the partial-state pattern of `index_semantic_graph` D-01.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Phase 64 work | ✓ | go 1.25.1 (per `go.mod`) | — |
| `golang.org/x/sync` | singleflight (D-02) | ✓ | v0.20.0 (`go.mod:51`) | — |
| `github.com/duckdb/duckdb-go/v2` | snapshot/overlay reads | ✓ | v2.10502.0 (`go.mod:6`) | — |
| `github.com/modelcontextprotocol/go-sdk` | MCP tools | ✓ | v1.5.0 (`go.mod:14`) | — |
| `github.com/blevesearch/bleve/v2` | retrieval (D-05) | ✗ | — | DuckDB FTS5 if D-08 gate fails |
| Phase 60-63 outputs in code | All four tools | ✓ | merged (per phase 62-09 + 63-02 SUMMARY) | — |
| Phase 62 `RankScheduler` quiescence | status tool | ✓ | `internal/semantic/graph/scheduler.go:413` | — |
| `compactBundle.LastFlushAt` accessor | status tool | ✓ | `internal/semantic/live/service/service.go:79` | — |
| Production `QueryEffectiveAdjacency` etc | retrieval + scheduler | ✗ | — | Phase 64 implements (deferred from 63 per CONTEXT.md item 6) |

**Missing dependencies with no fallback:**
- Production implementation of `QueryEffectiveAdjacency / CountStaleScoreRows / MarkAllScoreRowsStale` on `*Store` — Phase 64 must add. Test fakes exist; signatures locked.

**Missing dependencies with fallback:**
- bleve v2 — fallback to DuckDB FTS5 if D-08 sizing/throughput gate fails.

## Validation Architecture

> Per `.planning/config.json` `workflow.nyquist_validation: true` — included.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go test (`go test`) — Go 1.25.1 stdlib |
| Config file | none (implicit; per-package) |
| Quick run command | `go test ./internal/skill/semantic/... ./internal/semantic/store/... ./internal/semantic/retrieval/... -count=1` |
| Full suite command | `go test ./... -count=1` |
| Race-aware run (CAS / determinism) | `go test ./internal/skill/semantic/... ./internal/semantic/retrieval/... -race -count=1 -timeout 120s` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TOOL-01 | `index_semantic_graph` returns SPEC §23.1 envelope on success | unit (handler) | `go test ./internal/skill/semantic/ -run TestIndex_HappyPath_Envelope -count=1` | ❌ Wave 0 |
| TOOL-01 | Sync-with-timeout returns `partial=true, status=building` | integration | `go test ./internal/skill/semantic/ -run TestIndex_TimeoutPartial -count=1 -timeout 30s` | ❌ Wave 0 |
| TOOL-01 | Two concurrent `mode=full` calls receive same snapshot_id | integration | `go test ./internal/skill/semantic/ -run TestIndex_SingleflightJoin -race -count=1` | ❌ Wave 0 |
| TOOL-01 | `mode=auto` resolves to incremental when committed snapshot exists | unit | `go test ./internal/skill/semantic/ -run TestIndex_AutoResolution -count=1` | ❌ Wave 0 |
| TOOL-02 | `refresh_semantic_graph` never produces a new committed snapshot | invariant test | `go test ./internal/skill/semantic/ -run TestRefresh_NoSnapshotCommit -count=1` | ❌ Wave 0 |
| TOOL-02 | `paths` filter is strict subset | unit | `go test ./internal/skill/semantic/ -run TestRefresh_PathsStrictSubset -count=1` | ❌ Wave 0 |
| TOOL-02 | `wait_for_lsp:true` blocks up to `max_wait_ms` and reflects freshness | integration | `go test ./internal/skill/semantic/ -run TestRefresh_WaitForLSP -count=1 -timeout 15s` | ❌ Wave 0 |
| TOOL-03 | Empty store returns SPEC §23.3 envelope with zero-valued fields | unit | `go test ./internal/skill/semantic/ -run TestStatus_EmptyStore -count=1` | ❌ Wave 0 |
| TOOL-03 | Populated store surfaces score_status, cluster_status correctly | integration | `go test ./internal/skill/semantic/ -run TestStatus_PopulatedStore -count=1` | ❌ Wave 0 |
| TOOL-03 | `freshness` enum matches SPEC §26.2 closed values | property | `go test ./internal/skill/semantic/ -run TestStatus_FreshnessEnum -count=1` | ❌ Wave 0 |
| TOOL-04 | `get_semantic_context` returns ranked, evidence-backed candidates under token budget | integration | `go test ./internal/skill/semantic/ -run TestContext_RankedEvidence -count=1` | ❌ Wave 0 |
| TOOL-04 | bleve+RRF result order is byte-identical across 10 runs | determinism | `go test ./internal/skill/semantic/ -run TestContext_Determinism -count=10` | ❌ Wave 0 |
| TOOL-04 | `freshness=structurally_fresh_semantically_pending` after single edit, p95 <5ms | benchmark | `go test ./internal/skill/semantic/ -run TestContext_FreshnessAfterEdit -bench BenchmarkContext_PostEdit -count=1` | ❌ Wave 0 |
| TOOL-05 | `tools/list` returns 4 tools for "full" profile in any mode | profile filter | `go test ./internal/skill/semantic/ -run TestProfileFilter_FullSeesAll -count=1` | ❌ Wave 0 |
| TOOL-05 | `tools/list` filters per profile (when D-14 narrows in future) | profile filter | `go test ./internal/skill/semantic/ -run TestProfileFilter_PerProfile -count=1` | ❌ Wave 0 |
| TOOL-05 | `get_tool_help(tool_name=index_semantic_graph)` returns parameter docs | protocol | `go test ./internal/kernel/help/ -run TestGetToolHelp_SemanticTools -count=1` | ❌ Wave 0 |
| TOOL-05 | Mode-violation envelope is structured (not crash) | unit | `go test ./internal/skill/semantic/ -run TestIndex_ModeViolationEnvelope -count=1` | ❌ Wave 0 |
| Effective-graph queries | `QueryEffectiveAdjacency` returns CAS-correct adjacency | unit | `go test ./internal/semantic/store/ -run TestQueryEffectiveAdjacency -count=1` | ❌ Wave 0 |
| Effective-graph queries | `CountStaleScoreRows` is lock-free (no deadlock under contention) | property | `go test ./internal/semantic/store/ -run TestCountStaleScoreRows_LockFree -race -count=1 -timeout 60s` | ❌ Wave 0 |
| Effective-graph queries | `MarkAllScoreRowsStale` flips status idempotently | unit | `go test ./internal/semantic/store/ -run TestMarkAllScoreRowsStale -count=1` | ❌ Wave 0 |
| Bleve recovery | Restart with bleve missing rebuilds from latest snapshot | integration | `go test ./internal/semantic/retrieval/ -run TestRecovery_MissingBleve -count=1 -timeout 30s` | ❌ Wave 0 |
| Bleve recovery | Restart with bleve older than snapshot triggers rebuild | integration | `go test ./internal/semantic/retrieval/ -run TestRecovery_StaleBleve -count=1 -timeout 30s` | ❌ Wave 0 |
| Bleve recovery | `get_semantic_context` returns `retrieval_pending=true` during rebuild | integration | `go test ./internal/skill/semantic/ -run TestContext_RetrievalPending -count=1 -timeout 15s` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/skill/semantic/... ./internal/semantic/retrieval/... ./internal/semantic/store/ -count=1`
- **Per wave merge:** `go test ./... -race -count=1` (Phase 60+62+63 regression check) + `go vet ./...` + `go run ./cmd/vet-noduckdb ./...` + `go run ./cmd/vet-compact-uses-store ./...`
- **Phase gate:** Full suite green + benchmarks meet thresholds (bleve sizing, indexing throughput, post-edit p95) before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/skill/semantic/skill.go` — skill registration + ToolProvider impl
- [ ] `internal/skill/semantic/tools_*.go` — four tool handlers
- [ ] `internal/skill/semantic/runner.go` — IndexRunner with singleflight
- [ ] `internal/skill/semantic/mode_check.go` — modeTier enum + checkMode
- [ ] `internal/skill/semantic/envelope.go` — shared response shape with freshness/graph_version
- [ ] `internal/semantic/retrieval/bleve.go` + `corpus.go` + `rrf.go` + `recovery.go` — retrieval engine
- [ ] `internal/semantic/store/effective_graph.go` — production impl of `QueryEffectiveAdjacency / CountStaleScoreRows / MarkAllScoreRowsStale` + a NEW `LatestCommittedSnapshot(ctx, repoID) → uint64` accessor
- [ ] `internal/daemon/semantic_wiring.go` — bleve handle, IndexRunner wiring (mirror compact_wiring.go)
- [ ] `internal/daemon/imports.go` — add `_ "github.com/agenthands/helix/internal/skill/semantic"`
- [ ] All 5 profile YAMLs updated to include `semantic` skill — `internal/profile/profiles/{ci-bot,claude-code,codex,full,ide-assistant}.yaml`
- [ ] All 4 mode YAMLs updated for tool subset — `internal/profile/modes/{read,edit,review,admin}.yaml`
- [ ] Wave 0 fixture: 50k-symbol synthetic Go workspace with fixed seed (reusable by Phase 67)
- [ ] Bench task: bleve binary growth measurement + indexing throughput vs DuckDB FTS baseline (D-08 gate)
- [ ] `cmd/vet-semantic-mcp/` (optional per CONTEXT.md) — pin `internal/skill/semantic/` → `internal/semantic/store/` boundary

## Security Domain

Per `.planning/config.json` — security_enforcement is the implicit default (no explicit `false`); included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (transitive) | MCP transport-level only; Phase 64 introduces no new auth surface; tool calls inherit existing session-id-bound model. |
| V3 Session Management | yes | Mode-tier check inside handler reads `session.Mode` via locked snapshot (`internal/mcp/session.go:50-71`); race-free via session.Snapshot(). |
| V4 Access Control | yes | **NEW: per-tool mode-tier check.** First Helix tool to enforce; sets the precedent Phase 66 guardrails extend. |
| V5 Input Validation | yes | All four tools take typed-arg structs; jsonschema validation on `mcpsdk.AddTool` enforces shapes. `paths` filter MUST validate against path-traversal (mirror `internal/skill/repomap/skill.go:312-319` `..` rejection). |
| V6 Cryptography | no | No crypto; DuckDB store ACID handles persistence. |

### Known Threat Patterns for Helix MCP / DuckDB stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via `paths` filter (refresh) | Tampering | Reject `..`-bearing paths; reject absolute paths outside workspace root. Mirror `internal/skill/repomap/skill.go:311-319`. |
| Privilege escalation by spoofing mode | EoP | Read mode from session snapshot ONLY (never request payload); `internal/mcp/session.go:50-71` Snapshot is locked under RLock. |
| DoS via unbounded `max_duration_ms` | DoS | Cap by `BudgetFunc(toolName)` in TelemetryMiddleware; planner adds budget rows for the four tools. |
| DoS via unbounded `max_tokens` in `get_semantic_context` | DoS | Cap analogous to repomap (`internal/skill/repomap/skill.go:518-538` clamps to 32768/64). Reuse helper. |
| DoS via concurrent index calls | DoS | singleflight join (D-02) — second caller attaches; never spawns parallel. |
| SQL injection via `repoID` / `projection` | Tampering | Parameterized binds; `cmd/vet-noduckdb` enforces store-package boundary; see `snapshot.go:46-50` parameterized-binds discipline. |
| Concurrency-race on score-row writes | Tampering | Phase 62 P03 LockWorkspace contract on `SchedulerStore`; effective-graph reads run lock-free per `scheduler_store.go:48-56` contract. |
| bleve segment corruption | Tampering | Crash-recovery procedure rebuilds from snapshot (this RESEARCH §"Bleve recovery procedure"). |
| Information leak via raw error text in MCP envelope | Information Disclosure | Closed-enum reasons (mirror `internal/kernel/health/tools.go:50-81`); raw error logged via slog at warn, never in MCP response. |

## Project Constraints (from CLAUDE.md)

The following Helix-specific directives MUST be honored by Phase 64 plans:

- **Always run `go vet ./...` and `go test ./...` before completing any Go task.** Plans MUST include a final verification step running both.
- **MCP-first**: tools register via the official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`); never construct custom RPC transports.
- **Single binary**: no Python, no Docker, no runtime deps beyond what go.mod allows. Phase 64 adds bleve v2 (Apache-2.0) — within scope.
- **No JetBrains / proprietary backends.** Bleve is OSS pure-Go.
- **GSD workflow**: every Edit/Write must be via `/gsd:execute-phase` (this phase) or another GSD entry point.
- **SMTC-first tool routing**: for code-aware exploration during plan execution, prefer SMTC MCP tools (`goto_definition`, `find_references`, `list_file_outline`) over Read/Grep where applicable. Helix is Go-native — `java-security` capability does NOT apply here.
- **`cmd/vet-noduckdb`**: Phase 64 skill code MUST NOT import `duckdb-go`; routes through `internal/semantic/store/` only.
- **`cmd/vet-compact-uses-store`**: Phase 64 may not duplicate; only the compactor goes through this analyzer.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | bleve v2.4.4 is the latest v2 release as of 2026-05-07 | Standard Stack | Low — planner must `go list -m -versions` before pinning; trivially refreshed. |
| A2 | bleve binary growth fits within Helix's release-size constraints | bleve verdict | Medium — gates D-08 fallback to DuckDB FTS5; plan MUST include sizing benchmark in Wave 0. |
| A3 | bleve v2 indexing throughput on 50k-symbol fixture beats DuckDB FTS5 by enough to justify the dependency | bleve verdict | Medium — same gate; plan MUST benchmark before locking choice. |
| A4 | The mode-tier check inside the handler is acceptable to the user (vs middleware) | Pattern 3 | Low — CONTEXT.md "constraints" explicitly forbids middleware; assumption is user-locked. |
| A5 | Profile YAMLs use `skills:` not direct `tools:` listing for skill-level addition | Open Questions #4 | Low — verified by reading 5 YAMLs; pattern is consistent. |
| A6 | A new `LatestCommittedSnapshot(ctx, repoID) → uint64` accessor on `*Store` is cheap (single SELECT MAX) | TOOL-03 row | Low — DuckDB indexes `semantic_snapshots` on `(repo_id, status)`. |
| A7 | Adding 4 tools to TelemetryMiddleware's `BudgetFunc` is mechanical — just register budget rows | Pitfall 5 | Low — verified pattern in `internal/mcp/middleware.go:113-117`. |
| A8 | The `freshness=structurally_fresh_semantically_pending` requires <1ms post-edit budget | Pitfall 7 | Medium — CONTEXT.md says "foreground-tool budget"; planner must verify per-tool budget meets <1ms. |
| A9 | The retrieval engine's PageRank read goes through persisted scores (no recompute) | Pitfall 7 | Low — confirmed by `internal/semantic/graph/scheduler.go` flow; scheduler writes scores on graph_version advance, retrieval reads. |
| A10 | RRF default weights `K=60, w_text=1.0, w_graph=1.0` are sensible defaults | Pattern 4 / D-07 | Low — industry-standard RRF, classical defaults. Phase 67 evaluation harness can tune later. |

## Sources

### Primary (HIGH confidence — verified by source read)
- `internal/skill/repomap/skill.go` — skill pattern template (lines 62-65, 100-145, 192-254)
- `internal/kernel/symbols/tools.go` — typed-arg `mcpsdk.AddTool` pattern (lines 326-351)
- `internal/kernel/help/tools.go` + `help.go` — `get_tool_help` auto-extraction (lines 21-67)
- `internal/mcp/middleware.go` — `ProfileFilterMiddleware`, `TelemetryMiddleware`, `BudgetFunc` (lines 113-117, 277-371, 429-496)
- `internal/mcp/session.go` — locked Snapshot pattern for mode/profile reads (lines 50-71)
- `internal/mcp/registry.go` — `ToolRegistry`, `BriefDescriptions()` (lines 9-87)
- `internal/skill/skill.go` — `Skill`, `ToolProvider` interfaces (lines 23-45)
- `internal/profile/profile.go` + `profiles/*.yaml` + `modes/*.yaml` — profile/mode YAML format and `AllowedModeTransitions` (whole tree)
- `internal/semantic/store/snapshot.go` — `BeginSnapshot`, `WriteSnapshotFacts`, `CommitSnapshot`, `AbortSnapshot` (lines 242-512)
- `internal/semantic/store/overlay.go` — `BeginOverlayTx`, `CurrentGraphVersion`, `OverlayHasPendingRows` (lines 90-243, 983)
- `internal/semantic/store/duckdb.go` — `Available`, `DB`, `QueryEffective*` (lines 450-511)
- `internal/semantic/store/effective.go` — read API documentation (whole file)
- `internal/semantic/graph/scheduler_store.go` — `SchedulerStore` interface declaring the three deferred queries (lines 18-62)
- `internal/semantic/graph/scheduler.go` — `RankScheduler.IsQuiescent` (line 413), score row write (lines 270-310)
- `internal/semantic/graph/status.go` — `ScoreStatus` closed enum (lines 6-24)
- `internal/semantic/lspenrich/queue.go` — `LaneQueue.DepthAll`, `LastEnqueueAt` (lines 71-114)
- `internal/semantic/lspenrich/manager.go` — `singleflight.Group` precedent (lines 15, 87)
- `internal/semantic/live/service/service.go` — `Service.OnWorkspaceChanged`, `LastFlushAt`, `SetOnFlushHook` (lines 79, 184, 67)
- `internal/daemon/daemon.go` — `SetActivateCallback`, lazy bundle wiring (lines 322-410, 627-680)
- `internal/daemon/imports.go` — Caddy-style blank imports (whole file)
- `internal/daemon/rank_wiring.go` — bundle pattern template for `semantic_wiring.go` (lines 1-110)
- `internal/daemon/compact_wiring.go` — bundle pattern + accessor adapter pattern
- `internal/kernel/health/tools.go` — closed-enum reason classification (lines 50-113)
- `.planning/phases/63-compaction-retention/63-02-SUMMARY.md` — Phase 63 surface area & accessor names confirmed
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-09-SUMMARY.md` — Phase 62 final state confirmed
- `go.mod` — version pins for `golang.org/x/sync`, `duckdb-go/v2`, `modelcontextprotocol/go-sdk`
- `.planning/REQUIREMENTS.md` lines 65-73 — TOOL-01..TOOL-05 phrasing

### Secondary (MEDIUM confidence — official docs)
- [pkg.go.dev — bleve v2](https://pkg.go.dev/github.com/blevesearch/bleve/v2) — license, dependency count, scorch backend
- [github.com/blevesearch/bleve v2.4.4](https://github.com/blevesearch/bleve/tree/v2.4.4) — version, release-date confirmation

### Tertiary (LOW confidence — flagged for plan-time validation)
- bleve indexing throughput on 50k-symbol fixture: NO benchmark exists in the repo or in available web sources; D-08 explicitly gates a Wave-0 measurement.
- bleve binary growth: NO measurement available; D-08 gates a `go build` size diff.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH (`golang.org/x/sync`, `duckdb-go`, `modelcontextprotocol/go-sdk` all verified in `go.mod`); MEDIUM for bleve (license + version verified, runtime characteristics unmeasured).
- Architecture: HIGH (every named file/symbol verified by source read; pattern templates pinned to specific line numbers).
- Pitfalls: HIGH (1-3 derived from in-source comments calling out the same risks; 4-7 derived from CONTEXT.md decisions).
- Effective-graph queries: HIGH (interface in source; test fakes in source; production impl confirmed missing — actual gap-closing work for Phase 64).
- Bleve recovery procedure: MEDIUM (no analog in repo; design here is original; planner must validate at plan time).

**Research date:** 2026-05-07
**Valid until:** 2026-06-07 (30 days — stable Go stdlib + DuckDB-backed work; bleve version may move).

Sources:
- [bleve v2 package — pkg.go.dev](https://pkg.go.dev/github.com/blevesearch/bleve/v2)
- [bleve v2.4.4 release](https://github.com/blevesearch/bleve/tree/v2.4.4)
- [bleve scorch backend docs](https://pkg.go.dev/github.com/blevesearch/bleve/index/scorch)
- [bleve project home](https://blevesearch.com/)
