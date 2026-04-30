---
phase: 52-packaging-distribution-channels
verified: 2026-04-29T00:00:00Z
status: human_needed
score: 8/8 must-haves verified (1 minor doc-bookkeeping warning, 4 human-UAT items)
overrides_applied: 0
human_verification:
  - test: "Run `helix upgrade` end-to-end against a real published GitHub release tag (or staged release in a fork)"
    expected: "Daemon-detect skip on bare shell → permission probe → API fetch → semver compare → download → minisign verify against rotated production minisign.pub → extract → atomic swap → os.Exec relaunch reports new version on `helix --version`"
    why_human: "The full happy path can only be exercised against the live GitHub Releases API + a real signed archive; httptest stubs cover units (well-tested) but cannot validate the real network/IO ceremony"
  - test: "Confirm minisign.pub key rotation status before tagging v1.9.0"
    expected: "`head -1 minisign.pub` shows the production signing identity, NOT the literal `PLACEHOLDER`. Until rotation, `IsPlaceholderPubKey()` returns true and `helix upgrade` short-circuits with the 'this build was made before the maintainer rotated…' message — fail-closed but blocks the upgrade verb until rotation"
    why_human: "Key rotation is a maintainer/operator action with no automated trigger; the placeholder guard shipped in WR-05 documents the failure mode but does not perform the rotation"
  - test: "Manually validate CR-02 contract decision (option 1 vs option 2) for relaunch-after-upgrade flag stripping"
    expected: "Team reviews REVIEW-FIX.md note — current implementation strips upgrade-only flags but still relaunches with remaining args; alternative (don't relaunch at all) was not chosen. Confirm option 1 matches expected operator-experience semantics"
    why_human: "Team-policy / UX call recorded in REVIEW-FIX.md as needing human review; not a code-level bug"
  - test: "Visually inspect a snapshot release to confirm 6 archives are platform-correct and sign-correct"
    expected: "`ls dist/helix_v*.tar.gz` shows {darwin,linux,windows}×{amd64,arm64}; each has a sibling `.minisig`; `checksums.txt` exists; reproducibility CI gate (Phase 51) was last green"
    why_human: "Snapshot was already produced (6 archives present in dist/); a human spot-check before publishing confirms artifact correctness end-to-end"
warnings:
  - finding: "CONTRIBUTING.md:169 still references `serena_v<version>_<os>_<arch>.tar.gz` as the expected snapshot output of `make release-snapshot`. The actual output is `helix_v*` archives (verified in dist/). This is an active recipe, not historical attribution, so it contradicts SC-7 plan-06 truth #9. Suggested fix: replace with `helix_v<version>_<os>_<arch>.tar.gz`."
  - finding: "REQUIREMENTS.md keeps PKG-05/PKG-06/PKG-07 marked `[ ]` (incomplete) and the Traceability table column says `Pending` — even though the phase ROADMAP entry is closed and all three requirement payloads land in the codebase. Plan 06's frontmatter only claims PKG-02/03/04 (deferral bookkeeping); no plan flips PKG-05/06/07 from `[ ]` to `[x]` or from Pending → Complete. This is a documentation-bookkeeping gap, not an implementation gap. Suggested fix: flip all three to `[x]` and update the Traceability table column."
---

# Phase 52: packaging-distribution-channels Verification Report

