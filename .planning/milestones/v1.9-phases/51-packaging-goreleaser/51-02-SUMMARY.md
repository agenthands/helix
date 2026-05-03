---
phase: 51-packaging-goreleaser
plan: 02
subsystem: docs
tags: [packaging, documentation, makefile, contributing, install-guide, release-engineering]

# Dependency graph
requires:
  - phase: 51-packaging-goreleaser
    plan: 01
    provides: ".goreleaser.yaml archive name template (serena_{version}_{os}_{arch}.tar.gz), checksums.txt literal name override, MINISIGN_PRIVATE_KEY + MINISIGN_PASSWORD secret names referenced from release.yml, minisign.pub at repo root"
provides:
  - "INSTALL.md '## Install (pre-built binary)' lead H2 with single copy-paste D-04 verify ceremony using agenthands/helix URLs and Plan 01 filenames"
  - "README.md Quick Start with corrected agenthands/helix git clone URL and clarifying in-fence comment about transitional postfix/serena module path"
  - "Makefile release-snapshot target (Phase 50 D-07 self-doc style) for local goreleaser dry-run via 'goreleaser release --snapshot --clean --skip=sign'"
  - "CONTRIBUTING.md '## Releasing' H2 covering tag-cut workflow, repo secret names, one-time keypair setup, key rotation, and Key details trailer"
affects: [52-packaging-distribution]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "INSTALL.md restructure pattern: pre-built lead with single copy-paste verification block (D-04), build-from-source demoted (D-05)"
    - "Makefile self-doc target shape inherited verbatim from Phase 50 D-07 bench-baseline analog (single fixed-path output, no NAME= parameter, single command line)"
    - "CONTRIBUTING.md ceremony H2 shape inherited verbatim from Running Benchmarks analog (sh fences, intro paragraph naming constraints upfront, Key details: trailing bullet list)"

key-files:
  created: []
  modified:
    - "INSTALL.md"
    - "README.md"
    - "Makefile"
    - "CONTRIBUTING.md"
  deleted: []

key-decisions:
  - "Removed legacy 'go install github.com/postfix/serena/cmd/serena@latest' line from INSTALL.md entirely (not flagged with TODO) per RESEARCH Open Question 1 recommendation -- avoids advertising a path that will 404 post-go.mod-rename, while preserving working command in README.md with explanatory comment."
  - "CONTRIBUTING.md Releasing section uses 3 sh fences (dry-run, tag-push, keypair-setup) plus prose-only Key rotation subsection. Plan implementation note suggested 4 fences but Key rotation is naturally a prose explanation referencing the verification URL; 0 bash fences confirms house-style match."
  - "make release-snapshot was NOT exercised locally because of pre-existing DEF-51-01 (CGO build constraints in internal/treesitter/bindings/{r,swift} block CGO_ENABLED=0 cross-compile). 'make -n release-snapshot' dry-run prints the expected command and exits 0; full execution is deferred to phase gate UAT per VALIDATION.md."

requirements-completed: [PKG-01]

# Metrics
duration: 2min13s
completed: 2026-04-29
---

# Phase 51 Plan 02: docs-makefile Summary

**User-facing and contributor-facing documentation that turns Plan 01's bare release pipeline into a usable product story: INSTALL.md leads with a single copy-paste verify ceremony for pre-built binaries (closes ROADMAP success criterion 3), README.md / INSTALL.md repo identity drift is fixed, contributors get a `make release-snapshot` dry-run target, and CONTRIBUTING.md documents the maintainer release ceremony (secret setup, tag-cut, key rotation).**

## Performance

- **Duration:** 2 min 13 s
- **Started:** 2026-04-29T07:01:45Z
- **Tasks:** 4
- **Files modified:** 4 (0 created, 4 modified, 0 deleted)

## Accomplishments

- `INSTALL.md` restructured per D-04 + D-05: new `## Install (pre-built binary)` H2 leads with a single fenced bash block (download archive + .minisig + checksums.txt + minisign.pub, sha256sum verify, minisign -V verify, tar -xzf, ./serena --help). All URLs use `agenthands/helix`. macOS one-line note covers `shasum -a 256 -c` + `brew install minisign` substitutions. Former Prerequisites content demoted to `## Build from source` with corrected `agenthands/helix.git` clone URL. Lines from `## Quick Start` through end of file are byte-identical.
- `README.md` Quick Start install snippet fixed: `git clone https://github.com/postfix/serena.git` -> `https://github.com/agenthands/helix.git`, `cd serena` -> `cd helix`. The `go install github.com/postfix/serena/cmd/serena@latest` line preserved unchanged with new in-fence comment explaining the transitional module-path / repo-path split (RESEARCH Open Question 1).
- `Makefile` gained `release-snapshot` target on `.PHONY` line 1 with the recipe `goreleaser release --snapshot --clean --skip=sign`. Single-line `## help` annotation names the fixed output path (`dist/`) and gitignored status, mirroring `bench-baseline` cadence verbatim. `make -n release-snapshot` dry-run prints the expected command and exits 0.
- `CONTRIBUTING.md` gained `## Releasing` H2 between `## Running Benchmarks` and `## Adding a New MCP Tool`, with H3 subsections (Repository secrets, One-time keypair setup, Key rotation), `make release-snapshot` reference, exact secret names matching Plan 01's release.yml (`MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD`), the canonical `raw.githubusercontent.com/agenthands/helix/main/minisign.pub` URL, and a Key details trailing bullet list. House style mirrors Running Benchmarks: sh fences (3), no bash fences, plain prose, no emoji, no admonition syntax.

