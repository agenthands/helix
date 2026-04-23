# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Go Development Commands

- `go build ./cmd/serena` - Build the serena binary
- `go test ./...` - Run all Go tests
- `go vet ./...` - Run Go vet
- `gofmt -w .` - Format Go code
- `make build` - Build via Makefile
- `make test` - Run tests via Makefile

**Always run go vet and go test before completing any Go task.**

## Legacy Python Commands (run from legacy/ directory)

- `cd legacy && uv run poe format` - Format Python code (RUFF)
- `cd legacy && uv run poe type-check` - Run mypy type checking
- `cd legacy && uv run poe test` - Run Python tests with default markers (excludes java/rust)
- `cd legacy && uv run poe test -m "python or go"` - Run specific language tests
- `cd legacy && uv run poe lint` - Check Python code style without fixing

**Test Markers:**
Available pytest markers for selective testing:
- `python`, `go`, `java`, `rust`, `typescript`, `vue`, `php`, `perl`, `powershell`, `csharp`, `elixir`, `terraform`, `clojure`, `swift`, `bash`, `ruby`, `ruby_solargraph`
- `snapshot` - for symbolic editing operation tests

## Project

**Serena** — The IDE for your coding agent. A Go-native code intelligence platform for MCP.

Serena provides 41+ MCP tools for semantic code retrieval, editing, and refactoring across 52 languages via LSP. It ships as a **single Go binary** with no Python, Docker, or runtime dependencies, running as a **persistent daemon** that keeps language servers warm between agent sessions.

Targets coding agents (Claude Code, Codex, Gemini CLI, IDE assistants) that need symbol-level operations — go-to-definition, find references, rename across files, replace symbol body, blast-radius analysis — backed by real language servers with warm persistent caching, a ranked RepoMap for structural context, and fuzzy editing that tolerates LLM output drift.

**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, serves ranked structural context on demand, and exposes semantic code operations as agent tools — all from a single binary with one-command client setup.

**Legacy reference:** The `legacy/` directory contains the original Python Serena as a read-only reference. All active development is in Go. Serena is a standalone Go product originally inspired by Python Serena, not a port or rewrite.

## Architecture

4-layer architecture shipping as a single Go binary:

### Layer 0: MCP Runtime
- `internal/mcp/` -- MCP server with official Go SDK, tool registry, structured errors, profile filtering middleware
- `internal/daemon/` -- Persistent supervisor daemon with errgroup orchestration, signal-first lifecycle
- `internal/forwarder/` -- Stdio-to-gRPC proxy with auto-start
- `api/proto/serena/v1/` -- gRPC IPC between forwarder and daemon
- Transports: stdio (via forwarder), Streamable HTTP (direct)

### Layer 1: Code Intelligence Kernel
- `internal/kernel/` -- Kernel orchestrator, workspace runtime, language detection
- `internal/kernel/lspool/` -- LS worker pool: share-until-dirty, adaptive TTL, circuit breaking, pressure eviction
- `internal/kernel/symbols/` -- 9 symbol retrieval tools (definition, references, hover, implementations, call/type hierarchy, blast radius)
- `internal/kernel/edit/` -- 6 symbol editing tools with tree-sitter body surgery
- `internal/kernel/fileops/` -- 7 file operation tools (read, write, list, find, search, replace, fuzzy_edit)
- `internal/kernel/diag/` -- 3 diagnostic tools (diagnostics, code actions, formatting)
- `internal/kernel/jsonrpc/` -- Custom JSON-RPC 2.0 codec for LS communication
- `internal/kernel/health/` -- `get_health` MCP tool returning per-workspace LS status, capabilities, and indexing progress (kernel-level because it introspects kernel state); wrapped as a skill via `skill_adapter.go`
- `internal/kernel/help/` -- `get_tool_help` MCP tool serving on-demand comprehensive tool documentation pulled from the tool registry; wrapped as a skill via `skill_adapter.go`
- `internal/fuzzy/` -- 4-strategy fuzzy match cascade (exact match, whitespace-normalized, indentation-flexible, ellipsis-placeholder) with ambiguity refusal and indentation reflow; used by `replace_in_file`, `replace_symbol_body`, and standalone `fuzzy_edit` tool
- `internal/repomap/` -- Tag extraction (tree-sitter grammars for 23 languages + LSP documentSymbol fallback), SQLite tag cache with mtime invalidation, scope-aware elision, cross-file PageRank, token-budgeted tree renderer
- `protocol/gen/` -- Generated LSP 3.17 types (324 structs, 216 union types from metaModel.json)

