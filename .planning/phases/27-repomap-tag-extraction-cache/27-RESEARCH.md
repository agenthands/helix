# Phase 27: RepoMap Tag Extraction & Cache - Research

**Researched:** 2026-04-16
**Domain:** Tree-sitter query-based tag extraction, SQLite caching, scope-aware elision
**Confidence:** HIGH

## Summary

This phase builds a tag extraction system that uses tree-sitter `.scm` queries to extract definition and reference tags from source files (Go, Python, TypeScript, Rust), with LSP `documentSymbol` fallback for unsupported languages. Tags are cached in a separate SQLite database (`.serena/tags.db`) with mtime-based invalidation, and rendered with scope-aware elision at query time.

The project already has the tree-sitter Go bindings (`go-tree-sitter v0.25.0`) and four language grammars in `go.mod`. The existing `BodyExtractor` in `internal/kernel/edit/treesitter.go` demonstrates the grammar initialization pattern. The `internal/memory/index.go` provides a proven SQLite setup pattern with WAL mode and `modernc.org/sqlite`. The primary new work is: (1) a shared grammar registry extracted from `BodyExtractor`, (2) new `.scm` tag query files per language, (3) a tag cache backed by SQLite, and (4) an elision renderer that reads source files and replaces bodies with `...` markers.

**Primary recommendation:** Build `internal/treesitter/` as a shared grammar registry, `internal/repomap/` for tag extraction + cache + elision, with embedded `.scm` query files via `go:embed`. Follow the upstream tree-sitter `tags.scm` convention for capture names (`@name`, `@definition.*`, `@reference.*`).

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Two tag kinds only: `def` (definition) and `ref` (reference). No finer taxonomy.
- **D-02:** Flat tags -- name + kind + file + line + column. No scope nesting or parent chains.
- **D-03:** Qualified names when available -- methods include receiver/owner (e.g., `Server.Run` in Go), free functions use bare name.
- **D-04:** Byte offsets only (StartByte/EndByte) -- no raw text stored in tags. Elided text derived at render time.
- **D-05:** Embedded `.scm` query files per language via `go:embed`. Aider-style pattern.
- **D-06:** Shared grammar registry -- extract grammar/language mapping from `BodyExtractor` into shared package (e.g., `internal/treesitter/`).
- **D-07:** LSP fallback maps all `documentSymbol` results as `def` tags. References come only from tree-sitter queries or Phase 28's LSP enrichment.
- **D-08:** Both defs and refs extracted in Phase 27 via tree-sitter `.scm` queries.
- **D-09:** Separate SQLite database at `.serena/tags.db`. Independent lifecycle from memory store.
- **D-10:** File-level mtime invalidation. Track file path + mtime. If mtime changed, re-extract all tags for that file.
- **D-11:** Cache file lives in project `.serena/` directory alongside project config. Per-project, survives daemon restarts.
- **D-12:** Lazy cache warming -- extract tags on first access, not on project activation.
- **D-13:** Signature + ellipsis format -- full signature line with `...` replacing the body.
- **D-14:** Struct/class fields shown, method bodies elided -- fields are part of the type's "signature".
- **D-15:** Elision performed at render time, not extraction time. Cache stores byte ranges only.

### Claude's Discretion
- Tree-sitter `.scm` query specifics per language (which node types to match for defs/refs)
- SQLite schema column details (indexes, constraints, WAL mode)
- Shared grammar registry package structure and API
- Error handling for corrupt cache, missing grammars, parse failures

### Deferred Ideas (OUT OF SCOPE)
None.

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RMAP-01 | Agent can extract def/ref tags from source files via tree-sitter .scm queries (Go, Python, TypeScript, Rust) | Tree-sitter Query API verified (NewQuery, QueryCursor.Matches), upstream tags.scm patterns documented for all 4 languages |
| RMAP-02 | Languages without tree-sitter grammars fall back to LSP documentSymbol for tag extraction | Existing `GetSymbolOverview` in `internal/kernel/symbols/overview.go` provides the LSP path; maps to `def` tags per D-07 |
| RMAP-03 | Tag cache persists in SQLite with mtime-based invalidation, surviving client reconnects via daemon lifecycle | Proven SQLite pattern in `internal/memory/index.go` with WAL mode; schema design documented below |
| RMAP-09 | Output uses scope-aware elision (signatures without bodies via tree-sitter) | Elision renderer reads source + uses cached byte ranges to replace bodies with `...`; D-14 fields-shown pattern |

