# Phase 53: obs-metrics-gaps - Research

**Researched:** 2026-04-30
**Domain:** Prometheus metrics emission (Go-native, prometheus/client_golang)
**Confidence:** HIGH

## Summary

CONTEXT.md D-01..D-17 lock the metric names, labels, bucket layout, decoupling
patterns, and emission boundaries. This research fills the eight planner-blocking
gaps the orchestrator enumerated, and surfaces three fact-check corrections to
CONTEXT.md that the planner must reconcile before writing tasks (drift between
context-doc claims and code reality, not decision relitigation).

**Primary recommendation:** Plan tasks against the corrected facts in this
document — the 7-tool edit list is wrong in CONTEXT D-12 (the actual surface is
6 + 2 split across `internal/kernel/edit/` and `internal/kernel/fileops/`); the
strategy enum is wrong in D-11 (`fuzzy.Strategy` does not have `ellipsis`, it
has `failed`); and the HTTP-transport session boundary CANNOT be instrumented
in user code with the current SDK version (v1.5.0) — `transport="http"` must
either be deferred or implemented via a non-standard hook.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Cache hit metrics (lspool + repomap)**
- D-01: Single counter per family with a `result={hit,miss}` carve-out label.
  Two new vectors: `helix_lspool_lookups_total{language, result}` and
  `helix_repomap_lookups_total{language, result}`. Hit-ratio is a PromQL
  expression. Update ROADMAP wording from `*_cache_hits_total` to
  `helix_lspool_lookups_total` / `helix_repomap_lookups_total`.
- D-02: lspool "hit" boundary is share-warm-worker vs. spawn. Hit at
  `pool.go:114-122` (`workerForKeyLocked` returned a worker); miss at
  `pool.go:130-141` (spawn path) regardless of whether spawn succeeds. Dirty
  acquires count as misses. No `dirty` label dimension.
- D-03: repomap "hit" boundary is the mtime-match branch in
  `TagCache.GetOrExtract`. Hit at `cache.go:76-84`; miss at `cache.go:86-92`.
  Granularity per-file. `language` resolved via existing langregistry path
  (planner picks call — `langregistry.DetectFromPath` candidate).
- D-04: Labels = `{language, result}` for both. `language` already in
  AllowedLabels. `result` is a closed-enum carve-out for these two families
  with values `{hit, miss}`.

**RepoMap extraction histogram**
- D-05: Instrument by wrapping `extractFn` inside `TagCache.GetOrExtract`
  (cache.go:89). Single instrumentation point. Cache-hit overhead excluded.
  Wiring uses an injected `RepoMapMetricsSink` interface in
  `internal/repomap/`; `internal/repomap/` does NOT import `internal/obs/`.
- D-06: Custom buckets `{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25,
  0.5, 1.0, 2.5}` (1ms→2.5s). Metric name:
  `helix_repomap_extract_duration_seconds`.
- D-07: Labels = `{language, extractor}`. `extractor` is a closed-enum
  carve-out with values `{treesitter, lsp, fallback}` enforced at emission
  (drop unknown values, mirror `RenameStrategyInc`).

**Session lifecycle**
- D-08: Closed-enum `phase={started, ended, error}`. Three values. `started`
  and `ended` from forwarder stream lifecycle (`daemon.go:606,620`). `error`
  when the stream returns a non-nil error. Workspace activation NOT a phase.
  Metric name: `helix_session_lifecycle_total`.
- D-09: Add `transport={stdio, http}` carve-out. `stdio` for forwarder
  sessions. `http` for direct Streamable HTTP — planner identifies the
  matching session-create / session-end boundary in HTTP path during research.

**Edit-tool outcome counter**
- D-10: Closed-enum `outcome={success, no_match, ambiguous_match,
  validation_failed, ls_error, internal}`. Six values mirroring
  `outcomeEnum` discipline (middleware.go:111-119). Metric name:
  `helix_edit_outcome_total`.
- D-11: Separate `strategy` label from rename's existing dispatcher counter.
  `helix_edit_outcome_total{tool_name, outcome, strategy}`. `strategy` ∈
  `{exact, whitespace_normalized, indentation_flexible, ellipsis, none}` —
  *(see Correction C-2 below: the actual fuzzy.Strategy enum is
  `{exact, whitespace_normalized, indentation_flexible, failed}`; `ellipsis`
  is not a Strategy, and `failed` is. Planner must reconcile.)*
  Existing `helix_rename_strategy_total` (Phase 47 D-07) preserved unchanged.
- D-12: Tool scope = all 7 edit tools, reuse existing `tool_name` label.
  Stated tools: `replace_in_file`, `replace_symbol_body`, `fuzzy_edit`,
  `insert_after_symbol`, `insert_before_symbol`, `delete_lines`,
  `rename_symbol`. *(See Correction C-1: actual code surface is different;
  there is no `delete_lines` tool, and the existing `safe_delete_symbol` +
  `verify_edit` tools live in `internal/kernel/edit/`.)*

**Naming + ROADMAP correction**
- D-13: All new names use `helix_*` prefix. Update ROADMAP success-criterion-1.
  Final names: `helix_lspool_lookups_total`, `helix_repomap_lookups_total`,
  `helix_repomap_extract_duration_seconds`, `helix_session_lifecycle_total`,
  `helix_edit_outcome_total`.

**Wiring patterns**
- D-14: lspool extends existing `MetricsSink` interface with
  `LSPoolLookup(language, result string)`. `*obs.Metrics` implements;
  `NoopSink` gets a no-op stub.
- D-15: repomap declares `RepoMapMetricsSink` in `internal/repomap/` with
  `RepoMapLookup(language, result string)` and `RepoMapExtractObserve(
  language, extractor string, seconds float64)`. Wired at daemon post-init.
- D-16: Edit tools wire via package-level setter mirroring
  `mcp.RecordRenameStrategy`. New `mcp.RecordEditOutcome(ctx, toolName,
  outcome, strategy)` set from `InstallMiddleware`.
- D-17: Session lifecycle wires directly via `*obs.Provider`. Methods on
  `*obs.Metrics` named `SessionLifecycleInc(phase, transport string)`.

### Claude's Discretion
- Exact file:line for instrumentation hook insertion within each locked
  boundary (the boundary is locked by D-NN; the exact line is yours).
- Naming of new helper methods on `*obs.Metrics` beyond the locked
  `SessionLifecycleInc` (e.g., `LSPoolLookup`, `RepoMapLookup`,
  `RepoMapExtractObserve`).
- Test file names and their location within the existing test layout.
- The set of carve-out enum values added to `metrics_labels_test.go::carveOuts`
  — D-04/D-07/D-09 dictate the families and label names; specific carve-out
  formulation is yours within those constraints.

### Deferred Ideas (OUT OF SCOPE)
- Workspace-activation cold-start visibility (defer to Phase 54 / OBS-04).
- `dirty` dimension on lspool lookups.
- Per-query repomap lookups counter (vs. per-file).
- Refactoring `helix_rename_strategy_total` (Phase 47 contract preserved).
- Wiring metrics into `legacy/` Python tree.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-03 | Close v1.2 metrics gaps — cache hit-rate (lspool + repomap), RepoMap extraction latency histogram, session lifecycle counters, edit-tool outcome counters, all bounded labels. | All 5 metric families specified by D-01..D-12. Boundaries verified at the locked file:lines. Decoupling patterns (D-14..D-17) re-use the proven Phase 11 / Phase 47 sink + setter scaffolding. The CI cardinality lint extension closes the bounded-label success criterion. |

## Project Constraints (from CLAUDE.md)

These directives bind plans regardless of CONTEXT.md decisions:

