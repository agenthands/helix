---
phase: 68-precise-filefactdiff-populator
plan: 01
subsystem: semantic-store
tags: [semantic-store, accessor, duckdb, filefact, tier1-seam]
dependency_graph:
  requires: []
  provides:
    - "store.PriorFileFact + store.PriorSymbol public types"
    - "(*Store).GetLatestFileFact(ctx, repoID, path) (PriorFileFact, bool, error)"
  affects:
    - 68-04 (tryFullDiff dispatcher consumes the new seam)
tech_stack:
  added: []
  patterns:
    - "CurrentGraphVersion-style nil-guard + sql.ErrNoRows mapping (overlay.go:984-1002)"
    - "LatestCommittedSnapshot reuse (effective_graph.go:208-226) — no inline duplicate of the MAX(snapshot_id) query"
    - "Store-local PriorSymbol shape avoids extract→store import cycle (Pitfall 1)"
key_files:
  created:
    - internal/semantic/store/filefact_accessor.go
    - internal/semantic/store/filefact_accessor_test.go
  modified: []
decisions:
  - "PriorFileFact.Symbols carries []store.PriorSymbol, NOT []extract.SymbolFact — required to avoid an import cycle (extract already imports store via to_store.go:46). Plan invoked D-01 Claude's Discretion for the return shape; this implementation took one step further than the plan sketch."
  - "ExtractionStatus is string (not extract.ExtractionStatus) for the same cycle-avoidance reason; callers compare against the extract.ExtractionStatus* enum string values."
metrics:
  duration_minutes: 18
  completed: 2026-05-13
  tasks_completed: 3
  files_created: 2
  files_modified: 0
---

# Phase 68 Plan 01: GetLatestFileFact Accessor Summary

One-liner: pre-edit `*Store.GetLatestFileFact` accessor + store-local
`PriorFileFact`/`PriorSymbol` types — reads `semantic_live_overlay_*`
first, falls back to the latest committed snapshot, returns
`(_, false, nil)` cold-start; closes DIFF-02 and unblocks 68-04.

## Tasks Completed

| Task | Name | Status | Commit | Files |
|------|------|--------|--------|-------|
| 1 | RED — failing tests for GetLatestFileFact | done | `d1696d5a` | `internal/semantic/store/filefact_accessor_test.go` |
| 2 | GREEN — implement accessor + types | done | `a6c1f2f1` | `internal/semantic/store/filefact_accessor.go` (+ test edits) |
| 3 | Vet boundary regression check | done | (verify-only) | — |

## Final Test List

All six required tests pass under `go test -race ./internal/semantic/store/... -count=1`:

| Test | Scenario | Result |
|------|----------|--------|
| `TestGetLatestFileFact_OverlayHit` | overlay file row live + 2 symbol rows with valid `fact_json` | PASS |
| `TestGetLatestFileFact_SnapshotFallback` | no overlay; committed snapshot with 2 symbols | PASS |
| `TestGetLatestFileFact_ColdStart` | empty store | PASS — `(zero, false, nil)` |
| `TestGetLatestFileFact_OverlayPlaceholder` | overlay row with `file_id=0` (Phase 60 P02 placeholder) | PASS — falls through to snapshot |
| `TestGetLatestFileFact_NilStore` | call on `var s *Store` | PASS — returns `"GetLatestFileFact: nil store"` |
| `TestGetLatestFileFact_EmptyRepoID` | empty repoID arg | PASS — returns `"empty repoID"` |

Full store package `go test -race` is green (no regression in
existing tests). `make vet` exits 0 — including the
`vet-nokernel2semantic` / `vet-nosemantic2kernel` /
`vet-compact-uses-store` custom analyzers.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Cycle] PriorFileFact.Symbols cannot be []extract.SymbolFact**

- **Found during:** Task 1 (RED test failed to compile with import-cycle error).
- **Issue:** The plan and 68-RESEARCH.md Example 1 both specify
  `PriorFileFact.Symbols []extract.SymbolFact` and import
  `internal/semantic/extract` into `internal/semantic/store`. That
  import is impossible: `internal/semantic/extract/to_store.go:46`
  imports `internal/semantic/store` for the `ToStoreFacts` adapter.
  Reversing the direction is a hard structural blocker, NOT a
  preference.
- **Fix:** Defined a store-local `PriorSymbol` struct in
  `filefact_accessor.go` mirroring the subset of `extract.SymbolFact`
  fields the Tier-1 diff consumes (`ID`, `StableKey`, `Name`, `Kind`,
  `Signature`, `SignatureHash`, `Visibility`). Field names match
  `extract.SymbolFact` so JSON unmarshal from
  `semantic_live_overlay_symbols.fact_json` (which Phase 60 P04 will
  populate with `extract.SymbolFact` JSON) "just works."
  `ExtractionStatus` likewise downgraded to plain `string` (carries the
  same `"ready"`/`"partial"` values the `extract.ExtractionStatus`
  closed-enum exports).
- **Why the plan got this wrong:** Pitfall 1 in 68-RESEARCH.md
  acknowledges the naming collision but resolves it by introducing
  `store.PriorFileFact` and recommending `Symbols []extract.SymbolFact`
  on it. The pitfall analysis stopped one step short — it did not
  notice that `extract → store` already exists, so the symbol slice
  type is also constrained.
- **Files modified:** `internal/semantic/store/filefact_accessor.go`,
  `internal/semantic/store/filefact_accessor_test.go`.
