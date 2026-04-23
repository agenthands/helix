---
phase: 42-changelog-claude-md
plan: 01
subsystem: documentation
tags: [documentation, changelog, release-notes]
requires: []
provides:
  - "CHANGELOG.md entries for v1.3 through v1.7"
  - "Canonical release history through current milestone"
affects: [CHANGELOG.md]
tech_stack:
  added: []
  patterns:
    - "Reverse-chronological entries with ## vN.M — Title (YYYY-MM-DD) headings"
    - "### subsection grouping per shipped subsystem"
key_files:
  created: []
  modified:
    - CHANGELOG.md
decisions:
  - "Use .planning/milestones/v{N}-ROADMAP.md as canonical source for feature lists, with MILESTONES.md as cross-reference"
  - "Match README.md capitalization for feature names (Fuzzy Editing, Smart Errors, Progressive Descriptions, Lazy Initialization, Setup CLI)"
  - "No 'Unreleased' or 'Python port/rewrite' framing anywhere in the changelog"
metrics:
  duration_minutes: 2
  tasks_completed: 2
  completed_date: "2026-04-23"
  lines_before: 91
  lines_after: 206
---

# Phase 42 Plan 01: CHANGELOG Audit Summary

Added five missing milestone entries (v1.3 through v1.7) to CHANGELOG.md, bringing the canonical release history from stopping at v1.2 up to current state.

## Objective

CHANGELOG.md is the canonical release history for Serena. Prior to this plan it stopped at v1.2 (2026-04-10), hiding five shipped milestones from the user-facing record. This plan closes that gap using MILESTONES.md and the per-milestone v{N}-ROADMAP.md files as source of truth.

## Tasks Completed

| # | Task | Commit |
| - | ---- | ------ |
| 1 | Append v1.3, v1.4, v1.5 entries above v1.2 | `0e92917f` |
| 2 | Prepend v1.7 and v1.6 entries at top | `de7a58ad` |

## Changes

### CHANGELOG.md

Final ordering (top to bottom, reverse-chronological):

1. `## v1.7 — Developer Experience & Auto-Setup (2026-04-22)` — 6 subsections (Setup CLI, Health & Status, Claude Code Hooks, Smart Error Responses, Progressive Descriptions, Lazy Workspace Initialization)
2. `## v1.6 — Context Intelligence & Resilient Editing (2026-04-20)` — 5 subsections (Fuzzy Editing, Fuzzy Edit Integration, RepoMap Context Intelligence, Multi-Language Grammar Expansion, Verification & Wiring)
3. `## v1.5 — Typed Errors & Hardening (2026-04-15)` — 3 subsections (Error Taxonomy, Tool Migration, Testing)
4. `## v1.4 — Integration Testing v2 (2026-04-14)` — 3 subsections (Test Harness, Oracle Tests, LLM Behavioral Tests)
5. `## v1.3 — Documentation Catchup (2026-04-11)` — 3 subsections (README & Observability Docs, Install & Usage Guides, Changelog)
6. `## v1.2 — Performance & Production Hardening (2026-04-10)` (unchanged)
7. `## v1.1 — Integration Testing (2026-04-09)` (unchanged)
8. `## v1.0 — MVP (2026-04-08)` (unchanged)

**Line count:** 91 → 206 (+115 lines).

## Cross-Check: Feature Parity with MILESTONES.md

### v1.6 subsystem coverage

| Subsystem in MILESTONES.md / v1.6-ROADMAP | CHANGELOG subsection |
| ----------------------------------------- | -------------------- |
| Fuzzy editing 4-strategy cascade          | ### Fuzzy Editing    |
| Ellipsis placeholder, ambiguity refusal   | ### Fuzzy Editing bullets |
| Fuzzy integration with replace_* tools    | ### Fuzzy Edit Integration |
| Tree-sitter tag extraction + fallback     | ### RepoMap Context Intelligence |
| SQLite tag cache with mtime invalidation  | ### RepoMap Context Intelligence |
| PageRank ranker + scope-aware renderer    | ### RepoMap Context Intelligence |
| get_repo_map / get_context MCP tools      | ### RepoMap Context Intelligence |
| 23-grammar expansion (full aider parity)  | ### Multi-Language Grammar Expansion |
| Cache persistence verification            | ### Verification & Wiring |

### v1.7 subsystem coverage

| Subsystem in MILESTONES.md / v1.7-ROADMAP | CHANGELOG subsection |
| ----------------------------------------- | -------------------- |
| Setup CLI (`serena setup <client>`)       | ### Setup CLI        |
| Per-client CLI subprocess registration    | ### Setup CLI bullets |
| get_health MCP tool + serena status       | ### Health & Status  |
| Claude Code hooks (SessionStart/PreToolUse/Stop) | ### Claude Code Hooks |
| Did-you-mean smart error middleware       | ### Smart Error Responses |
| Progressive / brief descriptions + get_tool_help | ### Progressive Descriptions |
| sync.Once lazy workspace initialization   | ### Lazy Workspace Initialization |

Every shipped subsystem from MILESTONES.md bullets and v{N}-ROADMAP.md phase lists has a matching `###` subsection in the new CHANGELOG entries.

## Acceptance Criteria

- All 8 `## vN.M` headings present, one per line, in strictly descending version order from top to bottom.
- Milestone dates match MILESTONES.md exactly (v1.0=2026-04-08 through v1.7=2026-04-22).
- `grep -cE '\bport\b|\brewrite\b' CHANGELOG.md` returns `0` — no Python-port framing.
- Final line count 206 (> 180 target).
- v1.6 required markers present: `4-strategy cascade`, `fuzzy_edit`, `PageRank`, `get_repo_map`, `get_context`, `23 languages`.
- v1.7 required markers present: `serena setup <client>`, `get_health`, `SessionStart`, `PreToolUse`, `Did you mean`, `get_tool_help`, `Lazy`.

## Deviations from Plan

None — plan executed exactly as written. Inserted content matches the verbatim blocks specified in Task 1 and Task 2 action sections.

## Self-Check: PASSED

- `CHANGELOG.md` modifications present and committed.
- Commit `0e92917f` found in `git log --oneline`.
- Commit `de7a58ad` found in `git log --oneline`.
- All 8 version headings present in reverse-chronological order.
- Zero `port` / `rewrite` tokens in final file.
