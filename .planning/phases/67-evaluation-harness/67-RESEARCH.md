# Phase 67: Evaluation Harness — Research

**Researched:** 2026-05-10
**Domain:** Out-of-process agent-evaluation harness for an MCP runtime
**Confidence:** HIGH (most areas) / MEDIUM (CC json schema specifics, retention TOS surface)

## Summary

Phase 67 ships an out-of-process eval harness that drives **Claude Code CLI**
as the agent (`claude --print --output-format=json --strict-mcp-config
--mcp-config=<mode>.json --bare`) against four isolated daemon subprocesses
(`baseline / native / semantic / semantic_guarded`), merges daemon-side
TelemetryMiddleware traces with CC's JSON output, and emits per-task evidence
plus aggregate reports. The same code path also runs in-process for
`make eval-quick`.

The architecture is mostly assembly of already-shipped Helix primitives:
`config.Load` already supports CLI-overridden socket path + profile name + ad-
hoc config file; `forwarder.RunForwarder` already auto-spawns a daemon if
none is up; `phasegraph/pipelines/eval.go` already declares the canonical
10-phase eval DAG with `noopRun` placeholders waiting for real bodies; and
`TelemetryMiddleware` (now Phase 66-aware with `guardrail_warned/_blocked`
outcomes) is the daemon-side trace source-of-truth that EVAL-01 mandates. The
new code surface is small: a `cmd/helix-eval/` runner, an `internal/eval/`
runner library, the `baseline` profile YAML, the trace-merge + scorer + judge
code, and an `eval/` top-level corpus + reports tree.

**Primary recommendation:** Treat `phasegraph/pipelines/eval.go` as the
contract — fill in real `Run` bodies for the 10 phases instead of inventing a
parallel orchestrator. Drive isolation through `--socket=`, `--config=`,
`HELIX_HOME=` (project's user-config-dir lookup is `os.UserHomeDir()` so
overriding `HOME`/`USERPROFILE` is sufficient and requires zero new code).
Use `claude --bare --strict-mcp-config` for deterministic CI runs.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Corpus authoring (per-task dirs) | Repo / Filesystem | — | Hand-readable seed + git-tracked fixtures (D-04) |
| Mode dispatch (4-mode loop) | `cmd/helix-eval/` runner | `internal/eval/` lib | CLI is thin; library is testable |
| Daemon subprocess lifecycle | `internal/eval/runner` | `internal/forwarder` | Forwarder already starts daemons; reuse not reinvent |
| Isolation (HOME, socket, config) | `internal/eval/sandbox` | `os/exec` env | Per-mode tmpdir + env scoping |
| Agent process | `claude` CLI subprocess | `--mcp-config` | Locked decision D-01 |
| Tool-call telemetry | Daemon `TelemetryMiddleware` | gRPC stream tap | Source of truth (D-02); CC json is supplementary |
| Final-patch + reasoning | CC `--output-format=json` | stdout parser | CC-side; daemon never sees the LLM messages |
| Trace merge + reporting | `internal/eval/report` | — | One canonical `trace.json` per task |
| Scoring (heuristic) | `internal/eval/score` | YAML rule loader | CI-actionable per D-06 |
| Scoring (LLM judge) | `internal/eval/judge` | Anthropic SDK | Informational only per EVAL-07 |
| In-process `eval-quick` | `internal/daemon` direct call | profile overlay | Skip subprocess, same modes (EVAL-03) |
| Pipeline DAG | `internal/phasegraph/pipelines/eval.go` | — | Already declared; fill `noopRun`s |
| Reports + receipts surface | `eval/reports/` | filesystem | Hand-greppable, git-friendly |

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 — Agent runner: Claude Code CLI subprocess.** No in-house Go agent. Runner orchestrates `claude` subprocesses; future runners go behind `--agent=` later.
- **D-02 — Trace = daemon TelemetryMiddleware (source of truth) + CC `--output-format=json` (assistant messages / reasoning / final patch), merged.** Each per-task dir contains `trace_daemon.jsonl`, `trace_cc.json`, and merged `trace.json`.
- **D-03 — Record `claude --version` per run; do not pin.** Cross-run comparisons filter by version.
- **D-04 — Per-task directory format**: `eval/corpus/<task-id>/{task.md, repo/, verify.sh, expected_tools.yaml, budget.yaml}`. Generators (`eval/gen/*.go`) emit into the same shape.
- **D-05 — Corpus size**: `make eval-quick` ≈ 10 tasks (<30s wall, in-process, every-PR CI); `make eval` 50–100 tasks (single-digit minutes per mode, ~30 min total, nightly / pre-release).
- **D-06 — Heuristic scorer is primary + CI-actionable**; LLM judge is informational only and never gates merges.
- **D-07 — Tokens-only this phase.** No `$` price table. Tokens come from CC json `usage.input_tokens` / `usage.output_tokens`.
- **D-08 — Hard 4-axis budget caps**: `max_input_tokens` (def ~200k), `max_output_tokens` (def ~32k), `max_seconds` (def 300), `max_tool_calls` (def 100). Breach → `failed-with-cause: budget_<axis>`.

### Claude's Discretion (open questions to resolve in research/plan)

- Languages in seed corpus.
- Subprocess isolation mechanics (`HOME`, config dir, ports, sockets).
- `baseline` profile concrete YAML.
- In-process `eval-quick` plumbing.
- CC `--mcp-config` schema confirmation.
- Trace merge schema (concrete `trace.json` shape + ordering policy).
- Heuristic rule DSL.
- LLM judge model + prompt + rubric → EVAL-05 family map.
- TOS attestation block format in `EVAL.md`.
- PhaseGraph integration site.
- `eval/` (top-level) vs `internal/eval/` (runner internals) placement.

### Deferred Ideas (OUT OF SCOPE)

- Pluggable agent runners other than `claude` CLI (no Anthropic SDK direct, OpenAI Agents SDK, Codex, Gemini CLI). Future `--agent=` flag.
- Static price table / `$` conversion.
- LLM judge as gating signal (forbidden by EVAL-07).
- SWE-bench / public benchmark integration.
- Provider billing API reconciliation.
- Cross-version CC comparison infra beyond `--version` recording.
- Generator-driven full corpus (Phase 67 ships seed + scaffolding, not full corpus).

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EVAL-01 | `EvalResult` per task captures success / patch-applies / tests-pass / diagnostics-clean / duration / tokens / edit count / guardrail compliance / context precision-recall | Trace merge schema (§"Trace Merge Schema"), per-task dir layout, `verify.sh` exit-0 contract |
| EVAL-02 | Each mode runs in isolated daemon subprocess; `baseline` strips Helix tools; agent connects via stdio forwarder | Subprocess isolation design (§"Subprocess Isolation"), `baseline` profile (§"Baseline Profile"), forwarder reuse |
| EVAL-03 | `make eval-quick` in-process small fixture suite | In-process plumbing (§"In-Process eval-quick"); reuse `daemon.New(cfg, logger).Run` directly |
| EVAL-04 | Reports: `eval_report.{json,md}`, `cost_summary.json`, `tool_behavior.json`, `safety_compliance.json`, per-task traces + patches | Reports layout (§"File-System Layout") + reporter writes to `eval/reports/<run-id>/` |
| EVAL-05 | Tool-behavior heuristic scoring for rename/delete/public-API tasks; +1/-1 deltas | Heuristic rule DSL (§"Heuristic Rule DSL"); rule examples in CONTEXT.md D-06 |
| EVAL-06 | Synthetic-only corpus default (Helix OSS permissible secondary); retention-zero on commercial providers; TOS attestation in `EVAL.md` | TOS attestation format (§"TOS Attestation"); ZDR research (§"Provider Retention") |
| EVAL-07 | Cross-model judging informational; flaky judge never blocks merge | Judge separation (§"LLM Judge Design"): writes to `tool_behavior_judge.json`, never to gate signal |

## Standard Stack

### Core (already in repo — reuse)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `internal/phasegraph` | in-tree | Pipeline DAG (eval pipeline = `pipelines/eval.go`) | DAG-02 already declares the eval DAG; just fill `Run` bodies. [VERIFIED: read of `pipelines/eval.go`] |
| `internal/forwarder` | in-tree | stdio→gRPC + auto-daemon-start | EVAL-02 mandates "agent connects via stdio forwarder"; `ConnectOrStartDaemon` already does the lifecycle. [VERIFIED: `forwarder/dial.go:21`] |
| `internal/profile` | in-tree | YAML profile loader; `LoadEmbedded` + `LoadOverrides` | New `baseline.yaml` lands in `internal/profile/profiles/` and ships embedded via `embed.FS`. [VERIFIED: `loader.go:15`] |
| `internal/config` | in-tree | 4-layer koanf config; `Load(global, project, cliOverrides)` | Eval injects `--config=` and `--socket=` per mode without writing new config code. [VERIFIED: `config/loader.go:23`] |
| `internal/mcp.TelemetryMiddleware` | in-tree | Tool-call outcome classification incl. `guardrail_*` outcomes | Daemon-side trace source of truth (D-02). [VERIFIED: `mcp/middleware.go:170`] |
| `gopkg.in/yaml.v3` | in-tree | `expected_tools.yaml`, `budget.yaml` parsing | Already a transitive dep via `internal/profile`. [VERIFIED: `loader.go:10`] |

