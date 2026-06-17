---
phase: 78-internal-toolbench-go-first-languagerunner-interface
reviewed: 2026-06-17T00:00:00Z
depth: standard
files_reviewed: 22
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
  - cmd/helix-bench/main.go
  - cmd/helix-bench/run_cmd_test.go
  - internal/eval/sandbox/sandbox.go
  - internal/eval/sandbox/sandbox_test.go
  - Makefile
findings:
  critical: 0
  warning: 6
  info: 5
  total: 11
status: issues_found
---

# Phase 78: Code Review Report

**Reviewed:** 2026-06-17
**Depth:** standard
**Files Reviewed:** 22
**Status:** issues_found

## Summary

The Phase 78 "internal ToolBench" harness is well-structured and unusually well-commented; the load-bearing D-03 store-isolation seam (`WithWorkingDir` → per-cell cwd → per-cell `.helix/semantic.duckdb`) is sound, the matrix dispatcher's semaphore bound is correct and genuinely unit-tested, and the Go `test2json` parser correctly treats exit code as the authoritative gate (verified empirically against Go 1.26: a compile failure yields zero test rows and a non-zero exit, never a false pass). Path-traversal validation is applied consistently at both the matrix and cell layers.

No BLOCKER-class defects were found (no injection, no auth bypass, no crash, no data-loss-on-the-happy-path). The findings are concentrated in two areas the brief flagged: (1) the shipped `store_isolation_test.go` and `RunMatrix` themselves drive two concurrent goroutines into the **same durable artifact path** with an unsynchronized `os.WriteFile`, a real filesystem data race that `go test -race` cannot detect; and (2) the `CCLegPresent` "Nyquist signal" is structurally vacuous — `SynthCCTap` always emits two `Source:"cc"` events, so the assertion can never fail. Several quality issues (dead code, a hardcoded benchmark axis, a misleading doc comment about the claude leg) round out the list.

## Warnings

### WR-01: Concurrent unsynchronized writes to the same durable artifact path (filesystem data race)

**File:** `bench/runtime/cell.go:414-423`, `bench/runtime/store_isolation_test.go:62-96`, `bench/runtime/matrix.go:174-221`
**Issue:** `RunCell` writes `result.v2.json` and `trace.json` via `writeDurable` → `os.WriteFile` (O_CREATE|O_TRUNC|O_WRONLY) at `<OutDir>/<task>/<mode>/...`, a path keyed only by `(task, mode)` with no run-index segment (acknowledged in the IN-05 comment at cell.go:185-191). The shipped `store_isolation_test.go` then launches **two cells with identical `(Task, Mode)` and a shared `OutDir`** at `--parallel=2`, so both `RunCell` goroutines call `writeDurable` on the *same* two paths concurrently. Two goroutines doing `open(O_TRUNC)+write` on one path can interleave and produce a torn/partial file; this is a real data race on the filesystem that the Go race detector does **not** observe (it only instruments memory). The test comment ("last-writer-wins — benign here") understates it: there is no atomicity, so a reader could see a half-written `trace.json`. The same hazard exists in `RunMatrix` for any caller that expands the same `(task, mode)` twice into one `OutDir`.
**Fix:** Make durable writes atomic (write to a temp file in the same dir, then `os.Rename`), and/or give each cell a distinct durable path. Minimal atomic write:
```go
func writeDurable(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path) // atomic replace
}
```
For the store-isolation test specifically, prefer distinct `OutDir`s per cell (as `cross_cell_test.go` already does with `OutDir: t.TempDir()` per goroutine) so the durable collision never arises.

### WR-02: `CCLegPresent` Nyquist signal is structurally vacuous — can never be false

**File:** `bench/runtime/cell.go:391,462-473`, `bench/runtime/cctap.go:42-79`
**Issue:** `ccLegPresent(merged)` returns true iff any merged event has `Source == "cc"`. But `SynthCCTap` **always** emits a `SessionInit` and a `Result` event, both `Source:"cc"`, even when `steps` is empty (`SynthCCTap(nil)` still produces 2 cc events). Therefore `res.CCLegPresent` is `true` unconditionally for every cell — the "is the CC leg real?" assertion in `TestDaemonTap` (line 88), `TestCrossCell` (line 96), and the documented Nyquist signal #1 cannot ever fail, including in the claude branch where `steps` is empty by design. This is exactly the "exit-code-only smoke would alias all three signals" failure mode the code comments claim to defend against, reintroduced one layer up. The assertion gives false confidence that the agent leg carried tool activity.
**Fix:** Make the signal meaningful: assert presence of a cc-side *tool* event (e.g. `KindAssistantMsg` with a non-empty `ToolUses`, or `KindToolResult`) rather than any cc event. For example:
```go
func ccLegPresent(merged trace.MergedTrace) bool {
	for _, ev := range merged.Events {
		if ev.Source == "cc" && (ev.Kind == trace.KindToolResult || len(ev.ToolUses) > 0) {
			return true
		}
	}
	return false
}
```
This makes an empty-steps scripted/claude run correctly report `CCLegPresent == false`.

