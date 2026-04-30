# Phase 53: obs-metrics-gaps - Pattern Map

**Mapped:** 2026-04-30
**Files analyzed:** 22 (17 modified + 5 created — corrected from research C-1/C-2/C-3)
**Analogs found:** 22 / 22

All 22 files have a strong, in-repo analog. The Phase 11 (lspool sink + obs vectors) and Phase 47 (`renameStrategySink` package-level setter) scaffolding designs cover every shape Phase 53 needs except the HTTP transport session-id middleware (no precedent in this repo — documented as a new shape with the closest in-tree analog called out).

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/obs/metrics.go` (MODIFY) | observability config | event-driven counter/histogram emission | self (existing `RenameStrategy` block + helper methods) | exact (incremental extension) |
| `internal/obs/metrics_labels_test.go` (MODIFY) | test (lint) | static analysis on Gather() | self (existing `carveOuts` + `TestMetricsLabelsAllowlist`) | exact |
| `internal/obs/metrics_test.go` (MODIFY) | test | static — registered families | self (`TestMetrics_RegisteredFamilies` + `TestMetrics_NoopProviderReturnsUsableSink`) | exact |
| `internal/obs/obs.go` (MODIFY, optional) | observability config | provider construction | self (existing `Noop` + `Metrics()` accessor) | exact |
| `internal/kernel/lspool/metrics.go` (MODIFY) | sink interface | event-driven | self (existing `MetricsSink` + `NoopSink` + `EvictXxx` constants) | exact |
| `internal/kernel/lspool/metrics_test.go` (MODIFY) | test (recording sink) | event-driven | self (existing `recordingSink`) | exact |
| `internal/kernel/lspool/pool.go` (MODIFY) | service (worker pool) | event-driven (lease lifecycle) | self (existing eviction emission in `evictWorkerLocked`) | exact |
| `internal/repomap/cache.go` (MODIFY) | service (mtime cache) | request-response (cache lookup) | `internal/kernel/lspool/pool.go` AcquireLease (branch emission) | role-match (CRUD/cache → request-response) |
| `internal/skill/repomap/skill.go` (MODIFY) | dispatcher | request-response (timed wrap) | `internal/kernel/edit/replace.go` ReplaceBodyWithPlan (timing wrapper around an inner call) | partial (closest in-tree timed wrap) |
| `internal/repomap/render.go` (MODIFY, audit) | renderer | request-response (cache read) | `internal/skill/repomap/skill.go` (sibling caller of `GetOrExtract`) | exact |
| `internal/mcp/middleware.go` (MODIFY) | middleware (recorder seam) | event-driven (atomic.Pointer setter) | self (existing `renameStrategySink` block lines 18-44 + wiring at 75-86) | exact |
| `internal/kernel/edit/tools.go` (MODIFY) | controller (MCP tool handlers) | request-response | self (existing `mcp.RecordRenameStrategy(ctx, string(result.Strategy))` call at line 397) | exact |
| `internal/kernel/edit/replace.go` (MODIFY) | service (body surgery) | request-response | self (existing `*FuzzyMatchInfo` return shape carrying Strategy + Score) | exact |
| `internal/kernel/fileops/tools.go` (MODIFY) | controller (MCP tool handlers) | request-response | `internal/kernel/edit/tools.go` `registerRenameSymbol` | exact |
| `internal/daemon/daemon.go` (MODIFY) | bootstrap + forwarder handler | event-driven (stream lifecycle) | self (existing `forwarderServiceHandler.StreamMCP` + post-init sink wiring at lines 290-323) | exact |
| `internal/daemon/wiring_test.go` (MODIFY) | test (compile-time assertion) | static | self (existing `var _ lspool.MetricsSink = (*obs.Metrics)(nil)`) | exact |
| `USAGE.md` (MODIFY) | docs | n/a | self (existing Prometheus Metrics table at lines 644-677) | exact |
| `.planning/ROADMAP.md` (MODIFY) | docs | n/a | self (find/replace `serena_*` → `helix_*`) | exact |
| `internal/repomap/metrics.go` (CREATE) | sink interface | event-driven | `internal/kernel/lspool/metrics.go` | exact (template) |
| `internal/repomap/metrics_test.go` (CREATE) | test (recording sink + interface assertion) | event-driven | `internal/kernel/lspool/metrics_test.go` | exact |
| `internal/kernel/edit/tools_test.go` (CREATE) | test (handler outcome) | request-response | `internal/mcp/middleware_test.go` (recorder roundtrip) + `internal/kernel/lspool/metrics_test.go` (recordingSink) | role-match |
| `internal/kernel/fileops/tools_test.go` (CREATE) | test (handler outcome) | request-response | same as above | role-match |
| `internal/daemon/forwarder_test.go` or extension (CREATE) | test (handler) | event-driven | `internal/kernel/lspool/metrics_test.go` (recordingSink + lifecycle assertion) | partial |
| `internal/daemon/http_session_middleware.go` (CREATE) | http.Handler wrapper | request-response | **NO IN-REPO PRECEDENT** — closest is `internal/mcp/middleware.go::TelemetryMiddleware` (different layer: MCP middleware, not http.Handler) | absent — see "No Analog Found" below |

## Pattern Assignments

### `internal/obs/metrics.go` (observability config, event-driven emission)

**Analog:** self — extend the existing `RenameStrategy` block (lines 45-50, 108-116, 167-176).

**Imports pattern** (lines 22-23 — keep as is; no new imports needed):
```go
import "github.com/prometheus/client_golang/prometheus"
import "github.com/prometheus/client_golang/prometheus/collectors"
```

**Vector field declaration pattern** (lines 45-50, copy the comment+field shape per new vector):
```go
// Phase 47 D-07: rename_symbol dispatcher strategy counter.
// Closed-enum label "strategy" ∈ {"lsp-native", "rust-client-side"};
// enforced at emission sites (see *Metrics.RenameStrategyInc and
// internal/mcp.RecordRenameStrategy). The "strategy" label is carved
// out of AllowedLabels in metrics_labels_test.go for this family only.
RenameStrategy *prometheus.CounterVec
```

**Vector construction pattern** (lines 108-116, copy verbatim per new vector inside the `&Metrics{...}` literal):
```go
RenameStrategy: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "helix_rename_strategy_total",
        Help: "rename_symbol successes by strategy (lsp-native | rust-client-side).",
    },
    // "strategy" is a closed-enum dimension carved out of AllowedLabels
    // for this family only (Phase 47 D-07). Enforced at emission sites.
    []string{"strategy"},
),
```

For `helix_repomap_extract_duration_seconds`, mirror the `ToolDuration` HistogramVec pattern at lines 69-76 with custom `Buckets` per D-06:
```go
ToolDuration: prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "helix_tool_duration_seconds",
        Help:    "MCP tool call latency in seconds (RED: duration).",
        Buckets: prometheus.DefBuckets, // D-01
    },
    []string{"tool_name", "profile", "mode", "language"},
),
```

**MustRegister pattern** (lines 119-129; add new vectors to the existing `reg.MustRegister(...)` call):
```go
reg.MustRegister(
    m.ToolCalls,
    m.ToolDuration,
    m.LSPoolWorkers,
    m.LSPoolEvictions,
    m.LSPoolCircuitState,
    m.LSPoolRestarts,
    m.RenameStrategy,
    collectors.NewGoCollector(),
    collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
)
```

**Helper method + closed-enum drop pattern** (lines 167-176 — the canonical "drop unknown values" discipline; mirror per new helper):
```go
// RenameStrategyInc increments the helix_rename_strategy_total counter.
// strategy MUST be one of the closed-enum values {"lsp-native","rust-client-side"};
// any other value is dropped to preserve bounded cardinality (Phase 47 D-07,
// threat T-47-08 mitigation).
func (m *Metrics) RenameStrategyInc(strategy string) {
    if strategy != "lsp-native" && strategy != "rust-client-side" {
        return
    }
    m.RenameStrategy.WithLabelValues(strategy).Inc()
}
```

Apply this `if value != "x" && value != "y" { return }` early-return drop to `LSPoolLookup`, `RepoMapLookup`, `RepoMapExtractObserve`, `SessionLifecycleInc`, `EditOutcomeInc`. The CI lint enforces label NAMES; the helper enforces label VALUES.

---

### `internal/obs/metrics_labels_test.go` (test/lint)

**Analog:** self.

**carveOuts extension pattern** (lines 22-28, append four entries):
```go
var carveOuts = map[string]map[string]bool{
    "helix_lspool_evictions_total": {"reason": true},
    // Phase 47 D-07: closed-enum "strategy" label on the rename dispatcher
    // counter. Values enforced at emission (see *Metrics.RenameStrategyInc);
    // the CI lint only carves the label NAME.
    "helix_rename_strategy_total": {"strategy": true},
}
```

Append in Phase 53:
- `"helix_lspool_lookups_total": {"result": true}`
- `"helix_repomap_lookups_total": {"result": true}`
- `"helix_repomap_extract_duration_seconds": {"extractor": true}`
- `"helix_session_lifecycle_total": {"phase": true, "transport": true}`
- `"helix_edit_outcome_total": {"strategy": true}` (note `tool_name` and `outcome` are already in `AllowedLabels`)

**Primed-vector pattern** (lines 89-101 — extend with one Inc/Observe per new vector; this is the silent-bypass guard from Pitfall 3 in RESEARCH.md):
```go
m.ToolCalls.WithLabelValues("t", "p", "m", "go", "success").Inc()
m.ToolDuration.WithLabelValues("t", "p", "m", "go").Observe(0.001)
m.LSPoolWorkers.WithLabelValues("go").Set(1)
m.LSPoolEvictions.WithLabelValues("go", "idle").Inc()
m.LSPoolCircuitState.WithLabelValues("go").Set(0)
m.LSPoolRestarts.WithLabelValues("go").Inc()
m.RenameStrategy.WithLabelValues("lsp-native").Inc()
```

**Cardinality bound test pattern** — the existing `lintLabels` walk (lines 50-83) is the model; the new `TestMetrics_CardinalityBounds_*` tests mirror its `Gather()` walk, but aggregate `len(mf.GetMetric())` per family and compare against a constant. RESEARCH.md Code Examples §4 (line 583-616) is the verbatim template.

---

### `internal/obs/metrics_test.go` (test/registered families)

**Analog:** self — `TestMetrics_RegisteredFamilies` (lines 35-70) and `TestMetrics_NoopProviderReturnsUsableSink` (lines 75-91).

**Pattern** — extend `want[]` (lines 55-64):
```go
want := []string{
    "helix_tool_calls_total",
    "helix_tool_duration_seconds",
    "helix_lspool_workers",
    "helix_lspool_evictions_total",
    "helix_lspool_circuit_state",
    "helix_lspool_restarts_total",
    "go_goroutines",
    "process_resident_memory_bytes",
}
```

Append: `"helix_lspool_lookups_total"`, `"helix_repomap_lookups_total"`, `"helix_repomap_extract_duration_seconds"`, `"helix_session_lifecycle_total"`, `"helix_edit_outcome_total"`, `"helix_rename_strategy_total"` (this last one is missing from the existing `want[]` — surfaces a latent gap; planner can fix as part of this phase or note it).

In `TestMetrics_NoopProviderReturnsUsableSink` (lines 84-90), append helper method calls so the noop path exercises every new helper:
```go
m.LSPoolWorkersSet("go", 1)
m.LSPoolEviction("go", "idle")
m.LSPoolCircuitStateSet("go", 2)
m.LSPoolRestart("go")
```

Append: `m.LSPoolLookup("go", "hit")`, `m.RepoMapLookup("go", "hit")`, `m.RepoMapExtractObserve("go", "treesitter", 0.005)`, `m.SessionLifecycleInc("started", "stdio")`, `m.EditOutcomeInc("replace_symbol_body", "success", "exact")`.

---

### `internal/kernel/lspool/metrics.go` (sink interface — extend)

**Analog:** self (lines 1-63, the entire file is the canonical pattern).

**Interface extension pattern** (lines 10-25):
```go
type MetricsSink interface {
    LSPoolWorkersSet(language string, delta float64)
    LSPoolEviction(language, reason string)
    LSPoolCircuitStateSet(language string, state float64)
    LSPoolRestart(language string)
}
```

Append `LSPoolLookup(language, result string)` per D-14.

**Closed-enum constants pattern** (lines 30-43 — copy block-style for new constants):
```go
const (
    EvictIdle     = "idle"
    EvictPressure = "pressure"
    EvictCrash    = "crash"
    EvictShutdown = "shutdown"
)
```

Append:
```go
const (
    LookupHit  = "hit"
    LookupMiss = "miss"
)
```

**NoopSink stub pattern** (lines 45-60, append one stub method):
```go
func (NoopSink) LSPoolWorkersSet(string, float64) {}
func (NoopSink) LSPoolEviction(string, string)    {}
// ... add:
func (NoopSink) LSPoolLookup(string, string)       {}
```

**Compile-time assertion** (line 63 — leave unchanged; the assertion auto-validates the new method on `NoopSink`):
```go
var _ MetricsSink = NoopSink{}
```

---

### `internal/kernel/lspool/metrics_test.go` (recording sink — extend)

**Analog:** self (lines 1-100 are the canonical recording-sink pattern).

**Recording-sink event struct pattern** (lines 23-36):
```go
type workerEvent struct {
    lang  string
    delta float64
}

