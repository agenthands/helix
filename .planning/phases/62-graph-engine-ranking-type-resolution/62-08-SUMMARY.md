---
phase: 62-graph-engine-ranking-type-resolution
plan: 08
subsystem: rank-engine-observability
tags: [observability, stubs, metrics, phase-64-bridge, gap-closure]
gap_closure: true
requires:
  - 62-VERIFICATION.md (truth #21 / WR-05)
  - 62-CONTEXT.md (Phase 64 scope: typed effective-graph reads + MCP tool surface)
provides:
  - stub_no_data observability on rankStoreAdapter (metric + once-warn + TODO anchor)
affects:
  - internal/daemon/rank_wiring.go (stubObserveState harness; 4 stub methods now signal)
  - internal/daemon/daemon.go (newRankStoreAdapter constructor signature: +metrics, +logger)
  - internal/obs/metrics.go (closed-enum extends to stub_no_data)
  - internal/obs/metrics_labels_test.go (allowlist primes the new outcome value)
  - internal/daemon/rank_wiring_test.go (3 new tests; recording sink + slog handler)
tech_stack:
  added: []
  patterns:
    - sync.Once per (repoID, method) for once-gated logging
    - drop-on-unknown closed-enum discipline (consistent with rest of obs/metrics.go)
    - nil-safe observability harness (zero/nil receiver = silent)
key_files:
  created:
    - internal/daemon/rank_wiring_test.go
    - .planning/phases/62-graph-engine-ranking-type-resolution/62-08-SUMMARY.md
  modified:
    - internal/daemon/rank_wiring.go
    - internal/daemon/daemon.go
    - internal/obs/metrics.go
    - internal/obs/metrics_labels_test.go
decisions:
  - "Option C from gap brief (Both metric label + once-warn log) — cheap union maximises operator visibility without adding a new metric family"
  - "outcome='stub_no_data' extends the existing helix_semantic_graph_repair_total closed enum rather than creating a new metric (no new dashboard / alerting churn)"
  - "Once-gate keyed on (repoID, method) not just repoID, so each of the 4 distinct stubs surfaces on first call from a new workspace"
  - "Did NOT pre-implement Phase 64 queries — out of scope per CONTEXT.md; this plan only closes the observability gap"
metrics:
  duration_minutes: ~12
  tasks_completed: 2
  files_changed: 5
  files_created: 2
completed_date: 2026-05-07
---

# Phase 62 Plan 08: rankStoreAdapter stub-observability harness (WR-05) Summary

Closed verification gap truth #21 by routing the four Phase 64-deferred stubs on `rankStoreAdapter` through an observability harness so operators see "no data yet" instead of clean `outcome="applied"` repair counts.

## What Changed

**Operator-visible signal added without a new metric family.** The four stubbed read methods on `rankStoreAdapter` (`QueryEffectiveGraph`, `QueryEffectiveAdjacency`, `CountStaleScoreRows`, `MarkAllScoreRowsStale`) used to silently return empty results — the scheduler then ran a clean repair tx and incremented `helix_semantic_graph_repair_total{outcome="applied"}`, masking the deferred dependency. After this plan:

1. **Metric**: every call to a stub increments `helix_semantic_graph_repair_total{outcome="stub_no_data"}`. The closed-enum on the existing counter expands by exactly one value; no new family means no new dashboard / alerting setup churn.
2. **Log**: a `sync.Once`-gated WARN log fires AT MOST ONCE per `(repoID, method)` pair per process. Subsequent calls from the same workspace+method are silent — log volume stays bounded.
3. **TODO(phase-64) anchor**: a structured comment block immediately above the four stubs names the gap, references this plan ID (62-08), and points at the Phase 64 plan that will land real queries.

## Architecture

```text
rankStoreAdapter
├── store          *semanticstore.Store     (production reads)
└── stubObserve    *stubObserveState        (Phase 64 read harness)
                   ├── metrics  graphpkg.MetricsSink   (= *obs.Metrics in production)
                   ├── logger   *slog.Logger
                   └── onceMap  map[stubObserveKey]*sync.Once   (per (repo, method))
```

Each of the four stubs:

```go
func (a *rankStoreAdapter) QueryEffectiveGraph(_ context.Context, repoID, _ string) (..., error) {
    a.stubObserve.emit(repoID, "QueryEffectiveGraph")
    return nil, map[graphpkg.NodeID]map[graphpkg.NodeID]float64{}, nil
}
```

`emit()` is nil-safe: a zero-value receiver is silent so production paths that construct an adapter without metrics/logger keep working.

## Closed-enum expansion

`helix_semantic_graph_repair_total` outcome label set:

```text
{applied, frontier_overflow, preempted, error, stub_no_data}
```

Updated at four sites in `internal/obs/metrics.go`:

| Site | Update |
|------|--------|
| Field doc-comment (line 150-153) | Lists 5 values + 62-08 origin |
| `Help:` string in registration (line 372) | Operator-facing prose extends to `stub_no_data` and credits 62-08 |
| `graphRepairOutcomes` allowlist (line 720) | Drop-on-unknown helper now accepts `stub_no_data` |
| Helper godoc (line 766-770) | Names the call sites and the gap doc |

Test fixture (`internal/obs/metrics_labels_test.go`) primes the new value so the bounded-label lint scans it (Pitfall #3 — empty families are dropped by `Gather()`).

## Operator dashboard hint

Until Phase 64 lands real queries, the canary signal is:

```promql
sum(rate(helix_semantic_graph_repair_total{outcome="stub_no_data"}[5m]))
```

A non-zero rate proportional to scheduler activity = "rank engine wired but data not yet flowing — Phase 64 is still pending." Once Phase 64 ships and the four reads are wired, this rate goes to 0 permanently.

For the once-warn log, search log streams for `phase=64 see="62-VERIFICATION.md truth #21"`. Each unique `(repo_id, method)` pair appears once per process lifetime.

## Tests

| Test | What it asserts | Result |
|------|-----------------|--------|
| `TestRankStoreAdapter_StubObservability_Metric` | All 4 stubs increment `stub_no_data` once each (and never `applied`) | PASS |
| `TestRankStoreAdapter_StubObservability_OnceWarnPerWorkspace` | 5 calls on repo-A + 5 on repo-B = exactly 2 WARN records (once-gated) | PASS |
| `TestRankStoreAdapter_StubObservability_PerMethodGate` | 4 distinct methods on same workspace = 4 distinct WARN records | PASS |
| `go test ./internal/daemon/... -count=10 -run TestRankStoreAdapter_StubObservability` | No flake across 10 iterations | PASS |
| `go test ./internal/obs/... -count=1` | `TestMetricsLabelsAllowlist` accepts `stub_no_data` (carve-out unchanged — `outcome` is in `AllowedLabels`) | PASS |
| `go test ./internal/daemon/... ./internal/semantic/graph/... -count=1` | No regression on 18 verified Phase 62 must-haves | PASS |
| `go vet ./internal/daemon/... ./internal/obs/...` | Clean | PASS |
| `go build ./cmd/helix` | Clean (pre-existing tree-sitter Swift macro warning unrelated) | PASS |

## Deviations from Plan

None — plan executed exactly as written. The only minor adjustment: the `recordingMetricsSink` test helper implements the actual 4-method `graphpkg.MetricsSink` interface (not the 5 methods the plan example showed; `SemanticTypesResolutionInc` is on `*obs.Metrics` but is not part of `graphpkg.MetricsSink`). Compile-time assertion `var _ graphpkg.MetricsSink = (*recordingMetricsSink)(nil)` catches drift.

## Commits

| Hash | Type | Description |
|------|------|-------------|
| `9bd5fdb8` | test | Add failing observability tests for rankStoreAdapter stubs (RED) |
| `066da472` | feat | Add stub_no_data observability to rankStoreAdapter (GREEN) |

## TDD Gate Compliance

Plan was `type: tdd`. Both gates honoured:
- **RED**: `9bd5fdb8 test(62-08): add failing observability tests` — three tests added, all failed (`got 0, want 4` / `got 0, want 2` / `got 0, want 4`).
- **GREEN**: `066da472 feat(62-08): add stub_no_data observability` — same three tests now pass; `count=10` confirms no flake.
- **REFACTOR**: not needed; harness landed in its final shape.

## Self-Check: PASSED

Verified post-write:

- `internal/daemon/rank_wiring_test.go` — FOUND
- `internal/daemon/rank_wiring.go` — has `TODO(phase-64)` block + 4 `stubObserve.emit` calls (FOUND)
- `internal/obs/metrics.go` — has `stub_no_data` at 4 sites (FOUND)
- `internal/obs/metrics_labels_test.go` — has `stub_no_data` prime call (FOUND)
- Commit `9bd5fdb8` — present in `git log` (FOUND)
- Commit `066da472` — present in `git log` (FOUND)
- All target tests pass; full daemon + semantic graph regression PASS.
