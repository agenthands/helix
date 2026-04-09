---
phase: 11-metrics
plan: 01
subsystem: observability
tags: [metrics, prometheus, admin-listener, red-metrics, lspool]
requirements: [METRIC-01, METRIC-04, METRIC-05]
dependency-graph:
  requires:
    - Phase 10 obs.Provider + admin listener (internal/obs/, internal/daemon/telemetry.go)
  provides:
    - obs.Metrics type with pre-registered Prometheus vectors on an owned registry
    - AllowedLabels [5]string frozen allowlist + CI label-lint test
    - /metrics endpoint on the admin listener (loopback-only)
    - Frozen lspool sink method signatures consumed by plan 11-03
  affects:
    - Plan 11-02 (TelemetryMiddleware) consumes m.ToolCalls + m.ToolDuration
    - Plan 11-03 (lspool gauges) consumes the 4 LSPool* helper methods
tech-stack:
  added:
    - github.com/prometheus/client_golang v1.23.2 (sole new direct dep for phase 11)
    - transitive: prometheus/common, prometheus/procfs, prometheus/client_model, go.yaml.in/yaml/v2
  patterns:
    - Pre-registered vectors pattern (RESEARCH.md Pattern 1)
    - Owned *prometheus.Registry per Provider (no globals)
    - Noop-default sink (call sites never nil-check)
key-files:
  created:
    - internal/obs/metrics.go
    - internal/obs/metrics_test.go
    - internal/obs/metrics_labels_test.go
    - internal/daemon/telemetry_metrics_test.go
  modified:
    - internal/obs/obs.go (Provider gains metrics field + Metrics() accessor)
    - internal/daemon/daemon.go (Daemon gains obs field, constructed via obs.Noop)
    - internal/daemon/telemetry.go (mux.Handle /metrics via promhttp.HandlerFor)
    - go.mod
    - go.sum
decisions:
  - Exposed Metrics vector fields (ToolCalls, ToolDuration, LSPool*) rather than private + getters — middleware (plan 02) is in a different package and will need direct access; CI lint still enforces labels at the registry level so visibility does not loosen the contract
  - newMetrics() lives inside obs package (unexported) so plan 02/03 cannot bypass Provider.Metrics()
  - Added a compile-time assertion on prometheus/client_model via a package-level var in metrics_labels_test.go so the dto import is tracked (refactor-proof)
metrics:
  duration: 12min
  completed: 2026-04-10
---

# Phase 11 Plan 01: Metrics Scaffolding Summary

Stand up the `internal/obs/` metrics scaffold with pre-registered Prometheus vectors, enforce the bounded-label allowlist via CI test, and mount `/metrics` on the Phase 10 admin listener. Noop path remains zero-dep for tests that do not want metrics.

## What Was Built

### Task 1 — obs.Metrics with owned registry (commit d43d0480)

Added `github.com/prometheus/client_golang@v1.23.2` as the sole new direct dependency for phase 11, then created `internal/obs/metrics.go` with:

- **`AllowedLabels [5]string`** — immutable array `{"tool_name", "profile", "mode", "language", "outcome"}` (D-04).
- **`Metrics` struct** holding the six Serena-owned vectors:
  - `ToolCalls *CounterVec{tool_name, profile, mode, language, outcome}` — D-10
  - `ToolDuration *HistogramVec{tool_name, profile, mode, language}` using `prometheus.DefBuckets` — D-01, D-11
  - `LSPoolWorkers *GaugeVec{language}` — D-12
  - `LSPoolEvictions *CounterVec{language, reason}` — D-13
  - `LSPoolCircuitState *GaugeVec{language}` — D-14
  - `LSPoolRestarts *CounterVec{language}` — D-15
- **`newMetrics()`** — constructs a fresh `prometheus.NewRegistry()` per call and registers all six vectors plus `collectors.NewGoCollector()` and `collectors.NewProcessCollector(...)` (D-16, METRIC-04). The prometheus global registerer is never touched (T-11-05 mitigation).
- **Four lspool sink helper methods** with signatures frozen for plan 11-03:
  - `func (m *Metrics) LSPoolWorkersSet(language string, delta float64)`
  - `func (m *Metrics) LSPoolEviction(language, reason string)`
  - `func (m *Metrics) LSPoolCircuitStateSet(language string, state float64)`
  - `func (m *Metrics) LSPoolRestart(language string)`

