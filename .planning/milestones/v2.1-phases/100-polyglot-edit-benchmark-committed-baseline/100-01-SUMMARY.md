---
phase: 100-polyglot-edit-benchmark-committed-baseline
plan: 01
subsystem: testing
tags: [aider-polyglot, bench, editbench, replace_in_file, forwarder, gRPC, result-schema, mode-resolver, tdd]

# Dependency graph
requires:
  - phase: 85-adapter-aider
    provides: aiderpolyglot loader leaf (RunExercise, loadExercise, nativeTestCommand, WR-01 restorePristineTests)
  - phase: 87-swebench-rescore
    provides: SwebenchRawResolved *bool additive-open-key byte-template on result.v2
  - phase: 94-cli-surface-retirement
    provides: forwarder.OpenSession in-process gRPC StreamMCP dial (drive.go driveScript)
provides:
  - "EditFormatApplied *bool additive-open key on result.v2 (literal false preserved, nil drops, schema stays v2)"
  - "Exported LoadExercise / NativeTestCommand pure-stdlib wrappers on the aiderpolyglot leaf (boundary preserved)"
  - "bench/runners/aider_edit/MODE.md (mode aider_edit -> profile bench-full, zero mode_resolver change)"
  - "newDeterministicEditAgent (daemon-dialing EDIT-verb AgentFn) + newEditAgentWithApply (testable core seam) + newNativeTestFn (live TestFn) + const aiderEditMode in bench/runtime"
affects: [100-02, aider-edit-cell, committed-baseline-report]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Additive-open-key on result.v2: *bool + omitempty so a literal false survives; schema_version stays v2; not in required"
    - "Leaf-boundary preservation: thin exported stdlib delegators keep daemon-dial logic out of the aiderpolyglot leaf (go list -deps clean)"
    - "Injectable apply seam (applyEditFn): live path = replace_in_file CallTool; hermetic test = in-process file write (sole authoritative proof, no HELIX_BIN/network)"

key-files:
  created:
    - bench/runtime/aider_edit_agent.go
    - bench/runtime/aider_edit_agent_test.go
    - bench/runtime/result_edit_format_test.go
    - bench/datasets/aider-polyglot/loader_export_test.go
    - bench/runners/aider_edit/MODE.md
  modified:
    - bench/runtime/result.go
    - bench/datasets/aider-polyglot/loader.go
    - bench/runners/mode_resolver_test.go

key-decisions:
  - "replace_in_file whole-stub replacement via current-body-as-pattern (REAL keys path/pattern/replacement/is_regex; research relpath/content was wrong)"
  - "Edit-apply seam dropped *sess from its signature; the live seam closes over the open session instead, keeping the test seam daemon-free"
  - "Anti-vacuity: edit_format_applied records apply-success; pass/fail records grade — a wrong applied edit still sets applied=true but grades false"

patterns-established:
  - "Pattern 1: additive-open result.v2 key mirroring SwebenchRawResolved byte-for-byte (*bool, omitempty, v2, not required)"
  - "Pattern 2: hermetic sibling as sole authoritative proof — RunExercise driven with a fake grading TestFn + in-process apply seam, NO daemon/HELIX_BIN/network"

requirements-completed: [EDITBENCH-01, EDITBENCH-02, EDITBENCH-03]

# Metrics
duration: ~18min
completed: 2026-06-23
status: complete
---

# Phase 100 Plan 01: EDITBENCH Wiring Substrate Summary

**Deterministic daemon-dialing EDIT-verb AgentFn (replace_in_file whole-stub replace) + live native TestFn + additive `edit_format_applied *bool` result.v2 key + `aider_edit` MODE.md, all proven hermetically with no HELIX_BIN/network.**

## Performance

- **Duration:** ~18 min
- **Tasks:** 3
- **Files modified:** 8 (5 created, 3 modified)

## Accomplishments
- `EditFormatApplied *bool` open key on `ResultInput`/`resultDoc`, wired through `BuildResult` — literal false preserved, nil drops, schema_version stays `v2`, not in `required` (mirrors `SwebenchRawResolved` exactly).
- Exported `LoadExercise` / `NativeTestCommand` thin pure-stdlib delegators on the `aiderpolyglot` leaf — daemon-dial logic stays out of the leaf (`go list -deps` shows no `internal/forwarder`/`internal/kernel`/`bench/runtime`); `loadExercise`/WR-01 internals untouched.
- `bench/runners/aider_edit/MODE.md` (strict two-key `mode: aider_edit` / `profile: bench-full`) resolves with **zero** `mode_resolver.go` change (filesystem-as-table growth).
- `newDeterministicEditAgent` (in-process `forwarder.OpenSession` + `activate_project` + `replace_in_file` with REAL `path`/`pattern`/`replacement`/`is_regex` keys), `newEditAgentWithApply` (testable core behind an `applyEditFn` seam), `newNativeTestFn` (live native-argv TestFn, fail-closed unknown language), and `const aiderEditMode` for the Plan 02 cell branch.
- Hermetic sole-authoritative proof + anti-vacuity test: `RunExercise` driven with a fake grading TestFn + in-process apply seam — a wrong edit grades false.

## Task Commits

