# Phase 25: Fuzzy Edit Engine - Research

**Researched:** 2026-04-15
**Domain:** Line-based fuzzy text matching + indentation reflow in Go (pure library)
**Confidence:** HIGH

## Summary

Phase 25 builds `internal/fuzzy/`, a self-contained, pure (no I/O) line-based fuzzy
matcher with a 4-strategy cascade (exact → whitespace-normalized → indentation-flexible
→ fail-with-diff), ellipsis segmentation, and Aider-style common-prefix indentation
reflow. The algorithm is well-understood — Aider's `editblock_coder.py` is the canonical
Python reference and all required primitives are a straight line-by-line port into Go.
The external surface area is tiny: one `Match(source, search string, opts Options)`
entry point returning a `*Result` plus `error`.

No new external dependencies are needed. The only nontrivial question — how to format
the fail-with-diff payload — resolves to hand-rolling a minimal unified-diff-style
message because (a) `github.com/pmezard/go-difflib` is already transitively present
(BSD-3, but unmaintained since 2016 and currently `// indirect`), and (b) for a few
dozen lines of "here's what you searched for vs. the nearest N lines in the file" we
don't need its line-sequence matcher — we already know both halves and only need to
emit them with `---/+++/@@` headers. Stdlib (`strings`, `bufio`, `fmt.Fprintf`) covers it.

**Primary recommendation:** Port `editblock_coder.py`'s perfect_replace + dotdotdot
handling + replace_part_with_missing_leading_whitespace + match_but_for_leading_whitespace
into four separate Go functions behind a thin cascade orchestrator. Add one deviation
from Aider — a TrimSpace-per-line strategy between exact and indentation-flexible — and
tighten segment semantics per CONTEXT.md decisions. Diff payload: hand-rolled unified
format, no dependency.

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **Package location:** `internal/fuzzy/` — standalone, pure (no file I/O), zero deps
  on `internal/kernel/edit/` or `internal/kernel/fileops/`.
- **Primary entry point:** `func Match(source, search string, opts Options) (*Result, error)`.
- **Result fields:** `Strategy` (enum), `Score` (float64), `StartByte`, `EndByte`,
  `MatchedText`, `ReplacementText` (indentation-reflowed).
- **Options fields (initial):** `Replacement string`, `AllowEllipsis bool`.
- **4-strategy cascade, line-based:**
  1. Exact byte-for-byte line equality
  2. Whitespace-normalized — `strings.TrimSpace` each line before compare
  3. Indentation-flexible — `strings.TrimLeft(line, " \t")`
  4. Fail-with-diff → return `serr.InvalidArgs` with unified-diff-style payload
- **Discrete similarity tiers:** 1.0 / 0.95 / 0.85 / 0.0 — no real computation.
- **match_strategy enum:** `"exact" | "whitespace_normalized" | "indentation_flexible" | "failed"`.
- **Ellipsis marker:** literal `...` on its own line (leading whitespace ignored),
  splits search into segments; each segment must match in order in source with
  arbitrary lines allowed between; multiple `...` lines allowed; empty segments
  rejected; inline `...` is literal text (not a marker).
- **Replacement with ellipsis:** replacement block must use the SAME segmentation
  pattern (same count of `...` in matching positions).
- **Ambiguity (N > 1):** return `serr.InvalidArgs` immediately at that strategy,
  do NOT cascade. Error detail lists 1-indexed line numbers capped at 5 with
  `"...and N more"` overflow.
- **Indentation reflow:** common-prefix dedent + reapply matched region's leading
  bytes. Tab = 1 character, no expansion. Empty/whitespace-only replacement lines
  stay empty (no prefix added).
- **Error type:** `serr.InvalidArgs` for both ambiguity and fail-with-diff.

### Claude's Discretion

- Internal shape of helper functions, private type names, test organization.
- Exact wording of error messages within the locked format.
- Whether the cascade orchestrator is a single function or a slice of strategy
  closures.
- Hand-roll vs library for the fail-with-diff payload (research recommends hand-roll).

### Deferred Ideas (OUT OF SCOPE)

- **FUZZ-09** — configurable `MinScore` per call (future requirement, not this phase).
- All of Phase 26: standalone `fuzzy_edit` MCP tool, wiring into
  `replace_symbol_body` / `replace_content`, any edits to existing callers.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FUZZ-01 | 4-strategy cascade: exact → whitespace-normalized → indentation-flexible → fail-with-diff | Aider port — §Standard Stack, §Architecture Patterns |
| FUZZ-02 | Tool response reports match_strategy + similarity_score | Discrete tiers decided in CONTEXT; no computation needed |
| FUZZ-03 | Replacement inherits file's indentation on fuzzy match | `replace_part_with_missing_leading_whitespace` + `match_but_for_leading_whitespace` port — §Code Examples |
| FUZZ-07 | Refuse ambiguous edits (N > 1 matches) | `perfect_replace` sweep returns all hit offsets; count > 1 → serr.InvalidArgs — §Pitfalls |
| FUZZ-08 | Ellipsis `...` placeholder in search blocks | `try_dotdotdots` port with CONTEXT-specified deviations — §Code Examples |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Line splitting and normalization | `internal/fuzzy/` (pure) | — | Leaf utility, no callers crossed |
| 4-strategy match cascade | `internal/fuzzy/` | — | Self-contained reasoning over in-memory strings |
| Ellipsis segmentation | `internal/fuzzy/` | — | Text-only pre-processing, no tree-sitter |
| Indentation reflow | `internal/fuzzy/` | — | Operates on already-located byte range |
| Ambiguity detection | `internal/fuzzy/` | — | Just a count-of-matches check per strategy |
| Fail-with-diff payload | `internal/fuzzy/` | — | Formatting of already-known strings |
| File read/write | NOT in this phase | `internal/kernel/edit/`, `internal/kernel/fileops/` (Phase 26) | Engine stays pure |
| MCP tool registration | NOT in this phase | daemon bootstrap (Phase 26) | Engine has no protocol awareness |
| Tree-sitter body bounds | NOT in this phase | `internal/kernel/edit/treesitter.go` (Phase 26) | Engine is line-based text only |

