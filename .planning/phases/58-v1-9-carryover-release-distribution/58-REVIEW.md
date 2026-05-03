---
phase: 58-v1-9-carryover-release-distribution
reviewed: 2026-05-03T00:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - .github/workflows/release.yml
  - .goreleaser.yaml
  - CONTRIBUTING.md
  - INSTALL.md
  - internal/daemon/daemon.go
  - internal/forwarder/dial.go
  - internal/forwarder/forwarder.go
  - internal/obs/grpc.go
  - internal/upgrade/verify.go
  - internal/upgrade/upgrade.go
  - internal/upgrade/verify_test.go
  - internal/upgrade/upgrade_test.go
  - internal/upgrade/trustroot.go
  - internal/upgrade/trusted_root.json
  - internal/upgrade/testdata/generate_fixtures.go
  - internal/upgrade/testdata/trusted_root.json
  - internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json
  - internal/upgrade/testdata/sample-archive.tar.gz.rc.sigstore.json
  - internal/upgrade/testdata/sample-archive.tar.gz.wrong-issuer.sigstore.json
  - internal/upgrade/testdata/sample-archive.tar.gz.wrong-org.sigstore.json
  - Makefile
findings:
  critical: 2
  warning: 6
  info: 4
  total: 12
status: issues_found
---

# Phase 58: Code Review Report

**Reviewed:** 2026-05-03
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

## Summary

Phase 58 swaps the upgrade-verifier from minisign to cosign keyless via sigstore-go and rewires the goreleaser pipeline + GitHub Actions workflow accordingly. The crypto plumbing is well thought through (single canonical error literal, trust-root override hook for tests, asymmetric-checksum-pair refusal) and the test suite covers happy/tampered/wrong-identity/wrong-issuer paths. However:

- A "testable mirror" identity matcher (`matchesPinnedIdentity`) is asserted heavily by tests but is **not called by `VerifyArchive`** — the production path uses sigstore-go's `NewShortCertificateIdentity`. The mirror can drift silently, giving false security confidence.
- A real **resource leak** exists in the forwarder when daemon dial fails: the OTLP TracerProvider is constructed before the dial but its shutdown defer is registered after the dial succeeds.
- Documentation drifted from implementation: `CONTRIBUTING.md` describes a "Pass-1, Pass-2, Pass-3" reproducibility gate; `release.yml` only does two passes.
- Several `signs:` and workflow hardening gaps (id-token granted at workflow scope, missing `--oidc-issuer` pin in cosign args).
- Stage-dir cleanup is unreachable on the success path because `relaunchFn` replaces the process image before deferred cleanup runs.

## Critical Issues

### CR-01: `matchesPinnedIdentity` is dead in production but tested as if load-bearing

**File:** `internal/upgrade/verify.go:213-238` (definition); `internal/upgrade/verify_test.go:262-317` (the tests that depend on it)
**Issue:** `matchesPinnedIdentity(*x509.Certificate)` and `pinnedSANRegex` are defined as a "testable mirror" of the real identity policy (per the doc-comment "Task 3 refactor"), but `VerifyArchive` does **not** call them. Production goes exclusively through `pinnedCertificateIdentity()` → `verify.NewShortCertificateIdentity(...)` → sigstore-go's verifier. `TestMatchesPinnedIdentity_HappyPath` and `TestMatchesPinnedIdentity_RejectsWrongRepoSAN` therefore prove only that the *mirror* regex behaves; they do **not** prove that `VerifyArchive` rejects a "branch-not-tag" SAN, a wrong-workflow SAN, or a wrong-issuer when those bundles flow through `verify.Verify(...)`. If a future refactor narrows or breaks `pinnedCertificateIdentity`/`pinnedSANRegexLiteral` differently from `pinnedSANRegex`, the security policy can silently widen while every unit test stays green.

The two `*.wrong-org.sigstore.json` and `*.wrong-issuer.sigstore.json` fixture-driven tests do exercise the production path and are the actual security regression guard — but the four mirror sub-cases (`branch_not_tag`, `wrong_workflow`, etc.) have no fixture coverage and will not catch a real production regression.

