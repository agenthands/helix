# Install Guide

Serena is a single Go binary. Install it, point your coding agent at it, and you are ready to go.

## Prerequisites

**Install the binary:**

```bash
go install github.com/postfix/serena/cmd/serena@latest
```

Requires Go 1.25 or later.

Or build from source:

```bash
git clone https://github.com/postfix/serena.git
cd serena
go build ./cmd/serena
```

Verify the binary is in your PATH:

```bash
serena --help
```

If `serena` is not found, ensure `$(go env GOPATH)/bin` is in your PATH.

## Download a prebuilt binary

Prebuilt binaries for darwin/linux/windows × amd64/arm64 are published on the [Releases page](https://github.com/postfix/serena/releases) for every tagged version.

Download and install (Linux amd64 example — substitute `<os>` and `<arch>` for your platform):

```bash
VERSION=v1.9.0   # set to the release tag you want
OS=linux         # one of: linux, darwin, windows
ARCH=amd64       # one of: amd64, arm64

curl -LO "https://github.com/postfix/serena/releases/download/${VERSION}/serena_${OS}_${ARCH}.tar.gz"
tar -xzf "serena_${OS}_${ARCH}.tar.gz"
chmod +x "serena_${OS}_${ARCH}/serena"
sudo mv "serena_${OS}_${ARCH}/serena" /usr/local/bin/serena
serena --version
```

On Windows, download `serena_windows_${ARCH}.zip`, extract with `Expand-Archive`, and place `serena.exe` somewhere on `%PATH%`.

> Note: `--version` from a `go install`-built binary reports `2.0.0-dev`. Only release archives carry full version metadata.

## Verify a release binary

Every release archive ships with a cosign signature (`.sig`) and a Sigstore certificate (`.pem`). The release pipeline uses [cosign keyless signing](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/) — no long-lived signing keys are stored anywhere, and every signature is logged to the public Rekor transparency log.

Verify a downloaded archive end-to-end (Linux amd64 example):

```bash
VERSION=v1.9.0
OS=linux
ARCH=amd64
ARTIFACT="serena_${OS}_${ARCH}.tar.gz"

# Download archive + signature + certificate + checksums
curl -LO "https://github.com/postfix/serena/releases/download/${VERSION}/${ARTIFACT}"
curl -LO "https://github.com/postfix/serena/releases/download/${VERSION}/${ARTIFACT}.sig"
curl -LO "https://github.com/postfix/serena/releases/download/${VERSION}/${ARTIFACT}.pem"
curl -LO "https://github.com/postfix/serena/releases/download/${VERSION}/checksums.txt"

# 1. Verify the cosign signature against the canonical workflow identity
cosign verify-blob \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/postfix/serena/\.github/workflows/release\.yml@refs/tags/v.*$' \
  --signature "${ARTIFACT}.sig" \
  --certificate "${ARTIFACT}.pem" \
  "${ARTIFACT}"

# 2. Verify the SHA-256 checksum
sha256sum --ignore-missing -c checksums.txt
```

Both commands must succeed. The `--certificate-identity-regexp` pins verification to releases built by **this** repo's `release.yml` on a `v*` tag — a binary signed by any other workflow or repo will fail verification.

Requires [cosign](https://docs.sigstore.dev/cosign/installation/) v2.x on PATH.

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
