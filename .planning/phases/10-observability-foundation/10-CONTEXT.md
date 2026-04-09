# Phase 10: Observability Foundation - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Stand up the observability scaffolding (`internal/obs/` package, trace-aware slog handler, dedicated admin listener) with near-zero hot-path cost so later phases (11 metrics, 12 tracing) can plug into a single shim. Noop by default.

</domain>

<decisions>
## Implementation Decisions

### Admin Listener Configuration
- **D-01:** CLI flag + config layered approach. Operator can ad-hoc enable via `--admin-addr` flag without editing config files.
- **D-02:** Config schema: `cfg.Observability.AdminAddr string` (empty = disabled, `127.0.0.1:0` = auto-port, explicit address = use as-is).
- **D-03:** CLI flag `--admin-addr` overrides config (highest precedence in koanf 4-layer chain).
- **D-04:** Bind failure is non-fatal — admin listener runs in its own errgroup goroutine that logs but never returns the error to the supervisor.
- **D-05:** Default: disabled. Loopback-only when enabled (`127.0.0.1:*` only — refuse `0.0.0.0` until v1.3 auth lands).

### slog Handler Composition
- **D-06:** `obs.NewContextHandler(existingHandler slog.Handler) slog.Handler` wraps the daemon's existing handler. Existing call sites unchanged.
- **D-07:** Wrapper extracts `trace_id` and `span_id` from `context.Context` (per Phase 12 conventions) and adds them as record attributes when present.
- **D-08:** When ctx has no trace info, wrapper is pass-through (no allocations).
- **D-09:** Hot-path budget per Phase 9 baseline: ≤ +1 alloc/op vs Phase 9 measurements. Use `slog.LogAttrs` patterns to avoid boxing.

### pprof Endpoint Gating
- **D-10:** Config-time only: `cfg.Observability.EnablePprof bool`, default `false`. When `true`, `/debug/pprof/*` endpoints registered on admin listener.
- **D-11:** Admin listener is loopback-only (D-05), so EnablePprof=true on a localhost-bound listener is sufficient defense for v1.2. Admin auth deferred to v1.3.
- **D-12:** When `EnablePprof: false`, pprof handlers are NOT registered (zero attack surface, zero import bloat).

### Other Endpoints (Not Gated)
- **D-13:** `/healthz` always available on admin listener when listener is up (returns 200 + minimal status JSON).
- **D-14:** `/readyz` always available on admin listener when listener is up (returns 200 if daemon ready, 503 during startup/shutdown).

### Claude's Discretion
- Exact field names in `cfg.Observability` struct (Addr vs AdminAddr — match existing config style)
- `/healthz` and `/readyz` response body format (minimal JSON vs plain text)
- Whether to expose `obs.Provider` interface or just functions
- Where to install context handler in daemon bootstrap

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone Research
- `.planning/research/STACK.md` — `log/slog` stdlib only, no third-party logger
- `.planning/research/ARCHITECTURE.md` — Dedicated admin listener pattern, `internal/obs/` shim isolating future OTel/Prom deps
- `.planning/research/PITFALLS.md` — slog hot-path allocations (#3), flush race on SIGTERM (#4), sensitive data logging (#12)

### Phase 9 Baselines
- `test/bench/baselines/v1.1-github-hosted.txt` — Pre-instrumentation baseline (placeholder, will be re-baselined in CI)
- `test/bench/cmd/benchgate/` — Regression gate that Phase 10 must not exceed (≤ +1 alloc/op)

### Existing Code (reuse / integration points)
- `internal/daemon/daemon.go` — Where obs provider gets constructed and passed to subsystems
- `internal/config/` — koanf 4-layer config; add `Observability` struct
- `internal/cli/root.go` — Where `--admin-addr` CLI flag is registered

### External Standards
- [go.dev/blog/slog](https://go.dev/blog/slog) — Structured logging with context
- `golang.org/x/exp/slog` notes on hot-path allocation patterns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Existing daemon `slog.Logger` setup in `daemon.New()` — wrapper installs cleanly
- Errgroup-based subsystem lifecycle in daemon — admin listener slots in as another errgroup goroutine
- koanf 4-layer config — `Observability` is a new top-level section
- Phase 9 benchmark suite — measures slog hot-path allocations directly via `BenchmarkTools` and `BenchmarkMemory`

### Established Patterns
- Fail-fast for core subsystems, degraded-optional for non-critical providers — admin listener follows degraded-optional pattern (D-04)
- All packages export interfaces, not concrete types, for testability
- `//go:build integration` tag for slow tests
- `b.ReportAllocs()` mandatory in benchmarks

### Integration Points
- New package `internal/obs/` with provider, context handler, admin listener
- New config section `cfg.Observability` with AdminAddr, EnablePprof
- New CLI flag `--admin-addr` in root.go
- New file `internal/daemon/telemetry.go` housing the admin listener wiring (per ARCHITECTURE.md)
- daemon.New() instantiates obs provider and wraps existing slog handler
- Phase 9 benchmark suite re-runs with obs noop-default to confirm ≤ +1 alloc/op delta gate

</code_context>

<specifics>
## Specific Ideas

- Provider passed explicitly (no `otel.SetTracerProvider` global) — avoids test pollution
- Loopback-only listener; refuse non-loopback addresses with clear error
- pprof handlers registered conditionally on EnablePprof flag (no import unless enabled)
- Health endpoints always available when listener is up; pprof opt-in

</specifics>

<deferred>
## Deferred Ideas

- Admin listener authentication (e.g., token-based) — v1.3+
- Remote `/metrics` exposure (currently loopback-only, document reverse proxy story in Phase 14 USAGE.md)
- Alternative log formats (text/JSON/logfmt selector) — current daemon JSON is sufficient

</deferred>

---

*Phase: 10-observability-foundation*
*Context gathered: 2026-04-09*
