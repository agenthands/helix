# Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard - Research

**Researched:** 2026-06-21
**Domain:** Container orchestration via `os/exec`, sigstore/cosign image verification, cross-platform disk-space detection, Go import-boundary vet gates
**Confidence:** HIGH

## Summary

This phase builds the container substrate for **public benchmarks only** (SWE-bench / Multi-SWE-bench / Terminal-Bench run inside per-instance images in Phase 87). The work is four loosely-coupled, individually unit-testable pieces: (1) an `os/exec`-driven docker/podman engine shim with arch-mismatch refusal, (2) a SHA256-digest-pinned image cache mirroring the existing `bench/ragindex` cache convention, (3) cosign verification of a `ghcr.io/agenthands/helix-bench-*` mirror reusing the Phase 58 sigstore-go path, and (4) a pre-flight disk-budget guard.

The single most important codebase discovery is that **Helix already verifies cosign keyless signatures with the `sigstore/sigstore-go` Go library** (`internal/upgrade/verify.go`), NOT a `cosign` CLI shell-out. `sigstore/sigstore-go` (v1.1.4) and `google/go-containerregistry` (v0.20.7, `pkg/crane` + `pkg/v1/remote`) are **already in `go.mod`** — both the registry-pull-by-digest and the signature-verify primitives are available in-process with zero new direct dependencies and **no requirement for `cosign` on PATH**. The `$HELIX_CACHE_DIR` precedence function (`bench/ragindex/cache.go:cacheDir()`) and the HELIX_BIN `t.Skip` gating pattern are both established and must be reused verbatim.

**Primary recommendation:** Build a new leaf package `bench/container/` (engine shim + digest cache + arch gate + disk guard) plus a `bench/container/verify.go` that reuses `internal/upgrade`'s sigstore-go verifier pattern adapted to OCI image bundles. Drive docker/podman by `os/exec` only; pull-by-digest and verify in-process via the already-vendored libraries; gate all live-daemon/live-registry tests behind a `engineAvailable()` + `cosignBundleAvailable()` skip, exactly like `baseline_rag_test.go`'s HELIX_BIN gate. Add a `cmd/vet-docker-sdk-leakage` analyzer (optional, see Open Q5) and a `go.mod` grep gate in the Makefile for SC#1.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| docker/podman engine detection + CLI surface | Bench harness (`bench/container/`) | — | Pure `os/exec` subprocess control; belongs with bench, not kernel |
| Image pull by SHA256 digest | Bench harness | go-containerregistry `crane` | Either `docker pull repo@sha256:…` (os/exec) or `crane.Pull` in-process |
| Digest-pinned image cache layout | Bench harness | — | Mirrors `bench/ragindex` cache exactly; leaf package |
| Arch-mismatch refusal gate | Bench harness | `runtime.GOARCH` | Pure decision logic on host arch vs `.Architecture` |
| Cosign signature verification | Bench harness (`verify.go`) | `sigstore/sigstore-go` (reuse `internal/upgrade` pattern) | In-process Go-library verify; no CLI |
| GHCR mirror publish | CI (`.github/workflows/`) | cosign CLI in CI | Publish/sign is a CI job; reuses Phase 58 cosign-installer + OIDC |
| Disk-budget guard | Bench harness | `golang.org/x/sys/unix` + `x/sys/windows` | Cross-platform Statfs / GetDiskFreeSpaceEx |
| Container-backed cell dispatch | Bench harness (`bench/runtime/cell.go` `cfg.Agent` switch) | — | Phase 87 hook-in; this phase only builds the substrate |

