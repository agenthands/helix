# Phase 50: toolchain-go1.25-bench-local - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-28
**Phase:** 50-toolchain-go1.25-bench-local
**Areas discussed:** gopls upgrade strategy, Bench infra: keep vs remove, Local bench command surface, PROJECT.md tech-debt rewrite

---

## Background

Original Phase 50 design ("toolchain-go1.25-gopls-ci") wired benchmarks into GitHub Actions via `capture-baseline.yml` + `bench.yml` PR gate. During an attempted execution of the original plan, the user invoked the project-wide rule: **benchmarks run only locally, never on hosted CI runners**. The original plan was archived to `_superseded/`, ROADMAP and REQUIREMENTS were rewritten to reflect a local-only bench scope, and this discussion gathered context for the revised phase.

---

## Area 1: gopls upgrade strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Doc-only: explain in CONTRIBUTING.md | Add a CONTRIBUTING.md note: "gopls is installed at runtime by langregistry; we test against gopls@latest. v0.17.1 had a Go 1.25 linux/amd64 issue that >=v0.21 resolves." Update USAGE.md troubleshooting. No code/config changes. | ✓ |
| Pin a minimum gopls version in langregistry | Add a `min_version: v0.21.1` field to the Go LSEntry; warn at install time if user has an older gopls. More guardrails but more code surface to maintain. | |
| Pin an exact gopls version in langregistry | Force `gopls@v0.21.1` (or similar) at install time via langregistry. Reproducible LS behavior but locks the project to a specific version. | |

**User's choice:** Doc-only.
**Notes:** Discovery during scoping that bench workflows already used `gopls v0.21.1` and `internal/langregistry` doesn't pin a version made this question largely a docs-cleanup decision. The "v0.17.1 incompatibility" referenced in PROJECT.md and USAGE.md is historical — already resolved upstream by ≥v0.21. No code changes needed.

---

## Area 2: Bench infra — keep vs remove

### Sub-question 2a: `test/bench/cmd/benchgate`

| Option | Description | Selected |
|--------|-------------|----------|
| Keep as a local-only tool | Keep benchgate intact. Becomes a CLI contributors can run locally to compare a fresh bench against their own committed baseline. Document in CONTRIBUTING.md. | |
| Delete benchgate entirely | Remove benchgate, its tests, and any docs. Contributors who want comparisons can use upstream `benchstat` directly. Smallest surface area. | ✓ |
| Keep benchgate but mark deprecated | Keep the code with a deprecation notice; revisit removal later. Lowest risk but accumulates stale code. | |

**User's choice:** Delete entirely.

### Sub-question 2b: hosted-named baselines (`v1.*-github-hosted.txt`)

| Option | Description | Selected |
|--------|-------------|----------|
| Delete all baselines | Remove every test/bench/baselines/v1.*.txt file. Cleanest break. | ✓ |
| Rename + keep as historical-local artifacts | Rename to e.g. v1.1-darwin-arm64.txt to remove the misleading 'github-hosted' suffix and preserve as historical reference. | |
| Move to .planning/historical-baselines/ | Move outside the active test tree. Out of the way but still in repo for archeology. | |

**User's choice:** Delete all.

### Sub-question 2c: `test/bench/baselines/README.md`

