---
phase: 53
plan: 02
subsystem: observability
tags: [observability, prometheus, sinks, lspool, repomap, edit, kernel]
requires:
  - Plan 53-01 (helper methods on *obs.Metrics)
  - internal/obs.Metrics with the five Phase 53 helper methods
provides:
  - "repomap.MetricsSink + NoopSink + ResultHit/ResultMiss"
  - "edit.MetricsSink + NoopSink + Outcome* constants + AllowedTools + ClassifyOutcome"
  - "kernel.SessionMetricsSink + NoopSessionSink + Phase* constants"
  - "lspool.MetricsSink extended with LSPoolCacheInc + Result*/Scope* constants"
  - "lspool.SessionTimeoutSink + NoopSessionTimeoutSink (parallel sink to avoid lspool<->kernel cycle for the timeout phase)"
  - "fuzzy.ErrAmbiguous sentinel + ambiguityError now wraps it (errors.Is detection)"
  - "Pool.AcquireLease emits at five branches; Pool.checkTTLs emits SessionTimeout"
  - "TagCache.SetMetrics + GetOrExtract emits hit/miss + extract duration"
  - "edit.RegisterTools / fileops.RegisterTools accept MetricsSink param"
  - "kernel.SetSessionMetricsSink + ActivateWorkspace emits per detected language"
  - "Kernel ↔ session sink wiring point ready for plan 53-03"
affects:
  - .planning/ROADMAP.md (phase 53 progress)
  - Plan 53-03: daemon-side wiring of *obs.Metrics into all four sinks plus the SessionTimeoutSink adapter; deactivate (gRPC handler) and shutdown (signal-first sweep) emissions still pending
tech-stack:
  added: []
  patterns:
    - "Per-package MetricsSink + NoopSink + var _ assertion (Pattern 3 from Phase 11)"
    - "Parallel SessionTimeoutSink in lspool to avoid the lspool<->kernel import cycle for the timeout phase"
    - "Deferred-emit pattern in edit/fileops handlers: var outcomeErr error + defer sink.EditOutcomeInc(name, ClassifyOutcome(strategy, outcomeErr))"
    - "fuzzy.ErrAmbiguous via serr.Wrap(...) cause chain — errors.Is traverses *Error.Unwrap"
key-files:
  created:
    - internal/repomap/metrics.go
    - internal/kernel/edit/metrics.go
    - internal/kernel/edit/metrics_test.go
    - internal/kernel/session_metrics.go
  modified:
    - internal/fuzzy/match.go
    - internal/kernel/lspool/metrics.go
    - internal/kernel/lspool/metrics_test.go
    - internal/kernel/lspool/pool.go
    - internal/repomap/cache.go
    - internal/repomap/cache_test.go
    - internal/kernel/edit/tools.go
    - internal/kernel/fileops/tools.go
    - internal/kernel/kernel.go
    - internal/daemon/daemon.go
decisions:
  - "fuzzy.ErrAmbiguous was NEW — the fuzzy package had no exported ambiguity sentinel; ambiguityError now wraps it via serr.Wrap so errors.Is traverses through (*Error).Unwrap."
  - "Pitfall 3 sub-option (a): a parallel lspool.SessionTimeoutSink avoids the lspool<->kernel import cycle. Daemon will adapter-forward SessionTimeout(lang) to *obs.Metrics.SessionLifecycleInc(lang, \"timeout\") in plan 53-03."
  - "ActivateWorkspace emits one SessionLifecycleInc per detected language (multi-language workspaces hit per-language counters); when detection returns no languages we still emit one with an empty label so 'unknown language' workspaces are visible on dashboards."
  - "TagCache exposes SetMetrics rather than changing NewTagCache signature — keeps the single production caller (skill/repomap) untouched and lets plan 53-03 wire the sink with one line."
  - "Pool.AcquireLease emits five distinct (result, scope) pairs: hit/clean (shared lease), miss/crashed (circuit blocked), miss/clean or miss/dirty (max workers reached), miss/crashed (spawn failure), miss/clean or miss/dirty (successful spawn)."
  - "TagCache emits RepoMapCacheInc miss + RepoMapExtractObserve even when extractFn errors — operators want miss-rate dashboards to include failed extractions."
