<h1 align="center">Helix</h1>

<h3 align="center">
    Helix is the code editor for your LLM.
</h3>

<!-- TODO(post-v1.9): Add Helix logo + block-diagram assets under resources/ once the
     brand assets are produced. -->

<p align="center"><b>Not a code-assistant add-on &mdash; literally the editor your coding agent drives.</b><br/>
It replaces the agent's blunt, file-based tool calls &mdash; <code>read</code> whole files, <code>grep</code>, line-number / regex edits &mdash; with precise, IDE-grade <b>semantic operations</b> backed by real language servers: go-to-definition, find-references, call hierarchy, rename-across-files, replace-symbol-body.</p>

<p align="center">CLI-first code intelligence platform &mdash; agents drive the <code>helix &lt;verb&gt;</code> CLI across 52 languages.</p>

<p align="center"><sub>Helix is an independent, Go-native project &mdash; not a fork, port, or rewrite. It is partially inspired by prior art including <a href="https://github.com/oraios/serena">Serena</a>, <a href="https://github.com/Aider-AI/aider">Aider</a>, Graphify, and others, but its kernel, daemon, and tooling are its own. (The project carried an earlier name through v1.8; the full rename of binary, CLI, env vars, config dirs, and MCP server identity to <code>helix</code> shipped at v1.9 &mdash; see <a href="CHANGELOG.md">CHANGELOG.md</a> &gt; v1.9 Breaking Changes.)</sub></p>

* Helix provides essential **semantic code retrieval, editing and refactoring tools** that are akin to an IDE's capabilities,
  operating at the symbol level and exploiting relational structure.
* Agents drive Helix through the **`helix <verb>` CLI** (via Bash); the CLI dials a warm daemon over internal gRPC. The MCP Go SDK and gRPC IPC are retained as internal daemon plumbing — no longer an agent-facing surface.
* Ships as a **single Go binary** — no Python, no Docker, no runtime dependencies beyond the binary itself.

Helix's **agent-first tool design** involves robust high-level abstractions, distinguishing it from
approaches that rely on low-level concepts like line numbers or primitive search patterns.

Practically, this means that your agent operates **faster, more efficiently and more reliably**, especially in larger and
more complex codebases.

## Table of Contents

