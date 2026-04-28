# Phase 51: packaging-goreleaser - Research

**Researched:** 2026-04-28
**Domain:** Release engineering — multi-arch signed binaries via goreleaser + GitHub Actions
**Confidence:** HIGH

## Summary

Phase 51 builds a tag-triggered goreleaser pipeline that publishes 6 reproducible
darwin/linux/windows × amd64/arm64 archives, each with a minisign signature, and a
`checksums.txt` file. The decisions are already locked in CONTEXT.md (D-01 through
D-07) so this research does NOT explore alternatives — it confirms the precise
upstream syntax, package versions, and CI patterns the planner needs to write
`.goreleaser.yaml`, `release.yml`, and the INSTALL.md verification block.

Three findings change the planner's writing materially:

1. **Repo identity is a refactor, not a question.** The git remote is
   `git@github.com:agenthands/helix.git`, but `go.mod`, INSTALL.md, and README.md
   all still say `github.com/postfix/serena/cmd/serena`. The release pipeline
   targets `agenthands/helix`. The planner must update both INSTALL.md line 10/18
   and README.md lines 65/71 to point at `agenthands/helix`. Whether `go.mod`
   gets renamed in this phase is a separate decision (it's a much larger blast
   radius — every import path inside the binary changes). Tentative
   recommendation: **leave `go.mod` alone in Phase 51**; only fix the public-facing
   doc URLs. Goreleaser doesn't care about the module path; it cares about the
   GitHub repo, which is set via `release.github.owner/name` (or auto-detected
   from `git remote`). Flagged as Open Question 1.
2. **`checksums.txt` is not the goreleaser default.** The default name template
   is `{{ .ProjectName }}_{{ .Version }}_checksums.txt`. To match the D-04 INSTALL
   block (`curl -LO .../checksums.txt`), the planner must override
   `checksum.name_template: "checksums.txt"`.
3. **`.tar.gz` for Windows is not the goreleaser default either.** The default
   archive format is `tar.gz` for unix and `zip` for Windows (when
   `format_overrides` is configured the typical way). The D-04 INSTALL example
   uses `.tar.gz` for all OSes. Either (a) force `formats: ["tar.gz"]` with no
   overrides — Windows users get `.tar.gz` (unconventional but matches the
   single-block INSTALL UX), or (b) document the Windows `.zip` exception in
   INSTALL.md. Recommendation: **force tar.gz uniformly** to keep the D-04
   block as a single copy-paste — it's an explicit user preference for ceremony
   minimalism and tar.gz is fine on modern Windows (`tar` is in System32 since
   Windows 10 1803).

**Primary recommendation:** Two-plan split (config-and-workflow / docs-and-makefile),
~6 files touched, ~1 day of execution. The planner can write all six files
prescriptively from this RESEARCH; no further upstream lookup needed for the
default path.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Sign with **minisign** (Ed25519). Each archive gets a sibling `.minisig`. NOT cosign.
- **D-02:** Public key file at repo root: `minisign.pub`. INSTALL.md links to canonical raw URL.
- **D-02a:** GH Actions secrets: planner picks names (suggested `MINISIGN_PRIVATE_KEY` + `MINISIGN_PASSWORD`). Documented in CONTRIBUTING.md.
- **D-03:** **CI-enforced reproducibility.** Workflow builds twice, fails if archive sha256s differ (excluding `*.minisig`, `*.sig`, `checksums.txt`). Non-reproducible build MUST NOT publish a Release.
- **D-03a:** Standard reproducibility flags: `-trimpath`, `mod=readonly`, deterministic timestamps via `mod_timestamp: '{{ .CommitTimestamp }}'`. Researcher confirms exact list (see Standard Stack below).
- **D-04:** INSTALL.md verification is a **single fenced bash block** users can copy-paste end-to-end (URL pattern, checksum verify, minisig verify, untar). Block uses placeholders the planner replaces with actual repo URL and pubkey.
- **D-05:** INSTALL.md is **restructured** — pre-built binary section LEADS, "Build from source" demoted to subsection.
- **D-06:** **Tag-triggered auto-publish.** Any `v*` tag triggers a release. Pre-release tags (`v*-rc*`, `v*-beta*`, `v*-alpha*`) auto-mark as Pre-release. No draft step, no `workflow_dispatch` gate.
- **D-07:** **Goreleaser auto-generates release notes** from git log with conventional-commit grouping (feat/fix/docs/etc.). CHANGELOG.md stays hand-curated separately.

### Claude's Discretion

- Filename: `.goreleaser.yaml` vs `.goreleaser.yml` — **recommend `.goreleaser.yaml`** (newer goreleaser docs use `.yaml`).
- Release workflow filename — **recommend `.github/workflows/release.yml`**.
- Archive naming pattern beyond D-04 example — **recommend `serena_{{ .Version }}_{{ .Os }}_{{ .Arch }}`** (matches D-04 exactly).
- Whether to include LICENSE/README inside archives — **recommend the goreleaser default** (LICENSE + README + CHANGELOG bundled).
- Local dry-run command — **recommend `make release-snapshot`** following Phase 50 D-07 self-doc style; output goes to `dist/` (already gitignored).
- Exact reproducibility flag set (D-03a) — confirmed below in Standard Stack.
- Fate of `.github/workflows/publish.yml` — **DELETE** (zero cross-references found, see Investigation below).
- Secret name for minisign private key (D-02a) — **recommend `MINISIGN_PRIVATE_KEY` and `MINISIGN_PASSWORD`** (clear naming, conventional).
- Reproducibility check job shape (D-03) — **recommend single job, two snapshot builds** (see Pattern 4 below).
- Whether macOS gets parallel block — **recommend a one-line note** ("On macOS use `shasum -a 256 -c` instead of `sha256sum -c`"); D-04 explicitly favors single-block UX.

### Deferred Ideas (OUT OF SCOPE)

- cosign / Sigstore — defer to PKG-DEFER-02 (containers).
- Helper script `scripts/verify-release.sh` — chicken-and-egg trust problem.
- `workflow_dispatch`-only trigger — could be added later as additional trigger.
- Draft-first release flow — reproducibility gate (D-03) is the safety net.
- Local "release archive" directory — `dist/` already gitignored; nothing to do.
- In-binary auto-update — out of scope project-wide.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PKG-01 | GitHub Releases publish multi-arch binaries (darwin/linux/windows × amd64/arm64) with SHA-256 checksums and cryptographic signatures (cosign or minisign) via a reproducible goreleaser pipeline | All sections — Standard Stack confirms goreleaser+minisign config; Architecture Patterns confirms tag-triggered workflow + reproducibility gate; Code Examples provides drop-in YAML; Validation Architecture defines the success criteria check list. |

