---
phase: 51-packaging-goreleaser
plan: 05
subsystem: packaging
tags: [packaging, signing, checksums, gap-closure, wr-02, wr-05, in-01]
requires: [51-01, 51-02]
provides:
  - "checksums.txt is signed by minisign (.goreleaser.yaml signs.artifacts: all)"
  - "INSTALL.md verify block fails loud on typo'd archive name (strict grep-OK-or-exit-1)"
  - "INSTALL.md verifies checksums.txt.minisig BEFORE trusting any sha256 in it"
  - "make release-snapshot prints actionable hint when goreleaser is missing"
affects: [.goreleaser.yaml, INSTALL.md, Makefile]
tech-stack:
  added: []
  patterns:
    - "goreleaser signs.artifacts: all -- signs every produced artifact (archives + checksums.txt)"
    - "INSTALL.md verify chain: minisign(checksums.txt) -> sha256sum-grep(archive) -> minisign(archive)"
    - "Makefile recipe guard: command -v <tool> >/dev/null 2>&1 || { echo HINT; exit 1; }"
key-files:
  created: []
  modified:
    - .goreleaser.yaml
    - INSTALL.md
    - Makefile
decisions:
  - "Use signs.artifacts: all (not 'checksum' or per-block dual configs) -- one signs entry signs both archives and checksums.txt; matches REVIEW WR-02 fix recommendation."
  - "Place minisign-on-checksums.txt step BEFORE sha256sum -- trust order is pubkey -> checksums.txt sig -> sha256s -> archive bytes; fail-closed if user skips a step."
  - "Strict sha256sum form is grep-OK-or-exit-1 (not plain -c which spams 'missing' warnings on the 5 archives the user did not download)."
  - "Makefile guard error message names BOTH CONTRIBUTING.md (full setup) AND brew install goreleaser (one-shot macOS install) per IN-01 fix."
metrics:
  duration: ~12 minutes
  completed: 2026-04-29
  tasks_completed: 3
  files_modified: 3
---

# Phase 51 Plan 05: Checksums Signing & Strict Verify Summary

Strengthened the supply-chain integrity story by signing `checksums.txt` itself with minisign, replacing the `sha256sum --ignore-missing` fail-open with a strict `grep "...: OK"` form, and adding a `command -v goreleaser` guard to the `make release-snapshot` recipe so missing-tooling errors are actionable instead of cryptic.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Change `.goreleaser.yaml` `signs.artifacts: archive` -> `all` | `f8dedbc3` | `.goreleaser.yaml` |
| 2 | Strengthen `INSTALL.md` verify block (drop `--ignore-missing`, verify `checksums.txt.minisig`, fail loud on typos) | `1b14ca44` | `INSTALL.md` |
| 3 | Add `command -v goreleaser` guard to `Makefile` `release-snapshot` recipe | `e0e1496e` | `Makefile` |

## Verification Results

- `goreleaser check .goreleaser.yaml` -> **exits 0** (`all` is a valid v2 schema value; goreleaser was installed locally so this was checked, not deferred to CI).
- `make -n release-snapshot` -> **exits 0**; prints both the guard line and the `goreleaser release` line as expected.
- `git status --short` -> **clean** after all 3 commits; no untracked files, no unintended modifications.
- `go vet ./...` -> **exits 0** (only the pre-existing `swift` tree-sitter macro-redefine warning, unrelated to this plan).
- INSTALL.md has exactly **2** occurrences of `minisign -V -p minisign.pub` (one for `checksums.txt`, one for the archive) -- the new chain is wired correctly.
- INSTALL.md no longer contains `--ignore-missing`; the new strict `grep "...: OK" || { echo "checksum FAILED"; exit 1; }` form is in place.
- `.goreleaser.yaml` has exactly one `^signs:$` block and one `id: minisign` line (single signing config; the edit was a single-character-class swap on one line).

### Mental UAT for the strict sha256sum form

If a user typos `OS=darwn` (missing the `i`), the new strict form behaves correctly:

1. `sha256sum -c checksums.txt` runs against the real checksums.txt.
2. The user has downloaded `serena_v1.9.0_darwn_amd64.tar.gz` (which fails to download because the URL is 404'd, OR succeeds locally if they crafted a mismatched name).
3. The pipeline `grep "serena_v1.9.0_darwn_amd64.tar.gz: OK"` finds **no matching line** because the real `checksums.txt` only lists `darwin`, not `darwn`.
4. `grep` exits non-zero -> `||` triggers `echo "checksum FAILED"; exit 1`.
5. User sees the loud failure instead of a misleading silent OK.

Contrast with the old `sha256sum -c --ignore-missing checksums.txt`: that form ignored the missing `darwn` archive entirely, exited 0 with no error, and the user happily continued with a misnamed (or missing) archive.

## VERIFICATION Gaps Closed

| Finding | What this plan changed | How it closes the gap |
|---------|------------------------|------------------------|
| **WR-02** (checksums.txt unsigned) | `signs.artifacts: archive` -> `all` in `.goreleaser.yaml`; INSTALL.md verifies `checksums.txt.minisig` before trusting any sha256 | An attacker who swaps both an archive AND `checksums.txt` now also has to forge a minisign signature on the substituted `checksums.txt` -- impossible without the project private key. |
| **WR-05** (INSTALL.md `--ignore-missing` fail-open) | `sha256sum -c --ignore-missing checksums.txt` -> `sha256sum -c checksums.txt 2>&1 \| grep "<archive>: OK" \|\| { echo "checksum FAILED"; exit 1; }` | A user who fat-fingers `OS` or `ARCH` sees `checksum FAILED` and a non-zero exit instead of a misleading silent success. |
| **IN-01** (cryptic make error when goreleaser missing) | `make release-snapshot` recipe now starts with `@command -v goreleaser >/dev/null 2>&1 \|\| { echo "...CONTRIBUTING.md (Releasing). brew install goreleaser"; exit 1; }` | Contributors without goreleaser see a one-line install hint pointing at both `CONTRIBUTING.md` (full setup) and `brew install goreleaser` (macOS one-shot), instead of `make: goreleaser: command not found`. |

## Threat Mitigations

- **T-51-25 (T-checksums-substitution, Tampering)** -- mitigated by Task 1 (signs.artifacts: all) + Task 2 (INSTALL.md verifies checksums.txt.minisig before sha256sum). An attacker swapping both archive and checksums.txt must now forge a minisign signature on the new checksums.txt -- not feasible.
- **T-51-26 (T-fat-finger-checksum-bypass, Tampering/Confusion)** -- mitigated by Task 2 (strict grep form replaces --ignore-missing). User typos surface as `checksum FAILED` with exit 1.
- **T-51-27 (T-doc-theater-checksum, Confusion)** -- mitigated by Task 2 (each verification step has independent integrity: pubkey -> checksums.txt sig -> sha256 -> archive sig). Skipping any one step still leaves a real integrity claim from the others.
- **T-51-28 (T-cryptic-make-error, Confusion)** -- mitigated by Task 3 (presence guard + actionable hint).

## Deviations from Plan

None -- plan executed exactly as written. All three edits were surgical (one-line goreleaser change, verify-block replacement in INSTALL.md, two-line Makefile guard insert) and the verbatim text from the plan's `<action>` blocks landed byte-for-byte.

## Auth Gates

None.

## Cross-Plan Note for 51-06

Plan 51-05 owns `.goreleaser.yaml`, `INSTALL.md`, and `Makefile` only. The other half of VERIFICATION Concern B -- the CI pre-flight in `release.yml` that grep-fails on the PLACEHOLDER `minisign.pub` so a release cannot accidentally publish with the unsigned placeholder pubkey -- lives in Plan 51-06 (which owns `release.yml` this wave; 51-07 also touches it). Together, 51-05 + 51-06 close all of Concern B except the maintainer's offline keypair generation + secret upload (already documented as a `user_setup` action in 51-01).

## Self-Check: PASSED

- `.goreleaser.yaml` modified: FOUND (commit `f8dedbc3`)
- `INSTALL.md` modified: FOUND (commit `1b14ca44`)
- `Makefile` modified: FOUND (commit `e0e1496e`)
- All three commits present in `git log --oneline -4`: FOUND
- `goreleaser check` exits 0: VERIFIED
- `make -n release-snapshot` exits 0: VERIFIED
- INSTALL.md `minisign -V -p minisign.pub` occurrences == 2: VERIFIED
- No `--ignore-missing` in INSTALL.md: VERIFIED
- `git status --short` clean: VERIFIED
