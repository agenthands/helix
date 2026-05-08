---
phase: 65-existing-tool-integration-strangler-fig
plan: 01
subsystem: semantic-indexing
tags:
  - buildfn
  - extract
  - snapshot
  - tdd

# Dependency graph
requires:
  - phase: 64-new-mcp-tools
    provides: "empty-Facts placeholder makeProductionBuildFn (TODO(phase-65) anchor) + IndexRunner / SemanticSkill wiring"
  - phase: 59-extract
    provides: "extract.Provider interface + extract.NewExtractorRegistry + go/typescript/python providers"
  - phase: 60-live-update
    provides: "live.ClassifyPathChange + storeFileHashLookup adapter"
  - phase: 65-research
    provides: "RESEARCH.md §Pattern 4 verbatim pipeline shape; verified signatures"
provides:
  - "Production buildFn pipeline (walk → classify → extract → ToStoreFacts → WriteSnapshotFacts)"
  - "factsFromExtracted: per-file FileID / NodeID / RefID assignment with 63-bit masking for duckdb-go"
  - "collectCandidatePaths: filepath.WalkDir-based walker that skips .git / .helix / dot-directories + symlinks"
  - "langFromExt: canonical .go/.ts/.tsx/.js/.jsx/.py → language identifier"
  - "TestProductionBuildFn_WritesNonEmptyFacts integration harness in internal/daemon/semantic_wiring_test.go"
affects:
  - "phase 65 wave 1 (strangler-fig integ tools rely on a non-empty snapshot for ranking signal)"
  - "TestE2E_IndexThenContext_SymbolCount in internal/skill/semantic — continues passing on its locked fixture"
  - "ROADMAP requirements INTEG-01, INTEG-02"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-file ID stamping inside the buildFn (FileID monotonic, SymbolID/NodeID/RefID/OwnerSymbolID/ParentScopeID masked to 63 bits per duckdb-go binding constraint at internal/semantic/store/overlay.go:937)"
    - "Helper extraction: collectCandidatePaths + classifyAndExtract + factsFromExtracted + langFromExt keep makeProductionBuildFn body under 75 lines"

key-files:
  created:
    - "internal/daemon/semantic_wiring_test.go (TestProductionBuildFn_WritesNonEmptyFacts + testBuildState)"
  modified:
    - "internal/daemon/semantic_wiring.go (production pipeline replaces empty-Facts placeholder; adds extractRegistry field on semanticBundle and 4 new helpers)"
    - "internal/daemon/daemon.go (callsite passes semanticExtractRegistry through newSemanticBundle)"

key-decisions:
  - "FileID assignment is buildFn-local (1-based monotonic per snapshot). The store does NOT auto-assign IDs; ToStoreFacts leaves them zero by contract; the integration test fixture stamps FileID=1 manually. Mirroring that pattern in the production buildFn is the simplest correct path until a future schema bump moves ID allocation server-side."
  - "Mask SymbolID / NodeID / OwnerSymbolID / ParentScopeID / RefID to 63 bits in factsFromExtracted (& 0x7FFFFFFFFFFFFFFF). The duckdb-go database/sql binding rejects uint64 values with the high bit set ('uint64 values with high bit set are not supported'). Same mitigation already in production at internal/semantic/store/overlay.go:937 for edge IDs. Collision probability across the per-workspace symbol surface stays vanishingly small."
  - "Use ChangeSourceFsnotify (not ChangeSourceWatcher — the latter does not exist; the plan's prose was aspirational). The classifier's behavior under ChangeSourceFsnotify is the desired path-vs-store comparison; the buildFn is a synthetic-walk source, not an edit notifier, so 'fsnotify' is the closest semantic match."
  - "Incremental mode currently re-walks. The live overlay does not expose an enumerable per-path drain surface today — Phase 60 bumped pending counters but kept the per-path set internal to the coalescer. Documented as a follow-up in collectCandidatePaths comment; the test plan only requires mode=full for RED/GREEN."

patterns-established:
  - "Pattern: factsFromExtracted (compose-and-stamp). Wraps the locked extract.ToStoreFacts adapter in a per-file loop that re-stamps IDs / RepoID. Preserves the file ↔ symbol / reference linkage that the flat ToStoreFacts wire shape intentionally drops."
  - "Pattern: storeFileHashLookup reuse. The buildFn instantiates the same adapter the live bundle uses (live_wiring.go:156-164) so the classifier sees a consistent view of the snapshot+overlay store across the live + buildFn paths."

