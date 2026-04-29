---
phase: 51-packaging-goreleaser
plan: 01
subsystem: infra
tags: [packaging, release-engineering, goreleaser, ci, github-actions, signing, minisign, reproducible-builds]

# Dependency graph
requires:
  - phase: 50-toolchain-go1.25-bench-local
    provides: "Go 1.25.x toolchain pin in .github/workflows/go-test.yml; Makefile self-doc convention; .gitignore line 77 'dist/' coverage"
provides:
  - ".goreleaser.yaml v2 schema for 6-arch reproducible builds with minisign signing"
  - ".github/workflows/release.yml tag-triggered three-pass workflow (snapshot/snapshot/real) with sha256 reproducibility gate"
  - "minisign.pub placeholder at repo root for one-time maintainer keypair setup"
  - "Deletion of legacy Python publish.yml so it cannot fire on release: created"
  - "Required GitHub Actions secret names: MINISIGN_PRIVATE_KEY, MINISIGN_PASSWORD"
affects: [51-02-docs-makefile, 52-packaging-distribution]

# Tech tracking
tech-stack:
  added:
    - "goreleaser ~> v2 (release pipeline orchestrator)"
    - "goreleaser/goreleaser-action@v7 (GitHub Action wrapper)"
    - "minisign 0.12 pinned (Ed25519 archive signing)"
  patterns:
    - "Three-pass reproducibility gate: two --skip=sign snapshot passes + sha256 diff + real release pass; non-reproducible build fails BEFORE publication"
    - "Secret-key write hardening: printf '%s' under umask 077 (no echo, no command-line expansion)"
    - "Pinned-tarball install over apt for reproducible CI binaries"

key-files:
  created:
    - ".goreleaser.yaml"
    - ".github/workflows/release.yml"
    - "minisign.pub"
    - ".planning/phases/51-packaging-goreleaser/deferred-items.md"
  modified: []
  deleted:
    - ".github/workflows/publish.yml"

key-decisions:
  - "Use archives.ids (the post-deprecation field) instead of archives.builds. goreleaser v2.15.3 lints archives.builds as deprecated; ids is the supported replacement and produces identical behavior."
  - "Defer the local snapshot smoke test to CI / a follow-up phase. internal/treesitter/bindings/{r,swift} use CGO build constraints that exclude all source files under CGO_ENABLED=0; the goreleaser config is correct but the broader repo cannot currently produce a static cross-compiled binary. Tracked as DEF-51-01 in deferred-items.md."

patterns-established:
  - "Three-pass reproducibility gate workflow shape (snapshot pass 1 -> snapshot pass 2 -> diff -> real release) -- the natural fit for D-03 'CI-enforced reproducibility' that survives D-06 'tag-triggered auto-publish' without a manual gate"
  - "Secret hardening recipe for GitHub Actions: printf '%s' \"$VAR\" > file under umask 077, with the secret referenced as ${{ secrets.NAME }} only on the step that needs it"
  - "Pinned-tarball install of CI tooling (here: jedisct1/minisign 0.12) instead of apt to eliminate distro-version drift as a reproducibility hazard"

requirements-completed: [PKG-01]

# Metrics
duration: 4min
completed: 2026-04-29
---

# Phase 51 Plan 01: pipeline-config-workflow Summary

**Goreleaser-driven release pipeline that turns a `v*` git tag into a signed, reproducible 6-archive GitHub Release for agenthands/helix, with a CI-enforced sha256 diff gate that fails the build BEFORE publication if two snapshot runs are not byte-identical.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-29T06:51:55Z
- **Completed:** 2026-04-29T06:56:04Z
- **Tasks:** 4
- **Files modified:** 4 (3 created, 1 deleted)

## Accomplishments

- `.goreleaser.yaml` v2 with full reproducibility flag set (`-trimpath`, `CGO_ENABLED=0`, `GOFLAGS=-mod=readonly`, `mod_timestamp: '{{ .CommitTimestamp }}'`, `ldflags -X main.date={{.CommitDate}}`) that lints clean against goreleaser 2.15.3.
- `.github/workflows/release.yml` triggered ONLY on `push: tags: ['v*']`, with `permissions: contents: write` (least privilege), three goreleaser invocations, a sha256 diff gate that emits `::error::` and exits non-zero on non-reproducibility, and secret-key handling via `printf '%s' > file` under `umask 077`.
- `minisign.pub` 2-line placeholder at the repo root, ready for the maintainer to overwrite during one-time keypair setup.
- Removal of the legacy Python `publish.yml` workflow so its `on: release: types: [created]` trigger cannot fire when goreleaser publishes the new Release (Pitfall 5 closed).

