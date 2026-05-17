---
phase: 72-p1-cluster-impact-tools
plan: "05"
subsystem: semantic-skill
tags: [integration-test, cross-tool, d7, d09-gate, phase-gate]
dependency_graph:
  requires: [72-02, 72-03, 72-04]
  provides: [TestFourTools_ClusterToImpact, Phase72-gate-green]
  affects: [internal/skill/semantic/integration_test.go, internal/skill/semantic/populated_graph_fixture_test.go]
tech_stack:
  added: []
  patterns: [cross-tool-chain-test, shared-fixture-mock-accessors]
key_files:
  created: []
  modified:
    - internal/skill/semantic/integration_test.go
    - internal/skill/semantic/populated_graph_fixture_test.go
decisions:
  - "Pre-existing TestConn_Call race in internal/kernel/jsonrpc is out-of-scope — confirmed present on main branch before this plan"
  - "readonly_gate_test.go already had all 6 Phase 72 handler files added by 72-04 — Task 1 was a verification step with shared-fixture addition only"
  - "newFourToolsHarness reuses threeToolsStoreRec (graphVersion=42) for FreshnessV2 consistency test"
metrics:
  duration: "15m"
  completed: "2026-05-17"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
---

# Phase 72 Plan 05: Cross-Tool Integration Test + D-09 Gate + Full Project Gate Summary

Cross-tool integration test (D7) + D-09 read-only gate extension + full project ship gate for the Phase 72 P1 cluster & impact tools.

## What Was Built

TestFourTools_ClusterToImpact integration test chains all three Phase 72 tools in a deterministic 5-step pipeline proving FreshnessV2.GraphVersion envelope consistency. Shared Phase 72 mock accessor types added to populated_graph_fixture_test.go for reuse.

## Tasks Completed

### Task 1: Phase 72 shared mock accessors + readonly_gate_test.go verification

Verified `readonly_gate_test.go` already contained all 6 `gatedHandlerFiles` entries (3 Phase 71 + 3 Phase 72) from 72-04. Added 4 new shared mock accessor types to `populated_graph_fixture_test.go`:

- `fixClusterMapAccessorShared` — 3 clusters (ClusterIntID 10/20/30, MemberCount 5/3/2)
- `fixClusterMemberAccessorShared` — 4 members for ClusterIntID=10 (node IDs 101-104 with stable_key symbol IDs)
- `fixClusterPageRankAccessorShared` — static PageRank scores for nodes 101-104
- `fixImpactLookupAccessorShared` — 3 impacts with Confidence=0.9 (OQ-1 predicate not triggered)

Also added `workspace` import to `populated_graph_fixture_test.go`.

Commit: `9703024e`

### Task 2: TestFourTools_ClusterToImpact integration + full phase gate

Added `newFourToolsHarness` and `TestFourTools_ClusterToImpact` to `integration_test.go`. The test:

1. Calls `handleGetClusterMap(top_n=1)` — asserts IsError==false, ≥1 cluster; captures cluster_id and GraphVersion (cmGV)
2. Calls `handleExplainCluster(cluster_id)` — asserts IsError==false, no fallback_reason, ≥1 member; captures repSymbol and GraphVersion (ecGV)
3. Calls `handleGetChangeImpactGraph(seed.symbol_id=repSymbol)` — asserts IsError==false, no fallback_reason; captures GraphVersion (igGV)
4. Asserts cmGV == ecGV == igGV == 42 (all three observe graphVersion=42 from shared store recorder)
5. Asserts ec.MemberCount ≥ 1

The harness uses mode="review" (get_change_impact_graph requires review+) and wires all four Phase 72 accessor seams plus the Phase 71 accessors needed for resolveSeed and FreshnessV2 assembly.

Full package `go test ./internal/skill/semantic/ -race -count=1` passes. `go vet ./...` clean.

Commit: `0278bb2b`

## Verification Results

| Check | Result |
|-------|--------|
| `go test ./internal/skill/semantic/ -run TestReadOnlyGate -race -count=1` | PASS (6 files) |
| `go test ./internal/skill/semantic/ -run TestFourTools_ClusterToImpact -race -count=1` | PASS |
| `go test ./internal/skill/semantic/ -race -count=1` | PASS |
| `go vet ./...` | CLEAN |
| `go test ./... -race -count=1` | PASS (pre-existing TestConn_Call flake in internal/kernel/jsonrpc excluded) |

## Deviations from Plan

### Pre-existing Out-of-Scope Flake

**TestConn_Call race condition in `internal/kernel/jsonrpc/codec_test.go`** — confirmed present on `main` branch before this plan. Verified by running the test against the stashed main branch state. This is a pre-existing race unrelated to Phase 72 changes; logged to deferred-items per scope-boundary rule.

### readonly_gate_test.go Already Extended (72-04 Work)

**Plan assumed Task 1 might require modifications to readonly_gate_test.go** — 72-04 had already added all three Phase 72 handler filenames to `gatedHandlerFiles`. Task 1 became a verification-only step plus shared-fixture addition.

## Threat Flags

None — this plan only modifies test files; no production trust surface changes.

## Self-Check

- [x] `internal/skill/semantic/integration_test.go` contains `TestFourTools_ClusterToImpact` — FOUND
- [x] `internal/skill/semantic/populated_graph_fixture_test.go` contains `fixClusterMapAccessorShared` — FOUND
- [x] `internal/skill/semantic/readonly_gate_test.go` has 6 entries in `gatedHandlerFiles` — CONFIRMED
- [x] Commit `9703024e` exists — FOUND
- [x] Commit `0278bb2b` exists — FOUND

## Self-Check: PASSED