type evictionEvent struct {
    lang   string
    reason string
}
```

Append:
```go
type lookupEvent struct {
    lang   string
    result string
}
```

**Recording-sink mutex-guarded append pattern** (lines 38-60):
```go
func (r *recordingSink) LSPoolEviction(language, reason string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.evictions = append(r.evictions, evictionEvent{lang: language, reason: reason})
}
```

Append `func (r *recordingSink) LSPoolLookup(language, result string)` mirroring this exactly.

**Snapshot-with-copy pattern** (lines 62-71) — extend the multi-return tuple with `lk []lookupEvent`.

**Constants test pattern** (lines 94-99) — add `TestMetricsSink_LookupResultConstants` mirroring `TestMetricsSink_EvictionReasonConstants`:
```go
func TestMetricsSink_EvictionReasonConstants(t *testing.T) {
    assert.Equal(t, "idle", EvictIdle)
    assert.Equal(t, "pressure", EvictPressure)
    assert.Equal(t, "crash", EvictCrash)
    assert.Equal(t, "shutdown", EvictShutdown)
}
```

---

### `internal/kernel/lspool/pool.go` (instrument AcquireLease)

**Analog:** self — the existing eviction emission in `evictWorkerLocked` (called via `stopAll`/TTL/pressure paths) emits `metrics.LSPoolWorkersSet` and `metrics.LSPoolEviction` at branch points. AcquireLease at lines 108-147 is the new instrumentation site.

**Branch-emission pattern** (lines 108-147 — exact code that needs the new emission lines):
```go
func (p *Pool) AcquireLease(ctx context.Context, sessionID string, wsKey workspace.WorkspaceKey, dirty bool) (*WorkerLease, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if !dirty {
        if w := p.workerForKeyLocked(wsKey); w != nil {
            lease := NewWorkerLease(sessionID, w, false)
            p.leases[sessionID] = lease
            p.logger.Info("shared lease acquired", "session", sessionID, "worker", w.ID())
            // INSERT: p.metrics.LSPoolLookup(wsKey.Language, LookupHit)
            return lease, nil
        }
    }

    // INSERT before circuit/spawn block: p.metrics.LSPoolLookup(wsKey.Language, LookupMiss)

    cb := p.circuitForLanguage(wsKey.Language)
    if !cb.CanAttempt() {
        return nil, cb.CircuitOpenErr()
    }
    if len(p.workers) >= p.config.MaxWorkers {
        return nil, ErrMaxWorkersReached
    }

    worker, err := p.spawnWorkerLocked(ctx, wsKey)
    // ... unchanged
}
```

D-02 lock: miss is recorded BEFORE the circuit-breaker / max-workers check, so circuit-open and max-workers refusals still count as misses.

---

### `internal/repomap/cache.go` (instrument GetOrExtract — sink field)

**Analog:** `internal/kernel/lspool/pool.go` AcquireLease (sink field on the struct, branch emission at hit/miss). Compare struct layout:

**lspool reference** (`pool.go:40-54`):
```go
type Pool struct {
    mu        sync.RWMutex
    workers   map[string]*Worker
    // ...
    metrics   MetricsSink
    // ...
}
```

**lspool nil-sink → Noop default pattern** (`pool.go:61-77`):
```go
func NewPool(cfg PoolConfig, registry *langregistry.Registry, installer *langregistry.Installer, pressure MemoryPressure, logger *slog.Logger, metrics MetricsSink) *Pool {
    if metrics == nil {
        metrics = NoopSink{}
    }
    return &Pool{
        // ...
        metrics: metrics,
        // ...
    }
}
```

Apply to `TagCache`:
```go
type TagCache struct {
    db      *sql.DB
    mu      sync.Mutex
    version int64
    metrics MetricsSink // NEW (planner picks: constructor arg vs. SetMetricsSink setter)
}
```

**Hit-branch emission pattern** (`cache.go:76-84` — concrete in-tree code to instrument):
```go
if err == nil && cachedMtime == mtime {
    // Cache hit: load all tags for this file.
    tags, loadErr := c.loadTags(filePath)
    c.mu.Unlock()
    if loadErr != nil {
        return nil, fmt.Errorf("loading cached tags: %w", loadErr)
    }
    // INSERT: c.metrics.RepoMapLookup(LangFromExt(filePath), LookupHit)
    return tags, nil
}
```

**Miss-branch emission pattern** (`cache.go:86-92`):
```go
// Cache miss or mtime mismatch: extract fresh tags.
c.mu.Unlock()

