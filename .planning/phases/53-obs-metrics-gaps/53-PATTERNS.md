# Phase 53: obs-metrics-gaps - Pattern Map

**Mapped:** 2026-04-26
**Files analyzed:** 12 (5 NEW, 7 MODIFIED)
**Analogs found:** 12 / 12 (100% — entire phase mirrors Phase 11 / Phase 47 prior art)

This phase is a strict extension of the Phase 11 owned-registry observability stack with the Phase 47 closed-enum carve-out pattern reapplied. Every file to be created or modified has a direct in-tree analog. **No file is invented from RESEARCH.md library examples**; every excerpt below is from the live codebase.

## File Classification

| File | New/Mod | Role | Data Flow | Closest Analog | Match Quality |
|------|---------|------|-----------|----------------|---------------|
| `internal/obs/metrics.go` | MOD | observability registry | event-driven (counter/histogram emit) | self (Phase 11/47 carve-outs) | exact (extending self) |
| `internal/obs/metrics_test.go` | MOD | test (registered families) | request-response | self | exact |
| `internal/obs/metrics_labels_test.go` | MOD | test (label allowlist lint) | batch | self | exact |
| `internal/obs/metrics_cardinality_test.go` | NEW | test (cardinality bounds) | batch | `metrics_labels_test.go` (gather + assert pattern) | role-match |
| `internal/obs/metrics_alloc_test.go` | NEW | test (zero-alloc noop) | batch | `metrics_test.go` (TestMetrics_NoopProviderReturnsUsableSink) | role-match |
| `internal/repomap/metrics.go` | NEW | sink interface + NoopSink | event-driven | `internal/kernel/lspool/metrics.go` | exact |
| `internal/repomap/cache.go` | MOD | extraction emission site | event-driven | `internal/kernel/lspool/pool.go` (eviction emit pattern) | role-match |
| `internal/kernel/edit/metrics.go` | NEW | sink interface + NoopSink + ClassifyOutcome | event-driven | `internal/kernel/lspool/metrics.go` | exact |
| `internal/kernel/edit/tools.go` (+ replace.go / insert.go / rename.go / delete.go) | MOD | tool handler emission | request-response | self (existing handler shape) + `internal/mcp/middleware.go` (RecordRenameStrategy pattern) | exact |
| `internal/kernel/session_metrics.go` | NEW | sink interface + NoopSink | event-driven | `internal/kernel/lspool/metrics.go` | exact |
| `internal/daemon/wiring_test.go` | MOD | compile-time + runtime sink assertion | batch | self | exact |
| `internal/daemon/daemon.go` | MOD | sink wiring at construction | request-response | self (existing lspool sink wire-up) | exact |

## Pattern Assignments

### `internal/repomap/metrics.go` (NEW — sink interface)

**Analog:** `internal/kernel/lspool/metrics.go` (the canonical Phase 11 D-08 prior art).

**Copy verbatim in shape, swap names:**

```go
// internal/kernel/lspool/metrics.go:1-25 — pattern to mirror
package lspool

// MetricsSink is the minimal surface lspool needs from the observability
// layer. Implemented by *obs.Metrics (checked at wire-up time in
// internal/daemon); lspool itself never imports internal/obs (D-08).
//
// Method signatures are frozen to match the *obs.Metrics helpers declared in
// internal/obs/metrics.go. A compile-time assertion in internal/daemon/
// wiring_test.go pins the two together.
type MetricsSink interface {
    LSPoolWorkersSet(language string, delta float64)
    LSPoolEviction(language, reason string)
    LSPoolCircuitStateSet(language string, state float64)
    LSPoolRestart(language string)
}
```

**Closed-enum constants block (lspool/metrics.go:27-43):**
```go
const (
    EvictIdle     = "idle"
    EvictPressure = "pressure"
    EvictCrash    = "crash"
    EvictShutdown = "shutdown"
)
```

