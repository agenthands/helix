---
phase: 97-generated-per-verb-reference-deterministic-adoption-contract
verified: 2026-06-22T00:00:00Z
status: passed
score: 7/7 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification: # none — initial verification
gaps: []
deferred:
  - truth: "Bash sed/cat shapes steer to helix replace-in-file / read-file"
    addressed_in: "Phase 98 (STEER-01)"
    evidence: "DEFER-97-01 — isGrepReadTool's Bash arm matches only grep/find/rg/ag today; broadening the classifier is explicitly Phase 98 scope. Phase 97 plan forbids broadening ('Do NOT broaden the classifier — that is STEER-01, Phase 98'). The golden honestly pins bash-sed-i-silent / bash-cat-silent as silent sub-cases rather than asserting non-existent behavior."
---

# Phase 97: Generated Per-Verb Reference + Deterministic Adoption Contract Verification Report

**Phase Goal:** An agent has a complete, registry-generated per-verb reference installed alongside the terse skill, and a deterministic merge-gating contract proves both that the reference covers every frozen verb and that the nudge steers each standard-tool shape to the specific correct `helix` verb.
**Verified:** 2026-06-22
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | `go run ./cmd/helix-refgen` produces a `reference.md` covering every one of the 50 frozen verbs (synopsis, args, output shape, worked example, use-this-not-that) | ✓ VERIFIED | `grep -cE '^## \`helix '` → 50 sections; reference.md is 975 lines. Spot-check of `helix rename-symbol` section shows all 5 components, Args sourced from the verb catalog (real flag names/kinds/help, e.g. `--receipts (string-slice, optional)`), NOT InputSchema. |
| 2 | `helix-refgen --check` exits 1 on a stale/hand-edited reference and 0 in sync | ✓ VERIFIED | Committed reference: `--check` → "up to date", exit 0. Independently tampered (removed `search-symbols` section): `--check` → "out of date", exit 1. Restored → exit 0, tree clean. |
| 3 | `make verify-reference` runs the drift gate; CI runs it as a sibling of cligen/docgen gates | ✓ VERIFIED | `make verify-reference` → exit 0. Makefile:89-90 `verify-reference` HARD-FAIL target + :80-81 `reference` regen target. `.github/workflows/go-test.yml:114-119` "helix-refgen drift gate (REF-03)" step, sibling to cligen (:102) and docgen (:108). |
| 4 | `helix setup` installs BOTH SKILL.md and reference.md atomically with withinSkillRoot intact | ✓ VERIFIED | skill.go switched to `//go:embed skills/helix/*` + `embed.FS`; `installSkill` walks the FS, per-file temp+rename atomic write, entry names from embed only. `withinSkillRoot`/`lexicalSkillRoot`/`containedIn` intact (skill.go:152,205-232). `TestInstallSkillWritesBundle` + `TestInstallSkillContainment` PASS. uninstallSkill removes both entries. |
| 5 | EmbeddedSkillBody() returns SKILL.md only; SKILL-04 idle-cost ≤1536 cap still on SKILL.md's description | ✓ VERIFIED | `EmbeddedSkillBody()` → `embeddedSkillBytes()` reads `skills/helix/SKILL.md` only. `TestEmbeddedSkillBody` proves no reference.md banner bleed. `TestSkillIdleCostBound` + `TestSkillDescriptionCap` (cap1536) read `embeddedSkillBytes()` (SKILL.md) — both PASS. |
| 6 | Completeness gate iterates `cli.VerbToolNames()` (50 frozen verbs = authority, NOT refgen output); deleting a verb's section turns it RED | ✓ VERIFIED | `reference_contract_test.go:54` authority = `VerbToolNames()`, `require.Equal(t, 50, len(authority))`. Keys on section header `## \`helix <verb>\`` (not bare substring). Independently removed `find-references` section from embedded file → `TestReferenceCoversEveryVerb` FAILED (line 71). `TestReferenceCompletenessRevertFails` PASS. |
| 7 | Per-shape nudge golden maps each standard-tool shape to the SPECIFIC emitted `helix <verb>` (keyed on command, not "helix" substring); breaking a mapping turns it RED; empty-bucket rejected; negative control silent; exit-0 preserved | ✓ VERIFIED | `TestNudgeShapeGolden` 5 shapes → specific verbs (bash-grep→search-symbols, bash-grep-r→find-references, bash-find→find-files, read-tool→read-file, grep-tool→search-symbols). `require.GreaterOrEqual(len(cases),5)`. Per-shape `require.NoError` (exit-0). Negative control `grep TODO README.md` asserts `ok==false`. Weak `Contains(...,"helix")` at :344 replaced with `helix search-symbols`. Independently flipped bash-grep wantVerb to a wrong verb → golden FAILED, exit 1. `TestNudgeGoldenRevertFails` PASS. |