### Supporting (Go stdlib only — no new deps)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os/exec` | stdlib | Spawn `claude` + spawn daemon-under-test (already used by forwarder `dial.go:89`) | Mode runner |
| `encoding/json` | stdlib | Parse CC json output + emit reports | Reporter |
| `os` (`MkdirTemp`, env scoping) | stdlib | Per-mode HOME and config tmpdirs | Sandbox |
| `context` (timeouts) | stdlib | D-08 `max_seconds` enforcement | Budget watchdog |
| `bufio.Scanner` over CC stdout | stdlib | Stream `--output-format=stream-json` | If we choose stream-json over json |

### Anthropic SDK for LLM Judge (informational only)

| Library | Version | Purpose | Notes |
|---------|---------|---------|-------|
| `github.com/anthropics/anthropic-sdk-go` | latest stable | LLM judge calls (Sonnet / Opus) | Used ONLY by `internal/eval/judge`; never CI-gated. Verify exact module path before adding to `go.mod`. [ASSUMED: SDK exists; verify with `go doc github.com/anthropics/anthropic-sdk-go` before locking. Alternative: hand-rolled HTTP to `https://api.anthropic.com/v1/messages` is fine and avoids dep churn.] |

**Version verification step for the planner:** before pinning the Anthropic SDK,
run `go list -m -versions github.com/anthropics/anthropic-sdk-go` and confirm
the latest tagged release. If the module path is unstable, prefer a 60-LOC
HTTP client.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `claude --output-format=json` | `--output-format=stream-json --include-partial-messages` | Stream-json gives per-event reasoning + tool-use-as-they-happen, easier merge with daemon timeline. JSON gives one final blob. **Recommendation: use `stream-json --verbose`** — `system/init`, `tool_use`, `tool_result`, `assistant`, and `result` are first-class events; merge is wall-clock alignment, not regex. [CITED: code.claude.com/docs/en/headless §Stream responses] |
| Per-mode daemon `--socket=` | Per-mode `HELIX_HOME=` only | Both. Each mode needs a unique socket so a stale `helix-<uid>/daemon.sock` from another mode doesn't get reused (`config.DefaultSocketPath` is `$TMPDIR/helix-<uid>/daemon.sock` — UID-keyed, not mode-keyed). [VERIFIED: `config/loader.go:73`] |
| `eval/` top-level for everything | `internal/eval/` for runner, top-level `eval/` for corpus + reports | Split. Corpus + reports + `EVAL.md` are user-visible and git-tracked → top-level. Runner is implementation-private → `internal/eval/`. |

**Installation (none):**
```bash
# No new deps. Anthropic SDK is the only candidate; defer to plan.
```

## File-System Layout Recommendation

```
eval/                                 # TOP-LEVEL — user-visible, git-tracked
├── EVAL.md                           # TOS attestation, run protocol, retention review
├── corpus/                           # hand-authored + generator-emitted tasks
│   ├── go-rename-public-001/         # one per task (D-04)
│   │   ├── task.md                   # prompt + intent (the "user message" to the agent)
│   │   ├── repo/                     # seeded fixture; agent works against this
│   │   ├── verify.sh                 # exit 0 = pass; runs go test / grep / diff
│   │   ├── expected_tools.yaml       # heuristic patterns (D-06)
│   │   └── budget.yaml               # optional per-task overrides for D-08 caps
│   ├── ts-delete-symbol-001/
│   └── ...
├── gen/                              # programmatic generators (Go)
│   ├── rename.go
│   ├── delete.go
│   └── public_api.go
├── fixtures/                         # shared seed repos for eval-quick
└── reports/                          # generated; .gitignored except a sentinel
    └── <run-id>/                     # ISO-8601 timestamp + git SHA
        ├── eval_report.json          # EVAL-04 machine-readable
        ├── eval_report.md            # EVAL-04 human-readable
        ├── cost_summary.json
        ├── tool_behavior.json        # heuristic
        ├── tool_behavior_judge.json  # informational (EVAL-07)
        ├── safety_compliance.json    # guardrail counts (Phase 66 telemetry)
        ├── run_metadata.json         # claude --version, helix version, modes, env
        └── tasks/<task-id>/<mode>/
            ├── task.md (copy)
            ├── trace_daemon.jsonl    # TelemetryMiddleware events
            ├── trace_cc.json         # raw CC --output-format=json output
            ├── trace.json            # MERGED (single source for scoring)
            ├── patch.diff            # final agent patch (post-run)
            ├── verify.log            # verify.sh stdout/stderr + exit code
            └── result.json           # EvalResult (one per (task, mode) pair)

internal/eval/                        # RUNNER INTERNALS — not user-facing
├── runner/                           # mode dispatch loop
│   ├── runner.go
│   └── runner_test.go
├── sandbox/                          # HOME / socket / configdir isolation
│   ├── sandbox.go
│   └── sandbox_test.go
├── agent/                            # claude CLI subprocess wrapper
│   ├── claude.go                     # builds argv, spawns, captures
│   └── mcpconfig.go                  # writes per-mode --mcp-config JSON
├── trace/                            # daemon-side telemetry tap + merge
│   ├── tap.go                        # subscribes to TelemetryMiddleware events
│   ├── merge.go                      # merges trace_daemon + trace_cc → trace.json
│   └── schema.go                     # canonical event types
├── score/                            # heuristic scorer
│   ├── rules.go                      # DSL parser + matcher
│   └── score.go
├── judge/                            # informational LLM judge
│   ├── judge.go
│   └── prompts/
│       ├── tool_behavior.tmpl
│       └── rubric.md
├── report/                           # report writers
│   ├── eval_report.go
│   ├── cost_summary.go
│   └── safety_compliance.go
├── budget/                           # D-08 enforcement
│   └── budget.go
└── pipeline.go                       # wires phasegraph/pipelines/eval.go Run bodies

cmd/helix-eval/                       # ENTRYPOINT
└── main.go                           # cobra: corpus root, mode list, run-id, --quick
```

**Why this split:**
- `eval/` top-level is grep-friendly for humans (corpus authoring, TOS review, report inspection). Per CONTEXT.md, the `EVAL.md` attestation lives at top-level for visibility.
- `internal/eval/` follows the Helix convention that runner code is private — same as `internal/daemon/`, `internal/forwarder/`. Vet boundaries (no kernel→semantic, etc.) are unaffected.
- `cmd/helix-eval/` is a separate binary, not a subcommand of `helix`, because its dependency surface (Anthropic SDK if used, eval reporters) should not bloat the main daemon binary. `make eval` and `make eval-quick` both invoke it.

[CITED: CLAUDE.md "Build pipeline" + existing `cmd/helix/main.go` single-binary discipline]

## Subprocess Isolation Design

**Goal:** A pathological agent in mode A cannot poison mode B's outcome.
Each mode gets a clean filesystem-, socket-, and config-namespace.

### Isolation Mechanisms

| Surface | Mechanism | Notes |
|---------|-----------|-------|
| Helix global config (`~/.helix/helix_config.yml`) | Set `HELIX_HOME`-equivalent → `HOME=$tmp/<mode>` | `config.Load` resolves global path via `os.UserHomeDir()` (`config/loader.go:33`); overriding `HOME` (or `USERPROFILE` on Windows) redirects this read to a clean dir. **Zero new code in `internal/config`.** [VERIFIED: `loader.go:32-35`] |
| Project config (`.helix/project.yml`) | Eval task fixtures do NOT contain `.helix/` | Corpus repos start clean; if a task explicitly needs project config, it ships one |
| Profile name | `--profile=baseline` / `--profile=full` / `--profile=full` (with semantic on) / `--profile=full` (with semantic + guardrails) | The 4 modes differ in profile + config layer, NOT in 4 different profiles. Only `baseline` is a new profile. |
| Daemon socket | `--socket=$tmp/<mode>/daemon.sock` | Default is UID-keyed not mode-keyed (`$TMPDIR/helix-<uid>/daemon.sock`). Without override, two modes' daemons would collide on the same socket. [VERIFIED: `config/loader.go:73`] |
| Semantic store DuckDB | Per-task `repo/` is a fresh tmpdir → `<repo>/.helix/semantic.duckdb` is per-task | No cross-task contamination as long as the runner clones each task's `repo/` into a per-(task,mode) tmpdir |
| HTTP / admin port | `--admin-addr=""` (disabled in eval) and `--http-addr=` set per-mode if HTTP transport used (we use stdio per EVAL-02, so http-addr is irrelevant) | Default is loopback `:8080`; collision-free if disabled |
| Working directory | `cmd.Dir = $tmp/<mode>/<task>/repo` | Standard `os/exec` |
| Environment | `cmd.Env = []string{"PATH=...", "HOME=...", "HELIX_LOG_LEVEL=...", "ANTHROPIC_API_KEY=..."}` (clean — do NOT inherit) | Avoid leaking dev `~/.helix/` overrides into eval runs |
| OTEL exporter | `OTEL_EXPORTER_OTLP_ENDPOINT=""` unset in eval env | Forwarder degrades to noop tracer (`forwarder.go:32`); no leakage |

