# Phase 12: Tracing End-to-End - Discussion Log

**Date:** 2026-04-09
**Areas:** OTel SDK wiring, Span granularity, OTLP exporter lifecycle

---

## OTel SDK Wiring

| Option | Selected |
|--------|----------|
| Explicit via obs.Provider | ✓ |
| Global otel.SetTracerProvider | |
| Both: global + accessor | |

**Notes:** Test-safe, clean DI, matches Phase 10/11 pattern. otelgrpc/otelslog instantiated with explicit provider.

## Span Granularity

| Option | Selected |
|--------|----------|
| 3 spans (forwarder/daemon/kernel) | ✓ |
| 4 spans (+ LS child) | |
| Variable: 3 minimum, 4 on flag | |

**Notes:** Conservative tree. LS interactions as span events on kernel span (name=ls.request, attrs=lsp_method/language/duration). Keeps cardinality bounded.

## OTLP Exporter Lifecycle

| Option | Selected |
|--------|----------|
| Daemon-managed | ✓ |
| Explicit Init call | |
| You decide | |

**Notes:** Mirrors Phase 10 admin listener degraded-optional pattern. Construction errors log + fall back to noop. Shutdown uses dedicated 5s ctx (PITFALLS #4 flush race fix).

## Claude's Discretion
- Package layout, initialization order, attribute naming, tracer plumbing approach

## Deferred
- Per-LS child spans, exemplars, smart sampling, HTTP transport tracing
