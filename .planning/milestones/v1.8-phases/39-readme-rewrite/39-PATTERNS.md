# Phase 39: README Rewrite - Pattern Map

**Mapped:** 2026-04-23
**Files analyzed:** 2
**Analogs found:** 2 / 2

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `README.md` | config (documentation) | N/A | `README.md` (current, self-rewrite) + `USAGE.md` (style reference) | exact |
| `cmd/docgen/main.go` | utility (build tooling) | transform | `cmd/docgen/main.go` (self, add imports) + `internal/daemon/imports.go` (complete import list) | exact |

## Pattern Assignments

### `README.md` (documentation, restructure)

**Analog:** `README.md` (current version, 278 lines) -- rewrite in place

**Hero/header pattern** (lines 1-8):
```html
<p align="center" style="text-align:center;">
  <img src="resources/serena-logo.svg#gh-light-mode-only" style="width:500px">
  <img src="resources/serena-logo-dark-mode.svg#gh-dark-mode-only" style="width:500px">
</p>

<h3 align="center">
    Serena is the IDE for your coding agent.
</h3>
```
**Retain exactly** per D-01. Add subtitle line after closing `</h3>` per D-02.

**Auto-gen marker pattern** (lines 83-139 for languages, 145-184 for tools):
```html
<!-- BEGIN LANGUAGES -->
[auto-generated content -- DO NOT hand-edit]
<!-- END LANGUAGES -->
```
```html
<!-- BEGIN TOOLS -->
[auto-generated content -- DO NOT hand-edit]
<!-- END TOOLS -->
```
**Critical:** Preserve these markers exactly. Content between them is replaced by `go run ./cmd/docgen`.

**Advantages table pattern** (lines 34-41):
```markdown
| | File-based tools | Serena |
|---|---|---|
| **Navigation** | grep, find, read whole files | Go to definition, find references, symbol search, call hierarchy |
```
**Retain** -- this table is hand-authored and stays.

**Architecture section pattern** (lines 257-273):
```markdown
## Architecture

Serena is built as a 4-layer Go binary:

` ` `
MCP Runtime (stdio/HTTP transports, tool registry, profile middleware)
    |
Code Intelligence Kernel (LS worker pool, symbol ops, file ops, diagnostics)
    |
Skills & Multi-Language (52-language registry, memory system, skill plugins)
    |
Agent Profiles (5 profiles, 4 modes, token budget, layered config)
` ` `
```
**Retain brief** per D-10. No expansion.

**Collapsed details pattern** (from RESEARCH.md, for manual configs per D-11):
```html
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

</details>
```

**Style reference from USAGE.md** (lines 1-5):
```markdown
# Serena Usage Guide

This guide covers operational usage of Serena: tutorials for common workflows, profile and mode reference, configuration, troubleshooting, observability, and performance tuning.

For installation and feature overview, see [README.md](README.md).
```
Use same product-aware technical tone. Cross-link pattern: `[README.md](README.md)` / `[USAGE.md](USAGE.md)` / `[INSTALL.md](INSTALL.md)`.

---

### `cmd/docgen/main.go` (utility, transform -- add missing imports)

**Analog:** `internal/daemon/imports.go` (authoritative complete import list)

**Current docgen imports** (lines 22-28):
```go
	// Blank imports trigger skill.Register() via init() (same as daemon/imports.go).
	_ "github.com/postfix/serena/internal/kernel/diag"
	_ "github.com/postfix/serena/internal/kernel/edit"
	_ "github.com/postfix/serena/internal/kernel/fileops"
	_ "github.com/postfix/serena/internal/kernel/symbols"
	_ "github.com/postfix/serena/internal/profile"
	_ "github.com/postfix/serena/internal/skill/memory"
	_ "github.com/postfix/serena/internal/skill/workflow"
```

**Daemon imports.go** (lines 4-12, the complete authoritative list):
```go
	// Blank imports trigger skill.Register() via init() (Caddy-style).
	_ "github.com/postfix/serena/internal/kernel/diag"
	_ "github.com/postfix/serena/internal/kernel/edit"
	_ "github.com/postfix/serena/internal/kernel/fileops"
	_ "github.com/postfix/serena/internal/kernel/symbols"
	_ "github.com/postfix/serena/internal/profile"
	_ "github.com/postfix/serena/internal/skill/memory"
	_ "github.com/postfix/serena/internal/skill/repomap"
	_ "github.com/postfix/serena/internal/skill/workflow"
```

**Missing imports to add to docgen** (verified against source packages):
```go
	_ "github.com/postfix/serena/internal/skill/repomap"   // adds: get_repo_map, get_context
	_ "github.com/postfix/serena/internal/kernel/health"    // adds: get_health
	_ "github.com/postfix/serena/internal/kernel/help"      // adds: get_tool_help
```

**Import ordering pattern:** Blank skill imports are grouped alphabetically within the import block, `kernel/*` before `profile` before `skill/*`. Add `health` and `help` after `fileops`, add `repomap` after `memory`.

**Note:** `internal/daemon/imports.go` itself is also missing `health` and `help` -- but that is outside this phase's scope (daemon registers those tools through a different mechanism). Only docgen needs the fix for table generation.

---

## Shared Patterns

### Auto-Generated Content Markers
**Source:** `README.md` lines 83, 139, 145, 184
**Apply to:** README.md rewrite -- preserve markers exactly on their own lines
```html
<!-- BEGIN LANGUAGES -->
<!-- END LANGUAGES -->
<!-- BEGIN TOOLS -->
<!-- END TOOLS -->
```

### Product Identity Language
**Source:** D-01, D-04, LEGC-01 constraints
**Apply to:** All README.md content
- Keep: "Serena is the IDE for your coding agent"
- Footer only: "Originally inspired by Python Serena"
- Never use: "port", "rewrite", "based on", "derived from"

### Tool Count Reference
**Source:** RESEARCH.md verified tool inventory
**Apply to:** README.md prose (How Serena Works section, subtitle)
- Exact count after docgen fix: **40 tools**
- Prose recommendation: "40+" to future-proof
- Auto-generated table will show exact rows

### Quick Start CLI Reference
**Source:** `internal/cli/setup.go` ValidArgs and Flags
**Apply to:** README.md Quick Start section
- Valid clients: `claude-code`, `vscode`, `jetbrains`, `claude-desktop`, `gemini-cli`, `generic`
- Key flags: `--global`, `--uninstall`, `--skip-install`, `--dry-run`, `--output`, `--no-hooks`

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | -- | -- | Both files have exact analogs (self-rewrite pattern) |

## Metadata

**Analog search scope:** repository root, `cmd/docgen/`, `internal/daemon/`, `USAGE.md`
**Files scanned:** 6 (README.md, USAGE.md, cmd/docgen/main.go, internal/daemon/imports.go, internal/kernel/health/, internal/kernel/help/)
**Pattern extraction date:** 2026-04-23
