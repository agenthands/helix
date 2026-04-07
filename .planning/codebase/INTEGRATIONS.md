# External Integrations

**Analysis Date:** 2026-04-07

## APIs & External Services

**LLM Providers:**
- Anthropic Claude API - Token counting and model invocation
  - SDK/Client: `anthropic` 0.59.0 (imported in `src/serena/analytics.py`)
  - Auth: `ANTHROPIC_API_KEY` environment variable
  - Purpose: Token counting via `AnthropicTokenCount` class for exact token estimation

- Google Generative AI (optional) - Alternative LLM provider
  - SDK/Client: `google-genai` 1.27.0 (optional dependency)
  - Auth: `GOOGLE_API_KEY` environment variable (from `.env.example`)
  - Purpose: Alternative model integration for agent framework

- Agno Agent Framework (optional) - Higher-level agent orchestration
  - SDK/Client: `agno` 2.5.10 (optional dependency)
  - Location: `src/serena/agno.py` - SerenaAgnoAgentProvider, SerenaAgnoToolkit
  - Purpose: Wraps Serena tools as Agno-compatible functions for UI-based agent interaction

**IDE/Editor Integration:**
- JetBrains IDE Plugin
  - Client: `src/serena/jetbrains/jetbrains_plugin_client.py` (HTTP REST calls)
  - Protocol: HTTP requests to `http://[server_address]:[port]` (local IDE plugin)
  - Purpose: Remote code editing and symbol operations through JetBrains IDEs
  - Implementation: `JetBrainsPluginClient` class using `requests` library

**Protocol Layers:**
- Model Context Protocol (MCP) 1.26.0
  - Implementation: FastMCP server in `src/serena/mcp.py`
  - Transport: SSE (Server-Sent Events) by default, configurable via `--transport` flag
  - Port: Default 9121 (configurable via `SERENA_PORT`)
  - Purpose: Exposes Serena tools to Claude and other MCP-compatible clients
  - Lifespan: Async context manager with setup/teardown in `SerenaMCPFactory`

## Data Storage

**Databases:**
- SQLite (optional, via Agno integration)
  - ORM: SQLAlchemy 2.0.41 (optional dependency)
  - Purpose: Session persistence for Agno agent conversations
  - Location: `temp/agno_agent_storage.db` (deleted between sessions by design in `src/serena/agno.py`)
  - Usage: `SqliteDb` from agno.db.sqlite (line 9 of agno.py)

**File Storage:**
- Local filesystem only
  - Project root: User-specified directory or registered project
  - Language server cache: `~/.cache/serena/language_servers/`
  - Serena config: `~/.serena/` (user) or `.serena/` (project-local)
  - Memory storage: `.serena/memories/` - Markdown files for project knowledge persistence
  - Docker workspace: `/workspace/` (mounted volumes for containerized deployment)

**Caching:**
- joblib 1.5.1 - Job-level caching and parallelization (`src/solidlsp/util/cache.py`)
- Language server protocol handler caching - Reduces LSP overhead via `SolidLSPSettings`
- Token count estimator cache - In-memory singleton pattern for `TiktokenCountEstimator` and `AnthropicTokenCount` (`src/serena/analytics.py` lines 85-116)

## Authentication & Identity

**Auth Provider:**
- Custom API key-based authentication
  - Anthropic: `ANTHROPIC_API_KEY` environment variable
  - Google: `GOOGLE_API_KEY` environment variable
  - Implementation: `dotenv` library loads from `.env` file
  - No user identity system; authentication is service-level only

**Token Management:**
- Dual token estimators:
  1. Tiktoken (free, GPT-based estimation)
  2. Anthropic API (exact count via API, rate-limited, requires API key)
  3. Character-based fallback (naive estimation)
  - Selection via `RegisteredTokenCountEstimator` enum in `src/serena/analytics.py`
  - Used for token budget tracking in tool execution

## Monitoring & Observability

