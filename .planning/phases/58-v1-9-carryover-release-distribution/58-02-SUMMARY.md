---
phase: 58-v1-9-carryover-release-distribution
plan: 02
subsystem: release-distribution
tags: [release, signing, cosign, sigstore, verifier, self-upgrade, tdd]
requires:
  - REL-01 (verifier-side cosign keyless verification)
provides:
  - sigstore-go cosign-keyless VerifyArchive (replaces minisign)
  - pinned identity policy (GitHub Actions OIDC + agenthands/helix release.yml SAN regex)
  - embedded sigstore TUF trust root (refreshable via `make update-trust-root`)
  - user-facing INSTALL.md cosign verify-blob recipe (Q-3 fold-in)
  - test fixtures + generator (testdata/generate_fixtures.go, four bundles, test trust root)
affects:
  - internal/upgrade/{verify.go, upgrade.go, trustroot.go} — semantic rewrite
  - CONTRIBUTING.md — Repository secrets / Trust root refresh sections
  - INSTALL.md — verification recipe + in-binary upgrade flow description
  - Makefile — `update-trust-root` target replaces `embed-pubkey` / `verify-embed-pubkey`
  - go.mod / go.sum — sigstore-go v1.1.4 direct dep, cobra v1.10.2 forced bump
tech-stack:
  added:
    - github.com/sigstore/sigstore-go v1.1.4 (verifier API + pkg/testing/ca for fixtures)
    - github.com/sigstore/protobuf-specs v0.5.0 (transitive, used directly by generator)
  patterns:
    - cosign keyless verification (Fulcio short-lived cert + Rekor SET + TSA timestamp)
    - identity pinning via SAN regex + Fulcio OIDC issuer extension OID (1.3.6.1.4.1.57264.1.1)
    - in-process VirtualSigstore CA for hermetic test fixtures (no network)
    - Pitfall-4 canonical-error invariant preserved across library swap
key-files:
  created:
    - internal/upgrade/trusted_root.json
    - internal/upgrade/testdata/trusted_root.json
    - internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json
    - internal/upgrade/testdata/sample-archive.tar.gz.rc.sigstore.json
    - internal/upgrade/testdata/sample-archive.tar.gz.wrong-org.sigstore.json
    - internal/upgrade/testdata/sample-archive.tar.gz.wrong-issuer.sigstore.json
    - internal/upgrade/testdata/generate_fixtures.go
  modified:
    - internal/upgrade/verify.go (rewrite)
    - internal/upgrade/upgrade.go (asset-name + comment swaps)
    - internal/upgrade/verify_test.go (rewrite — 13 tests)
    - internal/upgrade/upgrade_test.go (mechanical rename + delete placeholder test)
    - Makefile
    - CONTRIBUTING.md
    - INSTALL.md
    - go.mod / go.sum
  renamed:
    - internal/upgrade/pubkey.go → internal/upgrade/trustroot.go
  deleted:
    - minisign.pub
    - internal/upgrade/minisign.pub
    - internal/upgrade/testdata/test_keypair.pub
    - internal/upgrade/testdata/test_keypair.key
    - internal/upgrade/testdata/sample-archive.tar.gz.minisig
decisions:
  - "Phase 58 D-02 / Task 3 Option A: matchesPinnedIdentity is a pure helper on *x509.Certificate, with pinnedCertificateIdentity centralizing the verify.NewShortCertificateIdentity construction so the production policy and the unit-test mirror share one source of truth"
  - "Bundle MediaType v0.1 (X509CertificateChain form 2) — VirtualSigstore.Sign() emits an InclusionPromise SET but no inclusion proof, so v0.2+ bundles cannot be constructed in-process from the test CA without a fake transparency-log root proof"
  - "VirtualSigstore-quirk fix in generator: the test CA stores transparency-log IDs as ASCII hex bytes; the verifier looks them up by raw bytes. The generator hex-decodes IDs into raw bytes before serializing the trust root JSON"
  - "Rekor SET regenerated in the generator via vs.RekorSignPayload over a canonical RekorPayload because tlog.NewEntry stores the SET on a private struct field with no public accessor; the regenerated SET is ECDSA-modulo-nonce-equivalent (any valid signature verifies)"
metrics:
  tasks_completed: 4
  tasks_total: 4
  files_created: 7
  files_modified: 9
  files_renamed: 1
  files_deleted: 5
  duration_minutes: ~60
  completed: 2026-05-03