**NoopSink + compile-time assertion (lspool/metrics.go:45-63):**
```go
type NoopSink struct{}
func (NoopSink) LSPoolWorkersSet(string, float64) {}
func (NoopSink) LSPoolEviction(string, string)    {}
func (NoopSink) LSPoolCircuitStateSet(string, float64) {}
func (NoopSink) LSPoolRestart(string)             {}

var _ MetricsSink = NoopSink{}
```

**Apply to repomap as:**
- Interface methods: `RepoMapCacheInc(language, result string)` and `RepoMapExtractObserve(language string, seconds float64)`.
- Constants: `ResultHit = "hit"`, `ResultMiss = "miss"` (D-02).
- `NoopSink struct{}` with both no-op methods + `var _ MetricsSink = NoopSink{}`.

---

### `internal/kernel/edit/metrics.go` (NEW — sink interface + ClassifyOutcome)

**Analog:** `internal/kernel/lspool/metrics.go` (interface shape) + `internal/obs/metrics.go:171-176` (closed-enum guard pattern for `ClassifyOutcome` consumers).

**Interface skeleton (mirror lspool/metrics.go:10-25):**
```go
package edit

type MetricsSink interface {
    EditOutcomeInc(tool, outcome string)
}

type NoopSink struct{}
func (NoopSink) EditOutcomeInc(string, string) {}

var _ MetricsSink = NoopSink{}
```

**Closed-enum constants block + tool allowlist (lspool/metrics.go:27-43 pattern):**
```go
const (
    OutcomeSuccess          = "success"
    OutcomeFuzzyApplied     = "fuzzy_applied"
    OutcomeRefusedAmbiguous = "refused_ambiguous"
    OutcomeFailed           = "failed"
)

// AllowedTools is the closed-enum allowlist for the {tool} label on
// serena_edit_outcome_total. Unknown values are dropped at the obs.Metrics
// helper (mirrors RenameStrategyInc at internal/obs/metrics.go:171-176).
var AllowedTools = map[string]bool{
    "replace_symbol_body":   true,
    "insert_before_symbol":  true,
    "insert_after_symbol":   true,
    "rename_symbol":         true,
    "safe_delete_symbol":    true,
    "replace_in_file":       true,
    "fuzzy_edit":            true,
    "create_file":           true,
    // verify_edit excluded per Open Question 3 recommendation
}
```

**ClassifyOutcome helper (NEW pattern; modeled on guard style of `RenameStrategyInc`):**
The shape is from RESEARCH.md Pitfall 4 (verbatim) but the closed-enum drop-on-unknown idiom comes from `internal/obs/metrics.go:171-176`:
```go
// internal/obs/metrics.go:171-176 (PRIOR ART)
func (m *Metrics) RenameStrategyInc(strategy string) {
    if strategy != "lsp-native" && strategy != "rust-client-side" {
        return
    }
    m.RenameStrategy.WithLabelValues(strategy).Inc()
}
```

---

### `internal/kernel/session_metrics.go` (NEW — sink interface)

**Analog:** `internal/kernel/lspool/metrics.go` again. Identical structure.

**Interface + constants:**
```go
package kernel

type SessionMetricsSink interface {
    SessionLifecycleInc(language, phase string)
}

const (
    PhaseActivate   = "activate"
    PhaseDeactivate = "deactivate"
    PhaseTimeout    = "timeout"
    PhaseShutdown   = "shutdown"
)

type NoopSessionSink struct{}
func (NoopSessionSink) SessionLifecycleInc(string, string) {}

var _ SessionMetricsSink = NoopSessionSink{}
```

(Note: package `kernel` already exists; do NOT redeclare a package-level `NoopSink` if one is in use elsewhere — use `NoopSessionSink` for clarity.)

---

### `internal/obs/metrics.go` (MOD — extend with 5 new vectors + helpers)

**Analog:** itself. The five new vectors slot into the existing `Metrics` struct at the same indentation as Phase 11 + Phase 47 fields.

