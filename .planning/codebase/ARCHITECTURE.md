# Architecture

**Analysis Date:** 2026-04-07

## Pattern Overview

**Overall:** Serena is a modular, multi-layered coding agent toolkit with a dual-stack backend supporting both Language Server Protocol (LSP) and JetBrains IDEs. The architecture follows a clear separation of concerns with a central orchestrator (`SerenaAgent`) coordinating language servers, tool execution, and MCP protocol exposure.

**Key Characteristics:**
- **Pluggable backends**: LSP (language servers) or JetBrains IDE integrations
- **Tool-driven interactions**: 40+ composable tools exposed via MCP protocol
- **Configuration-driven behavior**: Contexts and modes customize tool sets and prompts without code changes
- **Multi-language support**: 19+ languages via LSP servers with automatic runtime dependency management
- **Project-aware**: Per-project memory persistence, configuration, and tool sets

## Layers

**MCP Protocol Layer (`src/serena/mcp.py`):**
- Purpose: Expose SerenaAgent tools as a Model Context Protocol server
- Location: `src/serena/mcp.py`
- Contains: `SerenaMCPFactory` for server creation, tool schema generation, OpenAI compatibility transformation
- Depends on: SerenaAgent, Tool instances, configuration
- Used by: AI agents via MCP-compliant clients (Claude, ChatGPT, etc.)
- Converts Serena Tools to MCP tool specifications with parameter validation and docstring parsing

**Agent Orchestration Layer (`src/serena/agent.py`):**
- Purpose: Central orchestrator managing project activation, tool registry, modes, and context
- Location: `src/serena/agent.py`
- Contains: `SerenaAgent` (main orchestrator), `ToolSet` (tool filtering), `ActiveModes` (mode management), `AvailableTools` (tool exposure)
- Depends on: Tool registry, language server manager, project manager, configuration
- Used by: MCP server, CLI, JetBrains plugins
- Manages lifecycle: tool instantiation, language server initialization, project switching, mode activation

**Configuration & Customization Layer (`src/serena/config/`):**
- Purpose: Define execution contexts and operational modes
- Location: `src/serena/config/context_mode.py`, `src/serena/config/serena_config.py`
- Contains:
  - `SerenaAgentContext`: Tool availability and descriptions per integration context (IDE, agent, desktop-app, etc.)
  - `SerenaAgentMode`: Operational patterns (planning, editing, interactive) with tool inclusion/exclusion
  - `SerenaConfig`: Global configuration (language backend, logging, tool defaults)
  - `RegisteredProject`: Project metadata and per-project overrides
- Depends on: YAML loading, project registry
- Used by: SerenaAgent, Tool instantiation, prompt generation

**Tool System (`src/serena/tools/`):**
- Purpose: Provide composable, reusable code operations
- Location: `src/serena/tools/`
- Contains: Base `Tool` class and specialized tools:
  - `file_tools.py`: File I/O (read, create, append, delete)
  - `symbol_tools.py`: LSP-based symbol operations (find, navigate, edit)
  - `memory_tools.py`: Project memory persistence (read, write, delete)
  - `config_tools.py`: Project activation and configuration queries
  - `cmd_tools.py`: Command execution (shell, npm, etc.)
  - `query_project_tools.py`: Project metadata queries
  - `workflow_tools.py`: Onboarding and initialization
  - `jetbrains_tools.py`: JetBrains IDE integration
- Depends on: Projects, language servers, code editors
- Used by: MCP protocol, agents, IDEs
- Each tool has markers (`ToolMarker*` classes) indicating: editing capability, LSP requirements, optional status, beta status

**Language Server Wrapper (`src/solidlsp/ls.py`):**
- Purpose: Unified interface to multiple language servers via LSP
- Location: `src/solidlsp/ls.py`
- Contains: `SolidLanguageServer` - manages LSP communication, file buffers, caching, and symbol operations
- Depends on: LSP protocol handler, language-specific servers, file system
- Used by: Tools (via LanguageServerSymbolRetriever), symbol operations
- Handles: Request/response marshaling, file version tracking, symbol caching, error recovery

