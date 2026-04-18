# Phase 31: Multi-language grammar expansion - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-18
**Phase:** 31-multi-language-grammar-expansion
**Areas discussed:** Language scope, Grammar sourcing, Query coverage, Testing strategy

---

## Language Scope

| Option | Description | Selected |
|--------|-------------|----------|
| High-value 6-8 | Add Java, C, C++, C#, Ruby, PHP — covers ~90% of real-world codebases | |
| Full aider parity (~25) | Port all languages that have aider .scm queries | ✓ |
| Minimal 3-4 | Just Java, C/C++, Ruby — highest demand missing today | |

**User's choice:** Full aider parity (~25)
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Tiered waves | Wave 1: Java, C, C++, C#, Ruby, PHP, Kotlin. Wave 2: remaining ~18. | ✓ |
| Single sweep | All ~25 in one implementation pass | |
| You decide | Claude picks grouping strategy | |

**User's choice:** Tiered waves
**Notes:** None

---

## Grammar Sourcing

| Option | Description | Selected |
|--------|-------------|----------|
| Official + community | Official tree-sitter org bindings where available, community forks for the rest | ✓ |
| Official only | Only languages with official tree-sitter org Go bindings | |
| You decide | Claude evaluates per language | |

**User's choice:** Official + community
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Accept growth | Single binary value prop, all grammars always available (~20-40MB growth) | ✓ |
| Build tags for optional | Go build tags for opt-in grammars at compile time | |
| You decide | Claude picks based on actual size impact | |

**User's choice:** Accept growth
**Notes:** None

---

## Query Coverage

| Option | Description | Selected |
|--------|-------------|----------|
| Tags only | Focus on repomap tag queries only, LSP fallback handles editing | |
| Both tags and body | Ship complete query coverage for each language | ✓ |
| You decide | Claude assesses per-language whether body queries add value | |

**User's choice:** Both tags and body
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Merge best of both | Compare both aider collections per language, pick more complete queries | ✓ |
| tree-sitter-languages only | Use established collection only | |
| You decide | Claude evaluates quality per language | |

**User's choice:** Merge best of both
**Notes:** None

---

## Testing Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Fixture + golden | Small fixture file per language, golden file with expected tags | |
| Integration only | Test via MCP round-trips on multi-language fixtures | |
| Both unit + integration | Fixture/golden per language plus MCP integration tests | ✓ |

**User's choice:** Both unit + integration
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Tree-sitter tests sufficient | Grammar expansion is about tree-sitter, not LSP | |
| LS fixtures for Wave 1 | Wave 1 languages get LS fixture tests, Wave 2 gets tree-sitter only | ✓ |
| You decide | Claude determines which need LS fixtures | |

**User's choice:** LS fixtures for Wave 1
**Notes:** None

---

## Claude's Discretion

- Exact language list per wave
- Query adaptation from aider format to Serena patterns
- Per-language grammar binding package selection
- Test fixture content
- Wave 2 ordering

## Deferred Ideas

None — discussion stayed within phase scope.
