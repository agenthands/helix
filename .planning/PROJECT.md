# Serena 2.0

## What This Is

A Go-native code intelligence platform for MCP: universal LSP gateway at the core, agent skills as plugins. Full rewrite of the Python-based Serena, shipping as a single Go binary. Targets coding agents (Claude Code, Codex, IDE assistants) that need semantic code operations — symbol-level retrieval, editing, refactoring — backed by real language servers with warm persistent caching.

## Core Value

Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools — without the lifecycle/timeout/asyncio pain of the Python version.

## Requirements

### Validated

- ✓ Go rewrite with daemon skeleton, MCP runtime, gRPC forwarder, HTTP transport — v1.0 Phase 1
- ✓ Single binary distribution via `go build` — v1.0 Phase 1
- ✓ Python Serena moved to `legacy/` as reference — v1.0 Phase 1
- ✓ LSP 3.17 metamodel codegen (324 structs, 216 union types) — v1.0 Phase 2
- ✓ JSON-RPC codec for LS communication — v1.0 Phase 2
- ✓ LS worker pool with adaptive TTL, circuit breaking, pressure eviction — v1.0 Phase 2
- ✓ 9 symbol retrieval tools (definition, references, overview, search, hover, implementations, call/type hierarchy, blast radius) — v1.0 Phase 2
- ✓ 6 symbol editing tools with tree-sitter body surgery — v1.0 Phase 2
- ✓ 6 file operation tools — v1.0 Phase 2
- ✓ 3 diagnostic tools (diagnostics, code actions, formatting) — v1.0 Phase 2
- ✓ 52-language embedded registry with YAML override — v1.0 Phase 3
- ✓ Three-tier LS installer (PATH/download/error) — v1.0 Phase 3
- ✓ Memory system (markdown + SQLite FTS5) — v1.0 Phase 3
- ✓ Skill interface (Skill/ToolProvider/WorkflowProvider) — v1.0 Phase 3
- ✓ 7 memory MCP tools + onboarding/handoff workflows — v1.0 Phase 3
- ✓ 5 agent profiles (claude-code, codex, ide-assistant, ci-bot, full) — v1.0 Phase 4
- ✓ 4 modes (read/edit/review/admin) with switch_mode tool — v1.0 Phase 4
- ✓ Token budget reporting via get_token_budget — v1.0 Phase 4
- ✓ Layered config precedence (CLI > project > user > profile) — v1.0 Phase 4
- ✓ Dynamic tool registry with pluggable skill packs — v1.0
- ✓ Centralized daemon bootstrap with 38+ callable MCP tools — v1.0 Phase 5
- ✓ Kernel tool skill adapters for uniform composition model — v1.0 Phase 5

### Active

(None — define for next milestone via `/gsd-new-milestone`)

### Out of Scope

- JetBrains plugin backend — dropped, LSP-only going forward
- Python compatibility layer — clean Go rewrite, no Python interop
- Mobile/embedded targets — server-side only
- Custom language server implementations — wrap existing LSP servers, don't reimplement
- Knowledge graphs — CodeGraphContext/GitNexus own this space
- Vector/embedding search — Augment Context Engine does this better
- Git operations — GitHub MCP Server handles git comprehensively

## Context

Shipped v1.0 with ~25,500 lines of Go across 21 packages. Single binary, 52 languages, 38+ MCP tools fully wired.

Tech stack: Go 1.25, official MCP Go SDK, koanf v2, modernc.org/sqlite, go-tree-sitter, gRPC.

Architecture: 4-layer (MCP runtime → Code intelligence kernel → Skills → Agent profiles). Persistent daemon with stdio/HTTP edge adapters. Worker pool with share-until-dirty, adaptive TTL, platform-aware pressure eviction. All layers wired via centralized daemon bootstrap with fail-fast core / degraded-optional startup.

Python Serena preserved in `legacy/` as reference.

## Constraints

- **Language**: Go — single binary, native concurrency
- **Protocol**: MCP (Model Context Protocol) — primary interface
- **LSP only**: No JetBrains or proprietary backends
- **Repo**: Same repo, Python in `legacy/`

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go rewrite | Runtime/lifecycle correctness is the foundation problem | ✓ Good — clean single binary, native concurrency |
| Daemon + edge adapters (gopls pattern) | Warm cache, multi-client, reconnect | ✓ Good — worker pool with adaptive TTL |
| Drop JetBrains backend | Simplify to LSP-only | ✓ Good — 52 languages via LSP |
| 4-layer architecture | Prevents monolith; skills as plugins | ✓ Good — clean separation |
| gRPC internal IPC | Typed APIs between forwarder and daemon | ✓ Good — clean proto contract |
| Tree-sitter for body surgery | LSP-only editing was fragile (Serena pain point) | ✓ Good — precise body extraction for 4 languages |
| Full LSP metamodel codegen | Forward-sync with spec, no hand-maintained types | ✓ Good — 324 structs, 216 unions generated |
| Markdown + SQLite FTS5 for memory | Humans own content, engine owns search | ✓ Good — rebuildable index |
| Caddy-style skill registration | Compiled-in plugins without go-plugin overhead | ✓ Good — clean init() pattern |
| Centralized daemon bootstrap | Daemon owns all MCP registration; skills describe, daemon binds | ✓ Good — 38 tools registered, profile filtering, clean shutdown |
| Kernel tools as skill adapters | Uniform ToolProvider interface for profiles and modes | ✓ Good — 4 thin adapters, consistent composition model |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone:**
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-08 after Phase 5 (daemon bootstrap integration)*
