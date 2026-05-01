---
title: Circuit breaker open (ErrCircuitOpen)
severity: critical
metric: helix_lspool_circuit_state
since_phase: Phase 11
last_reviewed: 2026-05-01
---

# Circuit breaker open (ErrCircuitOpen)

Helix's per-language LS worker pool trips a circuit breaker after consecutive worker failures exceed `degradation.restart_budget` (default 3). When the circuit is open for a language, every `tools/call` to that language fails with `ErrCircuitOpen` until decorrelated-jitter backoff expires and a single half-open probe succeeds. If the restart budget has been fully consumed, the circuit stays open until the worker is removed and a fresh one succeeds.

## Symptoms

- Tool calls return the `circuit_open` outcome — `helix_tool_calls_total{outcome="circuit_open"}` rises.
- `helix_lspool_circuit_state{language="<lang>"} == 2` on the engine dashboard (0=closed, 1=half-open, 2=open).
- Daemon logs include `circuit breaker open` at WARN, with `language=`, `failures=`, `backoff_remaining=`, and `retry_after=` in the error detail.
- One language's tools become unavailable while other languages continue to serve normally.

## Triage

```promql
# Which language's circuit is currently open? (state 2 = open)
helix_lspool_circuit_state == 2
```
*Filter the result set to find the affected language(s); a non-empty result is the trigger condition.*

```promql
# Recent crash + pressure evictions — likely cause class (5min rate)
sum by (language, reason) (rate(helix_lspool_evictions_total{reason=~"crash|pressure"}[5m]))
```
*High `crash` rate → the LS process is dying repeatedly. High `pressure` rate → memory pressure is forcing eviction.*

```promql
# Cross-reference: tool calls that hit a circuit-open outcome (5min rate)
sum by (language) (rate(helix_tool_calls_total{outcome="circuit_open"}[5m]))
```
*Confirms how many user-facing calls are being refused per language.*

```promql
# Restart pressure — top languages by restart rate (5min)
topk(5, sum by (language) (rate(helix_lspool_restarts_total[5m])))
```
*A high restart rate alongside an open circuit means the budget was exhausted before the worker stabilised.*

## Likely Causes

- **LS process crash storm** — confirmed if `helix_lspool_evictions_total{reason="crash"}` is rising. Cross-reference the [ls-crash-restart runbook](ls-crash-restart.md).
- **Memory pressure eviction** — confirmed if `helix_lspool_evictions_total{reason="pressure"}` is rising. Cross-reference the [memory-pressure-eviction runbook](memory-pressure-eviction.md).
- **Slow LS responses exceeding deadlines** — confirmed if `helix_tool_calls_total{outcome="timeout"}` co-occurs in the same window. Cross-reference the [deadline-timeouts runbook](deadline-timeouts.md).

## Remediation

1. **(operator)** Identify the root cause class via the four triage queries above (crash vs. pressure vs. timeout).
2. **(operator)** If pressure-driven: free memory on the host (close other tools, restart competing processes) or raise `degradation.memory_limit_mb` in `~/.helix/helix_config.yml`.
3. **(operator)** If crash-driven: check LS version compatibility, inspect the daemon log around the crash, and restart the daemon to clear the open circuit.
4. **(engineer)** If neither pressure nor crash explains it: temporarily increase `degradation.restart_budget` (default 3) in `~/.helix/helix_config.yml`, reproduce, and file an issue with the recovered logs and the `OpenError` detail string.

## Code references

- `internal/kernel/lspool/circuit.go:33` — `NewCircuitBreaker(language, maxBackoff, restartBudget, sink)`; `restartBudget` defaults to 3 when ≤ 0.
- `internal/kernel/lspool/circuit.go:64` — `RecordFailure`; decorrelated jitter backoff (`prevSleep*3` capped at `maxBackoff`).
- `internal/kernel/lspool/circuit.go:132` — `CircuitOpenErr`; flattens `(language, failures, backoff_remaining, retry_after)` into the error Detail.
- `internal/kernel/lspool/pool.go:22` — `RestartBudget` field on `PoolConfig` (default 3, set from `degradation.restart_budget`).
- `internal/kernel/lspool/pool.go:320` — `cb = NewCircuitBreaker(language, 5*time.Minute, p.config.RestartBudget, p.metrics)`; the 5-minute cap on backoff lives at this call site.
