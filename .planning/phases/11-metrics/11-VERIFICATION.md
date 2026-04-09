---
phase: 11-metrics
verified: 2026-04-08T00:00:00Z
status: passed
score: 14/14 must-haves verified
overrides_applied: 0
---

# Phase 11: Metrics Verification Report

**Phase Goal:** Expose Prometheus-scrapeable RED metrics for tools and lspool health with a bounded-label contract that cannot silently explode cardinality

**Verified:** 2026-04-08
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

Merged from ROADMAP success criteria + four PLAN frontmatter `must_haves.truths` blocks.

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | Operator can scrape `/metrics` on the admin listener and see RED histograms (rate, errors, duration), lspool gauges, and Go runtime collectors (ROADMAP SC-1) | VERIFIED | `internal/daemon/telemetry.go:70-78` mounts `/metrics` via `promhttp.HandlerFor(d.obs.Metrics().Registry(), ...)`. `TestAdmin_MetricsEndpoint` in `internal/daemon/telemetry_metrics_test.go` asserts 200 + body contains all 6 `serena_*` families + `go_goroutines` + `process_resident_memory_bytes` |
| 2  | Tool latency histograms use buckets that resolve p50/p95/p99 for the Phase 9 distribution (ROADMAP SC-2) | VERIFIED | `internal/obs/metrics.go:66` uses `prometheus.DefBuckets` (.005..10s, 11 buckets). CONTEXT.md D-01/D-02 documents this as covering 95% of the Phase 9 distribution; per-category tuning deferred to v1.3 with explicit revisit criterion |
| 3  | CI lint rejects any new metric whose label set falls outside the allowlist `{tool_name, profile, mode, language, outcome}` (ROADMAP SC-3) | VERIFIED | `internal/obs/metrics_labels_test.go` defines `lintLabels` walking every metric family and asserting label names are in `AllowedLabels` or per-family carve-outs. `TestMetricsLabelsAllowlist_catchesDrift` is a negative proof using a `user_id` label |
| 4  | Enabling metrics produces no regression beyond the Phase 10 delta gate (ROADMAP SC-4) | VERIFIED | benchgate exit 0 vs `v1.2-phase10-github-hosted.txt` (documented in baseline header). Slog hot-path benchmarks remain at 0 allocs/op. TelemetryMiddleware delta = +1 alloc/op vs in-run BenchmarkBaselineMiddleware (budget +3) |
| 5  | Operator can scrape `/metrics` and receive 200 with Prometheus exposition format (PLAN 11-01) | VERIFIED | Same as Truth 1; `TestAdmin_MetricsEndpoint` checks status 200 and Content-Type `text/plain` |
| 6  | Response contains `go_goroutines`, `go_gc_*`, `process_resident_memory_bytes` runtime collectors (PLAN 11-01) | VERIFIED | `internal/obs/metrics.go:110-111` registers `collectors.NewGoCollector()` and `collectors.NewProcessCollector(...)`. Telemetry test asserts presence in body |
| 7  | Every registered `*Vec` uses label names from the 5-element allowlist (PLAN 11-01) | VERIFIED | `TestMetricsLabelsAllowlist` green; carve-out for `serena_lspool_evictions_total.reason` is documented inline in `metrics.go:82-85` per D-13 (closed 4-value enum) |
| 8  | `obs.Noop(...)` returns a Provider whose `Metrics()` accessor returns a non-nil sink (PLAN 11-01) | VERIFIED | `internal/obs/obs.go` Noop constructs `newMetrics()`; daemon `wiring_test.go` `TestObsMetricsIsLSPoolSink` asserts sink is non-nil |
| 9  | Every MCP `tools/call` produces exactly one increment on `serena_tool_calls_total` and one observation on `serena_tool_duration_seconds` (PLAN 11-02) | VERIFIED | `internal/mcp/middleware.go:111-158` calls `m.ToolCalls.WithLabelValues(...).Inc()` and `m.ToolDuration.WithLabelValues(...).Observe(duration.Seconds())` only when `method == "tools/call"`. `TestTelemetryMiddleware_toolsCallEmitsMetric` asserts 1:1 |
| 10 | TelemetryMiddleware only instruments `tools/call`; other methods are pass-through (PLAN 11-02) | VERIFIED | `middleware.go:135` `if method != "tools/call" { return result, err }`. `TestTelemetryMiddleware_skipsNonToolsCall` and `TestTelemetryMiddleware_initializeMethod` assert no metric emission but logs preserved |
| 11 | TelemetryMiddleware subsumes loggingMiddleware (one closure for log + metric) (PLAN 11-02) | VERIFIED | `grep loggingMiddleware internal/mcp/middleware.go` returns no matches. `TestTelemetryMiddleware_loggingMiddlewareGone` enforces. Single closure performs both `logger.Info`/`Warn` and metric emission |
| 12 | All five labels resolve to constant strings on the hot path — no `fmt.Sprintf` (PLAN 11-02) | VERIFIED | `grep fmt.Sprintf internal/mcp/middleware.go` returns no matches. classifyOutcome uses string constants only; session snapshot reads pre-stored fields |
| 13 | classifyOutcome maps MCP errors to one of 7 closed enum values (PLAN 11-02) | VERIFIED | `middleware.go:46-65` declares 7 constants `{success, invalid_args, not_found, circuit_open, ls_crash, timeout, internal}`. `denied` absent. `TestClassifyOutcome` and `TestClassifyOutcome_AllSevenEnumValuesExist` cover positive + negative |
| 14 | lspool emits per-language worker / eviction / restart / circuit-state metrics; package does NOT import `internal/obs` (PLAN 11-03) | VERIFIED | `internal/kernel/lspool/metrics.go` defines `MetricsSink` interface; pool/circuit hooks emit on lifecycle events. `grep "internal/obs" internal/kernel/lspool/` returns only doc-comment references — zero import statements. `wiring_test.go` `var _ lspool.MetricsSink = (*obs.Metrics)(nil)` compile-time assertion locks the contract |

