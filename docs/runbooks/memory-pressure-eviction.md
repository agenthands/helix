# Runbook: Memory-pressure-driven worker eviction

> The lspool is evicting workers because the host is under memory pressure
> (Linux cgroups / macOS `vm_stat`). Sustained pressure evictions can lead
> to thrash (evict → respawn → evict) which trips the circuit breaker;
> cross-link to [`ErrCircuitOpen.md`](./ErrCircuitOpen.md) if both fire.

## Symptoms

- Daemon log contains lines like:
  ```
  level=WARN msg="evicting worker due to memory pressure" language=java rss_pct=82
  ```
- Tool latency increases as workers are dropped and then respawned cold.
- `serena_lspool_evictions_total{reason="pressure"}` is growing.

## Inspect

```bash
serena status --verbose

curl -s http://127.0.0.1:9100/metrics \
  | grep '^serena_lspool_evictions_total{.*reason="pressure"'

# Resident memory (process collector — registered alongside *Metrics.Registry)
curl -s http://127.0.0.1:9100/metrics | grep '^process_resident_memory_bytes'

# Workers gauge — drops faster than steady state confirms eviction is active
curl -s http://127.0.0.1:9100/metrics | grep '^serena_lspool_workers'
```

Cross-check host-level memory at the same timestamp:

```bash
vm_stat                    # macOS
cat /proc/meminfo | head   # Linux
```

## Triage

1. **Host genuinely pressured, or threshold too sensitive?** Compare
   `vm_stat` / `/proc/meminfo` snapshots against the eviction spikes. If the
   host has plenty of free RAM but Serena is evicting anyway, the threshold
   is too low for this machine.
2. **Concentrated on one heavy LS** (e.g. jdtls)?
   ```bash
   curl -s http://127.0.0.1:9100/metrics \
     | grep '^serena_lspool_evictions_total{.*reason="pressure"'
   ```
   The `language=` label tells you which one. jdtls is the usual suspect.
3. **Restarts following the evictions (thrash)?**
   ```bash
   curl -s http://127.0.0.1:9100/metrics | grep '^serena_lspool_restarts_total'
   ```
   If yes, you're in evict→respawn→evict territory and the breaker is at
   risk of opening — fix the threshold/cap, not just symptoms.

## Remediate

- Raise the memory-pressure threshold in `~/.serena/serena_config.yml`:
  ```yaml
  lspool:
    eviction:
      memory_pressure_threshold_pct: 85   # default 75
  ```
- Cap concurrent heavy LSes to lower the steady-state working set:
  ```yaml
  lspool:
    max_workers_per_language: 1
  ```
- Apply with `pkill -USR1 serena`.

## Verify

Snapshot, wait ~10 minutes under similar load, snapshot again:

```bash
curl -s http://127.0.0.1:9100/metrics \
  | grep '^serena_lspool_evictions_total{.*reason="pressure"'
```

Counter should be stable, and `serena_lspool_workers` should hold steady at
the configured cap rather than oscillating.

## Escalate

File an issue at https://github.com/postfix/serena/issues with:

- `curl -s http://127.0.0.1:9100/metrics | grep -E 'serena_lspool_(evictions|restarts|workers)'`
- The last 10 minutes of daemon log.
- `serena status --json --verbose`
- Host memory snapshot: `vm_stat` (macOS) or `cat /proc/meminfo` (Linux).
