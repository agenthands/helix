# Runbook: Tool-call deadline timeouts

> Tool calls are exceeding their per-tool deadline budget. The MCP middleware
> classifies the outcome as `timeout` and the agent sees a deadline-exceeded
> error. Sustained timeouts often correlate with a circuit-open event
> downstream — check [`ErrCircuitOpen.md`](./ErrCircuitOpen.md) if both fire.

## Symptoms

- Agent receives "deadline exceeded" or "context canceled" errors from tool
  calls.
- Daemon log contains lines like:
  ```
  level=WARN msg="tool call timeout" tool=find_symbols duration=10.0s deadline=10s
  ```
- The `outcome="timeout"` share of `serena_tool_calls_total` is rising.

## Inspect

```bash
serena status --verbose

# Timeout share, grouped by tool
curl -s http://127.0.0.1:9100/metrics \
  | grep '^serena_tool_calls_total{.*outcome="timeout"'

# Latency context: histogram bucket counts per tool
curl -s http://127.0.0.1:9100/metrics \
  | grep '^serena_tool_duration_seconds_bucket' | head -40
```

> The MCP TelemetryMiddleware (`internal/mcp/middleware.go`, `outcomeEnum`)
> emits the closed enum
> `{success, invalid_args, not_found, circuit_open, ls_crash, timeout, internal}`.
> Anything other than `success` is a candidate for triage.

## Triage

1. **Localized to one tool?** Compare row counts in the `outcome="timeout"`
   grep above. A single hot tool name → that tool's deadline is too tight or
   it's hitting a slow LS.
2. **Correlated with a circuit-open event?** Re-run the inspection from
   [`ErrCircuitOpen.md`](./ErrCircuitOpen.md). If circuit is open on the
   language a tool routes to, fix that first.
3. **Per-tool deadline configured?** Inspect `mcp.tool_deadlines:` overrides
   in `~/.serena/serena_config.yml`.
4. **Session-activation timing out?** Distinct from tool timeout — look at
   `serena_session_lifecycle_total` with `phase="timeout"`:
   ```bash
   curl -s http://127.0.0.1:9100/metrics \
     | grep 'serena_session_lifecycle_total{.*phase="timeout"'
   ```

## Remediate

- Raise the per-tool deadline in `~/.serena/serena_config.yml`:
  ```yaml
  mcp:
    tool_deadlines:
      find_symbols: "30s"   # default 10s
  ```
- For session-activation timeouts, raise the activation budget:
  ```yaml
  kernel:
    activation_timeout: "60s"
  ```
- Apply with `pkill -USR1 serena` (graceful re-spawn).

## Verify

```bash
curl -s http://127.0.0.1:9100/metrics \
  | grep '^serena_tool_calls_total{.*outcome="timeout"'
```

Counter should stop growing across two snapshots ~30 seconds apart on the
previously-hot tool.

## Escalate

File an issue at https://github.com/postfix/serena/issues with:

- `curl -s http://127.0.0.1:9100/metrics | grep -E 'serena_tool_(calls|duration)'`
- The last 10 minutes of daemon log.
- `serena status --json --verbose`
