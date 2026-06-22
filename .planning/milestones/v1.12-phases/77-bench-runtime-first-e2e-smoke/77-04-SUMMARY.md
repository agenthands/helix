---
phase: 77-bench-runtime-first-e2e-smoke
plan: 04
subsystem: bench-runtime-cli
tags: [bench, cli, matrix, run-subcommand, parallel, claude-wired-not-gating, BENCH-04, BENCH-05]
requires:
  - bench/runtime.RunCell / CellConfig / CellResult (Plan 03 — per-cell spine the matrix dispatches)
  - bench/runtime.driveScript (Plan 03 — scripted forwarder drive)
  - bench/runners.ResolveProfile (Plan 01 — mode->profile, via RunCell)
  - bench/runners.DefaultContract (Plan 01/75 — fairness block, via RunCell)
  - bench/datasets/toolbench-go/sum-doubler (Plan 01 — seed task)
  - internal/eval/agent.Agent / Run / WriteMCPConfig / ErrClaudeNotFound (real claude branch)
  - internal/eval/runner.StepResult (scripted step results threaded through RunCell)
provides:
  - bench/runtime.ExpandMatrix (benchmark x mode x task cartesian product, V5-validated)
  - bench/runtime.RunMatrix / RunMatrixConfig / Cell / CellOutcome / Summary (--parallel-bounded dispatch)
  - bench/runtime/subprocess.StartClaude / ClaudeConfig / ErrClaudeNotFound (D-01 wired-not-gating claude branch)
  - cmd/helix-bench run subcommand (the operator-facing surface criterion #1 grades)
affects:
  - Plan 05 (Makefile bench-quick / bench / bench-<suite> invoke `helix-bench run --benchmarks=...`)
  - Phase 78 (corpus tasks dispatched via the same ExpandMatrix/RunMatrix; --tasks discovery scans dataset dirs)
  - Phase 80 (additional modes drop into --modes; the mode resolver already extends without code change)
tech-stack:
  added: []
  patterns:
    - cartesian matrix expansion with per-axis path-traversal rejection before any join (V5/T-77-10)
    - sem-channel-bounded dispatcher factored out (dispatch) so the --parallel bound is unit-tested without daemons
    - no TCP ports => no --parallel collisions by construction (D-06, inherited from RunCell)
    - cobra single-exit: RunE returns error iff zero cells succeeded (helix-eval semantics)
    - timestamp-only run_id (20060102T150405Z); git SHA stays in result provenance, not concatenated
    - --tasks discovery defaults to all dataset task dirs (criterion #1's --tasks=<one> still supported)
    - agent drive branch in RunCell (scripted CI gate vs claude wired-not-gating); spine otherwise identical
    - re-exported ErrClaudeNotFound sentinel so bench callers errors.Is-match without importing eval agent
key-files:
  created:
    - bench/runtime/matrix.go
    - bench/runtime/matrix_test.go
    - bench/runtime/subprocess/claude.go
    - cmd/helix-bench/run_cmd_test.go
  modified:
    - cmd/helix-bench/main.go
    - bench/runtime/cell.go
decisions:
  - "RunMatrix is split into a public wrapper + an internal dispatch(run func) so the parallel-bound invariant (T-77-12) is asserted with an injected counting runner — no real daemon needed to prove the bound"
  - "The --agent branch lives in RunCell's drive step (CellConfig.Agent), NOT a separate claude orchestrator: scripted and claude share the entire sandbox/daemon/tap/merge/result spine; only the drive leg differs. This wires claude without re-implementing the Plan 03 spine"
  - "run_id is timestamp-only per RESEARCH (NOT ISO-8601+gitSHA); git SHA is captured separately into result provenance. The out dir is run-scoped (<out>/<run_id>/) matching helix-eval"
  - "--tasks omitted defaults to ALL task dirs discovered under <datasets>/<benchmark>/ (sorted); an empty/absent benchmark dir is a hard error, not a silent exit-0 no-op"
  - "subprocess.StartClaude requires an absolute --helix-bin (WriteMCPConfig T-67-03) and reuses agent.Agent's argv+cleanEnv allowlist verbatim (T-77-11); it returns the re-exported ErrClaudeNotFound sentinel cleanly when claude is absent"
metrics:
  duration_seconds: 455
  completed: 2026-06-17
  tasks: 2
  files: 6
---

# Phase 77 Plan 04: helix-bench run CLI Surface Summary

The operator-facing surface criterion #1 grades: `helix-bench run` now expands the
`(benchmark x mode x task)` matrix, dispatches each cell through the Plan 03
`RunCell` spine bounded by `--parallel`, and applies helix-eval's single-exit
semantics (exit 0 iff ≥1 cell succeeded). The `--agent=claude` branch is wired and
locally runnable (D-01) but never on the scripted CI path. Verified end-to-end:
`helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=sum-doubler`
ran the seed cell in ~1s, exited 0, and wrote a schema-valid `result.v2.json`.

## What Was Built

### Task 1 — `matrix.go` + `matrix_test.go` + `subprocess/claude.go` (commit 91cf42bc)
- **`ExpandMatrix(benchmarks, modes, tasks) ([]Cell, error)`** — deterministic
  cartesian product (benchmark-outer, task-inner). Every id in every axis is
  validated against path traversal (`validateMatrixID`: reject `../`, separators,
  leading dot, absolute) BEFORE it can become a path segment (V5/T-77-10). Empty
  axes are a hard error.
- **`RunMatrix(ctx, cells, parallel, cfg) (Summary, error)`** — the public
  dispatcher, factored over an internal `dispatch(run func)` that runs cells under
  a `sem`-channel bounded by `parallel` (mirrors `internal/eval/runner.go:304`).
  No TCP ports are introduced, so "no collisions on `--parallel=N`" holds by
  construction (D-06, inherited from RunCell). `Summary.Succeeded` (≥1 success)
  drives the process exit.
- **`subprocess.StartClaude` + `ClaudeConfig`** — the D-01 wired-not-gating claude
  branch, a thin delegation to `internal/eval/agent.Agent.Run` (reusing its argv,
  `cleanEnv` credential allowlist T-77-11, and `WriteMCPConfig` verbatim). Returns
  the re-exported `ErrClaudeNotFound` sentinel cleanly (no panic) when claude is
  absent; requires an absolute `--helix-bin` (WriteMCPConfig T-67-03).
- **`matrix_test.go`** — cartesian count + no-duplicates + deterministic ordering;
  path-traversal rejection across all three axes; empty-axis guard; the
  parallel-bound is never exceeded (a barrier holds the first `parallel`
  goroutines and a counting runner records peak concurrency); success aggregation;
  nil-context guard. The bound test passes under `-race`.

### Task 2 — `cmd/helix-bench/main.go` run subcommand + `run_cmd_test.go` (commit 6878ba7d)
- **`newRunCmd`** replaces `notYetImplemented("run")` (the stub at the old line 89
  is gone; fetch-datasets/report still report not-yet-implemented — scope
  boundary). Flags: `--benchmarks` (default `toolbench-go`), `--modes`
  (`[your_agent_full]`), `--tasks` (nil → discover), `--parallel` (1), `--out`
  (`bench/reports`), `--agent` (`scripted`), `--helix-bin` (`helix`), plus
  `--run-id` and `--datasets`.
- **`runBench`** — timestamp-only `run_id` (git SHA stays in result provenance,
  RESEARCH run_id note); `--agent` validated to `scripted|claude`; `--tasks`
  defaults to all dataset task dirs via `discoverTasks` (criterion #1's
  `--tasks=<one>` still works by passing one id); run-scoped out dir
  `<out>/<run_id>/`; `ExpandMatrix` → `RunMatrix`; per-cell infra errors surfaced
  to stderr (non-fatal); single-exit returns an error iff zero cells succeeded.
  The `main()` verify-tos dispatch and the other 4 subcommands are untouched.
- **`cell.go` (modified)** — RunCell's drive step now branches on
  `CellConfig.Agent`: `""`/`scripted` → `driveScript` (unchanged CI gate);
  `claude` → `subprocess.StartClaude` (D-01); unknown agent → error. The rest of
  the spine (sandbox/daemon/PID-gated tap/2-leg merge/result build) is identical
  for both — this wires claude without re-implementing the Plan 03 spine. Added a
  `claudeMaxToolCalls` bound and a `Prompt` field.
- **`matrix.go` (modified)** — `runOneCell` threads `Agent` into RunCell and loads
  the task.json `prompt` (only when `claude` is selected) via `readTaskPrompt`.
- **`run_cmd_test.go`** — flag-registration assertion (all 9 flags), a
  synthetic-fixture `run` invocation tolerating a missing helix binary
  (best-effort artifact assertion, mirrors helix-eval), unknown-agent rejection,
  empty-benchmark-dir rejection, and a run-stub-removal guard.

## E2E Smoke: EXERCISED (not just unit-tested)

Built helix + helix-bench and ran the criterion #1 command against the seed task:

```
helix-bench run --benchmarks=toolbench-go --modes=your_agent_full \
  --tasks=sum-doubler --helix-bin=<built helix> --out=/tmp/bench-smoke-out
→ "helix-bench run complete: 1/1 cells succeeded"   exit 0, ELAPSED ~1s (≤30s budget)
```

Durable `result.v2.json` written and inspected — schema-valid, provenance-complete
/ metric-sparse (D-04):
`{schema_version:"v2", task_id:"sum-doubler", mode:"your_agent_full",
benchmark:"toolbench-go", run_index:0, tokens_input:0, tokens_output:0,
fairness:{overrides:[]}, outcome:"success",
trace_ref:".../trace.json", model_id:"claude-sonnet-4-5-20260128"}`. `trace.json`
also present.

The `--agent=claude` branch was exercised with claude absent from PATH: it
surfaced `bench/runtime: drive claude: claude CLI not found on PATH` cleanly
(no panic), the cell preserved its scratch (D-08), and the run exited 1 — wired,
locally runnable, NOT the scripted/CI default (D-01).

## How to Verify

- Build the daemon: `go build -o /tmp/helix/helix ./cmd/helix`.
- Unit (matrix + cmd): `go test ./bench/runtime/ -run Matrix -count=1` and
  `go test ./cmd/helix-bench/ -count=1` — pass. Bound test also under `-race`.
- Full bench tree (integration RUN): `HELIX_BIN=/tmp/helix/helix go test ./bench/... ./cmd/helix-bench/... -count=1` — all green.
- E2E (criterion #1): build helix-bench, then
  `helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=sum-doubler --helix-bin=/tmp/helix/helix --out=<tmp>`
  → 1/1 cells succeeded, exit 0, schema-valid result.v2.json under `<tmp>/<run_id>/sum-doubler/your_agent_full/`.
- `helix-bench run --help` lists all seven+ flags.
- `go build ./...`, `go vet ./bench/... ./cmd/helix-bench/...` clean; `gofmt -l` empty on all six touched files.

## Deviations from Plan

### Auto-fixed / In-scope wiring adjustments

**1. [Rule 3 - In-scope wiring] `--agent=claude` branch placed in RunCell's drive step, not the subprocess package alone**
- **Found during:** Task 2 — the plan wires claude via `subprocess/claude.go` and
  requires `--agent=claude` to be reachable through `RunCell`, but Plan 03's
  `RunCell` drove only the scripted path.
- **Adjustment:** Added `CellConfig.Agent` (+ `Prompt`) and branched the drive step
  in `cell.go` (scripted → `driveScript`; claude → `subprocess.StartClaude`). The
  sandbox/daemon/tap/merge/result spine is shared verbatim — only the ~15-line
  drive leg differs. This honors "consume the cell entry point, do not re-implement
  the spine" while making claude actually reachable.
- **Files modified:** bench/runtime/cell.go, bench/runtime/matrix.go.
- **Commit:** 6878ba7d.

No bugs (Rule 1), no missing critical functionality (Rule 2), and no architectural
changes (Rule 4) were required. The drive-branch wiring is the only divergence and
stays within the plan's stated key_links (`cmd → RunCell`, `subprocess/claude.go →
agent.Agent`).

## Scope Boundary Notes

- **No Makefile targets** — `bench`/`bench-quick`/`bench-<suite>` are Wave 5 / Plan
  05. This plan delivers only the `helix-bench run` surface those targets invoke.
- **Rich metrics stay null** (Phase 79): `result.v2.json` is provenance-complete /
  metric-sparse (tokens 0 for the scripted gate; no `edit_locality`/`regression_rate`/pass@k).
- **One benchmark per run**: `--benchmarks` is a single string (matches helix-eval's
  `--corpus`); `ExpandMatrix` accepts a `[]string` so Phase 78+ can widen to
  multiple suites without an API change.
- A large pre-existing repo-wide gofmt drift remains under `internal/`/`cmd/` (other
  Go-version formatting); it was NOT touched (out of scope, as Wave 3 noted). All
  six files this plan touches are gofmt-clean.

## Known Stubs

None. The matrix expander, dispatcher, run subcommand, and scripted drive are fully
wired and exercised end-to-end (the E2E smoke ran a real cell to a schema-valid
result). The `--agent=claude` branch is intentionally wired-not-gating per D-01 —
it is fully implemented (delegates to the real `agent.Agent`) and locally runnable,
not a data stub.

## Self-Check: PASSED

- bench/runtime/matrix.go — FOUND
- bench/runtime/matrix_test.go — FOUND
- bench/runtime/subprocess/claude.go — FOUND
- cmd/helix-bench/run_cmd_test.go — FOUND
- cmd/helix-bench/main.go (run subcommand) — FOUND (notYetImplemented("run") removed)
- bench/runtime/cell.go (agent drive branch) — FOUND
- Commit 91cf42bc (feat, Task 1) — FOUND
- Commit 6878ba7d (feat, Task 2) — FOUND
