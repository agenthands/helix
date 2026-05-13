---
phase: 68-precise-filefactdiff-populator
plan: 03
subsystem: observability
tags: [metrics, prometheus, closed-enum, cardinality]
requires: []
provides:
  - "obs.Metrics.LiveFileFactDiff *prometheus.CounterVec{tier,repo}"
  - "obs.Metrics.LiveFileFactDiffSynRsn *prometheus.CounterVec{reason}"
  - "obs.Metrics.LiveFileFactDiffInc(tier, repo string)"
  - "obs.Metrics.LiveFileFactDiffSyntheticReasonInc(reason string)"
  - "Metric helix_live_filefactdiff_total registered"
  - "Metric helix_live_filefactdiff_synthetic_reason_total registered"
affects: [DIFF-04 metric surface, Tier-3 fall-through reason breakdown]
tech-stack:
  added: []
  patterns:
    - "closed-enum drop-on-unknown helper (SemanticLiveUpdatesInc analog)"
    - "metrics_labels_test.go carve-out + prime-vector pattern (Pitfall #3 echo)"
key-files:
  created: []
  modified:
    - internal/obs/metrics.go
    - internal/obs/metrics_test.go
    - internal/obs/metrics_labels_test.go
decisions:
  - "Field name LiveFileFactDiffSynRsn (matches plan-frontmatter truth) — abbreviated to avoid hitting linters complaining about excessive Go identifier length elsewhere in the package; Help text and metric name spell out 'synthetic_reason' in full."
  - "Carve out new label names (tier, repo, reason) in metrics_labels_test.go rather than adding them to AllowedLabels, mirroring the Phase 60 D-07 'kind' carve-out precedent."
  - "Added local countSamples / sampleValue test helpers (Gather-based, dependency-free) rather than pulling in testutil — keeps the test file's existing dependency surface intact."
metrics:
  duration: ~10m
  completed: 2026-05-13
  tasks: 2
  files: 3
---

# Phase 68 Plan 03: LiveFileFactDiff Metrics Surface Summary

Registered the Phase 68 outcome metric `helix_live_filefactdiff_total{tier,repo}` (D-07) and the Tier-3 reason metric `helix_live_filefactdiff_synthetic_reason_total{reason}` (D-08) on `*obs.Metrics`, with closed-enum drop-on-unknown helpers and full label-allowlist + drop-on-unknown unit coverage. Plan 68-04 can now emit to these counters directly.

## Shipped Surface

### Struct fields (internal/obs/metrics.go)

```go
// Phase 68 D-07
LiveFileFactDiff *prometheus.CounterVec // {tier, repo}

// Phase 68 D-08
LiveFileFactDiffSynRsn *prometheus.CounterVec // {reason}
```

### Prometheus registrations (in `NewMetrics` construction block, after `SemanticLiveUpdates`)

| Vec field                  | Metric name                                       | Labels       | Help                                                                                  |
| -------------------------- | ------------------------------------------------- | ------------ | ------------------------------------------------------------------------------------- |
| `LiveFileFactDiff`         | `helix_live_filefactdiff_total`                   | `tier, repo` | Live FileFactDiff populator outcomes by tier (full/added-only/synthetic) and repo.    |
| `LiveFileFactDiffSynRsn`   | `helix_live_filefactdiff_synthetic_reason_total`  | `reason`     | Tier-3 synthetic-marker reason breakdown.                                             |

Both are appended to the `reg.MustRegister(...)` block immediately after `m.SemanticLiveUpdates`.

### Helper methods

```go
func (m *Metrics) LiveFileFactDiffInc(tier, repo string)
//   tier ∈ {"full","added-only","synthetic"} — unknown drops emission.
//   Nil-receiver / nil-vec safe.

func (m *Metrics) LiveFileFactDiffSyntheticReasonInc(reason string)
//   reason ∈ {"cold_start","extract_failed","extract_unsupported"} — unknown drops emission.
//   Nil-receiver / nil-vec safe.
```

### Label allowlist carve-outs (internal/obs/metrics_labels_test.go)

```go
"helix_live_filefactdiff_total":                  {"tier": true, "repo": true},
"helix_live_filefactdiff_synthetic_reason_total": {"reason": true},
```

