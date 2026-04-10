---
phase: 13-graceful-degradation
plan: 01
subsystem: degrade
tags: [timeout-budgets, tool-classification, config, graceful-degradation]
dependency_graph:
  requires: []
  provides: [degrade-package, tool-class-map, budget-lookup, degradation-config]
  affects: [internal/degrade, internal/config]
tech_stack:
  added: []
  patterns: [tool-class-enum, config-driven-budgets, clamp-to-default]
key_files:
  created:
    - internal/degrade/budget.go
    - internal/degrade/budget_test.go
  modified:
    - internal/config/config.go
    - internal/config/defaults.go
decisions:
  - "DegradationConfig struct placed in config package, consumed by degrade via import (no circular dependency)"
  - "Negative/zero config values clamped to hardcoded defaults per T-13-01 threat mitigation"
  - "Unmapped tools default to ClassRead (tightest 5s budget) as safety net per T-13-02"
metrics:
  duration_seconds: 148
  completed: "2026-04-10"
  tasks_completed: 2
  tasks_total: 2
  test_count: 10
  test_pass: 10
---

# Phase 13 Plan 01: Tool-Class Budget Foundation Summary

Tool-to-class mapping for all 38 MCP tools with per-class configurable timeout budgets and threat-mitigated clamping to hardcoded defaults.

## What Was Built

### internal/degrade/budget.go
- `ToolClass` string enum with 5 constants: `ClassRead`, `ClassSearch`, `ClassEdit`, `ClassIndex`, `ClassDiagnostics`
- `toolClassMap` mapping all 38 registered tool names to their timeout class
- `ToolClassFor(toolName)` returning the class (defaults to ClassRead for unknown tools)
- `BudgetFor(toolName, DegradationConfig)` returning the timeout duration with config override support
- `configTimeout` and `clampPositive` helpers enforcing T-13-01 (never returns zero/negative)

### internal/degrade/budget_test.go
- 10 table-driven tests covering all 38 tool mappings, unmapped defaults, per-class budget assertions, config overrides, negative clamping, and never-returns-zero invariant

### internal/config/config.go
- `DegradationConfig` struct with 7 koanf-tagged fields (5 timeout classes + memory_limit_mb + restart_budget)
- `Degradation` field added to `SerenaConfig`

### internal/config/defaults.go
- 7 default values registered in `DefaultConfig()` map matching D-02 decisions

## Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Failing tests for tool-class map | f26f0e7c | internal/degrade/budget_test.go |
| 1 (GREEN) | Implement tool-class map and budget lookup | ba8e25b5 | internal/degrade/budget.go, internal/degrade/budget_test.go, internal/config/config.go |
| 2 | Add DegradationConfig defaults | 6778d2fb | internal/config/defaults.go |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] DegradationConfig struct created in Task 1 instead of Task 2**
- **Found during:** Task 1 (TDD GREEN)
- **Issue:** Task 1's degrade package imports `config.DegradationConfig` which Task 2 was supposed to create. Tests cannot compile without it.
- **Fix:** Added `DegradationConfig` struct and `Degradation` field to `config.go` during Task 1's implementation phase. Task 2 then only needed to add defaults.
- **Files modified:** internal/config/config.go
- **Commit:** ba8e25b5

## Threat Surface

T-13-01 mitigated: `BudgetFor` clamps negative/zero config values to hardcoded defaults via `clampPositive`. Test `TestBudgetFor_NegativeConfigClampedToDefault` and `TestBudgetFor_NeverReturnsZero` verify this invariant.

T-13-02 accepted: `toolClassMap` is a compile-time constant. Unmapped tools get `ClassRead` (tightest budget). Test `TestToolClassMap_UnmappedDefault` verifies.

## Self-Check: PASSED

All 4 files found. All 3 commits verified.