1. **Task 1: Additive edit_format_applied *bool open key (EDITBENCH-03)** — `bedacf92` (feat, TDD test+impl)
2. **Task 2: Thin exported loader accessors + aider_edit MODE.md (EDITBENCH-02 + loader export)** — `671b221e` (feat)
3. **Task 3: Deterministic daemon-dialing EDIT-verb AgentFn + live TestFn (EDITBENCH-01)** — `ab9700b4` (feat, TDD test+impl)

_TDD tasks (1, 3) followed RED (failing compile) → GREEN (impl) in one atomic commit each; the failing-first state was verified in-loop before implementing._

## Files Created/Modified
- `bench/runtime/result.go` — added `EditFormatApplied *bool` to `ResultInput` + `resultDoc` (json `edit_format_applied,omitempty`) and `BuildResult` literal wiring.
- `bench/runtime/result_edit_format_test.go` — `TestEditFormatApplied`: false-preserved / true-marshals / nil-drops / schema-v2-valid.
- `bench/datasets/aider-polyglot/loader.go` — added exported `LoadExercise` / `NativeTestCommand` delegators (no new import).
- `bench/datasets/aider-polyglot/loader_export_test.go` — `TestLoadExerciseExported` (SrcDir set on go/wordy fixture) + `TestNativeTestCommandExported` (go/python argv + unknown fail-closed).
- `bench/runners/aider_edit/MODE.md` — the polyglot-edit mode entry.
- `bench/runners/mode_resolver_test.go` — `TestResolveAiderEdit` (resolves to `bench-full` via `ResolveProfileFromRoot`, zero resolver change).
- `bench/runtime/aider_edit_agent.go` — `aiderEditMode`, `applyEditFn` seam, `newEditAgentWithApply`, `newDeterministicEditAgent`, `newNativeTestFn`.
- `bench/runtime/aider_edit_agent_test.go` — hermetic + anti-vacuity + native-TestFn tests.

## Decisions Made
- **`replace_in_file` whole-stub replacement via current-body-as-pattern.** The REAL `ReplaceInFileArgs` keys are `path`/`pattern`/`replacement`/`is_regex` (confirmed `internal/kernel/fileops/tools.go:52-59`); RESEARCH's `relpath`/`content` were wrong. The deterministic transform reads the current stub body and passes it verbatim as the literal `pattern` (is_regex:false) with the reference body as `replacement`.
- **Apply seam signature `applyEditFn(ctx, workDir, stub, body)` — no `*sess` param.** The live seam closes over the already-open session; this keeps the hermetic test seam (an in-process file write) free of any daemon type, so the sole-authoritative proof needs no daemon.
- **Anti-vacuity separation.** `edit_format_applied` records that the edit format was applied (apply-seam success); pass/fail records that the applied edit graded green. A wrong applied edit sets `applied=true` yet grades false — the test proves the baseline pass is non-vacuous.

## Deviations from Plan

None - plan executed exactly as written. The plan's Task 2 acceptance grep (`grep -c 'internal/forwarder|internal/kernel|bench/runtime' loader.go` == 0) is a heuristic that necessarily matches the leaf's own pre-existing leaf-discipline doc comments (loader.go:10-11,77-78) and the new wrapper doc comments; the authoritative boundary check — `go list -deps ./bench/datasets/aider-polyglot/` — shows NONE of the forbidden packages are imported (clean leaf). New wrapper comments were phrased to minimize literal forbidden-package strings.

## Issues Encountered
None. Baseline `go test` of the three packages was green before changes; each TDD task's RED state was a compile failure (missing symbol) verified before implementing.

## User Setup Required
None - no external service configuration required. `git diff go.mod` is empty (zero new deps).

## Next Phase Readiness
- Plan 02 can now assemble `runAiderEditCell`: `LoadExercise` → `newDeterministicEditAgent(sockPath, &applied)` + `newNativeTestFn()` → `RunExercise` VERBATIM → `BuildResult(ResultInput{..., EditFormatApplied:&applied})` → `Validate` → durable write, keying the cell branch on `aiderEditMode`.
- The live daemon-dial path is exercised end-to-end under `HELIX_BIN` in Plan 02 (Plan 01 proves it hermetically only).
- WR-01 stays intact: `RunExercise`/`restorePristineTests` were reused verbatim, never modified.

## Self-Check: PASSED

- Created files verified present: `bench/runtime/aider_edit_agent.go`, `bench/runtime/aider_edit_agent_test.go`, `bench/runtime/result_edit_format_test.go`, `bench/datasets/aider-polyglot/loader_export_test.go`, `bench/runners/aider_edit/MODE.md`.
- Commits verified in git log: `bedacf92`, `671b221e`, `ab9700b4`.
- Hermetic tests green: `TestEditFormatApplied`, `TestResolveAiderEdit`, `TestLoadExerciseExported`/`TestNativeTestCommandExported`, `TestAiderEditAgentHermetic`/`TestAiderEditAgentAntiTamper`/`TestNativeTestFn`. `go vet ./...` clean; `git diff go.mod` empty.

---
*Phase: 100-polyglot-edit-benchmark-committed-baseline*
*Completed: 2026-06-23*
