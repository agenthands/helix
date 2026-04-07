# Phase 3: Multi-Language and Skills - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

40+ language server support with auto-discovery and quirk handling, memory persistence system, and workflow skill packs (onboarding, session handoff, plugin interface). Breadth expansion on a working single-language kernel.

</domain>

<decisions>
## Implementation Decisions

### Language Server Registry
- **D-01:** Embedded Go registry compiled into the binary. All 40+ LS configs (binary path, args, init options, capabilities) are compiled-in Go structs. No external catalog file dependency at boot.
- **D-02:** YAML override/extend mechanism. Users can override any embedded entry or add new languages via `.serena/languages.yaml` or `~/.serena/languages.yaml`.
- **D-03:** LS installation: prefer existing install first (PATH lookup), managed download as fallback — not blind auto-download. Both modes available: auto (default) or manual-only (flag/config).
- **D-04:** Per-language quirk handling extends existing `lspool/quirks.go` pattern. Each language gets a QuirkAdapter implementing initialization sequences, capability differences, encoding quirks behind the uniform LSAdapter interface.
- **D-05:** Port language configs from `legacy/src/solidlsp/language_servers/` as reference. Each Python adapter maps to a Go QuirkAdapter + registry entry.

### Memory System
- **D-06:** Canonical storage: Markdown files. `.serena/memories/**/*.md` (project) and `~/.serena/memories/**/*.md` (global). Humans own the content.
- **D-07:** Derived index: SQLite with FTS. `.serena/index/memories.db` (disposable, rebuildable). Engine owns the search.
- **D-08:** Index contains: name, topic, file path, scope (project/global), title/headings, tags, modified time, content hash, short extracted summary. Optional FTS table for body search.
- **D-09:** Rebuild behavior: automatic on startup, FS watch for changes, explicit reindex command. Corrupted/missing DB → rebuild from markdown files. Zero data loss on index corruption.
- **D-10:** Scoping: directory-based (global vs project). Topic/subtopic as first-class indexed metadata for filtering. Slash-separated names supported (matching current Serena pattern).

### Skill Pack Interface
- **D-11:** Go owns behavior; YAML owns selection and prompting. Compiled Go skill modules with YAML-controlled context, descriptions, prompts, and enablement.
- **D-12:** Two Go interfaces: `Skill` (capability package) and `ToolProvider` (contributes MCP tools). Two YAML specs: `ContextSpec` and `ModeSpec` for presentation/composition.
- **D-13:** Tool = atomic MCP-exposed callable (input/output schema, independently testable). Skill = reusable capability package that may contribute tools, prompt fragments, context defaults, policies, workflow recipes, enable/disable rules.
- **D-14:** Two kinds of skill payload via one registration model:
  - Tool skill: contributes MCP tools (e.g. memory skill → CRUD tools)
  - Workflow skill: contributes prompts/context/policies, optionally uses tools internally (e.g. onboarding skill)
- **D-15:** Every tool is protocol-facing; not every skill needs to be. Skills can influence the MCP surface without being directly callable.

### Claude's Discretion
- Specific language server binary names and download URLs for each of the 40+ languages
- SQLite schema details for the memory index
- Onboarding workflow specifics (what analysis to run, what memories to create)
- Session handoff format (what gets summarized, how it's stored)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture & Research
- `.planning/research/FEATURES.md` — Feature landscape, table stakes vs differentiators
- `.planning/research/ARCHITECTURE.md` — gopls patterns, component boundaries
- `.planning/research/PITFALLS.md` — LS init races, language-specific quirks

### Prior Phase Decisions
- `.planning/phases/01-foundation/01-CONTEXT.md` — YAML config (D-10), .serena/ dir (D-09), interface-based plugins
- `.planning/phases/02-code-intelligence-kernel/02-CONTEXT.md` — QuirkAdapter pattern (D-04), worker pool design, LSP codegen

### Existing Code (Phase 2 output)
- `internal/kernel/lspool/quirks.go` — Existing quirk adapters for Go/Python/TypeScript/Rust (extend for 40+)
- `internal/kernel/lspool/adapter.go` — LSAdapter interface (uniform LS communication)
- `internal/kernel/lspool/pool.go` — Worker pool (language detection, worker creation)
- `internal/kernel/kernel.go` — Kernel for workspace coordination
- `internal/mcp/registry.go` — Dynamic tool registry (skill tools register here)
- `internal/config/config.go` — Config with per-language settings (extend for memory, skills)

### Legacy Reference
- `legacy/src/solidlsp/language_servers/` — 50+ Python LS adapters to port
- `legacy/src/serena/project.py` — MemoriesManager class (markdown storage, scoping)
- `legacy/src/serena/tools/memory_tools.py` — Memory tool implementations
- `legacy/src/serena/tools/workflow_tools.py` — Onboarding, prepare-for-new-conversation
- `legacy/src/serena/config/context_mode.py` — Context and mode YAML definitions (port to skill system)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/kernel/lspool/quirks.go` — 4 language adapters already exist; extend pattern for remaining 36+
- `internal/mcp/registry.go` — Tool registration works; skills will use same pattern
- `internal/config/loader.go` — koanf-based config loading supports YAML overlay for language/skill configs

### Established Patterns
- QuirkAdapter per language (quirks.go) — proven pattern to extend
- MCP tool registration with JSON Schema args (tools.go in fileops, symbols, diag, edit)
- Config hierarchy (CLI > project > user > defaults) via koanf

### Integration Points
- New language adapters plug into `lspool/quirks.go` QuirkAdapter registry
- Memory skill tools register with `mcp/registry.go`
- Skill interface consumed by agent profiles in Phase 4 (ContextSpec/ModeSpec)
- SQLite index lives alongside markdown files in `.serena/`

</code_context>

<specifics>
## Specific Ideas

- Port legacy Python LS adapters systematically — each maps to a Go QuirkAdapter + embedded registry entry
- Memory DB at `.serena/index/memories.db` — disposable, rebuildable from markdown
- Skill interface: `Skill` (capability package) + `ToolProvider` (MCP tools), both registered at daemon startup
- Onboarding skill: analyze project structure, detect languages, create initial memories
- Session handoff: summarize tool usage, open files, recent changes into a memory

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-multi-language-and-skills*
*Context gathered: 2026-04-08*
