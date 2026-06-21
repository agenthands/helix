# Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 14 new + 2 modified
**Analogs found:** 14 / 16

## File Classification

All new code lives in a NEW leaf package `bench/container/` (import only stdlib + `sigstore-go` + `go-containerregistry` + `x/sys`; NEVER `internal/kernel` / `internal/semantic`).

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/container/container.go` | utility (package doc + public surface) | — | `bench/ragindex/cache.go` (leaf-pkg doc) | exact (role) |
| `bench/container/engine.go` | service (engine shim) | request-response (os/exec) | `bench/runtime/subprocess/ragserver.go` | role-match |
| `bench/container/engine_test.go` | test | — | `bench/runtime/baseline_rag_test.go` (skip gate) | role-match |
| `bench/container/arch.go` | utility (decision gate) | transform | `internal/langregistry/installer.go:70` (GOARCH) | partial |
| `bench/container/arch_test.go` | test | — | upgrade `verify_test.go` table style | role-match |
| `bench/container/cache.go` | utility (cache layout) | file-I/O | `bench/ragindex/cache.go` | exact |
| `bench/container/cache_test.go` | test | — | `baseline_rag_test.go` (HELIX_CACHE_DIR t.TempDir override) | exact |
| `bench/container/disk.go` | utility (disk guard) | transform | research Pattern 4 (new; no in-tree analog) | no analog |
| `bench/container/disk_unix.go` | utility (//go:build !windows) | file-I/O (syscall) | — | no analog |
| `bench/container/disk_windows.go` | utility (//go:build windows) | file-I/O (syscall) | — | no analog |
| `bench/container/disk_test.go` | test | — | injected `availFn` (research Pattern 4) | role-match |
| `bench/container/verify.go` | service (cosign verify) | request-response | `internal/upgrade/verify.go` | exact |
| `bench/container/verify_test.go` | test | — | `internal/upgrade/verify_test.go` + `testdata/generate_fixtures.go` | exact |
| `bench/container/pull.go` | service (verify-then-pull) | streaming (registry) | go-containerregistry `crane` (research Code Examples) | role-match |
| `Makefile` (`verify-no-docker-sdk` target + wire into `vet`) | config | — | `Makefile:71/305` (vettool + verify-tos gate) | exact |
| `.github/workflows/bench-mirror.yml` | config (CI publish) | event-driven | `.github/workflows/release.yml:390-549` | exact |

Optional (Open Q5, defense-in-depth only): `cmd/vet-docker-sdk-leakage/main.go` + `internal/lint/dockersdkleakage/analyzer.go` — analog `cmd/vet-bench-rag-leakage/main.go` (exact, 13-line wrapper).

## Pattern Assignments

### `bench/container/cache.go` (utility, file-I/O)

**Analog:** `bench/ragindex/cache.go` — COPY the `cacheDir()` precedence VERBATIM (CONTEXT requirement: must stay byte-identical to the rag-index convention).

**Cache-root precedence + leaf-pkg doc** (`bench/ragindex/cache.go:1-41`):
```go
// Package container ... LEAF package: imports only stdlib + sigstore-go +
// go-containerregistry + x/sys, never internal/kernel or internal/semantic.

const cacheDirEnv = "HELIX_CACHE_DIR"
const imagesSubdir = "bench-images" // <cacheDir>/bench-images/<sha>/

