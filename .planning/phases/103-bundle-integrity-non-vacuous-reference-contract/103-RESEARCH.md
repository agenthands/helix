# Phase 103: Bundle Integrity & Non-Vacuous Reference Contract - Research

**Researched:** 2026-06-24
**Domain:** Go `embed.FS` bundle hygiene + anti-vacuity test hardening (internal/cli)
**Confidence:** HIGH

## Summary

Phase 103 is a small, mechanical, well-specified safety patch with **zero new dependencies** and the smallest possible blast radius in the v2.2 milestone. The technical grounding already exists in the v2.2 milestone research (`.planning/research/{SUMMARY,ARCHITECTURE,PITFALLS}.md`); this RESEARCH.md consolidates it and pins every code seam against the **live tree** (verified 2026-06-24).

Three deliverables, all verified against source:
1. **Stop the live embed leak.** `internal/cli/skill.go` uses `//go:embed skills/helix/*` and both `installSkill`/`uninstallSkill` iterate **every** `ReadDir("skills/helix")` entry. The directory currently contains a third file — `SKILL-ISSUE.md` (18,460 bytes, confirmed present, **git-untracked**) — which therefore compiles into the binary AND is written to disk by `helix setup`. Fix: a closed-set `bundleFiles = {"SKILL.md","reference.md"}` allowlist applied **once** at the top of each ReadDir loop, plus moving `SKILL-ISSUE.md` out of the embed dir.
2. **Make the bundle test a closed-set assertion.** The existing `TestInstallSkillWritesBundle` (skill_test.go:317) is positive-only (asserts the two files are present). It must additionally assert the installed set equals **exactly** `{SKILL.md, reference.md}` and go RED on any stray installed file.
3. **Harden the reference-completeness contract (BUNDLE-02).** `internal/cli/reference_contract_test.go` already asserts `require.Equal(t, 50, len(VerbToolNames()))` and an exact-count revert proof. Verified: it is **already substantially hardened** — the planner must confirm the exact-count + known-absent-verb discriminators are present and add any missing anti-vacuity anchor (see BUNDLE-02 analysis below) so the gate cannot pass `∅ ⊇ ∅` through the Phase 104/105 churn.

**Primary recommendation:** Apply a single `bundleFiles` allowlist set (defined once in skill.go, consumed by both install + uninstall loops) as a pre-filter that leaves the 2-pass stage→rename atomicity and `withinSkillRoot` containment **byte-for-byte unchanged**; move `SKILL-ISSUE.md` to `.planning/` (working-tree move — it is untracked, so no `git mv`); upgrade `TestInstallSkillWritesBundle` in place to a closed-set assertion; confirm/complete the reference-contract hardening. Drive each gate RED→GREEN per `workflow.tdd_mode`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Embed bundle definition (`//go:embed`) | Layer 3 (Agent Setup, `internal/cli/skill.go`) | — | The skill bundle is a setup-surface asset compiled into the binary |
| Install/uninstall file writes | Layer 3 (`internal/cli/skill.go`) | OS filesystem | `installSkill`/`uninstallSkill` own atomic disk writes + containment |
| Reference completeness authority | Layer 3 (`internal/cli/verb.go` `VerbToolNames()`) | `cmd/helix-refgen` | 50 frozen verbs are the single source of truth; refgen renders, never authors completeness |
| Bundle/contract test gates | Layer 3 test files (`internal/cli/*_test.go`) | — | Pure in-package Go tests; no runtime tier involved |

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Embed leak is LIVE** (verified): `//go:embed skills/helix/*` + `installSkill` ReadDir-loop ships `SKILL-ISSUE.md`. Fix = closed-set `bundleFiles = {SKILL.md, reference.md}` allowlist applied **once** up front to **both** install and uninstall loops, placed **before** the 2-pass stage→rename, leaving atomicity and `withinSkillRoot` containment **UNCHANGED**. Move `SKILL-ISSUE.md` OUT of `internal/cli/skills/helix/` (to `.planning/`), updating the ROADMAP Backlog reference path.
- **Bundle test must be CLOSED-SET**: `TestInstallSkillWritesBundle` must assert the installed set equals **exactly** `{SKILL.md, reference.md}` and go RED on any stray file (a deliberate stray-file test runs RED before the fix — anti-vacuity).
- **Harden reference-completeness contract (BUNDLE-02)**: `reference_contract_test.go` must assert **exact count** `== len(VerbToolNames())` (currently 50) AND discriminate a **known-absent verb** (RED when a frozen verb is missing). Lands in Phase 103 so the gate cannot pass `∅ ⊇ ∅` vacuously through 104/105.
- **TDD ON** (`workflow.tdd_mode`): RED→GREEN→REFACTOR for the testable gates.
- **Preserve invariants**: the `## Decision matrix` `StripDecisionMatrix` anchor in SKILL.md, the `helix-refgen --check` byte-reproducibility, and `reference ⊇ VerbToolNames()` all stay green. **ZERO new Go deps** (edits inside already-vendored packages).