</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Single Go binary, no Python/Docker/runtime deps.** The release artifact is a single static binary produced from `cmd/serena`. CGO_ENABLED=0 in builds env.
- **Always run `go vet ./...` and `go test ./...` before completing any Go task.** Release workflow does not need to re-run tests (go-test.yml gates main; release runs only after a tag pushed against an already-tested commit), but the planner SHOULD make the release workflow `needs: test` or check that the green-CI assumption holds at tag time.
- **GSD workflow enforcement** — the planner uses `/gsd-plan-phase` artifacts; this RESEARCH.md is consumed by the planner.
- **Legacy Python lives in `legacy/`** — out of scope for Phase 51; `publish.yml` (Python publish workflow) is unrelated to anything in `legacy/`.

## Architectural Responsibility Map

This phase is pure release engineering — no application-tier concerns. Mapping by deliverable:

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Multi-arch binary build | Release pipeline (goreleaser) | — | Goreleaser owns cross-compilation matrix |
| Archive packaging | Release pipeline (goreleaser) | — | Goreleaser archives plugin |
| Cryptographic signing | Release pipeline (goreleaser signs:) | minisign CLI | Goreleaser invokes minisign as subprocess |
| Reproducibility gate | CI workflow (release.yml) | — | Workflow runs goreleaser twice, diffs sha256 |
| Release publication | CI workflow (release.yml) | GitHub Releases API | goreleaser-action calls Releases API |
| Verification UX | Documentation (INSTALL.md) | minisign CLI on user machine | User runs minisign locally; INSTALL.md is the recipe |
| Local dry-run | Makefile + goreleaser | dist/ gitignored output | `make release-snapshot` writes to `dist/` |
| Public key distribution | Repo root file (`minisign.pub`) | GitHub raw.githubusercontent.com | INSTALL.md links to canonical raw URL |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| goreleaser | `~> v2` (current major: v2.x) | Multi-arch build, archive, sign, publish orchestrator | Industry standard for Go release pipelines; native GitHub Releases support [VERIFIED: goreleaser docs] |
| goreleaser/goreleaser-action | `@v7` | GitHub Action wrapper for goreleaser | Official action; current latest stable [VERIFIED: github.com/goreleaser/goreleaser-action README] |
| minisign | latest (jedisct1/minisign) | Ed25519 signature tool | Picked in D-01 over cosign for footprint + simplicity |
| actions/checkout | `@v4` | Repo checkout, requires `fetch-depth: 0` for goreleaser changelog | Official, matches go-test.yml |
| actions/setup-go | `@v5` | Go toolchain install | Matches go-test.yml exactly |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `tar` (BSD/GNU) | system | Archive extraction in INSTALL.md | Required on user machine; built into macOS/Linux/Windows-10+ |
| `sha256sum` (GNU) / `shasum` (BSD) | system | Checksum verification in INSTALL.md | One-line note in INSTALL.md for macOS users |
| `curl` | system | Download archives in INSTALL.md | Standard everywhere |

### Alternatives Considered (REJECTED — locked decisions)
| Instead of | Could Use | Tradeoff | Status |
|------------|-----------|----------|--------|
| minisign | cosign / Sigstore | Keyless signing, transparency log; heavier verifier UX | REJECTED in D-01 |
| Tag-triggered | workflow_dispatch | Manual gate; better for typo-prone tags | REJECTED in D-06 |
| Hand-curated CHANGELOG | Auto from git log | Curated narrative; manual per-release work | REJECTED in D-07 (auto wins; CHANGELOG.md stays separate) |

**Installation (versions verified against docs 2026-04-28):**
- goreleaser-action: `goreleaser/goreleaser-action@v7` — latest stable per README
- goreleaser binary in action: `version: '~> v2'` — current major
- actions/checkout: `@v4` — matches go-test.yml
- actions/setup-go: `@v5` — matches go-test.yml
- minisign: `latest` from jedisct1/minisign — installed via apt or brew; see Pitfall 3 below for installation choice

## Architecture Patterns

### System Architecture Diagram

```
                          [Maintainer pushes git tag v1.9.0-rc1]
                                          |
                                          v
                              [.github/workflows/release.yml]
                                          |
                                          v
                              +-----------------------+
                              | actions/checkout@v4   |
                              | fetch-depth: 0        |
                              +-----------+-----------+
                                          |
                                          v
                              +-----------------------+
                              | actions/setup-go@v5   |
                              | go-version: 1.25.x    |
                              +-----------+-----------+
                                          |
                                          v
                              +-----------------------+
                              | install minisign      |
                              | (apt or release dl)   |
                              +-----------+-----------+
                                          |
                                          v
                       +------------------+------------------+
                       |                                     |
                       v                                     v
            +---------------------+              +---------------------+
            | goreleaser PASS 1   |              | goreleaser PASS 2   |
            | release --snapshot  |              | release --snapshot  |
            | --clean             |              | --clean             |
            | (writes dist1/)     |              | (writes dist2/)     |
            +----------+----------+              +----------+----------+
                       |                                     |
                       +------------------+------------------+
                                          |
                                          v
                              +-----------------------+
                              | sha256 diff:          |
                              | dist1/*.tar.gz vs     |
                              | dist2/*.tar.gz        |
                              | (exclude .minisig,    |
                              |  checksums.txt)       |
                              +-----------+-----------+
                                          |
                                  match? --+-- mismatch
                                          |          |
                                          v          v
                              +-----------------------+   FAIL job
                              | goreleaser PASS 3     |   (no Release)
                              | release --clean       |
                              | (signs + uploads)     |
                              +-----------+-----------+
                                          |
                                          v
                              [GitHub Release published]
                              - 6 archives (.tar.gz)
                              - 6 .minisig files
                              - checksums.txt
                              - auto-generated notes
                                          |
                                          v
                              [User: curl + minisign -V]
                              follows INSTALL.md block
```

