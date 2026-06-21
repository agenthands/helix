---
phase: 78-internal-toolbench-go-first-languagerunner-interface
reviewed: 2026-06-17T00:00:00Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - bench/languages/coverage.go
  - bench/languages/coverage_test.go
  - bench/languages/go/runner.go
  - bench/languages/go/runner_test.go
  - bench/languages/registry.go
  - bench/languages/runner.go
  - bench/runtime/cctap_test.go
  - bench/runtime/cell.go
  - bench/runtime/cell_test.go
  - bench/runtime/cross_cell_test.go
  - bench/runtime/daemon_tap_integration_test.go
  - bench/runtime/matrix.go
  - bench/runtime/matrix_test.go
  - bench/runtime/result_test.go
  - bench/runtime/store_isolation_test.go
  - bench/runtime/subprocess/daemon.go
  - bench/runtime/validate.go
  - cmd/helix-bench/main.go
  - cmd/helix-bench/run_cmd_test.go
  - Makefile
findings:
  critical: 0
  warning: 4
  info: 4
  total: 8
status: issues_found
---

# Phase 78: Code Review Report (re-review after fix pass)

**Reviewed:** 2026-06-17
**Depth:** standard
**Files Reviewed:** 23 (plus `bench/runtime/cctap.go`, `drive.go`, `result.go` read as call-chain context)
**Status:** issues_found

## Summary

This is a re-review of the Phase 78 internal-ToolBench harness after the
`b4b66c05..dce1c0d5` fix pass. I validated the four called-out fixes and hunted
for regressions the fixes may have introduced.

**Prior fixes — validation result (all genuinely fixed):**

- **WR-01 (atomic `writeDurable`)** — CORRECT. `cell.go:556-584` stages to
  `os.CreateTemp(dir, ".tmp-...")` in the **same** directory as the target (so
  `os.Rename` stays intra-filesystem and atomic), and every error path
  (`Write`/`Close`/`Chmod`/`Rename` failure) calls `os.Remove(tmpName)` before
  returning. No leftover temp on error; readers see old-or-complete, never torn.
  Last-writer-wins on a shared path is acknowledged and benign. Verified fixed.

- **WR-02 (`ccLegPresent` non-vacuity)** — CORRECT and non-vacuous.
  `ccLegPresent` (`cell.go:488-498`) now gates on a cc-side `KindToolResult` or a
  non-empty `ToolUses`, not "any `Source==cc`". `SynthCCTap(nil)` emits only a
  `SessionInit` + `Result` (no ToolResult, no ToolUses — verified in `cctap.go`),
  so the claude branch / empty-script case yields `CCLegPresent == false`. The
  signal CAN be false — the fix is real. (See WR-03 below: the *false* branch is
  never asserted by any test.)

- **WR-05 (`Setup` honors `ctx`) / WR-06 (`RunCell` calls `Setup`)** — CORRECT.
  `GoRunner.Setup` (`go/runner.go:41-43`) returns `ctx.Err()` (nil when live,
  cancellation error when already cancelled). `RunCell` (`cell.go:362-364`) now
  calls `r.Setup(ctx, repoDir)` BEFORE `r.RunTests`, routing a Setup error through
  `preserve(...)` as an infra failure. Setup is wired into the run path. Verified.

- **IN-05 (shared `validatePathSegment`)** — CORRECT. `bench/runtime/validate.go`
  holds the single predicate; `validateCellKey` (`cell.go:146-148`) and
  `validateMatrixID` (`matrix.go:87-89`) both delegate to it and keep only their
  distinct error-prefix wrapping. No leftover duplicate body. `go vet ./bench/...`
  is clean.

