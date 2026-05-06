# Phase 62: Graph Engine, Ranking & Type Resolution - Pattern Map

**Mapped:** 2026-05-06
**Files analyzed:** 36 (new + modified)
**Analogs found:** 33 / 36

## File Classification

### Plan P01 — `internal/graph/` shared engine + repomap migration

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/graph/pagerank.go` | utility (algorithm) | transform | `internal/repomap/pagerank.go` | exact (lift + determinism patch) |
| `internal/graph/options.go` | utility (struct) | transform | `internal/repomap/pagerank.go:20` (parameters) | role-match |
| `internal/graph/personalize.go` | utility | transform | `internal/repomap/pagerank.go:44-72` | exact |
| `internal/graph/pagerank_test.go` | test | unit | `internal/repomap/pagerank_test.go` | exact |
| `internal/graph/testdata/pagerank/` | fixture | test data | `internal/repomap/testdata/` (existing) | reuse |
| `internal/repomap/pagerank.go` | utility (REWRITE adapter) | transform | self (current impl) → adapter over `internal/graph` | re-pin |
| `internal/repomap/pagerank_test.go` | test (RE-PIN vectors) | unit | self | re-pin |

### Plan P02 — `graph_version` advance machinery + score_status read API

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/semantic/graph/repair.go` | model (typed value) | transform | `internal/semantic/live/coalescer/coalesce.go` (typed merge value) | role-match |
| `internal/semantic/graph/apply_repair.go` | service | request-response | `internal/semantic/store/overlay.go:79-141` (BeginOverlayTx mutex pattern) | role-match |
| `internal/semantic/graph/status.go` | service (read API) | request-response | `internal/semantic/lspenrich/status.go` | role-match |
| `internal/semantic/graph/ranker.go` | service (interface + impl) | request-response | `internal/semantic/lspenrich/manager.go:300-318` (Status snapshot) | role-match |
| `internal/semantic/store/overlay.go` (EXTEND `UpsertGraphScores`, `UpsertEdgesWithMerge`) | service | CRUD | `internal/semantic/store/overlay.go:204-249` (UpsertOverlayFile) | exact (same file extension) |
| `internal/semantic/live/handler/handler.go` (EXTEND post-commit hook) | controller | event-driven | self (existing Dispatch switch + markBulkPending) | exact (same file extension) |
| `internal/semantic/lspenrich/cascade.go:441` (TOUCH stub call site) | controller | request-response | self (line 440-441 stub) | exact (call site swap) |

### Plan P03 — RankScheduler + 1-hop frontier + WriteInvalidations consumer

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/semantic/graph/scheduler.go` | service (goroutine) | event-driven | `internal/semantic/live/coalescer/coalescer.go:80-203` (debounce + maxTimer) | exact |
| `internal/semantic/graph/frontier.go` | utility | transform | `internal/repomap/pagerank.go:80-118` (graph traversal) | role-match |
| `internal/semantic/graph/full_recompute.go` | service | batch | `internal/semantic/lspenrich/cascade.go` (one-shot worker) | role-match |
| `internal/semantic/graph/invalidations_consumer.go` | service | transform | `internal/semantic/lspenrich/cascade.go:79-89` (CascadeTx narrow seam) | role-match |
| `internal/semantic/graph/metrics.go` | utility (helpers) | request-response | `internal/obs/metrics.go:391-413` (drop-on-unknown helpers) | exact |
| `internal/semantic/graph/trace.go` | utility | request-response | `internal/semantic/lspenrich/trace.go` | exact |
| `internal/semantic/graph/scheduler_test.go` | test | unit | `internal/semantic/live/coalescer/coalescer_test.go` | exact |
| `internal/semantic/store/overlay.go` (real `WriteInvalidations`) | service | CRUD | `internal/semantic/store/overlay.go:264-306` (MarkSymbolsDeleted/MarkReferencesDeleted) | exact |

### Plan P04 — Weak-component clustering

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/semantic/cluster/weak.go` | utility (algorithm) | transform | `internal/graph/pagerank.go` (deterministic graph algo) | role-match |
| `internal/semantic/cluster/persist.go` | service | CRUD | `internal/semantic/store/overlay.go:204-249` (UpsertOverlayFile) | role-match |
| `internal/semantic/cluster/weak_test.go` | test | unit | `internal/repomap/pagerank_test.go` (golden hex digest test) | role-match |
| `internal/semantic/store/overlay.go` (EXTEND `UpsertClusters`, `UpsertClusterMembers`) | service | CRUD | `internal/semantic/store/overlay.go:264-283` (per-id loop pattern) | exact |

