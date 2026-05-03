---
phase: 51-packaging-goreleaser
plan: 06
subsystem: packaging-supply-chain
tags: [packaging, ci, supply-chain, threat-model, gap-closure, cr-02, cr-03, cr-04, wr-01, wr-06, in-02, in-03, in-04]
requires:
  - 51-01  # owns the release.yml structural skeleton this plan hardens
  - 51-02  # owns the .goreleaser.yaml + reproducibility gate the doc Task 3 describes
provides:
  - "release.yml pre-flight that fails closed if minisign.pub still contains the literal PLACEHOLDER marker"
  - "SHA-pinned third-party actions (actions/checkout, actions/setup-go, goreleaser/goreleaser-action)"
  - "minisign tarball SHA-256 verification before extraction (cryptographically tied to upstream signed .minisig)"
  - "uname -m arch guard that fails loud on non-x86_64 runners"
  - "if: always() shred-on-disk for /tmp/minisign.key as the last step of the release job"
  - "dist-pass-1 cleanup after the reproducibility diff gate"
  - "CONTRIBUTING.md reproducibility prose that accurately describes snapshot-pair determinism (not real-release determinism)"
  - "minisign.pub comment cross-referencing the workflow gate so a future maintainer keeps the PLACEHOLDER marker until the real key is generated"
affects:
  - .github/workflows/release.yml
  - CONTRIBUTING.md
  - minisign.pub
tech-stack:
  added: []
  patterns:
    - "Action-pinning by full 40-char commit SHA with trailing version-tag comment for review readability"
    - "Pre-flight grep on a load-bearing marker string (PLACEHOLDER) coupled to file content"
    - "Tarball integrity verification by SHA-256 piped through sha256sum -c, with the digest separately verified against an upstream signed .minisig sidecar"
    - "if: always() post-step for in-job secret cleanup"
key-files:
  created: []
  modified:
    - path: .github/workflows/release.yml
      role: hardened release workflow with pre-flight, SHA-pinned actions, tarball verification, arch guard, and cleanup steps
    - path: CONTRIBUTING.md
      role: Releasing section's reproducibility paragraph rewritten to match what the gate actually proves
    - path: minisign.pub
      role: line 1 comment now cross-references the workflow gate; line 2 byte-identical to Plan 51-01 placeholder
decisions:
  - "Took the doc-softening path for CR-03/IN-04 closure rather than extending the gate to compare real-release artifacts. The gate-extension alternative would require a Pass 4 build with --skip=publish --skip=sign after the real release pass, doubling CI time on every tag push. If a future maintainer determines that real-release-only non-determinism is materially harmful, the gate-extension path remains available."
  - "Computed MINISIGN_TARBALL_SHA256 locally because jedisct1/minisign 0.12 does not publish a signing.txt. The digest was validated by verifying the upstream-signed .minisig sidecar (https://github.com/jedisct1/minisign/releases/download/0.12/minisign-0.12-linux.tar.gz.minisig) against jedisct1's documented public key RWQf6LRCGA9i53mlYecO4IzT51TGPpvWucNSCh1CBM0QTaLn73Y7GFO3 from the upstream README. This is the fallback path the plan documented for the case where signing.txt is unavailable."
  - "Pinned the same goreleaser-action commit SHA across all three invocation sites (snapshot pass 1, snapshot pass 2, real release) so a single SHA bump updates all three together and they cannot drift."
metrics:
  duration: ~25min
  completed: 2026-04-29
---

# Phase 51 Plan 06: release.yml hardening + CR-03 doc-softening Summary

Hardened `.github/workflows/release.yml` with a PLACEHOLDER pre-flight, SHA-pinned third-party actions, minisign tarball SHA-256 verification, uname -m arch guard, dist-pass-1 cleanup, and an `if: always()` secret-key shred step; aligned `CONTRIBUTING.md`'s reproducibility paragraph with what the gate actually proves; and updated `minisign.pub`'s comment to cross-reference the workflow gate.

## What was built

### Task 1 -- release.yml pre-flight + cleanup steps (CR-02 + WR-06 + IN-03)

