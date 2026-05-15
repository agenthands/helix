---
phase: 70
plan: 05
subsystem: semantic-skill
tags: [mcp-tool, refresh, semantic-skill, overlay-drain, wave3]
dependency_graph:
  requires:
    - "70-01: *Store.OverlayChangedPathsSince accessor"
    - "70-03: Service.FlushNow synchronous coalescer drain"
    - "70-04: StoreAccessor + LiveAccessor interface extensions"
  provides:
    - "Honest refresh_semantic_graph.files_updated derived from the overlay seam"
  affects:
    - "internal/skill/semantic/tools_refresh.go"
    - "internal/skill/semantic/tools_refresh_test.go"
tech-stack:
  added: []
  patterns:
    - "preEpoch capture → drain → FlushNow → seam read"
    - "Non-fatal flush error via slog.Default().Warn"
    - "Seam-truth policy (response count from store, not request slice)"
key-files:
  created: []
  modified:
    - internal/skill/semantic/tools_refresh.go
    - internal/skill/semantic/tools_refresh_test.go
decisions:
  - "Seam-truth policy locked in tests: files_updated reflects what landed in the overlay since preEpoch, NOT len(args.Paths). args.Paths governs WHICH paths the live service drains, not the response count. The seam can legitimately surface fewer or more paths than args.Paths depending on coalescer behavior."
  - "FlushNow errors are non-fatal: slog.Warn + continue. The seam read still surfaces whatever landed, bounded by max_batch_delay_ms in the worst case (Pitfall 1 mitigation continues to function under flush failure)."
  - "preEpoch capture happens even when s.live == nil so the seam baseline remains consistent across has-live / no-live wirings — the response shape stays identical."
  - "Test-harness fix: skip s.SetLive when live==nil so s.live stays as a true nil interface (typed-nil pointer wrapped in an interface would defeat the s.live != nil guard in production code). Required by TestRefreshHandler_FilesUpdated_NoLive_StillReadsSeam."
metrics:
  duration: "~12 min"
  completed: "2026-05-15"
requirements_completed: [REFRESH-01]
---

# Phase 70 Plan 05: refresh_semantic_graph Overlay-Seam files_updated — Summary

Replaced the `files_updated := len(args.Paths)` wart in `handleRefreshSemanticGraph` with an honest count derived from the Phase 70-01 overlay seam. The handler now captures `preEpoch` via `store.CurrentOverlayEpoch` BEFORE firing `OnWorkspaceChanged`, synchronously drains the coalescer via `live.FlushNow(ctx, ws)` (closes RESEARCH.md Pitfall 1 — fire-and-forget race), then queries `store.OverlayChangedPathsSince(ctx, ws.Hash(), preEpoch)` for the actual set of paths whose overlay rows landed during the call. Closes the second seam consumer (CONTEXT.md D1 — single seam, two consumers; Plan 70-04 wired the first consumer in `collectCandidatePaths`).

## Tasks Completed

| Task | Name                                                      | Commit       |
| ---- | --------------------------------------------------------- | ------------ |
| 1    | RED — failing tests for seam-derived files_updated        | `99949fe7`   |
| 2    | GREEN — rewrite handleRefreshSemanticGraph derivation     | `a577821f`   |

## What Was Built

### Test surface (`internal/skill/semantic/tools_refresh_test.go`)

- **Extended `recorderStoreAccessor`** with Phase 70-05 seam injection fields:
  - `currentOverlayEpoch` / `currentOverlayEpochErr` + `currentOverlayEpochCalls` counter — injects the preEpoch baseline and records how many times the handler reads it.
  - `overlayChangedPaths` / `overlayChangedEpoch` / `overlayChangedErr` — canned return tuple for `OverlayChangedPathsSince`.
  - `mu` + `seamBaseEpochCalls []uint64` — records every `baseEpoch` argument the handler passes, so tests assert the preEpoch value flowed through correctly. Returned slice is copied to defeat aliasing.
