# Phase 38: Progressive Descriptions & Lazy Init - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-22
**Phase:** 38-progressive-descriptions-lazy-init
**Areas discussed:** Description tiering strategy, get_tool_help tool design, Behavioral test gating, Lazy init trigger mechanism
**Mode:** --auto (all decisions auto-selected)

---

## Description Tiering Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Extend ToolDef with BriefDescription | Add a new field to ToolDef, ProfileFilterMiddleware uses it for tools/list | [auto] |
| Store tiers in profile YAML only | Keep ToolDef unchanged, put brief descriptions in each profile's overrides | |
| Separate description registry | External YAML/JSON mapping tool names to brief descriptions | |

**User's choice:** [auto] Extend ToolDef with BriefDescription (recommended default)
**Notes:** Cleanest separation. ProfileFilterMiddleware already rewrites descriptions on tools/list, so this is a natural extension.

| Option | Description | Selected |
|--------|-------------|----------|
| Universal brief descriptions | Brief descriptions are a tool property, profiles can still override | [auto] |
| Profile-specific brief descriptions | Each profile defines its own brief descriptions | |

**User's choice:** [auto] Universal brief descriptions per tool (recommended default)
**Notes:** Brief descriptions describe what the tool does -- that's universal. Profile overrides remain for agent-specific rewording.

---

## get_tool_help Tool Design

| Option | Description | Selected |
|--------|-------------|----------|
| Full docs: description + params + examples + patterns | Comprehensive documentation matching DESC-02 | [auto] |
| Params only: extract from JSON Schema | Minimal -- just parameter documentation | |
| Link to external docs | Return URL or file reference | |

**User's choice:** [auto] Full description + parameter details + usage examples + common patterns (recommended default)
**Notes:** DESC-02 requires comprehensive documentation.

| Option | Description | Selected |
|--------|-------------|----------|
| Embedded Go strings co-located with registration | Help content near tool definitions | [auto] |
| External markdown files | Separate .md files per tool | |
| YAML-based help database | Structured help in YAML | |

**User's choice:** [auto] Embedded Go strings co-located with tool registration (recommended default)
**Notes:** Keeps help content close to tool definitions, single binary, no file system dependency.

| Option | Description | Selected |
|--------|-------------|----------|
| Kernel tool via RegisterTools | Consistent with get_health, direct schema access | [auto] |
| Skill tool | Uses ToolProvider interface | |

**User's choice:** [auto] Kernel tool registered via RegisterTools (recommended default)
**Notes:** Needs access to tool schemas, consistent with get_health pattern.

---

## Behavioral Test Gating

| Option | Description | Selected |
|--------|-------------|----------|
| Golden-file snapshot tests | Compare tool listings against known-good baselines | [auto] |
| Property-based tests | Assert invariants (length, content rules) without snapshots | |
| LLM-based behavioral tests | Use oracle judge to verify tool selection quality | |

**User's choice:** [auto] Golden-file snapshot tests (recommended default)
**Notes:** Existing bench/oracle test patterns use golden files. Direct, deterministic, CI-friendly.

| Option | Description | Selected |
|--------|-------------|----------|
| test/bench/ alongside tools_manifest_test.go | Consistent with existing patterns | [auto] |
| test/integration/ | Integration test directory | |

**User's choice:** [auto] test/bench/ alongside existing manifest tests (recommended default)
**Notes:** tools_manifest_test.go already exists as a pattern.

---

## Lazy Init Trigger Mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| MCP receiving middleware | Intercepts tools/call, auto-activates if needed | [auto] |
| Per-tool guard check | Each tool handler checks workspace state | |
| Kernel-level wrapper | Kernel methods check and activate lazily | |

**User's choice:** [auto] Middleware that intercepts tools/call (recommended default)
**Notes:** Middleware pattern is established, keeps tool handlers clean. LAZY-02 sync.Once is natural in middleware.

| Option | Description | Selected |
|--------|-------------|----------|
| Derive from tool arguments + config fallback | Use relative_path/repo_path from args, fall back to project root | [auto] |
| Always use config project root | Ignore tool arguments | |
| Require explicit activation | No lazy init -- always require activate_project first | |

**User's choice:** [auto] Derive from tool arguments with config fallback (recommended default)
**Notes:** Tools already pass path context, fallback to config is safe.

---

## Claude's Discretion

- Exact brief description wording for each tool
- Help text authoring style and depth per tool
- get_tool_help return format (markdown vs plain text)
- Internal structure of lazy init middleware
- Error handling when lazy init fails

## Deferred Ideas

None -- discussion stayed within phase scope.