- **Go-native only.** All instrumentation lives in Go; no Python/legacy edits.
- **`go vet ./...` and `go test ./...` must pass before completing any Go task.**
  Verification steps in tasks must include both.
- **`gofmt -w .` formatting** required.
- **SMTC-first for code-aware operations.** When tasks need to enumerate edit
  tool handlers / fuzzy match call sites / etc., use `mcp__smtc__find_call_sites_matching`,
  `find_references`, `goto_definition` rather than `grep`.
- **GSD workflow enforcement.** All edits flow through GSD commands; no direct
  edits without an active workflow.
- **Noop-default invariant.** `*obs.Provider` constructed via `Noop` returns
  non-nil `Metrics()`; vectors must be zero-cost when `/metrics` is unmounted
  (constructed at startup, never lazily). Locked at the package level by
  `internal/obs/obs.go` and `internal/obs/metrics.go` design rules.
- **Single registry per Provider.** Never touch `prometheus.DefaultRegisterer`
  (T-11-05). All new vectors register on `m.registry` inside `newMetrics()`.
- **Closed-enum labels enforced at emission site.** CI lint enforces label
  NAME bound; emission code drops unknown VALUES silently.
- **Coding language scope.** Phase touches: `internal/obs/`, `internal/kernel/lspool/`,
  `internal/repomap/`, `internal/kernel/edit/`, `internal/kernel/fileops/`,
  `internal/daemon/`, `internal/mcp/`. **Does not touch:** any HTTP transport
  internals (SDK is third-party), `legacy/` Python.
</phase_requirements>

</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|---|---|---|---|
| Vector construction & label-name lint | `internal/obs` (metrics.go) | `internal/obs` (metrics_labels_test.go) | All `prometheus/client_golang` imports already confined here per Phase 11 D-01. |
| lspool lookup emission | `internal/kernel/lspool` (pool.go) | `internal/obs` (metrics.go helper) | Hot-path emission lives where the branch decision is made; sink interface keeps lspool free of obs imports (D-14). |
| repomap lookup + extract observe | `internal/repomap` (cache.go) | `internal/obs` (metrics.go helpers) | Same decoupling pattern as lspool; new `RepoMapMetricsSink` declared in `internal/repomap/` (D-15). |
| Session lifecycle (stdio) | `internal/daemon` (daemon.go forwarder handler) | `internal/obs` (direct call) | Daemon already imports `internal/obs`; sink-interface adds no isolation value (D-17). |
| Session lifecycle (http) | **OPEN — see Correction C-3.** No stable user-code seam in SDK v1.5.0. | — | The `transport="http"` dimension cannot be implemented by adding a call to user code today; planner must choose between deferral, SDK middleware around `tools/call`, or a wrapping `http.Handler`. |
| Edit-outcome emission | `internal/mcp` (recorder, package-level setter) | `internal/kernel/edit/` + `internal/kernel/fileops/` (handler returns) | Mirrors Phase 47 D-07 `mcp.RecordRenameStrategy` (D-16). Edit tools split across two packages; both call the recorder. |
| ROADMAP correction | `.planning/ROADMAP.md` line 189 | — | Doc-only edit; `serena_*` → `helix_*` (D-13). |
| USAGE.md docs | `USAGE.md` lines 644–678 | — | Existing Prometheus Metrics table extended with 5 new rows + 2 new PromQL examples. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---|---|---|---|
| `prometheus/client_golang` | already in tree (`internal/obs/metrics.go:22`) | Counter/Gauge/Histogram vectors | Established by Phase 11; no version bump needed. |
| `prometheus/client_model/go` | already in tree (`internal/obs/metrics_labels_test.go:8`) | `dto.MetricFamily` for Gather() walks | Used by the existing label-allowlist lint and by gather-based cardinality assertions. |

### Supporting
| Library | Version | Purpose | When to Use |
|---|---|---|---|
| `stretchr/testify/assert` | already in tree (`internal/kernel/lspool/metrics_test.go:8`) | Test assertions | Match the existing test style in `metrics_test.go` files; do not introduce new assertion libraries. |

**Installation:** No new dependencies. All packages are already vendored.

**Version verification:** Skipped — no new dependencies are added in this phase.
The single new external surface (`prometheus.HistogramVec` with custom buckets)
is already used by `helix_tool_duration_seconds` so the construct is proven.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────┐
                    │          *obs.Provider              │
                    │   (Noop default; Metrics() != nil)  │
                    └──────────────┬──────────────────────┘
                                   │ owns
                                   ▼
                    ┌─────────────────────────────────────┐
                    │  *obs.Metrics                       │
                    │  - prometheus.Registry (owned)      │
                    │  - vectors constructed in           │
                    │    newMetrics()                     │
                    │  - helper methods (LSPoolLookup,    │
                    │    RepoMapLookup, ...)              │
                    └──────────────┬──────────────────────┘
                                   │ exposed via three patterns
            ┌──────────────────────┼──────────────────────────────┐
            ▼                      ▼                              ▼
   ┌─────────────────┐    ┌──────────────────┐         ┌──────────────────┐
   │ Sink Interface  │    │ Package-level    │         │ Direct method    │
   │ (lspool,        │    │ setter           │         │ call             │
   │  repomap)       │    │ (mcp.RecordX,    │         │ (daemon.go for   │
   │                 │    │  atomic.Pointer) │         │  session events) │
   └────────┬────────┘    └────────┬─────────┘         └────────┬─────────┘
            │                      │                            │
            ▼                      ▼                            ▼
   ┌─────────────────┐    ┌──────────────────┐         ┌──────────────────┐
   │ pool.go         │    │ kernel/edit/     │         │ daemon.go        │
   │ AcquireLease    │    │   tools.go       │         │   StreamMCP      │
   │ ───────────     │    │ + fileops/       │         │ (forwarder)      │
   │ cache.go        │    │   tools.go       │         │                  │
   │ GetOrExtract    │    │ at handler       │         │ http path: OPEN  │
   │                 │    │ return           │         │                  │
   └─────────────────┘    └──────────────────┘         └──────────────────┘

   Wired in internal/daemon/daemon.go post-init (steps 12-14):
     - lspool sink: passed to NewKernel(... observability.Metrics() ...)
     - repomap sink: NEW SetMetricsSink call (planner adds)
     - edit recorder: setEditOutcomeSink in InstallMiddleware
     - session direct: forwarder handler closes over observability.Metrics()
```

### Recommended Project Structure
```
internal/
├── obs/
│   ├── metrics.go              # ADD vectors + 4 new helper methods
│   ├── metrics_test.go         # EXTEND want[] family list
│   └── metrics_labels_test.go  # EXTEND carveOuts + cardinality bound tests
├── kernel/
│   ├── lspool/
│   │   ├── metrics.go          # EXTEND MetricsSink with LSPoolLookup; constants
│   │   ├── metrics_test.go     # EXTEND recordingSink + tests
│   │   └── pool.go             # ADD lookup emission at AcquireLease branches
│   ├── edit/
│   │   └── tools.go            # ADD mcp.RecordEditOutcome at handler returns (5 tools)
│   └── fileops/
│       └── tools.go            # ADD mcp.RecordEditOutcome at handler returns (2 tools)
├── repomap/
│   ├── metrics.go              # NEW: RepoMapMetricsSink interface + NoopSink + constants
│   ├── metrics_test.go         # NEW: emission tests
│   └── cache.go                # ADD MetricsSink field on TagCache; emit in GetOrExtract
├── mcp/
│   ├── middleware.go           # ADD editOutcomeSink + RecordEditOutcome (mirror rename)
│   └── middleware_test.go      # ADD recorder roundtrip tests
└── daemon/
    ├── daemon.go               # WIRE repomap sink + edit recorder + session call sites
    └── wiring_test.go          # ADD compile-time assertions for new sinks
