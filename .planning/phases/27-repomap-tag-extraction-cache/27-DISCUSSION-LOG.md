# Phase 27: RepoMap Tag Extraction & Cache - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-16
**Phase:** 27-repomap-tag-extraction-cache
**Areas discussed:** Tag data model, Tree-sitter query strategy, Cache & invalidation, Elision format

---

## Tag Data Model

| Option | Description | Selected |
|--------|-------------|----------|
| Def + Ref only | Two kinds: definition and reference. Simple, sufficient for PageRank graph | ✓ |
| Def + Ref + Import | Three kinds with imports as special reference type | |
| Rich taxonomy | Defs split into function/class/method/variable; refs split into call/use/import | |

**User's choice:** Def + Ref only
**Notes:** Matches aider's model and is sufficient for Phase 28's graph construction.

| Option | Description | Selected |
|--------|-------------|----------|
| Flat tags | Name + kind + file + line + column, no nesting | ✓ |
| Parent scope only | Each tag carries immediate parent symbol name | |
| Full scope chain | Full nesting path like pkg.Server.Run | |

**User's choice:** Flat tags
**Notes:** Phase 28 graph doesn't need scope for PageRank.

| Option | Description | Selected |
|--------|-------------|----------|
| Qualified when available | Server.Run for methods, plain Run for free functions | ✓ |
| Always bare name | Just Run regardless of context | |
| You decide | Claude picks | |

**User's choice:** Qualified when available
**Notes:** Gives disambiguation without full scope chains.

| Option | Description | Selected |
|--------|-------------|----------|
| Byte offsets only | StartByte/EndByte, text derived at elision time | ✓ |
| Store signature text | Cache elided string alongside offsets | |
| You decide | Claude picks | |

**User's choice:** Byte offsets only
**Notes:** Keeps cache compact.

---

## Tree-sitter Query Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Embedded .scm files | Ship .scm query files per language via go:embed | ✓ |
| Programmatic queries | Build queries in Go code like BodyExtractor | |
| You decide | Claude picks | |

**User's choice:** Embedded .scm files
**Notes:** Aider-style pattern. Easy to read, test, extend.

| Option | Description | Selected |
|--------|-------------|----------|
| Shared grammar registry | Extract grammar mapping into shared package | ✓ |
| Independent setup | RepoMap has own grammar init | |
| You decide | Claude picks | |

**User's choice:** Shared grammar registry
**Notes:** Avoids duplicating grammar config across edit and repomap packages.

| Option | Description | Selected |
|--------|-------------|----------|
| All symbols as defs | documentSymbol only returns declarations, map all to def | ✓ |
| Infer refs from symbols | Heuristic def/ref split based on SymbolKind | |
| You decide | Claude picks | |

**User's choice:** All symbols as defs
**Notes:** documentSymbol per LSP spec returns declarations, not references.

| Option | Description | Selected |
|--------|-------------|----------|
| Both defs and refs in Phase 27 | Tree-sitter queries extract both definitions and references | ✓ |
| Defs only in Phase 27 | All reference extraction deferred to Phase 28 | |
| You decide | Claude picks | |

**User's choice:** Both defs and refs in Phase 27
**Notes:** Phase 28 enriches with cross-file refs from LSP, but baseline comes from tree-sitter.

---

## Cache & Invalidation

| Option | Description | Selected |
|--------|-------------|----------|
| Separate database | Own .serena/tags.db file, independent lifecycle | ✓ |
| Shared database | Add tag tables to existing memory SQLite | |
| You decide | Claude picks | |

**User's choice:** Separate database
**Notes:** Cleaner separation, can delete/rebuild without affecting memories.

| Option | Description | Selected |
|--------|-------------|----------|
| File-level mtime | Track file path + mtime, re-extract if changed | ✓ |
| Content hash | SHA256 of file content | |
| You decide | Claude picks | |

**User's choice:** File-level mtime
**Notes:** Simple, reliable, matches RMAP-03 requirement exactly.

| Option | Description | Selected |
|--------|-------------|----------|
| Project .serena/ dir | At .serena/tags.db alongside project config | ✓ |
| XDG cache dir | At ~/.cache/serena/{hash}/tags.db | |
| You decide | Claude picks | |

**User's choice:** Project .serena/ dir
**Notes:** Matches existing .serena/ convention.

| Option | Description | Selected |
|--------|-------------|----------|
| Lazy on first access | Extract tags when file first queried | ✓ |
| Eager background scan | Scan all files on project activation | |
| You decide | Claude picks | |

**User's choice:** Lazy on first access
**Notes:** No startup delay. Phase 28's repo_map triggers bulk extraction.

---

## Elision Format

| Option | Description | Selected |
|--------|-------------|----------|
| Signature + ellipsis | Full signature with ⋯ replacing body | ✓ |
| Signature only | Just the signature, no body indicator | |
| Tree-sitter node slice | Everything except body node, including decorators | |

**User's choice:** Signature + ellipsis
**Notes:** Compact, preserves type info for agents.

| Option | Description | Selected |
|--------|-------------|----------|
| Show fields, elide methods | Struct/class fields shown, method bodies get ⋯ | ✓ |
| Collapse entire body | Single ⋯ for struct/class body | |
| You decide | Claude picks | |

**User's choice:** Show fields, elide methods
**Notes:** Fields are part of type's "signature". Agents need type shapes.

| Option | Description | Selected |
|--------|-------------|----------|
| Render time | Elision at output time, cache stores ranges only | ✓ |
| Extraction time | Elided text cached alongside tags | |
| You decide | Claude picks | |

**User's choice:** Render time
**Notes:** Allows different elision levels per query. Keeps cache simple.

---

## Claude's Discretion

- Tree-sitter .scm query specifics per language
- SQLite schema details (indexes, constraints, WAL mode)
- Shared grammar registry package structure and API
- Error handling for corrupt cache, missing grammars, parse failures

## Deferred Ideas

None — discussion stayed within phase scope.
