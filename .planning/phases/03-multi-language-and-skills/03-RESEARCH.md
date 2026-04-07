# Phase 3: Multi-Language and Skills - Research

**Researched:** 2026-04-07
**Domain:** Multi-language LSP registry, memory persistence with SQLite FTS, skill/plugin interface
**Confidence:** HIGH

## Summary

Phase 3 expands the working 4-language kernel to 40+ languages, adds a markdown-based memory system with SQLite FTS5 search, and establishes the skill/plugin interface pattern. The language expansion is primarily a data problem -- 52 legacy Python adapters provide exact LS binary names, args, init options, and quirks. The memory system is straightforward filesystem CRUD with a derived search index. The skill interface is the most design-sensitive piece, needing clean Go interfaces that the Phase 4 agent profiles will compose.

All three subsystems are well-understood: the legacy codebase provides proven patterns for each, and the Go ecosystem has mature libraries for every component (modernc.org/sqlite for CGO-free FTS5, fsnotify for file watching, Caddy-style init() registration for skills).

**Primary recommendation:** Port language configs data-first (embedded Go registry structs from legacy Python adapters), then layer auto-download and quirk adapters. Build the memory system as a self-contained package. Define the skill interface early so memory tools and workflow tools are the first two skills implemented through it.

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Embedded Go registry compiled into the binary. All 40+ LS configs (binary path, args, init options, capabilities) are compiled-in Go structs. No external catalog file dependency at boot.
- **D-02:** YAML override/extend mechanism. Users can override any embedded entry or add new languages via `.serena/languages.yaml` or `~/.serena/languages.yaml`.
- **D-03:** LS installation: prefer existing install first (PATH lookup), managed download as fallback -- not blind auto-download. Both modes available: auto (default) or manual-only (flag/config).
- **D-04:** Per-language quirk handling extends existing `lspool/quirks.go` pattern. Each language gets a QuirkAdapter implementing initialization sequences, capability differences, encoding quirks behind the uniform LSAdapter interface.
- **D-05:** Port language configs from `legacy/src/solidlsp/language_servers/` as reference. Each Python adapter maps to a Go QuirkAdapter + registry entry.
- **D-06:** Canonical storage: Markdown files. `.serena/memories/**/*.md` (project) and `~/.serena/memories/**/*.md` (global). Humans own the content.
- **D-07:** Derived index: SQLite with FTS. `.serena/index/memories.db` (disposable, rebuildable). Engine owns the search.
- **D-08:** Index contains: name, topic, file path, scope (project/global), title/headings, tags, modified time, content hash, short extracted summary. Optional FTS table for body search.
- **D-09:** Rebuild behavior: automatic on startup, FS watch for changes, explicit reindex command. Corrupted/missing DB -> rebuild from markdown files. Zero data loss on index corruption.
- **D-10:** Scoping: directory-based (global vs project). Topic/subtopic as first-class indexed metadata for filtering. Slash-separated names supported (matching current Serena pattern).
- **D-11:** Go owns behavior; YAML owns selection and prompting. Compiled Go skill modules with YAML-controlled context, descriptions, prompts, and enablement.
- **D-12:** Two Go interfaces: `Skill` (capability package) and `ToolProvider` (contributes MCP tools). Two YAML specs: `ContextSpec` and `ModeSpec` for presentation/composition.
- **D-13:** Tool = atomic MCP-exposed callable. Skill = reusable capability package that may contribute tools, prompt fragments, context defaults, policies, workflow recipes, enable/disable rules.
- **D-14:** Two kinds of skill payload via one registration model: Tool skill (contributes MCP tools) and Workflow skill (contributes prompts/context/policies, optionally uses tools internally).
- **D-15:** Every tool is protocol-facing; not every skill needs to be. Skills can influence the MCP surface without being directly callable.

