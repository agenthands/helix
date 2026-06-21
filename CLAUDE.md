# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Go Development Commands

- `go build ./cmd/helix` - Build the helix binary
- `go test ./...` - Run all Go tests
- `go vet ./...` - Run Go vet
- `gofmt -w .` - Format Go code
- `make build` - Build via Makefile
- `make test` - Run tests via Makefile

**Always run go vet and go test before completing any Go task.**

## Project

**Helix** — The IDE for your coding agent. A Go-native, CLI-first code intelligence platform.

Helix exposes its semantic code operations as `helix <verb>` CLI commands (the full verb inventory is auto-generated in `README.md`) for retrieval, editing, and refactoring across 52 languages via LSP. It ships as a **single Go binary** with no Python, Docker, or runtime dependencies, running as a **persistent daemon** that keeps language servers warm between agent sessions. The MCP Go SDK and gRPC IPC are retained as internal daemon plumbing — they are no longer an agent-facing surface.

Targets coding agents (Claude Code, Codex, Gemini CLI, IDE assistants) that need symbol-level operations — go-to-definition, find references, rename across files, replace symbol body, blast-radius analysis — backed by real language servers with warm persistent caching, a ranked RepoMap for structural context, and fuzzy editing that tolerates LLM output drift.

**Core Value:** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.

**Lineage:** Helix is a standalone, Go-native product — not a fork, port, or rewrite. It is partially inspired by prior art including Serena, Aider, Graphify, and others, but its kernel, daemon, and tooling are its own. The project carried an earlier name through v1.8 and was renamed to `helix` at v1.9 (see CHANGELOG.md > v1.9 Breaking Changes). The original Python reference codebase (formerly under `legacy/`) has been removed from the tree; see git history prior to v1.12 if you need it.

## Architecture

4-layer architecture shipping as a single Go binary:

### Layer 0: MCP Runtime
- `internal/mcp/` -- MCP server with official Go SDK, tool registry, structured errors, profile filtering middleware
- `internal/daemon/` -- Persistent supervisor daemon with errgroup orchestration, signal-first lifecycle
- `internal/forwarder/` -- Stdio-to-gRPC proxy with auto-start
- `api/proto/serena/v1/` -- gRPC IPC between forwarder and daemon (proto package directory name retained as a wire-format lineage artifact; see Phase 52-03 SUMMARY)
- Transport: the agent-facing surface is the `helix` CLI dialing the daemon over gRPC `StreamMCP` (unix socket / named-pipe by default; opt-in loopback-gated gRPC TCP). The stdio MCP forwarder head and the Streamable-HTTP `/mcp` head were removed in Phase 94 — only the internal gRPC `StreamMCP` wire remains.

### Layer 1: Code Intelligence Kernel
- `internal/kernel/` -- Kernel orchestrator, workspace runtime, language detection
- `internal/kernel/lspool/` -- LS worker pool: share-until-dirty, adaptive TTL, circuit breaking, pressure eviction
- `internal/kernel/symbols/` -- 9 symbol retrieval tools (definition, references, hover, implementations, call/type hierarchy, blast radius)
- `internal/kernel/edit/` -- 6 symbol editing tools with tree-sitter body surgery
- `internal/kernel/fileops/` -- 7 file operation tools (read, write, list, find, search, replace, fuzzy_edit)
- `internal/kernel/diag/` -- 3 diagnostic tools (diagnostics, code actions, formatting)
- `internal/kernel/jsonrpc/` -- Custom JSON-RPC 2.0 codec for LS communication
- `internal/kernel/health/` -- `get_health` MCP tool returning per-workspace LS status, capabilities, and indexing progress (kernel-level because it introspects kernel state); wrapped as a skill via `skill_adapter.go`
- `internal/kernel/help/` -- `get_tool_help` MCP tool serving on-demand comprehensive tool documentation pulled from the tool registry; wrapped as a skill via `skill_adapter.go`
- `internal/fuzzy/` -- 4-strategy fuzzy match cascade (exact match, whitespace-normalized, indentation-flexible, ellipsis-placeholder) with ambiguity refusal and indentation reflow; used by `replace_in_file`, `replace_symbol_body`, and standalone `fuzzy_edit` tool
- `internal/repomap/` -- Tag extraction (tree-sitter grammars for 23 languages + LSP documentSymbol fallback), SQLite tag cache with mtime invalidation, scope-aware elision, cross-file PageRank, token-budgeted tree renderer
- `protocol/gen/` -- Generated LSP 3.17 types (324 structs, 216 union types from metaModel.json)

