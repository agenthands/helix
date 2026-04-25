---
phase: 51
slug: packaging-goreleaser
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-26
---

# Phase 51 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Phase 51 ships a release pipeline (CI workflow + goreleaser config + docs); validation is **tooling-driven** (`goreleaser check`, `goreleaser release --snapshot --clean`) plus the existing Go test/vet gates.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (existing) + `goreleaser` CLI (Wave 0 — must be on PATH locally) |
| **Config file** | `.goreleaser.yml` (validated by `goreleaser check`) |
| **Quick run command** | `goreleaser check` |
| **Full suite command** | `goreleaser release --snapshot --clean && go vet ./... && go test ./...` |
| **Estimated runtime** | ~30–90s for snapshot on M-series; <10s for vet+test core |

---

## Sampling Rate

- **After every task commit:** Run `goreleaser check` (when `.goreleaser.yml` changes) OR `go vet ./... && go test ./...` (when Go source changes)
- **After every plan wave:** Run `goreleaser release --snapshot --clean` and inspect `dist/` for the expected 6 archives + `checksums.txt`
- **Before `/gsd-verify-work`:** All 8 D-21 PR success criteria green; `go vet` and `go test` clean
- **Max feedback latency:** ~90 seconds (snapshot run is the slow leg)

---

## Per-Task Verification Map

> Filled in by the planner. Each plan task gets one row; rows mapped to PKG-01.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 51-01-XX | 01 | 1 | PKG-01 | — | (filled by planner) | tooling | (filled by planner) | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `goreleaser` CLI on PATH (latest stable v2.x at planning time; pinned in `release.yml`)
- [ ] `cosign` CLI on PATH for local snapshot diff comparisons (optional — sigs only land on real CI runs via OIDC)
- [ ] `cmd/serena/main.go` — Wave 0 declares `version`/`commit`/`date` package vars (or per RESEARCH.md, `internal/cli/version.go`-equivalent — planner picks the exact location)
- [ ] `Makefile` `build:` target updated to inject ldflags from `git describe --tags --always`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| End-to-end cosign verify of a real release artifact | PKG-01 | Requires actual GitHub Actions OIDC run; D-22 puts this out of PR scope (post-merge throwaway-tag ritual) | After merge, push throwaway tag (e.g. `v0.0.0-test1`), download an artifact, run the INSTALL.md `cosign verify-blob` command, then delete the test tag/release |
| Reproducibility (byte-identical archives) | PKG-01 (D-07) | D-08 explicitly forbids a CI repro gate; pre-tag ritual only | Run `goreleaser release --snapshot --clean -o dist1` then again to `dist2`; `diff -r --exclude='*.sig' --exclude='*.pem' --exclude='checksums.txt*' --exclude='artifacts.json' --exclude='metadata.json' --exclude='config.yaml' dist1 dist2` must be empty |
| `publish.yml` deletion confirmed in git | PKG-01 (D-04) | git plumbing | `! test -f .github/workflows/publish.yml && git log --diff-filter=D -- .github/workflows/publish.yml` shows the deletion commit |
| `docker.yml` byte-identical to main | PKG-01 (D-06) | git plumbing | `git diff main -- .github/workflows/docker.yml` empty |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (goreleaser CLI install, ldflag scaffold)
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter (after planner fills the task map)

**Approval:** pending