### Claude's Discretion
Discuss was skipped (`workflow.skip_discuss`). Plan around the locked constraints above using the ROADMAP Phase 103 goal + success criteria, the v2.2 research, and codebase conventions. Open discretion areas:
- Exact one-place to apply the allowlist filter in both install + uninstall.
- Whether the closed-set bundle assertion belongs in the existing test or a new one.
- The precise RED-first construction for the hardened contract test (delete/rename a verb vs assert a fabricated-absent verb).

### Deferred Ideas (OUT OF SCOPE)
None for this phase. Generator fixes are Phase 104 (REFGEN-01); SKILL.md decision-matrix rewrite is Phase 105 (SKILL-01/02/03); DSPy harness is Phase 106 (TUNE-01). Do not touch refgen `render.go`, the SKILL.md decision matrix, or `tools/`.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BUNDLE-01 | `helix setup` installs exactly `{SKILL.md, reference.md}`; `installSkill`/`uninstallSkill` use a closed-set allowlist; bundle test asserts installed set == exactly the two files (RED on stray); `SKILL-ISSUE.md` moved out of embed dir | Verified embed leak (`skill.go:21,189,212-236,320-332`), `SKILL-ISSUE.md` present + untracked, closed-set test seam (`skill_test.go:317-340`), no non-planning consumer of `SKILL-ISSUE.md`, all consumers read by filename not dir-listing |
| BUNDLE-02 | Reference-completeness contract hardened to **exact count** (`== len(VerbToolNames())`, currently 50) + RED on a known-absent verb — landed before the 104/105 format churn | Verified `reference_contract_test.go` already asserts `require.Equal(t,50,...)` (line 58) and a one-verb revert proof (lines 82-104); planner confirms/extends the known-absent discriminator |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `embed` | (toolchain) | `//go:embed skills/helix/*` bundle | Already used; single-binary identity [VERIFIED: skill.go:4,21] |
| Go stdlib `os`, `path/filepath` | (toolchain) | atomic temp+rename writes, containment | Already used in `installSkill` [VERIFIED: skill.go:5-7] |
| `testing` (stdlib) | (toolchain) | test gates | Project standard [VERIFIED: skill_test.go] |
| `github.com/stretchr/testify` | (vendored) | `require`/`assert` in contract test | Already imported [VERIFIED: reference_contract_test.go:7-8] |

**No new dependencies.** `git diff go.mod go.sum` must be empty after this phase [CITED: REQUIREMENTS.md "Out of Scope — New Go module dependencies"].

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Allowlist pre-filter on `//go:embed skills/helix/*` | Narrow the embed glob to `//go:embed skills/helix/SKILL.md skills/helix/reference.md` | Narrowing the glob also stops the leak, but the runtime allowlist is defense-in-depth (a future stray file added to the dir is still filtered at install time) AND keeps a single closed-set source the test can reference. Research recommends **both** glob discipline (via moving the stray file out) and the runtime allowlist; the allowlist is the locked decision. |

**Installation:** No packages to install.

## Package Legitimacy Audit

Not applicable — this phase installs **no external packages**. All edits are inside already-vendored `internal/cli` and its test files. `go.mod`/`go.sum` unchanged.

## Architecture Patterns

### System Architecture Diagram

```
build time                          helix setup (runtime)
----------                          ---------------------
skills/helix/                       installSkill(targetDir)
  SKILL.md      ──┐                   │
  reference.md  ──┤ //go:embed        ├─ withinSkillRoot(targetDir)  [UNCHANGED guard]
  SKILL-ISSUE.md ─┘ skills/helix/*    │
        │            (LEAK today)     ├─ ReadDir("skills/helix")
        ▼                            │     │
  embed.FS embeddedSkillFS          │     ▼
        │                            │  ── bundleFiles filter ──  [NEW: applied ONCE]
        │                            │     │  keep only {SKILL.md, reference.md}
        ▼                            │     ▼
  binary ships ALL 3 files          ├─ Pass 1: stage each → <dst>.tmp  [UNCHANGED]
  (SKILL-ISSUE.md leaks)            ├─ Pass 2: rename .tmp → dst       [UNCHANGED]
                                    ▼
                          <claudeDir>/skills/helix/{SKILL.md, reference.md}
                                    (exactly two files — closed set)

FIX: move SKILL-ISSUE.md → .planning/  ⇒ embed glob no longer matches it
     AND bundleFiles allowlist ⇒ even a future stray dir file is filtered out
```

