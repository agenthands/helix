# Phase 52: packaging-distribution-channels - Context

**Gathered:** 2026-04-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Land three end-user install paths downstream of the goreleaser pipeline shipped in Phase 51:

1. **Homebrew** — `brew tap postfix/serena-packages https://github.com/postfix/serena-packages && brew install serena` works on macOS (arm64 + amd64) and Linux (amd64). Formula auto-updates on every tag.
2. **Scoop** — `scoop bucket add serena https://github.com/postfix/serena-packages && scoop install serena` works on Windows amd64. Manifest auto-updates on every tag.
3. **Linux native packages** — goreleaser nfpms produces `.deb` and `.rpm` artifacts published as GitHub Release assets. Users download + install with `dpkg -i` or `rpm -i`. Documented in INSTALL.md. Satisfies PKG-04.

Phase boundary excludes:
- Hosted apt/dnf/AUR repositories (deferred to PKG-DEFER-01)
- Docker / container images (deferred to PKG-DEFER-02)
- In-binary auto-update (Out of Scope project-wide)

</domain>

<decisions>
## Implementation Decisions

### Repository layout
- **D-01:** Single monorepo `github.com/postfix/serena-packages` holds the Homebrew Formula, the Scoop bucket manifest, and any future package metadata. Brew install is `brew tap postfix/serena-packages https://github.com/postfix/serena-packages && brew install serena`; Scoop install is `scoop bucket add serena https://github.com/postfix/serena-packages && scoop install serena`. Trade-off vs. separate `homebrew-serena` + `scoop-serena` repos: slightly longer install command for users in exchange for one repo, one token, one place to look.

### Linux native package format (PKG-04)
- **D-02:** Ship `.deb` + `.rpm` together via goreleaser's `nfpms` block. Both formats fall out of the existing pipeline for free and cover Debian/Ubuntu/Mint and Fedora/RHEL/openSUSE in one shot. AUR is explicitly deferred to PKG-DEFER-01. No second Linux format added in this phase.
- **D-03:** Distribution = direct download from GitHub Release assets. Users run `wget <release-url>/serena_<version>_amd64.deb && sudo dpkg -i …` (and rpm equivalent). No hosted apt/dnf repo, no GPG repo signing infra, no Cloudsmith. INSTALL.md documents the one-liner. Updates are manual — redownload on a new release. Phase 51's cosign+sha256 verify step is documented as optional alongside the install command.

### Auto-update credential model
- **D-04:** Authentication uses a **fine-grained Personal Access Token** scoped to ONLY `github.com/postfix/serena-packages` with `contents: write`. Stored as the `PACKAGES_PAT` GitHub Actions secret. RELEASING.md documents how to mint and rotate it. GitHub App approach is explicitly rejected for this phase — overhead exceeds the benefit at single-maintainer scale; revisit if the project ever accepts co-maintainers.
- **D-05:** Updates push **directly to the default branch** of `serena-packages` on every tag. No PR-based gate. The release tag triggers the formula/manifest commit and users can `brew upgrade` / `scoop update` minutes later. A bad release ships immediately; mitigation is to roll forward with a patch tag rather than to add a manual review step that would defeat the auto-update goal.

### End-to-end install verification
- **D-06:** Lightweight CI smoke verification only. New `release-verify.yml` workflow runs on `release: published` with a matrix:
  - macOS-latest: `brew tap postfix/serena-packages https://github.com/postfix/serena-packages && brew install serena && serena --version` (assert version matches the tag)
  - windows-latest: `scoop bucket add serena https://github.com/postfix/serena-packages && scoop install serena && serena --version`
  - ubuntu-latest container matrix: `dpkg -i serena_*.deb && serena --version` and `rpm -i serena_*.rpm && serena --version`
  Asserts `serena --version` includes the tag's version string. Skips deeper functional verification (no `serena setup …` flow). Catches the 90% failure mode (manifest/formula didn't publish, binary URL is wrong, package corrupt). On failure: workflow opens a GitHub issue. RELEASING.md does NOT require a separate manual install ritual — the CI smoke is the gate.

### Claude's Discretion
- Exact goreleaser config layout for the `brews:`, `scoops:`, and `nfpms:` blocks — follow goreleaser v2 idioms and Phase 51's existing `.goreleaser.yml` style; reuse the same artifact glob and template variables where possible.
- INSTALL.md section ordering — extend the existing structure from Phase 51 (Download → Verify → now adds Homebrew → Scoop → deb/rpm) using the same heading style.
- RELEASING.md additions — extend the existing maintainer ritual section from Phase 51 with the one-time `serena-packages` repo bootstrap (create empty repo, mint PAT, set secret) and an ongoing rotation note.
- CI matrix exact runner versions, container images, timeouts — pick reasonable defaults per goreleaser docs and the existing `release.yml` patterns.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 51 (foundation — read first)
- `.planning/phases/51-packaging-goreleaser/51-CONTEXT.md` — release pipeline decisions; especially D-13 (CGO_ENABLED=1 + zig cross), D-19 (INSTALL.md structure), D-07 (RELEASING.md ritual), D-22 (verification scoping)
- `.planning/phases/51-packaging-goreleaser/51-RESEARCH.md` — goreleaser v2 schema, cosign keyless flow, signing topology
- `.planning/phases/51-packaging-goreleaser/51-01-SUMMARY.md` — actual artifact matrix shipped (6 binaries × cosign), deviation log
- `.planning/phases/51-packaging-goreleaser/51-02-SUMMARY.md` — INSTALL.md / RELEASING.md / Makefile state at end of Phase 51

