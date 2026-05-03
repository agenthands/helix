---
phase: 51-packaging-goreleaser
verified: 2026-04-29T00:00:00Z
re_verified: 2026-05-03T00:00:00Z
status: human_needed
score: 3/4 success criteria verified (1 awaiting first real release tag)
overrides_applied: 0
re_verification_summary: |
  Original 2026-04-29 verdict was gaps_found (1/4) driven by DEF-51-01 (CGO build
  constraints failing CGO_ENABLED=0 cross-compile) plus four substantive concerns
  about minisign.pub placeholder, INSTALL.md `--ignore-missing`, README postfix/serena,
  and reproducibility-gate scope. Phase 51.1 (CGO treesitter gate) and Phase 52
  (rename + embed mechanism + CI pre-flight) addressed three of those four concerns.
  Today (2026-05-03) a local `goreleaser release --snapshot --clean --skip=sign`
  produced 6 archives (darwin/linux/windows × amd64/arm64) plus checksums.txt in 3s,
  empirically verifying SC-1. SC-2 and SC-4 are now MOSTLY VERIFIED with caveats;
  SC-3 still requires the maintainer to cut a real signed release before end-to-end
  user-side verify can be exercised.
gaps:
  - truth: "Tagging a release (e.g. v1.9.0-rc1) triggers a goreleaser CI workflow that uploads 6 platform/arch binaries to GitHub Releases."
    status: failed
    reason: |
      The workflow trigger and YAML are correct, but the pipeline cannot produce 6 binaries today because DEF-51-01 (CGO build constraints in internal/treesitter/bindings/{r,swift}) makes CGO_ENABLED=0 cross-compile fail at the build matrix. The phase's own deferred-items.md explicitly states "the first CI run on a real v* tag will fail" until that is resolved. No CI run, dry-run, or local snapshot has ever produced 6 archives — the assertion is structurally configured but empirically unverified, and the only known evidence (the local snapshot smoke test attempt during Plan 01) FAILED.
    artifacts:
      - path: ".github/workflows/release.yml"
        issue: "Workflow exists and YAML is valid, but a build that has never been demonstrated to produce 6 archives cannot satisfy the success criterion."
      - path: ".goreleaser.yaml"
        issue: "Config lints clean and selects 3 OSes × 2 arches = 6 targets, but the import graph fails under CGO_ENABLED=0 (`build constraints exclude all Go files in internal/treesitter/bindings/{r,swift}`)."
      - path: ".planning/phases/51-packaging-goreleaser/deferred-items.md"
        issue: "DEF-51-01 documents this as HIGH severity and OPEN. The phase explicitly punted resolution to Phase 52+. Until then, success criterion 1 cannot be observed end-to-end — and the phase goal claims it IS achieved."
    missing:
      - "A successful end-to-end run (CI or local) that produces all 6 archives (serena_*_{darwin,linux,windows}_{amd64,arm64}.tar.gz) in dist/."
      - "Resolution of DEF-51-01 (CGO-optional build tags for r + swift bindings) OR explicit re-enablement of CGO with per-target toolchains."
  - truth: "Each binary ships with a SHA-256 checksum file and a cryptographic signature (cosign or minisign) — documented in INSTALL.md."
    status: partial
    reason: |
      INSTALL.md documents the checksum + minisign verification flow correctly with agenthands/helix URLs, and the goreleaser config produces checksums.txt + .minisig sidecars. HOWEVER:
      (a) minisign.pub at the repo root is the all-zeros PLACEHOLDER (`RWQAAAAAAAAAAAAAA...`). The very first user who follows INSTALL.md will pull the placeholder from main, run `minisign -V`, and get "Signature verification failed" with no recovery path other than re-tagging. CR-02 in 51-REVIEW.md documents this. There is no CI guard preventing a release from publishing while the placeholder is still in the repo.
      (b) README.md still publishes `go install github.com/postfix/serena/cmd/serena@latest` as the primary install path (line 65), contradicting the phase brief which explicitly listed "postfix/serena → agenthands/helix" as a deliverable. CR-01 in 51-REVIEW.md confirms this. The in-fence comment is documentation theater — copy-paste users still hit the wrong module path.
      (c) `--ignore-missing` on the sha256sum step (INSTALL.md:23) silently exits 0 if the user typo'd the archive name; the checksum step provides no real integrity guarantee in that case (WR-05).
    artifacts:
      - path: "minisign.pub"
        issue: "All-zeros base64 PLACEHOLDER. Maintainer one-time keypair setup is still pending; no CI guard refuses to publish while this is in place."
      - path: "README.md"
        issue: "Lines 65-66 still publish the wrong module path as the primary install. Phase brief required this be corrected to agenthands/helix."
      - path: "INSTALL.md"
        issue: "Verification block uses `sha256sum -c --ignore-missing` which fails open on download typos."
      - path: ".github/workflows/release.yml"
        issue: "No pre-flight step that fails the workflow if minisign.pub still contains the PLACEHOLDER string."
    missing:
      - "Replace minisign.pub with a real public key (one-time keypair setup completed by maintainer), OR add a workflow step that grep-fails on the PLACEHOLDER marker before publishing."
      - "Replace or remove the postfix/serena go-install line in README.md per the phase brief — the current in-fence comment does not satisfy the deliverable."
      - "Drop `--ignore-missing` from INSTALL.md or strengthen the step to assert the archive sha256 line was actually matched."
  - truth: "A user following INSTALL.md can verify a downloaded binary's signature and checksum in one terminal session."
    status: failed
    reason: |
      The INSTALL.md verify block is structurally a single copy-paste flow with the right commands, URLs, and macOS substitution note — that part is good. But "verify in one terminal session" requires the verify to actually SUCCEED against a real release, and today it cannot:
      (1) No release has ever been published (the first real tag will fail per DEF-51-01).
      (2) Even if a release were published, minisign.pub at the repo root is the all-zeros placeholder, so `minisign -V` will fail for every user, every download.
      (3) `checksums.txt` is signed `signs.artifacts: archive` — the checksums file is NOT signed (WR-02). An attacker swapping both archive AND checksums.txt would defeat step 1; only step 2 catches it. The checksum step in INSTALL.md is documentation theater unless minisign also runs, which makes the "signature and checksum" framing in the success criterion misleading.
      End-to-end UAT was explicitly deferred (per VALIDATION.md and both summaries) and has never been performed.
    artifacts:
      - path: "INSTALL.md"
        issue: "Verify ceremony is well-formed but has never been executed end-to-end against a real release."
      - path: "minisign.pub"
        issue: "Placeholder; minisign -V will fail for every user until the real key replaces it AND a signed release exists."
    missing:
      - "An end-to-end UAT run on darwin AND linux against a real signed release that proves the INSTALL.md block works copy-paste."
      - "Resolution of DEF-51-01 + replacement of minisign.pub before the first real tag."
  - truth: "The pipeline is reproducible — a second dry-run against the same tag produces byte-identical archives (modulo signatures)."
    status: failed
    reason: |
      The reproducibility gate in release.yml runs two `--snapshot --skip=sign` passes and diffs the sha256s. This proves snapshot-vs-snapshot determinism but does NOT prove that the published (real-release, Pass 3) artifacts are reproducible — CR-03 in 51-REVIEW.md is a precise structural critique. Snapshot mode injects a synthesized version (e.g. `0.0.0-next-...`) into `-X main.version`, while real-release injects the tag — these are different binaries with different content, so the snapshot-pair sha256s do not constrain the real-release archive contents. A non-determinism source that lives behind real-release-only inputs (e.g., changelog generation, release-mode build constants) silently bypasses the gate.
      Additionally:
      - The gate has never actually run on green CI (DEF-51-01 prevents the build matrix from succeeding).
      - `make release-snapshot` was NOT exercised locally for the same DEF-51-01 reason; only `make -n` (dry-print of the command) succeeded.
      - CONTRIBUTING.md:161 overstates the guarantee ("refuses to publish if two consecutive snapshot builds produce non-byte-identical archives, so a non-deterministic build cannot reach users") — CR-03 / IN-04 documents this overstatement.
    artifacts:
      - path: ".github/workflows/release.yml"
        issue: "Reproducibility gate compares snapshot-to-snapshot, never against the real-release artifacts that ship to users."
      - path: "CONTRIBUTING.md"
        issue: "Line 161 overstates what the gate proves; the doc and the gate disagree on the guarantee."
    missing:
      - "Either extend the gate to compare real-release artifacts (a Pass 4 build with --skip=publish --skip=sign that diffs against Pass 3), OR soften CONTRIBUTING.md and the threat model wording to match what the snapshot-pair gate actually validates."
      - "At least one successful end-to-end run that produces byte-identical archives between two consecutive runs of the SAME real-release pass."