| Option | Description | Selected |
|--------|-------------|----------|
| Rewrite for local-only flow | Rewrite the README to document local-only flow: what the directory is for, how to capture a baseline locally, where contributors put committed baselines (or whether they're committed at all). | ✓ |
| Delete README.md and the baselines/ dir | Delete the whole directory. Forces the question of where contributors keep their local baseline. | |
| Replace with a one-line README | One-liner: "Local baseline capture lives here — see CONTRIBUTING.md." Gitignore the *.txt files. | |

**User's choice:** Rewrite for local-only flow (keep the directory).

### Sub-question 2d: PR-time regression safety net

| Option | Description | Selected |
|--------|-------------|----------|
| Nothing — honor system | No PR template, no docs reminder, no checklist. Bench impact is the contributor's responsibility, mentioned only in CONTRIBUTING.md. Lowest friction. | ✓ |
| PR template checkbox | Add `.github/pull_request_template.md` with a checkbox for confirming local bench run on hot-path changes. | |
| CONTRIBUTING.md prominent section only | Clear "Bench impact" subsection in CONTRIBUTING.md, no PR template. | |

**User's choice:** Honor system.

---

## Area 3: Local bench command surface

### Sub-question 3a: invocation shape

| Option | Description | Selected |
|--------|-------------|----------|
| Make targets: `bench` + `bench-baseline` | Two targets, easy to discover; mirrors existing `make bench-jdtls-warm` pattern. | ✓ |
| Single `make bench` target | One target; capture-to-file is documented `go test ... \| tee path` invocation in CONTRIBUTING.md. | |
| Documented `go test` invocations only | No new Make targets. CONTRIBUTING.md documents canonical invocations. Contributors copy-paste. | |
| Three targets: `bench`, `bench-baseline`, `bench-compare` | Most polished DX. (Conflicts with deleting benchgate — would use upstream benchstat.) | |

**User's choice:** Two targets.

### Sub-question 3b: `make bench-baseline` filename

| Option | Description | Selected |
|--------|-------------|----------|
| Required NAME parameter | `make bench-baseline NAME=v1.9-local` writes test/bench/baselines/v1.9-local.txt. Errors if NAME is missing. Avoids accidental overwrites. | |
| Auto-derive from milestone + uname | Reads current milestone from STATE.md and appends `$(uname -s)-$(uname -m)`. Zero-args invocation. | |
| Single fixed path | Always writes the same file and overwrites. Loses history but simplest. | ✓ |

**User's choice:** Single fixed path. *Rejected the recommended NAME-parameter option in favor of the simplest possible invocation.*

### Sub-question 3c: git status of captured baselines

| Option | Description | Selected |
|--------|-------------|----------|
| Gitignored — each contributor has their own | The captured file is gitignored. Personal scratch artifacts. The baselines/ folder stays in repo with just README.md. | ✓ |
| Committed — shared project baseline | A single committed baseline file lives in repo. Contributors update it via `make bench-baseline`. Shared reference but invites bench-result drift in PRs. | |
| No baselines/ directory at all | Delete the folder. `make bench-baseline` writes to /tmp or .bench-out/ (gitignored). Zero footprint. | |

**User's choice:** Gitignored.

---

## Area 4: PROJECT.md tech-debt rewrite

| Option | Description | Selected |
|--------|-------------|----------|
| Delete the bench part entirely | Remove the entire bench sentence. Local-only is the chosen design, not tech debt. Other items in the note (rust-analyzer, jdtls cold-start) stay. The GrammarRegistry item (resolved by Phase 49) gets cleaned up while editing. | ✓ |
| Rewrite as design statement (move out of tech-debt) | Move 'Benchmarks run locally only — hosted CI bench gating is intentionally not pursued' out of tech-debt and into Constraints or Key Decisions. Frames it as policy. | |
| Keep a note about gopls v0.17.1 history | Replace the bench sentence with: 'gopls v0.17.1 had a Go 1.25 linux/amd64 issue — we use ≥v0.21'. Preserves historical context. | |

**User's choice:** Delete the bench part entirely (and clean up resolved GrammarRegistry item).

---

## Claude's Discretion

- Exact filename for the gitignored local baseline (e.g., `test/bench/baselines/local.txt`) — planner picks the most sensible single canonical path.
- Wording of new CONTRIBUTING.md sections (gopls subsection, bench-impact note) — planner drafts; user reviews.
- Whether to delete `bench.yml` and `capture-baseline.yml` in one commit each or together — both must land in the same plan; ordering is a planner detail.
- Cleanup ordering of the bench infrastructure deletions vs. doc updates — planner determines a sensible task sequence.

## Deferred Ideas

- **Self-hosted runner for bench gating.** Hypothetical "release-tier self-hosted runner" mentioned in the original (superseded) Phase 50 docs would still violate the no-CI-bench rule. Out of scope unless the rule is explicitly reversed.
- **Auto-comparison tooling for local baselines.** With benchgate deleted, contributors needing comparisons use upstream `benchstat`. A small standalone "compare two baselines" helper could be added in a future polish phase if practical pain emerges.
- **`NAME=` parameter or auto-uname suffix for `make bench-baseline`.** D-08 picks single fixed path. If the lack of history-keeping becomes painful, a future micro-phase can add naming support without breaking the simple invocation.