**Fix:** Pick one:

1. **Preferred** — use the mirror in production so tests and code share the same code path. Replace the sigstore-go `NewShortCertificateIdentity` call with a custom `verify.PolicyOption` (or post-Verify cert inspection) that calls `matchesPinnedIdentity(certFromSignedEntity)` and returns the canonical error on mismatch. Then both the regex and the OIDC extension parser are exercised by every fixture test.
2. **Alternative** — generate fixtures for `branch_not_tag` and `wrong_workflow` SANs in `testdata/generate_fixtures.go` and add `TestVerifyArchiveBranchNotTag` / `TestVerifyArchiveWrongWorkflow` end-to-end tests that drive the production verifier. Keep the mirror but stop pretending it provides defense-in-depth coverage it does not.

Either way, add a comment block above `pinnedCertificateIdentity` and `matchesPinnedIdentity` calling out which one is the source of truth — the current "Centralized so VerifyArchive's policy and unit tests share the same source of truth" comment in `pinnedCertificateIdentity` is misleading because the unit tests *do not* go through that function.

### CR-02: `RunForwarder` leaks the OTel TracerProvider when daemon dial fails

**File:** `internal/forwarder/forwarder.go:27-37`
**Issue:** The TracerProvider is constructed (lines 28-32) **before** `ConnectOrStartDaemon` is called (line 34). The shutdown defer is only installed at line 41, **after** the dial-failure early-return at line 36. When `OTEL_EXPORTER_OTLP_ENDPOINT` is set and the dial fails (daemon down, socket missing, daemon crash during start), the SDK TracerProvider's batch span processor goroutine + OTLP gRPC connection are leaked for the lifetime of the forwarder process.

Single-shot `helix` CLI invocations may exit shortly after, masking this in normal use. But:
- `helix setup`-style flows call `RunForwarder` indirectly and may retry on dial failure → multiple leaks per CLI run.
- The forwarder is meant to be long-lived under stdio-managed agents; if the daemon is restarted out from under it and the forwarder retries via re-invocation, every failed retry leaks.

**Fix:** Move the shutdown defer to *immediately* after `fwdProvider` construction:

```go
fwdProvider := obs.WithTracing(...)
defer func() {
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = fwdProvider.ShutdownTracing(shutdownCtx)
}()

client, conn, err := ConnectOrStartDaemon(ctx, socketPath, logger, fwdProvider.TracerProvider())
if err != nil {
    return fmt.Errorf("connecting to daemon: %w", err)
}
defer conn.Close()
```

The existing comment "No-op on the Noop path" already documents that the unconditional shutdown is safe.

## Warnings

### WR-01: `id-token: write` granted at workflow scope, not job scope

**File:** `.github/workflows/release.yml:8-10`
**Issue:** `permissions: id-token: write` is set at the workflow level. Anyone adding a job to this workflow inherits id-token automatically — a least-privilege violation. The OIDC token is needed only by the `release` job (cosign sign step).
**Fix:** Move permissions to the job:

```yaml
permissions: {}     # workflow-level: deny everything by default

jobs:
  release:
    name: goreleaser
    permissions:
      contents: write
      id-token: write
    runs-on: ubuntu-22.04
    ...
```

This means a future reviewer who adds a "publish-docs" job alongside `release` does not unintentionally grant it the ability to mint Fulcio certs as the release identity.

### WR-02: Cosign sign-blob args do not pin `--oidc-issuer`

**File:** `.goreleaser.yaml:51-60`
**Issue:** The `signs:` block invokes `cosign sign-blob --bundle=... ${artifact} --yes`. There is no `--oidc-issuer https://token.actions.githubusercontent.com` flag. cosign defaults to auto-detecting the issuer from the GitHub Actions environment, which works in practice — but it means a misconfigured runner that exposes a different OIDC issuer environment variable (or future cosign defaults change) can mint a cert under a different issuer and the verifier will reject it at upgrade-time, breaking releases silently. Better to fail at sign-time with a clear "issuer mismatch" than at every user's `helix upgrade` call.
**Fix:** Pin the issuer explicitly:

