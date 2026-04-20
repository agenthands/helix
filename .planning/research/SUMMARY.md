# Project Research Summary

**Project:** Serena
**Domain:** MCP Server Developer Experience
**Researched:** 2026-04-20
**Confidence:** HIGH

## Executive Summary

Serena v1.7 is a pure integration milestone — zero new external dependencies required. All features compose existing capabilities (41 MCP tools, 52-language support, daemon architecture, typed errors, profile system) into a zero-friction setup and smarter agent interaction layer. The recommended approach is dependency-ordered phasing: setup CLI first (everything depends on it), then health/status (needed by hooks), then hooks (Claude Code primary), then smart errors (benefits from observing real patterns), and progressive descriptions last (requires behavioral test regression gate).

The primary risks are config file format instability across clients (mitigate by using client CLIs as subprocess rather than direct file manipulation), smart error suggestions creating infinite retry loops (mitigate by restricting to parameter corrections only), and progressive descriptions breaking agent tool selection (mitigate by implementing last with behavioral tests as gate).

The six new source files all follow established patterns: middleware for cross-cutting concerns, skills for new MCP tools, cobra subcommands for CLI operations. No architectural changes needed.

## Key Findings

### Recommended Stack

Zero new dependencies. All v1.7 features use existing Go stdlib + current dependency set.

**Core technologies (already in go.mod):**
- `cobra v1.9.1`: Setup CLI subcommands (`serena setup`, `serena status`, `serena hook`)
- `MCP Go SDK`: Health tool registration, tools/changed notification
- `koanf v2`: Config generation for client setup files
- `encoding/json` (stdlib): Client config file generation (JSON for all targets)
- `os/exec` (stdlib): Client CLI subprocess calls (`claude mcp add-json`, `code --install-extension`)

**What NOT to add:**
- go-enry — existing langregistry covers language detection
- Template engines — JSON marshaling sufficient for config generation
- HTTP client libs — stdlib net/http adequate for hook communication

### Expected Features

**Must have (table stakes):**
- `serena setup <client>` one-command registration for 6 clients
- `get_health` MCP tool reporting LS state and capabilities
- Actionable error messages with "did you mean" suggestions
- `serena status` CLI showing workspace health
- Error-only reporting (suppress noise by default)

**Should have (differentiators):**
- Claude Code hook auto-installation (PreToolUse/SessionStart/Stop)
- Lazy workspace init on first tool call
- Progressive tool descriptions (tiered detail levels)
- `get_tool_help` deep-dive tool for individual tool documentation

**Defer (v2+):**
- Meta-tool pattern (discover+execute indirection)
- VS Code/JetBrains hook systems (less mature than Claude Code)
- Auto-update mechanism
- GUI/TUI setup wizard
- AI-powered error explanations

### Architecture Approach

No new architectural layers. Six integration points into existing structure:

**Major components:**
1. `cmd/serena/setup/` — Cobra subcommand for client registration + hook installation + LS pre-install
2. `internal/mcp/middleware/` — Lazy init middleware, error enrichment middleware, description adapter
3. `internal/skill/health/` — Health skill (ToolProvider) exposing `get_health` MCP tool
4. `internal/setup/hooks/` — Hook template generation for Claude Code, VS Code, JetBrains
5. Existing `internal/errors/` — Extended with suggestion field for smart error responses
6. Existing profile YAMLs — Tiered description variants for progressive disclosure

### Critical Pitfalls

1. **Config format instability** — Use `claude mcp add-json --scope project` subprocess, not direct file writes
2. **Hook exit code semantics** — Claude Code: exit 1 is NON-blocking, only exit 2 blocks. Counterintuitive.
3. **Smart error retry loops** — Never redirect to different tools in error suggestions. Parameter corrections only.
4. **Progressive descriptions breaking selection** — Behavioral tests must gate description changes. Implement last.
5. **Lazy init race conditions** — Concurrent first calls must be synchronized via sync.Once pattern

## Implications for Roadmap

