# Phase 50: toolchain-go1.25-bench-local - Context

**Gathered:** 2026-04-28
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers two coupled outcomes:

**(A) Toolchain green on ubuntu-latest CI for build/vet/test.** A CI job on `ubuntu-latest` with Go 1.25 completes `go build ./...`, `go vet ./...`, and `go test ./... -count=1` green. Benchmarks are explicitly NOT executed on CI (project-wide rule).

**(B) Bench harness converted to local-only.** All hosted-CI bench plumbing (`bench.yml`, `capture-baseline.yml`, the `test/bench/cmd/benchgate` CLI, the `test/bench/baselines/v1.*-github-hosted.txt` files) is removed. A local bench flow is documented (Make targets + CONTRIBUTING.md). The benchmarks tech-debt note in PROJECT.md is cleaned up (along with the resolved GrammarRegistry item from Phase 49).

**Driver:** project-wide rule that benchmarks run only locally (hosted runners produce unstable, untrustable baselines). The original Phase 50 plan (archived in `_superseded/`) violated this rule and was discarded on 2026-04-28.

</domain>

<decisions>
## Implementation Decisions

### gopls strategy
- **D-01:** gopls strategy is doc-only. No code or langregistry version pinning. Update CONTRIBUTING.md with a sub-section explaining: (i) gopls is installed at runtime by `internal/langregistry`, (ii) the project tests against `gopls@latest`, (iii) gopls v0.17.1 had a Go 1.25 linux/amd64 incompatibility that was resolved upstream by >=v0.21. Update USAGE.md troubleshooting note (line 515) to reflect this. *No `min_version` field, no exact version pin.*

### Bench infra removal
- **D-02:** Delete `.github/workflows/bench.yml` and `.github/workflows/capture-baseline.yml`.
- **D-03:** Delete `test/bench/cmd/benchgate` entirely (the local Go CLI that compared benchstat output against a baseline). Contributors who want comparisons use upstream `benchstat` directly. Remove its package, tests, and any references in docs.
- **D-04:** Delete all `test/bench/baselines/v1.*.txt` files (v1.1-github-hosted.txt, v1.2-phase10/11/12-github-hosted.txt). They reference an old comparison flow that is being dismantled. No "historical-local" preservation, no rename.
- **D-05:** Keep `test/bench/baselines/` directory in repo. Rewrite `test/bench/baselines/README.md` to document the local-only flow: what the directory is for, how to capture a baseline locally (`make bench-baseline`), and that captured baselines are gitignored personal artifacts.
- **D-06:** No PR-time safety net — honor system. No `.github/pull_request_template.md` checkbox. Bench impact responsibility is mentioned in CONTRIBUTING.md only.

### Local bench command surface
- **D-07:** Add two Make targets, mirroring the `bench-jdtls-warm` self-doc style already in the Makefile:
  - `make bench` — runs the bench suite once and prints to stdout. Equivalent to: `go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...`
  - `make bench-baseline` — same command, captured into a single fixed file path (overwrites on each run). The exact path is a planning-time decision, expected to be `test/bench/baselines/local.txt` or similar single canonical location.
- **D-08:** `make bench-baseline` writes to a **single fixed path** (no `NAME=` parameter, no auto-derived `uname` suffix). Simplest invocation; overwrites on each run. Loses history-keeping but matches the user preference for minimal Make surface.
- **D-09:** The captured baseline file is **gitignored**. Add the canonical path (e.g. `test/bench/baselines/local.txt`) to `.gitignore`. Captured baselines are personal scratch artifacts; nothing is committed.

### PROJECT.md tech-debt cleanup
- **D-10:** Delete the bench sentence from PROJECT.md tech-debt note (line 139): the part beginning "Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest...". Local-only is the chosen design, not tech debt.
- **D-11:** While editing line 139, also remove the GrammarRegistry sentence ("3 redundant GrammarRegistry instances (functionally correct)") — already resolved by Phase 49. The remaining items in the note (rust-analyzer rename quirk, jdtls cold-start) stay.

### Claude's Discretion
- The exact filename for the gitignored local baseline (D-08, D-09) — planner picks the most sensible path within `test/bench/baselines/`.
- Wording of CONTRIBUTING.md sections (gopls subsection per D-01, bench-impact note per D-06) — planner drafts; user reviews.
- Whether to delete `.github/workflows/bench.yml`/`capture-baseline.yml` in one commit each or together — planner decides; both must land in the same plan.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project decisions and constraints
- `.planning/PROJECT.md` — core value, current tech-debt list (line 139 needs editing per D-10/D-11), constraints
- `.planning/REQUIREMENTS.md` §"Infra — Toolchain" — TOOL-01 and TOOL-02 (revised 2026-04-28 to reflect local-only bench)
- `.planning/ROADMAP.md` §"Phase 50" — revised goal, 5 success criteria, dependency note for Phase 51
- `.planning/phases/50-toolchain-go1.25-bench-local/_superseded/README.md` — explanation of why the original CI-bench plan was discarded
- `.planning/phases/50-toolchain-go1.25-bench-local/_superseded/50-RESEARCH.md` — DO NOT consume; archived for audit only

### CI / workflows (to be modified or removed)
- `.github/workflows/bench.yml` — DELETE (per D-02)
- `.github/workflows/capture-baseline.yml` — DELETE (per D-02)
- `.github/workflows/go-test.yml` — KEEP. This is the workflow that satisfies criterion 1 (build/vet/test green on ubuntu-latest with Go 1.25.x). Already exists; verify it currently runs green on main.
- `.github/workflows/pytest.yml` — references `gopls@latest`; review whether it still applies (legacy Python tests) and whether the gopls install line stays after Phase 50 docs update.