- **Extended `mockLiveAccessor`** with `flushNowErr` (injectable error) + `flushNowCalls atomic.Int64` (counter).
- **Updated `TestRefreshHandler_HappyPath_DrainsLive`** to inject 3 seam paths so the existing assertion `FilesUpdated == 3` stays valid under the new derivation. Same paths could land via either old or new code paths.
- **Updated `newSkillForRefreshTest`** to skip `s.SetLive(live)` when `live==nil`. Without this, passing a typed-nil `*mockLiveAccessor` through the `LiveAccessor` interface produces a non-nil interface wrapping a nil pointer, which defeats the `s.live != nil` guard in production code. The NoLive sub-test requires `s.live` to be a true nil interface.
- **Added four new tests** asserting the new behavior:
  - `TestRefreshHandler_FilesUpdated_FromSeam_UnfilteredDrain` — `args.Paths == nil`, seam returns 2 paths; asserts `FilesUpdated == 2`, exactly one `CurrentOverlayEpoch` call, exactly one `FlushNow` call, and `seamBaseEpochCalls == [5]` (preEpoch flowed through).
  - `TestRefreshHandler_FilesUpdated_FromSeam_FilteredSubset` — `args.Paths == ["a.go"]`, seam returns `["a.go","b.go"]`; asserts `FilesUpdated == 2` (seam-truth, NOT subset truth). Doc-string locks the policy.
  - `TestRefreshHandler_FilesUpdated_Seam_FlushNowErrorIsNonFatal` — FlushNow returns sentinel `errFlushNow`; handler still proceeds to seam read; response is success with `FilesUpdated == 1`.
  - `TestRefreshHandler_FilesUpdated_NoLive_StillReadsSeam` — `s.live == nil`; preEpoch capture (`currentOverlayEpochCalls == 1`) and seam read (`seamBaseEpochCalls == [2]`) still happen; `FilesUpdated == 0` because no drain.

### Production handler (`internal/skill/semantic/tools_refresh.go`)

Three localized edits inside `handleRefreshSemanticGraph`:

1. **preEpoch capture** (before the `if s.live != nil {` block):
   ```go
   var preEpoch uint64
   if s.store != nil {
       preEpoch, _ = s.store.CurrentOverlayEpoch(ctx, ws.Hash())
   }
   ```
   Cold-start safe: nil store or never-touched overlay returns 0, which the seam treats as "match all rows" (Plan 70-01 contract).

2. **Synchronous FlushNow** (inside the `if s.live != nil {` block, after `OnWorkspaceChanged` returns successfully):
   ```go
   if err := s.live.FlushNow(ctx, ws); err != nil {
       slog.Default().Warn("refresh_semantic_graph: live flush error; files_updated may undercount",
           "ws", ws.Hash(), "err", err)
   }
   ```
   Non-fatal — the seam read still proceeds and surfaces whatever the coalescer already landed (bounded by `max_batch_delay_ms` in the worst case).

3. **Seam-derived files_updated** (replacing the `len(args.Paths)` line):
   ```go
   var filesUpdated int
   if s.store != nil {
       changed, _, _ := s.store.OverlayChangedPathsSince(ctx, ws.Hash(), preEpoch)
       filesUpdated = len(changed)
   }
   ```

Import block gains `"log/slog"` (other semantic skill files already use it; verified via `grep -l '"log/slog"' internal/skill/semantic/*.go`).

## Verification

| Check | Command | Result |
|---|---|---|
| New tests pass | `go test ./internal/skill/semantic/ -race -run 'TestRefreshHandler_FilesUpdated' -count=1` | PASS |
| Full refresh suite | `go test ./internal/skill/semantic/ -race -run 'TestRefresh' -count=1` | PASS |
| Full skill package | `go test ./internal/skill/semantic/ -race -count=1` | PASS (5.9s) |
| `go vet` semantic skill | `go vet ./internal/skill/semantic/...` | clean (only pre-existing tree-sitter swift cgo warnings) |
| `make vet` (all four tools) | `make vet` | clean (vettool, vet-nokernel2semantic, vet-nosemantic2kernel, vet-compact-uses-store) |

Grep gates (acceptance criteria from the plan):

```
$ grep -nE '\b(BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshot)\b' internal/skill/semantic/tools_refresh.go
(no matches — D-09/D-13 gate GREEN)

$ grep -nE 'files_updated semantics: when args\.Paths is supplied' internal/skill/semantic/tools_refresh.go
(no matches — old wart comment removed)

$ grep -nE 'OverlayChangedPathsSince\(ctx' internal/skill/semantic/tools_refresh.go
198:		changed, _, _ := s.store.OverlayChangedPathsSince(ctx, ws.Hash(), preEpoch)

$ grep -nE 'FlushNow\(ctx' internal/skill/semantic/tools_refresh.go
175:		if err := s.live.FlushNow(ctx, ws); err != nil {
```

All gates match: exactly one seam read, exactly one FlushNow call, zero forbidden tokens.

## TDD Gate Compliance

Strict RED → GREEN sequence enforced:

- **RED** at `99949fe7`: 4 new failing tests committed; assertions all referenced `files_updated` or the new seam counters. Existing tests stayed green (the HappyPath test was updated in the SAME commit to inject seam paths so it remains assertion-valid).
- **GREEN** at `a577821f`: production handler edits; all 4 new tests pass; existing tests remain green.
- **REFACTOR** — not needed; implementation landed cleanly on first GREEN pass.

