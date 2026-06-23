---
phase: 104-reference-generator-per-verb-correctness
reviewed: 2026-06-24T00:00:00Z
depth: standard
iteration: 2
files_reviewed: 3
files_reviewed_list:
  - cmd/helix-refgen/render.go
  - cmd/helix-refgen/render_test.go
  - internal/cli/skills/helix/reference.md
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: clean
---

# Phase 104: Code Review Report (Iteration 2)

**Reviewed:** 2026-06-24T00:00:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** clean

## Summary

Re-review after the iteration-1 fixes. Both prior Warnings were fixed as
test-only changes in `cmd/helix-refgen/render_test.go`; `render.go` and
`reference.md` are unchanged in substance. I confirmed resolution by **mutation
testing** — deliberately breaking production code and the test helper to verify
each guard goes RED — rather than by reading the test prose alone. All 8 tests
in the package pass; `go vet ./cmd/helix-refgen/` is clean; regenerating
`reference.md` (`go run ./cmd/helix-refgen`) is a no-op, so the committed
artifact is byte-in-sync with the generator.

**WR-01 (Guard B vacuity) — RESOLVED.** The rewritten
`TestOverrideDiffersFromGroupDefault` is now genuinely non-vacuous. The old
`x == x` discriminator is gone; the new one routes a synthetic copied-default
value through a `dispatchOutput` helper that mirrors production `outputShape`,
and — critically — the helper is *pinned to production* (line 218:
`dispatchOutput(outputShapeOverrides, ...) == outputShape(...)`) so it cannot
silently re-implement the dispatch differently. Mutation results:

- Reverting `outputShape`/`useThisNotThat` to group-default-only (the exact
  scenario the old discriminator could not catch) → the loop at
  `render_test.go:191/193` goes RED.
- Emptying both override maps → RED at the loop AND at the
  `dispatchOutput(outputShapeOverrides, ...)` discriminator (line 216) and the
  `require.Contains` precondition (line 204).
- Breaking the `dispatchOutput` helper so it stops mirroring production (always
  returns the group default) → RED at line 213, proving the helper-to-production
  pin (line 218) is real, not a tautology. The helper therefore **cannot pass
  independently of production code**.

**WR-02 (no key-set parity test) — RESOLVED.** `TestOverrideMapsHaveIdenticalKeys`
pins `outputShapeOverrides` and `useThisNotThatOverrides` to the same key set
(bidirectional presence + length parity) and additionally cross-checks the
hand-maintained `overriddenVerbs` test slice against the production map size and
membership — closing the iteration-1 IN-01 "hand-maintained slice can silently
shrink" surface. Mutation results:

- Adding a key to only one map → RED ("`...` in outputShapeOverrides but missing
  from useThisNotThatOverrides" + length mismatch).
- Dropping a verb from the `overriddenVerbs` slice → RED ("overriddenVerbs test
  slice must match the production override-map size").

No Critical or Warning findings remain. Two Info items carried over from
iteration 1 are still open (both genuinely Info-level, neither blocks `clean`).

## Info

### IN-01: Genuine memory-query verbs assert only the Output default, not the Use-this default

**File:** `cmd/helix-refgen/render_test.go:107-116`
**Issue:**
For the 3 genuine memory-query verbs (`read-memory`, `search-memories`,
`list-memories`) the loop asserts (a) absence from both override maps — good —
and (b) that the rendered section keeps the Output group default
(`oldMemoryOutputMarker`, line 114). It does NOT assert the section keeps the
*Use-this* group default (`oldMemoryUseThisMarker`). I verified in the committed
`reference.md` that `read-memory`'s Use-this line IS the group default ("Use
`helix read-memory` for durable project/session memory ..."), so the
partition is correct today — but a regression that overrode only the Use-this
line for a query verb would not be caught by this one-sided check. (Carried over
from iteration-1 IN-02; unaddressed.)
**Fix:** Add the symmetric assertion in the query-verb loop:
```go
assert.Containsf(t, sec, oldMemoryUseThisMarker,
    "genuine memory query verb %q must keep the group-default Use-this prose", verb)
```

### IN-02: `firstSentence` 160-char synopsis cap is a bare magic number

**File:** `cmd/helix-refgen/render.go:116`
**Issue:**
`if idx <= 0 || idx >= 160` hard-codes the synopsis truncation cap as an inline
literal with no named constant or rationale comment. Pre-existing (not part of
the Phase 104 override change) and not a correctness issue; flagged only because
the file is under review. (Carried over from iteration-1 IN-03; unaddressed.)
**Fix:** Extract `const maxSynopsisLen = 160` with a one-line rationale.
Low priority.

---

_Reviewed: 2026-06-24T00:00:00Z (iteration 2)_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