</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Grammar registry | Shared library (`internal/treesitter/`) | -- | Both `edit` and `repomap` need grammar access; shared avoids duplication |
| Tag extraction (tree-sitter) | `internal/repomap/` | -- | New package owns tag extraction logic with embedded .scm queries |
| Tag extraction (LSP fallback) | `internal/repomap/` | `internal/kernel/lspool/` | RepoMap triggers LSP documentSymbol via existing pool adapter |
| Tag cache (SQLite) | `internal/repomap/` | -- | Independent `.serena/tags.db` with own lifecycle per D-09 |
| Elision rendering | `internal/repomap/` | `internal/treesitter/` | Render-time body detection uses shared grammars |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go-tree-sitter | v0.25.0 | Tree-sitter Go bindings: parse, query, capture | Already in go.mod, verified [VERIFIED: go.mod] |
| tree-sitter-go | v0.25.0 | Go grammar | Already in go.mod [VERIFIED: go.mod] |
| tree-sitter-python | v0.25.0 | Python grammar | Already in go.mod [VERIFIED: go.mod] |
| tree-sitter-rust | v0.24.2 | Rust grammar | Already in go.mod [VERIFIED: go.mod] |
| tree-sitter-typescript | v0.23.2 | TypeScript grammar | Already in go.mod [VERIFIED: go.mod] |
| modernc.org/sqlite | v1.48.1 | CGO-free SQLite for tag cache | Already in go.mod, proven in memory index [VERIFIED: go.mod] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| go:embed | stdlib | Embed .scm query files into binary | Per D-05, query files compiled in |
| database/sql | stdlib | SQLite driver interface | Standard Go database access |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| modernc.org/sqlite | mattn/go-sqlite3 | mattn requires CGO; modernc is CGO-free and already used |
| .scm embedded queries | Hardcoded Go strings | .scm files are readable, testable, and match upstream convention |

## Architecture Patterns

### System Architecture Diagram

```
Source File (.go/.py/.ts/.rs)
        |
        v
[Language Detection] -- langregistry.ByExtension()
        |
        +--> Has tree-sitter grammar?
        |       |
        |    YES: [Tree-sitter Tag Extractor]
        |       |   1. Parse source with grammar
        |       |   2. Run .scm query (go:embed)
        |       |   3. Map captures to Tag{name, kind, file, line, col, startByte, endByte}
        |       |
        |    NO:  [LSP Fallback Extractor]
        |           1. Request documentSymbol from lspool
        |           2. Map DocumentSymbol[] to Tag{kind: "def"} only
        |
        v
[Tag Cache (SQLite .serena/tags.db)]
    - Keyed by file path
    - mtime-based invalidation
    - Stores: file_path, mtime, tags (name, kind, line, col, start_byte, end_byte)
        |
        v
[Elision Renderer] -- at query time
    1. Read source file bytes
    2. For each def tag with body range, replace body with "..."
    3. Output: signature lines with elided bodies
```

### Recommended Project Structure

```
internal/
  treesitter/              # Shared grammar registry (D-06)
    registry.go            # GrammarRegistry: language -> *tree_sitter.Language
    registry_test.go
  repomap/                 # Tag extraction, cache, elision
    tags.go                # Tag struct, TagKind enum
    extractor.go           # TagExtractor: tree-sitter-based extraction
    extractor_test.go
    fallback.go            # LSP documentSymbol fallback extractor
    fallback_test.go
    cache.go               # TagCache: SQLite persistence + mtime invalidation
    cache_test.go
    schema.go              # SQL schema constant
    elide.go               # ElisionRenderer: signature + ... format
    elide_test.go
    queries/               # Embedded .scm files (go:embed)
      go_tags.scm
      python_tags.scm
      typescript_tags.scm
      rust_tags.scm
```

