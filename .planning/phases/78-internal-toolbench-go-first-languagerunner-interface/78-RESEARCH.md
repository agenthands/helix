# Phase 78: Internal ToolBench — Go First + LanguageRunner Interface - Research

**Researched:** 2026-06-17
**Domain:** Go test/benchmark infrastructure (bench runtime extension, capability-fixture corpus, runner-interface design)
**Confidence:** HIGH (all claims verified against the live codebase via Read/grep/SMTC + a real `go test -json` run)

## Summary

Phase 78 extends the already-operational Phase 77 bench spine (`bench/runtime/`) along three orthogonal seams, all confirmed present and additive in the current tree: (1) a **language axis** on the matrix (`Cell.Language` + `<lang>` path segment + `--languages` flag), (2) a **`LanguageRunner` interface** in `bench/languages/` (currently a bare `.gitkeep`) whose `RunTests` wraps `go test ./... -json` and replaces the Phase 77 `verify.sh` outcome path while keeping `verify.sh` as a runner-less fallback, and (3) a **10-fixture Go capability corpus** under `bench/datasets/internal-toolbench/go/`, each a scripted "solve-then-`go test`" task that calls the capability's Helix MCP tool by construction.

The single load-bearing technical seam is D-03 (the store-on working-directory). I verified the exact mechanism: `internal/eval/sandbox/sandbox.go:StartDaemon` (line 231) builds `exec.CommandContext` but **never sets `cmd.Dir`**, so the cell daemon inherits the harness CWD; `internal/semantic/store/duckdb.go:Open` (line 188) **rejects absolute paths** (`filepath.IsAbs` → `serr.ErrInvalidArgs`) and the default store path is the workspace-relative `.helix/semantic.duckdb` (`internal/config/defaults.go:56`); the daemon opens it **eagerly at startup** when `SemanticIndex.Enabled` (`internal/daemon/daemon.go:317-318`). The cleanest additive fix — **set `cmd.Dir` to the per-cell repo dir** — is therefore correct and is recommended below as a functional-option extension to eval's `StartDaemon` (non-breaking, non-forking).

A second high-value finding de-risks the read/edit fixtures: the **GuardrailMiddleware is installed** (`daemon.go:845`) and gates the six destructive tools, BUT (a) the default enforcement level is `LevelWarn` (forwards the call + attaches a warning — never blocks) and (b) the G-001 public-API/referenced-symbol rule is a documented **no-op stub in production** (`daemon.go:859` — "structurally Allow"). So store-off scripted edits on referenced symbols are NOT blocked this phase; receipts are optional. This means D-05 read-capability fixtures force tool use through the **`go test` assertion design**, not through guardrail gating.

**Primary recommendation:** Extend eval `StartDaemon` with a variadic functional-option (`WithWorkingDir(dir)`) that sets `cmd.Dir`; thread it through `subprocess.StartDaemon` and gate it behind a per-cell store-opt-in derived from `task.json` `capability == "incremental update"`. Build the 10 fixtures as Go modules whose `go test` cases fail unless the scripted agent consumed the named tool's full output. Migrate the seed via `git mv`, flip `toolbench-go → internal-toolbench` defaults in one clean cutover (Makefile + main.go + LICENSES.md + test literals), and keep `make bench-quick` green by updating its hardcoded `--tasks=sum-doubler` to the seed's new `IT-go-*` task dir name.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Matrix language axis | bench runtime (`matrix.go`) | `cmd/helix-bench` flag | Cell expansion + path join own the `<lang>` segment; the CLI exposes `--languages` |
| Per-cell daemon working dir | eval sandbox (`StartDaemon`) | bench `subprocess` → `cell.go` | Store path is CWD-relative; only the process spawner can set `cmd.Dir` |
| Test outcome (structured) | `bench/languages/go` runner | `cell.go` dispatch | `RunTests` parses `go test -json`; richer than `verify.sh` exit code |
| Capability fixtures | `bench/datasets/internal-toolbench/go/<task>` | scripted_agent.yaml + Helix MCP tools | Fixtures are data; the daemon's kernel tools do the work |
| Coverage report | runner `Capabilities()` (declared) × `task.json` `capability` (covered) | `CAPABILITIES.md` (human doc) | D-06/D-11 decouple coverage logic from ID strings |
| Semantic store (incremental update only) | `internal/semantic` daemon subsystem | per-cell CWD (D-03) | Only this capability needs the Phase 70 overlay-drain refresh |

## Project Constraints (from CLAUDE.md)

