# Phase 51: packaging-goreleaser - Context

**Gathered:** 2026-04-26
**Status:** Ready for research / planning

<domain>
## Phase Boundary

Tagging a stable or pre-release version (`v*`) triggers a goreleaser CI workflow
that builds 6 reproducible Go binaries (darwin/linux/windows × amd64/arm64),
publishes them to a GitHub Release with a SHA-256 `checksums.txt`, and signs
each artifact with **cosign keyless** via GitHub OIDC. `INSTALL.md` documents
how a user verifies a downloaded binary's signature and checksum in one
terminal session. The pipeline is reproducible — `goreleaser release --snapshot`
invoked twice on the same commit produces byte-identical archives modulo
signatures.

Phase 51 ships only the binary-release pipeline. Homebrew, Scoop, and Linux
native packages are Phase 52. Docker images continue to be handled by the
existing `docker.yml` workflow — Phase 51 does not touch docker.
</domain>

<canonical_refs>
## Canonical References

- `.planning/REQUIREMENTS.md` — PKG-01 (this phase), PKG-02..04 (Phase 52, downstream)
- `.planning/ROADMAP.md` — Phase 51 entry (success criteria 1–4)
- `.planning/PROJECT.md` — Single-binary, no Python/Docker/runtime deps in shipping artifact
- `.planning/phases/50-toolchain-go1.25-gopls-ci/50-CONTEXT.md` — D-A2 "honest signal, no half-measures" precedent (informs publish.yml deletion below)
- `.github/workflows/go-test.yml` — existing CI (untouched by this phase)
- `.github/workflows/publish.yml` — legacy Python publisher (deleted by this phase)
- `.github/workflows/docker.yml` — existing ghcr.io publisher (untouched by this phase)
- `INSTALL.md` — to be amended with "Download a prebuilt binary" + "Verify a release binary" sections
- `cmd/serena/main.go`, `internal/cli/root.go` — currently hardcodes `"serena version 2.0.0-dev"`; needs `main.version`/`main.commit`/`main.date` ldflag scaffolding
- `Makefile` — phase MAY add `make release-snapshot` / `make release-verify` targets (Claude's discretion)
- External: <https://goreleaser.com/customization/builds/go/> (build customization), <https://goreleaser.com/customization/sign/> (cosign signing), <https://docs.sigstore.dev/cosign/signing/signing_with_blobs/> (cosign keyless workflow), <https://docs.sigstore.dev/cosign/verifying/verify/> (verify-blob with `--certificate-identity-regexp`)
</canonical_refs>

<decisions>
## Implementation Decisions

### Signing
- **D-01:** Signing tool is **cosign keyless** via Sigstore. The release workflow obtains a short-lived signing certificate from Fulcio using the GitHub Actions OIDC token; signatures are recorded in the Rekor transparency log. No private keys stored in repo secrets.
- **D-02:** Each of the 6 binary archives gets a paired `.sig` and `.pem` (cert) file uploaded to the GitHub Release alongside `checksums.txt`. The `checksums.txt` itself is also signed (`checksums.txt.sig` + `checksums.txt.pem`).
- **D-03:** `INSTALL.md` "Verify a release binary" section pins verification to the canonical issuer + identity:
  - `--certificate-oidc-issuer https://token.actions.githubusercontent.com`
  - `--certificate-identity-regexp '^https://github\.com/postfix/serena/\.github/workflows/release\.yml@refs/tags/v.*$'`
  - This wording is mandatory in INSTALL.md so end-users have a copy-pasteable command.

### Legacy Python publisher
- **D-04:** `.github/workflows/publish.yml` is **deleted** in the same PR as Phase 51's other changes. Rationale: Python is read-only legacy reference per PROJECT.md; the existing TestPyPI publisher contradicts the single-Go-binary product framing. Mirrors the Phase 50 D-A2 precedent — honest signal, no half-measures.
- **D-05:** Anyone needing a final legacy Python artifact can build from `legacy/` source. No replacement workflow, no manual-dispatch fallback.

### Docker
- **D-06:** `.github/workflows/docker.yml` is **not touched** by Phase 51. The goreleaser config has no `dockers:` section. Phase 51 ships binaries + checksums + signatures only. If the docker contract needs revisiting, file as a Phase 52 follow-up or a separate phase.

### Reproducibility verification
- **D-07:** Reproducibility check is **manual, documented in `RELEASING.md`** (new file at repo root, or alternatively a `## Releasing` section appended to `CONTRIBUTING.md` — Claude's discretion). The maintainer runs `goreleaser release --snapshot --clean` twice and compares the two `dist/` trees with `diff -r --exclude='*.sig' --exclude='*.pem' --exclude='checksums.txt*' --exclude='artifacts.json' --exclude='metadata.json' dist1 dist2`. Pre-tag ritual; not enforced by CI.
- **D-08:** No CI reproducibility job. Same precedent as Phase 50's local-bench (D-A2/D-A4): an honest signal — if the maintainer skips it, the missing ritual is visible, not silently bypassed.

### Tag / trigger policy
- **D-09:** Workflow trigger pattern: `tags: ['v*']`. Both stable (`v1.9.0`) and pre-release tags (`v1.9.0-rc1`, `v2.0.0-beta.1`, `v1.0.0-alpha.2`) fire the same `release.yml` workflow.
- **D-10:** Goreleaser uses `release.prerelease: auto` so any tag containing `-rc`, `-beta`, or `-alpha` publishes as a GitHub **pre-release**; clean semver tags publish as a full release. Single workflow, single goreleaser config — release type is derived from the tag suffix.

### Goreleaser config & build flags
- **D-11:** Config file lives at repo root: `.goreleaser.yml` (yaml, not yml/yaml split). Goreleaser version pinned in the workflow (e.g., `goreleaser/goreleaser-action@v6` with `version: vX.Y.Z`) to keep the pipeline reproducible across config drift.
- **D-12:** Build matrix locked to PKG-01: `goos: [darwin, linux, windows]` × `goarch: [amd64, arm64]` = 6 binaries. No `linux/arm/v7`, no `freebsd`, no `darwin/universal`.
- **D-13:** Build flags (locked, drives reproducibility): `-trimpath`, `-buildvcs=false`, `CGO_ENABLED=0`, `ldflags="-s -w -X main.version={{.Version}} -X main.commit={{.FullCommit}} -X main.date={{.CommitDate}}"`. Note `CommitDate` (not `Date`) for byte-identical re-runs on the same commit — `Date` injects build wall-clock time which breaks reproducibility.
- **D-14:** Archive format follows goreleaser defaults: `.tar.gz` for darwin/linux, `.zip` for windows. Each archive includes the `serena` binary plus `LICENSE`, `README.md`, `INSTALL.md`. Single combined `checksums.txt` (SHA-256) per release.

### Source-side scaffolding
- **D-15:** Currently `internal/cli/root.go` hardcodes `"serena version 2.0.0-dev"`. Phase 51 introduces a `main.version` / `main.commit` / `main.date` linker-flag scaffold (var declarations in `cmd/serena/main.go` consumed by the version handler). The `--version` output becomes `serena version <version> (commit <short-sha>, built <date>)` for tagged builds; falls back to `dev` (or current `2.0.0-dev`) for `go install` / `go build` from source.
- **D-16:** `Makefile`'s `build:` target should also pass an equivalent `-ldflags` block populated from `git describe --tags --always` so locally-built binaries report a meaningful version. Claude's discretion on exact wording; do not over-engineer.

### Workflow file
- **D-17:** New file: `.github/workflows/release.yml`. Triggers on tag push matching `v*`. Uses `goreleaser/goreleaser-action@v6` with `args: release --clean`. Permissions: `contents: write` (for the GitHub Release), `id-token: write` (for OIDC → Sigstore), `packages: write` is NOT needed (no docker push from this workflow).
- **D-18:** Workflow runs on `ubuntu-latest` with Go `1.25.x` (matches Phase 50's TOOL-01 baseline). Single job; no matrix split — goreleaser handles cross-compilation internally with `CGO_ENABLED=0`.

### INSTALL.md amendments
- **D-19:** `INSTALL.md` gains two sections after the existing `## Prerequisites` block:
  1. `## Download a prebuilt binary` — links to the GitHub Releases page; `curl -LO` example for darwin/linux/windows; one-line untar/unzip; `chmod +x`; `mv` to a directory on PATH.
  2. `## Verify a release binary` — `cosign verify-blob --certificate-oidc-issuer ... --certificate-identity-regexp '^https://github\.com/postfix/serena/...$' --signature serena.sig --certificate serena.pem serena` plus a `sha256sum -c checksums.txt` example. Mandatory copy-pasteable form.
- **D-20:** No new content added to README.md beyond a single line near the install section pointing at `INSTALL.md` for prebuilt + verify instructions. Avoid duplicating the verify command in two places (drift risk).

### Verification (PR-level)
- **D-21:** Phase 51 lands as a single PR. PR success criteria:
  1. `.goreleaser.yml` exists at repo root and `goreleaser check` passes.
  2. `.github/workflows/release.yml` exists; `goreleaser release --snapshot --clean` invoked locally produces 6 archives + `checksums.txt` + 12 sig/cert files.
  3. `.github/workflows/publish.yml` is deleted; `git grep publish.yml .github/` returns nothing.
  4. `.github/workflows/docker.yml` is unchanged byte-for-byte (`git diff main -- .github/workflows/docker.yml` is empty).
  5. `INSTALL.md` contains the two new sections with the exact `--certificate-identity-regexp` from D-03.
  6. `cmd/serena/main.go` (or equivalent) declares `version`/`commit`/`date` package-level vars wired into the `--version` output.
  7. `RELEASING.md` (or `CONTRIBUTING.md` `## Releasing` section) documents the manual snapshot + diff repro check.
  8. Existing CI (`go-test.yml`, `docker.yml`) stays green.
- **D-22:** Tagged release verification (`v1.9.0-rc0` or similar dry-run on a throwaway tag) is **out of scope for the PR** — it's a maintainer post-merge ritual. The PR only proves the pipeline is wired correctly via `goreleaser release --snapshot --clean` locally.

### Out of Scope (reaffirmed)
- **D-23:** Homebrew tap, Scoop bucket, Linux native packages → Phase 52.
- **D-24:** Docker image publishing → existing `docker.yml`, untouched.
- **D-25:** Backfill signatures for historical Git tags (none exist for the Go product anyway).
- **D-26:** Documentation in `docs/runbooks/`, dashboards, or metrics → Phase 53/54.
- **D-27:** SBOM generation (cyclonedx, syft) — explicitly deferred. Cosign keyless covers provenance via Rekor; SBOM is a separate phase if/when needed.

### Plan structure (suggested, planner has final say)
- **D-28:** Two-plan split, single PR:
  - **Plan 51-01 (release pipeline):** add `.goreleaser.yml`, add `.github/workflows/release.yml`, delete `.github/workflows/publish.yml`, add `main.version`/`main.commit`/`main.date` scaffold to `cmd/serena/main.go` + `internal/cli/root.go`. Verifier: `goreleaser check`, `goreleaser release --snapshot --clean` succeeds locally, `--version` output reflects ldflags.
  - **Plan 51-02 (docs):** amend `INSTALL.md` with download + verify sections, add `RELEASING.md` (or `## Releasing` to `CONTRIBUTING.md`), update `Makefile` `build:` target with version ldflags. Verifier: greps for the cosign verify command snippet, manual snapshot+diff command documented, `make build && ./serena --version` shows real git describe output.

### Claude's Discretion
- Goreleaser version pin (latest stable at planning time is fine; document it).
- Whether `RELEASING.md` is a separate file or a `## Releasing` section in `CONTRIBUTING.md`.
- Exact `--version` output format string.
- Whether to add `make release-snapshot` / `make release-verify` Makefile targets to make the maintainer ritual one-line.
- Precise release notes template / `release.draft` setting (default `false` is fine).
- Whether to include `LICENSE` and `README.md` inside the archive (recommended) or just the binary.
</decisions>

<specifics>
## Specifics & References

- **"Honest signal" precedent** (Phase 50 D-A2): if a check isn't enforced, it should be visibly absent, not half-implemented. Drives D-04 (delete publish.yml outright) and D-08 (no CI repro job).
- **"Single Go binary, no Python/Docker in shipping artifact"** (PROJECT.md core value): drives D-04 and D-06 — Phase 51 doesn't broaden the product surface.
- **Cosign keyless precedent**: Kubernetes, Prometheus, Terraform, and most major Go OSS projects in 2026 use Sigstore keyless. INSTALL.md verify command should look familiar to anyone who has verified these.
- **Goreleaser action version**: pin to a specific minor (e.g., `v6.X.Y`) in `release.yml` — auto-bumping breaks reproducibility.
</specifics>

<deferred>
## Deferred Ideas (out of scope, captured for backlog)

- **SBOM generation** (cyclonedx, syft) — deferred. Cosign keyless + Rekor provenance is sufficient for v1.9. Revisit if a downstream consumer (corp / govt) requires SBOM.
- **GoReleaser Pro features** (multi-repo, monorepo, snapcraft) — not needed.
- **macOS notarization / codesigning with Apple Developer cert** — not needed for a CLI tool distributed via GitHub Releases. Users may see Gatekeeper warnings on first run; document workaround in INSTALL.md if reports come in.
- **Windows code-signing certificate** — same reasoning. Users can run via PowerShell with `Unblock-File`.
- **`linux/arm/v7` (Raspberry Pi 32-bit)** — out of scope; PKG-01 is explicit about amd64/arm64 only.
- **`darwin/universal` fat binary** — out of scope; users pick darwin/amd64 or darwin/arm64.
- **Reproducibility CI gate** — deferred. Manual ritual now (D-07/D-08); revisit if non-determinism creeps in.
- **Docker absorption into goreleaser** — deferred. May revisit during Phase 52 when wiring up package managers.
</deferred>

<questions_for_research>
## Questions for the Research Phase

The phase researcher should investigate before planning:

1. **Cosign keyless gotchas** — Are there known issues with `goreleaser`'s `signs:` block + cosign keyless on `ubuntu-latest` runners as of late 2025 / early 2026? Any flags needed to avoid Rekor rate-limits or transient Fulcio outages?
2. **Goreleaser archive contents** — Confirm goreleaser's default `archives:` config includes `LICENSE` and `README.md` automatically, or whether `files:` must be specified.
3. **`main.version` ldflag injection** — Confirm `{{.Version}}` vs `{{.Tag}}` semantics in goreleaser; pick the one that strips the leading `v` if we want `serena version 1.9.0` rather than `serena version v1.9.0`.
4. **`prerelease: auto` exact behavior** — Confirm that goreleaser's `auto` mode treats `-rc.1`, `-rc1`, `-beta.1`, `-alpha.2` all as pre-release. Verify against the goreleaser changelog around v2.x.
5. **Reproducibility under `-buildvcs`** — `-buildvcs=false` is a Go 1.18+ flag; confirm interaction with goreleaser's own VCS info injection. May need to disable goreleaser's VCS detection too.
6. **Snapshot + diff exclusions** — What additional metadata files does goreleaser's `--snapshot` emit that need exclusion from the repro diff (artifacts.json, metadata.json, etc.)?
7. **Workflow permissions least-privilege** — Confirm the exact minimum permissions for `id-token: write` + cosign keyless against GitHub's current docs. `contents: write` for releases, anything else?
</questions_for_research>
