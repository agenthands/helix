---
phase: 65
plan: 00
subsystem: lint/architecture-vet
tags:
  - vet
  - architecture
  - allowlist
  - tdd
dependency_graph:
  requires:
    - internal/lint/nokernel2semantic/analyzer.go (Phase 60 LIVE-07 #1 baseline)
  provides:
    - "internal/semantic/integ allowlist seam — kernel packages may now import this exact path (or sub-packages) without tripping make vet"
    - "TestAnalyzer_RejectsKernelImportingSemanticIntegLookalike (A6 regression guard)"
  affects:
    - 65-03 (analyze_blast_radius integration — needs allowlist to vet-clean)
    - 65-06 (Wave-2 consumer adapters that import internal/semantic/integ from kernel-side)
tech_stack:
  added:
    - "(none — uses existing golang.org/x/tools/go/analysis/analysistest)"
  patterns:
    - "Slash-boundary HasPrefix allowlist (exact match OR prefix+'/') — defends against bare-HasPrefix lookalike bypass"
    - "Helper function isForbidden(path) extracted from inline predicate to keep the Analyzer.Run loop body readable"
key_files:
  created:
    - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/semantic/integ/stub.go
    - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/semantic/integ_evil/stub.go
    - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/integimport/integimport.go
    - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/integlookalike/integlookalike.go
  modified:
    - internal/lint/nokernel2semantic/analyzer.go
    - internal/lint/nokernel2semantic/analyzer_test.go
decisions:
  - "Slash-boundary check (path == allowedSemanticIntegPath OR HasPrefix(path, allowedSemanticIntegPath+'/')) instead of bare HasPrefix — closes A6"
  - "Extract isForbidden(path) helper rather than inline the boolean — the three-clause predicate is easier to audit and test as a named function"
  - "Single named constant allowlist (no config-driven expansion path) — architectural boundary lives in source; future expansion requires a deliberate code edit + new analysistest fixture"
  - "Stub fixture packages live under testdata/src/.../internal/semantic/integ AND .../integ_evil — analysistest only needs the import target to resolve, not the production package"
metrics:
  duration: "~6 min wall clock (RED→GREEN→commit)"
  tasks_completed: 1
  files_changed: 6
  completed: "2026-05-08"
---

# Phase 65 Plan 00: nokernel2semantic Integ Allowlist Summary

Phase 65 wave-0 M-vet unblock — `nokernel2semantic` analyzer now allowlists exactly `internal/semantic/integ` (and sub-packages) so subsequent waves can import the new types-only seam from kernel packages without tripping `make vet`, while a slash-boundary regression guard prevents `internal/semantic/integ_evil`-style lookalike escape hatches.

## What Was Built

| Component | File | Purpose |
|---|---|---|
| Allowlist constant | `internal/lint/nokernel2semantic/analyzer.go` | `allowedSemanticIntegPath = "github.com/agenthands/helix/internal/semantic/integ"` |
| Predicate helper | `internal/lint/nokernel2semantic/analyzer.go` | `isForbidden(path)` — three-clause check: under semantic prefix AND not exact integ AND not integ-with-slash |
| Allowlist test (T1) | `internal/lint/nokernel2semantic/analyzer_test.go` | `TestAnalyzer_AllowsKernelImportingSemanticInteg` — kernel/integimport stays silent |
| A6 regression guard (T3) | `internal/lint/nokernel2semantic/analyzer_test.go` | `TestAnalyzer_RejectsKernelImportingSemanticIntegLookalike` — kernel/integlookalike still fires |
| Allowlist stub | `testdata/.../internal/semantic/integ/stub.go` | Resolvable import target for fixture |
| Lookalike stub | `testdata/.../internal/semantic/integ_evil/stub.go` | Resolvable bare-prefix lookalike |
| Allow fixture | `testdata/.../internal/kernel/integimport/integimport.go` | Imports allowlisted path, no `// want` |
| Reject fixture | `testdata/.../internal/kernel/integlookalike/integlookalike.go` | Imports lookalike path with `// want` directive |

## TDD Cycle

| Phase | Commit | What Happened |
|---|---|---|
| RED | `d2a56741` `test(65-00): add failing tests for nokernel2semantic integ allowlist` | Tests + fixtures + stubs added; `TestAnalyzer_AllowsKernelImportingSemanticInteg` fails as expected against unmodified analyzer (`unexpected diagnostic: internal/kernel/* must not import internal/semantic/* (got import "github.com/agenthands/helix/internal/semantic/integ" ...)`) |
| GREEN | `39499269` `feat(65-00): allowlist internal/semantic/integ in nokernel2semantic analyzer` | `allowedSemanticIntegPath` constant + `isForbidden(path)` helper landed; all four tests pass under `-race` |
| REFACTOR | (skipped — none needed) | Helper extraction was part of GREEN; no separate cleanup pass warranted |

## Verification

| Gate | Command | Result |
|---|---|---|
| Analyzer test (race) | `go test ./internal/lint/nokernel2semantic/... -count=1 -race` | PASS — 4/4 (RejectsKernelImportingSemantic, AllowsKernelWithoutSemanticImport, AllowsKernelImportingSemanticInteg, RejectsKernelImportingSemanticIntegLookalike) |
| Analyzer canary grep | `grep -c "allowedSemanticIntegPath" internal/lint/nokernel2semantic/analyzer.go` | 4 (≥ 2 required) |
| Test canary grep | `grep -c "AllowsKernelImportingSemanticInteg" internal/lint/nokernel2semantic/analyzer_test.go` | 2 (≥ 1 required) |
| Analyzer go vet | `go vet ./internal/lint/nokernel2semantic/...` | EXIT 0 |
| Project go build | `go build ./...` | EXIT 0 (only pre-existing cgo macro warning in tree-sitter swift bindings) |
| `make vet` (full singlechecker matrix) | `make vet` | EXIT 0 — `vet-nokernel2semantic`, `vet-noduckdb`, `vet-nosemantic2kernel`, `vet-compact-uses-store` all clean against the full tree |

## Acceptance Criteria — Status

- [x] `go test ./internal/lint/nokernel2semantic/... -count=1 -race` exits 0 with PASS for all three new/modified tests
- [x] `grep -c "allowedSemanticIntegPath" internal/lint/nokernel2semantic/analyzer.go` returns ≥ 2 (got 4)
- [x] `grep -c "AllowsKernelImportingSemanticInteg" internal/lint/nokernel2semantic/analyzer_test.go` returns ≥ 1 (got 2)
- [x] `go vet ./internal/lint/nokernel2semantic/...` exits 0
- [x] `go build ./...` exits 0

## Must-Haves — Truths Verified

- [x] Kernel packages can import `internal/semantic/integ` without violating nokernel2semantic — proven by `TestAnalyzer_AllowsKernelImportingSemanticInteg` PASS against the GREEN analyzer
- [x] Kernel packages still cannot import `internal/semantic/store`, `internal/semantic/retrieval`, `internal/semantic/graph` — `TestAnalyzer_RejectsKernelImportingSemantic` (existing) still PASSes against the kernel/badpkg → semantic/foo fixture; the slash-boundary helper rejects everything under `internal/semantic/*` except the explicit allowlist path
- [x] `analysistest` fixture confirms allowlisted import passes; existing fixtures still fail as expected — all four tests PASS under `-count=1 -race`

## Threat Model Disposition

| Threat | Status | Evidence |
|---|---|---|
| T-65-00-01 (T — bypass via bare-HasPrefix) | mitigate ✓ | `isForbidden` requires exact equality OR `+"/"` slash boundary; `TestAnalyzer_RejectsKernelImportingSemanticIntegLookalike` regression-asserts `internal/semantic/integ_evil` still fires |
| T-65-00-02 (E — silent allowlist expansion) | mitigate ✓ | Allowlist is a single named constant in source. No config-driven expansion path. Adding more requires a code edit AND a new analysistest fixture; package doc-comment now states this contract explicitly. |
| T-65-00-03 (I — analyzer error text leaks secrets) | accept | Diagnostic message contains import path + package path only. No secrets in source. |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — blocking] Removed analysistest-tripping `// want` token from fixture doc comment**

