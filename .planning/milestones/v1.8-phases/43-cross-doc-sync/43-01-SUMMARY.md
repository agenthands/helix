---
phase: 43-cross-doc-sync
plan: 01
subsystem: docs
tags: [docs, readme, cross-doc-sync]
requires: []
provides:
  - "README.md canonical 7-client Quick Start"
  - "README tool count aligned on 41+"
  - "README Architecture Layer 3 label matching CLAUDE.md"
  - "README Fuzzy Editing strategy vocabulary matching CHANGELOG"
affects:
  - README.md
tech-stack:
  added: []
  patterns: []
key-files:
  created: []
  modified:
    - README.md
decisions:
  - "Canonicalize Layer 3 label on CLAUDE.md form ('Agent Profiles & Setup') since v1.7 setup CLI is a genuine Layer 3 component"
metrics:
  duration: "~5 minutes"
  completed: "2026-04-23"
  tasks: 3
  files_modified: 1
  commits: 3
---

# Phase 43 Plan 01: Align README with Code Truth Summary

One-liner: Updated README.md Quick Start, hero/intro tool count, Architecture Layer 3 label, and Fuzzy Editing strategy list to match canonical sources (internal/cli/setup_clients.go, CLAUDE.md, CHANGELOG.md).

## What Was Done

### Task 1 — Quick Start client enumeration (commit 9f5c1bb9)
- Added `serena setup opencode` and `serena setup generic` registrars
- Removed `/ Cursor` parenthetical from vscode line (no cursor registrar exists)
- Quick Start now enumerates all 7 canonical clients in registration order matching `internal/cli/setup_clients.go:37-46`

### Task 2 — Tool count and Layer 3 label (commit 28052801)
- Hero subtitle: `40+ tools across 52 languages` → `41+ tools across 52 languages`
- Intro paragraph: `40+ MCP tools` → `41+ MCP tools`
- Architecture block Layer 3: `Agent Profiles (5 profiles, 4 modes, token budget, layered config)` → `Agent Profiles & Setup (5 profiles, 4 modes, token budget, layered config, setup CLI)`

### Task 3 — Fuzzy Editing strategy vocabulary (commit 1bfff29c)
- Key Features table Fuzzy Editing row now lists all 4 canonical strategy names including `ellipsis-placeholder`, matching CHANGELOG.md:43 and `internal/fuzzy/strategies.go`

## Verification

All 7 plan-level checks pass:

| Check | Expected | Actual |
|-------|----------|--------|
| `grep -c "serena setup opencode" README.md` | ≥1 | 1 |
| `grep -c "serena setup generic" README.md` | ≥1 | 1 |
| `grep "VS Code / Cursor" README.md` | empty | empty |
| `grep "40+ MCP tools\|40+ tools across" README.md` | empty | empty |
| `grep -c "41+" README.md` | ≥2 | 2 |
| `grep -c "Agent Profiles & Setup" README.md` | ≥1 | 1 |
| `grep -c "ellipsis-placeholder" README.md` | ≥1 | 1 |

## Deviations from Plan

None - plan executed exactly as written.

## Commits

- `9f5c1bb9` docs(43-01): enumerate all 7 setup clients in README Quick Start
- `28052801` docs(43-01): align README tool count on 41+ and Layer 3 label
- `1bfff29c` docs(43-01): add ellipsis-placeholder to Fuzzy Editing strategy list

## Requirements Closed

- README-04 (Quick Start client enumeration matches setup_clients.go)
- CLMD-01 (README tool count aligned with CLAUDE.md)
- CLMD-02 (README Layer 3 label matches CLAUDE.md)

## Self-Check: PASSED

- FOUND: README.md (modified)
- FOUND commit 9f5c1bb9
- FOUND commit 28052801
- FOUND commit 1bfff29c
