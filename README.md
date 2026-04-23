<p align="center" style="text-align:center;">
  <img src="resources/serena-logo.svg#gh-light-mode-only" style="width:500px">
  <img src="resources/serena-logo-dark-mode.svg#gh-dark-mode-only" style="width:500px">
</p>

<h3 align="center">
    Serena is the IDE for your coding agent.
</h3>

<p align="center">Code intelligence platform for MCP &mdash; 41+ tools across 52 languages.</p>

* Serena provides essential **semantic code retrieval, editing and refactoring tools** that are akin to an IDE's capabilities,
  operating at the symbol level and exploiting relational structure.
* It integrates with any client/LLM via the model context protocol (**MCP**).
* Ships as a **single Go binary** — no Python, no Docker, no runtime dependencies beyond the binary itself.

Serena's **agent-first tool design** involves robust high-level abstractions, distinguishing it from
approaches that rely on low-level concepts like line numbers or primitive search patterns.

Practically, this means that your agent operates **faster, more efficiently and more reliably**, especially in larger and
more complex codebases.

## Table of Contents

- [How Serena Works](#how-serena-works)
- [Key Advantages](#key-advantages-over-file-based-approaches)
- [Quick Start](#quick-start)
- [Key Features](#key-features)
- [RepoMap](#repomap)
- [Production & Observability](#production--observability)
- [Languages](#programming-language-support)
- [Tools](#features)
- [Architecture](#architecture)
- [Acknowledgements](#acknowledgements)

## How Serena Works

Serena provides 41+ MCP tools for coding workflows, backed by real language servers.
An LLM orchestrates these tools to navigate, understand, and edit code.

Serena runs as a **persistent daemon** that keeps language servers warm between sessions.
Agents connect via the **model context protocol (MCP)** through:
* **stdio** — direct integration with Claude Code, Codex, OpenCode, Gemini-CLI
* **Streamable HTTP** — for IDEs, web clients, and multi-client scenarios

<img src="resources/serena-block-diagram.svg">

## Key Advantages Over File-Based Approaches

| | File-based tools | Serena |
|---|---|---|
| **Navigation** | grep, find, read whole files | Go to definition, find references, symbol search, call hierarchy |
| **Editing** | Line-number replacements, regex | Replace symbol body, insert before/after, rename across files |
| **Safety** | Hope the line numbers are right | Reference checking before delete, post-edit diagnostic verification |
| **Performance** | Re-read files every session | Warm language server cache, shared across sessions |
| **Scale** | Degrades in large codebases | Symbol-level operations stay fast regardless of codebase size |

## Quick Start

### Install

```bash
go install github.com/postfix/serena/cmd/serena@latest
```

Or build from source:

```bash
git clone https://github.com/postfix/serena.git
cd serena
go build ./cmd/serena
```

### Configure Your Client

```bash
serena setup claude-code    # Claude Code
serena setup vscode         # VS Code
serena setup jetbrains      # JetBrains IDEs
serena setup gemini-cli     # Gemini CLI
serena setup claude-desktop # Claude Desktop
serena setup opencode       # OpenCode
serena setup generic        # Generic MCP client
```

Add `--global` for user-wide registration. Run `serena setup --help` for all options.

Serena uses **lazy initialization** — workspaces are configured on first tool call, so there is no upfront indexing delay.

<details>
<summary>Manual configuration (without serena setup)</summary>

**Claude Code** (`.claude/settings.json`):
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```

**Codex** (`.codex/config.json`):
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio", "--profile=codex"]
    }
  }
}
```

**IDE Assistant** (generic MCP client config):
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio", "--profile=ide-assistant"]
    }
  }
}
```

**HTTP mode** (for IDEs, web clients, multi-client):
```bash
serena --serve --http-addr=:9091
# Connect your client to http://localhost:9091/mcp
```

For detailed configuration options, see [INSTALL.md](INSTALL.md).

</details>

### Select a Profile

```bash
serena --profile=claude-code    # Curated for Claude Code (excludes file tools it already has)
serena --profile=codex          # Curated for Codex
serena --profile=ide-assistant  # Read-focused for IDE assistants
serena --profile=ci-bot         # Read-only for CI/review bots
serena --profile=full           # All tools (default)
```

For full profile and mode reference, see [USAGE.md](USAGE.md).

## Key Features

| Feature | Description |
|---------|-------------|
| **Fuzzy Editing** | 4-strategy cascade (exact, whitespace-normalized, indentation-flexible, ellipsis-placeholder) handles imprecise LLM-generated edits |
| **Smart Errors** | "Did you mean?" suggestions via Levenshtein distance on misspelled parameter names and enum values |
| **Progressive Descriptions** | Tools expose short descriptions for listing; full documentation available on demand via `get_tool_help` |
| **Health Monitoring** | `get_health` reports runtime status of language servers and worker pool |
| **Lazy Initialization** | Workspaces initialize on first tool call — no upfront indexing delay |
| **Setup CLI** | One-command client registration: `serena setup claude-code` with language detection and health check |

## RepoMap

Serena includes a structural code intelligence engine for understanding repository layout and finding task-relevant context:

- **`get_repo_map`** — Generates a structural overview of the repository using tree-sitter tag extraction and PageRank ranking. Token-budget-aware output scales to any repository size.
- **`get_context`** — Given a set of files relevant to your task, returns ranked symbols and definitions across the codebase that are most relevant, using dependency graph analysis.

## Production & Observability

Serena ships with built-in production infrastructure — metrics, tracing, health checks, and graceful degradation — so you can deploy it as a long-running service with confidence.

### Metrics & Monitoring

- Prometheus `/metrics` endpoint with RED histograms (rate, errors, duration) per tool
- lspool health gauges: active workers, evictions, restarts, circuit state
- Bounded-label contract (allowlist: `tool_name`, `profile`, `mode`, `language`, `outcome`) enforced by CI lint

### Distributed Tracing

- End-to-end trace propagation: forwarder -> daemon -> kernel -> language server via `otelgrpc`
- Per-tool sub-spans in telemetry middleware
- Optional OTLP/gRPC exporter — off by default (`ParentBased(TraceIDRatioBased(0.0))`)

### Admin Endpoints

A dedicated loopback admin listener, isolated from MCP traffic, exposes:

| Endpoint | Purpose |
|----------|---------|
| `/healthz` | Daemon liveness |
| `/readyz` | Readiness (language servers initialized) |
| `/metrics` | Prometheus scrape target |
| `/debug/pprof/*` | Go profiling, gated behind admin profile scope |

### Graceful Degradation

- Per-class timeout budgets: read 5s, search 15s, edit 10s, index 120s, diagnostics 20s
- Deadline propagation from forwarder through daemon and kernel to language server
- Circuit breaker with decorrelated jitter backoff and single-probe half-open
- LS crash recovery with configurable restart budget
- `GOMEMLIMIT` support via `runtime/debug.SetMemoryLimit` for memory-constrained environments
- Clean shutdown: SIGTERM drains in-flight calls, flushes telemetry within 5s

## Programming Language Support

Serena supports **52 programming languages** via Language Server Protocol (LSP):

<!-- BEGIN LANGUAGES -->
| Language | LS Command | File Extensions | Install |
|----------|------------|-----------------|--------|
| Al | `al-language-server` | .al | Install "al-language-server" manually and ensure it is in your PATH. |
| Ansible | `ansible-language-server` | .yml, .yaml | npm install -g @ansible/ansible-language-server |
| Bash | `bash-language-server` | .sh, .bash, .zsh | npm install -g bash-language-server |
| Clojure | `clojure-lsp` | .clj, .cljs, .cljc, .edn | Download clojure-lsp from the project's release page and add it to your PATH. |
| C/C++ | `clangd` | .c, .cpp, .cc, .cxx, .h, .hpp, .hxx | Download clangd from the project's release page and add it to your PATH. |
| C/C++ (ccls) | `ccls` | .c, .cpp, .cc, .cxx, .h, .hpp, .hxx | Install "ccls" manually and ensure it is in your PATH. |
| C# | `csharp-ls` | .cs | dotnet tool install -g csharp-ls |
| Csharp (Omnisharp) | `OmniSharp` | .cs | Download OmniSharp from the project's release page and add it to your PATH. |
| Dart | `dart` | .dart | Install "dart" manually and ensure it is in your PATH. |
| Elixir | `elixir-ls` | .ex, .exs | Download elixir-ls from the project's release page and add it to your PATH. |
| Elm | `elm-language-server` | .elm | npm install -g @elm-tooling/elm-language-server |
| Erlang | `erlang_ls` | .erl, .hrl | Install "erlang_ls" manually and ensure it is in your PATH. |
| Fortran | `fortls` | .f90, .f95, .f03, .f08, .f, .for | pip install fortls |
| F# | `fsautocomplete` | .fs, .fsi, .fsx | dotnet tool install -g fsautocomplete |
| Go | `gopls` | .go | Install "gopls" manually and ensure it is in your PATH. |
| Groovy | `groovy-language-server` | .groovy, .gradle | Download groovy-language-server from the project's release page and add it to your PATH. |
| Haskell | `haskell-language-server-wrapper` | .hs, .lhs | Install "haskell-language-server-wrapper" manually and ensure it is in your PATH. |
| Hlsl | `shader_language_server` | .hlsl, .hlsli, .fx | cargo install shader_language_server |
| Java | `jdtls` | .java | Download eclipse.jdt.ls from the project's release page and add it to your PATH. |
| Julia | `julia` | .jl | Install "julia" manually and ensure it is in your PATH. |
| Kotlin | `kotlin-language-server` | .kt, .kts | Download kotlin-language-server from the project's release page and add it to your PATH. |
| Lean4 | `lean` | .lean | Install "lean" manually and ensure it is in your PATH. |
| Lua | `lua-language-server` | .lua | Download lua-language-server from the project's release page and add it to your PATH. |
| Luau | `luau-lsp` | .luau | Install "luau-lsp" manually and ensure it is in your PATH. |
| Markdown | `marksman` | .md, .markdown | Download marksman from the project's release page and add it to your PATH. |
| Matlab | `matlab-language-server` | .m | Install "matlab-language-server" manually and ensure it is in your PATH. |
| Nix | `nixd` | .nix | Install "nixd" manually and ensure it is in your PATH. |
| Ocaml | `ocamllsp` | .ml, .mli | Install "ocamllsp" manually and ensure it is in your PATH. |
| Pascal | `pasls` | .pas, .pp, .lpr | Install "pasls" manually and ensure it is in your PATH. |
| Perl | `perl` | .pl, .pm | Install "perl" manually and ensure it is in your PATH. |
| Php | `intelephense` | .php | npm install -g intelephense |
| Php (Phpactor) | `phpactor` | .php | Install "phpactor" manually and ensure it is in your PATH. |
| Powershell | `pwsh` | .ps1, .psm1, .psd1 | Download PowerShellEditorServices from the project's release page and add it to your PATH. |
| Python | `pyright-langserver` | .py, .pyi | pip install pyright |
| Python (Jedi) | `jedi-language-server` | .py, .pyi | pip install jedi-language-server |
| Python (Ty) | `ty` | .py, .pyi | pip install ty |
| R | `R` | .r, .R, .rmd | Install "R" manually and ensure it is in your PATH. |
| Rego | `regal` | .rego | Download regal from the project's release page and add it to your PATH. |
| Ruby | `ruby-lsp` | .rb, .rake, .gemspec | gem install ruby-lsp |
| Ruby (Solargraph) | `solargraph` | .rb, .rake, .gemspec | gem install solargraph |
| Rust | `rust-analyzer` | .rs | Install "rust-analyzer" manually and ensure it is in your PATH. |
| Scala | `metals` | .scala, .sc, .sbt | Install "metals" manually and ensure it is in your PATH. |
| Solidity | `solidity-ls` | .sol | npm install -g @nomicfoundation/solidity-language-server |
| Swift | `sourcekit-lsp` | .swift | Install "sourcekit-lsp" manually and ensure it is in your PATH. |
| Systemverilog | `verible-verilog-ls` | .sv, .svh, .v | Install "verible-verilog-ls" manually and ensure it is in your PATH. |
| Terraform | `terraform-ls` | .tf, .tfvars | Download terraform-ls from the project's release page and add it to your PATH. |
| Toml | `taplo` | .toml | cargo install taplo-cli |
| Typescript | `typescript-language-server` | .ts, .tsx, .mts, .cts | npm install -g typescript-language-server@5.1.3 |
| Typescript (Vts) | `vue-language-server` | .ts, .tsx, .js, .jsx | npm install -g @vue/language-server |
| Vue | `vue-language-server` | .vue | npm install -g @vue/language-server |
| Yaml | `yaml-language-server` | .yaml, .yml | npm install -g yaml-language-server |
| Zig | `zls` | .zig | Install "zls" manually and ensure it is in your PATH. |

<!-- END LANGUAGES -->

Language servers are **auto-discovered** from PATH or **downloaded on demand** via the three-tier installer (PATH lookup > managed download > helpful error message).

## Features

<!-- BEGIN TOOLS -->
| Tool | Category | Description |
|------|----------|-------------|
| `get_diagnostics` | diagnostics | Returns current diagnostics (errors, warnings) for a file |
| `get_code_actions` | diagnostics | Returns available code actions/quick fixes for a position or range |
| `format_code` | diagnostics | Formats a file via the language server and writes the result |
| `read_file` | file-ops | Read a file's content, optionally a specific line range |
| `create_file` | file-ops | Create a new file with content (errors if file already exists) |
| `list_directory` | file-ops | List directory contents with file type, size, and modification time |
| `find_files` | file-ops | Find files matching a glob pattern (supports ** for recursive matching) |
| `search_in_files` | file-ops | Search for a regex pattern across the codebase, with optional context lines |
| `replace_in_file` | file-ops | Replace all occurrences of a pattern in a file (literal or regex) |
| `fuzzy_edit` | file-ops | Fuzzy-match and replace text in a file using 4-strategy cascade |
| `get_health` | health | Get workspace health status and language server states |
| `get_tool_help` | help | Get comprehensive documentation for any MCP tool including parameters, types, and usage examples |
| `write_memory` | memory | Write information about this project that can be useful for future tasks to a memory in md format. |
| `read_memory` | memory | Read the content of a memory file. |
| `list_memories` | memory | List available memories, optionally filtered by scope ("project" or "global") and topic. |
| `search_memories` | memory | Search memories by full-text query, optionally filtered by scope ("project" or "global"). |
| `rename_memory` | memory | Rename or move a memory. |
| `edit_memory` | memory | Edit a memory by replacing a search string with a replacement string. |
| `delete_memory` | memory | Delete a memory file. |
| `switch_mode` | profile | Switch the current session's operational mode (read/edit/review/admin) |
| `get_token_budget` | profile | Get token budget breakdown for the current or specified profile/mode |
| `get_repo_map` | repomap | Get a ranked structural overview of the repository. |
| `get_context` | repomap | Get the most relevant code context for a set of files or a task. |
| `replace_symbol_body` | symbol-editing | Replace a symbol's body with new content using tree-sitter for precise extraction |
| `insert_before_symbol` | symbol-editing | Insert content immediately before a symbol |
| `insert_after_symbol` | symbol-editing | Insert content immediately after a symbol |
| `rename_symbol` | symbol-editing | Rename a symbol across all files in the workspace |
| `safe_delete_symbol` | symbol-editing | Delete a symbol if it has no references; reports reference count if blocked |
| `verify_edit` | symbol-editing | Check for compilation errors after an edit; returns diagnostic summary |
| `go_to_definition` | symbol-retrieval | Go to the definition of a symbol at a given position |
| `find_references` | symbol-retrieval | Find all references to a symbol at a given position |
| `get_symbol_overview` | symbol-retrieval | Get a hierarchical outline of all symbols in a file |
| `search_symbols` | symbol-retrieval | Search for symbols across the workspace by name |
| `get_hover_info` | symbol-retrieval | Get hover/type information for a symbol at a given position |
| `find_implementations` | symbol-retrieval | Find all implementations of an interface or abstract method |
| `get_call_hierarchy` | symbol-retrieval | Get call hierarchy (callers and/or callees) for a symbol |
| `get_type_hierarchy` | symbol-retrieval | Get type hierarchy (subtypes and/or supertypes) for a symbol |
| `analyze_blast_radius` | symbol-retrieval | Analyze the blast radius (impact) of changing a symbol |
| `onboard_project` | workflow | Analyze the project structure, detect languages, count files, and return onboarding instructions. |
| `prepare_for_new_conversation` | workflow | Prepare a session handoff summary for continuation in a new conversation. |

<!-- END TOOLS -->

## Architecture

Serena is built as a 4-layer Go binary:

```
MCP Runtime (stdio/HTTP transports, tool registry, profile middleware)
    |
Code Intelligence Kernel (LS worker pool, symbol ops, file ops, diagnostics)
    |
Skills & Multi-Language (52-language registry, memory system, skill plugins)
    |
Agent Profiles & Setup (5 profiles, 4 modes, token budget, layered config, setup CLI)
```

The **persistent daemon** keeps language servers warm between sessions. The **worker pool** uses share-until-dirty semantics with adaptive TTL, circuit breaking for crashy servers, and platform-aware memory pressure eviction.

The **admin listener** exposes health checks (`/healthz`, `/readyz`), Prometheus metrics, and gated pprof on a dedicated loopback port.

## Acknowledgements

A significant part of Serena, especially support for various languages, was contributed by the open source community.
We are very grateful for the many contributors who made this possible and who played an important role in making Serena
what it is today.

---

<sub>Originally inspired by [Python Serena](https://github.com/lks-ai/serena).</sub>
