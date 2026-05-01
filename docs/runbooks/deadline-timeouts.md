---
title: Deadline timeouts (tool budget exceeded)
severity: warning
metric: helix_tool_calls_total
since_phase: Phase 11
last_reviewed: 2026-05-01
---

# Deadline timeouts (tool budget exceeded)

Helix's `TelemetryMiddleware` injects a per-tool deadline into the request context via a `BudgetFunc` derived from `DegradationConfig`. Tools that exceed their class deadline return `context deadline exceeded` and are recorded with `outcome=timeout` on `helix_tool_calls_total`. The five timeout classes — read, search, edit, index, diagnostics — each have a separate, operator-tunable knob in `~/.helix/helix_config.yml`.

## Symptoms

- `helix_tool_calls_total{outcome="timeout"}` rate rises for one or more tools.
- p95 of `helix_tool_duration_seconds` approaches or exceeds the configured class deadline.
- Operators or end users report "tool returned context deadline exceeded".
- Affected tool class is consistent (e.g., only `search_*` tools, or only `replace_symbol_body`).

## Triage

```promql
# Tools currently hitting timeout outcome (5min rate)
sum by (tool_name) (rate(helix_tool_calls_total{outcome="timeout"}[5m]))
```
*Identifies which tools are timing out. A single tool stands out → class-specific knob; many tools across classes → host-wide problem.*

```promql
# Timeout rate as a fraction of total calls per tool
sum by (tool_name) (rate(helix_tool_calls_total{outcome="timeout"}[5m])) / sum by (tool_name) (rate(helix_tool_calls_total[5m]))
```
*Fraction near 1.0 = the tool is broken for everyone; fraction below 0.1 = transient slow path.*

```promql
# p95 latency by tool — look for tools brushing their deadline
histogram_quantile(0.95, sum by (le, tool_name) (rate(helix_tool_duration_seconds_bucket[5m])))
```
*Compare p95 against the class default (5s read, 15s search, 10s edit, 120s index, 20s diagnostics).*

## Likely Causes

- **Heavy LS workload** (large repo, slow LS startup) — confirm if p95 latency is consistently near the deadline, especially for `search_*` and `index` tool classes.
- **Network partition between Helix and an external LS** — confirm via process logs (look for repeated reconnect attempts or LSP request retries).
- **Misconfigured low timeouts** — confirm by reading the `degradation:` block in `~/.helix/helix_config.yml`; values lower than the documented defaults will cause spurious timeouts.

## Remediation

1. **(operator)** Identify the affected tool class via the per-tool timeout-rate query above.
2. **(operator)** Adjust the corresponding `DegradationConfig` knob in `~/.helix/helix_config.yml`. Defaults: `degradation.timeout_read=5s` / `degradation.timeout_search=15s` / `degradation.timeout_edit=10s` / `degradation.timeout_index=120s` / `degradation.timeout_diagnostics=20s`. For monorepos, doubling `timeout_search` and raising `timeout_index` to 300s is typical.
3. **(operator)** Restart the daemon so the new budgets take effect.
4. **(engineer)** If a single tool consistently misses its deadline by more than 2× even after tuning, the problem is kernel-side, not config-side — file an issue with traces and the relevant per-tool latency histogram.

## Code references

- `internal/mcp/middleware.go:113` — `BudgetFunc` type; `nil` disables deadline injection entirely.
- `internal/mcp/middleware.go:277` — `TelemetryMiddleware` constructor; captures `provider.Metrics()` and the tracer once per install.
- `internal/mcp/middleware.go:304-313` — D-01 deadline injection block: `ctx, budgetCancel = context.WithTimeout(ctx, budget)` runs before the tracing span starts so the timeout covers both span and handler.
- `internal/config/config.go:27-34` — `DegradationConfig` struct: `TimeoutRead` / `TimeoutSearch` / `TimeoutEdit` / `TimeoutIndex` / `TimeoutDiagnostics`.
- `internal/config/defaults.go:25-29` — default values: 5s / 15s / 10s / 120s / 20s.
- `USAGE.md:816-820` — operator-facing knob table mapping config keys to tool classes.
