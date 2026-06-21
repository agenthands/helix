# Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 17 new/modified (7 runners + 7 testdata sets, adapter pkg, schema/result, aggregator, container, verify-licenses)
**Analogs found:** 17 / 17 (every file clones an in-tree, proven analog — this phase is fixture authoring + parser cloning, not architecture)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/languages/python/runner.go` | runner (LanguageRunner) | file-I/O + transform (pytest `--json-report-file` → parse) | `bench/languages/go/runner.go` | exact (template) |
| `bench/languages/typescript/runner.go` | runner | stdout streaming + transform (vitest json) | `bench/languages/go/runner.go` | exact |
| `bench/languages/javascript/runner.go` | runner | stdout streaming + transform (jest json) | `bench/languages/go/runner.go` | exact |
| `bench/languages/java/runner.go` | runner | file-I/O + transform (surefire XML); live-skip (no JDK) | `bench/languages/go/runner.go` | exact |
| `bench/languages/csharp/runner.go` | runner | file-I/O + transform (TRX XML) | `bench/languages/go/runner.go` | exact |
| `bench/languages/cpp/runner.go` | runner | file-I/O + transform (ctest JUnit XML) | `bench/languages/go/runner.go` | exact |
| `bench/languages/rust/runner.go` | runner | stdout streaming + transform (libtest TEXT) | `bench/languages/go/runner.go` | exact (parser differs: text not JSON) |
| `bench/languages/<L>/runner_test.go` ×7 | test (hermetic golden + gated live) | transform | `bench/languages/go/runner_test.go` | exact |
| `bench/languages/<L>/testdata/*` ×7 | fixture (committed reporter output) | — | RESEARCH captured TRX/ctest/cargo/pytest samples | exact (C#/C++/Rust captured live) |
| `bench/datasets/internal-toolbench/<L>/IT-<L>-<cap>-1/task.json` | fixture (coverage gate input) | — | existing `internal-toolbench/go/*/task.json` (`capability` field) | exact |
| `bench/datasets/aider-polyglot/loader.go` | service/loader | file-I/O + batch (shallow clone @sha, config.json map) | `bench/ragindex/cache.go` (cache dir); `bench/container/engine.go` (fixed-argv subprocess) | role-match |
| `bench/datasets/aider-polyglot/pin.go` | config (pinned sha const) | — | — (trivial const file) | n/a |
| `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` | data (audit table) | — | `bench/PROVIDERS.md` (row-per-entry frontmatter) | role-match |
| `bench/datasets/aider-polyglot/fixtures/` | fixture (hermetic adapter test) | — | — (new committed task set) | n/a |
| `bench/schema/result.v2.schema.json` (modify) | schema | — | the `embedder_id` additive open-key block | exact |
| `bench/runtime/result.go` (modify) | model | transform | `EmbedderID` field thread (lines 57-63, 125-129, 180) | exact |
| `bench/aggregator/*.go` (modify) | service (aggregation) | transform (per-language slice) | `reduceLeaderRow` (aggregate.go:156) mode grouping | role-match |
| `bench/container/run.go` (new) + `engine.go` (modify) | service (subprocess) | event/process (`--network=none` run) | `engine.go` `pullArgs`/`PullByDigest` (lines 43-78) | exact (same fixed-argv seam) |
| `cmd/helix-bench/verify_licenses.go` (new) | CLI validator | transform (strict-decode gate) | `cmd/helix-bench/verify_tos.go` | exact |

## Pattern Assignments

### `bench/languages/<L>/runner.go` ×7 (runner, transform) — THE PRIMARY CLONE

**Analog:** `bench/languages/go/runner.go` (160 LOC; clone verbatim, swap only `Detect`, `Setup` argv, the `exec.CommandContext` invocation, and the parser).

**init()-registration + conformance assertion** (`go/runner.go:19-27`) — IDENTICAL in all 7, swapping benchmark stays `"internal-toolbench"`, lang + runner type change:
```go
func init() { languages.Register("internal-toolbench", "go", &GoRunner{}) }
type GoRunner struct{}
var _ languages.LanguageRunner = (*GoRunner)(nil)
```

**Detect** (`go/runner.go:30-33`) — swap the marker file(s) per language (Python: `pyproject.toml`/`setup.py`/`requirements.txt`; Rust: `Cargo.toml`; C#: `*.csproj`/`*.sln`; C++: `CMakeLists.txt`; Java: `pom.xml`; TS: `package.json`+`tsconfig.json`; JS: `package.json`):
```go
func (GoRunner) Detect(repoDir string) bool {
	_, err := os.Stat(filepath.Join(repoDir, "go.mod"))
	return err == nil
}
```

**Setup** (`go/runner.go:41-43`) — even a no-op MUST consult ctx (WR-05). Runners needing dep fetch (cargo fetch, pnpm install) replace the body but keep the ctx-first discipline:
```go
func (GoRunner) Setup(ctx context.Context, repoDir string) error {
	return ctx.Err()
}
```

**RunTests — the exit-code-authoritative gate + ctx→infra split** (`go/runner.go:70-104`) — this skeleton is IDENTICAL across all 7; only the `exec.CommandContext` argv and the `parseX(raw)` call change. Do NOT alter the `ctx.Err()` check, the `*exec.ExitError` branch, or `Passed: exitCode == 0`:
```go
func (GoRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "./...", "-json")
	cmd.Dir = repoDir
	raw, err := cmd.Output()
	if ctx.Err() != nil {
		return languages.TestOutcome{Raw: raw}, ctx.Err()   // infra, not a task outcome
	}
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return languages.TestOutcome{Raw: raw}, err       // non-ExitError (binary missing) = infra
		}
	}
	tests := parseTest2JSON(raw)
	return languages.TestOutcome{
		Passed: exitCode == 0, Tests: tests, Raw: raw, ExitCode: exitCode,
	}, nil
}
```

**Per-language argv (RESEARCH §Reporter Formats — VERIFIED live for C#/C++/Rust):**
- Python: `pytest --json-report --json-report-file=<repoDir>/.report.json -q` → parse JSON file `tests[].nodeid` + `.outcome`
- TS: `vitest run --reporter=json` → parse stdout `testResults[].assertionResults[].status`
- JS: `jest --json` → parse stdout `testResults[].assertionResults[].status`
- Java: `mvn test -Dsurefire.useFile=false` → parse `target/surefire-reports/TEST-*.xml` (`<testcase>` + child `<failure>/<error>/<skipped>`). **Live-skips: no JDK/mvn in env (A5 — golden surefire fixture must come from a real run elsewhere, NOT hand-fabricated).**
- C#: `dotnet test --logger "trx;LogFileName=R.trx"` → parse TRX `<UnitTestResult testName= outcome="Passed|Failed">`
- C++: `ctest --output-junit R.xml` → parse JUnit `<testcase>` + child `<failure>`
- **Rust: `cargo test` → parse libtest TEXT `test NAME ... ok|FAILED|ignored` + `test result:` line. DO NOT use `--message-format=json` (compiler-artifact only, no per-test rows — Pitfall 2).**

**Malformed-line-resync parser discipline** (`go/runner.go:127-159`) — for line-oriented streams (Rust libtest text, jest/vitest if line-delimited), mirror the `bufio.Scanner` + skip-and-resync (a bad line is skipped, never aborts the stream; rows are advisory because `Passed` is gated on exit code):
```go
scan := bufio.NewScanner(bytes.NewReader(raw))
scan.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)   // grow beyond 64KiB default
for scan.Scan() {
	// ... unmarshal/match; on error: continue (re-sync, never abort)
}
```
For XML reporters (TRX/surefire/ctest) use `encoding/xml` struct unmarshal of ONLY the needed fields (mirror `test2jsonEvent`'s minimal-struct discipline at `go/runner.go:108-113`), NOT regex.

**Capabilities** (`go/runner.go:46-59`) — Go declares all 10; new runners declare ≥8 (Python/TS/JS/Java/Rust) or ≥6 (C#/C++). The capability CONSTANTS are the fixed enum in `bench/languages/runner.go:25-36` (string values are source-of-truth for `task.json`).

---

### `bench/languages/<L>/runner_test.go` ×7 (test) — the false-green guard

**Analog:** `bench/languages/go/runner_test.go` + RESEARCH §Code Examples.

Two layers (Pitfall 1 — the dominant risk this phase). The hermetic golden test is the SOLE authoritative proof; the live test is additive and skippable:
```go
// HERMETIC — parses committed real reporter output, NO subprocess, runs everywhere
func TestParsePytestJSONGolden(t *testing.T) {
	raw, _ := os.ReadFile("testdata/pytest_report.json")
	got := parsePytestJSON(raw)
	// assert pass/fail/skip rows with correct Passed/Skipped
}
// LIVE — additive only; skips cleanly when toolchain absent
func TestRunTestsLive(t *testing.T) {
	if _, err := exec.LookPath("pytest"); err != nil { t.Skip("no pytest") }
}
```
**Anti-pattern (forbidden):** a runner package whose tests ALL begin with `exec.LookPath(...)` skip guards → vacuous SKIP, false-green (the Phase 81 `no_semantic` SIGKILL-vacuous-gate failure in MEMORY).

---

### `bench/datasets/internal-toolbench/<L>/IT-<L>-<cap>-1/task.json` (fixture) — drives the ≥N/10 gate

**Analog:** existing `internal-toolbench/go/*/task.json`; gate logic in `bench/languages/coverage.go:37-72`.

The coverage gate counts DISTINCT `capability` values across `task.json` files (decodes ONLY the `capability` field — coverage.go:96-104, NOT the task id). Meeting ≥8 (or ≥6) is achieved by AUTHORING that many capability-tagged fixtures per language, not by code. Each fixture needs a distinct `capability` from the enum (`bench/languages/runner.go:25-36`). `Coverage(corpusRoot, benchmark, lang, declared)` takes benchmark as an explicit param (WR-04) — never hardcode `internal-toolbench` downstream.

---

### `bench/schema/result.v2.schema.json` + `bench/runtime/result.go` (modify) — additive `language` field

**Analog (exact template):** the `embedder_id` open-key thread in `result.go`. Clone it for `language`.

`result.go` has the field in THREE places — replicate all three for `Language`:
1. `ResultInput` open-provenance field (result.go:57-63, the `EmbedderID` block) — add `Language string`.
2. `resultDoc` json-tagged field WITH omitempty (result.go:125-129):
```go
EmbedderID string `json:"embedder_id,omitempty"`
// → add:
Language   string `json:"language,omitempty"`
```
3. `BuildResult` doc assembly (result.go:180, `EmbedderID: in.EmbedderID`) — add `Language: in.Language`.

Populate from `Cell.Language` (the field already exists at `bench/runtime/cell.go:189`; thread it into `ResultInput` at the cell's BuildResult call site). Schema: top-level `additionalProperties` is OPEN, so this is additive-minor (NO v3 bump) — mirror the `embedder_id` schema doc entry exactly. Document `language` as optional/omitempty (old artifacts without it still validate — Runtime State Inventory).

---

### `bench/aggregator/*.go` (modify) — per-language slice

**Analog:** `reduceLeaderRow` mode-grouping in `aggregate.go:156` + `Loaded.Modes` enumeration in `load.go:85`.

Today the aggregator hardcodes `Benchmark: "internal-toolbench"` (aggregate.go:157, 223) and groups by (task, mode); it emits (mode × benchmark) rows. For SC#1 add an OPTIONAL per-language grouping: read `language` from the preserved `Row.Doc` map (load.go:48 `Doc map[string]json.RawMessage` — same access pattern `rowModelID` uses at aggregate.go:292 to read `model_id`), and add a per-language reduction analogous to `reduceLeaderRow`. Fallback if schema change is undesired: derive language from the `aider-polyglot/<lang>/<task>` task_id (Pitfall 3 / Open Q1). Add `language` to `rowMetrics`/`Row` decode the same way `model_id` is read open.

---

### `bench/container/run.go` (new) + `engine.go` (modify) — `--network=none` seam

**Analog (exact):** `engine.go` `pullArgs` (lines 43-55) + `PullByDigest` (lines 62-78). The run seam is the SAME fixed-argv + strict-env shape; the seam exists for pull/verify only (Pitfall 4 — there is no run/exec today).

Build a `runArgs(image, args, mounts, netNone)` pure-argv builder (mirror `pullArgs` returning `[]string, error`, fail-closing on bad inputs BEFORE crossing os/exec) and a `Run(ctx, ...)` (mirror `PullByDigest:62-78`):
```go
// mirror PullByDigest exactly:
cmd := exec.CommandContext(ctx, e.bin, args...)
cmd.SysProcAttr = procGroupAttr()   // group-kill on ctx cancel
cmd.Env = allowlistEnv()            // PATH/HOME/HELIX_CACHE_DIR ONLY (engine.go:83-91)
```
Argv MUST include `--network=none`, `--rm`, `-v <repoDir>:/work` and use `--` to terminate option parsing (engine.go:54). NO docker Go SDK (the `verify-no-docker-sdk` gate enforces it — RESEARCH §Don't Hand-Roll). Hermetic test asserts the argv contains `--network=none` + correct mount/image WITHOUT a live engine (mirror `engine_test.go` `pullArgs` argv-assertion); live `Run` skips on `Detect()` → `errEngineUnavailable` (engine.go:21,29-36).

---

### `bench/datasets/aider-polyglot/loader.go` + `pin.go` (new) — dataset-loader-only adapter

**Analogs:** `bench/ragindex/cache.go` `cacheDir()` (clone cache dir) + `bench/container/engine.go` (fixed-argv subprocess + env allowlist for the git invocation).

Cache dir precedence — clone the `cacheDir()` three-step exactly (cache.go:32-41): `$HELIX_CACHE_DIR` → `os.UserCacheDir()/helix` → `~/.helix/cache`. Sub-segment e.g. `aider-polyglot/<pinned_sha>/` (sha is hex, no separators — cannot escape the root, cache.go:84-89 path-safety note).

Clone via os/exec fixed argv (mirror `PullByDigest` discipline): `git clone --depth 1` then fetch+checkout the pinned sha (pin by SHA, never a branch/tag — Security Threat: malicious upstream). `pin.go` holds the repo URL + pinned sha CONSTANT (mirror the `engineCandidates`/sentinel const style).

Loader logic (RESEARCH §Pattern 4): per exercise read `.meta/config.json` `files.solution`/`files.test`/`files.example`; restore pristine solution stub; 2-attempt protocol (`tries=2`, 180s) with stderr re-prompt on attempt 2. Validate exercise names with path-segment validation (mirror `validatePathSegment`/`validateCellKey` from cell.go — V5/V12 controls). For the Aider adapter use the dataset's NATIVE per-language commands (`pytest`, `cargo test -- --include-ignored`, `./gradlew test`, jest, cmake/ctest), NOT the TOOLBENCH runner commands.

Hermetic adapter test drives a committed small fixture task set under `fixtures/` (proves file-mapping + 2-attempt); live shallow-clone test is network-gated and skips offline.

---

### `cmd/helix-bench/verify_licenses.go` (new) + Makefile `verify-licenses:` — SC#4 gate

**Analog (exact clone):** `cmd/helix-bench/verify_tos.go`.

Clone the strict-decode hard-fail shape:
- INJECTED `today`-style determinism where applicable; strict-decode with `dec.KnownFields(true)` (verify_tos.go:71) → unknown key = error = non-zero exit.
- First-error-returns; fail-CLOSED on a malformed/missing entry (verify_tos.go:73-77).
- COUNT-asserted: return `(count, error)` so a test proves the count is checked, not just that the file passes (verify_tos.go `verifyTOSCount`, lines 41-103, the CR-01 discipline) — assert `count == 0` is itself an error (verify_tos.go:99-101).
- cobra subcommand NOT added to the root tree (count-fixed); invoked by the Makefile (verify_tos.go:150-172).

`LICENSE-AUDIT.md` shape (RESEARCH §License Audit): one row per Exercism track `{track, source_repo, license (SPDX), license_sha256, redistribution_clause_excerpt}`; validator asserts each track has a non-empty license + sha256. Compute sha256 over each track's committed LICENSE at the pinned sha (A2 — read each track's ACTUAL license, don't assume MIT).

Makefile target (RESEARCH §Code Examples):
```makefile
verify-licenses:
	go run ./cmd/helix-bench verify-licenses bench/datasets/aider-polyglot/LICENSE-AUDIT.md
```

## Shared Patterns

### ctx → infra-error vs exit-code split (the authoritative gate)
**Source:** `bench/languages/go/runner.go:70-104`
**Apply to:** all 7 runners + `container.Run` + Aider loader subprocesses.
`ctx.Err() != nil` → return error (infra/cancellation). `*exec.ExitError` → record `ExitCode`, NOT an error. Non-ExitError (binary missing) → infra error. `Passed = (exitCode == 0)`, NEVER inferred from `len(failing)==0` (Pitfall 4).

### Fixed-argv + strict env allowlist subprocess
**Source:** `bench/container/engine.go:43-91` (`pullArgs`, `allowlistEnv`, `procGroupAttr`)
**Apply to:** `container.Run`, Aider `git clone`, every runner's `exec.CommandContext`.
Fixed argv (never a shell), `--` to terminate option parsing, env = PATH/HOME/HELIX_CACHE_DIR only, fail-close validation BEFORE crossing the os/exec boundary, process-group ownership for ctx group-kill.

### Additive open-key (no schema major bump)
**Source:** `bench/runtime/result.go` `EmbedderID` (lines 57-63, 125-129, 180) + `result.v2.schema.json` open `additionalProperties`
**Apply to:** the new `language` field (and any further provenance keys). omitempty so absent old artifacts still validate; minor schema bump only.

### Hermetic-golden-first, live-skippable-second test split
**Source:** RESEARCH §Code Examples; `go/runner_test.go`
**Apply to:** all 7 runners, the Aider adapter, and `container.Run`.
The committed-fixture/argv-assertion layer is the SOLE proof and runs everywhere; the live-toolchain/live-engine layer is additive and `t.Skip`s cleanly when absent. Never let a missing toolchain go green.

### Coverage gate driven by `capability` field (not task id)
**Source:** `bench/languages/coverage.go:37-104`
**Apply to:** every language's fixture authoring; benchmark is an explicit param (WR-04), capability enum is `runner.go:25-36`.

### Path-segment validation (untrusted exercise / clone names)
**Source:** `bench/runtime/cell.go` `validateCellKey`/`validatePathSegment`; `bench/ragindex` `validatePath`
**Apply to:** Aider exercise names, clone target dir, container mount paths (V5/V12 — reject `..`, absolute, post-Join escape).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `bench/datasets/aider-polyglot/fixtures/` (content) | fixture | — | New committed task set; shape derived from real `.meta/config.json` (RESEARCH `wordy` sample), no in-tree analog dataset to copy |
| Java `testdata/surefire-*.xml` | fixture | — | No JDK/mvn in env (A5) — must be sourced from a real surefire run elsewhere, NOT hand-fabricated; all other lang fixtures captured live in RESEARCH |
| Per-language toolchain IMAGES (SC#3 offline dep pre-resolution) | build artifact | — | Greenfield; if image-build is too heavy, descope to native-run + `--network=none` seam + argv proof + non-hermetic flagging and confirm SC#3 image scope with operator (Open Q2) |

## Metadata

**Analog search scope:** `bench/languages/`, `bench/runtime/`, `bench/aggregator/`, `bench/container/`, `bench/ragindex/`, `bench/schema/`, `cmd/helix-bench/`
**Files scanned (read in full or targeted):** go/runner.go, runner.go, registry.go, coverage.go, ragindex/cache.go, verify_tos.go, container/engine.go, runtime/result.go, aggregator/aggregate.go+load.go, runtime/cell.go (grep), test_runner.go (grep)
**Pattern extraction date:** 2026-06-21
