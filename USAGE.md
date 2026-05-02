# Helix Usage Guide

This guide covers operational usage of Helix: tutorials for common workflows, profile and mode reference, configuration, troubleshooting, observability, and performance tuning.

For installation and feature overview, see [README.md](README.md). If you haven't installed Helix yet, see [INSTALL.md](INSTALL.md) first.

## Quick Tutorials

### Tutorial 1: Onboarding a New Project

This tutorial walks through setting up Helix for a new codebase from scratch.

**Step 1: Install Helix**

```bash
go install github.com/agenthands/helix/cmd/helix@latest
```

**Step 2: Register with your MCP client**

```bash
helix setup claude-code
```

This auto-detects your project, registers Helix as an MCP server, detects programming languages, pre-installs language servers, and runs a health check. Supported clients: `claude-code`, `vscode`, `jetbrains`, `claude-desktop`, `gemini-cli`, `opencode`, `generic`.

For HTTP mode (IDEs, web clients, multi-client):

```bash
helix --serve --http-addr=:9091
```

**Step 3: Start a session and onboard**

Once connected, the agent can use the `onboard_project` tool. This triggers Helix to:

1. Analyze the repository structure
2. Detect programming languages and locate language servers
3. Create an onboarding memory summarizing the project's architecture

Expected output: Helix produces a structured project overview including detected languages, key directories, symbol counts, and architectural notes. This is stored as a persistent memory for future sessions.

**Step 4: Session handoff**

When ending a session, use `prepare_for_new_conversation` to create a handoff summary. The next session can pick up where you left off by reading the handoff memory.

### Tutorial 2: Refactoring Workflow

This tutorial demonstrates renaming a Go function across an entire workspace using Helix's symbolic tools.

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

Helix renames the symbol across all files in the workspace. Post-edit diagnostics run automatically to verify the rename didn't break anything.

**Step 5: Verify**

```
get_diagnostics("internal/api/handler.go")
```

Check for any new errors or warnings after the refactoring.

### Tutorial 3: Code Review Workflow

This tutorial shows how to use Helix for code review, leveraging read-only tools and diagnostics.

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

Helix's `fuzzy_edit` tool matches search blocks against file content using a 4-strategy cascade that tolerates whitespace and indentation differences. The strategies, in order: **exact match** (byte-for-byte), **whitespace-normalized** (ignores leading/trailing whitespace per line), **indentation-flexible** (tabs and spaces interchangeable at line start), and **ellipsis-placeholder** (allows `...` in the search block to skip intermediate content). If all four strategies miss, the tool returns a unified-diff envelope showing the nearest candidate match. If multiple matches are found at any strategy level, the tool returns an error asking you to add more context to disambiguate.

Search and replacement blocks support **ellipsis** (`...` on its own line) to skip intermediate content. Segments match in forward-only order, and search and replacement must have the same number of segments.

```
fuzzy_edit(path="src/handler.go", search="func HandleRequest(\n...\n)", replacement="func HandleRequest(ctx context.Context,\n...\n)")
```

### RepoMap and Context

Two tools provide structural code intelligence using tree-sitter tag extraction and PageRank ranking:

- **`get_repo_map`** returns a structural overview of the repository, ranking symbols by importance. Accepts a `token_budget` parameter (default 4096, max 32768) to control output size.
- **`get_context`** takes a list of files relevant to your current task and returns ranked symbols from across the codebase using Personalized PageRank on the dependency graph. Default budget is 2048 tokens.

Both tools support 23 languages via tree-sitter grammars: Go, Python, TypeScript, TSX, Rust, Java, C, C++, C#, Ruby, PHP, JavaScript, Kotlin, Scala, Bash, Haskell, Julia, OCaml, Lua, Zig, HCL, R, and Swift.

```
get_repo_map(token_budget=4096)
get_context(files=["src/api/handler.go", "src/models/user.go"])
```

### Setup CLI

The `helix setup <client>` command registers Helix with your MCP client in one step. It resolves the Helix binary path, writes the MCP configuration, detects programming languages in your project, pre-installs required language servers, and runs a post-setup health check.

