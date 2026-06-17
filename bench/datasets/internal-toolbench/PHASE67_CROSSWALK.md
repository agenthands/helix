# Phase 67 → Internal ToolBench Crosswalk (Inspiration Only)

> **INSPIRATION MAPPING ONLY — NO CODE MIGRATION.**
>
> This document maps which Phase 67 concepts inspired which `IT-go-*` capability
> fixtures. It is **not** a migration record:
>
> - **No code is ported.** No `T-67-*` task, harness, or fixture is copied into
>   `bench/`. The Phase 78 fixtures were authored fresh against the Phase 77 bench spine.
> - **Namespaces are deliberately disjoint (zero collision, criterion C4).** Internal
>   ToolBench ids use the `IT-go-*` prefix; Phase 67 used the `T-67-*` prefix. No id
>   appears in both spaces. The disjointness is enforced by the static test in
>   `bench/languages/coverage_test.go` (every id matches `^IT-go-`, none matches
>   `^T-67-`).
> - **No on-disk `T-67-*` corpus exists.** `T-67-*` are **planning task IDs** from the
>   Phase 67 plan/history, not runtime fixtures. The current `internal/eval/` tree is
>   package-structured (`agent/ budget/ judge/ runner/ sandbox/ …`) and carries no
>   `task.json` files (78-RESEARCH §"Phase 67 Crosswalk"). The `T-67-*` IDs below are
>   sourced from Phase 67 planning history, not from any on-disk dataset.

## What carried over (conceptually)

Phase 67 stood up the in-process `internal/eval/` PR-gate wiring smoke. Several of its
*concepts* — the scripted-agent harness, the per-cell sandbox isolation, and the
daemon-tap observation channel — shaped how the Phase 77/78 `bench/` spine and these
`IT-go-*` fixtures are structured. The *implementation* lives independently in `bench/`
(`bench/runtime`, `bench/languages`, `bench/runners`); see `bench/BENCH.md` "eval ↔ bench
separation note" — the two trees share no code.

## Crosswalk table

| Phase 67 task (`T-67-*`) | Concept | Inspired `IT-go-*` fixture(s) | What carried over conceptually |
|--------------------------|---------|-------------------------------|--------------------------------|
| **T-67-06a** scripted-agent harness | Replay a hard-coded MCP call sequence over the daemon socket (no real LLM) | **All 10** `IT-go-*` fixtures (each ships a `scripted_agent.yaml`) | The solve-then-verify shape: a deterministic, no-API-key scripted agent drives the capability's named tool(s); only the *idea* of scripted replay carried over — the parser is `bench/runtime`'s `runner.LoadScript`, authored fresh. |
| **T-67-01** sandbox isolation | Per-cell ephemeral sandbox (isolated HOME/socket/env, 0700, Setpgid) | All 10 (each cell runs in an isolated sandbox); load-bearing for `IT-go-incremental-update-1` | Per-cell isolation as the unit of execution. The `bench/` implementation reuses the eval `Sandbox.StartDaemon` primitive via the Phase 77/78 `WithWorkingDir` option — reuse of an existing primitive, not a port of a `T-67` fixture. |
| **T-67-04** daemon-tap | Observe the daemon's MCP traffic out-of-band (PID-gated tap) | `IT-go-incremental-update-1` (the store-ON cell whose isolation is asserted) | The notion of out-of-band per-cell observation; the `bench/runtime` daemon tap (`RejectedForeignPid` gating) is the Phase 77 implementation. |
| **T-67-02** scripted step / `expect_error` | A scripted step may assert an expected tool failure and recover | `IT-go-failure-handling-1` | The `expect_error: true` recover-and-continue pattern; implemented via `bench/runtime`'s `ScriptedStep.ExpectError`. |
| **T-67-03** structured grading | Grade a task by structured test output rather than a bare exit code | All 10 (graded via `GoRunner.RunTests` → `go test ./... -json`) | The idea of structured, per-test grading feeding downstream evaluators (Phase 79); the test2json parser is authored fresh in `bench/languages/go`. |

## Boundary statement

The `IT-go-*` corpus is a **new artifact** on the Phase 77 bench spine. Phase 67
contributed ideas and reusable primitives (sandbox, daemon tap) that already live in
`internal/eval/` and are *invoked* by `bench/runtime` — it did not contribute any fixture
or task that was copied into `bench/datasets/`. The `IT-go-*` and `T-67-*` namespaces
remain disjoint by construction.
