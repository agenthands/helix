# Feature Landscape

**Domain:** MCP-based code intelligence platform (LSP gateway for AI coding agents)
**Researched:** 2026-04-07
**Overall confidence:** HIGH (based on existing Serena codebase analysis + ecosystem survey of 15+ competing tools)

## Table Stakes

Features users expect from an MCP code intelligence server. Missing any of these and agents will use built-in tools or a competitor instead.

### Symbol Retrieval

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Go-to-definition | Every LSP-MCP bridge offers this; agents need to trace code | Low | Direct LSP `textDocument/definition` wrapper |
| Find references | Core navigation; "who calls this?" is the #1 agent question | Low | Direct LSP `textDocument/references` wrapper |
| Symbol overview (file) | Agents need structural understanding before diving in | Low | LSP `textDocument/documentSymbol` |
| Workspace symbol search | Cross-file symbol discovery by name/pattern | Low | LSP `workspace/symbol` |
| Hover/type info | Quick symbol documentation without reading full body | Low | LSP `textDocument/hover` |
| Find implementations | Interface-to-concrete navigation, critical for typed languages | Low | LSP `textDocument/implementation` |
| Type hierarchy | Understanding inheritance chains; offered by Kiro, CodeMCP, Serena (JetBrains) | Med | LSP `typeHierarchy/*` -- not all language servers support it |
| Call hierarchy (callers/callees) | Impact analysis foundation; CodePathFinder and CodeMCP highlight this | Med | LSP `callHierarchy/*` -- inconsistent LS support |

### Symbol Editing

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Replace symbol body | Targeted editing without line-number drift; Serena's proven differentiator | Med | Requires symbol range resolution + safe replacement |
| Insert before/after symbol | Structural insertion (new methods, imports) without manual line calc | Med | Symbol boundary detection + content insertion |
| Rename symbol (cross-file) | Refactoring primitive; every LSP-MCP bridge offers this | Low | LSP `textDocument/rename` with workspace edit application |
| Safe delete | Remove symbol + verify no remaining references | Med | References check + deletion; currently JetBrains-only in Serena |

### File Operations

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Read file (with range) | Basic but necessary fallback when symbolic read is overkill | Low | Direct file read |
| Create/overwrite file | Agents need to create new files | Low | Path validation + write |
| List directory | Project structure exploration | Low | fs.ReadDir wrapper |
| Find file by pattern | Glob/name-based file discovery | Low | filepath.Walk + glob matching |
| Search for pattern (regex) | Grep-like search across codebase | Low | ripgrep-style implementation or LSP |
| Content replacement (regex) | Text-level editing when symbolic editing is not applicable | Med | Regex match + safe replacement with conflict detection |

