# Phase 61: LSP Enrichment Worker — Pattern Map

**Mapped:** 2026-05-05
**Files analyzed:** 17 (12 NEW, 5 MODIFIED)
**Analogs found:** 17 / 17 (every new file has an in-tree analog)

This pattern map is the single source of "where to copy from" for the
Phase 61 planner. Every excerpt below is anchored at `file:line` so the
planner can paste a real reference into PLAN tasks.

---

## File Classification

### NEW files

| File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/semantic/lspenrich/acquirer.go` | interface (kernel/semantic seam) | request-response | `internal/kernel/notifier.go` (EditNotifier) | exact |
| `internal/semantic/lspenrich/queue.go` (or extend `live/lspqueue/`) | utility (typed channel) | event-driven | `internal/semantic/live/lspqueue/queue.go` | exact |
| `internal/semantic/lspenrich/worker.go` | service (goroutine drainer) | event-driven | `internal/semantic/live/coalescer/coalescer.go` | exact |
| `internal/semantic/lspenrich/cascade.go` | service (LSP call orchestration) | request-response | `internal/repomap/lsp_enrich.go` (`enrichRepoMapFromLSP` in `daemon.go:402`) | role-match |
| `internal/semantic/lspenrich/budget.go` | utility (value type) | transform | `internal/semantic/config.go` `LSPEnrichmentConfig` (lines 161-172) | role-match |
| `internal/semantic/lspenrich/metrics.go` | utility (closed-enum helpers) | transform | `internal/kernel/lspool/metrics.go` + `internal/obs/metrics.go:170-176, 313-320` | exact |
| `internal/semantic/lspenrich/status.go` | service (read-only accessor for D-09) | request-response | `internal/kernel/lspool/health.go` (HealthReport snapshot) | role-match |
| `internal/semantic/lspenrich/manager.go` | service (per-workspace start/stop wrapper) | event-driven | `internal/semantic/live/scanner/manager.go` style (mirrored by `liveWatcherManager` interface in `live_wiring.go:38-41`) | role-match |
| `internal/semantic/lspenrich/worker_test.go` | test | event-driven | `internal/semantic/live/coalescer/coalescer_test.go` | exact |
| `internal/semantic/lspenrich/queue_test.go` | test | event-driven | `internal/semantic/live/lspqueue/queue_test.go` | exact |
| `internal/semantic/lspenrich/stress_test.go` (build tag `stress`) | test | event-driven | `internal/lint/nokernel2semantic/realtree_integration_test.go:1` (build-tag header pattern) + `internal/semantic/live/watcher/editor_fixtures_test.go:1` | role-match |
| `internal/semantic/lspenrich/testdata/{stress,cascade}/...` | fixture | n/a | `internal/kernel/lspool/testdata/` Go/Java/Rust trees | role-match |

### MODIFIED files

| File | Lines that change | Reason |
|---|---|---|
| `internal/semantic/live/handler/handler.go` | `144-146` (single Enqueue site) + new `markBulkPending` helper + lane selection switch | D-01 lane choice; D-05 bulk suppression |
| `internal/kernel/lspool/pool.go` | additive: new `lastForegroundLease` field on `Pool` (line 43+), new `ForegroundBusy(wsKey)` method, two-line edit inside `AcquireLease` (line 119-165) to stamp the map when sessionID does NOT start with `"lsp-enrichment:"` | D-04 ForegroundBusy implementation |
| `internal/config/defaults.go` | additive: 2 new keys after line 106 (`lsp_enrichment.max_concurrent_workers`, `lsp_enrichment.yield_check_window_ms`) | D-02, D-04 config keys |
| `internal/semantic/config.go` | additive: 2 new fields on `LSPEnrichmentConfig` (lines 163-172) — `MaxConcurrentWorkers int` and `YieldCheckWindowMs int` | mirror config struct |
| `internal/daemon/live_wiring.go` | additive: extend `buildLiveBundle` (lines 125-252) with worker construction + manager start/stop hooks; extend `liveBundle` struct (line 46-51) with `enrichMgr` field | D-09 daemon bootstrap wiring |
| `internal/semantic/store/overlay.go` | additive: new `OverlayTx.MarkFileSemanticPending(ctx, repoID, path, reason)` method after `MarkFileDeleted` (line 232) | D-05 bulk-update API |
| `internal/semantic/store/migrations.go` | doc-comment only: extend lines 370-379 closed-enum doc with three new values (`"preempted"`, `"bulk_update_pending"`, `"lsp_unavailable"`) | D-05/D-06 closed-enum doc |
| `internal/config/loader_test.go` | additive: new `TestLoad_LSPEnrichmentDefaults` after `TestLoad_LiveUpdatesDefaults` (line 651) | per-feature defaults pin |
| `internal/obs/metrics.go` + `metrics_labels_test.go` | additive: 5 new vectors + helpers + carve-out entries | metrics |

---

## Pattern Assignments

### `internal/semantic/lspenrich/acquirer.go` — narrow interface (kernel/semantic seam)

**Analog:** `internal/kernel/notifier.go` (the Phase 60 EditNotifier template
named verbatim in CONTEXT D-04, lines 232-243).

**Imports** (`internal/kernel/notifier.go:14-20`):

```go
package kernel  // <- new file is `package lspenrich`

