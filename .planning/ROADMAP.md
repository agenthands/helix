# Roadmap: Serena 2.0

## Overview

Serena 2.0 is a ground-up Go rewrite of the Python-based MCP code intelligence server. The build follows a strict bottom-up order: daemon skeleton and MCP runtime first (the foundation everything plugs into), then the code intelligence kernel with LS worker pool and core operations (the value engine), then multi-language expansion with memory and workflow skills (breadth), and finally agent profiles (differentiation). Each phase validates the layer below before building on it.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation** - Repo migration, daemon skeleton, MCP runtime, and edge adapters
- [ ] **Phase 2: Code Intelligence Kernel** - LS worker pool, symbol operations, file operations, and diagnostics
- [ ] **Phase 3: Multi-Language and Skills** - Broad language support, memory system, and workflow skills
- [ ] **Phase 4: Agent Profiles and Configuration** - Pre-built profiles, mode switching, and config hierarchy

## Phase Details

### Phase 1: Foundation
**Goal**: A working daemon accepts MCP connections via stdio forwarder and Streamable HTTP, dispatches to registered tools, and returns structured responses -- all from a single Go binary built in a migrated repo
**Depends on**: Nothing (first phase)
**Requirements**: MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, MCP-06, MCP-07, DMN-01, DMN-02, DMN-03, DMN-04, DMN-05, DMN-06, DMN-12, DMN-13, WRK-01, WRK-04, MIG-01, MIG-02, MIG-03
**Success Criteria** (what must be TRUE):
  1. Python Serena code lives in legacy/ and Go module is initialized at repo root; `go build` produces a single binary
  2. Running `serena daemon start` launches a persistent supervisor that accepts connections via Unix socket and survives client disconnects without losing state
  3. A stdio forwarder proxies MCP JSON-RPC traffic to the daemon; a Streamable HTTP endpoint serves multi-client connections
  4. Agents can discover available tools via MCP tool listing with JSON Schema definitions, and receive structured error codes on failures
  5. User can activate a project by repo path and the daemon tracks workspace/session state keyed correctly
**Plans**: 3 plans

Plans:
- [x] 01-01-PLAN.md — Repo migration to legacy/ and Go project scaffold with CLI entry point
- [x] 01-02-PLAN.md — Daemon skeleton with config, workspace registry, socket listener, and graceful shutdown
- [x] 01-03-PLAN.md — MCP server, tool registry, gRPC IPC, stdio forwarder, and Streamable HTTP transport

### Phase 2: Code Intelligence Kernel
**Goal**: Agents can perform symbol-level retrieval, editing, file operations, and diagnostics on a Go project through warm LS workers managed by the daemon
**Depends on**: Phase 1
**Requirements**: DMN-07, DMN-08, DMN-09, DMN-10, DMN-11, SYM-01, SYM-02, SYM-03, SYM-04, SYM-05, SYM-06, SYM-07, SYM-08, SYM-09, EDT-01, EDT-02, EDT-03, EDT-04, EDT-05, EDT-06, FIL-01, FIL-02, FIL-03, FIL-04, FIL-05, FIL-06, DGN-01, DGN-02, DGN-03, WRK-02, WRK-03, LNG-04
**Success Criteria** (what must be TRUE):
  1. Clean sessions attach to warm LS workers; sessions with dirty buffers promote to their own LS view; idle workers retire after TTL; crashy workers are circuit-broken and restarted
  2. User can go to definition, find references, get symbol overview, search symbols, get hover info, find implementations, and traverse call/type hierarchy -- all backed by a live language server
  3. User can replace a symbol body, insert before/after a symbol, rename across files, and safely delete a symbol with reference checking and post-edit diagnostic verification
  4. User can read files, create files, list directories, find files by pattern, search by regex, and replace content via regex
  5. Server auto-detects project languages, initializes appropriate LS, and supports multi-project workspaces
**Plans**: 6 plans

Plans:
- [x] 02-01-PLAN.md — LSP 3.17 metamodel codegen and JSON-RPC codec
- [x] 02-02-PLAN.md — File operations (read, write, list, find, search, replace)
- [x] 02-03-PLAN.md — LS worker pool with lifecycle, TTL, pressure eviction, circuit breaking
- [x] 02-04-PLAN.md — Symbol retrieval tools (definition, references, hover, hierarchy, blast radius)
- [x] 02-05-PLAN.md — Diagnostics tools (diagnostics, code actions, formatting)
- [ ] 02-06-PLAN.md — Symbol editing with tree-sitter body surgery

### Phase 3: Multi-Language and Skills
**Goal**: The server supports 40+ languages with auto-discovery and quirk handling, persists project knowledge across sessions, and provides workflow skills for onboarding and session handoff
**Depends on**: Phase 2
**Requirements**: LNG-01, LNG-02, LNG-03, MEM-01, MEM-02, MEM-03, MEM-04, MEM-05, WFL-01, WFL-02, WFL-03
**Success Criteria** (what must be TRUE):
  1. User can open projects in Python, TypeScript, Rust, Java, and 35+ other languages with language servers auto-discovered and downloaded as needed
  2. Per-language quirk handling works transparently -- initialization sequences, capability differences, and encoding quirks are abstracted behind a uniform interface
  3. User can write, read, list, search, edit, rename, and delete project-scoped and global memories that persist across sessions
  4. Automated onboarding generates project understanding; prepare-for-new-conversation summarizes session state; skill packs can extend tools without touching core
**Plans**: 3 plans

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD

### Phase 4: Agent Profiles and Configuration
**Goal**: Different agent clients (Claude Code, Codex, IDE assistants, CI bots) get tailored tool sets, modes, and prompts out of the box with a layered configuration system
**Depends on**: Phase 3
**Requirements**: PRF-01, PRF-02, PRF-03, PRF-04, PRF-05
**Success Criteria** (what must be TRUE):
  1. Pre-built profiles for Claude Code, Codex, IDE assistant, and CI bot each expose a curated tool set with appropriate descriptions and prompt overrides
  2. User can switch modes within a session (planning, editing, review) and the available tools and behavior adapt accordingly
  3. Configuration loads correctly from CLI args, project config, user config, and active profile in precedence order; tools report schema size for token budget optimization
**Plans**: 3 plans

Plans:
- [ ] 04-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 3/3 | Complete |  |
| 2. Code Intelligence Kernel | 0/6 | In Progress | - |
| 3. Multi-Language and Skills | 0/2 | Not started | - |
| 4. Agent Profiles and Configuration | 0/1 | Not started | - |
