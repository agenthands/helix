# Phase 14: Documentation - Research

**Researched:** 2026-04-10
**Domain:** Go codegen tooling, markdown documentation, marker-comment table generation
**Confidence:** HIGH

## Summary

Phase 14 is a documentation-only phase that ships three files (README.md, USAGE.md, CHANGELOG.md) plus a Go codegen tool (`cmd/docgen`) that auto-generates tool and language tables from the existing Go registries. The codebase already has a well-structured README.md with most content in place -- the work is evolutionary (fill gaps, swap static lists for generated ones, add USAGE.md and CHANGELOG.md).

The codegen approach is straightforward: the skill system's `Tools()` methods and `langregistry.defaultEntries` map are the canonical data sources. The codegen tool imports these packages directly, iterates their registrations, and outputs markdown tables that are injected between marker comments in README.md. This pattern already exists in `cmd/lspgen` for LSP type generation.

The documentation content itself requires synthesizing information from the config structs (`internal/config/config.go`), profile YAMLs (`internal/profile/profiles/*.yaml`), observability package (`internal/obs/`), and degradation package (`internal/degrade/`). All source material is in the codebase; no external research is needed.

**Primary recommendation:** Build `cmd/docgen` as a standalone Go binary that imports the skill and langregistry packages, outputs markdown tables, and replaces content between marker comments in README.md. Add a `make docs` target. Write USAGE.md and CHANGELOG.md as new files.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Evolve existing README.md rather than rewriting from scratch. Fill gaps required by DOC-01 through DOC-04 -- add missing client configs (Codex, IDE assistants), swap static tool/language lists with auto-generated ones.
- **D-02:** Remove the "Legacy Python Version" section entirely.
- **D-03:** USAGE.md: hybrid style -- start with 2-3 quick tutorials, then transition to reference sections.
- **D-04:** Observability quickstart (DOC-08) and performance tuning (DOC-09) live as sections within USAGE.md, not separate files.
- **D-05:** CHANGELOG: start fresh for Go rewrite. Replace current CHANGELOG.md with v1.0, v1.1, v1.2 entries only.
- **D-06:** Version format: `v1.0` / `v1.1` / `v1.2`.
- **D-07:** Build Go codegen tool (`cmd/docgen` or `internal/docgen`) that reads langregistry and tool registrations to output markdown tables. Run via `go generate` or `make docs`.
- **D-08:** Use marker comments (`<!-- BEGIN TOOLS -->` / `<!-- END TOOLS -->`, `<!-- BEGIN LANGUAGES -->` / `<!-- END LANGUAGES -->`). Codegen replaces content between markers.

### Claude's Discretion
- Exact README section ordering and prose beyond DOC-01 through DOC-04
- USAGE.md tutorial scenario selection (which 2-3 scenarios)
- CHANGELOG entry detail level
- Codegen tool internal architecture (how it discovers tools and languages)

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DOC-01 | README.md with project pitch, install instructions, capabilities overview | Existing README has pitch + install + capabilities. Evolve per D-01 |
| DOC-02 | README.md includes full 38-tool table (auto-generated from registry) | Codegen reads skill `Tools()` methods. All 7 skill adapters return `[]*mcp.ToolDef` with name+description |
| DOC-03 | README.md includes 52-language table with LS install commands | Codegen reads `langregistry.defaultEntries` map. `LSEntry` has Language, Command, FileExts, `InstallHint()` |
| DOC-04 | README.md includes client configs for Claude Code, Codex, IDE assistants | README already has Claude Code config. Add Codex and IDE assistant configs |
| DOC-05 | USAGE.md with profile/mode reference and config precedence | Profile YAMLs in `internal/profile/profiles/*.yaml`, config struct in `internal/config/config.go` |
| DOC-06 | USAGE.md with common workflow examples | Tutorial scenarios from skill workflow tools (onboard, handoff) |
| DOC-07 | USAGE.md with troubleshooting guide | Common issues: LS not found (three-tier installer), cache stale, mode restrictions |
| DOC-08 | USAGE.md with observability quickstart | Config fields: `admin_addr`, `enable_pprof`, `tracing_endpoint`, `tracing_sample_ratio` |
| DOC-09 | USAGE.md with performance tuning guide | Config fields: `worker_pool.*`, `degradation.*`, `memory_limit_mb` |
| DOC-10 | CHANGELOG.md with v1.0, v1.1, v1.2 entries | Milestone audits in `.planning/milestones/`, ROADMAP.md phase summaries |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Always run `go vet` and `go test` before completing any Go task** -- applies to the `cmd/docgen` tool
- Build command: `go build ./cmd/serena`
- Test command: `go test ./...`
- Format command: `gofmt -w .`
- Project is Go-native, single binary. The `cmd/docgen` tool is a separate binary (not bundled into `cmd/serena`)

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `os`, `text/template`, `strings`, `sort`, `regexp` | Go 1.25.1 | Codegen tool file I/O, template rendering, marker replacement | No external deps needed for a codegen tool [VERIFIED: go.mod] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/langregistry` | in-tree | Language table data source | Codegen imports directly to iterate `defaultEntries` [VERIFIED: codebase] |
| `internal/skill` + skill packages | in-tree | Tool table data source | Codegen calls `skill.Register` flow then `skill.ToolProviders()` to enumerate [VERIFIED: codebase] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Direct Go import of registries | AST parsing of source files | AST is fragile, direct import is type-safe and always matches runtime |
| `text/template` | `html/template` | Markdown output, no HTML escaping needed |

**Installation:**
No external dependencies. All code is in-tree.

## Architecture Patterns

### Codegen Tool Structure
```
cmd/
  docgen/
    main.go          # CLI entry, reads README.md, calls generators, writes output
