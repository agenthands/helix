---
phase: 75-schema-fairness-contract-tree-skeleton
plan: 02
subsystem: bench
tags: [bench, skeleton, tos-attestation, infra, docs]
requires:
  - "75-01: Phase 64 microbench relocated to internal/semantic/bench/ (frees bench/ namespace)"
provides:
  - "Canonical six-dir bench/ layout (datasets, runners, languages, evaluators, reports, schema)"
  - "bench/BENCH.md: operator-doc + runtime-vs-contract map + INFRA-03 separation note + make-bench collision flag"
  - "bench/PROVIDERS.md: per-provider TOS attestation surface with D-14 YAML frontmatter (verify-tos input)"
  - "bench/LICENSES.md: per-dataset license-audit scaffold"
  - "eval/EVAL.md reciprocal INFRA-03 cross-link"
  - "REQUIREMENTS.md / ROADMAP graded-criteria reconciliation (BENCH-03 v2, six-dir, eval-narrowing)"
affects:
  - "Phases 76-89: every downstream bench phase writes into this dir contract"
  - "Plan 75-05: make verify-tos parses bench/PROVIDERS.md frontmatter"
tech-stack:
  added: []
  patterns:
    - "YAML frontmatter per-provider block (D-14 six-key field set) for machine-parseable TOS attestation"
    - "Permissive-default flags with an explicit honest deferral note (no overclaimed verification)"
key-files:
  created:
    - bench/datasets/.gitkeep
    - bench/runners/.gitkeep
    - bench/languages/.gitkeep
    - bench/evaluators/.gitkeep
    - bench/reports/.gitkeep
    - bench/schema/.gitkeep
    - bench/BENCH.md
    - bench/PROVIDERS.md
    - bench/LICENSES.md
  modified:
    - eval/EVAL.md
    - .planning/REQUIREMENTS.md
decisions:
  - "Set benchmarking_permitted/publish_permitted true for ALL providers (Anthropic/OpenAI/Google) + local models as PERMISSIVE DEFAULTS; live-TOS verification explicitly deferred to a future phase (user decision at checkpoint 75-02-02)"
  - "Added a Local / self-hosted provider block with tos_url: local sentinel (no external API TOS)"
  - "Kept attested_by: helix-maintainers; bumped attested_on to 2026-06-15 on all blocks so verify-tos 90-day gate passes"
metrics:
  duration: "~10min (task 1, prior agent) + ~8min (task 2 + finalize, this agent)"
  completed: 2026-06-15
---

# Phase 75 Plan 02: Bench Skeleton + Provider TOS Attestation Summary

Stood up the canonical six-directory `bench/` skeleton and authored the foundation prose docs
(BENCH.md, PROVIDERS.md, LICENSES.md) with the eval/EVAL.md reciprocal INFRA-03 cross-link and
the REQUIREMENTS/ROADMAP graded-criteria reconciliation; then, after the human-verify
checkpoint, set permissive TOS defaults (benchmarking + publish permitted) for all providers and
local models with an explicit honest deferral note, deferring live-TOS verification to a future
phase to unblock the provider-independent bench system.

## What Was Built

- **Six-dir bench/ skeleton** (task 1, commit `3801ba4e`): `datasets/`, `runners/`,
  `languages/`, `evaluators/`, `reports/` (the five BENCH-01 runtime dirs) plus `schema/` (the
  sixth D-07 contract dir), each tracked via `.gitkeep`.
- **bench/BENCH.md** (task 1): operator-side prereqs (Python 3.11+, Docker Engine, per-language
  toolchains), six-dir runtime-vs-contract map, the INFRA-03 eval↔bench separation note, and the
  flagged `make bench` name collision (deferred to Phase 77).
- **bench/PROVIDERS.md** (tasks 1 + 2): per-provider TOS attestation surface. Each provider
  section opens with a `---`-delimited YAML frontmatter block carrying the D-14 six-key field set
  (`provider`, `tos_url`, `attested_by`, `attested_on`, `benchmarking_permitted`,
  `publish_permitted`). Now contains four blocks: Anthropic, OpenAI, Google, and Local /
  self-hosted.
- **bench/LICENSES.md** (task 1): per-dataset license-audit table scaffold, seeded with the
  internal ToolBench-Go row.
- **eval/EVAL.md** (task 1): one reciprocal INFRA-03 pointer paragraph to `bench/BENCH.md`.
- **REQUIREMENTS.md / ROADMAP reconciliation** (task 1): BENCH-03 `v1`→`v2`, six-dir
  enumeration, and the eval-byte-identical narrowing.

## Checkpoint Resolution (75-02-02 human-verify)

The blocking human-verify checkpoint asked the maintainer to confirm the provider TOS flags
against live provider terms. The user's decision was to NOT perform live-TOS verification now,
and instead:

- Set `benchmarking_permitted: true` and `publish_permitted: true` for **every** provider
  (Anthropic, OpenAI, Google) and for local/self-hosted models, as **permissive defaults**.
- Add a Local / self-hosted provider block (`tos_url: local`, no external API TOS).
- Keep `attested_by: helix-maintainers`; set `attested_on: 2026-06-15` on every modified block.
- Add a prominent top-of-file note (and per-block prose) stating the flags are PERMISSIVE
  DEFAULTS, NOT independently verified attestations, and that live-TOS verification is
  explicitly deferred to a future phase — keeping the record honest.

Applied in commit `47dfcd47`. The honesty requirement (no overclaimed verification) is
satisfied: the file no longer frames the flags as confirmed attestations awaiting a checkpoint;
it frames them as deferred permissive defaults.

## Deviations from Plan

The plan's task 2 was a `checkpoint:human-verify` that, as written, expected the maintainer to
confirm flags against live TOS. The user instead chose to defer live-TOS verification and apply
permissive defaults uniformly. This changes the OpenAI/Google blocks from the seed `false`
values (and the "future placeholders" framing) to `true` with permissive-default framing, and
adds a fourth Local / self-hosted block not enumerated in the original plan. This is a direct
application of the user's checkpoint decision, not an autonomous deviation.

No Rule 1-4 auto-fixes were required. `go vet ./...` exits 0.

## Verification

- `find bench -maxdepth 1 -mindepth 1 -type d | wc -l` == 6.
- PROVIDERS.md: 4 provider blocks; each of the six D-14 keys appears exactly 4 times; zero
  `false` flags remain; all four `attested_on` values are `2026-06-15`.
- Top-of-file `PERMISSIVE DEFAULTS` note present; per-block permissive-default prose present.
- Reciprocal INFRA-03 links present in both BENCH.md and EVAL.md (task 1).
- REQUIREMENTS.md BENCH-03 reads v2; six-dir + eval-narrowing present (task 1).
- `go vet ./...` exits 0.

## Self-Check: PASSED

- FOUND: bench/PROVIDERS.md
- FOUND: commit 3801ba4e (task 1)
- FOUND: commit 47dfcd47 (task 2)
