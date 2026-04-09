# Phase 7: Symbol Editing + Multi-Language Fixtures - Research

**Researched:** 2026-04-08
**Domain:** Integration testing -- symbol editing tools + multi-language LSP fixture projects
**Confidence:** HIGH

## Summary

Phase 7 builds on Phase 6's proven test harness (`StartTestDaemon`, `PrepareFixture`, `callTool`, `requireGopls`) to add two capabilities: (1) integration tests for all 6 symbol editing tools (replace body, insert before/after, rename, safe delete, verify edit), and (2) fixture projects for Python, TypeScript, Java, and Rust with known symbols for cross-language symbol retrieval validation.

The codebase already has tree-sitter grammars for all 4 target languages (Go, Python, TypeScript, Rust) compiled into the `BodyExtractor`. The edit tools use a consistent pattern: `PlanEdit` resolves the symbol via LSP `documentSymbol`, then the operation applies the edit and notifies the LS via `didChange`. Each fixture needs minimal files (2-3) with known symbols that have predictable names, line positions, and cross-file references.

**Primary recommendation:** Create fixtures from scratch with deterministic symbol layouts. Use the Go fixture as the template -- each language fixture needs a main file with functions/structs and a secondary file with an interface/trait + implementation for cross-file reference chains. Edit tests should run against Go fixtures first (gopls is the most reliable LS), then extend to other languages.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Create minimal fixtures from scratch (2-3 files per language, known symbols). Do NOT port legacy repos -- they carry unnecessary complexity (Maven, npm, nested dirs).
- **D-02:** Each fixture must have: a class/struct, a function, a cross-file reference, and a symbol that can be edited/renamed/deleted.
- **D-03:** Standard set: pyright (Python), typescript-language-server (TypeScript), jdtls (Java), rust-analyzer (Rust). These match Serena's language registry defaults.
- **D-04:** Each language test uses `requireLS(t, "language-server-name")` pattern (similar to `requireGopls`) to skip gracefully when LS not installed.
- **D-05:** Use `PrepareFixture(t, lang)` per test -- copy entire fixture dir to `t.TempDir()`. Proven pattern from Phase 6, safe even if test fails mid-edit.
- **D-06:** Edit round-trip pattern: `get_symbol_overview` -> `replace_symbol_body` -> `get_symbol_overview` again -> assert new body matches.

