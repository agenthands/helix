---
phase: 14-documentation
plan: 01
subsystem: documentation
tags: [codegen, readme, documentation]
dependency_graph:
  requires: []
  provides: [docgen-tool, readme-tables]
  affects: [README.md, Makefile]
tech_stack:
  added: []
  patterns: [marker-based-codegen, init-registration-for-codegen]
key_files:
  created:
    - cmd/docgen/main.go
    - cmd/docgen/main_test.go
  modified:
    - internal/langregistry/registry.go
    - README.md
    - Makefile
decisions:
  - "Skill Name() used as tool table Category column for grouping"
  - "displayName() maps registry keys to human-readable names with special cases for cpp/csharp/fsharp"
  - "replaceSection preserves markers and is idempotent by design"
metrics:
  duration: 3min
  completed: 2026-04-10
---

# Phase 14 Plan 01: README Codegen Tool and Update Summary

Docgen codegen tool that auto-generates tool (38+) and language (52) tables in README.md from Go skill and language registries, with --check CI mode for drift detection.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Build cmd/docgen codegen tool with tests (TDD) | eccd61a2 | cmd/docgen/main.go, cmd/docgen/main_test.go, internal/langregistry/registry.go, Makefile |
| 2 | Update README.md with markers, client configs, and run docgen | 87e2c83f | README.md |

## Implementation Details

### Task 1: Docgen Codegen Tool

Built `cmd/docgen/main.go` following TDD flow (RED: 6 failing tests, GREEN: implementation passes all).

Key design:
- Imports all skill packages via blank imports (same as daemon/imports.go) to trigger init() registration
- Calls `skill.ToolProviders()` to collect all tools with name and description grouped by skill Name()
- Creates `langregistry.NewRegistry()` and calls new `Entries()` method for sorted language access
- `replaceSection()` replaces content between `<!-- BEGIN X -->` / `<!-- END X -->` marker comments
- `--check` flag compares generated output vs current file, exits 1 if different (CI mode)
- `--readme` flag for custom README path (default `README.md`)

Added `Entries()` method to `langregistry.Registry` for sorted access to all entries.

Added `docs` target to Makefile: `go run ./cmd/docgen`.

### Task 2: README Update

- Replaced hand-maintained tool listings with marker-enclosed auto-generated table (38+ tools)
- Replaced inline language list with marker-enclosed auto-generated table (52 languages)
- Added Codex client config (`--profile=codex`)
- Added IDE Assistant client config (`--profile=ide-assistant`)
- Removed "Legacy Python Version" section per D-02
- Verified idempotency: `go run ./cmd/docgen --check` exits 0

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

- `go test ./cmd/docgen/... -v` -- 6/6 tests pass
- `go vet ./cmd/docgen/...` -- clean
- `go run ./cmd/docgen --check` -- exits 0 (tables in sync)
- `make docs` -- runs without error
- README.md contains all 4 marker comments
- README.md contains 3 client config blocks (Claude Code, Codex, IDE Assistant)
- README.md does not contain "Legacy Python Version"

## Self-Check: PASSED

All 6 files verified present. Both task commits (eccd61a2, 87e2c83f) verified in git log.
