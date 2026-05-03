---
phase: 51-packaging-goreleaser
reviewed: 2026-04-29T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/treesitter/bindings/r/binding.go
  - internal/treesitter/bindings/r/binding_nocgo.go
  - internal/treesitter/bindings/swift/binding.go
  - internal/treesitter/bindings/swift/binding_nocgo.go
  - internal/treesitter/registry.go
  - .goreleaser.yaml
  - .github/workflows/release.yml
  - Makefile
  - README.md
  - INSTALL.md
  - CONTRIBUTING.md
  - minisign.pub
findings:
  blocker: 3
  warning: 5
  info: 4
  total: 12
status: issues_found
---

# Phase 51 (Wave 3): Code Review Report

**Reviewed:** 2026-04-29
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

Wave 3 closed the prior CR-01..CR-04 / WR-01..WR-06 / IN-01..IN-04 findings. The R/Swift CGO build-tag plumbing (`bindings/{r,swift}/binding.go` + `_nocgo.go` + nil-guarded registration in `registry.go`) is correct: stubs return `nil`, registration is guarded, and the language count narrative (21 vs 23) is consistent across binding.go, binding_nocgo.go, registry.go, and the deferred-items doc. The release workflow correctly fails-closed on the PLACEHOLDER public key (CR-02) and verifies the minisign tarball SHA-256 before extracting (CR-04).

However, three BLOCKERs remain:

1. **The goreleaser pipeline cannot produce a binary at all under `CGO_ENABLED=0`** because every upstream tree-sitter Go binding (20 packages -- go, python, rust, typescript, c, cpp, csharp, java, javascript, kotlin, php, ruby, bash, haskell, julia, ocaml, scala, hcl, lua, zig) is CGO-only and is imported unconditionally in `internal/treesitter/registry.go`. I verified this directly: `CGO_ENABLED=0 go build ./cmd/serena` fails with 20 "build constraints exclude all Go files" errors. This is the structural blocker tracked as DEF-51-02 in `deferred-items.md`; it leaves SC-1 ("6 archives via goreleaser") unable to succeed end-to-end. Wave 3's fix only addressed the two locally-vendored bindings (R and Swift) and explicitly documents this gap, but the file under review (`registry.go`) imports the broken upstreams unconditionally and ships in main. Any tag pushed today will fail in CI.

2. **The INSTALL.md verification recipe produces 404s** because of a goreleaser version-template mismatch: `name_template: "serena_{{ .Version }}_{{ .Os }}_{{ .Arch }}"` produces `serena_1.9.0_linux_amd64.tar.gz` (no `v` prefix; goreleaser strips it from `.Version`), but `INSTALL.md` tells users `VERSION=v1.9.0` and constructs URLs as `serena_${VERSION}_..._.tar.gz` -> `serena_v1.9.0_linux_amd64.tar.gz`. The two will never match. Every copy/paste user hits a 404.

3. **`minisign.pub` is committed as a literal `PLACEHOLDER`.** The release.yml pre-flight grep does block a release tag from publishing, but if a user follows INSTALL.md against `main` today (e.g., to verify a future signed release after rotating their local copy of the public key), they fetch garbage. The fix for this BLOCKER is operational (generate the real key, commit, set secrets), not code -- but it must happen before any v* tag is pushed and is in scope for "phase 51 must close" verification.

The remaining warnings are smaller-but-real correctness issues in the release workflow and documentation (sequence of secret-key disk lifetime, Makefile/CONTRIBUTING claims that overstate what the dry-run validates, identity drift between "Serena" and "Helix" naming).

## Critical Issues (BLOCKER)

### CR-01: `CGO_ENABLED=0 go build ./cmd/serena` fails -- registry.go unconditionally imports 20 CGO-only upstream bindings

**File:** `internal/treesitter/registry.go:11-37`
**Severity:** BLOCKER

**Issue:**
The Wave 3 fix introduced build-tag stubs only for the two locally-vendored bindings (`internal/treesitter/bindings/r` and `internal/treesitter/bindings/swift`). But every other tree-sitter binding imported by `registry.go` -- `tree-sitter-go`, `tree-sitter-python`, `tree-sitter-rust`, `tree-sitter-typescript`, `tree-sitter-c`, `tree-sitter-cpp`, `tree-sitter-c-sharp`, `tree-sitter-java`, `tree-sitter-javascript`, `tree-sitter-kotlin`, `tree-sitter-php`, `tree-sitter-ruby`, `tree-sitter-bash`, `tree-sitter-haskell`, `tree-sitter-julia`, `tree-sitter-ocaml`, `tree-sitter-scala`, `tree-sitter-hcl`, `tree-sitter-lua`, `tree-sitter-zig` -- ships an upstream `binding.go` with `// #cgo CFLAGS: ... import "C"` and **no `//go:build cgo` tag**. Running `CGO_ENABLED=0 go build ./cmd/serena` therefore fails at compile time with 20 errors of the form `build constraints exclude all Go files in .../bindings/go`.

