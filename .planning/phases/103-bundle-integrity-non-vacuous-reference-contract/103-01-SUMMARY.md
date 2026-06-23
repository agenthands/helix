---
phase: 103-bundle-integrity-non-vacuous-reference-contract
plan: 01
subsystem: cli-skill-bundle
tags: [security, supply-chain, embed, allowlist, anti-vacuity, tdd]
requires: []
provides:
  - "bundleFiles closed-set allowlist (install + uninstall single source of truth)"
  - "closed-set install test (RED-on-stray)"
  - "fabricated-absent-verb contract discriminator"
affects:
  - internal/cli/skill.go
  - internal/cli/skill_bundle_test.go
  - internal/cli/reference_contract_test.go
tech-stack:
  added: []
  patterns: ["filter-once-up-front allowlist over embed ReadDir", "closed-set (RED-on-stray) bundle assertion", "fabricated-absent anti-vacuity discriminator"]
key-files:
  created:
    - internal/cli/skill_bundle_test.go
    - .planning/SKILL-ISSUE.md
    - .planning/phases/103-bundle-integrity-non-vacuous-reference-contract/deferred-items.md
  modified:
    - internal/cli/skill.go
    - internal/cli/reference_contract_test.go
decisions:
  - "bundleFiles is the single closed set both installSkill and uninstallSkill consult via filterBundleEntries (one filter pass, not per-loop continue guards)"
  - "SKILL-ISSUE.md moved with PLAIN mv (it was git-untracked) then git add at .planning/ destination; no git mv"
  - "ROADMAP repath was a NO-OP — no literal old-path string existed (only bare-filename prose, left intact)"
metrics:
  duration: "~4m"
  completed: "2026-06-23"
status: complete
---

# Phase 103 Plan 01: Bundle Integrity + Non-Vacuous Reference Contract Summary

Stopped a live supply-chain embed leak by gating `installSkill`/`uninstallSkill` on a single closed-set `bundleFiles` allowlist and moving the 18 KB untracked maintainer notes out of the embed dir, while sealing the reference-completeness gate against vacuity with a fabricated-absent-verb discriminator — landed FIRST so Phases 104/105 churn a clean embed dir behind a non-vacuous contract.

## What Was Built

- **`bundleFiles` allowlist** (`internal/cli/skill.go`): package-level `map[string]bool{"SKILL.md": true, "reference.md": true}` — the EXACT closed set shipped to disk.
- **`filterBundleEntries`**: one filter-once-up-front pass over the embed `ReadDir` slice. Called immediately after `ReadDir` in `installSkill` (the filtered slice feeds the unchanged 2-pass stage→rename body) and identically in `uninstallSkill` (the removal loop only touches allowlisted files).
- **`TestInstallSkillInstallsExactlyBundle`** + **`TestUninstallSkillRemovesBundleSymmetric`** (new `skill_bundle_test.go`): closed-set install assertion (RED on any stray installed file) and uninstall-drop symmetry.
- **`TestReferenceContractDiscriminatesAbsentVerb`** (added to `reference_contract_test.go`): appends `"totally-not-a-verb"` to `VerbToolNames()` and asserts `referenceMissingVerbs` reports it — proving the gate keys on the real frozen authority, not `∅ ⊇ ∅`.
- **`SKILL-ISSUE.md` moved** out of the embed dir to `.planning/SKILL-ISSUE.md`.

## RED → GREEN Transition (anti-vacuity proof)

- **Task 1 (RED, commit 37c6138f):** `TestInstallSkillInstallsExactlyBundle` failed on the current 3-file install — installed set was `{SKILL-ISSUE.md, SKILL.md, reference.md}` ≠ `{SKILL.md, reference.md}`. Verified: `--- FAIL: TestInstallSkillInstallsExactlyBundle`. The discriminator + uninstall-symmetry tests passed immediately (confirm-and-seal anchors). `go vet` clean.
- **Task 2 (GREEN, commit 4a0684ae):** added the allowlist + filter, moved the doc out of embed → the same test passed (`ok`). The closed-set test demonstrably ran RED before the allowlist landed.

## Where the filter-once pass lives

`internal/cli/skill.go`, immediately after `entries, err := embeddedSkillFS.ReadDir("skills/helix")` in `installSkill` (and the identical call in `uninstallSkill`): `entries = filterBundleEntries(entries)`. The 2-pass `staged`/`cleanupTemps`/WriteFile-to-`.tmp`/Rename/revert body and the `withinSkillRoot`/`lexicalSkillRoot`/`containedIn` containment guards are byte-for-byte unchanged.

## SKILL-ISSUE.md move

`mv internal/cli/skills/helix/SKILL-ISSUE.md .planning/SKILL-ISSUE.md` (PLAIN mv — the file was git-UNTRACKED `??`, so `git mv` was not used), then `git add .planning/SKILL-ISSUE.md`. `ls internal/cli/skills/helix/` now lists exactly `SKILL.md` + `reference.md`. The embed glob `skills/helix/*` no longer matches the notes; the allowlist is defense-in-depth on top.

## ROADMAP repath

NO-OP. `grep -rn "internal/cli/skills/helix/SKILL-ISSUE" .planning/ROADMAP.md` returned empty — the ROADMAP carries only bare-filename prose mentions (which reference the document's findings, not its path) and were left intact, per the plan.

## Invariant gates (Task 3, commit b1322862)

| Gate | Result |
|------|--------|
| `go vet ./...` | exit 0 |
| `go run ./cmd/helix-refgen -check` | "reference.md is up to date." exit 0 — byte-reproducibility preserved |
| `go test ./test/oracle/adopt/... -count=1` | green — StripDecisionMatrix / SKILL.md anchor untouched |
| binary-clean | `strings ./helix \| grep -c "SKILL.md Decision Matrix Review"` == 0 |
| `gofmt -l` (3 touched files) | empty |
| `git diff go.mod go.sum` | empty (ZERO new deps) |
| `internal/cli` targeted suite | green (closed-set + discriminator + atomicity + containment all pass) |

refgen `-check` exit 0 and the adopt anchor confirm `reference.md` and `SKILL.md` were NOT touched.

## Deviations from Plan

None — plan executed as written. The filter retains a harmless `if e.IsDir() { continue }` in `installSkill` Pass 1 (kept to leave that block byte-stable); `filterBundleEntries` already excludes dirs, so it is a no-op guard, not a behavior change.

## Deferred Issues

- **`cmd/helix-bench/TestRunSubcommandWiresDeltaPass`** fails under full `go test ./...` (missing `ablation_deltas` / delta-pass not wired into `runBench`; HuggingFace HTTP 404). **Out of scope** — reproduced on baseline `37c6138f~1` (before any Phase 103 commit), unrelated to the `internal/cli` skill change. Logged in `deferred-items.md`. Not fixed per SCOPE BOUNDARY.

## Self-Check: PASSED

- `internal/cli/skill.go` (modified, `var bundleFiles` present), `internal/cli/skill_bundle_test.go` (created), `internal/cli/reference_contract_test.go` (modified), `.planning/SKILL-ISSUE.md` (created) — all on disk.
- Commits 37c6138f, 4a0684ae, b1322862 — all in `git log`.
