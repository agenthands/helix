---
phase: 52-packaging-distribution-channels
reviewed: 2026-04-29T00:00:00Z
depth: standard
files_reviewed: 114
files_reviewed_list:
  - .github/workflows/go-test.yml
  - .github/workflows/release.yml
  - .gitignore
  - .goreleaser.yaml
  - CHANGELOG.md
  - CLAUDE.md
  - CONTRIBUTING.md
  - INSTALL.md
  - Makefile
  - README.md
  - USAGE.md
  - api/proto/serena/v1/ipc.pb.go
  - api/proto/serena/v1/ipc.proto
  - api/proto/serena/v1/ipc_grpc.pb.go
  - cmd/helix/main.go
  - go.mod
  - go.sum
  - internal/cli/activate.go
  - internal/cli/deactivate.go
  - internal/cli/deactivate_test.go
  - internal/cli/nudge.go
  - internal/cli/nudge_test.go
  - internal/cli/root.go
  - internal/cli/setup.go
  - internal/cli/setup_clients.go
  - internal/cli/setup_hooks.go
  - internal/cli/setup_hooks_test.go
  - internal/cli/setup_test.go
  - internal/cli/status.go
  - internal/cli/update.go
  - internal/cli/upgrade.go
  - internal/cli/upgrade_test.go
  - internal/config/config.go
  - internal/config/defaults.go
  - internal/config/loader.go
  - internal/daemon/daemon.go
  - internal/daemon/daemon_integration_test.go
  - internal/daemon/daemon_test.go
  - internal/daemon/telemetry_metrics_test.go
  - internal/errors/errors.go
  - internal/errors/kinds.go
  - internal/forwarder/forwarder_test.go
  - internal/kernel/edit/rename_override.go
  - internal/kernel/fileops/find.go
  - internal/kernel/fileops/write.go
  - internal/kernel/lspool/metrics.go
  - internal/kernel/lspool/quirks.go
  - internal/kernel/lspool/quirks_test.go
  - internal/langregistry/installer.go
  - internal/mcp/middleware.go
  - internal/mcp/server.go
  - internal/mcp/session.go
  - internal/memory/store.go
  - internal/obs/metrics.go
  - internal/obs/metrics_labels_test.go
  - internal/obs/metrics_test.go
  - internal/obs/obs.go
  - internal/obs/tracing_test.go
  - internal/profile/profile.go
  - internal/skill/memory/skill_test.go
  - internal/skill/repomap/skill.go
  - internal/skill/skill.go
  - internal/skill/workflow/skill.go
  - internal/skill/workflow/skill_test.go
  - internal/upgrade/api_test.go
  - internal/upgrade/archive.go
  - internal/upgrade/archive_test.go
  - internal/upgrade/daemon_detect.go
  - internal/upgrade/daemon_detect_test.go
  - internal/upgrade/github.go
  - internal/upgrade/minisign.pub
  - internal/upgrade/permission.go
  - internal/upgrade/permission_chmod_unix.go
  - internal/upgrade/permission_chmod_windows.go
  - internal/upgrade/permission_test.go
  - internal/upgrade/pubkey.go
  - internal/upgrade/semver.go
  - internal/upgrade/semver_test.go
  - internal/upgrade/stage.go
  - internal/upgrade/swap_assert.go
  - internal/upgrade/swap_unix.go
  - internal/upgrade/swap_unix_test.go
  - internal/upgrade/swap_windows.go
  - internal/upgrade/swap_windows_test.go
  - internal/upgrade/testdata/README.md
  - internal/upgrade/testdata/release_latest.json
  - internal/upgrade/upgrade.go
  - internal/upgrade/upgrade_test.go
  - internal/upgrade/verify.go
  - internal/upgrade/verify_test.go
  - llms-install.md
  - test/bench/bench_helpers_test.go
  - test/bench/fullrepo_smoke_test.go
  - test/bench/metrics_bench_test.go
  - test/bench/tracing_bench_test.go
  - test/harness/runner.go
  - test/integration/a_doc.go
  - test/integration/golden.go
  - test/integration/harness.go
  - test/integration/jdtlscache/cache.go
  - test/integration/jdtlscache/cache_test.go
  - test/integration/symbols_test.go
  - test/integration/trace_propagation_test.go
  - test/integration/trace_shutdown_test.go
  - test/oracle/judge/judge_test.go
  - test/oracle/llm/client.go
  - test/oracle/llm/selection_test.go
  - test/oracle/protocol/handshake_test.go
  - test/oracle/scenario/repomap_polyglot_test.go
  - test/oracle/scenario/sql_test.go
