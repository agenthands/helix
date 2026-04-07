# Architecture Patterns

**Domain:** Go daemon-based MCP server with LSP worker management
**Researched:** 2026-04-07
**Overall confidence:** HIGH (gopls patterns well-documented, Go MCP SDKs stable)

## Recommended Architecture

### The gopls Model, Adapted

The architecture directly adapts the gopls daemon/forwarder/cache/session/view/snapshot hierarchy to an MCP server that manages **multiple** language servers instead of being one.

```
                        MCP Clients
                    (Claude Code, Codex, IDE)
                           |
              +------------+------------+
              |                         |
      stdio forwarder          Streamable HTTP
      (tiny proxy binary)      (embedded in daemon)
              |                         |
              +------- Unix socket -----+
                           |
                    +------+------+
                    |   DAEMON    |
                    |             |
                    | MCP Runtime |  <-- Layer 0
                    | (sessions,  |
                    |  transport, |
                    |  registry)  |
                    +------+------+
                           |
                    +------+------+
                    |   KERNEL    |
                    |             |
                    | Workspace   |  <-- Layer 1
                    | Cache/View/ |
                    | Snapshot    |
                    +------+------+
                           |
              +------------+------------+
              |            |            |
         LS Worker    LS Worker    LS Worker
         (gopls)      (pyright)   (typescript)
         [child proc] [child proc] [child proc]
```

### Core Insight from gopls

gopls uses a **Cache > Session > View > Snapshot** hierarchy where:
- **Cache** lives for the process lifetime, shared across all sessions
- **Session** represents one client connection (one editor)
- **View** represents one workspace folder with specific build config
- **Snapshot** is an immutable point-in-time view of all files after an edit

For Serena 2.0, adapt this to:
- **Cache** = shared LS worker pool + parsed symbol indices + file content cache
- **Session** = one MCP client connection with its mode/profile/tool set
- **View** = one workspace (repo root + language) binding to warm LS workers
- **Snapshot** = workspace state including dirty buffer overlays for a session

### Component Boundaries

| Component | Responsibility | Talks To | Layer |
|-----------|---------------|----------|-------|
| **stdio forwarder** | Thin proxy, connects to daemon via Unix socket | Daemon (Unix socket) | Edge |
| **HTTP adapter** | Streamable HTTP MCP endpoint | Daemon (in-process) | Edge |
| **MCP Runtime** | Transport, JSON-RPC dispatch, session lifecycle, tool registry, capability negotiation | Sessions, Tool Registry | L0 |
| **Session Manager** | Per-client state, mode/profile binding, dirty buffer overlays | MCP Runtime, Views | L0 |
| **Tool Registry** | Dynamic tool registration, skill pack loading, per-session tool filtering | Sessions, Skills | L0 |
| **Workspace Manager** | Workspace key resolution, view creation, cache sharing policy | Sessions, LS Pool | L1 |
| **LS Pool** | Child process lifecycle, warm/idle/circuit-broken states, TTL eviction | Workspace Manager, LS Adapters | L1 |
| **LS Adapter** | Generic LSP client (JSON-RPC over stdio to child LS), language-specific quirk handling | LS Pool, individual LS processes | L1 |
| **Symbol Graph** | References, definitions, rename, symbol overview from LS responses | LS Adapter, Skills | L1 |
| **Edit Planner** | Deterministic edit planning + verification, conflict detection | Symbol Graph, File Cache | L1 |
| **File Cache** | File content, overlay management (saved vs unsaved), file watching | Workspace Manager, Skills | L1 |
| **Skills** | Pluggable tool implementations (semantic retrieval, editing, memory, onboarding) | Tool Registry, Kernel components | L2 |
| **Agent Profiles** | Tool set presets, mode defaults, capability negotiation hints | Session Manager, Tool Registry | L3 |

### Data Flow

#### Read Operation (e.g., "find symbol Foo")

