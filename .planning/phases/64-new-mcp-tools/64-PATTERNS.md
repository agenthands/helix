# Phase 64: New MCP Tools (P0 set of 4) - Pattern Map

**Mapped:** 2026-05-07
**Files analyzed:** 15 new + 9 modified = 24
**Analogs found:** 24 / 24 (every new file has a concrete in-tree analog)

This map is consumed by the planner. For each new/modified file the planner
must reference (a) the analog source path + line numbers and (b) the
specific excerpt block under "Pattern Assignments". Cross-cutting auth /
shape excerpts live in **Shared Patterns** at the bottom.

## File Classification

| New/Modified File                                           | Role               | Data Flow         | Closest Analog                                            | Match Quality |
|-------------------------------------------------------------|--------------------|-------------------|-----------------------------------------------------------|---------------|
| `internal/skill/semantic/skill.go`                          | skill (provider)   | request-response  | `internal/skill/repomap/skill.go`                         | exact         |
| `internal/skill/semantic/tools_index.go`                    | tool handler       | event-driven (sf) | `internal/kernel/symbols/tools.go:326-351` + `internal/semantic/lspenrich/manager.go:15,87` | exact (combined) |
| `internal/skill/semantic/tools_refresh.go`                  | tool handler       | request-response  | `internal/kernel/symbols/tools.go:326-351`                | exact         |
| `internal/skill/semantic/tools_status.go`                   | tool handler       | request-response  | `internal/kernel/symbols/tools.go:326-351`                | exact         |
| `internal/skill/semantic/tools_context.go`                  | tool handler       | request-response  | `internal/skill/repomap/skill.go:289-351`                 | role-match    |
| `internal/skill/semantic/mode_check.go`                     | utility (auth)     | -                 | `internal/mcp/middleware.go:339-346` (read pattern) + `internal/kernel/health/tools.go:50-113` (closed-enum) | role-match (NEW pattern) |
| `internal/skill/semantic/envelope.go`                       | utility (shape)    | -                 | `internal/kernel/health/tools.go` closed-enum + SPEC §23.1-23.4 | partial (SPEC-driven) |
| `internal/skill/semantic/runner.go`                         | service            | event-driven      | `internal/semantic/lspenrich/manager.go:15,87,82-88`      | role-match    |
| `internal/skill/semantic/skill_test.go`                     | test               | -                 | `internal/skill/repomap/*_test.go`                        | role-match    |
| `internal/skill/semantic/tools_*_test.go`                   | test (table)       | -                 | `internal/kernel/symbols/*_test.go`                       | role-match    |
| `internal/skill/semantic/runner_singleflight_test.go`       | test (concurrency) | -                 | `internal/semantic/lspenrich/manager_test.go` (singleflight assertions) | role-match |
| `internal/semantic/retrieval/bleve.go`                      | service (FTS)      | CRUD (index/query)| (no in-tree analog — bleve is new) | NO ANALOG (use bleve docs) |
| `internal/semantic/retrieval/corpus.go`                     | transform          | batch             | `internal/repomap/cache.go` (extractor → row mapping)     | partial       |
| `internal/semantic/retrieval/rrf.go`                        | utility (algo)     | transform         | `internal/repomap/pagerank.go` (pure-Go ranking primitive)| partial       |
| `internal/semantic/retrieval/recovery.go`                   | service (lifecycle)| event-driven      | `internal/skill/repomap/skill.go:362-471` (lazy walk + cache populate) | role-match |
| `internal/semantic/retrieval/*_test.go`                     | test (determinism) | -                 | Phase 62 sort-before-iterate determinism harness pattern  | role-match    |
| `internal/semantic/store/effective_graph.go`                | model (store ext)  | CRUD              | `internal/semantic/store/duckdb.go:483-511` (existing `QueryEffective*` stubs) + `internal/semantic/store/snapshot.go:242-512` (real query + tx pattern) | exact (extension) |
| `internal/semantic/store/effective_graph_test.go`           | test               | -                 | `internal/semantic/store/snapshot_test.go`                | role-match    |
| `internal/daemon/semantic_wiring.go`                        | config (bundle)    | request-response  | `internal/daemon/compact_wiring.go` + `internal/daemon/rank_wiring.go` | exact         |
| `internal/daemon/imports.go` (modify)                       | config             | -                 | self (existing line 11)                                   | exact         |
| `internal/daemon/daemon.go` (modify, post-init wiring)      | config             | -                 | Phase 62/63 wiring blocks (look for `compactBndl.ensureCompactor`, `SetActivateCallback`) | exact |
| `internal/profile/profiles/{full,claude-code,codex,ide-assistant,ci-bot}.yaml` (modify) | config | -    | `internal/profile/profiles/full.yaml:11-21` (`skills:` block) | exact |
| `internal/profile/modes/{read,review,admin,edit}.yaml` (modify) | config         | -                 | `internal/profile/modes/review.yaml:6-9` (`skills:` block) | exact         |
| `cmd/vet-semantic-mcp/main.go` (optional)                   | tool (lint)        | static            | `cmd/vet-noduckdb/`, `cmd/vet-compact-uses-store/`        | role-match    |