### Claude's Discretion
- Exact fixture file contents and symbol names per language
- Whether to add `requireLS` as a generic helper or per-language (like `requireGopls`)
- How many edit operations to test per language (full set vs representative subset)
- Cross-file reference fixture design (interface -> implementation patterns per language)

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EDIT-01 | Edit round-trip works: read symbol -> replace body -> re-read -> verify change took effect | `replace_symbol_body` tool + `get_symbol_overview` for verification; tree-sitter body extraction confirmed for go/python/typescript/rust |
| EDIT-02 | Insert before/after symbol places content at correct position | `insert_before_symbol` / `insert_after_symbol` tools; use `read_file` to verify positioning |
| EDIT-03 | Rename symbol updates all references across files | `rename_symbol` tool (LSP `textDocument/rename`); requires cross-file fixture; line/col are 1-indexed in tool API |
| EDIT-04 | Safe delete blocks when references exist and succeeds when unreferenced | `safe_delete_symbol` tool; checks `find_references` internally; needs both referenced and unreferenced symbols in fixture |
| LANG-01 | Python fixture project with known symbols | pyright-langserver (pip install pyright); needs .py files only, no venv |
| LANG-02 | TypeScript fixture project with known symbols | typescript-language-server (npm); needs .ts files + tsconfig.json |
| LANG-03 | Java fixture project with known symbols | jdtls; needs .java files, may need minimal project markers |
| LANG-04 | Rust fixture project with known symbols | rust-analyzer; needs Cargo.toml + src/*.rs |
| LANG-05 | Cross-file reference chains verified across multi-file fixtures | Each fixture has interface/trait -> implementation pattern spanning 2+ files |
</phase_requirements>

## Standard Stack

No new libraries needed. Phase 7 uses existing dependencies:

### Core (Already in Project)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/stretchr/testify` | (in go.mod) | Test assertions | Already used in Phase 6 tests |
| `github.com/modelcontextprotocol/go-sdk/mcp` | (in go.mod) | MCP client for tool invocation | Already used in harness |
| tree-sitter grammars (go/python/typescript/rust) | (in go.mod) | Body extraction for replace_symbol_body | Already compiled into BodyExtractor |

**No new packages to install.**

## Architecture Patterns

### Recommended Fixture Structure
```
testdata/fixtures/
├── go/              # (existing) main.go + pkg/greeter.go
├── python/          # NEW: main.py + utils.py
├── typescript/      # NEW: main.ts + greeter.ts + tsconfig.json
├── java/            # NEW: Main.java + Greeter.java
└── rust/            # NEW: Cargo.toml + src/main.rs + src/greeter.rs
```

### Recommended Test File Structure
```
test/integration/
├── harness.go          # (existing)
├── helpers.go          # (existing) + add requireLS generic helper
├── symbols_test.go     # (existing) Go fixture symbol tests
├── edit_test.go        # NEW: edit round-trip tests (Go fixture)
├── python_test.go      # NEW: Python fixture symbol + edit tests
├── typescript_test.go  # NEW: TypeScript fixture symbol + edit tests
├── java_test.go        # NEW: Java fixture symbol + edit tests
└── rust_test.go        # NEW: Rust fixture symbol + edit tests
```

### Pattern 1: Generic requireLS Helper
**What:** A single helper function to replace per-language skip functions. [VERIFIED: codebase harness.go]
**When to use:** Every language-specific test file.
**Example:**
```go
// requireLS skips the test if the given language server binary is not in PATH.
func requireLS(t *testing.T, binary string) {
    t.Helper()
    if _, err := exec.LookPath(binary); err != nil {
        t.Skipf("%s not installed, skipping integration test", binary)
    }
}
```
**Note:** `requireGopls` already exists; keep it as an alias or replace all calls. The generic version handles D-04 cleanly.

### Pattern 2: Edit Round-Trip Test
**What:** Read -> edit -> re-read -> assert cycle for edit tools. [VERIFIED: CONTEXT.md D-06]
**When to use:** EDIT-01 testing.
**Example:**
```go
func TestEditRoundTrip_ReplaceBody(t *testing.T) {
    requireLS(t, "gopls")
    fixture := PrepareFixture(t, "go")
    td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

    // 1. Read initial state
    overview1 := callTool(t, td.Session, "get_symbol_overview", map[string]any{
        "path": "main.go",
    })
    assert.Contains(t, textContent(overview1), "Helper")

    // 2. Replace body
    callTool(t, td.Session, "replace_symbol_body", map[string]any{
        "path":        "main.go",
        "symbol_name": "Helper",
        "new_body":    "{\n\tfmt.Println(\"Modified\")\n}",
    })

    // 3. Re-read and verify via read_file (file-level verification)
    content := callTool(t, td.Session, "read_file", map[string]any{
        "path": "main.go",
    })
    assert.Contains(t, textContent(content), "Modified")
}
```

### Pattern 3: Safe Delete Block/Succeed
**What:** Test both paths of safe_delete_symbol. [VERIFIED: delete.go]
**When to use:** EDIT-04.
**Example:**
```go
// Referenced symbol should block deletion
result := callTool(t, td.Session, "safe_delete_symbol", map[string]any{
    "path":        "main.go",
    "symbol_name": "Helper",  // called by UsingHelper
})
assert.Contains(t, textContent(result), "reference")  // NOT an error -- returns ref info

// Unreferenced symbol should delete successfully
callTool(t, td.Session, "safe_delete_symbol", map[string]any{
    "path":        "main.go",
    "symbol_name": "UnusedFunc",
})
```
**Important:** `safe_delete_symbol` returns a non-error result with reference info when blocked. It does NOT set `IsError=true`. The test must check the text content for reference mentions. [VERIFIED: delete.go SafeDelete returns DeleteResult{Deleted: false} which the tool handler formats as regular text]

### Pattern 4: Rename Cross-File Verification
**What:** Rename a symbol and verify all references updated. [VERIFIED: rename.go]
**When to use:** EDIT-03.
**Example:**
```go
// rename_symbol takes 1-indexed line/col
callTool(t, td.Session, "rename_symbol", map[string]any{
    "path":    "main.go",
    "line":    11,    // 1-indexed line of Helper definition
    "column":  6,     // 1-indexed column
    "new_name": "RenamedHelper",
})
// Verify: read_file main.go should contain "RenamedHelper" and not "Helper"
```
**Critical:** The `rename_symbol` tool takes 1-indexed line/col (converts to 0-indexed internally: `args.Line-1, args.Col-1`). [VERIFIED: tools.go line 220]

### Anti-Patterns to Avoid
- **Sharing daemon across edit tests:** Each edit test modifies files. Use separate `PrepareFixture` + `StartTestDaemon` per test function, or at minimum per subtest that edits.
- **Asserting on exact line numbers after edits:** Line numbers shift after insert/delete operations. Assert on symbol names and content, not positions.
- **Forgetting LS initialization wait:** `WaitForLS` polls `search_symbols` for non-empty results. For languages with slower indexing (Java/jdtls), use longer `LSTimeout`. [VERIFIED: harness.go WaitForLS]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| File content verification after edit | Read file directly with os.ReadFile | `read_file` MCP tool via `callTool` | Tests should exercise the MCP tool chain end-to-end |
| Symbol body verification | Parse file manually | `get_symbol_overview` then compare | Validates the full pipeline including LS re-indexing |
| Cross-file rename verification | Grep for old/new names | `find_references` + `read_file` | Exercises actual tool behavior |

## Common Pitfalls

### Pitfall 1: Java jdtls Initialization Timeout
**What goes wrong:** jdtls takes significantly longer to initialize than other language servers (10-30s for even small projects).
**Why it happens:** jdtls downloads/builds workspace metadata, creates .project files, indexes Java classpath.
**How to avoid:** Use longer `LSTimeout` (60s) for Java tests. Keep fixtures minimal (no Maven/Gradle, just .java files). Consider whether jdtls even works without a project file -- may need a `.classpath` or `build.gradle` stub.
**Warning signs:** Tests timeout at 30s default; jdtls process still running after test cleanup.

### Pitfall 2: TypeScript Fixture Needs tsconfig.json
**What goes wrong:** typescript-language-server may not index .ts files without a tsconfig.json in the root.
**Why it happens:** tsserver (underlying TypeScript server) uses tsconfig.json to determine project boundaries and compile options.
**How to avoid:** Include a minimal `tsconfig.json` in every TypeScript fixture: `{"compilerOptions": {"strict": true, "target": "ES2020", "module": "ES2020"}}`. [ASSUMED]
**Warning signs:** `search_symbols` returns "(no results)" after WaitForLS timeout.

### Pitfall 3: Rust Fixture Needs Cargo.toml
**What goes wrong:** rust-analyzer requires a Cargo.toml to function as a language server.
**Why it happens:** rust-analyzer uses cargo metadata for project structure, dependency resolution, and compilation.
**How to avoid:** Include a minimal `Cargo.toml` in every Rust fixture. Even `[package]\nname = "fixture"\nversion = "0.1.0"\nedition = "2021"` is sufficient. [ASSUMED]
**Warning signs:** rust-analyzer fails to start or returns empty symbol results.

### Pitfall 4: safe_delete_symbol Does Not Return IsError for Blocked Deletes
**What goes wrong:** Test uses `callToolExpectError` for a blocked delete, but the tool returns success with reference info.
**Why it happens:** The safe_delete_symbol tool returns a normal (non-error) result containing reference count and locations when deletion is blocked. Only returns `IsError` for actual failures (workspace not activated, symbol not found).
**How to avoid:** Use `callTool` (not `callToolExpectError`) and check text content for reference information. [VERIFIED: tools.go line 252-258]
**Warning signs:** Test fails with "tool should have returned error but succeeded".

### Pitfall 5: rename_symbol Line/Column Indexing
**What goes wrong:** Wrong symbol gets renamed or rename fails with "symbol not found".
**Why it happens:** `rename_symbol` takes 1-indexed line/col in the MCP API but converts to 0-indexed for LSP internally. The fixture symbols must have known 1-indexed positions.
**How to avoid:** Document exact 1-indexed positions as comments in fixture files. Cross-check against `get_symbol_overview` output which shows `(line N)` in 1-indexed format. [VERIFIED: tools.go line 157, 220]
**Warning signs:** "rename: symbol not found" errors; wrong symbol renamed.

### Pitfall 6: LS Worker Dirty Session After Edits
**What goes wrong:** A second daemon or test reusing the same temp dir gets stale LS state.
**Why it happens:** Edit operations mark sessions as dirty (`AcquireSession(ctx, "default", true)`). The worker pool's share-until-dirty model means the dirty worker won't be shared.
**How to avoid:** Already handled by `PrepareFixture(t, lang)` which creates a fresh copy per test. Each `StartTestDaemon` gets its own workspace. Don't try to share daemons across edit tests. [VERIFIED: harness.go PrepareFixture]
**Warning signs:** Stale symbol data returned after edits in sequential subtests.

### Pitfall 7: rust-analyzer Not Properly Installed
**What goes wrong:** `rust-analyzer` binary exists in PATH (from rustup component) but fails with "Unknown binary" error.
**Why it happens:** On this machine, `rustup component add rust-analyzer` was done but the binary requires explicit toolchain specification. [VERIFIED: local environment check]
**How to avoid:** `requireLS` will catch this since exec.LookPath finds the binary but it fails to run. May need a more robust check that actually executes the binary. Or install via `brew install rust-analyzer` instead.
**Warning signs:** requireLS passes but tests fail during LS startup.

## Code Examples

### Minimal Python Fixture (2 files)
```python
# testdata/fixtures/python/main.py
from utils import greet

def helper() -> str:
    """A simple helper function for edit tests."""
    return "hello"

class DemoClass:
    def __init__(self, value: int) -> None:
        self.value = value

    def get_value(self) -> int:
        return self.value

def unused_func() -> None:
    """An unreferenced function for safe_delete tests."""
    pass

def using_helper() -> str:
    return helper()
```
```python
# testdata/fixtures/python/utils.py
def greet(name: str) -> str:
    """Greet someone by name."""
    return f"Hello, {name}!"

class Greeter:
    """Interface-like class for cross-file reference tests."""
    def greet(self, name: str) -> str:
        return greet(name)
```

### Minimal TypeScript Fixture (3 files)
```typescript
// testdata/fixtures/typescript/main.ts
import { Greeter } from "./greeter";

export function helper(): string {
    return "hello";
}

export class DemoClass {
    constructor(public value: number) {}

    getValue(): number {
        return this.value;
    }
}

export function unusedFunc(): void {}

export function usingHelper(): string {
    return helper();
}

const g = new Greeter();
console.log(g.greet("World"));
```
```typescript
// testdata/fixtures/typescript/greeter.ts
export interface IGreeter {
    greet(name: string): string;
}

export class Greeter implements IGreeter {
    greet(name: string): string {
        return `Hello, ${name}!`;
    }
}
```
```json
// testdata/fixtures/typescript/tsconfig.json
{
    "compilerOptions": {
        "strict": true,
        "target": "ES2020",
        "module": "ES2020",
        "moduleResolution": "bundler"
    }
}
```

### Minimal Rust Fixture (3 files)
```toml
# testdata/fixtures/rust/Cargo.toml
[package]
name = "fixture"
version = "0.1.0"
edition = "2021"
```
```rust
// testdata/fixtures/rust/src/main.rs
mod greeter;
use greeter::Greeter;

fn helper() -> String {
    "hello".to_string()
}

struct DemoStruct {
    value: i32,
}

impl DemoStruct {
    fn get_value(&self) -> i32 {
        self.value
    }
}

fn unused_func() {}

fn using_helper() -> String {
    helper()
}

fn main() {
    let g = greeter::SimpleGreeter;
    println!("{}", g.greet("World"));
    println!("{}", helper());
}
```
```rust
// testdata/fixtures/rust/src/greeter.rs
pub trait Greeter {
    fn greet(&self, name: &str) -> String;
}

pub struct SimpleGreeter;

impl Greeter for SimpleGreeter {
    fn greet(&self, name: &str) -> String {
        format!("Hello, {}!", name)
    }
}
```

### Minimal Java Fixture (2 files)
```java
// testdata/fixtures/java/Main.java
public class Main {
    public static String helper() {
        return "hello";
    }

    public static void unusedMethod() {}

    public static String usingHelper() {
        return helper();
    }

    public static void main(String[] args) {
        Greeter g = new SimpleGreeter();
        System.out.println(g.greet("World"));
        System.out.println(helper());
    }
}
```
```java
// testdata/fixtures/java/Greeter.java
interface Greeter {
    String greet(String name);
}

class SimpleGreeter implements Greeter {
    @Override
    public String greet(String name) {
        return "Hello, " + name + "!";
    }
}
```

### Generic requireLS Helper
```go
// In test/integration/harness.go (or helpers.go)

// requireLS skips the test if the given language server binary is not in PATH.
func requireLS(t *testing.T, binary string) {
    t.Helper()
    if _, err := exec.LookPath(binary); err != nil {
        t.Skipf("%s not installed, skipping integration test", binary)
    }
}
```

### Language-Specific LS Binary Names
```go
const (
    lsPyright    = "pyright-langserver"
    lsTypeScript = "typescript-language-server"
    lsJDTLS      = "jdtls"
    lsRustAnalyzer = "rust-analyzer"
    lsGopls      = "gopls"
)
```

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| gopls | Go fixture tests | (not checked, assumed from Phase 6) | -- | t.Skip |
| pyright-langserver | LANG-01 Python | NOT FOUND | -- | t.Skip via requireLS |
| typescript-language-server | LANG-02 TypeScript | FOUND | 5.0.0 | -- |
| jdtls | LANG-03 Java | FOUND | -- | -- |
| rust-analyzer | LANG-04 Rust | BROKEN (rustup binary fails) | -- | t.Skip via requireLS |
| node | TypeScript LS runtime | FOUND | v25.6.1 | -- |
| python3 | pyright runtime | FOUND | 3.14.3 | -- |
| java | jdtls runtime | NOT FOUND (JRE missing) | -- | t.Skip via requireLS |
| rustc/cargo | Rust project compilation | FOUND | 1.90.0 | -- |

**Missing dependencies with no fallback:**
- None -- all are handled by `requireLS(t, binary)` skip pattern (D-04)

**Missing dependencies with fallback:**
- pyright-langserver: Install via `pip install pyright` to enable Python tests
- rust-analyzer: Install via `brew install rust-analyzer` (standalone, not rustup component) to enable Rust tests
- Java JRE: Install to enable jdtls/Java tests

**Note:** The `requireLS` skip pattern means tests gracefully degrade when language servers are unavailable. This is by design (D-04). CI environments can install all language servers; developer machines may have a subset.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | TypeScript fixture needs tsconfig.json for typescript-language-server to index files | Pitfalls | TS tests would fail to find symbols; easy to add tsconfig.json if needed |
| A2 | Rust fixture needs Cargo.toml for rust-analyzer | Pitfalls | Rust tests would fail; easy to add Cargo.toml |
| A3 | jdtls can work with bare .java files without Maven/Gradle | Java fixture design | May need a minimal .classpath or build.gradle; would need investigation if jdtls refuses to start |
| A4 | Python pyright works without a venv or pyrightconfig.json for simple fixtures | Python fixture design | May need a pyrightconfig.json with basic settings |

## Open Questions

1. **Does jdtls work without a build system?**
   - What we know: jdtls is Eclipse-based, typically expects Maven/Gradle projects
   - What's unclear: Whether bare .java files in a directory are sufficient for symbol resolution
   - Recommendation: Try bare files first. If jdtls fails, add a minimal `.classpath` or `settings.gradle`. Keep it as simple as possible per D-01.

2. **Should edit tests run against all 4 languages or just Go?**
   - What we know: Edit tools use language-agnostic LSP operations (documentSymbol, rename) plus tree-sitter for body extraction. Tree-sitter grammars exist for all 4 languages.
   - What's unclear: Whether each LS handles rename/documentSymbol identically enough for the same test patterns
   - Recommendation: Full edit tests on Go fixture (most reliable). Representative subset (replace body + rename) on other languages to validate tree-sitter integration. This balances coverage with test maintenance cost. Discretion area per CONTEXT.md.

3. **rust-analyzer installation path?**
   - What we know: The rustup component exists but the binary fails with "Unknown binary" error on this machine
   - What's unclear: Whether this is a local issue or affects CI too
   - Recommendation: Use `brew install rust-analyzer` or install the standalone binary. The `requireLS` skip pattern handles the graceful degradation.

## Sources

### Primary (HIGH confidence)
- `test/integration/harness.go` -- PrepareFixture, StartTestDaemon, WaitForLS, requireGopls patterns
- `test/integration/helpers.go` -- callTool, callToolExpectError, textContent helpers
- `test/integration/symbols_test.go` -- Go fixture symbol test patterns
- `internal/kernel/edit/tools.go` -- All 6 edit tool registrations, arg schemas, 1-indexed line/col conversion
- `internal/kernel/edit/rename.go` -- RenameSymbol implementation, applyTextEdits
- `internal/kernel/edit/delete.go` -- SafeDelete implementation, reference checking, non-error blocking
- `internal/kernel/edit/replace.go` -- ReplaceBody with tree-sitter body extraction
- `internal/kernel/edit/insert.go` -- InsertBefore/InsertAfter implementations
- `internal/kernel/edit/treesitter.go` -- BodyExtractor with go/python/typescript/rust grammars
- `internal/kernel/symbols/tools.go` -- formatOutline output format (1-indexed lines)
- `internal/langregistry/languages.go` -- LS binary names and install info for all 4 languages
- `testdata/fixtures/go/` -- Existing Go fixture pattern (main.go + pkg/greeter.go)

### Secondary (MEDIUM confidence)
- Local environment probing -- verified which LS binaries are available on this machine

### Tertiary (LOW confidence)
- Java jdtls bare-file support (A3) -- not verified against official docs
- TypeScript tsconfig.json requirement (A1) -- common knowledge but not verified for this specific LS version

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all tools already exist
- Architecture: HIGH -- follows proven Phase 6 patterns with straightforward extensions
- Pitfalls: MEDIUM -- Java/jdtls initialization behavior needs validation during implementation
- Fixture design: MEDIUM -- language-specific LS requirements (A1-A4) need runtime validation

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (stable domain, tool APIs unlikely to change)
