---
phase: 84-container-runtime-cosign-signed-ghcr-mirror-disk-budget-guar
verified: 2026-06-21T00:00:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
  gaps_closed: []
  gaps_remaining: []
  regressions: []
deferred:
  - truth: "LIVE published-mirror signed-pull-accepted / tampered-pull-rejected confirmation (CONTAINER-03 operational half)"
    addressed_in: "Operator action after Phase 84 (workflow_dispatch of bench-mirror.yml from main + GHCR namespace set PUBLIC); ROADMAP-sanctioned phase boundary, not a later phase"
    evidence: "deferred-items.md [84-04] + 84-04-SUMMARY.md Task 2 (orchestrator-resolved APPROVED-WITH-DEFERRAL); runtime verify-before-pull decision logic ALREADY proven hermetically with VirtualSigstore fixtures in Plan 84-03"
---

# Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard Verification Report

**Phase Goal:** Container infra for public benchmarks — os/exec to docker (podman drop-in, NO docker SDK), SHA256-pinned per-instance images cached at `$HELIX_CACHE_DIR/bench-images/<sha>/`, cosign-signed GHCR mirror (`ghcr.io/agenthands/helix-bench-*`) verified before pull, and a pre-flight 50 GB disk-budget guard.
**Verified:** 2026-06-21
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (Success Criterion) | Status | Evidence |
|---|---------------------------|--------|----------|
| 1 | **CONTAINER-01** — os/exec to docker/podman, NO Engine SDK in go.mod; arch-mismatch refused unless `BENCH_ARCH_MISMATCH_OK=1` | ✓ VERIFIED | `grep -E 'github.com/docker/docker(\s\|$)' go.mod` empty (only allowed transitive `docker/cli`, `docker/distribution`, `docker-credential-helpers`); `make vet` (with `verify-no-docker-sdk` prerequisite) exits 0; `engine.go:29 Detect()` probes docker→podman via `exec.LookPath`, returns `errEngineUnavailable` when neither present; `arch.go:28 ArchGate` refuses mismatch, only exact "1" opens gate. Tests: `TestDetect*` (4), `TestArchGate*` (5), `TestPullByDigest*` (3) all PASS hermetically |
| 2 | **CONTAINER-02** — images pinned by SHA256 digest not tag; cache at `$HELIX_CACHE_DIR/bench-images/<sha>/`; cache-hit on re-run | ✓ VERIFIED | `cache.go:48 ImagePath` fail-closes via `isHexSHA256` BEFORE `filepath.Join` (path-escape guard), layout `<cacheDir>/bench-images/<sha>/`; `cacheDir()` copies the 3-step ragindex precedence with `HELIX_CACHE_DIR` verbatim; `engine.go:50` builds `repo@sha256:<hex>` never a tag; `Ensure` sentinel-gated cache hit. Tests: `TestImagePathUnderBenchImages`, `TestImagePathRejectsNonHex`, `TestCacheHitOnRerun`, `TestCacheHelixCacheDirPrecedence` PASS |
| 3 | **CONTAINER-03** — cosign-signed mirror verified BEFORE pull; tampered image rejected | ✓ VERIFIED (runtime half hermetic; live-mirror confirmation honestly DEFERRED) | `verify.go VerifyImage` pins issuer (`token.actions.githubusercontent.com`) + SAN regex; every failure branch returns byte-identical canonical `"signature verification FAILED"` (oracle-free); `pull.go:117-129 VerifyThenPull` runs `verifyFn` strictly before `pullFn` inside the `Ensure` fetch closure → verify failure publishes zero cache bytes. Real VirtualSigstore fixtures on disk (canonical/wrong-issuer/wrong-org bundles + trusted_root.json). SAN literal in `verify.go:49` byte-matches the SAN `bench-mirror.yml` documents minting (`@refs/heads/main`). Tests: `TestVerifyImageRejectsTampered/WrongOrgSAN/WrongIssuer/Unsigned`, `TestVerifyErrorTextIsIdenticalAcrossBranches`, `TestVerifyThenPullVerifiesBeforeCacheFill` PASS. Live `TestVerifyThenPullLive` SKIPs (namespace unpublished) — deferral honestly recorded |
| 4 | **CONTAINER-04** — disk-budget guard fails if < 50 GiB before SWE-bench full run; synthetic low-disk test trips it with one-line remediation | ✓ VERIFIED | `disk.go:9 MinFreeBytes = 50 * 1024^3` (GiB); `DiskGuard.Check` injectable `availFn` seam; refusal message is single-line with remediation. Build-tag split `disk_unix.go` (unix.Statfs) / `disk_windows.go` (GetDiskFreeSpaceEx) both cross-compile. Tests: `TestDiskGuardRefusesBelowThreshold`, `TestDiskGuardAllowsAtOrAboveThreshold`, `TestDiskGuardPropagatesAvailErr`, `TestDiskGuardRemediationIsOneLine` PASS |