**Vector field declaration pattern (metrics.go:39-50):**
```go
// lspool gauges and counters (plan 11-03).
LSPoolWorkers      *prometheus.GaugeVec
LSPoolEvictions    *prometheus.CounterVec
LSPoolCircuitState *prometheus.GaugeVec
LSPoolRestarts     *prometheus.CounterVec

// Phase 47 D-07: rename_symbol dispatcher strategy counter.
// ... comment explaining closed-enum carve-out ...
RenameStrategy *prometheus.CounterVec
```

**Add new fields with the same comment style (one block-comment per new vector explaining the carve-out and its CONTEXT.md decision tag):**
- `LSPoolCache *prometheus.CounterVec`        // Phase 53 D-01/D-02
- `RepoMapCache *prometheus.CounterVec`       // Phase 53 D-01/D-02
- `RepoMapExtractDuration *prometheus.HistogramVec` // Phase 53 D-10/D-11
- `SessionLifecycle *prometheus.CounterVec`   // Phase 53 D-04/D-05
- `EditOutcome *prometheus.CounterVec`        // Phase 53 D-07/D-09

**Vector construction pattern (metrics.go:84-93 — closest match for new counters with carve-outs):**
```go
LSPoolEvictions: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "serena_lspool_evictions_total",
        Help: "LS worker evictions by reason (idle/pressure/crash/shutdown).",
    },
    // CONTEXT.md D-13: "reason" is a closed 4-value enum enforced at
    // emission sites. It is NOT in the D-04 RED allowlist; the CI label
    // lint carves it out explicitly for this family.
    []string{"language", "reason"},
),
```

**Histogram construction pattern (metrics.go:69-76) — for `RepoMapExtractDuration`:**
```go
ToolDuration: prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "serena_tool_duration_seconds",
        Help:    "MCP tool call latency in seconds (RED: duration).",
        Buckets: prometheus.DefBuckets, // D-01
    },
    []string{"tool_name", "profile", "mode", "language"},
),
```

**MustRegister extension (metrics.go:119-129):** add the five new vectors to the `reg.MustRegister(...)` call site in the same order they appear as fields.

**Helper method pattern with closed-enum guard (metrics.go:171-176 — VERBATIM shape):**
```go
func (m *Metrics) RenameStrategyInc(strategy string) {
    if strategy != "lsp-native" && strategy != "rust-client-side" {
        return
    }
    m.RenameStrategy.WithLabelValues(strategy).Inc()
}
```

**Apply five times** for `LSPoolCacheInc`, `RepoMapCacheInc`, `RepoMapExtractObserve` (no enum guard — language passes through), `SessionLifecycleInc`, `EditOutcomeInc`. The `EditOutcomeInc` helper additionally consults `edit.AllowedTools` (cross-package read of the const map — fine, no import cycle since obs imports nothing back).

**Histogram observation pattern (no in-tree call site of HistogramVec.Observe yet on `*obs.Metrics`):** use the bare prometheus call:
```go
func (m *Metrics) RepoMapExtractObserve(language string, seconds float64) {
    m.RepoMapExtractDuration.WithLabelValues(language).Observe(seconds)
}
```

---

### `internal/obs/metrics_labels_test.go` (MOD — extend carveOuts)

**Analog:** itself, lines 22-28.

**Existing carve-out map (metrics_labels_test.go:22-28):**
```go
var carveOuts = map[string]map[string]bool{
    "serena_lspool_evictions_total": {"reason": true},
    "serena_rename_strategy_total":  {"strategy": true},
}
```

**Extend per D-02/D-05/D-07/D-08:**
```go
var carveOuts = map[string]map[string]bool{
    "serena_lspool_evictions_total":            {"reason": true},
    "serena_rename_strategy_total":             {"strategy": true},
    "serena_lspool_cache_total":                {"result": true, "scope": true},
    "serena_repomap_cache_total":               {"result": true},
    "serena_repomap_extract_duration_seconds":  {}, // language is in AllowedLabels — no carve-outs
    "serena_session_lifecycle_total":           {"phase": true},
    "serena_edit_outcome_total":                {"tool": true, "outcome": true},
}
```

