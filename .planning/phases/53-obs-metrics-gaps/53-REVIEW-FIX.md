---
phase: 53-obs-metrics-gaps
fixed_at: 2026-04-26T00:00:00Z
review_path: .planning/phases/53-obs-metrics-gaps/53-REVIEW.md
iteration: 2
findings_in_scope: 11
fixed: 10
skipped: 1
status: partial
---

# Phase 53: Code Review Fix Report

**Fixed at:** 2026-04-26
**Source review:** .planning/phases/53-obs-metrics-gaps/53-REVIEW.md
**Iterations:** 1 (Warnings) + 2 (Info)

**Summary:**
- Findings in scope: 11 (Critical=0, Warning=5, Info=6)
- Fixed: 10 (5 Warnings in iteration 1 + 5 Info in iteration 2)
- Skipped: 1 (IN-06 — explicitly tagged "out of scope for Phase 53" by reviewer)

Iteration 1 closed all five Warnings. Iteration 2 reviewed every Info
finding and applied a targeted fix for IN-01 through IN-05; IN-06 is
documented in `deferred-items.md` rather than fixed (per the reviewer's
own out-of-scope tag).

**Verification gates:**
- `go vet ./...` clean (only the pre-existing TOKEN_COUNT macro warning
  from the vendored Swift tree-sitter scanner, unchanged from baseline).
- `go test ./internal/...` green across every package.
- `go test ./test/...` green except the pre-existing
  `TestSymbols_JavaFixture` and `TestEdit_JavaFixture` JDT-LS integration
  failures already documented in `deferred-items.md`; reproduce on the
  base commit and are unrelated to Phase 53.

## Fixed Issues

### Iteration 1 (Warnings)

#### WR-01: `replace_in_file` emits `outcome=success` on zero-replacement no-ops

**Files modified:** `internal/kernel/fileops/tools.go`
**Commit:** 4542cf44
**Applied fix:** Added a `count == 0 && args.IsRegex` post-fuzzy branch
that sets `outcomeErr = serr.New(serr.InvalidArgs, "no matches for
pattern")` before returning the "0 replacement(s) made" text, and
updated the `readErr != nil` branch in the literal/fuzzy fallback path
to set `outcomeErr` with the read failure context. The deferred
`EditOutcomeInc` now records `OutcomeFailed` for both no-op shapes,
preserving the success-rate signal in `serena_edit_outcome_total`.

#### WR-02: `kernel.ActivateWorkspace` idempotency invariant is undocumented and untested