**Why three goreleaser invocations?** Two snapshot runs to verify reproducibility, one
real release run to actually publish. The two snapshot runs use `--snapshot` so they
don't try to upload; the third run uses real release mode. Cost: ~3x build time (each
build is ~1-2 minutes on a 6-target Go binary). This is the simplest shape that
meets D-03's hard requirement that a non-reproducible build MUST NOT publish.

**Alternative considered:** Run goreleaser twice in `--snapshot` mode, then have a
second job (`needs: verify`) do the real release. This adds CI overhead (separate
checkout, separate setup-go) for no real gain — same job is cheaper and the gate
semantics are identical. Recommendation stands at single-job/three-passes.

### Recommended Project Structure
```
/
├── .goreleaser.yaml           # NEW — goreleaser config (D-03a, D-06, D-07)
├── .github/
│   └── workflows/
│       ├── release.yml        # NEW — tag-triggered release workflow (D-03 gate)
│       ├── go-test.yml        # KEEP — Phase 50 ubuntu-latest CI
│       ├── publish.yml        # DELETE — legacy Python uv publish (zero refs)
│       └── ...                # KEEP all others (codeql, codespell, docker, docs, junie, pytest)
├── minisign.pub               # NEW — public key at repo root (D-02)
├── INSTALL.md                 # MAJOR EDIT — pre-built lead, D-04 verify block, D-05 restructure
├── README.md                  # SMALL EDIT — line 65 + 71 repo path postfix→agenthands
├── Makefile                   # EDIT — add release-snapshot target (Phase 50 D-07 style)
├── CONTRIBUTING.md            # EDIT — add "Releasing" subsection (secrets, dry-run, key rotation)
└── dist/                      # gitignored already (line 77 of .gitignore)
```

### Pattern 1: Reproducible builds: block (D-03a)

**What:** Goreleaser config with the canonical reproducible-build flag set.
**When to use:** Every build target in this project.
**Example:**
```yaml
# Source: https://goreleaser.com/customization/builds/go/ + reproducible-builds blog post
# Verified 2026-04-28
version: 2

project_name: serena

builds:
  - id: serena
    main: ./cmd/serena
    binary: serena
    env:
      - CGO_ENABLED=0
      - GOFLAGS=-mod=readonly
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.CommitDate}}
    mod_timestamp: '{{ .CommitTimestamp }}'
```

**Confirmed reproducibility flag set (D-03a):**
- `flags: [-trimpath]` — strips build-host paths from binary [VERIFIED: goreleaser docs verifiable_builds + carlosbecker.com reproducible-builds post]
- `env: GOFLAGS=-mod=readonly` — forbids module mutation during build [VERIFIED: confirmed standard pattern in goreleaser builds/go docs]
- `env: CGO_ENABLED=0` — pure-Go build for host-independence [VERIFIED: goreleaser builds/go docs]
- `mod_timestamp: '{{ .CommitTimestamp }}'` — sets archive entry mtimes from commit time [VERIFIED: goreleaser docs builds/go]
- `ldflags: -X main.date={{.CommitDate}}` — replaces default build-time date with commit date [VERIFIED: reproducible-builds blog]
- `ldflags: -s -w` — strips symbol table & DWARF (smaller binary, identical bytes) [VERIFIED: standard Go practice; goreleaser examples]

**Pitfall the planner must NOT trip on:** Goreleaser's default ldflags inject
`main.date={{.Date}}` (real wall-clock build time, NOT commit date). If the
planner omits the `ldflags:` block, the binary will embed the timestamp of each
build run and reproducibility will fail. **The `ldflags` override is mandatory.**

### Pattern 2: archives + checksum block (D-04 INSTALL match)

```yaml
# Source: https://goreleaser.com/customization/archive/ + checksum/
archives:
  - id: serena
    builds: [serena]
    formats: ["tar.gz"]
    name_template: "serena_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE*
      - README.md
      - INSTALL.md
      - CHANGELOG.md

checksum:
  name_template: "checksums.txt"   # OVERRIDE default; D-04 INSTALL block expects this
  algorithm: sha256
```

**Why `formats: ["tar.gz"]` (not `format_overrides` for windows zip):** D-04
INSTALL.md uses `.tar.gz` for all OSes. Forcing tar.gz uniformly keeps the
single-block UX (no OS branching in the verify recipe). Modern Windows (10
1803+) ships `tar` in System32, so users on Windows can extract `.tar.gz`
without third-party tools.

**Why `name_template: "checksums.txt"`:** Goreleaser's default is
`{{ .ProjectName }}_{{ .Version }}_checksums.txt` which would produce
`serena_v1.9.0_checksums.txt` and break the D-04 verify block. The override
is small but mandatory.

### Pattern 3: signs block (minisign — D-01)

```yaml
# Source: https://goreleaser.com/customization/sign/sign/ adapted for minisign
# Verified against jedisct1/minisign manpage 2026-04-28
signs:
  - id: minisign
    cmd: minisign
    artifacts: archive             # Sign each archive only (NOT checksums.txt; verifier flow checks sha256 of checksums.txt itself separately)
    signature: "${artifact}.minisig"
    args:
      - "-S"                       # sign mode
      - "-s"                       # secret key file (next arg)
      - "/tmp/minisign.key"        # path written by release.yml step from secret
      - "-x"                       # output sig path
      - "${signature}"
      - "-m"                       # file to sign
      - "${artifact}"
    stdin: '{{ .Env.MINISIGN_PASSWORD }}'   # echo password to stdin
```

**Notes:**
- `artifacts: archive` signs the 6 archives, NOT the checksums.txt file. (Users
  verify checksums.txt by trusting the archive signature transitively, OR the
  planner can sign `all` and add a sigfile for checksums.txt too — recommend
  signing only `archive` to keep the verify recipe single-step.)
- `stdin: '{{ .Env.MINISIGN_PASSWORD }}'` — minisign reads encrypted-key
  password from stdin; goreleaser pipes the env var into the subprocess
  [VERIFIED: minisign manpage shows stdin password support; goreleaser
  signs.stdin field documented]
- Alternative: use unencrypted secret key (minisign `-W` flag at keygen time)
  and skip the password entirely. **Recommend keeping the password** —
  defense-in-depth in case the GH secret leaks separately from the
  password secret.
- The release.yml step writes `MINISIGN_PRIVATE_KEY` secret content to
  `/tmp/minisign.key` before goreleaser runs (see Pattern 5).

### Pattern 4: changelog (D-07 — conventional commits)

