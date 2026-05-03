---
phase: 47-bug-rust-analyzer-rename
reviewed: 2026-04-24T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/kernel/edit/rename.go
  - internal/kernel/edit/rename_override.go
  - internal/kernel/edit/rename_override_test.go
  - internal/kernel/edit/tools.go
  - internal/kernel/lspool/quirks.go
  - internal/kernel/lspool/quirks_test.go
  - internal/kernel/lspool/worker.go
  - internal/kernel/lspool/lease.go
  - internal/mcp/middleware.go
  - internal/obs/metrics.go
  - internal/obs/metrics_labels_test.go
  - test/integration/rust_test.go
findings:
  blocker: 0
  critical: 0
  major: 0
  minor: 2
  info: 3
  total: 5
status: issues-found
---

# Phase 47: Code Review Report

**Reviewed:** 2026-04-24
**Depth:** standard
**Files Reviewed:** 12 (+ USAGE.md)
**Status:** issues-found (all minor/info — no ship-blockers)

## Summary

Phase 47's hybrid rust-analyzer rename implementation is solid, well-tested, and matches the planned design (RCA + readiness gate + dispatcher + override wrapper + strategy metric + unskipped integration test + USAGE.md update). Import-cycle avoidance via `RustAnalyzerRenameOverride` wrapper is clean; closed-enum metric discipline is enforced at both the emission site (`obs.Metrics.RenameStrategyInc`) and the label-lint carve-out. Concurrency in `RustAnalyzerAdapter` (atomic flag + mutex-guarded channel) is correctly shaped for the one-writer / many-reader pattern, with the notification handler as the sole writer.

No blocker, critical, or major issues were found. Five minor/info items are noted below — all are cosmetic, edge-case, or ergonomic rather than correctness risks on the Phase 47 scope.

A threat-model cross-check confirms:
- T-47-01 (malformed payload): handled — `json.Unmarshal` into typed struct, no panic, no state change on error.
- T-47-02 (DoS via WaitUntilRenameReady): handled — 10s hard timeout + ctx cancellation path.
- T-47-05 (race): handled — atomic.Bool for hot path, mutex-guarded channel recreation.
- T-47-08 (metric cardinality): handled — closed-enum check in `RenameStrategyInc` and `RecordRenameStrategy`.

## Minor

### MN-01: Dead code — unused `pathToURI` helper in `rename.go`

**File:** `internal/kernel/edit/rename.go:249-255`
**Issue:** `pathToURI` is defined but has zero call-sites in the package or the repo (grep confirms only the definition matches). The conversion in the rename pipeline goes the other direction via `uriToPath` (defined in `replace.go`). This is pre-existing dead code but sits in a file heavily touched by Phase 47, so it's worth cleaning up here rather than letting it rot.
**Fix:** Remove the function and its `strings` import if no longer needed (check whether `strings` is used elsewhere in the file — it is not, so drop the import too).
```go
// Delete these lines:
// pathToURI converts a filesystem path to a file:// URI.
func pathToURI(path string) string {
    if strings.HasPrefix(path, "file://") {
        return path
    }
    return "file://" + path
}
```

### MN-02: `prepareRename` refined position ignored when symbol is at (0, 0)

**File:** `internal/kernel/edit/rename.go:142`
**Issue:** The guard `(rangeResult.Start.Line > 0 || rangeResult.Start.Character > 0)` treats a zero-value range as "prepareRename did not return a usable range", but LSP position (0, 0) is a legitimate file-start position. A symbol declared at the very first byte of a file (e.g. a shebang-less script entry, first identifier of `mod.rs`) would silently fall back to the user-supplied (line, col) rather than the `prepareRename`-refined position. In practice the user-supplied input for line/col 1 / 1 (1-indexed) converts to 0 / 0 anyway, so impact is nil for most workflows — but the check is semantically wrong.
**Fix:** Drop the zero-value guard and trust `json.Unmarshal` success to mean the range is usable. If the server returns `{start, end}` at all, it refined the position:
```go
var rangeResult struct {
    Start gen.Position `json:"start"`
    End   gen.Position `json:"end"`
}
if err := json.Unmarshal(prepareResult, &rangeResult); err == nil {
    // Heuristic: a real prepareRename range has Start <= End and End > Start
    // within the same line. Use the refined start regardless of whether it's (0,0).
    if rangeResult.End.Line > rangeResult.Start.Line ||
        (rangeResult.End.Line == rangeResult.Start.Line && rangeResult.End.Character > rangeResult.Start.Character) {
        pos = rangeResult.Start
    }
}
```

