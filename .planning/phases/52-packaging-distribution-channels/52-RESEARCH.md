# Phase 52: packaging-distribution-channels - Research

**Researched:** 2026-04-26
**Domain:** Release distribution channels — Homebrew tap, Scoop bucket, nfpms (deb/rpm), GitHub Actions auto-update plumbing
**Confidence:** HIGH for stack and patterns; MEDIUM-HIGH on the brews-vs-casks decision (one open call out for the planner — see Open Questions Q1)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Single monorepo `github.com/postfix/serena-packages` holds Homebrew Formula + Scoop manifest + future package metadata. Brew install: `brew tap postfix/serena-packages https://github.com/postfix/serena-packages && brew install serena`. Scoop install: `scoop bucket add serena https://github.com/postfix/serena-packages && scoop install serena`.
- **D-02:** Linux native packages = `.deb` + `.rpm` together via goreleaser `nfpms`. AUR deferred to PKG-DEFER-01.
- **D-03:** Linux distribution = direct download from GitHub Release assets (no hosted apt/dnf repo, no GPG repo signing). INSTALL.md documents wget/curl + `dpkg -i` / `rpm -i` one-liner. Updates are manual (redownload on new release). Phase 51 cosign verify documented as optional.
- **D-04:** Auth = fine-grained PAT scoped to `postfix/serena-packages` ONLY, with `contents: write`. Stored as `PACKAGES_PAT` GitHub Actions secret. RELEASING.md documents minting and rotation. GitHub App rejected for now.
- **D-05:** Updates push DIRECTLY to default branch of `serena-packages` on every tag — no PR gate. Bad release rolled forward via patch tag.
- **D-06:** New `release-verify.yml` workflow runs on `release: published` with matrix:
  - macos-latest: `brew tap … && brew install serena && serena --version` (assert version matches tag)
  - windows-latest: `scoop bucket add … && scoop install serena && serena --version`
  - ubuntu-latest containers: `dpkg -i serena_*.deb && serena --version` and `rpm -i serena_*.rpm && serena --version`
  Asserts version string includes the tag. On failure: opens a GitHub issue.

### Claude's Discretion

- Exact goreleaser config layout for `brews:` / `homebrew_casks:`, `scoops:`, `nfpms:` blocks — follow goreleaser v2 idioms and Phase 51's `.goreleaser.yml` style.
- INSTALL.md section ordering — extend Phase 51 structure (Download → Verify → Homebrew → Scoop → deb/rpm).
- RELEASING.md additions — extend the Phase 51 maintainer ritual with one-time `serena-packages` repo bootstrap and PAT rotation note.
- CI matrix exact runner versions, container images, timeouts — pick reasonable defaults per goreleaser docs and `release.yml` patterns.

### Deferred Ideas (OUT OF SCOPE)

- Hosted apt + dnf repositories (PKG-DEFER-01).
- AUR PKGBUILD publisher (PKG-DEFER-01).
- GitHub App for releases — revisit at multi-maintainer scale.
- Deeper functional CI verification (`serena setup …` post-install).
- PR-based formula/manifest update flow.
- Docker / container images (PKG-DEFER-02).

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PKG-02 | `brew install <tap>/serena` works on macOS arm64+amd64 and Linux amd64 with auto formula update on release | Stack §Homebrew, Architecture §brews block, Pitfall 1 (brews vs casks), Q1 (cask vs formula decision) |
| PKG-03 | `scoop install serena` works on Windows amd64 with auto manifest update on release | Stack §Scoop, Architecture §scoops block, Pitfall 2 (zip vs binary archive) |
| PKG-04 | `.deb` + `.rpm` produced by goreleaser nfpms; documented direct-download install in INSTALL.md | Stack §nfpms, Architecture §nfpms block, Pitfall 3 (musl + glibc compat), Pitfall 4 (rpm name template) |

</phase_requirements>

## Summary

Phase 52 extends Phase 51's `.goreleaser.yml` with three new top-level blocks (`brews:` or `homebrew_casks:`, `scoops:`, `nfpms:`) and adds one new GitHub Actions workflow (`release-verify.yml`) that runs after the existing `release.yml` finishes uploading artifacts. All three channels piggyback on Phase 51's existing 6-binary build matrix and signed checksum file — no rebuild, no parallel pipeline. The `serena-packages` monorepo (created out-of-band by the maintainer per RELEASING.md) holds the auto-published Formula + Scoop manifest; goreleaser pushes directly to its default branch using `PACKAGES_PAT` (a fine-grained PAT scoped to that single repo).

The most consequential planning decision the user has NOT yet locked is whether to use the legacy `brews:` block or the modern `homebrew_casks:` block in goreleaser v2 (deprecation notice landed in v2.10). **`brews:` still works**, is the only path that supports Linuxbrew (cask is macOS-only), and is what Phase 52's CONTEXT.md success criteria implicitly require ("Linux amd64" via brew is in scope per CONTEXT.md §Phase Boundary item 1). **Recommended: stay on `brews:`** for this phase and revisit at goreleaser v3 (no planned date). See Q1 for the full tradeoff.

The Scoop side is straightforward — `scoops:` (plural, v2 schema) generates a single `serena.json` manifest in the bucket repo. The nfpms side is also straightforward — one `nfpms:` entry with `formats: [deb, rpm]`, `bindir: /usr/bin`, attaches both packages to the GitHub Release as additional assets (no separate hosted repo needed per D-03). All three channels reuse Phase 51's `linux/darwin/windows × amd64/arm64` matrix; nfpms only ships `linux/{amd64,arm64}` (the deb/rpm formats are linux-only).

**Primary recommendation:** Add three blocks to `.goreleaser.yml` — `brews:` (NOT `homebrew_casks:`) targeting `postfix/serena-packages` Formula directory, `scoops:` targeting the same repo's bucket directory, and `nfpms:` for deb+rpm. Add the `PACKAGES_PAT` env var to the goreleaser step in `release.yml`. Create new `release-verify.yml` workflow with three matrix jobs. Document the one-time `serena-packages` bootstrap (create empty repo, mint fine-grained PAT, set secret) in RELEASING.md and the four install paths (brew, scoop, deb, rpm) in INSTALL.md.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Generate Homebrew Formula Ruby file | CI (goreleaser `brews:`) | — | goreleaser materializes Ruby from archive metadata + checksums [VERIFIED: goreleaser docs] |
| Generate Scoop manifest JSON file | CI (goreleaser `scoops:`) | — | goreleaser materializes JSON from archive metadata [VERIFIED: goreleaser docs] |
| Build deb/rpm packages | CI (goreleaser `nfpms:` invokes embedded nFPM) | — | nFPM is goreleaser's built-in packager; no external tool [VERIFIED: goreleaser docs] |
| Push Formula+manifest to packages repo | CI (goreleaser → git push using PACKAGES_PAT) | — | goreleaser's `repository:` block handles git auth + push [VERIFIED: goreleaser docs] |
| Attach .deb/.rpm to GitHub Release | CI (goreleaser default behavior) | — | nfpms artifacts auto-uploaded as Release assets [VERIFIED: goreleaser nfpms docs] |
| Auto-update on user side | Local (`brew upgrade`, `scoop update`, manual redownload for deb/rpm) | — | Package managers poll their bucket/tap on user command; deb/rpm have no auto-update path (D-03) |
| Post-release smoke verification | CI (release-verify.yml on `release: published` event) | — | Separate workflow because it must run AFTER goreleaser uploads complete |
| Issue creation on smoke failure | CI (gh CLI in release-verify.yml) | — | `gh issue create` with `actions/github-script` or `peter-evans/create-issue-from-file` |

