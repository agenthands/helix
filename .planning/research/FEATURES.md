# Feature Landscape

**Domain:** Developer Experience & Auto-Setup for MCP Code Intelligence Server
**Researched:** 2026-04-20
**Scope:** Only features needed for the v1.7 milestone. Existing infrastructure (41 MCP tools, 52-language support, 5 profiles, 4 modes, typed errors, daemon architecture, onboarding workflow) is the baseline. This milestone adds zero-friction setup, client hooks, health tools, smart error guidance, and progressive tool descriptions.

## Table Stakes

Features users expect from a mature MCP server tool. Missing = product feels incomplete or requires unnecessary manual work.

### Setup & Onboarding

| Feature | Why Expected | Complexity | Dependencies (existing) | Notes |
|---------|--------------|------------|------------------------|-------|
| `serena setup <client>` CLI command | Every MCP ecosystem tool has one-command registration: `codex mcp add`, Amazon Q `q mcp add`, IntelliJ "Auto-Configure" button. Manual JSON editing is 2024-era friction. | Medium | cobra CLI, INSTALL.md patterns (6 clients documented), profile system | Must write correct JSON to correct config path per client. Detect binary path, validate. Support: claude-code, codex, cursor, gemini, vscode, opencode |
| Language detection during setup | Serena already does this in `gatherProjectInfo()` (langExtensions map, 20 languages). Setup should detect and report so user knows what LSes will be needed. | Low | Existing `gatherProjectInfo()` in workflow/skill.go, three-tier LS installer | Reuse existing detection logic from onboard_project tool. Optionally trigger LS pre-install |
| LS pre-installation on setup | First tool call after setup should be instant. Three-tier installer exists but only triggers on first use. | Low | Three-tier LS installer (PATH/download/error), language detection | Call installer for detected languages during setup. Report success/skip/fail per language |
| Setup verification (health check) | Currently INSTALL.md says "run onboard_project to confirm." That requires the agent to be running. Setup should self-verify. | Low | Daemon start, kernel init | Start daemon, check LSes respond, exit with clear pass/fail + actionable error |
| `--dry-run` flag for setup | Users want to review what will change before it changes. Standard CLI practice. | Low | Setup command logic | Print what would be written, to which path, without writing. |

### Health & Status

| Feature | Why Expected | Complexity | Dependencies (existing) | Notes |
|---------|--------------|------------|------------------------|-------|
| Health/status MCP tool (`get_health`) | Agents need "is this working?" without guessing. IBM MCP Context Forge filed this as a feature request. Dedicated MCP health monitor servers exist (mcp-server-health-monitor). Built-in is table stakes. | Low | Kernel pool state (`lspool/`), language registry, workspace state | Report: active LSes + their state, indexing progress, workspace root, detected languages, pool stats (active/idle/circuit-broken workers) |
| `serena status` CLI command | Human-readable version of health for debugging. Complements the MCP tool. | Low | Same data as get_health, formatted for terminal | JSON and human-readable modes. Can pipe to agent via hook. |
| Error-only diagnostic reporting | Agents waste tokens processing irrelevant LS diagnostics (unused variable warnings, import suggestions). Surface only actionable failures. | Low | Existing `diag/` package, LSP diagnostic severity levels | Filter by severity (Error only by default, configurable). Already have the data, just need filtering |

### Error Guidance

| Feature | Why Expected | Complexity | Dependencies (existing) | Notes |
|---------|--------------|------------|------------------------|-------|
| Actionable error messages with suggestions | When agent calls wrong tool or passes bad params, modern tools say "did you mean X?" Git does this. Rust compiler does this. k8sgpt does this. | Medium | Typed error taxonomy (7 kinds from v1.5), tool registry, parameter schemas | Pattern match common misuse: wrong tool for intent, missing required params, symbol not found but similar exists. Build suggestion catalog. |
| "Tool not available in current mode" guidance | When agent tries to use an edit tool in read mode, explain what mode switch is needed rather than generic "permission denied." | Low | Profile/mode system, `switch_mode` tool | Error response includes: current mode, required mode, `switch_mode` tool call example |

## Differentiators

Features that set Serena apart. Not expected from every MCP server, but high-value for a code intelligence platform targeting coding agents.