## Deviations from Plan

- **[Rule 2 — Critical correctness] Test-harness typed-nil-interface fix.** The plan's `NoLive_StillReadsSeam` sub-test would have silently passed (or hit a nil-pointer panic) without fixing the `newSkillForRefreshTest` helper. When the helper calls `s.SetLive(live)` with `live *mockLiveAccessor = nil`, Go wraps the typed-nil pointer in a non-nil `LiveAccessor` interface, so the production `s.live != nil` guard would still enter the live block. Added a `live != nil` conditional in the helper so `s.live` stays as a true nil interface, with an inline comment explaining the typed-nil-in-interface trap. This is harness-correctness work, not a behavior change in production code.

- **Sentinel error pattern for FlushNow injection.** The plan suggested generic error injection. Implemented as a package-level `var errFlushNow = flushNowError("flush failed")` with a tiny `flushNowError` string type. Self-contained inside the test file; matches the existing test-file convention for sentinel errors.

Otherwise the plan executed as written: locked seam-truth policy in the FilteredSubset test (with the policy explanation copied verbatim into the test doc-comment), three production edits at the exact locations the plan called out, slog import added.

## Decisions Made

1. **Seam-truth policy LOCKED for files_updated.** When `args.Paths` is non-empty (strict-subset request), `files_updated` still reflects the seam's count — what actually landed in the overlay since `preEpoch` — NOT `len(args.Paths)`. This is encoded as a hard assertion in `TestRefreshHandler_FilesUpdated_FromSeam_FilteredSubset` (seam returns 2, request asks for 1, response carries 2). Rationale: the seam is the ground truth for "what changed in the workspace's overlay during this call." The `args.Paths` slice is a *driver* (tells the live service which paths to push through), not a *filter* on the count. Coalescer behavior may legitimately surface fewer or more paths than requested; the response reflects reality.

2. **FlushNow errors are non-fatal.** Implemented via `slog.Default().Warn(...)`. The seam read still proceeds. Worst case: the response under-counts by what the next natural coalescer flush eventually picks up (bounded by `max_batch_delay_ms`). A fatal flush error would make the tool fail open across an entire workspace's pipeline during transient coalescer hiccups, which is the wrong tradeoff for a read+ tool every session calls. Verified by `TestRefreshHandler_FilesUpdated_Seam_FlushNowErrorIsNonFatal`.

3. **preEpoch capture is unconditional on `s.live != nil`.** Even with no live service wired, the handler still captures `preEpoch` and reads the seam — they form a paired baseline/landing pair. This keeps the response shape stable across no-live test wirings and edge-case daemon bootstrap (live not yet wired). The cost is one accessor call per refresh; negligible vs. the LSP-wait polling already in the handler.

## Known Stubs

None. All new production code paths are wired; the test stubs already extended in Plan 70-04 now have full injection points.

## Threat Flags

None. The new code only consumes already-trusted seams (`StoreAccessor.CurrentOverlayEpoch`, `LiveAccessor.FlushNow`, `StoreAccessor.OverlayChangedPathsSince`) and adds a `slog.Warn` log line on flush error. No new network, auth, file-access, or schema surface introduced. The D-09 / D-13 read-only invariant on `tools_refresh.go` is preserved (grep gate green).

## Commits

| Hash | Message |
|---|---|
| `99949fe7` | `test(70-05): add failing tests for seam-derived files_updated` |
| `a577821f` | `feat(70-05): refresh files_updated derives from overlay seam` |

## Self-Check: PASSED

- `[ -f internal/skill/semantic/tools_refresh.go ]` → FOUND (modified — preEpoch capture, FlushNow call, seam read)
- `[ -f internal/skill/semantic/tools_refresh_test.go ]` → FOUND (modified — 4 new tests + injection fields)
- `git log --oneline | grep 99949fe7` → FOUND `test(70-05): add failing tests for seam-derived files_updated`
- `git log --oneline | grep a577821f` → FOUND `feat(70-05): refresh files_updated derives from overlay seam`
- D-09/D-13 grep gate: 0 matches for `Begin/Commit/Abort/WriteSnapshot` tokens
- Old wart comment removed: 0 matches for `files_updated semantics: when args\.Paths is supplied`
- New seam read: exactly 1 match for `OverlayChangedPathsSince\(ctx`
- New flush call: exactly 1 match for `FlushNow\(ctx`
- `go test ./internal/skill/semantic/ -race -count=1` → PASS
- `make vet` → clean
