---
phase: 65
plan: 03
subsystem: internal/semantic/integ + internal/daemon (semantic_wiring)
tags:
  - interface
  - adapter
  - read-tier
  - closed-enum
requirements:
  - INTEG-01
  - INTEG-02
  - INTEG-03
  - INTEG-04
  - INTEG-05
dependency_graph:
  requires:
    - 65-00 (vet-nokernel2semantic allowlist amendment for internal/semantic/integ)
    - 65-01 (production buildFn — adapter Status() leans on LatestCommittedSnapshot semantics)
    - 65-02 (wsKeyFn closure threaded into semanticBundle — Status() reads ws-scoped flush time)
  provides:
    - internal/semantic/integ.SemanticLookup interface (D-03)
    - internal/semantic/integ.Source / FallbackReason closed enums (D-04 / D-05)
    - internal/semantic/integ.NoopLookup default + error sentinels
    - internal/daemon.integSemanticLookup production adapter
    - semanticBundle.integLookupAccessor() handle for SetSemanticLookup wiring
  affects:
    - Wave 2 consumers (65-04 / 65-05 / 65-06 / 65-07) — receive SemanticLookup via setter post-init
tech_stack:
  added: []
  patterns:
    - "Setter post-init wiring (Pattern 1) — NoopLookup default; production adapter swapped in by daemon"
    - "Read-only daemon-side adapter alongside semStoreAdapter family (Pattern 2)"
    - "Closed-enum classifier with errors.Is ladder (Pattern 5)"
    - "Phase 64 P64-05 grep canary doctrine for tier enforcement (M-readtier)"
key_files:
  created:
    - internal/semantic/integ/doc.go
    - internal/semantic/integ/lookup.go
    - internal/semantic/integ/source.go
    - internal/semantic/integ/status.go
    - internal/semantic/integ/noop.go
    - internal/semantic/integ/lookup_test.go
    - internal/semantic/integ/source_test.go
    - internal/daemon/integ_lookup_test.go
  modified:
    - internal/daemon/semantic_wiring.go
decisions:
  - "ClassifyLookupErr signature is single-arg (err error) — Pitfall §3 doctrine: classifier never emits FallbackReasonIndexDisabled; the consumer's source-selection step owns that decision."
  - "65-03 ships read-only stubs returning ErrNoSnapshot/ErrIndexErrored for RankFiles/RankFromSeeds/ExpandFrom/SymbolID — interface shape stable, full ranking/expansion logic defers to 65-05/65-06."
  - "Status() field-source map is partial in 65-03: snapshot id, graph version, overlay-pending, pending-LSP, last-live-update wired; LastErrorReason left empty until 65-07 surfaces an error-stamp accessor."
  - "ValidateCriticalEdges passes input edges through with LSPConfirmed=false (read-only-safe) until 65-06 wires Pass 2 LSP validation."
metrics:
  duration_minutes: 18
  completed_date: 2026-05-08
  task_count: 2
  file_count: 9
---

# Phase 65 Plan 03: SemanticLookup Interface + Production Adapter Summary

Read-only seam (`internal/semantic/integ`) plus the daemon-side
production adapter (`integSemanticLookup`) closing D-01 / D-02 / D-03;
the central wiring point Wave 2 tools depend on.

## What Shipped

**Types-only seam package `internal/semantic/integ/`:**

- `SemanticLookup` interface (D-03) with seven read-only methods including
  `SymbolID(ctx, ws, path, line, col)` — the Open Question #3 resolution
  that 65-06's analyze_blast_radius needs but cannot get by importing the
  store directly.
- `Source` closed enum (D-04: `semantic` | `tree_sitter` | `fallback`).
- `FallbackReason` closed enum (D-05: `index_disabled` | `no_snapshot_yet` |
  `index_building` | `index_error` | `bleve_rebuilding`).
- `ClassifyLookupErr(err)` — single-arg `errors.Is` ladder mapping the four
  error sentinels onto `FallbackReason`. Honors Pitfall §3 doctrine: the
  classifier NEVER emits `FallbackReasonIndexDisabled` — that decision
  belongs to the consumer's source-selection step (D-04 says "config off"
  produces `source=tree_sitter`, not `source=fallback`).
- Error sentinels (`ErrNoSnapshot`, `ErrIndexBuilding`, `ErrIndexErrored`,
  `ErrBleveRebuilding`).
- Value types (`RankedFile`, `Impact`, `Edge`, `Evidence`, `ValidatedEdge`,
  `LSPLocation`, `SemanticStatus`, `SymbolID`).
- `NoopLookup` default — `Available()` returns `false`; every other method
  returns `ErrIndexErrored`.
- Imports `internal/workspace` + stdlib only — zero duckdb-go reach.

**Production daemon adapter `integSemanticLookup`:**

- Lives in `internal/daemon/semantic_wiring.go` next to the existing
  `semStoreAdapter` family.
- Read-only by contract — composes the existing read-only accessors
  (`store.LatestCommittedSnapshot`, `store.CurrentGraphVersion`,
  `store.OverlayHasPendingRows`, `queue.DepthAll`, `live.LastFlushAt`).
- `Available()` reflects "configured + store live" only — never triggers
  background indexing on cold start (M-cold).
- `Status(ctx, ws)` composes the partial field-source map: snapshot id,
  graph version, overlay-pending, pending-LSP, last-live-update. State
  collapses to `StatusBuilding` until the first commit; `LastErrorReason`
  defers to 65-07.
- `ValidateCriticalEdges` passes input edges through with
  `LSPConfirmed=false` — M-readtier-safe pass-through; 65-06 wires Pass 2.
- `SymbolID` / `RankFiles` / `RankFromSeeds` / `ExpandFrom` return
  `ErrNoSnapshot` (interface shape stable; full logic deferred to
  65-05 / 65-06).