### Client Hooks (Claude Code focus)

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| Auto-install Claude Code hooks during setup | Not just register MCP server -- install hooks that make integration feel native. No other MCP server does this. Claude Code hooks are well-documented JSON in settings.json. | Medium | `serena setup claude-code`, Claude Code `.claude/settings.json` hooks format | Three hooks: SessionStart (inject workspace state), PreToolUse (remind about Serena tools), Stop (cleanup) |
| SessionStart hook: inject workspace state | On session start, automatically tell agent about project capabilities, active LSes, available modes. Agent starts informed without calling onboard_project. | Low | Hook installation, `serena status --json` CLI | Hook runs `serena status --json`, output goes to Claude's context via stdout. Documented pattern in hook ecosystem. |
| PreToolUse hook: remind about Serena tools | Before agent uses raw Edit/Write, remind it that Serena has symbol-level editing (replace_symbol_body, rename_symbol). Reduces raw file manipulation. | Low | Hook installation, tool name matchers | Matcher: `Edit(*)` or `Write(*)` triggers reminder. customInstructions field in hook response. Well-documented Claude Code pattern. |
| Stop hook: session cleanup | When session ends, notify daemon to release workspace resources, trim pool, save session metrics. | Low | Hook installation, daemon IPC | Lightweight signal: `serena session-end`. Daemon can GC idle workers. |

### Progressive Tool Surface

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| Tiered tool descriptions (short default) | Reduce context window overhead from 41 tool descriptions. Speakeasy/SynapticLabs report 85-100x token reduction with progressive disclosure. Serena's 41 tools at ~200 tokens each = ~8K tokens of tool schemas. | Medium | Profile system, tool registry, MCP SDK tool listing | NOT the meta-tool pattern (too disruptive). Instead: short descriptions by default (1 line), full descriptions available via a `get_tool_help` tool. Profiles already filter tools. |
| Tool categorization in descriptions | Group tools logically so agents can scan: "Symbol Retrieval (9)", "Symbol Editing (6)", "File Operations (6)", etc. | Low | Tool registry metadata | Add category field to ToolDef. Descriptions start with "[Category] one-line description". Helps agents find relevant tools faster. |
| `get_tool_help` meta-tool | Agent calls `get_tool_help("replace_symbol_body")` to get full description, parameters, examples, common mistakes. Keeps base descriptions short. | Low | Tool registry, static help content | Simple lookup tool. Content can be embedded strings or generated from tool schemas + usage notes. |
| Usage examples in tool help | Show agents concrete examples of correct tool usage. Reduces trial-and-error. | Low | `get_tool_help` tool | Examples are static content: "To rename a Go function: ..." Pattern from Claude Skills best practices doc. |

### Smart Error Responses

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| "Did you mean?" tool suggestions | Agent calls `get_definition` but meant `find_symbol`. Response includes "Did you mean find_symbol? get_definition requires an exact symbol path." | Medium | Tool registry, error context, intent-to-tool mapping | Build intent catalog: "find something" -> find_symbol/search_for_pattern, "see definition" -> get_definition (with exact path), etc. |
| Parameter correction hints | Agent passes `file` but tool expects `relative_path`. Response: "Unknown parameter 'file'. Did you mean 'relative_path'?" | Low | Tool parameter schemas, string similarity | Levenshtein distance on param names. Already have fuzzy matching infrastructure from v1.6. |
| Common mistake catalog | Document top-10 agent mistakes with specific corrections. Embedded in error responses. | Low | Observation from LLM behavioral tests (v1.4) | Examples: calling symbol tools without workspace, using absolute paths, forgetting to switch mode |

### Lazy Init

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| Lazy workspace init on first tool call | User skips `serena setup`, agent calls any tool, Serena bootstraps. Zero-config path for quick usage. | Medium | Workspace detection, LS installer, kernel init, health reporting | Must be fast enough that agent doesn't timeout (~30s budget). Background indexing with immediate partial results. Return "initializing, X% ready" status. |
| Graceful "not ready yet" responses | During lazy init, tools that need LSes return "workspace initializing, try again in N seconds" instead of cryptic errors. | Low | Health tool state, init progress tracking | Use existing Timeout error kind. Include ETA based on LS startup patterns. |

## Anti-Features