// INSERT: c.metrics.RepoMapLookup(LangFromExt(filePath), LookupMiss)

tags, err := extractFn()
if err != nil {
    return nil, err
}
```

**Language resolution** — use `repomap.LangFromExt(filePath)` (defined at `internal/repomap/render.go:202`); A1 in RESEARCH.md confirms this is canonical for this package and `langregistry.DetectFromPath` does NOT exist.

**Histogram observation NOT in cache.go** — per Q-2 Option 2 in RESEARCH.md (recommended), the timing happens in the dispatcher at `internal/skill/repomap/skill.go`, not in `cache.go`. Cache emits lookups only.

---

### `internal/repomap/metrics.go` (NEW — sink interface)

**Analog:** `internal/kernel/lspool/metrics.go` (entire file) — the canonical template for a sink interface in a kernel-owned package that must not import `internal/obs/`.

**Verbatim template to copy-and-adapt:**

```go
package repomap

// MetricsSink is the minimal surface repomap needs from the observability
// layer. Implemented by *obs.Metrics (checked at wire-up time in
// internal/daemon); repomap itself never imports internal/obs (D-15).
//
// Method signatures are frozen to match the *obs.Metrics helpers declared in
// internal/obs/metrics.go. A compile-time assertion in internal/daemon/
// wiring_test.go pins the two together.
type MetricsSink interface {
    // RepoMapLookup increments the helix_repomap_lookups_total counter for
    // a (language, result) pair. result must be one of the LookupXxx
    // constants below (D-04 closed enum).
    RepoMapLookup(language, result string)

    // RepoMapExtractObserve records the elapsed seconds of an extractor
    // invocation. extractor must be one of the ExtractorXxx constants below
    // (D-07 closed enum).
    RepoMapExtractObserve(language, extractor string, seconds float64)
}

