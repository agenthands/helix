---
phase: 12
slug: tracing-end-to-end
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-10
---

# Phase 12 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| forwarder -> daemon (gRPC) | Traceparent metadata crosses process boundary; daemon trusts forwarder-supplied trace IDs | Trace/span IDs (non-sensitive) |
| daemon -> OTLP collector | Span batch crosses loopback network; operator-trusted collector in v1.2 | Span names, durations, bounded attributes |
| kernel -> language server | JSON-RPC; LS is in-process child | No tracing data crosses this boundary |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-12-01 | Information Disclosure | kernel.tool.* span attributes | mitigate | D-07: zero SetAttributes in spanwrap.go; span name only | closed |
| T-12-02 | Information Disclosure | ls.request span events | mitigate | Allowlist: lsp.method, lsp.language, lsp.duration_ms only; no file.path/symbol.name/uri | closed |
| T-12-03 | Denial of Service | Unbounded cardinality from tool_name/profile | mitigate | Bounded by Phase 11 metrics label allowlist; span attributes reuse same values | closed |
| T-12-04 | Information Disclosure | OTLP/gRPC WithInsecure() | accept | v1.2 loopback-default; TLS configuration deferred to v1.3 | closed |
| T-12-05 | Availability | Shutdown race drops in-flight spans | mitigate | Dedicated 5s flush context.Background() + TestTraceFlushOnShutdown integration test | closed |
| T-12-06 | Tampering | Forged traceparent from forwarder | accept | Forwarder runs in user's own process tree; no multi-tenant scenario in v1.2 | closed |
| T-12-07 | Elevation of Privilege | otel.SetTracerProvider global abuse | mitigate | D-01 enforced: zero otel.SetTracerProvider/otel.GetTracerProvider calls in internal/ | closed |

*Status: open / closed*
*Disposition: mitigate (implementation required) / accept (documented risk) / transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-12-01 | T-12-04 | v1.2 targets loopback-only OTLP collector; TLS adds config complexity deferred to v1.3 | Phase 12 plan | 2026-04-10 |
| AR-12-02 | T-12-06 | Forwarder is a local process in user's tree; traceparent forgery has no security impact without multi-tenancy | Phase 12 plan | 2026-04-10 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-10 | 7 | 7 | 0 | gsd-secure-phase |

---

## Evidence

### T-12-01: No attributes on kernel sub-spans
```
grep -rn 'SetAttributes' internal/kernel/spanwrap.go -> zero matches
```

### T-12-02: ls.request allowlist only
```
worker.go:271: attribute.String("lsp.method", method)
worker.go:272: attribute.String("lsp.language", w.language)
worker.go:273: attribute.Int64("lsp.duration_ms", duration.Milliseconds())
grep 'file.path|symbol.name|uri' -> zero sensitive data matches
```

### T-12-05: Shutdown flush with dedicated context
```
shutdown.go:36: flushCtx, flushCancel := context.WithTimeout(context.Background(), 5*time.Second)
shutdown.go:37: if err := d.obs.ShutdownTracing(flushCtx); err != nil {
test/integration/trace_shutdown_test.go exists and passes
```

### T-12-07: No global TracerProvider
```
grep -rn 'otel.SetTracerProvider|otel.GetTracerProvider' internal/ -> zero actual calls (comments only)
```

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-10