Features to explicitly NOT build.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Full meta-tool progressive disclosure (discover + execute indirection) | Adds latency, complexity, breaks existing tool integrations. Agents already know tool names from prior sessions. 41 tools is manageable with profiles already filtering to ~15-20 per profile. | Tiered descriptions (short default) + `get_tool_help` for details. All tools stay directly callable. |
| Auto-update mechanism | Single binary via `go install`. Auto-updaters add security surface, daemon restart complexity, version conflicts. | Print "update available: vX.Y.Z" in health output. Let user `go install` manually. |
| GUI/TUI setup wizard | Agents (primary users) cannot interact with TUIs. Human users are technical enough for CLI. | CLI with sensible defaults + `--dry-run` for preview |
| Client-specific plugin/extension packages | Maintaining VS Code extensions, JetBrains plugins is orthogonal to core value. Massive support burden. | JSON config injection via `serena setup` covers all clients without platform-specific binaries. |
| Monitoring dashboard | Already exports Prometheus metrics (v1.2). Custom dashboard duplicates Grafana. | `get_health` MCP tool for agents + existing /metrics for operators |
| AI-powered error explanation | Adds LLM dependency to error path. Unpredictable latency, cost, and failure modes. | Deterministic pattern-matched suggestions. "Did you mean X" is reliable and instant. |
| Hook installation for VS Code/JetBrains | These editors have their own MCP integration that doesn't use Claude Code-style hooks. The hook system is Claude Code specific. | Support MCP registration for all clients, hooks only for Claude Code (where the system exists and is documented) |
| Workspace auto-detection across multiple projects | Ambiguous when user has multiple repos. Which one to activate? | Explicit workspace via setup or first tool call with `relative_path` indicating the root |

## Feature Dependencies

```
serena setup <client>
  --> Language detection (reuse gatherProjectInfo from workflow/skill.go)
  --> LS pre-installation (three-tier installer, existing)
  --> Hook installation (Claude Code: settings.json hooks; others: N/A)
  --> Health check verification (serena status)
  --> Profile selection (existing 5 profiles)

get_health MCP tool
  --> Kernel pool state access (lspool/ metrics)
  --> Language registry queries (langregistry/)
  --> Workspace state (kernel/)

serena status CLI
  --> Same data as get_health, terminal-formatted
  --> Used by SessionStart hook

Smart error responses
  --> Typed error taxonomy (v1.5 done, 7 kinds)
  --> Tool registry (for "did you mean" lookups)
  --> Parameter schema introspection (MCP SDK tool schemas)
  --> Intent-to-tool mapping catalog (new, static)

Progressive descriptions
  --> Profile system (already filters tools per profile)
  --> Tool registry metadata (add category, short_description fields)
  --> get_tool_help tool (new, simple lookup)

Lazy workspace init
  --> Workspace detection logic (existing in kernel/)
  --> Background LS startup (existing lspool/ with circuit breaking)
  --> Health tool (to report "still initializing")
  --> Timeout error kind (existing from v1.5)

Client hooks (Claude Code)
  --> serena setup claude-code (installs them)
  --> serena status --json (SessionStart hook calls it)
  --> Daemon IPC (Stop hook signals session end)
```

## MVP Recommendation

### Priority 1 -- Setup CLI (unlocks everything else)

1. `serena setup claude-code` -- writes .mcp.json + installs hooks to .claude/settings.json
2. `serena setup codex` / `cursor` / `gemini` / `opencode` / `vscode` -- writes MCP config (no hooks for these)
3. `serena status` CLI command (human-readable + JSON mode)
4. `--dry-run` and `--profile` flags
5. Setup verification (start daemon, health check, report)

### Priority 2 -- Agent-facing DX

6. `get_health` MCP tool -- active LSes, indexing state, workspace capabilities, pool stats
7. Smart error responses -- "did you mean" suggestions on InvalidArgs/NotFound errors
8. Error-only reporting mode -- severity filtering for get_diagnostics
9. Mode-switch guidance in permission errors

### Priority 3 -- Progressive refinement

10. Tiered tool descriptions -- short defaults, full via `get_tool_help`
11. Lazy workspace init -- first-call bootstrapping with "not ready" responses
12. Client hook refinement -- tune PreToolUse matchers, stop cleanup

**Defer:** Meta-tool pattern (too disruptive), VS Code/JetBrains hooks (no system exists), auto-update (nice-to-have).

## Complexity Budget

| Feature | Estimated Effort | Risk | Confidence |
|---------|-----------------|------|------------|
| Setup CLI framework (cobra subcommand) | 1 day | Low | HIGH |
| Client config writers (6 clients) | 2-3 days | Low -- JSON templates known from INSTALL.md | HIGH |
| Claude Code hook installation | 1-2 days | Low -- format is stable, well-documented | HIGH |
| LS pre-installation during setup | 1 day | Low -- calls existing installer | HIGH |
| `serena status` CLI + get_health MCP tool | 1-2 days | Low -- reads existing state | HIGH |
| Smart error suggestions (intent catalog) | 2-3 days | Medium -- need to catalog common misuse patterns from LLM tests | MEDIUM |
| Parameter correction hints | 1 day | Low -- Levenshtein on param names, reuse fuzzy infra | HIGH |
| Tiered tool descriptions | 2 days | Medium -- need to write good short descriptions for 41 tools | MEDIUM |
| `get_tool_help` meta-tool | 1 day | Low -- simple lookup | HIGH |
| Lazy workspace init | 2-3 days | Medium -- timeout handling, partial readiness, race conditions | MEDIUM |
| Error-only diagnostic mode | 0.5 days | Low -- severity filter on existing diagnostics | HIGH |
| Mode-switch error guidance | 0.5 days | Low -- add to existing permission check | HIGH |

