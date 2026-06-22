---
phase: 97-generated-per-verb-reference-deterministic-adoption-contract
plan: 01
subsystem: infra
tags: [code-generation, embed-fs, drift-gate, skill-bundle, cli, registry-walk]

# Dependency graph
requires:
  - phase: 90-cli-verb-spine
    provides: "verbSpecs catalog (verbs_gen.go) + VerbToolNames() authority"
  - phase: 93-skill-on-demand
    provides: "embedded SKILL.md + installSkill path-contained atomic write + SKILL-04 idle-cost cap"
provides:
  - "cmd/helix-refgen: registry-walk generator producing a per-verb reference.md with a --check drift gate"
  - "internal/cli VerbSpecsForDocs() read-only verb->flags accessor (doc-facing, fresh-copy)"
  - "internal/cli/skills/helix/reference.md: committed generated per-verb reference (50 verbs)"
  - "internal/cli/skill.go switched from embedded string to embed.FS multi-file skill bundle"
  - "make verify-reference + CI helix-refgen drift gate (REF-03)"
affects: [97-02 deterministic adoption contract, future verb/description changes]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Registry-as-source generation with a --check drift gate (third instance after docgen/cligen)"
    - "embed.FS multi-file skill bundle with path-contained per-file atomic install"
    - "Doc-facing read-only fresh-copy accessor mirroring VerbToolNames discipline"

key-files:
  created:
    - cmd/helix-refgen/main.go
    - cmd/helix-refgen/render.go
    - cmd/helix-refgen/main_test.go
    - internal/cli/skills/helix/reference.md
  modified:
    - internal/cli/verb.go
    - internal/cli/verb_test.go
    - internal/cli/skill.go
    - internal/cli/skill_test.go
    - internal/cli/setup_test.go
    - Makefile
    - .github/workflows/go-test.yml

key-decisions:
  - "Sourced per-verb args from VerbSpecsForDocs() (verbs_gen.go) NOT tool.InputSchema — mcp.ToolDef has no InputSchema field (RESEARCH Pitfall 1)"
  - "reference.md is a generated build artifact (like verbs_gen.go) produced by the generator, never hand-edited"
  - "reference.md exempt from the SKILL-04 1536-char idle-cost cap; EmbeddedSkillBody() returns SKILL.md only"
  - "Normalized generator output to exactly one trailing newline so the bundle install single-newline policy is meaningful"

patterns-established:
  - "helix-refgen copies the docgen blank-import-parity-with-daemon block verbatim; --check gate is the anti-drift protection"
  - "installSkill/uninstallSkill walk the embedded FS; entry names come only from the embed (never user input), preserving T-93-01 path-traversal posture"

requirements-completed: [REF-01, REF-02, REF-03]

# Metrics
duration: ~35min
completed: 2026-06-22
status: complete
---

# Phase 97 Plan 01: Generated Per-Verb Reference + embed.FS Bundle Substrate Summary

**cmd/helix-refgen generates a 50-verb reference.md from the live tool registry (args from the new VerbSpecsForDocs accessor, not InputSchema), shipped via an embed.FS skill bundle and gated by a `--check` drift gate wired into make + CI.**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-06-22T13:42:21Z
- **Tasks:** 4
- **Files modified:** 11 (4 created, 7 modified)

## Accomplishments
- New `cmd/helix-refgen` binary: walks `skill.ToolProviders()` for synopsis prose and reads `cli.VerbSpecsForDocs()` for per-verb args, rendering a deterministic per-verb reference with synopsis / args / output-shape / worked-example / use-this-not-that per group.
- Exported `VerbSpecsForDocs()` doc-facing accessor (VerbDoc/FlagDoc) in `internal/cli/verb.go`, mirroring `VerbToolNames()`'s sorted fresh-copy discipline; flagKind rendered as a stable lowercase token.
- Committed generated `internal/cli/skills/helix/reference.md` (50 verbs, 975 lines).
- Switched `internal/cli/skill.go` from `//go:embed skills/helix/SKILL.md` string to `//go:embed skills/helix/*` `embed.FS`; `installSkill`/`uninstallSkill` now walk the bundle and write/remove each entry atomically within the skill root. `EmbeddedSkillBody()` / `skillDescription()` / SKILL-04 idle-cost cap remain pinned to SKILL.md only.
- Wired `make verify-reference` (HARD-FAIL) + a `reference` regen target + a CI `helix-refgen drift gate (REF-03)` step sibling to the docgen/cligen gates.