## Task Commits

Each task was committed atomically (worktree mode -- per-task no-verify commits):

1. **Task 1: Add release-snapshot Makefile target** - `770b5db4` (feat)
2. **Task 2: Restructure INSTALL.md per D-04 + D-05** - `1a11adb6` (docs)
3. **Task 3: Fix README.md repo identity drift on lines 65, 71** - `77212a7c` (docs)
4. **Task 4: Add ## Releasing subsection to CONTRIBUTING.md** - `752cba6a` (docs)

## Files Created/Modified

- `INSTALL.md` (MODIFIED) -- top ~29 lines restructured into `## Install (pre-built binary)` lead H2 + `## Build from source` demoted H2; surgical edit, lines 31+ (Quick Start onward) byte-identical to original. Net delta: +32 / -8 lines.
- `README.md` (MODIFIED) -- 4 surgical line changes inside the Quick Start fenced block: clone URL fixed, `cd serena`->`cd helix`, in-fence comment added explaining go.mod transitional state. Net delta: +3 / -2 lines.
- `Makefile` (MODIFIED) -- 2 edits: append `release-snapshot` to .PHONY (line 1), append new target block after `bench-baseline`. Net delta: +4 / -1 lines.
- `CONTRIBUTING.md` (MODIFIED) -- single block insertion: 57 new lines inserted between `## Running Benchmarks` (ends at the Honor system bullet) and `## Adding a New MCP Tool` (heading unchanged, everything below it byte-identical). Net delta: +57 / -0 lines.

## Cross-reference check (Plan 01 contracts honored verbatim)

Confirmed string-for-string match between Plan 02 references and Plan 01 pipeline output:

| Plan 02 reference | Plan 01 source | Match |
|-|-|-|
| `MINISIGN_PRIVATE_KEY` (CONTRIBUTING.md) | `release.yml` env var on real release step | exact |
| `MINISIGN_PASSWORD` (CONTRIBUTING.md) | `release.yml` env var on real release step | exact |
| `serena_${VERSION}_${OS}_${ARCH}.tar.gz` (INSTALL.md) | `.goreleaser.yaml` archives.name_template | exact |
| `checksums.txt` (INSTALL.md) | `.goreleaser.yaml` checksum.name_template literal override | exact |
| `agenthands/helix` (INSTALL.md, README.md, CONTRIBUTING.md) | `.goreleaser.yaml` release.github.owner/name | exact |
| `https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub` (INSTALL.md, CONTRIBUTING.md) | `minisign.pub` placeholder at repo root | exact |
| `goreleaser release --snapshot --clean --skip=sign` (Makefile, CONTRIBUTING.md) | RESEARCH Pattern 6 | exact |

## Decisions Made

- **Omit legacy `go install` line from INSTALL.md entirely** -- per RESEARCH Q1 recommendation: do not advertise a path that will break post-go.mod-rename. INSTALL.md is now the single canonical install guide pointing exclusively at agenthands/helix. README.md retains the working `go install github.com/postfix/serena/cmd/serena@latest` command with an in-fence comment explaining the transitional state, so users who already have that command working still see it (and the comment makes the postfix/agenthands relationship transparent).
- **CONTRIBUTING.md Releasing section uses 3 sh fences, not 4** -- the plan implementation suggested key rotation might warrant its own fence, but the natural shape is prose: rotation is "repeat the one-time setup, commit the new key, update both secrets" plus a referenced URL. Adding a code fence would either duplicate the One-time keypair setup commands or add a fence around a single URL, both unnatural.
- **`make release-snapshot` was NOT exercised end-to-end locally** -- DEF-51-01 (CGO build constraints in `internal/treesitter/bindings/{r,swift}`) prevents `CGO_ENABLED=0` cross-compile from succeeding. `make -n release-snapshot` dry-run validates the target shape; full goreleaser run is deferred to the phase gate UAT (per VALIDATION.md sampling rate). Plan 01's deferred-items.md tracks DEF-51-01.

## Deviations from Plan

None. All four tasks executed exactly as written.

## Issues Encountered

- **DEF-51-01 reaffirmed (carried from Plan 01):** Local execution of `make release-snapshot` cannot be smoke-tested end-to-end until the CGO build constraints in `internal/treesitter/bindings/{r,swift}` are resolved. The Makefile target itself is correct (recipe matches RESEARCH Pattern 6 verbatim; `make -n` dry-run succeeds). This is an existing repo-state issue documented in `.planning/phases/51-packaging-goreleaser/deferred-items.md` (HIGH severity); Plan 02 does not introduce or worsen it. Recommended phase 52+ task: add `//go:build !nocgo` build tags with stub fallbacks so `CGO_ENABLED=0` cross-compile succeeds.