### Layer 2: Skills & Multi-Language
- `internal/skill/` -- Skill/ToolProvider/WorkflowProvider interfaces, Caddy-style init() registration
- `internal/skill/memory/` -- 7 memory MCP tools wrapping markdown + SQLite FTS5 search
- `internal/skill/workflow/` -- Onboarding and session handoff tools
- `internal/skill/repomap/` -- `get_repo_map` and `get_context` MCP tools wrapping the RepoMap engine; registers via daemon post-init wiring (`SetEnrichFn`) for LSP enrichment of tags
- `internal/memory/` -- Memory store, FTS5 index, fsnotify watcher
- `internal/langregistry/` -- 52-language embedded registry with YAML override, three-tier LS installer

### Layer 3: Agent Profiles & Setup
- `internal/profile/` -- 5 agent profiles (claude-code, codex, ide-assistant, ci-bot, full), 4 modes (read/edit/review/admin)
- `internal/config/` -- 4-layer config: CLI > project (.helix/) > user (~/.helix/) > profile defaults
- `internal/cli/setup.go`, `internal/cli/setup_clients.go`, `internal/cli/setup_detect.go`, `internal/cli/setup_hooks.go`, `internal/cli/setup_output.go`, `internal/cli/setup_health.go` -- `helix setup <client>` installs the Helix Agent Skill + hooks (Claude-family clients) and idempotently tears down any prior Helix MCP-server registration across 7 clients (Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, OpenCode, generic), with language detection, LS pre-installation, and the Claude Code hook installer. Non-skill clients (vscode, jetbrains, gemini-cli, opencode, generic) get MCP-teardown only — no skill is written. The binary's internal MCP daemon head is left intact.
- `internal/cli/status.go`, `internal/cli/status_output.go` -- `helix status` CLI producing human-readable workspace health summary (`--json`, `--verbose` modes)
- `cmd/helix/main.go` -- single entrypoint; all CLI subcommands (setup, status, activate, deactivate, nudge, root, daemon wiring) live in `internal/cli/` and are mounted via cobra in `internal/cli/root.go`

### MCP Middleware Stack (installed in `internal/daemon/daemon.go` steps 14, 14b, 14c)
Four real middlewares, defined in `internal/mcp/`:
- `TelemetryMiddleware` (middleware.go) -- absorbs the pre-v1.2 logging middleware; emits RED metrics on `tools/call`, injects per-tool deadlines via `BudgetFunc`, classifies outcomes (success / timeout / circuit_open / internal / ...)
- `ProfileFilterMiddleware` (middleware.go) -- filters `tools/list` by active profile; ALSO applies brief descriptions from `ToolRegistry.BriefDescriptions()` (middleware.go:286-294) and then profile-specific description overrides. Brief descriptions are NOT a separate middleware — they piggyback on the profile filter's `tools/list` pass.
- `SuggestionMiddleware` (suggest.go) -- enriches parameter-typo and enum-value errors with "Did you mean?" suggestions via Levenshtein distance on tool schemas; never redirects to a different tool
- `LazyInitMiddleware` (lazy_init.go) -- `sync.Once` per workspace path; transparently activates the workspace on first tool call, serializes concurrent first calls