## Standard Stack

### Core

| Tool / Block | Version | Purpose | Why Standard |
|--------------|---------|---------|--------------|
| goreleaser `brews:` block | v2.x schema (deprecated v2.10, still functional through v2.x; removal planned for v3, no date) | Generate + push Homebrew Formula | Only goreleaser path that supports Linuxbrew (CONTEXT.md requires Linux amd64 brew) [VERIFIED: goreleaser deprecation notice — see Sources] |
| goreleaser `scoops:` block | v2.x | Generate + push Scoop manifest | De facto Scoop publishing path; plural in v2 (was `scoop:` singular in v1) [VERIFIED: goreleaser scoop docs] |
| goreleaser `nfpms:` block | v2.x | Generate deb + rpm | Only first-party goreleaser path for Linux native packages [VERIFIED: goreleaser nfpm docs] |
| `PACKAGES_PAT` (fine-grained PAT) | GH fine-grained PAT format | Auth to `serena-packages` repo | Per D-04; least-privilege over classic PAT [CITED: github.com/settings/tokens?type=beta] |
| `release-verify.yml` workflow | net new | Post-release install smoke test | Per D-06 |
| `peter-evans/create-issue-from-file` or `actions/github-script` | latest | Open issue on smoke fail | Per D-06 final clause [ASSUMED: standard pattern; verify pin at planning time] |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| Homebrew on macos-latest runner | preinstalled | `brew install` smoke test | `release-verify.yml` macOS leg |
| Scoop on windows-latest runner | NOT preinstalled | `scoop install` smoke test | Must install scoop first via official one-liner [CITED: scoop.sh installer] |
| `dpkg`, `rpm` on ubuntu-latest containers | preinstalled in respective images | Linux smoke test | `release-verify.yml` Linux legs (ubuntu:24.04 for deb, rockylinux:9 or fedora:40 for rpm) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `brews:` (deprecated v2.10) | `homebrew_casks:` (v2.10+ replacement) | Casks are macOS-only; users on Linuxbrew (e.g. CI containers, some Linux dev workstations) cannot install via cask. CONTEXT.md success criteria includes Linux amd64 via brew. **Decision: stay on `brews:`** [VERIFIED: goreleaser v2.10 announcement — quoted in Sources]. Reassess at v3. |
| Direct push (D-05) | PR-based update | PR adds manual review gate; D-05 explicitly rejects this for auto-update speed. |
| nFPM `formats: [deb, rpm, archlinux]` | Add archlinux | AUR is the proper Arch Linux distribution channel, not nfpms `archlinux` format. D-02 defers AUR. Skip the `archlinux` nfpms format. |
| Hosted apt/dnf repo (Cloudsmith / GH Pages) | Direct download from Releases | Adds GPG signing infra + repo metadata maintenance. Deferred to PKG-DEFER-01 per D-03. |
| Classic PAT with `repo` scope | Fine-grained PAT scoped to one repo | Larger blast radius; D-04 chose fine-grained. |

### Installation / Pin

```yaml
# .goreleaser.yml additions (illustrative — full block in §Architecture)
brews:
  - repository:
      owner: postfix
      name: serena-packages
      branch: main
      token: "{{ .Env.PACKAGES_PAT }}"
    directory: Formula
scoops:
  - repository:
      owner: postfix
      name: serena-packages
      branch: main
      token: "{{ .Env.PACKAGES_PAT }}"
    directory: bucket
nfpms:
  - formats: [deb, rpm]
    bindir: /usr/bin
```

```yaml
# .github/workflows/release.yml — env addition to existing goreleaser step
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  PACKAGES_PAT: ${{ secrets.PACKAGES_PAT }}   # NEW
```

**Version verification commands** (Wave 0 / pre-flight):

```bash
goreleaser --version    # expect v2.x
goreleaser check        # validates the augmented .goreleaser.yml schema
gh auth status          # verify PAT has contents:write on serena-packages
```

## Architecture Patterns

### System Architecture Diagram

```
   tag push v* (existing Phase 51 trigger)
        │
        ▼
┌────────────────────────────────────────────────────────────────────┐
│  release.yml (Phase 51, MODIFIED)                                  │
│                                                                    │
│   actions/checkout ──► setup-go ──► setup-zig ──► cosign-installer │
│                                                          │         │
│                                                          ▼         │
│   goreleaser-action ──► goreleaser CLI v2.x                        │
│      env: GITHUB_TOKEN, PACKAGES_PAT                               │
│        │                                                           │
│        ├─► builds: 6 × `go build` (unchanged from Phase 51)        │
│        ├─► archives: 6 × .tar.gz/.zip (unchanged)                  │
│        ├─► checksum: checksums.txt (unchanged)                     │
│        ├─► signs: cosign keyless (unchanged)                       │
│        │                                                           │
│        ├─► NEW: nfpms: → serena_*.deb + serena_*.rpm               │
│        │      (linux/{amd64,arm64} only)                           │
│        │      attached to GitHub Release as assets                 │
│        │                                                           │
│        ├─► NEW: brews: → Formula/serena.rb                         │
│        │      git push to postfix/serena-packages:main             │
│        │      (using PACKAGES_PAT)                                 │
│        │                                                           │
│        └─► NEW: scoops: → bucket/serena.json                       │
│               git push to postfix/serena-packages:main             │
│               (using PACKAGES_PAT)                                 │
└────────────────────────────────────────────────────────────────────┘
        │
        ▼
   GitHub Release: published event fires
        │
        ▼
┌────────────────────────────────────────────────────────────────────┐
│  release-verify.yml (NEW, on release: published)                   │
│                                                                    │
│   matrix:                                                          │
│     ├─ macos-latest:                                               │
│     │    brew tap postfix/serena-packages <url>                    │
│     │    brew install serena                                       │
│     │    serena --version | grep ${TAG}                            │
│     │                                                              │
│     ├─ windows-latest:                                             │
│     │    install scoop                                             │
│     │    scoop bucket add serena <url>                             │
│     │    scoop install serena                                      │
│     │    serena --version | findstr ${TAG}                         │
│     │                                                              │
│     ├─ ubuntu-latest (deb in container ubuntu:24.04):              │
│     │    download serena_*.deb from release                        │
│     │    dpkg -i serena_*_amd64.deb                                │
│     │    serena --version | grep ${TAG}                            │
│     │                                                              │
│     └─ ubuntu-latest (rpm in container rockylinux:9):              │
│          download serena_*.rpm from release                        │
│          rpm -i serena_*.x86_64.rpm                                │
│          serena --version | grep ${TAG}                            │
│                                                                    │
│   on any failure: gh issue create --title "release-verify failed   │
│                                    on ${tag} for ${channel}"      │
└────────────────────────────────────────────────────────────────────┘
        │
        ▼
   End user runs:
     brew install serena         (poll the tap)
     scoop install serena        (poll the bucket)
     wget …deb && dpkg -i …      (manual)
     wget …rpm && rpm -i …       (manual)
```

