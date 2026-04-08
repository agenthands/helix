<p align="center" style="text-align:center;">
  <img src="resources/serena-logo.svg#gh-light-mode-only" style="width:500px">
  <img src="resources/serena-logo-dark-mode.svg#gh-dark-mode-only" style="width:500px">
</p>

<h3 align="center">
    Serena is the IDE for your coding agent.
</h3>

* Serena provides essential **semantic code retrieval, editing and refactoring tools** that are akin to an IDE's capabilities,
  operating at the symbol level and exploiting relational structure.
* It integrates with any client/LLM via the model context protocol (**MCP**).
* Ships as a **single Go binary** — no Python, no Docker, no runtime dependencies beyond the binary itself.

Serena's **agent-first tool design** involves robust high-level abstractions, distinguishing it from
approaches that rely on low-level concepts like line numbers or primitive search patterns.

Practically, this means that your agent operates **faster, more efficiently and more reliably**, especially in larger and
more complex codebases.

## How Serena Works

Serena provides 38+ MCP tools for coding workflows, backed by real language servers.
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

## Programming Language Support

Serena supports **52 programming languages** via Language Server Protocol (LSP), including:

AL, Ansible, Bash, C#, C/C++, Clojure, Dart, Elixir, Elm, Erlang, Fortran, F#, GLSL, Go, Groovy, Haskell, HLSL, Java, JavaScript, Julia, Kotlin, Lean 4, Lua, Luau, Markdown, MATLAB, Nix, OCaml, Perl, PHP, PowerShell, Python, R, Ruby, Rust, Scala, Solidity, Swift, SystemVerilog, Terraform, TOML, TypeScript, Vue, WGSL, YAML, Zig, and more.

Language servers are **auto-discovered** from PATH or **downloaded on demand** via the three-tier installer (PATH lookup > managed download > helpful error message).

## Features

### Symbol Retrieval (9 tools)
- **go_to_definition** — jump to where a symbol is defined
- **find_references** — find all usages of a symbol
- **get_symbols_overview** — file outline showing all symbols
- **search_symbols** — workspace-wide symbol search by name
- **get_hover_info** — type information and documentation
- **find_implementations** — find implementations of interfaces
- **get_call_hierarchy** — callers and callees of a function
- **get_type_hierarchy** — class/interface inheritance tree
- **analyze_blast_radius** — combined references + hierarchy for impact analysis

### Symbol Editing (6 tools)
- **replace_symbol_body** — replace a function/method body with new content
- **insert_before_symbol** / **insert_after_symbol** — add code relative to a symbol
- **rename_symbol** — rename across all files in the workspace
- **safe_delete_symbol** — delete with reference checking
- All edits include **automatic post-edit diagnostic verification**

### File Operations (6 tools)
- **read_file** / **create_text_file** — read and write files
- **list_dir** / **find_files** — directory listing and glob search
- **search_in_files** — regex search across the codebase
- **replace_in_file** — regex or literal replacement

### Diagnostics (3 tools)
- **get_diagnostics** — LSP diagnostics (errors, warnings) after edits
- **get_code_actions** — quick fixes and refactoring suggestions
- **format_code** — format via LSP

### Memory System (7 tools)
- **write_memory** / **read_memory** — persistent markdown-based knowledge
- **list_memories** / **search_memories** — browse and full-text search (SQLite FTS5)
- **rename_memory** / **edit_memory** / **delete_memory** — manage knowledge base
- Project-scoped and global memories, auto-indexed with fsnotify

### Workflow Tools
- **onboard_project** — automated project analysis for new sessions
- **prepare_for_new_conversation** — session handoff with state summary

### Agent Profiles & Modes
- **5 pre-built profiles**: claude-code, codex, ide-assistant, ci-bot, full
- **4 operational modes**: read, edit, review, admin
- **switch_mode** — change available tools mid-session
- **get_token_budget** — token cost awareness for context optimization
- Profile filtering ensures each agent gets a curated, relevant tool set

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

**HTTP mode** (for IDEs, web clients, multi-client):
```bash
serena --serve --http-addr=:9091
# Connect your client to http://localhost:9091/mcp
```

### Select a Profile

```bash
serena --profile=claude-code    # Curated for Claude Code (excludes file tools it already has)
serena --profile=codex          # Curated for Codex
serena --profile=ide-assistant  # Read-focused for IDE assistants
serena --profile=ci-bot         # Read-only for CI/review bots
serena --profile=full           # All tools (default)
```

## Architecture

Serena is built as a 4-layer Go binary:

```
MCP Runtime (stdio/HTTP transports, tool registry, profile middleware)
    |
Code Intelligence Kernel (LS worker pool, symbol ops, file ops, diagnostics)
    |
Skills & Multi-Language (52-language registry, memory system, skill plugins)
    |
Agent Profiles (5 profiles, 4 modes, token budget, layered config)
```

The **persistent daemon** keeps language servers warm between sessions. The **worker pool** uses share-until-dirty semantics with adaptive TTL, circuit breaking for crashy servers, and platform-aware memory pressure eviction.

## Legacy Python Version

The original Python-based Serena is preserved in the `legacy/` directory. See `legacy/README.md` for the original documentation, including JetBrains plugin support.

## Acknowledgements

A significant part of Serena, especially support for various languages, was contributed by the open source community.
We are very grateful for the many contributors who made this possible and who played an important role in making Serena
what it is today.