### Daemon Bootstrap (`internal/daemon/daemon.go`)
- Creates language registry, installer, kernel with pool, GrammarRegistry (23 languages), TagCache (SQLite, persistent)
- Imports all skill packages via blank imports (`imports.go`) for init() registration
- Calls `skill.InitAll()`, registers all tools centrally with MCP SDK
- Post-init wiring: `SetEnrichFn` for RepoMap LSP enrichment, `SetActivateCallback` for workspace activation, FallbackExtractor for languages without tree-sitter coverage
- Installs middleware in three steps (14, 14b, 14c), resolves active profile
- Fail-fast for core subsystems, degraded mode for optional providers
- Kernel-first shutdown ordering

## Technology Stack

- **Language:** Go (single binary, native concurrency)
- **MCP:** Official MCP Go SDK
- **RepoMap:** `github.com/tree-sitter/go-tree-sitter` with 23 language grammars (including locally vendored Swift and R bindings), hand-rolled PageRank (~60 LOC)
- **Config:** koanf v2 (4-layer precedence)
- **Database:** modernc.org/sqlite (CGO-free, for FTS5 memory search)
- **Tree-sitter:** go-tree-sitter (body extraction for symbol editing)
- **IPC:** gRPC (forwarder-daemon communication)
- **CLI:** cobra v1.9.1 -- the agent interface; agents drive `helix <verb>` via Bash
- **Protocol:** MCP Go SDK + gRPC IPC retained as internal daemon plumbing (not an agent-facing surface)
- **LSP:** LSP 3.17 (generated types from official metamodel)

**Build pipeline (CGO=1, split-runner per Phase 59.1):**

- **Source tree:** single-mode `CGO_ENABLED=1`. The previous CGO=0 stub
  apparatus (Phase 51.1 D-02) was removed in Phase 59.1; the source tree
  no longer carries `//go:build cgo` constraints (except the platform-
  conditional `internal/semantic/store/duckdb.go` `!(windows && arm64)`
  from D-14, which is platform-conditional, not CGO-conditional).
- **Local builds:** `make build` uses your host CC. zig is NOT required for
  local development.
- **Local releases (partial):** `make release-snapshot` requires `zig` on
  PATH; produces a host-platform partial matrix only (a darwin contributor
  gets 2 darwin archives via Apple clang; a linux contributor with zig
  gets 4 linux+windows archives via zig cc). Full 6-archive matrix is
  CI-only.
- **CI builds (split-runner per D-15):**
  - **`ubuntu-22.04`:** builds linux/{amd64,arm64} + windows/{amd64,arm64}
    (4 archives) via `CC="zig cc -target <triple>"`. zig pinned to
    0.14.1 via SHA-pinned `mlugg/setup-zig` action.
  - **`macos-14`:** builds darwin/{amd64,arm64} (2 archives) natively via
    Apple clang against the Xcode 15.x SDK. NOT signed with Apple
    Developer ID; cosign Sigstore keyless attestation is the only
    signature.
  - **Merge job (`ubuntu-22.04`):** stitches both runners' partial
    artifacts; runs cosign keyless attestation uniformly across all 6
    archives; publishes the GitHub Release.
- **Reproducibility (per-target within-runner Pass-1 ≡ Pass-2):** each
  runner rebuilds its own targets twice in the same job and asserts
  byte-identical sha256s. Cross-runner byte-equality is not asserted.

## Key Patterns

### Tool Registration
- Kernel tools use `RegisterTools(server *mcp.SerenaMCPServer, ...)` with typed args + `mcpsdk.AddTool` (the `SerenaMCPServer` Go identifier is retained for internal-API stability per Phase 52-03 SUMMARY; user-facing MCP `Implementation.Name` is `helix`)
- Skill tools use `ToolProvider.Tools()` returning `[]*mcp.ToolDef`, daemon registers centrally
- Kernel tools wrapped as thin skill adapters for uniform ToolProvider interface
- "Skills for composition, tool names for execution"
- Full tool inventory (50 frozen `helix` verbs with profile/mode matrix; the README table renders 51 rows because `analyze-blast-radius` is dual-categorized) is auto-generated in `README.md`; do not hand-edit the tool table

