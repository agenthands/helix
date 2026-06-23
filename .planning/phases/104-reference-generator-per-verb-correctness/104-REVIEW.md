---
phase: 104-reference-generator-per-verb-correctness
reviewed: 2026-06-24T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - cmd/helix-refgen/render.go
  - cmd/helix-refgen/render_test.go
  - internal/cli/skills/helix/reference.md
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
---

# Phase 104: Code Review Report

**Reviewed:** 2026-06-24T00:00:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

The change adds two keyed per-verb override maps (`outputShapeOverrides`,
`useThisNotThatOverrides`) consulted before a group-default fallback in
`cmd/helix-refgen/render.go`, fixing the "group collapse" where 10 non-query
verbs folded into the `groupMemory` GroupID inherited semantically wrong
memory-query prose. I verified the core invariants against the live codebase:

- **Override set completeness is correct.** `internal/cli/verbs_gen.go` carries
  exactly 13 verbs with `groupID: "memory"`. The override set is the 10
  non-query verbs (`write/edit/delete/rename-memory`, `switch-mode`,
  `get-token-budget`, `onboard-project`, `prepare-for-new-conversation`,
  `get-health`, `get-tool-help`); the 3 genuine query verbs left on the default
  are `read-memory`, `list-memories`, `search-memories`. 10 + 3 = 13, an exact
  partition with no verb missed or double-covered.
- **Dispatch is override-then-fallback with keyed lookups.** `outputShape` /
  `useThisNotThat` do a single keyed map read (`m[verb]`), never a range, so
  output stays byte-deterministic.
- **The two override maps have identical key sets** (verified by inspection: the
  same 10 kebab keys appear in both), and `reference.md` is internally
  consistent with the generator — every overridden verb's section carries the
  override prose, and the 3 query verbs retain the `ranked FTS5 search result`
  default.
- `go vet ./cmd/helix-refgen/` and `go test ./cmd/helix-refgen/` both pass.

The one substantive defect is in the **Guard B vacuity discriminator**
(`TestOverrideDiffersFromGroupDefault`): its discriminator is a tautology
(`x == x`) that never exercises the production dispatch path it claims to guard.
In an anti-vacuity milestone this is a real test defect (WR-01). Guard A is
genuinely non-vacuous and well-constructed. A secondary gap (WR-02): no test
pins the two maps' key sets to be identical, so the two maps can drift out of
parity silently.

## Warnings

### WR-01: Guard B discriminator is a tautology — does not exercise the dispatch path it claims to guard

**File:** `cmd/helix-refgen/render_test.go:184-192`
**Issue:**
The loop in `TestOverrideDiffersFromGroupDefault` (lines 171-179) is sound: it
asserts `outputShape(verb, group) != groupOutputDefault(group)` and
`useThisNotThat(verb, group) != groupUseDefault(group, verb)` for each overridden
verb — that genuinely catches a "fix" that copied the default verbatim.

The *discriminator* appended to prove the difference-check is non-vacuous does
not do what its comment claims. It reads:

```go
copiedDefault := groupOutputDefault(group2)
var noOps []string
if copiedDefault == groupOutputDefault(group2) {
    noOps = append(noOps, "synthetic-copied-default")
}
require.Lenf(t, noOps, 1, "a copied-default override ... MUST be flagged as a no-op, got %v", noOps)
```

`copiedDefault` is assigned `groupOutputDefault(group2)` and then compared to
`groupOutputDefault(group2)` again — i.e. `x == x`, which is unconditionally
true for any pure function. This branch would pass even if `outputShape`, the
override maps, and the entire dispatch were deleted or broken. It does NOT route
a synthetic copied-default value through `outputShape` (the production predicate
the loop relies on), so it proves nothing about the invariant. This is exactly
the "green-path-only test" the milestone forbids: the discriminator cannot fail
when the invariant breaks.