### Pattern 1: Tree-sitter Query Execution

**What:** Compile `.scm` query, execute against parsed tree, iterate captures to extract tags.
**When to use:** For all 4 supported languages with tree-sitter grammars.
**Example:**

```go
// Source: go doc github.com/tree-sitter/go-tree-sitter [VERIFIED: local go doc]
import tree_sitter "github.com/tree-sitter/go-tree-sitter"

func extractTags(source []byte, lang *tree_sitter.Language, querySource string) ([]Tag, error) {
    parser := tree_sitter.NewParser()
    defer parser.Close()
    if err := parser.SetLanguage(lang); err != nil {
        return nil, err
    }
    tree := parser.Parse(source, nil)
    if tree == nil {
        return nil, errors.New("parse failed")
    }
    defer tree.Close()

    query, qerr := tree_sitter.NewQuery(lang, querySource)
    if qerr != nil {
        return nil, fmt.Errorf("query compile: %s", qerr.Message)
    }
    defer query.Close()

    cursor := tree_sitter.NewQueryCursor()
    defer cursor.Close()

    captureNames := query.CaptureNames()
    var tags []Tag
    matches := cursor.Matches(query, tree.RootNode(), source)
    for m := matches.Next(); m != nil; m = matches.Next() {
        var name string
        var kind TagKind
        var nameNode, defNode tree_sitter.Node

        for _, capture := range m.Captures {
            capName := captureNames[capture.Index]
            switch {
            case capName == "name":
                name = capture.Node.Utf8Text(source)
                nameNode = capture.Node
            case strings.HasPrefix(capName, "definition."):
                kind = TagDef
                defNode = capture.Node
            case strings.HasPrefix(capName, "reference."):
                kind = TagRef
                defNode = capture.Node
            }
        }
        if name != "" && kind != 0 {
            tags = append(tags, Tag{
                Name:      name,
                Kind:      kind,
                Line:      int(nameNode.StartPosition().Row),
                Column:    int(nameNode.StartPosition().Column),
                StartByte: defNode.StartByte(),
                EndByte:   defNode.EndByte(),
            })
        }
    }
    return tags, nil
}
```

### Pattern 2: SQLite Cache with Mtime Invalidation

**What:** Store tags per file with mtime tracking. On access, check file mtime against cached mtime; re-extract if stale.
**When to use:** Every tag access goes through the cache.
**Example:**

```go
// Source: internal/memory/index.go pattern [VERIFIED: codebase]
func (c *TagCache) GetOrExtract(ctx context.Context, filePath string, extractFn func() ([]Tag, error)) ([]Tag, error) {
    info, err := os.Stat(filePath)
    if err != nil {
        return nil, err
    }
    mtime := info.ModTime().UnixNano()

    // Check cache
    cached, cachedMtime, err := c.loadFromDB(filePath)
    if err == nil && cachedMtime == mtime {
        return cached, nil
    }

    // Extract fresh tags
    tags, err := extractFn()
    if err != nil {
        return nil, err
    }

    // Store in cache
    if err := c.storeToDB(filePath, mtime, tags); err != nil {
        slog.Warn("failed to cache tags", "file", filePath, "err", err)
    }
    return tags, nil
}
```

### Pattern 3: Capture Name Convention

**What:** Tag `.scm` queries use the upstream tree-sitter `tags.scm` convention for capture names.
**When to use:** All `.scm` query files.

The convention from upstream tree-sitter grammar repos: [CITED: github.com/tree-sitter/tree-sitter-go/queries/tags.scm]
- `@name` -- the identifier node (symbol name text)
- `@definition.function`, `@definition.method`, `@definition.class`, `@definition.type` -- definition nodes
- `@reference.call`, `@reference.type` -- reference nodes

Serena maps these to the flat D-01 taxonomy:
- Any `@definition.*` capture -> `TagDef`
- Any `@reference.*` capture -> `TagRef`

