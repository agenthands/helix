---
phase: 55-obs-trace-coverage-audit
plan: 05
subsystem: observability/docs
tags: [observability, tracing, documentation, sampling]
requires:
  - 55-03  # uses tracing.go sampler facts established/confirmed by Plan 03
provides:
  - "USAGE.md ### Trace Sampling H3 subsection (head-based sampler reference doc)"
affects:
  - USAGE.md
tech-stack:
  added: []
  patterns:
    - "Pure-insert USAGE.md edit (Phase 54-05 D-02 pattern)"
key-files:
  created: []
  modified:
    - USAGE.md
decisions:
  - "Placed ### Trace Sampling AFTER ### Enable Tracing (line 746) — overrides CONTEXT.md 'before Prometheus Metrics' guidance per RESEARCH Pitfall 5 / A5; coherent operator flow Enable Tracing → Trace Sampling → Enable pprof."
  - "Documented tail-sampling explicitly as NOT-shipped future scope to set operator expectations (avoids confusion with head-only sampler)."
  - "Kept the existing 2-line YAML example in ### Enable Tracing untouched; the new section adds a richer 3-key example that includes service_name."
metrics:
  duration_minutes: 4
  completed: 2026-05-02
---

# Phase 55 Plan 05: USAGE.md Trace Sampling Subsection Summary

Added a new `### Trace Sampling` H3 subsection to USAGE.md that documents Helix's head-based `ParentBased(TraceIDRatioBased(tracing_sample_ratio))` sampler, the three relevant `ObservabilityConfig` keys (`tracing_endpoint`, `tracing_sample_ratio`, `service_name`), a behavior matrix covering the noop short-circuit (D-11/D-17) and four sampling regimes, operational guidance for prod/dev/smoke, and an explicit note that tail-sampling is future scope. Pure-insert edit — zero existing lines modified.

## What Was Done

- **Located insertion point:** Verified `### Enable Tracing` at USAGE.md:734 and the next H3 (`### Enable pprof`) at line 746 (pre-edit). Inserted the new section between the trailing prose of "Enable Tracing" and the blank line before "Enable pprof".
- **Authored ~39 lines** covering:
  - Sampler architecture (head-based, W3C tracecontext propagation, exact OTel sampler type).
  - YAML config example with all three keys (`tracing_endpoint`, `tracing_sample_ratio`, `service_name`).
  - 4-row behavior matrix: empty endpoint, ratio 0.0, ratio 0.1, ratio 1.0.
  - Operational guidance for production (1%), development (100%), smoke tests (link to runbook).
  - Explicit "NOT shipped today" tail-sampling note pointing operators at downstream OTel collector tail-sampling processors.
- **Verified facts against source:**
  - `internal/obs/tracing.go:62` confirms `ParentBased(TraceIDRatioBased(cfg.SampleRatio))`.
  - `internal/obs/tracing.go:78-80` confirms D-11 noop short-circuit when `cfg.Endpoint == ""`.
  - `internal/config/config.go:50-56` confirms YAML keys: `tracing_endpoint`, `tracing_sample_ratio`, `service_name`.

## Final Header Layout (post-edit)

```
USAGE.md:660  ### Prometheus Metrics
USAGE.md:734  ### Enable Tracing
USAGE.md:746  ### Trace Sampling      <-- NEW
USAGE.md:785  ### Enable pprof
```

Order invariant satisfied: `Enable Tracing (734) < Trace Sampling (746) < Enable pprof (785)`.

## Pure-Insert Verification

- `git diff USAGE.md | awk '/^-[^-]/'` returns empty — zero existing lines deleted/modified.
- `git show --stat HEAD` reports `1 file changed, 39 insertions(+)` (no deletions).
- This matches the Phase 54-05 D-02 pure-insert pattern.

## Acceptance Criteria Results

| Criterion | Result |
|-----------|--------|
| `grep -c '^### Trace Sampling$' USAGE.md` == 1 | PASS (1) |
| `### Enable Tracing` < `### Trace Sampling` < `### Enable pprof` | PASS (734 < 746 < 785) |
| `grep -F 'tracing_sample_ratio' USAGE.md \| wc -l` >= 2 | PASS (5) |
| `grep -F 'ParentBased(TraceIDRatioBased' USAGE.md` matches once | PASS (1) |
| `grep -iF 'tail-sampling' USAGE.md` >= 1 | PASS (2) |
| `grep -F 'noop tracer' USAGE.md` matches | PASS |
| `git diff USAGE.md | awk '/^-[^-]/'` empty | PASS |
| `### Trace Sampling` not before `### Prometheus Metrics` | PASS (746 > 660) |

## Pitfall 5 / A5 Reconciliation

CONTEXT.md originally suggested placing the new subsection "before Prometheus Metrics". RESEARCH (Pitfall 5 / Assumption A5) flagged that the existing `### Enable Tracing` H3 at line 734 was the natural anchor and that CONTEXT.md's earlier guidance pre-dated awareness of that section. The plan explicitly overrode CONTEXT.md and instructed insertion AFTER `### Enable Tracing`. Outcome: a coherent operator-facing flow (`Enable Tracing → Trace Sampling → Enable pprof`) within the existing Observability cluster, with the subsection arriving naturally as readers learn to enable tracing and immediately want to know how to control its volume.

## Deviations from Plan

None — plan executed exactly as written. Single auto task, single commit, all acceptance criteria green on first run.

## Commit

- `cbfeca94` — `docs(55-05): add ### Trace Sampling H3 to USAGE.md (after ### Enable Tracing)`

## OBS-04 Closure

Success criterion 3 of OBS-04 ("USAGE.md Observability section documents sampling configuration: ratio, head vs. tail, and how to adjust it via ObservabilityConfig") is satisfied by this plan.

## Self-Check: PASSED

- `USAGE.md` modification verified: `### Trace Sampling` H3 present at line 746, between `### Enable Tracing` (734) and `### Enable pprof` (785).
- Commit `cbfeca94` exists in `git log --oneline`.
- Pure-insert: 39 insertions, 0 deletions.
