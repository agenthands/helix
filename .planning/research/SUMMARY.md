# Project Research Summary

**Project:** Serena 2.0 (Go-native MCP code intelligence platform)
**Domain:** Daemon-based MCP server with LSP worker management
**Researched:** 2026-04-07
**Confidence:** HIGH

## Executive Summary

Serena 2.0 is a ground-up rewrite of the existing Python-based MCP code intelligence server into a Go daemon architecture. The core value proposition is not the rewrite itself but the runtime model change: a persistent supervisor daemon with warm LSP worker pools that survive client disconnects, eliminating the 5-30 second cold start penalty that plagues every competing MCP code intelligence server. The Go ecosystem has all the necessary building blocks -- the official MCP Go SDK (v1.4.1), proven daemon patterns from gopls, and mature concurrency primitives -- but critically lacks a usable LSP type library, requiring a one-time code generation investment from the LSP metamodel.

The recommended approach adapts the gopls Cache/Session/View/Snapshot hierarchy to manage multiple language servers behind an MCP interface. The architecture is cleanly layered: Layer 0 (MCP runtime + daemon), Layer 1 (code intelligence kernel with LS pool), Layer 2 (pluggable skill packs exposing tools), and Layer 3 (agent profiles). This layering enforces strict dependency direction and keeps the daemon core stable while skills evolve independently. The build order is bottom-up: daemon skeleton first, then LS worker pool with a single language (Go/gopls), then kernel operations, then user-facing tools.

The primary risks are all in Layer 1 process management: pipe deadlocks on child process I/O, LSP initialization races, goroutine leaks from unreapable child processes, and cache invalidation races with file watchers. These are well-understood problems with documented solutions, but they must be solved correctly from day one -- retrofitting process lifecycle management is extremely painful. The secondary risk is scope creep: the existing Python Serena has 40+ tools, and the temptation to port them all before shipping will kill the project. The MVP should ship with 5-8 core tools that cover 80% of agent usage, letting the daemon architecture prove its value before expanding the tool surface.

## Key Findings

### Recommended Stack

The stack is deliberately conservative, favoring stdlib and official libraries over community alternatives. The official MCP Go SDK is chosen over the more popular mcp-go community SDK because spec-author maintenance matters more than API ergonomics for a long-lived daemon. koanf replaces Viper for configuration (no key lowercasing, lighter dependencies). ristretto v2 provides the shared symbol cache with TinyLFU eviction. All concurrency is stdlib-based (errgroup, semaphore, context).

**Core technologies:**
- **modelcontextprotocol/go-sdk v1.4.1**: MCP server runtime -- official SDK, tracks spec fastest
- **Custom LSP types (generated from metamodel)**: No viable external library exists; gopls approach proven
- **go.lsp.dev/jsonrpc2**: JSON-RPC 2.0 transport for LS communication -- small, correct, may be replaced later
- **knadh/koanf v2**: Layered config (defaults/global/project/env/CLI) -- lightweight Viper alternative
- **dgraph-io/ristretto v2**: In-memory symbol cache with cost-based eviction -- shared across sessions
- **spf13/cobra**: CLI framework -- de facto standard, no reason to deviate
- **log/slog (stdlib)**: Structured logging -- zero dependencies, adequate for daemon use

### Expected Features

**Must have (table stakes):**
- Symbol retrieval: go-to-definition, find references, symbol overview, workspace search, hover, implementations
- Symbol editing: replace body, insert before/after, cross-file rename
- File operations: read, create, list, find, search, regex replace
- MCP protocol: stdio transport, tool listing with schemas, structured errors, cancellation
- Project activation: auto-detect language, start LS, multi-project support
- Memory persistence: write/read/list/delete project memories
- Multi-language support: 15+ languages with auto-discovery

**Should have (differentiators):**
- Persistent supervisor daemon with warm LS cache (no competitor does this)
- Edge adapter pattern (stdio forwarder to daemon socket)
- Agent profiles (Claude Code, Codex, IDE presets with tool filtering)
- Dynamic mode switching (planning/editing/review)
- Blast radius analysis (references + call hierarchy combined)
- Diagnostics subscription and code actions after edits
- Circuit breaking for crashy language servers

**Defer (v2+):**
- Plugin/skill extensibility for third-party packs
- Change impact analysis from git diffs
- Web dashboard / observability UI
- Streamable HTTP transport (parallel to stdio)
- Knowledge graphs, vector search, code generation, git operations, cloud hosting

### Architecture Approach

The architecture adapts gopls's daemon/forwarder pattern to a multi-language-server MCP gateway. A thin stdio forwarder connects to the daemon via Unix socket. The daemon manages MCP sessions (Layer 0), a code intelligence kernel with workspace management and LS worker pool (Layer 1), pluggable skill packs that implement tools (Layer 2), and agent profiles for per-client tool customization (Layer 3). The key insight is separating transport, session, and workspace state -- workspaces persist independently of client connections.

