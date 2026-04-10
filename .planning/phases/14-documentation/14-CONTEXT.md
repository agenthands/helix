# Phase 14: Documentation - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Ship the user-facing docs (README.md, USAGE.md, CHANGELOG.md) so external users can install, configure, operate, and troubleshoot Serena without reading source. Includes a Go codegen tool for auto-generating tool and language tables from the registries.

</domain>

<decisions>
## Implementation Decisions

### README Strategy
- **D-01:** Evolve the existing README.md rather than rewriting from scratch. The current README already has solid content (tool lists, install instructions, profiles, architecture overview). Fill gaps required by DOC-01 through DOC-04 — add missing client configs (Codex, IDE assistants), swap static tool/language lists with auto-generated ones.
- **D-02:** Remove the "Legacy Python Version" section entirely. The Go rewrite is the product now; legacy history lives in the `legacy/` directory and git.

### USAGE.md Structure
- **D-03:** Hybrid style — start with 2-3 quick tutorials (onboarding a new project, refactoring workflow, code review workflow), then transition to reference sections (profiles, modes, config precedence, troubleshooting).
- **D-04:** Observability quickstart (DOC-08) and performance tuning (DOC-09) live as sections within USAGE.md, not separate files. USAGE.md is the single operational reference.

### CHANGELOG Approach
- **D-05:** Start fresh for the Go rewrite. Replace the current CHANGELOG.md (which has legacy Python history 0.1.3 through 1.0.0) with entries for v1.0, v1.1, and v1.2 only. Legacy history is preserved in git and `legacy/` directory.
- **D-06:** Version format: `v1.0` / `v1.1` / `v1.2` — matches milestone naming in the roadmap.

### Table Generation Tooling
- **D-07:** Build a Go codegen tool (e.g., `cmd/docgen` or `internal/docgen`) that reads the language registry (`internal/langregistry/languages.go`) and tool registrations (skill/kernel packages) to output markdown tables. Run via `go generate` or `make docs`.
- **D-08:** Use marker comments in README.md (`<!-- BEGIN TOOLS -->` / `<!-- END TOOLS -->`, `<!-- BEGIN LANGUAGES -->` / `<!-- END LANGUAGES -->`). The codegen tool replaces content between markers. README stays as the single source of truth.

### Claude's Discretion
- Exact README section ordering and prose beyond what DOC-01 through DOC-04 require
- USAGE.md tutorial scenario selection (which 2-3 scenarios best demonstrate Serena's value)
- CHANGELOG entry detail level (bullet points vs paragraphs, level of technical detail)
- Codegen tool internal architecture (how it discovers tools and languages from the Go source)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Documentation
- `README.md` — Current README to evolve (DOC-01 through DOC-04 baseline)
- `CHANGELOG.md` — Current legacy CHANGELOG to replace (DOC-10 baseline)
- `CLAUDE.md` — Project instructions, architecture overview, build/test commands

### Registries (codegen sources)
- `internal/langregistry/languages.go` — 52-language embedded registry (codegen source for language table)
- `internal/langregistry/entry.go` — Language entry struct definition
- `internal/profile/embed.go` — Embedded profile/mode YAML files via `//go:embed`
- `internal/profile/profiles/*.yaml` — Profile definitions (claude-code, codex, ide-assistant, ci-bot, full)
- `internal/profile/modes/*.yaml` — Mode definitions (read, edit, review, admin)

### Tool Registration (codegen sources)
- `internal/kernel/symbols/` — 9 symbol retrieval tools
- `internal/kernel/edit/` — 6 symbol editing tools
- `internal/kernel/fileops/` — 6 file operation tools
- `internal/kernel/diag/` — 3 diagnostic tools
- `internal/skill/memory/skill.go` — 7 memory tools
- `internal/skill/workflow/skill.go` — Workflow tools (onboarding, handoff)
- `internal/profile/skill.go` — Profile tools (switch_mode, get_token_budget)

### Observability & Degradation (for USAGE.md sections)
- `internal/obs/` — Observability package (metrics, tracing, admin listener)
- `internal/degrade/` — Degradation package (timeout budgets, circuit breaker)
- `internal/config/` — Configuration system (4-layer precedence)

### Planning Artifacts
- `.planning/ROADMAP.md` — Phase 14 requirements and success criteria
- `.planning/REQUIREMENTS.md` — DOC-01 through DOC-10 definitions
- `.planning/milestones/` — Milestone summaries for CHANGELOG references

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `README.md` — Already has tool categories, install section, profile examples, architecture diagram. Evolve, don't rewrite.
- `internal/langregistry/languages.go` — `defaultEntries` slice contains all 52 language definitions with names, LS commands, file extensions. Direct codegen source.
- `internal/profile/embed.go` — Embedded `profiles/*.yaml` and `modes/*.yaml` provide profile/mode data for documentation.

### Established Patterns
- Tool registration follows two patterns: kernel tools via `RegisterTools()` functions, skill tools via `ToolProvider.Tools()` returning `[]*mcp.ToolDef`. Codegen must handle both.
- Profile YAMLs define tool subsets per profile — useful for documenting which tools each profile exposes.

### Integration Points
- `Makefile` — Add `docs` target for running codegen
- `cmd/serena/main.go` — CLI entry point, documents available flags
- `go.mod` — Module path for `go install` command in README

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 14-documentation*
*Context gathered: 2026-04-10*
