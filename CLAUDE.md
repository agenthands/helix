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
- `internal/kernel/fileops/` -- 6 file operation tools (read, write, list, find, search, replace)
- `internal/kernel/diag/` -- 3 diagnostic tools (diagnostics, code actions, formatting)
- `internal/kernel/jsonrpc/` -- Custom JSON-RPC 2.0 codec for LS communication
- `protocol/gen/` -- Generated LSP 3.17 types (324 structs, 216 union types from metaModel.json)

### Layer 2: Skills & Multi-Language
- `internal/skill/` -- Skill/ToolProvider/WorkflowProvider interfaces, Caddy-style init() registration
- `internal/skill/memory/` -- 7 memory MCP tools wrapping markdown + SQLite FTS5 search
- `internal/skill/workflow/` -- Onboarding and session handoff tools
- `internal/memory/` -- Memory store, FTS5 index, fsnotify watcher
- `internal/langregistry/` -- 52-language embedded registry with YAML override, three-tier LS installer

### Layer 3: Agent Profiles
- `internal/profile/` -- 5 agent profiles (claude-code, codex, ide-assistant, ci-bot, full), 4 modes (read/edit/review/admin)
- `internal/config/` -- 4-layer config: CLI > project (.serena/) > user (~/.serena/) > profile defaults

### Daemon Bootstrap (`internal/daemon/daemon.go`)
- Creates language registry, installer, kernel with pool
- Imports all skill packages via blank imports (`imports.go`) for init() registration
- Calls `skill.InitAll()`, registers all tools centrally with MCP SDK
- Installs `ProfileFilterMiddleware`, resolves active profile
- Fail-fast for core subsystems, degraded mode for optional providers
- Kernel-first shutdown ordering

## Technology Stack

- **Language:** Go (single binary, native concurrency)
- **MCP:** Official MCP Go SDK
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

### Skill System
- Caddy-style `init()` registration: `skill.Register(&MySkill{})`
- `skill.InitAll(deps)` initializes all skills with shared dependencies
- `skill.ToolProviders()` returns all skills implementing ToolProvider

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
