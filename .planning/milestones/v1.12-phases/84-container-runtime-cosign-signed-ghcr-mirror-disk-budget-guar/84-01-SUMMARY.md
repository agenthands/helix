---
phase: 84
plan: 01
subsystem: bench/container
tags: [container, docker, podman, os-exec, arch-gate, vet-gate]
requires: []
provides:
  - "bench/container.ImageRef (Repo, Digest) digest-pinned image identity"
  - "bench/container.isHexSHA256 / errBadDigest digest guards"
  - "bench/container.Engine + Detect() docker|podman os/exec shim"
  - "bench/container.errEngineUnavailable sentinel (live-test skip signal)"
  - "bench/container.(*Engine).PullByDigest / pullArgs (argv seam)"
  - "bench/container.ArchGate / ArchGateForHost arch-mismatch refusal"
  - "BENCH_ARCH_MISMATCH_OK env escape hatch (exact \"1\" only)"
  - "Makefile verify-no-docker-sdk gate wired into make vet"
affects:
  - "go.mod (go-containerregistry promoted to direct require)"
  - "Makefile (vet aggregate gains verify-no-docker-sdk prerequisite)"
tech-stack:
  added:
    - "github.com/google/go-containerregistry v0.20.7 (promoted indirect → direct)"
  patterns:
    - "os/exec fixed-argv + strict env allowlist + Setpgid (ragserver.go discipline)"
    - "injectable seams for hermetic tests (fake PATH dir; host-arch as parameter)"
    - "grep hard-fail Makefile supply-chain gate (verify-tos shape)"
key-files:
  created:
    - bench/container/container.go
    - bench/container/engine.go
    - bench/container/engine_test.go
    - bench/container/arch.go
    - bench/container/arch_test.go
  modified:
    - go.mod
    - Makefile
decisions:
  - "go.mod promotion done as a hand-edit (move require line), NOT go mod tidy — no Plan-01/02 file imports go-containerregistry yet, so tidy would demote it back to indirect; Plan 03's crane.Config import makes the direct require self-sustaining."
  - "ArchGate is a pure function with host arch as a parameter (not runtime.GOARCH) so the mismatch gate is hermetically testable both ways; ArchGateForHost supplies runtime.GOARCH."
  - "Escape hatch opens ONLY on exact \"1\" (not truthy strings) to avoid ambiguity."
metrics:
  duration: "3m22s"
  completed: 2026-06-21
---

# Phase 84 Plan 01: Container Runtime Engine Shim + Arch & Docker-SDK Gates Summary

Established the `bench/container/` leaf package: a digest-pinned `os/exec` docker/podman engine shim (no Docker SDK), a host-vs-manifest architecture-mismatch refusal gate with a `BENCH_ARCH_MISMATCH_OK=1` escape hatch, and the `verify-no-docker-sdk` grep gate wired into `make vet` — satisfying CONTAINER-01.

## What Was Built

- **`bench/container/container.go`** — leaf-package doc (stdlib + go-containerregistry + sigstore-go + x/sys only, never internal/kernel|semantic); `ImageRef{Repo,Digest}`; `isHexSHA256` (total 64-lowercase-hex check, no regexp); `errBadDigest`.
- **`bench/container/engine.go`** — `Engine{bin}`; `Detect()` probes `docker` then `podman` via `exec.LookPath` (docker wins when both present), returns `errEngineUnavailable` when neither; `pullArgs(repo,digest)` is the hermetically-assertable argv seam that fail-closes on non-hex digests and builds `repo@sha256:<hex>` (never a tag); `PullByDigest` runs the fixed argv with a strict env allowlist (PATH/HOME/HELIX_CACHE_DIR) and `Setpgid:true` for group-kill — mirroring `ragserver.go` discipline (T-84-01-01).
- **`bench/container/arch.go`** — `ArchGate(hostArch,manifestArch)` pure decision: nil on match, refusal otherwise unless `BENCH_ARCH_MISMATCH_OK=="1"`; error names both arches + the escape var; `ArchGateForHost` supplies `runtime.GOARCH` (installer.go idiom, never `uname -m`).
- **`go.mod`** — `github.com/google/go-containerregistry v0.20.7` promoted from `// indirect` to a direct require.
- **`Makefile`** — `verify-no-docker-sdk` target (grep `github.com/docker/docker` → `::error::` + `exit 1`; no-match passes) added to `.PHONY` and wired as a `vet:` prerequisite.

## Tests (TDD, all hermetic)

RED → GREEN cycle per task (4 commits: 2× test, 2× feat).

- `engine_test.go`: `TestDetect{FindsDocker,FindsPodmanWhenNoDocker,PrefersDocker,NoneAvailable}` via fake-PATH temp dir with 0755 stub executables (stubs never executed — only LookPath probed); `TestPullByDigest{UsesAtSha256NeverTag,RejectsNonHexDigest}`; `TestIsHexSHA256`. No live engine required; no skip-only tests (Pitfall 1 avoided).
- `arch_test.go`: `TestArchGate{Match,MismatchRefused,MismatchEscapeHatch,EscapeHatchOnlyExactOne,ForHostUsesRuntimeGOARCH}` — escape hatch tested both ways and against truthy-string ambiguity (`"0"`, `"true"`, `" 1"` all stay refused).

## Verification Results

- `go test ./bench/container/ -count=1` — PASS
- `go vet ./bench/container/...` — clean
- `go build ./...` — PASS
- `make verify-no-docker-sdk` — exits 0 (and verified to exit non-zero when a `github.com/docker/docker` line is temporarily injected)
- `make vet` (full aggregate) — PASS (verify-no-docker-sdk runs first as prerequisite)
- `go list -m github.com/google/go-containerregistry` — `v0.20.7` (direct)
- Leaf invariant: `go list -deps ./bench/container/` shows no `internal/kernel` or `internal/semantic`.

## Deviations from Plan

None — plan executed as written. The plan pre-authorized the go.mod hand-edit over `go mod tidy`; that path was taken (see Decisions) because no Plan-01 source file imports go-containerregistry yet, so `go mod tidy` would revert the direct require. The require line resolves correctly via `go list -m` and `go build ./...` succeeds. Plan 03's `crane.Config` import will make the direct require self-sustaining under future tidies.

## Authentication Gates

None.

## Known Stubs

None. `PullByDigest`'s live exec path is exercised in Plan 03's pull tests; its argv-construction and digest-guard logic are fully covered hermetically here via the `pullArgs` seam — not a stub.

## TDD Gate Compliance

Per-task RED/GREEN gates present in git log:
- Task 1: `test(84-01): add failing tests for container engine shim` (8e4cc727) → `feat(84-01): implement docker|podman os/exec engine shim` (0e965abd)
- Task 2: `test(84-01): add failing tests for arch-mismatch gate` (246dd9da) → `feat(84-01): arch-mismatch gate + verify-no-docker-sdk vet gate` (5e358ef5)
No REFACTOR commits (implementations were minimal-clean on first GREEN).

## Self-Check: PASSED

All 5 created files exist on disk; all 4 per-task commit hashes (8e4cc727, 0e965abd, 246dd9da, 5e358ef5) present in git log.
