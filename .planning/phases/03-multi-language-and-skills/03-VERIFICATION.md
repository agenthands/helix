---
phase: 03-multi-language-and-skills
verified: 2026-04-07T22:00:00Z
status: passed
score: 11/11 must-haves verified
gaps: []
---

# Phase 03: Multi-Language and Skills Verification Report

**Phase Goal:** The server supports 40+ languages with auto-discovery and quirk handling, persists project knowledge across sessions, and provides workflow skills for onboarding and session handoff
**Verified:** 2026-04-07
**Status:** PASSED
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | 40+ languages have embedded registry entries with command, args, file extensions, and install info | VERIFIED | `languages.go` contains 52 `Language:` entries in `defaultEntries` map (grep count: 52) |
| 2 | Registry merges YAML overrides from .serena/languages.yaml and ~/.serena/languages.yaml on top of embedded defaults | VERIFIED | `NewRegistry(overridePaths ...string)` implements deep-merge; tests cover field-level merge preserving unspecified fields |
| 3 | Installer resolves LS via PATH lookup first, managed download as fallback, helpful error as last resort | VERIFIED | `Resolve()` at installer.go:45 calls `exec.LookPath` first, then checks `AutoInstall && entry.Install != nil`, falls through to error with `InstallHint()` |
| 4 | User can write a memory and read it back by name | VERIFIED | `store.go` has `Write()` and `Read()` with path resolution; store_test.go covers roundtrip |
| 5 | User can list all memories and search by text query | VERIFIED | `store.go` delegates to `index.go` FTS5; `Search()` uses `MATCH` with `ORDER BY rank` |
| 6 | User can rename, edit (append/replace), and delete memories | VERIFIED | `Rename()`, `Edit()`, `Delete()` all implemented in store.go with index updates |
| 7 | Global memories persist in ~/.serena/memories/; project memories in .serena/memories/ | VERIFIED | Path resolution in store.go: `global/` prefix routes to globalDir, else projectDir |
| 8 | SQLite FTS5 index is disposable and rebuilds from markdown files | VERIFIED | `schema.go` has `CREATE VIRTUAL TABLE ... USING fts5`; `index.go` has `Rebuild()` that drops all rows and re-walks dirs |
| 9 | QuirkAdapter is an interface with per-language behavioral hooks | VERIFIED | `type QuirkAdapter interface` in quirks.go with InitOptions, NotificationHandlers, NormalizeSymbolName, PostInitialize |
| 10 | Pool spawns workers using the language registry instead of hardcoded DefaultQuirks | VERIFIED | pool.go imports langregistry, uses `p.registry.Get(wsKey.Language)` and `GetQuirkAdapter(entry)` at spawn time; no DefaultQuirks references remain |
| 11 | Memory MCP tools and workflow skills register via skill interface | VERIFIED | Both `internal/skill/memory/skill.go` and `internal/skill/workflow/skill.go` call `skill.Register()` in `init()` |

**Score:** 11/11 truths verified

### Required Artifacts

