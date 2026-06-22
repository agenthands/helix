---
phase: 97-generated-per-verb-reference-deterministic-adoption-contract
plan: 02
subsystem: cli
tags: [adoption-contract, anti-vacuity, drift-gate, nudge, golden-test, tdd, merge-gate]

# Dependency graph
requires:
  - phase: 97-generated-per-verb-reference-deterministic-adoption-contract
    plan: 01
    provides: "embedded skills/helix/reference.md (50 verbs) + embed.FS bundle + VerbToolNames()/VerbSpecsForDocs accessors"
  - phase: 90-cli-verb-spine
    provides: "VerbToolNames() 50-verb completeness authority"
  - phase: 93-skill-on-demand
    provides: "nudge classifier (steerMessage/bashSteerMessage) + runNudgeCapture/parseAdvisory test harness"
provides:
  - "ADOPT-01a: reference completeness gate (authority = VerbToolNames()) + revert-and-fail proof, untagged (BLOCKS merge)"
  - "ADOPT-01b: per-shape nudge golden keyed on the emitted helix verb + revert-and-fail + empty-bucket floor, untagged (BLOCKS merge)"
  - "Replaced the weak Contains(...,\"helix\") nudge assertion with a specific-verb assertion"
  - "DEFER-97-01: documented that Bash sed/cat are silent today (classifier gate, not steerMessage); routed to STEER-01/Phase 98"
affects: [98 STEER-01 nudge-classifier broadening, future verb/description changes]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Anti-vacuity contract: completeness sourced from the registry authority (never the generator output) + mandatory revert-and-fail"
    - "Per-shape golden keyed on the EMITTED command/verb (not a substring) with an empty-bucket floor"
    - "Section-header-anchored completeness check (## `helix <verb>`) so prose mentions don't count as coverage"

key-files:
  created:
    - internal/cli/reference_contract_test.go
  modified:
    - internal/cli/nudge_test.go

key-decisions:
  - "Completeness keys on the per-verb SECTION HEADER (## `helix <verb>`), not a bare kebab substring — a verb mentioned only in another verb's use-this-not-that prose does NOT count as covered, making the revert-and-fail genuine"
  - "The ADOPT-01b golden asserts the LIVE current mapping, not bashSteerMessage's aspirational branch set: Bash sed/cat are silent today because isGrepReadTool's Bash arm matches only grep/find/rg/ag — asserting they fire would be a FALSE golden (the vacuity this phase kills)"
  - "Used the Read tool (read-file) + Grep tool (search-symbols) shapes to cover 5 distinct steering shapes without broadening the classifier (STEER-01/Phase 98 is out of scope)"

patterns-established:
  - "Revert-and-fail factors the gate into a pure helper (referenceMissingVerbs) so the green test and the adversarial test run IDENTICAL logic against intact vs mutated bytes"

requirements-completed: [ADOPT-01]

# Metrics
duration: ~5min
completed: 2026-06-22
status: complete
---

# Phase 97 Plan 02: Deterministic Adoption Contract (ADOPT-01) Summary

**Two merge-gating, non-vacuous contracts: (a) reference.md covers every frozen verb sourced from VerbToolNames() as authority, and (b) each standard-tool shape steers to the SPECIFIC emitted `helix <verb>` keyed on the command — both with mandatory revert-and-fail proofs and an empty-bucket floor, replacing the weak `Contains(...,"helix")` assertion.**

## Performance

- **Duration:** ~5 min
- **Completed:** 2026-06-22T13:51:33Z
- **Tasks/Features:** 2 (ADOPT-01a, ADOPT-01b)
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

