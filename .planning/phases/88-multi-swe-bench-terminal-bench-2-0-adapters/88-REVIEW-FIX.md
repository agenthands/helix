---
phase: 88-multi-swe-bench-terminal-bench-2-0-adapters
fixed_at: 2026-06-21T07:42:24Z
review_path: .planning/phases/88-multi-swe-bench-terminal-bench-2-0-adapters/88-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 88: Code Review Fix Report

**Fixed at:** 2026-06-21T07:42:24Z
**Source review:** .planning/phases/88-multi-swe-bench-terminal-bench-2-0-adapters/88-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (the 3 Warnings; the 4 Info findings were out of scope)
- Fixed: 3
- Skipped: 0

All three Warnings were fixed. The Info findings (IN-01..04) were explicitly out
of scope and were not touched. The phase's longwall invariants and all existing
hermetic tests still pass; the full `make vet` gate (including every custom
vettool) is green; the windows cross-compile of the changed files is clean.

## Fixed Issues

### WR-01: Terminal-Bench runner-kind seam mis-wired

**Files modified:** `bench/evaluators/terminalbench/harness.go`, `bench/evaluators/terminalbench/harness_test.go`
**Commit:** 65abb590
**Applied fix:** Added a `kind runnerKind` field to `Harness`, set by `Detect`
to the binary it actually resolved (`&Harness{bin: p, kind: k}`). Changed
`RunArgs(r)` to `RunArgs(k runnerKind, r HarnessRun)` so argv is built from the
resolved kind's `spec()`, not the global `defaultRunnerKind`; `Run` now passes
`h.kind`. Added two hermetic tests — `TestRunArgsHonorsKind` (argv carries the
GIVEN kind's dataset-flag value, anchored on `spec()` not a literal) and
`TestRunHonorsResolvedKind` (a `kind=harbor` Harness builds harbor argv, a
`kind=tb` Harness builds tb argv via the runShim seam). The two argv slices are
identical today, so the tests pin the Detect→Run wiring and will fail loudly the
moment harbor's `spec()` diverges from tb's — exactly the latent regression the
review flagged. Existing `RunArgs` call sites updated to the two-arg form.

### WR-02: `Setpgid` without `cmd.Cancel` orphans Docker descendants

**Files modified (Phase-88 adapters):** `bench/evaluators/multiswebench/harness.go`, `bench/evaluators/multiswebench/procgroup_unix.go`, `bench/evaluators/multiswebench/procgroup_windows.go`, `bench/evaluators/terminalbench/harness.go`, `bench/evaluators/terminalbench/procgroup_unix.go`, `bench/evaluators/terminalbench/procgroup_windows.go`
**Commit:** 06e8ecd8
**Applied fix:** Added a build-tag-split `setGroupKillCancel(cmd *exec.Cmd)`
helper alongside the existing `procGroupAttr` in both adapters. The unix variant
wires `cmd.Cancel` to `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)` (negative
pid → whole process group, mirroring `bench/runtime/subprocess/ragserver.go`)
plus `cmd.WaitDelay = 10s` for SIGKILL escalation; the windows variant is a no-op
(no POSIX process groups — the existing Win32 Job Object limitation is already
documented on `procGroupAttr`). Both `Run` methods now call `setGroupKillCancel(cmd)`
right after setting `SysProcAttr`. The build-tag-split keeps the windows
cross-compile clean (no unix-only `syscall.Kill` in the windows TU).

**Additional commit (template parity — swebench, Phase-87):**
**Files modified:** `bench/evaluators/swebench/harness.go`, `bench/evaluators/swebench/procgroup_unix.go`, `bench/evaluators/swebench/procgroup_windows.go`
**Commit:** 17bf0e95
**Applied fix:** The review noted the defect was inherited verbatim from the
Phase-87 swebench template and that the identical one-line fix MAY be applied
there as a SEPARATE commit if it does not regress swebench tests. It is a
trivial, identical addition, so the same `setGroupKillCancel` was added to the
swebench harness. Kept as its own commit; swebench `go build` / `go vet` /
`go test` all stay green.

### WR-03: `cellKey` panic aborts the entire long-wall run

**Files modified:** `bench/longwall/checkpoint.go`, `bench/longwall/scheduler.go`, `bench/longwall/checkpoint_test.go`, `bench/longwall/scheduler_test.go`
**Commit:** 2f3f0479
**Applied fix:** Changed `cellKey` from `panic(err)` to returning
`(string, error)`; validation stays TOTAL (every empty / separator-bearing /
traversal-bearing segment is still refused) and now iterates the segments in a
FIXED order so the surfaced error is deterministic (the previous `map` range
order was randomized). `CellID.Key()` now returns `(string, error)`.
`Scheduler.Run` resolves the key first and, on error, counts the cell `Failed`
and `continue`s — a malformed coordinate (e.g. a Lang/Task from a bad dataset
dir or row) degrades to a per-cell failure instead of tearing down the whole
resilience-oriented pass. Added `TestCellKeyRejectsMalformed` (every bad shape
returns an error + empty key, no panic) and `TestMalformedCellDegradesNotAborts`
(a batch with one bad cell sandwiched between two valid ones still runs BOTH
valid cells, counts the bad one Failed, and writes only the two valid
checkpoints). Existing test call sites updated via `mustCellKey`/`keyOf`
helpers. The four longwall invariants (round-trip / resume-skips-done /
idempotent re-entry / failed-not-persisted) and the SC#3 48h-advance injected-
clock test all still pass.

## Skipped Issues

None — all three in-scope Warnings were fixed.

## Verification

All required gates pass:

- `go build ./...` — clean
- `go vet ./bench/...` — clean
- `go test -count=1 ./bench/evaluators/multiswebench/... ./bench/evaluators/terminalbench/... ./bench/longwall/...` — all `ok` (plus swebench `ok` for the parity commit)
- `make vet` (full gate, all custom vettools) — clean
- `GOOS=windows GOARCH=amd64 go build ...` — the changed files compile clean for
  windows/amd64. The only windows errors are 3 PRE-EXISTING transitive
  `build constraints exclude` failures from the duckdb / treesitter-bindings
  chain pulled in via `bench/runtime` → `internal/daemon`; these are present at
  the original phase tip (`f3eaced5`, verified) and are unrelated to these fixes.
- Longwall invariants — round-trip / resume-skips-done / idempotent re-entry /
  failed-not-persisted all pass; no regression.

Note: `bench/evaluators/multiswebench/config.go` carries a pre-existing `gofmt`
deviation (struct-tag comment alignment). It was untouched by these fixes and was
deliberately NOT reformatted to keep each fix narrowly scoped.

---

_Fixed: 2026-06-21T07:42:24Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