- [How Helix Works](#how-helix-works)
- [Key Advantages](#key-advantages-over-file-based-approaches)
- [Quick Start](#quick-start)
- [Key Features](#key-features)
- [RepoMap](#repomap)
- [Production & Observability](#production--observability)
- [Languages](#programming-language-support)
- [Tools](#features)
- [Architecture](#architecture)
- [Acknowledgements](#acknowledgements)

## How Helix Works

Helix exposes its coding-workflow operations as `helix <verb>` CLI commands, backed by real language servers.
An LLM orchestrates these verbs (via Bash) to navigate, understand, and edit code.

Helix runs as a **persistent daemon** that keeps language servers warm between sessions.
Agents invoke `helix <verb>` via Bash; the CLI dials the warm daemon over **internal gRPC** (a unix socket by default). The MCP Go SDK and gRPC IPC live behind that wire as internal daemon plumbing — they are not an agent-facing surface.

<!-- Block diagram asset deferred — see TODO at top of file. -->

## Key Advantages Over File-Based Approaches

Every row below is a tool call your agent makes today — `Read`, `Grep`, `Edit`, `Bash` — that Helix replaces with a semantic, language-server-backed equivalent:

| | Agent's default tools (`Read`/`Grep`/`Edit`/`Bash`) | Helix |
|---|---|---|
| **Navigation** | grep, find, read whole files | Go to definition, find references, symbol search, call hierarchy |
| **Editing** | Line-number replacements, regex | Replace symbol body, insert before/after, rename across files |
| **Safety** | Hope the line numbers are right | Reference checking before delete, post-edit diagnostic verification |
| **Performance** | Re-read files every session | Warm language server cache, shared across sessions |
| **Scale** | Degrades in large codebases | Symbol-level operations stay fast regardless of codebase size |

## Quick Start

### Install

Pre-built binaries for darwin/linux/windows on amd64/arm64 are published on the [Releases page](https://github.com/agenthands/helix/releases) with SHA-256 checksums and minisign signatures. See [INSTALL.md](INSTALL.md) for the full verification recipe.

Or build from source:

```bash
git clone https://github.com/agenthands/helix.git
cd helix
go build ./cmd/helix
```

#### macOS users: Gatekeeper workaround

Helix's darwin binaries are signed with [Sigstore](https://www.sigstore.dev/) cosign keyless attestation but are **not signed with an Apple Developer ID** and **not notarized** through Apple's notary service. On first launch, macOS Gatekeeper will block the binary. Use the right-click → Open workaround (one-time per binary) documented in [INSTALL.md](./INSTALL.md#macos-gatekeeper-workaround).

Apple Developer ID signing + notarization is tracked as `DEF-59-NOTARIZE` in `.planning/deferred-items.md` and may land in a future release.

### Configure Your Client

```bash
helix setup claude-code    # Claude Code
helix setup vscode         # VS Code
helix setup jetbrains      # JetBrains IDEs
helix setup gemini-cli     # Gemini CLI
helix setup claude-desktop # Claude Desktop
helix setup opencode       # OpenCode
helix setup generic        # Generic client (MCP-teardown only; no skill)
```

Add `--global` for user-wide registration. Run `helix setup --help` for all options.

Helix uses **lazy initialization** — workspaces are configured on first tool call, so there is no upfront indexing delay.

<details>
<summary>Manual configuration (without helix setup)</summary>

`helix setup <client>` is the supported path — it installs the agent skill + hooks so the agent drives the `helix <verb>` CLI directly (via Bash). There is no MCP-server registration to hand-write: the legacy agent-facing daemon transport modes and HTTP endpoint were removed; only the internal auto daemon remains, dialed over gRPC by the CLI.

If you prefer to wire a client by hand, point it at the `helix` binary's per-verb commands and choose a profile with `--profile`:

```bash
# The agent calls verbs directly, e.g.:
helix go-to-definition path/to/file.go:42:8
helix find-references path/to/file.go:42:8 --profile=codex

# Inspect the available verbs and profiles:
helix --help
helix setup --help
```

For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode — see [INSTALL.md#manual-configuration](INSTALL.md#manual-configuration) for full examples.

</details>

### Select a Profile

```bash
helix --profile=claude-code    # Curated for Claude Code (excludes file tools it already has)
helix --profile=codex          # Curated for Codex
helix --profile=ide-assistant  # Read-focused for IDE assistants
helix --profile=ci-bot         # Read-only for CI/review bots
helix --profile=full           # All tools (default)
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
| **Setup CLI** | One-command skill + hooks install (and prior-MCP teardown): `helix setup claude-code` with language detection and health check |

## RepoMap

Helix includes a structural code intelligence engine for understanding repository layout and finding task-relevant context:

- **`get_repo_map`** — Generates a structural overview of the repository using tree-sitter tag extraction and PageRank ranking. Token-budget-aware output scales to any repository size.
- **`get_context`** — Given a set of files relevant to your task, returns ranked symbols and definitions across the codebase that are most relevant, using dependency graph analysis.

## Production & Observability

Helix ships with built-in production infrastructure — metrics, tracing, health checks, and graceful degradation — so you can deploy it as a long-running service with confidence.

### Metrics & Monitoring

- Prometheus `/metrics` endpoint with RED histograms (rate, errors, duration) per tool
- lspool health gauges: active workers, evictions, restarts, circuit state
- Bounded-label contract (allowlist: `tool_name`, `profile`, `mode`, `language`, `outcome`) enforced by CI lint

### Distributed Tracing

- End-to-end trace propagation: forwarder -> daemon -> kernel -> language server via `otelgrpc`
- Per-tool sub-spans in telemetry middleware
- Optional OTLP/gRPC exporter — off by default (`ParentBased(TraceIDRatioBased(0.0))`)

### Admin Endpoints

A dedicated loopback admin listener, isolated from daemon traffic, exposes:

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

Helix supports **52 programming languages** via Language Server Protocol (LSP):

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

### Capability tiers

LSP coverage is broad, but the *depth* of intelligence varies by language across three independent layers:

| Layer | What it powers | Coverage |
|-------|----------------|----------|
| **LSP** | Semantic navigation & edits — goto-def, references, rename, hover, hierarchy, diagnostics | **52 languages** (every row above) |
| **RepoMap + body edit** | Native tree-sitter tag extraction for `get_repo_map`/`get_context`, scope-aware elision, **AST body surgery** for `replace_symbol_body` | **23 grammars / ~21 first-class** |
| **Semantic graph** | Live fact graph — `find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`, cross-file type ladder | **3 languages** (Go, Python, TS/JS) |

A language is **first-class** when it has both a warm LSP server *and* a native tree-sitter grammar (RepoMap tags + body-surgery editing). The three semantic-graph languages are the deepest tier (see below).

**Legend:** ✅ full · ◐ via LSP `documentSymbol` fallback · — not available · ✓ exercised by a Go test · ⓛ covered only by the legacy Python suite.

#### First-class (LSP + native grammar + body-surgery edits)

| Language | LSP | RepoMap | Body edit | Sem. graph | Type ladder | Fixtures |
|----------|:---:|:-------:|:---------:|:----------:|:-----------:|:--------:|
| Go | ✅ | ✅ | ✅ | ✅ | ✅ | ✓ |
| Python | ✅ | ✅ | ✅ | ✅ | ✅ | ✓ |
| TypeScript / JavaScript | ✅ | ✅ | ✅ | ✅ | ✅ | ✓ |
| Java | ✅ | ✅ | ✅ | — | ✅ | ✓ |
| PHP | ✅ | ✅ | ✅ | — | ✅ | ✓ |
| Ruby | ✅ | ✅ | ✅ | — | ✅ | ✓ |
| Rust | ✅ | ✅ | ✅ | — | — | ✓ |
| C / C++ | ✅ | ✅ | ✅ | — | — | ✓ |
| C# | ✅ | ✅ | ✅ | — | — | ✓ |
| Kotlin | ✅ | ✅ | ✅ | — | — | ✓ |
| Scala | ✅ | ✅ | ✅ | — | — | ✓ |
| Swift | ✅ | ✅ | ✅ | — | — | ✓ |
| Lua | ✅ | ✅ | ✅ | — | — | ✓ |
| Zig | ✅ | ✅ | ✅ | — | — | ✓ |
| Julia | ✅ | ✅ | ✅ | — | — | ✓ |
| R | ✅ | ✅ | ✅ | — | — | ✓ |
| Bash | ✅ | ✅ | ✅ | — | — | ✓ |

#### Grammar only (RepoMap tags, no body-surgery edits yet)

| Language | LSP | RepoMap | Body edit | Fixtures |
|----------|:---:|:-------:|:---------:|:--------:|
| Haskell | ✅ | ✅ | — | ✓ |
| OCaml | ✅ | ✅ | — | ✓ |
| Terraform (HCL) | ✅ | ✅ | — | ⓛ |

#### LSP only (RepoMap via `documentSymbol` fallback)

Markdown, Vue, Clojure, Elixir, Perl, PowerShell, Dart, Elm, Erlang, Fortran, F#, Groovy, HLSL, MATLAB, Nix, Pascal, Rego, Solidity, SystemVerilog, TOML, YAML, AL, Ansible — full LSP navigation, RepoMap via `documentSymbol` (◐), no native grammar or body-surgery editing.

### Semantic graph (Go, Python, TS/JS)

The semantic graph (`internal/semantic/`) — the live fact graph behind `find_related_symbols`, `explain_symbol_deep`, and `validate_graph_edge` — is intentionally **narrower** than the LSP/tree-sitter surface above. It supports exactly three languages:

| Language | Provider package | Covers |
|----------|------------------|--------|
| Go | `internal/semantic/extract/golang/` | `.go` |
| Python | `internal/semantic/extract/python/` | `.py` |
| TypeScript / JavaScript | `internal/semantic/extract/typescript/` | `.ts .tsx .js .jsx .mjs .cjs` |

It is a **per-language extractor model**, not a reuse of the 23-grammar RepoMap tagger:

- Each language ships its own `extract.Provider` with a dedicated tree-sitter query file (`golang/queries.scm`, `python/queries.scm`, `typescript/queries.scm` — the only three `.scm` files under `internal/semantic/`).
- Providers are wired into the daemon-owned `Registry` keyed by `Provider.Language()` (`internal/semantic/extract/registry.go`), which **panics on a duplicate or missing provider** — so the supported set is a hard, enumerated allow-list.
- The graph engine itself (`internal/semantic/graph/`) is **language-agnostic** — it operates on abstract `NodeID`/`EdgeKind`, so the only gating factor is "does a `Provider` exist for this language." Files in any other language get `ExtractionStatus = "unsupported"`.

This is a deliberate first-release scope (per `SPEC-DRAFT.md`: *"scope production correctness to Go, TypeScript/JavaScript, and Python first, with other languages best-effort"*). Adding a fourth language means implementing a new `extract.Provider` package with its own `queries.scm`.

## Features

<!-- BEGIN TOOLS -->
| Tool | Category | Description |
|------|----------|-------------|
| `helix get-diagnostics` | diagnostics | Returns current diagnostics (errors, warnings) for a file |
| `helix get-code-actions` | diagnostics | Returns available code actions/quick fixes for a position or range |
| `helix format-code` | diagnostics | Formats a file via the language server and writes the result |
| `helix read-file` | file-ops | Read a file's content, optionally a specific line range |
| `helix create-file` | file-ops | Create a new file with content (errors if file already exists) |
| `helix list-directory` | file-ops | List directory contents with file type, size, and modification time |
| `helix find-files` | file-ops | Find files matching a glob pattern (supports ** for recursive matching) |
| `helix search-in-files` | file-ops | Search for a regex pattern across the codebase, with optional context lines |
| `helix replace-in-file` | file-ops | Replace all occurrences of a pattern in a file (literal or regex) |
| `helix fuzzy-edit` | file-ops | Fuzzy-match and replace text in a file using 4-strategy cascade |
| `helix get-health` | health | Get workspace health status and language server states |
| `helix get-tool-help` | help | Get comprehensive documentation for any MCP tool including parameters, types, and usage examples |
| `helix write-memory` | memory | Write information about this project that can be useful for future tasks to a memory in md format. |
| `helix read-memory` | memory | Read the content of a memory file. |
| `helix list-memories` | memory | List available memories, optionally filtered by scope ("project" or "global") and topic. |
| `helix search-memories` | memory | Search memories by full-text query, optionally filtered by scope ("project" or "global"). |
| `helix rename-memory` | memory | Rename or move a memory. |
| `helix edit-memory` | memory | Edit a memory by replacing a search string with a replacement string. |
| `helix delete-memory` | memory | Delete a memory file. |
| `helix switch-mode` | profile | Switch the current session's operational mode (read/edit/review/admin) |
| `helix get-token-budget` | profile | Get token budget breakdown for the current or specified profile/mode |
| `helix get-repo-map` | repomap | Get a ranked structural overview of the repository. |
| `helix get-context` | repomap | Get the most relevant code context for a set of files or a task. |
| `helix index-semantic-graph` | semantic | Build or refresh a committed semantic snapshot. |
| `helix refresh-semantic-graph` | semantic | Apply pending live source changes (read+). |
| `helix get-semantic-graph-status` | semantic | Return semantic graph status (read+). |
| `helix get-semantic-context` | semantic | Ranked, evidence-backed semantic context (read+). |
| `helix explain-symbol-deep` | semantic | Deep symbol explanation: type chain, callers, edges, cluster (read+). |
| `helix find-related-symbols` | semantic | Top-k semantically related symbols around a seed (read+). |
| `helix validate-graph-edge` | semantic | Validate a (from, to, edge_kind) graph claim with confidence + evidence (read+). |
| `helix get-cluster-map` | semantic | Workspace-level cluster overview: count, top-N clusters, members, representative symbols, dominant edge kinds (read+). |
| `helix explain-cluster` | semantic | Full cluster member list with per-member PageRank, cohesion/conductance metrics, and dominant entry points (read+). |
| `helix get-change-impact-graph` | semantic | Pre-edit blast-radius subgraph (nodes + edges + edge kinds) for a seed symbol (review+). |
| `helix replace-symbol-body` | symbol-editing | Replace a symbol's body with new content using tree-sitter for precise extraction |
| `helix insert-before-symbol` | symbol-editing | Insert content immediately before a symbol |
| `helix insert-after-symbol` | symbol-editing | Insert content immediately after a symbol |
| `helix rename-symbol` | symbol-editing | Rename a symbol across all files in the workspace |
| `helix safe-delete-symbol` | symbol-editing | Delete a symbol if it has no references; reports reference count if blocked |
| `helix verify-edit` | symbol-editing | Check for compilation errors after an edit; returns diagnostic summary |
| `helix go-to-definition` | symbol-retrieval | Go to the definition of a symbol at a given position |
| `helix find-references` | symbol-retrieval | Find all references to a symbol at a given position |
| `helix get-symbol-overview` | symbol-retrieval | Get a hierarchical outline of all symbols in a file |
| `helix search-symbols` | symbol-retrieval | Search for symbols across the workspace by name |
| `helix get-hover-info` | symbol-retrieval | Get hover/type information for a symbol at a given position |
| `helix find-implementations` | symbol-retrieval | Find all implementations of an interface or abstract method |
| `helix get-call-hierarchy` | symbol-retrieval | Get call hierarchy (callers and/or callees) for a symbol |
| `helix get-type-hierarchy` | symbol-retrieval | Get type hierarchy (subtypes and/or supertypes) for a symbol |
| `helix analyze-blast-radius` | symbol-retrieval | Analyze the blast radius (impact) of changing a symbol |
| `helix analyze-blast-radius` | symbols | Analyze the blast radius (impact) of changing a symbol |
| `helix onboard-project` | workflow | Analyze the project structure, detect languages, count files, and return onboarding instructions. |
| `helix prepare-for-new-conversation` | workflow | Prepare a session handoff summary for continuation in a new conversation. |

<!-- END TOOLS -->

## Architecture

Helix is built as a 4-layer Go binary:

```
CLI + Daemon Runtime (helix-verb CLI over internal gRPC, tool registry, profile middleware)
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

Helix is an independent Go project, but it stands on the shoulders of prior art and a large open-source ecosystem.

**Inspiration.** The agent-tooling design draws on ideas from [Serena](https://github.com/oraios/serena) (symbol-level MCP operations over LSP), [Aider](https://github.com/Aider-AI/aider) (the ranked, PageRank-style RepoMap), Graphify, and other code-intelligence projects.

**Foundations.** Helix is built directly on top of:

- the dozens of open-source **language servers** it drives through LSP, and the communities that maintain them;
- the [**tree-sitter**](https://github.com/tree-sitter/tree-sitter) grammars used for RepoMap tag extraction and AST body surgery;
- the official [**MCP Go SDK**](https://github.com/modelcontextprotocol/go-sdk).

Without these projects Helix would not exist. See [CHANGELOG.md](CHANGELOG.md) for release notes.