// Cache lookup result constants — closed enum per D-04.
const (
    LookupHit  = "hit"
    LookupMiss = "miss"
)

// Extractor type constants — closed enum per D-07.
const (
    ExtractorTreesitter = "treesitter"
    ExtractorLSP        = "lsp"
    ExtractorFallback   = "fallback"
)

// NoopSink is used by tests and bootstrap paths where metrics are not wired.
type NoopSink struct{}

func (NoopSink) RepoMapLookup(string, string)                 {}
func (NoopSink) RepoMapExtractObserve(string, string, float64) {}

var _ MetricsSink = NoopSink{}
```

---

### `internal/repomap/metrics_test.go` (NEW — sink test)

**Analog:** `internal/kernel/lspool/metrics_test.go` (lines 1-105 cover the sink-only tests; ignore the Pool-lifecycle parts).

**Pattern — recordingSink + interface assertion + constants**:

```go
func TestMetricsSink_NoopSinkSatisfiesInterface(t *testing.T) {
    var _ MetricsSink = NoopSink{}
}

func TestNoopSink_safe(t *testing.T) {
    var sink MetricsSink = NoopSink{}
    sink.RepoMapLookup("go", LookupHit)
    sink.RepoMapLookup("go", LookupMiss)
    sink.RepoMapExtractObserve("go", ExtractorTreesitter, 0.001)
    sink.RepoMapExtractObserve("go", ExtractorLSP, 0.05)
    sink.RepoMapExtractObserve("go", ExtractorFallback, 0.1)
}

func TestMetricsSink_LookupResultConstants(t *testing.T) {
    assert.Equal(t, "hit", LookupHit)
    assert.Equal(t, "miss", LookupMiss)
}

func TestMetricsSink_ExtractorConstants(t *testing.T) {
    assert.Equal(t, "treesitter", ExtractorTreesitter)
    assert.Equal(t, "lsp", ExtractorLSP)
    assert.Equal(t, "fallback", ExtractorFallback)
}
```

Add a `recordingSink` here too if the cache-emission test needs one (mirror `recordingSink` shape from lspool exactly: `mu sync.Mutex`, append-only event slices, `snapshot()` returning copies).

---

### `internal/skill/repomap/skill.go` (dispatcher — wrap with timing)

**Analog:** `internal/kernel/edit/replace.go::ReplaceBodyWithPlan` — closest in-tree timing-wrapper precedent (it wraps `fuzzy.Match` and returns `*FuzzyMatchInfo` carrying Strategy + Score). For Phase 53 the wrap is simpler: `time.Now()` / `time.Since().Seconds()` around the dispatcher branches.

**Dispatcher branches to instrument** (`skill.go:366-399`):
```go
_, extractErr := s.cache.GetOrExtract(path, func() ([]repomap.Tag, error) {
    // Primary path: tree-sitter extraction
    if s.extractor != nil && (s.registry == nil || s.registry.SupportsLanguage(lang)) {
        // ... → extractor type is "treesitter"
    }

    // Fallback path: LSP documentSymbol
    if s.fallbackDeps != nil && s.fallbackDeps.AcquireFn != nil {
        // ... → extractor type is "lsp" (or "fallback" if non-LSP fallback path)
    }
    // ...
})
```

**Pattern (Q-2 Option 2)** — caller wraps the extractor branch with timing:
```go
extractor := repomap.ExtractorTreesitter // or LSP / Fallback per branch
start := time.Now()
tags, err := /* the actual extraction call */
s.metrics.RepoMapExtractObserve(lang, extractor, time.Since(start).Seconds())
return tags, err
```

The `s.metrics MetricsSink` field is wired via a setter (mirror `SetEnrichFn`/`SetFallbackDeps` pattern at `daemon.go:296,310`):
```go
// 12d. Wire repomap skill metrics sink (D-15).
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
    rs.SetMetricsSink(observability.Metrics()) // *obs.Metrics satisfies repomap.MetricsSink
}
```

---

### `internal/repomap/render.go` (audit second caller of GetOrExtract)

**Analog:** `skill.go` (sibling caller).

**Pattern** — `render.go:127-129` calls `GetOrExtract` with a no-op extractFn that returns `(nil, nil)` when there's nothing to extract:
```go
tags, err := r.cache.GetOrExtract(filePath, func() ([]Tag, error) {
    return nil, nil // no extraction function available at render time
})
```

Decision per planner: this caller never triggers a real extractor, so emitting `RepoMapExtractObserve` here would muddy the histogram. The cache lookup emission (hit/miss) WILL fire here automatically because it lives in `GetOrExtract` itself. No change needed in `render.go`; document this in the plan.

---

### `internal/mcp/middleware.go` (atomic.Pointer setter — exact mirror of Phase 47)

**Analog:** self, lines 18-44, the `renameStrategySink` block.

**Verbatim pattern to clone for `editOutcomeSink`**:
```go
// renameStrategySink is the package-level recorder wired by InstallMiddleware.
// It accepts the closed-enum strategy string and increments the corresponding
// Prometheus counter on obs.Metrics. Nil until the first InstallMiddleware call;
// RecordRenameStrategy no-ops until wiring happens (e.g. during test setup).
// Phase 47 D-07: helix_rename_strategy_total bounded-label counter.
var renameStrategySink atomic.Pointer[func(ctx context.Context, strategy string)]

