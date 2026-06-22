---
phase: 85-aider-polyglot-adapter-7-remaining-per-language-runners
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 18
files_reviewed_list:
  - bench/languages/python/runner.go
  - bench/languages/java/runner.go
  - bench/languages/csharp/runner.go
  - bench/languages/cpp/runner.go
  - bench/languages/rust/runner.go
  - bench/languages/typescript/runner.go
  - bench/languages/javascript/runner.go
  - bench/languages/go/runner.go
  - bench/datasets/aider-polyglot/loader.go
  - bench/datasets/aider-polyglot/clone.go
  - bench/datasets/aider-polyglot/pin.go
  - bench/container/run.go
  - bench/container/engine.go
  - bench/aggregator/aggregate.go
  - bench/aggregator/report.go
  - bench/runtime/result.go
  - bench/runtime/cell.go
  - cmd/helix-bench/verify_licenses.go
  - cmd/helix-bench/main.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 85: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 18
**Status:** issues_found

## Summary

The 7 per-language runners are near-identical clones of the sound `bench/languages/go/runner.go` template, and the shared pattern is correct: `Passed = (exitCode == 0)` is the authoritative gate in **every** runner, the `ctx.Err()→infra` vs `*exec.ExitError→exit-code` split is uniform, every parser is total on malformed/empty/truncated input (returns `nil`, never panics), and no parser infers pass from the absence of failing rows. Rust correctly parses libtest TEXT (not `--message-format=json`) with the malformed-line-resync scanner and a grown buffer. The subprocess seams (clone, container.Run, runner exec) use fixed argv, strict env allowlists, no shell, `--` terminators, and `isHexSHA1`/`isHexSHA256`/`isValidMountPath` fail-closed validators with leading-`-` flag-smuggling rejection. The license gate strict-decodes with `KnownFields(true)` and hard-fails on empty `license`/`license_sha256` and zero blocks. The additive `language` doc key is correct `omitempty` and the per-language aggregator slice is purely additive and does not perturb the `(task,mode)` grouping or the RNG.

The defects below are not in the runner parse path. The most material is **WR-01**: the Aider 2-attempt driver (`RunExercise`) documents and ships a `restorePristine` step that restores the pristine **test** file each attempt (the upstream anti-tamper invariant), but the driver loop never calls it — so the invariant is unimplemented in the orchestration path, and the only proof that the protocol is "correct" is a hermetic test that also never exercises restoration. The remaining items are robustness/clarity issues.

## Warnings

### WR-01: 2-attempt driver never restores the pristine test file it documents — upstream anti-tamper invariant is unimplemented

**File:** `bench/datasets/aider-polyglot/loader.go:159-171, 203-234`
**Issue:** The package and `RunExercise` doc-comments state the protocol "restores the exercise's pristine solution stub(s) AND test file(s) ... exactly the upstream benchmark.py 'restore solution from pristine before running tests' step" and that the test file is "restored pristine each attempt." `restorePristine` exists to do exactly this. But `RunExercise` (the driver loop) never calls `restorePristine` (nor `copyFile`) — confirmed: `restorePristine` has **no caller outside `loader_test.go`**. The loop is: `agent.editOrCall(...)` → `runTests(...)`. Nothing re-establishes the pristine test file between attempt 1 and attempt 2, and nothing establishes it before attempt 1. Consequences:
  - The upstream invariant that the agent cannot tamper with the test file to force a green run is **not enforced** in the shipped driver. An `AgentFn` (or the agent it wraps) that modifies a `files.test` path makes a subsequent `runTests` pass spuriously — `Passed=exitCode==0` is honest about the exit code, but the exit code itself was produced against a tampered test.
  - On attempt 2 the solution stub is also not reset, so attempt 2 runs against attempt-1's edited tree. That may be intentional (iterative fixing), but the *test*-file reset is the load-bearing one and is absent.
  - The hermetic `loader_test.go` proof of "the 2-attempt protocol" therefore does not cover the restoration step, so a success criterion that rests on "tests restored pristine each attempt" is proven only by a test that does not exercise it.

