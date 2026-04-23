---
phase: 39-readme-rewrite
plan: 02
subsystem: documentation
tags: [readme, product-identity, restructure]
dependency_graph:
  requires: [accurate-readme-tables]
  provides: [complete-product-readme]
  affects: [README.md]
tech_stack:
  added: []
  patterns: []
key_files:
  created: []
  modified:
    - README.md
decisions:
  - Led with user value (MCP tools + 52 languages) rather than Go-native as hero framing
  - Used collapsed details block for manual JSON configs to keep Quick Start clean
  - Added cross-links to USAGE.md and INSTALL.md from Quick Start section
metrics:
  duration: 3m
  completed: 2026-04-23
---

# Phase 39 Plan 02: Restructure README with New Section Order and Content Summary

Full README restructure per 13 locked decisions: new section order with TOC, Quick Start moved up with serena setup CLI, Key Features table for v1.7 highlights, RepoMap section, and legacy footer.

## Task Summary

| Task | Name | Commit | Status |
|------|------|--------|--------|
| 1 | Restructure README.md with new section order and content | 107041e5 | Done |

## What Was Done

1. **Added subtitle line** after hero tagline: "Code intelligence platform for MCP -- 40+ tools across 52 languages" (D-02)

2. **Added Table of Contents** with anchor links to all major sections (D-05)

3. **Moved Quick Start up** after advantages table (was after language/tool tables at line 191). Now leads with `serena setup <client>` commands for 5 clients (D-07, D-11). Manual JSON configs collapsed into `<details>` block.

4. **Added Key Features section** (D-13) with table rows for: Fuzzy Editing, Smart Errors, Progressive Descriptions, Health Monitoring, Lazy Initialization, Setup CLI.

5. **Added RepoMap section** (D-12) describing `get_repo_map` (tree-sitter + PageRank structural overview) and `get_context` (task-focused dependency graph analysis).

6. **Updated tool count** from "35+ MCP tools" to "40+ MCP tools" in How Serena Works section.

7. **Added legacy footer** one-liner: "Originally inspired by Python Serena" with link (D-04, LEGC-01).

8. **Added cross-links** to USAGE.md (from profile section) and INSTALL.md (from manual config details).

9. **Preserved all auto-gen markers** intact -- `go run ./cmd/docgen --check` confirms "README.md is up to date".

## Section Order (D-06 compliance)

1. Hero (logo + tagline + subtitle)
2. Bullet points (what Serena does)
3. Table of Contents
4. How Serena Works
5. Key Advantages table
6. Quick Start (with serena setup CLI)
7. Key Features (v1.7 highlights)
8. RepoMap
9. Production & Observability
10. Programming Language Support (auto-gen)
11. Features / Tools table (auto-gen)
12. Architecture (brief)
13. Acknowledgements
14. Legacy footer

## Deviations from Plan

None -- plan executed exactly as written.

## Verification

- `go run ./cmd/docgen --check` passes: "README.md is up to date"
- `grep -c 'serena setup' README.md` returns 8 (>= 5 required)
- `grep -c 'Originally inspired' README.md` returns 1
- No occurrences of "port" or "rewrite" in product context (only "loopback port" in architecture)
- `grep -c '## Table of Contents' README.md` returns 1
- `grep -c '## RepoMap' README.md` returns 1
- `grep -c '## Key Features' README.md` returns 1
- Quick Start at line 58, Key Features at line 149, Languages at line 203 (correct order)

## Self-Check: PASSED
