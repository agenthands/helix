---
phase: 78-internal-toolbench-go-first-languagerunner-interface
plan: 01
subsystem: bench/languages + eval sandbox
tags: [language-runner, go-test-json, functional-options, registry, tdd]
requires:
  - internal/eval/sandbox.Sandbox.StartDaemon (P77 D-07 embedded spawn)
  - bench/runtime/cell.runVerify (ctx.Err→infra / *exec.ExitError→exit-code split, reused shape)
provides:
  - bench/languages.LanguageRunner interface (Detect/Setup/RunTests/Capabilities)
  - bench/languages.TestOutcome / TestResult / Capability (10 D-06 string values)
  - bench/languages.Register / RunnerFor (Caddy-style registry, nil = D-10 fallback)
  - bench/languages/go.GoRunner (go test ./... -json wrapper + test2json parser)
  - internal/eval/sandbox.DaemonOption / WithWorkingDir (additive cmd.Dir option, D-03)
  - bench/runtime/subprocess.StartDaemon variadic opts threading
affects:
  - Phase 79 (grades structured go test -json output)
  - Phase 85 (drops 7 more runners behind LanguageRunner)
  - Plans 02/03/04/05 (Cell.Language, fixtures, capability strings, coverage)
tech-stack:
  added: []
  patterns:
    - "Functional options (daemonOpts + DaemonOption + WithWorkingDir) — written fresh, no prior repo idiom (PATTERNS No-Analog)"
    - "Caddy-style (benchmark, lang) registry modeled on internal/skill/registry.go"
    - "test2json streaming decode via json.NewDecoder over bytes.Reader; per-test rows on pass/fail/skip"
key-files:
  created:
    - bench/languages/runner.go
    - bench/languages/registry.go
    - bench/languages/go/runner.go
    - bench/languages/go/runner_test.go
  modified:
    - internal/eval/sandbox/sandbox.go
    - internal/eval/sandbox/sandbox_test.go
    - bench/runtime/subprocess/daemon.go
decisions:
  - "Passed = (exit code == 0) is the authoritative gate; parsed test rows are advisory detail (D-10)"
  - "Compile failure → Passed=false, Tests=[] (Pitfall 4): pass is never inferred from absence of failing rows"
  - "cmd.Output() retains captured stdout on *exec.ExitError, so the test2json stream survives a non-zero exit — no separate buffer needed"
  - "WithWorkingDir is the ONLY new behavior line in StartDaemon (cmd.Dir); Setpgid/env/log/waitSocket untouched (additive, non-forking — P77 D-07)"
metrics:
  duration: ~14min
  completed: 2026-06-17
---

# Phase 78 Plan 01: LanguageRunner Interface + Go Runner + WithWorkingDir Summary

The Phase 85-facing seam: a `LanguageRunner` interface + Caddy-style registry, a `GoRunner` that wraps `go test ./... -json` into a structured `TestOutcome` (Passed=exit==0, parsed per-test rows, compile-fail handled), and an additive `WithWorkingDir` daemon option threading `cmd.Dir` through `subprocess.StartDaemon` for per-cell store isolation.

## What Was Built

### Task 1 — LanguageRunner contract + GoRunner (TDD RED→GREEN)
- `bench/languages/runner.go`: `LanguageRunner` interface (`Detect`, `Setup`, `RunTests`, `Capabilities`); `TestOutcome{Passed, Tests, Raw, ExitCode}`; `TestResult{Name, Package, Passed, Elapsed}`; `Capability` string enum with the 10 D-06 source-of-truth values (`semantic_view`, `lsp_diagnostics`, `rename_safety`, `fuzzy_search`, `call_graph`, `dependency_graph`, `patch_apply`, `context_minimization`, `incremental_update`, `failure_handling`).
- `bench/languages/registry.go`: `RWMutex`-guarded map keyed `benchmark+"/"+lang`; `Register` (init-time) + `RunnerFor` (nil = D-10 verify.sh fallback signal), modeled on `internal/skill/registry.go`.
- `bench/languages/go/runner.go`: `GoRunner`. `Detect` checks `go.mod`; `Setup` no-op; `Capabilities` returns all 10; `RunTests` shells `go test ./... -json` with `cmd.Dir=repoDir`, applies the `ctx.Err()→infra` vs `*exec.ExitError→exit-code` split, sets `Passed=(ExitCode==0)`, and streams the test2json events into `[]TestResult`. Registered for `(internal-toolbench, go)` via `init()`.
- `bench/languages/go/runner_test.go`: compile-time conformance assertion + behavioral tests (passing module, failing module, compile-failure/Pitfall-4, Detect, Capabilities-10, RunnerFor nil-fallback).