### Per-Task Per-Mode Layout

```
$TMPDIR/helix-eval-<run-id>/
└── <task-id>/
    └── <mode>/
        ├── home/                         # HOME=here
        │   └── .helix/
        │       └── helix_config.yml      # generated per-mode (semantic on/off, guardrails)
        ├── repo/                         # cloned from corpus/<task-id>/repo/
        ├── daemon.sock                   # --socket=here
        ├── daemon.log
        ├── claude.stdout                 # --output-format=stream-json captured here
        ├── claude.stderr
        └── mcp-config.json               # --mcp-config=here (points at our forwarder)
```

### Mode Differences (concrete)

| Mode | `--profile` | `helix_config.yml` toggles |
|------|-------------|----------------------------|
| `baseline` | `baseline` | `semantic_index.enabled: false`; guardrails off |
| `native` | `full` | `semantic_index.enabled: false`; guardrails off |
| `semantic` | `full` | `semantic_index.enabled: true`; guardrails off |
| `semantic_guarded` | `full` | `semantic_index.enabled: true`; `guardrails.enforcement: warn` |

**Note:** `baseline` and `native` differ only in **which Helix tools are exposed** (none vs. all). `native` is "agent has all of Helix's hand-rolled tools but no semantic graph behind them" — it represents what v1.9 shipped before v1.10. `baseline` represents what the agent does with no Helix at all (just CC's built-in `Read/Edit/Bash/Grep`).

### `--strict-mcp-config` Discipline

`claude --strict-mcp-config --mcp-config <path>` ignores all other MCP configs (user `~/.claude.json`, project `.mcp.json`, env). This is mandatory for eval reproducibility — without it a developer's local MCP servers leak into the run. [CITED: code.claude.com/docs/en/cli-reference §--strict-mcp-config]

Combine with `--bare` to skip auto-discovery of hooks, skills, plugins, auto memory, and CLAUDE.md. **Bare mode is the documented "recommended mode for scripted and SDK calls"** and "will become the default for `-p` in a future release." Without bare, the run picks up whatever the developer's `~/.claude/` contains. [CITED: code.claude.com/docs/en/headless §Bare mode]

## Baseline Profile Design

Concrete YAML for `internal/profile/profiles/baseline.yaml`:

```yaml
name: baseline
description: >
  Eval-harness baseline: Helix exposes the bare MCP server with NO Helix
  tools. The agent must accomplish tasks using only its host's built-in tools
  (CC's Read / Edit / Bash / Grep). Used as the control case in Phase 67
  evaluation to measure how much Helix moves the needle.

prompt: |
  You are running in EVAL BASELINE mode. No Helix tools are available. Use
  whatever tools your host environment provides (file read/write, shell, grep).

skills: []
tools: []

# We can't list every tool by name in exclude_tools, and the existing five
# profiles all use a positive `skills` list. Empty skills list with empty
# tools list = ProfileFilterMiddleware filters everything out.
exclude_tools: []

tool_description_overrides: {}

default_mode: edit
single_project: false

guardrails:
  enforcement: off

allowed_mode_transitions:
  read:    [edit, review]
  edit:    [read, review]
  review:  [edit, read]
  admin:   [read, edit, review]
```

**Verification step required in plan:** confirm by reading
`internal/profile/loader.go` and the ProfileFilterMiddleware logic
(`internal/mcp/middleware.go`) that an empty `skills: []` plus empty
`tools: []` causes `tools/list` to return zero Helix tools (vs. some
default-include-everything fallback). [ASSUMED: empty list = empty
exposed; the planner's first task should write a unit test asserting
this before the harness depends on it.]

If empty-list-means-empty is NOT the existing behavior, the alternative
is a `disable_all_tools: true` flag on the profile schema. The discuss-
phase did not lock the mechanism, only the outcome ("strips Helix tools
entirely"); the planner has discretion to extend the profile schema if
necessary.

## CC `--mcp-config` Schema

Confirmed schema (CC v2.1.x, current as of 2026-05-10):

```json
{
  "mcpServers": {
    "helix": {
      "type": "stdio",
      "command": "/abs/path/to/helix",
      "args": [],
      "env": {
        "HOME": "/tmp/helix-eval-<run-id>/<task>/<mode>/home",
        "HELIX_SOCKET": "/tmp/helix-eval-<run-id>/<task>/<mode>/daemon.sock"
      }
    }
  }
}
```

[CITED: code.claude.com/docs/en/mcp §"Add a local stdio server" + CC docs example showing
`{"type":"stdio","command":"...","args":[...],"env":{...}}`]

**Caveats:**
- `type: "stdio"` is now explicit per the v2.1 schema; older docs sometimes omit it. Include it for forward-compat.
- `command` should be the absolute path to the helix binary (not `helix` on PATH) so eval doesn't depend on the developer's PATH.
- `args` can be empty: `helix` with no args defaults to stdio forwarder mode (`internal/cli/root.go:99-103`). To force a specific socket without altering env, pass `args: ["--socket", "<path>"]`.
- `env` is merged with the inherited environment; we explicitly clean it via the eval runner's `cmd.Env`.

**Version-dependence:** the `mcpServers` top-level key has been stable since CC's MCP launch. The `type: "stdio"` key is post-v2.0; if we discover a regression on a specific CC version, the runner records `claude --version` (D-03) and the report can filter.

## Trace Merge Schema

### Two Streams

**Stream A — Daemon TelemetryMiddleware (`trace_daemon.jsonl`)**

The middleware classifies tool-call outcomes with the closed enum from
`internal/mcp/middleware.go:160-181`:

```
success | invalid_args | not_found | circuit_open | ls_crash |
timeout | internal | guardrail_warned | guardrail_blocked
```

Each event already carries (existing — no new fields needed): tool name,
session ID, RED metrics tags, span context (OTel), timing, outcome. We tap
this stream by adding a thin file-sink middleware-extension OR by reading
the daemon's structured logs (`HELIX_LOG_FORMAT=json`) — the latter is
zero-coupling.

**Recommendation:** capture the daemon's stderr (already JSON-formatted via
`--json` log flag) and grep `tools/call` log lines into `trace_daemon.jsonl`.
No new MCP middleware needed. Cleanup: when v1.11 introduces a structured
trace export, swap to it.

**Stream B — Claude Code (`trace_cc.json` or `trace_cc.jsonl` for stream-json)**

With `--output-format=stream-json --verbose --include-partial-messages` we
get newline-delimited events:

| Event type | Subtype | Carries |
|------------|---------|---------|
| `system` | `init` | session_id, model, tools, mcp_servers |
| `system` | `api_retry` | retry diagnostics |
| `assistant` | (n/a) | message blocks (text + `tool_use`) |
| `user` | (n/a) | tool_result blocks |
| `stream_event` | `text_delta` | streaming text (skip in trace) |
| `result` | (n/a) | final result, **`usage.input_tokens`**, **`usage.output_tokens`**, `total_cost_usd`, session_id, num_turns |

[CITED: code.claude.com/docs/en/headless §Stream responses + §Get structured output]

### Merged `trace.json` Shape

```json
{
  "schema_version": "1",
  "task_id": "go-rename-public-001",
  "mode": "semantic",
  "run_id": "2026-05-10T08-30-00Z-abc1234",
  "claude_version": "2.1.138 (Claude Code)",
  "helix_version": "1.10.0-dev+abc1234",
  "started_at": "2026-05-10T08:30:00.123Z",
  "ended_at":   "2026-05-10T08:32:14.567Z",
  "duration_ms": 134444,
  "outcome": "success | failed | failed-with-cause:budget_seconds | failed-with-cause:budget_tool_calls | budget_input_tokens | budget_output_tokens",
  "events": [
    {
      "t": "2026-05-10T08:30:00.456Z",
      "source": "cc",
      "kind": "session_init",
      "session_id": "...",
      "model": "claude-sonnet-4-6",
      "mcp_servers": ["helix"]
    },
    {
      "t": "2026-05-10T08:30:01.789Z",
      "source": "cc",
      "kind": "assistant_message",
      "text_summary": "I'll start by finding the symbol's references...",
      "tool_uses": [{"id":"toolu_x","name":"mcp__helix__find_references","input":{...}}]
    },
    {
      "t": "2026-05-10T08:30:01.812Z",
      "source": "daemon",
      "kind": "tool_call",
      "tool": "find_references",
      "outcome": "success",
      "duration_ms": 23,
      "trace_id": "...",
      "args_summary": "...",
      "result_size_bytes": 1234,
      "guardrail": null
    },
    {
      "t": "2026-05-10T08:30:01.815Z",
      "source": "cc",
      "kind": "tool_result",
      "tool_use_id": "toolu_x",
      "is_error": false
    },
    {
      "t": "2026-05-10T08:30:09.001Z",
      "source": "daemon",
      "kind": "tool_call",
      "tool": "rename_symbol",
      "outcome": "guardrail_warned",
      "guardrail": {"rule":"G-001","level":"warn"}
    }
    // ...
  ],
  "usage": {
    "input_tokens": 12345,
    "output_tokens": 4567,
    "cache_read_tokens": 0,
    "cache_creation_tokens": 0
  },
  "tool_call_summary": {
    "total": 17,
    "by_tool": {"find_references":3, "rename_symbol":1, "verify_edit":2, ...},
    "by_outcome": {"success":15, "guardrail_warned":1, "internal":1}
  },
  "guardrails": {
    "warned": 1,
    "blocked": 0,
    "receipts_issued": 5
  }
}
```

### Ordering Policy

CC events have wall-clock timestamps; daemon events have wall-clock
timestamps. Both processes share the same OS clock (we run them on the
same host). **Sort merged events by `t` ascending; on tie, daemon-side
event wins** (daemon-side tool_call necessarily precedes CC-side tool_result
because the result comes back through the same gRPC stream). This means
ordering ambiguity is bounded to the < 1ms window between tool_use emit
(CC) and tool_call dispatch (daemon), which is inconsequential for scoring.

**No correlation IDs needed for v1.** A future improvement is to thread
the CC `tool_use_id` into the MCP envelope as a metadata header, then
match daemon events to CC events deterministically. Out of scope this phase.

[ASSUMED: daemon and CC clocks within 1ms of each other; verified by
spot-check during plan.]

## Heuristic Rule DSL (`expected_tools.yaml`)

Per-task heuristic patterns. The scorer reads merged `trace.json` and applies rules.

```yaml
# eval/corpus/go-rename-public-001/expected_tools.yaml
task_kind: rename                       # informational; one of {rename, delete, public_api, large_edit, security}

# +1 patterns: presence of these sequences scores positively
expect_sequence:
  - id: rename-after-references
    score: +1
    pattern:
      - tool: find_references
        args_match:
          symbol: "AuthMiddleware"     # optional; omit to match any
      - tool: rename_symbol            # may have intervening events
        args_match:
          old_name: "AuthMiddleware"

# Set patterns: presence of all listed tools (any order) scores
expect_set:
  - id: verified-after-edit
    score: +1
    tools: [rename_symbol, verify_edit]

# -1 patterns: presence of these scores negatively
forbid_sequence:
  - id: rename-by-grep
    score: -1
    pattern:
      - tool: search_for_pattern       # or `grep`/`Grep` at host level
      - tool: replace_in_file          # OR fuzzy_edit / replace_content
        args_match:
          # `find` arg is a likely identifier
          find_regex: "^[A-Za-z_][A-Za-z0-9_]{2,}$"

forbid_set:
  - id: delete-without-references
    score: -1
    when:
      tool_used: delete_file           # or safe_delete_symbol
    require_prior:
      any_of: [find_references, analyze_blast_radius]

# Receipt-aware (Phase 66) heuristics
receipts:
  - id: safe-delete-with-receipts
    score: +1
    when:
      tool_used: safe_delete_symbol
    require:
      receipts_non_empty: true
```

### Matcher Semantics

| Construct | Semantics |
|-----------|-----------|
| `expect_sequence[].pattern` | Ordered subsequence in event timeline; intervening events allowed; first match wins |
| `expect_set[].tools` | Each tool name must appear ≥ 1× (any order, any args) |
| `forbid_sequence` | Same as expect, but score is negative on match |
| `forbid_set.when.tool_used` | Triggers the rule only if this tool fires |
| `forbid_set.require_prior.any_of` | One of these tools must precede the trigger; if NONE precede, score the negative |
| `args_match` | Substring match against stringified arg; `_regex` suffix switches to regex |
| `receipts_non_empty` | Inspect the destructive tool call's `receipts: []ReceiptID` arg |

The DSL is intentionally small. v1 scope = sequence + set + receipt
presence. Argument matching uses substring/regex on a flattened JSON
arg string — no JSONPath in v1.

### Rule Schema Validation

Ship a `score.LoadRules(path)` that fails fast on unknown keys
(yaml.v3 `KnownFields(true)`). CI runs `helix-eval validate-rules`
on the corpus to catch typos.

### Starter Rule Coverage (EVAL-05)

The seed corpus must include rules for at least:
- **rename** family: `+1 rename_symbol after find_references`,
  `-1 grep + replace_in_file on identifier`
- **delete** family: `+1 safe_delete_symbol with receipts`,
  `-1 delete_file on symbol-bearing file without prior find_references`
- **public-API** family: `+1 analyze_blast_radius before public-symbol edit`,
  `-1 replace_symbol_body on exported symbol without blast-radius`

Each rule directly traces to a CONTEXT.md D-06 example.

## LLM Judge Design

### Model Selection

**Default:** `claude-sonnet-4-6` (per CONTEXT.md open question; planner can
override). Sonnet is the cost-effective tier for evaluation; reserve Opus for
the rubric writing phase, not per-task scoring.

**Model is hard-coded into `eval/judge/prompts/rubric.md`** so the same model
runs on every task — switching models invalidates the comparison.

**Cost ceiling:** at ~5k input + ~1k output tokens per judge call × 100 tasks ×
4 modes = 2.4M tokens per full eval run. At Sonnet's published rate this is
single-digit dollars per run; well within nightly CI budget.

### Prompt Structure

```
internal/eval/judge/prompts/
├── rubric.md           # static rubric per EVAL-05 task families
├── tool_behavior.tmpl  # Go text/template; renders per-task
└── few_shot.md         # 2-3 worked examples (optional)
```

Template:

```
You are an expert reviewer of an AI coding agent's tool-use trace.

Task family: {{ .TaskKind }}
Task description: {{ .TaskDescription }}

Trace (chronological tool calls + outcomes):
{{ range .Events }}- {{ .T }} {{ .Tool }}({{ .ArgsSummary }}) → {{ .Outcome }}
{{ end }}

Final outcome: {{ .Outcome }}
Tests pass: {{ .TestsPass }}

Rubric (from rubric.md):
{{ .Rubric }}

Score 1–5 on each axis:
- right_tool_for_job (was the canonical tool used?)
- evidence_before_action (was a read receipt gathered before destructive action?)
- minimal_blast_radius (did the agent confine changes appropriately?)
- recovery (did the agent recover from errors gracefully?)

Return strict JSON: {"scores":{...},"reasoning":"...","flags":[...]}.
```

### Rubric Per EVAL-05 Family

| Family | Right-Tool | Evidence | Blast Radius | Recovery |
|--------|-----------|----------|--------------|----------|
| rename | `rename_symbol` (LSP-native) | `find_references` issued receipt | scoped to symbol | retried on conflict |
| delete | `safe_delete_symbol` | `analyze_blast_radius` for public, `find_references` for private | confined to symbol | rolled back if tests broke |
| public_api | LSP-rename + per-call-site update | `analyze_blast_radius` mandatory | tracked external-package fanout | reverted on test red |
| large_edit | `replace_symbol_body` over `replace_in_file` | `get_context` / `get_repo_map` first | bounded by symbol | re-read on diff drift |
| security | symbol edits + `verify_edit` post-edit | `get_diagnostics` post-edit | imports change reviewed | rolled back on diagnostic regression |

### Output Contract

Judge writes one entry per (task, mode) into `tool_behavior_judge.json`:

```json
{
  "schema_version": "1",
  "judge_model": "claude-sonnet-4-6",
  "judged_at": "2026-05-10T...",
  "tasks": [
    {
      "task_id": "go-rename-public-001",
      "mode": "semantic_guarded",
      "scores": {"right_tool":5,"evidence":5,"blast_radius":4,"recovery":3},
      "reasoning": "...",
      "flags": ["minor: edited test file unnecessarily"]
    }
  ]
}
```

### Hard CI Boundary

`tool_behavior_judge.json` is **never** read by `make eval`'s exit-code
logic. It is appended to `eval_report.md` in a clearly-labeled
"Informational: LLM judge" section. EVAL-07 verbatim: "Cross-model
judging is informational, never a CI gate; a flaky judge does not block
merges." [CITED: REQUIREMENTS.md EVAL-07]

The runner's `--no-judge` flag (default false; flips to true if
`ANTHROPIC_API_KEY` unset or judge call errors) lets eval still finish
without the judge.

