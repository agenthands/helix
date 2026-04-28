# Phase 51: packaging-goreleaser - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in 51-CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-28
**Phase:** 51-packaging-goreleaser
**Areas discussed:** Signing tool, Reproducibility enforcement, INSTALL.md verify UX, Release trigger & flow

---

## Signing tool

| Option | Description | Selected |
|--------|-------------|----------|
| cosign (keyless via OIDC) | Goreleaser signs each artifact with a short-lived cert from Sigstore via GH Actions OIDC. No long-lived key. Verifier needs cosign + trust in Sigstore's Rekor log. | |
| minisign (Ed25519 keypair) | One keypair, private key in GH Actions secret, public key shipped in repo. Tiny verifier binary, no external trust surface, simple ceremony. | ✓ |
| Both (cosign primary, minisign secondary) | Ship both signatures per artifact. Maximum verification flexibility but more INSTALL.md surface. | |

**User's choice:** minisign (Ed25519 keypair).
**Notes:** Decision driven by minimal-ceremony preference and minimal external-trust surface. cosign is deferred — natural pairing if future PKG-DEFER-02 (container images) lands.

### Sub-question — Public key location

| Option | Description | Selected |
|--------|-------------|----------|
| Repo root: `minisign.pub` | Visible at first directory listing; INSTALL.md links to canonical raw.githubusercontent.com URL. | ✓ |
| `.github/minisign.pub` | Hidden in `.github/`; less directory clutter. | |
| Embed in INSTALL.md as a fenced block | No separate file; pubkey + usage example co-located but rotation means editing prose. | |

**User's choice:** Repo root: `minisign.pub`.
**Notes:** Discoverability wins over directory cleanliness. Rotation = normal commit edit.

---

## Reproducibility enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| CI-enforced (diff job) | Release workflow runs goreleaser twice; sha256 of archives must match (excluding signatures/checksums); release fails on mismatch. Hard guarantee, ~3-5 min per release. | ✓ |
| Documented + locally verifiable | Goreleaser uses standard reproducibility flags; CONTRIBUTING.md teaches local reproduction. No CI gate. Matches Phase 50's doc-only philosophy. | |
| Snapshot smoke test only | On every push to main, run `goreleaser release --snapshot`. Cheap CI signal but no reproducibility diff. | |

**User's choice:** CI-enforced (diff job).
**Notes:** User intentionally inverted Phase 50's doc-only-over-CI-gating preference because release artifacts are a public integrity claim and a doc-only story is too easy to drift from. Implementation shape (separate jobs vs steps; pre-publish gate vs post-audit) left to planner, with the constraint that a non-reproducible build must NOT publish a GitHub Release.

---

## INSTALL.md verify UX

| Option | Description | Selected |
|--------|-------------|----------|
| One-block copy-paste | Single fenced bash block: download + checksum + signature in one paste. Lowest friction. | ✓ |
| Three numbered steps | Each step's purpose explicit (download / checksum / signature) with prose between. More pedagogical. | |
| Helper script in repo | `scripts/verify-release.sh` runs everything. One-command UX but chicken-and-egg trust problem for first-time downloaders. | |

**User's choice:** One-block copy-paste with the previewed format.
**Notes:** Optimization is for ceremony minimalism, not pedagogy. Block may include short inline `# verify checksum` / `# verify signature` comments to keep meaning visible without breaking copy-paste flow.

### Sub-question — INSTALL.md placement

| Option | Description | Selected |
|--------|-------------|----------|
| New first section, above `go install` | Pre-built binary becomes default install path; existing `go install` / build-from-source demoted to "Build from source". | ✓ |
| New peer section after `go install` | Keep `go install` as lead; download path is next section. Minimal churn. | |
| `## Verify a downloaded binary` subsection only | Don't add download narrative; assume user already downloaded. | |

**User's choice:** New first section, above `go install`.
**Notes:** Aligns with v1.9 binary-first stance from REQUIREMENTS.md "Out of Scope" rationale (Docker rejected because "binary-first story is stronger for local dev agents"). Pre-built download is the path most users should take.

---

## Release trigger & flow

| Option | Description | Selected |
|--------|-------------|----------|
| Tag-triggered, auto-published, generated notes | Push any `v*` tag → goreleaser builds, signs, publishes immediately. RC tags auto-mark Pre-release. Notes from git log. Zero-touch. | ✓ |
| Tag-triggered, draft-first, hand-edited notes | Tag → DRAFT release; maintainer reviews/edits notes, manually publishes. | |
| Workflow-dispatch + tag, generated notes | Production releases require explicit `workflow_dispatch`. Hardest to mis-release; doesn't match ROADMAP wording. | |

**User's choice:** Tag-triggered, auto-published, generated notes.
**Notes:** User accepted the typo-publishes-immediately trade-off; reproducibility gate (D-03) is the safety net against shipping broken artifacts. CHANGELOG.md remains hand-curated separately for human-readable narrative — not the source for goreleaser's auto notes.

---

## Claude's Discretion

The planner has authority on:
- Goreleaser config filename (`.goreleaser.yaml` vs `.goreleaser.yml`).
- Release workflow filename (e.g. `.github/workflows/release.yml`).
- Archive naming pattern beyond the convention shown in D-04.
- Whether to include `LICENSE` / `README.md` / `INSTALL.md` inside each archive (goreleaser defaults acceptable).
- Local dry-run command surface — likely a Makefile target mirroring Phase 50's self-doc style.
- Exact set of reproducibility flags applied to the goreleaser config.
- Fate of the legacy `.github/workflows/publish.yml` (likely deleted; planner verifies).
- Exact GitHub Actions secret name for the minisign private key.
- Whether the reproducibility check runs as a separate job or two steps in one job.
- Whether macOS verification gets a parallel fenced block in INSTALL.md or a one-line note.

## Research-Phase Items (open questions for the researcher)

These were flagged during discussion as needing research-phase confirmation, not user-decision items:

- **Repo identity discrepancy.** `INSTALL.md` line 11 says `go install github.com/postfix/serena/cmd/serena@latest`, but the working tree path is `github.com/agenthands/helix`. Researcher confirms which GitHub repo hosts the public Releases and whether INSTALL.md line 11 needs updating regardless.
- **Goreleaser reproducibility flags.** Confirm against current goreleaser docs the canonical flag set for byte-identical archives (-trimpath, mod=readonly, mod_timestamp / SOURCE_DATE_EPOCH).
- **Goreleaser minisign integration.** Confirm the plugin/config block name and how to wire the GH Actions secret.
- **Goreleaser changelog auto-generation knobs.** Confirm the section-grouping defaults the project will rely on.

## Deferred Ideas

- cosign / Sigstore signing — not in v1.9 scope; revisit with PKG-DEFER-02 containers.
- Helper script `scripts/verify-release.sh` — chicken-and-egg trust problem ruled it out as INSTALL.md UX option; could be added later as developer convenience without changing INSTALL.md.
- `workflow_dispatch`-only release trigger — rejected for D-06; could be added later as an additional trigger if needed.
- Draft-first release flow — rejected for D-06; could be added later if a release-PR review process emerges.
- Local-baseline-style "release archive" directory — covered by goreleaser's default `dist/`; just needs a `.gitignore` entry if not already covered.
- In-binary auto-update mechanism — explicitly out of scope project-wide (REQUIREMENTS.md).
