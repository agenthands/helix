---
phase: 77-bench-runtime-first-e2e-smoke
plan: 01
subsystem: bench-runtime
tags: [bench, runtime, sandbox, subprocess, resolver, fixture, tdd]
requires:
  - internal/eval/sandbox.Sandbox (embed target)
  - internal/eval/runner.LoadScript (scripted yaml strict-decode pattern)
  - internal/profile/profiles/bench-full.yaml (profile name target)
  - bench/runners.DefaultContract (downstream fairness source, Plan 03)
provides:
  - bench/runtime/sandbox.Sandbox (embeds eval sandbox + durable bench paths)
  - bench/runtime/subprocess.StartDaemon (per-cell Unix-socket daemon lifecycle)
  - bench/runners.ResolveProfile / ResolveProfileFromRoot (mode->profile)
  - bench/runners/your_agent_full/MODE.md (seed mode convention)
  - bench/datasets/toolbench-go/sum-doubler (seed failing-then-passing Go task)
affects:
  - Plan 02 (CC-tap synth + result.v2 builder reuse these primitives)
  - Plan 03 (cell orchestrator composes sandbox + subprocess + resolver + seed)
  - Phase 80 (mode resolver extends via new MODE.md dirs, no Go change)
tech-stack:
  added: []
  patterns:
    - struct-embed reuse (bench sandbox embeds eval sandbox, never forks)
    - table-driven filesystem-as-table resolver (MODE.md frontmatter)
    - strict yaml.v3 KnownFields decode (fail-closed on unknown keys)
    - path-traversal validation before filepath.Join (T-77-01 / V5)
    - self-contained hermetic Go fixture module (no external deps)
key-files:
  created:
    - bench/runners/mode_resolver.go
    - bench/runners/mode_resolver_test.go
    - bench/runners/your_agent_full/MODE.md
    - bench/runtime/sandbox/sandbox.go
    - bench/runtime/subprocess/daemon.go
    - bench/datasets/toolbench-go/sum-doubler/go.mod
    - bench/datasets/toolbench-go/sum-doubler/sum.go
    - bench/datasets/toolbench-go/sum-doubler/sum_test.go
    - bench/datasets/toolbench-go/sum-doubler/task.json
    - bench/datasets/toolbench-go/sum-doubler/scripted_agent.yaml
    - bench/datasets/toolbench-go/sum-doubler/verify.sh
  modified: []
decisions:
  - "D-05 resolver reads MODE.md frontmatter (not a Go map); filesystem is the table so Phase 80 adds dirs with zero code change"
  - "D-07 bench sandbox EMBEDS *evalsandbox.Sandbox; no eval method reimplemented; Cleanup gating deferred to cell call site (Plan 03)"
  - "D-06 daemon spawn delegates to embedded StartDaemon with empty --http-addr; no TCP port introduced by the wrapper"
  - "D-03 seed scripted edit uses replace_in_file with a tab-anchored exact pattern (no warm LSP needed; 77-RESEARCH Open Q2)"
  - "D-01 --agent=claude spawn branch reserved (documented, not implemented) in bench/runtime/subprocess for Plan 04"
metrics:
  duration_seconds: 291
  completed: 2026-06-17
  tasks: 3
  files: 11
---

# Phase 77 Plan 01: Bench Runtime Foundation Primitives Summary

Leaf foundation primitives for the bench runtime: a struct-embed bench sandbox over the Phase 67 eval sandbox, a per-cell Unix-socket daemon-subprocess wrapper, a MODE.md-driven mode->profile resolver, and the single hermetic failing-then-passing Go seed task — all thin-wrap reuse of `internal/eval/`, no new dependencies.

## What Was Built