Extended `internal/obs/obs.go`:
- `Provider` gains a `metrics *Metrics` field.
- `Noop(inner slog.Handler)` now constructs `newMetrics()` so the noop path still gets a working (but unscraped) sink. Call sites never branch on nil.
- Added `Provider.Metrics() *Metrics` accessor.

Created `internal/obs/metrics_test.go` with five tests:
- `TestMetrics_NewMetricsReturnsIsolatedRegistry` — registry non-nil and not the global registerer
- `TestMetrics_RegisteredFamilies` — all 8 metric families reach Gather()
- `TestMetrics_NoopProviderReturnsUsableSink` — exercises all vectors through the noop provider
- `TestMetrics_MultipleConstructionNoPanic` — proves each call gets a fresh registry (no double-reg)
- `TestMetrics_AllowedLabelsShape` — locks the 5-element array shape

### Task 2 — Label allowlist CI lint (commit 3d076596)

Created `internal/obs/metrics_labels_test.go` with:

- **`lintLabels(t, gatherer)`** — walks every metric family in a registry, skips `go_*` and `process_*` runtime families (not Serena-owned), and for each remaining family asserts every label name is in `AllowedLabels` OR the per-family `carveOuts` map.
- **`carveOuts`** — currently a single entry: `serena_lspool_evictions_total` allows the `reason` label. Documented inline with a block comment referencing CONTEXT.md D-13: reason is a closed 4-value enum (`idle/pressure/crash/shutdown`) enforced at the emission site.
- **`TestMetricsLabelsAllowlist`** — primes every Serena vector and runs lintLabels on the real registry. Green today; breaks loudly if any future plan adds a forbidden label.
- **`TestMetricsLabelsAllowlist_catchesDrift`** — the negative proof. Registers a temporary `serena_drift_test_total{user_id}` counter on a throwaway registry and asserts `lintLabels()` flags it. If this test ever silently passes, the lint is broken.

The negative test also asserts the problem message contains both the metric name and the label name so CI failures are actionable without a debugger.

### Task 3 — /metrics on admin listener (commit 18bdd85d)

Modified `internal/daemon/daemon.go`:
- Added `obs *obs.Provider` field to the `Daemon` struct.
- `New()` now constructs `obs.Noop(logger.Handler())` and stores it on the daemon. A single Provider owns both the Phase 10 trace-aware slog handler and the Phase 11 Prometheus sink.

Modified `internal/daemon/telemetry.go`:
- Added `github.com/prometheus/client_golang/prometheus/promhttp` import.
- In `listenAdmin`, after the `EnablePprof` block, register `/metrics` unconditionally (independent of pprof — metrics is a separate opt-in surface gated only by `AdminAddr`):
  ```go
  mux.Handle("/metrics", promhttp.HandlerFor(
      d.obs.Metrics().Registry(),
      promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError},
  ))
  ```
- Nil-guarded `d.obs` so the existing Phase 10 `minimalDaemon` test helper (which doesn't set `obs`) keeps working for the /healthz + /readyz unit tests.

Created `internal/daemon/telemetry_metrics_test.go`:
- **`TestAdmin_MetricsEndpoint`** — builds a Daemon with a populated `obs.Provider`, primes every vector, probes `/metrics`, asserts 200, `text/plain` Content-Type, all six `serena_*` families, `go_goroutines`, `process_resident_memory_bytes`, and verifies the primed `tool_name="read_file"` label round-trips through the exposition so we know the label resolution path works end-to-end.
- **`TestAdmin_MetricsRegressionHealthReadyz`** — Phase 10 regression guard: `/healthz`, `/readyz`, and `/metrics` all return 200 on the same listener after the new registration.

## Dependency Version

**`github.com/prometheus/client_golang v1.23.2`** (resolved 2026-04-10, matches the v1.23.x band specified in the phase constraints).

Transitive additions:
- `github.com/prometheus/common v0.66.1`
- `github.com/prometheus/procfs v0.16.1`
- `github.com/prometheus/client_model v0.6.2`
- `go.yaml.in/yaml/v2 v2.4.2`

## Allowlist Carve-out

`serena_lspool_evictions_total.reason` is carved out of the D-04 RED allowlist because it is an internal dimensional slice on lspool worker lifecycle, not a request-side label on tool calls. The enum is a closed 4-value set (`idle`, `pressure`, `crash`, `shutdown`) enforced at emission sites in plan 11-03, not at the scrape-time CI lint. The carve-out is documented inline in `metrics_labels_test.go` referencing CONTEXT.md D-13.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Missing Infra] Wired `obs.Provider` into the Daemon struct**

