---
phase: 50-toolchain-go1.25-gopls-ci
plan: 02
subsystem: docs / build / planning
tags:
  - docs
  - requirements
  - makefile
  - roadmap
  - validation
dependency_graph:
  requires:
    - Phase 50 Plan 50-01 (deletes .github/workflows/bench.yml + capture-baseline.yml; rewrites test/bench/baselines/README.md) — landed at base 84ca745a
  provides:
    - CONTRIBUTING.md ## Benchmarks + ## gopls pin sections (D-A7, D-A8)
    - PROJECT.md tech-debt sentence rewritten per D-A12
    - REQUIREMENTS.md TOOL-02 marked Cancelled (Phase 50) per D-A10
    - Optional Makefile bench-capture / bench-compare targets per D-A9
    - ROADMAP.md milestone-list one-liner aligned with post-pivot intent
    - 50-VALIDATION.md per-task map + manual-only rows refreshed for post-pivot plans
  affects:
    - .planning/REQUIREMENTS.md milestone v1.9 traceability table
    - .planning/PROJECT.md "Known tech debt" paragraph
tech_stack:
  added: []
  patterns:
    - Strikethrough-with-inline-note for cancelled requirements (preserves audit trail vs. outright deletion — see T-50-06 mitigation)
    - Makefile `## help-text` annotations consistent with existing `docs:`, `clean-jdtls-cache:`, `bench-jdtls-warm:` style
key_files:
  created:
    - .planning/phases/50-toolchain-go1.25-gopls-ci/50-02-SUMMARY.md
  modified:
    - CONTRIBUTING.md
    - .planning/PROJECT.md
    - .planning/REQUIREMENTS.md
    - Makefile
    - .planning/ROADMAP.md
    - .planning/phases/50-toolchain-go1.25-gopls-ci/50-VALIDATION.md
decisions:
  - D-A7 → CONTRIBUTING.md ## Benchmarks section landed (when/how/where/compare/why-not-CI)
  - D-A8 → CONTRIBUTING.md ## gopls pin section landed (v0.21.1 pin, rationale, location, bump policy)
  - D-A9 → Optional Makefile targets ADDED (end-state A): bench-capture + bench-compare, 13-line addition stays under 20-line discretion threshold
  - D-A10 → REQUIREMENTS.md TOOL-02 strikethrough + Cancelled (Phase 50) traceability row
  - D-A11 → TOOL-01 row preserved verbatim
  - D-A12 → PROJECT.md tech-debt first sentence rewritten verbatim
metrics:
  duration_min: ~12
  completed: 2026-04-25
  tasks: 6 executed (Tasks 1, 2, 3, 5, 6, 7) + 1 verify-only (Task 8); Task 4 was renumbered to Task 8 in the post-pivot plan
  files_touched: 6 modified, 1 created
---

# Phase 50 Plan 02: Post-pivot doc landing Summary

**One-liner:** Land the post-pivot doc set — CONTRIBUTING.md Benchmarks + gopls pin sections, PROJECT.md tech-debt rewrite, REQUIREMENTS.md TOOL-02 cancellation, optional Makefile bench wrappers, ROADMAP one-liner refresh, 50-VALIDATION.md task-map refresh — locking the strategy change captured in CONTEXT.md Pivot 2026-04-25 into the repo's contributor-facing surface.

## What Shipped

### Task 1 — CONTRIBUTING.md (commit `a7279533`)