```bash
helix setup claude-code          # Project-scoped (default)
helix setup vscode --global      # User-scoped registration
helix setup jetbrains --dry-run  # Preview without changes
```

Supported clients: `claude-code`, `vscode`, `jetbrains`, `claude-desktop`, `gemini-cli`, `opencode`, `generic`.

For Claude Code, setup also installs **session hooks** that automatically activate and deactivate workspaces. Three hooks are registered: `SessionStart` (activates the workspace), `PreToolUse` (nudges the agent toward Helix tools on Grep/Read/Bash), and `Stop` (deactivates the workspace). Use `--no-hooks` to skip hook installation.

```bash
helix setup claude-code --no-hooks     # Skip hook installation
helix setup claude-code --uninstall    # Remove registration and hooks
```

### Smart Errors

When a tool call contains a misspelled parameter name or incorrect enum value, Helix suggests corrections using Levenshtein distance matching. For example, passing `path` instead of `relative_path` returns a "Did you mean `relative_path`?" suggestion alongside the validation error. Exact substring matches (like `path` within `relative_path`) are prioritized for high-confidence suggestions.

This is automatic middleware behavior -- no tool call needed.

### Progressive Descriptions

Helix uses a two-tier description system to reduce token consumption. When agents enumerate tools via `tools/list`, each tool returns a brief description (under 100 tokens). Full documentation -- including parameter details, usage examples, and patterns -- is available on demand via the `get_tool_help` tool.

```
get_tool_help(tool_name="fuzzy_edit")
```

### Lazy Workspace Init

Helix transparently activates a workspace on the first `tools/call` if no workspace is currently active. The workspace path is resolved from the `repo_path` argument of the tool call or falls back to the configured default root. This eliminates the need to explicitly call `activate_project` before using tools.

If automatic activation fails, Helix returns an actionable error message suggesting you call `activate_project` explicitly with the correct path.

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
helix --profile=claude-code
helix --profile=codex
helix --profile=ci-bot
helix --profile=full          # default
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

Helix uses a 4-layer configuration system. Higher layers override lower ones:

1. **CLI flags** (highest priority) -- e.g., `--profile=codex`, `--http-addr=:9091`
2. **Project config** -- `.helix/project.yml` in the project root
3. **User config** -- `~/.helix/helix_config.yml`
4. **Profile defaults** (lowest priority) -- built-in defaults

Example: if `~/.helix/helix_config.yml` sets `profile: full` but you pass `--profile=codex` on the CLI, Codex is used.

### Configuration Keys

#### Daemon Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `daemon.socket_path` | string | `/tmp/helix-$UID/daemon.sock` | Unix socket path for daemon IPC |
| `daemon.http_addr` | string | `:8080` | Listen address for Streamable HTTP transport |
| `daemon.shutdown_timeout` | int | - | Graceful shutdown timeout in seconds |