---

# Phase 58 Plan 02: Verifier rewrite (cosign keyless) Summary

Rewrote `internal/upgrade/` from minisign signature verification to sigstore-go cosign-keyless bundle verification with pinned GitHub Actions OIDC issuer + agenthands/helix release.yml SAN regex. All minisign-era artifacts (root + embedded pub keys, test fixtures, Makefile targets, build-time embed-copy gate) deleted. Trust root JSON now embedded via `//go:embed trusted_root.json`, refreshable via `make update-trust-root`. Updated CONTRIBUTING.md (Trust root refresh section) and INSTALL.md (user-facing `cosign verify-blob --bundle` recipe with the same pinned identity policy as `verify.go`).

The TDD cycle was completed within the plan: 1 RED commit (failing tests + fixtures), 1 GREEN commit (verifier + asset-name swap), 1 REFACTOR commit (helper extraction + central identity policy), and 1 chore commit (artifact deletion + docs/Makefile cleanup).

## Tasks Executed

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1    | RED — failing tests + sigstore fixtures + generator | `1db190af` | verify_test.go, upgrade_test.go, testdata/generate_fixtures.go, testdata/{trusted_root,sample-archive.tar.gz.{sigstore,rc.sigstore,wrong-org.sigstore,wrong-issuer.sigstore}}.json, go.mod/go.sum |
| 2    | GREEN — sigstore-go verifier + asset-name swap | `622259b2` | verify.go (rewrite), trustroot.go (rename + repurpose pubkey.go), trusted_root.json, upgrade.go, go.mod/go.sum |
| 3    | REFACTOR — extract identity matcher (Option A) | `060f7157` | verify.go |
| 4    | Delete minisign artifacts; refresh docs + Makefile | `e22182cd` | minisign.pub, internal/upgrade/minisign.pub, testdata/test_keypair.{pub,key}, testdata/sample-archive.tar.gz.minisig, Makefile, CONTRIBUTING.md, INSTALL.md |

## Task 1 — RED

Generated four cosign bundle fixtures + a test trust root via `internal/upgrade/testdata/generate_fixtures.go` (build-tag `ignore`). The generator uses sigstore-go's in-process `pkg/testing/ca` VirtualSigstore — no network, no real Fulcio / Rekor / TSA — and signs the existing `sample-archive.tar.gz` with four distinct identity policies (canonical, RC tag, wrong-org SAN, wrong issuer). Each bundle round-trips through `bundle.NewBundle` inside the generator to sanity-check the JSON shape before write.

The plan's literal acceptance check `grep -c canonicalErrText internal/upgrade/verify_test.go` returns 9 — one assertion per failure case (10 verify cases minus the happy-path + RC-tag acceptance tests, which assert `err == nil`). The wrong-identity test uses a different-org SAN (`some-other-org/helix`) so the broader regex genuinely rejects it on the org-pinning portion of the regex, not just a tag-shape rejection — matches the plan's fork-SAN posture.

Stayed stdlib-only in `verify_test.go` (no testify), preserving the Pitfall-4 grep gate.

RED state confirmed: tests fail to compile (`undefined: testTrustedRootOverride`, `testRekorURLOverride`, `matchesPinnedIdentity`, `isRekorUnreachable`). Recorded at `/tmp/58-02-red.log`.

## Task 2 — GREEN

`internal/upgrade/verify.go` rewritten end-to-end:
- Drops `github.com/jedisct1/go-minisign`, adds `sigstore-go/pkg/{bundle,root,verify}`.
- `VerifyArchive(archivePath, bundlePath string) error` signature unchanged.
- Pitfall-4 invariant comment block updated (minisign → sigstore wording) but the literal `"signature verification FAILED"` is preserved at every failure branch with the inline `// canonical: signature verification FAILED — see Pitfall 4.` comment.
- `errors.New("signature verification FAILED")` at every literal-only failure branch; `fmt.Errorf("signature verification FAILED: Rekor transparency-log verification requires network access: %w", err)` at the SOLE Rekor-unreachable branch (D-04 user-friendly wording).
- `pinnedOIDCIssuer = "https://token.actions.githubusercontent.com"` and `pinnedSANRegexLiteral = ^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$` — accepting final and pre-release tags (`-rc`, `-beta`, `-alpha`) per goreleaser `prerelease: auto`.
- `WithSignedTimestamps(1)`, `WithTransparencyLog(1)`, `WithIntegratedTimestamps(1)` policy options.
- `currentTrustedRoot()` + `testTrustedRootOverride []byte` mirror the old `currentPubKey()` / `testPubKeyOverride` pattern (Package-private; preserves the test-injection seam).
- `isRekorUnreachable(err error) bool` best-effort substring/errors.As classifier (looks for `*net.OpError`, `connection refused`, `no such host`, `i/o timeout`, etc.).
- `testRekorURLOverride` injection seam for deterministic unit coverage of the Rekor-unreachable wording.