- Replaced the stale `## Benchmark CI Gate` section (~lines 178-184 pre-edit) with a new `## Benchmarks` section (~52 lines) covering when/how/where/compare/why-not-CI plus a `## gopls pin` section (~21 lines) covering v0.21.1 pin, rationale, location, bump policy.
- Updated the `## Running Benchmarks` section: replaced the three CI-claim bullets with one bullet pointing at the new `## Benchmarks` section.
- Net effect on top-level section ordering: `## Running Benchmarks` → ... → `## Benchmarks` → `## gopls pin` → `## Legacy Python` (the new sections occupy the slot vacated by the retired `## Benchmark CI Gate`).
- Acceptance greps: `grep -c '^## Benchmarks$' CONTRIBUTING.md` = 1; `grep -c '^## gopls pin$' CONTRIBUTING.md` = 1; `grep -cF '.github/workflows/bench.yml' CONTRIBUTING.md` = 0; `grep -cF 'GOPLS_VERSION' CONTRIBUTING.md` = 0.

A second amendment to CONTRIBUTING.md was made in Task 5 (commit `6ae5bdbe`) — added the optional `make bench-capture` / `make bench-compare` convenience-wrapper bullet inside the `## Benchmarks` section's compare block.

### Task 2 — PROJECT.md tech-debt sentence (commit `d7f765a3`)

Single-line replacement inside the `**Known tech debt:**` paragraph.

- **Removed:** "Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest due to gopls v0.17.1 incompatibility with Go 1.25 on linux/amd64."
- **Added:** "Benchmark gate intentionally not enforced on CI; baselines are captured locally pre-release. See CONTRIBUTING.md \"Benchmarks\" section."
- Other tech-debt sentences (GrammarRegistry, rust-analyzer rename, jdtls cold-start) preserved verbatim. `git diff --stat` shows 1 insertion / 1 deletion.

### Task 3 — REQUIREMENTS.md TOOL-02 cancellation (commit `0e09ab8e`)

Two coordinated edits.

- **Edit A (entry, ~line 20):** flipped `[ ]` to `[x]`, wrapped `**TOOL-02**` and the original requirement text in `~~...~~`, appended the Cancelled-in-Phase-50 inline note pointing to `.planning/phases/50-toolchain-go1.25-gopls-ci/50-CONTEXT.md` "Pivot 2026-04-25".
- **Edit B (traceability row, ~line 79):** `| TOOL-02 | Phase 50 | Pending |` → `| TOOL-02 | Phase 50 | Cancelled (Phase 50) |`.
- TOOL-01 row left untouched (D-A11). Coverage block ("Mapped to phases: 14") unchanged — cancelled requirements still count as mapped.

### Task 5 — Optional Makefile bench targets (commit `6ae5bdbe`)

**Decision:** Proceeded — END-STATE A.

All four D-A9 criteria satisfied: 13-line addition stays well under 20-line threshold; targets follow the existing `## help-text` annotation style; do not duplicate `bench-jdtls-warm:`; do not auto-install `benchstat` (target prints an install hint and exits 2 if missing).

- `.PHONY` line extended with `bench-capture bench-compare`.
- `bench-capture: ## Capture a local pre-release benchmark baseline (override OUT=...)` — defaults `OUT` to `test/bench/baselines/v-local-$(go env GOOS)-$(go env GOARCH).txt` when unset (deliberately lacks a `<version>` token so the maintainer overrides for a real release; default lets `make bench-capture` smoke-test without crashing).
- `bench-compare: ## Diff two baselines via benchstat (usage: make bench-compare OLD=<path> NEW=<path>)` — exits 2 with usage message if `OLD`/`NEW` unset; exits 2 with install hint if `benchstat` missing.
- Verified: `make -n bench-capture` and `make -n bench-compare OLD=a NEW=b` both expand cleanly; `make bench-compare` (no args) exits with code 2 and the documented usage line on stderr.
- Cross-doc: added a single bullet inside CONTRIBUTING.md `## Benchmarks` "How to compare" block: `Or use the convenience wrappers: make bench-capture (with OUT=...) and make bench-compare OLD=... NEW=....`

### Task 6 — ROADMAP.md milestone-list one-liner (commit `9a4fc1e6`)

Single-line replacement at line 24.