- **Found during:** Task 3
- **Issue:** The plan assumed `d.obs` already existed on `*Daemon`, but Phase 10 had installed the ContextHandler in `cli/root.go` to "avoid daemon.New signature churn" (per 10-01 decision log). There was no `d.obs` field to reference from `telemetry.go`.
- **Fix:** Added `obs *obs.Provider` field to the `Daemon` struct and constructed it via `obs.Noop(logger.Handler())` at the end of `New()`. No signature change — `New()` already takes a `*slog.Logger`.
- **Files modified:** `internal/daemon/daemon.go`
- **Commit:** 18bdd85d

**2. [Rule 3 - Test isolation] Nil-guard `d.obs` in `listenAdmin`**

- **Found during:** Task 3
- **Issue:** The existing `minimalDaemon` helper in `telemetry_test.go` does not populate `obs`, so Phase 10 tests for /healthz, /readyz, and pprof gating would nil-deref if `/metrics` registration dereferenced `d.obs` unconditionally.
- **Fix:** Guarded the `/metrics` registration with `if d.obs != nil && d.obs.Metrics() != nil`. Production path always hits both branches (daemon.New guarantees `obs` is set). Test path for the Phase 10 minimalDaemon tests skips metrics registration cleanly.
- **Files modified:** `internal/daemon/telemetry.go`
- **Commit:** 18bdd85d

**3. [Rule 1 - Test fixture] Primed all vectors in exposition smoke test**

- **Found during:** Task 3 (test failure)
- **Issue:** Initial `TestAdmin_MetricsEndpoint` primed only `ToolCalls` and `LSPoolWorkers`. Prometheus `Gather()` drops metric families with zero observations, so the exposition body was missing `serena_tool_duration_seconds`, `serena_lspool_evictions_total`, `serena_lspool_circuit_state`, and `serena_lspool_restarts_total`.
- **Fix:** Primed every vector via the frozen lspool sink methods plus direct `.Observe` / `.Inc` on ToolDuration / ToolCalls.
- **Files modified:** `internal/daemon/telemetry_metrics_test.go`
- **Commit:** 18bdd85d (fix folded into the task commit)

### Scope-deferred

- Existing `borrow/aider/tests/fixtures/languages/go/test.go` fails `go vet` with an unused-import warning. Out of scope for plan 11-01 (pre-existing, unrelated to observability). Not logged to `deferred-items.md` because it pre-dates phase 11 by multiple milestones.

## Authentication Gates

None — no secrets, no external services involved.

## Verification

- `go vet ./internal/obs/... ./internal/daemon/...` — clean
- `go test ./internal/obs/... ./internal/daemon/...` — green
- Task 1 acceptance grep checks — all pass, including the 4 frozen lspool signature matches:
  - `func (m *Metrics) LSPoolWorkersSet(language string, delta float64)` (1)
  - `func (m *Metrics) LSPoolEviction(language, reason string)` (1)
  - `func (m *Metrics) LSPoolCircuitStateSet(language string, state float64)` (1)
  - `func (m *Metrics) LSPoolRestart(language string)` (1)
- Task 2 acceptance: `TestMetricsLabelsAllowlist` green, negative drift test confirms the lint catches `user_id`.
- Task 3 acceptance: `/metrics` returns 200 with all 6 `serena_*` families + `go_goroutines` + `process_resident_memory_bytes`; `/healthz` and `/readyz` still work.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | d43d0480 | feat(11-01): add obs.Metrics with pre-registered prometheus vectors |
| 2 | 3d076596 | test(11-01): enforce bounded label allowlist via CI lint (METRIC-05) |
| 3 | 18bdd85d | feat(11-01): mount /metrics on admin listener (METRIC-01) |

## Self-Check: PASSED

All claimed files exist on disk:
- `internal/obs/metrics.go` FOUND
- `internal/obs/metrics_test.go` FOUND
- `internal/obs/metrics_labels_test.go` FOUND
- `internal/daemon/telemetry_metrics_test.go` FOUND

All claimed commits exist in git log:
- `d43d0480` FOUND
- `3d076596` FOUND
- `18bdd85d` FOUND
