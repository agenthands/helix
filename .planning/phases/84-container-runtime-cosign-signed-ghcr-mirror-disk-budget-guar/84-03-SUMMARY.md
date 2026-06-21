---
phase: 84
plan: 03
subsystem: bench/container
tags: [container, cosign, sigstore-go, verify, crane, supply-chain]
requires:
  - "bench/container.isHexSHA256 / errBadDigest / errBadRepo / isValidRepo (Plan 01, same package)"
  - "bench/container.ArchGate(hostArch, manifestArch) (Plan 01, same package)"
  - "bench/container.Ensure(sha, fetch) / cacheDir() / ImagePath / cacheOKMarker (Plan 02, same package)"
  - "bench/container.Detect() (Plan 01, live-test skip gate)"
  - "internal/upgrade/verify.go sigstore-go keyless-verify pattern (analog, cloned not imported)"
provides:
  - "bench/container.VerifyImage(manifestBytes, bundleBytes []byte) error — in-process cosign keyless verify, single canonical error"
  - "bench/container.pinnedOIDCIssuer (token.actions.githubusercontent.com) + pinnedSANRegexLiteral (bench-mirror.yml@refs/heads/main)"
  - "bench/container.testTrustedRootOverride / currentTrustedRoot() test seam (no embedded prod root this phase)"
  - "bench/container.VerifyThenPull(ctx, repo, digest, opts...) (hit, dir, err) — arch-gate → verify → cache fill ordering"
  - "bench/container Option seams (unexported: withHostArch/withArchFn/withFetchMetaFn/withVerifyFn/withPullFn/withEnsureFn)"
  - "VirtualSigstore fixture generator + committed trust root + canonical/wrong-org/wrong-issuer bundles + sample manifest"
  - "HELIX_BENCH_MIRROR live-test env gate (absent ⇒ TestVerifyThenPullLive SKIPs)"
affects:
  - "go.mod / go.sum (crane@v0.20.7 dependency closure pinned in; +docker/cli +docker/distribution +docker-credential-helpers as crane registry-auth indirects — NOT the Engine SDK)"
  - "Makefile verify-no-docker-sdk gate (Rule 1 fix: anchored grep so docker-credential-helpers no longer false-positives)"
tech-stack:
  added:
    - "github.com/google/go-containerregistry/pkg/crane (daemon-free Config/Digest/Pull — already in go.mod at v0.20.7, now in the build graph)"
  patterns:
    - "sigstore-go NewSignedEntityVerifier + NewShortCertificateIdentity pinned issuer/SAN cloned from internal/upgrade/verify.go"
    - "single-canonical-error discipline (Pitfall 4): every failure branch returns errors.New(\"signature verification FAILED\"), no oracle"
    - "verify-then-pull ordering (Pitfall 3 TOCTOU): VerifyImage gates Ensure's fetch closure so a verify failure publishes zero cache bytes"
    - "VirtualSigstore (pkg/testing/ca) ephemeral-CA fixtures so tampered/wrong-org/wrong-issuer are rejected on identity, not chain-of-trust"
    - "unexported functional-option test seams so hermetic tests drive ordering/arch-gate without a live registry"
key-files:
  created:
    - "bench/container/verify.go"
    - "bench/container/verify_test.go"
    - "bench/container/pull.go"
    - "bench/container/pull_test.go"
    - "bench/container/testdata/generate_fixtures.go"
    - "bench/container/testdata/manifest.json"
    - "bench/container/testdata/trusted_root.json"
    - "bench/container/testdata/manifest.canonical.sigstore.json"
    - "bench/container/testdata/manifest.wrong-org.sigstore.json"
    - "bench/container/testdata/manifest.wrong-issuer.sigstore.json"
  modified:
    - "go.mod"
    - "go.sum"
    - "Makefile"
