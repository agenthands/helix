---
phase: 70
plan: 03
subsystem: live-pipeline
tags: [coalescer, live-pipeline, metrics, observability, wave1]
requires:
  - internal/semantic/live/coalescer
  - internal/semantic/live/service
  - internal/obs
provides:
  - "Coalescer.FlushNow synchronous flush API"
  - "Service.FlushNow workspace delegator"
  - "obs.Metrics.IncrementalRefreshFallback CounterVec"
  - "obs.IncrementalRefreshFallbackInc helper (drops unknown reasons)"
  - "4 IncrementalRefreshFallbackReason* package constants"
affects:
  - plan 70-04 (collectCandidatePaths emission site for the new metric)
  - plan 70-05 (refresh tool calls Service.FlushNow before overlay drain)
tech-stack:
  added: []
  patterns:
    - "closed-enum drop-on-unknown helper (mirrors LiveFileFactDiffInc / SemanticLiveUpdatesInc)"
    - "labels carve-out + family priming (Phase 53 D-13 Pitfall #3)"
    - "synchronous flush via stop-timers + snapshot pending + release lock"
key-files:
  created: []
  modified:
    - internal/semantic/live/coalescer/coalescer.go
    - internal/semantic/live/coalescer/coalescer_test.go
    - internal/semantic/live/service/service.go
    - internal/obs/metrics.go
    - internal/obs/metrics_test.go
    - internal/obs/metrics_labels_test.go
decisions:
  - "FlushNow stops both timers under lock then snapshots+releases before Dispatch to avoid OnFlush re-entry deadlock — same pattern makeFlush already uses."
  - "Empty FlushNow still fires the hook + stamps LastFlushAt to mirror makeFlush empty-flush short-circuit (keeps idle-clock advancing)."
  - "Concurrent Enqueue races are allowed per plan acceptance: items left on c.in after a FlushNow snapshot drain on the next flush. Test uses retry-loop trailing FlushNow to assert drainable."
metrics:
  duration: ~17 minutes
  completed: 2026-05-15T11:13:00Z
---

# Phase 70 Plan 03: Coalescer FlushNow + IncrementalRefreshFallback Metric Summary

One-liner: Two Wave-1 prerequisites for the refresh tool — synchronous `Coalescer.FlushNow` + `Service.FlushNow` delegator (closes RESEARCH.md Pitfall 1 fire-and-forget race), and a closed-enum `helix_incremental_refresh_fallback_total` CounterVec with bounded {reason, repo} labels.

## What Shipped

### Task 1 — Coalescer.FlushNow + Service.FlushNow (commit `de92f350`)

- `func (c *Coalescer) FlushNow(ctx context.Context) error` in `internal/semantic/live/coalescer/coalescer.go`:
  - Acquires `c.mu`, stops both `timer` and `maxTimer` (race-safe — `Timer.Stop()` returns false on already-fired and that path re-acquires `c.mu` to see the now-empty `pending`).
  - Empty `pending` → release lock, stamp `lastFlushNanos`, fire `OnFlush`, return nil.
  - Non-empty → snapshot `pending`, reset map, release lock, then `CoalesceEvents` + `Dispatch` using the caller's `ctx` (so a cancelled refresh aborts the flush cleanly).
  - Per-event errors don't abort the batch (D-02 invariant).
- `func (s *Service) FlushNow(ctx context.Context, ws workspace.WorkspaceKey) error` in `internal/semantic/live/service/service.go`:
  - RLocks the registry, looks up `s.coalescers[ws]`, returns nil if absent (no-op for unregistered workspaces).
  - Otherwise delegates to `c.FlushNow(ctx)`.
- 3 new race-clean tests in `coalescer_test.go`:
  - `TestCoalescer_FlushNow_DrainsPendingBatch` — Enqueue with 10s debounce; FlushNow drains; OnFlush hook fires exactly once.
  - `TestCoalescer_FlushNow_EmptyIsNoop` — Run loop NOT started; FlushNow returns nil; zero dispatches.
  - `TestCoalescer_FlushNow_ConcurrentEnqueueSafe` — 8 goroutines Enqueue concurrently with FlushNow; completes well under 2s; retry-loop trailing FlushNow drains residual c.in entries.

### Task 2 — IncrementalRefreshFallback metric (commit `eec8ee28`)

- 4 package constants in `internal/obs/metrics.go`: `IncrementalRefreshFallbackReason{ColdStart,OverlayRotated,EmptyOverlay,Error}`.
- `Metrics.IncrementalRefreshFallback *prometheus.CounterVec` field, registered alongside `LiveFileFactDiff` in `newMetrics()` with name `helix_incremental_refresh_fallback_total` and labels `{reason, repo}`.
- `IncrementalRefreshFallbackInc(reason, repo)` helper — nil-safe, switch-drops unknown reasons (same pattern as `LiveFileFactDiffInc`).
- `metrics_labels_test.go` carve-out: `"helix_incremental_refresh_fallback_total": {"reason": true, "repo": true}` + family-priming call in `TestMetricsLabelsAllowlist`.
- 3 new tests in `metrics_test.go`: known reasons, drop-unknown, nil-safe.

## Verification

- `go test ./internal/obs/ ./internal/semantic/live/coalescer/ ./internal/semantic/live/service/ -race -count=1` → all green.
- `go vet ./internal/obs/... ./internal/semantic/live/...` → clean (pre-existing tree-sitter swift C warnings unrelated).
- `make vet` → clean (vet-nokernel2semantic, vet-nosemantic2kernel, vet-compact-uses-store all green).
- `TestMetricsLabelsAllowlist` stays green with the new carve-out entry.

## Deviations from Plan

None — plan executed as written. The plan's `concurrent Enqueue → FlushNow` acceptance criterion explicitly permits residual c.in events to flush in a subsequent call; the test uses a small retry loop to drain them (also documents the at-most-one-flush-call-may-miss-trailing-Enqueue contract for callers).

## Self-Check

- `[ -f internal/semantic/live/coalescer/coalescer.go ]` → FOUND
- `[ -f internal/semantic/live/service/service.go ]` → FOUND
- `[ -f internal/obs/metrics.go ]` → FOUND
- `grep 'func (c \*Coalescer) FlushNow' coalescer.go` → 1 match
- `grep 'func (s \*Service) FlushNow' service.go` → 1 match
- `grep 'helix_incremental_refresh_fallback_total' metrics.go` → matches (counter Name + comment)
- `grep 'IncrementalRefreshFallbackInc' metrics.go` → matches (helper def)
- commits `de92f350`, `eec8ee28` present in `git log --oneline`

## Self-Check: PASSED
