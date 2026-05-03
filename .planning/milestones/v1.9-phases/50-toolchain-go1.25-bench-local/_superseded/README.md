---
superseded_on: 2026-04-28
reason: CI-benchmark scope violates project no-CI-bench rule
---

# Superseded — original Phase 50 artifacts

These files belong to the **original** Phase 50 plan, which was replanned on 2026-04-28 because its core deliverable — a CI benchmark gate on `ubuntu-latest` plus a `v1.9-github-hosted.txt` baseline — violates the project rule that **benchmarks run only locally, never on hosted CI runners**.

Specifically:
- `50-01-PLAN.md` — triggered `capture-baseline.yml` on `ubuntu-latest` to produce a CI-captured baseline.
- `50-02-PLAN.md` — wired the CI baseline into `bench.yml` PR gate.
- `50-CONTEXT.md`, `50-RESEARCH.md`, `50-PATTERNS.md`, `50-DISCUSSION-LOG.md`, `50-VALIDATION.md` — all the prior context/research/decision artifacts assumed CI-based bench gating.

The legitimate non-bench parts of original Phase 50 (Go 1.25 toolchain bump, ubuntu-latest CI for build/vet/test, gopls v0.17.1 resolution, PROJECT.md tech-debt cleanup) are preserved in the revised Phase 50 scope. The revised plan additionally **removes** the existing `bench.yml`, `capture-baseline.yml`, and hosted baseline artifacts in `test/bench/baselines/`, converting the bench harness to local-only.

Kept here for reference and audit trail.
