# Technology Stack

**Analysis Date:** 2026-04-07

## Languages

**Primary:**
- Python 3.11+ - Core Serena agent toolkit, language server abstraction, MCP server implementation
- TypeScript/JavaScript - Vue.js dashboard frontend, language servers (node-based LSP implementations)
- Fortran - Built-in Fortran language server support (fortls dependency)
- Go, Java, Rust, C++, Kotlin, C#, PHP, Perl, Ruby, Clojure, Scala, Haskell, Elixir, Swift, Bash, Terraform, Lua, Zig, Nix, Dart, Erlang, OCaml, F#, Rego, Markdown, Julia, MATLAB, SystemVerilog, HLSL, Lean 4, Solidity, Ansible, AL - 40+ languages via Language Server Protocol adapters

**Supporting:**
- YAML - Configuration files
- Shell/Bash - Docker and deployment scripts
- SQL (SQLite) - Agno agent session persistence

## Runtime

**Environment:**
- Python 3.11 - 3.14 (requires-python >= 3.11, < 3.15)
- Node.js 22.18.0 (in Docker container for language server support)
- Docker - Container-based deployment (Python 3.11-slim base image)

**Package Manager:**
- uv 1.0+ - Fast Python package installer and resolver
- npm/yarn - For Node-based language servers

## Frameworks

**Core:**
- MCP (Model Context Protocol) 1.26.0 - Protocol layer for Claude integration via FastMCP
- FastMCP - Async server implementation for MCP protocol
- Flask 3.1.3 - Web dashboard backend (HTTP API, static file serving)
- Pydantic 2.12.5 - Data validation and settings management
- PyYAML 6.0.2, ruamel.yaml 0.18.14 - Configuration parsing

**UI/Desktop:**
- pywebview (custom git rev) - Cross-platform desktop/web UI wrapper
- PyTray 0.19.5 - System tray integration
- PIL (Pillow) - Image processing for dashboard

**Testing:**
- pytest 8.4.1 - Test runner with xdist (3.8.0) for parallelization
- syrupy 4.9.1 - Snapshot testing for symbolic editing operations
- pytest-timeout 2.4.0 - Test timeout management

**Build/Dev:**
- Sphinx 7.4.7 - Documentation generation
- sphinx_rtd_theme 2.0.0, sphinx_book_theme 1.1.4 - Documentation themes
- jupyter-book 1.0.4 - Interactive documentation
- mypy 1.17.0 - Static type checking
- ruff 0.12.5 - Fast linter and code formatter
- Poethepoet 0.36.0 - Task runner (poethepoet poe commands)

## Key Dependencies

**Critical:**
- mcp 1.26.0 - Model Context Protocol for Claude integration, core communication layer
- anthropic 0.59.0 - Anthropic Claude API client (token counting, model integration)
- requests 2.33.0 - HTTP client for project server communication and JetBrains plugin API
- sensai-utils 1.5.0 - Utility library for logging, string handling, validation

**Infrastructure:**
- pathspec 0.12.1 - Gitignore-style path matching for project scanning
- python-dotenv 1.2.1, dotenv 0.9.9 - Environment variable loading from .env files
- psutil 7.0.0 - System process monitoring (for language server lifecycle)
- filelock 3.25.2 - Cross-platform file locking for concurrent access
- joblib 1.5.1 - Job parallelization and caching
- tqdm 4.67.1 - Progress bar visualization

**Analysis & Formatting:**
- tiktoken 0.12.0 - GPT tokenizer for token estimation (fallback estimator)
- beautifulsoup4 4.14.2 - HTML parsing for dashboard generation
- docstring_parser 0.17.0 - Python docstring parsing for tool introspection
- pyright 1.1.403 - Type checking and symbol resolution
- fortls 3.2.2 - Fortran language server

**Platform Support:**
- pythonnet 3.1.0-rc0 (Windows only) - .NET runtime integration for Windows-specific language servers

**Security/Network:**
- urllib3 2.6.3 - HTTP client library (transitive, pinned for security)
- cryptography 46.0.6 - Cryptographic primitives (transitive, pinned for security)
- werkzeug 3.1.7 - WSGI utilities for Flask (transitive, pinned for CVE fixes)
- starlette 1.0.0 - ASGI web framework (transitive, for MCP server)

**Optional:**
- agno 2.5.10, sqlalchemy 2.0.41 - Agno agent framework and ORM (optional, requires extras)
- google-genai 1.27.0 - Google Generative AI API (optional, requires extras)

## Configuration

**Environment:**
- Located in: `.env` (user level) or `.env.example` (template)
- Key variables: `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`
- Project config: `.serena/project.yml` - Per-project language server and tool settings
- User config: `~/.serena/serena_config.yml` - Global preferences, dashboard settings
- Runtime config: Environment variables from `PYDEVD_DISABLE_FILE_VALIDATION`, `SERENA_HOME`, `SERENA_DOCKER`, `SERENA_PORT`, `SERENA_DASHBOARD_PORT`

**Build:**
- `pyproject.toml` - Main build and dependency configuration with:
  - Build backend: hatchling
  - Tool configs: mypy, ruff, pytest, sphinx
  - Task definitions: format, lint, test, doc-build via poethepoet
  - Optional dependency groups: `dev` (testing, docs), `agno` (agent framework), `google` (generative AI)
- `uv.lock` - Lock file for deterministic builds
- `compose.yaml` - Docker Compose for containerized deployment
- `Dockerfile` - Multi-stage production image with Python 3.11, Node.js 22, Rust toolchain

## Platform Requirements

**Development:**
- Python 3.11+
- uv package manager
- Git (for version tracking and .gitignore handling)
- Node.js (for TypeScript/JavaScript language servers)
- Rust (for rust-analyzer and other Rust-based language servers)
- .NET SDK (optional, for C# language server)
- System dependencies: build-essential, curl, ssh (for language server downloads)

**Production:**
- Docker container or Python 3.11+ environment
- Language servers downloaded on-demand to: `~/.cache/serena/language_servers/`
- Dashboard accessible on port 24282 (configurable via `SERENA_DASHBOARD_PORT`)
- MCP server accessible on configured port (default 9121)

## Language Server Architecture

Language servers run as separate subprocess instances managed by `SolidLanguageServer` (`src/solidlsp/ls.py`). Each language has:
- Language enum entry in `src/solidlsp/ls_config.py`
- Implementation class in `src/solidlsp/language_servers/[language]_*.py`
- Automatic download and caching via subprocess utilities
- LSP protocol communication over stdin/stdout

Supported language server implementations include:
- Pyright, Jedi (Python)
- Gopls (Go)
- Eclipse JDT (Java)
- Rust Analyzer (Rust)
- TypeScript Server (TypeScript/JavaScript/Vue)
- Omnisharp (C#)
- Kotlin Language Server
- Scala Language Server
- And 30+ additional language servers

---

*Stack analysis: 2026-04-07*
