---
phase: 85-aider-polyglot-adapter-7-remaining-per-language-runners
fixed_at: 2026-06-21T00:00:00Z
review_path: .planning/phases/85-aider-polyglot-adapter-7-remaining-per-language-runners/85-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 85: Code Review Fix Report

**Source review:** 85-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (Warnings WR-01, WR-02, WR-03; Info IN-01..04 out of scope)
- Fixed: 3
- Skipped: 0

## Fixed Issues

### WR-03: `cloneArgs` dest-leaf validation silently accepted hostile leaves

**Files modified:** `bench/datasets/aider-polyglot/clone.go`, `bench/datasets/aider-polyglot/clone_test.go`
**Commit:** `1b7aa2f5`
**Applied fix:** Replaced the guard that called `validatePathSegment` and then
discarded most of its rejections (re-checking only `== ".."` / contains `/`) with
a direct `return nil, err` that PROPAGATES the validator's rejection. A
leading-dot leaf (`.git`, `.ssh`) is now rejected instead of silently accepted; a
generated `t.TempDir()` leaf (no leading dot, no separator, clean) still passes.
Added `TestCloneArgsRejectsHostileLeaf` covering `.git`, `.ssh`, `.`, and `..`
leaves.

### WR-01: 2-attempt driver never restored the pristine test file (anti-tamper gap)

**Files modified:** `bench/datasets/aider-polyglot/loader.go`, `bench/datasets/aider-polyglot/loader_test.go`
**Commit:** `fc2c8138`
**Applied fix:** Added `restorePristineTests` (restores only the graded
`files.test` path(s), not the solution stub) and wired it into the `RunExercise`
attempt loop AFTER the agent edit and BEFORE each `runTests` call, gated on
`ex.SrcDir != ""` so the existing scripted-fake unit tests (which set no SrcDir
and have no on-disk files) are unaffected. This enforces the upstream
benchmark.py invariant: the agent may iteratively edit the solution across
attempts, but the graded test file is reset every attempt so test-tampering
cannot force a spurious green. `restorePristine` now delegates its test-file leg
to `restorePristineTests`. Added `TestRunExerciseRestoresTamperedTest`: an agent
that overwrites the test file with `assert True` has that edit reverted before
grading (proven by inspecting on-disk content at grade time), while its solution
edit is preserved. Logic-bearing change, but covered by a hermetic test that
fails if the restore step is removed.

### WR-02: Rust acceptance argv must run `#[ignore]`-gated tests

**Files modified:** `bench/datasets/aider-polyglot/loader.go`, `bench/datasets/aider-polyglot/loader_test.go`
**Commit:** `8b250cee`
**Applied fix:** The Aider adapter's `nativeTestCommand("rust")` already returns
`cargo test -- --include-ignored`; the gap was that this load-bearing requirement
was neither documented in source nor guarded by an explicit regression test, and
the review flagged the risk that a do-nothing Rust stub passes vacuously when the
ignored acceptance tests are skipped. Documented WHY the flag is required (and why
the TOOLBENCH `bench/languages/rust/runner.go` deliberately keeps plain
`cargo test` for its OWN dataset, left unchanged). Added
`TestRustAcceptanceArgvRunsIgnoredTests` asserting the adapter argv contains
`--include-ignored` AND that it follows the `--` terminator so cargo forwards it
to the libtest harness. The TOOLBENCH Rust runner was intentionally NOT modified
(its plain `cargo test` is correct for its dataset, and `parseLibtestText`'s
`ignored -> Skipped` mapping stays valid there).

## Notes

- Info findings IN-01..IN-04 were out of scope (fix_scope = warnings only) and
  were not addressed.
- WR-02's adapter command was already correct in code; the fix is documentation +
  a regression guard, not a behavior change. No live `TestFn` wiring was added
  (that is IN-01's deferred staging concern for a downstream wiring phase), so the
  guard locks the argv contract at the seam that exists today.

## Verification (final, on the committed worktree state)

- `go build ./...` -> exit 0
- `go vet ./bench/... ./cmd/helix-bench/...` -> exit 0
- `go test -count=1 ./bench/...` -> exit 0 (all packages ok)
- `make vet` (full gate incl. verify-no-docker-sdk + all custom vettools) -> exit 0
- `make verify-licenses` -> exit 0
- `make verify-no-docker-sdk` -> exit 0
- `gofmt -l` on all edited files -> clean

---

_Fixed: 2026-06-21_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