Added three new steps to `.github/workflows/release.yml`:

1. **`Refuse PLACEHOLDER minisign public key`** (inserted between Checkout and Set up Go). Runs `grep -q 'PLACEHOLDER' minisign.pub` and exits 1 with a `::error file=minisign.pub::` annotation if matched. The CI workflow now fails closed BEFORE downloading Go or building anything if the maintainer has not yet replaced the placeholder key. Wasted CI minutes are minimized.

2. **`Clean dist-pass-1 after diff gate`** (inserted after the diff step, before the real release step). Runs `rm -rf dist-pass-1`. Cosmetic on ephemeral runners but tidy.

3. **`Wipe minisign secret key`** (inserted as the last step of the job). Has `if: always()` so it runs whether the real release succeeds, fails, or is cancelled. Body is `shred -u /tmp/minisign.key 2>/dev/null || rm -f /tmp/minisign.key` -- belt-and-suspenders cleanup that shrinks the in-job blast radius if a future step pulls a third-party action that scans /tmp.

**Commit:** `90325067` -- `feat(51-06): add release.yml pre-flight, dist-pass-1 cleanup, and key wipe steps`

### Task 2 -- action SHA-pinning + minisign tarball verification (WR-01 + CR-04 + IN-02)

Pinned all third-party action `uses:` lines to full 40-char commit SHAs (with trailing version-tag comments for review readability):

| Action | Full SHA | Tag | Verified |
|--------|----------|-----|----------|
| `actions/checkout` | `11bd71901bbe5b1630ceea73d27597364c9af683` | `v4.2.2` | 2026-04-29 via `gh api repos/actions/checkout/git/refs/tags/v4.2.2` |
| `actions/setup-go` | `3041bf56c941b39c61721a86cd11f3bb1338122a` | `v5.2.0` | 2026-04-29 via `gh api repos/actions/setup-go/git/refs/tags/v5.2.0` |
| `goreleaser/goreleaser-action` | `ec59f474b9834571250b370d4735c50f8e2d1e29` | `v7.0.0` | 2026-04-29 via `gh api repos/goreleaser/goreleaser-action/git/refs/tags/v7.0.0` |

`goreleaser/goreleaser-action` appears at three call sites (snapshot pass 1, snapshot pass 2, real release); all three pin to the same SHA so a single bump updates all three together.

Note: the plan's `<interfaces>` section listed candidate SHAs current as of 2026-04 (`b4ffde65...` for checkout v4.2.2, `41dfa10b...` for setup-go v5.2.0, `9c156ee8...` for goreleaser v7.0.0). The executor re-resolved each tag at execution time per the plan's "executor MUST verify" instruction. The actual SHAs returned by the GitHub Refs API today differ from the plan's pre-recorded values, so the values committed are the freshly-resolved ones above. The trailing version-tag comments preserve human readability.

Replaced the `Install minisign (pinned)` step body with:
- A `MINISIGN_TARBALL_SHA256="9a599b48ba6eb7b1e80f12f36b94ceca7c00b7a5173c95c3efc88d9822957e73"` constant for `minisign-0.12-linux.tar.gz`. Source: digest computed locally against the upstream GitHub release tarball, then validated by verifying the upstream-signed `.minisig` sidecar (`https://github.com/jedisct1/minisign/releases/download/0.12/minisign-0.12-linux.tar.gz.minisig`) against jedisct1's published public key `RWQf6LRCGA9i53mlYecO4IzT51TGPpvWucNSCh1CBM0QTaLn73Y7GFO3` (documented in https://github.com/jedisct1/minisign README). Verification ran successfully: `Signature and comment signature verified -- Trusted comment: timestamp:1737030580 file:minisign-0.12-linux.tar.gz hashed`. Verified 2026-04-29.
- An `ARCH="$(uname -m)"` guard that fails with an actionable `::error::` annotation if the runner is not x86_64. The guard runs BEFORE the curl, so a non-x86_64 runner fails with the explicit "release.yml minisign install assumes x86_64 runner" message rather than a cryptic curl-404 or tar-extraction error.
- An `echo "${MINISIGN_TARBALL_SHA256}  /tmp/minisign.tar.gz" | sha256sum -c -` line that runs BEFORE `tar -xzf`. A compromised tarball fails the workflow before any attacker-controlled binary handles `MINISIGN_PRIVATE_KEY`.