### Multi-Language LSP Support

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| 15+ languages out of box | mcp-language-server, lsp-mcp, Kiro all support 10-18 languages | Med | Language server config registry + auto-download |
| Auto language server discovery | Agents shouldn't configure LSP manually | Med | Detect project languages, find/install LS binaries |
| Language-specific quirk handling | gopls, pyright, typescript-language-server all have different behaviors | High | Per-LS adapter layer (Serena's existing pattern, validated) |

### MCP Protocol Compliance

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| stdio transport | Default MCP transport; every client expects it | Low | Standard stdin/stdout framing |
| Streamable HTTP transport | MCP spec 2025-06-18+; remote/multi-client scenarios | Med | HTTP/SSE server with session management |
| Tool listing with schemas | MCP discovery mechanism; agents need this to know what's available | Low | JSON Schema generation from tool definitions |
| Structured errors | Agents need parseable error responses, not stack traces | Low | MCP error codes + structured detail |
| Cancellation support | Long-running operations must be cancellable | Med | Context propagation through LSP calls |
| Progress notifications | MCP spec supports progress; critical during indexing | Low | MCP notification forwarding |

### Project/Workspace Management

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Project activation by path | Agent points at a repo, server handles the rest | Low | Root detection + LS initialization |
| Multi-project support | Monorepos, multi-service architectures | Med | Workspace key registry |
| Configuration persistence | .serena/ or equivalent project-local config | Low | YAML/JSON config in project root |

### Memory/Knowledge Persistence

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Write/read project memories | Cross-session knowledge persistence; Serena's proven feature | Low | Markdown files in .serena/memories/ |
| List/search memories | Discovery of stored knowledge | Low | Directory listing + optional search |
| Memory CRUD (rename, delete, edit) | Full lifecycle management | Low | File operations on memory store |
| Global vs project-scoped memories | Style guides vs project-specific knowledge | Low | Directory-based scoping |

## Differentiators

Features that set Serena 2.0 apart. Not expected, but create competitive advantage.

### Daemon Architecture (Primary Differentiator)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Persistent supervisor daemon | Warm LS cache survives client disconnects; no per-session startup cost. NO competing MCP code intelligence server does this. | High | Unix socket/named pipe daemon with process management |
| Warm LS worker pool | Language servers stay hot between sessions; sub-second tool response vs 5-30s cold start | High | TTL-based worker lifecycle, workspace key matching |
| Edge adapter pattern (stdio forwarder) | Thin proxy connects to daemon; crash/reconnect is transparent | Med | stdio-to-unix-socket relay |
| Circuit breaking for crashy LS | Auto-restart with backoff; don't let one bad LS kill the server | Med | Health tracking + restart policy per LS |
| Dirty buffer promotion | Unsaved edits get their own LS view; clean sessions share workers | High | Buffer overlay management, LS workspace/folder management |
| Multi-client sharing | Multiple agents/IDE sessions share one daemon with warm caches | High | Session isolation + shared workspace state |

**Why this matters:** The rywalker.com comparison explicitly notes: "No tool has nailed incremental, real-time graph updates that keep pace with active development." A daemon with warm LS workers is the foundation for solving this.

### Agent Profile System

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Pre-built agent profiles (Claude Code, Codex, IDE, CI) | Tool set + prompt + behavior tuned per client. Serena v1 has contexts/modes; v2 makes this first-class. | Med | Profile = context + mode + tool filter + prompt template |
| Dynamic mode switching | Planning -> editing -> review within one session | Low | Tool registry hot-swap |
| Tool description overrides per profile | Same tool, different guidance per agent type | Low | Already proven in Serena v1 contexts |
| Token budget awareness | Show tool token costs (like Kiro /tools); help agents optimize context | Med | Schema size calculation + budget tracking |

### Compound/Workflow Operations

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Blast radius analysis | "What breaks if I change this?" -- CodeMCP and knowledge graph tools offer this; LSP-backed version is more accurate | High | References + call hierarchy + type hierarchy combined |
| Onboarding workflow | Auto-generate project understanding; proven in Serena v1 | Med | Structured analysis + memory creation |
| Change impact from diff | Analyze git diff for affected symbols/callers before commit | High | Diff parsing + symbol resolution + reference tracing |
| Thinking/reflection tools | Force agent to pause and assess; reduces wasted tool calls. Serena v1 validates this pattern. | Low | Prompt-returning tools that trigger self-assessment |
| Prepare-for-new-conversation | Summarize session state for handoff to next session | Low | Context gathering + memory write |

### Plugin/Skill Extensibility

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Pluggable skill packs (Layer 2) | Community-contributed workflows without touching core | High | Go plugin interface or subprocess-based extension |
| Language/framework packs | React-specific, Django-specific tool bundles | Med | Skill pack specialization |
| Dynamic tool registry | Add/remove tools at runtime without restart | Med | Hot-reload tool definitions |
| Custom tool definition format | Users define tools via YAML/config (like Serena v1 modes) | Med | Schema-driven tool generation |

### Diagnostics and Code Quality

| Feature | Why Valuable | Complexity | Notes |
|---------|-------------|------------|-------|
| Real-time diagnostics after edit | Claude Code's LSP integration does this; agents fix errors inline | Med | LSP `textDocument/publishDiagnostics` subscription |
| Code actions / quick fixes | Apply LSP-suggested fixes; lsp-mcp bridge offers this | Med | LSP `textDocument/codeAction` |
| Formatting on demand | LSP-backed formatting via tool call | Low | LSP `textDocument/formatting` |

### Observability

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Web dashboard | Session info, tool usage stats, LS health. Serena v1 has this. | Med | Embedded HTTP server with simple UI |
| MCP usage analytics | Track tool-level usage (like Sourcegraph's MCP analytics) | Low | Request counting + aggregation |
| LS health monitoring | Per-language-server status, latency, crash count | Med | Health check loop + metric collection |

## Anti-Features

Features to explicitly NOT build. These are traps.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| AI/LLM integration in the server | The server provides tools; the LLM client does the reasoning. Mixing concerns kills composability. | Expose clean tools, let Claude/Codex/etc. do the thinking |
| Code generation tools | Agents generate code themselves; server should navigate and edit, not generate | Provide precise editing primitives; generation is the agent's job |
| Custom language server implementations | Massive maintenance burden; existing LS implementations are better | Wrap existing LS binaries (gopls, pyright, rust-analyzer, etc.) |
| Knowledge graph construction | CodeGraphContext, GitNexus already do this well; LSP-backed references are more accurate for real-time queries | Use LSP call/type hierarchies for structural queries; recommend graph tools for offline analysis |
| Repository packing/context stuffing | Repomix (22k stars) owns this space; context packing is the agent's concern | Provide targeted retrieval tools; let agents decide what to include |
| Cloud/SaaS hosting of the server | Privacy-first, local-only is the market position; Sourcegraph/Greptile own cloud | Ship a single binary that runs locally |
| IDE-specific plugins | MCP is the universal protocol; IDE plugins fragment the surface area | MCP-only; let IDE MCP clients connect |
| Autocomplete/inline suggestions | This is the IDE's job (Copilot, Continue, etc.); MCP tools are for agentic workflows | Focus on tool-use patterns, not inline completion |
| Vector/embedding-based search | Augment's Context Engine does this with massive investment; competing is losing | Use LSP symbol search + grep; recommend Augment Context Engine MCP for semantic search |
| Git operations | GitHub MCP server (the most popular MCP server) already does this comprehensively | Focus on code intelligence; let git MCP servers handle git |

## Feature Dependencies

```
stdio transport ─────────────────────────────────── MCP Protocol Core
  │
  ├── Tool listing + schemas
  │     └── Agent profiles (tool filtering per profile)
  │           └── Dynamic mode switching
  │
  ├── Project activation
  │     ├── LS worker initialization
  │     │     ├── Symbol retrieval tools (all)
  │     │     │     ├── Symbol editing tools (need symbol resolution)
  │     │     │     ├── Blast radius analysis (references + call hierarchy)
  │     │     │     └── Change impact from diff
  │     │     ├── Diagnostics subscription
  │     │     │     └── Code actions / quick fixes
  │     │     └── Formatting
  │     ├── File operations (no LS needed)
  │     └── Memory system (no LS needed)
  │
  ├── Daemon architecture
  │     ├── Warm LS worker pool (requires daemon lifecycle)
  │     ├── Multi-client sharing (requires session isolation)
  │     ├── Edge adapter / stdio forwarder (requires daemon socket)
  │     ├── Circuit breaking (requires worker health tracking)
  │     └── Dirty buffer promotion (requires worker pool)
  │
  └── Streamable HTTP transport (parallel to stdio, requires daemon)
        └── Multi-client sharing (natural fit with HTTP)

Plugin/skill extensibility ──── Dynamic tool registry
                                  └── Language/framework packs
                                  └── Custom tool definitions
```

## MVP Recommendation

### Phase 1: Foundation (must-ship)
1. **MCP runtime** -- stdio transport, tool listing, structured errors
2. **Core symbol retrieval** -- definition, references, symbol overview, workspace search, hover
3. **Core symbol editing** -- replace body, insert before/after, rename
4. **File operations** -- read, create, list, find, search, replace
5. **Single language proof** -- Go (gopls) as first-class, validating the adapter pattern
6. **Project activation** -- point at repo, auto-detect language, start LS

### Phase 2: Daemon + Multi-Language
7. **Daemon supervisor** -- persistent process, Unix socket, stdio forwarder
8. **Warm LS pool** -- worker lifecycle, TTL, restart
9. **3-5 more languages** -- TypeScript, Python, Rust, Java
10. **Memory system** -- write/read/list/delete project memories

### Phase 3: Agent Profiles + Polish
11. **Agent profiles** -- Claude Code, Codex, IDE assistant presets
12. **Mode switching** -- planning/editing/review modes
13. **Diagnostics + code actions** -- post-edit error reporting
14. **Call/type hierarchy** -- blast radius foundation
15. **Streamable HTTP transport** -- multi-client scenarios

### Phase 4: Extensibility + Advanced
16. **Plugin skill packs** -- extension interface
17. **Change impact analysis** -- diff-based blast radius
18. **Onboarding workflow** -- automated project understanding
19. **Web dashboard** -- observability
20. **Remaining 30+ languages** -- broad coverage

**Defer indefinitely:** Knowledge graphs, vector search, git operations, code generation, cloud hosting.

## Sources

- Serena v1 codebase analysis (40+ tools across 6 tool modules) -- HIGH confidence
- [Ry Walker: Code Intelligence Tools for AI Agents Compared](https://rywalker.com/research/code-intelligence-tools) -- MEDIUM confidence
- [Augment Code Context Engine MCP](https://www.augmentcode.com/blog/context-engine-mcp-now-live) -- MEDIUM confidence
- [Kiro Code Intelligence CLI](https://kiro.dev/docs/cli/code-intelligence/) -- MEDIUM confidence
- [CodeMCP / CKB](https://github.com/SimplyLiz/CodeMCP) -- MEDIUM confidence
- [mcp-language-server (Go)](https://github.com/isaacphi/mcp-language-server) -- MEDIUM confidence
- [CodePathFinder MCP](https://codepathfinder.dev/mcp) -- MEDIUM confidence
- [lsp-mcp bridge](https://glama.ai/mcp/servers/blackwell-systems/LSP-MCP) -- MEDIUM confidence
- [GitHub MCP Server tool-specific configuration](https://github.blog/changelog/2025-12-10-the-github-mcp-server-adds-support-for-tool-specific-configuration-and-more/) -- MEDIUM confidence
- [Sourcegraph Cody MCP](https://sourcegraph.com) -- MEDIUM confidence
- [Continue.dev MCP support](https://docs.continue.dev/customize/deep-dives/mcp) -- MEDIUM confidence
- [MCP health check best practices](https://mcpcat.io/guides/building-health-check-endpoint-mcp-server/) -- LOW confidence
