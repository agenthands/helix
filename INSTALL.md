# Install Guide

Helix is a single Go binary. Install it, point your coding agent at it, and you are ready to go.

> **Note (pre-release):** No signed releases have been cut yet. The
> verification recipe below will not succeed until the maintainer pushes the
> first `v*` tag from the cosign-keyless release workflow. Until then, prefer
> **Build from source** below.

## Install (pre-built binary)

Pre-built binaries for darwin/linux/windows on amd64/arm64 are published on the [Releases page](https://github.com/agenthands/helix/releases) for every tagged version. Archives follow the naming convention `helix_v<version>_<os>_<arch>.tar.gz` (for example `helix_v1.9.0_linux_amd64.tar.gz`); each archive ships with a SHA-256 checksum and a sigstore cosign-keyless signature bundle.

### Verifying release artifacts

Helix releases are signed via [sigstore cosign keyless](https://docs.sigstore.dev/cosign/verifying/verify/) — the GitHub Actions release workflow exchanges its OIDC token for a short-lived Fulcio certificate, signs the archive with that cert, and submits the signature to Rekor for transparency-log inclusion. There is no long-lived signing key. Verification requires only the [cosign CLI](https://github.com/sigstore/cosign/releases) (v2.4+) on your `$PATH`.

The recipe below downloads an archive plus its `.sigstore.json` bundle, verifies the cosign signature against the pinned identity policy (GitHub Actions OIDC issuer + agenthands/helix release.yml SAN), verifies the SHA-256 checksum, and extracts the binary. Fill in `VERSION`, `OS`, and `ARCH` for your platform.

```sh
VERSION=v1.9.0
OS=linux
ARCH=amd64

# Download archive, sigstore bundle, and checksums
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/helix_${VERSION}_${OS}_${ARCH}.tar.gz
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/helix_${VERSION}_${OS}_${ARCH}.tar.gz.sigstore.json
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt.sigstore.json

# Verify the archive's sigstore bundle. The identity-pinning regex must
# match the verifier's pinned policy byte-for-byte (security invariant).
cosign verify-blob \
  --bundle helix_${VERSION}_${OS}_${ARCH}.tar.gz.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v\d+\.\d+\.\d+(-rc\d+|-beta\d+|-alpha\d+)?$' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  helix_${VERSION}_${OS}_${ARCH}.tar.gz

# Defense-in-depth: verify checksums.txt is also signed (catches a
# substituted checksums.txt) and that the archive's SHA matches the
# (now-trusted) checksums.txt entry.
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v\d+\.\d+\.\d+(-rc\d+|-beta\d+|-alpha\d+)?$' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

EXPECTED_HASH=$(grep "helix_${VERSION}_${OS}_${ARCH}.tar.gz" checksums.txt | awk '{print $1}')
ACTUAL_HASH=$(sha256sum "helix_${VERSION}_${OS}_${ARCH}.tar.gz" 2>/dev/null | awk '{print $1}')
[ -z "$ACTUAL_HASH" ] && ACTUAL_HASH=$(shasum -a 256 "helix_${VERSION}_${OS}_${ARCH}.tar.gz" | awk '{print $1}')
if [ -z "$EXPECTED_HASH" ] || [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
  echo "checksum FAILED: expected=$EXPECTED_HASH actual=$ACTUAL_HASH"; exit 1
fi
echo "checksum OK: $ACTUAL_HASH"

# Extract and run
tar -xzf helix_${VERSION}_${OS}_${ARCH}.tar.gz
# v1.10.7 and earlier: archive packed the binary at mode 0644; if `./helix`
# returns "permission denied", run `chmod +x helix` first. Fixed from v1.10.8.
./helix --help
```

**macOS users:** install cosign with `brew install cosign`. The checksum block above already falls back to `shasum -a 256` when `sha256sum` is unavailable, so the recipe runs as-is. Everything else is identical.

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

## macOS Gatekeeper workaround

Helix's darwin binaries are signed with [Sigstore](https://www.sigstore.dev/) cosign keyless attestation but are **not signed with an Apple Developer ID** and **not notarized** through Apple's notary service. On first launch, macOS Gatekeeper will block the binary with an error like:

> "helix" can't be opened because Apple cannot check it for malicious software.

Or:

> "helix" is damaged and can't be opened. You should move it to the Trash.

**Workaround (one-time per binary):** use the right-click → Open Gatekeeper-bypass.

1. Open Finder and navigate to the extracted `helix` binary.
2. **Right-click** (or Control-click) on the `helix` binary.
3. Select **Open** from the context menu.
4. macOS will prompt: "Are you sure you want to open it?" — click **Open**.

After this one-time approval, the binary runs without further Gatekeeper prompts.

**Alternative (terminal):**

Clear the Gatekeeper quarantine flag from a terminal:

```bash
xattr -d com.apple.quarantine /path/to/helix
```

**Why this is required:**

Apple Developer ID signing requires an Apple Developer Program membership ($99/year) and Apple-credential management infrastructure. Phase 59.1 is scoped to the CGO=1 build pipeline; signing/notarization is out-of-scope. The Sigstore cosign keyless attestation that Helix DOES apply works on all platforms uniformly (linux, windows, darwin) and is verified by `helix upgrade` against the public-good Sigstore TUF root.

Apple Developer ID signing + notarization is tracked as `DEF-59-NOTARIZE` in `.planning/deferred-items.md`. It may land in a future v1.10.x or v1.11+ release.

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

Performs in order: daemon-detect → permission probe → API fetch → semver compare → archive download → cosign bundle verification → extract → atomic swap → re-launch. Exits cleanly with a clear message in any of these cases:

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

If you prefer not to use the in-binary upgrade, the verification recipe at the top of this file still works — fetch the archive + sigstore bundle + checksums from the GitHub Releases page, verify with cosign, and replace your binary by hand.

## Verify Installation

After configuring your agent, use these tools to confirm everything works:

- **`get_health`** -- quick check that the Helix daemon is running and language servers are healthy
- **`onboard_project`** -- full workspace initialization; prints a summary of detected languages and available tools

Expected behavior: Helix starts a background daemon, launches a language server for your project's primary language, and returns a project summary.

## Next Steps

- [USAGE.md](USAGE.md) -- Configuration, profiles, modes, observability, and performance tuning
- [README.md](README.md) -- Feature overview, architecture, and full tool list
