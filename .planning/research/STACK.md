# Technology Stack

**Project:** Serena 2.0 (Go-native MCP code intelligence platform)
**Researched:** 2026-04-07

## Recommended Stack

### MCP Runtime

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | v1.4.1 | MCP server/client, tool registry, transport | Official SDK, maintained by Anthropic + Google. Supports stdio, Streamable HTTP, and custom transports. Struct-tag-based tool schemas. 4.3k stars, active CVE patching (March 2026 Origin header fix). Targets MCP spec 2025-11-25. | HIGH |

**Why not mark3labs/mcp-go:** While mcp-go has more stars (8.5k) and a friendlier builder-pattern API, the official SDK is the long-term bet. It's maintained by the spec authors, will always be first to support new spec versions, and has Google co-maintenance. mcp-go is a good community library but carries the risk of lagging behind spec changes. The official SDK API is already idiomatic enough (struct tags + `mcp.AddTool()`). If the official SDK proves insufficient during implementation, mcp-go is a viable fallback -- but start with official.

### LSP Client Layer

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Custom generated types (gopls pattern) | n/a | LSP protocol types, JSON-RPC framing | No good external LSP client library exists in Go. gopls uses internal generated types from the spec. mcp-language-server and opencode both copy/generate from gopls. This is the ecosystem consensus. | HIGH |
| [go.lsp.dev/jsonrpc2](https://pkg.go.dev/go.lsp.dev/jsonrpc2) | v0.10.0 | JSON-RPC 2.0 transport layer | Provides Stream abstraction over stdio/TCP. Lightweight, decoupled from LSP types. Alternative: roll your own (gopls does), but jsonrpc2 is small and correct. | MEDIUM |

**The LSP types problem:** There is no well-maintained, current, exportable LSP type library in Go. The options are:

1. **go.lsp.dev/protocol** -- Stuck at LSP 3.15 (2022), pre-v1, 12 importers. Dead.
2. **sourcegraph/go-lsp** -- Minimal subset of types, unmaintained.
3. **bugst/go-lsp** -- 14 stars, alpha, no version guarantees.
4. **owenrumney/go-lsp** -- Server-focused, LSP 3.17 types but server-side dispatch.
5. **gopls internal/protocol** -- Best types in Go, auto-generated from spec, but `internal/` package. There's an open issue (golang/go#67658) to export them; still unresolved.

**Recommendation:** Generate your own LSP types from the LSP metamodel JSON (same approach as gopls). The [LSP specification](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/) publishes a machine-readable metamodel. Write a code generator that produces Go structs + JSON tags. This is a one-time investment (~2-3 days) that gives you:
- Full LSP 3.17 coverage (or whatever version you target)
- No dependency on abandoned libraries
- Types exactly matching your client needs (not server-side bloat)
- Same approach proven by gopls, mcp-language-server, and opencode

For JSON-RPC 2.0 framing specifically, go.lsp.dev/jsonrpc2 is adequate as a starting point, though you may end up with a thin custom implementation given the daemon's multiplexed connection needs.

### Process Management & Daemon

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `os/exec` (stdlib) | Go 1.22+ | Spawn/manage language server processes | Standard library. exec.CommandContext for cancellation-aware process management. No library needed. | HIGH |
| `os/signal` + `context` (stdlib) | Go 1.22+ | Graceful shutdown, signal handling | `signal.NotifyContext` for SIGTERM/SIGINT. Context cancellation propagates through the entire daemon. Idiomatic Go pattern. | HIGH |
| `net` (stdlib) | Go 1.22+ | Unix domain socket listener for forwarder-daemon IPC | `net.Listen("unix", path)` is all you need. No IPC library required. | HIGH |
| Custom supervisor (goroutine-based) | n/a | LS worker lifecycle, circuit breaking, TTL management | The supervisor/restart pattern in Go uses a manager goroutine that monitors worker goroutines via channels. No library needed -- this is a core architectural component that should be purpose-built for LS worker semantics (warm TTL, crash counting, circuit breaking). | HIGH |

**Why no external process supervisor library:** The daemon IS the supervisor. External process supervisors (like ochinchina/supervisord) are for managing the daemon itself from outside. Internally, LS worker management is tightly coupled to your workspace/session model and needs custom logic for: warm cache retention, graceful drain on idle, crash-count-based circuit breaking, and session affinity. A generic library would add indirection without value.

**Daemon socket location:** Follow XDG conventions. `$XDG_RUNTIME_DIR/serena/daemon.sock` on Linux, `~/Library/Caches/serena/daemon.sock` on macOS. PID file alongside for liveness checking.

### Plugin / Skill System

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go interfaces + registry pattern | n/a | Skill pack registration, agent profile loading | For in-process plugins (compiled into the binary), Go interfaces are the right abstraction. Define a `Skill` interface with `Name()`, `Tools()`, `Init()` methods. Register at init time. Simple, type-safe, zero overhead. | HIGH |

**Why not hashicorp/go-plugin:** go-plugin is designed for out-of-process plugins communicating over gRPC. It's excellent when you need: crash isolation between plugins, cross-language plugins, or plugins from untrusted authors. Serena's skill packs are first-party Go code compiled into the binary. go-plugin would add: subprocess overhead per skill, gRPC serialization cost on every tool call, and complexity in the build/deploy pipeline. The 4-layer architecture already provides clean boundaries via interfaces.

**Future escape hatch:** If third-party plugin support becomes a requirement later, go-plugin can be introduced for Layer 2 skills specifically, while keeping Layer 0-1 in-process. The interface-based design doesn't preclude this -- it's additive.

### Configuration

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| [knadh/koanf](https://github.com/knadh/koanf) | v2 (latest) | Layered config: defaults -> global -> project -> CLI | Lightweight alternative to Viper. Respects key case (Viper lowercases everything). Modular providers -- only pull in YAML parser, not the entire dependency tree. 313% smaller binary than Viper. Supports merging multiple sources in order. | HIGH |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML parsing for config files | Standard YAML library for Go. Used by koanf's YAML provider. | HIGH |

**Config hierarchy (matching existing Serena):**
1. Built-in defaults (hardcoded)
2. Global config: `~/.serena/serena_config.yml`
3. Project config: `.serena/project.yml`
4. Environment variables: `SERENA_*`
5. CLI flags (highest precedence)

Koanf's `Load()` with ordered providers handles this naturally.

**Why not Viper:** Forces lowercase keys (breaks YAML specs), pulls massive dependency tree, global state by default. Koanf is the modern Go community choice for new projects.

### Caching

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| [dgraph-io/ristretto](https://github.com/dgraph-io/ristretto) | v2.0.0+ | In-memory symbol cache, workspace index | Generic-aware (v2), TinyLFU admission + SampledLFU eviction gives best-in-class hit rates. Cost-based eviction (can weight by symbol tree size). Concurrent-safe. Ideal for the shared workspace cache where multiple sessions read the same symbol data. | MEDIUM |
| `sync.Map` + custom (stdlib) | n/a | Session-scoped dirty buffer overlays | Session overlays are small, short-lived, and need fast path for the common case (no dirty buffers). sync.Map or a simple mutex-guarded map is sufficient. Don't over-engineer this. | HIGH |

**Cache topology:**
- **Workspace cache** (ristretto): Symbol trees, file indexes, LSP response cache. Shared across sessions on same workspace. Keyed by workspace fingerprint.
- **Session overlay** (simple map): Dirty buffers, unsaved edits. Per-session. Promoted to workspace cache on save.
- **LSP response cache** (ristretto): Recent textDocument/definition, references results. Short TTL, invalidated on file change events.

### CLI

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| [spf13/cobra](https://github.com/spf13/cobra) | v1.8+ | CLI commands: `serena daemon start`, `serena init`, etc. | De facto standard. Used by kubectl, docker, gh, hugo. Subcommand support, flag parsing, shell completion, help generation. No reason to use anything else. | HIGH |

### Logging

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `log/slog` (stdlib) | Go 1.22+ | Structured logging throughout | Standard library since Go 1.21. Zero dependencies. JSON and text handlers built in. Context-aware. Good enough for a daemon; no need for zerolog's marginal perf gains. Keeps dependency count down. | HIGH |

### Testing

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `testing` (stdlib) | Go 1.22+ | Test framework | Standard Go testing. Table-driven tests. | HIGH |
| [stretchr/testify](https://github.com/stretchr/testify) | v1.9+ | Assertions, mocks | `assert` and `require` packages reduce test boilerplate significantly. `mock` package for interface mocking (LS client, MCP transport). Most widely used Go test extension. | HIGH |

### Build & Distribution

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| [goreleaser](https://goreleaser.com/) | latest | Cross-platform binary releases | Single binary distribution is a key Go advantage. GoReleaser handles cross-compilation, checksums, GitHub releases, homebrew taps. | MEDIUM |
| Go modules | Go 1.22+ | Dependency management | Standard. `go.mod` + `go.sum`. | HIGH |

### Concurrency Primitives

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `golang.org/x/sync/errgroup` | latest | Parallel LSP reads with error propagation | Groups of goroutines with shared context cancellation. Perfect for "fan-out N definition lookups, cancel all on first error." | HIGH |
| `golang.org/x/sync/semaphore` | latest | Concurrency limits on LS worker pool | Weighted semaphore for bounding concurrent LS operations. Prevents overloading a single language server. | HIGH |

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| MCP SDK | modelcontextprotocol/go-sdk | mark3labs/mcp-go | Community vs official. Official will track spec faster. |
| LSP types | Generated from metamodel | go.lsp.dev/protocol | Stuck at LSP 3.15 (2022), unmaintained |
| LSP types | Generated from metamodel | sourcegraph/go-lsp | Minimal subset, unmaintained |
| Config | koanf v2 | spf13/viper | Viper lowercases keys, heavy deps, global state |
| Plugin | Go interfaces | hashicorp/go-plugin | Over-engineered for in-process compiled plugins |
| Cache | ristretto v2 | allegro/bigcache | BigCache optimizes for large byte blobs; ristretto better for typed objects with varying costs |
| Logging | slog (stdlib) | rs/zerolog | Marginal perf gain not worth the dependency for a daemon |
| JSON-RPC | go.lsp.dev/jsonrpc2 or custom | creachadair/jrpc2 | jsonrpc2 is simpler; custom may be needed for multiplexed daemon connections |

## Minimum Go Version

**Go 1.22** -- Required for:
- `log/slog` (1.21+)
- Enhanced `net/http` routing patterns (1.22)
- Improved `for range` semantics (1.22)
- Generic type support maturity

Target Go 1.23 if available at development start, for the latest `sync` package improvements.

## Installation (Initial Dependencies)

```bash
# Initialize module
go mod init github.com/postfix/serena

# Core
go get github.com/modelcontextprotocol/go-sdk@v1.4.1
go get github.com/knadh/koanf/v2
go get github.com/knadh/koanf/providers/file
go get github.com/knadh/koanf/parsers/yaml
go get github.com/spf13/cobra
go get github.com/dgraph-io/ristretto/v2

# Concurrency
go get golang.org/x/sync

# Testing
go get github.com/stretchr/testify

# JSON-RPC (evaluate during LSP layer implementation)
go get go.lsp.dev/jsonrpc2
```

## What NOT to Install

| Library | Why Not |
|---------|---------|
| `spf13/viper` | Heavy, lowercases keys, global state |
| `go.lsp.dev/protocol` | Abandoned at LSP 3.15 |
| `hashicorp/go-plugin` | Subprocess overhead for in-process plugins |
| `gorilla/mux` | stdlib `net/http` routing is sufficient since Go 1.22 |
| `gin`/`echo`/`fiber` | No web framework needed; MCP SDK handles HTTP transport |
| Any ORM | No traditional database; cache is in-memory, persistence is file-based |

## Sources

- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) -- v1.4.1, verified March 2026
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) -- v0.17.0+, MCP spec 2025-11-25
- [MCP Specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25)
- [go.lsp.dev/protocol](https://pkg.go.dev/go.lsp.dev/protocol) -- v0.12.0, LSP 3.15, last updated 2022
- [golang/go#67658](https://github.com/golang/go/issues/67658) -- Request to export gopls LSP types (unresolved)
- [isaacphi/mcp-language-server](https://github.com/isaacphi/mcp-language-server) -- Precedent for gopls-derived LSP types in Go MCP server
- [opencode-ai/opencode](https://github.com/opencode-ai/opencode) -- Go MCP client with LSP integration, uses custom LSP types
- [knadh/koanf](https://github.com/knadh/koanf) -- v2, lightweight config management
- [dgraph-io/ristretto](https://github.com/dgraph-io/ristretto) -- v2.0.0, generics-aware cache
- [VictoriaMetrics: Graceful Shutdown in Go](https://victoriametrics.com/blog/go-graceful-shutdown/) -- Daemon shutdown patterns
- [Supervisor/Restart Pattern in Go](https://compositecode.blog/2025/06/26/go-concurrency-patternssupervisor-restart-pattern/) -- Worker supervision patterns
- [CVE-2026-33252](https://advisories.gitlab.com/pkg/golang/github.com/modelcontextprotocol/go-sdk/CVE-2026-33252/) -- MCP Go SDK HTTP Origin fix in v1.4.1
