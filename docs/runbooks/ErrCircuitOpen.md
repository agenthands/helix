# Runbook: ErrCircuitOpen

> The lspool circuit breaker has opened for one or more languages. New
> acquires for the affected language fail with a typed `ErrCircuitOpen` until
> the breaker half-opens.

## Symptoms

- Tool calls against a specific language (e.g. Java, Rust) return
  `ErrCircuitOpen` to the agent.
- `serena status` reports the language as `degraded` or `circuit_open`.
- Daemon log contains lines like:
  ```
  level=WARN msg="lspool circuit opened" language=java consecutive_failures=5
  ```

## Inspect

Local-only — no external services required.

```bash
# Quick health view
serena status --verbose

# Direct registry view (replace 9100 with your admin_addr port)
curl -s http://127.0.0.1:9100/metrics \
  | grep -E '^serena_lspool_(circuit|restarts|evictions)'
```

The `serena_lspool_circuit_state` line shows one entry per language with
value `0` (closed), `1` (half-open), or `2` (open). Anything `>= 2` is the
problem.

## Triage

1. **Is the LS process crashing?** Check the restart counter:
   ```bash
   curl -s http://127.0.0.1:9100/metrics | grep '^serena_lspool_restarts_total'
   ```
   A non-zero growth between two snapshots a few seconds apart means
   workers are dying.

2. **Is memory pressure killing workers?** Check eviction reasons:
   ```bash
   curl -s http://127.0.0.1:9100/metrics \
     | grep '^serena_lspool_evictions_total{.*reason="pressure"'
   ```
   If non-zero growth: cross-link to
   [`memory-pressure-eviction.md`](./memory-pressure-eviction.md).

3. **Is the host I/O- or fd-saturated?** `process_open_fds` and
   `process_resident_memory_bytes` (also in `/metrics`) give a fast view.

## Remediate

- If a single language is affected and the LS binary is misbehaving, restart
  the daemon to force a fresh worker spawn:
  ```bash
  pkill -USR1 serena   # graceful re-spawn (or: launchctl/systemctl restart)
  ```
- If the breaker is too aggressive for your workload, raise the threshold in
  `~/.serena/serena_config.yml`:
  ```yaml
  lspool:
    circuit_breaker:
      consecutive_failures: 8   # default 5
  ```
  Apply with `pkill -USR1 serena`.

## Verify

After remediation:

```bash
curl -s http://127.0.0.1:9100/metrics \
  | grep '^serena_lspool_circuit_state'
```

All language entries should report `0`. Take a second snapshot 30 seconds
later and confirm `serena_lspool_restarts_total` is no longer growing.

## Escalate

File an issue at https://github.com/postfix/serena/issues with:

- `curl -s http://127.0.0.1:9100/metrics | grep -E 'serena_lspool_(circuit|restarts|evictions)'`
- The last 10 minutes of daemon log (`~/.serena/logs/daemon.log` or platform
  equivalent).
- `serena status --json --verbose`
