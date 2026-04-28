# Operator runbooks

Triage-and-remediate guides for the four most common Serena failure modes.
All inspection is local-only — `serena status`, `curl /metrics`, and daemon
logs. No Prometheus, no Grafana, no Docker.

| Runbook | When |
|---|---|
| [`ErrCircuitOpen.md`](./ErrCircuitOpen.md) | Tool calls return typed `ErrCircuitOpen`; lspool breaker is open for one or more languages. |
| [`deadline-timeouts.md`](./deadline-timeouts.md) | Agent sees deadline-exceeded errors; `outcome="timeout"` share is rising. |
| [`ls-crash-restart.md`](./ls-crash-restart.md) | Specific language(s) disconnect frequently; `serena_lspool_restarts_total` and crash-evictions are growing. |
| [`memory-pressure-eviction.md`](./memory-pressure-eviction.md) | Workers being dropped under host memory pressure; `serena_lspool_evictions_total{reason="pressure"}` is non-zero. |

## Prerequisites

The daemon must expose its admin listener (which serves `/metrics`):

```yaml
# ~/.serena/serena_config.yml
observability:
  admin_addr: "127.0.0.1:9100"
```

Verify it's reachable:

```bash
curl -s http://127.0.0.1:9100/metrics | head -5
```

If you get connection-refused, `pkill -TERM serena` and let the next start
(forwarder or `serena --serve`) pick up the config.