### Anti-Patterns to Avoid
- **Storing tag text in the cache:** D-04 says byte offsets only. Text is derived at render time from the source file. Storing text wastes space and becomes stale.
- **Eager cache warming on project open:** D-12 requires lazy warming. Don't scan the entire project on daemon start.
- **Fine-grained tag categories:** D-01 says two kinds only (def/ref). Don't create separate categories for function, method, class, type, etc.
- **Query re-compilation per file:** Compile `.scm` queries once per language and reuse across files. Query compilation is not cheap.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Source parsing | Custom lexer/regex | tree-sitter grammar + query | AST-accurate, handles edge cases, upstream maintained |
| SQLite access | Manual file I/O or custom serialization | `database/sql` + `modernc.org/sqlite` | WAL mode, crash safety, concurrent access, proven in codebase |
| Language detection | File extension mapping | `langregistry.ByExtension()` | Already covers 52 languages with proper metadata |
| Body range detection for elision | Regex-based body finder | tree-sitter parse + field lookup | The existing `BodyExtractor` already solves this; share via registry |

**Key insight:** Tree-sitter queries provide AST-accurate tag extraction without building custom parsers. The `.scm` query language is declarative and maintainable. Upstream grammar repos provide reference `tags.scm` files that cover the important node types.

## Common Pitfalls

### Pitfall 1: QueryError is not a Go error
**What goes wrong:** `tree_sitter.NewQuery()` returns `(*Query, *QueryError)` where `QueryError` is a struct pointer, not a Go `error` interface. Checking `if err != nil` won't compile.
**Why it happens:** The go-tree-sitter bindings mirror the C API pattern.
**How to avoid:** Check `if qerr != nil` and access `qerr.Message` for the error string.
**Warning signs:** Compilation failure on `NewQuery` return handling.
[VERIFIED: go doc output]

### Pitfall 2: Capture.Node is a value type (Node), not a pointer (*Node)
**What goes wrong:** Trying to use `capture.Node` as `*Node` causes type errors.
**Why it happens:** `QueryCapture` struct has `Node Node` (value), not `*Node` (pointer).
**How to avoid:** Use `capture.Node.StartByte()` etc. directly on the value.
**Warning signs:** Compilation errors about `Node` vs `*Node`.
[VERIFIED: go doc QueryCapture]

### Pitfall 3: Tree-sitter TypeScript has separate Language functions
**What goes wrong:** Using `tree_sitter_typescript.Language()` which doesn't exist.
**Why it happens:** The TypeScript binding has `LanguageTypescript()` and `LanguageTSX()` as separate functions.
**How to avoid:** Use `tree_sitter_typescript.LanguageTypescript()` for `.ts` files.
**Warning signs:** Link error or missing function.
[VERIFIED: internal/kernel/edit/treesitter.go line 65]

### Pitfall 4: SQLite mtime precision
**What goes wrong:** Mtime comparison fails due to filesystem precision differences (some FS have second precision, others nanosecond).
**Why it happens:** `os.FileInfo.ModTime()` returns `time.Time` with nanosecond precision but the filesystem may only store second or millisecond precision.
**How to avoid:** Store mtime as `UnixNano()` int64 in SQLite. Compare as integers. If the filesystem rounds, the comparison still works because both reads from `os.Stat` will return the same rounded value.
**Warning signs:** Tags being unnecessarily re-extracted on every access.
[ASSUMED]

### Pitfall 5: go:embed requires the embed import
**What goes wrong:** `go:embed` directive silently ignored without the `embed` import.
**Why it happens:** Go requires `import _ "embed"` even when using `//go:embed` directives with `string` or `[]byte` variables.
**How to avoid:** Always include `import _ "embed"` or `import "embed"` in files using `//go:embed`.
**Warning signs:** Empty string variables at runtime.
[VERIFIED: Go spec]

### Pitfall 6: Query captures for references can be noisy
**What goes wrong:** Tree-sitter reference captures (`@reference.call`) match every function call, creating many ref tags per file.
**Why it happens:** Unlike definitions which are few per file, references are numerous.
**How to avoid:** This is expected behavior per D-08. The ref tags are needed for Phase 28's cross-file reference graph. Consider batch inserts for performance.
**Warning signs:** Large tag counts per file (hundreds of refs is normal).
[CITED: aider.chat/2023/10/22/repomap.html]

