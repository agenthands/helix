# Runbook: LS crash / restart loop

> A language-server worker is crashing and being respawned by the lspool
> supervisor. Sustained crash-restart loops degrade tool latency and can trip
> the circuit breaker (see [`ErrCircuitOpen.md`](./ErrCircuitOpen.md)).

## Symptoms

- Specific languages report frequent disconnects from agent tool calls.
- Daemon log contains repeated `worker spawn` / `worker exit` pairs for the
  same language:
  ```
  level=INFO msg="lspool worker spawned" language=java pid=12345
  level=WARN msg="lspool worker exited" language=java pid=12345 reason=crash
  ```
- `serena_lspool_restarts_total` and
  `serena_lspool_evictions_total{reason="crash"}` are both growing.

## Inspect

```bash
serena status --verbose

# Restart counter (growth across two snapshots = active loop)
curl -s http://127.0.0.1:9100/metrics | grep '^serena_lspool_restarts_total'

# Evictions split by reason — confirms crash vs. pressure
curl -s http://127.0.0.1:9100/metrics | grep '^serena_lspool_evictions_total'
```

## Triage

1. **Single language or multiple?**
   - Single → likely an LS-binary issue (version mismatch, corrupt cache, bad
     `PATH` override).
   - Multiple → host-level problem (memory, file-descriptor limits, OOM
     killer). Cross-link to
     [`memory-pressure-eviction.md`](./memory-pressure-eviction.md).
2. **Eviction reason `crash` or `pressure`?** Continue here for `crash`;
   route to memory-pressure runbook for `pressure`.
3. **Auto-managed LS or a `PATH` override?** `PATH` overrides are the most
   common foot-gun — an old or incompatible binary first on `PATH`. Check
   `~/.serena/serena_config.yml` for any `ls_binary_override` entries.

## Remediate

- Force a fresh worker spawn (drops the crashy worker so the next acquire
  spins up a clean one):
  ```bash
  pkill -USR1 serena
  ```
- Pin to the auto-managed LS by removing the `PATH` override in
  `~/.serena/serena_config.yml`:
  ```yaml
  languages:
    java:
      ls_binary_override: ""   # remove PATH override; use auto-managed
  ```
- Cap concurrent workers per language to reduce thrash while you investigate:
  ```yaml
  lspool:
    max_workers_per_language: 2
  ```
- Apply with `pkill -USR1 serena`.

## Verify

Snapshot the restart counter, wait ~10 minutes, snapshot again:

```bash
curl -s http://127.0.0.1:9100/metrics | grep '^serena_lspool_restarts_total'
```

The numeric value for the previously-affected language should be stable
across the two snapshots. Same goes for
`serena_lspool_evictions_total{reason="crash"}`.

## Escalate

File an issue at https://github.com/postfix/serena/issues with:

- `curl -s http://127.0.0.1:9100/metrics | grep -E 'serena_lspool_(restarts|evictions|circuit)'`
- The last 10 minutes of daemon log.
- `serena status --json --verbose`