- **Removed:** `Unblock Go 1.25 / gopls on \`ubuntu-latest\` and restore the CI benchmark gate`
- **Added:** `Make CI green on ubuntu-latest with Go 1.25 and retire the CI benchmark gate`
- The detailed Phase 50 entry later in the file already reflected the pivot; line 24 was the last pre-pivot wording.
- Acceptance: `grep -cF 'restore the CI benchmark gate' .planning/ROADMAP.md` = 0; `grep -cF 'retire the CI benchmark gate' .planning/ROADMAP.md` = 1.

### Task 7 — 50-VALIDATION.md refresh (commit `d78fe1db`)

- **Test Infrastructure block:** Config file row now lists `go-test.yml`, `CONTRIBUTING.md`, and `test/bench/baselines/README.md` (deleted workflows out); estimated runtime row cites `~5 min for go-test.yml on PR`.
- **Sampling Rate bullets:** `Before /gsd-verify-work` row cites `go-test.yml green on the verification PR (D-A13 bullet 1)`; max-feedback-latency row cites the same.
- **Per-Task Verification Map:** rebuilt with rows for `50-01-01`, `50-01-02`, `50-02-01`, `50-02-02`, `50-02-03`, `50-02-05`, `50-02-06`, `50-02-07`, `50-02-08`. (Plan 50-02 Task 4 was renumbered to Task 8 — the cross-doc consistency sweep — in the post-pivot plan; no separate `50-02-04` row.)
- **Manual-Only Verifications:** body collapsed to a single row — `go-test.yml is green on the verification PR on ubuntu-latest with Go 1.25 (D-A13 bullet 1)` mapped to TOOL-01.

### Task 8 — Cross-doc consistency sweep (verify-only, no commit)

All four primary docs internally consistent post-edit:

- `grep -cF '.github/workflows/bench.yml'` in CONTRIBUTING.md / .planning/PROJECT.md / .planning/REQUIREMENTS.md = 0 in each.
- `grep -cF '.github/workflows/capture-baseline.yml'` in same set = 0 in each.
- `test/bench/baselines/README.md` already cross-references `CONTRIBUTING.md` (Plan 50-01).
- `CONTRIBUTING.md` cross-references `test/bench/baselines/` (6 occurrences in the new `## Benchmarks` section).
- `PROJECT.md` cross-references `CONTRIBUTING.md` (3 occurrences, including the new tech-debt sentence).
- `Pivot 2026-04-25` anchor reachable from CONTRIBUTING.md (2x), REQUIREMENTS.md (1x), and `test/bench/baselines/README.md` (Plan 50-01).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] Self-reference paradox in 50-VALIDATION.md row 50-02-07**

- **Found during:** Task 7 verification.
- **Issue:** The plan's Task 7 acceptance pattern asked for `! grep -qF 'bench.yml' .planning/phases/50-toolchain-go1.25-gopls-ci/50-VALIDATION.md` to succeed. But the same row in the verification map contains the literal `bench.yml` substring inside its own grep test command, making the negative grep trivially false. The original 50-01-01 row also contains `! test -e .github/workflows/bench.yml` which legitimately re-introduces the literal string. Same paradox for `capture-baseline.yml`.
- **Fix:** Rewrote the 50-02-07 row to assert positive markers only (`grep -F 'go-test.yml is green'` and `grep -F '50-02-08'`) and added an explicit exemption note: "Pre-pivot strings such as `bench.yml`, `capture-baseline.yml`, `v1.9-github-hosted.txt`, and `bench-gate` are exempted from absence assertion against this file because they legitimately re-appear inside other rows' grep test commands; absence in the live tree is covered cross-doc by 50-02-08." This preserves the verifier's ability to confirm post-pivot intent without contradicting itself.
- **Files modified:** `.planning/phases/50-toolchain-go1.25-gopls-ci/50-VALIDATION.md`
- **Commit:** `d78fe1db`

No other deviations — Tasks 1, 2, 3, 5, 6 executed exactly as written.