```
1. MCP Client sends tools/call via stdio or HTTP
2. Edge adapter forwards JSON-RPC to daemon
3. MCP Runtime dispatches to session
4. Session resolves tool from registry (filtered by profile/mode)
5. Skill handler receives call
6. Skill calls Kernel: SymbolGraph.FindSymbol(workspace, "Foo")
7. Kernel checks Cache for existing result
8. Cache miss -> Kernel routes to LS Adapter for workspace's language
9. LS Adapter sends textDocument/documentSymbol to warm LS worker
10. LS Worker responds with symbols
11. Kernel caches result, returns to Skill
12. Skill formats MCP response
13. Response flows back: Session -> MCP Runtime -> Edge -> Client
```

#### Write Operation (e.g., "replace function body")

```
1-5. Same as read
6. Skill calls Kernel: EditPlanner.ReplaceBody(symbol, newCode)
7. EditPlanner acquires workspace write lock (serialized mutations)
8. EditPlanner resolves symbol location via SymbolGraph
9. EditPlanner computes edit, applies to overlay in FileCache
10. FileCache notifies LS Adapter of didChange
11. LS Worker re-indexes (diagnostics flow back async)
12. EditPlanner verifies edit (no parse errors from LS)
13. Release write lock
14. Response flows back with success + diagnostics
```

#### Concurrency Model

- **Parallel reads**: Multiple sessions can read from same workspace concurrently
- **Serialized writes**: One write at a time per workspace (mutex or channel-based serialization)
- **Cross-session isolation**: Dirty buffers are session-scoped overlays; clean state is shared
- **LS communication**: Each LS worker has a dedicated goroutine pair (reader + writer) for JSON-RPC

## Go Package Layout

Use the gopls-proven `cmd/` + `internal/` pattern. No `pkg/` directory -- this is a self-contained binary, not a library.

```
serena/
  cmd/
    serena/              # Main daemon binary
      main.go            # Flag parsing, daemon startup
    serena-forwarder/    # Thin stdio-to-socket proxy
      main.go
  internal/
    daemon/              # Daemon lifecycle, signal handling, socket listener
      daemon.go
      forwarder.go       # Forwarder connection handling (daemon side)
    mcp/                 # Layer 0: MCP Runtime
      server.go          # MCP server (wraps official SDK or mcp-go)
      session.go         # Session lifecycle, per-client state
      transport.go       # Transport abstraction (stdio, HTTP, socket)
      registry.go        # Dynamic tool registry
      negotiate.go       # Capability negotiation
    kernel/              # Layer 1: Code Intelligence Kernel
      workspace.go       # Workspace manager, view creation
      cache.go           # Shared cache (gopls Cache analog)
      snapshot.go        # Immutable workspace snapshot
      overlay.go         # Dirty buffer overlay management
      filewatcher.go     # File system watching, invalidation
    kernel/lspool/       # LS worker pool
      pool.go            # Worker lifecycle, TTL, circuit breaking
      worker.go          # Single LS worker (child process + JSON-RPC)
      adapter.go         # Generic LSP client protocol
      quirks.go          # Language-specific LSP quirk registry
    kernel/symbols/      # Symbol operations
      graph.go           # Symbol graph (refs, defs, rename)
      edit.go            # Edit planner and verifier
      search.go          # Pattern search across workspace
    skills/              # Layer 2: Pluggable skills
      skill.go           # Skill interface definition
      retrieval/         # Semantic retrieval workflows
      editing/           # Targeted editing workflows
      memory/            # Project memory persistence
      onboarding/        # Repo understanding
    profiles/            # Layer 3: Agent profiles
      profile.go         # Profile interface
      claude.go          # Claude Code preset
      codex.go           # Codex preset
      ide.go             # IDE assistant preset
    protocol/            # LSP protocol types (generated or vendored)
      lsp.go
      types.go
    config/              # Configuration loading
      config.go
      project.go         # Per-project .serena/ config
```

**Key conventions:**
- `internal/` enforces that nothing is importable outside the module
- Each `cmd/` produces one binary
- Package names match directory names (Go convention)
- No circular imports: daemon -> mcp -> kernel -> (skills use kernel interfaces, not vice versa)

## Interface Design

### Core Interfaces