- **Go only, single binary, CGO=1.** No new language toolchains, no containers, no Python in `bench/languages/go`.
- **Run `go vet ./...` and `go test ./...` before completing any Go task.** Criterion #3 explicitly requires `go vet` + the conformance test to pass.
- **`gofmt -w .`** on all new Go files.
- **SMTC-first for semantic Go questions** when a symbol name is known (used during this research; planner/executors should too).
- **GSD workflow enforcement** — edits go through a GSD command.
- **`make bench-quick` is a hard CI gate** (BENCH-05: exits 0, ≥1 task succeeds, ≤90s, no network/API key). D-09 requires it stays green through the cutover.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Targeted semantic-index enablement. Semantic index stays **disabled by default** per cell (fast live-LSP / tree-sitter / RepoMap path, as Phase 77 cell.go does via `semantic_index.enabled=false`). Store-dependent fixtures **opt in** per cell. 9/10 capabilities work store-off; only incremental update structurally needs the store.
- **D-02:** **Only the `incremental update` capability fixture(s) enable the persistent store.** Everything else runs store-off. (A call-graph/dependency-graph fixture MAY opt in if found deterministically un-testable without the store — but default rule is store-on only when the capability cannot be exercised without it.)
- **D-03:** The store-enable mechanism is a **per-cell daemon working-directory**. The store rejects absolute paths (T-57-02-01) and is opened eagerly at daemon startup relative to daemon CWD (`daemon.go:308/318`), so without isolation every parallel cell opens the SAME `.helix/semantic.duckdb` and deadlocks on its lock. The fix: a **private per-cell working directory**. Requires an **additive extension to eval's `StartDaemon`** to accept a working-dir (extending, not forking — within P77 D-07). Gated behind the opt-in. **Planner: confirm the cleanest additive seam (extend eval `StartDaemon` signature vs a sandbox-level wrapper that sets `cmd.Dir`).**
- **D-04:** Uniform "solve-then-`go test`" fixtures. Every capability is a task the scripted agent solves; code state verified by `go test` (reuse Phase 77 outcome path). ONE fixture shape — no golden-output diffing.
- **D-05:** Read/query capabilities engineered so correct completion **REQUIRES consuming the tool's full output** (e.g. "add a nil-check to every caller of F" — miss one and a `go test` case fails). The fixture's `scripted_agent.yaml` **explicitly calls the capability's tool**. The "did it use the right tool" signal for real-`claude` comes from the merged trace (Phase 79) — NOT graded here.
- **D-06:** Capability recorded as an explicit `capability` enum field in `task.json`. Runner aggregates per-language coverage by reading the `capability` field. `CAPABILITIES.md` is the human-facing doc. The `IT-go-<capability>-<n>` task ID **mirrors** the field for readability but is **NOT** the source of truth.
- **D-07:** Add a language axis. On-disk layout `bench/datasets/internal-toolbench/<lang>/<task>`. Benchmark name `internal-toolbench`; `Cell` gains a `Language` field; `ExpandMatrix` + `validateMatrixID` extend to the language segment; `helix-bench run` gains a `--languages` flag (default `go`). Seed-dir join becomes `<DatasetsRoot>/<benchmark>/<lang>/<task>`. Segments stay slash-free for V5 validators.
- **D-08:** Migrate the Phase 77 seed via `git mv` from `bench/datasets/toolbench-go/sum-doubler` into `bench/datasets/internal-toolbench/go/<task>`. Assign an `IT-go-<capability>-<n>` ID and a `capability` tag (its `replace_in_file` edit→`go test` shape maps to **patch apply** or **fuzzy search** — Claude's discretion).
- **D-09:** Flip `toolbench-go` → `internal-toolbench` defaults THIS phase: `Makefile` (`SUITE ?=`), `cmd/helix-bench/main.go` (`--benchmarks` default), `bench/BENCH.md`, cell.go/matrix.go doc-comment examples. **Hard constraint: `make bench-quick` stays green throughout** (single clean cutover, no stale `toolbench-go`). Update `bench/LICENSES.md` if its `internal-toolbench-go` row needs reconciling.
- **D-10:** `RunTests` is the structured outcome source; `verify.sh` becomes a fallback. The cell dispatches by language to a registered `LanguageRunner`; `RunTests` runs `go test ./... -json`, parses it, returns structured pass/fail + per-test result. For `internal-toolbench` cells, `RunTests` **replaces** the Phase 77 `runVerify(verify.sh)` path. `verify.sh` is **preserved as an escape hatch** (cell falls back when no runner registered). Swap must keep `bench-quick` green.
- **D-11:** `Capabilities()` returns a STATIC per-language declaration (Go = all 10). Coverage report cross-references **declared** set against **fixtures actually present** (`task.json` `capability` fields), so "declared-but-missing" gaps are explicit.

### Claude's Discretion
- Exact `LanguageRunner` method **signatures**, the `RunTests` return type / `Capabilities` value type, and the `bench/languages/` package layout (D-10/D-11 fix semantics, not Go syntax). Keep minimal; design for additive Phase 85 conformance.
- The **per-capability fixture designs** (10 Go modules + their `scripted_agent.yaml` tool sequences and `go test` assertions), within the D-04/D-05 shape.
- The seed task's exact `capability` assignment and its `IT-go-*` index (D-08).
- `PHASE67_CROSSWALK.md` scope: an **inspiration-mapping doc only** — no code migration; namespaces deliberately disjoint.
- Whether the store opt-in knob lives in `task.json` (e.g. `semantic_index: true`) or is derived from the `capability` value (incremental-update ⇒ store-on) — pick the least-surprising.

### Deferred Ideas (OUT OF SCOPE)
- The other 7 LanguageRunners + their fixtures (Python, TS, JS, Java, C#, C++, Rust) — **Phase 85**.
- Evaluators + the 17 result-schema metrics — **Phase 79** (consumes the structured `go test -json` output).
- Golden-output assertion mechanism for pure-read capabilities — explicitly NOT built (D-04 chose uniform solve-then-`go test`).
- Container runtime / per-language toolchain images / GHCR mirror — Phases 84–88.
- Multi-run repetition / pass@k path threading (`<task>/<mode>/<run_index>/`) — Phase 79/82 (P77 IN-05 single-rep-per-OutDir still holds).
- Relaxing the semantic store's absolute-path rejection (T-57-02-01) — out of scope; D-03 uses the working-dir route.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TOOLBENCH-01 | 10 capability test classes under `bench/datasets/internal-toolbench/`; each ≥1 deterministic test per supported language; documented in `CAPABILITIES.md` | Capability→tool mapping table below; all 10 classes mapped to live MCP tools verified in `internal/kernel/*` + `internal/skill/semantic`; fixture shape from the verified seed template |
| TOOLBENCH-02 | Go coverage — all 10 capabilities ≥1 fixture under `internal-toolbench/go/`; `bench/languages/go/runner.go` wraps `go test ./... -json`; per-language coverage reported | `go test -json` output verified (test2json events); `bench/languages/` confirmed empty (`.gitkeep`); coverage logic = `task.json.capability` × `Capabilities()` (D-06/D-11) |
| TOOLBENCH-10 | `bench/languages/<L>/runner.go` implements `LanguageRunner` (`Detect`, `Setup`, `RunTests`, `Capabilities`); `go vet` + conformance test passes | Interface is greenfield (no prior runner); recommended minimal signatures below; conformance test = compile-time `var _ LanguageRunner = (*GoRunner)(nil)` + behavioral test on the seed |
</phase_requirements>

## Standard Stack

No new external packages. Everything is in-tree or stdlib.

### Core
| Library / Package | Version | Purpose | Why Standard |
|-------------------|---------|---------|--------------|
| `encoding/json` (stdlib) | go 1.23 | Parse `go test -json` test2json events; `task.json` `capability` field | The runner output is line-delimited JSON; decode per line |
| `os/exec` (stdlib) | go 1.23 | `RunTests` shells `go test ./... -json`; eval `StartDaemon` already uses `exec.CommandContext` | Mirrors existing `runVerify` / `StartDaemon` patterns |
| `gopkg.in/yaml.v3` | (in `go.mod`) | `scripted_agent.yaml` decode via `runner.LoadScript` (strict `KnownFields`) | Already the scripted-agent format; fixtures conform to `runner.ScriptedStep` |
| `bench/runtime` (in-tree) | — | `Cell`, `RunCell`, `ExpandMatrix`, `RunMatrix`, `driveScript` | The Phase 77 spine being extended |
| `internal/eval/sandbox` (in-tree) | — | `Sandbox.StartDaemon` — the D-03 working-dir extension target | Reuse-don't-fork (P75 D-08, P77 D-07) |

**Installation:** none. `go build ./cmd/helix ./cmd/helix-bench` and `go test ./bench/...` are the verification commands.

### `go test -json` format (VERIFIED by running it on the seed)
`go test ./... -json` emits one JSON object per line (the `test2json` event stream):
```json
{"Time":"...","Action":"start","Package":"toolbenchseed/sumdoubler"}
{"Time":"...","Action":"run","Package":"...","Test":"TestDouble"}
{"Time":"...","Action":"output","Package":"...","Test":"TestDouble","Output":"=== RUN   TestDouble\n"}
{"Time":"...","Action":"fail","Package":"...","Test":"TestDouble","Elapsed":0.0}
```
`Action` ∈ {`start`,`run`,`pass`,`fail`,`skip`,`output`,`bench`,`pause`,`cont`}. The per-test pass/fail rows (`Action`+`Test`+`Elapsed`) are exactly what `RunTests` parses into a structured per-test result for Phase 79's `test_runner` evaluator + `compile_errors_before/after`. `[VERIFIED: live `go test ./... -json` run, 2026-06-17]`

## Architecture Patterns

### System Architecture Diagram

```
helix-bench run --benchmarks=internal-toolbench --languages=go
        │
        ▼
ExpandMatrix(benchmarks, languages, modes, tasks)  ──►  []Cell{Benchmark,Language,Mode,Task}   (matrix.go, V5-validated)
        │
        ▼  per cell (sem-bounded by --parallel)
RunCell(CellConfig{...Language, StoreOptIn})       (cell.go)
        │
        ├─ resolve mode→profile (bench-full)
        ├─ sandbox.New + Prepare + CloneRepo  (seed = <DatasetsRoot>/<benchmark>/<lang>/<task>)
        ├─ writeCellConfig → semantic_index.enabled = StoreOptIn   (D-01/D-02)
        ├─ subprocess.StartDaemon(... WithWorkingDir(repoDir))     (D-03; cmd.Dir = per-cell repo)
        │        │
        │        ▼  daemon opens .helix/semantic.duckdb relative to cmd.Dir  → per-cell store, no lock collision
        ├─ driveScript(socket, scripted_agent.yaml)  ──► MCP tools/call: get_call_hierarchy / replace_in_file / …
        ├─ h.Kill() ; trace.TapDaemonLog(pid)        (PID-gated, criterion #4)
        ├─ OUTCOME:  LanguageRunner.RunTests(repoDir)  ──► go test ./... -json ──► structured pass/fail
        │            (fallback: runVerify(verify.sh) when no runner registered)   (D-10)
        ├─ SynthCCTap + trace.Merge  (2-leg)
        └─ BuildResult + Validate → result.v2.json    (<OutDir>/<task>/<mode>/)

bench/languages/                     (the Phase 85 seam)
  ├─ runner.go        LanguageRunner interface  (Detect, Setup, RunTests, Capabilities)
  ├─ registry.go      RunnerFor(benchmark, lang) → LanguageRunner | nil  (nil ⇒ verify.sh fallback)
  └─ go/runner.go     GoRunner: Capabilities() = all 10; RunTests wraps `go test ./... -json`
```

### Recommended Project Structure
```
bench/
├── languages/
│   ├── runner.go            # LanguageRunner interface + RunTests result types + registry
│   └── go/
│       ├── runner.go        # GoRunner: Detect/Setup/RunTests/Capabilities
│       └── runner_test.go   # conformance (compile-time + behavioral) test
└── datasets/
    └── internal-toolbench/
        ├── CAPABILITIES.md          # criterion #1 (10 classes documented)
        ├── PHASE67_CROSSWALK.md     # criterion #4 (inspiration map, no code migration)
        └── go/
            ├── IT-go-patch-apply-1/        # the migrated seed (git mv from toolbench-go/sum-doubler)
            ├── IT-go-call-graph-1/
            ├── IT-go-semantic-view-1/
            ├── IT-go-lsp-diagnostics-1/
            ├── IT-go-rename-safety-1/
            ├── IT-go-fuzzy-search-1/
            ├── IT-go-dependency-graph-1/
            ├── IT-go-context-min-1/
            ├── IT-go-incremental-update-1/   # the ONE store-on fixture (D-02)
            └── IT-go-failure-handling-1/
```
Each `<task>/` mirrors the verified seed file set: `task.json`, `scripted_agent.yaml`, `verify.sh` (retained as fallback), `<module>.go`, `<module>_test.go`, `go.mod`.

### Pattern 1: Additive working-dir extension to eval `StartDaemon` (D-03)
**What:** Add a functional-option parameter so eval's `StartDaemon` can set `cmd.Dir` without changing existing call sites.
**When to use:** The store-on cell (incremental update). Store-off cells pass no option and are unaffected.
**Why this is the cleanest additive seam (recommendation):** `internal/eval/sandbox/sandbox.go:231` `StartDaemon(ctx, taskID, mode, profileName, cfgPath)` builds `cmd := exec.CommandContext(...)` at line 244 and **never assigns `cmd.Dir`**. A variadic option is purely additive (existing 5-arg callers compile unchanged); a separate "sandbox wrapper that sets cmd.Dir" would have to duplicate the entire spawn body (env allowlist, Setpgid, log redirection, socket wait) — that is a fork, which P77 D-07 forbids. Verified: `bench/runtime/subprocess/daemon.go:41` delegates straight into `sb.StartDaemon(...)`, so the option threads through one extra parameter.
```go
// Source: recommended extension to internal/eval/sandbox/sandbox.go (verified current signature line 231)
type daemonOpts struct{ workDir string }
type DaemonOption func(*daemonOpts)

// WithWorkingDir sets the daemon process working directory (cmd.Dir). When unset,
// the daemon inherits the harness CWD (the Phase 77 behavior — unchanged).
func WithWorkingDir(dir string) DaemonOption { return func(o *daemonOpts) { o.workDir = dir } }

func (s *Sandbox) StartDaemon(ctx context.Context, taskID, mode, profileName, cfgPath string, opts ...DaemonOption) (*DaemonHandle, error) {
    var o daemonOpts
    for _, fn := range opts { fn(&o) }
    // ... existing body ...
    cmd := exec.CommandContext(ctx, s.helixBin, args...)
    if o.workDir != "" { cmd.Dir = o.workDir }   // ← the ONE new line of behavior
    // ... unchanged: SysProcAttr Setpgid, env allowlist, log redirect, waitSocket ...
}
```
**Store-path consequence (VERIFIED):** With `cmd.Dir = <per-cell repo>`, the daemon's eager `semanticstore.Open` (daemon.go:317-318) resolves the workspace-relative default `.helix/semantic.duckdb` (`internal/config/defaults.go:56`) under that private dir → per-cell store file → no shared-lock deadlock. The absolute-path route is impossible because `duckdb.go:188` rejects `filepath.IsAbs(path)` with `serr.ErrInvalidArgs`. `[VERIFIED: duckdb.go:188, daemon.go:317, defaults.go:56, sandbox.go:244]`

> **Note (stale comment in cell.go):** `cell.go:223-235` and `:441` contain doc comments claiming the cell "pins an ABSOLUTE per-cell store path." The actual `writeCellConfig` (cell.go:446-458) only writes `semantic_index:\n  enabled: false` — it does NOT pin any path. The comment is stale/aspirational and contradicts the absolute-path rejection. The planner should treat `writeCellConfig`'s real behavior (enabled:false) as ground truth and have the executor reconcile/correct these comments during D-09's doc-comment cutover.

### Pattern 2: `LanguageRunner` interface (minimal, additive-conformance) — recommendation
**What:** A 4-method interface in `bench/languages/runner.go` plus a registry lookup.
```go
// Source: recommended new file bench/languages/runner.go (greenfield — dir is .gitkeep today)
type Capability string   // closed enum: the 10 classes (semantic_view, lsp_diagnostics, …)

type TestOutcome struct {
    Passed   bool
    Tests    []TestResult   // per-test pass/fail parsed from `go test -json`
    Raw      []byte         // the test2json stream (for Phase 79 evaluators)
    ExitCode int
}
type TestResult struct{ Name, Package string; Passed bool; Elapsed float64 }

type LanguageRunner interface {
    Detect(repoDir string) bool                                  // e.g. go.mod present
    Setup(ctx context.Context, repoDir string) error             // no-op for Go (hermetic go.mod)
    RunTests(ctx context.Context, repoDir string) (TestOutcome, error)  // wraps `go test ./... -json`
    Capabilities() []Capability                                  // STATIC declared set (Go = all 10)
}
```
**Conformance test (criterion #3):** `var _ LanguageRunner = (*golang.GoRunner)(nil)` (compile-time) + a behavioral test that runs `GoRunner.RunTests` on the migrated seed and asserts `Passed == true` after the scripted edit. `go vet ./bench/...` must be clean.
**Registry + fallback (D-10):** `bench/languages.RunnerFor(benchmark, lang) LanguageRunner` returns the Go runner for `internal-toolbench`/`go` and `nil` otherwise; `cell.go` uses `RunTests` when non-nil, else falls back to `runVerify(verify.sh)`.

### Pattern 3: Language axis on the matrix (D-07)
**What:** Add `Language` to `Cell` (matrix.go:29-33), validate it with `validateMatrixID`, extend `ExpandMatrix` to a 4-way product, and join `<DatasetsRoot>/<benchmark>/<lang>/<task>` in `runOneCell` (matrix.go:213).
**Verified current state:** `Cell{Benchmark,Mode,Task}` (matrix.go:29-33); `validateMatrixID` rejects separators/parent-refs/leading-dots (matrix.go:83-91); seed join is `filepath.Join(cfg.DatasetsRoot, c.Benchmark, c.Task)` (matrix.go:213); CLI passes `[]string{o.Benchmarks}` to `ExpandMatrix` (main.go:201). Adding a language list is a strictly additive 4th product dimension.
**Constraint:** `go` (and future lang ids) are slash-free → pass the same V5 validator. `RunCell` re-validates defensively (cell.go:151-159) so add a `validateCellKey(cfg.Language, "language")` there too.

### Pattern 4: Uniform solve-then-`go test` fixture (D-04/D-05)
**What:** Copy the verified seed template. `scripted_agent.yaml` is a list of `{tool, args, expect_error}` steps (strict `KnownFields` per `runner.LoadScript`); the driver replays them over the per-cell socket after an implicit `activate_project` (drive.go:111-128). The `go test` assertion is the grade.
**D-05 mechanism (the read-capability trick):** design the test so it fails unless the agent acted on the tool's FULL output. Example call-graph fixture: three callers of `Compute` each need a guard inserted; the scripted steps are `get_call_hierarchy` (direction `incoming`) → three `replace_in_file` edits; `<module>_test.go` exercises all three call sites with inputs that panic unless guarded. Miss one → a test case fails → `RunTests.Passed == false`.

### Anti-Patterns to Avoid
- **Forking eval `StartDaemon`** to add `cmd.Dir` — violates P77 D-07. Extend with an option (Pattern 1).
- **Absolute store paths** to isolate cells — rejected at `duckdb.go:188`. Use the CWD route (D-03).
- **Deriving coverage from the `IT-go-*` ID string** — D-06 says the `task.json` `capability` field is the source of truth; the ID only mirrors it.
- **Golden-output diffing for read tools** — D-04 explicitly chose ONE shape (solve-then-`go test`). Force tool use via test design, not output comparison.
- **Enabling the store for all 10 fixtures** — D-01/D-02: store-on ONLY for incremental update; the per-cell DuckDB/indexing latency threatens the ≤90s `bench-quick` budget.
- **Leaving `--tasks=sum-doubler` hardcoded in `bench-quick`** after `git mv` — it will reference a non-existent path and break the gate (Makefile:131).

## Capability → Tool → Fixture Map (TOOLBENCH-01 / D-05)

The 10 classes are fixed by criterion #1. Tool names verified against the live registry (`internal/kernel/*`, `internal/skill/semantic`). **Note the CONTEXT's `get_callers`/`get_callees` are the single tool `get_call_hierarchy` with `direction: incoming|outgoing|both`.**

| # | Capability (`task.json` enum) | Helix MCP tool(s) | Arg struct (verified) | Deterministic "solve-then-`go test`" sketch | Store? |
|---|-------------------------------|-------------------|------------------------|-----------------------------------------------|--------|
| 1 | semantic view | `get_symbol_overview` | `SymbolOverviewArgs{Path}` (symbols/tools.go:37) | "Implement every stubbed method the overview lists in `iface.go`" — overview enumerates N stubs; `go test` checks all N | off |
| 2 | LSP diagnostics | `get_diagnostics` | `GetDiagnosticsArgs{Path}` (diag) | "Fix every error `get_diagnostics` reports in `broken.go`"; test compiles+runs only if all fixed | off |
| 3 | rename safety | `find_references` → `rename_symbol` | `FindReferencesArgs{Path,Line,Col,IncludeDecl}` (symbols/tools.go); `RenameSymbolArgs{Path,Line,Col,NewName,Receipts}` (edit/tools.go) | "Rename `Foo`→`Bar` everywhere"; test imports the new name across files | off |
| 4 | fuzzy search | `replace_in_file` (fuzzy fallback) / `fuzzy_edit` | `ReplaceInFileArgs{Path,Pattern,Replacement,IsRegex,Receipts}` (fileops/write.go) | edit a body whose pattern needs whitespace-tolerant match; test asserts new behavior | off |
| 5 | call graph | `get_call_hierarchy` (`direction:incoming`) | `CallHierarchyArgs{Path,Line,Col,Direction}` (symbols/tools.go:61) | "guard every caller of `Compute`"; test panics unless ALL callers guarded (the D-05 exemplar) | off |
| 6 | dependency graph | `get_repo_map` / `get_context` | (skill/repomap) | "wire the helper into every package the map shows depends on `core`"; test builds only if all wired | off (flag if not det. without store) |
| 7 | patch apply | `replace_symbol_body` / `replace_in_file` | `ReplaceSymbolBodyArgs` (edit); `ReplaceInFileArgs` (fileops) | the seed's edit→`go test` shape (D-08 candidate) | off |
| 8 | context minimization | `get_context` | (skill/repomap) | task solvable ONLY with the symbols `get_context` surfaces (large repo, one relevant fn); test checks the targeted fn | off |
| 9 | incremental update | `refresh_semantic_graph` + an edit | `RefreshSemanticGraphArgs{Paths,WaitForLSP,MaxWaitMs}` (skill/semantic/skill.go:340) | edit a file → `refresh_semantic_graph{wait_for_lsp:true}` drives the Phase 70 overlay-drain; then a semantic query must reflect the edit; test asserts post-refresh state | **ON** |
| 10 | failure handling | any tool with `expect_error: true` | `ScriptedStep.ExpectError` (scripted_agent.go:30) | call a tool against a bad arg / missing symbol; fixture recovers (e.g. corrects path) and `go test` passes | off |

**Determinism flags (D-02 escape hatch):**
- **Dependency graph (#6) / call graph (#5):** With the store OFF, these are served by live LSP + tree-sitter + RepoMap (the Phase 77 path). For Go (first-class LSP tier per CLAUDE.md), `get_call_hierarchy` and `get_repo_map` resolve cross-file edges without the DuckDB store. **Recommendation: keep both store-off.** Only escalate to store-on if a planning spike shows the live path is non-deterministic under `--parallel`. Flag: `[ASSUMED]` that live-path call/dependency graph is deterministic for a small hermetic Go module — verify with a spike during planning (A2 below).
- **Incremental update (#9):** structurally needs the store (overlay-drain refresh). This is the ONE store-on fixture. `refresh_semantic_graph` (verified at skill.go:340) is the driver; `RefreshSemanticGraphArgs.WaitForLSP=true` blocks on LSP enrichment draining (capped by `MaxWaitMs`). End-to-end requires `semantic_index.enabled=true` in the cell config AND the per-cell CWD (D-03).

**Guardrail receipts — NOT a blocker this phase (VERIFIED):** `rename_symbol`/`replace_in_file`/`replace_symbol_body` carry an optional `Receipts []guardrails.ReceiptID` field. The GuardrailMiddleware IS installed (`daemon.go:845`), but (a) the default enforcement level is `LevelWarn` — it forwards the call and only attaches a warning, never blocking (`enforcement.go:68`, `guardrail_middleware.go` Warn branch); and (b) the G-001 public-API/referenced-symbol rule uses a `noopOutlineProvider{}` in production and is logged as "structurally Allow" (`daemon.go:840,859`). So store-off scripted edits succeed without receipts. Fixtures MAY still issue read-side calls first (it aligns with D-05 and future-proofs against an `enforce` profile), but they are not required to pass. `[VERIFIED: daemon.go:845/840/859, enforcement.go:68, guardrail_middleware.go]`

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Spawn per-cell daemon with isolated HOME/socket/env | A new exec wrapper | eval `Sandbox.StartDaemon` + a `WithWorkingDir` option | Symlink-reject, 0700, Setpgid, log-redirect, socket-wait all already hardened (sandbox.go) |
| Replay scripted tool calls over the socket | New JSON-RPC client | `bench/runtime.driveScript` (drive.go) | id-keyed dispatcher, stdin-close race fix, per-call deadline already solved |
| Parse `scripted_agent.yaml` | New YAML schema | `runner.LoadScript` + `runner.ScriptedStep` | Strict `KnownFields`; fixtures must conform to `{tool,args,expect_error}` |
| Parse `go test` output | Regex on stdout | `encoding/json` over `go test -json` test2json events | Structured, stable, feeds Phase 79 evaluators (D-10) |
| Matrix expansion + parallel dispatch | New runner loop | `ExpandMatrix` + `RunMatrix`/`dispatch` (matrix.go) | Sem-bounded, V5-validated, deterministic ordering |
| Path-segment safety | Custom sanitizer | `validateMatrixID` / `validateCellKey` | V5/T-77 path-traversal contract; reuse verbatim for the `<lang>` segment |

**Key insight:** Phase 78 is ~90% reuse. The only genuinely new code is (a) the `LanguageRunner` interface + Go runner, (b) the `Cell.Language` axis plumbing, (c) the `cmd.Dir` option, (d) the 10 fixture data dirs, and (e) the cutover string flips. Everything else is the Phase 77 spine.

## Cutover Mechanics (D-08 / D-09)

**`make bench-quick` actually runs (VERIFIED Makefile:126-134):**
```
go build -o $(BINARY) ./cmd/helix
go run ./cmd/helix-bench run --benchmarks=$(SUITE) --modes=your_agent_full \
    --tasks=sum-doubler --agent=scripted --helix-bin=$(CURDIR)/$(BINARY) --out bench/reports
```
`SUITE ?= toolbench-go` (Makefile:115). To keep it green through the cutover, ALL of these must flip together in one commit:

| File:line | Current | After cutover |
|-----------|---------|---------------|
| `Makefile:115` | `SUITE ?= toolbench-go` | `SUITE ?= internal-toolbench` |
| `Makefile:131` | `--tasks=sum-doubler` | `--tasks=<seed's new IT-go-* dir name>` (+ add `--languages=go` once the flag exists) |
| `Makefile:114,117` (comments) | `default toolbench-go` | `default internal-toolbench` |
| `cmd/helix-bench/main.go:142` | `StringVar(&benchmarks, "benchmarks", "toolbench-go", …)` | `… "internal-toolbench", …` + new `--languages` (default `go`) flag |
| `cmd/helix-bench/main.go:194,201` | `discoverTasks` / `ExpandMatrix` use `<benchmark>/<task>` | thread `<benchmark>/<lang>/<task>` |
| `bench/BENCH.md:77-78` | `default toolbench-go` | `default internal-toolbench` |
| `bench/LICENSES.md:14` | row key `internal-toolbench-go`, path `bench/datasets/internal-toolbench/` | reconcile row key to `internal-toolbench` (path already correct) |
| `cell.go:53,68` + `matrix.go` doc comments | `e.g. "toolbench-go"`, `bench/datasets/toolbench-go/sum-doubler` | `internal-toolbench` examples; reconcile the stale absolute-store-path comments (cell.go:223-235,441) |
| `cmd/helix-bench/run_cmd_test.go:21,25,36,38,58` | synthetic `toolbench-go`/`sum-doubler` | update to `internal-toolbench`/`go`/new task id |
| `bench/runtime/cell_test.go`, `cctap_test.go`, `result_test.go`, `cross_cell_test.go` | `toolbench-go`/`sum-doubler` literals | update test literals (these are in-test strings, not disk-coupled except cross_cell/run_cmd which create synthetic dirs) |

`matrix_test.go` uses `toolbench-go`/`toolbench-rust` as **synthetic validation literals** (line 14, 16) — they are not disk-coupled; update only if the planner wants naming consistency, but they don't break the cutover.

**`git mv` seed relocation (D-08, Phase 64 microbench pattern VERIFIED as the precedent):**
```
git mv bench/datasets/toolbench-go/sum-doubler bench/datasets/internal-toolbench/go/IT-go-patch-apply-1
# then rmdir the now-empty bench/datasets/toolbench-go/
```
Preserves `git log --follow`. After the move, edit the relocated `task.json`: change `"benchmark":"toolbench-go"` → `"internal-toolbench"`, add `"capability":"patch apply"` (or `"fuzzy search"` — D-08 discretion), set `"id":"IT-go-patch-apply-1"`. `go.mod` module path (`toolbenchseed/sumdoubler`) is internal to the fixture and need not change, but renaming it for consistency is harmless.

## Phase 67 Crosswalk (criterion #4)

**Finding:** There is NO `internal/eval/datasets/` or task-dir tree in the current repo — `internal/eval/` is package-structured (`agent/ budget/ judge/ pipeline.go runner/ report/ sandbox/ score/ trace/`), with no `task.json` files. The `T-67-*` identifiers are **planning task IDs** from the Phase 67 plan/history, not runtime fixtures on disk (the `67-*` phase directory has also been pruned from `.planning/phases/`).

**Implication for `PHASE67_CROSSWALK.md`:** It is an **inspiration-mapping doc only** (CONTEXT discretion + criterion #4 wording). It maps which Phase 67 *concepts/tasks* (e.g. the scripted-agent harness `T-67-06a`, the daemon-tap `T-67-04`, the sandbox-isolation `T-67-01`) inspired which `IT-go-*` capability fixtures. The `IT-go-*` namespace is deliberately disjoint from `T-67-*` (zero collision — different prefix, different domain). **No code migration; no fixture porting.** The planner should source the `T-67-*` ID list from `.planning/STATE.md` / the Phase 67 milestone history (the eval EVAL.md reciprocal pointer added under INFRA-03) rather than from any on-disk task corpus.

## Common Pitfalls

### Pitfall 1: Parallel store-lock deadlock if store-on cells share a CWD
**What goes wrong:** Two incremental-update cells running under `--parallel` both open `.helix/semantic.duckdb` relative to the same harness CWD → DuckDB file-lock deadlock.
**Why:** The store path is CWD-relative (`defaults.go:56`) and opened eagerly (`daemon.go:317`); absolute paths are rejected (`duckdb.go:188`).
**How to avoid:** D-03 `WithWorkingDir(repoDir)` per cell. The per-cell repo dirs are already isolated by the sandbox (`RepoFor` → `<tmp>/<task>/<mode>/repo`), so `.helix/semantic.duckdb` lands inside each.
**Warning signs:** a cell hanging at daemon startup; `socket did not appear within 10s` (sandbox.go:286).

### Pitfall 2: `bench-quick` breaks the instant the seed is `git mv`'d
**What goes wrong:** `Makefile:131` hardcodes `--tasks=sum-doubler`; after the move that task dir no longer exists at `<datasets>/toolbench-go/`.
**How to avoid:** Flip the Makefile, main.go default, AND the seed location in the SAME commit (D-09 single clean cutover). Run `make bench-quick` locally before/after to prove green.
**Warning signs:** `no task dirs under …` (main.go:263) or `0/N cells succeeded`.

### Pitfall 3: ≤90s budget blown by store indexing
**What goes wrong:** Enabling the semantic store per cell adds DuckDB open + initial-walk + LSP-enrichment latency. If `bench-quick` ever runs the incremental-update fixture, the budget can blow.
**How to avoid:** Keep `bench-quick` pinned to a store-OFF fixture (the migrated seed). The store-on incremental-update fixture runs in the full `make bench`, not the quick gate. D-01's targeted enablement exists precisely to protect this budget.
**Warning signs:** `bench-quick` wall-clock creeping toward 90s; CI timeout.

### Pitfall 4: `go test -json` exit code vs parse — both needed
**What goes wrong:** `go test` exits non-zero on failure AND a compile error produces a `build-fail` action with no per-test rows. Relying only on per-test `fail` rows misses compile failures.
**How to avoid:** `RunTests` returns `Passed = (exit==0)` as the authoritative gate AND parses per-test rows for the structured detail; on a compile failure (no test rows, non-zero exit) report `Passed=false` with `Tests=[]` so Phase 79's `compile_errors_*` can populate.
**Warning signs:** a fixture that doesn't compile reported as "0 failures, passed."

### Pitfall 5: Stale cell.go comments contradict reality
**What goes wrong:** `cell.go:223-235,441` describe an absolute per-cell store path that the code does not implement (`writeCellConfig` only sets `enabled:false`). An executor trusting the comment would attempt the rejected absolute-path route.
**How to avoid:** Treat `writeCellConfig` (cell.go:446-458) as ground truth; reconcile the comments during the D-09 doc-comment cutover.

## Runtime State Inventory

> This is a corpus-extension + interface phase, not a rename of live runtime state. The "rename" (D-09 `toolbench-go`→`internal-toolbench`) touches only repo-tracked files, not stored data or live services. Included for completeness per the refactor trigger.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — bench cells write ephemeral `/tmp/helix-eval-*` scratch (deleted on success) + durable `bench/reports/<run_id>/` artifacts. The renamed string `toolbench-go` is NOT a DB key/collection. The per-cell `.helix/semantic.duckdb` is created fresh per run. | None — verified: no datastore keys reference `toolbench-go` (grep across repo found only code/doc/test literals). |
| Live service config | None — no external service stores `toolbench-go`. The daemon is spawned fresh per cell. | None. |
| OS-registered state | None — no Task Scheduler / launchd / systemd / pm2 registration references the benchmark name. | None. |
| Secrets/env vars | None — no env var or secret key is named after the benchmark. The daemon env allowlist is `HOME/PATH/HELIX_LOG_LEVEL` (sandbox.go:255-261). | None. |
| Build artifacts | The migrated seed's `go.mod` module `toolbenchseed/sumdoubler` is self-contained; no installed package or compiled binary embeds `toolbench-go`. `cmd/helix-bench` reads the default at runtime (no codegen). | None beyond the source flips in the cutover table. |

## Validation Architecture

> nyquist_validation = true (`.planning/config.json`). Section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (go 1.23) + `go test -json` for the bench corpus |
| Config file | none (Go convention) |
| Quick run command | `go test ./bench/... -count=1` |
| Full suite command | `go vet ./... && go test ./...` |
| Bench gate | `make bench-quick` (build helix, scripted smoke, ≤90s, ≥1 cell succeeds) |

### Success Criteria → Test Map (the 4 criteria + 3 requirements, independently verifiable)
| Criterion / Req | Behavior | Test Type | Automated Command | Exists? |
|------|----------|-----------|-------------------|---------|
| C1 / TOOLBENCH-01 | 10 capability classes documented + ≥1 fixture each | doc + corpus check | a Go test that reads every `internal-toolbench/go/*/task.json`, asserts the 10 `capability` values are all covered; assert `CAPABILITIES.md` lists 10 classes | ❌ Wave 0 |
| C2 / TOOLBENCH-02 | Go full run passes; coverage 10/10 | integration | `go run ./cmd/helix-bench run --benchmarks=internal-toolbench --languages=go --agent=scripted` exits 0 with 10/10 cells; coverage report = `Capabilities()` ∩ `task.json` = 10/10 | ❌ Wave 0 |
| C2 | `RunTests` wraps `go test ./... -json` | unit | `GoRunner.RunTests(seedRepo)` returns `Passed=true`, parses ≥1 `TestResult` | ❌ Wave 0 |
| C3 / TOOLBENCH-10 | `LanguageRunner` defined; conformance + vet | compile-time + unit | `var _ languages.LanguageRunner = (*golang.GoRunner)(nil)`; `go vet ./bench/...` clean | ❌ Wave 0 |
| C4 | `IT-go-*` namespace, zero `T-67-*` collision | static | a test asserting every task id matches `^IT-go-` and none matches `^T-67-`; `PHASE67_CROSSWALK.md` exists | ❌ Wave 0 |
| D-03 | per-cell store isolation under `--parallel` | integration | run 2 store-on cells `--parallel=2`; assert both succeed, distinct `.helix/semantic.duckdb` files, `RejectedForeignPid==0` | ❌ Wave 0 |
| D-09 | `bench-quick` green post-cutover | gate | `make bench-quick` exits 0 ≤90s | ✅ (target exists; must stay green) |
| D-10 | verify.sh fallback when no runner | unit | a cell with `RunnerFor()==nil` falls back to `runVerify`; existing `cell_test.go` covers verify path | partial — extend |

### Sampling Rate
- **Per task commit:** `go test ./bench/... -count=1` (+ `gofmt`, `go vet ./bench/...`).
- **Per wave merge:** `go vet ./... && go test ./...` + `make bench-quick`.
- **Phase gate:** full `go run ./cmd/helix-bench run --benchmarks=internal-toolbench --languages=go` green (10/10 cells) before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `bench/languages/runner.go` — interface + registry + `TestOutcome`/`TestResult` types (none exist; dir is `.gitkeep`).
- [ ] `bench/languages/go/runner.go` + `runner_test.go` — Go runner + conformance test.
- [ ] `bench/datasets/internal-toolbench/CAPABILITIES.md` — 10-class doc.
- [ ] `bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md` — inspiration map.
- [ ] A corpus-coverage test (reads all `task.json` `capability` fields, asserts 10/10 vs declared).
- [ ] A `--parallel` store-isolation integration test for the incremental-update fixture.
- Framework install: none — Go testing is built in.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | `go test ./... -json`, build | ✓ | go 1.23 (module `go 1.23`) | — |
| gopls (Go LSP) | semantic-view / call-graph / rename / incremental fixtures (first-class tier) | assumed present (Helix manages LS install) | — | tree-sitter fallback for best-effort ops; first-class Go needs gopls for cross-file edges |
| `helix` binary | per-cell daemon + forwarder | built by `make bench-quick` first | — | `bench-quick` builds it; CI must build before run |
| DuckDB (CGO) | incremental-update fixture's store | ✓ (CGO=1 build, in-tree `internal/semantic/store`) | — | platform-stubbed on windows/arm64 only (D-14) — out of scope |
| Docker / network | — | not needed | — | hermetic by design (D-01); no network |

**Missing dependencies with no fallback:** gopls must be resolvable for the 4 LSP-backed Go fixtures (semantic view, call graph, rename, incremental). The planner should add a `Setup`/`Detect`-time gopls availability check (or rely on Helix's three-tier LS installer). `[ASSUMED]` gopls is installed in CI — confirm during planning (A1).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `verify.sh` exit code is the outcome (Phase 77) | `LanguageRunner.RunTests` parses `go test -json` for structured per-test results; verify.sh demoted to runner-less fallback | This phase (D-10) | Feeds Phase 79 `test_runner` + `compile_errors_*`; richer than exit code |
| `<benchmark>/<task>` matrix (Phase 77) | `<benchmark>/<lang>/<task>` with `Cell.Language` + `--languages` | This phase (D-07) | Phase 85 drops in 7 languages with no matrix rewrite |
| `toolbench-go` benchmark name | `internal-toolbench` (language is an axis, not part of the name) | This phase (D-09) | Single benchmark, N languages |
| semantic store globally disabled per cell (Phase 77) | targeted per-cell opt-in via per-cell CWD (D-03) | This phase | Incremental-update capability becomes testable; others stay fast |

**Deprecated/outdated:**
- The `toolbench-go` benchmark name and the `bench/datasets/toolbench-go/` dir — fully retired this phase (D-09, no stale dir left behind).
- cell.go's "absolute per-cell store path" comments (cell.go:223-235,441) — never implemented; reconcile.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | gopls is available in the CI/dev environment for the 4 LSP-backed Go fixtures | Environment Availability | Semantic-view/call-graph/rename/incremental fixtures non-deterministic or failing; mitigate with a `Setup`/`Detect` availability gate |
| A2 | Call-graph (#5) and dependency-graph (#6) are deterministically testable store-OFF (live LSP + RepoMap) for a small hermetic Go module | Capability map | If not, those 1–2 fixtures opt into the store under the same D-03 knob (D-02 escape hatch) — adds latency, threatens budget if in `bench-quick` |
| A3 | The `IT-go-*` ↔ `T-67-*` crosswalk can be authored from planning history (STATE.md / EVAL.md), since no on-disk `T-67-*` corpus exists | Phase 67 Crosswalk | If a richer T-67 task list is needed, source it from the Phase 67 milestone doc; crosswalk stays inspiration-only regardless |
| A4 | Default guardrail enforcement stays `LevelWarn` and G-001 stays a no-op stub for `bench-full` cells, so store-off edits need no receipts | Capability map (receipts) | If a future profile sets `enforce`, fixtures editing referenced symbols must carry receipts from prior `find_references`/`analyze_blast_radius` calls — which D-05 read fixtures already do |
| A5 | `WithWorkingDir` functional-option is the team-preferred additive seam over a sandbox wrapper | Pattern 1 | If the team prefers an explicit 6th positional param, the change is equally additive but breaks the 5-arg call sites; the option avoids that |

## Open Questions (RESOLVED)

> All three resolved at plan time and adopted verbatim by the Phase 78 plans (Plan 02 for Q1/Q2, Phase 79 handoff for Q3).

1. **Store opt-in knob location (CONTEXT discretion D-06/D-11 note).**
   - What we know: it can live in `task.json` (explicit `"semantic_index": true`) or be derived from `capability == "incremental update"`.
   - What's unclear: which is least-surprising for Phase 85 authors.
   - RESOLVED: **derive from `capability`** (single source of truth, no second flag to forget), with `task.json` able to OVERRIDE for the D-02 escape-hatch case (a call-graph fixture that turns out to need the store). Keeps the common case zero-config. (Adopted in Plan 02: `StoreOptIn` derived from `capability=="incremental_update"`, `task.json semantic_index:true` override.)

2. **`bench-quick` task selection after cutover.**
   - What we know: it hardcodes `--tasks=sum-doubler`.
   - What's unclear: whether to pin it to the migrated seed task id or let it discover-all (which would pull in the store-on incremental fixture and risk the 90s budget).
   - RESOLVED: **pin `--tasks` to the migrated store-OFF seed** (`IT-go-patch-apply-1`); full `make bench` runs all 10. (Adopted in Plan 02: bench-quick pinned to `IT-go-patch-apply-1`.)

3. **Does `RunTests` need a separate "before" run for `compile_errors_before`?**
   - What we know: Phase 79 wants `compile_errors_before/after`. Phase 78 only produces `after` (post-edit `go test`).
   - RESOLVED: out of scope — Phase 79 owns the before/after orchestration; `RunTests` just needs to be re-runnable. Noted for the Phase 79 handoff. (Adopted: Plan 01 `RunTests` is stateless/re-runnable; before/after deferred to Phase 79.)

## Sources

### Primary (HIGH confidence — read in this session)
- `bench/runtime/cell.go` (RunCell spine, writeCellConfig:446-458, runVerify:394-413, stale store comments:223-235/441)
- `bench/runtime/matrix.go` (Cell:29-33, ExpandMatrix:100, validateMatrixID:83, seed join:213)
- `bench/runtime/subprocess/daemon.go` (StartDaemon delegation:37-46)
- `bench/runtime/drive.go` (driveScript, activate_project:111-128, id-keyed dispatcher)
- `bench/runtime/sandbox/sandbox.go` (embeds eval sandbox; durable paths only)
- `internal/eval/sandbox/sandbox.go` (StartDaemon:231, cmd build:244 — no cmd.Dir; env allowlist:255-261; Setpgid:252)
- `internal/eval/runner/scripted_agent.go` (ScriptedStep:23-31, LoadScript strict KnownFields:38-53)
- `internal/semantic/store/duckdb.go` (Open:169, absolute-path reject:188)
- `internal/config/defaults.go:56` (`.helix/semantic.duckdb` default), `internal/semantic/config.go:103-106` (workspace-relative)
- `internal/daemon/daemon.go` (store open:304-331, guardrail install:834-859, middleware sites:795-898)
- `internal/kernel/symbols/tools.go` (SymbolOverviewArgs:37, FindReferencesArgs, CallHierarchyArgs:61, BlastRadiusArgs:77, GoToDefinitionArgs)
- `internal/kernel/fileops/{tools.go,write.go}` (ReplaceInFileArgs, SearchInFilesArgs, Receipts field)
- `internal/kernel/edit/tools.go` (RenameSymbolArgs, ReplaceSymbolBodyArgs, Receipts)
- `internal/skill/semantic/skill.go:340` (refresh_semantic_graph), `RefreshSemanticGraphArgs`
- `internal/mcp/guardrail_middleware.go` (Warn vs Block; isDestructiveTool:172), `internal/guardrails/enforcement.go:57-69` (default LevelWarn)
- `Makefile:106-134` (bench/bench-quick targets, SUITE), `cmd/helix-bench/main.go:101-267` (flags, runBench, discoverTasks)
- `bench/datasets/toolbench-go/sum-doubler/*` (full seed file set), `bench/BENCH.md`, `bench/LICENSES.md:14`
- `.planning/milestones/v1.12-ROADMAP.md` §Phase 78/79/85, `.planning/REQUIREMENTS.md` (TOOLBENCH-01/02/10)
- **Live verification:** `go build ./cmd/helix-bench` (clean), `go vet ./bench/...` (clean), `go test ./... -json` on the seed (test2json event stream confirmed)

### Tertiary (LOW confidence)
- gopls CI availability (A1) — assumed, not probed in this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all in-tree/stdlib, no external packages, no legitimacy audit needed (no `npm install` / `pip install` this phase).
- Architecture (matrix axis, runner interface, cmd.Dir seam): HIGH — every seam read at the line level; the `cmd.Dir` gap and absolute-path rejection verified directly.
- Capability→tool mapping: HIGH for tool names/args (verified registry); MEDIUM for the store-off determinism of call/dependency graph (A2 — recommend a planning spike).
- Cutover surface: HIGH — exhaustive grep of `toolbench-go`/`sum-doubler` across active code/tests/Makefile.
- Guardrail non-blocking finding: HIGH — install site + default level + no-op stub all read directly.

**Research date:** 2026-06-17
**Valid until:** 2026-07-17 (stable in-repo target; re-verify cell.go/matrix.go/daemon.go line numbers if Phase 77 follow-ups land first).

## Package Legitimacy Audit

Not applicable — Phase 78 installs **zero** external packages. All dependencies are Go stdlib or already in `go.mod` (`gopkg.in/yaml.v3`, in-tree `bench/*`, `internal/eval/*`, `internal/semantic/*`). No npm/PyPI/crates legitimacy gate required.
