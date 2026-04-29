---
phase: 51-packaging-goreleaser
reviewed: 2026-04-29T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - .goreleaser.yaml
  - .github/workflows/release.yml
  - minisign.pub
  - INSTALL.md
  - README.md
  - Makefile
  - CONTRIBUTING.md
findings:
  critical: 4
  warning: 6
  info: 5
  total: 15
status: issues_found
---

# Phase 51: Code Review Report

**Reviewed:** 2026-04-29
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 51 wires up a goreleaser-driven release pipeline (config, GH Actions workflow, minisign verification ceremony, docs, Makefile target). The high-level shape is sound — tag-triggered, three-pass reproducibility gate, archive signing, auto-generated changelog — and the workflow handles the minisign secret reasonably (`umask 077`, `printf '%s'` to avoid log leaks).

However, the phase prompt explicitly listed "README/INSTALL repo identity correction (`postfix/serena` → `agenthands/helix`)" as a deliverable, and README.md still publishes the wrong module path. There are also four supply-chain / correctness gaps that can ship a broken or compromised release: a placeholder public key that nothing prevents from being used, an unverified third-party tarball download that signs every release, a reproducibility gate that compares snapshot-to-snapshot but never validates the published artifacts, and unpinned third-party actions handling the signing key. Each is concrete and fixable; collectively they undermine the supply-chain story the phase is trying to establish.

## Critical Issues

### CR-01: README.md still installs `github.com/postfix/serena`, contradicting the phase deliverable

**File:** `README.md:64-67`
**Issue:** The phase 51 brief explicitly calls for `postfix/serena` → `agenthands/helix` correction. `INSTALL.md` was updated, but `README.md` still publishes:

```bash
go install github.com/postfix/serena/cmd/serena@latest
# Note: module path is github.com/postfix/serena pending a separate rename decision; the public repo lives at agenthands/helix.
```

Users following the README's primary install path will pull from a foreign module path that may or may not exist, may not be controlled by the project, and definitely is not the `agenthands/helix` repo this release pipeline is publishing to. The footnote does not save the copy-paste user. This is the exact identity-correction defect the phase set out to fix.

**Fix:** Either remove the `go install` block (binaries are now the recommended install per INSTALL.md) or update it to a path the project actually owns. If the module rename is genuinely deferred, replace the block with a pointer to `make build` / pre-built binaries:

```markdown
### Install

Pre-built binaries for darwin/linux/windows on amd64/arm64 — see [INSTALL.md](INSTALL.md).

Or build from source:

```bash
git clone https://github.com/agenthands/helix.git
cd helix
go build ./cmd/serena
```
```

### CR-02: `minisign.pub` is an all-zeros placeholder with no CI guard against tagging a release

**File:** `minisign.pub:1-2`
**Issue:** The committed public key is literally `RWQAAAAA...` (32 zero bytes after the 2-byte algorithm/key-id prefix). `CONTRIBUTING.md:182-187` documents that `MINISIGN_PRIVATE_KEY` and `MINISIGN_PASSWORD` must be set on the repo before tagging, but nothing enforces that the public key in the repo matches the private key in CI. Failure mode if a `v*` tag is pushed today:

1. CI signs archives with whatever key is in `MINISIGN_PRIVATE_KEY` (real or unset).
2. Users follow `INSTALL.md:20` to fetch `minisign.pub` from `main` — they get the placeholder.
3. `minisign -V` fails for every user, every download, with a confusing "Signature verification failed" — and the release is unrecoverable without re-tagging.

Worse, if the placeholder is replaced post-tag, anyone who already cached the old `minisign.pub` from `INSTALL.md`'s one-time-fetch instruction is permanently broken.

**Fix:** Add a workflow pre-flight check that refuses to release with the placeholder, e.g. as a step before the reproducibility gate:

