---
phase: 77-bench-runtime-first-e2e-smoke
plan: 03
subsystem: bench-runtime
tags: [bench, runtime, cell, forwarder-drive, daemon-tap, trace-merge, integration, BENCH-04]
requires:
  - bench/runtime/sandbox.Sandbox (Plan 01 — embed + durable paths)
  - bench/runtime/subprocess.StartDaemon (Plan 01 — per-cell daemon lifecycle)
  - bench/runners.ResolveProfile / ResolveProfileFromRoot (Plan 01 — mode->profile)
  - bench/runners.DefaultContract (Plan 01/75 — fairness block)
  - bench/datasets/toolbench-go/sum-doubler (Plan 01 — seed task)
  - bench/runtime.BuildResult / Validate (Plan 02 — result.v2 builder + gate)
  - bench/runtime.SynthCCTap (Plan 02 — CC-leg synthesis)
  - internal/eval/runner.LoadScript / Script / StepResult (scripted step replay)
  - internal/eval/trace.TapDaemonLog / Merge / MergeInput / MergedTrace (2-leg trace)
provides:
  - bench/runtime.driveScript (forwarder drive of scripted steps over the per-cell socket)
  - bench/runtime.RunCell / CellConfig / CellResult (end-to-end single-cell orchestrator)
  - bench/runtime daemon-tap + cross-cell integration tests (BENCH-04 / criterion #4)
affects:
  - Plan 04 (cmd/helix-bench run: the matrix expander calls RunCell per cell; --agent=claude branch wires into drive/subprocess)
  - Plan 05 (Makefile bench-quick builds helix then runs the integration-gated smoke)
  - Phase 78 (corpus tasks needing semantic tools must revisit the per-cell semantic-disable workaround)
tech-stack:
  added: []
  patterns:
    - forwarder-drive NDJSON transport (helix --mode=stdio --socket=), stdin never closed early
    - activate_project (repo_path) as harness setup before scripted edits, not recorded as a task step
    - capture daemonPID via h.Pid() BEFORE h.Kill(); tap PID-gated with it (criterion #4)
    - kill daemon to flush slog THEN TapDaemonLog (Pitfall 4)
    - 2-leg trace.Merge (real daemon tap + synth CC leg) carrying the 3 Nyquist signals
    - D-08 preserve-on-failure: Cleanup gated to the fully-successful path only
    - validateCellKey rejects ../ in task/mode/benchmark before any path join (V5/T-77-08)
    - per-cell config disables semantic_index to keep parallel daemons hermetic
    - integration tests SKIP without a resolvable helix binary (HELIX_BIN env or PATH)
key-files:
  created:
    - bench/runtime/drive.go
    - bench/runtime/cell.go
    - bench/runtime/cell_test.go
    - bench/runtime/daemon_tap_integration_test.go
    - bench/runtime/cross_cell_test.go
  modified: []
decisions:
  - "D-06 driveScript shells helix --mode=stdio --socket= once, sends initialize + activate_project + one tools/call per scripted step over NDJSON, never closing stdin early (forwarder CloseSend races response delivery)"
  - "activate_project uses arg key repo_path (NOT path — internal/mcp/server.go ActivateProjectArgs json tag); the inprocess.go reference used path, which is silently dropped — the bench drive corrects it. Without activation replace_in_file returns no_workspace and the edit never lands"
  - "activate_project is harness setup, NOT recorded as a scripted StepResult (keeps the synth CC leg faithful to the task's tool sequence); it DOES appear as a real daemon tool_call, so cross_cell allows {activate_project, replace_in_file}"
  - "parseRPCLine surfaces MCP result.isError text as StepResult.Err so tool-level failures (e.g. no_workspace) are observable, not silently nil"
  - "[Rule 3] per-cell config sets semantic_index.enabled=false: the semantic store opens EAGERLY at daemon startup (daemon.go:318) relative to the daemon CWD; the cwd-relative default .helix/semantic.duckdb makes parallel cells deadlock on the DuckDB file lock. T-57-02-01 forbids absolute store paths and D-07 forbids forking eval StartDaemon to set cmd.Dir, so disabling the semantic INFRA (not the bench-full tool surface) is the hermetic fix for criterion #1"
  - "D-08 preserve-on-failure implemented in RunCell: Cleanup() runs only on the fully-successful tail; any infra error returns with ScratchPreserved=true and the retained scratch path surfaced"
metrics:
  duration_seconds: 2400
  completed: 2026-06-17
  tasks: 3
  files: 5
---

# Phase 77 Plan 03: End-to-End Single-Cell Spine Summary

The BENCH-04 heart of the phase: a forwarder-drive transport (`drive.go`) and a single-cell
orchestrator (`cell.go`) that compose Plans 01+02 into a runnable cell — spawn a per-cell daemon
over a Unix socket, drive the scripted edit through the `helix --mode=stdio` forwarder, kill the
daemon, PID-gated-tap `daemon.log`, run `verify.sh`, synthesize the CC leg, `trace.Merge` a 2-leg
trace, build + schema-validate `result.v2.json`, write durable artifacts, and clean up scratch
(preserve-on-failure). Two integration tests prove the three Nyquist smoke assertions end-to-end
against a real helix binary.

## What Was Built

### Task 1 — `drive.go` (D-06 forwarder transport)
`driveScript(ctx, helixBin, sockPath, workspaceRoot, script)` shells `helix --mode=stdio
--socket=<sock>` ONCE, sends an `initialize` frame, then an `activate_project` frame (pointing the
daemon workspace at the cloned repo so relative edit paths resolve), then one `tools/call` frame per
scripted step over NDJSON. It records each scripted step as a `runner.StepResult{Tool, AtTime,
Response, Err}` with a distinct dispatch instant (Pitfall 5) for `SynthCCTap`. stdin is NEVER closed
between frames or before responses arrive (closing races the forwarder CloseSend against response
delivery). A background reader keys responses by JSON-RPC id; `parseRPCLine` surfaces both
transport-level JSON-RPC errors and MCP `result.isError` tool failures into `StepResult.Err`.

### Task 2 — `cell.go` (BENCH-04 spine) + `cell_test.go` (unit)
`RunCell(ctx, CellConfig) (CellResult, error)` runs the RESEARCH steps 1-11 for one
`<run_id>/<task>/<mode>` cell: resolve mode->profile (D-05) → bench sandbox Prepare+CloneRepo (D-07,
ephemeral OS-temp) → spawn per-cell daemon and capture `daemonPID := h.Pid()` BEFORE any Kill
(criterion #4) → drive the scripted edit → `h.Kill()` (flush slog) THEN `TapDaemonLog(daemonLog,
daemonPID)` (PID-gated) → run `verify.sh` → `SynthCCTap(steps)` → 2-leg `trace.Merge` →
`BuildResult`+`Validate` the result.v2 (outcome from `merged.Outcome`, tokens 0, fairness from
`DefaultContract`, `trace_ref` = durable trace path) → write `result.v2.json` + `trace.json` under
`<out>/<task>/<mode>/` → on success `Cleanup()` scratch; on ANY infra error PRESERVE scratch and
surface the path (D-08). `CellResult` exposes the three Nyquist signals
(`ToolCallTotal`+`CCLegPresent`, `RejectedForeignPid`, `ResultValid`). `validateCellKey` rejects
`../`/separators/leading-dot in task/mode/benchmark before any path join (V5/T-77-08). `cell_test.go`
(unit, no daemon) asserts the `<out>/<task>/<mode>/{result.v2.json,trace.json}` layout via the
sandbox path helpers, path-traversal rejection, and preserve-on-failure gating (a missing seed dir
forces a CloneRepo failure; the scratch dir is asserted to still exist on disk).

### Task 3 — integration tests (`daemon_tap_integration_test.go`, `cross_cell_test.go`)
`TestDaemonTap` (BENCH-04) runs one cell on the seed `sum-doubler` task in `your_agent_full` mode and
asserts all three Nyquist signals — (1) `ToolCallTotal>=1` AND the CC leg is present, (2)
`RejectedForeignPid==0`, (3) `result.v2` schema-valid — plus `verifyExit==0` (the scripted edit made
`go test` pass), `Outcome=="success"`, and that the durable artifacts exist on disk. `TestCrossCell`
(criterion #4 / METRIC-06) boots 3 cells in parallel and asserts per cell `RejectedForeignPid==0` and
that no merged trace contains a tool name outside `{activate_project, replace_in_file}` (no
foreign-cell leakage). Both tests resolve the helix binary via the `HELIX_BIN` env override (so CI /
`make bench-quick` can point at a freshly-built binary) then `helix` on PATH, and SKIP cleanly when
neither is available — mirroring eval's `daemon_tap_integration_test.go`.

## Integration Tests: RAN (not skipped)

Both integration tests were executed GREEN against a freshly-built helix binary in this environment:
`go build -o /tmp/helix-bench-bin/helix ./cmd/helix`, then
`HELIX_BIN=/tmp/helix-bench-bin/helix go test ./bench/runtime/ -run 'DaemonTap|CrossCell' -count=1`
→ `--- PASS: TestCrossCell`, `--- PASS: TestDaemonTap`. The SKIP path was also verified: with helix
absent from PATH and no `HELIX_BIN`, both report `--- SKIP` and the suite stays green.

## How to Verify

- Build the daemon first: `go build -o /tmp/helix/helix ./cmd/helix`.
- Unit: `go test ./bench/runtime/ -run 'CellLayout|ResultV2Valid|SynthCCTap' -count=1` — passes.
- Integration (RUN): `HELIX_BIN=/tmp/helix/helix go test ./bench/runtime/ -run 'DaemonTap|CrossCell' -count=1` — passes.
- Integration (SKIP): with `helix` off PATH and no `HELIX_BIN`, the same command SKIPs (does not fail).
- Whole bench tree: `HELIX_BIN=/tmp/helix/helix go test ./bench/... -count=1` — all green.
- `go build ./...`, `go vet ./bench/...` clean; `gofmt -l bench/` empty.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] activate_project arg key is `repo_path`, not `path`**
- **Found during:** Task 3 (DaemonTap first run returned `verifyExit=1`).
- **Issue:** Without a workspace, `replace_in_file` returned `no_workspace: no active workspace` and
  the edit never landed. The inprocess.go reference (inprocess.go:278-280) activates with
  `{"path": repoDir}`, but `internal/mcp/server.go ActivateProjectArgs` has json tag `repo_path` —
  so `path` is silently dropped and the workspace activates at an empty root.
- **Fix:** `drive.go` prepends an `activate_project` frame with the correct `repo_path` arg before
  the scripted steps; after the fix `replace_in_file` reports "1 replacement(s) made" and `go test`
  passes.
- **Files modified:** bench/runtime/drive.go.
- **Commit:** 5051d33d.

**2. [Rule 1 - Bug] tool-level MCP errors were recorded as nil StepResult.Err**
- **Found during:** Task 3 (the no_workspace failure was invisible because responses carried
  `result.isError:true`, not a JSON-RPC `error`).
- **Fix:** `parseRPCLine` now surfaces `result.isError` (with its text content) into `StepResult.Err`,
  so tool failures are observable and propagate into the synth CC leg's `IsError`.
- **Files modified:** bench/runtime/drive.go.
- **Commit:** 5051d33d.

**3. [Rule 3 - Blocking] parallel cells deadlock on the shared cwd-relative DuckDB semantic store**
- **Found during:** Task 3 (CrossCell — daemon socket "did not appear within 10s"; daemon.log showed
  a DuckDB "Conflicting lock" on `bench/runtime/.helix/semantic.duckdb`).
- **Issue:** The semantic store opens EAGERLY at daemon startup (daemon.go:318) relative to the
  daemon process CWD, before any workspace is active. The cwd-relative default
  (`.helix/semantic.duckdb`) makes every parallel cell daemon open the SAME DuckDB file and deadlock
  on its file lock — breaking criterion #1's "no port/resource collisions on --parallel". An absolute
  per-cell store path is rejected by T-57-02-01 (`must be workspace-relative`), and eval's StartDaemon
  exposes no `cmd.Dir` (D-07 forbids forking it).
- **Fix:** `cell.go` writes a per-cell `helix_config.yml` (passed via StartDaemon's `--config=`) that
  pins the resolved profile and sets `semantic_index.enabled=false`. This isolates the semantic INFRA
  (daemon.go:308 makes the store nil and the daemon proceeds) without touching the bench-full TOOL
  surface (still applied via `--profile=bench-full`). The Phase 77 seed task drives a text-level
  `replace_in_file` edit that needs no semantic store. Documented for Phase 78+ to revisit (corpus
  tasks needing semantic tools should spawn the daemon with a per-cell cwd).
- **Files modified:** bench/runtime/cell.go.
- **Commit:** 5051d33d.

No architectural (Rule 4) changes were required; all three were correctness/blocking fixes within the
plan's scope.

## Scope Boundary Notes

- The `helix-bench run` cobra command (Wave 4 / Plan 04) and the Makefile targets (Wave 5 / Plan 05)
  were NOT built — out of this plan's scope. `RunCell` is the per-cell entry the matrix expander will
  call; the `--agent=claude` branch is reserved for Plan 04 (the driver and subprocess package both
  document that seam).
- A large pre-existing gofmt drift exists across `internal/` and `cmd/` (files not gofmt-clean under
  this Go version). `gofmt -w .` touched ~90 unrelated files; all were reverted (out of scope). Only
  the five `bench/runtime/` files are changed by this plan, and all are gofmt-clean.
- Pre-existing untracked build artifacts (`helix-bench`, `vet-ablation-leakage`, `.smtc-cache/`)
  predate this session and are not this plan's responsibility.

## Known Stubs

None. `drive.go` and `cell.go` are fully wired and exercised end-to-end by the green integration
tests. The `--agent=claude` seam is an intentional, plan-mandated Plan 04 deferral (documented in the
subprocess package since Plan 01), not a data stub blocking this plan's goal.

## Self-Check: PASSED

- bench/runtime/drive.go — FOUND
- bench/runtime/cell.go — FOUND
- bench/runtime/cell_test.go — FOUND
- bench/runtime/daemon_tap_integration_test.go — FOUND
- bench/runtime/cross_cell_test.go — FOUND
- Commit 8256d8b0 (feat, Task 1) — FOUND
- Commit b61ec7cc (feat, Task 2) — FOUND
- Commit 5051d33d (test, Task 3) — FOUND
