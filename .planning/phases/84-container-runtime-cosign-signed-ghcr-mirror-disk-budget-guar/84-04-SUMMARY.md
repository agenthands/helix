---
phase: 84
plan: 04
subsystem: ci/bench-container
tags: [container, cosign, ghcr, ci, publish, mirror, docs]
requires:
  - "bench/container.pinnedSANRegexLiteral / pinnedOIDCIssuer (Plan 84-03 verify.go — the runtime SAN/issuer pin this workflow's signing identity must match)"
  - "bench/container.VerifyThenPull (Plan 84-03 — runtime verify-before-pull half; this plan is the CI/publish half)"
  - "Phase 58 release.yml cosign-installer keyless OIDC flow (analog, adapted sign-blob → image cosign sign)"
provides:
  - ".github/workflows/bench-mirror.yml — workflow_dispatch publish+sign of ghcr.io/agenthands/helix-bench-* (cosign keyless, by-digest)"
  - "bench/BENCH.md 'Container path (public benchmarks)' section — engine/arch/cache/verify/disk/mirror/gating operator docs"
affects:
  - ".github/workflows/ (new bench-mirror.yml; no change to release.yml)"
  - "bench/BENCH.md (new section; no other section touched)"
tech-stack:
  added: []
  patterns:
    - "cosign keyless OIDC sign-by-DIGEST (repo@sha256:…, never mutable tag) — Fulcio short-lived cert + Rekor, SHA-pinned cosign-installer v3.10.1/cosign-release v2.4.1 (reused verbatim from the audited Phase 58 release.yml)"
    - "publish-identity (CI workflow path + refs/heads/main) === verify-identity (verify.go pinnedSANRegexLiteral): the minted SAN equals the runtime regex byte-for-byte; top-of-file comment + BENCH.md document the lockstep coupling"
    - "matrix-driven extensible mirror source list (suite/instance/upstream); GITHUB_TOKEN packages:write (no PAT secret); env-passed matrix values into quoted shell vars (no untrusted event input in run:)"
key-files:
  created:
    - ".github/workflows/bench-mirror.yml"
  modified:
    - "bench/BENCH.md"
    - ".planning/phases/84-container-runtime-cosign-signed-ghcr-mirror-disk-budget-guar/deferred-items.md"
decisions:
  - "Signing identity pinned to .github/workflows/bench-mirror.yml@refs/heads/main — EXACT match to Plan 03's pinnedSANRegexLiteral (`^https://github\\.com/agenthands/helix/\\.github/workflows/bench-mirror\\.yml@refs/heads/main$`); confirmed by side-by-side grep before commit."
  - "workflow_dispatch only (operator-driven mirror refresh), NOT push/schedule — a scheduled trigger is documented as addable later but MUST still run against refs/heads/main to keep the SAN valid."
  - "GHCR auth via the workflow GITHUB_TOKEN (packages:write in the permissions block) through docker/login-action — no PAT secret required."
  - "Matrix source list seeded with three placeholder smoke upstream refs (swe/multi-swe/terminal); the namespace + retag + push + sign-by-digest mechanics are identical regardless of the concrete upstream ref, so the list is documented as extensible by appending matrix rows (no workflow-structure change)."
  - "LIVE mirror tamper-rejection verification (checkpoint Task 2) DEFERRED — namespace not yet published; orchestrator-resolved APPROVED-WITH-DEFERRAL. Runtime decision logic already proven hermetically in Plan 03 (VirtualSigstore fixtures)."
metrics:
  duration: ~6min
  tasks: 2
  files: 3
  completed: 2026-06-21
---

# Phase 84 Plan 04: Cosign-Signed GHCR Bench Mirror — CI Publish + Container Docs Summary

CI publish workflow that builds, pushes, and **cosign-keyless-signs by digest** a
GHCR mirror of the public-benchmark instance images under
`ghcr.io/agenthands/helix-bench-*`, with a signing identity that matches Plan
84-03's runtime verifier SAN pin byte-for-byte — completing the CI/publish half of
CONTAINER-03 (the runtime verify-before-pull half shipped in Plan 03). BENCH.md
gains a full "Container path" operator section.

## What Was Built

### Task 1 — `bench-mirror.yml` + BENCH.md container docs (commit 1b3db552)

**`.github/workflows/bench-mirror.yml`** — operator-triggered (`workflow_dispatch`,
run from `main`) publish+sign job:
- Permissions: `contents: read`, `id-token: write` (Phase 58 D-02 keyless OIDC
  Fulcio short-lived cert), `packages: write` (push to GHCR).
- SHA-pinned actions reused verbatim from the audited Phase 58 release.yml:
  `actions/checkout@11bd71901…` (v4.2.2) and
  `sigstore/cosign-installer@7e8b541e…` (v3.10.1, `cosign-release: v2.4.1`). GHCR
  login via SHA-pinned `docker/login-action` using `${{ github.actor }}` +
  `${{ secrets.GITHUB_TOKEN }}` (the workflow token already carries
  packages:write — no PAT secret).
- Extensible `matrix.include` source list (suite/instance/upstream) seeded with
  three placeholder smoke refs (swe / multi-swe / terminal). Per row: pull
  upstream → retag to `ghcr.io/agenthands/helix-bench-<suite>:<instance>` → push →
  capture the **pushed DIGEST** via `docker inspect RepoDigests` →
  `cosign sign --yes "<repo>@${DIGEST}"`. Signing is **by digest, never by tag**
  (T-84-04-02), and **CI-only** — `cosign sign` never enters Go runtime code.
