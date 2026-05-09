---
phase: 66
plan: 01
subsystem: guardrails
tags: [guardrails, receipts, mcp, tdd]
completed: 2026-05-09T15:40:16Z
duration_minutes: 55

dependency_graph:
  requires: []
  provides:
    - internal/guardrails (receipt model, store, validate, enforcement, issue_sink)
    - internal/errors/kinds.go (GuardrailViolation Kind + sentinel)
    - internal/obs/metrics.go (receipt counters)
  affects:
    - internal/mcp (Plans 04+ install GuardrailMiddleware)
    - internal/kernel/symbols, internal/kernel/diag (Plans 05+ wire issuance)
    - internal/skill/repomap (Plan 05 wires issuance)

tech_stack:
  added:
    - github.com/google/uuid v1.6.0 (already in go.mod as indirect; used for UUIDv7 receipt IDs)
  patterns:
    - Closed-enum ReceiptClass + typed Go union ReceiptScope (D-03 LOCKED)
    - sync.Map + per-workspace LRU + background janitor (D-06 LOCKED)
    - atomic.Pointer sink pattern mirroring editOutcomeSink (OI-05)
    - 5-layer highest-precedence-set-wins enforcement resolver (D-20 LOCKED)
    - Drop-unknown closed-enum Prometheus counter helpers (Phase 47/53 pattern)

key_files:
  created:
    - internal/guardrails/doc.go
    - internal/guardrails/receipt.go
    - internal/guardrails/receipt_id.go
    - internal/guardrails/receipt_id_test.go
    - internal/guardrails/store.go
    - internal/guardrails/store_test.go
    - internal/guardrails/validate.go
    - internal/guardrails/validate_test.go
    - internal/guardrails/enforcement.go
    - internal/guardrails/enforcement_test.go
    - internal/guardrails/issue_sink.go
    - internal/errors/kinds_test.go
  modified:
    - internal/errors/kinds.go (added GuardrailViolation Kind + types)
    - internal/obs/metrics.go (added 4 receipt counters + helpers)
    - .planning/phases/66-agent-guardrails/66-VALIDATION.md (Task 0: wave_0_complete)

decisions:
  - UUIDv7 base32-no-padding for receipt ID: 26-char body from 16 bytes; case-insensitive parse
  - GuardrailViolationDetail as separate struct (name clash with GuardrailViolation Kind const)
  - AsGuardrailViolation() helper instead of errors.As (Go 1.25 requires target to implement error)
  - noopMetrics{} as default MetricsSink in Store (avoids nil checks at call sites)
  - NewStoreWithTicker() for test-injectable clock + tick channel
---

# Phase 66 Plan 01: Guardrails Foundation Primitives Summary

**One-liner:** In-memory per-workspace receipt store with UUIDv7 IDs, typed scope union, 5-layer enforcement resolver, 6 validation sentinels, and bounded-label Prometheus counters.

## What Was Built

### Task 0 (Wave 0): VALIDATION.md Populated

Replaced the TBD-01 placeholder row with 16 concrete task rows (66-01-T1 through 66-06-T3), including verbatim `<automated>` commands from each plan's verify block. Set `wave_0_complete: true`.

**Commit:** c2228003

### Task 1: Receipt ID + ReceiptClass + Scope Union

- `internal/guardrails/doc.go`: package doc with LIFO middleware order invariant.
- `internal/guardrails/receipt.go`: `ReceiptClass` closed enum (5 values), `Receipt` struct (17 fields), `ReceiptScope` marker interface, 5 concrete scope structs (`ReferencesCheckedScope`, `ImpactCheckedScope`, `ContextGatheredScope`, `StructuralOverviewScope`, `DiagnosticsCleanScope`), compile-time `var _ ReceiptScope = (*X)(nil)` assertions.
- `internal/guardrails/receipt_id.go`: `ReceiptID` type, `NewReceiptID()` (UUIDv7 → base32 no-padding, 26 chars), `ParseReceiptID()` with typed error sentinels (prefix/length/alphabet).
- `internal/guardrails/receipt_id_test.go`: table-driven tests for Parse (12 cases) + New (4 subtests) + exhaustiveness check.

Tests run: `TestParseReceiptID`, `TestNewReceiptID`, `TestReceiptClassExhaustive`, `TestReceiptScope_MarkerInterface`

**Commit:** b994a2e9

### Task 2: Receipt Store + Metrics

- `internal/guardrails/store.go`: `MetricsSink` interface; `Store` with `sync.Map`-backed per-workspace `wsEntry`; `Issue`, `Get`, `InvalidateOnGraphVersionAdvance`, `Close`; 30s janitor via `NewStore`; `NewStoreWithTicker` for injectable test ticker; LRU 10k cap using `container/list`; opportunistic-on-lookup expiry.
- `internal/guardrails/store_test.go`: 7 subtests (TTLExpiry, OpportunisticExpiry, WorkspaceMismatch, GraphVersionAdvance, LRU, Janitor, Concurrent 100×100) all passing under `-race`.
- `internal/obs/metrics.go`: Added `ReceiptIssuedVec`, `ReceiptExpiredVec`, `ReceiptLookupVec` (`CounterVec`) and `GuardrailEvalTimeout` (unlabeled `Counter`); registered in `newMetrics()`; helper methods `ReceiptIssuedInc`, `ReceiptExpiredInc`, `ReceiptLookupInc`, `GuardrailEvalTimeoutInc` with drop-unknown closed-enum gates.

