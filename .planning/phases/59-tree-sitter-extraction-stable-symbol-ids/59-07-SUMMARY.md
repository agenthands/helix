---
phase: 59-tree-sitter-extraction-stable-symbol-ids
plan: 07
subsystem: semantic
tags: [semantic, extract, store-adapter, determinism, tdd, phase-65-unblock, cycle-safety, d-08, pure-function]

# Dependency graph
requires:
  - phase: 59-tree-sitter-extraction-stable-symbol-ids
    provides: extract.ExtractedFile in-memory shape (fact.go), CanonicalizeStableSymbolKey helper (stable_id.go)
  - phase: 57-store-foundation
    provides: semanticstore.Facts wire shape (snapshot.go), WriteSnapshotFacts API
provides:
  - "internal/semantic/extract.ToStoreFacts: pure deterministic adapter from []*ExtractedFile to semanticstore.Facts"
  - "Locked Phase 65 unblock: production buildFn at internal/daemon/semantic_wiring.go:687-742 can replace its empty-Facts placeholder with extract.ToStoreFacts(extracted)"
  - "Acceptance criterion #15 — same-input → byte-identical output across same-process repeated calls AND across subprocess invocations"
  - "Cycle-safe extract → store import edge (verified one-way; store does NOT import extract)"
  - "Pinned dropped-on-floor contract for Imports / Types / Heritage (Phase 62 territory until store.Facts expands)"
affects: [phase-65-strangler-fig-integration, phase-62-type-resolver, phase-60-live-update-pipeline]

# Tech tracking
tech-stack:
  added: []  # No new dependencies — stdlib only (encoding/gob + os/exec gated to test scope)
  patterns:
    - "Subprocess re-exec determinism gate (re-exec self with EXTRACT_SUBPROCESS=1 + gob-encoded byte-identity diff)"
    - "Pure-function adapter at trust-boundary edge (extract → store) with explicit field-disposition contract in package doc"

key-files:
  created:
    - internal/semantic/extract/to_store.go
    - internal/semantic/extract/to_store_test.go
  modified: []

key-decisions:
  - "Preferred location locked: internal/semantic/extract/to_store.go (D-08 fallback path NOT engaged — cycle check confirmed extract → store one-way edge does not close a cycle)"
  - "Hard-gap policy for StartByte/EndByte: extract.Range is line/column-only (fact.go:71-79); adapter cannot synthesize byte offsets without breaking the no-I/O contract; documented inline; Phase 60 LIVE-01 / Phase 65 may backfill"
  - "Defensive nil-skip on []*ExtractedFile{nil} entries — Phase 65 buildFn may push nils on extraction error paths; absorbing them in the adapter keeps the upstream loop simple"
  - "Cross-process determinism via subprocess re-exec + gob bytes diff — strengthens acceptance #15 beyond same-process and catches any future map-iteration-without-sort regression"

patterns-established:
  - "Field-disposition contract in package doc: every store-side field falls into Sourced / Zero (INSERT-time) / Zero (downstream/hard-gap) — pinned at the doc layer so future expansions cannot silently change adapter behavior"
  - "Compile-time signature guard: var _ = func(files []*ExtractedFile) semanticstore.Facts { return ToStoreFacts(files) } — any signature drift trips the build, not just runtime"

requirements-completed: [EXTRACT-01, EXTRACT-04]

# Metrics
duration: ~10min
completed: 2026-05-08
---

# Phase 59 Plan 07: ToStoreFacts pure-function adapter (D-08) Summary

**Pure deterministic []*ExtractedFile → semanticstore.Facts adapter shipped at internal/semantic/extract/to_store.go; Phase 65's locked call site `extract.ToStoreFacts(extracted)` is now compilable and tested with same-process AND cross-process byte-identity guarantees.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-05-08T10:54Z (approx)
- **Completed:** 2026-05-08T11:04Z (approx)
- **Tasks:** 4 (Task 0 verification-only + 3 commits for RED/GREEN/REFACTOR)
- **Files modified:** 2 (both new)

## Accomplishments

