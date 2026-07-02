# Codebase Structure

**Analysis Date:** 2026-04-07

> **⚠ STALENESS BANNER (added 2026-07-01, v2.13 docs refresh).** Everything
> below this addendum describes the **pre-Go Python `serena` codebase**
> (`src/serena/`, `src/solidlsp/`, `SerenaAgent`, Python `*_tools.py`) as it
> stood on 2026-04-07 — the map's sole `git` commit. That tree was **removed**
> from the repo (see CHANGELOG > v1.9 rename and the pre-v1.12 legacy removal);
> Helix now ships as a **single Go binary** (`cmd/helix`, `internal/**`). This
> file has NOT tracked the v1.9→v2.13 Go rewrite (~13 milestones). Treat the
> Python layout below as **historical only**. A dedicated re-mapping pass is
> recommended — see `.planning/milestones/v2.13-DOCS-REFRESH.md`. The single
> current-tree section that follows is the only part reflecting the shipped Go
> codebase.

## v2.13 Addendum — Intraprocedural Data-Flow Subsystem (current Go tree)

*Scope-limited current-tree entry added for the v2.13 milestone close; the rest
of this file is stale (see banner).*

**`internal/semantic/dataflow/` — case-1 + in-body intraprocedural flow engine:**
- Purpose: computes a per-function **case-1 flow summary** — for each parameter,
  the exact syntactic def-use targets its value reaches (the function's return,
  and call-argument positions), with **no over-approximation**. v2.13 extends it
  to model **in-body origins**: the return value of an in-body call
  (`y := producer(); sink(y)`) flowing into a later call argument.
- Key types (`summary.go`): `Summary{Params []ParamFlow, InBodyFlows []InBodyFlow}`;
  `Origin{Param int, Callee string}` (Callee=="" ⇒ param origin, else callReturn
  origin); `InBodyFlow{Producer, Consumer string, ArgPos int}`;
  `ParamFlow{Name, Index, Returns, CallArgs}`.
- Entry point: `AnalyzeFlow(node, source) *Summary` — walks a function-declaration
  tree-sitter node; returns nil under a generalized anti-vacuity gate (no param
  reaches a target AND no in-body flow recorded).
- **Leaf package**: stdlib + tree-sitter only (mirrors the `minhash` / `relatedidx`
  / `classifier` leaf boundary). Invoked from the shared
  `extract.FingerprintBody` seam (`fingerprint.go:54`), so **all 11 language
  providers** inherit flow-summary computation with no per-provider edit; the
  result rides on `ExtractedSymbol.FlowSummary` (`fact.go:127`).
- Tests: `dataflow_test.go`, `dataflow_matrix_test.go` (all-11-grammar unit matrix).

