---
phase: 58-v1-9-carryover-release-distribution
fixed_at: 2026-05-03T00:00:00Z
review_path: .planning/phases/58-v1-9-carryover-release-distribution/58-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 12
skipped: 0
status: all_fixed
---

# Phase 58: Code Review Fix Report

**Fixed at:** 2026-05-03
**Source review:** `.planning/phases/58-v1-9-carryover-release-distribution/58-REVIEW.md`
**Iteration:** 1

**Summary:**

- Findings in scope: 12 (2 Critical + 6 Warning + 4 Info, `--all` mode)
- Fixed: 12
- Skipped: 0
- Status: `all_fixed`

## Summary table

| Finding | Severity | Status | Commit  | Notes |
|---------|----------|--------|---------|-------|
| CR-01   | Critical | fixed  | d37d02ae | Deletion path chosen (per orchestrator directive). Removed `matchesPinnedIdentity`, `pinnedSANRegex`, `fulcioOIDCIssuerOID` and the four `TestMatchesPinnedIdentity_*` cases + `certWithSANAndIssuer` helper. Existing wrong-org / wrong-issuer fixture-driven tests are the production-path security regression guard; updated `pinnedCertificateIdentity` doc-comment to reflect that fixture coverage is what prevents drift. |
| CR-02   | Critical | fixed  | 91d3c261 | Moved `ShutdownTracing` defer to immediately after `obs.WithTracing(...)` returns the provider, before the `ConnectOrStartDaemon` call. Dial-failure no longer leaks SDK goroutines + OTLP gRPC conn. |
| WR-01   | Warning  | fixed  | 38cbebc2 | Set workflow-level `permissions: {}`; moved `contents: write` + `id-token: write` under the `release` job. |
| WR-02   | Warning  | fixed  | b2b0430b | Added `--oidc-issuer=https://token.actions.githubusercontent.com` to the `signs:` block in `.goreleaser.yaml`. |
| WR-03   | Warning  | fixed  | cefbf767 | Extended both repro-gate find-globs to `\( -name '*.tar.gz' -o -name 'checksums.txt' \)`. |
| WR-04   | Warning  | fixed  | 9b9933cf | Replaced "Pass-1, Pass-2, and Pass-3" wording with "Pass-1 and Pass-2"; dropped the "Pass-3 limitation" framing. |
| WR-05   | Warning  | fixed  | fefc3849 | Added explicit `cleanup()` between `swapFn` success and `relaunchFn` invocation, plus a `stageCleaned` flag so the deferred path becomes a no-op once the explicit cleanup has run. Verify-fail postmortem retention (WR-07) is unaffected. |
| WR-06   | Warning  | fixed  | d99520c6 | (a) Coalesced `os.Stdout.Write(payload)` + newline into `os.Stdout.Write(append(payload, '\n'))`. (b) `generateSessionID` now panics on `crypto/rand` failure instead of returning an all-zero ID. |
| IN-01   | Info     | fixed  | 1e140ef2 | Replaced the dead `_ = err` block with a single `_ = os.Chmod(...)` line; comment retained. |
| IN-02   | Info     | fixed  | d37d02ae | Bundled with CR-01 since both touched the same test file; deleted `_ = errors.New` from `TestRekorUnreachable`. |
| IN-03   | Info     | fixed  | 0395ccb5 | Mirror the verify-failed postmortem hint through `opts.Stdout` in addition to `os.Stderr` so test captures via `Options.Stdout` see the wording. |
| IN-04   | Info     | fixed  | d37d02ae | Bundled with CR-01 since both touched `verify.go`; tightened `pinnedSANRegexLiteral` version segment from `[\d.]+` to `\d+\.\d+\.\d+` and updated `INSTALL.md` cosign verify-blob recipes in lockstep. |

## Files touched

- `.github/workflows/release.yml` (WR-01, WR-03)
- `.goreleaser.yaml` (WR-02)
- `CONTRIBUTING.md` (WR-04)
- `INSTALL.md` (IN-04)
- `internal/forwarder/forwarder.go` (CR-02, WR-06)
- `internal/upgrade/upgrade.go` (WR-05, IN-01, IN-03)
- `internal/upgrade/verify.go` (CR-01, IN-04)
- `internal/upgrade/verify_test.go` (CR-01, IN-02)

## Commit log (this branch)

```
0395ccb5 fix(58): IN-03 mirror verify-failed postmortem hint to opts.Stdout
1e140ef2 fix(58): IN-01 drop dead `_ = err` no-op on Chmod failure
d99520c6 fix(58): WR-06 coalesce forwarder stdout write and treat rand failure as fatal
fefc3849 fix(58): WR-05 clean stage dir before relaunchFn replaces the process image
9b9933cf fix(58): WR-04 correct CONTRIBUTING.md repro-gate description (no Pass-3)
cefbf767 fix(58): WR-03 extend repro gate find-globs to include checksums.txt
b2b0430b fix(58): WR-02 pin cosign sign-blob --oidc-issuer to GitHub Actions
38cbebc2 fix(58): WR-01 move id-token permission from workflow scope to release-job scope
91d3c261 fix(58): CR-02 register forwarder TracerProvider shutdown before daemon dial
d37d02ae fix(58): CR-01/IN-02/IN-04 remove dead testable-mirror helper and tighten SAN regex
```

## Verification

- `go vet ./internal/...` — clean (the only emission is a pre-existing `TOKEN_COUNT` macro-redefined warning in the vendored Swift tree-sitter binding under `internal/treesitter/bindings/swift/`, unrelated to phase 58).
- `go build ./cmd/helix` — clean exit.
- `go test ./internal/upgrade/ ./internal/forwarder/ ./internal/obs/ -count=1` — all PASS.
- `go test ./internal/... -count=1 -timeout=300s` — all PASS across every internal package.

## Notes on bundled commits

CR-01, IN-02, and IN-04 were committed together in `d37d02ae` because all three touch the same two source files (`internal/upgrade/verify.go` and `internal/upgrade/verify_test.go`) and the changes interlock — splitting the helper deletion (CR-01) from the test deletion (CR-01) and the regex tightening (IN-04) would have required mid-file partial diffs that obscure intent. The commit message explicitly enumerates each finding and its rationale.

All other findings have one commit per finding.

---

_Fixed: 2026-05-03_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