// cacheDir resolves the cache root with a three-step precedence:
//  1. $HELIX_CACHE_DIR (verbatim) if set
//  2. os.UserCacheDir()/helix
//  3. ~/.helix/cache (last-resort fallback)
func cacheDir() string {
	if d := os.Getenv(cacheDirEnv); d != "" {
		return d
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "helix")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".helix", "cache")
}
```

**Path-escape guard** (mirror `IndexPath` at `bench/ragindex/cache.go:84-90`): the sha MUST be validated hex-sha256 (no path separators) BEFORE `filepath.Join` (T-83-01-03 mitigation). The new code adds an explicit `isHexSHA256(sha)` check that ragindex gets for free (its key is computed, not caller-supplied):
```go
func ImagePath(sha string) (string, error) {
	if !isHexSHA256(sha) { return "", errBadDigest }
	return filepath.Join(cacheDir(), imagesSubdir, sha), nil
}
```

---

### `bench/container/verify.go` (service, request-response)

**Analog:** `internal/upgrade/verify.go` — the load-bearing analog. Adapt the artifact source from `os.ReadFile(archivePath)` to the OCI image manifest digest bytes; keep the verifier construction, OIDC/SAN pinning, and single-canonical-error discipline identical.

**Imports** (`internal/upgrade/verify.go:3-14`):
```go
import (
	"bytes"
	"errors"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)
```

**Pinned identity** (`internal/upgrade/verify.go:41,59` — CHANGE the SAN to the new mirror workflow; KEEP the issuer):
```go
const pinnedOIDCIssuer = "https://token.actions.githubusercontent.com" // unchanged
// NEW SAN regex pins to bench-mirror.yml (Open Q3). If published on a schedule
// (not a tag), pin to @refs/heads/main with an EXACT identity instead of a tag regex:
const pinnedSANRegexLiteral = `^https://github\.com/agenthands/helix/\.github/workflows/bench-mirror\.yml@refs/...$`
```

**Verifier construction + verify call** (`internal/upgrade/verify.go:147-175`):
```go
v, err := verify.NewSignedEntityVerifier(tr,
	verify.WithSignedTimestamps(1),
	verify.WithTransparencyLog(1),
	verify.WithIntegratedTimestamps(1))
// ...
policy := verify.NewPolicy(
	verify.WithArtifact(bytes.NewReader(manifestBytes)), // CHANGED: was archiveBytes
	verify.WithCertificateIdentity(identity))
if _, err := v.Verify(b, policy); err != nil {
	// canonical: signature verification FAILED — see Pitfall 4.
	return errors.New("signature verification FAILED")
}
```

**CRITICAL — single-canonical-error discipline** (`internal/upgrade/verify.go:16-37,116-176`): EVERY failure branch returns the identical literal `errors.New("signature verification FAILED")` with an inline `// canonical: ... — see Pitfall 4.` comment so a `grep -c` gate confirms coverage. Do NOT branch error text on verify outcome (Pitfall 4 — error-text oracle). The Rekor-unreachable branch (`verify.go:168-171`, `isRekorUnreachable` at `:200-233`) is the SOLE exception that wraps the literal with network-hint wording.

**Test-override seam** (`internal/upgrade/verify.go:61-85`): `testTrustedRootOverride []byte` + `currentTrustedRoot()` let `verify_test.go` substitute a `VirtualSigstore`-generated trust root (see `internal/upgrade/testdata/generate_fixtures.go`) without touching the embedded production root. Copy this seam so `verify_test.go` can drive tampered/unsigned/wrong-identity fixtures hermetically (no network).

---

### `bench/container/engine.go` (service, request-response via os/exec)