RED commit `a59541ab` (build failed: `GoRunner` undefined) → GREEN commit `12bd32a9`.

### Task 2 — Additive WithWorkingDir daemon option (D-03)
- `internal/eval/sandbox/sandbox.go`: added `daemonOpts{workDir}`, `DaemonOption func(*daemonOpts)`, `WithWorkingDir(dir)`. `StartDaemon` signature gained `opts ...DaemonOption`; the one new behavior line `if o.workDir != "" { cmd.Dir = o.workDir }` sits immediately after `exec.CommandContext`. Setpgid/env-allowlist/log-redirect/waitSocket unchanged.
- `bench/runtime/subprocess/daemon.go`: `StartDaemon` gained `opts ...evalsandbox.DaemonOption`, forwarded into the embedded `sb.StartDaemon(..., opts...)`. (The `benchsandbox.Sandbox` embeds `*evalsandbox.Sandbox`, so the variadic is promoted with no extra wrapper.)
- `internal/eval/sandbox/sandbox_test.go`: `TestWithWorkingDirSetsWorkDir` asserts `WithWorkingDir("/x")` folds into `daemonOpts.workDir` and the zero value stays empty.

Commit `05614c15`.

## Verification Results

- `go test ./bench/languages/... -count=1` — PASS (all RED tests now GREEN).
- `go build ./...` — PASS (all existing 5-arg `StartDaemon` callers compile; additive proof for D-03).
- `go vet ./...` — clean.
- `gofmt -l` on all created/modified files — reports nothing.
- Source assertions: `var _ ...LanguageRunner = ...GoRunner` present; all 10 capability values verbatim in `runner.go`; `func WithWorkingDir` + `cmd.Dir = o.workDir` present; `opts ...evalsandbox.DaemonOption` threaded in subprocess.

## Success Criteria Met

- **C3 / TOOLBENCH-10**: `LanguageRunner` defined in `bench/languages/`; compile-time conformance assertion; `go vet ./bench/...` clean.
- **C2 / TOOLBENCH-02 (RunTests half)**: `GoRunner.RunTests` wraps `go test ./... -json`, returns structured per-test results, `Passed=(exit==0)`.
- **D-03 substrate**: `WithWorkingDir` sets `cmd.Dir`, threaded through `subprocess.StartDaemon`, additive (no broken callers). The parallel integration assertion lands in Plan 04.
- **D-11**: `Capabilities()` returns all 10 declared classes.

## Deviations from Plan

None — plan executed exactly as written.

## Threat Surface

No new external-input paths introduced. `WithWorkingDir(dir)` and `RunTests(repoDir)` both take harness-supplied sandbox paths (T-78-01 accept; not attacker-controlled this plan). `RunTests` uses `exec.CommandContext`, so the caller's deadline kills the subprocess (T-78-02 mitigate). Zero external package installs (T-78-SC accept).

## TDD Gate Compliance

RED gate (`test(78-01): ...` commit `a59541ab`, build-failed with undefined `GoRunner`) preceded GREEN gate (`feat(78-01): ...` commit `12bd32a9`). No unexpected RED-phase pass. No REFACTOR commit needed.

## Self-Check: PASSED

All 7 created/modified files present on disk; all 3 commit hashes (`a59541ab`, `12bd32a9`, `05614c15`) present in git log.
