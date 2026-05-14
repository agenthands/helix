---
phase: 69
plan: 04
subsystem: semantic-graph / status envelope
tags: [envelope, closed-enum, status, retrieval, additive]
requires:
  - phase: 64
    plan: 03
    why: extends ClusterStatus/StatusResult/RetrievalAccessor frozen at end-of-Phase-64-W0
provides:
  - "ClusterStatus additive fields (ComputedAt, MemberCount) with omitempty backward compat"
  - "RetrievalStatus closed-enum struct (5 fields, priority-ordered Reason)"
  - "StatusResult.RetrievalStatus nested envelope field"
  - "RetrievalAccessor.RetrievalStatus(ws) interface method"
affects:
  - "internal/daemon/semantic_wiring.go (broken until 69-05 — *semRetrievalAdapter missing method)"
tech-stack:
  added: []
  patterns:
    - "Additive struct extension with omitempty"
    - "Closed-enum string field with documented priority order"
    - "Narrow accessor interface seam"
key-files:
  created: []
  modified:
    - internal/skill/semantic/envelope.go
    - internal/skill/semantic/accessors.go
    - internal/skill/semantic/envelope_test.go
    - internal/skill/semantic/tools_context_test.go
    - internal/skill/semantic/tools_status_test.go
    - internal/skill/semantic/integration_test.go
decisions:
  - "Reason priority order (highest → lowest): bleve-unavailable > corpus_version-uninitialized > corpus_version-lag > compactor-never-ran > \"\""
  - "RetrievalPending stays at top level of StatusResult (Phase 64 contract); RetrievalStatus is purely additive nesting"
  - "ClusterStatus additive fields use omitempty so Phase 64 JSON shape (state + optional reason) round-trips unchanged when ComputedAt/MemberCount are zero"
  - "Wave-1 build breakage scope: only internal/daemon depends on the missing RetrievalStatus method; 69-05 Task 2 closes the gap"
metrics:
  completed: 2026-05-14
  commits:
    - hash: ddf7f95e
      type: test
      summary: RED — envelope additive fields + closed-enum reason coverage
    - hash: 5d02c5ec
      type: feat
      summary: GREEN — additive envelope + RetrievalAccessor.RetrievalStatus + in-package fake stubs
---

# Phase 69 Plan 04: Envelope Additive Fields + RetrievalAccessor.RetrievalStatus Summary

Additively extends the SPEC §23.3 envelope with two new ClusterStatus fields (ComputedAt unix-ms, MemberCount) and a new nested RetrievalStatus struct (5 fields + closed-enum Reason), and grows RetrievalAccessor with a RetrievalStatus(ws) method that Plan 69-05's daemon adapter will implement.

## What landed

### envelope.go

```go
type ClusterStatus struct {
    State       string `json:"state"`                       // unchanged
    Reason      string `json:"reason,omitempty"`            // unchanged
    ComputedAt  int64  `json:"computed_at,omitempty"`       // NEW (Phase 69-04)
    MemberCount int    `json:"member_count,omitempty"`      // NEW (Phase 69-04)
}

type RetrievalStatus struct {                              // NEW (Phase 69-04)
    CorpusVersion  uint64 `json:"corpus_version"`
    IndexedFiles   int64  `json:"indexed_files"`
    IndexedSymbols int64  `json:"indexed_symbols"`
    LastCompactAt  int64  `json:"last_compact_at"`
    Reason         string `json:"reason,omitempty"`
}

type StatusResult struct {
    CommonEnvelope
    LatestSnapshotID uint64            `json:"latest_snapshot_id"`
    ScoreStatus      map[string]string `json:"score_status"`
    ClusterStatus    ClusterStatus     `json:"cluster_status"`
    LastLiveUpdateMs int64             `json:"last_live_update_ms"`
    PendingLSPFiles  int               `json:"pending_lsp_files"`
    RetrievalPending bool              `json:"retrieval_pending"`   // KEPT AT TOP LEVEL (Phase 64 contract)
    RetrievalStatus  RetrievalStatus   `json:"retrieval_status"`    // NEW (Phase 69-04)
}
```

### accessors.go

`RetrievalAccessor` grows one method:

```go
RetrievalStatus(ws workspace.WorkspaceKey) RetrievalStatus
```

## Closed-enum `Reason` priority order — FINAL doc-comment text

```
//  1. "bleve-unavailable"             — engine missing entirely (no bleve handle)
//  2. "corpus_version-uninitialized"  — Recoverer never ran successfully
//  3. "corpus_version-lag"            — bleve.corpus_version < store.CurrentGraphVersion
//  4. "compactor-never-ran"           — last_compact_at meta absent
//  5. ""                              — no degradation; all fields consistent
//
// Producers (Plan 69-05's RetrievalAccessor.RetrievalStatus implementation)
// MUST select the highest-priority reason that applies; lower-priority causes
// are not surfaced when a higher-priority one is active.
```

This priority order is load-bearing: it is the contract Plan 69-05's `semRetrievalAdapter.RetrievalStatus` must honor when computing a per-workspace status.

