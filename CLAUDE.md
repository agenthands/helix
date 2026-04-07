# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Go Development Commands

- `go build ./cmd/serena` - Build the serena binary
- `go test ./...` - Run all Go tests
- `go vet ./...` - Run Go vet
- `gofmt -w .` - Format Go code
- `make build` - Build via Makefile
- `make test` - Run tests via Makefile

**Always run go vet and go test before completing any Go task.**

## Legacy Python Commands (run from legacy/ directory)

- `cd legacy && uv run poe format` - Format Python code (RUFF)
- `cd legacy && uv run poe type-check` - Run mypy type checking
- `cd legacy && uv run poe test` - Run Python tests with default markers (excludes java/rust)
- `cd legacy && uv run poe test -m "python or go"` - Run specific language tests
- `cd legacy && uv run poe test -m vue` - Run Vue tests
- `cd legacy && uv run poe lint` - Check Python code style without fixing

**Test Markers:**
Available pytest markers for selective testing:
- `python`, `go`, `java`, `rust`, `typescript`, `vue`, `php`, `perl`, `powershell`, `csharp`, `elixir`, `terraform`, `clojure`, `swift`, `bash`, `ruby`, `ruby_solargraph`
- `snapshot` - for symbolic editing operation tests

**Project Management:**
- `cd legacy && uv run serena-mcp-server` - Start MCP server
- `cd legacy && uv run index-project` - Index project for faster tool performance

## Architecture Overview

Serena is a dual-layer coding agent toolkit:

### Core Components

**1. SerenaAgent (`legacy/src/serena/agent.py`)**
- Central orchestrator managing projects, tools, and user interactions
- Coordinates language servers, memory persistence, and MCP server interface
- Manages tool registry and context/mode configurations

**2. SolidLanguageServer (`legacy/src/solidlsp/ls.py`)**  
- Unified wrapper around Language Server Protocol (LSP) implementations
- Provides language-agnostic interface for symbol operations
- Handles caching, error recovery, and multiple language server lifecycle

**3. Tool System (`legacy/src/serena/tools/`)**
- **file_tools.py** - File system operations, search, regex replacements
- **symbol_tools.py** - Language-aware symbol finding, navigation, editing
- **memory_tools.py** - Project knowledge persistence and retrieval
- **config_tools.py** - Project activation, mode switching
- **workflow_tools.py** - Onboarding and meta-operations

**4. Configuration System (`legacy/src/serena/config/`)**
- **Contexts** - Define tool sets for different environments (desktop-app, agent, ide-assistant)
- **Modes** - Operational patterns (planning, editing, interactive, one-shot)
- **Projects** - Per-project settings and language server configs

### Language Support Architecture

Each supported language has:
1. **Language Server Implementation** in `legacy/src/solidlsp/language_servers/`
2. **Runtime Dependencies** - Automatic language server downloads when needed
3. **Test Repository** in `legacy/test/resources/repos/<language>/`
4. **Test Suite** in `legacy/test/solidlsp/<language>/`

### Memory & Knowledge System

- **Markdown-based storage** in `.serena/memories/` directories
- **Project-specific knowledge** persistence across sessions
- **Contextual retrieval** based on relevance
- **Onboarding support** for new projects

## Development Patterns

### Adding New Languages
1. Create language server class in `src/solidlsp/language_servers/`
2. Add to Language enum in `src/solidlsp/ls_config.py` 
3. Update factory method in `src/solidlsp/ls.py`
4. Create test repository in `test/resources/repos/<language>/`
5. Write test suite in `test/solidlsp/<language>/`
6. Add pytest marker to `pyproject.toml`

### Adding New Tools
1. Inherit from `Tool` base class in `src/serena/tools/tools_base.py`
2. Implement required methods and parameter validation
3. Register in appropriate tool registry
4. Add to context/mode configurations

### Testing Strategy
- Language-specific tests use pytest markers
- Symbolic editing operations have snapshot tests
- Integration tests in `test_serena_agent.py`
- Test repositories provide realistic symbol structures