### Claude's Discretion
- Specific language server binary names and download URLs for each of the 40+ languages
- SQLite schema details for the memory index
- Onboarding workflow specifics (what analysis to run, what memories to create)
- Session handoff format (what gets summarized, how it's stored)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LNG-01 | Server supports 40+ languages via LSP | 52 legacy adapters catalogued; embedded registry pattern with YAML override |
| LNG-02 | Language servers are auto-discovered and downloaded when needed | PATH lookup first, managed download fallback; 3 installer types (npm, pip, binary download) |
| LNG-03 | Per-language quirk handling via adapter layer | Existing QuirkAdapter pattern in quirks.go; extends to per-language init options, notification handlers |
| MEM-01 | User can write project-scoped memories (markdown files) | Legacy MemoriesManager pattern; slash-separated naming; `.serena/memories/` directory |
| MEM-02 | User can read memories by name | Direct file read by resolved path; `.md` extension auto-appended |
| MEM-03 | User can list and search stored memories | SQLite FTS5 index for search; directory walk for listing |
| MEM-04 | User can rename, edit, and delete memories | File operations on markdown + index update; regex/literal edit support |
| MEM-05 | Global memories persist across projects; project memories are scoped | `~/.serena/memories/` (global) vs `.serena/memories/` (project); `global/` prefix convention |
| WFL-01 | Automated onboarding workflow generates project understanding | Legacy onboarding prompt template; returns instructions for agent to analyze and write memories |
| WFL-02 | Prepare-for-new-conversation summarizes session state for handoff | Legacy template; summarizes context into a memory for next session |
| WFL-03 | Plugin/skill pack interface allows extending tools without touching core | Caddy-style init() registration; Skill + ToolProvider interfaces; YAML-driven composition |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| modernc.org/sqlite | v1.48.1 | CGO-free SQLite with FTS5 | Only production-grade CGO-free SQLite for Go; FTS5 compiled in by default; cross-compiles cleanly |
| github.com/fsnotify/fsnotify | v1.9.0 | File system notifications for memory index rebuild | Already an indirect dependency; de facto standard for Go file watching; supports recursive via `...` suffix |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| database/sql | stdlib | SQL interface for modernc.org/sqlite driver | Standard Go DB access; modernc.org/sqlite registers as `database/sql` driver |
| os/exec | stdlib | LS auto-download (npm install, pip install, binary downloads) | Spawning package manager commands for managed downloads |
| net/http | stdlib | Binary downloads for LS auto-install | Downloading pre-built LS binaries (clangd, jdtls, etc.) |
| crypto/sha256 | stdlib | Verify downloaded LS binary integrity | Checksum verification for downloaded archives |
| text/template | stdlib | Prompt templates for workflow skills | Rendering onboarding/handoff prompts with variables |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| modernc.org/sqlite | mattn/go-sqlite3 | mattn requires CGO; faster by ~20-30% but breaks cross-compilation and single-binary goal |
| modernc.org/sqlite | zombiezen.com/go/sqlite | Higher-level API wrapping modernc; adds indirection without clear benefit for our use case |
| fsnotify | polling | Simpler but higher latency and CPU; fsnotify is already a dependency |

**Installation:**
```bash
go get modernc.org/sqlite@v1.48.1
# fsnotify already in go.mod as indirect dep; promote to direct
go get github.com/fsnotify/fsnotify@v1.9.0
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
  langregistry/           # Language server registry (LNG-01, LNG-02, LNG-03)
    registry.go           # Embedded Go registry + YAML overlay
    entry.go              # LSEntry struct (command, args, init options, capabilities, install info)
    installer.go          # Auto-download logic (npm, pip, binary)
    installer_test.go
  kernel/lspool/
    quirks.go             # Extended: 40+ QuirkAdapter entries (from 4 today)
  memory/                 # Memory system (MEM-01 through MEM-05)
    store.go              # MemoryStore: CRUD on markdown files
    index.go              # SQLite FTS5 index (build, rebuild, query)
    watcher.go            # fsnotify-based auto-rebuild
    schema.go             # DB schema + migrations
    memory_test.go
  skill/                  # Skill interface (WFL-03)
    skill.go              # Skill and ToolProvider interfaces
    registry.go           # Skill registration (init()-based)
    spec.go               # ContextSpec and ModeSpec YAML types
  skill/memory/           # Memory skill (MEM tools as first skill)
    skill.go              # Registers memory MCP tools
  skill/workflow/          # Workflow skill (WFL-01, WFL-02)
    skill.go              # Onboarding, prepare-for-new-conversation, thinking tools
    prompts.go            # Embedded prompt templates
```

### Pattern 1: Embedded Registry with YAML Override (D-01, D-02)

**What:** All 40+ LS configs compiled into Go structs. Users override/extend via YAML.
**When to use:** Boot-time language resolution. Zero external file dependencies for default behavior.

```go
// internal/langregistry/entry.go
type LSEntry struct {
    Language     string            // e.g. "go", "python", "typescript"
    Command      string            // e.g. "gopls", "pyright-langserver"
    Args         []string          // e.g. ["serve"], ["--stdio"]
    InitOptions  map[string]any    // LSP initializationOptions
    NeedsWorkspace bool
    FileExts     []string          // e.g. [".go"], [".py", ".pyi"]
    Install      *InstallInfo      // nil = system-only, no managed download
    IgnoredDirs  []string          // e.g. ["vendor", "node_modules"]
}

type InstallInfo struct {
    Type    string // "npm", "pip", "binary", "cargo"
    Package string // e.g. "pyright", "bash-language-server"
    Version string // pinned version
    // For binary downloads:
    URLs    map[string]string // platform -> URL
    SHA256  map[string]string // platform -> checksum
}

// internal/langregistry/registry.go
var defaultEntries = map[string]LSEntry{
    "go": {
        Language: "go", Command: "gopls", Args: []string{"serve"},
        NeedsWorkspace: true, FileExts: []string{".go"},
        InitOptions: map[string]any{"experimentalWorkspaceModule": true},
        IgnoredDirs: []string{"vendor"},
    },
    // ... 40+ more entries
}

func NewRegistry(overridePaths ...string) *Registry {
    // Start with defaultEntries, merge YAML overrides
}
```

### Pattern 2: Three-Tier LS Installation (D-03)

**What:** PATH lookup -> managed download -> error with install instructions.
**When to use:** Every time a language is activated.

```go
// internal/langregistry/installer.go
func (i *Installer) Resolve(entry LSEntry) (command string, args []string, err error) {
    // Tier 1: Check PATH
    if path, err := exec.LookPath(entry.Command); err == nil {
        return path, entry.Args, nil
    }
    
    // Tier 2: Managed download (if auto-install enabled and InstallInfo present)
    if i.autoInstall && entry.Install != nil {
        installed, err := i.download(entry)
        if err == nil {
            return installed, entry.Args, nil
        }
        // Fall through to Tier 3 with both errors
    }
    
    // Tier 3: Error with helpful message
    return "", nil, fmt.Errorf("language server %q not found; install: %s",
        entry.Command, entry.InstallHint())
}
```

### Pattern 3: Caddy-Style Skill Registration (D-11, WFL-03)

**What:** Skills register via init() functions; YAML controls composition.
**When to use:** All skills (memory, workflow, future community skills).

```go
// internal/skill/skill.go
type Skill interface {
    Name() string
    Description() string
    // Init is called once at daemon startup with dependencies.
    Init(deps SkillDeps) error
}

// ToolProvider is a Skill that contributes MCP tools.
type ToolProvider interface {
    Skill
    // Tools returns the MCP tool definitions this skill provides.
    Tools() []*mcp.ToolDef
}

// WorkflowProvider is a Skill that contributes prompts/context.
type WorkflowProvider interface {
    Skill
    // Prompts returns prompt templates this skill provides.
    Prompts() map[string]string
}

// internal/skill/registry.go
var globalRegistry = &skillRegistry{skills: make(map[string]Skill)}

func Register(s Skill) {
    globalRegistry.mu.Lock()
    defer globalRegistry.mu.Unlock()
    globalRegistry.skills[s.Name()] = s
}

// Usage in skill packages:
// internal/skill/memory/skill.go
func init() {
    skill.Register(&MemorySkill{})
}
```

### Pattern 4: Memory Store with Derived Index (D-06, D-07)

**What:** Markdown files are source of truth; SQLite FTS5 is a disposable cache.
**When to use:** All memory CRUD operations go through MemoryStore; search goes through index.

```go
// internal/memory/store.go
type MemoryStore struct {
    projectDir string // .serena/memories/
    globalDir  string // ~/.serena/memories/
    index      *Index
    watcher    *Watcher
}

func (s *MemoryStore) Write(name, content string) error {
    path := s.resolvePath(name)
    os.MkdirAll(filepath.Dir(path), 0755)
    os.WriteFile(path, []byte(content), 0644)
    return s.index.Upsert(name, path, content)
}

func (s *MemoryStore) Search(query string) ([]MemoryEntry, error) {
    return s.index.Search(query) // FTS5 MATCH query
}
```

### Anti-Patterns to Avoid
- **Monolithic quirks file:** Do NOT put all 40+ QuirkAdapter implementations in a single quirks.go. Group by language family or keep the data in the registry and the adaptation logic generic.
- **Eager LS download:** Do NOT download language servers at daemon startup. Only resolve when a workspace activates a language.
- **Index as source of truth:** NEVER store data only in SQLite. Markdown files are canonical; index is always rebuildable.
- **Skills doing too much:** Skills should NOT directly import each other. Use the tool registry for inter-skill communication.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQLite FTS | Custom text search/indexing | modernc.org/sqlite + FTS5 | FTS5 handles tokenization, ranking, phrase matching; hand-rolling this is months of work |
| File watching | Polling loop or inotify syscalls | fsnotify | Cross-platform, battle-tested, already in dependency tree |
| Markdown parsing for index | Full markdown AST parser | Simple heading/frontmatter extraction | Only need titles, tags, headings for index; regex is sufficient |
| npm/pip package install | Custom package manager integration | `exec.Command("npm", "install", ...)` | Legacy pattern works; shell out to system package managers |
| LS binary downloads | Custom HTTP download + archive extraction | net/http + archive/zip + crypto/sha256 | Standard library covers this completely |
| Prompt templating | Custom string interpolation | text/template (stdlib) | Go's template package handles all needed variable substitution |

**Key insight:** The memory system and language registry are both data-heavy but logic-light. The complexity is in getting the data right (52 language configs, memory naming conventions), not in the algorithms.

## Common Pitfalls

### Pitfall 1: SQLite Locking with Concurrent Access
**What goes wrong:** Multiple goroutines writing to the same SQLite DB cause "database is locked" errors.
**Why it happens:** SQLite uses file-level locking; concurrent writers block each other.
**How to avoid:** Use a single writer goroutine with a channel-based queue, or use WAL mode (`PRAGMA journal_mode=WAL`) which allows concurrent reads during writes. Set `_busy_timeout=5000` in the DSN.
**Warning signs:** Sporadic "database is locked" errors under load.

### Pitfall 2: fsnotify Event Deduplication
**What goes wrong:** A single file save triggers multiple events (CREATE, WRITE, CHMOD), causing multiple index rebuilds.
**Why it happens:** OS file operations are not atomic; editors may write to temp file then rename.
**How to avoid:** Debounce events with a timer (100-500ms). Batch all events within the window and process unique files only.
**Warning signs:** Index rebuild running continuously during editing.

### Pitfall 3: Language Server Init Race on First Activation
**What goes wrong:** The LS binary does not exist on first activation, triggering a download that takes 30+ seconds while the MCP tool call times out.
**Why it happens:** Auto-download is synchronous in the activation path.
**How to avoid:** Return a progress notification immediately, download in background, report status via MCP progress. Consider a pre-check command (`serena check-languages`) for offline preparation.
**Warning signs:** First tool call after fresh install takes >30s or times out.

### Pitfall 4: Memory Name Collisions Between Global and Project
**What goes wrong:** User writes `global/style` and `style` memories; searches return unexpected results.
**Why it happens:** `global/` prefix convention is a naming convention, not a namespace separator.
**How to avoid:** The legacy pattern handles this well: `global/` prefix routes to `~/.serena/memories/`; everything else routes to `.serena/memories/`. Preserve this convention exactly. Store scope as indexed metadata.
**Warning signs:** Users confused about which memory they are reading/writing.

### Pitfall 5: modernc.org/sqlite Build Time
**What goes wrong:** First build with modernc.org/sqlite takes 3-5 minutes due to transpiled C code.
**Why it happens:** The library is a mechanical translation of SQLite's C source to Go; it generates ~200K lines of Go code.
**How to avoid:** Accept the one-time build cost; subsequent builds are cached. Document this in the development setup. Consider build cache warming in CI.
**Warning signs:** CI builds suddenly 5x slower after adding dependency.

### Pitfall 6: YAML Override Merge Semantics
**What goes wrong:** User YAML partially overrides an entry, losing fields they did not specify.
**Why it happens:** Naive YAML unmarshal replaces entire structs.
**How to avoid:** Use deep merge semantics: YAML fields override only the fields they specify; unspecified fields retain embedded defaults. The koanf library already supports this pattern.
**Warning signs:** User overrides `command` but loses `initOptions`.

## Code Examples

### SQLite FTS5 Index Schema and Queries

```go
// internal/memory/schema.go
const createSchema = `
CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    file_path TEXT NOT NULL,
    scope TEXT NOT NULL CHECK(scope IN ('project', 'global')),
    topic TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    headings TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL,
    modified_at DATETIME NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    name, title, headings, tags, summary, content,
    content_rowid='id'
);

