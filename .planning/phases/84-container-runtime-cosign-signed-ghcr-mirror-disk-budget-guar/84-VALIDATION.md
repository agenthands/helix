---
phase: 84
slug: container-runtime-cosign-signed-ghcr-mirror-disk-budget-guard
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-21
---

# Phase 84 — Validation Strategy

> Per-phase validation contract. Seeded from the "Validation Architecture" section of
> 84-RESEARCH.md — the planner fills the Per-Task Verification Map from the four success criteria.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go toolchain |
| **Quick run command** | `go vet ./bench/... && go test ./bench/...` |
| **Full suite command** | `go test ./...` (live docker/podman/cosign/GHCR tests SKIP unless engine + network present) |
| **Estimated runtime** | ~60–120 seconds (unit); live integration tests gated/skipped |

---

## Sampling Rate

- **After every task commit:** Run the quick run command for touched packages
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner fills from RESEARCH Validation Architecture) | | | CONTAINER-01..04 | | | unit | | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` grep gate: `verify-no-docker-sdk` Makefile target asserts no `github.com/docker/docker` (SC#1)
- [ ] Engine detection unit test (docker vs podman on PATH; injectable PATH lookup) (SC#1)
- [ ] Arch-mismatch refusal unit test (amd64 image on arm64 host refused unless `BENCH_ARCH_MISMATCH_OK=1`) via crane `.Architecture` + injected host arch (SC#1)
- [ ] SHA256 digest-pin + image-cache-layout unit test at `$HELIX_CACHE_DIR/bench-images/<sha>/`; cache-hit on re-run (SC#2)
- [ ] cosign/sigstore-go signature-verify decision unit test: tampered/unsigned manifest rejected with single canonical error (SC#3, mirroring internal/upgrade/verify.go)
- [ ] Disk-budget guard unit test: injectable `availFn` trips guard < 50 GB with one-line remediation (SC#4)

*Live docker/podman/cosign/GHCR-pull tests SKIP cleanly when engine/network absent (HELIX_BIN-style gating).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real cosign-signed GHCR mirror pull + tamper rejection against live `ghcr.io/agenthands/helix-bench-*` | CONTAINER-03 | Requires the public mirror namespace to exist + network + a signed image | After the `bench-mirror.yml` CI publish job runs, pull a real signed image and a tampered one; confirm accept/reject |
| Real `docker`/`podman run` of a pinned instance image | CONTAINER-01 | Requires a container engine + the image pulled | Run a cell against a real pinned image with docker, then podman |

*All decision logic (engine detect, digest pin, cache layout, arch refusal, verify branch, disk guard) has hermetic automated verification via injected seams.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