decisions:
  - "pinnedSANRegexLiteral pinned to .github/workflows/bench-mirror.yml@refs/heads/main (Open Q3: mirror publishes on a branch schedule, not a semver tag); couples to Plan 04 publish ref — documented in-file."
  - "No embedded production trust root this phase: currentTrustedRoot() returns only the test override; production callers fail closed (canonical error) until Plan 04 wires a real root + the live test is gated/SKIPed. Production-root embedding deferred to Plan 04."
  - "Fixture build tag is `fixturegen` (not the conventional `ignore`): Go 1.26 `go run -tags ignore` re-enables stdlib/x-module `//go:build ignore` gen-tool files, causing an import-cycle cascade. A unique tag isolates the generator."
  - "crane dependency added via `go get github.com/google/go-containerregistry/pkg/crane@v0.20.7` (pinned), NOT `go mod tidy` — full tidy is blocked by a pre-existing unrelated s2a-go resolution failure (same blocker noted in Phase 83-01)."
  - "No live fetchMeta/pull wiring this phase: notWiredFetchMeta/notWiredPull fail closed with sentinels until the Plan 04 mirror exists; the gated live test injects real collaborators when HELIX_BENCH_MIRROR is set."
metrics:
  duration: ~7min
  tasks: 2
  files: 13
  completed: 2026-06-21
---

# Phase 84 Plan 03: Container Runtime — In-Process Cosign Verify + Verify-Then-Pull Summary

In-process cosign keyless verifier for the GHCR bench mirror (cloning the `internal/upgrade/verify.go` sigstore-go pattern: pinned OIDC issuer + SAN regex + single-canonical-error discipline) plus the verify-then-pull orchestration that inspects manifest arch daemon-free via go-containerregistry `crane`, runs the Plan 01 ArchGate, verifies the signature, and only then fills the Plan 02 cache — so a verify failure publishes zero cache bytes (CONTAINER-03 runtime half).

## What Was Built

