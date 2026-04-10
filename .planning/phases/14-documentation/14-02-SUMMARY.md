---
phase: 14-documentation
plan: 02
subsystem: documentation
tags: [docs, usage-guide, profiles, observability, performance]
dependency_graph:
  requires: []
  provides: [USAGE.md]
  affects: []
tech_stack:
  added: []
  patterns: [hybrid-tutorial-reference-docs]
key_files:
  created:
    - USAGE.md
  modified: []
decisions:
  - "Structured USAGE.md as hybrid: tutorials first, then reference sections (per D-03)"
  - "Observability and performance tuning are sections within USAGE.md, not separate files (per D-04)"
metrics:
  duration: 2min
  completed: "2026-04-10"
  tasks_completed: 1
  tasks_total: 1
  files_created: 1
  files_modified: 0
---

# Phase 14 Plan 02: USAGE.md Operational Reference Summary

Complete operational reference for Serena operators: tutorials, profile/mode tables, config reference, troubleshooting, observability quickstart, and performance tuning with concrete config keys from the codebase.

## What Was Done

### Task 1: Create USAGE.md with tutorials and reference sections

Created `USAGE.md` at repo root (~400 lines) covering all DOC-05 through DOC-09 requirements:

- **Quick Tutorials** (3 tutorials): onboarding a new project, refactoring workflow with symbolic tools, code review workflow with ci-bot profile
- **Profiles and Modes**: all 5 profiles (claude-code, codex, ide-assistant, ci-bot, full) with tool exclusion lists from actual YAML files; all 4 modes (read, edit, review, admin) with tool access details; mode switching documentation
- **Configuration Reference**: 4-layer precedence explanation, all koanf keys from SerenaConfig/WorkerPoolConfig/ObservabilityConfig/DegradationConfig with types and defaults, complete example YAML files for user and project config
- **Troubleshooting**: language server not starting (three-tier resolution), cache/stale results, mode restrictions, circuit breaker open (ErrCircuitOpen), memory pressure
- **Observability Quickstart**: admin listener setup, health checks (/healthz, /readyz), Prometheus metrics scrape config with key metrics table, distributed tracing via OTLP, pprof profiling
- **Performance Tuning**: worker pool sizing with guidance per project size, timeout budgets table per tool class, memory limits (GOMEMLIMIT), restart budget and circuit breaker tuning

**Commit:** `50ba1dd3`

## Deviations from Plan

None -- plan executed exactly as written.

## Decisions Made

1. **Hybrid structure**: tutorials first, reference sections after (per D-03 decision)
2. **Single file**: observability and performance as sections within USAGE.md (per D-04 decision)

## Requirements Satisfied

- DOC-05: Profiles, modes, and configuration documented with tables and examples
- DOC-06: Three workflow tutorials (onboarding, refactoring, code review)
- DOC-07: Troubleshooting section with 5 scenarios and fixes
- DOC-08: Observability quickstart with admin listener, metrics, tracing, pprof
- DOC-09: Performance tuning with worker pool sizing, timeout budgets, memory limits
