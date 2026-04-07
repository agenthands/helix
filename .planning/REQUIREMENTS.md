# Requirements: Serena 2.0

**Defined:** 2026-04-07
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### MCP Runtime (Layer 0)

- [x] **MCP-01**: Server exposes tools via stdio transport with JSON-RPC framing
- [x] **MCP-02**: Server exposes tools via Streamable HTTP transport with session management
- [x] **MCP-03**: Tools are listed with JSON Schema definitions for agent discovery
- [x] **MCP-04**: Errors are returned as structured MCP error codes with parseable detail
- [x] **MCP-05**: Long-running operations support cancellation via MCP protocol
- [x] **MCP-06**: Server sends progress notifications during indexing and heavy operations
- [x] **MCP-07**: Dynamic tool registry allows adding/removing tools at runtime without restart

### Daemon & Process Management

- [x] **DMN-01**: Persistent supervisor daemon manages workspace registry, LS workers, caches, and file watchers
- [x] **DMN-02**: Daemon survives MCP client disconnects and reconnects without losing warm state
- [x] **DMN-03**: Thin stdio forwarder proxies MCP traffic to daemon via Unix socket/named pipe
- [x] **DMN-04**: Daemon serves Streamable HTTP directly for multi-client/remote scenarios
- [x] **DMN-05**: Workspace state keyed by repo root + language + toolchain fingerprint
- [x] **DMN-06**: Session state keyed by MCP session + dirty buffer overlay + mode/capability profile
- [ ] **DMN-07**: Clean sessions attach to existing warm LS workers
- [ ] **DMN-08**: Sessions with divergent unsaved buffers promote to their own LS view
- [ ] **DMN-09**: Idle LS workers stay warm for configurable TTL, then retire
- [ ] **DMN-10**: Crashy LS workers are circuit-broken with backoff and restarted without killing the server
- [ ] **DMN-11**: Mutations for the same session/view are serialized; reads are parallel where safe
- [x] **DMN-12**: Daemon handles graceful shutdown with signal handling and child process cleanup
- [x] **DMN-13**: Stale Unix socket files are detected and cleaned up on daemon start

### Symbol Retrieval (Layer 1)

- [ ] **SYM-01**: User can go to definition of a symbol
- [ ] **SYM-02**: User can find all references to a symbol
- [ ] **SYM-03**: User can get symbol overview (file outline) showing all symbols in a file
- [ ] **SYM-04**: User can search for symbols across the workspace by name/pattern
- [ ] **SYM-05**: User can get hover/type information for a symbol
- [ ] **SYM-06**: User can find implementations of an interface/abstract type
- [ ] **SYM-07**: User can get call hierarchy (callers and callees) for a function
- [ ] **SYM-08**: User can get type hierarchy for a class/interface
- [ ] **SYM-09**: User can analyze blast radius (combined references + call/type hierarchy) for a symbol

### Symbol Editing (Layer 1)

- [ ] **EDT-01**: User can replace a symbol's body with new content
- [ ] **EDT-02**: User can insert content before a symbol
- [ ] **EDT-03**: User can insert content after a symbol
- [ ] **EDT-04**: User can rename a symbol across all files in the workspace
- [ ] **EDT-05**: User can safely delete a symbol (with reference check)
- [ ] **EDT-06**: Edits are verified against LSP diagnostics after application

### File Operations

- [x] **FIL-01**: User can read a file or file range
- [x] **FIL-02**: User can create or overwrite a file
- [x] **FIL-03**: User can list directory contents
- [x] **FIL-04**: User can find files by glob/name pattern
- [x] **FIL-05**: User can search for regex patterns across the codebase
- [x] **FIL-06**: User can replace content via regex or literal match

### Memory & Knowledge

- [ ] **MEM-01**: User can write project-scoped memories (markdown files)
- [ ] **MEM-02**: User can read memories by name
- [ ] **MEM-03**: User can list and search stored memories
- [ ] **MEM-04**: User can rename, edit, and delete memories
- [ ] **MEM-05**: Global memories persist across projects; project memories are scoped

### Multi-Language LSP Support

- [ ] **LNG-01**: Server supports 40+ languages via LSP (matching current Serena coverage)
- [ ] **LNG-02**: Language servers are auto-discovered and downloaded when needed
- [ ] **LNG-03**: Per-language quirk handling via adapter layer (initialization, capabilities, encoding)
- [x] **LNG-04**: LSP types generated from official metamodel JSON (not hardcoded)

### Agent Profiles & Configuration (Layer 3)

- [ ] **PRF-01**: Pre-built agent profiles for Claude Code, Codex, IDE assistant, and CI bot
- [ ] **PRF-02**: Dynamic mode switching within a session (planning, editing, review)
- [ ] **PRF-03**: Tool description and prompt overrides per profile
- [ ] **PRF-04**: Token budget awareness — tools report schema size for context optimization
- [ ] **PRF-05**: Configuration loaded from CLI args → project config → user config → active profile

### Diagnostics & Code Quality

- [ ] **DGN-01**: Server reports LSP diagnostics after edits (errors, warnings)
- [ ] **DGN-02**: User can request code actions / quick fixes from LSP
- [ ] **DGN-03**: User can format code via LSP on demand

### Project & Workspace Management

- [x] **WRK-01**: User can activate a project by pointing at a repo path
- [ ] **WRK-02**: Server auto-detects project languages and initializes appropriate LS
- [ ] **WRK-03**: Multi-project support for monorepos and multi-service architectures
- [x] **WRK-04**: Project-local configuration via .serena/ directory

