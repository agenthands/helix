# Phase 78: Internal ToolBench — Go First + LanguageRunner Interface - Pattern Map

**Mapped:** 2026-06-17
**Files analyzed:** 14 (4 new code, 1 new test, 2 new docs, 5 modified code, 10 fixture dirs, 4 cutover/doc targets)
**Analogs found:** 13 / 14 (one genuinely greenfield idiom — see "No Analog Found")

> RESEARCH.md already carries file:line seams for nearly every target. This map confirms
> each analog against the live tree, extracts the load-bearing excerpts, and flags the ONE
> pattern (functional options) that has **no existing precedent in the repo** — so the
> planner does not send an executor hunting for an analog that does not exist.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/languages/runner.go` (interface + registry + result types) | interface/registry | transform | `internal/skill/registry.go` (Register/Get/All map+mutex) | role-match (registry); interface greenfield |
| `bench/languages/go/runner.go` (`GoRunner`, wraps `go test ./... -json`) | service | request-response (subprocess) | `bench/runtime/cell.go::runVerify` (394-413) | role-match (subprocess+exit-code) |
| `bench/languages/go/runner_test.go` (conformance) | test | — | `internal/skill/registry_test.go` | role-match |
| `bench/runtime/matrix.go` (`Cell.Language`, ExpandMatrix, validateMatrixID, `<lang>` join) | config/orchestrator | transform | itself (29-33, 83-91, 100, 213) | exact (extend in place) |
| `bench/runtime/cell.go` (runner dispatch + store opt-in + verify fallback) | orchestrator | request-response | itself (`runVerify` 394-413, `writeCellConfig` 446-458) | exact (extend in place) |
| `internal/eval/sandbox/sandbox.go` (`WithWorkingDir` option, `cmd.Dir`) | service | process-spawn | itself (`StartDaemon` 231, cmd build 244) | exact (extend in place); **option idiom has NO repo precedent** |
| `bench/runtime/subprocess/daemon.go` (thread the option through) | service | process-spawn | itself (delegation 37-46) | exact |
| `bench/datasets/internal-toolbench/go/<task>/` x10 | fixture (data) | — | `bench/datasets/toolbench-go/sum-doubler/` (full file set) | exact |
| `bench/datasets/internal-toolbench/CAPABILITIES.md` | doc | — | `bench/BENCH.md` | role-match |
| `bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md` | doc | — | `bench/BENCH.md` | role-match |
| `cmd/helix-bench/main.go` (`--languages` flag, `--benchmarks` default) | CLI | — | itself (flags 142-150, opts 158-205) | exact |
| `Makefile`, `bench/BENCH.md`, `bench/LICENSES.md` | config/doc | — | RESEARCH cutover table (Makefile:115/131, BENCH:77-78, LICENSES:14) | exact |

## Pattern Assignments

### `bench/languages/runner.go` (interface + registry, NEW)

**Analog (registry idiom):** `internal/skill/registry.go` — this is the repo's canonical
"package-level map + RWMutex + `Register`/`Get`/`All`" pattern. Copy this shape for
`RunnerFor(benchmark, lang)`.

**Registry pattern to copy** (`internal/skill/registry.go:9-29`):
```go
var (
	globalMu sync.RWMutex
	skills   = make(map[string]Skill)
)

// Register adds a skill to the global registry.
// Intended to be called from init() in skill packages (Caddy-style registration).
func Register(s Skill) {
	globalMu.Lock()
	defer globalMu.Unlock()
	skills[s.Name()] = s
}

