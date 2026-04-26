---
phase: 53-obs-metrics-gaps
fixed_at: 2026-04-26T00:00:00Z
review_path: .planning/phases/53-obs-metrics-gaps/53-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 53: Code Review Fix Report

**Fixed at:** 2026-04-26
**Source review:** .planning/phases/53-obs-metrics-gaps/53-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (Critical=0, Warning=5; Info findings deferred per fix_scope=critical_warning)
- Fixed: 5
- Skipped: 0

All five Warning findings were addressed. Each fix was committed atomically
with a `fix(53): {ID} ...` message. Project-wide `go vet ./...` is clean
(only the pre-existing TOKEN_COUNT macro-redefinition warning from the
vendored Swift tree-sitter scanner). Project-wide `go test ./...` is green
across every package; the only failing tests (`TestSymbols_JavaFixture`,
`TestEdit_JavaFixture`) are pre-existing Java/JDT-LS integration failures
that reproduce on the base commit `bbd5869b` and are unrelated to the
Phase 53 metric surface touched by these fixes.

## Fixed Issues

### WR-01: `replace_in_file` emits `outcome=success` on zero-replacement no-ops

**Files modified:** `internal/kernel/fileops/tools.go`
**Commit:** 4542cf44
**Applied fix:** Added a `count == 0 && args.IsRegex` post-fuzzy branch that
sets `outcomeErr = serr.New(serr.InvalidArgs, "no matches for pattern")`
before returning the "0 replacement(s) made" text, and updated the
`readErr != nil` branch in the literal/fuzzy fallback path to set
`outcomeErr` with the read failure context. The deferred
`EditOutcomeInc` now records `OutcomeFailed` for both no-op shapes,
preserving the success-rate signal in `serena_edit_outcome_total`.

### WR-02: `kernel.ActivateWorkspace` idempotency invariant is undocumented and untested

**Files modified:** `internal/kernel/kernel.go`, `internal/kernel/kernel_test.go`
**Commit:** 084b5d6b
**Applied fix:** Added an inline comment block at the early-return cache
hit in `ActivateWorkspace` that documents the WR-02 invariant ("activate
emitted exactly once per workspace per daemon lifetime") and references
the new regression test. Created `internal/kernel/kernel_test.go` with
`TestActivateWorkspace_EmitsActivateOnce` that calls `ActivateWorkspace`
three times on the same root and asserts exactly one
`SessionLifecycleInc(_, "activate")` emission via a recording sink.

### WR-03: `Pool.AcquireLease` capacity-exhaustion path emits `result=miss, scope=clean`

**Files modified:** `internal/kernel/lspool/pool.go`, `internal/kernel/lspool/metrics_test.go`
**Commit:** 22b00739
**Applied fix:** Removed the `LSPoolCacheInc` emission from the
`len(p.workers) >= p.config.MaxWorkers` branch in `AcquireLease`,
matching the "PREFER skipping over mislabeling" pattern used elsewhere
in Phase 53 (e.g. `DeactivateWorkspace`). Replaced the two old
sub-tests (`miss/clean max workers reached`, `miss/dirty max workers
reached`) in `metrics_test.go` with a single
`max workers reached skips emission (WR-03)` test that asserts the
recording sink's cache slice is empty for both `dirty=false` and
`dirty=true` callers when MaxWorkers=0.

### WR-04: TagCache `os.Stat` failure path skips both miss-emission and extract-observation

**Files modified:** `internal/repomap/cache.go`, `internal/repomap/cache_test.go`
**Commit:** c58f0013
**Applied fix:** Hoisted `lang := LangFromExt(filePath)` above the
`os.Stat` call (it doesn't need the file to exist) and emit a
`RepoMapExtractObserve(lang, 0)` + `RepoMapCacheInc(lang, ResultMiss)`
pair before returning the wrapped stat error. Mirrors the existing
"errors still emit miss" contract for the `extractFn`-failure branch.
Added `TestTagCache_MetricsEmission_MissOnStatError` covering a missing
file path, asserting hits=0, misses=1, observed=1, and language label
"go" resolved from extension.

### WR-05: `lspoolSessionTimeoutAdapter` silently drops emissions when `m == nil`

**Files modified:** `internal/daemon/daemon.go`
**Commit:** d725c33e
**Applied fix:** Removed the `if a.m == nil { return }` guard from
`SessionTimeout` and replaced the old comment with a WR-05 reference
explaining that the wiring contract guarantees a non-nil `*obs.Metrics`,
so a nil here is a wiring defect that should fail loudly rather than
silently swallow emissions. Production wiring at `newDaemon` step 5/14
already passes a real `metrics`; no test changes required.

---

_Fixed: 2026-04-26_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