CREATE INDEX IF NOT EXISTS idx_memories_scope ON memories(scope);
CREATE INDEX IF NOT EXISTS idx_memories_topic ON memories(topic);
`

// internal/memory/index.go
func (idx *Index) Search(query string, scope string) ([]MemoryEntry, error) {
    sql := `SELECT m.name, m.file_path, m.scope, m.topic, m.title, m.summary,
                   rank
            FROM memories_fts f
            JOIN memories m ON f.rowid = m.id
            WHERE memories_fts MATCH ?`
    args := []any{query}
    if scope != "" {
        sql += " AND m.scope = ?"
        args = append(args, scope)
    }
    sql += " ORDER BY rank LIMIT 20"
    // ... execute query
}
```

### Memory Store Path Resolution (Port of Legacy Pattern)

```go
// internal/memory/store.go
func (s *MemoryStore) resolvePath(name string) (string, Scope) {
    name = strings.TrimSuffix(name, ".md")
    
    if strings.HasPrefix(name, "global/") {
        subName := strings.TrimPrefix(name, "global/")
        if subName == "" {
            return "", "" // invalid: bare "global" is not a valid memory name
        }
        return filepath.Join(s.globalDir, subName+".md"), ScopeGlobal
    }
    
    return filepath.Join(s.projectDir, name+".md"), ScopeProject
}
```