**Sanity check:** every capability lands in `internal/fuzzy/`. If a plan suggests
touching `edit/replace.go`, `fileops/replace.go`, daemon, or any MCP registration,
it belongs in Phase 26 — reject it.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `strings` (stdlib) | Go 1.25 | Split / TrimSpace / TrimLeft / Contains / ReplaceAll | [VERIFIED: stdlib] Line-based work is exactly what stdlib `strings` is for — zero dep cost. |
| `fmt` (stdlib) | Go 1.25 | Error message formatting, diff header rendering | [VERIFIED: stdlib] Used by every other package in the repo. |
| `strings.Builder` (stdlib) | Go 1.25 | Efficient string assembly for diff payload and replacement text | [VERIFIED: stdlib] Standard idiom for string building in hot paths. |
| `github.com/postfix/serena/internal/errors` (serr) | internal | Return `serr.InvalidArgs` with structured detail | [VERIFIED: internal/errors/errors.go read this session] v1.5 locked this as the error convention for all tools. |
| `github.com/stretchr/testify/assert` + `require` | already in go.mod | Unit test assertions | [VERIFIED: used in internal/errors/errors_test.go, internal/kernel/edit/edit_test.go] project-wide standard. |

### Supporting

None required. The engine has no runtime deps beyond stdlib + serr.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled unified diff | `github.com/pmezard/go-difflib/difflib` | [VERIFIED: go.sum] Already present transitively at v1.0.0 (BSD-3). But [VERIFIED: `go mod why`] it's `// indirect` and the main module does not need it; adding a direct import would promote it. [CITED: pkg.go.dev/github.com/pmezard/go-difflib/difflib] Last release 2016-01-10, not actively maintained. We only need to emit `---/+++/@@` headers around known "searched" and "nearest" blocks — no line-sequence matcher needed. **Recommend hand-roll.** |
| Hand-rolled unified diff | `github.com/sergi/go-diff/diffmatchpatch` | [ASSUMED based on training] Character-level DMP, heavier, overkill for our line-based output. Not in go.sum currently — would be a new direct dep. **Reject.** |
| Discrete 4-tier similarity | Levenshtein / ratcliff-obershelp | CONTEXT.md locked discrete tiers. Real similarity adds code for zero agent benefit — the tier values already tell the agent "how drifted was the match." **Locked out.** |
| Byte-indexed scanning | Rune-aware scanning | Search blocks and source files are Go source (ASCII indentation dominant). Tab = 1 char, no expansion is locked. Bytes are correct. **Bytes.** |

**Installation:** no new deps. No `go get` needed.

**Version verification:**
```bash
go version        # expect go1.25.x (matches current repo)
grep pmezard go.sum   # confirms go-difflib v1.0.0 already indirect
```
[VERIFIED: ran grep this session] `github.com/pmezard/go-difflib v1.0.0 // indirect` in go.mod.

## Architecture Patterns

### System Architecture Diagram

```
              ┌─────────────────────────────────────────┐
              │  Caller (Phase 26: edit/fileops)        │
              │  - reads file into `source string`      │
              │  - builds Options{ Replacement, ... }   │
              └──────────────────┬──────────────────────┘
                                 │ fuzzy.Match(source, search, opts)
                                 ▼
        ┌────────────────────────────────────────────────────┐
        │                internal/fuzzy/                     │
        │                                                    │
        │  1. Pre-processing                                 │
        │     ├─ split source / search / replacement on '\n' │
        │     ├─ if AllowEllipsis && search has '...' lines: │
        │     │    segment search + validate replacement     │
        │     │    has matching segment count                │
        │     │    (each segment recursively routed into     │
        │     │    the strategy loop below)                  │
        │     └─ compute common-prefix dedent of search +    │
        │        replacement (once, shared by strategies 3)  │
        │                                                    │
        │  2. Strategy cascade (stop at first hit or         │
        │     ambiguity)                                     │
        │     ├─ Exact (tier 1.0)                            │
        │     │    ├─ 0 hits → next strategy                 │
        │     │    ├─ 1 hit  → reflow + return Result        │
        │     │    └─ N hits → serr.InvalidArgs (stop)       │
        │     │                                              │
        │     ├─ Whitespace-normalized (tier 0.95)           │
        │     │    ├─ TrimSpace per line on search + window  │
        │     │    └─ same 0/1/N logic                       │
        │     │                                              │
        │     ├─ Indentation-flexible (tier 0.85)            │
        │     │    ├─ TrimLeft(" \t") per line               │
        │     │    └─ same 0/1/N logic                       │
        │     │                                              │
        │     └─ Fail (tier 0.0)                             │
        │          └─ format unified-diff payload of         │
        │             search vs. nearest source window →     │
        │             serr.InvalidArgs                       │
        │                                                    │
        │  3. Post-processing (only on hit)                  │
        │     ├─ compute MatchedText (source[start:end])     │
        │     ├─ derive sourcePrefix (leading bytes of       │
        │     │  matched region's first line)                │
        │     ├─ dedent replacement by searchPrefix          │
        │     ├─ reapply sourcePrefix to each non-empty      │
        │     │  replacement line                            │
        │     └─ build Result{ Strategy, Score, StartByte,   │
        │         EndByte, MatchedText, ReplacementText }    │
        └────────────────────────────────────────────────────┘
                                 │
                                 ▼
                    *Result, error (serr.InvalidArgs on fail)
```