## Task Commits

1. **Task 1: Export VerbSpecsForDocs accessor** - `2b72daee` (feat, TDD: RED+GREEN combined)
2. **Task 2: cmd/helix-refgen renderer + --check gate** - `17f708fb` (feat, TDD: RED+GREEN combined)
3. **Task 3: Generate reference.md + switch skill.go to embed.FS** - `e4c831ff` (feat)
4. **Task 4: Wire verify-reference into make + CI; assert bundle install** - `566a752f` (feat)

## Files Created/Modified
- `cmd/helix-refgen/main.go` - generator entrypoint: docgen-parity blank imports, --out/--check flags, referenceStale().
- `cmd/helix-refgen/render.go` - renderReference(): registry walk + VerbSpecsForDocs args + per-group templates; pipe-escapes Descriptions.
- `cmd/helix-refgen/main_test.go` - covers-all-verbs (authority = VerbToolNames), determinism, check round-trip.
- `internal/cli/skills/helix/reference.md` - generated per-verb reference (committed artifact).
- `internal/cli/verb.go` - VerbDoc/FlagDoc structs + VerbSpecsForDocs() + flagKindToken().
- `internal/cli/verb_test.go` - count-matches-authority, fields-mirror-catalog, read-only fresh-copy tests.
- `internal/cli/skill.go` - embed.FS switch, embeddedSkillBytes() accessor, multi-file install/uninstall walk.
- `internal/cli/skill_test.go` - repointed embeddedSkillMD refs to embeddedSkillBytes(); added TestInstallSkillWritesBundle + TestEmbeddedSkillBody.
- `internal/cli/setup_test.go` - repointed one embeddedSkillMD reference.
- `Makefile` - verify-reference drift gate + reference regen target.
- `.github/workflows/go-test.yml` - helix-refgen drift gate (REF-03) CI step.

## Decisions Made
- Args sourced from `VerbSpecsForDocs()` (backed by the already-drift-gated verbs_gen.go) rather than constructing a daemon `SerenaMCPServer` — `mcp.ToolDef` exposes no `InputSchema` (RESEARCH Pitfall 1). The accessor stays read-only with deep-copied flag slices.
- Generator output normalized to exactly one trailing newline (`strings.TrimRight(...)+"\n"`) so the bundle install's single-trailing-newline policy is assertable without double-newline drift.
- `reference.md` exempt from the 1536-char SKILL-04 idle-cost cap (it is the on-demand progressive-disclosure tier); the cap and `EmbeddedSkillBody()` stay pinned to SKILL.md only.

## Deviations from Plan

None - plan executed exactly as written. (The trailing-newline normalization in render.go was an in-scope refinement of Task 3/4's bundle contract, committed with Task 4; no behavioral deviation from the plan's intent.)

## Issues Encountered
- An initial `TestInstallSkillWritesBundle` assertion required exactly one trailing newline, but the generator emitted a section-trailing `\n\n`. Resolved by normalizing the generator output to a single trailing newline and regenerating the committed reference.md, keeping the bundle policy assertion meaningful. No scope change.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The committed `reference.md` and the `embed.FS` bundle shape now exist, so Plan 02's deterministic adoption-contract test (ADOPT-01) can read the embedded reference and assert `reference ⊇ VerbToolNames()` completeness plus the per-shape nudge golden.
- Zero new dependencies (`git diff go.mod` empty) and no wire change (`git diff api/proto/` empty) — milestone invariants intact.

## Self-Check: PASSED

- All created/modified files exist on disk (cmd/helix-refgen/*, reference.md, verb.go, skill.go).
- All four task commits present in git history (2b72daee, 17f708fb, e4c831ff, 566a752f).
- reference.md covers 50 verbs; `go run ./cmd/helix-refgen --check` clean; `go vet ./...` clean; `go test ./...` green.
- Invariants: `git diff go.mod` empty, `git diff api/proto/` empty.

---
*Phase: 97-generated-per-verb-reference-deterministic-adoption-contract*
*Completed: 2026-06-22*
