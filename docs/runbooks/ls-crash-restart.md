---
title: LS crash and restart loop
severity: critical
metric: helix_lspool_evictions_total
since_phase: Phase 11
last_reviewed: 2026-05-01
---

# LS crash and restart loop

Language server (LS) workers can die for many reasons — segfault, OOM kill, malformed `jdtls` workspace, `rust-analyzer` crate-graph mismatch. The Helix worker pool detects unhealthy worker state outside the normal transition graph and books an `EvictCrash` eviction. A new worker is spawned subject to `degradation.restart_budget` (default 3); once the budget is exhausted, the circuit breaker stays open until manual intervention. The handler-panic recovery in the worker ensures a crashy LS never crashes the daemon itself.

## Symptoms

- `helix_lspool_evictions_total{reason="crash"}` rate rises.
- `helix_lspool_restarts_total` rate rises while the budget is not yet exhausted.
- `helix_lspool_workers` gauge dips for the affected language between crash and respawn.
- Daemon logs include `evicting unhealthy worker` at WARN, with state context, and possibly a recovered-panic ERROR with stack trace.

## Triage

```promql
# Crash eviction rate by language (5min)
sum by (language) (rate(helix_lspool_evictions_total{reason="crash"}[5m]))
```
*Identifies which language's LS is crashing.*

```promql
# Restart rate alongside — crashes that successfully respawned
sum by (language) (rate(helix_lspool_restarts_total[5m]))
```
*If restart rate matches crash rate, the budget is keeping up. If it lags, the budget will exhaust and the circuit will open.*

```promql
# Worker count drop-off — gauge view per language
helix_lspool_workers{language=~"$language"}
```
*Sustained dips indicate the pool is falling behind; flat zero indicates the circuit has tripped.*

## Likely Causes

- **LS binary OOM** — confirm by checking `dmesg | tail` on Linux or `Console.app` (subsystem `kernel`) on macOS for the LS process getting killed.
- **Repository-specific LS bug** (e.g., `jdtls` + corrupted workspace, `rust-analyzer` + crate-graph mismatch) — confirm by reproducing on a different repo or a clean clone.
- **Helix daemon panic-recovered handler** — recovered panics are logged at ERROR with the recovered value and method name. The daemon never crashes from these, but an LS-side bug is still firing.

## Remediation

1. **(operator)** Inspect daemon logs around the crash for the LS exit code, signal, and any nearby ERROR-level recovered panic.
2. **(operator)** If repository-specific: open the repo with the LS standalone (e.g., `gopls` or `rust-analyzer` directly, outside Helix) to confirm the LS — not Helix — is the failing component.
3. **(operator)** If OOM: free memory or move the workload to a larger host; cross-reference the [memory-pressure-eviction runbook](memory-pressure-eviction.md).
4. **(engineer)** If a Helix-side panic is in the recovery log: file an issue with the recovered stack trace and the method name from the log line.

## Code references

- `internal/kernel/lspool/pool.go:416` — crash booking: `p.evictWorkerLocked(id, w, EvictCrash)` fires when an unhealthy worker state is detected outside the normal transition graph.
- `internal/kernel/lspool/circuit.go:93` — `RecordSuccess` resets the failure counter, backoff, and probe flag after the next successful op.
- `internal/kernel/lspool/worker.go:434` — comment introducing handler-panic recovery (the actual `recover()` is at line 454); panics are recovered, logged at ERROR, and never crash the daemon.
- `internal/kernel/lspool/circuit.go:64` — `RecordFailure` decorrelated jitter backoff governs the restart cadence between crashes.