### Embedded Language Registry Entry (Data from Legacy)

```go
// internal/langregistry/languages.go
// Generated/maintained from legacy/src/solidlsp/language_servers/*.py

var defaultEntries = map[string]LSEntry{
    "go": {
        Language: "go", Command: "gopls", Args: []string{"serve"},
        NeedsWorkspace: true, FileExts: []string{".go"},
        InitOptions: map[string]any{"experimentalWorkspaceModule": true},
        IgnoredDirs: []string{"vendor", "node_modules", "dist", "build"},
    },
    "python": {
        Language: "python", Command: "pyright-langserver", Args: []string{"--stdio"},
        NeedsWorkspace: true, FileExts: []string{".py", ".pyi"},
        Install: &InstallInfo{Type: "pip", Package: "pyright"},
    },
    "typescript": {
        Language: "typescript", Command: "typescript-language-server", Args: []string{"--stdio"},
        NeedsWorkspace: true, FileExts: []string{".ts", ".tsx", ".mts", ".cts"},
        Install: &InstallInfo{Type: "npm", Package: "typescript-language-server", Version: "5.1.3"},
    },
    "rust": {
        Language: "rust", Command: "rust-analyzer", Args: nil,
        NeedsWorkspace: true, FileExts: []string{".rs"},
        InitOptions: map[string]any{
            "cargo": map[string]any{"buildScripts": map[string]any{"enable": true}},
        },
    },
    "bash": {
        Language: "bash", Command: "bash-language-server", Args: []string{"start"},
        NeedsWorkspace: false, FileExts: []string{".sh", ".bash"},
        Install: &InstallInfo{Type: "npm", Package: "bash-language-server", Version: "5.6.0"},
    },
    "cpp": {
        Language: "cpp", Command: "clangd", Args: []string{"--background-index"},
        NeedsWorkspace: false, FileExts: []string{".c", ".cpp", ".cc", ".h", ".hpp"},
        Install: &InstallInfo{
            Type: "binary",
            URLs: map[string]string{
                "linux-x64":  "https://github.com/clangd/clangd/releases/download/19.1.2/clangd-linux-19.1.2.zip",
                "darwin-x64": "https://github.com/clangd/clangd/releases/download/19.1.2/clangd-mac-19.1.2.zip",
                "darwin-arm64": "https://github.com/clangd/clangd/releases/download/19.1.2/clangd-mac-19.1.2.zip",
            },
        },
    },
    "java": {
        Language: "java", Command: "jdtls", Args: nil,
        NeedsWorkspace: true, FileExts: []string{".java"},
        // jdtls requires complex setup with workspace data dir -- special QuirkAdapter
        Install: &InstallInfo{Type: "binary", Package: "eclipse.jdt.ls"},
    },
    "vue": {
        Language: "vue", Command: "vue-language-server", Args: []string{"--stdio"},
        NeedsWorkspace: true, FileExts: []string{".vue"},
        Install: &InstallInfo{Type: "npm", Package: "@vue/language-server", Version: "3.1.5"},
        // Vue requires companion TypeScript LS -- special QuirkAdapter
    },
    // ... remaining 40+ entries from legacy adapters
}
```

