---
phase: 33-fallback-extractor-wiring
plan: 01
subsystem: repomap
tags: [fallback-extractor, lsp, wiring, integration]
dependency_graph:
  requires: [repomap-fallback, treesitter-registry, lspool]
  provides: [fallback-extraction-pipeline]
  affects: [daemon-startup, walkAndExtract]
tech_stack:
  added: []
  patterns: [post-init-setter-wiring, nil-safe-registry-check]
key_files:
  created: []
  modified:
    - internal/skill/repomap/skill.go
    - internal/daemon/daemon.go
decisions:
  - "Nil-safe registry check: (s.registry == nil || s.registry.SupportsLanguage(lang)) preserves backward compat for tests that construct RepoMapSkill without Init()"
metrics:
  duration_seconds: 204
  completed: "2026-04-20T13:38:33Z"
---

# Phase 33 Plan 01: FallbackExtractor Wiring Summary

Wire FallbackExtractor into walkAndExtract pipeline with daemon post-init lease acquisition for LSP-based tag extraction on non-tree-sitter languages.

## What Was Done

### Task 1: Add FallbackDeps, registry field, SetFallbackDeps setter, and modify walkAndExtract
- Added `FallbackDeps` struct with `Registry`, `AcquireFn`, and `Extractor` fields
- Added `registry` and `fallbackDeps` fields to `RepoMapSkill` struct
- Stored `GrammarRegistry` on skill during `Init()` for `SupportsLanguage` checks
- Added `SetFallbackDeps` setter following existing `SetEnrichFn` pattern
- Changed `walkAndExtract` signature to accept `context.Context` parameter
- Gated tree-sitter path on `registry.SupportsLanguage(lang)` with nil-safe check
- Added fallback path calling `FallbackExtractor.Extract` via `AcquireFn`-provided `SymbolRequester`
- Missing LSP silently skips with debug log per D-33-02
- **Commit:** `17f5ef68`

### Task 2: Wire FallbackDeps in daemon.go post-init block
- Added section 12c wiring block after existing SetEnrichFn block
- Created per-language `AcquireFn` closure using `Pool.AcquireLease` with `WorkspaceKey{Language: lang}`
- Session IDs use `"fallback-{lang}"` pattern to prevent collisions (T-33-02 mitigation)
- Fixed nil-safe registry check for backward compatibility with tests constructing skill without Init()
- **Commit:** `919e60e8`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Nil pointer dereference on registry in walkAndExtract**
- **Found during:** Task 2 verification
- **Issue:** Existing integration tests construct `RepoMapSkill` directly without calling `Init()`, leaving `s.registry` nil. The original plan's condition `s.registry.SupportsLanguage(lang)` panicked.
- **Fix:** Changed to nil-safe check `(s.registry == nil || s.registry.SupportsLanguage(lang))` -- when registry is nil, fall through to extractor (backward-compatible behavior).
- **Files modified:** `internal/skill/repomap/skill.go`
- **Commit:** `919e60e8`

## Verification Results

- `go build ./...` -- PASS
- `go vet ./...` -- PASS
- `go test ./internal/skill/repomap/... -count=1` -- PASS
- `go test ./internal/repomap/... -count=1` -- PASS
- `SetFallbackDeps` present in both skill.go and daemon.go -- CONFIRMED
- `SupportsLanguage` gates tree-sitter path -- CONFIRMED

## Self-Check: PASSED
