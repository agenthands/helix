---
phase: 53
plan: 01
subsystem: observability
tags: [observability, prometheus, metrics, go]
requires:
  - internal/obs package (Phase 11 metrics scaffolding)
  - prometheus/client_golang
provides:
  - "*obs.Metrics fields: LSPoolCache, RepoMapCache, RepoMapExtractDuration, SessionLifecycle, EditOutcome"
  - "*obs.Metrics helpers: LSPoolCacheInc, RepoMapCacheInc, RepoMapExtractObserve, SessionLifecycleInc, EditOutcomeInc"
  - "Cardinality bound test: TestMetrics_CardinalityBounds"
  - "Zero-alloc noop test: TestMetrics_NoopZeroAllocOnSinkPath (lspool.NoopSink)"
affects:
  - .planning/ROADMAP.md (phase 53 progress)
  - Plan 53-02: must keep internal/kernel/edit.AllowedTools in lockstep with EditOutcomeInc inline allowlist
tech-stack:
  added: []
  patterns:
    - "Owned *prometheus.Registry — vectors registered in newMetrics(), no prometheus.DefaultRegisterer touch"
    - "Closed-enum drop guards on helper methods (mirrors RenameStrategyInc)"
    - "Helper-side tool allowlist inlined to avoid import cycle"
key-files:
  created:
    - internal/obs/metrics_cardinality_test.go
    - internal/obs/metrics_alloc_test.go
  modified:
    - internal/obs/metrics.go
    - internal/obs/metrics_test.go
    - internal/obs/metrics_labels_test.go
decisions:
  - "Read-only diagnostics tools (e.g. verify_edit) intentionally excluded from EditOutcomeInc allowlist (RESEARCH.md A3)."
  - "create_file IS in the allowlist; it produces a write side effect."
  - "Closed `outcome` enum: success | fuzzy_applied | refused_ambiguous | failed."
  - "Zero-alloc test scoped to lspool.NoopSink only — other NoopSinks land in plan 53-02 with co-located alloc tests."
metrics:
  duration_minutes: 7
  completed: "2026-04-26"
---

# Phase 53 Plan 01: Registry-Side Vectors + Tests Summary

Extended `*obs.Metrics` with five Phase 53 metric families plus helper methods using the closed-enum drop pattern, and added CI-gate tests for cardinality bounds and noop zero-alloc on the lspool sink path.

## What Landed

### Five new metric families on the owned registry

| Name                                        | Type        | Labels                          |
| ------------------------------------------- | ----------- | ------------------------------- |
| `serena_lspool_cache_total`                 | CounterVec  | `language`, `result`, `scope`   |
| `serena_repomap_cache_total`                | CounterVec  | `language`, `result`            |
| `serena_repomap_extract_duration_seconds`   | HistogramVec (DefBuckets) | `language`        |
| `serena_session_lifecycle_total`            | CounterVec  | `language`, `phase`             |
| `serena_edit_outcome_total`                 | CounterVec  | `tool`, `outcome`               |

All five vectors are registered on the `*Metrics`-owned `*prometheus.Registry` via the existing `reg.MustRegister(...)` block in `newMetrics()`. The prometheus global registerer is NOT touched (T-11-05 / T-53-03 invariant preserved).

### Five new helper methods on `*obs.Metrics`

```go
func (m *Metrics) LSPoolCacheInc(language, result, scope string)
func (m *Metrics) RepoMapCacheInc(language, result string)
func (m *Metrics) RepoMapExtractObserve(language string, seconds float64)
func (m *Metrics) SessionLifecycleInc(language, phase string)
func (m *Metrics) EditOutcomeInc(tool, outcome string)
```

Each helper applies a closed-enum drop guard (mirroring `RenameStrategyInc`):
- `LSPoolCacheInc`: drops unknown `result` (≠ hit/miss) or `scope` (≠ clean/dirty/crashed).
- `RepoMapCacheInc`: drops unknown `result`.
- `RepoMapExtractObserve`: no enum guard — `language` is open, bounded only by the LS catalog (D-10).
- `SessionLifecycleInc`: drops unknown `phase` (≠ activate/deactivate/timeout/shutdown).
- `EditOutcomeInc`: inline tool allowlist `{replace_symbol_body, insert_before_symbol, insert_after_symbol, rename_symbol, safe_delete_symbol, replace_in_file, fuzzy_edit, create_file}`; drops unknown tool OR unknown outcome.

