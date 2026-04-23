# Phase 40: USAGE Refresh - Research

**Researched:** 2026-04-23
**Domain:** Documentation -- USAGE.md feature reference and troubleshooting
**Confidence:** HIGH

## Summary

Phase 40 updates USAGE.md to document all features shipped in v1.6 and v1.7. This is a documentation-only phase with no code changes. The current USAGE.md is 717 lines, well-structured with consistent formatting patterns (tutorials, profiles, config reference, troubleshooting, observability, performance tuning). The phase adds a new "Feature Guide" section and updates existing content.

All features to be documented are fully implemented and verified in the codebase. The research below catalogs the exact behavior of each feature from source code, providing the factual basis for documentation content. The user has locked 12 decisions covering placement, depth, tutorial updates, and troubleshooting format.

**Primary recommendation:** Structure the work as two plans -- (1) add Feature Guide section and update Tutorial 1, (2) add troubleshooting entries. Both are low-risk text-only changes.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Add a new "Feature Guide" section between Quick Tutorials and Profiles and Modes
- **D-02:** Each feature group gets its own subsection within the Feature Guide (Fuzzy Editing, RepoMap & Context, Setup CLI, Smart Errors, Progressive Descriptions, Lazy Workspace Init, Health Monitoring)
- **D-03:** Tutorials remain as workflow-oriented guides; Feature Guide serves as feature reference
- **D-04:** Each feature gets 1-2 paragraphs explaining what it does and why, followed by a concrete tool call or usage example
- **D-05:** Enough to discover and use the feature, not exhaustive internals -- the "concept + usage example" pattern
- **D-06:** Strategy cascades (e.g. fuzzy editing's 4 strategies) are listed but not deeply explained
- **D-07:** Update Tutorial 1 (Onboarding) to use `serena setup <client>` instead of manual JSON config
- **D-08:** Tutorials 2 (Refactoring) and 3 (Code Review) remain unchanged -- workflows haven't changed
- **D-09:** No new tutorials added -- Feature Guide covers feature discovery
- **D-10:** New troubleshooting entries follow existing Symptom/Cause/Fix pattern for consistency
- **D-11:** Add jdtls cold-start delay entry with indexing timeout workaround
- **D-12:** Add gopls/Go 1.25 benchmark constraint entry with version requirement

### Claude's Discretion
- Exact wording and paragraph structure within each Feature Guide subsection
- Order of features within the Feature Guide section
- Whether tree-sitter grammar coverage (23 languages) gets its own subsection or is mentioned within fuzzy editing
- Level of detail in troubleshooting workaround steps

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| USAGE-01 | USAGE.md documents all v1.6 features (fuzzy editing, RepoMap/get_context, 23 tree-sitter grammars) | Feature Guide subsections for Fuzzy Editing and RepoMap & Context; grammar list verified from registry.go (23 grammars confirmed) |
| USAGE-02 | USAGE.md documents all v1.7 features (setup CLI, health tool, hooks, smart errors, progressive descriptions, lazy init) | Feature Guide subsections for Setup CLI, Smart Errors, Progressive Descriptions, Lazy Workspace Init, Health Monitoring; hooks documented under Setup CLI |
| USAGE-03 | USAGE.md troubleshooting section is current with known issues and workarounds | New entries for jdtls cold-start and gopls/Go 1.25 benchmark constraint; existing rust-analyzer entry already present |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Documentation content | Static file (USAGE.md) | -- | Pure text editing, no runtime components |

## Feature Inventory (from source code)

This section catalogs exact feature behavior for documentation accuracy. All claims are verified from codebase inspection.

### v1.6 Features

#### Fuzzy Editing (`internal/fuzzy/`)
[VERIFIED: codebase `internal/fuzzy/match.go`, `strategies.go`, `ellipsis.go`]

**4-strategy cascade** (match.go lines 16-23):
1. **Exact** (score 1.0) -- byte-for-byte line match
2. **Whitespace** (score 0.95) -- TrimSpace per line (leading + trailing)
3. **IndentFlex** (score 0.85) -- TrimLeft(" \t") per line (tabs/spaces interchangeable)
4. **Failed** (score 0.0) -- returns unified-diff-style error showing nearest match

Ambiguity handling: N > 1 hits at any strategy returns error immediately, does NOT cascade. User must add anchor context to disambiguate.

**Ellipsis support** (ellipsis.go):
- Search/replacement blocks can use `...` on its own line to skip intermediate content
- Segments match in forward-only order
- Search and replacement must have same segment count
- Empty segments are rejected
- Per-segment tier aggregation: weakest strategy across all segments determines overall score

**Tool name:** `fuzzy_edit` (registered in file-ops category)

#### RepoMap (`internal/repomap/`, `internal/skill/repomap/`)
[VERIFIED: codebase `internal/skill/repomap/skill.go`, `internal/repomap/pagerank.go`, `render.go`]

Two MCP tools:
- **`get_repo_map`** -- Structural overview via tree-sitter tag extraction + PageRank ranking. Token-budget-aware (default 4096, max 32768, min 64).
- **`get_context`** -- Given files relevant to a task, returns ranked symbols using Personalized PageRank on the dependency graph. Default budget 2048.

Backed by:
- TagCache (SQLite-based tag storage with versioning)
- FileGraph (weighted edges from cross-file references)
- TreeRenderer (tree-structured output with elided symbol definitions)
- PageRank with configurable damping (0.85), convergence epsilon, and max iterations

#### 23 Tree-Sitter Grammars (`internal/treesitter/registry.go`)
[VERIFIED: codebase `internal/treesitter/registry.go`]

Languages: Go, Python, TypeScript, TSX, Rust, Java, C, C++, C#, Ruby, PHP, JavaScript, Kotlin, Scala, Bash, Haskell, Julia, OCaml, Lua, Zig, HCL, R, Swift.

Used by both fuzzy editing (BodyExtractor for `replace_symbol_body`) and RepoMap (TagExtractor for `get_repo_map`/`get_context`).

### v1.7 Features

#### Setup CLI (`internal/cli/setup.go`, `setup_clients.go`, `setup_hooks.go`, `setup_health.go`, `setup_detect.go`)
[VERIFIED: codebase]

**Command:** `serena setup <client>`

6 supported clients: `claude-code`, `vscode`, `jetbrains`, `claude-desktop`, `gemini-cli`, `generic`.

**Flags:**
- `--global` -- Register globally (user-scoped) instead of project-scoped
- `--uninstall` -- Remove Serena registration
- `--skip-install` -- Skip language server pre-installation
- `--dry-run` -- Show what would happen without making changes
- `--output` -- Output path for generic client config (default: stdout)
- `--no-hooks` -- Skip hook installation (Claude Code only)

**Flow:** validate client -> resolve binary path -> register MCP config -> detect languages -> pre-install language servers -> health check.

#### Claude Code Hooks (`internal/cli/setup_hooks.go`)
[VERIFIED: codebase]

Installed automatically during `serena setup claude-code` (unless `--no-hooks`). Three hook events:
- **SessionStart:** `serena activate --workspace "$CLAUDE_PROJECT_DIR"` (timeout 30s)
- **PreToolUse:** `serena nudge` on Grep/Read/Bash tools (timeout 5s)
- **Stop:** `serena deactivate --workspace "$CLAUDE_PROJECT_DIR"` (timeout 10s)

All hooks tagged with `"serena_managed": true` for idempotent add/remove. Written to `.claude/settings.json`.

#### get_health Tool (`internal/kernel/health/tools.go`)
[VERIFIED: codebase]

**Tool:** `get_health`
- **verbose=false** (default): Shows only unhealthy workers and non-closed circuits. If all healthy, returns "All N language servers healthy".
- **verbose=true**: Shows all language servers including healthy ones.

Returns JSON with workspace health status, worker states, circuit breaker states.

#### Smart Error Suggestions (`internal/mcp/suggest_lev.go`)
[VERIFIED: codebase]

Levenshtein-distance-based "Did you mean?" suggestions for:
- Misspelled parameter names (e.g., "path" -> "relative_path" via substring match)
- Incorrect enum values

Features:
- Stack-allocated arrays for strings up to 64 chars (zero-heap fast path)
- Exact substring matches bypass maxDistance threshold (high confidence)
- Applied via middleware on schema validation errors

#### Progressive Tool Descriptions (`internal/mcp/registry.go`, `middleware.go`)
[VERIFIED: codebase]

Two-tier description system:
- **BriefDescription** (< 100 tokens): Shown in `tools/list` responses. Reduces token consumption when agents enumerate tools.
- **Full Description**: Available via `get_tool_help` tool. Includes parameter docs, usage examples, patterns.

Applied via ProfileFilterMiddleware: when listing tools, BriefDescription replaces the full Description for tools that have one.

#### get_tool_help Tool (`internal/kernel/help/tools.go`)
[VERIFIED: codebase]

**Tool:** `get_tool_help`
- Input: `tool_name` (required)
- Returns: Full description, parameter documentation (name, type, required/optional, allowed values), usage examples from HelpText.
- Error case: Lists all available tool names if requested tool not found.

#### Lazy Workspace Init (`internal/mcp/lazy_init.go`)
[VERIFIED: codebase]

Middleware that transparently activates workspace on first `tools/call` if none active:
- Thread-safe via `sync.Once` per workspace path
- Resolves workspace from `repo_path` argument or falls back to configured default root
- On failure: returns actionable error "Workspace activation failed: ... Call activate_project explicitly."
- Installed LAST in middleware chain (runs FIRST in LIFO order) -- before telemetry deadline

### Troubleshooting Entries to Add

#### jdtls Cold-Start Delay
[VERIFIED: codebase `test/integration/java_test.go` line 19]

Known issue: jdtls cold-start indexing can exceed 2 minutes in temp/new workspaces. The integration test skips with message: "jdtls cold-start indexing exceeds 2min in temp workspaces".

**Workaround:** Increase `degradation.timeout_index` to 300s for Java projects. The JdtlsAdapter creates a workspace-specific data directory (`-data <workdir>/.jdtls-data`) to avoid cross-workspace conflicts.

#### gopls/Go 1.25 Benchmark Constraint
[VERIFIED: codebase `.planning/RETROSPECTIVE.md` line 25, `go.mod` line 3]

Known issue: gopls v0.17.1 incompatibility with Go 1.25 on linux/amd64 caused CI build failures. The project uses Go 1.25.1 (`go.mod`). Benchmarks use `testing.B.Loop` (Go 1.24+ feature).

**Workaround:** Ensure gopls version is compatible with Go 1.25. The benchmark suite (`test/bench/`) requires Go 1.24+ for `testing.B.Loop`. After Go version bumps, re-baseline benchmarks using `capture-baseline.yml` workflow.

#### rust-analyzer rename (already documented)
[VERIFIED: codebase USAGE.md lines 420-434]

Already present in USAGE.md. No changes needed.

## Current USAGE.md Structure (insertion points)

[VERIFIED: codebase USAGE.md]

Current section order:
1. Header + intro (lines 1-6)
2. **Quick Tutorials** (lines 8-143) -- Tutorial 1 (Onboarding), Tutorial 2 (Refactoring), Tutorial 3 (Code Review)
3. **Profiles and Modes** (lines 144-213)
4. **Configuration Reference** (lines 215-328)
5. **Troubleshooting** (lines 330-459)
6. **Observability Quickstart** (lines 461-598)
7. **Performance Tuning** (lines 600-717)

Per D-01: Insert **Feature Guide** section between Quick Tutorials (after line 143) and Profiles and Modes.

Per D-07: Tutorial 1 Step 2 (lines 19-33) currently shows manual JSON config. Replace with `serena setup <client>` command.

Per D-10/D-11/D-12: Add new troubleshooting entries after the existing "rust-analyzer rename" entry (after line 434) and before "Circuit Breaker Open" (line 439).

## Documentation Patterns (from existing USAGE.md)

[VERIFIED: codebase USAGE.md]

### Feature Explanation Pattern (to use in Feature Guide)
Based on D-04/D-05, each subsection should follow:
```
### Feature Name

[1-2 paragraphs: what it does and why it matters]

[Concrete example: tool call or CLI usage]
```

### Troubleshooting Pattern (existing)
```
### Issue Title

**Symptom:** [what the user sees]

**Cause:** [why it happens]

**Fix:**
[numbered steps or description]
```

### Tutorial Step Pattern (existing)
```
**Step N: Action**

[description]

```command or tool call```
```

## Common Pitfalls

### Pitfall 1: Section Ordering Drift
**What goes wrong:** Inserting the Feature Guide in the wrong location breaks the document flow.
**Why it happens:** Line numbers shift as content is added.
**How to avoid:** Use section headers as anchors, not line numbers. Insert after "## Quick Tutorials" content ends and before "## Profiles and Modes".
**Warning signs:** Feature Guide appearing after Profiles or inside Configuration Reference.

### Pitfall 2: Inconsistent Depth
**What goes wrong:** Some features get deep technical detail while others get one-liners, violating D-04/D-05.
**Why it happens:** Writer knows some features better than others.
**How to avoid:** Enforce the "concept + usage example" template for every subsection. Each gets 1-2 paragraphs + one concrete example.
**Warning signs:** Any subsection without a code/tool-call example, or any exceeding 3 paragraphs of explanation.

### Pitfall 3: Tutorial 1 Partial Update
**What goes wrong:** Updating Step 2 to use `serena setup` but leaving stale references to manual JSON config elsewhere in the tutorial.
**Why it happens:** The manual config is referenced in Step 2 but the "For HTTP mode" block and other context may need adjustment.
**How to avoid:** Read the full Tutorial 1 section and update all references to the old config method.
**Warning signs:** Mixed instructions (some saying "add to settings.json", others saying "serena setup").

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Feature descriptions | Guessing at feature behavior | Source code inspection (this research) | Documentation must match actual implementation |
| Tool call examples | Made-up parameter names | Actual tool schemas from `internal/kernel/*/tools.go` | Parameters must be accurate |
| Grammar count | Manual counting | `internal/treesitter/registry.go` (23 grammars verified) | Count must match code |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (go test) |
| Config file | go.mod |
| Quick run command | `go vet ./...` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| USAGE-01 | v1.6 features documented | manual-only | Visual inspection of USAGE.md | N/A |
| USAGE-02 | v1.7 features documented | manual-only | Visual inspection of USAGE.md | N/A |
| USAGE-03 | Troubleshooting current | manual-only | Visual inspection of USAGE.md | N/A |

### Sampling Rate
- **Per task commit:** `go vet ./...` (no code changes, but safety check)
- **Per wave merge:** Visual review of USAGE.md structure and content
- **Phase gate:** All three requirements verified by content review

### Wave 0 Gaps
None -- documentation-only phase, no test infrastructure needed.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | gopls/Go 1.25 troubleshooting should document version pinning workaround | Troubleshooting Entries | Low -- wording can be adjusted in review |
| A2 | Claude Code hooks should be documented as sub-feature of Setup CLI rather than standalone | Feature Inventory | Low -- placement is Claude's discretion per CONTEXT.md |

## Open Questions

1. **gopls/Go 1.25 benchmark constraint exact workaround**
   - What we know: gopls v0.17.1 had incompatibility with Go 1.25 on linux/amd64 (from RETROSPECTIVE.md). Benchmarks require Go 1.24+.
   - What's unclear: Is this still an active issue or was it resolved by upgrading gopls? The troubleshooting entry should describe the current workaround.
   - Recommendation: Frame as "ensure gopls version compatibility with your Go version" and mention the specific v0.17.1 issue as historical context.

## Sources

### Primary (HIGH confidence)
- `internal/fuzzy/match.go` -- 4-strategy cascade implementation
- `internal/fuzzy/strategies.go` -- sweep functions (exact, whitespace, indentFlex)
- `internal/fuzzy/ellipsis.go` -- ellipsis segment handling
- `internal/repomap/pagerank.go` -- PageRank computation
- `internal/repomap/render.go` -- TreeRenderer with token budgeting
- `internal/skill/repomap/skill.go` -- get_repo_map and get_context tool registration
- `internal/treesitter/registry.go` -- 23 grammar registrations
- `internal/cli/setup.go` -- setup CLI command and flow
- `internal/cli/setup_clients.go` -- 6 client registrars
- `internal/cli/setup_hooks.go` -- Claude Code hooks (3 event types)
- `internal/cli/setup_health.go` -- health check (binary existence)
- `internal/kernel/health/tools.go` -- get_health tool
- `internal/kernel/help/tools.go` -- get_tool_help tool
- `internal/mcp/suggest_lev.go` -- Levenshtein suggestions
- `internal/mcp/registry.go` -- ToolDef with BriefDescription
- `internal/mcp/middleware.go` -- BriefDescription substitution in tools/list
- `internal/mcp/lazy_init.go` -- lazy workspace initialization middleware
- `USAGE.md` -- current document structure (717 lines)
- `.planning/RETROSPECTIVE.md` -- gopls/Go 1.25 issue context

## Metadata

**Confidence breakdown:**
- Feature inventory: HIGH -- all verified from source code
- Documentation patterns: HIGH -- verified from existing USAGE.md
- Troubleshooting content: MEDIUM -- gopls workaround details partially inferred from project history

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (stable -- documentation phase, no moving targets)