- **Caller impact:** Plan 68-04 (the `tryFullDiff` dispatcher in
  `internal/semantic/live/handler/difffacts.go`) lives in a package
  that CAN import both `store` and `extract`. It can convert
  `[]store.PriorSymbol → []extract.SymbolFact` for the existing
  `diffSymbols` helper in 68-PATTERNS.md, OR adapt `diffSymbols` to
  consume `store.PriorSymbol` directly. The fields it actually compares
  (`Signature`, `Visibility`, `Kind`, `StableKey`, `ID`) are present on
  both shapes.
- **Commit:** `a6c1f2f1`.

**2. [Rule 1 — Test infra] `t.Parallel()` incompatible with `t.Chdir`**

- **Found during:** Task 2 GREEN run.
- **Issue:** The plan mandates `t.Parallel()` on all six tests. The
  store test bring-up helper `openStoreForOverlayTest` calls
  `configFor` (`store_test.go:54`) which calls `t.Chdir`. Go ≥ 1.23
  panics when `t.Chdir` is invoked from a parallel test.
- **Fix:** Removed `t.Parallel()` from all six tests. Race detection
  still runs (`-race` flag at the package level catches data races
  during sequential execution).
- **Files modified:** `internal/semantic/store/filefact_accessor_test.go`.
- **Commit:** `a6c1f2f1`.

## Acceptance Criteria

- [x] File `internal/semantic/store/filefact_accessor.go` exists.
- [x] File `internal/semantic/store/filefact_accessor_test.go` exists.
- [x] `grep -c 'func (s \*Store) GetLatestFileFact'` = 1.
- [x] `grep -c 'type PriorFileFact struct'` = 1.
- [x] `grep -v '^//' filefact_accessor.go | grep -c 'internal/kernel'` = 0
      (vet-nokernel2semantic invariant preserved).
- [x] `grep -c 'BumpGraphVersion' filefact_accessor.go` = 0
      (D-06 single-call-site invariant).
- [x] 6 `TestGetLatestFileFact_*` tests present (target was ≥ 4 minimum).
- [x] `go test -race -run 'TestGetLatestFileFact' ./internal/semantic/store/... -count=1` → PASS.
- [x] `go test -race ./internal/semantic/store/... -count=1` → PASS (no
      regression in existing store tests).
- [x] `make vet` → exit 0 (all four custom vettools clean).

## Notes for Downstream Plans

### Overlay branch reachability (Pitfall 2)

The overlay-symbol-write path (Phase 60 P04 — full symbol upsert with
`fact_json` stamping) is **partially deferred** per 68-RESEARCH.md A1.
At time of writing, `semantic_live_overlay_symbols.fact_json` is
expected to be `NULL` or absent for rows produced by the production
live-update pipeline — only file-row upserts (P02) ship today. The
accessor's overlay branch is therefore exercised in tests via direct
JSON seeding, and is wired correctly for the future Phase 60 P04
landing, but **the production Tier-1 path in 68-04 will currently
return via the snapshot fallback** for any file that has not yet been
re-snapshotted since its last edit.

This is acceptable: snapshot is authoritative for committed state, and
the only practical loss is Tier-1 on rapid-fire edits to the same file
within one snapshot window (Pitfall 2 disposition). Operators will
observe `helix_live_filefactdiff_total{tier="full"}` rate climb when
Phase 60 P04 lands without any change to this accessor.

### Stable seam for 68-04

```go
func (s *Store) GetLatestFileFact(ctx context.Context, repoID, path string) (PriorFileFact, bool, error)

type PriorFileFact struct {
    Path             string
    Language         string
    ExtractionStatus string         // "ready" | "partial" | "unsupported" | "failed"
    Symbols          []PriorSymbol
}

type PriorSymbol struct {
    ID            semantic.SymbolID
    StableKey     string
    Name          string
    Kind          string
    Signature     string
    SignatureHash string
    Visibility    string             // "exported" | "private"
}
```

Plan 68-04's `FileFactStore` handler interface (per 68-PATTERNS.md
section "handler.go MOD — wiring") should be updated to:

```go
type FileFactStore interface {
    GetLatestFileFact(ctx context.Context, repoID, path string) (store.PriorFileFact, bool, error)
}
```

and `diffSymbols` should consume `[]store.PriorSymbol` directly (or
convert from the in-tx `*extract.ExtractedFile.Symbols` via a thin
adapter). The fields compared (`Signature`, `Visibility`, `Kind`,
`StableKey`, `ID`) are present on both `store.PriorSymbol` and
`extract.SymbolFact`.

## Threat Flags

None — this plan adds a read-only DuckDB accessor inside the existing
semantic-store trust domain. No new network surface, no new auth path,
no new file-access pattern at a trust boundary. The threat register
items T-68-01..T-68-04 are addressed:

- T-68-01 (tampering of fact_json): accepted — same trust domain.
  Per-symbol JSON unmarshal failure is non-fatal (skip + continue), so
  corruption causes data loss for that row but no privilege escalation.
- T-68-02 (info disclosure in errors): mitigated — error messages wrap
  `repoID` and `path` only, no DB column values leaked.
- T-68-03 (DoS via unbounded rows): accepted — bounded by per-file
  symbol count.
- T-68-04 (vet boundary): mitigated — Task 3 confirmed `make vet`
  including `vet-nokernel2semantic` exits 0.

## Self-Check: PASSED

- `internal/semantic/store/filefact_accessor.go` — FOUND.
- `internal/semantic/store/filefact_accessor_test.go` — FOUND.
- Commit `d1696d5a` (Task 1 RED) — FOUND.
- Commit `a6c1f2f1` (Task 2 GREEN) — FOUND.