I verified this directly against the working tree: `CGO_ENABLED=0 go build` produced exactly the 20 errors listed.

`.goreleaser.yaml:13` sets `CGO_ENABLED=0`. The release pipeline therefore cannot build a single binary, let alone the 6-archive matrix that SC-1 requires. The `release.yml` workflow will fail at the first goreleaser invocation (the pass-1 reproducibility snapshot) before any signing or publishing happens. This is the same failure DEF-51-02 records in `deferred-items.md`.

The Wave-3 R/Swift work is internally consistent and was a prerequisite, but it does not by itself unblock SC-1. Until DEF-51-02 lands a workable strategy (vendor every upstream binding with build tags, or split the registry under a `cgo`-tagged file with a non-CGO fallback that registers no tree-sitter languages), phase 51 cannot ship.

**Fix:**
Pick one of the two strategies sketched in `deferred-items.md` DEF-51-02 and execute it. Either:

(a) Split `registry.go` into two files (`registry_cgo.go` with `//go:build cgo` and `registry_nocgo.go` with `//go:build !cgo`) where the no-CGO file builds an empty registry, and gate every consumer that calls into the registry to tolerate an empty languages map. This preserves a CGO_ENABLED=0 binary at the cost of zero tree-sitter language coverage.

(b) Vendor every upstream binding the way R and Swift were vendored (drop a `bindings/{lang}/binding.go` with `//go:build cgo` plus a `_nocgo.go` stub returning `nil`), update `registry.go` to import the local copies, and accept the maintenance burden of tracking upstream parser/scanner.c updates.

Either way: do NOT ship the current `registry.go` as-is and expect the goreleaser pipeline to succeed.

---

### CR-02: INSTALL.md verification recipe produces 404s -- archive name template uses `{{ .Version }}` (strips `v`) but recipe uses `VERSION=v1.9.0`

**File:** `INSTALL.md:11-32` (cross-references `.goreleaser.yaml:35`)
**Severity:** BLOCKER

**Issue:**
`INSTALL.md` instructs the user to set `VERSION=v1.9.0` (line 12) and then constructs every download URL as:

```
serena_${VERSION}_${OS}_${ARCH}.tar.gz
```

which expands to `serena_v1.9.0_linux_amd64.tar.gz`. But `.goreleaser.yaml:35` uses `name_template: "serena_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`. In goreleaser, `{{ .Version }}` is the version string with the leading `v` **stripped** (this is a long-standing goreleaser convention; the prefixed form is `{{ .Tag }}`). The actual published archive will therefore be `serena_1.9.0_linux_amd64.tar.gz` -- without the `v`.

Result: every user who copy-pastes the verification recipe gets `HTTP 404` on the first `curl -LO`, then on every subsequent `curl -LO`, then never reaches the `minisign -V` line. The recipe block is the centerpiece of INSTALL.md and is documented as "a single block you can copy and paste end-to-end."

This is a hard correctness regression introduced (or at least preserved) in this wave because the same recipe is repeated in the release-process docs (`CONTRIBUTING.md`) and was not flagged when the goreleaser config was reviewed.

**Fix:**
Pick one of the two and apply consistently:

(a) **Easier:** change the goreleaser archive name template to use the prefixed tag:

```yaml
# .goreleaser.yaml
archives:
  - id: serena
    ids: [serena]
    formats: ["tar.gz"]
    name_template: "serena_v{{ .Version }}_{{ .Os }}_{{ .Arch }}"  # add literal 'v'
```

(b) **Equally valid:** change INSTALL.md to set `VERSION=1.9.0` (no `v`) and document URL paths as `download/v${VERSION}/serena_${VERSION}_...`:

```bash
VERSION=1.9.0  # without 'v' prefix; the GitHub release tag is v$VERSION
OS=linux
ARCH=amd64
curl -LO https://github.com/agenthands/helix/releases/download/v$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz
```

Either fix needs to land in BOTH `INSTALL.md` and `CONTRIBUTING.md` and be verified against an actual `goreleaser release --snapshot --clean --skip=sign` archive name in `dist/`.

---

### CR-03: `minisign.pub` is committed as the literal PLACEHOLDER