- **`bench/runners` mode resolver (D-05, TDD).** `ResolveProfile("your_agent_full")` returns `"bench-full"` by reading `bench/runners/your_agent_full/MODE.md` frontmatter via strict `yaml.v3` `KnownFields(true)`. `ResolveProfileFromRoot` accepts an injectable root for testability. The lookup is table-driven (filesystem-as-table) so Phase 80 adds the other five modes as new `MODE.md` dirs with no Go change. Mode names are validated against path traversal (`..`, separators, leading dot, absolute) **before** any `filepath.Join` (T-77-01 / V5), mirroring `runner.validateTaskID`.
- **`bench/runtime/sandbox` (D-07).** `Sandbox` embeds `*evalsandbox.Sandbox` (no fork; all symlink/0700/short-socket hardening inherited) and adds only the durable-path helpers `ResultPath(task,mode)` → `result.v2.json` and `MergedTracePath(task,mode)` → `trace.json` under the run out dir (D-08). No embedded method is reimplemented; `Cleanup` delete-vs-preserve gating is intentionally left to the Plan 03 cell call site.
- **`bench/runtime/subprocess` (D-06).** `StartDaemon` delegates to the embedded eval `StartDaemon`, which spawns `helix --serve --socket=<cell> --http-addr= --json [--profile=…]` — empty `--http-addr`, so no TCP port and no `--parallel` collisions by construction. The `--agent=claude` spawn branch (D-01, wired-not-gating) is documented as this package's responsibility, reserved for Plan 04.
- **`bench/datasets/toolbench-go/sum-doubler` seed task (D-03).** A self-contained Go module (`toolbenchseed/sumdoubler`, no external deps) whose `Double` returns `x` (wrong) so `sum_test.go` FAILS pre-edit. `scripted_agent.yaml` carries one `replace_in_file` step flipping the tab-anchored `return x` to `return x * 2` (LoadScript-strict keys only), after which `go test` passes. `task.json` holds prompt/metadata; executable `verify.sh` exits with `go test`'s code to feed `MergeInput.VerifyExitCode` (D-04).

## How to Verify

- `go test ./bench/runners/ -run ModeResolver -count=1` — passes (4 resolver tests).
- `go build ./bench/... ./cmd/helix-bench/...` and `go vet ./bench/... ./cmd/helix-bench/...` — clean.
- `go test ./bench/... ./cmd/helix-bench/...` — all pass.
- Seed fixture: `cd bench/datasets/toolbench-go/sum-doubler && go test ./...` FAILS (exit 1) before edit; applying the scripted edit (`return x` → `return x * 2`) makes it PASS (verified by hand-apply + revert during execution).
- `gofmt -l bench/runners bench/runtime` — empty.

## TDD Gate Compliance

Task 1 followed RED → GREEN:
- RED: `test(77-01): add failing mode->profile resolver tests (D-05)` — `a891d52d` (compile failure: undefined `ResolveProfile`).
- GREEN: `feat(77-01): implement mode->profile resolver reading MODE.md (D-05)` — `2f20177f`.
No REFACTOR commit needed.

## Deviations from Plan

None — plan executed exactly as written. The seed Go module/test/metadata files live at the task root (per the plan's `files_modified` list), distinct from eval's `repo/` subdir layout; this is the plan's explicit structure, not a deviation.

## Scope Boundary Notes

- The seed module is its OWN Go module (`go.mod`), so its deliberately-failing test is isolated from the parent `github.com/agenthands/helix` module — confirmed not picked up by parent `go list ./...`, and the whole repo still builds.
- No corpus, evaluators, other modes, containers, cell orchestrator, CC-tap synth, or result.v2 builder were built — those are Plans 02-04 and Phases 78-80, honoring the Plan 01 leaf-primitive scope.

## Known Stubs

- `bench/runtime/subprocess.StartClaude` is intentionally reserved (documented, no exported signature committed) for the D-01 `--agent=claude` branch — Plan 04 implements it. This is an intentional, plan-mandated deferral (not a data-stub blocking Plan 01's goal); the Phase 77 CI gate exercises only the scripted path.

## Self-Check: PASSED

All 11 created files exist on disk; all 4 task commits (`a891d52d`, `2f20177f`, `49473595`, `3a248b10`) found in git history.