#### Logging Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `logging.format` | string | `text` | Log format: `text` or `json` |
| `logging.level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `logging.dir` | string | `~/.helix/logs/` | Log file directory |

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
| `observability.service_name` | string | `helix` | Service name used in tracing span attributes (OTel `service.name`) |

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

A complete `~/.helix/helix_config.yml` with common settings:

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
  dir: ~/.helix/logs/

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
  service_name: "my-helix-instance"

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

Project-level overrides go in `.helix/project.yml`:

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

2. Helix uses a three-tier resolution strategy:
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

4. After installing, verify the binary is accessible and restart Helix.

### Cache Issues / Stale Results

**Symptom:** Symbol definitions point to old locations after refactoring, or search results don't reflect recent changes.

**Cause:** The worker pool uses share-until-dirty semantics. When you edit through Helix's tools, the cache is automatically invalidated and the worker is marked dirty. However, edits made outside Helix (e.g., directly in your editor or via git operations) are not detected.

**Fix:**

- Edit through Helix whenever possible -- cache invalidation is automatic
- If you edited outside Helix: restart the session to force fresh workers
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

1. Increase the indexing timeout for Java projects in `.helix/project.yml`:
   ```yaml
   degradation:
     timeout_index: 300  # 5 minutes for jdtls cold-start
   ```

2. Wait for indexing to complete before issuing symbol queries. Helix creates a workspace-specific data directory (`.jdtls-data`) to avoid cross-workspace conflicts.

3. Subsequent sessions reuse the cached index, so cold-start delay is only on first activation per workspace.

### gopls version compatibility

**Symptom:** Build failures or unexpected behavior from the Go integration tests after upgrading Go.

**Cause:** Older gopls releases (notably v0.17.1) predate Go 1.25 and exhibit incompatibilities on linux/amd64. The fix shipped upstream in gopls v0.21 and later.

**Fix:** Update gopls after every Go upgrade:

```bash
go install golang.org/x/tools/gopls@latest
```

Helix does not pin a gopls version — `internal/langregistry` invokes whatever `gopls` is on `$PATH`. Use `>=v0.21` as a floor, not as a pin.

### `rename_symbol` on Rust symbols

`rename_symbol` works against Rust symbols via rust-analyzer using a hybrid
strategy. The tool response includes a `strategy:` line indicating which path
produced the rename:

- `strategy: lsp-native` — rust-analyzer's `textDocument/rename` succeeded.
  Fully semantically accurate (cross-crate trait-impls, macro expansion,
  re-exports).
- `strategy: rust-client-side` — a references-driven fallback was used
  because rust-analyzer's native rename returned
  `"No references found at position"` for the temp workspace. The fallback is
  deterministic for the common case (single identifier, direct references),
  but does NOT match native rust-analyzer rename on:
  - cross-crate trait-impl method renames,
  - macro-expansion call sites,
  - re-exports that rename at the re-export site.

  If you need cross-crate or macro-accurate rename, open the project in a
  persistent workspace (so rust-analyzer's crate graph is fully loaded) or
  track upstream-fix work under `BUG-DEFER-02`.

If both paths fail, `rename_symbol` returns an `unsupported` error pointing
you at `fuzzy_edit`, `replace_symbol_body`, or `search_in_files` for a manual
rename.

### Circuit Breaker Open

**Symptom:** `ErrCircuitOpen` error from the worker pool.

**Cause:** A language server crashed repeatedly, exceeding the `restart_budget` (default: 3 consecutive crashes). The circuit breaker opens to prevent crash loops.

**Recovery:**

1. Check the Helix logs for the underlying language server crash reason:
   ```bash
   tail -f ~/.helix/logs/helix.log
   ```

2. Common causes:
   - Language server ran out of memory on a large project
   - Incompatible language server version
   - Corrupted project files that crash the parser

3. Fix the underlying issue (install newer LS version, increase memory, fix corrupted files).

4. The circuit breaker auto-recovers via a half-open probe with decorrelated jitter. After the backoff period, Helix retries the language server. If it succeeds, the circuit closes and normal operation resumes.

5. To force immediate recovery: restart the daemon.

### Memory Pressure

**Symptom:** Workers are evicted, performance degrades, or you see "pressure eviction" in logs.

**Cause:** The combined memory usage of language server workers exceeds system thresholds. Helix monitors memory via platform-aware checks (Linux cgroups, macOS `vm_stat`).

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
   curl http://127.0.0.1:9100/metrics | grep helix_lspool_evictions_total
   ```

## Observability Quickstart

Helix's observability features are opt-in. Nothing is enabled by default.

### Enable the Admin Listener

The admin listener exposes health checks, Prometheus metrics, and pprof endpoints on a loopback address.

```yaml
# ~/.helix/helix_config.yml
observability:
  admin_addr: "127.0.0.1:9100"
```

Or via CLI:

```bash
helix --admin-addr=127.0.0.1:9100
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

### Grafana Dashboards

![Helix overview dashboard](docs/images/helix-overview-dashboard.png)

Two dashboards live in `deploy/grafana/`:

- [`helix-overview.json`](deploy/grafana/helix-overview.json) — RED metrics + workspace activity. Primary operator view.
- [`helix-engine.json`](deploy/grafana/helix-engine.json) — lspool, repomap, edit-tool internals. On-call deep-dive.

In Grafana, **Dashboards → New → Import**, upload the JSON, and select your
Prometheus datasource when prompted for `DS_PROMETHEUS`.

The dashboards expose `$language` and `$instance` template variables for
drill-down — leave both at `All` for a global view.

### Runbooks

Operational runbooks for the four most common Helix failure modes live in `docs/runbooks/`:

- [`ErrCircuitOpen.md`](docs/runbooks/ErrCircuitOpen.md) — circuit breaker tripped on a language pool.
- [`deadline-timeouts.md`](docs/runbooks/deadline-timeouts.md) — tool calls hitting the configured deadline.
- [`ls-crash-restart.md`](docs/runbooks/ls-crash-restart.md) — language server process crashing.
- [`memory-pressure-eviction.md`](docs/runbooks/memory-pressure-eviction.md) — workers evicted under memory pressure.

Each runbook lists symptoms, triage PromQL, likely causes, and remediation steps.

### Prometheus Metrics

Scrape metrics from the admin listener:

```bash
curl http://127.0.0.1:9100/metrics
```

Key metrics to monitor:

| Metric | Type | Description |
|--------|------|-------------|
| `helix_tool_duration_seconds` | histogram | MCP tool call latency in seconds (labels: `tool_name`, `profile`, `mode`, `language`) |
| `helix_tool_calls_total` | counter | Total MCP tool calls by outcome (labels: `tool_name`, `profile`, `mode`, `language`, `outcome`) |
| `helix_lspool_workers` | gauge | Active language server workers per language (labels: `language`) |
| `helix_lspool_evictions_total` | counter | Total worker evictions (labels: `language`, `reason`). Reason values: `idle`, `pressure`, `crash`, `shutdown` |
| `helix_lspool_circuit_state` | gauge | Circuit breaker state per language (0=closed, 1=half-open, 2=open) (labels: `language`) |
| `helix_lspool_restarts_total` | counter | Language server worker restarts per language (labels: `language`) |
| `helix_lspool_lookups_total` | counter | LSP worker-pool cache lookups by result (labels: `language`, `result`). `result` ∈ `{hit, miss}` -- hit = shared warm worker via `workerForKeyLocked`; miss = spawn (or refusal due to circuit-open / max-workers). Dirty acquires count as misses (Phase 53 D-02). |
| `helix_repomap_lookups_total` | counter | RepoMap TagCache lookups by result (labels: `language`, `result`). `result` ∈ `{hit, miss}` -- hit = mtime match; miss = extractor invoked (Phase 53 D-03). |
| `helix_repomap_extract_duration_seconds` | histogram | RepoMap extractor latency, cache-miss path only (labels: `language`, `extractor`). `extractor` ∈ `{treesitter, lsp, fallback}`. Custom buckets `{1ms, 2.5ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s}` give p50 visibility for tree-sitter and tail visibility for LSP fallback (Phase 53 D-05/D-06). |
| `helix_session_lifecycle_total` | counter | MCP session lifecycle phase transitions by transport (labels: `phase`, `transport`). `phase` ∈ `{started, ended, error}`; `transport` ∈ `{stdio, http}`. **Note:** for `transport=http`, `phase=ended` is best-effort -- emitted on `DELETE /mcp` only; sessions that disappear due to server-side timeout do not register (the MCP SDK v1.5.0 does not expose a per-session lifecycle hook). For `transport=stdio`, all three phases are reliably emitted from the forwarder stream lifecycle (Phase 53 D-08/D-09). |
| `helix_edit_outcome_total` | counter | Edit-tool handler outcomes (labels: `tool_name`, `outcome`, `strategy`). `tool_name` ∈ the 7 production edit/fileops tools (`replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`, `replace_in_file`, `fuzzy_edit`); `outcome` ∈ `{success, no_match, ambiguous_match, validation_failed, ls_error, internal}`; `strategy` ∈ `{exact, whitespace_normalized, indentation_flexible, none}`. `none` is emitted by non-fuzzy tools and by failure paths where no fuzzy strategy ran. The existing `helix_rename_strategy_total` (Phase 47) tracks an orthogonal LSP-native vs. client-side dispatch dimension and continues to coexist (Phase 53 D-10/D-11/D-12). |

Example Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: helix
    static_configs:
      - targets: ["127.0.0.1:9100"]
    scrape_interval: 15s
```

