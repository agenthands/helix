---
status: complete
phase: 63-compaction-retention
source:
  - .planning/phases/63-compaction-retention/63-01-SUMMARY.md
  - .planning/phases/63-compaction-retention/63-02-SUMMARY.md
started: 2026-05-07T16:43:37Z
updated: 2026-05-07T16:50:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test (build + vet)
expected: go build ./cmd/helix + go vet ./... + cmd/vet-compact-uses-store ./internal/... ./cmd/... + cmd/vet-noduckdb on compact pkg all exit 0.
result: pass
evidence: |
  go build ./cmd/helix → exit 0 (only Swift binding macro-redefined warning, pre-existing)
  go vet ./... → exit 0 (warnings in tmp/ test fixtures only, not in source tree)
  go run ./cmd/vet-compact-uses-store ./internal/... ./cmd/... → exit 0 (compact→store boundary clean)
  go run ./cmd/vet-noduckdb ./internal/semantic/compact/... → exit 0

### 2. Snapshot-write API tests pass (P63-01)
expected: TestBeginSnapshot|TestSnapshot_|TestFakeCompactor_ → ok
result: pass
evidence: go test ./internal/semantic/store/ -run 'TestBeginSnapshot|TestSnapshot_|TestFakeCompactor_' -count=1 -timeout 120s → exit 0

### 3. Compaction gate + compactor unit tests pass (P63-02)
expected: TestGate_|TestCompactor_ → ok
result: pass
evidence: go test ./internal/semantic/compact/ -run 'TestGate_|TestCompactor_' -count=1 -timeout 120s → exit 0

### 4. CAS interleave property test passes under -race
expected: TestCAS_ -race → ok (rows committed at write_epoch > captured survive ClearOverlayLE)
result: pass
evidence: go test ./internal/semantic/compact/ -run TestCAS_ -count=1 -race -timeout 60s → exit 0

### 5. Migration004 lands; accessors round-trip
expected: TestApplyMigration004|TestStore_Overlay|TestStore_Vacuum|TestStore_Checkpoint|TestStore_UpdateLastVacuumAt → ok; CurrentSchemaVersion at least 4 in migrations_types.go
result: pass
evidence: |
  go test ./internal/semantic/store/ -run 'TestApplyMigration004|TestStore_Overlay|TestStore_Vacuum|TestStore_Checkpoint|TestStore_UpdateLastVacuumAt' -count=1 → exit 0
  CurrentSchemaVersion = 5 (a post-Phase-63 migration005 has since been added; Phase 63's migration004 still lands as designed)
note: |
  Doc drift only — 63-02-SUMMARY.md states "CurrentSchemaVersion = 4" but a later
  migration has since incremented it to 5. Migration004 itself is intact.

### 6. Coalescer / lspenrich / scheduler / kernel accessor tests pass
expected: coalescer + lspenrich + graph + kernel test suites → ok
result: pass
evidence: go test ./internal/semantic/live/coalescer/ ./internal/semantic/lspenrich/ ./internal/semantic/graph/ ./internal/kernel/ -count=1 -timeout 180s → exit 0

### 7. Closed-enum metrics cardinality holds
expected: TestSemanticCompaction|TestSemanticVacuum cardinality tests → ok
result: pass
evidence: go test ./internal/obs/ -run 'TestSemanticCompaction|TestSemanticVacuum' -count=1 -timeout 60s → exit 0

### 8. Maintenance config defaults present
expected: TestLoad_MaintenanceDefaults → ok
result: pass
evidence: go test ./internal/config/ -run TestLoad_MaintenanceDefaults -count=1 → exit 0

### 9. Compact bundle wired into daemon (no regression)
expected: daemon test suite green; daemon.go references newCompactBundle and registers compact field
result: pass
evidence: |
  go test ./internal/daemon/ -count=1 -timeout 180s → exit 0
  internal/daemon/daemon.go:169 — "compact *compactBundle" struct field
  internal/daemon/daemon.go:389,395 — compactBndl declaration + newCompactBundle(...) call

### 10. Daemon cold boot wires compact bundle live
expected: helix --serve boots cleanly, compact bundle constructed with expected config, post-flush hook wired, gRPC workspace activation succeeds.
result: pass
evidence: |
  Fresh ./helix --serve started against clean socket; daemon log shows:
    "compact bundle constructed" compact_after_idle_ms=5000
      lsp_compaction_max_wait_ms=3000 max_overlay_rows=4000
      vacuum_enabled=false vacuum_interval=168h0m0s
    "compactor post-flush hook wired to live service"
    "workspace activated" component=kernel root=<helix> languages=[go]
    "workspace activated via gRPC" path=<helix>
note: |
  helix status --json returns {"workspaces":[]} — a pre-Phase-63 quirk where
  the status CLI reads a separate workspace registry from the live in-process
  activation registry. Not a Phase 63 regression. Daemon-side activation
  itself is healthy.

## Summary

total: 10
passed: 10
issues: 0
pending: 0
skipped: 0

## Gaps

[none — all tests passed]

## Notes

Phase 63 ships daemon-internal compaction/retention infrastructure with no MCP/network
surface. UAT was driven entirely via automated technical checks per the project's
"UAT must automate daemon/MCP checks" rule — no user input was required.

Two minor doc-drift observations surfaced (not regressions):
- 63-02-SUMMARY.md says "CurrentSchemaVersion = 4"; reality is now 5 (later migration).
- helix status CLI reports empty workspaces despite live activation — pre-existing.
