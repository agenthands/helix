---
phase: 51
slug: packaging-goreleaser
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-29
---

# Phase 51 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: 51-RESEARCH.md §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | bash assertions in CI (`release.yml`) + manual local dry-run via `make release-snapshot` |
| **Config file** | `.goreleaser.yaml` (config-as-test); `.github/workflows/release.yml` (CI gate) |
| **Quick run command** | `make release-snapshot` (verifies build matrix produces 6 archives) |
| **Full suite command** | `goreleaser check` (lints config) then `goreleaser release --snapshot --clean` |
| **Estimated runtime** | ~60–90 seconds local snapshot; ~3–5 min in CI including reproducibility gate |

---

## Sampling Rate

- **After every task commit:** `goreleaser check` (config lint, sub-second) when `.goreleaser.yaml` is touched; otherwise `go vet ./...` only.
- **After every plan wave:** `make release-snapshot` — confirms 6 archives + checksums.txt produced, no signing required.
- **Before `/gsd-verify-work`:** `goreleaser check` clean AND `make release-snapshot` clean AND a throwaway `v0.0.0-rc-test` tag pushed to a test branch fires `release.yml` and passes the reproducibility gate end-to-end (signing inclusive — needs CI secrets).
- **Max feedback latency:** 90 seconds for local snapshot; ~5 min for CI gate.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 51-01-* | 01 | 1 | PKG-01b/c/d | — | Reproducible 6-arch build | unit (build) | `make release-snapshot && test "$(ls dist/serena_*.tar.gz \| wc -l \| tr -d ' ')" = "6"` | ❌ W0 | ⬜ pending |
| 51-01-* | 01 | 1 | PKG-01a/d/e | T-secret-leak | Tag-triggered signed release | smoke | Push `v0.0.0-rc-test` tag → observe `release.yml` succeeds; `gh release view v0.0.0-rc-test --json assets` shows 6 archives + 6 `.minisig` + `checksums.txt` | ❌ W0 | ⬜ pending |
| 51-02-* | 02 | 2 | PKG-01f | — | Verification ceremony works end-to-end | manual UAT | Follow INSTALL.md verify block on linux+darwin against a real test release; `serena --help` exits 0 | ❌ W0 | ⬜ pending |
| 51-02-* | 02 | 2 | PKG-01b/c local | — | Local dry-run works | smoke | `make release-snapshot` exits 0 on a clean checkout | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Per-task IDs are placeholders pending PLAN.md generation; the planner fills exact `{N}-{plan}-{task}` IDs and binds each to its `requirements_addressed` row.*

---

## Wave 0 Requirements

- [ ] `.goreleaser.yaml` — covers PKG-01b/c/d/e (config-as-test)
- [ ] `.github/workflows/release.yml` — covers PKG-01a/d/e (CI sampling + reproducibility gate)
- [ ] `minisign.pub` — covers PKG-01e (verifier needs pubkey to validate)
- [ ] `INSTALL.md` D-04 verification block — covers PKG-01f
- [ ] `CONTRIBUTING.md` "Releasing" subsection — covers maintainer ergonomics (key rotation, secret setup, dry-run)
- [ ] `Makefile` `release-snapshot` target — covers PKG-01b/c local sampling
- [ ] `goreleaser` binary on contributor/CI machines — documented as prerequisite in CONTRIBUTING.md (`brew install goreleaser` on macOS; goreleaser-action on CI)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| INSTALL.md verify block runs end-to-end against a real published Release | PKG-01f | Verifies the user-facing UX, including network fetch from `releases/download/` URLs and minisign trust on a fresh shell | Cut a `v0.0.0-rc-test` Release on a fork or test branch; on a fresh linux + darwin shell, copy-paste the INSTALL.md block exactly; assert `serena --help` exits 0 and `minisign -V -P RWQ... -m ...` reports "Signature and comment signature verified" |
| Pre-release tag auto-marks Pre-release | PKG-01g | Requires a real GitHub Release page to observe the badge | Cut `v0.0.0-rc-test` → confirm Releases page shows "Pre-release" badge |
| Production tag does NOT mark Pre-release | PKG-01h | Same — observation against the GitHub Releases UI | Cut a synthetic `v0.0.0-test` (no `-rc/-beta/-alpha` suffix) → confirm no Pre-release badge |
| Minisign keypair generation and secret loading | D-02 / D-02a | One-time maintainer task with a private key that must never appear in CI logs or commits | Maintainer runs `minisign -G -p minisign.pub -s minisign.key`; commits `minisign.pub`; uploads `MINISIGN_PRIVATE_KEY` (file contents) and `MINISIGN_PASSWORD` to repo secrets via `gh secret set`; documented in CONTRIBUTING.md |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s for unit; < 5min for CI gate
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