### Task 1 — `VerifyImage` (commit 3e4b0934)
- `VerifyImage(manifestBytes, bundleBytes []byte) error` — sigstore-go `NewSignedEntityVerifier(WithSignedTimestamps/WithTransparencyLog/WithIntegratedTimestamps)` + `NewShortCertificateIdentity(pinnedOIDCIssuer, "", "", pinnedSANRegexLiteral)` over `WithArtifact(manifestBytes)`. The signature takes `[]byte` (not file paths, unlike the upgrade analog) because the manifest comes from crane in pull.go.
- `pinnedOIDCIssuer = "https://token.actions.githubusercontent.com"` (unchanged from upgrade); `pinnedSANRegexLiteral` RE-PINNED to `^https://github\.com/agenthands/helix/\.github/workflows/bench-mirror\.yml@refs/heads/main$`.
- Single canonical `errors.New("signature verification FAILED")` at all 5 failure branches — **no oracle** (Pitfall 4). No Rekor-unreachable exception (unlike upgrade) since there is no offline-self-upgrade UX to preserve here.
- `testTrustedRootOverride`/`currentTrustedRoot()` seam; **no embedded production trust root this phase** (deferred to Plan 04's live mirror).
- VirtualSigstore (`//go:build fixturegen`) fixture generator + committed trust root and canonical/wrong-org/wrong-issuer bundles + sample manifest.
- `verify_test.go`: accept-canonical / reject-tampered / reject-wrong-org / reject-wrong-issuer / reject-unsigned, an explicit **error-text-identity** regression (tampered ≡ wrong-org ≡ wrong-issuer ≡ unsigned ≡ no-trust-root-parse), and an in-test comment-stripped grep-coverage assertion (≥5 canonical sites).

### Task 2 — `VerifyThenPull` (commit 5490b5c7)
- `VerifyThenPull(ctx, repo, digest, opts...) (hit, dir, err)` orders: `isHexSHA256`/`isValidRepo` fail-closed → daemon-free `crane.Config` arch inspect → `ArchGate(hostArch, manifestArch)` (Pitfall 5, **before** any verify/pull) → `Ensure(digest, fetch)` where the fetch closure fetches manifest+bundle, calls `VerifyImage`, and **only on verify success** pulls layers into the staging dir.
- **Verify gates the cache fill (Pitfall 3 TOCTOU):** a verify failure returns the canonical error so `Ensure` removes the staging dir and publishes no `.container-cache-ok` — proven hermetically by `TestVerifyThenPullVerifiesBeforeCacheFill` (asserts call order `arch→fetchMeta→verify`, pull never runs, no cache dir created).
- Unexported `Option` test seams (`withHostArch/withArchFn/withFetchMetaFn/withVerifyFn/withPullFn/withEnsureFn`) so hermetic tests drive ordering/arch-gate without a live registry; production callers get the real crane/`VerifyImage`/`Ensure` defaults. Live fetch/pull deferred to Plan 04 (fail-closed `notWiredFetchMeta`/`notWiredPull` sentinels until then).
- `pull_test.go`: verify-before-cache-fill, happy-path order+sentinel, arch-gate refusal (only `arch` touched), arch-mismatch override, cache-hit skip (no verify/fetch/pull), bad-digest/repo rejection, and the **sole** gated `TestVerifyThenPullLive` (SKIPs cleanly without `HELIX_BENCH_MIRROR`+engine; its hermetic siblings prove the verify-before-fetch branch — Pitfall 1, no skip-only-without-sibling).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `verify-no-docker-sdk` gate false-positives crane's docker-credential-helpers**
- **Found during:** Task 2 (after `go get` pulled crane's dependency closure into the build graph).
- **Issue:** crane's default registry-auth keychain transitively requires `github.com/docker/docker-credential-helpers` (and `docker/cli`, `docker/distribution`). The CONTAINER-01/SC#1 gate `grep -q 'github.com/docker/docker' go.mod` matched `docker/docker-credential-helpers` as a substring — a false positive that would HARD-FAIL `make vet`. The banned artifact is the Docker **Engine SDK** (`github.com/docker/docker`), which these registry-credential helpers are not.
- **Fix:** Anchored the grep to `github\.com/docker/docker[[:space:]]` (the module path followed by the go.mod module/version separator) so only the Engine SDK line matches. Added a comment documenting the crane-keychain rationale.
- **Files modified:** Makefile.
- **Commit:** 5490b5c7.

### Plan-deviating choices (documented in `decisions` frontmatter)
- **Fixture build tag `fixturegen` instead of `ignore`** (plan said `-tags ignore`): Go 1.26 `go run -tags ignore` re-enables `//go:build ignore` gen-tool files across the stdlib and x/* modules, producing an import-cycle cascade. A unique tag isolates the generator. The plan's `<verify>` command was adjusted accordingly.
- **crane added via pinned `go get ...@v0.20.7`, not `go mod tidy`** — full tidy is blocked by a pre-existing unrelated `github.com/google/s2a-go` resolution failure (the same blocker Phase 83-01 hit and deferred). The pinned `go get` added only the needed closure (8 go.mod lines, 14 go.sum) and kept crane at the existing v0.20.7.

## Threat Model Coverage
All `mitigate` dispositions in the plan's STRIDE register are satisfied: T-84-03-01 (sigstore-go keyless verify, no hand-rolled chain), T-84-03-02 (single canonical error + error-text-identity test + grep coverage), T-84-03-03 (pinned issuer + SAN regex; wrong-org/wrong-issuer fixtures rejected), T-84-03-04 (verify gates Ensure; verify failure leaves no cache bytes), T-84-03-05 (crane arch inspect + ArchGate before verify/pull). T-84-03-SC (`accept`): no new install requiring a human checkpoint — crane and sigstore-go were already in go.sum.

## Known Stubs
- `notWiredFetchMeta` / `notWiredPull` (pull.go) are intentional fail-closed defaults: the live GHCR manifest+bundle fetch and the crane.Pull layer export land with the Plan 04 mirror. They return distinct sentinels (not silent success), so an un-wired production `VerifyThenPull` fails closed rather than skipping verification or publishing unverified bytes. The gated live test injects real collaborators; the hermetic tests inject recorders. This is the documented Plan-03/Plan-04 split (CONTAINER-03 runtime half vs CI publish half).

## Verification
- `go build ./...` — PASS.
- `go vet ./bench/container/...` — PASS.
- `go test ./bench/container/ -count=1` — PASS (verify + pull hermetic tests; live test SKIPs).
- `make vet` — PASS (includes the fixed `verify-no-docker-sdk` gate + all 6 custom vettools).
- `grep -Eq 'github\.com/docker/docker[[:space:]]' go.mod` — no match (Engine SDK absent).
- Comment-stripped `grep -c 'signature verification FAILED' verify.go` = 5 (≥5 failure branches).

## Self-Check: PASSED
- All 10 created files present on disk.
- Both task commits (3e4b0934, 5490b5c7) found in git history.