```go
// Tool is what skills expose to the MCP layer
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, session *Session, input json.RawMessage) (*ToolResult, error)
}

// Skill is a pluggable pack of related tools
type Skill interface {
    Name() string
    Tools() []Tool
    // Init is called once when the skill is loaded
    Init(kernel Kernel) error
}

// Kernel is what skills call to access code intelligence
type Kernel interface {
    // Workspace operations
    Workspace(ctx context.Context, root string) (*Workspace, error)
    
    // Symbol operations (read path)
    FindSymbol(ctx context.Context, ws *Workspace, query SymbolQuery) ([]Symbol, error)
    GetReferences(ctx context.Context, ws *Workspace, sym Symbol) ([]Location, error)
    GetDefinition(ctx context.Context, ws *Workspace, sym Symbol) (*Location, error)
    SymbolOverview(ctx context.Context, ws *Workspace, path string) ([]Symbol, error)
    
    // Edit operations (write path, serialized)
    ReplaceBody(ctx context.Context, ws *Workspace, sym Symbol, newBody string) (*EditResult, error)
    InsertBefore(ctx context.Context, ws *Workspace, sym Symbol, code string) (*EditResult, error)
    InsertAfter(ctx context.Context, ws *Workspace, sym Symbol, code string) (*EditResult, error)
    Rename(ctx context.Context, ws *Workspace, sym Symbol, newName string) (*EditResult, error)
    
    // File operations
    ReadFile(ctx context.Context, ws *Workspace, path string) ([]byte, error)
    SearchPattern(ctx context.Context, ws *Workspace, pattern string) ([]Match, error)
}

// LSWorker manages a single language server child process
type LSWorker interface {
    Language() string
    Healthy() bool
    Request(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
    Notify(ctx context.Context, method string, params interface{}) error
    Shutdown(ctx context.Context) error
}

// Profile defines agent-specific tool/mode defaults
type Profile interface {
    Name() string
    DefaultMode() string
    ToolFilter() func(Tool) bool  // Which tools this profile exposes
    MaxConcurrentReads() int
}
```

### Dependency Direction (strict)

```
profiles -> skills -> kernel -> lspool -> protocol
              |                    |
              +-----> mcp <--------+  (mcp depends on nothing below kernel)
                       |
                    daemon
```

Skills depend on the Kernel interface, never on concrete LS workers. The Kernel interface is the stability boundary -- everything above it can change independently of LS implementation details.

## Process Supervision Patterns

### Daemon Lifecycle

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer cancel()
    
    daemon := daemon.New(config)
    
    // Start subsystems
    g, gctx := errgroup.WithContext(ctx)
    g.Go(func() error { return daemon.ListenSocket(gctx, socketPath) })
    g.Go(func() error { return daemon.ListenHTTP(gctx, httpAddr) })
    g.Go(func() error { return daemon.RunHealthChecks(gctx) })
    g.Go(func() error { return daemon.RunIdleReaper(gctx) })
    
    if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
        log.Fatal(err)
    }
    
    // Graceful shutdown: drain sessions, stop LS workers
    daemon.Shutdown(shutdownCtx)
}
```

### LS Worker Supervision

```go
type WorkerState int
const (
    WorkerStarting WorkerState = iota
    WorkerReady                        // Initialized, serving requests
    WorkerIdle                         // No active requests, TTL counting
    WorkerCircuitOpen                  // Too many crashes, backing off
    WorkerShutdown                     // Graceful shutdown in progress
)

type WorkerPolicy struct {
    IdleTTL         time.Duration   // How long idle before retirement (e.g., 5m)
    MaxRestarts     int             // Before circuit breaking (e.g., 3)
    RestartBackoff  time.Duration   // Exponential backoff base (e.g., 1s)
    CircuitTimeout  time.Duration   // How long circuit stays open (e.g., 30s)
    InitTimeout     time.Duration   // Max time for LS initialize handshake (e.g., 30s)
    RequestTimeout  time.Duration   // Default per-request timeout (e.g., 10s)
}
```

### Child Process Management

- Use `exec.CommandContext(ctx, ...)` so context cancellation kills the child
- Set `Setpgid: true` in `SysProcAttr` to isolate process groups
- Dedicated goroutine per worker: reads stdout (JSON-RPC responses), writes stdin (requests)
- Use `cmd.Wait()` in a goroutine to detect unexpected exits and trigger restart logic
- On daemon SIGTERM: send `shutdown` LSP request to each worker, wait up to N seconds, then SIGKILL stragglers

### Two-Phase Shutdown

```
Phase 1 (graceful): Stop accepting new sessions, drain active requests,
                     send LSP shutdown to all workers, save caches to disk