### Skill System
- Caddy-style `init()` registration: `skill.Register(&MySkill{})`
- `skill.InitAll(deps)` initializes all skills with shared dependencies
- `skill.ToolProviders()` returns all skills implementing ToolProvider
- Kernel-resident tools (`internal/kernel/health/`, `internal/kernel/help/`) are exposed as skills via their `skill_adapter.go`, keeping the ToolProvider surface uniform

### Configuration
- 4-layer precedence: CLI flags > project `.helix/project.yml` > user `~/.helix/helix_config.yml` > profile defaults
- Profile YAMLs define tool subsets, description overrides, mode transitions
- `config.ResolveProfile()` bridges config profile name to ProfileStore

### Worker Pool
- Share-until-dirty: clean sessions share warm LS workers
- Adaptive TTL with reuse scoring
- Circuit breaking with exponential backoff for crashy workers
- Platform-aware memory pressure eviction (Linux cgroups, macOS vm_stat)
- Three-tier LS resolution: PATH lookup > managed download > helpful error

### Fuzzy Editing & RepoMap
- Edit tools fall back to `fuzzy.Match()` when exact matching fails; strategy used is reported in the result
- Ambiguous fuzzy matches are REFUSED with a diff — never silently applied
- RepoMap extraction is lazy: `walkAndExtract` populates the SQLite cache on demand; `get_repo_map` / `get_context` read from cache and fall back to tree-sitter or LSP `documentSymbol` when cache is cold
- Token budget fitting uses binary search over the elided tree; output scales to any repository size

### Setup & Hooks
- `helix setup <client>` (implemented in `internal/cli/setup*.go`) installs the embedded Helix Agent Skill (Claude-family clients) and hooks, and tears down any prior Helix MCP-server entry (e.g., `claude mcp remove`) — it no longer registers an MCP server
- Claude Code hooks installed during `helix setup claude-code` (see `internal/cli/setup_hooks.go`): SessionStart (activate workspace), PreToolUse (nudge toward symbolic tools), Stop (cleanup); `--no-hooks` opts out
- Language detection (`internal/cli/setup_detect.go`) scans the project directory for known file extensions and pre-installs LSs via the three-tier installer before installing the Helix skill + hooks (and tearing down any prior MCP registration)

### Middleware Execution Order (LIFO)
Install order in `internal/daemon/daemon.go` is:
1. `InstallMiddleware` -- adds `TelemetryMiddleware` then `ProfileFilterMiddleware` (step 14)
2. `InstallSuggestionMiddleware` -- adds `SuggestionMiddleware` (step 14b)
3. `InstallLazyInitMiddleware` -- adds `LazyInitMiddleware` LAST (step 14c)

Because `mcp-go-sdk.AddReceivingMiddleware` composes in LIFO order, the execution order on an incoming request is the reverse of the install order:

`LazyInitMiddleware` → `SuggestionMiddleware` → `ProfileFilterMiddleware` (applies brief descriptions on `tools/list`) → `TelemetryMiddleware` → tool handler

LazyInit MUST run first so the workspace is activated before `TelemetryMiddleware` applies its per-tool deadline (see the install-order comment at `internal/mcp/lazy_init.go:106-108`). Any change to this order must preserve that invariant.

## Constraints

- **Language**: Go -- single binary, native concurrency
- **Interface**: the `helix` CLI is the agent interface; the MCP Go SDK + gRPC IPC are retained as internal daemon plumbing
- **LSP only**: No JetBrains or proprietary backends
- **Repo**: Go-only; the historical Python reference tree has been removed (see git history)

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.

## Helix CLI tool routing (this product's agent surface)

Agents drive Helix via the `helix <verb>` CLI (Bash), not MCP. Prefer these terse, `relpath:line:col`-anchored verbs over `grep`/`sed`/`cat`/`Read` — they parse the AST and (for first-class languages) consult the LSP, returning real definitions, callers, and type-resolved references instead of string matches. Every verb below is a real frozen `helix` command (the full inventory is auto-generated in README.md; kebab form = the tool name with `_`→`-`).