- Top-of-file comment box documents the CRITICAL SAN coupling: the minted cert SAN
  is `…/bench-mirror.yml@refs/heads/main`, which must stay equal to verify.go's
  `pinnedSANRegexLiteral`; renaming the file or publishing from a non-`main` ref
  breaks runtime verification and must be mirrored in verify.go + BENCH.md.
- Security: only matrix-author-controlled values flow into `run:` (passed via
  `env:` and referenced as quoted `${SUITE}`/`${INSTANCE}`/`${UPSTREAM}`); no
  untrusted GitHub-event input is interpolated — the workflow-injection patterns
  do not apply.

**`bench/BENCH.md`** — new "## Container path (public benchmarks)" section
documenting: the daemon-free/SDK-free `bench/container/` package + the
`verify-no-docker-sdk` SC#1 gate; engine detection (docker→podman, neither ⇒ live
SKIP); `BENCH_ARCH_MISMATCH_OK=1` escape (with the qemu silent-emulation rationale);
the digest-pinned `$HELIX_CACHE_DIR/bench-images/<sha>/` cache + TOCTOU-safe Ensure
marker; cosign verify-before-pull (single canonical `signature verification FAILED`,
no bytes on reject); the 50 GiB disk-budget guard + remediation; the
`ghcr.io/agenthands/helix-bench-*` namespace + the bench-mirror.yml publish
workflow; the SAN-coupling callout; the live-test gating contract
(`HELIX_BENCH_MIRROR` + engine; hermetic siblings ⇒ not false-green); and the
DEFERRED live-mirror verification note.

### Task 2 — checkpoint:human-verify (orchestrator-resolved: APPROVED-WITH-DEFERRAL)

The one-time GHCR namespace creation + live signed/tampered pull (84-VALIDATION.md
manual-only) **cannot run** in this environment: the `helix-bench-*` namespace is
not yet published, and Claude cannot create an org package namespace or set it
PUBLIC. Per the orchestrator's checkpoint resolution, this is **DEFERRED** (logged
in `deferred-items.md`), not falsely claimed done. The runtime decision logic
(verify-before-pull ordering, single-canonical-error, no-bytes-on-reject) is
already proven hermetically with VirtualSigstore fixtures in Plan 84-03, so only
the live published-mirror confirmation is outstanding.

## Deviations from Plan

None — plan executed as written. The checkpoint Task 2 was resolved by the
orchestrator as APPROVED-WITH-DEFERRAL (the live mirror namespace is not yet
published); the deferral is recorded in `deferred-items.md` and BENCH.md rather
than blocking.

## SAN Coupling Verification (the critical invariant)

Confirmed by side-by-side grep before commit:
- verify.go pin: `^https://github\.com/agenthands/helix/\.github/workflows/bench-mirror\.yml@refs/heads/main$`
- workflow minted SAN: `https://github.com/agenthands/helix/.github/workflows/bench-mirror.yml@refs/heads/main` (file path `.github/workflows/bench-mirror.yml`, dispatch run from `refs/heads/main`).

The regex matches the minted SAN exactly. A CI-signed mirror image will pass the
runtime verifier.

## Threat Model Coverage

All `mitigate` dispositions in the plan's STRIDE register are satisfied:
- **T-84-04-01 (Spoofing):** cosign keyless OIDC signs from the pinned workflow
  path + refs/heads/main; runtime pins the matching SAN regex + issuer.
- **T-84-04-02 (Tampering):** each image signed BY DIGEST (`repo@sha256:…`), never
  by mutable tag.
- **T-84-04-03 (Repudiation):** cosign keyless default writes a Rekor
  transparency-log entry (public audit record).
- **T-84-04-04 (publish↔verify drift):** top-of-file comment + BENCH.md document
  the SAN/ref coupling so a rename/ref change is mirrored in verify.go.
- **T-84-04-SC (action supply chain):** sigstore/cosign-installer SHA-pinned
  (v3.10.1) + cosign-release v2.4.1 — same pins as the audited Phase 58
  release.yml; no new unpinned action. (checkout + docker/login-action are also
  SHA-pinned.)

## Deferred Items

- **LIVE mirror tamper-rejection verification** (checkpoint Task 2) — DEFERRED
  until an operator runs `bench-mirror.yml` once and sets the `helix-bench-*`
  packages PUBLIC. Steps to resolve are in `deferred-items.md` and the BENCH.md
  "Live-test gating contract" note. Runtime logic already hermetically proven
  (Plan 03).

## Verification

- `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/bench-mirror.yml'))"` — PASS (valid YAML).
- Workflow declares `packages: write` + `id-token: write`; uses SHA-pinned cosign-installer v3.10.1/v2.4.1.
- `grep 'cosign sign' .github/workflows/bench-mirror.yml` — present (sign-by-digest).
- `grep 'bench-mirror' bench/container/verify.go` — present; SAN regex matches the workflow path/ref.
- `grep -E 'bench-images|BENCH_ARCH_MISMATCH_OK|HELIX_BENCH_MIRROR|DEFERRED' bench/BENCH.md` — all present.
- `go build ./...` — PASS.
- `make vet` — PASS (all 6 custom vettools + `verify-no-docker-sdk` SC#1 gate clean).
- actionlint not installed in this environment — YAML validated via python yaml.safe_load instead.

## Self-Check: PASSED
- `.github/workflows/bench-mirror.yml` present on disk.
- `bench/BENCH.md` "Container path (public benchmarks)" section present.
- Task 1 commit 1b3db552 found in git history.
