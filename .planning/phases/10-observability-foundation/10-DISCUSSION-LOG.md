# Phase 10: Observability Foundation - Discussion Log

> **Audit trail only.**

**Date:** 2026-04-09
**Areas discussed:** Admin listener config, slog handler composition, pprof endpoint gating

---

## Admin Listener Configuration

| Option | Selected |
|--------|----------|
| Single AdminAddr field | |
| Nested Admin block | |
| CLI flag + config | ✓ |
| You decide | |

**User's choice:** CLI flag + config layered. `--admin-addr` overrides config for ad-hoc enable.

---

## slog Handler Composition

| Option | Selected |
|--------|----------|
| Wrap existing handler | ✓ |
| Replace daemon handler | |
| Both: wrap default, replace optional | |

**User's choice:** Wrap existing handler. Zero behavior change for existing call sites; pass-through when ctx has no trace info.

---

## pprof Endpoint Gating

| Option | Selected |
|--------|----------|
| Config-time only | ✓ |
| Admin profile required | |
| Both | |
| You decide | |

**User's choice:** Config-time `EnablePprof` flag. Loopback-only listener is sufficient defense for v1.2; admin auth deferred to v1.3.

---

## Claude's Discretion
- Field naming, response body format, interface vs functions, install location

## Deferred
- Admin auth, remote metrics exposure, log format selector