**Major components:**
1. **stdio forwarder** (`cmd/serena-forwarder/`) -- thin proxy, connects to daemon via Unix socket
2. **Daemon + MCP Runtime** (Layer 0) -- transport, JSON-RPC dispatch, session lifecycle, tool registry
3. **Workspace Manager** (Layer 1) -- workspace key resolution, view creation, cache sharing
4. **LS Worker Pool** (Layer 1) -- child process lifecycle, warm/idle/circuit-broken states, TTL eviction
5. **LS Adapter** (Layer 1) -- generic LSP client, language-specific quirk handling
6. **Symbol Graph + Edit Planner** (Layer 1) -- refs/defs/rename via LSP, serialized mutations with verification
7. **Skills** (Layer 2) -- pluggable tool implementations (retrieval, editing, memory, onboarding)
8. **Agent Profiles** (Layer 3) -- tool set presets, mode defaults per client type

### Critical Pitfalls

1. **Pipe deadlock on child process I/O** -- Dedicate separate goroutines for stdin/stdout/stderr; never call `cmd.Wait()` from the goroutine doing I/O; close stdin before Wait; use context timeout with SIGTERM/SIGKILL escalation
2. **LSP initialization race** -- Gate ALL requests behind a per-worker state machine (Starting/Initializing/Ready/ShuttingDown/Stopped); buffer didOpen during initialization
3. **Goroutine leak from unreapable children** -- ProcessReaper per worker with hard kill deadline (SIGTERM 3s, SIGKILL 2s); use process groups; track all workers in registry with health checks
4. **Unix socket stale file on crash** -- On startup, probe existing socket (connect test + PID file check); clean up stale socket; place in XDG_RUNTIME_DIR
5. **MCP session/transport confusion** -- Separate transport connection, MCP session, and workspace state from day one; sessions are lightweight views, workspaces own LS handles
6. **Python-to-Go porting trap** -- Use Python code as feature spec, not implementation guide; redesign for Go idioms (context, errgroup, channels, typed errors)

## Implications for Roadmap

### Phase 1: Daemon Skeleton + MCP Runtime
**Rationale:** Everything else plugs into this. Establishes the foundational process model (daemon, socket, forwarder) and MCP tool dispatch. Can be tested with dummy tools before any LS integration.
**Delivers:** Working daemon that accepts MCP connections via stdio forwarder, dispatches to registered tools, returns responses. `serena daemon start`, `serena init` CLI commands.
**Addresses:** MCP protocol compliance (stdio transport, tool listing, structured errors), project activation skeleton
**Avoids:** Stale socket file (Pitfall 4), session/transport confusion (Pitfall 5)
**Stack:** go-sdk, cobra, koanf, slog, Unix socket via net stdlib

### Phase 2: LS Worker Pool + Single Language (gopls)
**Rationale:** The LS pool is the hardest novel component and the core value proposition. Must be stable before building tools on top. gopls is the natural first target (Go project analyzing Go code).
**Delivers:** Warm LS worker pool with lifecycle management, circuit breaking, TTL eviction. Single-language proof that the adapter pattern works. Basic symbol retrieval (definition, references, symbol overview) via gopls.
**Addresses:** LS worker initialization, core symbol retrieval, workspace key caching
**Avoids:** Pipe deadlock (Pitfall 1), initialization race (Pitfall 2), goroutine leak (Pitfall 3), daemon health check issues (Pitfall 11)
**Stack:** exec.CommandContext, jsonrpc2, generated LSP types, ristretto cache, errgroup/semaphore

### Phase 3: Kernel Operations + Core Tools
**Rationale:** With a stable LS pool, build the full kernel (symbol graph, edit planner, file cache, snapshots) and expose as user-facing tools. This is where agents start using the system.
**Delivers:** 8-10 core MCP tools: find_symbol, get_symbols_overview, read_symbol_body, find_references, get_definition, search_for_pattern, read_file, replace_symbol_body, insert_before_symbol, rename_symbol. Working end-to-end for Go projects.
**Addresses:** Symbol editing, file operations, immutable snapshots, write serialization
**Avoids:** Serialized mutation bottleneck (Pitfall 10), oversized responses (Pitfall 15), feature parity obsession (Pitfall 8)
**Stack:** Kernel interface, skill registration, file-level RWMutex

### Phase 4: Multi-Language + Memory System
**Rationale:** Multi-language is configuration, not architecture. Adding pyright, typescript-language-server, and rust-analyzer validates the quirk abstraction. Memory system is an independent skill pack with no LS dependency.
**Delivers:** 4-5 language support (Go, Python, TypeScript, Rust, Java). Project memory persistence. Language-specific quirk profiles ported from Python Serena.
**Addresses:** Multi-language LSP support, memory/knowledge persistence, language server quirk handling
**Avoids:** LSP server quirks long tail (Pitfall 9) -- start narrow, validate abstraction
**Stack:** LanguageServerProfile type, per-LS quirk registry, memory skill pack

### Phase 5: Agent Profiles + Advanced Features
**Rationale:** With the core platform stable and multi-language, add the differentiation layer: agent profiles, mode switching, diagnostics, call/type hierarchy, Streamable HTTP.
**Delivers:** Claude Code / Codex / IDE presets, planning/editing/review modes, post-edit diagnostics, blast radius foundation, HTTP transport for multi-client.
**Addresses:** Agent profile system, compound operations, diagnostics, Streamable HTTP transport
**Avoids:** Scope creep -- only add these after core is proven with real users

