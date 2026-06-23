# Phase 99: Vendored Aider Fixtures + Mixed-License Gate - Context

**Gathered:** 2026-06-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

A deterministic, offline, mixed-license vendored fixture tree (MIT Exercism polyglot subset + Apache-2.0 aider edit-format fixtures) lands in the tree with correct per-file SPDX headers, per-track attribution/NOTICE, and a manifest, guarded by an extended hard-fail license gate that goes RED on any tampered or missing header.

**Requirements:** VENDOR-01, VENDOR-02, VENDOR-03

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Key constraints carried from research (SUMMARY.md / STACK.md / PITFALLS.md) and the user's ratified scope:
- **Mixed-license tree (user ratified):** vendor BOTH (VENDOR-01) the MIT-licensed Exercism polyglot fixtures (from `Aider-AI/polyglot-benchmark`, which redistributes Exercism content under MIT — byte-verified in the existing `bench/datasets/aider-polyglot/LICENSE-AUDIT.md`) AND (VENDOR-02) Aider's Apache-2.0 edit-format fixtures (from the `Aider-AI/aider` tool repo). Per-file `SPDX-License-Identifier:` headers: MIT for the polyglot subset, Apache-2.0 for the aider edit-format fixtures.
- **Vendor a recorded, deterministic SUBSET** (not all 225 tasks / 6 tracks) — record the exact selection in a `VENDOR-MANIFEST.md`. The subset must be enough to drive the Phase 100 polyglot edit benchmark + the Phase 102 corpora, but small enough to keep the tree lean. Pin the upstream commit SHAs (mirror the existing `bench/datasets/aider-polyglot/pin.go` pinned-sha + `isHexSHA1` discipline).
- **Reuse, don't rebuild:** the existing Phase 85 adapter (`bench/datasets/aider-polyglot/`) already has clone/loader/pin + a `LICENSE-AUDIT.md`. This phase ADDS a committed `fixtures/` tree + the Apache-2.0 edit-format fixtures + extends the gate. Do NOT rebuild the loader.
- **Extended `make verify-licenses` (VENDOR-03):** the existing hard-fail license gate (Phase 85, mirrors Phase 75 `verify_tos.go` strict-decode discipline) is extended to scan the FULL vendored tree under BOTH dispositions (MIT + Apache-2.0), failing RED on a missing/incorrect SPDX header or NOTICE. Ship an adversarial tamper test (flip/remove a header → gate RED) — anti-vacuity (the v1.12 lesson).
- **Network dependency:** vendoring requires fetching the upstream repos (git clone of `Aider-AI/polyglot-benchmark` and `Aider-AI/aider` at pinned SHAs). Research MUST confirm network/clone availability in this environment and the exact upstream directory layout; if a clone isn't possible, the plan must surface that as a blocker rather than fabricating fixture content.
- Per-track NOTICE/attribution files preserving the upstream copyright + license text (Exercism per-track licenses are MIT; aider is Apache-2.0 — include the Apache-2.0 NOTICE requirements).

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Relevant existing files: `bench/datasets/aider-polyglot/` (Phase 85: clone.go, loader.go, pin.go, LICENSE-AUDIT.md — the pinned-sha clone + isHexSHA1 + 2-attempt protocol + per-track license audit), the existing `make verify-licenses` target + its Go gate (mirrors Phase 75 `verify_tos.go`), `bench/LICENSES.md` (Phase 88 — license rows), the `vet-ablation-leakage` boundary, `Makefile`.

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the ROADMAP success criteria — discuss phase skipped. Refer to ROADMAP Phase 99 description and its 4 success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
