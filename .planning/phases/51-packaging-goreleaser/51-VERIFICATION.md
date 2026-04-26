---
phase: 51-packaging-goreleaser
verified: 2026-04-26T12:30:00Z
status: human_needed
score: 4/4 must-haves verified (with 1 FLAG on darwin runner; 2 SCs require post-merge tag-driven verification)
overrides_applied: 0
human_verification:
  - test: "Push a throwaway tag (e.g. v0.0.0-test1) and observe release.yml run end-to-end on GitHub Actions"
    expected: "All 6 archives + checksums.txt + 12 sig/cert files (or 14 incl. checksum sig+cert) appear on the auto-created GitHub Release; cosign signing succeeds against Sigstore Fulcio + Rekor"
    why_human: "SC#1 (tag triggers workflow + uploads 6 binaries) cannot be verified locally — requires real GitHub Actions runner with OIDC; only the wiring is in-tree"
  - test: "Download one signed archive from the test release and run the INSTALL.md verify command end-to-end"
    expected: "cosign verify-blob with --certificate-identity-regexp succeeds AND sha256sum -c checksums.txt succeeds"
    why_human: "SC#3 (one-terminal-session verify) is a UX claim that needs a real signed artifact, not a snapshot"
  - test: "Run `make release-verify` (or the manual snapshot+diff ritual in RELEASING.md) twice on the same commit"
    expected: "diff -r with the documented exclusions returns empty (byte-identical archives modulo signatures)"
    why_human: "SC#4 reproducibility requires a working local goreleaser+zig+CGO toolchain that can produce all 6 archives; the macOS host cannot cross-compile darwin targets due to the open zig+darwin SDK gap (see FLAG below)"
  - test: "Decide darwin cross-compile strategy (Option A drop / Option B macos-latest split / Option C goreleaser-cross image) before tagging the first real release"
    expected: "release.yml updated to actually produce darwin archives (or the matrix explicitly reduced); current pipeline ships 6 targets on paper but darwin builds will fail on ubuntu-latest without an Apple SDK / osxcross / native darwin runner"
    why_human: "Architectural decision deferred from 51-01 (see SUMMARY 'Issues Encountered'); requires maintainer judgment, not code"
flags:
  - issue: "darwin cross-compile from ubuntu-latest runner is not actually proven to work"
    detail: "CGO_ENABLED=1 + zig overrides for darwin/{amd64,arm64} require Apple SDK headers (mach/mach_vm.h) that zig does not redistribute. Local snapshot smoke on macOS host failed for darwin targets. The release.yml currently runs only on ubuntu-latest with no Apple SDK, so a real tag push will likely fail darwin builds. Pipeline machinery accommodates Options A/B/C but the decision is unmade."
    severity: WARNING
    classification: "FLAG (not FAIL) — the goreleaser+release.yml machinery exists and validates; the gap is a runtime cross-compile capability question that surfaces only on first tag push (honest signal, by design)."
deferred:
  - truth: "Pre-existing Java integration test flakes (TestSymbols_JavaFixture, TestEdit_JavaFixture/replace_body)"
    addressed_in: "Phase 48 (bug-jdtls-warm-cache) and/or Phase 56 (bug-ls-notification-dispatch-and-jdtls-readiness)"
    evidence: "Reproduces on base commit 75c9afde before any Phase 51 edits; documented in .planning/phases/51-packaging-goreleaser/deferred-items.md; Phase 48 goal explicitly targets warm jdtls workspace for default `go test ./...` and Phase 56 ships JdtlsAdapter.WaitUntilJavaReady"
---

# Phase 51: packaging-goreleaser Verification Report

**Phase Goal (ROADMAP, contract wording):** GitHub Releases publish reproducible multi-arch signed binaries for darwin/linux/windows × amd64/arm64 via a goreleaser pipeline.

**Phase Goal (Verifier-task wording):** Multi-arch signed release pipeline via goreleaser as the foundation for downstream channels.

**Verified:** 2026-04-26
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal-Backward Analysis

For "multi-arch signed release pipeline as foundation for downstream channels" to be TRUE, the codebase must:

