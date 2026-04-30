# MCP Installation instructions

This document is mainly used as instructions for AI-assistants like Cline and others that try to do an automatic install based on freeform instructions.

Helix ships as a single, self-contained Go binary. There is no `uv`/Python step, no clone-and-run workflow, and no template `project.yml` to copy. Pre-built binaries are published on the [Releases page](https://github.com/agenthands/helix/releases) for darwin/linux/windows on amd64/arm64.

## Recommended path: `helix setup <client>`

After installing the binary (see [INSTALL.md](INSTALL.md)), run the one-command setup:

```bash
helix setup claude-code      # Claude Code
helix setup vscode           # VS Code
helix setup jetbrains        # JetBrains IDEs
helix setup gemini-cli       # Gemini CLI
helix setup claude-desktop   # Claude Desktop
helix setup opencode         # OpenCode
helix setup generic          # any other MCP client
```

This auto-detects project languages, pre-installs language servers via Helix's three-tier installer, registers the MCP server with the chosen client, and (for Claude Code) installs session hooks. Helix uses **lazy initialization** — workspaces are configured on first tool call, so there is no upfront indexing delay.

## Manual MCP registration

For clients without a CLI integration, add the JSON entry directly:

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

Replace `helix` with an absolute path if the binary is not on `$PATH`. See [INSTALL.md](INSTALL.md) for per-client config-file locations and the manual verification recipe.

## Upgrading

```bash
helix update    # read-only check
helix upgrade   # in-place upgrade with minisign signature verification
```

See [INSTALL.md](INSTALL.md) > **Upgrading** for full flag list and `GITHUB_TOKEN` rate-limit guidance.

## v1.8 → v1.9 migration note

Helix was renamed from its prior product name at v1.9. If your existing MCP client config registers the old binary name, the registration silently stops working — re-run `helix setup <client>` to register under the new identity. See [CHANGELOG.md](CHANGELOG.md) > **v1.9 Breaking Changes** for the full migration story.