(Note: `outcome` is already in `AllowedLabels` — but `tool` is NOT `tool_name`, so it must be carved. Verify at plan time.)

**Extend `TestMetricsLabelsAllowlist` (metrics_labels_test.go:89-108)** to prime each new vector with `WithLabelValues(...)` so Gather() sees it. The pattern is verbatim from lines 94-100:
```go
m.ToolCalls.WithLabelValues("t", "p", "m", "go", "success").Inc()
m.ToolDuration.WithLabelValues("t", "p", "m", "go").Observe(0.001)
m.LSPoolWorkers.WithLabelValues("go").Set(1)
m.LSPoolEvictions.WithLabelValues("go", "idle").Inc()
m.LSPoolCircuitState.WithLabelValues("go").Set(0)
m.LSPoolRestarts.WithLabelValues("go").Inc()
m.RenameStrategy.WithLabelValues("lsp-native").Inc()
```

Add five priming lines for the new vectors.

---

### `internal/obs/metrics_test.go` (MOD — extend RegisteredFamilies want list)

**Analog:** itself, lines 35-70.

**Existing pattern (lines 55-64):**
```go
want := []string{
    "serena_tool_calls_total",
    "serena_tool_duration_seconds",
    "serena_lspool_workers",
    "serena_lspool_evictions_total",
    "serena_lspool_circuit_state",
    "serena_lspool_restarts_total",
    "go_goroutines",
    "process_resident_memory_bytes",
}
```

(Note: this list is currently MISSING `serena_rename_strategy_total` — Phase 47 oversight that should NOT be reproduced. Plan 53-01 should add it AND the five new families.) Add: `serena_lspool_cache_total`, `serena_repomap_cache_total`, `serena_repomap_extract_duration_seconds`, `serena_session_lifecycle_total`, `serena_edit_outcome_total`, `serena_rename_strategy_total`.

**Also extend `TestMetrics_NoopProviderReturnsUsableSink` (lines 75-91)** to call each new helper:
```go
// Existing pattern (lines 85-90)
m.ToolCalls.WithLabelValues("t", "p", "m", "go", "success").Inc()
m.ToolDuration.WithLabelValues("t", "p", "m", "go").Observe(0.001)
m.LSPoolWorkersSet("go", 1)
m.LSPoolEviction("go", "idle")
m.LSPoolCircuitStateSet("go", 2)
m.LSPoolRestart("go")
```

Add five lines covering `LSPoolCacheInc / RepoMapCacheInc / RepoMapExtractObserve / SessionLifecycleInc / EditOutcomeInc`.

---

### `internal/obs/metrics_cardinality_test.go` (NEW — D-03 cap test)

**Analog:** `metrics_labels_test.go` (Gather + iterate-and-assert). No exact prior art for cardinality assertion; modeled on the lint walker.

**Reusable lintLabels gatherer pattern (metrics_labels_test.go:50-83):**
```go
mfs, err := gatherer.Gather()
if err != nil {
    t.Fatalf("Gather: %v", err)
}
for _, mf := range mfs {
    name := mf.GetName()
    // ... iterate metric series, count, assert ≤ cap ...
}
```

Caps per D-03 + analogous bounds:
| Family | Cap |
|--------|-----|
| `serena_lspool_cache_total` | 52 × 2 × 3 = 312 |
| `serena_repomap_cache_total` | 52 × 2 = 104 |
| `serena_session_lifecycle_total` | 52 × 4 = 208 |
| `serena_edit_outcome_total` | 8 (tools) × 4 (outcomes) = 32 |
| `serena_repomap_extract_duration_seconds` | 52 (one HistogramVec series per language; bucket count is internal) |

Test primes each vector with worst-case labels and asserts `len(metric.Metric) ≤ cap` per family.

---

### `internal/obs/metrics_alloc_test.go` (NEW — D-15 zero-alloc invariant)

**Analog:** RESEARCH.md Wave-0 Gap. No prior `testing.AllocsPerRun` use in `internal/obs/`.

