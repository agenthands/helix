# Phase 40: Usage Refresh - Pattern Map

**Mapped:** 2026-04-23
**Files analyzed:** 1 (USAGE.md -- single file, three modification zones)
**Analogs found:** 1 / 1 (file is its own analog)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `USAGE.md` (Zone A: Tutorial 1 update) | documentation | N/A | `USAGE.md` lines 9-52 (Tutorial 1) | exact |
| `USAGE.md` (Zone B: new Feature Guide section) | documentation | N/A | `USAGE.md` lines 144-213 (Profiles and Modes) | exact |
| `USAGE.md` (Zone C: new troubleshooting entries) | documentation | N/A | `USAGE.md` lines 420-436 (rust-analyzer entry) | exact |

## Pattern Assignments

### Zone A: Tutorial 1 Update (lines 19-37)

**Analog:** `USAGE.md` Tutorial 1 Steps (lines 9-52)

**Tutorial step pattern** (lines 13-17):
```markdown
**Step 1: Install Serena**

```bash
go install github.com/postfix/serena/cmd/serena@latest
```
```

Step 2 (lines 19-37) currently shows manual JSON config. Replace with `serena setup <client>` following the same `**Step N: Action**` + description + code block pattern.

**Replacement content source:** `internal/cli/setup.go` -- the `serena setup <client>` command. Supported clients: `claude-code`, `vscode`, `jetbrains`, `claude-desktop`, `gemini-cli`, `generic`. Key flags: `--global`, `--uninstall`, `--dry-run`.

**Key constraint:** Must preserve the HTTP mode alternative (lines 34-37) as a secondary option, since `serena setup` handles stdio registration but HTTP mode is still a valid path for IDEs/multi-client.

---

### Zone B: Feature Guide Section (insert after line 143, before line 144)

**Analog:** `USAGE.md` "Profiles and Modes" section (lines 144-213) -- similar reference-style section with subsections.

**Section heading pattern** (line 144):
```markdown
## Profiles and Modes
```
New section uses: `## Feature Guide`

**Subsection heading pattern** (line 146):
```markdown
### Profiles
```
Each feature group uses: `### Feature Name`

**Feature subsection template** (derived from D-04/D-05 decisions):
```markdown
### Feature Name

[1-2 paragraphs: what it does and why it matters for the user]

[Concrete example: tool call, CLI command, or configuration snippet]
```

**Subsections to create (7 total, per D-02):**

1. **Fuzzy Editing** -- source: `internal/fuzzy/match.go` (4-strategy cascade), `internal/fuzzy/ellipsis.go` (ellipsis support). Tool: `fuzzy_edit`. List strategies (Exact, Whitespace, IndentFlex, Failed) per D-06 without deep explanation.

2. **RepoMap and Context** -- source: `internal/skill/repomap/skill.go`. Tools: `get_repo_map` (token budget default 4096), `get_context` (budget default 2048). Mention PageRank ranking and tree-sitter tag extraction.

3. **Setup CLI** -- source: `internal/cli/setup.go`, `setup_hooks.go`. Command: `serena setup <client>`. Include hooks sub-feature (SessionStart, PreToolUse, Stop) for Claude Code. Mention `--no-hooks` flag.

4. **Smart Errors** -- source: `internal/mcp/suggest_lev.go`. Levenshtein "Did you mean?" for misspelled parameters and incorrect enum values. No tool call needed -- automatic middleware behavior.

5. **Progressive Descriptions** -- source: `internal/mcp/registry.go`. Two-tier: brief in `tools/list`, full via `get_tool_help`. Tool: `get_tool_help`.

6. **Lazy Workspace Init** -- source: `internal/mcp/lazy_init.go`. Transparent activation on first `tools/call`. No tool call needed -- automatic middleware behavior.

7. **Health Monitoring** -- source: `internal/kernel/health/tools.go`. Tool: `get_health` with `verbose` parameter.