// Get returns a skill by name and whether it was found.
func Get(name string) (Skill, bool) {
	globalMu.RLock()
	defer globalMu.RUnlock()
	s, ok := skills[name]
	return s, ok
}
```
**Apply:** model `RunnerFor(benchmark, lang string) LanguageRunner` on `Get` — return `nil`
when no runner registered (the D-10 verify.sh-fallback signal). The Go runner registers via
an `init()` blank-import (Caddy-style), exactly as skills do. Keyed by `<benchmark>/<lang>`
or just `<lang>` — planner's discretion; D-10 only requires "registered by benchmark/language".

**Interface + result types** — use the RESEARCH.md Pattern 2 recommendation verbatim
(`LanguageRunner{Detect,Setup,RunTests,Capabilities}`, `TestOutcome`, `TestResult`,
`Capability string`). This is greenfield; there is no prior runner interface to copy.

---

### `bench/languages/go/runner.go` (`GoRunner`, NEW)

**Analog (subprocess + exit-code outcome):** `bench/runtime/cell.go::runVerify` (394-413).
`RunTests` is structurally `runVerify` + a `go test -json` line-decoder. Copy the
exec/exit-code/context-cancel handling; ADD the json decode.

**Subprocess + exit-code pattern to copy** (`bench/runtime/cell.go:394-413`):
```go
func runVerify(ctx context.Context, scriptPath, repoDir string) (int, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", scriptPath)
	cmd.Dir = repoDir
	err := cmd.Run()
	if ctx.Err() != nil {
		return 0, fmt.Errorf("verify cancelled/timed out: %w", ctx.Err())
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil  // task FAIL, not infra error
		}
		return 1, nil
	}
	return 0, nil
}
```
**Apply to `RunTests`:**
- Build `exec.CommandContext(ctx, "go", "test", "./...", "-json")`, set `cmd.Dir = repoDir`.
- `cmd.Output()` (capture stdout = the test2json stream into `TestOutcome.Raw`).
- Reuse the **exact same** `ctx.Err()` → infra-error vs `*exec.ExitError` → task-fail split.
- `Passed = (exit == 0)` is the authoritative gate (Pitfall 4: compile-fail yields no test
  rows but a non-zero exit — do NOT infer pass from "0 failing rows").

**JSON line-decode pattern** — there is no `go test -json` parser in the repo today. Use the
stdlib streaming decoder (the repo's established `encoding/json` line idiom — see
`runner.LoadScript` which uses `yaml.NewDecoder` over a `bytes.Reader` the same way):
```go
dec := json.NewDecoder(bytes.NewReader(raw))
for {
	var ev struct{ Action, Package, Test string; Elapsed float64 }
	if err := dec.Decode(&ev); err == io.EOF { break } else if err != nil { ... }
	// collect ev.Action ∈ {pass,fail,skip} with ev.Test != "" into []TestResult
}
```
(`Action` set verified live in RESEARCH §"go test -json format", line 102.)

**Conformance test** (`runner_test.go`) — analog `internal/skill/registry_test.go`. Add the
compile-time assertion `var _ languages.LanguageRunner = (*golang.GoRunner)(nil)` plus a
behavioral test that runs `RunTests` on the migrated seed repo and asserts `Passed == true`.

---

### `bench/runtime/matrix.go` (MODIFY — add language axis, D-07)

**Analog: itself.** Extend in place; do not rewrite.

**`Cell` struct** (matrix.go:29-33) — add the field:
```go
type Cell struct {
	Benchmark string
	Mode      string
	Task      string
	// Language string   // ← ADD (D-07); 4th path segment
}
```

**Validator to reuse VERBATIM for the new segment** (matrix.go:83-91):
```go
func validateMatrixID(id, kind string) error {
	if id == "" { return fmt.Errorf("%s is empty", kind) }
	if id != filepath.Clean(id) || strings.ContainsAny(id, `/\`) || strings.HasPrefix(id, ".") {
		return fmt.Errorf("%s %q contains path separators, parent refs, or a leading dot", kind, id)
	}
	return nil
}
```
**Apply:** `ExpandMatrix` becomes a 4-way product `benchmarks × languages × modes × tasks`;
call `validateMatrixID(lang, "language")` in the same validation loop (matrix.go:111-120 block)
BEFORE it becomes a path segment. The seed-dir join at matrix.go:213
(`filepath.Join(cfg.DatasetsRoot, c.Benchmark, c.Task)`) becomes
`filepath.Join(cfg.DatasetsRoot, c.Benchmark, c.Language, c.Task)`.
`RunCell` re-validates defensively (cell.go:151-159) — add `validateCellKey(cfg.Language, "language")` there too (RESEARCH Pattern 3, line 215).

---

### `bench/runtime/cell.go` (MODIFY — runner dispatch + store opt-in + verify fallback, D-03/D-10)

**Analog: itself.**

**Verify outcome path to REPLACE/wrap** (`runVerify` 394-413, shown above) — D-10: dispatch to
`languages.RunnerFor(benchmark, lang)`; when non-nil call `RunTests`, else fall back to
`runVerify(verify.sh)`.

**Store opt-in knob** — current `writeCellConfig` (446-458) hardcodes `enabled: false`:
```go
cfg := fmt.Sprintf("profile: %s\nsemantic_index:\n  enabled: false\n", profileName)
```
**Apply:** parameterize the `enabled` value from a per-cell `StoreOptIn bool` (derived from
`task.json.capability == "incremental update"`, RESEARCH Open Q1 recommendation; `task.json`
may OVERRIDE for the D-02 escape hatch). When `StoreOptIn`, the cell ALSO passes
`WithWorkingDir(repoDir)` into `StartDaemon` (below).

**Stale-comment reconciliation (Pitfall 5):** cell.go:223-235 and :441 describe an
"absolute per-cell store path" that `writeCellConfig` never implements. Treat the code
(`enabled: false`) as ground truth; correct these comments during the D-09 doc-comment cutover.

---

### `internal/eval/sandbox/sandbox.go` + `bench/runtime/subprocess/daemon.go` (MODIFY — working-dir seam, D-03)

**Analog: itself (the spawn body).** The single new behavior line is `cmd.Dir = o.workDir`.

**Current spawn (sandbox.go:231-244)** — `cmd.Dir` is never set:
```go
func (s *Sandbox) StartDaemon(ctx context.Context, taskID, mode, profileName, cfgPath string) (*DaemonHandle, error) {
	...
	cmd := exec.CommandContext(ctx, s.helixBin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}  // ← cmd.Dir absent
```
**Current delegation (subprocess/daemon.go:37-46):**
```go
func StartDaemon(ctx context.Context, sb *benchsandbox.Sandbox, taskID, mode, profileName, cfgPath string) (*evalsandbox.DaemonHandle, error) {
	h, err := sb.StartDaemon(ctx, taskID, mode, profileName, cfgPath)
	...
}
```
**Apply (RESEARCH Pattern 1, lines 166-182):** add `opts ...DaemonOption` to BOTH signatures,
a `daemonOpts{workDir string}` struct, a `WithWorkingDir(dir)` constructor, and one line
`if o.workDir != "" { cmd.Dir = o.workDir }` after the `exec.CommandContext`. Existing 5-arg
callers compile unchanged (additive, non-forking — satisfies P77 D-07).

> **CRITICAL for planner:** functional-options is **NOT an existing idiom in this repo**
> (see "No Analog Found"). The executor must write it fresh from the RESEARCH Pattern 1
> excerpt, not copy a precedent. Keep it minimal and local to `sandbox.go`.

---

### `bench/datasets/internal-toolbench/go/<task>/` x10 (NEW fixtures, D-04/D-05)

**Analog: the full Phase 77 seed file set** `bench/datasets/toolbench-go/sum-doubler/`
(`task.json`, `scripted_agent.yaml`, `verify.sh`, `<module>.go`, `<module>_test.go`, `go.mod`).
Every fixture mirrors this exact 6-file shape.

**`task.json` template to copy** (seed, verified):
```json
{
  "id": "sum-doubler",
  "benchmark": "toolbench-go",
  "language": "go",
  "prompt": "...",
  "verify": "verify.sh"
}
```
**Apply:** change `id` → `IT-go-<capability>-<n>`, `benchmark` → `internal-toolbench`, ADD
`"capability": "<one of the 10 enum values>"` (D-06 — the coverage source of truth, NOT the id).

**`scripted_agent.yaml` template to copy** (seed, verified — strict `runner.ScriptedStep`
keys `tool`/`args`/`expect_error` only, per scripted_agent.go:23-31):
```yaml
steps:
  - tool: replace_in_file
    args:
      path: sum.go
      pattern: "\treturn x\n"
      replacement: "\treturn x * 2\n"
```
**Apply (D-05 read-capability trick):** each fixture's steps must EXPLICITLY call the
capability's tool (e.g. `get_call_hierarchy{direction: incoming}`, `get_symbol_overview`,
`find_references`, `get_diagnostics`, `get_context`, `refresh_semantic_graph`) — see
RESEARCH Capability→Tool map (lines 233-244) for the verified tool name + arg struct per
capability — and the `<module>_test.go` must fail unless the agent acted on the tool's FULL
output (e.g. all N callers guarded).

**`verify.sh` template** (seed, verified) — retained as the D-10 runner-less fallback:
```sh
#!/bin/sh
set -e
cd "$(dirname "$0")"
exec go test ./...
```

**`<module>.go` / `<module>_test.go` / `go.mod`** — copy the seed's "deliberately-wrong body +
table-driven test + dependency-free go.mod (`module <name>` / `go 1.23`)" shape. Hermetic: no
external deps so `go test` runs offline.

**Seed migration (D-08):** `git mv bench/datasets/toolbench-go/sum-doubler
bench/datasets/internal-toolbench/go/IT-go-patch-apply-1` (Phase 64 microbench precedent),
then rmdir the empty `toolbench-go/`, then edit the relocated `task.json`
(benchmark/id/capability). `go.mod` module path is fixture-internal — rename optional.

---

### `cmd/helix-bench/main.go` (MODIFY — `--languages` flag + flipped `--benchmarks` default, D-07/D-09)

**Analog: itself** (flag block 142-150). Copy the existing `StringArrayVar`/`StringVar` idiom:
```go
cmd.Flags().StringVar(&benchmarks, "benchmarks", "toolbench-go", "benchmark suite")
cmd.Flags().StringArrayVar(&modes, "modes", []string{"your_agent_full"}, "mode(s); repeatable")
```
**Apply:** flip default `"toolbench-go"` → `"internal-toolbench"`; add
`StringArrayVar(&languages, "languages", []string{"go"}, "language(s); repeatable")`; add a
`Languages []string` field to `runBenchOpts` (158-205); thread it into the now-4-way
`runtime.ExpandMatrix(...)` call (currently `ExpandMatrix([]string{o.Benchmarks}, o.Modes, tasks)`
at ~line 201) and into `discoverTasks` (the `<benchmark>/<lang>/<task>` join).

---

## Shared Patterns

### Path-traversal validation (V5 / T-77-10)
**Source:** `bench/runtime/matrix.go:83-91` (`validateMatrixID`) + `cell.go:151-159` (`validateCellKey`).
**Apply to:** the new `Language` segment in `matrix.go` AND the defensive re-check in `cell.go`.
Every new path segment passes the identical slash-free / no-parent-ref / no-leading-dot check.

### Subprocess outcome (exit-code + context-cancel split)
**Source:** `bench/runtime/cell.go:394-413` (`runVerify`).
**Apply to:** `GoRunner.RunTests`. `ctx.Err()` → infra error; `*exec.ExitError` → task fail
(record exit code); non-ExitError → fail. Never let a harness cancellation be recorded as a
task FAILURE.

### Strict-decode parsing idiom
**Source:** `internal/eval/runner/scripted_agent.go:38-53` (`yaml.NewDecoder(bytes.NewReader(...))`
+ `KnownFields(true)`).
**Apply to:** the `go test -json` decoder — same "decoder over an in-memory byte reader" shape
with `encoding/json` instead of yaml. Fixtures' `scripted_agent.yaml` must conform to the strict
`ScriptedStep` key set (`tool`/`args`/`expect_error`).

### Caddy-style registry
**Source:** `internal/skill/registry.go:9-29` (map + RWMutex + `Register`/`Get`).
**Apply to:** `bench/languages.RunnerFor`. `nil` return is the D-10 verify.sh-fallback signal.

## No Analog Found

| File / Pattern | Role | Reason |
|------|------|--------|
| `WithWorkingDir` functional-option in `internal/eval/sandbox/sandbox.go` | service (process-spawn) | **No functional-options idiom exists anywhere in the repo** (verified: zero `Option func(*...)` declarations, zero `opts ...` variadic-option signatures under `internal/` or `bench/`). RESEARCH Pattern 1 (lines 166-182) is the authoritative template — the executor writes it fresh, minimal, local to sandbox.go. The registry idiom (skill) is the closest *structural* cousin but is not a functional-option. |
| `LanguageRunner` interface body | interface | Greenfield — `bench/languages/` is a bare `.gitkeep`; no prior test-runner interface. Use RESEARCH Pattern 2 (lines 188-208) verbatim. |
| `go test -json` test2json decoder | service | No existing parser in-tree (RESEARCH "Don't Hand-Roll" line 259 confirms: build it on `encoding/json`). Borrow only the *decoder shape* from `LoadScript`. |

## Metadata

**Analog search scope:** `bench/runtime/`, `bench/languages/` (empty), `bench/datasets/`,
`internal/eval/sandbox/`, `internal/eval/runner/`, `internal/skill/`, `internal/langregistry/`,
`cmd/helix-bench/`.
**Files scanned (read at line level):** matrix.go, cell.go, sandbox.go, subprocess/daemon.go,
scripted_agent.go, skill/registry.go, main.go, and the full seed file set.
**Repo-wide negative searches:** `Option func(*`, `opts ...` (no functional-options precedent).
**Pattern extraction date:** 2026-06-17