import (
	"context"

	"github.com/agenthands/helix/internal/workspace"
)
```

**Interface declaration shape** (`notifier.go:22-32`):

```go
// EditNotifier is the interface a downstream consumer (Phase 60
// semantic/live service) implements ...
type EditNotifier interface {
	OnEdit(ctx context.Context, workspaceID workspace.WorkspaceKey, paths []string) error
}
```

**Phase 61 mirror (per CONTEXT D-04):**

```go
package lspenrich

import (
	"context"

	"github.com/agenthands/helix/internal/kernel/lspool" // types only — NOT internal/kernel
	"github.com/agenthands/helix/internal/workspace"
)

type LeaseAcquirer interface {
	AcquireLease(ctx context.Context, sessionID string, wsKey workspace.WorkspaceKey, dirty bool) (*lspool.WorkerLease, error)
	ForegroundBusy(wsKey workspace.WorkspaceKey) bool
}
```

**Critical rule:** the new package imports `internal/kernel/lspool` types
ONLY. Never `internal/kernel` (that is the kernel-level `Kernel` struct).
The existing `internal/lint/nokernel2semantic/analyzer.go` enforces the
**reverse** direction (kernel→semantic forbidden). The forward direction
acceptance — `internal/semantic/lspenrich/` does NOT import
`internal/kernel` — is upheld by code review + the package boundary
(import only `internal/kernel/lspool` and `internal/workspace`, never the
parent). If a sibling `nosemantic2kernel` analyzer is desired the planner
should treat that as out of scope (CONTEXT 61 says "verified by the
existing analyzer" — the existing analyzer fires only on kernel-side
imports of semantic; the semantic-side rule is upheld by the absence of
any `import "github.com/agenthands/helix/internal/kernel"` line in
`internal/semantic/lspenrich/*.go`, which a planner check on tree shape
makes mechanical).

---

### `internal/semantic/lspenrich/queue.go` — 2-lane priority queue

**Analog:** `internal/semantic/live/lspqueue/queue.go` (entire file, 58 lines).

**Existing single-channel queue** (`lspqueue/queue.go:23-58`):

```go
type Queue struct {
	ch chan RevalidateFileJob
}

func New(buffer int) *Queue {
	if buffer <= 0 {
		buffer = 1024
	}
	return &Queue{ch: make(chan RevalidateFileJob, buffer)}
}

func (q *Queue) Enqueue(job RevalidateFileJob) bool {
	select {
	case q.ch <- job:
		return true
	default:
		return false
	}
}

func (q *Queue) Channel() <-chan RevalidateFileJob { return q.ch }
func (q *Queue) Len() int                          { return len(q.ch) }
```

**Phase 61 mirror — 2-lane wrapper** (CONTEXT D-01 select snippet, lines 158-172):

```go
type Lane string

const (
	LaneHigh       Lane = "high"
	LaneBackground Lane = "background"
)

type LaneQueue struct {
	high       *Queue
	background *Queue
}

func NewLaneQueue(highBuf, bgBuf int) *LaneQueue { ... }

// EnqueueLane is non-blocking; drops + bumps drop counter on full lane.
func (q *LaneQueue) EnqueueLane(lane Lane, job RevalidateFileJob) bool { ... }

// Drain blocks until ctx done or a job is read; strict priority (D-01):
//   - if high has a job, return it; otherwise
//   - select on (high, background, ctx.Done()).
func (q *LaneQueue) Drain(ctx context.Context) (RevalidateFileJob, Lane, error) { ... }
```

**Test analog** (`internal/semantic/live/lspqueue/queue_test.go:9-36`):

```go
func TestQueue_EnqueueDequeue(t *testing.T) {
	q := lspqueue.New(1)
	if !q.Enqueue(lspqueue.RevalidateFileJob{Path: "a"}) {
		t.Fatal("first enqueue should succeed")
	}
	if q.Enqueue(lspqueue.RevalidateFileJob{Path: "b"}) {
		t.Fatal("second enqueue on cap-1 buffer should drop")
	}
	got := <-q.Channel()
	...
}
```

Phase 61 acceptance #3 (CONTEXT line 432-435) specifies the test:
"enqueue 100 background jobs followed by 1 high job and assert the high
job is drained before any background job after the first in-flight one
completes."

---

### `internal/semantic/lspenrich/worker.go` — goroutine lifecycle + drain loop

**Analog:** `internal/semantic/live/coalescer/coalescer.go` (cited
explicitly by CONTEXT lines 670-672 as the start/stop discipline template).

**Goroutine struct + Run pattern** (`coalescer.go:80-171`):

```go
type Coalescer struct {
	workspaceID workspace.WorkspaceKey
	cfg         Config
	handler     EventHandler
	logger      Logger
	metrics     MetricsSink
	in          chan live.SourceChangeEvent
	drops       atomic.Uint64
	mu          sync.Mutex
	pending     map[string]live.SourceChangeEvent
	timer       *time.Timer
	maxTimer    *time.Timer
}

// Run drains the input channel until ctx is cancelled.  Single
// goroutine per workspace; serializes accept + flush.
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
```

**Worker mirror (Phase 61):** the `Run` loop pulls a `RevalidateFileJob` +
`Lane` from `LaneQueue.Drain(ctx)`, calls `cascade.Run(ctx, job, budget)`
synchronously, commits via `OverlayTx`, and increments the
outcome-classified metric. Same `<-ctx.Done() / <-channel` skeleton; no
debounce timer (cascade is run-to-completion modulo voluntary yield).

**Voluntary yield between cascade steps** (per CONTEXT D-03):

```go
for _, step := range cascadeSteps {
	if w.acquirer.ForegroundBusy(wsKey) {
		// commit partial facts; mark partial_reason="preempted"; return.
		return outcomePreempted
	}
	if !budget.HasRemaining(step) {
		// mark partial_reason="budget exhausted"; return.
		return outcomeBudgetExhausted
	}
	if err := step.Run(ctx, lease); err != nil { ... }
}
```

**MetricsSink + noopMetrics nil-safety pattern** (`coalescer.go:32-34, 247-250`):

```go
type MetricsSink interface {
	SemanticLiveUpdatesInc(kind, outcome string)
}
// ... in New():
metrics := cfg.Metrics
if metrics == nil { metrics = noopMetrics{} }
// ...
type noopMetrics struct{}
func (noopMetrics) SemanticLiveUpdatesInc(string, string) {}
```

Phase 61 ships an analogous `MetricsSink` interface + `noopMetrics` for
the 5 Phase 61 metrics so unit tests construct the worker without an
`*obs.Metrics`.

---

### `internal/semantic/lspenrich/cascade.go` — §14.4 LSP cascade

**Analog:** the existing `enrichRepoMapFromLSP` callback wired in
`internal/daemon/daemon.go:402-411`. The structure is "open lease, fan out
LSP calls, write to a downstream store" — exactly the Phase 61 cascade
shape, just for repomap rather than overlay.

**Daemon-side wiring template** (`daemon.go:399-411`):

```go
// 12b. Wire repomap skill LSP enrichment callback (RMAP-08).
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
	tagCache := rs.Cache()
	rs.SetEnrichFn(func(g *repomapPkg.FileGraph) {
		wsKey := activeWSKey
		if wsKey.RepoRoot == "" { return }
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		enrichRepoMapFromLSP(ctx, k, wsKey, g, tagCache, logger)
	})
}
```

**Lease acquisition pattern, with sessionID prefix (CONTEXT
"Claude's Discretion" lines 472-481):**

```go
sessionID := fmt.Sprintf("lsp-enrichment:%s:%s", wsKey.RepoRoot, wsKey.Language)
lease, err := acquirer.AcquireLease(ctx, sessionID, wsKey, false /* clean */)
if err != nil {
	if errors.Is(err, lspool.ErrCircuitOpen) || errors.Is(err, lspool.ErrMaxWorkersReached) {
		// outcome=dropped; no per-file error log
		return
	}
	// readiness-gate timeout / spawn failure → partial_reason="lsp_unavailable"
}
defer acquirer.ReleaseLease(sessionID) // long-lived; release on workspace deactivate
```

The cascade orchestrates 6 LSP calls (CONTEXT D-07 lines 350-374) inside
this lease, between each call invokes `acquirer.ForegroundBusy(wsKey)`,
and writes facts via `OverlayTx`.

---

### `internal/semantic/lspenrich/budget.go` — value type for §14.2

**Analog:** `internal/semantic/config.go:161-172` `LSPEnrichmentConfig` is
the wire-format struct. Phase 61 needs a runtime `Budget` value with
remaining-symbol / remaining-reference / remaining-time counters.

```go
type Budget struct {
	deadline       time.Time
	totalDeadline  time.Time
	symbolsLeft    int
	refsPerSymbol  int
	refsPerFile    int
	callDepth      int
	typeDepth      int
}