**Table pattern** (from Profiles section, lines 150-157 -- use if listing items):
```markdown
| Profile | Default Mode | Description | Skills |
|---------|-------------|-------------|--------|
| `claude-code` | edit | Curated for Claude Code CLI. ... | symbol-retrieval, ... |
```

**Config cross-reference pattern** (derived from existing document structure):
```markdown
See [Configuration Reference](#configuration-reference) for `degradation.timeout_index` and other tuning keys.
```

---

### Zone C: Troubleshooting Entries (insert after line 436, before line 438)

**Analog:** `USAGE.md` rust-analyzer troubleshooting entry (lines 420-436)

**Troubleshooting entry pattern** (lines 420-436):
```markdown
### rust-analyzer rename fails in temp/fresh workspaces

**Symptom:** `rename_symbol` returns `"internal: rename (No references found at position)"` when targeting a Rust symbol, even though `get_hover_info`, `find_references`, and `search_symbols` all work correctly at the same position.

**Cause:** rust-analyzer (tested with v1.90) has a known limitation where `textDocument/rename` and `textDocument/prepareRename` return "No references found at position" in freshly-opened workspaces. This occurs despite:

- `textDocument/didOpen` being sent for all `.rs` files
- `workspace/symbol` successfully finding the symbol
...

**Workaround:** Use `replace_symbol_body` (tree-sitter-based) instead of `rename_symbol` (LSP-based) for Rust refactoring.
```

**Entry 1: jdtls Cold-Start Delay** (per D-11)
- **Symptom/Cause/Fix structure** matching the analog
- Source: `test/integration/java_test.go` line 19 (skip message), `internal/kernel/lspool/quirks.go` (JdtlsAdapter)
- Workaround: increase `degradation.timeout_index` to 300s
- Cross-reference config key `degradation.timeout_index` (USAGE.md line 280)

**Entry 2: gopls/Go 1.25 Benchmark Constraint** (per D-12)
- **Symptom/Cause/Fix structure** matching the analog
- Source: `.planning/RETROSPECTIVE.md` line 25, `go.mod` line 3
- Workaround: ensure gopls version compatibility with Go version, mention v0.17.1 specific issue

---

## Shared Patterns

### Markdown Heading Hierarchy
**Source:** `USAGE.md` entire document
**Apply to:** All zones
- `##` for top-level sections (Feature Guide, Troubleshooting)
- `###` for subsections (individual features, individual troubleshooting entries)
- `####` never used in tutorials or troubleshooting -- only in Configuration Reference for settings groups

### Code Block Conventions
**Source:** `USAGE.md` lines 15-16, 23-31, 85-86
**Apply to:** All zones
```markdown
```bash
serena setup claude-code
```

```
get_repo_map()
```
```
- CLI commands use `bash` fence
- Tool calls use unfenced or plain fence (no language tag) -- see lines 60-61, 68-69, 85-86
- YAML config uses `yaml` fence -- see lines 289-328
- JSON uses `json` fence -- see lines 23-31

### Bold Label Pattern
**Source:** `USAGE.md` lines 19, 40, 50
**Apply to:** Tutorial updates, troubleshooting entries
- Tutorial steps: `**Step N: Action**`
- Troubleshooting fields: `**Symptom:**`, `**Cause:**`, `**Fix:**`, `**Workaround:**`

### Internal Link Pattern
**Source:** `USAGE.md` line 418
**Apply to:** Feature Guide cross-references
```markdown
Check the [Profiles](#profiles) section for allowed transitions.
```

## No Analog Found

No files without analogs -- this phase modifies a single existing file using its own established patterns.

## Metadata

**Analog search scope:** `USAGE.md` (self-referential -- documentation update to existing file)
**Source files consulted:** 7 (fuzzy/match.go, skill/repomap/skill.go, cli/setup.go, cli/setup_hooks.go, mcp/suggest_lev.go, mcp/lazy_init.go, kernel/health/tools.go)
**Pattern extraction date:** 2026-04-23