**Metric labels:** The `helix_tool_duration_seconds` and `helix_tool_calls_total` metrics share four label dimensions: `tool_name`, `profile`, `mode`, and `language`. The `helix_tool_calls_total` counter adds a fifth label, `outcome`, for error tracking. Use these for targeted queries:

```promql
# Error rate per tool (last 5 minutes). The `outcome` label is a closed
# enum {success, invalid_args, not_found, circuit_open, ls_crash, timeout,
# internal}; in v1.2 the practical failure set is {circuit_open, timeout,
# internal} (invalid_args, not_found, ls_crash are reserved for v1.3
# typed-error work). Union the failure outcomes:
rate(helix_tool_calls_total{outcome=~"timeout|circuit_open|internal"}[5m])
  / rate(helix_tool_calls_total[5m])

# Equivalent inverse — non-success rate, robust to enum additions:
1 - rate(helix_tool_calls_total{outcome="success"}[5m])
  / rate(helix_tool_calls_total[5m])

# p95 tool latency
histogram_quantile(0.95, rate(helix_tool_duration_seconds_bucket[5m]))

# Active workers by language
helix_lspool_workers

# Eviction rate by reason
rate(helix_lspool_evictions_total[5m])
```

```promql
# Phase 53 D-01: lspool cache hit-ratio (5-minute window).
# Closer to 1.0 = more sessions reuse warm workers; closer to 0 = more spawns.
sum(rate(helix_lspool_lookups_total{result="hit"}[5m]))
  / sum(rate(helix_lspool_lookups_total[5m]))
```

```promql
# Phase 53 D-05/D-06: per-extractor p95 RepoMap extraction latency.
# Surfaces "is the LSP fallback dragging us down for $LANG?" in one query.
histogram_quantile(0.95, sum by (le, extractor) (rate(helix_repomap_extract_duration_seconds_bucket[5m])))
```

> **HTTP session `ended` is best-effort.** `helix_session_lifecycle_total{transport="http", phase="ended"}` only fires on a client-issued `DELETE /mcp` (the MCP-spec clean termination signal). Sessions that disappear because the SDK times them out internally are NOT counted as `ended`. For reliable session tracking on HTTP, use `started` minus `error` and treat the gap as outstanding sessions plus quietly-timed-out ones. For stdio transport (the default), all three phases are reliably emitted.

### Enable Tracing

Helix supports distributed tracing via OpenTelemetry (OTLP/gRPC):

```yaml
observability:
  tracing_endpoint: "localhost:4317"   # OTLP/gRPC collector address
  tracing_sample_ratio: 0.1           # 10% sampling
```

Traces include spans for tool execution, language server communication, and worker pool operations. Connect to any OTLP-compatible backend (Jaeger, Tempo, Honeycomb, etc.).

### Trace Sampling

Helix uses head-based sampling — the decision to record a trace is made at the
root span and propagated to every child via the W3C tracecontext spec. The
sampler is `ParentBased(TraceIDRatioBased(tracing_sample_ratio))` (see
`internal/obs/tracing.go`).

**Configuration:**

```yaml
observability:
  tracing_endpoint: "localhost:4317"   # OTLP/gRPC collector; empty = tracing OFF
  tracing_sample_ratio: 0.1            # 0.0 = none, 1.0 = all, 0.1 = 10%
  service_name: "helix"                # populates resource.service.name
```

**Behavior:**

| `tracing_endpoint` | `tracing_sample_ratio` | Result |
|--------------------|-----------------------|--------|
| empty / unset      | (any)                 | Tracing fully OFF — noop tracer, zero allocations on hot path. |
| set                | `0.0`                 | Tracer wired, ratio sampler drops every trace. Useful for verifying the export path without volume. |
| set                | `0.1`                 | 10% of root traces are sampled; child spans inherit the parent decision. |
| set                | `1.0`                 | Every trace is sampled; recommended only for debugging or smoke runs. |

