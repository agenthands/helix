---
phase: 85
plan: 04
subsystem: bench/languages
tags: [language-runner, cpp, rust, hermetic-golden, ctest-junit, libtest-text, capability-coverage, pitfall-2]
requires:
  - bench/languages.LanguageRunner seam (Plan 02 / Phase 78 Go runner)
  - bench/languages.Coverage ≥N/10 gate
  - bench/languages registry (Register / RunnerFor)
provides:
  - bench/languages/cpp.CppRunner (internal-toolbench, cpp)
  - bench/languages/rust.RustRunner (internal-toolbench, rust)
  - hermetic golden parsers: parseCtestJUnit, parseLibtestText
  - TestRustDoesNotUseMessageFormatJSON anti-Pitfall-2 argv guard
affects:
  - bench dispatch can now run C++/Rust toolbench tasks structurally
tech-stack:
  added: []
  patterns:
    - "Clone go/runner.go verbatim; swap only Detect/Setup-argv/exec-argv/parser"
    - "Passed = (exit code == 0) authoritative gate; rows advisory (Pitfall 4)"
    - "ctx.Err()→infra vs *exec.ExitError→exit-code split, verbatim from Go runner"
    - "encoding/xml struct unmarshal for ctest JUnit (NEVER regex)"
    - "bufio.Scanner malformed-line-resync for libtest TEXT (NOT --message-format json — Pitfall 2)"
key-files:
  created:
    - bench/languages/cpp/runner.go
    - bench/languages/cpp/runner_test.go
    - bench/languages/cpp/testdata/ctest-junit.xml
    - bench/languages/rust/runner.go
    - bench/languages/rust/runner_test.go
    - bench/languages/rust/testdata/cargo-libtest.txt
    - bench/languages/coverage_cpprust_test.go
    - bench/datasets/internal-toolbench/cpp/ (7 task.json)
    - bench/datasets/internal-toolbench/rust/ (9 task.json)
  modified: []
decisions:
  - "C++ ctest JUnit golden is a REAL live capture (CMake 3.31.6): a pass_test (status=run, no child) and a fail_test (child <failure>)"
  - "Rust libtest TEXT golden is a REAL live capture (cargo 1.96.0): ... ok / ... FAILED / ... ignored plus the surrounding banner, failures section, and test result: summary — all of which the parser must re-sync past"
  - "Rust parses libtest TEXT, NEVER --message-format=json (Pitfall 2: stable cargo emits only compiler-artifact JSON, zero per-test rows); guarded by TestRustDoesNotUseMessageFormatJSON over a testArgv() hook"
  - "Capability gaps declared explicitly: C++ omits lsp_diagnostics/call_graph/dependency_graph (clangd surface not wired); Rust omits lsp_diagnostics (rust-analyzer diagnostics not wired)"
metrics:
  duration: ~18m
  completed: 2026-06-21
---

# Phase 85 Plan 04: C++ + Rust Language Runners Summary

Two more `LanguageRunner`s landed by cloning the Phase 78 Go runner template verbatim — **C++** (`ctest --output-junit R.xml` → JUnit XML, TOOLBENCH-08) and **Rust** (`cargo test` → libtest TEXT, TOOLBENCH-09) — each proven by a HERMETIC golden-fixture parser test that never shells out. C++ meets ≥6/10 (7 declared/covered), Rust meets ≥8/10 (9 declared/covered). The Rust runner sidesteps the cargo `--message-format=json` trap (Pitfall 2) by parsing the plain libtest text lines, guarded by an explicit argv-assertion test.

## What Was Built

- **C++ (`CppRunner`)**: `Detect` on `CMakeLists.txt`; `RunTests` runs `ctest --output-junit R.xml` against a configured+built tree (`cmd.Dir=repoDir`), reads `repoDir/R.xml`; `parseCtestJUnit` (`encoding/xml`) treats a `<testcase>` with no child as a pass and a child `<failure>` as a fail (RESEARCH §Reporter Formats). Declares 7 capabilities (omits `lsp_diagnostics`/`call_graph`/`dependency_graph` as explicit gaps; `semantic_view` is declared for clangd).
- **Rust (`RustRunner`)**: `Detect` on `Cargo.toml`; `RunTests` runs PLAIN `cargo test` (captured via `cmd.Output()`); `parseLibtestText` uses the Go template's `bufio.Scanner` malformed-line-resync discipline to match only `test <name> ... ok|FAILED|ignored` lines (`ok`→Passed, `FAILED`→fail, `ignored`→Skipped). Every other line — the `running N tests` banner, the `failures:` section, the `---- NAME stdout ----` block, the indented failure list, and the `test result:` summary — is skipped (re-sync), never aborting the scan. Declares 9 capabilities (omits `lsp_diagnostics`).

