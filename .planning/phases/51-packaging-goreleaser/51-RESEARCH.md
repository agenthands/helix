# Phase 51: packaging-goreleaser - Research

**Researched:** 2026-04-26
**Domain:** Release engineering — goreleaser pipeline + cosign keyless signing on GitHub Actions
**Confidence:** HIGH (cross-verified against goreleaser docs, sigstore docs, and the canonical
`goreleaser/example-supply-chain` reference repo)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Cosign keyless via Sigstore + GitHub OIDC; no private keys in repo secrets.
- **D-02:** Each archive ships with `.sig` + `.pem`; `checksums.txt` is also signed.
- **D-03:** INSTALL.md verify command pins `--certificate-oidc-issuer https://token.actions.githubusercontent.com`
  and `--certificate-identity-regexp '^https://github\.com/postfix/serena/\.github/workflows/release\.yml@refs/tags/v.*$'`.
- **D-04:** `.github/workflows/publish.yml` deleted in same PR.
- **D-05:** No replacement workflow for legacy Python publisher.
- **D-06:** `.github/workflows/docker.yml` not touched; no `dockers:` section in `.goreleaser.yml`.
- **D-07:** Reproducibility check is manual, documented in `RELEASING.md` (or `CONTRIBUTING.md ## Releasing`).
  Maintainer runs `goreleaser release --snapshot --clean` twice and `diff -r` with exclusions.
- **D-08:** No CI reproducibility job.
- **D-09:** Tag pattern `v*` triggers release.
- **D-10:** `release.prerelease: auto`; pre-release tags publish as GitHub pre-release, clean semver as full release.
- **D-11:** `.goreleaser.yml` at repo root. `goreleaser/goreleaser-action@v6` with explicit `version: vX.Y.Z` pin.
- **D-12:** Build matrix locked to `darwin/linux/windows × amd64/arm64` = 6 binaries.
- **D-13:** Build flags: `-trimpath`, `-buildvcs=false`, `CGO_ENABLED=0`,
  `ldflags="-s -w -X main.version={{.Version}} -X main.commit={{.FullCommit}} -X main.date={{.CommitDate}}"`.
- **D-14:** Archive: `.tar.gz` darwin/linux, `.zip` windows. Each archive contains `serena` + `LICENSE` + `README.md` + `INSTALL.md`. Single `checksums.txt` (SHA-256).
- **D-15:** Add `main.version`/`main.commit`/`main.date` package vars in `cmd/serena/main.go`; wire into `--version`. Falls back to `dev`/`2.0.0-dev` for `go build` from source.
- **D-16:** Makefile `build:` target adds equivalent `-ldflags` from `git describe --tags --always`. Don't over-engineer.
- **D-17:** `.github/workflows/release.yml` triggers on `tags: ['v*']`. `goreleaser/goreleaser-action@v6` with `args: release --clean`. Permissions: `contents: write`, `id-token: write`. `packages: write` NOT needed.
- **D-18:** Runs on `ubuntu-latest`, Go `1.25.x`. Single job; goreleaser handles cross-compile via `CGO_ENABLED=0`.
- **D-19:** INSTALL.md gains `## Download a prebuilt binary` and `## Verify a release binary` sections.
- **D-20:** README.md gets a single one-line pointer to INSTALL.md.
- **D-21:** PR success criteria as listed (8 bullets).
- **D-22:** Tagged release dry-run is post-merge ritual, out of scope for the PR.
- **D-23..27:** Out-of-scope reaffirmations (Phase 52, docker.yml untouched, no historical backfill, no runbooks/dashboards, SBOM deferred).
- **D-28:** Two-plan split: 51-01 release pipeline; 51-02 docs.

### Claude's Discretion

- Goreleaser version pin (latest stable at planning time).
- `RELEASING.md` separate file vs. `## Releasing` section in `CONTRIBUTING.md`.
- Exact `--version` output format string.
- Optional `make release-snapshot` / `make release-verify` Makefile targets.
- Release notes template / `release.draft` setting (default `false` is fine).
- Whether archive contains LICENSE/README/INSTALL (recommended; goreleaser auto-includes LICENSE+README anyway — see Q2).

### Deferred Ideas (OUT OF SCOPE)

- SBOM (cyclonedx, syft).
- GoReleaser Pro features.
- macOS notarization / Windows code signing.
- `linux/arm/v7`, `darwin/universal`.
- Reproducibility CI gate.
- Docker absorption into goreleaser.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PKG-01 | Multi-arch (darwin/linux/windows × amd64/arm64) GitHub Releases with SHA-256 checksums + cosign keyless signatures via reproducible goreleaser pipeline | Q1 (cosign keyless on ubuntu-latest), Q2 (archive defaults), Q3 (`{{.Version}}` strips leading `v`), Q4 (`prerelease: auto`), Q5 (`-buildvcs=false` interaction), Q6 (snapshot-diff exclusions), Q7 (workflow permissions). All 7 questions answered with citations below. |
</phase_requirements>

## Summary

