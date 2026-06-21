# Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

The container infra needed only for public benchmarks — `os/exec` to `docker` (with podman drop-in), pinned-SHA256 per-instance images, cosign-signed mirror under `ghcr.io/agenthands/helix-bench-*` (reusing v1.10 Phase 58 cosign keyless infra), and a pre-flight disk-budget guard so a contributor's laptop doesn't melt before the first SWE-bench pull.

**Requirements:** CONTAINER-01, CONTAINER-02, CONTAINER-03, CONTAINER-04

**Success Criteria (what must be TRUE):**

1. `grep "github.com/docker/docker"` in `go.mod` returns empty; bench harness works with either `docker` or `podman` on PATH; arch-mismatch refusal (e.g., SWE-bench Verified amd64 image on arm64 host) refuses to run unless `BENCH_ARCH_MISMATCH_OK=1` is set.
2. Per-instance images are pinned by SHA256 digest, not tag; image-cache state lives under `$HELIX_CACHE_DIR/bench-images/<sha>/`; a cache-hit test passes on re-run.
3. A cosign-signed mirror of SWE-bench / Multi-SWE-bench / Terminal-Bench instance images is published to `ghcr.io/agenthands/helix-bench-*`; bench harness verifies the cosign signature before pulling; a tampered image is rejected.
4. Disk-budget guard fails the run if available disk on the bench host is < 50 GB before a SWE-bench full run; the synthetic low-disk test trips the guard with a one-line remediation message.

**Depends on:** Phase 75 (cost table, INFRA hygiene), Phase 82 (aggregator can ingest container-cell results), v1.10 Phase 58 (cosign keyless flow reused for the mirror)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use the ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

- Container engine via `os/exec` to `docker`/`podman` on PATH — NO `github.com/docker/docker` SDK dependency (SC#1 forbids it). Detect engine, normalize the CLI surface.
- Reuse the v1.10 Phase 58 cosign keyless attestation flow for the GHCR mirror; verify signatures before pulling.
- Live docker/cosign/GHCR operations are environment-dependent: gate real container/registry integration tests behind availability checks (skip cleanly when docker/podman/cosign or network are absent, like the HELIX_BIN gating pattern) so unit-level logic (engine detection, SHA256 pinning, cache layout, disk-budget guard, signature-verify decision) is hermetically testable without a live daemon.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors:
- v1.10 Phase 58 cosign keyless attestation infra (reused for the GHCR mirror) — research must locate it.
- `$HELIX_CACHE_DIR` resolution convention (already used by Phase 83 `bench/ragindex` cache at `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`) — mirror for `$HELIX_CACHE_DIR/bench-images/<sha>/`.
- Phase 82 aggregator ingestion path (container-cell results feed it).
- Phase 75 cost table + INFRA hygiene gates.

</code_context>

<specifics>
## Specific Ideas

No additional requirements beyond CONTAINER-01..04 and the four success criteria above — discuss phase skipped. Refer to the ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
