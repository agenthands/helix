---
phase: 58-v1-9-carryover-release-distribution
verified: 2026-05-03T19:13:11Z
status: human_needed
score: 3/4 must-haves verified (1 requires maintainer release-cut to complete)
overrides_applied: 0
gaps:
  - truth: "REL-02, REL-03, REL-04 are explicitly recorded as won't-do under Phase 58 D-01 across PROJECT.md / REQUIREMENTS.md / v1.10-ROADMAP.md / CONTRIBUTING.md"
    status: partial
    reason: "REQUIREMENTS.md, v1.10-ROADMAP.md, and CONTRIBUTING.md updated; PROJECT.md was deliberately deferred to a post-phase update_project_md step that has not yet run. PROJECT.md still lists PKG-DEFER-03/04/05 as 're-evaluate priority' (line 115, 170) and 're-scope or land' (line 154), and still references minisign keypair (lines 112, 162, 167)."
    artifacts:
      - path: ".planning/PROJECT.md"
        issue: "Lines 112, 115, 154, 162, 167-170 still describe minisign keypair, PKG-DEFER-03/04/05 deferred (not won't-do), and Phase 51/55 architectural debt as open. The post-phase update_project_md step required by 58-CONTEXT.md §Deferred has not been executed."
    missing:
      - "Update PROJECT.md line 112 from minisign-keypair gate to cosign keyless"
      - "Update PROJECT.md line 115 from 're-evaluate priority' to 'Won't-do per Phase 58 D-01'"
      - "Update PROJECT.md line 154 from 're-scope or land PKG-DEFER-03/04/05' to record won't-do"
      - "Update PROJECT.md line 162 tech-stack: drop minisign, add sigstore cosign"
      - "Move PROJECT.md tech-debt entries (lines 167-170) from 'accepted at v1.9 close' to a new 'Resolved at v1.10' section once Phase 58 closes"
human_verification:
  - test: "Push v1.10.0-rc1 (or v1.10.0) git tag and verify CI release workflow runs end-to-end"
    expected: ".github/workflows/release.yml completes: cosign-installer pinned SHA loads, GitHub Actions OIDC mints Fulcio cert, cosign sign-blob produces .sigstore.json bundle for every archive, GitHub release uploads all archives + bundles + checksums.txt + checksums.txt.sigstore.json"
    why_human: "Cannot programmatically push tags or trigger CI workflows from a verifier. Local snapshot signing was OIDC-gated (no browser/device-flow available) — the real-CI path can only be validated by an actual tag push by a maintainer. SC-1 requires 'against a real published v1.10.0 release artifact' which does not yet exist."
  - test: "Once v1.10.0 release exists on GitHub: download a prior helix binary, run `helix upgrade`, confirm it verifies the sigstore cosign bundle (Rekor inclusion-proof check) and atomically swaps the binary"
    expected: "helix upgrade succeeds end-to-end against the real v1.10.0 release; the binary is atomically swapped (`syscall.Exec` on Unix or `os.Exit(0)` on Windows). Verifier accepts the production trust root and rejects any tampered or wrong-identity bundle."
    why_human: "Requires a real published release artifact on github.com/agenthands/helix; verifier code was tested only against in-process VirtualSigstore CA fixtures. Real Fulcio/Rekor/TSA bundles can only be produced by the GitHub Actions release runner."
---

# Phase 58: v1.9 Carryover — Release & Distribution Verification Report

**Phase Goal:** Cut the first real signed Helix release, close the v1.9 deployment-gated and architectural-debt items, and explicitly de-scope the deferred distribution channels (Homebrew, Scoop, native Linux package) — all in parallel with the early semantic phases so release risk is not concentrated near the v1.10 ship date.
**Verified:** 2026-05-03T19:13:11Z
**Status:** human_needed
**Re-verification:** No — initial verification.

## Goal Achievement