func setRenameStrategySink(fn func(ctx context.Context, strategy string)) {
    renameStrategySink.Store(&fn)
}

func RecordRenameStrategy(ctx context.Context, strategy string) {
    p := renameStrategySink.Load()
    if p == nil || *p == nil {
        return
    }
    (*p)(ctx, strategy)
}
```

**Phase 53 adaptation** — same shape, additional fields per D-16:
```go
var editOutcomeSink atomic.Pointer[func(ctx context.Context, toolName, outcome, strategy string)]

func setEditOutcomeSink(fn func(ctx context.Context, toolName, outcome, strategy string)) {
    editOutcomeSink.Store(&fn)
}

func RecordEditOutcome(ctx context.Context, toolName, outcome, strategy string) {
    p := editOutcomeSink.Load()
    if p == nil || *p == nil {
        return
    }
    (*p)(ctx, toolName, outcome, strategy)
}
```

**InstallMiddleware wiring pattern** (lines 75-86 — clone for the new sink):
```go
if provider != nil {
    if m := provider.Metrics(); m != nil {
        // Adapter closure discards ctx for now; future OTel integration
        // can read span context from ctx here without touching callers.
        setRenameStrategySink(func(_ context.Context, strategy string) {
            m.RenameStrategyInc(strategy)
        })
    }
}
```

Append a parallel block calling `setEditOutcomeSink` with `m.EditOutcomeInc(toolName, outcome, strategy)`.

**outcomeEnum pattern** (lines 100-119 — copy verbatim shape for `editOutcomeEnum` and `lifecyclePhaseEnum`):
```go
const (
    outcomeSuccess     = "success"
    outcomeInvalidArgs = "invalid_args"
    outcomeNotFound    = "not_found"
    outcomeCircuitOpen = "circuit_open"
    outcomeLSCrash     = "ls_crash"
    outcomeTimeout     = "timeout"
    outcomeInternal    = "internal"
)

var outcomeEnum = []string{
    outcomeSuccess,
    outcomeInvalidArgs,
    // ...
}
```

The `OutcomeEnumForTest` accessor pattern at lines 265-271 should be cloned for `EditOutcomeEnumForTest` and `LifecyclePhaseEnumForTest`.

---

### `internal/kernel/edit/tools.go` (instrument 5 handler returns)

**Analog:** self — line 397 already emits `mcp.RecordRenameStrategy(ctx, string(result.Strategy))`. The Phase 53 pattern adds `mcp.RecordEditOutcome` calls at each handler's return — a defer at handler entry mirroring RESEARCH.md Code Examples §3.

**Per-handler defer pattern (RESEARCH.md §Code Examples 3)**:
```go
mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
    Name:        "replace_symbol_body",
    // ...
}, kernel.WrapToolSpan(tracer, "replace_symbol_body", func(ctx context.Context, req *mcpsdk.CallToolRequest, args ReplaceBodyArgs) (*mcpsdk.CallToolResult, any, error) {
    outcome, strategy := outcomeSuccess, strategyNone
    defer func() { mcp.RecordEditOutcome(ctx, "replace_symbol_body", outcome, strategy) }()
    // ... existing body; mutate `outcome` and `strategy` at each error branch
    if fuzzyInfo != nil {
        strategy = string(fuzzyInfo.Strategy)
    }
    return textResult(text), nil, nil
}))
```

**Existing rename emission to preserve** (line 397):
```go
// Phase 47 D-07: emit closed-enum strategy metric.
mcp.RecordRenameStrategy(ctx, string(result.Strategy))
```

Per D-11, rename ALSO increments `helix_edit_outcome_total{tool_name=rename_symbol, strategy=none}` — both families coexist.

**Outcome classification rules** — RESEARCH.md "Edit-Tool Outcome Map" (lines 619-658) is the source of truth. Each `errorResult(...)` call site maps to a specific outcome bucket per the table. Plans must cite the table per error branch; the planner picks one classifier strategy (likely a small `classifyEditError(err) string` helper analogous to `classifyOutcome` at middleware.go:128-145).

**existing classifyOutcome pattern to mirror** (`middleware.go:128-145`):
```go
func classifyOutcome(result mcpsdk.Result, err error) string {
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return outcomeTimeout
        }
        if errors.Is(err, serr.ErrCircuitOpen) {
            return outcomeCircuitOpen
        }
        return outcomeInternal
    }
    if ctr, ok := result.(*mcpsdk.CallToolResult); ok && ctr != nil && ctr.IsError {
        return outcomeInternal
    }
    return outcomeSuccess
}
```

---

### `internal/kernel/edit/replace.go` (surface fuzzy.Strategy to caller)

**Analog:** self — already returns `*FuzzyMatchInfo{Strategy, Score}` (lines 17-20, 93). No change to the return shape; just confirm the caller wires `fuzzyInfo.Strategy` into `mcp.RecordEditOutcome` (already used at `tools.go:265-267` for response text; just needs the metric emit too).

```go
type FuzzyMatchInfo struct {
    Strategy fuzzy.Strategy
    Score    float64
}
```

---

### `internal/kernel/fileops/tools.go` (instrument 2 handler returns)

**Analog:** `internal/kernel/edit/tools.go::registerRenameSymbol` (the same `mcp.RecordEditOutcome` pattern applied to fileops handlers — same defer shape, same bucket map).

**Pattern** — `replace_in_file` already surfaces `fResult.Strategy` (lines 388-401):
```go
fResult, fErr := fuzzy.Match(content, args.Pattern, fuzzy.Options{...})
if fErr != nil {
    return errorResult(fErr.Error()), nil, nil
}
// ...
text := fmt.Sprintf("1 replacement made in %s (fuzzy)\nmatch_strategy: %s\nsimilarity_score: %.2f",
    args.Path, fResult.Strategy, fResult.Score)