```yaml
# Source: https://goreleaser.com/customization/changelog/
changelog:
  use: git
  sort: asc
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Bug Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Refactor
      regexp: '^.*?refactor(\([[:word:]]+\))??!?:.+$'
      order: 2
    - title: Documentation
      regexp: '^.*?docs(\([[:word:]]+\))??!?:.+$'
      order: 3
    - title: Others
      order: 999
  filters:
    exclude:
      - '^test:'
      - '^chore:'
      - 'Merge pull request'
      - 'Merge branch'
```

This matches Helix's existing commit style observed in `git log` (e.g.
`docs(state): record phase 51 context session`, `fix(50.1): symbol tools
1-indexed positions`). **Test commits and chores are filtered out** of release
notes; everything else groups by prefix.

### Pattern 5: release block + GitHub Action workflow (D-06)

```yaml
# .goreleaser.yaml — release section
release:
  github:
    owner: agenthands
    name: helix
  prerelease: auto              # auto-detects v*-rc*, v*-beta*, v*-alpha*  [VERIFIED: goreleaser release docs]
  draft: false                  # publish immediately, no draft step (D-06)
  mode: replace                 # if re-running for the same tag, replace artifacts
  name_template: "Serena {{ .Tag }}"
```

```yaml
# .github/workflows/release.yml
name: release
on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write       # required: create release, upload assets

jobs:
  release:
    name: goreleaser
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0       # required: goreleaser changelog needs full git history

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'   # mirrors go-test.yml exactly
          cache: true

      - name: Install minisign
        run: |
          set -euo pipefail
          # Use a pinned release artifact for byte-identical CI behavior
          MINISIGN_VERSION="0.12"
          curl -fsSL -o /tmp/minisign.tar.gz \
            "https://github.com/jedisct1/minisign/releases/download/${MINISIGN_VERSION}/minisign-${MINISIGN_VERSION}-linux.tar.gz"
          tar -xzf /tmp/minisign.tar.gz -C /tmp
          sudo install -m 0755 /tmp/minisign-linux/x86_64/minisign /usr/local/bin/minisign
          minisign -v

      - name: Write minisign secret key to disk
        env:
          MINISIGN_PRIVATE_KEY: ${{ secrets.MINISIGN_PRIVATE_KEY }}
        run: |
          umask 077
          printf '%s' "$MINISIGN_PRIVATE_KEY" > /tmp/minisign.key

      - name: Reproducibility gate (snapshot pass 1)
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --snapshot --clean --skip=sign

      - name: Capture pass 1 sha256s
        run: |
          set -euo pipefail
          mv dist dist-pass-1
          (cd dist-pass-1 && find . -name '*.tar.gz' -print0 | sort -z | xargs -0 sha256sum) > /tmp/pass1.sha256

      - name: Reproducibility gate (snapshot pass 2)
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --snapshot --clean --skip=sign

      - name: Diff sha256s — fail if non-reproducible
        run: |
          set -euo pipefail
          (cd dist && find . -name '*.tar.gz' -print0 | sort -z | xargs -0 sha256sum) \
            | sed 's| dist/| dist-pass-1/|' > /tmp/pass2.sha256
          if ! diff -u /tmp/pass1.sha256 /tmp/pass2.sha256; then
            echo "::error::archives are not byte-identical between two snapshot runs"
            exit 1
          fi
          echo "Reproducibility gate PASS"

      - name: Real release (sign + publish)
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          MINISIGN_PASSWORD: ${{ secrets.MINISIGN_PASSWORD }}
```

**Key shape decisions in this workflow:**
1. **`--skip=sign` on snapshot passes** — signing is non-deterministic (minisign
   has a nonce per signature), so signing during reproducibility-check would
   trip the diff. The diff also explicitly looks at `*.tar.gz` only (not
   `.minisig` or `checksums.txt`), but `--skip=sign` is cleaner: the snapshot
   passes never produce sigs at all.
2. **`mv dist dist-pass-1`** — `--clean` wipes `dist/` at the start of each run,
   so we save the first pass before the second pass overwrites it. Path
   normalization in the diff step makes the sha256sum output comparable.
3. **GITHUB_TOKEN is only set on the real release pass** — snapshot passes
   don't upload, so they don't need it. (Goreleaser's snapshot mode doesn't
   try to push to GitHub.)
4. **`needs: test`** on the release job is NOT used here — release.yml is
   triggered by `push: tags`, and go-test.yml runs on `push: branches: [main]`
   so they're independent. The locked decision (D-06) is "tag triggers
   immediately" and the reproducibility gate is the safety net. The planner
   could add a `needs:` between jobs in the same workflow but adding a
   cross-workflow dependency on go-test.yml requires a more complex
   `repository_dispatch` or status-check shape — recommend NOT adding it.

### Pattern 6: Makefile release-snapshot target

```makefile
# Phase 50 D-07 self-doc style; mirrors `bench` / `bench-baseline`
release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
	goreleaser release --snapshot --clean --skip=sign
```

**Why `--skip=sign` locally:** Most contributors don't have the project's
minisign secret key, so signing would fail. The snapshot is for verifying the
build matrix produces archives — signing is verified in CI only.

### Anti-Patterns to Avoid

- **Don't omit `ldflags`.** Goreleaser's default injects `main.date` with wall-clock build time; reproducibility will fail. Always set `-X main.date={{.CommitDate}}`.
- **Don't sign during reproducibility-check passes.** Minisign signatures contain a per-call nonce; two signs of the same file produce different `.minisig` bytes. Use `--skip=sign` on the verify passes.
- **Don't use `goreleaser release` (without `--snapshot`) for the verify passes.** That would try to upload to GitHub Releases and either duplicate-fail or actually publish broken artifacts. Always `--snapshot` for verification.
- **Don't trust goreleaser's default `checksum.name_template`.** It produces `serena_v1.9.0_checksums.txt`, not `checksums.txt`. The D-04 INSTALL block requires the override.
- **Don't trust goreleaser's default archive `formats`.** The D-04 INSTALL block uses `.tar.gz` everywhere; goreleaser's typical example uses `format_overrides` for Windows zip. Force `formats: ["tar.gz"]` uniformly OR document the Windows exception in INSTALL.md (the planner picked uniformly per Discretion above).
- **Don't sign `checksums.txt`.** D-04 INSTALL flow uses `sha256sum -c` against `checksums.txt` and `minisign -V` against the archive. Signing the checksums file too adds a second verification step that breaks the single-block UX.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-compilation matrix | Custom bash loop with `GOOS=... GOARCH=... go build` | goreleaser `builds.goos/goarch` | Goreleaser handles per-arch ldflags, parallel builds, archive packaging in one tool |
| Archive packaging | `tar -czf` per OS + custom name template | goreleaser `archives:` | Format overrides, file inclusion (LICENSE/README), naming consistency |
| Checksum file generation | `sha256sum dist/* > checksums.txt` | goreleaser `checksum:` | Reproducible naming, configurable algorithms, integrates with sign step |
| Release notes | Custom `git log` parsing, sed/awk filters | goreleaser `changelog:` | Conventional-commit grouping, exclusion filters, native upload to GH Releases |
| GitHub Release upload | `gh release create` + `gh release upload` per file | goreleaser-action | Atomic upload, proper Pre-release detection, asset SHA upload integration |
| Reproducibility check | None — there's no library for this | Two-pass snapshot + sha256 diff (Pattern 5) | This IS the project's hand-rolled bit; no upstream gives this for free |

