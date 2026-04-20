# Technology Stack

**Project:** Serena v1.7 Developer Experience & Auto-Setup
**Researched:** 2026-04-20

## Recommended Stack Additions

### Zero New Go Dependencies Required

The v1.7 milestone features are **integration code** and **new MCP tools** -- they compose existing capabilities rather than requiring new libraries. Every feature maps cleanly to stdlib + existing deps.

### Core Framework (No Changes)

| Technology | Version | Purpose | Status |
|------------|---------|---------|--------|
| Go | 1.25.1 | Language runtime | Already in use |
| cobra | v1.9.1 | CLI framework -- add `setup` subcommand | Already in use |
| koanf/v2 | v2.3.4 | Config reading (not writing -- client configs use json.Marshal) | Already in use |
| MCP Go SDK | v1.5.0 | Register health/status MCP tool | Already in use |
| modernc.org/sqlite | v1.48.1 | Available if state persistence needed | Already in use |
| gRPC + protobuf | v1.80.0 / v1.36.11 | Hook CLI <-> daemon IPC | Already in use |

### Standard Library Packages Used Per Feature

| Feature | Stdlib Packages | Notes |
|---------|----------------|-------|
| `serena setup <client>` | `encoding/json`, `os`, `path/filepath`, `runtime` | JSON config generation + file writing |
| Language detection in setup | (none new) | Reuse `internal/langregistry` + `internal/kernel/workspace.go` DetectLanguages |
| LS pre-installation | (none new) | Reuse `internal/langregistry/installer.go` three-tier installer |
| Client hooks generation | `encoding/json` | Write hooks JSON into `.claude/settings.json` |
| Health/status MCP tool | (none new) | New skill querying existing lspool/kernel state |
| Smart error responses | (none new) | Add `Suggestion` field to existing `internal/errors` builder |
| Progressive descriptions | (none new) | Extend profile YAML schema, existing koanf reads it |
| Lazy workspace init | (none new) | Middleware in existing MCP request pipeline |
| Hook subcommands | `encoding/json`, `os` | Read stdin JSON, write stdout JSON, gRPC to daemon |

## Architecture Decisions

### 1. CLI: Introduce Subcommands (Evolving from Flat Design)

Current CLI is flat ("Per D-02: flat CLI with flags, no subcommands" in root.go). For v1.7, add subcommand trees because setup/hook are distinct from server operation:

```
serena                         # existing: stdio/http/daemon (unchanged)
serena setup claude-code       # NEW: write MCP config + hooks
serena setup vscode            # NEW: write .vscode/mcp.json
serena setup jetbrains         # NEW: write .junie/mcp/mcp.json
serena setup --detect          # NEW: detect languages, report what would be installed
serena status                  # NEW: query daemon health (terminal output)
serena hook pre-tool-use       # NEW: hook handler (stdin JSON -> stdout JSON)
serena hook session-start      # NEW: hook handler
serena hook stop               # NEW: hook handler
```

**Why subcommands are correct here:** Setup and hook handlers are *not* server modes -- they're utility commands. Cobra supports this natively. The root command RunE stays untouched for backward compatibility.

**File structure:**
```
internal/cli/
  root.go          # existing, unchanged
  setup.go         # NEW: setup subcommand + client-specific logic
  status.go        # NEW: status subcommand
  hook.go          # NEW: hook subcommand handlers
```

### 2. Client MCP Config Formats (Verified)

| Client | Config File | JSON Schema | Confidence |
|--------|-------------|-------------|------------|
| Claude Code | `.claude/settings.json` (project) or `~/.claude/settings.json` (global) | `{ "mcpServers": { "serena": { "command": "serena", "args": [] } } }` | HIGH |
| VS Code | `.vscode/mcp.json` | `{ "servers": { "serena": { "type": "stdio", "command": "serena", "args": [] } } }` | HIGH |
| JetBrains (Junie) | `.junie/mcp/mcp.json` (project) or `~/.junie/mcp/mcp.json` (user) | `{ "mcpServers": { "serena": { "command": "serena", "args": [] } } }` | MEDIUM |

**Implementation:** Pure `encoding/json` with `json.MarshalIndent`. No templating library needed -- these are small deterministic JSON structures.