## User Setup Required

**Outstanding maintainer prerequisite (carried from Plan 01):** Before pushing the first real `v*` tag, the maintainer must complete the one-time keypair setup now documented in `CONTRIBUTING.md` "## Releasing > One-time keypair setup":

```sh
minisign -G -p minisign.pub -s minisign.key
gh secret set MINISIGN_PRIVATE_KEY < minisign.key
gh secret set MINISIGN_PASSWORD
git add minisign.pub
git commit -m "feat(release): commit minisign public key"
git push
```

The placeholder `minisign.pub` committed in Plan 01 Task 4 must be overwritten in this same step. After upload, delete the local `minisign.key`; store the password in a password manager.

CONTRIBUTING.md `## Releasing > One-time keypair setup` is the canonical reference and matches Plan 01's release.yml secret names string-for-string.

## Code unaffected (CLAUDE.md compliance)

This plan touched **zero** Go source files. `go vet ./...` and `go test ./...` baseline status is preserved. The four files modified are documentation (INSTALL.md, README.md, CONTRIBUTING.md) and build-tooling configuration (Makefile). No changes to `cmd/serena/`, `internal/`, `protocol/`, or `api/proto/`.

## Next Phase Readiness

**Phase gate UAT (deferred to `/gsd-verify-work` per VALIDATION.md):** Cut a throwaway `v0.0.0-rc-test` tag on a test branch, observe `release.yml` fires, the reproducibility gate passes (DEF-51-01 must be resolved first or the build matrix will fail), 6 archives + 6 .minisig + checksums.txt publish to GitHub Releases, and a fresh shell can copy-paste the `INSTALL.md` verify block end-to-end on darwin and linux. This UAT closes ROADMAP success criterion 3 ("A user following INSTALL.md can verify a downloaded binary's signature and checksum in one terminal session").

**Phase 52 inputs (PKG-02/03/04 -- Homebrew tap, Scoop bucket, native Linux packages):** The goreleaser config Plan 01 wrote already targets agenthands/helix and uses serena_{version}_{os}_{arch} archive naming; Phase 52 can add `brews:` and `scoops:` sections additively without rewriting Plan 01's config. CONTRIBUTING.md `## Releasing` documents only the binary-publish ceremony; Phase 52 will extend it with package-manager-specific flows.

## Self-Check: PASSED

- `Makefile` contains `release-snapshot` on `.PHONY` line 1: FOUND
- `Makefile` contains `release-snapshot:` target with `goreleaser release --snapshot --clean --skip=sign` recipe: FOUND
- `INSTALL.md` contains `## Install (pre-built binary)` H2: FOUND
- `INSTALL.md` contains `## Build from source` H2 BEFORE `## Quick Start`: FOUND
- `INSTALL.md` contains `agenthands/helix/releases/download` URL: FOUND
- `INSTALL.md` contains `raw.githubusercontent.com/agenthands/helix/main/minisign.pub`: FOUND
- `INSTALL.md` contains `minisign -V -p minisign.pub`: FOUND
- `INSTALL.md` does NOT contain `git clone https://github.com/postfix/serena.git`: CONFIRMED ABSENT
- `INSTALL.md` does NOT contain `go install github.com/postfix/serena/cmd/serena@latest`: CONFIRMED ABSENT
- `README.md` contains `git clone https://github.com/agenthands/helix.git`: FOUND
- `README.md` does NOT contain `git clone https://github.com/postfix/serena.git`: CONFIRMED ABSENT
- `README.md` contains `cd helix`: FOUND
- `README.md` contains in-fence comment about module path: FOUND
- `README.md` retains `go install github.com/postfix/serena/cmd/serena@latest`: FOUND
- `CONTRIBUTING.md` contains `## Releasing` H2: FOUND
- `CONTRIBUTING.md` contains `### Repository secrets`, `### One-time keypair setup`, `### Key rotation` H3 subsections: FOUND
- `CONTRIBUTING.md` contains `MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD`: FOUND
- `CONTRIBUTING.md` contains `make release-snapshot`: FOUND
- `CONTRIBUTING.md` contains `gh secret set MINISIGN_PRIVATE_KEY < minisign.key`: FOUND
- `CONTRIBUTING.md` Releasing section uses sh fences only (3 sh, 0 bash): VERIFIED
- Commit `770b5db4` (Task 1, Makefile): FOUND in git log
- Commit `1a11adb6` (Task 2, INSTALL.md): FOUND in git log
- Commit `77212a7c` (Task 3, README.md): FOUND in git log
- Commit `752cba6a` (Task 4, CONTRIBUTING.md): FOUND in git log

---
*Phase: 51-packaging-goreleaser*
*Plan: 02-docs-makefile*
*Completed: 2026-04-29*
