# Serena Usage Guide

This guide covers operational usage of Serena: tutorials for common workflows, profile and mode reference, configuration, troubleshooting, observability, and performance tuning.

For installation and feature overview, see [README.md](README.md).

## Quick Tutorials

### Tutorial 1: Onboarding a New Project

This tutorial walks through setting up Serena for a new codebase from scratch.

**Step 1: Install Serena**

```bash
go install github.com/postfix/serena/cmd/serena@latest
```

**Step 2: Register with your MCP client**

```bash
serena setup claude-code
```

This auto-detects your project, registers Serena as an MCP server, detects programming languages, pre-installs language servers, and runs a health check. Supported clients: `claude-code`, `vscode`, `jetbrains`, `claude-desktop`, `gemini-cli`, `generic`.

For HTTP mode (IDEs, web clients, multi-client):

```bash
serena --serve --http-addr=:9091
```

**Step 3: Start a session and onboard**

Once connected, the agent can use the `onboard_project` tool. This triggers Serena to:

1. Analyze the repository structure
2. Detect programming languages and locate language servers
3. Create an onboarding memory summarizing the project's architecture

Expected output: Serena produces a structured project overview including detected languages, key directories, symbol counts, and architectural notes. This is stored as a persistent memory for future sessions.

**Step 4: Session handoff**

When ending a session, use `prepare_for_new_conversation` to create a handoff summary. The next session can pick up where you left off by reading the handoff memory.

### Tutorial 2: Refactoring Workflow

This tutorial demonstrates renaming a Go function across an entire workspace using Serena's symbolic tools.

**Step 1: Find the symbol**

```
search_symbols("HandleRequest")
```

This returns all symbols matching "HandleRequest" across the workspace, with file paths and symbol kinds.

**Step 2: Understand the symbol's structure**

```
get_symbols_overview("internal/api/handler.go")
```

This shows all symbols in the file -- functions, types, methods -- giving you context for the refactoring.

**Step 3: Analyze blast radius**

```
analyze_blast_radius("HandleRequest", "internal/api/handler.go")
```

This combines `find_references` and hierarchy analysis to show every file and symbol that would be affected by a rename.

**Step 4: Rename the symbol**

```
rename_symbol("HandleRequest", "ProcessRequest", "internal/api/handler.go")
```

Serena renames the symbol across all files in the workspace. Post-edit diagnostics run automatically to verify the rename didn't break anything.

**Step 5: Verify**

```
get_diagnostics("internal/api/handler.go")
```

Check for any new errors or warnings after the refactoring.

### Tutorial 3: Code Review Workflow

This tutorial shows how to use Serena for code review, leveraging read-only tools and diagnostics.

**Step 1: Switch to review mode**

```
switch_mode("review")
```

Review mode restricts available tools to read-only operations plus diagnostics. No accidental edits.

**Step 2: Get a file overview**

```
get_symbols_overview("internal/api/handler.go")
```

Quickly understand the file's structure without reading every line.

**Step 3: Check references and impact**

```
find_references("ProcessRequest", "internal/api/handler.go")
```

See everywhere a symbol is used to assess the impact of changes under review.

**Step 4: Get type and documentation info**

```
get_hover_info("ProcessRequest", "internal/api/handler.go")
```

Retrieve type signatures, documentation comments, and other hover information.

**Step 5: Run diagnostics**

```
get_diagnostics("internal/api/handler.go")
```

Surface compiler errors, linter warnings, and other issues. Present findings with file paths, symbol names, and line numbers for actionable review feedback.

The `ci-bot` profile is purpose-built for this workflow -- it restricts tools to read-only and review operations, preventing any file modifications.

## Feature Guide

### Fuzzy Editing

Serena's `fuzzy_edit` tool matches search blocks against file content using a 4-strategy cascade that tolerates whitespace and indentation differences. The strategies, in order: **Exact** (byte-for-byte match), **Whitespace** (ignores leading/trailing whitespace per line), **IndentFlex** (tabs and spaces interchangeable), and **Failed** (returns a unified-diff showing the nearest match). If multiple matches are found at any strategy level, the tool returns an error asking you to add more context to disambiguate.

Search and replacement blocks support **ellipsis** (`...` on its own line) to skip intermediate content. Segments match in forward-only order, and search and replacement must have the same number of segments.

