# Phase 77: Bench Runtime & First E2E Smoke - Pattern Map

**Mapped:** 2026-06-17
**Files analyzed:** 11 new + 2 modified
**Analogs found:** 12 / 13 (1 builder has schema/golden only, no Go analog)

> This phase is **reuse, not invention**. Every new file mirrors an existing
> `internal/eval/` analog except the `result.v2.json` builder (no Go writer exists
> today). The 77-RESEARCH.md already cites every reused symbol with `file:line`;
> this PATTERNS.md pins the *concrete excerpt* the executor copies per file and
> flags each divergence from its analog.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/runtime/sandbox/sandbox.go` (NEW) | wrapper (struct embed) | file-I/O | `internal/eval/sandbox/sandbox.go` | exact (embed) |
| `bench/runtime/subprocess/daemon.go` (NEW) | lifecycle | event-driven (spawn/kill) | `internal/eval/sandbox/sandbox.go:231` `StartDaemon` + `:193` `Kill` | exact |
| `bench/runtime/cell.go` (NEW, runner glue) | orchestration | request-response + transform | `internal/eval/runner/daemon_tap_integration_test.go:50-104` (spine) + `runner.go` `RunTask` | exact (spine) |
| `bench/runtime/drive.go` (NEW, forwarder drive) | lifecycle | streaming (NDJSON) | `daemon_tap_integration_test.go:106-199` `driveSingleToolCall` | exact |
| `bench/runtime/cctap.go` (NEW, CC-tap synth) | transform | transform | `internal/eval/trace/tap.go:136-146` `CCTapResult` + `schema.go` `Event` | role-match (new glue) |
| `bench/runtime/result.go` (NEW, result.v2 builder) | builder | transform | `bench/schema/result.v2_test.go` + `testdata/result.v2.golden.json` + `bench/runners/fairness_contract.go` | partial (schema/golden only — NO Go builder) |
| `bench/runners/mode_resolver.go` (NEW) | resolver | transform (YAML→string) | `internal/eval/runner/scripted_agent.go:38-53` `LoadScript` (yaml.v3 strict pattern) | role-match |
| `bench/runners/your_agent_full/MODE.md` (NEW) | config | — | (none — minimal frontmatter, D-05 discretion) | no analog |
| `bench/datasets/toolbench-go/<task>/` (NEW fixture) | fixture | file-I/O | `eval/fixtures/quick-large-edit-001/*` | exact |
| `cmd/helix-bench/run_cmd_test.go` (NEW) | test | — | `cmd/helix-eval/run_cmd_test.go` | exact |
| `cmd/helix-bench/main.go` (MODIFIED) | cmd | request-response | `cmd/helix-eval/main.go:58-160` run subcommand | exact |
| root `Makefile` (MODIFIED) | build | — | `Makefile:230` `eval-quick` / `:240` `eval` | exact (BUT `bench:` name collision — see Shared) |

## Pattern Assignments

### `bench/runtime/sandbox/sandbox.go` (wrapper, struct embed) — D-07

**Analog:** `internal/eval/sandbox/sandbox.go`

**Pattern: EMBED, do not fork.** The bench sandbox wraps `eval/sandbox.Sandbox`
and adds only bench-specific durable paths. All hardening (lstat-symlink reject,
0700, macOS socket-length handling, `/tmp` short-root) is inherited verbatim.

Reused methods (already exist on the embedded type — do NOT reimplement):
- `NewSandbox(runID, helixBin string) (*Sandbox, error)` (sandbox.go:39)
- `Prepare(taskID, mode string) error` (sandbox.go:110)
- `CloneRepo(srcRepo, taskID, mode string) error` (sandbox.go:126)
- `StartDaemon(ctx, taskID, mode, profileName, cfgPath string) (*DaemonHandle, error)` (sandbox.go:231)
- `ModeDir`/`HomeFor`/`RepoFor`/`SocketFor`/`McpConfigPath` (sandbox.go:84-106)
- `Cleanup() error` (sandbox.go:291)

**Excerpt to mirror — embed + add bench paths:**
```go
package sandbox

import evalsandbox "github.com/agenthands/helix/internal/eval/sandbox"

// Sandbox embeds the eval sandbox (D-07: thin-wrap, never fork) and adds the
// bench-specific DURABLE artifact paths. Ephemeral scratch (HOME, repo, socket)
// stays in the embedded eval sandbox under /tmp; durable artifacts live under outDir.
type Sandbox struct {
	*evalsandbox.Sandbox
	outDir string // durable out dir: bench/reports/<run_id>/<task>/<mode>/  (D-08)
}

// ResultPath / MergedTracePath are the only NEW path helpers (D-04/D-08 durable side).
func (s *Sandbox) ResultPath(task, mode string) string      { /* filepath.Join(s.outDir, ...) result.v2.json */ }
func (s *Sandbox) MergedTracePath(task, mode string) string { /* ... trace.json */ }
```

**Divergence:** eval's `Cleanup()` always `RemoveAll(Root)`. D-08 requires
**delete on success, PRESERVE on failure** — so the cell orchestrator must call
`Cleanup()` only on the success path and skip it (logging the preserved temp dir)
on failure. Do NOT change eval's `Cleanup`; gate the call site in `cell.go`.

---

### `bench/runtime/subprocess/daemon.go` (lifecycle) — D-06/D-07

**Analog:** `internal/eval/sandbox/sandbox.go:231` `StartDaemon` + `:193` `Kill` + `:185` `Pid`

**Daemon argv (copy verbatim — this is the no-TCP-port mechanism, D-06):**
```go
// sandbox.go:236-242 — HTTP disabled (empty --http-addr), per-cell Unix socket
args := []string{"--serve", "--socket=" + sockPath, "--http-addr=", "--json"}
if profileName != "" { args = append(args, "--profile="+profileName) }
if cfgPath != ""     { args = append(args, "--config="+cfgPath) }
// env allowlist (sandbox.go:247-254): HOME=<HomeFor>, HELIX_LOG_LEVEL=info, PATH (inherited)
// stdout+stderr → <modeDir>/daemon.log (sandbox.go:260-268); polls socket up to 10s (sandbox.go:275)
```

**PID-capture ordering (criterion #4 — capture BEFORE Kill):**
```go
h, _ := sb.StartDaemon(ctx, taskID, mode, profileName, cfgPath)
daemonPID := h.Pid()   // sandbox.go:185 — capture NOW; Pid() may return 0 after Wait()
// ... drive agent ...
_ = h.Kill()           // sandbox.go:193 — flushes slog buffers to daemon.log
// THEN tap with daemonPID
```

**Divergence:** subprocess pkg also OWNS the eventual `claude` spawn
(`agent.Agent.Run`, wired-not-gating, D-01). For Phase 77 CI gate only the daemon
path is exercised; the claude branch is wired behind `--agent=claude`.

---

### `bench/runtime/cell.go` (orchestration spine) — BENCH-04, the whole loop

**Analog:** `internal/eval/runner/daemon_tap_integration_test.go:50-104` — this IS
the spawn→drive→kill→tap→merge spine, copy-paste quality. Generalize to one cell.

**Spine excerpt to mirror (integration_test:65-104):**
```go
sockPath := sb.SocketFor(taskID, mode)
// drive (scripted: iterate scripted_agent.yaml steps; one tools/call frame each)
_ = driveToolCall(ctx, helixBin, sockPath, tool, argsJSON)