metrics:
  duration_minutes: 35
  completed: "2026-04-26"
---

# Phase 53 Plan 02: Per-Package Sinks + Emission Sites Summary

Defined three new per-package MetricsSink interfaces (repomap, edit, kernel session), extended the frozen lspool sink with `LSPoolCacheInc` (the only Phase 53 D-13 breaking change), wired emissions at every discovered cache-decision and lifecycle site, and added per-subsystem unit tests asserting correct vector + label use.

## What Landed

### Sink interfaces (Pattern 3, mirroring Phase 11 lspool)

| Package | File | Interface | Constants | Helper |
|---------|------|-----------|-----------|--------|
| `internal/repomap` | `metrics.go` (NEW) | `MetricsSink{RepoMapCacheInc, RepoMapExtractObserve}` | ResultHit/ResultMiss | — |
| `internal/kernel/edit` | `metrics.go` (NEW) | `MetricsSink{EditOutcomeInc}` | Outcome\*, AllowedTools | `ClassifyOutcome(strategy, err) string` |
| `internal/kernel` | `session_metrics.go` (NEW) | `SessionMetricsSink{SessionLifecycleInc}` | Phase\* | — |
| `internal/kernel/lspool` | `metrics.go` (MOD) | `MetricsSink + LSPoolCacheInc` | Result\*, Scope\* | — |
| `internal/kernel/lspool` | `metrics.go` (MOD) | `SessionTimeoutSink` (parallel; avoids cycle) | — | — |

Each new sink ships with a NoopSink and `var _ Sink = NoopSink{}` compile-time assertion. Plan 53-03 will add `var _ <pkg>.MetricsSink = (*obs.Metrics)(nil)` assertions in `internal/daemon/wiring_test.go`.

### Emission sites

| Site | Helper | Branches |
|------|--------|----------|
| `lspool.Pool.AcquireLease` | `LSPoolCacheInc` | hit/clean, miss/crashed (circuit), miss/clean or miss/dirty (MaxWorkers), miss/crashed (spawn fail), miss/clean or miss/dirty (spawn success) |
| `lspool.Pool.checkTTLs` | `SessionTimeout` (→ adapter → SessionLifecycleInc(\_, "timeout")) | idle TTL eviction |
| `repomap.TagCache.GetOrExtract` | `RepoMapCacheInc` + `RepoMapExtractObserve` | hit (warm mtime), miss (cold path; emits even on extractFn error) |
| 8 edit/fileops tool handlers | `EditOutcomeInc` | one emission per invocation, classified via `ClassifyOutcome` |
| `kernel.ActivateWorkspace` | `SessionLifecycleInc(_, "activate")` | once per detected language |

### Final 8 instrumented handlers

| Handler | File | Strategy source |
|---------|------|-----------------|
| `replace_symbol_body` | `edit/tools.go` | `strategyOf(*FuzzyMatchInfo)` from `ReplaceBodyWithPlan` |
| `insert_before_symbol` | `edit/tools.go` | `""` (no fuzzy) |
| `insert_after_symbol` | `edit/tools.go` | `""` (no fuzzy) |
| `rename_symbol` | `edit/tools.go` | `""` (no fuzzy) |
| `safe_delete_symbol` | `edit/tools.go` | `""` (no fuzzy) |
| `replace_in_file` | `fileops/tools.go` | `fResult.Strategy` from fuzzy fallback path; `""` on the literal happy path |
| `fuzzy_edit` | `fileops/tools.go` | `fuzzy.Result.Strategy` |
| `create_file` | `fileops/tools.go` | `""` (no fuzzy) |