```
fuzzy_edit("src/handler.go", "func HandleRequest(", "...", "func HandleRequest(ctx context.Context,", "...")
```

### RepoMap and Context

Two tools provide structural code intelligence using tree-sitter tag extraction and PageRank ranking:

- **`get_repo_map`** returns a structural overview of the repository, ranking symbols by importance. Accepts a `max_tokens` parameter (default 4096, max 32768) to control output size.
- **`get_context`** takes a list of files relevant to your current task and returns ranked symbols from across the codebase using Personalized PageRank on the dependency graph. Default budget is 2048 tokens.

Both tools support 23 languages via tree-sitter grammars: Go, Python, TypeScript, TSX, Rust, Java, C, C++, C#, Ruby, PHP, JavaScript, Kotlin, Scala, Bash, Haskell, Julia, OCaml, Lua, Zig, HCL, R, and Swift.

```
get_repo_map(max_tokens=4096)
get_context(files=["src/api/handler.go", "src/models/user.go"])
```

### Setup CLI

The `serena setup <client>` command registers Serena with your MCP client in one step. It resolves the Serena binary path, writes the MCP configuration, detects programming languages in your project, pre-installs required language servers, and runs a post-setup health check.

```bash
serena setup claude-code          # Project-scoped (default)
serena setup vscode --global      # User-scoped registration
serena setup jetbrains --dry-run  # Preview without changes
```

Supported clients: `claude-code`, `vscode`, `jetbrains`, `claude-desktop`, `gemini-cli`, `generic`.

For Claude Code, setup also installs **session hooks** that automatically activate and deactivate workspaces. Three hooks are registered: `SessionStart` (activates the workspace), `PreToolUse` (nudges the agent toward Serena tools on Grep/Read/Bash), and `Stop` (deactivates the workspace). Use `--no-hooks` to skip hook installation.

```bash
serena setup claude-code --no-hooks     # Skip hook installation
serena setup claude-code --uninstall    # Remove registration and hooks
```

### Smart Errors

When a tool call contains a misspelled parameter name or incorrect enum value, Serena suggests corrections using Levenshtein distance matching. For example, passing `path` instead of `relative_path` returns a "Did you mean `relative_path`?" suggestion alongside the validation error. Exact substring matches (like `path` within `relative_path`) are prioritized for high-confidence suggestions.

This is automatic middleware behavior -- no tool call needed.

### Progressive Descriptions

Serena uses a two-tier description system to reduce token consumption. When agents enumerate tools via `tools/list`, each tool returns a brief description (under 100 tokens). Full documentation -- including parameter details, usage examples, and patterns -- is available on demand via the `get_tool_help` tool.

```
get_tool_help(tool_name="fuzzy_edit")
```

### Lazy Workspace Init

Serena transparently activates a workspace on the first `tools/call` if no workspace is currently active. The workspace path is resolved from the `repo_path` argument of the tool call or falls back to the configured default root. This eliminates the need to explicitly call `activate_project` before using tools.

If automatic activation fails, Serena returns an actionable error message suggesting you call `activate_project` explicitly with the correct path.

This is automatic middleware behavior -- no tool call needed.

### Health Monitoring

The `get_health` tool reports the status of all active language server workers and circuit breakers.

```
get_health()               # Shows only unhealthy workers (if any)
get_health(verbose=true)   # Shows all workers including healthy ones
```

When all language servers are healthy, the default (non-verbose) call returns a simple "All N language servers healthy" message. Use verbose mode to inspect individual worker states, circuit breaker status, and workspace assignments.