```yaml
- name: Refuse placeholder minisign public key
  run: |
    set -euo pipefail
    if grep -q 'PLACEHOLDER' minisign.pub; then
      echo "::error::minisign.pub is still the placeholder; replace it before tagging a release."
      exit 1
    fi
    # Optional: assert the pubkey matches the secret keypair by re-deriving from the secret.
```

Until a real key is generated, the workflow should fail closed.

### CR-03: Reproducibility gate compares snapshot-to-snapshot, never against the published artifacts

**File:** `.github/workflows/release.yml:53-84`
**Issue:** Pass 1 and Pass 2 both run `release --snapshot --clean --skip=sign`. The "real release" Pass 3 (line 86-91) runs `release --clean` (no `--snapshot`). Snapshot and real release embed different `-X main.version=...` ldflag values (snapshot uses a synthesized version like `0.0.0-next-...`, real uses the tag). The `name_template` also expands `{{ .Version }}` differently between modes.

Concretely: the gate proves "two snapshot builds are byte-identical" but the artifacts that ship to users come from a third build that was never compared to anything. A non-determinism source that is gated behind real-release-only inputs (e.g., the version string interacting with a build cache, a tag-triggered code path, a future `release.extra_files`) will silently bypass the gate.

**Fix:** Either (a) gate on Pass 3 too — checksum the real-release `dist/` and compare against a fourth build of the same tag — or (b) explicitly document that the gate validates *build-environment* determinism (toolchain, mod_timestamp, trimpath) and not artifact-content equality. Option (a):

```yaml
- name: Capture real release sha256s
  run: |
    set -euo pipefail
    (cd dist && find . -name '*.tar.gz' -print0 | sort -z | xargs -0 sha256sum) > /tmp/real.sha256

- name: Real release pass 2 (verify byte-identical)
  uses: goreleaser/goreleaser-action@v7
  with: { distribution: goreleaser, version: '~> v2', args: 'release --clean --skip=publish --skip=sign' }
  # ... then diff /tmp/real.sha256 against the second pass
```

If gating Pass 3 is too expensive, at minimum tighten the language in `CONTRIBUTING.md:161` — "refuses to publish if two consecutive snapshot builds produce non-byte-identical archives" overstates what the gate actually proves.

### CR-04: Minisign tarball is fetched without integrity verification — single point of supply-chain failure

**File:** `.github/workflows/release.yml:34-39`
**Issue:** The minisign binary is downloaded from GitHub releases over TLS but with no checksum or signature check:

```bash
curl -fsSL -o /tmp/minisign.tar.gz \
  "https://github.com/jedisct1/minisign/releases/download/${MINISIGN_VERSION}/minisign-${MINISIGN_VERSION}-linux.tar.gz"
tar -xzf /tmp/minisign.tar.gz -C /tmp
sudo install -m 0755 /tmp/minisign-linux/x86_64/minisign /usr/local/bin/minisign
```

This `minisign` binary signs every release archive with the project's private key. If GitHub release infrastructure for `jedisct1/minisign` is compromised, or DNS/TLS is hijacked on the runner, an attacker-controlled `minisign` binary handles your secret key. The whole signing ceremony exists to defend against this exact class of compromise; doing it with an unverified third-party binary defeats the threat model.

**Fix:** Pin a SHA-256 of the tarball and verify before extracting:

```bash
MINISIGN_VERSION="0.12"
MINISIGN_SHA256="<paste from upstream release notes / verified locally>"
curl -fsSL -o /tmp/minisign.tar.gz \
  "https://github.com/jedisct1/minisign/releases/download/${MINISIGN_VERSION}/minisign-${MINISIGN_VERSION}-linux.tar.gz"
echo "${MINISIGN_SHA256}  /tmp/minisign.tar.gz" | sha256sum -c -
tar -xzf /tmp/minisign.tar.gz -C /tmp
```

Bonus: also verify the upstream minisign signature on the tarball (jedisct1 self-signs releases with a documented public key).

## Warnings

