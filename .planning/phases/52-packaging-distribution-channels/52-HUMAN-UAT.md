---
status: partial
phase: 52-packaging-distribution-channels
source: [52-VERIFICATION.md]
started: 2026-04-30T00:00:00Z
updated: 2026-04-30T00:00:00Z
---

## Current Test

[awaiting v1.9.0 tag — UAT-1 full ceremony + UAT-2 key rotation are release-tag-gated]

## Tests

### 1. End-to-end `helix upgrade` against a real published GitHub release tag
expected: Daemon-detect skip on bare shell → permission probe → API fetch → semver compare → download → minisign verify against rotated production minisign.pub → extract → atomic swap → os.Exec relaunch reports new version on `helix --version`
result: partial — agent-verified in three sub-checks (2026-04-30):
  - `helix update` hits live GitHub Releases API correctly: returned `not_found: github release not found (url=https://api.github.com/repos/agenthands/helix/releases/latest)` (no v* release published yet — expected pre-tag).
  - Daemon-detect short-circuit fires: `HELIX_RUNNING_AS_DAEMON=1 ./helix upgrade` returned "helix is running as a daemon child process; restart the daemon manually after upgrading from a non-daemon shell" — exit 0 (clean refusal).
  - All 4 flags exposed via `helix upgrade --help`: `--check`, `--dry-run`, `--prerelease`, `--version`.
  Full happy-path (download → verify → swap → relaunch) blocked on UAT-2 rotation + v1.9.0 tag publication; will be exercised at release time.
why_human: Live happy-path ceremony requires a real published release with a real signature — chicken-and-egg with UAT-2 (cannot tag v1.9.0 until key is rotated, cannot exercise upgrade until v1.9.0 is tagged).

### 2. Confirm minisign.pub key rotation status before tagging v1.9.0
expected: `head -1 minisign.pub` shows the production signing identity, NOT the literal `PLACEHOLDER`. Until rotation, `IsPlaceholderPubKey()` returns true and `helix upgrade` short-circuits with the "this build was made before the maintainer rotated…" message — fail-closed, but blocks the upgrade verb until rotation.
result: pending — agent-verified placeholder still embedded (2026-04-30):
  - `head -2 minisign.pub` → `untrusted comment: serena minisign public key -- PLACEHOLDER` (repo-root signing source still placeholder).
  - `head -2 internal/upgrade/minisign.pub` → identical placeholder content (build-time embed copy mirrors the source byte-for-byte; `make verify-embed-pubkey` is clean).
  Action required: maintainer rotates `minisign.pub` to the real production signing key before tagging v1.9.0. WR-05 fail-closed guard ensures pre-rotation builds cannot upgrade — they short-circuit with "this build was made before the maintainer rotated the minisign public key" rather than attempting verification.
why_human: Key rotation is a maintainer/operator action with no automated trigger.

### 3. Validate CR-02 contract decision (option 1 vs option 2) for relaunch-after-upgrade flag stripping
expected: Team reviews REVIEW-FIX.md note — current implementation strips upgrade-only flags but still relaunches with remaining args (option 1); alternative (don't relaunch at all, like `gh extension upgrade`) was not chosen. Confirm option 1 matches expected operator-experience semantics.
result: passed — option 1 confirmed (2026-04-30): user reviewed REVIEW-FIX.md note and confirmed the shipped behavior matches expected operator experience. Code in internal/upgrade/upgrade.go:271 stripUpgradeVerb stays as-is.

### 4. Visually inspect a snapshot release to confirm 6 archives are platform-correct and sign-correct
expected: `ls dist/helix_v*.tar.gz` shows {darwin,linux,windows}×{amd64,arm64}; each has a sibling `.minisig`; `checksums.txt` exists; reproducibility CI gate (Phase 51) was last green.
result: passed (with documented signing-skip caveat) — agent-verified on 2026-04-30:
  - 6 archives present: helix_v1.8-SNAPSHOT-1b4b6cce_{darwin,linux,windows}_{amd64,arm64}.tar.gz (sizes 8.4–9.4 MB).
  - dist/checksums.txt exists.
  - No sibling .minisig files — expected for snapshot builds: `make release-snapshot` skips signing because the secret key lives only in CI (CONTRIBUTING.md:160 documents this); real .minisig signatures are produced by `release.yml` on tag push, not by snapshot.

## Summary

total: 4
passed: 2
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
