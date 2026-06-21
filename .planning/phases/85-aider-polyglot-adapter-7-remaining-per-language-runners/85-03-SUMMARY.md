---
phase: 85
plan: 03
subsystem: bench/languages
tags: [language-runner, python, java, csharp, hermetic-golden, pytest-json, surefire, trx, capability-coverage]
requires:
  - bench/languages.LanguageRunner seam (Plan 02 / Phase 78 Go runner)
  - bench/languages.Coverage ≥N/10 gate
  - bench/languages registry (Register / RunnerFor)
provides:
  - bench/languages/python.PyRunner (internal-toolbench, python)
  - bench/languages/java.JavaRunner (internal-toolbench, java)
  - bench/languages/csharp.CSharpRunner (internal-toolbench, csharp)
  - hermetic golden parsers: parsePytestJSON, parseSurefireXML, parseTRX
  - TestJavaFixtureProvenance gate (A5 anti-tautology)
affects:
  - bench dispatch can now run Python/Java/C# toolbench tasks structurally
tech-stack:
  added: []
  patterns:
    - "Clone go/runner.go verbatim; swap only Detect/Setup-argv/exec-argv/parser"
    - "Passed = (exit code == 0) authoritative gate; rows advisory (Pitfall 4)"
    - "ctx.Err()→infra vs *exec.ExitError→exit-code split, verbatim from Go runner"
    - "encoding/xml struct unmarshal for surefire/TRX (NEVER regex)"
    - "REAL-sourced golden + provenance gate guards against tautological fixtures"
key-files:
  created:
    - bench/languages/python/runner.go
    - bench/languages/python/runner_test.go
    - bench/languages/python/testdata/pytest_report.json
    - bench/languages/java/runner.go
    - bench/languages/java/runner_test.go
    - bench/languages/java/testdata/surefire-TEST.xml
    - bench/languages/csharp/runner.go
    - bench/languages/csharp/runner_test.go
    - bench/languages/csharp/testdata/results.trx
    - bench/languages/coverage_python_test.go
    - bench/languages/coverage_java_csharp_test.go
    - bench/datasets/internal-toolbench/python/ (9 task.json)
    - bench/datasets/internal-toolbench/java/ (9 task.json)
    - bench/datasets/internal-toolbench/csharp/ (7 task.json)
  modified: []
decisions:
  - "pytest golden built from the documented pytest-json-report 1.5.0 schema (plugin not installable offline in this env); labeled as schema-derived in the test"
  - "Java surefire golden sourced VERBATIM from apache/maven-surefire@5ee132b CircleTest report; provenance gate enforces non-placeholder marker"
  - "C# TRX golden is a REAL live-captured dotnet 8.0.416 run (Passed+Failed+NotExecuted)"
  - "Capability gaps declared explicitly: Python omits lsp_diagnostics; Java omits fuzzy_search; C# omits lsp_diagnostics/call_graph/dependency_graph"
metrics:
  duration: ~25m
  completed: 2026-06-21
---

# Phase 85 Plan 03: Python + Java + C# Language Runners Summary

Three file-reporter `LanguageRunner`s landed by cloning the Phase 78 Go runner template verbatim — Python (`pytest --json-report`), Java (`mvn test` → surefire XML), C# (`dotnet test --logger trx`) — each proven by a HERMETIC golden-fixture parser test that never shells out, satisfying TOOLBENCH-03/06/07 with capability coverage of 9/9 (Python), 9/9 (Java), 7/7 (C#) against the declared sets.

## What Was Built