**Language Server Process Management (`src/solidlsp/ls_process.py`):**
- Purpose: Spawn and manage language server processes
- Location: `src/solidlsp/ls_process.py`
- Contains: `LanguageServerProcess` - subprocess lifecycle, stdio communication, restart logic
- Depends on: Platform-specific process management, subprocess utilities
- Used by: SolidLanguageServer
- Manages: Process spawning, PID tracking, signal handling, automatic restart on crash

**Project Context (`src/serena/project.py`):**
- Purpose: Encapsulate project-specific operations and state
- Location: `src/serena/project.py`
- Contains:
  - `Project`: Project root, config, language servers, file operations
  - `MemoriesManager`: Project-local and global markdown-based memory files
- Depends on: File system, .serena project directory, language server manager
- Used by: Tools, SerenaAgent
- Manages: Project metadata, .gitignore handling, memory persistence, line-ending conventions

**Language Server Manager (`src/serena/ls_manager.py`):**
- Purpose: Manage language server instances per project
- Location: `src/serena/ls_manager.py`
- Contains: `LanguageServerManager`, `LanguageServerFactory`
- Depends on: SolidLanguageServer, project configuration
- Used by: Project, Symbol tools
- Manages: Multi-language server lifecycle, parallel startup, fallback servers

**Prompt & Templating (`src/interprompt/`):**
- Purpose: Generate system prompts from Jinja2 templates with context variables
- Location: `src/interprompt/`
- Contains: `JinjaTemplate` (template rendering), `MultiLangPrompt` (language-specific prompt fallback)
- Depends on: Jinja2
- Used by: SerenaAgent, mode configuration
- Provides: Dynamic prompt generation based on active tools, modes, and project context

## Data Flow

**Initialization Flow:**

1. **Startup** (`src/serena/cli.py` or `src/serena/mcp.py`)
   - Load SerenaConfig from `~/.serena/serena_config.yml` or command-line overrides
   - Load SerenaAgentContext (e.g., "agent" for MCP server)
   - Create SerenaAgent with configuration

2. **SerenaAgent.__init__** (`src/serena/agent.py`)
   - Instantiate all Tool subclasses from `ToolRegistry`
   - Compute base tool set based on language backend, context, and modes
   - Activate startup project (if provided)
   - Initialize language servers for active project's languages
   - Start web dashboard (if enabled)

3. **Project Activation** (via `ActivateProjectTool` or startup)
   - Load `.serena/project.yml` or register project by path
   - Create `LanguageServerManager` from configured languages
   - Spawn LSP processes for each language in parallel
   - Update active modes based on project configuration

**Tool Execution Flow:**

1. **MCP Call** (from AI agent)
   - FastMCP server receives tool invocation with parameters

2. **Tool Dispatch** (`src/serena/mcp.py::SerenaMCPFactory._set_mcp_tools`)
   - Look up tool instance by name
   - Invoke `Tool.apply_ex()` with parameters

3. **Tool Execution** (`src/serena/tools/tools_base.py::Tool`)
   - Access active project via `self.project` (raises if none active)
   - Perform operation (file read/write, symbol lookup, etc.)
   - Log execution and timing
   - Return result as string

4. **Symbol Lookup** (for symbol tools)
   - Create `LanguageServerSymbolRetriever` (wraps language server manager)
   - Request symbols via LSP `textDocument/documentSymbol` or `workspace/symbol`
   - Cache results in language server's file buffer
   - Return unified symbol representation

5. **Result Return**
   - Tool result converted to JSON for MCP response
   - Logged to web dashboard and memory log handler

**State Management:**

- **Tool State**: Stateless; access project/language servers on demand
- **Project State**: Held in `SerenaAgent._active_project`; persists for tool session
- **Language Server State**: Persistent processes with file buffer versioning; auto-restarts on crash
- **Mode/Context State**: Loaded on demand; can change during session
- **Memory State**: Markdown files in `.serena/memories/` or `~/.serena/memories/global/`

## Key Abstractions

**Tool:**
- Purpose: Represents a single, composable action (read file, find symbol, replace code, etc.)
- Examples: `ReadFileTool`, `FindSymbolTool`, `WriteMemoryTool`, `ReplaceContentTool`
- Location: `src/serena/tools/tools_base.py` (base), individual files for implementations
- Pattern: Inherit from `Tool`, implement `apply()` method with typed parameters, use markers for capabilities

**ToolRegistry:**
- Purpose: Central registry of all available tools with discovery and factory methods
- Location: `src/serena/tools/__init__.py`
- Pattern: Singleton; auto-discovers Tool subclasses; provides name-to-class mapping

