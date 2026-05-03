---
phase: 52
slug: packaging-distribution-channels
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-29
---

# Phase 52 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — go.mod governs |
| **Quick run command** | `go test ./internal/upgrade/... -count=1` |
| **Full suite command** | `go test ./... -count=1 && go vet ./...` |
| **Estimated runtime** | ~30s (quick) / ~120s (full) |

---

## Sampling Rate

- **After every task commit:** Run quick command on the touched package
- **After every plan wave:** Run full suite + `gofmt -l .`
- **Before `/gsd-verify-work`:** Full suite must be green AND `goreleaser --snapshot --clean` succeeds AND `grep -rn "github.com/postfix/serena\|cmd/serena\|SERENA_\|\\.serena/" --include="*.go" --include="*.md" --include="*.yaml" --include="Makefile" .` returns zero hits (only allowed in CHANGELOG.md history)
- **Max feedback latency:** 30 seconds for quick, 120 seconds for full

---

## Per-Task Verification Map

> Filled in by the planner during plan generation. Each task in each PLAN.md gets a row tying it to a REQ-ID, threat ref (if any), expected secure behavior, test type, and automated command.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | PKG-02..04 (deferral bookkeeping) / D-01..D-15 | TBD | TBD | TBD | TBD | TBD | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/upgrade/testdata/` — directory for test fixtures (test minisign keypair, sample release-archive bytes, fixture release JSON)
- [ ] `internal/upgrade/testdata/test_keypair.pub` + `test_keypair.key` — test keypair generated locally; public key checked in, private key checked in under testdata only (not used to sign anything outside tests)
- [ ] `internal/upgrade/testdata/release_latest.json` — captured fixture of `GET /repos/agenthands/helix/releases/latest` shape
- [ ] `internal/upgrade/testdata/sample-archive.tar.gz` + `.minisig` — minimal archive with a stub `helix` binary, signed with the test keypair
- [ ] `internal/upgrade/upgrade_test.go` — table-driven tests over the upgrade flow (download / verify / swap-dry-run)
- [ ] `internal/upgrade/upgrade_unix_test.go` (build-tag `!windows`) — POSIX rename / inode-replace path
- [ ] `internal/upgrade/upgrade_windows_test.go` (build-tag `windows`) — rename-to-.old + new-write path; CI matrix runs this on `windows-latest`
- [ ] `internal/upgrade/api_test.go` — `httptest.NewServer` GitHub Releases stub; tests version compare, asset URL extraction, prerelease filter, downgrade refusal
- [ ] `Makefile` target `embed-pubkey` — copies repo-root `minisign.pub` to `internal/upgrade/minisign.pub` before `go build` (D-13 build-time copy mechanism)
- [ ] `Makefile` target `verify-embed-pubkey` — diffs the two `minisign.pub` files; non-zero exit if drift; called by CI on every PR

*If Wave 0 is skipped, no per-task verification is possible — the upgrade subcommand is untestable.*

---

## Per-Dimension Coverage (Nyquist)

> Maps each Nyquist dimension to the phase's testable surface. Each row names at least one Wave 0 artifact + one runtime test that exercises it.

| # | Dimension | Coverage | Wave 0 Artifact | Runtime Test |
|---|-----------|----------|-----------------|--------------|
| 1 | Boundary | versions older / equal / newer than current; valid / corrupt / missing signatures; permission probe writable / unwritable; daemon-detected / not-daemon | `release_latest.json`, `sample-archive.tar.gz` | `TestVersionCompare_AcceptsNewerRefusesOlder`, `TestPermissionProbe_RefusesUnwritable`, `TestDaemonDetect_HonorsEnvVar` |
| 2 | Functional | `helix update` prints current+latest; `helix upgrade` end-to-end (in --dry-run mode); `--prerelease` filters correctly; `--version vX.Y.Z` pins; `--check` is alias of `update` | `release_latest.json`, GitHub stub | `TestUpdate_PrintsCurrentAndLatest`, `TestUpgradeDryRun_DownloadsAndVerifies`, `TestUpgrade_VersionPinOverridesPrerelease` |
| 3 | Adversarial | corrupt signature (flip one byte) → fail closed; serve valid sig but wrong pubkey → fail closed; redirect MITM (302 to attacker URL) → fail closed once signature check runs; truncated archive → fail closed; rate-limit-exhausted (429) → clear error message | `sample-archive.tar.gz` + tampered variant | `TestVerify_RejectsCorruptSignature`, `TestVerify_RejectsWrongPubkey`, `TestUpgrade_FailsClosedOnTruncated` |
| 4 | Performance | check round-trip < 2s against httptest stub; download path uses streaming I/O (no full-buffer); embed-pubkey Makefile step is O(stat) | n/a | `TestUpdate_LatencyUnder2s` (with httptest), bench-tagged stream copy |
| 5 | Concurrency | concurrent `helix upgrade` invocations from same user — second invocation either no-ops or refuses cleanly (advisory file lock recommended); upgrade-during-MCP-call: signal handling on the running daemon during in-binary swap | stage-dir + lock file | `TestUpgrade_RefusesIfLockHeld`, manual smoke for daemon |
| 6 | State | post-swap state on Unix: old inode is freed only when the running process exits; on Windows: `.old` exists until next launch (cleanup test) | n/a | `TestSwap_LeavesOldOnWindowsForNextRun` |
| 7 | Compatibility | renamed binary still compiles `CGO_ENABLED=0` for all 6 goreleaser targets (linux/darwin x amd64/arm64; windows/amd64; freebsd/amd64); reproducible-build flags from Phase 51 still pass; `goreleaser --snapshot --clean` succeeds with new archive name template | CI matrix | `make build`, `goreleaser --snapshot --clean`, `make release-snapshot` |
| 8 | Integration | `helix setup claude-code` on a freshly-renamed dev box detects no prior `serena` registration on macOS / removes it cleanly if present; `helix update` against the real GitHub repo (gated under build tag, run manually on release boundary); INSTALL.md verification recipe still works post-rename (curl-fetch flow still reads repo-root `minisign.pub`); CHANGELOG.md v1.9 entry calls out the breaking renames per project convention | `INSTALL.md`, `CHANGELOG.md`, `internal/cli/setup_clients_test.go` | `TestSetupClaudeCode_ClearsPriorSerenaRegistration`, manual recipe walkthrough on PR |

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real `helix upgrade` against GitHub | D-07 end-to-end | Hits real GitHub Releases API + real signature; running it in CI on every PR would burn the 60 req/hr unauth rate limit and require a real signed release present | After cutting v1.9.1 (or any subsequent release), on a clean dev box: install v1.9.0 manually via Phase 51 recipe → run `helix update` → `helix upgrade` → confirm `helix --version` reports new version |
| Cross-platform swap on a real Windows machine | D-07 Windows branch | `windows-latest` CI runner permits `os.Rename` of self in some cases due to its sandboxing; only a real Windows desktop reliably exercises the `.old`-rename path | Maintainer runs the manual upgrade on a Windows 11 machine; documents result in CHANGELOG v1.9 release notes |
| `helix upgrade` from inside a running daemon | D-08 daemon-aware branch | Daemon-detection requires a real long-running daemon process; can't simulate the parent-pid relationship reliably in tests | Start `helix daemon`; from another shell run `helix upgrade --dry-run`; confirm the manual-restart hint prints and the upgrade verb does not exec |
| MCP server registration name flip on a real Claude Code install | D-05 | Calling `claude mcp add-json` mutates the user's real Claude Code config; we don't shell out to it in tests | After upgrade, re-run `helix setup claude-code` on a dev box; confirm `claude mcp list` shows `helix` and not `serena` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (test keypair + httptest stub + Makefile embed-pubkey + Makefile verify-embed-pubkey)
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s for quick, < 120s for full
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