func NewBudget(now time.Time, cfg semantic.LSPEnrichmentConfig) Budget { ... }
func (b *Budget) HasRemaining() bool { return time.Now().Before(b.deadline) && b.symbolsLeft > 0 }
func (b *Budget) ConsumeSymbol() { b.symbolsLeft-- }
```

The `LSPEnrichmentConfig` struct fields (already declared) are the
canonical names — copy them verbatim.

---

### `internal/semantic/lspenrich/metrics.go` — bounded-label closed-enum metrics

**Analog 1 (interface + drop-on-unknown):** `internal/kernel/lspool/metrics.go`
(the `MetricsSink` interface that lspool exposes; obs implements it).

**Analog 2 (registration + helpers):** `internal/obs/metrics.go:170-176`
(LSPoolLookups vector) and `internal/obs/metrics.go:313-320` (drop-unknown
LSPoolLookup helper).

**Vector registration** (`obs/metrics.go:170-176`):

```go
LSPoolLookups: prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "helix_lspool_lookups_total",
		Help: "lspool AcquireLease lookups by result (hit=shared warm worker, miss=spawn or refusal). Phase 53 D-01.",
	},
	[]string{"language", "result"},
),
```

**Drop-on-unknown helper** (`obs/metrics.go:313-320`):

```go
// LSPoolLookup increments helix_lspool_lookups_total. result ∈ {"hit","miss"};
// any other value is dropped (Phase 53 D-04 closed enum, T-53-01 mitigation).
func (m *Metrics) LSPoolLookup(language, result string) {
	if result != "hit" && result != "miss" {
		return
	}
	m.LSPoolLookups.WithLabelValues(language, result).Inc()
}
```

**Phase 61 surface (per CONTEXT lines 495-510):**

| Metric | Labels | Closed enum |
|---|---|---|
| `helix_semantic_lsp_enrichment_total` | language, outcome | applied, partial_budget, partial_preempted, partial_lsp_unavailable, dropped |
| `helix_semantic_lsp_enrichment_duration_seconds` | language | (histogram) |
| `helix_semantic_lsp_enrichment_errors_total` | language, outcome | timeout, ls_crash, circuit_open, readiness_timeout, other |
| `helix_semantic_lsp_enrichment_lane_depth` | lane | high, background |
| `helix_semantic_lsp_enrichment_bulk_suppressed_total` | (none) | — |

**Carve-out entry** (mirror `metrics_labels_test.go:65`):

```go
"helix_semantic_lsp_enrichment_total":             {"outcome": true},
"helix_semantic_lsp_enrichment_errors_total":      {"outcome": true},
"helix_semantic_lsp_enrichment_lane_depth":        {"lane": true},
```

`language` and `lane` need to be members of `AllowedLabels` already; if
`lane` is not, add it to `AllowedLabels` in `obs/metrics.go:28` (CONTEXT
allows additive carve-out).

---

### `internal/semantic/lspenrich/status.go` — D-09 read-only accessor

**Analog:** `internal/kernel/lspool/health.go` (the `HealthReport` snapshot
exposed via `Kernel.HealthStatus()` in `kernel.go:132`). Same shape: a
struct of counters + per-language map, returned by value (snapshot, not
live pointer).

**Status struct (per CONTEXT D-09 lines 514-518):**

```go
type Status struct {
	LaneDepths            map[Lane]int
	InFlight              int
	FilesEnriched         uint64
	FilesDropped          uint64
	FilesPreempted        uint64
	FilesPending          uint64
	LastErrorPerLanguage  map[string]string
}

