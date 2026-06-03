---
phase: 74-close-gap-wire-p1-tool-accessors-in-production-daemon
plan: "04"
subsystem: daemon
tags: [semantic, accessor, wiring, setters, P1, daemon, BLOCKER-1, BLOCKER-2]

requires:
  - phase: 74-02
    provides: Five D-01 store-backed adapter structs and factory constructors (symbolByNameAccessor, extractorRunAccessor, clusterMapAccessor, clusterMemberAccessor, clusterPageRankAccessor)
  - phase: 74-03
    provides: Two D-01a FOLD adapter structs and factory constructors (symbolEdgesAccessor, clusterMembershipAccessor)

provides:
  - setters block extended with 8 new b.skill.Set* calls closing BLOCKER-1 and BLOCKER-2
  - SetImpactLookup wired via integLookupAccessor (no adapter wrapper needed — integ.SemanticLookup satisfies ImpactLookupAccessor)
  - log line updated: "setters", 14 (was 8)
  - deferral comment for TypeChain/EdgeEvidence (Phase 75)

affects:
  - 74-05 (bootstrap test can now assert all 8 P1 accessors are non-nil after wiring)

tech-stack:
  added: []
  patterns:
    - "Single-block invariant (D-04): all Set* calls in existing if b.skill != nil block, no new helper"
    - "integLookupAccessor() return type (integ.SemanticLookup) satisfies semantic.ImpactLookupAccessor directly — no thin wrapper adapter needed (both require ExpandFrom + Status with identical signatures)"

key-files:
  created: []
  modified:
    - internal/daemon/semantic_wiring.go

key-decisions:
  - "integ.SemanticLookup satisfies semantic.ImpactLookupAccessor at compile time — both interfaces require ExpandFrom and Status with the same signature; no adapter struct needed"
  - "TypeChain (SetTypeChain) and EdgeEvidence (SetEdgeEvidence) deferred to Phase 75: schema v6 lacks tree_sitter_kind, tier, and evidence_kind columns"
  - "D-04 single-block invariant preserved: 8 new Set* calls added inline, no wireP1Accessors() helper created"

requirements-completed: [AUDIT-R-01, AUDIT-R-02, AUDIT-R-04]

duration: 5min
completed: 2026-06-03
---

# Phase 74 Plan 04: Wire P1 Set* Calls in Production Setters Block Summary

**Setters block at semantic_wiring.go:252-278 extended with 8 new b.skill.Set* calls; log line updated from "setters", 8 to "setters", 14; BLOCKER-1 (8 P1 accessors unwired) and BLOCKER-2 (SetExtractorRun missing) closed in the production daemon.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-06-03T13:50:00Z
- **Completed:** 2026-06-03T13:55:51Z
- **Tasks:** 1 (Task 1: Extend setters block — add 8 P1 Set* calls and update log line to 14)
- **Files modified:** 1

## Accomplishments

- Extended the `if b.skill != nil` setters block in `newSemanticBundle` with 8 new P1 Set* calls:
  - `b.skill.SetSymbolByName(b.symbolByNameAccessor())` — uses Plan 74-02 factory
  - `b.skill.SetExtractorRun(b.extractorRunAccessor())` — uses Plan 74-02 factory; closes BLOCKER-2
  - `b.skill.SetClusterMap(b.clusterMapAccessor())` — uses Plan 74-02 factory
  - `b.skill.SetClusterMember(b.clusterMemberAccessor())` — uses Plan 74-02 factory
  - `b.skill.SetClusterPageRank(b.clusterPageRankAccessor())` — uses Plan 74-02 factory
  - `b.skill.SetImpactLookup(b.integLookupAccessor())` — `integ.SemanticLookup` satisfies `ImpactLookupAccessor` directly
  - `b.skill.SetSymbolEdges(b.symbolEdgesAccessor())` — uses Plan 74-03 factory
  - `b.skill.SetClusterMembership(b.clusterMembershipAccessor())` — uses Plan 74-03 factory
