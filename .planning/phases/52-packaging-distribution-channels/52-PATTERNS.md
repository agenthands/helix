# Phase 52: packaging-distribution-channels - Pattern Map

**Mapped:** 2026-04-26
**Files analyzed:** 4 (1 net new + 3 modified)
**Analogs found:** 4 / 4

All four target files have direct analogs already in the repo from Phase 51's merged work. This phase is purely additive: append new top-level blocks, extend existing doc sections, add one sibling workflow file. No file is being rewritten.

## File Classification

| File | Status | Role | Data Flow | Closest Analog | Match Quality |
|------|--------|------|-----------|----------------|---------------|
| `.goreleaser.yml` | modified (append blocks) | config (release pipeline) | batch / build-publish | self (Phase 51 baseline) — same file, append-only | exact (self-extension) |
| `.github/workflows/release.yml` | modified (one env line) | config (CI workflow) | event-driven (`push: tags`) | self (Phase 51 baseline) | exact (self-extension) |
| `.github/workflows/release-verify.yml` | created | config (CI workflow) | event-driven (`release: published`) | `.github/workflows/release.yml` (Phase 51) | role-match — same workflow shape, different trigger + matrix |
| `INSTALL.md` | modified (4 new sections appended) | docs (user-facing install guide) | request-response (reader → install command) | INSTALL.md `## Download a prebuilt binary` and `## Verify a release binary` sections (Phase 51) | exact (extend existing structure) |
| `RELEASING.md` | modified (2 new sections appended) | docs (maintainer ritual) | request-response (maintainer → release ritual) | RELEASING.md `## Prerequisites`, `## Pre-tag reproducibility ritual`, `## Post-merge verification` sections (Phase 51) | exact (extend existing structure) |

## Pattern Assignments

### `.goreleaser.yml` (config, batch / build-publish) — APPEND ONLY

**Analog:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/.goreleaser.yml` (Phase 51 baseline; same file)

**Header pattern to preserve at top of file** (lines 1-6):
```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema.json
# Phase 51 — packaging-goreleaser. See .planning/phases/51-packaging-goreleaser/.
# Locked decisions: D-02 (sign archives + checksum), D-10 (prerelease auto),
# D-11 (v2 schema, repo-root path), D-12 (6-binary matrix), D-13 (build flags),
# D-14 (archive layout).
version: 2
```

**Schema version pin** (line 6): `version: 2` — `brews:`, `scoops:`, `nfpms:` are v2 schema names (plural). Do NOT introduce singular `scoop:` (v1).

**Existing block boundaries — DO NOT MODIFY:**
- `before:` lines 32-34
- `builds:` lines 36-90 (6-binary matrix with zig CC overrides)
- `gomod:` lines 92-93
- `checksum:` lines 95-97
- `archives:` lines 99-112 (id is `archive` — referenced by new `brews.ids` and `scoops.ids`)
- `signs:` lines 117-141 (two entries — checksum + archive; covers nfpms transitively via `checksums.txt`)
- `release:` lines 143-146 (`prerelease: auto`, `mode: replace`)

**Append pattern (place AFTER `release:` at end of file):**

The new blocks are top-level sibling keys to `builds:` / `archives:` / `signs:` / `release:`. Append them in this order to mirror the typical goreleaser-published example projects:

1. `nfpms:` (deb/rpm) — no external repo dependency, lowest risk
2. `brews:` (Homebrew Formula) — pushes to `serena-packages`
3. `scoops:` (Scoop manifest) — pushes to `serena-packages`

**Comment-block prefix style** to match Phase 51's authorial voice (see lines 10-31, 114-116, 49-50 of `.goreleaser.yml`): every new block must carry a leading comment naming the requirement (PKG-02 / PKG-03 / PKG-04), the relevant decision IDs (D-01..D-05 from 52-CONTEXT.md), and a one-line rationale for non-obvious fields.

**Concrete excerpt to copy from for `ids:` cross-reference** (lines 99-101):
```yaml
archives:
  - id: archive
    formats: [tar.gz]