**ToolSet:**
- Purpose: Represent a filtered subset of tools based on inclusion/exclusion rules
- Location: `src/serena/agent.py`
- Pattern: Apply `ToolInclusionDefinition`s to create new tool sets; supports legacy tool name mapping

**Project:**
- Purpose: Encapsulate all project-specific state and operations
- Examples: Project root, source languages, file paths, language servers, memories
- Location: `src/serena/project.py`
- Pattern: Singleton per SerenaAgent session; lazily initialized on project activation

**MemoriesManager:**
- Purpose: Persistent, markdown-based knowledge storage
- Pattern: Project-local memories in `.serena/memories/`; global memories in `~/.serena/memories/global/`
- Scope: Topic-based organization via "/" separators; read-only and ignored patterns for protection

**SolidLanguageServer:**
- Purpose: Unified LSP client wrapping language-specific servers
- Location: `src/solidlsp/ls.py`
- Pattern: One instance per language per project; manages file buffers, caching, request marshaling

**Language Server Config:**
- Purpose: Language-specific server setup (binary paths, arguments, initialization options)
- Location: `src/solidlsp/ls_config.py`
- Pattern: Per-language subclass; defines LSP server implementation and runtime dependencies

## Entry Points

**CLI (`src/serena/cli.py`):**
- Location: `src/serena/cli.py`
- Triggers: User runs `serena` command via entry point
- Responsibilities: Parse arguments, manage command lifecycle, project discovery, tool invocation
- Core commands: activate-project, init, config, start-mcp-server

**MCP Server (`src/serena/mcp.py`):**
- Location: `src/serena/mcp.py`
- Triggers: AI agent connects via MCP protocol
- Responsibilities: Create SerenaMCPFactory, instantiate SerenaAgent, expose tools as MCP server
- Flow: SerenaMCPFactory.create_mcp_server() -> starts FastMCP -> waits for tool calls

**Prompt Factory (`src/serena/prompt_factory.py`):**
- Location: `src/serena/prompt_factory.py`
- Triggers: SerenaAgent needs to format system prompt
- Responsibilities: Render Jinja2 template from mode with agent context variables
- Used by: Mode prompt generation for language model system message

## Error Handling

**Strategy:** Layered error recovery with graceful degradation

**Patterns:**

- **Language Server Crashes**: `LanguageServerProcess` automatically restarts on error; file buffers invalidated
- **Tool Execution Errors**: Wrapped in try/catch by MCP layer; errors logged and returned to client
- **Missing Project**: Tools raise `ValueError` if no active project; MCP handles and formats error
- **LSP Timeouts**: Configurable timeout per request; raises `LSPError` which propagates
- **Invalid Paths**: `Project.validate_relative_path()` prevents directory traversal before file operations
- **Encoding Issues**: File operations use project-specific encoding (default UTF-8); explicit handling in file tools

## Cross-Cutting Concerns

**Logging:** 
- Framework: Python `logging` module with structured logging via `sensai.util.logging`
- Patterns: Module-level loggers with level control via config; memory log handler for dashboard
- Files: `src/serena/util/logging.py` - custom handler `MemoryLogHandler` captures logs for web UI

**Validation:**
- File paths: `Project.validate_relative_path()` checks .gitignore, ignores patterns, bounds
- Relative path enforcement: No absolute paths allowed in tool parameters
- Tool parameters: Pydantic models auto-validate via `FuncMetadata`

**Authentication:**
- LSP: Per-server configuration (environment variables for creds, API keys)
- Project: No project-level auth; assumes file system access is granted
- Secrets: Tool operations do not handle secrets; delegation to IDE/environment

**Concurrency:**
- Language servers: Run in separate processes (isolated from Python asyncio)
- Tools: Execute in single executor thread (`TaskExecutor`) to maintain linear execution
- Startup: Parallel language server process spawning via threading
- Dashboard: Separate thread for web server

**Multi-tenancy:**
- Project-local: Memories, language servers, state isolated per project
- Global: Memories shared via `~/.serena/memories/global/`, contexts/modes in `~/.serena/`
- Sessions: Each SerenaAgent instance is independent; multiple sessions can coexist

---

*Architecture analysis: 2026-04-07*
