---
phase: 58-v1-9-carryover-release-distribution
plan: 02
type: tdd
wave: 2
depends_on: ["58-01"]
files_modified:
  - go.mod
  - go.sum
  - internal/upgrade/verify.go
  - internal/upgrade/verify_test.go
  - internal/upgrade/upgrade.go
  - internal/upgrade/pubkey.go
  - internal/upgrade/trustroot.go
  - internal/upgrade/trusted_root.json
  - internal/upgrade/testdata/trusted_root.json
  - internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json
  - internal/upgrade/testdata/generate_fixtures.go
  - Makefile
  - CONTRIBUTING.md
  - INSTALL.md  # if INSTALL.md does not exist at execution time, the recipe lands in README.md §Installation; executor preflight-checks and updates frontmatter
  - minisign.pub
  - internal/upgrade/minisign.pub
  - internal/upgrade/testdata/test_keypair.pub
  - internal/upgrade/testdata/test_keypair.key
  - internal/upgrade/testdata/sample-archive.tar.gz.minisig
autonomous: true
requirements: [REL-01]
must_haves:
  truths:
    - "VerifyArchive accepts a valid sigstore bundle backed by a pinned trusted root"
    - "VerifyArchive accepts pre-release tags (v1.10.0-rc1, -beta2, -alpha3) under the broader SAN regex"
    - "VerifyArchive rejects bundles with wrong cert identity SAN (T-58-03)"
    - "VerifyArchive rejects bundles with wrong OIDC issuer (T-58-03)"
    - "VerifyArchive rejects tampered bundles (T-58-02)"
    - "VerifyArchive returns the literal string 'signature verification FAILED' at every failure branch (Pitfall-4 invariant)"
    - "helix upgrade requests `${archive}.sigstore.json` instead of `${archive}.minisig`"
    - "When Rekor is unreachable, the verifier returns a structured error explicitly mentioning 'Rekor transparency-log verification requires network access' (D-04)"
    - "Zero minisign artifacts (.pub keys, Makefile targets, go-minisign import, .minisig fixtures) remain in the repo"
    - "Trust root JSON is embedded via //go:embed and refreshable via `make update-trust-root`"
    - "Production trust root (internal/upgrade/trusted_root.json) differs from the test trust root (internal/upgrade/testdata/trusted_root.json) — mixing them would disable identity pinning"
  artifacts:
    - path: "internal/upgrade/verify.go"
      provides: "VerifyArchive(archivePath, bundlePath string) error using sigstore-go"
      contains: "func VerifyArchive"
    - path: "internal/upgrade/trustroot.go"
      provides: "//go:embed trusted_root.json into trustedRootJSON"
      contains: "//go:embed trusted_root.json"
    - path: "internal/upgrade/trusted_root.json"
      provides: "Production sigstore TUF trust root snapshot embedded into the binary"
    - path: "internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json"
      provides: "Bundle fixture used by happy-path test"
    - path: "internal/upgrade/testdata/generate_fixtures.go"
      provides: "Maintainer script regenerating bundle + trust root fixtures"
    - path: "Makefile"
      provides: "update-trust-root target; embed-pubkey/verify-embed-pubkey targets removed"
      contains: "update-trust-root"
    - path: "CONTRIBUTING.md"
      provides: "Cosign keyless ceremony + trust-root refresh sections"
      contains: "Trust root refresh"
    - path: "INSTALL.md"
      provides: "User-facing cosign verify-blob --bundle recipe (Q-3 fold-in)"
      contains: "cosign verify-blob --bundle"
  key_links:
    - from: "internal/upgrade/upgrade.go"
      to: "internal/upgrade/verify.go::VerifyArchive"
      via: "Step 6 verify call"
      pattern: "VerifyArchive\\(stage"
    - from: "internal/upgrade/verify.go"
      to: "github.com/sigstore/sigstore-go/pkg/verify"
      via: "import + verifier construction"
      pattern: "sigstore-go/pkg/verify"
    - from: "internal/upgrade/trustroot.go"
      to: "internal/upgrade/verify.go::currentTrustedRoot()"
      via: "package-private accessor with test override"
      pattern: "testTrustedRootOverride"
---

<objective>
Rewrite `internal/upgrade/` from minisign verification to sigstore-go bundle verification with pinned OIDC issuer + SAN regex. Delete all minisign artifacts (root + embedded pub keys, test fixtures, Makefile targets, go-minisign dependency). Add a trust-root JSON embed, a fixture-generation script, the new bundle fixture, and a maintainer-facing trust-root refresh target. Update INSTALL.md with the user-side `cosign verify-blob --bundle` recipe (Q-3 default).

This plan depends on Plan 01 (release side must already produce `.sigstore.json` artifacts; otherwise the asset-name swap in `upgrade.go` would target nonexistent files).

This is a TDD plan: tests are written RED first (including the three NEW tests for wrong-identity, wrong-issuer, and RC-tag acceptance), then GREEN with the sigstore-go verifier, then REFACTOR extracts the identity matcher.

Purpose: REL-01 verifier (D-02 cosign keyless, D-03 hard cut, D-04 Rekor-required online). Closes T-58-02 (signature spoofing) and T-58-03 (identity confusion).
Output: A green `go test ./internal/upgrade/...` with nine test cases (six rewritten + three NEW), a single coordinated commit set covering rename + embed + delete (per PATTERNS R-3 ordering hazard), and updated docs.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-VALIDATION.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-01-release-side-cosign-PLAN.md
@internal/upgrade/verify.go
@internal/upgrade/verify_test.go
@internal/upgrade/upgrade.go
@internal/upgrade/pubkey.go
@CONTRIBUTING.md
@Makefile

