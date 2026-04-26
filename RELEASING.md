# Releasing Serena

Maintainer-facing guide for cutting a release. End users should read [INSTALL.md](INSTALL.md) instead.

Phase 51 ships a [goreleaser](https://goreleaser.com)-driven pipeline that builds 6 binaries (darwin/linux/windows × amd64/arm64), signs them with [cosign keyless](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/) via Sigstore, and publishes them to a GitHub Release on every `v*` tag push. See `.goreleaser.yml` and `.github/workflows/release.yml` for the configuration.

## Prerequisites

Install the maintainer toolchain locally:

```bash
# macOS
brew install goreleaser cosign

# Or via go install
go install github.com/goreleaser/goreleaser/v2@latest
go install github.com/sigstore/cosign/v2/cmd/cosign@latest
```

Verify versions:

```bash
goreleaser --version   # expect v2.x
cosign version         # expect v2.x
```

## Pre-tag reproducibility ritual (D-07)

Before pushing a release tag, confirm the pipeline produces byte-identical archives across two snapshot runs on the same commit. This catches non-deterministic build inputs early.

There is **no CI gate for reproducibility** (D-08). The check is a manual ritual; if you skip it, the missing signal is visible — `goreleaser` does not silently fail.

Run the snapshot twice and diff the outputs:

```bash
# First snapshot
goreleaser release --snapshot --clean
mv dist /tmp/dist1

# Second snapshot (same commit)
goreleaser release --snapshot --clean
mv dist /tmp/dist2

# Diff with metadata exclusions (signatures and run-time metadata are
# inherently non-reproducible; everything else MUST be byte-identical).
diff -r \
  --exclude='*.sig' \
  --exclude='*.pem' \
  --exclude='checksums.txt*' \
  --exclude='artifacts.json' \
  --exclude='metadata.json' \
  --exclude='config.yaml' \
  /tmp/dist1 /tmp/dist2
```

Expected: empty output. Any diff outside the exclusion list is a reproducibility regression — investigate before tagging.

The Makefile wraps both halves of this ritual:

```bash
make release-snapshot   # runs goreleaser release --snapshot --clean
make release-verify     # runs the diff command above
```

### Why these exclusions

- `*.sig`, `*.pem` — every cosign signature is unique even for identical content (Sigstore short-lived certs).
- `checksums.txt*` — the bare `checksums.txt` is reproducible, but its `.sig`/`.pem` companions are not; excluding the glob is defensive.
- `artifacts.json`, `metadata.json` — goreleaser embeds run timestamps and run IDs.
- `config.yaml` — goreleaser may embed a snapshot timestamp; safest to exclude.

See `.planning/phases/51-packaging-goreleaser/51-RESEARCH.md` Q6 for the full file-by-file justification.

## Tag and push

Once the reproducibility ritual is green, tag and push:

```bash
# Stable release
git tag -a v1.9.0 -m "v1.9.0"
git push origin v1.9.0

# Pre-release (any of -rc.N, -rcN, -beta.N, -alpha.N — goreleaser auto-detects)
git tag -a v1.9.0-rc.1 -m "v1.9.0-rc.1"
git push origin v1.9.0-rc.1
```

Tag pattern is locked to `v*` (D-09). Pre-release detection is automatic: any tag with a semver pre-release suffix (`-rc`, `-beta`, `-alpha`, etc.) publishes as a GitHub pre-release; clean semver tags publish as full releases (D-10).

The `release.yml` workflow on the `postfix/serena` repo runs automatically on tag push and uploads the 6 archives, `checksums.txt`, and per-artifact `.sig` + `.pem` files to the GitHub Release.

## Post-merge verification (D-22)

The PR landing Phase 51 only proves the pipeline is wired correctly via `goreleaser check` and `goreleaser release --snapshot --clean` locally. Tag-driven verification is a post-merge ritual:

1. Push a throwaway tag to your fork or to a private branch — e.g. `v0.0.0-test1`.
2. Watch the `release` workflow run on GitHub Actions.
3. Download one artifact (any os/arch).
4. Run the [INSTALL.md verify command](INSTALL.md#verify-a-release-binary) end-to-end against the downloaded artifact.
5. Delete the test tag and the auto-created release.

If verification succeeds, the pipeline is production-ready. If it fails, the most common causes are:

- `--certificate-identity-regexp` mismatch (the regex pins to `postfix/serena`'s `release.yml` on `refs/tags/v.*` — running from a fork breaks this; that's by design).
- Sigstore Fulcio/Rekor transient outage — re-run the workflow.
- A workflow rename or path change — update INSTALL.md regex if the workflow path changes.

## Out of scope

The following are intentionally **not** part of this release pipeline (deferred or owned elsewhere):

- Homebrew, Scoop, Linux native packages — Phase 52 (D-23).
- Docker images — `.github/workflows/docker.yml` is the existing publisher; not consolidated into goreleaser (D-06).
- SBOM (cyclonedx, syft) — deferred to a future phase (D-27).
- macOS notarization, Windows code-signing certs — out of scope for a CLI distributed via GitHub Releases.
- In-binary auto-update — package managers (brew/scoop/apt) own updates; in-binary updaters add security surface.