Both keep the load-bearing invariants verbatim from `go/runner.go`: `Passed = exitCode == 0` (never inferred from zero failing rows — Pitfall 4), the `ctx.Err()`→infra vs `*exec.ExitError`→exit-code split, and the `var _ languages.LanguageRunner = (*XRunner)(nil)` conformance assertion. Each registers via `init()` for its `(internal-toolbench, <lang>)` key.

## The Rust Pitfall 2 Guard

`cargo test --message-format=json` emits only `compiler-artifact`/`build-finished` JSON on stable Rust — NO per-test rows. The runner's fixed argv (`cargoTestArgv = []string{"cargo", "test"}`) deliberately excludes any `--message-format` flag. `TestRustDoesNotUseMessageFormatJSON` inspects the argv via an exported `testArgv()` hook and fails if `--message-format` / `--message-format=json` / a bare `json` token ever appears, so the footgun cannot be silently re-introduced (T-85-04-04). Exit code stays the authoritative `Passed` gate; the text rows are advisory.

## Hermetic Proof (the sole authoritative gate)

- `TestParseCtestJUnitGolden` — 1 pass (`pass_test`) + 1 fail (`fail_test`, child `<failure>`) from committed `ctest-junit.xml`, no subprocess.
- `TestParseLibtestTextGolden` — `tests::it_works` (ok) + `tests::it_fails` (FAILED) + `tests::it_is_ignored` (ignored→Skipped) from committed `cargo-libtest.txt`, no subprocess; the surrounding banner/failures/summary lines are correctly re-synced past.
- Both parsers are total: malformed/nil input yields zero rows, never panics. The Rust parser additionally proves a 1 MiB line followed by a real row does not exhaust the scanner (buffer capped at 4 MiB, T-85-04-03).

## Fixture Provenance

Both goldens are REAL live captures taken this session (not hand-fabricated):
- **ctest JUnit**: generated by a tiny 2-target CMake project (a `return 0` and a `return 1` executable), `cmake --build` + `ctest --output-junit R.xml`, under CMake 3.31.6 — committed verbatim.
- **cargo libtest TEXT**: generated by a tiny crate with `it_works` (passing), `it_fails` (`assert_eq!` panic), and `#[ignore] it_is_ignored`, `cargo test --offline` under cargo 1.96.0 — committed verbatim including the panic/failures sections so the resync discipline is exercised against real noise.

The live `ctest`/`cargo` layers (`TestRunTestsLive`) additionally run end-to-end here against generated projects (both toolchains present) but are never the sole proof — the golden tests are.

## Capability Coverage

| Lang | Declared | Covered | Threshold | Result |
|------|----------|---------|-----------|--------|
| C++  | 7 | 7 | ≥6 | PASS |
| Rust | 9 | 9 | ≥8 | PASS |

Coverage asserted by `coverage_cpprust_test.go` (`TestCppCapabilitiesAtLeast6`, `TestRustCapabilitiesAtLeast8`) driving the existing `languages.Coverage` aggregator over authored capability-tagged `task.json` fixtures.

## Deviations from Plan

None — plan executed exactly as written. Both toolchains were present (the plan anticipated this), so the additive live layers run rather than skip, but the hermetic goldens remain the sole authoritative proof per the plan's MEMORY false-green guard.

## TDD Gate Compliance

Both tasks followed RED→GREEN with separate commits:
- `test(85-04)` RED: `cb37795e` (C++), `376acfb2` (Rust) — uncompilable/failing hermetic tests committed first (verified failing: undefined `CppRunner`/`RustRunner`).
- `feat(85-04)` GREEN: `8deaa242` (C++), `d81e254e` (Rust) — runners making them pass.

## Verification

- `go build ./...` — clean.
- `go vet ./bench/...` — clean.
- `go test ./bench/languages/... -count=1` — all packages ok (cpp, rust, csharp, go, java, languages, python).
- Live ctest + cargo layers run green here (both toolchains present); golden tests are the authoritative proof.
- `git log --grep='^test(85-04)'` and `--grep='^feat(85-04)'` both non-empty (2 each).

## Self-Check: PASSED
