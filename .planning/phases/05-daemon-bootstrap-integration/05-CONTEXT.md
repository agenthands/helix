# Phase 5: Daemon Bootstrap Integration - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire all existing Phase 2-4 components (kernel, tools, skills, profiles, middleware) into the daemon startup path so the production binary exposes all 30+ MCP tools. No new features — purely orchestration code connecting individually-tested subsystems.

</domain>

<decisions>
## Implementation Decisions

### Skill-to-MCP Wiring Strategy
- **D-01:** Daemon owns all MCP tool registration centrally. Skills describe tools via `ToolProvider.Tools()`, daemon iterates all providers and calls `mcpsdk.AddTool` for each. RegisterFn on ToolDef becomes vestigial (can remain as compatibility shim but is not the primary registration path).
- **D-02:** Future `SyncRegistry(session/profile/modes)` pattern: daemon computes the effective tool list, diffs against current state, registers/unregisters through the MCP SDK. This keeps registration, replacement, profile filtering, and future `list_changed` behavior in one place.

### Profile Skill Name Alignment
- **D-03:** Wrap kernel tool packages (symbols, edit, fileops, diag) as thin skill adapters implementing `ToolProvider`. All capabilities flow through one `Skill/ToolProvider` interface — no two-tier authoring.
- **D-04:** Profile YAMLs reference skill names consistently. At runtime, daemon resolves skill names to flat tool-name allowlists before MCP registration. "Skills for composition, tool names for execution."
- **D-05:** Fix profile YAML skill names to match the actual skill wrapper names (e.g., `symbol-retrieval` maps to a real registered SymbolRetrievalSkill).

### Daemon Startup Sequence
- **D-06:** Fail-fast for core subsystems (config, language registry, kernel, MCP server, workspace registry). If any core subsystem fails, daemon refuses to start with a clear error.
- **D-07:** Degraded mode only for optional capability providers. Rule: "The daemon may start degraded only when failed subsystems are capability providers whose absence does not compromise control-plane correctness, session semantics, or safety guarantees." E.g., if one skill fails init, log warning and continue without its tools.
- **D-08:** Startup order: config -> lang registry -> kernel (creates pool) -> skill.InitAll(deps) -> daemon computes effective tool set from skills+profile -> register all tools with MCP SDK -> install ProfileFilterMiddleware -> start listeners (socket + HTTP) in errgroup.

### Pool-Installer Wiring
- **D-09:** Pool must call `langregistry.Installer.Resolve()` for three-tier LS resolution (PATH lookup -> managed download -> helpful error) instead of using `entry.Command` directly. Claude's discretion on the exact integration point.

### Shutdown
- **D-10:** Daemon shutdown must stop kernel/pool (drain workers, close LS processes) before closing listeners. Claude's discretion on ordering details.

### Claude's Discretion
- Exact Go struct composition for kernel tool skill wrappers (thin adapters vs. generated)
- Whether to remove RegisterFn from ToolDef or leave as dead code
- Blank import location (daemon package, cli package, or dedicated `imports.go`)
- Error message formatting for startup failures

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Audit Report
- `.planning/v1.0-MILESTONE-AUDIT.md` -- Complete gap analysis with 7 integration gaps, 5 broken E2E flows, and per-requirement wiring status

### Key Source Files (read before modifying)
- `internal/daemon/daemon.go` -- Current daemon bootstrap (needs wiring)
- `internal/daemon/shutdown.go` -- Current shutdown (needs kernel/pool stop)
- `internal/mcp/server.go` -- MCP server with `mcpsdk.AddTool` pattern
- `internal/mcp/middleware.go` -- ProfileFilterMiddleware (needs installation)
- `internal/kernel/kernel.go` -- Kernel orchestrator (needs instantiation)
- `internal/kernel/lspool/pool.go` -- Worker pool (needs installer wiring)
- `internal/skill/skill.go` -- Skill/ToolProvider/WorkflowProvider interfaces
- `internal/skill/registry.go` -- Global skill registry with Register/All/InitAll
- `internal/config/loader.go` -- ResolveProfile function (needs calling)
- `internal/profile/skill.go` -- Profile skill with SetSessionProvider

### Kernel Tool Registration Points
- `internal/kernel/symbols/tools.go` -- `RegisterTools(server, kernel, wsKeyFn)`
- `internal/kernel/edit/tools.go` -- `RegisterTools(server, kernel, extractor, diagStore, wsKeyFn)`
- `internal/kernel/fileops/tools.go` -- `RegisterTools(server, workspaceRoot)`
- `internal/kernel/diag/tools.go` -- `RegisterTools(server, store, workspaceRoot, leaseFn)`

### Skill Packages (need blank imports for init())
- `internal/skill/memory/skill.go` -- 7 MCP tools, RegisterFn no-op
- `internal/skill/workflow/skill.go` -- 2 MCP tools, RegisterFn no-op
- `internal/profile/skill.go` -- 2 MCP tools (switch_mode, get_token_budget)

### Profile YAMLs (need skill name fixes)
- `internal/profile/profiles/claude-code.yaml`
- `internal/profile/profiles/codex.yaml`
- `internal/profile/profiles/ide-assistant.yaml`
- `internal/profile/profiles/ci-bot.yaml`
- `internal/profile/profiles/full.yaml`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `mcpsdk.AddTool(server, tool, handler)` -- The SDK registration primitive, already used for ping/echo/activate_project
- `skill.All()` -- Returns all registered skills (after init() runs)
- `skill.InitAll(deps)` -- Initializes all skills with shared dependencies
- `ToolProvider.Tools() []*mcp.ToolDef` -- Interface for skill-provided tools
- `RegisterTools()` functions in kernel packages -- Already take `*mcp.SerenaMCPServer`
- `ProfileFilterMiddleware` -- Ready to install, filters tools/list responses
- `config.ResolveProfile()` -- Bridges config profile name to ProfileStore
- `ProfileSkill.SetSessionProvider()` -- Wires session awareness into profile skill

### Established Patterns
- errgroup for concurrent subsystem orchestration (daemon.Run)
- Signal-first context cancellation (signal.NotifyContext before any goroutine)
- Kernel tools: `RegisterTools(server *mcp.SerenaMCPServer, ...)` with typed args structs + `mcpsdk.AddTool`
- Skill tools: `ExecuteTool(name string, params map[string]interface{}) (string, error)` dispatch
- Caddy-style `init()` registration for skills

### Integration Points
- `daemon.New()` -- Where kernel, registry, and skills need to be created
- `daemon.Run()` -- Where startup ordering and errgroup subsystems are orchestrated
- `daemon.shutdown()` -- Where kernel/pool cleanup needs to happen
- `NewSerenaMCPServer()` -- Where tool registration and middleware installation happens
- `cmd/serena/main.go` or `internal/cli/root.go` -- Where blank imports for skill packages go

</code_context>

<specifics>
## Specific Ideas

- Daemon should own a `SyncRegistry` function that computes effective tool list from skills+profile+mode, enabling future dynamic tool set changes
- "Skills for composition, tool names for execution" -- clear separation of authoring vs. runtime model
- Kernel tool skill wrappers should be thin adapters, not heavy abstractions

</specifics>

<deferred>
## Deferred Ideas

None -- discussion stayed within phase scope

</deferred>

---

*Phase: 05-daemon-bootstrap-integration*
*Context gathered: 2026-04-08*