- **Python (`PyRunner`)**: `Detect` on pyproject/setup.py/requirements; `RunTests` runs `pytest --json-report --json-report-file=.report.json -q`; `parsePytestJSON` decodes `tests[].nodeid`+`outcome` into pass/fail/skip tri-state rows via `encoding/json`. Declares 9 capabilities (omits `lsp_diagnostics`).
- **Java (`JavaRunner`)**: `Detect` on pom.xml; `RunTests` runs `mvn test -Dsurefire.useFile=false`, globs `target/surefire-reports/TEST-*.xml`; `parseSurefireXML` (`encoding/xml`) treats a `<testcase>` with no child as a pass, a child `<failure>`/`<error>` as a fail, `<skipped>` as a skip. Declares 9 capabilities (omits `fuzzy_search`). mvn/javac absent → live layer skips cleanly.
- **C# (`CSharpRunner`)**: `Detect` on `*.csproj`/`*.sln`; `RunTests` runs `dotnet test --logger "trx;LogFileName=R.trx"`; `parseTRX` (`encoding/xml`, `Results>UnitTestResult` path) maps `outcome="Passed"`→pass, `"NotExecuted"`→skip, else fail. Declares 7 capabilities.

All three keep the load-bearing invariants verbatim from `go/runner.go`: `Passed = exitCode == 0` (never inferred from zero failing rows — Pitfall 4), the `ctx.Err()`→infra vs `*exec.ExitError`→exit-code split, and the `var _ languages.LanguageRunner = (*XRunner)(nil)` conformance assertion. Each registers via `init()` for its `(internal-toolbench, <lang>)` key.

## Hermetic Proof (the sole authoritative gate)

- `TestParsePytestJSONGolden` — 1 pass + 1 fail + 1 skip from committed `pytest_report.json`, no subprocess.
- `TestParseSurefireXMLGolden` — 6 pass + 1 failure + 1 error from the real apache/maven-surefire CircleTest report, no subprocess.
- `TestParseTRXGolden` — Passed + Failed + NotExecuted from the live-captured dotnet TRX, no subprocess.
- All parsers are total: malformed/nil input yields zero rows, never panics.

## Fixture Provenance (A5)

The Java surefire golden was **not** hand-fabricated. It is `apache/maven-surefire@5ee132b4e95b31a3b247f295d465d7f69ae69b22 :: maven-surefire-report-plugin/.../TEST-com.shape.CircleTest.xml`, committed verbatim with a leading `<!-- provenance: ... -->` XML comment. `TestJavaFixtureProvenance` asserts the marker is present and non-placeholder; it was verified to FAIL when the marker is temporarily replaced with "hand-built TODO" and PASS once restored byte-identically — proving it is a real guard, not a rubber stamp. The C# TRX is a real `dotnet test` capture (dotnet 8.0.416 present in this env).

## Capability Coverage

| Lang | Declared | Covered | Threshold | Result |
|------|----------|---------|-----------|--------|
| Python | 9 | 9 | ≥8 | PASS |
| Java | 9 | 9 | ≥8 | PASS |
| C# | 7 | 7 | ≥6 | PASS |

Coverage asserted by `coverage_python_test.go` and `coverage_java_csharp_test.go` driving the existing `languages.Coverage` aggregator over authored capability-tagged `task.json` fixtures.

## Deviations from Plan

None — plan executed as written. The plan's Task 1 (`checkpoint:human-verify` for `pytest-json-report` legitimacy) was pre-approved by the orchestrator (numirias/pytest-json-report is a legitimate plugin; the [SUS] verdict was an unknown-downloads registry gap, not a slopsquat). Because the plugin is not offline-installable in this env, the Python hermetic test parses a committed schema-derived fixture (the sole proof) and the live `pytest --json-report` layer skips cleanly when the plugin is absent — exactly the false-green guard the plan mandated.

## TDD Gate Compliance

Both waves followed RED→GREEN with separate commits:
- `test(85-03)` RED: `6f69f110` (Python), `a85e4fe0` (Java/C#) — failing/uncompilable hermetic tests committed first.
- `feat(85-03)` GREEN: `cb0dc258` (Python), `317a3522` (Java/C#) — runners making them pass.

## Verification

- `go build ./...` — clean.
- `go vet ./bench/...` — clean.
- `go test ./bench/languages/... -count=1` — all packages ok (python, java, csharp, languages, go).
- Live mvn/javac layers skip cleanly (absent); dotnet present.

## Self-Check: PASSED