## Configuration Hierarchy

Configuration is loaded from (in order of precedence):
1. Command-line arguments to `serena-mcp-server`
2. Project-specific `.serena/project.yml`
3. User config `~/.serena/serena_config.yml`
4. Active modes and contexts

## Key Implementation Notes

- **Symbol-based editing** - Uses LSP for precise code manipulation
- **Caching strategy** - Reduces language server overhead
- **Error recovery** - Automatic language server restart on crashes
- **Multi-language support** - 19 languages with LSP integration (including Vue)
- **MCP protocol** - Exposes tools to AI agents via Model Context Protocol
- **Async operation** - Non-blocking language server interactions

## Working with the Codebase

- Project uses Python 3.11 with `uv` for dependency management
- Strict typing with mypy, formatted with ruff
- Language servers run as separate processes with LSP communication
- Memory system enables persistent project knowledge
- Context/mode system allows workflow customization

<!-- GSD:project-start source:PROJECT.md -->
## Project

**Serena 2.0**

A Go-native code intelligence platform for MCP: universal LSP gateway at the core, agent skills as plugins. Full rewrite of the Python-based Serena, keeping the same repo and brand. Targets coding agents (Claude Code, Codex, IDE assistants) that need semantic code operations — symbol-level retrieval, editing, refactoring — backed by real language servers.

**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools — without the lifecycle/timeout/asyncio pain of the Python version.

### Constraints

