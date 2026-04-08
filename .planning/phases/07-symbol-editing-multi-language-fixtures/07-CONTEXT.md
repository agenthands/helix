# Phase 7: Symbol Editing + Multi-Language Fixtures - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Verify edit round-trips (read → edit → verify) and validate that symbol tools work across Python, TypeScript, Java, and Rust fixture projects. This phase adds editing tests and multi-language coverage on top of Phase 6's harness and Go dogfooding.

</domain>

<decisions>
## Implementation Decisions

### Fixture Porting Strategy
- **D-01:** Create minimal fixtures from scratch (2-3 files per language, known symbols). Do NOT port legacy repos — they carry unnecessary complexity (Maven, npm, nested dirs).
- **D-02:** Each fixture must have: a class/struct, a function, a cross-file reference, and a symbol that can be edited/renamed/deleted.

### Language Server Targets
- **D-03:** Standard set: pyright (Python), typescript-language-server (TypeScript), jdtls (Java), rust-analyzer (Rust). These match Serena's language registry defaults.
- **D-04:** Each language test uses `requireLS(t, "language-server-name")` pattern (similar to `requireGopls`) to skip gracefully when LS not installed.

### Edit Test Isolation
- **D-05:** Use `PrepareFixture(t, lang)` per test — copy entire fixture dir to `t.TempDir()`. Proven pattern from Phase 6, safe even if test fails mid-edit.
- **D-06:** Edit round-trip pattern: `get_symbol_overview` → `replace_symbol_body` → `get_symbol_overview` again → assert new body matches.

### Claude's Discretion
- Exact fixture file contents and symbol names per language
- Whether to add `requireLS` as a generic helper or per-language (like `requireGopls`)
- How many edit operations to test per language (full set vs representative subset)
- Cross-file reference fixture design (interface → implementation patterns per language)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 6 Harness (reuse)
- `test/integration/harness.go` — StartTestDaemon, WaitForLS, PrepareFixture, requireGopls
- `test/integration/helpers.go` — callTool, textContent, callToolExpectError
- `testdata/fixtures/go/` — Go fixture pattern (main.go + pkg/greeter.go)

### Existing Edit Tool Tests
- `test/integration/symbols_test.go` — Phase 6 symbol tests with strict assertions
- `internal/kernel/edit/edit_test.go` — Unit tests for edit tools (mock-based)

### Language Registry
- `internal/langregistry/` — 52-language embedded registry, language detection, LS installer config

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `PrepareFixture(t, "go")` — copies testdata/fixtures/{lang}/ to temp dir, proven pattern
- `StartTestDaemon(t, Options{WorkspaceDir: dir})` — activates workspace, waits for LS
- `callTool(t, session, name, args)` — tool invocation with assertions
- `requireGopls(t)` — skip pattern for missing LS

### Established Patterns
- `//go:build integration` on all test files
- Structural assertions for fixture tests, behavioral for codebase smoke
- One daemon per test file, subtests for individual tools

### Integration Points
- New fixtures in `testdata/fixtures/{python,typescript,java,rust}/`
- New test files in `test/integration/` (edit_test.go, python_test.go, typescript_test.go, etc.)

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches informed by Phase 6 patterns.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 07-symbol-editing-multi-language-fixtures*
*Context gathered: 2026-04-08*