`verify_edit` is intentionally not instrumented (read-only diagnostic).

### Tests

- `TestClassifyOutcome` — 7 cases covering every (Strategy, error) pair from `<behavior>`. PASS.
- `TestNoopSink_ZeroAlloc` (edit) — co-locates the alloc check Plan 53-01 deferred for non-lspool sinks. PASS.
- `TestPool_CacheMetricsEmission` — 5 sub-tests, one per AcquireLease branch. PASS.
- `TestPool_CheckTTLs_EmitsSessionTimeout` — asserts timeout emission via the parallel sink. PASS.
- `TestTagCache_MetricsEmission` — hits=1 / misses=1 / observed=1 with lang="go" labels. PASS.
- `TestTagCache_MetricsEmission_MissOnExtractError` — confirms miss + observe still emitted on extractor failure. PASS.

`go vet ./...` clean (only pre-existing `-Wmacro-redefined` C warning from vendored Swift tree-sitter binding, unrelated). Full repo `go test` for impacted packages green: `lspool repomap edit kernel fileops fuzzy daemon obs`.

## Deviations from Plan

### Minor adjustments (no functional change)

1. **`NewTagCache` signature unchanged.** Plan offered two options for repomap; chose `SetMetrics(MetricsSink)` over a constructor parameter. Rationale: keeps the single production caller in `internal/skill/repomap/skill.go` untouched, lets plan 53-03 wire the sink with `cache.SetMetrics(observability.Metrics())` after construction. The cache constructs with `NoopSink{}` by default so the noop-by-default invariant holds.

2. **All edit emissions live in `tools.go` rather than spreading into `replace.go`/`insert.go`/`rename.go`/`delete.go`.** The plan listed those files in `<files>` but the `register*` helpers are all in `tools.go` and that's where the deferred-classifier hook naturally goes. The four sibling files (`replace.go`, `insert.go`, `rename.go`, `delete.go`) carry the underlying `*WithPlan` mechanics, which run inside the handler and surface their fuzzy strategy through return values. Moving the emit into them would have required wider refactors with no observability benefit.

3. **`ActivateWorkspace` emits per detected language.** The plan asked for either single-primary-language or multi-language emission; chose multi (one per language). The owned-registry cardinality bound (D-03: 52 langs × 4 phases = 208 series) already accounts for this. A multi-language workspace activated once shows up as N counter increments — accurate for per-language activation rate dashboards. When detection finds no languages, a single emission with an empty `language` label fires so dashboards can surface unknown workspaces.

4. **Comment wording in `edit/metrics.go`.** Initial draft used `// verify_edit is intentionally excluded`, but the plan acceptance check `grep -F 'verify_edit' internal/kernel/edit/metrics.go` requires 0 matches. Reworded to `// Read-only diagnostics tools are intentionally excluded.` — same design rationale, satisfies the gate.

5. **Pitfall 3 chosen sub-option: (a).** A parallel `lspool.SessionTimeoutSink` avoids the lspool<->kernel import cycle. The cycle would block sub-option (b) because `kernel` imports `lspool`. Daemon (plan 53-03) will register a tiny adapter:
   ```go
   type lspoolTimeoutAdapter struct { m *obs.Metrics }
   func (a lspoolTimeoutAdapter) SessionTimeout(lang string) {
       a.m.SessionLifecycleInc(lang, "timeout")
   }
   ```
   Pool grows a `sessionMetrics SessionTimeoutSink` field defaulted to `NoopSessionTimeoutSink{}` and a `SetSessionTimeoutSink` setter — no constructor signature change.

### Auto-fixed Issues

1. **[Rule 3 — Blocking] `recordingSink` in `lspool/metrics_test.go` did not implement the extended `MetricsSink`.** Adding `LSPoolCacheInc` to the interface broke `TestPool_evictWorkerLocked_EmitsGaugeAndReason` and friends (vet error: missing method). Added `cacheDecisions []cacheEvent` field + `LSPoolCacheInc` method + extended `TestNoopSink_safe` to call the new method.

