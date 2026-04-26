---
phase: 52
slug: packaging-distribution-channels
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-26
---

# Phase 52 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + GitHub Actions matrix (release-verify.yml) |
| **Config file** | `.goreleaser.yml`, `.github/workflows/release-verify.yml` |
| **Quick run command** | `goreleaser check` |
| **Full suite command** | `goreleaser release --snapshot --clean` |
| **Estimated runtime** | ~120 seconds (snapshot build); release-verify.yml runs only on tag |

---

## Sampling Rate

- **After every task commit:** Run `goreleaser check`
- **After every plan wave:** Run `goreleaser release --snapshot --clean` (verifies brews/scoops/nfpms artifacts produced)
- **Before `/gsd-verify-work`:** Snapshot must produce expected artifacts; release-verify.yml workflow must lint clean (`actionlint`)
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 52-01-01 | 01 | 0 | PKG-02/03/04 | — | LICENSE SPDX read into goreleaser config | unit | `goreleaser check` | ✅ | ⬜ pending |
| 52-02-01 | 02 | 1 | PKG-02 | — | brews block produces tap PR on release | snapshot | `goreleaser release --snapshot --clean && ls dist/homebrew/` | ✅ | ⬜ pending |
| 52-03-01 | 03 | 1 | PKG-03 | — | scoops block produces bucket manifest | snapshot | `goreleaser release --snapshot --clean && ls dist/scoop/` | ✅ | ⬜ pending |
| 52-04-01 | 04 | 1 | PKG-04 | — | nfpms produces deb+rpm | snapshot | `goreleaser release --snapshot --clean && ls dist/*.deb dist/*.rpm` | ✅ | ⬜ pending |
| 52-05-01 | 05 | 2 | PKG-02/03/04 | — | release-verify.yml installs and asserts version | e2e | `actionlint .github/workflows/release-verify.yml` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Read `LICENSE` and capture SPDX identifier for substitution into brews/scoops/nfpms blocks
- [ ] Bootstrap `postfix/serena-packages` repo (Homebrew tap + Scoop bucket) with default branch
- [ ] Mint fine-grained PAT scoped to `postfix/serena-packages` with `contents:write`; store as `PACKAGES_PAT` repo secret
- [ ] Confirm `actionlint` available in CI for new workflow lint

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real `brew install <tap>/serena` on macOS arm64 | PKG-02 | Requires real macOS host + tap PR merged | After first tagged release: `brew tap postfix/serena-packages && brew install serena && serena --version` |
| Real `scoop install serena` on Windows amd64 | PKG-03 | Requires real Windows host + bucket auto-update merged | After first tagged release: `scoop bucket add postfix https://github.com/postfix/serena-packages && scoop install serena && serena --version` |
| Linux native package install via apt or rpm | PKG-04 | Requires real distro container (covered in release-verify.yml automated leg, but production install path validated manually first time) | After first release: `docker run --rm -v $PWD:/pkg debian:stable apt install /pkg/serena_*_amd64.deb && serena --version` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (LICENSE read, packages repo bootstrap, PAT secret)
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter after wave-0 complete

**Approval:** pending
