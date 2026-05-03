---
phase: 51-packaging-goreleaser
fixed_at: 2026-04-29T00:00:00Z
review_path: .planning/phases/51-packaging-goreleaser/51-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 7
skipped: 1
status: partial
---

# Phase 51: Code Review Fix Report

**Fixed at:** 2026-04-29
**Source review:** .planning/phases/51-packaging-goreleaser/51-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 8 (3 BLOCKER + 5 WARNING)
- Fixed: 7
- Skipped: 1 (CR-01, deferred to DEF-51-02 -- maintainer architectural decision)

## Fixed Issues

### CR-02: INSTALL.md verification recipe produces 404s -- archive name template strips `v`

**Files modified:** `.goreleaser.yaml`, `CONTRIBUTING.md`
**Commit:** 93c0a7b6
**Applied fix:** Chose option (a) from the review. Changed `name_template` from `serena_{{ .Version }}_...` to `serena_v{{ .Version }}_...` so the published archive carries the `v` prefix the INSTALL.md recipe (`VERSION=v1.9.0`) constructs URLs with. Updated the example archive name in `CONTRIBUTING.md` to match (`serena_v<version>_<os>_<arch>.tar.gz`). INSTALL.md itself needed no change because the existing recipe already used `${VERSION}` with the `v` prefix.

### CR-03: `minisign.pub` is a literal PLACEHOLDER

**Files modified:** `INSTALL.md`
**Commit:** 2ae35def
**Applied fix:** Operational blocker (cannot generate the real keypair from review). Applied the reviewer's documented fallback: added a top-of-document pre-release notice to INSTALL.md telling users no signed releases exist yet, that the verification recipe will not succeed until the maintainer rotates in the real public key, and to prefer "Build from source" until DEF-51-03 closes. The release.yml pre-flight already fails closed on the placeholder, so CI is safe; this fix protects users who copy-paste the recipe today against caching the placeholder as their root of trust.

### WR-01: `/tmp/minisign.key` is on disk during snapshot passes that don't need it

**Files modified:** `.github/workflows/release.yml`
**Commit:** 01e2049d
**Applied fix:** Moved the "Write minisign secret key to disk (umask 077)" step from before the snapshot pass-1 to immediately before the "Real release (sign + publish)" step. The two snapshot passes (`--skip=sign`) never read the key, so the key is no longer present during the reproducibility gate. The post-job `shred` cleanup is unchanged.

### WR-02: README.md mixes "Serena" and "Helix" branding inconsistently

**Files modified:** `README.md`
**Commit:** d465a926
**Applied fix:** Replaced the open-ended "upcoming refactor" disclaimer with an explicit description of the current naming split (repo + GitHub URLs use `helix`; binary, CLI, and config keys use `serena`) and a link to `.planning/ROADMAP.md` where the full rename is tracked. Did not propagate the rename to `serena setup ...` examples or config snippets because that is a multi-file rename operation requiring its own phase, not a code-review nit.

### WR-03: CONTRIBUTING.md claims `release-snapshot` validates the build matrix -- but the dry-run currently fails

**Files modified:** `CONTRIBUTING.md`
**Commit:** 1a1ce93a
**Applied fix:** Added a "Known issue (phase 51, DEF-51-02)" callout above the `make release-snapshot` example explaining that the dry-run fails with `build constraints exclude all Go files` until the upstream tree-sitter Go bindings are vendored with `//go:build cgo` tags. Softened the later line that claimed the dry-run validates build matrix / archive packaging / checksums.txt to "once DEF-51-02 closes, will validate ...". Did NOT relax `CGO_ENABLED=0` -- that's the alternative path in the review and is a phase-level decision, not a documentation fix.

### WR-04: `release.yml` runner is `ubuntu-latest` but minisign install URL is x86_64-only

**Files modified:** `.github/workflows/release.yml`
**Commit:** 2540877f
**Applied fix:** Pinned `runs-on: ubuntu-22.04` (LTS) so the architecture is fixed at workflow level. The runtime `uname -m` check stays as a defense-in-depth backstop for runner-image changes within the LTS series.

### WR-05: INSTALL.md checksum check matches more files than downloaded -- typos can pass

**Files modified:** `INSTALL.md`
**Commit:** 378dd812
**Applied fix:** Replaced `sha256sum -c checksums.txt | grep ...: OK || ...` with an explicit single-file expected-vs-actual hash comparison: extract the hash for the downloaded archive from `checksums.txt` via `grep | awk`, compute the actual hash via `sha256sum` (or `shasum -a 256` fallback), and fail if the entry is missing or hashes differ. Folded the macOS `shasum -a 256` fallback into the same recipe so the macOS users note no longer needs to substitute commands.

## Skipped Issues

### CR-01: `CGO_ENABLED=0 go build ./cmd/serena` fails -- registry.go imports 20 CGO-only upstream bindings

**File:** `internal/treesitter/registry.go:11-37`
**Reason:** Architectural decision deferred to DEF-51-02 (already documented in `.planning/phases/51-packaging-goreleaser/deferred-items.md`); requires maintainer route selection among the 4 documented paths (vendor every upstream binding with build tags, split registry.go under `cgo` build tag with a no-op fallback, relax `CGO_ENABLED=0` and ship CGO binaries, or drop tree-sitter coverage from the binary). This is a multi-day implementation regardless of route, not a single fix the reviewer's agent can apply. Per the orchestrator context note for this run, the fix was explicitly out-of-scope.
**Original issue:** `.goreleaser.yaml` sets `CGO_ENABLED=0`, but every upstream tree-sitter Go binding ships an unconditional `// #cgo CFLAGS: ... import "C"` block with no `//go:build cgo` tag, so `CGO_ENABLED=0 go build` fails with 20 "build constraints exclude all Go files" errors. The release pipeline cannot produce a single archive in this state.

---

_Fixed: 2026-04-29_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