**Score:** 14/14 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/obs/metrics.go` | Metrics struct + Registry + 6 vectors + 4 LSPool* methods | VERIFIED | 149 lines; contains `prometheus.NewRegistry`, `AllowedLabels`, all 6 vectors, all 4 frozen LSPool* helper methods with verbatim signatures |
| `internal/obs/metrics_labels_test.go` | CI allowlist enforcement | VERIFIED | Contains `TestMetricsLabelsAllowlist`, `lintLabels`, `carveOuts`, and `TestMetricsLabelsAllowlist_catchesDrift` negative test using `user_id` |
| `internal/obs/metrics_test.go` | Metric construction tests | VERIFIED | 5 tests covering registry isolation, family enumeration, noop sink usability, multi-construction safety, allowlist shape |
| `internal/daemon/telemetry.go` | `/metrics` mounted via promhttp.HandlerFor | VERIFIED | Lines 70-78; conditional on `d.obs != nil` for test ergonomics; production path always populated |
| `internal/daemon/telemetry_metrics_test.go` | /metrics scrape + regression | VERIFIED | `TestAdmin_MetricsEndpoint` + `TestAdmin_MetricsRegressionHealthReadyz` |
| `internal/mcp/middleware.go` | TelemetryMiddleware + classifyOutcome + 7-enum | VERIFIED | TelemetryMiddleware function present (line 111); classifyOutcome present (line 75); 7 outcome constants; `loggingMiddleware` removed |
| `internal/mcp/telemetry_middleware_test.go` | 10 tests covering classification + behavior | VERIFIED | All tests pass under `go test -race` |
| `internal/mcp/session.go` | Language field + SetLanguage + Snapshot copy | VERIFIED | Field, setter, and snapshot copy all present |
| `internal/kernel/lspool/metrics.go` | MetricsSink interface + NoopSink + enum constants | VERIFIED | 63 lines; interface, NoopSink value-type, EvictXxx + CircuitXxx constants, compile-time NoopSink assertion |
| `internal/kernel/lspool/metrics_test.go` | Lifecycle hook tests | VERIFIED | Recording sink + tests for spawn/evict/restart/circuit transitions |
| `internal/daemon/wiring_test.go` | Compile-time obs.Metrics ↔ lspool.MetricsSink assertion | VERIFIED | `var _ lspool.MetricsSink = (*obs.Metrics)(nil)` at line 17 |
| `test/bench/metrics_bench_test.go` | BenchmarkTelemetryMiddleware + Case-B baseline bench | VERIFIED | Three benchmarks (Baseline, Telemetry, Telemetry_ToolsList) using `b.Loop()` and `b.ReportAllocs()` |
| `test/bench/baselines/v1.2-phase11-github-hosted.txt` | Phase 11 baseline with provenance header | VERIFIED | Header has date / commit / go version / runner; documents +1 alloc delta vs in-run baseline; benchgate exit 0 noted |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/daemon/telemetry.go` | `internal/obs/metrics.go` | `d.obs.Metrics().Registry()` → `promhttp.HandlerFor` | WIRED | telemetry.go:71-76 |
| `internal/obs/metrics.go` | `prometheus.NewRegistry` | owned non-global registry | WIRED | metrics.go:51 — `reg := prometheus.NewRegistry()`; no `DefaultRegisterer` references |
| `internal/mcp/server.go` | `internal/mcp/middleware.go` | `InstallMiddleware` installs TelemetryMiddleware | WIRED | middleware.go:30 `server.AddReceivingMiddleware(TelemetryMiddleware(...))` |
| `internal/mcp/middleware.go` | `internal/obs/metrics.go` | `provider.Metrics().ToolCalls / ToolDuration` | WIRED | middleware.go:111-158 — closure captures `m := provider.Metrics()` |
| `internal/kernel/lspool/pool.go` | `internal/kernel/lspool/metrics.go` | `sink.LSPoolWorkersSet` etc. on lifecycle events | WIRED | Spawn/evict/restart/shutdown call sites confirmed in 11-03 SUMMARY hook table |
| `internal/daemon/daemon.go` | `internal/kernel/lspool/pool.go` | `kernel.NewKernel(..., obs.Metrics())` threading sink to NewPool | WIRED | wiring_test.go assertion + obs provider construction reordered before kernel |
| `test/bench/metrics_bench_test.go` | `internal/mcp/middleware.go` | constructs `TelemetryMiddleware` + drives tools/call | WIRED | Bench file contains `TelemetryMiddleware` and `b.Loop()` |
| `test/bench/baselines/v1.2-phase11-...txt` | `test/bench/baselines/v1.2-phase10-...txt` | benchgate comparison | WIRED | Header documents benchgate invocation with exit 0 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `/metrics` HTTP response | exposition body | `d.obs.Metrics().Registry()` → promhttp gathers all 8 metric families | YES — production daemon registers real `obs.Provider` via `obs.Noop(logger.Handler())` in `daemon.New`; TelemetryMiddleware writes on every tools/call; lspool hooks write on lifecycle events | FLOWING |
| `serena_tool_calls_total` | counter cell | TelemetryMiddleware closure-captured `m := provider.Metrics()` calls `.WithLabelValues(...).Inc()` per tools/call | YES — middleware installed via `InstallMiddleware` in daemon, real provider, real session snapshot | FLOWING |
| `serena_lspool_workers` | gauge cell | Pool spawn/destroy/idle/evict/shutdown call sites call `p.metrics.LSPoolWorkersSet(lang, ±1)` | YES — daemon wires `obs.Metrics()` into kernel/pool at construction | FLOWING |
| `serena_lspool_circuit_state` | gauge cell | Circuit `setStateLocked` calls `cb.sink.LSPoolCircuitStateSet(language, state)` | YES — circuit gained language field; sink threaded through pool→circuit | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase 11 packages compile clean | `go vet ./internal/obs/... ./internal/mcp/... ./internal/kernel/lspool/... ./internal/daemon/...` | clean | PASS |
| Phase 11 unit tests pass | `go test -count=1 ./internal/obs/... ./internal/mcp/... ./internal/kernel/lspool/... ./internal/daemon/...` | 4 packages OK | PASS |
| MetricsSink contract holds at compile time | `var _ lspool.MetricsSink = (*obs.Metrics)(nil)` in wiring_test.go | builds | PASS |
| Hot-path no `fmt.Sprintf` | `grep fmt.Sprintf internal/mcp/middleware.go` | no matches | PASS |
| `loggingMiddleware` deleted | `grep loggingMiddleware internal/mcp/middleware.go` | no matches | PASS |
| `denied` outcome absent from enum | `grep '"denied"' internal/mcp/middleware.go` | no matches | PASS |
| lspool decoupled from obs | `grep '"github.com/postfix/serena/internal/obs"' internal/kernel/lspool/` | no import statements (only doc comments) | PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| METRIC-01 | 11-01 | `/metrics` endpoint on admin listener (Prometheus format) | SATISFIED | `internal/daemon/telemetry.go:70-78` mounts via promhttp.HandlerFor; `TestAdmin_MetricsEndpoint` asserts 200 + format |
| METRIC-02 | 11-02, 11-04 | RED histograms per tool with tuned buckets | SATISFIED | `serena_tool_calls_total` + `serena_tool_duration_seconds` emitted by TelemetryMiddleware; DefBuckets per D-01 covers Phase 9 distribution; alloc budget verified by BenchmarkTelemetryMiddleware (+1 alloc/op vs +3 budget) |
| METRIC-03 | 11-03 | lspool gauges (workers, evictions, restarts, circuit state) | SATISFIED | All 4 vectors registered in `internal/obs/metrics.go`; `MetricsSink` interface + lifecycle hooks in `internal/kernel/lspool/`; daemon wiring locked by compile-time assertion |
| METRIC-04 | 11-01 | Go runtime collectors (goroutines, GC, memory) | SATISFIED | `collectors.NewGoCollector()` + `collectors.NewProcessCollector()` registered at `internal/obs/metrics.go:110-111`; presence asserted in admin listener test |
| METRIC-05 | 11-01 | Bounded-label contract enforced by CI lint | SATISFIED | `AllowedLabels` 5-element array; `TestMetricsLabelsAllowlist` walks registry; `TestMetricsLabelsAllowlist_catchesDrift` negative proof |

