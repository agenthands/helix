# Phase 50: toolchain-go1.25-gopls-ci - Context

**Gathered:** 2026-04-25
**Amended:** 2026-04-25 (post-Plan-50-01 capture run failure — added D-15..D-19)
**Pivoted:** 2026-04-25 (after second `capture-baseline.yml` failure — drops CI bench gate entirely, see "Pivot 2026-04-25" block)
**Status:** Ready for re-planning under the pivoted decisions

<domain>
## Phase Boundary

Make CI green on `ubuntu-latest` with Go 1.25 (TOOL-01), document the gopls
strategy and local benchmark workflow in `CONTRIBUTING.md`, remove the
stale benchmarks tech-debt note from `PROJECT.md`, and **retire** the
GitHub-CI benchmark gate (delete `bench.yml` + `capture-baseline.yml`).
TOOL-02 is dropped — see Pivot 2026-04-25.

</domain>

<decisions>
## Implementation Decisions (current — post-pivot)

### CI scope
- **D-A1:** `go-test.yml` on `ubuntu-latest` with Go 1.25.x is the sole CI evidence for TOOL-01. No bench job runs on PRs.
- **D-A2:** Delete `.github/workflows/bench.yml` and `.github/workflows/capture-baseline.yml` outright. Both files are removed by Phase 50; no rename, no demotion to `workflow_dispatch`-only. Honest signal: no enforcement, no half-measures.
- **D-A3:** Keep `test/bench/cmd/benchgate/` as a local pre-release tool. The binary still builds, ships in the source tree, and is invoked manually. Document its `make` target and direct CLI usage in CONTRIBUTING.md.

### Baseline strategy (post-pivot)
- **D-A4:** Benchmarks are captured **locally pre-release** on whatever stable hardware the maintainer has. Output filename pattern: `test/bench/baselines/v<version>-local-<goos>-<goarch>.txt`. No CI workflow produces baselines.
- **D-A5:** Existing CI-hosted baselines (`v1.1-github-hosted.txt`, `v1.2-phase10-github-hosted.txt`, `v1.2-phase11-github-hosted.txt`, `v1.2-phase12-github-hosted.txt`) stay on disk as **historical reference**. `test/bench/baselines/README.md` is rewritten to describe the new local-first workflow and label the historical files as "captured under the retired CI gate".
- **D-A6:** No v1.9 baseline file is produced by Phase 50. The first local baseline lands when the next release-driven run does it; Phase 50 documents the procedure and leaves capture to the maintainer's release ritual.

### Documentation (CONTRIBUTING.md)
- **D-A7:** Add a `## Benchmarks` section to `CONTRIBUTING.md` covering: (a) when to run (pre-release / suspicion of regression), (b) how to run (`go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...` — same flags the retired gate used, for continuity), (c) where to put the result (`test/bench/baselines/v<version>-local-<goos>-<goarch>.txt`), (d) how to compare (`benchstat old.txt new.txt` and/or `go run ./test/bench/cmd/benchgate --baseline ...`), (e) why we are NOT enforcing on CI (shared-CPU runner noise + cold-start fragility — link to this CONTEXT.md amendment block).
- **D-A8:** Add a `## gopls pin` section to `CONTRIBUTING.md`: current pin `v0.21.1`, why pinned (skips broken v0.17.1 on linux/amd64; stabilizes cold-start/warm-reuse bench metrics — Pitfall 4 from `50-RESEARCH.md`), where the pin lives (`Makefile` / dev-tooling install scripts, NOT in workflows since `bench.yml` and `capture-baseline.yml` are gone), bump policy: any version change should trigger a fresh local baseline before the next release.
- **D-A9:** Optional `## Makefile bench targets` mention if Plan 50-02 adds them (e.g., `make bench-capture`, `make bench-compare`). Discretion: include only if Plan 50-02 actually adds the targets.