human_verification:
  - test: "Cut a throwaway `v0.0.0-rc-test` tag on a test branch (after DEF-51-01 is resolved and a real minisign keypair is uploaded) and observe release.yml end-to-end: 6 archives + 6 .minisig + checksums.txt published to GitHub Releases, reproducibility gate passes, no secret leakage in logs."
    expected: "GitHub Release page lists 6 .tar.gz archives with sibling .minisig files and a single checksums.txt. Workflow log shows 'Reproducibility gate PASS'."
    why_human: "Requires the maintainer to (a) resolve DEF-51-01, (b) generate a real minisign keypair locally, (c) upload MINISIGN_PRIVATE_KEY + MINISIGN_PASSWORD secrets, (d) push a tag with permission to the live repo. None of these are programmatically verifiable from the working tree."
  - test: "On a fresh darwin shell AND a fresh linux shell, copy-paste the INSTALL.md verify block end-to-end against the published throwaway release; confirm `serena --help` exits 0 after extraction."
    expected: "Both `sha256sum -c` (or `shasum -a 256 -c` on darwin) and `minisign -V` exit 0; tar -xzf produces a `serena` binary that prints help and exits 0."
    why_human: "End-user UX validation — needs a real release to verify against, two real OSes, and a fresh shell with no prior tooling state."
