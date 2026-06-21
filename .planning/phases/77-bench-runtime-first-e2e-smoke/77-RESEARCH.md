# Phase 77: Bench Runtime & First E2E Smoke - Research

**Researched:** 2026-06-17
**Domain:** Go subprocess orchestration, MCP forwarder drive, OTel trace merge, JSON-schema validation, cobra CLI, Makefile wiring
**Confidence:** HIGH (every reused symbol read directly from source; no external library introduced)

## Summary

Phase 77 is **almost entirely wiring of existing, verified code** — no new dependencies, no
external packages. The Phase 67 `internal/eval/` machinery already implements every hard part:
per-`(task,mode)` filesystem isolation (`internal/eval/sandbox`), subprocess daemon spawn over a
per-cell Unix socket with HTTP disabled (`Sandbox.StartDaemon`), the forwarder-drive path that
triggers `TelemetryMiddleware` "tool call" JSONL emission (`daemon_tap_integration_test.go`), the
PID-gated daemon-tap (`trace.TapDaemonLog`), the agent-tap stream type (`trace.CCTapResult`), and
the timestamp-ordered merge with outcome resolution (`trace.Merge`). The bench runtime thin-wraps
these into `bench/runtime/` and implements the `notYetImplemented("run")` stub at
`cmd/helix-bench/main.go:89`.

The single genuinely-new code is glue: (1) a mode→profile resolver reading
`bench/runners/your_agent_full/MODE.md` frontmatter → `bench-full`; (2) a synthesizer that turns
the scripted agent's executed `[]StepResult` into a `trace.CCTapResult` (the D-02 agent-tap leg)
so the 2-leg merge is asserted hermetically; (3) a `result.v2.json` builder (NONE exists today —
only the schema + a golden fixture); (4) the cobra `run` subcommand + flags; (5) one seed Go
fixture under `bench/datasets/toolbench-go/<task>/`; (6) Make targets.

**Primary recommendation:** Build `bench/runtime/` by composing the *subprocess* daemon path
(`runner.go` `RunTask` + `daemon_tap_integration_test.go` drive pattern), NOT the in-process
`inprocess.go` `RunQuick` path. `RunQuick` boots an in-process `daemon.New` over
`InMemoryTransports` and produces a *synthetic single-leg* trace with no PID gate — that violates
D-06/D-08 (subprocess + PID-gated tap). Reuse `inprocess.go` only for two patterns: the
`scripted_agent.yaml` LoadScript→ScriptedAgent.Run loop, and the `sessionCaller` MCPCaller adapter
shape. Drive the scripted agent through the *forwarder* (subprocess) like the integration test,
not through `InMemoryTransports`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Scripted agent gates CI; real `claude` CLI path wired but NOT gating. `make bench-quick`
  and the first E2E smoke run the in-process scripted agent (`internal/eval/runner/scripted_agent.go`,
  driven by `scripted_agent.yaml`) — deterministic, hermetic, no `$ANTHROPIC_API_KEY`, inside ≤30s/≤90s.
  The real `claude` CLI path (`internal/eval/agent/claude.go` + `mcpconfig.go`) is fully wired and
  locally runnable (e.g. `helix-bench run --agent=claude …`) but NOT a CI gate this phase.
- **D-02:** The agent-tap leg is synthesized as a `trace.CCTapResult` from the scripted agent's
  executed tool-call sequence, so hermetic `bench-quick` produces and asserts a real 2-leg merged
  trace (`trace.Merge` over `Daemon` + `CC` streams) with zero orphan spans and a PID-gated
  daemon-tap (`trace.TapDaemonLog(daemonPID)`). The real `claude` path produces the same
  `CCTapResult` shape from its transcript.
- **D-03:** Ship exactly ONE minimal-but-real Go task at `bench/datasets/toolbench-go/<task>/`
  (tiny Go module + one failing test + `task.json` + a `verify` step running `go test`). The
  scripted agent replays a real MCP edit (e.g. `replace_in_file` / structured-edit tool) to make
  the test pass. This is the SEED task; Phase 78 grows the corpus in the same directory.