### Code to remove
- `test/bench/cmd/benchgate/` — DELETE (per D-03), including all `*.go` files and tests
- `test/bench/baselines/v1.1-github-hosted.txt`, `v1.2-phase10-github-hosted.txt`, `v1.2-phase11-github-hosted.txt`, `v1.2-phase12-github-hosted.txt` — DELETE (per D-04)

### Docs to edit
- `Makefile` — add `bench` and `bench-baseline` targets following the `bench-jdtls-warm` self-doc style (per D-07)
- `test/bench/baselines/README.md` — rewrite for local-only flow (per D-05)
- `CONTRIBUTING.md` — add gopls sub-section (per D-01) and bench-impact note (per D-06)
- `USAGE.md` — line 515 troubleshooting note about gopls v0.17.1 needs updating (per D-01)
- `.planning/PROJECT.md` — line 139 tech-debt note edit (per D-10, D-11)
- `.gitignore` — add the single canonical local-baseline path (per D-09)

### External / upstream context
- gopls release notes for v0.17.1 → v0.21.x — researcher should confirm the linux/amd64 + Go 1.25 incompatibility was indeed fixed upstream and which version first ships the fix (this informs the wording in CONTRIBUTING.md per D-01)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`Makefile` — `bench-jdtls-warm` target** uses the self-doc `## help` annotation style and platform-aware shell case. New `bench` and `bench-baseline` targets should mirror this style for consistency.
- **`test/bench/tools_bench_test.go`** — the existing bench suite (38+ tools via `BenchmarkTools` table-driven). Already invoked via `go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...` in the soon-to-be-deleted bench.yml; the same invocation becomes the `make bench` body.
- **`internal/langregistry/languages.go:158`** — the Go LSEntry has `Command: "gopls"` with no version field. Confirms D-01: the project doesn't pin gopls today, the doc-only strategy matches existing reality.
- **`.github/workflows/go-test.yml`** — already runs `go vet ./...` and `go test ./... -count=1` on ubuntu-latest with Go 1.25.x (does NOT install gopls — runtime tests use `internal/langregistry` to install LS binaries on demand). This workflow already targets the criterion-1 outcome.

### Established Patterns
- **Self-documenting Makefile** — `bench-jdtls-warm: ## Run Java integration suite cold then warm; print both wall-clocks` — the `## ` after the colon is the help text. Follow this pattern for new targets.
- **gopls runtime install via langregistry** — confirms gopls is not a build-time dependency of Serena itself; it's a runtime LS binary the user/system provides. This is the foundation for the doc-only gopls strategy (D-01).
- **tech-debt items inline in PROJECT.md "Context" section** — comma-separated sentences in a single paragraph (line 139). Edit pattern: remove specific sentence(s), leave the rest of the paragraph intact.

### Integration Points
- **Phase 51 (packaging-goreleaser) depends on Phase 50** — needs green ubuntu-latest CI for build/vet/test (criterion 1). Phase 51's release-job hosting depends on go-test.yml being reliable. The dependency is preserved: deleting bench.yml/capture-baseline.yml does not affect go-test.yml.
- **`USAGE.md` Troubleshooting section** — the gopls v0.17.1 sentence (line 515) is in a user-facing doc. Edit must remain accurate for users running on Go 1.25 + linux/amd64; phrase to acknowledge resolution rather than describe an active bug.
- **`.gitignore`** — currently has entries from `chore: snapshot for Helix repo migration` (e.g., `borrow/`, `.claude/` transients). The new `test/bench/baselines/local.txt` (or whatever single canonical path the planner picks) gets a fresh entry.

</code_context>

<specifics>
## Specific Ideas

- The user wants the simplest possible Make target shape (single fixed-path `make bench-baseline`, no NAME parameter, no uname-derived suffix) — preference for minimal interface ceremony observed across multiple decisions in this discussion.
- The user explicitly rejected "honor system + PR template" and "honor system + CONTRIBUTING.md prominent section" — picked just "honor system, mention in CONTRIBUTING.md as one consideration among many." Bench-impact awareness is a mild contributor responsibility, not a process gate.
- Original (superseded) plan's existence is preserved for audit (`_superseded/README.md`) but has zero downstream weight — researcher and planner MUST NOT read those archived files for guidance.

</specifics>

<deferred>
## Deferred Ideas

- **Self-hosted runner for bench gating.** The original bench plan referenced a future "release-tier self-hosted runner" (D-02 in `_superseded/test/bench/baselines/README.md`). This contradicts the project's no-CI-bench rule. If self-hosted bench infra is ever revisited, it would need a fresh requirements pass and explicit rule reversal — not in any current milestone scope.
- **Auto-comparison tooling for local baselines.** Decision A2a deletes `benchgate`. If contributors later request a local "compare two baselines" helper, it can be added as a small standalone script in a future polish phase — not in scope here.
- **Bench-baseline NAME parameter / history-keeping.** D-08 picks single fixed path for simplicity. If the lack of history becomes painful in practice, a future micro-phase could add `NAME=` support without breaking the simple invocation.

</deferred>

---

*Phase: 50-toolchain-go1.25-bench-local*
*Context gathered: 2026-04-28*