### Recommended Project Structure
No new files required by the locked plan. Optional new test file is at the planner's discretion (see BUNDLE-01 below). Affected paths:
```
internal/cli/
├── skill.go                      # add bundleFiles set; filter both ReadDir loops
├── skill_test.go                 # upgrade TestInstallSkillWritesBundle to closed-set
├── reference_contract_test.go    # confirm/extend exact-count + known-absent discriminator
└── skills/helix/
    ├── SKILL.md                  # UNTOUCHED
    └── reference.md              # UNTOUCHED
.planning/
└── SKILL-ISSUE.md                # MOVED here from internal/cli/skills/helix/
```

### Pattern 1: Closed-set allowlist as a one-place pre-filter
**What:** Define `bundleFiles` once; both loops consult it before any staging/removal.
**When to use:** Always — it is the locked decision and keeps install/uninstall symmetric.
**Example (illustrative — exact placement at planner discretion):**
```go
// Source: derived from VERIFIED skill.go:189-236, 320-332
// Defined once at package level (single source of truth the test can reference).
var bundleFiles = map[string]struct{}{
    "SKILL.md":     {},
    "reference.md": {},
}

// inside installSkill, immediately after `entries, err := ReadDir(...)`:
for _, e := range entries {
    if e.IsDir() {
        continue
    }
    if _, ok := bundleFiles[e.Name()]; !ok {
        continue // closed-set allowlist: ignore non-bundle files
    }
    // ... existing Pass-1 staging body UNCHANGED ...
}
// identical filter clause added to uninstallSkill's loop (skill.go:324-332).
```
**Key:** the filter is a `continue` guard at the top of the existing loop body — it does **not** restructure the 2-pass stage→rename (skill.go:200-256) nor `withinSkillRoot` (skill.go:273-306). Both invariants are preserved verbatim.

### Anti-Patterns to Avoid
- **Re-deriving the embed glob in the test from the directory listing** — the completeness/closed-set authority must be the explicit `{SKILL.md, reference.md}` set or `bundleFiles`, never `ReadDir` of the source dir (a set-compared-to-itself tautology) [CITED: reference_contract_test.go:48-52 Pitfall 2 note].
- **Sourcing the reference-completeness check from refgen's own output** — already correctly avoided: authority is `VerbToolNames()` [VERIFIED: reference_contract_test.go:53-59].
- **`git mv` on `SKILL-ISSUE.md`** — it is **untracked** (verified `git ls-files` empty, `git status` shows `??`). Use a plain filesystem move and `git add` at the destination.
- **Touching SKILL.md or the `## Decision matrix` anchor** — out of scope for 103; that is Phase 105.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic multi-file bundle write | A new write path | Existing 2-pass stage→rename (skill.go:194-256) | Already correct (WR-97-02); the allowlist is a pre-filter only |
| Path-traversal containment | A new guard | Existing `withinSkillRoot`/`lexicalSkillRoot`/`containedIn` (skill.go:273-306) | Already correct (T-93-01/T-97-01); unchanged |
| Completeness authority | A hardcoded verb list | `VerbToolNames()` (verb.go:81) | 50 frozen verbs, single source of truth |

**Key insight:** This phase changes **what files** the existing machinery processes, not **how** it processes them. Every safety invariant is pre-existing and must be preserved byte-for-byte.

## Common Pitfalls

### Pitfall 1: Vacuous closed-set assertion
**What goes wrong:** A bundle test that only checks "both files present" passes even when `SKILL-ISSUE.md` also installs.
**Why it happens:** The current `TestInstallSkillWritesBundle` iterates an expected list, not the actual installed directory.
**How to avoid:** Assert the **actual** installed dir listing (`os.ReadDir(targetDir)`) equals exactly `{SKILL.md, reference.md}`. Anti-vacuity proof: before the allowlist lands (RED), the install writes 3 files and the closed-set assertion fails.
**Warning signs:** Test passes with `SKILL-ISSUE.md` still in the embed dir.

