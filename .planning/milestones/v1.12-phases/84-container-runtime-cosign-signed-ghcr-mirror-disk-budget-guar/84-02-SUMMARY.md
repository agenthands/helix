---
phase: 84
plan: 02
subsystem: bench/container
tags: [container, cache, helix-cache-dir, disk-budget, statfs, cross-platform]
requires:
  - "bench/container.isHexSHA256 / errBadDigest (Plan 01, same package)"
provides:
  - "bench/container.cacheDir() three-step $HELIX_CACHE_DIR precedence (verbatim from ragindex)"
  - "bench/container.ImagePath(sha) digest-pinned cache path under bench-images/<sha>/ with path-escape guard"
  - "bench/container.Ensure(sha, fetch) verified-then-rename cache fill + .container-cache-ok sentinel (cache-hit seam for Plan 03)"
  - "bench/container.MinFreeBytes (50 GiB) + DiskGuard{availFn} + NewDiskGuard() + (DiskGuard).Check (CONTAINER-04)"
  - "bench/container.availBytes build-tag split: unix.Statfs (!windows) / GetDiskFreeSpaceEx (windows)"
  - "bench/container.procGroupAttr() build-tag-split seam (unblocks engine.go windows cross-compile)"
affects:
  - "bench/container/engine.go (Setpgid moved behind procGroupAttr() seam — Rule 3 fix)"
tech-stack:
  added: []
  patterns:
    - "cacheDir() three-step $HELIX_CACHE_DIR precedence copied verbatim from bench/ragindex (CONTAINER-02 byte-for-byte convention)"
    - "caller-supplied digest fail-closed via isHexSHA256 BEFORE filepath.Join (path-escape mitigation)"
    - "verified-then-rename cache fill: fetch into temp staging dir, sentinel, os.Rename publishes only on success (TOCTOU / Pitfall 3)"
    - "injectable availFn seam for 100%-hermetic synthetic low-disk test (no real volume dependency)"
    - "build-tag split keeps x/sys/unix and syscall.Setpgid out of untagged files so windows release archive cross-compiles (Pitfall 6)"
key-files:
  created:
    - bench/container/cache.go
    - bench/container/cache_test.go
    - bench/container/disk.go
    - bench/container/disk_unix.go
    - bench/container/disk_windows.go
    - bench/container/disk_test.go
    - bench/container/procgroup_unix.go
    - bench/container/procgroup_windows.go
  modified:
    - bench/container/engine.go
decisions:
  - "Ensure's sentinel is a marker FILE (.container-cache-ok) inside the published dir, not just dir-existence: a dir without the marker is treated as cold and re-filled, so a crash between rename and a later write is never mistaken for a complete cache."
  - "Reused the package-wide validHex const already declared in engine_test.go rather than redeclaring it in cache_test.go (would have collided)."
  - "windows availBytes returns GetDiskFreeSpaceEx's lpFreeBytesAvailable (the per-user quota-honoring free amount), mirroring unix Bavail (free-to-non-root) rather than total free — same semantics across platforms."
  - "[Rule 3] engine.go's syscall.SysProcAttr{Setpgid:true} (Plan 01) does not compile on windows; extracted into a procGroupAttr() build-tag-split seam (unix sets Setpgid, windows returns nil) — Plan 01 only ran host go build so it never caught this; this plan's verify explicitly cross-compiles for windows."
metrics:
  duration: "3m10s"
  completed: 2026-06-21
---

# Phase 84 Plan 02: Image Cache + Cross-Platform Disk-Budget Guard Summary

Built the SHA256-digest-pinned bench image cache at `$HELIX_CACHE_DIR/bench-images/<sha>/` (CONTAINER-02) — keyed strictly by digest with a path-escape guard, filled via verified-then-rename — and a cross-platform pre-flight disk-budget guard that refuses a run below 50 GiB with a single-line remediation (CONTAINER-04). Both are fully hermetic: the cache test points `HELIX_CACHE_DIR` at `t.TempDir()`, and the disk test injects a synthetic `availFn`, so neither needs a live engine, registry, or real full volume.

## What Was Built

- **`bench/container/cache.go`** — `cacheDirEnv` const and `cacheDir()` three-step precedence copied VERBATIM from `bench/ragindex/cache.go` (so the image cache mirrors the rag-index `$HELIX_CACHE_DIR` convention byte-for-byte); `imagesSubdir = "bench-images"`; `ImagePath(sha)` fail-closes via `isHexSHA256` (Plan 01) BEFORE `filepath.Join`, so a `"../etc"` or `"sha256:.."` key cannot escape `bench-images/` (T-84-02-01); `Ensure(sha, fetch)` fills the cache via fetch-into-temp-staging → write `.container-cache-ok` sentinel → `os.Rename` (atomic publish only on success, T-84-02-02 / Pitfall 3), and returns `hit=true` on a re-run without re-fetching.
- **`bench/container/disk.go`** — `MinFreeBytes = 50 GiB` (binary, A3); `DiskGuard{availFn}` with an injectable available-bytes seam; `NewDiskGuard()` wires `availFn` to the build-tagged `availBytes`; `(DiskGuard).Check(path)` propagates a syscall error verbatim (no "refused" framing) and otherwise refuses below threshold with a single-line message naming free/path/requirement + the `$HELIX_CACHE_DIR` remediation (CONTAINER-04).
- **`bench/container/disk_unix.go`** (`//go:build !windows`) — `availBytes` via `unix.Statfs` (`Bavail * Bsize`); `golang.org/x/sys/unix` imported ONLY here (Pitfall 6).
- **`bench/container/disk_windows.go`** (`//go:build windows`) — `availBytes` via `windows.GetDiskFreeSpaceEx` (`lpFreeBytesAvailable`); identical signature to the unix sibling.
- **`bench/container/procgroup_unix.go` / `procgroup_windows.go`** — Rule 3 fix: `procGroupAttr()` build-tag-split seam (unix `Setpgid:true`, windows `nil`) so `engine.go` no longer fails the windows cross-compile.