### REQUIREMENTS.md / TOOL-02
- **D-A10:** Mark TOOL-02 as **cancelled** in `.planning/REQUIREMENTS.md` with an inline note: `~~TOOL-02~~ — Cancelled in Phase 50 (CI-hosted bench gate proven unreliable on shared-CPU GitHub runners across two failed capture attempts; benchmarks now run locally pre-release. See .planning/phases/50-.../50-CONTEXT.md "Pivot 2026-04-25"). The traceability table for TOOL-02 reads `Cancelled (Phase 50)`.
- **D-A11:** TOOL-01 wording stays as-is. Phase 50 still satisfies it via `go-test.yml` on ubuntu-latest with Go 1.25.

### PROJECT.md tech-debt cleanup
- **D-A12:** Replace (not delete) the existing tech-debt sentence about CI bench baselines. New wording captures the post-pivot state: `Benchmark gate intentionally not enforced on CI; baselines are captured locally pre-release. See CONTRIBUTING.md "Benchmarks" section.` Other tech-debt items in that paragraph are unaffected.

### Verification
- **D-A13:** Phase 50 lands as a **single verification PR** bundling Plans 50-01 + 50-02. PR success criteria:
  1. `go-test.yml` on the PR is green on ubuntu-latest with Go 1.25 (TOOL-01).
  2. `bench.yml` and `capture-baseline.yml` are absent from `.github/workflows/`.
  3. `CONTRIBUTING.md` has the new Benchmarks + gopls pin sections.
  4. `PROJECT.md` tech-debt sentence rewritten per D-A12.
  5. `REQUIREMENTS.md` shows TOOL-02 cancelled per D-A10.
  6. `test/bench/baselines/README.md` rewritten per D-A5.
- **D-A14:** Out of scope (reaffirmed): `docker.yml`, `junie.yml`, `pytest.yml`, `docs.yaml`, `publish.yml`, `codespell.yml`. If anything breaks during the work, file as a follow-up phase.

### Plan structure
- **D-A15:** Two plans, single PR (per discuss-phase decision):
  - **Plan 50-01 (CI surgery):** delete `bench.yml`, delete `capture-baseline.yml`, rewrite `test/bench/baselines/README.md`. No code changes outside `.github/` and `test/bench/baselines/`. Verifier: `ls .github/workflows/bench*.yml capture-baseline.yml` returns nothing; `git grep -l 'capture-baseline\|bench.yml' .github/` returns nothing.
  - **Plan 50-02 (docs):** add `CONTRIBUTING.md` Benchmarks + gopls pin sections, rewrite `PROJECT.md` tech-debt sentence, mark TOOL-02 cancelled in `REQUIREMENTS.md`, optionally add `make bench-capture` / `make bench-compare` targets if trivial. Verifier: grep CONTRIBUTING.md for both new headings; grep PROJECT.md for new wording; grep REQUIREMENTS.md for `Cancelled (Phase 50)`.

### Claude's Discretion
- Exact heading levels of the CONTRIBUTING.md sections.
- Whether to add `make bench-*` targets (D-A9). Add only if Makefile structure already accommodates them and the addition is <20 lines.
- Whether `test/bench/baselines/README.md` keeps a deprecation table for the historical CI baselines or just a one-paragraph note.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements
- `.planning/ROADMAP.md` §"Phase 50: toolchain-go1.25-gopls-ci" — goal, depends-on; the success criteria there will need updating to match D-A13.
- `.planning/REQUIREMENTS.md` §TOOL-01, §TOOL-02 — TOOL-02 cancellation per D-A10.
- `.planning/PROJECT.md` Context section "Known tech debt" — sentence to be rewritten per D-A12.

### CI workflows (deletion targets)
- `.github/workflows/bench.yml` — DELETE (D-A2).
- `.github/workflows/capture-baseline.yml` — DELETE (D-A2).
- `.github/workflows/go-test.yml` — UNCHANGED; provides TOOL-01 evidence.

### Local bench tooling (preserved)
- `test/bench/cmd/benchgate/` — kept; CONTRIBUTING.md documents local invocation.
- `test/bench/baselines/README.md` — REWRITE per D-A5.
- Existing `v1.1-github-hosted.txt` and four `v1.2-phase*-github-hosted.txt` files — keep, label as historical.

### Documentation targets
- `CONTRIBUTING.md` — adds Benchmarks (D-A7) and gopls pin (D-A8) sections.
- `Makefile` — optional bench targets per D-A9.

### Pivot rationale (must read)
- `50-RESEARCH.md` Pitfall 5 (shared-CPU runner variance) and Pitfall 11 (release-tier on self-hosted only) — already flagged the unreliability that drove the pivot.
- This file's "Pivot 2026-04-25" block.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `go-test.yml` already runs on ubuntu-latest with Go 1.25.x and `cache: true` — TOOL-01 evidence path is intact.
- `test/bench/cmd/benchgate/` (main.go + main_test.go) is self-contained and CGO-free — suitable for local use without modification.
- `benchstat` is a one-line `go install` from the user's machine — no project-side wiring needed for D-A7's CONTRIBUTING.md instructions.

### Established Patterns
- CONTRIBUTING.md (if it exists) likely uses `##` headings for top-level sections; new sections should follow the same level.
- `Makefile` already has `## help-text` annotations on targets per `make help` convention; new bench targets should follow the same style if added.