Both vectors are primed in `TestMetricsLabelsAllowlist` so the lint actually scans their labels (Pitfall #3 — empty families are dropped by `Gather()`):

```go
m.LiveFileFactDiffInc("full", "repo-a")
m.LiveFileFactDiffSyntheticReasonInc("cold_start")
```

## Tests (all passing under `-race`)

| Test                                                          | Asserts                                                              |
| ------------------------------------------------------------- | -------------------------------------------------------------------- |
| `TestLiveFileFactDiffInc_KnownTiers`                          | full / added-only / synthetic each produce sample value == 1.        |
| `TestLiveFileFactDiffInc_UnknownTierDropped`                  | tier="nonsense" produces zero samples.                               |
| `TestLiveFileFactDiffInc_NilSafe`                             | `(*Metrics)(nil).LiveFileFactDiffInc(...)` does not panic.           |
| `TestLiveFileFactDiffSyntheticReasonInc_KnownReasons`         | cold_start / extract_failed / extract_unsupported each value == 1.   |
| `TestLiveFileFactDiffSyntheticReasonInc_UnknownReasonDropped` | reason="bogus" produces zero samples.                                |
| `TestLiveFileFactDiffSyntheticReasonInc_NilSafe`              | nil receiver does not panic.                                         |
| `TestMetricsLabelsAllowlist` (existing)                       | Both new families pass the allowlist+carve-out lint after priming.   |

Verification commands:

- `go build ./internal/obs/...` → OK
- `go vet ./internal/obs/...` → OK
- `go test -race -run 'TestLiveFileFactDiff' ./internal/obs/... -count=1` → OK
- `go test -race ./internal/obs/... -count=1` → OK (no regressions in the surrounding suite)

## Commits

| Task | Subject                                                            | Hash       |
| ---- | ------------------------------------------------------------------ | ---------- |
| 1    | `feat(68-03): add LiveFileFactDiff + synthetic-reason metrics`     | `cd5c62b4` |
| 2    | `test(68-03): cover drop-on-unknown for LiveFileFactDiff helpers`  | `9fff91a7` |

## Deviations from Plan

**[Rule 2 - Missing critical functionality] Added label-allowlist carve-outs + vector priming.**

- **Found during:** Task 1 (while wiring registrations).
- **Issue:** The plan's `<action>` steps A–D did not mention `metrics_labels_test.go`. Without a carve-out for the new label names `tier` / `repo` / `reason`, the existing `TestMetricsLabelsAllowlist` would fail at runtime as soon as either helper is emitted (Pitfall #3 from Phase 53 D-13 — but only after the vectors are primed).
- **Fix:** Added two carve-out entries and primed both new vectors in `TestMetricsLabelsAllowlist`, mirroring the Phase 60 `kind` carve-out and the Phase 63 IN-04 `reason` priming precedent.
- **Files modified:** internal/obs/metrics_labels_test.go.
- **Commit:** cd5c62b4 (folded into Task 1 so the package + label-lint always compile together).

**[Rule 3 - Blocking issue] Skipped `testutil.ToFloat64` reuse.**

- **Found during:** Task 2 (when planning the test helper).
- **Issue:** Existing tests in `internal/obs/metrics_test.go` use `m.registry.Gather()` + manual `GetMetric()` traversal (see `TestSemanticLiveUpdatesInc_*`), not `testutil.ToFloat64`. The plan suggested `testutil.ToFloat64 or similar`.
- **Fix:** Added local `countSamples` and `sampleValue` helpers matching the existing Gather-based pattern. No new imports.
- **Files modified:** internal/obs/metrics_test.go.

## Threat Flags

No new security surface introduced. T-68-09 (DoS via unbounded labels) is the only register entry and is mitigated as planned: closed-enum switch with default `return` in both helpers + drop-on-unknown unit tests covering both label dimensions. T-68-10 (`repo` info disclosure) accepted as planned — `repo` is a workspace identifier already exposed on other metrics, not a secret.

## Self-Check: PASSED

- internal/obs/metrics.go — FOUND
- internal/obs/metrics_test.go — FOUND
- internal/obs/metrics_labels_test.go — FOUND
- commit cd5c62b4 — FOUND
- commit 9fff91a7 — FOUND
- `go build ./internal/obs/...` — OK
- `go test -race -run 'TestLiveFileFactDiff' ./internal/obs/... -count=1` — OK
- `go test -race ./internal/obs/... -count=1` — OK (no regressions)