- Updated log line from `"setters", 8` to `"setters", 14`
- Added deferral comment for TypeChain/EdgeEvidence (Phase 75)
- D-04 single-block invariant maintained: no new wireP1Accessors() helper, all calls inline

## Task Commits

1. **Task 1: Extend setters block with 8 P1 Set* calls; log line → "setters", 14** — `193ea5d6` (feat)

## Files Created/Modified

- `internal/daemon/semantic_wiring.go` — 8 new b.skill.Set* calls + updated log count + deferral comment

## Decisions Made

- `integ.SemanticLookup` satisfies `semantic.ImpactLookupAccessor` at compile time: both require `ExpandFrom(ctx, ws, sym integ.SymbolID, depth int) ([]integ.Impact, error)` and `Status(ctx, ws) (integ.SemanticStatus, error)` — no thin adapter wrapper needed (the PATTERNS.md adapter fallback was not required)
- TypeChain and EdgeEvidence deferred to Phase 75: schema v6 lacks the required columns (`tree_sitter_kind`, `tier`, `evidence_kind`)

## Deviations from Plan

None — plan executed exactly as written. The `integLookupAccessor()` return type satisfied `ImpactLookupAccessor` directly without requiring the fallback adapter struct described in the plan's action notes.

## Verification

All acceptance criteria verified:

- `go build ./internal/daemon/...` exits 0
- `go vet ./internal/daemon/...` exits 0
- `go vet -tags vet-nokernel2semantic ./internal/daemon/... ./internal/skill/semantic/...` exits 0
- `go vet -tags vet-noduckdb ./internal/daemon/... ./internal/skill/semantic/...` exits 0
- `go test -race -count=1 ./internal/daemon/...` exits 0 (8.671s)
- `grep -E '"setters", 14' internal/daemon/semantic_wiring.go` — 1 match
- `grep -E '"setters", 8' internal/daemon/semantic_wiring.go` — 0 matches
- 8 new b.skill.Set* calls confirmed present
- `grep "SetTypeChain\|SetEdgeEvidence" internal/daemon/semantic_wiring.go` — 0 production calls

## Audit Requirements Closed

| Requirement | Description | Status |
|-------------|-------------|--------|
| AUDIT-R-01 | BLOCKER-1: 8 P1 accessors wired in production daemon | CLOSED |
| AUDIT-R-02 | BLOCKER-2: SetExtractorRun called; FreshnessV2 can reach "current" | CLOSED |
| AUDIT-R-04 | Wiring block is the single source of truth (D-04) | CLOSED |

## Known Stubs

None — all 8 new Set* calls wire real production accessors with live store-backed implementations.

## Threat Model Coverage

| Threat ID | Mitigation Status |
|-----------|-------------------|
| T-74-04-01 | COVERED: `integ.SemanticLookup` satisfies `ImpactLookupAccessor` verified at compile time by `go build` — type compatibility confirmed, no adapter needed |
| T-74-04-SC | ACCEPTED: no new external packages added |

## Self-Check

- [x] `internal/daemon/semantic_wiring.go` modified and committed at 193ea5d6
- [x] `go build ./internal/daemon/...` exits 0
- [x] `go vet ./internal/daemon/...` exits 0
- [x] `go test -race -count=1 ./internal/daemon/...` exits 0
- [x] `grep -E '"setters", 14' internal/daemon/semantic_wiring.go` returns 1 match
- [x] `grep -E '"setters", 8' internal/daemon/semantic_wiring.go` returns 0 matches
- [x] 8 new b.skill.Set* calls confirmed (grep -c returns 8)
- [x] SetTypeChain/SetEdgeEvidence absent from production setters block

## Self-Check: PASSED

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes. Pure wiring addition to an existing daemon construction function.

---
*Phase: 74-close-gap-wire-p1-tool-accessors-in-production-daemon*
*Completed: 2026-06-03*