**Standard `testing.AllocsPerRun` shape (search the tree for prior in-tree alloc tests if any):**
```go
allocs := testing.AllocsPerRun(100, func() {
    var sink lspool.MetricsSink = lspool.NoopSink{}
    sink.LSPoolEviction("go", "idle")
})
if allocs > 0 {
    t.Fatalf("NoopSink.LSPoolEviction allocates %v allocs/op; want 0", allocs)
}
```

(Confirmed via `obs.Noop(...).Metrics()` returns a real `*Metrics` whose vector path DOES allocate label tuples — so this test must scope to the `NoopSink{}` types in each consumer package, not to `*obs.Metrics`. RESEARCH.md says exactly this.)

---

### `internal/repomap/cache.go` (MOD — emit cache hit/miss + extract duration)

**Analog:** `internal/kernel/lspool/pool.go` eviction-emit pattern (one emit per branch; metrics field is a sink interface, defaulted to NoopSink at construction). Per Pitfall 2 RESEARCH.md, prefer Option (b): plumb the sink into `TagCache`.

**Sink-as-struct-field pattern (existing in lspool/pool.go where `p.metrics` is the sink):**
The `TagCache` struct gains a `metrics MetricsSink` field, and `NewTagCache` gains a constructor parameter (or a `WithMetrics` option). The default in callers that don't wire it is `NoopSink{}`.

**Branch-based emission pattern (RESEARCH.md Code Examples — shape verbatim):**
```go
if err == nil && cachedMtime == mtime {
    // ... load tags ...
    c.metrics.RepoMapCacheInc(lang, "hit") // EMIT
    return tags, nil
}
// cold path
start := time.Now()
tags, err := extractFn()
c.metrics.RepoMapExtractObserve(lang, time.Since(start).Seconds())
c.metrics.RepoMapCacheInc(lang, "miss")
```

`lang` is computed via `LangFromExt(filePath)` — already used elsewhere in the package (verified at `internal/repomap/render.go:124`).

---

### `internal/kernel/edit/tools.go` + replace.go / insert.go / rename.go / delete.go (MOD)

**Analog:** the existing `registerReplaceBody` / `registerInsertBefore` etc. handler shape. Sink is passed into `RegisterTools` and threaded into each `register*` helper as a parameter.

**Existing handler-end emission analog — `RecordRenameStrategy` adapter at `internal/mcp/middleware.go` (cited by RESEARCH.md as VERIFIED prior art):** shows the "after the call returns, classify, emit one counter, return original result" idiom. Plan 53-02 reuses this idiom inside each edit-tool handler.

**Pattern (RESEARCH.md Code Examples — TEMPLATE, anchored on `internal/kernel/edit/tools.go:223-272`):**
```go
info, err := /* … existing call to ReplaceBodyWithPlan … */
outcome := edit.ClassifyOutcome(strategyOf(info), err)
sink.EditOutcomeInc("replace_symbol_body", outcome)
// … existing return …
```

For tools that don't carry `fuzzy.Strategy` (rename_symbol, insert_*, safe_delete_symbol, create_file) pass `""` and the classifier returns `success` or `failed` based on `err` only.

---

### `internal/kernel/lspool/pool.go` (MOD — emit cache decisions in AcquireLease)

**Analog:** the existing eviction-emit at `pool.go:298` (cited in RESEARCH.md). Same `p.metrics.<Helper>(...)` shape, four call sites mapped to the four scope/result branches.

**Pattern (existing eviction emit — call-out per branch):** verified at `internal/kernel/lspool/pool.go:298`.

**Branch map per RESEARCH.md Pitfall 1 + Code Examples:**
| Branch | Emit |
|--------|------|
| Shared lease (lines 117-122) | `LSPoolCacheInc(lang, "hit", "clean")` |
| Circuit blocked (lines 127-129) | `LSPoolCacheInc(lang, "miss", "crashed")` |
| MaxWorkers reached | `LSPoolCacheInc(lang, "miss", "clean")` |
| Spawn failure | `LSPoolCacheInc(lang, "miss", "crashed")` (per Open Question 2 recommendation) |
| Successful spawn | `LSPoolCacheInc(lang, "miss", scope)` where `scope = "dirty"` if dirty else `"clean"` |