### ADOPT-01a — Reference completeness gate (authority = VerbToolNames)
- `internal/cli/reference_contract_test.go` (new). `TestReferenceCoversEveryVerb` iterates `cli.VerbToolNames()` (the 50 frozen verbs = authority, NOT refgen's own output — 97-RESEARCH Pitfall 2) and asserts the embedded `reference.md` has each verb's own section header `## \`helix <verb>\``. Anchors `len(authority) == 50` so a future verb addition without a regen trips both this gate and REF-03's `--check`.
- `referenceMissingVerbs(refBytes, authority)` helper factors the check so the revert sibling runs IDENTICAL logic. Keys on the section header (not a bare substring) so a verb mentioned only in another verb's prose does not count as covered.
- `TestReferenceCompletenessRevertFails` (mandatory revert-and-fail, T-97-06): strips one verb's section header in-memory and asserts the SAME helper reports exactly that verb missing — proving the gate is registry-sourced and CAN go RED.

### ADOPT-01b — Per-shape nudge golden, keyed on the emitted command
- `internal/cli/nudge_test.go` (extended). `TestNudgeShapeGolden` maps the five live steering shapes to the SPECIFIC emitted verb, keyed on the command: bash-grep→`search-symbols`, bash-grep-r→`find-references`, bash-find→`find-files`, read-tool→`read-file`, grep-tool→`search-symbols`. Empty-bucket floor `require.GreaterOrEqual(len(cases), 5)` + per-shape `t.Run` assertions; exit-0 (`runNudge` returns nil) asserted per shape.
- Negative control folded in (`grep TODO README.md` → no advisory). Silent Bash sed/cat shapes pinned as explicit sub-cases (DEFER-97-01) so STEER-01 makes them fire as a visible test change rather than silent drift.
- `TestNudgeGoldenRevertFails` (mandatory revert-and-fail): the read shape must NOT contain `helix rename-symbol` and MUST contain `helix read-file` — proving the golden keys on the specific verb, not a `helix` substring.
- **Replaced** (not merely augmented) the weak `Contains(..., "helix")` at `nudge_test.go:344` with a specific-verb assertion (`helix search-symbols`).

## Task Commits

1. **ADOPT-01a completeness gate + revert-and-fail** — `f311be54` (test)
2. **ADOPT-01b per-shape nudge golden + revert-and-fail + weak-assertion replacement** — `bcfe57e5` (test)

## Files Created/Modified
- `internal/cli/reference_contract_test.go` (new) — `readEmbeddedReference`, `referenceSectionHeader`, `referenceMissingVerbs`, `TestReferenceCoversEveryVerb`, `TestReferenceCompletenessRevertFails`.
- `internal/cli/nudge_test.go` (modified) — strengthened the weak `:344` assertion; added `nudgeShapeCase`/`nudgeShapeGoldenCases`, `TestNudgeShapeGolden` (5 shapes + negative control + 2 DEFER-97-01 silent sub-cases), `TestNudgeGoldenRevertFails`.
- `.planning/phases/97-.../deferred-items.md` (new) — DEFER-97-01.

## Decisions Made
- **Section-header-anchored completeness:** keying on `## \`helix <verb>\`` (not a bare kebab substring) is what makes `TestReferenceCompletenessRevertFails` genuine — kebab tokens appear 3× per verb (header + example + use-this-not-that prose), so a substring check could pass on prose alone. The section-header anchor ensures dropping a verb's own section turns it RED.
- **Golden asserts live behavior, not aspirational branches:** see Deviations / DEFER-97-01.

## Deviations from Plan

### [Rule 1 - discovered behavior] Bash `sed`/`cat` shapes are silent today; golden asserts live behavior, not bashSteerMessage's dead branches
- **Found during:** ADOPT-01b golden authoring.
- **Issue:** The plan's golden table (from 97-PATTERNS.md / 97-RESEARCH.md, authored off `bashSteerMessage`) maps Bash `cat`→`read-file` and `sed -i`→`replace-in-file`. But `runNudge` (nudge.go:105) only reaches `bashSteerMessage` when `isGrepReadTool()` is true, and its Bash arm (nudge.go:434-438) matches ONLY `grep`/`find`/`rg`/`ag` — NOT `sed`/`cat`. So Bash `sed`/`cat` emit NO advisory today; the `cat`/`sed` branches in `bashSteerMessage` are dead for Bash callers.
- **Resolution:** The plan explicitly forbids broadening the classifier ("Do NOT broaden the classifier (that is STEER-01, Phase 98) — assert only the current mapping"). Asserting Bash sed/cat → a verb would be a FALSE golden — exactly the vacuity this phase exists to kill. So the golden asserts the five shapes that genuinely steer (using the Read tool for the cat-equivalent `read-file` shape and the Grep tool for the grep-tool `search-symbols` shape), pins the silent Bash sed/cat shapes as explicit sub-cases, and documents the gap as DEFER-97-01 routed to STEER-01/Phase 98. No classifier code changed; `bashSteerMessage`/`steerMessage` are untouched.
- **Files modified:** `internal/cli/nudge_test.go` (test only); `deferred-items.md` (new).
- **Commit:** `bcfe57e5`.

This is a faithful, in-scope adaptation: the contract still covers ≥5 distinct steering shapes, keyed on the emitted command, with both revert-and-fail proofs and an empty-bucket floor. It simply refuses to assert behavior that does not exist.

## Issues Encountered
- The initial golden (Bash sed/cat → verb) went RED because those shapes don't fire — surfaced the `isGrepReadTool` gate vs `bashSteerMessage` mismatch above. Resolved by asserting live behavior + DEFER-97-01.

## Anti-Vacuity Verification (the phase's heart)
- **Completeness RED-ability proven:** `TestReferenceCompletenessRevertFails` runs `referenceMissingVerbs` against a copy with one verb's section deleted and asserts it reports that verb missing.
- **Golden RED-ability proven:** beyond `TestNudgeGoldenRevertFails`, I temporarily flipped the bash-grep `wantVerb` to `helix rename-symbol` and confirmed `TestNudgeShapeGolden/bash-grep` FAILED, then restored to GREEN.
- **Empty-bucket rejected:** `require.GreaterOrEqual(len(cases), 5)` + per-shape `t.Run` (never an empty iteration).
- **Authority = registry:** completeness sourced from `VerbToolNames()` (len pinned to 50), never refgen's own output.

## User Setup Required
None.

## Next Phase Readiness
- ADOPT-01 is a merge gate in the default untagged `go test` suite. STEER-01 (Phase 98) owns the decision on whether Bash `sed`/`cat` should fire; when it does, the `bash-sed-i-silent` / `bash-cat-silent` sub-cases in `TestNudgeShapeGolden` must flip to firing assertions (per DEFER-97-01).
- Invariants intact: `git diff go.mod` empty; `go run ./cmd/helix-refgen --check` clean; `go vet ./...` clean; `go test ./...` green.

## Self-Check: PASSED

- `internal/cli/reference_contract_test.go` exists on disk; `internal/cli/nudge_test.go` modified.
- Both task commits present in git history (`f311be54`, `bcfe57e5`).
- `go vet ./...` clean; `go test ./...` green; `go test ./internal/cli/...` green; `go run ./cmd/helix-refgen --check` clean; `git diff go.mod` empty.

---
*Phase: 97-generated-per-verb-reference-deterministic-adoption-contract*
*Completed: 2026-06-22*