```

The tool follows the existing `cmd/lspgen` pattern: a standalone Go binary in `cmd/` that imports internal packages and generates output. [VERIFIED: cmd/lspgen/ exists with same pattern]

### Pattern 1: Marker-Comment Replacement
**What:** README.md contains HTML comment markers. The codegen tool reads the file, finds marker pairs, replaces content between them, and writes back.
**When to use:** For auto-generated tables (tools, languages) that must stay in sync with code.
**Example:**
```go
// [ASSUMED] Standard pattern for marker replacement
func replaceMarkerContent(content, beginMarker, endMarker, replacement string) string {
    re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(beginMarker) + `.*?` + regexp.QuoteMeta(endMarker))
    return re.ReplaceAllString(content, beginMarker + "\n" + replacement + "\n" + endMarker)
}
```

### Pattern 2: Tool Discovery via Skill System
**What:** The codegen tool initializes skills with minimal deps, then iterates `skill.ToolProviders()` to collect all tool definitions.
**When to use:** For generating the tool table.
**Constraint:** Skill `Init()` requires `SkillDeps` -- the codegen tool must either:
- (a) Import skill packages for their `init()` side effects (registers skills), then call `Tools()` on each skill adapter without calling `Init()` (since adapters are no-ops for kernel skills), OR
- (b) Hard-code the tool list in the codegen tool (fragile, defeats purpose)

**Recommended approach (a):** The kernel skill adapters (`symbols/skill.go`, `edit/skill.go`, `fileops/skill.go`, `diag/skill_adapter.go`) all have no-op `Init()` and return static `[]*mcp.ToolDef` from `Tools()`. The memory and workflow skills require real `Init()` with deps, but their tool lists are also static in the `Tools()` method. The codegen tool can:
1. Import all skill packages for `init()` registration
2. Call `skill.AllSkills()` or iterate registered providers
3. Call `Tools()` on each -- for kernel adapters this is safe without `Init()`
4. For memory/workflow, the `Tools()` method constructs `ToolDef` slices that don't depend on `Init()` state

[VERIFIED: All skill.go files return static ToolDef slices from Tools() that don't reference any initialized state]

### Pattern 3: Language Registry Iteration
**What:** The codegen imports `langregistry` and iterates `defaultEntries` to build the language table.
**Constraint:** `defaultEntries` is unexported (`var defaultEntries`). The codegen tool needs either:
- (a) An exported accessor function, OR
- (b) Using the exported `Registry` type's methods

[VERIFIED: `defaultEntries` is unexported in languages.go]

The `Registry` type needs to be checked for exported iteration methods.

```go
// Check: does Registry expose iteration?
// If not, add a func like DefaultEntries() map[string]LSEntry
```

### Anti-Patterns to Avoid
- **Parsing Go source with regex to extract tool names:** Fragile, breaks on formatting changes. Use Go imports.
- **Duplicating tool/language data in the docgen tool:** Defeats the purpose of codegen. Always read from the canonical source.
- **Running the full daemon to enumerate tools:** Overkill. The skill adapters return static data.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Markdown table generation | Custom string concatenation | `text/tabwriter` or simple `fmt.Sprintf` with pipe formatting | Alignment and edge cases |
| File marker replacement | Manual string splitting | `regexp` with `(?s)` dotall flag | Handles newlines, robust |
| CHANGELOG formatting | Custom date formatting | `time.Format("2006-01-02")` | Standard Go pattern |

## Common Pitfalls

### Pitfall 1: Unexported Registry Data
**What goes wrong:** `defaultEntries` in `langregistry/languages.go` is unexported. Codegen tool cannot access it directly.
**Why it happens:** The registry was designed for internal use by the pool, not external enumeration.
**How to avoid:** Add an exported function like `DefaultEntries() map[string]LSEntry` or `Languages() []LSEntry` to the langregistry package. This is a minimal, non-breaking addition.
**Warning signs:** Compile error when codegen tries to access `defaultEntries`.

### Pitfall 2: Skill Init Dependencies
**What goes wrong:** Calling `skill.Init()` on memory/workflow skills fails because they need real file paths and databases.
**Why it happens:** Skills expect `SkillDeps` with valid directories.
**How to avoid:** Don't call `Init()` in the codegen tool. The `Tools()` methods on all skill adapters return static `[]*mcp.ToolDef` slices that don't depend on initialized state. Just import for `init()` side effects and call `Tools()` directly.
**Warning signs:** "memory skill init" or "no such file or directory" errors.

### Pitfall 3: Tool Count Mismatch
**What goes wrong:** The generated table has a different number of tools than the "38+" claim in README.
**Why it happens:** Profile skill adds 2 tools (switch_mode, get_token_budget) that are easy to forget. Also `verify_edit` in the edit skill brings the total to exactly 38 when counting: 9 (symbols) + 6 (edit) + 6 (fileops) + 3 (diag) + 7 (memory) + 2 (workflow) + 2 (profile) = 35. There may be additional tools.
**How to avoid:** Let the codegen tool count dynamically. Don't hard-code "38" in README.
**Warning signs:** Static tool count that doesn't match generated table length.

### Pitfall 4: Marker Comments Not Preserved
**What goes wrong:** The generated README loses the marker comments, making subsequent runs fail.
**Why it happens:** Replacement logic consumes the markers instead of preserving them.
**How to avoid:** The replacement must include both the BEGIN and END markers in the output, with generated content between them.
**Warning signs:** Running docgen twice produces an error or empty table.

### Pitfall 5: Language Key vs Display Name
**What goes wrong:** Language table shows internal keys like "python_jedi" or "cpp_ccls" instead of human-readable names.
**Why it happens:** `LSEntry.Language` is the registry key, not a display name.
**How to avoid:** Map language keys to display names in the codegen tool, or group variants under their primary language. E.g., "python_jedi" -> "Python (Jedi)", "cpp_ccls" -> "C/C++ (ccls)".
**Warning signs:** Confusing language names in the generated table.

### Pitfall 6: CHANGELOG Source Material Missing
**What goes wrong:** v1.0 and v1.1 entries lack sufficient detail because milestone audits don't contain prose summaries.
**Why it happens:** Planning artifacts are structured for tracking, not user-facing documentation.
**How to avoid:** Use ROADMAP.md phase descriptions + requirement lists as the source. The ROADMAP has phase goals, success criteria, and plan summaries for all completed phases.
**Warning signs:** Vague changelog entries like "Phase 1 complete".

## Code Examples

### Marker Replacement Pattern
```go
// [ASSUMED] Standard approach for marker-based content injection
func replaceSection(content, beginMarker, endMarker, newContent string) (string, error) {
    begin := strings.Index(content, beginMarker)
    end := strings.Index(content, endMarker)
    if begin == -1 || end == -1 {
        return "", fmt.Errorf("markers not found: %s / %s", beginMarker, endMarker)
    }
    endOfEndMarker := end + len(endMarker)
    return content[:begin] + beginMarker + "\n" + newContent + "\n" + endMarker + content[endOfEndMarker:], nil
}
```

### Tool Table Generation
```go
// [VERIFIED: skill adapter pattern from internal/kernel/symbols/skill.go]
// Import all skill packages for init() side effects
import (
    _ "github.com/postfix/serena/internal/kernel/symbols"
    _ "github.com/postfix/serena/internal/kernel/edit"
    _ "github.com/postfix/serena/internal/kernel/fileops"
    _ "github.com/postfix/serena/internal/kernel/diag"
    _ "github.com/postfix/serena/internal/skill/memory"
    _ "github.com/postfix/serena/internal/skill/workflow"
    _ "github.com/postfix/serena/internal/profile"
)