**File:** `minisign.pub:1-3`
**Severity:** BLOCKER

**Issue:**
The file ships in `main` with `untrusted comment: serena minisign public key -- PLACEHOLDER (release.yml pre-flight greps this marker; ...)` and a body of all-zero base64. Two consequences:

1. The release pre-flight in `release.yml:22-33` correctly refuses to publish a tag with this file in place (this part is good).
2. INSTALL.md tells users to `curl https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub` "one-time" and use that key to verify every future archive. Anyone who follows the recipe today downloads the placeholder. After the maintainer rotates the key and a real release is published, the user's locally-cached `minisign.pub` (downloaded at "one-time" step) verifies nothing -- and `minisign -V` will fail with a confusing parse error rather than a clear "wrong key" message, because the placeholder isn't even a syntactically valid key.

The release.yml gate prevents catastrophic mis-signing in CI, but it does not prevent users from caching a placeholder as their root of trust.

**Fix:**
Generate the real keypair on a trusted local machine, commit `minisign.pub`, and upload the secret half + password as repo secrets (procedure already documented in `CONTRIBUTING.md`, "One-time keypair setup"). This must happen BEFORE the first `v*` tag is pushed AND before INSTALL.md is published as a stable installation guide. There is nothing the code review can fix here -- it is an operational blocker in the same commit set.

If the team intentionally wants to ship INSTALL.md and the goreleaser config without a real key (i.e., phase-51 closes "build pipeline ready, tag-cutting deferred"), at minimum INSTALL.md should carry a top-of-document warning that no signed releases exist yet, and the recipe should be marked "not yet usable." The current INSTALL.md reads as if signed releases are already available.

---

## Warnings

### WR-01: `/tmp/minisign.key` is on disk during both reproducibility-gate snapshot passes, even though those passes use `--skip=sign`

**File:** `.github/workflows/release.yml:79-89, 91-122`
**Severity:** WARNING

**Issue:**
The "Write minisign secret key to disk (umask 077)" step runs before the snapshot pass-1, snapshot pass-2, the diff gate, and finally the real release. The two snapshot passes use `--skip=sign` and therefore do not need the key on disk; they nonetheless run with the key present at `/tmp/minisign.key` for several minutes of build time. The blast radius is small (ephemeral runner, umask 077), but a third-party GitHub Action invoked elsewhere in the build matrix (e.g., a future `goreleaser-action` upgrade, or a `setup-go` post-step) that scans `/tmp` could exfiltrate the key during work that does not actually require it.

The runner is ephemeral so this is not a credential-persistence issue, but defense-in-depth has been the explicit goal of the wave-3 hardening (CR-04, the post-job `shred`).

**Fix:**
Move the "Write minisign secret key to disk" step to immediately before "Real release (sign + publish)", after the reproducibility gate has passed. The two snapshot passes do not read the key. Sketch:

```yaml
- name: Reproducibility gate -- snapshot pass 1
  uses: goreleaser/goreleaser-action@...
  ...

- name: Reproducibility gate -- snapshot pass 2
  uses: goreleaser/goreleaser-action@...
  ...

- name: Diff sha256s -- fail if non-reproducible
  run: |
    ...

- name: Write minisign secret key to disk (umask 077)   # <-- moved here
  env:
    MINISIGN_PRIVATE_KEY: ${{ secrets.MINISIGN_PRIVATE_KEY }}
  run: |
    set -euo pipefail
    umask 077
    printf '%s' "$MINISIGN_PRIVATE_KEY" > /tmp/minisign.key

- name: Real release (sign + publish)
  ...
```

The post-job `shred` step continues to handle cleanup either way.

---

### WR-02: README.md still mixes "Serena" and "Helix" branding inconsistently

**File:** `README.md:7-12, 64, 69, 71, 88, 137, 156-161, 318-330, 346`
**Severity:** WARNING

**Issue:**
README.md announces the rename in lines 12 and 346 ("Helix started as a rewrite of Serena MCP. Full rename in progress; existing references to 'Serena' in this README will move to 'Helix' in an upcoming refactor."), but the rest of the document continues to use "Serena" throughout, including in user-facing CLI examples (`serena setup claude-code`), config snippets (`"serena": { ... }`), and architecture descriptions. INSTALL.md (line 1: "Serena is a single Go binary") and CONTRIBUTING.md (line 1: "Contributing to Serena") are similarly Serena-only.