**Commit:** `d4b5d689` -- `feat(51-06): pin actions to commit SHAs and verify minisign tarball`

### Task 3 -- CONTRIBUTING.md reproducibility paragraph softening (CR-03 + IN-04)

Replaced the third sentence of the Releasing intro paragraph. The previous prose claimed `a non-deterministic build cannot reach users`, which overstates what the gate validates. The replacement accurately describes:
- What the gate DOES catch: snapshot-pair determinism, build-environment drift (toolchain, mod_timestamp, trimpath, GOFLAGS).
- What the gate DOES NOT catch: real-release-only non-determinism (tag-only build constants, changelog generation).
- An actionable workaround: `goreleaser release --snapshot --clean --skip=sign` locally on the same tag and diff against the published `dist/` from CI.

**Commit:** `9d7968a2` -- `docs(51-06): soften reproducibility paragraph to match what gate proves`

### Task 4 -- minisign.pub cross-reference comment (Concern B documentation half)

Updated line 1 to:
```
untrusted comment: serena minisign public key -- PLACEHOLDER (release.yml pre-flight greps this marker; remove "PLACEHOLDER" when overwriting with the real key)
```

The literal substring `PLACEHOLDER` is preserved so the workflow grep still triggers (which is the desired state until the maintainer overwrites the file with a real key offline). Line 2 (`RWQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`) is byte-identical to Plan 51-01's placeholder -- this plan does NOT generate or commit a real key; that remains a maintainer offline `user_setup` action.

**Commit:** `5b7555b0` -- `docs(51-06): cross-reference release.yml pre-flight in minisign.pub comment`

## VERIFICATION gaps closed

| Gap | Disposition before | Disposition after | Mechanism |
|-----|---------------------|--------------------|-----------|
| **CR-02** -- placeholder release publication | unmitigated | mitigate | release.yml pre-flight greps `minisign.pub` for the literal `PLACEHOLDER` marker before any build runs; exits 1 with file annotation. |
| **CR-03 / IN-04** -- doc/gate disagreement | overstated | mitigate (doc-softening path) | CONTRIBUTING.md prose rewritten; gate is unchanged. |
| **CR-04** -- minisign tarball substitution | accepted (residual risk) | mitigate | SHA-256 verified before extraction, with the digest itself cross-validated against jedisct1's signed `.minisig` and documented public key. |
| **WR-01** -- mutable major-tag action pins | unmitigated | mitigate | All third-party `uses:` lines pinned to full 40-char commit SHAs. |
| **WR-06** -- secret key persists in /tmp | unmitigated | mitigate | `if: always()` last step shreds (or rm-fs) `/tmp/minisign.key`. |
| **IN-02** -- runner arch flip | unmitigated | mitigate | `uname -m` guard fails loud on non-x86_64. |
| **IN-03** -- dist-pass-1 left in workspace | unmitigated | mitigate | Explicit `rm -rf dist-pass-1` after diff gate. |

Concern B's keypair-generation half remains a maintainer offline `user_setup` action (documented in CONTRIBUTING.md "Releasing > One-time keypair setup"). The release.yml pre-flight added in this plan is the structural enforcement that makes "shipped a placeholder release" impossible until the maintainer completes the offline setup.

## Decisions made

1. **CR-03 closure path: doc-softening, not gate-extension.** The plan offered two paths: rewrite the CONTRIBUTING.md prose (Task 3 -- chosen) or add a Pass 4 build with `--skip=publish --skip=sign` after the real release pass to compare real-release artifacts (alternative). The gate-extension would double CI time on every tag push. If a future maintainer determines real-release-only non-determinism is materially harmful, the gate-extension path is documented as deferred work. No new entry in `deferred-items.md` because this is a known alternative covered in the plan's `<objective>`, not a discovered gap.