```
The new `brews:` and `scoops:` entries MUST set `ids: [archive]` so they pick up Phase 51's archive entry. Do not invent a new archive id.

**Cross-reference for `release.prerelease: auto` interaction** (lines 143-145):
```yaml
release:
  prerelease: auto
```
This is what makes `skip_upload: auto` work on `brews:` / `scoops:` (Pitfall 5 in 52-RESEARCH.md). Both new entries MUST include `skip_upload: auto`.

**License field value:** the repo's `LICENSE` file is `MIT License` (verified via `head -3 LICENSE`). Use `license: "MIT"` in `brews:`, `scoops:`, and `nfpms:` — NOT `Apache-2.0` (the research doc used Apache-2.0 as a placeholder; correct it at planning/implementation time).

---

### `.github/workflows/release.yml` (config, event-driven) — ONE LINE ADDED

**Analog:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/.github/workflows/release.yml` (Phase 51 baseline; same file)

**Existing structure — DO NOT MODIFY:**
- Trigger `on: push: tags: ['v*']` (lines 6-9)
- Job-level `permissions:` (lines 20-22) — `contents: write` + `id-token: write`. Do NOT add `packages: write` or alter scope.
- Step order: checkout (full history) → setup-go 1.25.x → setup-zig 0.13.0 → cosign-installer → goreleaser-action

**Patch target** — the goreleaser step's `env:` block (lines 49-56):
```yaml
- name: Run goreleaser
  uses: goreleaser/goreleaser-action@v6.4.0
  with:
    distribution: goreleaser
    version: '~> v2'
    args: release --clean
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Append `PACKAGES_PAT` to that step's `env:` map (NOT to workflow-level env — see Anti-Patterns in 52-RESEARCH.md):**
```yaml
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    PACKAGES_PAT: ${{ secrets.PACKAGES_PAT }}   # Phase 52 (D-04): brews/scoops push to postfix/serena-packages
```

**Comment style** to match Phase 51 (lines 1-3, 17-22, 27-29, 38-40): each non-obvious line gets a trailing or preceding comment naming the decision ID. Reuse `# Phase 52 (D-XX): ...` form.

---

### `.github/workflows/release-verify.yml` (config, event-driven) — NET NEW

**Analog:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/.github/workflows/release.yml` (Phase 51) — closest workflow file in the repo by role; reuse its header style, comment voice, action-pinning convention.

**Header / metadata pattern to copy** (release.yml lines 1-4):
```yaml
# Phase 51 — release pipeline. See .planning/phases/51-packaging-goreleaser/.
# Locked decisions: D-09 (tag pattern v*), D-17 (permissions), D-18 (ubuntu-latest, Go 1.25.x).
# Pinned action versions per D-11; bump intentionally, not auto.
name: release
```
Apply analogous opener to `release-verify.yml`:
```yaml
# Phase 52 — release verification smoke test. See .planning/phases/52-packaging-distribution-channels/.
# Locked decisions: D-06 (CI smoke is the gate), D-01 (tap = postfix/serena-packages).
# Pinned action versions; bump intentionally, not auto.
name: release-verify
```

**Trigger pattern** — DO NOT copy `release.yml`'s `push: tags`; use `release: published` instead (per 52-RESEARCH.md Anti-Patterns: "Running release-verify.yml on `push: tags` instead of `release: published`"):
```yaml
on:
  release:
    types: [published]
```

**Permissions pattern to copy and narrow** (release.yml lines 18-22):
```yaml
# Source — release.yml job-level permissions
permissions:
  contents: write
  id-token: write
```
Narrow for verification: `contents: read` + `issues: write` (only writes are GitHub issues on smoke failure):
```yaml
permissions:
  contents: read
  issues: write