---

## Pattern Assignments

### `internal/skill/semantic/skill.go` (skill provider, request-response)

**Analog:** `internal/skill/repomap/skill.go`

**Imports + struct + `init()` + Caddy registration** (lines 1-21, 32-65):
```go
package semantic

import (
    "log/slog"
    "sync"

    serr "github.com/agenthands/helix/internal/errors"
    "github.com/agenthands/helix/internal/mcp"
    "github.com/agenthands/helix/internal/skill"
)

type SemanticSkill struct {
    // narrow accessors set by daemon post-init (see SetXxx setters below)
    store     storeAccessor       // adapts *internal/semantic/store.Store
    scheduler schedulerAccessor   // adapts *internal/semantic/graph.RankScheduler
    queue     queueAccessor       // adapts *internal/semantic/lspenrich.LaneQueue
    live      liveAccessor        // adapts *internal/semantic/live/service.Service
    runner    *IndexRunner        // singleflight+background-build coordinator
    retrieval *retrieval.Engine   // bleve+RRF (NEW package)

    mu     sync.Mutex
    logger *slog.Logger
}

func init() { skill.Register(&SemanticSkill{}) }

func (s *SemanticSkill) Name() string        { return "semantic" }
func (s *SemanticSkill) Description() string { return "Semantic graph indexing and retrieval" }
func (s *SemanticSkill) Init(deps skill.SkillDeps) error {
    s.logger = deps.Logger
    if s.logger == nil { s.logger = slog.Default() }
    return nil
}
```

**Post-init setter pattern** (mirror `repomap.SetEnrichFn` lines 113-119):
```go
// SetStore is called by daemon.New after the *Store is constructed.
// Mirrors RepoMapSkill.SetEnrichFn — post-init dependency injection.
func (s *SemanticSkill) SetStore(a storeAccessor) {
    s.mu.Lock(); defer s.mu.Unlock(); s.store = a
}
// SetScheduler / SetQueue / SetLive / SetRunner / SetRetrieval — same shape.
```

**`Tools()` returning `[]*mcp.ToolDef`** (mirror lines 192-198):
```go
func (s *SemanticSkill) Tools() []*mcp.ToolDef {
    return []*mcp.ToolDef{
        {Name: "index_semantic_graph",      Description: "...", BriefDescription: "...", HelpText: indexHelp},
        {Name: "refresh_semantic_graph",    Description: "...", BriefDescription: "...", HelpText: refreshHelp},
        {Name: "get_semantic_graph_status", Description: "...", BriefDescription: "...", HelpText: statusHelp},
        {Name: "get_semantic_context",      Description: "...", BriefDescription: "...", HelpText: contextHelp},
    }
}

// Public accessor for daemon post-init wiring.
func GetSemanticSkill() *SemanticSkill {
    s, ok := skill.Get("semantic")
    if !ok { return nil }
    return s.(*SemanticSkill)
}
```

**Pattern variation vs analog:** Phase 64 follows the **kernel** registration
shape for the actual `mcpsdk.AddTool` call (typed args + generic — see
tools_index.go below) instead of the repomap nil-RegisterFn shape, because
`get_tool_help` extracts param docs from the typed-args jsonschema (per
`internal/kernel/help/help.go:21-67`). The `ToolDef` returned from `Tools()`
exists primarily for `BriefDescription` + `HelpText` registration; the
actual MCP `AddTool` call lives in a `RegisterTools(server, deps...)`
function called from `internal/daemon/daemon.go`.

---

### `internal/skill/semantic/tools_index.go` (tool handler, event-driven dispatch)

**Analog:** `internal/kernel/symbols/tools.go:326-351` (kernel typed-args
registration) **combined with** `internal/semantic/lspenrich/manager.go:15,
82-88` (singleflight idiom).

**Typed args + `mcpsdk.AddTool` registration** (mirror kernel/symbols
tools.go lines 326-351):
```go
type IndexSemanticGraphArgs struct {
    Mode          string   `json:"mode,omitempty"            jsonschema:"auto, full, incremental, or refresh"`
    MaxDurationMs int      `json:"max_duration_ms,omitempty" jsonschema:"per-call timeout in ms (default 120000)"`
    Paths         []string `json:"paths,omitempty"           jsonschema:"optional path filter (relative to workspace root)"`
}

func registerIndexSemanticGraph(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "index_semantic_graph",
        Description: "Build or refresh a committed semantic snapshot.",
    }, kernel.WrapToolSpan(tracer, "index_semantic_graph",
        func(ctx context.Context, req *mcpsdk.CallToolRequest, args IndexSemanticGraphArgs) (*mcpsdk.CallToolResult, any, error) {
            // 1. Mode-tier check FIRST (NEW PATTERN — see mode_check.go)
            if err := checkMode(s.session(ctx), modeTierReview); err != nil {
                return errorResult(err.Error()), nil, nil
            }
            // 2. Path-traversal validation (mirror repomap pattern in shared)
            if err := validatePaths(args.Paths, s.workspaceRoot()); err != nil {
                return errorResult(err.Error()), nil, nil
            }
            // 3. Resolve mode=auto via D-03 rule
            mode := args.Mode
            if mode == "" || mode == "auto" {
                mode = s.runner.ResolveAuto(ctx, s.workspaceKey())
            }
            // 4. singleflight join — see runner.go
            res, err := s.runner.Run(ctx, s.workspaceKey(), mode, args.MaxDurationMs)
            if err != nil { return errorResult(err.Error()), nil, nil }
            return jsonResult(res), nil, nil
        }))
    server.Registry().Register(&mcp.ToolDef{
        Name: "index_semantic_graph",
        Description: "...", BriefDescription: "...", HelpText: indexHelp,
    })
}
```