### Phase 6: Extensibility + Polish
**Rationale:** Plugin system, onboarding workflows, dashboard, and broad language coverage are expansion features that build on a stable core.
**Delivers:** Skill pack extension interface, automated onboarding, web dashboard, 15+ languages
**Addresses:** Plugin extensibility, observability, remaining language coverage

### Phase Ordering Rationale

- **Bottom-up by dependency**: Daemon before LS pool before kernel before tools. Each phase provides the substrate for the next.
- **Risk front-loading**: The three hardest problems (daemon lifecycle, child process management, LS initialization) are tackled in Phases 1-2 before any user-facing features.
- **Early user value**: Phase 3 delivers a usable single-language system. Agents can start testing by Phase 3 completion, providing feedback before multi-language expansion.
- **Pitfall avoidance**: Feature parity obsession (Pitfall 8) is explicitly countered by the Phase 3 MVP scope of 8-10 tools, not 40+.
- **Architecture validation**: Each phase validates the layer below. If the LS pool design is wrong, Phase 2 reveals it before Phase 3 builds on it.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1:** Daemon socket lifecycle patterns, MCP SDK session management API -- needs hands-on SDK exploration
- **Phase 2:** LSP type generation from metamodel -- needs spike/prototype before committing to approach. Child process I/O patterns need integration testing with real LS processes early.
- **Phase 4:** Per-language-server quirk porting -- the Python codebase's `language_servers/` directory is the source of truth and needs systematic extraction

Phases with standard patterns (skip research-phase):
- **Phase 3:** Kernel operations follow well-documented gopls patterns (snapshots, edit planning). Tool/skill registration is straightforward interface-based Go.
- **Phase 5:** Agent profiles are thin filtering/configuration layers. Streamable HTTP is handled by the MCP SDK.
- **Phase 6:** Plugin interfaces and web dashboards are well-trodden Go patterns.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Official SDK verified, alternatives well-analyzed, Go ecosystem mature for this domain |
| Features | HIGH | Based on existing Serena v1 codebase (40+ tools) + survey of 15+ competing tools |
| Architecture | HIGH | gopls daemon pattern is proven and well-documented; 4-layer design is sound |
| Pitfalls | HIGH | Specific Go issues with child process I/O, LSP races backed by GitHub issues and existing Python workarounds |

**Overall confidence:** HIGH

### Gaps to Address

- **LSP type generation**: No one has published a standalone, reusable Go LSP type generator. The recommendation to generate from metamodel is sound but needs a prototype spike (estimated 2-3 days) to validate complexity. Fallback: vendor gopls internal types with build tag hacks.
- **MCP SDK session-scoped tool registration**: The official Go SDK's API for per-session tool filtering is unclear from docs alone. Needs hands-on evaluation. Fallback: mcp-go has a clearer API for this.
- **Dirty buffer promotion heuristics**: When to promote a session from shared LS worker to dedicated worker is a design question with no prior art. Needs experimentation during Phase 2-3.
- **Cross-platform daemon behavior**: Research focused on Unix (macOS/Linux). Windows named pipe support for the daemon socket is unresearched. If Windows support is required, this needs dedicated research.
- **ristretto v2 generic API stability**: ristretto v2 with generics is relatively new. Evaluate API stability during Phase 2 cache implementation.

## Sources

### Primary (HIGH confidence)
- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) -- v1.4.1, MCP runtime
- [MCP Specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25) -- protocol compliance
- [gopls daemon documentation](https://go.dev/gopls/daemon) -- daemon/forwarder architecture
- [gopls implementation design](https://github.com/golang/tools/blob/master/gopls/doc/design/implementation.md) -- Cache/Session/View/Snapshot
- [LSP Specification 3.17](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/) -- protocol types and lifecycle
- Current Serena Python codebase -- feature specification and quirk knowledge

### Secondary (MEDIUM confidence)
- [mcp-language-server](https://github.com/isaacphi/mcp-language-server) -- Go MCP+LSP precedent
- [opencode-ai/opencode](https://github.com/opencode-ai/opencode) -- Go MCP client with LSP integration
- [Ry Walker tool comparison](https://rywalker.com/research/code-intelligence-tools) -- competitive landscape
- [knadh/koanf](https://github.com/knadh/koanf) -- config management
- [dgraph-io/ristretto](https://github.com/dgraph-io/ristretto) -- cache library
- [2026 MCP Roadmap](https://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/) -- session evolution

### Tertiary (LOW confidence)
- [fsnotify race/deadlock issues](https://github.com/fsnotify/fsnotify/issues/666) -- file watcher pitfalls (needs validation during implementation)
- [golang/go#67658](https://github.com/golang/go/issues/67658) -- gopls LSP type export request (still unresolved, may change approach)

---
*Research completed: 2026-04-07*
*Ready for roadmap: yes*
