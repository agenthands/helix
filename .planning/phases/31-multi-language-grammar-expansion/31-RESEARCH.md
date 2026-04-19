# Phase 31: Multi-language grammar expansion - Research

**Researched:** 2026-04-18
**Domain:** Tree-sitter grammar bindings, query authoring (Go)
**Confidence:** HIGH

## Summary

This phase expands Serena's tree-sitter grammar support from 4 languages (Go, Python, TypeScript/TSX, Rust) to ~25 languages at aider parity. The work is primarily additive: register new grammar bindings in `GrammarRegistry`, add `.scm` query files for tag extraction (repomap) and body extraction (edit), and adapt aider reference queries to Serena's capture pattern convention.

The critical finding is a **capture pattern mismatch**: aider queries use `@name.definition.X` (composite capture name) while Serena's extractor expects separate `@name` and `@definition.X` captures. All aider queries must be adapted to Serena's convention before use. Additionally, some aider queries use custom predicates (`#strip!`, `#set-adjacent!`, `#select-adjacent!`, `#is-not?`, `#not-match?`) that should be stripped since they control doc-comment extraction which Serena does not use, and the go-tree-sitter binding may not implement all of them.

Go binding availability splits cleanly: **13 languages have official or well-maintained Go bindings** ready for static compilation (the Wave 1 + most of Wave 2). Approximately 5-8 languages from the aider list lack any Go binding compatible with `tree-sitter/go-tree-sitter` and would need either runtime loading via purego or deferral.

**Primary recommendation:** Implement in 2 waves. Wave 1 (7 high-value languages with confirmed Go bindings): Java, C, C++, C#, Ruby, PHP, JavaScript/Kotlin. Wave 2: remaining languages with available bindings. Languages without Go bindings get tag-query-only support via LSP fallback (which already exists).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Full aider parity -- target all ~25 languages that have `.scm` queries in the aider reference collections (`borrow/aider/aider/queries/`).
- **D-02:** Tiered waves -- Wave 1: Java, C, C++, C#, Ruby, PHP, Kotlin (highest demand). Wave 2: remaining ~18 languages.
- **D-03:** Official tree-sitter org Go bindings preferred. Community-maintained forks acceptable. No CGO required.
- **D-04:** Accept binary size growth -- all grammars compiled in, always available. No build-tag gating.
- **D-05:** Both query types for each new language -- repomap tag queries AND edit body queries.
- **D-06:** Merge best of both aider reference collections -- compare `tree-sitter-languages` and `tree-sitter-language-pack` per language.
- **D-07:** Both unit and integration tests -- fixture file per language with golden expected tags.
- **D-08:** Wave 1 languages also get LS fixture integration tests. Wave 2 gets tree-sitter-only test coverage.