```

**Action pinning convention** — release.yml pins exact versions (`actions/checkout@v4`, `actions/setup-go@v5`, `mlugg/setup-zig@v1`, `sigstore/cosign-installer@v3.7.0`, `goreleaser/goreleaser-action@v6.4.0`). Match this style; pin `actions/github-script` to a specific tag (e.g., `@v7`).

**Job structure pattern** — release.yml uses one `release:` job with `runs-on: ubuntu-latest` and `timeout-minutes: 30` (lines 12-14). For release-verify.yml, use a matrix-equivalent: one job per channel (brew-macos, scoop-windows, deb-ubuntu, rpm-rocky) plus a final `open-issue-on-failure` job with `if: failure()` and `needs:` listing all four. Use shorter `timeout-minutes: 10` per leg (smoke tests are fast or hung).

**Skeleton to use as starting point:** see 52-RESEARCH.md "Pattern: `release-verify.yml` skeleton" (lines 370-473 of 52-RESEARCH.md). That skeleton already encodes:
- The four matrix legs with correct shells (`bash` default for *nix, `pwsh` for Windows)
- Version-string assertion against `${TAG#v}` (Linux/macOS) and `$env:TAG.TrimStart('v')` (Windows)
- The `actions/github-script@v7` issue-creation block
- Container images `ubuntu:24.04` for deb leg and `rockylinux:9` for rpm leg

**Arch-token convention to encode** (52-RESEARCH.md Pitfall 4): deb file name uses `amd64`/`arm64`; rpm file name uses `x86_64`/`aarch64`. The download URLs in the deb-ubuntu and rpm-rocky steps MUST use the format-native arch token. Verify exact strings against `goreleaser release --snapshot --clean` output before locking the workflow.

---

### `INSTALL.md` (docs, request-response) — APPEND 4 SECTIONS

**Analog:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/INSTALL.md` (Phase 51; same file)

**Existing section ordering — DO NOT REORDER:**
1. `## Prerequisites` (lines 5-29) — `go install` + build from source
2. `## Download a prebuilt binary` (lines 31-51) — env-var-driven curl/tar one-liner
3. `## Verify a release binary` (lines 53-85) — cosign + sha256
4. `## Quick Start` (lines 87-109)
5. `## Manual Configuration` (lines 111-274)
6. `## HTTP Mode` (lines 276-284)
7. `## Verify Installation` (lines 286-293)
8. `## Next Steps` (lines 295-298)
9. `## Legacy Python` (lines 300-302)

**Insertion point:** New sections go AFTER `## Verify a release binary` (line 85) and BEFORE `## Quick Start` (line 87). The reading flow is: Prerequisites → Download → Verify → **[NEW: Homebrew → Scoop → deb → rpm]** → Quick Start. Reason: package-manager paths are alternate ways to obtain the binary; they belong adjacent to the manual download path.

**Heading-style pattern to copy** (line 31, line 53):
```markdown
## Download a prebuilt binary
```
```markdown
## Verify a release binary
```
Apply same `## <verb> via <channel>` form for new sections:
```markdown
## Install via Homebrew (macOS, Linux)
## Install via Scoop (Windows)
## Install via .deb (Debian, Ubuntu)
## Install via .rpm (Fedora, RHEL, openSUSE)
```

**Env-var-driven snippet pattern to copy** (lines 37-47):
```bash
VERSION=v1.9.0   # set to the release tag you want
OS=linux         # one of: linux, darwin, windows
ARCH=amd64       # one of: amd64, arm64

curl -LO "https://github.com/postfix/serena/releases/download/${VERSION}/serena_${OS}_${ARCH}.tar.gz"
tar -xzf "serena_${OS}_${ARCH}.tar.gz"
chmod +x "serena_${OS}_${ARCH}/serena"
sudo mv "serena_${OS}_${ARCH}/serena" /usr/local/bin/serena
serena --version
```
Apply same `VERSION=...; ARCH=...; curl -LO ...` shape for the deb and rpm sections. Do NOT switch to inline tags — the env-var preface is the established style.

**Brew + Scoop sections** are simpler (no curl/checksum dance — the package manager handles it):
```bash
# Homebrew (per D-01 install command)
brew tap postfix/serena-packages https://github.com/postfix/serena-packages
brew install serena
serena --version
```
```powershell
# Scoop
scoop bucket add serena https://github.com/postfix/serena-packages
scoop install serena
serena --version
```

**Optional cosign verify cross-reference** — the deb/rpm sections should link back to `## Verify a release binary` (D-03: cosign verify is documented as optional alongside install) using anchor `#verify-a-release-binary`. Reuse the cosign command body verbatim (lines 71-77) — change only `${ARTIFACT}` to `serena_${VERSION}_amd64.deb` / `serena_${VERSION}_x86_64.rpm`.

---

### `RELEASING.md` (docs, request-response) — APPEND 2 SECTIONS

**Analog:** `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/RELEASING.md` (Phase 51; same file)

**Existing section ordering — DO NOT REORDER:**
1. `## Prerequisites` (lines 7-25) — install goreleaser + cosign
2. `## Pre-tag reproducibility ritual (D-07)` (lines 27-72)
3. `## Tag and push` (lines 74-90)
4. `## Post-merge verification (D-22)` (lines 92-106)
5. `## Out of scope` (lines 108-116) — currently lists Homebrew/Scoop/Linux native as deferred to Phase 52 (line 112)

**Insertion points:**
- Insert **`## One-time bootstrap of serena-packages`** AFTER `## Prerequisites` (line 25) and BEFORE `## Pre-tag reproducibility ritual` (line 27). Reason: bootstrap is a one-time prerequisite, parallel in role to "install goreleaser + cosign".
- Insert **`## Rotating PACKAGES_PAT`** AFTER `## Post-merge verification (D-22)` (line 106) and BEFORE `## Out of scope` (line 108). Reason: rotation is an ongoing maintenance ritual; sits with the other operational rituals.
- **Edit `## Out of scope`** (lines 108-116): remove the Phase 52 deferral line ("Homebrew, Scoop, Linux native packages — Phase 52 (D-23)") since Phase 52 lands these. Leave Docker, SBOM, notarization, in-binary auto-update entries intact.
- **Extend `## Pre-tag reproducibility ritual`** diff exclude list (lines 46-53): add `--exclude='*.deb'` and `--exclude='*.rpm'` defensively per Open Question 3 in 52-RESEARCH.md (deb/rpm internal timestamps may not be byte-stable across snapshot runs). Document the rationale in the surrounding prose.

**Heading-style pattern to copy** (line 27, line 74, line 92):
```markdown
## Pre-tag reproducibility ritual (D-07)
## Tag and push
## Post-merge verification (D-22)
```
Apply same `## <gerund/imperative> ...` form with optional decision-ID suffix:
```markdown
## One-time bootstrap of serena-packages (D-01, D-04)
## Rotating PACKAGES_PAT (D-04)
```

**Block-style pattern to copy** for ritual instructions (lines 35-54): numbered or fenced shell blocks with "Expected:" callouts. Bootstrap section should follow the same pattern:
```markdown
## One-time bootstrap of serena-packages (D-01, D-04)

Before the first Phase-52-aware tag, the maintainer creates the publish target
and the PAT that authenticates pushes to it.

1. Create the empty repo (with an initial commit so `main` exists — see Pitfall 7):
   ```bash
   gh repo create postfix/serena-packages --public \
     --description "Homebrew tap and Scoop bucket for Serena" \
     --add-readme
   ```

2. Mint a fine-grained PAT scoped ONLY to `postfix/serena-packages` with
   `contents: write` (Settings → Developer settings → Personal access tokens →
   Fine-grained tokens → Generate new token).

3. Set the PAT as the `PACKAGES_PAT` secret on `postfix/serena`:
   ```bash
   gh secret set PACKAGES_PAT --repo postfix/serena
   ```
```

**"Why these exclusions" sub-section pattern** (lines 65-72) — use the same `### Why ...` H3 sub-header style if you need to justify the bootstrap choices.

**Cross-reference style** (line 72: `See .planning/phases/51-packaging-goreleaser/51-RESEARCH.md Q6`) — reuse for citing 52-RESEARCH.md pitfalls.

---

## Shared Patterns

### Schema version pin
**Source:** `.goreleaser.yml:6` (`version: 2`)
**Apply to:** `.goreleaser.yml` only (already set; preserve)
**Why:** brews/scoops/nfpms in this phase are all v2 schema. Singular `scoop:` is v1 and must NOT appear.
```yaml
version: 2
```

### Decision-ID-tagged comments
**Source:** `.goreleaser.yml:2-5` and `release.yml:1-3`
**Apply to:** all 4 modified/new files
**Why:** every Phase-51 file uses inline comments naming the decision ID. Phase 52 must keep this audit trail discoverable.
```yaml
# Phase 51 — packaging-goreleaser. See .planning/phases/51-packaging-goreleaser/.
# Locked decisions: D-02 (sign archives + checksum), D-10 (prerelease auto), ...
```
Phase 52 form:
```yaml
# Phase 52 — packaging-distribution-channels. See .planning/phases/52-packaging-distribution-channels/.
# Locked decisions: D-01 (single packages monorepo), D-04 (fine-grained PAT), ...
```

### Pinned action versions
**Source:** `release.yml:25, 33, 43, 47, 50` — `actions/checkout@v4`, `actions/setup-go@v5`, `mlugg/setup-zig@v1`, `sigstore/cosign-installer@v3.7.0`, `goreleaser/goreleaser-action@v6.4.0`
**Apply to:** `.github/workflows/release-verify.yml`
**Why:** Phase 51 D-11 mandates intentional version bumps. New workflow must follow.
```yaml
# Source — release.yml line 25
- uses: actions/checkout@v4
# Source — release.yml line 50
- uses: goreleaser/goreleaser-action@v6.4.0
```
Add `actions/github-script@v7` pinned at v7 (current major).

### Step-level (NOT workflow-level) env for secrets
**Source:** `release.yml:55-56`
**Apply to:** `.github/workflows/release.yml` (the new `PACKAGES_PAT` line)
**Why:** keeps the secret bound to the goreleaser step only; never leaks into other steps. Anti-Pattern in 52-RESEARCH.md.
```yaml
# Existing pattern — secrets live ONLY on the goreleaser step's env:
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Env-var-driven shell snippets in INSTALL.md
**Source:** `INSTALL.md:37-47, 60-81`
**Apply to:** new Homebrew / Scoop / deb / rpm sections in INSTALL.md
**Why:** Phase 51 established the `VERSION=...; OS=...; ARCH=...; curl -LO ...` form as the house style for copy-paste install snippets.
```bash
VERSION=v1.9.0
OS=linux
ARCH=amd64
curl -LO "https://github.com/postfix/serena/releases/download/${VERSION}/<artifact>"
```

### Numbered ritual blocks in RELEASING.md
**Source:** `RELEASING.md:35-54, 96-100`
**Apply to:** new bootstrap and rotation sections in RELEASING.md
**Why:** Phase 51's maintainer rituals are presented as numbered shell-block sequences with "Expected:" callouts. Match this voice.

### `skip_upload: auto` for prerelease tags
**Source:** `.goreleaser.yml:144` (`prerelease: auto`) — establishes the prerelease-detection contract
**Apply to:** new `brews:` and `scoops:` blocks in `.goreleaser.yml`
**Why:** Pitfall 5 in 52-RESEARCH.md — without this, `v1.9.0-rc1` would push the Formula and pin stable users to an RC.
```yaml
brews:
  - name: serena
    skip_upload: auto
scoops:
  - name: serena
    skip_upload: auto
```
Do NOT add to `nfpms:` (deb/rpm should still attach to prerelease GitHub Releases for testers).

### Shared `ids: [archive]` cross-reference
**Source:** `.goreleaser.yml:100` (`- id: archive`)
**Apply to:** new `brews:` and `scoops:` blocks
**Why:** Both brews and scoops select which archive to wrap. Phase 51's archives entry has id `archive`; the new blocks must point at it.
```yaml
brews:
  - ids: [archive]
scoops:
  - ids: [archive]
```

## No Analog Found

None. Every file in scope has a Phase 51 analog in the repo today (either the same file being extended, or `release.yml` as the closest sibling for `release-verify.yml`).

## Metadata

**Analog search scope:**
- `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/.goreleaser.yml`
- `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/.github/workflows/` (release.yml, docker.yml, go-test.yml, codespell.yml, docs.yaml, junie.yml, pytest.yml)
- `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/INSTALL.md`
- `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/RELEASING.md`
- `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/LICENSE` (for SPDX identifier resolution → MIT)

**Files scanned:** 11
**Strong analogs identified:** 4 (one per file in scope)
**Pattern extraction date:** 2026-04-26