**Score:** 7/7 truths verified (0 present, behavior-unverified)

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | Bash `sed -i` / `cat` shapes steer to a `helix` verb | Phase 98 (STEER-01) | DEFER-97-01: `isGrepReadTool`'s Bash arm matches only grep/find/rg/ag today. Phase 97 plan explicitly forbids broadening the classifier. The golden honestly pins `bash-sed-i-silent` / `bash-cat-silent` as silent sub-cases (not false-asserted behavior) — this is the non-vacuity the phase exists to enforce, not a gap. |

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | ----------- | ------ | ------- |
| `cmd/helix-refgen/main.go` | Generator entrypoint, --out/--check, daemon-parity blank imports | ✓ VERIFIED | `func main`, flags present; blank-import set IDENTICAL to `cmd/docgen/main.go` (11 imports, the stated authority — T-97-04 anti-drift). |
| `cmd/helix-refgen/render.go` | renderReference() registry walk + args from cli accessor | ✓ VERIFIED | `renderReference()` reads `VerbSpecsForDocs()`, pipe-escapes Descriptions; deterministic (covered by `cmd/helix-refgen` tests, PASS). |
| `internal/cli/skills/helix/reference.md` | Generated per-verb reference (≥100 lines) | ✓ VERIFIED | 975 lines, 50 verb sections, generator banner "DO NOT EDIT". `--check` clean. |
| `internal/cli/verb.go` | VerbSpecsForDocs() fresh-copy accessor | ✓ VERIFIED | `VerbSpecsForDocs()` (verb.go:142), VerbDoc/FlagDoc, sorted deep-copy. `TestVerbSpecsForDocs_*` (count/fields/read-only) PASS. |
| `internal/cli/skill.go` | embed.FS bundle + multi-file atomic install | ✓ VERIFIED | `embed.FS` (skill.go:22), multi-file walk install/uninstall, path guard intact. |
| `internal/cli/reference_contract_test.go` | ADOPT-01a completeness + revert-and-fail (authority=VerbToolNames) | ✓ VERIFIED | `VerbToolNames` authority, section-header keyed, revert sibling present. Runs in `package cli` (untagged). |
| `internal/cli/nudge_test.go` | ADOPT-01b per-shape golden + revert + empty-bucket floor | ✓ VERIFIED | `GreaterOrEqual(len(cases),5)`, per-shape t.Run, revert-and-fail, weak assertion replaced. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| render.go | verb.go (VerbSpecsForDocs) | refgen reads args from accessor, not InputSchema | ✓ WIRED | `VerbSpecsForDocs` consumed in render; mcp.ToolDef has no InputSchema (RESEARCH Pitfall 1 respected). |
| main.go | internal/skill (ToolProviders) | blank-import set == docgen | ✓ WIRED | 11 imports identical to docgen; daemon delta (guardrails) provides 0 tools — no verb drop (completeness gate=50 confirms). |
| skill.go (installSkill) | skills/helix/* (embed.FS) | walks embedded FS, atomic per-file write within skill root | ✓ WIRED | `embeddedSkillFS.ReadDir("skills/helix")` → temp+rename per entry; `TestInstallSkillWritesBundle` PASS. |
| reference_contract_test.go | verb.go (VerbToolNames) + skill.go (embedded reference.md) | iterate 50-verb authority, assert each section present | ✓ WIRED | Test reads `embeddedSkillFS` reference.md, iterates `VerbToolNames()`. |
| nudge_test.go | nudge.go (bashSteerMessage/steerMessage) | runNudgeCapture asserts specific verb per shape | ✓ WIRED | Golden parses advisory, asserts `tc.wantVerb`. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| go build | `go build ./...` | exit 0 | ✓ PASS |
| go vet | `go vet ./...` | exit 0 | ✓ PASS |
| Full untagged suite | `go test ./... -count=1` | exit 0 (no failures) | ✓ PASS |
| cli + refgen suites | `go test ./internal/cli/... ./cmd/helix-refgen/... -count=1` | both `ok` | ✓ PASS |
| Drift gate (committed) | `go run ./cmd/helix-refgen --check` | exit 0 "up to date" | ✓ PASS |
| Drift gate (tampered) | removed a verb section, `--check` | exit 1 "out of date" | ✓ PASS (RED proven) |
| make verify-reference | `make verify-reference` | exit 0 | ✓ PASS |
| Completeness RED | deleted embedded `find-references` section, ran gate | FAIL (line 71) | ✓ PASS (RED proven) |
| Golden RED | flipped bash-grep wantVerb to wrong verb | FAIL exit 1 | ✓ PASS (RED proven) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| REF-01 | 97-01 | helix-refgen generates reference.md covering every verb (synopsis/args/output/example/use-this-not-that) | ✓ SATISFIED | 50 sections, 5 components each, args from VerbSpecsForDocs. |
| REF-02 | 97-01 | skill.go → embed.FS; installSkill ships reference.md alongside SKILL.md atomically w/ path containment | ✓ SATISFIED | embed.FS switch, multi-file atomic install, withinSkillRoot intact, both files land. |
| REF-03 | 97-01 | helix-refgen --check merge-gating drift gate (make + CI), docgen blank-import parity | ✓ SATISFIED | --check fails on stale, make verify-reference + CI step present, imports == docgen. |
| ADOPT-01 | 97-02 | deterministic merge-gating contract: reference ⊇ VerbToolNames + per-shape nudge golden, revert-and-fail, no empty-bucket | ✓ SATISFIED | Both gates in untagged package cli, both revert-and-fail proven RED, empty-bucket floor, negative control. |

All 4 requirement IDs from PLAN frontmatter map to REQUIREMENTS.md (lines 108-117, all "Phase 97"). No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | none | — | No unreferenced debt markers (TBD/FIXME/XXX) in modified source. "TODO" matches in nudge_test.go are test-data strings (`grep TODO README.md` negative-control input), not debt markers. |

### Human Verification Required

None — all phase behaviors have automated verification, and the merge-gating / anti-vacuity invariants were independently re-proven by tampering (not just by reading the tests).

### Gaps Summary

No gaps. All 7 must-haves are VERIFIED against the live tree via real command execution:
- The reference is registry-generated, complete (50/50 verbs, all 5 components), and `--check` independently proven to go RED on a stale/hand-edited file.
- The embed.FS bundle ships both files atomically with the SKILL-04 cap and withinSkillRoot guard intact.
- The adoption contract runs in the untagged `go test ./...` suite (CI line 100) so it BLOCKS merge; its completeness gate is sourced from `VerbToolNames()` (the authority, not refgen output) and the per-shape nudge golden keys on the specific emitted verb. Both anti-vacuity revert-and-fail invariants were independently re-proven RED.
- Zero-dep (`git diff go.mod` empty) and no-wire invariants intact.

The single deferred item (DEFER-97-01: Bash sed/cat shapes silent) is correctly out of scope (Phase 98 STEER-01) and is honestly pinned by the golden as silent — this is the phase's non-vacuity discipline working as designed, not a gap.

---

_Verified: 2026-06-22_
_Verifier: Claude (gsd-verifier)_