func (m *Manager) Status() Status { ... } // RLock-only snapshot
```

Phase 65 wires this into `get_health` — Phase 61 ships the accessor only.

---

### Modified: `internal/semantic/live/handler/handler.go:144-146`

**Existing single Enqueue site** (handler.go:128-148):

```go
func (h *Handler) UpdateChangedFile(ctx context.Context, repoID semantic.RepoID, path string) error {
	hash, err := h.Hasher(path)
	if err != nil { return fmt.Errorf("UpdateChangedFile: hash %s: %w", path, err) }
	tx, err := h.Store.BeginOverlayTx(ctx, string(repoID))
	if err != nil { return fmt.Errorf("UpdateChangedFile: begin tx: %w", err) }
	if err := tx.UpsertOverlayFile(ctx, path, hash); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("UpdateChangedFile: upsert: %w", err)
	}
	if err := tx.Commit(); err != nil { return err }
	if h.LSPQueue != nil {
		_ = h.LSPQueue.Enqueue(lspqueue.RevalidateFileJob{RepoID: repoID, Path: path})
	}
	return nil
}
```

**Phase 61 modification (CONTEXT D-01 + D-05, lines 286-301):** the
producer is the single decision point for lane choice AND bulk
suppression. Two structural changes:

1. The handler's `LSPQueue` field changes type from
   `LSPRevalidationEnqueuer` (single-channel) to a 2-lane interface that
   exposes `EnqueueLane(Lane, RevalidateFileJob) bool`. Backward-compatible
   because the field is nil-safe (Phase 60 CR-04).

2. The `Dispatch` switch (line 108-121) gains a `ChangeBulkUpdate` branch
   that calls a new `markBulkPending` helper instead of falling through
   to `HandleBulkUpdate`'s scheduler delegation. The `LSPQueue.Enqueue`
   call moves from `UpdateChangedFile` (line 144-146) to a new
   `selectLane(ev.Kind)` callsite per the CONTEXT snippet:

```go
// in UpdateChangedFile, after Commit succeeds:
if h.LSPQueue != nil {
	lane := selectLane(ev.Kind) // Kind=ChangeHelixEdit → high; others → background
	_ = h.LSPQueue.EnqueueLane(lane, lspqueue.RevalidateFileJob{RepoID: repoID, Path: path})
}
```

3. New helper `markBulkPending(ctx, repoID, paths, reason)` opens an
   overlay tx and calls the new `tx.MarkFileSemanticPending(...)` API on
   every path (CONTEXT D-05). NO enqueue.

---

### Modified: `internal/kernel/lspool/pool.go` — ForegroundBusy + lastForegroundLease

**Analog (existing AcquireLease mutex region):** `pool.go:119-165`.

```go
func (p *Pool) AcquireLease(ctx context.Context, sessionID string, wsKey workspace.WorkspaceKey, dirty bool) (*WorkerLease, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// ... existing share-until-dirty + circuit-breaker checks ...
}
```

**Phase 61 additive change (CONTEXT D-04 lines 248-260):**

1. Add field on `Pool` struct (after line 60):
   ```go
   lastForegroundLease map[workspace.WorkspaceKey]time.Time
   yieldCheckWindow    time.Duration // populated from cfg
   ```

2. Inside `AcquireLease` (line 122 after `defer p.mu.Unlock()`), stamp
   the map on every non-enrichment lease:
   ```go
   if !strings.HasPrefix(sessionID, "lsp-enrichment:") {
       if p.lastForegroundLease == nil {
           p.lastForegroundLease = make(map[workspace.WorkspaceKey]time.Time)
       }
       p.lastForegroundLease[wsKey] = time.Now()
   }
   ```

3. New method:
   ```go
   func (p *Pool) ForegroundBusy(wsKey workspace.WorkspaceKey) bool {
       p.mu.RLock()
       defer p.mu.RUnlock()
       last, ok := p.lastForegroundLease[wsKey]
       if !ok { return false }
       return time.Since(last) < p.yieldCheckWindow
   }
   ```

**Critical invariants (CONTEXT D-04 lines 268-277):** read-lock only
(non-blocking — `ForegroundBusy` is called between every cascade step);
window is per-`wsKey` not global; the `lsp-enrichment:` prefix is the
sole discriminator.

**Acceptance #5 test (CONTEXT line 436-438):** unit test uses a fake
clock to assert `ForegroundBusy` returns `true` within
`yield_check_window_ms` after a non-enrichment `AcquireLease` and `false`
after the window elapses.

---

### Modified: `internal/config/defaults.go` — 2 new keys

**Existing block** (defaults.go:98-106):

```go
"semantic_index.lsp_enrichment.enabled":                   true,   // SPEC §25
"semantic_index.lsp_enrichment.timeout_per_file":          "5s",   // SPEC §25
"semantic_index.lsp_enrichment.timeout_total":             "120s", // SPEC §25
"semantic_index.lsp_enrichment.max_symbols_per_file":      200,    // SPEC §25
"semantic_index.lsp_enrichment.max_references_per_symbol": 1000,   // SPEC §25
"semantic_index.lsp_enrichment.max_references_per_file":   5000,   // SPEC §25
"semantic_index.lsp_enrichment.max_call_hierarchy_depth":  2,      // SPEC §25
"semantic_index.lsp_enrichment.max_type_hierarchy_depth":  2,      // SPEC §25
```

**Phase 61 addition** (insert after line 106):

```go
// Phase 61 D-02 + D-04: enrichment worker concurrency cap and
// foreground-yield window.  Closed-enum doc comment in
// internal/semantic/store/migrations.go (partial_reason) extended.
"semantic_index.lsp_enrichment.max_concurrent_workers": 1,    // P61 D-02
"semantic_index.lsp_enrichment.yield_check_window_ms":  200,  // P61 D-04
```

**Per-feature defaults test** (mirror `loader_test.go:636-650`
TestLoad_LiveUpdatesDefaults):

```go
// TestLoad_LSPEnrichmentDefaults pins the two Phase 61 D-02 + D-04
// keys (max_concurrent_workers, yield_check_window_ms) at their
// published defaults. Mirrors TestLoad_LiveUpdatesDefaults shape.
func TestLoad_LSPEnrichmentDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/global.yml", "", nil)
	if err != nil { t.Fatalf("Load: %v", err) }
	if cfg.SemanticIndex.LSPEnrichment.MaxConcurrentWorkers != 1 {
		t.Errorf("default MaxConcurrentWorkers = %d, want 1",
			cfg.SemanticIndex.LSPEnrichment.MaxConcurrentWorkers)
	}
	if cfg.SemanticIndex.LSPEnrichment.YieldCheckWindowMs != 200 {
		t.Errorf("default YieldCheckWindowMs = %d, want 200",
			cfg.SemanticIndex.LSPEnrichment.YieldCheckWindowMs)
	}
}
```

---

### Modified: `internal/semantic/config.go:161-172` — 2 new fields

**Existing struct:**

```go
type LSPEnrichmentConfig struct {
	Enabled                bool   `koanf:"enabled"`
	TimeoutPerFile         string `koanf:"timeout_per_file"`           // default "5s"
	TimeoutTotal           string `koanf:"timeout_total"`              // default "120s"
	MaxSymbolsPerFile      int    `koanf:"max_symbols_per_file"`       // default 200
	MaxReferencesPerSymbol int    `koanf:"max_references_per_symbol"`  // default 1000
	MaxReferencesPerFile   int    `koanf:"max_references_per_file"`    // default 5000
	MaxCallHierarchyDepth  int    `koanf:"max_call_hierarchy_depth"`   // default 2
	MaxTypeHierarchyDepth  int    `koanf:"max_type_hierarchy_depth"`   // default 2
}
```

**Phase 61 addition (append, no field reorder):**

```go
	MaxConcurrentWorkers   int    `koanf:"max_concurrent_workers"`     // default 1 (P61 D-02)
	YieldCheckWindowMs     int    `koanf:"yield_check_window_ms"`      // default 200 (P61 D-04)
