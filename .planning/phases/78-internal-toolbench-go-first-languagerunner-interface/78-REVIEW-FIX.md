---
phase: 78-internal-toolbench-go-first-languagerunner-interface
fixed_at: 2026-06-17T17:40:00Z
review_path: .planning/phases/78-internal-toolbench-go-first-languagerunner-interface/78-REVIEW.md
iteration: 1
findings_in_scope: 11
fixed: 11
skipped: 0
status: all_fixed
---

# Phase 78: Code Review Fix Report

**Fixed at:** 2026-06-17T17:40:00Z
**Source review:** .planning/phases/78-internal-toolbench-go-first-languagerunner-interface/78-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 11 (6 warning + 5 info; fix_scope=all)
- Fixed: 11
- Skipped: 0

All fixes were verified green after application:
- `go vet ./bench/... ./cmd/helix-bench/... ./internal/eval/...` — clean
- `HELIX_BIN=$(pwd)/helix go test ./bench/... ./internal/eval/...` — all `ok`
  (the real-daemon `bench/runtime` integration suite ran: daemon-tap, cross-cell,
  store-isolation — all pass)
- `make bench-quick` — `1/1 cells succeeded`
- `gofmt -w` applied to every changed file

OUT-OF-SCOPE `bench/datasets/internal-toolbench/**` corpus fixtures were not
touched. Staging was per-file with explicit `git add`; the pre-existing unrelated
`.planning/` phase-57/58 deletions and untracked build binaries were left untouched.

## Fixed Issues

### WR-01: Concurrent unsynchronized writes to the same durable artifact path

**Files modified:** `bench/runtime/cell.go`
**Commit:** b4b66c05
**Applied fix:** Rewrote `writeDurable` to stage the bytes to a temp file in the
same directory (`os.CreateTemp`), `Chmod` to 0600, then `os.Rename` into place.
Rename is atomic within a filesystem, so a concurrent reader sees either the old
file or the fully-written new one — never a torn/partial file. Concurrent writers
still race for last-writer-wins, but each individual file is always complete. (The
store-isolation test's shared-OutDir collision is now benign because the write is
atomic; distinct-OutDir-per-cell, used by `cross_cell_test.go`, remains available.)

### WR-02: `CCLegPresent` Nyquist signal was structurally vacuous — requires human verification

**Files modified:** `bench/runtime/cell.go`
**Commit:** b4b66c05
**Applied fix:** `ccLegPresent` now gates on a genuine cc-side TOOL event — an event
with `Source == "cc"` AND (`Kind == trace.KindToolResult` OR a non-empty
`ToolUses`). The old `any Source==cc` check could never be false because
`SynthCCTap` always emits a `SessionInit` + `Result` pair (both `Source:"cc"`), even
for empty steps. The new predicate makes an empty-steps run correctly report
`CCLegPresent == false`. The scripted integration cells (`TestDaemonTap`,
`TestCrossCell`) drive `scripted_agent.yaml`, which produces real per-step
`ToolResult` events, so they still assert `true` — verified green.

**Note:** flagged `requires human verification` because this changes the semantics
of a load-bearing Nyquist signal. The scripted and store-isolation integration
suites pass under the new predicate, but a reviewer should confirm the intended
meaning ("the agent leg carried tool activity") matches the new gate.

### WR-03: Claude branch discarded `agent.Result` and the doc comment was wrong

**Files modified:** `bench/runtime/cell.go`
**Commit:** b4b66c05
**Applied fix:** Corrected the `case "claude"` comment to state plainly that `steps`
stays nil for the claude path, `SynthCCTap` reads only `steps` (it does not consult
the daemon leg), so the synthesized CC leg currently carries no claude tool activity
and `CCLegPresent` is `false` for a claude cell (its tool activity is observable
only via the daemon tap this phase). Documented that threading the discarded
`*agent.Result` into the CC synth is deferred to whoever finishes the claude path.
This is the doc-correction option the review offered (the faithful synth-threading
fix is out of scope for this quality pass, and the claude path is wired-not-gating).

