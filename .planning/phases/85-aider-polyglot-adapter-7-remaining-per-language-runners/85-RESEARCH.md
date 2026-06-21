# Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners - Research

**Researched:** 2026-06-21
**Domain:** Go bench harness — per-language test runners + external dataset adapter (Aider Polyglot / Exercism)
**Confidence:** HIGH (codebase anchors verified by reading source + running real toolchains; external dataset MEDIUM)

## Summary

Phase 85 has two coupled deliverables on the Phase 77/78 bench spine: (1) seven new `bench/languages/<L>/runner.go` files cloning the Phase 78 Go `LanguageRunner` template verbatim (Python, TypeScript, JavaScript, Java, C#, C++, Rust), each shelling out to a native test runner and parsing structured output into the existing `languages.TestOutcome` struct; and (2) a dataset-loader-only Aider-Polyglot adapter (shallow git clone at a pinned sha, 2-attempt + stderr-reprompt protocol). The `LanguageRunner` seam, registry, coverage gate, and Phase 79 grader are already built and consume `TestOutcome` unchanged — the seven runners drop in with zero seam changes. The Go runner (`bench/languages/go/runner.go`) is a 160-line, exit-code-authoritative template; clone it seven times, swapping only `Detect`, `Setup`, the `exec.Command` invocation, and the parser.

**Three load-bearing gaps the planner must close** (none are blockers, all are scoped): (a) **no `language` axis in `result.v2` or the Phase 82 aggregator** — SC#1's "per-language pass-rate" requires either an additive `language` schema field + aggregator slicing, or recovering language from `task_id`/`benchmark`; (b) **`cargo test --message-format=json` does NOT emit per-test pass/fail on stable Rust** (it emits compiler-artifact JSON only; per-test results go to stdout as plain `test NAME ... ok/FAILED` text, or require `cargo nextest`/nightly libtest-json) — the Rust runner must parse the libtest text lines, not the message-format JSON, for per-test rows; (c) **SC#3 container test-execution (`--network=none`) is unbuilt** — `bench/container/` today only does pull + sigstore-verify (`PullByDigest`, `VerifyThenPull`); there is no `Run`/`exec` with `--network=none`, so the planner must add a container-run seam.

**Primary recommendation:** Clone the Go runner template seven times; drive EVERY runner's parse + capability logic from committed golden reporter-output fixtures (real pytest-json / TRX / cargo-text / ctest-JUnit samples captured in this research) so `go test ./...` proves correctness WITHOUT the live toolchain; gate every live end-to-end run and every container `--network=none` run on toolchain/engine availability with a clean `t.Skip` (HELIX_BIN-style), never letting absence go false-green.

## User Constraints (from CONTEXT.md)

### Locked Decisions
None hard-locked — discuss phase was skipped (`workflow.skip_discuss: true`). All implementation choices are at Claude's Discretion.

### Claude's Discretion
- The 7 per-language runners follow the Phase 78 Go `LanguageRunner` interface/template verbatim — research located that interface + the Go runner as the template to clone (see §Standard Stack / §Code Examples).
- Each runner shells out to the language's native test runner with a JSON/structured reporter (pytest/vitest/jest/mvn/dotnet/ctest/cargo) and parses results; semantic-view capabilities use existing LSP wiring where applicable.
- **Environment reality:** toolchains present — python3 3.13, node 20/npm/pnpm, dotnet 8, g++ 14, cargo/rustc 1.96, go; **javac is ABSENT (JRE 21 only, no JDK compiler)**; `mvn` ABSENT; `jest`/`vitest` not global (npx-only). Per-language live test execution and container `--network=none` runs (SC#3) MUST gate on toolchain/engine availability and SKIP cleanly when absent — with hermetic siblings (golden reporter-output fixtures parsed without the real toolchain) so each runner's parse/capability logic is unit-tested regardless. Plain `go test ./...` MUST NOT be false-green.
- Aider Polyglot adapter is dataset-loader-only (shallow clone at pinned sha) — gate the live clone/run on network; provide a small committed fixture task set for hermetic adapter tests.
- License audit + `make verify-licenses` is a hard gate following the Phase 75 `make validate-cost-table` / `make verify-tos` pattern.

### Deferred Ideas (OUT OF SCOPE)
None — discuss phase skipped.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ADAPTER-AIDER-01 | Aider Polyglot adapter via `dataset-loader-only`; shallow clone `Aider-AI/polyglot-benchmark` at pinned sha; 225 tasks × 6 langs; 2-attempt + stderr re-prompt. Acceptance: full run completes; per-language pass-rate matches sanity benchmarks. | §Aider Polyglot Dataset; §Architecture Pattern 4; §Open Questions Q1/Q3 (language axis, per-track license) |
| TOOLBENCH-03 | Python — `pytest --json-report`. ≥ 8/10 capabilities; gaps logged. | §Reporter Formats (pytest-json); §Code Examples; pkg `pytest-json-report` 1.5.0 [SUS-benign] |
| TOOLBENCH-04 | TypeScript — `vitest --reporter=json` (or `jest --json`); ≥ 8/10; tsserver diagnostics. | §Reporter Formats (vitest json); langregistry has `typescript-language-server` |
| TOOLBENCH-05 | JavaScript — `jest --json`; ≥ 8/10; eslint diagnostics. | §Reporter Formats (jest json); pkg `jest` 30.4.2 [OK] |
| TOOLBENCH-06 | Java — `mvn test -Dsurefire.useFile=false`; ≥ 8/10; jdtls semantic view. | §Reporter Formats (surefire XML); **javac+mvn ABSENT → live-skip**; langregistry has `jdtls` |
| TOOLBENCH-07 | C# — `dotnet test --logger trx`; ≥ 6/10. | §Reporter Formats (TRX, captured live); langregistry has `csharp-ls`/OmniSharp |
| TOOLBENCH-08 | C++ — `cmake/ctest`; ≥ 6/10; clangd semantic view. | §Reporter Formats (ctest JUnit, captured live); langregistry has `clangd` |
| TOOLBENCH-09 | Rust — `cargo test --message-format=json`; ≥ 8/10; rust-analyzer semantic view. | §Reporter Formats (cargo — **message-format does NOT carry per-test rows**, Pitfall 2); langregistry has `rust-analyzer` |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Per-language test execution | `bench/languages/<L>/` runner (os/exec subprocess) | `bench/runtime/cell.go` dispatch (`languages.RunnerFor`) | Runner owns the toolchain subprocess + parse; cell owns lifecycle (Setup→pre-snapshot→RunTests) |
| Structured test → metrics | `bench/evaluators/test_runner` (pure transform) | `bench/evaluators/coordinator` | Phase 79 grader already consumes `TestOutcome`; runners need zero grader change |
| Per-language pass-rate slicing | `bench/aggregator` (NEEDS language axis — gap) | `result.v2` schema (NEEDS additive `language` field — gap) | SC#1 sanity-benchmark comparison is per-language; today aggregator slices (task,mode) only |
| Dataset clone/load (Aider) | new `bench/datasets/aider-polyglot/` loader | `$HELIX_CACHE_DIR` clone cache (mirror `bench/ragindex/cache.go`) | dataset-loader-only: clone + map config.json files; NO upstream Python harness |
| Container `--network=none` run | new `bench/container` Run/exec seam (NEEDS building) | `bench/container/engine.go` (pull/verify exist; run does not) | SC#3 hermetic toolchain images; engine today is pull+verify only |
| License audit gate | new `cmd/helix-bench verify-licenses` + Makefile target | `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` | Mirror Phase 75 `verify-tos` validator + hard-fail Makefile gate |

## Standard Stack

### Core (already in tree — clone, do not add)
| Component | Location | Purpose | Why Standard |
|-----------|----------|---------|--------------|
| `LanguageRunner` interface | `bench/languages/runner.go` | The seam all 7 runners implement | Phase 85-facing contract; built explicitly for this phase [VERIFIED: read source] |
| `GoRunner` template | `bench/languages/go/runner.go` (160 LOC) | The exact shape to clone ×7 | exit-code-authoritative gate, malformed-line-resync parser, ctx-as-infra-error split [VERIFIED] |
| Registry | `bench/languages/registry.go` | `init()`-time `Register(benchmark, lang, runner)`; `RunnerFor` resolves | Caddy-style; nil = verify.sh fallback [VERIFIED] |
| Coverage gate | `bench/languages/coverage.go` | declared∩covered capability counting per `task.json` | ≥N/10 measured from `task.json` `capability` field (D-06), NOT the id [VERIFIED] |
| Phase 79 grader | `bench/evaluators/test_runner/test_runner.go` | `TestOutcome` → success/correctness/compile-error metrics | pure transform; consumes runners unchanged [VERIFIED] |
| cell dispatch | `bench/runtime/cell.go:535,659-698` | `RunnerFor(Benchmark,Language)` → Setup→RunTests; nil→verify.sh | already wired; new runners auto-dispatch via registry [VERIFIED] |
| clone cache pattern | `bench/ragindex/cache.go` | `$HELIX_CACHE_DIR` > `os.UserCacheDir()/helix` > `~/.helix/cache` | reuse for Aider shallow-clone cache [VERIFIED] |
| Makefile gate template | `cmd/helix-bench/verify_tos.go` + Makefile `verify-tos:` | hard-fail validator + `go run ./cmd/helix-bench <cmd>` target | the `make verify-licenses` template [VERIFIED] |

### Supporting (external test-reporter deps — install per language image)
| Dep | Version | Ecosystem | Purpose | Verdict |
|-----|---------|-----------|---------|---------|
| `pytest-json-report` | 1.5.0 | PyPI | `pytest --json-report --json-report-file=…` machine-readable output | [SUS — benign, see audit] |
| `pytest` | 8.3.5 (present) | PyPI | Python test runner | present in env |
| `vitest` | 4.1.9 | npm | `vitest run --reporter=json` | [SUS — too-new patch, 70M wk dl] |
| `jest` | 30.4.2 | npm | `jest --json` | [OK] |
| `maven-surefire-plugin` | (mvn builtin) | Maven | `mvn test` → `target/surefire-reports/*.xml` | n/a — **mvn ABSENT in env** |
| `dotnet test --logger trx` | sdk 8.0.416 (present) | dotnet | TRX XML | present in env |
| `ctest --output-junit` | CMake 3.31.6 (present, ≥3.21) | CMake | JUnit XML | present in env |
| `cargo test` | 1.96.0 (present) | rust | libtest text output (NOT message-format json for rows) | present in env |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `cargo test --message-format=json` for per-test rows | parse libtest text (`test NAME ... ok/FAILED`) OR `cargo nextest --message-format libtest-json` | message-format json is compiler-artifact only on stable; text parse is dependency-free and exit-code gate still authoritative (rows are advisory) |
| `vitest` for TS | `jest --json` for both TS+JS | one parser instead of two; but TOOLBENCH-04 names vitest. Recommend vitest for TS, jest for JS as specified |
| `mvn test` surefire XML | `./gradlew test` JUnit XML | Aider-Polyglot itself uses `./gradlew test`; HELIX TOOLBENCH-06 specifies mvn. Both absent (no JDK), both live-skip |

**Installation (per-language toolchain image / dev):**
```bash
pip install --no-index --find-links=<wheels> pytest-json-report   # Python (offline)
pnpm install --offline --frozen-lockfile                          # TS/JS (vitest+jest in node_modules)
# dotnet test / ctest / cargo test need no extra reporter package
```

## Package Legitimacy Audit

> Ran `gsd-tools query package-legitimacy check` for every external reporter dep.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `pytest-json-report` | PyPI | published 2022-03 | unknown (registry gap) | github.com/numirias/pytest-json-report | [SUS] | **Flagged** — reason is `unknown-downloads` only; mature, widely-used plugin by `numirias`. Planner: add one `checkpoint:human-verify` before pinning; not a slopsquat. |
| `pytest` | PyPI (present in env) | mature | — | github.com/pytest-dev/pytest | [OK]* | Approved (already installed) |
| `vitest` | npm | latest 4.1.9 (2026-06-15) | 70.2M/wk | github.com/vitest-dev/vitest | [SUS] | **Flagged** — reason `too-new` (it's the freshest patch of an enormous package). Pin a slightly older stable (e.g. 4.1.x) or human-verify. Not a slopsquat. |
| `jest` | npm | 30.4.2 (2026-05-09) | 45.3M/wk | github.com/jestjs/jest | [OK] | Approved |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** `pytest-json-report` (unknown-downloads), `vitest` (too-new). Both are legitimate, enormously popular packages whose SUS verdicts come from a download-count registry gap and a fresh-patch publish date respectively — NOT hallucination/slopsquat signals (both have real source repos and `npm view`/`pip index` confirm them). The planner must still insert a `checkpoint:human-verify` task before pinning each in a toolchain image, per protocol. `pytest-json-report` discovered via training + WebSearch then registry-confirmed → treat the *name* as `[ASSUMED]` until the human-verify gate.

## Architecture Patterns

### System Architecture Diagram

```
                          ┌─────────────────────────────────────────────┐
  helix-bench run         │  bench/runtime: ExpandMatrix → RunCell        │
  (benchmark, lang,       │                                               │
   mode, task, runs)      │   resolve mode→profile → spawn daemon         │
        │                 │        │                                      │
        ▼                 │        ▼                                      │
  ┌───────────────┐       │   languages.RunnerFor(benchmark, lang) ───────┼──► nil? → verify.sh fallback
  │ DATASET LOAD  │       │        │ (registry, init()-registered)        │
  │  internal-    │       │        ▼                                      │
  │  toolbench    │       │   r.Setup(ctx, repoDir)   [dep fetch/no-op]   │
  │   OR          │──────►│   r.RunTests(ctx, repoDir)                    │
  │  aider-       │ seed  │        │                                      │
  │  polyglot     │ dir   │        ▼ os/exec subprocess                   │
  │ (shallow      │       │   ┌──────────────────────────────────────┐   │
  │  clone @sha)  │       │   │ pytest --json-report  → report.json  │   │
  └───────────────┘       │   │ vitest run --reporter=json → stdout  │   │
                          │   │ jest --json           → stdout       │   │
                          │   │ mvn test              → surefire xml │   │
                          │   │ dotnet test --logger trx → *.trx     │   │
                          │   │ ctest --output-junit  → junit xml    │   │
                          │   │ cargo test            → libtest text │   │
                          │   └──────────────┬───────────────────────┘   │
                          │                  ▼ parse                      │
                          │   languages.TestOutcome{Passed(exit==0),      │
                          │     Tests[], Raw, ExitCode}                   │
                          │                  │                            │
                          └──────────────────┼────────────────────────────┘
                                             ▼
                          bench/evaluators/test_runner.Grade(post,pre)
                                             ▼
                          result.v2.json {task_success, verified_correctness,
                            compile_errors_*}  ◄── (gap: NO `language` field)
                                             ▼
                          bench/aggregator: group (task,mode) ◄── (gap: no per-language slice)
                                             ▼
                          leaderboard.md  /  (SC#1 needs per-language rows)
```

SC#3 overlay (container path, mostly unbuilt): the runner's `RunTests` subprocess would run INSIDE a `<lang>-toolchain` image via `<engine> run --network=none --rm -v repoDir:/work …` — but `bench/container/` today exposes only `PullByDigest` / `VerifyThenPull`; there is no run/exec seam yet.

### Recommended Project Structure
```
bench/languages/
├── runner.go              # interface (exists)
├── registry.go            # exists
├── coverage.go            # exists
├── go/runner.go           # TEMPLATE (exists)
├── python/runner.go       # NEW — pytest --json-report
├── typescript/runner.go   # NEW — vitest run --reporter=json
├── javascript/runner.go   # NEW — jest --json
├── java/runner.go         # NEW — mvn test (surefire xml)  [live-skip: no JDK]
├── csharp/runner.go       # NEW — dotnet test --logger trx
├── cpp/runner.go          # NEW — ctest --output-junit
├── rust/runner.go         # NEW — cargo test (libtest text parse)
└── <L>/testdata/          # NEW — committed golden reporter-output fixtures per lang

bench/datasets/aider-polyglot/
├── loader.go              # NEW — shallow clone @ pinned sha, config.json file mapping
├── pin.go                 # NEW — pinned sha constant + repo URL
├── LICENSE-AUDIT.md       # NEW — per-track sha256 + redistribution clause (SC#4)
└── fixtures/              # NEW — small committed fixture task set (hermetic adapter test)

cmd/helix-bench/verify_licenses.go   # NEW — mirror verify_tos.go
```

### Pattern 1: Clone the Go runner verbatim, swap four things
**What:** Each new runner is a copy of `go/runner.go` with only `Detect`, `Setup`, the `exec.CommandContext` argv, and the parser changed. The exit-code-authoritative gate, the ctx-cancel→infra-error split, the `*exec.ExitError` handling, and `var _ languages.LanguageRunner = (*XRunner)(nil)` conformance assertion are IDENTICAL.
**When to use:** all 7 runners.

### Pattern 2: Reporter file vs stdout
**What:** Two parse strategies depending on whether the runner writes a file or streams to stdout:
- **File-writing** (pytest `--json-report-file`, dotnet `--logger trx;LogFileName=`, ctest `--output-junit`, mvn surefire dir): pass a deterministic output path, run, then read+parse the file.
- **Stdout-streaming** (vitest `--reporter=json`, jest `--json`, cargo libtest text, go test2json): capture `cmd.Output()` and parse the bytes (the Go template's approach).
**When to use:** pick per reporter; prefer file-writing where the runner supports it (cleaner separation from build banners).

### Pattern 3: Capability declaration drives the ≥N/10 gate
**What:** `Capabilities()` statically declares the capability classes. The `coverage.Coverage(corpusRoot, benchmark, lang, declared)` gate counts how many declared capabilities have ≥1 covering `task.json` (`capability` field). A runner declaring 8 capabilities with 8 covering fixtures reports 8/10. The threshold (≥8 for Python/TS/JS/Java/Rust, ≥6 for C#/C++) is met by AUTHORING that many capability-tagged fixture tasks per language, not by code.
**When to use:** every runner needs N fixture `task.json` files under `bench/datasets/internal-toolbench/<lang>/` with distinct `capability` values.

### Pattern 4: Aider 2-attempt + stderr re-prompt
**What:** dataset-loader-only — clone `Aider-AI/polyglot-benchmark` @ pinned sha; per exercise read `.meta/config.json` `files.solution` / `files.test`; restore pristine solution stub; run the per-language test command; on fail, re-prompt the agent with the captured test stderr/stdout (attempt 2). Upstream aider hardcodes `tries=2`, `timeout=180s`. **HELIX's TOOLBENCH runner commands (mvn/vitest/dotnet/ctest) differ from aider-polyglot's own commands** (`pytest`, `./gradlew test`, `npm-test.sh`, `cpp-test.sh`, `cargo test -- --include-ignored`, `go test ./...`) — for the Aider adapter, use the dataset's native per-language commands, not the TOOLBENCH runner commands. See Open Q1.
**When to use:** the Aider adapter only.

### Anti-Patterns to Avoid
- **Inferring pass from zero failing rows.** Pitfall 4 (Go template): a compile failure is non-zero exit + empty `Tests`. `Passed` MUST be `exit==0`, never `len(failing)==0`.
- **Parsing `cargo test --message-format=json` for per-test results.** On stable it emits only `compiler-artifact`/`build-finished` JSON; per-test pass/fail is plain text. (Pitfall 2.)
- **Letting a missing toolchain go green.** A runner whose only test invokes the live toolchain becomes a vacuous SKIP when (mvn/javac/jest) is absent — exactly the Phase 81 `no_semantic` SIGKILL-vacuous-gate failure in MEMORY. Every runner needs a hermetic golden-fixture test that does NOT shell out.
- **Hardcoding `internal-toolbench` as the benchmark.** Coverage and dispatch take benchmark as a parameter (WR-04); the Aider adapter is a second benchmark axis value.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exit-code gate / ctx-as-infra split | custom per-runner | clone `go/runner.go` RunTests body | Already encodes Pitfall 4, ctx.Err()→infra, ExitError handling |
| TRX / surefire / JUnit XML parse | regex | `encoding/xml` struct unmarshal (only the fields needed, like `test2jsonEvent`) | XML structs are total + golden-testable |
| Shallow clone cache dir | new env logic | `bench/ragindex/cache.go` cacheDir() precedence | $HELIX_CACHE_DIR convention already established |
| Hard-fail license validator | new CLI scaffolding | clone `cmd/helix-bench/verify_tos.go` shape | injected-`today`, strict-decode, first-error-returns, count-asserted |
| Container run with `--network=none` | docker SDK | os/exec fixed-argv (like `engine.go` PullByDigest) | SC#1 bans the docker Go SDK; `verify-no-docker-sdk` gate enforces it |

**Key insight:** The seam (`LanguageRunner`, registry, grader, cell dispatch, coverage gate) is 100% built and proven by the Go runner. Phase 85 is overwhelmingly *fixture authoring + parser cloning*, not architecture.

## Reporter Formats (verified by running the real toolchains where present)

| Lang | Command | Output | Parse target | Per-test rows? |
|------|---------|--------|--------------|----------------|
| Python | `pytest --json-report --json-report-file=R.json` | JSON file | `tests[].nodeid` + `tests[].outcome` (`passed`/`failed`/`skipped`); `summary{passed,failed,…}`; top-level `exitcode` | yes [CITED: pytest-json-report README] |
| TS | `vitest run --reporter=json` | JSON (stdout or `--outputFile`) | `testResults[].assertionResults[].status` (`passed`/`failed`) + `.title`; `numPassedTests`/`numFailedTests` | yes [CITED: vitest docs] |
| JS | `jest --json` | JSON stdout | `testResults[].assertionResults[].status` + `.fullName`; `numPassedTests` | yes [CITED: jest CLI docs] |
| Java | `mvn test -Dsurefire.useFile=false` | `target/surefire-reports/TEST-*.xml` (JUnit) | `<testcase name= classname=>`, child `<failure>`/`<error>`/`<skipped>` absent ⇒ pass; `<testsuite tests= failures= errors=>` | yes [CITED: surefire docs] — **mvn/javac ABSENT, live-skip** |
| C# | `dotnet test --logger "trx;LogFileName=R.trx"` | TRX XML | `<UnitTestResult testName= outcome="Passed|Failed|…"/>`; `<Counters total= passed= failed=/>` | yes [VERIFIED: captured live, dotnet 8.0.416] |
| C++ | `ctest --output-junit R.xml` | JUnit XML | `<testcase name=>`, child `<failure>` ⇒ fail; `<testsuite tests= failures=>` | yes [VERIFIED: captured live, CMake 3.31.6] |
| Rust | `cargo test` (libtest) | plain text stdout | lines `test <name> ... ok|FAILED|ignored`; summary `test result: ok. N passed; M failed; …` | yes — **text, NOT message-format json** [VERIFIED: captured live, cargo 1.96.0] |

**Captured live samples (use as golden-fixture seeds):**
- TRX: `<UnitTestResult ... testName="T.UnitTest1.TestPass" ... outcome="Passed" />` and `<Counters total="1" executed="1" passed="1" failed="0" .../>`
- ctest JUnit: `<testcase name="pass_test">` / `<testcase name="fail_test"><failure …>` / `<testsuite tests=… failures=…>`
- cargo text: `test it_works ... ok` / `test it_fails ... FAILED` / `test result: FAILED. 1 passed; 1 failed; 0 ignored; …`
- cargo message-format json (proof it lacks rows): `{"reason":"compiler-artifact",…}` then `{"reason":"build-finished","success":true}` — NO per-test objects.

## Common Pitfalls

### Pitfall 1: Vacuous SKIP false-green (the dominant risk this phase)
**What goes wrong:** A runner's only test runs the live toolchain; when mvn/javac/jest/vitest is absent the test `t.Skip`s and `go test ./...` is green with zero real coverage.
**Why it happens:** This env lacks javac, mvn; jest/vitest are npx-only; container engine may be absent.
**How to avoid:** Every runner's parse + capability logic MUST have a HERMETIC test driven by a committed golden reporter-output fixture (the captured samples above) that never shells out. Live runs are an ADDITIONAL, skippable layer. Mirror the MEMORY `helix-bench-smoke-false-green` lesson and Phase 84's hermetic-vs-gated split.
**Warning signs:** a runner package whose tests all begin with `if _, err := exec.LookPath(...); err != nil { t.Skip }`.

### Pitfall 2: cargo `--message-format=json` ≠ per-test results
**What goes wrong:** Cloning the Go test2json approach for Rust by reading `--message-format=json` yields only compiler artifacts; per-test rows are silently empty, every Rust task looks like a compile-only run.
**How to avoid:** Parse libtest TEXT (`test NAME ... ok/FAILED`, `test result:` line). Exit code stays authoritative; rows are advisory. (Alternatively `cargo nextest run --message-format libtest-json`, but that adds a dep.)
**Warning signs:** Rust `TestOutcome.Tests` empty on a passing suite.

### Pitfall 3: No language axis downstream
**What goes wrong:** SC#1 wants per-language pass-rate, but `result.v2` has `task_id`+`benchmark` and NO `language` field; the aggregator groups (task,mode) and emits (mode×benchmark) rows — no language slice.
**How to avoid:** Add an additive OPTIONAL `language` field to `result.v2.schema.json` (minor bump, `additionalProperties` open — consistent with Phase 79/80 additive policy) populated from `Cell.Language`, and add a per-language grouping to the aggregator OR derive language from the `aider-polyglot/<lang>/<task>` task_id. See Open Q1.
**Warning signs:** aggregator output cannot answer "Python pass-rate".

### Pitfall 4: SC#3 container-run seam does not exist
**What goes wrong:** Planning assumes `bench/container/` can run tests with `--network=none`; it only pulls + verifies.
**How to avoid:** Add a fixed-argv `Run(ctx, image, args, mounts, network)` seam to `bench/container/` (os/exec, no docker SDK — `verify-no-docker-sdk` gate). Hermetic test asserts the argv contains `--network=none` and the mount/image WITHOUT a live engine (mirror `engine_test.go` `pullArgs` argv-assertion). Live run gated on `Detect()`.
**Warning signs:** SC#3 tasks reference a `container.Run` that isn't in the package.

### Pitfall 5: Offline dep resolution per toolchain
**What goes wrong:** `--network=none` images can't fetch deps at test time; `mvn`/`cargo`/`pnpm`/`pip` try network and fail.
**How to avoid:** Pre-resolve in the image build, then `mvn -o`, `cargo test --offline`, `pnpm install --offline --frozen-lockfile`, `pip install --no-index --find-links=<wheels>`. Flag any task that still needs network as non-hermetic in the dataset (SC#3).

## Code Examples

### Runner skeleton (clone of go/runner.go — Python shown)
```go
// Source: bench/languages/go/runner.go [VERIFIED: read source]
package python

import ( "context"; "encoding/json"; "errors"; "os"; "os/exec"; "path/filepath"
        "github.com/agenthands/helix/bench/languages" )

func init() { languages.Register("internal-toolbench", "python", &PyRunner{}) }

type PyRunner struct{}
var _ languages.LanguageRunner = (*PyRunner)(nil)

func (PyRunner) Detect(repoDir string) bool { // analog of go.mod check
    for _, m := range []string{"pyproject.toml", "setup.py", "requirements.txt"} {
        if _, err := os.Stat(filepath.Join(repoDir, m)); err == nil { return true }
    }
    return false
}
func (PyRunner) Setup(ctx context.Context, repoDir string) error { return ctx.Err() }
func (PyRunner) Capabilities() []languages.Capability { /* ≥8 of the 10 const enums */ }

func (PyRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
    out := filepath.Join(repoDir, ".report.json")
    cmd := exec.CommandContext(ctx, "pytest", "--json-report",
        "--json-report-file="+out, "-q")
    cmd.Dir = repoDir
    err := cmd.Run()
    if ctx.Err() != nil { return languages.TestOutcome{}, ctx.Err() }      // infra
    exitCode := 0
    if err != nil { var ee *exec.ExitError
        if errors.As(err, &ee) { exitCode = ee.ExitCode() } else {
            return languages.TestOutcome{}, err } }                         // infra (no pytest)
    raw, _ := os.ReadFile(out)
    return languages.TestOutcome{
        Passed: exitCode == 0, Tests: parsePytestJSON(raw), Raw: raw, ExitCode: exitCode,
    }, nil
}
```

### Hermetic golden-fixture test (the false-green guard)
```go
// parses a COMMITTED real pytest report; NO subprocess — runs everywhere.
func TestParsePytestJSONGolden(t *testing.T) {
    raw, _ := os.ReadFile("testdata/pytest_report.json") // captured real output
    got := parsePytestJSON(raw)
    // assert 1 passed + 1 failed + 1 skipped rows with correct Passed/Skipped
}
// live layer — skips cleanly when pytest absent (NOT the sole proof)
func TestRunTestsLive(t *testing.T) {
    if _, err := exec.LookPath("pytest"); err != nil { t.Skip("no pytest") }
    // ...
}
```

### Makefile hard-fail gate (mirror verify-tos)
```makefile
# Source: Makefile verify-tos: [VERIFIED]
verify-licenses:
	go run ./cmd/helix-bench verify-licenses bench/datasets/aider-polyglot/LICENSE-AUDIT.md
```

## Aider Polyglot Dataset

- **Repo:** `github.com/Aider-AI/polyglot-benchmark`; 6 langs (C++, Go, Java, JavaScript, Python, Rust); 225 exercises total (the subset of Exercism problems solved by ≤3 models) [CITED: aider.chat/2024/12/21/polyglot.html].
- **Layout:** `<lang>/exercises/practice/<exercise>/` with `.meta/config.json` carrying `files.solution` (stub the agent edits), `files.test` (test file, restored each attempt), `files.example` (reference solution under `.meta/`) [VERIFIED: fetched real config.json — `wordy` has `files.solution:[wordy.py]`, `files.test:[wordy_test.py]`, `files.example:[.meta/example.py]`]. Python track shows ~34 practice exercises.
- **Test commands (aider's own, authoritative):** Python `pytest`; Rust `cargo test -- --include-ignored`; Go `go test ./...`; Java `./gradlew test`; JS `npm-test.sh` (jest); C++ `cpp-test.sh` (cmake/ctest). Timeout 180s. [VERIFIED: fetched aider/benchmark/benchmark.py `run_unit_tests` dict].
- **2-attempt protocol:** `for i in range(tries=2)`: attempt → restore solution from pristine → run tests → on fail set `instructions = errors + test_failures prompt` and loop; `break` on pass [VERIFIED: benchmark.py].
- **Sanity pass-rates (Sonnet-class):** Claude 3.5 Sonnet 45.3% at debut (Dec 2024); Claude 3.7 Sonnet ~60.4% direct via aider; current leaders ~88% (GPT-5) [CITED: aider.chat leaderboards, epoch.ai]. Per-language sanity numbers are NOT individually published in primary sources → SC#1's "per-language" comparison needs the planner to pick a tolerance band against the aggregate, OR run a one-time baseline to establish per-language expectations (see Open Q3).

## License Audit (SC#4)

- Exercism exercise content is "copyright © Exercism, used under Exercism's open source licenses." Individual tracks each carry their OWN `LICENSE` in `github.com/exercism/<lang>` — predominantly **MIT** (Python track confirmed MIT) [CITED: github.com/exercism/python]. The polyglot-benchmark repo redistributes them with attribution.
- **LICENSE-AUDIT.md shape (mirror cost-table.yaml row discipline):** one row per track `{track, source_repo, license (SPDX), license_sha256, redistribution_clause_excerpt}`. `verify-licenses` (clone of `verify_tos.go`) strict-decodes, asserts each track has a non-empty license + sha256, and HARD-FAILS on a missing/malformed entry. Compute the sha256 over each track's committed LICENSE text at the pinned sha.

## Runtime State Inventory

> Greenfield additive phase (new packages + additive schema field). One migration-adjacent item:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `result.v2.json` artifacts under `bench/reports/*` lack a `language` field | additive schema field is backward-compatible; old artifacts read fine (field optional) — code edit only, no data migration |
| Live service config | None — verified: bench is offline, no external service stores language | none |
| OS-registered state | None | none |
| Secrets/env vars | `$HELIX_CACHE_DIR` (existing convention, reused for Aider clone cache) — no rename | none |
| Build artifacts | Per-language toolchain images (SC#3) are NEW build artifacts; pre-resolved dep caches baked at build time | build new images; flag non-hermetic tasks |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) |
| Config file | none — `go test` |
| Quick run command | `go test ./bench/languages/...` |
| Full suite command | `go test ./...` then `HELIX_BIN="$(pwd)/helix" go test ./bench/...` (per MEMORY bench-smoke gate) |

### Phase Requirements → Test Map
| Req | Behavior | Test Type | Automated Command | File Exists? |
|-----|----------|-----------|-------------------|-------------|
| TOOLBENCH-03 | pytest-json parse pass/fail/skip | unit (golden fixture) | `go test ./bench/languages/python/ -run Golden` | ❌ Wave 0 |
| TOOLBENCH-04 | vitest json parse | unit (golden) | `go test ./bench/languages/typescript/ -run Golden` | ❌ Wave 0 |
| TOOLBENCH-05 | jest json parse | unit (golden) | `go test ./bench/languages/javascript/ -run Golden` | ❌ Wave 0 |
| TOOLBENCH-06 | surefire xml parse | unit (golden) | `go test ./bench/languages/java/ -run Golden` | ❌ Wave 0 (live skips: no JDK) |
| TOOLBENCH-07 | TRX parse | unit (golden) | `go test ./bench/languages/csharp/ -run Golden` | ❌ Wave 0 |
| TOOLBENCH-08 | ctest JUnit parse | unit (golden) | `go test ./bench/languages/cpp/ -run Golden` | ❌ Wave 0 |
| TOOLBENCH-09 | cargo libtest text parse | unit (golden) | `go test ./bench/languages/rust/ -run Golden` | ❌ Wave 0 |
| TOOLBENCH-03..09 | ≥N/10 capability coverage | unit | `go test ./bench/languages/ -run Coverage` (per-lang fixtures) | ❌ Wave 0 (needs fixtures) |
| ADAPTER-AIDER-01 | config.json file mapping + 2-attempt | unit (committed fixture task set) | `go test ./bench/datasets/aider-polyglot/` | ❌ Wave 0 |
| ADAPTER-AIDER-01 | live shallow-clone @ sha | integration (network-gated) | `go test ./bench/datasets/aider-polyglot/ -run Live` (skips offline) | ❌ Wave 0 |
| SC#4 | verify-licenses hard-fail | unit + gate | `make verify-licenses` | ❌ Wave 0 |
| SC#3 | container run `--network=none` argv | unit (argv assertion, no engine) | `go test ./bench/container/ -run Network` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/languages/<lang>/` (the touched runner)
- **Per wave merge:** `go test ./...` + `HELIX_BIN=$(pwd)/helix go test ./bench/...`
- **Phase gate:** full suite green; `make verify-licenses` green; capability coverage ≥ threshold per language

### Test strategy split (the load-bearing decision)
| Layer | Drives | Skips when | Sole proof? |
|-------|--------|-----------|-------------|
| **Hermetic golden-fixture** (per runner) | committed real reporter output (the captured TRX/ctest/cargo/pytest samples) → parser | never (no toolchain needed) | YES — this is the authoritative proof of parse/capability logic |
| **Live end-to-end** (per runner) | real toolchain subprocess | toolchain absent (mvn/javac/jest/vitest/engine) | NO — additive confidence only |
| **Adapter hermetic** | committed fixture task set (few exercises, config.json) | never | YES — proves file-mapping + 2-attempt logic |
| **Adapter live** | shallow clone @ pinned sha | offline | NO |

### Wave 0 Gaps
- [ ] `bench/languages/<L>/testdata/*` — committed golden reporter outputs (7 langs; TRX/ctest/cargo samples captured in this research are ready to commit)
- [ ] `bench/datasets/internal-toolbench/<L>/IT-<L>-<cap>-1/task.json` — ≥N capability-tagged fixtures per language (drives the coverage gate)
- [ ] `bench/datasets/aider-polyglot/fixtures/` — small committed fixture task set for hermetic adapter test
- [ ] `result.v2.schema.json` additive `language` field + `bench/runtime/result.go` `Language` field
- [ ] `bench/container` `Run(--network=none)` seam + argv-assertion test
- [ ] `cmd/helix-bench/verify_licenses.go` + `Makefile verify-licenses:`

## Security Domain

> `security_enforcement` absent in config ⇒ enabled. This phase shells out to external toolchains over os/exec and clones a remote repo — the relevant controls:

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | path-segment validation already enforced by `validateCellKey`/`validatePathSegment` (cell.go); apply to exercise names + clone target dir |
| V12 Files/Resources | yes | confine clone + test exec to `$HELIX_CACHE_DIR`/scratch; mirror `bench/ragindex` `validatePath` (rejects `..`, absolute, post-Join escape) |
| V6 Cryptography | yes | per-track LICENSE sha256 (SC#4) + pinned-sha clone (commit-pinned, not tag) — never trust a mutable ref |

### Known Threat Patterns
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| os/exec argv injection (exercise name → test argv) | Tampering | fixed argv + path-segment validation; never shell (mirror `engine.go` fixed-argv + `--` terminator) |
| Malicious/changed upstream dataset | Tampering | pin clone by sha (not branch/tag); `--depth 1` shallow at the exact commit |
| Untrusted test code running outside sandbox | Elevation | SC#3 `--network=none` container; offline dep resolution; non-hermetic tasks flagged |
| Env leakage to toolchain subprocess | Info Disclosure | strict env allowlist (mirror `allowlistEnv()` PATH/HOME/HELIX_CACHE_DIR only) |

## State of the Art

| Old Approach | Current Approach | When | Impact |
|--------------|------------------|------|--------|
| `cargo test -- -Z unstable-options --format json` (nightly libtest json) | parse stable libtest text, or `cargo nextest --message-format libtest-json` | stable rust 1.70+ | Rust runner must NOT rely on `-Z`; verified rejected on 1.96.0 stable |
| ctest text / `--output-on-failure` scraping | `ctest --output-junit <file>` | CMake ≥ 3.21 | clean JUnit XML; env has 3.31.6 ✓ |

**Deprecated/outdated:** none material.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `pytest-json-report` is the canonical pytest JSON plugin (name discovered via training+WebSearch, registry-confirmed but [SUS] download-gap) | Standard Stack | low — well-known; human-verify gate before pinning |
| A2 | Exercism tracks are predominantly MIT per-track (Python confirmed MIT; others assumed-MIT pending per-repo check) | License Audit | medium — SC#4 must read each track's actual LICENSE at the pinned sha, not assume |
| A3 | Per-language sanity pass-rates are not individually published → SC#1 compares against aggregate or a self-established baseline | Aider Dataset / Open Q3 | medium — defines what "matches sanity benchmarks" means; planner must pick the comparison method |
| A4 | vitest/jest emit Jest-compatible `testResults[].assertionResults[].status` JSON | Reporter Formats | low — both documented; verify with a golden fixture during impl |
| A5 | Surefire XML shape (`<testcase>` + child `<failure>/<error>/<skipped>`) — assumed from docs, NOT captured live (no JDK in env) | Reporter Formats | medium — Java golden fixture must be sourced from a real surefire run elsewhere, not hand-fabricated |

## Open Questions

1. **No `language` axis in result.v2 / aggregator — how to slice per-language pass-rate (SC#1)?**
   - Known: schema has `task_id`+`benchmark`, no `language`; aggregator groups (task,mode), emits (mode×benchmark) rows. `Cell.Language` exists in the harness but isn't persisted.
   - **RESOLVED — recommendation:** Add an additive OPTIONAL `language` string to `result.v2.schema.json` (minor bump, `additionalProperties` stays open — consistent with the Phase 79/80 additive-only policy in STATE.md) and a `Language` field to `result.go`, populated from `Cell.Language`. Extend the aggregator with an optional per-language grouping. This is the cleanest; deriving language from `aider-polyglot/<lang>/<task>` task_id is a fallback if schema change is undesired.

2. **SC#3 container `--network=none` test execution is unbuilt — is it in scope?**
   - Known: `bench/container/` does pull+verify only; no run/exec seam.
   - **RESOLVED — recommendation:** Build a fixed-argv `container.Run(ctx, image, args, mounts, netNone)` seam (os/exec, no docker SDK — `verify-no-docker-sdk` gate holds). Prove SC#3 hermetically via an argv-assertion test (`--network=none` present, correct mount) WITHOUT a live engine; the live container run skips when `Detect()` fails. Treat the toolchain-image BUILD (offline dep pre-resolution) as the larger sub-effort; if image-build is too heavy for this phase, descope to "runner runs natively + the `--network=none` seam + argv proof + non-hermetic flagging" and defer image baking — but SC#3 names images, so confirm scope with the operator.

3. **"Per-language pass-rate matches published sanity benchmarks" — against what numbers (SC#1)?**
   - Known: aggregate Sonnet numbers exist (3.5≈45%, 3.7≈60% direct); per-language splits not in primary sources.
   - **RESOLVED — recommendation:** Define the SC#1 gate as a tolerance band against the *aggregate* published Sonnet-direct number for the pinned model, plus a committed one-time per-language baseline captured on first full run (stored like a golden) so future runs detect regression. Do NOT hard-assert exact per-language figures that aren't published.

4. **Java runner: mvn AND javac absent — can it land at all?**
   - Known: no JDK compiler, no mvn; only JRE 21 + jdtls (langregistry).
   - **RESOLVED — recommendation:** Land the Java runner with full HERMETIC coverage (golden surefire XML fixture sourced from a real run on another machine, NOT hand-fabricated — see A5) + capability fixtures; the live `mvn test` layer `t.Skip`s in this env. CI/toolchain image must provide a JDK for the live layer. This satisfies TOOLBENCH-06's parse/capability requirement without a local JDK.

5. **Rust per-test rows: text-parse vs nextest dep?**
   - **RESOLVED — recommendation:** Parse libtest TEXT (no new dep); exit code is the authoritative gate, rows advisory. `cargo nextest` is a cleaner JSON source but adds a toolchain dep to every Rust image — not worth it given rows are advisory.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| python3 / pytest | Python runner live | ✓ | 3.13.5 / pytest 8.3.5 | golden-fixture hermetic test |
| pytest-json-report | Python runner | ✗ (not installed; offline install failed) | 1.5.0 (PyPI) | golden fixture; install in image |
| node / npm / pnpm | TS/JS runner live | ✓ | 20.19.6 / 11.16 / 10.13 | golden fixture |
| vitest / jest | TS/JS runner live | ✗ global (npx) | 4.1.9 / 30.4.2 | golden fixture; node_modules in image |
| dotnet | C# runner live | ✓ | 8.0.416 | — (live TRX captured ✓) |
| g++ / cmake / ctest | C++ runner live | ✓ | 14.2.0 / 3.31.6 | — (live JUnit captured ✓) |
| cargo / rustc | Rust runner live | ✓ | 1.96.0 | — (live text captured ✓) |
| **javac (JDK compiler)** | Java runner live | ✗ | JRE 21 only | **live-skip; golden surefire fixture (A5)** |
| **mvn** | Java runner live | ✗ | — | live-skip; image provides |
| docker/podman engine | SC#3 container runs | ✗ (assume absent) | — | argv-assertion hermetic test; live run skips |
| git | Aider shallow clone | ✓ (assumed) | — | committed fixture task set hermetic test |

**Missing with no fallback:** none — every gap has a hermetic golden-fixture path.
**Missing with fallback:** javac, mvn, vitest, jest, pytest-json-report, container engine — all covered by committed golden fixtures + skippable live layers.

## Sources

### Primary (HIGH confidence — read source / ran live)
- `bench/languages/runner.go`, `go/runner.go`, `registry.go`, `coverage.go`, `go/runner_test.go` — the LanguageRunner seam + template
- `bench/evaluators/test_runner/test_runner.go` — Phase 79 grader consuming TestOutcome
- `bench/runtime/cell.go` (535, 659-698), `matrix.go` — dispatch + language axis
- `bench/container/engine.go`, `container.go`, `pull.go` — pull/verify (no run seam)
- `bench/ragindex/cache.go` — $HELIX_CACHE_DIR clone-cache pattern
- `cmd/helix-bench/verify_tos.go` + Makefile `verify-tos:`/`validate-cost-table:` — hard-gate template
- `internal/langregistry/languages.go` — LSP servers for all 7 langs (pyright, typescript-language-server, rust-analyzer, clangd, csharp-ls/OmniSharp, jdtls)
- Live runs: dotnet TRX, ctest JUnit, cargo libtest text + message-format json (captured this session)
- `gsd-tools query package-legitimacy check` — vitest/jest/pytest-json-report verdicts

### Secondary (MEDIUM — official docs / verified)
- aider/benchmark/benchmark.py `run_unit_tests` dict + 2-attempt loop [WebFetch]
- polyglot-benchmark `.meta/config.json` (`wordy`) files.solution/test/example [WebFetch]
- aider.chat leaderboards / epoch.ai — aggregate Sonnet pass-rates

### Tertiary (LOW — WebSearch only, flagged)
- Per-language sanity pass-rate splits (not in primary sources → A3/Open Q3)
- Exercism per-track license = MIT for non-Python tracks (Python confirmed; others A2)

## Metadata

**Confidence breakdown:**
- Standard stack / seam: HIGH — read every relevant source file; template is concrete
- Reporter formats: HIGH for C#/C++/Rust/Go (captured live), MEDIUM for Python/TS/JS (docs+plugin), MEDIUM for Java (docs only, no JDK to capture)
- Aider dataset: MEDIUM — benchmark.py + one config.json fetched; per-language sanity numbers LOW
- Pitfalls/gaps: HIGH — the 3 gaps (language axis, cargo json, container run) verified by reading source + running cargo

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable; codebase anchors don't move, external reporter formats stable)

## RESEARCH COMPLETE
