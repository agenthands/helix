---
quick_id: 260620-k5n
slug: readme-langsupport-lineage
date: 2026-06-20
status: complete
type: docs
files_changed:
  - README.md
---

# Summary: README language-support matrix + lineage/acknowledgements cleanup

Doc-only change to `README.md`. Four coupled edits, all data verified against source.

## What changed

1. **Capability-tier table + legend** added to `## Programming Language Support`,
   placed immediately after the `<!-- END LANGUAGES -->` auto-gen marker (line 277,
   marker at 273) so docgen regen cannot clobber it. Three-layer framing
   (52 LSP / 23 grammar = ~21 first-class / 3 semantic-graph) + per-language matrix
   (LSP, RepoMap, Body edit, Sem. graph, Type ladder, Fixtures) split into
   first-class (18 langs), grammar-only (3), and LSP-only (fallback) tiers.

2. **Semantic-graph subsection** (line 325): Go / Python / TS-JS only; per-language
   `extract.Provider` + `queries.scm`; `Registry` keyed by `Provider.Language()`
   panics on dup/missing (hard allow-list); language-agnostic graph engine; other
   langs ⇒ `ExtractionStatus = "unsupported"`; first-release scope (SPEC-DRAFT §97).

3. **Lineage framing** (line 13): replaced "originally started as a rewrite of
   Python Serena" with "independent, Go-native project — not a fork, port, or
   rewrite … partially inspired by Serena, Aider, Graphify, and others." v1.9
   rename + legacy/ read-only notes preserved.

4. **Acknowledgements**: replaced the prose lifted verbatim from Serena's
   acknowledgements (which credited Serena's community for Helix's language support)
   with honest credit to inspiration projects (Serena, Aider, Graphify) and the
   ecosystems Helix builds on (LSP servers, tree-sitter, MCP Go SDK).

## Data sources (verified)
- `internal/langregistry/languages.go` — 51 LS entries (~44 base + 7 alternates)
- `internal/treesitter/registry.go` — 23 native grammars
- `internal/kernel/edit/treesitter.go` — 19 body-surgery languages
- `internal/semantic/extract/{golang,python,typescript}` — 3 graph providers (+3 .scm)
- `internal/semantic/types/{golang,java,php,python,ruby,typescript}` — 6 type-ladder langs

## Verification
- `go build ./cmd/helix` → OK (docs only).
- No "fork"/"rewrite of Serena" framing remains in README.
- New table confirmed OUTSIDE the BEGIN/END LANGUAGES auto-gen block.

## Not done (out of scope, flagged)
- CHANGELOG.md retains its 13 historical Serena mentions (real release history —
  intentionally not rewritten).
- INSTALL.md / CONTRIBUTING.md "legacy/ contains the original Python Serena" lines
  left as-is (factual, not fork-framing).
- No CONTRIBUTORS/AUTHORS file exists; GitHub's contributor graph is auto-derived
  from commit history and cannot be hand-edited — no `gh` action was needed.
- README header still says "41+ tools"; live registry is 53 tools (separate drift,
  not in scope here).