| Question | Use this | Not this |
|---|---|---|
| Where is symbol `X` defined? | `helix go-to-definition` | `Grep "X"` |
| Who references / calls `X`? | `helix find-references` | `grep -r "X"` |
| Who calls into `Y` (call tree)? | `helix get-call-hierarchy` | manual grep chain |
| Implementations of an interface | `helix find-implementations` | `grep "implements"` |
| Type relationships (super/sub) | `helix get-type-hierarchy` | read + reason |
| File outline (symbols in a file) | `helix get-symbol-overview` | `Read <file>` |
| Find symbols by name across repo | `helix search-symbols` | `grep "func X"` |
| Type / signature at a location | `helix get-hover-info` | infer by reading |
| Blast radius of a change | `helix analyze-blast-radius` | manual trace |
| Rename a symbol across files | `helix rename-symbol` | `sed` |
| Replace a function/method body | `helix replace-symbol-body` | line-number edit |
| Insert before / after a symbol | `helix insert-before-symbol` / `helix insert-after-symbol` | regex edit |
| Delete a symbol safely | `helix safe-delete-symbol` | `sed -d` |
| Fuzzy / drift-tolerant text edit | `helix fuzzy-edit` | brittle exact patch |
| Replace text in a file | `helix replace-in-file` | `sed -i` |
| Search text across the repo | `helix search-in-files` | `grep -r` |
| Find files by name/glob | `helix find-files` | `find` |
| Read a file | `helix read-file` | `cat` |
| Diagnostics for a file | `helix get-diagnostics` | parse build output |
| Available code actions / quick-fixes | `helix get-code-actions` | manual fix |
| Format code | `helix format-code` | hand-format |
| Ranked structural repo overview | `helix get-repo-map` | read many files |
| Context bundle around a symbol | `helix get-context` | several reads |
| Project / session memory | `helix read-memory` / `helix write-memory` / `helix search-memories` | ad-hoc notes |

Helix ships NO taint/CFG/IR/slice tools — those are SMTC-only (below). Do not invent `helix` verbs; cite only names that appear in `internal/cli/verbs_gen.go`.

> NOTE: the `## Code intelligence: SMTC-first tool routing` section below describes an **external** dev MCP server (show-me-the-code, the `smtc` MCP tools), NOT Helix's own tools — do not conflate the two. SMTC is a development-environment concern for working in this repo; the `helix <verb>` table above is Helix's product surface.

## Code intelligence: SMTC-first tool routing

For code-aware operations, prefer SMTC MCP tools over `Bash` / `Grep` / `Read`. SMTC parses the AST and (for first-class languages) consults the LSP, so it returns *semantic* results — actual definitions, real callers, type-resolved references — instead of string matches. Pick by the question being asked, not by tool habit.

### Decision matrix

