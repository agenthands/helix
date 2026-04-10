# Phase 14: Documentation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 14-documentation
**Areas discussed:** README strategy, USAGE.md depth & structure, CHANGELOG approach, Table generation tooling

---

## README Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Evolve existing (Recommended) | Keep the current structure and content, fill gaps (DOC-01-DOC-04). Swap static tool/language lists with auto-generated ones. Add missing client configs. | ✓ |
| Rewrite from scratch | Start fresh with a new structure. Risk: lose good content that's already there. | |
| Minimal update | Only add what DOC-01-DOC-04 require, touch nothing else. | |

**User's choice:** Evolve existing
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Keep brief mention (Recommended) | One line pointing to legacy/ directory for historical reference. | |
| Remove entirely | Go rewrite is the product now, no need to reference Python. | ✓ |
| Expand with migration guide | Help existing Python Serena users migrate to Go version. | |

**User's choice:** Remove entirely
**Notes:** None

---

## USAGE.md Depth & Structure

| Option | Description | Selected |
|--------|-------------|----------|
| Reference manual (Recommended) | Structured reference: profiles, modes, config precedence, then sections for workflows, troubleshooting, observability, perf tuning. | |
| Tutorial-first | Walk through common scenarios step-by-step. Reference material woven in. | |
| Hybrid | Start with 2-3 quick tutorials, then transition to reference sections. | ✓ |

**User's choice:** Hybrid
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| In USAGE.md (Recommended) | Single reference doc. Observability and perf tuning are operational topics. | ✓ |
| Separate files | OBSERVABILITY.md and PERFORMANCE.md alongside USAGE.md. | |

**User's choice:** In USAGE.md
**Notes:** None

---

## CHANGELOG Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Start fresh for Go (Recommended) | New CHANGELOG with v1.0, v1.1, v1.2. Legacy history in git and legacy/ dir. | ✓ |
| Keep legacy entries | Append Go versions above legacy entries. Full history in one file. | |
| Archive legacy separately | Move legacy entries to legacy/CHANGELOG.md, fresh file for Go versions. | |

**User's choice:** Start fresh for Go
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| v1.0 / v1.1 / v1.2 (Recommended) | Matches milestone naming in roadmap. Simple, clear. | ✓ |
| Semver (1.0.0 / 1.1.0 / 1.2.0) | Standard semver format. More formal. | |
| Dated (2026-04-08) | Date-based like the legacy entries. | |

**User's choice:** v1.0 / v1.1 / v1.2
**Notes:** None

---

## Table Generation Tooling

| Option | Description | Selected |
|--------|-------------|----------|
| Go codegen tool (Recommended) | A cmd/docgen or internal/docgen tool that reads registries and outputs markdown. | ✓ |
| Makefile script | Shell script that greps Go source. Simpler but more fragile. | |
| go:generate with template | Use go:generate directives in registry packages. Keeps generation close to source. | |

**User's choice:** Go codegen tool
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Marker comments in README (Recommended) | README has markers, codegen replaces content between them. | ✓ |
| Separate .md fragments | Generate docs/tools-table.md and docs/languages-table.md. | |
| Inline in codegen output | Codegen writes entire README. Full control but harder to maintain prose. | |

**User's choice:** Marker comments in README
**Notes:** None

---

## Claude's Discretion

- README section ordering and prose
- USAGE.md tutorial scenario selection
- CHANGELOG entry detail level
- Codegen tool internal architecture

## Deferred Ideas

None — discussion stayed within phase scope.