```

---

### Modified: `internal/daemon/live_wiring.go` — bootstrap extension

**Existing buildLiveBundle skeleton** (`live_wiring.go:125-252`). The
critical existing pattern is:

1. Construct components in dependency order.
2. Each component is nil-safe — failure to construct one does not abort.
3. The `liveBundle` struct holds them all (`live_wiring.go:46-51`).
4. `SetActivateCallback` (in `daemon.go:489+`) starts per-workspace
   lifecycle via the bundle.

**Phase 61 extension after step 6 (line 196 onward):**

```go
// 7. Construct the LSP enrichment worker (Phase 61).
//    Requires the LSPQueue (already in bundle.lspQueue) and a
//    LeaseAcquirer adapter around k.Pool().  Nil-safe per CR-04 pattern.
if cfg.LSPEnrichment.Enabled && cfg.LSPEnrichment.MaxConcurrentWorkers > 0 {
	enrichBudget := lspenrich.NewBudgetConfig(cfg.LSPEnrichment)
	acquirer := lspenrich.NewPoolAcquirer(k.Pool()) // adapter; *kernel.Kernel NOT used
	enrichMgr := lspenrich.NewManager(
		bundle.lspQueue,           // 2-lane queue from step 1
		acquirer,
		&storeOverlayWriter{store: store}, // reuse existing adapter
		enrichBudget,
		metrics,
		logger,
	)
	bundle.enrichMgr = enrichMgr
	logger.Info("lsp-enrichment worker constructed",
		"max_concurrent_workers", cfg.LSPEnrichment.MaxConcurrentWorkers,
		"yield_check_window_ms", cfg.LSPEnrichment.YieldCheckWindowMs)
}
```

**Per-workspace lifecycle hook** (extend `liveBundle.startWorkspace` in
`live_wiring.go:55-70`):

```go
if b.enrichMgr != nil {
	if err := b.enrichMgr.Start(ctx, ws); err != nil {
		logger.Warn("live: enrichment worker start", "ws", ws, "err", err)
	}
}
```

---

### Modified: `internal/semantic/store/overlay.go` — MarkFileSemanticPending

**Analog:** `OverlayTx.MarkFileDeleted` (`overlay.go:232-249`):

```go
func (t *OverlayTx) MarkFileDeleted(ctx context.Context, path string) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("MarkFileDeleted: nil tx")
	}
	_, err := t.tx.ExecContext(ctx, `
		INSERT INTO semantic_live_overlay_files (
			repo_id, path, file_id, content_hash, language, status, updated_at, write_epoch
		) VALUES (?, ?, 0, '', '', 'deleted', now(), ?)
		ON CONFLICT (repo_id, path) DO UPDATE SET
			status       = 'deleted',
			updated_at   = now(),
			write_epoch  = excluded.write_epoch
	`, t.repoID, path, t.epoch)
	if err != nil {
		return fmt.Errorf("MarkFileDeleted(%q, %q): %w", t.repoID, path, err)
	}
	return nil
}
```

**Phase 61 mirror — `MarkFileSemanticPending`** (CONTEXT D-05 line 305-307):
this is a per-file UPDATE on `semantic_files.partial_reason +
semantic_status` (NOT the overlay table — `bulk_update_pending` belongs
on the snapshot fact-store row). Schema is unchanged: `partial_reason
TEXT` already exists per `migrations.go:429, 436, 440`. Closed enum is
extended in the doc comment only (lines 370-379).

```go
// MarkFileSemanticPending stamps semantic_files.partial_reason for
// (repoID, path) with the given closed-enum reason. Does NOT change
// schema; the column accepts any TEXT. Reason MUST be one of:
//   - "preempted"            (cascade yielded mid-file)
//   - "bulk_update_pending"  (D-05; producer suppressed enqueue)
//   - "lsp_unavailable"      (readiness timeout / circuit open)
//   - "budget exhausted"     (existing pre-P61 value)
func (t *OverlayTx) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	// validate closed enum; UPDATE semantic_files SET partial=true,
	// partial_reason=?, updated_at=now() WHERE repo_id=? AND path=?
}
```

---

### Modified: `internal/semantic/store/migrations.go:370-379` — doc-comment only

**Existing comment block:**

```go
//	semantic_files: 6 columns
//	  ...
//	  partial_reason      TEXT  (nullable; closed enum from D-05)
//	  ...
//	semantic_symbols: 2 columns
//	  partial             BOOLEAN DEFAULT false
//	  partial_reason      TEXT
//	semantic_references: 2 columns
//	  partial             BOOLEAN DEFAULT false
//	  partial_reason      TEXT
```

**Phase 61 update — add a "Closed enum values" sub-block** (no schema
change, doc only):

```go
// Closed enum for partial_reason TEXT (extended in P61):
//   - "budget exhausted"      (P59 D-05; per-file timeout / max-symbols)
//   - "preempted"             (P61 D-03; cascade yielded between calls)
//   - "bulk_update_pending"   (P61 D-05; producer suppressed enqueue)
//   - "lsp_unavailable"       (P61 D-08; readiness timeout / circuit open)
// Phase 62 may add lower-confidence values; the column itself remains
// permissive TEXT — callers MUST validate against this enum.
```

---

## Shared Patterns

### Setter-style cross-package wiring

**Source:** `internal/kernel/notifier.go:34-45` (`Kernel.SetEditNotifier`),
`internal/daemon/daemon.go:402` (`rs.SetEnrichFn`),
`internal/daemon/daemon.go:489+` (`mcpServer.SetActivateCallback`).

**Shape:**
- The owning struct holds an `atomic.Value`-wrapped optional callback /
  notifier.
- `SetXxx(v)` replaces it (last write wins).
- Callers nil-check before invoking.

**Apply to:** Phase 61 daemon bootstrap. CONTEXT line 528-529 names
`SetEnrichmentWorker(worker)` as the suggested setter — but planner
should evaluate whether kernel needs a back-edge to the worker (CONTEXT
line 770-772 says "likely not required"). If only `*Pool.ForegroundBusy`
is needed, no kernel-level setter is required at all; the bootstrap
simply hands the worker the `acquirer` and starts its goroutine in the
errgroup.

### Nil-safe queue/worker fields (Phase 60 CR-04)

**Source:** `internal/semantic/live/handler/handler.go:74-77, 144-146`:

```go
type LSPRevalidationEnqueuer interface {
	Enqueue(job lspqueue.RevalidateFileJob) bool
}
// ...
if h.LSPQueue != nil {
	_ = h.LSPQueue.Enqueue(...)
}
```

**Apply to:** every Phase 61 component that the bootstrap might fail to
construct (worker manager, lane queue, status accessor). Daemon log
warns on degraded mode; semantic-enabled tests pass without it.

### Closed-enum drop-on-unknown helpers

**Source:** `internal/obs/metrics.go:313-352` (LSPoolLookup,
SessionLifecycleInc, EditOutcomeInc all guard with explicit equality
checks before `WithLabelValues(...).Inc()`).

**Apply to:** the 3 Phase 61 metrics with closed-enum labels
(`outcome` on enrichment_total + errors_total, `lane` on lane_depth).
Helper signatures live on `*obs.Metrics`; the lspenrich package consumes
them via a thin `MetricsSink` interface (mirror `lspool/metrics.go:10-31`).

### `//go:build` test isolation