Data flow: Caller → Match → pre-process → cascade → (optional reflow) → Result.
No side-effects; caller owns all I/O.

### Recommended Project Structure

```
internal/fuzzy/
├── fuzzy.go              # exported Match(), Result, Options, Strategy enum
├── lines.go              # splitLines, joinLines, trim helpers
├── ellipsis.go           # segment detection/validation, segment dispatcher
├── strategies.go         # exactMatch, whitespaceMatch, indentFlexMatch (all line-based)
├── indent.go             # commonLeadingPrefix, dedent, reapplyPrefix
├── diff.go               # formatFailureDiff (hand-rolled unified output)
├── errors.go             # ambiguityError, failureError — thin serr.InvalidArgs builders
├── fuzzy_test.go         # top-level Match() table-driven tests
├── strategies_test.go    # per-strategy unit tests
├── ellipsis_test.go      # segmentation edge cases
├── indent_test.go        # tabs/spaces/mixed, empty-line preservation
└── diff_test.go          # failure payload format golden checks
```

**Why this split:** each file maps to a planner task roughly 1-to-1 (see Notes for
Planner in CONTEXT.md which already proposed this 7-task breakdown). Keeping
`fuzzy.go` to only the exported surface makes Phase 26 integration trivial.

### Pattern 1: Line sweep with match-count detection

**What:** Port of Aider's `perfect_replace` — O(N*M) window sweep, collect **all**
starting offsets that match, then branch on count.
**When to use:** all three success strategies (exact, whitespace, indentation-flexible).
**Example:**

```go
// Source: ported from Aider editblock_coder.py lines 73-82
// [CITED: github.com/Aider-AI/aider/blob/main/aider/coders/editblock_coder.py]
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

Caller then:
```go
hits := sweepExact(sourceLines, searchLines)
switch len(hits) {
case 0:
    // fall through to next strategy
case 1:
    // build Result at hits[0]
default:
    // return ambiguityError(hits, "exact")
}
```

The whitespace and indentation-flex strategies use the same sweep shape; the only
difference is the comparator (`strings.TrimSpace(a) == strings.TrimSpace(b)` or
`strings.TrimLeft(a, " \t") == strings.TrimLeft(b, " \t")`).

### Pattern 2: Common-prefix dedent + reapply (indentation preservation)

**What:** Aider's `replace_part_with_missing_leading_whitespace` + `match_but_for_leading_whitespace`.
**When to use:** always in post-processing — even exact matches go through reflow so
the caller gets a consistently-formatted `ReplacementText`. For exact matches,
searchPrefix == sourcePrefix so reflow is a no-op; for whitespace / indentation-flex
strategies it reapplies the matched region's actual leading bytes.

**Algorithm (from CONTEXT.md, verified against Aider):**
1. `searchPrefix` = longest common leading-whitespace byte prefix across all
   non-empty lines of the search block.
2. Strip `searchPrefix` from every line of the search block (shared with strategies 2/3).
3. Strip the same number of leading bytes from every non-empty line of the
   replacement block. Empty/whitespace-only lines are left unchanged.
4. `sourcePrefix` = leading whitespace bytes of the **first line of the matched
   source region**.
5. Prefix every non-empty line of the dedented replacement with `sourcePrefix`;
   leave empty lines empty.

**Deviation from Aider, documented:** Aider's `match_but_for_leading_whitespace`
(editblock_coder.py lines 223-244) collects the set of whitespace offsets across
ALL matched source lines and rejects the match if more than one distinct offset
exists (`if len(add) != 1: return`). Our decision is simpler: **use only the first
matched line's leading bytes** as `sourcePrefix`. This is more permissive — a match
succeeds even when the matched region contains non-uniform indentation (e.g., a
function body with nested blocks), which is the common case. Document this
deviation explicitly in code comments.

### Pattern 3: Ellipsis segmentation

**What:** Port of Aider's `try_dotdotdots` (lines 130-176) with CONTEXT-specified
relaxations.
**Aider's rule:** exact line regex `(^\s*\.\.\.\n)` (leading whitespace allowed,
dots on their own line). **Same as ours.**
**Aider's segment matching:** each segment must appear **exactly once** in the
whole (`whole.count(part) == 1`); pairs applied via `whole.replace(part, replace, 1)`.
**Our deviation (CONTEXT.md):** segments must match **in order** in the source,
with arbitrary lines allowed between. This is strictly looser than Aider.
**Our anchor requirement:** every segment must contain ≥ 1 non-empty line
(rejects empty segments, which would otherwise match anywhere). Aider implicitly
handles this via the count-once rule.

**Implementation path:**
```go
// Regex: (?m)^\s*\.\.\.\n  — matches a dots line, allowing Unix leading ws.
// Use regexp.MustCompile once at package init.
var dotsRe = regexp.MustCompile(`(?m)^[ \t]*\.\.\.$`)