## Code Examples

### Go Tags Query (.scm)

Based on upstream `tree-sitter-go/queries/tags.scm`: [CITED: github.com/tree-sitter/tree-sitter-go/queries/tags.scm]

```scheme
; Definitions
(function_declaration
  name: (identifier) @name) @definition.function

(method_declaration
  name: (field_identifier) @name) @definition.method

(type_spec
  name: (type_identifier) @name) @definition.type

; References
(call_expression
  function: [
    (identifier) @name
    (selector_expression field: (field_identifier) @name)
  ]) @reference.call

(type_identifier) @name @reference.type
```

### Python Tags Query (.scm)

Based on upstream `tree-sitter-python/queries/tags.scm`: [CITED: github.com/tree-sitter/tree-sitter-python/queries/tags.scm]

```scheme
; Definitions
(function_definition
  name: (identifier) @name) @definition.function

(class_definition
  name: (identifier) @name) @definition.class

; References
(call
  function: [
    (identifier) @name
    (attribute attribute: (identifier) @name)
  ]) @reference.call
```

### Qualified Name Extraction for Go Methods (D-03)

```go
// For method_declaration, extract receiver type to build qualified name
// e.g., "func (s *Server) Run(...)" -> "Server.Run"
func qualifiedName(node tree_sitter.Node, source []byte, captureName string) string {
    name := node.Utf8Text(source)
    if captureName != "definition.method" {
        return name
    }
    // Walk up to find method_declaration parent, extract receiver
    parent := node.Parent()
    if parent == nil || parent.Kind() != "method_declaration" {
        return name
    }
    // Go: receiver is the "receiver" field -> parameter_list -> first param -> type
    recv := parent.ChildByFieldName("receiver")
    if recv == nil {
        return name
    }
    recvText := recv.Utf8Text(source)
    // Extract type name from "(s *Server)" -> "Server"
    typeName := extractReceiverType(recvText)
    if typeName != "" {
        return typeName + "." + name
    }
    return name
}
```

### SQLite Tag Cache Schema

```sql
-- Per D-09: separate tags.db, per D-10: mtime invalidation
CREATE TABLE IF NOT EXISTS file_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT NOT NULL,
    mtime_ns INTEGER NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK(kind IN ('def', 'ref')),
    line INTEGER NOT NULL,
    col INTEGER NOT NULL,
    start_byte INTEGER NOT NULL,
    end_byte INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_file_tags_path ON file_tags(file_path);
CREATE INDEX IF NOT EXISTS idx_file_tags_name ON file_tags(name);
```

### Elision Renderer