preKillPid := h.Pid()              // capture BEFORE Kill (line 73)
require.NoError(t, h.Kill())       // line 74 — flush slog
daemonLog := filepath.Join(sb.ModeDir(taskID, mode), "daemon.log")
tap, _ := trace.TapDaemonLog(daemonLog, preKillPid)   // line 79 — PID-gated (tap.go:44)

merged, _ := trace.Merge(trace.MergeInput{            // line 93 (merge.go:13)
	TaskID: taskID, Mode: mode, RunID: runID,
	StartedAt: start, EndedAt: time.Now(),
	Daemon: tap,
	CC:     synthesizedCCTap,   // D-02 — the 2nd leg (NEW, see cctap.go)
	VerifyExitCode: verifyExit, // D-04 — outcome source
	RepoRoot: sb.RepoFor(taskID, mode),
})
// assert (sampling-rate guards, RESEARCH §"Nyquist"):
//   merged.ToolCallSummary.Total >= 1  (line 102)
//   tap.RejectedForeignPid == 0        (criterion #4)
```

**Divergences from the integration test:**
1. Test uses `get_health` (side-effect-free); the cell drives the **seed task's
   real edit tool** from `scripted_agent.yaml` then runs `go test` as `verify`.
2. Test passes only `Daemon:`; the cell ALSO passes `CC:` (synth) + `VerifyExitCode:`
   + `RepoRoot:` to get a 2-leg merge and a real outcome (D-02/D-04).
3. Add the result.v2 build + schema-validate + durable-write steps after `Merge`.

---

### `bench/runtime/drive.go` (forwarder drive) — D-06 transport

**Analog:** `daemon_tap_integration_test.go:106-199` `driveSingleToolCall` — copy
nearly verbatim; parameterize `tool` + `argsJSON` and loop over scripted steps.

**Critical NDJSON excerpt (integration_test:117-152):**
```go
cmd := exec.CommandContext(fwdCtx, helixBin, "--mode=stdio", "--socket="+sockPath)
cmd.Env = append(os.Environ(), "HELIX_LOG_LEVEL=info")
stdin, _ := cmd.StdinPipe(); stdout, _ := cmd.StdoutPipe()
// DO NOT close stdin between frames or before responses arrive — closing races
// the forwarder's CloseSend against response delivery (integration_test:139-143).
initFrame := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bench","version":"0"}}}`
callFrame := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, tool, argsJSON)
fmt.Fprintln(stdin, initFrame)
fmt.Fprintln(stdin, callFrame)
// read responses on a goroutine with a deadline (integration_test:154-198)
```

**Divergence:** integration test hard-codes `"arguments":{}`; bench passes the
step's `Args` map as `argsJSON`. Record each call as a `runner.StepResult`
(`{Tool, AtTime, Response, Err}`, scripted_agent.go:55-65) to feed `cctap.go`.

---

### `bench/runtime/cctap.go` (CC-tap synth) — D-02, the genuinely-new glue

**Analog:** target type `internal/eval/trace/tap.go:136-146` `CCTapResult` +
event kinds in `internal/eval/trace/schema.go`. No direct synth analog exists;
mirror the shape `TapCCStream` produces so the leg looks identical to a claude run.

**Target type (tap.go:136-146):**
```go
type CCTapResult struct {
	Events         []Event   // BUILD these from []runner.StepResult
	Usage          Usage     // scripted = zero (D-01); claude = from result event (Pitfall 6)
	UnknownTypes   int
	FinalSessionID string
}
```

**Synthesis recipe (RESEARCH Pattern 4):** one `Source:"cc", Kind:KindSessionInit`;
per step a `Kind:KindAssistantMsg` carrying `ToolUse{Name: step.Tool, Input: step.Args}`
then `Kind:KindToolResult{ToolUseID, IsError: step.Err!=nil}`; finally `Kind:KindResult`
with zero `Usage`. Use `step.AtTime` (real dispatch instants, scripted_agent.go:59)
for event timestamps — NOT a single batch `time.Now()` (Pitfall 5).

**Key invariant:** `Merge` builds `ToolCallSummary` ONLY from `Source:"daemon"`
`KindToolCall` events (merge.go:97-99) — the CC leg adds 2nd-leg continuity
**without** double-counting tool calls. Scripted tokens are legitimately 0; do
NOT fabricate (Pitfall 6).

---

### `bench/runtime/result.go` (result.v2 builder) — D-04 (NO Go analog)

**Analog:** `bench/schema/result.v2_test.go` (validation pattern) +
`bench/schema/testdata/result.v2.golden.json` (shape) +
`bench/runners/fairness_contract.go:84` `DefaultContract` (fairness block).
**Pitfall 2: NO Go builder/type exists — this file is genuinely new.**

**Schema facts (drive the builder):** the ONLY required field is `schema_version`;
top-level `additionalProperties` is OPEN. Named props: `schema_version`, `task_id`,
`mode`, `benchmark`, `run_index`, `tokens_*`, `fairness`. `outcome` / `trace_ref` /
`model_id` are NOT named props — they land as open additional props (valid).

**Fairness block source (fairness_contract.go:84-100):**
```go
fc := runners.DefaultContract  // ModelID "claude-sonnet-4-5-20260128", Temperature 0.0,
                               // MaxTokens 8192, SystemPromptHash, Retry, Cache, Overrides{}