### Phase 1: Setup CLI Foundation
**Rationale:** Everything else depends on setup working — hooks need config paths, health needs workspace context
**Delivers:** `serena setup claude-code|vscode|jetbrains`, MCP registration, language detection, LS pre-install
**Addresses:** Table stakes (one-command setup), error-only reporting (suppress noise during install)
**Avoids:** Config format instability (uses client CLIs as subprocess)

### Phase 2: Health & Status
**Rationale:** Low-risk new skill; hooks and lazy init depend on health reporting
**Delivers:** `get_health` MCP tool, `serena status` CLI, compact workspace health reporting
**Addresses:** Table stakes (health tool), feedback loop (visibility into LS state)
**Avoids:** Noisy health (capped at <500 tokens, error-only default)

### Phase 3: Client Hooks (Claude Code)
**Rationale:** Depends on setup and health; primary differentiator; well-documented Claude Code API
**Delivers:** PreToolUse remind, SessionStart activate, Stop cleanup hooks auto-installed by setup
**Addresses:** Differentiator (native Claude Code integration), discoverability (nudge toward symbolic tools)
**Avoids:** Exit code confusion (validate in integration tests), config instability (hook templates versioned)

### Phase 4: Smart Error Responses
**Rationale:** Benefits from observing real error patterns in phases 1-3; extends existing typed error taxonomy
**Delivers:** Error enrichment middleware, "did you mean" suggestions, parameter correction hints
**Addresses:** Error clarity, self-diagnosis capability for agents
**Avoids:** Retry loops (parameter corrections only, never tool redirections)

### Phase 5: Progressive Descriptions & Lazy Init
**Rationale:** Must be last — requires strong behavioral test coverage as regression gate
**Delivers:** Tiered descriptions, `get_tool_help` tool, lazy workspace init on first call
**Addresses:** Discoverability (41 tools → manageable surface), zero-config fallback
**Avoids:** Breaking agent tool selection (behavioral tests gate), race conditions (sync.Once)

### Phase Ordering Rationale

- Dependency chain: Setup → Health → Hooks → Errors → Descriptions (each builds on previous)
- Risk gradient: Low risk first (setup, health), high risk last (progressive descriptions)
- Pitfall avoidance: Smart errors after real usage patterns observed, descriptions after behavioral test coverage
- Feedback loop: Each phase immediately usable, no "big bang" integration

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3:** Hook architecture (inline command vs HTTP to admin listener), exit code validation, concurrent hook execution
- **Phase 5:** Token savings measurement, behavioral test coverage assessment, description tier triggering logic

Phases with standard patterns (skip research-phase):
- **Phase 1:** Well-documented client configs, cobra subcommands, established patterns
- **Phase 2:** Caddy-style skill registration, straightforward ToolProvider
- **Phase 4:** Middleware pattern proven in existing codebase (telemetry, profile filtering)

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified zero new deps needed against go.mod |
| Features | HIGH | 6+ MCP clients documented, patterns established |
| Architecture | HIGH | All integration points verified against source |
| Pitfalls | HIGH | Official docs + community issue trackers confirm |

**Overall confidence:** HIGH

### Gaps to Address

- JetBrains config path stability: `.junie/mcp/mcp.json` may evolve with Junie product
- MCP SDK `tools/changed` notification support: needed for dynamic descriptions, verify during Phase 5
- Optimal short description token count: needs empirical measurement with agent tool selection
- Windows path handling: config locations differ, needs testing in setup CLI

## Sources

### Primary (HIGH confidence)
- Claude Code hooks documentation (code.claude.com/docs/en/hooks)
- VS Code MCP configuration (official docs)
- Serena source code (internal/daemon, internal/mcp, internal/skill, cmd/serena)
- Python Serena reference implementation (oraios.github.io/serena)
- MCP Go SDK documentation

### Secondary (MEDIUM confidence)
- JetBrains Junie MCP docs (product is newer, format may shift)
- Community MCP server implementations (patterns, not specifications)
- Git/Rust compiler "did you mean" pattern analysis
- LLM behavioral test methodology (emerging practice)

### Tertiary (LOW confidence)
- Progressive description token savings estimates (needs validation)
- Meta-tool pattern benchmarks (academic, not production-tested)

---
*Research completed: 2026-04-20*
*Ready for roadmap: yes*
