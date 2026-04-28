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
| **Framework** | go test + GitHub Actions workflows |
| **Config file** | `.github/workflows/bench.yml`, `.github/workflows/capture-baseline.yml` |
| **Quick run command** | `go vet ./... && go build ./...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~120 seconds (local); CI bench gate ~10 min |

---

## Sampling Rate

- **After every task commit:** Run `go vet ./... && go build ./...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite green AND bench-gate green on PR CI
- **Max feedback latency:** 120 seconds local, ~10 min for CI bench gate

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 50-01-01 | 01 | 1 | TOOL-01 | — | N/A (CI-only) | manual | `gh workflow run capture-baseline.yml -f milestone=v1.9` | ✅ | ⬜ pending |
| 50-02-01 | 02 | 2 | TOOL-02 | — | N/A | unit | `grep -c "v1.9-github-hosted.txt" .github/workflows/bench.yml` (==2) | ✅ | ⬜ pending |
| 50-02-02 | 02 | 2 | TOOL-01 | — | Documents gopls strategy | unit | `grep -q "gopls v0.21.1" CONTRIBUTING.md` | ✅ | ⬜ pending |
| 50-02-03 | 02 | 2 | TOOL-01 | — | Removes tech-debt note | unit | `! grep -q "benchmarks tech-debt" PROJECT.md` | ✅ | ⬜ pending |
| 50-02-04 | 02 | 2 | TOOL-01,TOOL-02 | — | CI green | manual | PR-CI run reports `bench-gate` and `test` jobs green | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. Phase 50 is a docs + baseline-swap phase; no new test scaffolding needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Capture baseline workflow produces `v1.9-github-hosted.txt` artifact | TOOL-02 | Requires GitHub Actions runner | Trigger `capture-baseline.yml` with `milestone=v1.9`; confirm artifact uploaded to `baselines/v1.9-github-hosted.txt` and committed |
| PR-CI bench gate fires when thresholds exceeded | TOOL-02 | Requires real PR with regression | After verification PR opens, confirm `bench-gate` job ran on `ubuntu-latest` with `--p50 0.15 --p95 0.25` |

---

## Validation Sign-Off

- [x] All tasks have automated verify or are flagged manual with reason
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (N/A — none required)
- [x] No watch-mode flags
- [x] Feedback latency < 120s local
- [x] `nyquist_compliant: true` set in frontmatter once plans land

**Approval:** approved 2026-04-25
