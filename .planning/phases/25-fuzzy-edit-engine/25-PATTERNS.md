# Phase 25: Fuzzy Edit Engine - Pattern Map

**Mapped:** 2026-04-15
**Files analyzed:** 12 (7 production + 5 test)
**Analogs found:** 12 / 12

## File Classification

`internal/fuzzy/` is a brand-new pure Go package — no I/O, no init(), no global state, no external deps beyond stdlib + `serr`. Every file maps to a leaf-utility analog already in the repo (`internal/errors/` is the closest existing pure package; `internal/kernel/edit/` and `internal/kernel/fileops/` show the project's table-driven test idiom and `serr` error construction style).

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `internal/fuzzy/types.go` (Options, Result, Strategy enum) | types/config | transform | `internal/kernel/edit/planner.go` (lines 13-30, EditType consts + EditPlan struct) | exact — same shape: const enum + plain struct of plan/result fields |
| `internal/fuzzy/lines.go` (line splitter) | utility | transform | `internal/kernel/edit/replace.go:71-102` `rangeToByteOffsets` (line/byte offset accumulator) | role + flow match — pure byte/line accounting helper |
| `internal/fuzzy/ellipsis.go` (segment parser) | utility | transform | `internal/kernel/fileops/replace.go` (regex compile + segmentation discipline) | role match — regexp.MustCompile + slice-of-segments output |
| `internal/fuzzy/strategies.go` (4 strategy functions) | core algorithm | transform | `internal/kernel/edit/replace.go:14-69` `ReplaceBody` family (multi-strategy fallback chain) | flow match — sequential strategy cascade with early return |
| `internal/fuzzy/match.go` (Match() entry + cascade) | controller | request-response | `internal/kernel/edit/replace.go:14-23` `ReplaceBody` (top-level orchestrator delegating to helpers) | exact role — single exported entry point dispatching to internal helpers |
| `internal/fuzzy/indent.go` (dedent + reapply) | utility | transform | `internal/kernel/edit/replace.go:71-102` `rangeToByteOffsets` (pure byte arithmetic helper) | role match — pure string transform, no I/O |
| `internal/fuzzy/diff.go` (fail-with-diff payload) | utility | transform | `internal/errors/errors.go:51-56` `Error()` formatter (pure string assembly via fmt) | role match — string-builder formatting only |
| `internal/fuzzy/fuzzy_test.go` | test | — | `internal/kernel/edit/edit_test.go:11-57` `TestRangeToByteOffsets` (table-driven, t.Run, testify) | exact match |
| `internal/fuzzy/strategies_test.go` | test | — | `internal/errors/errors_test.go:15-31` `TestKinds` (testify table-driven) | exact match |
| `internal/fuzzy/ellipsis_test.go` | test | — | `internal/kernel/edit/edit_test.go:11-57` (table + t.Run) | exact match |
| `internal/fuzzy/indent_test.go` | test | — | `internal/kernel/edit/edit_test.go:11-57` | exact match |
| `internal/fuzzy/diff_test.go` | test | — | `internal/errors/errors_test.go:61-70` `TestErrorString` (golden-string subtests) | exact match |

## Pattern Assignments

### `internal/fuzzy/types.go` (types, transform)

**Analog:** `internal/kernel/edit/planner.go` lines 13-30
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/kernel/edit/planner.go`

**Const-enum pattern** (lines 13-20):
```go
// EditType enumerates the kinds of edits the planner can produce.
const (
    EditTypeReplaceBody  = "replace_body"
    EditTypeInsertBefore = "insert_before"
    EditTypeInsertAfter  = "insert_after"
    EditTypeRename       = "rename"
    EditTypeDelete       = "delete"
)
```

**Plain-struct result pattern** (lines 22-30):
```go
// EditPlan describes a resolved edit ready for execution.
type EditPlan struct {
    URI            string
    SymbolName     string
    EditType       string    // one of EditType* constants
    Range          gen.Range // computed edit range (full symbol extent)
    SelectionRange gen.Range // identifier range (for cursor positioning)
    NewContent     string    // content to insert/replace
}
```

**Apply to fuzzy/types.go:** Use the same shape — typed string for `Strategy` (RESEARCH §Code Examples line 494 already shows `type Strategy string` + 4 typed consts: `StrategyExact`, `StrategyWhitespace`, `StrategyIndentationFlex`, `StrategyFailed`). `Result` and `Options` mirror the `EditPlan` shape — plain exported struct, godoc on type and on every exported field, no methods, no constructor.

**Godoc style** (planner.go:13, 22 — also reflected in errors.go:8-22 for the more detailed type-doc style):
- Single-line doc: `// EditType enumerates ...`
- Multi-line doc with paragraph + critical note: see `internal/errors/errors.go` lines 8-15.

---

### `internal/fuzzy/lines.go` (utility, transform)

**Analog:** `internal/kernel/edit/replace.go` lines 71-102
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/kernel/edit/replace.go`

**Line-to-byte offset accumulator pattern** (lines 71-102):
```go
// rangeToByteOffsets converts an LSP Range to byte offsets in source.
func rangeToByteOffsets(source []byte, r gen.Range) (uint, uint) {
    lines := strings.SplitAfter(string(source), "\n")

    var startByte, endByte uint
    var currentByte uint

    for lineIdx, line := range lines {
        lineNum := uint32(lineIdx)
        lineLen := uint(len(line))

        if lineNum == r.Start.Line {
            startByte = currentByte + uint(r.Start.Character)
        }
        if lineNum == r.End.Line {
            endByte = currentByte + uint(r.End.Character)
            break
        }
        currentByte += lineLen
    }

    // Clamp to source bounds.
    srcLen := uint(len(source))
    if startByte > srcLen {
        startByte = srcLen
    }
    if endByte > srcLen {
        endByte = srcLen
    }

    return startByte, endByte
}
```

**Apply to fuzzy/lines.go:** Build `lineByteOffsets []int` by accumulating `len(line)+1` per line — same accumulator idiom but pre-computed once at top of `Match()` and indexed by line number (RESEARCH §Pitfall 3 mandates this). Use `strings.Split(s, "\n")` (NOT `SplitAfter`) per RESEARCH §Pitfall 4 — pick one convention and document in the file's package-comment-equivalent godoc.

**Stdlib-only imports header** (replace.go:1-12 vs. fileops/validate.go:1-10):
```go
package fuzzy

import (
    "strings"

    serr "github.com/postfix/serena/internal/errors"
)
```
Only `strings` + `serr`. NO `os`, NO `context`, NO `lspool`, NO `gen` — engine is pure.

---

### `internal/fuzzy/ellipsis.go` (utility, transform)

**Analog:** `internal/kernel/fileops/replace.go` lines 14-47
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/kernel/fileops/replace.go`

**Regex compile + count + branch pattern** (lines 29-47):
```go
if isRegex {
    re, err := regexp.Compile(pattern)
    if err != nil {
        return 0, serr.Wrap(serr.InvalidArgs, "invalid regex pattern", err)
    }
    // Count matches
    matches := re.FindAllStringIndex(content, -1)
    count = len(matches)
    if count == 0 {
        return 0, nil
    }
    newContent = re.ReplaceAllString(content, replacement)
}
```

**Apply to fuzzy/ellipsis.go:**
- Compile the dots-line regex once at package init: `var dotsRe = regexp.MustCompile(`(?m)^[ \t]*\.\.\.$`)` (RESEARCH §Pattern 3 line 319).
- `segmentSearch(s string) ([]string, error)` returns `nil, nil` for "no ellipsis" (caller falls through to whole-search path), or a slice of segment strings, or `serr.New(serr.InvalidArgs, "...")` for empty segments.
- Validation pattern (segment count mismatch between search and replacement) follows the same guard-clause + early-return shape: validate, return `serr.InvalidArgs` with detail, never panic.

**Anti-pattern reminder (RESEARCH §Pitfall 7):** segments must match in order against a *cursor-restricted* source slice — coordinator lives in `match.go`, not here. `ellipsis.go` is pure parse + validate, no source scanning.

---

### `internal/fuzzy/strategies.go` (core algorithm, transform)

**Analog:** `internal/kernel/edit/replace.go` lines 25-69 `ReplaceBodyWithPlan`
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/kernel/edit/replace.go`

**Strategy fallback / branching pattern** (lines 36-46):
```go
// Try tree-sitter body extraction first (D-14).
if extractor != nil && extractor.SupportsLanguage(lang) {
    startByte, endByte, err = extractor.ExtractBody(source, lang, plan.SymbolName, plan.Range)
    if err != nil {
        // Fall back to full symbol range (D-15).
        startByte, endByte = rangeToByteOffsets(source, plan.Range)
    }
} else {
    // No tree-sitter grammar: fall back to full symbol range (D-15).
    startByte, endByte = rangeToByteOffsets(source, plan.Range)
}
```

**Bounds guard pattern** (lines 48-50):
```go
if startByte > uint(len(source)) || endByte > uint(len(source)) || startByte > endByte {
    return serr.New(serr.Internal, fmt.Sprintf("invalid byte range [%d:%d] for file of length %d", startByte, endByte, len(source)))
}
```

**Apply to fuzzy/strategies.go:**
- One unexported function per strategy: `sweepExact`, `sweepWhitespace`, `sweepIndentFlex`. Each returns `[]int` of starting line indices (per RESEARCH §Pattern 1, lines 234-253):

```go
func sweepExact(whole, part []string) []int {
    partLen := len(part)
    if partLen == 0 || partLen > len(whole) {
        return nil
    }
    var hits []int
    for i := 0; i <= len(whole)-partLen; i++ {
        match := true
        for j := 0; j < partLen; j++ {
            if whole[i+j] != part[j] {
                match = false
                break
            }
        }
        if match {
            hits = append(hits, i)
        }
    }
    return hits
}
```

- The whitespace and indent-flex sweeps differ ONLY in the comparator. RESEARCH §Pitfall 1 mandates the `i <= len(whole)-partLen` upper bound.
- **No `serr` calls in this file** — strategies return `[]int` only. Cascade orchestrator in `match.go` translates 0/1/N counts into Result-or-error.

---

### `internal/fuzzy/match.go` (controller, request-response)

**Analog:** `internal/kernel/edit/replace.go` lines 14-23 `ReplaceBody`
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/kernel/edit/replace.go`

**Top-level orchestrator pattern** (lines 14-23):
```go
// ReplaceBody replaces a symbol's body using tree-sitter for precise body extraction.
// Per D-14: tree-sitter-first for body surgery, LSP-assisted for discovery.
// Per D-15: falls back to full symbol range replacement when tree-sitter unavailable.
func ReplaceBody(ctx context.Context, lease *lspool.WorkerLease, extractor *BodyExtractor, uri string, symbolName string, newBody string, lang string) error {
    plan, err := PlanEdit(ctx, lease, uri, symbolName, EditTypeReplaceBody, newBody)
    if err != nil {
        return serr.Wrap(serr.Internal, "plan edit", err)
    }
    return ReplaceBodyWithPlan(ctx, lease, extractor, plan, lang)
}
```

**Empty-input guard pattern** (planner.go:41-43):
```go
if outline == nil {
    return nil, serr.New(serr.NotFound, "symbol not found").WithDetail(symbolName)
}
```

**Apply to fuzzy/match.go:**
- `Match(source, search string, opts Options) (*Result, error)` is the only exported function; godoc must mention "pure, no I/O" and reference Phase 25 + the 4-strategy cascade.
- Top-of-function guard chain (RESEARCH §Pitfall 2):
  ```go
  if strings.TrimSpace(search) == "" {
      return nil, serr.New(serr.InvalidArgs, "empty search block")
  }
  ```
- Cascade body: call `sweepExact` → branch on len(hits) `0/1/N`; on N return ambiguity error; on 1 build Result via `indent.Reflow`; on 0 fall through to `sweepWhitespace`, etc. Final fall-through builds the diff-failure error via `formatFailureDiff`.
- **Per-strategy ambiguity returns immediately, does not cascade** — CONTEXT.md §4 is explicit. Encode this as 4 separate `if`/`switch` blocks rather than a loop, so readers can see the locked behavior.

---

### `internal/fuzzy/indent.go` (utility, transform)

**Analog:** `internal/kernel/edit/replace.go` lines 53-56 (byte-slice rebuild) + `internal/errors/errors.go` lines 51-56 (pure string assembly)

**Byte-slice rebuild pattern** (replace.go:53-56):
```go
// Replace bytes from startByte to endByte with newBody.
var result []byte
result = append(result, source[:startByte]...)
result = append(result, []byte(plan.NewContent)...)
result = append(result, source[endByte:]...)
```

**Apply to fuzzy/indent.go:**
- Three pure helpers: `commonLeadingPrefix(lines []string) string`, `dedent(lines []string, prefix string) []string`, `reapplyPrefix(lines []string, prefix string) []string`.
- All operate on `[]string` slices, allocate new slices, never mutate input — same "build new, leave original alone" discipline as replace.go:53-56.
- Empty/whitespace-only line guard for `reapplyPrefix` (RESEARCH §Pitfall 6):
  ```go
  if strings.TrimSpace(line) == "" {
      out = append(out, "")
      continue
  }
  out = append(out, prefix+line)
  ```
- Document the Aider deviation in a top-of-function comment block (per RESEARCH §Pattern 2, "Deviation from Aider, documented") — use the same comment-paragraph style as `replace.go` lines 14-16.

---

### `internal/fuzzy/diff.go` (utility, transform)

**Analog:** `internal/errors/errors.go` lines 51-56
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/errors/errors.go`

**Pure fmt-based string assembly pattern** (lines 51-56):
```go
// Error returns a human-readable string: "kind: message" or
// "kind: message (detail)" when Detail is non-empty.
func (e *Error) Error() string {
    if e.Detail != "" {
        return fmt.Sprintf("%s: %s (%s)", e.Kind, e.Message, e.Detail)
    }
    return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}
```

**Apply to fuzzy/diff.go:**
- `formatFailureDiff(searchLines, sourceLines []string, nearestStart int) string` — pure formatter, returns the unified-diff-style payload string. Use `strings.Builder` (RESEARCH §Standard Stack confirms this is the project idiom for hot-path string building).
- **No `serr` construction in this file** — `diff.go` returns a *string*; `match.go` is the one that wraps it as `serr.New(serr.InvalidArgs, "no match found").WithDetail(diffPayload)`. Keeps `diff.go` testable as a pure string function.
- Hand-roll the `--- search` / `+++ nearest` / `@@ -1,N +1,M @@` headers per RESEARCH §"Don't Hand-Roll" table — no `go-difflib` import.

**Ambiguity message format** (RESEARCH §Pitfall 5 — also belongs in this file as a sibling formatter, e.g. `formatAmbiguity(hits []int) string`):
```go
shown := hits
suffix := ""
if len(hits) > 5 {
    shown = hits[:5]
    suffix = fmt.Sprintf(", ...and %d more", len(hits)-5)
}
```
1-indexed line numbers in the message (CONTEXT.md §4 mandate).

---

### `internal/fuzzy/fuzzy_test.go` (test)

**Analog:** `internal/kernel/edit/edit_test.go` lines 11-57 `TestRangeToByteOffsets`
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/kernel/edit/edit_test.go`

**Table-driven + t.Run + testify pattern** (lines 11-57):
```go
func TestRangeToByteOffsets(t *testing.T) {
    source := []byte("line 0\nline 1\nline 2\n")

    tests := []struct {
        name      string
        r         gen.Range
        wantStart uint
        wantEnd   uint
    }{
        {
            name: "first line",
            r: gen.Range{
                Start: gen.Position{Line: 0, Character: 0},
                End:   gen.Position{Line: 0, Character: 6},
            },
            wantStart: 0,
            wantEnd:   6,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            start, end := rangeToByteOffsets(source, tt.r)
            assert.Equal(t, tt.wantStart, start)
            assert.Equal(t, tt.wantEnd, end)
        })
    }
}
```

**Imports header** (edit_test.go:1-9):
```go
package fuzzy

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**Apply to fuzzy_test.go:** Top-level table-driven `TestMatch` covering each requirement from ROADMAP Phase 25 — one row per: exact hit, whitespace hit, indent-flex hit, ambiguity refusal, failure with diff, ellipsis success, ellipsis empty-segment rejection, indent inheritance.

**Error matching pattern** for ambiguity/failure tests — copy from `internal/kernel/fileops/fileops_test.go` lines 21-23:
```go
if !errors.Is(err, serr.ErrInvalidArgs) {
    t.Fatalf("expected InvalidArgs error, got: %v", err)
}
```
Or testify equivalent: `assert.ErrorIs(t, err, serr.ErrInvalidArgs)` (used at `internal/errors/errors_test.go:74`).

---

### `internal/fuzzy/strategies_test.go` (test)

**Analog:** `internal/errors/errors_test.go` lines 15-31 `TestKinds`
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/errors/errors_test.go`

**Compact table for pure functions** (lines 15-31):
```go
func TestKinds(t *testing.T) {
    tests := []struct {
        kind serr.Kind
        str  string
    }{
        {serr.NotFound, "not_found"},
        // ...
    }
    for _, tt := range tests {
        assert.Equal(t, tt.str, string(tt.kind), "Kind %q should have string value %q", tt.kind, tt.str)
    }
}
```

**Apply to strategies_test.go:** One `TestSweepExact`, `TestSweepWhitespace`, `TestSweepIndentFlex` each with a table of `(whole, part, wantHits)` rows. RESEARCH §Pitfall 1 requires explicit row for the off-by-one upper-bound case: 1-line search at end-of-file. RESEARCH §Pitfall 2 requires explicit row for empty-part returning empty hits.

---

### `internal/fuzzy/ellipsis_test.go` (test)

**Analog:** `internal/kernel/edit/edit_test.go` lines 11-57

**Apply:** Table-driven `TestSegmentSearch` with rows for: no-ellipsis (returns nil, nil), single-ellipsis 2-segment, multi-ellipsis N-segment, empty-segment rejection (empty leading, empty trailing, empty middle), inline `...` literal preserved, leading-whitespace before dots tolerated. Use `assert.ErrorIs(t, err, serr.ErrInvalidArgs)` for rejection rows.

---

### `internal/fuzzy/indent_test.go` (test)

**Analog:** `internal/kernel/edit/edit_test.go` lines 11-57

**Apply:** Table-driven `TestCommonLeadingPrefix`, `TestDedent`, `TestReapplyPrefix`. Required edge-case rows from RESEARCH §Pitfall 6 + CONTEXT.md §5: tabs only, spaces only, mixed tabs/spaces, empty lines preserved as `""` (no trailing whitespace), single-line input, all-blank input.

---

### `internal/fuzzy/diff_test.go` (test)

**Analog:** `internal/errors/errors_test.go` lines 61-70 `TestErrorString`
**File:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/errors/errors_test.go`

**Golden-string subtests pattern** (lines 61-70):
```go
func TestErrorString(t *testing.T) {
    t.Run("without detail", func(t *testing.T) {
        err := serr.New(serr.NotFound, "msg")
        assert.Equal(t, "not_found: msg", err.Error())
    })
    t.Run("with detail", func(t *testing.T) {
        err := serr.New(serr.NotFound, "msg").WithDetail("detail")
        assert.Equal(t, "not_found: msg (detail)", err.Error())
    })
}
```

**Apply to diff_test.go:**
- `TestFormatFailureDiff` with golden expected strings for the unified-diff payload.
- `TestFormatAmbiguity` with golden strings for: 1 hit (shouldn't be called but defensive), 2 hits, exactly 5 hits (no `...and N more` suffix), 6 hits (`...and 1 more`), 10 hits (`...and 5 more`) — RESEARCH §Pitfall 5 explicitly demands this.

## Shared Patterns

### Error Construction (`serr`)

**Source:** `internal/errors/errors.go` lines 25-47 + `internal/kernel/edit/planner.go:42`, `internal/kernel/fileops/replace.go:32`
**Apply to:** `match.go` (cascade error returns), `ellipsis.go` (validation errors). NOT in `strategies.go` / `indent.go` / `diff.go` / `lines.go` — those are pure helpers that return values, not errors.

**Construction style** (the canonical project idiom — appears identically in 12+ places):
```go
return nil, serr.New(serr.InvalidArgs, "ambiguous match").WithDetail(detailString)
```

**Wrap style** when an inner cause exists:
```go
return nil, serr.Wrap(serr.InvalidArgs, "invalid regex pattern", err)
```

**Sentinel matching in tests** (`internal/errors/errors_test.go:74`, `internal/kernel/fileops/fileops_test.go:21-23`):
```go
assert.ErrorIs(t, err, serr.ErrInvalidArgs)
```

**Critical:** per `internal/errors/errors.go` lines 11-16 godoc — never assign a typed `*Error` nil and return it; always `return nil, nil` for the success path of helpers that may produce serr errors.

### Import Alias Convention

**Source:** `internal/errors/kinds.go` package doc lines 1-6
**Apply to:** every file in `internal/fuzzy/` that imports the errors package
```go
import (
    serr "github.com/postfix/serena/internal/errors"
)
```
Always alias as `serr` to avoid shadowing stdlib `errors`. Required by package-doc convention.

### Package Doc Comment

**Source:** `internal/kernel/edit/treesitter.go` line 1, `internal/errors/kinds.go` lines 1-6
**Apply to:** Exactly one file in `internal/fuzzy/` (recommend `match.go` since it holds the entry point) carries the `// Package fuzzy ...` doc:
```go
// Package fuzzy implements a pure line-based fuzzy text matcher with a
// 4-strategy cascade (exact, whitespace-normalized, indentation-flexible,
// fail-with-diff), ellipsis segmentation, and Aider-style indentation
// reflow. It performs no I/O — callers read source, invoke Match, and
// decide whether to write.
package fuzzy
```

### Pure-Package Discipline

**Source:** `internal/errors/` is the closest comparable pure package — zero `init()`, zero global state apart from the sentinel vars in `kinds.go:26-34`, zero I/O.
**Apply to:** `internal/fuzzy/` follows the same discipline. The single allowed package-level state is `var dotsRe = regexp.MustCompile(...)` in `ellipsis.go` (compiled-once regex — same idiom as `internal/kernel/fileops/replace.go:31` `regexp.Compile`, just hoisted to package level since the pattern is constant).

### Table-Driven Tests with Subtests

**Source:** `internal/kernel/edit/edit_test.go:11-57` `TestRangeToByteOffsets`
**Apply to:** All 5 test files. Standard shape:
1. Anonymous struct slice with named `name` field
2. `for _, tt := range tests { t.Run(tt.name, func(t *testing.T) { ... }) }`
3. Use `require` for setup assertions that block the test, `assert` for value comparisons.
4. Use `assert.ErrorIs(t, err, serr.ErrInvalidArgs)` for typed-error checks (NOT string contains).

## No Analog Found

None — every file in this phase has at least a role-match analog already in the repo. The fuzzy package is greenfield in *content* but not in *style*; every pattern it needs is already idiomatic in `internal/errors/`, `internal/kernel/edit/`, or `internal/kernel/fileops/`.

## Metadata

**Analog search scope:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/errors/`, `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/kernel/edit/`, `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/internal/kernel/fileops/`
**Files scanned:** errors.go, kinds.go, errors_test.go, replace.go (edit), planner.go, treesitter.go, edit_test.go, replace.go (fileops), validate.go, fileops_test.go
**Pattern extraction date:** 2026-04-15
**Pre-existing fuzzy package:** none (verified `ls internal/fuzzy` → does not exist)

## PATTERN MAPPING COMPLETE