`internal/upgrade/pubkey.go` renamed to `internal/upgrade/trustroot.go` and re-purposed for the trust-root embed (`//go:embed trusted_root.json`); `placeholderMarker`, `IsPlaceholderPubKey`, `currentPubKey`, and the `bytes` import all deleted.

`internal/upgrade/upgrade.go` updated mechanically:
- `sigName` → `bundleName` + `.sigstore.json` (and `stageSig` → `stageBundle`).
- `checksums.txt.minisig` → `checksums.txt.sigstore.json` (both literal sites in the asymmetric-pair guard, plus the comment).
- Step 6 comment: minisign verify → cosign verify the archive bundle.
- The `IsPlaceholderPubKey()` guard at the old lines 122-133 deleted entirely.

`go.mod` / `go.sum` tidied (with `-e` to bypass an unrelated transitive test-dep lookup error in go-openapi/swag/jsonutils — does NOT affect the build or runtime). `sigstore-go v1.1.4` is now a direct require. `cobra v1.9.1 → v1.10.2` was forced by sigstore-go's minimum requirement (sigstore-go v1.1.4 transitively requires cobra v1.10.2).

Production trust root (`internal/upgrade/trusted_root.json`) is sourced from the upstream `sigstore-go/main/examples/trusted-root-public-good.json` snapshot taken on 2026-05-03. SHA snapshot trail: file size 7014 bytes; `cmp -s internal/upgrade/trusted_root.json internal/upgrade/testdata/trusted_root.json` returns non-zero (byte-distinct from the test trust root, which is the `! cmp -s` security invariant — mixing them would erase identity pinning at production).

Test results: `go test ./internal/upgrade/... -count=1` passes (12 tests + the Task 1 helper tests). `go vet ./internal/upgrade/...` clean.

## Task 3 — REFACTOR

The `matchesPinnedIdentity` helper landed alongside the GREEN commit (it backs the `TestMatchesPinnedIdentity_*` unit tests that were RED-shipped in Task 1). The Task 3 commit completes the refactor by adding `pinnedCertificateIdentity()` — a small helper that constructs the `verify.NewShortCertificateIdentity` used by `VerifyArchive`. This centralizes the OIDC issuer + SAN regex source-of-truth between the production policy and the unit-test helper, so drift between them would silently widen the trusted set (an identity-pinning regression).

Refactor option chosen: **Option A** (pure helper on `*x509.Certificate`). Tests `TestMatchesPinnedIdentity_HappyPath` (5 sub-cases: final, -rc1, -rc12, -beta2, -alpha3) and `TestMatchesPinnedIdentity_RejectsWrongRepoSAN` (4 sub-cases: wrong-org, wrong-workflow, wrong-issuer, branch-not-tag) drive the helper directly through hand-constructed certificates with the canonical Fulcio extensions (`subjectAltName` URI + `1.3.6.1.4.1.57264.1.1` OIDC issuer extension).

12 tests pass post-refactor. `VerifyArchive` signature unchanged.

## Task 4 — Delete minisign artifacts; refresh docs + Makefile

Deleted (R-3 ordering hazard now safely past since Task 2 already removed the `//go:embed minisign.pub` directive):
- `minisign.pub` (root)
- `internal/upgrade/minisign.pub`
- `internal/upgrade/testdata/test_keypair.pub`
- `internal/upgrade/testdata/test_keypair.key`
- `internal/upgrade/testdata/sample-archive.tar.gz.minisig`

`Makefile`: removed `embed-pubkey` and `verify-embed-pubkey` from `.PHONY` and dropped them as targets; removed `embed-pubkey` as a `build:` dep; added `update-trust-root` target that downloads the upstream sigstore TUF snapshot and overwrites `internal/upgrade/trusted_root.json`.