### Pitfall 2: Asymmetric allowlist (install filtered, uninstall not)
**What goes wrong:** `uninstallSkill` tries to remove `SKILL-ISSUE.md` and either errors or silently leaves user state inconsistent.
**Why it happens:** Filtering only one of the two ReadDir loops.
**How to avoid:** Apply the **identical** `bundleFiles` filter clause to both loops (skill.go:212 and skill.go:324). `uninstallSkill` already tolerates `os.IsNotExist`, but the filter keeps the two surfaces symmetric and intent-explicit.
**Warning signs:** Uninstall behavior differs between bundle and non-bundle files.

### Pitfall 3: Breaking byte-reproducibility / the StripDecisionMatrix anchor
**What goes wrong:** An incidental SKILL.md or reference.md edit breaks `helix-refgen --check` or the `## Decision matrix` split consumed by `test/oracle/adopt/scorecard.go` (`StripDecisionMatrix`, line 103) and `internal/cli/prime.go`.
**Why it happens:** Editing assets that 103 must leave untouched.
**How to avoid:** Do not modify `skills/helix/SKILL.md` or `reference.md` at all. Verified green now: `go run ./cmd/helix-refgen -check` → "reference.md is up to date." (exit 0).
**Warning signs:** `helix-refgen -check` exits non-zero; `test/oracle/adopt` tests fail.

## Code Examples

### Closed-set bundle assertion (the RED-first BUNDLE-01 gate)
```go
// Source: pattern over VERIFIED installSkill (skill.go) + skill_test.go:320
func TestInstallSkillInstallsExactlyBundle(t *testing.T) {
    dir := skillTargetDir(t.TempDir())
    if err := installSkill(dir); err != nil {
        t.Fatalf("installSkill: %v", err)
    }
    entries, err := os.ReadDir(dir)
    if err != nil {
        t.Fatalf("ReadDir(%s): %v", dir, err)
    }
    var got []string
    for _, e := range entries {
        if !e.IsDir() {
            got = append(got, e.Name())
        }
    }
    sort.Strings(got)
    want := []string{"SKILL.md", "reference.md"}
    sort.Strings(want)
    assert.Equal(t, want, got,
        "helix setup must install EXACTLY the bundle; any extra file is a leak")
}
// RED today: with SKILL-ISSUE.md still embedded, got == [SKILL-ISSUE.md SKILL.md reference.md].
```