```yaml
signs:
  - id: cosign
    cmd: cosign
    artifacts: all
    signature: "${artifact}.sigstore.json"
    args:
      - "sign-blob"
      - "--oidc-issuer=https://token.actions.githubusercontent.com"
      - "--bundle=${signature}"
      - "${artifact}"
      - "--yes"
```

### WR-03: Reproducibility gate does not include `checksums.txt`

**File:** `.github/workflows/release.yml:44-67`
**Issue:** Pass-1/Pass-2 sha256 capture only globs `*.tar.gz`. `checksums.txt` is also a goreleaser artifact and is signed in the real-release pass, but its byte-stability between snapshot passes is not verified. Any non-determinism in the order of entries (parallel build matrix → race in append order to checksums.txt) would not be caught by the gate.
**Fix:** Extend the find globs to `\( -name '*.tar.gz' -o -name 'checksums.txt' \)` in both pass-1 capture (line 48) and pass-2 capture (line 62). If goreleaser deterministically sorts checksums.txt entries (which it does, by filename), this should be a no-op cost. If it doesn't, this gate catches the bug before users do.

### WR-04: `CONTRIBUTING.md` documents a Pass-3 that does not exist

**File:** `CONTRIBUTING.md:163`
**Issue:** Reads: "the gate compares Pass-1, Pass-2, and Pass-3 snapshot hashes within the same source revision". `release.yml` runs only Pass-1 (snapshot pass 1), Pass-2 (snapshot pass 2), and the real-release (signed). There is no Pass-3 in the current workflow. Either the intent was three snapshot passes (and the workflow is missing one) or the doc copy-pasted from an earlier plan.
**Fix:** Update `CONTRIBUTING.md:161-163` to say "Pass-1 and Pass-2", or — if Pass-3 was intentional — add the third pass to `release.yml` after the diff gate to catch non-determinism that only triggers on a second build with the same dist directory absent.

### WR-05: Stage dir leaked on every successful upgrade

**File:** `internal/upgrade/upgrade.go:169-180, 297-300`
**Issue:** The `defer cleanup()` (line 169) runs only when control falls out of `Upgrade`. On the success path, `relaunchFn(exec, relaunchArgs, os.Environ())` (line 298) replaces the process image via `syscall.Exec` on Unix or calls `os.Exit(0)` on Windows — neither returns. Deferred functions registered after `relaunchFn` is invoked do not run. So on every successful upgrade, the staging directory under (typically) `~/.helix/upgrade-stage-*` persists indefinitely. After ten upgrades, ten copies of the previous archive + extracted binary live on disk.
**Fix:** Run cleanup explicitly *before* the swap step, after extraction has produced `newBin`:

```go
// After extraction succeeds, the stage dir's only useful artifact is newBin;
// copy it out and clean up before the irreversible swap.
finalNewBin := filepath.Join(filepath.Dir(stage), "helix.new")
if err := os.Rename(newBin, finalNewBin); err != nil {
    return serr.Wrap(serr.Internal, "moving extracted binary out of stage", err)
}
cleanup()                  // remove the stage tree
verifyKept = false         // belt and suspenders: signal the deferred path is a no-op
newBin = finalNewBin
```

Or, simpler: track which stage roots have been left behind (write a `~/.helix/stage-cleanup-todo` file) and have the next `helix upgrade` invocation prune them on entry. The current postmortem-retention behavior on verify failure is a separate path (gated by `verifyKept = true`) and stays as-is.

### WR-06: Forwarder writes to stdout without locking, ignores rand error

