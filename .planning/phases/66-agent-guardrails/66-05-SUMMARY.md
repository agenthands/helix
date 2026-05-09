---
phase: "66"
plan: "05"
subsystem: guardrails
tags: [guardrails, receipts, metrics, issuance, mcp-tools]
dependency_graph:
  requires: [66-01, 66-02, 66-03, 66-04]
  provides: [receipt-issuance-wiring]
  affects:
    - internal/kernel/symbols/tools.go
    - internal/kernel/diag/tools.go
    - internal/kernel/edit/tools.go
    - internal/skill/repomap/skill.go
    - internal/skill/semantic/tools_context.go
    - internal/kernel/fileops/tools.go
    - internal/kernel/fileops/write.go
    - internal/mcp/integ/
tech_stack:
  added: []
  patterns: [IssueReceiptOnSuccess-atomic-sink, SetReceiptIssueSinkForTest-cleanup, obs.Noop-prometheus-registry]
key_files:
  created:
    - internal/kernel/symbols/receipt_issuance_test.go
    - internal/kernel/edit/receipt_issuance_test.go
    - internal/mcp/integ/doc.go
    - internal/mcp/integ/receipt_issuance_smoke_test.go
  modified:
    - internal/kernel/symbols/tools.go
    - internal/kernel/diag/tools.go
    - internal/kernel/edit/tools.go
    - internal/skill/repomap/skill.go
    - internal/skill/semantic/tools_context.go
    - internal/kernel/fileops/tools.go
    - internal/kernel/fileops/write.go
decisions:
  - "IssueReceiptOnSuccess called on explicit success path only — never in defer blocks (Pitfall 4)"
  - "DiagnosticsClean receipt gated on errorCount == 0 in tool handler, not in IssueReceiptOnSuccess itself"
  - "run_diagnostics does not exist as an MCP tool; plan covered 7 of stated 8 tools"
  - "computeContextTaskHash helper added to repomap/skill.go; computeSemanticTaskHash to tools_context.go"
  - "D-08 Receipts field added to ReplaceBodyArgs, RenameSymbolArgs, SafeDeleteArgs, ReplaceInFileArgs, FuzzyEditArgs, DeleteFileArgs"
metrics:
  duration: "~3 hours (cross-session)"
  completed: "2026-05-09"
  tasks_completed: 3
  tasks_total: 3
  files_created: 4
  files_modified: 7
---

# Phase 66 Plan 05: Receipt Issuance Wiring Summary

Receipt issuance wired into 7 read-side MCP tool handlers via `IssueReceiptOnSuccess`, and uniform `Receipts []ReceiptID` field added to 6 destructive tool arg structs, completing the guardrails loop end-to-end.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Kernel tool issuance (symbols, diag, edit) | `2c420016` | `symbols/tools.go`, `diag/tools.go`, `edit/tools.go` + unit tests |
| 2 | Skill tool issuance + destructive args Receipts field | `6dfb02e6` | `repomap/skill.go`, `tools_context.go`, `fileops/tools.go`, `fileops/write.go` |
| 3 | Integration smoke test | `b567935f` | `internal/mcp/integ/` (doc.go + test) |

## What Was Built

### Task 1: Kernel tool receipt issuance

- `find_references`: issues `ClassReferencesChecked` with `ReferencesCheckedScope` containing ref count and file path
- `analyze_blast_radius`: issues `ClassImpactChecked` in both TreeSitter/fallback arm and semantic arm; semantic arm populates `SymbolID`, `IncludedCallers: true`, `IncludedTypes: true`
- `get_diagnostics`: tracks `errorCount` and `warningCount` per LSP diagnostic; issues `ClassDiagnosticsClean` only when `errorCount == 0` (T-66-25 gate)
- `verify_edit`: issues `ClassDiagnosticsClean` only when `!result.HasErrors`
- Unit tests: `symbols/receipt_issuance_test.go` and `edit/receipt_issuance_test.go` cover success + no-issuance-on-error paths

