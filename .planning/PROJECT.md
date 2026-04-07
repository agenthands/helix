# Serena 2.0

## What This Is

A Go-native code intelligence platform for MCP: universal LSP gateway at the core, agent skills as plugins. Full rewrite of the Python-based Serena, keeping the same repo and brand. Targets coding agents (Claude Code, Codex, IDE assistants) that need semantic code operations — symbol-level retrieval, editing, refactoring — backed by real language servers.

## Core Value

Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools — without the lifecycle/timeout/asyncio pain of the Python version.

## Requirements

### Validated

<!-- Inferred from existing Python Serena codebase -->

- ✓ Symbol-level code retrieval (find symbol, symbol overview, references) — existing
- ✓ Symbolic editing (replace body, insert before/after, safe delete) — existing
- ✓ Multi-language LSP support (40+ languages) — existing
- ✓ MCP protocol exposure (stdio transport) — existing
- ✓ Pattern search across codebase — existing
- ✓ File operations (read, list, find) — existing
- ✓ Configurable tool sets via contexts and modes — existing
- ✓ Project-specific memory persistence — existing
- ✓ Rename refactoring via LSP — existing

### Active

- [ ] Go rewrite of full stack (Layers 0-3)
- [ ] Persistent supervisor daemon with warm LS workers
- [ ] Thin edge adapters: stdio forwarder + Streamable HTTP server
- [ ] Workspace cache separate from session views (gopls pattern)
- [ ] Daemon-managed LS workers with TTLs, circuit breaking, restart policy
- [ ] Serialized mutations, parallel reads per session/workspace
- [ ] Dynamic tool registry with pluggable skill packs
- [ ] Per-session/per-workspace isolation with dirty buffer promotion
- [ ] Agent profiles (Claude Code, Codex, IDE assistant, CI bot)
- [ ] Drop JetBrains backend dependency entirely
- [ ] Move Python Serena to `legacy/` folder in same repo

### Out of Scope

- JetBrains plugin backend — dropped, LSP-only going forward
- Python compatibility layer — clean Go rewrite, no Python interop
- Mobile/embedded targets — server-side only
- Custom language server implementations — wrap existing LSP servers, don't reimplement

## Context

This is a brownfield rewrite in the same repo (`postfix/serena`). The existing Python codebase provides:
- Proven tool taxonomy (40+ tools across symbol ops, file ops, memory, config, workflows)
- Validated LSP integration patterns for 40+ languages
- Battle-tested MCP protocol handling
- Known pain points: asyncio complexity, per-session LS startup cost, timeout/race issues, MCP tool timeouts

The Go rewrite addresses runtime correctness as the foundation problem. Current Serena release notes show ongoing work to reduce asyncio, move to synchronous LSP, background init, and linearized tool execution — all signals that the Python runtime model is fighting the architecture.

Go is chosen for: single binary distribution, native concurrency (goroutines for LS workers), low memory footprint, and ecosystem fit (gopls is the reference implementation for the daemon+forwarder pattern).

## Architecture

### Layer 0: MCP Runtime
- stdio + Streamable HTTP transport
- Capability negotiation
- Dynamic tool registry
- Per-session/per-workspace isolation
- Cancellation, timeouts, structured errors

### Layer 1: Code Intelligence Kernel
- LSP adapters (generic protocol, language-specific quirks)
- Symbol graph
- References/definitions/rename/edit primitives
- Workspace snapshots/index/cache
- Deterministic edit planning and verification

### Layer 2: Skills (pluggable)
- Semantic retrieval workflows
- Targeted editing workflows
- Memory system
- Repo understanding/onboarding
- Language/framework packs
- Review/refactor/test-generation helpers

### Layer 3: Agent Profiles (pluggable)
- Claude Code profile
- Codex profile
- IDE assistant profile
- CI/code-review bot profile

**Key rule:** Everything in Layer 2+ must be removable without weakening Layer 0-1.

### Runtime Model

**Daemon core + edge adapters:**
- One persistent supervisor daemon (workspace registry, LS process manager, caches, file watchers, health checks)
- stdio forwarder for local MCP hosts (tiny proxy → daemon via Unix socket)
- Streamable HTTP server for multi-client/remote/IDE scenarios
- Workspace key = repo root + language + toolchain fingerprint
- Session key = MCP session + dirty buffer overlay + mode/capability profile
- Share caches broadly, share live LS state cautiously

**LS worker policy:**
- Clean sessions attach to existing warm workers
- Divergent unsaved buffers promote to own LS view
- Idle workers stay warm for TTL, then retire
- Crashy workers get circuit-broken and restarted

## Constraints

- **Language**: Go — single binary, native concurrency, gopls ecosystem precedent
- **Protocol**: MCP (Model Context Protocol) — primary interface for all clients
- **LSP only**: No JetBrains or proprietary backends
- **Repo strategy**: Same repo, Python code moved to `legacy/`
- **Compatibility**: Must support the same 40+ languages currently supported via LSP

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go rewrite (not incremental Python improvement) | Runtime/lifecycle correctness is the foundation problem; Python asyncio fights the architecture | — Pending |
| Daemon + edge adapters (gopls pattern) | Solves warm cache, multi-client, reconnect; avoids per-session LS startup cost | — Pending |
| Drop JetBrains backend | Simplify to LSP-only; JetBrains coupling adds complexity without universal benefit | — Pending |
| 4-layer architecture with removable upper layers | Prevents monolith; keeps core universal; skills are plugins not requirements | — Pending |
| Same repo with legacy/ folder | Brand continuity, git history preserved, gradual migration path | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-07 after initialization*