**Operational guidance:**

- Production: start at `0.01` (1%); raise during incident response.
- Development: `1.0` to capture every trace.
- Smoke tests: see `docs/runbooks/trace-smoke.md` for a one-command Jaeger smoke setup using `1.0`.

**What is NOT shipped today:**

- **Tail-sampling** (deciding to keep/drop traces *after* all spans complete, e.g.
  "always keep traces with errors"). Helix uses head-only sampling. Tail-sampling
  is on the future-scope backlog; if you need it today, run a tail-aware OTLP
  collector (Grafana Agent, OpenTelemetry Collector with `tail_sampling`
  processor) downstream of Helix.

### Enable pprof

For profiling the Helix daemon process:

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

Helix ships a benchmark suite in `test/bench/` for measuring tool response times, LSP indexing throughput, observability overhead, and memory usage. Benchmarks require `gopls` installed and use `testing.B.Loop` (Go 1.24+).

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

- Set `memory_limit_mb` to ~75% of available system memory for dedicated Helix hosts
- For shared development machines, set to a conservative value (e.g., 1024-2048 MB)
- Monitor actual usage via the `/metrics` endpoint before tuning

### Restart Budget

```yaml
degradation:
  restart_budget: 3  # Max consecutive LS crashes before circuit opens (default)
```

When a language server crashes, Helix automatically restarts it. If it crashes more than `restart_budget` times consecutively, the circuit breaker opens and Helix stops attempting restarts for that language.

- Increase `restart_budget` if a language server occasionally crashes but recovers (e.g., during indexing of very large projects)
- Keep it low (2-3) for development machines to avoid crash loops consuming resources
- The circuit breaker uses decorrelated jitter backoff, so recovery attempts are spaced out even after the circuit re-closes

## Development

### Java integration tests (warm jdtls cache)

Helix's Java integration suite (`test/integration/java_test.go`) runs as part of
the default `go test ./...`. First-time runs are slow because jdtls must index the
Java fixture; subsequent runs reuse a persistent workspace under the user cache
directory, cutting wall-clock dramatically.

**Cache location:**

- Linux: `$XDG_CACHE_HOME/helix-test/jdtls/<fixture-hash>-<jdtls-hash>/` (default `~/.cache/...`).
- macOS: `~/Library/Caches/helix-test/jdtls/...`.
- Windows: `%LocalAppData%\helix-test\jdtls\...`.

The cache key is derived from the Java fixture contents (`testdata/fixtures/java/**`)
and the resolved `jdtls` binary identity. Changing either produces a new directory;
old ones coexist until manually cleaned.

**Commands:**

- `go test ./test/integration/ -run 'TestSymbols_JavaFixture|TestEdit_JavaFixture' -count=1` — run the Java suite once. Skips cleanly if `jdtls` is not on PATH.
- `make bench-jdtls-warm` — wipe the cache, run the Java suite cold, then run it warm; prints both wall-clocks.
- `make clean-jdtls-cache` — remove `$XDG_CACHE_HOME/helix-test/jdtls/` (POSIX only; on Windows delete the folder manually).

**Test-only environment variable:**

`HELIX_TEST_JDTLS_DATA_DIR` — when set to a non-empty absolute path, Helix's
jdtls adapter uses that path as the `-data` argument instead of the default
`workDir/.jdtls-data`. This is the seam the Java integration tests use to inject
the warm cache directory. **Do not set this variable in production.**

**Known limitations:**

- Running two `go test ./...` invocations concurrently against the same fixture
  will race on the warm `-data` directory — jdtls requires an exclusive workspace
  lock. Mitigation: avoid concurrent integration-test runs on the same machine,
  or run `make clean-jdtls-cache` between them. A file-system advisory lock is
  tracked as a follow-up (see RESEARCH.md Pitfall 4; deferred beyond Phase 48).
- CI reuses the same cache via `actions/cache@v3` in `.github/workflows/go-test.yml`;
  cache misses (new fixture content or new jdtls version) fall back to cold start.
