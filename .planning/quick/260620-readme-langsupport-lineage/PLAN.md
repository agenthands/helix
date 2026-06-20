---
quick_id: 260620-k5n
slug: readme-langsupport-lineage
date: 2026-06-20
type: docs
---

# Quick Task: README language-support matrix + lineage/acknowledgements cleanup

Doc-only changes (no code). Four coupled edits, all verified against source.

## Tasks

1. **README — capability-tier table + legend.** Insert AFTER the auto-generated
   `<!-- END LANGUAGES -->` marker in `## Programming Language Support` so docgen
   regen never clobbers it. Three tiers: 52 LSP-configured · 23 grammars /
   ~21 first-class (RepoMap tags + AST body surgery) · 3 semantic-graph langs.
   Per-language matrix (LSP / RepoMap / Body edit / Sem. graph / Type ladder /
   Fixtures) + legend.
   - Sources: `internal/langregistry/languages.go` (51 entries),
     `internal/treesitter/registry.go` (23 grammars),
     `internal/kernel/edit/treesitter.go` (19 body-surgery langs),
     `internal/semantic/extract/{golang,python,typescript}` (3),
     `internal/semantic/types/{golang,java,php,python,ruby,typescript}` (6).

2. **README — semantic-graph subsection.** Go / Python / TS-JS only;
   per-language `extract.Provider` + `queries.scm`; daemon `Registry` keyed by
   `Provider.Language()` panics on dup/missing (hard allow-list); graph engine
   language-agnostic; other langs ⇒ `ExtractionStatus = "unsupported"`;
   deliberate first-release scope (SPEC-DRAFT.md §97).

3. **README — remove fork/rewrite lineage framing.** Line 13: "rewrite of Python
   Serena" → independent Go project, partially inspired by Serena, Aider,
   Graphify and other prior art (not a fork/port/rewrite). Keep v1.9 rename +
   legacy/ read-only notes. Do not touch CHANGELOG history.

4. **README — fix Acknowledgements.** Current prose is lifted verbatim from
   Serena's acknowledgements and credits Serena's contributor community for
   Helix's language support. Rewrite to credit prior-art projects (Serena,
   Aider, Graphify) + the ecosystems Helix builds on (LSP servers, tree-sitter,
   MCP Go SDK). No CONTRIBUTORS/AUTHORS file exists.

## Verification
- New table sits OUTSIDE the `BEGIN/END LANGUAGES` block.
- `go build ./cmd/helix` unaffected (docs only).
- No "fork"/"rewrite of Serena" framing remains in README lead.