## Task Commits

Each task was committed atomically:

1. **Task 1: Delete legacy publish.yml** - `1b686e34` (chore)
2. **Task 2: Write .goreleaser.yaml** - `cc87eff0` (feat)
3. **Task 3: Write release.yml** - `de66c8cf` (feat)
4. **Task 4: Add minisign.pub placeholder** - `96e8d73c` (feat)

## Files Created/Modified

- `.goreleaser.yaml` (NEW) — goreleaser v2 config: 6-arch matrix builds, tar.gz archives with bundled LICENSE/README/INSTALL/CHANGELOG, `checksums.txt` literal name override, minisign signing block (artifacts: archive only), conventional-commit changelog grouping, `agenthands/helix` release identity, `prerelease: auto` for v*-rc*/beta*/alpha*.
- `.github/workflows/release.yml` (NEW) — tag-triggered (`push: tags: ['v*']`) three-pass workflow: snapshot pass 1 -> capture pass 1 sha256s -> snapshot pass 2 -> diff -> real release with sign+publish. Pinned minisign 0.12 install from jedisct1/minisign release tarball. Secrets used: `MINISIGN_PRIVATE_KEY` (write to `/tmp/minisign.key`), `MINISIGN_PASSWORD` (env on real release step), `GITHUB_TOKEN` (auto-injected, real release step only).
- `minisign.pub` (NEW) — 2-line placeholder file (`untrusted comment: ...PLACEHOLDER...` + `RWQ...A...`); will be overwritten by maintainer during one-time keypair setup; `minisign -V` against the placeholder fails loudly by design.
- `.github/workflows/publish.yml` (DELETED) — legacy Python `uv build` -> PyPI workflow whose `release: types: [created]` trigger would have fired on the new goreleaser-published Release. Zero cross-references confirmed.
- `.planning/phases/51-packaging-goreleaser/deferred-items.md` (NEW) — documents DEF-51-01 (CGO-gated tree-sitter bindings r + swift fail under `CGO_ENABLED=0`), tracked for resolution before the first real tag push lands.

## Decisions Made

- **Use `archives.ids: [serena]` instead of `archives.builds: [serena]`** — goreleaser v2.15.3 emits a deprecation error during `goreleaser check` for `archives.builds`. The plan dictated verbatim copy from RESEARCH Pattern 2, but the plan's own Task 2 acceptance criterion requires `goreleaser check` to exit 0 when goreleaser is available locally. `archives.ids` is the supported replacement field per goreleaser deprecation docs; behaviour is identical (selects which builds: stanza feeds into the archive). Treated as Rule 3 (blocking lint failure) deviation, applied automatically.
- **Defer end-to-end snapshot smoke test** — Plan 02 / phase gate (per VALIDATION.md) rather than this plan; the goreleaser config itself lints clean and contains every required element. The pre-existing `internal/treesitter/bindings/{r,swift}` CGO build-constraint situation is out of scope for Plan 51-01.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Renamed `archives.builds` -> `archives.ids` in `.goreleaser.yaml`**
- **Found during:** Task 2 (`.goreleaser.yaml` -- local `goreleaser check`)
- **Issue:** goreleaser v2.15.3 (current Homebrew release) errors during `goreleaser check` with "DEPRECATED: archives.builds should not be used anymore". Task 2's acceptance criterion explicitly requires `goreleaser check` to exit 0 when goreleaser is available locally; the plan's Task 2 verbatim YAML used `builds: [serena]` inside the `archives:` block.
- **Fix:** Renamed the single line `builds: [serena]` to `ids: [serena]` inside the `archives:` block. Behavior is identical (it selects which `builds:` stanza feeds into each archive); the field rename is documented in goreleaser's deprecations page. All other content of `.goreleaser.yaml` is verbatim from RESEARCH Pattern 1-5.
- **Files modified:** `.goreleaser.yaml` (1 line changed before commit)
- **Verification:** `goreleaser check` exits 0 with "1 configuration file(s) validated" + "thanks for using GoReleaser!"
- **Committed in:** `cc87eff0` (Task 2 commit; the deviation was applied before the commit, so the deviation does not have its own hash).

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** No scope creep. The deviation was a single field rename to satisfy a verify-gate the plan itself imposed. Functional behavior of the goreleaser config is unchanged.

## Issues Encountered