### Observable Truths

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| SC-1 | `helix upgrade` against a real published v1.10.0 release verifies a sigstore cosign keyless signature (Rekor inclusion-proof) and atomically swaps the binary; CI uses Actions OIDC → Fulcio → cosign sign-blob → Rekor | ⚠️ NEEDS HUMAN | Verifier wired (`internal/upgrade/verify.go` calls `verify.NewSignedEntityVerifier` with `WithSignedTimestamps(1) WithTransparencyLog(1) WithIntegratedTimestamps(1)` + pinned identity); CI signing wired (`.goreleaser.yaml:51-60` cosign sign-blob, `.github/workflows/release.yml:8-10` `id-token: write`, line 32-35 cosign-installer pinned). All upgrade tests pass (12 verify tests via in-process VirtualSigstore fixtures). However, **no v1.10.x tag has been pushed** (`git tag -l v1.10*` returns empty) so the "real published" artifact does not yet exist; this is the human-verification item below. Atomic swap path: `internal/upgrade/upgrade.go:281` calls `swapFn(exec, newBin)` then `relaunchFn`. |
| SC-2 | CONTRIBUTING.md documents the Phase 51 reproducibility-gate Pass-3 limitation | ✓ VERIFIED | `CONTRIBUTING.md:163` contains the literal phrase `Pass-3 limitation` and the reopen-path wording `comparison job can be added`. Note: REVIEW WR-04 flags that the paragraph text says "compares Pass-1, Pass-2, and Pass-3 snapshot hashes" but `release.yml` only runs two passes — the literal anchor phrase satisfies the SC, but the surrounding rationale text is internally inconsistent (recorded as a Warning below). |
| SC-3 | `forwarder.tools.call` OTel client span is correlated with the daemon-side gRPC server span via TraceContext propagation | ✓ VERIFIED | `internal/forwarder/dial.go:72` calls `obs.ClientStatsHandler(tp)`; `internal/daemon/daemon.go:573` calls `obs.ServerStatsHandler(d.obs.TracerProvider())`; `internal/obs/grpc.go:36,46` wires `propagation.TraceContext{}` per-handler (no global). `test/integration/trace_continuity_test.go` asserts `require.Equal(fwdStub.SpanContext.TraceID(), srvStub.SpanContext.TraceID())` — test passes (`go test -tags=integration -run TestE2ETraceContinuity ./test/integration/ -count=3`: ok 1.472s). |
| SC-4 | REL-02/03/04 explicitly recorded as won't-do under Phase 58 D-01 across PROJECT.md / REQUIREMENTS.md / v1.10-ROADMAP.md / CONTRIBUTING.md | ✗ FAILED (partial) | REQUIREMENTS.md:130-132 carry `- [~]` markers + canonical D-01 rationale; v1.10-ROADMAP.md:45,50 lists `REL-01, REL-05, REL-06` only and points to D-01; REQUIREMENTS.md:9 has Status legend. **PROJECT.md was deliberately deferred** to a post-phase `update_project_md` step (per 58-CONTEXT.md §Deferred and 58-04-PLAN.md note) and has NOT been updated — PKG-DEFER-03/04/05 still listed as "re-evaluate priority" (lines 115, 170) and "re-scope or land" (line 154); minisign tech debt still listed as open (lines 112, 167). The phase plans collectively did not own PROJECT.md edits, but SC-4 explicitly requires PROJECT.md to record the won't-do. |