`CONTRIBUTING.md`: rewrote `### Repository secrets` for cosign keyless via OIDC token (no long-lived secrets); deleted `### One-time keypair setup` and `### Key rotation` (no keypair concept under cosign keyless); added `### Trust root refresh` section documenting `make update-trust-root` and the refresh cadence ("before each minor release"). Final state: zero `minisign` / `MINISIGN_` literal hits (`grep -ic` returns 0).

`INSTALL.md`: reworked the pre-release note (no PLACEHOLDER concept), and replaced the entire pre-built-binary verification recipe with a `cosign verify-blob --bundle` recipe pinned to the same identity policy as `verify.go` (broader regex including `-rc\d+`, `-beta\d+`, `-alpha\d+` suffixes). The macOS hint switched to `brew install cosign`. Updated the in-binary `helix upgrade` flow description (minisign verification → cosign bundle verification) and the "Replace your binary by hand" fallback (minisign → cosign).

Final consistency check: `go build ./...` succeeds; `go test ./... -count=1` passes across the full repo.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] sigstore-go transitively requires github.com/jedisct1/go-minisign**

- **Found during:** Task 2 GREEN gate
- **Issue:** The plan's Task 2 acceptance gate `! grep -q 'jedisct1/go-minisign' go.mod` was written under the assumption that `go mod tidy` would fully evict go-minisign from the dependency tree. In practice, `sigstore-go/pkg/sign` imports `sigstore/rekor/pkg/pki` which transitively imports `jedisct1/go-minisign` (rekor's PKI module supports minisign-format keys for legacy artifact verification, regardless of helix's signing strategy). After `go mod tidy`, go-minisign moves from the direct `require` block to the indirect block but does not disappear from `go.mod`.
- **Fix:** I removed go-minisign as a *direct* dependency (the only reference helix's own code had to it). The literal grep gate cannot pass without modifying sigstore-go's transitive surface, which is out of scope and would entail vendoring a fork of sigstore-go just to remove rekor-pkg-pki support — clearly disproportionate to the security goal. The plan's intent — "no helix code path verifies minisign signatures" — is satisfied; the literal grep gate is documented as a known-not-met deviation.
- **Files modified:** `go.mod`, `go.sum` (sigstore-go promoted to direct require; go-minisign demoted to indirect).
- **Commit:** `622259b2` (Task 2 GREEN).

**2. [Rule 3 — Blocking] `go mod tidy` failure in unrelated transitive test-only deps**

- **Found during:** Task 2 GREEN tidy
- **Issue:** `go mod tidy` (without flags) fails with `module github.com/go-openapi/testify/v2@latest found (v2.5.0), but does not contain package github.com/go-openapi/testify/v2/assert/yaml`. This is a test-only import in `github.com/go-openapi/swag/jsonutils/fixtures_test`, transitively reached through sigstore-go → rekor-pkg-pki → go-openapi/runtime → go-openapi/swag/jsonutils, none of which run during helix's build or test path.
- **Fix:** Used `go mod tidy -e` to ignore the unrelated lookup failure. The result is byte-identical to a future `go mod tidy` once the transitive issue is resolved upstream — the missing package is referenced only from a `_test.go` file three module hops away from helix.
- **Files modified:** `go.mod`, `go.sum`.
- **Commit:** `622259b2` (Task 2 GREEN).

**3. [Rule 3 — Blocking] cobra v1.9.1 → v1.10.2 forced by sigstore-go**

- **Found during:** Task 1 RED (when adding sigstore-go via `go get`)
- **Issue:** `go get github.com/sigstore/sigstore-go@v1.1.4` rejected the existing `cobra v1.9.1` pin: "sigstore-go v1.1.4 requires github.com/spf13/cobra@v1.10.2". The plan did not anticipate the cobra bump.
- **Fix:** Accepted the cobra v1.9.1 → v1.10.2 bump. Cobra v1 is stable, both follow semver, and helix's CLI code is build-checked clean across the bump (full `go test ./... -count=1` passes).
- **Files modified:** `go.mod`, `go.sum`.
- **Commit:** `1db190af` (Task 1 RED).

**4. [Rule 1 — Bug] Bundle round-trip drops the Rekor SET — fixed in fixture generator**

- **Found during:** Task 1 RED smoke-test of generated fixtures
- **Issue:** `tlog.NewEntry` stores the Rekor signedEntryTimestamp on a private struct field (`entry.signedEntryTimestamp`) but does NOT write it back to the proto's `tle.InclusionPromise.SignedEntryTimestamp`. When the bundle is round-tripped through `protojson.Marshal` and parsed by `bundle.NewBundle`, the SET is missing, and `tlog.VerifySET` fails with "unable to verify SET".
- **Fix:** The generator regenerates the SET by re-signing a canonical `tlog.RekorPayload` via `vs.RekorSignPayload(...)`. The regenerated SET is ECDSA-modulo-nonce-equivalent — any valid signature over the same payload-hash with the same key verifies on the verify side. Documented in the generator code comment block.
- **Files modified:** `internal/upgrade/testdata/generate_fixtures.go`.
- **Commit:** `1db190af` (Task 1 RED).

**5. [Rule 1 — Bug] VirtualSigstore stores transparency-log IDs in ASCII-hex format — fixed in fixture generator**

- **Found during:** Task 1 RED smoke-test of generated fixtures
- **Issue:** The test CA's `TransparencyLog.ID` is set to `[]byte(hexString)` — i.e., the ASCII bytes of the hex-encoded SHA256, not the raw 32-byte hash. The verifier's lookup is `verifiers[hex.EncodeToString(rawBytes(entry.LogKeyID()))]`. So if the trust root's ID is the 64-byte ASCII hex, the lookup key would be `hex(asciiHex)` = 128 chars, mismatched against the entry's 64-char hex. The test trust root would silently fail to verify any SET.
- **Fix:** The generator hex-decodes each transparency-log ID before serializing the trust root JSON, so the trust root carries raw 32-byte IDs that round-trip correctly through the verifier's lookup path.
- **Files modified:** `internal/upgrade/testdata/generate_fixtures.go`.
- **Commit:** `1db190af` (Task 1 RED).

### No PATTERNS line-number drift

The actual `internal/upgrade/` file layout matched `58-PATTERNS.md` exactly. All call sites were located by symbol or `name:` rather than by line number. No PATTERNS-prescribed change was missing or moved.

### Architectural deviations from the plan's task ordering

**Task 3 helper landed in Task 2.** The `matchesPinnedIdentity` helper was implemented in `internal/upgrade/verify.go` during Task 2 because the Task 1 RED tests reference it directly — leaving it undefined would have left the tests still failing to compile after the GREEN commit, which would have re-defined GREEN as "code compiles" rather than "GREEN means tests pass." The Task 3 commit completed the refactor by adding `pinnedCertificateIdentity()` and routing `VerifyArchive` through it; the matcher itself was already in place. This preserves the spirit of the plan's Task 3 (extract for testability, add helper-targeted unit tests, no behavior change) without artificially failing Task 2's GREEN gate.

## Authentication / OIDC gates

None encountered. Cosign keyless signing is OIDC-gated on the release runner (Plan 01 already documented this), but verification is purely offline — Rekor SET is verified against the embedded trust root's public key, no network call required at upgrade time. The RC-tag, wrong-org-SAN, and wrong-issuer test cases use in-process VirtualSigstore CA fixtures (no real Fulcio / Rekor / TSA).

## Threat-model alignment

| Threat ID | Mitigation status after Plan 02 |
|-----------|--------------------------------|
| T-58-02 (Spoofing: tampered archive accepted) | mitigated — sigstore-go DSSE/messageSignature verification against the Fulcio cert chain rooted in the embedded trust root; `TestVerifyArchiveTamperedBundle` + `TestVerifyArchiveWrongTrustRoot` enforce. |
| T-58-03 (Spoofing: attacker-controlled cert with valid Fulcio chain) | mitigated — pinned SAN regex (broader form accepting -rc/-beta/-alpha pre-release tags) + pinned OIDC issuer. `TestVerifyArchiveAcceptsRCTag` regression-guards against narrowing the regex; `TestVerifyArchiveWrongIdentity` (fork-org SAN) and `TestVerifyArchiveWrongIssuer` (different OIDC issuer) drive the negative branch. `TestMatchesPinnedIdentity_RejectsWrongRepoSAN` also covers `branch_not_tag` and `wrong_workflow` directly against the matcher. |
| T-58-04 (DoS / silent acceptance: Rekor unreachable masked as success) | mitigated — `WithTransparencyLog(1)` minimum threshold; `isRekorUnreachable` classifies network errors and surfaces the user-friendly D-04 wording. |
| T-58-02-aux (Tampering: pub-key drift between repo root and embedded copy) | eliminated — the dual-pub-key system + `embed-pubkey` / `verify-embed-pubkey` Make targets are deleted. Only one trust root file (`internal/upgrade/trusted_root.json`) is embedded; refreshable via `make update-trust-root` from upstream sigstore TUF. |
| T-58-03-aux (Spoofing: go-minisign supply-chain compromise) | partially mitigated — go-minisign is no longer a direct dependency, and helix's verifier no longer parses minisign signatures. go-minisign survives as a transitive dep through `sigstore-go/pkg/sign → sigstore/rekor/pkg/pki/minisign` (rekor's PKI module supports minisign-format keys). This residual transitive surface is outside our control without vendoring a fork of sigstore-go. The threat-model entry is unchanged at the helix-code-path layer (no minisign parsing). |
| T-58-03-mix (Production trust root accidentally replaced with the test trust root) | mitigated — production and test trust roots are byte-distinct (`! cmp -s internal/upgrade/trusted_root.json internal/upgrade/testdata/trusted_root.json` succeeds). Documented in CONTRIBUTING.md. |