### Known-absent-verb discriminator (the BUNDLE-02 anti-vacuity proof, already largely present)
```go
// Source: VERIFIED reference_contract_test.go:82-104 (revert-and-fail proof)
// The existing TestReferenceCompletenessRevertFails drops ONE real verb section
// in-memory and asserts the SAME helper reports exactly that verb missing.
// This proves the gate is non-vacuous and CAN go RED. The exact-count anchor
// (require.Equal(t, 50, len(VerbToolNames())) at line 58) prevents ∅ ⊇ ∅.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single-file SKILL.md install | 2-pass atomic multi-file bundle (SKILL.md + reference.md) | Phase 97 (WR-97-02) | This phase adds a closed-set filter on top — does not change the 2-pass mechanism |
| `reference ⊇ VerbToolNames()` (superset) | exact-count `== 50` + per-verb section + revert proof | Phase 97 (ADOPT-01a) → confirmed live | Already hardened; 103 confirms/seals it before the 104/105 churn |

**Deprecated/outdated:** None relevant to this phase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `.planning/` is the chosen destination for `SKILL-ISSUE.md` (ARCHITECTURE.md recommends it; `docs/` also exists with only `images/`+`runbooks/`) | BUNDLE-01 | Low — destination is cosmetic; any non-embedded path satisfies the requirement. Planner picks; `.planning/` matches research recommendation. |
| A2 | The exact `bundleFiles` placement (package-level var vs in-func literal) is at planner discretion | Pattern 1 | Low — both satisfy "applied once"; package-level lets the test reference the same set. |

**Note:** No `[ASSUMED]` factual/technical claims — every code-seam claim is `[VERIFIED]` against the live tree. The two assumptions above are scope/style choices the planner resolves, not unverified facts.

## Open Questions

1. **One-place allowlist: package-level `var bundleFiles` vs a local set in each function?**
   - What we know: both install (skill.go:212) and uninstall (skill.go:324) loops need the identical filter.
   - What's unclear: whether to share one package-level set (lets the closed-set test import it) or inline per function.
   - Recommendation: package-level `var bundleFiles` so the closed-set test asserts against the **same** authority (DRY, non-tautological because the test also checks the **installed disk** listing).

2. **New test file vs extend `TestInstallSkillWritesBundle`?**
   - What we know: ARCHITECTURE.md line suggests a new `internal/cli/skill_bundle_test.go`; the existing positive-only test is at skill_test.go:317.
   - What's unclear: whether to add a new closed-set test alongside or replace the body.
   - Recommendation: **add** a distinct closed-set test (keep the existing trailing-newline/non-empty checks; add the exact-set assertion). Either co-located in `skill_test.go` or a new `skill_bundle_test.go` — planner's call. Do not delete the existing assertions.

3. **BUNDLE-02 — is anything actually missing, or is it confirm-only?**
   - What we know: `reference_contract_test.go` ALREADY asserts `require.Equal(t, 50, len(VerbToolNames()))` (line 58) AND a single-verb revert proof that goes RED on a dropped verb (lines 82-104). Both target hardening properties appear present.
   - What's unclear: whether the requirement intends an additional explicit **fabricated-absent-verb** assertion (assert a verb name NOT in the set is reported, proving the membership test discriminates) beyond the existing real-verb-drop proof.
   - Recommendation: Treat BUNDLE-02 as **confirm-and-seal**. The planner should add, RED-first, one explicit discriminator if not already equivalent: e.g., assert `referenceMissingVerbs(ref, append(authority, "totally-not-a-verb"))` reports the fabricated verb — proving the gate keys on the real frozen set and cannot pass vacuously. Keep the existing exact-count `== 50` anchor as the churn tripwire for 104/105.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build + test | ✓ (assumed — repo builds) | host | — |
| `go run ./cmd/helix-refgen -check` | byte-reproducibility gate | ✓ (verified exit 0) | — | — |

No external services. Verified live: `go test ./internal/cli/ -run ...` green; `helix-refgen -check` exit 0; `go vet ./internal/cli/` exit 0.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + `github.com/stretchr/testify` (`require`/`assert`) |
| Config file | none — standard `go test` |
| Quick run command | `go test ./internal/cli/ -run 'InstallSkill|Bundle|Reference' -count=1` |
| Full suite command | `go test ./... && go vet ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BUNDLE-01 | install writes EXACTLY `{SKILL.md, reference.md}` (RED on stray) | unit | `go test ./internal/cli/ -run TestInstallSkillInstallsExactlyBundle -count=1 -v` | ❌ Wave 0 (new closed-set test; existing positive-only `TestInstallSkillWritesBundle` at skill_test.go:317 stays) |
| BUNDLE-01 | uninstall filtered identically; idempotent | unit | `go test ./internal/cli/ -run 'TestInstallSkillIdempotent|Uninstall' -count=1` | ✅ (idempotent test exists; extend for symmetry) |
| BUNDLE-01 | `SKILL-ISSUE.md` no longer embedded | unit | `go test ./internal/cli/ -run TestInstallSkillInstallsExactlyBundle -count=1` (asserts via install set) + `go build ./cmd/helix` | ✅/❌ covered by closed-set test once SKILL-ISSUE.md moved |
| BUNDLE-01 | atomicity + containment preserved | unit | `go test ./internal/cli/ -run 'TestInstallSkillNoTempLeftover|TestInstallSkillContainment' -count=1` | ✅ (skill_test.go:291,304 — must stay green) |
| BUNDLE-02 | exact-count `== len(VerbToolNames())` (50) | unit | `go test ./internal/cli/ -run TestReferenceCoversEveryVerb -count=1 -v` | ✅ (reference_contract_test.go:53, asserts 50) |
| BUNDLE-02 | RED on a known-absent verb (revert proof + fabricated-absent discriminator) | unit | `go test ./internal/cli/ -run 'TestReferenceCompletenessRevertFails' -count=1 -v` | ✅ revert proof exists (lines 82-104); ❌ Wave 0 if explicit fabricated-absent discriminator added |
| Invariant | byte-reproducibility | gate | `go run ./cmd/helix-refgen -check` (expect exit 0) | ✅ (verified green) |
| Invariant | `StripDecisionMatrix` / adopt scorecard unaffected | unit | `go test ./test/oracle/adopt/... -count=1` | ✅ (must stay green) |

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/ -run 'InstallSkill|Bundle|Reference' -count=1`
- **Per wave merge:** `go test ./... && go vet ./...`
- **Phase gate:** full suite green + `go run ./cmd/helix-refgen -check` exit 0 + `go test ./test/oracle/adopt/... -count=1` green, before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/cli/skill_test.go` (or new `internal/cli/skill_bundle_test.go`) — closed-set `TestInstallSkillInstallsExactlyBundle` covering BUNDLE-01 (RED-first: write it before the allowlist + before moving SKILL-ISSUE.md so it fails on the 3-file install).
- [ ] `internal/cli/reference_contract_test.go` — confirm-and-seal BUNDLE-02; add an explicit fabricated-absent-verb discriminator only if the planner judges the existing real-verb-drop proof insufficient (RED-first).
- [ ] No framework install needed — `testing` + `testify` already present.