**Merge strategy:** Read existing file if present, unmarshal to `map[string]any`, merge serena entry, re-marshal. Preserves user's other MCP servers.

### 3. Claude Code Hooks Integration (Verified via Official Docs)

Claude Code hooks (v2.1.114, 26 lifecycle events) use JSON in `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "serena hook session-start",
        "once": true,
        "timeout": 30
      }]
    }],
    "PreToolUse": [{
      "matcher": "mcp__serena__.*",
      "hooks": [{
        "type": "command",
        "command": "serena hook pre-tool-use",
        "timeout": 10
      }]
    }],
    "Stop": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "serena hook stop",
        "timeout": 10
      }]
    }]
  }
}
```

**Hook handler protocol:**
- Stdin: JSON with hook context (event, tool name, arguments)
- Stdout: JSON response (empty = allow, structured = modify/block)
- Exit 0 = success/allow, Exit 2 = block (stderr becomes error message)

**What each hook does:**
- `SessionStart` (once=true): Triggers lazy workspace init, language detection, LS warm-up
- `PreToolUse` (matcher: serena tools): Inject context reminders, validate workspace readiness
- `Stop`: Signal daemon for session cleanup, persist session state

### 4. Health/Status MCP Tool

New skill: `internal/skill/health/` implementing `skill.ToolProvider`:

```go
// Exposes: get_health MCP tool
// Returns: active language servers, indexing progress, workspace capabilities,
//          circuit breaker states, memory pressure level
```

Data sources (all existing, query only):
- `lspool.Pool` -- active workers, languages, circuit states
- `WorkspaceRuntime` -- detected languages, root path
- `langregistry.Registry` -- installed vs available LSes
- Daemon uptime, version from `daemon.go`

### 5. Smart Error Responses (Extend Existing Taxonomy)

The existing `internal/errors` package has a builder pattern with Kind enum. Add suggestion metadata:

```go
// New field in Error struct (internal/errors/errors.go)
type Suggestion struct {
    CorrectTool string `json:"correct_tool,omitempty"`
    CorrectArgs map[string]string `json:"correct_args,omitempty"`
    Reason      string `json:"reason"`
}

// Builder extension
func (b *Builder) WithSuggestion(s Suggestion) *Builder
```

Suggestions are populated by tool-specific validation logic (already inline at 24 kernel tool boundaries). No new deps.

### 6. Progressive Tool Descriptions

Extend profile YAML with description tiers:

```yaml
# profiles/claude-code.yml
tools:
  get_symbol_definition:
    description_brief: "Get symbol definition by name"
    description_detailed: "Find where a symbol is defined. Use name_path for nested symbols (e.g., 'ClassName/method'). Supports substring matching."
    description_tutorial: "Use this when you need to read the source code of a function, class, or method. Provide the symbol name and optionally the file path to narrow scope."
```

The MCP SDK's tool description field is a string set at registration time. Progressive disclosure works by:
1. Starting with `description_brief` in tools/list response
2. Including `description_detailed` in error responses when the agent misuses the tool
3. Offering `description_tutorial` via the health tool when an agent asks for help

No new library needed -- koanf already reads nested YAML, and the profile system already supports description overrides.

### 7. Lazy Workspace Init

Implement as middleware in the existing MCP request pipeline (`internal/mcp/middleware.go`):

```go
// LazyInitMiddleware checks if workspace is initialized before tool execution.
// If not, triggers DetectLanguages + LS warm-up, then proceeds.
func LazyInitMiddleware(kernel *kernel.Kernel) mcp.Middleware
```

The kernel and workspace runtime already support this flow -- `DetectLanguages` + pool acquire. The middleware just makes it automatic on first tool call.

### 8. gRPC Proto Extension (Minimal)

Add to existing `api/proto/serena/v1/serena.proto`:

```protobuf
// Hook notification from CLI -> daemon
rpc NotifyHook(HookRequest) returns (HookResponse);

// Health query from CLI status command
rpc GetHealth(HealthRequest) returns (HealthResponse);
```

This follows the existing forwarder pattern. `protoc` generates the Go code -- no new tooling deps.

## What NOT to Add

