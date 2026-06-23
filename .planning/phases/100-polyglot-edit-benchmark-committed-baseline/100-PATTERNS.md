# Phase 100: Polyglot Edit Benchmark + Committed Baseline - Pattern Map

**Mapped:** 2026-06-23
**Files analyzed:** 7 (2 new Go, 2 modified Go, 1 new data, 1 new test cluster, 1 new committed artifact)
**Analogs found:** 7 / 7 (all in-tree; this is a wiring phase, no new subsystem)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/runtime/aider_edit_agent.go` (NEW) | service (AgentFn factory, daemon-dialing) | request-response (gRPC `CallTool`) | `bench/runtime/drive.go` `driveScript` (:67-124) | exact (same `OpenSession`+`activate_project`+`CallTool` shape) |
| `bench/runtime/aider_edit_cell.go` or add to `cell.go` (NEW) `runAiderEditCell` | controller (cell drive leg) | event-driven (sandbox→spawn→drive→verify→write) | `bench/runtime/rag.go` `runRAGCell` (:57) | exact (mode-name branch off the daemon spine) |
| `bench/runtime/cell.go` (MODIFY) — add `aiderEditMode` branch | controller (dispatch) | request-response | `cell.go` `baselineRagMode` branch (:434-441) | exact (byte-template for the branch) |
| `bench/runtime/result.go` (MODIFY) — add `EditFormatApplied *bool` | model (result.v2 schema) | transform (struct→JSON) | `result.go` `SwebenchRawResolved *bool` (:138-139, :228-229) | exact (additive open-key, byte-template) |
| `bench/runners/aider_edit/MODE.md` (NEW) | config (data, filesystem-as-table) | n/a | `bench/runners/baseline_rag/MODE.md` | exact (zero resolver change) |
| Test cluster (NEW): `aider_edit_agent_test.go`, `aider_edit_cell_test.go`, `result_test.go` ext, `mode_resolver_test.go` ext, aggregator golden | test (hermetic golden + HELIX_BIN sentinel) | n/a | `loader_test.go` (hermetic fake-tester) + `byte_reproducible_test.go` (:25-54) | role-match |
| `bench/reports/<run>/` (NEW committed) | config (committed baseline artifact) | n/a | existing `bench/reports/` + `renderAll` (`aggregate.go:193`) | role-match |
| `bench/datasets/aider-polyglot/loader.go` (MODIFY, thin) — export `LoadExercise`/`NativeTestCommand` | utility (pure-stdlib accessors) | n/a | `loadExercise` (:96), `nativeTestCommand` (:280) | exact (delegate wrappers) |

## Pattern Assignments

### `bench/runtime/aider_edit_agent.go` (service, request-response) — NEW

**Analog:** `bench/runtime/drive.go` `driveScript` (:67-124) + `loader.go:204` (`AgentFn` signature)

**Critical boundary:** This is daemon-dialing → it MUST live in `bench/runtime`, NOT in the `aiderpolyglot` loader leaf (stdlib-only — `vet-ablation-leakage` forbids the leaf importing `internal/forwarder`).

**Imports + session-open pattern** (`drive.go:1-13`, `drive.go:67-95`):
```go
import (
	"github.com/agenthands/helix/internal/forwarder"
	// ...
)

// A bench driver needs no structured logging surfaced; discard it.
logger := slog.New(slog.NewTextHandler(nopWriter{}, nil))

sess, err := forwarder.OpenSession(ctx, sockPath, "", logger, "bench-runtime")
if err != nil {
	return nil, fmt.Errorf("bench/runtime: open daemon session: %w", err)
}
defer sess.Close()