(Open Question 2: spawn-failure scope still needs planner sign-off.)

---

### `internal/skill/repomap/skill.go` (MOD — minimal change)

If TagCache plumbing approach (Pitfall 2 Option b) is taken, this file changes only at the `NewTagCache(...)` construction site to pass the sink. **Analog:** the existing `NewTagCache` call at `skill.go` near `NewTreeRenderer` — wraps with the new sink argument.

---

### `internal/kernel/kernel.go` + `internal/daemon/daemon.go` (MOD — session lifecycle emit)

**Analog:** the existing eviction emit pattern in `lspool/pool.go` mirrored at four call sites.

| Call site | Emit |
|-----------|------|
| `kernel.ActivateWorkspace` (kernel.go:59-93, both lazy + explicit converge here) | `sink.SessionLifecycleInc(lang, kernel.PhaseActivate)` |
| gRPC `DeactivateWorkspace` handler (daemon.go:614-662) | `sink.SessionLifecycleInc(lang, kernel.PhaseDeactivate)` |
| `pool.checkTTLs` worker idle eviction (pool.go:354-356) | `sink.SessionLifecycleInc(lang, kernel.PhaseTimeout)` (per Pitfall 3 recommendation) |
| Daemon shutdown sweep | `sink.SessionLifecycleInc(lang, kernel.PhaseShutdown)` per still-active workspace |

---

### `internal/daemon/wiring_test.go` (MOD — extend with three new var _ assertions)

**Analog:** itself, lines 17-33. The exact pattern to mirror.

**Existing assertion (wiring_test.go:17):**
```go
var _ lspool.MetricsSink = (*obs.Metrics)(nil)
```

**Extend per D-14:**
```go
var _ lspool.MetricsSink   = (*obs.Metrics)(nil) // EXISTING (lspool extended w/ LSPoolCacheInc)
var _ repomap.MetricsSink  = (*obs.Metrics)(nil) // NEW
var _ edit.MetricsSink     = (*obs.Metrics)(nil) // NEW
var _ kernel.SessionMetricsSink = (*obs.Metrics)(nil) // NEW
```

**Existing runtime smoke pattern (wiring_test.go:22-33):**
```go
func TestObsMetricsIsLSPoolSink(t *testing.T) {
    var sink lspool.MetricsSink = obs.Noop(nil).Metrics()
    if sink == nil {
        t.Fatal("obs.Noop(...).Metrics() returned nil sink")
    }
    sink.LSPoolWorkersSet("go", +1)
    sink.LSPoolWorkersSet("go", -1)
    sink.LSPoolEviction("go", lspool.EvictIdle)
    sink.LSPoolCircuitStateSet("go", lspool.CircuitClosed)
    sink.LSPoolRestart("go")
}
```

**Mirror three more times** (`TestObsMetricsIsRepoMapSink`, `TestObsMetricsIsEditSink`, `TestObsMetricsIsSessionSink`) calling each new helper with valid enum values.

---

### `internal/daemon/daemon.go` (MOD — wire sinks at construction)

**Analog:** the existing lspool sink wire-up. Find where `pool.New(...)` (or equivalent) is constructed and the `*obs.Metrics` is passed in; mirror that for the three new consumers (`TagCache`, edit `RegisterTools`, kernel `SetSessionMetricsSink` or constructor parameter).

## Shared Patterns

### Owned-Registry + Vector Construction (Pattern 1)
**Source:** `internal/obs/metrics.go:53-131`
**Apply to:** All five new vectors. Each is constructed once in `newMetrics()`, registered with `reg.MustRegister(...)` against the owned `*prometheus.Registry`. Never touch `prometheus.DefaultRegisterer`.