**Tier note:** Publishing+signing the mirror is a CI responsibility (a GitHub Actions job, like Phase 58's release-merge job). VERIFYING the mirror before pulling is a bench-harness runtime responsibility (in-process Go). Keep these two strictly separate — the planner must not put `cosign sign` in Go runtime code.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/exec` (stdlib) | go1.25 | Drive `docker`/`podman` CLI | SC#1 forbids the Docker Go SDK; CLI shell-out is the mandated path `[VERIFIED: CONTEXT.md SC#1 + go.mod grep]` |
| `github.com/sigstore/sigstore-go/pkg/{bundle,root,verify}` | v1.1.4 | In-process cosign keyless verify | Already used by `internal/upgrade/verify.go`; zero new deps `[VERIFIED: go.mod:22 + internal/upgrade/verify.go]` |
| `github.com/google/go-containerregistry/pkg/{crane,v1/remote}` | v0.20.7 | Pull image by digest, inspect manifest/config (`.Architecture`), resolve tag→digest | Already in go.mod (indirect — promote to direct); pure-Go, no daemon needed `[VERIFIED: go.mod:129 + ~/go/pkg/mod listing]` |
| `golang.org/x/sys/unix` | v0.42.0 | `Statfs` for available disk (linux/darwin) | Already direct dep; `Statfs_t.Bavail * Bsize` = bytes free `[VERIFIED: go.mod:53 + go doc unix.Statfs_t]` |
| `golang.org/x/sys/windows` | v0.42.0 | `GetDiskFreeSpaceEx` for available disk (windows) | Same module, ships `zsyscall_windows.go` wrapper `[VERIFIED: x/sys@v0.42.0/windows listing]` |
| `runtime` (stdlib) | go1.25 | `runtime.GOARCH` host-arch detection | Established repo pattern (`internal/langregistry/installer.go:70`) `[VERIFIED: grep runtime.GOARCH]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/tools/go/analysis/singlechecker` | (in tree) | Wrap a `go.mod`/import vet analyzer | If a static docker-SDK-leakage gate is added (Open Q5) — mirrors `cmd/vet-bench-rag-leakage` `[VERIFIED: cmd/vet-bench-rag-leakage/main.go]` |
| `crypto/sha256` + `encoding/hex` (stdlib) | go1.25 | Digest handling / cache keys | Mirror `bench/ragindex/cache.go` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `crane.Pull` (in-process registry) | `docker pull repo@sha256:…` via os/exec | os/exec needs a live docker/podman daemon (CI-absent); crane pulls from the registry directly with NO daemon — better for hermetic verify+cache, but the eventual *run* still needs an engine. **Recommendation: use crane/remote for digest resolution + manifest arch inspection + the verify-before-pull cache fill (daemon-free, unit-testable); use os/exec `docker run` only at actual cell-execution time (Phase 87).** |
| sigstore-go (Go lib) verify | `cosign verify` CLI shell-out | CLI requires cosign on PATH (CI-absent, extra gating). The Go lib is already wired in `internal/upgrade` and needs no external binary. **Recommendation: Go lib for runtime verify; cosign CLI only in the CI publish job.** |
| `x/sys` Statfs | `syscall.Statfs_t` (stdlib) | stdlib `syscall` has identical Statfs_t on unix but NO Windows GetDiskFreeSpaceEx wrapper; `x/sys` covers both and is already a direct dep. **Recommendation: x/sys.** |

**Installation:** No `go get` needed for runtime — all libraries are already in `go.mod`. Promote `google/go-containerregistry` from `// indirect` to a direct require (it becomes directly imported). Verify:
```bash
go list -m github.com/google/go-containerregistry github.com/sigstore/sigstore-go golang.org/x/sys
```

**Version verification (performed this session):**
- `github.com/google/go-containerregistry v0.20.7` `[VERIFIED: go list -m]`
- `github.com/sigstore/sigstore-go v1.1.4` `[VERIFIED: go.mod:22]`
- `golang.org/x/sys v0.42.0` `[VERIFIED: go.mod:53]`

## Package Legitimacy Audit

> All packages are already present in the Helix `go.mod` (verified in-tree, not discovered via WebSearch). No new external package is introduced. The audit below confirms the three load-bearing modules.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/google/go-containerregistry` | Go proxy | ~6 yrs | very high (k8s ecosystem) | github.com/google/go-containerregistry | OK | Approved (already in go.mod, promote to direct) |
| `github.com/sigstore/sigstore-go` | Go proxy | ~2 yrs | high (sigstore) | github.com/sigstore/sigstore-go | OK | Approved (already in-tree, used by internal/upgrade) |
| `golang.org/x/sys` | Go proxy | core | core | go.googlesource.com/sys | OK | Approved (already direct dep) |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*Note: the Go ecosystem pins by module path + version + go.sum hash; the slopsquat vector is materially lower than npm/PyPI. All three are battle-tested CNCF/Google/Go-team modules already locked in go.sum.*

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────────────────────┐
   bench run (Phase 87)  │            bench/container/  (NEW)            │
   ─────────────────────▶│                                              │
   instance image ref    │  1. engine.Detect()  ──▶ docker | podman    │
   (repo + sha256)       │       (exec.LookPath; "" ⇒ degraded/skip)   │
                         │                                              │
                         │  2. DiskGuard.Check(path, 50GiB)            │
                         │       unix.Statfs / win GetDiskFreeSpaceEx  │
                         │       fail-closed ⇒ one-line remediation    │
                         │                                              │
                         │  3. ArchGate(hostGOARCH, manifest.Arch)     │
                         │       mismatch ⇒ refuse unless              │
                         │       BENCH_ARCH_MISMATCH_OK=1              │
                         │                                              │
   ghcr.io/agenthands/   │  4. Verify(ref@sha256) ───┐                 │
   helix-bench-*         │     sigstore-go bundle    │ FAIL ⇒ refuse   │
   (cosign keyless)  ◀───┼─────  fetch + verify  ◀───┘ (canonical msg) │
                         │           │ PASS                            │
                         │           ▼                                 │
   $HELIX_CACHE_DIR/     │  5. ImageCache.Ensure(sha) ── hit ⇒ reuse  │
   bench-images/<sha>/ ◀─┼─────  crane.Pull (daemon-free) OR          │
                         │        docker pull repo@sha256 (run-time)   │
                         │           │                                 │
                         │           ▼                                 │
                         │  6. (Phase 87) engine.Run(image) ─▶ cell    │
                         └─────────────────────────────────────────────┘
                                         │
                                         ▼
                    bench/runtime/cell.go  RunCell  cfg.Agent switch
                    (Phase 87 adds a "container" case here)
```

The verify step (4) gates the cache-fill (5): a tampered/unsigned image is rejected BEFORE any bytes land in the cache. Disk guard (2) and arch gate (3) are pre-flight refusals that run before any network pull.

### Recommended Project Structure
```
bench/container/                  # NEW leaf package
├── container.go                  # package doc + public surface (Engine, ImageRef)
├── engine.go                     # Detect() docker|podman; exec.CommandContext wrappers
├── engine_test.go                # detection via fake PATH; hermetic
├── arch.go                       # ArchGate: host GOARCH vs manifest .Architecture
├── arch_test.go                  # table-driven mismatch + BENCH_ARCH_MISMATCH_OK escape
├── cache.go                      # cacheDir() COPIED from ragindex; bench-images/<sha>/
├── cache_test.go                 # cache-hit-on-rerun; HELIX_CACHE_DIR override
├── disk.go                       # DiskGuard.Check; build-tagged unix/windows impls
├── disk_unix.go                  # //go:build !windows  — unix.Statfs
├── disk_windows.go               # //go:build windows    — GetDiskFreeSpaceEx
├── disk_test.go                  # synthetic low-disk via injectable availFn
├── verify.go                     # sigstore-go OCI verify (mirror internal/upgrade)
├── verify_test.go                # tampered/unsigned ⇒ canonical refusal
└── pull.go                       # crane.Pull-by-digest + verify-before-pull wiring
```

### Pattern 1: Engine detection + os/exec CLI surface (SC#1)
**What:** Detect docker or podman on PATH, normalize to a single struct, never link the Docker SDK.
**When to use:** Every container operation.
**Example:**
```go
// Source: pattern from bench/runtime/subprocess/ragserver.go:161 (exec.CommandContext)
//         + internal/eval/agent/claude.go + bench/evaluators/patch_validator/patch_validator.go:168
type Engine struct{ bin string } // "docker" or "podman"

func Detect() (*Engine, error) {
    for _, name := range []string{"docker", "podman"} {
        if p, err := exec.LookPath(name); err == nil {
            return &Engine{bin: p}, nil
        }
    }
    return nil, errEngineUnavailable // tests SKIP on this, like HELIX_BIN gate
}

func (e *Engine) PullByDigest(ctx context.Context, repo, digest string) error {
    // digest pinning: repo@sha256:<hex>, NEVER a tag (CONTAINER-02)
    ref := fmt.Sprintf("%s@sha256:%s", repo, digest)
    cmd := exec.CommandContext(ctx, e.bin, "pull", ref) // fixed argv
    return cmd.Run()
}
```

### Pattern 2: Cache layout reused verbatim from `bench/ragindex`
**What:** Resolve `$HELIX_CACHE_DIR` with three-step precedence; key by sha256; reject path escape.
**When to use:** Image cache root.
**Example:**
```go
// Source: bench/ragindex/cache.go:32-41 (cacheDir) — COPY this precedence exactly.
//   1. $HELIX_CACHE_DIR verbatim  2. os.UserCacheDir()/helix  3. ~/.helix/cache
const imagesSubdir = "bench-images"   // <cacheDir>/bench-images/<sha>/

func ImagePath(sha string) (string, error) {
    // sha is hex sha256 — no path separators, cannot escape root (T-83-01-03 mitigation)
    if !isHexSHA256(sha) { return "", errBadDigest }
    return filepath.Join(cacheDir(), imagesSubdir, sha), nil
}
```

### Pattern 3: In-process cosign verify (reuse Phase 58 sigstore-go)
**What:** Verify a keyless cosign bundle for an OCI image with pinned OIDC issuer + SAN regex, returning a single canonical failure string.
**When to use:** Before filling the cache from the GHCR mirror.
**Example:**
```go
// Source: internal/upgrade/verify.go:116-176 (VerifyArchive) — adapt artifact source
//         from os.ReadFile(archive) to the image manifest digest bytes.
// Pin (NEW for bench mirror): OIDC issuer = https://token.actions.githubusercontent.com
// SAN regex = ^https://github\.com/agenthands/helix/\.github/workflows/<mirror-publish>\.yml@refs/tags/...
v, err := verify.NewSignedEntityVerifier(tr,
    verify.WithSignedTimestamps(1),
    verify.WithTransparencyLog(1),
    verify.WithIntegratedTimestamps(1))
policy := verify.NewPolicy(
    verify.WithArtifact(bytes.NewReader(manifestBytes)),
    verify.WithCertificateIdentity(identity)) // pinned issuer + SAN regexp
_, err = v.Verify(bundle, policy) // err != nil ⇒ canonical "image signature verification FAILED"
```
**Critical:** copy the **single-canonical-error-message** discipline from `internal/upgrade/verify.go` (every failure branch returns the identical string so an attacker cannot distinguish tampered-vs-wrong-identity by error text — Pitfall 4).

### Pattern 4: Cross-platform disk guard with injectable availFn
**What:** Available-bytes detection split by build tag, with a test seam.
**Example:**
```go
// disk.go
type DiskGuard struct{ availFn func(path string) (uint64, error) } // injectable for tests
const MinFreeBytes = 50 * 1024 * 1024 * 1024 // 50 GiB

func (g DiskGuard) Check(path string) error {
    free, err := g.availFn(path)
    if err != nil { return err }
    if free < MinFreeBytes {
        return fmt.Errorf("bench refused: %d GiB free at %s, need 50 GiB — free space or set $HELIX_CACHE_DIR to a larger volume",
            free/(1<<30), path) // one-line remediation (CONTAINER-04)
    }
    return nil
}

// disk_unix.go  //go:build !windows
func availBytes(path string) (uint64, error) {
    var st unix.Statfs_t
    if err := unix.Statfs(path, &st); err != nil { return 0, err }
    return st.Bavail * uint64(st.Bsize), nil
}

// disk_windows.go  //go:build windows  → windows.GetDiskFreeSpaceEx
```

### Anti-Patterns to Avoid
- **Importing `github.com/docker/docker`:** hard SC#1 violation; the `go.mod` grep gate fails the build. Use `os/exec` (run-time) + `crane`/`remote` (daemon-free pull/inspect).
- **Shelling out to `cosign verify`:** adds a PATH dependency CI doesn't have. The sigstore-go Go path is already wired and needs no external binary.
- **Pinning images by tag:** CONTAINER-02 mandates `repo@sha256:<digest>`. A tag is mutable and defeats the verify.
- **Branching log output on verify outcome:** breaks the canonical-single-message security property (Pitfall 4 from `internal/upgrade`).
- **`os.Statfs` blocking tests on real disk:** inject `availFn` so the synthetic low-disk test is hermetic — do NOT require an actual full volume.
- **Filling the cache before verify:** verify MUST gate the pull; never write unverified bytes to `bench-images/<sha>/`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cosign keyless bundle verification | Custom Fulcio/Rekor/TSA chain validation | `sigstore/sigstore-go` (already in `internal/upgrade`) | Trust-root parsing, SET/inclusion-proof, TSA, SAN/issuer pinning are all subtle; reuse the audited path |
| Registry pull-by-digest / manifest inspect | Custom OCI registry HTTP client | `go-containerregistry` `crane`/`remote` | Auth, manifest media-types, multi-arch index handling are non-trivial; daemon-free |
| `$HELIX_CACHE_DIR` precedence | New env resolution | COPY `bench/ragindex/cache.go:cacheDir()` | Must stay byte-identical to the rag-index convention (CONTEXT requirement) |
| Cross-platform free-disk | `df` subprocess parsing | `x/sys/unix.Statfs` + `x/sys/windows.GetDiskFreeSpaceEx` | `df` output format varies; syscalls are deterministic and testable |
| Host arch detection | Parse `uname -m` | `runtime.GOARCH` | Compile-time correct; already the repo idiom (`installer.go:70`) |
| Import-boundary enforcement | Manual code review | `go/analysis` singlechecker (mirror `cmd/vet-bench-rag-leakage`) | Static gate runs every `make vet`; survives refactors |

**Key insight:** Three of the four success criteria are satisfied by libraries **already vendored for other features** (upgrade verification, x/sys). The phase is mostly *wiring + gating*, not new dependency surface.

## Common Pitfalls

### Pitfall 1: False-green tests when docker/podman/cosign/network absent
**What goes wrong:** Integration tests `t.Skip` silently in CI/dev, so `go test ./...` is green without exercising the real path (the exact MEMORY.md "bench smoke false-green" failure mode).
**Why it happens:** Live engine/registry are environment-dependent; the skip gate is correct but hides coverage.
**How to avoid:** Split logic so engine detection, digest validation, cache layout, arch gate, disk guard, and the verify *decision branch* are **hermetically unit-testable** (no daemon). Gate only the genuinely-live operations behind `engineAvailable()`/`cosignBundleAvailable()` skips, matching `baseline_rag_test.go:47`. Document in BENCH.md that the live container path needs `docker`/`podman` + `HELIX_BENCH_IMAGES` to actually run.
**Warning signs:** A `*_test.go` that ONLY has a `t.Skip` path with no hermetic sibling test.

### Pitfall 2: Tag-vs-digest drift
**What goes wrong:** Pinning by tag lets the registry serve a different image later, silently defeating the cosign verify and cache key.
**How to avoid:** Always `repo@sha256:<hex>`; validate the digest is 64 hex chars before use; the cache key IS the digest.
**Warning signs:** Any `repo:tag` string reaching `PullByDigest` or the cache path.

### Pitfall 3: Verify-then-pull TOCTOU / unverified bytes in cache
**What goes wrong:** Pulling first and verifying after leaves tampered bytes on disk if verify fails.
**How to avoid:** Verify the manifest signature, THEN pull into a temp dir, THEN atomically rename into `bench-images/<sha>/` only on success (mirror ragindex's atomic cache-fill discipline).
**Warning signs:** Cache write before the verify return.

### Pitfall 4: Distinguishable verify-failure error strings
**What goes wrong:** Different error text for "tampered" vs "wrong identity" leaks attacker-useful signal.
**How to avoid:** Single canonical failure literal at every branch — copy `internal/upgrade/verify.go`'s discipline (and its grep-coverage comment convention).
**Warning signs:** `fmt.Errorf` with verify-internal detail in the user-facing path.

### Pitfall 5: arm64 host pulling amd64 SWE-bench image silently emulates
**What goes wrong:** Docker/qemu silently runs amd64 under emulation on arm64 — slow and unrepresentative — masking an arch mismatch.
**How to avoid:** Inspect manifest `.Architecture` (via `crane`/`remote` config, daemon-free) and refuse when it != `runtime.GOARCH` unless `BENCH_ARCH_MISMATCH_OK=1` (SC#1). Test the escape-hatch env var both ways.
**Warning signs:** No arch comparison before run.

### Pitfall 6: Windows build breakage from unix-only Statfs
**What goes wrong:** `unix.Statfs` doesn't compile on windows; the 6-archive release matrix breaks.
**How to avoid:** Build-tag split `disk_unix.go` (`//go:build !windows`) / `disk_windows.go` (`//go:build windows`); CI builds windows/{amd64,arm64}, so cross-compile this package.
**Warning signs:** `golang.org/x/sys/unix` imported in a file without a build tag.

## Runtime State Inventory

> This is a greenfield feature phase (new `bench/container/` package + new GHCR namespace + new CI publish job). No rename/refactor of existing runtime state. The one cross-cutting state surface is the on-disk cache and the external GHCR registry.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | NEW cache dir `$HELIX_CACHE_DIR/bench-images/<sha>/` (mirrors existing `bench-rag-index/`). No migration of existing data. | New code path only |
| Live service config | NEW GHCR namespace `ghcr.io/agenthands/helix-bench-*` must be created and made public; CI needs `packages: write` + `id-token: write` (Phase 58 already grants id-token on the merge job). | CI workflow + GHCR package settings (manual one-time) |
| OS-registered state | None — verified by absence of task-scheduler/systemd registration in bench tree. | None |
| Secrets/env vars | NEW read-only env: `BENCH_ARCH_MISMATCH_OK` (escape hatch), reuses `HELIX_CACHE_DIR`. No new secret keys (cosign keyless = no private key). | Document in BENCH.md |
| Build artifacts | NEW optional vet binary `vet-docker-sdk-leakage` (if Open Q5 adopted) installed to `$GOPATH/bin`. | Add to Makefile `vet:` deps |

**Nothing found in category:** OS-registered state — verified by grep of bench/ for scheduler/systemd/launchd registration (none).

## Code Examples

### Resolve tag→digest and inspect arch daemon-free (verify-before-pull)
```go
// Source: github.com/google/go-containerregistry/pkg/crane (v0.20.7, in go.mod)
import "github.com/google/go-containerregistry/pkg/crane"

// Resolve a tag to its immutable digest (then pin to it forever after).
digest, err := crane.Digest("ghcr.io/agenthands/helix-bench-swe:instance-123")
// digest == "sha256:abc..." — store THIS, never the tag.

// Inspect platform without a docker daemon:
cfg, err := crane.Config("ghcr.io/agenthands/helix-bench-swe@" + digest)
// cfg is the raw config JSON; unmarshal to read .architecture (e.g. "amd64").
```

### HELIX_BIN-style availability skip (apply to engine + cosign)
```go
// Source: bench/runtime/baseline_rag_test.go:46-52 — copy this skip shape.
func requireEngine(t *testing.T) *container.Engine {
    t.Helper()
    e, err := container.Detect()
    if err != nil {
        t.Skip("no docker/podman on PATH; skipping live container test")
    }
    return e
}
```

### go.mod docker-SDK grep gate (SC#1, Makefile)
```makefile
# Source: pattern of Makefile verify-tos hard-fail gate (Makefile:305)
.PHONY: verify-no-docker-sdk
verify-no-docker-sdk:
	@if grep -q 'github.com/docker/docker' go.mod; then \
	  echo "::error::SC#1 violation: github.com/docker/docker present in go.mod"; exit 1; \
	fi
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Docker Go SDK (`github.com/docker/docker`) | `os/exec` to docker/podman CLI | This phase (SC#1) | No CGO/SDK bloat; podman drop-in for free |
| cosign CLI shell-out for verify | `sigstore/sigstore-go` in-process | Phase 58 (already done for upgrade) | No cosign-on-PATH requirement at runtime |
| minisign archive signing | sigstore keyless (Fulcio+Rekor+TSA) | Phase 58 D-02 cutover | Mirror reuses the same trust model |
| `df`/`uname` subprocess parsing | `x/sys` syscalls + `runtime.GOARCH` | This phase | Deterministic, testable, no shell |

**Deprecated/outdated:**
- Docker SDK: forbidden by SC#1.
- `cosign verify` CLI at runtime: avoid — use Go lib (CLI only in CI publish job).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (table-driven) `[VERIFIED: repo convention]` |
| Config file | none — `go test` |
| Quick run command | `go test ./bench/container/...` |
| Full suite command | `make test` (runs `vet` + `go test ./...`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CONTAINER-01 | No docker SDK in go.mod | static gate | `make verify-no-docker-sdk` (or grep in CI) | ❌ Wave 0 |
| CONTAINER-01 | docker OR podman detected on PATH | unit (fake PATH) | `go test ./bench/container -run TestDetect` | ❌ Wave 0 |
| CONTAINER-01 | arch mismatch refused unless BENCH_ARCH_MISMATCH_OK=1 | unit (table) | `go test ./bench/container -run TestArchGate` | ❌ Wave 0 |
| CONTAINER-01 | live pull works with docker/podman | integration-gated | `go test ./bench/container -run TestPullLive` (SKIP if no engine) | ❌ Wave 0 |
| CONTAINER-02 | digest-pinned ref (`repo@sha256:`) | unit | `go test ./bench/container -run TestDigestPin` | ❌ Wave 0 |
| CONTAINER-02 | cache layout + cache-hit on rerun | unit (HELIX_CACHE_DIR override) | `go test ./bench/container -run TestCacheHit` | ❌ Wave 0 |
| CONTAINER-03 | tampered/unsigned image rejected (verify decision) | unit (fixture bundle, mirror upgrade verify_test) | `go test ./bench/container -run TestVerify` | ❌ Wave 0 |
| CONTAINER-03 | live GHCR mirror verify before pull | integration-gated | `go test ./bench/container -run TestVerifyLive` (SKIP if no network/bundle) | ❌ Wave 0 |
| CONTAINER-03 | mirror published + signed | CI job | GitHub Actions publish workflow | ❌ Wave 0 |
| CONTAINER-04 | guard fails < 50 GiB w/ one-line remediation | unit (injected availFn) | `go test ./bench/container -run TestDiskGuard` | ❌ Wave 0 |

### Unit-testable vs Integration-gated (per success criterion)
| SC | Unit-testable (hermetic) | Integration-gated (SKIP cleanly) |
|----|--------------------------|----------------------------------|
| SC#1 | engine detection (fake PATH), arch-gate decision, go.mod grep | real `docker pull` |
| SC#2 | digest validation, cache path + cache-hit-on-rerun | real registry pull into cache |
| SC#3 | verify decision branch (fixture bundle, tampered ⇒ reject) | live GHCR fetch + verify; CI publish/sign |
| SC#4 | disk-guard threshold + remediation string (injected availFn) | (none — fully hermetic) |

**Critical (Pitfall 1):** every integration-gated test MUST have a hermetic sibling so `go test ./...` is not false-green. SC#4 is 100% hermetic. SC#1/2/3 each have a hermetic decision-layer test plus an env-gated live test.

### Sampling Rate
- **Per task commit:** `go test ./bench/container/...`
- **Per wave merge:** `make test` (vet gates + full suite)
- **Phase gate:** Full suite green + `make verify-no-docker-sdk` green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `bench/container/engine_test.go` — CONTAINER-01 detection (fake PATH dir w/ stub docker/podman)
- [ ] `bench/container/arch_test.go` — CONTAINER-01 arch-mismatch + escape hatch
- [ ] `bench/container/cache_test.go` — CONTAINER-02 cache-hit (set HELIX_CACHE_DIR to t.TempDir())
- [ ] `bench/container/verify_test.go` — CONTAINER-03 reject tampered (reuse upgrade testdata fixture-gen approach)
- [ ] `bench/container/disk_test.go` — CONTAINER-04 synthetic low-disk via injected availFn
- [ ] Makefile target `verify-no-docker-sdk` (SC#1 grep gate) + wire into `vet`/CI
- [ ] (optional) `cmd/vet-docker-sdk-leakage` + analyzer (Open Q5)

## Security Domain

> `security_enforcement` is not explicitly false in config — included. Supply-chain integrity is the core security concern of this phase.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — (GHCR pulls are anonymous for public packages) |
| V5 Input Validation | yes | Validate sha256 digest is 64-hex before path-join (cache-escape mitigation, T-83-01-03 pattern) |
| V6 Cryptography | yes | NEVER hand-roll signature verify — reuse `sigstore/sigstore-go`; pin OIDC issuer + SAN regex |
| V10 Malicious Code / Supply Chain | yes | cosign keyless verify before pull; pinned digest; canonical single-error-message |
| V12 File/Resource | yes | Atomic verified-then-rename cache fill; disk-budget guard prevents resource exhaustion |

### Known Threat Patterns for this phase
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Tampered/unsigned mirror image | Tampering | cosign verify (sigstore-go) before pull; reject on any failure |
| Tag remapping (mutable tag) | Tampering | pin by `@sha256:` digest, never tag |
| Path traversal via crafted "digest" | Tampering | `isHexSHA256` guard before filepath.Join (mirror ragindex) |
| Error-text oracle (tampered vs wrong-id) | Information Disclosure | single canonical failure string (copy internal/upgrade) |
| qemu silent cross-arch emulation | (correctness/integrity) | arch gate refuses mismatch unless BENCH_ARCH_MISMATCH_OK=1 |
| Disk exhaustion DoS on contributor laptop | Denial of Service | 50 GiB pre-flight disk-budget guard |
| Wrong-org signing identity | Spoofing | pin SAN regex to `github.com/agenthands/helix/.github/workflows/...` + OIDC issuer |

## Open Questions

1. **Should runtime image pull use `crane.Pull` (daemon-free) or `docker pull` (os/exec)?**
   - What we know: `crane`/`remote` are in go.mod and pull/inspect WITHOUT a daemon; `docker run` (the eventual Phase 87 cell execution) needs a real engine.
   - **RESOLVED — Recommendation:** Use `crane`/`remote` for digest resolution, manifest arch inspection, and the verify-before-pull cache fill (all daemon-free, unit-testable). Reserve `os/exec` `docker`/`podman` for actual container *execution* (Phase 87). This phase can deliver verify+cache entirely daemon-free; the os/exec engine shim ships now but its *run* path is exercised in Phase 87.

2. **Cosign verify in-process (sigstore-go) vs `cosign verify` CLI?**
   - **RESOLVED:** In-process sigstore-go — already wired in `internal/upgrade/verify.go`, no PATH dependency, hermetic fixture tests possible via the VirtualSigstore approach in `internal/upgrade/testdata/generate_fixtures.go`. Use the `cosign` CLI ONLY in the CI publish/sign job (mirrors Phase 58 release-merge).

3. **What SAN regex pins the mirror's signing identity?**
   - What we know: `internal/upgrade` pins `github.com/agenthands/helix/.github/workflows/release.yml@refs/tags/v…`.
   - **RESOLVED — Recommendation:** Add a dedicated mirror-publish workflow (e.g. `.github/workflows/bench-mirror.yml`) and pin the SAN regex to it. Decide tag-vs-branch ref shape at plan time; if the mirror is published on a schedule (not a tag), pin to `@refs/heads/main` with `--certificate-identity` (exact) rather than tag regex. Document the exact issuer (`https://token.actions.githubusercontent.com`) — same as upgrade.

4. **Does the GHCR mirror need to exist before this phase's tests pass?**
   - **RESOLVED:** No for unit tests (hermetic fixtures). The live `TestVerifyLive`/`TestPullLive` SKIP until the mirror namespace exists + is public. Creating + populating `ghcr.io/agenthands/helix-bench-*` is a CI/ops task that can land in the same phase's publish-workflow plan but gates only the live tests.

5. **Add a static `go vet` analyzer forbidding `github.com/docker/docker` imports?**
   - What we know: `cmd/vet-bench-rag-leakage` + `internal/lint/benchragleakage` is the established pattern; a `go.mod` grep already covers the direct-dep case.
   - **RESOLVED — Recommendation:** The `go.mod` grep gate (`verify-no-docker-sdk`) is sufficient for SC#1's literal acceptance (`grep "github.com/docker/docker" go.mod` empty). A full import-graph analyzer is OPTIONAL defense-in-depth (catches transitive linkage); recommend the grep gate as required, the analyzer as a nice-to-have if time permits. Keep scope tight.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `docker` or `podman` | Live container pull/run (Phase 87) | ✗ (dev/CI likely absent) | — | crane daemon-free pull for verify+cache; live tests SKIP |
| `cosign` CLI | CI publish/sign job ONLY | n/a at runtime | (CI: v2.4.1 via cosign-installer) | runtime uses sigstore-go Go lib — no CLI needed |
| GHCR network | Live mirror verify/pull | ✗ in hermetic CI | — | fixture bundles for unit tests; live tests SKIP |
| `golang.org/x/sys` | Disk guard | ✓ | v0.42.0 | — |
| `go-containerregistry` | Digest resolve / arch inspect | ✓ (go.mod indirect) | v0.20.7 | promote to direct require |
| `sigstore/sigstore-go` | In-process verify | ✓ | v1.1.4 | — |

**Missing dependencies with no fallback:** none — every required *unit-test* dependency is vendored; live engine/registry are gated SKIPs.
**Missing dependencies with fallback:** docker/podman (→ crane daemon-free for verify+cache; live run deferred to Phase 87), GHCR network (→ fixtures).

## Project Constraints (from CLAUDE.md)

- **Go single binary, no SDK bloat** — reinforces SC#1 (no Docker SDK); use os/exec + already-vendored libs.
- **Always run `go vet ./...` and `go test ./...` before completing.** — phase gate.
- **SMTC-first tool routing** — used during this research to locate the verify/sandbox seams.
- **GSD workflow enforcement** — edits go through GSD execute-phase.
- **No CGO requirement for new code** — `x/sys`, `crane`, `sigstore-go` are pure-Go; do NOT introduce CGO. (`modernc.org/sqlite` is the CGO-free precedent.)
- **Leaf-package discipline** (per `bench/ragindex` doc) — `bench/container/` should stay a leaf: import only stdlib + the three vendored libs, NEVER `internal/kernel`/`internal/semantic`. This makes a future vet gate trivial and keeps the package hermetically testable.

## User Constraints (from CONTEXT.md)

### Locked Decisions
None — discuss phase was skipped (`workflow.skip_discuss: true`).

### Claude's Discretion
All implementation choices are at Claude's discretion. Specifically called out in CONTEXT.md:
- Container engine via `os/exec` to `docker`/`podman` on PATH — NO `github.com/docker/docker` SDK (SC#1).
- Reuse the v1.10 Phase 58 cosign keyless attestation flow for the GHCR mirror; verify signatures before pulling.
- Gate live docker/cosign/GHCR integration tests behind availability checks that SKIP cleanly (HELIX_BIN pattern); unit logic must be hermetically testable.

### Deferred Ideas (OUT OF SCOPE)
None.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CONTAINER-01 | os/exec to docker/podman, no Docker SDK in go.mod | Engine detection pattern (Pattern 1), arch gate (Pattern 1 + Pitfall 5), go.mod grep gate (Code Examples), all sourced from existing exec.CommandContext idioms |
| CONTAINER-02 | Per-instance images pinned by SHA256 digest; cache at `$HELIX_CACHE_DIR/bench-images/<sha>/` | Cache layout reuses `bench/ragindex/cache.go:cacheDir()` verbatim (Pattern 2); digest pinning via `repo@sha256:` |
| CONTAINER-03 | cosign-signed GHCR mirror; verify before pull; reject tampered | In-process sigstore-go verify reusing `internal/upgrade/verify.go` (Pattern 3); CI publish reuses Phase 58 cosign-installer |
| CONTAINER-04 | Disk-budget guard fails < 50 GiB w/ remediation | Cross-platform Statfs/GetDiskFreeSpaceEx with injectable availFn (Pattern 4); fully hermetic test |

## Sources

### Primary (HIGH confidence)
- `internal/upgrade/verify.go` (in-tree) — sigstore-go keyless verify pattern, canonical-error discipline, OIDC/SAN pinning
- `bench/ragindex/cache.go` (in-tree) — `$HELIX_CACHE_DIR` precedence + sha-keyed cache layout + path-escape mitigation
- `bench/runtime/baseline_rag_test.go` (in-tree) — HELIX_BIN `t.Skip` availability-gating pattern
- `bench/runtime/subprocess/{daemon,ragserver}.go`, `internal/eval/sandbox/sandbox.go` — os/exec subprocess + StartDaemon seam; `cfg.Agent` switch in `bench/runtime/cell.go:543` (Phase 87 hook-in)
- `internal/lint/benchragleakage/analyzer.go` + `cmd/vet-bench-rag-leakage/main.go` — import-boundary vet-gate pattern
- `.github/workflows/release.yml:409-549` — Phase 58 cosign-installer (v2.4.1) + keyless OIDC sign job to reuse for mirror publish
- `go.mod` (verified via `go list -m`) — sigstore-go v1.1.4, go-containerregistry v0.20.7, x/sys v0.42.0 all present
- `go doc golang.org/x/sys/unix.Statfs_t` + x/sys windows zsyscall listing — disk APIs confirmed vendored

### Secondary (MEDIUM confidence)
- WebSearch (cosign keyless GHCR verify) — confirmed `--certificate-identity`/`--certificate-identity-regexp` + `--certificate-oidc-issuer` flag semantics and reject-unsigned behavior (CLI shape, for the CI publish reference only)

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library verified present in go.mod and exercised by existing in-tree code.
- Architecture: HIGH — all seams (cache, verify, sandbox, cell dispatch, vet gate) located and read directly.
- Pitfalls: HIGH — derived from existing in-tree security discipline (upgrade verify) and MEMORY.md false-green finding.
- CI publish identity (SAN regex shape): MEDIUM — depends on a plan-time decision (tag vs scheduled publish ref); resolved with a recommendation in Open Q3.

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable; in-tree anchors won't drift, vendored versions pinned in go.sum)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The mirror will be published via a GitHub Actions workflow analogous to release.yml (enabling keyless OIDC reuse) | Architecture / Open Q3 | If published by a different mechanism, the SAN-regex pin and Phase-58 reuse story change; verify approach still holds |
| A2 | `crane.Config`/`crane.Digest` resolve arch + digest without a docker daemon in this environment | Code Examples / Open Q1 | If network-restricted, live resolution SKIPs; unit logic unaffected |
| A3 | The 50 GiB threshold is GiB (binary), not GB (decimal) | Pattern 4 / CONTAINER-04 | Off-by-7% threshold; trivially adjustable, confirm units at plan time |

**Note:** A1–A3 are plan-time confirmables, none block the hermetic unit-test surface. All library/version/seam claims are `[VERIFIED]` against the live tree.