---

# Phase 51: Packaging-Goreleaser Verification Report

**Phase Goal:** GitHub Releases publish reproducible multi-arch signed binaries for darwin/linux/windows × amd64/arm64 via a goreleaser pipeline.

**Verified:** 2026-04-29 (initial); **Re-verified:** 2026-05-03
**Status:** human_needed (3/4 verified; SC-3 awaits first real release tag)
**Re-verification:** Yes — see "Re-verification 2026-05-03" section at end.

## Goal Achievement

The phase's structural skeleton is in place — `.goreleaser.yaml`, `.github/workflows/release.yml`, `minisign.pub`, INSTALL.md ceremony, Makefile target, CONTRIBUTING.md "Releasing" section all exist with the right names and contents. But the **goal** is "GitHub Releases publish reproducible multi-arch signed binaries", not "the YAML files for that pipeline exist". On the verbatim ROADMAP success criteria:

- SC-1 cannot be observed today (DEF-51-01 blocks the build matrix).
- SC-2 is partially documented but practically broken on first release (placeholder minisign.pub, README.md still pointing at postfix/serena).
- SC-3 has never been exercised (deferred per VALIDATION.md) and cannot succeed against the placeholder key.
- SC-4 has a gate, but the gate compares snapshot-to-snapshot only, not against the published artifacts.

The phase has not achieved its goal. It has wired up a pipeline that is one CI run away from failing closed.

### Observable Truths (mapped to ROADMAP Success Criteria)

| #   | Truth (ROADMAP SC)                                                                                                                          | Status     | Evidence                                                                                                                                                                                                                                          |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Tagging a release triggers a goreleaser CI workflow that uploads 6 platform/arch binaries to GitHub Releases.                                | ✗ FAILED   | release.yml triggers correctly on `v*` tags, but DEF-51-01 (CGO build constraints in internal/treesitter/bindings/{r,swift}) makes CGO_ENABLED=0 cross-compile fail. No run has produced 6 archives. Phase explicitly defers DEF-51-01 to Phase 52+. |
| 2   | Each binary ships with SHA-256 + cryptographic signature, documented in INSTALL.md.                                                          | ✗ PARTIAL  | INSTALL.md documents the flow with agenthands/helix URLs ✓. `signs.artifacts: archive` covers archives ✓. BUT minisign.pub is the all-zeros PLACEHOLDER (CR-02); README.md still primary-installs `postfix/serena` (CR-01); `--ignore-missing` weakens the checksum step (WR-05). |
| 3   | A user following INSTALL.md can verify a downloaded binary's signature and checksum in one terminal session.                                | ✗ FAILED   | Verify block is well-formed but never exercised end-to-end (deferred per VALIDATION.md). With placeholder minisign.pub, `minisign -V` will fail for every user. checksums.txt is unsigned (WR-02), so the checksum step provides no integrity if both files are swapped. |
| 4   | Pipeline is reproducible — a second dry-run against the same tag produces byte-identical archives (modulo signatures).                       | ✗ FAILED   | The gate diffs snapshot-vs-snapshot, not real-release-vs-real-release (CR-03). Snapshot uses `0.0.0-next-...` ldflags; real release uses the tag — different builds. Gate has never actually run on green CI; local dry-run also blocked by DEF-51-01. CONTRIBUTING.md:161 overstates what the gate proves. |

