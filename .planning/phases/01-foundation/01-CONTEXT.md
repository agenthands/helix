# Phase 1: Foundation - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Repo migration (Python → legacy/), daemon skeleton, MCP runtime with tool dispatch, and edge adapters (stdio forwarder + Streamable HTTP). Delivers a working Go binary that accepts MCP connections and dispatches to registered tools.

</domain>

<decisions>
## Implementation Decisions

### CLI & Invocation
- **D-01:** Binary name is `serena` (same as current Python CLI)
- **D-02:** Flat CLI with flags, not subcommand style. E.g. `serena --mode=stdio`, `serena --serve`
- **D-03:** Daemon auto-starts from forwarder (gopls pattern). No explicit `serena daemon start` required. Forwarder checks for running daemon, launches if not found.

### Legacy Migration
- **D-04:** All Python-related code moves to `legacy/` folder — src/, test/, scripts/, pyproject.toml, docs/, etc. Go project owns the repo root.
- **D-05:** CI preserved in legacy/ so Python tests can still run if needed during transition
- **D-06:** Legacy code serves as reference for Go reimplementation — consult it for tool behavior, LSP quirk handling, config schema

### IPC Protocol
- **D-07:** gRPC for internal communication between stdio forwarder and daemon over Unix socket
- **D-08:** Proto definitions live in `api/proto/` (top-level, Google convention)

### Project Config
- **D-09:** Keep `.serena/` as project config directory name
- **D-10:** YAML format for config files (continuity with current Serena)
- **D-11:** Full config scope matching current Serena — contexts, modes, tool overrides, memory config, LS-specific settings

### Go Module Layout
- **D-12:** Standard Go layout — `cmd/serena/`, `internal/` (private), `pkg/` (public API if any)
- **D-13:** Proto files in `api/proto/`

### Socket Location
- **D-14:** Daemon Unix socket at `/tmp/serena-$UID/daemon.sock` (gopls-similar pattern)
- **D-15:** Windows uses named pipes as Unix socket equivalent (full Windows support in v1)

### Logging & Observability
- **D-16:** Configurable log format — text by default, `--json` flag for structured JSON (Go slog)
- **D-17:** Logs go to stderr (forwarder) + rotated file (daemon at ~/.serena/logs/)

### Error Model
- **D-18:** MCP error codes with structured JSON detail (cause, suggestion) surfaced to clients
- **D-19:** Internal Go errors use sentinel errors + wrapping (errors.Is/As pattern) with domain-specific sentinels (ErrLSCrashed, ErrSessionExpired, etc.)

### Claude's Discretion
- No areas deferred to Claude's discretion — all decisions locked.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture
- `.planning/research/ARCHITECTURE.md` — gopls daemon/forwarder pattern, Cache/Session/View/Snapshot hierarchy, package layout
- `.planning/research/STACK.md` — Go library choices (MCP SDK, koanf, slog), version recommendations

### Pitfalls
- `.planning/research/PITFALLS.md` — Pipe deadlocks, LSP init races, socket stale files, signal handling

### Existing Codebase (Reference)
- `legacy/src/serena/mcp.py` — Current MCP server implementation (SerenaMCPFactory, tool schema generation)
- `legacy/src/serena/agent.py` — Current agent orchestrator (tool registry, mode management)
- `legacy/src/serena/config/serena_config.py` — Current config schema (contexts, modes, projects)
- `legacy/src/serena/config/context_mode.py` — Context and mode YAML definitions
- `legacy/src/serena/cli.py` — Current CLI structure (click-based)

### MCP Protocol
- `.planning/research/SUMMARY.md` — Research synthesis with build order and phase recommendations

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Current Python Serena's tool taxonomy (40+ tools) — use as reference for Go tool interface design
- `.serena/` config YAML schema — reuse format for Go config parsing
- Context/mode YAML definitions in `src/serena/resources/` — port to Go

### Established Patterns
- Tool base class with parameter validation and docstring-based schema generation
- Config hierarchy: CLI > project > user > defaults
- MCP tool registration: Tool → schema → MCP exposure

### Integration Points
- MCP clients connect via stdio or HTTP — daemon must handle both transports
- Language servers are child processes communicating via JSON-RPC over stdio — daemon manages their lifecycle

</code_context>

<specifics>
## Specific Ideas

- Follow gopls daemon auto-start pattern exactly: forwarder checks for socket, starts daemon if absent
- gRPC internal IPC gives typed APIs for forwarder↔daemon communication, separate from MCP wire protocol
- Proto files at top-level api/proto/ since IPC may become a public extension point later

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-04-07*
