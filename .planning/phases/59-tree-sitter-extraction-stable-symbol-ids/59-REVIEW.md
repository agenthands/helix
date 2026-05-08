---
phase: 59-tree-sitter-extraction-stable-symbol-ids
reviewed: 2026-05-08T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - internal/semantic/extract/provider.go
  - internal/semantic/extract/provider_polymorphic_test.go
  - internal/semantic/extract/registry_test.go
  - internal/semantic/extract/to_store.go
  - internal/semantic/extract/to_store_test.go
  - internal/semantic/scheduler/scheduler_static_test.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 59 (D-06/D-07/D-08/D-11): Code Review Report

**Reviewed:** 2026-05-08
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

The delta is a small, surgical interface widening (`extract.Provider` gains
`Extract(...)`) plus a pure deterministic `ToStoreFacts` adapter and a
source-grep regression gate against scheduler scope-creep. Implementation
quality is high overall: the `Extract` signature mirrors the existing
concrete signatures byte-for-byte, the adapter is genuinely state-free, and
the test suite covers determinism (in-process AND cross-process), nil
safety, golden field shapes, and the dropped-on-floor contract.

No BLOCKER-level defects found — no security issues, no data loss,
no correctness regressions in the production paths I could verify by
inspection. The findings below are quality concerns: one brittle test
mechanism (brace-balanced source grep that does not understand string
literals or comments), one over-broad forbidden-token pattern that will
generate false positives on future legitimate refactors, one
no-op/anchor test that should be a comment, and a small handful of
magic numbers / hard-coded strings that should be named constants.

The Phase 59 delta does its declared job: `provider_polymorphic_test.go`
binds against the *interface* (load-bearing for the RED gate),
`registry_test.go` keeps construction-side panics in scope, `to_store.go`
preserves determinism and the no-I/O contract, and the scheduler static
test would catch a naive future commit that pushes file-walk logic into
`ScheduleInitialExtraction`.

## Warnings

### WR-01: Static scheduler-body grep does not skip strings or comments

**File:** `internal/semantic/scheduler/scheduler_static_test.go:41-60`
**Issue:** `TestScheduleInitialExtraction_BodyRemainsStateOnly` extracts the
function body by raw byte-counting `{` / `}` and then `strings.Contains`-
searching for forbidden tokens. The brace counter does NOT track string
literals, raw strings, runes, or comments. Two future failure modes:

1. False *negative* (skipped detection): a legitimately-added string
   like `"unbalanced }"` early in the body could close the brace count
   prematurely, causing the test to extract a too-short body that does
   not contain the forbidden token even when present.
2. False *positive* (spurious failure): a comment or doc string in the
   function body containing `os.ReadFile` or `.Provider(` triggers the
   gate even though no real call exists.

For the current `scheduler.go` shape this works; for "future regression
gate" durability it does not.

**Fix:** Use `go/parser` to parse `scheduler.go` into an AST, walk to the
`ScheduleInitialExtraction` `*ast.FuncDecl`, and use `ast.Inspect` to
flag forbidden selector expressions (`os.ReadFile`, `filepath.Walk`,
etc.) by their resolved `*ast.SelectorExpr` shape. This is the same
amount of code and immune to comments/strings:

```go
fset := token.NewFileSet()
f, err := parser.ParseFile(fset, "scheduler.go", nil, parser.SkipObjectResolution)
// ... find FuncDecl named "ScheduleInitialExtraction"
ast.Inspect(fn.Body, func(n ast.Node) bool {
    sel, ok := n.(*ast.SelectorExpr)
    if !ok { return true }
    ident, ok := sel.X.(*ast.Ident)
    if !ok { return true }
    key := ident.Name + "." + sel.Sel.Name
    if forbidden[key] {
        t.Fatalf("D-07 violated: %s called", key)
    }
    return true
})
```

### WR-02: Forbidden-token pattern `".Provider("` is over-broad and will produce false positives

**File:** `internal/semantic/scheduler/scheduler_static_test.go:66`
**Issue:** The forbidden token list at line 62-67 includes the bare
substring `".Provider("`. This will match ANY method call whose name is
`Provider` on any receiver — for example, a future legitimate method
on `Scheduler` like `s.health.Provider(...)` (a getter for an LSP
provider, a config provider, a metrics provider). The intent of D-07 is
specifically that the scheduler must not invoke
`extractRegistry.Provider(lang)` to dispatch extraction; a substring
match does not encode that intent. Combined with WR-01 (no comment/string
filtering), the gate will eventually false-trip and force a maintainer to
either edit the test or contort the production code.

**Fix:** Tighten the check by requiring the receiver name. If the
production wiring path always uses an `extractRegistry` field of type
`*extract.Registry`, search for the more specific form
`"extractRegistry.Provider("` (or whatever the chosen field name is).
Better: combine with the AST-based approach in WR-01 and assert the
selector's receiver type via `go/types`. At minimum, document in a
comment WHY `.Provider(` is forbidden so a future maintainer hitting a
spurious failure can diagnose without spelunking phase docs.

### WR-03: Cross-process determinism subprocess writes gob bytes to the same stdout that `go test` may also write to