```

### Pattern 1: lspool sink-interface extension (D-14)
**What:** Append one method to an existing interface; implement on `*obs.Metrics`; add no-op on `NoopSink`.
**When to use:** A package the kernel owns that already has a `MetricsSink` and must not import obs.
**Example:**
```go
// internal/kernel/lspool/metrics.go (add to existing interface)
type MetricsSink interface {
    LSPoolWorkersSet(language string, delta float64)
    LSPoolEviction(language, reason string)
    LSPoolCircuitStateSet(language string, state float64)
    LSPoolRestart(language string)
    LSPoolLookup(language, result string) // NEW
}

const (
    LookupHit  = "hit"
    LookupMiss = "miss"
)

func (NoopSink) LSPoolLookup(string, string) {}
```
```go
// internal/obs/metrics.go (add helper)
func (m *Metrics) LSPoolLookup(language, result string) {
    if result != "hit" && result != "miss" {
        return
    }
    m.LSPoolLookups.WithLabelValues(language, result).Inc()
}
```

### Pattern 2: repomap NEW sink-interface (D-15)
**What:** Declare a new sink interface in `internal/repomap/` because the package previously had no metrics seam.
**When to use:** First time wiring observability into a package without using global state.
**Example:**
```go
// internal/repomap/metrics.go (NEW file)
type MetricsSink interface {
    RepoMapLookup(language, result string)
    RepoMapExtractObserve(language, extractor string, seconds float64)
}

type NoopSink struct{}
func (NoopSink) RepoMapLookup(string, string)                      {}
func (NoopSink) RepoMapExtractObserve(string, string, float64)     {}
var _ MetricsSink = NoopSink{}

const (
    LookupHit  = "hit"
    LookupMiss = "miss"
    ExtractorTreesitter = "treesitter"
    ExtractorLSP        = "lsp"
    ExtractorFallback   = "fallback"
)
```
The `TagCache` struct gains a `metrics MetricsSink` field; `NewTagCache` accepts
or accepts-via-Option a sink (planner picks). Default to `NoopSink{}` when nil
to preserve test ergonomics.

### Pattern 3: package-level setter via `atomic.Pointer` (D-16)
**What:** Mirrors `mcp.renameStrategySink` exactly. New parallel sink for edit outcomes.
**When to use:** When a downstream package (`internal/kernel/edit`, `internal/kernel/fileops`)
must emit metrics without taking an obs dependency, and a plain function call at
return-site is acceptable (no per-call sink injection through every layer).
**Example:**
```go
// internal/mcp/middleware.go (add alongside renameStrategySink)
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
Wired in `InstallMiddleware` immediately after the existing `setRenameStrategySink`
adapter (middleware.go:75-86).

### Pattern 4: Direct call from daemon (D-17)
**What:** `daemon.go:StreamMCP` already has access to `observability` (built at line ~191).
The forwarder handler can close over it and call `observability.Metrics().SessionLifecycleInc(phase, transport)` directly.
**When to use:** Caller already imports `internal/obs`; sink interface adds no value.
**Example:**
```go
// internal/daemon/daemon.go forwarderServiceHandler
type forwarderServiceHandler struct {
    serenav1.UnimplementedForwarderServiceServer
    mcpServer  *helixMCP.SerenaMCPServer
    kernel     *kernel.Kernel
    logger     *slog.Logger
    metrics    *obs.Metrics // NEW field
}

func (h *forwarderServiceHandler) StreamMCP(stream serenav1.ForwarderService_StreamMCPServer) error {
    firstMsg, err := stream.Recv()
    if err != nil {
        return fmt.Errorf("receiving first message: %w", err)
    }
    sessionID := firstMsg.SessionId
    h.logger.Info("new forwarder stream", "session_id", sessionID)
    h.metrics.SessionLifecycleInc("started", "stdio") // NEW

    transport := helixMCP.NewGRPCTransport(stream, sessionID, firstMsg)
    session, err := h.mcpServer.SDK().Connect(stream.Context(), transport, nil)
    if err != nil {
        h.metrics.SessionLifecycleInc("error", "stdio") // NEW
        return fmt.Errorf("connecting MCP session: %w", err)
    }

    err = session.Wait()
    h.logger.Info("forwarder stream ended", "session_id", sessionID)
    if err != nil {
        h.metrics.SessionLifecycleInc("error", "stdio") // NEW
    } else {
        h.metrics.SessionLifecycleInc("ended", "stdio") // NEW
    }
    return err
}
```

### Anti-Patterns to Avoid
- **Lazy vector construction.** All vectors must be built in `newMetrics()`
  during `Noop` construction so the noop path is zero-allocation per call,
  not per-startup. CI label lint relies on this — vectors that don't exist
  at Gather() time can hide forbidden labels.
- **Global registerer.** `prometheus.DefaultRegisterer` is banned (T-11-05).
- **Branching on nil sink.** `*obs.Metrics` is never nil in the noop path; do
  not introduce nil checks at emission sites — the helper methods can drop
  unknown enum values internally, which IS the bounded discipline.