2. **MINISIGN_TARBALL_SHA256 sourced via local computation + .minisig verification.** Upstream jedisct1/minisign 0.12 does not publish a `signing.txt` checksums file (the release ships individual `.minisig` files per asset). The plan's documented fallback path applies: download the tarball locally, compute its SHA-256, and verify the upstream `.minisig` against jedisct1's published public key. The chained verification (`.minisig` against the documented public key, then SHA-256 against the verified tarball) is cryptographically equivalent to a `signing.txt` workflow and is documented in the workflow file's comment block.

3. **Same SHA across all three goreleaser-action call sites.** The action appears three times (snapshot pass 1, snapshot pass 2, real release). All three pin to `ec59f474b9834571250b370d4735c50f8e2d1e29` (v7.0.0). A future bump must update all three together; this is enforced by a future Renovate-style review (the plan does not add automated enforcement -- a single bump tool typically updates all occurrences in one PR).

## Threat surface delta

This plan reduces the supply-chain threat surface; no new attack surface introduced. The full STRIDE register from the plan's `<threat_model>` section is now mitigated for T-51-29 through T-51-35 -- every identified threat has a tied text change in the modified files.

## Outstanding maintainer prerequisites (carried from Plan 51-01)

The release workflow cannot publish a usable signed release until the maintainer completes the offline keypair setup:

1. Generate the keypair locally on a trusted machine: `minisign -G -p minisign.pub -s minisign.key`
2. Overwrite `minisign.pub` (in this repo) with the real public key. **REMOVE the word "PLACEHOLDER" from the comment line** -- the workflow's pre-flight grep is what enforces this.
3. Upload `MINISIGN_PRIVATE_KEY` (contents of `minisign.key`) and `MINISIGN_PASSWORD` as GitHub Actions secrets.
4. Cut a `v0.0.0-rc-test` tag to validate the workflow end-to-end before the first production release.

CONTRIBUTING.md "Releasing > One-time keypair setup" is the canonical reference.

## Self-Check: PASSED

Files verified to exist:
- FOUND: `.github/workflows/release.yml` (modified, 117 lines after edits)
- FOUND: `CONTRIBUTING.md` (modified)
- FOUND: `minisign.pub` (modified)
- FOUND: `.planning/phases/51-packaging-goreleaser/51-06-SUMMARY.md` (this file)

Commits verified to exist in git log:
- FOUND: `90325067` -- Task 1 (pre-flight + cleanup steps)
- FOUND: `d4b5d689` -- Task 2 (action SHAs + tarball verification)
- FOUND: `9d7968a2` -- Task 3 (CONTRIBUTING.md prose)
- FOUND: `5b7555b0` -- Task 4 (minisign.pub cross-reference)

Plan-level verification (all 11 success criteria):
- PASS: PLACEHOLDER pre-flight before Set up Go (line ordering verified via awk)
- PASS: Clean dist-pass-1 between diff gate and Real release
- PASS: Wipe minisign secret key with `if: always()` as the last step (no subsequent steps)
- PASS: 0 mutable major-tag pins on third-party actions
- PASS: 5 SHA-pinned `uses:` lines (1 checkout + 1 setup-go + 3 goreleaser-action)
- PASS: 64-hex `MINISIGN_TARBALL_SHA256` and `sha256sum -c -` line
- PASS: `uname -m` arch guard before the curl
- PASS: CONTRIBUTING.md prose rewritten (no `non-deterministic build cannot reach users`; contains `two consecutive snapshot builds with identical inputs`, `real-release artifacts that ship to users`, `goreleaser release --snapshot --clean --skip=sign`)
- PASS: minisign.pub line 1 contains `PLACEHOLDER` and `release.yml pre-flight greps this marker`
- PASS: YAML still parses (`python3 -c "import yaml; yaml.safe_load(...)"` exits 0)
- PASS: Exactly 3 modified files since base (`.github/workflows/release.yml`, `CONTRIBUTING.md`, `minisign.pub`)