### WR-01: Third-party actions are pinned to mutable major-version tags, not SHAs

**File:** `.github/workflows/release.yml:18, 23, 54, 69, 87`
**Issue:** `actions/checkout@v4`, `actions/setup-go@v5`, `goreleaser/goreleaser-action@v7` are major-tag pins. Major tags are mutable references — the upstream repo can repoint `v4` at a different commit at any time, and a compromised maintainer account can do so silently. For a release pipeline that signs binaries with a private key (`MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD`), GitHub's own hardening guide recommends commit SHA pinning.

**Fix:** Pin to commit SHAs and add a comment with the version for human readability:

```yaml
- uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11  # v4.1.1
- uses: actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491  # v5.0.0
- uses: goreleaser/goreleaser-action@7ec5c2b0c6cdda6e8bbb49444bc797dd33d74dd8  # v7.0.0
```

Use `dependabot` or `renovate` to keep them current.

### WR-02: `checksums.txt` is not signed; INSTALL.md's checksum step adds no integrity guarantee

**File:** `.goreleaser.yaml:46-49`, `INSTALL.md:22-23`
**Issue:** `signs.artifacts: archive` signs only the `.tar.gz` files. `checksums.txt` is not signed. `INSTALL.md` instructs:

```bash
sha256sum -c --ignore-missing checksums.txt   # step 1
minisign -V -p minisign.pub -m serena_..._.tar.gz  # step 2
```

If an attacker swaps both the archive and `checksums.txt`, step 1 passes. Only step 2 catches it. So the checksum step provides zero independent integrity — it is documentation theater unless the user runs minisign too. Worse, `--ignore-missing` silently exits 0 if the user accidentally downloaded only the signature file.

**Fix:** Either drop the checksum step from INSTALL.md (minisign alone is sufficient), or set `signs.artifacts: all` so `checksums.txt` is also signed, then have the user `minisign -V` on `checksums.txt` first and validate the archive against it. Pick one — current state is misleading without strengthening anything.

### WR-03: README.md HTTP-mode flag conflicts with INSTALL.md

**File:** `README.md:134`, `INSTALL.md:248-249`
**Issue:** Two different commands documented for the same feature:

- `README.md:134`: `serena --serve --http-addr=:9091`
- `INSTALL.md:248`: `serena --mode=http --http-addr=127.0.0.1:8080`

At least one is wrong relative to the actual binary surface. Users will copy-paste one or the other and hit `unknown flag` or "served on the wrong port from what the next line says".

**Fix:** Pick the canonical syntax (cross-reference `internal/cli/root.go` for the actual flag), update both docs to match, and pick one default port.

### WR-04: README.md attribution is internally inconsistent

**File:** `README.md:12, 348`
**Issue:** Line 12 says "Helix started as a rewrite of [Serena MCP](https://github.com/oraios/serena)". Line 348 footnote says "Originally inspired by [Python Serena](https://github.com/lks-ai/serena)". Two different upstream URLs (`oraios/serena` vs `lks-ai/serena`) are credited as the origin in the same README. One of them is wrong.

**Fix:** Verify the actual upstream and reconcile. If `oraios/serena` is correct, fix the footnote. If neither is correct, fix both.

### WR-05: `--ignore-missing` on `sha256sum` hides "wrong file downloaded" failures

**File:** `INSTALL.md:23`
**Issue:** `sha256sum -c --ignore-missing checksums.txt` exits 0 even if the user downloaded zero matching files. A user who fat-fingers the archive name (typo in `OS` or `ARCH`) sees "OK" and proceeds to a confusing minisign error, never realizing the checksum step did nothing.

**Fix:** Drop `--ignore-missing` and tell users to grep the line for their archive first, or replace the step with:

```bash
sha256sum -c checksums.txt 2>&1 | grep "serena_${VERSION}_${OS}_${ARCH}.tar.gz: OK" \
  || { echo "checksum FAILED"; exit 1; }
```