**File:** `internal/semantic/extract/to_store_test.go:513-553`
**Issue:** The subprocess branch of `TestToStoreFacts_CrossProcessDeterminism`
calls `os.Stdout.Write(buf.Bytes())` and then `os.Exit(0)`. Because the
process is exited mid-test (before `m.Run()` returns), the Go test
framework does not get to print its trailing PASS/`ok` line. This is the
intent — the parent reads raw gob bytes from stdout. However, the test
relies on the assumption that NO output is written to stdout before the
gob bytes:

- If a future `TestMain` is added that prints anything before invoking
  `m.Run()`, that text gets prepended to the gob stream and the parent's
  `bytes.Equal(a, b)` may still succeed (both runs prepend the same
  text), but `gob.NewDecoder(out)` would fail if any consumer ever tried
  to decode the captured bytes.
- More concretely: `-test.v` or any future verbose test logging that
  some CI sets globally would corrupt the stream. The test does not
  pass `-test.v=false` defensively.

The current code works in CI today, but the contract is fragile and
under-documented.

**Fix:** Either (a) write the gob bytes to a unique temp file and read
them back in the parent (avoids stdout entirely), or (b) explicitly pass
`-test.v=false` and document the assumption that the subprocess produces
*only* gob bytes on stdout. Add a defensive comment on the
`os.Stdout.Write` call explaining why no other stdout writes are
permitted in this code path.

```go
// Subprocess MUST NOT write anything else to stdout — the parent reads
// raw gob bytes. Any test framework output would corrupt the stream.
// We exit immediately via os.Exit(0) to bypass m.Run()'s trailing PASS.
_, _ = os.Stdout.Write(buf.Bytes())
os.Exit(0)
```

## Info

### IN-01: `TestD07_IdempotencyAnchor` is a no-op test that always passes

**File:** `internal/semantic/scheduler/scheduler_static_test.go:81-83`
**Issue:** This test does nothing but `t.Log` a string. It exists as a
"discoverability anchor" so a grep for `D-07` lands on a test name. This
is not test value — it adds noise to the test summary, increases
"passing test" count without exercising any behavior, and the same
discoverability is achievable by a comment.

**Fix:** Replace with a top-of-file or top-of-test-package comment:

```go
// D-07: idempotency invariant for ScheduleInitialExtraction is covered
// by scheduler_test.go:TestScheduler_Idempotent. The file-walk-scope
// invariant is covered above by TestScheduleInitialExtraction_BodyRemainsStateOnly.
```

### IN-02: Magic capacity factor `4*len(files)` in adapter pre-allocation

**File:** `internal/semantic/extract/to_store.go:65-66`
**Issue:** The adapter pre-sizes `Symbols` and `References` capacity at
`4 * len(files)`. The `4` is unexplained — it appears to be a guess at
average symbols-per-file. If the average is too low, append() will
re-allocate; if too high, memory is wasted on small workspaces. Not a
correctness bug, but unexplained constants are a code-smell that future
maintainers cannot tune without re-deriving the number.

**Fix:** Either drop the heuristic and let `append` grow naturally
(`make([]semanticstore.SymbolFact, 0)` is fine for cold starts), or
extract to a named constant with a comment citing the empirical basis
(e.g. "average symbols-per-file ≈ 4 across go/ts/py corpus per Phase 59
T-59-04-02 measurement").

### IN-03: Hard-coded `"exported"` string for the `Exported` boolean derivation

**File:** `internal/semantic/extract/to_store.go:144`
**Issue:** `Exported: s.Visibility == "exported"` uses a magic string.
`fact.go:98` documents Visibility as the string union
`"exported|private|package|..."`. SPEC §11.1 is described as a "closed
enum" in the doc comment. There is no shared constant
`ExtractedVisibilityExported` to compare against — so a typo
("Exported", "EXPORTED", "exported ") in either the producer or the
adapter would silently produce `Exported=false` with no test coverage of
the case-sensitivity contract.

**Fix:** Define a constant near the `SymbolFact.Visibility` field in
`fact.go` (e.g. `VisibilityExported = "exported"`) and reference it from
both producers (`internal/semantic/extract/{golang,typescript,python}/`)
and the adapter:

```go
Exported: s.Visibility == VisibilityExported,
```

Add a unit test that pins the case-sensitive contract (e.g.
`Visibility="Exported"` (capital E) → `Exported=false`).

### IN-04: Duplicate package-level doc comment block in `to_store.go`

**File:** `internal/semantic/extract/to_store.go:1-43`
**Issue:** The file opens with a 42-line comment block immediately
preceding `package extract`. Per Go's godoc convention, this is treated
as additional package documentation and concatenated with `doc.go`'s
existing package-level comment. The result is two unrelated package
overviews stacked in `go doc internal/semantic/extract`, which is
discoverability noise. The intent of the block is clearly to document
`ToStoreFacts` (it is entirely about that function's contract), not the
package as a whole.

**Fix:** Either (a) move the comment block onto the `ToStoreFacts`
function declaration (line 58) so godoc renders it as the function's
doc, or (b) separate it from the `package` clause with a blank line so
gofmt treats it as a free-floating comment rather than the package doc:

```go
// (free-floating block here, separated by blank line)

package extract
```

Option (a) is preferred — it's the standard Go convention for "function
contract" documentation.

---

_Reviewed: 2026-05-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
