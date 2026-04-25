# Phase 50: toolchain-go1.25-gopls-ci - Context

**Gathered:** 2026-04-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Make CI green on `ubuntu-latest` with Go 1.25, bring the benchmark regression gate into PR-blocking enforcement against a real ubuntu-latest baseline (not the existing darwin/arm64 placeholders), document the gopls strategy in CONTRIBUTING.md, and remove the now-stale benchmarks tech-debt note from `PROJECT.md`. Closes TOOL-01 and TOOL-02.

</domain>

<decisions>
## Implementation Decisions

### Baseline strategy
- **D-01:** Capture a fresh `v1.9` ubuntu-latest baseline by running `.github/workflows/capture-baseline.yml` with `milestone=v1.9`. Output file: `test/bench/baselines/v1.9-github-hosted.txt`.
- **D-02:** Switch `.github/workflows/bench.yml` `--baseline` flag and the `benchstat` summary step to compare against `test/bench/baselines/v1.9-github-hosted.txt` (replacing the `v1.1-github-hosted.txt` reference).
- **D-03:** Keep the existing darwin/arm64 placeholder baselines (`v1.1-github-hosted.txt`, `v1.2-phase10/11/12-github-hosted.txt`) on disk as historical reference. Do not delete.
- **D-04:** v1.9 baseline must be captured with the **same flags as the PR gate**: `-short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...` (Pitfall 1 parity). `BenchmarkFullRepoSmoke` stays excluded.

### gopls strategy documentation
- **D-05:** Add a dedicated subsection in `CONTRIBUTING.md` covering: (a) current pin `v0.21.1`, (b) why pinned (skips broken v0.17.1 on linux/amd64; pin also stabilizes cold-start/warm-reuse bench metrics — Pitfall 4), (c) where the pin is set (`GOPLS_VERSION` env in `bench.yml` and `capture-baseline.yml`), (d) bump policy: any version change requires a re-baseline PR per `test/bench/baselines/README.md` refresh policy.
- **D-06:** Keep `GOPLS_VERSION` declared independently in `bench.yml` and `capture-baseline.yml`. Do NOT introduce a reusable workflow or shared versions file. The CONTRIBUTING.md note must explicitly state the two values MUST stay in sync and reference Pitfall 1.

### PR threshold tuning
- **D-07:** Keep PR-tier thresholds at the current `15%` time / `25%` allocs at `p<0.05`. Do not pre-emptively loosen or tighten.
- **D-08:** No pre-launch noise sampling required. Observe real PR behavior post-launch; only revisit thresholds if false-positive rate is unacceptable.
- **D-09:** No in-workflow retry / auto-retry logic. Flake escape hatch is the GitHub Actions "Re-run failed jobs" button — manual only. No CONTRIBUTING.md "best-of-3" prescription.

### Verification scope
- **D-10:** Verification approach is a single **verification PR**: bundle the v1.9 baseline file + bench.yml `--baseline` switch + CONTRIBUTING.md gopls section + PROJECT.md tech-debt removal. The PR's own `go-test.yml` and `bench.yml` runs on ubuntu-latest are the success-criteria evidence (both green = SC #1 and SC #2 satisfied).
- **D-11:** No `workflow_dispatch` dry-run before opening the PR.
- **D-12:** Out of scope for Phase 50: auditing/fixing `docker.yml`, `junie.yml`, `pytest.yml`, `docs.yaml`, `publish.yml`, `codespell.yml`. Only `go-test.yml` and `bench.yml` (plus the `capture-baseline.yml` invocation) are in scope. Anything broken in other workflows that surfaces during work goes to deferred ideas, not this phase.

### Tech-debt cleanup
- **D-13:** SC #4 — remove the "Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest due to gopls v0.17.1 incompatibility..." sentence from `.planning/PROJECT.md` Context section (`Known tech debt`). Other tech-debt items in that paragraph (3 redundant GrammarRegistry instances was Phase 49; rust-analyzer rename = Phase 47; jdtls cold-start ≥ 2min = Phase 48/56) stay or are independently updated by their owning phases.
- **D-14:** SC #3 — the gopls strategy writeup in CONTRIBUTING.md (D-05) is the documented "upgrade" decision (chosen over patch / replacement). Phase review notes can be brief: the upgrade-to-v0.21.1 path is already implemented in workflows; Phase 50 simply ratifies and documents it.