### Existing pipeline files to extend (not rewrite)
- `.goreleaser.yml` — Phase 52 adds `brews:`, `scoops:`, `nfpms:` blocks; do not redo `builds:` or `archives:` or `signs:`
- `.github/workflows/release.yml` — needs `PACKAGES_PAT` plumbing into the goreleaser step env
- `INSTALL.md` — extend with Homebrew / Scoop / deb / rpm sections after the existing Download + Verify content
- `RELEASING.md` — extend the maintainer ritual with the one-time `serena-packages` bootstrap and PAT rotation note
- `Makefile` — no changes expected; `release-snapshot` already covers local verification

### Requirements
- `.planning/REQUIREMENTS.md` PKG-02, PKG-03, PKG-04 — the three install paths and the auto-update-on-release contract

### External docs (downstream researcher should read)
- goreleaser v2 docs for `brews`, `scoops`, and `nfpms` blocks (live URLs to be captured by `gsd-phase-researcher`)
- Homebrew custom-tap docs for the URL-form `brew tap <user>/<repo> <url>` install
- Scoop custom-bucket docs for `scoop bucket add <name> <url>`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `.goreleaser.yml` — already has the 6-binary matrix, dual cosign signs, reproducibility knobs. Phase 52 only appends new top-level blocks (`brews:`, `scoops:`, `nfpms:`); it does not modify the existing builds/archives/signs.
- `release.yml` — already runs on tag, has `contents: write` and `id-token: write` permissions, sets up zig + Go 1.25, runs goreleaser. Phase 52 wires in the `PACKAGES_PAT` env var on the goreleaser step.
- `INSTALL.md` Download + Verify sections (Phase 51) — give the heading style and verbatim cosign verify command Phase 52 will reuse for deb/rpm verification.
- `RELEASING.md` maintainer ritual (Phase 51) — extend rather than replace.
- `cmd/serena/main.go` `--version` output — the verification matrix asserts this string contains the tag.

### Established Patterns
- v2 goreleaser schema (Phase 51) — Phase 52 must match the same schema version; `brews/scoops/nfpms` block names follow v2 docs.
- Reproducibility intent (D-13) — nfpms inherits from the existing `builds:` so `-trimpath`, `mod_timestamp`, `{{.CommitDate}}` carry through automatically.
- Cosign keyless OIDC signing (Phase 51) — already covers all release artifacts including future deb/rpm because they sign by checksum file.

### Integration Points
- New repo `postfix/serena-packages` — created out-of-band by maintainer (one-time bootstrap step in RELEASING.md).
- New secret `PACKAGES_PAT` — set in `serena` repo Actions secrets.
- New workflow `.github/workflows/release-verify.yml` — separate from `release.yml`, triggered on `release: published` so it runs after goreleaser has uploaded artifacts and updated the packages repo.

</code_context>

<specifics>
## Specific Ideas

- Tap and bucket users see a single `serena` package name (no namespace prefix). Trade-off accepted on the longer initial `tap`/`bucket add` command in exchange for a clean install verb.
- `release-verify.yml` failure → open a GitHub issue (don't just turn the workflow red). This makes the failure visible even on a quiet repo.
- The smoke verification asserts the version string in `serena --version` output (which Phase 51's `cli.FormatVersion` already builds from ldflags) — that single assertion proves binary, formula/manifest, and download URL are all coherent.

</specifics>

<deferred>
## Deferred Ideas

- **Hosted apt + dnf repositories** (e.g. via Cloudsmith or self-hosted on GitHub Pages) — gives `apt update`/`dnf upgrade` UX. Belongs in PKG-DEFER-01 if/when direct-download UX proves insufficient.
- **AUR (Arch Linux) PKGBUILD publisher** — distinct ecosystem, separate publisher in goreleaser, requires AUR account + SSH key. Tracked under PKG-DEFER-01.
- **GitHub App for releases** instead of fine-grained PAT — better for multi-maintainer projects; revisit when Serena accepts co-maintainers.
- **Deeper functional verification in CI** (e.g. `serena setup claude-code` smoke after install) — out of scope for PKG-04's "install works" bar; could land later if regressions occur.
- **PR-based formula/manifest update flow** — would add a manual review gate; rejected for now in favor of direct-push speed.
- **Docker / container images** — already deferred project-wide to PKG-DEFER-02.

</deferred>

---

*Phase: 52-packaging-distribution-channels*
*Context gathered: 2026-04-26*