**Key insight:** Goreleaser is the kitchen sink for Go release pipelines.
Anything in this phase that ISN'T "make goreleaser do the right thing" or
"make CI run goreleaser correctly" is probably a sign of going off-script.
The two genuinely custom pieces are (1) the reproducibility-diff step and
(2) the INSTALL.md verification ceremony — both are user-facing project
contracts, not generic infrastructure.

## Common Pitfalls

### Pitfall 1: Snapshot version template breaks the diff

**What goes wrong:** Both `--snapshot` runs append `-SNAPSHOT-{ShortCommit}` to
the version. If the planner forgets to use `--clean` between runs OR diffs the
archives by name including version, mismatched paths cause spurious diff failures.
**Why it happens:** Goreleaser's default snapshot version_template is
`{{ .Version }}-SNAPSHOT-{{.ShortCommit}}` [VERIFIED: snapshots docs]. Both
snapshot runs in the SAME checkout produce the SAME ShortCommit, so the names
are identical — but only if `--clean` is set both times.
**How to avoid:** Always pass `--clean` to both snapshot invocations and `mv`
the first dist/ aside before the second run (Pattern 5 does this). Compare
sha256s of relative paths after path normalization.
**Warning signs:** Diff fails with "only in dist-pass-1/" or "only in dist/"
file lists.

### Pitfall 2: Repo identity drift (postfix → agenthands)

**What goes wrong:** `go.mod` says `github.com/postfix/serena`, INSTALL.md and
README.md show `go install github.com/postfix/serena/cmd/serena@latest`, but the
GitHub repo is `agenthands/helix`. After Phase 51 publishes Releases at
`agenthands/helix/releases`, the doc URLs still point users at the wrong place.
**Why it happens:** Repo was migrated from `postfix/serena` to `agenthands/helix`
(commit history shows "chore: snapshot for Helix repo migration") but the doc
URLs were not updated.
**How to avoid:** Phase 51 plan MUST include line-edits to:
- INSTALL.md line 10: `go install github.com/postfix/serena/cmd/serena@latest` → either delete (if go.mod doesn't change, this `go install` won't work) or move to "Build from source" with the new path
- INSTALL.md line 18: `git clone https://github.com/postfix/serena.git` → `git clone https://github.com/agenthands/helix.git`
- README.md line 65 + 71: same fix
- All `<owner>/<repo>` placeholders in D-04 → `agenthands/helix`
**Warning signs:** Tagged release publishes to agenthands/helix; user follows
INSTALL.md and downloads from postfix/serena (404). See Open Question 1 about
whether to also rename `go.mod`.

### Pitfall 3: minisign install method on ubuntu-latest

**What goes wrong:** Picking the wrong install path makes the workflow brittle
or non-reproducible.
**Why it happens:** Three install options: (a) `apt-get install minisign` —
package exists in Ubuntu Universe but version may lag; (b) `brew install
minisign` — not available on ubuntu-latest by default; (c) GitHub release
tarball download from jedisct1/minisign — pinnable to exact version.
**How to avoid:** Use **option (c) with a pinned version** (Pattern 5 above).
This gives reproducible CI behavior — the same minisign binary every run.
Distro-package version drift is a known reproducibility hazard.
**Warning signs:** CI flake on minisign install; signature format changes
between minisign versions (rare but possible).

### Pitfall 4: Goreleaser action passes `--clean` but `dist/` is not gitignored

**What goes wrong:** If `dist/` ever ends up tracked in git, `--clean` deletes
it from the working tree, leaving uncommitted deletions and weird state.
**Why it happens:** `--clean` removes the `dist/` directory before each run.
**How to avoid:** Confirm `dist/` is in `.gitignore`. **Already verified:
.gitignore line 77 has `dist/`** — no action needed.
**Warning signs:** N/A — already handled.

### Pitfall 5: Conflict with stale Python publish.yml

**What goes wrong:** Both `publish.yml` (Python uv publish) and `release.yml`
(new goreleaser) co-exist. On a `release: created` event, `publish.yml` fires
and tries to run `uv build` against a Go-only working tree, producing confusing
failures.
**Why it happens:** `publish.yml` is triggered on `release: types: [created]`.
When goreleaser-action publishes a release, GitHub fires a `release: created`
event, which then triggers publish.yml.
**How to avoid:** **DELETE `.github/workflows/publish.yml`** in Phase 51.
Confirmed zero cross-references in `.github/`, `docs/`, `CONTRIBUTING.md`,
`README.md`. The legacy/ Python tree does not invoke this workflow; the
workflow's purpose was the original Python serena-agent PyPI publish, which
the Helix Go rewrite has obsoleted.
**Warning signs:** Two workflows running on tag push; mysterious `uv build`
failure in CI.

### Pitfall 6: `mod_timestamp` requires the `.CommitTimestamp` Unix-epoch variant

**What goes wrong:** Using `{{ .Date }}` or `{{ .CommitDate }}` (RFC3339 string)
in `mod_timestamp` produces a string-vs-int mismatch — goreleaser expects an
integer Unix epoch.
**Why it happens:** `mod_timestamp` field expects a numeric template; only
`.CommitTimestamp` (epoch int) and `.Timestamp` (current build epoch) match.
`.Date` and `.CommitDate` are strings used in ldflags.
**How to avoid:** **Always use `mod_timestamp: '{{ .CommitTimestamp }}'` —
the literal string from the CONTEXT D-03a.**
**Warning signs:** Goreleaser error like "invalid mod_timestamp" or archive
mtimes set to wrong values.