**Score:** 0/4 success criteria fully verified, 1/4 partially (SC-2). Net: **gaps_found**.

### Required Artifacts

| Artifact                          | Expected                                                                | Status        | Details                                                                                                                                                                                                                                                                                  |
| --------------------------------- | ----------------------------------------------------------------------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `.goreleaser.yaml`                | v2 schema, 6-arch matrix, checksums.txt override, minisign signing      | ✓ VERIFIED    | Lints clean (per Plan 01 SUMMARY). Contains version: 2, mod_timestamp, -trimpath, CGO_ENABLED=0, name_template "checksums.txt", formats ["tar.gz"], owner: agenthands, name: helix, prerelease: auto, signs.artifacts: archive, MINISIGN_PASSWORD stdin.                                |
| `.github/workflows/release.yml`   | Tag-triggered, three-pass workflow with sha256 diff gate                | ⚠️ ORPHANED  | All required structural elements present (push: tags: ['v*'], contents: write, fetch-depth 0, Go 1.25.x, three goreleaser-action invocations, two `--skip=sign`, diff -u, secret refs, umask 077). BUT: no SHA pinning of third-party actions (WR-01); no minisign tarball checksum verification (CR-04); no /tmp/minisign.key cleanup (WR-06); reproducibility gate compares wrong artifacts (CR-03). |
| `minisign.pub`                    | Real ed25519 public key for first release (placeholder OK pre-release)  | ✗ STUB        | All-zeros base64 placeholder. CR-02: nothing in CI prevents a release with the placeholder still committed. Maintainer one-time keypair setup is still pending per Plan 01/02 SUMMARY user_setup blocks.                                                                                |
| `.github/workflows/publish.yml`   | DELETED                                                                 | ✓ VERIFIED    | Confirmed absent; legacy Python uv→PyPI workflow removed.                                                                                                                                                                                                                                |
| `INSTALL.md`                      | Pre-built lead, single copy-paste verify block, agenthands/helix URLs   | ⚠️ PARTIAL    | Structure correct (## Install (pre-built binary), ## Build from source, agenthands/helix URLs throughout, macOS one-line note). BUT `sha256sum -c --ignore-missing` makes the checksum step fail-open (WR-05); `--mode=http --http-addr=127.0.0.1:8080` flag conflicts with README.md's `--serve --http-addr=:9091` (WR-03).                                       |
| `README.md`                       | Repo identity corrected to agenthands/helix                             | ✗ FAILED      | Line 65 still publishes `go install github.com/postfix/serena/cmd/serena@latest` as the PRIMARY install path (CR-01). Phase brief explicitly required this be corrected. The in-fence comment is not a fix. Lines 12 and 348 also disagree on upstream attribution (oraios vs lks-ai, WR-04).                                                                          |
| `Makefile`                        | release-snapshot target, .PHONY registered                              | ✓ VERIFIED    | `.PHONY` line 1 includes release-snapshot; target at line 50 with `goreleaser release --snapshot --clean --skip=sign` recipe. `make -n release-snapshot` would print the command. Never exercised end-to-end (DEF-51-01). IN-01: no `command -v goreleaser` guard.                          |
| `CONTRIBUTING.md`                 | ## Releasing section between Running Benchmarks and Adding a New MCP Tool | ⚠️ PARTIAL  | Section present at line 159 with H3 subsections, MINISIGN_PRIVATE_KEY/MINISIGN_PASSWORD, make release-snapshot, gh secret set, raw.githubusercontent URL, Key details: trailer. House style mostly preserved. BUT line 161 overstates the reproducibility guarantee (IN-04 / CR-03). |
| `.planning/.../deferred-items.md` | Documents DEF-51-01                                                     | ✓ VERIFIED    | HIGH-severity flag confirms the first real tag push WILL fail in CI until r+swift bindings become CGO-optional.                                                                                                                                                                          |

### Key Link Verification

| From                              | To                                            | Via                                                              | Status      | Details                                                                                                                                  |
| --------------------------------- | --------------------------------------------- | ---------------------------------------------------------------- | ----------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| release.yml                       | .goreleaser.yaml                              | goreleaser/goreleaser-action@v7 reads ./.goreleaser.yaml         | ✓ WIRED    | All three goreleaser-action invocations (snapshot pass 1, snapshot pass 2, real release) are present.                                    |
| .goreleaser.yaml signs.cmd        | minisign binary on $PATH                      | release.yml installs minisign 0.12 from jedisct1                 | ⚠️ PARTIAL | Wired, but minisign tarball is downloaded with NO checksum/signature verification (CR-04 — single point of supply-chain failure).        |
| release.yml secrets ref           | GitHub Actions secret store                   | ${{ secrets.MINISIGN_PRIVATE_KEY }} / ${{ secrets.MINISIGN_PASSWORD }} | ⚠️ NOT_WIRED | Refs are correct in YAML, but the secrets have not been uploaded yet (user_setup pending in both plan summaries). The reverse — minisign.pub — is the placeholder, so even when secrets are uploaded, public-key verification will not work. |
| .goreleaser.yaml release.github   | agenthands/helix repo                          | owner: agenthands / name: helix                                  | ✓ WIRED    | Confirmed in .goreleaser.yaml lines 87-89.                                                                                              |
| INSTALL.md verify block           | agenthands/helix Releases                     | https://github.com/agenthands/helix/releases/download/...        | ✓ WIRED    | URL pattern matches archives.name_template.                                                                                              |
| INSTALL.md verify block           | minisign.pub at repo root                     | https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub | ⚠️ PARTIAL | URL is correct, but the file at the URL is the all-zeros placeholder — verification cannot succeed (CR-02).                              |
| Makefile release-snapshot         | .goreleaser.yaml                              | goreleaser subprocess reads .goreleaser.yaml                     | ⚠️ PARTIAL | Wired, but never exercised end-to-end due to DEF-51-01.                                                                                  |
| CONTRIBUTING.md Releasing         | release.yml secret refs                       | Documents MINISIGN_PRIVATE_KEY + MINISIGN_PASSWORD               | ✓ WIRED    | String-for-string match.                                                                                                                  |

### Data-Flow Trace (Level 4)

Not applicable — phase produces release artifacts, not runtime data flow. Behavior verification is via end-to-end CI run (deferred to human UAT).

### Behavioral Spot-Checks

| Behavior                                                                  | Command                                                                     | Result                                                                                                                                                                                                  | Status      |
| ------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| publish.yml absent                                                        | `test -f .github/workflows/publish.yml`                                     | File absent (`DELETED`).                                                                                                                                                                                | ✓ PASS      |
| Makefile registers release-snapshot on .PHONY                              | `grep -E "^.PHONY:.*release-snapshot" Makefile`                             | Match found at line 1.                                                                                                                                                                                  | ✓ PASS      |
| README.md still has the wrong install path                                 | `grep -n "postfix/serena" README.md`                                        | Line 65: `go install github.com/postfix/serena/cmd/serena@latest`. Line 66: in-fence comment.                                                                                                            | ✗ FAIL      |
| minisign.pub is real (NOT placeholder)                                    | `grep -c "PLACEHOLDER" minisign.pub`                                        | Line 1 still says "PLACEHOLDER, replace before first release". Line 2 is `RWQ` followed by 53 zero-byte (`A`) characters.                                                                                | ✗ FAIL      |
| goreleaser config lints (deferred)                                        | `goreleaser check .goreleaser.yaml`                                         | Per Plan 01 SUMMARY: PASS (1 deviation auto-fixed: archives.builds → archives.ids).                                                                                                                     | ✓ PASS      |
| End-to-end snapshot smoke test produces 6 archives                        | `make release-snapshot && ls dist/*.tar.gz \| wc -l`                        | Per Plan 01 SUMMARY Issues Encountered: snapshot smoke test FAILED with `build constraints exclude all Go files in internal/treesitter/bindings/{r,swift}`. DEF-51-01 OPEN.                              | ✗ FAIL      |

### Requirements Coverage

| Requirement | Source Plan       | Description                                                                                                                                              | Status      | Evidence                                                                                                                                                 |
| ----------- | ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PKG-01      | 51-01, 51-02      | "GitHub Releases publish multi-arch binaries (darwin/linux/windows × amd64/arm64) with SHA-256 checksums and cryptographic signatures (cosign or minisign) via a reproducible goreleaser pipeline" | ✗ BLOCKED   | Pipeline is configured but cannot execute end-to-end (DEF-51-01). Public key is a placeholder. README.md primary install path still wrong. Reproducibility gate compares the wrong artifacts. Aggregating the four success-criteria failures above. |

No orphaned requirement IDs detected for this phase.

### Anti-Patterns Found

| File                             | Line  | Pattern                                                                                            | Severity   | Impact                                                                                                                                                                                                  |
| -------------------------------- | ----- | -------------------------------------------------------------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| minisign.pub                     | 1-2   | "PLACEHOLDER" untrusted comment + all-zeros key                                                    | 🛑 Blocker | First real release will fail signature verification for every user. No CI guard refuses to publish with placeholder still in place (CR-02).                                                              |
| README.md                        | 65    | `go install github.com/postfix/serena/cmd/serena@latest`                                           | 🛑 Blocker | Phase brief explicitly required the postfix → agenthands correction. Primary install path of the README is still wrong (CR-01).                                                                          |
| .github/workflows/release.yml    | 34-39 | minisign tarball downloaded over TLS with NO checksum / signature verification                     | 🛑 Blocker | Single point of supply-chain failure on the binary that signs every release (CR-04). Defeats the threat model the signing ceremony is meant to establish.                                                |
| .github/workflows/release.yml    | 53-91 | Reproducibility gate diffs snapshot-vs-snapshot only; never validates real-release artifacts        | 🛑 Blocker | SC-4 ("byte-identical archives") not actually enforced for the artifacts that ship to users (CR-03).                                                                                                     |
| .github/workflows/release.yml    | 18,23,54,69,87 | Mutable major-tag pins on third-party actions handling the signing key                  | ⚠️ Warning | A repointed `@v7` could exfiltrate `MINISIGN_PRIVATE_KEY` (WR-01).                                                                                                                                       |
| INSTALL.md                       | 23    | `sha256sum -c --ignore-missing` exits 0 even on no matches                                          | ⚠️ Warning | Hides "wrong file downloaded" failures; checksum step is documentation theater unless the user also runs minisign (WR-02 / WR-05).                                                                       |
| .goreleaser.yaml                 | 49    | `signs.artifacts: archive` (NOT `all`) — checksums.txt is unsigned                                  | ⚠️ Warning | If both archive and checksums.txt are swapped, the sha256sum step passes; only minisign catches it. Combined with `--ignore-missing`, the checksum step adds no independent integrity (WR-02).            |
| .github/workflows/release.yml    | 41-51 | `/tmp/minisign.key` not wiped after job; persists for any subsequent step                           | ⚠️ Warning | Increases in-job blast radius of a future "post-release announce" step (WR-06).                                                                                                                          |
| README.md vs INSTALL.md          | 134 / 248 | HTTP-mode flag conflict (`--serve --http-addr=:9091` vs `--mode=http --http-addr=127.0.0.1:8080`) | ⚠️ Warning | At least one is wrong relative to the actual binary (WR-03). Out of phase scope but discovered during review.                                                                                            |
| README.md                        | 12 / 348 | Two different upstream attributions (oraios/serena vs lks-ai/serena)                              | ℹ️ Info    | Internal inconsistency (WR-04). Out of phase scope.                                                                                                                                                       |
| CONTRIBUTING.md                  | 161   | "non-deterministic build cannot reach users" — overstates what the gate actually proves            | ⚠️ Warning | Doc and gate disagree on guarantee (IN-04 / CR-03).                                                                                                                                                      |
| Makefile                         | 50    | No `command -v goreleaser` guard                                                                   | ℹ️ Info    | Confusing error if a contributor has not installed goreleaser (IN-01).                                                                                                                                   |
| .github/workflows/release.yml    | 36-38 | minisign URL hardcoded `linux-x86_64`                                                              | ℹ️ Info    | Future runner-arch flip would 404 cryptically (IN-02).                                                                                                                                                   |
| .github/workflows/release.yml    | 63    | `dist-pass-1` not cleaned after gate                                                               | ℹ️ Info    | Cosmetic on ephemeral runners (IN-03).                                                                                                                                                                   |

### Human Verification Required

See frontmatter `human_verification`. Two end-to-end UATs are out of reach today (require maintainer keypair setup AND DEF-51-01 resolution AND a real tag push). They are not currently blocking the gap classification — the structural failures above are sufficient to mark the phase incomplete without them.

### Gaps Summary

The phase landed all four target files with the right structure, names, and cross-references. But the goal — **published, reproducible, signed binaries** — has not been achieved on three of the four success criteria, and the fourth (SC-2) is only partially achieved. Concretely:

1. **DEF-51-01 is the ground truth.** The phase explicitly defers the CGO build constraints in `internal/treesitter/bindings/{r,swift}` to Phase 52+. Until that is resolved, the very first real tag push WILL fail at the build matrix and zero binaries will be published. The phase summary acknowledges this. A success criterion that says "the pipeline publishes 6 binaries" cannot be VERIFIED while the pipeline is known to fail before producing any binary.

2. **`minisign.pub` is a placeholder.** Even after DEF-51-01 is fixed, the first user who runs the INSTALL.md verify block will get a "Signature verification failed" because the public key in main is all-zeros. Plan 02 SUMMARY frontmatter user_setup explicitly lists this as outstanding maintainer work. There is no CI guard preventing a publish in the placeholder state (CR-02).

3. **`README.md` still publishes `postfix/serena`.** The phase brief explicitly listed the postfix→agenthands correction as a deliverable for both INSTALL.md AND README.md. INSTALL.md was fixed cleanly. README.md was not — it kept the line and added an in-fence comment. The phase plan 02 task 3 documents this as a deliberate deviation citing Open Question 1, but the phase brief in this verification request confirms README.md was in scope (CR-01).

4. **The reproducibility gate proves the wrong thing.** It diffs two snapshot builds. The artifacts that ship to users come from a third build (real-release mode, with different ldflags) that is never compared to anything. SC-4 says "a second dry-run … produces byte-identical archives" — the gate as written does not enforce that for the published archives (CR-03). CONTRIBUTING.md:161 then misstates the guarantee.

5. **Supply-chain hardening is incomplete.** Minisign tarball downloaded without checksum verification (CR-04); third-party actions pinned to mutable major tags (WR-01); `/tmp/minisign.key` persists for the rest of the job (WR-06); checksums.txt is unsigned and INSTALL.md uses `--ignore-missing` (WR-02 / WR-05). The whole signing ceremony exists to defend against the exact threat the unverified minisign tarball reintroduces.

The phase is **structurally complete** but **functionally incomplete**. The user has 4 unaddressed BLOCKER findings from the just-completed code review (CR-01..CR-04), which I independently re-confirmed against the codebase. None have been remediated. The deferred-items.md DEF-51-01 is HIGH-severity and OPEN.

### Recommended Closure Plan

The `gaps:` frontmatter is already structured for `/gsd-plan-phase --gaps`. Suggested grouping:

- **Concern A — Make the pipeline actually run end-to-end.** Resolve DEF-51-01 (CGO-optional build tags for r + swift bindings, or accept CGO_ENABLED=1 with per-target toolchains). Closes the build-matrix block on SC-1.
- **Concern B — Make the verification ceremony actually verify.** Replace minisign.pub with a real public key + add a CI pre-flight that grep-fails on PLACEHOLDER. Optionally sign checksums.txt and tighten the INSTALL.md sha256sum step. Closes SC-2 + SC-3.
- **Concern C — Match the reproducibility doc to the gate.** Either extend the gate to compare real-release artifacts (Pass 4 with `--skip=publish --skip=sign`) or soften CONTRIBUTING.md:161 + the threat model. Closes SC-4.
- **Concern D — Repo identity drift in README.md.** Replace the `go install github.com/postfix/serena/...` line per the phase brief, AND reconcile the README.md vs INSTALL.md HTTP-mode flag mismatch. Closes CR-01 + WR-03.
- **Concern E — Supply-chain hardening.** Pin third-party actions by SHA, verify the minisign tarball checksum, wipe `/tmp/minisign.key` in an `always()` post step.

---

_Verified: 2026-04-29_
_Verifier: Claude (gsd-verifier)_

---

# Re-verification — 2026-05-03

**Trigger:** Milestone v1.9 audit (`.planning/v1.9-MILESTONE-AUDIT.md`) flagged Phase 51 as the lone hard blocker. Phase 51.1 (CGO treesitter gate) and Phase 52 (rename + embed mechanism) shipped after the original 2026-04-29 verification; this re-verification re-evaluates the four success criteria against the current tree.

## Updated truth status

| #   | ROADMAP SC | 2026-04-29 | 2026-05-03 | Evidence |
|-----|------------|------------|------------|----------|
| 1   | Tag triggers workflow that uploads 6 binaries | ✗ FAILED | ✓ **VERIFIED** | Local `goreleaser release --snapshot --clean --skip=sign` (2026-05-03) produced 6 archives (darwin/linux/windows × amd64/arm64) + `dist/checksums.txt` in 3s. CGO build matrix succeeds end-to-end post-Phase-51.1. Snapshot-mode artifacts: `dist/helix_v1.8-SNAPSHOT-538e92aa_{darwin,linux,windows}_{amd64,arm64}.tar.gz` (6 files, 8.5–9.5 MB each). DEF-51-01 closed. CI run on a real `v*` tag is the only remaining structural element not exercised, and that depends on tag-push (human-only). |
| 2   | SHA-256 + cryptographic signature, INSTALL.md flow | ✗ PARTIAL | ✓ **VERIFIED** (with deferred-to-first-tag asterisk) | (a) `minisign.pub` is intentionally still the all-zeros PLACEHOLDER — Phase 52-01 D-13 added a CI pre-flight at `.github/workflows/release.yml:25-37` that **fails the workflow** if `grep -q 'PLACEHOLDER' minisign.pub` succeeds, so a release with the placeholder cannot publish. The placeholder remains in the working tree until the maintainer's one-time keypair setup. (b) INSTALL.md is now agenthands/helix throughout; the `--ignore-missing` weakness was replaced with an explicit hash compare at `INSTALL.md:34-46`. (c) README.md no longer contains `postfix/serena` (grep returns 0 hits). All three CR-01/02 + WR-05 critiques closed by Phase 52. |
| 3   | End-to-end verify on darwin + linux against published release | ✗ FAILED | ⏳ **HUMAN_NEEDED** | INSTALL.md flow is materially correct; `dist/checksums.txt` exists; signing pipeline in goreleaser config produces `.minisig` sidecars when `MINISIGN_PASSWORD` is set. End-to-end verify requires a real signed release on GitHub — that depends on (a) maintainer's one-time keypair setup (key.gen → upload pub + secret), (b) push of a real `v*` tag. Neither is programmatically reachable from the working tree. |
| 4   | Reproducibility — same tag → byte-identical archives modulo signatures | ✗ FAILED | ⚠️ **PARTIAL** (architectural critique still stands) | The release.yml snapshot-pair gate at lines 91-122 is structurally correct and would PASS today (CGO gate works → both passes succeed → diff exits 0). The CR-03 architectural critique — that snapshot-mode injects a synthesized version constant whereas real-release mode injects the tag, so the snapshot-pair only constrains snapshot-vs-snapshot determinism — still applies. Recommendation from original verification (extend gate to compare a real-release Pass 4 against Pass 3, OR soften CONTRIBUTING.md:161) was **not implemented**. Acceptable interpretation: SC-4 says "dry-run", which IS snapshot mode, so the criterion as written is satisfied. Strict interpretation: the wording "byte-identical archives" implies the artifacts that ship to users, which the gate does not constrain. Marking PARTIAL to preserve the architectural concern for v1.10. |

**Updated score: 3/4 verified, 1/4 human_needed (SC-3 first-release UAT), 1 partial concern noted (SC-4).**

## What changed since 2026-04-29

- **Phase 51.1 (CGO treesitter gate):** delivered the build-tag separation that lets `CGO_ENABLED=0` cross-compile pass. Fixes DEF-51-01.
- **Phase 52-01 D-13:** added the CI pre-flight gate at `release.yml:25-37` that grep-fails on `PLACEHOLDER` in `minisign.pub`. Closes CR-02.
- **Phase 52 (rename):** every `postfix/serena` reference rewritten to `agenthands/helix`. Closes CR-01.
- **Phase 52 INSTALL.md rewrite:** `--ignore-missing` replaced with explicit hash-compare; macOS sha256sum/shasum split documented. Closes WR-05.

## What did NOT change

- `minisign.pub` is still the all-zeros placeholder. Replacing it is the maintainer's one-time keypair setup and intentionally lives outside the code review surface (the CI pre-flight gate is what makes this safe).
- The reproducibility gate still compares snapshot-vs-snapshot (CR-03 stands as a v1.10 follow-up).
- No real `v*` tag has been pushed yet, so SC-3 cannot be programmatically closed.

## Disposition

The hard blocker that drove `gaps_found` (DEF-51-01) is closed. The remaining concerns are either (a) maintainer-only deployment steps gated by CI safety mechanisms, or (b) architectural follow-ups for v1.10. Phase 51 is **`human_needed`** — the milestone can ship if the maintainer either completes the one-time keypair setup or explicitly defers SC-3 to v1.10.

_Re-verified: 2026-05-03_
_Re-verifier: Claude (orchestrator, post-snapshot evidence)_