**Path-traversal sub-pattern** copied from `internal/skill/repomap/skill.go:
311-319` (see Shared Patterns).

---

### `internal/skill/semantic/tools_refresh.go` (tool handler, request-response)

**Analog:** `internal/kernel/symbols/tools.go:326-351`

**Same typed-args + AddTool shape** as tools_index.go above. Body:
```go
type RefreshSemanticGraphArgs struct {
    Paths       []string `json:"paths,omitempty"`
    WaitForLSP  bool     `json:"wait_for_lsp,omitempty"`
    MaxWaitMs   int      `json:"max_wait_ms,omitempty"  jsonschema:"default 3000"`
}

// Handler body:
//   1. checkMode(modeTierRead) — every session passes
//   2. validatePaths(args.Paths, root)
//   3. Drain via live.Service.OnWorkspaceChanged (see service.go:184 signature)
//   4. If args.WaitForLSP { block on s.queue.DepthAll() <= prev_depth || max_wait }
//   5. Read graph_version via s.store.CurrentGraphVersion (overlay.go:983)
//   6. Build envelope.go shape: freshness, graph_version, files_updated, deltas, pending_lsp
//   7. INVARIANT: never call BeginSnapshot, never call compactor.OnFlush (D-13)
```

**Critical invariant** (D-09 / D-13): refresh must NOT call
`BeginSnapshot` / `WriteSnapshotFacts` / `CommitSnapshot` /
`compactor.OnFlush()` / `IndexRunner.Run()`. The compile-time check is the
fact that `tools_refresh.go` only imports the read-side surface of `*Store`
(`CurrentGraphVersion`, `OverlayHasPendingRows`, `QueryEffective*`) and
`live.Service.OnWorkspaceChanged`. Plan must list this as an explicit grep
test (`! grep -E "BeginSnapshot|OnFlush" internal/skill/semantic/tools_refresh.go`).

---

### `internal/skill/semantic/tools_status.go` (tool handler, request-response)

**Analog:** `internal/kernel/symbols/tools.go:326-351` for the registration shape;
the **body is a fan-out of read-only accessors** (no analog needs copying — it
literally calls existing functions).

**Accessor map (already verified, no new methods needed):**
```go
// Per RESEARCH.md §"Status accessors" — every accessor exists today:
//   graph_version       → s.store.CurrentGraphVersion(ctx, repoID)        // overlay.go:983
//   overlay_active      → s.store.OverlayHasPendingRows(repoID)            // overlay.go:232
//   pending_lsp_files   → s.queue.DepthAll()                                // queue.go:87
//   last_live_update    → s.live.LastFlushAt(ws)                            // service.go:79
//   rank_quiescent      → s.scheduler.IsQuiescent(repoID)                   // scheduler.go:413
//   score_status enum   → graph.ScoreStatus closed enum                     // status.go:6-24
//   latest_snapshot_id  → s.store.LatestCommittedSnapshot(ctx, repoID)      // *** NEW (RESEARCH.md flags it; lands in effective_graph.go) ***
```

The **only NEW method** this handler needs is
`LatestCommittedSnapshot(ctx, repoID) → uint64` on `*Store`; everything else
is a verbatim call. See `effective_graph.go` pattern below.

---

### `internal/skill/semantic/tools_context.go` (tool handler, request-response)

**Analog:** `internal/skill/repomap/skill.go:289-351` (`execGetContext` —
parameter handling + path validation + ranking + budgeted render).

**Imports + arg validation** (mirror repomap lines 289-322):
```go
type GetSemanticContextArgs struct {
    Task          string   `json:"task,omitempty"           jsonschema:"natural-language task description"`
    Files         []string `json:"files,omitempty"          jsonschema:"anchor files for personalized PageRank"`
    Symbols       []string `json:"symbols,omitempty"        jsonschema:"anchor symbol IDs"`
    MaxTokens     int      `json:"max_tokens,omitempty"     jsonschema:"token budget (default 2048, max 32768)"`
    FreshnessMode string   `json:"freshness_mode,omitempty" jsonschema:"allow_stale | require_current | validate_live"`
}

// Handler body:
//   1. checkMode(modeTierRead)
//   2. validatePaths(args.Files, root) + validateSymbolIDs(args.Symbols)
//   3. budget := clampTokens(args.MaxTokens, defaultContextBudget=2048, max=32768)
//      — copy clampTokens from repomap.extractTokenBudget (lines 518-538)
//   4. textRanks := s.retrieval.QueryBleve(args.Task, anchors)        // retrieval/bleve.go
//   5. graphRanks := s.retrieval.PersonalizedPageRank(anchors)        // reads persisted scores
//   6. fused := rrf.Fuse(textRanks, graphRanks, RRFConfig{K:60, w_text:1, w_graph:1})  // rrf.go
//   7. sort.SliceStable(fused, ...) by (score desc, graph_version desc, symbol_id asc)  // sort-before-iterate
//   8. greedyPack(fused, budget) — token-budget packing (D-discretion default greedy)
//   9. envelope: freshness, freshness_mode, graph_version, overlay_active, pending_lsp_files,
//      candidates with per-candidate {evidence: {text_rank, graph_rank, matched_terms[], top_edges[]}, confidence}
```