No new security surface beyond what the threat model already covers.

## Self-Check: PASSED

**Created files:**
- `internal/upgrade/trusted_root.json` — FOUND
- `internal/upgrade/trustroot.go` — FOUND (renamed from pubkey.go, contents replaced)
- `internal/upgrade/testdata/trusted_root.json` — FOUND
- `internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json` — FOUND
- `internal/upgrade/testdata/sample-archive.tar.gz.rc.sigstore.json` — FOUND
- `internal/upgrade/testdata/sample-archive.tar.gz.wrong-org.sigstore.json` — FOUND
- `internal/upgrade/testdata/sample-archive.tar.gz.wrong-issuer.sigstore.json` — FOUND
- `internal/upgrade/testdata/generate_fixtures.go` — FOUND

**Deleted files:**
- `minisign.pub` — DELETED
- `internal/upgrade/minisign.pub` — DELETED
- `internal/upgrade/pubkey.go` — DELETED (renamed to trustroot.go)
- `internal/upgrade/testdata/test_keypair.pub` — DELETED
- `internal/upgrade/testdata/test_keypair.key` — DELETED
- `internal/upgrade/testdata/sample-archive.tar.gz.minisig` — DELETED

**Commits:**
- `1db190af` — FOUND (Task 1 RED)
- `622259b2` — FOUND (Task 2 GREEN)
- `060f7157` — FOUND (Task 3 REFACTOR)
- `e22182cd` — FOUND (Task 4 chore)

