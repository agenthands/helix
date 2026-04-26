---
phase: 53-obs-metrics-gaps
reviewed: 2026-04-26T00:00:00Z
depth: standard
files_reviewed: 27
files_reviewed_list:
  - internal/daemon/daemon.go
  - internal/daemon/shutdown.go
  - internal/daemon/telemetry_metrics_test.go
  - internal/daemon/wiring_test.go
  - internal/fuzzy/match.go
  - internal/kernel/edit/metrics_test.go
  - internal/kernel/edit/metrics.go
  - internal/kernel/edit/tools.go
  - internal/kernel/edit/replace.go
  - internal/kernel/edit/insert.go
  - internal/kernel/edit/rename.go
  - internal/kernel/edit/delete.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/kernel.go
  - internal/kernel/lspool/metrics_test.go
  - internal/kernel/lspool/metrics.go
  - internal/kernel/lspool/pool.go
  - internal/kernel/session_metrics.go
  - internal/obs/metrics_alloc_test.go
  - internal/obs/metrics_cardinality_test.go
  - internal/obs/metrics_labels_test.go
  - internal/obs/metrics_test.go
  - internal/obs/metrics.go
  - internal/repomap/cache_test.go
  - internal/repomap/cache.go
  - internal/repomap/metrics.go
  - internal/skill/repomap/skill.go
  - USAGE.md
findings:
  critical: 0
  warning: 5
  info: 6
  total: 11
status: issues_found
---

# Phase 53: Code Review Report

**Reviewed:** 2026-04-26
**Depth:** standard
**Files Reviewed:** 27 (28 incl. USAGE.md)
**Status:** issues_found

## Summary

Phase 53 adds five metric families (`serena_lspool_cache_total`, `serena_repomap_cache_total`, `serena_repomap_extract_duration_seconds`, `serena_session_lifecycle_total`, `serena_edit_outcome_total`), per-package `MetricsSink` interfaces (D-08 import-cycle invariant honoured), closed-enum guards at the helper layer, allowlist + cardinality + alloc tests, daemon wiring (including the lspool→adapter→obs forwarding for the timeout phase), and USAGE.md documentation.

The plumbing is solid: compile-time `var _ Sink = (*obs.Metrics)(nil)` assertions in `wiring_test.go` pin the four consumer interfaces against the helper signatures; closed-enum guards drop unknown values silently to keep cardinality bounded; `NoopSink` defaults are honoured everywhere (`SetMetrics`/constructors all coerce nil→noop).

No BLOCKERs found. The findings below are correctness-bordering smells around outcome classification on no-op tool returns, idempotency of activation emission across the lazy-init / `setActivateCallback` paths, and a couple of documentation/test gaps. The shutdown emission ordering (snapshot ActiveLanguages → emit shutdown → kernel.Shutdown clears the map) is correct and worth preserving — the inline comment already documents the ordering invariant.

## Warnings

### WR-01: `replace_in_file` emits `outcome=success` on zero-replacement no-ops

**File:** `internal/kernel/fileops/tools.go:407-431`
**Issue:** The handler's `defer` calls `sink.EditOutcomeInc("replace_in_file", edit.ClassifyOutcome(strategy, outcomeErr))`. On the path where the literal pattern returned `count == 0`, `args.IsRegex == true` (no fuzzy fallback runs) the function falls through to line 431 returning `"0 replacement(s) made in <path>"` with `strategy=""` and `outcomeErr=nil`. `ClassifyOutcome("", nil)` returns `OutcomeSuccess`. Same shape on line 408-410 when literal returns 0 hits, fuzzy fallback also can't read the file (`readErr != nil`), and we silently fall through to `"0 replacement(s) made"` — also classified as success. A no-op edit is not a success; it dilutes the success-rate signal that `serena_edit_outcome_total{outcome="success"}` is meant to convey, and obscures regressions where regex patterns silently fail to match.
**Fix:** Treat 0-replacement returns as `OutcomeFailed` (or introduce an explicit `OutcomeNoMatch` if the closed enum is allowed to grow — but D-07 froze the four-value enum, so reuse `OutcomeFailed`). Suggested patch around line 401-431:
```go
count, err := ReplaceInFile(root, args.Path, args.Pattern, args.Replacement, args.IsRegex)
if err != nil {
    outcomeErr = err
    return errorResult(err.Error()), nil, nil
}

if count == 0 && !args.IsRegex {
    // ... existing fuzzy fallback ...
}

if count == 0 {
    // Regex path with 0 hits, or literal+fuzzy that fell through silently.
    outcomeErr = serr.New(serr.InvalidArgs, "no matches for pattern")
    return textResult(fmt.Sprintf("0 replacement(s) made in %s", args.Path)), nil, nil
}
```
The same pattern should be reviewed for the `readErr != nil` branch (line 408-410): currently it returns success-classified text even though the fuzzy fallback failed to even read the file.