| Library | Why Tempting | Why Skip |
|---------|--------------|----------|
| go-enry/go-enry | Language detection for setup | Serena's langregistry already has 52 languages with FileExts + marker file detection. go-enry adds ~15MB binary overhead. |
| charmbracelet/bubbletea | Interactive setup wizard | Agents call `serena setup` non-interactively. Humans get plain text output. |
| charmbracelet/lipgloss | Pretty terminal status | Same -- agents don't see colors. `fmt.Fprintf` suffices. |
| survey/huh | Interactive prompts | Setup must be non-interactive (zero-friction means no prompts). |
| viper | Config file writing | `json.MarshalIndent` + `os.WriteFile` for 3 simple JSON formats. Viper is 10x the complexity. |
| text/template | Config generation | Templates add indirection for trivial JSON structures. Direct struct marshaling is clearer. |
| embed | Template files | No template files needed -- JSON structures built in Go code. |
| fatih/color | Colored CLI output | Agents ignore ANSI. Keep output parseable. |

## Integration Points with Existing Stack

### Cobra CLI Extension (internal/cli/root.go)

```go
rootCmd.AddCommand(newSetupCmd())   // serena setup <client>
rootCmd.AddCommand(newStatusCmd())  // serena status
rootCmd.AddCommand(newHookCmd())    // serena hook <event>
```

### Health Skill Registration (Caddy pattern)

```go
// internal/skill/health/health.go
func init() { skill.Register(&HealthSkill{}) }

// internal/daemon/imports.go -- add blank import
_ "github.com/postfix/serena/internal/skill/health"
```

### Error Taxonomy Extension (internal/errors/)

Add `Suggestion` struct and `WithSuggestion` builder method. Serializes into existing JSON error response. All 24 kernel validation points can optionally attach suggestions.

### Profile YAML Extension (internal/profile/)

Add `description_brief`, `description_detailed`, `description_tutorial` fields to tool YAML schema. The profile loader already uses koanf for nested YAML -- just add struct fields.

### gRPC Service Extension (api/proto/serena/v1/)

Add `NotifyHook` and `GetHealth` RPCs to existing service definition. The forwarder already connects to daemon via gRPC.

## Installation

```bash
# No new dependencies. Build as before:
go build ./cmd/serena

# If proto changes for hook/health RPCs:
protoc --go_out=. --go-grpc_out=. api/proto/serena/v1/serena.proto
```

## Dependency Impact

| Metric | Before (v1.6) | After (v1.7) | Delta |
|--------|---------------|--------------|-------|
| Direct dependencies | 20 | 20 | +0 |
| Binary size | ~49MB | ~49MB | No change |
| CGO required | No | No | No change |
| New packages | 0 | 0 | Zero new external deps |
| New internal packages | -- | +3 | `skill/health`, `cli/setup`, `cli/hook` (within existing dirs) |

## Confidence Assessment

| Decision | Confidence | Rationale |
|----------|------------|-----------|
| Zero new deps | HIGH | Verified go.mod covers all needs; features are integration code |
| Claude Code hooks format | HIGH | Verified via official docs (code.claude.com/docs/en/hooks) |
| VS Code mcp.json format | HIGH | Verified via VS Code official docs |
| JetBrains mcp.json format | MEDIUM | .junie/mcp/mcp.json path may shift as Junie evolves |
| Cobra subcommands | HIGH | Standard pattern, existing dep |
| Skip go-enry | HIGH | langregistry already handles 52 languages |
| gRPC for hooks | HIGH | Existing proto + forwarder IPC pattern proven |
| Health as skill | HIGH | Follows established Caddy-style skill pattern |

## Sources

- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks) -- Official hooks API with 26 lifecycle events, handler types, matcher patterns
- [VS Code MCP Configuration Reference](https://code.visualstudio.com/docs/copilot/reference/mcp-configuration) -- mcp.json schema
- [VS Code MCP Server Setup](https://code.visualstudio.com/docs/copilot/customization/mcp-servers) -- .vscode/mcp.json format
- [JetBrains AI Assistant MCP](https://www.jetbrains.com/help/ai-assistant/configure-an-mcp-server.html) -- MCP config docs
- [Junie MCP Configuration](https://junie.jetbrains.com/docs/junie-cli-mcp-configuration.html) -- .junie/mcp/mcp.json paths
- [go-enry/go-enry](https://github.com/go-enry/go-enry) -- Evaluated and rejected (existing langregistry sufficient)