```go
// Per D-13, D-14, D-15: render elided view at query time
func elide(source []byte, bodyStart, bodyEnd uint) string {
    // Keep everything before body, replace body with " ... ", keep closing
    before := source[:bodyStart]
    after := source[bodyEnd:]
    return string(before) + " ... " + string(after)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| ctags/etags for tag extraction | tree-sitter .scm queries | 2023+ | AST-accurate, no external binary needed |
| Pygments lexer for references | tree-sitter reference captures | 2024+ | More accurate, captures call sites not just identifiers |
| Aider Python tree-sitter-languages bundle | Per-language Go grammar bindings | Current | Type-safe, no FFI bundle, each grammar is a separate Go module |

**Deprecated/outdated:**
- `tree-sitter-languages` Python package: was monolithic bundle, now individual grammar packages preferred

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | SQLite mtime as UnixNano int64 is sufficient for invalidation across filesystems | Pitfall 4 | Tags could be unnecessarily re-extracted; LOW risk since both stat calls see the same rounded value |
| A2 | Batch INSERT for tags is fast enough without needing prepared statement pooling | Anti-Patterns | Could need optimization for very large files; mitigated by lazy per-file extraction |

## Open Questions

1. **Qualified names for Python/TS/Rust methods**
   - What we know: Go methods have clear receiver syntax. D-03 says qualified names when available.
   - What's unclear: Python method qualification (class.method) requires walking up to the enclosing class_definition. TypeScript methods need similar logic. Rust methods are inside impl blocks.
   - Recommendation: Implement qualified name extraction per language. For Python: walk parent to `class_definition`. For TS: walk parent to `class_declaration`. For Rust: walk parent to `impl_item` and extract the type name from the `type` field. This is Claude's discretion area.

2. **TSX file support**
   - What we know: `tree-sitter-typescript` provides both `LanguageTypescript()` and `LanguageTSX()`.
   - What's unclear: Should `.tsx` files use the TSX parser or TypeScript parser?
   - Recommendation: Use `LanguageTSX()` for `.tsx` files in the grammar registry. The TypeScript parser won't handle JSX syntax correctly.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None needed (Go standard) |
| Quick run command | `go test ./internal/repomap/ ./internal/treesitter/ -count=1 -short` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RMAP-01 | Tree-sitter .scm queries extract def/ref tags from Go/Python/TS/Rust | unit | `go test ./internal/repomap/ -run TestExtract -count=1` | Wave 0 |
| RMAP-02 | LSP fallback produces def tags from documentSymbol | unit | `go test ./internal/repomap/ -run TestFallback -count=1` | Wave 0 |
| RMAP-03 | SQLite cache persists with mtime invalidation | unit | `go test ./internal/repomap/ -run TestCache -count=1` | Wave 0 |
| RMAP-09 | Elision renders signatures without bodies | unit | `go test ./internal/repomap/ -run TestElid -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/repomap/ ./internal/treesitter/ -count=1 -short`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/treesitter/registry_test.go` -- covers grammar registration and SupportsLanguage
- [ ] `internal/repomap/extractor_test.go` -- covers RMAP-01 with inline source snippets per language
- [ ] `internal/repomap/fallback_test.go` -- covers RMAP-02 with mock LSP adapter
- [ ] `internal/repomap/cache_test.go` -- covers RMAP-03 with temp SQLite DB
- [ ] `internal/repomap/elide_test.go` -- covers RMAP-09 with known source + expected output

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | -- |
| V3 Session Management | no | -- |
| V4 Access Control | no | -- |
| V5 Input Validation | yes | Validate file paths are within workspace bounds before extraction |
| V6 Cryptography | no | -- |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via file_path in tags.db | Tampering | Validate paths are within workspace root before cache lookup |
| SQL injection via file path | Tampering | Parameterized queries only (already standard in codebase) |

## Project Constraints (from CLAUDE.md)

- **Language:** Go, single binary
- **Always run:** `go vet` and `go test` before completing any Go task
- **Build:** `go build ./cmd/serena`
- **Test:** `go test ./...`
- **Format:** `gofmt -w .`
- **SQLite:** `modernc.org/sqlite` (CGO-free)
- **Tree-sitter:** `go-tree-sitter` bindings already in use

## Sources

### Primary (HIGH confidence)
- `go.mod` -- verified all tree-sitter and SQLite dependency versions
- `internal/kernel/edit/treesitter.go` -- existing BodyExtractor grammar pattern
- `internal/memory/index.go` + `schema.go` -- existing SQLite pattern with WAL mode
- `go doc github.com/tree-sitter/go-tree-sitter` -- verified Query, QueryCursor, QueryCapture API
- `internal/kernel/symbols/overview.go` -- existing documentSymbol LSP integration

### Secondary (MEDIUM confidence)
- [tree-sitter-go/queries/tags.scm](https://github.com/tree-sitter/tree-sitter-go/blob/master/queries/tags.scm) -- upstream Go tag query patterns
- [tree-sitter-python/queries/tags.scm](https://github.com/tree-sitter/tree-sitter-python/blob/master/queries/tags.scm) -- upstream Python tag query patterns
- [Aider RepoMap blog post](https://aider.chat/2023/10/22/repomap.html) -- tag extraction architecture reference

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all dependencies already in go.mod, APIs verified via local go doc
- Architecture: HIGH -- patterns directly derived from existing codebase code (BodyExtractor, memory Index)
- Pitfalls: HIGH -- verified via go doc output and codebase inspection

**Research date:** 2026-04-16
**Valid until:** 2026-05-16 (stable domain, all dependencies pinned)