1. **Have a goreleaser config** that declares the 6-target matrix and signing — so Phase 52 (Homebrew/Scoop/nfpms) has artifacts to consume.
2. **Have a tag-triggered workflow** with the right OIDC permissions wired to that config — so the pipeline actually fires.
3. **Have version metadata injection** matching the config's ldflag targets — so released binaries report a meaningful `--version`.
4. **Document end-user verification and maintainer reproducibility** — so SC#3 (one-session verify) and SC#4 (reproducibility) are achievable by humans, not just by reading code.
5. **Not regress existing CI** — `go-test.yml`, `docker.yml` untouched; legacy Python `publish.yml` removed per the "single Go binary" framing.

All five conditions are met by what's on disk. The remaining gap is purely runtime — proof that a real tag push produces all 6 signed archives — which is intentionally a post-merge ritual per D-22 and is captured in the `human_verification` section.

## Goal Achievement — Success Criteria

| # | ROADMAP Success Criterion | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Tagging a release (e.g. `v1.9.0-rc1`) triggers a goreleaser CI workflow that uploads 6 platform/arch binaries to GitHub Releases | PASS (machinery) / HUMAN (runtime proof) | `.github/workflows/release.yml` exists with `on.push.tags: ['v*']`, `permissions: contents:write + id-token:write`, pinned `goreleaser/goreleaser-action@v6.4.0`, `actions/checkout@v4 fetch-depth:0`, `actions/setup-go@v5 go-version:1.25.x`, `mlugg/setup-zig@v1`, `sigstore/cosign-installer@v3.7.0`. `.goreleaser.yml` declares the 6-target build matrix (linux/darwin/windows × amd64/arm64). Tag-driven end-to-end run is a post-merge ritual (D-22). |
| 2 | Each binary ships with a SHA-256 checksum file and a cryptographic signature (cosign or minisign) — documented in INSTALL.md | PASS | `.goreleaser.yml` declares `checksum: { algorithm: sha256, name_template: checksums.txt }` and TWO `signs:` entries (`artifacts: checksum` + `artifacts: archive`) using cosign keyless two-file form (`--output-signature` + `--output-certificate`). INSTALL.md `## Verify a release binary` documents both `cosign verify-blob` and `sha256sum -c checksums.txt`. |
| 3 | A user following INSTALL.md can verify a downloaded binary's signature and checksum in one terminal session | PASS (docs) / HUMAN (real-artifact proof) | INSTALL.md ships a single copy-paste-able block: download archive + .sig + .pem + checksums.txt → cosign verify-blob with the EXACT D-03 `--certificate-identity-regexp '^https://github\.com/postfix/serena/\.github/workflows/release\.yml@refs/tags/v.*$'` → sha256sum verify. End-to-end runtime verification requires a real signed artifact (post-tag). |
| 4 | The pipeline is reproducible — a second dry-run against the same tag produces byte-identical archives (modulo signatures) | PASS (machinery + ritual) / HUMAN (proof on working toolchain) | Reproducibility knobs in `.goreleaser.yml`: `-trimpath`, `-buildvcs=false`, `mod_timestamp: {{ .CommitTimestamp }}`, `archives.builds_info.mtime: {{ .CommitTimestamp }}`, ldflags use `{{.CommitDate}}` (not `{{.Date}}`). RELEASING.md and `make release-verify` codify the manual diff ritual with all 6 Q6 exclusions verbatim. Note: maintainer cannot run the FULL ritual on a non-darwin host today due to the darwin cross-compile gap (FLAG). |

**Score:** 4/4 success criteria met at machinery+docs level; 3 of 4 require post-merge human verification per D-22.

## Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `.goreleaser.yml` | v2 schema, 6-binary matrix, dual cosign signs, reproducibility knobs | VERIFIED | `version: 2`, 3×goos × 2×goarch, two `signs:` blocks, `mod_timestamp` + `builds_info.mtime`, ldflags target `internal/cli` (matches Task 1). DEVIATION: `CGO_ENABLED=1` + per-target `overrides:` with `CC=zig cc -target …` (documented inline; reproducibility intent preserved). `goreleaser check` exits 0. |
| `.github/workflows/release.yml` | Tag-triggered, ubuntu-latest Go 1.25.x, contents+id-token only, pinned actions, fetch-depth: 0 | VERIFIED | All checks confirmed via Read + grep. Adds `mlugg/setup-zig@v1` (consequence of CGO deviation). |
| `.github/workflows/publish.yml` | Deleted | VERIFIED | `git ls-files .github/workflows/publish.yml` returns nothing; `git grep publish.yml .github/` returns nothing; ls of `.github/workflows/` confirms absence. |
| `.github/workflows/docker.yml` | Byte-identical to main | VERIFIED | `git diff main -- .github/workflows/docker.yml` is empty. |
| `internal/cli/version.go` | Version/Commit/Date vars + FormatVersion() | VERIFIED | File present; vars + sentinel defaults + 7-char commit truncation; matches PLAN spec verbatim. |
| `internal/cli/version_test.go` | 4 tests incl. regression gate | VERIFIED | TestFormatVersionDefault, TestFormatVersionInjected, TestFormatVersionShortCommit, TestRootGoLiteralRemoved — all 3 runnable Format tests PASS in `go test ./internal/cli/ -run Version -v`. |
| `internal/cli/root.go` | Calls FormatVersion() | VERIFIED | Line 62: `fmt.Println(FormatVersion())`. Bare literal removed (regression test guards it). |
| `cmd/serena/main.go` | Doc comment pointing at internal/cli ldflag target | VERIFIED | Comment at lines 4-5 + import of `github.com/postfix/serena/internal/cli`. |
| `INSTALL.md` | Download + Verify sections after Prerequisites, before Quick Start, with exact D-03 regex | VERIFIED | Sections at lines 31 and 53; Quick Start at 87. Regex on line 74 matches D-03 verbatim. |
| `RELEASING.md` | Maintainer guide with snapshot+diff ritual, all 6 Q6 exclusions, tag/push, post-merge ritual, out-of-scope | VERIFIED | All 6 exclusions present (`*.sig`, `*.pem`, `checksums.txt*`, `artifacts.json`, `metadata.json`, `config.yaml`); both stable + pre-release tag examples; cross-link to INSTALL.md verify section; out-of-scope covers Phase 52, docker.yml, SBOM. |
| `README.md` | Single-line pointer to INSTALL.md, no cosign duplication | VERIFIED | Line 74: pointer present; `! grep -q certificate-identity-regexp README.md` (no duplication). |
| `Makefile` | build target with -ldflags from git describe; release-snapshot/release-verify targets | VERIFIED | VERSION/COMMIT/DATE shell-out; LDFLAGS targets match `internal/cli` package; both new targets in `.PHONY` and recipes present. |

## Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `.github/workflows/release.yml` | `.goreleaser.yml` | `args: release --clean` | WIRED | Line 54 of release.yml. |
| `.goreleaser.yml builds[0].ldflags` | `internal/cli` package vars | `-X github.com/postfix/serena/internal/cli.{Version,Commit,Date}` | WIRED | All three -X bindings present at lines 88-90; package path matches `internal/cli/version.go` declaration. |
| `internal/cli/root.go --version branch` | `internal/cli/version.go FormatVersion()` | function call | WIRED | `fmt.Println(FormatVersion())` at root.go:62. |
| `.github/workflows/release.yml` | zig cross-compiler | `mlugg/setup-zig@v1` step | WIRED | Lines 41-44; precedes goreleaser step. |
| `INSTALL.md verify section` | `release.yml` workflow identity | D-03 certificate-identity-regexp | WIRED | Verbatim match at INSTALL.md:74. |
| `Makefile build` | `internal/cli` vars | -ldflags from git describe | WIRED | Lines 10-20; package path matches goreleaser config. |
| `RELEASING.md ritual` | `.goreleaser.yml` snapshot output | diff -r with Q6 exclusions | WIRED | All 6 exclusions present; `make release-verify` mirrors. |
| `README.md` | `INSTALL.md` prebuilt+verify | markdown anchor link | WIRED | `INSTALL.md#download-a-prebuilt-binary` at README.md:74. |

## Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| `serena --version` (binary) | Version/Commit/Date | ldflag injection from goreleaser OR Makefile | YES (proven) | FLOWING — `go build -ldflags "-X .../cli.Version=1.9.0 -X .../cli.Commit=deadbeefcafe1234 -X .../cli.Date=2026-04-26" ./cmd/serena && ./serena --version` produced `serena version 1.9.0 (commit deadbee, built 2026-04-26)` (commit truncated correctly). Default sentinel path also works (other tests). |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| version tests pass | `go test ./internal/cli/ -run Version -v` | 3 RUN/PASS lines for FormatVersion tests | PASS |
| ldflag injection round-trip | `go build -ldflags "..." -o /tmp/serena-test ./cmd/serena && /tmp/serena-test --version` | `serena version 1.9.0 (commit deadbee, built 2026-04-26)` | PASS |
| goreleaser config valid | `goreleaser check` | "1 configuration file(s) validated" | PASS |
| publish.yml removed | `git ls-files .github/workflows/publish.yml` + `git grep -l publish.yml .github/` | empty / no match | PASS |
| docker.yml unchanged | `git diff main -- .github/workflows/docker.yml` | empty | PASS |
| `goreleaser release --snapshot --clean` produces 6 archives | not run locally | n/a | SKIP — requires zig+CGO toolchain to produce darwin archives; this is the FLAG. Verified by Plan 51-01 SUMMARY for linux/windows targets. |

## Anti-Patterns Found

None blocking. The CGO deviation from D-13 is documented inline in `.goreleaser.yml` lines 10-31 with rationale and reproducibility-preservation argument; this is a deliberate, declared deviation, not a smell.

## Requirements Coverage

| Requirement | Source Plan | Description (from REQUIREMENTS.md context) | Status | Evidence |
| --- | --- | --- | --- | --- |
| PKG-01 | 51-01 + 51-02 | Multi-arch signed release pipeline via goreleaser | SATISFIED (machinery + docs) / NEEDS HUMAN (live tag run) | All four ROADMAP success criteria met at the artifact level; tag-driven end-to-end is intentionally post-merge per D-22. |

## Human Verification Required (also in frontmatter)

1. **First tag push end-to-end** — Push `v0.0.0-test1` (or similar throwaway) and confirm: workflow fires, all 6 archives + checksums + sigs uploaded to the auto-created GitHub Release, cosign keyless against Fulcio/Rekor succeeds. Closes SC#1's runtime half.
2. **Real-artifact verify** — Download one signed archive from the test release, run the INSTALL.md `## Verify a release binary` block end-to-end, confirm both cosign and sha256sum succeed. Closes SC#3's runtime half.
3. **Reproducibility ritual on a working toolchain** — Run `make release-verify` (or the manual snapshot+diff) on a host where all 6 targets compile; confirm empty diff. Closes SC#4's runtime half.
4. **Darwin runner decision** — Pick Option A (drop darwin), Option B (parallel macos-latest job), or Option C (goreleaser-cross Docker image). Update `release.yml` accordingly before the first real release tag.

## Gaps Summary

There are **no must-have gaps in the codebase artifacts**. Every file PLAN frontmatter required exists, has the required content, and is wired to its consumers. The four ROADMAP Success Criteria are all met at the level of "the pipeline machinery and documentation are in place to deliver this." Three of the four (SC#1, SC#3, SC#4) have a runtime confirmation half that can only be done on real GitHub Actions infrastructure with a real tag push — that is intentional per decision D-22 (post-merge maintainer ritual, not PR-blocking).

The one notable FLAG is the **darwin cross-compile gap**: the `.goreleaser.yml` declares all 6 targets on paper, but cross-compiling darwin from `ubuntu-latest` with CGO+zig requires an Apple SDK that zig does not bundle. The Phase 51-01 SUMMARY explicitly handed this forward as an open architectural question with three documented options. The pipeline file accommodates any of them. This is classified as a WARNING/FLAG rather than a FAIL because:

- The phase boundary stops at "machinery + docs landed in PR"; tag verification is post-merge by design.
- The current `.goreleaser.yml` does not need editing for Options A or B — only `release.yml` shifts. For Option C, only `release.yml` shifts to a container.
- The honest-signal precedent (Phase 50 D-A2) is that the gap surfaces loudly on first real tag rather than being papered over.

The deferred Java integration flakes are unrelated to Phase 51 (reproduce on base commit) and addressed by Phases 48 / 56.

---

_Verified: 2026-04-26_
_Verifier: Claude (gsd-verifier)_
