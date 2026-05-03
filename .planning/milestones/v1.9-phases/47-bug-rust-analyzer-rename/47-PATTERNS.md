# Phase 47: bug-rust-analyzer-rename - Pattern Map

**Mapped:** 2026-04-24
**Files analyzed:** 7 (2 new Go, 1 new test, 3 modified Go/test, 1 modified doc, 1 new artifact)
**Analogs found:** 6 / 7 (RCA artifact has no code analog)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/kernel/edit/rename.go` (modify) | kernel-edit orchestrator | request-response (LSP) | `internal/kernel/edit/replace.go` (split + FuzzyMatchInfo strategy tag) | exact (role + data flow) |
| `internal/kernel/edit/rename_override.go` (NEW) | optional-interface + client-side rename helper | request-response (LSP references → text edits) | `internal/kernel/lspool/quirks.go:14-18` (`ArgsModifier`) + `internal/kernel/symbols/retrieval.go:43-55` (`FindReferences`) | exact (optional-iface pattern) |
| `internal/kernel/edit/rename_override_test.go` (NEW) | unit test | table-driven | existing `internal/kernel/edit/*_test.go` table-driven tests | role-match |
| `internal/kernel/lspool/quirks.go` (modify) | LS adapter (quirk) | request-response | existing `RustAnalyzerAdapter.PostInitialize` at `quirks.go:117-123` and `JdtlsAdapter.ExtraArgs` (optional-method impl) at `quirks.go:193-197` | exact |
| `test/integration/rust_test.go` (modify) | integration test | end-to-end MCP tool call | existing `t.Run("replace_body", ...)` subtest at `rust_test.go:98-117` | exact |
| `USAGE.md` (modify) | documentation | — | existing Troubleshooting entry lines 531-537 | exact |
| `.planning/phases/47-.../47-RCA.md` (NEW) | planning artifact | — | (no code analog; doc) | n/a |

## Pattern Assignments

### `internal/kernel/edit/rename.go` (modify)

**Analog:** current `rename.go` (itself) + `internal/kernel/edit/replace.go` for strategy-tag-in-result shape.

**Current imports** (`rename.go:1-13`) — keep unchanged, add nothing new; the override lives in a sibling file:
```go
package edit

import (
    "context"
    "encoding/json"
    "os"
    "sort"
    "strings"

    serr "github.com/postfix/serena/internal/errors"
    "github.com/postfix/serena/internal/kernel/lspool"
    gen "github.com/postfix/serena/protocol/gen"
)
```

**Strategy-tag pattern to mirror** — from `replace.go:15-20`:
```go
// FuzzyMatchInfo carries fuzzy match metadata back to the tool handler
// for response formatting (D-07).
type FuzzyMatchInfo struct {
    Strategy fuzzy.Strategy
    Score    float64
}
```

Add to `RenameResult` (currently `rename.go:15-20`):
```go
type RenameResult struct {
    FilesChanged int
    EditsApplied int
    Files        []string
    Strategy     RenameStrategy // NEW — D-07
}
```

**Dispatcher split pattern** — current single-function shape to refactor (`rename.go:26-115`) becomes:

1. `RenameSymbol(ctx, lease, uri, line, col, newName)` — public dispatcher: runs `prepareRename`, does optional-interface type-assertion on `lease.Adapter()` for `RenameOverrider`, attempts native first, falls back to override on native error. Sets `Strategy` on success. On both-fail, returns `serr.New(serr.Unsupported, ...)`.

2. `tryNativeRename(ctx, lease, uri, pos, newName)` — private, contains the current body starting at `rename.go:53` (`gen.RenameParams{...}` through end of `DocumentChanges` loop).

**Existing prepareRename + rename wire** (lines 32-62) — retain verbatim inside `tryNativeRename`:
```go
prepareParams := gen.PrepareRenameParams{
    TextDocumentPositionParams: gen.TextDocumentPositionParams{
        TextDocument: gen.TextDocumentIdentifier{URI: uri},
        Position:     pos,
    },
}
var prepareResult json.RawMessage
prepareErr := lease.Request(ctx, "textDocument/prepareRename", prepareParams, &prepareResult)
// ... extract refined position ...
params := gen.RenameParams{
    TextDocument: gen.TextDocumentIdentifier{URI: uri},
    Position:     pos,
    NewName:      newName,
}
var wsEdit gen.WorkspaceEdit
if err := lease.Request(ctx, "textDocument/rename", params, &wsEdit); err != nil {
    return nil, serr.Wrap(serr.Internal, "rename", err).WithDetail(err.Error())
}
```

**Structured error pattern (D-06)** — from existing `rename.go:61, 70, 104`:
```go
return nil, serr.Wrap(serr.Internal, "rename", err).WithDetail(err.Error())
```
For D-06 "both paths failed", use:
```go
return nil, serr.New(serr.Unsupported,
    "rename_symbol cannot proceed for this Rust symbol (rust-analyzer rename path unavailable); " +
    "use fuzzy_edit, replace_symbol_body, or search_in_files for a manual rename").
    WithTool("rename_symbol").WithDetail(overrideErr.Error())
```

**Reuse path for client-side edit application** — `applyTextEdits` at `rename.go:117-153` stays package-private; the new `rename_override.go` file lives in the same package so it can call `applyTextEdits` without export.

---

### `internal/kernel/edit/rename_override.go` (NEW)

**Analog 1 (optional-interface):** `internal/kernel/lspool/quirks.go:14-18`:
```go
// ArgsModifier is an optional interface that QuirkAdapters can implement
// to inject extra command-line arguments when starting the LS process.
type ArgsModifier interface {
    ExtraArgs(workDir string, args []string) []string
}
```
Call-site type-assertion pattern (used elsewhere for `ArgsModifier`): `if mod, ok := adapter.(ArgsModifier); ok { ... }`.

**Analog 2 (LSP reference gather):** `internal/kernel/symbols/retrieval.go:43-55`:
```go
func FindReferences(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, includeDecl bool) ([]SymbolLocation, error) {
    params := gen.ReferenceParams{
        TextDocumentPositionParams: makePositionParams(uri, line, col),
        Context: gen.ReferenceContext{
            IncludeDeclaration: includeDecl,
        },
    }
    var result []gen.Location
    if err := lease.Request(ctx, "textDocument/references", params, &result); err != nil {
        return nil, serr.Wrap(serr.Internal, "references", err)
    }
    return locationsToSymbolLocations(result), nil
}
```

**Strategy enum pattern** — mirror `fuzzy.Strategy` typed-string constants (from `internal/fuzzy/types.go` per RESEARCH.md §Pattern 2):
```go
// RenameStrategy enumerates which path produced a successful rename.
// String values are a public contract: they appear in rename_symbol
// tool responses and in the serena_rename_strategy_total metric label set.
type RenameStrategy string

const (
    StrategyLSPNative      RenameStrategy = "lsp-native"
    StrategyRustClientSide RenameStrategy = "rust-client-side"
)
```

**Optional-interface declaration (pattern-match `ArgsModifier`):**
```go
// RenameOverrider is an optional interface that lspool.QuirkAdapter
// implementations can satisfy to bypass textDocument/rename for a
// specific LS. Called by RenameSymbol after a failed native attempt.
//
// Semantic accuracy: overrides are NOT required to match native LSP
// rename fidelity (cross-crate trait-impl, macro expansion). Document
// limits in the implementation's doc comment and in USAGE.md.
//
// Returns (*RenameResult, nil) on success (Strategy set by caller).
// Returns (nil, serr.*) on failure.
type RenameOverrider interface {
    RenameOverride(
        ctx context.Context,
        lease *lspool.WorkerLease,
        uri string,
        line, col int,
        newName string,
    ) (*RenameResult, error)
}
```

**Client-side rename body pattern** — synthesize the two analogs above. Uses `symbols.FindReferences(..., includeDecl=true)` to gather ranges, groups by URI, constructs `gen.TextEdit{Range, NewText: newName}` per reference, and calls `applyTextEdits` (package-private, same package).

**Import-cycle resolution (final, pre-decided — see Shared Patterns below for the metric-counter variant):** interface declared HERE in `package edit` returning `*edit.RenameResult`; a small wrapper type `RustAnalyzerRenameOverride` lives in `package edit` wrapping `*lspool.RustAnalyzerAdapter`. Dispatcher type-asserts for `*lspool.RustAnalyzerAdapter` directly and constructs the wrapper inline. `lspool` never imports `edit`.

---

### `internal/kernel/edit/rename_override_test.go` (NEW)

**Analog:** existing `internal/kernel/edit/` test files (table-driven with mock `WorkerLease`).

**Pattern** — table of cases per RESEARCH.md Wave 0 Gap:
```go
cases := []struct {
    name         string
    nativeErr    error
    overrideRes  *RenameResult
    overrideErr  error
    wantStrategy RenameStrategy
    wantErrKind  serr.Kind
}{
    {"native-success", nil, nil, nil, StrategyLSPNative, 0},
    {"native-fail-override-success", errFake, &RenameResult{FilesChanged: 1, EditsApplied: 3}, nil, StrategyRustClientSide, 0},
    {"both-fail", errFake, nil, errFake2, "", serr.Unsupported},
}
```

Mock adapter implements `RenameOverrider` with canned responses; mock `WorkerLease.Request` returns `errFake` for `textDocument/rename` in fail cases.

---

### `internal/kernel/lspool/quirks.go` (modify)

**Analog (same file):** `RustAnalyzerAdapter` block at `quirks.go:87-123` + `JdtlsAdapter.ExtraArgs` at `quirks.go:193-197` as an example of an optional-method implementation on a specific adapter.

**Existing struct** (`quirks.go:87-92`) — keep:
```go
// RustAnalyzerAdapter provides Rust-specific quirks for rust-analyzer.
// Enables cargo build scripts in initialization options.
type RustAnalyzerAdapter struct {
    Entry langregistry.LSEntry
}
```

**Existing `PostInitialize`** (`quirks.go:117-123`) — keep; may be extended to register an `experimental/serverStatus` notification handler (see also `NotificationHandlers()` at line 109-111):
```go
func (r *RustAnalyzerAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
    didOpenAllFilesRecursive(ctx, adapter, ".rs", "rust")
    return nil
}
```

**Optional-method implementation pattern** — from `JdtlsAdapter.ExtraArgs` (`quirks.go:193-197`):
```go
// ExtraArgs injects -data <dir> so jdtls has a workspace-specific data directory.
func (j *JdtlsAdapter) ExtraArgs(workDir string, args []string) []string {
    dataDir := filepath.Join(workDir, ".jdtls-data")
    _ = os.MkdirAll(dataDir, 0o755)
    return append([]string{"-data", dataDir}, args...)
}
```

Plan 01 uses this shape to add `WaitUntilRenameReady` + `NotificationHandlers` on `RustAnalyzerAdapter`. Plan 02 does NOT modify quirks.go — the `RenameOverrider` wrapper lives in `package edit` as a dispatcher-local seam (see Shared Patterns §Rename override wrapper below).

**Registry unchanged** — `adapterFactory` at `quirks.go:450-464` already maps `"rust"` → `&RustAnalyzerAdapter{Entry: e}`.

**Doc-comment update requirement (D-05):** The struct comment at `quirks.go:87-88` must be expanded to cite the client-side rename's documented semantic limits (no cross-crate trait-impl discovery, macro-expansion corner cases) and reference `BUG-DEFER-02`.

---

### `test/integration/rust_test.go` (modify)

**Analog (same file):** the `t.Run("replace_body", ...)` subtest at lines 98-117 is the exact shape template for an MCP-tool-call integration test in this suite.

**Pattern to mirror** — `rust_test.go:98-117`:
```go
t.Run("replace_body", func(t *testing.T) {
    fixture := PrepareFixture(t, "rust")
    td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 45 * time.Second, LSQuery: "helper"})

    result := callTool(t, td.Session, "replace_symbol_body", map[string]any{
        "path":        "src/main.rs",
        "symbol_name": "helper",
        "new_body":    `"replaced".to_string()`,
    })
    text := textContent(result)
    t.Logf("replace_body: %s", text)
    assert.Contains(t, text, "Replaced")
    // ... read-back verification ...
})
```

**Existing skipped `rename` subtest** (`rust_test.go:119-141`) — the mutation:
1. Remove `t.Skip(...)` at line 120.
2. Keep the existing `rename_symbol` call and `renamed_helper` read-back assertion (lines 126-140).
3. Add the new assertion (D-08):
```go
assert.Regexp(t, `strategy: (lsp-native|rust-client-side)`, text,
    "rename result must advertise strategy tag (D-07)")
```

---

### `USAGE.md` (modify)

**Analog:** existing Troubleshooting entry at `USAGE.md:531-537` (per RESEARCH.md §Sources).

**Pattern:** update-in-place (not a new section). The current entry recommends `replace_symbol_body` as a workaround — the Phase 47 rewrite should state that `rename_symbol` now works, note that `strategy: rust-client-side` signals the fallback path was used, and document the fallback's semantic limits with a pointer to `BUG-DEFER-02`.

---

### `.planning/phases/47-bug-rust-analyzer-rename/47-RCA.md` (NEW)

No code analog. This is a planning artifact capturing:
- Wire-trace of failing `textDocument/rename` vs. succeeding `hover` / `textDocument/references` at `src/main.rs:4:4` in the rust fixture.
- Confirmation or refutation of Assumption A1 (rust-analyzer rename has a distinct readiness gate from `workspace/symbol`).
- Upstream issue pinning (candidate list in RESEARCH.md §Sources Tertiary) — either cite an exact match or file a new one for `BUG-DEFER-02`.

Satisfies Phase 47 Success Criterion #4 and D-02.

---

## Shared Patterns

### Structured error construction
**Source:** `internal/kernel/edit/rename.go:61, 70, 104, 124, 149` (already-used within the file to touch).
**Apply to:** every new error-return in `rename.go`, `rename_override.go`, and the adapter method.
```go
// Wrap existing error:
return nil, serr.Wrap(serr.Internal, "apply rename edits", err).WithDetail(fileURI)

// Synthesize new error (D-06):
return nil, serr.New(serr.Unsupported,
    "rename_symbol cannot proceed for this Rust symbol ...").
    WithTool("rename_symbol").WithDetail(overrideErr.Error())
```
**Kind selection (per RESEARCH.md §Error message for D-06):** use `serr.Unsupported` for both-paths-failed — do NOT use `Internal` (noisy in telemetry outcome classifier). Use `serr.Wrap(serr.Internal, ...)` only for genuinely unexpected I/O or LSP transport failures inside the override body.

### Strategy-tag-in-result
**Source:** `internal/kernel/edit/replace.go:15-20` (`FuzzyMatchInfo`) + `internal/fuzzy/types.go` (typed-string `Strategy` constants referenced by RESEARCH.md §Pattern 2).
**Apply to:** `RenameResult.Strategy` field, rendered in tool-layer response text (mirroring `tools.go:266` per RESEARCH.md §Code Examples).

### Optional-method on QuirkAdapter (type-assertion, not base interface)
**Source:** `internal/kernel/lspool/quirks.go:14-18` (`ArgsModifier`), implemented by only `JdtlsAdapter.ExtraArgs` at `quirks.go:193-197`.
**Apply to:** readiness-signal additions on `RustAnalyzerAdapter` in Plan 01 (`WaitUntilRenameReady` + `NotificationHandlers`). NEVER add to base `QuirkAdapter` interface at `quirks.go:22-32`.

### Rename override wrapper (lspool → edit cycle avoidance — pre-decided)
**Source:** RESEARCH.md §Pitfall 1 + this phase's plan-checker resolution.
**Decision:** The `RenameOverrider` interface lives in `package edit`. A wrapper type `edit.RustAnalyzerRenameOverride struct { Inner *lspool.RustAnalyzerAdapter }` implements it. The dispatcher in `edit.RenameSymbol` type-asserts directly for `*lspool.RustAnalyzerAdapter` (cross-package type assertion is legal) and constructs the wrapper inline. `lspool` MUST NOT import `edit`. Verified by:
```bash
! grep -q '"github.com/postfix/serena/internal/kernel/edit"' internal/kernel/lspool/*.go
```

### Strategy metric recorder (pre-decided — no cycle exists)
**Source:** Plan-checker W5 investigation. Confirmed via grep that `internal/kernel/edit/tools.go` and `internal/kernel/edit/skill.go` ALREADY import `internal/mcp` today:
```bash
grep -rE '"github.com/postfix/serena/internal/mcp"' internal/kernel/edit/
# internal/kernel/edit/skill.go: "github.com/postfix/serena/internal/mcp"
# internal/kernel/edit/tools.go: "github.com/postfix/serena/internal/mcp"
```
**Decision:** Place `RecordRenameStrategy(ctx, strategy string)` as an exported function in `internal/mcp/middleware.go` alongside the existing `serena_tool_calls_total` counter. `internal/kernel/edit/tools.go` calls it via its existing `mcp` import. NO new package (`internal/telemetry`) is introduced. NO registration callback pattern is needed. The "if an import cycle emerges, move to `internal/telemetry`" hedge from earlier drafts is REMOVED.

**Apply to:** Plan 02 Step B (define + export `RecordRenameStrategy` in `internal/mcp/middleware.go`) and Plan 02 Step C (call `mcp.RecordRenameStrategy(ctx, string(result.Strategy))` from `internal/kernel/edit/tools.go` on successful rename).

### LSP request via `lease.Request`
**Source:** `internal/kernel/symbols/retrieval.go:34-38, 50-53` and existing `rename.go:42, 60`.
**Apply to:** all LSP calls in the new override path (`textDocument/references`).
```go
if err := lease.Request(ctx, "textDocument/references", params, &result); err != nil {
    return nil, serr.Wrap(serr.Internal, "references", err)
}
```

### Package-private helper reuse (applyTextEdits)
**Source:** `internal/kernel/edit/rename.go:119-153`.
**Apply to:** client-side rename body inside `rename_override.go` — live in `package edit` so `applyTextEdits` stays unexported (avoids a public-surface expansion).

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `.planning/phases/47-bug-rust-analyzer-rename/47-RCA.md` | planning artifact | — | No code analog; doc-only, shape defined by GSD phase conventions. |

---

## Metadata

**Analog search scope:** `internal/kernel/edit/`, `internal/kernel/lspool/`, `internal/kernel/symbols/`, `internal/fuzzy/`, `internal/errors/`, `test/integration/`.
**Files scanned (read):** `internal/kernel/edit/rename.go`, `internal/kernel/edit/replace.go` (partial), `internal/kernel/lspool/quirks.go`, `internal/kernel/symbols/retrieval.go` (partial), `internal/fuzzy/strategies.go` (partial), `test/integration/rust_test.go` (partial), plus CONTEXT.md and RESEARCH.md for scope.
**Pattern extraction date:** 2026-04-24

## PATTERN MAPPING COMPLETE