**Token-clamp helper to copy verbatim** (`internal/skill/repomap/skill.go:516-538`):
```go
func extractTokenBudget(args map[string]interface{}, defaultVal int) int {
    raw, ok := args["token_budget"]
    if !ok { return defaultVal }
    f, ok := raw.(float64); if !ok { return defaultVal }
    budget := int(f)
    if budget < minTokenBudget { budget = minTokenBudget }
    if budget > maxTokenBudget { budget = maxTokenBudget }
    return budget
}
// Reuse with const maxTokenBudget=32768, minTokenBudget=64.
```

---

### `internal/skill/semantic/mode_check.go` (utility, **NEW pattern**)

**Closest analog read pattern:** `internal/mcp/middleware.go:339-346`
(session snapshot read).

**Closest analog error envelope pattern:** `internal/kernel/health/tools.go:50-113`
(closed-enum reason classification).

**NEW for this codebase** — no existing tool enforces mode tier at the handler.
Phase 64 establishes this discipline; Phase 66 GuardrailMiddleware will
piggy-back on the error envelope shape introduced here.

**Recommended implementation:**
```go
// internal/skill/semantic/mode_check.go
package semantic

import (
    "fmt"
    "strings"

    serr "github.com/agenthands/helix/internal/errors"
    "github.com/agenthands/helix/internal/mcp"
)

type modeTier int
const (
    modeTierRead   modeTier = iota
    modeTierReview          // session must be in review or admin
    modeTierAdmin           // session must be in admin
)

// checkMode returns nil when the session's current mode satisfies the
// required tier; otherwise returns a structured PermissionDenied error
// whose Detail field hints at switch_mode elevation.
//
// Source for snap.Mode read: internal/mcp/middleware.go:339-346 — same
// session.Snapshot() pattern under RLock guarantees a race-free read.
func checkMode(snap mcp.SessionSnapshot, required modeTier) error {
    cur := strings.ToLower(snap.Mode)
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
        WithDetail("call switch_mode(target_mode=\"review\") to elevate")
}
```

**Pitfall 6 hardening** (RESEARCH.md): the elevation hint must read
`profile.AllowedModeTransitions[currentMode]` to confirm the transition is
reachable; if not, the hint becomes "this profile cannot reach review;
switch to a less restrictive profile". Planner adds a unit test with
`single_project: true` profile that has no `read→review` edge.

---

### `internal/skill/semantic/envelope.go` (utility, response shape)

**No code analog** — shape is SPEC §23.1-23.4-driven. Use closed-enum
discipline from `internal/kernel/health/tools.go:50-81` for `freshness` and
`status` fields.

**Closed enums (must match SPEC §26.2 / §23.1):**
```go
type Freshness string
const (
    FreshnessFresh                              Freshness = "fresh"
    FreshnessStale                              Freshness = "stale"
    FreshnessStructurallyFreshSemanticallyPending Freshness = "structurally_fresh_semantically_pending"
    FreshnessOverlayActive                      Freshness = "overlay_active"
)

type IndexStatus string  // index_semantic_graph response
const (
    IndexStatusCommitted IndexStatus = "committed"
    IndexStatusBuilding  IndexStatus = "building"
    IndexStatusFailed    IndexStatus = "failed"
)
```

Cross-reference SPEC §23.1-§23.4 line-by-line at plan time before locking
field names. RESEARCH.md "Open Question 3" flags `status=building` as
SPEC-verification-required.

---

### `internal/skill/semantic/runner.go` (service, event-driven)

**Analog:** `internal/semantic/lspenrich/manager.go:15, 82-88` — already
imports `golang.org/x/sync/singleflight` and declares `acquireFlight
singleflight.Group`. Phase 64 mirrors verbatim with key=(workspace, mode).

**Imports + struct** (mirror manager.go lines 1-16, 81-88):
```go
package semantic

import (
    "context"
    "sync"
    "sync/atomic"
    "time"

    semanticstore "github.com/agenthands/helix/internal/semantic/store"
    liveservice   "github.com/agenthands/helix/internal/semantic/live/service"
    "github.com/agenthands/helix/internal/workspace"
    "golang.org/x/sync/singleflight"
)

type IndexRunner struct {
    sf       singleflight.Group     // key="<repoRoot>|<mode>"
    inFlight sync.Map               // key=repoRoot → *buildState
    store    *semanticstore.Store
    live     *liveservice.Service
    timeout  time.Duration          // default per-call ceiling
}

type buildState struct {
    snapshotID    uint64
    startedAt     time.Time
    mode          string
    filesIndexed  atomic.Int64       // atomic for Q-2 background-progress accessor
    filesReused   atomic.Int64
    cancel        context.CancelFunc
    done          chan struct{}
}
```