**Total estimated effort:** 15-20 days for all features.

## Edge Cases and Known Difficulties

### Setup CLI
- **Binary not in PATH:** Detect and use absolute path in config. Warn user.
- **Config file already has serena entry:** Update vs. skip? Default: update with backup, `--no-overwrite` flag.
- **Multiple config locations (project vs. global):** Default to project-level, `--global` flag for user-level.
- **Client not installed:** Detect and warn "cursor not found, writing config anyway."
- **Permissions issues:** Config dir not writable, hooks dir not writable. Clear error message.

### Client Hooks
- **Existing hooks in settings.json:** Must merge, not overwrite. Parse existing JSON, add Serena hooks.
- **Hook format changes:** Claude Code hooks are versioned. Pin to known-working format.
- **Hook execution environment:** PATH may differ from user's shell. Use absolute path to serena binary.
- **Windows paths:** Different config locations, different path separators.

### Lazy Init
- **Timeout budget:** Agent MCP calls typically timeout at 30-60s. LS startup (gopls, pyright) can take 10-30s for large projects. Must return partial results within budget.
- **Concurrent first calls:** Multiple tools called simultaneously before init completes. Need init-once synchronization (sync.Once pattern).
- **Init failure:** LS download fails, no network. Must degrade gracefully: file ops work, symbol ops return "LS unavailable" error.

### Progressive Descriptions
- **MCP SDK constraints:** Tool descriptions are set at registration time. Dynamic descriptions may need tools/list refresh notification. Check MCP spec for `tools/changed` notification support.
- **Agent caching:** Some agents cache tool lists. Short descriptions must be self-sufficient for tool selection.

## Sources

- [Claude Code Hooks reference](https://code.claude.com/docs/en/hooks) -- hook events, matchers, JSON format, PreToolUse/SessionStart/Stop
- [Claude Code MCP configuration](https://code.claude.com/docs/en/mcp) -- .mcp.json locations, format
- [Claude Code hooks deep dive](https://blog.vincentqiao.com/en/posts/claude-code-settings-hooks/) -- settings.json structure, matcher syntax
- [Meta-Tool Pattern for Progressive Disclosure](https://blog.synapticlabs.ai/bounded-context-packs-meta-tool-pattern) -- architecture pattern, 85x token savings
- [Progressive Disclosure MCP: 85x Token Savings Benchmark](https://matthewkruczek.ai/blog/progressive-disclosure-mcp-servers.html) -- quantified benefits
- [Speakeasy: 100x Token Reduction with Dynamic Toolsets](https://www.speakeasy.com/blog/how-we-reduced-token-usage-by-100x-dynamic-toolsets-v2) -- progressive vs semantic approach comparison
- [MCPrism - Progressive tool disclosure](https://github.com/jbabin91/mcprism) -- reference implementation saving 98% context
- [MCP Server Health Monitor](https://mcpservers.org/servers/dbsectrainer/mcp-server-health-monitor) -- health check patterns, SQLite history
- [IBM MCP Context Forge health dashboard request](https://github.com/ibm/mcp-context-forge/issues/547) -- market demand signal
- [OpenAI Codex MCP setup](https://developers.openai.com/codex/mcp) -- `codex mcp add` command pattern
- [Gemini CLI MCP setup](https://geminicli.com/docs/cli/tutorials/mcp-setup/) -- settings.json format
- [Claude Code hooks mastery](https://github.com/disler/claude-code-hooks-mastery) -- community patterns and examples
- [Claude Skills best practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices) -- progressive disclosure, domain organization
- [MCP Tools specification](https://modelcontextprotocol.io/specification/2025-11-25/server/tools) -- tool listing, descriptions, changed notifications
- [Less is More: MCP design patterns](https://www.klavis.ai/blog/less-is-more-mcp-design-patterns-for-ai-agents) -- 4 patterns for better MCP servers