metrics:
  duration_seconds: 438
  completed_at: "2026-05-08T12:56:07Z"
  red_commit: "8c614586"
  green_commit: "8ac60229"
  refactor_commit: "504a6b01"
---

# Phase 65 Plan 01: Production buildFn Pipeline (D-09 carryover #1) Summary

Replaced the Phase 64 empty-Facts placeholder in `internal/daemon/semantic_wiring.go::makeProductionBuildFn` with the production walk → classify → extract → ToStoreFacts → WriteSnapshotFacts pipeline documented at RESEARCH.md §Pattern 4. The strangler-fig integration tools in Wave 1+ now have real ranking signal — every `semantic`-source envelope renders against a populated graph instead of an empty snapshot.

## Tasks Executed

| Task | Commit | What landed |
|------|--------|-------------|
| RED | `8c614586` | Add `extractRegistry *extract.Registry` field on `semanticBundle`; thread through `newSemanticBundle` signature + `daemon.go` callsite. Add `TestProductionBuildFn_WritesNonEmptyFacts` against the unchanged empty-Facts placeholder — fails as expected (`FilesIndexed = 0, want >= 3`; `symbolCount = 0, want > 0`). |
| GREEN | `8ac60229` | Replace placeholder with the production pipeline: `collectCandidatePaths` (walker, skips .git / .helix / dot-dirs / symlinks) + `langFromExt` + classifier loop + `factsFromExtracted` (per-file FileID assignment with 63-bit masking) + snapshot lifecycle. Test passes (`FilesIndexed=3`, `symbolCount=3`). Closes Phase 64 VERIFICATION.md TODO(phase-65) carryover. |
| REFACTOR | `504a6b01` | Extract per-path classify+extract loop into `classifyAndExtract` helper. `makeProductionBuildFn` body shrinks from 134 → 74 lines (under plan threshold). All tests still pass. |

## Verification

- `go vet ./...` exits 0 (warnings are CGO-only; not vet errors).
- `go test ./internal/daemon/... -count=1 -race` PASSES (full suite, 4s).
- `go test ./internal/skill/semantic/... -run TestE2E_IndexThenContext_SymbolCount -count=1` PASSES — confirms the locked-fixture E2E continues to land on a non-zero count.
- `grep -c "ToStoreFacts" internal/daemon/semantic_wiring.go` returns 9 (≥ 1).
- `grep -c "ClassifyPathChange" internal/daemon/semantic_wiring.go` returns 5 (≥ 1).
- `grep "semanticstore.Facts{}" internal/daemon/semantic_wiring.go` returns one match — inside `factsFromExtracted` as the empty-fallback when zero files extracted; the placeholder was correctly removed (the plan allows it "in zero-fallback paths").

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Plan referenced live.ChangeSourceWatcher which does not exist**