return textResult(text), nil, nil
```

Strategy is `string(fResult.Strategy)` when fuzzy ran, else `none` (per D-11 + C-2: `strategy ∈ {exact, whitespace_normalized, indentation_flexible, failed, none}`).

`fuzzy_edit` (lines 409-438) ALWAYS runs `fuzzy.Match`; strategy is always one of the four `fuzzy.Strategy` values.

---

### `internal/daemon/daemon.go` (forwarder + HTTP wiring)

**Analog:** self — the `forwarderServiceHandler.StreamMCP` block at lines 597-622 is the stdio session boundary; the post-init wiring section at lines 290-323, 336 is where new sinks are wired.

**Forwarder lifecycle pattern** (lines 597-622 — instrumentation targets):
```go
func (h *forwarderServiceHandler) StreamMCP(stream serenav1.ForwarderService_StreamMCPServer) error {
    firstMsg, err := stream.Recv()
    if err != nil {
        return fmt.Errorf("receiving first message: %w", err)
    }

    sessionID := firstMsg.SessionId
    h.logger.Info("new forwarder stream", "session_id", sessionID)
    // INSERT: h.metrics.SessionLifecycleInc("started", "stdio")

    transport := helixMCP.NewGRPCTransport(stream, sessionID, firstMsg)

    session, err := h.mcpServer.SDK().Connect(stream.Context(), transport, nil)
    if err != nil {
        // INSERT: h.metrics.SessionLifecycleInc("error", "stdio")
        return fmt.Errorf("connecting MCP session: %w", err)
    }

    err = session.Wait()
    h.logger.Info("forwarder stream ended", "session_id", sessionID)
    if err != nil {
        // INSERT: h.metrics.SessionLifecycleInc("error", "stdio")
    } else {
        // INSERT: h.metrics.SessionLifecycleInc("ended", "stdio")
    }
    return err
}
```

The `forwarderServiceHandler` struct (lines 589-595) gains a `metrics *obs.Metrics` field; the constructor (planner finds the call site that builds the handler) populates it from `observability.Metrics()`.

**Sink-wiring pattern** (`daemon.go:290-323` — the canonical post-init wiring block; copy the `if rs := repomapSkill.GetRepoMapSkill(); rs != nil { rs.SetXxx(...) }` shape):
```go
// 12b. Wire repomap skill LSP enrichment callback (RMAP-08).
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
    tagCache := rs.Cache()
    rs.SetEnrichFn(func(g *repomapPkg.FileGraph) { /* ... */ })
}
```

Append a 12d block:
```go
// 12d. Wire repomap skill metrics sink (D-15). *obs.Metrics satisfies
// repomap.MetricsSink via the helper methods declared in internal/obs/metrics.go.
if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
    rs.SetMetricsSink(observability.Metrics())
    // also propagate the same sink into the underlying TagCache:
    rs.Cache().SetMetricsSink(observability.Metrics())
}
```

The `editOutcomeSink` is wired inside `InstallMiddleware` per the rename precedent (no separate post-init step needed — see `middleware.go:75-86`).

**HTTP handler wrap pattern** (lines 553-582, current shape):
```go
func (d *Daemon) listenHTTP(ctx context.Context) error {
    mux := http.NewServeMux()
    mux.Handle("/mcp", d.mcpServer.HTTPHandler())
    // ...
}
```

Wrap the inner handler with the new `httpSessionMiddleware` per Q-1 Option 2:
```go
mux.Handle("/mcp", httpSessionMiddleware(d.mcpServer.HTTPHandler(), d.observability.Metrics()))
```

---

### `internal/daemon/wiring_test.go` (compile-time assertion — extend)

**Analog:** self — the existing `var _ lspool.MetricsSink = (*obs.Metrics)(nil)` line at line 17 is the canonical pattern.

**Pattern**:
```go
// Compile-time proof that *obs.Metrics satisfies lspool.MetricsSink.
//
// The lspool.MetricsSink interface in plan 11-03 was frozen against the
// *obs.Metrics helper method signatures declared by plan 11-01. If this
// line stops compiling, the two plans have drifted apart and the fix is
// to ALIGN lspool.MetricsSink (plan 11-03 owns the interface) — NOT to
// mutate internal/obs/metrics.go (plan 11-01 is the upstream contract).
var _ lspool.MetricsSink = (*obs.Metrics)(nil)
```

**Add** (mirroring exactly):
```go
// Compile-time proof that *obs.Metrics satisfies repomap.MetricsSink (D-15).
var _ repomap.MetricsSink = (*obs.Metrics)(nil)
```

The existing assertion auto-validates the new `LSPoolLookup` method on `*obs.Metrics`; if `LSPoolLookup` is missing on either side, this file fails to compile (the failure mode the test exists to catch).

**Companion runtime test pattern** (lines 22-33):
```go
func TestObsMetricsIsLSPoolSink(t *testing.T) {
    var sink lspool.MetricsSink = obs.Noop(nil).Metrics()
    if sink == nil {
        t.Fatal("obs.Noop(...).Metrics() returned nil sink")
    }
    sink.LSPoolWorkersSet("go", +1)
    sink.LSPoolEviction("go", lspool.EvictIdle)
    sink.LSPoolCircuitStateSet("go", lspool.CircuitClosed)
    sink.LSPoolRestart("go")
}
```

Add `TestObsMetricsIsRepoMapSink` mirroring this exactly.

---

### `internal/kernel/edit/tools_test.go` (NEW — outcome+strategy emission)

**Analog:** Two-layer composite:
1. `internal/mcp/middleware_test.go` (the recorder roundtrip via `setRenameStrategySink` — planner reads this file to find the test pattern).
2. `internal/kernel/lspool/metrics_test.go::recordingSink` (the recording-sink shape).

**Pattern** — install a recording recorder via `setEditOutcomeSink` (test-only export needed in `internal/mcp/middleware_test.go` or a small `SetEditOutcomeSinkForTest` accessor), drive each handler with valid + error inputs, assert the recorded `(toolName, outcome, strategy)` tuple. The recordingSink shape from lspool is the right shape:

```go
type editEvent struct {
    toolName string
    outcome  string
    strategy string
}