**`Run(ctx, ws, mode, maxMs)` body** — see RESEARCH.md §"Pattern 4"
sketch (lines 297-353 of RESEARCH.md). Critical invariants:
1. `singleflight.DoChan` returns the *same* result to all waiters → both
   concurrent callers receive the same `snapshot_id` (acceptance test #1).
2. Background ctx is `context.Background()`-derived, NOT the request ctx,
   so a timed-out caller still observes the eventual commit via
   `get_semantic_graph_status` (D-04).
3. Background goroutines registered in daemon errgroup so daemon shutdown
   cancels in-flight builds.

---

### `internal/semantic/retrieval/bleve.go` (service, FTS index)

**No in-tree analog** — bleve is new dependency. Use bleve docs:
- [pkg.go.dev/github.com/blevesearch/bleve/v2](https://pkg.go.dev/github.com/blevesearch/bleve/v2)
- Scorch backend default in v2.0+; `bleve.New(path, mapping)` → `Index`,
  `Index.Index(id, doc)`, `Index.Search(req)`.

**Concept skeleton (planner refines with bleve API at plan time):**
```go
package retrieval

import (
    "github.com/blevesearch/bleve/v2"
    "github.com/blevesearch/bleve/v2/mapping"
)

type Engine struct {
    idx     bleve.Index
    rrfCfg  RRFConfig          // Go-internal constants per D-07
    store   storeReader        // narrow seam → *semantic/store.Store
}

func Open(path string) (*Engine, error)
func (e *Engine) Upsert(ctx context.Context, batch []SymbolDoc) error
func (e *Engine) QueryBleve(task string, anchors []string) ([]TextRank, error)
func (e *Engine) Close() error
```

**Recovery analog** — see `recovery.go` row.

---

### `internal/semantic/retrieval/corpus.go` (transform, batch)

**Closest analog:** `internal/repomap/cache.go` (extractor → row mapping
discipline). Corpus.go takes the same shape: `(snapshot row) → (bleve doc)`.

**D-06 indexed fields:**
```go
type SymbolDoc struct {
    ID       string  // symbol_id (stable key)
    Name     string  // camelCase/snake_case tokenized
    Path     string  // file path components tokenized
    Doc      string  // full docstring/JSDoc/godoc
    CommentWindow string  // ~5-line window above/below decl
}

func MapSymbolToDoc(row semanticstore.SymbolRow, source []byte) SymbolDoc {
    return SymbolDoc{
        ID:            row.SymbolID,
        Name:          tokenizeIdent(row.Name),       // helper splits camel/snake
        Path:          tokenizePath(row.Path),        // splits on / and -
        Doc:           row.Docstring,
        CommentWindow: extractCommentWindow(source, row.LineStart, 5),
    }
}
```

---

### `internal/semantic/retrieval/rrf.go` (utility, transform)

**Analog:** `internal/repomap/pagerank.go` (pure-Go ranking algorithm with
deterministic output).

**RRF formula per CONTEXT.md D-07:**
```go
type RRFConfig struct {
    K       int     // 60 by default
    WText   float64 // 1.0 default (equal weights = classic RRF)
    WGraph  float64 // 1.0 default
}

// Fuse merges text and graph rankings via weighted Reciprocal Rank Fusion.
// Result is sort.SliceStable-d by (score desc, graph_version desc, symbol_id asc)
// to satisfy Phase 62 CR-03 sort-before-iterate doctrine + acceptance test #8.
func Fuse(text []TextRank, graph []GraphRank, cfg RRFConfig) []FusedCandidate {
    scores := make(map[string]float64)
    for rank, t := range text  { scores[t.SymbolID] += cfg.WText  / float64(cfg.K + rank + 1) }
    for rank, g := range graph { scores[g.SymbolID] += cfg.WGraph / float64(cfg.K + rank + 1) }

    out := make([]FusedCandidate, 0, len(scores))
    for id, s := range scores {
        out = append(out, FusedCandidate{SymbolID: id, Score: s, GraphVersion: lookupGV(id)})
    }
    sort.SliceStable(out, func(i, j int) bool {
        if out[i].Score != out[j].Score { return out[i].Score > out[j].Score }
        if out[i].GraphVersion != out[j].GraphVersion { return out[i].GraphVersion > out[j].GraphVersion }
        return out[i].SymbolID < out[j].SymbolID
    })
    return out
}
```

---

### `internal/semantic/retrieval/recovery.go` (service, lifecycle)

**Analog:** `internal/skill/repomap/skill.go:362-471` (lazy walk + cache
populate on first use). Recovery walks `semantic_symbols` at the latest
snapshot rather than the filesystem.

**Recovery procedure** — see RESEARCH.md §"Bleve recovery procedure"
lines 522-544. Key invariants:
1. On `SetActivateCallback` (mirrors `repomap.SetWorkspaceRoot`), compare
   `bleve.IndexMeta.last_indexed_snapshot_id` vs
   `*Store.LatestCommittedSnapshot(ctx, repoID)`.
2. If mismatched → spawn rebuild goroutine attached to **daemon errgroup**.
3. Until rebuild completes, `get_semantic_context` returns
   `freshness=stale, retrieval_pending=true`.

**Pattern variation vs analog:** repomap's lazy-walk fires on the FIRST tool
call (`ensureCache`). Bleve recovery fires on **workspace activation** so
the rebuild is non-blocking for the first tool call (which gets the
`retrieval_pending=true` partial response).

---

### `internal/semantic/store/effective_graph.go` (model, store ext)

**Analog (existing stub shape):** `internal/semantic/store/duckdb.go:483-511`
(existing `QueryEffectiveFiles/Symbols/References/Edges` returning Schema-1
empty stubs).

**Analog (real query shape):** `internal/semantic/store/snapshot.go:242-512`
(real DuckDB tx open, parameterized binds, defensive abort).

**Methods to add (signatures locked by `internal/semantic/graph/scheduler_store.go:43, 56, 61`):**
```go
package store

// QueryEffectiveAdjacency returns (out, in) adjacency for the (repo, projection)
// effective graph (snapshot ⊕ overlay − tombstones). Reads under no workspace
// lock per scheduler_store.go:48-56. Consumes Phase 60 D-04 CAS contract.
func (s *Store) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
    out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
    // 1. Open read tx (no LockOverlayWorkspace per scheduler_store.go:48-56).
    // 2. SELECT src, dst, weight FROM semantic_edges WHERE repo_id=? AND projection=?
    //    UNION ALL
    //    SELECT src, dst, weight FROM semantic_live_overlay_edges
    //    WHERE repo_id=? AND projection=? AND tombstone=false
    // 3. Pivot rows into out[src][dst]=weight and in[dst][src]=weight.
    // 4. Return.
}

// CountStaleScoreRows returns (stale, total) for (repo, projection).
// LOCK-FREE per scheduler_store.go:48-56.
func (s *Store) CountStaleScoreRows(ctx context.Context, repoID, projection string) (
    stale, total int, err error,
) {
    // SELECT
    //   COUNT(*) FILTER (WHERE status='stale') AS stale,
    //   COUNT(*) AS total
    // FROM semantic_pagerank_scores WHERE repo_id=? AND projection=?
}

// MarkAllScoreRowsStale flips every score row to status='stale'.
func (s *Store) MarkAllScoreRowsStale(ctx context.Context, repoID, projection string) error {
    // UPDATE semantic_pagerank_scores SET status='stale'
    // WHERE repo_id=? AND projection=?
}

// LatestCommittedSnapshot returns the latest committed snapshot id for repoID.
// NEW accessor (RESEARCH.md flags it; consumed by tools_status.go and
// retrieval/recovery.go).
func (s *Store) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
    // SELECT MAX(snapshot_id) FROM semantic_snapshots
    //   WHERE repo_id=? AND status='committed'
    // Returns 0 + nil error if no committed snapshot exists yet.
}
```

**Daemon wiring** is already in place: `internal/daemon/rank_wiring.go:
337-356` declares the four methods on `rankStoreAdapter` as stubs that
delegate to `*Store`. Once these methods exist on `*Store`, the adapter
delegates verbatim — Plan 64 swaps each stub body for `return a.store.X(...)`.

---

### `internal/daemon/semantic_wiring.go` (config, bundle)

**Analog:** `internal/daemon/compact_wiring.go` (whole file) +
`internal/daemon/rank_wiring.go` (whole file). Identical shape:

**Bundle struct** (mirror `compactBundle` lines 33-54):
```go
type semanticBundle struct {
    cfg       semanticConfig
    store     *semanticstore.Store
    runner    *semantic.IndexRunner
    retrieval *retrieval.Engine
    skill     *semantic.SemanticSkill
    logger    *slog.Logger
    metrics   *obs.Metrics
}
```

**Constructor** (mirror `newCompactBundle` lines 59-125): nil-safe — returns
nil when `store == nil` (semantic disabled).

**`Run(ctx context.Context) error`** (mirror compact_wiring.go lines 127-138):
blocks on ctx.Done; bundles per-workspace state via `ensureXxx` lazy
construction.

**`ensureRetrieval(ctx, ws)` lazy construction** (mirror
`ensureCompactor` lines 142-187): on first activation, opens the bleve
index in `<workspaceDir>/.helix/semantic.bleve/` and triggers
`recovery.Probe(ws)` in a goroutine.

**`SetActivateCallback` integration** (Phase 64 wiring lands in
`daemon.go` after lines 322-410 / 627-680 — Phase 62/63 wiring blocks):
```go
// Phase 64 wiring (after Phase 63 compactor wiring):
sBndl := newSemanticBundle(...)
g.Go(func() error { return sBndl.Run(ctx) })

if sBndl != nil {
    s := semantic.GetSemanticSkill()
    s.SetStore(newSemanticStoreAdapter(store))
    s.SetScheduler(newSchedulerAdapter(rankBndl))
    s.SetQueue(lspQueue)
    s.SetLive(liveBndl.Service())
    s.SetRunner(sBndl.runner)
    s.SetRetrieval(sBndl.retrieval)

    // Hook into activate callback for bleve recovery probe.
    server.SetActivateCallback(func(ws workspace.WorkspaceKey) {
        sBndl.ensureRetrieval(ctx, ws)  // also fires recovery goroutine
    })

    registerIndexSemanticGraph(server, s, tracer)
    registerRefreshSemanticGraph(server, s, tracer)
    registerGetSemanticGraphStatus(server, s, tracer)
    registerGetSemanticContext(server, s, tracer)
}
```

---

### `internal/daemon/imports.go` (modify — single-line add)

**Analog:** existing line 11 (`_ "github.com/agenthands/helix/internal/skill/repomap"`).

**Diff:**
```go
// Add to the blank-import block after line 12:
_ "github.com/agenthands/helix/internal/skill/semantic"
```

Single line. No new pattern.

---

### Profile/Mode YAML modifications

**Analog (profile):** `internal/profile/profiles/full.yaml:11-21` — `skills:` block.
**Analog (mode):** `internal/profile/modes/review.yaml:6-9` — `skills:` block.

**D-14 shape: add `semantic` to all 5 profiles' `skills:` block:**
```yaml
# diff for full.yaml / claude-code.yaml / codex.yaml / ide-assistant.yaml / ci-bot.yaml
 skills:
   - symbol-retrieval
   - symbol-editing
   ...
+  - semantic
```

**Mode-tier coverage:**
- `read.yaml`: ADD `- semantic` to skills (the `read+` tools — refresh,
  status, context — need to be visible in `tools/list`).
  ALSO add `index_semantic_graph` to `exclude_tools` (so it's filtered out
  from `tools/list` for read-mode sessions; the handler-side `checkMode`
  is the second layer per RESEARCH Open-Q #5 belt-and-braces recommendation).
- `review.yaml`: ADD `- semantic` to skills.
- `admin.yaml`: ADD `- semantic` to skills.
- `edit.yaml`: ADD `- semantic` to skills (same `read+`/`review+` mode-tier
  enforcement applies inside handlers).

---

### Test files (table-driven + concurrency + property)

**Analog patterns:**
- Unit table-driven: `internal/kernel/symbols/*_test.go` (typed args + handler call).
- Concurrency: any `*_race_test.go` using `t.Parallel()` + `singleflight`-target invariant.
- Property/determinism: Phase 62 sort-before-iterate test — sort 10 times, assert byte-equal.
- Profile filter: existing `internal/mcp/middleware_test.go` for `ProfileFilterMiddleware`.

**Acceptance-test → file map** (RESEARCH.md §"Phase Requirements → Test Map"
ID 655-678 is exhaustive; planner copies that table verbatim into the
PLAN.md test plan):

| Acceptance | Test file |
|------------|-----------|
| #1 sync-with-timeout (TOOL-01) | `tools_index_test.go::TestIndex_TimeoutPartial` |
| #1 singleflight join (TOOL-01) | `runner_singleflight_test.go::TestIndex_SingleflightJoin -race` |
| #2 refresh never commits (TOOL-02) | `tools_refresh_test.go::TestRefresh_NoSnapshotCommit` |
| #3 status SPEC §23.3 (TOOL-03) | `tools_status_test.go::TestStatus_*` |
| #4 context determinism (TOOL-04) | `tools_context_test.go::TestContext_Determinism -count=10` |
| #5 mode-violation envelope | `tools_index_test.go::TestIndex_ModeViolationEnvelope` |
| #6 bleve sizing | `cmd/bench-bleve/` Wave-0 task |
| #7 effective-graph CAS | `internal/semantic/store/effective_graph_test.go::TestQueryEffectiveAdjacency` |
| #8 bleve recovery | `internal/semantic/retrieval/recovery_test.go::TestRecovery_*` |

---

## Shared Patterns

### Auth: Mode-tier check (NEW — Phase 64 establishes)
**Source:** `internal/skill/semantic/mode_check.go` (NEW; see row above).
**Apply to:** All 4 tool handlers, FIRST line of handler body before any work.
```go
if err := checkMode(s.session(ctx), modeTierReview); err != nil {  // or modeTierRead
    return errorResult(err.Error()), nil, nil
}
```

### Auth: Path-traversal validation
**Source:** `internal/skill/repomap/skill.go:311-319`.
**Apply to:** All tool handlers that accept `Paths []string` or `Files []string`
(index, refresh, context).
```go
if strings.Contains(fp, "..") {
    s.logger.Warn("rejected file path with path traversal", "path", fp)
    return "", serr.New(serr.InvalidArgs,
        fmt.Sprintf("file path %q contains '..' (path traversal not allowed)", fp)).WithTool(toolName)
}
if filepath.IsAbs(fp) && !strings.HasPrefix(fp, s.resolveRoot()) {
    s.logger.Warn("rejected absolute file path outside workspace root", "path", fp)
    return "", serr.New(serr.InvalidArgs,
        fmt.Sprintf("file path %q is outside workspace root", fp)).WithTool(toolName)
}
```

### Error envelope: closed-enum reason classification
**Source:** `internal/kernel/health/tools.go:50-113` + `internal/errors/serr.go`.
**Apply to:** All tool handlers (mode_violation, internal, timeout, partial, success).
**Pattern:** never include raw error strings in MCP response; log the raw error
via `slog` at `Warn`, return a closed-enum reason in the structured envelope.
This matches RESEARCH.md security row "Information leak via raw error text".

### Token-budget clamp
**Source:** `internal/skill/repomap/skill.go:516-538` (`extractTokenBudget`).
**Apply to:** `tools_context.go` only (`MaxTokens` field).
**Constants:** `defaultContextBudget=2048, maxTokenBudget=32768, minTokenBudget=64`.

### Workspace activation hook
**Source:** `internal/skill/repomap/skill.go:100-106` (`SetWorkspaceRoot`)
+ `internal/daemon/compact_wiring.go:142-187` (`ensureCompactor`).
**Apply to:** `internal/daemon/semantic_wiring.go::ensureRetrieval(ctx, ws)`.
Per-workspace lazy construction with `sync.Map` registry.

### Sort-before-iterate determinism
**Source:** Phase 62 CR-03 doctrine, reaffirmed at
`internal/semantic/graph/scheduler.go` flow.
**Apply to:** `internal/semantic/retrieval/rrf.go::Fuse` and any place
`tools_context.go` iterates a map-of-scores.
**Tiebreak:** `(score desc, graph_version desc, symbol_id asc)`.
**Test:** determinism harness asserts byte-identical JSON across 10 runs
of the same query.

### Singleflight idiom
**Source:** `internal/semantic/lspenrich/manager.go:15, 82-88`.
**Apply to:** `internal/skill/semantic/runner.go`.
**Key:** `(workspace_repoRoot, mode)` per CONTEXT.md D-02.
**Critical:** background goroutine derives ctx from `context.Background()`
(NOT request ctx) so timed-out callers still observe the eventual commit
via `get_semantic_graph_status`.

### Compile-time interface guards
**Source:** `internal/daemon/rank_wiring.go:420-424`,
`internal/daemon/compact_wiring.go:259-268`.
**Apply to:** `internal/daemon/semantic_wiring.go` for every accessor adapter.
```go
var (
    _ semantic.StoreAccessor     = (*semanticStoreAdapter)(nil)
    _ semantic.SchedulerAccessor = (*schedulerAccessorForSemantic)(nil)
    // etc.
)
```

### Daemon errgroup attachment
**Source:** `internal/daemon/rank_wiring.go:171-176` (`go func() { s.Run(runCtx); ... }()`).
**Apply to:** `runner.go` background-build goroutines AND `recovery.go` rebuild
goroutine. Both must use the daemon's `runCtx` (captured via the bundle's
`Run(ctx)` entrypoint), NOT `context.Background()` from the request thread.

---

## No Analog Found

| File | Role | Reason |
|------|------|--------|
| `internal/semantic/retrieval/bleve.go` | service (FTS) | bleve is a new dependency; no in-tree analog. Use bleve docs at pkg.go.dev. Recovery procedure design originates in RESEARCH.md §"Bleve recovery procedure" lines 522-544. |
| `internal/skill/semantic/envelope.go` | utility (shape) | Response shape is SPEC §23.1-§23.4-driven. Closed-enum discipline copied from `internal/kernel/health/tools.go:50-81`. |

---

## Metadata

**Analog search scope:**
- `internal/skill/{repomap,memory,workflow}/` (skill-package precedent)
- `internal/kernel/{health,help,symbols,edit,fileops,diag}/` (tool registration + adapters)
- `internal/mcp/` (middleware, registry, session)
- `internal/daemon/{daemon,compact_wiring,rank_wiring,imports}.go` (bundle pattern)
- `internal/semantic/{store,graph,live,lspenrich,compact}/` (existing semantic surface)
- `internal/profile/{profiles,modes}/*.yaml` (config shape)
- `cmd/{vet-noduckdb,vet-compact-uses-store}/` (vet analyzer template)

**Files scanned:** ~30 (path-pinned by RESEARCH.md plus 6 follow-up reads).

**Pattern extraction date:** 2026-05-07.

**Confidence:**
- Skill registration / typed-args / wiring patterns: **HIGH** (every excerpt
  pinned to a specific in-tree line range that the planner can copy).
- Mode-tier check: **MEDIUM** (NEW pattern; the read-from-snapshot half is
  well-established at `internal/mcp/middleware.go:339-346`, the
  enforcement half is greenfield).
- Bleve retrieval: **LOW** (no in-tree analog; planner relies on bleve
  v2.4.4 official docs + RESEARCH.md design sketch).
