# Phase 51: packaging-goreleaser - Context

**Gathered:** 2026-04-28
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers a **goreleaser-driven GitHub Releases pipeline** that publishes
6 reproducible, signed binaries (darwin/linux/windows × amd64/arm64) on every
`v*` tag, plus the user-facing verification flow in INSTALL.md.

**In scope:**
- A goreleaser config (`.goreleaser.yaml` or `.goreleaser.yml`) producing 6 platform/arch archives.
- A GitHub Actions release workflow triggered on `v*` tags that runs goreleaser and uploads to GitHub Releases.
- A reproducibility-enforcement step in the release workflow: a second build whose archive sha256s must match the first (modulo signature/checksum files), or the release fails.
- minisign signing of every archive (`.minisig` per artifact) and a project-wide `checksums.txt` (SHA-256).
- Public key file (`minisign.pub`) committed at the repo root.
- INSTALL.md restructured: download-pre-built-binary becomes the lead section, with a one-block copy-paste verification ceremony. Existing `go install` / build-from-source moves to a "Build from source" subsection.
- Documentation of the local dry-run path (e.g. a Make target) so contributors can reproduce a release without pushing a tag.

**Out of scope (other phases or future milestones):**
- Homebrew tap (PKG-02 → Phase 52).
- Scoop bucket (PKG-03 → Phase 52).
- Native Linux packages — apt/deb, rpm, AUR (PKG-04 → Phase 52).
- Docker / container images (PKG-DEFER-02, deferred).
- In-binary auto-update mechanism (explicitly out of scope project-wide per REQUIREMENTS.md).
- cosign / Sigstore signing (rejected in this phase — see D-01).

</domain>

<decisions>
## Implementation Decisions

### Signing
- **D-01:** Sign with **minisign** (Ed25519). Each archive gets a sibling `.minisig` file. The project owns one keypair: private key stored as a GitHub Actions secret, public key committed to the repo. Rationale: small dependency footprint (~200 KB binary), zero external trust (no Sigstore/Rekor), simple verification ceremony for users. Cosign was considered and rejected for this phase; revisit if PKG-DEFER-02 (containers) lands and Sigstore becomes valuable for image attestation.
- **D-02:** The minisign **public key file is at the repo root**: `minisign.pub`. INSTALL.md links to the canonical `https://raw.githubusercontent.com/<owner>/<repo>/main/minisign.pub` URL. Visible at first directory listing — signals "this project signs releases" immediately. Key rotation, if ever needed, edits this file in a normal commit.
- **D-02a:** The GitHub Actions secret holding the minisign **private key** is named consistently and documented in `CONTRIBUTING.md` under a "Releasing" or "Maintainer" subsection (planner picks the exact secret name; suggested: `MINISIGN_PRIVATE_KEY` + `MINISIGN_PASSWORD` if the key is password-protected). The secret is referenced from the release workflow only.

### Reproducibility
- **D-03:** Reproducibility is **CI-enforced**. The release workflow builds the same tag with goreleaser **twice** (e.g. `goreleaser release --snapshot --clean` then a second invocation), computes SHA-256 of every archive **excluding** `*.minisig`, `*.sig`, and `checksums.txt` (signatures are non-deterministic), and **fails the release** if any pair differs. Implementation shape (separate jobs vs single job with two steps; pre-publish gate vs post-publish audit) is planner discretion, but the planner MUST choose a flow where a non-reproducible build does NOT result in a published GitHub Release.
- **D-03a:** Goreleaser config uses the standard reproducibility flags: `-trimpath` in build flags, `mod=readonly`, deterministic archive timestamps (typically derived from the tag's commit timestamp via `SOURCE_DATE_EPOCH` or goreleaser's `mod_timestamp`). Researcher confirms the precise list against goreleaser's current docs.

