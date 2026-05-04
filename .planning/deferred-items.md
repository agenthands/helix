# Helix Deferred Items (project-level index)

> Project-level register of items that were considered, scoped, but
> intentionally deferred to a later phase or milestone. Each entry has a
> stable `DEF-<phase>-<slug>` ID, a citation back to the phase/decision
> that deferred it, and the trigger condition that would justify
> reconsidering.

> Milestone-level deferred-items files
> (`.planning/milestones/<milestone>/<phase>/deferred-items.md`) remain
> the source-of-truth for phase-internal scoping; this file is the
> project-level INDEX surfacing items that may matter to future
> milestones.

---

## DEF-59-NOTARIZE: Apple Developer ID code signing + notarization for darwin archives

**Deferred by:** Phase 59.1 D-19 (locked 2026-05-04).

**Source artifacts:**
- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-CONTEXT.md` (D-19)
- `INSTALL.md` "macOS Gatekeeper workaround" section
- `README.md` macOS install note

**What was deferred:**

Apple Developer ID code signing + notarization through Apple's notary
service (`notarytool`). Without these signatures, macOS Gatekeeper blocks
the unsigned darwin binaries on first launch with errors like:

> "helix" can't be opened because Apple cannot check it for malicious
> software.

Or:

> "helix" is damaged and can't be opened. You should move it to the Trash.

Users currently must use the right-click → Open workaround (documented in
`INSTALL.md`) or clear the quarantine xattr from a terminal.

**Why deferred:**

- Apple Developer Program membership costs $99/year — a real expense
  that requires user / org sign-off.
- Apple-credential management infrastructure (`AC_PASSWORD` /
  `AC_USERNAME` / Apple ID / app-specific-password) needs design.
- Phase 59.1 is scoped to the CGO=1 build pipeline; signing/notarization
  is a separate concern.

**Trigger to reconsider:**

- A real Mac user files a Gatekeeper-blocking install bug.
- Helix moves toward "production binary distribution" milestone in v1.11+.
- Annual macOS release (Sequoia / Tahoe / etc.) tightens Gatekeeper
  defaults to the point that the right-click workaround no longer works.

**Implementation sketch:**

1. Provision an Apple Developer Program account.
2. Add `notarytool` step in the `release-darwin` job:
   ```yaml
   - name: Notarize darwin binaries
     env:
       AC_USERNAME: ${{ secrets.APPLE_ID }}
       AC_PASSWORD: ${{ secrets.APPLE_APP_SPECIFIC_PASSWORD }}
       AC_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}
     run: |
       xcrun notarytool submit dist/helix-darwin_*/helix \
         --apple-id "$AC_USERNAME" \
         --password "$AC_PASSWORD" \
         --team-id "$AC_TEAM_ID" \
         --wait
   ```
3. Add `goreleaser` `notarize:` block (if goreleaser supports it) or
   perform notarization out-of-band before `goreleaser release`.

**Cost:** ~$99/yr (Apple Developer Program) + ~$0/run (notarytool is
free). Implementation effort: 1-2 days including credential setup.

---

## DEF-59-DARWIN-CANARY: Nightly darwin rebuild canary workflow

**Deferred by:** Phase 59.1 D-16 (locked 2026-05-04).

**Source artifacts:**
- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-CONTEXT.md` (D-16)
- `.github/workflows/release.yml` (`release-darwin` job has tag-gated `if:` per D-16)

**What was deferred:**

A separate workflow (`.github/workflows/darwin-canary.yml`) that rebuilds
darwin from `main` on a daily schedule. Detects Xcode-image drift on
macos-14 between tag-cut runs (which can be months apart), surfacing
breakage early instead of at the next release-cut moment.

**Why deferred:**

- GitHub bills macos-14 at ~10× the linux rate.
- 30 macOS-minutes/day × 365 days = ~180 macOS-hours/year of CI billing
  for an unused-most-of-the-time canary.
- PR validation already covers linux+windows; darwin breakage on PRs is
  intentionally not gated and surfaces at tag-cut time (D-16).

**Trigger to reconsider:**