**File:** `internal/forwarder/forwarder.go:111-113, 125-129`
**Issue (a):** `os.Stdout.Write(msg.Payload); os.Stdout.Write([]byte("\n"))` is two unsynchronized syscalls. If the receive goroutine is interleaved with logging-to-stdout (the daemon process or any panic stack dump uses stderr, but slog text handler defaults to stderr → safe today), this is fine. But if anything in the same process ever logs to stdout, the message and its trailing newline can be split across log lines.
**Issue (b):** `_, _ = io.ReadFull(cryptoRand.Reader, b)` (line 127) discards the error. On a healthy POSIX system this never fails. But on misconfigured environments (chroot without `/dev/urandom`, FIPS-mode kernel quirks, tightly-jailed containers), `cryptoRand.Reader` can return an error and `b` stays all zeros — collapsing every session ID to `00000000000000000000000000000000`. The daemon trusts session IDs as session-isolation keys; collisions could merge sessions silently.
**Fix:**
- (a) Coalesce the two writes: `os.Stdout.Write(append(msg.Payload, '\n'))` or `fmt.Fprintln(os.Stdout, string(msg.Payload))`. The append is one allocation but is on the response hot path; the fmt path adds string conversion. Pick whichever fits the perf budget; either is one syscall.
- (b) Treat rand failure as fatal:
```go
if _, err := io.ReadFull(cryptoRand.Reader, b); err != nil {
    panic(fmt.Sprintf("crypto/rand failed: %v", err))   // unrecoverable
}
```
Or return an error from `generateSessionID` and propagate to `RunForwarder`.

## Info

### IN-01: `_ = err` no-op on chmod failure obscures intent

**File:** `internal/upgrade/upgrade.go:276-280`
**Issue:** The block reads:
```go
if err := os.Chmod(newBin, 0o755); err != nil {
    // Non-fatal: extraction may have set mode 0o755 already; some
    // filesystems disallow chmod. Continue.
    _ = err
}
```
The `_ = err` is dead — `err` falls out of scope at the `}` regardless. Drop the line; the comment alone is sufficient. Or `slog`-debug the error so a CI-time failure on a weird filesystem is at least observable in verbose mode.
**Fix:** Replace `_ = err` with `logger.Debug(...)` (requires threading logger through; not strictly worth it) or just delete the line.

### IN-02: `_ = errors.New` dead expression in trace test

**File:** `internal/upgrade/verify_test.go:232`
**Issue:** `_ = errors.New` at the end of `TestRekorUnreachable`. No-op; appears to be left over from an earlier draft that imported `errors` but didn't use it. The `errors` package is now used legitimately by `TestIsRekorUnreachable_ClassifiesNetworkErrors`, so the side-effect line is unnecessary.
**Fix:** Delete `_ = errors.New`.

### IN-03: `fmt.Fprintf` to `os.Stderr` for postmortem hint not threaded through `Stdout`

**File:** `internal/upgrade/upgrade.go:179`
**Issue:** Most user-facing messages from `Upgrade` go to `out` (configurable, defaulting to `os.Stdout`). The verify-fail postmortem hint goes directly to `os.Stderr`, bypassing the test-injected writer. Tests that capture upgrade output via `Options.Stdout` will not see this message and silently miss a regression in its wording. (The current asymmetric-checksum test pattern — accept either "release artifacts incomplete" or "not writable" — already shows this fragility.)
**Fix:** Either route through a configurable `Stderr` field on `Options`, or keep `os.Stderr` but add a parallel `out` line so `Stdout`-only test captures see the surface-level message.

### IN-04: SAN regex permits non-semver tags like `v1.2.3.4.5`

**File:** `internal/upgrade/verify.go:54`, `INSTALL.md:35`
**Issue:** `pinnedSANRegexLiteral` uses `v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$` — the `[\d.]+` accepts any digit-and-dot run, including ill-formed tags like `v1.2.3.4` or `v..1` or `v1...`. The exact tag shape is constrained at release time by goreleaser's `prerelease: auto` regex, but defense-in-depth pinning at the verifier should mirror that: `v\d+\.\d+\.\d+(-rc\d+|-beta\d+|-alpha\d+)?$`. This is a tightening, not a security bug today (Fulcio + GitHub Actions only mint certs for actual pushed tags), but a malicious pre-release fixture could exploit the loose regex if combined with another flaw.
**Fix:** Replace `v[\d.]+` with `v\d+\.\d+\.\d+` in both `verify.go:54` and `INSTALL.md:35`/`44`. Update the existing fixtures (which all use proper semver) and add a unit test asserting that `v1.2.3.4` is rejected by the matcher.

---

_Reviewed: 2026-05-03_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