### Claude's Discretion
- Exact placement / heading wording of the gopls subsection in `CONTRIBUTING.md` (top-level vs. nested under "CI" / "Benchmarks").
- Whether the v1.9 baseline capture is committed via the existing `stefanzweifel/git-auto-commit-action@v5` flow in `capture-baseline.yml` or pulled into the verification PR by hand. Either is fine as long as the file exists in the verification PR.
- Whether PROJECT.md edit also adjusts surrounding sentences for flow, vs. deletion-only.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements
- `.planning/ROADMAP.md` §"Phase 50: toolchain-go1.25-gopls-ci" — goal, depends-on, success criteria, requirements TOOL-01/TOOL-02
- `.planning/REQUIREMENTS.md` §TOOL-01, §TOOL-02 — full requirement text
- `.planning/PROJECT.md` Context section "Known tech debt" — sentence to be removed in D-13

### CI workflows touched
- `.github/workflows/bench.yml` — PR benchmark gate; D-02 switches `--baseline` flag, references Pitfalls 1, 4, 5, 8, 9, 11 in header comments
- `.github/workflows/capture-baseline.yml` — milestone-tagged baseline capture; D-01 invokes it
- `.github/workflows/go-test.yml` — Go test job on ubuntu-latest; SC #1 evidence

### Benchmark gate internals
- `test/bench/baselines/README.md` — three-step rollout history and refresh policy; D-05 cites this for the gopls bump policy
- `test/bench/cmd/benchgate/` — gate enforcement binary invoked by bench.yml

### Documentation targets
- `CONTRIBUTING.md` — D-05 adds the gopls strategy subsection here

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `.github/workflows/capture-baseline.yml` — already wired with `workflow_dispatch`, milestone input, and auto-commit via `stefanzweifel/git-auto-commit-action@v5`. D-01 just runs it.
- `GOPLS_VERSION: v0.21.1` env in both workflows — already moved off the broken v0.17.1.
- `benchgate` and `benchstat` parsing already in blocking mode in `bench.yml`.

### Established Patterns
- All Go workflows (`bench.yml`, `capture-baseline.yml`, `go-test.yml`) standardize on `actions/setup-go@v5` with `go-version: '1.25.x'` and `cache: true`.
- `GOMAXPROCS: "4"` pinned in bench workflows to mitigate runner-core variance (Pitfall 5). The v1.9 capture inherits this automatically.
- `SERENA_TEST_LS_TIMEOUT: "4m"` in `go-test.yml` accommodates jdtls cold-start on ubuntu-latest — leave alone.

### Integration Points
- `bench.yml` line(s) referencing `test/bench/baselines/v1.1-github-hosted.txt` — D-02 changes both the `benchstat` invocation and the `benchgate --baseline` flag.
- `PROJECT.md` Context section, paragraph starting "**Known tech debt:**" — D-13 deletes the gopls/baseline sentence only.
- `CONTRIBUTING.md` — D-05 inserts a new subsection; exact location is Claude's discretion.

</code_context>

<specifics>
## Specific Ideas

- gopls "strategy" framing in CONTRIBUTING.md should be one short subsection (not a full incident writeup) — focus on current state, where to change it, and what triggers a re-baseline. Pitfall 1 (parity between bench.yml and capture-baseline.yml `GOPLS_VERSION`) must be called out explicitly because the values are duplicated, not centralized.
- The verification PR title/body should make the SC #1/SC #2 evidence claim explicit so reviewers can confirm both green checks satisfy the phase.

</specifics>

<deferred>
## Deferred Ideas

- **Single source of truth for `GOPLS_VERSION`** (reusable workflow or `.github/versions.env`). Considered, rejected for Phase 50 — keep as a future cleanup if the parity comment ever drifts in practice.
- **Audit of `docker.yml`, `junie.yml`, `pytest.yml`, `publish.yml`, `docs.yaml`, `codespell.yml`** for Go 1.25 / ubuntu-latest correctness. Out of scope; if any are broken, file as a follow-up phase or todo.
- **Pre-launch noise sampling** to data-derive PR-tier thresholds. Considered, deferred — only revisit if false-positive rate is bad after launch.
- **Bench retry / auto-retry logic** in bench.yml. Considered, rejected — flake escape hatch stays manual.
- **Release-tier baseline (full bench, no `-short`)** including `BenchmarkFullRepoSmoke`. Out of scope; release-tier gate runs on a future self-hosted runner per Pitfall 11.

</deferred>

---

*Phase: 50-toolchain-go1.25-gopls-ci*
*Context gathered: 2026-04-25*
