---
phase: 51
plan: 02
subsystem: packaging
tags: [packaging, docs, makefile, release-engineering]
requires:
  - 51-01 (release pipeline; provides .goreleaser.yml, release.yml, internal/cli vars)
provides:
  - INSTALL.md prebuilt-binary download + cosign verify sections (D-19)
  - RELEASING.md maintainer guide with snapshot+diff reproducibility ritual (D-07)
  - README.md → INSTALL.md pointer (D-20, no cosign duplication)
  - Makefile build target with git-derived version metadata (D-16)
  - Makefile release-snapshot / release-verify targets (Open Question 2 recommendation)
affects:
  - End-user install/verify UX
  - Maintainer release workflow
tech-stack:
  added: []
  patterns:
    - Verbatim D-03 cosign certificate-identity-regexp single-source-of-truth in INSTALL.md
    - Makefile -ldflags injection of internal/cli.{Version,Commit,Date} via git describe
key-files:
  created:
    - RELEASING.md
    - .planning/phases/51-packaging-goreleaser/51-02-SUMMARY.md
  modified:
    - INSTALL.md
    - README.md
    - Makefile
decisions:
  - D-07 (Claude discretion): separate RELEASING.md file at repo root, not a CONTRIBUTING.md section. Rationale: maintainer-only, larger than a sub-section, deserves its own URL for linking, and CONTRIBUTING.md remains contributor-focused.
metrics:
  duration: ~10 min
  tasks_completed: 3
  files_touched: 4
  completed: 2026-04-26
---

# Phase 51 Plan 02: Docs + Makefile Polish for Release Pipeline Summary

One-liner: Wire user-facing install/verify docs (INSTALL.md), maintainer release guide (RELEASING.md), and `make build` git-derived `--version` metadata around the Plan 51-01 goreleaser pipeline.

## What Shipped

1. **INSTALL.md** — added `## Download a prebuilt binary` and `## Verify a release binary` sections between Prerequisites and Quick Start. The verify section ships the canonical D-03 cosign command verbatim, including the exact `--certificate-identity-regexp '^https://github\.com/postfix/serena/\.github/workflows/release\.yml@refs/tags/v.*$'`.
2. **README.md** — single one-line pointer to `INSTALL.md#download-a-prebuilt-binary` immediately after the build-from-source block. No cosign command duplication (D-20 drift guard).
3. **RELEASING.md** — new maintainer guide covering toolchain prerequisites, pre-tag snapshot+diff reproducibility ritual (with all 6 RESEARCH Q6 exclusions verbatim), tag-and-push workflow for stable + pre-release tags, post-merge throwaway-tag verification (D-22), and out-of-scope reaffirmations (D-06 docker, D-23 Phase 52, D-27 SBOM).
4. **Makefile** — `build` target now injects `-ldflags` derived from `git describe --tags --always`, `git rev-parse HEAD`, and `git log -1 --format=%cI`, falling back to `internal/cli` defaults when git is unavailable. Two new targets `release-snapshot` and `release-verify` wrap the D-07 ritual; both guard against missing `goreleaser` on PATH with a helpful install hint.

### Smoke output

```
$ make build && ./serena --version
serena version v1.8-145-geb4904db (commit eb4904d, built 2026-04-26T12:02:43+03:00)
```

Format matches `serena version <X> (commit <Y>, built <Z>)`. Commit is real git data (not the `none` fallback), confirming git-describe injection works end-to-end.

## D-21 Success Criteria Closed

- D-21 #5 ✓ — INSTALL.md ships the download + verify sections with the verbatim D-03 regex.
- D-21 #7 ✓ — RELEASING.md documents the manual snapshot+diff reproducibility ritual; `make release-verify` makes it one-line invocable.
- D-21 #8 ✓ — existing CI (`go-test.yml`, `docker.yml`) untouched; `go vet ./...` exits 0; `go test ./...` passes for everything except a pre-existing Java integration flake unrelated to this plan (see Deferred Issues).

Phase 51 PR-ready signal: all 8 D-21 criteria green across 51-01 + 51-02.

## D-07 Discretion Decision

Chose `RELEASING.md` as a new file at repo root over adding a `## Releasing` section to `CONTRIBUTING.md`. Rationale:
- Release ritual is maintainer-only (different audience from contributor onboarding).
- ~120 lines of content — too large for a sub-section without dwarfing the rest of CONTRIBUTING.md.
- Standalone file gives a clean URL for cross-references (e.g., from issue templates, future internal docs).
- Convention: `RELEASING.md` is recognized at-a-glance by maintainers across the Go ecosystem.

## Deviations from Plan

None — plan executed exactly as written. The plan's "Step 1 — INSTALL.md" prose explicitly anchored the insertion site to the line ending `If \`serena\` is not found, ensure \`$(go env GOPATH)/bin\` is in your PATH.`; that's where the new sections landed.

## Authentication Gates

None.

## Deferred Issues

- **Pre-existing Java integration test flake** (`TestSymbols_JavaFixture/{get_hover_info,find_references_cross_file}`, `TestEdit_JavaFixture/replace_body`) reproduces on the unmodified base (`git stash` confirmed). This is a jdtls/Java workspace setup issue unrelated to docs/Makefile changes in this plan. Out of scope per Rule 4 (separate investigation).

## Out-of-Scope Reaffirmations

- Phase 52 packaging work (Homebrew, Scoop, Linux native packages) — D-23.
- No changes to `.github/workflows/docker.yml` — D-06.
- No SBOM generation — D-27.
- No macOS notarization, Windows code-signing, or in-binary auto-update.

## Self-Check: PASSED

- INSTALL.md sections present and ordered correctly (Prerequisites → Download → Verify → Quick Start).
- README.md contains exactly 1 line pointing at INSTALL.md anchor; no `certificate-identity-regexp` duplicated there.
- RELEASING.md exists with all 6 exclude flags, both stable + pre-release tag examples, INSTALL.md cross-link, and out-of-scope coverage of Phase 52 / docker.yml / SBOM.
- Makefile contains `git describe`, all three `internal/cli.{Version,Commit,Date}` ldflag bindings, and both new release targets in `.PHONY`.
- `make build && ./serena --version` produces canonical-format output with real git data.
- Commits exist: `96af40e7` (Task 1), `eb4904db` (Task 2), `31ef6ecb` (Task 3).