## Tests (TDD, all hermetic)

RED → GREEN cycle per task (4 commits: 2× test, 2× feat).

- `cache_test.go`: `TestImagePathUnderBenchImages` (joins under `$HELIX_CACHE_DIR/bench-images/<hex>`), `TestImagePathRejectsNonHex` (`../etc`, `sha256:abc`, empty, uppercase → `errBadDigest`, empty path, no Join), `TestCacheHitOnRerun` (fetch counter == 1 over two `Ensure` calls; published file present), `TestCacheHelixCacheDirPrecedence`. Uses `t.Setenv("HELIX_CACHE_DIR", t.TempDir())`. Reuses the package-wide `validHex`.
- `disk_test.go` (no build tag — `Check` is platform-neutral, driven through injected `availFn`): `TestDiskGuardRefusesBelowThreshold` (10 GiB → names free/path/50 GiB/`$HELIX_CACHE_DIR`), `TestDiskGuardAllowsAtOrAboveThreshold` (`MinFreeBytes` and 60 GiB → nil), `TestDiskGuardPropagatesAvailErr` (sentinel propagated, no "refused" framing), `TestDiskGuardRemediationIsOneLine` (no `\n`), `TestMinFreeBytesIs50GiB`.

## Verification Results

- `go test ./bench/container/ -count=1` — PASS (cache + disk + Plan-01 suites)
- `go vet ./bench/container/...` — clean
- `go build ./...` — PASS
- `make vet` (full aggregate incl. leaf-invariant + verify-no-docker-sdk gates) — exit 0
- Cross-compile: `GOOS=windows GOARCH=amd64`, `GOOS=linux GOARCH=arm64`, `GOOS=darwin GOARCH=arm64` `go build ./bench/container/` all OK (build-tag split holds, Pitfall 6)
- Leaf invariant: `go list -deps ./bench/container/` shows no `internal/kernel` or `internal/semantic`
- Cache-hit re-run proven by fetch-call counter == 1 over two `Ensure` calls

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] engine.go Setpgid broke the windows cross-compile**
- **Found during:** Task 2 (first `GOOS=windows go build ./bench/container/` in the verify step).
- **Issue:** Plan 01's `bench/container/engine.go:63` set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`. `Setpgid` is a unix-only field of `syscall.SysProcAttr`; the package therefore failed to compile for `windows`. Plan 01's verify only ran host `go build ./...` (linux), so it never surfaced. This plan's CONTAINER build-tag-split invariant (Pitfall 6) and explicit `GOOS=windows` verify step require the package to cross-compile.
- **Fix:** Extracted the process-group attribute into a `procGroupAttr()` build-tag-split seam: `procgroup_unix.go` (`//go:build !windows`) returns `&syscall.SysProcAttr{Setpgid: true}`; `procgroup_windows.go` (`//go:build windows`) returns `nil` (POSIX process groups have no windows analog). Removed the now-unused `syscall` import from the untagged `engine.go` and replaced the literal with `procGroupAttr()`.
- **Files modified:** `bench/container/engine.go`; created `bench/container/procgroup_unix.go`, `bench/container/procgroup_windows.go`.
- **Commit:** 3a4a2e1b (committed alongside the Task 2 GREEN, since it is what unblocks the Task 2 cross-compile verify gate).

## Authentication Gates

None.

## Known Stubs

None. `Ensure`'s `fetch` closure is an intentional injection seam (Plan 03 supplies the real verify-then-pull); its cache-fill/sentinel/rename logic is fully exercised hermetically here.

## Deferred Issues

- `gofmt -l bench/container/engine_test.go` reports a formatting drift on the Plan-01 inline-comment alignment block (lines ~106-110). This is a pre-existing Plan-01 file, out of scope for this plan, and is NOT a `make vet` gate (gofmt is only in the write-target `gofmt -w .`, not a check gate). Left untouched per the scope boundary. Logged to `.planning/phases/84-container-runtime-cosign-signed-ghcr-mirror-disk-budget-guar/deferred-items.md`.

## TDD Gate Compliance

Per-task RED/GREEN gates present in git log:
- Task 1: `test(84-02): add failing tests for digest-pinned image cache` (4e1e3fb9) → `feat(84-02): digest-pinned image cache at $HELIX_CACHE_DIR/bench-images/<sha>/` (001af3dc)
- Task 2: `test(84-02): add failing tests for cross-platform disk-budget guard` (98006538) → `feat(84-02): cross-platform disk-budget guard, <50 GiB refusal (CONTAINER-04)` (3a4a2e1b)
No REFACTOR commits (implementations were minimal-clean on first GREEN).

## Self-Check: PASSED

All 8 created files exist on disk; `engine.go` modified; all 4 per-task commit hashes (4e1e3fb9, 001af3dc, 98006538, 3a4a2e1b) present in git log.
