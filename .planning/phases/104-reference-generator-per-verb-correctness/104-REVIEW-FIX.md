---
phase: 104-reference-generator-per-verb-correctness
fixed_at: 2026-06-23T21:59:32Z
review_path: .planning/phases/104-reference-generator-per-verb-correctness/104-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 104: Code Review Fix Report

**Fixed at:** 2026-06-23T21:59:32Z
**Source review:** .planning/phases/104-reference-generator-per-verb-correctness/104-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2 (WR-01, WR-02; critical+warning scope, 0 critical)
- Fixed: 2
- Skipped: 0

These are TEST-only fixes. `cmd/helix-refgen/render.go` and the generated
`internal/cli/skills/helix/reference.md` were deliberately NOT touched —
`go run ./cmd/helix-refgen --check` remains byte-clean (`reference.md is up to
date`) after the fixes.

## Fixed Issues

### WR-01: Guard B discriminator is a tautology — does not exercise the dispatch path it claims to guard

**Files modified:** `cmd/helix-refgen/render_test.go`
**Commit:** 3eff61a9
**Applied fix:** Replaced the tautological discriminator (`copiedDefault :=
groupOutputDefault(group2)` then `if copiedDefault == groupOutputDefault(group2)`
→ `x == x`, unconditionally true) in `TestOverrideDiffersFromGroupDefault` with a
genuine break-the-invariant check. Added a `dispatchOutput(overrides, verb, group)`
helper that mirrors the production `outputShape` override-then-group-default
dispatch but against a caller-supplied map. The new discriminator:
1. Picks a real overridden verb (`switch-mode`, asserted present in
   `outputShapeOverrides`).
2. Builds a SYNTHETIC override map whose value is a copied group default (a no-op
   "fix") and asserts — routed through `dispatchOutput` — it is indistinguishable
   from the group default (the case the loop's `NotEqual` check exists to reject).
3. Asserts the REAL override genuinely differs from the group default through the
   SAME dispatch, and that `dispatchOutput(outputShapeOverrides, …)` matches the
   production `outputShape(…)`.

**Non-vacuity proof (mutation test):** temporarily stubbing `dispatchOutput`'s
override branch to always return `groupOutputDefault(group)` (simulating the
override branch being deleted) made `TestOverrideDiffersFromGroupDefault` go RED
with "the real \"switch-mode\" override must differ from the group default via the
production dispatch"; restoring the branch returned it to green. The discriminator
therefore FAILS if the override maps were emptied or the dispatch reverted to
group-default-only — the property the milestone requires.

### WR-02: No test pins the two override maps to identical key sets — silent drift possible

**Files modified:** `cmd/helix-refgen/render_test.go`
**Commit:** 3eff61a9
**Applied fix:** Added `TestOverrideMapsHaveIdenticalKeys`, which asserts (a) every
key of `outputShapeOverrides` is present in `useThisNotThatOverrides` and vice
versa, (b) the two maps have equal length (identical verb set — a verb gaining one
override but not the other would emit a corrected Output line beside a stale
group-default Use-this line in `reference.md`). It additionally pins the test's own
hand-maintained `overriddenVerbs` slice to the production map size and membership
(addresses the IN-01 vacuity surface), so all three sources of truth (two maps +
test slice) are cross-checked.

**Note on commit grouping:** WR-01 and WR-02 both edit the single file
`cmd/helix-refgen/render_test.go` and were staged together; they landed in one
atomic commit (`3eff61a9`) whose message is WR-01-scoped but which carries the
WR-02 `TestOverrideMapsHaveIdenticalKeys` test as well. The working tree is clean.

## Verification

- `go test ./cmd/helix-refgen/` — all four override tests PASS (`TestRenderOverride`,
  `TestOverrideKeysAreRealVerbs`, `TestOverrideDiffersFromGroupDefault`,
  `TestOverrideMapsHaveIdenticalKeys`).
- `go vet ./...` — clean (only pre-existing unrelated Swift tree-sitter C macro
  warning).
- `go run ./cmd/helix-refgen --check` — `reference.md is up to date` (generator
  output byte-clean; no production-code or `reference.md` change).
- WR-01 discriminator verified non-vacuous by mutation (see above).
- One pre-existing, env-dependent failure in `cmd/helix-bench`
  (`TestRunSubcommandWiresDeltaPass`) fails identically on the untouched baseline
  (verified via `git stash`); it is unrelated to this test-only change in
  `cmd/helix-refgen` and was not introduced by these fixes.

No existing passing assertion was weakened.

---

_Fixed: 2026-06-23T21:59:32Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
