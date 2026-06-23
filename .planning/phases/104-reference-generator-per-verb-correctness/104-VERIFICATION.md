---
phase: 104-reference-generator-per-verb-correctness
verified: 2026-06-24T01:20:00Z
status: passed
score: 6/6 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 104: Reference Generator Per-Verb Correctness Verification Report

**Phase Goal:** Every verb's "use this, not that" and "Output" lines in the generated `reference.md` are correct for that verb's actual semantics, fixed at the generator root cause (the group collapse), with the corrected `reference.md` regenerated, committed, and reproducible.
**Verified:** 2026-06-24T01:20:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth                                                                                                       | Status     | Evidence                                                                                                                                                                       |
| --- | ----------------------------------------------------------------------------------------------------------- | ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | The 6 previously-collapsed non-memory verbs render their OWN semantics, not memory-query prose              | ✓ VERIFIED | All of switch-mode, get-token-budget, onboard-project, prepare-for-new-conversation, get-health, get-tool-help sections in committed `reference.md` carry no FTS5/durable-memory prose (per-section awk scan) |
| 2   | The 4 mutating memory verbs render mutation-shaped prose, not query-shaped                                  | ✓ VERIFIED | write/edit/delete/rename-memory sections carry confirmation/mutation prose; no `ranked FTS5 search result` or `durable project/session memory` markers in those sections      |
| 3   | The fix lives in the GENERATOR (override map + group-default fallback), not a hand-edit of reference.md     | ✓ VERIFIED | `cmd/helix-refgen/render.go:207-234` defines `outputShapeOverrides`/`useThisNotThatOverrides`; `:238-243`/`:267-272` keyed-lookup-then-fallback; phase commits touch only render.go/render_test.go/reference.md |
| 4   | Regenerated reference.md is byte-reproducible: `--check` up-to-date and `git diff` empty after fresh regen | ✓ VERIFIED | `go run ./cmd/helix-refgen` → `Updated`; `--check` → `reference.md is up to date.`; `git diff --stat` empty (exit 0)                                                            |
| 5   | reference ⊇ VerbToolNames() stays green at exactly 50; cligen --check clean (categoryToGroup untouched)     | ✓ VERIFIED | `TestReferenceCoversEveryVerb` PASS; 50 `## \`helix` sections; `go run ./cmd/helix-cligen --check` → `verbs_gen.go is up to date.`; cligen render.go last touched commit 02235fde (phase 92) |
| 6   | Guard A (no non-verb key) + Guard B (overridden line ≠ group default), each with a break-the-invariant test | ✓ VERIFIED | `TestOverrideKeysAreRealVerbs` + `TestOverrideDiffersFromGroupDefault` PASS; mutation test (emptying both maps) drives Guard B + TestRenderOverride RED — non-vacuous            |

**Score:** 6/6 truths verified (0 present, behavior-unverified)

### Success Criteria (ROADMAP contract)

| SC   | Criterion                                                                | Status     |
| ---- | ------------------------------------------------------------------------ | ---------- |
| SC#1 | Root-cause generator fix via per-verb override map w/ group-default fallback (not hand-edit); collapsed verbs read correctly | ✓ VERIFIED |
| SC#2 | Regenerated reference.md committed, passes `helix-refgen --check` byte-for-byte | ✓ VERIFIED |
| SC#3 | reference ⊇ VerbToolNames() ==50 contract + --check drift gate green; cligen/docgen/blank-import parity re-verified | ✓ VERIFIED |
| SC#4 | Vacuity guards prove the override map is real (Guard A non-verb key; Guard B overridden line ≠ default), break-the-invariant | ✓ VERIFIED |

### Required Artifacts

| Artifact                                    | Expected                                              | Status     | Details                                                                                                       |
| ------------------------------------------- | ----------------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------ |
| `cmd/helix-refgen/render.go`                | Override maps + group-default helpers + keyed dispatch | ✓ VERIFIED | `outputShapeOverrides` (10 keys), `useThisNotThatOverrides` (10 keys), `groupOutputDefault`, `groupUseDefault`, `(verb,group)` signatures, call sites pass `d.Verb` |
| `cmd/helix-refgen/render_test.go`           | Override golden + Guard A + Guard B (+ key-parity)     | ✓ VERIFIED | 4 override tests present and passing; Guard B pinned to production `outputShape` (line 218); +`TestOverrideMapsHaveIdenticalKeys` from review fix |
| `internal/cli/skills/helix/reference.md`    | Regenerated, corrected per-verb prose, 50 verbs       | ✓ VERIFIED | 50 sections; 10 collapsed verbs corrected; 3 genuine query verbs keep memory default; byte-clean under `--check` |