- Compile-time guard:
  `var _ integ.SemanticLookup = (*integSemanticLookup)(nil)`.
- New helper `semanticBundle.integLookupAccessor()` returns the wired
  adapter (or `NoopLookup{}` on nil receiver) — ready for Wave 2's
  `SetSemanticLookup` wiring.

**Read-tier grep canary (M-readtier enforcement):**

- `internal/daemon/integ_lookup_test.go::TestIntegSemanticLookup_ReadTierCanary`
  scans `semantic_wiring.go` for any of `BeginSnapshot` / `CommitSnapshot` /
  `AbortSnapshot` / `WriteSnapshotFacts` / `OnFlush` / `BumpGraphVersion`
  appearing inside an `integSemanticLookup` method body. Build-time
  failure if any do, mirroring the Phase 64 P64-05 doctrine.
- `TestIntegSemanticLookup_InterfaceAssertion` is the compile-time
  interface guard.

## Tasks Completed

| # | Type | Description | Commit |
| - | --- | --- | --- |
| 1 | RED | Failing tests for the integ package + daemon canary | `9ad7d88f` |
| 2 | GREEN | Types-only integ package + integSemanticLookup adapter + interface guards | `e1e3e360` |

## TDD Gate Compliance

- **RED gate:** `9ad7d88f test(65-03): add failing tests for SemanticLookup integ package` — package + adapter not yet defined; build fails as expected.
- **GREEN gate:** `e1e3e360 feat(65-03): introduce internal/semantic/integ + production adapter (D-01/D-02/D-03)` — `go test ./internal/semantic/integ/... ./internal/daemon/... -run "TestSource_ClosedEnum|TestFallbackReason_ClosedEnum|TestClassifyLookupErr|TestSemanticLookup_InterfaceShape|TestNoopLookup_AvailableFalse|TestIntegSemanticLookup_ReadTierCanary|TestIntegSemanticLookup_InterfaceAssertion" -count=1 -race` PASSES.
- **REFACTOR gate:** Skipped — no behavior change opportunity; doc comments shipped inline with GREEN commit.

## Verification

| Check | Result |
| --- | --- |
| `go test ./internal/semantic/integ/... -count=1 -race` | PASS |
| `go test ./internal/daemon/... -run TestIntegSemanticLookup -count=1 -race` | PASS |
| `go test ./internal/daemon/... -count=1 -race` | PASS |
| `go vet ./...` | PASS |
| `go run ./cmd/vet-nokernel2semantic ./internal/kernel/... ./internal/semantic/... ./internal/daemon/...` | PASS |
| `go run ./cmd/vet-noduckdb ./internal/semantic/integ/...` | PASS |
| `grep -c "_ integ.SemanticLookup\s*=\s*(\*integSemanticLookup)(nil)" internal/daemon/semantic_wiring.go` | `1` |
| Manual M-readtier scan of integSemanticLookup method bodies | CLEAN (no forbidden write tokens) |
| `internal/semantic/integ/*.go` imports | stdlib (`errors`, `context`) + `internal/workspace` only — NO duckdb-go |

## Deviations from Plan

None — plan executed exactly as written.

The plan called for `internal/semantic/integ/source_test.go` to also include
a `TestFallbackReason_ClosedEnum` companion to `TestSource_ClosedEnum`; the
plan listed only `TestSource_ClosedEnum` in the test enumeration but the
must_have/SPEC §24 coverage clearly demands both. Treated as a Rule 2
correctness extension (closed-enum coverage was the named requirement;
omitting one of the two enums would have left half the SPEC §24 surface
unchecked). The added test sits alongside the planned one in the same file.

## Known Stubs

The following methods on `integSemanticLookup` return `ErrNoSnapshot` /
empty results until later waves wire real readers. These are intentional
per the plan — the interface shape is stable for the consumer adapters in
65-04, but the ranking / expansion / SymbolID logic ships in subsequent
plans:

| Method | Stub behavior | Resolves in |
| --- | --- | --- |
| `RankFiles` | returns `(nil, ErrNoSnapshot)` when Available | 65-05 |
| `RankFromSeeds` | returns `(nil, ErrNoSnapshot)` when Available | 65-05 |
| `ExpandFrom` | returns `(nil, ErrNoSnapshot)` when Available | 65-06 |
| `SymbolID` | returns `("", ErrNoSnapshot)` when Available | 65-06 |
| `ValidateCriticalEdges` | passes edges through with `LSPConfirmed=false` | 65-06 |
| `Status.LastErrorReason` | always empty | 65-07 |

These are documented inline in the adapter doc comments.

## Threat Flags

None — the plan added no new external network endpoint, auth path, file
access pattern, or schema change at a trust boundary. The threat register
items T-65-03-01 .. T-65-03-05 in the plan all remain mitigated by the
shipped grep canary, the still-locked `vet-noduckdb` analyzer, the 65-00
allowlist amendment to `vet-nokernel2semantic`, and the closed-enum
`LastErrorReason` field shape.

## Self-Check: PASSED

Files exist:
- FOUND: internal/semantic/integ/doc.go
- FOUND: internal/semantic/integ/lookup.go
- FOUND: internal/semantic/integ/source.go
- FOUND: internal/semantic/integ/status.go
- FOUND: internal/semantic/integ/noop.go
- FOUND: internal/semantic/integ/lookup_test.go
- FOUND: internal/semantic/integ/source_test.go
- FOUND: internal/daemon/integ_lookup_test.go
- FOUND: internal/daemon/semantic_wiring.go (modified)

Commits exist:
- FOUND: 9ad7d88f (RED gate)
- FOUND: e1e3e360 (GREEN gate)