No regression was introduced by the fix pass itself (verified against the
`b4b66c05..dce1c0d5` diff). The findings below are genuinely-present issues at the
current state — two pre-existing process-lifecycle gaps in `sandbox.go` (in scope,
on `RunCell`'s hot path), one aggregator over-count latent in `coverage.go`, and
one test-coverage gap the WR-02 fix left behind.

## Warnings

### WR-01: Happy-path daemon kill reaps only the group leader — children leak under `--parallel`

**File:** `internal/eval/sandbox/sandbox.go:193-220`
**Issue:** `StartDaemon` puts the daemon in its own process group
(`SysProcAttr.Setpgid = true`, line 284) specifically so the whole group can be
SIGKILL'd. But `Kill()` only does `h.cmd.Process.Kill()` on the normal path
(line 197), which signals the **group leader only** — not the group. The
`syscall.Kill(-pid, SIGKILL)` group-wide reap exists ONLY in the 5s-timeout
fallback (line 216). On the overwhelmingly common happy path (daemon exits within
5s), any descendants the daemon spawned (language servers, a `go test` child) are
left orphaned, not reaped. Under `--parallel=N` this is a real per-cell process
leak that accumulates across a run. `RunCell` always reaches `Kill()`
(`cell.go:337`), so this is on the hot path for every cell.
**Fix:** Signal the group on the normal path too, then wait:
```go
func (h *DaemonHandle) Kill() error {
	if h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	pid := h.cmd.Process.Pid
	// Daemon is a group leader (Setpgid); kill the whole group so LS / go test
	// children are reaped, not just the leader.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	if err := h.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("sandbox: kill daemon %s/%s: %w", h.taskID, h.mode, err)
	}
	// ...existing 5s-bounded Wait()...
}
```

### WR-02: 5s-timeout Kill fallback returns without reaping the in-flight `Wait()`

**File:** `internal/eval/sandbox/sandbox.go:209-219`
**Issue:** When `h.cmd.Wait()` does not return within 5s, the fallback sends
`syscall.Kill(-pid, SIGKILL)` and then **returns the error immediately without
blocking on the in-flight `Wait()` goroutine** (line 217). The
`go func(){ done <- h.cmd.Wait() }()` goroutine is still parked in `Wait()`. It
will eventually complete after the group SIGKILL and send to the buffered `done`
channel (cap 1, so the send won't block), so the reap happens *eventually* — but
the function has already returned an infra error to the caller
(`cell.go:337-339` → `preserve(...)`) with no confirmation the process actually
died, and the reaping goroutine outlives the call. Pairing this with the WR-01
group-kill on the normal path shrinks how often this fallback fires.
**Fix:** After the group SIGKILL in the fallback, drain the already-running `done`
channel so `Wait()` is guaranteed to complete before returning:
```go
case <-time.After(5 * time.Second):
	_ = syscall.Kill(-h.cmd.Process.Pid, syscall.SIGKILL)
	<-done // the in-flight Wait() now returns after the group SIGKILL — reap confirmed
	return fmt.Errorf("sandbox: daemon %s/%s did not exit within 5s after kill", h.taskID, h.mode)
```

### WR-03: `ccLegPresent` false-branch (the WR-02 fix's whole point) is never asserted by any test

**File:** `bench/runtime/cell.go:488-498` (predicate); `bench/runtime/cctap_test.go`, `cross_cell_test.go:96`, `daemon_tap_integration_test.go:88`
**Issue:** The prior WR-02 fix made `ccLegPresent` meaningful by requiring a
cc-side tool event. But no test exercises the **false** branch. The integration
tests assert only `CCLegPresent == true` (scripted path). `TestSynthCCTapMerge`
counts `Source=="cc"` events directly (`cctap_test.go:123-129`) rather than
calling `ccLegPresent`, so the predicate itself has zero direct unit coverage of
either branch. A future regression that reverted `ccLegPresent` to "any
`Source==cc`" (vacuous again) would pass the entire suite — exactly the failure
mode WR-02 set out to prevent. The fix is correct but unguarded.
**Fix:** Add a unit test on `ccLegPresent` directly that asserts BOTH branches:
empty steps (`SynthCCTap(nil)`) → false; one scripted step → true. Build a
`trace.MergedTrace` from the synth events and call `ccLegPresent` on it so the
predicate, not a re-implementation, is under test.

### WR-04: `Coverage` can report `Covered > Declared` when `declared` contains a duplicate capability

**File:** `bench/languages/coverage.go:37-67`
**Issue:** `Declared` is computed from a **de-duplicated** set
(`len(declaredSet)`, line 64) while `coveredN` is incremented by iterating the raw
`declared` **slice** (lines 52-57). If a caller passes a `declared` slice with a
repeated capability that is covered in the corpus, `coveredN` double-counts it but
`Declared` does not — yielding `Covered > Declared`, an incoherent report that
would also corrupt the "10/10" gate semantics. `GoRunner.Capabilities()` returns
10 distinct values today so this is latent, but `declared` is an external
parameter (a hand-authored future runner could repeat one), and this aggregator IS
the D-11 "gaps are explicit" gate — it must not be able to over-count. The
`declaredSet` map is built but its membership is otherwise unused (only `len()` is
read), which is the tell that dedup was meant to flow into the count.
**Fix:** Count over the deduplicated set, not the raw slice:
```go
coveredN := 0
for c := range declaredSet {
	if covered[c] {
		coveredN++
	} else {
		missing = append(missing, c)
	}
}
sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
```

## Info

### IN-01: Redundant `os.Chmod` on a freshly `CreateTemp`'d file

**File:** `bench/runtime/cell.go:575-578`
**Issue:** `os.CreateTemp` already creates the file with mode 0600. The explicit
`os.Chmod(tmpName, 0600)` (line 575) is a no-op on every platform and adds an
extra error path. Harmless, but dead defensive code.
**Fix:** Drop the `Chmod` block; rely on `CreateTemp`'s 0600 default (leave a
one-line comment if the intent needs documenting).

### IN-02: `discoverTasks` union-across-languages fabricates guaranteed-to-fail cells

**File:** `cmd/helix-bench/main.go:264-297`
**Issue:** When `--tasks` is omitted and multiple `--languages` are given, the
discovered task set is the union across languages (documented at lines 202-208). A
task present only under `go/` then yields a `(rust, that-task)` cell whose seed dir
does not exist, surfacing as a per-cell infra error rather than a skip. Intentional
and documented; harmless for the default single-language `go` invocation, but with
N languages it inflates the matrix with cells guaranteed to fail. Worth a guard or
per-language discovery once the language axis is actually exercised in Phase 85.
**Fix:** Discover per-language (pair a task only with languages whose dir contains
it), or document the cross-product fan-out in the `--languages` flag help.

### IN-03: `deriveStoreOptIn` / `readTaskPrompt` parse the same `task.json` twice per cell

**File:** `bench/runtime/matrix.go:229-245, 275-307`
**Issue:** `runOneCell` may read+unmarshal `<seedDir>/task.json` twice — once in
`readTaskPrompt` (claude path) and once in `deriveStoreOptIn` (always). Two small
reads + two `json.Unmarshal` of the same file. Purely a tidiness issue (both reads
are benign-on-error), not a correctness concern at this scale.
**Fix:** Read `task.json` once into a single struct with `capability`,
`semantic_index`, and `prompt` fields and pass the decoded value to both helpers.

### IN-04: `validatePathSegment` doc comment mis-attributes which clause catches a single separator

**File:** `bench/runtime/validate.go:22-29`
**Issue:** The comment credits the `filepath.Clean` mismatch with rejecting
"redundant separators". That holds for `a//b` (Clean rewrites it) but a single
embedded separator like `a/b` is caught by the explicit
`strings.ContainsAny(name, "/\\")` clause, not by the Clean check (Clean leaves
`a/b` unchanged). Behavior is correct and fully covered by tests
(`matrix_test.go` "language separator"/"task separator"); only the comment's
attribution is muddled. Doc nit.
**Fix:** Reword to: "rejects names that `filepath.Clean` rewrites (`..`, `a//b`,
trailing slash), that contain any path separator, or that begin with a dot."

---

## Narrative Findings (AI reviewer) — scope notes

- `go vet ./bench/... ./cmd/helix-bench/... ./internal/eval/sandbox/...` is clean;
  no unused imports or dead-symbol regressions were introduced by the fix pass.
- The four called-out fixes (WR-01 atomic write, WR-02 non-vacuous
  `ccLegPresent`, WR-05/06 Setup wiring, IN-05 shared validator) are all genuinely
  and correctly applied; none were re-reported.
- The two process-lifecycle warnings (WR-01/WR-02 here) live in `sandbox.go`,
  which predates this fix pass but is in the review file list and is on `RunCell`'s
  hot path for every cell; the `Setpgid` machinery makes the leader-only happy-path
  kill an actual (not theoretical) child-reaping gap.
- The result-schema validator (`result.go` `Validate`), the matrix dispatcher's
  semaphore bound (`matrix.go` `dispatch`), the test2json parser
  (`go/runner.go` `parseTest2JSON`, re-synced in `c43e291f`), the
  `writeDurable` atomic write, and the path-traversal validators were traced and
  found correct.

---

_Reviewed: 2026-06-17_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