### WR-02: `kernel.ActivateWorkspace` emits `phase=activate` exactly once for the lifetime of a workspace, but lazy-init + `SetActivateCallback` both invoke it — silent duplicate paths in the call graph

**File:** `internal/kernel/kernel.go:75-119`, `internal/daemon/daemon.go:376-421`
**Issue:** `Kernel.ActivateWorkspace` is idempotent (returns early on hash hit at line 86-88 without re-emitting), so the duplicate-emission risk is mitigated *today*. However, both `lazyActivateFn` (daemon.go:376) and `mcpServer.SetActivateCallback` (daemon.go:400) call `k.ActivateWorkspace(ctx, repoPath)`. Reviewers (and future maintainers refactoring `ActivateWorkspace` to re-detect on cache miss/different signatures) are likely to break the "one activate emission per workspace per daemon lifetime" invariant. The kernel's emission decision is also coupled to `len(langs) == 0 → emit empty-string label` which is documented in USAGE.md as a desired signal but means ANY future caller adding an extra `ActivateWorkspace` invocation can silently double-count if the early-return path changes.
**Fix:** Add a regression test in `internal/kernel/kernel_test.go` (or `daemon/wiring_test.go`) that calls `ActivateWorkspace` twice on the same root and asserts `serena_session_lifecycle_total{phase="activate"}` increments exactly once across both calls. This locks the idempotency invariant against refactors. Additionally consider moving the emission to *after* the early-return branch with an explicit comment "// idempotency: emit only on first activation".

### WR-03: `Pool.AcquireLease` `MaxWorkers=0` configuration emits `result=miss, scope=clean` on EVERY call — distorts cache hit-rate when pool is intentionally disabled

**File:** `internal/kernel/lspool/pool.go:150-160`
**Issue:** When `cfg.MaxWorkers == 0` (used in tests, but valid in any config that wants to disable pooling), every `AcquireLease` call goes through `len(p.workers) >= p.config.MaxWorkers` and emits `LSPoolCacheInc(lang, miss, clean|dirty)` before returning `ErrMaxWorkersReached`. Operators reading the metric will see a 0% hit rate that has nothing to do with cache effectiveness — it reflects pool exhaustion. The semantics in USAGE.md (line 697) describe `scope=clean` as "a clean (non-dirty) acquire" without any nuance for "couldn't even attempt due to capacity."
**Fix:** Either (a) introduce a 4th scope value `exhausted` or `capacity` and update the closed-enum guard in `obs.Metrics.LSPoolCacheInc`, the carve-out in `metrics_labels_test.go`, the cardinality cap in `metrics_cardinality_test.go`, AND USAGE.md; or (b) skip emission on the capacity path (matching the `DeactivateWorkspace` "PREFER skipping" pattern used in daemon.go:716) so capacity exhaustion doesn't pollute hit-rate. Option (b) is lower-risk and consistent with the rest of Phase 53's "skip rather than mislabel" stance:
```go
if len(p.workers) >= p.config.MaxWorkers {
    // Capacity exhaustion is orthogonal to cache effectiveness; skip emission.
    return nil, ErrMaxWorkersReached
}
```

### WR-04: TagCache `os.Stat` failure path skips both miss-emission and extract-observation — silently invisible failures

