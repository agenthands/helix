# Phase 11: Metrics - Discussion Log

**Date:** 2026-04-09
**Areas:** Histogram buckets, Cardinality lint, Middleware placement

---

## Histogram Buckets

| Option | Selected |
|--------|----------|
| Default (5ms-10s) | ✓ |
| SLO-tuned from Phase 9 | |
| Per-tool-class | |

**Notes:** Simple wins; revisit in v1.3 if p99 resolution insufficient.

## Cardinality Lint

| Option | Selected |
|--------|----------|
| Go test allowlist | ✓ |
| Runtime panic wrapper | |
| Both | |

**Notes:** CI-time enforcement sufficient; list is compile-time constant.

## Middleware Placement

| Option | Selected |
|--------|----------|
| Central mcp/middleware.go | ✓ |
| Per-category RegisterTools | |
| Central + lspool hooks | |

**Notes:** Central TelemetryMiddleware for tool RED. lspool owns its own gauges (Claude's discretion on follow-up question: chose direct lspool emission to avoid accessor leakage).

## Claude's Discretion
- Metric vector initialization, Provider accessor shape, hot-path optimizations

## Deferred
- Per-category buckets, native histograms, exemplars, cardinality self-monitoring