## Security Domain

`security_enforcement` is not set to `false` in config, so this section is included. This phase is a **defensive** patch — it tightens an existing supply-chain/asset surface.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes (path) | `withinSkillRoot` + `filepath.Rel` containment — UNCHANGED [VERIFIED: skill.go:273-306] |
| V6 Cryptography | no | — |
| V12 File handling | yes | Allowlist (closed set) + entry names sourced ONLY from embed FS (never user input); atomic temp+rename [VERIFIED: skill.go:226-228, 320-332] |

### Known Threat Patterns for Go embed bundle / setup writer
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Stray/unintended asset shipped in binary + to disk (the live leak) | Information Disclosure / Tampering | Closed-set `bundleFiles` allowlist + move stray file out of embed glob (THIS PHASE) |
| Path traversal on install target | Tampering | `withinSkillRoot` lexical-root + `filepath.Rel` ".." guard — preserved unchanged |
| Torn/partial bundle on failed write | Tampering / DoS | 2-pass stage→rename with cleanup — preserved unchanged |

**Note:** The allowlist is *additive* defense-in-depth — even after `SKILL-ISSUE.md` is moved out, a future stray file dropped into `skills/helix/` is still filtered at install time.

## Sources

### Primary (HIGH confidence)
- `internal/cli/skill.go` (live read) — `//go:embed skills/helix/*` (l.21), `installSkill` ReadDir loop + 2-pass (l.180-257), `uninstallSkill` (l.315-339), `withinSkillRoot`/`lexicalSkillRoot`/`containedIn` (l.273-306).
- `internal/cli/skill_test.go` (live read) — `TestInstallSkillWritesBundle` positive-only (l.317-340), containment/no-temp/idempotent tests.
- `internal/cli/reference_contract_test.go` (live read) — exact-count `== 50` (l.58), per-verb section gate, revert-and-fail proof (l.82-104).
- `internal/cli/verb.go` (live read) — `VerbToolNames()` (l.81-88); `internal/cli/verbs_gen.go` — 50 `toolName:` entries (verified count).
- Live tool runs — `go test ./internal/cli/ -run ...` (PASS), `go run ./cmd/helix-refgen -check` (exit 0, "up to date"), `go vet ./internal/cli/` (exit 0), `ls skills/helix/` (SKILL-ISSUE.md 18460 bytes present), `git ls-files`/`git status` (SKILL-ISSUE.md untracked).
- Consumer scan — `grep` confirms `SKILL-ISSUE.md` referenced ONLY in `.planning/` docs; `StripDecisionMatrix` lives in `test/oracle/adopt/scorecard.go:103`; `## Decision matrix` anchor in SKILL.md:35 + prime.go.

### Secondary (MEDIUM confidence)
- `.planning/research/ARCHITECTURE.md` (l.155-180) — allowlist design + move recommendation to `.planning/` + build order.
- `.planning/research/SUMMARY.md`, `.planning/research/PITFALLS.md` — embed leak + positive-only-test pitfall + anti-vacuity discipline.
- `.planning/ROADMAP.md` (l.36,40,53) — Phase 103 backlog + success criteria + SKILL-ISSUE.md prose references to repath.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no deps; all seams verified in-tree.
- Architecture: HIGH — allowlist placement + invariants verified against live `installSkill`/`uninstallSkill`.
- Pitfalls: HIGH — vacuity + asymmetry + reproducibility risks confirmed against live tests and `helix-refgen -check`.

**Research date:** 2026-06-24
**Valid until:** 2026-07-24 (stable; only invalidated if `skills/helix/` contents, `VerbToolNames()` count, or the install machinery change before Phase 103 executes).
