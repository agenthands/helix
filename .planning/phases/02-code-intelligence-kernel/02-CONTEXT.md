# Phase 2: Code Intelligence Kernel - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

LS worker pool management (warm workers, TTL, circuit breaking, dirty buffer promotion) + all symbol retrieval tools (definition, references, overview, search, hover, implementations, call/type hierarchy, blast radius) + symbol editing tools (replace body, insert before/after, rename, safe delete) + file operations + diagnostics. Validated with 3-4 language servers (gopls, pyright, typescript-language-server, rust-analyzer).

</domain>

<decisions>
## Implementation Decisions

### LS Worker Lifecycle
- **D-01:** Standalone engine service model — daemon owns the pool, not embedded in client lifetime. Three core objects: WorkspaceRuntime, SessionView, WorkerLease.
- **D-02:** Share-until-dirty policy: all clean sessions share warm LS workers. First unsaved edit promotes to own worker (gopls pattern).
- **D-03:** Adaptive TTL with decaying reuse score: `score = decay(score, now-last_used_at) + 1` on reuse. `ttl = clamp(base_ttl * f(score), base_ttl, ceiling)`. Avoids stale privilege from yesterday's hot workers.
- **D-04:** Auto-tuned TTL from observed reuse gaps: `warm_penalty = median(cold_start + first_query_after_start)`, `reuse_gap = p75(now - last_used_at on successful reuse)`, `ttl_target = clamp(max(base_ttl, reuse_gap * 1.5), base_ttl, ceiling)`, biased upward if warm_penalty is high.
- **D-05:** Platform-aware pressure eviction:
  - Linux: PSI (`/proc/pressure/memory`) with threshold triggers via poll/epoll
  - macOS: `os_proc_available_memory` for headroom, `task_info` for per-worker `resident_size`
  - Eviction order: (1) RSS > hard cap, (2) unhealthy/stuck, (3) LRU with score==0, (4) lowest score/highest RSS ratio, (5) oldest idle
- **D-06:** Circuit breaking with exponential backoff (1s, 2s, 4s, 8s...). Never fully breaks — always retries with increasing delay.
- **D-07:** "TTL=0" (always-on mode) means no idle timeout, NOT immortal. Pressure eviction always active.

### LSP Type Generation
- **D-08:** Full LSP 3.17 metamodel codegen — generate all protocol types and method bindings. Implement only the needed subset of methods.
- **D-09:** Package layout: `cmd/lspgen/` (custom generator binary), `protocol/metaModel.json` (pinned upstream input), `protocol/gen/` (generated .go), `protocol/patch/` (generator fixups/compatibility shims), `protocol/generate.go` (`//go:generate go run ../../cmd/lspgen`)
- **D-10:** Union representation: gopls-style pragmatic hybrid — named tagged wrapper types by default (Or_X_Y with custom JSON marshal/unmarshal + typed accessors like AsTextEdit(), SetTextEdit(...)). High-frequency unions promoted to explicit one-of structs (like DocumentChange).
- **D-11:** Patch layer for metamodel quirks (e.g. RenameParams mismatch). Generated core + controlled fixes, not hand-curated types.

### Symbol Editing
- **D-12:** Hybrid approach: LSP for symbol identity and discovery, tree-sitter for precise body boundary surgery.
- **D-13:** insert-before/insert-after: LSP-first (DocumentSymbol.range boundaries sufficient). Tree-sitter optional refinement.
- **D-14:** replace-body: tree-sitter-first (parse file, map symbol to AST node, derive precise body node). LSP-assisted for symbol discovery.
- **D-15:** Fallback for languages without tree-sitter grammar: insert-before/after from LSP range; replace-body degrades to best-effort or full-symbol replacement.

### Language Scope (Phase 2)
- **D-16:** Phase 2 ships with 3-4 languages: gopls (Go), pyright (Python), typescript-language-server (TypeScript), rust-analyzer (Rust). Proves multi-LS worker pool. Phase 3 expands to 40+.

### Claude's Discretion
- File operations implementation (FIL-01 through FIL-06) — straightforward Go stdlib, no gray areas
- Diagnostic subscription and code action forwarding (DGN-01 through DGN-03) — standard LSP protocol handling
- Multi-project workspace routing (WRK-02, WRK-03) — builds on workspace registry from Phase 1

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture & Research
- `.planning/research/ARCHITECTURE.md` — gopls Cache/Session/View/Snapshot hierarchy, package layout, component boundaries
- `.planning/research/STACK.md` — Go library choices, MCP SDK v1.4.1 API, koanf v2
- `.planning/research/PITFALLS.md` — Pipe deadlocks on child I/O, LSP init races, Go exec.Command gotchas

### Phase 1 Context (prior decisions)
- `.planning/phases/01-foundation/01-CONTEXT.md` — gRPC IPC (D-07), standard Go layout (D-12), error model (D-18/D-19)

### Existing Code (Phase 1 output)
- `internal/daemon/daemon.go` — Daemon lifecycle, errgroup pattern
- `internal/workspace/workspace.go` — Thread-safe workspace/session registry (extend for WorkerLease)
- `internal/workspace/key.go` — WorkspaceKey with composite keying
- `internal/mcp/server.go` — MCP server wrapping SDK (add tool implementations here)
- `internal/mcp/registry.go` — Dynamic tool registry (register kernel tools)
- `internal/mcp/errors.go` — Structured MCP errors + sentinel errors
- `internal/config/config.go` — SerenaConfig (extend for worker pool settings)

### Legacy Reference
- `legacy/src/solidlsp/ls.py` — Current Python LSP wrapper (SolidLanguageServer), caching strategy
- `legacy/src/serena/tools/symbol_tools.py` — Current symbol tool implementations (find, navigate, edit)
- `legacy/src/serena/tools/file_tools.py` — Current file tool implementations
- `legacy/src/solidlsp/language_servers/` — Per-language quirk handling adapters

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/workspace/workspace.go` — Registry pattern extends naturally to WorkerLease tracking
- `internal/mcp/registry.go` — Tool registration pattern for all kernel tools
- `internal/mcp/middleware.go` — Per-session tool filtering middleware
- `internal/daemon/daemon.go` — errgroup lifecycle pattern for managing LS child processes

### Established Patterns
- Sentinel errors + wrapping (errors.go) — extend for LS-specific errors (ErrLSInitFailed, ErrWorkerRetired)
- Config with koanf layered loading — extend for per-language worker pool settings
- gRPC bidirectional streaming — same pattern for LS JSON-RPC communication

### Integration Points
- New LS worker pool plugs into daemon's errgroup lifecycle
- Kernel tools register with existing mcp/registry.go
- Worker state tracked alongside workspace/session state in workspace package

</code_context>

<specifics>
## Specific Ideas

- WorkspaceRuntime, SessionView, and WorkerLease as the three core objects in the worker pool design
- Decaying reuse score for TTL (not lifetime counter) — workers cool off naturally
- Linux PSI for pressure detection, macOS os_proc_available_memory for headroom
- gopls-style metamodel codegen with patch layer for quirks
- Tree-sitter for precise body surgery on replace-body, LSP-first for insert-before/after
- Use go-tree-sitter Go bindings for tree-sitter integration

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-code-intelligence-kernel*
*Context gathered: 2026-04-07*