**Fix:** Drive a copied-default value through the same dispatch the loop uses,
via a synthetic override map, and assert the difference-check flags it. For
example, prove that when an override value equals the group default the
`NotEqual` predicate (the loop's actual check) reports them equal:

```go
// Discriminator: a synthetic override whose value EQUALS the group default
// must be caught by the SAME predicate the loop uses (assertNotEqual). Build a
// local lookup that mimics outputShape with a no-op (copied-default) entry and
// assert the difference predicate sees no difference.
const verb = "switch-mode"
synthetic := map[string]string{verb: groupOutputDefault(group)} // a no-op "fix"
got := synthetic[verb] // what outputShape would return for this copied entry
require.Equalf(t, groupOutputDefault(group), got,
    "a copied-default override is indistinguishable from the group default — "+
        "the loop's NotEqual check is what rejects it")
// and confirm the REAL override is NOT a no-op, closing the loop:
require.NotEqualf(t, groupOutputDefault(group), outputShape(verb, group),
    "the real override must differ, else the no-op check above would be vacuous")
```

The key property a real discriminator must have: it must FAIL if `outputShape`'s
override branch were removed (causing it to return the group default). The
current `x == x` form cannot.

### WR-02: No test pins the two override maps to identical key sets — silent drift possible

**File:** `cmd/helix-refgen/render.go:207-234` (maps); `cmd/helix-refgen/render_test.go` (missing assertion)
**Issue:**
`outputShapeOverrides` and `useThisNotThatOverrides` are two independent maps
that, by design, must cover the identical 10-verb set — the doc comment at
`render.go:221-222` says "the same 10 verbs (see outputShapeOverrides)". But no
test asserts key-set equality between the two maps.

`TestRenderOverride` iterates `overriddenVerbs` (a third, hand-maintained list
in the test at lines 17-28) and `require`s each verb is present in *both* maps
(lines 89-95), which incidentally catches a verb missing from either map *if it
is in `overriddenVerbs`*. But if a future edit adds a key to only one map (e.g.
adds `"get-health"` to `outputShapeOverrides` but forgets
`useThisNotThatOverrides`) AND simultaneously the new verb is not added to the
`overriddenVerbs` test list, the divergence goes undetected — that verb would
get an overridden Output line but a stale group-default Use-this line in
`reference.md`, the exact class of inconsistency this phase exists to prevent.

The `overriddenVerbs` test slice duplicating the map key set is itself a
maintenance hazard: three lists (two maps + one test slice) must be kept in
lockstep with no cross-check between the two maps directly.

**Fix:** Add a direct key-set-parity assertion so the two maps cannot drift:

```go
func TestOverrideMapsHaveIdenticalKeys(t *testing.T) {
    for k := range outputShapeOverrides {
        _, ok := useThisNotThatOverrides[k]
        assert.Truef(t, ok, "%q in outputShapeOverrides but missing from useThisNotThatOverrides", k)
    }
    for k := range useThisNotThatOverrides {
        _, ok := outputShapeOverrides[k]
        assert.Truef(t, ok, "%q in useThisNotThatOverrides but missing from outputShapeOverrides", k)
    }
    assert.Len(t, useThisNotThatOverrides, len(outputShapeOverrides),
        "override maps must cover the identical verb set")
}
```

Optionally also assert `len(outputShapeOverrides) == len(overriddenVerbs)` to
pin the test's own list to the production maps.

## Info

### IN-01: `overriddenVerbs` test list could be derived from the maps rather than hand-maintained

**File:** `cmd/helix-refgen/render_test.go:17-28`
**Issue:**
`overriddenVerbs` is a hand-copied duplicate of the override-map key set. It is
the canonical input for `TestRenderOverride` and `TestOverrideDiffersFromGroupDefault`,
so a verb dropped from this slice silently shrinks coverage without any test
failing (a vacuity surface). Combined with WR-02, three sources of truth must be
synchronized by hand.
**Fix:** Derive the iteration set from `outputShapeOverrides` keys (sorted for
determinism) instead of a literal slice, and keep `overriddenVerbs` only if an
explicit, independent cross-check against the maps is desired (WR-02 supplies
that cross-check). This makes "verb dropped from coverage" impossible.

### IN-02: Group-default partition is asserted only for Output, not Use-this, on genuine query verbs

**File:** `cmd/helix-refgen/render_test.go:107-116`
**Issue:**
For the 3 genuine memory-query verbs the test asserts they are absent from both
override maps (lines 108-111, good) and that the rendered section keeps the
Output default (`oldMemoryOutputMarker`, line 114). It does NOT assert the
section keeps the *Use-this* group default (`oldMemoryUseThisMarker`). The
positive "default retained" check is therefore one-sided — a regression that
overrode only the Use-this line for `read-memory` would not be caught here.
**Fix:** Add the symmetric assertion in the query-verb loop:
```go
assert.Containsf(t, sec, oldMemoryUseThisMarker,
    "genuine memory query verb %q must keep the group-default Use-this prose", verb)
```

### IN-03: `firstSentence` abbreviation guard re-uses the `". "` boundary length but the `160` cap is a magic number

**File:** `cmd/helix-refgen/render.go:116`
**Issue:**
`if idx <= 0 || idx >= 160` hard-codes the 160-char synopsis truncation cap as a
bare literal inline. This is unrelated to the Phase 104 override change (it is
pre-existing in the same function) but is a magic number with no named constant
or comment explaining the 160 choice; a future tweak risks an inconsistent cap.
Not a correctness issue.
**Fix:** Extract `const maxSynopsisLen = 160` with a one-line rationale comment.
Low priority — flagged only because the file was under review.

---

_Reviewed: 2026-06-24T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