func segmentSearch(search string) ([]string, error) {
    // Split on dots-only lines. regexp.Split gives segments between separators.
    segments := dotsRe.Split(search, -1)
    if len(segments) == 1 {
        return nil, nil // no ellipsis; caller uses whole-search path
    }
    for i, seg := range segments {
        if strings.TrimSpace(seg) == "" {
            return nil, fmt.Errorf("empty segment at position %d", i)
        }
    }
    return segments, nil
}
```

**Replacement segmentation check:** call `segmentSearch` on both search and
replacement; if counts differ → `serr.InvalidArgs` ("ellipsis segment mismatch:
search has N, replacement has M"). Each `(searchSegment, replaceSegment)` pair
is then dispatched through the same strategy cascade against a source *cursor*
that advances past each segment's match end, enforcing in-order matching.

### Anti-Patterns to Avoid

- **Running strategies in parallel goroutines.** Cascade is sequential by design
  — we need to stop on ambiguity. Adds zero wall-clock benefit for typical search
  blocks (tens of lines, microsecond runtime).
- **Real similarity scoring.** CONTEXT.md locks discrete tiers. Don't import
  Levenshtein. Don't compute edit distance. Don't "improve" the tiers.
- **Rune-aware indentation counting.** Tab = 1 char is locked. Byte counting is
  correct and faster.
- **Mutating `source` or `search` strings.** Engine is pure. Allocate and return
  new strings; caller owns originals.
- **Normalizing internal whitespace (collapsing multiple spaces inside a line).**
  CONTEXT.md is explicit: TrimSpace per line collapses **leading/trailing** only.
  Internal whitespace inside a line must match byte-for-byte in strategies 1 and 2.
- **Using go-difflib for the failure payload.** See §Standard Stack alternatives.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Structured error with kind + detail | Custom error type | `serr.InvalidArgs` via `serr.New(serr.InvalidArgs, "msg").WithDetail(...)` | v1.5 taxonomy is locked; Phase 26 consumers check `errors.Is(err, serr.ErrInvalidArgs)` |
| Line splitting | Manual byte loop | `strings.Split(s, "\n")` | [VERIFIED: stdlib] Correct for our `tab=1 char, no expansion` rule |
| Whitespace trimming | Manual char loop | `strings.TrimSpace`, `strings.TrimLeft` | [VERIFIED: stdlib] Already handles tab + space |
| Test table scaffolding | Bespoke test helpers | testify `assert` / `require` + plain `[]struct{}` tables | [VERIFIED: internal/errors/errors_test.go, internal/kernel/edit/edit_test.go] Project convention |

| Problem | DO Hand-Roll | Why |
|---------|-------------|-----|
| Unified-diff failure payload | Yes | Only needs to emit `--- search`, `+++ nearest`, `@@ -1,N +1,M @@`, and the two line blocks. <40 lines of Go. Avoids promoting an unmaintained indirect dep. |
| Line-window nearest-neighbor selection | Yes | Simple: pick the source window of same length as search with the most exactly-equal lines (or zero if none). Don't need diff-match-patch for this. |

**Key insight:** the engine is small. Every character of external dep is cost,
and the only candidate library (`go-difflib`) is unmaintained and only needed
for a trivial output-formatting job. Aider's own algorithms are <100 lines of
Python; our Go port will be ~250 lines including comments.

## Runtime State Inventory

**Nothing found in any category:** This is a greenfield package with no existing
state, no stored data, no live services, no OS registrations, no secrets, no
build artifacts. Verified by:

| Category | Check | Result |
|----------|-------|--------|
| Stored data | `ls internal/fuzzy` | Directory does not exist |
| Live service config | Engine has no service surface | N/A |
| OS-registered state | Engine has no OS integration | N/A |
| Secrets/env vars | No config consumed | N/A |
| Build artifacts | `grep -r fuzzy internal/` | No files match |

**Section applies primarily to rename/refactor phases;** included here to record
that Phase 25 has zero migration surface.

## Common Pitfalls

### Pitfall 1: Off-by-one in sweep upper bound
**What goes wrong:** `for i := 0; i < len(whole) - partLen; i++` misses the final
valid window by 1.
**Why it happens:** Subtraction overflow intuition carried from C.
**How to avoid:** Use `i <= len(whole) - partLen`. Guard `partLen > len(whole)`
with an early return of empty hits.
**Warning signs:** Tests pass for 2-line search but fail for 1-line search at
end-of-file.

### Pitfall 2: Empty search block
**What goes wrong:** `len(part) == 0` makes the sweep loop run `len(whole)+1`
times, returning every offset — every test becomes "ambiguous".
**Why it happens:** Callers might pass `""` if a symbol has no body.
**How to avoid:** Top of `Match()`: if `strings.TrimSpace(search) == ""` → return
`serr.InvalidArgs` with message "empty search block".
**Warning signs:** Every call to `Match("", ...)` returns ambiguity errors.

### Pitfall 3: Byte offsets vs line offsets
**What goes wrong:** Returning line numbers where callers expect byte offsets
(or vice versa). Phase 26 `replace.go` uses `startByte`/`endByte` for
`source[startByte:endByte]` slicing (verified in
`internal/kernel/edit/replace.go:54-56`).
**Why it happens:** Easy to confuse while porting line-based algorithms.
**How to avoid:** `Result.StartByte` and `Result.EndByte` are byte offsets into
the **original** `source` string, computed by accumulating `len(line)+1` (for the
trailing `\n`) across line indices. Maintain a `lineByteOffsets []int` table
built once at the top of `Match` — indexable as `lineByteOffsets[matchedLineIdx]`.
**Warning signs:** Phase 26 integration tests corrupt file contents around the
matched region.

### Pitfall 4: Trailing newline handling
**What goes wrong:** File ends with `\n`, `strings.Split(s, "\n")` yields a
trailing `""` empty element, which shifts line indices and breaks offset math.
**Why it happens:** Go's `Split` is not symmetric with `Join` when input has a
trailing delimiter.
**How to avoid:** Document convention: engine works on lines as produced by
`strings.Split(source, "\n")` — if the file ends with `\n`, the last element
is empty and participates in matching (search blocks that end with `\n` will have
the same phantom empty line). Byte offset table must include the phantom.
Alternatively, normalize: strip trailing `\n` before splitting, re-append on
rebuild. Pick one convention, document in `lines.go` package doc comment.
**Warning signs:** Match at end-of-file leaves spurious blank line; or loses
final `\n` on write.

### Pitfall 5: Ambiguity overflow message formatting
**What goes wrong:** CONTEXT.md requires cap at 5 line numbers with `"...and N more"`.
Easy to emit `"...and 0 more"` when exactly 5 matches exist.
**Why it happens:** `if len(hits) > 5` uses wrong boundary.
**How to avoid:** `shown := hits; suffix := ""; if len(hits) > 5 { shown = hits[:5]; suffix = fmt.Sprintf(", ...and %d more", len(hits)-5) }`. Test with exactly 5, 6, 10 matches.
**Warning signs:** Any ambiguity test that expects `"...and 1 more"` for 6 matches.

### Pitfall 6: Indentation reflow on empty replacement lines
**What goes wrong:** Blank line inside replacement gets `sourcePrefix` applied,
emitting `"        \n"` (trailing whitespace), which fails gofmt / lint on
committed files.
**Why it happens:** Naive "prefix every line" logic.
**How to avoid:** CONTEXT.md is explicit: empty / whitespace-only replacement
lines stay empty. Guard: `if strings.TrimSpace(line) == "" { keep empty } else { prefix + line }`.
**Warning signs:** Post-Phase-26 integration creates files that fail `gofmt -l`.

### Pitfall 7: Ellipsis segment matching drift
**What goes wrong:** Each segment is dispatched through the same cascade, and a
looser-strategy segment earlier in the source swallows anchor lines needed by
a later segment.
**Why it happens:** Cascade runs per segment without coordination.
**How to avoid:** Match segments in order with a cursor — after segment `k`
matches at source lines `[a, b]`, segment `k+1` only searches source lines
`[b+1:]`. Ambiguity is checked per-segment against the cursor-restricted source.
**Warning signs:** Test: search `foo\n...\nfoo` against source with three `foo`s
gives a surprising assignment.

### Pitfall 8: Windows line endings
**What goes wrong:** Source file has `\r\n`, search block has `\n`. Exact
strategy fails; fall through to whitespace-normalized (which strips `\r` as
part of `TrimSpace`) and reports score 0.95 on a file that's semantically
identical.
**Why it happens:** Serena supports cross-platform clients.
**How to avoid:** DOCUMENT as a known limitation. Phase 25 does NOT do CRLF
normalization — agents running on Windows hosts will see legitimate 0.95 scores
on identical content. Phase 26 may layer normalization at the call site if
needed. Add a test case pinning the current behavior so it's intentional.
**Warning signs:** Windows CI reports 0.95 scores on verbatim search blocks.

## Code Examples

### Example 1: Top-level Match entry point shape

```go
// Source: hand-authored for Phase 25, inspired by editblock_coder.py
// [CITED: github.com/Aider-AI/aider/blob/main/aider/coders/editblock_coder.py]
package fuzzy

