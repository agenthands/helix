# Phase 58 Research: v1.9 Carryover — Release & Distribution

**Researched:** 2026-05-03
**Domain:** release signing (sigstore/cosign keyless), self-upgrade verifier rewrite, OTel gRPC trace propagation
**Confidence:** HIGH (release-side YAML, codebase blast radius, otelgrpc handler shape) / MEDIUM (sigstore-go vs cosign-go library choice — see Risk R-1)

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01** Drop REL-02/03/04 entirely. Helix ships exclusively as a single self-contained signed binary.
- **D-02** Replace minisign with sigstore cosign keyless signing. CI uses GitHub Actions OIDC → Fulcio short-lived cert → `cosign sign-blob` → Rekor transparency-log entry.
- **D-03** Hard cut at v1.10.0 — cosign-only, no minisign coexistence. v1.9.0 was never published with a real signature, so no shipped v1.9.x binary loses its self-upgrade path.
- **D-04** `helix upgrade` requires online Rekor for transparency-log inclusion-proof verification. If Rekor is unreachable, return a structured error explaining the network requirement; do not fall back to weaker verification.
- **D-05** Document the Pass-3 reproducibility-gate limitation in `CONTRIBUTING.md`. No real-artifact comparison job.
- **D-06** Add `otelgrpc.NewClientHandler` to the forwarder's gRPC dial; emit a real `forwarder.tools.call` client span. End-to-end trace assertion via in-memory exporter.

### Claude's Discretion
- Whether the cosign migration is one plan or two (release-side YAML swap + verifier rewrite vs combined). See §"Plan-splitting recommendation" below.
- Test strategy for Rekor-online: real Rekor calls in integration tests vs mocked. Default to mocked unless real-call is genuinely cheap.
- Whether `won't-do` recording for REL-02/03/04 lands as its own plan or as a tail-end task in the cosign plan.