// schema's only fairness sub-field is overrides[] (empty array when no overrides)
```

**Validate-on-write (copy result.v2_test.go:20-37 — jsonschema/v6 v6.0.2, offline):**
```go
c := jsonschema.NewCompiler()
c.DefaultDraft(jsonschema.Draft2020)
doc, _ := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))   // preserves json.Number
_ = c.AddResource(schemaPath, doc)
sch, _ := c.Compile(schemaPath)
inst, _ := jsonschema.UnmarshalJSON(bytes.NewReader(resultBytes))
if err := sch.Validate(inst); err != nil { /* fail the cell */ }
```

**Fields to populate (D-04, provenance-complete / metric-sparse):** `schema_version:"v2"`,
`task_id`, `mode`, `benchmark`, `run_index`, outcome (from `merged.Outcome`/VerifyExitCode),
tokens (0 for scripted), fairness (from `DefaultContract`), `trace_ref` (path to trace.json).
Rich metrics (`edit_locality`, `regression_rate`, pass@k) **left absent** (Phase 79).
Document chosen key names (`outcome`, `trace_ref`, `model_id`) in `bench/BENCH.md` so
Phase 79 doesn't rename (Open Question 3).

---

### `bench/runners/mode_resolver.go` (resolver) — D-05

**Analog:** `internal/eval/runner/scripted_agent.go:38-53` `LoadScript` (the strict
yaml.v3 decode pattern). Do NOT reuse eval's `profileForMode` (runner.go:434) — it
hard-codes eval modes (`baseline/native/...`), an anti-pattern for bench.

**Pattern:** table-driven scan of `bench/runners/*/MODE.md`, parse frontmatter
`profile:` key. Map `your_agent_full` → `bench-full`, then call
`StartDaemon(ctx, task, "your_agent_full", "bench-full", cfgPath)` — `StartDaemon`
already separates the `mode` label (per-cell dir/socket) from `profileName`
(`--profile=` value, sandbox.go:237-239). Make it extensible so Phase 80 drops in
5 more dirs with no code change.

**YAML decode shape (mirror scripted_agent.go:46-52):**
```go
dec := yaml.NewDecoder(bytes.NewReader(frontmatterBytes))
dec.KnownFields(true)   // strict — unknown keys are a hard error
```
`bench-full` profile confirmed present at `internal/profile/profiles/bench-full.yaml`.

---

### `bench/runners/your_agent_full/MODE.md` (config) — D-05, no analog

Minimal frontmatter only (discretion; Phase 80 owns full ABLATE-01 convention):
```markdown
---
mode: your_agent_full
profile: bench-full
---
```

---

### `bench/datasets/toolbench-go/<task>/` (fixture) — D-03

**Analog:** `eval/fixtures/quick-large-edit-001/*` — exact layout to mirror:
`repo/`, `scripted_agent.yaml`, `verify.sh`, `task.md`, `budget.yaml`.

**Scripted YAML shape (eval/fixtures/quick-large-edit-001/scripted_agent.yaml):**
```yaml
steps:
  - tool: replace_symbol_body     # bench: prefer replace_in_file w/ exact anchor (Open Q2 — no warm LSP needed)
    args:
      symbol: Process
      body: |
        func Process(x int) int {
        	return x * 2
        }
```

**Divergences for the bench seed:** (1) `repo/` is a **self-contained Go module
with one FAILING test** (go.mod with NO external deps — hermetic verify, Assumption A4);
(2) add `task.json` (bench prompt/metadata, vs eval's `task.md`); (3) `verify`
step runs `go test` and its exit code feeds `MergeInput.VerifyExitCode` (D-04).
ONE task only — corpus is Phase 78.

---

### `cmd/helix-bench/main.go` (MODIFIED) — replace stub at line 89

**Analog:** `cmd/helix-eval/main.go:58-160` (run subcommand + flag plumbing + RunE
single-exit pattern).

**Current state to replace (main.go:85-91):**
```go
func newRunCmd() *cobra.Command {
	return &cobra.Command{Use: "run", Short: "Run the bench suite",
		RunE: notYetImplemented("run")}   // ← replace this
}
```

**Flag plumbing to mirror (helix-eval main.go:60-93 var block + Flags()):**
```go
cmd.Flags().StringVar(&benchmarks, "benchmarks", "toolbench-go", "benchmark suite(s)")
cmd.Flags().StringArrayVar(&modes, "modes", []string{"your_agent_full"}, "mode(s); repeatable")
cmd.Flags().StringArrayVar(&tasks, "tasks", nil, "task id(s); repeatable")
cmd.Flags().IntVar(&parallel, "parallel", 1, "max concurrent cells")
cmd.Flags().StringVar(&out, "out", "bench/reports", "durable output dir")
cmd.Flags().StringVar(&agent, "agent", "scripted", "agent driver: scripted|claude")
cmd.Flags().StringVar(&helixBin, "helix-bin", "helix", "path to helix binary (mirror helix-eval main.go:93)")
```

**Exit semantics (mirror helix-eval main.go:31-36 + :158-160):** `main()` is the
only `os.Exit` site; RunE returns an error → exit 1. Exit 0 iff ≥1 task succeeded.
Default `run-id` = `time.Now().UTC().Format("20060102T150405Z")` (main.go:104;
git SHA captured separately in provenance, RESEARCH run_id note).

**Divergence:** helix-bench `main.go:30` has a special-case dispatch for `verify-tos`
BEFORE `newRootCmd()` — leave it intact; only swap `newRunCmd`'s body.

---

### `cmd/helix-bench/run_cmd_test.go` (NEW) — flag plumbing + exit semantics

**Analog:** `cmd/helix-eval/run_cmd_test.go:16-79` `TestRunSubcommandWiresThemAll`.

**Pattern to mirror:** build a minimal synthetic fixture in `t.TempDir()`, invoke
via cobra `root.SetArgs([]string{"run", "--benchmarks", ..., "--out", tmp, ...})`,
tolerate a missing `helix` binary (`_ = err`), assert the durable out dir +
expected artifacts (`result.v2.json`) appear best-effort. Mirrors helix-eval's
"helix binary may not exist; check artifacts were created" tolerance (run_cmd_test.go:50-52).

## Shared Patterns

### PID-gated daemon-tap (criterion #4)
**Source:** `internal/eval/trace/tap.go:44` `TapDaemonLog(path string, expectedPid int) (DaemonTapResult, error)`
**Apply to:** `cell.go`, `cross_cell_test.go`
Always `Kill()` the daemon BEFORE tapping (slog flush); capture `preKillPid` before
`Kill()`. Assert `DaemonTapResult.RejectedForeignPid == 0` per cell. Per-cell
`daemon.log` path (`<modeDir>/daemon.log`) makes parallel cells isolated by construction.

### Trace merge + outcome resolution
**Source:** `internal/eval/trace/merge.go:13-42` `Merge(MergeInput)` ; outcome priority
`Budget breach > VerifyExitCode!=0 → "failed" > "success"` (merge.go:141-149).
**Apply to:** `cell.go`. NOTE `MergedTrace.SchemaVersion` is the **trace** schema
("1"), distinct from result.v2's `schema_version:"v2"`.

### Fairness contract (FAIR-02 pin)
**Source:** `bench/runners/fairness_contract.go:84` `DefaultContract`
**Apply to:** `result.go`. Feed `Overrides{}` → empty `fairness.overrides[]`;
`ModelID`/`MaxTokens` go into open provenance props.

### Task-ID input validation (V5/V12)
**Source:** `internal/eval/runner/runner.go:65` `validateTaskID` (reject `../`)
**Apply to:** the matrix expander before joining task/benchmark/mode into paths.

### Makefile target collision (BLOCKER — BENCH-05)
**Source:** `Makefile:95` `bench:` ALREADY exists (`go test -bench=. ./test/bench/...`),
listed in `.PHONY` (line 1). Eval analog targets: `Makefile:230` `eval-quick:`
(`go run ./cmd/helix-eval run --quick ...`) and `:240` `eval:`.
**Apply to:** `Makefile`. Planner MUST reconcile explicitly (RESEARCH Pitfall 1 /
Open Q1): recommended rename microbench → `bench-micro`; new full-run `bench:`
invokes `go run ./cmd/helix-bench run --benchmarks=...`; add `bench-quick:` (hermetic
scripted `your_agent_full` on the seed task, ≤90s) and `bench-<suite>:`. Update
`.PHONY`. **Build sequencing:** `bench-quick` must `go build ./cmd/helix` first (the
forwarder-drive path needs `helix` on PATH or it SKIPs).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `bench/runtime/result.go` | builder | transform | NO Go result.v2 builder exists — only schema + golden + validation test (Pitfall 2). Build a new struct/map with `json` tags + reuse the `result.v2_test.go` validate pattern. |
| `bench/runners/your_agent_full/MODE.md` | config | — | New convention (D-05); minimal `mode`+`profile` frontmatter, full ABLATE-01 is Phase 80. |

## Metadata

**Analog search scope:** `internal/eval/{sandbox,runner,trace,agent}/`, `bench/{schema,runners}/`,
`cmd/helix-eval/`, `cmd/helix-bench/`, `eval/fixtures/`, `Makefile`, `internal/profile/profiles/`
**Files read this session:** `internal/eval/sandbox/sandbox.go` (1-300), `cmd/helix-bench/main.go`,
`cmd/helix-eval/main.go` (1-160), `internal/eval/runner/daemon_tap_integration_test.go` (60-199),
`internal/eval/runner/scripted_agent.go` (1-120), `bench/runners/fairness_contract.go` (70-110),
`cmd/helix-eval/run_cmd_test.go`, `eval/fixtures/quick-large-edit-001/*`, `Makefile` (93-104, 228-249)
**Pattern extraction date:** 2026-06-17
