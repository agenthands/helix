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

## DEF-59.1-LINUX-ZIG-LIBSTDCXX: zig cc + linux-musl target cannot link duckdb-go-bindings prebuilt static lib (libstdc++ missing)

**Deferred by:** Phase 59.1 Wave 5 first-CI-repro (run 25342130151,
sha 644d8f04, captured 2026-05-04 in
`.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-05-FIRST-CI-REPRO-LOG.md`).

**Source artifacts:**
- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-05-FIRST-CI-REPRO-LOG.md`
  (root cause analysis + full link-error trace)
- `.goreleaser.yaml` lines 35–37 (`CC=zig cc -target *-linux-musl`)
- Phase 59.1 Wave 2 SNAPSHOT-BASELINE.md (now-disproven assertion that
  CI ubuntu does not hit this).

**What was deferred:**

End-to-end CI build of the 4 helix-non-darwin targets (linux/{amd64,arm64}
+ windows/{amd64,arm64}). Phase 59.1 Wave 5's first CI exercise of the
split-runner pipeline surfaced that `zig cc -target x86_64-linux-musl`
cannot link `libduckdb_static.a` because the prebuilt duckdb-go-bindings
linux-amd64 static lib was compiled against libstdc++ on glibc, while
zig's musl target ships libc++ only. ld.lld emitted ~25 undefined-symbol
errors (`std::cout`, `std::__cxx11::basic_string`, `typeinfo for
std::ostream`, `backtrace`, `malloc_trim`, etc.) on the linux_amd64
target.

The darwin path (Apple clang on macos-14) is unaffected and is
end-to-end PASS.

**Decision:** the FALLBACK-B-MULTI-BUILD-ID architecture itself is
sound; this is a target-selection issue inside the helix-non-darwin
build entry. Resolution paths considered (full set in the FIRST-CI-REPRO
log signoff section):

1. Switch zig target musl → gnu (4-line edit; needs verification on
   whether zig+gnu provides libstdc++ or still libc++).
2. Switch linux build to host gcc + apt-installed cross toolchains
   (defeats D-02 hermetic toolchain partially).
3. Reduce shipped targets to linux/amd64 + darwin/{amd64,arm64} (3
   archives) for v1.10.0; defer linux/arm64 + windows.
4. Compile duckdb from source against libc++ during CI (scope creep).

Phase 59.1 closes as APPROVED-WITH-DEFERRAL with this item registered;
selection among options 1–4 happens in the dedicated follow-up phase.

**Trigger to revisit:**

Either:
- A maintainer wants to ship the full 6-archive matrix on a real
  `vX.Y.Z` tag-cut (linux+windows currently absent), OR
- A user reports `helix` is unavailable on linux/arm64 or windows in
  their distribution channel.

**Recommended next phase:** Phase 59.2 (zig-vs-libstdc++ resolution +
6-archive matrix close-out + first real `vX.Y.Z` cosign self-test
verification per project memory rule).

**Cost:** ~1–3 days of work depending on which option (1–4) wins. The
project memory rule "verify against actual CI bundle before declaring
done" applies — the resolution must be validated against a real CI run,
NOT a local snapshot.

---

## Index of milestone-level deferred-items files

| Milestone | Phase | File |
|-----------|-------|------|
| v1.1 | 06-test-harness-go-dogfooding | `.planning/milestones/v1.1-phases/06-test-harness-go-dogfooding/deferred-items.md` |
| v1.1 | 08-advanced-testing | `.planning/milestones/v1.1-phases/08-advanced-testing/deferred-items.md` |
| v1.9 | 51-packaging-goreleaser | `.planning/milestones/v1.9-phases/51-packaging-goreleaser/deferred-items.md` |
| v1.9 | 51.1-cgo-treesitter-gate | `.planning/milestones/v1.9-phases/51.1-cgo-treesitter-gate-gate-internal-treesitter-behind-go-build/deferred-items.md` |
| v1.9 | 52-packaging-distribution-channels | `.planning/milestones/v1.9-phases/52-packaging-distribution-channels/deferred-items.md` |

## DEF-67-F01-FULL-DIFF: Full added/removed/changed FileFactDiff population

**Deferred by:** Quick-fix close-out 2026-05-12 (F-01 best-effort closure).

**What was deferred:** Full-precision diff between pre-edit FileFact and
post-edit FileFact in the live handler post-commit hook. The 2026-05-12
close-out shipped a tiered populator
(`internal/semantic/live/handler/difffacts.go`): Tier 1 (full diff) and
Tier 2 (added-only) fall through to Tier 3 (synthetic marker) when the
snapshot store / extractor APIs do not yet expose the prior-FileFact
accessor and per-file extractor.

The synthetic-marker fallback guarantees `graph_version` ADVANCES on every
live edit (the load-bearing F-01 contract per
`.planning/v1.10-MILESTONE-AUDIT.md`) but carries no actionable per-symbol
detail — every edit triggers the full cluster-recompute path.

**Trigger to reconsider:** A rank-aware MCP tool consumer reports stale
scores after a live edit (would imply the synthetic marker is too coarse
for the targeted-invalidation path) OR profiling shows the full-cluster
recompute on every edit is a hot-path cost.

**Implementation sketch:**
1. Add `GetFileFact(repoID, path) (FileFact, error)` to the snapshot store
   (`internal/semantic/store/`).
2. Add a per-file extractor entrypoint in `internal/semantic/extract/`
   exposing `ExtractFile(repoID, path) (FileFact, error)`.
3. Wire both into `tryFullDiff` in
   `internal/semantic/live/handler/difffacts.go`.
4. Add `tryAddedOnlyDiff` fallback that calls the extractor alone.

**Cost:** ~1-2 engineer-days.

**Code pointers:**
- `internal/semantic/live/handler/difffacts.go` — three-tier populator
  (Tier 1/2 are scaffolding stubs that return false; Tier 3 fallback
  ships active).
- `internal/semantic/live/handler/handler.go:382-392` — the wiring point
  where `populateRecorderForFile` is invoked when no test seam is set.