### Layer 2: Skills & Multi-Language
- `internal/skill/` -- Skill/ToolProvider/WorkflowProvider interfaces, Caddy-style init() registration
- `internal/skill/memory/` -- 7 memory MCP tools wrapping markdown + SQLite FTS5 search
- `internal/skill/workflow/` -- Onboarding and session handoff tools
- `internal/skill/repomap/` -- `get_repo_map` and `get_context` MCP tools wrapping the RepoMap engine; registers via daemon post-init wiring (`SetEnrichFn`) for LSP enrichment of tags
- `internal/memory/` -- Memory store, FTS5 index, fsnotify watcher
- `internal/langregistry/` -- 52-language embedded registry with YAML override, three-tier LS installer

### Layer 3: Agent Profiles & Setup
- `internal/profile/` -- 5 agent profiles (claude-code, codex, ide-assistant, ci-bot, full), 4 modes (read/edit/review/admin)
- `internal/config/` -- 4-layer config: CLI > project (.serena/) > user (~/.serena/) > profile defaults
- `internal/cli/setup.go`, `internal/cli/setup_clients.go`, `internal/cli/setup_detect.go`, `internal/cli/setup_hooks.go`, `internal/cli/setup_output.go`, `internal/cli/setup_health.go` -- `serena setup <client>` one-command MCP registration for 7 clients (Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, OpenCode, generic) with language detection, LS pre-installation, and Claude Code hook installer
- `internal/cli/status.go`, `internal/cli/status_output.go` -- `serena status` CLI producing human-readable workspace health summary (`--json`, `--verbose` modes)
- `cmd/serena/main.go` -- single entrypoint; all CLI subcommands (setup, status, activate, deactivate, nudge, root, daemon wiring) live in `internal/cli/` and are mounted via cobra in `internal/cli/root.go`

### MCP Middleware Stack (installed in `internal/daemon/daemon.go` steps 14, 14b, 14c)
Four real middlewares, defined in `internal/mcp/`:
- `TelemetryMiddleware` (middleware.go) -- absorbs the pre-v1.2 logging middleware; emits RED metrics on `tools/call`, injects per-tool deadlines via `BudgetFunc`, classifies outcomes (success / timeout / circuit_open / internal / ...)
- `ProfileFilterMiddleware` (middleware.go) -- filters `tools/list` by active profile; ALSO applies brief descriptions from `ToolRegistry.BriefDescriptions()` (middleware.go:286-294) and then profile-specific description overrides. Brief descriptions are NOT a separate middleware — they piggyback on the profile filter's `tools/list` pass.
- `SuggestionMiddleware` (suggest.go) -- enriches parameter-typo and enum-value errors with "Did you mean?" suggestions via Levenshtein distance on tool schemas; never redirects to a different tool
- `LazyInitMiddleware` (lazy_init.go) -- `sync.Once` per workspace path; transparently activates the workspace on first tool call, serializes concurrent first calls

### Daemon Bootstrap (`internal/daemon/daemon.go`)
- Creates language registry, installer, kernel with pool, GrammarRegistry (23 languages), TagCache (SQLite, persistent)
- Imports all skill packages via blank imports (`imports.go`) for init() registration
- Calls `skill.InitAll()`, registers all tools centrally with MCP SDK
- Post-init wiring: `SetEnrichFn` for RepoMap LSP enrichment, `SetActivateCallback` for workspace activation, FallbackExtractor for languages without tree-sitter coverage
- Installs middleware in three steps (14, 14b, 14c), resolves active profile
- Fail-fast for core subsystems, degraded mode for optional providers
- Kernel-first shutdown ordering

## Technology Stack