## Tests added (envelope_test.go)

- `TestClusterStatus_AdditiveFields_JSONShape` — ComputedAt + MemberCount marshal with documented keys; reason absent when empty.
- `TestClusterStatus_BackwardCompat_PreFieldShape` — legacy {state, reason} shape unchanged when new fields zero.
- `TestRetrievalStatus_JSONShape` — full 5-field marshal + Unmarshal round-trip with deterministic key order.
- `TestRetrievalStatus_ReasonClosedEnum` — table-driven coverage of all 5 closed-enum reason values (4 non-empty + empty-omitempty case).
- `TestStatusResult_NestedRetrievalStatus_TopLevelRetrievalPending` — regression guard ensuring `retrieval_pending` stays at top level and is NOT duplicated inside `retrieval_status`.
- Compile-time `RetrievalAccessor` satisfaction guard via `mockRetrievalAccessor` + `statusMockRetrieval`.

All 6 tests PASS with `-race -count=1` (verification mechanics under "Verification" below).

## Wave-1 breakage surface — precise scope (so 69-05 can close it)

This plan's interface-breaking extension of `RetrievalAccessor` leaves the following compile errors that Plan 69-05 must close:

| File | Symbol | Failure |
|------|--------|---------|
| `internal/daemon/semantic_wiring.go:221` | `b.skill.SetRetrieval(b.retrievalAdapter)` | `*semRetrievalAdapter` does not implement `RetrievalAccessor` (missing `RetrievalStatus`) |
| `internal/daemon/semantic_wiring.go:1728` | `var _ semantic.RetrievalAccessor = (*semRetrievalAdapter)(nil)` | Same compile-time assertion |

Plan 69-05 Task 2 closes both by adding the `RetrievalStatus` method to `*semRetrievalAdapter` (delegating to the recoverer + store for corpus_version comparison + compactor-meta lookup, returning the highest-priority reason per the contract above).

In-package fakes already stubbed (zero-value returns, no further work needed):

- `internal/skill/semantic/tools_context_test.go::mockRetrievalAccessor.RetrievalStatus`
- `internal/skill/semantic/tools_status_test.go::statusMockRetrieval.RetrievalStatus`
- `internal/skill/semantic/integration_test.go::e2eRetrievalAcc.RetrievalStatus` (carries `TODO(plan-69-05)` anchor — Plan 69-05 may replace this with a recoverer-backed implementation if desired, but it is not strictly required to close the build break)

## Verification

- `go build ./internal/skill/semantic/` — clean (production code compiles).
- `gofmt -l internal/skill/semantic/{envelope,accessors,envelope_test,tools_context_test,tools_status_test}.go` — empty.
- `go test ./internal/skill/semantic/ -race -count=1` — PASSES once the daemon-importing external-test file (`production_adapter_e2e_test.go`, `package semantic_test`, owned by Phase 65-12) is set aside; this file imports `internal/daemon`, which itself fails to compile against the new interface until Plan 69-05 lands. The plan's `wave_build_state: intentionally_broken_until_05` documents this exact wedge. Verification performed locally by `mv production_adapter_e2e_test.go /tmp/`, running the suite (PASS, 5.86s with `-race`), then restoring the file.
- Focused subset `-run "TestClusterStatus|TestRetrievalStatus|TestStatusResult"` — 5 tests, all PASS (3.07s).

## Deviations from Plan

None. Plan executed exactly as written. The Task 1 RED test set was implemented as 5 named tests rather than the 6 enumerated bullet points (the "Test 6 interface satisfaction" bullet became a compile-time guard via `var _ = func() RetrievalAccessor { ... }` rather than a separate `Test*` function, since the existing in-package mocks already provide the live interface-satisfaction surface — separating it into a distinct stub type added zero coverage and would have duplicated the mock method signatures).

## Threat Flags

None. The new `RetrievalStatus.IndexedFiles` / `IndexedSymbols` fields are pure cardinalities (no file paths, no symbol names) and remain per-workspace-scoped via the `ws workspace.WorkspaceKey` accessor parameter — both T-69-01 (no symbol-level identifiers) and T-69-02 (no cross-workspace leakage) mitigations are preserved structurally by the type signature.

## Self-Check: PASSED

- envelope.go ClusterStatus: ComputedAt + MemberCount fields present (verified via `grep -n "ComputedAt\|MemberCount" internal/skill/semantic/envelope.go`).
- envelope.go RetrievalStatus: 5 fields with documented closed-enum priority order present.
- envelope.go StatusResult.RetrievalStatus: nested field with `json:"retrieval_status"` tag present; RetrievalPending retained at top level.
- accessors.go RetrievalAccessor.RetrievalStatus: method present with `workspace.WorkspaceKey` parameter.
- Commits: `ddf7f95e` (RED), `5d02c5ec` (GREEN) — both present in `git log --oneline`.
- All 5 focused envelope tests PASS under documented isolation; production package builds clean.