### Recommended Project Structure (additions only)

```
.goreleaser.yml                  # AMENDED — append brews:, scoops:, nfpms:
.github/workflows/
  release.yml                    # AMENDED — add PACKAGES_PAT env to goreleaser step
  release-verify.yml             # NEW (D-06)
INSTALL.md                       # AMENDED — add Homebrew, Scoop, deb, rpm sections
RELEASING.md                     # AMENDED — add serena-packages bootstrap + PAT rotation note

# Out-of-band (NOT in serena repo):
github.com/postfix/serena-packages/   # NEW repo — empty README only at bootstrap
  Formula/serena.rb              # auto-generated by brews:
  bucket/serena.json             # auto-generated by scoops:
  README.md                      # one-time human commit
```

### Pattern: Reference `.goreleaser.yml` additions

These three blocks APPEND to Phase 51's existing `.goreleaser.yml`. Do not modify `builds:`, `archives:`, `checksum:`, `signs:`, or `release:`.

#### `brews:` block (PKG-02)

```yaml
# Phase 52 — Homebrew Formula auto-publish
# Decision: use brews: (not homebrew_casks:) because casks are macOS-only and
# our success criteria require Linux amd64 brew install. brews: is deprecated
# in v2.10 but still functional; removal is in v3 (no date).
brews:
  - name: serena
    # Use the macOS+Linux archives produced by Phase 51's `archives:` block.
    ids:
      - archive
    repository:
      owner: postfix
      name: serena-packages
      branch: main
      token: "{{ .Env.PACKAGES_PAT }}"
    directory: Formula
    homepage: "https://github.com/postfix/serena"
    description: "Serena — The IDE for your coding agent. LSP-backed MCP runtime."
    license: "Apache-2.0"   # Confirm against repo LICENSE; placeholder if different
    # Goreleaser auto-generates on_macos / on_linux / on_intel / on_arm blocks
    # from the build matrix. No need to write Ruby manually.
    install: |
      bin.install "serena"
    test: |
      system "#{bin}/serena", "--version"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    commit_msg_template: "Brew formula update for {{ .ProjectName }} version {{ .Tag }}"
```

**Why `ids: [archive]`:** ties the Formula to Phase 51's existing archive entry (id `archive`), so the Formula's download URLs point to the published `.tar.gz` files.