<interfaces>
<!-- Existing minisign-era surface (extracted from PATTERNS) — being replaced. -->

From internal/upgrade/verify.go (current — to be rewritten):
```go
// VerifyArchive verifies the minisign signature of an archive.
// Returns nil on success; returns an error containing the literal string
// "signature verification FAILED" at every failure branch (Pitfall-4).
func VerifyArchive(archivePath, sigPath string) error
```

Post-rewrite shape (preserve signature; semantics swap minisign → sigstore bundle):
```go
// VerifyArchive verifies a sigstore bundle against the embedded trusted root.
// Returns nil on success; returns an error containing the literal string
// "signature verification FAILED" at every failure branch (Pitfall-4).
// On Rekor-unreachable failure, the wrapped error message also contains
// "Rekor transparency-log verification requires network access" (D-04).
func VerifyArchive(archivePath, bundlePath string) error
```

From internal/upgrade/upgrade.go (current — only string constants change):
```go
archiveName := archiveAssetName(rel.TagName, runtime.GOOS, runtime.GOARCH)
sigName := archiveName + ".minisig"
// → bundleName := archiveName + ".sigstore.json"
checksumsAsset := rel.FindAsset("checksums.txt")
checksumsSigAsset := rel.FindAsset("checksums.txt.minisig")
// → checksumsSigAsset := rel.FindAsset("checksums.txt.sigstore.json")
```

New imports for verify.go (per PATTERNS):
```go
"github.com/sigstore/sigstore-go/pkg/bundle"
"github.com/sigstore/sigstore-go/pkg/root"
"github.com/sigstore/sigstore-go/pkg/verify"
```

Identity-pinning policy (from RESEARCH §"Identity pinning"; see RESEARCH line 139 / 440 for canonical regex):
- Issuer: `https://token.actions.githubusercontent.com`
- SAN regex: `^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$`
  - Accepts both final tags (`v1.10.0`) AND pre-release tags produced by goreleaser `prerelease: auto` in `.goreleaser.yaml:94` (`v1.10.0-rc1`, `v1.10.0-beta2`, `v1.10.0-alpha3`).
  - Final-tag-only form (`v[0-9]+\.[0-9]+\.[0-9]+$`) would reject any signed pre-release self-upgrade — DO NOT narrow.