- **Found during:** GREEN, first build attempt.
- **Issue:** Plan's `<interfaces>` section and RESEARCH.md §Pattern 4 sample call use `live.ChangeSourceWatcher`. The actual constant in `internal/semantic/live/signal.go:30-46` is `ChangeSourceFsnotify`. Build error: undefined identifier.
- **Fix:** Used `live.ChangeSourceFsnotify` in `classifyAndExtract`. Documented the rationale in `key-decisions` above (the buildFn is a synthetic walk, not a notifier; fsnotify is the closest semantic match for the classifier's desired path-vs-store comparison).
- **Files modified:** `internal/daemon/semantic_wiring.go`
- **Commit:** `8ac60229`

**2. [Rule 3 - Blocking] *Store does not implement live.FileHashLookup directly**

- **Found during:** GREEN, second build attempt.
- **Issue:** Plan's `<interfaces>` block documents `live.ClassifyPathChange(ctx, repoID, path, b.store, nil, ...)` — implying `*Store` is a `FileHashLookup`. It is not (`EffectiveContentHash` lives on the `storeFileHashLookup` adapter at `internal/daemon/live_wiring.go:156-164`, not on `*Store`).
- **Fix:** Reuse the existing `storeFileHashLookup` adapter — the same one the live bundle uses — so both paths see a consistent view.
- **Files modified:** `internal/daemon/semantic_wiring.go`
- **Commit:** `8ac60229`

**3. [Rule 1 - Bug] Per-file FileID collisions on (snapshot_id, file_id) primary key**

- **Found during:** GREEN, first test run (after build clean).
- **Issue:** `extract.ToStoreFacts` leaves `FileID` zero-valued by contract (per its package doc, "FileID is assigned by store at INSERT time"). In practice the store does NOT auto-assign — it inserts whatever the caller provides. Without per-file FileID stamping, every FileFact lands with `FileID=0`; the second insert violates the `PRIMARY KEY (snapshot_id, file_id)` constraint.
- **Fix:** Added `factsFromExtracted` helper that assigns a 1-based monotonic FileID per `ExtractedFile`, propagates it to every Symbol / Reference belonging to that file, and stamps `RepoID` on every FileFact. Mirrors the integration test fixture pattern (`makeFixtureFacts` at `internal/skill/semantic/integration_test.go:317`).
- **Files modified:** `internal/daemon/semantic_wiring.go`
- **Commit:** `8ac60229`

**4. [Rule 1 - Bug] uint64 high-bit set rejected by duckdb-go binding**

- **Found during:** GREEN, second test run (after FileID fix).
- **Issue:** Real xxhash64 SymbolIDs from the per-language extractors frequently have the high bit set. `database/sql` rejects them with `"uint64 values with high bit set are not supported"` at INSERT time on `semantic_symbols`.
- **Fix:** Mask `SymbolID / NodeID / OwnerSymbolID / ParentScopeID / RefID` to 63 bits inside `factsFromExtracted` (`& 0x7FFFFFFFFFFFFFFF`). Same mitigation already in production at `internal/semantic/store/overlay.go:937` for edge IDs. Collision probability across the per-workspace symbol surface stays vanishingly small.
- **Files modified:** `internal/daemon/semantic_wiring.go`
- **Commit:** `8ac60229`

### Auth Gates

None — fully autonomous TDD plan.

## Threat Surface

The plan's `<threat_model>` flagged 4 threats (`T-65-01-01..04`). Status:

- **T-65-01-01 (DoS via huge file)**: mitigated. `live.ClassifyPathChange` already enforces size + symlink rules (Phase 60 T-60-04-03); the buildFn reuses it instead of bypassing. `collectCandidatePaths` additionally rejects symlinks at enumeration time and skips `.git` / `.helix` / dot-directories.
- **T-65-01-02 (per-file extraction errors)**: mitigated. Read failures, classifier errors, and provider.Extract errors all log at debug and `continue` the loop. Total failure still commits an empty snapshot.
- **T-65-01-03 (info disclosure via error text)**: accepted as planned. Errors stay in `slog` at debug level inside the daemon; no MCP envelope at this layer.
- **T-65-01-04 (snapshot lifecycle write tier)**: accepted as planned. This plan owns the WRITE path (BeginSnapshot / CommitSnapshot); the read-tier enforcement lives in 65-03.

No new threat surface introduced.

## Known Stubs

None. The buildFn writes real Facts; per-file extraction errors are non-fatal but every successfully-extracted file lands on disk.

## TDD Gate Compliance

- RED gate (`test(65-01): ...`): `8c614586` — failing test landed before any implementation.
- GREEN gate (`feat(65-01): ...`): `8ac60229` — minimal implementation that flips the test from FAIL to PASS.
- REFACTOR gate (`refactor(65-01): ...`): `504a6b01` — `classifyAndExtract` extraction with the test still passing.

## Self-Check: PASSED

- `internal/daemon/semantic_wiring_test.go` exists.
- `internal/daemon/semantic_wiring.go` modified (production pipeline + helpers).
- `internal/daemon/daemon.go` modified (callsite).
- Commit `8c614586` (RED) found in `git log`.
- Commit `8ac60229` (GREEN) found in `git log`.
- Commit `504a6b01` (REFACTOR) found in `git log`.
- `go vet ./...` passes.
- `go test ./internal/daemon/... -count=1 -race` passes.
- `go test ./internal/skill/semantic/... -run TestE2E_IndexThenContext_SymbolCount -count=1` passes.
