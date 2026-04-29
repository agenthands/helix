# Install Guide

Serena is a single Go binary. Install it, point your coding agent at it, and you are ready to go.

> **Note (pre-release):** No signed releases have been cut yet. The `minisign.pub`
> committed to `main` is a `PLACEHOLDER` and the verification recipe below will
> not succeed until the maintainer rotates in the real public key and pushes the
> first `v*` tag (release CI fails closed on the placeholder). Track this in
> `.planning/phases/51-packaging-goreleaser/deferred-items.md` (DEF-51-03).
> Until then, prefer **Build from source** below.

## Install (pre-built binary)

Pre-built binaries for darwin/linux/windows on amd64/arm64 are published on the [Releases page](https://github.com/agenthands/helix/releases) for every tagged version. Each archive ships with a SHA-256 checksum and a minisign signature.

The verification recipe below downloads an archive, verifies the checksum, verifies the cryptographic signature, and extracts the binary -- a single block you can copy and paste end-to-end. Fill in `VERSION`, `OS`, and `ARCH` for your platform.

```bash
VERSION=v1.9.0
OS=linux
ARCH=amd64

# Download archive, signatures, checksums, and (one-time) the project public key
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz.minisig
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt.minisig
curl -LO https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub  # one-time

# Verify the checksums file is signed by the project (catches a substituted checksums.txt)
minisign -V -p minisign.pub -m checksums.txt

# Verify the archive's sha256 matches the (now-trusted) checksums.txt entry --
# fails loud on typos (and avoids the macOS sha256sum/shasum split since we
# operate on a single file).
EXPECTED_HASH=$(grep "serena_${VERSION}_${OS}_${ARCH}.tar.gz" checksums.txt | awk '{print $1}')
ACTUAL_HASH=$(sha256sum "serena_${VERSION}_${OS}_${ARCH}.tar.gz" 2>/dev/null | awk '{print $1}')
[ -z "$ACTUAL_HASH" ] && ACTUAL_HASH=$(shasum -a 256 "serena_${VERSION}_${OS}_${ARCH}.tar.gz" | awk '{print $1}')
if [ -z "$EXPECTED_HASH" ] || [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
  echo "checksum FAILED: expected=$EXPECTED_HASH actual=$ACTUAL_HASH"; exit 1
fi
echo "checksum OK: $ACTUAL_HASH"

# Verify the archive's minisign signature (independent of checksums.txt)
minisign -V -p minisign.pub -m serena_${VERSION}_${OS}_${ARCH}.tar.gz

# Extract and run
tar -xzf serena_${VERSION}_${OS}_${ARCH}.tar.gz
./serena --help
```

**macOS users:** install minisign with `brew install minisign`. The checksum block above already falls back to `shasum -a 256` when `sha256sum` is unavailable, so the recipe runs as-is. Everything else is identical.

Supported `OS` values: `darwin`, `linux`, `windows`. Supported `ARCH` values: `amd64`, `arm64`. Modern Windows (10 1803+) ships `tar` in System32, so the same `.tar.gz` archive extracts on Windows without third-party tools.

## Build from source

If you prefer to build the binary yourself:

```bash
git clone https://github.com/agenthands/helix.git
cd helix
go build ./cmd/serena
```

Requires Go 1.25 or later. Verify the binary is in your PATH:

```bash
serena --help
```

If `serena` is not found, ensure `$(go env GOPATH)/bin` is in your PATH.

## Quick Start

The fastest way to configure Serena for your coding agent is the setup CLI:

```bash
serena setup claude-code    # Claude Code
serena setup vscode         # VS Code
serena setup jetbrains      # JetBrains IDEs
serena setup gemini-cli     # Gemini CLI
serena setup claude-desktop # Claude Desktop
serena setup opencode       # OpenCode
serena setup generic        # Generic MCP client
```

Add `--global` for user-wide registration. Other flags:

- `--uninstall` -- remove Serena registration from the client
- `--dry-run` -- preview what would happen without making changes
- `--no-hooks` -- skip hook installation (Claude Code only)

Run `serena setup --help` for all options.

Serena uses **lazy initialization** -- workspaces are configured on first tool call, so there is no upfront indexing delay.

## Manual Configuration

<details>
<summary>Manual configuration (without serena setup)</summary>

For clients that use their own CLI for MCP registration, run the CLI command directly. For file-based clients, add the JSON config to the appropriate file.

### Claude Code

Claude Code uses the `claude` CLI for MCP registration:

```bash
claude mcp add-json serena '{"command":"serena","args":["--mode=stdio"]}'
```

Add `--scope user` for global registration. Or just use `serena setup claude-code`.

### Gemini CLI

Gemini CLI uses the `gemini` CLI for MCP registration:

```bash
gemini mcp add --scope project -t stdio serena serena -- --mode=stdio
```

Add `--scope user` for global registration. Or just use `serena setup gemini-cli`.

### VS Code

Add to `.vscode/mcp.json` (project) or `~/.config/Code/User/mcp.json` (global):

```json
{
  "servers": {
    "serena": {
      "type": "stdio",
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```

**Note:** VS Code uses the `"servers"` key, not `"mcpServers"`. The `"type": "stdio"` field is required.

### JetBrains

Add to `.junie/mcp/mcp.json` (project) or `~/.junie/mcp/mcp.json` (global):

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

### Claude Desktop

Add to the platform-specific config file:

- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Linux:** `~/.config/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

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

**Note:** OpenCode uses the `"mcp"` key (not `"mcpServers"`), and `"command"` is an array.

### Cursor

Add to `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global):

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

Use absolute paths for the command if `serena` is not in your PATH.

### Generic (any MCP client)

Use the `"mcpServers"` format:

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

You can also generate this with `serena setup generic`, or write it to a file with `serena setup generic --output config.json`.

</details>

## HTTP Mode

For shared daemon or multi-agent scenarios, run Serena in HTTP mode:

```bash
serena --mode=http --http-addr=127.0.0.1:8080
```

Point your agent's MCP client at `http://127.0.0.1:8080`. The daemon persists across client disconnects and shares warm language server caches between sessions.

## Verify Installation

After configuring your agent, use these tools to confirm everything works:

- **`get_health`** -- quick check that the Serena daemon is running and language servers are healthy
- **`onboard_project`** -- full workspace initialization; prints a summary of detected languages and available tools

Expected behavior: Serena starts a background daemon, launches a language server for your project's primary language, and returns a project summary.

## Next Steps

- [USAGE.md](USAGE.md) -- Configuration, profiles, modes, observability, and performance tuning
- [README.md](README.md) -- Feature overview, architecture, and full tool list

## Legacy Python

The `legacy/` directory contains the original Python Serena for reference only. It is not actively developed.