import (
    "strings"

    serr "github.com/postfix/serena/internal/errors"
)

type Strategy string

const (
    StrategyExact             Strategy = "exact"
    StrategyWhitespace        Strategy = "whitespace_normalized"
    StrategyIndentationFlex   Strategy = "indentation_flexible"
    StrategyFailed            Strategy = "failed"
)

type Options struct {
    Replacement   string
    AllowEllipsis bool
}

type Result struct {
    Strategy        Strategy
    Score           float64
    StartByte       int
    EndByte         int
    MatchedText     string
    ReplacementText string
}

func Match(source, search string, opts Options) (*Result, error) {
    if strings.TrimSpace(search) == "" {
        return nil, serr.New(serr.InvalidArgs, "empty search block")
    }
    // ... split lines, build line-byte table, handle ellipsis, run cascade ...
}
```

### Example 2: Reflow step (port of match_but_for_leading_whitespace, relaxed)

```go
// Deviation documented: use first matched line's leading bytes, not Aider's
// set-of-one constraint across all matched lines.
// [CITED: github.com/Aider-AI/aider/blob/main/aider/coders/editblock_coder.py#L223]
func reflow(matchedRegion []string, searchPrefix, replacement string) string {
    var sourcePrefix string
    for _, line := range matchedRegion {
        if strings.TrimSpace(line) != "" {
            sourcePrefix = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
            break
        }
    }

    // Dedent replacement by searchPrefix length.
    var b strings.Builder
    lines := strings.Split(replacement, "\n")
    for i, line := range lines {
        if strings.TrimSpace(line) == "" {
            // Empty/ws-only: stay empty to avoid trailing whitespace.
        } else {
            dedent := line
            if strings.HasPrefix(line, searchPrefix) {
                dedent = line[len(searchPrefix):]
            }
            b.WriteString(sourcePrefix)
            b.WriteString(dedent)
        }
        if i < len(lines)-1 {
            b.WriteByte('\n')
        }
    }
    return b.String()
}
```

### Example 3: Ambiguity detail formatting

```go
func ambiguityError(strategy Strategy, matchLines []int) error {
    // matchLines is 1-indexed
    shown := matchLines
    suffix := ""
    if len(matchLines) > 5 {
        shown = matchLines[:5]
        suffix = fmt.Sprintf(", ...and %d more", len(matchLines)-5)
    }
    parts := make([]string, len(shown))
    for i, ln := range shown {
        parts[i] = fmt.Sprintf("%d", ln)
    }
    msg := fmt.Sprintf(
        "ambiguous match: search block matches %d locations (lines %s%s)",
        len(matchLines), strings.Join(parts, ", "), suffix,
    )
    return serr.New(serr.InvalidArgs, msg).
        WithDetail(fmt.Sprintf("strategy=%s count=%d", strategy, len(matchLines)))
}
```

### Example 4: Fail-with-diff payload (hand-rolled)

```go
// Minimal unified-diff-like rendering — not a real diff engine, just a
// labeled two-column view for the agent.
func formatFailureDiff(search string, nearestWindow string) string {
    var b strings.Builder
    b.WriteString("--- search\n")
    b.WriteString("+++ nearest source region\n")
    b.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n",
        strings.Count(search, "\n")+1,
        strings.Count(nearestWindow, "\n")+1))
    for _, line := range strings.Split(search, "\n") {
        b.WriteString("-")
        b.WriteString(line)
        b.WriteByte('\n')
    }
    for _, line := range strings.Split(nearestWindow, "\n") {
        b.WriteString("+")
        b.WriteString(line)
        b.WriteByte('\n')
    }
    return b.String()
}