- **Language**: Go — single binary, native concurrency, gopls ecosystem precedent
- **Protocol**: MCP (Model Context Protocol) — primary interface for all clients
- **LSP only**: No JetBrains or proprietary backends
- **Repo strategy**: Same repo, Python code moved to `legacy/`
- **Compatibility**: Must support the same 40+ languages currently supported via LSP
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Python 3.11+ - Core Serena agent toolkit, language server abstraction, MCP server implementation
- TypeScript/JavaScript - Vue.js dashboard frontend, language servers (node-based LSP implementations)
- Fortran - Built-in Fortran language server support (fortls dependency)
- Go, Java, Rust, C++, Kotlin, C#, PHP, Perl, Ruby, Clojure, Scala, Haskell, Elixir, Swift, Bash, Terraform, Lua, Zig, Nix, Dart, Erlang, OCaml, F#, Rego, Markdown, Julia, MATLAB, SystemVerilog, HLSL, Lean 4, Solidity, Ansible, AL - 40+ languages via Language Server Protocol adapters
- YAML - Configuration files
- Shell/Bash - Docker and deployment scripts
- SQL (SQLite) - Agno agent session persistence
## Runtime
- Python 3.11 - 3.14 (requires-python >= 3.11, < 3.15)
- Node.js 22.18.0 (in Docker container for language server support)
- Docker - Container-based deployment (Python 3.11-slim base image)
- uv 1.0+ - Fast Python package installer and resolver
- npm/yarn - For Node-based language servers
## Frameworks
- MCP (Model Context Protocol) 1.26.0 - Protocol layer for Claude integration via FastMCP
- FastMCP - Async server implementation for MCP protocol
- Flask 3.1.3 - Web dashboard backend (HTTP API, static file serving)
- Pydantic 2.12.5 - Data validation and settings management
- PyYAML 6.0.2, ruamel.yaml 0.18.14 - Configuration parsing
- pywebview (custom git rev) - Cross-platform desktop/web UI wrapper
- PyTray 0.19.5 - System tray integration
- PIL (Pillow) - Image processing for dashboard
- pytest 8.4.1 - Test runner with xdist (3.8.0) for parallelization
- syrupy 4.9.1 - Snapshot testing for symbolic editing operations
- pytest-timeout 2.4.0 - Test timeout management
- Sphinx 7.4.7 - Documentation generation
- sphinx_rtd_theme 2.0.0, sphinx_book_theme 1.1.4 - Documentation themes
- jupyter-book 1.0.4 - Interactive documentation
- mypy 1.17.0 - Static type checking
- ruff 0.12.5 - Fast linter and code formatter
- Poethepoet 0.36.0 - Task runner (poethepoet poe commands)
## Key Dependencies
- mcp 1.26.0 - Model Context Protocol for Claude integration, core communication layer
- anthropic 0.59.0 - Anthropic Claude API client (token counting, model integration)
- requests 2.33.0 - HTTP client for project server communication and JetBrains plugin API
- sensai-utils 1.5.0 - Utility library for logging, string handling, validation
- pathspec 0.12.1 - Gitignore-style path matching for project scanning
- python-dotenv 1.2.1, dotenv 0.9.9 - Environment variable loading from .env files
- psutil 7.0.0 - System process monitoring (for language server lifecycle)
- filelock 3.25.2 - Cross-platform file locking for concurrent access
- joblib 1.5.1 - Job parallelization and caching
- tqdm 4.67.1 - Progress bar visualization
- tiktoken 0.12.0 - GPT tokenizer for token estimation (fallback estimator)
- beautifulsoup4 4.14.2 - HTML parsing for dashboard generation
- docstring_parser 0.17.0 - Python docstring parsing for tool introspection
- pyright 1.1.403 - Type checking and symbol resolution
- fortls 3.2.2 - Fortran language server
- pythonnet 3.1.0-rc0 (Windows only) - .NET runtime integration for Windows-specific language servers
- urllib3 2.6.3 - HTTP client library (transitive, pinned for security)
- cryptography 46.0.6 - Cryptographic primitives (transitive, pinned for security)
- werkzeug 3.1.7 - WSGI utilities for Flask (transitive, pinned for CVE fixes)
- starlette 1.0.0 - ASGI web framework (transitive, for MCP server)
- agno 2.5.10, sqlalchemy 2.0.41 - Agno agent framework and ORM (optional, requires extras)
- google-genai 1.27.0 - Google Generative AI API (optional, requires extras)
## Configuration
- Located in: `.env` (user level) or `.env.example` (template)
- Key variables: `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`
- Project config: `.serena/project.yml` - Per-project language server and tool settings
- User config: `~/.serena/serena_config.yml` - Global preferences, dashboard settings
- Runtime config: Environment variables from `PYDEVD_DISABLE_FILE_VALIDATION`, `SERENA_HOME`, `SERENA_DOCKER`, `SERENA_PORT`, `SERENA_DASHBOARD_PORT`
- `pyproject.toml` - Main build and dependency configuration with:
- `uv.lock` - Lock file for deterministic builds
- `compose.yaml` - Docker Compose for containerized deployment
- `Dockerfile` - Multi-stage production image with Python 3.11, Node.js 22, Rust toolchain
## Platform Requirements
- Python 3.11+
- uv package manager
- Git (for version tracking and .gitignore handling)
- Node.js (for TypeScript/JavaScript language servers)
- Rust (for rust-analyzer and other Rust-based language servers)
- .NET SDK (optional, for C# language server)
- System dependencies: build-essential, curl, ssh (for language server downloads)
- Docker container or Python 3.11+ environment
- Language servers downloaded on-demand to: `~/.cache/serena/language_servers/`
- Dashboard accessible on port 24282 (configurable via `SERENA_DASHBOARD_PORT`)
- MCP server accessible on configured port (default 9121)
## Language Server Architecture
- Language enum entry in `src/solidlsp/ls_config.py`
- Implementation class in `src/solidlsp/language_servers/[language]_*.py`
- Automatic download and caching via subprocess utilities
- LSP protocol communication over stdin/stdout
- Pyright, Jedi (Python)
- Gopls (Go)
- Eclipse JDT (Java)
- Rust Analyzer (Rust)
- TypeScript Server (TypeScript/JavaScript/Vue)
- Omnisharp (C#)
- Kotlin Language Server
- Scala Language Server
- And 30+ additional language servers
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- snake_case for all Python files: `file_tools.py`, `symbol_tools.py`, `serena_config.py`
- Test files follow pattern: `test_<domain>.py` (e.g., `test_python_basic.py`, `test_serena_agent.py`)
- Package directories use snake_case: `src/serena/tools/`, `src/solidlsp/language_servers/`
- PascalCase for all class names: `ReadFileTool`, `LanguageServerManager`, `SerenaConfig`
- Tool classes end with `Tool`: `ReadFileTool`, `CreateTextFileTool`, `ReplaceContentTool`
- Enum classes inherit from StrEnum: `class LineType(StrEnum):`
- Marker classes for tools: `ToolMarkerCanEdit`, `ToolMarkerSymbolicRead`, `ToolMarkerBeta`
- snake_case for all function and method names: `apply()`, `get_symbol_overview()`, `create_language_server_manager()`
- Private/internal methods start with underscore: `_create_ls()`, `_limit_length()`
- Context managers prefixed with `start_` or suffixed with `_context`: `start_ls_context()`, `project_context()`
- snake_case for variables: `repo_path`, `original_content`, `language_server`
- Constants in UPPER_SNAKE_CASE: `DEFAULT_SOURCE_FILE_ENCODING`, `SERENA_MANAGED_DIR_NAME`
- Type variables use single uppercase letter: `T`, `TTool` (with `TypeVar` bounds where needed)
- Private instance variables prefixed with underscore: `self._tool_names`, `self._encoding`
- Boolean variables often prefixed: `is_ci`, `is_windows`, `has_malformed_name`
- Each module has a module logger: `log = logging.getLogger(__name__)`
- Docstring at module top level: `"""File and file system-related tools..."""`
## Code Style
- Tool: ruff (run via `uv run poe format`)
- Line length: 140 characters (configured in `pyproject.toml`)
- Quote style: double quotes for strings
- Indent: 4 spaces
- Docstring formatting: enabled via `docstring-code-format = true`
- Tool: ruff check (run via `uv run poe lint`)
- Configuration: `pyproject.toml` with extensive rule selection
- Key ignored rules: unused variables allowed, long lines allowed, unspecific except clauses allowed for error recovery
- Max complexity: 20 (McCabe)
- Tool: mypy (run via `uv run poe type-check`)
- Settings: strict mode with `disallow_untyped_defs = true` for core code
- Exception: `disallow_untyped_defs = false` for test code (tests use less strict typing)
- Use `TYPE_CHECKING` guard for circular import prevention: `if TYPE_CHECKING: from serena.agent import SerenaAgent`
## Import Organization
- No path aliases configured; use absolute imports from `src/` roots
- Import from public namespaces: `from serena.tools import ReadFileTool` not internal paths
- Use `TYPE_CHECKING` guards for optional/circular imports
## Error Handling
- Explicit exception types preferred over broad `except Exception`
- Language server failures use `SolidLSPException` with checks like `e.is_language_server_terminated()`
- Tool execution wraps errors in `apply_ex()` method with automatic retry on language server restart
- Custom exceptions extend Exception: `class ProjectNotFoundError(Exception): pass`
- Validation exceptions use `ValueError` with descriptive messages
- File path validation: `self.project.validate_relative_path(relative_path, require_not_ignored=True)`
- File not found: `raise FileNotFoundError(f"Relative path {relative_path} does not exist.")`
- Language server crashes trigger automatic restart via `get_language_server_manager_or_raise().restart_language_server()`
- Tool execution catches exceptions and returns error strings to LLM: `return f"Error executing tool: {e.__class__.__name__} - {e}"`
- Graceful degradation: shortened result factories try progressively shorter versions when output too long
## Logging
- Module-level logger: `log = logging.getLogger(__name__)`
- Log levels: INFO for normal operations, WARNING for non-fatal issues, ERROR for failures
- Use f-strings in log messages: `log.info(f"Starting language server for {language} {repo_path}")`
- Parameter logging in tool execution: `log.info(f"{self.get_name_from_cls()}: {dict_string(params)}")`
- Exception logging: `log.error(f"Error executing tool: {e}", exc_info=e)` includes exception info
- LSP communication tracing: optional via `trace_lsp_communication=True` parameter
- Logging configured at test startup: `configure(level=logging.INFO)` in conftest
- Log viewer available via GUI in production mode
- Default log level in tests: ERROR to reduce noise
## Comments
- Add docstrings for all public methods and classes (enforced by ruff D rules)
- Explain non-obvious algorithm choices: `# capture kind names and depth-0 snapshots before grouping, which mutates the dicts`
- Explain temporary workarounds: `# TODO: Fix when language server behavior changes`
- Document parameter constraints in docstrings
- Use triple-quoted docstrings for modules, classes, and functions
- Include parameter descriptions with type hints: `:param relative_path: the relative path to the file to read`
- Include return type description: `:return: a message indicating success or failure`
- Example from `ReadFileTool.apply()`:
## Function Design
- Prefer focused, single-responsibility functions (max complexity 20)
- Break large methods into private helper methods with leading underscore: `_create_ls()`, `_limit_length()`
- Use nested functions for context managers: `start_ls_context()` yields after logging entry
- Use type hints on all parameters: `def apply(self, relative_path: str, start_line: int = 0, ...) -> str:`
- Optional parameters with `| None`: `repo_path: str | None = None`
- Use keyword-only arguments in dataclasses: `@dataclass(kw_only=True)`
- Union types use `|` syntax (Python 3.10+): `Literal["read", "write"]`
- Avoid mutable defaults; use `None` with factory pattern: `def __init__(self, items: list[T] | None = None): self.items = items or []`
- Single return type clearly typed: `def get_name(self) -> str:`
- Union returns use `|`: `str | None`
- Strings for tool results (consumed by LLM): tools always return `str`
- Context managers use `Iterator[T]` return type: `def start_ls_context(...) -> Iterator[SolidLanguageServer]:`
- Use `SUCCESS_RESULT = "OK"` constant for successful operations
## Module Design
- Public classes/functions exposed at module level: `from serena.tools import Tool, ReadFileTool`
- Private implementation details use leading underscore: `_create_ls()`, `_disabled_languages`
- No star imports: always use explicit imports
- `src/serena/tools/__init__.py` exports public tool API
- `src/serena/__init__.py` exports version and main classes
- Each language server directory has `__init__.py` exporting main class
- Tools inherit from `Tool` base class: `class ReadFileTool(Tool):`
- Tools implement `apply()` method with documented parameters
- Tools use marker classes for capabilities: `class CreateTextFileTool(Tool, ToolMarkerCanEdit):`
- Docstring on class and on `apply()` method both required for MCP tool registration
## Dataclass Usage
- Use `@dataclass(kw_only=True)` for required keyword arguments
- Add field docstrings on the same line or after the field
- Include methods for formatting/display
- Use `field(default_factory=list)` for mutable defaults
## Code Organization Examples
- Class docstring explains purpose
- `apply()` method signature shows all parameters with types
- Docstring on `apply()` explains each parameter and return
- Implementation handles edge cases (start_line, end_line)
- Uses helper methods for output limiting: `self._limit_length(result, max_answer_chars)`
- Class per logical test group: `class TestPythonLanguageServerBasics:`
- Pytest marker on class: `@pytest.mark.python`
- Parametrized fixtures: `@pytest.mark.parametrize("language_server", PYTHON_BACKEND_LANGUAGES, indirect=True)`
- Clear test names: `test_request_references_user_class()`
- Assertion-heavy verification of expected conditions
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- **Pluggable backends**: LSP (language servers) or JetBrains IDE integrations
- **Tool-driven interactions**: 40+ composable tools exposed via MCP protocol
- **Configuration-driven behavior**: Contexts and modes customize tool sets and prompts without code changes
- **Multi-language support**: 19+ languages via LSP servers with automatic runtime dependency management
- **Project-aware**: Per-project memory persistence, configuration, and tool sets
## Layers
- Purpose: Expose SerenaAgent tools as a Model Context Protocol server
- Location: `src/serena/mcp.py`
- Contains: `SerenaMCPFactory` for server creation, tool schema generation, OpenAI compatibility transformation
- Depends on: SerenaAgent, Tool instances, configuration
- Used by: AI agents via MCP-compliant clients (Claude, ChatGPT, etc.)
- Converts Serena Tools to MCP tool specifications with parameter validation and docstring parsing
- Purpose: Central orchestrator managing project activation, tool registry, modes, and context
- Location: `src/serena/agent.py`
- Contains: `SerenaAgent` (main orchestrator), `ToolSet` (tool filtering), `ActiveModes` (mode management), `AvailableTools` (tool exposure)
- Depends on: Tool registry, language server manager, project manager, configuration
- Used by: MCP server, CLI, JetBrains plugins
- Manages lifecycle: tool instantiation, language server initialization, project switching, mode activation
- Purpose: Define execution contexts and operational modes
- Location: `src/serena/config/context_mode.py`, `src/serena/config/serena_config.py`
- Contains:
- Depends on: YAML loading, project registry
- Used by: SerenaAgent, Tool instantiation, prompt generation
- Purpose: Provide composable, reusable code operations
- Location: `src/serena/tools/`
- Contains: Base `Tool` class and specialized tools:
- Depends on: Projects, language servers, code editors
- Used by: MCP protocol, agents, IDEs
- Each tool has markers (`ToolMarker*` classes) indicating: editing capability, LSP requirements, optional status, beta status
- Purpose: Unified interface to multiple language servers via LSP
- Location: `src/solidlsp/ls.py`
- Contains: `SolidLanguageServer` - manages LSP communication, file buffers, caching, and symbol operations
- Depends on: LSP protocol handler, language-specific servers, file system
- Used by: Tools (via LanguageServerSymbolRetriever), symbol operations
- Handles: Request/response marshaling, file version tracking, symbol caching, error recovery
- Purpose: Spawn and manage language server processes
- Location: `src/solidlsp/ls_process.py`
- Contains: `LanguageServerProcess` - subprocess lifecycle, stdio communication, restart logic
- Depends on: Platform-specific process management, subprocess utilities
- Used by: SolidLanguageServer
- Manages: Process spawning, PID tracking, signal handling, automatic restart on crash
- Purpose: Encapsulate project-specific operations and state
- Location: `src/serena/project.py`
- Contains:
- Depends on: File system, .serena project directory, language server manager
- Used by: Tools, SerenaAgent
- Manages: Project metadata, .gitignore handling, memory persistence, line-ending conventions
- Purpose: Manage language server instances per project
- Location: `src/serena/ls_manager.py`
- Contains: `LanguageServerManager`, `LanguageServerFactory`
- Depends on: SolidLanguageServer, project configuration
- Used by: Project, Symbol tools
- Manages: Multi-language server lifecycle, parallel startup, fallback servers
- Purpose: Generate system prompts from Jinja2 templates with context variables
- Location: `src/interprompt/`
- Contains: `JinjaTemplate` (template rendering), `MultiLangPrompt` (language-specific prompt fallback)
- Depends on: Jinja2
- Used by: SerenaAgent, mode configuration
- Provides: Dynamic prompt generation based on active tools, modes, and project context
## Data Flow
- **Tool State**: Stateless; access project/language servers on demand
- **Project State**: Held in `SerenaAgent._active_project`; persists for tool session
- **Language Server State**: Persistent processes with file buffer versioning; auto-restarts on crash
- **Mode/Context State**: Loaded on demand; can change during session
- **Memory State**: Markdown files in `.serena/memories/` or `~/.serena/memories/global/`
## Key Abstractions
- Purpose: Represents a single, composable action (read file, find symbol, replace code, etc.)
- Examples: `ReadFileTool`, `FindSymbolTool`, `WriteMemoryTool`, `ReplaceContentTool`
- Location: `src/serena/tools/tools_base.py` (base), individual files for implementations
- Pattern: Inherit from `Tool`, implement `apply()` method with typed parameters, use markers for capabilities
- Purpose: Central registry of all available tools with discovery and factory methods
- Location: `src/serena/tools/__init__.py`
- Pattern: Singleton; auto-discovers Tool subclasses; provides name-to-class mapping
- Purpose: Represent a filtered subset of tools based on inclusion/exclusion rules
- Location: `src/serena/agent.py`
- Pattern: Apply `ToolInclusionDefinition`s to create new tool sets; supports legacy tool name mapping
- Purpose: Encapsulate all project-specific state and operations
- Examples: Project root, source languages, file paths, language servers, memories
- Location: `src/serena/project.py`
- Pattern: Singleton per SerenaAgent session; lazily initialized on project activation
- Purpose: Persistent, markdown-based knowledge storage
- Pattern: Project-local memories in `.serena/memories/`; global memories in `~/.serena/memories/global/`
- Scope: Topic-based organization via "/" separators; read-only and ignored patterns for protection
- Purpose: Unified LSP client wrapping language-specific servers
- Location: `src/solidlsp/ls.py`
- Pattern: One instance per language per project; manages file buffers, caching, request marshaling
- Purpose: Language-specific server setup (binary paths, arguments, initialization options)
- Location: `src/solidlsp/ls_config.py`
- Pattern: Per-language subclass; defines LSP server implementation and runtime dependencies
## Entry Points
- Location: `src/serena/cli.py`
- Triggers: User runs `serena` command via entry point
- Responsibilities: Parse arguments, manage command lifecycle, project discovery, tool invocation
- Core commands: activate-project, init, config, start-mcp-server
- Location: `src/serena/mcp.py`
- Triggers: AI agent connects via MCP protocol
- Responsibilities: Create SerenaMCPFactory, instantiate SerenaAgent, expose tools as MCP server
- Flow: SerenaMCPFactory.create_mcp_server() -> starts FastMCP -> waits for tool calls
- Location: `src/serena/prompt_factory.py`
- Triggers: SerenaAgent needs to format system prompt
- Responsibilities: Render Jinja2 template from mode with agent context variables
- Used by: Mode prompt generation for language model system message
## Error Handling
- **Language Server Crashes**: `LanguageServerProcess` automatically restarts on error; file buffers invalidated
- **Tool Execution Errors**: Wrapped in try/catch by MCP layer; errors logged and returned to client
- **Missing Project**: Tools raise `ValueError` if no active project; MCP handles and formats error
- **LSP Timeouts**: Configurable timeout per request; raises `LSPError` which propagates
- **Invalid Paths**: `Project.validate_relative_path()` prevents directory traversal before file operations
- **Encoding Issues**: File operations use project-specific encoding (default UTF-8); explicit handling in file tools
## Cross-Cutting Concerns
- Framework: Python `logging` module with structured logging via `sensai.util.logging`
- Patterns: Module-level loggers with level control via config; memory log handler for dashboard
- Files: `src/serena/util/logging.py` - custom handler `MemoryLogHandler` captures logs for web UI
- File paths: `Project.validate_relative_path()` checks .gitignore, ignores patterns, bounds
- Relative path enforcement: No absolute paths allowed in tool parameters
- Tool parameters: Pydantic models auto-validate via `FuncMetadata`
- LSP: Per-server configuration (environment variables for creds, API keys)
- Project: No project-level auth; assumes file system access is granted
- Secrets: Tool operations do not handle secrets; delegation to IDE/environment
- Language servers: Run in separate processes (isolated from Python asyncio)
- Tools: Execute in single executor thread (`TaskExecutor`) to maintain linear execution
- Startup: Parallel language server process spawning via threading
- Dashboard: Separate thread for web server
- Project-local: Memories, language servers, state isolated per project
- Global: Memories shared via `~/.serena/memories/global/`, contexts/modes in `~/.serena/`
- Sessions: Each SerenaAgent instance is independent; multiple sessions can coexist
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