- **D-04:** `result.v2.json` populated with outcome + provenance, schema-valid. Fields: pass/fail
  outcome from the verify exit code (reuse `trace.Merge`'s `VerifyExitCode` resolution), the
  fairness block (from `bench/runners/fairness_contract.go`), token/cost columns available from the
  run, the merged-trace reference, and `schema_version: "v2"`. Rich metrics (`edit_locality`,
  `regression_rate`, pass@k) left null/absent (Phase 79). MUST validate against
  `bench/schema/result.v2.schema.json` today.
- **D-05:** Seed the ABLATE-01-shaped convention for ONLY the mode Phase 77 needs. Create
  `bench/runners/your_agent_full/MODE.md` whose frontmatter declares `profile: bench-full`, plus a
  small resolver that maps a canonical mode name → profile name by reading that MODE.md. The runner
  translates `--modes=your_agent_full` → `profileName=bench-full`, then calls the existing
  `StartDaemon(ctx, taskID, mode, profileName, cfgPath)` — which ALREADY separates the `mode` label
  (per-cell dir/socket) from the `profileName` (passed as `--profile=`). Phase 80 grows the
  resolver; it does NOT refactor. Do NOT rename Phase 76 profile files.
- **D-06:** Per-cell Unix domain socket; HTTP disabled; no TCP ports. Reuse eval's path verbatim:
  each `(task × mode)` spawns `helix --serve --socket=<per-cell-path> --http-addr= --json`; the
  agent reaches the daemon via `helix --mode=stdio --socket=…` forwarder. "No port collisions on
  `--parallel=4`" holds by construction.
- **D-07:** `bench/runtime/sandbox/` EMBEDS `internal/eval/sandbox.Sandbox` (thin wrapper adding
  only bench-specific paths such as `result.v2.json` and synthesized-JSONL locations) — does NOT
  fork/copy. `bench/runtime/subprocess/` owns the `helix daemon` (and eventual `claude`) lifecycle.
- **D-08:** Cell key = `<run_id>/<task>/<mode>`; durable artifacts vs ephemeral scratch split.
  Durable (`result.v2.json`, merged-trace JSON, `daemon.log`) → out dir (default
  `bench/reports/<run_id>/<task>/<mode>/`, overridable via `--out`). Ephemeral (repo working copy,
  daemon HOME, Unix socket) → OS temp sandbox, deleted on success, PRESERVED on failure. Daemon-tap
  PID-gated to the spawned daemon's PID (`trace.TapDaemonLog(daemonPID)`). `run_id` reuses eval's
  shape.

### Claude's Discretion
- `MODE.md` frontmatter schema beyond `mode` + `profile` keys — keep minimal; Phase 80 owns the full ABLATE-01 convention.
- `run_id` exact string format (reuse eval's; exact layout is discretion).
- Exact `make bench` / `bench-quick` / `bench-<suite>` target wording and default args. Fixed
  semantics: `bench-quick` runs the hermetic scripted `your_agent_full` smoke on the one
  `toolbench-go` task ≤90s; `bench-<suite>` parameterizes `--benchmarks=<suite>`; all invoke
  `cmd/helix-bench run --benchmarks=…`.
- Whether `bench/reports/<run_id>/` is git-ignored (ops detail; `reports/` is a runtime dir).
- The `scripted_agent.yaml` step sequence for the seed task and exact MCP edit tool used.
- Internal package layout of `bench/runtime/` (`sandbox/`, `subprocess/`, runner glue) and the
  `helix-bench run` flag plumbing (`--benchmarks`, `--modes`, `--tasks`, `--parallel`, `--out`, `--agent`).

### Deferred Ideas (OUT OF SCOPE)
- Real `claude` CLI as a CI gate (needs API-key secret + network; local/nightly only — path wired, not gating).
- Full ToolBench Go corpus + capability matrix — Phase 78.
- Rich result metrics (`edit_locality`, `regression_rate`, pass@k, multi-run aggregation) — Phase 79/82.
- Remaining 5 modes + full `bench/runners/<mode>/MODE.md` convention (ABLATE-01) — Phase 80.
- Container runtime / per-language runners / GHCR mirror — Phases 84–88.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BENCH-04 | Bench runtime reuses Phase 67 `internal/eval/sandbox` + subprocess patterns; one daemon subprocess per `(task × mode)`. Accept: smoke ≤30s smallest task; no port collisions on parallel. | `Sandbox.StartDaemon` (sandbox.go:231) spawns `--serve --socket=… --http-addr= --json`; no-TCP-port design ⇒ no collisions by construction (D-06). Subprocess+tap proven by `daemon_tap_integration_test.go`. `RunMatrix` (runner.go:304) is the per-cell concurrency template (`sem` bounded by `MaxParallel`). |
| BENCH-05 | `make bench`, `make bench-quick`, `make bench-<suite>` exist and invoke `cmd/helix-bench run --benchmarks=…`. Accept: `make bench-quick` exits 0 with ≥1 task succeeding ≤90s. | Existing Make targets `eval-quick`/`eval` (Makefile:230,240) are the wiring template; the `bench:` target name is currently CLAIMED by a Go microbenchmark (Makefile:95) — see **Pitfall 1 (target-name collision)**. |

> NOTE: BENCH-04 acceptance text says "in-process smoke run" but D-06/D-08 lock the **subprocess**
> daemon path (per-cell Unix socket, PID-gated tap). The roadmap success criterion #1 + #3 + #4 are
> authoritative and require subprocess. The "in-process" wording in REQUIREMENTS.md line 20 is
> imprecise; honor D-06/D-08 (subprocess).
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Per-cell FS isolation (HOME/socket/repo) | `bench/runtime/sandbox` (embeds eval sandbox) | `internal/eval/sandbox` | D-07: thin-wrap, never fork. All path hardening already lives in eval sandbox. |
| Daemon subprocess lifecycle | `bench/runtime/subprocess` | `internal/eval/sandbox.StartDaemon` | D-07: subprocess owns spawn/kill; StartDaemon already spawns `helix --serve --socket=…`. |
| Agent drive (scripted / claude) | `bench/runtime` runner glue | `internal/eval/runner.ScriptedAgent`, `internal/eval/agent.Agent` | D-01: scripted gates, claude wired-not-gating. Both already implemented. |
| MCP tool dispatch to daemon | helix forwarder subprocess (`--mode=stdio --socket=`) | — | D-06: forwarder is the transport; no in-process server in the smoke. |
| Daemon-side telemetry (tool-call JSONL) | helix daemon `TelemetryMiddleware` | — | Already emits `msg:"tool call"` lines tapped by `TapDaemonLog`. |
| Trace tap + merge | `internal/eval/trace` | bench runner glue | D-02: reuse `TapDaemonLog` + `Merge`; synthesize the `CCTapResult` leg. |
| result.v2.json build + validate | `bench/runtime` (NEW builder) | `bench/schema` (schema only) | D-04: no builder exists today; must be written. |
| Mode→profile resolution | `bench/runners` (NEW resolver + MODE.md) | `internal/profile/profiles/bench-full.yaml` | D-05: extensible resolver reading MODE.md frontmatter. |
| Outcome resolution | `internal/eval/trace.Merge` | — | D-04: reuse `VerifyExitCode` → outcome priority in `Merge`. |

## Standard Stack

No new external dependency is introduced. Everything reused already lives in `go.mod`.

### Core (already present — reuse, do not re-add)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.9.1 | `helix-bench run` subcommand + flags | Project CLI standard; helix-eval `run` is the exact analog. `[VERIFIED: go.mod]` |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | Validate `result.v2.json` against schema (Draft 2020-12) | Already used by `bench/schema/result.v2_test.go`; reuse its compile pattern verbatim. `[VERIFIED: go.mod:20 + result.v2_test.go:8]` |
| `gopkg.in/yaml.v3` | v3.0.1 | Parse `scripted_agent.yaml` + `MODE.md` frontmatter | Already used by `scripted_agent.go` (`yaml.NewDecoder` + `KnownFields(true)`). `[VERIFIED: go.mod:56 + scripted_agent.go:19]` |
| `github.com/modelcontextprotocol/go-sdk/mcp` | (indirect, present) | In-process session type (only if you keep `sessionCaller`) | Used by `inprocess.go`; for the subprocess drive you instead shell the forwarder (no SDK client needed). `[VERIFIED: inprocess.go:25]` |

### Supporting (internal packages — reuse directly)
| Package | Purpose | Key Symbols |
|---------|---------|-------------|
| `internal/eval/sandbox` | Per-cell isolation + daemon spawn | `Sandbox`, `NewSandbox`, `StartDaemon`, `ModeDir`, `HomeFor`, `RepoFor`, `SocketFor`, `McpConfigPath`, `Prepare`, `CloneRepo`, `DaemonHandle.Pid`, `DaemonHandle.Kill`, `Cleanup` |
| `internal/eval/trace` | Tap + merge | `TapDaemonLog`, `Merge`, `MergeInput`, `DaemonTapResult`, `CCTapResult`, `MergedTrace`, `Event`, `Usage`, `ToolCallSummary`, `KindToolCall`, `KindToolResult`, `KindResult`, `KindSessionInit` |
| `internal/eval/runner` | Scripted agent + drive pattern | `ScriptedAgent`, `NewScriptedAgent`, `LoadScript`, `Script`, `ScriptedStep`, `StepResult`, `MCPCaller` |
| `internal/eval/agent` | Real claude path (wired-not-gating) | `Agent`, `NewAgent`, `Run`, `Task`, `BudgetParams`, `ErrClaudeNotFound`, `WriteMCPConfig`, `MCPConfig` |
| `bench/runners` (pkg `runners`) | Fairness contract | `DefaultContract` (`FairnessContract`), `ModelID`, `Temperature`, `MaxTokens`, `SystemPromptHash`, `Retry`, `Cache`, `Overrides` |
| `internal/eval/budget` | Watchdog + breach | `budget.Default`, `budget.LoadFromFile`, `budget.NewWatchdog`, `budget.BreachReason` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Subprocess daemon + forwarder drive (D-06) | In-process `daemon.New` + `InMemoryTransports` (`inprocess.go`) | In-process is simpler BUT has no PID gate and produces a single-leg synthetic trace → violates D-02/D-06/D-08. Reject for the smoke. |
| Synthesizing `CCTapResult` from `[]StepResult` (D-02) | Skipping the agent-tap leg entirely | A 1-leg merge cannot assert "2-leg merged trace, zero orphan spans" (criterion #4). Must synthesize. |

**Installation:** No `go get` needed. All imports already resolve.

**Version verification:** `[VERIFIED: go.mod]` — `santhosh-tekuri/jsonschema/v6 v6.0.2`, `yaml.v3 v3.0.1`,
`cobra v1.9.1`. No registry fetch performed (no new package); all three are pre-existing direct deps.

## Package Legitimacy Audit

> No external packages are installed by this phase. All reused libraries are pre-existing direct
> dependencies in `go.mod`. The legitimacy gate is **N/A** (zero new packages).

| Package | Registry | Status | Disposition |
|---------|----------|--------|-------------|
| (none) | — | — | No new dependency introduced |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=<id> [--parallel=4] [--agent=scripted|claude] [--out=DIR]
        │
        ▼
 [matrix expander]  expand (benchmark × mode × task) → cells; cell key = <run_id>/<task>/<mode>
        │
        ▼ (per cell, bounded by --parallel sem)
 ┌──────────────────────────── bench/runtime runner glue ─────────────────────────────┐
 │  1. resolve mode→profile     your_agent_full → read bench/runners/your_agent_full/  │
 │                              MODE.md frontmatter → "bench-full"          (D-05)      │
 │  2. bench sandbox (EMBEDS internal/eval/sandbox.Sandbox)                 (D-07)      │
 │       NewSandbox(runID+"-"+task) → Prepare(task,mode) → CloneRepo(seed) │
 │       ephemeral OS temp: HOME, repo working copy, daemon.sock           (D-08)      │
 │  3. subprocess: StartDaemon(ctx, task, mode, "bench-full", cfgPath)     (D-06)      │
 │       spawns:  helix --serve --socket=<cell>/daemon.sock --http-addr= --json --profile=bench-full
 │       capture daemonPID = handle.Pid()   ← PID gate seed                            │
 │                                  │                                                   │
 │  4. drive agent ─────────────────┤                                                  │
 │     ┌── scripted (CI, D-01) ─────┴── claude (--agent=claude, wired-not-gating) ──┐  │
 │     │  shell: helix --mode=stdio    │  agent.Agent.Run: claude CLI subprocess    │  │
 │     │   --socket=<cell>/daemon.sock │  with --mcp-config (WriteMCPConfig points  │  │
 │     │  feed NDJSON: initialize +    │  helix forwarder at the cell socket);      │  │
 │     │   tools/call per step from    │  writes claude.stdout (stream-json)        │  │
 │     │   scripted_agent.yaml         │                                            │  │
 │     │  collect []StepResult         │                                            │  │
 │     └───────────────┬──────────────┴───────────────────┬────────────────────────┘  │
 │  5. kill daemon (flush slog) → daemon.log               │                           │
 │  6. daemon-tap:  TapDaemonLog(daemon.log, daemonPID)  ──┤  agent-tap (D-02):        │
 │       → DaemonTapResult (PID-gated; RejectedForeignPid) │   scripted → synthesize   │
 │                                                         │   CCTapResult from steps  │
 │                                                         │   claude → TapCCStream    │
 │  7. run verify step (go test) in repo → verifyExit                                  │
 │  8. trace.Merge(MergeInput{Daemon, CC, VerifyExitCode, RepoRoot,…}) → MergedTrace   │
 │       outcome = budget>verify priority; zero-orphan by construction                 │
 │  9. build result.v2.json (NEW): schema_version=v2, outcome, benchmark, mode,        │
 │       task_id, run_index, tokens (from CC.Usage), fairness (DefaultContract),       │
 │       trace-ref → durable out dir; metrics absent                       (D-04)      │
 │ 10. validate result.v2.json against bench/schema/result.v2.schema.json (jsonschema) │
 │ 11. write durable artifacts → bench/reports/<run_id>/<task>/<mode>/     (D-08)      │
 │       delete ephemeral temp on success; PRESERVE on failure                         │
 └─────────────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
 exit 0 iff ≥1 task succeeded (mirror helix-eval exit semantics: RunE returns error on fail)
```

### Component Responsibilities

| Path (new) | Responsibility | Wraps / Reuses |
|------------|----------------|----------------|
| `bench/runtime/sandbox/` | Embed `eval/sandbox.Sandbox`; add `ResultPath()`, `MergedTracePath()`, synthesized-JSONL path | `internal/eval/sandbox.Sandbox` (struct embed) |
| `bench/runtime/subprocess/` | Spawn/kill daemon (call `StartDaemon`); optionally spawn claude | `Sandbox.StartDaemon`, `agent.Agent` |
| `bench/runtime/` (runner glue, e.g. `cell.go`) | Orchestrate one cell end-to-end (steps 1–11) | `runner.ScriptedAgent`, `trace.*`, resolver |
| `bench/runtime/` (result builder, e.g. `result.go`) | Build + validate `result.v2.json` | `bench/runners.DefaultContract`, `jsonschema/v6` |
| `bench/runners/your_agent_full/MODE.md` | Frontmatter `profile: bench-full` | — |
| `bench/runners/` (resolver, e.g. `mode_resolver.go`) | `mode name → profile name` via MODE.md | `yaml.v3` |
| `bench/datasets/toolbench-go/<task>/` | Seed Go fixture (module + failing test + task.json + scripted_agent.yaml + verify) | mirrors `eval/fixtures/<id>/` layout |
| `cmd/helix-bench/main.go` | Implement `newRunCmd` body; add flags | replace `notYetImplemented("run")` (line 89) |
| Root `Makefile` | `bench-quick`, `bench-<suite>`; reconcile `bench` | mirror `eval-quick`/`eval` (lines 230,240) |

### Recommended Project Structure
```
bench/
├── runtime/
│   ├── sandbox/        # embeds internal/eval/sandbox.Sandbox + bench paths (D-07)
│   ├── subprocess/     # helix daemon + claude lifecycle (D-07)
│   └── *.go            # cell orchestration, result.v2 builder, matrix expander
├── runners/
│   ├── your_agent_full/
│   │   └── MODE.md     # frontmatter: profile: bench-full (D-05)
│   ├── fairness_contract.go   # EXISTS (Phase 75)
│   └── mode_resolver.go       # NEW: mode → profile
├── datasets/
│   └── toolbench-go/
│       └── <task>/     # seed: go.mod, *.go (failing test), task.json, scripted_agent.yaml, verify (D-03)
├── reports/            # durable out: <run_id>/<task>/<mode>/{result.v2.json,trace.json,daemon.log} (D-08)
└── schema/             # EXISTS (Phase 75) result.v2.schema.json
```

### Pattern 1: Subprocess daemon spawn over per-cell Unix socket (D-06)
**What:** Spawn one `helix --serve` daemon per cell on a per-cell `daemon.sock`, HTTP disabled.
**When:** Every cell. This is the no-port-collision mechanism.
**Exact signature** `[VERIFIED: internal/eval/sandbox/sandbox.go:231]`:
```go
func (s *Sandbox) StartDaemon(ctx context.Context, taskID, mode, profileName, cfgPath string) (*DaemonHandle, error)
// args passed to the daemon (sandbox.go:236-242):
//   ["--serve", "--socket="+sockPath, "--http-addr=", "--json"]   // HTTP off (empty addr)
//   + "--profile="+profileName  (when profileName != "")
//   + "--config="+cfgPath       (when cfgPath != "")
// env allowlist (sandbox.go:247-254): HOME=<HomeFor>, HELIX_LOG_LEVEL=info, PATH (inherited)
// stderr+stdout → <modeDir>/daemon.log  (sandbox.go:260-268)
// polls socket up to 10s (sandbox.go:275); kills+waits on failure
```
Capture the PID **immediately** for the tap gate: `daemonPID := handle.Pid()` (`sandbox.go:185`).

### Pattern 2: Drive a single tools/call through the forwarder (the smoke's agent transport)
**What:** Shell `helix --mode=stdio --socket=<sock>`, feed NDJSON `initialize` + `tools/call`,
which proxies to the daemon and triggers `TelemetryMiddleware` "tool call" JSONL emission.
**Source — copy this pattern** `[VERIFIED: internal/eval/runner/daemon_tap_integration_test.go:113-199]`:
```go
cmd := exec.CommandContext(fwdCtx, helixBin, "--mode=stdio", "--socket="+sockPath)
cmd.Env = append(os.Environ(), "HELIX_LOG_LEVEL=info")
stdin, _ := cmd.StdinPipe(); stdout, _ := cmd.StdoutPipe()
// NDJSON frames (do NOT close stdin between frames — closing races response delivery):
initFrame := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bench","version":"0"}}}`
callFrame := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, tool, argsJSON)
```
For the scripted seed task you iterate `scripted_agent.yaml` steps, one `tools/call` frame each,
recording each as a `StepResult` (tool name + AtTime + response/err) for the D-02 CCTapResult synth.

### Pattern 3: PID-gated daemon-tap (criterion #4 "zero PID cross-talk")
**What:** Read `daemon.log` and keep only `msg=="tool call"` / `"receipt issued"` lines whose
`pid == expectedPid`; foreign-PID lines counted in `RejectedForeignPid`.
**Exact signature** `[VERIFIED: internal/eval/trace/tap.go:44]`:
```go
func TapDaemonLog(path string, expectedPid int) (DaemonTapResult, error)
// DaemonTapResult{ Events []Event; RejectedForeignPid int; SkippedTruncated int }  (tap.go:11-20)
```
**Critical ordering** (`runner.go:181-187`, `integration_test:73-79`): KILL the daemon BEFORE
tapping so slog buffers flush; capture `preKillPid := handle.Pid()` BEFORE `Kill()` (Pid() may
return 0 after Wait on some platforms).

### Pattern 4: Synthesize CCTapResult from scripted steps (D-02, the genuinely-new glue)
**What:** Convert `[]runner.StepResult` into a `trace.CCTapResult` so `Merge` sees a real 2nd leg.
**Target type** `[VERIFIED: internal/eval/trace/tap.go:136-146 + schema.go]`:
```go
type CCTapResult struct {
    Events         []Event   // build these from steps
    Usage          Usage     // tokens; scripted = 0 (or omit); claude = from result event
    UnknownTypes   int
    FinalSessionID string
}
// Event with Source:"cc" carries: Kind (KindSessionInit|KindAssistantMsg|KindToolResult|KindResult),
// ToolUses []ToolUse, ToolUseID, IsError, SessionID, Model. (schema.go:104-135)
```
Suggested synthesis (mirror real claude stream shape so the leg looks identical to a claude run):
emit one `Source:"cc", Kind:KindSessionInit`; then per step a `Kind:KindAssistantMsg` carrying a
`ToolUse{Name: step.Tool, Input: step.Args}` followed by a `Kind:KindToolResult{ToolUseID, IsError: step.Err!=nil}`;
finally a `Kind:KindResult` with `Usage` (zero for scripted). Then `Merge` interleaves these with
the daemon tool_call events by timestamp. NOTE: `Merge` builds `ToolCallSummary` ONLY from
`Source:"daemon"` `KindToolCall` events (`merge.go:97-99`) — so the CC leg adds trace continuity
(the agent-side "2nd leg") without double-counting tool calls.

### Pattern 5: Trace merge + outcome resolution (D-04)
**Exact input/output** `[VERIFIED: internal/eval/trace/merge.go:13-42, 52]`:
```go
func Merge(in MergeInput) (MergedTrace, error)
type MergeInput struct {
    TaskID, Mode, RunID, ClaudeVersion, HelixVersion string
    StartedAt, EndedAt time.Time
    Daemon DaemonTapResult
    CC     CCTapResult
    VerifyExitCode int           // 0 = pass, non-zero = fail  (merge.go:29)
    Budget *budget.BreachReason  // non-nil → "failed-with-cause: budget_<axis>"
    PatchPaths []string          // T-67-02 path-prefix check
    RepoRoot string
}
// Outcome priority (merge.go:141-149): Budget breach > VerifyExitCode!=0 → "failed" > "success"
// MergedTrace.SchemaVersion is hard-coded "1" (merge.go:53); this is the TRACE schema, distinct
// from result.v2's schema_version="v2".
```

### Pattern 6: Scripted agent step format (D-03)
**Exact YAML shape** `[VERIFIED: internal/eval/runner/scripted_agent.go:22-36 + eval/fixtures/*/scripted_agent.yaml]`:
```yaml
steps:
  - tool: <mcp_tool_name>        # e.g. replace_in_file, replace_symbol_body
    args:                        # map[string]any passed verbatim
      <k>: <v>
    expect_error: false          # optional
```
`LoadScript` uses `KnownFields(true)` — unknown keys are a hard parse error (scripted_agent.go:48).
The `ScriptedAgent.Run` loop enforces a `budget.Watchdog` before each call (scripted_agent.go:97).

### Pattern 7: Mode→profile resolver (D-05)
The runner already separates `mode` (per-cell dir/socket label) from `profileName` (the
`--profile=` value). Build a resolver that reads `bench/runners/<mode>/MODE.md` frontmatter
(`profile: bench-full`) and returns the profile string, then call
`StartDaemon(ctx, task, "your_agent_full", "bench-full", cfgPath)`. Make the lookup table-driven
(scan `bench/runners/*/MODE.md`) so Phase 80 drops in 5 more dirs with no code change.
The bench profile `bench-full` is confirmed present `[VERIFIED: internal/profile/profiles/bench-full.yaml:12]`.

### Anti-Patterns to Avoid
- **Using `inprocess.go` RunQuick for the smoke** — it boots an in-process daemon over
  `InMemoryTransports` and builds a single-leg `buildSyntheticTrace` with NO PID gate
  (`inprocess.go:375-413`). That defeats criteria #3 (subprocess) and #4 (PID cross-talk + 2-leg).
  Reuse only its scripted-loop and `MCPCaller` shapes.
- **Adding TCP ports / `--http-addr` non-empty** — breaks the by-construction no-collision property (D-06).
- **Reusing eval's `mode→profile` mapping** — `runner.go:434 profileForMode` and
  `writeModeConfig` (runner.go:441) are hard-coded eval modes (`baseline/native/semantic/...`),
  NOT bench modes. Do not call them; write the bench resolver (D-05).
- **Tapping before killing the daemon** — slog lines won't be flushed; tap returns empty.
- **Naming the new full-run target `bench`** without reconciling the existing `bench:` microbench
  target (Makefile:95) — see Pitfall 1.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Per-cell FS isolation + symlink/traversal hardening | custom tmpdir mgmt | `eval/sandbox.Sandbox` (embed) | lstat-symlink reject, 0700, macOS socket-len handling already done (sandbox.go:39-81) |
| Daemon spawn + socket-wait + kill+reap | `exec.Command` by hand | `Sandbox.StartDaemon` / `DaemonHandle.Kill` | 10s socket poll, process-group SIGKILL, log capture done (sandbox.go:231,193) |
| PID-gated JSONL parse | custom log scanner | `trace.TapDaemonLog` | PID gate, truncation tolerance, 256KB buffer, RFC3339 defensive parse (tap.go) |
| Trace interleave + outcome | custom merge | `trace.Merge` | stable timestamp sort, daemon-tie-break, path-prefix invariant, budget>verify priority |
| claude CLI argv + MCP config + env scrub | custom subprocess | `agent.Agent.Run` + `WriteMCPConfig` | exact `--bare --strict-mcp-config --output-format stream-json` argv + allowlist env (claude.go:76,99) |
| JSON-schema validation | regex / manual field checks | `jsonschema/v6` (compile pattern from result.v2_test.go) | Draft 2020-12, offline, `UnmarshalJSON` preserving json.Number |
| Scripted step parse | custom YAML | `runner.LoadScript` | strict KnownFields, budget watchdog loop |

**Key insight:** Phase 67's F-07 close-out already proved subprocess daemon-tap works end-to-end;
the `daemon_tap_integration_test.go` is a ready-made, copy-paste-quality template for the entire
spawn→drive→kill→tap→merge spine. The bench runner is that test, generalized to N cells + the
synthesized CC leg + the result.v2 builder.

## Runtime State Inventory

> Phase 77 is greenfield code creation under `bench/` (new packages, new fixture, new Make
> targets), not a rename/refactor of existing runtime state. No stored data, live-service config,
> OS-registered state, secrets, or build artifacts carry an old identifier that must migrate.
> **None — verified by scope (all new files; no string-rename of existing keys/IDs).**

## Common Pitfalls

### Pitfall 1: `make bench` target-name collision (BLOCKER for BENCH-05)
**What goes wrong:** The Makefile ALREADY defines `bench:` (`Makefile:95`) as a Go microbenchmark
(`go test -bench=. ./test/bench/...`) and lists it in `.PHONY` (line 1). BENCH-05 requires
`make bench` to "invoke `cmd/helix-bench run --benchmarks=…`". A naive add silently shadows or
conflicts.
**Why:** Two unrelated meanings of "bench" predate this milestone.
**How to avoid:** The planner must decide reconciliation explicitly — options: (a) rename the
microbench target to `bench-micro` (it is gitignored/local per Makefile:98); (b) make the new full
benchmark target `bench` and migrate microbench callers. Document the choice in the plan; update
`.PHONY` accordingly. This is a Claude's-discretion area (target wording) but the collision is NOT
optional to resolve.
**Warning signs:** `make bench` runs `go test -bench` instead of `helix-bench run`.

### Pitfall 2: No result.v2.json Go builder exists yet
**What goes wrong:** Planner assumes a builder/type exists (CONTEXT mentions "existing Go builder").
It does NOT — only `result.v2.schema.json` + a golden fixture + a validation test exist
(`bench/schema/`). `bench/evaluators/aggregator/` does NOT exist (only `.gitkeep`).
**Why:** Phase 75 shipped the contract, not a writer.
**How to avoid:** Plan a NEW result builder (a struct with `json` tags matching the schema, or a
`map[string]any`) AND a validation step using the `result.v2_test.go` compile pattern. The schema's
ONLY required field is `schema_version` (`result.v2.schema.json:8`) and top-level
`additionalProperties` is OPEN — so a minimal provenance-complete doc (schema_version + outcome
fields + tokens + fairness + trace-ref) validates trivially, and metrics may be omitted (D-04).
**Note:** the schema has NO `outcome` or `trace_ref` property defined — they land as open
additional properties (valid by the additive-only=minor design). Confirm with a validate-on-write
test, do not assume named properties exist.

### Pitfall 3: Daemon-tap PID cross-talk on `--parallel` (F-07; criterion #4)
**What goes wrong:** Multiple concurrent cells; if a tap reads the wrong `daemon.log` or with the
wrong PID, foreign tool calls leak into a cell's trace.
**Why:** Shared-host, many daemons. PITFALLS.md §"trace merge" (line 330) flags this as the v1.12
scaling break of the Phase 67 single-host assumption.
**How to avoid:** (1) per-cell `daemon.log` path (already so — `<modeDir>/daemon.log`); (2) pass the
captured per-cell `daemonPID` to `TapDaemonLog`; (3) **add a cross-cell regression test** that
boots 2+ daemons in parallel and asserts `RejectedForeignPid == 0` AND zero foreign tools in each
merged trace (PITFALLS.md line 337 prescribes exactly this). Bound parallelism sensibly
(PITFALLS suggests `NumCPU()/2`); `--parallel=4` is the criterion-#1 target.
**Warning signs:** `DaemonTapResult.RejectedForeignPid > 0`; a tool in the trace the scripted task never called.

### Pitfall 4: Tapping before daemon flush returns an empty trace
**What goes wrong:** Reading `daemon.log` before `DaemonHandle.Kill()` yields zero events.
**Why:** slog buffers flush on process exit.
**How to avoid:** Always `Kill()` then tap; capture `preKillPid` before Kill (runner.go:181-187).

### Pitfall 5: Single-host wall-clock merge ordering (METRIC-06 boundary)
**What goes wrong:** `Merge` orders events by timestamp with a daemon-wins tie-break
(merge.go:73-88). Sub-ms skew between the synthesized CC events (`time.Now()` per event) and daemon
JSONL timestamps can invert ordering.
**Why:** `Merge` documents the single-host clock assumption (merge.go:48-51); PITFALLS line 338
recommends trace_id ordering as the durable fix (deferred — Phase 79+).
**How to avoid (this phase):** Acceptable for the smoke; assert on `ToolCallSummary.Total` and
`RejectedForeignPid==0`, NOT on exact event interleave order. Synthesize CC event timestamps from
the recorded `StepResult.AtTime` (real dispatch instants) rather than a single batch `time.Now()`
so ordering is approximately faithful.

### Pitfall 6: tokens_to_model vs tokens_through_daemon (FAIR/Pitfall-5 boundary)
**What goes wrong:** Reporting Helix's MCP-side byte counter as the headline token number.
**Why:** PITFALLS §Pitfall 5 (line 131): the headline `tokens_input/output` MUST come from the
provider `usage` block (claude stream-json `result` event → `CCTapResult.Usage`), not the daemon
meter.
**How to avoid (this phase):** For the scripted CI gate, tokens are legitimately ~0 (no model) —
write them as 0/absent, do not fabricate. For the wired `--agent=claude` path, source tokens from
`CCTapResult.Usage` (populated by `TapCCStream` from the `result` event, tap.go:285-292). Do NOT
wire the headline token columns to any daemon-side counter. Rich token attribution is Phase 79.

## Code Examples

### Validate result.v2.json on write (reuse the Phase 75 compile pattern)
```go
// Source: bench/schema/result.v2_test.go:20-37 (jsonschema/v6 v6.0.2, Draft 2020-12, offline)
c := jsonschema.NewCompiler()
c.DefaultDraft(jsonschema.Draft2020)
doc, _ := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes)) // preserves json.Number
_ = c.AddResource(schemaPath, doc)
sch, _ := c.Compile(schemaPath)
inst, _ := jsonschema.UnmarshalJSON(bytes.NewReader(resultBytes))
if err := sch.Validate(inst); err != nil { /* fail the cell */ }
```

### Source the fairness block from DefaultContract (D-04)
```go
// Source: bench/runners/fairness_contract.go:84-100
fc := runners.DefaultContract            // ModelID, Temperature, MaxTokens, SystemPromptHash, Retry, Cache, Overrides
// result.fairness.overrides[] is the schema's only fairness sub-field (result.v2.schema.json:55-76);
// with no overrides it is an empty array. ModelID/budget go into provenance fields (open additional props).
```

### run_id shape (D-08; eval's actual shape — NOTE the discrepancy)
```go
// Source: cmd/helix-eval/main.go:104 and internal/eval/report/run_metadata.go:68
runID := time.Now().UTC().Format("20060102T150405Z")   // ISO-8601 timestamp ONLY
// NOTE: CONTEXT.md and runner.go comments say "ISO-8601 + git SHA", but the actual eval RunID is
// timestamp-only; the git SHA is captured SEPARATELY into RunMetadata (helixVersion = "git-"+sha[:12],
// run_metadata.go:54) via debug.BuildInfo vcs.revision, NOT concatenated into RunID. The planner may
// reuse the timestamp-only run_id (discretion) and record the git SHA in result provenance.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| eval in-process daemon (`inprocess.go` RunQuick) for quick smoke | subprocess daemon + PID-gated tap (F-07 close-out, commits e7bc406f/ab8ad6cb) | Phase 67 late | bench MUST use the subprocess path for criteria #3/#4 |
| Single-host timestamp merge | (durable) trace_id-based merge | deferred to Phase 79+ | this phase accepts timestamp merge; don't assert exact order |

**Deprecated/outdated:** none relevant. The eval mode names (`baseline/native/semantic/semantic_guarded`)
are eval-specific and must NOT be reused for bench modes (`your_agent_full`, …).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | result.v2 doc with only `schema_version` + open additional props (outcome/tokens/trace-ref) is "schema-valid" per D-04 | Pitfall 2, Code Examples | LOW — schema requires only `schema_version`, additionalProperties open (verified result.v2.schema.json:8); a validate-on-write test removes residual risk |
| A2 | Synthesizing CCTapResult as SessionInit→(AssistantMsg+ToolResult)*→Result is a faithful enough 2-leg shape | Pattern 4 | LOW — matches `TapCCStream` event kinds (tap.go); exact field choice is discretion (D-02 says "same CCTapResult shape") |
| A3 | `--parallel=4` is safe on CI hardware for the single seed task | Pitfall 3 | LOW — one tiny Go task ×4 cells; PITFALLS recommends NumCPU()/2 as the general fence |
| A4 | The seed task's verify step (`go test`) runs hermetically in the cloned repo without network | D-03 fixture | MEDIUM — Go test of a self-contained module is hermetic; ensure go.mod has no external deps |

**If this table is empty:** (it is not) — A1–A4 are low/medium risk; A1/A2 retired by tests.

## Open Questions

1. **`bench` Make target reconciliation (Pitfall 1).**
   - What we know: `bench:` already exists as a microbench target (Makefile:95).
   - What's unclear: rename microbench vs. rename the new full-run target.
   - Recommendation: rename microbench → `bench-micro`; make the new full-run target `bench` to
     satisfy BENCH-05 literally; update `.PHONY` + any callers. Confirm with user if microbench
     callers exist in CI.

2. **Exact MCP edit tool for the seed scripted task (D-03 discretion).**
   - What we know: `replace_in_file` (fuzzy/exact) and structured-edit tools are in `bench-full`.
   - What's unclear: which makes the single failing `go test` pass most robustly.
   - Recommendation: `replace_in_file` with an exact-match anchor (most portable; does not depend on
     LSP being warm for the tiny module). Keep the step list to 1–2 tools to stay ≤30s.

3. **result.v2 provenance field names (outcome, trace_ref, model_id) — schema has no named props.**
   - What we know: schema leaves top-level open; only `schema_version`, `task_id`, `mode`,
     `benchmark`, `run_index`, `tokens_*`, `fairness` are named.
   - What's unclear: canonical key names for outcome / trace-ref so Phase 79 consumes them.
   - Recommendation: choose stable snake_case keys now (`outcome`, `trace_ref`, `model_id`) and
     document them in `bench/BENCH.md` so Phase 79 doesn't rename. Low risk (additive-only).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build helix + helix-bench; seed `go test` verify | ✓ | (host) | none (hard requirement; already used by repo) |
| `helix` binary on PATH | subprocess daemon + forwarder drive | build-time | `go build ./cmd/helix` | integration test SKIPS if absent; bench run must build/locate it (mirror `--helix-bin` flag, helix-eval main.go:93) |
| `claude` CLI | ONLY `--agent=claude` path (D-01, not gating) | not required for CI | — | scripted agent (CI default); `ErrClaudeNotFound` surfaces cleanly (claude.go:18) |
| `git` | baseline commit + `git diff` patch capture | ✓ (typical) | — | `gitInitBaseline` failure is non-fatal (runner.go:117-120) |
| `$ANTHROPIC_API_KEY` | ONLY `--agent=claude` | not for CI | — | scripted path needs no key (D-01) |

**Missing dependencies with no fallback:** none for the scripted CI gate.
**Missing dependencies with fallback:** `claude` CLI + API key — scripted agent is the CI default.

**Build sequencing note:** `bench-quick` must ensure the `helix` daemon binary exists before the
run (the integration test relies on `helix` being on PATH and SKIPS otherwise — Makefile/CI must
`go build ./cmd/helix` first, or the run must resolve/build it, like helix-eval's `--helix-bin`).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` + `github.com/stretchr/testify` (require/assert) + `jsonschema/v6` for schema |
| Config file | none (go test) |
| Quick run command | `go test ./bench/... ./cmd/helix-bench/...` |
| Full suite command | `go test ./...` (per CLAUDE.md: also `go vet ./...`, `gofmt -w .`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-05 | mode `your_agent_full` resolves to profile `bench-full` via MODE.md | unit | `go test ./bench/runners/ -run ModeResolver -x` | ❌ Wave 0 |
| D-04 | result.v2 builder emits a schema-valid doc (outcome+fairness+tokens+trace-ref) | unit | `go test ./bench/runtime/ -run ResultV2Valid -x` | ❌ Wave 0 |
| D-08 | cell key/path layout `<run_id>/<task>/<mode>`; durable vs ephemeral split; preserve-on-failure | unit | `go test ./bench/runtime/ -run CellLayout -x` | ❌ Wave 0 |
| D-02 | synthesize CCTapResult from []StepResult; Merge yields 2-leg trace, Total>=1 | unit | `go test ./bench/runtime/ -run SynthCCTap -x` | ❌ Wave 0 |
| BENCH-04 | subprocess daemon spawn + forwarder drive + PID-gated tap → MergedTrace.ToolCallSummary.Total>=1 | integration | `go test ./bench/runtime/ -run DaemonTap -x` (mirror `daemon_tap_integration_test.go`; SKIP if no `helix`) | ❌ Wave 0 |
| METRIC-06 / criterion #4 | zero PID cross-talk on parallel cells: `RejectedForeignPid==0`, no foreign tools | integration | `go test ./bench/runtime/ -run CrossCell -x` (boot 2 daemons in parallel; PITFALLS line 337) | ❌ Wave 0 |
| D-03 | seed scripted task: real MCP edit makes `go test` pass (verify exit 0) | integration | `go test ./bench/datasets/... -run SeedTaskVerify` OR via full `helix-bench run` | ❌ Wave 0 |
| Criterion #1 | `helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=<one>` ≤30s, schema-valid result | e2e/smoke | timed `helix-bench run …`; assert exit 0 + result.v2 validates | ❌ Wave 0 |
| BENCH-05 / criterion #2 | `make bench-quick` exit 0, ≥1 task succeeds, ≤90s | e2e/smoke | `time make bench-quick` (CI) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/... ./cmd/helix-bench/... && go vet ./...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** full suite green + `make bench-quick` exits 0 ≤90s + the ≤30s single-task smoke + `gofmt -w .` clean before `/gsd-verify-work`.

### Sampling/observability boundary (the "Nyquist rate" of this smoke)
The ≤30s (single task) and ≤90s (`make bench-quick`) budgets are the sampling rate the smoke must
measure. Asserting ONLY exit code aliases real failures — three escapes the smoke MUST also assert:
1. **Zero orphan spans / 2-leg trace** — assert `MergedTrace.ToolCallSummary.Total >= 1` AND that the
   CC leg is present (an exit-code-only smoke would pass even if the agent-tap leg silently dropped).
2. **Zero PID cross-talk** — assert `DaemonTapResult.RejectedForeignPid == 0` and no foreign tool
   names in the merged trace (an exit-code-only smoke would not detect a neighbor's leakage).
3. **result.v2 schema validity** — validate the emitted doc against `result.v2.schema.json` (an
   exit-code-only smoke would pass on a malformed/empty result file).
Without these three, a green smoke could hide a broken tap, leaked spans, or an invalid result.

### Wave 0 Gaps
- [ ] `bench/runners/mode_resolver_test.go` — covers D-05
- [ ] `bench/runtime/result_test.go` — covers D-04 (build + schema-validate)
- [ ] `bench/runtime/cell_test.go` — covers D-08 layout + D-02 CCTap synth (unit)
- [ ] `bench/runtime/daemon_tap_integration_test.go` — covers BENCH-04 (mirror eval's, SKIP if no `helix`)
- [ ] `bench/runtime/cross_cell_test.go` — covers METRIC-06/criterion #4 (parallel PID cross-talk)
- [ ] `cmd/helix-bench/run_cmd_test.go` — covers flag plumbing + exit semantics (mirror `cmd/helix-eval/run_cmd_test.go`)
- [ ] Seed fixture under `bench/datasets/toolbench-go/<task>/` with a verify-passing scripted edit
- [ ] Framework install: none — `testing` + `testify` + `jsonschema/v6` all present

## Security Domain

> `security_enforcement` is not set false in config; ASVS applies but this is internal benchmark
> tooling with no network surface in the gating path. The relevant controls are the inherited
> Phase 67 hardening (path traversal + symlink rejection), which carry forward via the sandbox embed.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | `validateTaskID` (runner.go:65) rejects path-traversal task IDs; reuse for bench task IDs. `LoadScript` strict YAML (KnownFields). |
| V12 File/Resources | yes | sandbox lstat-symlink reject + 0700 (sandbox.go:59-81); patch path-prefix invariant in `Merge` (T-67-02, merge.go:124). |
| V6 Cryptography | no | none (no secrets generated/stored by the gating path) |
| V2 Auth / V3 Session | no | local subprocess over Unix socket; no auth surface |

### Known Threat Patterns for {Go subprocess bench harness}
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malicious task ID escapes join root (`../../`) | Tampering | `validateTaskID` (runner.go:65) — reuse verbatim for bench task/benchmark/mode names |
| Symlink at predictable tmpdir | Tampering | `checkNotSymlink` lstat (sandbox.go:72) — inherited via embed |
| Patch writes outside repo | Tampering | `Merge` path-prefix check (merge.go:124, T-67-02) — pass `PatchPaths`+`RepoRoot` |
| Foreign-PID log injection into trace | Spoofing | `TapDaemonLog` PID gate (tap.go:71, T-67-04) — pass the captured daemonPID |
| Dev credentials leaking into agent env | Info disclosure | env allowlist in `StartDaemon` (sandbox.go:247) + `agent.cleanEnv` (claude.go:62) |

## Sources

### Primary (HIGH confidence) — read directly this session
- `internal/eval/sandbox/sandbox.go` — Sandbox, StartDaemon, path helpers, hardening, DaemonHandle
- `internal/eval/runner/runner.go` — RunTask/RunMatrix dispatch, daemon-tap wiring, outcome resolution, validateTaskID
- `internal/eval/runner/daemon_tap_integration_test.go` — the spawn→drive→kill→tap→merge template
- `internal/eval/runner/scripted_agent.go` — ScriptedStep/Script/LoadScript/ScriptedAgent/MCPCaller/StepResult
- `internal/eval/runner/inprocess.go` — RunQuick (anti-pattern for smoke; reuse scripted loop + sessionCaller only)
- `internal/eval/trace/{merge,tap,schema}.go` — Merge/MergeInput, TapDaemonLog/DaemonTapResult/CCTapResult, MergedTrace/Event/Usage
- `internal/eval/agent/{claude,mcpconfig}.go` — Agent.Run argv, ErrClaudeNotFound, WriteMCPConfig
- `bench/schema/result.v2.schema.json` + `result.v2_test.go` + `testdata/result.v2.golden.json` — required field + validation pattern
- `bench/runners/fairness_contract.go` — DefaultContract shape
- `cmd/helix-bench/main.go` (notYetImplemented stub line 89) + `cmd/helix-eval/main.go` (run analog)
- `internal/profile/profiles/bench-full.yaml` (name: bench-full confirmed)
- `internal/cli/root.go:68` (daemon `--profile` flag)
- `Makefile` (bench:95, bench-baseline:98, eval-quick:230, eval:240; `.PHONY` line 1)
- `eval/fixtures/quick-large-edit-001/*` (fixture layout: repo/, scripted_agent.yaml, verify.sh, task.md, budget.yaml)
- `.planning/REQUIREMENTS.md` (BENCH-04 line 20, BENCH-05 line 21, METRIC-06)
- `.planning/research/PITFALLS.md` (F-07 PID cross-talk line 330-345; tokens line 131-140; fairness line 92)
- `.planning/milestones/v1.12-ROADMAP.md` §Phase 77 (4 success criteria)
- `go.mod` (jsonschema/v6 v6.0.2, yaml.v3 v3.0.1, cobra v1.9.1)

### Secondary (MEDIUM confidence)
- none (no web/external sources needed — pure internal-codebase reuse phase)

### Tertiary (LOW confidence)
- none

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps; all reused symbols read from source with file:line
- Architecture: HIGH — the daemon-tap integration test IS the spine, copied not paraphrased
- Pitfalls: HIGH — F-07/token/clock pitfalls grounded in PITFALLS.md + verified in merge/tap source;
  the `make bench` collision found by reading the Makefile

**Research date:** 2026-06-17
**Valid until:** 2026-07-17 (stable internal codebase; revalidate if `internal/eval/` is refactored
or `bench/schema/result.v2.schema.json` adds required fields)