- **Adding label names to AllowedLabels.** None of `result`, `extractor`,
  `phase`, `transport` go in AllowedLabels; they are all closed-enum carve-outs
  added to `metrics_labels_test.go::carveOuts`. The CI lint already supports
  per-family carve-outs (`metrics_labels_test.go:22-28`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---|---|---|---|
| Counter with bounded labels | Custom `sync.Map` + atomic counters | `prometheus.NewCounterVec` + `WithLabelValues(...).Inc()` | Already integrated with `/metrics` scrape, registry, and label-pair encoding. |
| Latency histogram | Manual percentile sketch | `prometheus.NewHistogramVec` | Existing `helix_tool_duration_seconds` is the model. |
| Closed-enum value validation | Stringly-typed branching at emission | `if value != "x" && value != "y" { return }` (drop unknown) — pattern from `RenameStrategyInc` (`metrics.go:171-176`) | Single early-return guards the cardinality bound; matches the in-tree convention. |
| Per-family cardinality test | Custom limit type | Gather all families on a primed registry, count `len(mf.GetMetric())`, compare to constant | The existing `metrics_labels_test.go::lintLabels` already walks `Gather()` output; copy that loop with a different aggregator. |

**Key insight:** The Phase 11 + Phase 47 scaffolding has already designed away
every mistake we could re-make in Phase 53. Tasks should call into existing
patterns, not redesign them.

## Common Pitfalls

### Pitfall 1: Histogram cardinality calculation off-by-one
**What goes wrong:** Computing the cardinality bound as `N_buckets * N_label_combos` undercounts.
**Why it happens:** Every `prometheus.HistogramVec` series produces N+1 bucket samples (the N explicit buckets + `+Inf`) PLUS a `_count` AND a `_sum` series per label combination. So per (language, extractor) combo we emit `(N_buckets + 1) + 1 + 1` Prometheus series for a histogram, not `N_buckets`.
**How to avoid:** Use the formula `series_per_combo = (N_explicit_buckets + 1) + 2` and `total ≤ N_languages × N_extractors × series_per_combo`.
**Concrete numbers (D-06):**
- 11 explicit buckets → 12 with `+Inf` → 14 series per combo (incl. `_count`, `_sum`).
- N_extractors = 3.
- Worst case: `52 languages × 3 extractors × 14 = 2,184` series for `helix_repomap_extract_duration_seconds`.
- Practically much lower because `langregistry` ships only 52 entries and most never appear in cache misses.
- **CONTEXT.md `<specifics>` line 145 says "≤ 3 × N_languages × N_buckets". This undercounts by ~17% (it omits `_count` + `_sum`).** The cardinality assertion in tests should use the corrected formula.
**Warning signs:** A `len(mf.GetMetric())` comparison that "looks low" — for a histogram, that count returns label-combo count, NOT total series. The total series count comes from `dto.Metric.Histogram.Bucket` slice + 2.

### Pitfall 2: Counter cardinality assertion via `len(mf.GetMetric())`
**What goes wrong:** Counter vectors expose one `dto.Metric` per label-value tuple; histograms also expose one `dto.Metric` per tuple but each carries a `Histogram` payload with `len(buckets)+1` samples internally.
**Why it happens:** Operator instinct says "series = sample lines in the scrape output", but the lint test counts `dto.Metric` entries, which equals label-tuple count.
**How to avoid:** For the cardinality-bound assertion, count `len(mf.GetMetric())` and assert it ≤ `N_label_combos`. State explicitly in the test comment what is being bounded (label combos vs. scrape lines).
**Warning signs:** A test that primes 14 series via observation and expects 14 entries from `Gather()`.

### Pitfall 3: Forgetting to prime new vectors in the lint test
**What goes wrong:** `TestMetricsLabelsAllowlist` (`metrics_labels_test.go:89`) primes every existing vector before calling `Gather()` because empty CounterVec / HistogramVec emit no families. New vectors added but not primed silently pass the lint without ever being checked.
**How to avoid:** Every new vector added in Phase 53 must get a corresponding `WithLabelValues(...).Inc()` / `.Observe(...)` line inside `TestMetricsLabelsAllowlist` (mirror the existing lines 94-100). Adding a new vector and forgetting to prime it is a silent lint bypass.
**Warning signs:** The total number of primed vectors does not match the total number of `*prometheus.XxxVec` fields in `*Metrics`.

### Pitfall 4: HTTP transport instrumentation without a stable hook
**What goes wrong:** Instrumenting `transport="http"` in user code attempts to wrap something the SDK does not expose.
**Why it happens:** `mcpsdk.NewStreamableHTTPHandler` (`internal/mcp/server.go:184`) takes only a `getServer` callback and `*StreamableHTTPOptions`. The SDK's `streamable.go` (v1.5.0) creates session entries internally at line 522, and the only user-visible signal is the `getServer` closure firing on every request — NOT once per session. There is no `OnSessionStart` / `OnSessionEnd` callback.
**How to avoid:** See Open Question Q-1 below for three planner-decision options.
**Warning signs:** A plan that says "add the call in `HTTPHandler()`" — that function returns the SDK handler unchanged; it has nowhere to add a per-session hook.

### Pitfall 5: Mis-classifying edit-tool errors as `internal`
**What goes wrong:** Today `edit/tools.go` returns `errorResult(err.Error())` for almost every failure, surfacing the message but losing the typed category. Mapping these blindly to outcome buckets will collapse `no_match` / `validation_failed` / `ls_error` into a single `internal` bucket.
**How to avoid:** The classification must inspect the actual error/result before bucketing. Mapping table in this document's Edit-Tool Outcome Map (below).
**Warning signs:** A tool handler that always emits `outcome=internal` on the unhappy path.

### Pitfall 6: Strategy enum drift between code and decision doc
**What goes wrong:** CONTEXT D-11 says `strategy` ∈ `{exact, whitespace_normalized, indentation_flexible, ellipsis, none}` but `internal/fuzzy/types.go:14-28` defines `{exact, whitespace_normalized, indentation_flexible, failed}`. There is no `ellipsis` Strategy; ellipsis is an `Options.AllowEllipsis` flag, NOT a Strategy value. There IS a `StrategyFailed` value the doc omits.
**How to avoid:** The planner must reconcile this. Two options: (a) emit the enum that matches `fuzzy.Strategy` exactly + `none` (so `{exact, whitespace_normalized, indentation_flexible, failed, none}`), or (b) keep CONTEXT's enum and explicitly never emit `ellipsis`/`failed` despite the type system. Option (a) matches FUZZ-02 ("string values are part of the public contract: they are reported back to agents in tool responses ... and MUST NOT change without a coordinated update to downstream consumers" — `types.go:11-12`).
**Warning signs:** A test that asserts `strategy="ellipsis"` is emittable but `strategy="failed"` is rejected — that contradicts the source of truth.

## Runtime State Inventory

> Phase is greenfield instrumentation, not a rename/refactor/migration. No
> stored data, OS-registered state, secrets, or build artifacts carry stale
> identifiers. Skipping per the runbook.

## Code Examples

### Example 1: lspool lookup emission at AcquireLease (D-02)
```go
// internal/kernel/lspool/pool.go (modified region around line 108-147)
func (p *Pool) AcquireLease(ctx context.Context, sessionID string, wsKey workspace.WorkspaceKey, dirty bool) (*WorkerLease, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if !dirty {
        if w := p.workerForKeyLocked(wsKey); w != nil {
            lease := NewWorkerLease(sessionID, w, false)
            p.leases[sessionID] = lease
            p.logger.Info("shared lease acquired", "session", sessionID, "worker", w.ID())
            p.metrics.LSPoolLookup(wsKey.Language, LookupHit) // NEW
            return lease, nil
        }
    }

    // Spawn path — counts as miss regardless of spawn success/failure.
    p.metrics.LSPoolLookup(wsKey.Language, LookupMiss) // NEW

    cb := p.circuitForLanguage(wsKey.Language)
    if !cb.CanAttempt() {
        return nil, cb.CircuitOpenErr()
    }
    if len(p.workers) >= p.config.MaxWorkers {
        return nil, ErrMaxWorkersReached
    }
    worker, err := p.spawnWorkerLocked(ctx, wsKey)
    if err != nil {
        cb.RecordFailure()
        return nil, fmt.Errorf("spawning worker: %w", err)
    }
    cb.RecordSuccess()
    // ... rest unchanged
}
```

### Example 2: repomap extract emission at GetOrExtract (D-03 + D-05)
```go
// internal/repomap/cache.go (modified region around line 60-103)
// TagCache gains an `metrics MetricsSink` field; populated via setter or constructor option.

func (c *TagCache) GetOrExtract(filePath string, extractFn func() ([]Tag, error)) ([]Tag, error) {
    info, err := os.Stat(filePath)
    if err != nil {
        return nil, fmt.Errorf("stat %s: %w", filePath, err)
    }
    mtime := info.ModTime().UnixNano()

    lang := LangFromExt(filePath) // existing func in this package; see render.go:124

    c.mu.Lock()
    var cachedMtime int64
    err = c.db.QueryRow(
        "SELECT mtime_ns FROM file_tags WHERE file_path = ? LIMIT 1",
        filePath,
    ).Scan(&cachedMtime)

    if err == nil && cachedMtime == mtime {
        tags, loadErr := c.loadTags(filePath)
        c.mu.Unlock()
        if loadErr != nil {
            return nil, fmt.Errorf("loading cached tags: %w", loadErr)
        }
        c.metrics.RepoMapLookup(lang, LookupHit) // NEW
        return tags, nil
    }
    c.mu.Unlock()

    c.metrics.RepoMapLookup(lang, LookupMiss) // NEW

    // D-05: time the extractor only on the miss path.
    extractor := classifyExtractor(c.metricsExtractorHint) // see Open Question Q-2
    start := time.Now()
    tags, err := extractFn()
    elapsed := time.Since(start).Seconds()
    c.metrics.RepoMapExtractObserve(lang, extractor, elapsed) // NEW

    if err != nil {
        return nil, err
    }
    // ... store branch unchanged
}
```

### Example 3: edit-tool outcome emission (D-10, D-11, D-16)
```go
// internal/kernel/edit/tools.go (modified registerReplaceBody region)
func registerReplaceBody(server *mcp.SerenaMCPServer, k *kernel.Kernel, extractor *BodyExtractor, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "replace_symbol_body",
        Description: "...",
    }, kernel.WrapToolSpan(tracer, "replace_symbol_body", func(ctx context.Context, req *mcpsdk.CallToolRequest, args ReplaceBodyArgs) (*mcpsdk.CallToolResult, any, error) {
        outcome, strategy := outcomeSuccess, strategyNone
        defer func() { mcp.RecordEditOutcome(ctx, "replace_symbol_body", outcome, strategy) }()

        // ... existing arg validation and lease setup; on each errorResult set outcome accordingly:
        // - missing field           → outcome = "internal" (or new "invalid_args" — see Open Question Q-3)
        // - acquire session error   → outcome = "ls_error"
        // - PlanEdit error          → outcome = "no_match" if symbol-not-found (need typed err); else "internal"
        // - fuzzy.Match error       → if errors.Is(err, ambiguous) → "ambiguous_match"; else "no_match"
        // - VerifyEdit reports HasErrors → "validation_failed"
        // ...

        if fuzzyInfo != nil {
            strategy = string(fuzzyInfo.Strategy) // exact / whitespace_normalized / indentation_flexible
        }
        return textResult(text), nil, nil
    }))
}
```

### Example 4: cardinality bound test
```go
// internal/obs/metrics_labels_test.go (new test alongside TestMetricsLabelsAllowlist)
func TestMetrics_CardinalityBounds_LSPoolLookups(t *testing.T) {
    m := newMetrics()
    // Prime label-name shape so the family appears in Gather().
    m.LSPoolLookups.WithLabelValues("go", "hit").Inc()

    // Simulate worst-case emission across the AllowedLabels bound.
    for _, lang := range []string{"go", "python", "typescript", "rust", "java"} {
        for _, result := range []string{"hit", "miss"} {
            m.LSPoolLookups.WithLabelValues(lang, result).Inc()
        }
    }

    mfs, err := m.Registry().Gather()
    if err != nil {
        t.Fatalf("Gather: %v", err)
    }
    var family *dto.MetricFamily
    for _, mf := range mfs {
        if mf.GetName() == "helix_lspool_lookups_total" {
            family = mf
            break
        }
    }
    if family == nil {
        t.Fatal("helix_lspool_lookups_total not registered")
    }
    // 5 languages × 2 results = 10 label combos. Plus the seed "go,hit" already counted.
    // Cardinality bound for the family is 2 × N_languages_observed_at_runtime.
    // Closed-enum {hit,miss} caps the result dimension to 2; language is unbounded
    // by the lint but bounded in practice by the langregistry (52 languages).
    if got := len(family.GetMetric()); got > 2*52 {
        t.Errorf("helix_lspool_lookups_total exceeded language×2 bound: %d", got)
    }
}
```

## Edit-Tool Outcome Map

This table is the planner's source of truth for D-10 outcome bucketing across
all 7 tools. Source-of-truth columns are validated against the actual code,
not CONTEXT D-12 (see Correction C-1).

| Tool | Package | Handler return sites (file:line) | Strategy emission |
|------|---------|----------------------------------|-------------------|
| `replace_symbol_body` | `internal/kernel/edit/tools.go` | 228, 232, 236, 241, 248, 252, 257, 261, 269 | `fuzzyInfo.Strategy` if non-nil (line 265-267); else `none` |
| `insert_before_symbol` | `internal/kernel/edit/tools.go` | 281, 285, 289, 294, 300, 304, 309, 312, 316 | always `none` (no fuzzy) |
| `insert_after_symbol` | `internal/kernel/edit/tools.go` | 328, 332, 336, 341, 347, 351, 356, 359, 363 | always `none` (no fuzzy) |
| `rename_symbol` | `internal/kernel/edit/tools.go` | 375, 379, 384, 388, 394, 401 | always `none` (rename does not run fuzzy cascade); rename ALSO emits `helix_rename_strategy_total{strategy=lsp-native\|rust-client-side}` per Phase 47 D-07 — both families increment, see `tools.go:397` |
| `safe_delete_symbol` | `internal/kernel/edit/tools.go` | 413, 417, 422, 428, 432, 437, 441, 451, 455 | always `none` (no fuzzy) |
| `replace_in_file` | `internal/kernel/fileops/tools.go` | 366, 370, 374, 379, 393, 397, 401, 404 | `fResult.Strategy` if fuzzy-fallback (line 399-400); else `none` |
| `fuzzy_edit` | `internal/kernel/fileops/tools.go` | 416, 420, 424, 430, 435 | `result.Strategy` always (every call runs fuzzy.Match) |

**Tools NOT in scope (per CONTEXT D-12 + correction):**
- `verify_edit` (`internal/kernel/edit/tools.go:460`) is a read-only
  diagnostic; emitting `success/internal` for it would dilute the edit-outcome
  signal. Excluded by CONTEXT scope ("edit tools").
- Other `fileops` tools (`read_file`, `create_file`, `list_directory`,
  `find_files`, `search_in_files`) are not edit tools.

**Outcome classification rules (D-10):**

| Bucket | Source signal | Code reference |
|--------|--------------|----------------|
| `success` | Handler returns `textResult(...)` (no IsError) without taking any error branch | the bottom of each happy path |
| `no_match` | `fuzzy.ErrAmbiguous` is NOT in tree (see below). Specifically: `fuzzy.Match` returns an `serr.InvalidArgs`-typed error whose message starts with "no match" — see `internal/fuzzy/match.go:11-23`. PlanEdit symbol-not-found error category. | `fuzzy/match.go`, edit `PlanEdit` |
| `ambiguous_match` | `fuzzy.Match` returns `serr.InvalidArgs` whose message starts with "ambiguous match: search block matches" — see `internal/fuzzy/diff.go:15,28`. | `fuzzy/diff.go:15,28` |
| `validation_failed` | After `appendVerifyInfo`: `VerifyResult.HasErrors == true`. Currently embedded in the success text; classifier needs to inspect `vr` directly, not the formatted string. | `internal/kernel/edit/verify.go:11-16,42-48`; `tools.go:127` |
| `ls_error` | `rt.AcquireSession(...)` returns error wrapped in `serr.Internal` with cause "acquire session"; or `notifyDidChange` returns error; or any LS rename/edit-apply error. | `tools.go:248,257,309` and similar |
| `internal` | Catch-all when no other classifier matches; missing required field validation; unexpected errors. | each `errorResult(serr.New(serr.InvalidArgs, ...))` site |

**Important:** there is NO `fuzzy.ErrAmbiguous` named exported error today
— ambiguity is signalled by `serr.InvalidArgs` with a structured message via
`ambiguityError` (`internal/fuzzy/diff.go`). The classifier therefore must
either: (a) match on the message prefix `"ambiguous match: search block matches"`,
(b) introduce a new typed error/sentinel, or (c) check `serr.Code` plus a
detail flag. Option (b) is the cleanest and is the planner's discretion.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|---|---|---|---|
| `prometheus.DefBuckets` for sub-millisecond histograms | Custom buckets matching the operational latency band (D-06: 1ms→2.5s) | Phase 53 | Fixes resolution loss in `helix_repomap_extract_duration_seconds`; precedent already set by `helix_tool_duration_seconds` keeping `DefBuckets` for the broader MCP tool latency band where 5ms→10s is the right resolution. |
| Two-counter "hits + misses" shape | Single counter with `result={hit,miss}` carve-out | Phase 53 D-01 | Half the families, same operator query (PromQL ratio). Accepted-pattern in `prometheus/client_golang` ecosystem; precedent: `helix_lspool_evictions_total{reason}` (Phase 11 D-13). |

**Deprecated/outdated:**
- ROADMAP success-criterion-1 strings (`serena_*`) are stale post-Phase 52
  rename; D-13 removes them.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `langregistry` does not export a `DetectFromPath` (or `LanguageFromPath`) function — the actual language-from-path resolver in `internal/repomap/` is `repomap.LangFromExt(path)` (used at `skill.go:361` and `render.go:124`). | Architectural Responsibility Map; Code Examples | If a `DetectFromPath` exists elsewhere and is preferred, the repomap sink wiring should use it for label normalization instead. **VERIFIED via `grep -rn "^func " internal/langregistry/*.go` — no `Detect` function exists in `internal/langregistry/`. `LangFromExt` IS the canonical helper at the cache instrumentation site.** Downgraded from assumption to verified fact. |
| A2 | The histogram cardinality bound `≤ 3 × N_languages × N_buckets` in CONTEXT.md `<specifics>` line 145 is an undercount that omits `_count` and `_sum`. The corrected bound is `3 × N_languages × (N_buckets + 1 + 2)` for the per-series count visible in `Gather()` output, but `len(mf.GetMetric())` still equals `3 × N_languages` (one entry per label combo). | Common Pitfalls #1; Code Examples #4 | If the planner writes the assertion against the CONTEXT formula, the test will pass while incorrectly stating its bound. The fix is documentary, not behavioural — the test still functions. |
| A3 | The MCP SDK v1.5.0 (in use per `go.mod:14`) does NOT expose a per-session lifecycle hook on `StreamableHTTPHandler` for the v1.2 release window. SDK design comment at `streamable.go:213-217` explicitly TODOs this: *"investigate the best API for callers to configure their session lifecycle"*. | Common Pitfalls #4; Open Question Q-1 | If a hook DOES exist that I missed, `transport="http"` could be wired without the workarounds in Q-1. **VERIFIED via grep of the SDK v1.5.0 source: no `OnSession*`, no `Hook*`, no exported callback in `StreamableHTTPOptions` (`streamable.go:131-186`).** Downgraded from assumption to verified fact. |
| A4 | The CONTEXT D-12 list of 7 edit tools is incorrect: `delete_lines` does not exist in the codebase (no occurrence in `internal/kernel/`); `safe_delete_symbol` exists in `internal/kernel/edit/tools.go:407` and is presumably the intended target. | Edit-Tool Outcome Map; Correction C-1 | If the planner writes a task to "instrument `delete_lines`" the task will be unimplementable. Reconciliation MUST happen in the plan. |
| A5 | The CONTEXT D-11 strategy enum `{exact, whitespace_normalized, indentation_flexible, ellipsis, none}` mismatches the `fuzzy.Strategy` source of truth `{exact, whitespace_normalized, indentation_flexible, failed}`. | Common Pitfalls #6; Correction C-2 | Plans that emit `strategy="ellipsis"` will cite a value that does not appear in `fuzzy.Strategy`. Plans that ALWAYS map `StrategyFailed` to `outcome=no_match` and never emit it as a `strategy` value need explicit handling. |

## Open Questions

### Q-1 — How does `transport="http"` get wired given no SDK hook?

**What we know:** Three candidate seams exist.

1. **`getServer` callback** (`internal/mcp/server.go:184`). Fires on every HTTP request, not once per session. Can use the `Mcp-Session-Id` header to detect "first request from this session ID" via a `sync.Map[sessionID]bool` deduper. Workable for `phase=started`. NO seam for `phase=ended` (the SDK closes the session internally; `streamable.go:471-481` runs the cleanup callback in private code).
2. **HTTP middleware wrapping `mcpServer.HTTPHandler()`** at `daemon.go:556`. Wrap `mux.Handle("/mcp", d.mcpServer.HTTPHandler())` in an `http.Handler` that observes session-id deduper + response status. Same `started` capability as option 1, plus the wrapping handler can detect session terminations from `DELETE /mcp` requests (the spec's session-end signal — see `streamable.go` request handlers).
3. **Defer `transport="http"` to a later phase.** Emit `helix_session_lifecycle_total{transport="stdio"}` only in this phase; document that `http` will be added when the SDK exposes a hook.

**What's unclear:** Whether the v1.2 ship blocker is "five families exist" (then defer `http`) or "session lifecycle covers both transports" (then implement option 2).

**Recommendation:** **Option 2** for `started` (wrapping `http.Handler` watches `Mcp-Session-Id` first-seen) + `error` (status 5xx + first-seen). Best-effort for `ended` via observing `DELETE /mcp` and 404-on-stale. Document explicitly in USAGE.md that `transport="http"` `phase="ended"` is best-effort, not authoritative. This avoids deferring a labeled dimension across releases (which is harder than not labeling at all).

**Planner action:** Pick option 1 / 2 / 3 in plan-checking. Each option produces a different task list.

### Q-2 — How does `cache.go:GetOrExtract` know which extractor was used?

**What we know:** `extractFn` is opaque to the cache (it's an injected closure
from `skill.go:366-399` or `render.go:127-129`). The cache does not know
whether tree-sitter, LSP fallback, or no-extraction will be used — that's a
caller decision.

**What's unclear:** Two solutions:

1. **Closure exposes the extractor type.** Change `extractFn` signature to
   `func() (tags []Tag, extractor string, err error)`. Caller in `skill.go`
   returns `"treesitter"` / `"lsp"` / `"fallback"` based on which path it took.
2. **Cache instruments only, caller observes separately.** Cache emits
   `helix_repomap_lookups_total{language, result}` only. The caller
   (`skill.go`) emits `helix_repomap_extract_duration_seconds` from its own
   timing wrapper — it knows which extractor it picked.

**Recommendation:** **Option 2.** It keeps `cache.go` simple (lookup-only),
keeps the extractor-typing logic where it belongs (the dispatcher in
`skill.go:368-398`), and matches the D-15 sink interface as written
(`RepoMapExtractObserve(language, extractor, seconds)` is a method on
`MetricsSink` — which is callable from `skill.go` directly, not just
`cache.go`).

**Planner action:** Use option 2. Plans wire the sink into both `skill.go`
(dispatch+observe) and `cache.go` (lookup) — two call sites, same sink, single
NoopSink default.

### Q-3 — Does `outcome=invalid_args` belong in the edit-outcome enum?

**What we know:** D-10 locks the outcome enum to 6 values:
`{success, no_match, ambiguous_match, validation_failed, ls_error, internal}`.
There is no `invalid_args`. The existing tool-call counter has a TODO comment
(`middleware.go:102`) reserving it for v1.3.

**What's unclear:** Whether plans should bucket "missing required field"
errors as `internal` (to fit the locked enum) or whether D-10 implicitly
allows `invalid_args` as parity with `outcomeEnum`.

**Recommendation:** Map missing-field validations to `outcome=internal` to
respect the locked D-10 enum. Document in code that this is a known
under-classification scheduled for v1.3. The cardinality table stays at 6
outcomes × 7 tools × 5 strategies (or 4, post-correction) = 210 max.

### Q-4 — Should `safe_delete_symbol` be in scope?

**What we know:** CONTEXT D-12 omits `safe_delete_symbol` and includes the
non-existent `delete_lines`. The actual edit-tool surface includes
`safe_delete_symbol` (`internal/kernel/edit/tools.go:407`).

**Recommendation:** The phase goal ("Operators can observe ... edit-tool
outcomes") is best served by including `safe_delete_symbol` (a real edit
tool) and excluding `delete_lines` (which doesn't exist). Cardinality
remains identical: 7 tools.

**Planner action:** Confirm the corrected list in the first plan and update
USAGE.md accordingly.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | ✓ | (project Go version) | — |
| `prometheus/client_golang` | Vector construction | ✓ | already vendored | — |
| `prometheus/client_model/go` | Test gather walk | ✓ | already vendored | — |
| `stretchr/testify/assert` | Test assertions | ✓ | already vendored | — |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package + `stretchr/testify/assert` |
| Config file | none (Go convention) |
| Quick run command | `go test ./internal/obs/... ./internal/kernel/lspool/... ./internal/repomap/... ./internal/mcp/... ./internal/kernel/edit/... ./internal/kernel/fileops/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OBS-03 | All 5 new metric families register on the obs.Metrics owned registry | unit | `go test ./internal/obs -run TestMetrics_RegisteredFamilies -v` | exists; EXTEND `want[]` |
| OBS-03 | New label carve-outs (`result`, `extractor`, `phase`, `transport`) accepted by lint | unit | `go test ./internal/obs -run TestMetricsLabelsAllowlist -v` | exists; EXTEND primed-vector list + `carveOuts` |
| OBS-03 | Lint still rejects unknown label names | unit | `go test ./internal/obs -run TestMetricsLabelsAllowlist_catchesDrift -v` | exists; no change needed (tests negative path on `helix_drift_test_total`) |
| OBS-03 | lspool emits `result=hit` on share path, `result=miss` on spawn path | unit | `go test ./internal/kernel/lspool -run TestPool_AcquireLease_LookupEmission -v` | ❌ Wave 0 |
| OBS-03 | repomap cache emits `result=hit` on mtime match, `result=miss` on extractFn path | unit | `go test ./internal/repomap -run TestTagCache_GetOrExtract_LookupEmission -v` | ❌ Wave 0 |
| OBS-03 | repomap extract histogram observes seconds with `extractor` label set to one of `{treesitter,lsp,fallback}` and rejects others | unit | `go test ./internal/repomap -run TestRepoMapExtractObserve -v` | ❌ Wave 0 |
| OBS-03 | session lifecycle counter emits `phase∈{started,ended,error}` × `transport∈{stdio,http}` from forwarder handler | unit | `go test ./internal/daemon -run TestForwarderHandler_SessionLifecycle -v` | ❌ Wave 0 |
| OBS-03 | edit outcome counter emits per-tool outcome with strategy = `none` for non-fuzzy tools, fuzzy.Strategy for fuzzy tools | unit | `go test ./internal/kernel/edit -run TestEditTools_OutcomeEmission -v && go test ./internal/kernel/fileops -run TestFileopsTools_OutcomeEmission -v` | ❌ Wave 0 |
| OBS-03 | mcp.RecordEditOutcome roundtrips through `setEditOutcomeSink` and increments the obs.Metrics vector | unit | `go test ./internal/mcp -run TestRecordEditOutcome -v` | ❌ Wave 0 |
| OBS-03 | Cardinality bound: lspool_lookups ≤ 2 × N_languages | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_LSPoolLookups -v` | ❌ Wave 0 |
| OBS-03 | Cardinality bound: repomap_lookups ≤ 2 × N_languages | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_RepoMapLookups -v` | ❌ Wave 0 |
| OBS-03 | Cardinality bound: repomap_extract_duration label-combo count ≤ 3 × N_languages (series count is `× (N_buckets+3)`) | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_RepoMapExtract -v` | ❌ Wave 0 |
| OBS-03 | Cardinality bound: session_lifecycle ≤ 3 × 2 = 6 | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_SessionLifecycle -v` | ❌ Wave 0 |
| OBS-03 | Cardinality bound: edit_outcome ≤ 7 × 6 × strategy_count = 168 (after correction; was 210) | unit | `go test ./internal/obs -run TestMetrics_CardinalityBounds_EditOutcome -v` | ❌ Wave 0 |
| OBS-03 | Noop-default invariant preserved: `obs.Noop(...)` Metrics() exposes all new helpers without panic | unit | `go test ./internal/obs -run TestMetrics_NoopProviderReturnsUsableSink -v` | exists; EXTEND helper-call list |
| OBS-03 | Compile-time assertion: `*obs.Metrics` satisfies new `repomap.MetricsSink` | unit | `go test ./internal/daemon -run TestObsMetricsIsRepoMapSink -v` | ❌ Wave 0 |
| OBS-03 | Compile-time assertion: `*obs.Metrics` satisfies extended `lspool.MetricsSink` (with new LSPoolLookup) | unit | `go test ./internal/daemon -run TestObsMetricsIsLSPoolSink -v` | exists; existing test must STILL pass after interface extension |
| OBS-03 | USAGE.md documents all 5 new metrics with labels and semantics | manual-only | grep validation in CI: `grep -q 'helix_lspool_lookups_total' USAGE.md && ... ` | ❌ Wave 0 (or accept manual review) |
| OBS-03 | ROADMAP.md success-criterion-1 uses `helix_*` not `serena_*` | manual-only | `! grep -q 'serena_lspool_cache_hits_total\|serena_repomap_cache_hits_total\|serena_repomap_extract_duration_seconds\|serena_session_lifecycle_total\|serena_edit_outcome_total' .planning/ROADMAP.md` | ❌ Wave 0 (CI-runnable shell check) |

### Sampling Rate
- **Per task commit:** `go test ./internal/obs/...` (≤ 5s)
- **Per wave merge:** `go test ./internal/obs/... ./internal/kernel/lspool/... ./internal/repomap/... ./internal/mcp/... ./internal/kernel/edit/... ./internal/kernel/fileops/... ./internal/daemon/...` (≤ 30s)
- **Phase gate:** `go vet ./... && go test ./...` green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/repomap/metrics.go` — NEW MetricsSink interface + NoopSink + constants
- [ ] `internal/repomap/metrics_test.go` — NEW emission + interface satisfaction tests
- [ ] `internal/kernel/lspool/metrics_test.go` — EXTEND `recordingSink` with `lookups []lookupEvent`; ADD `TestPool_AcquireLease_LookupEmission`
- [ ] `internal/kernel/edit/tools_test.go` — NEW; assert outcome+strategy emission for each of 5 handlers
- [ ] `internal/kernel/fileops/tools_test.go` — NEW; assert outcome+strategy for `replace_in_file` + `fuzzy_edit`
- [ ] `internal/mcp/middleware_test.go` — EXTEND with `TestRecordEditOutcome` parallel to existing rename tests (look for existing pattern at `middleware.go:38-44`)
- [ ] `internal/daemon/forwarder_test.go` (or extend `wiring_test.go`) — `TestForwarderHandler_SessionLifecycle`
- [ ] `internal/obs/metrics_labels_test.go` — EXTEND primed vectors in `TestMetricsLabelsAllowlist` + ADD 5 cardinality bound tests
- [ ] `internal/daemon/wiring_test.go` — ADD `var _ repomap.MetricsSink = (*obs.Metrics)(nil)` line + companion runtime test

## Sources

### Primary (HIGH confidence)
- `internal/obs/metrics.go` (full file read) — registry, vectors, helper-method conventions, `RenameStrategyInc` pattern (line 167-176)
- `internal/obs/metrics_labels_test.go` (full file read) — `carveOuts` map, `lintLabels` walk, drift test
- `internal/obs/metrics_test.go` (full file read) — `want[]` list to extend, primed-vectors pattern
- `internal/obs/obs.go` (full file read) — Noop guarantees
- `internal/kernel/lspool/metrics.go` (full file read) — `MetricsSink` interface, `EvictXxx` constants, `NoopSink`
- `internal/kernel/lspool/pool.go:108-147` — confirmed AcquireLease branches at lines 116 (hit), 130-141 (miss)
- `internal/kernel/lspool/metrics_test.go` (full file read) — `recordingSink` test pattern to copy for new sink
- `internal/repomap/cache.go` (full file read) — confirmed GetOrExtract branches at lines 76-84 (hit), 86-92 (miss), extractFn at 89
- `internal/skill/repomap/skill.go:344-399` — caller of `GetOrExtract`; this is where extractor-type discrimination happens (treesitter vs. fallback)
- `internal/repomap/render.go:117-135` — second caller of `GetOrExtract` (no extraction)
- `internal/fuzzy/types.go:11-28` — fuzzy.Strategy enum source of truth: `{StrategyExact, StrategyWhitespace, StrategyIndentationFlex, StrategyFailed}`
- `internal/fuzzy/match.go:1-40` — Match function entry; ambiguity error path
- `internal/fuzzy/diff.go:15,28` — ambiguity error format ("ambiguous match: search block matches %d locations")
- `internal/kernel/edit/tools.go` (full file read) — 6 tools: `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`, `verify_edit`. NO `replace_in_file` / `fuzzy_edit` / `delete_lines`.
- `internal/kernel/edit/replace.go:1-115` — fuzzy match site for replace_symbol_body
- `internal/kernel/edit/verify.go` (full file read) — `VerifyResult` type; HasErrors signal
- `internal/kernel/fileops/tools.go:359-438` — `replace_in_file` and `fuzzy_edit` tool registrations (the OTHER 2 of "7 edit tools")
- `internal/mcp/middleware.go` (full file read) — `renameStrategySink` setter at lines 18-44; `outcomeEnum` at 100-119; `InstallMiddleware` wiring at 75-86
- `internal/mcp/server.go:182-187` — HTTPHandler wraps `mcpsdk.NewStreamableHTTPHandler` with no session hooks
- `internal/daemon/daemon.go` lines 188-191, 280-307, 325-367, 553-622 — kernel construction; post-init wiring; HTTP listener; forwarder handler
- `internal/daemon/wiring_test.go` (full file read) — compile-time assertion pattern to mirror
- `internal/langregistry/registry.go` (full file read) — confirmed NO `DetectFromPath` exists; `Get(language)` and `ByExtension(ext)` are the public path-related helpers
- `internal/repomap/render.go:124` and `internal/skill/repomap/skill.go:361` — `repomap.LangFromExt(path)` is the canonical language-from-path resolver
- `.planning/REQUIREMENTS.md:36` — OBS-03 verbatim
- `.planning/ROADMAP.md:183-193` — Phase 53 success criteria with stale `serena_*` strings
- `.planning/phases/53-obs-metrics-gaps/53-CONTEXT.md` — locked decisions D-01..D-17
- `USAGE.md:634-678` — current Prometheus Metrics section to extend
- `go.mod:14` — `github.com/modelcontextprotocol/go-sdk v1.5.0`

### Secondary (MEDIUM confidence)
- `/Users/Janis_Vizulis/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.5.0/mcp/streamable.go:51-228` — SDK source for `StreamableHTTPHandler`; confirms no public session-lifecycle hook exists in this version. The TODO at lines 213-217 is the maintainer's own note that the API surface for session lifecycle config has not been designed.

### Tertiary (LOW confidence)
- None. All claims in this research are tagged `[VERIFIED]` against in-tree code or `[CITED]` against CONTEXT.md / SDK source.

## Corrections to CONTEXT.md (planner must reconcile in plans)

The following are documented divergences between CONTEXT.md and code reality.
Each is verifiable by an SMTC / grep query against this repo at HEAD.

### C-1 — Edit tool list (D-12) is wrong

**CONTEXT D-12 says:** "Tool scope: `replace_in_file`, `replace_symbol_body`,
`fuzzy_edit`, `insert_after_symbol`, `insert_before_symbol`, **`delete_lines`**,
`rename_symbol`."

**Code reality:**
- `delete_lines` does **not** exist (verified: `grep -rn 'delete_lines' --include='*.go'` returns nothing in `internal/`).
- `safe_delete_symbol` exists at `internal/kernel/edit/tools.go:407`.
- The 7 edit tools span TWO packages:
  - `internal/kernel/edit/tools.go`: `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol` (5 tools)
  - `internal/kernel/fileops/tools.go`: `replace_in_file`, `fuzzy_edit` (2 tools)

**Action:** Plans must instrument the corrected 7-tool list. Cardinality unchanged (still 7). USAGE.md table must list the corrected names.

### C-2 — Strategy enum (D-11) is wrong

**CONTEXT D-11 says:** `strategy ∈ {exact, whitespace_normalized, indentation_flexible, ellipsis, none}`.

**Code reality (`internal/fuzzy/types.go:14-28`):** `fuzzy.Strategy` has four values: `{exact, whitespace_normalized, indentation_flexible, failed}`. There is NO `ellipsis`. `ellipsis` is an `Options.AllowEllipsis` flag (`types.go:38`), not a Strategy.

**Action:** Plans must align the emitted enum with `fuzzy.Strategy`'s actual values. Recommended emission set:

```
strategy ∈ {exact, whitespace_normalized, indentation_flexible, failed, none}
```

with cardinality `5`. The bound calculation becomes `7 × 6 × 5 = 210` (unchanged from D-12 wording). If the planner instead chooses to never emit `failed` (mapping `StrategyFailed` paths to `outcome=no_match` with `strategy=none`), the practical strategy enum becomes 4 values and the bound is `7 × 6 × 4 = 168`. Either is valid; **pick one explicitly in the first plan**.

### C-3 — HTTP transport boundary (D-09) has no SDK hook

**CONTEXT D-09 says:** "the planner identifies the matching session-create / session-end boundary in the HTTP path during research".

**Research finding:** No such boundary exists in user code today. The MCP SDK
v1.5.0 (`go.mod:14`) uses internal-only session lifecycle in
`streamable.go:471-481` and exposes no `OnSessionStart` / `OnSessionEnd`
callbacks via `StreamableHTTPOptions` (struct read in full at lines 131-186).

**Action:** Planner picks one of the three Q-1 options. Recommended: **Option 2** (HTTP middleware wrapping `mcpServer.HTTPHandler()` at `daemon.go:556`). Best-effort `phase=ended` documented in USAGE.md.

### C-4 — Histogram cardinality formula in `<specifics>` is an undercount

**CONTEXT `<specifics>` line 145 says:** "repomap_extract_duration ≤ 3 × N_languages × N_buckets".

**Correction:** That counts buckets only; Prometheus emits `_count` and `_sum` series per label combo, plus the `+Inf` bucket. The total scraped-line count per family is `3 × N_languages × (N_buckets + 1 + 2)`. The `len(mf.GetMetric())` count from `Gather()`, however, returns the per-label-combo count, which IS `3 × N_languages`. So cardinality TESTS will use `3 × N_languages` as the assertion target while documentation should communicate the true scraped-series upper bound.

**Action:** Document both numbers in USAGE.md and test code; do not bury the distinction.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all patterns proven by Phase 11 / Phase 47.
- Architecture: HIGH — every wiring decision (D-14 through D-17) has a working in-tree precedent that I traced end-to-end.
- Pitfalls: HIGH — pitfalls #1–#3 and #5 are mechanical; pitfall #4 (HTTP) is verified by SDK source read; pitfall #6 (strategy enum drift) is verified by typing comparison.
- Open questions: MEDIUM — Q-1 has three viable options none of which I can pick without product-direction input from the user/planner; Q-2 has a clear recommendation; Q-3 hews to D-10's locked enum; Q-4 is mechanical reconciliation.

**Research date:** 2026-04-30
**Valid until:** 2026-05-30 (30 days; in-tree code references will drift if other phases land first; SDK version pin is firm)