## Code Examples

### Verifying a release on Linux (D-04 verify block — final form)

```bash
# Source: D-04 in CONTEXT.md, with placeholders resolved
# Linux verification flow
VERSION=v1.9.0
OS=linux
ARCH=amd64
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz.minisig
curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt
curl -LO https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub  # one-time

# Verify checksum
sha256sum -c --ignore-missing checksums.txt    # GNU; macOS users: shasum -a 256 -c --ignore-missing checksums.txt

# Verify signature
minisign -V -p minisign.pub -m serena_${VERSION}_${OS}_${ARCH}.tar.gz

# Extract
tar -xzf serena_${VERSION}_${OS}_${ARCH}.tar.gz
./serena --help
```

[VERIFIED: `minisign -V -p <file> -m <file>` is canonical syntax per minisign
README and manpage. `-P <base64>` form also works if INSTALL.md prefers
embedding the pubkey string directly rather than asking users to download it.]

### macOS verification one-line note (parallel to D-04)

If the planner chooses the one-line note (recommended per Discretion above):

> **macOS users:** replace `sha256sum -c` with `shasum -a 256 -c`. Install
> minisign with `brew install minisign`. Everything else is identical.

[VERIFIED: jedisct1/minisign README lists `brew install minisign` as canonical
macOS install.]

### Local dry-run

```bash
# Source: Pattern 6 above
make release-snapshot
# writes archives to dist/ (overwrites previous; gitignored)
ls dist/serena_*.tar.gz   # 6 archives
```

## Runtime State Inventory

> Phase 51 is a configuration / new-files phase, not a rename. The only
> rename-adjacent concern is the postfix→agenthands repo identity drift in
> docs (Pitfall 2). Categorizing for completeness:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — phase introduces no datastores | None |
| Live service config | Minisign secret keypair stored as GitHub Actions secrets (`MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD`) | NEW — generate keypair, upload secrets via `gh secret set`. Document in CONTRIBUTING.md "Releasing" section. |
| OS-registered state | None | None |
| Secrets/env vars | NEW: `MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD` (GH Actions); existing: `GITHUB_TOKEN` (auto-injected) | None for existing; new secrets must exist BEFORE first tag push |
| Build artifacts | `dist/` directory created by goreleaser; already gitignored | None — `.gitignore` line 77 covers it |

**Doc URL drift (postfix → agenthands):** Found in INSTALL.md (lines 10, 18),
README.md (lines 65, 71). Code edit, not data migration. **Verified by grep**:
```
INSTALL.md:10:go install github.com/postfix/serena/cmd/serena@latest
INSTALL.md:18:git clone https://github.com/postfix/serena.git
README.md:65:go install github.com/postfix/serena/cmd/serena@latest
README.md:71:git clone https://github.com/postfix/serena.git
```

**`go.mod` module path drift:** `module github.com/postfix/serena` — see Open
Question 1.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25.x | Local `make release-snapshot`, CI release.yml | ✓ (project requirement) | 1.25.1 (per go.mod) | — |
| goreleaser | Local `make release-snapshot`, CI (via action) | Locally: needs `brew install goreleaser` (macOS) or release tarball; CI: provided by `goreleaser/goreleaser-action@v7` | `~> v2` | — |
| minisign CLI | Local key generation, signing dry-run; CI release.yml signing pass; user verification | Locally: `brew install minisign` (macOS), `apt install minisign` (Ubuntu); CI: GitHub release tarball; user: same | `0.12+` | — |
| `tar` | User extraction | ✓ everywhere (built into macOS/Linux/Windows-10+) | system | — |
| `sha256sum` (Linux) / `shasum` (macOS) | User checksum verification | ✓ system | — | one-line OS note in INSTALL.md |
| `gh` CLI | Maintainer secret upload (one-time) | optional; web UI also works | — | GitHub web UI |
| `git tag --sign` | Maintainer cuts release | optional but recommended | — | unsigned tags work too |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None requiring fallback flow design.

**Maintainer prerequisite (NOT runtime — one-time setup):** The minisign keypair
must be generated locally (`minisign -G -s minisign.key -p minisign.pub`) and
the `minisign.key` content uploaded as `MINISIGN_PRIVATE_KEY` GH secret BEFORE
the first `v*` tag is pushed. CONTRIBUTING.md "Releasing" section must document
this with exact commands.

## Validation Architecture

> Nyquist validation is enabled (`workflow.nyquist_validation: true`).