## D-A13 Verification Bullet → Artifact Map (Ready-for-PR Checklist)

| D-A13 Bullet | Verification | Source |
|---|---|---|
| 1. `go-test.yml` green on the verification PR (TOOL-01) | Manual-only — verified on the PR's CI run | Manual-Only Verifications row in 50-VALIDATION.md |
| 2. `bench.yml` and `capture-baseline.yml` absent from `.github/workflows/` | `! test -e .github/workflows/bench.yml && ! test -e .github/workflows/capture-baseline.yml` | Plan 50-01 (commit `f96d675a`); 50-VALIDATION.md row 50-01-01 |
| 3. CONTRIBUTING.md has new Benchmarks + gopls pin sections | `grep -c '^## Benchmarks$' CONTRIBUTING.md` = 1; `grep -c '^## gopls pin$' CONTRIBUTING.md` = 1; `grep -F 'v0.21.1' CONTRIBUTING.md` matches | Plan 50-02 Task 1 (commit `a7279533`); 50-VALIDATION.md row 50-02-01 |
| 4. PROJECT.md tech-debt sentence rewritten per D-A12 | `grep -F 'Benchmark gate intentionally not enforced on CI; baselines are captured locally pre-release.' .planning/PROJECT.md` matches | Plan 50-02 Task 2 (commit `d7f765a3`); 50-VALIDATION.md row 50-02-02 |
| 5. REQUIREMENTS.md TOOL-02 cancelled per D-A10 | `grep -F '~~**TOOL-02**~~' .planning/REQUIREMENTS.md && grep -F 'Cancelled (Phase 50)' .planning/REQUIREMENTS.md` | Plan 50-02 Task 3 (commit `0e09ab8e`); 50-VALIDATION.md row 50-02-03 |
| 6. `test/bench/baselines/README.md` rewritten per D-A5 | `grep -F 'Capturing a Baseline (local pre-release)' test/bench/baselines/README.md` | Plan 50-01 Task 2 (commit `8f71fde9`); 50-VALIDATION.md row 50-01-02 |

Plan 50-01 covers bullets 2 + 6 (already landed at base `84ca745a`). This plan covers bullets 3, 4, 5. Bullet 1 is the verification PR's own `go-test.yml` run.

## Threat Flags

None — this plan modified only Markdown documentation, the Makefile (with mitigations T-50-04 / T-50-05 already documented in plan threat register), and planning artifacts. No new code, no new third-party dependency, no change to runtime trust surface.

## Self-Check: PASSED

**Files:**
- FOUND: CONTRIBUTING.md (modified)
- FOUND: .planning/PROJECT.md (modified)
- FOUND: .planning/REQUIREMENTS.md (modified)
- FOUND: Makefile (modified)
- FOUND: .planning/ROADMAP.md (modified)
- FOUND: .planning/phases/50-toolchain-go1.25-gopls-ci/50-VALIDATION.md (modified)
- FOUND: .planning/phases/50-toolchain-go1.25-gopls-ci/50-02-SUMMARY.md (this file)

**Commits:**
- FOUND: `a7279533` — docs(50-02): retire CI bench gate section, add Benchmarks + gopls pin sections to CONTRIBUTING.md
- FOUND: `d7f765a3` — docs(50-02): rewrite bench tech-debt sentence in PROJECT.md per D-A12
- FOUND: `0e09ab8e` — docs(50-02): mark TOOL-02 as cancelled in REQUIREMENTS.md per D-A10
- FOUND: `6ae5bdbe` — feat(50-02): add bench-capture / bench-compare Makefile targets per D-A9
- FOUND: `9a4fc1e6` — docs(50-02): rewrite ROADMAP line 24 milestone-list one-liner to post-pivot wording
- FOUND: `d78fe1db` — docs(50-02): refresh 50-VALIDATION.md per-task map and manual-only rows post-pivot