Phase 2 (forced):   After timeout, SIGKILL remaining workers, close sockets
```

## Patterns to Follow

### Pattern 1: Immutable Snapshots for Request Consistency

**What:** Each MCP tool call operates against an immutable snapshot of workspace state, taken at request start. No concurrent edits can invalidate the data mid-request.

**Why:** Eliminates an entire class of race conditions. gopls proves this works at scale.

**Implementation:** Snapshot holds references to cached file contents and LS responses. New edits create a new snapshot; old ones are GC'd when no requests reference them.

### Pattern 2: Workspace Key for Cache Identity

**What:** `WorkspaceKey = hash(repoRoot, language, toolchainVersion)`. Two sessions hitting the same workspace key share cache and LS workers.

**Why:** Avoids duplicate LS startup for the same repo. The key must include toolchain version because different Go/Python/TS versions produce different type information.

### Pattern 3: Overlay Promotion

**What:** Clean sessions share a base LS worker. When a session modifies files (unsaved edits), those edits live in a session-scoped overlay. If overlays diverge significantly, promote to a dedicated LS view.

**Why:** Balances sharing (most sessions are read-heavy) with correctness (divergent edits need isolated type checking).

### Pattern 4: Skill as Plugin, Not as Monolith

**What:** Skills register tools dynamically. The daemon core knows nothing about "find symbol" or "memory" -- those are skill-provided tools discovered at startup via a skill registry.

**Why:** Prevents Layer 0-1 from accreting feature code. Skills can be developed, tested, and versioned independently.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Global Mutable State

**What:** Shared mutable maps/slices accessed from multiple goroutines without synchronization.
**Why bad:** Race conditions that manifest only under load. Go's race detector catches some, not all.
**Instead:** Immutable snapshots + message passing via channels. Use `sync.RWMutex` only at cache boundaries.

### Anti-Pattern 2: Direct LS Protocol in Skills

**What:** Skill code constructing raw LSP requests (e.g., `textDocument/hover` JSON).
**Why bad:** Couples skills to LSP protocol details, language-specific quirks, and LS lifecycle.
**Instead:** Skills call the Kernel interface. Kernel handles LSP translation and quirk normalization.

### Anti-Pattern 3: Synchronous LS Initialization in Request Path

**What:** First request to a workspace blocks while LS starts up (can be 5-30 seconds).
**Why bad:** MCP tool call timeout (typically 30s-60s) easily exceeded. User perceives hang.
**Instead:** Workspace activation is async. Return "workspace initializing" status. Background goroutine warms the LS. Subsequent requests succeed once ready.

### Anti-Pattern 4: One goroutine per MCP request for LS communication

**What:** Spawning a new goroutine per tool call that directly talks to the LS.
**Why bad:** Unbounded concurrency to a single-threaded LS process causes request queuing and timeout cascading.
**Instead:** LS worker has a request channel with bounded concurrency. Reads are parallelized up to LS capacity; writes are serialized.

## Suggested Build Order

Based on dependency analysis, build bottom-up:

### Phase 1: Daemon Skeleton + MCP Runtime (Layer 0)
**Dependencies:** None (foundational)
**Build:**
1. `cmd/serena/main.go` -- daemon startup, signal handling, socket listener
2. `internal/daemon/` -- lifecycle, socket accept loop
3. `internal/mcp/` -- wrap official Go MCP SDK, session management, tool registry
4. `cmd/serena-forwarder/` -- thin stdio proxy to Unix socket
5. Integration: stdio forwarder -> daemon -> MCP handshake

**Why first:** Everything else plugs into this. You can test with dummy tools before any LS integration.

### Phase 2: LS Worker Pool + Single Language (Layer 1 foundation)
**Dependencies:** Phase 1 (daemon to host the pool)
**Build:**
1. `internal/kernel/lspool/worker.go` -- single LS child process management
2. `internal/kernel/lspool/adapter.go` -- generic LSP JSON-RPC client
3. `internal/kernel/lspool/pool.go` -- pool lifecycle, TTL, circuit breaking
4. `internal/kernel/workspace.go` -- workspace key, view creation
5. Start with gopls as first LS (dog-food Go analysis on Go code)

**Why second:** The LS pool is the hardest novel component. Get it stable before building on top.

### Phase 3: Kernel Operations (Layer 1 complete)
**Dependencies:** Phase 2 (needs LS workers to query)
**Build:**
1. `internal/kernel/symbols/` -- symbol graph, find/refs/defs via LSP
2. `internal/kernel/symbols/edit.go` -- edit planner with write serialization
3. `internal/kernel/cache.go` + `snapshot.go` -- caching layer, immutable snapshots
4. `internal/kernel/overlay.go` -- dirty buffer management
5. Wire Kernel interface to expose operations to skills

**Why third:** Kernel operations are the core value. Having the LS pool stable lets you focus on correctness of symbol resolution and edit planning.

### Phase 4: Core Skills (Layer 2)
**Dependencies:** Phase 3 (Kernel interface)
**Build:**
1. `internal/skills/retrieval/` -- find_symbol, symbol_overview, get_references
2. `internal/skills/editing/` -- replace_body, insert_before, rename
3. `internal/skills/skill.go` -- skill loading, tool registration
4. File operations skills (read, search, list)

**Why fourth:** Skills are the user-facing tools. They're straightforward once the Kernel interface is solid.

### Phase 5: Multi-Language + Profiles (Layer 1 expansion + Layer 3)
**Dependencies:** Phase 4 (working single-language system)
**Build:**
1. `internal/kernel/lspool/quirks.go` -- language-specific quirk registry
2. Add pyright, typescript-language-server, etc.
3. `internal/profiles/` -- agent profiles, tool filtering
4. `internal/skills/memory/` -- project memory persistence
5. `internal/skills/onboarding/` -- repo understanding

**Why last:** Multi-language is configuration, not architecture. Profiles are thin filtering layers. Memory/onboarding are independent skill packs.

## MCP SDK Choice

**Use the official Go SDK** (`github.com/modelcontextprotocol/go-sdk`) because:
- Maintained by MCP project + Google -- will track spec changes fastest
- Follows Go conventions (struct tags for schema generation)
- The project's identity is MCP-native; being on the official SDK reduces spec drift risk
- mcp-go (mark3labs) is more popular today but community-maintained; for a long-lived daemon, official backing matters more

If the official SDK lacks a needed feature (e.g., session-scoped tool registration is easier in mcp-go), wrap it. The MCP layer is a thin wrapper either way.

## Sources

- [gopls daemon documentation](https://go.dev/gopls/daemon)
- [gopls daemon design (GitHub)](https://github.com/golang/tools/blob/master/gopls/doc/daemon.md)
- [gopls implementation design](https://github.com/golang/tools/blob/master/gopls/doc/design/implementation.md)
- [gopls cache package](https://pkg.go.dev/golang.org/x/tools/gopls/internal/cache)
- [gopls lsprpc package](https://pkg.go.dev/golang.org/x/tools/gopls/internal/lsp/lsprpc)
- [gopls architecture (DeepWiki)](https://deepwiki.com/golang/tools/3-gopls-language-server)
- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [mcp-go community SDK](https://github.com/mark3labs/mcp-go)
- [mcp-go architecture (DeepWiki)](https://deepwiki.com/mark3labs/mcp-go)
- [Go project layout](https://go.dev/doc/modules/layout)
- [Graceful shutdown patterns in Go](https://victoriametrics.com/blog/go-graceful-shutdown/)
- [HashiCorp consul-template child process management](https://github.com/hashicorp/consul-template/blob/main/child/child.go)
- [go-child-process-manager](https://github.com/AgustinSRG/go-child-process-manager)
