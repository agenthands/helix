# Phase 41: Install & Contributing - Pattern Map

**Mapped:** 2026-04-23
**Files analyzed:** 2
**Analogs found:** 2 / 2

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `INSTALL.md` | config (documentation) | N/A | `README.md` lines 58-135 (Quick Start + manual config) | exact |
| `CONTRIBUTING.md` | config (documentation) | N/A | `CONTRIBUTING.md` (self, current version) | exact |

## Pattern Assignments

### `INSTALL.md` (documentation rewrite)

**Analog:** `README.md` (Quick Start section) + current `INSTALL.md`

The README.md Quick Start section (lines 58-135) already uses `serena setup <client>` as the primary path with manual config in a `<details>` collapse. INSTALL.md should follow this same structure but with more detail.

**Setup CLI as primary path pattern** (README.md lines 73-84):
```markdown
### Configure Your Client

```bash
serena setup claude-code    # Claude Code
serena setup vscode         # VS Code / Cursor
serena setup jetbrains      # JetBrains IDEs
serena setup gemini-cli     # Gemini CLI
serena setup claude-desktop # Claude Desktop
```

Add `--global` for user-wide registration. Run `serena setup --help` for all options.
```

**Manual config as collapsed secondary pattern** (README.md lines 88-135):
```markdown
<details>
<summary>Manual configuration (without serena setup)</summary>

**Claude Code** (`.claude/settings.json`):
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
...
</details>
```

**HTTP mode as separate section pattern** (current INSTALL.md lines 124-132):
```markdown
### HTTP Mode

For agents that support HTTP-based MCP servers, or for connecting multiple agents to a shared daemon:

```bash
serena --mode=http --http-addr=127.0.0.1:8080
```

Point your agent's MCP client at `http://127.0.0.1:8080`. The daemon persists across client disconnects and shares warm language server caches between sessions.
```

**Verify installation pattern** (current INSTALL.md lines 134-138):
```markdown
## Verify Installation

After configuring your agent, run the `onboard_project` tool from your agent to confirm everything works. This initializes Serena for your workspace and prints a summary of available capabilities.
```

**Opening line tone** (current INSTALL.md line 3):
```markdown
Serena is a single Go binary. Install it, point your coding agent at it, and you are ready to go.
```

**Key structural decisions for rewrite:**
- Section order per D-05: Prerequisites -> Quick Start (setup CLI) -> Manual Configuration -> HTTP Mode -> Verify Installation -> Next Steps
- Client list per D-03 must match `clientRegistry()` in `internal/cli/setup_clients.go` line 37-46: claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic
- Remove per D-04: Codex, OpenCode, Cursor, Antigravity sections (current lines 41-122)
- Setup CLI flags from `internal/cli/setup.go` lines 31-36: `--global`, `--uninstall`, `--skip-install`, `--dry-run`, `--output`, `--no-hooks`
- VS Code uses `"servers"` key NOT `"mcpServers"` (Research pitfall 1)
- Claude Code and Gemini CLI use their own CLIs -- manual section should show CLI commands, not JSON (Research pitfall 4)

---

### `CONTRIBUTING.md` (documentation rewrite)

**Analog:** Current `CONTRIBUTING.md` (self) + `CLAUDE.md` for architecture description

**Opening and scope pattern** (CONTRIBUTING.md lines 1-13):
```markdown
# Contributing to Serena

Thank you for your interest in contributing to Serena! We welcome contributions that improve and extend the project.

## Scope of Contributions

The following types of contributions can be submitted directly via pull requests:

- Isolated additions that extend Serena along existing lines (e.g., adding support for a new language server)
- Small bug fixes
- Documentation improvements

For larger changes, please open an issue first to discuss your ideas with the maintainers.
```

**Development commands table pattern** (CONTRIBUTING.md lines 24-37):
```markdown
## Development Commands

