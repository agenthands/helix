---
phase: 52-packaging-distribution-channels
fixed_at: 2026-04-29T00:00:00Z
review_path: .planning/phases/52-packaging-distribution-channels/52-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 12
skipped: 0
status: all_fixed
---

# Phase 52: Code Review Fix Report

**Fixed at:** 2026-04-29
**Source review:** `.planning/phases/52-packaging-distribution-channels/52-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 12 (4 Critical + 8 Warning; CR-01..CR-04 + WR-01..WR-08)
- Fixed: 12
- Skipped: 0

Required gates after all fixes:
- `go vet ./internal/upgrade/... ./internal/cli/...` — clean
- `go test -count=1 ./internal/upgrade/... ./internal/cli/...` — green

(`go vet ./...` and `go test ./...` over the full tree surface only
pre-existing, unrelated noise: a Swift tree-sitter `TOKEN_COUNT` macro
warning that predates this phase, and an untracked `tmp/graphify/tests/`
fixture in the working tree that is not part of the module.)

## Fixed Issues

### CR-01: GITHUB_TOKEN leaks across redirects (no CheckRedirect)

**Files modified:** `internal/upgrade/github.go`, `internal/upgrade/api_test.go`
**Commit:** `bef5cff4`
**Applied fix:** Added a `CheckRedirect` policy on the package-level `httpClient` that drops the `Authorization` header whenever the next-hop `req.URL.Host` differs from the originating host (and caps redirect chains at 10). Added `TestHttpClientStripsAuthOnCrossHostRedirect` regression test using two `httptest` servers — first server 302-redirects to second; with `GITHUB_TOKEN` set, the test asserts the first hop carried `Authorization: Bearer secret-pat-token` and the second hop saw an empty header.

### CR-02: Post-upgrade relaunch misinterprets user flags

**Files modified:** `internal/upgrade/upgrade.go`, `internal/upgrade/upgrade_test.go`
**Commit:** `6a234a6a`
**Applied fix:** Rewrote `stripUpgradeVerb` to (a) find the verb position, (b) preserve all pre-verb args, and (c) drop the verb plus the upgrade-only flag set: `--prerelease`, `--check`, `--dry-run`, `--version` (both detached `--version v` and attached `--version=v` forms — the detached form skips the next token). Updated `TestUpgradeStripUpgradeVerb` per the prompt direction: changed the broken-behavior case `{"helix", "upgrade", "--prerelease"}` → `{"helix", "--prerelease"}` to the correct expected `{"helix"}`, plus new cases for `--version v1.10.0`, `--version=v1.10.0`, combined flags, and a verbatim pass-through path for verb-less invocations like a bare `helix --version`. The flagged test case `{"helix", "--version"}` → `{"helix", "--version"}` is preserved only as the verb-less pass-through; the broken contract no longer ships.
**Note:** This is a logic change to production behavior, not just a test fix; the new contract should be reviewed by a human to confirm the relaunch semantics match the team's expectation (option 1 — strip flags but keep relaunching — was applied; option 2 — stop relaunching at all — was not).

### CR-03: Cross-check checksum skipped when only one of two assets is missing

**Files modified:** `internal/upgrade/upgrade.go`, `internal/upgrade/upgrade_test.go`, `CHANGELOG.md`
**Commit:** `1d59db3c`
**Applied fix:** Added an asymmetric-pair guard before the `checksumsAsset != nil && checksumsSigAsset != nil` cross-check branch: if exactly one of the two assets is present, print a `warning: release asset X present but Y missing — refusing to upgrade` line to stdout and return a typed `serr.NotFound` error. Either both files are present and verified, or neither is — no fallback to archive-signature-only. Added `TestUpgradeAsymmetricChecksumsRefused` that drives `Upgrade()` against an `httptest` server returning a release with `checksums.txt` but no `.minisig` and asserts the upgrade is refused with "release artifacts incomplete". CHANGELOG `Self-upgrade subcommand pair > Hard refusals` section documents the new contract.

### CR-04: tar.TypeRegA case is a forward-compatibility landmine

**Files modified:** `internal/upgrade/archive.go`
**Commit:** `b6f54b48`
**Applied fix:** Replaced `case tar.TypeReg, tar.TypeRegA:` with `case tar.TypeReg:`. Inline comment notes that `tar.Reader.Next()` already normalizes the legacy `'\x00'` typeflag to `TypeReg` so the alias was both deprecated and redundant. No behavior change; eliminates a future-Go-removal landmine.

### WR-01: Permission-probe TOCTOU window between probe and rename

**Files modified:** `internal/upgrade/upgrade.go`
**Commit:** `92725bfc`
**Applied fix:** Detect `errors.Is(err, fs.ErrPermission)` on the swap return; on permission errors, re-emit `SudoHint(exec, sudoHintArgs(opts))` to stdout so the user gets the same actionable `sudo helix upgrade <flags>` message they would have received at probe time. The `Internal`-wrapped error is still returned so exit-code semantics are unchanged. Imported `errors` and `io/fs`.

### WR-02: No size limit on archive download or tar extraction

**Files modified:** `internal/upgrade/github.go`, `internal/upgrade/archive.go`
**Commit:** `c91e77a7`
**Applied fix:** Added `maxDownloadBytes = 256 << 20` constant; wrapped `downloadFile`'s `io.Copy` in `io.LimitReader(resp.Body, maxDownloadBytes)`. In `extractTarGz`, replaced `io.Copy(out, tr)` with `io.Copy(out, io.LimitReader(tr, hdr.Size))` — now matches the existing comment that promised header-declared-size enforcement.

### WR-03: Stale env-var reference in user-facing docs (USAGE.md)

**Files modified:** `USAGE.md`
**Commit:** `5ef335a2` (combined with WR-04)
**Applied fix:** USAGE.md:858 `SERENA_TEST_JDTLS_DATA_DIR` → `HELIX_TEST_JDTLS_DATA_DIR`.

### WR-04: Stale `serena-test` cache path in CI workflow and Makefile

**Files modified:** `.github/workflows/go-test.yml`, `Makefile`
**Commit:** `5ef335a2` (combined with WR-03)
**Applied fix:** Confirmed source of truth at `test/integration/jdtlscache/cache.go:44` writes to `helix-test/jdtls`. Updated `.github/workflows/go-test.yml:88` cache path `serena-test/jdtls` → `helix-test/jdtls` and `Makefile:31-32` `clean-jdtls-cache` Darwin/Linux paths to match. CHANGELOG migration table (which documents the SERENA_*→HELIX_* rename) is intentionally left intact.

### WR-05: PLACEHOLDER minisign public key currently embedded in binary

**Files modified:** `internal/upgrade/pubkey.go`, `internal/upgrade/upgrade.go`, `internal/upgrade/upgrade_test.go`
**Commit:** `9fa44b23`
**Applied fix:** Added `IsPlaceholderPubKey()` exported function that calls `bytes.Contains(currentPubKey(), []byte("PLACEHOLDER"))` — uses `currentPubKey()` so the test override path is honored (only an actual placeholder reports true). Wired into `Upgrade()` as step 1b (right after daemon-detect; placed after daemon-detect rather than before because tests have a daemon-short-circuit path that does not install a test override). On placeholder detection, returns `serr.Unsupported` with a distinct message: `"this build was made before the maintainer rotated in the production minisign key — \`helix upgrade\` is unavailable; install from GitHub Releases (see INSTALL.md > Upgrading)"`. Added `TestUpgradePlaceholderPubKeyDistinguished` that installs a `PLACEHOLDER`-marked key via `testPubKeyOverride` and asserts the error message contains "production minisign key" and does NOT contain the canonical "signature verification FAILED" wording.

### WR-06: HTTP request lacks context-aware cancellation in body read

**Files modified:** `internal/upgrade/github.go`
**Commit:** `f09be0e2`
**Applied fix:** Added `watchContextClose(ctx, body)` helper that spawns a goroutine closing `body` on `ctx.Done()` and returns a `func()` stop signal. Wired into both `fetchJSON` (just after `defer Body.Close()` — covers `json.NewDecoder.Decode`) and `downloadFile` (covers the multi-MB `io.Copy`). The stop function is `defer`-ed to terminate the goroutine on the normal completion path so it does not leak.

### WR-07: Stale stage dir survives signature failure but is never garbage-collected

**Files modified:** `internal/upgrade/upgrade.go`, `internal/upgrade/stage.go`
**Commit:** `5eb68e0a`
**Applied fix:** Two-part:
1. In `Upgrade()`'s `verifyKept` defer branch, print the retained stage path on stderr: `"verify failed; staged artifacts retained for postmortem at <path> (a subsequent helix upgrade run will overwrite this directory)"`.
2. In `NewStageDir`, `os.Stat` the dir before the destructive wipe; if it exists, print a stderr warning that prior-attempt artifacts are being overwritten.

The architectural alternatives (timestamped dirs, refuse-to-retry on existing stage dir) would change the storage layout / contract beyond a localized fix and are not applied here. The two prints close the silent-wipe half of the worst-of-both-worlds failure mode.

### WR-08: `tar.TypeXHeader` and `tar.TypeXGlobalHeader` not handled

**Files modified:** `internal/upgrade/archive.go`
**Commit:** `8836da55`
**Applied fix:** Added explicit `case tar.TypeXHeader, tar.TypeXGlobalHeader:` clause with a comment noting that `tar.NewReader` consumes PAX bodies and applies metadata to the next header internally, so the explicit `continue` is documentation against a future maintainer who might add a reject branch. No behavior change.

---

_Fixed: 2026-04-29_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