### WR-06: `MINISIGN_PRIVATE_KEY` lives on disk in `/tmp/minisign.key` for the rest of the job with no cleanup

**File:** `.github/workflows/release.yml:41-51`
**Issue:** `umask 077` is good defense for the file permissions, but the secret persists at `/tmp/minisign.key` for the duration of the runner job. Any subsequent step (e.g., a future-added "post-release announce" step that pulls a third-party action) can read it. The runner itself is ephemeral, but the in-job blast radius is unnecessarily large.

**Fix:** Add a `post`-style cleanup step that runs `always()`:

```yaml
- name: Wipe minisign secret key
  if: always()
  run: shred -u /tmp/minisign.key 2>/dev/null || rm -f /tmp/minisign.key
```

Place it as the last step in the job so it runs whether sign succeeds or fails.

## Info

### IN-01: Makefile `release-snapshot` produces a confusing error when goreleaser is missing

**File:** `Makefile:50-51`
**Issue:** `release-snapshot` invokes `goreleaser` directly. If a contributor doesn't have it installed, they see `make: goreleaser: No such file or directory` — `CONTRIBUTING.md:169` mentions `brew install goreleaser` but contributors typically discover this via the make target failing.

**Fix:** Guard the target with a `command -v` check:

```makefile
release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
	@command -v goreleaser >/dev/null 2>&1 || { \
	  echo "goreleaser not installed; see CONTRIBUTING.md (Releasing). brew install goreleaser"; exit 1; }
	goreleaser release --snapshot --clean --skip=sign
```

### IN-02: Minisign binary URL is hardcoded to `linux-x86_64` with no runner-arch guard

**File:** `.github/workflows/release.yml:36-38`
**Issue:** `runs-on: ubuntu-latest` is x86_64 today, so the URL works. If ubuntu-latest ever flips to arm64 (or someone changes the runner) the curl 404s with a cryptic tar error. Not a current bug — flagging because release infrastructure outlives one's intentions.

**Fix:** Add an explicit assertion or compute the URL from `uname -m`:

```bash
ARCH="$(uname -m)"
[ "$ARCH" = "x86_64" ] || { echo "unsupported runner arch: $ARCH"; exit 1; }
```

### IN-03: `dist-pass-1` directory is left around after the gate

**File:** `.github/workflows/release.yml:63`
**Issue:** `mv dist dist-pass-1` is never cleaned up. Pass 3's `--clean` only wipes `dist/`. Runners are ephemeral, but the leftover directory inflates cache snapshots if anyone later adds a workspace cache.

**Fix:** Add `rm -rf dist-pass-1` after the diff step succeeds, or `if: always()` to clean both ways.

### IN-04: CONTRIBUTING.md overstates the reproducibility guarantee

**File:** `CONTRIBUTING.md:161`
**Issue:** "The CI-enforced reproducibility gate refuses to publish if two consecutive snapshot builds produce non-byte-identical archives, so a non-deterministic build cannot reach users." As detailed in CR-03, the gate only proves snapshot determinism. A non-determinism that lives behind the real-release code path can reach users despite the gate.

**Fix:** Either fix CR-03 (extend the gate to real-release artifacts) or soften the claim — e.g. "two consecutive snapshot builds" → "two consecutive builds with identical inputs". Pair the doc with whatever the gate actually does.

### IN-05: `untrusted comment:` in `minisign.pub` literally says "PLACEHOLDER, replace before first release"

**File:** `minisign.pub:1`
**Issue:** This is intentional and fine for a placeholder, but combined with CR-02 (no CI guard) it's a footgun the placeholder will outlive its welcome. The comment serves humans, not the workflow.

**Fix:** See CR-02 for the actual gate. Once a real key replaces the placeholder, drop the "PLACEHOLDER" string from the untrusted comment so a future regression doesn't go unnoticed.

---

_Reviewed: 2026-04-29_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