| Question | Use this | Not this |
|---|---|---|
| Where is symbol `X` defined? | `mcp__smtc__goto_definition` | `Grep "X"` |
| Find declarations matching a pattern | `mcp__smtc__find_declarations` | `grep "func \w+"` |
| Who calls function `Y`? | `mcp__smtc__get_callers` / `find_references` | `grep -r "Y("` |
| What does `Y` call? | `mcp__smtc__get_callees` | manual Read chain |
| Can `A` transitively reach `B`? | `mcp__smtc__get_reachability` | recursive grep |
| Multi-source / multi-sink reachability | `mcp__smtc__get_multi_reachability` | — |
| Callers + callees around `Y` | `mcp__smtc__get_neighborhood` | two grep passes |
| All references to a symbol | `mcp__smtc__find_references` | `Grep "name"` |
| Call sites by signature / receiver / arg | `mcp__smtc__find_call_sites_matching` | regex on file |
| File outline (declarations + imports) | `mcp__smtc__list_file_outline` | `Read <file>` |
| Type hierarchy (super / sub) | `mcp__smtc__get_type_hierarchy` | `grep "extends"` |
| Type of an expression | `mcp__smtc__get_type_info` | infer by reading |
| Imports of a file or directory | `mcp__smtc__get_import_graph` | `grep "^import"` |
| Field reads / writes (Java) | `mcp__smtc__get_field_flow` | grep field name |
| Def-use chains within a function | `mcp__smtc__get_dataflow` | manual trace |
| Cross-function data flow | `mcp__smtc__get_interprocedural_flow` | manual trace |
| Control flow graph of a function | `mcp__smtc__get_cfg` | read + reason |
| SSA / IR view | `mcp__smtc__get_ir` | — |
| Backward / forward / thin slice | `mcp__smtc__get_slice` | — |
| Tree-sitter S-expression query | `mcp__smtc__run_ast_query` | complex regex |
| Find by annotation / kind / visibility | `mcp__smtc__search_pattern` | regex |
| Literal values (strings, numbers, bools) | `mcp__smtc__find_literals` | `Grep '"…"'` |
| Syntactic identifier occurrences | `mcp__smtc__find_name_occurrences` | `Grep` |
| Externally reachable entry points | `mcp__smtc__find_entry_points` | grep `@RestController` |
| Taint sinks by CWE category | `mcp__smtc__find_taint_sinks` | grep `Runtime.exec` |
| Trace taint source → sink | `mcp__smtc__trace_taint_path` | manual reasoning |
| Why X reaches Y (with chain) | `mcp__smtc__why_reaches` / `explain_taint_path` | — |
| Sibling methods missing a check | `mcp__smtc__find_defense_gaps` | — |
| Unsafe deserialization reachable from net | `mcp__smtc__find_unsafe_deser` | grep `readObject` |

### Activation (only when the workspace language matches a capability)

Security tools are gated behind capability activation, and capabilities are **language-specific**. Always run `mcp__smtc__list_capabilities` first; only activate a capability whose language matches the workspace.

- **Java projects** → `mcp__smtc__activate_capabilities(["java-security"])`
- **Go / Rust / TS / JS / Python / others** → no security capability ships today; skip activation and treat the security rows of the decision matrix as not applicable
- **This repo (Helix, Go-native)** → no security capability available; do **not** activate `java-security` here. The security rows exist in the matrix for cross-project portability of this guidance, not for use against this codebase.

Tools gated behind activation: `find_taint_sinks`, `trace_taint_path`, `find_entry_points`, `find_unsafe_deser`, `find_defense_gaps`, `explain_taint_path`, `trace_to_sources`, `why_reaches`.

### When grep / Bash / Read IS still correct
- Free-text search in comments, READMEs, docstrings, log messages
- Non-code files: YAML, JSON, TOML, Markdown, Dockerfiles, shell scripts
- You don't yet know the symbol name — grep first to find candidates, then switch to SMTC once you have a name to anchor on
- Build / test output inspection, env-var checks, file-system shape

### Anti-patterns (do not do these)
- `Grep "func X"` to locate a definition → use `goto_definition` or `find_declarations` (handles overloads, receivers, generics)
- `Grep "X("` to find callers → use `get_callers` (resolves dynamic dispatch, ignores comments / strings)
- `Read file.go` to understand structure → use `list_file_outline` (smaller, just the shape)
- Multiple `Grep` passes to trace a data flow → use `get_interprocedural_flow` or `get_slice`
- Manual recursion through callers / callees → use `get_reachability` or `get_multi_reachability`
- Grep-based security audit → activate `java-security`, then `find_taint_sinks` + `trace_taint_path`
- `Read` an entire large file just to check one symbol's definition → `goto_definition`

### Language tier matters
SMTC accuracy depends on language tier:
- **First-class** (full LSP-backed resolution): Java, Go, Rust, TypeScript, JavaScript
- **Best-effort** (tree-sitter only, no cross-file symbol resolution): C, C++, PHP, Kotlin, Python, Ruby, Swift, HTML, CSS, others

For first-class languages there is **no efficiency reason** to fall back to grep for semantic questions. For best-effort languages SMTC tools still work but `find_references` / `goto_definition` may miss cross-file edges — in that case, SMTC for the in-file part, grep to widen.

## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