### Plan P05 — Type resolver shared core + per-language

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/semantic/types/resolver.go` (interface + dispatch) | service (registry) | request-response | `internal/semantic/extract/registry.go` (per-language dispatch) | exact |
| `internal/semantic/types/chain.go` | utility (algorithm) | transform | RESEARCH §38.4 spec only — no codebase analog | spec-only |
| `internal/semantic/types/fixpoint.go` | utility (algorithm) | transform | RESEARCH §38.5 spec only — no codebase analog | spec-only |
| `internal/semantic/types/ladder.go` | utility (constants) | transform | RESEARCH §38.2 spec only | spec-only |
| `internal/semantic/types/emit.go` | service (edge emission + merge) | CRUD | `internal/semantic/lspenrich/cascade.go` (edge emission pattern) | role-match |
| `internal/semantic/types/golang/resolver.go` | service | request-response | `internal/semantic/extract/golang/provider.go` | exact (layout mirror) |
| `internal/semantic/types/python/resolver.go` | service | request-response | `internal/semantic/extract/python/provider.go` | exact |
| `internal/semantic/types/typescript/resolver.go` | service | request-response | `internal/semantic/extract/typescript/provider.go` | exact |
| `internal/semantic/types/java/stub.go` | service (LSP-conditional stub) | request-response | RESEARCH Code Example 4 + cascade edge query | role-match |
| `internal/semantic/types/{php,ruby}/stub.go` | service (always-0.20 stub) | request-response | (sibling stub.go in java) | role-match |
| `internal/semantic/types/<lang>/scope.go` | utility (path helper) | transform | (none — pure stdlib `filepath.Stat`) | spec-only |
| `internal/semantic/types/<lang>/comment.go` | utility (parser) | transform | (none — hand-rolled per D-12) | spec-only |
| `internal/semantic/types/testdata/<lang>/` | fixture | test data | `internal/semantic/extract/golang/testdata/` | exact (layout mirror) |

### Cross-cutting wiring

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/daemon/daemon.go` (EXTEND RankScheduler + Resolver wiring) | controller (bootstrap) | event-driven | `internal/daemon/daemon.go:295-317` (live bundle wiring), `:652-659` (errgroup g.Go) | exact (same file extension) |
| `internal/config/defaults.go` (4 new keys) | config | static | `internal/config/defaults.go:115-125` (graph.* + pagerank.*) | exact (same file extension) |
| `internal/semantic/config.go` (struct fields) | config | static | self (existing PageRank/Types sub-structs at lines 190-264) | exact |
| `internal/obs/metrics.go` (5 new metric Vecs) | utility (registration) | request-response | `internal/obs/metrics.go:285-313` (LSPEnrichment* family) | exact (same file extension) |
| `internal/obs/metrics_labels_test.go` (Phase 62 carve-out) | test | unit | self (existing Phase 60/61 carve-outs) | exact |

## Pattern Assignments

### `internal/graph/pagerank.go` (utility, transform)

**Analog:** `internal/repomap/pagerank.go`

**Algorithm body to lift** (lines 20-132 of `internal/repomap/pagerank.go`):

The current implementation already has the math correct; what changes is **determinism**. RESEARCH.md GRAPH-01 row pinpoints the exact map-iteration leak sites:

- `:55` — uniform-teleport loop (currently iterates `files` slice; OK)
- `:62-67` — re-normalize loop (`for f := range teleport` — **LEAK**: ranges map)
- `:65` — `for f := range teleport { teleport[f] /= total }` — **LEAK**
- `:75-78` — rank init `for f, w := range teleport` — **LEAK**
- `:104-106` — `for _, f := range files` (OK; iterates slice; but `files` constructed at `:28-31` via `for f := range g.Files` is ALREADY non-deterministic before sort)
- `:108-118` — link component `for src, targets := range edges` — outer iter is **LEAK**; inner `for dst, w := range targets` is associative (OK)
- `:121-123` — convergence `for _, f := range files` (depends on sort)

**Determinism patch** (D-03): sort `files` ONCE at entry, iterate `sorted` everywhere a map is keyed by node id. Inner edge sums (`for dst, w := range edges[src]`) stay map-ranged because addition is commutative.

**Imports pattern** (D-01 hard invariant — `math`, `sort`, `cmp` ONLY, no project imports):
```go
package graph

import (
    "cmp"
    "math"
    "sort"
)
```

**Public surface** (D-01):
```go
type Options struct {
    Damping       float64           // default 0.85
    Epsilon       float64           // default 1e-6
    MaxIter       int               // default 100
    Personalize   map[any]float64   // optional; nil → uniform teleport
}

func PageRank[T cmp.Ordered](nodes []T, edges map[T]map[T]float64, opts Options) map[T]float64
func RankNodes[T cmp.Ordered](nodes []T, edges map[T]map[T]float64, opts Options) []Ranked[T] // sorted desc
```