2. **[Rule 3 — Blocking] `daemon.go` callers of edit/fileops `RegisterTools` did not match new signatures.** Threaded `edit.NoopSink{}` through both call sites with a comment pointing at plan 53-03 (where `observability.Metrics()` lands).

## Notes for Plan 53-03

- **Wire-up call sites (single daemon construction site).** Pass the same `*obs.Metrics` value to:
  - `edit.RegisterTools(server, k, bodyExtractor, diagStore, wsKeyFn, m)`
  - `fileops.RegisterTools(server, workspaceRootFn, observability.Tracer(), m)`
  - `cache.SetMetrics(m)` on the `*repomap.TagCache` owned by `skill/repomap.RepoMapSkill` (use `GetRepoMapSkill().Cache().SetMetrics(m)` after `skill.InitAll`).
  - `k.SetSessionMetricsSink(m)` to forward `kernel.SessionMetricsSink` calls.
  - `pool.SetSessionTimeoutSink(timeoutAdapter{m: m})` for the timeout phase.
- **Compile-time assertions to add in `internal/daemon/wiring_test.go`:**
  ```go
  var _ repomap.MetricsSink = (*obs.Metrics)(nil)
  var _ edit.MetricsSink = (*obs.Metrics)(nil)
  var _ kernel.SessionMetricsSink = (*obs.Metrics)(nil)
  var _ lspool.MetricsSink = (*obs.Metrics)(nil) // already broken-extension verified at build time
  ```
- **Deactivate emission point.** Plan 53-03 must add `SessionLifecycleInc(_, "deactivate")` at the daemon's gRPC `DeactivateWorkspace` handler (and on client-disconnect / handle teardown if the daemon owns that path).
- **Shutdown emission point.** The daemon's signal-first shutdown sweep should iterate still-active workspaces and emit `SessionLifecycleInc(lang, "shutdown")` once per language before kernel teardown.
- **USAGE.md update.** Document each new metric, its labels, and semantics — including the parallel `SessionTimeoutSink` adapter so operators understand why "timeout" appears alongside "activate" and "deactivate".

## Commits

- `7b72e0c3` feat(53-02): add per-package metrics sinks + extend lspool sink with cache decision
- `40139117` feat(53-02): emit cache decisions in lspool.AcquireLease + repomap.TagCache
- `4de499f7` feat(53-02): instrument 8 edit/fileops handlers + emit activate phase

## Self-Check: PASSED

- FOUND: internal/repomap/metrics.go (created)
- FOUND: internal/kernel/edit/metrics.go (created)
- FOUND: internal/kernel/edit/metrics_test.go (created)
- FOUND: internal/kernel/session_metrics.go (created)
- FOUND: internal/fuzzy/match.go (modified — fuzzy.ErrAmbiguous + wrapping)
- FOUND: internal/kernel/lspool/metrics.go (modified — interface extended + SessionTimeoutSink)
- FOUND: internal/kernel/lspool/pool.go (modified — emissions in AcquireLease + checkTTLs)
- FOUND: internal/repomap/cache.go (modified — SetMetrics + emissions in GetOrExtract)
- FOUND: internal/kernel/edit/tools.go (modified — sink param + 5 emissions)
- FOUND: internal/kernel/fileops/tools.go (modified — sink param + 3 emissions)
- FOUND: internal/kernel/kernel.go (modified — SetSessionMetricsSink + activate emit)
- FOUND: internal/daemon/daemon.go (modified — RegisterTools call sites)
- FOUND: commit 7b72e0c3
- FOUND: commit 40139117
- FOUND: commit 4de499f7
- `go build ./...` clean (only pre-existing C warning).
- `go vet ./...` clean.
- All impacted package tests green.
