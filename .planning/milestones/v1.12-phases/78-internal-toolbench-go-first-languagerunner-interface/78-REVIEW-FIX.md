---
phase: 78-internal-toolbench-go-first-languagerunner-interface
fixed_at: 2026-06-17T18:00:00Z
review_path: .planning/phases/78-internal-toolbench-go-first-languagerunner-interface/78-REVIEW.md
iteration: 2
findings_in_scope: 8
fixed: 8
skipped: 0
status: all_fixed
---

# Phase 78: Code Review Fix Report (iteration 2)

**Fixed at:** 2026-06-17
**Source review:** `.planning/phases/78-internal-toolbench-go-first-languagerunner-interface/78-REVIEW.md`
**Iteration:** 2

**Summary:**
- Findings in scope: 8 (4 Warning + 4 Info; `fix_scope: all`)
- Fixed: 8
- Skipped: 0

All eight iteration-2 findings were fixed. The two shared-infra
process-lifecycle warnings (WR-01/WR-02 in `internal/eval/sandbox/sandbox.go`)
were applied ONLY after confirming the entire `go test ./internal/eval/...`
suite stays green — they did not regress the broader eval harness, so the
conditional skip path in the fixer guidance was not needed.

**Green-tree gate (all pass after fixes):**
- `go vet ./...` — clean
- `go build -o helix ./cmd/helix` — ok
- `HELIX_BIN=$(pwd)/helix go test ./bench/... ./internal/eval/...` — all ok
- `make bench-quick` — 1/1 cells succeeded
- full corpus `helix-bench run --benchmarks=internal-toolbench --languages=go --agent=scripted` — **10/10 cells succeeded**
- `gofmt` — all changed files formatted

## Fixed Issues

### WR-01: Happy-path daemon kill reaps only the group leader — children leak under `--parallel`

**Files modified:** `internal/eval/sandbox/sandbox.go`
**Commit:** 833ef97a
**Applied fix:** `Kill()` now SIGKILLs the whole process group
(`syscall.Kill(-pid, SIGKILL)`) on the normal path — not just the group
leader — so language-server / `go test` descendants spawned by the daemon are
reaped on the common happy path, not only in the 5s-timeout fallback. The
group kill is best-effort (ESRCH once the group is gone).
**Verification:** Tier 1 (re-read) + Tier 2 (`go vet ./internal/eval/sandbox/`
clean) + full `go test ./internal/eval/...` green (sandbox, agent, runner,
judge, score, report, trace all ok). This is shared eval infrastructure, so
the whole-suite green confirmation was the gating condition before commit.
**Requires human verification:** process-lifecycle/concurrency change to
shared infra — recommend a maintainer eyeball the group-kill ordering.

### WR-02: 5s-timeout Kill fallback returns without reaping the in-flight `Wait()`

**Files modified:** `internal/eval/sandbox/sandbox.go`
**Commit:** 833ef97a (committed together with WR-01 — same `Kill()` rewrite)
**Applied fix:** The 5s-timeout fallback now drains the already-running
`Wait()` goroutine (`<-done`) after re-sending the group SIGKILL, so the
process is confirmed reaped before the function returns the infra error. The
`done` channel is buffered (cap 1) so the goroutine's send never blocks.
**Verification:** Same gate as WR-01 — full `go test ./internal/eval/...`
green. No test hangs introduced (sandbox tests complete in 0.530s).
**Requires human verification:** same shared-infra caveat as WR-01.

### WR-03: `ccLegPresent` false-branch is never asserted by any test

**Files modified:** `bench/runtime/cctap_test.go`
**Commit:** 03aae4fe
**Applied fix:** Added `TestCCLegPresentBothBranches`, a direct unit test that
builds a real `trace.MergedTrace` via `trace.Merge`/`SynthCCTap` and calls
`ccLegPresent` (the predicate itself, not a re-implemented event count) for
both branches: `SynthCCTap(nil)` → `false`, one scripted step → `true`. A
regression to the vacuous "any `Source==cc`" form now fails this test instead
of passing the whole suite silently.
**Verification:** Tier 1 + Tier 2 (`go vet ./bench/runtime/` clean) + test
passes; full `bench/runtime` suite green.