**Error Tracking:**
- Custom exception handling via `src/serena/util/exception.py`
- No external error tracking service (Sentry, etc.) integrated
- Fatal exceptions display via `show_fatal_exception_safe()` function

**Logs:**
- In-memory logging via `MemoryLogHandler` from `sensai.util.logging`
- Console/file output with sensai logging framework (`src/serena/util/logging.py`)
- Log level configurable via `LOG_LEVEL` environment variable
- GUI log viewer: `src/serena/gui_log_viewer.py` - pywebview-based log display
- Dashboard analytics: Tool usage statistics and token tracking in `src/serena/analytics.py`

**Profiling:**
- pyinstrument 5.1.1 (dev dependency) - Performance profiling support

## CI/CD & Deployment

**Hosting:**
- Docker Compose - Local/self-hosted deployment via `compose.yaml`
- Docker image: `serena:latest` built from `Dockerfile` (Python 3.11-slim base)
- Container ports: 9121 (MCP) and 24282 (Dashboard)
- Environment variables: `SERENA_PORT`, `SERENA_DASHBOARD_PORT`, `SERENA_DOCKER`

**CI Pipeline:**
- GitHub Actions - Configured in `.github/workflows/docker.yml`
- No external CI service integration documented; local testing via pytest

## Environment Configuration

**Required env vars:**
- `ANTHROPIC_API_KEY` - Anthropic Claude API key (optional if using alternative tokenizer)
- `GOOGLE_API_KEY` - Google Generative AI API key (optional, only if using Google models)

**Optional env vars:**
- `LOG_LEVEL` - Logging verbosity (default: INFO)
- `SERENA_HOME` - Serena config directory (default: `~/.serena/`)
- `SERENA_DOCKER` - Flag to indicate Docker environment (set to 1 in container)
- `SERENA_PORT` - MCP server port (default: 9121)
- `SERENA_DASHBOARD_PORT` - Dashboard port (default: 24282, hex 0x5EDA)
- `FASTMCP_*` - FastMCP configuration via environment (settings prefixed with `FASTMCP_`)
- `PYDEVD_DISABLE_FILE_VALIDATION` - PyCharm debugger compatibility (set in `tool.poe.env`)

**Secrets location:**
- User: `~/.serena/serena_config.yml` - User-level sensitive config (not committed)
- Project: `.serena/project.yml` - Project-level config with language server paths
- Environment: `.env` file (excluded from git, template provided as `.env.example`)

## Webhooks & Callbacks

**Incoming:**
- Dashboard API endpoints:
  - HTTP GET `/heartbeat` - Health check
  - HTTP POST `/query_project` - Project querying via `ProjectServer` (`src/serena/project_server.py`)
  - HTTP GET `/api/` routes - REST API for dashboard operations
  - HTTP static file serving - HTML/CSS/JS dashboard assets from `src/serena/resources/dashboard/`
  - WebSocket (implied): pywebview desktop app communication

**Outgoing:**
- JetBrains IDE Plugin - HTTP requests to IDE plugin server
  - Purpose: Symbol updates, code editing commands
  - Implementation: `JetBrainsPluginClient` REST calls in `src/serena/jetbrains/jetbrains_plugin_client.py`

## Language Server Communication

**Protocol:**
- JSON-RPC 2.0 over stdin/stdout - LSP protocol via `src/solidlsp/lsp_protocol_handler/server.py`
- 40+ language servers as external processes managed by `SolidLanguageServer`
- Subprocess management via `src/solidlsp/ls_process.py` and `src/solidlsp/util/subprocess_util.py`

**Key External Language Servers:**
- pyright (Python) - Microsoft's static type checker
- gopls (Go) - Official Go language server
- eclipse-jdtls (Java) - Eclipse JDT language server
- rust-analyzer (Rust) - Official Rust language server
- typescript-language-server (TypeScript) - Node-based implementation
- And 35+ additional language servers (comprehensive list in `src/solidlsp/language_servers/`)

---

*Integration audit: 2026-04-07*
