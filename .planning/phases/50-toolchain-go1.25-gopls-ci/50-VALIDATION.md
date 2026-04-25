---
phase: 50
slug: toolchain-go1-25-gopls-ci
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-25
---

# Phase 50 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + GitHub Actions `go-test.yml` (post-pivot: no bench gate on CI) |
| **Config file** | `.github/workflows/go-test.yml`, `CONTRIBUTING.md` (Benchmarks + gopls pin sections), `test/bench/baselines/README.md` |
| **Quick run command** | `go vet ./... && go build ./...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~120 seconds (local); `go-test.yml` on PR ~5 min on ubuntu-latest |

---

## Sampling Rate

- **After every task commit:** Run `go vet ./... && go build ./...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite green AND `go-test.yml` green on the verification PR (D-A13 bullet 1); bench gate intentionally retired (D-A12)
- **Max feedback latency:** 120 seconds local, ~5 min for `go-test.yml` on PR

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 50-01-01 | 01 | 1 | TOOL-02 | T-50-01 | Removes deleted CI bench gate | unit | `! test -e .github/workflows/bench.yml && ! test -e .github/workflows/capture-baseline.yml` | ✅ | ⬜ pending |
| 50-01-02 | 01 | 1 | TOOL-01 | — | Documents local-first baseline workflow | unit | `grep -F 'Capturing a Baseline (local pre-release)' test/bench/baselines/README.md && grep -F 'Pivot 2026-04-25' test/bench/baselines/README.md` | ✅ | ⬜ pending |
| 50-02-01 | 02 | 1 | TOOL-01 | — | Documents benchmarks + gopls pin | unit | `grep -c '^## Benchmarks$' CONTRIBUTING.md` (==1) `&& grep -c '^## gopls pin$' CONTRIBUTING.md` (==1) `&& grep -F 'v0.21.1' CONTRIBUTING.md && ! grep -q '^## Benchmark CI Gate$' CONTRIBUTING.md` | ✅ | ⬜ pending |
| 50-02-02 | 02 | 1 | TOOL-01 | — | Tech-debt sentence reflects pivot | unit | `grep -F 'Benchmark gate intentionally not enforced on CI; baselines are captured locally pre-release.' .planning/PROJECT.md && ! grep -qF 'Benchmark baselines captured locally (darwin/arm64)' .planning/PROJECT.md` | ✅ | ⬜ pending |
| 50-02-03 | 02 | 1 | TOOL-02 | T-50-06 | Cancels TOOL-02 with audit trail | unit | `grep -F '~~**TOOL-02**~~' .planning/REQUIREMENTS.md && grep -F 'Cancelled (Phase 50)' .planning/REQUIREMENTS.md && grep -E '^\| TOOL-01 \| Phase 50 \| Pending \|$' .planning/REQUIREMENTS.md` | ✅ | ⬜ pending |
| 50-02-05 | 02 | 1 | TOOL-01 | T-50-04 | Optional Makefile bench targets (D-A9) | unit | End-state A: `make -n bench-capture >/dev/null && make -n bench-compare OLD=a NEW=b >/dev/null`; End-state B: `git diff Makefile` empty (skip path) | ✅ | ⬜ pending |
| 50-02-06 | 02 | 1 | TOOL-02 | — | ROADMAP one-liner reflects pivot | unit | `grep -F 'retire the CI benchmark gate' .planning/ROADMAP.md && ! grep -qF 'restore the CI benchmark gate' .planning/ROADMAP.md` | ✅ | ⬜ pending |
| 50-02-07 | 02 | 1 | TOOL-01,TOOL-02 | — | Validation map reflects post-pivot tasks | unit | `grep -F 'go-test.yml is green' .planning/phases/50-toolchain-go1.25-gopls-ci/50-VALIDATION.md && grep -F '50-02-08' .planning/phases/50-toolchain-go1.25-gopls-ci/50-VALIDATION.md` (Pre-pivot strings such as `bench.yml`, `capture-baseline.yml`, `v1.9-github-hosted.txt`, and `bench-gate` are exempted from absence assertion against this file because they legitimately re-appear inside other rows' grep test commands; absence in the live tree is covered cross-doc by 50-02-08.) | ✅ | ⬜ pending |
| 50-02-08 | 02 | 1 | TOOL-01,TOOL-02 | — | Cross-doc consistency sweep | unit | `! grep -rn -F '.github/workflows/bench.yml' CONTRIBUTING.md .planning/PROJECT.md .planning/REQUIREMENTS.md test/bench/baselines/README.md && ! grep -rn -F '.github/workflows/capture-baseline.yml' CONTRIBUTING.md .planning/PROJECT.md .planning/REQUIREMENTS.md test/bench/baselines/README.md` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. Phase 50 is a docs + baseline-swap phase; no new test scaffolding needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| go-test.yml is green on the verification PR on ubuntu-latest with Go 1.25 (D-A13 bullet 1) | TOOL-01 | Requires real PR triggering GitHub Actions on ubuntu-latest | Open the verification PR after both plans land; confirm `go-test.yml` reports a green check on ubuntu-latest with the Go 1.25 toolchain. This is the only D-A13 bullet that cannot be asserted locally. |

---

## Validation Sign-Off

- [x] All tasks have automated verify or are flagged manual with reason
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (N/A — none required)
- [x] No watch-mode flags
- [x] Feedback latency < 120s local
- [x] `nyquist_compliant: true` set in frontmatter once plans land

**Approval:** approved 2026-04-25