**Verify gates:**
- `go test ./internal/upgrade/... -count=1` — ok (12 verify tests + helper tests)
- `go test ./... -count=1` — ok (full repo green)
- `go vet ./internal/upgrade/...` — clean
- `go build ./...` — succeeds
- Pitfall-4: 8 canonical-string returns (comment-stripped), 9 `// canonical:` markers — PASS
- Pitfall-4 strengthening: every `fmt.Errorf("signature verification FAILED…")` carries `// canonical:` — PASS
- Production vs test trust root byte-distinct: `! cmp -s` — PASS
- `grep -c '\.minisig' internal/upgrade/upgrade.go` — 0 — PASS
- `grep -q 'sigstore/sigstore-go' go.mod` — PASS
- `grep -ic 'minisign\|MINISIGN_' CONTRIBUTING.md` — 0 — PASS
- `grep -q 'cosign verify-blob' INSTALL.md` — PASS
- `grep -q 'update-trust-root' Makefile` — PASS
- `grep -c 'embed-pubkey' Makefile` — 0 — PASS

**Plan output requirements:**
- Final sigstore-go version pinned: `v1.1.4`.
- Trusted-root snapshot trail: `https://raw.githubusercontent.com/sigstore/sigstore-go/main/examples/trusted-root-public-good.json`, fetched via `curl -sSL` on 2026-05-03; file size 7014 bytes. The exact 40-char SHA of the file as committed is recorded by `git ls-files -s internal/upgrade/trusted_root.json` and inspectable via `git log internal/upgrade/trusted_root.json`.
- Refactor option chosen: **Option A** (pure helper on `*x509.Certificate`).
- Pitfall-4 grep-gate count: **8** canonical-string returns (comment-stripped) in `internal/upgrade/verify.go`.
- Production and test trust roots are byte-distinct: **YES** (`! cmp -s` succeeds).
- INSTALL.md hosts the user-facing recipe (preflight check found `INSTALL.md` exists at the repo root): **YES**.