## Info

### IN-01: Potential missed "ready" edge under rapid `quiescent` flap

**File:** `internal/kernel/lspool/quirks.go:185-218` (`ensureReadyCh` / `signalReady` / `resetReadyCh`)
**Issue:** A `WaitUntilRenameReady` caller captures `ch := r.ensureReadyCh()` while `quiescent=false`, then blocks on `<-ch`. If the notification sequence is false → true → false → true in rapid succession, the sequence is:
1. `signalReady` closes `ch` (caller wakes, re-reads `quiescent.Load()`).
2. But if a `resetReadyCh` and a subsequent `signalReady` happen before the caller wakes, the caller still wakes up on the original closed `ch`, sees `quiescent.Load() == true` (current value), and returns `true`. OK.

The ordering that is actually problematic is: caller captures `ch`, then `quiescent` flips true→false (`resetReadyCh` replaces the channel with a fresh one), then true again (`signalReady` closes the new channel). The caller is still blocked on the stale un-closed original channel and never wakes. Mitigated by the 10s timeout fallback, but worth noting.
**Fix:** Not required for v1 given the timeout floor; if a tighter bound is needed, `WaitUntilRenameReady` could re-fetch `r.ensureReadyCh()` inside a short polling loop, or `resetReadyCh` could close the old channel after installing the new one (consumers would then wake, re-check `quiescent.Load()`, see `false`, and re-enter the wait via a retry loop). Low priority.

### IN-02: `signalReady` lazy-init branch leaks intent

**File:** `internal/kernel/lspool/quirks.go:198-202`
**Issue:** When `r.readyCh == nil`, `signalReady` constructs a pre-closed channel and stores it. This is correct but subtle: a future caller that hits `ensureReadyCh` after a `resetReadyCh` would get a fresh un-closed channel, whereas the pre-closed channel here shortcuts `WaitUntilRenameReady`. The two lazy-init paths (`ensureReadyCh` → fresh un-closed; `signalReady` → pre-closed) are mirror images and rely on the caller ordering being "`signalReady` may precede any `ensureReadyCh`". A one-line comment would save a future reader ten minutes.
**Fix:** Add a comment:
```go
// signalReady may be called before any WaitUntilRenameReady caller has
// observed the adapter. In that case we install a pre-closed channel so
// the next ensureReadyCh consumer returns immediately — matching the
// atomic quiescent=true store we just performed.
```

### IN-03: `RecordRenameStrategy` drops ctx but keeps it in signature

**File:** `internal/mcp/middleware.go:36-43`
**Issue:** `RecordRenameStrategy(ctx context.Context, strategy string)` takes a ctx, ignores it (`_ = ctx`), and calls a ctx-free callback. The TODO-style comment notes OTel integration is future. This is fine, but the unused-param pattern `_ = ctx` is slightly misleading compared to Go's usual practice of simply naming the parameter `_` or accepting it and leaving it for future use. Also the sink signature `func(strategy string)` could be `func(ctx context.Context, strategy string)` to pre-wire the OTel hook without a later call-site churn.
**Fix:** (Optional) widen the sink type now to avoid signature churn later:
```go
var renameStrategySink atomic.Pointer[func(ctx context.Context, strategy string)]
// and wire with an adapter closure:
setRenameStrategySink(func(ctx context.Context, strategy string) {
    m.RenameStrategyInc(strategy)
})
```

---

_Reviewed: 2026-04-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