**Why no `dependencies:`:** Serena is a single static binary (zig-built static against musl on Linux per Phase 51's CC overrides). No runtime deps.

**Why no `caveats:`:** Setup is documented in INSTALL.md / `serena setup <client>`; no install-time message needed.

#### `scoops:` block (PKG-03)

```yaml
# Phase 52 — Scoop manifest auto-publish (Windows amd64).
# Note: plural `scoops:` in v2 schema (was singular `scoop:` in v1).
scoops:
  - name: serena
    ids:
      - archive
    repository:
      owner: postfix
      name: serena-packages
      branch: main
      token: "{{ .Env.PACKAGES_PAT }}"
    directory: bucket
    homepage: "https://github.com/postfix/serena"
    description: "Serena — The IDE for your coding agent. LSP-backed MCP runtime."
    license: "Apache-2.0"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    commit_msg_template: "Scoop update for {{ .ProjectName }} version {{ .Tag }}"
```

**No `persist:`:** Serena does not write user data outside `~/.serena/` (which Scoop's persist mechanism doesn't manage; it persists files inside the install dir). Skip.

**No `pre_install:` / `post_install:`:** No manual setup required at install time.

**No `depends:`:** Single static binary.

**No `shortcuts:`:** Serena is a CLI; Start menu entry would be misleading.

#### `nfpms:` block (PKG-04)

```yaml
# Phase 52 — deb + rpm via embedded nFPM. Single entry covers both formats.
# Linux only (amd64 + arm64); macOS/Windows are skipped automatically because
# the `builds:` matrix limits which Os/Arch combos nfpms can target.
nfpms:
  - id: linux-pkg
    package_name: serena
    builds:
      - serena
    vendor: "postfix"
    homepage: "https://github.com/postfix/serena"
    maintainer: "postfix <noreply@github.com>"
    description: |
      Serena — The IDE for your coding agent.
      LSP-backed MCP runtime providing semantic code retrieval, editing,
      and refactoring across 52 languages.
    license: "Apache-2.0"
    formats:
      - deb
      - rpm
    bindir: /usr/bin
    # No contents: stanza needed — goreleaser auto-installs the binary at bindir.
    # No dependencies — static binary built against musl (Phase 51 zig CC).
    # No scripts (preinstall/postinstall) — nothing to set up beyond the binary.
    file_name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Arch }}{{ if eq .Format "rpm" }}{{ end }}
    # rpm-specific summary (rpm requires a one-line summary distinct from description).
    rpm:
      summary: "LSP-backed MCP runtime for coding agents"
```

**Reproducibility carry-over:** Per Phase 51's CONTEXT.md notes, `mod_timestamp` and `-trimpath` from the `builds:` block carry through to the nfpms binary embedded inside the deb/rpm. The `.deb`/`.rpm` archive itself contains a build timestamp; goreleaser sets this from `--release-date` or the tag commit date by default. Phase 51's reproducibility ritual (D-07) will need its diff exclude list extended to cover deb/rpm; document in RELEASING.md.

**Static binary tradeoff:** Linux Phase 51 binaries are zig-built against **musl** for portability. Most distros ship glibc; running a musl-static binary is fine on glibc systems (musl statically linked, no dynamic resolver involved). The deb/rpm therefore have NO runtime `dependencies:` declared. See Pitfall 3.

**No archlinux format:** AUR is the proper publisher and uses a separate goreleaser block (`aurs:`); deferred to PKG-DEFER-01 per D-02.

### Pattern: `release.yml` patch (Phase 51 file, modified)

```yaml
# Patch to existing .github/workflows/release.yml — only env: gets a new key.
- name: Run goreleaser
  uses: goreleaser/goreleaser-action@v6.4.0
  with:
    distribution: goreleaser
    version: '~> v2'
    args: release --clean
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    PACKAGES_PAT: ${{ secrets.PACKAGES_PAT }}   # NEW (Phase 52)
```

The job-level `permissions:` block does NOT need changes — `PACKAGES_PAT` is a separate token whose authority lives entirely in the destination repo (`serena-packages`), not in `GITHUB_TOKEN`'s scope set.

### Pattern: `release-verify.yml` skeleton

```yaml
# .github/workflows/release-verify.yml — NEW (Phase 52, D-06)
name: release-verify

on:
  release:
    types: [published]

# Read-only on this repo; we just open issues on failure.
permissions:
  contents: read
  issues: write

jobs:
  brew-macos:
    runs-on: macos-latest
    timeout-minutes: 10
    steps:
      - name: brew tap + install + version assert
        env:
          TAG: ${{ github.event.release.tag_name }}
        run: |
          set -euo pipefail
          brew tap postfix/serena-packages https://github.com/postfix/serena-packages
          brew install serena
          OUT=$(serena --version)
          echo "$OUT"
          # TAG starts with v; strip leading v for {{.Version}} match.
          VERSION="${TAG#v}"
          echo "$OUT" | grep -F "$VERSION"

  scoop-windows:
    runs-on: windows-latest
    timeout-minutes: 10
    steps:
      - name: install scoop
        shell: pwsh
        run: |
          Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
          irm get.scoop.sh -outfile 'install.ps1'
          .\install.ps1 -RunAsAdmin
      - name: scoop bucket add + install + version assert
        shell: pwsh
        env:
          TAG: ${{ github.event.release.tag_name }}
        run: |
          scoop bucket add serena https://github.com/postfix/serena-packages
          scoop install serena
          $out = serena --version
          Write-Host $out
          $version = $env:TAG.TrimStart('v')
          if ($out -notmatch [regex]::Escape($version)) {
            throw "version mismatch: expected $version in '$out'"
          }

  deb-ubuntu:
    runs-on: ubuntu-latest
    container: ubuntu:24.04
    timeout-minutes: 10
    steps:
      - name: install + version assert
        env:
          TAG: ${{ github.event.release.tag_name }}
        run: |
          set -euo pipefail
          apt-get update && apt-get install -y curl
          VERSION="${TAG#v}"
          curl -LO "https://github.com/postfix/serena/releases/download/${TAG}/serena_${VERSION}_amd64.deb"
          dpkg -i "serena_${VERSION}_amd64.deb"
          serena --version | grep -F "$VERSION"

  rpm-rocky:
    runs-on: ubuntu-latest
    container: rockylinux:9
    timeout-minutes: 10
    steps:
      - name: install + version assert
        env:
          TAG: ${{ github.event.release.tag_name }}
        run: |
          set -euo pipefail
          dnf install -y curl
          VERSION="${TAG#v}"
          curl -LO "https://github.com/postfix/serena/releases/download/${TAG}/serena_${VERSION}_x86_64.rpm"
          rpm -i "serena_${VERSION}_x86_64.rpm"
          serena --version | grep -F "$VERSION"

  open-issue-on-failure:
    needs: [brew-macos, scoop-windows, deb-ubuntu, rpm-rocky]
    if: failure()
    runs-on: ubuntu-latest
    steps:
      - uses: actions/github-script@v7
        with:
          script: |
            const tag = context.payload.release.tag_name;
            await github.rest.issues.create({
              owner: context.repo.owner,
              repo: context.repo.repo,
              title: `release-verify failed for ${tag}`,
              body: `One or more install smoke tests failed for release ${tag}. See workflow run: ${context.serverUrl}/${context.repo.owner}/${context.repo.repo}/actions/runs/${context.runId}`,
              labels: ['release-verify-failure']
            });
```

**Important — file name templating:** the deb/rpm file name templates above (`serena_${VERSION}_amd64.deb`, `serena_${VERSION}_x86_64.rpm`) match goreleaser's nfpms default `file_name_template`. Verify the exact pattern against goreleaser's emitted artifacts list (snapshot run) before locking these strings in `release-verify.yml`. Also note: rpm uses `x86_64` arch convention; deb uses `amd64`. See Pitfall 4.

### Anti-Patterns to Avoid

- **Putting `PACKAGES_PAT` at workflow-level `env:`** instead of step-level — leaks the secret into every step's environment. Keep it on the goreleaser step only.
- **Granting `PACKAGES_PAT` write access to anything other than `serena-packages`** — defeats the least-privilege intent of D-04. Use a fine-grained PAT with repository selection limited to that single repo.
- **Using `homebrew_casks:` instead of `brews:`** — would break Linuxbrew users (success criteria #1 explicitly requires Linux amd64). See Q1.
- **Adding `dependencies:` to `nfpms:`** — Serena is a static musl binary; declaring deps would make rpm/deb refuse to install on minimal containers and confuse users.
- **Auto-publishing on pre-release tags (e.g. `v1.9.0-rc1`)** — pre-release should NOT update the brew Formula or Scoop manifest because users `brew install serena` expects stable. Use `skip_upload: auto` on `brews:` and `scoops:` (skips when tag is detected as pre-release per Phase 51 D-10's `release.prerelease: auto`).
- **Forgetting to set Phase 51's `release.mode: replace` interaction** — re-running the release on the same tag should re-push the Formula. Goreleaser's `brews:`/`scoops:` overwrite the Formula file on each run; this is correct, but the `commit_msg_template` will produce a no-op commit if nothing changed (harmless).
- **Hard-coding the goreleaser bot identity vs. an alias** — using `goreleaserbot` as commit author makes the audit trail explicit. Keep it; do not impersonate the maintainer.
- **Running release-verify.yml on `push: tags`** instead of `release: published` — the tag fires before goreleaser uploads finish; verify will race against the upload. Use `release: published`.
- **Skipping the scoop installer step on windows-latest** — scoop is NOT preinstalled on GitHub Windows runners; you must install it. (Documented in `release-verify.yml` skeleton above.)

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Homebrew Formula Ruby file | hand-written `.rb` checked into a tap repo + manual update on each release | goreleaser `brews:` with template generation | goreleaser computes SHA256, fills in URLs, handles cross-platform `on_macos`/`on_linux`/`on_intel`/`on_arm` blocks atomically |
| Scoop manifest JSON | hand-written `.json` | goreleaser `scoops:` | goreleaser computes hashes, fills in URLs, picks the correct windows artifact |
| deb/rpm packages | bash scripts invoking `dpkg-deb`/`rpmbuild` | goreleaser `nfpms:` (embeds nFPM) | nFPM produces both formats from one config; no build hosts needed |
| Cross-repo git push from CI | `git clone … && git commit … && git push …` shell | goreleaser `repository:` block with `token:` | goreleaser handles auth, retries, and conflict resolution |
| Release smoke verification | manual checklist in RELEASING.md | `release-verify.yml` matrix on `release: published` | D-06 picked CI smoke as the gate; manual is documented as fallback only if CI is unavailable |
| Issue creation on workflow failure | webhook + custom service | `actions/github-script` `github.rest.issues.create` | Standard pattern; no extra infra |
| Auto-update polling on user side | custom in-binary updater | Native package managers (brew/scoop/dpkg) | OUT OF SCOPE per REQUIREMENTS.md "Auto-update mechanism inside the binary" |

**Key insight:** Every channel's auto-update mechanism is **already built into the user's package manager** (`brew upgrade`, `scoop update`). The only "auto-update" work for us is publishing the Formula/manifest atomically on each release tag. The deb/rpm path explicitly does NOT auto-update by design (D-03) — users redownload, this is documented, and PKG-DEFER-01 tracks the future hosted-repo enhancement.

## Common Pitfalls

### Pitfall 1: `brews:` deprecated in v2.10 — but `homebrew_casks:` does not work on Linux

**What goes wrong:** A reader sees the v2.10 deprecation notice and migrates to `homebrew_casks:` blindly. Linux users (`brew install serena` on Linuxbrew) get "no formulae match" because casks are macOS-only.

**Why it happens:** goreleaser's deprecation announcement focuses on the macOS UX (signing, notarization, .app bundles) and does not discuss Linuxbrew compatibility. CONTEXT.md Phase Boundary item 1 explicitly requires Linux amd64.

**How to avoid:** Stay on `brews:` for this phase. Add a code comment and a RELEASING.md note explaining the deferred migration: "Revisit when goreleaser v3 ships and Linuxbrew compatibility for casks is documented (or when we drop Linuxbrew support, whichever comes first)."

**Warning signs:**
- `release-verify.yml` ubuntu leg fails with "Error: Formula not found" after switching to casks.
- Linuxbrew users report `brew install` failures via GitHub issues.

**Sources:** [goreleaser v2.10 announcement](https://goreleaser.com/blog/goreleaser-v2.10/); [goreleaser deprecations page](https://goreleaser.com/deprecations/) [VERIFIED]

### Pitfall 2: Scoop expects `.zip` archives only on Windows

**What goes wrong:** If the `archives:` block emits `.tar.gz` for windows by mistake, `scoops:` either picks the wrong artifact or generates a manifest pointing to a `.tar.gz` Scoop can't extract natively.

**Why it happens:** The `format_overrides` block in Phase 51 already handles this (`goos: windows → formats: [zip]`), but a future edit could regress it.

**How to avoid:** Keep the `archives.format_overrides` `goos: windows → formats: [zip]` rule. Add a `goreleaser check` Wave-0 task to validate the augmented config.

**Warning signs:** `release-verify.yml` windows leg fails with "Couldn't find a suitable archiver" or scoop reports "no candidate archive".

### Pitfall 3: musl static binary inside .deb/.rpm — declaring glibc dependency

**What goes wrong:** A maintainer adds `dependencies: [libc6]` to `nfpms:` thinking "it's a Linux binary, of course it needs libc". The package then refuses to install on Alpine derivatives, on minimal containers, and on systems with non-standard libc.

**Why it happens:** Phase 51 builds Linux binaries with `zig cc -target x86_64-linux-musl` (CC override) — they are statically linked against musl, NOT dynamically against glibc. They have NO runtime libc dependency at all.

**How to avoid:** Leave `dependencies:` empty (omit the field). Document in RELEASING.md: "Serena's Linux binaries are static (musl). They run on glibc and musl systems unchanged."

**Warning signs:** User bug reports of "missing libc6 dependency" on glibc systems (the binary is fine — it doesn't link libc dynamically).

**Sources:** [Phase 51 .goreleaser.yml CC override comment block](file:.goreleaser.yml) — explicitly maps linux/* to `zig cc -target *-linux-musl` [VERIFIED in this repo]

### Pitfall 4: rpm vs deb arch naming (`x86_64` vs `amd64`)

**What goes wrong:** The hand-written download URL in INSTALL.md or `release-verify.yml` uses the wrong arch token for the format. `serena_1.9.0_amd64.rpm` doesn't exist (rpm uses `x86_64`); `serena_1.9.0_x86_64.deb` doesn't exist (deb uses `amd64`).

**Why it happens:** goreleaser's nfpms default file name template uses the format-native arch convention. Maintainers who memorize one convention forget the other.

**How to avoid:**
- After the first snapshot run, capture the actual emitted file names from `dist/`.
- Use those names verbatim in INSTALL.md and `release-verify.yml`.
- Codify in RELEASING.md: "deb uses `amd64`/`arm64`; rpm uses `x86_64`/`aarch64`."

**Warning signs:** `release-verify.yml` fails with `404 Not Found` from the GitHub Releases CDN.

### Pitfall 5: `release.prerelease: auto` interaction with `brews:` / `scoops:`

**What goes wrong:** Tagging `v1.9.0-rc1` triggers an UPDATE to the Formula and Scoop manifest, pinning users on `brew install serena` to a release candidate.

**Why it happens:** Without explicit `skip_upload: auto`, goreleaser pushes the formula/manifest on every release including pre-releases.

**How to avoid:** Add `skip_upload: auto` to both `brews:` and `scoops:` entries. This honors Phase 51's `release.prerelease: auto` detection (D-10) and skips the formula/manifest push for any tag matching `-rc`/`-beta`/`-alpha`.

```yaml
brews:
  - name: serena
    skip_upload: auto   # don't push formula on pre-release tags
    # ... rest as above
scoops:
  - name: serena
    skip_upload: auto
    # ... rest as above
```

**nfpms behavior:** deb/rpm artifacts are still attached to the GitHub Release for pre-release tags (no `skip_upload: auto` on `nfpms:` is needed; the artifacts are scoped to that specific Release). This is the intended behavior — pre-release users who want to test early can still grab the deb/rpm from the pre-release page.

**Source:** [goreleaser brews skip_upload docs](https://goreleaser.com/customization/homebrew/) [VERIFIED]

### Pitfall 6: Fine-grained PAT expiration without rotation alarm

**What goes wrong:** Fine-grained PATs MUST have an expiration (max 1 year as of GitHub's 2024 enforcement). The PAT silently expires; the next release tag triggers `403 Bad credentials` from goreleaser; the formula/manifest doesn't update; the smoke test catches it (D-06 issue creation).

**Why it happens:** No calendar reminder; the maintainer minted the PAT 11 months ago and forgot.

**How to avoid:**
- Document rotation in RELEASING.md with the exact UI path: "Settings → Developer settings → Personal access tokens → Fine-grained tokens → `serena-packages-publish` → Regenerate".
- Set the PAT expiration to something less than the maximum (e.g. 90 days) and add a recurring calendar reminder in the maintainer's personal calendar (out-of-band; we don't manage the maintainer's calendar).
- The `release-verify.yml` issue-on-failure (D-06) catches an expired PAT case the FIRST time it bites — in that sense the smoke test is also a PAT-expiry alarm.

**Warning signs:** `goreleaser` step in `release.yml` fails with `403` when pushing to `serena-packages`; release-verify smoke test fails with "Formula not found" / "manifest not found" because the publish never happened.

**Sources:** [GitHub fine-grained PAT docs](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) [CITED]

### Pitfall 7: First-run race — `serena-packages` repo missing or empty default branch

**What goes wrong:** Maintainer creates the `serena-packages` repo but forgets the initial commit; default branch doesn't exist; goreleaser fails to push because `main` doesn't resolve.

**Why it happens:** GitHub creates a repo without a default branch until the first commit is pushed.

**How to avoid:** RELEASING.md bootstrap step explicitly: "Create the repo WITH `Initialize this repository with a README` checked, OR `git push` an initial empty commit to `main` before tagging the first Phase-52-aware release."

**Warning signs:** First post-Phase-52 release fails with `couldn't find remote ref refs/heads/main`.

## Validation Architecture

> Phase requires Nyquist validation (workflow.nyquist_validation enabled in config.json). Phase requirements PKG-02, PKG-03, PKG-04.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Mixed: `goreleaser check` for static config; `goreleaser release --snapshot --clean` for local pipeline; `release-verify.yml` for live install smoke; existing Go test suite for `serena --version` ldflags |
| Config file | `.goreleaser.yml` (validated by `goreleaser check`); `.github/workflows/release.yml` and `release-verify.yml` (validated by GitHub Actions itself on push) |
| Quick run command | `goreleaser check` (< 1s, schema validation of augmented config) |
| Full suite command | `goreleaser release --snapshot --clean` (~30-90s; produces deb/rpm into `dist/`, generates Formula/manifest LOCALLY into `dist/homebrew/` and `dist/scoop/` for inspection) |
| Phase gate | All four success criteria in ROADMAP.md Phase 52 + `release-verify.yml` green on the first real tag after merge |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| PKG-02 | `.goreleaser.yml` brews block is schema-valid | tooling | `goreleaser check` | Wave 0 — needs new block in `.goreleaser.yml` |
| PKG-02 | Snapshot generates Formula Ruby locally | tooling | `goreleaser release --snapshot --clean && cat dist/homebrew/Formula/serena.rb` | — |
| PKG-02 | `brew install` works on macOS-latest after release | live smoke | `release-verify.yml` `brew-macos` job | Wave 0 — needs `release-verify.yml` |
| PKG-02 | `brew install` works on Linux amd64 after release | manual / out of CI | Documented in RELEASING.md ritual; CI runner does not include Linuxbrew | Manual |
| PKG-02 | Pre-release tag does NOT update Formula | tooling | `goreleaser release --snapshot --clean -p` then verify no Formula push attempt for snapshot | — |
| PKG-03 | `.goreleaser.yml` scoops block is schema-valid | tooling | `goreleaser check` | Wave 0 |
| PKG-03 | Snapshot generates Scoop manifest JSON locally | tooling | `goreleaser release --snapshot --clean && cat dist/scoop/bucket/serena.json` | — |
| PKG-03 | `scoop install` works on Windows after release | live smoke | `release-verify.yml` `scoop-windows` job | Wave 0 |
| PKG-04 | `.goreleaser.yml` nfpms block is schema-valid | tooling | `goreleaser check` | Wave 0 |
| PKG-04 | Snapshot produces `serena_*.deb` + `serena_*.rpm` | tooling | `goreleaser release --snapshot --clean && ls dist/serena_*.{deb,rpm}` | — |
| PKG-04 | `dpkg -i` works on ubuntu:24.04 after release | live smoke | `release-verify.yml` `deb-ubuntu` job | Wave 0 |
| PKG-04 | `rpm -i` works on rockylinux:9 after release | live smoke | `release-verify.yml` `rpm-rocky` job | Wave 0 |
| PKG-04 | INSTALL.md documents direct-download install | grep | `grep -E '(dpkg -i\|rpm -i)' INSTALL.md` | — |
| All | `serena --version` output contains the tag's version string post-install | live smoke (asserted by every release-verify.yml leg) | `serena --version \| grep "$VERSION"` | — |
| All | Pre-existing Go test suite still passes | unit | `go test ./... && go vet ./...` (CLAUDE.md mandate) | — |

### Sampling Rate

- **Per task commit:** `goreleaser check` (< 1s) — should pass at every diff that touches `.goreleaser.yml`.
- **Per wave merge:** `goreleaser release --snapshot --clean` — confirms full local pipeline including deb/rpm/Formula/manifest generation. Signing fails locally without OIDC; that's expected.
- **Phase gate:** All Phase 52 ROADMAP success criteria + first real tag's `release-verify.yml` green + `serena-packages` repo contains both `Formula/serena.rb` and `bucket/serena.json` reflecting that tag. `go test ./...` and `go vet ./...` pass (CLAUDE.md mandate, even though Phase 52 changes are mostly YAML/MD — the Makefile `build-versioned` target from Phase 51 must still work).
- **Post-merge ritual (out of PR):** Maintainer creates first real release tag (e.g., `v1.9.0-rc2`) on a fork or in a controlled branch; observes goreleaser publishes the artifacts, observes release-verify.yml runs and passes, manually inspects the `serena-packages` repo's commits to confirm the formula and manifest landed.

### Wave 0 Gaps

- [ ] `.goreleaser.yml` — append `brews:`, `scoops:`, `nfpms:` blocks. Net new in this file (Phase 51 did not include them).
- [ ] `.github/workflows/release.yml` — add `PACKAGES_PAT` env var to the goreleaser step. Edit existing file; one line added under `env:`.
- [ ] `.github/workflows/release-verify.yml` — net new file.
- [ ] `INSTALL.md` — net new sections: `## Install via Homebrew`, `## Install via Scoop`, `## Install via deb (Debian/Ubuntu)`, `## Install via rpm (Fedora/RHEL)`. Extend existing structure from Phase 51.
- [ ] `RELEASING.md` — net new sections: `## One-time bootstrap of serena-packages` (create repo + mint PAT + set secret); `## Rotating PACKAGES_PAT`; extend the existing reproducibility diff exclude list to cover deb/rpm artifacts (`*.deb`, `*.rpm` are reproducible if `mod_timestamp` carries through, but the build timestamp inside the package itself may not be — exclude defensively or document carefully).
- [ ] No code changes to `cmd/serena/`, `internal/`, or `Makefile` are expected. The phase is YAML + workflow + docs only.

## Reference Exemplars

### 1. `goreleaser/homebrew-tap` — goreleaser's own brew tap

GoReleaser publishes itself via the `brews:` block to `goreleaser/homebrew-tap`. Its `.goreleaser.yaml` is a canonical reference for the `brews:` block structure used with the still-supported (deprecated-but-functional) v2 form. Cross-platform `on_macos`/`on_linux`/`on_intel`/`on_arm` is auto-generated by goreleaser; the maintainer does not write any Ruby beyond the `install` and `test` blocks.

### 2. Scoop bucket — `ScoopInstaller/Main`

The Scoop docs ([scoop.sh](https://scoop.sh)) document `scoop bucket add <name> <url>` for custom buckets. Custom buckets host JSON manifests at the repo root (or in a subdirectory specified by the goreleaser `scoops.directory` field; we use `bucket`).

### 3. nFPM-published Go projects

`grafana/loki`, `caddyserver/caddy`, and many other Go projects ship deb/rpm via goreleaser nfpms. The pattern of `formats: [deb, rpm]`, single binary in `bindir: /usr/bin`, no scripts, no dependencies is the same across them.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `brews:` for Homebrew Formula generation | `homebrew_casks:` (v2.10+) for macOS-only delivery | goreleaser v2.10 (2024-2025 timeframe) | We DO NOT migrate yet — casks don't support Linuxbrew. Stay on `brews:` until v3 forces a decision. |
| `scoop:` (singular) | `scoops:` (plural) | goreleaser v2.0 schema migration | Use plural form; Phase 51 already uses v2 schema so this is consistent. |
| Hand-written .deb / .rpm via `dpkg-deb`/`rpmbuild` | goreleaser nfpms (embedded nFPM library) | Stable since goreleaser v0.x; no recent change | Use nfpms; do not invoke nFPM directly. |
| Classic PAT with broad `repo` scope | Fine-grained PAT scoped to single repo | GitHub fine-grained PAT GA (2022) + 1-year expiration enforcement (2024) | D-04 picks fine-grained; rotation is now part of RELEASING.md ritual. |
| PR-based formula/manifest update | Direct push to default branch (D-05) | N/A — both are valid; user choice | Direct push trades manual review for speed; mitigation = roll forward via patch tag. |

**Deprecated/outdated patterns to avoid:**

- `brews:` deprecation notice in goreleaser v2.10 — ACKNOWLEDGED, intentionally kept due to Linuxbrew compatibility (Pitfall 1).
- `archives.format_overrides[].format` (singular) — already handled by Phase 51 (uses plural `formats:`).
- `scoop:` (singular) — never used in Phase 51, no migration needed.
- `goreleaser-action@v6` vs `@v7` — Phase 51 D-11 picked v6.4.0; stay on v6 for Phase 52 (no v7 migration in this phase).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `peter-evans/create-issue-from-file` or `actions/github-script` is acceptable for issue creation in `release-verify.yml` | Standard Stack | Low — easily swappable; both are widely used. Plan should pin a specific version and SHA at implementation time. |
| A2 | Linuxbrew users are present in Serena's user base today | Pitfall 1 / Q1 | Low-MEDIUM — if no Linuxbrew users exist, the cask migration becomes safe and the deprecation note can be honored sooner. The CONTEXT.md success criteria explicitly mentions "Linux amd64" via brew, so we honor that contract. |
| A3 | rpm uses `x86_64`/`aarch64`, deb uses `amd64`/`arm64` in goreleaser's default nfpms file_name_template | Pitfall 4 / release-verify.yml skeleton | Low — verify by inspecting the first snapshot's `dist/` listing and updating the strings if needed. |
| A4 | `release: published` event fires AFTER goreleaser uploads all assets including auto-pushed Formula/manifest commits | release-verify.yml architecture | Low — `release: published` fires when the GitHub Release transitions from draft to published, which goreleaser triggers at the end of its own run. The Formula/manifest pushes to `serena-packages` happen earlier in the goreleaser pipeline and are independent of the GitHub Release publish event. |
| A5 | musl-static binaries from `zig cc -target *-linux-musl` (Phase 51) install cleanly via deb/rpm onto glibc systems | Pitfall 3 / nfpms block | Low — musl statically linked binaries do not invoke the dynamic libc resolver; they run unmodified on any Linux kernel ≥ a small floor. Phase 51 already validated this for the raw archive download path; deb/rpm just bundles the same binary. |
| A6 | Apache-2.0 is the project's actual license (used in example `license:` fields above) | brews/scoops/nfpms blocks | Low — verify against repo `LICENSE` file at planning time and substitute the real SPDX identifier. |

## Open Questions

1. **Should we use `brews:` (deprecated v2.10, Linuxbrew compatible) or `homebrew_casks:` (current, macOS-only)?**
   - What we know: `brews:` works for both macOS and Linux Homebrew; `homebrew_casks:` works for macOS only. CONTEXT.md success criteria explicitly require Linux amd64 via brew. goreleaser v3 has no planned date.
   - What's unclear: how many Serena users (current or projected) actually use Linuxbrew, vs. those who would prefer the cleaner cask UX (which handles macOS Gatekeeper quarantine more gracefully, signing notwithstanding).
   - Recommendation: **Use `brews:`** for Phase 52. Add a code comment + RELEASING.md note pointing at goreleaser v3 as the trigger to revisit. Document the deprecation in the phase summary so it's discoverable next time someone touches the file. If user wants to drop Linuxbrew compatibility, this becomes a quick swap.

2. **License identifier — what is Serena's actual SPDX license?**
   - What we know: example values in this research use `Apache-2.0`. Phase 51's archive includes `LICENSE*` files (auto-glob).
   - What's unclear: actual SPDX ID — must verify against the repo's `LICENSE` file at planning time. Wrong SPDX in the Formula/manifest is a cosmetic bug but visible to users.
   - Recommendation: planner runs `head -3 LICENSE` and substitutes the correct SPDX identifier in all three blocks. Wave-0 task.

3. **deb/rpm reproducibility extension to RELEASING.md diff command.**
   - What we know: Phase 51 RELEASING.md has a snapshot-diff ritual. nfpms artifacts add timestamps inside the package metadata that may or may not be reproducible.
   - What's unclear: whether `mod_timestamp` carries through into the deb/rpm metadata. goreleaser's nfpms uses `--release-date` from the tag commit timestamp by default, so it SHOULD be reproducible — but I have not verified by snapshot diff.
   - Recommendation: planner runs `goreleaser release --snapshot --clean` twice locally, diffs the deb and rpm with `--exclude='*.sig' --exclude='*.pem'`, and either confirms reproducibility (no diff) or extends the exclude list. Document the result in the RELEASING.md additions.

4. **Should `release-verify.yml` also validate cosign signatures of the deb/rpm?**
   - What we know: D-03 says cosign verify is OPTIONAL for deb/rpm install. Phase 51's signs already cover the checksum file which transitively covers all archives + nfpms artifacts.
   - What's unclear: whether the smoke test should additionally exercise the verify path (catches signing infra regressions earlier).
   - Recommendation: keep release-verify.yml focused on install + version assertion (D-06's stated 90% failure mode). Don't pile in signature verification. If signing breaks, Phase 51's pipeline failure surfaces first; the smoke test is downstream.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `goreleaser` CLI | local snapshot validation, `goreleaser check` | TBD on dev machine — verify via `command -v goreleaser` | v2.x | `brew install goreleaser` (macOS) / `go install github.com/goreleaser/goreleaser/v2@latest` |
| `cosign` CLI | inherited from Phase 51 | should be present from Phase 51 | v2.x | `brew install cosign` |
| `dpkg` / `rpm` | local nfpms snapshot inspection (optional) | macOS dev: NO; Linux dev: usually yes | — | None — release-verify.yml is the gate; local inspection is optional. Maintainer can `tar tzf serena_*.deb` to inspect (deb is an ar archive of tar.gz). |
| `gh` CLI | local PAT minting / verification | should be present | latest | `brew install gh` |
| GitHub Actions runners (`macos-latest`, `windows-latest`, `ubuntu-latest`) | release-verify.yml | ✓ | — | — |
| `serena-packages` GitHub repo | brews+scoops push target | NOT YET CREATED — one-time bootstrap step in RELEASING.md | — | None — must exist before first Phase-52-enabled tag |
| `PACKAGES_PAT` GH Actions secret | brews+scoops push auth | NOT YET CREATED — one-time bootstrap step in RELEASING.md | — | None — must exist before first Phase-52-enabled tag |

**Missing dependencies with no fallback:**
- `serena-packages` repo and `PACKAGES_PAT` secret: BOTH require maintainer action before the first release tag after Phase 52 merges. RELEASING.md must call this out as a HARD prerequisite. The first plan should add a "pre-flight" item that ensures the maintainer has done the bootstrap before tagging.

**Missing dependencies with fallback:** N/A.

## Project Constraints (from CLAUDE.md)

- **Run `go vet ./...` and `go test ./...` before completing any Go task** — Phase 52 doesn't change Go code, but the Wave-0 / per-commit gate from Phase 51 must keep passing. No regressions from YAML/MD changes (none expected).
- **Use `go build ./cmd/serena` / `make build` to build** — unchanged. Phase 52 doesn't alter build commands.
- **GSD workflow enforced** — all file edits via plan execution.
- **Single Go binary, no Python/Docker/runtime deps in shipping artifact** — Phase 52 honors this: deb/rpm packages contain only the static binary; no scripts, no runtime deps declared in nfpms `dependencies:`.
- **Legacy reference is read-only** — Phase 52 does not touch `legacy/`.

## Sources

### Primary (HIGH confidence)

- [goreleaser docs — Homebrew (brews / casks)](https://goreleaser.com/customization/homebrew/) — `brews:` and `homebrew_casks:` schema [VERIFIED via WebFetch]
- [goreleaser docs — Scoop](https://goreleaser.com/customization/scoop/) — `scoops:` schema, plural form, repository.token [VERIFIED via WebFetch]
- [goreleaser docs — nFPM](https://goreleaser.com/customization/nfpm/) — formats, bindir, contents, scripts, format-specific overrides [VERIFIED via WebFetch]
- [goreleaser blog — Announcing GoReleaser v2.10](https://goreleaser.com/blog/goreleaser-v2.10/) — `brews:` deprecation announcement, cask migration guidance, v3 timeline [VERIFIED via WebFetch]
- [goreleaser deprecations page](https://goreleaser.com/deprecations/) — canonical deprecation list [CITED]
- [Scoop documentation — Custom buckets](https://scoop.sh) — `scoop bucket add <name> <url>` [CITED]
- Phase 51 `.goreleaser.yml` and `release.yml` (this repo) — locked baseline that Phase 52 extends [VERIFIED in repo]
- Phase 51 `51-RESEARCH.md` (this repo) — goreleaser v2 schema details, signing patterns, reproducibility tradeoffs [VERIFIED in repo]

### Secondary (MEDIUM confidence)

- [Bindplane — Creating Homebrew Formulas with GoReleaser](https://bindplane.com/blog/creating-homebrew-formulas-with-goreleaser) — practical brews: examples
- [DEV — Distribute your Go CLI tools with GoReleaser and Homebrew](https://dev.to/40percentironman/distribute-your-go-cli-tools-with-goreleaser-and-homebrew-4jd8) — Go CLI specific brews: walkthrough
- [GitHub fine-grained PAT documentation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) — fine-grained PAT scope model and expiration

### Tertiary (LOW confidence — flag for validation)

- Exact rpm `x86_64`/deb `amd64` arch token in goreleaser's default `file_name_template` for nfpms — verified through reading docs but not by snapshot inspection in this research session. Wave-0 should snapshot-and-list before locking download URLs in INSTALL.md and `release-verify.yml`. (Pitfall 4 / Assumption A3.)
- Whether `release: published` strictly fires after Formula/manifest pushes complete — strong inference from goreleaser pipeline order, but not verified end-to-end. (Assumption A4.)

## Metadata

**Confidence breakdown:**

- Standard stack (brews / scoops / nfpms blocks): HIGH — verified directly from goreleaser docs.
- Architecture (workflow wiring, environment, secret model): HIGH — patterns are well-documented; PACKAGES_PAT model is canonical for cross-repo publishes.
- brews vs casks decision: MEDIUM-HIGH — clear technical answer (casks are macOS-only) but rests on Assumption A2 (Linuxbrew users exist or are anticipated).
- Pitfalls: HIGH — six of seven verified directly against goreleaser docs and the repo's own Phase 51 artifacts; Pitfall 4 (rpm/deb arch naming) is convention-based and worth verifying via snapshot.
- release-verify.yml skeleton: MEDIUM-HIGH — patterns are standard but exact runner image tags, container versions, and arch token strings are best confirmed by snapshot before locking.

**Research date:** 2026-04-26
**Valid until:** 2026-05-26 (30 days; goreleaser v2 schema stable, brews deprecation timeline aligned with v3 (no date), no anticipated breaking changes in Scoop or nfpms surfaces).