### Task 2: Skill tool issuance + D-08 Receipts field

- `get_repo_map`: issues `ClassStructuralOverview` with `StructuralOverviewScope`
- `get_context`: issues `ClassContextGathered` with sha256-derived `TaskHash` (first 16 hex chars)
- `get_semantic_context`: issues `ClassContextGathered` using `context.Background()` (handler uses local ctx var)
- D-08 `Receipts []guardrails.ReceiptID` added to: `ReplaceBodyArgs`, `RenameSymbolArgs`, `SafeDeleteArgs` (edit), `ReplaceInFileArgs`, `FuzzyEditArgs` (fileops), `DeleteFileArgs` (fileops/write)

### Task 3: Integration smoke test

- New `internal/mcp/integ` package with `//go:build integration` tag
- `TestReceiptIssuanceSmoke_AllEightToolsIncrementCounter` installs a `SetReceiptIssueSinkForTest` closure backed by a real `*guardrails.Store` with `obs.Noop` Prometheus metrics
- Asserts `helix_receipt_issued_total{class}` increments by exactly 1 for each of 7 tools
- Asserts negative gate: counter unchanged when `ErrorCount > 0` gate intentionally skips issuance
- Asserts 5 distinct class labels exercised

## Deviations from Plan

### Auto-noted Deviations

**1. [Plan Deviation] run_diagnostics does not exist as an MCP tool**
- **Found during:** Task 1
- **Issue:** Plan states 8 issuing tools including `run_diagnostics`. Searching the codebase found `run_diagnostics` only in `internal/phasegraph/pipelines/eval.go` (not a registered MCP tool). No `registerRunDiagnostics` function exists anywhere.
- **Resolution:** Proceeded with the 7 existing MCP tools. Noted in smoke test comments. Counter for plan's stated 8 tools: 7 actually wired.
- **Impact:** None on correctness — `run_diagnostics` is not an agent-callable tool in production.

**2. [Infrastructure] Worktree missing Plans 01-04 code**
- **Found during:** Plan start (prior session)
- **Issue:** Worktree was at commit `d60e40d3` (Phase 62 baseline); `internal/guardrails/` package and all Phase 66 plans 01-04 infrastructure were absent.
- **Fix:** `git merge 24f729511bbb6fadae5e83d5278b03c5182699c1` (fast-forward, no conflicts).
- **Commit:** (pre-task infrastructure fix)

## Pre-existing Test Failures (out of scope)

The following test failures were present before this plan and are not caused by these changes:

- `internal/kernel/jsonrpc/TestConn_Call`: data race in jsonrpc codec test
- `test/bench/TestBenchToolsManifestMatchesRegistry`: expects 43 tools, registry has 47 (manifest out of date)
- `test/bench/TestToolDescriptionsGoldenFile`: golden file mismatch
- `test/integration/TestEdit_JavaFixture/replace_body`: data race in Java integration test

All packages directly modified by this plan pass with `go test -race -count=1`.

## Threat Flags

None. This plan only adds read-only issuance calls on success paths and adds `omitempty` JSON fields to arg structs. No new network endpoints, auth paths, or trust boundary crossings.

## Self-Check: PASSED

- `internal/mcp/integ/doc.go`: FOUND
- `internal/mcp/integ/receipt_issuance_smoke_test.go`: FOUND
- `internal/kernel/symbols/receipt_issuance_test.go`: FOUND
- `internal/kernel/edit/receipt_issuance_test.go`: FOUND
- Commit `2c420016`: FOUND
- Commit `6dfb02e6`: FOUND
- Commit `b567935f`: FOUND
- `go test -tags=integration ./internal/mcp/integ/ -run TestReceiptIssuanceSmoke_AllEightToolsIncrementCounter`: PASS
- `go vet ./...`: PASS (only pre-existing C macro warning)