**Source:**
- `internal/lint/nokernel2semantic/realtree_integration_test.go:1`
  → `//go:build integration`
- `internal/semantic/live/watcher/editor_fixtures_test.go:1`
  → `//go:build editor`

**Apply to:** Phase 61 stress test
(`internal/semantic/lspenrich/stress_test.go:1` →
`//go:build stress`). Acceptance #11 (CONTEXT line 461-466) explicitly
gates the test on the `-stress` build tag. CI does not run it; ops do
locally per the user's "Benchmarks are local-only — never on CI" rule
(see auto-memory).

### nil-safe MetricsSink with package-private noop

**Source:** `internal/semantic/live/coalescer/coalescer.go:32-34, 247-250`.

```go
type MetricsSink interface { SemanticLiveUpdatesInc(kind, outcome string) }
// in New: if metrics == nil { metrics = noopMetrics{} }
type noopMetrics struct{}
func (noopMetrics) SemanticLiveUpdatesInc(string, string) {}
```

**Apply to:** Phase 61 worker constructor. Tests inject a recording
mock; production wires `*obs.Metrics`.

### vet analyzer enforcement

**Source:** `internal/lint/nokernel2semantic/analyzer.go:25-57` is the
only existing import-direction analyzer. It enforces
`internal/kernel → internal/semantic` is forbidden. CONTEXT 61
acceptance #1 ("`internal/semantic/lspenrich/` does NOT import
`internal/kernel`") is verified by the absence of any
`"github.com/agenthands/helix/internal/kernel"` import in
`internal/semantic/lspenrich/*.go` (planner enforces via code review +
`go list -deps ./internal/semantic/lspenrich/...` against an allowed
prefix list).