**Files modified:** `internal/kernel/kernel.go`, `internal/kernel/kernel_test.go`
**Commit:** 084b5d6b
**Applied fix:** Added an inline comment block at the early-return cache
hit in `ActivateWorkspace` documenting the WR-02 invariant ("activate
emitted exactly once per workspace per daemon lifetime") and referencing
the new regression test. Created `internal/kernel/kernel_test.go` with
`TestActivateWorkspace_EmitsActivateOnce` that calls `ActivateWorkspace`
three times on the same root and asserts exactly one
`SessionLifecycleInc(_, "activate")` emission via a recording sink.

#### WR-03: `Pool.AcquireLease` capacity-exhaustion path emits `result=miss, scope=clean`

**Files modified:** `internal/kernel/lspool/pool.go`, `internal/kernel/lspool/metrics_test.go`
**Commit:** 22b00739
**Applied fix:** Removed the `LSPoolCacheInc` emission from the
`len(p.workers) >= p.config.MaxWorkers` branch in `AcquireLease`,
matching the "PREFER skipping over mislabeling" pattern used elsewhere
in Phase 53 (e.g. `DeactivateWorkspace`). Replaced the two old sub-tests
(`miss/clean max workers reached`, `miss/dirty max workers reached`) in
`metrics_test.go` with a single `max workers reached skips emission
(WR-03)` test that asserts the recording sink's cache slice is empty for
both `dirty=false` and `dirty=true` callers when MaxWorkers=0.

#### WR-04: TagCache `os.Stat` failure path skips both miss-emission and extract-observation

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

#### WR-05: `lspoolSessionTimeoutAdapter` silently drops emissions when `m == nil`

**Files modified:** `internal/daemon/daemon.go`
**Commit:** d725c33e
**Applied fix:** Removed the `if a.m == nil { return }` guard from
`SessionTimeout` and replaced the old comment with a WR-05 reference
explaining that the wiring contract guarantees a non-nil `*obs.Metrics`,
so a nil here is a wiring defect that should fail loudly rather than
silently swallow emissions. Production wiring at `newDaemon` step 5/14
already passes a real `metrics`; no test changes required.

### Iteration 2 (Info)

#### IN-01: USAGE.md "session timeout" semantics are mislabelled

**Files modified:** `USAGE.md`,
`.planning/phases/53-obs-metrics-gaps/deferred-items.md`
**Commit:** b85551b1
**Applied fix:** Strengthened the warning in the
`serena_session_lifecycle_total` doc paragraph for `phase="timeout"`
from a soft NOTE to a bold WARNING that explicitly tells operators NOT
to alert on it as a user-session-timeout signal, and points at the
deferred rename to `phase="worker_idle_evicted"` planned for Phase 54+.
Logged the rename follow-up in `deferred-items.md` alongside the IN-06
pointer-receiver-discipline note (which the reviewer explicitly tagged
out-of-scope for Phase 53).

#### IN-02: NoopSink alloc-tests not pinned at every package boundary

**Files modified:** `internal/repomap/metrics_test.go` (new),
`internal/kernel/session_metrics_test.go` (new)
**Commit:** 51ad6362
**Applied fix:** Created `TestNoopSink_ZeroAlloc` in
`internal/repomap/metrics_test.go` exercising `RepoMapCacheInc` (hit +
miss) and `RepoMapExtractObserve`, and `TestNoopSessionSink_ZeroAlloc`
in `internal/kernel/session_metrics_test.go` exercising all four
`Phase*` enum values. Both run under `testing.AllocsPerRun(100, ...)`
and fail if the noop path ever allocates. Modeled on
`internal/kernel/edit/metrics_test.go:42-50`. Phase 53 D-15 invariant
("noop default must not allocate") is now pinned at every consumer-side
package boundary (lspool, edit, repomap, kernel session).

#### IN-03: `EditOutcomeInc` tool allowlist drift between obs.Metrics and edit.AllowedTools

**Files modified:** `internal/daemon/wiring_test.go`
**Commit:** 98230383
**Applied fix:** Added `TestEditOutcomeAllowlist_DriftGuard` in
`internal/daemon/wiring_test.go` that iterates every entry in
`edit.AllowedTools`, feeds it through a registered `*obs.Metrics` via
`EditOutcomeInc(tool, OutcomeSuccess)`, and asserts the labelled series
incremented exactly once. If a tool is in `edit.AllowedTools` but NOT in
the inline switch in `(*obs.Metrics).EditOutcomeInc`, the `ToFloat64`
read returns 0 and the test fails with a remediation message naming the
two files to align. Closes the silent-drift gap acknowledged in the
edit/metrics.go:42-44 comment.

#### IN-04: Restart-emission depends on temporal ordering of cb.RecordSuccess

**Files modified:** `internal/kernel/lspool/pool.go`
**Commit:** 26651d3e
**Applied fix:** Moved the `priorFailures := cb.Failures()` snapshot and
the `LSPoolRestart` emission out of `spawnWorkerLocked` and into the two
callers (`AcquireLease`, `PromoteToDirty`). The previous design relied
on `cb.RecordSuccess()` being called by the caller AFTER
`spawnWorkerLocked` returned, so the helper could read `Failures()` in
the brief window before the success counter reset. A future refactor
that inlined `RecordSuccess` inside `spawnWorkerLocked` (e.g. for
symmetry with `RecordFailure`) would silently break the restart
emission. The caller-driven snapshot decouples the metric from temporal
ordering. **Logic-bug verification:** `go vet` clean and the full
`internal/kernel/lspool` test suite passes (including
`TestPool_CacheMetricsEmission` and `TestCircuit_stateReport`); requires
human verification that the new caller-side emission still fires in
production when expected.

#### IN-05: Activate/Deactivate empty-label policy asymmetry

**Files modified:** `internal/daemon/daemon.go`,
`internal/daemon/telemetry_metrics_test.go`,
`internal/kernel/kernel.go`
**Commit:** f50d1e63
**Applied fix:** Aligned `DeactivateWorkspace`'s emission policy with
`ActivateWorkspace` and the shutdown sweep — when the kernel TRACKS the
workspace (new `Kernel.HasWorkspace` helper) but `LanguagesForRoot`
returns empty, emit one `SessionLifecycleInc("", PhaseDeactivate)`
instead of skipping. The handler still skips emission entirely when the
kernel has no record of the workspace path at all, since that is the
"already torn down / unknown" signal rather than a "session ended"
event. Added `Kernel.HasWorkspace` to disambiguate the two cases (the
previous `LanguagesForRoot` nil-return conflated them). Updated the
`TestSessionLifecycleMetrics` comment to document the new behaviour;
the test's belt-and-braces explicit emit is preserved so the
phase=deactivate enum check still observes the expected reach for a
workspace with at least one detected language. **Logic-bug
verification:** `go vet` clean and `internal/daemon` + `internal/kernel`
tests pass; requires human verification that the new empty-label
deactivate emit aligns with operator expectations for PromQL `sum
by(language)` balance.

## Skipped Issues

### IN-06: `WorkerLease`/`Worker` adapter type-assertion in `defaultOverriderResolver`

**File:** `internal/kernel/edit/rename.go:42-57`
**Reason:** Reviewer explicitly tagged this finding "Out of scope for
Phase 53 to fix" — the rename code pre-dates Phase 53 and the failure
shape (a future adapter implementing `RenameOverrider` only on a pointer
receiver while `lease.Adapter()` returns a value) is hypothetical until
a new adapter is introduced. Captured the discipline guidance in
`deferred-items.md` (commit b85551b1) so future adapter authors add
`var _ RenameOverrider = (*MyAdapter)(nil)` compile-time assertions and
prefer pointer-receiver implementation when the adapter ships.
**Original issue:** "When new adapters are added in future phases,
prefer pointer-receiver implementation of `RenameOverrider` and explicit
`var _ RenameOverrider = (*MyAdapter)(nil)` compile-time assertions. Out
of scope for Phase 53 to fix."

---

_Fixed: 2026-04-26_
_Fixer: Claude (gsd-code-fixer)_
_Iterations: 1 (Warnings) + 2 (Info)_