**Fix:** Call `restorePristine` inside the loop before each `runTests` (and require the caller to pass `srcRoot`/`workRoot`, or thread them onto `Exercise`). Sketch:
```go
for i := 0; i < tries; i++ {
    res.Attempts++
    if err := agent.editOrCall(ctx, ex, workDir, prompt); err != nil { ... continue }
    // Restore pristine test files (and, per upstream, leave the solution stub as edited)
    // so the agent cannot tamper with the test to force a pass.
    if err := restorePristineTests(ex, ex.SrcDir, workDir); err != nil {
        res.LastOutput = err.Error()
        continue
    }
    attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
    tr := runTests(attemptCtx, ex, workDir)
    cancel()
    ...
}
```
At minimum, if restoration is deliberately deferred to a later phase, the doc-comments must stop claiming the test file is "restored pristine each attempt," because that wording asserts a guarantee the code does not provide.

### WR-02: Rust `nativeTestCommand` runs ignored tests but the live driver / runner cannot reconcile that with the libtest `ignored` parse

**File:** `bench/datasets/aider-polyglot/loader.go:254-256` and `bench/languages/rust/runner.go:97-127, 141-179`
**Issue:** Two Rust test invocations coexist with divergent semantics, and only one is wired to a parser:
  - The Aider native command is `cargo test -- --include-ignored` (`loader.go:255`). With `--include-ignored`, `#[ignore]` tests are **executed**, so libtest prints them as `test NAME ... ok|FAILED`, never `ignored`. That is fine for the *native* path (exit code is the gate).
  - The TOOLBENCH runner (`rust/runner.go`) runs plain `cargo test` (no `--include-ignored`) and `parseLibtestText` maps `ignored → Skipped=true`. Also fine in isolation.

  The risk is the documented intent that these are "the same concept": the package header says the adapter uses the dataset's native commands "never the TOOLBENCH runner argv," but there is **no live `TestFn`** that actually shells out to `nativeTestCommand` (it has no non-test caller). So the only Rust pass/fail measurement that can reach a parser today is the TOOLBENCH `cargo test` path, whose `--include-ignored`-less invocation will mark a dataset's `#[ignore]`d acceptance tests as `Skipped` rather than running them — silently under-counting the test surface for any Aider Rust exercise that gates acceptance behind `#[ignore]` (the Exercism Rust convention is exactly that: all but the first test are `#[ignore]`). Because `Passed=exitCode==0` and ignored tests don't run, a stub that compiles but implements nothing can exit 0 and read as `Passed=true`.
**Fix:** When the Rust runner is used against the Aider Polyglot dataset, it must run `cargo test -- --include-ignored` to match the dataset's acceptance convention; otherwise an unimplemented Rust solution passes vacuously (compiles, only the one non-ignored test runs, exit 0). Either wire the live `TestFn` to `nativeTestCommand` (so the `--include-ignored` argv is actually used), or add `--include-ignored` to the Rust TOOLBENCH runner for Polyglot tasks. As-is the "Rust passes" measurement is not trustworthy for ignored-gated exercises.

### WR-03: `cloneArgs` dest-leaf validation is structured so a real traversal leaf is silently accepted

**File:** `bench/datasets/aider-polyglot/clone.go:84-92`
**Issue:** The dest-leaf guard is:
```go
if err := validatePathSegment(filepath.Base(destDir), "clone dest"); err != nil {
    if filepath.Base(destDir) == ".." || strings.Contains(filepath.Base(destDir), "/") {
        return nil, err
    }
}
```
`validatePathSegment` already rejects a leading dot (`strings.HasPrefix(name, ".")`), so a leaf like `.git` or `.ssh` makes `validatePathSegment` return a non-nil `err`, but the inner re-check (`== ".." || contains "/"`) is false, so the error is **swallowed and the path is accepted**. The comment says the intent is to allow a generated `t.TempDir()` leaf while still rejecting traversal, but the implementation re-derives a *narrower* predicate that no longer matches `validatePathSegment`'s contract: any leading-dot leaf (and `filepath.Clean`-rewritable leaves on Windows via `\`) now passes. `filepath.Base("..")` is `".."` so the literal `..` case is caught, but `a/../..` style or a leading-dot leaf is not. This is defense-in-depth (the sha and URL are validated, and `destDir` is `clonePath(sha)` in the wired path), so it is not independently exploitable today — but the guard does not do what its name/comment claims and will not catch a future caller passing a hostile leaf.
**Fix:** Decide the contract explicitly. If the goal is "reject `..` and separators but allow generated temp leaves," check those conditions directly instead of calling `validatePathSegment` and then discarding most of its rejections:
```go
leaf := filepath.Base(destDir)
if leaf == ".." || strings.ContainsAny(leaf, `/\`) {
    return nil, fmt.Errorf("aiderpolyglot: cloneArgs: unsafe clone dest leaf %q", leaf)
}
```
Relying on `validatePathSegment` and then ignoring its error for the leading-dot case is the trap — it reads as "validated" but is not.