**Analog:** `bench/runtime/subprocess/ragserver.go:150-189` — the `exec.CommandContext` + fixed-argv + strict-env subprocess shape. NO `github.com/docker/docker` SDK (SC#1).

**Fixed-argv exec wrapper** (`subprocess/ragserver.go:161-189`):
```go
cmd := exec.CommandContext(ctx, ragServerBin, "--corpus="+corpusRoot) // fixed argv, no shell
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own process group for group-kill
// strict env allowlist (do NOT inherit full env):
env := []string{}
for _, k := range []string{"PATH", "HOME", "HELIX_CACHE_DIR"} {
	if v := os.Getenv(k); v != "" { env = append(env, k+"="+v) }
}
cmd.Env = env
```

**Detection + digest-pin** (research Pattern 1, sourced from this exec idiom):
```go
type Engine struct{ bin string } // "docker" or "podman"
func Detect() (*Engine, error) {
	for _, name := range []string{"docker", "podman"} {
		if p, err := exec.LookPath(name); err == nil { return &Engine{bin: p}, nil }
	}
	return nil, errEngineUnavailable // live tests SKIP on this (baseline_rag_test gate)
}
func (e *Engine) PullByDigest(ctx context.Context, repo, digest string) error {
	ref := fmt.Sprintf("%s@sha256:%s", repo, digest) // NEVER a tag (CONTAINER-02; Pitfall 2)
	return exec.CommandContext(ctx, e.bin, "pull", ref).Run()
}
```

---

### `bench/container/arch.go` (utility, transform)

**Analog:** `internal/langregistry/installer.go:70` (`runtime.GOOS + "-" + runtime.GOARCH`) — use `runtime.GOARCH`, never parse `uname -m`.

```go
// host runtime.GOARCH vs manifest .Architecture (from crane.Config, daemon-free).
// Mismatch ⇒ refuse unless BENCH_ARCH_MISMATCH_OK=1 (SC#1; Pitfall 5 — qemu
// silent cross-arch emulation). Test the escape-hatch env var both ways.
```

---

### `bench/container/pull.go` (service, streaming)

**Analog:** go-containerregistry `crane` (research Code Examples lines 298-310; v0.20.7 already in go.mod — promote to direct require).

```go
import "github.com/google/go-containerregistry/pkg/crane"
digest, err := crane.Digest("ghcr.io/agenthands/helix-bench-swe:instance-123") // tag→immutable digest
cfg, err := crane.Config("ghcr.io/agenthands/helix-bench-swe@" + digest)       // .architecture, daemon-free
```

**Verify-then-pull ordering (Pitfall 3 TOCTOU):** verify the manifest signature FIRST (`verify.go`), THEN `crane.Pull` into a temp dir, THEN atomically rename into `bench-images/<sha>/` only on success. Never write unverified bytes to the cache. Mirror ragindex's atomic verified-then-rename cache-fill discipline.

---

### `bench/container/disk.go` + `disk_unix.go` + `disk_windows.go` (utility, syscall)

**Analog:** none in-tree (no existing disk-space code). Follow research Pattern 4 exactly.

```go
// disk.go — injectable availFn test seam (Pitfall: hermetic low-disk test)
type DiskGuard struct{ availFn func(path string) (uint64, error) }
const MinFreeBytes = 50 * 1024 * 1024 * 1024 // 50 GiB (confirm GiB vs GB at plan time — A3)
func (g DiskGuard) Check(path string) error {
	free, err := g.availFn(path)
	if err != nil { return err }
	if free < MinFreeBytes {
		return fmt.Errorf("bench refused: %d GiB free at %s, need 50 GiB — free space or set $HELIX_CACHE_DIR to a larger volume", free/(1<<30), path) // one-line remediation (CONTAINER-04)
	}
	return nil
}
// disk_unix.go  //go:build !windows  → unix.Statfs; return st.Bavail * uint64(st.Bsize)
// disk_windows.go //go:build windows → windows.GetDiskFreeSpaceEx
```
**Pitfall 6:** `golang.org/x/sys/unix` MUST NOT be imported in a file without a build tag — Windows release archives won't compile. Split unix/windows by `//go:build`.

---

### `Makefile` — `verify-no-docker-sdk` target (config)

**Analog:** `Makefile:71-72` (vettool install + `vet:` wiring) and `Makefile:305-310` (`verify-tos` hard-fail gate).

```makefile
.PHONY: verify-no-docker-sdk
verify-no-docker-sdk:
	@if grep -q 'github.com/docker/docker' go.mod; then \
	  echo "::error::SC#1 violation: github.com/docker/docker present in go.mod"; exit 1; \
	fi
```
Wire into the existing `vet:` aggregate target (`Makefile:47`) and/or CI. The optional analyzer (Open Q5) follows the `$(VETTOOL_BENCH_RAG_LEAKAGE)` install+dep pattern at `Makefile:71` if adopted.

---

### `.github/workflows/bench-mirror.yml` (config, CI publish — event-driven)

**Analog:** `.github/workflows/release.yml` — the Phase 58 cosign keyless publish flow.

**cosign-installer** (`release.yml:409-417`):
```yaml
- name: Install cosign
  uses: sigstore/cosign-installer@7e8b541eb2e61bf99390e1afd4be13a184e9ebc5  # v3.10.1
  with:
    cosign-release: 'v2.4.1'
```

**Job permissions** (`release.yml:390-392` — add `packages: write` for GHCR push):
```yaml
permissions:
  contents: read
  id-token: write   # Phase 58 D-02: cosign keyless OIDC (Fulcio short-lived cert)
  packages: write   # NEW: push images to ghcr.io/agenthands/helix-bench-*
```

**Keyless sign step shape** (`release.yml:532-549` — adapt `sign-blob` → image `cosign sign` of the pushed digest):
```yaml
- name: Cosign keyless sign (real tag/scheduled publish only)
  run: |
    cosign sign --yes "ghcr.io/agenthands/helix-bench-swe@${DIGEST}"
```
The runtime verify (`bench/container/verify.go`) MUST pin its SAN regex to THIS workflow's path (`bench-mirror.yml`) — keep publish-identity (CI) and verify-identity (Go runtime) consistent. Per research Tier note: NEVER put `cosign sign` in Go runtime code; signing is CI-only.

## Shared Patterns

### Single-canonical-error discipline (security)
**Source:** `internal/upgrade/verify.go:16-37` (invariant doc) + every return in `:116-176`.
**Apply to:** `bench/container/verify.go` — all verify failure branches return the identical literal with `// canonical:` comments for grep-coverage; never branch error text on outcome (Pitfall 4).

### HELIX_BIN-style availability skip (false-green prevention)
**Source:** `bench/runtime/baseline_rag_test.go:46-52`.
**Apply to:** every live test (`engine_test.go` live pull, `verify_test.go` live GHCR, `pull_test.go`). Each integration-gated test MUST have a hermetic sibling so `go test ./...` is not false-green (Pitfall 1, MEMORY.md bench-smoke finding).
```go
e, err := container.Detect()
if err != nil { t.Skip("no docker/podman on PATH; skipping live container test") }
```

### HELIX_CACHE_DIR test override
**Source:** `bench/runtime/baseline_rag_test.go` (sets `HELIX_CACHE_DIR` to `t.TempDir()`).
**Apply to:** `cache_test.go` — point `HELIX_CACHE_DIR` at `t.TempDir()` for a fresh, collision-free cache; assert cache-hit on rerun.

### Fixture-driven sigstore verify (VirtualSigstore)
**Source:** `internal/upgrade/testdata/generate_fixtures.go` + `testTrustedRootOverride` seam.
**Apply to:** `verify_test.go` — generate tampered / unsigned / wrong-identity bundles hermetically; no network, no cosign binary.

### Leaf-package import discipline
**Source:** `bench/ragindex/cache.go:1-6` package doc.
**Apply to:** all of `bench/container/` — stdlib + the three vendored libs only; never `internal/kernel` / `internal/semantic` / `bench/runtime`. Keeps the package hermetically testable and a future vet gate trivial.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `bench/container/disk.go` (+`_unix`/`_windows`) | utility | syscall | No existing disk-space / Statfs code in the tree; follow research Pattern 4 (`x/sys` syscalls, injectable availFn). Closest idiom is the build-tag split style used elsewhere, but no semantic analog exists. |

## Metadata

**Analog search scope:** `bench/`, `internal/upgrade/`, `internal/langregistry/`, `internal/lint/`, `cmd/vet-*`, `.github/workflows/`, `Makefile`
**Files scanned:** 9 read (verify.go, ragindex/cache.go, cell.go, ragserver.go, vet-bench-rag-leakage/main.go, release.yml, baseline_rag_test.go) + grep across Makefile/installer/workflows
**Pattern extraction date:** 2026-06-21