**Tiebreak invariant** (GRAPH-03 + D-03): equal scores → sorted-key NodeID order via `RankNodes` stable sort with secondary key `node-id`.

---

### `internal/repomap/pagerank.go` (utility, transform — REWRITE as adapter)

**Analog:** self + `internal/graph/PageRank`

**FileGraph public API stays unchanged** (CONTEXT D-04 invariant). The body delegates:

```go
// Pseudo-shape of the rewrite (planner finalizes signatures)
func (g *FileGraph) PageRank(damping, epsilon float64, maxIter int, personalization map[string]float64) map[string]float64 {
    g.mu.RLock()
    nodes := make([]string, 0, len(g.Files))
    for f := range g.Files { nodes = append(nodes, f) }
    edges := copyEdges(g.Edges) // existing :34-41 copy under lock
    g.mu.RUnlock()
    return graph.PageRank(nodes, edges, graph.Options{
        Damping: damping, Epsilon: epsilon, MaxIter: maxIter,
        Personalize: toAnyMap(personalization),
    })
}
```

**Test re-pin** (D-04, one-time accept): `internal/repomap/pagerank_test.go` golden vectors recomputed against the new deterministic engine. Score *values* unchanged; only ordering across equal scores becomes stable.

---

### `internal/semantic/graph/scheduler.go` (service, event-driven)

**Analog:** `internal/semantic/live/coalescer/coalescer.go`

**Lifecycle pattern** (lines 80-203):
```go
// From coalescer.go:80-93
type Coalescer struct {
    workspaceID workspace.WorkspaceKey
    cfg         Config
    handler     EventHandler
    logger      Logger
    metrics     MetricsSink
    in          chan live.SourceChangeEvent
    drops       atomic.Uint64

    mu       sync.Mutex
    pending  map[string]live.SourceChangeEvent
    timer    *time.Timer
    maxTimer *time.Timer
}

// From coalescer.go:153-171 — closed-channel Run loop
func (c *Coalescer) Run(ctx context.Context) error {
    flush := c.makeFlush(ctx)
    for {
        select {
        case <-ctx.Done():
            c.mu.Lock()
            if c.timer != nil { c.timer.Stop() }
            if c.maxTimer != nil { c.maxTimer.Stop() }
            c.mu.Unlock()
            return ctx.Err()
        case ev := <-c.in:
            c.accept(ev, flush)
        }
    }
}

// From coalescer.go:193-202 — debounce + max-batch ceiling pattern
if c.timer != nil { c.timer.Stop() }
c.timer = time.AfterFunc(c.cfg.Debounce, flush)
if c.maxTimer == nil {
    // Start the max-batch ceiling on the FIRST event in this batch.
    c.maxTimer = time.AfterFunc(c.cfg.MaxBatchDelay, flush)
}
```

**RankScheduler maps:**
- `Coalescer.in chan live.SourceChangeEvent` → `RankScheduler.in chan GraphVersion`
- `cfg.Debounce` → `pagerank.repair_debounce_ms` (D-08 default 2000ms)
- `cfg.MaxBatchDelay` → `pagerank.full_recompute_idle_ms` (D-08 default 60000ms; semantically a "long-idle" not "max-batch", but timer mechanic is identical)
- `flush()` → `runIncrementalRepair(ctx, lastSeenGV)` / `maybeFullRecompute(ctx)`

**Pitfall 4 (Frontier Overflow Race)** mandates that the long-idle timer is reset only on (a) full recompute completion or (b) incremental covering all stale rows — NOT on every `in` event. This means the RankScheduler does NOT mirror coalescer's `accept()` 1:1; the long-idle reset is gated.

---

### `internal/semantic/graph/apply_repair.go` (service, request-response)

**Analog:** `internal/semantic/store/overlay.go:79-141` (BeginOverlayTx mutex pattern)