- **Language:** Go (single binary, native concurrency)
- **MCP:** Official MCP Go SDK
- **RepoMap:** `github.com/tree-sitter/go-tree-sitter` with 23 language grammars (including locally vendored Swift and R bindings), hand-rolled PageRank (~60 LOC)
- **Config:** koanf v2 (4-layer precedence)
- **Database:** modernc.org/sqlite (CGO-free, for FTS5 memory search)
- **Tree-sitter:** go-tree-sitter (body extraction for symbol editing)
- **IPC:** gRPC (forwarder-daemon communication)
- **CLI:** cobra v1.9.1
- **Protocol:** MCP (Model Context Protocol) -- primary interface for all clients
- **LSP:** LSP 3.17 (generated types from official metamodel)

## Key Patterns

### Tool Registration
- Kernel tools use `RegisterTools(server *mcp.SerenaMCPServer, ...)` with typed args + `mcpsdk.AddTool`
- Skill tools use `ToolProvider.Tools()` returning `[]*mcp.ToolDef`, daemon registers centrally
- Kernel tools wrapped as thin skill adapters for uniform ToolProvider interface
- "Skills for composition, tool names for execution"
- Full tool inventory (41+ callable tools with profile/mode matrix) is auto-generated in `README.md`; do not hand-edit the tool table

### Skill System
- Caddy-style `init()` registration: `skill.Register(&MySkill{})`
- `skill.InitAll(deps)` initializes all skills with shared dependencies
- `skill.ToolProviders()` returns all skills implementing ToolProvider
- Kernel-resident tools (`internal/kernel/health/`, `internal/kernel/help/`) are exposed as skills via their `skill_adapter.go`, keeping the ToolProvider surface uniform

### Configuration
- 4-layer precedence: CLI flags > project `.serena/project.yml` > user `~/.serena/serena_config.yml` > profile defaults
- Profile YAMLs define tool subsets, description overrides, mode transitions
- `config.ResolveProfile()` bridges config profile name to ProfileStore

### Worker Pool
- Share-until-dirty: clean sessions share warm LS workers
- Adaptive TTL with reuse scoring
- Circuit breaking with exponential backoff for crashy workers
- Platform-aware memory pressure eviction (Linux cgroups, macOS vm_stat)
- Three-tier LS resolution: PATH lookup > managed download > helpful error

### Fuzzy Editing & RepoMap
- Edit tools fall back to `fuzzy.Match()` when exact matching fails; strategy used is reported in the result
- Ambiguous fuzzy matches are REFUSED with a diff — never silently applied
- RepoMap extraction is lazy: `walkAndExtract` populates the SQLite cache on demand; `get_repo_map` / `get_context` read from cache and fall back to tree-sitter or LSP `documentSymbol` when cache is cold
- Token budget fitting uses binary search over the elided tree; output scales to any repository size

### Setup & Hooks
- `serena setup <client>` (implemented in `internal/cli/setup*.go`) invokes client CLIs as subprocess (e.g., `claude mcp add-json`) rather than writing config files directly
- Claude Code hooks installed during `serena setup claude-code` (see `internal/cli/setup_hooks.go`): SessionStart (activate workspace), PreToolUse (nudge toward symbolic tools), Stop (cleanup); `--no-hooks` opts out
- Language detection (`internal/cli/setup_detect.go`) scans the project directory for known file extensions and pre-installs LSs via the three-tier installer before registering the MCP server

### Middleware Execution Order (LIFO)
Install order in `internal/daemon/daemon.go` is:
1. `InstallMiddleware` -- adds `TelemetryMiddleware` then `ProfileFilterMiddleware` (step 14)
2. `InstallSuggestionMiddleware` -- adds `SuggestionMiddleware` (step 14b)
3. `InstallLazyInitMiddleware` -- adds `LazyInitMiddleware` LAST (step 14c)

Because `mcp-go-sdk.AddReceivingMiddleware` composes in LIFO order, the execution order on an incoming request is the reverse of the install order:

`LazyInitMiddleware` → `SuggestionMiddleware` → `ProfileFilterMiddleware` (applies brief descriptions on `tools/list`) → `TelemetryMiddleware` → tool handler

LazyInit MUST run first so the workspace is activated before `TelemetryMiddleware` applies its per-tool deadline (see the install-order comment at `internal/mcp/lazy_init.go:106-108`). Any change to this order must preserve that invariant.

## Constraints

- **Language**: Go -- single binary, native concurrency
- **Protocol**: MCP (Model Context Protocol) -- primary interface
- **LSP only**: No JetBrains or proprietary backends
- **Repo**: Same repo, Python in `legacy/`

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.

## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
