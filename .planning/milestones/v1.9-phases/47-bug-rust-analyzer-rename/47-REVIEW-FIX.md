---
phase: 47-bug-rust-analyzer-rename
fixed_at: 2026-04-24T00:00:00Z
review_path: .planning/phases/47-bug-rust-analyzer-rename/47-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 4
skipped: 1
status: partial
---

# Phase 47: Code Review Fix Report

**Fixed at:** 2026-04-24
**Source review:** .planning/phases/47-bug-rust-analyzer-rename/47-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (0 blocker, 0 critical, 0 major, 2 minor, 3 info)
- Fixed: 4 (MN-01, MN-02, IN-02, IN-03)
- Skipped: 1 (IN-01)

## Fixed Issues

### MN-01: Dead code — unused `pathToURI` helper in `rename.go`

**Files modified:** `internal/kernel/edit/rename.go`
**Commit:** b598e173
**Applied fix:** Removed the unused `pathToURI` function and the now-unused `strings` import. Verified via grep that no callers exist in the package or repo (the `pathToURI` in `internal/kernel/symbols/tools.go` is a different 2-arg function in a different package). `go build ./internal/kernel/edit/` succeeds.

### MN-02: `prepareRename` refined position ignored when symbol is at (0, 0)

**Files modified:** `internal/kernel/edit/rename.go`
**Commit:** 968d7241
**Applied fix:** Replaced the zero-value guard `(rangeResult.Start.Line > 0 || rangeResult.Start.Character > 0)` with an `End > Start` invariant check. The refined Start position is now adopted even at `(0, 0)`, as long as the server returned a well-formed range (End strictly after Start). This also extracts the End field from the JSON payload. Added a reference comment citing the REVIEW. `go build ./internal/kernel/edit/` succeeds.

### IN-02: `signalReady` lazy-init branch leaks intent

**Files modified:** `internal/kernel/lspool/quirks.go`
**Commit:** 235d01bb
**Applied fix:** Added a comment inside the `r.readyCh == nil` branch of `signalReady` explaining why a pre-closed channel is installed (a future `ensureReadyCh` caller gets immediate wake-up matching the just-stored `quiescent=true`, mirroring the fresh un-closed channel lazily created by `ensureReadyCh` on its own first-access path). `go build ./internal/kernel/lspool/` succeeds.

### IN-03: `RecordRenameStrategy` drops ctx but keeps it in signature

**Files modified:** `internal/mcp/middleware.go`
**Commit:** 41cab279
**Applied fix:** Widened the package-level sink type from `func(strategy string)` to `func(ctx context.Context, strategy string)`. Removed the `_ = ctx` pattern in `RecordRenameStrategy`; ctx is now passed to the sink. Updated the `InstallMiddleware` wire-up to install an adapter closure that discards ctx today but provides a seam for future OTel span attribute attachment without caller churn. `go build ./...` and `go vet ./internal/mcp/` succeed.

## Skipped Issues

### IN-01: Potential missed "ready" edge under rapid `quiescent` flap

**File:** `internal/kernel/lspool/quirks.go:185-218`
**Reason:** skipped: review explicitly states "Not required for v1 given the timeout floor" and classifies the finding as low priority. The 10s hard timeout in `WaitUntilRenameReady` already bounds the worst-case stall, and the proposed mitigations (polling loop, close-then-replace channel rotation) trade away the single-channel-per-generation invariant for marginal latency improvement on an unlikely race. Deferring to a future phase if tighter bounds become necessary.
**Original issue:** A `WaitUntilRenameReady` caller that captures a channel and then observes a `true → false → true` flap may remain blocked on the stale un-closed original channel until the 10s fallback fires, because `resetReadyCh` installs a fresh channel and the subsequent `signalReady` closes only the new one.

---

_Fixed: 2026-04-24_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