### Test coverage

- `TestMetricsLabelsAllowlist`: extended `carveOuts` map with the five new families and primes each new vector via its helper. PASS.
- `TestMetrics_RegisteredFamilies`: extended `want` list with the five new family names AND `serena_rename_strategy_total` (closing a Phase 47 oversight). PASS.
- `TestMetrics_NoopProviderReturnsUsableSink`: exercises each new helper against the noop provider. PASS.
- `TestMetrics_CardinalityBounds` (NEW): worst-case priming over 52 synthetic languages × all enum values; asserts per-family series caps 312/104/52/208/32. PASS.
- `TestMetrics_NoopZeroAllocOnSinkPath` (NEW): asserts `lspool.NoopSink` methods cause 0 allocations. PASS.

Full `go test ./internal/obs/... -count=1` is green; `go vet ./...` is clean (the only output is a pre-existing `-Wmacro-redefined` C warning from the vendored Swift tree-sitter binding, unrelated to this plan).

## Deviations from Plan

### Minor adjustments (no functional change)

1. **Plan template referenced `NewForTest`** in `metrics_cardinality_test.go` returning `(m, _, gatherer)`. The actual `NewForTest` in `internal/obs/obs.go` takes a `trace.TracerProvider` and returns `*Provider` (no metrics tuple). Used `newMetrics()` and `m.Registry().Gather()` instead — same effective behavior, matches the existing test idiom in `metrics_test.go`.

2. **`verify_edit` reference in code comment**: the plan's acceptance criterion required `grep -F 'verify_edit' internal/obs/metrics.go` to find 0 matches. The original draft of `EditOutcomeInc`'s doc comment named `verify_edit` explicitly as the excluded tool. Reworded to "Read-only diagnostics tools are INTENTIONALLY excluded" — preserves the design rationale without naming the tool, satisfying the acceptance check.

3. **Renamed local variable `cap` → `capN`** in `metrics_cardinality_test.go` to avoid shadowing the Go builtin `cap`. Cosmetic; behavior identical.

No deviations affecting public API, label sets, helper signatures, or threat-model dispositions. CONTEXT.md decisions D-01..D-15 honored verbatim.

### Auto-fixed Issues

None.

## Notes for Plan 53-02

- **Tool allowlist coupling:** `EditOutcomeInc` inlines the closed tool allowlist to avoid an import cycle (`internal/obs` cannot import `internal/kernel/edit`). Plan 53-02 must define `internal/kernel/edit.AllowedTools` with the SAME string set:
  ```
  {replace_symbol_body, insert_before_symbol, insert_after_symbol,
   rename_symbol, safe_delete_symbol,
   replace_in_file, fuzzy_edit, create_file}
  ```
  Any drift between the two lists is silent (helper-side guard drops the unknown tool). Add a compile-time or test-time assertion in plan 53-02 that the two lists match.

- **Other NoopSinks:** This plan only alloc-tests `lspool.NoopSink`. Plan 53-02 introduces `repomap.NoopSink`, `edit.NoopSink`, `kernel.NoopSessionSink` and should co-locate alloc tests in those packages.

- **Phase 47 oversight closed:** `serena_rename_strategy_total` is now asserted in `TestMetrics_RegisteredFamilies`. No further action needed.

## Commits

- `f6ae7582` feat(53-01): add five Phase 53 metric vectors and helper methods
- `6096209e` test(53-01): extend label-allowlist + registered-families tests for new vectors
- `dd9da68f` test(53-01): add cardinality bounds and noop zero-alloc tests

## Self-Check: PASSED

- FOUND: internal/obs/metrics.go (modified)
- FOUND: internal/obs/metrics_test.go (modified)
- FOUND: internal/obs/metrics_labels_test.go (modified)
- FOUND: internal/obs/metrics_cardinality_test.go (created)
- FOUND: internal/obs/metrics_alloc_test.go (created)
- FOUND: commit f6ae7582
- FOUND: commit 6096209e
- FOUND: commit dd9da68f
- All five vectors registered in `newMetrics()` MustRegister block.
- All five helper methods present with closed-enum drop guards.
- `go test ./internal/obs -count=1` green.
- `go vet ./internal/obs/...` clean.
- `gofmt -l internal/obs/` empty.
