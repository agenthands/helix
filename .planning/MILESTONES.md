# Milestones

## v1.0 MVP (Shipped: 2026-04-08)

**Phases completed:** 5 phases, 20 plans, 91 commits
**Lines of Go:** 25,779 across 21 packages
**Timeline:** 2 days (2026-04-07 - 2026-04-08)

**Key accomplishments:**

- Go rewrite with daemon skeleton, MCP runtime, gRPC forwarder, and Streamable HTTP transport — single binary distribution
- Full LSP 3.17 type generation from metaModel.json (324 structs, 216 union types) plus JSON-RPC 2.0 codec with session-prefixed ID routing
- 24 MCP tools for symbol retrieval (9), symbol editing (6), file operations (6), and diagnostics (3) — backed by live language servers with tree-sitter body surgery
- Multi-LS worker pool with share-until-dirty policy, adaptive TTL, circuit breaking, and platform-aware pressure eviction
- 52-language embedded registry with YAML deep-merge overlay and three-tier LS installer (PATH/download/error)
- Markdown-based memory system with SQLite FTS5 search, 7 MCP tools, and fsnotify auto-reindex
- Skill/plugin interfaces with Caddy-style init() registration, onboarding workflow, and session handoff
- 5 agent profiles (Claude Code, Codex, IDE assistant, CI bot, full), 4 modes (read/edit/review/admin), token budget awareness, profile filtering middleware
- Full daemon bootstrap wiring: 38+ callable MCP tools, centralized registration, fail-fast startup, kernel-first shutdown

---