**Daemon emission (`internal/daemon/semantic_similarity_edges.go`) — feeds `DATA_FLOWS`:**
- `dataFlowEdges` (v2.9) — `caller.param → callee.param`, Source `def_use`, conf 0.55.
- `inBodyDataFlowEdges` (v2.13) — `producer.function → consumer.param`, Source
  `def_use_inbody`, conf 0.50 (a return value has no distinct graph node, so it
  honestly anchors on the producer's *function* node — D-ANCHOR).
- `returnBridgeEdges` (v2.13) — `param → enclosing-function`, Source
  `def_use_return`, conf 0.55; the minimal zero-schema multi-hop connector
  (`producer.fn → transform.param → transform.fn → sink.param`).
- All three share the anti-mis-bind guard (`nameCount==1` ⇒ no fabricated edge)
  and directed dedup on `(SrcNodeID, DstNodeID)`. Wired in `factsFromExtracted`
  (`semantic_wiring.go:2546-2554`) **strictly after** `dataFlowEdges` — EdgeID is
  a dense append-order stamp, so ordering is an M1 hard constraint.
- v2.13 also widened `langFromExt` (`semantic_wiring.go:1838`) to map
  `.rs/.kt/.kts/.php/.rb → rust/kotlin/php/ruby`, so **all 11 languages** index +
  emit `DATA_FLOWS` through the real daemon (their type resolvers stay nil-stubs
  ⇒ no `has_type`/`uses_type`). Read surface + full ledger: `docs/edge-types.md`.

## Directory Layout

```
serena/
├── src/
│   ├── serena/                          # Main agent and tools package
│   │   ├── agent.py                     # SerenaAgent orchestrator
│   │   ├── cli.py                       # Click CLI interface
│   │   ├── mcp.py                       # MCP server factory
│   │   ├── project.py                   # Project context and memory management
│   │   ├── ls_manager.py                # Language server lifecycle management
│   │   ├── code_editor.py               # Code editing operations
│   │   ├── symbol.py                    # Symbol retrieval and manipulation
│   │   ├── task_executor.py             # Linear task executor for tool execution
│   │   ├── prompt_factory.py            # System prompt generation
│   │   ├── dashboard.py                 # Web dashboard API and viewer
│   │   ├── jetbrains/                   # JetBrains IDE plugin integration
│   │   ├── config/
│   │   │   ├── context_mode.py          # Context and Mode configuration classes
│   │   │   └── serena_config.py         # Global Serena configuration and project registry
│   │   ├── tools/                       # Tool implementations
│   │   │   ├── tools_base.py            # Tool base class and markers
│   │   │   ├── file_tools.py            # File I/O tools
│   │   │   ├── symbol_tools.py          # LSP symbol operation tools
│   │   │   ├── memory_tools.py          # Project memory tools
│   │   │   ├── config_tools.py          # Configuration and project activation tools
│   │   │   ├── cmd_tools.py             # Command execution tools
│   │   │   ├── workflow_tools.py        # Onboarding and workflow tools
│   │   │   ├── query_project_tools.py   # Project metadata query tools
│   │   │   ├── jetbrains_tools.py       # JetBrains-specific tools
│   │   │   └── __init__.py              # Tool registry and exports
│   │   ├── resources/
│   │   │   └── config/
│   │   │       ├── contexts/            # Built-in context configurations (agent, ide, chatgpt, etc.)
│   │   │       ├── modes/               # Built-in mode configurations (planning, editing, interactive, etc.)
│   │   │       ├── internal_modes/      # Internal modes (jetbrains, etc.)
│   │   │       └── prompt_templates/    # System prompt and tool output templates
│   │   ├── util/                        # Utility modules
│   │   │   ├── logging.py               # MemoryLogHandler and logging utilities
│   │   │   ├── file_system.py           # File scanning, .gitignore parsing
│   │   │   ├── text_utils.py            # Text search and replacement utilities
│   │   │   └── [other utilities]
│   │   └── constants.py                 # Module constants and defaults
│   │
│   ├── solidlsp/                        # Language Server Protocol wrapper
│   │   ├── ls.py                        # SolidLanguageServer main implementation
│   │   ├── ls_config.py                 # Language enumeration and server configs
│   │   ├── ls_types.py                  # LSP type definitions and UnifiedSymbolInformation
│   │   ├── ls_process.py                # Language server process management
│   │   ├── ls_request.py                # LSP request building and execution
│   │   ├── ls_utils.py                  # LSP utility functions (file, text, path ops)
│   │   ├── ls_exceptions.py             # LSP-specific exceptions
│   │   ├── settings.py                  # SolidLSP settings configuration
│   │   ├── language_servers/            # Language-specific server implementations
│   │   │   ├── common.py                # RuntimeDependency and shared base classes
│   │   │   ├── gopls.py                 # Go language server
│   │   │   ├── eclipse_jdtls.py         # Java language server
│   │   │   ├── [50+ other language servers]
│   │   │   └── elixir_tools/            # Elixir-specific tools
│   │   ├── lsp_protocol_handler/        # LSP protocol implementation
│   │   │   └── server.py                # LSP server connection and communication
│   │   └── util/
│   │       ├── cache.py                 # LSP cache operations
│   │       └── subprocess_util.py       # Cross-platform subprocess utilities
│   │
│   └── interprompt/                     # Prompt templating and formatting
│       ├── jinja_template.py            # Jinja2 template wrapper
│       ├── multilang_prompt.py          # Multi-language prompt fallback logic
│       ├── prompt_factory.py            # Prompt factory interface
│       └── util/                        # Utility functions
│
├── test/
│   ├── serena/
│   │   ├── test_serena_agent.py         # SerenaAgent integration tests
│   │   ├── test_cli_project_commands.py # CLI command tests
│   │   ├── test_mcp.py                  # MCP server tests
│   │   └── [other test modules]
│   ├── solidlsp/
│   │   ├── [language]/                  # Language-specific symbol operation tests
│   │   └── util/                        # LSP utility tests
│   └── resources/
│       └── repos/                       # Test repositories for each language
│           ├── python/
│           ├── go/
│           ├── java/
│           └── [other languages]/
│
├── pyproject.toml                       # Project metadata, dependencies, tool config
├── .serena/                             # Project configuration (created per project)
│   ├── project.yml                      # Project-specific overrides
│   └── memories/                        # Project-local memories (markdown files)
└── ~/.serena/                           # User home Serena directory (created at first use)
    ├── serena_config.yml                # Global Serena configuration
    ├── contexts/                        # User-defined contexts (override built-in)
    ├── modes/                           # User-defined modes (override built-in)
    ├── prompt_templates/                # User-defined prompt templates
    ├── memories/
    │   └── global/                      # Global memories shared across projects
    └── solidlsp/                        # Language server cache and runtime deps
```

## Directory Purposes

**`src/serena/`:**
- Purpose: Core agent orchestration, tool system, and project management
- Contains: Agent orchestrator, tool registry, project activation, mode/context config
- Key files: `agent.py` (main), `cli.py` (entry), `mcp.py` (MCP protocol), `project.py` (project state)

**`src/serena/tools/`:**
- Purpose: Implements 40+ composable tools for code operations
- Contains: File I/O, symbol operations, memory management, configuration, command execution
- Organization: One file per logical group (file_tools, symbol_tools, etc.); tools auto-registered via `__init__.py`
- Base class: `Tool` in `tools_base.py` with markers for capabilities

**`src/serena/config/`:**
- Purpose: Configuration system for contexts, modes, and global settings
- Contains:
  - `context_mode.py`: `SerenaAgentContext` (tool availability by integration), `SerenaAgentMode` (operational patterns)
  - `serena_config.py`: `SerenaConfig` (global settings), `ProjectConfig` (per-project overrides), `RegisteredProject` (project registry)
- YAML-based: Loaded from `src/serena/resources/config/` built-ins or `~/.serena/` user configs

**`src/serena/resources/config/`:**
- Purpose: Built-in configuration templates (read-only)
- Contains:
  - `contexts/`: Configurations for different integration points (agent, IDE, ChatGPT, etc.)
  - `modes/`: Operational patterns (planning, editing, interactive, one-shot, onboarding)
  - `internal_modes/`: Special modes for internal use (JetBrains plugin)
  - `prompt_templates/`: Jinja2 templates for system prompt and tool output formatting

**`src/solidlsp/`:**
- Purpose: Unified LSP (Language Server Protocol) wrapper for 19+ languages
- Contains: Language server abstraction, process management, protocol handling, file buffering
- Key file: `ls.py` (main implementation, 2500+ lines)
- Configuration: `ls_config.py` defines Language enum and per-language server setup

**`src/solidlsp/language_servers/`:**
- Purpose: Language-specific server implementations
- Contains: 50+ files, one per language (gopls.py, eclipse_jdtls.py, etc.)
- Pattern: Each defines server startup command, runtime dependencies, initialization options
- Runtime deps: Auto-downloaded from URLs with SHA256 verification via `RuntimeDependency`

**`src/interprompt/`:**
- Purpose: Prompt templating and multi-language fallback
- Contains: Jinja2 template wrapper, multi-language prompt logic, factory
- Used by: Mode prompt generation, system prompt rendering

**`src/serena/util/`:**
- Purpose: Shared utility functions
- Key modules:
  - `logging.py`: `MemoryLogHandler` for capturing logs to web dashboard
  - `file_system.py`: .gitignore parsing, path validation
  - `text_utils.py`: Text search, line matching, content replacement

**`.serena/` (project-local):**
- Purpose: Per-project Serena metadata and state
- Contains:
  - `project.yml`: Language configuration, tool defaults, mode overrides
  - `memories/`: Markdown files organized by topic (auto-created on first write)
- User-controlled: Committed to version control

**`~/.serena/` (user home):**
- Purpose: Global Serena user configuration and data
- Contains:
  - `serena_config.yml`: Global defaults, project registry, language backend choice
  - `contexts/`, `modes/`, `prompt_templates/`: User overrides (inherit/override built-ins)
  - `memories/global/`: Markdown memories shared across projects
  - `solidlsp/`: LSP cache and runtime dependencies
- Managed: Created by Serena on first use; customizable via SERENA_HOME env var

## Key File Locations

**Entry Points:**
- `src/serena/cli.py`: CLI command handler (Click-based)
- `src/serena/mcp.py`: MCP server factory
- `src/serena/agent.py:SerenaAgent`: Central orchestrator class

**Configuration:**
- `src/serena/config/serena_config.py`: SerenaConfig (global), ProjectConfig (per-project)
- `src/serena/resources/config/contexts/*.yml`: Built-in contexts
- `src/serena/resources/config/modes/*.yml`: Built-in modes
- `~/.serena/serena_config.yml`: User global configuration
- `.serena/project.yml`: User project configuration

**Core Logic:**
- `src/serena/agent.py`: SerenaAgent orchestration, tool instantiation, mode/context management
- `src/serena/project.py`: Project state, memory management, language server coordination
- `src/serena/tools/tools_base.py`: Tool base class and marker definitions
- `src/solidlsp/ls.py`: SolidLanguageServer LSP client implementation

**Tool Implementations:**
- `src/serena/tools/file_tools.py`: ReadFileTool, CreateTextFileTool, ReplaceContentTool
- `src/serena/tools/symbol_tools.py`: FindSymbolTool, GetSymbolsOverviewTool, RenameSymbolTool
- `src/serena/tools/memory_tools.py`: ReadMemoryTool, WriteMemoryTool, ListMemoriesTool
- `src/serena/tools/config_tools.py`: ActivateProjectTool, GetCurrentConfigTool

**Testing:**
- `test/serena/test_serena_agent.py`: Integration tests for SerenaAgent
- `test/serena/test_mcp.py`: MCP server tests
- `test/serena/test_cli_project_commands.py`: CLI command tests
- `test/resources/repos/`: Test repositories (python/, go/, java/, etc.) for language testing

## Naming Conventions

**Files:**
- Snake_case for Python modules: `file_tools.py`, `serena_config.py`
- Class names in CamelCase: `SerenaAgent`, `SolidLanguageServer`, `ToolRegistry`
- Tool classes: Always `*Tool` suffix: `ReadFileTool`, `FindSymbolTool`, `WriteMemoryTool`
- Exception classes: Always `*Error` or `*Exception` suffix: `ProjectNotFoundError`, `SolidLSPException`
- Config files: Lowercase with hyphens for multi-word names: `planning.yml`, `one-shot.yml`

**Directories:**
- Plural for resource collections: `tools/`, `language_servers/`, `contexts/`, `modes/`
- Singular for logical modules: `config/`, `jetbrains/`, `util/`
- Underscore for internal/private directories: `_pycache__/`, `.git/`

**Classes:**
- Base classes: Suffix with clear role: `Tool`, `ToolMarker`, `Component`, `LanguageServerConfig`
- Concrete tool implementations: Always `*Tool`: `ReadFileTool`, `RestartLanguageServerTool`
- Managers/orchestrators: Suffix with `Manager` or `Factory`: `LanguageServerManager`, `LanguageServerFactory`
- Enums: PascalCase: `Language`, `LanguageBackend`, `SymbolKind`

**Functions:**
- Lowercase with underscores: `apply()`, `get_project_root()`, `apply_ex()`
- Private/internal: Prefix with underscore: `_update_active_tools()`, `_format_prompt()`
- Factory methods: Prefix with `from_` or `create_`: `from_yaml()`, `create_mcp_server()`

## Where to Add New Code

**New Feature (e.g., add a new code editing capability):**
- Primary code: `src/serena/tools/` → Create tool class inheriting from `Tool`
  - If file-related: Add to `file_tools.py`
  - If symbol-related: Add to `symbol_tools.py`
  - If new category: Create new file matching pattern `*_tools.py`
- Markers: Add appropriate `ToolMarker*` classes to indicate capabilities
- Tests: `test/serena/test_[category].py`
- Registration: Tools auto-register via `__init__.py` imports
- Exposure: No additional config needed; tool automatically included in available tool sets

**New Language Server:**
- Language support: `src/solidlsp/language_servers/[language]_language_server.py`
  - Subclass `LanguageServerConfig` from `ls_config.py`
  - Define `RuntimeDependency` items for binaries/downloads
  - Specify LSP server command and initialization options
- Configuration: Update `Language` enum in `src/solidlsp/ls_config.py`
- Factory: Update `LanguageServerConfig.get_language_server_config()` method
- Testing: Create `test/resources/repos/[language]/` with sample code
- Tests: Create `test/solidlsp/[language]/` for symbol operations

**New Context:**
- Definition: `src/serena/resources/config/contexts/[name].yml`
  - Specify tool availability, descriptions, exclusions
  - Format: YAML with `fixed_tools` or `included_optional_tools`/`excluded_tools`
- Loading: Contexts auto-discovered from `contexts/` directory
- Usage: Reference by name in CLI or config: `--context [name]`
- User override: Copy to `~/.serena/contexts/[name].yml` to customize

**New Mode:**
- Definition: `src/serena/resources/config/modes/[name].yml`
  - Provide Jinja2 prompt template, tool inclusion/exclusion, description
- Loading: Modes auto-discovered from `modes/` directory
- Usage: Reference by name in modes list: `--mode [name]`
- User override: Copy to `~/.serena/modes/[name].yml` to customize

**Utilities & Helpers:**
- Shared utility functions: `src/serena/util/`
- LSP-specific utilities: `src/solidlsp/util/`
- Tool-shared helpers: Module-level functions in `src/serena/tools/tools_base.py`

## Special Directories

**`src/serena/jetbrains/`:**
- Purpose: JetBrains IDE plugin integration
- Generated: Contains generated proxy code for IDE communication
- Committed: Yes, part of source control

**`src/serena/generated/`:**
- Purpose: Auto-generated code (if any)
- Generated: Likely empty or minimal
- Committed: Depends on generation strategy

**`test/resources/repos/`:**
- Purpose: Sample projects for language-specific testing
- Contains: Real (small) code samples in each language
- Generated: No; hand-written test fixtures
- Committed: Yes; part of test suite

**`.serena/memories/`:**
- Purpose: Project-local persistent memories
- Generated: Yes; created on first memory write
- Committed: Yes; part of project history (user-managed content)

**`~/.serena/`:**
- Purpose: User global Serena data
- Generated: Yes; created on first run
- Committed: No; user-specific (per system)

**`~/.serena/solidlsp/`:**
- Purpose: Language server runtime dependencies cache
- Generated: Yes; auto-downloaded on first language server startup
- Committed: No; built from declared dependencies

---

*Structure analysis: 2026-04-07*
