# Phase 38: Progressive Descriptions & Lazy Init - Context

**Gathered:** 2026-04-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Tool surface is self-documenting for agents, and workspaces activate automatically on first use. Covers DESC-01 (tiered descriptions), DESC-02 (get_tool_help deep documentation), DESC-03 (behavioral test gating), LAZY-01 (first-call workspace activation), LAZY-02 (thread-safe lazy init).

</domain>

<decisions>
## Implementation Decisions

### Description Tiering Strategy
- **D-01:** Extend `ToolDef` in `internal/mcp/registry.go` with a `BriefDescription` field. Keep existing `Description` as the detailed version. Brief descriptions are under 100 tokens each (per success criteria #1).
- **D-02:** Brief descriptions are universal per tool (not profile-specific). Profile `ToolDescriptionOverrides` remain available on top for agent-specific rewording. `ProfileFilterMiddleware` uses `BriefDescription` for `tools/list` responses when the field is populated.
- **D-03:** Each tool registration site (kernel tools in symbols/edit/fileops/diag/health, skill tools in memory/workflow/repomap) adds a brief description alongside the existing detailed description.

### get_tool_help Tool Design
- **D-04:** New MCP tool `get_tool_help` accepts a `tool_name` string parameter and returns comprehensive documentation: full description, parameter details with types and constraints (from JSON Schema), usage examples, and common patterns.
- **D-05:** Help content is stored as embedded Go strings co-located with tool registration (e.g., a `helpText` const or var near each tool's `AddTool` call). This keeps help content close to the tool it documents.
- **D-06:** `get_tool_help` is a kernel-level tool registered via `RegisterTools` pattern (consistent with `get_health`). It needs access to the tool schema registry to extract parameter documentation dynamically.
- **D-07:** Parameter details (names, types, required/optional, enum values) are extracted from the MCP SDK's registered tool `InputSchema` at runtime — not hardcoded. Only usage examples and common patterns are authored manually per tool.

### Behavioral Test Gating (DESC-03)
- **D-08:** Golden-file snapshot tests comparing tool listings (names + brief descriptions) against known-good baselines. A description change that doesn't update the golden file fails the test, forcing explicit review.
- **D-09:** Tests live in `test/bench/` alongside the existing `tools_manifest_test.go` pattern. May extend that file or create a sibling `tools_descriptions_test.go`.
- **D-10:** Test covers: (a) brief descriptions exist for all tools, (b) brief descriptions are under 100 tokens, (c) golden-file match against baseline snapshot.

### Lazy Init Trigger Mechanism
- **D-11:** Implement as MCP receiving middleware in `internal/mcp/` that intercepts `tools/call` requests. If no workspace is active for the request context, the middleware auto-activates before forwarding to the tool handler.
- **D-12:** Workspace path is derived from tool arguments (`relative_path` or `repo_path` fields) when present, falling back to the daemon's configured project root from config. This mirrors how `activate_project` already works.
- **D-13:** Thread-safety via `sync.Once` (per workspace path) ensures concurrent first calls don't race on activation (LAZY-02). The middleware maintains a map of `sync.Once` per workspace root.
- **D-14:** Lazy init is transparent — tools behave identically whether the workspace was pre-activated (via `serena setup` or `activate_project`) or lazy-activated. The only observable difference is a slightly longer first-call latency.

### Claude's Discretion
- Exact brief description wording for each tool (under 100 tokens, factual)
- Help text authoring style and depth per tool
- Whether `get_tool_help` returns markdown or plain text
- Internal structure of lazy init middleware (single file or separate package)
- Error handling when lazy init fails (degrade gracefully or return error to tool call)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Tool registration and descriptions
- `internal/mcp/registry.go` -- ToolDef struct that needs BriefDescription field added
- `internal/mcp/middleware.go` -- ProfileFilterMiddleware that rewrites descriptions on tools/list, InstallMiddleware entry point
- `internal/mcp/server.go` -- SerenaMCPServer, tool schema storage (`toolSchemas` field), activate_project tool
- `internal/profile/profile.go` -- Profile struct with ToolDescriptionOverrides map
- `internal/profile/profiles/claude-code.yaml` -- Example profile with tool_description_overrides

### Tool registration sites (need brief descriptions added)
- `internal/kernel/symbols/tools.go` -- 9 symbol retrieval tools
- `internal/kernel/edit/tools.go` -- 6 symbol editing tools
- `internal/kernel/fileops/tools.go` -- 6 file operation tools
- `internal/kernel/diag/tools.go` -- 3 diagnostic tools
- `internal/kernel/health/tools.go` -- Health tool
- `internal/skill/memory/skill.go` -- 7 memory tools
- `internal/skill/workflow/skill.go` -- Workflow tools (onboarding, session handoff)
- `internal/skill/repomap/skill.go` -- Repomap tool

### Lazy init and workspace activation
- `internal/kernel/kernel.go` -- `ActivateWorkspace()` method that lazy init calls
- `internal/workspace/workspace.go` -- Workspace registry and state
- `internal/daemon/daemon.go` -- Daemon bootstrap, middleware installation, tool registration

### Existing test patterns
- `test/bench/tools_manifest_test.go` -- Existing tool manifest golden-file tests to extend or mirror

### Requirements
- `.planning/REQUIREMENTS.md` -- DESC-01, DESC-02, DESC-03, LAZY-01, LAZY-02

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ToolDef` in `internal/mcp/registry.go` -- Core struct to extend with BriefDescription. Already has Name, Description, RegisterFn.
- `ProfileFilterMiddleware` in `internal/mcp/middleware.go` -- Already rewrites descriptions on tools/list. Natural place to switch to BriefDescription.
- `toolSchemas []*mcpsdk.Tool` in SerenaMCPServer -- Already stores tool schemas for suggestion middleware (Phase 37). get_tool_help can introspect these for parameter documentation.
- `ToolRegistry.Names()` -- Enumerates all tools. Useful for get_tool_help validation.
- `ActivateWorkspace()` in kernel.go -- Existing workspace activation logic that lazy init middleware calls.
- `tools_manifest_test.go` in test/bench/ -- Golden-file pattern for tool listing validation.

### Established Patterns
- Middleware pattern: `mcpsdk.Middleware` wrapping `MethodHandler`, method-specific interception (tools/call, tools/list)
- Kernel tool registration: `RegisterTools(server, kernel, pool)` with typed args and mcpsdk.AddTool
- Skill tool registration: `ToolProvider.Tools()` returning `[]*mcp.ToolDef`, daemon registers centrally
- Golden-file testing: snapshot comparison with `testdata/` baselines

### Integration Points
- `internal/mcp/registry.go` -- Add BriefDescription to ToolDef
- `internal/mcp/middleware.go` -- Modify ProfileFilterMiddleware to use BriefDescription; add lazy init middleware
- `internal/daemon/daemon.go` -- Install lazy init middleware, register get_tool_help tool
- All tool registration sites -- Add brief descriptions
- `test/bench/` -- Add description regression tests

</code_context>

<specifics>
## Specific Ideas

No specific requirements -- open to standard approaches. Key concerns from STATE.md: verify MCP SDK `tools/changed` notification support for dynamic descriptions during implementation.

</specifics>

<deferred>
## Deferred Ideas

None -- discussion stayed within phase scope

</deferred>

---

*Phase: 38-progressive-descriptions-lazy-init*
*Context gathered: 2026-04-22*