- **Local snapshot smoke test failed on CGO build constraints** — When attempting to run `goreleaser release --snapshot --clean --skip=sign` for behavior validation, the build matrix failed at the first target with `build constraints exclude all Go files in internal/treesitter/bindings/{r,swift}`. These bindings have CGO-gated build constraints; goreleaser sets `CGO_ENABLED=0` per Pattern 1 (project's "single static binary, no runtime deps" invariant per CLAUDE.md). The packages have no non-CGO fallback. **Resolution:** This is pre-existing repo state, OUT of scope for Plan 51-01 (`<files_modified>` does not include any Go sources or treesitter bindings). Documented as **DEF-51-01** in `.planning/phases/51-packaging-goreleaser/deferred-items.md` with three resolution paths and a HIGH-severity flag — the first real `v*` tag push WILL fail in CI until this is resolved. The goreleaser config itself is correct (lint pass) and the workflow is correct (YAML lint pass).

## User Setup Required

**External services require manual configuration before the first real tag push.** Two GitHub Actions secrets must exist BEFORE pushing a `v*` tag, otherwise `release.yml` fails at the "Real release (sign + publish)" step:

- `MINISIGN_PRIVATE_KEY` — contents of the `minisign.key` file produced by `minisign -G -p minisign.pub -s minisign.key` on the maintainer's local machine. Upload via `gh secret set MINISIGN_PRIVATE_KEY < minisign.key` (or paste into Settings -> Secrets -> Actions in the GitHub web UI).
- `MINISIGN_PASSWORD` — the password chosen during `minisign -G` keypair generation. Upload via `gh secret set MINISIGN_PASSWORD` (interactive prompt) or paste into the web UI.

The placeholder `minisign.pub` checked in by Task 4 must be overwritten in the same maintainer-local step (commit the real public key in a normal commit). Plan 02 will add the full procedure to a CONTRIBUTING.md "Releasing" subsection.

## Next Phase Readiness

**Plan 02 inputs (preserve verbatim):**
- Secret names referenced in `release.yml`: `MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD`. CONTRIBUTING.md "Releasing" subsection in Plan 02 MUST reference these names exactly.
- Release URL pattern: `https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz` (and `.minisig` sibling). INSTALL.md verification block in Plan 02 MUST use `agenthands/helix` URLs.
- Checksum file name: `checksums.txt` (literal -- override applied in `.goreleaser.yaml` `checksum.name_template`). INSTALL.md `sha256sum -c` step in Plan 02 MUST use this exact name (NOT `serena_v1.9.0_checksums.txt`).
- Public key URL for INSTALL.md: `https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub` (per D-02).
- Maintainer keypair setup command: `minisign -G -p minisign.pub -s minisign.key` followed by `gh secret set MINISIGN_PRIVATE_KEY < minisign.key && gh secret set MINISIGN_PASSWORD`. Plan 02 documents this in CONTRIBUTING.md.

**Phase blockers / concerns:**
- **DEF-51-01 (HIGH):** CGO build constraints on `internal/treesitter/bindings/{r,swift}` block any successful `CGO_ENABLED=0` cross-compile. Plan 51-01's pipeline is correct, but the first CI run on a real tag will fail at the build matrix until this is resolved (likely a Phase 52+ task to make those bindings CGO-optional via build tags). The goreleaser config and workflow themselves require no further changes.
- **`go.mod` module path stays `github.com/postfix/serena`** (Open Question 1, RESOLVED -- NO rename in Phase 51). Goreleaser does not depend on the module path; it depends only on the GitHub repo identity (`release.github.owner/name`), which IS `agenthands/helix`. INSTALL.md edits in Plan 02 must respect this and either delete or annotate the demoted `go install github.com/postfix/serena/...` line.

## Self-Check: PASSED

- `.goreleaser.yaml` exists at repo root: FOUND
- `.github/workflows/release.yml` exists: FOUND
- `minisign.pub` exists at repo root: FOUND
- `.github/workflows/publish.yml` does NOT exist: CONFIRMED ABSENT
- `.planning/phases/51-packaging-goreleaser/deferred-items.md` exists: FOUND
- Commit `1b686e34` (Task 1, publish.yml deletion): FOUND in git log
- Commit `cc87eff0` (Task 2, .goreleaser.yaml): FOUND in git log
- Commit `de66c8cf` (Task 3, release.yml): FOUND in git log
- Commit `96e8d73c` (Task 4, minisign.pub): FOUND in git log

---
*Phase: 51-packaging-goreleaser*
*Plan: 01-pipeline-config-workflow*
*Completed: 2026-04-29*
