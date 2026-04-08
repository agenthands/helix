# Phase 5: Daemon Bootstrap Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-08
**Phase:** 05-daemon-bootstrap-integration
**Areas discussed:** Skill-to-MCP wiring, Profile skill name alignment, Daemon startup sequence

---

## Skill-to-MCP Wiring

| Option | Description | Selected |
|--------|-------------|----------|
| Daemon iterates Tools() | Daemon calls skill.All(), iterates each ToolProvider.Tools(), calls mcpsdk.AddTool. RegisterFn becomes unused. One place for all registration. | :heavy_check_mark: |
| Fix RegisterFn to work | Pass real *mcp.SerenaMCPServer into RegisterFn. Each skill registers itself. More decentralized. | |
| Claude's discretion | Let planner decide. | |

**User's choice:** Daemon iterates Tools() -- centralized registration. RegisterFn becomes vestigial.
**Notes:** User provided extensive rationale referencing MCP SDK design, GitHub MCP server precedent, and the need for one registration path to support future profile/mode-driven tool visibility. Recommended `SyncRegistry(session/profile/modes)` pattern for computing effective tool list with diff-based register/unregister.

---

## Profile Skill Name Alignment

| Option | Description | Selected |
|--------|-------------|----------|
| Wrap kernel tools as skills | Create thin skill wrappers implementing ToolProvider. All capabilities flow through one interface. Profiles reference consistent skill names. | :heavy_check_mark: |
| Two-tier: skills + kernel tools | Profiles reference two namespaces. Profile filtering works on tool names directly. Simpler code but less uniform. | |
| Claude's discretion | Let planner choose. | |

**User's choice:** Wrap kernel tools as thin skill adapters. Profiles resolve skill names to tool-name allowlists at runtime.
**Notes:** User emphasized avoiding two-tier authoring as "architectural leak" that causes long-term drift. Rule: "Skills for composition, tool names for execution." Referenced GitHub agent configuration as precedent for curated toolsets resolving to tool-name identifiers.

---

## Daemon Startup Sequence

| Option | Description | Selected |
|--------|-------------|----------|
| Fail-fast | Any subsystem failure prevents daemon start. Simple, predictable. | :heavy_check_mark: (with refinement) |
| Partial startup / degraded mode | Daemon starts with whatever initialized. Missing subsystems disable related tools. | (for optional providers only) |
| Claude's discretion | Let planner decide. | |

**User's choice:** Fail-fast for core subsystems. Degraded mode only for optional capability providers.
**Notes:** User provided clear rule: "The daemon may start degraded only when failed subsystems are capability providers whose absence does not compromise control-plane correctness, session semantics, or safety guarantees."

---

## Claude's Discretion

- Exact Go struct composition for kernel tool skill wrappers
- Whether to remove RegisterFn from ToolDef or leave as dead code
- Blank import location
- Error message formatting for startup failures
- Pool-to-installer integration details
- Shutdown ordering details

## Deferred Ideas

None -- discussion stayed within phase scope
