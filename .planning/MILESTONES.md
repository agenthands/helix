# Milestones

## v1.0 MVP (Shipped: 2026-04-08)

**Phases completed:** 4 phases, 17 plans, 30 tasks

**Key accomplishments:**

- Python code migrated to legacy/, Go module initialized with cobra CLI skeleton producing single serena binary
- 1. [Rule 1 - Bug] Fixed Unix socket path length on macOS
- MCP server with official SDK, dummy tools (ping/echo/activate_project), gRPC forwarder-daemon IPC, stdio forwarder with auto-start, and Streamable HTTP endpoint
- Full LSP 3.17 type generation from metaModel.json (324 structs, 216 union types) plus Content-Length framed JSON-RPC 2.0 codec with session-prefixed ID routing
- 6 pure Go file operation tools (read, write, list, find, search, replace) with symlink-aware path security and MCP registration
- Multi-LS worker pool with share-until-dirty policy, adaptive TTL, circuit breaking, pressure eviction, and workspace-scoped language detection
- 1. [Rule 2] Added LeaseProvider abstraction
- Tree-sitter body extraction for 4 languages with 6 symbol editing MCP tools and automatic post-edit diagnostic verification
- 52-language embedded registry with YAML deep-merge overlay and three-tier LS installer (PATH/download/error)
- Markdown-based memory CRUD with SQLite FTS5 search, project/global scoping, and fsnotify auto-reindex
- Go skill/plugin interfaces with init()-based registry and YAML-driven context/mode composition for tool filtering
- QuirkAdapter interface replacing hardcoded LanguageQuirks with per-language behavioral hooks, wired to language registry for LS resolution
- Memory skill wrapping MemoryStore as 7 MCP tools and workflow skill with onboarding project analysis and session handoff via Caddy-style init() registration
- Profile/Mode types extending skill specs, 5 agent profile YAMLs and 4 mode YAMLs embedded via go:embed, loader with override merging and 6 tests
- switch_mode and get_token_budget MCP tools via profile skill with per-session mode tracking and transition validation
- Profile selection wired through 4-layer koanf config precedence with ProfileFilterMiddleware applying tool filtering and description overrides on MCP tools/list responses

---