Users reading INSTALL.md against a repo named `helix` (and a release on `agenthands/helix`) get a slightly disorienting experience: the binary is `serena`, the verification recipe pulls archives named `serena_*`, but the GitHub URL says `helix`. This is not a correctness bug (the URLs all resolve), but it is a quality issue for first-time users and makes the README's own "rename in progress" disclaimer feel stale.

**Fix:**
Either complete the rename or pin the inconsistency to a tracked plan (e.g., link the disclaimer in README.md:12 to a specific phase in the roadmap). Current state where the disclaimer is open-ended ("upcoming refactor", no link) does not give the user a way to know whether `serena_v1.9.0_linux_amd64.tar.gz` or `helix_v1.9.0_linux_amd64.tar.gz` will be the artifact name they download next month.

---

### WR-03: CONTRIBUTING.md `Releasing` section claims the dry-run "validates the build matrix" -- but the dry-run currently fails

**File:** `CONTRIBUTING.md:163-169, 213`
**Severity:** WARNING

**Issue:**
CONTRIBUTING.md tells contributors:

> To dry-run the build matrix locally (signs are skipped because the secret key lives only in CI):
>
> ```sh
> make release-snapshot
> ```
>
> Output goes to `dist/` (gitignored, overwrites). On a clean checkout you should see 6 archives (`serena_<version>_<os>_<arch>.tar.gz`) and a `checksums.txt` file.

Per CR-01, this command currently fails on a clean CGO_ENABLED=0 checkout (which is what `.goreleaser.yaml:13` enforces). A new contributor who runs `make release-snapshot` to "validate the build matrix" gets 20 build-constraint errors and no `dist/`. The doc is currently misleading.

Line 213 makes the same implicit claim: "The dry-run still validates the build matrix, archive packaging, and checksums.txt generation." It does not, until DEF-51-02 closes.

**Fix:**
Until DEF-51-02 is resolved, either:
- Add a clear note: "Until phase 51 DEF-51-02 closes, `make release-snapshot` is expected to fail with `build constraints exclude all Go files in .../tree-sitter-*/bindings/go`. The pipeline will be functional once we vendor the upstream bindings with build tags."
- OR temporarily set `CGO_ENABLED=1` in `.goreleaser.yaml` and accept that releases ship CGO binaries (with the static-binary invariant relaxed). This is a phase-level decision, not a code-review nit.

---

### WR-04: `release.yml` minisign install only verifies an x86_64 Linux tarball -- relies on `ubuntu-latest` runner architecture

**File:** `.github/workflows/release.yml:60-65, 13`
**Severity:** WARNING

**Issue:**
The `ARCH != x86_64` check correctly fails the workflow loud (`exit 1`) if GitHub flips `ubuntu-latest` to a non-x86_64 runner. This is the right default. The warning is that **this never gets exercised in CI** until the day it actually flips, at which point every pending tag fails simultaneously. A more robust posture would be to additionally pin the runner to `ubuntu-22.04` (or whatever LTS the release matrix currently expects) so the architecture is fixed at the workflow level rather than detected at runtime.

This is a quality / operational-resilience issue, not a correctness bug.

**Fix:**
```yaml
jobs:
  release:
    runs-on: ubuntu-22.04   # was: ubuntu-latest -- pin so 'ubuntu-latest' rolling forward never silently breaks the minisign install
```

The runtime `uname -m` check stays as a defense-in-depth backstop for future runner image changes within the pinned series.

---

### WR-05: `INSTALL.md` checksum verification matches more files than the user downloaded -- typos can pass

**File:** `INSTALL.md:27-28, 38`
**Severity:** WARNING

**Issue:**
The recipe runs:
```bash
sha256sum -c checksums.txt 2>&1 | grep "serena_${VERSION}_${OS}_${ARCH}.tar.gz: OK" \
  || { echo "checksum FAILED"; exit 1; }
```

`checksums.txt` lists 6 archives (one per OS/arch). The user has downloaded only one of them. `sha256sum -c` will print 5 `FAILED open or read` lines plus 1 `OK` line, and return non-zero. The pipe-to-grep then matches the OK line and the `||` branch never fires -- so the recipe works for the user, but the side output (`FAILED open or read` lines mixed with the OK line) looks alarming and contradicts the inline comment "fails loud on typos." If a user mis-types `OS` or `ARCH` such that the typo'd name is not in checksums.txt, the recipe still succeeds because grep matches some other archive's OK line that happened to be downloaded by accident -- or the mismatch only surfaces at the later `minisign -V` step, by which point the message is murkier.

**Fix:**
Tighten the check to inspect only the file the user actually downloaded:

```bash
# Verify the downloaded archive's sha256 is present in the (now-trusted) checksums.txt
EXPECTED_HASH=$(grep "serena_${VERSION}_${OS}_${ARCH}.tar.gz" checksums.txt | awk '{print $1}')
ACTUAL_HASH=$(sha256sum "serena_${VERSION}_${OS}_${ARCH}.tar.gz" | awk '{print $1}')
if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
  echo "checksum FAILED: expected=$EXPECTED_HASH actual=$ACTUAL_HASH"; exit 1
fi
echo "checksum OK: $ACTUAL_HASH"
```

This also avoids the macOS/Linux split for `sha256sum -c` vs `shasum -a 256 -c` since we always operate on a single file.

---

## Info

### IN-01: `internal/treesitter/registry.go:50` doc comment is stale

**File:** `internal/treesitter/registry.go:50`
**Severity:** INFO

**Issue:**
```go
// NewGrammarRegistry creates a GrammarRegistry with Go, Python, TypeScript, TSX, and Rust grammars.
```

The function actually registers 23 languages (or 21 under CGO_ENABLED=0). The comment hasn't been updated since waves 1, 2a, 2b, and 2b-gap-closure expanded the registry.

**Fix:**
```go
// NewGrammarRegistry creates a GrammarRegistry with all built-in tree-sitter
// grammars (23 under CGO_ENABLED=1; 21 under CGO_ENABLED=0 -- R and Swift omit
// when their CGO bindings are stubbed out). See the imports above for the full
// list and bindings/{r,swift}/binding_nocgo.go for the CGO=0 fallback.
func NewGrammarRegistry() *GrammarRegistry {
```

---

### IN-02: README.md tool table claims "41+ MCP tools" but the table lists 40 rows

**File:** `README.md:10, 40, 273-316`
**Severity:** INFO

**Issue:**
Lines 10 and 40 advertise "41+ MCP tools." The auto-generated tool table (`<!-- BEGIN TOOLS --> ... <!-- END TOOLS -->`) contains 40 rows by my count. Either the count is off-by-one, or one tool is gated out of the table (some categories?) and not surfaced. Since `make docs` regenerates this table, the 41+ string is hand-written and may have drifted.

**Fix:**
Either:
- Update the prose to "40+ MCP tools" until a 41st ships, or
- Audit `cmd/docgen` to confirm whether any registered tool is being filtered out of the README table.

The marketing copy ("41+") is forgiving but the discrepancy is the kind of thing an alert reader notices and files an issue against.

---

### IN-03: `Makefile:50-53` `release-snapshot` target's missing-binary error message is macOS-only

**File:** `Makefile:50-53`
**Severity:** INFO

**Issue:**
```make
release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
	@command -v goreleaser >/dev/null 2>&1 || { \
	  echo "goreleaser not installed; see CONTRIBUTING.md (Releasing). brew install goreleaser"; exit 1; }
	goreleaser release --snapshot --clean --skip=sign
```

The "brew install goreleaser" hint is macOS-only. Linux users who hit this branch don't get an actionable hint (CONTRIBUTING.md does mention the Linux tarball path, but the make output points only to `brew`).

**Fix:**
```make
@command -v goreleaser >/dev/null 2>&1 || { \
  echo "goreleaser not installed; see CONTRIBUTING.md (Releasing)."; \
  echo "  macOS:  brew install goreleaser"; \
  echo "  Linux:  download tarball from https://github.com/goreleaser/goreleaser/releases"; \
  exit 1; }
```

---

### IN-04: `release.yml` reproducibility gate runs goreleaser THREE times per release -- two snapshot passes plus the real run

**File:** `.github/workflows/release.yml:91-136`
**Severity:** INFO

**Issue:**
On every `v*` tag push the workflow does:
1. `goreleaser release --snapshot --clean --skip=sign` (pass 1)
2. `goreleaser release --snapshot --clean --skip=sign` (pass 2)
3. `goreleaser release --clean` (real)

Three full multi-arch builds. With the 30-minute `timeout-minutes: 30` already on the job, this leaves ~10 minutes per build on a single ubuntu-latest runner. For the current binary size that's fine, but the budget will tighten as the project grows (more dependencies, more LSP types). The reproducibility-gate design is fundamentally correct (CI must verify reproducibility before publishing); this is a budget-watch note, not a defect.

**Fix:**
None required for this phase. Consider adding a tracking note in `deferred-items.md` if the team wants to revisit the gate strategy (e.g., separate workflow on schedule, or compare pass-2 against the real-release archives instead of running pass-2 separately).

---

_Reviewed: 2026-04-29_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