- A tag run fails on darwin due to Xcode-image drift between tags.
- Tag-cut frequency drops below quarterly (drift window widens beyond
  tolerable).
- GitHub introduces a cheaper macOS runner tier.

**Implementation sketch:**

Mirror `release-darwin` job into a separate workflow:
```yaml
name: darwin-canary
on:
  schedule:
    - cron: '0 6 * * *'   # daily at 06:00 UTC
  workflow_dispatch:
jobs:
  canary:
    runs-on: macos-14
    steps:
      # ...same Xcode select + build as release-darwin, but no upload
      # to merge; just `goreleaser build --id helix-darwin --snapshot`
      # and a green/red signal.
```
Failure triggers a Slack/email alert.

**Cost:** ~30 macOS-minutes/day = ~$2-4/month at GitHub's macos-14 rate.

---

## DEF-59-WIN-ARM64-RESTORE: Restore native windows-arm64 semantic-store path

**Deferred by:** Phase 59.1 D-14 (Path-3 platform stub, closed by Wave 1).

**Source artifacts:**
- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-RESEARCH.md` (Post-Research Decision Lock D-14)
- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-01-SUMMARY.md` (Wave 1 — duckdb_winarm64.go shipped)
- `internal/semantic/store/duckdb_winarm64.go` (the platform stub)
- `internal/semantic/store/duckdb.go` (carries `//go:build !(windows && arm64)`)

**What was deferred:**

Native windows-arm64 support for the semantic-store / duckdb backend.
Currently `internal/semantic/store/duckdb_winarm64.go` returns
`serr.Unsupported` for the semantic-store entry points on the
`windows && arm64` target. This keeps the 6-archive goreleaser matrix
intact while honestly signaling that the semantic-store feature is
unavailable on win-arm64.

**Why deferred:**

- `duckdb-go-bindings` does NOT ship a `lib/windows-arm64` artifact
  upstream.
- Building `duckdb_static.a` for windows-arm64 from source requires
  MSVC ARM64 cross-compile tooling and is out-of-scope for Phase 59.1.
- The platform stub is **platform**-conditional, NOT CGO-conditional, so
  it does not violate the phase's "no more `_nocgo.go` accumulation"
  invariant.

**Trigger to reconsider:**

- `duckdb-go-bindings` upstream ships a `lib/windows-arm64` artifact.
- A real Windows-on-ARM user files a "semantic_index unavailable" bug.
- We adopt MSVC ARM64 cross-compile tooling for other reasons.

**Implementation sketch:**

1. Watch `github.com/duckdb/duckdb-go-bindings` for a `lib/windows-arm64`
   directory.
2. When available: delete `internal/semantic/store/duckdb_winarm64.go`;
   drop the `//go:build !(windows && arm64)` constraint from
   `internal/semantic/store/duckdb.go` (rename if needed).
3. Update `.goreleaser.yaml` to add `windows/arm64` to the
   `helix-non-darwin` build entry's matrix (currently already there;
   verify).
4. Validate via a manual Windows-on-ARM device or VM.

**Cost:** ~1 day of work once upstream lib is available; primary cost
is the upstream wait.

---

## Index of milestone-level deferred-items files

| Milestone | Phase | File |
|-----------|-------|------|
| v1.1 | 06-test-harness-go-dogfooding | `.planning/milestones/v1.1-phases/06-test-harness-go-dogfooding/deferred-items.md` |
| v1.1 | 08-advanced-testing | `.planning/milestones/v1.1-phases/08-advanced-testing/deferred-items.md` |
| v1.9 | 51-packaging-goreleaser | `.planning/milestones/v1.9-phases/51-packaging-goreleaser/deferred-items.md` |
| v1.9 | 51.1-cgo-treesitter-gate | `.planning/milestones/v1.9-phases/51.1-cgo-treesitter-gate-gate-internal-treesitter-behind-go-build/deferred-items.md` |
| v1.9 | 52-packaging-distribution-channels | `.planning/milestones/v1.9-phases/52-packaging-distribution-channels/deferred-items.md` |
