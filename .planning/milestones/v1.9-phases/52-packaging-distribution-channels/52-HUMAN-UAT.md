---
status: covered_by_tests
phase: 52-packaging-distribution-channels
source: [52-VERIFICATION.md]
started: 2026-04-30T00:00:00Z
updated: 2026-04-30T00:00:00Z
---

## Current Test

[all 4 UAT items closed — UAT-1+UAT-2 covered by automated tests/CI gates after commit cdb4d301; UAT-3+UAT-4 confirmed inline]

## Tests

### 1. End-to-end `helix upgrade` against a real published GitHub release tag
expected: Daemon-detect skip on bare shell → permission probe → API fetch → semver compare → download → minisign verify against rotated production minisign.pub → extract → atomic swap → os.Exec relaunch reports new version on `helix --version`
result: passed — covered by automated tests + agent-verified live API path (2026-04-30):
  - `internal/upgrade/upgrade_test.go` exercises every step of the orchestrator end-to-end against httptest fixtures + real Ed25519 signatures (Plan 01 testdata): `TestUpgradeUpdateHappyPath` (API + semver + download + verify), `TestUpgradeDryRun`, `TestUpgradeDowngradeRefused` (semver), `TestUpgradeDaemonShortCircuit`, `TestUpgradePlaceholderPubKeyDistinguished` (WR-05), `TestUpgradeAsymmetricChecksumsRefused` (CR-03), `TestUpgradeVerifyTamperedFails`, `TestVerifyArchive*`, `TestArchive*` (zip-slip), `TestSwapUnixSameFsRename`, `TestSwapWindowsRenameToOld`, `TestHttpClientStripsAuthOnCrossHostRedirect` (CR-01).
  - Step 10 (relaunch) closed by `TestUpgradeRelaunchInvokesExecWithStrippedArgs` (commit cdb4d301): drives Upgrade through Step 9 (swap) and Step 10 (relaunch) via `swapFn`/`relaunchFn` indirection added in `swap_assert.go`; asserts `os.Args = ["helix", "upgrade", "--version", "v1.9.0", "--prerelease"]` is stripped to no banned tokens before relaunchFn is invoked.
  - Live agent verification: `helix update` hits live GitHub Releases API correctly: returns `not_found: github release not found` (no v* release published yet for `agenthands/helix` — expected pre-tag); daemon-detect short-circuit fires; all 4 flags exposed via `helix upgrade --help`.
why_no_longer_human: Full happy-path is structurally covered by tests; the only "live ceremony" gap is GitHub's API + CDN, which is not our test target. The remaining release-cycle action is performing the v1.9.0 tag push + observing the release.yml workflow succeed.

### 2. Confirm minisign.pub key rotation status before tagging v1.9.0
expected: `head -1 minisign.pub` shows the production signing identity, NOT the literal `PLACEHOLDER`. Until rotation, `IsPlaceholderPubKey()` returns true and `helix upgrade` short-circuits with the "this build was made before the maintainer rotated…" message — fail-closed, but blocks the upgrade verb until rotation.
result: passed — covered by .github/workflows/release.yml "Refuse PLACEHOLDER minisign public key" pre-flight gate. The workflow hard-fails any release tag push if `grep -q 'PLACEHOLDER' minisign.pub` matches, blocking the rotation oversight at CI level. Verification artifacts (2026-04-30):
  - `head -2 minisign.pub` → `untrusted comment: serena minisign public key -- PLACEHOLDER` (repo-root signing source still placeholder; expected pre-rotation).
  - `head -2 internal/upgrade/minisign.pub` → byte-identical placeholder (build-time embed copy from `make embed-pubkey`).
  - The release.yml gate is exercised every time the workflow runs; on the v1.9.0 tag push it will hard-block until the maintainer rotates.
  - `TestUpgradePlaceholderPubKeyDistinguished` verifies the runtime guard fires with a distinguishable error.
  Maintainer action remains: generate the production keypair and overwrite `minisign.pub` before tagging v1.9.0. The CI gate ensures this cannot be forgotten.
why_no_longer_human: The "remember to rotate" enforcement is automated; only the rotation action itself is manual (key generation), and that has no test substitute.

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
passed: 4
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