### WR-03: Claude branch discards `agent.Result` and synthesizes an empty CC leg — doc comment is wrong

**File:** `bench/runtime/cell.go:307-321,369-370,310-311`
**Issue:** In the `case "claude"` branch the returned `*agent.Result` from `subprocess.StartClaude` is discarded (`if _, cerr := ...`). `steps` is never populated for the claude path, so `SynthCCTap(steps)` runs with `nil` and the CC leg carries no tool activity. The comment at cell.go:310-311 claims "the scripted StepResults are empty, so the CC leg is synthesized from the daemon side only" — but `SynthCCTap` does **not** read the daemon leg at all; it only synthesizes from `steps`. The merged trace therefore has the claude agent's real tool calls *only* on the daemon leg, and a content-free CC leg, which combined with WR-02 makes a claude cell's `CCLegPresent` misleadingly `true`. This is wired-not-gating code (never the CI default), so it is a WARNING, not a BLOCKER, but the discarded result and the inaccurate comment will mislead whoever finishes the claude path in a later phase.
**Fix:** Either thread the `agent.Result` into the CC synth (so the claude agent's tool uses populate the cc leg) or correct the comment to state plainly that the claude CC leg is currently empty and the claude path's tool activity is observable only via the daemon tap. Do not leave a comment asserting a synthesis that does not happen.

### WR-04: `Coverage` hardcodes the `internal-toolbench` benchmark, silently ignoring any other suite

**File:** `bench/languages/coverage.go:31-32`
**Issue:** `Coverage(corpusRoot, lang, declared)` builds `langDir := filepath.Join(corpusRoot, "internal-toolbench", lang)` — the benchmark segment is a string literal. The rest of the harness treats `benchmark` as a first-class axis (`Cell.Benchmark`, `cellSeedDir`, `ExpandMatrix`), and `Makefile`'s `bench` target is explicitly parameterized by `SUITE`. A caller computing coverage for any benchmark other than `internal-toolbench` would silently read the wrong directory (or get a "reading corpus dir ... no such file" error) with no indication that the benchmark axis was ignored. This is a latent correctness gap the moment a second suite is added (the package doc in `runner.go` anticipates Rust/TS/Python adapters).
**Fix:** Add a `benchmark` parameter and join it explicitly:
```go
func Coverage(corpusRoot, benchmark, lang string, declared []Capability) (CoverageReport, error) {
	langDir := filepath.Join(corpusRoot, benchmark, lang)
	...
}
```
Update the single caller in `coverage_test.go` accordingly.

### WR-05: `Setup`'s `ctx` parameter is unused — a cancelled context is silently ignored

**File:** `bench/languages/go/runner.go:36-38`
**Issue:** `func (GoRunner) Setup(ctx context.Context, repoDir string) error { return nil }` accepts a `context.Context` but never consults it. The interface contract (`runner.go:66-67`) documents Setup as "dependency fetch, build cache warm-up" — operations a future runner will make cancellable. For the Go no-op this is harmless today, but the no-op silently ignores an already-cancelled context, and `RunCell` never calls `Setup` at all (see WR-06), so the interface method is both unexercised and not ctx-aware. When a non-trivial runner (Rust `cargo fetch`, npm install) implements this, a copy of this no-op shape risks ignoring cancellation.
**Fix:** For the Go runner, at minimum honor cancellation: `return ctx.Err()` (returns nil when not cancelled). More importantly, ensure callers actually invoke `Setup` (WR-06) so the seam is real rather than dead.

### WR-06: `RunCell` never calls `LanguageRunner.Setup` — half the interface is dead in the only caller

**File:** `bench/runtime/cell.go:350-366`
**Issue:** The dispatch block resolves `r := languages.RunnerFor(...)` and calls `r.RunTests(...)`, but never calls `r.Setup(ctx, repoDir)` beforehand. `Setup` is a declared method of the `LanguageRunner` contract intended to run "pre-test preparation (dependency fetch, build cache warm-up)" before `RunTests`. For the hermetic Go fixture this happens to be a no-op so nothing breaks, but the contract is silently unhonored: any future runner whose `RunTests` depends on `Setup` having run (e.g. fetching modules into an offline cache) will fail when driven through `RunCell`. The interface promises a lifecycle the only production caller does not execute.
**Fix:** Call `Setup` before `RunTests` and route its error through `preserve` like any other infra failure:
```go
if r := languages.RunnerFor(cfg.Benchmark, cfg.Language); r != nil {
	if serr := r.Setup(ctx, repoDir); serr != nil {
		return preserve(fmt.Errorf("bench/runtime: runner setup: %w", serr))
	}
	outcome, runErr := r.RunTests(ctx, repoDir)
	...
}
```

## Info

### IN-01: Dead no-op branch in `runBench`

**File:** `cmd/helix-bench/main.go:176-179`
**Issue:** `ctx := cmd.Context(); if ctx == nil { ctx = cmd.Context() }` assigns the result of `cmd.Context()` to `ctx`, then if it is nil reassigns the *same* `cmd.Context()` (which returns `context.Background()` and is never nil anyway). The branch can never change anything.
**Fix:** Delete the `if` block; `ctx := cmd.Context()` suffices (cobra's `Context()` never returns nil). If a non-nil guarantee is wanted, fall back to `context.Background()`:
```go
ctx := cmd.Context()
if ctx == nil {
	ctx = context.Background()
}
```

### IN-02: Stale/aspirational package doc — `main.go` header says "Phase 75 ... run (not yet wired)" but `run` is fully wired

**File:** `cmd/helix-bench/main.go:1-13,63-64`
**Issue:** The package doc and the root `Long` help both describe `run` as "(Phase NN, not yet wired)" / "(not yet implemented)", yet `newRunCmd` is a complete implementation that expands the matrix and dispatches cells. The help text shown to operators (`Long`, line 63) will misreport `run` as unimplemented.
**Fix:** Update the package comment and the root `Long` block to reflect that `run` is now implemented (Phase 78); keep the "not yet implemented" wording only for `fetch-datasets` and `report`.

### IN-03: `StartClaude` doc in `subprocess/daemon.go` claims it is "intentionally unimplemented in Plan 01", but it is implemented in `claude.go`

**File:** `bench/runtime/subprocess/daemon.go:51-61`
**Issue:** The long comment block reserves `StartClaude` as an unimplemented stub ("It is intentionally unimplemented in Plan 01 ... No exported signature is committed yet"). In reality `StartClaude` is fully defined in the sibling `bench/runtime/subprocess/claude.go` (confirmed: `func StartClaude(ctx, sb, cfg ClaudeConfig) (*agent.Result, error)`), and `cell.go` calls it. The reservation comment is now stale and contradicts the shipped code, which will confuse the next maintainer about where the claude spawn lives.
**Fix:** Delete or rewrite the stub comment in `daemon.go` to point at the real implementation in `claude.go`.

### IN-04: `parseTest2JSON` silently swallows malformed mid-stream lines

**File:** `bench/languages/go/runner.go:114-141`
**Issue:** On any JSON decode error (other than EOF) `parseTest2JSON` `break`s out of the loop, discarding all subsequent events. The comment justifies this for a leading build-error banner, and empirically (Go 1.26) `go test -json` now emits build failures as well-formed `build-output`/`build-fail` JSON, so the early-break path is not hit for compile failures. But a single corrupt line anywhere in a large stream silently truncates the per-test detail (the `Tests` slice) with no diagnostic. Because `Passed` is gated on exit code this never produces a wrong pass/fail, so it is Info, not a bug — but the silent truncation could hide real test results in a partially-corrupt stream.
**Fix:** On a non-EOF decode error, `continue` past the offending line (re-syncing the decoder by reading line-by-line) rather than abandoning the rest of the stream, or at minimum record that parsing was truncated so the advisory `Tests` detail is not silently incomplete.

### IN-05: `validateCellKey` / `validateMatrixID` are duplicated verbatim

**File:** `bench/runtime/cell.go:145-153`, `bench/runtime/matrix.go:86-94`
**Issue:** The two validators are byte-for-byte identical in body (empty check, `filepath.Clean` mismatch, separator/leading-dot rejection) and only differ in the wrapping package/error prose. The matrix.go comment even notes RunCell "re-validates ... defensively." Two copies of a security-relevant predicate risk drifting apart (a future hardening applied to one and not the other silently weakens the cell-layer check).
**Fix:** Extract a single unexported helper (e.g. in a small shared file in package `runtime`) and have both call sites delegate to it, preserving the distinct error-prefix wrapping at the call site.

---

_Reviewed: 2026-06-17_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