If the planner decides a sibling `nosemantic2kernel` analyzer is
warranted, the template is `analyzer.go:25-57` with the prefixes
swapped:
```go
const checkedPkgPrefix    = "github.com/agenthands/helix/internal/semantic"
const forbiddenImportPrefix = "github.com/agenthands/helix/internal/kernel"
```
Then carve out `internal/kernel/lspool` and `internal/workspace` as
allowed imports inside the `Run` body. **CONTEXT does not require this
analyzer; treat as out of scope unless a plan specifically asks.**

---

## No Analog Found

None — every Phase 61 file has a strong in-tree analog. The closest
"lower-confidence" fit is `cascade.go` (no existing 6-step LSP cascade
orchestrator in tree), but `enrichRepoMapFromLSP` (called from
`daemon.go:402-411`) provides the lease-acquire-and-fanout shape, and
the LSP call signatures themselves come from
`internal/protocol/gen/` (LSP 3.17 generated types). The cascade
sequence + budget + voluntary-yield logic is **new** structural code,
but every individual building block (lease acquisition, JSON-RPC call,
tx commit, slog warn) has multiple in-tree references.

---

## Metadata

**Analog search scope:**
- `internal/semantic/{live,store,scheduler}/...`
- `internal/kernel/{lspool,fileops,edit}/...`
- `internal/daemon/{daemon,live_wiring}.go`
- `internal/config/{defaults,loader_test}.go`
- `internal/obs/metrics{,_labels_test}.go`
- `internal/lint/nokernel2semantic/`

**Files scanned:** 27 (read directly) + 4 directory listings.

**Pattern extraction date:** 2026-05-05.

**Phase 61 plan layout (CONTEXT-suggested, lines 522-531):**
- **P01:** LeaseAcquirer + ForegroundBusy + LaneQueue + handler producer
  rewiring + bulk suppression API. Modifies `pool.go`, `handler.go`,
  `lspqueue/queue.go` (or new `lspenrich/queue.go`), `overlay.go`,
  `migrations.go` doc.
- **P02:** Worker goroutine + cascade engine + budget + readiness gates +
  per-file overlay commit. New `worker.go`, `cascade.go`, `budget.go`.
- **P03:** Metrics + trace spans + Status accessor + per-language
  capability cache + daemon bootstrap. New `metrics.go`, `status.go`,
  `manager.go`; modifies `live_wiring.go`, `defaults.go`, `config.go`,
  `obs/metrics.go`.
- **P04:** ENRICH-05 stress test + Go/Java integration tests + REQ
  check-off. New `stress_test.go`, `testdata/{stress,cascade}/...`.
