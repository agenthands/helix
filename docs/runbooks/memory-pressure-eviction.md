---
title: Memory-pressure eviction
severity: warning
metric: helix_lspool_evictions_total
since_phase: Phase 11
last_reviewed: 2026-05-01
---

# Memory-pressure eviction

On Linux, Helix reads `/proc/pressure/memory` (PSI avg10) to detect host memory pressure; on macOS, it polls `vm_stat` for the free-vs-inactive page ratio. Under sustained pressure the worker pool proactively evicts idle or low-score LS workers (`EvictPressure`) before the OS OOM killer fires. This is normal behaviour during memory contention — operators should only investigate when the pressure-eviction rate stays high without an obvious external cause.

## Symptoms

- `helix_lspool_evictions_total{reason="pressure"}` rate rises.
- System-level memory pressure is visible to other processes (slow disk swap, swap-out activity, other apps reporting memory pressure).
- `helix_lspool_workers` dips even though no LS process actually crashed.
- Daemon logs include `evicting worker: RSS exceeds hard cap` or `evicting zero-score worker` at WARN.

## Triage

```promql
# Pressure-driven eviction rate by language (5min)
sum by (language) (rate(helix_lspool_evictions_total{reason="pressure"}[5m]))
```
*Identifies which language's LS is being shed under pressure.*

```promql
# Compare to all eviction reasons — what fraction is pressure?
sum (rate(helix_lspool_evictions_total{reason="pressure"}[5m])) / sum (rate(helix_lspool_evictions_total[5m]))
```
*A fraction near 1.0 means pressure is the dominant eviction cause; near 0 means crashes/idle dominate.*

## Likely Causes

- **Other processes competing for memory** (Claude Code, IDE, build tooling) — confirm via `ps aux --sort=-rss | head` (Linux) or Activity Monitor → Memory tab (macOS).
- **Helix LS workers individually large** (e.g., warm `jdtls` cache) — confirm by checking per-worker RSS in daemon logs (`rss_mb=` field on eviction lines).
- **Host is undersized for the LS mix** — confirm by raising memory and re-observing; if the pressure rate falls sharply, capacity was the cause.

## Remediation

1. **(operator) Linux:** `cat /proc/pressure/memory` — interpret the `avg10` column on the `some` line; values consistently above 10 indicate sustained pressure.
2. **(operator) macOS:** `vm_stat 5` — watch the "Pages free" and "Pages inactive" columns; a steady fall in free pages with low inactive pages is the pressure signature.
3. **(operator)** Free memory by closing competing tools, or relaunch the daemon with a higher memory ceiling (`degradation.memory_limit_mb` in `~/.helix/helix_config.yml`, or the `GOMEMLIMIT` env var).
4. **(engineer)** If the eviction rate stays high under no apparent host-level pressure, verify the thresholds in `internal/kernel/lspool/pressure_linux.go` / `pressure_darwin.go` — the platform detector may be overly sensitive on this kernel/OS version.

## Code references

- `internal/kernel/lspool/pressure_linux.go:22` — `os.ReadFile("/proc/pressure/memory")`; Linux PSI source.
- `internal/kernel/lspool/pressure_darwin.go:24` — `exec.Command("vm_stat").Output()`; macOS pressure detector.
- `internal/kernel/lspool/pool.go:403` — first `evictWorkerLocked(..., EvictPressure)` call site (RSS hard cap path).
- `internal/kernel/lspool/pool.go:432` — second `evictWorkerLocked(..., EvictPressure)` call site (zero-score worker path).
- `internal/kernel/lspool/pool.go:452` — third `evictWorkerLocked(..., EvictPressure)` call site (oldest-idle fallback).
- `internal/kernel/lspool/pressure.go:4-16` — `PressureLevel` enum (None / Low / Medium / High / Critical).
- `internal/kernel/lspool/metrics.go:38` — `EvictPressure = "pressure"` constant emitted as `reason` label on `helix_lspool_evictions_total`.