**Phase Goal:** Users can install Helix as a single self-contained signed binary and upgrade it in place via `helix upgrade`. The binary, env vars, config dirs, and MCP server registration name all flip from `serena` to `helix` as a hard-cut breaking change at v1.9. An embed-audit manifest documents what ships inside the binary versus what the binary downloads at runtime.
**Verified:** 2026-04-29
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `cmd/helix/main.go` builds CGO=0 and produces a `helix` binary; `cmd/serena/` does not exist; module path is `github.com/agenthands/helix` | VERIFIED | `go.mod:1` is `module github.com/agenthands/helix`; `ls cmd/` shows `helix` (no `serena`); `CGO_ENABLED=0 go build ./cmd/helix` produced a Mach-O 64-bit arm64 binary; `grep -rn "github.com/postfix/serena" --include="*.go" .` returns zero hits |
| 2 | `make release-snapshot` produces 6 `helix_v*` archives in `dist/` with no leftover `serena_v*` archives | VERIFIED | `ls dist/helix_v*.tar.gz \| wc -l` = 6 (`{darwin,linux,windows}×{amd64,arm64}` for `v1.8-SNAPSHOT-1b4b6cce`); `ls dist/serena_v*.tar.gz` returns zero matches; `.goreleaser.yaml:11-12` `binary: helix` and `:36` `name_template: "helix_v{{ .Version }}_{{ .Os }}_{{ .Arch }}"` |
| 3 | `helix update` queries the GitHub Releases API and prints `current/latest/release-notes` without filesystem mutation | VERIFIED | `internal/cli/update.go:38-49` `runUpdate` calls `upgrade.Update`; `internal/upgrade/upgrade.go:73-99` `Update()` writes only to `Stdout`, makes one API call via `selectRelease`, and returns — no swap, no download, no extract; covered by `internal/upgrade/upgrade_test.go` Update-path tests |
| 4 | `helix upgrade` end-to-end: daemon-detect → permission probe → API fetch → semver compare → download → minisign verify → extract → atomic swap → `os.Exec` relaunch with same args minus `upgrade` | VERIFIED | `internal/upgrade/upgrade.go:111-307` `Upgrade()` implements the 10-step flow exactly per the plan: step 1 daemon-detect (`:118-121`), 1b placeholder-pubkey guard (`:130-133`), 2 permission probe (`:135-148`), 3 API fetch (`:150-154`), 4 semver compare with hard-refuse downgrade (`:156-164`), 5-6 stage+download+verify (`:166-262`), 7 extract (`:264-272`), 8 dry-run bail (`:274-278`), 9 atomic swap (`:280-296`), 10 relaunch via `relaunch()` which is `syscall.Exec` on Unix (`internal/upgrade/swap_unix.go:35-37`) and `cmd.Start + os.Exit(0)` on Windows (`internal/upgrade/swap_windows.go:42-56`). `stripUpgradeVerb` (`upgrade.go:339-385`) strips the verb + upgrade-only flags before relaunch (per CR-02 fix). |
| 5 | Tampered or wrong-key signatures rejected with single canonical error message; verification uses build-time-synced embedded `minisign.pub` | VERIFIED | `internal/upgrade/verify.go:59-76` `VerifyArchive` returns the literal `"signature verification FAILED"` at every failure branch (decode-pubkey, parse-sig, verify, missing); `internal/upgrade/pubkey.go:25-26` `//go:embed minisign.pub` embeds the build-time-synced key; `Makefile` `embed-pubkey` syncs repo-root → `internal/upgrade/minisign.pub` and `verify-embed-pubkey` is the CI gate (verified passing); placeholder-key short-circuit `IsPlaceholderPubKey()` (`pubkey.go:48-50`) provides distinct DX-error for pre-rotation builds. |
| 6 | `EMBED-AUDIT.md` exists and classifies every runtime asset; gap-flagged items closed in-phase | VERIFIED | `.planning/phases/52-packaging-distribution-channels/EMBED-AUDIT.md` (154 lines); `## Embedded` `## External-by-Design` `## Gaps Closed` sections all present; row count: 24 embedded entries + 14 external groups; explicit "0 gaps" claim at `EMBED-AUDIT.md:38`. |
| 7 | Every user-facing doc surface uses `helix`; CHANGELOG v1.9 has Breaking Changes subsection | MOSTLY VERIFIED — minor warning | `CHANGELOG.md` has `### Breaking Changes (v1.8 → v1.9)` enumerating binary/env-var/config-dir/MCP-name renames; INSTALL.md has 0 `serena_v*` refs; README.md has 0; USAGE.md has 0; CLAUDE.md has 0. **Warning:** `CONTRIBUTING.md:169` still says snapshot output is `serena_v<version>_<os>_<arch>.tar.gz` (active recipe, not historical-attribution context). Minor doc bug. |
| 8 | `go test ./...`, `go vet ./...`, `make verify-embed-pubkey`, `make release-snapshot` all green | VERIFIED | `go test -count=1 -short` over `./internal/... ./cmd/... ./api/... ./protocol/...` — green; `go vet` over the same scope — clean (only environmental Swift `TOKEN_COUNT` macro warning + tmp/graphify scratch noise, both deferred per `deferred-items.md`); `make verify-embed-pubkey` exit 0; `dist/` already populated with 6 helix_v* snapshot archives + checksums.txt. |