### Complete Language List (from legacy/src/solidlsp/language_servers/)

52 adapter files found. Mapped to registry entries:

| Language | Binary | Install Type | Quirk Level |
|----------|--------|-------------|-------------|
| Go | gopls | system | Medium (custom init, symbol normalization) |
| Python (pyright) | pyright-langserver | pip | Low |
| Python (jedi) | jedi-language-server | pip | Low |
| TypeScript | typescript-language-server | npm | Low |
| Rust | rust-analyzer | system/rustup | Medium (cargo build scripts) |
| Bash | bash-language-server | npm | Low |
| C/C++ (clangd) | clangd | binary download | Medium (compile_commands.json) |
| C/C++ (ccls) | ccls | system | Medium |
| Java (jdtls) | jdtls | binary download | HIGH (workspace dir, gradle, plugins) |
| C# (omnisharp) | OmniSharp | binary download | Medium |
| C# (roslyn) | csharp-ls | system | Low |
| Vue | vue-language-server | npm | HIGH (companion TS server, hybrid mode) |
| Kotlin | kotlin-language-server | binary download | Medium |
| Scala | metals | system/coursier | Medium |
| Ruby (ruby-lsp) | ruby-lsp | gem | Low |
| Ruby (solargraph) | solargraph | gem | Low |
| PHP (intelephense) | intelephense | npm | Low |
| PHP (phpactor) | phpactor | system | Low |
| Perl | perl-language-server | cpan | Low |
| PowerShell | PowerShellEditorServices | binary download | Medium |
| F# | fsautocomplete | dotnet tool | Medium |
| Dart | dart-language-server | system | Low |
| Elixir | elixir-ls | binary download | Medium |
| Erlang | erlang-ls | system | Low |
| Haskell | haskell-language-server | ghcup | Medium |
| Lua | lua-language-server | binary download | Low |
| Luau | luau-lsp | system | Low |
| Julia | julia-language-server | julia pkg | Medium |
| OCaml | ocamllsp | opam | Low |
| Lean 4 | lean4 | system | Low |
| Fortran | fortls | pip | Low |
| YAML | yaml-language-server | npm | Low |
| TOML (taplo) | taplo | cargo/binary | Low |
| Markdown (marksman) | marksman | binary download | Low |
| Terraform | terraform-ls | binary download | Low |
| Zig | zls | system | Low |
| Clojure | clojure-lsp | binary download | Low |
| Swift | sourcekit-lsp | system | Low |
| HLSL | hlsl-tools-lsp | system | Low |
| R | r-languageserver | R package | Low |
| Groovy | groovy-language-server | binary download | Low |
| Elm | elm-language-server | npm | Low |
| Pascal | pasls | system | Low |
| Matlab | matlab-language-server | system | Low |
| Nix | nixd | system | Low |
| Solidity | solidity-ls | npm | Low |
| SystemVerilog | verible | system | Low |
| Ansible | ansible-language-server | npm | Low |
| Rego (OPA) | regal | binary download | Low |
| Python (ty) | ty | pip | Low |