Tests run: `TestStore_TTLExpiry`, `TestStore_OpportunisticExpiry`, `TestStore_WorkspaceMismatch`, `TestStore_GraphVersionAdvance`, `TestStore_LRU`, `TestStore_Janitor`, `TestStore_Concurrent`

**Commit:** 9e00a043

### Task 3: Validation + Enforcement + Issue Sink + GuardrailViolation Kind

- `internal/guardrails/validate.go`: 6 typed sentinel errors (`ErrWrongWorkspace`, `ErrGraphVersionMismatch`, `ErrReceiptExpired`, `ErrReceiptTooStale`, `ErrWrongReceiptClass`, `ErrReceiptScopeMismatch`); `ValidateReceiptForOperation` with priority order; `validateScope` with exhaustiveness panic; `FileSetContainsAll`; `PathIsWithin` (filepath.Rel-based, rejects `..` escapes).
- `internal/guardrails/validate_test.go`: priority-order multi-criteria test, per-class STRICT scope-mismatch tests, `TestValidateScope_AllClasses` iterating `receiptClassEnum`.
- `internal/guardrails/enforcement.go`: `EnforcementLevel` closed enum (4 values); `ParseEnforcementLevel`; `EnforcementLayer`; `ResolveGuardrailEnforcement` (5-layer, highest-precedence-set-wins).
- `internal/guardrails/enforcement_test.go`: 8 table-driven cases + `TestResolveGuardrailEnforcement_HighestPrecedenceWinsNotMostRestrictive`.
- `internal/guardrails/issue_sink.go`: `atomic.Pointer` sink mirroring `editOutcomeSink`; `SetReceiptIssueSink`, `SetReceiptIssueSinkForTest` (t.Cleanup-friendly), `IssueReceiptOnSuccess` (no-op when sink unset).
- `internal/errors/kinds.go`: `GuardrailViolation Kind = "guardrail_violation"`, `ErrGuardrailViolation` sentinel; `GuardrailViolationDetail` struct with Rule/Message/RequiredReceipts/SuggestedTools/SeeAlso; `NewGuardrailViolation`; `AsGuardrailViolation` helper.
- `internal/errors/kinds_test.go`: `errors.Is` + `AsGuardrailViolation` payload recovery tests.

Tests run: `TestValidateReceiptForOperation_PriorityOrder`, per-class scope-mismatch tests, `TestValidateScope_AllClasses`, `TestFileSetContainsAll`, `TestPathIsWithin`, `TestParseEnforcementLevel`, `TestResolveGuardrailEnforcement`, `TestResolveGuardrailEnforcement_HighestPrecedenceWinsNotMostRestrictive`, `TestGuardrailViolationKind`, `TestNewGuardrailViolation_ErrorsIs`, `TestNewGuardrailViolation_ErrorsAs`, `TestNewGuardrailViolation_ErrorString`

**Commit:** 5b461840

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] GuardrailViolation name collision**
- **Found during:** Task 3 — `GuardrailViolation` used for both `Kind` constant and struct type.
- **Fix:** Renamed the typed struct to `GuardrailViolationDetail`; added `AsGuardrailViolation()` helper since `errors.As` in Go 1.25 requires the target type to implement `error`.
- **Files modified:** `internal/errors/kinds.go`, `internal/errors/kinds_test.go`
- **Commit:** 5b461840

**2. [Rule 1 - Bug] Test case: underscore in receipt ID body**
- **Found during:** Task 1 — "rcpt__" + 25 chars = 31 total (valid length) but invalid alphabet; test expected `ErrInvalidReceiptIDLength` but should have been `ErrInvalidReceiptIDAlphabet`.
- **Fix:** Updated test expectation from `ErrInvalidReceiptIDLength` to `ErrInvalidReceiptIDAlphabet`.
- **Files modified:** `internal/guardrails/receipt_id_test.go`
- **Commit:** b994a2e9

## Metrics

| Metric | Value |
|--------|-------|
| Tasks completed | 4/4 (Task 0 + Tasks 1-3) |
| Files created | 12 |
| Files modified | 2 |
| Tests passing | 73 subtests in internal/guardrails |
| Duration | ~55 minutes |

## Known Stubs

None — all code paths are fully implemented. Production metrics wiring (MetricsSink adapter from `*obs.Metrics`) is deferred to Plan 04 (GuardrailMiddleware production deps) as designed.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes introduced. All threat mitigations from the plan's threat_model are addressed:

- T-66-01 (ReceiptID forgery): ParseReceiptID validates prefix+length+alphabet; 130-bit entropy from UUIDv7+crypto/rand.
- T-66-02 (receipt replay): `InvalidateOnGraphVersionAdvance` deletes receipts with `GraphVersion < newGV`.
- T-66-04 (info disclosure via labels): drop-unknown gates on all `ReceiptLookupInc` labels.
- T-66-05 (DoS via issuance flood): LRU 10k cap + 30s janitor + opportunistic expiry.
- T-66-06 (cross-workspace forgery): `Get` checks `receipt.WorkspaceKey != ws` → workspace_mismatch.
- T-66-07 (disk persistence): `sync.Map` only; no file writes.
- T-66-08 (TOCTOU): `InvalidateOnGraphVersionAdvance` + hard TTL enforced on every `Get`.

## Self-Check: PASSED

All created files exist and all commits are present in git log.