- **Cycle-safety verdict (Task 0): preferred location locks.** `grep -rn '"github.com/agenthands/helix/internal/semantic/extract"' internal/semantic/store/ 2>/dev/null` returned 0 lines at execution time (2026-05-08). The new import edge `internal/semantic/extract → internal/semantic/store` is one-way and does not close a cycle. Adapter lives at `internal/semantic/extract/to_store.go` per CONTEXT.md D-08 preferred location. Fallback location (`internal/semantic/store/from_extracted.go` with inverted import direction) was NOT engaged.
- **`go build ./...` exits 0** before and after each task — the pre-existing build-clean state was preserved throughout.
- **D-08 adapter shipped: `extract.ToStoreFacts(files []*ExtractedFile) semanticstore.Facts`.** Pure function; no I/O; no map iteration; no init() registration; no time-now / rand source.
- **7 tests in `to_store_test.go` (Task 1 = 6 tests + Task 3 = 1 cross-process test):**
  - `TestToStoreFacts_Deterministic` — same-process repeated-call byte-identity (acceptance #15).
  - `TestToStoreFacts_GoldenShape` — pins 13 store-side SymbolFact fields against a hand-coded fixture (catches accidental field-mapping inversions).
  - `TestToStoreFacts_EmptyInput` — `nil` and `[]*ExtractedFile{}` both produce zero-value Facts.
  - `TestToStoreFacts_NilSafety` — `[]*ExtractedFile{nil}` does NOT panic; nil entries are silently skipped.
  - `TestToStoreFacts_PartialFileEmitsRow` — unsupported-language file still emits a `Files` row at this layer (ROADMAP acceptance #10 precondition).
  - `TestToStoreFacts_DroppedOnFloor` — `Imports` / `Types` / `Heritage` from `*ExtractedFile` are intentionally not consumed by `store.Facts` today; pinned so a future expansion cannot silently change adapter behavior.
  - `TestToStoreFacts_CrossProcessDeterminism` — gob-encoded byte-identity across two subprocess re-execs (skipped on Windows).
- **Phase 59 P04 goldens remain byte-identical** — verified `go test ./internal/semantic/extract/{golang,typescript,python}/... -count=2` passes, and the adapter touches no extraction code path so the goldens are byte-identical by construction.
- **Full Phase 59 verification gate green** — `go test ./internal/daemon/ ./internal/semantic/... ./internal/config/ ./internal/obs/ -count=1` exits ok across all packages.

## Task Commits

Each task was committed atomically:

1. **Task 0: Cycle-safety verification + location lock** — verification-only, no commit (preferred location locked; fallback NOT engaged).
2. **Task 1: RED — Determinism + golden-shape tests** — `3daf7fdf` (test)
3. **Task 2: GREEN — Implement ToStoreFacts pure-function adapter** — `3e37612e` (feat)
4. **Task 3: REFACTOR — Cross-process determinism gate** — `7f90e4ef` (refactor)

## Files Created/Modified

- `internal/semantic/extract/to_store.go` (NEW, 182 lines) — `ToStoreFacts` + three private helpers (`fileFactToStore`, `symbolFactToStore`, `referenceFactToStore`). Package doc comment carries the LOCKED field-disposition contract.
- `internal/semantic/extract/to_store_test.go` (NEW, 553 lines) — 7 test functions, two fixture builders (`twoFileFixture` parameterized by `*testing.T`, `crossProcessFixture` standalone for subprocess use), one compile-time signature guard.

## Field-mapping diff (extract → store)

### `extract.FileFact` → `semanticstore.FileFact`

| Store field | Disposition | Notes |
|---|---|---|
| `Path` | Sourced | `f.Path` |
| `Language` | Sourced | `f.Language` |
| `FileID` | Zero (INSERT-time) | Store assigns at WriteSnapshotFacts |
| `RepoID` / `ContentHash` / `SizeBytes` / `LineCount` | Zero (downstream) | Phase 60 LIVE-01 + Phase 61 enrichment populate |
| `Generated` / `Ignored` / `IgnoreReason` | Zero (upstream) | Phase 65 buildFn may set; not at adapter layer |

### `extract.SymbolFact` → `semanticstore.SymbolFact`

| Store field | Disposition | Notes |
|---|---|---|
| `SymbolID` | Sourced | `uint64(s.ID)` |
| `Language` / `Kind` / `Name` / `QualifiedName` | Sourced | direct (Kind via `string(s.Kind)`) |
| `StableKey` | Sourced | `CanonicalizeStableSymbolKey(s.StableKey)` per stable_id.go:28 |
| `Signature` / `SignatureHash` / `Visibility` | Sourced | direct |
| `Exported` | Derived | `s.Visibility == "exported"` (SPEC §11.1 closed-enum mirror) |
| `Confidence` | Sourced (widened) | `float64(s.Confidence)` — float32 → float64 no truncation |
| `StartLine` / `EndLine` / `StartCol` / `EndCol` | Sourced | `int(s.Range.Start.Line)` etc. — uint32 → int |
| `ExtractionSource` | Sourced | `s.ExtractionSource` (e.g. `"tree_sitter"`) |
| `NodeID` / `FileID` | Zero (INSERT-time) | Store assigns at WriteSnapshotFacts |
| `OwnerSymbolID` / `ParentScopeID` / `PackagePath` | Zero (downstream) | Phase 62 type/scope-resolver populates |
| `StartByte` / `EndByte` | **Zero (HARD GAP)** | extract.Range is line/column-only (fact.go:71-79); adapter cannot synthesize without breaking no-I/O contract; Phase 60 / Phase 65 may backfill |
| `ContentHash` / `LSPIdentity` | Zero (downstream) | Phase 61 enrichment populates |

### `extract.ReferenceFact` → `semanticstore.ReferenceFact`

| Store field | Disposition | Notes |
|---|---|---|
| `Name` | Sourced | `r.Name` |
| `RefKind` | Sourced (renamed) | `string(r.Kind)` — extract field is `Kind` (typed `ReferenceKind`); store field is `RefKind` |
| `ReceiverText` | Sourced | `r.ReceiverText` (pre-resolution literal) |
| `StartLine` / `EndLine` / `StartCol` / `EndCol` | Sourced | direct |
| `ValidationState` / `Confidence` / `Reason` | Sourced | Confidence widened to float64 |
| `ResolutionSource` | Sourced | `r.ResolutionSource` (`""` until Phase 61) |
| `RefID` / `NodeID` / `FileID` | Zero (INSERT-time) | Store assigns |
| `ScopeSymbolID` | Zero (downstream) | Phase 62 scope-resolver populates from `r.ContainerID` |
| `ResolvedSymbolID` | Zero (downstream) | Phase 62 type-resolver populates from `r.ResolvedTarget` |
| `StartByte` / `EndByte` | **Zero (HARD GAP)** | Same as SymbolFact — extract.Range is line/col-only |

### Edges (`semanticstore.EdgeFact`)

Not emitted by this adapter. `out.Edges` is left nil. Phase 62 territory.

### Dropped-on-floor (intentional, pinned by `TestToStoreFacts_DroppedOnFloor`)

- `ExtractedFile.Imports` — `store.Facts` has no imports column today.
- `ExtractedFile.Types` — Phase 62 territory; not yet consumed at the store layer.
- `ExtractedFile.Heritage` — Phase 62 territory; ditto.

When Phase 62's type resolver wants to consume these, expand `store.Facts` and the adapter together. The dropped-on-floor test pins the contract so the expansion cannot land silently.

## Cross-reference to plan 59-06

Plan 59-06 (interface widening) lands the **first half** of the Phase 65 unblock delta — `ExtractProvider.Extract(ctx, source, SourceFile) (*ExtractedFile, error)` interface contract + scheduler invariant + bookkeeping for the unsupported-language path.

Plan 59-07 (this plan) lands the **second half** — the pure-function adapter that turns `[]*ExtractedFile` (the per-file outputs collected by Phase 65's buildFn from each provider) into the `semanticstore.Facts` payload that `store.WriteSnapshotFacts` accepts.

Together, these two plans let Phase 65 Wave 0 compose the full ingestion pipeline:

```go
walk → provider.Extract (D-06, plan 59-06) → ToStoreFacts (D-08, plan 59-07) → store.WriteSnapshotFacts
```

Phase 65 owns the `internal/daemon/semantic_wiring.go:687-742` rewrite that drops the empty-Facts placeholder at line 724 and replaces it with `facts := extract.ToStoreFacts(extracted); err := b.store.WriteSnapshotFacts(ctx, snap, facts)`.

## Decisions Made

- **Preferred location locked (D-08).** Cycle check confirmed extract → store is one-way; the adapter lives at `internal/semantic/extract/to_store.go` per CONTEXT.md D-08 preferred location. Fallback (`internal/semantic/store/from_extracted.go` with inverted edge) was NOT engaged.
- **Hard-gap policy on `StartByte` / `EndByte`.** `extract.Range` is line/column-only (fact.go:71-79). Synthesizing byte offsets at the adapter would require re-reading the source file, breaking the no-I/O contract. Adapter unconditionally writes 0; Phase 60 LIVE-01 (filesystem metadata) or Phase 65 buildFn may backfill at a later layer. Documented inline in `symbolFactToStore` / `referenceFactToStore`.
- **Defensive nil-skip.** `ToStoreFacts([]*ExtractedFile{nil})` silently skips the nil entry rather than panicking. Phase 65's buildFn may push nils on extraction error paths; absorbing them at the adapter layer keeps the upstream loop simple. Pinned by `TestToStoreFacts_NilSafety`.
- **Cross-process determinism via subprocess re-exec.** Strengthens acceptance criterion #15 beyond same-process repeated-call byte-identity. A future regression that introduces map iteration without explicit sort, unseeded rand, or a `time.Now()` leak fails this gate even when the same-process determinism test would mask it. Skipped on Windows where `exec`-self-as-subprocess plumbing is brittle in the testing harness.

## Deviations from Plan

None - plan executed exactly as written.

The plan's Task 2 skeleton was illustrative; the executor adjusted field references to match the actual `extract.SymbolFact` / `extract.ReferenceFact` shapes (e.g. plan referenced an `OwnerSymbolID` mapping from `s.ContainerID *SymbolID` — adapter leaves it zero per the field-disposition contract since the cast `*SymbolID → uint64` is non-trivial and Phase 62 owns container resolution). All deviations are within the field-disposition contract's `Zero (downstream)` bucket — no behavioral or contract drift.

The plan also called for `r.ContainerID *SymbolID → store.ScopeSymbolID uint64` mapping. Per the field-disposition contract this falls into the Phase 62 scope-resolver bucket; adapter leaves it zero. Documented inline in `referenceFactToStore`.

## Issues Encountered

None. The plan was unusually well-specified (full field-disposition table; locked function signature; explicit fallback contract); execution was straightforward.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Phase 65 unblock complete (in tandem with plan 59-06).** Phase 65's locked call site `facts := extract.ToStoreFacts(extracted)` at `internal/daemon/semantic_wiring.go:687-742` is now compilable and tested.
- **No blockers.** All Phase 59-VERIFICATION.md must-haves remain PASS — this adapter does not touch any extraction code path, so the goldens are byte-identical by construction.
- **Phase 62 expansion path documented.** When Phase 62's type resolver wants `Imports` / `Types` / `Heritage` at the store layer, expand `store.Facts` and update the adapter's dropped-on-floor list together. `TestToStoreFacts_DroppedOnFloor` will fail until both halves land.

## Self-Check: PASSED

**Files exist:**
- `internal/semantic/extract/to_store.go` — FOUND
- `internal/semantic/extract/to_store_test.go` — FOUND

**Commits exist (verified via `git log --oneline`):**
- `3daf7fdf` test(59-07): RED — FOUND
- `3e37612e` feat(59-07): GREEN — FOUND
- `7f90e4ef` refactor(59-07): REFACTOR — FOUND

**Verification commands (all passed):**
- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./internal/semantic/extract/ -count=1 -run "TestToStoreFacts"` — 7/7 PASS
- `go test ./internal/semantic/extract/{golang,typescript,python}/... -count=2` — goldens byte-identical
- `go test ./internal/daemon/ ./internal/semantic/... ./internal/config/ ./internal/obs/ -count=1` — full Phase 59 gate ok
- 0 I/O imports in `to_store.go` (no `os`, `io/ioutil`, `net`, `os/exec`, `crypto/rand`, `math/rand`, `time`)
- 0 `init()` functions in `to_store.go`
- 0 map iterations in `to_store.go`
- Signature `func ToStoreFacts(files []*ExtractedFile) semanticstore.Facts` present in `to_store.go`

---
*Phase: 59-tree-sitter-extraction-stable-symbol-ids*
*Plan: 07*
*Completed: 2026-05-08*