| Artifact | Lines | Status | Details |
|----------|-------|--------|---------|
| `internal/langregistry/entry.go` | 72 | VERIFIED | LSEntry, InstallInfo types, InstallHint() |
| `internal/langregistry/languages.go` | 270 | VERIFIED | 52 embedded defaultEntries |
| `internal/langregistry/registry.go` | 163 | VERIFIED | NewRegistry, Get, Languages, ByExtension with YAML merge |
| `internal/langregistry/installer.go` | 229 | VERIFIED | Three-tier Resolve with LookPath, download, error |
| `internal/langregistry/registry_test.go` | 180 | VERIFIED | Tests for defaults count, lookups, extensions, YAML merge |
| `internal/langregistry/installer_test.go` | 83 | VERIFIED | Tests for PATH resolution, disabled auto-install, hints |
| `internal/memory/store.go` | 194 | VERIFIED | Full CRUD: Write, Read, List, Search, Rename, Edit, Delete |
| `internal/memory/index.go` | 305 | VERIFIED | SQLite FTS5 with WAL, Upsert, Search, Rebuild |
| `internal/memory/schema.go` | 40 | VERIFIED | FTS5 virtual table schema |
| `internal/memory/watcher.go` | 189 | VERIFIED | fsnotify with 300ms debounce, recursive dir watching |
| `internal/memory/store_test.go` | 289 | VERIFIED | Store CRUD tests |
| `internal/memory/memory_test.go` | 120 | VERIFIED | Watcher integration tests |
| `internal/skill/skill.go` | 44 | VERIFIED | Skill, ToolProvider, WorkflowProvider interfaces, SkillDeps |
| `internal/skill/registry.go` | 97 | VERIFIED | Register, Get, All, InitAll, Reset |
| `internal/skill/spec.go` | 115 | VERIFIED | ContextSpec, ModeSpec, LoadContextSpecs, LoadModeSpecs, ResolveTools |
| `internal/skill/registry_test.go` | 332 | VERIFIED | 16 tests covering registry and spec operations |
| `internal/skill/memory/skill.go` | 278 | VERIFIED | 7 MCP tools wrapping MemoryStore |
| `internal/skill/memory/skill_test.go` | 223 | VERIFIED | Memory skill tests |
| `internal/skill/workflow/skill.go` | 244 | VERIFIED | onboard_project and prepare_for_new_conversation tools |
| `internal/skill/workflow/prompts.go` | 139 | VERIFIED | Embedded Go templates for onboarding and handoff |
| `internal/skill/workflow/skill_test.go` | 199 | VERIFIED | Workflow skill tests |
| `internal/kernel/lspool/quirks.go` | 228 | VERIFIED | QuirkAdapter interface, 6 adapters, GetQuirkAdapter factory |
| `internal/kernel/lspool/quirks_test.go` | 169 | VERIFIED | Quirk adapter tests |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| langregistry/registry.go | langregistry/languages.go | defaultEntries map | WIRED | Registry loads defaultEntries at init |
| langregistry/installer.go | langregistry/entry.go | Resolve takes LSEntry | WIRED | `func (i *Installer) Resolve(ctx context.Context, entry LSEntry)` |
| memory/store.go | memory/index.go | Store calls Index.Upsert | WIRED | `s.index.Upsert()` in Write and Rename |
| memory/watcher.go | memory/index.go | Watcher triggers Index.Upsert | WIRED | `w.index.Upsert()` in processEvent |
| lspool/pool.go | langregistry/registry.go | Pool uses Registry.Get | WIRED | `p.registry.Get(wsKey.Language)` at line 236 |
| lspool/pool.go | lspool/quirks.go | spawnWorkerLocked calls GetQuirkAdapter | WIRED | `GetQuirkAdapter(entry)` at line 241 |
| skill/memory/skill.go | memory/store.go | MemorySkill wraps MemoryStore | WIRED | `s.store.Write/Read/List/Search/Rename/Edit/Delete` |
| skill/memory/skill.go | skill/registry.go | init() calls skill.Register | WIRED | `skill.Register(&MemorySkill{})` |
| skill/workflow/skill.go | skill/registry.go | init() calls skill.Register | WIRED | `skill.Register(&WorkflowSkill{})` |
| skill/registry.go | skill/skill.go | Registry stores Skill implementations | WIRED | `map[string]Skill` pattern confirmed |
| skill/spec.go | skill/skill.go | Specs reference skill names | WIRED | `Skills []string` in ContextSpec/ModeSpec |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| langregistry builds | `go build ./internal/langregistry/` | Clean | PASS |
| memory builds | `go build ./internal/memory/` | Clean | PASS |
| skill builds | `go build ./internal/skill/...` | Clean | PASS |
| kernel builds | `go build ./internal/kernel/...` | Clean | PASS |
| langregistry tests | `go test ./internal/langregistry/` | ok 0.561s | PASS |
| memory tests | `go test ./internal/memory/` | ok 3.015s | PASS |
| skill tests | `go test ./internal/skill/...` | 3 packages ok | PASS |
| lspool tests | `go test ./internal/kernel/lspool/` | ok 0.494s | PASS |
| go vet all | `go vet ./internal/...` | Clean | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| LNG-01 | 03-01 | Server supports 40+ languages via LSP | SATISFIED | 52 embedded entries in languages.go |
| LNG-02 | 03-01 | Language servers are auto-discovered and downloaded when needed | SATISFIED | Three-tier Resolve: LookPath, download, error |
| LNG-03 | 03-04 | Per-language quirk handling via adapter layer | SATISFIED | QuirkAdapter interface with 6 implementations; pool wired to registry |
| MEM-01 | 03-02 | User can write project-scoped memories | SATISFIED | MemoryStore.Write with project scope path resolution |
| MEM-02 | 03-02 | User can read memories by name | SATISFIED | MemoryStore.Read with auto .md extension |
| MEM-03 | 03-02 | User can list and search stored memories | SATISFIED | FTS5 index with List and Search; watcher auto-reindex |
| MEM-04 | 03-02 | User can rename, edit, and delete memories | SATISFIED | Rename, Edit, Delete in store.go with index sync |
| MEM-05 | 03-02 | Global memories persist across projects; project memories are scoped | SATISFIED | global/ prefix routes to ~/.serena/memories/; else .serena/memories/ |
| WFL-01 | 03-05 | Automated onboarding workflow generates project understanding | SATISFIED | onboard_project tool with template rendering project info |
| WFL-02 | 03-05 | Prepare-for-new-conversation summarizes session state for handoff | SATISFIED | prepare_for_new_conversation tool with handoff template and optional memory save |
| WFL-03 | 03-03 | Plugin/skill pack interface allows extending tools without touching core | SATISFIED | Skill/ToolProvider/WorkflowProvider interfaces; init()-based registration; ResolveTools composition |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | - | - | No TODO, FIXME, PLACEHOLDER, or stub patterns found |

### Human Verification Required

### 1. Language Server Auto-Download

**Test:** Configure a project with a language whose LS is not in PATH, verify download triggers
**Expected:** Installer detects missing binary, downloads via appropriate package manager, LS starts successfully
**Why human:** Requires external package manager (npm/pip/cargo) and network access

### 2. Memory Watcher in Real Project

**Test:** Start server with a project, manually edit a .md file in .serena/memories/, verify search picks it up
**Expected:** File change detected within ~300ms debounce, index updated, search returns new content
**Why human:** Requires running server process and real filesystem events

### 3. Onboarding Workflow End-to-End

**Test:** Run onboard_project on a real multi-language project
**Expected:** Detects languages, counts files, returns meaningful onboarding instructions
**Why human:** Output quality assessment requires human judgment

### Gaps Summary

No gaps found. All 11 requirements are satisfied with verified implementations. All artifacts exist, are substantive (no stubs), and are properly wired. All tests pass. No anti-patterns detected.

---

_Verified: 2026-04-07_
_Verifier: Claude (gsd-verifier)_