### Judge Bias Mitigations (M-class pitfall m4)

- **Different judge model than agent model:** if the agent runs on
  Sonnet, judge can also be Sonnet (same family is fine — bias is
  about training data, not exact model). Document the choice.
- **Trace-only, no patch:** judge sees tool calls + outcomes, NOT the
  raw patch diff. Prevents "the patch looks plausible" hallucination.
- **Family-fixed rubric:** judge sees the rubric for the task's family,
  not free-form. Reduces criteria drift.
- **Cross-mode aggregate, not per-mode comparison:** report mean ±
  stddev across all 4 modes; do not let judge "pick a winner."

## TOS Attestation Format (for `EVAL.md`)

Per EVAL-06: "Provider data-retention TOS is verified at planning time
and recorded in EVAL.md." Concrete block:

```markdown
## Provider Retention Attestation

**Verified at:** 2026-05-10 (Phase 67 planning)
**Re-verification cadence:** Every minor Helix release (quarterly minimum)

### Anthropic API (used by `claude` CLI in eval modes)

| Attribute | Value | Source |
|-----------|-------|--------|
| Default API log retention | 7 days (since 2025-09-14) | https://privacy.claude.com/en/articles/8956058 |
| Zero Data Retention (ZDR) availability | Enterprise opt-in via DPA | https://platform.claude.com/docs/en/build-with-claude/api-and-data-retention |
| ZDR enforcement mechanism | Account-level (no per-request header); enforced by Anthropic when DPA signed | https://platform.claude.com/docs/en/build-with-claude/api-and-data-retention |
| Claude Code ZDR | Available via Claude for Enterprise | https://code.claude.com/docs/en/zero-data-retention |
| Retrieved on | 2026-05-10 |

**Eval-run policy:**
- Default mode (no enterprise account configured): runs against synthetic
  corpus only; OSS Helix repo permitted as secondary source.
- Enterprise-ZDR mode (env var `HELIX_EVAL_ZDR_VERIFIED=1`): may run
  against private corpora. Operator attests they hold a signed Anthropic
  DPA covering the API key in use. Helix performs no header-based
  verification (none exists per Anthropic docs).
- Eval runner refuses to start against non-synthetic, non-OSS-Helix
  corpora unless `HELIX_EVAL_ZDR_VERIFIED=1` is set.

### Future Providers (placeholder)

| Provider | Status | TOS link | Retrieved |
|----------|--------|----------|-----------|
| OpenAI (Codex) | Not in scope (deferred) | — | — |
| Google (Gemini CLI) | Not in scope (deferred) | — | — |
```