### INSTALL.md UX
- **D-04:** Verification ceremony in INSTALL.md is a **single fenced bash block** users can copy-paste end-to-end. Format previewed during discussion:
  ```bash
  VERSION=v1.9.0
  OS=linux
  ARCH=amd64
  curl -LO https://github.com/<owner>/<repo>/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz
  curl -LO https://github.com/<owner>/<repo>/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz.minisig
  curl -LO https://github.com/<owner>/<repo>/releases/download/$VERSION/checksums.txt
  sha256sum -c --ignore-missing checksums.txt
  minisign -V -P RWQ... -m serena_${VERSION}_${OS}_${ARCH}.tar.gz
  tar -xzf serena_${VERSION}_${OS}_${ARCH}.tar.gz
  ```
  Block uses placeholders (`<owner>/<repo>`, real `RWQ...` pubkey) that the planner replaces with the actual repo URL once D-08 is resolved and the actual minisign public key once it's generated. macOS users get a parallel block with `shasum -a 256 -c` (the BSD coreutils name).
- **D-05:** INSTALL.md is **restructured**. A new top section — title TBD by planner, e.g. "Install (pre-built binary)" — sits **above** the current "Prerequisites" section and contains: download URL pattern, the verification block from D-04, and a note about supported OS/arch combos. The existing `go install` and `git clone` content moves to a new section titled "Build from source" (or similar). Rationale: most users should download a verified binary, not compile; this matches the v1.9 binary-first stance recorded in REQUIREMENTS.md "Out of Scope" (Docker rejected because "binary-first story is stronger for local dev agents").

### Release Trigger & Flow
- **D-06:** **Any `v*` git tag triggers an auto-published release.** Pre-release tags matching `v*-rc*`, `v*-beta*`, or `v*-alpha*` (planner can refine the exact pattern) auto-mark the GitHub Release as "Pre-release". Production tags (e.g. `v1.9.0`) publish as a regular release. No draft step; no `workflow_dispatch` gate. Matches ROADMAP success criterion 1 wording: "Tagging a release (e.g. `v1.9.0-rc1`) triggers a goreleaser CI workflow."
- **D-07:** **Release notes are auto-generated by goreleaser** from the git log between tags, using goreleaser's standard changelog grouping (feat/fix/docs/etc.). CHANGELOG.md remains hand-curated separately for human-readable narrative — it is **not** the source for goreleaser's notes. Rationale: removes a manual step per release; the curated CHANGELOG.md still exists for users who want a more polished history.

### Claude's Discretion
The planner has authority to decide the following without re-asking:
- The specific filename for the goreleaser config (`.goreleaser.yaml` vs `.goreleaser.yml`).
- The specific filename for the release workflow (e.g. `.github/workflows/release.yml`).
- Archive naming pattern beyond the convention shown in D-04 (goreleaser defaults are acceptable; the planner may tweak to match the D-04 INSTALL.md example exactly).
- Whether to include `LICENSE`, `README.md`, `INSTALL.md` inside each archive (goreleaser default of including LICENSE + README is fine; planner picks).
- The exact local dry-run command surface — likely a Makefile target mirroring the self-doc style from Phase 50 D-07 (e.g. `make release-snapshot`). Single fixed-path output mirrors Phase 50's user preference for minimal Make ceremony.
- The exact set of reproducibility flags applied to the goreleaser config (D-03a) — researcher confirms against current goreleaser docs.
- The fate of the legacy `.github/workflows/publish.yml` (Python `uv build` workflow). Likely deleted in this phase since `legacy/` is reference-only and the new release.yml supersedes it; planner verifies nothing else depends on it before deleting.
- The exact GitHub Actions secret name for the minisign private key (D-02a).
- Whether the reproducibility check runs as a separate job or two steps in one job (D-03), as long as a non-reproducible build does not publish a GitHub Release.
- Whether macOS verification gets a parallel fenced block in INSTALL.md (`shasum -a 256 -c`) or a one-line note next to the Linux block.

### Folded Todos
None — `gsd-sdk query todo.match-phase 51` returned no matches.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project decisions and constraints
- `.planning/PROJECT.md` — core value, current tech-debt list, "single Go binary" stance
- `.planning/REQUIREMENTS.md` §"Packaging & Distribution" — PKG-01 wording (this phase) and PKG-02/03/04 (Phase 52, out of scope here); §"Out of Scope" — the binary-first rationale that drives D-05
- `.planning/ROADMAP.md` §"Phase 51" — goal, 4 success criteria, dependency note on Phase 50
- `.planning/STATE.md` — current milestone position (Phase 50 complete; Phase 51 next)