### Integration Points
- `PROJECT.md` Context section, paragraph starting "**Known tech debt:**" — D-A12 rewrites the bench sentence in place.
- `REQUIREMENTS.md` TOOL-02 row + traceability table — D-A10 strikethroughs and adds inline note.
- `test/bench/baselines/README.md` — D-A5 full rewrite of the framing.

</code_context>

<specifics>
## Specific Ideas

- The CONTRIBUTING.md "Benchmarks" section should explicitly state, in one sentence, **why** CI no longer enforces benchmarks. This prevents future contributors from "re-enabling" the gate without understanding the noise/fragility history.
- The PROJECT.md rewrite should be deletion-plus-replacement, not augmentation — keep the paragraph length neutral.
- The verification PR title should be something like `phase 50: retire CI bench gate, document local bench + gopls pin`. PR body must call out TOOL-02 cancellation explicitly so reviewers don't think it was forgotten.

</specifics>

<deferred>
## Deferred Ideas

- **Self-hosted runner for benchmarks** to bring back automated enforcement. Reaffirmed deferred — proper future phase, depends on infra availability.
- **Advisory bench comments on PRs** (post benchstat output as a PR comment, never fail). Considered, rejected for Phase 50 — adds noise without enforcement; revisit if/when self-hosted runner phase happens.
- **Single source of truth for `GOPLS_VERSION`** (was D-06 deferred). Now moot since the workflows referencing it are deleted; remaining `gopls` install reference (Makefile / dev tooling) is single-source by default.
- **Centralized timeout/version config** (was D-19 deferred). Moot for the same reason.
- **Audit of `docker.yml`, `junie.yml`, `pytest.yml`, `publish.yml`, `docs.yaml`, `codespell.yml`** for Go 1.25 / ubuntu-latest correctness. Out of scope; if any are broken, file as a follow-up phase.

</deferred>

<pivot_2026_04_25>
## Pivot 2026-04-25 — Drop CI bench gate

After two failed `capture-baseline.yml` runs on `gsd/phase-50-toolchain-go1.25-gopls-ci`:

| Run ID | Outcome | Root cause |
|--------|---------|------------|
| `24938112931` | FAIL | LS readiness 30s timeout in `test/bench/bench_helpers_test.go:280` (cold-start blew past 30s on ubuntu-latest) |
| `24938933745` | CANCELLED mid-bench | Manually cancelled after observing the same fragility pattern repeating |

The 2026-04-25 amendment (D-15..D-19) attempted to patch around the symptoms (env-driven LS timeout, bump `timeout-minutes` to 60). User decision after the second failure: **the strategy is the problem, not the timeouts**. GitHub-hosted runners are noisy shared-CPU hosts (own RESEARCH.md Pitfall 5) and unsuitable for benchmark baselines or PR-blocking gates.