See [Configuration Reference](#configuration-reference) for `degradation.timeout_index` and other tuning keys.

## Profiles and Modes

### Profiles

Profiles define which skills and tools are available to an agent. Each profile is designed for a specific client type.

| Profile | Default Mode | Description | Skills |
|---------|-------------|-------------|--------|
| `claude-code` | edit | Curated for Claude Code CLI. Excludes file tools Claude already has. | symbol-retrieval, symbol-editing, diagnostics, memory, workflow |
| `codex` | edit | Curated for Codex. Excludes file tools and shell commands Codex handles natively. | symbol-retrieval, symbol-editing, diagnostics, memory, workflow |
| `ide-assistant` | read | Read-focused for IDE assistants. Excludes editing tools the IDE handles. | symbol-retrieval, diagnostics, memory |
| `ci-bot` | review | Read-only for CI/review bots. Cannot modify any files. | symbol-retrieval, diagnostics, memory |
| `full` | edit | All skills and tools enabled. No exclusions. | symbol-retrieval, symbol-editing, diagnostics, memory, workflow, file-ops |

**Profile tool details:**

**claude-code** excludes: `read_file`, `create_text_file`, `replace_content`, `execute_shell_command`, `prepare_for_new_conversation`

**codex** excludes: `read_file`, `create_text_file`, `replace_content`, `execute_shell_command`, `prepare_for_new_conversation`

**ide-assistant** excludes: `read_file`, `create_text_file`, `replace_content`, `execute_shell_command`, `prepare_for_new_conversation`, `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`

**ci-bot** excludes: `create_text_file`, `replace_content`, `execute_shell_command`, `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `delete_symbol`, `replace_lines`, `insert_at_line`, `delete_lines`

**full** excludes: nothing

Select a profile at startup:

```bash
serena --profile=claude-code
serena --profile=codex
serena --profile=ci-bot
serena --profile=full          # default
```

### Modes

Modes control what operations are permitted within a session. Modes are orthogonal to profiles -- a profile sets the base tool set, and the mode further restricts it.

| Mode | Purpose | Typical Use |
|------|---------|-------------|
| `read` | Symbol retrieval and file reading only. No edits. | Safe exploration, onboarding |
| `edit` | Full editing capabilities. Default for most profiles. | Active development |
| `review` | Read plus diagnostics and blast radius analysis. No edits. | Code review, CI analysis |
| `admin` | All tools including administrative features. Grants pprof access. | Configuration, debugging |

**Mode tool access:**

- **read**: symbol-retrieval skills + `read_file`, `search_for_pattern`, `list_dir`, `find_file`. Excludes all editing and shell tools.
- **edit**: symbol-retrieval, symbol-editing, diagnostics, memory, workflow, file-ops skills. Excludes `replace_lines`, `insert_at_line`, `delete_lines`.
- **review**: symbol-retrieval, diagnostics, memory skills + `get_blast_radius`, `search_for_pattern`, `read_file`. Excludes all editing and shell tools.
- **admin**: All skills and tools enabled including `activate_project` and `switch_mode`.

### Switching Modes

Agents can switch modes mid-session using the `switch_mode` tool:

```
switch_mode("edit")
```

Mode transitions are governed by the active profile. For example, `ci-bot` can only transition between `read` and `review` -- it cannot enter `edit` mode. Most other profiles allow transitions between `read`, `edit`, and `review`.

To check your current mode and available tools:

```
get_token_budget()
```

This returns the current profile, mode, and tool count.

## Configuration Reference

### Config Precedence

Serena uses a 4-layer configuration system. Higher layers override lower ones:

1. **CLI flags** (highest priority) -- e.g., `--profile=codex`, `--http-addr=:9091`
2. **Project config** -- `.serena/project.yml` in the project root
3. **User config** -- `~/.serena/serena_config.yml`
4. **Profile defaults** (lowest priority) -- built-in defaults

Example: if `~/.serena/serena_config.yml` sets `profile: full` but you pass `--profile=codex` on the CLI, Codex is used.

### Configuration Keys

#### Daemon Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `daemon.socket_path` | string | `/tmp/serena-$UID/daemon.sock` | Unix socket path for daemon IPC |
| `daemon.http_addr` | string | `:8080` | Listen address for Streamable HTTP transport |
| `daemon.shutdown_timeout` | int | - | Graceful shutdown timeout in seconds |

#### Logging Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `logging.format` | string | `text` | Log format: `text` or `json` |
| `logging.level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `logging.dir` | string | `~/.serena/logs/` | Log file directory |

#### Worker Pool Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `worker_pool.base_ttl` | int | `300` | Base idle timeout in seconds |
| `worker_pool.ceiling_ttl` | int | `3600` | Maximum idle timeout in seconds |
| `worker_pool.max_workers` | int | `10` | Maximum concurrent language server workers |
| `worker_pool.rss_hard_cap_mb` | int | `2048` | Per-worker RSS hard cap in MB for pressure eviction |
| `worker_pool.pressure_check_interval` | int | `10` | Interval in seconds between memory pressure checks |

#### Profile and Mode

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `profile` | string | `full` | Active agent profile name |
| `mode` | string | (profile default) | Initial operational mode override |

#### Observability Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `observability.admin_addr` | string | (disabled) | Admin listener address (e.g., `127.0.0.1:9100`) |
| `observability.enable_pprof` | bool | `false` | Enable pprof endpoints (requires admin mode) |
| `observability.tracing_endpoint` | string | (disabled) | OTLP/gRPC collector endpoint for distributed tracing |
| `observability.tracing_sample_ratio` | float | `0.0` | Tracing sample ratio (0.0 to 1.0) |
| `observability.service_name` | string | `serena` | Service name used in tracing span attributes (OTel `service.name`) |

#### Degradation Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `degradation.timeout_read` | int | `5` | Timeout in seconds for read operations (go_to_definition, find_references, etc.) |
| `degradation.timeout_search` | int | `15` | Timeout in seconds for search operations (search_symbols, search_in_files) |
| `degradation.timeout_edit` | int | `10` | Timeout in seconds for edit operations (replace_symbol_body, rename_symbol) |
| `degradation.timeout_index` | int | `120` | Timeout in seconds for initial language server workspace indexing |
| `degradation.timeout_diagnostics` | int | `20` | Timeout in seconds for diagnostics and code actions |
| `degradation.memory_limit_mb` | int | `0` | GOMEMLIMIT soft target in MB (0 = disabled) |
| `degradation.restart_budget` | int | `3` | Max consecutive language server crashes before circuit opens |

### Example Configuration

A complete `~/.serena/serena_config.yml` with common settings:

```yaml
# Profile and mode
profile: claude-code
mode: edit

# Daemon settings
daemon:
  http_addr: ":9091"
  shutdown_timeout: 30

# Logging
logging:
  format: json
  level: info
  dir: ~/.serena/logs/

# Worker pool tuning
worker_pool:
  max_workers: 8
  base_ttl: 600
  ceiling_ttl: 7200
  rss_hard_cap_mb: 1024
  pressure_check_interval: 15

# Observability (opt-in)
observability:
  admin_addr: "127.0.0.1:9100"
  enable_pprof: false
  service_name: "my-serena-instance"

# Graceful degradation
degradation:
  timeout_read: 5
  timeout_search: 15
  timeout_edit: 10
  timeout_index: 120
  timeout_diagnostics: 20
  memory_limit_mb: 2048
  restart_budget: 3
```

Project-level overrides go in `.serena/project.yml`:

```yaml
# Override profile for this specific project
profile: full

# Increase indexing timeout for large monorepos
degradation:
  timeout_index: 300

# More workers for polyglot repos
worker_pool:
  max_workers: 15
```

## Troubleshooting

### Language Server Not Starting

**Symptom:** Tools return errors like "no language server available" or "failed to start worker."

**Diagnosis:**

1. Check if the language server binary is in your PATH:
   ```bash
   which gopls          # Go
   which pyright        # Python
   which typescript-language-server  # TypeScript
   ```

2. Serena uses a three-tier resolution strategy:
   - **Tier 1: PATH lookup** -- finds existing installations
   - **Tier 2: Managed download** -- auto-installs via npm, pip, cargo, or binary download
   - **Tier 3: Helpful error** -- provides the exact install command if auto-install fails

3. Install the language server manually using the provided hint:
   ```bash
   # Go
   go install golang.org/x/tools/gopls@latest

   # Python
   npm install -g pyright

   # TypeScript
   npm install -g typescript typescript-language-server

   # Rust
   rustup component add rust-analyzer

   # Java
   # JDT Language Server -- see https://github.com/eclipse-jdtls/eclipse.jdt.ls
   ```

4. After installing, verify the binary is accessible and restart Serena.

### Cache Issues / Stale Results

**Symptom:** Symbol definitions point to old locations after refactoring, or search results don't reflect recent changes.

**Cause:** The worker pool uses share-until-dirty semantics. When you edit through Serena's tools, the cache is automatically invalidated and the worker is marked dirty. However, edits made outside Serena (e.g., directly in your editor or via git operations) are not detected.

**Fix:**

- Edit through Serena whenever possible -- cache invalidation is automatic
- If you edited outside Serena: restart the session to force fresh workers
- For large-scale external changes (e.g., `git checkout` to a different branch): restart the daemon

### Mode Restrictions

**Symptom:** Error message "tool not available in current mode" or a tool is missing from the tool list.

**Cause:** The current mode restricts the available tool set. For example, `read` mode excludes all editing tools.

**Fix:**

1. Check your current mode:
   ```
   get_token_budget()
   ```
   This reports the active profile, mode, and available tool count.

2. Switch to a mode that includes the needed tool:
   ```
   switch_mode("edit")    # For editing tools
   switch_mode("review")  # For diagnostics and blast radius
   switch_mode("admin")   # For administrative tools
   ```

3. Mode transitions are governed by your profile. The `ci-bot` profile cannot enter `edit` mode. Check the [Profiles](#profiles) section for allowed transitions.

### jdtls cold-start indexing delay

**Symptom:** Java tools (`go_to_definition`, `find_references`, `search_symbols`) return timeout errors or empty results in freshly-opened workspaces, especially in projects with many dependencies.

**Cause:** Eclipse JDT Language Server (jdtls) performs full workspace indexing on first activation, which can exceed 2 minutes in new or temporary workspaces. During indexing, symbol resolution is unavailable. The default `degradation.timeout_index` of 120 seconds may not be sufficient for large Java projects.

**Fix:**

1. Increase the indexing timeout for Java projects in `.serena/project.yml`:
   ```yaml
   degradation:
     timeout_index: 300  # 5 minutes for jdtls cold-start
   ```

2. Wait for indexing to complete before issuing symbol queries. Serena creates a workspace-specific data directory (`.jdtls-data`) to avoid cross-workspace conflicts.

3. Subsequent sessions reuse the cached index, so cold-start delay is only on first activation per workspace.

### gopls version incompatibility with Go 1.25

**Symptom:** Build failures or unexpected behavior when running Serena's benchmark suite (`test/bench/`) or when gopls returns errors after a Go version upgrade.

**Cause:** gopls v0.17.1 has a known incompatibility with Go 1.25 on linux/amd64. Additionally, the benchmark suite uses `testing.B.Loop` which requires Go 1.24 or later.

**Fix:**

1. Ensure your gopls version is compatible with your Go version. After upgrading Go, update gopls:
   ```bash
   go install golang.org/x/tools/gopls@latest
   ```

2. For benchmarks, verify you are running Go 1.24 or later:
   ```bash
   go version  # Must be 1.24+
   ```

3. After Go version changes, re-baseline benchmarks using the `capture-baseline.yml` CI workflow to avoid false regression alerts from benchstat comparisons.

### Circuit Breaker Open

**Symptom:** `ErrCircuitOpen` error from the worker pool.

**Cause:** A language server crashed repeatedly, exceeding the `restart_budget` (default: 3 consecutive crashes). The circuit breaker opens to prevent crash loops.

**Recovery:**

1. Check the Serena logs for the underlying language server crash reason:
   ```bash
   tail -f ~/.serena/logs/serena.log
   ```

2. Common causes:
   - Language server ran out of memory on a large project
   - Incompatible language server version
   - Corrupted project files that crash the parser

3. Fix the underlying issue (install newer LS version, increase memory, fix corrupted files).

4. The circuit breaker auto-recovers via a half-open probe with decorrelated jitter. After the backoff period, Serena retries the language server. If it succeeds, the circuit closes and normal operation resumes.

5. To force immediate recovery: restart the daemon.

### Memory Pressure

**Symptom:** Workers are evicted, performance degrades, or you see "pressure eviction" in logs.

**Cause:** The combined memory usage of language server workers exceeds system thresholds. Serena monitors memory via platform-aware checks (Linux cgroups, macOS `vm_stat`).

**Fix:**

1. Increase the per-worker RSS cap:
   ```yaml
   worker_pool:
     rss_hard_cap_mb: 4096  # default: 2048
   ```

2. Reduce concurrent workspaces by lowering `max_workers`:
   ```yaml
   worker_pool:
     max_workers: 5  # default: 10
   ```

3. Monitor evictions via the admin listener metrics:
   ```bash
   curl http://127.0.0.1:9100/metrics | grep serena_lspool_evictions_total
   ```

## Observability Quickstart

Serena's observability features are opt-in. Nothing is enabled by default.

### Enable the Admin Listener

The admin listener exposes health checks, Prometheus metrics, and pprof endpoints on a loopback address.

```yaml
# ~/.serena/serena_config.yml
observability:
  admin_addr: "127.0.0.1:9100"
```

Or via CLI:

```bash
serena --admin-addr=127.0.0.1:9100
```

The admin listener binds to loopback only. Bind failure is non-fatal -- the daemon continues without admin endpoints.

### Health Checks

Once the admin listener is enabled:

```bash
# Liveness -- returns 200 if the daemon process is running
curl http://127.0.0.1:9100/healthz

# Readiness -- returns 200 when the daemon is fully initialized
curl http://127.0.0.1:9100/readyz
```

### Prometheus Metrics

Scrape metrics from the admin listener:

```bash
curl http://127.0.0.1:9100/metrics
```

Key metrics to monitor:

| Metric | Type | Description |
|--------|------|-------------|
| `serena_tool_duration_seconds` | histogram | MCP tool call latency in seconds (labels: `tool_name`, `profile`, `mode`, `language`) |
| `serena_tool_calls_total` | counter | Total MCP tool calls by outcome (labels: `tool_name`, `profile`, `mode`, `language`, `outcome`) |
| `serena_lspool_workers` | gauge | Active language server workers per language (labels: `language`) |
| `serena_lspool_evictions_total` | counter | Total worker evictions (labels: `language`, `reason`). Reason values: `idle`, `pressure`, `crash`, `shutdown` |
| `serena_lspool_circuit_state` | gauge | Circuit breaker state per language (0=closed, 1=half-open, 2=open) (labels: `language`) |
| `serena_lspool_restarts_total` | counter | Language server worker restarts per language (labels: `language`) |

Example Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: serena
    static_configs:
      - targets: ["127.0.0.1:9100"]
    scrape_interval: 15s
```

**Metric labels:** The `serena_tool_duration_seconds` and `serena_tool_calls_total` metrics share four label dimensions: `tool_name`, `profile`, `mode`, and `language`. The `serena_tool_calls_total` counter adds a fifth label, `outcome`, for error tracking. Use these for targeted queries:

```promql
# Error rate per tool (last 5 minutes)
rate(serena_tool_calls_total{outcome="error"}[5m])

# p95 tool latency
histogram_quantile(0.95, rate(serena_tool_duration_seconds_bucket[5m]))

# Active workers by language
serena_lspool_workers

# Eviction rate by reason
rate(serena_lspool_evictions_total[5m])
```

### Enable Tracing

Serena supports distributed tracing via OpenTelemetry (OTLP/gRPC):

```yaml
observability:
  tracing_endpoint: "localhost:4317"   # OTLP/gRPC collector address
  tracing_sample_ratio: 0.1           # 10% sampling
```

Traces include spans for tool execution, language server communication, and worker pool operations. Connect to any OTLP-compatible backend (Jaeger, Tempo, Honeycomb, etc.).

### Enable pprof

For profiling the Serena daemon process:

```yaml
observability:
  admin_addr: "127.0.0.1:9100"
  enable_pprof: true  # requires admin mode
```

Access Go pprof endpoints at `http://127.0.0.1:9100/debug/pprof/`.

Common profiling commands:

```bash
# CPU profile (30 seconds)
go tool pprof http://127.0.0.1:9100/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://127.0.0.1:9100/debug/pprof/heap

# Goroutine dump
curl http://127.0.0.1:9100/debug/pprof/goroutine?debug=2
```

### Benchmarks

Serena ships a benchmark suite in `test/bench/` for measuring tool response times, LSP indexing throughput, observability overhead, and memory usage. Benchmarks require `gopls` installed and use `testing.B.Loop` (Go 1.24+).

**Run all benchmarks:**

```bash
go test -bench=. ./test/bench/ -timeout 300s
```

**Run a specific suite:**

```bash
go test -bench=BenchmarkTools ./test/bench/ -timeout 300s
```

**With memory allocation stats:**

```bash
go test -bench=. -benchmem ./test/bench/ -timeout 300s
```

**Comparing runs with benchstat:**

Run benchmarks multiple times for statistical significance, then compare:

```bash
# Install benchstat
go install golang.org/x/perf/cmd/benchstat@latest

# Capture before and after (5+ runs recommended)
go test -bench=. -benchmem -count=5 ./test/bench/ -timeout 300s > bench-before.txt
# ... make changes ...
go test -bench=. -benchmem -count=5 ./test/bench/ -timeout 300s > bench-after.txt

# Compare
benchstat bench-before.txt bench-after.txt
```

benchstat reports per-benchmark deltas with confidence intervals. A `~` result means no statistically significant change; `+` or `-` indicates a measurable regression or improvement.

**Tips:**

- Use `-short` to skip `BenchmarkFullRepoSmoke` (the longest-running suite)
- Use `GOMAXPROCS=4` to reduce noise on machines with many cores
- For CI benchmark gate details, see [CONTRIBUTING.md](CONTRIBUTING.md)

## Performance Tuning

### Worker Pool Sizing

The worker pool manages language server processes. Each worker is an LS process serving one or more workspaces.

| Key | Default | Description |
|-----|---------|-------------|
| `worker_pool.max_workers` | `10` | Maximum concurrent LS workers. Increase for polyglot repos. |
| `worker_pool.base_ttl` | `300` (5 min) | Base idle timeout before a clean worker is eligible for eviction. |
| `worker_pool.ceiling_ttl` | `3600` (1 hr) | Maximum idle timeout. Frequently reused workers earn higher TTLs. |
| `worker_pool.rss_hard_cap_mb` | `2048` | Per-worker RSS hard cap. Workers exceeding this are evicted. |
| `worker_pool.pressure_check_interval` | `10` | Seconds between memory pressure checks. |

**Tuning guidance:**

- For **small projects** (single language): `max_workers: 3`, `base_ttl: 600`
- For **medium polyglot** projects: `max_workers: 8`, defaults are fine
- For **large monorepos**: `max_workers: 15`, `ceiling_ttl: 7200`, increase `rss_hard_cap_mb`

### Timeout Budgets

Each tool class has a configurable timeout. If a language server doesn't respond within the budget, the operation fails gracefully with a timeout error rather than hanging.

| Tool Class | Config Key | Default | Tools |
|-----------|------------|---------|-------|
| Read | `degradation.timeout_read` | 5s | go_to_definition, find_references, get_hover_info, find_implementations, get_call_hierarchy, get_type_hierarchy |
| Search | `degradation.timeout_search` | 15s | search_symbols, search_in_files |
| Edit | `degradation.timeout_edit` | 10s | replace_symbol_body, rename_symbol, insert_before_symbol, insert_after_symbol, safe_delete_symbol |
| Index | `degradation.timeout_index` | 120s | Initial language server workspace indexing (startup cost) |
| Diagnostics | `degradation.timeout_diagnostics` | 20s | get_diagnostics, get_code_actions, format_code |

**Tuning guidance:**

- If you see frequent timeouts on `search_symbols` in large repos, increase `timeout_search` to 30s
- For monorepos with slow indexing (e.g., Java/TypeScript), increase `timeout_index` to 300s
- Keep `timeout_read` low (5-10s) -- read operations should be fast with a warm cache

### Memory Limits

```yaml
degradation:
  memory_limit_mb: 2048  # Sets GOMEMLIMIT; 0 = disabled (default)
```

When set, the Go runtime soft-targets this limit for garbage collection. This helps prevent OOM kills by encouraging more aggressive GC before memory usage becomes critical.

Workers are evicted under memory pressure before OOM occurs. The eviction order is: idle workers first, then least-recently-used, then by RSS size.

**Recommendations:**

- Set `memory_limit_mb` to ~75% of available system memory for dedicated Serena hosts
- For shared development machines, set to a conservative value (e.g., 1024-2048 MB)
- Monitor actual usage via the `/metrics` endpoint before tuning

### Restart Budget

```yaml
degradation:
  restart_budget: 3  # Max consecutive LS crashes before circuit opens (default)
```

When a language server crashes, Serena automatically restarts it. If it crashes more than `restart_budget` times consecutively, the circuit breaker opens and Serena stops attempting restarts for that language.

- Increase `restart_budget` if a language server occasionally crashes but recovers (e.g., during indexing of very large projects)
- Keep it low (2-3) for development machines to avoid crash loops consuming resources
- The circuit breaker uses decorrelated jitter backoff, so recovery attempts are spaced out even after the circuit re-closes
