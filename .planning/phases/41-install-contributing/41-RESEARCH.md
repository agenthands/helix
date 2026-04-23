# Phase 41: Install & Contributing - Research

**Researched:** 2026-04-23
**Domain:** Documentation (INSTALL.md, CONTRIBUTING.md)
**Confidence:** HIGH

## Summary

Phase 41 updates two documentation files -- INSTALL.md and CONTRIBUTING.md -- to accurately reflect the current codebase through v1.7. The work is straightforward documentation restructuring with no code changes, external dependencies, or architectural risk.

INSTALL.md needs restructuring around `serena setup <client>` as the primary path (replacing per-agent manual JSON sections), with the client list matching the actual `clientRegistry()` in `setup_clients.go`. CONTRIBUTING.md needs updated package listings, corrected tool counts, and documentation of the oracle test hierarchy that was added in v1.6-v1.7.

**Primary recommendation:** Treat as two independent document rewrites. Each can be verified by diff against the source code artifacts identified below.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Lead with `serena setup <client>` as the primary installation method
- **D-02:** Manual JSON configs become a secondary section
- **D-03:** Client list must match `clientRegistry()`: claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic
- **D-04:** Remove Codex, OpenCode, Cursor, Antigravity as separate sections
- **D-05:** Section order: Prerequisites -> Quick Start (setup CLI) -> Manual Configuration -> HTTP Mode -> Verify Installation -> Next Steps
- **D-06:** HTTP mode stays as separate section
- **D-07:** Verify Installation references `get_health` tool alongside `onboard_project`
- **D-08:** Document oracle hierarchy: protocol, contract, runtime, scenario, llm, judge
- **D-09:** Document test harness package (`test/harness/`)
- **D-10:** Keep integration test and benchmark sections, update details
- **D-11:** Add missing packages: `internal/repomap/`, `internal/fuzzy/`, `internal/cli/`, `internal/kernel/health/`, `internal/mcp/suggest_lev.go`
- **D-12:** Update tool count references (verify against actual count)
- **D-13:** Add `test/oracle/` directory to project structure
- **D-14:** Maintain product-aware technical tone
- **D-15:** Python legacy as brief one-liner at bottom

### Claude's Discretion
- Exact wording of setup CLI examples and help text
- Whether manual JSON configs use `<details>` collapse or flat secondary section
- Level of detail in oracle test layer descriptions
- Whether to add a "Troubleshooting Installation" subsection

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INST-01 | INSTALL.md reflects current install paths and `serena setup <client>` as primary method | Setup CLI source verified in `internal/cli/setup.go` and `setup_clients.go`; 6 clients in registry confirmed |
| INST-02 | INSTALL.md has accurate MCP configs for all 6 supported clients | Client registrars verified: claude-code (uses `claude mcp add-json`), vscode (`.vscode/mcp.json` with `servers` key), jetbrains (`.junie/mcp/mcp.json`), claude-desktop (platform-specific path), gemini-cli (uses `gemini mcp add`), generic (stdout JSON) |
| CONT-01 | CONTRIBUTING.md reflects current Go codebase structure and dev workflow | Full `internal/` package listing verified; corrected tool counts documented below |
| CONT-02 | CONTRIBUTING.md references current test harness (oracle tests, integration tags, benchmark gates) | Oracle test hierarchy verified: 6 layers with specific test files enumerated |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| INSTALL.md rewrite | Documentation | -- | Pure prose, references CLI source for accuracy |
| CONTRIBUTING.md rewrite | Documentation | -- | Pure prose, references codebase structure for accuracy |

## Standard Stack

No libraries needed. This phase is documentation-only.

## Architecture Patterns

Not applicable -- documentation rewrite phase.

## Verified Source Data

The following data was extracted from the codebase and serves as the authoritative source for documentation accuracy.

### Setup CLI Client Registry [VERIFIED: internal/cli/setup_clients.go]