### Test Framework
| Property | Value |
|----------|-------|
| Framework | bash assertions in CI (release.yml) + manual dry-run via `make release-snapshot` |
| Config file | `.goreleaser.yaml` (config-as-test); `.github/workflows/release.yml` (CI gate) |
| Quick run command | `make release-snapshot` (verifies build matrix produces 6 archives) |
| Full suite command | `goreleaser release --snapshot --clean` then `goreleaser check` (lints config) |
| Phase gate | Tagged release artifact (e.g. `v1.9.0-rc1`) appears on agenthands/helix Releases with 6 archives + 6 minisigs + checksums.txt |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PKG-01a | Tag push triggers workflow | smoke | Push test tag `v0.0.0-rc-test` → observe release.yml run | ❌ Wave 0 (workflow doesn't exist yet) |
| PKG-01b | 6-arch matrix produces 6 archives | unit (build) | `make release-snapshot && ls dist/serena_*.tar.gz \| wc -l` → expect 6 | ❌ Wave 0 (.goreleaser.yaml doesn't exist yet) |
| PKG-01c | Checksums.txt produced and named correctly | unit | `make release-snapshot && test -f dist/checksums.txt` | ❌ Wave 0 |
| PKG-01d | Reproducibility — two snapshot runs are byte-identical | integration | release.yml diff step (Pattern 5); locally `goreleaser release --snapshot --clean --skip=sign && cp -r dist /tmp/r1 && goreleaser release --snapshot --clean --skip=sign && diff <(sha256sum /tmp/r1/*.tar.gz) <(sha256sum dist/*.tar.gz \| sed 's\|dist/\|/tmp/r1/\|')` | ❌ Wave 0 |
| PKG-01e | Each archive has a sibling .minisig (CI only — needs secret) | smoke | release.yml output: `ls dist/*.minisig \| wc -l` → expect 6 | ❌ Wave 0 |
| PKG-01f | INSTALL.md verify block runs end-to-end against a published release | manual UAT | Cut a `v0.0.0-rc-test` release, follow INSTALL.md block on a fresh machine, confirm `serena --help` works | ❌ Wave 0 (block doesn't exist yet) |
| PKG-01g | Pre-release tag auto-marks Pre-release | manual UAT | Cut `v0.0.0-rc-test` → check Releases page shows "Pre-release" badge | ❌ Wave 0 |
| PKG-01h | Production tag does NOT mark Pre-release | manual UAT | Cut `v1.9.0` (or test variant `v1.0.0-test`) → check no Pre-release badge | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `goreleaser check` (config lint, fast) + `make release-snapshot` (build matrix smoke, ~1 min on a developer machine)
- **Per wave merge:** Push a throwaway tag like `v0.0.0-test-N` to a fork or branch protected for testing; observe release.yml fires, completes the reproducibility gate, and signs successfully. Delete the test release afterward.
- **Phase gate:** A real `v1.9.0-rc1` tag passes the full pipeline AND a human follows the INSTALL.md block end-to-end on darwin and linux (matches success criterion 3 of ROADMAP).

### Wave 0 Gaps

- [ ] `.goreleaser.yaml` — covers PKG-01b/c/d/e
- [ ] `.github/workflows/release.yml` — covers PKG-01a/d/e (CI sampling)
- [ ] `minisign.pub` — covers PKG-01e (verifier needs pubkey)
- [ ] INSTALL.md verify block — covers PKG-01f
- [ ] CONTRIBUTING.md "Releasing" subsection — covers maintainer ergonomics (key rotation, secret setup, dry-run)
- [ ] `Makefile` `release-snapshot` target — covers PKG-01b/c local sampling
- [ ] Goreleaser binary install on developer machines — documented in CONTRIBUTING.md (`brew install goreleaser` macOS, release tarball Linux)

### ROADMAP Success Criteria → Validation Evidence Map

| Roadmap criterion | Validation evidence |
|-------------------|---------------------|
| 1. Tagging `v1.9.0-rc1` triggers a goreleaser CI workflow uploading 6 platform/arch binaries | release.yml on tag push, observable via Actions UI; PKG-01a/b smoke tests |
| 2. Each binary ships with SHA-256 checksum and cryptographic signature, documented in INSTALL.md | checksums.txt + N.minisig files in Release; INSTALL.md D-04 block; PKG-01c/e |
| 3. User following INSTALL.md verifies signature + checksum in one terminal session | Manual UAT against a real cut release; PKG-01f |
| 4. Pipeline reproducible — second dry-run produces byte-identical archives modulo signatures | release.yml reproducibility gate (Pattern 5); PKG-01d. CI fails the run if non-reproducible — by construction, no non-reproducible release can ever be published |

## Security Domain

> `security_enforcement` is not explicitly disabled in `.planning/config.json`,
> so this section is included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No auth surface in this phase |
| V3 Session Management | no | No sessions |
| V4 Access Control | partial | GitHub Actions secret access (`MINISIGN_PRIVATE_KEY`) — controlled by repo permissions, not in our code |
| V5 Input Validation | no | No user input is parsed in this phase |
| V6 Cryptography | **yes** | minisign Ed25519 (jedisct1/minisign — never hand-roll); SHA-256 checksums (goreleaser default) |
| V14 Configuration | **yes** | Workflow permissions (`contents: write` only — least-privilege); secrets scoped to repo, not org |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Tampered binary in Releases | Tampering | Minisign signature on every archive; INSTALL.md ceremony makes verification one-step |
| Compromised secret leaks signing key | Information Disclosure | Encrypted-key minisign keypair + separate `MINISIGN_PASSWORD` secret. Documented key-rotation procedure in CONTRIBUTING.md (D-02). |
| Workflow privilege escalation via `pull_request` | Elevation of Privilege | Workflow triggers ONLY on `push: tags: ['v*']` — pull-request-triggered workflows have no signing access |
| Non-reproducible build hides supply-chain attack | Tampering | CI-enforced reproducibility gate (D-03) catches any build that varies between two same-checkout runs |
| Stale public key in repo (key rotation lag) | Spoofing | `minisign.pub` at repo root + main branch protection; rotation = a normal commit. INSTALL.md links to `raw.githubusercontent.com/.../main/minisign.pub` so users always fetch the current key |
| Legacy publish.yml fires on release: created | Tampering / Confusion | DELETE `.github/workflows/publish.yml` in this phase (Pitfall 5) |

**Threat-model conclusion:** The security posture is dominated by the
reproducibility gate (D-03) and minisign signing (D-01). The remaining risk
surface is operational (secret rotation, runner compromise of `ubuntu-latest`)
and is the same surface every Go project's release pipeline carries.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hand-rolled cross-compile + tar + sha256 + sign in CI | goreleaser orchestrates all of it | goreleaser became Go-ecosystem standard ~2018 | Single config file replaces ~200 lines of bash |
| GPG signing | minisign or cosign for OSS Go binaries | mid-2020s shift | Smaller verifier toolchain (200 KB) vs GPG (15 MB+); cosign for keyless+attestation |
| `goreleaser-action@v3/v4/v5/v6` | `@v7` | 2026 (current latest) | Drop-in upgrade; same args |
| `goreleaser` v1.x | v2.x | 2024 schema bump (`version: 2`) | Some field renames (e.g. `format` → `formats` list) |
| Drafts as default | `draft: false` for tag-triggered | Project-by-project | No human gate on tag push |

**Deprecated/outdated:**
- Goreleaser v1 schema (`version: 1`): some configs in older posts use `format:` (string) instead of `formats:` (list). The planner uses `formats: ["tar.gz"]`.
- `format_overrides` for Windows zip: still works, but D-04's single-block UX favors uniform tar.gz.
- `softprops/action-gh-release` for asset upload: legacy publish.yml uses this; goreleaser-action supersedes it for Go projects.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Modern Windows ships `tar` in System32 (10 1803+) so `.tar.gz` is acceptable for Windows users | Pattern 2 | If wrong: Windows users need a third-party tool to extract `.tar.gz`; planner could swap to `format_overrides: [{goos: windows, formats: [zip]}]` and add an OS-conditional to INSTALL.md (breaks single-block UX) |
| A2 | Reproducibility gate of two snapshot runs (same CI VM, same checkout) is sufficient evidence — running on different VMs / different days is not in scope | Pattern 5, Validation | If wrong: a "real" reproducibility check would need separate machines / time-shifted runs. Project scope (D-03) is "byte-identical between two CI invocations on the same VM" — this is a weaker but well-defined contract |
| A3 | The minisign `0.12` release tarball URL pattern (`releases/download/${VERSION}/minisign-${VERSION}-linux.tar.gz`) is current | Pattern 5 install step | If wrong: planner must adjust the URL; minisign release pattern is stable but version 0.12 was the latest at training. Recommend the planner re-verify before writing release.yml. **Confirm with `gh release list -R jedisct1/minisign` at plan time.** |
| A4 | Goreleaser v2 schema is current major (no impending v3) | Standard Stack version | Goreleaser-action `version: '~> v2'` will keep working until v3 ships; minimal risk |

## Open Questions

1. **Should `go.mod` module path be renamed from `github.com/postfix/serena` to `github.com/agenthands/helix` in this phase?**
   - What we know: git remote is agenthands/helix; doc URLs are postfix/serena; go.mod is postfix/serena. Renaming go.mod is a sweeping internal-import rewrite (affects every `package` import statement that uses the module path).
   - What's unclear: whether the project has a pending separate decision about the module rename, OR whether the project intentionally keeps the postfix/serena module path while publishing to agenthands/helix as a transitional state.
   - Recommendation: **Phase 51 fixes only the public-facing doc URLs (INSTALL.md, README.md). The `go.mod` rename is a separate phase or explicit user decision.** Goreleaser does not need the go.mod path to match the GitHub repo; it cares about `release.github.owner/name` only. A user who runs `go install github.com/postfix/serena/cmd/serena@latest` against a renamed module would see a 404 from Go's proxy — but post-binary-first INSTALL.md (D-05) demotes that command. The planner should ASK the user during plan-checking before silently renaming go.mod.

2. **Should the minisign secret key be password-protected or `-W`-unencrypted?**
   - What we know: D-01 says "private key stored as a GitHub Actions secret"; D-02a says "private key + optional password". Both options work.
   - What's unclear: whether the project values the password defense-in-depth or prefers fewer secrets to manage.
   - Recommendation: **Use a password-protected key**. Two secrets (`MINISIGN_PRIVATE_KEY` + `MINISIGN_PASSWORD`) is trivially more friction than one and adds a real defense-in-depth layer. CONTRIBUTING.md "Releasing" section documents both. If the user explicitly prefers `-W` unencrypted for simplicity, the `signs.stdin` line in Pattern 3 is removed.

3. **Should `release.yml` add a `needs:` dependency on a green go-test.yml run?**
   - What we know: D-06 says "tag triggers immediately"; reproducibility gate is the safety net.
   - What's unclear: whether the user wants additional belt-and-suspenders by gating release on a fresh test run.
   - Recommendation: **No additional gate.** D-06 is explicit. Release runs on tag push; tags are typically cut from a green main commit; running tests AGAIN would add ~5min for marginal value. The planner should NOT propose a `needs:` clause. (If a typo'd tag publishes, the project takes the hit and re-cuts.)

## Sources

### Primary (HIGH confidence)
- [Goreleaser docs — builds (Go)](https://goreleaser.com/customization/builds/go/) — confirmed builds: schema, version: 2 header, mod_timestamp/-trimpath/CGO_ENABLED placement
- [Goreleaser docs — archives](https://goreleaser.com/customization/archive/) — confirmed default name_template, formats list, format_overrides
- [Goreleaser docs — checksum](https://goreleaser.com/customization/checksum/) — confirmed default name_template is NOT `checksums.txt` (override required)
- [Goreleaser docs — sign](https://goreleaser.com/customization/sign/sign/) — confirmed `signs:` schema, stdin/stdin_file fields, artifacts options
- [Goreleaser docs — release](https://goreleaser.com/customization/release/) — confirmed `prerelease: auto` matches v*-rc*/v*-beta*/v*-alpha*
- [Goreleaser docs — changelog](https://goreleaser.com/customization/changelog/) — confirmed groups regexp + filters.exclude pattern
- [Goreleaser docs — snapshots](https://goreleaser.com/customization/snapshots/) — confirmed default version_template, --snapshot semantics
- [Goreleaser docs — GitHub Actions](https://goreleaser.com/ci/actions/) — confirmed @v7 action, fetch-depth: 0, version: '~> v2'
- [Goreleaser blog — reproducible builds](https://goreleaser.com/blog/reproducible-builds/) — confirmed full reproducible builds: block including ldflags
- [goreleaser/goreleaser-action README](https://github.com/goreleaser/goreleaser-action) — confirmed @v7 is latest
- [jedisct1/minisign README + manpage](https://github.com/jedisct1/minisign/blob/master/src/manpage.md) — confirmed `-V -p file -m archive` and `-V -P base64 -m archive` syntax; brew/scoop/choco install methods; -W flag for unencrypted keys; stdin password support

### Secondary (MEDIUM confidence)
- WebSearch (multiple) — confirmed minisign apt package exists in Ubuntu Universe but version may lag (drove pinned-release-tarball recommendation in Pattern 5)
- WebSearch — confirmed `echo PWD | minisign -S ...` is the documented stdin password pattern (cross-checked against issue threads in jedisct1/minisign)

### Tertiary (LOW confidence — flagged for plan-time validation)
- A3 (Assumptions Log) — minisign 0.12 tarball URL pattern. Planner re-verifies at plan time with `gh release list -R jedisct1/minisign`.

## Metadata

**Confidence breakdown:**
- Standard stack: **HIGH** — every config block quoted from goreleaser docs and verified against current major version
- Architecture: **HIGH** — three-pass workflow shape is the natural fit for D-03 (CI-enforced reproducibility); pattern is straightforward
- Pitfalls: **HIGH** — six documented pitfalls cover the surface area the planner will hit when writing six files

**Research date:** 2026-04-28
**Valid until:** ~2026-07-28 (90 days; goreleaser v2 schema and goreleaser-action v7 are stable; minisign is mature; the only fast-moving piece is the minisign tarball URL pinned version which planner re-verifies)

## RESEARCH COMPLETE
