---
phase: 53-obs-metrics-gaps
plan: 01
subsystem: observability
tags: [observability, prometheus, metrics, helix, foundation]
requires: []
provides:
  - 5 new helix_* Prometheus vector fields on *obs.Metrics
  - 5 new closed-enum helper methods (LSPoolLookup, RepoMapLookup, RepoMapExtractObserve, SessionLifecycleInc, EditOutcomeInc)
  - extended carveOuts map with 5 new closed-enum carve-outs
  - 5 cardinality bound tests pinning per-family ceilings
affects:
  - downstream waves 2 + 3: pool.go, cache.go, edit handlers, forwarder, http session middleware all emit through these helpers
tech-stack:
  added: []
  patterns:
    - drop-unknown closed-enum discipline at emission (mirrors RenameStrategyInc Phase 47 D-07)
    - cardinality bound test per family (label-combo count, not scraped-line count)
    - per-vector primed Inc/Observe in lint test (Pitfall #3 silent-bypass guard)
key-files:
  created: []
  modified:
    - internal/obs/metrics.go (+129 lines)
    - internal/obs/metrics_test.go (+31 lines)
    - internal/obs/metrics_labels_test.go (+163 lines)
decisions:
  - "Strategy enum is 4 values {exact, whitespace_normalized, indentation_flexible, none} (D-11 AMENDED 2026-04-30 + Q-4)"
  - "Cardinality bound for helix_edit_outcome_total is 168 (7×6×4), down from 210 in original D-11"
  - "Latent gap fix: helix_rename_strategy_total added to TestMetrics_RegisteredFamilies want[]"
  - "Tasks 1.1 + 1.2 committed as separate atomic commits per executor protocol; intermediate RED state documented in commit message rather than combined-commit shortcut"
metrics:
  duration: "~12 minutes"
  completed: "2026-04-30"
  tasks: 3
  files_modified: 3
  lines_added: 323
---

# Phase 53 Plan 01: Obs Metrics Foundation Summary

**One-liner:** Landed 5 new helix_* Prometheus vectors, 5 closed-enum helper methods, and 5 cardinality bound tests in `internal/obs/metrics.go` so all Wave 2/3 downstream emission sites compile and emit without circular dependencies.

## What Landed

### Vectors (5)

Added to the `*Metrics` struct, registered in `newMetrics()`, exposed on the owned registry:

| Family | Labels | Type | Buckets |
|---|---|---|---|
| `helix_lspool_lookups_total` | `{language, result}` | CounterVec | — |
| `helix_repomap_lookups_total` | `{language, result}` | CounterVec | — |
| `helix_repomap_extract_duration_seconds` | `{language, extractor}` | HistogramVec | `{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5}` (D-06 verbatim) |
| `helix_session_lifecycle_total` | `{phase, transport}` | CounterVec | — |
| `helix_edit_outcome_total` | `{tool_name, outcome, strategy}` | CounterVec | — |

### Helper methods (5)

Each follows the canonical drop-unknown pattern from `RenameStrategyInc` (Phase 47 D-07): closed-enum guard at the top, early `return` on unknown values, `WithLabelValues(...).Inc()` / `.Observe()` last:

- `(m *Metrics) LSPoolLookup(language, result string)` — drops result ∉ {hit, miss}
- `(m *Metrics) RepoMapLookup(language, result string)` — drops result ∉ {hit, miss}
- `(m *Metrics) RepoMapExtractObserve(language, extractor string, seconds float64)` — drops extractor ∉ {treesitter, lsp, fallback}
- `(m *Metrics) SessionLifecycleInc(phase, transport string)` — drops phase ∉ {started, ended, error} OR transport ∉ {stdio, http}
- `(m *Metrics) EditOutcomeInc(toolName, outcome, strategy string)` — drops outcome ∉ {success, no_match, ambiguous_match, validation_failed, ls_error, internal} OR strategy ∉ {exact, whitespace_normalized, indentation_flexible, none}

### Lint extensions (`metrics_labels_test.go`)

- `carveOuts` map gains 5 new entries (one per new family) covering `result`, `extractor`, `phase`, `transport`, `strategy` as closed-enum carve-outs.
- `TestMetricsLabelsAllowlist` primes every new vector with a sample `Inc()` / `Observe()` so Gather() actually scans their labels (Pitfall #3 silent-bypass guard).
- The negative drift test `TestMetricsLabelsAllowlist_catchesDrift` is untouched and still asserts rejection of an unknown label name on a temporary registry.

### Cardinality bound tests (5)

Added `TestMetrics_CardinalityBounds_*` tests with a shared `gatherFamily(...)` helper:

| Test | Family | Bound | Rationale |
|---|---|---|---|
| `_LSPoolLookups` | helix_lspool_lookups_total | ≤ 2 × 52 | 2 result values × N_languages |
| `_RepoMapLookups` | helix_repomap_lookups_total | ≤ 2 × 52 | 2 result values × N_languages |
| `_RepoMapExtract` | helix_repomap_extract_duration_seconds | ≤ 3 × 52 | 3 extractors × N_languages (label-combo count, NOT scraped-line count — see RESEARCH Pitfall #1 + C-4) |
| `_SessionLifecycle` | helix_session_lifecycle_total | ≤ 6 | 3 phases × 2 transports |
| `_EditOutcome` | helix_edit_outcome_total | ≤ 168 | 7 tools × 6 outcomes × 4 strategies (per AMENDED D-11 + Q-4: ellipsis dropped, failed maps to outcome=no_match,strategy=none) |

### Registered-families test (`metrics_test.go`)

- `TestMetrics_RegisteredFamilies` `want[]` extended with the 5 new families AND the latent-gap entry `helix_rename_strategy_total` (Phase 47 D-07 was missing from the original want[]).
- Companion priming added inside the test so the 6 newly-listed families appear in `Gather()` output.

### Noop coverage (`metrics_test.go`)

- `TestMetrics_NoopProviderReturnsUsableSink` exercises every new helper (12 calls covering hit/miss × all extractors × all phases × all transports × representative outcome+strategy tuples) plus the previously-uncovered `RenameStrategyInc`. All callable on `obs.Noop(...).Metrics()` without panic.

## Strategy enum decision (recorded for Wave 2 / plan 53-04)

The `strategy` label on `helix_edit_outcome_total` is a closed enum of **4 values**:

```
{exact, whitespace_normalized, indentation_flexible, none}
```

This is the AMENDED D-11 contract (2026-04-30 reconciliation). The original D-11 listed 5 values including `ellipsis`, but the codebase (`internal/fuzzy/types.go`) does not declare a `fuzzy.Strategy` value for it. Per Q-4 resolution, `fuzzy.StrategyFailed` paths map to `outcome=no_match, strategy=none` at the call sites — `failed` is NOT emitted as a strategy label value. The cardinality bound for `helix_edit_outcome_total` is therefore 7 × 6 × 4 = **168** (down from 210 in the pre-amendment text). Wave 2 plan 53-04 must implement this 4-value enum at every edit-tool emission site and reject `"failed"` and `"ellipsis"` at the helper layer (already done in `EditOutcomeInc`'s drop-unknown switch).

## Latent fix

`helix_rename_strategy_total` (introduced in Phase 47 D-07) was missing from `TestMetrics_RegisteredFamilies.want[]`. The test silently passed because Gather() omits empty families and the test only checks for required entries. PATTERNS.md line 173 surfaced the gap; this plan fixes it by:

1. Priming `m.RenameStrategy.WithLabelValues("lsp-native").Inc()` inside the test.
2. Adding `"helix_rename_strategy_total"` to `want[]`.

A future drift would now be caught.

## Files modified (line counts)

| File | Lines added |
|---|---|
| `internal/obs/metrics.go` | +129 |
| `internal/obs/metrics_test.go` | +31 |
| `internal/obs/metrics_labels_test.go` | +163 |
| **Total** | **+323** |

No files deleted. No files renamed.

## Commits

| Task | Type | Hash | Message |
|---|---|---|---|
| 1.1 | test | `1415f604` | add 5 cardinality bound tests for new metric families (RED state expected, documented) |
| 1.2 | feat | `a287a8f0` | add 5 metric vectors + 5 closed-enum helper methods (flips Task 1.1 to GREEN) |
| 1.3 | test | `7984b2c4` | extend lint carve-outs, registered-families list, noop coverage |

## Verification

```text
$ go vet ./internal/obs/...
ok

$ go test ./internal/obs/...
ok      github.com/agenthands/helix/internal/obs    10.640s

$ go test ./internal/obs/... -run 'TestMetrics_CardinalityBounds' -v
=== RUN   TestMetrics_CardinalityBounds_LSPoolLookups
--- PASS: TestMetrics_CardinalityBounds_LSPoolLookups
=== RUN   TestMetrics_CardinalityBounds_RepoMapLookups
--- PASS: TestMetrics_CardinalityBounds_RepoMapLookups
=== RUN   TestMetrics_CardinalityBounds_RepoMapExtract
--- PASS: TestMetrics_CardinalityBounds_RepoMapExtract
=== RUN   TestMetrics_CardinalityBounds_SessionLifecycle
--- PASS: TestMetrics_CardinalityBounds_SessionLifecycle
=== RUN   TestMetrics_CardinalityBounds_EditOutcome
--- PASS: TestMetrics_CardinalityBounds_EditOutcome
PASS
```

All success criteria met:

- [x] 5 new helix_* metric families register on the obs.Metrics owned registry
- [x] Each helper drops unknown enum values (closed-enum at emission)
- [x] Cardinality bound tests pass: 2*N, 2*N, 3*N, 6, 168
- [x] `go test ./internal/obs/...` green

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written, with one consciously-tracked sequencing choice:

**[Process note] Task 1.1 + 1.2 commit sequencing.** The plan included an advisory recommendation to "combine Task 1.1 stubs + Task 1.2 vector construction in the SAME commit so the build never breaks mid-wave." Per executor protocol's stricter rule that "each task is committed individually," I committed Task 1.1 in a RED state (5 tests referencing `m.LSPoolLookups` etc. which were undefined), then immediately committed Task 1.2 to flip the package to GREEN. The intermediate RED commit is documented in its own commit message ("Compilation is RED at this commit ...; Task 1.2 follows immediately and flips it GREEN"). Net effect on the merged feature branch: identical to the combined-commit recommendation, but with cleaner per-task atomic commits for `git bisect`.

## Threat surface scan

No new threat surface introduced beyond what plan 53-01's `<threat_model>` already covers. T-53-01 (cardinality DoS) is mitigated by:

- Closed-enum drop-unknown helpers (5 of 5 new helpers reject unknown values silently).
- 5 cardinality bound tests pin per-family ceilings (build fails if any vector exceeds the documented bound).
- `TestMetricsLabelsAllowlist` lint fails the build if any new label name appears outside `AllowedLabels` ∪ `carveOuts`.

T-53-02 and T-53-03 dispositions are `accept` and remain unchanged.

## Self-Check: PASSED

- internal/obs/metrics.go exists at 305 lines
- internal/obs/metrics_test.go exists at 144 lines
- internal/obs/metrics_labels_test.go exists at 308 lines
- Commit 1415f604 exists in git log
- Commit a287a8f0 exists in git log
- Commit 7984b2c4 exists in git log
- 5 cardinality bound tests pass in `go test ./internal/obs/...`
- 0 known stubs in modified files