```go
m := &Metrics{
    registry: reg,
    LSPoolEvictions: prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "serena_lspool_evictions_total",
            Help: "LS worker evictions by reason (idle/pressure/crash/shutdown).",
        },
        []string{"language", "reason"},
    ),
    // ... etc
}
reg.MustRegister(m.LSPoolEvictions, /* ... */)
```

### Closed-Enum Carve-Out at Sink + CI Label Lint (Pattern 2)
**Source:** `internal/obs/metrics.go:171-176` (sink-side guard) + `internal/obs/metrics_labels_test.go:22-28` (CI carve-out)
**Apply to:** `result`, `scope`, `phase`, `outcome`, edit-`tool` labels.

```go
// metrics.go side:
func (m *Metrics) RenameStrategyInc(strategy string) {
    if strategy != "lsp-native" && strategy != "rust-client-side" {
        return
    }
    m.RenameStrategy.WithLabelValues(strategy).Inc()
}

// metrics_labels_test.go side:
"serena_rename_strategy_total": {"strategy": true},
```

### Per-Package Sink + NoopSink + Compile-Time Assertion (Pattern 3)
**Source:** `internal/kernel/lspool/metrics.go:1-63` + `internal/daemon/wiring_test.go:17`
**Apply to:** `internal/repomap/metrics.go`, `internal/kernel/edit/metrics.go`, `internal/kernel/session_metrics.go` (or similar in-package file).

```go
type MetricsSink interface { /* ... */ }
type NoopSink struct{}
// ... no-op method receivers ...
var _ MetricsSink = NoopSink{}
```

Wiring is asserted in `internal/daemon/wiring_test.go` via:
```go
var _ <pkg>.MetricsSink = (*obs.Metrics)(nil)
```

### Test Vector Priming (Pattern 4)
**Source:** `internal/obs/metrics_labels_test.go:94-100` and `internal/obs/metrics_test.go:40-45`
**Apply to:** Every test in `internal/obs/` that asserts on `Gather()` output. CounterVec/HistogramVec/GaugeVec series are dropped from `Gather()` until labelled at least once.

```go
m.LSPoolEvictions.WithLabelValues("go", "idle").Inc()
m.LSPoolCircuitState.WithLabelValues("go").Set(0)
```

### Noop-Default Invariant (Pattern 5 — D-15)
**Source:** `internal/obs/obs.go:47-53` (`obs.Noop` constructs `newMetrics()` unconditionally)
**Apply to:** All call sites. Never branch on `metrics == nil`. The vectors are constructed even in noop mode; they go unscraped.

## No Analog Found

| File | Role | Reason |
|------|------|--------|
| `internal/obs/metrics_alloc_test.go` | zero-alloc test | No prior `testing.AllocsPerRun` usage in `internal/obs/`. Standard library idiom; not a pattern, just a stdlib API. |
| `internal/obs/metrics_cardinality_test.go` | cardinality cap test | No prior cardinality bound assertion. Modeled on `metrics_labels_test.go` Gather-walker shape; cap arithmetic per D-03 is new. |

Both are minor — the surrounding test infrastructure (Gather walker, vector priming, `*Metrics` construction) is fully covered by existing patterns; only the assertion semantics differ.

## Metadata

**Analog search scope:**
- `internal/obs/` (all .go files)
- `internal/kernel/lspool/` (metrics.go, pool.go, eviction emission)
- `internal/repomap/` (cache.go, render.go for `LangFromExt`)
- `internal/kernel/edit/` (existing handler shape in tools.go)
- `internal/daemon/` (wiring_test.go, daemon.go construction)
- `internal/mcp/` (middleware.go RecordRenameStrategy adapter — referenced but not directly mirrored)

**Files scanned:** 14 (read in full or relevant ranges)
**Pattern extraction date:** 2026-04-26

**Phase 11 / Phase 47 lineage:** Every primary pattern in this phase has a frozen prior-art file. Plan authors should treat the analog file paths and line numbers cited above as the source of truth — when in doubt, re-read the analog and copy its shape rather than reinventing.
