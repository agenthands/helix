---
phase: 105-skill-md-decision-matrix-rewrite
reviewed: 2026-06-24T00:00:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - internal/cli/skill_test.go
  - internal/cli/skills/helix/SKILL.md
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: clean
---

# Phase 105: Code Review Report

**Reviewed:** 2026-06-24
**Depth:** standard
**Files Reviewed:** 2
**Status:** clean

## Summary

Adversarial review of the Phase 105 `## Decision matrix` rewrite in the hand-authored
bundle file `internal/cli/skills/helix/SKILL.md` and the three new anti-vacuity guards
(`TestSkillMatrixNoQueryActionMix`, `TestSkillMatrixNoEmptyNotThis`,
`TestSkillMatrixGraphPrereq`) plus the `parseMatrixRows` helper and
`TestSkillMatrixClassificationComplete` in `internal/cli/skill_test.go`.

The starting hypothesis was that the guards are green-path-only and the matrix
invariants do not actually hold on the real embedded bytes. **That hypothesis is
refuted by direct evidence.** I did not flag the guards as vacuous because I proved
the opposite by mutation: injecting each offender into the *real* SKILL.md drives the
corresponding positive arm RED, and every negative arm rejects a real offender parsed
by the same `parseMatrixRows`. No BLOCKER or WARNING defects were found. Two INFO items
record a minor over-claim in a code comment and a latent parser fragility that is not
currently exercised.

### Evidence gathered (not assertion-by-vibe)

1. **Classification authority (prompt scrutiny #2) — VERIFIED EXACT.** Extracted the
   50 frozen tool names from `internal/cli/verbs_gen.go` and diffed them against
   `querySet ∪ actionSet`: 50 classified, **zero gaps, zero overlaps, zero stray keys**
   (`comm` produced empty diffs both directions; no dupes). `TestSkillMatrixClassificationComplete`
   keys on `VerbToolNames()` (the generated catalog, `verb.go:81`), not a hardcoded
   list, and asserts `len(querySet)+len(actionSet) == len(VerbToolNames())`. A future
   verb add cannot silently fall behind.

2. **Anti-vacuity (prompt scrutiny #1) — VERIFIED BY MUTATION.** With the real
   embedded bytes:
   - Injecting a QUERY+ACTION mix (`read-memory` + `write-memory` in one `Use this`
     cell) → `TestSkillMatrixNoQueryActionMix` FAILS with the offending cell named.
   - Replacing a real `Not this` cell with U+2014 → `TestSkillMatrixNoEmptyNotThis`
     FAILS naming `helix get-hover-info`.
   - Stripping the `†` from the `validate-graph-edge` row → `TestSkillMatrixGraphPrereq`
     FAILS naming `helix validate-graph-edge`.
   - Removing the `helix index-semantic-graph` legend citation →
     `TestSkillMatrixGraphPrereq` FAILS on the legend assertion.
   All four mutations were reverted; the suite returns to green. These are NOT
   synthetic-only guards — they bite on the production bundle.

3. **SKILL-02 `string(rune(0x2014))` claim — CONFIRMED CORRECT IN SUBSTANCE.** The
   checker `matrixRowEmptyNotThis` inspects only `row.notThis` (the last table cell).
   U+2014 does appear as a literal in `skill_test.go` and SKILL.md, but **only as prose
   punctuation in comments / the description / the legend line — never inside a
   `| Not this |` data cell**. Because the checker is cell-scoped, those prose
   em-dashes are structurally invisible to it. The rune construction keeps the *test
   offender* out of a casual scan; the checker still detects a real em-dash in a real
   `Not this` cell (proven by mutation above). See IN-01 for a precision note on how
   the rationale comment phrases this.

4. **parseMatrixRows robustness (prompt scrutiny #3) — VERIFIED.** The parser extracts
   exactly **43 data rows** from the real table; the source has 45 `|`-leading lines
   after the heading (43 data + 1 header + 1 separator). No silent row drops. It
   correctly: skips the separator (`-:` run test), skips the header (the `use this` +
   `not this` heuristic, which matches ONLY the real header — confirmed no data row
   contains both phrases), keeps the Capability lead column, parses multi-verb cells
   (row 26 yields both `get_cluster_map` and `explain_cluster`), and detects 7
   graph-reader rows covering all 8 graph-reader verbs. The two graph BUILDERS
   (`index_semantic_graph`/`refresh_semantic_graph`) are correctly excluded from
   `graphReaderVerbs`, so the "Build / refresh" row (no `†`) is not false-flagged.

5. **Anchor + cap preservation (prompt scrutiny #4) — INTACT.** `frontmatter name:
   helix`, the SKILL-04 token-note (idle cost = 599 bytes), the 1536-char cap test,
   the verb-membership drift gate, and the bundle/containment tests all pass. The
   legend's `helix index-semantic-graph` citation is a real frozen verb, so it does
   not trip the drift gate.

6. **Zero-dep invariant — HELD.** `git diff go.mod go.sum` is empty; the new helpers
   use stdlib + the pre-existing testify only. `gofmt -l skill_test.go` is clean;
   `go vet ./internal/cli/` is clean; full `go test ./internal/cli/` is green.

## Info

### IN-01: SKILL-02 rationale comment slightly over-claims "never appears as a source token"

**File:** `internal/cli/skill_test.go:564-565`
**Issue:** The comment for `matrixRowEmptyNotThis` states the em-dash "is constructed
from its rune so the literal placeholder character **never appears as a source token**
the SKILL-02 negative-grep could scan." Strictly, U+2014 *does* appear as a literal
in this very file (10 occurrences, all prose em-dashes in comments — e.g. lines 17,
142, 560, 573). The intended and accurate claim is narrower: the literal never appears
*as a `Not this` table-cell value or as the negative-arm's fabricated offender* — both
of which are constructed via `string(rune(0x2014))`. The checker is cell-scoped, so the
prose em-dashes are harmless, but the comment as written is falsifiable by a literal
grep and could mislead a future maintainer into thinking the file is em-dash-free.
**Fix:** Tighten the comment to scope the claim to the cell/offender, e.g.:
```go
// emDash is constructed from its rune so the placeholder never appears as a
// `Not this` cell value or fabricated offender — only as prose punctuation
// elsewhere, which this cell-scoped checker ignores.
emDash := string(rune(0x2014))
```

### IN-02: parseMatrixRows header-skip is heuristic, not structural (latent fragility, not currently triggered)

**File:** `internal/cli/skill_test.go:430-435`
**Issue:** The header row is identified by the substring heuristic `contains("use this")
&& contains("not this")` rather than by position (first non-separator row). Today no
*data* row contains both phrases, so the heuristic is correct (verified by grep). But it
is content-coupled to the header wording: if a future row's `Question`/`Not this` prose
happened to contain both literals (e.g. a row explaining "use this, not this fallback"),
that real data row would be silently dropped from all three guards, re-introducing
vacuity for that row. This is a robustness smell, not a present defect — flagged so the
coupling is a conscious choice.
**Fix:** Prefer a position-based skip (drop the first data-shaped row immediately
following the separator), or anchor the heuristic to the exact header cells
(`cells[len-2] == "Use this" && cells[len-1] == "Not this"`) so it cannot match a
lowercased data-row substring:
```go
if cells[len(cells)-2] == "Use this" && cells[len(cells)-1] == "Not this" {
    continue // exact header match, not a substring heuristic
}
```

---

_Reviewed: 2026-06-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