Phase 50 pivots to **delete the CI bench gate** rather than fix it. This:
- Eliminates the LS-timeout / workflow-budget tail of failures permanently.
- Honestly reflects what's actually being enforced.
- Preserves benchmark capability via local tooling (benchgate + `make bench` + benchstat).
- Cancels TOOL-02 as a requirement; TOOL-01 (CI green on ubuntu-latest with Go 1.25) is unaffected and still satisfied by `go-test.yml`.

Decisions D-01 through D-19 from the original CONTEXT and the first amendment are **superseded** by D-A1 through D-A15 above. The history is preserved in the "Superseded Decisions" block below for traceability.

</pivot_2026_04_25>

<superseded_decisions>
## Superseded Decisions (history)

The following decisions from the original 2026-04-25 CONTEXT and the first 2026-04-25 amendment were superseded by the Pivot 2026-04-25 (above). Kept for audit trail.

### Original baseline / gate / verification decisions (superseded)
- ~~D-01~~ Capture v1.9 baseline via `capture-baseline.yml` on ubuntu-latest. → **Superseded by D-A4/D-A6** (no CI baseline; local capture per release).
- ~~D-02~~ Switch `bench.yml --baseline` flag to `v1.9-github-hosted.txt`. → **Superseded by D-A2** (`bench.yml` deleted).
- ~~D-03~~ Keep historical baselines on disk. → **Carried forward as D-A5**.
- ~~D-04~~ v1.9 baseline captured with PR-gate flag parity. → **Moot** (no CI gate). The same flags are still recommended for local capture per D-A7.
- ~~D-07~~ Keep PR thresholds at 15%/25% p<0.05. → **Moot** (no CI gate).
- ~~D-08~~ No pre-launch noise sampling. → **Moot**.
- ~~D-09~~ No in-workflow retry logic. → **Moot**.
- ~~D-10~~ Single verification PR bundling baseline + bench.yml swap + docs. → **Replaced by D-A13** (single verification PR, different bundle: deletions + docs).
- ~~D-11~~ No workflow_dispatch dry-run before opening PR. → **Moot**.

### gopls / docs decisions (mostly carried forward, restated)
- ~~D-05~~ CONTRIBUTING.md gopls subsection. → **Replaced by D-A8** (same intent, references removed instead of bench.yml).
- ~~D-06~~ Keep `GOPLS_VERSION` independent in bench.yml + capture-baseline.yml. → **Moot** (both files deleted; pin lives in Makefile / dev tooling).
- ~~D-13~~ Remove the bench-baseline tech-debt sentence from PROJECT.md. → **Replaced by D-A12** (rewrite, not delete).
- ~~D-14~~ Document gopls upgrade-to-v0.21.1. → **Carried forward in D-A8**.
- ~~D-12~~ Out-of-scope workflow audit list. → **Carried forward as D-A14**.

### Amendment (D-15..D-19) decisions (superseded by pivot)
- ~~D-15~~ Env-driven LS readiness timeout in `bench_helpers_test.go`. → **No longer required** (no CI capture). Local capture inherits the same constant; if a developer hits it locally on cold cache, they can run twice. Not a Phase 50 task.
- ~~D-16~~ Bump `timeout-minutes` to 60 in bench.yml + capture-baseline.yml. → **Moot** (both files deleted).
- ~~D-17~~ Insert Plan 50-00 prerequisite. → **Moot** (the prereq existed only to make D-15/D-16 feasible).
- ~~D-18~~ Plan 50-01 retry semantics for partial baseline files. → **Moot**.
- ~~D-19~~ No reusable workflow / shared timeouts file. → **Moot** (workflows deleted).

</superseded_decisions>

---

*Phase: 50-toolchain-go1.25-gopls-ci*
*Context gathered: 2026-04-25*
*Amended: 2026-04-25 (post Plan 50-01 dry-run failure on run id 24938112931)*
*Pivoted: 2026-04-25 (post second capture failure / cancellation on run id 24938933745) — drops CI bench gate*