type recordingRecorder struct {
    mu     sync.Mutex
    events []editEvent
}

func (r *recordingRecorder) record(_ context.Context, toolName, outcome, strategy string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.events = append(r.events, editEvent{toolName, outcome, strategy})
}
```

Same shape applies to `internal/kernel/fileops/tools_test.go`.

---

### `internal/mcp/middleware_test.go` (extend — recorder roundtrip test)

**Analog:** self — look for the existing test that exercises `setRenameStrategySink` + `RecordRenameStrategy`. Mirror that for `setEditOutcomeSink` + `RecordEditOutcome`.

The pattern is: use the package-internal `setEditOutcomeSink` (same package — test file is in `package mcp`) to install a recorder closure; call `RecordEditOutcome(ctx, "tool", "success", "exact")`; assert the closure observed the call. Drop unknown values is enforced upstream in `obs.Metrics.EditOutcomeInc` (the closed-enum guard there mirrors `RenameStrategyInc:171-176`).

---

### `internal/daemon/forwarder_test.go` or extension to `wiring_test.go` (NEW)

**Analog:** Composite — `internal/kernel/lspool/metrics_test.go` (recordingSink shape) + the existing `forwarderServiceHandler.StreamMCP` (the SUT).

**Pattern** — inject a recording metrics sink into `forwarderServiceHandler` (or use a real `obs.Noop().Metrics()` and gather from its registry). Drive a fake stream that delivers a `firstMsg`, observes `started`, then closes; assert the gathered `helix_session_lifecycle_total` shows `phase=started` and either `phase=ended` or `phase=error` depending on the test case.

The `Gather()` walk pattern from `metrics_labels_test.go::lintLabels` (lines 50-83) is the right shape for "given a registry, find this family and assert its label tuples".

---

### `internal/daemon/http_session_middleware.go` (NEW — http.Handler wrapper)

**Analog:** **NO IN-REPO PRECEDENT.** The closest in-tree analogs are:
- `internal/mcp/middleware.go::TelemetryMiddleware` (different layer: MCP `mcpsdk.Middleware`, not `http.Handler`).
- `internal/daemon/daemon.go::listenHTTP` (line 553-582 — the install site, not the wrapper).

**This is a new shape.** Document explicitly in PATTERNS.md (here) and in the plan: there is no http.Handler wrapper in this codebase yet. The closest code-organisation analogue is `TelemetryMiddleware` (closure over a metrics sink, dispatch-then-emit pattern).

**Recommended shape (no in-tree analog — pattern is hand-rolled per Q-1 Option 2)**:
```go
package daemon

import (
    "net/http"
    "sync"

    "github.com/agenthands/helix/internal/obs"
)

// httpSessionMiddleware wraps the SDK's StreamableHTTPHandler to emit
// helix_session_lifecycle_total{transport="http"} on first-seen
// Mcp-Session-Id (started), DELETE /mcp (ended, best-effort), and 5xx
// responses (error). See Phase 53 D-09 + Q-1 Option 2 for the design rationale.
//
// CAVEAT: phase="ended" for HTTP is best-effort. The MCP SDK v1.5.0
// (go.mod:14) does not expose a per-session lifecycle hook; the SDK
// owns session timeout/cleanup internally. This wrapper observes only
// the externally-visible signals.
func httpSessionMiddleware(next http.Handler, metrics *obs.Metrics) http.Handler {
    var seen sync.Map // sessionID -> struct{}
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sessionID := r.Header.Get("Mcp-Session-Id")
        if sessionID != "" {
            if _, loaded := seen.LoadOrStore(sessionID, struct{}{}); !loaded {
                metrics.SessionLifecycleInc("started", "http")
            }
        }
        if r.Method == http.MethodDelete && sessionID != "" {
            metrics.SessionLifecycleInc("ended", "http")
            seen.Delete(sessionID)
        }
        rw := &statusRecorder{ResponseWriter: w}
        next.ServeHTTP(rw, r)
        if rw.status >= 500 {
            metrics.SessionLifecycleInc("error", "http")
        }
    })
}

type statusRecorder struct {
    http.ResponseWriter
    status int
}

func (r *statusRecorder) WriteHeader(code int) {
    r.status = code
    r.ResponseWriter.WriteHeader(code)
}
```

**This shape needs a unit test** — drive an `httptest.NewRecorder` + a stub inner handler; assert the metrics sink observed the right (phase, transport) tuples.

---

## Shared Patterns

### Closed-enum drop-unknown discipline

**Source:** `internal/obs/metrics.go:171-176` (`RenameStrategyInc`).
**Apply to:** All new helper methods on `*obs.Metrics` (`LSPoolLookup`, `RepoMapLookup`, `RepoMapExtractObserve`, `SessionLifecycleInc`, `EditOutcomeInc`).

```go
func (m *Metrics) RenameStrategyInc(strategy string) {
    if strategy != "lsp-native" && strategy != "rust-client-side" {
        return
    }
    m.RenameStrategy.WithLabelValues(strategy).Inc()
}
```

The CI lint enforces label NAMES; the helper enforces label VALUES. Neither layer alone is sufficient — both are required.

### Sink-interface-with-NoopSink decoupling

**Source:** `internal/kernel/lspool/metrics.go` (entire file).
**Apply to:** `internal/repomap/metrics.go` (NEW); preserved in extended `internal/kernel/lspool/metrics.go`.

The interface lives in the upstream package (lspool, repomap); `*obs.Metrics` implements it ad-hoc by virtue of having method names that match. Daemon wires the binding at startup.

### atomic.Pointer[T] package-level setter

**Source:** `internal/mcp/middleware.go:18-44` (renameStrategySink).
**Apply to:** `internal/mcp/middleware.go` (new editOutcomeSink — exact mirror).

```go
var renameStrategySink atomic.Pointer[func(ctx context.Context, strategy string)]