[CITED: privacy.claude.com/en/articles/8956058 — 7-day default + 30-day opt-in via DPA]
[CITED: code.claude.com/docs/en/zero-data-retention — Claude Code ZDR via Enterprise]
[CITED: platform.claude.com/docs/en/build-with-claude/api-and-data-retention — ZDR account-level]

**Important caveat (M5 mitigation):** ZDR is account-level, NOT
per-request. There is no `X-ZDR-Required: true` header. Operators must
verify their account has ZDR before running eval against any non-public
corpus. The `HELIX_EVAL_ZDR_VERIFIED=1` env var is a HUMAN attestation
gate; it does not contact Anthropic to verify status.

## PhaseGraph Integration

`internal/phasegraph/pipelines/eval.go` already declares the 10-phase eval
DAG with `noopRun` placeholders. Phase 67 fills in the bodies:

| PhaseID | Implementation |
|---------|----------------|
| `prepare_workspace` | `internal/eval/sandbox`: clone `corpus/<task>/repo/` into per-(task,mode) tmpdir, set up HOME |
| `configure_mode` | Write `helix_config.yml` and `mcp-config.json` per mode |
| `run_agent` | `internal/eval/agent`: spawn daemon (or in-process for `eval-quick`), spawn `claude` subprocess, wait |
| `collect_trace` | `internal/eval/trace`: read daemon JSONL stderr + claude stdout; emit `trace_daemon.jsonl`, `trace_cc.json` |
| `apply_patch_check` | Run `git diff` on the workspace, write `patch.diff`; check it applies cleanly |
| `run_tests` | Execute `verify.sh`; capture exit, stdout, stderr → `verify.log` |
| `run_diagnostics` | (optional) request `get_diagnostics` from the daemon post-run |
| `score_tool_behavior` | `internal/eval/score`: load `expected_tools.yaml`, apply rules, write `tool_behavior.json` entries |
| `score_guardrails` | Pull guardrail counts from `trace_daemon.jsonl`, write `safety_compliance.json` entries |
| `aggregate_report` | `internal/eval/report`: write per-task `result.json`, then aggregate `eval_report.{json,md}`, `cost_summary.json` |

**Note:** the eval pipeline is the per-task DAG. The (task, mode) cross
product is driven by the runner outside the DAG: `for each task: for
each mode: pipelinegraph.RunPhaseGraph(...)`.

LLM judge runs OUTSIDE the DAG (post-aggregate), as a separate
optional pass writing `tool_behavior_judge.json`. Keeping judge out of
the DAG enforces EVAL-07: a judge failure cannot poison phase outcomes.

[VERIFIED: `internal/phasegraph/pipelines/eval.go` 10-phase declaration]

## In-Process `eval-quick`

EVAL-03 mandates a fast in-process variant. Plumbing:

1. **Daemon as a library:** `daemon.New(cfg, logger).Run(ctx)` is already
   the entrypoint. For `eval-quick`, the runner constructs a daemon
   in-process and connects to it via in-memory gRPC (or directly via the
   exported tool registry — verify cleanest seam during plan).
2. **Skip the agent process:** `eval-quick` uses a synthetic "scripted
   agent" that issues a hard-coded sequence of MCP tool calls and asserts
   responses. This skips both the `claude` subprocess and any LLM API
   call. Wall-clock target <30s for 10 tasks.
3. **Same 10-phase DAG** but `run_agent` body is the scripted-agent runner
   instead of the CC subprocess wrapper.
4. **Same scorer + reporters** — the trace shape is identical (the events
   come from the in-process daemon's TelemetryMiddleware, which is the
   same code path).

`eval-quick` proves the harness wiring without spending tokens. It does
NOT prove agent behavior — that's `make eval`'s job.

[VERIFIED: `internal/daemon/daemon.go` `New()` + `Run()` signatures suitable for in-process use]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| MCP-server JSON config writing | Hand-write JSON encoder for `--mcp-config` | `encoding/json` + a `McpConfig` struct | Trivial; SDK-level concern is parsing CC json output, not generating MCP config |
| Pipeline orchestration | Custom for-loop with goroutines | `internal/phasegraph` already shipped + tested | DAG-02 verbatim mandates phasegraph use |
| Daemon lifecycle | Custom subprocess supervisor | `internal/forwarder.ConnectOrStartDaemon` | Already auto-spawns daemon; just point at our socket |
| Trace event schema | Custom binary format / protobuf | JSONL of typed Go structs | Hand-greppable for debugging eval failures; perf is not a concern at 100 tasks |
| LLM judge HTTP client | DIY retry / backoff loop | Anthropic Go SDK *or* a 60-LOC `net/http` client with `context.WithTimeout` | Either is fine; both are cheap |
| Token counting | Self-tokenize the prompt | Read `result.usage.input_tokens` from CC json | CC reports it directly; tokenizing locally is wrong (different tokenizer than the API uses for billing) |
| Cost computation | Static price table | Tokens-only per D-07 | EXPLICITLY out of scope this phase |
| Profile filtering | Custom strip-Helix-tools middleware | New `baseline.yaml` + existing ProfileFilterMiddleware | Reuse, don't add a new middleware |
| Per-mode socket discovery | Auto-port-allocator | `--socket=<tmpdir>/daemon.sock` (Unix domain sockets, no port conflicts) | UDS doesn't have port collisions; trivial to make unique |