### Prior phase context (carry-forward)
- `.planning/phases/50-toolchain-go1.25-bench-local/50-CONTEXT.md` — minimal-ceremony preference (D-07/D-08 there: single fixed-path Make targets), doc-only-over-CI-gating philosophy (relevant point of contrast: this phase intentionally inverts that preference for D-03 because release-artifact integrity is user-facing). Also: `.github/workflows/go-test.yml` is the green-CI workflow Phase 51 depends on.

### CI / workflows (to be added or modified)
- `.github/workflows/go-test.yml` — KEEP. The release workflow runs alongside it, not in place of it.
- `.github/workflows/publish.yml` — LIKELY DELETE (legacy Python `uv build` → PyPI). Planner verifies and removes if nothing depends on it.
- `.github/workflows/codeql.yml`, `codespell.yml`, `docker.yml`, `docs.yaml`, `junie.yml`, `pytest.yml` — KEEP, untouched by Phase 51.

### Code & files to create/modify
- `.goreleaser.yaml` (or `.goreleaser.yml`) — NEW. Goreleaser config with 6 platform/arch matrix, minisign signing, reproducibility flags.
- `.github/workflows/release.yml` (filename planner-discretion) — NEW. Tag-triggered release workflow with reproducibility-diff step.
- `minisign.pub` — NEW at repo root.
- `INSTALL.md` — MAJOR EDIT. Restructured per D-05 with new lead section per D-04.
- `Makefile` — ADD a release-dry-run target (e.g. `release-snapshot`) following the `bench` / `bench-baseline` self-doc style established in Phase 50 D-07.
- `CONTRIBUTING.md` — ADD a "Releasing" subsection covering: how to cut a release (push a `v*` tag), what the secrets are, how to do a local dry-run, key rotation procedure for minisign.

### Repo identity (research item — confirm before planning)
- `INSTALL.md` line 11 currently says `go install github.com/postfix/serena/cmd/serena@latest`. The working tree path is `github.com/agenthands/helix`. **The release pipeline must publish to whichever repo is the public canonical home.** The researcher confirms: (a) which GitHub repo hosts the public Releases (postfix/serena vs agenthands/helix vs another), (b) the current `go.mod` module path, and (c) whether INSTALL.md line 11 needs updating regardless. This is a research-phase question — the planner reads the answer in RESEARCH.md and uses the confirmed identity throughout `.goreleaser.yaml`, `release.yml`, and INSTALL.md.

### External / upstream context (research-phase reading)
- Goreleaser docs §"Reproducible Builds" — confirm the current canonical flag set for byte-identical output.
- Goreleaser docs §"Signing" with minisign — confirm the plugin/config name and how to wire the GH Actions secret.
- Goreleaser docs §"Changelog" — confirm the auto-generation knobs the project will use for D-07.
- minisign upstream README — confirm the verification command syntax shown in D-04 is current.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`Makefile` self-doc style** (Phase 50 D-07 established `bench` / `bench-baseline` targets with `## help` annotations). The new `release-snapshot` target follows this exact style for consistency: single-line help annotation, single fixed-path output if it produces a file, no `NAME=` parameter.
- **`.github/workflows/go-test.yml`** — established the project's CI shape on `ubuntu-latest` with Go 1.25.x and `actions/setup-go@v5`. The release workflow reuses the same `setup-go` step and Go version (1.25.x).
- **`cmd/serena/main.go`** — the single binary entrypoint goreleaser builds against. No subcommand split needed — `serena` is one binary with cobra subcommands.

