---
phase: 58
slug: v1-9-carryover-release-distribution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-03
---

# Phase 58 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Source: 58-RESEARCH.md §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `stretchr/testify` (already in `go.mod`) |
| **Config file** | none — `go test ./...` is the entry point |
| **Quick run command** | `go test ./internal/upgrade/... ./internal/forwarder/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30-90 seconds (quick) / ~5-10 min (full, includes integration) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/upgrade/... ./internal/forwarder/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1 && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite green + `make build` produces signed `.sigstore.json` outputs in dry-run snapshot
- **Max feedback latency:** 90 seconds (quick run)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 58-P1 | release-side cosign | 1 | REL-01 | T-58-01 (release-pipeline integrity) | goreleaser produces `.sigstore.json` bundles via Fulcio OIDC | integration | `goreleaser release --snapshot --skip=publish` | ✅ existing snapshot path | ⬜ pending |
| 58-P1 | release-side cosign | 1 | REL-01 | T-58-01 | release.yml has `id-token: write` permission | unit | `grep -q 'id-token: write' .github/workflows/release.yml` | ✅ existing | ⬜ pending |
| 58-P2 | verifier rewrite | 2 | REL-01 | T-58-02 (signature spoofing) | `VerifyArchive` accepts a valid sigstore bundle | unit | `go test ./internal/upgrade/ -run TestVerifyArchiveHappyPath -count=1` | ✅ rewrite (current minisign-targeted) | ⬜ pending |
| 58-P2 | verifier rewrite | 2 | REL-01 | T-58-02 | `VerifyArchive` rejects a tampered bundle | unit | `go test ./internal/upgrade/ -run TestVerifyArchiveTampered -count=1` | ✅ rewrite | ⬜ pending |
| 58-P2 | verifier rewrite | 2 | REL-01 | T-58-03 (identity confusion) | `VerifyArchive` rejects bundle with wrong cert identity SAN | unit (NEW) | `go test ./internal/upgrade/ -run TestVerifyArchiveWrongIdentity -count=1` | ❌ Wave 0 | ⬜ pending |
| 58-P2 | verifier rewrite | 2 | REL-01 | T-58-03 | `VerifyArchive` rejects bundle with wrong OIDC issuer | unit (NEW) | `go test ./internal/upgrade/ -run TestVerifyArchiveWrongIssuer -count=1` | ❌ Wave 0 | ⬜ pending |
| 58-P2 | verifier rewrite | 2 | REL-01 | T-58-04 (Rekor unreachable) | `helix upgrade` fetches `.sigstore.json` not `.minisig` | unit | `go test ./internal/upgrade/ -run TestUpgradeAssetNames -count=1` | ✅ adapt existing `archiveAssetName` parity test | ⬜ pending |
| 58-P3 | forwarder OTel | 1 | REL-06 | T-58-05 (trace gap) | `forwarder.tools.call` and `serena.v1.ForwarderService/StreamMCP` share a TraceID | integration | `go test ./test/integration/ -run TestE2ETraceContinuity -count=1` | ❌ Wave 0 | ⬜ pending |
| 58-P3 | forwarder OTel | 1 | REL-06 | T-58-05 | propagator option set on both client and server otelgrpc handlers | unit (compile-time) | `go vet ./internal/forwarder/... ./internal/daemon/...` | ✅ existing | ⬜ pending |
| 58-P4 | docs + won't-do | 1 | REL-05 | — | CONTRIBUTING.md contains literal phrase "Pass-3 limitation" | shell | `grep -q 'Pass-3 limitation' CONTRIBUTING.md` | ✅ existing file | ⬜ pending |
| 58-P4 | docs + won't-do | 1 | REL-02/03/04 | — | REQUIREMENTS.md REL-02/03/04 carry `- [~]` marker | shell | `grep -E '^- \[~\] \*\*REL-(02\|03\|04)' .planning/REQUIREMENTS.md \| wc -l` (expect 3) | ✅ existing file | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/upgrade/verify_test.go::TestVerifyArchiveWrongIdentity` — covers REL-01 identity-pinning regex
- [ ] `internal/upgrade/verify_test.go::TestVerifyArchiveWrongIssuer` — covers REL-01 OIDC issuer pinning
- [ ] `internal/upgrade/testdata/` — bundle fixture generation script (regenerate `sample-archive.tar.gz.sigstore.json` against a test trust root + ephemeral key, replacing `test_keypair.{pub,key}` and `sample-archive.tar.gz.minisig`)
- [ ] `test/integration/trace_continuity_test.go` — covers REL-06 single-trace assertion (in-memory exporter pattern from sigstore-go-verification example)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| First real signed v1.10.0 release artifacts published to GitHub Releases | REL-01 | Requires git tag push by maintainer; CI signs using GitHub Actions OIDC | (1) Tag `v1.10.0` on main, (2) push, (3) verify `.sigstore.json` artifacts appear alongside each archive in the release page, (4) `cosign verify-blob --bundle ...sigstore.json` against the published archive succeeds with the agenthands/helix identity SAN |
| `helix upgrade` against the published v1.10.0 verifies and atomically swaps | REL-01 | Needs a real prior-version binary running; downloads real artifact | (1) Build pre-release binary, (2) run `helix upgrade`, (3) confirm verifier accepts the bundle, (4) confirm binary is swapped, (5) confirm `helix --version` reports v1.10.0 |
| Rekor-unreachable error message is user-friendly | REL-01 D-04 | Requires network manipulation | (1) Block `rekor.sigstore.dev` via `/etc/hosts` or firewall rule, (2) run `helix upgrade`, (3) confirm error explicitly states "Rekor transparency-log verification requires network access" |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (4 wave-0 items above)
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s (quick) / 600s (full)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
