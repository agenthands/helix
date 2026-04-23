# Phase 40: USAGE Refresh - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Update USAGE.md to comprehensively document all features through v1.7 so users can discover and use every capability. Covers v1.6 features (fuzzy editing, RepoMap, tree-sitter grammars), v1.7 features (setup CLI, health tool, hooks, smart errors, progressive descriptions, lazy init), and troubleshooting updates for known issues.

</domain>

<decisions>
## Implementation Decisions

### Feature documentation placement
- **D-01:** Add a new "Feature Guide" section between Quick Tutorials and Profiles and Modes
- **D-02:** Each feature group gets its own subsection within the Feature Guide (Fuzzy Editing, RepoMap & Context, Setup CLI, Smart Errors, Progressive Descriptions, Lazy Workspace Init, Health Monitoring)
- **D-03:** Tutorials remain as workflow-oriented guides; Feature Guide serves as feature reference

### Documentation depth per feature
- **D-04:** Each feature gets 1-2 paragraphs explaining what it does and why, followed by a concrete tool call or usage example
- **D-05:** Enough to discover and use the feature, not exhaustive internals — the "concept + usage example" pattern
- **D-06:** Strategy cascades (e.g. fuzzy editing's 4 strategies) are listed but not deeply explained

### Tutorial updates
- **D-07:** Update Tutorial 1 (Onboarding) to use `serena setup <client>` instead of manual JSON config
- **D-08:** Tutorials 2 (Refactoring) and 3 (Code Review) remain unchanged — workflows haven't changed
- **D-09:** No new tutorials added — Feature Guide covers feature discovery

### Troubleshooting format
- **D-10:** New troubleshooting entries follow existing Symptom/Cause/Fix pattern for consistency
- **D-11:** Add jdtls cold-start delay entry with indexing timeout workaround
- **D-12:** Add gopls/Go 1.25 benchmark constraint entry with version requirement

### Claude's Discretion
- Exact wording and paragraph structure within each Feature Guide subsection
- Order of features within the Feature Guide section
- Whether tree-sitter grammar coverage (23 languages) gets its own subsection or is mentioned within fuzzy editing
- Level of detail in troubleshooting workaround steps

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` -- USAGE-01, USAGE-02, USAGE-03 mapped to this phase

### Current State
- `USAGE.md` -- Current document being updated (717 lines, well-structured)
- `README.md` -- Reference for feature overview (being rewritten in Phase 39)

### Feature Implementations
- `internal/fuzzy/strategies.go` -- Fuzzy editing 4-strategy cascade implementation
- `internal/repomap/` -- RepoMap subsystem (cache.go, pagerank.go, render.go)
- `internal/cli/setup.go` -- `serena setup <client>` CLI command
- `internal/mcp/lazy_init.go` -- Lazy workspace initialization
- `internal/mcp/suggest_lev.go` -- Smart error suggestions (Levenshtein-based)
- `internal/mcp/progressive.go` -- Progressive tool descriptions
- `internal/kernel/lspool/quirks.go` -- Language server quirks (jdtls, gopls issues)

### Prior Phase Context
- `.planning/phases/39-readme-rewrite/39-CONTEXT.md` -- Product tone decisions (D-03: product-aware technical), setup CLI as primary path (D-11)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- USAGE.md already has well-structured sections with consistent formatting patterns
- Troubleshooting section uses established Symptom/Cause/Fix template
- Configuration reference tables are comprehensive and can be referenced from Feature Guide

### Established Patterns
- Tutorials use step-by-step numbered format with tool call examples
- Config keys documented in table format (Key/Type/Default/Description)
- Troubleshooting entries follow Symptom → Cause → Fix structure with code examples

### Integration Points
- Feature Guide subsections should cross-reference relevant config keys in Configuration Reference
- `serena setup <client>` in Tutorial 1 replaces manual JSON config block (lines 19-33)
- Troubleshooting entries for jdtls and gopls connect to existing timeout config documentation

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the decisions above — open to standard approaches for feature documentation.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 40-usage-refresh*
*Context gathered: 2026-04-23*