func failureError(search, nearest string) error {
    return serr.New(serr.InvalidArgs, "no fuzzy match found").
        WithDetail(formatFailureDiff(search, nearest))
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `replace_content` hard-failed on any whitespace drift from LLM output | 4-strategy cascade with indentation reflow | Aider popularized it ~2023 in `editblock_coder.py`; now industry standard for LLM edit tools | Huge agent UX win — drift in leading whitespace no longer breaks edits |
| Character-level edit distance (DMP) | Line-based perfect sweep + normalize | Aider's `search_replace.py` prototype tried DMP and Git cherry-pick approaches but production uses the simpler line-based path | Simpler, deterministic, agent-readable failure modes |

**Deprecated / ignored:**
- `aider/coders/search_replace.py` — experimental module with Git cherry-pick and
  diff-match-patch strategies. **Not used in production Aider.** Production path
  is `editblock_coder.py` only. Verified via [CITED: github.com/Aider-AI/aider
  editblock_coder.py main branch as of research date].

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `go-difflib` v1.0.0 remains installable and stable for the lifetime of the unused indirect dep; if we promote it to direct, no security advisories apply | §Standard Stack alternatives | LOW — we recommend NOT using it, so moot |
| A2 | Aider's `editblock_coder.py` hasn't materially changed since WebFetch pulled it this session | §Architecture Patterns, §Code Examples | LOW — the algorithm is stable and widely ported |
| A3 | `sergi/go-diff` is heavier than go-difflib (training-era knowledge, not re-verified this session) | §Standard Stack alternatives | LOW — we reject both in favor of hand-roll |
| A4 | Windows CRLF is a real-enough concern to document as Pitfall 8 | §Pitfalls | LOW — if no Windows user ever reports it, the test still pins behavior |
| A5 | Phase 26's `replace_symbol_body` will pass the tree-sitter-extracted body range's **substring** (not the whole file) as `source` to `Match`, so the engine need not be tree-sitter aware | §Open Questions, §Validation Architecture | MEDIUM — if Phase 26 passes whole-file source instead, byte offsets returned need to be whole-file offsets; this is still achievable (Phase 26 adds body.StartByte to the Result), but it changes the integration shape. Engine's API supports either model — contract is "byte offsets are relative to whatever `source` string you passed in." Document this clearly. |

**User confirmation sought:** None of the assumptions above change the Phase 25
plan's structure. A5 is the highest-impact — see §Open Questions Q1 for the
explicit decision we're deferring to Phase 26.

## Open Questions

1. **Does Phase 26 pass the tree-sitter body substring or the whole file as `source` for `replace_symbol_body`?**
   - What we know: `internal/kernel/edit/replace.go:37-46` currently extracts
     `startByte, endByte` via `extractor.ExtractBody(...)` then does byte-slice
     surgery on the whole file. The fuzzy fallback could either be called with
     `source[startByte:endByte]` (clean, engine stays local) or with the whole
     file + pre-computed anchor range (requires engine to know about a sub-range).
   - What's unclear: Phase 26's preference.
   - Recommendation: **The engine's contract is "byte offsets in Result are
     relative to the source string you passed."** This lets Phase 26 pass either
     shape and handle offset translation at the call site. Document this in
     `fuzzy.go` package doc comment. Phase 26 picks when it exists.

2. **Should the fail-with-diff "nearest window" selection be sophisticated or naive?**
   - What we know: we need to emit *something* as "nearest source region" for
     the payload. Options: (a) pick the source window of same length as search
     that has the most line-equal matches under the indentation-flexible comparator,
     (b) pick the first source window with any line matching the first search
     line, (c) just emit the first N lines of source.
   - What's unclear: CONTEXT.md doesn't specify.
   - Recommendation: **Option (a)** — cheapest meaningful signal. Implementation:
     during the indentation-flexible sweep, track `bestScore = max(linesMatching)`
     and `bestOffset`. If no strategy succeeds, `formatFailureDiff` uses
     `source[bestOffset:bestOffset+len(search)]` as `nearest`. If no lines matched
     anywhere, fall back to source[0:len(search)]. Zero extra passes over source.

3. **Tree-sitter awareness in Phase 25?**
   - What we know: CONTEXT.md §Domain Boundary is explicit: "Build a self-contained
     fuzzy text matching/replacement library ... the engine itself stays text-only."
     `internal/kernel/edit/treesitter.go` exists and is used by Phase 26 callers,
     but `internal/fuzzy/` MUST NOT import it.
   - What's unclear: Nothing — this is locked.
   - Recommendation: **No tree-sitter in Phase 25.** Enforced by keeping the
     import list to `strings`, `fmt`, `regexp`, `internal/errors`. The planner
     should reject any task that introduces `go-tree-sitter` imports.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Package build | ✓ (assumed) | 1.25+ | — |
| `strings`, `fmt`, `regexp` stdlib | All modules | ✓ | Go 1.25 | — |
| `github.com/stretchr/testify` | Unit tests | ✓ (already in go.mod) | — | — |
| `internal/errors` (serr) | Error construction | ✓ (internal package) | v1.5 | — |

No external dependencies needed. No missing dependencies, no blockers.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` + `github.com/stretchr/testify` (assert + require) |
| Config file | none — standard `go test` discovery |
| Quick run command | `go test ./internal/fuzzy/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FUZZ-01 | Exact strategy matches byte-identical search | unit | `go test ./internal/fuzzy -run TestMatch_ExactStrategy` | ❌ Wave 0 |
| FUZZ-01 | Whitespace-normalized strategy matches after leading/trailing trim | unit | `go test ./internal/fuzzy -run TestMatch_WhitespaceStrategy` | ❌ Wave 0 |
| FUZZ-01 | Indentation-flexible strategy matches after TrimLeft | unit | `go test ./internal/fuzzy -run TestMatch_IndentationStrategy` | ❌ Wave 0 |
| FUZZ-01 | No strategy matches → fail-with-diff | unit | `go test ./internal/fuzzy -run TestMatch_FailWithDiff` | ❌ Wave 0 |
| FUZZ-01 | Cascade does NOT proceed past ambiguity | unit | `go test ./internal/fuzzy -run TestCascade_StopsOnAmbiguity` | ❌ Wave 0 |
| FUZZ-02 | Result.Strategy is exact/ws/indent/failed correctly | unit | `go test ./internal/fuzzy -run TestMatch_StrategyReporting` | ❌ Wave 0 |
| FUZZ-02 | Result.Score is 1.0/0.95/0.85/0.0 correctly | unit | `go test ./internal/fuzzy -run TestMatch_ScoreTiers` | ❌ Wave 0 |
| FUZZ-03 | Replacement gets sourcePrefix reapplied | unit | `go test ./internal/fuzzy -run TestReflow_AppliesSourcePrefix` | ❌ Wave 0 |
| FUZZ-03 | Empty replacement lines stay empty (no trailing ws) | unit | `go test ./internal/fuzzy -run TestReflow_PreservesEmptyLines` | ❌ Wave 0 |
| FUZZ-03 | Tab-indented source + space-indented search → tabs preserved | unit | `go test ./internal/fuzzy -run TestReflow_TabsAndSpaces` | ❌ Wave 0 |
| FUZZ-07 | N > 1 matches at exact → serr.InvalidArgs immediately | unit | `go test ./internal/fuzzy -run TestAmbiguity_Exact` | ❌ Wave 0 |
| FUZZ-07 | Ambiguity detail lists 1-indexed line numbers, cap 5 | unit | `go test ./internal/fuzzy -run TestAmbiguity_LineNumberCap` | ❌ Wave 0 |
| FUZZ-07 | "...and N more" suffix for > 5 matches | unit | `go test ./internal/fuzzy -run TestAmbiguity_OverflowSuffix` | ❌ Wave 0 |
| FUZZ-07 | Ambiguity does NOT cascade to next strategy | unit | `go test ./internal/fuzzy -run TestAmbiguity_DoesNotCascade` | ❌ Wave 0 |
| FUZZ-08 | Single `...` line splits search into 2 segments | unit | `go test ./internal/fuzzy -run TestEllipsis_TwoSegments` | ❌ Wave 0 |
| FUZZ-08 | Multiple `...` lines → N segments | unit | `go test ./internal/fuzzy -run TestEllipsis_MultipleSegments` | ❌ Wave 0 |
| FUZZ-08 | Leading whitespace on `...` line accepted | unit | `go test ./internal/fuzzy -run TestEllipsis_LeadingWhitespace` | ❌ Wave 0 |
| FUZZ-08 | Inline `...` (not on own line) treated as literal | unit | `go test ./internal/fuzzy -run TestEllipsis_InlineLiteral` | ❌ Wave 0 |
| FUZZ-08 | Empty segment (search starts/ends with `...`) → error | unit | `go test ./internal/fuzzy -run TestEllipsis_EmptySegmentRejected` | ❌ Wave 0 |
| FUZZ-08 | Replacement must have matching segment count | unit | `go test ./internal/fuzzy -run TestEllipsis_ReplacementSegmentMismatch` | ❌ Wave 0 |
| FUZZ-08 | Segments match in order; interleaved source lines preserved | unit | `go test ./internal/fuzzy -run TestEllipsis_InOrderMatching` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/fuzzy/... && go vet ./internal/fuzzy/...`
- **Per wave merge:** `go test ./internal/fuzzy/... -race && gofmt -l internal/fuzzy/`
- **Phase gate:** `go test ./... && go vet ./... && gofmt -l .` full-repo green
  before `/gsd-verify-work`. Per CLAUDE.md: "Always run go vet and go test before
  completing any Go task."

### Wave 0 Gaps

- [ ] `internal/fuzzy/fuzzy_test.go` — top-level Match() tests covering FUZZ-01,
      FUZZ-02 integration cases
- [ ] `internal/fuzzy/strategies_test.go` — per-strategy unit tests, ambiguity
      (FUZZ-07), cascade-stop behavior
- [ ] `internal/fuzzy/ellipsis_test.go` — all FUZZ-08 segment edge cases
- [ ] `internal/fuzzy/indent_test.go` — FUZZ-03 reflow cases including
      tabs/spaces/mixed and empty-line preservation
- [ ] `internal/fuzzy/diff_test.go` — golden-style assertions on
      `formatFailureDiff` output
- [ ] No framework install needed — testify already in go.mod

### Validation Dimensions (from CONTEXT.md Notes for Researcher)

1. **Per-strategy unit tests** — each of the 4 tiers in isolation with minimal input.
2. **Cascade orchestration** — order, early-stop, score/strategy field population.
3. **Ambiguity edge cases** — 2, 5, 6, 10 match counts; at each strategy layer.
4. **Ellipsis segmentation** — 1/2/N segments, leading ws on marker, inline literal,
   empty segment rejection, replacement pattern mismatch, in-order matching across
   interleaved source content.
5. **Indentation reflow** — all-tabs source, all-spaces source, mixed
   indentation within matched region (verify first-line-wins deviation),
   empty lines preserved, whitespace-only lines preserved, deep-nested
   (8-level) reflow.
6. **Fail-with-diff payload format** — header lines, hunk marker, `-`/`+` prefixes,
   multi-line search, multi-line nearest, empty nearest fallback.
7. **Boundary conditions** — empty search, single-line search, search longer than
   source, search matching at end-of-file, search with only blank lines.
8. **Newline handling** — source with trailing `\n`, source without trailing `\n`,
   search with trailing `\n`, search without.

## Project Constraints (from CLAUDE.md)

The following CLAUDE.md directives are in force for Phase 25 and must be
respected by all plans:

- **Go is the active development language.** Phase 25 lands Go code under
  `internal/fuzzy/`. No Python, no changes under `legacy/`.
- **Run `go vet ./...` and `go test ./...` before completing any Go task.**
  This is the phase gate. Plans must include the commands in task verification
  steps.
- **Format with `gofmt -w .`** before any commit.
- **GSD workflow enforcement:** all edits go through GSD commands. This phase
  has a CONTEXT.md, so planning proceeds via `/gsd-plan-phase`; implementation
  via `/gsd-execute-phase`.
- **Repo layout:** the engine lives in `internal/fuzzy/`. Do not place code
  under `pkg/`, `cmd/`, or anywhere outside `internal/` — the package is
  private to the module.
- **Single binary / native concurrency:** the engine is pure, synchronous
  code; no goroutines needed (and CONTEXT.md's strategy-is-sequential decision
  forbids parallelism anyway).

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Engine has no auth surface |
| V3 Session Management | no | Stateless pure function |
| V4 Access Control | no | No privileges; callers control file access |
| V5 Input Validation | **yes** | Input validation: empty search → `serr.InvalidArgs`; malformed ellipsis (empty segments) → `serr.InvalidArgs`; segment count mismatch → `serr.InvalidArgs`. Use explicit length checks before slicing. |
| V6 Cryptography | no | No crypto |

### Known Threat Patterns for `internal/fuzzy/`

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unbounded input causing pathological sweep time | Denial of Service | The sweep is O(N*M) where N = source lines, M = search lines. Pathological: 10k-line search against 10k-line source = 100M comparisons (~100ms). This is acceptable for an agent-invoked tool. No explicit bound needed. Document in package comment. |
| Regex injection via source content | Tampering | Only one regex in the package: `(?m)^[ \t]*\.\.\.$` applied to the **search block**, never to source content. The regex pattern is a compile-time constant — no user input reaches `regexp.MustCompile`. Safe. |
| Integer overflow in byte offset math | Tampering | Go's `int` bound is 9e18 on 64-bit; file sizes are bounded by disk. Use `int` consistently (not `int32`). No overflow risk at realistic scales. |
| Nil-pointer return from Match hiding error | Security-adjacent correctness | Guard against `*Error`-nil-pointer trap per `internal/errors/errors.go:14-16` — return `nil` directly on success, never a typed nil `*serr.Error`. |

The engine is pure text manipulation with no file I/O and no process or
network surface. Standard input validation on the API boundary is the only
real security concern.

## Sources

### Primary (HIGH confidence)

- `github.com/Aider-AI/aider/blob/main/aider/coders/editblock_coder.py` — canonical
  reference for the 4-strategy cascade, common-prefix dedent, and ellipsis handling.
  Fetched verbatim line numbers 73-82 (perfect_replace), 107-145 (cascade orchestrator),
  118-124 (spurious blank line retry), 130-176 (try_dotdotdots),
  188-220 (replace_part_with_missing_leading_whitespace), 223-244
  (match_but_for_leading_whitespace).
- `internal/errors/errors.go`, `internal/errors/kinds.go` — read this session.
  `serr.New(Kind, message).WithDetail(str)` / `serr.Wrap(kind, msg, cause)`.
  `InvalidArgs = "invalid_args"`.
- `internal/kernel/edit/replace.go` — read this session. Confirms Phase 26
  caller shape: passes source as bytes/string, uses `startByte/endByte`
  offsets, returns `error`. Engine's API is compatible.
- `internal/kernel/fileops/replace.go` — read this session. Returns
  `(count int, error)` from a `strings.ReplaceAll` path. Phase 26 will layer
  fuzzy fallback when count == 0, passing the same `content` through the
  engine.
- `internal/kernel/edit/edit_test.go` + `internal/errors/errors_test.go` —
  confirmed test style: testify `assert`/`require`, `[]struct{}` tables,
  `t.Run(tt.name, ...)` per-case.
- `go.mod` / `go.sum` — confirmed `github.com/pmezard/go-difflib v1.0.0` is
  `// indirect` and `go mod why` reports main module doesn't need it.

### Secondary (MEDIUM confidence)

- `pkg.go.dev/github.com/pmezard/go-difflib/difflib` — WebFetch confirmed
  BSD-3 license, v1.0.0 published 2016-01-10, not actively maintained. API
  surface (`WriteUnifiedDiff`, `GetUnifiedDiffString`, `UnifiedDiff` struct)
  documented; we choose not to use it.

### Tertiary (LOW confidence)

- Training-era knowledge about `github.com/sergi/go-diff/diffmatchpatch`
  relative weight compared to go-difflib. Not re-verified this session
  because we recommend against using it anyway.

## Metadata

**Confidence breakdown:**

- Standard stack: **HIGH** — stdlib + testify + internal/errors, all verified in
  current session against real files.
- Architecture: **HIGH** — Aider's editblock_coder.py fetched verbatim this
  session; algorithm is small, well-scoped, and widely implemented elsewhere.
- Pitfalls: **HIGH** for items 1-6 (generic to line-based fuzzy matching and
  verified against CONTEXT.md decisions); **MEDIUM** for items 7-8 (ellipsis
  segment drift and CRLF are speculative until tests are written).
- Validation: **HIGH** — test framework and style are verified from real files
  in the current repo.

**Research date:** 2026-04-15
**Valid until:** 2026-05-15 (30 days — stable algorithm, stable stack).

## RESEARCH COMPLETE