**Score:** 8/8 truths VERIFIED (1 with a minor doc warning that does not block goal achievement)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/helix/main.go` | renamed entrypoint with version variable wired for ldflag injection | VERIFIED | 23 lines; `var version = "dev"`; calls `cli.SetVersion(version)` and `mcp.SetVersion(version)` |
| `cmd/serena/main.go` | should NOT exist | VERIFIED | `ls cmd/` — no `serena` dir |
| `go.mod` | module path `github.com/agenthands/helix` | VERIFIED | line 1 |
| `.goreleaser.yaml` | produces helix_v* archives | VERIFIED | `binary: helix`, `name_template: helix_v{{ .Version }}_…` |
| `internal/upgrade/upgrade.go` | Upgrade(ctx, opts) + Update(ctx, opts) entrypoints (≥80 LOC) | VERIFIED | 460 LOC; both entrypoints exported and exercised by upgrade_test.go |
| `internal/upgrade/github.go` | GitHub Releases API client with redirect-strip CheckRedirect (CR-01 fix) | VERIFIED | 354 LOC; `httpClient` declares `CheckRedirect`; size-cap via `maxDownloadBytes` (WR-02) |
| `internal/upgrade/verify.go` | minisign verification with single canonical error message | VERIFIED | 76 LOC; literal `"signature verification FAILED"` repeated at every failure branch (per Pitfall 4) |
| `internal/upgrade/swap_unix.go` | Unix inode-replace + syscall.Exec | VERIFIED | 37 LOC |
| `internal/upgrade/swap_windows.go` | Windows rename-to-.old + cmd.Start+Exit | VERIFIED | 56 LOC |
| `internal/upgrade/swap_assert.go` | compile-time signature parity | VERIFIED | `var _ = swap; var _ = relaunch` |
| `internal/upgrade/daemon_detect.go` | RunningInDaemon reads HELIX_RUNNING_AS_DAEMON | VERIFIED | 26 LOC; `daemon/daemon.go:451` sets the env var on daemon startup |
| `internal/upgrade/pubkey.go` | //go:embed minisign.pub + IsPlaceholderPubKey | VERIFIED | 50 LOC; embed directive at line 25 |
| `internal/upgrade/minisign.pub` | build-time copy of repo-root minisign.pub | VERIFIED | exists; `make verify-embed-pubkey` byte-diff is clean |
| `internal/cli/update.go` | cobra subcommand `helix update` | VERIFIED | 49 LOC; SilenceUsage/SilenceErrors per pattern; --prerelease flag |
| `internal/cli/upgrade.go` | cobra subcommand `helix upgrade` | VERIFIED | 93 LOC; --prerelease/--version/--check/--dry-run flags with locked precedence |
| `internal/mcp/server.go` | MCP Implementation.Name = "helix" | VERIFIED | line 70: `&mcpsdk.Implementation{Name: "helix", Version: currentVersion}` |
| `internal/cli/setup_clients.go` | registers MCP server as "helix" | VERIFIED | lines 155, 174, 185, 191, 200 — all use `"helix"` literal |
| `internal/config/defaults.go` | config under ~/.helix/ | VERIFIED | line 17: `filepath.Join(homeDir, ".helix", "logs")` |
| `EMBED-AUDIT.md` | classification manifest | VERIFIED | 154 lines, three required sections present, zero open gaps |
| `Makefile` | embed-pubkey + verify-embed-pubkey targets, build depends on embed-pubkey | VERIFIED | targets present and self-doc'd; `build: embed-pubkey` prerequisite wired |
| `.github/workflows/release.yml` | verify-embed-pubkey CI gate | VERIFIED (per Plan 01 commit history) | gate hooked before publish |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `cmd/helix/main.go:14` | `cli.SetVersion` | ldflag-injected version threaded into cli pkg | WIRED — `cli.SetVersion(version); mcp.SetVersion(version)` lines 15-16 |
| `.goreleaser.yaml -X main.version` | `cmd/helix/main.go var version` | Go ldflag bind | WIRED — verified by building with `-ldflags="-X main.version=v1.9.0-test"` and observing `helix --version` reports `helix version v1.9.0-test` |
| `archiveNameTemplate` constant in upgrade.go | goreleaser `archives[].name_template` | shared template | WIRED — both render `helix_v<v>_<os>_<arch>.tar.gz`; parity asserted by upgrade_test.go (`archiveAssetName` test cases) |
| Upgrade() | RunningInDaemon | env-var short-circuit | WIRED — upgrade.go:118 calls `RunningInDaemon()`, daemon.go:451 sets the var |
| Upgrade() | ProbeWritable | permission probe BEFORE network I/O | WIRED — upgrade.go:144 (step 2) precedes selectRelease at :151 (step 3) |
| Upgrade() | VerifyArchive | minisign verify embedded pubkey | WIRED — upgrade.go:259, also :248 for checksums.txt cross-check |
| Upgrade() | swap | atomic swap | WIRED — upgrade.go:286 |
| Upgrade() | relaunch | os.Exec relaunch | WIRED — upgrade.go:303 with stripped args from stripUpgradeVerb |
| EMBED-AUDIT.md > Embedded | internal/upgrade/pubkey.go | minisign.pub embed | WIRED — explicit row in EMBED-AUDIT.md:46 |
| EMBED-AUDIT.md > External-by-Design | internal/langregistry/installer.go | LS three-tier installer | WIRED — explicit row citing the installer |
| CHANGELOG.md v1.9 Breaking Changes | INSTALL.md Upgrading | doc cross-reference | WIRED — verified manually |

### Data-Flow Trace (Level 4)

| Artifact | Data Source | Produces Real Data | Status |
|----------|-------------|--------------------|--------|
| `helix update` output | GitHub Releases API JSON via `fetchJSON` in github.go | YES — real API call (or httptest stub via `opts.baseURL`) | FLOWING |
| `helix upgrade` archive | GitHub Releases asset URL via `downloadFile` | YES — wired to real `BrowserDownloadURL` from API response | FLOWING |
| `VerifyArchive` pubkey input | embedded `pubKeyBytes` from `//go:embed minisign.pub` | YES — bytes flow into `minisign.DecodePublicKey(string(currentPubKey()))` at verify.go:60 | FLOWING |
| MCP server registration | `Name: "helix"` literal at server.go:70 | YES — flows into MCP SDK `Implementation` struct exposed on every initialize handshake | FLOWING |
| Default config dir | `filepath.Join(homeDir, ".helix", ...)` in defaults.go:17 | YES — used by koanf 4-layer config | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| CGO=0 build of cmd/helix produces Mach-O binary | `CGO_ENABLED=0 go build -o /tmp/helix ./cmd/helix` | exit 0; `file` reports `Mach-O 64-bit executable arm64` | PASS |
| `helix --version` reports ldflag-injected version | `go build -ldflags="-X main.version=v1.9.0-test" ./cmd/helix && helix --version` | `helix version v1.9.0-test` | PASS |
| `helix --help` lists update + upgrade subcommands | `helix --help \| grep -E "update\|upgrade"` | `update — Check for a newer Helix release without installing`; `upgrade — Download, verify, and install a newer Helix release` | PASS |
| upgrade tests pass with `-race` | `go test -count=1 -race ./internal/upgrade/...` | `ok github.com/agenthands/helix/internal/upgrade 1.677s` | PASS |
| in-scope vet clean | `go vet ./internal/... ./cmd/...` | no errors (Swift CGO macro warning is cosmetic / pre-existing) | PASS |
| in-scope test green | `go test -count=1 -short ./internal/... ./cmd/... ./api/... ./protocol/...` | all `ok` (no FAIL lines) | PASS |
| `make verify-embed-pubkey` byte-diff | `make verify-embed-pubkey` | exit 0 (no drift) | PASS |
| 6 helix_v* archives in dist/ | `ls dist/helix_v*.tar.gz \| wc -l` | `6` | PASS |
| zero serena_v* archives | `ls dist/serena_v*.tar.gz` | `no matches found` | PASS |
| zero `github.com/postfix/serena` imports remain | `grep -rn 'github.com/postfix/serena' --include='*.go' .` | (empty) | PASS |
| zero `SERENA_*` env-var refs in production Go | `grep -rn 'SERENA_' --include='*.go' --exclude-dir=legacy .` | (empty) | PASS |
| zero `.serena/` config-path refs in production Go | `grep -rn '\.serena/' --include='*.go' --exclude-dir=legacy .` | (empty) | PASS |
| MCP `Implementation.Name` = "helix" | `grep -n 'Name: \"helix\"' internal/mcp/server.go` | line 70 | PASS |
| MCP setup registers as "helix" | `grep -n '\"helix\"' internal/cli/setup_clients.go` | 5 hits across mcp add-json + mergeJSONConfig calls | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|------------|-------------|-------------|--------|----------|
| PKG-05 (rename binary/module/env/config/MCP) | Plans 02 + 03 (no frontmatter claim — bookkeeping gap) | binary/module/env/config-dir/MCP rename per D-01..D-05 | SATISFIED in code; BOOKKEEPING WARNING in REQUIREMENTS.md | go.mod, cmd/helix, defaults.go, server.go, setup_clients.go all flipped; REQUIREMENTS.md still marks it `[ ]` Pending |
| PKG-06 (in-binary self-upgrade) | Plan 04 (no frontmatter claim — bookkeeping gap) | `helix update` + `helix upgrade` per D-06..D-12 | SATISFIED in code; BOOKKEEPING WARNING in REQUIREMENTS.md | internal/upgrade/* + internal/cli/{update,upgrade}.go; REQUIREMENTS.md still marks it `[ ]` Pending |
| PKG-07 (EMBED-AUDIT + minisign embed) | Plan 05 (no frontmatter claim — bookkeeping gap) | manifest + //go:embed minisign.pub per D-13..D-15 | SATISFIED in code; BOOKKEEPING WARNING in REQUIREMENTS.md | EMBED-AUDIT.md + internal/upgrade/pubkey.go (//go:embed minisign.pub at :25); REQUIREMENTS.md still marks it `[ ]` Pending |
| PKG-02 (Homebrew tap) | Plan 06 (claimed in frontmatter) | DEFERRED to PKG-DEFER-03 | SATISFIED (deferral bookkeeping) | REQUIREMENTS.md:25 marks `[x]` deferred; line 86 `Deferred` in Traceability; line 52 PKG-DEFER-03 entry exists |
| PKG-03 (Scoop bucket) | Plan 06 (claimed in frontmatter) | DEFERRED to PKG-DEFER-04 | SATISFIED (deferral bookkeeping) | REQUIREMENTS.md:26 marks `[x]` deferred; line 87 `Deferred` in Traceability; line 53 PKG-DEFER-04 entry exists |
| PKG-04 (native Linux package) | Plan 06 (claimed in frontmatter) | DEFERRED to PKG-DEFER-05 | SATISFIED (deferral bookkeeping) | REQUIREMENTS.md:27 marks `[x]` deferred; line 88 `Deferred` in Traceability; line 54 PKG-DEFER-05 entry exists |

**Deferral Bookkeeping Check:** PASS — PKG-02/03/04 are explicitly mapped to PKG-DEFER-03/04/05 in REQUIREMENTS.md with rationale citing the Phase 52 rescope; the Traceability table column is consistent; the v1.9 requirement count footer (`104:*Last updated: 2026-04-30 — Phase 52 rescope: PKG-02/03/04 deferred to PKG-DEFER-03/04/05; PKG-05/06/07 added in-scope`) accurately reflects the deferral.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `CONTRIBUTING.md` | 169 | `serena_v<version>_<os>_<arch>.tar.gz` referenced as snapshot output (active recipe, not historical) | Warning | Users following the local-build recipe will look for `serena_v*` files that no longer exist; they should look for `helix_v*`. Single doc edit fixes it. |
| `.planning/REQUIREMENTS.md` | 28-30, 89-91 | PKG-05/06/07 still marked `[ ]` Pending despite phase being complete | Warning | Bookkeeping inconsistency only — does not affect runtime. Five-line edit (`[ ]` → `[x]` on three lines, `Pending` → `Complete` on three table rows) fixes it. |
| upgrade.go, verify.go | various | TODO/FIXME/PLACEHOLDER scan | Info | Only `placeholderMarker = "PLACEHOLDER"` constant in `pubkey.go:35` — that is the documented WR-05 sentinel for the pre-rotation key, not a stub. |

### Human Verification Required

See `human_verification` block in frontmatter for the four manual UAT items. Highest priority items:

1. **End-to-end `helix upgrade` against a real GitHub release** — httptest stubs are well-covered, but the live ceremony (real network, real archive, real signature) needs a manual run before declaring v1.9 ready to ship. Easy to gate via a fork release.
2. **Minisign key rotation** — the embedded `minisign.pub` still carries the `PLACEHOLDER` marker; until the maintainer rotates to a real production keypair, `helix upgrade` short-circuits with the WR-05 distinct error. Fail-closed but operationally blocks the upgrade verb.

### Gaps Summary

**No blocking gaps.** All eight Roadmap success criteria are met by the codebase; `helix` builds CGO=0, dist/ holds 6 `helix_v*` archives, the upgrade subcommand pair implements the full flow with correct ordering and a single canonical signature-failure message, the EMBED-AUDIT manifest is complete with zero open gaps, and the deferral bookkeeping for PKG-02/03/04 → PKG-DEFER-03/04/05 is in place.

**Two low-severity warnings** that are pure documentation polish and do not block phase closure:

1. `CONTRIBUTING.md:169` still describes snapshot output using `serena_v*` instead of `helix_v*` (active recipe in user-facing doc — should be flipped per Plan 06 truth #9).
2. `REQUIREMENTS.md` keeps PKG-05/06/07 marked `[ ]` Pending in both the bullet list and the Traceability table; phase is complete in ROADMAP.md but no plan flipped these three IDs to `[x]`/`Complete` (Plan 06's frontmatter only claimed the deferral PKG-02/03/04 trio).

**Four human-UAT items** routed to the maintainer:

1. End-to-end `helix upgrade` dry-run against a real GitHub release (network ceremony only checkable manually).
2. Confirm minisign key rotation status before tagging v1.9.0.
3. Validate CR-02 contract decision (option 1 — strip flags but relaunch — was applied; option 2 was not).
4. Manual spot-check of dist/ snapshot archive correctness.

**Phase status:** human_needed — implementation is complete and goal is achieved; the two doc warnings are non-blocking polish, and the four UAT items are operator-domain checks that the agent literally cannot perform.

---

_Verified: 2026-04-29_
_Verifier: Claude (gsd-verifier)_
