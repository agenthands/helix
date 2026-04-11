# Install Guide

Serena is a single Go binary. Install it, point your coding agent at it, and you are ready to go.

## Prerequisites

**Install the binary:**

```bash
go install github.com/postfix/serena/cmd/serena@latest
```

Requires Go 1.25 or later. After installation, verify the binary is in your PATH:

```bash
serena --help
```

If `serena` is not found, ensure `$(go env GOPATH)/bin` is in your PATH.

## Agent Setup

Each coding agent has its own MCP configuration format. Pick the section for your agent below.

### Claude Code

Add to `.claude/settings.json` (project-level) or `~/.claude/settings.json` (global):

```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```

### Codex

Add to `.codex/config.json`:

```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio", "--profile=codex"]
    }
  }
}
```

The `--profile=codex` flag loads a tool set tuned for Codex's capabilities.

### OpenCode

Add to `opencode.json` (project root) or `~/.config/opencode/opencode.json` (global):

```json
{
  "mcp": {
    "serena": {
      "type": "local",
      "command": ["serena", "--mode=stdio"],
      "enabled": true
    }
  }
}
```

Note: OpenCode uses the `"mcp"` key (not `"mcpServers"`), and `"command"` is an array rather than a string.

### Cursor

Add to `.cursor/mcp.json` (project-level) or `~/.cursor/mcp.json` (global):

```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio", "--profile=ide-assistant"]
    }
  }
}
```

Use absolute paths if `serena` is not in your PATH (e.g., `"/home/user/go/bin/serena"`).

### Gemini CLI

Add to `~/.gemini/settings.json` (global) or `.gemini/settings.json` (project-level):

```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```

### Antigravity

Open the Agent Panel, click "..." > MCP Servers > Manage > Edit configuration, and add:

```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```

Use absolute paths for the command. The config file location varies by platform.

### HTTP Mode

For agents that support HTTP-based MCP servers, or for connecting multiple agents to a shared daemon:

```bash
serena --mode=http --http-addr=127.0.0.1:8080
```

Point your agent's MCP client at `http://127.0.0.1:8080`. The daemon persists across client disconnects and shares warm language server caches between sessions.

## Verify Installation

After configuring your agent, run the `onboard_project` tool from your agent to confirm everything works. This initializes Serena for your workspace and prints a summary of available capabilities.

Expected behavior: Serena starts a background daemon, launches a language server for your project's primary language, and returns a project summary with detected languages and available tools.

## Next Steps

- [USAGE.md](USAGE.md) -- Configuration, profiles, modes, observability, and performance tuning
- [README.md](README.md) -- Feature overview, architecture, and full tool list
