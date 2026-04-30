---
status: partial
phase: 52-packaging-distribution-channels
source: [52-VERIFICATION.md]
started: 2026-04-30T00:00:00Z
updated: 2026-04-30T00:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. End-to-end `helix upgrade` against a real published GitHub release tag (or staged release in a fork)
expected: Daemon-detect skip on bare shell → permission probe → API fetch → semver compare → download → minisign verify against rotated production minisign.pub → extract → atomic swap → os.Exec relaunch reports new version on `helix --version`
result: [pending]
why_human: The full happy path can only be exercised against the live GitHub Releases API + a real signed archive; httptest stubs cover units (well-tested) but cannot validate the real network/IO ceremony.

### 2. Confirm minisign.pub key rotation status before tagging v1.9.0
expected: `head -1 minisign.pub` shows the production signing identity, NOT the literal `PLACEHOLDER`. Until rotation, `IsPlaceholderPubKey()` returns true and `helix upgrade` short-circuits with the "this build was made before the maintainer rotated…" message — fail-closed, but blocks the upgrade verb until rotation.
result: [pending]
why_human: Key rotation is a maintainer/operator action with no automated trigger; the placeholder guard shipped in WR-05 documents the failure mode but does not perform the rotation.

### 3. Validate CR-02 contract decision (option 1 vs option 2) for relaunch-after-upgrade flag stripping
expected: Team reviews REVIEW-FIX.md note — current implementation strips upgrade-only flags but still relaunches with remaining args (option 1); alternative (don't relaunch at all, like `gh extension upgrade`) was not chosen. Confirm option 1 matches expected operator-experience semantics.
result: [pending]
why_human: Team-policy / UX call recorded in REVIEW-FIX.md as needing human review; not a code-level bug.

### 4. Visually inspect a snapshot release to confirm 6 archives are platform-correct and sign-correct
expected: `ls dist/helix_v*.tar.gz` shows {darwin,linux,windows}×{amd64,arm64}; each has a sibling `.minisig`; `checksums.txt` exists; reproducibility CI gate (Phase 51) was last green.
result: [pending]
why_human: Snapshot was already produced (6 archives present in dist/); a human spot-check before publishing confirms artifact correctness end-to-end.

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