// activate_project: point the daemon workspace at the cloned repo so relative
// paths resolve. Harness setup — NOT recorded as a scripted StepResult.
activateCtx, activateCancel := context.WithTimeout(ctx, driveDeadline)
res, err := sess.CallTool(activateCtx, "activate_project", map[string]any{"repo_path": workspaceRoot})
activateCancel()
if err != nil { /* transport failure is fatal */ }
if res != nil && res.IsError { /* surface tool error */ }
```

**Per-edit CallTool pattern** (`drive.go:107-119`):
```go
stepCtx, stepCancel := context.WithTimeout(ctx, driveDeadline)
callRes, callErr := sess.CallTool(stepCtx, step.Tool, orEmptyArgs(step.Args))
stepCancel()
if callErr != nil { /* record StepResult.Err; do NOT abort */ }
if callRes != nil && callRes.IsError { /* record verb error */ }
```

**AgentFn signature to implement** (`loader.go:202-204`):
```go
// AgentFn lets the agent edit the solution stub in workDir given the current prompt.
type AgentFn func(ctx context.Context, ex *Exercise, workDir string, prompt string) error
```

**Deterministic transform (the baseline driver):** for each `ex.Config.Files.Solution[i]` stub, read the reference body from `ex.Config.Files.Example[i]` (`.meta/example.<ext>`, confirmed present in the vendored tree) under `ex.SrcDir`, then `sess.CallTool(ctx, "replace_in_file", {...})`. Set `*applied = true` on verb success, `*applied = false` on verb error. **Verb MCP tool names are the underscore form** (`replace_in_file`, `fuzzy_edit`, `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`).

> **`[ASSUMED]` — confirm at code time:** the `replace_in_file` arg keys (`relpath`/`content`) must be read from the tool's `InputSchema` in `internal/kernel/fileops/` before coding (Assumption A1, RESEARCH.md:351).

---

### `bench/runtime/aider_edit_cell.go` `runAiderEditCell` (controller, event-driven) — NEW

**Analog:** `bench/runtime/rag.go` `runRAGCell` (:44-130+)

**Mode-name const pattern** (`rag.go:34-37`):
```go
// baselineRagMode is detected BY MODE NAME in RunCell (NOT a MODE.md frontmatter
// key — the resolver is strict two-key with KnownFields(true)).
const baselineRagMode = "baseline_rag"
// → add: const aiderEditMode = "aider_edit"
```

**Sandbox / preserve-on-failure spine** (`rag.go:57-86`) — reuse verbatim:
```go
sb, err := benchsandbox.New(cfg.RunID, cfg.HelixBin, cfg.OutDir)
if err != nil { return res, fmt.Errorf("bench/runtime: create sandbox: %w", err) }
res.ScratchDir = sb.Root
cleanedUp := false
cleanup := func() { if cleanedUp { return }; _ = sb.Cleanup(); cleanedUp = true }
preserve := func(e error) (CellResult, error) {
	res.ScratchPreserved = true
	fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s failed; preserving scratch at %s: %v\n",
		cfg.Task, cfg.Mode, sb.Root, e)
	return res, e
}
if err := sb.Prepare(cfg.Task, cfg.Mode); err != nil { return preserve(...) }
```

**Divergence from the daemon spine** (`rag.go` doc :44-56): like `runRAGCell`, the aider-edit branch OWNS its own fixture path resolution (the standard `cellSeedDir`/`task.json` seed-dir assumptions don't apply — the aider fixtures live at `bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/<ex>`). Branch BEFORE the seed-dir clone. (RESEARCH Open Q3 recommends (a) — own the path resolution.)

**Cell sequence to assemble:** `subprocess.StartDaemon` (Unix socket, HTTP off) → `aiderpolyglot.LoadExercise(...)` → build deterministic `AgentFn` (above) + live `TestFn` (below) → `aiderpolyglot.RunExercise(ctx, ex, workDir, testFn, agentFn)` VERBATIM → kill daemon + PID-gated tap (mirror `cell.go:603-622`) → `SynthCCTap(editSteps)` → `BuildResult(ResultInput{..., EditFormatApplied:&applied})` → `Validate` → `writeDurable`.

**SynthCCTap zero-Usage leg** (`cctap.go:33-85`) — record the EDIT-verb `CallTool`s as `runner.StepResult`s; `Usage` stays the zero value (no model → tokens legitimately 0, never fabricated, `cctap.go:26-27,73-83`).

---

### `bench/runtime/cell.go` (controller, dispatch) — MODIFY

**Analog:** the `baselineRagMode` branch at `cell.go:434-441` — byte-template:
```go
if cfg.Mode == baselineRagMode {
	_ = profileName // resolver validated the MODE.md two-key frontmatter as a side effect
	return runRAGCell(ctx, cfg, res)
}
```
Add a sibling immediately after (or before, near the same fairness-gate anchor):
```go
if cfg.Mode == aiderEditMode {
	_ = profileName
	return runAiderEditCell(ctx, cfg, res)
}
```
Detection is BY MODE NAME (NOT a MODE.md frontmatter key — the resolver is strict two-key). The unconditional fairness gate + path layout (`ResultPath`/`MergedTracePath`) already ran above.

---

### `bench/runtime/result.go` (model, transform) — MODIFY

**Analog:** `SwebenchRawResolved *bool` — byte-exact template.

**`ResultInput` field** (mirror `result.go:138-139`; doctrine comment :130-137):
```go
// *bool (NOT bool) WITH omitempty so a nil drops the key but a literal false
// (edit-format NOT applied — load-bearing, Pitfall 2) is PRESERVED. Mirrors the
// SwebenchRawResolved additive-minor discipline EXACTLY: omitempty, schema_version
// stays "v2", additionalProperties stays OPEN, NOT added to required.
EditFormatApplied *bool
```

**`resultDoc` json tag** (mirror `result.go:228-229`):
```go
EditFormatApplied *bool `json:"edit_format_applied,omitempty"`
```

**`BuildResult` wiring** (the `resultDoc` literal, `result.go:264-287`):
```go
EditFormatApplied: in.EditFormatApplied,
```

**Anti-pattern (Pitfall, RESEARCH:200):** do NOT use a value-type `bool` — a literal `false` would be dropped by `omitempty`. Use `*bool`. Do NOT bump `schema_version` to v3.

---

### `bench/runners/aider_edit/MODE.md` (config, data) — NEW

**Analog:** `bench/runners/baseline_rag/MODE.md`. The strict two-key frontmatter is `mode:` + `profile:` (`mode_resolver.go:35-38`, `KnownFields(true)` :119). `ResolveProfileFromRoot` (:83-106) reads `<root>/<mode>/MODE.md` — **zero Go change** to add a 7th mode dir.

**Frontmatter to ship:**
```markdown
---
mode: aider_edit
profile: bench-full
---
```
Body documents: the polyglot-edit bench mode (EDITBENCH-02); resolves to `bench-full` (full helix EDIT-verb surface); detected BY MODE NAME in `RunCell` (like `baseline_rag`) so the cell branches to `runAiderEditCell` — the MODE.md still validates the two-key frontmatter as a side effect. `mode_resolver.go` needs ZERO change.

---

### `bench/datasets/aider-polyglot/loader.go` (utility, thin export wrappers) — MODIFY

**Analog:** the existing unexported `loadExercise` (:96-124) and `nativeTestCommand` (:280-294). `runAiderEditCell` (in `bench/runtime`) needs to load an exercise + get the native test argv, but both are package-private (only `loader_test.go` can call them today — Open Q1, A2).

**Add thin EXPORTED wrappers that delegate (pure-stdlib, preserve the leaf boundary):**
```go
func LoadExercise(dir, language string) (*Exercise, error) { return loadExercise(dir, language) }
func NativeTestCommand(language string) ([]string, error)  { return nativeTestCommand(language) }
```
`loadExercise` sets `SrcDir: dir` (:120) — load-bearing so WR-01 `restorePristineTests` actually runs (`loader.go:243-249` runs ONLY when `ex.SrcDir != ""`). Do NOT add any daemon/forwarder import here; confirm with `go vet` + `vet-ablation-leakage`.

**Live `TestFn` to build in `bench/runtime`** (NOT in the leaf — shells `exec`):
```go
func newNativeTestFn() aiderpolyglot.TestFn {
	return func(ctx context.Context, ex *aiderpolyglot.Exercise, workDir string) aiderpolyglot.TestResult {
		argv, err := aiderpolyglot.NativeTestCommand(ex.Language)
		if err != nil { return aiderpolyglot.TestResult{Passed: false, Output: err.Error()} }
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = workDir
		out, runErr := cmd.CombinedOutput()
		return aiderpolyglot.TestResult{Passed: runErr == nil, Output: string(out)}
	}
}
```

---

### Test cluster (test) — NEW

**Hermetic golden analog:** `loader_test.go` drives `RunExercise` with a scripted FAKE `TestFn` (no toolchain; the empty-`SrcDir` path skips WR-01 restore, `loader.go:228-229`). The hermetic sibling is the **SOLE authoritative proof** — it runs the deterministic AgentFn + `RunExercise` with NO `HELIX_BIN` and NO network.

**Byte-reproducibility analog:** `byte_reproducible_test.go:25-54` — double-render diff-empty + committed-golden compare:
```go
// Double-render diff-empty: the two renders are byte-identical.
assert.Equal(t, string(first), string(second), "... is NOT byte-reproducible ...")
// And the render matches the committed golden (no drift from the frozen bytes).
assert.Equal(t, string(want), string(first), "... drifted from its committed golden")
```

**HELIX_BIN "did it RUN" sentinel:** when `HELIX_BIN` IS set, assert the live cell actually produced a `result.v2.json` carrying the expected `outcome` + `edit_format_applied` (fail-CLOSED on a missing file / empty bucket / missing metric — never read absence as pass). The live leg MAY `t.Skip` when `HELIX_BIN==""` ONLY because the hermetic sibling covers its logic.

**Anti-vacuity test (Pitfall 4):** revert-and-fail — break the WR-01 restore (or feed a wrong reference body) and assert the native test FAILS, so the baseline pass is non-vacuous.

---

### `bench/reports/<run>/` committed baseline (config, committed) — NEW

**Analog:** `renderAll` (`aggregate.go:193-214`) — the SINGLE byte-stable render path (zero-RNG, sort-before-emit, double-render diff-empty). Route the baseline report through it; never hand-roll a markdown emitter.

**Determinism rules (Pitfall 3):** commit ONLY deterministic metrics (pass/fail, `edit_format_applied`, file/edit counts). EXCLUDE live latency/tokens (machine-specific → non-reproducible; latency belongs to `make bench-micro`). Scope: ONE exercise × ONE mode (`aider_edit`) × one hermetic language (**go or python** — Pitfall 5: rust/java/js fetch deps offline and aren't reproducible; `flagNonHermetic` `loader.go:313-326`). Suggested pick: `go/wordy` or `python/wordy`.

## Shared Patterns

### Daemon dial (in-process gRPC `StreamMCP`)
**Source:** `bench/runtime/drive.go:73,87,108`
**Apply to:** `aider_edit_agent.go` (the AgentFn)
`forwarder.OpenSession(ctx, sockPath, "", logger, name)` → `sess.CallTool(ctx, "activate_project", {"repo_path": workspaceRoot})` → per-edit `sess.CallTool(ctx, verb, args)` → `defer sess.Close()`. NEVER shell `helix <verb>` per edit (Phase 94 removed the stdio head).

### WR-01 anti-tamper (do NOT bypass)
**Source:** `bench/datasets/aider-polyglot/loader.go:177-184,243-249`
**Apply to:** `runAiderEditCell` (call `RunExercise` VERBATIM)
`restorePristineTests` restores the pristine TEST file AFTER the agent edits but BEFORE each grade, ONLY when `ex.SrcDir != ""`. The AgentFn must edit ONLY `files.solution` stubs, never `files.test`. Do NOT move restore into the AgentFn.

### Path-segment validation BEFORE any `filepath.Join` (V5)
**Source:** `loader.go:81` (`validatePathSegment`), `loader.go:129-132` (`copyFile` guard), `mode_resolver.go:54` (`validateModeName`), `cell.go:301` (`validateCellKey`)
**Apply to:** all new path joins (exercise/mode/relpath). Reuse the existing validators — never join a raw exercise/mode/relpath.

### Additive open `result.v2` key (`*bool`/`omitempty`, no v3 bump)
**Source:** `result.go:130-139,222-229`
**Apply to:** `EditFormatApplied`. `schema_version` stays `"v2"`, `additionalProperties` stays OPEN, NOT added to `required`.

### Fail-CLOSED on missing/empty result (no vacuous pass)
**Source:** `cell.go:99-141,160-179` (`scrapeSemanticReadsTotal` `present` signal)
**Apply to:** the HELIX_BIN sentinel + baseline golden — a missing `result.v2.json`, empty run dir, or missing `edit_format_applied` line is a HARD ERROR, never a pass.

## No Analog Found

None. Every component has a byte-exact or strong in-tree precedent — this is a wiring phase. `git diff go.mod` must stay empty (no new dependency).

## Metadata

**Analog search scope:** `bench/runtime/{rag,drive,cell,result,cctap}.go`, `bench/datasets/aider-polyglot/loader.go`, `bench/runners/{mode_resolver.go,baseline_rag/MODE.md}`, `bench/aggregator/{aggregate.go,byte_reproducible_test.go}`
**Files scanned:** 10
**Pattern extraction date:** 2026-06-23
