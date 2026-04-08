# Phase 4: Agent Profiles and Configuration - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Pre-built agent profiles (Claude Code, Codex, IDE assistant, CI bot + full/vanilla fallback), dynamic mode switching via switch_mode MCP tool, layered config precedence, and token budget reporting via aggregate endpoint.

</domain>

<decisions>
## Implementation Decisions

### Profile Design
- **D-01:** Heavily curated built-in profiles — each has specific tool subset, rewritten descriptions per client, prompt overrides, mode defaults, and safety/policy knobs. Profiles should noticeably change agent behavior.
- **D-02:** 5 built-in profiles: `claude-code`, `codex`, `ide-assistant`, `ci-bot`, `full` (neutral escape hatch with broad tool exposure, minimal prompt shaping).
- **D-03:** Profile = ContextSpec (from Phase 3 skill interface). Each profile is a YAML ContextSpec file loaded at daemon startup. Go code provides defaults; YAML overrides.

### Mode Switching
- **D-04:** Explicit `switch_mode` MCP tool as the product API, implemented via dynamic tool registry hot-swap under the hood.
- **D-05:** 4 modes: `read`, `edit`, `review`, `admin`. `switch_mode` is the only way to cross mode boundaries.
- **D-06:** Mode = ModeSpec (from Phase 3 skill interface). Each mode defines tool inclusion/exclusion rules, prompt fragments, and behavior policies.
- **D-07:** Per-session mode state — each MCP session tracks its current mode independently. Mode changes are auditable.

### Token Budget
- **D-08:** Aggregate `get_token_budget` MCP tool, NOT per-tool schema metadata. Signature: `get_token_budget(profile?, mode?, client?, model?) -> { total, loaded_now, deferred, cacheable, per_tool_breakdown[] }`
- **D-09:** Per-tool breakdown included in response for debugging, but not part of the standard MCP tool listing.

### Configuration Precedence
- **D-10:** CLI args > project `.serena/config.yaml` > user `~/.serena/config.yaml` > active profile defaults. Already implemented in Phase 1 via koanf — Phase 4 wires profiles into the precedence chain.

### Claude's Discretion
- Specific tool subsets per profile (which tools to include/exclude for Claude Code vs Codex vs IDE vs CI)
- Prompt override content per profile
- Mode transition validation rules (which mode transitions are allowed)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 3 Skill Interface (foundation for profiles)
- `internal/skill/spec.go` — ContextSpec, ModeSpec YAML types, ResolveTools function
- `internal/skill/registry.go` — Skill registry with init() registration
- `internal/skill/skill.go` — Skill, ToolProvider, WorkflowProvider interfaces

### Phase 1 Config & MCP
- `internal/config/loader.go` — koanf layered config loading (extend for profile selection)
- `internal/mcp/middleware.go` — Per-session tool filtering middleware (profiles apply here)
- `internal/mcp/server.go` — MCP server (mode switching updates tool listing)
- `internal/mcp/registry.go` — Dynamic tool registry (add/remove for mode switch)
- `internal/mcp/session.go` — Session state (add current mode tracking)

### Legacy Reference
- `legacy/src/serena/config/context_mode.py` — Current context/mode YAML definitions to port
- `legacy/src/serena/resources/` — Context/mode YAML files (desktop-app, agent, ide-assistant, etc.)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/skill/spec.go` — ContextSpec and ModeSpec already defined; profiles are ContextSpec instances
- `internal/mcp/middleware.go` — Tool filtering middleware already supports per-session filtering
- `internal/config/loader.go` — Layered config already works; just needs profile as a config source

### Established Patterns
- YAML-driven tool set composition (ContextSpec.IncludeTools, ExcludeTools)
- Caddy-style skill registration (profiles register as skills)
- Per-session state in session.go

### Integration Points
- Profiles loaded at daemon startup via config
- Mode state tracked per MCP session
- switch_mode tool triggers registry hot-swap via middleware
- get_token_budget reads current profile + mode state

</code_context>

<specifics>
## Specific Ideas

- Profiles as embedded YAML ContextSpec files in `internal/profile/` with Go defaults
- switch_mode state machine: read → edit allowed, edit → review allowed, admin → any, etc.
- get_token_budget computes schema sizes using tiktoken-compatible token counting

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-agent-profiles-and-configuration*
*Context gathered: 2026-04-08*