### Established Patterns
- **Single-source-of-truth Makefile** — every contributor-facing command lives in the Makefile with a `## help` annotation. The release-snapshot target inherits this rule.
- **Tech-debt notes inline in PROJECT.md "Context" section** — Phase 50 D-10/D-11 established the edit pattern (remove specific sentences, leave paragraph intact). If Phase 51 adds release-pipeline considerations to PROJECT.md, follow the same pattern.
- **REQUIREMENTS.md Out-of-Scope row "Auto-update mechanism inside the binary"** — package managers own updates. This phase respects that boundary: PKG-01 ships the binary; users (or their package manager from PKG-02–04) handle updating.

### Integration Points
- **Phase 52 (PKG-02/03/04: Homebrew/Scoop/native Linux pkgs) depends on Phase 51.** Goreleaser's Homebrew and Scoop support reads from the same `.goreleaser.yaml` we create here. Plan the config with Phase 52 in mind: structure it so adding `brews:` and `scoops:` sections in Phase 52 is additive, not a rewrite.
- **`legacy/` Python tree** — `publish.yml` is part of the legacy world. Deleting it (planner-confirmed) does not affect anything in `legacy/` itself; that directory is read-only reference material.

</code_context>

<specifics>
## Specific Ideas

- The user picked **minisign over cosign** explicitly. The rationale matters for downstream agents: simpler verifier UX, smaller dependency footprint, no external transparency-log trust surface. If a future phase ever revisits supply-chain signing (e.g. for container images), cosign is the natural pairing then — but binary signing for v1.9 stays minisign.
- The user picked **CI-enforced reproducibility** explicitly, intentionally inverting Phase 50's "doc-only over heavy CI gating" preference. The rationale matters: release artifacts are a public integrity claim, and a doc-only reproducibility story is too easy to drift from. The planner should NOT propose downgrading this to a docs-only flow even though it would match the prior phase's stance — the user weighed the trade-off and chose the harder gate.
- The user picked **one-block copy-paste verification UX** explicitly over numbered steps. Optimization is for ceremony minimalism, not pedagogy. The block can include short prose comments inside the fence (`# verify checksum`, `# verify signature`) to keep meaning visible without breaking the copy-paste flow.
- The user picked **download-binary-first INSTALL.md** explicitly — pre-built becomes the dominant path; `go install` demoted. Aligns with v1.9 "binary-first" stance recorded in REQUIREMENTS.md.
- The user picked **tag-triggered auto-publish with auto-generated notes** explicitly. They accepted the trade-off that a typo'd tag publishes immediately; the reproducibility gate (D-03) is the safety net against shipping a broken artifact. The planner should NOT propose adding a draft step or workflow_dispatch gate.

</specifics>

<deferred>
## Deferred Ideas

- **cosign / Sigstore signing.** Rejected for binary signing in this phase. Natural revisit when PKG-DEFER-02 (container images) lands — cosign is the standard for OCI image attestation. Not in v1.9 scope.
- **Helper script `scripts/verify-release.sh`.** Rejected as INSTALL.md UX option (chicken-and-egg: getting the script means trusting an unsigned curl or cloning the repo). If it's added later as a developer convenience, INSTALL.md still leads with the copy-paste block per D-04.
- **`workflow_dispatch`-only release trigger.** Considered and rejected for D-06. Could be added later as an additional trigger (alongside tag-triggered) if a release ever needs to be re-cut without a new tag — but that's a future ergonomic addition, not part of PKG-01.
- **Draft-first release flow.** Considered and rejected for D-06. Could be added later if the project grows a release-PR review process — but with auto-generated notes, a typo'd tag is the only realistic mishap, and reproducibility gating (D-03) catches anything more serious.
- **Local-baseline-style "release archive" directory.** Phase 50 added a gitignored `test/bench/baselines/local.txt` for local artifacts. The release-snapshot Make target probably writes its output to a similarly gitignored path under `dist/` (goreleaser's default) — no new pattern needed, but worth a quick note in `.gitignore` if `dist/` isn't already covered.
- **In-binary auto-update mechanism.** Explicitly out of scope per REQUIREMENTS.md. Reaffirmed here.

### Reviewed Todos (not folded)
None — the cross_reference_todos step found no matching todos for Phase 51.

</deferred>

---

*Phase: 51-packaging-goreleaser*
*Context gathered: 2026-04-28*
