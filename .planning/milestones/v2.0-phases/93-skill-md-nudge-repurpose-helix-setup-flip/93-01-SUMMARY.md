---
phase: 93-skill-md-nudge-repurpose-helix-setup-flip
plan: 01
subsystem: cli
tags: [skill, go-embed, setup, claude-code, drift-gate, security]
requires:
  - internal/cli/verb.go (VerbToolNames — drift-gate seam)
  - internal/cli/verbs_gen.go (frozen Phase 92 verb catalog)
provides:
  - embeddedSkillMD (package cli) — the //go:embed'd SKILL.md body
  - installSkill(targetDir string) error — atomic contained skill writer
  - skillTargetDir(claudeDir string) string — <claudeDir>/skills/helix resolver
  - internal/cli/skills/helix/SKILL.md — the embedded skill asset
affects:
  - 93-03 (helix setup wires installSkill/skillTargetDir into each registrar)
  - 93-04 (behavioral oracle consumes embeddedSkillMD; fills SKILL-04 token numbers)
tech-stack:
  added: []
  patterns:
    - "go:embed single markdown file as string var (analog: internal/eval/judge/client.go)"
    - "atomic temp-write+rename disk write (analog: internal/cli/nudge.go saveSessionStats)"
    - "path-traversal containment before write (analog: internal/cli/render.go readSnippetLine)"
    - "in-default-suite drift gate against a generated catalog (analog: verbs_gen_test.go)"
key-files:
  created:
    - internal/cli/skills/helix/SKILL.md
    - internal/cli/skill.go
    - internal/cli/skill_test.go
  modified: []
decisions:
  - "allowed-tools emitted as `Bash(helix *)` (docs-confirmed scoped-Bash form); the REQUIREMENTS SKILL-01 colon form `Bash(helix:*)` recorded in an HTML comment for traceability"
  - "containment expressed as a `skills/helix` root-segment check (Join collapses crafted `..` away from the root) rather than a raw `..`-scan, since filepath.Join pre-cleans the target"
  - "description scoped to symbol-level code nav/edit with an explicit dormant clause for prose/log/config (Pitfall 1)"
metrics:
  duration: ~50m (incl. environmental module-cache repair)
  completed: 2026-06-21
  tasks: 2
  files: 3
status: complete
---

# Phase 93 Plan 01: SKILL.md + go:embed + installSkill Summary

Authored the embedded Claude Code `SKILL.md` skill (frontmatter + a `| Question | Use this | Not this |` decision table citing all 50 frozen Phase 92 `helix` kebab verbs and the terse `relpath:line:col<TAB>payload` output), embedded it as a `string` via `//go:embed`, and added the `installSkill`/`skillTargetDir` atomic, idempotent, traversal-contained disk writer that 93-03 consumes — all with zero new dependencies and zero proto change.

## What shipped

- **`internal/cli/skills/helix/SKILL.md`** — frontmatter (`name: helix`, a 595-byte `description` scoped to symbol-level code navigation/refactor with an explicit dormant clause for prose/log/config, `allowed-tools: Bash(helix *)`); a decision table mapping each code question → a `helix <verb>` → the grep/sed/cat it replaces, citing verbs by capability group (navigation/edit/fileops/diagnostics/repomap/memory); a terse-output note; and a reserved `<!-- Token note (SKILL-04) ... -->` line for 93-04.
- **`internal/cli/skill.go`** — `//go:embed skills/helix/SKILL.md → var embeddedSkillMD string`; `skillTargetDir(claudeDir) → <claudeDir>/skills/helix`; `installSkill(targetDir)` (MkdirAll 0755 + temp-write 0644 + os.Rename, single-trailing-newline policy, `skills/helix` root containment).
- **`internal/cli/skill_test.go`** — 12 tests, all in the **default untagged suite**: embed-non-empty, frontmatter-valid, description ≤1536 (SKILL-04 idle-cost bound, no API key), **verb-membership drift gate** (every cited `helix <kebab>` maps kebab→snake into `cli.VerbToolNames()`), no-verb-count-literal, token-note presence, plus installSkill content-faithful/nested-dir/idempotent/no-tmp-leftover/containment.

## Verification

- `go test ./internal/cli/... -count=1` — green (12 skill tests + existing).
- `go build ./...` — succeeds (the `//go:embed` resolves the asset).
- `go vet ./internal/cli/` — clean.
- `gofmt -l` on the new files — clean.
- `git diff go.mod` empty; `git diff go.sum` empty; `git diff api/proto/` empty (carried invariants hold).
- 50 unique `helix <verb>` citations, all members of the live catalog; description 595/1536 bytes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Repaired a corrupt local Go module cache (environmental, not code)**
- **Found during:** Task 1 RED run — `go build`/`go test` failed at HEAD (baseline) with `no required module provides package google.golang.org/protobuf/...` and `... does not contain package ...`.
- **Root cause:** Several extracted modules in `$GOPATH/pkg/mod` (`google.golang.org/protobuf@v1.36.11`, `github.com/google/jsonschema-go@v0.4.2`, and 7 others reported by `go mod verify` as "dir has been modified") were extracted with **zero `.go` files** while their cached zips were intact. A stale build cache compounded it.
- **Fix:** Re-extracted the affected modules from their intact cached zips into the module cache (read-only) and ran `go clean -cache`. `go mod verify` then reported "all modules verified" and the build/tests passed. **No `go.mod`/`go.sum` edits** — an intermediate `-mod=mod` build had promoted `bleve`/`uuid` to direct and bumped `jsonschema-go`; those changes were reverted with `git checkout -- go.mod go.sum` to preserve the zero-dep invariant.
- **Files modified:** none in-repo (module cache only).
- **Commit:** n/a (environment repair, not a code change).

## Containment design note

The plan's Test 5 passes `filepath.Join(root, "skills", "helix", "..", "..", "..", "escaped")` to `installSkill`. `filepath.Join` pre-cleans, collapsing the `..` to `<parent-of-root>/escaped` — so a raw `..`-element scan cannot detect the escape. The implementation instead requires the cleaned target to carry the consecutive `skills/helix` segment sequence (the invariant every `skillTargetDir` output holds); a traversal-crafted target collapses away from that root and is refused before any write, while a legitimate nested target (`skills/helix/<sub>`) still passes. This is the functional analog of `readSnippetLine`'s `filepath.Rel` + `..`-prefix guard, adapted to a single-argument signature.

## SKILL-04 status

The idle-cost upper bound is asserted dependency-free (description ≤1536 bytes; the body loads only on trigger). The exact idle-skill-cost and preloaded-full-MCP-schema token numbers are reserved by the `<!-- Token note (SKILL-04) ... -->` line and filled by plan 93-04 (behavioral oracle), per the plan.

## Self-Check: PASSED

- internal/cli/skills/helix/SKILL.md — FOUND
- internal/cli/skill.go — FOUND
- internal/cli/skill_test.go — FOUND
- Commit b7fb60a8 (test RED Task1) — FOUND
- Commit 1e48cb09 (feat GREEN Task1) — FOUND
- Commit 0895577f (test RED Task2) — FOUND
- Commit 1cabaaed (feat GREEN Task2) — FOUND