**Per-workspace mutex acquisition** (Don't Hand-Roll table — re-use `s.overlayLockFor(repoID)`):
```go
// From overlay.go:79-141 — the lock dance ApplyRepair must mirror
mu := s.overlayLockFor(repoID)
mu.Lock()
// ... do work that also writes the overlay row ...
// epoch bump issued on s.db (NOT on tx) so rollback doesn't rewind it
// THEN open the user-visible tx
tx, err := s.db.BeginTx(ctx, nil)
```

**ApplyRepair flow** (CONTEXT D-06 + RESEARCH Pattern 2):
1. Acquire per-workspace mutex (same one BeginOverlayTx holds).
2. Short-circuit if `repair.IsEmpty()` (body-only edits).
3. Apply tombstone/dirty markers via existing `MarkSymbolsDeleted` / `MarkEdgesDeleted` helpers.
4. Apply two-phase comment merge via new `tx.UpsertEdgesWithMerge` (D-14).
5. `UPDATE semantic_live_overlay_meta SET graph_version = graph_version + 1 RETURNING graph_version`.
6. Emit on `graph_version` channel for RankScheduler.
7. Commit (releases mutex via existing `t.releaseLock()` defer).

**Single-bump invariant** (D-06): only this method calls the `graph_version` UPDATE statement; tested by `TestGraphVersionAdvanceCount`.

---

### `internal/semantic/store/overlay.go` (service, CRUD — EXTEND)

**Analog:** self — Add new methods following the same shape as existing `UpsertOverlayFile` (lines 204-222) and `MarkSymbolsDeleted` (lines 264-283).

**Pattern for new `UpsertGraphScores`** (mirrors UpsertOverlayFile exactly):
```go
// Mirrors overlay.go:204-222
func (t *OverlayTx) UpsertGraphScores(ctx context.Context, projection string, rows []ScoreRow) error {
    if t == nil || t.tx == nil { return fmt.Errorf("UpsertGraphScores: nil tx") }
    if len(rows) == 0 { return nil } // empty-list no-op (mirrors :268)
    for _, r := range rows {
        _, err := t.tx.ExecContext(ctx, `
            INSERT INTO semantic_graph_scores (
                repo_id, projection, node_id, score, graph_version,
                status, write_epoch, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, now())
            ON CONFLICT (repo_id, projection, node_id) DO UPDATE SET
                score          = excluded.score,
                graph_version  = excluded.graph_version,
                status         = excluded.status,
                write_epoch    = excluded.write_epoch,
                updated_at     = excluded.updated_at
        `, t.repoID, projection, r.NodeID, r.Score, r.GraphVersion, r.Status, t.epoch)
        if err != nil { return fmt.Errorf("UpsertGraphScores(%q, %d): %w", t.repoID, r.NodeID, err) }
    }
    return nil
}
```

**Pattern for `UpsertEdgesWithMerge`** (D-14 two-phase merge predicate enforced at storage boundary, RESEARCH Pitfall 3):
```sql
-- Step 1 (skip-if-LSP-already-validated; planner adapts to Go):
SELECT 1 FROM semantic_live_overlay_edges
 WHERE repo_id = ? AND src_node_id = ? AND dst_node_id = ?
   AND edge_kind = ?
   AND validation_state = 'validated'
   AND confidence >= 1.0
   AND source LIKE 'lsp.%'
   AND status = 'live'
 LIMIT 1;

-- Step 2 (delete comment row before LSP insert):
DELETE FROM semantic_live_overlay_edges
 WHERE repo_id = ? AND src_node_id = ? AND dst_node_id = ?
   AND edge_kind = ?
   AND source LIKE 'comment.%'
   AND status = 'live';

-- Step 3 (insert):
INSERT INTO semantic_live_overlay_edges (...) VALUES (...);
```

**Pattern for `WriteInvalidations`** (real impl) follows `MarkEdgesDeleted` (lines 376-396) — per-id loop in SQL, empty-list no-op, returns wrapped error.

---

### `internal/semantic/lspenrich/cascade.go:441` (controller, request-response — TOUCH)

**Analog:** self (line 440-441 stub)

**Current state** (line 440-441):
```go
// Phase 60 D-04 invalidations stub — Phase 62 wires the consumer.
_ = tx.WriteInvalidations(ctx)
```

**Phase 62 swap:** the producer side (cascade) does NOT change. Phase 62 fills the **consumer** (`internal/semantic/graph/invalidations_consumer.go`) which reads the rows the now-real `WriteInvalidations` writes (or, per RESEARCH Open Question 3, derives `GraphRepair` from existing tombstone rows on `semantic_live_overlay_edges`/`_symbols` — the recommendation is the derived path, no schema migration).

**If derived path chosen (recommended):** cascade's `_ = tx.WriteInvalidations(ctx)` becomes a real call that writes nothing today (or is removed if the comment+stub are vestigial); `apply_repair.go` reads tombstones directly from `semantic_live_overlay_edges` via `effective.go`.

---

### `internal/semantic/live/handler/handler.go` (controller, event-driven — EXTEND post-commit hook)

**Analog:** self — Mirror Phase 61 D-05's `markBulkPending` lane-decision pattern (handler.go:140-149) and the `Dispatch` switch (lines 125-153).

**Hook placement:** RESEARCH Assumption A2 marks the handler as the natural fire site. `updateChangedFileWithKind` (lines 177-200) is the single method that opens an OverlayTx, upserts, commits, and enqueues. Post-commit hook fires `ApplyRepair` AFTER `tx.Commit()` succeeds and BEFORE `EnqueueLane`:

```go
// Pseudo-shape (planner finalizes):
if err := tx.Commit(); err != nil { return err }
// === Phase 62 P02 hook ===
if h.RankApplier != nil {
    repair := graphpkg.ComputeGraphRepair(tx.Diff()) // tx exposes diff or handler tracks
    if !repair.IsEmpty() {
        _ = h.RankApplier.ApplyRepair(ctx, repoID, repair)
    }
}
// === end hook ===
if h.LSPQueue != nil { _ = h.LSPQueue.EnqueueLane(...) }
```

**Decision policy** (D-06): `repair.IsEmpty()` short-circuits — body-only edits do NOT bump `graph_version`.

---

### `internal/semantic/types/resolver.go` (service, request-response — dispatch)

**Analog:** RESEARCH Code Example 3 + `internal/semantic/extract/registry.go` per-language dispatch.

**Dispatcher shape** (D-11):
```go
type Resolver interface {
    ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error)
    ResolveSymbol(ctx context.Context, req SymbolRequest) (SymbolResponse, error)
}

type registry struct { byLang map[string]Resolver }

func NewDispatcher() *registry {
    return &registry{byLang: map[string]Resolver{
        "go":         golang.NewResolver(),
        "python":     python.NewResolver(),
        "typescript": typescript.NewResolver(),
        "javascript": typescript.NewResolver(), // shared with TS
        "java":       java.NewStub(...),
        "php":        php.NewStub(),
        "ruby":       ruby.NewStub(),
    }}
}
```

**Unknown-language fallback** (D-12 invariant): unknown languages → `Confidence: 0.20, ValidationState: "unresolved"`, NOT a missing row.

---

### `internal/semantic/types/golang/resolver.go` (service, request-response)

**Analog:** `internal/semantic/extract/golang/provider.go`

**Layout pattern** (lines 1-50):
```go
// Package goextract is the per-language tree-sitter extraction provider
// for Go source files. ...
package goextract

import (
    "context"
    _ "embed"
    "fmt"
    "path/filepath"
    "strings"

    tree_sitter "github.com/tree-sitter/go-tree-sitter"
    "github.com/agenthands/helix/internal/semantic/extract"
    "github.com/agenthands/helix/internal/treesitter"
)

type Provider struct {
    grammar *tree_sitter.Language
    query   *tree_sitter.Query
}
func NewProvider(grammars *treesitter.GrammarRegistry) extract.Provider { ... }
```

**Type resolver mirror** (D-11 + CLAUDE.md "no second GrammarRegistry"):
```go
package goresolve

import (
    "context"
    "github.com/agenthands/helix/internal/semantic/store"
    "github.com/agenthands/helix/internal/semantic/types"
)

type Resolver struct {
    store types.EffectiveReader // narrow seam over store.QueryEffectiveEdges etc.
}

func NewResolver(s types.EffectiveReader) *Resolver { return &Resolver{store: s} }
func (r *Resolver) ResolveChain(ctx context.Context, req types.ChainRequest) (types.ChainResponse, error) { ... }
```

**Critical: NO tree-sitter import** — type resolvers read facts already extracted by Phase 59 (CLAUDE.md constraint, RESEARCH Anti-Pattern "Tree-sitter access in type resolver").

**Testdata layout mirror** (`testdata/<fixture-name>/{input.go,expected.json}`) follows `internal/semantic/extract/golang/testdata/`.

---

### `internal/semantic/types/java/stub.go` (service, request-response)

**Analog:** RESEARCH Code Example 4

**Short-circuit pattern** (D-12 invariant):
```go
type Stub struct{ store EffectiveReader }

func (s *Stub) ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error) {
    edges, err := s.store.QueryEffectiveEdges(ctx, store.EdgeQuery{
        RepoID: req.RepoID, SrcNodeID: req.RefNodeID, EdgeKind: "RESOLVES_TO",
    })
    if err != nil { return ChainResponse{}, err }
    for _, e := range edges {
        if e.Confidence >= 1.0 && strings.HasPrefix(e.Source, "lsp.") {
            return ChainResponse{
                Resolved: true, Target: e.DstNodeID,
                Confidence: 1.0, ValidationState: "validated",
                Source: e.Source, // preserve "lsp.<call>" lineage
            }, nil
        }
    }
    return ChainResponse{
        Resolved: false, Confidence: 0.20,
        ValidationState: "unresolved",
        Reason: "java: no LSP fact + no static analyzer",
    }, nil
}
```

PHP/Ruby stubs DO NOT take a store dep — they unconditionally return the 0.20 unresolved response.

---

### `internal/daemon/daemon.go` (controller, event-driven — EXTEND)

**Analog:** self — Two patterns to mirror.

**Pattern A: setter wiring** (lines 295-317, the live bundle):
```go
// daemon.go:295-317 — live-update pipeline wiring
live := buildLiveBundle(
    cfg.SemanticIndex.LiveUpdates,
    cfg.SemanticIndex.LSPEnrichment,
    semanticStore,
    semanticScheduler,
    k,
    observability.Metrics(),
    logger,
)
if live != nil {
    logger.Info("live-update pipeline wired", ...)
}
```

Phase 62 adds an analogous `buildRankBundle(...)` constructor returning a struct holding `RankScheduler` + `Resolver` + `ApplyRepair` callback. RESEARCH Open Question 4 suggests one combined setter `SetSemanticGraph(ranker, resolver)`.

**Pattern B: errgroup ownership** (lines 645-659):
```go
// daemon.go:645-659 — Phase 61 enrichment manager runs in errgroup
g, gctx := errgroup.WithContext(ctx)

g.Go(func() error { return d.kernel.Run(gctx) })

if d.live != nil {
    g.Go(func() error {
        return d.live.Run(gctx)
    })
}
```

Phase 62 adds (after live):
```go
if d.rank != nil {
    g.Go(func() error { return d.rank.Run(gctx) })
}
```

The RankScheduler is one goroutine per workspace (D-08); when a workspace activates the scheduler is attached. This may live inside the `live.Run` errgroup OR as a sibling — planner picks. The simpler option is sibling errgroup g.Go that internally manages per-workspace child schedulers.

---

### `internal/config/defaults.go` (config, static — EXTEND)

**Analog:** self (lines 115-125, the existing `pagerank.*` keys).

**Pattern** (mirrors lines 122-125 verbatim):
```go
// pagerank.* — existing keys (already shipped Phase 57)
"semantic_index.pagerank.damping":        float64(0.85),
"semantic_index.pagerank.epsilon":        float64(0.000001),
"semantic_index.pagerank.max_iterations": 100,

// Phase 62 NEW keys (4 total per RESEARCH "New config keys"):
"semantic_index.pagerank.repair_debounce_ms":       2000,          // D-08
"semantic_index.pagerank.full_recompute_idle_ms":   60000,         // D-08
"semantic_index.pagerank.full_recompute_threshold": float64(0.25), // D-08 (koanf float gotcha)
"semantic_index.types.comment_parsers_enabled":     []string{"tsdoc", "jsdoc", "godoc", "python_type_comments", "phpdoc", "yard"}, // D-12 (koanf slice gotcha)
```

**Float/slice gotchas** (file header):
- Float: wrap `float64(0.25)` — koanf's confmap decodes untyped Go literals as `int`.
- Slice: declare as `[]string{...}` — bare `[]any{...}` does not bind to `[]string`.

The existing `semantic_index.type_resolution.*` block (lines 154-159) already covers `max_chain_depth`, `max_fixpoint_iterations`, `comment_fallbacks`, `emit_unresolved_edges`. RESEARCH "Environment Availability" line 1136 confirms most type_resolution.* keys ship; only `comment_parsers_enabled` is new.

**`internal/semantic/config.go` mirror struct fields** must be added to existing `PageRankConfig` and `TypeResolutionConfig` sub-structs (lines 190-264 per RESEARCH Sources).

---

### `internal/obs/metrics.go` (utility, request-response — EXTEND)

**Analog:** self — `LSPEnrichment*` family (lines 285-313 + 322-346 register block + 391-413 helper methods).

**Vec declaration pattern** (mirrors LSPEnrichmentDurationVec lines 285-294):
```go
// Phase 62 P02/P03: PageRank engine duration histogram.
// Closed-enum "scope" ∈ {"incremental","full"}; "projection" ∈ {"call_graph"}.
SemanticGraphPagerankDurationVec *prometheus.HistogramVec

// Phase 62: score status counter (read-time emission).
// Closed-enum "projection" ∈ {"call_graph"}; "status" ∈ {"exact","approximate","stale","missing"}.
SemanticGraphScoreStatusVec *prometheus.CounterVec

// Phase 62: repair outcome counter.
// Closed-enum "outcome" ∈ {"applied","frontier_overflow","preempted","error"}.
SemanticGraphRepairVec *prometheus.CounterVec

// Phase 62: graph_version gauge per workspace.
// Closed-enum "workspace_label" via existing bounded carve-out.
SemanticGraphVersionGauge *prometheus.GaugeVec

// Phase 62: type-resolution outcome counter.
// Closed-enum "language" ∈ AllowedLabels; "confidence_tier" ∈ {"1.00","0.90","0.80","0.70","0.60","0.45","0.20"}.
SemanticTypesResolutionVec *prometheus.CounterVec
```

**Helper-method drop-on-unknown pattern** (lines 391-413, RenameStrategyInc / RepoMapLookup template):
```go
// Phase 62 helpers — drop unknown closed-enum values to preserve bounded cardinality.
var pagerankScopes = map[string]struct{}{"incremental": {}, "full": {}}
var pagerankProjections = map[string]struct{}{"call_graph": {}}

func (m *Metrics) SemanticGraphPagerankObserve(scope, projection string, seconds float64) {
    if _, ok := pagerankScopes[scope]; !ok { return }
    if _, ok := pagerankProjections[projection]; !ok { return }
    m.SemanticGraphPagerankDurationVec.WithLabelValues(scope, projection).Observe(seconds)
}
// Same drop-on-unknown shape for the other 4 metrics.
```

**MustRegister block** (lines 322-346): append the 5 new vecs.

**`internal/obs/metrics_labels_test.go` carve-out**: add closed-enum value lists for the 4 new label NAMES (`scope`, `projection`, `status`, `confidence_tier`) — `outcome`, `language`, `lane` already in `AllowedLabels`.

## Shared Patterns

### Per-Workspace Mutex (overlay write serialization)

**Source:** `internal/semantic/store/overlay.go:79-141` + `:147-159` (`overlayLockFor`)
**Apply to:** `Engine.ApplyRepair`, `RankScheduler` score writes, `Resolver` two-phase comment merge

```go
// Re-use the SAME mutex BeginOverlayTx acquires — no second mutex map.
mu := s.overlayLockFor(repoID)
mu.Lock()
defer mu.Unlock()
// ... bump graph_version + write score rows in one tx ...
```

**Hard invariant** (Phase 60 D-04 + Phase 62 D-06): bump + score-write happens UNDER the same mutex that protects `current_epoch`. Re-using avoids a second lock-ordering surface.

### Bounded-Label Metrics with Drop-on-Unknown

**Source:** `internal/obs/metrics.go:388-413` (RenameStrategyInc, LSPoolLookup, RepoMapLookup, SessionLifecycleInc)
**Apply to:** All 5 new Phase 62 metrics

```go
func (m *Metrics) <NewMetric>Inc(labelA, labelB string) {
    if _, ok := allowedA[labelA]; !ok { return } // drop unknown → bounded cardinality
    if _, ok := allowedB[labelB]; !ok { return }
    m.<NewMetricVec>.WithLabelValues(labelA, labelB).Inc()
}
```

**Hard invariant:** every label value passes through a closed-enum allowlist BEFORE reaching `WithLabelValues` — no string interpolation into label values (T-53-01 mitigation).

### Errgroup Goroutine Lifecycle

**Source:** `internal/daemon/daemon.go:645-659` + `internal/semantic/lspenrich/manager.go:236-286`
**Apply to:** `RankScheduler.Run` (one per workspace)

```go
// Manager pattern (manager.go:236-286)
func (m *Manager) Run(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    m.cancelMu.Lock(); m.cancel = cancel; m.cancelMu.Unlock()
    g, gctx := errgroup.WithContext(ctx)
    g.Go(func() error { return w.RunN(gctx, n) })
    err := g.Wait()
    m.releaseAll() // cleanup on Run exit
    return err
}
```

**Hard invariant** (Phase 60 D-08): closed-channel start/stop, structured slog, `ctx.Err()` on cancel, deferred cleanup (release leases / stop timers).

### Narrow Tx Seam (interface-driven testability)

**Source:** `internal/semantic/lspenrich/cascade.go:79-89` (CascadeTx interface)
**Apply to:** New `Engine.ApplyRepair` (declare a narrow `RepairTx` subset of `*store.OverlayTx`)

```go
// cascade.go:79-89 template
type CascadeTx interface {
    UpsertSymbols(ctx context.Context, path string, syms []Symbol) error
    // ... only the methods cascade actually calls ...
    Commit() error
    Rollback() error
    Epoch() uint64
}
```

**Hard invariant:** the interface is INTENTIONALLY narrower than `*store.OverlayTx` — the production adapter wraps `*store.OverlayTx`, the unit tests use a recording fake. Phase 62 mirrors this for `RankScheduler` and `Resolver` so unit tests don't need DuckDB.

### Per-Language Layout

**Source:** `internal/semantic/extract/{golang,typescript,python}/`
**Apply to:** `internal/semantic/types/{golang,typescript,python,java,php,ruby}/`

```
extract/<lang>/
├── provider.go    # Provider struct + NewProvider
├── queries.scm    # tree-sitter queries (Phase 59)
├── provider_test.go
├── smoke_test.go
├── stable_id_test.go
└── testdata/      # fixture trees
```

Phase 62 mirrors:
```
types/<lang>/
├── resolver.go    # Resolver struct + NewResolver (full ladder)
├── scope.go       # package-detection helper (per D-13)
├── comment.go     # hand-rolled doc-comment parser (per D-12)
├── resolver_test.go
└── testdata/      # ladder-coverage fixtures
```

For `{java,php,ruby}/`: just `stub.go` (+ `stub_test.go`). No `scope.go` / `comment.go`.

**Hard invariant** (D-11): shared core has zero language-specific code; language quirks live entirely under `<lang>/`.

### Sort-Before-Iterate Determinism (engine + frontier + cluster)

**Source:** D-03 + RESEARCH Pattern 1 + Pitfall 1
**Apply to:** `internal/graph/pagerank.go`, `internal/semantic/graph/frontier.go`, `internal/semantic/cluster/weak.go`

Every place a node-keyed map is iterated in a way whose order matters for output:
1. Build `sorted := slices.Sorted(maps.Keys(m))` (Go 1.23+) OR `sort.Slice` over a copy.
2. Iterate `sorted` everywhere.
3. Inner edge sums (`for dst, w := range edges[src]`) stay map-ranged because addition is commutative.

A `sortedNodeIDs(m map[NodeID]struct{}) []NodeID` helper in `internal/semantic/graph/util.go` is recommended (RESEARCH Pitfall 1) so frontier + scheduler + cluster all share one canonical sort.

### Empty-List No-Op (overlay write helpers)

**Source:** `internal/semantic/store/overlay.go:264-283` (MarkSymbolsDeleted) and `:376-395` (MarkEdgesDeleted)
**Apply to:** `UpsertGraphScores`, `UpsertClusters`, `UpsertClusterMembers`, `UpsertEdgesWithMerge`, `WriteInvalidations`

```go
if len(rows) == 0 { return nil } // saves a round-trip
```

**Hard invariant:** every batch helper accepts an empty list and returns nil without touching the tx. Test fixtures rely on this.

## No Analog Found

Files with no close codebase analog (planner uses RESEARCH.md / SPEC-DRAFT.md sections directly):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/semantic/types/chain.go` | utility (algorithm) | transform | SPEC §38.4 access-chain walker is novel; no existing tree-walker in the codebase has the same access-chain shape (`a.b.c.d()`). |
| `internal/semantic/types/fixpoint.go` | utility (algorithm) | transform | SPEC §38.5 bounded fixpoint with `state.Hash()` change detection is novel. |
| `internal/semantic/types/ladder.go` | utility (constants) | transform | SPEC §38.2 7-tier ladder constants — pure data. |
| `internal/semantic/types/<lang>/scope.go` | utility | transform | Pure `filepath.Stat` + path walking; per D-13 no I/O beyond Stat. No existing analog because Phase 59 extractors don't need package scope. |
| `internal/semantic/types/<lang>/comment.go` | utility (parser) | transform | Hand-rolled per-language regex parsers (D-12 forbids new third-party deps). No existing comment-parser in the codebase. |
| `internal/semantic/cluster/weak.go` | utility (algorithm) | transform | SPEC §19.2 weak-component BFS. The codebase has no graph-clustering algorithm today. |

**For these files, the planner should reference:**
- SPEC-DRAFT.md §38.4 (access-chain), §38.5 (fixpoint), §38.2 (ladder), §38.6 (comments), §19.2 (weak components).
- RESEARCH.md Code Examples 1-4 for shape templates.
- RESEARCH.md Anti-Patterns to avoid (no tree-sitter in resolvers, no goroutines in `internal/graph`, no `graph_version` bump outside `ApplyRepair`).

## Metadata

**Analog search scope:**
- `internal/repomap/` (PageRank algorithm, FileGraph, test vectors)
- `internal/semantic/store/` (overlay tx, mutex, schema)
- `internal/semantic/live/coalescer/` (goroutine lifecycle, debounce timers)
- `internal/semantic/live/handler/` (post-commit hook surface)
- `internal/semantic/lspenrich/` (manager bootstrap, cascade tx interface, status tracker, metrics)
- `internal/semantic/extract/{golang,python,typescript}/` (per-language layout)
- `internal/daemon/daemon.go` (setter wiring + errgroup)
- `internal/config/defaults.go` (koanf precedence + float/slice gotchas)
- `internal/obs/metrics.go` (bounded-label registration + drop-on-unknown helpers)

**Files scanned:** ~25 (full + partial reads)
**Pattern extraction date:** 2026-05-06
**Skill / project rules consulted:** `CLAUDE.md` (Architecture, Constraints, SMTC-First Tool Routing, GSD Workflow), no `.claude/skills/` or `.agents/skills/` present at repo root.