No orphaned requirements: REQUIREMENTS.md maps METRIC-01..05 to Phase 11 and all five appear in PLAN frontmatter `requirements:` fields.

### Anti-Patterns Found

None. Scans for `TODO`, `FIXME`, `placeholder`, `not yet implemented`, hardcoded empty returns, and `fmt.Sprintf` on the hot path returned no blockers in the modified files. The `invalid_args / not_found / ls_crash` outcome enum values are documented as "v1.3 — pending typed errors from kernel" in `middleware.go` comments — this is a documented forward-compat slot, not a stub.

### Human Verification Required

None. All success criteria verified programmatically:
- HTTP scrape behavior is asserted by `TestAdmin_MetricsEndpoint` against a real listener.
- Label allowlist drift detection is asserted by `TestMetricsLabelsAllowlist_catchesDrift`.
- Allocation budget is asserted by `BenchmarkTelemetryMiddleware` with documented +1/+3 alloc delta.
- benchgate vs Phase 10 baseline returned exit 0 (documented in baseline header).

There is no UX or visual surface to inspect; the entire phase output is machine-consumable Prometheus exposition + tests.

### Gaps Summary

No gaps. Phase 11 achieves the goal: `/metrics` is scrapeable on the loopback admin listener, exposes RED histograms per tool plus lspool gauges plus Go/Process runtime collectors, the label set is bounded by a CI-enforced 5-element allowlist with a single documented carve-out for `serena_lspool_evictions_total.reason`, the hot-path alloc budget (≤ +3 allocs/op vs Phase 10) is met empirically (+1 alloc/op), and benchgate at PR tier exits 0 against the Phase 10 baseline.

The DefBuckets choice (vs explicit "SLO-tuned") is a deliberate in-phase decision (CONTEXT.md D-01/D-02): the default 11-bucket set covers 95% of the Phase 9 distribution and resolves p50/p95/p99 cleanly; per-category tuning is deferred to v1.3 with an explicit revisit criterion. This satisfies SC-2's intent ("resolves p50/p95/p99 for the actual response-time distribution captured in Phase 9").

---

_Verified: 2026-04-08_
_Verifier: Claude (gsd-verifier)_
