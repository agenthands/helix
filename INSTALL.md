# Install Guide

Helix is a single Go binary. Install it, point your coding agent at it, and you are ready to go.

> **Note (pre-release):** No signed releases have been cut yet. The `minisign.pub`
> committed to `main` is a `PLACEHOLDER` and the verification recipe below will
> not succeed until the maintainer rotates in the real public key and pushes the
> first `v*` tag (release CI fails closed on the placeholder). Track this in
> `.planning/phases/51-packaging-goreleaser/deferred-items.md` (DEF-51-03).
> Until then, prefer **Build from source** below.

## Install (pre-built binary)

Pre-built binaries for darwin/linux/windows on amd64/arm64 are published on the [Releases page](https://github.com/agenthands/helix/releases) for every tagged version. Archives follow the naming convention `helix_v<version>_<os>_<arch>.tar.gz` (for example `helix_v1.9.0_linux_amd64.tar.gz`); each archive ships with a SHA-256 checksum and a minisign signature.

The verification recipe below downloads an archive, verifies the checksum, verifies the cryptographic signature, and extracts the binary -- a single block you can copy and paste end-to-end. Fill in `VERSION`, `OS`, and `ARCH` for your platform.

```bash
VERSION=v1.9.0
OS=linux
ARCH=amd64

# Download archive, signatures, checksums, and (one-time) the project public key
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/helix_${VERSION}_${OS}_${ARCH}.tar.gz
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/helix_${VERSION}_${OS}_${ARCH}.tar.gz.minisig
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt.minisig
curl -LO https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub  # one-time

# Verify the checksums file is signed by the project (catches a substituted checksums.txt)
minisign -V -p minisign.pub -m checksums.txt

# Verify the archive's sha256 matches the (now-trusted) checksums.txt entry --
# fails loud on typos (and avoids the macOS sha256sum/shasum split since we
# operate on a single file).
EXPECTED_HASH=$(grep "helix_${VERSION}_${OS}_${ARCH}.tar.gz" checksums.txt | awk '{print $1}')
ACTUAL_HASH=$(sha256sum "helix_${VERSION}_${OS}_${ARCH}.tar.gz" 2>/dev/null | awk '{print $1}')
[ -z "$ACTUAL_HASH" ] && ACTUAL_HASH=$(shasum -a 256 "helix_${VERSION}_${OS}_${ARCH}.tar.gz" | awk '{print $1}')
if [ -z "$EXPECTED_HASH" ] || [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
  echo "checksum FAILED: expected=$EXPECTED_HASH actual=$ACTUAL_HASH"; exit 1
fi
echo "checksum OK: $ACTUAL_HASH"

# Verify the archive's minisign signature (independent of checksums.txt)
minisign -V -p minisign.pub -m helix_${VERSION}_${OS}_${ARCH}.tar.gz

# Extract and run
tar -xzf helix_${VERSION}_${OS}_${ARCH}.tar.gz
./helix --help
```

**macOS users:** install minisign with `brew install minisign`. The checksum block above already falls back to `shasum -a 256` when `sha256sum` is unavailable, so the recipe runs as-is. Everything else is identical.

Supported `OS` values: `darwin`, `linux`, `windows`. Supported `ARCH` values: `amd64`, `arm64`. Modern Windows (10 1803+) ships `tar` in System32, so the same `.tar.gz` archive extracts on Windows without third-party tools.

## Build from source

If you prefer to build the binary yourself:

```bash
git clone https://github.com/agenthands/helix.git
cd helix
go build ./cmd/helix
```

Requires Go 1.25 or later. Verify the binary is in your PATH:

```bash
helix --help
```

If `helix` is not found, ensure `$(go env GOPATH)/bin` is in your PATH.

## Quick Start

The fastest way to configure Helix for your coding agent is the setup CLI:

```bash
helix setup claude-code    # Claude Code
helix setup vscode         # VS Code
helix setup jetbrains      # JetBrains IDEs
helix setup gemini-cli     # Gemini CLI
helix setup claude-desktop # Claude Desktop
helix setup opencode       # OpenCode
helix setup generic        # Generic MCP client
```

Add `--global` for user-wide registration. Other flags:

- `--uninstall` -- remove Helix registration from the client
- `--dry-run` -- preview what would happen without making changes
- `--no-hooks` -- skip hook installation (Claude Code only)

Run `helix setup --help` for all options.

Helix uses **lazy initialization** -- workspaces are configured on first tool call, so there is no upfront indexing delay.

## Manual Configuration

<details>
<summary>Manual configuration (without helix setup)</summary>

For clients that use their own CLI for MCP registration, run the CLI command directly. For file-based clients, add the JSON config to the appropriate file.

### Claude Code

Claude Code uses the `claude` CLI for MCP registration:

```bash
claude mcp add-json helix '{"command":"helix","args":["--mode=stdio"]}'
```

Add `--scope user` for global registration. Or just use `helix setup claude-code`.

### Gemini CLI

Gemini CLI uses the `gemini` CLI for MCP registration:

```bash
gemini mcp add --scope project -t stdio helix helix -- --mode=stdio
```

Add `--scope user` for global registration. Or just use `helix setup gemini-cli`.

### VS Code

Add to `.vscode/mcp.json` (project) or `~/.config/Code/User/mcp.json` (global):

```json
{
  "servers": {
    "helix": {
      "type": "stdio",
      "command": "helix",
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
    "helix": {
      "command": "helix",
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
    "helix": {
      "command": "helix",
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
    "helix": {
      "command": "helix",
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
    "helix": {
      "type": "local",
      "command": ["helix", "--mode=stdio"],
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
    "helix": {
      "command": "helix",
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
    "helix": {
      "command": "helix",
      "args": ["--mode=stdio"]
    }
  }
}
```

Use absolute paths for the command if `helix` is not in your PATH.

### Generic (any MCP client)

Use the `"mcpServers"` format:

```json
{
  "mcpServers": {
    "helix": {
      "command": "helix",
      "args": ["--mode=stdio"]
    }
  }
}
```

You can also generate this with `helix setup generic`, or write it to a file with `helix setup generic --output config.json`.

</details>

## HTTP Mode

For shared daemon or multi-agent scenarios, run Helix in HTTP mode:

```bash
helix --mode=http --http-addr=127.0.0.1:8080
```

Point your agent's MCP client at `http://127.0.0.1:8080`. The daemon persists across client disconnects and shares warm language server caches between sessions.

## Upgrading

Helix can upgrade itself in place. Two verbs (modeled after `apt update` / `apt upgrade`):

### Check for a newer release (read-only)

```sh
helix update
```

Prints current and latest versions plus the release notes from GitHub. No filesystem mutation; safe to run anytime. Honors `GITHUB_TOKEN` if set (recommended in CI to avoid the 60 req/hr unauthenticated rate limit).

### Install the latest release

```sh
helix upgrade
```

Performs in order: daemon-detect → permission probe → API fetch → semver compare → archive download → minisign verification → extract → atomic swap → re-launch. Exits cleanly with a clear message in any of these cases:

- **Install path not writable:** prints the exact `sudo helix upgrade` re-invocation. (No internal sudo prompt.)
- **Already up to date** (or remote latest is older than your installed version): exits 0 with "already up to date". There is no `--force-downgrade` flag — go to GitHub Releases manually and use the verification recipe above for an older version.
- **Running as a daemon child:** prints "restart the daemon manually" and exits 0. Stop the daemon first.
- **Tampered or wrong-key signature:** rejected with a single canonical "signature verification FAILED" error message; the staged download is retained for postmortem inspection.

### Flags

| Flag | Purpose |
|------|---------|
| `--prerelease` | Include `v*-rc*` / `v*-beta*` / `v*-alpha*` tags. Defaults to stable-only. |
| `--version vX.Y.Z` | Pin to a specific version. Overrides `--prerelease`. |
| `--check` | Alias for `helix update`. Functionally identical. |
| `--dry-run` | Download + verify but skip swap and re-launch. Useful for CI verification. |

### Rate limits

The GitHub Releases API permits 60 requests/hour per IP for unauthenticated callers. If `helix update` or `helix upgrade` returns a "github API rate limited" error, set `GITHUB_TOKEN` (a fine-grained personal access token with `public_repo` scope is sufficient) and retry. Authenticated calls get 5000/hour:

```sh
export GITHUB_TOKEN=ghp_...
helix update
```

### Manual upgrade (alternative)

If you prefer not to use the in-binary upgrade, the Phase 51 verification recipe at the top of this file still works — fetch the archive + signature + checksums from the GitHub Releases page, verify with minisign, and replace your binary by hand.

## Verify Installation

After configuring your agent, use these tools to confirm everything works:

- **`get_health`** -- quick check that the Helix daemon is running and language servers are healthy
- **`onboard_project`** -- full workspace initialization; prints a summary of detected languages and available tools

Expected behavior: Helix starts a background daemon, launches a language server for your project's primary language, and returns a project summary.

## Next Steps

- [USAGE.md](USAGE.md) -- Configuration, profiles, modes, observability, and performance tuning
- [README.md](README.md) -- Feature overview, architecture, and full tool list

## Legacy Python

The `legacy/` directory contains the original Python Serena for reference only. It is not actively developed.
