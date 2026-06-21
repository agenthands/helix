---
name: helix
description: >-
  Symbol-aware code navigation and editing via the `helix` CLI. Use when you need
  to find where a symbol is defined, who calls a function, every reference to a
  name, an interface's implementations, a type or call hierarchy, a file's symbol
  outline, the blast radius of a change, or to rename a symbol / replace a
  function body across files — instead of grep, sed, or cat. Output is terse
  `relpath:line:col<TAB>payload`, copy-paste-able straight into the next verb.
  Dormant for prose, log, and config tasks (READMEs, *.log, YAML/JSON/TOML) —
  do not load this for free-text search or non-code edits.
allowed-tools: Bash(helix *)
---

<!--
  allowed-tools form note: REQUIREMENTS.md SKILL-01 wrote the colon form
  `Bash(helix:*)`. The Claude Code skills doc shows scoped Bash as
  `Bash(git add *)` (space then `*`), so this file emits the docs-confirmed
  `Bash(helix *)` form. Kept here for traceability (SKILL-01).
-->

# helix — symbol-level code intelligence

`helix` answers semantic questions about code by parsing the AST and consulting a
language server, so it returns real definitions, callers, and type-resolved
references — not string matches. Prefer a `helix` verb over grep/sed/cat whenever
the question is about code symbols. Pick by the question, not by tool habit.

Output is terse: `relpath:line:col<TAB>payload`, one locus per line, sorted and
deduped, 1-based. Navigation verbs append one snippet line. `--abs` gives
absolute paths; `--json` gives one compact `{path,line,col,payload}` per line;
piped output is plain (no ANSI). A failing verb prints a typed `kind: msg` to
stderr and exits non-zero.

## Decision matrix

| Question | Use this | Not this |
|---|---|---|
| Where is symbol `X` defined? | `helix go-to-definition --path --line --column` | `grep "X"` |
| Find a symbol by name | `helix search-symbols --query=Foo` | `grep "func Foo"` |
| Who calls / references `Y`? | `helix find-references` / `helix get-call-hierarchy` | `grep -r "Y("` |
| Implementations of an interface | `helix find-implementations` | `grep "implements"` |
| Type hierarchy (super/sub) | `helix get-type-hierarchy` | `grep "extends"` |
| Type/signature at a position | `helix get-hover-info` | infer by reading |
| A file's shape (outline) | `helix get-symbol-overview` | `cat file` |
| Impact of changing a symbol | `helix analyze-blast-radius` | recursive grep |
| Content search across code | `helix search-in-files --pattern=...` | `grep -r ...` |
| Find files by glob | `helix find-files --pattern='**/*.go'` | `find . -name` |
| List a directory | `helix list-directory --path=...` | `ls` |
| Read a file (or a range) | `helix read-file --path=...` | `cat` / `sed -n` |
| Create a new file | `helix create-file --path --content` | `cat >file` |
| Rename a symbol across files | `helix rename-symbol --new-name=...` | `sed -i` |
| Replace a function body | `helix replace-symbol-body` | `sed -i` |
| Find-and-replace in a file | `helix replace-in-file` | `sed -i` |
| Edit tolerant of LLM drift | `helix fuzzy-edit` | `sed -i` |
| Insert near a symbol | `helix insert-before-symbol` / `helix insert-after-symbol` | manual splice |
| Delete a symbol safely | `helix safe-delete-symbol` | `sed -i '/d'` |
| Confirm an edit applied | `helix verify-edit` | re-`cat` the file |
| Errors / warnings in a file | `helix get-diagnostics` | parse build output |
| Quick fixes for a diagnostic | `helix get-code-actions` | hand-edit |
| Format code | `helix format-code` | external formatter |
| Ranked structural repo overview | `helix get-repo-map` | `tree` / many `cat` |
| Context around a symbol/task | `helix get-context` / `helix get-semantic-context` | grep chain |
| Cluster map of the codebase | `helix get-cluster-map` / `helix explain-cluster` | manual grouping |
| Deep explanation of a symbol | `helix explain-symbol-deep` | read every caller |
| Symbols related to one symbol | `helix find-related-symbols` | recursive grep |
| Change-impact graph | `helix get-change-impact-graph` | manual trace |
| Semantic graph status / build | `helix get-semantic-graph-status` / `helix index-semantic-graph` / `helix refresh-semantic-graph` | — |
| Validate a graph edge | `helix validate-graph-edge` | — |
| Project memory (read/write/list) | `helix read-memory` / `helix write-memory` / `helix list-memories` | scratch files |
| Search / rename / edit / delete memory | `helix search-memories` / `helix rename-memory` / `helix edit-memory` / `helix delete-memory` | — |
| Onboard a project / new conversation | `helix onboard-project` / `helix prepare-for-new-conversation` | — |
| Switch profile mode / token budget | `helix switch-mode` / `helix get-token-budget` | — |
| Daemon / LS health | `helix get-health` | guess |
| Full tool help | `helix get-tool-help` | — |

Verbs are grouped by capability (navigation, edit, fileops, diagnostics,
repomap, memory). Run `helix get-tool-help` for the full argument reference of
any verb.

<!-- Token note (SKILL-04): idle skill cost = 599 bytes (this frontmatter
     description, the only text loaded into context until the skill triggers;
     the body below loads ONLY when the skill fires), with a dependency-free
     upper bound of the Claude Code 1536-char description-listing cap (≈150
     tokens at ~4 chars/token). Preloaded full MCP tools/list brief-description
     blob = 2467 bytes across 39 tools (FormatToolList output, the "before"
     baseline; ≈617 tokens, a conservative lower bound since the live
     tools/list also ships per-tool JSON input schemas not counted here). Net:
     the on-demand skill replaces an always-on ~2467-byte preload with a
     ~599-byte idle listing. The 599/1536-byte idle bound is asserted by the
     non-gated SKILL-04 unit test (no API key needed); an exact count_tokens of
     both blobs is recorded in the keyed test/oracle/llm transcript when the
     llm-tagged oracle is run with an API key. Measured 2026-06-21. -->