### Deferred Ideas (OUT OF SCOPE)
- Real-artifact reproducibility comparison job (v1.11+).
- Backporting cosign verification to v1.9.x (rejected per D-03).
- Distribution channels (Homebrew/Scoop/Linux pkg) — rejected per D-01.
- Cached Rekor inclusion proofs for offline upgrade (rejected per D-04).
- PROJECT.md "Resolved at v1.10" section move (post-phase, not in plans).

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REL-01 | First signed v1.10.0 release via cosign keyless; CI rejects PLACEHOLDER (now obsolete); `helix upgrade` verifies cosign sig + Rekor inclusion + atomically swaps. | §REL-01 covers verifier API, identity pinning, Rekor behavior, goreleaser block, release.yml additions, internal/upgrade/ blast radius. |
| REL-05 | Reproducibility gate Pass-3 limitation documented in `CONTRIBUTING.md`. | §REL-05 covers existing Releasing prose anchor and recommended placement. |
| REL-06 | `forwarder.tools.call` span unified with gRPC server span; verified by end-to-end trace assertion. | §REL-06 covers the existing dial site, the missing piece (propagator + non-Noop tracer), server-handler verification, and an in-memory test pattern that's already in the repo. |
| (won't-do recording) | REL-02/03/04 marked `- [~]` with rationale string in REQUIREMENTS.md; roadmap Phase 58 entry shrinks. | §"REQUIREMENTS.md won't-do recording" |

## Summary

1. **REL-06 is mostly already done.** `otelgrpc.NewClientHandler` is installed at `internal/forwarder/dial.go:70-72`, server-side `otelgrpc.NewServerHandler` at `internal/daemon/daemon.go:573`. The actual gap is **(a)** the forwarder is constructed with `obs.Noop(...)` (`forwarder.go:25`) so its tracer never emits, and **(b)** no `otel.SetTextMapPropagator` is set anywhere — so even when both handlers are wired, traceparent metadata isn't injected/extracted. CONTEXT D-06's "add `NewClientHandler`" framing is mis-targeted; the work is "give the forwarder a real TracerProvider, set a TextMapPropagator, and add an end-to-end assertion."
2. **REL-01 cosign migration has a larger blast radius than the goreleaser swap.** `internal/upgrade/verify.go` has only 1 import of `github.com/jedisct1/go-minisign` (used at 2 call sites), but `upgrade.go:189` builds the `.minisig` URL, the `checksums.txt.minisig` cross-check at lines 196-256 needs to track the new artifact-name shape, and `IsPlaceholderPubKey` / `pubKeyBytes` / the Makefile `embed-pubkey` + `verify-embed-pubkey` gate / the release.yml PLACEHOLDER pre-flight grep / both `minisign.pub` files all become obsolete in lockstep.
3. **`sigstore-go` (v1.1.4, Dec 2025) is the recommended Go library for verifying sigstore bundles** — not `cosign/v2/pkg/cosign`, which exists but exposes a heavier surface oriented at OCI signatures. sigstore-go is "considered stable and ready for production use," requires Go 1.23+, and is what cosign itself uses internally for bundle verification. Pin the verifier rewrite to `github.com/sigstore/sigstore-go` v1.1.x.
4. **Goreleaser cosign output is a single `.sigstore.json` bundle**, not separate `.sig` + `.pem`. This simplifies the verifier (one file fetch + one bundle verifier call) and matches cosign's recommended modern flow. The goreleaser `signs:` block is a 5-line YAML change.
5. **Identity pinning is the critical security control.** Without `--certificate-identity[-regexp]` + `--certificate-oidc-issuer`, any GitHub Actions workflow's signature verifies, including a malicious fork's. Verifier MUST pin issuer to `https://token.actions.githubusercontent.com` and identity to a regex matching `https://github.com/agenthands/helix/.github/workflows/release.yml@refs/tags/v*`.

**Primary recommendation:** Two execution plans for REL-01 (release-side YAML+CI swap before any verifier rewrite ships, so the verifier is never asked to verify minisign artifacts that no longer exist), one plan for REL-06 (focused: fix propagator + tracer wiring + in-memory test), one plan for REL-05+won't-do recording (pure docs). Total: 4 plans.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Sign release archives | CI / GitHub Actions OIDC | goreleaser subprocess | OIDC token only exists in CI; goreleaser invokes `cosign sign-blob` with the runner's id-token. |
| Verify downloaded archive at runtime | Application binary (`internal/upgrade/`) | Network (Rekor public-good) | Self-upgrade is in-process; Rekor inclusion proof is online-only per D-04. |
| Embed trust root for verification | Build-time embed (`go:embed` of TUF root or vendored Fulcio root) | sigstore-go TUF client | Single binary distribution per D-01 — no runtime config files. |
| Forwarder→daemon trace continuity | gRPC metadata (otelgrpc handlers + W3C TraceContext propagator) | OTel SDK TracerProvider in each process | Forwarder is a separate process; only gRPC metadata bridges them. |
| Reproducibility gate | CI (release.yml) | none | Already lives in `release.yml`; no runtime impact. |

## REL-01: cosign keyless migration

### Verifier API surface (sigstore-go)

**Library choice (HIGH).** Use `github.com/sigstore/sigstore-go` v1.1.x ([sigstore-go README](https://github.com/sigstore/sigstore-go), v1.1.4 released 2025-12-10, Go 1.23+ floor). It is the upstream-blessed Go API for verifying sigstore bundles and is what `cosign` itself uses internally. The alternative — `github.com/sigstore/cosign/v2/pkg/cosign` — also exposes `VerifyBlobSignature` ([pkg.go.dev cosign](https://pkg.go.dev/github.com/sigstore/cosign/v2/pkg/cosign)) but is OCI-oriented and pulls a much larger dep graph (containerd, OCI registry clients) into the helix binary.

**Required imports** (verbatim from [sigstore-go-verification example](https://raw.githubusercontent.com/sigstore/sigstore-go/main/examples/sigstore-go-verification/main.go)):

```go
import (
    "github.com/sigstore/sigstore-go/pkg/bundle"
    "github.com/sigstore/sigstore-go/pkg/root"
    "github.com/sigstore/sigstore-go/pkg/verify"
    // optional, for fetching trusted_root.json over TUF at runtime:
    "github.com/sigstore/sigstore-go/pkg/tuf"
    "github.com/sigstore/sigstore-go/pkg/util"
    "github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
)
```

**Minimal verify flow** (paraphrasing example main.go lines 90-220 — flow + symbol names are verbatim, error handling collapsed):

```go
// 1. Load the .sigstore.json bundle from disk.
b, err := bundle.LoadJSONFromPath(bundlePath)

// 2. Build verifier config (transparency log + cert-transparency-log + observer-timestamp checks).
verifierConfig := []verify.VerifierOption{
    verify.WithSignedCertificateTimestamps(1),
    verify.WithObserverTimestamps(1),
    verify.WithTransparencyLog(1),  // requires Rekor inclusion proof
}

// 3. Identity policy — PINS the OIDC issuer + cert SAN to GitHub Actions release workflow.
certID, err := verify.NewShortCertificateIdentity(
    "https://token.actions.githubusercontent.com",  // expectedOIDIssuer
    "",                                              // expectedOIDIssuerRegex (not used)
    "",                                              // expectedSAN (use regex below)
    `^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$`,
)
identityPolicies := []verify.PolicyOption{verify.WithCertificateIdentity(certID)}

// 4. Trusted root — embedded JSON at build time (see "Trust root sourcing" below).
trustedRoot, err := root.NewTrustedRootFromJSON(embeddedTrustedRootJSON)
trustedMaterial := root.TrustedMaterialCollection{trustedRoot}

// 5. Verifier.
sev, err := verify.NewVerifier(trustedMaterial, verifierConfig...)

// 6. Artifact policy — pass the actual archive bytes for digest comparison.
file, _ := os.Open(archivePath)
defer file.Close()
artifactPolicy := verify.WithArtifact(file)

// 7. Execute verification.
res, err := sev.Verify(b, verify.NewPolicy(artifactPolicy, identityPolicies...))
```

**Key types** (from [pkg.go.dev cosign CheckOpts](https://pkg.go.dev/github.com/sigstore/cosign/v2/pkg/cosign), reproduced for cosign-go but the sigstore-go shape is parallel):

- `bundle.Bundle` — parsed `.sigstore.json` (signature + cert + Rekor inclusion proof in one).
- `root.TrustedRoot` / `root.TrustedMaterialCollection` — Fulcio + Rekor + CT log root keys.
- `verify.SignedEntityVerifier` (returned by `NewVerifier`) — verification engine.
- `verify.PolicyOption` (e.g. `WithCertificateIdentity`) — identity-pinning closures.
- `verify.ArtifactPolicyOption` — `WithArtifact(io.Reader)` (preferred; recomputes hash) or `WithArtifactDigest(algo, bytes)`.
- `verify.VerificationResult` — returned by `Verify()`; contains cert subject, issuer, Rekor entry.

**Trust root sourcing.** Two options, document tradeoff in plan:

| Option | What | Pro | Con |
|--------|------|-----|-----|
| Embed `trusted_root.json` | `//go:embed trusted_root.json` of [public-good TUF root snapshot](https://github.com/sigstore/sigstore-go/blob/main/examples/trusted-root-public-good.json) | Fully offline-capable trust root; no extra network call beyond Rekor inclusion-proof check | Must refresh during binary builds when Sigstore rotates roots (rare; Fulcio CA has multi-year validity) |
| Fetch via TUF at runtime | `tuf.New(opts)` + `client.GetTarget("trusted_root.json")` | Always-current trust root | Adds a network call beyond Rekor; more failure modes |

Recommend **embed** for v1.10 (single-binary ethos per D-01 + CONTEXT §"Out of scope" rejecting offline-capable Rekor caching specifically because upgrade is online — but trust roots are different and embedding is a self-contained-binary win). The embedded JSON is byte-stable per release, so the reproducibility gate is unaffected.

### Identity pinning

**Critical.** Without identity pinning, any GitHub Actions OIDC token signs a valid Fulcio cert and the verifier passes. The exact strings:

- **OIDC issuer (literal):** `https://token.actions.githubusercontent.com`
- **Certificate SAN regex:** `^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$`

The SAN regex deliberately:
- Anchors `^` and `$` (otherwise `agenthands/helix-malicious-fork/...` matches as a substring).
- Pins repo `agenthands/helix` exactly.
- Pins workflow path `.github/workflows/release.yml` exactly (matches the existing file).
- Allows tag refs `v1.10.0`, `v1.10.0-rc1`, `v1.10.0-beta1`, `v1.10.0-alpha1` per the existing `prerelease: auto` convention in `.goreleaser.yaml:94`.
- Forbids `refs/heads/main` or `refs/heads/*` — only release-tag-triggered workflow runs verify.

CLI equivalent for INSTALL.md verify recipe: `cosign verify-blob <archive> --bundle <archive>.sigstore.json --certificate-identity-regexp "^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v.*" --certificate-oidc-issuer https://token.actions.githubusercontent.com` — see [cosign GH workflow keyless verification example, gh cli precedent](https://docs.sigstore.dev/cosign/verifying/verify/) and [WebSearch result, github-keyless verify pattern].

### Rekor online behavior

`verify.WithTransparencyLog(1)` (sigstore-go) requires at least one Rekor inclusion proof to be present and valid. Behavior on Rekor unreachable:

- The bundle ALREADY contains the Rekor `LogEntry` body + inclusion-proof + SET (Signed Entry Timestamp) at sign time. So the inclusion-proof check is **offline by default** as long as the bundle is intact and the trusted root has the Rekor public key.
- The "online" Rekor check is only required if the verifier wants to confirm the entry still exists in Rekor's current tree (a tlog-tampering check). sigstore-go's default `WithTransparencyLog(1)` is satisfied by the SET inside the bundle and does NOT make a network call to Rekor by default.

**Implication for D-04:** CONTEXT D-04 says "online Rekor for transparency-log inclusion-proof verification." Strict reading: the inclusion-proof check is satisfied offline by the bundle's SET. To force an online tlog tree-head check would require additional code. Recommend the planner treat D-04 as "the inclusion proof is verified" (which the SET covers) and document this nuance — D-04's "online Rekor" framing was written assuming detached signatures, but the bundle format collapses online-vs-offline for the inclusion-proof check itself.

If the user's intent in D-04 is genuinely "fail closed when Rekor is unreachable for a freshness check," that's an extra step beyond the example flow and should be raised back as Q-1 in §"Open questions." Default plan assumes the SET-based offline check satisfies D-04.

**Error path.** Bundle missing or malformed → `bundle.LoadJSONFromPath` returns error. Inclusion proof invalid → `sev.Verify` returns error. The verifier returns the SAME canonical error string `"signature verification FAILED"` at all branches (preserving the existing Pitfall 4 anti-side-channel posture from `verify.go:9-19`). This requires a wrapper that swallows the underlying sigstore-go error message and never logs it (or logs only at debug verbosity behind a flag).

### goreleaser cosign block

Replace `.goreleaser.yaml:50-63` (the entire `signs:` block + minisign details) with ([goreleaser sign customization](https://goreleaser.com/customization/sign/), [docs.sigstore.dev sign-blob](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/)):

```yaml
signs:
  - id: cosign
    cmd: cosign
    artifacts: all
    signature: "${artifact}.sigstore.json"
    args:
      - "sign-blob"
      - "--bundle=${signature}"
      - "${artifact}"
      - "--yes"  # non-interactive; required in CI
```

**Field-by-field rationale:**

- `cmd: cosign` — replaces `cmd: minisign`.
- `artifacts: all` — same as before; signs every archive + checksums.txt.
- `signature: "${artifact}.sigstore.json"` — the bundle file; replaces `${artifact}.minisig`.
- `args` — `sign-blob` subcommand; `--bundle=` writes the combined bundle (NOT separate `.sig` + `.pem`); positional artifact path; `--yes` skips the interactive confirm prompt.
- **No `stdin:` field needed** — cosign keyless reads the OIDC token from `$GITHUB_TOKEN` (`id-token: write` permission) automatically; there's no password to feed.
- **No `MINISIGN_PASSWORD` env var** — gone with minisign.

### release.yml additions

Required edits to `.github/workflows/release.yml`:

| Action | Lines (current) | Change |
|--------|---------------|--------|
| Add `permissions: id-token: write` | line 8-9 | Add `id-token: write` alongside `contents: write`. Required for OIDC token. |
| Delete PLACEHOLDER pre-flight grep | lines 25-36 | Drop entire step. `minisign.pub` no longer exists. |
| Delete embed-pubkey verification | lines 38-45 | Drop `make verify-embed-pubkey` step; the embed target is gone. |
| Replace minisign install | lines 53-89 | Delete entire 37-line block. Replace with cosign installer (see below). |
| Delete minisign secret-key write | lines 131-145 | Delete entire step. No keys; no `/tmp/minisign.key`. |
| Replace `Real release` env vars | lines 147-155 | Drop `MINISIGN_PASSWORD: …`. Keep `GITHUB_TOKEN: …`. Cosign reads OIDC via `actions/core` from the runner. |
| Delete minisign secret wipe | lines 157-164 | Drop entire step. Nothing on disk to wipe. |
| Drop `MINISIGN_PRIVATE_KEY` repo secret | external | Repo Settings > Secrets — remove (out of repo, document in plan). |

Add the cosign installer step before `Set up Go` (or before `Reproducibility gate`):

```yaml
      - name: Install cosign
        uses: sigstore/cosign-installer@v3
        with:
          cosign-release: 'v2.4.1'  # pin the release; bump as cosign upgrades land in CI
```

Pin a specific cosign-installer SHA in the same hash-pin style the workflow already uses (`actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683` etc.) — the maintainer must add the SHA from the release page.

### internal/upgrade/ blast radius

Function-by-function table of changes. File-paths absolute; line numbers as of HEAD (commit 5c0cbcd1).

| File:line | Symbol | Action | Notes |
|-----------|--------|--------|-------|
| `internal/upgrade/verify.go:1-77` (entire file) | `VerifyArchive`, `currentPubKey`, `testPubKeyOverride` | Rewrite | Replace minisign call with sigstore-go bundle verify. Keep `testPubKeyOverride` semantics (test-mode trust root substitution) — see "test strategy" below. Preserve canonical-error-string Pitfall 4 invariant. |
| `internal/upgrade/verify.go:6` | `import "github.com/jedisct1/go-minisign"` | Delete | Replace with sigstore-go imports. |
| `internal/upgrade/pubkey.go:1-51` (entire file) | `pubKeyBytes`, `IsPlaceholderPubKey`, `placeholderMarker` | Delete or repurpose | The `//go:embed minisign.pub` directive must be removed (Risk R-3); otherwise `go build` fails when the file is deleted. **Recommend renaming the file to `trustroot.go`** and using it for `//go:embed trusted_root.json` (sigstore TUF root), which preserves the embed pattern and minimizes churn. The `IsPlaceholderPubKey` distinct-error path (`upgrade.go:130-133`) becomes dead code — delete the call site too. |
| `internal/upgrade/minisign.pub` | (file) | Delete | No longer embedded. |
| `minisign.pub` (repo root) | (file) | Delete | No longer the embed source. |
| `internal/upgrade/upgrade.go:189` | `sigName := archiveName + ".minisig"` | Rewrite | Change to `bundleName := archiveName + ".sigstore.json"`. |
| `internal/upgrade/upgrade.go:196-200` | `sigAsset := rel.FindAsset(sigName)` | Rewrite | Look up the bundle asset instead. |
| `internal/upgrade/upgrade.go:201-205` | `checksumsAsset`, `checksumsSigAsset` | Update | `checksums.txt.minisig` → `checksums.txt.sigstore.json`. The asymmetric-pair guard at lines 213-224 stays — adapt the strings. |
| `internal/upgrade/upgrade.go:226-227, 231-233` | `stageSig`, `downloadFile(... sigAsset.BrowserDownloadURL ...)` | Rewrite | Download bundle to `stageBundle`. |
| `internal/upgrade/upgrade.go:236-256` | checksums verification block | Rewrite | Verify `checksums.txt.sigstore.json` then crossCheckSha256. |
| `internal/upgrade/upgrade.go:258-262` | `VerifyArchive(stageArchive, stageSig)` | Update signature | New signature: `VerifyArchive(archivePath, bundlePath string) error` — call sites are unchanged structurally. |
| `internal/upgrade/upgrade.go:122-133` | `IsPlaceholderPubKey()` placeholder guard | Delete | No placeholder concept in cosign keyless. |
| `internal/upgrade/verify_test.go:1-134` | All tests | Rewrite | Need new testdata: a real sigstore bundle for the `sample-archive.tar.gz` (or a fixture-time-generated one). Three options: (a) generate via `cosign sign-blob` against a test-only repo at fixture-prep time, (b) use sigstore-go's signing API in test setup to produce a bundle against a test trust root + ephemeral key, (c) check in a real bundle from a one-time `cosign sign-blob` invocation. Option (b) is the cleanest — same pattern as `verify.NewVerifier` against custom `trustedMaterial`. The 6 existing test cases (happy path / tampered / wrong key / missing archive / missing sig / malformed key) all map 1:1 to bundle-equivalents. |
| `internal/upgrade/testdata/test_keypair.{pub,key}` | (files) | Delete | Replaced by test-only trust root + cert-bundle fixture. |
| `internal/upgrade/testdata/sample-archive.tar.gz.minisig` | (file) | Delete; regenerate as `.sigstore.json` | Regenerate via Option (b) above. |
| `internal/upgrade/upgrade_test.go` | `archiveNameTemplate` parity test | Update | If the test name-checks `.minisig` paths, update. |
| `Makefile:61-66` | `embed-pubkey`, `verify-embed-pubkey` targets | Delete | And the `build: embed-pubkey` dep at line 6 — drop. The drift-prevention concern that motivated these targets is gone (single-source TUF root JSON). |
| `Makefile:1` | `.PHONY:` | Update | Remove `embed-pubkey verify-embed-pubkey` from the list. |
| `go.mod:9` | `github.com/jedisct1/go-minisign v0.0.0-...` | Remove | `go mod tidy` after rewrite removes it from go.sum too. |
| `go.mod` (additions) | `github.com/sigstore/sigstore-go v1.1.x` | Add | Plus transitive deps: `theupdateframework/go-tuf/v2` if using TUF runtime path. |
| `CONTRIBUTING.md:159-214` | "Releasing" section | Update | Drop minisign keypair setup, repo-secrets ceremony, `MINISIGN_PRIVATE_KEY`/`MINISIGN_PASSWORD` mentions. Add a 2-3 paragraph cosign keyless section noting `id-token: write` permission requirement and the no-keypair posture. |
| `INSTALL.md` | (entire signature-verification recipe) | Rewrite | Document the user-side `cosign verify-blob --bundle … --certificate-identity-regexp … --certificate-oidc-issuer …` ceremony. **Out of scope per CONTEXT § In scope** — leaving in research as a flag because INSTALL.md tells users how to verify; if missed, users are stranded. Recommend planner adds an INSTALL.md update task to plan #1 even though it's not REQ-tagged. |

**Atomic-swap mechanics unchanged.** CONTEXT.md claim verified: `internal/upgrade/swap_unix.go`, `swap_windows.go`, `stage.go`, `archive.go`, `permission.go`, `daemon_detect.go`, `semver.go`, `github.go` are untouched by the cosign migration. Only `verify.go`, `pubkey.go`, the `minisign.pub` files, and the call sites in `upgrade.go` change.

## REL-05: CONTRIBUTING.md reproducibility paragraph

### Current state

`CONTRIBUTING.md:159-214` has a `## Releasing` section. The reproducibility-gate prose lives at `CONTRIBUTING.md:161` (one long sentence inside the section's intro):

> "The CI-enforced reproducibility gate runs two consecutive snapshot builds with identical inputs and refuses to publish if their archive sha256s differ, catching most build-environment non-determinism (toolchain drift, mod_timestamp, trimpath, GOFLAGS) before publication. The gate does not, however, compare against the real-release artifacts that ship to users -- a non-determinism source that lives only behind the real-release code path (e.g. tag-only build constants, changelog generation) would not be caught. If you suspect a real-release-only non-determinism, do a local build of the same tag with `goreleaser release --snapshot --clean --skip=sign` after your tag and diff against the published `dist/` from CI."

This is already the spirit of D-05 — but it does **not** use the literal phrase "Pass-3 limitation" that CONTEXT §"Specifics" pins as the future-grep anchor. Existing prose calls it "the gate does not compare against real-release artifacts." A future maintainer searching `grep -ri 'pass-3' CONTRIBUTING.md` will not find it.

### Recommended placement and wording shell

Two viable options:

- **Option A (minimal):** Edit the existing sentence at `CONTRIBUTING.md:161` to introduce the phrase "Pass-3 limitation." E.g., change "The gate does not, however, compare against the real-release artifacts" to "(This is the documented **Pass-3 limitation**: the gate does not, however, compare against the real-release artifacts…)".
- **Option B (new heading):** Add `### Reproducibility` as a new H3 inside `## Releasing` (between line 158 and line 159, before the existing Releasing intro), with a 4-5 sentence paragraph anchored on "Pass-3 limitation" and citing the v1.9 milestone audit acceptance.

**Recommendation:** Option A. It avoids fragmenting `## Releasing` and keeps the text in the place a contributor reading the release flow already encounters it. The phrase "Pass-3 limitation" lands as a parenthetical anchor; the rest of the sentence already explains the limitation. Leave CONTEXT D-05's rationale-trade-off requirement satisfied by the existing sentence's "If you suspect a real-release-only non-determinism…" follow-up, which already gives the future-maintainer escape hatch.

**Wording shell** (planner can adjust):

> "The CI-enforced reproducibility gate runs two consecutive snapshot builds with identical inputs and refuses to publish if their archive sha256s differ, catching most build-environment non-determinism (toolchain drift, mod_timestamp, trimpath, GOFLAGS) before publication. **This is the documented "Pass-3 limitation":** the gate does NOT compare Pass-1 / Pass-2 / Pass-3 hashes against the real-release artifacts that ship to users — a non-determinism source that lives only behind the real-release code path (e.g. tag-only build constants, changelog generation) would not be caught. The trade-off was deliberate at v1.9: a real-artifact comparison job commits the project to regenerating expected hashes per release for verification value the v1.9 milestone audit deemed acceptable. If release-artifact divergence becomes a concern, a comparison job can be added — see `.planning/milestones/v1.9-MILESTONE-AUDIT.md` for the original trade-off discussion. If you suspect a real-release-only non-determinism today, do a local build of the same tag with `goreleaser release --snapshot --clean --skip=sign` after your tag and diff against the published `dist/` from CI."

This satisfies all CONTEXT §D-05 requirements: explicit Pass-1 / Pass-2 / Pass-3 mention, explicit "does NOT diff against published release artifacts" statement, trade-off rationale that lets a future maintainer flip the decision, future-grep anchor "Pass-3 limitation."

## REL-06: forwarder span unification

### Current dial site

Already wired correctly:

- **Client handler installed:** `internal/forwarder/dial.go:70-72` calls `grpc.WithStatsHandler(otelgrpc.NewClientHandler(otelgrpc.WithTracerProvider(tp)))` inside `tryConnect`.
- **TracerProvider threaded through:** `dial.go:24-40` accepts `tp trace.TracerProvider` parameter; `forwarder.go:27` passes it to `ConnectOrStartDaemon`.
- **Server handler installed:** `internal/daemon/daemon.go:572-576` calls `grpc.StatsHandler(otelgrpc.NewServerHandler(otelgrpc.WithTracerProvider(d.obs.TracerProvider())))` on the gRPC server.

### Why traces don't unify today (the actual gap)

Three blockers, none of which is "the client handler is missing":

1. **Forwarder TracerProvider is Noop.** `internal/forwarder/forwarder.go:25` constructs `fwdProvider := obs.Noop(logger.Handler())`. `obs.Noop` returns a `tracenoop.NewTracerProvider()` (`internal/obs/obs.go:51`), so the `forwarder.tools.call` span at `forwarder.go:140-144` is a no-op span — it has no trace ID, never gets a context, and `otelgrpc.NewClientHandler` has nothing to inject as `traceparent` metadata. The daemon's server handler then starts a fresh root span instead of becoming a child of the forwarder's span.
2. **No global TextMapPropagator is set.** Search across `internal/` for `otel.SetTextMapPropagator` / `propagation.TraceContext` / `propagation.NewCompositeTextMapPropagator` returns empty. `otelgrpc` defaults to `otel.GetTextMapPropagator()`, which in turn defaults to a NoopTextMapPropagator if nothing is set globally. So even if (1) is fixed, traceparent isn't injected into gRPC metadata. Both processes need to install at least `propagation.TraceContext{}` (W3C TraceContext) — `otelgrpc.WithPropagators(propagation.TraceContext{})` is the per-handler form that avoids touching the global.
3. **`activate.go:53` also creates a Noop forwarder.** `internal/cli/activate.go:53` calls `forwarder.ConnectOrStartDaemon(... noop.NewTracerProvider())`. Whichever caller path actually emits the `forwarder.tools.call` span needs a real provider; activate is a one-shot CLI subcommand and may stay Noop, but verify which path matters during planning.

### otelgrpc client handler install — exact code shape

The client install is correct as-is:

```go
// internal/forwarder/dial.go:70-72 (current — no change needed)
grpc.WithStatsHandler(otelgrpc.NewClientHandler(
    otelgrpc.WithTracerProvider(tp),
)),
```

The actual REL-06 changes are:

```go
// dial.go (NEW) — propagator option, both client and server handlers:
grpc.WithStatsHandler(otelgrpc.NewClientHandler(
    otelgrpc.WithTracerProvider(tp),
    otelgrpc.WithPropagators(propagation.TraceContext{}),  // <-- ADD
)),

// daemon.go:573-575 (parallel change):
grpc.StatsHandler(otelgrpc.NewServerHandler(
    otelgrpc.WithTracerProvider(d.obs.TracerProvider()),
    otelgrpc.WithPropagators(propagation.TraceContext{}),  // <-- ADD
)),
```

### TracerProvider sourcing — real-vs-Noop migration

Forwarder is a **separate process** spawned by `internal/cli/activate.go` or directly via stdio entry. CONTEXT §code_context line 119 makes this point: OTel context propagates over gRPC metadata, NOT in-process TracerProvider sharing.

Two migration paths:

| Option | What | Pro | Con |
|--------|------|-----|-----|
| Env-var-driven OTLP | Forwarder reads `OTEL_EXPORTER_OTLP_ENDPOINT` from env at startup. If set, `obs.WithTracing` instead of `obs.Noop`. | Standard OTel pattern; daemon already supports it via `obs.TracingConfig{Endpoint: ...}` (`tracing.go:75`). | Adds env-var sniffing to forwarder that wasn't there before. |
| CLI-arg-driven | Forwarder accepts `--otlp-endpoint` flag, passes through to `obs.WithTracing`. | Explicit; matches existing daemon `--socket` flag style. | More invasive; forwarder is currently flagless. |

**Recommendation:** Option 1 (env-var). The OTel ecosystem's standard discovery path is `OTEL_EXPORTER_OTLP_ENDPOINT` — adopting it costs ~10 lines in `forwarder.go` and is the same pattern users will set for any OTel-instrumented binary in their environment. The forwarder's invocation is already env-var-friendly (it inherits the calling shell's env). Daemon end is already wired (`daemon.go:129-133`).

### Server-side handler verification

✓ Confirmed: `internal/daemon/daemon.go:572-576` installs `otelgrpc.NewServerHandler` with `WithTracerProvider(d.obs.TracerProvider())` on the gRPC server before registering the `ForwarderServiceServer`. CONTEXT §D-06 assumption ("server-side handler is already installed") is correct.

The only addition is the propagator option (see code shape above).

### Single-trace assertion test pattern

The repo **already** has the in-memory-exporter pattern at `internal/forwarder/forwarder_test.go:111-141` (`TestForwarderRootSpan`). It uses:

```go
import (
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/trace/tracetest"
    tracenoop "go.opentelemetry.io/otel/trace/noop"
)

exporter := tracetest.NewInMemoryExporter()
tp := sdktrace.NewTracerProvider(
    sdktrace.WithSampler(sdktrace.AlwaysSample()),
    sdktrace.WithSyncer(exporter),
)
defer tp.Shutdown(context.Background())
// ... exercise forwarder ...
spans := exporter.GetSpans()
```

The new end-to-end assertion test should:

1. Spin up an in-memory daemon (the test harness in `test/integration/` does this).
2. Inject an `sdktrace.TracerProvider` backed by `tracetest.NewInMemoryExporter()` into BOTH the daemon's `obs.Provider` (via `obs.NewForTest` at `obs/obs.go:58-62`) AND the forwarder's `obs.Provider`.
3. Issue a `tools/call` request through stdio.
4. After the request completes, call `exporter.GetSpans()` from BOTH exporters (or a single shared exporter if the test process can share it across the two `obs.Provider` instances).
5. Assert: at least 2 spans across the exporters; both spans share the same `TraceID()`; one span name is `forwarder.tools.call`; the other span name is `serena.v1.ForwarderService/StreamMCP` (the otelgrpc-generated server span).

If the daemon and forwarder run in the same process under test (e.g., the integration harness already shares the process), a single shared `tracetest.InMemoryExporter` works. If split, the test must export both and aggregate.

**Imports for the new test:** all already present in the codebase — `sdktrace`, `tracetest` from `forwarder_test.go:11-12`. No new dep additions for the test.

## REQUIREMENTS.md won't-do recording

### Marker convention

`- [~]` is **not yet used in REQUIREMENTS.md**. It appears once in the codebase at `.planning/milestones/v1.9-phases/56-bug-ls-notification-dispatch-and-jdtls-readiness/56-04-SUMMARY.md:133` in a different context (a verification step that was opted-out of locally). Phase 58 introduces it as the "rejected / won't-do" status marker for REQUIREMENTS.md.

The CONTEXT.md §Specifics passage pins this literally: `- [~]` is the marker. Plans MUST use the exact glyph (tilde), not `- [-]` or `- [N/A]` or other variants. The rationale string is also pinned literally:

> `won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience`

**Future-search anchor:** add a one-line legend somewhere in REQUIREMENTS.md explaining `- [~]` (e.g., near the top of the file or just above the REL section) so a future contributor encountering the marker doesn't have to grep for it. Recommend the legend lives at the top of the file as a brief "## Status legend" section above all requirements.

**Roadmap edit (companion):** `.planning/milestones/v1.10-ROADMAP.md` Phase 58 section (line 42 per CONTEXT) shrinks to REL-01/05/06 only, removing REL-02/03/04 list items and the corresponding SC-2/SC-3 lines. This is a separate file edit from REQUIREMENTS.md — both happen in the same plan/task.

## Risks & Gotchas

- **R-1 [HIGH]: sigstore-go vs cosign-go library choice.** Both verify bundles. sigstore-go is the upstream-blessed minimal API (HIGH confidence per [sigstore-go README](https://github.com/sigstore/sigstore-go) "considered stable and ready for production use"); cosign-go is heavier and OCI-oriented. If during implementation the planner finds sigstore-go is missing a feature (rare per the README's conformance-test claim), fall back to cosign-go. Pin sigstore-go v1.1.x for v1.10.
- **R-2 [HIGH]: identity-pinning regex correctness.** A flawed regex anchors badly and lets `agenthands/helix-fork/...` or `org/agenthands-helix/...` verify. The recommended regex above uses `^`/`$` anchors and escaped dots. **Plan MUST include a security-review task that pen-tests the regex against malicious-looking SAN strings** (e.g., `https://github.com/agenthands/helix.attacker.com/...`, `https://github.com/agenthands-fork/helix/...`). Test with at least 4-5 negative cases.
- **R-3 [MEDIUM]: dangling `//go:embed minisign.pub` directive.** `internal/upgrade/pubkey.go:25` has `//go:embed minisign.pub`. If the file is deleted but the directive isn't, `go build` fails with "pattern minisign.pub: no matching files found." Plan must order operations: rewrite verify.go FIRST (drops the embed import + directive), THEN delete `minisign.pub` files. The recommended rename-to-`trustroot.go` approach inverts this: replace the directive with `//go:embed trusted_root.json`, ship the new file, then delete the old.
- **R-4 [MEDIUM]: GitHub Actions OIDC token TTL.** Tokens are valid for ~10 minutes. `cosign sign-blob` is sub-second per artifact. With 7 archives + 1 checksums.txt = 8 sign operations × <1s each, plenty of headroom. Not a real risk for v1.10's archive count, but document the ceiling for future scale-up.
- **R-5 [LOW]: Rekor public-good rate limits.** 60 req/min/IP unauthenticated. `helix upgrade` reads the SET from the bundle (no Rekor call) for the inclusion-proof check by default — see §"Rekor online behavior." So end-user upgrade does not hit Rekor at all unless the planner wires an additional online tlog-tree-head freshness check. Sign-side (CI) hits Rekor once per release, well under the limit.
- **R-6 [HIGH]: forwarder is a separate process.** Repeating CONTEXT §code_context for emphasis: do NOT "share TracerProvider" between forwarder and daemon code paths. Each process owns its own `*sdktrace.TracerProvider`; the trace continuity comes from `traceparent` gRPC metadata propagated by the otelgrpc handlers + a non-Noop `TextMapPropagator`. A plan that imports `daemon.obs.TracerProvider()` into the forwarder is wrong and probably won't compile (separate processes).
- **R-7 [MEDIUM]: trusted_root.json staleness.** If embedded, the JSON is byte-stable per release. Sigstore rotates the public-good Fulcio CA infrequently (multi-year cadence) but a stale embedded root could in principle reject post-rotation signatures. Document a "refresh trusted_root.json before each minor release" step in CONTRIBUTING.md release ceremony, even if no rotation is pending.
- **R-8 [LOW]: cosign v2.x API churn.** Per [pkg.go.dev cosign](https://pkg.go.dev/github.com/sigstore/cosign/v2/pkg/cosign) the v2 API has been stable since 2023. sigstore-go v1.x is also stable per its README. The risk surface is small but pin both deps to a `v1.1.x` / `v2.4.x` range and test before bumping minors.
- **R-9 [LOW]: canonical error string preservation.** Current `verify.go:9-19` Pitfall 4 mandates a single `"signature verification FAILED"` string at every failure branch (anti-side-channel). The sigstore-go API surfaces verbose errors (cert chain failure vs SAN mismatch vs Rekor missing). Wrapper MUST collapse all to the canonical string and not leak the underlying error to logs at default verbosity. The 6 existing verify_test.go cases enforce this — keep them.

## Plan-splitting recommendation

**Recommend 4 plans for Phase 58.** Rationale: ordering matters (release-side ships before verifier so the verifier never inherits a state where minisign artifacts no longer exist), and the three areas (cosign, forwarder OTel, docs) have independent worktree-isolation safety.

| Plan | Scope | Files touched | Why a separate plan |
|------|-------|---------------|---------------------|
| **P1: Release-side cosign swap** | `.goreleaser.yaml` `signs:` block; `release.yml` permissions/installer/secret-cleanup; remove `MINISIGN_PRIVATE_KEY`/`MINISIGN_PASSWORD` from repo secrets (manual step, plan calls it out); update `CONTRIBUTING.md` Releasing section to drop minisign keypair ceremony. | 3 files + 1 manual ceremony. | Must merge BEFORE P2 ships. After P1 merges, the repo signs releases with cosign but the binary still verifies with minisign — releases between P1-merge and P2-merge would be unverifiable by `helix upgrade`. **Mitigate:** don't tag a `v*` release between P1 and P2 merging. The cosign-only hard cut means P1 + P2 BOTH must be merged before tagging v1.10.0. |
| **P2: Verifier rewrite + key cleanup** | `internal/upgrade/verify.go`, `pubkey.go` → `trustroot.go`, `upgrade.go` archive-name + URL paths, `verify_test.go`, testdata fixtures, `Makefile` `embed-pubkey` removal, `go.mod`/`go.sum` swap, `INSTALL.md` user-side verify ceremony. Both `minisign.pub` files deleted. | ~10 files. | Largest blast radius; deserves dedicated review pass on identity-pinning regex (R-2). Worktree-isolated because nothing else in `internal/upgrade/` changes structurally. |
| **P3: Forwarder span unification (REL-06)** | `internal/forwarder/forwarder.go` (Noop → real TracerProvider via env), `dial.go` and `daemon.go` (add `otelgrpc.WithPropagators`), `forwarder_test.go` end-to-end span assertion (or a new `test/integration/` test). | ~3-4 files. | Independent of cosign work — different package, different concern. Can ship in parallel worktree. |
| **P4: Won't-do recording + REL-05 doc** | `.planning/REQUIREMENTS.md` (REL-02/03/04 → `- [~]` + rationale + status legend), `.planning/milestones/v1.10-ROADMAP.md` Phase 58 section shrink, `CONTRIBUTING.md` Pass-3 limitation parenthetical edit. | 3 doc files. | Pure docs; tail-end of the phase. Could be folded into P1 (CONTEXT §Discretion permits) but keeping it separate makes the diff scannable for the Phase 58 closeout. |

**Ordering constraint:** P1 and P2 MUST merge before tagging v1.10.0. P3 and P4 are independent. P1 → P2 is the strict-order pair; P3 and P4 can ship in any order relative to P1/P2.

**Alternative considered:** combining P1 + P2 into one mega-plan. Rejected because (a) the diff is ~15 files which is at the edge of reviewable in one PR, (b) the security-critical identity-pinning regex deserves its own focused review pass per R-2, (c) worktree isolation is cleaner with two separate plans. The "P1 must merge before P2" constraint is a documentation/sequencing detail, not an argument against splitting.

## Open questions for planner

1. **Q-1: D-04 strict reading — does "online Rekor" mean the SET-based offline check, or an additional online tlog-tree-head freshness check?** The bundle's embedded SET satisfies inclusion-proof verification offline. Forcing an online tlog-tree-head check is extra code and an extra failure mode. Default: treat the SET as satisfying D-04. If user wants a stricter online check, the planner should raise this back via discuss-phase before P2 implementation starts. Confidence on default reading: MEDIUM — the cosign documentation on this nuance is thin.
2. **Q-2: Trusted root sourcing — embed `trusted_root.json` or fetch via TUF at runtime?** §"Trust root sourcing" recommends embed; user-side intent (single-binary ethos) supports embed. Confirm before P2.
3. **Q-3: INSTALL.md update scope.** INSTALL.md owns the user-side `cosign verify-blob` recipe today (with minisign instructions). It is NOT in CONTEXT §In scope but is structurally tied to P2 — users who follow INSTALL.md after P2 ships will fail with minisign instructions. Recommend adding to P2; confirm.
4. **Q-4: Canonical error string vs developer-mode debug logs.** Pitfall 4 mandates a single error string at default verbosity. Should `--debug` or `HELIX_DEBUG=1` un-collapse it (so a developer can see "cert SAN mismatch" vs "Rekor missing")? The current minisign code has no such escape hatch. Recommend matching the existing posture (no escape hatch) for consistency, but note the trade-off.
5. **Q-5: Who actually runs the forwarder with a non-Noop TracerProvider?** §REL-06 §"TracerProvider sourcing" recommends env-var (`OTEL_EXPORTER_OTLP_ENDPOINT`). For the test in P3 we use `obs.NewForTest`. For real usage, who sets the env var? Likely no one in the v1.10 default install; the test asserts the *capability* exists. Confirm this is sufficient for D-06's "verified by an end-to-end trace assertion in an integration test."

## Code Examples

### Verifier minimal flow (REL-01)

```go
// internal/upgrade/verify.go (post-rewrite skeleton)
// Source: github.com/sigstore/sigstore-go/examples/sigstore-go-verification/main.go (Apache 2.0)
package upgrade

import (
    _ "embed"
    "errors"

    "github.com/sigstore/sigstore-go/pkg/bundle"
    "github.com/sigstore/sigstore-go/pkg/root"
    "github.com/sigstore/sigstore-go/pkg/verify"
)

//go:embed trusted_root.json
var trustedRootJSON []byte

const (
    expectedOIDIssuer    = "https://token.actions.githubusercontent.com"
    expectedSANRegex     = `^https://github\.com/agenthands/helix/\.github/workflows/release\.yml@refs/tags/v[\d.]+(-rc\d+|-beta\d+|-alpha\d+)?$`
    canonicalVerifyError = "signature verification FAILED"
)

// VerifyArchive verifies that bundlePath contains a valid sigstore bundle for
// archivePath, signed by the agenthands/helix release workflow under the
// public-good Sigstore trust root. Returns the canonical error string at every
// failure site (Pitfall 4 anti-side-channel posture).
func VerifyArchive(archivePath, bundlePath string) error {
    b, err := bundle.LoadJSONFromPath(bundlePath)
    if err != nil {
        return errors.New(canonicalVerifyError)
    }

    trustedRoot, err := root.NewTrustedRootFromJSON(trustedRootJSON)
    if err != nil {
        return errors.New(canonicalVerifyError)
    }
    trustedMaterial := root.TrustedMaterialCollection{trustedRoot}

    sev, err := verify.NewVerifier(trustedMaterial,
        verify.WithSignedCertificateTimestamps(1),
        verify.WithObserverTimestamps(1),
        verify.WithTransparencyLog(1),
    )
    if err != nil {
        return errors.New(canonicalVerifyError)
    }

    certID, err := verify.NewShortCertificateIdentity(expectedOIDIssuer, "", "", expectedSANRegex)
    if err != nil {
        return errors.New(canonicalVerifyError)
    }

    file, err := os.Open(archivePath)
    if err != nil {
        return errors.New(canonicalVerifyError)
    }
    defer file.Close()

    _, err = sev.Verify(b, verify.NewPolicy(verify.WithArtifact(file), verify.WithCertificateIdentity(certID)))
    if err != nil {
        return errors.New(canonicalVerifyError)
    }
    return nil
}
```

### otelgrpc client + propagator (REL-06)

```go
// internal/forwarder/dial.go (post-edit, lines 70-72 area)
// Source: go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc API
import (
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
    "go.opentelemetry.io/otel/propagation"
)

grpc.WithStatsHandler(otelgrpc.NewClientHandler(
    otelgrpc.WithTracerProvider(tp),
    otelgrpc.WithPropagators(propagation.TraceContext{}),
)),
```

```go
// internal/daemon/daemon.go:572-576 (post-edit)
d.grpcServer = grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler(
        otelgrpc.WithTracerProvider(d.obs.TracerProvider()),
        otelgrpc.WithPropagators(propagation.TraceContext{}),
    )),
)
```

```go
// internal/forwarder/forwarder.go:25 (post-edit)
// Source: internal/obs/tracing.go:75 WithTracing pattern
endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
fwdProvider := obs.WithTracing(logger.Handler(), obs.TracingConfig{
    Endpoint:    endpoint,                  // empty → falls back to noop, see tracing.go:78
    ServiceName: "helix-forwarder",
    SampleRatio: 1.0,
}, logger)
```

## Sources

### Primary (HIGH confidence)
- `internal/forwarder/dial.go:24-80` — current dial site with otelgrpc.NewClientHandler
- `internal/forwarder/forwarder.go:1-145` — Noop tracer construction + sendWithSpan
- `internal/forwarder/forwarder_test.go:111-141` — existing in-memory exporter pattern
- `internal/daemon/daemon.go:572-576` — server-side otelgrpc.NewServerHandler install
- `internal/upgrade/verify.go:1-77` — current minisign verifier, error-canonicalization invariant
- `internal/upgrade/upgrade.go:1-461` — archive-name + URL builders, .minisig fetch, atomic-swap orchestration
- `internal/upgrade/pubkey.go:1-51` — //go:embed directive + IsPlaceholderPubKey
- `internal/obs/obs.go:30-95` — Provider, Noop, NewForTest, TracerProvider accessor
- `internal/obs/tracing.go:1-94` — WithTracing constructor, OTLP endpoint config
- `.goreleaser.yaml:50-63` — current minisign signs: block (replacement target)
- `.github/workflows/release.yml:1-165` — current release workflow with PLACEHOLDER pre-flight + minisign install + secret-key handling
- `CONTRIBUTING.md:159-214` — current Releasing section, including the existing reproducibility prose at line 161
- `.planning/REQUIREMENTS.md:115-126` — REL-01..REL-06 row definitions
- `.planning/PROJECT.md:166-169` — Tech debt accepted at v1.9 close

### Secondary (HIGH-MEDIUM confidence)
- [docs.sigstore.dev sign-blob](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/) — `cosign sign-blob --bundle` output is single .sigstore.json; `--bundle` recommended over separate sig+cert
- [docs.sigstore.dev verify](https://docs.sigstore.dev/cosign/verifying/verify/) — `--certificate-identity` + `--certificate-oidc-issuer` for keyless verification
- [goreleaser sign customization](https://goreleaser.com/customization/sign/) — exact `signs:` YAML for cosign keyless: `cmd: cosign`, `signature: "${artifact}.sigstore.json"`, args including `sign-blob`, `--bundle=${signature}`, `--yes`
- [pkg.go.dev cosign](https://pkg.go.dev/github.com/sigstore/cosign/v2/pkg/cosign) — `VerifyBlobSignature(ctx, sig, *CheckOpts)`, `CheckOpts.Identities []Identity{Issuer, Subject, IssuerRegExp, SubjectRegExp}`
- [github.com/sigstore/sigstore-go](https://github.com/sigstore/sigstore-go) — v1.1.4 (2025-12-10), Go 1.23+, "considered stable and ready for production use," conformance-test-suite-passing
- [sigstore-go-verification example main.go](https://raw.githubusercontent.com/sigstore/sigstore-go/main/examples/sigstore-go-verification/main.go) — verbatim Go code for the verify flow used in §"Verifier API surface"
- [WebSearch result, GitHub Actions keyless verify pattern](https://blog.sigstore.dev/cosign-verify-bundles/) — `--certificate-identity-regexp` example anchored on `^https://github.com/<org>/<repo>/.github/workflows/release.yml@refs/tags/v.*`

### Tertiary (LOW — flagged for validation)
- Default Rekor inclusion-proof check is "offline via embedded SET" in sigstore-go default config — inferred from `verify.WithTransparencyLog(1)` semantics + bundle format, not directly cited from sigstore-go README. Validate during P2 by writing a test that disables network and confirms verification still succeeds. (See Q-1.)

## Project Constraints (from CLAUDE.md)

- **Go is the implementation language.** All v1.10 work is Go (`go build ./cmd/helix`, `go test ./...`, `go vet ./...`). No Python in `internal/`.
- **Single Go binary.** No CGO (`CGO_ENABLED=0` in `.goreleaser.yaml:13`), no Docker, no runtime deps. Trust root must be embedded or fetched at runtime; no config file expected on disk.
- **Always run `go vet` and `go test`** before completing any Go task — this gates every plan in this phase.
- **Benchmarks are local-only.** No CI bench workflow plumbing for any cosign-perf claims; if performance becomes a concern, document local benchstat numbers in PR description (honor system, no PR-time gate).
- **GSD Workflow.** Direct edits outside a GSD command are forbidden. All Phase 58 plans go through `/gsd:execute-phase`.
- **SMTC-first for code-aware questions.** This research used SMTC-aware reasoning indirectly (file-by-file blast radius) but was constrained by the fact that this is a documentation phase. Implementing plans should use `mcp__smtc__find_references` / `goto_definition` for any minisign call-site or `otelgrpc` symbol re-verification before edits.

## Validation Architecture

(Workflow `nyquist_validation` not explicitly disabled in `.planning/config.json` — section included.)

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `stretchr/testify` (already in `go.mod`) |
| Config file | none — `go test ./...` is the entry point |
| Quick run command | `go test ./internal/upgrade/... ./internal/forwarder/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REL-01 | `VerifyArchive` accepts a valid sigstore bundle | unit | `go test ./internal/upgrade/ -run TestVerifyArchiveHappyPath -count=1` | ✅ rewrite (current minisign-targeted) |
| REL-01 | `VerifyArchive` rejects a tampered bundle | unit | `go test ./internal/upgrade/ -run TestVerifyArchiveTampered -count=1` | ✅ rewrite |
| REL-01 | `VerifyArchive` rejects a bundle with wrong cert identity (SAN mismatch) | unit (NEW) | `go test ./internal/upgrade/ -run TestVerifyArchiveWrongIdentity -count=1` | ❌ Wave 0 (new test for the regex anti-pinning case) |
| REL-01 | `VerifyArchive` rejects a bundle with wrong OIDC issuer | unit (NEW) | `go test ./internal/upgrade/ -run TestVerifyArchiveWrongIssuer -count=1` | ❌ Wave 0 |
| REL-01 | `helix upgrade` fetches `.sigstore.json` not `.minisig` | unit | `go test ./internal/upgrade/ -run TestUpgradeAssetNames -count=1` | ✅ adapt existing `archiveAssetName` parity test |
| REL-01 | release.yml passes lint + `actionlint` (if present) | integration | `actionlint .github/workflows/release.yml` | ❌ Wave 0 if `actionlint` not installed |
| REL-05 | CONTRIBUTING.md contains literal phrase "Pass-3 limitation" | integration | `grep -q 'Pass-3 limitation' CONTRIBUTING.md` | ❌ Wave 0 (or shell verification step in plan) |
| REL-06 | `forwarder.tools.call` and `serena.v1.ForwarderService/StreamMCP` share a TraceID | integration | `go test ./test/integration/ -run TestE2ETraceContinuity -count=1` | ❌ Wave 0 |
| REL-06 | propagator option set on both client and server otelgrpc handlers | unit (compile-time) | `go vet ./internal/forwarder/... ./internal/daemon/...` | ✅ existing |
| (won't-do) | REQUIREMENTS.md REL-02/03/04 carry `- [~]` marker | manual or shell | `grep -E '^- \[~\] \*\*REL-(02\|03\|04)' .planning/REQUIREMENTS.md \| wc -l` (expect 3) | ❌ Wave 0 (or shell verification step in plan) |

### Sampling Rate

- **Per task commit:** `go test ./internal/upgrade/... ./internal/forwarder/... -count=1`
- **Per wave merge:** `go test ./... -count=1 && go vet ./...`
- **Phase gate:** Full suite green + `make build` produces signed `.sigstore.json` outputs in dry-run snapshot before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/upgrade/verify_test.go::TestVerifyArchiveWrongIdentity` — covers REL-01 identity-pinning regex
- [ ] `internal/upgrade/verify_test.go::TestVerifyArchiveWrongIssuer` — covers REL-01 OIDC issuer pinning
- [ ] `internal/upgrade/testdata/` — bundle fixture generation script (regenerate `sample-archive.tar.gz.sigstore.json` against a test trust root + ephemeral key, replacing `test_keypair.{pub,key}` and `sample-archive.tar.gz.minisig`)
- [ ] `test/integration/trace_continuity_test.go` (or equivalent) — covers REL-06 single-trace assertion
- [ ] Shell verification commands for REL-05 phrase + REQUIREMENTS.md `- [~]` marker count — recommend the planner expresses these as bash one-liners in the plan's verify steps rather than as Go tests

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `cosign` CLI | Local maintainer dry-run only (CI installs via `sigstore/cosign-installer@v3`) | not checked at research time | — | Document `brew install cosign` (macOS) or [GitHub release tarball](https://github.com/sigstore/cosign/releases) (Linux/Windows) in CONTRIBUTING.md |
| `goreleaser` | Local dry-run | already documented in CONTRIBUTING.md:169 | — | unchanged |
| Go 1.25+ | All work | required by go.mod | per `go.mod` | — |
| `actionlint` | Optional release.yml lint | not standard | — | skip; visual review of YAML diff |
| `make` | Build chain | required | — | — |

**Missing dependencies with no fallback:** none — all the work happens in CI for the actual release; local dry-runs use `--skip=sign` per `make release-snapshot` precedent.

**Missing dependencies with fallback:** `cosign` for local verification of a real signed release (out of scope per D-04 — Rekor-online verification is the user path).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | sigstore-go v1.1.x preferred over cosign-go for the verifier rewrite | §"Verifier API surface" | Verifier compiles but pulls heavier dep graph; functionally OK either way. Verifiable in P2 by spiking both. |
| A2 | The bundle's embedded SET satisfies D-04's "online Rekor for transparency-log inclusion-proof verification" | §"Rekor online behavior" | If user wants a stricter online tlog-tree-head check, P2 needs an extra step. Raised as Q-1. |
| A3 | Embedding `trusted_root.json` is preferred over runtime TUF fetch | §"Trust root sourcing" | If user wants always-current trust root, switch to TUF runtime path. Raised as Q-2. |
| A4 | The forwarder's TracerProvider should be wired via `OTEL_EXPORTER_OTLP_ENDPOINT` env var, not a CLI flag | §REL-06 §"TracerProvider sourcing" | Different ergonomics; either works. |
| A5 | Phase 58 plans MUST split REL-01 into "release-side first, verifier second" pairs | §"Plan-splitting recommendation" | Single mega-plan is reviewable but harder to security-review. CONTEXT §Discretion permits either. |
| A6 | INSTALL.md user-side verify recipe is structurally part of P2 even though not in CONTEXT §In scope | §"internal/upgrade/ blast radius" + Q-3 | Users who follow stale INSTALL.md after v1.10.0 ships are stranded. |
| A7 | `make embed-pubkey` and `make verify-embed-pubkey` Makefile targets get deleted (no replacement gate) | §"internal/upgrade/ blast radius" | Loss of drift detection — but the embedded asset (trust root JSON) is now upstream-sourced from sigstore TUF, so drift is a different problem. |

## Metadata

**Confidence breakdown:**
- Release-side YAML + release.yml additions: HIGH — verified against goreleaser docs + cosign sign-blob docs + the existing `release.yml` line-by-line.
- internal/upgrade/ blast radius: HIGH — verified by reading every `*.go` file in the package and grep'ing import sites.
- otelgrpc + propagator analysis (REL-06): HIGH — verified by reading the existing dial.go, forwarder.go, daemon.go, obs/tracing.go and grep'ing for `SetTextMapPropagator` (none found).
- sigstore-go v1.1.4 stability claim: HIGH — direct citation from upstream README.
- Identity-pinning regex correctness: MEDIUM — recommended pattern is sound but needs negative-test pen-testing in P2 (R-2). Per [WebSearch result, GitHub Actions keyless verify pattern](https://blog.sigstore.dev/cosign-verify-bundles/) the anchor + escape pattern matches established practice; specific to agenthands/helix it is novel.
- D-04 SET-based-offline-check reading: MEDIUM — inferred from sigstore-go semantics, not directly cited. See Q-1.

**Research date:** 2026-05-03
**Valid until:** 2026-06-03 (sigstore-go and cosign release cadence is monthly-ish; check for security advisories before P2 execution if research is older than 30 days)

Sources used (URLs):
- https://docs.sigstore.dev/cosign/signing/signing_with_blobs/
- https://docs.sigstore.dev/cosign/verifying/verify/
- https://goreleaser.com/customization/sign/
- https://pkg.go.dev/github.com/sigstore/cosign/v2/pkg/cosign
- https://pkg.go.dev/github.com/sigstore/sigstore-go
- https://github.com/sigstore/sigstore-go
- https://raw.githubusercontent.com/sigstore/sigstore-go/main/examples/sigstore-go-verification/main.go
- https://blog.sigstore.dev/cosign-verify-bundles/
- https://github.com/sigstore/cosign/blob/main/doc/cosign_verify-blob.md