### WR-04: `Coverage` hardcoded the `internal-toolbench` benchmark

**Files modified:** `bench/languages/coverage.go`, `bench/languages/coverage_test.go`
**Commit:** c354ce3d
**Applied fix:** Added a `benchmark` parameter to `Coverage(corpusRoot, benchmark,
lang, declared)` and joined it explicitly (`filepath.Join(corpusRoot, benchmark,
lang)`) instead of the string literal. Updated both test callers
(`TestCoverageGoIsTenOfTen`, `TestCoverageDetectsGap`) to pass
`"internal-toolbench"`.

### WR-05: `Setup`'s `ctx` parameter was unused

**Files modified:** `bench/languages/go/runner.go`
**Commit:** c43e291f
**Applied fix:** `GoRunner.Setup` now returns `ctx.Err()` (nil when live, the
cancellation error when already cancelled) instead of bare `nil`, so a future
non-trivial runner copying this no-op shape inherits the cancellation check.

### WR-06: `RunCell` never called `LanguageRunner.Setup`

**Files modified:** `bench/runtime/cell.go`
**Commit:** b4b66c05
**Applied fix:** The dispatch block now calls `r.Setup(ctx, repoDir)` before
`r.RunTests(...)` and routes a `Setup` error through `preserve` as an infrastructure
failure, honoring the full interface lifecycle the only production caller previously
skipped. No-op for the Go fixture; live for any future runner whose `RunTests`
depends on `Setup`.

### IN-01: Dead no-op `ctx == nil` branch in `runBench`

**Files modified:** `cmd/helix-bench/main.go`
**Commit:** 24721bdf
**Applied fix:** Replaced the `if ctx == nil { ctx = cmd.Context() }` no-op with a
real fallback to `context.Background()` and added the `context` import.

### IN-02: Stale package doc — `run` described as not yet wired

**Files modified:** `cmd/helix-bench/main.go`
**Commit:** 24721bdf
**Applied fix:** Updated the package comment and the root `Long` help to reflect that
`run` is wired (Phase 78: expands the matrix and dispatches cells), keeping the "not
yet implemented" wording only for `fetch-datasets` and `report`.

### IN-03: Stale `StartClaude` "intentionally unimplemented" stub comment

**Files modified:** `bench/runtime/subprocess/daemon.go`
**Commit:** dce1c0d5
**Applied fix:** Rewrote the stub comment to point at the real `StartClaude`
implementation and its committed signature in the sibling `claude.go`, instead of
claiming it is unimplemented with no committed signature.

### IN-04: `parseTest2JSON` silently swallowed malformed mid-stream lines

**Files modified:** `bench/languages/go/runner.go`
**Commit:** c43e291f
**Applied fix:** Switched from a streaming `json.Decoder` (which stops at the first
decode error) to a line-by-line `bufio.Scanner` that SKIPS a malformed line and
re-syncs on the next, so an isolated corrupt line no longer truncates the advisory
per-test detail. `Passed` remains gated on the subprocess exit code, so a skipped
line can never flip pass/fail. Grew the scanner buffer to 4 MiB for long lines and
swapped the now-unused `io` import for `bufio`.

### IN-05: `validateCellKey` / `validateMatrixID` duplicated verbatim

**Files modified:** `bench/runtime/validate.go` (new), `bench/runtime/cell.go`, `bench/runtime/matrix.go`
**Commit:** b243161e (helper + matrix.go), b4b66c05 (cell.go delegation)
**Applied fix:** Extracted a single unexported `validatePathSegment(name, kind)`
helper into the new `bench/runtime/validate.go`. Both `validateCellKey` and
`validateMatrixID` now delegate to it while preserving their distinct error-prefix
prose at the call site, so the security-relevant path-traversal predicate cannot
drift. Dropped the now-unused `strings` import in both `cell.go` and `matrix.go`.
(The cell.go delegation rides in the cell.go bundle commit because it edits the same
file as WR-01/02/03/06.)

---

_Fixed: 2026-06-17T17:40:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