### Claude's Discretion
- Exact language list per wave (based on grammar binding availability)
- Query adaptation from aider `.scm` format to Serena's tag/body extraction patterns
- Per-language grammar binding package selection (when multiple community options exist)
- Test fixture content (representative code samples per language)
- Wave 2 ordering within the wave

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Grammar registration | Code Intelligence Kernel | -- | `internal/treesitter/registry.go` owns grammar lifecycle |
| Tag query authoring | Code Intelligence Kernel | -- | `internal/repomap/queries/` consumed by TagExtractor |
| Body query authoring | Code Intelligence Kernel | -- | `internal/kernel/edit/queries/` consumed by BodyExtractor |
| Tag extraction pipeline | Code Intelligence Kernel | -- | `internal/repomap/extractor.go` wires queries to parser |
| Body extraction pipeline | Code Intelligence Kernel | -- | `internal/kernel/edit/treesitter.go` wires configs to parser |
| Qualified name logic | Code Intelligence Kernel | -- | `buildQualifiedName` in extractor.go per language |
| Dependency management | Build System | -- | `go.mod` additions for grammar packages |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/tree-sitter/go-tree-sitter` | v0.25.0 | Tree-sitter Go runtime | Already in use, official binding [VERIFIED: go.mod] |
| `github.com/tree-sitter/tree-sitter-java/bindings/go` | v0.23.5 | Java grammar | Official tree-sitter org [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-c/bindings/go` | v0.24.1 | C grammar | Official tree-sitter org [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-cpp/bindings/go` | v0.23.4 | C++ grammar | Official tree-sitter org [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-c-sharp/bindings/go` | v0.23.5 | C# grammar | Official tree-sitter org [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-ruby/bindings/go` | v0.23.1 | Ruby grammar | Official tree-sitter org [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-php/bindings/go` | v0.24.2 | PHP grammar | Official tree-sitter org [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-javascript/bindings/go` | v0.25.0 | JavaScript grammar | Official tree-sitter org [VERIFIED: pkg.go.dev 200] |
| `github.com/fwcd/tree-sitter-kotlin/bindings/go` | v0.3.2 | Kotlin grammar | Community, well-maintained [VERIFIED: pkg.go.dev 200] |

### Wave 2 Bindings
| Library | Version | Purpose | Status |
|---------|---------|---------|--------|
| `github.com/tree-sitter/tree-sitter-scala/bindings/go` | v0.26.0 | Scala | Official [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-bash/bindings/go` | v0.25.1 | Bash | Official [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-haskell/bindings/go` | v0.23.1 | Haskell | Official [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-julia/bindings/go` | v0.25.0 | Julia | Official [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter/tree-sitter-ocaml/bindings/go` | v0.24.2 | OCaml | Official [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter-grammars/tree-sitter-lua/bindings/go` | v0.5.0 | Lua | Community grammars org [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go` | v1.1.2 | Zig | Community grammars org [VERIFIED: pkg.go.dev 200] |
| `github.com/tree-sitter-grammars/tree-sitter-hcl/bindings/go` | v1.2.0 | HCL/Terraform | Community grammars org [VERIFIED: pkg.go.dev 200] |
| `github.com/alex-pinkus/tree-sitter-swift/bindings/go` | v0.7.1* | Swift | Community [VERIFIED: pkg.go.dev 200] |
| `github.com/r-lib/tree-sitter-r/bindings/go` | v1.2.0 | R | Community [VERIFIED: pkg.go.dev 200] |

*Swift version tags are not semver-prefixed -- may need `@v0.0.0-YYYYMMDD-hash` pseudo-version.

### Languages WITHOUT Go Bindings (aider has queries but no Go grammar available)
| Language | Aider Collection | Status | Recommendation |
|----------|-----------------|--------|----------------|
| Elixir | Both | No Go binding found [VERIFIED: checked 4 repos] | Skip grammar, rely on LSP fallback (RMAP-02) |
| Dart | tree-sitter-language-pack | No Go binding found [VERIFIED] | Skip grammar, rely on LSP fallback |
| Elm | tree-sitter-languages | No Go binding found [VERIFIED] | Skip grammar, rely on LSP fallback |
| Clojure | tree-sitter-language-pack | No Go binding found [VERIFIED] | Skip grammar, rely on LSP fallback |
| Fortran | tree-sitter-languages | No Go binding found [VERIFIED] | Skip grammar, rely on LSP fallback |
| Elisp | Both | No Go binding found [VERIFIED] | Low priority, skip |
| QL | tree-sitter-languages only | No Go binding found [ASSUMED] | Niche language, skip |
| Gleam | tree-sitter-language-pack | No Go binding found [VERIFIED] | Skip |
| D | tree-sitter-language-pack | No Go binding found [VERIFIED] | Skip |
| Pony | tree-sitter-language-pack | No Go binding found [ASSUMED] | Niche, skip |

**Installation (Wave 1):**
```bash
go get github.com/tree-sitter/tree-sitter-java/bindings/go@v0.23.5
go get github.com/tree-sitter/tree-sitter-c/bindings/go@v0.24.1
go get github.com/tree-sitter/tree-sitter-cpp/bindings/go@v0.23.4
go get github.com/tree-sitter/tree-sitter-c-sharp/bindings/go@v0.23.5
go get github.com/tree-sitter/tree-sitter-ruby/bindings/go@v0.23.1
go get github.com/tree-sitter/tree-sitter-php/bindings/go@v0.24.2
go get github.com/tree-sitter/tree-sitter-javascript/bindings/go@v0.25.0
go get github.com/fwcd/tree-sitter-kotlin/bindings/go@v0.3.2
```

## Architecture Patterns

### System Architecture Diagram

```
                    Aider Reference Queries (.scm)
                    borrow/aider/aider/queries/
                              |
                    [Manual adaptation step]
                    - Strip custom predicates (#strip!, #set-adjacent!, etc.)
                    - Convert @name.definition.X -> @name + @definition.X
                    - Author body queries (no aider reference)
                              |
                    +---------+---------+
                    |                   |
          Tag Queries (.scm)    Body Queries (.scm)
          internal/repomap/    internal/kernel/edit/
          queries/{lang}_tags  queries/{lang}.scm
                    |                   |
              [go:embed]          [langConfig]
                    |                   |
            TagExtractor         BodyExtractor
            (compiles queries    (walks AST for
             at startup)          declaration nodes)
                    |                   |
                    +------- + ---------+
                             |
                    GrammarRegistry
                    internal/treesitter/registry.go
                    (holds *tree_sitter.Language per lang)
                             |
                    Grammar Binding Packages
                    (go.mod dependencies)
```

### Recommended Project Structure (additions)
```
internal/treesitter/
  registry.go              # Add ~20 new language entries
internal/repomap/queries/
  java_tags.scm            # NEW - Wave 1
  c_tags.scm               # NEW - Wave 1
  cpp_tags.scm             # NEW - Wave 1
  csharp_tags.scm          # NEW - Wave 1
  ruby_tags.scm            # NEW - Wave 1
  php_tags.scm             # NEW - Wave 1
  javascript_tags.scm      # NEW - Wave 1
  kotlin_tags.scm          # NEW - Wave 1
  scala_tags.scm           # NEW - Wave 2
  bash_tags.scm            # NEW - Wave 2
  ... (more Wave 2)
internal/kernel/edit/queries/
  java.scm                 # NEW - Wave 1
  c.scm                    # NEW - Wave 1
  cpp.scm                  # NEW - Wave 1
  csharp.scm               # NEW - Wave 1
  ruby.scm                 # NEW - Wave 1
  php.scm                  # NEW - Wave 1
  javascript.scm           # NEW - Wave 1
  kotlin.scm               # NEW - Wave 1
  ... (Wave 2)
```

### Pattern 1: Query Adaptation from Aider to Serena

**What:** Aider queries use `@name.definition.X` capture convention; Serena uses separate `@name` and `@definition.X` captures.

**When to use:** Every new language query adaptation.

**Aider format (DO NOT USE directly):**
```scheme
; Source: borrow/aider/aider/queries/tree-sitter-languages/java-tags.scm
(class_declaration
  name: (identifier) @name.definition.class) @definition.class
```

**Serena format (MUST convert to):**
```scheme
; Adapted for Serena capture convention
(class_declaration
  name: (identifier) @name) @definition.class
```

The key transformation: `@name.definition.X` becomes `@name` (the extractor determines kind from the `@definition.X` capture). [VERIFIED: internal/repomap/extractor.go lines 106-119]

### Pattern 2: Grammar Registration

**What:** Add new language to the shared grammar registry.

**Example:**
```go
// Source: internal/treesitter/registry.go (existing pattern)
import tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"

// In NewGrammarRegistry():
r.languages["java"] = tree_sitter.NewLanguage(tree_sitter_java.Language())
```

**PHP special case** -- PHP binding exports `LanguagePHP()` and `LanguagePHPOnly()` (not just `Language()`). [VERIFIED: pkg.go.dev]

**TypeScript binding pattern** -- Already used: exports `LanguageTypescript()` and `LanguageTSX()`. [VERIFIED: registry.go]

### Pattern 3: Body Query Authoring

**What:** Body queries are NOT available from aider. They must be authored from scratch per language.

**Structure:** Body queries capture `@name` and `@body` on declaration nodes. The BodyExtractor also needs a `langConfig` entry listing declaration node types and body field name.

**Example (Java):**
```scheme
; Java declaration types for body extraction
(method_declaration
  name: (identifier) @name
  body: (block) @body)

(constructor_declaration
  name: (identifier) @name
  body: (constructor_body) @body)
```

**Plus langConfig entry:**
```go
be.configs["java"] = &langConfig{
    declarationTypes: map[string]bool{
        "method_declaration":      true,
        "constructor_declaration": true,
    },
    bodyFieldName: "body",
}
```

### Pattern 4: Qualified Name Builder

**What:** Some languages need `buildQualifiedName` extensions for method-inside-class qualification (e.g., `ClassName.methodName`).

**Currently supported:** Go, Python, TypeScript, Rust. [VERIFIED: extractor.go lines 161-273]

**New languages needing qualification:** Java (method inside class), C# (method inside class), Ruby (method inside class/module), Kotlin (function inside class), C++ (method inside class with namespace), Scala (function inside class/object).

### Pattern 5: Tag Query Embed and Registration

**What:** Each new tag query file needs a `go:embed` directive and map entry in `NewTagExtractor`.

```go
//go:embed queries/java_tags.scm
var javaTagsQuery string

// In NewTagExtractor:
querySources["java"] = javaTagsQuery
```

### Anti-Patterns to Avoid
- **Using aider queries verbatim:** The capture convention is different. `@name.definition.X` will fail or produce wrong results in Serena's extractor which expects separate `@name` captures.
- **Including custom predicates:** `#strip!`, `#set-adjacent!`, `#select-adjacent!` control doc-comment extraction that Serena doesn't use. These may cause query compilation errors with `go-tree-sitter`. Strip them.
- **Sharing C/C++ grammar entries:** C and C++ have separate tree-sitter grammars and separate query files. The langregistry uses "cpp" for both `.c` and `.cpp` files, but the tree-sitter registry needs separate "c" and "cpp" entries with different grammars.
- **Forgetting JavaScript/TypeScript overlap:** JavaScript gets its own grammar (`tree-sitter-javascript`), separate from TypeScript. The existing TypeScript query shares with TSX via the same query string but different grammar. JavaScript needs its own query file.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tree-sitter queries | Custom AST walkers | Adapted aider `.scm` queries | Battle-tested across aider's user base |
| Grammar bindings | Building grammars from source | Official Go binding packages | Pre-compiled, version-tagged, no CGO |
| Language detection (file ext -> lang) | New detection logic | Existing langregistry + kernel workspace | Already maps 52 languages to extensions |

## Common Pitfalls

### Pitfall 1: Capture Name Convention Mismatch
**What goes wrong:** Using aider's `@name.definition.X` directly causes Serena's extractor to not find the `@name` capture (it looks for exactly `"name"`, not `"name.definition.class"`).
**Why it happens:** Aider uses a combined capture name convention; Serena uses separate captures.
**How to avoid:** Convert every `@name.definition.X` to `@name` and keep `@definition.X` on the outer node. Validate by checking `CaptureNames()` output matches extractor expectations.
**Warning signs:** Zero tags extracted despite valid source code.

### Pitfall 2: Custom Predicate Compilation Errors
**What goes wrong:** Queries with `#strip!`, `#set-adjacent!`, `#select-adjacent!` fail to compile in `tree_sitter.NewQuery()`.
**Why it happens:** These are custom predicates from the tree-sitter-tags crate, not standard tree-sitter predicates. The Go binding may not support them.
**How to avoid:** Strip all custom predicates and `@doc` captures when adapting aider queries. Only keep structural patterns and `@name`/`@definition`/`@reference` captures.
**Warning signs:** `NewTagExtractor` returns error at startup.

### Pitfall 3: C vs C++ Grammar Confusion
**What goes wrong:** Using the C++ grammar to parse C code (or vice versa) produces wrong/incomplete AST.
**Why it happens:** The langregistry maps both `.c` and `.cpp` to "cpp" language key. But tree-sitter needs separate grammars.
**How to avoid:** Register both "c" and "cpp" in `GrammarRegistry`. Map queries by tree-sitter language key, not langregistry key. May need a mapping layer from file extension to tree-sitter language (`.c` -> "c", `.cpp` -> "cpp").
**Warning signs:** Missing struct definitions when parsing C headers, or macro issues.

### Pitfall 4: PHP Grammar Function Name
**What goes wrong:** `tree_sitter_php.Language()` doesn't exist -- the function is `LanguagePHP()`.
**Why it happens:** PHP binding follows a different naming convention than most other grammars.
**How to avoid:** Check the actual exported function name for each grammar before coding. PHP exports `LanguagePHP()` and `LanguagePHPOnly()`.
**Warning signs:** Compilation error on import.

### Pitfall 5: Missing Body Queries for New Languages
**What goes wrong:** Tag extraction works but body extraction fails because the BodyExtractor has no config for the new language.
**Why it happens:** Aider reference only covers tag queries. Body queries must be authored separately.
**How to avoid:** For every language added, author BOTH a `{lang}_tags.scm` AND a `{lang}.scm` body query, plus add `langConfig` entry.
**Warning signs:** `SupportsLanguage()` returns false for newly added grammar.

### Pitfall 6: Swift Binding Version Tag
**What goes wrong:** `go get github.com/alex-pinkus/tree-sitter-swift@v0.7.1` fails because tags aren't Go-module-compatible.
**Why it happens:** The repository's tags don't follow semver with `v` prefix required by Go modules.
**How to avoid:** Use pseudo-version format: `go get github.com/alex-pinkus/tree-sitter-swift@latest` and let Go resolve the pseudo-version.
**Warning signs:** `go get` fails with "unknown revision" error.

## Code Examples

### Complete Wave 1 Language Addition (Java)

**Step 1: Grammar registration** (`internal/treesitter/registry.go`):
```go
import tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"

// In NewGrammarRegistry():
r.languages["java"] = tree_sitter.NewLanguage(tree_sitter_java.Language())
```

**Step 2: Tag query** (`internal/repomap/queries/java_tags.scm`):
```scheme
; Adapted from borrow/aider/aider/queries/tree-sitter-languages/java-tags.scm
; Converted @name.definition.X -> @name + @definition.X

(class_declaration
  name: (identifier) @name) @definition.class

(method_declaration
  name: (identifier) @name) @definition.method

(interface_declaration
  name: (identifier) @name) @definition.interface

; References
(method_invocation
  name: (identifier) @name
  arguments: (argument_list)) @reference.call

(object_creation_expression
  type: (type_identifier) @name) @reference.class

(superclass (type_identifier) @name) @reference.class
```

**Step 3: Body query** (`internal/kernel/edit/queries/java.scm`):
```scheme
; Java declaration types for body extraction
(method_declaration
  name: (identifier) @name
  body: (block) @body)

(constructor_declaration
  name: (identifier) @name
  body: (constructor_body) @body)
```

**Step 4: Embed + registration** (`internal/repomap/extractor.go`):
```go
//go:embed queries/java_tags.scm
var javaTagsQuery string

// In NewTagExtractor querySources map:
"java": javaTagsQuery,
```

**Step 5: Body config** (`internal/kernel/edit/treesitter.go`):
```go
be.configs["java"] = &langConfig{
    declarationTypes: map[string]bool{
        "method_declaration":      true,
        "constructor_declaration": true,
    },
    bodyFieldName: "body",
}
```

**Step 6: Qualified name** (`internal/repomap/extractor.go`):
```go
case lang == "java" && captureName == "definition.method":
    return qualifyJavaMethod(nameNode, source, name)

func qualifyJavaMethod(nameNode tree_sitter.Node, source []byte, name string) string {
    // Walk up: identifier -> method_declaration -> class_body -> class_declaration
    methodDecl := nameNode.Parent()
    if methodDecl == nil || methodDecl.Kind() != "method_declaration" {
        return ""
    }
    classBody := methodDecl.Parent()
    if classBody == nil || classBody.Kind() != "class_body" {
        return ""
    }
    classDef := classBody.Parent()
    if classDef == nil || classDef.Kind() != "class_declaration" {
        return ""
    }
    classNameNode := classDef.ChildByFieldName("name")
    if classNameNode == nil {
        return ""
    }
    return classNameNode.Utf8Text(source) + "." + name
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| smacker/go-tree-sitter (CGO) | tree-sitter/go-tree-sitter (purego) | 2024 | No CGO needed, official support |
| Per-grammar Go modules separate | Grammar repos include bindings/go | 2024 | `go get` per grammar, no mono-repo |
| Manual grammar compilation | Pre-built in binding packages | 2024 | Zero build complexity |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Body queries for new languages can use `body` as field name universally | Architecture Patterns | Some grammars may use different field names -- verify per grammar's `node-types.json` |
| A2 | `#strip!` and `#set-adjacent!` predicates will cause compilation errors | Pitfalls | If they're silently ignored, no issue -- but if they cause errors, all queries with these predicates need stripping |
| A3 | PHP `LanguagePHP()` is the correct function for full PHP parsing (not `LanguagePHPOnly()`) | Standard Stack | `LanguagePHPOnly()` might exclude HTML-embedded PHP -- need to verify |
| A4 | Swift pseudo-version will work with go-tree-sitter v0.25.0 | Standard Stack | Grammar API may be incompatible with older/newer tree-sitter runtime |
| A5 | QL, Pony, D, Gleam, Elisp have no Go bindings | Standard Stack | May exist in repos not checked |

## Open Questions (RESOLVED)

1. **C/C++ file extension mapping** (RESOLVED)
   - What we know: Langregistry uses "cpp" for both `.c` and `.cpp` files. Tree-sitter needs separate "c" and "cpp" grammars.
   - Resolution: C and C++ use separate registry keys ("c" and "cpp") in GrammarRegistry. `LangFromExt` already maps `.c` -> "c" and `.cpp` -> "cpp" independently (verified: `internal/repomap/render.go` lines 224-227). The tag extraction pipeline receives the language key from `LangFromExt`, which correctly distinguishes C from C++. No additional mapping layer needed.

2. **Body field name verification** (RESOLVED)
   - What we know: Go, Python, TypeScript, Rust all use "body" as the field name for the function/method body.
   - Resolution: Most Wave 1 languages use "body" as the field name (Java, C, C++, C#, PHP, JavaScript, Kotlin). Ruby uses "body" on method nodes. For Wave 2 languages where "body" does not apply (Haskell, OCaml with equation-based definitions; HCL with block-based structure; R with assignment-based functions), the `langConfig` is simply omitted and `BodyExtractor.SupportsLanguage()` returns false, falling back to LSP-based editing. Body field names must be verified per grammar at implementation time; the plans instruct executors to check and adjust accordingly.

3. **JavaScript vs TypeScript query sharing** (RESOLVED)
   - What we know: JavaScript and TypeScript have separate grammars. The existing TypeScript tag query may partially work for JavaScript but may miss JS-specific patterns (CommonJS `module.exports`, etc.).
   - Resolution: JavaScript gets a separate tag query (`javascript_tags.scm`) adapted from the aider JavaScript reference query. JS has different AST node types (e.g., `function` vs TypeScript `function_declaration`, `variable_declaration` with arrow functions for CommonJS patterns). Plan 01 Task 2a creates the dedicated JavaScript query file.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.x |
| Config file | None needed -- standard `go test` |
| Quick run command | `go test ./internal/treesitter/... ./internal/repomap/... ./internal/kernel/edit/... -run Test -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-01 | All ~25 aider languages registered in GrammarRegistry | unit | `go test ./internal/treesitter/... -run TestSupportedLanguages -count=1` | Wave 0 |
| D-05 | Tag queries compile and extract defs/refs for each language | unit | `go test ./internal/repomap/... -run TestExtract_ -count=1` | Extend existing |
| D-05 | Body queries extract function/method bodies for each language | unit | `go test ./internal/kernel/edit/... -run TestExtractBody_ -count=1` | Extend existing |
| D-07 | Fixture files produce expected golden tags | unit | `go test ./internal/repomap/... -run TestExtract_ -count=1` | Wave 0 per language |

### Sampling Rate
- **Per task commit:** `go test ./internal/treesitter/... ./internal/repomap/... ./internal/kernel/edit/... -count=1`
- **Per wave merge:** `go test ./... -count=1 && go vet ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] Test functions for each new language tag extraction (e.g., `TestExtract_JavaFunction`, `TestExtract_CFunction`)
- [ ] Test functions for each new language body extraction (e.g., `TestExtractBody_JavaMethod`, `TestExtractBody_CFunction`)
- [ ] `TestSupportedLanguages` to verify all registered languages count

## Language Binding Availability Matrix

Complete inventory of all languages from the aider reference collections:

| Language | Aider Source | Go Binding Package | Binding Status | Wave | Query Adaptation |
|----------|-------------|-------------------|---------------|------|-----------------|
| Java | Both | `tree-sitter/tree-sitter-java` | Official | 1 | Strip predicates, split captures |
| C | Both | `tree-sitter/tree-sitter-c` | Official | 1 | Split captures only |
| C++ | Both | `tree-sitter/tree-sitter-cpp` | Official | 1 | Strip predicates, split captures |
| C# | Both (named c_sharp / csharp) | `tree-sitter/tree-sitter-c-sharp` | Official | 1 | Split captures only |
| Ruby | Both | `tree-sitter/tree-sitter-ruby` | Official | 1 | Strip predicates, split captures |
| PHP | Both (lang-pack only) | `tree-sitter/tree-sitter-php` | Official | 1 | Split captures |
| JavaScript | Both | `tree-sitter/tree-sitter-javascript` | Official | 1 | Strip predicates, split captures |
| Kotlin | Both (ts-languages) | `fwcd/tree-sitter-kotlin` | Community | 1 | Split captures only |
| Scala | ts-languages | `tree-sitter/tree-sitter-scala` | Official | 2 | Split captures only |
| Bash | ts-languages (none) | `tree-sitter/tree-sitter-bash` | Official | 2 | No aider query -- author from scratch |
| Haskell | ts-languages | `tree-sitter/tree-sitter-haskell` | Official | 2 | Split captures only |
| Julia | ts-languages | `tree-sitter/tree-sitter-julia` | Official | 2 | Split captures only |
| OCaml | Both | `tree-sitter/tree-sitter-ocaml` | Official | 2 | Strip predicates, split captures |
| Lua | ts-language-pack | `tree-sitter-grammars/tree-sitter-lua` | Community | 2 | Split captures only |
| Zig | ts-languages | `tree-sitter-grammars/tree-sitter-zig` | Community | 2 | Split captures only |
| HCL | ts-languages | `tree-sitter-grammars/tree-sitter-hcl` | Community | 2 | Split captures only |
| Swift | ts-language-pack | `alex-pinkus/tree-sitter-swift` | Community | 2 | Split captures only |
| R | ts-language-pack | `r-lib/tree-sitter-r` | Community | 2 | Split captures only |
| Dart | ts-language-pack | None found | No binding | Skip | N/A |
| Elixir | Both | None found | No binding | Skip | N/A |
| Elm | ts-languages | None found | No binding | Skip | N/A |
| Clojure | ts-language-pack | None found | No binding | Skip | N/A |
| Fortran | ts-languages | None found | No binding | Skip | N/A |
| Elisp | Both | None found | No binding | Skip | N/A |
| MATLAB | Both | None found | No binding | Skip | N/A |

**Summary:** 18 languages with Go bindings (8 Wave 1 + 10 Wave 2). 7 languages without Go bindings (skip, use LSP fallback).

## Security Domain

No security implications -- this phase adds tree-sitter grammar parsing of source code files. No network, authentication, cryptography, or user input handling involved.

## Sources

### Primary (HIGH confidence)
- `internal/treesitter/registry.go` -- current grammar registry pattern [VERIFIED: codebase]
- `internal/repomap/extractor.go` -- tag extractor capture convention [VERIFIED: codebase]
- `internal/kernel/edit/treesitter.go` -- body extractor config pattern [VERIFIED: codebase]
- `borrow/aider/aider/queries/` -- aider reference queries [VERIFIED: codebase]
- `go.mod` -- current dependency versions [VERIFIED: codebase]
- pkg.go.dev -- Go binding availability per grammar [VERIFIED: HTTP status checks]
- `go list -m -versions` -- latest binding versions [VERIFIED: Go module proxy]

### Secondary (MEDIUM confidence)
- [go-tree-sitter README](https://github.com/tree-sitter/go-tree-sitter) -- purego runtime loading, predicate support
- [tree-sitter predicates docs](https://tree-sitter.github.io/tree-sitter/using-parsers/queries/3-predicates-and-directives.html) -- predicate semantics

### Tertiary (LOW confidence)
- Custom predicate compilation behavior in go-tree-sitter -- needs runtime verification [A2]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all binding versions verified against Go module proxy and pkg.go.dev
- Architecture: HIGH -- patterns directly derived from existing working code in the codebase
- Pitfalls: HIGH -- capture convention mismatch verified by reading extractor source code

**Research date:** 2026-04-18
**Valid until:** 2026-05-18 (stable domain, grammar packages evolve slowly)