### Onboarding & Workflows (Layer 2)

- [ ] **WFL-01**: Automated onboarding workflow generates project understanding
- [ ] **WFL-02**: Prepare-for-new-conversation summarizes session state for handoff
- [ ] **WFL-03**: Plugin/skill pack interface allows extending tools without touching core

### Repo Migration

- [x] **MIG-01**: Current Python Serena moved to legacy/ folder
- [x] **MIG-02**: Go project initialized at repo root with standard Go module layout
- [x] **MIG-03**: Single binary distribution (go build produces one executable)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Analysis

- **ADV-01**: Change impact analysis from git diff (affected symbols/callers)
- **ADV-02**: Web dashboard for session info, tool usage, LS health
- **ADV-03**: MCP usage analytics (tool-level usage tracking)

### Extensibility

- **EXT-01**: Language/framework-specific skill packs (React, Django, etc.)
- **EXT-02**: Custom tool definition via YAML/config

## Out of Scope

| Feature | Reason |
|---------|--------|
| JetBrains plugin backend | Dropped — LSP-only simplifies architecture |
| AI/LLM integration in server | Server provides tools; agent does reasoning |
| Code generation tools | Agent's job, not the server's |
| Knowledge graph construction | CodeGraphContext/GitNexus own this space |
| Vector/embedding search | Augment Context Engine does this better |
| Repository packing | Repomix (22k stars) owns this |
| Git operations | GitHub MCP Server handles git comprehensively |
| Cloud/SaaS hosting | Local-only, privacy-first positioning |
| IDE-specific plugins | MCP is the universal protocol |
| Autocomplete/inline suggestions | IDE's job (Copilot, Continue) |
| Custom LS implementations | Wrap existing LS binaries, don't reimplement |
| Python compatibility layer | Clean Go rewrite, no Python interop |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| MCP-01 | Phase 1 | Complete |
| MCP-02 | Phase 1 | Complete |
| MCP-03 | Phase 1 | Complete |
| MCP-04 | Phase 1 | Complete |
| MCP-05 | Phase 1 | Complete |
| MCP-06 | Phase 1 | Complete |
| MCP-07 | Phase 1 | Complete |
| DMN-01 | Phase 1 | Complete |
| DMN-02 | Phase 1 | Complete |
| DMN-03 | Phase 1 | Complete |
| DMN-04 | Phase 1 | Complete |
| DMN-05 | Phase 1 | Complete |
| DMN-06 | Phase 1 | Complete |
| DMN-07 | Phase 2 | Pending |
| DMN-08 | Phase 2 | Pending |
| DMN-09 | Phase 2 | Pending |
| DMN-10 | Phase 2 | Pending |
| DMN-11 | Phase 2 | Pending |
| DMN-12 | Phase 1 | Complete |
| DMN-13 | Phase 1 | Complete |
| SYM-01 | Phase 2 | Pending |
| SYM-02 | Phase 2 | Pending |
| SYM-03 | Phase 2 | Pending |
| SYM-04 | Phase 2 | Pending |
| SYM-05 | Phase 2 | Pending |
| SYM-06 | Phase 2 | Pending |
| SYM-07 | Phase 2 | Pending |
| SYM-08 | Phase 2 | Pending |
| SYM-09 | Phase 2 | Pending |
| EDT-01 | Phase 2 | Pending |
| EDT-02 | Phase 2 | Pending |
| EDT-03 | Phase 2 | Pending |
| EDT-04 | Phase 2 | Pending |
| EDT-05 | Phase 2 | Pending |
| EDT-06 | Phase 2 | Pending |
| FIL-01 | Phase 2 | Complete |
| FIL-02 | Phase 2 | Complete |
| FIL-03 | Phase 2 | Complete |
| FIL-04 | Phase 2 | Complete |
| FIL-05 | Phase 2 | Complete |
| FIL-06 | Phase 2 | Complete |
| MEM-01 | Phase 3 | Pending |
| MEM-02 | Phase 3 | Pending |
| MEM-03 | Phase 3 | Pending |
| MEM-04 | Phase 3 | Pending |
| MEM-05 | Phase 3 | Pending |
| LNG-01 | Phase 3 | Pending |
| LNG-02 | Phase 3 | Pending |
| LNG-03 | Phase 3 | Pending |
| LNG-04 | Phase 2 | Complete |
| PRF-01 | Phase 4 | Pending |
| PRF-02 | Phase 4 | Pending |
| PRF-03 | Phase 4 | Pending |
| PRF-04 | Phase 4 | Pending |
| PRF-05 | Phase 4 | Pending |
| DGN-01 | Phase 2 | Pending |
| DGN-02 | Phase 2 | Pending |
| DGN-03 | Phase 2 | Pending |
| WRK-01 | Phase 1 | Complete |
| WRK-02 | Phase 2 | Pending |
| WRK-03 | Phase 2 | Pending |
| WRK-04 | Phase 1 | Complete |
| WFL-01 | Phase 3 | Pending |
| WFL-02 | Phase 3 | Pending |
| WFL-03 | Phase 3 | Pending |
| MIG-01 | Phase 1 | Complete |
| MIG-02 | Phase 1 | Complete |
| MIG-03 | Phase 1 | Complete |

**Coverage:**
- v1 requirements: 68 total
- Mapped to phases: 68
- Unmapped: 0

---
*Requirements defined: 2026-04-07*
*Last updated: 2026-04-07 after roadmap creation*
