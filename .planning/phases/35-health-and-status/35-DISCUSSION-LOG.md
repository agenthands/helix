# Phase 35: Health & Status - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-21
**Phase:** 35-health-and-status
**Areas discussed:** MCP tool interface, CLI status command, Error-only default, LS status granularity
**Mode:** --auto (all decisions auto-selected with recommended defaults)

---

## MCP Tool Interface (get_health)

| Option | Description | Selected |
|--------|-------------|----------|
| Structured JSON per-workspace | Per-workspace breakdown with per-language LS status, capabilities, indexing | ✓ |
| Flat list | Simple list of all LSes across workspaces | |
| Summary only | Single health score without per-LS details | |

**User's choice:** [auto] Structured JSON per-workspace (recommended default)
**Notes:** Aligns with HLTH-01 (see active LSes and status) and HLTH-02 (capabilities per workspace). Structured format enables both MCP tool and CLI consumption.

---

## CLI Status Command

| Option | Description | Selected |
|--------|-------------|----------|
| Compact colored output | Match SetupPrinter style from Phase 34 with checkmarks/crosses | ✓ |
| Full table | Detailed table with all fields | |
| Minimal one-liner | Single line summary | |

**User's choice:** [auto] Compact colored output (recommended default)
**Notes:** Consistency with Phase 34 setup output. --json flag available for machine consumption.

---

## Error-Only Default

| Option | Description | Selected |
|--------|-------------|----------|
| Actionable failures only | Crashed, missing, circuit-open, stalled indexing | ✓ |
| All non-healthy | Any state that isn't fully healthy | |
| Everything always | No filtering, verbose by default | |

**User's choice:** [auto] Actionable failures only (recommended default)
**Notes:** Per HLTH-04: surface actionable failures, suppress noise. Single summary line when all healthy.

---

## LS Status Granularity

| Option | Description | Selected |
|--------|-------------|----------|
| Three states + indexing sub-state | healthy/degraded/failed with indexing as transient | ✓ |
| Binary healthy/unhealthy | Simple up/down check | |
| Five states | Running, indexing, degraded, crashed, missing | |

**User's choice:** [auto] Three states + indexing sub-state (recommended default)
**Notes:** Balances granularity with simplicity. Maps directly to pool circuit breaker states.

---

## Claude's Discretion

- Internal health query API structure
- Exact error message formatting
- Health check probe timeout
- Cache vs live query decision

## Deferred Ideas

None — discussion stayed within phase scope