**Score:** 2/4 truths fully verified, 1 requires human (SC-1), 1 partial (SC-4).

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `.goreleaser.yaml` | cosign keyless `signs:` block | ✓ VERIFIED | Lines 51-60: `id: cosign`, `cmd: cosign`, `sign-blob`, `--bundle=${signature}`, `--yes`. Zero `minisign` literals. |
| `.github/workflows/release.yml` | `id-token: write` + cosign-installer pinned | ✓ VERIFIED | Line 10: `id-token: write`. Line 33: `sigstore/cosign-installer@7e8b541eb2e61bf99390e1afd4be13a184e9ebc5  # v3.10.1` (40-char SHA pin). Zero minisign references. |
| `internal/upgrade/verify.go` | `VerifyArchive` using sigstore-go | ✓ VERIFIED | Lines 14-17: imports `sigstore-go/pkg/{bundle,root,verify}`. Line 121: `func VerifyArchive(archivePath, bundlePath string) error`. Pinned SAN regex line 54, OIDC issuer line 44. |
| `internal/upgrade/trustroot.go` | `//go:embed trusted_root.json` | ✓ VERIFIED | Line 20-21: `//go:embed trusted_root.json` directive on `var trustedRootJSON []byte`. |
| `internal/upgrade/trusted_root.json` | Production sigstore TUF trust root | ✓ VERIFIED | 7014 bytes; sourced from upstream sigstore-go `examples/trusted-root-public-good.json`. Byte-distinct from test trust root. |
| `internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json` (and 3 sibling fixtures) | Bundle fixtures | ✓ VERIFIED | All four (canonical + RC + wrong-org + wrong-issuer) present, drive `verify_test.go`'s 12 tests (all pass). |
| `internal/upgrade/testdata/generate_fixtures.go` | Maintainer regeneration script | ✓ VERIFIED | File present (build-tag `ignore`); uses `pkg/testing/ca` VirtualSigstore. |
| `Makefile` | `update-trust-root` target | ✓ VERIFIED | `grep -q 'update-trust-root' Makefile` passes; `embed-pubkey` / `verify-embed-pubkey` removed. |
| `CONTRIBUTING.md` | Trust root refresh + §Tracing + Pass-3 limitation | ✓ VERIFIED | Line 186: `### Trust root refresh`; line 191: `make update-trust-root`; line 205: `## Tracing`; line 211: `OTEL_EXPORTER_OTLP_ENDPOINT`; line 220: `WithPropagators(propagation.TraceContext{})`; line 226: `trace_continuity_test.go`; line 163: `Pass-3 limitation` + reopen-path wording. |
| `INSTALL.md` | `cosign verify-blob --bundle` recipe | ✓ VERIFIED | (Plan 02 SUMMARY confirms; not re-read in this verification, but Plan 02's grep gate `grep -q 'cosign verify-blob' INSTALL.md` passed at execution time.) |
| `internal/forwarder/forwarder.go` | `obs.WithTracing` env-driven Provider | ✓ VERIFIED | Lines 27-32: reads `OTEL_EXPORTER_OTLP_ENDPOINT`, calls `obs.WithTracing(...)`; degraded-optional fallback documented. |
| `internal/forwarder/dial.go` | otelgrpc with TraceContext propagator via helper | ✓ VERIFIED | Line 72: `grpc.WithStatsHandler(obs.ClientStatsHandler(tp))`. |
| `internal/daemon/daemon.go` | Server-side propagator via helper | ✓ VERIFIED | Line 573: `grpc.StatsHandler(obs.ServerStatsHandler(d.obs.TracerProvider()))`. |
| `internal/obs/grpc.go` | Centralised propagator wiring | ✓ VERIFIED | Lines 33-48: `ClientStatsHandler` + `ServerStatsHandler` both pin `propagation.TraceContext{}`. Zero `otel.SetTextMapPropagator` calls (only doc comments). |
| `test/integration/trace_continuity_test.go` | E2E TraceID continuity test | ✓ VERIFIED | Build tag `integration`; `require.Equal(fwdStub.SpanContext.TraceID(), srvStub.SpanContext.TraceID())` line 189; SERVER-kind filter applied; passes 3x consecutive runs. |
| `.planning/REQUIREMENTS.md` | REL-02/03/04 won't-do markers + Status legend | ✓ VERIFIED | Lines 130-132 each have `- [~]` + canonical D-01 rationale string. Line 9: `## Status legend` section. |
| `.planning/milestones/v1.10-ROADMAP.md` | Phase 58 entry shrunken | ✓ VERIFIED | Line 13 (top-of-file summary): cosign, no minisign/Homebrew/Scoop/Linux. Line 45: `**Requirements**: REL-01, REL-05, REL-06`. Line 50: D-01 cross-reference blockquote. |
| `.planning/PROJECT.md` | Won't-do recording for REL-02/03/04 | ✗ MISSING | No edits applied. Lines 112, 115, 154, 162, 167-170 still reflect pre-Phase-58 state (minisign keypair carry-over, PKG-DEFER-03/04/05 as "re-evaluate priority"/"re-scope or land", minisign in tech stack). |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `.github/workflows/release.yml` | `.goreleaser.yaml signs.cosign` | goreleaser invocation w/ cosign on PATH | ✓ WIRED | cosign-installer step (line 32-35) precedes the `Real release` goreleaser step (line 77-86). |
| `internal/upgrade/upgrade.go` | `internal/upgrade/verify.go::VerifyArchive` | Step 6 verify call | ✓ WIRED | Line 254: `if err := VerifyArchive(stageArchive, stageBundle); err != nil`. Line 243: also `VerifyArchive(stageChecksums, stageChecksumsSig)` for the asymmetric-pair guard. |
| `internal/upgrade/verify.go` | `sigstore-go/pkg/verify` | import + verifier construction | ✓ WIRED | Line 16: import; line 152: `verify.NewSignedEntityVerifier(tr, ...)`. |
| `internal/upgrade/trustroot.go` | `internal/upgrade/verify.go::currentTrustedRoot()` | package-private accessor with test override | ✓ WIRED | trustroot.go embeds `trustedRootJSON`; verify.go:85-90 `currentTrustedRoot()` + `testTrustedRootOverride` seam. |
| `internal/forwarder/forwarder.go` | `internal/obs/tracing.go::WithTracing` | Provider construction | ✓ WIRED | Line 28: `fwdProvider := obs.WithTracing(...)`. |
| `internal/forwarder/dial.go` | `propagation.TraceContext{}` | TraceContext propagator option | ✓ WIRED (via helper) | Single `WithStatsHandler(obs.ClientStatsHandler(tp))` call site (line 72); helper at `internal/obs/grpc.go:36` wires `propagation.TraceContext{}`. |
| `internal/daemon/daemon.go` | `propagation.TraceContext{}` | TraceContext propagator option | ✓ WIRED (via helper) | Single `StatsHandler(obs.ServerStatsHandler(...))` call site (line 573); helper at `internal/obs/grpc.go:46` wires `propagation.TraceContext{}`. |
| `test/integration/trace_continuity_test.go` | `tracetest.NewInMemoryExporter` | in-memory span collection | ✓ WIRED | Test imports `tracetest` and asserts TraceID equality. |
| `.planning/REQUIREMENTS.md` | `.planning/milestones/v1.10-ROADMAP.md` Phase 58 Requirements line | matching REL-IDs | ✓ WIRED | Both list REL-01, REL-05, REL-06 (won't-do for 02/03/04). |
| `.planning/REQUIREMENTS.md` REL-02/03/04 rationale | `58-CONTEXT.md` D-01 | exact rationale string match | ✓ WIRED | Three lines carry the canonical D-01 rationale verbatim. |

### Data-Flow Trace (Level 4)

Not applicable in the dynamic-rendering sense — Phase 58 produces verifier code paths and CI YAML, not data-rendering UI. Equivalent flow checks performed:

| Artifact | Data Source | Produces Real Data | Status |
| -------- | ----------- | ------------------ | ------ |
| `VerifyArchive` | embedded `trustedRootJSON` (or `testTrustedRootOverride`) | Yes — 7014-byte production trust root sourced from upstream sigstore TUF | ✓ FLOWING |
| `obs.WithTracing` in forwarder | `OTEL_EXPORTER_OTLP_ENDPOINT` env var | Yes when set, else degraded-optional Noop | ✓ FLOWING |
| `obs.ClientStatsHandler` / `ServerStatsHandler` | `tp` argument from caller | Yes — propagator wires `propagation.TraceContext{}` | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Upgrade verifier accepts canonical bundle, rejects tampered/wrong-identity/wrong-issuer | `go test ./internal/upgrade/... -count=1` | ok 0.376s (12 tests pass) | ✓ PASS |
| Forwarder/daemon/obs build clean | `go build ./internal/upgrade/... ./internal/forwarder/... ./internal/daemon/... ./internal/obs/... ./test/integration/...` | clean (only pre-existing CGO warning in unrelated swift binding) | ✓ PASS |
| Forwarder/daemon/obs vet clean | `go vet ./internal/upgrade/... ./internal/forwarder/... ./internal/daemon/... ./internal/obs/...` | clean | ✓ PASS |
| Phase 58 unit + integration test sweep | `go test ./internal/upgrade/... ./internal/forwarder/... ./internal/daemon/... ./internal/obs/... -count=1` | ok all 4 packages | ✓ PASS |
| TraceID continuity contract | `go test -tags=integration -run TestE2ETraceContinuity ./test/integration/ -count=3` | ok 1.472s (3 consecutive runs, no flake) | ✓ PASS |
| No global propagator | `grep -RIn 'otel\.SetTextMapPropagator' . --include='*.go'` | 2 hits, both in doc comments (`internal/obs/grpc.go:14`, `test/integration/trace_continuity_test.go:9`) | ✓ PASS |
| End-to-end release-runner pass | (would require `git tag -a v1.10.0` + push + workflow run) | not possible from verifier | ? SKIP — see human verification below |
| Real `.sigstore.json` produced by goreleaser | local snapshot run hit OIDC device-flow gate (`/tmp/58-01-snapshot.log` per Plan 01 SUMMARY) | accepted as documented local-vs-CI distinction; CI not yet exercised | ? SKIP — see human verification below |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| REL-01 | 58-01, 58-02 | First signed Helix release end-to-end via `goreleaser` (D-02 cosign keyless replaces minisign); `helix upgrade` verifies signature and atomically swaps | ⚠️ NEEDS HUMAN | Code wired and tested (verifier + CI); awaiting maintainer tag-push to validate end-to-end. **Note:** REQUIREMENTS.md:129 still describes REL-01 in minisign terms — text not refreshed to match the Phase 58 D-02 cosign keyless reality. Minor doc drift; SC behavior is satisfied. |
| REL-02 | (none) | Homebrew tap | ✓ SATISFIED (won't-do) | REQUIREMENTS.md:130 marked `- [~]` with D-01 rationale; v1.10-ROADMAP.md no longer references; CONTRIBUTING.md / PROJECT.md note follow-up needed. |
| REL-03 | (none) | Scoop bucket | ✓ SATISFIED (won't-do) | REQUIREMENTS.md:131 marked `- [~]` with D-01 rationale. |
| REL-04 | (none) | Native Linux package | ✓ SATISFIED (won't-do) | REQUIREMENTS.md:132 marked `- [~]` with D-01 rationale. |
| REL-05 | 58-04 | Reproducibility gate Pass-3 limitation documented | ✓ SATISFIED | CONTRIBUTING.md:163 contains literal phrase `Pass-3 limitation` + reopen-path wording. (Internal text inconsistency flagged in Anti-Patterns.) |
| REL-06 | 58-03 | `forwarder.tools.call` span unified with gRPC server span | ✓ SATISFIED | TestE2ETraceContinuity passes; per-handler propagator wired via `obs.{Client,Server}StatsHandler`. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| `internal/upgrade/verify.go` | 213-238 | `matchesPinnedIdentity` is a tested mirror but **not called by `VerifyArchive`** (REVIEW CR-01). Mirror regex can drift from `pinnedCertificateIdentity()` silently widening identity policy. Fixture-driven tests for `branch_not_tag` + `wrong_workflow` don't exist. | ⚠️ Warning | Documentation in `pinnedCertificateIdentity` ("unit tests share the same source of truth") is misleading — the unit tests do NOT go through this function. |
| `internal/forwarder/forwarder.go` | 28-37 | TracerProvider constructed (lines 28-32) BEFORE `ConnectOrStartDaemon` dial (line 34); shutdown defer registered AFTER dial-success at line 41 (REVIEW CR-02). On dial-failure early return (line 36), the SDK batch span processor + OTLP gRPC connection leak. | 🛑 Blocker (resource leak) | Single-shot CLI invocations may hide it; long-lived/retry flows leak per failed attempt. |
| `.github/workflows/release.yml` | 8-10 | `id-token: write` granted at WORKFLOW scope, not job scope (REVIEW WR-01). Future jobs added to this workflow inherit OIDC mint capability. | ⚠️ Warning | Least-privilege violation; not a security bug today (only one job exists). |
| `.goreleaser.yaml` | 51-60 | `cosign sign-blob` args do not pin `--oidc-issuer` (REVIEW WR-02). cosign auto-detects the issuer; misconfigured runner could mint a cert under a different issuer. | ⚠️ Warning | Defense-in-depth gap; verifier rejects mismatched issuer at upgrade time, but better to fail at sign time. |
| `.github/workflows/release.yml` | 44-67 | Reproducibility gate diffs only `*.tar.gz`; `checksums.txt` stability not verified (REVIEW WR-03). | ⚠️ Warning | Non-determinism in checksums.txt entry order would not be caught. |
| `CONTRIBUTING.md` | 163 | Paragraph claims "compares Pass-1, Pass-2, **and Pass-3** snapshot hashes" but `release.yml` runs only two passes (REVIEW WR-04). The literal anchor `Pass-3 limitation` satisfies the SC, but the surrounding rationale is internally inconsistent — readers may infer a third pass exists. | ⚠️ Warning | Doc drift; the "Pass-3 limitation" framing was probably intended as "the limitation IS that there's no Pass-3 (containerized comparison)." Reads ambiguously. |
| `internal/upgrade/upgrade.go` | 169-180, 297-300 | `defer cleanup()` for stage dir runs only on `Upgrade` return; `relaunchFn` replaces process image (Unix `syscall.Exec`) or calls `os.Exit(0)` (Windows) — neither returns. Stage dirs persist indefinitely on success path (REVIEW WR-05). | ⚠️ Warning | After 10 upgrades, 10 stage tree copies on disk. |
| `internal/forwarder/forwarder.go` | 111-113, 125-129 | Two unsynchronized stdout writes for response payload (a); `_, _ = io.ReadFull(cryptoRand.Reader, b)` discards rand error (b) — could collapse all session IDs to zero (REVIEW WR-06). | ⚠️ Warning | (a) currently safe since slog → stderr; (b) only triggers in chroot/FIPS edge cases. |
| `internal/upgrade/upgrade.go` | 276-280 | `_ = err` dead expression on chmod failure (REVIEW IN-01). | ℹ️ Info | Cosmetic; `err` falls out of scope regardless. |
| `internal/upgrade/verify_test.go` | 232 | Stray `_ = errors.New` left over from earlier draft (REVIEW IN-02). | ℹ️ Info | Cosmetic. |
| `internal/upgrade/upgrade.go` | 179 | Postmortem hint goes to `os.Stderr` directly, bypassing the test-injected `Stdout` writer (REVIEW IN-03). | ℹ️ Info | Test wording captures fragile. |
| `internal/upgrade/verify.go` | 54 | `pinnedSANRegexLiteral` uses `v[\d.]+` — accepts ill-formed tags like `v1.2.3.4` or `v..1` (REVIEW IN-04). Mitigation today is upstream Fulcio + Actions only mint certs for actual pushed tags. | ℹ️ Info | Defense-in-depth tightening; not a security bug today. |
| `internal/upgrade/upgrade.go` | 103, 124 | Stale doc comments: "minisign verify" still in flow description; "removed alongside the minisign embed" reference. | ℹ️ Info | Non-load-bearing prose drift after the cosign cutover. |
| `internal/upgrade/github.go` | 314 | Stale comment referencing minisign verification | ℹ️ Info | Non-load-bearing. |
| `.planning/REQUIREMENTS.md` | 129 | REL-01 description still talks about "minisign keypair" and "PLACEHOLDER pubkey" despite Phase 58 D-02 cosign cutover. | ℹ️ Info | Requirement-text drift; runtime behavior matches success criterion. Plan 04 didn't include REL-01 description refresh in its scope. |
| `.planning/PROJECT.md` | 112, 115, 154, 162, 167-170 | Tech-debt entries still reference minisign keypair, PKG-DEFER-03/04/05 as "re-evaluate priority"/"re-scope or land", minisign in tech stack — none updated to reflect Phase 58 closures. | 🛑 Blocker (SC-4 partial fail) | Required by SC-4. Plans deliberately deferred to a post-phase `update_project_md` step that has not run. |

### Human Verification Required

#### 1. Tag-pushed CI release dry-run (or first real v1.10.x release)

**Test:** Push `v1.10.0-rc1` (or full `v1.10.0`) git tag from a green-CI commit on `main`. Watch `.github/workflows/release.yml` execute end-to-end on a real GitHub Actions runner.
**Expected:**
- cosign-installer step pins `7e8b541eb2e61bf99390e1afd4be13a184e9ebc5  # v3.10.1` and resolves; cosign v2.4.1 lands on PATH.
- Reproducibility gate Pass-1 + Pass-2 match (sha256 diff returns no changes).
- Real release pass: GitHub Actions OIDC token retrieved automatically, Fulcio mints short-lived cert, cosign sign-blob produces `${archive}.sigstore.json` for each of the 6 archives + checksums.txt, Rekor inclusion-proof entry uploaded.
- GitHub release page lists 6 archives + 6 `.sigstore.json` bundles + `checksums.txt` + `checksums.txt.sigstore.json`.
**Why human:** Cannot push tags or trigger CI workflows from a verifier agent. Local snapshot signing was OIDC device-flow-gated (Plan 01 SUMMARY confirms cosign reached `Enter the verification code MTMS-JSQR …` and timed out). The actual end-to-end CI path can only be exercised by a tag push.

#### 2. Real `helix upgrade` against the published v1.10.0 release

**Test:** From a v1.9 (or pre-v1.10.0) helix binary, run `helix upgrade`. Verify the binary downloads the new archive + `.sigstore.json`, sigstore-go verifies the bundle against the embedded `trustedRootJSON`, identity pinning matches the `agenthands/helix/.github/workflows/release.yml@refs/tags/v1.10.x` SAN regex + GitHub Actions OIDC issuer extension, and the binary atomically swaps + relaunches.
**Expected:** `helix upgrade` succeeds; the new `helix --version` reports v1.10.0; daemon-aware re-launch occurs (or daemon-running refusal is shown if D-13's locked behavior triggers).
**Why human:** Requires a real published release artifact on github.com/agenthands/helix. Unit tests use in-process VirtualSigstore CA fixtures only — real Fulcio/Rekor/TSA bundle verification has not been exercised against the embedded production trust root.

### Gaps Summary

**One real gap (SC-4 partial):** `.planning/PROJECT.md` was not updated. Phase 58's plans deliberately deferred PROJECT.md changes to a post-phase `update_project_md` step (per `58-CONTEXT.md` §Deferred and `58-04-PLAN.md:51`), but SC-4 explicitly requires PROJECT.md to record the won't-do across the doc surface. Specifically, lines 112, 115, 154, 162, and 167-170 still describe minisign-keypair carry-over, PKG-DEFER-03/04/05 as "re-evaluate priority"/"re-scope or land", and minisign in the tech stack. These should move to a "Resolved at v1.10" section or be flipped to won't-do markers.

**One real review-blocker (CR-02):** `internal/forwarder/forwarder.go:27-37` constructs the OTel TracerProvider before the daemon dial; the shutdown defer is installed only after the dial succeeds. On dial-failure early return, the SDK batch span processor + OTLP gRPC connection are leaked. This is a real resource leak documented in REVIEW CR-02 with a concrete one-line fix (move shutdown defer immediately after Provider construction). Not an SC-blocker (SC-3 only checks the trace continuity contract), but worth flagging for closure planning since REL-06's "real spans unified" criterion implicitly relies on the forwarder being a well-behaved tracer.

**Several other REVIEW findings** (CR-01 dead-mirror identity helper, WR-01 workflow-scope id-token, WR-02 unpinned `--oidc-issuer`, WR-03 checksums.txt not in repro gate, WR-04 Pass-3 doc inconsistency, WR-05 stage-dir leak, WR-06 forwarder I/O hardening, IN-01..IN-04, doc drift in upgrade.go and REQUIREMENTS.md REL-01) are warnings or info-level and do not block goal achievement, but should be tracked as follow-up.

**Score recap:**
- SC-1 — code wired and unit-tested; awaiting maintainer release-cut for end-to-end validation against a real artifact (human-verification item).
- SC-2 — VERIFIED (literal anchor present; minor surrounding-text inconsistency).
- SC-3 — VERIFIED (TestE2ETraceContinuity passes 3x; no global propagator; per-handler propagator wired via centralised helpers).
- SC-4 — PARTIAL (REQUIREMENTS.md / v1.10-ROADMAP.md / CONTRIBUTING.md done; **PROJECT.md missing**).

---

_Verified: 2026-05-03T19:13:11Z_
_Verifier: Claude (gsd-verifier)_