func setRenameStrategySink(fn func(ctx context.Context, strategy string)) {
    renameStrategySink.Store(&fn)
}

func RecordRenameStrategy(ctx context.Context, strategy string) {
    p := renameStrategySink.Load()
    if p == nil || *p == nil {
        return
    }
    (*p)(ctx, strategy)
}
```

Wired from `InstallMiddleware` with an adapter closure that hides `*obs.Metrics` from the caller (see `middleware.go:75-86`).

### Compile-time interface assertion + runtime companion

**Source:** `internal/daemon/wiring_test.go:17` (`var _ lspool.MetricsSink = (*obs.Metrics)(nil)`).
**Apply to:** `internal/daemon/wiring_test.go` (add `var _ repomap.MetricsSink = (*obs.Metrics)(nil)`).

The compile-time assertion catches signature drift; the runtime test (`TestObsMetricsIsLSPoolSink`) catches `obs.Noop(...).Metrics() == nil` regressions. Both are required.

### Recording-sink test double

**Source:** `internal/kernel/lspool/metrics_test.go:11-71` (recordingSink).
**Apply to:** `internal/repomap/metrics_test.go` (NEW); the new tests in `internal/kernel/edit/tools_test.go`, `internal/kernel/fileops/tools_test.go`, and the daemon forwarder test (with adapted struct shapes for the recorder closure pattern, see middleware-recorder note above).

Always: mutex-guarded append-only event slices, `snapshot()` returning copies, single-event-per-method.

### Per-vector primed Inc/Observe in lint test

**Source:** `internal/obs/metrics_labels_test.go:89-101` (`TestMetricsLabelsAllowlist`).
**Apply to:** Same file. Pitfall 3 in RESEARCH.md is the explicit rule — every new vector MUST be primed in this test, or the lint silently bypasses it.

### Add-to-want[]-and-prime-in-noop pattern

**Source:** `internal/obs/metrics_test.go:55-64,84-90` (`TestMetrics_RegisteredFamilies`, `TestMetrics_NoopProviderReturnsUsableSink`).
**Apply to:** Same file. Both lists must mention every new family.

### Instrumentation-at-boundary pattern

**Source:** `internal/kernel/lspool/pool.go::evictWorkerLocked` (existing eviction emission); `internal/kernel/edit/tools.go:397` (existing rename emission).
**Apply to:** `internal/kernel/lspool/pool.go::AcquireLease` (lookup); `internal/repomap/cache.go::GetOrExtract` (lookup); all 7 edit tool handlers (outcome); `internal/skill/repomap/skill.go` extractor branches (extract observe); forwarder StreamMCP + http session middleware (lifecycle).

Boundary pattern: emit at the SAME line as the log entry that already marks the transition. Keeps one source of truth per event.

### Post-init sink wiring block

**Source:** `internal/daemon/daemon.go:290-323` (the 12a/12b/12c sequential `if rs := ...; rs != nil { rs.SetXxx(...) }` blocks).
**Apply to:** Add 12d for `rs.SetMetricsSink(observability.Metrics())` and `rs.Cache().SetMetricsSink(observability.Metrics())`.

### Closed-enum constants block + accessor for tests

**Source:** `internal/mcp/middleware.go:100-119,265-271` (outcomeEnum + OutcomeEnumForTest).
**Apply to:** New `editOutcomeEnum`, `lifecyclePhaseEnum`, `transportEnum` blocks + their `XxxEnumForTest` accessors.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/daemon/http_session_middleware.go` | http.Handler wrapper | request-response | First http.Handler middleware in this codebase. The MCP SDK middleware (`internal/mcp/middleware.go::TelemetryMiddleware`) operates one layer higher (on `mcpsdk.MethodHandler`, after the SDK has already parsed the JSON-RPC envelope and extracted method/params). The HTTP wrapper operates on raw HTTP requests before the SDK sees them — semantically different layer. Pattern is hand-rolled per Q-1 Option 2 in RESEARCH.md; the closest organisational analogue is `TelemetryMiddleware`'s closure-over-metrics shape, but the imports, signatures, and lifecycle are distinct. |

## Metadata

**Analog search scope:**
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/obs/`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/kernel/lspool/`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/kernel/edit/`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/kernel/fileops/`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/repomap/`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/skill/repomap/`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/mcp/`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/daemon/`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/fuzzy/`

**Files scanned:** 22 files read or grepped (full or targeted) — `metrics.go`, `metrics_test.go`, `metrics_labels_test.go`, `obs.go`, `lspool/metrics.go`, `lspool/metrics_test.go`, `lspool/pool.go`, `repomap/cache.go`, `repomap/render.go`, `skill/repomap/skill.go`, `mcp/middleware.go`, `kernel/edit/tools.go`, `kernel/edit/replace.go`, `kernel/edit/verify.go`, `kernel/fileops/tools.go`, `daemon/daemon.go`, `daemon/wiring_test.go`, `fuzzy/types.go`, `fuzzy/diff.go`, `USAGE.md`, plus 53-CONTEXT.md and 53-RESEARCH.md.

**Pattern extraction date:** 2026-04-30

**Confidence:** HIGH for 21 of 22 files (the only "no analog" is the http.Handler wrapper, where the absence is itself documented as a finding to flag in the plan).

**Cross-references the planner should pin:**
- Phase 47 D-07 introduced `renameStrategySink` — the canonical setter pattern. Phase 53 D-16 explicitly mirrors it.
- Phase 11 D-13 introduced the `reason` carve-out — the canonical closed-enum carve-out pattern. Phase 53 D-04/D-07/D-09/D-11 follow it.
- RESEARCH.md Corrections C-1, C-2, C-3 must be cited in the plan: edit-tool list (7 tools across two packages, no `delete_lines`), strategy enum (`failed` not `ellipsis`), HTTP session boundary (no SDK hook — Q-1 Option 2 chosen).