findings:
  blocker: 4
  warning: 8
  total: 12
status: issues_found
---

# Phase 52: Code Review Report

**Reviewed:** 2026-04-29
**Depth:** standard
**Files Reviewed:** 114
**Status:** issues_found

## Summary

Phase 52 ships a binary/module/env-var rename plus a brand-new self-upgrade
subsystem (`internal/upgrade/`). The mechanical rewrites are largely clean,
but I found four blockers concentrated in the new code:

1. **HTTP client follows redirects with the GITHUB_TOKEN bearer header
   intact.** GitHub returns 302 redirects for asset downloads; without a
   custom `CheckRedirect`, Go's default policy forwards the
   `Authorization: Bearer <PAT>` header to the redirected host. If a
   future redirect ever points off `*.github.com` (or an attacker
   subverts DNS), the user's PAT leaks. This is exactly the bug pattern
   that Go 1.17 partially mitigated (same-domain only) but the bearer
   header is still echoed across `*.githubusercontent.com` subdomains
   today.
2. **The relaunch step after a successful upgrade exits without doing
   useful work in the most common invocation modes.** `stripUpgradeVerb`
   only strips the verb, leaving the user's original flags. For
   `helix upgrade --version vX.Y.Z` on Unix, syscall.Exec re-runs the
   binary as `helix --version vX.Y.Z`, which is parsed as the root
   command's `--version` boolean flag plus a positional arg —
   `runRoot` prints "helix version dev" and exits. That is not what
   the user invoked; the user requested an upgrade and got their version
   printed.
3. **The `crossCheckSha256` defense-in-depth path is silently skipped
   when the release ships *only* `checksums.txt` without a `.minisig`
   sidecar (or vice-versa).** The current code requires BOTH assets to
   exist before invoking the cross-check; if one is missing the upgrade
   proceeds with archive-signature verification only. There is no log
   message and no test of the asymmetric case. An attacker who can
   replace `checksums.txt` on the release page (without producing a
   matching `.minisig`) effectively turns the cross-check off without
   the upgrade flow noticing.
4. **`tar.TypeRegA` was removed from Go's `archive/tar` package and is
   currently aliased.** The build works today on Go 1.25 because
   TypeRegA is `Deprecated: Use TypeReg instead.` but still defined.
   Future Go releases may remove it; the case clause is also
   semantically redundant (TypeRegA aliases TypeReg). Not a current
   crash, but a near-future build break.

The eight warnings cover incomplete migration cleanup (one
user-visible `SERENA_TEST_JDTLS_DATA_DIR` doc reference still in
USAGE.md), a permission-probe TOCTOU window between probe and rename,
absent download size limits (zip-bomb / disk-fill exposure), missing
context cancellation propagation in the HTTP path, the embedded
PLACEHOLDER public key shipping in the binary, and a few smaller items.

## Critical Issues

### CR-01: GITHUB_TOKEN leaks across redirects (no CheckRedirect)

**File:** `internal/upgrade/github.go:95-111`, `internal/upgrade/github.go:251-269`
**Issue:** `httpClient` is initialized with only a Timeout — no
`CheckRedirect` policy. GitHub Releases return a 302 that redirects
the asset download from `api.github.com` / `github.com` to
`objects.githubusercontent.com` (and historically other CDNs). Go's
default redirect handler does forward custom headers including
`Authorization` to redirects pointing at the same registered domain
or a sibling subdomain — and `*.githubusercontent.com` is treated as
a sibling of `github.com` for header forwarding under Go's matching
rules in some net/http versions. If GitHub ever switches asset hosting
to a different host (or DNS is poisoned), the bearer token leaks. The
download path in `downloadFile` (line 251-269) and `fetchJSON` (line
101) both attach `Authorization: Bearer <tok>` and rely on the default
client.
**Fix:** Strip the Authorization header on cross-host redirects:
```go
var httpClient = &http.Client{
    Timeout: 60 * time.Second,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        if len(via) >= 10 {
            return errors.New("too many redirects")
        }
        // Drop Authorization on cross-host redirects.
        if len(via) > 0 && req.URL.Host != via[0].URL.Host {
            req.Header.Del("Authorization")
        }
        return nil
    },
}
```
Add a unit test that returns a 302 to a different host and asserts the
token is not on the second hop.

### CR-02: Post-upgrade relaunch misinterprets user flags