### Key Link Verification

| From                          | To                                          | Via                                          | Status     | Details                                       |
| ----------------------------- | ------------------------------------------- | -------------------------------------------- | ---------- | --------------------------------------------- |
| `renderVerb`                  | `outputShape`/`useThisNotThat` (verb,group) | passes `d.Verb` to both selectors            | ✓ WIRED    | render.go:81, render.go:91                    |
| `outputShape`/`useThisNotThat`| override maps                               | `m[verb]` keyed lookup before group fallback | ✓ WIRED    | render.go:239, render.go:268                  |
| Guard A test                  | `cli.VerbToolNames()`                        | kebab authority every override key must ∈    | ✓ WIRED    | render_test.go:48-57, :137-161                |

### Behavioral Spot-Checks

| Behavior                                                       | Command                                              | Result                              | Status |
| ------------------------------------------------------------- | --------------------------------------------------- | ----------------------------------- | ------ |
| refgen override + guard tests pass                            | `go test -count=1 ./cmd/helix-refgen/`               | 8/8 PASS                            | ✓ PASS |
| reference contract ==50 + discriminators                     | `go test -count=1 -run Reference ./internal/cli/`    | TestReferenceCoversEveryVerb + 3 PASS | ✓ PASS |
| byte-reproducible regen                                       | `go run ./cmd/helix-refgen && --check && git diff`   | up-to-date, diff empty              | ✓ PASS |
| cligen verb-SET parity                                        | `go run ./cmd/helix-cligen --check`                  | `verbs_gen.go is up to date.`       | ✓ PASS |
| docgen blank-import parity                                    | `go run ./cmd/docgen --check`                        | `README.md is up to date.`          | ✓ PASS |
| Guard B non-vacuity (mutation)                               | empty both override maps → re-run guards             | TestOverrideDiffersFromGroupDefault + TestRenderOverride go RED | ✓ PASS |
| go vet                                                        | `go vet ./...`                                       | exit 0                              | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description                                                   | Status      | Evidence                                                            |
| ----------- | ----------- | ------------------------------------------------------------ | ----------- | ------------------------------------------------------------------ |
| REFGEN-01   | 104-01-PLAN | Per-verb "use this/not that" + Output lines correct, generator root-cause fix, reference.md regenerated & --check byte-clean | ✓ SATISFIED | All 4 SCs verified; REQUIREMENTS.md:16 marked complete; traceability table maps REFGEN-01 → Phase 104 |

### Anti-Patterns Found

| File                          | Line | Pattern                | Severity | Impact |
| ----------------------------- | ---- | ---------------------- | -------- | ------ |
| —                             | —    | None                   | —        | No debt markers (TBD/FIXME/XXX), no ranged map iteration (keyed lookup only — deterministic), no stubs |

### Deferred / Out-of-Scope

`cmd/helix-bench/TestRunSubcommandWiresDeltaPass` FAILS on full `go test ./...`. Confirmed PRE-EXISTING and out-of-scope: phase-104 commits (3e1371eb, 1c0736ef, 7bf6458d, 3eff61a9) touch only `cmd/helix-refgen/render.go`, `cmd/helix-refgen/render_test.go`, and `internal/cli/skills/helix/reference.md` — nothing under `cmd/helix-bench`. Documented in `deferred-items.md` and the verification emphasis note; identical failure on baseline `1e382f5c`. NOT counted as a phase-104 regression or gap.

### Review-Fix Confirmation

The iteration-1 code review flagged Guard B as a vacuous tautology (`x == x`). The fix (commit 3eff61a9) is REAL and verified in the codebase: `TestOverrideDiffersFromGroupDefault` now routes a synthetic copied-default through `dispatchOutput` and pins it to production `outputShape` (render_test.go:218), with the real-override-differs assertion at :216. Independent mutation test (emptying both override maps) confirms the guard goes RED — it cannot pass on the green path alone.

### Gaps Summary

None. All six observable truths and all four ROADMAP success criteria are verified against the actual codebase (not SUMMARY claims). The generator root-cause fix is present and keyed-deterministic, reference.md is byte-reproducible, the ==50 contract and cligen/docgen parity hold, and both vacuity guards are proven non-vacuous by mutation. REFGEN-01 fully satisfied.

---

_Verified: 2026-06-24T01:20:00Z_
_Verifier: Claude (gsd-verifier)_
