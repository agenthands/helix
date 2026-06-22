---
phase: 85
plan: 05
subsystem: bench/languages
tags: [language-runner, typescript, javascript, vitest, jest, toolbench, tdd]
requires:
  - bench/languages.LanguageRunner (seam)
  - bench/languages.Coverage (capability gate)
provides:
  - TSRunner (vitest run --reporter=json) registered (internal-toolbench, typescript)
  - JSRunner (jest --json) registered (internal-toolbench, javascript)
  - shared parseJestStyleJSON Jest-compatible parser
affects:
  - bench harness language resolution (RunnerFor now answers typescript + javascript)
tech-stack:
  added: []
  patterns:
    - Clone of go/runner.go template (ctx.Err()->infra vs *exec.ExitError->exit-code split)
    - Passed=exitCode==0 authoritative gate (Pitfall 4)
    - Hermetic golden-fixture parser test as sole authoritative proof (MEMORY false-green)
key-files:
  created:
    - bench/languages/typescript/runner.go
    - bench/languages/typescript/runner_test.go
    - bench/languages/typescript/testdata/vitest-report.json
    - bench/languages/javascript/runner.go
    - bench/languages/javascript/runner_test.go
    - bench/languages/javascript/testdata/jest-report.json
    - bench/languages/coverage_tsjs_test.go
    - bench/datasets/internal-toolbench/typescript/ (9 task.json fixtures)
    - bench/datasets/internal-toolbench/javascript/ (9 task.json fixtures)
  modified: []
decisions:
  - "Detect precedence: TS requires package.json AND tsconfig.json; JS requires package.json AND absence of tsconfig.json — no double-claim of a shared dir"
  - "Shared parseJestStyleJSON cloned into both packages (self-contained sibling pattern); vitest 'skipped' and jest 'pending'/'todo' both map to TestResult.Skipped"
  - "TestResult.Name = assertionResult.fullName (fallback to title)"
  - "Both runners declare 9/10 capabilities incl CapLSPDiagnostics (tsserver / eslint); CapCallGraph is the explicit declared gap"
metrics:
  duration: ~12m
  completed: 2026-06-21
---

# Phase 85 Plan 05: TypeScript + JavaScript Language Runners Summary

Landed the final two `LanguageRunner`s by cloning the Go runner template: **TSRunner**
(`vitest run --reporter=json`, TOOLBENCH-04) and **JSRunner** (`jest --json`,
TOOLBENCH-05). Both reporters emit the Jest-compatible
`testResults[].assertionResults[].status`/`.fullName` shape, so a single
`parseJestStyleJSON` strategy (cloned per package) decodes both, with vitest's
`skipped` and jest's `pending`/`todo` mapped to `TestResult.Skipped`. Each runner is
proven hermetically by a committed golden fixture (the sole authoritative proof) and
gated to ≥8/10 capability coverage by 9 authored capability-tagged dataset fixtures
each.

## What Was Built

- **`bench/languages/typescript/runner.go`** — `TSRunner` implementing
  `languages.LanguageRunner`, registered for `(internal-toolbench, typescript)` via
  `init()`. `Detect` requires BOTH `package.json` and `tsconfig.json`. `RunTests`
  invokes `exec.CommandContext(ctx, "vitest", "run", "--reporter=json")`, captures
  stdout via `cmd.Output()`, keeps the `ctx.Err()→infra` vs `*exec.ExitError→exit-code`
  split VERBATIM, and returns `Passed: exitCode==0`. Declares 9 capabilities incl
  `CapLSPDiagnostics` (tsserver).
- **`bench/languages/javascript/runner.go`** — `JSRunner`, registered for
  `(internal-toolbench, javascript)`. `Detect` requires `package.json` AND the
  ABSENCE of `tsconfig.json` (the precedence rule that prevents TS/JS double-claim).
  `RunTests` invokes `exec.CommandContext(ctx, "jest", "--json")`. Declares 9
  capabilities incl `CapLSPDiagnostics` (eslint).
- **Shared parser** `parseJestStyleJSON` (cloned into both packages) decoding only
  `testResults[].assertionResults[].{status,fullName,title}` via `encoding/json`.
  Total on malformed/empty input (returns nil, never panics) — advisory only; the
  exit code stays the gate.
- **Hermetic golden fixtures** `testdata/vitest-report.json` and
  `testdata/jest-report.json`, each with ≥1 passed + ≥1 failed + 1 skipped/pending
  assertionResult, built from the documented Jest-compatible schema (RESEARCH
  §Reporter Formats; vitest/jest are npx-only in this env so live capture is not the
  proof).
- **Capability fixtures** — 9 `task.json` per language under
  `bench/datasets/internal-toolbench/{typescript,javascript}/`, covering all 9
  declared capabilities (semantic_view, lsp_diagnostics, rename_safety, fuzzy_search,
  dependency_graph, patch_apply, context_minimization, incremental_update,
  failure_handling).
- **`coverage_tsjs_test.go`** — `TestTSCapabilitiesAtLeast8` /
  `TestJSCapabilitiesAtLeast8` driving `languages.Coverage` against the authored
  corpus.

## TDD Gate Compliance

- RED commit `4ba9c097` (`test(85-05): ...`) — golden + capability + detect-precedence
  tests committed first; failed to compile (no runner), confirming RED.
- GREEN commit `fe21dd60` (`feat(85-05): ...`) — runners + fixtures make all targeted
  tests pass.
- No REFACTOR commit needed.

## Verification

- `go build ./...` — clean.
- `go vet ./bench/...` — clean.
- `gofmt -l` on all created files — clean (no diffs).
- `go test ./bench/languages/... -count=1` — all 9 language packages + aggregator pass.
- Targeted: `go test ./bench/languages/typescript/ ./bench/languages/javascript/ ./bench/languages/ -run 'JestStyle|Vitest|Jest|TSCapabilities|JSCapabilities' -count=1` — `ok`.
- Hermetic golden parser tests pass with NO toolchain (the authoritative proof); live
  layers `t.Skip` cleanly because `vitest`/`jest` are not on PATH (npx-only env).
- Coverage gate: typescript Covered=9 (≥8), javascript Covered=9 (≥8).
- Gate commits present: `git log --oneline --grep='^test(85-05)'` and `--grep='^feat(85-05)'` both non-empty.

## Deviations from Plan

**Task 1 (vitest legitimacy checkpoint):** Pre-resolved by the orchestrator (vitest =
vitest-dev/vitest, legitimate; `--reporter=json` confirmed). No human block needed.
Since the hermetic test parses a COMMITTED golden fixture, no live `vitest` install was
required — the fixture is built from the documented Jest-compatible schema and labeled as
such in the test doc comment.

Otherwise: None — plan executed as written.

## Notes for Downstream

- **Detect precedence is load-bearing:** a directory with both `package.json` and
  `tsconfig.json` is claimed by TS only; a bare `package.json` by JS only. Any future
  harness change to resolution order must preserve this to avoid double-claim.
- **`Passed` is never inferred from rows.** A type-check / syntax failure yields a
  non-zero exit with zero/partial rows → `Passed=false`. Do not re-introduce
  `numFailedTests==0`-style inference.
- vitest/jest live execution needs a `node_modules` install; the live test bodies are
  intentionally minimal and gated behind a PATH presence check.

## Self-Check: PASSED

- All 7 named created files FOUND on disk; 9 TS + 9 JS dataset fixtures present.
- Commits `4ba9c097` (RED) and `fe21dd60` (GREEN) FOUND in git history.