### WR-04: `Coverage` can report `Covered > Declared` with a duplicate declared capability

**Files modified:** `bench/languages/coverage.go`, `bench/languages/coverage_test.go`
**Commit:** 6255bf0a
**Applied fix:** `Coverage` now counts `coveredN`/`missing` by iterating the
deduplicated `declaredSet` map rather than the raw `declared` slice, so a
repeated capability can no longer over-count `Covered` past `Declared`. Added
`TestCoverageDeduplicatesDeclared` asserting `Covered <= Declared` (and
`Declared == Covered == 1`) for a doubly-declared capability.
**Verification:** Tier 1 + Tier 2 + `TestCoverageGoIsTenOfTen` /
`TestCoverageDetectsGap` / the new test all pass; the live 10/10 Go gate is
unchanged.

### IN-01: Redundant `os.Chmod` on a freshly `CreateTemp`'d file

**Files modified:** `bench/runtime/cell.go`
**Commit:** 6f1350e6
**Applied fix:** Removed the no-op `os.Chmod(tmpName, 0600)` block in
`writeDurable` (and its error path); `os.CreateTemp` already creates the file
0600. Left a one-line comment documenting the reliance on the CreateTemp
default.
**Verification:** Tier 1 + Tier 2 (`go vet` clean) + `bench/runtime` suite
green.

### IN-02: `discoverTasks` union-across-languages fabricates guaranteed-to-fail cells

**Files modified:** `cmd/helix-bench/main.go`
**Commit:** e174c2fc
**Applied fix:** Documented the cross-product fan-out directly in the
`--languages` flag help: with `--tasks` omitted and multiple languages, the
task set is the union across languages, so a task present only under one
language yields failing cells under the others; pin `--tasks` to avoid the
fan-out. (Chose the documentation option over per-language discovery, matching
the review's "or document" guidance and deferring the per-language axis to
Phase 85 where it is actually exercised.)
**Verification:** Tier 1 + Tier 2 (`go vet ./cmd/helix-bench/` clean) +
`cmd/helix-bench` tests green.

### IN-03: `deriveStoreOptIn` / `readTaskPrompt` parse the same `task.json` twice per cell

**Files modified:** `bench/runtime/matrix.go`
**Commit:** fce7dfa1
**Applied fix:** Unified the prompt + store-opt-in decode into a single
`taskMeta` struct read once by `readTaskMeta(seedDir)` in `runOneCell`,
replacing the two separate file reads (`readTaskPrompt` + `deriveStoreOptIn`).
`taskMeta.storeOptIn()` carries the opt-in rule; `deriveStoreOptIn` is kept as
a thin wrapper so the existing `store_isolation_test.go` standalone-predicate
test continues to compile and pass unchanged.
**Verification:** Tier 1 + Tier 2 (`go vet` clean) + full `bench/runtime`
suite green (including `store_isolation_test.go`).

### IN-04: `validatePathSegment` doc comment mis-attributes which clause catches a single separator

**Files modified:** `bench/runtime/validate.go`
**Commit:** 3c219817
**Applied fix:** Reworded the doc comment to correctly attribute each
rejection to the clause that performs it: `filepath.Clean` rewrites (`..`,
`a//b`, trailing slash), the explicit `ContainsAny` separator clause (which
catches a single embedded `a/b` that Clean leaves unchanged), and the leading
dot. Behavior is unchanged (doc-only nit).
**Verification:** Tier 1 + Tier 2 (`go vet` clean). Doc-only change; existing
`matrix_test.go` separator cases continue to pass.

## Skipped Issues

None — all 8 in-scope findings were fixed.

---

_Fixed: 2026-06-17_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