**Quirk levels:**
- **Low:** Standard --stdio, no special init, basic capabilities. Just registry data.
- **Medium:** Custom init options, special notification handlers, symbol normalization.
- **HIGH:** Companion servers, complex setup (workspace dirs, plugins), custom protocol extensions.

### QuirkAdapter Evolution

The current `LanguageQuirks` struct in `quirks.go` is data-only (command, args, init options). For Phase 3, it needs to become an interface to handle behavioral quirks:

```go
// Evolution of quirks.go
type QuirkAdapter interface {
    // InitOptions returns language-specific initialization options.
    InitOptions(workDir string) map[string]any
    // NotificationHandlers returns handlers for language-specific notifications.
    NotificationHandlers() map[string]func(params json.RawMessage)
    // NormalizeSymbolName adjusts symbol names for language conventions.
    NormalizeSymbolName(name string) string
    // IgnoredDirs returns directories to skip for this language.
    IgnoredDirs() []string
    // PostInitialize is called after successful LSP initialize handshake.
    PostInitialize(adapter *LSAdapter) error
}

// DefaultQuirkAdapter provides no-op defaults for languages with no quirks.
type DefaultQuirkAdapter struct{}

// GoplsQuirks handles Go-specific behavior.
type GoplsQuirks struct{ DefaultQuirkAdapter }
func (g *GoplsQuirks) NormalizeSymbolName(name string) string {
    // Strip package prefix: "pkg.Foo" -> "Foo"
    return name[strings.LastIndex(name, ".")+1:]
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| mattn/go-sqlite3 (CGO) | modernc.org/sqlite (pure Go) | 2022-2023 | Eliminates CGO dependency; cross-compilation works |
| FTS3/FTS4 | FTS5 | SQLite 3.9.0 (2015) | Better ranking, configurable tokenizers, column filters |
| Go plugin package | Interface + init() registration | Always preferred | Go's `plugin` package is Linux-only and fragile; Caddy pattern is universal |
| fsnotify v1.x polling | fsnotify v1.9.0 with `...` recursive | 2024-2025 | Native recursive watching support (path + `/...`) |

**Deprecated/outdated:**
- Go `plugin` package: Platform-limited, version-sensitive, widely avoided. Use compiled-in registration.
- mattn/go-sqlite3: Still works but adds CGO requirement. modernc.org/sqlite is the Go-native choice.

## Open Questions

1. **jdtls Setup Complexity**
   - What we know: Eclipse JDTLS requires a workspace data directory, specific Java version, and complex launcher arguments. The legacy adapter is 400+ lines.
   - What's unclear: How much of this complexity can be abstracted into the registry entry vs requiring a dedicated QuirkAdapter.
   - Recommendation: Implement jdtls as a HIGH-quirk adapter with its own setup routine. Accept that it will be the most complex single adapter.

2. **Vue Companion Server Pattern**
   - What we know: Vue requires a companion TypeScript server running alongside the Vue LS. The legacy implementation is 700+ lines.
   - What's unclear: Whether the Phase 2 worker pool can manage two workers per "language" (Vue = Vue LS + TS LS).
   - Recommendation: Model Vue as a single WorkspaceKey with the QuirkAdapter internally managing the companion process. The pool sees one worker; the adapter manages two processes.

3. **LS Version Pinning Strategy**
   - What we know: Legacy pins specific versions (e.g., bash-language-server 5.6.0, typescript-language-server 5.1.3).
   - What's unclear: How often these versions need updating and who maintains them.
   - Recommendation: Pin versions in embedded registry. YAML override lets users update without waiting for a Serena release. Document version update process.

## Project Constraints (from CLAUDE.md)

- **Formatting:** `uv run poe format` (RUFF) -- only allowed formatting command
- **Type checking:** `uv run poe type-check` (mypy) -- only allowed type checking command
- **Testing:** `uv run poe test` with marker-based selection
- **Pre-completion:** Always run format, type-check, and test before completing any task
- **Go module:** Uses `uv` for Python legacy but Go code uses standard `go build`/`go test`
- **Language support:** 19 languages with LSP integration currently in legacy (expanding to 40+ in Phase 3)

Note: CLAUDE.md commands apply to the legacy Python codebase. New Go code uses `go test`, `go vet`, and the project's Go tooling.

## Sources

### Primary (HIGH confidence)
- Legacy codebase: `legacy/src/solidlsp/language_servers/*.py` -- 52 adapter files with exact LS configs, init options, download URLs
- Legacy codebase: `legacy/src/serena/project.py` -- MemoriesManager implementation (naming, scoping, CRUD)
- Legacy codebase: `legacy/src/serena/tools/memory_tools.py` -- Memory tool API surface
- Legacy codebase: `legacy/src/serena/tools/workflow_tools.py` -- Onboarding and handoff tool patterns
- Legacy codebase: `legacy/src/serena/config/context_mode.py` -- Context and Mode YAML loading pattern
- Existing code: `internal/kernel/lspool/quirks.go` -- Current QuirkAdapter data structure
- Existing code: `internal/mcp/registry.go` -- Tool registration pattern
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) -- CGO-free SQLite with FTS5 compiled in

### Secondary (MEDIUM confidence)
- [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) -- v1.9.0, recursive watching via `...` suffix
- [Caddy plugin architecture](https://caddyserver.com/docs/extending-caddy) -- init() registration pattern for compiled-in modules
- [modernc.org/sqlite deep wiki](https://deepwiki.com/modernc-org/sqlite) -- FTS5 support confirmed as compiled-in extension

### Tertiary (LOW confidence)
- jdtls download URLs and setup complexity -- need validation against current release
- Specific LS version currency -- legacy versions may be stale

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - modernc.org/sqlite and fsnotify are well-established, verified available
- Architecture: HIGH - all patterns directly port from proven legacy implementations
- Language registry: HIGH - 52 legacy adapters provide exact specifications
- Memory system: HIGH - legacy MemoriesManager is simple and well-understood
- Skill interface: MEDIUM - interface design is new (no legacy equivalent), but follows established Go patterns (Caddy)
- Pitfalls: HIGH - based on direct legacy codebase experience and Go ecosystem knowledge

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable domain, slow-moving dependencies)