**Score:** 4/4 truths verified

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | CONTAINER-03 LIVE published-mirror confirmation (signed-pull-accepted / tampered-pull-rejected against the real GHCR namespace) | Operator one-time action after Phase 84 (ROADMAP-sanctioned phase boundary) | `deferred-items.md` [84-04] + `84-04-SUMMARY.md` Task 2 orchestrator-resolved APPROVED-WITH-DEFERRAL. The runtime verify-before-pull DECISION LOGIC is already proven hermetically with VirtualSigstore fixtures (Plan 84-03). Only the live published-mirror pull is outstanding — the namespace does not exist until an operator runs `bench-mirror.yml` and sets packages PUBLIC. This is NOT a gap: it is a published-state dependency outside the codebase |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `bench/container/container.go` | leaf doc + ImageRef + isHexSHA256 + errBadDigest + isValidRepo | ✓ VERIFIED | Present, substantive, wired; leaf invariant holds (no internal/kernel|semantic imports) |
| `bench/container/engine.go` | Detect + PullByDigest + pullArgs + allowlistEnv | ✓ VERIFIED | os/exec docker|podman, fixed argv with `--` separator, 3-key env allowlist, Setpgid (unix) |
| `bench/container/arch.go` | ArchGate + ArchGateForHost + BENCH_ARCH_MISMATCH_OK | ✓ VERIFIED | Pure decision fn, only exact "1" opens gate |
| `bench/container/cache.go` | cacheDir precedence + ImagePath + Ensure | ✓ VERIFIED | bench-images/<sha>/ layout, WR-01 idempotent publish (sentinel re-check + RemoveAll stale finalDir) |
| `bench/container/disk.go` (+unix/windows) | MinFreeBytes 50 GiB + injectable availFn | ✓ VERIFIED | Build-tag split, both platforms cross-compile |
| `bench/container/verify.go` | VerifyImage pinned issuer/SAN + canonical error | ✓ VERIFIED | Oracle-free literal at every branch, SAN matches workflow |
| `bench/container/pull.go` | VerifyThenPull verify-before-pull + ErrNotConfigured guard | ✓ VERIFIED | WR-03 fix: top-of-fn `isNotWired` guard returns ErrNotConfigured before any verify, by function identity not error text |
| `.github/workflows/bench-mirror.yml` | cosign keyless sign-by-digest, refs/heads/main | ✓ VERIFIED | WR-02 fix: dest-repo digest selected by `grep "^${DEST_REPO}@"` not `index 0`; SHA-pinned actions; SAN coupling documented |
| `bench/container/testdata/*.sigstore.json` | VirtualSigstore fixtures | ✓ VERIFIED | canonical + wrong-issuer + wrong-org bundles + trusted_root.json on disk |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| Makefile `vet` | go.mod | `verify-no-docker-sdk` anchored grep gate | ✓ WIRED (prerequisite of `vet:`, anchored `[[:space:]]` excludes credential-helpers) |
| engine.go Detect | os/exec.LookPath | docker|podman PATH probe | ✓ WIRED |
| cache.go ImagePath | container.go isHexSHA256 | path-escape guard before Join | ✓ WIRED |
| pull.go VerifyThenPull | verify.go VerifyImage | verify-before-pull gating Ensure | ✓ WIRED (verifyFn called before pullFn inside fetch closure) |
| pull.go | crane.Config | daemon-free arch inspect | ✓ WIRED |
| verify.go pinnedSANRegexLiteral | bench-mirror.yml | SAN identity (@refs/heads/main) | ✓ WIRED (byte-identical literal) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full build | `go build ./...` | exit 0 | ✓ PASS |
| Container tests | `go test -count=1 ./bench/container/...` | ok, all pass | ✓ PASS |
| Full vet + SDK gate | `make vet` | exit 0 (verify-no-docker-sdk ran first) | ✓ PASS |
| Forbidden SDK absent | `grep -E 'github.com/docker/docker(\s\|$)' go.mod` | empty | ✓ PASS |
| Windows cross-compile (WR-04) | `GOOS=windows GOARCH=amd64 go build ./bench/container/...` | exit 0 | ✓ PASS |
| Live verify+pull | `TestVerifyThenPullLive` | SKIP (no engine/mirror) | ? SKIP (deferred, hermetic siblings cover decision branch) |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| CONTAINER-01 | 84-01 | ✓ SATISFIED | SDK ban gate + Detect + ArchGate (Truth 1) |
| CONTAINER-02 | 84-02 | ✓ SATISFIED | digest-pin + bench-images cache + cache-hit (Truth 2) |
| CONTAINER-03 | 84-03 (runtime), 84-04 (CI publish) | ✓ SATISFIED (live op deferred) | verify-before-pull hermetic + bench-mirror.yml sign-by-digest (Truth 3); live mirror confirmation ROADMAP-sanctioned deferral |
| CONTAINER-04 | 84-02 | ✓ SATISFIED | 50 GiB DiskGuard + synthetic low-disk test (Truth 4) |