| Command | Description |
|---------|-------------|
| `go build ./cmd/serena` | Build the serena binary |
| `go test ./...` | Run all tests |
| `go vet ./...` | Run static analysis |
| `gofmt -w .` | Format code |
| `make build` | Build via Makefile |
...
```

**Project structure listing pattern** (CONTRIBUTING.md lines 42-63):
```markdown
## Project Structure

Serena uses a 4-layer architecture shipping as a single Go binary:

- `cmd/serena/` -- CLI entry point
- `internal/mcp/` -- MCP runtime (Layer 0)
- `internal/daemon/` -- Persistent supervisor daemon (Layer 0)
...
```

**Integration test section pattern** (CONTRIBUTING.md lines 66-86):
```markdown
## Running Integration Tests

The integration test harness lives in `test/integration/`. It starts a real Serena daemon...

Run all integration tests:

```sh
go test ./test/integration/ -v -timeout 120s
```

Run a specific test:

```sh
go test ./test/integration/ -run TestSymbolRetrieval -v
```

Key details:
- bullet points with specifics
```

**Adding a tool guide pattern** (CONTRIBUTING.md lines 112-129):
```markdown
## Adding a New MCP Tool

1. **Choose the right layer:**
   - Kernel tools go in `internal/kernel/{category}/`
   - Skill tools go in `internal/skill/{name}/`

2. **Implement the tool:**
   ...
```

**Key structural decisions for rewrite:**
- Add missing packages per D-11 (verified list from RESEARCH.md lines 108-137): `internal/cli/`, `internal/errors/`, `internal/fuzzy/`, `internal/kernel/health/`, `internal/kernel/help/`, `internal/repomap/`, `internal/skill/repomap/`, `internal/treesitter/`, `internal/workspace/`
- Fix tool count per D-12: `internal/kernel/fileops/` is **7** not 6 (fuzzy_edit added)
- Add oracle test hierarchy per D-08: 6 layers -- protocol, contract, runtime, scenario, llm, judge (verified at `test/oracle/`)
- Add test harness per D-09: `test/harness/` with runner.go, tools.go, golden.go, fixture.go
- Add `test/oracle/` to project structure per D-13
- Drop `internal/mcp/progressive.go` reference -- file does not exist (Research pitfall 3)

---

## Shared Patterns

### Documentation Tone
**Source:** `README.md`, current `INSTALL.md`, `USAGE.md`
**Apply to:** Both INSTALL.md and CONTRIBUTING.md

Product-aware technical tone (D-14). Short declarative sentences. No marketing fluff but not dry either. Example opening from README.md line 12:
```markdown
* Serena provides essential **semantic code retrieval, editing and refactoring tools** that are akin to an IDE's capabilities,
  operating at the symbol level and exploiting relational structure.
```

Uses `--` (em dash) not `---` or `—` for inline dashes, consistent across all docs.

### Cross-Document Links
**Source:** Current INSTALL.md lines 142-143, README.md line 133
**Apply to:** Both files

```markdown
- [USAGE.md](USAGE.md) -- Configuration, profiles, modes, observability, and performance tuning
- [README.md](README.md) -- Feature overview, architecture, and full tool list
```

### Legacy Python One-Liner
**Source:** Current CONTRIBUTING.md lines 144-146
**Apply to:** Both files (at bottom)

```markdown
## Legacy Python

The `legacy/` directory contains the original Python Serena for reference only. It is not actively developed.
```

### Code Block Formatting
**Source:** All existing docs
**Apply to:** Both files

- Shell commands use triple-backtick `sh` or `bash` fence
- JSON configs use triple-backtick `json` fence
- Inline tool names and paths use backtick quoting: `serena setup`, `get_health`

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | -- | -- | Both files are rewrites of existing files with clear self-analogs |

## Metadata

**Analog search scope:** Repository root (`*.md`), `internal/cli/setup*.go`
**Files scanned:** 6 (INSTALL.md, CONTRIBUTING.md, README.md, USAGE.md, setup.go, setup_clients.go)
**Pattern extraction date:** 2026-04-23