Canonical error literal (Pitfall-4):
```
signature verification FAILED
```
With inline comment `// canonical: signature verification FAILED — see Pitfall 4.` above every `errors.New(...)` AND `fmt.Errorf("signature verification FAILED…")` failure return.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: RED — write failing tests for the cosign verifier (rewrite + 3 NEW)</name>
  <files>internal/upgrade/verify_test.go, internal/upgrade/testdata/generate_fixtures.go, internal/upgrade/testdata/trusted_root.json, internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json</files>
  <read_first>
    - internal/upgrade/verify_test.go (current — 6 minisign cases + helper at lines 19-28)
    - internal/upgrade/verify.go (current — Pitfall-4 invariant block at lines 9-19; canonical error literal `"signature verification FAILED"`)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`internal/upgrade/verify_test.go`" (test-case map: 6 renames + 2 NEW; this plan adds a third NEW for RC-tag regression)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"Pitfall-4 canonical-error-string invariant"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-VALIDATION.md rows for 58-P2 (commands per case)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md §"Identity pinning" line 139/440 (broader SAN regex defaults)
  </read_first>
  <behavior>
    - TestVerifyArchiveHappyPath: a freshly-generated bundle against the test trust root + ephemeral Fulcio-equivalent key for the canonical archive verifies successfully (returns nil). SAN of fixture is `…/agenthands/helix/.github/workflows/release.yml@refs/tags/v1.10.0`.
    - **NEW** TestVerifyArchiveAcceptsRCTag: a bundle whose certificate SAN is `…/agenthands/helix/.github/workflows/release.yml@refs/tags/v1.10.0-rc1` verifies successfully (regression guard against regex over-tightening). Also covers `-beta\d+` and `-alpha\d+` tag suffixes via table-driven sub-tests.
    - TestVerifyArchiveTamperedBundle: a bundle with one byte flipped in its DSSE envelope returns an error whose `.Error()` contains the canonical literal `"signature verification FAILED"`.
    - TestVerifyArchiveWrongTrustRoot: when the override trust root is unrelated to the bundle's issuer chain, returns an error with the canonical literal.
    - TestVerifyArchiveMissingArchive: archive file absent → canonical literal.
    - TestVerifyArchiveMissingBundle: bundle file absent → canonical literal.
    - TestVerifyArchiveMalformedTrustRoot: override trust root is `[]byte("not-json")` → canonical literal.
    - **NEW** TestVerifyArchiveWrongIdentity: bundle is otherwise valid but its certificate's SAN points at a DIFFERENT GITHUB ORG path (e.g., `…/some-other-org/helix/.github/workflows/release.yml@refs/tags/v1.0.0`) → canonical literal. Maps to T-58-03. **The fork-SAN posture is critical**: a SAN like `…/agenthands/helix/.github/workflows/release.yml@refs/tags/v1.0.0-experiment` would match the broader regex and ERRONEOUSLY pass — use a different-org SAN to guarantee the negative test exercises the org-pinning portion of the regex, not just a tag-shape rejection.
    - **NEW** TestVerifyArchiveWrongIssuer: bundle is otherwise valid but signed by a non-`token.actions.githubusercontent.com` issuer → canonical literal. Maps to T-58-03.
    - TestRekorUnreachable: VerifyArchive called with the Rekor URL pointed at a non-routable address (e.g., `127.0.0.1:1`) returns an error whose `.Error()` contains BOTH `"signature verification FAILED"` AND `"Rekor transparency-log verification requires network access"` (D-04 user-friendly wording from CONTEXT §Specifics).
  </behavior>
  <action>
    1. Create `internal/upgrade/testdata/generate_fixtures.go` (build-tag `//go:build ignore`) using sigstore-go's signing API to generate, on demand:
       - `internal/upgrade/testdata/trusted_root.json` (test trust root with ephemeral Fulcio + Rekor + TSA fakes — DISTINCT from the production `internal/upgrade/trusted_root.json`)
       - `internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json` (bundle for the existing `sample-archive.tar.gz` fixture, signed against tag `v1.10.0`)
       - **NEW** `internal/upgrade/testdata/sample-archive.tar.gz.rc.sigstore.json` (bundle signed against tag `v1.10.0-rc1` for the RC-tag regression test) — generated alongside the canonical bundle in this script.
       - Adversarial bundles for wrong-identity (different-org SAN) and wrong-issuer cases generated in `t.TempDir()` rather than checked in (matches existing testify convention from PATTERNS — "Tampered-fixture tests build the tampered file in t.TempDir() rather than checking it in").
    2. Run the script once to produce committed fixtures:
       ```bash
       go run -tags ignore ./internal/upgrade/testdata/generate_fixtures.go
       ```
    3. Rewrite `internal/upgrade/verify_test.go` per PATTERNS test-case map:
       - Top-of-file `const` block: keep `canonicalErrText = "signature verification FAILED"`; replace minisign-path consts with `testdataTrustRoot`, `testdataBundle`, `testdataRCBundle`.
       - Helper rename: `withTestKey(t)` → `withTestTrustRoot(t)` reading `testdata/trusted_root.json` and setting `testTrustedRootOverride`. Preserve `t.Helper()` + `t.Cleanup` posture.
       - Replace 6 cases per the PATTERNS rename table; ADD `TestVerifyArchiveAcceptsRCTag`, `TestVerifyArchiveWrongIdentity`, `TestVerifyArchiveWrongIssuer`; ADD `TestRekorUnreachable`.
       - For `TestVerifyArchiveAcceptsRCTag`: table-driven sub-tests covering `-rc1`, `-rc12`, `-beta2`, `-alpha3` SAN suffixes. Each sub-test asserts `err == nil`.
       - For `TestVerifyArchiveWrongIdentity`: use a fork-SAN fixture (different GitHub org) so the broader regex genuinely rejects it. Document the fixture posture in a code comment so future maintainers don't naively swap in a different-tag-shape SAN.
       - Per-case failure assertion uses the existing pattern from PATTERNS lines 215-219 verbatim.
       - For `TestRekorUnreachable`, additionally assert `strings.Contains(err.Error(), "Rekor transparency-log verification requires network access")`.
       - Stay stdlib-only (`t.Fatalf` + `strings.Contains`); do NOT introduce testify here (parity with the Pitfall-4 grep gate per PATTERNS).
    4. **At this stage tests MUST FAIL** because `verify.go` is still the minisign implementation. Run the suite to confirm RED:
       ```bash
       go test ./internal/upgrade/... -count=1 -run 'TestVerifyArchive|TestRekorUnreachable' 2>&1 | tee /tmp/58-02-red.log
       ```
       Compile failure is acceptable as RED state — record the exact compile error in /tmp/58-02-red.log.
    5. Commit shape: `test(58-02): add failing tests for cosign-go verifier (RED)` covering the test file + script + JSON fixtures.
  </action>
  <verify>
    <automated>! go test ./internal/upgrade/... -count=1 -run 'TestVerifyArchive|TestRekorUnreachable' 2&gt;&amp;1 | tee /tmp/58-02-red.log; grep -qE '(FAIL|cannot find|undefined)' /tmp/58-02-red.log</automated>
  </verify>
  <acceptance_criteria>
    - `internal/upgrade/testdata/generate_fixtures.go` exists with `//go:build ignore` (excluded from production build)
    - `internal/upgrade/testdata/trusted_root.json` exists and is valid JSON (parses with `python3 -m json.tool` or `jq .`)
    - `internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json` exists and parses as JSON
    - `internal/upgrade/testdata/sample-archive.tar.gz.rc.sigstore.json` exists and parses as JSON (signed against `v1.10.0-rc1` tag)
    - `internal/upgrade/verify_test.go` contains all 10 test names (`TestVerifyArchiveHappyPath`, `TestVerifyArchiveAcceptsRCTag`, `TestVerifyArchiveTamperedBundle`, `TestVerifyArchiveWrongTrustRoot`, `TestVerifyArchiveMissingArchive`, `TestVerifyArchiveMissingBundle`, `TestVerifyArchiveMalformedTrustRoot`, `TestVerifyArchiveWrongIdentity`, `TestVerifyArchiveWrongIssuer`, `TestRekorUnreachable`)
    - `grep -c canonicalErrText internal/upgrade/verify_test.go` returns at least 9 (one assertion per failure case; happy-path + RC-tag acceptance test do NOT assert the literal)
    - The wrong-identity test uses a different-org SAN (grep `internal/upgrade/verify_test.go` for `some-other-org` or equivalent fork-org marker)
    - `go test ./internal/upgrade/... -count=1` is RED (compile error or test failure recorded in /tmp/58-02-red.log)
    - The RED-failure grep gate matches at least one of: `FAIL`, `cannot find`, `undefined` (count assertion not strict; presence is sufficient given compile-failure as legitimate RED)
    - No `testify` import added to `verify_test.go`
  </acceptance_criteria>
  <done>
    Tests exist, fixtures generated, RED state confirmed in /tmp/58-02-red.log. Commit `test(58-02): ... (RED)` lands.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: GREEN — implement cosign verifier; rename pubkey.go → trustroot.go; update upgrade.go asset names; tidy go.mod</name>
  <files>go.mod, go.sum, internal/upgrade/verify.go, internal/upgrade/upgrade.go, internal/upgrade/pubkey.go, internal/upgrade/trustroot.go, internal/upgrade/trusted_root.json</files>
  <read_first>
    - internal/upgrade/verify.go (current full file — Pitfall-4 invariant comment block, per-failure-site error pattern, testPubKeyOverride pattern)
    - internal/upgrade/pubkey.go (current full file — `//go:embed minisign.pub` + `placeholderMarker` + `IsPlaceholderPubKey()` to be removed)
    - internal/upgrade/upgrade.go lines 122-133 (`IsPlaceholderPubKey` call site to delete) and 188-262 (asset-name + verify-and-keep-stage patterns)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`internal/upgrade/verify.go`"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`internal/upgrade/pubkey.go` → `internal/upgrade/trustroot.go`" (R-3 ordering hazard)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`internal/upgrade/upgrade.go`"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`go.mod` / `go.sum`"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md line 440 (broader SAN regex literal)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md §"Rekor online behavior" assumption A2 (Q-1 default: SET-based offline check sufficient; leave TODO comment)
  </read_first>
  <behavior>
    - VerifyArchive uses sigstore-go's `bundle.Bundle`, `root.NewTrustedRootFromJSON`, and `verify.NewSignedEntityVerifier` to verify a cosign-signed archive bundle.
    - Identity policy pins both OIDC issuer and a SAN regex (broader form per RESEARCH line 440, accepting RC/beta/alpha tags).
    - Rekor-unreachable failures return a wrapped error containing both the canonical literal AND the user-friendly Rekor wording (D-04).
    - Every failure branch returns the canonical literal `"signature verification FAILED"` with `// canonical: ...` inline comment immediately above each `errors.New` AND `fmt.Errorf("signature verification FAILED…")` site (Pitfall-4 invariant strengthened).
    - upgrade.go fetches `${archive}.sigstore.json` instead of `${archive}.minisig`.
    - go.mod no longer references `jedisct1/go-minisign`; carries `sigstore/sigstore-go v1.1.x`.
    - Production trust root file (`internal/upgrade/trusted_root.json`) is byte-distinct from the test trust root (`internal/upgrade/testdata/trusted_root.json`) — verified by `! cmp -s`.
  </behavior>
  <action>
    Implement in this order (R-3 ordering hazard from PATTERNS — embed swap and minisign.pub deletion must coordinate):

    1. **Rename + re-purpose `pubkey.go` → `trustroot.go`** (in the SAME commit that ships `trusted_root.json`):
       - `git mv internal/upgrade/pubkey.go internal/upgrade/trustroot.go`
       - Replace contents with the PATTERNS-prescribed shape:
         ```go
         package upgrade

         import _ "embed"

         // trustedRootJSON is the embedded sigstore TUF trusted root. The byte
         // content is a snapshot of the public-good Sigstore trust root taken at
         // build time; refresh per CONTRIBUTING.md §"Trust root refresh" before
         // each minor release. Consumed by verify.go via root.NewTrustedRootFromJSON.
         //
         //go:embed trusted_root.json
         var trustedRootJSON []byte
         ```
       - Delete `placeholderMarker` const, `IsPlaceholderPubKey` function, and the `bytes` import.
    2. **Ship the production trust root** at `internal/upgrade/trusted_root.json`. Source: pinned snapshot of `https://github.com/sigstore/sigstore-go/blob/main/examples/trusted-root-public-good.json` (per RESEARCH §"Trust root sourcing"; Q-2 default — embed). Commit two distinct copies (production at `internal/upgrade/trusted_root.json`, test override at `internal/upgrade/testdata/trusted_root.json` which Task 1 already shipped) so the test override path stays distinct. **They MUST differ byte-for-byte** (mixing them would erase identity-pinning enforcement at production).
    3. **Rewrite `internal/upgrade/verify.go`** with the sigstore-go API:
       - Replace import `github.com/jedisct1/go-minisign` with the three sigstore-go imports listed in `<interfaces>`.
       - Function signature unchanged: `func VerifyArchive(archivePath, bundlePath string) error`.
       - Implementation outline:
         ```go
         func VerifyArchive(archivePath, bundlePath string) error {
             // Phase 58 D-02: cosign keyless verification via sigstore-go.

             archiveBytes, err := os.ReadFile(archivePath)
             if err != nil {
                 // canonical: signature verification FAILED — see Pitfall 4.
                 return errors.New("signature verification FAILED")
             }
             bundleBytes, err := os.ReadFile(bundlePath)
             if err != nil {
                 // canonical: signature verification FAILED — see Pitfall 4.
                 return errors.New("signature verification FAILED")
             }

             tr, err := root.NewTrustedRootFromJSON(currentTrustedRoot())
             if err != nil {
                 // canonical: signature verification FAILED — see Pitfall 4.
                 return errors.New("signature verification FAILED")
             }

             b := &bundle.Bundle{}
             if err := b.UnmarshalJSON(bundleBytes); err != nil {
                 // canonical: signature verification FAILED — see Pitfall 4.
                 return errors.New("signature verification FAILED")
             }

             // Phase 58 D-02 / R-2: pin OIDC issuer + SAN regex.
             // TODO(maintainer): for strict tlog-tree-head freshness (Q-1 in
             // 58-RESEARCH.md), call tr.RekorLogs() and assert the bundle's
             // inclusion proof references a tlog tree-head no older than N
             // hours. Default behavior (SET-based offline check) is sufficient
             // per Q-1 default.
             v, err := verify.NewSignedEntityVerifier(tr,
                 verify.WithSignedCertificateTimestamps(1),
                 verify.WithObserverTimestamps(1),
                 verify.WithTransparencyLog(1),
             )
             if err != nil {
                 // canonical: signature verification FAILED — see Pitfall 4.
                 return errors.New("signature verification FAILED")
             }

             policy := verify.NewPolicy(
                 verify.WithArtifact(bytes.NewReader(archiveBytes)),
                 verify.WithCertificateIdentity(verify.NewCertificateIdentity(
                     pinnedSANRegex,
                     pinnedOIDCIssuer,
                 )),
             )

             if _, err := v.Verify(b, policy); err != nil {
                 if isRekorUnreachable(err) {
                     // canonical: signature verification FAILED — see Pitfall 4.
                     return fmt.Errorf("signature verification FAILED: Rekor transparency-log verification requires network access: %w", err)
                 }
                 // canonical: signature verification FAILED — see Pitfall 4.
                 return errors.New("signature verification FAILED")
             }
             return nil
         }
         ```
       - Define package-level constants:
         ```go
         const (
             pinnedOIDCIssuer = "https://token.actions.githubusercontent.com"
             // Phase 58 D-02 / R-2: SAN regex pinned to release.yml on a semver
             // tag with optional -rc/-beta/-alpha pre-release suffix (per
             // goreleaser `prerelease: auto` in .goreleaser.yaml:94 and
             // 58-RESEARCH.md line 139/440).
             pinnedSANRegexLiteral = `^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$`
         )
         var pinnedSANRegex = regexp.MustCompile(pinnedSANRegexLiteral)
         ```
       - Define `currentTrustedRoot()` and `testTrustedRootOverride []byte` mirroring the OLD `currentPubKey()` pattern (PATTERNS §"Test override pattern to PRESERVE"). Package-private.
       - Define `isRekorUnreachable(err error) bool` — best-effort substring-or-errors.As classifier (look for `*net.OpError` / `connection refused` / `no such host` in the error chain).
       - Preserve the Pitfall-4 invariant comment block at the top of the file (replace "minisign" with "sigstore" in the explanatory text but KEEP the literal canonical string as `"signature verification FAILED"`).
       - Use `errors.New(...)` (NOT `fmt.Errorf` with `%w`) at every literal-only canonical return; use `fmt.Errorf` ONLY for the Rekor branch where additional structured wording is mandated by D-04. Both `errors.New("signature verification FAILED")` AND `fmt.Errorf("signature verification FAILED…")` callsites MUST be preceded by the `// canonical: signature verification FAILED — see Pitfall 4.` comment line (Pitfall-4 strengthening).
    4. **Update `internal/upgrade/upgrade.go`** (mechanical string changes only):
       - Line 188-189: `sigName := archiveName + ".minisig"` → `bundleName := archiveName + ".sigstore.json"`. Update all downstream variable names (`stageSig` → `stageBundle`).
       - Line 201-224: replace `"checksums.txt.minisig"` → `"checksums.txt.sigstore.json"` at both string literal sites. Keep guard structure verbatim.
       - Line 258-262: comment update `"Step 6: minisign verify ..."` → `"Step 6: cosign verify the archive bundle."`. Function signature of VerifyArchive unchanged.
       - Lines 122-133: DELETE the `IsPlaceholderPubKey()` call site entirely.
    5. **`go mod tidy`** — removes `github.com/jedisct1/go-minisign`, adds `github.com/sigstore/sigstore-go v1.1.4` (or the latest v1.1.x available; pin to a specific minor version per RESEARCH R-8).
    6. Run the test suite. All 10 tests MUST pass:
       ```bash
       go test ./internal/upgrade/... -count=1 2>&1 | tee /tmp/58-02-green.log
       ```
    7. Commit shape: `feat(58-02): implement cosign-go verifier and asset-name swap (GREEN)`.

    **Do NOT delete the old minisign artifacts in this commit** — that is Task 4. The build must continue to pass with the OLD pub-key files still on disk.

    **R-3 hazard reminder:** `//go:embed minisign.pub` is removed in this commit (replaced by `//go:embed trusted_root.json`); the actual `minisign.pub` files stay on disk until Task 4. Build will succeed because no `//go:embed` directive references them after this task.
  </action>
  <verify>
    <automated>go test ./internal/upgrade/... -count=1 2&gt;&amp;1 | tee /tmp/58-02-green.log &amp;&amp; grep -qE 'PASS|ok\s+.*internal/upgrade' /tmp/58-02-green.log &amp;&amp; ! grep -q 'FAIL' /tmp/58-02-green.log &amp;&amp; go vet ./internal/upgrade/... &amp;&amp; ! grep -q 'jedisct1/go-minisign' go.mod &amp;&amp; grep -q 'sigstore/sigstore-go' go.mod &amp;&amp; ! cmp -s internal/upgrade/trusted_root.json internal/upgrade/testdata/trusted_root.json &amp;&amp; grep -B1 'fmt.Errorf("signature verification FAILED' internal/upgrade/verify.go | grep -q 'canonical: signature verification FAILED'</automated>
  </verify>
  <acceptance_criteria>
    - All 10 tests in `internal/upgrade/` pass (`go test ./internal/upgrade/... -count=1` returns ok)
    - `go vet ./internal/upgrade/...` is clean
    - `internal/upgrade/trustroot.go` exists; `internal/upgrade/pubkey.go` does NOT exist
    - `grep -q 'go:embed trusted_root.json' internal/upgrade/trustroot.go` returns true
    - No remaining `//go:embed minisign.pub` directive anywhere under `internal/upgrade/`
    - **Production trust root differs from test trust root:** `! cmp -s internal/upgrade/trusted_root.json internal/upgrade/testdata/trusted_root.json` succeeds (mixing them would disable identity pinning).
    - Pitfall-4 grep gate (comment-stripped): `grep -v '^[[:space:]]*//' internal/upgrade/verify.go | grep -c 'signature verification FAILED'` returns at least 5
    - `grep -c 'canonical: signature verification FAILED' internal/upgrade/verify.go` returns at least 5 (one comment per failure branch)
    - **Pitfall-4 strengthening:** every `fmt.Errorf("signature verification FAILED…")` callsite carries the `// canonical: signature verification FAILED` comment immediately above it: `grep -B1 'fmt.Errorf("signature verification FAILED' internal/upgrade/verify.go | grep -q 'canonical: signature verification FAILED'`
    - `grep -q 'sigstore/sigstore-go' go.mod` returns true; `grep -q 'jedisct1/go-minisign' go.mod` returns false
    - `internal/upgrade/upgrade.go` has zero hits for `.minisig` (`grep -c '.minisig' internal/upgrade/upgrade.go` returns 0)
    - The pinned SAN regex literal matches the RESEARCH-default value (broader form) exactly: `^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$`
    - The pinned OIDC issuer literal is `https://token.actions.githubusercontent.com`
    - `TestVerifyArchiveAcceptsRCTag` passes (broader-regex regression guard)
  </acceptance_criteria>
  <done>
    Verifier rewritten, tests GREEN, vet clean. go.mod tidied. The old minisign.pub files still exist on disk (deletion is Task 4's responsibility).
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: REFACTOR — extract identity matcher into helper for testability</name>
  <files>internal/upgrade/verify.go, internal/upgrade/verify_test.go</files>
  <read_first>
    - internal/upgrade/verify.go (post-Task-2 state)
    - internal/upgrade/verify_test.go (post-Task-1 state)
  </read_first>
  <behavior>
    - The pinned-identity check (issuer + SAN regex) factored into a small helper `matchesPinnedIdentity(cert *x509.Certificate) bool` (or equivalent).
    - Two new direct unit tests for the helper: `TestMatchesPinnedIdentity_HappyPath`, `TestMatchesPinnedIdentity_RejectsWrongRepoSAN`.
    - All 12 tests pass. No behavior change visible to `VerifyArchive` callers.
  </behavior>
  <action>
    1. Refactor: extract the `verify.NewCertificateIdentity(...)` construction (and any pre-validation around the SAN regex compile / issuer comparison) into a package-private helper. Two minimal options:
       - Option A: pure helper that operates on `*x509.Certificate` directly (best for unit testing).
       - Option B: wrapper helper returning a `verify.Policy` with the identity already attached.

       Choose Option A unless sigstore-go's API surface makes it impractical (in which case Option B; record the choice in the SUMMARY).
    2. Add two helper-targeted unit tests in `verify_test.go`. Use `crypto/x509` to construct a cert with the matching SAN (happy) and a non-matching SAN (reject). Stay stdlib-only.
    3. Run the full suite. All tests PASS:
       ```bash
       go test ./internal/upgrade/... -count=1 -v 2>&1 | tee /tmp/58-02-refactor.log
       ```
    4. Commit shape: `refactor(58-02): extract identity matcher for testability`.
  </action>
  <verify>
    <automated>go test ./internal/upgrade/... -count=1 -v 2&gt;&amp;1 | tee /tmp/58-02-refactor.log &amp;&amp; grep -q 'TestMatchesPinnedIdentity_HappyPath' /tmp/58-02-refactor.log &amp;&amp; grep -q 'TestMatchesPinnedIdentity_RejectsWrongRepoSAN' /tmp/58-02-refactor.log &amp;&amp; ! grep -q 'FAIL' /tmp/58-02-refactor.log &amp;&amp; go vet ./internal/upgrade/...</automated>
  </verify>
  <acceptance_criteria>
    - All 12 tests pass (10 from Task 1 + 2 helper tests)
    - `internal/upgrade/verify.go` has a private helper named `matchesPinnedIdentity` (or equivalent recorded in SUMMARY)
    - `go vet ./internal/upgrade/...` clean
    - No `VerifyArchive` signature change
    - Pitfall-4 grep gate still passes: `grep -v '^[[:space:]]*//' internal/upgrade/verify.go | grep -c 'signature verification FAILED'` returns at least 5
  </acceptance_criteria>
  <done>
    Refactor commit landed; suite green; helper has its own coverage.
  </done>
</task>

<task type="auto">
  <name>Task 4: Delete minisign artifacts; update Makefile, CONTRIBUTING.md, INSTALL.md</name>
  <files>minisign.pub, internal/upgrade/minisign.pub, internal/upgrade/testdata/test_keypair.pub, internal/upgrade/testdata/test_keypair.key, internal/upgrade/testdata/sample-archive.tar.gz.minisig, Makefile, CONTRIBUTING.md, INSTALL.md</files>
  <read_first>
    - Makefile (current — lines 1, 6, 61-66 per PATTERNS)
    - CONTRIBUTING.md lines 159-214 per PATTERNS
    - INSTALL.md if present (root); otherwise plan to add the recipe to README.md §Installation per RESEARCH Q-3
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`Makefile`" (target deletion mechanics)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`CONTRIBUTING.md`"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md Q-2 default (`update-trust-root` Makefile target)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md Q-3 default (INSTALL.md recipe)
  </read_first>
  <action>
    **Preflight:** Check whether `INSTALL.md` exists at the repo root. If it does NOT exist, the recipe lands in `README.md` §Installation instead. Update plan frontmatter `files_modified` accordingly during execution (replace `INSTALL.md` with `README.md`). Do NOT create a new INSTALL.md from scratch.

    1. **Delete minisign artifacts** (R-3 ordering hazard — safe to delete now because Task 2 already removed the `//go:embed minisign.pub` directive):
       ```bash
       git rm minisign.pub
       git rm internal/upgrade/minisign.pub
       git rm internal/upgrade/testdata/test_keypair.pub
       git rm internal/upgrade/testdata/test_keypair.key
       git rm internal/upgrade/testdata/sample-archive.tar.gz.minisig
       ```
    2. **Edit Makefile** per PATTERNS §"`Makefile`":
       - Remove `embed-pubkey` and `verify-embed-pubkey` from the `.PHONY:` line
       - Remove the `embed-pubkey` dep edge from `build:`
       - Delete the entire `embed-pubkey:` and `verify-embed-pubkey:` target blocks
       - Add a new `update-trust-root` target (per RESEARCH Q-2 default):
         ```makefile
         update-trust-root: ## Refresh internal/upgrade/trusted_root.json from sigstore upstream
         	@curl -sSL https://raw.githubusercontent.com/sigstore/sigstore-go/main/examples/trusted-root-public-good.json -o internal/upgrade/trusted_root.json
         	@echo "trusted_root.json refreshed; commit and bump per CONTRIBUTING.md"
         ```
       - Add `update-trust-root` to `.PHONY`. Use TAB indentation for the recipe (Makefile-required).
    3. **Edit CONTRIBUTING.md** per PATTERNS §"`CONTRIBUTING.md`":
       - Delete lines 180-187 (Repository secrets — `MINISIGN_PRIVATE_KEY` / `MINISIGN_PASSWORD`)
       - Delete lines 189-203 (One-time keypair setup)
       - Delete or replace lines 205-207 (Key rotation): replace with a one-line "no key rotation needed" note for cosign keyless.
       - Add a new H3 `### Repository secrets` paragraph stating no long-lived secrets, with `id-token: write` mention.
       - Add a new H3 `### Trust root refresh` paragraph (per PATTERNS sample text), referencing `make update-trust-root` and a refresh cadence ("before each minor release").
       - Preserve the existing `## Releasing` H2 anchor and the surrounding sh code-fence convention.
       - Note: REL-05 "Pass-3 limitation" wording is added by Plan 04, not this plan. Do NOT touch that section here.
    4. **Update INSTALL.md** (or, if no `INSTALL.md` at the repo root, README.md §Installation). Add a `### Verifying release artifacts` subsection with the user-facing recipe (per RESEARCH Q-3 default). The `--certificate-identity-regexp` MUST match the broader form used in `verify.go` exactly:
       ```sh
       cosign verify-blob \
         --bundle helix_v1.10.0_linux_amd64.tar.gz.sigstore.json \
         --certificate-identity-regexp '^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$' \
         --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
         helix_v1.10.0_linux_amd64.tar.gz
       ```
       Use the same regex + issuer values as the verifier's pinned policy (parity is the security invariant — applies to both final and pre-release tags).
    5. **Final consistency check**:
       ```bash
       go build ./... && go test ./internal/upgrade/... -count=1
       ```
    6. Commit shape: `chore(58-02): delete minisign artifacts; refresh docs and Makefile`.
  </action>
  <verify>
    <automated>! test -f minisign.pub &amp;&amp; ! test -f internal/upgrade/minisign.pub &amp;&amp; ! test -f internal/upgrade/testdata/test_keypair.pub &amp;&amp; ! test -f internal/upgrade/testdata/test_keypair.key &amp;&amp; ! test -f internal/upgrade/testdata/sample-archive.tar.gz.minisig &amp;&amp; ! grep -q 'embed-pubkey' Makefile &amp;&amp; grep -q 'update-trust-root' Makefile &amp;&amp; grep -q 'Trust root refresh' CONTRIBUTING.md &amp;&amp; ! grep -q 'MINISIGN_PRIVATE_KEY' CONTRIBUTING.md &amp;&amp; (grep -q 'cosign verify-blob' INSTALL.md 2&gt;/dev/null || grep -q 'cosign verify-blob' README.md) &amp;&amp; (grep -q '\-rc\\\\d+|-beta\\\\d+|-alpha\\\\d+' INSTALL.md 2&gt;/dev/null || grep -q '\-rc\\\\d+|-beta\\\\d+|-alpha\\\\d+' README.md) &amp;&amp; go build ./... &amp;&amp; go test ./internal/upgrade/... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - All five minisign artifact files are deleted (`test ! -f` passes for each)
    - `Makefile` has zero hits for `embed-pubkey` (`grep -c embed-pubkey Makefile` returns 0)
    - `Makefile` has the `update-trust-root` target (`grep -q 'update-trust-root' Makefile`)
    - `CONTRIBUTING.md` has zero hits for `MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD`, and `minisign` (`grep -ic 'minisign\|MINISIGN_' CONTRIBUTING.md` returns 0)
    - `CONTRIBUTING.md` contains the literal string `Trust root refresh`
    - The user-facing cosign recipe lives in `INSTALL.md` OR `README.md` (whichever exists) and matches the regex/issuer values used in `verify.go` (broader form including `-rc\d+|-beta\d+|-alpha\d+` suffix)
    - `go build ./...` succeeds; `go test ./internal/upgrade/... -count=1` is green
  </acceptance_criteria>
  <done>
    All minisign residue erased; docs refreshed; Makefile updated; final build + test pass.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Embedded trusted root → verifier process | Trust root JSON is the pinning anchor — any compromise lets an attacker swap signing identities |
| Network → verifier (Rekor + Fulcio cert chain) | Verifier must require Rekor inclusion proof; mTLS endpoints carry transparency-log evidence |
| Bundle file → cert identity check | Identity policy (issuer + SAN regex) is the gate that rejects same-issuer-but-wrong-repo signatures |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-58-02 | Spoofing | VerifyArchive accepts wrong-key signature | mitigate | sigstore-go verifier validates DSSE signature against Fulcio short-lived cert chain rooted in embedded trusted root; tampered-bundle and wrong-trust-root unit tests enforce |
| T-58-03 | Spoofing | VerifyArchive accepts attacker-controlled cert with valid Fulcio chain | mitigate | Pinned SAN regex (broader form `^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$`) + pinned OIDC issuer (`https://token.actions.githubusercontent.com`); three NEW unit tests `TestVerifyArchiveAcceptsRCTag`, `TestVerifyArchiveWrongIdentity` (fork-org SAN), and `TestVerifyArchiveWrongIssuer` guard regression |
| T-58-04 | Information Disclosure / Denial of Service | Rekor unreachable masked as success | mitigate | D-04 default: SET-based offline check requires `WithTransparencyLog(1)` minimum; `TestRekorUnreachable` asserts both canonical literal AND user-friendly error wording |
| T-58-02-aux | Tampering | Local pub-key drift between repo root and embedded copy | accept (eliminated by design) | The `embed-pubkey` / `verify-embed-pubkey` Makefile targets and the dual-pub-key system are deleted; only one trust root file is embedded, sourced from upstream sigstore TUF |
| T-58-03-aux | Spoofing | go-minisign supply-chain compromise | mitigate | go-minisign dependency removed entirely; sigstore-go is pinned to a specific v1.1.x minor version (RESEARCH R-8) |
| T-58-03-mix | Spoofing | Production trust root accidentally replaced with the test trust root (which trusts an ephemeral Fulcio fake) | mitigate | Acceptance criterion `! cmp -s internal/upgrade/trusted_root.json internal/upgrade/testdata/trusted_root.json` ensures the two files are byte-distinct; CI fails if a maintainer mistakenly copies one over the other |
</threat_model>

<verification>
- All 12 tests pass (`go test ./internal/upgrade/... -count=1`)
- `go vet ./internal/upgrade/...` clean
- `go build ./...` succeeds with no embedded `minisign.pub` references
- Pitfall-4 grep gate (comment-stripped, see acceptance criteria) holds
- Pitfall-4 strengthening: every `fmt.Errorf("signature verification FAILED…")` carries the `// canonical:` comment immediately above it
- Production trust root is byte-distinct from the test trust root
- No minisign residue in repo (binary files, Go imports, Makefile targets, docs)
- INSTALL.md / README.md user-facing cosign recipe matches verifier's pinned identity policy (broader regex form)
- `TestVerifyArchiveAcceptsRCTag` enforces RC/beta/alpha pre-release tag acceptance (regression guard against regex over-tightening)
</verification>

<success_criteria>
- Verifier rewrite is the GREEN-test gate; tests pass on a clean checkout
- Asset-name swap in `upgrade.go` is mechanical and verified by zero `.minisig` grep hits
- All minisign artifacts (binaries, deps, Makefile targets, docs) are erased
- The pinned identity policy is byte-identical between `verify.go`, the user-facing INSTALL recipe, and the threat-register entry (security-invariant parity across all three surfaces)
- A signed pre-release (`v1.10.0-rc1`) self-upgrade succeeds end-to-end (`TestVerifyArchiveAcceptsRCTag` is the unit-level gate)
- Plan 04 (won't-do recording + REL-05 doc) can land independently of this plan; the only inter-plan touchpoint is CONTRIBUTING.md, where Plan 04's "Pass-3 limitation" addition is in a separate section from Plan 02's "Trust root refresh" addition
</success_criteria>

<output>
After completion, create `.planning/phases/58-v1-9-carryover-release-distribution/58-02-SUMMARY.md` documenting:
- Final sigstore-go version pinned in go.mod
- The exact 40-char SHA / version trail of any external trusted-root snapshot used
- The chosen refactor option (A or B from Task 3)
- Confirmation of Pitfall-4 grep-gate count (`signature verification FAILED` literal occurrences in verify.go)
- Confirmation that production and test trust roots are byte-distinct
- Whether INSTALL.md or README.md hosts the user-facing recipe (preflight check result)
</output>