| Client | Registrar | Config Key | Config Path (project) | Config Path (global) |
|--------|-----------|------------|----------------------|---------------------|
| claude-code | ClaudeCodeRegistrar | N/A (uses `claude mcp add-json`) | via `claude` CLI | via `claude` CLI `--scope user` |
| vscode | VSCodeRegistrar | `servers` (NOT `mcpServers`) | `.vscode/mcp.json` | `~/.config/Code/User/mcp.json` (macOS: `~/Library/Application Support/Code/User/mcp.json`) |
| jetbrains | JetBrainsRegistrar | `mcpServers` | `.junie/mcp/mcp.json` | `~/.junie/mcp/mcp.json` |
| claude-desktop | ClaudeDesktopRegistrar | `mcpServers` | N/A (global only) | macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`, Linux: `~/.config/Claude/claude_desktop_config.json`, Windows: `%APPDATA%/Claude/claude_desktop_config.json` |
| gemini-cli | GeminiCLIRegistrar | N/A (uses `gemini mcp add`) | via `gemini` CLI | via `gemini` CLI `--scope user` |
| generic | GenericRegistrar | `mcpServers` | stdout or `--output` path | same |

**Key pitfall for VS Code:** Uses `"servers"` key, NOT `"mcpServers"`. Also requires `"type": "stdio"` field. [VERIFIED: setup_clients.go line 293, 315]

### Corrected Tool Counts [VERIFIED: kernel and skill source files]

| Category | Package | Count | Tools |
|----------|---------|-------|-------|
| Symbol retrieval | `internal/kernel/symbols/` | 9 | (unchanged) |
| Symbol editing | `internal/kernel/edit/` | 6 | (unchanged) |
| File operations | `internal/kernel/fileops/` | **7** | read_file, create_file, list_directory, find_files, search_in_files, replace_in_file, fuzzy_edit |
| Diagnostics | `internal/kernel/diag/` | 3 | (unchanged) |
| Health | `internal/kernel/health/` | 1 | get_health |
| Help | `internal/kernel/help/` | 1 | get_tool_help |
| Memory | `internal/skill/memory/` | 7 | write_memory, read_memory, list_memories, search_memories, rename_memory, edit_memory, delete_memory |
| Workflow | `internal/skill/workflow/` | 2 | onboard_project, prepare_for_new_conversation |
| RepoMap | `internal/skill/repomap/` | 2 | get_repo_map, get_context |

**Total: ~38 tools** (28 kernel + 10 skill, varies slightly by profile)

**CONTRIBUTING.md currently says:** "6 file operation tools" -- should be **7** (fuzzy_edit was added). [VERIFIED: fileops/tools.go]

### Complete Internal Package Listing [VERIFIED: `ls internal/`]

Current `internal/` packages (alphabetical):
- `internal/cli/` -- CLI commands including setup, status, activate, deactivate, nudge
- `internal/config/` -- 4-layer configuration (Layer 3)
- `internal/daemon/` -- Persistent supervisor daemon (Layer 0)
- `internal/degrade/` -- Graceful degradation
- `internal/errors/` -- Structured error types
- `internal/forwarder/` -- Stdio-to-gRPC proxy (Layer 0)
- `internal/fuzzy/` -- Fuzzy editing strategies (whitespace-normalized, indentation-flexible)
- `internal/kernel/` -- Code intelligence kernel (Layer 1)
- `internal/kernel/diag/` -- 3 diagnostic tools
- `internal/kernel/edit/` -- 6 symbol editing tools (tree-sitter body surgery)
- `internal/kernel/fileops/` -- 7 file operation tools
- `internal/kernel/health/` -- get_health tool
- `internal/kernel/help/` -- get_tool_help tool
- `internal/kernel/jsonrpc/` -- Custom JSON-RPC 2.0 codec for LS communication
- `internal/kernel/lspool/` -- LS worker pool (share-until-dirty, adaptive TTL, circuit breaking)
- `internal/kernel/symbols/` -- 9 symbol retrieval tools
- `internal/langregistry/` -- 52-language embedded registry with YAML override (Layer 2)
- `internal/mcp/` -- MCP runtime, suggest_lev.go (smart error suggestions), lazy_init.go (Layer 0)
- `internal/memory/` -- Memory store, FTS5 index, fsnotify watcher
- `internal/obs/` -- Observability (metrics, tracing, admin listener)
- `internal/profile/` -- 5 agent profiles (Layer 3)
- `internal/repomap/` -- RepoMap subsystem (PageRank-based context selection)
- `internal/skill/` -- Skill plugin system, Caddy-style init() registration (Layer 2)
- `internal/skill/memory/` -- 7 memory tools
- `internal/skill/repomap/` -- 2 RepoMap tools (get_repo_map, get_context)
- `internal/skill/workflow/` -- 2 workflow tools (onboard_project, prepare_for_new_conversation)
- `internal/treesitter/` -- Tree-sitter grammar integration
- `internal/workspace/` -- Workspace key/state

**Missing from current CONTRIBUTING.md:** `internal/cli/`, `internal/errors/`, `internal/fuzzy/`, `internal/kernel/health/`, `internal/kernel/help/`, `internal/repomap/`, `internal/skill/repomap/`, `internal/treesitter/`, `internal/workspace/`

**Note:** CONTEXT.md D-11 mentions `internal/mcp/progressive.go` -- this file does not exist. The progressive tool descriptions feature is likely implemented elsewhere or was renamed. The suggest functionality lives in `internal/mcp/suggest.go` and `internal/mcp/suggest_lev.go`. [VERIFIED: grep found no file matching "progressive" in internal/]

### Oracle Test Hierarchy [VERIFIED: test/oracle/ directory listing]

| Layer | Package | Purpose | Test Files |
|-------|---------|---------|------------|
| protocol | `test/oracle/protocol/` | MCP handshake, reconnect, session isolation, smoke, tools listing | 5 test files |
| contract | `test/oracle/contract/` | Schema validation, golden output, error contracts, selectability | 4 test files + testdata |
| runtime | `test/oracle/runtime/` | Degraded start, pool stress, shutdown, deferred errors | 4 test files + testdata |
| scenario | `test/oracle/scenario/` | Multi-language end-to-end scenarios (Go, Python, TS, Java, Rust, PHP, C++, JS, Swift, Zig, SQL, Markdown, fuzzy edit, collision, degraded, polyglot, profile, repomap, unsupported, cache persistence) | 20 test files |
| llm | `test/oracle/llm/` | LLM-driven disambiguation, interpretation, selection | 3 test files + testdata |
| judge | `test/oracle/judge/` | Automated scoring with rubrics, aggregation | 1 test file + testdata |

### Test Harness [VERIFIED: test/harness/ directory]

Files: `runner.go`, `tools.go`, `golden.go`, `fixture.go`, `doc.go`, `runner_test.go`

Provides: Runner (starts daemon + exercises MCP round-trips), tool helpers, golden file comparison, fixture management.

### Integration Tests [VERIFIED: test/integration/ directory]

23 test files covering: symbols, edit, fileops, diag, errors, memory, workflow, profiles, modes, concurrency, trace propagation, trace shutdown, smoke HTTP, plus language-specific tests (Go/Python/TypeScript/Java/Rust).

### Benchmark Suite [VERIFIED: test/bench/ directory]

Files: tools_bench, lsp_index_bench, memory_bench, metrics_bench, obs_bench, tracing_bench, heap_snapshot, fullrepo_smoke, tools_descriptions, tools_manifest, tools_integration, plus baselines directory and pprof/rss helpers.

### Go Version [VERIFIED: go.mod]

`go 1.25.1` -- INSTALL.md currently says "Go 1.25 or later" which is acceptable.

### Setup CLI Usage [VERIFIED: internal/cli/setup.go]

```
serena setup [client]
```

Flags: `--global`, `--uninstall`, `--skip-install`, `--dry-run`, `--output` (generic only), `--no-hooks` (Claude Code only)

Valid clients: claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic

## Don't Hand-Roll

Not applicable -- documentation phase.

## Common Pitfalls

### Pitfall 1: VS Code uses "servers" not "mcpServers"
**What goes wrong:** If manual config examples show `mcpServers` for VS Code, they won't work.
**How to avoid:** The setup CLI handles this automatically. Any manual config section must use `"servers"` key and include `"type": "stdio"`.
**Warning signs:** VS Code users reporting Serena not appearing as MCP server.

### Pitfall 2: Stale tool counts
**What goes wrong:** CONTRIBUTING.md says "6 file operation tools" but there are now 7 (fuzzy_edit added).
**How to avoid:** Use the verified counts from this research. Don't copy old numbers.

### Pitfall 3: progressive.go reference
**What goes wrong:** CONTEXT.md D-11 references `internal/mcp/progressive.go` which doesn't exist.
**How to avoid:** The suggest functionality is in `suggest.go` and `suggest_lev.go`. Progressive descriptions may be implemented as middleware or within the registry. Don't reference a non-existent file.

### Pitfall 4: Claude Code and Gemini CLI use their own CLIs
**What goes wrong:** Showing manual JSON config for Claude Code or Gemini CLI when `serena setup` actually shells out to `claude mcp add-json` and `gemini mcp add` respectively.
**How to avoid:** The manual config section should only show JSON for clients that use file-based config (vscode, jetbrains, claude-desktop, generic). Claude Code and Gemini CLI should reference their native CLI commands.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Total tool count is ~38 | Verified Source Data | Low -- counts are verified from source, but profiles may filter some tools |

**Most claims in this research were verified directly from source code.**

## Open Questions

1. **progressive.go doesn't exist**
   - What we know: CONTEXT.md D-11 mentions `internal/mcp/progressive.go` but the file doesn't exist. Progressive tool descriptions may be implemented as middleware within `server.go` or `middleware.go`.
   - Recommendation: Drop this specific file reference from CONTRIBUTING.md. Mention progressive descriptions as a feature of the MCP layer without pointing to a specific file.

2. **Manual config for CLI-based clients**
   - What we know: Claude Code and Gemini CLI registration works through their respective CLIs, not file writes. There's no simple JSON config a user can paste for these two.
   - Recommendation: In the manual config section, only provide JSON examples for file-based clients (vscode, jetbrains, claude-desktop, generic). For Claude Code and Gemini CLI, show the CLI commands instead.

## Sources

### Primary (HIGH confidence)
- `internal/cli/setup_clients.go` -- Authoritative client registry, all 6 registrars with config paths
- `internal/cli/setup.go` -- Setup command flags and usage
- `internal/kernel/*/tools.go` -- Authoritative tool counts per category
- `internal/skill/*/skill.go` -- Authoritative skill tool definitions
- `test/oracle/` -- Verified 6-layer oracle hierarchy
- `test/harness/` -- Verified harness package contents
- `go.mod` -- Go version requirement (1.25.1)

## Metadata

**Confidence breakdown:**
- Source data accuracy: HIGH -- all numbers verified from source code
- Documentation structure: HIGH -- decisions are clear and comprehensive
- Pitfalls: HIGH -- identified from direct code inspection

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (stable -- documentation phase with locked codebase)