**Key insight:** Phase 67 is integration work, not infrastructure work.
The runner is glue between already-shipped subsystems. Resist new
abstractions.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testify` (already in repo) |
| Config file | `go.mod` (no per-package config) |
| Quick run command | `go test ./internal/eval/...` (unit + integration of the runner itself) |
| Full suite command | `go test ./...` (project-wide; runs in CI today) |

### Distinguishing Tests of the Runner FROM Eval Runs

This is critical and easy to confuse. Two separable things:

| What | Validates | Mechanism | When |
|------|-----------|-----------|------|
| **Tests of `internal/eval/*`** | Runner code is correct (sandbox isolates, scorer applies rules, trace merger sorts events, report writer emits valid JSON) | `go test` with table-driven fixtures | Every PR (`make test`) |
| **`make eval-quick`** | Helix MCP server + scripted agent + scorer + reporter wire together end-to-end | In-process DAG run on 10 fixture tasks | Every PR (CI gate) |
| **`make eval`** | Real Claude Code agent's behavior on 50–100 tasks, across 4 modes | Out-of-process subprocesses, real LLM calls | Nightly / pre-release (NOT every-PR — token cost) |
| **LLM judge pass** | Informational depth on tool-use quality | Anthropic API calls | Optional; runs after `make eval` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| EVAL-01 | `EvalResult` schema fields populated correctly | unit | `go test ./internal/eval/report -run TestEvalResult` | ❌ Wave 0 |
| EVAL-01 | `verify.sh` exit-code drives `tests_pass` | integration | `go test ./internal/eval/runner -run TestVerifyDrivesTestsPass` | ❌ Wave 0 |
| EVAL-02 | Mode subprocesses use isolated socket + HOME | integration | `go test ./internal/eval/sandbox -run TestModesIsolated` | ❌ Wave 0 |
| EVAL-02 | `baseline` profile exposes zero Helix tools | unit | `go test ./internal/profile -run TestBaselineEmptyTools` | ❌ Wave 0 (also asserts the empty-skills assumption from §"Baseline Profile") |
| EVAL-02 | Agent connects via stdio forwarder | integration | `go test ./internal/eval/agent -run TestForwarderTransport` | ❌ Wave 0 |
| EVAL-03 | `make eval-quick` runs <30s on 10 fixtures, no subprocess | integration | `make eval-quick && [ $? -eq 0 ]` | ❌ Wave 0 (Makefile target) |
| EVAL-04 | All 5 reports emitted with valid schema | integration | `go test ./internal/eval/report -run TestReportsEmitted` | ❌ Wave 0 |
| EVAL-05 | Heuristic rule DSL parses + scorer applies +1/-1 | unit | `go test ./internal/eval/score -run TestRuleDSL` | ❌ Wave 0 |
| EVAL-05 | Rename / delete / public-API rule examples produce expected scores | table-driven | `go test ./internal/eval/score -run TestStarterRules` | ❌ Wave 0 |
| EVAL-06 | Eval refuses to run on non-synthetic corpus without `HELIX_EVAL_ZDR_VERIFIED=1` | integration | `go test ./internal/eval/runner -run TestZDRGate` | ❌ Wave 0 |
| EVAL-06 | `EVAL.md` exists at repo root with TOS attestation block | check | `test -f EVAL.md && grep -q "Provider Retention" EVAL.md` | ❌ Wave 0 |
| EVAL-07 | Judge failure does not affect `make eval` exit code | integration | `go test ./internal/eval/judge -run TestJudgeIsolation` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/eval/...` (unit + small integration, <30s)
- **Per wave merge:** `go test ./...` (full repo)
- **Phase gate:** `make eval-quick` green; `make eval` ran nightly with report attached to release notes

### Wave 0 Gaps

- [ ] `internal/eval/runner/runner_test.go` — covers EVAL-01, EVAL-02 mode subprocess
- [ ] `internal/eval/sandbox/sandbox_test.go` — covers EVAL-02 isolation
- [ ] `internal/eval/agent/claude_test.go` — covers CC subprocess wrapper (uses `exec.LookPath` skip if `claude` not on PATH)
- [ ] `internal/eval/trace/merge_test.go` — covers EVAL-01 trace merge
- [ ] `internal/eval/score/rules_test.go` — covers EVAL-05 DSL parser + matcher
- [ ] `internal/eval/score/starter_rules_test.go` — covers EVAL-05 rename/delete/public-API
- [ ] `internal/eval/budget/budget_test.go` — covers D-08 caps
- [ ] `internal/eval/report/eval_report_test.go` — covers EVAL-04 report shape
- [ ] `internal/eval/judge/judge_test.go` — covers EVAL-07 isolation (mock HTTP)
- [ ] `internal/profile/profiles/baseline.yaml` — exists (EVAL-02)
- [ ] `internal/profile/baseline_test.go` — asserts baseline exposes zero tools
- [ ] `cmd/helix-eval/main_test.go` — covers Makefile entrypoint
- [ ] `Makefile`: `eval` and `eval-quick` targets
- [ ] `eval/EVAL.md` — TOS attestation
- [ ] `eval/corpus/` — at least 10 hand-authored seed tasks (Wave > 0; Wave 0 just creates the layout)

## Common Pitfalls

### Pitfall 1: Inheriting developer ENV / `~/.claude/` config into eval runs

**What goes wrong:** Eval results vary by who's running them. Developer A's
`~/.claude/agents/` adds a custom subagent that completes tasks differently
than CI's clean environment.
**Why it happens:** `claude -p` auto-discovers user/project/local config
unless `--bare` and `--strict-mcp-config` are set.
**How to avoid:** Always pass `--bare --strict-mcp-config` AND use
`exec.Command{Env: cleanEnv}` to scrub the inherited environment to a
documented allowlist (`PATH`, `HOME=<our-tmp>`, `ANTHROPIC_API_KEY`,
`HELIX_LOG_LEVEL`).
**Warning signs:** Eval scores reproducible on CI but flaky locally, or vice
versa.
[CITED: code.claude.com/docs/en/headless §Bare mode]

### Pitfall 2: Daemon-socket collision across modes

**What goes wrong:** Mode B's daemon connects to mode A's still-running
socket; trace pollution.
**Why it happens:** Default socket is `$TMPDIR/helix-<uid>/daemon.sock` —
UID-keyed, not mode-keyed. [VERIFIED: `config/loader.go:73`]
**How to avoid:** Pass `--socket=<per-mode-tmpdir>/daemon.sock` AND ensure
prior daemon is reaped before next mode starts (`exec.Cmd.Process.Kill` +
`Wait` with a deadline).
**Warning signs:** Daemon log lines from mode A appearing in mode B's
trace_daemon.jsonl.

### Pitfall 3: CC version drift between local dev and CI

**What goes wrong:** Local `claude` is v2.1.140; CI is v2.1.138; `--mcp-config`
schema or `--output-format=stream-json` event types differ silently.
**Why it happens:** D-03 explicitly says "record, don't pin." Acceptable but
needs explicit handling.
**How to avoid:** Reporter records `claude --version` in
`run_metadata.json`; report-comparison code MUST filter by version when
diffing runs.
**Warning signs:** Reports look fine but tool-call counts shift between
runs of the same code on the same corpus.

### Pitfall 4: Token counting drift (count locally vs. read from CC)

**What goes wrong:** Counting tokens locally with a tokenizer disagrees with
billing.
**Why it happens:** Anthropic's exact tokenizer differs from open-source
approximations.
**How to avoid:** ONLY use `result.usage.input_tokens` /
`result.usage.output_tokens` from CC json. Never tokenize prompts locally.
**Warning signs:** "cost_summary.json" diverges from Anthropic console
usage by > 5%.

### Pitfall 5: Trace merge assumes monotonic clocks across processes

**What goes wrong:** Daemon and CC are spawned within milliseconds of each
other; OS clock skew doesn't matter, but if a future fork uses two hosts,
event ordering breaks.
**Why it happens:** Wall-clock alignment is the chosen ordering policy.
**How to avoid:** Document "single-host invariant" in
`internal/eval/trace/merge.go`. If we ever cross hosts, switch to a
correlation-ID based merge (thread `tool_use_id` through MCP envelope).
**Warning signs:** Trace events appearing out of causal order
(`tool_result` before `tool_call`).

### Pitfall 6: `eval-quick` falsely advertises end-to-end coverage

**What goes wrong:** Engineers think `eval-quick` exercises the full agent
loop; it does not (it uses a scripted "agent" that fires hard-coded calls).
**Why it happens:** The name suggests parity.
**How to avoid:** Document loudly in `EVAL.md` and `make eval-quick` output
("eval-quick: scripted-agent harness validation only; agent behavior is
NOT measured. Run `make eval` for behavior."). Also include a CI assertion:
the runner refuses to mark `EvalResult.success=true` for `eval-quick` runs
unless explicitly invoked with `--quick`.
**Warning signs:** A regression that breaks real-agent tool selection
slips past `eval-quick` and into nightly `make eval`.

### Pitfall 7: ZDR attestation gate is human-trust-only

**What goes wrong:** `HELIX_EVAL_ZDR_VERIFIED=1` is a checkbox; nothing
checks Anthropic's actual account state.
**Why it happens:** Anthropic does not expose a per-request ZDR header or
account-status API. ZDR is a contract-level setting.
**How to avoid:** EVAL.md must include an operator-checklist that points
at the signed DPA and instructs to re-verify quarterly. The env-var gate
is a circuit-breaker on accidental private-corpus runs, not a
verification mechanism.
**Warning signs:** A non-OSS, non-synthetic corpus shows up in
`eval/corpus/`; CI must reject this without `HELIX_EVAL_ZDR_VERIFIED=1`
in the run env.
[CITED: platform.claude.com/docs/en/build-with-claude/api-and-data-retention]

### Pitfall 8: Judge non-determinism gates merges by accident

**What goes wrong:** Someone wires `tool_behavior_judge.json` into a CI
threshold check; flaky judge starts blocking PRs.
**Why it happens:** The data is sitting next to `tool_behavior.json`; easy
to copy-paste-extend.
**How to avoid:** Judge output goes in a separate file with a clearly
labeled "INFORMATIONAL — DO NOT USE FOR CI GATING" comment at the top of
the JSON (yes, JSON-as-comment-via-`__readme` field). Linter rule that
fails if `tool_behavior_judge.json` is referenced in CI workflow YAML.
[CITED: REQUIREMENTS.md EVAL-07]

### Pitfall 9: CGO-enabled cross-platform release vs. eval runner

**What goes wrong:** `cmd/helix-eval/` adds CGO deps that break
windows/arm64 (still on the platform-conditional stub per Phase 59.1).
**Why it happens:** Anthropic SDK or some new dep pulls in cgo.
**How to avoid:** Vet that `cmd/helix-eval/` builds on every release-matrix
target. If Anthropic SDK is CGO-clean, fine. If not, hand-roll HTTP.
**Warning signs:** `make release-snapshot` fails on darwin or windows-arm64.
[CITED: CLAUDE.md "Build pipeline (CGO=1, split-runner per Phase 59.1)"]

## Code Examples

### Spawn a mode daemon with isolated socket + HOME

```go
// Source: internal/eval/sandbox/sandbox.go (planned)
func (s *Sandbox) StartDaemon(ctx context.Context, mode string) (*DaemonHandle, error) {
    sockPath := filepath.Join(s.modeDir(mode), "daemon.sock")
    cfgPath  := filepath.Join(s.modeDir(mode), "home", ".helix", "helix_config.yml")

    cmd := exec.CommandContext(ctx, s.helixBin,
        "--serve",
        "--socket", sockPath,
        "--config", cfgPath,
        "--profile", profileForMode(mode),
        "--json", // structured logs we tap for trace_daemon.jsonl
    )
    cmd.Env = []string{
        "PATH="+os.Getenv("PATH"),
        "HOME="+filepath.Join(s.modeDir(mode), "home"),
        "HELIX_LOG_LEVEL=info",
    }
    cmd.Stderr = s.daemonLogFile(mode)
    if err := cmd.Start(); err != nil { return nil, err }

    // Wait for socket to exist (forwarder.waitForDaemon pattern)
    if err := waitSocket(ctx, sockPath, 10*time.Second); err != nil {
        _ = cmd.Process.Kill()
        return nil, err
    }
    return &DaemonHandle{Cmd: cmd, Socket: sockPath}, nil
}
```

[Pattern source: `internal/forwarder/dial.go:83-115` `startDaemon` + `waitForDaemon`]

### Spawn `claude` subprocess with isolated MCP config

```go
// Source: internal/eval/agent/claude.go (planned)
func (a *Agent) Run(ctx context.Context, task Task, mode string) (*ClaudeOutput, error) {
    mcpCfg := mcpConfigJSON(a.helixBin, a.sandbox.SocketFor(mode), a.sandbox.HomeFor(mode))
    if err := os.WriteFile(a.mcpConfigPath(task, mode), mcpCfg, 0600); err != nil {
        return nil, err
    }

    cmd := exec.CommandContext(ctx, "claude",
        "--print",
        "--bare",
        "--strict-mcp-config",
        "--mcp-config", a.mcpConfigPath(task, mode),
        "--output-format", "stream-json",
        "--verbose",
        "--include-partial-messages",
        "--max-turns", strconv.Itoa(task.Budget.MaxToolCalls),
        task.Prompt,
    )
    cmd.Dir = a.sandbox.RepoFor(task, mode)
    cmd.Env = a.cleanEnv()
    cmd.Stdout = a.ccStdoutFile(task, mode)
    cmd.Stderr = a.ccStderrFile(task, mode)

    return run(cmd, task.Budget.MaxSeconds)
}
```

[Pattern source: code.claude.com/docs/en/cli-reference flags table; code.claude.com/docs/en/headless §Bare mode]

### Apply a heuristic rule against a merged trace

```go
// Source: internal/eval/score/score.go (planned)
func (s *Scorer) Score(trace MergedTrace, rules Rules) Score {
    var score Score
    for _, rule := range rules.ExpectSequence {
        if matchSubsequence(trace.Events, rule.Pattern) {
            score.Add(rule.ID, +rule.Score)
        }
    }
    for _, rule := range rules.ForbidSequence {
        if matchSubsequence(trace.Events, rule.Pattern) {
            score.Add(rule.ID, -rule.Score)
        }
    }
    // ... expect_set, forbid_set, receipts
    return score
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `claude -p --output-format=text` for headless | `claude -p --bare --output-format=stream-json --verbose` | CC v2.x onward (2.1.x stable today) | Rich event stream with tokens + tool_use; mandatory for our merger |
| Pinning `claude` version | Recording `claude --version` per run | Phase 67 D-03 | Comparability without upgrade-blocking |
| Static price tables | Token-counting from API response | EVAL D-07 | Avoids stale-pricing reports |
| In-process MCP test harness | Out-of-process subprocess per mode | EVAL-02 | Real client, real isolation |

**Deprecated/outdated:**
- "headless mode" terminology is now "Agent SDK CLI" per CC docs; `-p` flag unchanged. [CITED: code.claude.com/docs/en/headless]
- Older `--mcp-config` examples that omit `"type": "stdio"`. Include `type` for forward-compat.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Empty `skills: []` + `tools: []` in a profile YAML causes ProfileFilterMiddleware to expose zero Helix tools | §"Baseline Profile" | If wrong, baseline.yaml needs a new `disable_all_tools: true` field; small schema bump + middleware patch. **Planner's first task: write the assertion test before depending on it.** |
| A2 | Anthropic Go SDK module path is `github.com/anthropics/anthropic-sdk-go` and is CGO-clean | §"Standard Stack" | If wrong / not CGO-clean, hand-roll HTTP (60 LOC). Trivial pivot. |
| A3 | Daemon and CC subprocesses share OS clock within ~1ms | §"Trace Merge Schema" | If wrong (clock-skew on the same host?), trace ordering breaks; would need correlation IDs. Unlikely — same host. |
| A4 | `claude --version` output is stable enough to use as a comparability key | §"State of the Art" | If output format changes, parser breaks. Mitigation: regex tolerant of "X.Y.Z (Claude Code)" shape. |
| A5 | CC's `result.usage.{input,output}_tokens` reflects billing 1:1 | §"Don't Hand-Roll" | If diverges, cost_summary is misleading. Spot-check against Anthropic console during plan. |
| A6 | `phasegraph.RunPhaseGraph` can host an eval pipeline without modification | §"PhaseGraph Integration" | If shape doesn't fit (e.g., needs error-tolerant phase), extend phasegraph; coordinate with Phase 57 owner. |
| A7 | Budget enforcement via `context.WithTimeout` + tool-call counter middleware is sufficient | §"Don't Hand-Roll" | Token budget enforcement may need a separate watchdog reading streaming events. Acceptable additional code. |

## Open Questions

1. **Anthropic SDK vs. hand-rolled HTTP for judge**
   - What we know: Anthropic publishes a Go SDK; `net/http` works fine.
   - What's unclear: SDK CGO-cleanness; SDK API stability vs. our pinning policy.
   - Recommendation: planner picks; default to hand-rolled HTTP if SDK adds any new build constraint.

2. **Languages in seed corpus**
   - What we know: Helix's first-class LSP tier is Java/Go/Rust/TS/JS (CLAUDE.md).
   - What's unclear: rust-analyzer + jdtls warm-up time may stretch eval-quick beyond 30s budget if included.
   - Recommendation: seed = **Go + TypeScript + Python** (3 langs covers scripting + statically typed + the Helix codebase itself; jdtls/rust-analyzer add to a Wave 2 corpus once eval-quick budget proves headroom).

3. **In-process gRPC for `eval-quick`**
   - What we know: `daemon.Run()` blocks on signal; gRPC server binds a Unix socket.
   - What's unclear: cleanest seam to talk to the in-process daemon — call ToolRegistry directly (skipping MCP transport), or run gRPC over a `bufconn` listener.
   - Recommendation: planner inspects `internal/daemon/daemon.go` and chooses; bufconn is the most realistic.

4. **Judge model default**
   - What we know: Sonnet 4.6 is current Helix-blessed default.
   - What's unclear: whether to lock Opus for the rubric and Sonnet for per-task scoring.
   - Recommendation: Sonnet for both; let Opus run be opt-in via `--judge-model=opus`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `claude` (Claude Code CLI) | `make eval` (out-of-process modes) | ✓ | 2.1.138 (Claude Code) | None — `make eval` skipped if absent. `make eval-quick` does NOT require it (scripted agent). |
| Go toolchain | All builds + tests | ✓ | (project-pinned) | — |
| `git` | `apply_patch_check` phase | ✓ | (system) | — |
| `bash` (for `verify.sh`) | per-task verifiers | ✓ | (system) | Could use `sh` if portability becomes an issue; verify.sh contracts are bash-only today. |
| `ANTHROPIC_API_KEY` | `--bare` mode (no OAuth) and LLM judge | unverified at research time | — | Eval fails fast with clear "set ANTHROPIC_API_KEY for `make eval`" error; `make eval-quick` does NOT need it. |
| `make` | Makefile targets | ✓ | (system) | — |

**Missing dependencies with no fallback:** none for Wave 0 / `eval-quick`.

**Missing dependencies with fallback:**
- `claude` CLI on a contributor's machine: `make eval` skipped with clear error; `make eval-quick` runs.
- `ANTHROPIC_API_KEY`: judge skipped silently (per EVAL-07; informational).

## Threat Model Seeds

The eval harness writes patches to disk, spawns subprocesses, and invokes
external services. Worth a STRIDE pass; this is research seed material
that the planner expands into a full threat model in PLAN.md.

| Asset | Attacker(s) | Threat (STRIDE) | Mitigation |
|-------|-------------|-----------------|------------|
| Eval workspace tmpdirs | Local attacker / coworker on shared host | **Tampering**: poisoning a per-task `repo/` tmpdir mid-run | `os.MkdirTemp` with default 0700 perms; reject if tmpdir lookup follows a symlink. |
| Per-mode HOME dir | Local attacker | **Tampering**: writing a `.helix/helix_config.yml` mid-run that the daemon re-reads | Daemon reads config once at start; subsequent writes ignored. Document this. |
| `claude` subprocess stdout/stderr files | Local attacker | **Information Disclosure**: trace contains user prompts | Files mode 0600; tmpdir 0700. Reports written under same perms. |
| `ANTHROPIC_API_KEY` env var | Subprocess (`claude` itself, hooks) | **Information Disclosure**: leaks via `process.environ` introspection | `--bare` skips hooks; runner scrubs `cmd.Env` to allowlist. |
| Eval corpus (private repos under ZDR mode) | Anthropic API (no ZDR) / log retention | **Information Disclosure**: prompt + code shipped to Anthropic and stored 7d (default) or 0d (ZDR) | EVAL.md attestation gate; `HELIX_EVAL_ZDR_VERIFIED=1` env-var on non-synthetic corpora. |
| `verify.sh` execution | Malicious task author | **Elevation of Privilege**: `verify.sh` runs as eval user | Per-task `repo/` is a clone; `verify.sh` is in-corpus and reviewed at PR time. CI does NOT pull external corpora. |
| MCP config file `mcp-config.json` | Local attacker | **Tampering**: redirect daemon socket to attacker-controlled socket | File mode 0600; written into 0700 tmpdir; absolute path passed to `claude`. |
| Daemon Unix socket | Local attacker | **Spoofing**: bind a fake daemon to the same socket | Per-mode tmpdir socket (0700 dir); fail if socket exists at start. |
| LLM judge prompt | (none directly attackable; informational) | **Tampering**: judge prompt manipulation by task author | Judge sees ONLY merged trace, NOT raw `task.md` content; rubric is checked into source. |
| Reports (eval/reports/) | Reviewer reading reports | **Information Disclosure**: reports may contain code snippets from non-OSS corpora | Reports of non-synthetic runs marked `private: true` in metadata; CI must not upload private reports as artifacts. |
| Network egress to Anthropic | Hostile network | **Tampering / DoS**: MITM injects bad responses | TLS via Anthropic SDK / `net/http` defaults; pin certs is overkill at this scope. |
| Concurrent eval runs | Two devs running `make eval` on same host | **DoS / Tampering**: socket / tmpdir collision | Run-id namespacing (`$TMPDIR/helix-eval-<run-id>/`); document concurrent runs are supported. |

**Trust boundaries:**
1. **Eval runner ↔ daemon-under-test**: trusted (we spawned it; clean env).
2. **Eval runner ↔ `claude` subprocess**: semi-trusted (binary we did not build); contained via `--bare`, `--strict-mcp-config`, scrubbed env.
3. **Eval runner ↔ Anthropic API**: untrusted network (TLS); contained via per-account ZDR contract.
4. **Eval runner ↔ corpus task author**: semi-trusted (PR-reviewed); `verify.sh` runs in workspace tmpdir, not user $HOME.

**Out-of-scope this phase:**
- Sandboxing `verify.sh` from the runner's user (would need containers).
- Defending against a compromised `claude` binary (would need binary attestation).

## Sources

### Primary (HIGH confidence)
- `.planning/REQUIREMENTS.md` lines 110-118 — EVAL-01..EVAL-07 verbatim.
- `.planning/phases/67-evaluation-harness/67-CONTEXT.md` — D-01..D-08 + open questions.
- `.planning/milestones/v1.10-ROADMAP.md` lines 194-204 — Phase 67 charter.
- `internal/profile/loader.go`, `profiles/*.yaml`, `profile.go` — profile loading + ProfileGuardrailsConfig.
- `internal/forwarder/forwarder.go`, `dial.go` — `RunForwarder` + `ConnectOrStartDaemon` patterns.
- `internal/mcp/middleware.go` lines 102-181 — TelemetryMiddleware outcome enum (incl. Phase 66 additions).
- `internal/config/loader.go` lines 23-76 — config layering + `DefaultSocketPath`.
- `internal/phasegraph/pipelines/eval.go` — 10-phase eval DAG declaration.
- `internal/cli/root.go` lines 60-176 — daemon flag surface (`--socket`, `--config`, `--profile`, `--admin-addr`, `--serve`).

### Secondary (MEDIUM confidence)
- [code.claude.com/docs/en/cli-reference](https://code.claude.com/docs/en/cli-reference) — `--print`, `--mcp-config`, `--strict-mcp-config`, `--output-format`, `--bare`, `--max-turns`, `--max-budget-usd` (retrieved 2026-05-10).
- [code.claude.com/docs/en/headless](https://code.claude.com/docs/en/headless) — Bare mode + JSON output schema + stream-json events (retrieved 2026-05-10).
- [code.claude.com/docs/en/mcp](https://code.claude.com/docs/en/mcp) — `--mcp-config` JSON schema with `type: stdio`, `command`, `args`, `env` (retrieved 2026-05-10).
- [privacy.claude.com/en/articles/8956058](https://privacy.claude.com/en/articles/8956058-i-have-a-zero-data-retention-agreement-with-anthropic-what-products-does-it-apply-to) — ZDR scope.
- [platform.claude.com/docs/en/build-with-claude/api-and-data-retention](https://platform.claude.com/docs/en/build-with-claude/api-and-data-retention) — ZDR opt-in mechanics.
- [code.claude.com/docs/en/zero-data-retention](https://code.claude.com/docs/en/zero-data-retention) — Claude Code ZDR via Enterprise.

### Tertiary (LOW confidence — flagged for plan-time validation)
- Anthropic Go SDK module path + CGO-cleanness (assumption A2).
- Empty-skills profile semantics (assumption A1).
- Token-billing 1:1 mapping (assumption A5).

## Metadata

**Confidence breakdown:**
- File-system layout: HIGH — derived from existing repo conventions.
- Subprocess isolation: HIGH — backed by code reads of forwarder + config.
- Baseline profile: MEDIUM — depends on profile-loader empty-list semantics (A1); planner verifies first.
- Trace merge schema: HIGH — both event sources documented.
- Heuristic DSL: HIGH — small, conservative; aligns with CONTEXT.md D-06 examples.
- LLM judge: MEDIUM — model + cost ceiling estimates; rubric needs design.
- TOS attestation: HIGH — backed by Anthropic public docs.
- PhaseGraph: HIGH — pipeline already declared.
- Threat model: MEDIUM — first-pass; planner expands.

**Research date:** 2026-05-10
**Valid until:** ~2026-06-10 (CC version + Anthropic ZDR docs are the most-likely-to-drift sources; recheck monthly).