**File:** `internal/upgrade/upgrade.go:250-253`, `internal/upgrade/upgrade.go:271-285`, `internal/cli/root.go:73,87-92`
**Issue:** After a successful swap, `Upgrade` runs:
```go
relaunchArgs := stripUpgradeVerb(os.Args)
if err := relaunch(exec, relaunchArgs, os.Environ()); err != nil { ... }
```
`stripUpgradeVerb` only removes the literal token `upgrade` (or
`update`). For `helix upgrade --version v1.10.0` the relaunch becomes
`helix --version v1.10.0`. The root command at `internal/cli/root.go:73`
defines `--version` as a Bool flag; cobra/pflag parses
`--version v1.10.0` as `--version=true` plus a positional arg
`v1.10.0`. `runRoot` then short-circuits at line 88-92 and prints
`helix version dev` (or whatever the new binary's ldflag injects),
discarding the positional argument. The user just upgraded and the
post-upgrade process did nothing useful. Likewise for `--prerelease`
(treated as an unknown flag at the root level — cobra returns an error
because root does not declare `--prerelease`) and `--dry-run`
(unknown flag at root, hard error). The unit test at `upgrade_test.go:256`
captures the same broken behavior as expected output (`{"helix", "--version"}`
→ `{"helix", "--version"}`), encoding the bug into the test suite.
**Fix:** Strip the entire upgrade subcommand args, not just the verb.
Two reasonable options:
1. Strip `os.Args` down to just `[]string{argv0}` after upgrade — the
   relaunched process behaves as if the user invoked `helix` with no
   flags. This matches the apparent intent in the comment at line
   247-249. Update the test in `upgrade_test.go` accordingly.
2. Stop relaunching at all on success. The user's original `helix
   upgrade ...` invocation completes; the new binary is on disk for
   the next invocation. This is what `cargo install --upgrade`,
   `gh extension upgrade`, and `rustup self update` do.

Either way, the current behavior surprises the user and ships a test
that affirms the bug.

### CR-03: Cross-check checksum skipped when only one of two assets is missing

**File:** `internal/upgrade/upgrade.go:180-212`
**Issue:** The defense-in-depth cross-check between `checksums.txt`
and the archive sha256 is gated on `if checksumsAsset != nil &&
checksumsSigAsset != nil`. If a release ships `checksums.txt` but the
release-signing pipeline failed to produce or upload
`checksums.txt.minisig` (or vice-versa), the entire cross-check is
silently skipped — there is no warning, no log line, no error.
Verification falls back to single-signature on the archive only,
which is exactly the surface the cross-check was added to defend. An
attacker who can substitute `checksums.txt` on a compromised release
asset can also delete the `.minisig` sidecar to disable the check
entirely. Goreleaser's signs stanza in `.goreleaser.yaml` is
configured `artifacts: all`, which currently signs both files, but
"both files exist" is not a verifiable invariant at upgrade time.
**Fix:** Either:
1. Make the cross-check unconditional when `checksumsAsset != nil`
   (i.e., refuse to upgrade if checksums.txt is present without a sig
   — that's a positive signal that signing failed):
   ```go
   if checksumsAsset != nil {
       if checksumsSigAsset == nil {
           return serr.New(serr.NotFound, "checksums.txt present but checksums.txt.minisig missing — release artifact is incomplete")
       }
       // ... verify and cross-check ...
   }
   ```
2. Log a structured warning when one is present without the other so
   operators can spot the asymmetric case in production.

Add a test that downloads checksums.txt without sig and asserts the
upgrade refuses (or warns).

### CR-04: tar.TypeRegA case is a forward-compatibility landmine

**File:** `internal/upgrade/archive.go:77`
**Issue:** `case tar.TypeReg, tar.TypeRegA:` — `archive/tar.TypeRegA`
is documented as `Deprecated: Use TypeReg instead.` since at least
Go 1.11. The deprecation is non-fatal today (TypeRegA still exists
as a constant equal to `'\x00'` in some Go versions; in others it is
intentionally dropped). A future Go toolchain bump (1.26+ has been
discussed) may remove the symbol entirely, breaking the build. The
case clause is also semantically redundant on modern `archive/tar`
because `tar.Reader.Next()` normalizes typeflag '\x00' (the legacy
"regular file" indicator) to TypeReg before returning the header.
**Fix:** Drop `tar.TypeRegA`:
```go
case tar.TypeReg:
```
There is no test that exercises a TypeRegA-flagged tar entry, so the
behavior change is invisible to the existing suite. If you actually
need to accept legacy `'\x00'` entries, add a regression test against
a hand-built fixture; otherwise just delete the alias.

## Warnings

### WR-01: Permission-probe TOCTOU window between probe and rename

**File:** `internal/upgrade/permission.go:20-30`, `internal/upgrade/upgrade.go:130-134,242-244`
**Issue:** `ProbeWritable` creates and immediately removes a temp file
in the install directory; `Upgrade` then runs five+ network/CPU steps
(download, sig verify, cross-check, extract, swap) before invoking
`os.Rename(newBin, currentPath)`. Between probe-time and swap-time
the directory's permission can change (e.g., a sysadmin runs `chmod
go-w /usr/local/bin` mid-upgrade, or the user's effective UID shifts
because they're inside `nix-shell --pure`). The error returned at
swap time (`"atomic swap"`-wrapped Internal) does not surface the
SudoHint message; the user just sees a wrap of `os.Rename`'s
permission-denied error and has to figure out what happened.
**Fix:** When the swap step fails with a permission error, re-run the
SudoHint formatting so the user gets the actionable message they got
at probe time. Detect via `errors.Is(err, fs.ErrPermission)`.

### WR-02: No size limit on archive download or tar extraction

**File:** `internal/upgrade/github.go:271-282`, `internal/upgrade/archive.go:81-89`
**Issue:** `downloadFile` does `io.Copy(out, resp.Body)` with no
LimitReader; the comment in `archive.go:85` says "Limit copy to
header-declared size to defend against zip-bombs" but the
implementation does not honor `hdr.Size` — it just `io.Copy`s the
entire entry into the destination. A malicious release archive whose
gzip ratio is 1000:1 can fill the user's disk before the integrity
check ever runs. Both vectors are mitigated in practice by minisign
verification (a tampered archive fails verify), but verify runs
*after* download and extract — the disk is already full by then.
**Fix:**
1. In `downloadFile`, cap to a reasonable ceiling (256 MB is well
   over Helix's archive size today and a fine guardrail):
   ```go
   if _, err := io.Copy(out, io.LimitReader(resp.Body, 256<<20)); err != nil { ... }
   ```
2. In `archive.go`, honor `hdr.Size`:
   ```go
   if _, err := io.Copy(out, io.LimitReader(tr, hdr.Size)); err != nil { ... }
   ```

### WR-03: Stale env-var reference in user-facing docs (USAGE.md)

**File:** `USAGE.md:858`
**Issue:** CHANGELOG.md v1.9 documents the rename
`SERENA_TEST_JDTLS_DATA_DIR` → `HELIX_TEST_JDTLS_DATA_DIR` (line 12).
The Go code (`internal/kernel/lspool/quirks.go:478`) reads
`HELIX_TEST_JDTLS_DATA_DIR`. But `USAGE.md:858` still tells users
to set `SERENA_TEST_JDTLS_DATA_DIR`. Anyone following USAGE.md will
silently no-op the override and waste time debugging "why isn't my
warm cache being used."
**Fix:** Replace `SERENA_TEST_JDTLS_DATA_DIR` with
`HELIX_TEST_JDTLS_DATA_DIR` at USAGE.md:858.

### WR-04: Stale `serena-test` cache path in CI workflow and Makefile

**File:** `.github/workflows/go-test.yml:88`, `Makefile:31-32`
**Issue:** The Java integration test caches workspace data at
`~/.cache/serena-test/jdtls` in CI (line 88) and the
`clean-jdtls-cache` Makefile target points at
`$HOME/Library/Caches/serena-test/jdtls` (mac) or
`$XDG_CACHE_HOME/serena-test/jdtls` (linux). USAGE.md:854 documents
the path as `helix-test/jdtls`, and CHANGELOG.md says all SERENA_*
artifacts moved to HELIX_*. Either:
- The test code actually writes to `serena-test/...` (in which case
  USAGE.md is wrong) and the rename was incomplete, OR
- The test code writes to `helix-test/...` (in which case the CI
  cache key path and Makefile target are wrong and the cache will
  miss every run because the directory doesn't exist).

The integration harness at `test/integration/jdtlscache/cache.go`
needs to be the source of truth — verify which path it actually uses
and align CI + Makefile + USAGE.md to match.

**Fix:** Read `test/integration/jdtlscache/cache.go` to identify the
canonical path, then update the two stale call sites to match.

### WR-05: PLACEHOLDER minisign public key currently embedded in binary

**File:** `internal/upgrade/minisign.pub:2`, `internal/upgrade/pubkey.go:23`
**Issue:** The embedded key is the placeholder `RWQAAAAAA...AAAAAA`
(all zeros base64). Any production binary built today will fail every
signature verification — `helix upgrade` is unusable. CHANGELOG.md
correctly calls this out as a Known Issue, and `release.yml` has a
pre-flight grep that fails CI on a tagged release if the placeholder
is still present (line 32-35), so the gate is in place. But two gaps:

1. The pre-flight check only fires on `push: tags: ['v*']`. A user who
   builds from `main` via `make build` produces a binary whose embedded
   key is the placeholder. `helix upgrade` from such a build will
   always fail the signature check with the canonical "signature
   verification FAILED" — but the user has no way to distinguish
   "tampered archive" from "you built from a placeholder-keyed source
   tree" because the error text is deliberately identical at every
   failure site (see verify.go:46-50).

2. The placeholder string `PLACEHOLDER` lives in `minisign.pub` but
   not in `internal/upgrade/minisign.pub` — except the Makefile copies
   it via `embed-pubkey`, so the embedded copy WILL contain
   `PLACEHOLDER` until the maintainer rotates. There is no runtime
   check that warns "you're running a binary built with the
   placeholder key — `helix upgrade` is non-functional, build from a
   tagged release."

**Fix:** Add a runtime startup check: if `pubKeyBytes` contains
the literal `PLACEHOLDER`, the upgrade subcommand should print a
distinct error message ("this binary was built before the maintainer
rotated in the production minisign key — `helix upgrade` is
unavailable; install from GitHub Releases") instead of the generic
canonical signature-failure message. This is a developer-experience
issue, not a security one.

### WR-06: HTTP request lacks context-aware cancellation in body read

**File:** `internal/upgrade/github.go:101-146`
**Issue:** `fetchJSON` uses `http.NewRequestWithContext` to attach
ctx (good), but the JSON decode at line 143 reads `resp.Body`
synchronously without honoring ctx. If the user Ctrl-Cs during a
multi-MB asset response (or the GitHub API hangs after sending
headers), `json.NewDecoder(resp.Body).Decode(out)` blocks
indefinitely. Same issue at line 275 in `downloadFile`.
**Fix:** Wrap the body read so ctx cancellation aborts the read:
```go
go func() {
    <-ctx.Done()
    _ = resp.Body.Close() // forces in-flight io.Copy / json.Decode to error out
}()
```
or use `httputil.NewControlledReader` patterns. The `Stop` channel /
goroutine should be torn down cleanly when the request completes
normally.

### WR-07: Stale stage dir survives signature failure but is never garbage-collected

**File:** `internal/upgrade/upgrade.go:160-165,205-218`, `internal/upgrade/stage.go:32-42`
**Issue:** On signature-verification failure, `verifyKept = true`
intentionally suppresses cleanup so a postmortem inspection is
possible (per VALIDATION.md State row, documented at upgrade.go:158).
Subsequent upgrade attempts call `NewStageDir` which does
`_ = os.RemoveAll(dir)` (stage.go:36), wiping the postmortem
artifact silently. So:
- If the user runs `helix upgrade` again, the postmortem evidence
  vanishes without warning.
- If the user never runs `helix upgrade` again, the stage dir
  (+ multi-MB tampered archive, + signature) sits forever in
  `/usr/local/bin/.helix-upgrade-stage/` consuming disk.

The "preserve for postmortem" decision is in tension with the
"clean stale stage on next run" decision. Neither tells the user
explicitly that the stage dir exists.
**Fix:** When `verifyKept = true`:
1. Print the path of the retained stage dir on stderr so the user
   knows where to look.
2. Either timestamp the dir (`.helix-upgrade-stage-<unix>`) so multiple
   failed attempts don't clobber each other, OR explicitly require
   the user to delete the previous attempt before retrying. Current
   "next run silently wipes" is the worst of both worlds.

### WR-08: `tar.TypeXHeader` and `tar.TypeXGlobalHeader` not handled

**File:** `internal/upgrade/archive.go:99-103`
**Issue:** The `default:` branch silently skips device files, fifos,
*and* PAX extended/global header records (`tar.TypeXHeader`,
`tar.TypeXGlobalHeader`). For Helix archives produced by goreleaser
this is fine — goreleaser does not emit PAX extended attributes — but
if a future archive includes a PAX header for a long path, the
following entry's metadata may depend on the PAX record. Skipping the
PAX header without consuming its body is what `tar.NewReader.Next()`
already handles internally, so the current code is technically
correct. The risk: if someone later moves to a different archive
producer, the silent-skip default is a footgun.
**Fix:** Be explicit: `case tar.TypeXHeader, tar.TypeXGlobalHeader:
continue` (with a comment noting tar.NewReader handles them
internally). This is documentation, not behavior.

---

_Reviewed: 2026-04-29_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