## Info

### IN-01: Entire `aiderpolyglot` adapter and `container.Run` have no production caller — success criteria rest on unit tests only

**File:** `bench/datasets/aider-polyglot/loader.go`, `clone.go`; `bench/container/run.go:121`
**Issue:** `Clone`, `RunExercise`, `restorePristine`, `nativeTestCommand`, and `Engine.Run` are reachable only from `*_test.go` (verified by grep across `bench/` and `cmd/`). For a "dataset-loader-only adapter" phase this is an expected staging state, and the live legs are correctly network-gated/skip-clean. Flagging per the instruction to surface any success criterion whose only proof is a gated/skipped or unit-only test: the `--network=none` enforcement (`runArgs` emits `--network=none` iff `netNone`) is proven by the hermetic argv golden only — no production call site passes `netNone=true` yet, so "hermetic tasks route through `container.Run(--network=none)`" (SC#3) is not exercised end-to-end this phase. Acceptable if downstream phases wire it; not a defect in the reviewed code, but the SC should not be considered proven by these files alone.
**Fix:** None required this phase; ensure the wiring phase adds an integration test that asserts `netNone=true` actually reaches a real (or shimmed) engine invocation, and that `RunExercise` is driven with a live `TestFn`.

### IN-02: `headSHA` builds an `exec.Command` without context — not cancellable

**File:** `bench/datasets/aider-polyglot/clone.go:144-152`
**Issue:** Unlike `Clone`, `headSHA` uses `exec.Command` (no `CommandContext`), so a hung `git rev-parse` cannot be cancelled by an operator/timeout. It is a local, fast, no-network op used only by the live test, so impact is low, but it is inconsistent with the cancellation discipline the rest of the package advertises.
**Fix:** Accept a `ctx` and use `exec.CommandContext(ctx, "git", ...)`, or document why the local rev-parse is exempt.

### IN-03: C# `parseTRX` treats only `NotExecuted` as skipped; other non-`Passed` outcomes silently bucket as fail

**File:** `bench/languages/csharp/runner.go:138-146`
**Issue:** `parseTRX` maps `Passed → Passed=true`, `NotExecuted → Skipped=true`, everything else → fail. TRX also emits `Inconclusive`, `Pending`, `Disabled`, `Warning`, etc. Mapping `Inconclusive`/`Pending` to fail is defensible (conservative), but the comment claims the row tri-state is faithful to the outcome vocabulary while it actually collapses several non-failing states into fail. This is **detail-only** (the exit code is the gate, the rows are advisory), so it cannot flip pass/fail — noted for accuracy of the advisory rows, not as a correctness bug.
**Fix:** Optionally extend the skip set (`NotExecuted`, `Disabled`, `Pending`) or soften the comment to "any outcome other than Passed/NotExecuted is reported as a non-pass advisory row."

### IN-04: Jest/vitest `parseJestStyleJSON` is duplicated verbatim across two packages with no shared home

**File:** `bench/languages/typescript/runner.go:111-155` and `bench/languages/javascript/runner.go:114-158`
**Issue:** `jestStyleReport` and `parseJestStyleJSON` are byte-for-byte identical in the TS and JS runners (the comment in each even references the other). A future fix to the status mapping must be applied in two places or they drift. The clone discipline is intentional for the per-language *shape*, but this specific parser is shared semantics, not language-specific.
**Fix:** Hoist `jestStyleReport`/`parseJestStyleJSON` into a small shared helper (e.g. `bench/languages/internal/jestjson`) imported by both runners, or accept the duplication explicitly with a lockstep comment (as `cell.go`'s `scrapeSemanticReadsTotal`/`scanReadsTotalLine` pair already documents for its own duplicate).

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