- **Found during:** Task 1 RED-phase verification
- **Issue:** Initial integimport.go fixture's doc comment included the literal phrase `analysistest \`// want\` directive`. analysistest's pre-parser scans the entire file for the substring `// want`-like patterns and reported `literal not terminated` because the trailing backtick was on a different line of the comment.
- **Fix:** Rewrote the fixture's doc comment to describe the contract without using the literal `// want` token. The semantic meaning is unchanged.
- **Files modified:** `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/integimport/integimport.go`
- **Commit:** Folded into `d2a56741` (RED)

No other deviations. Plan executed exactly as written; both TDD gates (`test(...)` then `feat(...)`) landed in the prescribed order.

## Authentication Gates

None.

## Known Stubs

None. The two stub packages (`testdata/.../internal/semantic/integ/stub.go` and `testdata/.../internal/semantic/integ_evil/stub.go`) are intentional analysistest-fixture-only artifacts — they exist solely so the analysis fixture imports resolve to a real package. They are not production code and are not consumed by anything outside `testdata/`.

## Threat Flags

None — this plan touched only the analyzer + analysistest fixtures. No new network endpoints, auth paths, file access patterns, or schema changes introduced.

## Commits

| Order | Hash | Type | Summary |
|---|---|---|---|
| 1 | `d2a56741` | test | Failing tests + fixtures (RED) |
| 2 | `39499269` | feat | Analyzer allowlist + helper (GREEN) |

## Self-Check: PASSED

- `internal/lint/nokernel2semantic/analyzer.go` — FOUND
- `internal/lint/nokernel2semantic/analyzer_test.go` — FOUND
- `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/integimport/integimport.go` — FOUND
- `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/integlookalike/integlookalike.go` — FOUND
- `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/semantic/integ/stub.go` — FOUND
- `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/semantic/integ_evil/stub.go` — FOUND
- Commit `d2a56741` (RED) — FOUND
- Commit `39499269` (GREEN) — FOUND

## TDD Gate Compliance

- RED gate: `d2a56741` `test(65-00): ...` — present in git log
- GREEN gate: `39499269` `feat(65-00): ...` — present in git log, after RED
- REFACTOR gate: not required (helper extraction was part of GREEN; no further cleanup needed)

All gates compliant.