All four CONTAINER-01..04 IDs declared in plan frontmatter `requirements` are accounted for in REQUIREMENTS.md (lines 93-96, all `[x]`). Note: REQUIREMENTS.md traceability table rows 197-200 still read "TBD | Pending" — a documentation lag, not a code gap (informational; does not affect goal achievement).

### Code-Review Warning Closure

| Warning | Fix | Verified |
|---------|-----|----------|
| WR-01 cache rename wedge | `Ensure` re-checks sentinel + RemoveAll stale finalDir before atomic rename | ✓ `TestEnsureSecondFillOverPreexistingValidFinalDir` + `TestEnsureConcurrentDoubleFillNoWedge` PASS |
| WR-02 bench-mirror digest selection | dest-repo digest by `grep "^${DEST_REPO}@"` not `index 0` | ✓ Confirmed in bench-mirror.yml:128-133 |
| WR-03 pull.go oracle | `ErrNotConfigured` guard by function identity before any verify | ✓ `TestVerifyThenPullNotConfiguredOnDefaults` PASS |
| WR-04 windows comment | overstated "sufficient" downgraded to KNOWN LIMITATION note | ✓ Confirmed in procgroup_windows.go:13-20; windows build clean |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none in modified source) | — | — | — | Debt markers (`TBD`) appear only in REQUIREMENTS.md traceability table (pre-existing planning doc), not in phase-modified Go source. No FIXME/XXX/HACK/PLACEHOLDER in `bench/container/*.go`. The `notWired*` stubs are intentional, guarded by `ErrNotConfigured`, and documented as Plan-04-deferred live wiring (review IN-03) |

### Human Verification Required

None required for phase goal acceptance. The runtime verify-before-pull decision logic is hermetically proven. The live published-mirror confirmation is a deferred operator action (recorded in `deferred-items.md`), not a phase-completion gate — it depends on the GHCR namespace existing, which is outside this codebase.

### Gaps Summary

No gaps. All four success criteria (CONTAINER-01..04) are achieved and proven against the live codebase:
- The forbidden Docker Engine SDK is absent and statically gated; docker/podman detection and arch-refusal work hermetically.
- Images are digest-pinned into `$HELIX_CACHE_DIR/bench-images/<sha>/` with a path-escape guard and a verified cache-hit.
- Cosign verification runs strictly before any cache bytes land, with oracle-free canonical errors and VirtualSigstore-fixture-proven rejection of tampered/unsigned/wrong-identity images; the CI publish/sign workflow signs by dest-repo digest with a SAN that byte-matches the runtime pin.
- The 50 GiB disk-budget guard refuses below-threshold with a one-line remediation, proven via an injectable synthetic seam.

All four code-review warnings (WR-01..04) are fixed without SC regression. `go build ./...`, `go test -count=1 ./bench/container/...`, and `make vet` all pass when run by the verifier. The single live test SKIP is a legitimate, honestly-recorded, ROADMAP-sanctioned deferral (namespace not yet published) with full hermetic sibling coverage of the decision branch.

---

_Verified: 2026-06-21_
_Verifier: Claude (gsd-verifier)_