// Then iterate registered tool providers
for _, tp := range skill.ToolProviders() {
    for _, tool := range tp.Tools() {
        // tool.Name, tool.Description
    }
}
```

### Language Table Generation
```go
// [VERIFIED: langregistry/languages.go exports defaultEntries as unexported var]
// Need to add an exported accessor, then:
for _, entry := range langregistry.DefaultEntries() {
    // entry.Language, entry.Command, entry.FileExts, entry.InstallHint()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Static tool list in README | Auto-generated from skill registry | Phase 14 (now) | Tables cannot drift from code |
| Legacy Python CHANGELOG | Go-only CHANGELOG starting v1.0 | Phase 14 (now) | Clean break from Python history |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `skill.ToolProviders()` or equivalent exported function exists to iterate all registered skills | Architecture Patterns | Would need to add an exported accessor to the skill package |
| A2 | Memory and workflow skill `Tools()` methods work without calling `Init()` first | Architecture Patterns | Would need to extract static tool lists separately |
| A3 | The existing Makefile `docs` target does not exist yet | Architecture Patterns | Would need to update rather than add |

## Open Questions

1. **Does langregistry export an iteration function?**
   - What we know: `defaultEntries` is unexported. `Registry` type exists but may not expose iteration.
   - What's unclear: Whether there's already an exported accessor.
   - Recommendation: Check `Registry` methods. If none, add `DefaultEntries() map[string]LSEntry` -- minimal change.

2. **Does the skill package export a function to list all registered providers?**
   - What we know: `skill.Register()` stores skills. `skill.Get()` retrieves by name. `skill.ToolProviders()` is referenced in CLAUDE.md.
   - What's unclear: Exact exported API for iterating all providers.
   - Recommendation: Check `internal/skill/` exports. If `ToolProviders()` exists, use it. If not, add it.

3. **Exact tool count**
   - What we know: README says "38+". Counting skill adapters: 9+6+6+3+7+2+2 = 35. The `verify_edit` is included in the 6 edit tools. Some tools in README (like `find_files`, `search_in_files`) have slightly different names from skill adapters (`find_files` vs actual tool name).
   - Recommendation: Let codegen determine the authoritative count. Update README text dynamically.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (built-in) |
| Config file | none |
| Quick run command | `go test ./cmd/docgen/...` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DOC-02 | Tool table generated with correct count | unit | `go test ./cmd/docgen/... -run TestToolTable` | Wave 0 |
| DOC-03 | Language table generated with correct count | unit | `go test ./cmd/docgen/... -run TestLanguageTable` | Wave 0 |
| DOC-02/03 | Marker replacement preserves markers | unit | `go test ./cmd/docgen/... -run TestMarkerReplace` | Wave 0 |
| DOC-01-10 | Generated README/USAGE/CHANGELOG are valid markdown | smoke | `go build ./cmd/docgen && ./docgen --check` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./cmd/docgen/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go vet ./... && go test ./...`

### Wave 0 Gaps
- [ ] `cmd/docgen/main_test.go` -- covers marker replacement, table generation
- [ ] Exported accessor in `langregistry` for iterating entries (if missing)
- [ ] Exported accessor in `skill` for iterating all providers (if missing)

## Security Domain

Security enforcement is not directly applicable to this documentation phase. The codegen tool reads source code and writes markdown -- no user input, no network, no secrets.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | no | Codegen reads trusted in-tree data only |
| V6 Cryptography | no | N/A |

## Sources

### Primary (HIGH confidence)
- Codebase inspection: `internal/langregistry/languages.go`, `entry.go` -- language registry structure [VERIFIED]
- Codebase inspection: `internal/kernel/*/skill.go`, `internal/skill/memory/skill.go`, `internal/skill/workflow/skill.go`, `internal/profile/skill.go` -- all skill adapters and tool definitions [VERIFIED]
- Codebase inspection: `internal/config/config.go` -- full config schema for USAGE.md content [VERIFIED]
- Codebase inspection: `internal/profile/profiles/*.yaml` -- 5 profile definitions [VERIFIED]
- Codebase inspection: `README.md` -- current state, 170 lines, well-structured [VERIFIED]
- Codebase inspection: `CHANGELOG.md` -- current state, legacy Python history to replace [VERIFIED]
- Codebase inspection: `cmd/lspgen/` -- existing codegen tool pattern [VERIFIED]
- Codebase inspection: `Makefile` -- existing targets, no `docs` target yet [VERIFIED]

### Secondary (MEDIUM confidence)
- `.planning/ROADMAP.md` -- phase descriptions and milestone summaries for CHANGELOG content [VERIFIED]
- `.planning/milestones/` -- v1.0 and v1.1 milestone audit files [VERIFIED: files exist]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all code is in-tree Go stdlib, no external deps needed
- Architecture: HIGH -- skill adapter pattern and langregistry structure are fully verified
- Pitfalls: HIGH -- identified from direct code inspection of exported/unexported boundaries

**Research date:** 2026-04-10
**Valid until:** 2026-05-10 (stable -- documentation phase with no external dependencies)