**File:** `internal/repomap/cache.go:75-80`
**Issue:** When `os.Stat(filePath)` fails (e.g. file deleted between walk and extract, permission error), `GetOrExtract` returns immediately at line 78-79 without any metric emission. The cache.go code comment on line 116-118 explicitly states the design choice: "Emit miss whether or not extractFn errored — operators want the miss-rate to include failed extractions." Stat failures violate that same principle: an operator watching `serena_repomap_cache_total` will never see this class of failure. This is doubly noteworthy because `TestTagCache_MetricsEmission_MissOnExtractError` (cache_test.go:262-280) exists specifically to lock the "errors still emit miss" contract, but only covers extractFn errors, not stat errors.
**Fix:** Resolve `lang := LangFromExt(filePath)` before the `os.Stat` call (it doesn't need the file to exist) and emit a miss + extract-observation on stat failure:
```go
func (c *TagCache) GetOrExtract(filePath string, extractFn func() ([]Tag, error)) ([]Tag, error) {
    lang := LangFromExt(filePath)
    info, err := os.Stat(filePath)
    if err != nil {
        c.mu.Lock()
        sink := c.metrics
        c.mu.Unlock()
        sink.RepoMapCacheInc(lang, ResultMiss)
        return nil, fmt.Errorf("stat %s: %w", filePath, err)
    }
    mtime := info.ModTime().UnixNano()
    // ... rest unchanged ...
}
```
Add a corresponding test mirroring `TestTagCache_MetricsEmission_MissOnExtractError`.

### WR-05: `lspoolSessionTimeoutAdapter` silently drops emissions when `m == nil` — production-only dead code, but the nil-guard hides wiring bugs

**File:** `internal/daemon/daemon.go:124-134`
**Issue:** `SessionTimeout` early-returns on `a.m == nil`. `daemonSessionProvider` and the wiring at line 217 always pass a non-nil `metrics` in production, but in synthetic tests (or any future code that constructs the adapter directly) a nil `m` will silently swallow the emission with no log, no panic, no test failure. The pattern is inconsistent with the rest of the file: `obs.Metrics`'s helpers (e.g. `EditOutcomeInc`) drop unknown enum values silently because cardinality must be bounded — but here the nil drop is masking a wiring bug, not a cardinality concern.
**Fix:** Either remove the nil-guard (let it nil-deref so the test fails loudly) or add a one-time `slog.Warn` so misuse surfaces:
```go
func (a lspoolSessionTimeoutAdapter) SessionTimeout(lang string) {
    if a.m == nil {
        // Dev-time guard: this adapter must always be constructed with a real
        // *obs.Metrics. A nil here is a wiring bug, not a runtime condition.
        return
    }
    a.m.SessionLifecycleInc(lang, kernel.PhaseTimeout)
}
```
Better: drop the guard entirely and rely on the wiring contract.

## Info

### IN-01: USAGE.md "session timeout" semantics are mislabelled — `phase="timeout"` measures *worker idle eviction*, not user session timeout

**File:** `USAGE.md:738-739`
**Issue:** The doc itself acknowledges the mislabel ("`timeout` means a warm worker was idle-evicted by `pool.checkTTLs`. NOTE: this is *worker-level* idle eviction, not a user-session timeout"). Acknowledging a misleading metric name in prose does not undo the metric name. Operators wiring alerts on `phase="timeout"` rate spikes will reasonably assume "user sessions are timing out" rather than "warm LSP workers are idling out at the BaseTTL". This is a naming bug that will compound over time.
**Fix:** Phase 54+: rename to `phase="worker_idle_evicted"` (closed enum extension, but the producer is a single helper so churn is small). For Phase 53, prepend a stronger warning to the doc paragraph (e.g. "WARNING: do NOT alert on this as a user-session timeout signal") and file a follow-up in `deferred-items.md`.

### IN-02: `metrics_alloc_test.go` only proves zero-alloc for `lspool.NoopSink`; new edit/repomap/kernel session NoopSinks are alloc-tested in their own packages but not pinned at the boundary

**File:** `internal/obs/metrics_alloc_test.go:16-26`
**Issue:** The comment block (lines 12-15) explicitly notes: "Other package NoopSinks (repomap.NoopSink, edit.NoopSink, kernel.NoopSessionSink) land in plan 53-02 and get their own alloc-tests in those packages — this plan scopes the assertion to lspool.NoopSink only". `internal/kernel/edit/metrics_test.go:42-50` covers `edit.NoopSink`. The repomap and kernel session NoopSinks have NO alloc test in this submission. D-15 invariant ("noop default must not allocate") is unproven for them.
**Fix:** Add `TestNoopSink_ZeroAlloc`-style tests in `internal/repomap/metrics_test.go` (new file) and `internal/kernel/session_metrics_test.go` (new file). Modeled on `internal/kernel/edit/metrics_test.go:42-50`.

### IN-03: `EditOutcomeInc` tool allowlist is duplicated in two places — silent drift risk

**File:** `internal/obs/metrics.go:284-298`, `internal/kernel/edit/metrics.go:46-55`
**Issue:** `obs.Metrics.EditOutcomeInc` carries an inline 8-tool switch case (line 285-288). `edit.AllowedTools` (line 46-55) carries the same 8 entries as a `map[string]bool`. The comment on edit/metrics.go:42-44 acknowledges the coupling: "This MUST stay in lockstep with the inline allowlist in obs.Metrics.EditOutcomeInc — drift is silent". Drift would manifest as: operator adds a new edit tool, registers it in `edit.AllowedTools`, instruments the handler, but forgets the obs.Metrics switch — emissions silently drop. No CI lint catches this today.
**Fix:** Add a CI-time test in `internal/daemon/wiring_test.go` (or a dedicated `metrics_alloc_drift_test.go`) that iterates `edit.AllowedTools` and feeds each tool through `obs.Noop(...).Metrics().EditOutcomeInc(tool, "success")` against a primed registry, asserting each yields one labelled series. If a tool is in `edit.AllowedTools` but not in the obs.Metrics switch, the registry won't gain that series and the test fails.

### IN-04: `pool.go:338-340` "restart-after-failures" emission depends on observable side-effects from RecordSuccess running AFTER the metric is checked

**File:** `internal/kernel/lspool/pool.go:336-340`
**Issue:** The comment is accurate ("RecordSuccess will reset the counter immediately after, so the order matters") but the invariant is fragile: if a future refactor inlines `cb.RecordSuccess()` earlier in `AcquireLease` (e.g. moves it inside `spawnWorkerLocked` for symmetry with `cb.RecordFailure()` at line 165), `Failures()` reads zero and the restart counter never increments. The whole emission depends on temporal ordering at a distance.
**Fix:** Capture the failure count before invoking spawn and use the captured value:
```go
priorFailures := 0
if cb, ok := p.circuits[wsKey.Language]; ok {
    priorFailures = cb.Failures()
}
worker, err := p.spawnWorkerLocked(ctx, wsKey)
if err != nil {
    cb.RecordFailure()
    p.metrics.LSPoolCacheInc(wsKey.Language, ResultMiss, ScopeCrashed)
    return nil, fmt.Errorf("spawning worker: %w", err)
}
cb.RecordSuccess()
if priorFailures > 0 {
    p.metrics.LSPoolRestart(wsKey.Language)
}
```
Removes the temporal dependency.

### IN-05: `kernel.ActivateWorkspace` emits empty-string `language` label for zero-detection workspaces — already documented and tested, but breaks the "PREFER skipping" pattern used in DeactivateWorkspace

**File:** `internal/kernel/kernel.go:108-116` vs `internal/daemon/daemon.go:709-721`
**Issue:** Two emission sites, two opposite policies:
- `ActivateWorkspace` emits with `language=""` when no languages detected (kernel.go:111).
- `DeactivateWorkspace` *skips* the emit when `LanguagesForRoot()` returns empty (daemon.go:716).

Both reference Phase 53 D-04 in their comments; both appear to be intentional design choices. But the asymmetry means: a workspace with no detected languages will register one `phase=activate` increment with `language=""`, then NEVER register a `phase=deactivate` or `phase=shutdown` increment (shutdown.go:29 also iterates `ActiveLanguages()` which returns `[""]` for zero-detection per kernel.go:191-194 — so shutdown emits with `language=""`, but deactivate does not). PromQL `sum by(language) (serena_session_lifecycle_total)` will show unbalanced counts for `language=""`.
**Fix:** Pick one policy. If "PREFER skipping" is the rule, drop the empty-label emit in kernel.go:110-112 (and adjust kernel.go:191-194's `ActiveLanguages` empty-string append). If "always emit, with empty label" is the rule, fix DeactivateWorkspace to emit with `language=""` when LanguagesForRoot returns nothing. Document the chosen policy in CONTEXT.md D-04 unambiguously.

### IN-06: `WorkerLease`/`Worker` adapter type-assertion in `defaultOverriderResolver` uses `any(adapter).(RenameOverrider)` — pointer-vs-value receiver edge case

**File:** `internal/kernel/edit/rename.go:42-57`
**Issue:** Line 50: `if o, ok := any(adapter).(RenameOverrider); ok` checks whether the adapter (whatever `lease.Adapter()` returns) directly satisfies `RenameOverrider`. The fall-through at line 53 special-cases `*lspool.RustAnalyzerAdapter` and wraps it in `RustAnalyzerRenameOverride{Inner: ra}`. If a future adapter type implements `RenameOverrider` only on the pointer receiver but `lease.Adapter()` returns a value, the assertion at line 50 will silently fail and fall through to line 53's `*RustAnalyzerAdapter` check, which won't match either, producing `nil` (= "no override available"). The native error is then propagated verbatim — silently degrading rename to no-fallback for a freshly-added adapter.

This is not a Phase 53 bug per se (the rename code is older), but Phase 53's `EditOutcomeInc("rename_symbol", ...)` will record the failure as `OutcomeFailed`, masking what is really a wiring miss as a generic rename failure in the metrics dashboard.
**Fix:** When new adapters are added in future phases, prefer pointer-receiver implementation of `RenameOverrider` and explicit `var _ RenameOverrider = (*MyAdapter)(nil)` compile-time assertions. Out of scope for Phase 53 to fix.

---

_Reviewed: 2026-04-26_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