Phase 51 lands a textbook 2026 Go release pipeline: a single `release.yml` GitHub Actions workflow on `ubuntu-latest` invokes `goreleaser/goreleaser-action@v6` (CONTEXT.md D-11) which cross-compiles 6 binaries, archives them with auto-included LICENSE/README, generates `checksums.txt`, and signs the **checksums file only** with cosign keyless via OIDC (Fulcio cert + Rekor transparency log). Per-archive `.sig` + `.pem` is achievable but the canonical pattern (used by goreleaser's own example-supply-chain repo and by cosign itself) signs only the checksum, since SHA-256 already binds the archives — D-02 calls for both, which we MUST satisfy by using `artifacts: archive` in addition to `artifacts: checksum` (two `signs:` entries).

The reference exemplar is `goreleaser/example-supply-chain` ([source](https://github.com/goreleaser/example-supply-chain)) — its `.goreleaser.yaml` and `release.yml` were retrieved verbatim and translate almost 1:1 to our locked decisions, with three deltas: (1) we drop the `dockers_v2:` and `docker_signs:` sections (D-06), (2) we drop SBOM (`sboms:`) per D-27, (3) we add `goos: windows` (the example only ships linux/darwin).

**Primary recommendation:** Copy `example-supply-chain/.goreleaser.yaml` minus docker/SBOM, add `goos: [windows]`, add a second `signs:` entry with `artifacts: archive` to satisfy D-02 (per-archive sigs), pin `goreleaser-action@v6.4.0` (latest v6) with explicit `version: ~> v2` for the goreleaser CLI. INSTALL.md verify section uses `--bundle` form (cosign 2.x default) — but D-03 specifies separate `--signature` + `--certificate` flags, which is the older two-file form; we use that form per the locked decision.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Cross-compile 6 binaries | CI (goreleaser) | — | Goreleaser orchestrates `go build` matrix internally; no local build needed |
| Archive + checksums | CI (goreleaser) | — | `archives:` + `checksum:` blocks in `.goreleaser.yml` |
| Keyless signing | CI (goreleaser `signs:` invokes cosign) | Sigstore (Fulcio CA, Rekor TL) | OIDC token from GH Actions → Fulcio short-lived cert → Rekor entry |
| Version metadata | Source (`cmd/serena/main.go` vars) + CI (ldflags injection) | — | Package vars wired via `-X main.version=...` at link time |
| End-user verify | Local (`cosign verify-blob` + `sha256sum -c`) | Sigstore | Documented copy-paste in INSTALL.md |
| Pre-release detection | Goreleaser (`release.prerelease: auto`) | — | Tag suffix `-rc/-beta/-alpha` → GitHub pre-release flag |

## Standard Stack

### Core

| Library / Tool | Version | Purpose | Why Standard |
|----------------|---------|---------|--------------|
| goreleaser CLI | v2.x (latest stable; `~> v2` constraint) | Build/archive/release orchestrator | De facto standard for Go OSS releases [VERIFIED: goreleaser docs] |
| `goreleaser/goreleaser-action` | **v6.4.0** (latest v6 release; v7.1.0 also stable) | GH Actions wrapper | Pinned per D-11 [VERIFIED: github.com/goreleaser/goreleaser-action/releases] |
| cosign CLI | v2.x (no env var needed; keyless is default) | Keyless signing via Sigstore | Standard Sigstore client [VERIFIED: sigstore/cosign CHANGELOG] |
| `sigstore/cosign-installer` | v3.x (latest stable; `@v3.7.0` recent pin) | Installs cosign on runner | Required because goreleaser's `signs:` shells out to `cosign` on PATH [VERIFIED: example-supply-chain release.yml uses `cosign-installer` action] |
| `actions/setup-go` | v5 | Install Go 1.25.x | Already used in `go-test.yml` [VERIFIED: existing workflow] |
| `actions/checkout` | v4 | Checkout source | Standard [VERIFIED: existing workflow] |

**Action version note:** Example-supply-chain uses **v7.1.0** of goreleaser-action (April 2024 release per release page) pinned by SHA. CONTEXT.md D-11 specifies v6 — both are valid; v6.4.0 is the latest v6 minor (August 2023). **Recommendation:** Honor D-11 (v6) but plan should note v7 is also production-ready in case the user wants to bump. Pin by SHA for supply-chain hygiene per the example repo's pattern (`@e24998b8b67b290c2fa8b7c14fcfa7de2c5c9b8c # v7.1.0`).

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| cosign keyless | minisign (PKG-01 wording allows it) | Simpler but no transparency log, requires key custody. Sigstore is the 2026 industry default. |
| cosign keyless `--signature`+`--certificate` two-file form (D-03) | `--bundle` single-file form (cosign 2.x default) | Bundle is one file vs. two; D-03 locks two-file form so users can copy-paste regardless of cosign version. Honor D-03. |
| `goreleaser-action@v6` | `goreleaser-action@v7` | v7 is current; v6 still supported. D-11 locks v6. |

### Installation / Pin

```yaml
# .github/workflows/release.yml — version pins
- uses: actions/checkout@v4
  with:
    fetch-depth: 0  # required for goreleaser to see tags
- uses: actions/setup-go@v5
  with:
    go-version: '1.25.x'
    cache: true
- uses: sigstore/cosign-installer@v3.7.0
- uses: goreleaser/goreleaser-action@v6.4.0
  with:
    distribution: goreleaser
    version: '~> v2'
    args: release --clean
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Version verification commands** the planner should bake into a Wave-0 task or pre-flight check:

```bash
# verify goreleaser version compatibility with our config schema
goreleaser --version    # expect v2.x
goreleaser check        # validates .goreleaser.yml against schema
cosign version          # expect 2.x
```

## Architecture Patterns

### System Architecture Diagram

```
   tag push (v*)
        │
        ▼
┌───────────────────────────────────────────────────────────────────┐
│  GitHub Actions: release.yml (ubuntu-latest, Go 1.25.x)           │
│                                                                   │
│   actions/checkout (fetch-depth:0) ──► setup-go ──► cosign-       │
│                                                     installer     │
│                                                          │        │
│                                                          ▼        │
│   goreleaser-action@v6 ──► goreleaser CLI v2.x                    │
│        │                          │                               │
│        │                          ├─► builds: 6 × `go build`      │
│        │                          │   (CGO_ENABLED=0,             │
│        │                          │    -trimpath, ldflags)        │
│        │                          │                               │
│        │                          ├─► archives: 6 × .tar.gz/.zip  │
│        │                          │   + LICENSE/README (auto)     │
│        │                          │                               │
│        │                          ├─► checksum: checksums.txt     │
│        │                          │                               │
│        │                          └─► signs: cosign sign-blob     │
│        │                              │                           │
│        │                              ▼                           │
│        │              ┌───────────────────────────┐               │
│        │              │ Sigstore (external)       │               │
│        │              │  Fulcio CA: short-lived   │               │
│        │              │   cert from OIDC token    │               │
│        │              │  Rekor: append signature  │               │
│        │              │   to transparency log     │               │
│        │              └───────────────────────────┘               │
│        │                                                          │
│        ▼                                                          │
│   Upload to GitHub Release: 6 archives + checksums.txt +          │
│   per-artifact .sig + .pem (D-02)                                 │
└───────────────────────────────────────────────────────────────────┘
        │
        ▼
   End user: `cosign verify-blob ... && sha256sum -c checksums.txt`
```

### Recommended Project Structure (additions only)

```
.goreleaser.yml             # NEW — at repo root (D-11)
.github/workflows/
  release.yml               # NEW (D-17)
  publish.yml               # DELETED (D-04)
  go-test.yml               # untouched
  docker.yml                # untouched (D-06)
RELEASING.md                # NEW (D-07) — or append to CONTRIBUTING.md
INSTALL.md                  # AMENDED (D-19)
README.md                   # one-line pointer (D-20)
cmd/serena/main.go          # var version, commit, date string (D-15)
internal/cli/root.go        # consume vars in --version handler (D-15)
Makefile                    # add ldflags to build target (D-16)
```

### Pattern: Reference `.goreleaser.yml`

Adapted from [`goreleaser/example-supply-chain/.goreleaser.yaml`](https://github.com/goreleaser/example-supply-chain/blob/main/.goreleaser.yaml) (retrieved verbatim during this research) minus docker/SBOM, plus windows/zip and per-archive sig:

```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema.json
version: 2

project_name: serena

builds:
  - id: serena
    main: ./cmd/serena
    binary: serena
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    flags:
      - -trimpath
      - -buildvcs=false           # D-13 — see Q5 below
    mod_timestamp: "{{ .CommitTimestamp }}"
    ldflags:
      - -s -w
      - -X main.version={{.Version}}      # strips leading 'v' — see Q3
      - -X main.commit={{.FullCommit}}    # full SHA per D-13
      - -X main.date={{.CommitDate}}      # commit date for repro — see Q5

gomod:
  proxy: true                     # ensures consistent module fetch (repro)

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

archives:
  - id: archive
    formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    wrap_in_directory: true
    builds_info:
      mtime: "{{ .CommitTimestamp }}"     # repro: deterministic mtime in archive
    # files: omitted — goreleaser auto-includes LICENSE* and README* (Q2)
    # We MAY add INSTALL.md explicitly per D-14:
    files:
      - LICENSE*
      - README*
      - INSTALL.md

# D-02: sign BOTH the checksum file AND each archive
signs:
  - id: sign-checksum
    cmd: cosign
    artifacts: checksum
    signature: "${artifact}.sig"
    certificate: "${artifact}.pem"
    args:
      - sign-blob
      - "--output-signature=${signature}"
      - "--output-certificate=${certificate}"
      - "${artifact}"
      - "--yes"
    output: true
  - id: sign-archives
    cmd: cosign
    artifacts: archive
    signature: "${artifact}.sig"
    certificate: "${artifact}.pem"
    args:
      - sign-blob
      - "--output-signature=${signature}"
      - "--output-certificate=${certificate}"
      - "${artifact}"
      - "--yes"
    output: true

release:
  prerelease: auto                # Q4 — auto-detects -rc/-beta/-alpha
  draft: false                    # explicit; default
  mode: replace                   # idempotent re-runs against same tag
```

**Key deviation from example-supply-chain:** the example uses `signature: "${artifact}.sigstore.json"` with `--bundle=${signature}` (single bundle file, cosign 2.x preferred form). D-03 mandates the older `--signature` + `--certificate` two-file form so the verify command in INSTALL.md matches the canonical 2-file template most users still copy-paste. We honor D-03.

### Pattern: Reference `release.yml`

Skeleton derived from [example-supply-chain release.yml](https://github.com/goreleaser/example-supply-chain/blob/main/.github/workflows/release.yml) minus docker/QEMU/buildx/syft/attestations:

```yaml
name: release

on:
  push:
    tags: ['v*']

permissions:
  contents: write   # required: create GitHub Release, upload assets
  id-token: write   # required: GitHub OIDC token → Fulcio
  # NOTE: packages: write NOT needed (no docker push) — D-17
  # NOTE: attestations: write NOT needed (no SBOM/provenance) — D-27

jobs:
  release:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # CRITICAL — goreleaser needs full tag history
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true
      - uses: sigstore/cosign-installer@v3.7.0
      - uses: goreleaser/goreleaser-action@v6.4.0
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Anti-Patterns to Avoid

- **`fetch-depth: 1` on checkout:** breaks goreleaser's tag/changelog detection. Always `fetch-depth: 0` for release workflows.
- **Mutating action pin (`@v6` only, no SHA):** acceptable for D-11's "pin to specific minor" but example-supply-chain pins to SHA. Plan should document both options; user chose tag pin.
- **Signing only archives, not checksum:** end users typically verify the checksum file (D-02 covers both — keep both `signs:` entries).
- **Using `--bundle` and also `--signature`/`--certificate`:** mutually exclusive in cosign 2.x. Pick one form. We use the two-file form per D-03.
- **`{{.Date}}` instead of `{{.CommitDate}}` in ldflags:** kills reproducibility (Q5). D-13 already mandates `CommitDate`.
- **Forgetting `mod_timestamp` and `builds_info.mtime`:** archive contents have non-deterministic mtimes without these (Q6).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-compile matrix | bash loop over `GOOS/GOARCH` | goreleaser `builds:` block | Handles ldflags, archives, checksums atomically |
| Tag → release flow | manual `gh release create` | goreleaser `release:` block | Idempotent, handles pre-release detection (Q4) |
| Keyless signing | hand-rolled OIDC + Fulcio HTTP calls | `cosign sign-blob` invoked from `signs:` | cosign handles certificate exchange + Rekor upload + retries |
| Version stamping | git describe in Makefile only | goreleaser ldflags + Makefile fallback (D-15/16) | Both paths converge on same `main.version` var |
| Archive packing | tar/zip in CI script | goreleaser `archives:` | Auto-includes LICENSE/README, deterministic mtime, format overrides per OS |

## Common Pitfalls

### Pitfall 1: Rekor / Fulcio rate-limits and transient outages

**What goes wrong:** Sigstore's public-good infrastructure (Fulcio CA + Rekor transparency log) occasionally rate-limits or 5xx's. Without retry, a release fails partway through signing.

**Why it happens:** Free-tier shared infrastructure; spikes during major release waves.

**How to avoid:**
- cosign 2.x has built-in retry with exponential backoff for Rekor uploads (no flag needed).
- Goreleaser's `signs:` runs each artifact serially per entry — long delays compound. With 6 archives × 2 sig files (D-02) that's 14 cosign calls (6 archives + 1 checksum, each producing .sig + .pem from one call). Budget 30-min job timeout.
- If Sigstore is having an outage, fail loudly — do not retry the whole goreleaser run blindly because partial uploads can leave a half-published GitHub Release. Use `mode: replace` in `release:` so a re-run cleans state.
- `--tlog-upload=true` is the cosign 2.x default; do not disable it (transparency is the whole point).

**Warning signs:** Job log shows `failed to upload to rekor` or `429 Too Many Requests` from `rekor.sigstore.dev`.

**Sources:** [sigstore/cosign CHANGELOG](https://github.com/sigstore/cosign/blob/main/CHANGELOG.md), [Sigstore FAQ](https://docs.sigstore.dev/about/faq/) [VERIFIED]

### Pitfall 2: `fetch-depth: 1` defaults

**What goes wrong:** `actions/checkout` defaults to shallow clone. Goreleaser then can't read tag history, fails with `git describe`-style errors or generates wrong version.

**How to avoid:** Always `fetch-depth: 0` in release workflows. The example-supply-chain release.yml has an inline comment specifically warning about this.

### Pitfall 3: cosign env-var deprecations (cosign 2.x)

**What goes wrong:** Old tutorials reference `COSIGN_EXPERIMENTAL=1`. cosign 2.x removed it; keyless is now default.

**How to avoid:** Don't set `COSIGN_EXPERIMENTAL`. Don't set `--oidc-issuer` explicitly in `args:` — cosign auto-detects GitHub Actions from env vars. Adding it doesn't hurt but is redundant.

**Sources:** [cosign CHANGELOG removal of `COSIGN_EXPERIMENTAL`](https://github.com/sigstore/cosign/blob/main/CHANGELOG.md) [VERIFIED]

### Pitfall 4: Snapshot output is not byte-identical to release output

**What goes wrong:** Maintainer runs `--snapshot --clean` twice, diffs `dist/`, sees diff, panics. Some files are inherently non-reproducible (Q6 lists them). The repro check must EXCLUDE these.

**How to avoid:** Document the exclusion list in RELEASING.md (Q6 below).

### Pitfall 5: `go install`-built binaries report `dev` forever

**What goes wrong:** Users who `go install` Serena get `2.0.0-dev` because ldflags only fire under goreleaser/Makefile. Confusing in bug reports.

**How to avoid:** D-15 covers this — accepted tradeoff. Document in INSTALL.md that `--version` accuracy requires a release binary.

### Pitfall 6: `--certificate-identity-regexp` mismatch on tag format

**What goes wrong:** D-03 pins identity to `refs/tags/v.*` — works for `v1.9.0`, `v1.9.0-rc1`. But if anyone tags without the leading `v` (e.g., `1.9.0`), the regex fails and verify panics. D-09 locks tag pattern to `v*` so this stays consistent — but emphasize in INSTALL.md that the regex is paired to our tag policy.

## Validation Architecture

> Phase requires Nyquist validation (no override in config); phase requirements PKG-01.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | None native — pipeline validation is **manual + tooling** (`goreleaser check`, `goreleaser release --snapshot --clean`, `cosign verify-blob`) |
| Config file | `.goreleaser.yml` (validated by `goreleaser check`) |
| Quick run command | `goreleaser check` (schema validation, < 1s) |
| Full suite command | `goreleaser release --snapshot --clean` (full local pipeline run, ~30-90s on M-series mac) |
| Phase gate | All 8 D-21 PR success criteria green |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| PKG-01 | `.goreleaser.yml` is schema-valid | tooling | `goreleaser check` | Created in 51-01 |
| PKG-01 | 6 binaries built locally | tooling (snapshot) | `goreleaser release --snapshot --clean && ls dist/serena_*/serena*` | — |
| PKG-01 | `checksums.txt` generated with SHA-256 | tooling (snapshot) | `goreleaser release --snapshot --clean && wc -l dist/checksums.txt` (expect 6) | — |
| PKG-01 | Each archive has `.sig` + `.pem` | tooling (snapshot, requires GH Actions for OIDC — fails locally without `COSIGN_PASSWORD`/identity) | manual: `find dist -name '*.sig' \| wc -l` (expect 7: 6 archives + checksums) | — |
| PKG-01 | `--version` reflects ldflags | unit | `make build && ./serena --version \| grep -E 'serena version [0-9]+\.[0-9]+\.[0-9]+'` | Wave 0 — needs `cmd/serena/main.go` vars |
| PKG-01 | Reproducibility (modulo metadata + sigs) | manual ritual | `goreleaser release --snapshot --clean -o /tmp/dist1 && goreleaser release --snapshot --clean -o /tmp/dist2 && diff -r --exclude=...` | Documented in RELEASING.md |
| PKG-01 | INSTALL.md verify command runs end-to-end | **manual** (post-tag, requires real release artifact) | `cosign verify-blob --certificate-oidc-issuer ... --certificate-identity-regexp ... --signature x.sig --certificate x.pem x` | Out of PR scope per D-22 |
| PKG-01 | `publish.yml` deleted | grep | `! test -f .github/workflows/publish.yml` | — |
| PKG-01 | `docker.yml` byte-identical | grep | `git diff main -- .github/workflows/docker.yml` empty | — |

### Sampling Rate

- **Per task commit:** `goreleaser check` (< 1s).
- **Per wave merge:** `goreleaser release --snapshot --clean` (local; signing fails locally without keyless OIDC — that's expected; the binaries+archives+checksum still build).
- **Phase gate:** All 8 D-21 success criteria; full suite green; `go vet ./...` and `go test ./...` pass (CLAUDE.md mandate).
- **Post-merge ritual (out of PR):** Maintainer creates throwaway tag (e.g. `v0.0.0-test1`) on a fork or branch, observes real release.yml run, downloads one artifact, runs the INSTALL.md verify command end-to-end, then deletes the test tag/release.

### Wave 0 Gaps

- [ ] `cmd/serena/main.go` — declare `var (version, commit, date = "dev", "none", "unknown")` package-level vars. Currently the file just calls `cli.NewRootCommand().Execute()` (verified: 16 lines, no version vars).
- [ ] `internal/cli/root.go` — replace hardcoded `"serena version 2.0.0-dev"` (line 62) with formatted output consuming `main.version`/`main.commit`/`main.date`. Will need an exported setter or import path because `internal/cli` cannot import `main`. **Plan must address this** — recommended pattern: declare vars in `internal/cli` (e.g., `cli.Version`, `cli.Commit`, `cli.Date`) and ldflag `-X github.com/postfix/serena/internal/cli.Version=...`. This is the standard cobra+goreleaser pattern (cited: [jvt.me cobra goreleaser version post](https://www.jvt.me/posts/2023/02/27/go-cobra-goreleaser-version/)).
- [ ] `Makefile` `build:` target — currently `$(GO) build -o $(BINARY) ./cmd/serena`. Needs `-ldflags` from `git describe --tags --always` (D-16).
- [ ] `.goreleaser.yml` — net-new file at repo root.
- [ ] `.github/workflows/release.yml` — net-new file.
- [ ] `RELEASING.md` (or `CONTRIBUTING.md ## Releasing`) — net-new content.
- [ ] `INSTALL.md` — two new sections (D-19).

## Answers to the 7 Research Questions

### Q1. Cosign keyless gotchas on `ubuntu-latest` (Rekor rate-limits, Fulcio outages, retry/backoff)

**Answer:** No special goreleaser flag is required. cosign 2.x has built-in retry/backoff for Rekor and is the default signing mode (no `COSIGN_EXPERIMENTAL` needed). Known operational gotchas:

1. **Sigstore public-good infrastructure occasionally 5xx's / 429s** during release spikes — cosign retries internally; goreleaser will surface the eventual failure if retries exhaust. There is no goreleaser-level retry knob for `signs:`.
2. **Rekor uploads cannot be silenced** in cosign 2.x default config (`--tlog-upload=true` is default; `--tlog-upload=false` exists but defeats the purpose).
3. **Job timeout** — with 6 archives + 1 checksum × 2 signs (D-02 = both archive and checksum signed), expect 14+ cosign network calls. Set workflow timeout to 30 min (the example-supply-chain doesn't set an explicit timeout but their job is similarly sized).
4. **OIDC token scoping** — `id-token: write` at job level is sufficient; do not put it at workflow level (least-privilege).
5. **No special args needed** — `--oidc-issuer` is auto-detected from GH Actions env (`ACTIONS_ID_TOKEN_REQUEST_URL`). Adding it explicitly is harmless.

**Source:** [Sigstore Quickstart](https://docs.sigstore.dev/quickstart/quickstart-cosign/), [cosign CHANGELOG](https://github.com/sigstore/cosign/blob/main/CHANGELOG.md), [example-supply-chain release.yml](https://github.com/goreleaser/example-supply-chain/blob/main/.github/workflows/release.yml). [VERIFIED via WebFetch of canonical refs]

**Confidence:** HIGH.

### Q2. Goreleaser default `archives:` content — does it auto-include LICENSE/README?

**Answer:** **YES.** When `files:` is empty/absent in `archives:`, goreleaser automatically includes:

```
LICENSE*, README*, CHANGELOG, license*, readme*, changelog
```

(Quoted from goreleaser docs.) This is enough to satisfy the LICENSE+README portion of D-14. To also include `INSTALL.md` (which D-14 specifies), an explicit `files:` block IS required — but adding `files:` makes you responsible for re-adding LICENSE/README explicitly because once the list is non-empty, the auto-include disengages. Recommended `files:` for our case:

```yaml
files:
  - LICENSE*
  - README*
  - INSTALL.md
```

**Source:** [goreleaser archives docs](https://goreleaser.com/customization/archive/) — quoted defaults verbatim. [VERIFIED]

**Confidence:** HIGH.

### Q3. `{{.Version}}` vs `{{.Tag}}` — which strips the leading `v`?

**Answer:** `{{.Version}}` **strips** the leading `v`. `{{.Tag}}` is the raw git tag.

For tag `v1.9.0`:
- `{{.Version}}` → `1.9.0`
- `{{.Tag}}` → `v1.9.0`

D-13 already specifies `{{.Version}}` which is correct for our desired output `serena version 1.9.0 ...`.

**Source:** [goreleaser cookbook: main.version ldflag](https://goreleaser.com/cookbooks/using-main.version/) — quoted: "By default, GoReleaser will set the following 3 ldflags: main.version with the current Git tag (the v prefix is stripped) or the name of the snapshot, if you're using the --snapshot flag." [VERIFIED]

**Confidence:** HIGH.

### Q4. `prerelease: auto` exact behavior — does it match `-rc.1`, `-rc1`, `-beta.1`, `-alpha.2`?

**Answer:** **Effectively yes** — `prerelease: auto` treats any tag whose semver build/prerelease segment (anything after `-`) is non-empty as a pre-release. The docs say: *"If set to auto, will mark the release as not ready for production in case there is an indicator for this in the tag e.g. v1.0.0-rc1."* The implementation reads the semver prerelease component, so:

- `v1.9.0-rc.1` → prerelease ✓
- `v1.9.0-rc1` → prerelease ✓
- `v1.9.0-beta.1` → prerelease ✓
- `v1.9.0-alpha.2` → prerelease ✓
- `v1.9.0` → full release ✓

[CITED: https://goreleaser.com/customization/release/]
[VERIFIED: behavior corroborated by issue tracker — github.com/goreleaser/goreleaser/issues/667 ("GitHub pre-releases on alpha, beta, rc versions")]

**Caveat ([ASSUMED, low risk]):** The exact regex/code path is not explicitly enumerated in the docs we retrieved. The behavior is consistently described in the issue history but the planner should add a **post-merge verification step**: tag a throwaway `v0.0.0-rc0` on a test branch and confirm GitHub marks it pre-release. This is part of D-22's post-merge ritual anyway.

**Confidence:** MEDIUM-HIGH (docs + issue history agree; haven't read source).

### Q5. `-buildvcs=false` interaction with goreleaser's own VCS injection

**Answer:** They do not collide. `-buildvcs=false` is a Go toolchain flag that suppresses the `runtime/debug.BuildInfo`'s VCS metadata embedded by `go build` (Go 1.18+). Goreleaser's "VCS injection" is a different mechanism — it injects values via `-X` ldflags into your `main.version`/`main.commit`/`main.date` package vars. The two are independent:

- `-buildvcs=false` → strips `vcs.revision`, `vcs.time`, `vcs.modified` from `runtime/debug.ReadBuildInfo()`.
- Goreleaser ldflags → set `main.version`/`main.commit`/`main.date` package vars.

You CAN use both. Disabling `-buildvcs` is recommended for reproducibility because the VCS time embedded by Go can drift if the working tree's index file mtime changes between snapshot runs (which it can, e.g., via `git status` between runs).

**Sources:**
- [Reproducible builds blog post](https://goreleaser.com/blog/reproducible-builds/) — quoted: "Go 1.18+ embeds information about the current checkout directory of your code, including modified and new files. In some cases this interferes with reproducibility. You can turn this off using the -buildvcs=false flag." [VERIFIED]
- [Carlos Becker repro post](https://carlosbecker.com/posts/goreleaser-reproducible-buids/) [CITED]

**Confidence:** HIGH.

### Q6. Snapshot diff exclusions — full list of metadata files goreleaser emits

**Answer:** When you run `goreleaser release --snapshot --clean`, `dist/` contains:

| File / Pattern | Reproducible? | Diff action |
|----------------|---------------|-------------|
| `dist/<name>_<os>_<arch>_<v1>/<binary>` | YES (with our flags) | INCLUDE in diff |
| `dist/<name>_<os>_<arch>.tar.gz` / `.zip` | YES (with `mod_timestamp` + `builds_info.mtime`) | INCLUDE in diff |
| `dist/checksums.txt` | YES (deterministic, sha256 of archives) | INCLUDE in diff |
| `dist/artifacts.json` | NO (timestamps, run IDs) | **EXCLUDE** |
| `dist/metadata.json` | NO (run timestamp) | **EXCLUDE** |
| `dist/config.yaml` | YES (effective resolved config) | usually INCLUDE; safe either way |
| `dist/*.sig`, `dist/*.pem` | NO (each cosign signature is unique even for identical content) | **EXCLUDE** |
| `dist/checksums.txt.sig`, `dist/checksums.txt.pem` | NO | **EXCLUDE** |

**Canonical RELEASING.md diff command:**

```bash
goreleaser release --snapshot --clean
mv dist /tmp/dist1
goreleaser release --snapshot --clean
mv dist /tmp/dist2
diff -r \
  --exclude='*.sig' \
  --exclude='*.pem' \
  --exclude='artifacts.json' \
  --exclude='metadata.json' \
  /tmp/dist1 /tmp/dist2
# expected: empty diff (modulo signatures, per D-07)
```

D-07 already lists `--exclude='checksums.txt*'` — that's defensive (it's only needed if you sign the checksum, in which case the `.sig`/`.pem` exclusions catch it; the bare `checksums.txt` IS reproducible). Keep D-07's exclusion list as-written; it's a superset and harmless.

**Sources:**
- [goreleaser snapshots docs](https://goreleaser.com/customization/snapshots/) [VERIFIED]
- [goreleaser artifacts.json docs](https://goreleaser.com/customization/general/artifacts/) [VERIFIED]
- WebSearch confirmed `dist/` contains `artifacts.json`, `metadata.json`, `config.yaml` plus build outputs [VERIFIED via WebSearch result citing goreleaser docs]

**Confidence:** HIGH for the file list; MEDIUM for `config.yaml` reproducibility (depends on goreleaser version embedding a timestamp; safest to exclude).

### Q7. Workflow permissions least-privilege for `id-token: write` + cosign keyless + GitHub Release

**Answer:** Minimum permissions block for `release.yml`:

```yaml
permissions:
  contents: write   # create release, upload assets
  id-token: write   # request OIDC token from GitHub for cosign keyless
```

That is it. NOT needed:

- `packages: write` — only for ghcr.io / docker push, which D-06 excludes.
- `attestations: write` — only for `actions/attest-build-provenance`, which D-27 defers.
- `pull-requests: write` — only for changelog-related actions, not used here.

The example-supply-chain release.yml has `packages: write` and `attestations: write` because it pushes docker images and generates SBOM provenance — both out of scope for Phase 51.

**Refinement (best practice):** Set `permissions:` at JOB level rather than workflow level so other potential jobs don't inherit `id-token: write`. Single-job workflow makes this moot; either placement is fine.

**Sources:**
- [GitHub Docs: Controlling permissions for GITHUB_TOKEN](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/controlling-permissions-for-github_token) [VERIFIED]
- [Chainguard: Zero-friction keyless signing with GitHub Actions](https://www.chainguard.dev/unchained/zero-friction-keyless-signing-with-github-actions) [CITED]
- [example-supply-chain release.yml](https://github.com/goreleaser/example-supply-chain/blob/main/.github/workflows/release.yml) — `packages: write` and `attestations: write` ARE needed only because that example does docker+SBOM. Confirms our minimal set is correct. [VERIFIED]

**Confidence:** HIGH.

## Reference Exemplars

### 1. `goreleaser/example-supply-chain` — canonical maintained reference

[Repo](https://github.com/goreleaser/example-supply-chain) | [.goreleaser.yaml retrieved verbatim] | [release.yml retrieved verbatim]

This is the official goreleaser-published example for keyless signing. Strip docker + SBOM, add windows, and you have ~80% of our `.goreleaser.yml`. Strip docker login + QEMU/buildx + syft + attestations, and you have ~90% of our `release.yml`. Pinned action versions in their release.yml (April 2024):

- `actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2`
- `actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0`
- `sigstore/cosign-installer@cad07c2e89fa2edd6e2d7bab4c1aa38e53f76003 # v4.1.1`
- `goreleaser/goreleaser-action@e24998b8b67b290c2fa8b7c14fcfa7de2c5c9b8c # v7.1.0`

Note: their `cosign-installer` is pinned to a v4.x SHA. The action `sigstore/cosign-installer@v3.x` is the more widely-cited modern tag (v3.7.0 is recent). The example using v4.x suggests `cosign-installer` has moved to v4 as of mid-2024; planner should verify latest at implementation time.

**Differences vs. our locked decisions:**

| Their setting | Our decision | Rationale |
|---------------|--------------|-----------|
| Includes `dockers_v2:` + `docker_signs:` | omitted | D-06 |
| Includes `sboms:` | omitted | D-27 |
| Builds linux+darwin only | adds windows | PKG-01 / D-12 |
| Signs with `--bundle` (single .sigstore.json) | uses `--signature` + `--certificate` (two files) | D-03 — preserves canonical 2-file verify command |
| `goreleaser-action@v7` | `@v6` | D-11 — user choice; v7 also valid |

### 2. `sigstore/cosign` — its own release pipeline

Cosign signs its own releases. Their `.goreleaser.yml` `signs:` block (retrieved via WebFetch) shows the **pattern of multiple `signs:` entries selecting different `artifacts:` selectors** (binary, checksum, package), each producing a separate `.sigstore.json` bundle. This validates our approach of using two `signs:` entries (one `artifacts: archive`, one `artifacts: checksum`) per D-02. Their `archives:` uses `formats: [binary]` (raw binary, no tar) which is NOT what we want — we want the tar.gz/zip per D-14.

[Source: github.com/sigstore/cosign/.goreleaser.yml — retrieved via WebFetch; signs: block reproduced in this research's WebFetch evidence]

### 3. Pattern for cobra + goreleaser version wiring

[Jamie Tanna: "Getting a `--version` flag for Cobra CLIs in Go, built with GoReleaser"](https://www.jvt.me/posts/2023/02/27/go-cobra-goreleaser-version/)

Standard pattern: declare `Version`, `Commit`, `Date` package vars in the `cli` package (not `main`, because `internal/cli` cannot import `main`), then `-X github.com/<org>/<repo>/internal/cli.Version=...`. Cobra's `cmd.Version = cli.Version` slot can also be used. **Plan 51-01 should choose this approach** — the existing `runRoot` already handles `--version` manually so we don't need cobra's built-in. Keep current handler, replace the hardcoded literal with `fmt.Sprintf("serena version %s (commit %s, built %s)\n", Version, ShortCommit(Commit), Date)`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `COSIGN_EXPERIMENTAL=1` for keyless | Keyless is default in cosign 2.x | cosign 2.0.0 (mid-2023) | Don't set the env var; ignore old tutorials |
| GPG / minisign for OSS releases | Sigstore keyless | 2022-2024 industry shift | Verify command is `cosign verify-blob ...`, not `gpg --verify` |
| Manual `gh release create` + `cosign sign-blob` in shell | goreleaser `signs:` block | Goreleaser 1.x → 2.x | Single source of truth for release config |
| `--signature` + `--certificate` two-file form | `--bundle=...sigstore.json` single bundle | cosign 2.x | Bundle is preferred but two-file is still supported and more familiar in docs; D-03 picks two-file |
| goreleaser v1 schema | goreleaser v2 schema (`version: 2` header) | Mid-2024 (v2.0 release announcement) | Use `version: 2` in `.goreleaser.yml` — see [Announcing GoReleaser v2](https://goreleaser.com/blog/goreleaser-v2/) |

**Deprecated/outdated:**

- `archives.format: tar.gz` (singular) — replaced by `archives.formats: [tar.gz]` (plural list) in v2 schema. Use the new form. (Visible in example-supply-chain.)
- `archives.format_overrides[].format` (singular) → `archives.format_overrides[].formats: [zip]` (plural). Same v2 migration.
- `dockers:` (v1) → `dockers_v2:` (v2 schema) — irrelevant for us (D-06).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `prerelease: auto` matches all four prerelease forms (`-rc.1`, `-rc1`, `-beta.1`, `-alpha.2`) consistently | Q4 | Low — if one form misses, GH publishes as full release instead of pre-release; cosmetic, not security. Mitigate via D-22 post-merge ritual. |
| A2 | `config.yaml` in dist/ is reproducible across snapshot runs | Q6 | Low — listed as "either way" in our exclude list; safest to exclude defensively. |
| A3 | `cosign-installer@v3.7.0` is current at implementation time | Standard Stack | Low — verify latest tag at implementation time; v3.x and v4.x both work. |

If any locked decision (D-01..D-28) ends up resting on one of these assumptions in a way that's load-bearing, surface for user confirmation in `/gsd-discuss-phase`. As written, none of the assumptions block the plan.

## Open Questions

1. **Should `cli.Version`/`cli.Commit`/`cli.Date` live in `internal/cli` or a new `internal/version` package?**
   - Recommendation: `internal/cli` is fine (already consumed by `runRoot`). Keep diff small.
   - Alternative: new `internal/version` package if you anticipate other consumers (telemetry, MCP `serverInfo` response). Defer until needed.

2. **Should Makefile `build:` keep the existing simple form and add a separate `make release-snapshot` / `make release-verify` target instead of mutating `build:`?**
   - User left this to Claude's discretion. Recommendation: KEEP `build:` simple (no `-ldflags`), add THREE targets:
     - `make build` — simple `go build`, falls back to `dev` version (current behavior, preserves muscle memory).
     - `make build-versioned` — adds `-ldflags` from `git describe --tags --always`. New.
     - `make release-snapshot` — wraps `goreleaser release --snapshot --clean`.
     - `make release-verify` — wraps the diff exclude command from Q6.
   - This keeps `make build` cheap and discoverable while making the maintainer ritual one-line per D-16.

3. **Does cosign 2.x's automatic Rekor upload work without any flag in the goreleaser-spawned cosign process?** — Strong YES from docs; verify in 51-01 by inspecting first snapshot's behavior locally (cosign will still try to upload to Rekor but will fail without an OIDC token; that's fine for snapshot validation).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `goreleaser` CLI | local snapshot validation, `goreleaser check` | TBD on dev machine — verify via `command -v goreleaser` | needs v2.x | `go install github.com/goreleaser/goreleaser/v2@latest` or `brew install goreleaser` |
| `cosign` CLI | local sign smoke test (will fail without OIDC, that's fine) | TBD | v2.x | `brew install cosign` or `go install github.com/sigstore/cosign/v2/cmd/cosign@latest` |
| Go | build | ✓ (already 1.25.x per Phase 50) | 1.25.x | — |
| `git` | tag inspection | ✓ | — | — |
| GitHub Actions runner (`ubuntu-latest`) | release pipeline | ✓ on push | — | — |
| Sigstore Fulcio + Rekor | keyless signing at tag time | ✓ public-good infra | — | None — Phase 51 commits to keyless |

**Missing dependencies with no fallback:** None blocking the PR. (Local goreleaser/cosign install is a Wave-0 task; the PR can land without them on the dev machine because GH Actions does the real work — D-22.)

**Missing dependencies with fallback:** N/A.

## Project Constraints (from CLAUDE.md)

These directives must be honored by every plan in Phase 51:

- **Run `go vet ./...` and `go test ./...` before completing any Go task** — applies to plan 51-01 (cmd/serena/main.go and internal/cli/root.go edits).
- **Use `go build ./cmd/serena` / `make build` to build** — Makefile changes (D-16) must keep `make build` working.
- **GSD workflow enforced** — all file edits via plan execution, not direct.
- **Single Go binary, no Python/Docker/runtime deps in shipping artifact** (PROJECT.md core value referenced from CLAUDE.md) — drives the deletion of publish.yml (D-04) and the no-docker stance (D-06). Research has confirmed both align with the canonical example-supply-chain pattern minus their docker/SBOM additions.
- **Legacy reference is read-only** — Phase 51 must not touch `legacy/` for any Python concern. D-05 already enforces this.

## Sources

### Primary (HIGH confidence)

- [goreleaser docs — archives](https://goreleaser.com/customization/archive/) — auto-include defaults
- [goreleaser docs — sign](https://goreleaser.com/customization/sign/) — keyless signs: block syntax, artifact selectors
- [goreleaser docs — release](https://goreleaser.com/customization/release/) — `prerelease: auto`
- [goreleaser docs — snapshots](https://goreleaser.com/customization/snapshots/) — snapshot semantics
- [goreleaser docs — main.version cookbook](https://goreleaser.com/cookbooks/using-main.version/) — `{{.Version}}` strips `v`
- [goreleaser blog — reproducible builds](https://goreleaser.com/blog/reproducible-builds/) — `-trimpath`, `mod_timestamp`, `CommitDate`
- [goreleaser blog — Announcing GoReleaser v2](https://goreleaser.com/blog/goreleaser-v2/) — v2 schema
- [goreleaser/example-supply-chain — .goreleaser.yaml](https://github.com/goreleaser/example-supply-chain/blob/main/.goreleaser.yaml) — canonical exemplar [retrieved verbatim]
- [goreleaser/example-supply-chain — release.yml](https://github.com/goreleaser/example-supply-chain/blob/main/.github/workflows/release.yml) — canonical exemplar [retrieved verbatim]
- [sigstore/cosign — CHANGELOG](https://github.com/sigstore/cosign/blob/main/CHANGELOG.md) — `COSIGN_EXPERIMENTAL` removal, keyless default
- [Sigstore docs — Quickstart](https://docs.sigstore.dev/quickstart/quickstart-cosign/)
- [Sigstore docs — verify-blob](https://docs.sigstore.dev/cosign/verifying/verify/)
- [GitHub Docs — Controlling permissions for GITHUB_TOKEN](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/controlling-permissions-for-github_token)

### Secondary (MEDIUM confidence)

- [Carlos Becker — Reproducible builds with GoReleaser](https://carlosbecker.com/posts/goreleaser-reproducible-buids/)
- [Chainguard — Zero-friction keyless signing](https://www.chainguard.dev/unchained/zero-friction-keyless-signing-with-github-actions)
- [Jamie Tanna — Cobra + GoReleaser version flag](https://www.jvt.me/posts/2023/02/27/go-cobra-goreleaser-version/)
- [goreleaser/goreleaser-action releases page](https://github.com/goreleaser/goreleaser-action/releases) — v6.4.0 latest v6 (Aug 2023), v7.1.0 latest v7 (Apr 2024)
- [github.com/goreleaser/goreleaser/issues/667](https://github.com/goreleaser/goreleaser/issues/667) — prerelease auto behavior

### Tertiary (LOW confidence — flag for validation)

- Exact regex used by `prerelease: auto` (Q4 / A1) — docs imply standard semver `-` indicator behavior; unverified in source.

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — versions verified via release pages and example repo
- Architecture: HIGH — pattern is well-established (goreleaser/example-supply-chain is the literal documentation example)
- Pitfalls: HIGH — surveyed via cosign CHANGELOG, sigstore docs, blog posts
- Q1-Q3, Q5, Q6, Q7: HIGH
- Q4 (`prerelease: auto` regex): MEDIUM-HIGH — behavior consistent in docs and issues, source unverified

**Research date:** 2026-04-26
**Valid until:** 2026-05-26 (30 days; goreleaser v2 schema is stable, cosign 2.x is stable, no anticipated breaking changes)
