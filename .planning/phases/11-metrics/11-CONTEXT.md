# Phase 11: Metrics - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Expose Prometheus-scrapeable RED metrics for all 38 MCP tools and lspool health with a bounded-label contract enforced by CI. Adds `/metrics` endpoint on the Phase 10 admin listener.

</domain>

<decisions>
## Implementation Decisions

### Histogram Buckets
- **D-01:** Use `prometheus.DefBuckets` (`.005 / .01 / .025 / .05 / .1 / .25 / .5 / 1 / 2.5 / 5 / 10` seconds) for all tool latency histograms.
- **D-02:** Single bucket set for all tools (no per-category tuning). Covers 95% of observed distribution from Phase 9 baseline; revisit in v1.3 if p99 resolution is insufficient.

### Cardinality Lint
- **D-03:** CI-time enforcement via Go test `metrics_labels_test.go` that enumerates all registered vectors and asserts each label name is in the allowlist.
- **D-04:** Allowlist (immutable constant): `tool_name`, `profile`, `mode`, `language`, `outcome`. Any other label fails the test.
- **D-05:** NO runtime panic wrapper — the list is known at compile time; CI catches drift before merge. Simpler and faster than defense-in-depth.

### Middleware Placement
- **D-06:** Central `TelemetryMiddleware` in `internal/mcp/middleware.go` wraps ALL `tools/call` requests. Originally specified as running BEFORE `ProfileFilterMiddleware` so denied calls would still emit metrics — but post-revision investigation confirmed ProfileFilterMiddleware only filters `tools/list` in v1.2 (no deny path at tool-call time). Middleware ordering between Telemetry and ProfileFilter is therefore INDEPENDENT and documented as such in code.
- **D-07:** Middleware resolves all 5 labels: `tool_name` from request, `profile`+`mode` from session, `language` from active workspace, `outcome` from result status.
- **D-08:** lspool emits its OWN gauges directly via `obs.Provider` — `internal/kernel/lspool/metrics.go` registers workers/evictions/restarts/circuit-state gauges. Colocated with pool internals. Middleware doesn't peek through accessors.

### RED Metrics
- **D-09:** Rate: implicit from histogram count
- **D-10:** Errors: `serena_tool_calls_total{tool_name, profile, mode, language, outcome}` counter where `outcome` ∈ `{success, invalid_args, not_found, circuit_open, ls_crash, timeout, internal}` (7 values; `denied` removed post-revision — ProfileFilterMiddleware only filters tools/list in v1.2, so there is no deny path at tool-call time. Reintroduce in v1.3 if per-call filtering lands.)
- **D-11:** Duration: `serena_tool_duration_seconds{tool_name, profile, mode, language}` histogram

### lspool Gauges
- **D-12:** `serena_lspool_workers{language}` — active worker count per language
- **D-13:** `serena_lspool_evictions_total{language, reason}` counter — `reason` ∈ `{idle, pressure, crash, shutdown}`
- **D-14:** `serena_lspool_circuit_state{language}` — 0=closed, 1=half-open, 2=open
- **D-15:** `serena_lspool_restarts_total{language}` counter

### Go Runtime Collectors
- **D-16:** Use `collectors.NewGoCollector()` (goroutines, GC, memory) — standard prom pattern, zero config

### Endpoint
- **D-17:** `/metrics` registered on Phase 10 admin listener (loopback-only). No auth in v1.2.

### Claude's Discretion
- Exact initialization of metric vectors (`init()` vs explicit constructor called in obs.Provider.Init)
- Whether to expose `obs.Provider.Metrics()` accessor or package-level helpers
- Hot-path optimizations (pre-bound label values vs per-call lookups) — target: ≤ +3 allocs/op vs Phase 10 baseline

</decisions>

<canonical_refs>
## Canonical References

### Milestone Research
- `.planning/research/STACK.md` — `prometheus/client_golang` v1.23.x, not OTel metrics
- `.planning/research/ARCHITECTURE.md` — Metrics middleware slots before ProfileFilterMiddleware
- `.planning/research/PITFALLS.md` — #2 cardinality explosion, #8 default bucket trap

### Phase 9 Baseline
- `test/bench/baselines/v1.1-github-hosted.txt` — original baseline
- `test/bench/baselines/v1.2-phase10-github-hosted.txt` — Phase 10 reference (Phase 11 must not exceed this + tiered gate)
- `test/bench/cmd/benchgate/main.go` — regression gate, --baseline/--new/--release-tier flags

### Phase 10 Observability
- `internal/obs/` — Provider interface, ContextHandler, SpanContext
- `internal/daemon/telemetry.go` — admin listener mux (Phase 11 adds `/metrics` to this mux)
- `cfg.Observability.{AdminAddr, EnablePprof}` — config already wired

### Existing MCP
- `internal/mcp/middleware.go` — ProfileFilterMiddleware pattern to mirror
- `internal/mcp/server.go` — Where middleware chain is installed
- `internal/kernel/lspool/pool.go` — Where lspool gauges get wired

### External
- [prometheus/client_golang docs](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus)
- [Prometheus naming conventions](https://prometheus.io/docs/practices/naming/)
- [Robust Perception: Cardinality is Key](https://www.robustperception.io/cardinality-is-key/)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 10 `obs.Provider` interface — Phase 11 extends with metrics methods or adds sibling interface
- Phase 10 admin listener mux — Phase 11 registers `/metrics` handler
- Phase 10 ContextHandler — tracing integration point (Phase 12 later)
- `internal/mcp/middleware.go` — ProfileFilterMiddleware is the pattern; TelemetryMiddleware follows it
- Phase 9 benchmark harness — used to verify metric overhead ≤ +3 allocs/op
- benchgate CLI — runs delta gate vs v1.2-phase10 baseline

### Established Patterns
- Noop-default providers; obs.Provider gets constructed in daemon.New()
- Middleware chain in MCP server installed via InstallMiddleware()
- Config via koanf in internal/config/; extended with `Observability.MetricsEnabled bool` (defaults to true when AdminAddr is set)

### Integration Points
- New file `internal/obs/metrics.go` — metric vector constructors
- New file `internal/obs/metrics_labels_test.go` — CI label allowlist enforcement
- New file `internal/kernel/lspool/metrics.go` — lspool gauges (per D-08)
- Modified `internal/mcp/middleware.go` — TelemetryMiddleware added before ProfileFilter
- Modified `internal/daemon/telemetry.go` — register `/metrics` handler on admin mux
- Modified `internal/daemon/daemon.go` — wire obs.Metrics() into middleware install
- New benchmark: `test/bench/metrics_bench_test.go` or extend existing — verify ≤ +3 allocs/op

</code_context>

<specifics>
## Specific Ideas

- CI-time label lint via Go test (catch-before-merge) — no runtime panic
- Default histogram buckets (simple; tune in v1.3 if needed)
- Central middleware for tool RED; lspool owns its own gauges
- `outcome` label enumeration is closed (8 values) to prevent unbounded error-string cardinality

</specifics>

<deferred>
## Deferred Ideas

- SLO-tuned per-category histogram buckets (v1.3+)
- Native histograms (Prometheus 2.40+) — requires scrape-side support
- Metric exemplars linking histograms to trace IDs (requires Phase 12 tracing first)
- Runtime cardinality budget self-monitoring

</deferred>

---

*Phase: 11-metrics*
*Context gathered: 2026-04-09*
