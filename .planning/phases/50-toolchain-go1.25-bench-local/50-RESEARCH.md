# Phase 50: toolchain-go1.25-bench-local - Research

**Researched:** 2026-04-28
**Domain:** CI workflow surgery + doc/Makefile edits (no production code changes)
**Confidence:** HIGH

## Summary

Phase 50 is a low-novelty, doc-and-config phase that delivers two coupled outcomes: (A) keep the existing `go-test.yml` workflow as the single CI gate that proves build/vet/test green on `ubuntu-latest` with Go 1.25, and (B) tear down all hosted-CI bench plumbing and replace it with two `make` targets plus documentation. There is no new production code. Every decision is locked in CONTEXT.md (D-01..D-11). Research focused on confirming three external facts (gopls release timeline, `benchstat` install path, `go-test.yml` already meets criterion 1), inventorying every file/line to be deleted or edited, and proposing a Validation Architecture that is independently verifiable per the Nyquist gate.

**Primary recommendation:** Plan three plans — (1) CI/code deletion + go-test.yml verification, (2) Makefile + .gitignore + bench README rewrite, (3) PROJECT.md / USAGE.md / CONTRIBUTING.md doc edits. Each plan owns disjoint files; all three can be authored in parallel by the planner with a single sequential merge order at execution time. The phase is intentionally small — a single plan would also be defensible if the planner prefers to bundle the deletions and doc edits.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### gopls strategy
- **D-01:** gopls strategy is doc-only. No code or langregistry version pinning. Update CONTRIBUTING.md with a sub-section explaining: (i) gopls is installed at runtime by `internal/langregistry`, (ii) the project tests against `gopls@latest`, (iii) gopls v0.17.1 had a Go 1.25 linux/amd64 incompatibility that was resolved upstream by >=v0.21. Update USAGE.md troubleshooting note (line 515) to reflect this. *No `min_version` field, no exact version pin.*

#### Bench infra removal
- **D-02:** Delete `.github/workflows/bench.yml` and `.github/workflows/capture-baseline.yml`.
- **D-03:** Delete `test/bench/cmd/benchgate` entirely (the local Go CLI that compared benchstat output against a baseline). Contributors who want comparisons use upstream `benchstat` directly. Remove its package, tests, and any references in docs.
- **D-04:** Delete all `test/bench/baselines/v1.*.txt` files (v1.1-github-hosted.txt, v1.2-phase10/11/12-github-hosted.txt). They reference an old comparison flow that is being dismantled. No "historical-local" preservation, no rename.
- **D-05:** Keep `test/bench/baselines/` directory in repo. Rewrite `test/bench/baselines/README.md` to document the local-only flow: what the directory is for, how to capture a baseline locally (`make bench-baseline`), and that captured baselines are gitignored personal artifacts.
- **D-06:** No PR-time safety net — honor system. No `.github/pull_request_template.md` checkbox. Bench impact responsibility is mentioned in CONTRIBUTING.md only.

#### Local bench command surface
- **D-07:** Add two Make targets, mirroring the `bench-jdtls-warm` self-doc style already in the Makefile:
  - `make bench` — runs the bench suite once and prints to stdout. Equivalent to: `go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...`
  - `make bench-baseline` — same command, captured into a single fixed file path (overwrites on each run). The exact path is a planning-time decision, expected to be `test/bench/baselines/local.txt` or similar single canonical location.
- **D-08:** `make bench-baseline` writes to a **single fixed path** (no `NAME=` parameter, no auto-derived `uname` suffix). Simplest invocation; overwrites on each run. Loses history-keeping but matches the user preference for minimal Make surface.
- **D-09:** The captured baseline file is **gitignored**. Add the canonical path (e.g. `test/bench/baselines/local.txt`) to `.gitignore`. Captured baselines are personal scratch artifacts; nothing is committed.

#### PROJECT.md tech-debt cleanup
- **D-10:** Delete the bench sentence from PROJECT.md tech-debt note (line 139): the part beginning "Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest...". Local-only is the chosen design, not tech debt.
- **D-11:** While editing line 139, also remove the GrammarRegistry sentence ("3 redundant GrammarRegistry instances (functionally correct)") — already resolved by Phase 49. The remaining items in the note (rust-analyzer rename quirk, jdtls cold-start) stay.

### Claude's Discretion
- The exact filename for the gitignored local baseline (D-08, D-09) — planner picks the most sensible path within `test/bench/baselines/`.
- Wording of CONTRIBUTING.md sections (gopls subsection per D-01, bench-impact note per D-06) — planner drafts; user reviews.
- Whether to delete `.github/workflows/bench.yml`/`capture-baseline.yml` in one commit each or together — planner decides; both must land in the same plan.

### Deferred Ideas (OUT OF SCOPE)
- **Self-hosted runner for bench gating.** Contradicts the project's no-CI-bench rule. Would need fresh requirements and explicit rule reversal.
- **Auto-comparison tooling for local baselines.** `benchgate` is being deleted; contributors use upstream `benchstat`. A small "compare two baselines" helper could be added in a future polish phase if needed.
- **Bench-baseline NAME parameter / history-keeping.** D-08 picks single fixed path for simplicity; can be revisited later if pain emerges.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TOOL-01 | Serena builds, vets, and tests green on `ubuntu-latest` with Go 1.25 and a compatible gopls version — root cause of gopls v0.17.1 linux/amd64 incompatibility resolved (upgrade, patch, or replacement). Benchmarks explicitly out of scope for CI per local-only rule. | Confirmed `.github/workflows/go-test.yml` already targets `ubuntu-latest` with Go 1.25.x and runs `go vet ./...` + `go test ./... -count=1` (no gopls install — runtime LS install via `internal/langregistry`). gopls release timeline confirmed: v0.21.x is the current resolution per the upstream release cadence (v0.17.1 Dec 2024 → v0.21.x late 2025; v0.21.0 explicitly references Go 1.25 features). Strategy per D-01 is doc-only — no code changes needed for TOOL-01. |
| TOOL-02 | Bench harness is local-only — `bench.yml`, `capture-baseline.yml`, and `*-github-hosted.txt` baselines removed; benches runnable via `make bench` / `make bench-baseline`; flow documented in CONTRIBUTING.md and `test/bench/baselines/README.md`; PROJECT.md tech-debt note cleaned. | Confirmed `benchgate` is referenced only in 4 in-tree locations (all to be deleted/rewritten in this phase): `test/bench/cmd/benchgate/{main,main_test}.go`, `test/bench/baselines/README.md`, `.github/workflows/bench.yml`. Historical `.planning/` artifacts mention `benchgate` and are NOT touched. Existing `bench-jdtls-warm` Makefile target establishes the self-doc style (`## help-text` after the colon) that new targets must mirror. Upstream `benchstat` install one-liner confirmed: `go install golang.org/x/perf/cmd/benchstat@latest`. |
</phase_requirements>

## Architectural Responsibility Map

This phase changes only CI configuration, build tooling, and documentation. There are no runtime application capabilities to assign to architectural tiers. The map below records that explicitly so the planner does not waste cycles trying to assign tier ownership for a code-free phase.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| CI build/vet/test gate | CI infra (GitHub Actions) | — | `.github/workflows/go-test.yml` is the only CI surface this phase touches; it already exists and runs on `ubuntu-latest` with Go 1.25.x. |
| Local benchmark execution | Build tooling (Makefile) | Developer machine | `make bench` and `make bench-baseline` are pure developer ergonomics — no daemon, no LS, no MCP. Same `go test -bench` invocation as the deleted `bench.yml`. |
| Documentation of toolchain + bench policy | Markdown docs | — | CONTRIBUTING.md, USAGE.md, PROJECT.md, `test/bench/baselines/README.md` carry the policy; no application code reads these. |
| Gitignore of local baseline artifact | Repo-root config | — | `.gitignore` only — no programmatic enforcement. |

## Standard Stack

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Go | 1.25.x | Build / vet / test toolchain on CI runner | Already pinned in `go-test.yml` step `actions/setup-go@v5 with go-version: '1.25.x'`. Matches `go.mod` toolchain. |
| gopls | `@latest` (>= v0.21.x) | Runtime LS for Go integration tests / benches; installed by `internal/langregistry` on demand | `internal/langregistry/languages.go:158` does NOT pin a version (`Command: "gopls"` with no version field). Project policy is "test against gopls@latest." v0.21.x is the first contemporaneous gopls release with Go 1.25 awareness (release notes mention `strings.Cut` introduced in Go 1.25). |
| benchstat | latest | Compare two benchfmt outputs locally (replaces the deleted `benchgate`) | Canonical upstream tool from `golang.org/x/perf`. Install one-liner: `go install golang.org/x/perf/cmd/benchstat@latest`. Cited in CONTRIBUTING.md per D-06. |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| GNU make | any | Driver for `make bench` / `make bench-baseline` | Already used by existing `bench-jdtls-warm`; no new dependency. |
| `tee` (POSIX) | n/a | Optional, for `make bench-baseline` if planner prefers a "show + capture" form | Used in deleted `bench.yml` (`tee /tmp/bench-new.txt`). Planner discretion. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `benchstat` | Custom `benchgate` CLI (deleted per D-03) | benchgate locked in v1.2-era CI thresholds (15%/25% time/allocs at p<0.05). Local contributors don't need configurable gating — they just need a side-by-side diff. `benchstat` does that out of the box. |
| Pinning gopls in `internal/langregistry` | Doc-only strategy (D-01) | A pin would be a code change with version-bump maintenance. The doc-only strategy matches existing reality (no version field today) and lets `gopls@latest` carry the project as upstream tracks Go releases. |
| Multiple `make bench-*` targets (e.g. `bench-compare`) | Two-target surface (D-07, D-08) | User explicitly preferred minimal Make surface. `bench-compare` would re-introduce a benchgate-shaped responsibility that D-03 deleted. |

**Installation (for the new CONTRIBUTING.md "Running Benchmarks" section):**
```bash
# benchstat is not part of the Serena build; it is an optional contributor convenience
go install golang.org/x/perf/cmd/benchstat@latest
```

**Version verification:** `gopls` is installed at runtime by `internal/langregistry`; the project does not pin gopls in any `go.sum` or `go.mod` constraint. `benchstat` is not a Serena dependency at all — it is a local-only contributor tool. No `go.mod` updates required for this phase.

## Architecture Patterns

### System (Pre/Post) Diagram

```
PRE (current state)
┌─────────────────┐         ┌───────────────────────┐
│ PR / push to    │────────▶│ go-test.yml           │  KEEP
│ main            │         │ ubuntu-latest, Go 1.25│
│                 │         │ go vet + go test      │
│                 │         └───────────────────────┘
│                 │         ┌───────────────────────┐
│                 │────────▶│ bench.yml             │  DELETE
│                 │         │ ubuntu-latest         │
│                 │         │ benchgate gate        │
│                 │         └───────────────────────┘
└─────────────────┘
       │ (manual workflow_dispatch)
       └──────────────────────▶┌───────────────────────┐
                               │ capture-baseline.yml  │  DELETE
                               │ commits *-github-     │
                               │ hosted.txt            │
                               └───────────────────────┘

POST (target state)
┌─────────────────┐         ┌───────────────────────┐
│ PR / push to    │────────▶│ go-test.yml           │  KEEP
│ main            │         │ ubuntu-latest, Go 1.25│  (criterion 1)
│                 │         │ go vet + go test      │
└─────────────────┘         └───────────────────────┘

┌─────────────────┐         ┌───────────────────────┐
│ Developer       │────────▶│ make bench            │  NEW
│ machine         │         │ go test -bench        │
│                 │         │ → stdout              │
│                 │         └───────────────────────┘
│                 │         ┌───────────────────────┐
│                 │────────▶│ make bench-baseline   │  NEW
│                 │         │ → test/bench/baselines│
│                 │         │   /local.txt          │  (gitignored)
│                 │         └───────────────────────┘
│                 │         ┌───────────────────────┐
│                 │────────▶│ benchstat A B         │  CONTRIBUTOR-INSTALLED
│                 │         │ (optional comparison) │
└─────────────────┘         └───────────────────────┘
```

### Recommended File Topology
```
.github/workflows/
├── go-test.yml             # KEEP — already correct
├── bench.yml               # DELETE
├── capture-baseline.yml    # DELETE
└── pytest.yml              # UNCHANGED (legacy Python; gopls@latest install is independent)

test/bench/
├── tools_bench_test.go     # KEEP — the bench suite stays
├── (other *_test.go)       # KEEP
├── cmd/
│   └── benchgate/          # DELETE entire directory (main.go + main_test.go)
└── baselines/
    ├── README.md                          # REWRITE for local-only
    ├── v1.1-github-hosted.txt             # DELETE
    ├── v1.2-phase10-github-hosted.txt     # DELETE
    ├── v1.2-phase11-github-hosted.txt     # DELETE
    ├── v1.2-phase12-github-hosted.txt     # DELETE
    └── local.txt                          # NEW path (gitignored, not committed)

Makefile                    # ADD bench, bench-baseline targets; ADD them to .PHONY
.gitignore                  # ADD test/bench/baselines/local.txt
CONTRIBUTING.md             # EDIT — gopls subsection (D-01); rewrite "Running Benchmarks" + "Benchmark CI Gate" sections
USAGE.md                    # EDIT — line 515 troubleshooting note
.planning/PROJECT.md        # EDIT — line 139 tech-debt sentence
```

### Pattern 1: Self-documenting Makefile target
**What:** Targets carry a `## help text` comment after the colon. The Serena Makefile already uses this style for `bench-jdtls-warm`.
**When to use:** Every new `.PHONY` target users invoke directly.
**Example (verbatim from current Makefile lines 37-42):**
```make
bench-jdtls-warm: ## Run Java integration suite cold then warm; print both wall-clocks
	@$(MAKE) clean-jdtls-cache
	@echo "=== jdtls COLD run ==="
	-@time $(GO) test -run 'Java' ./test/integration/... -count=1
	@echo "=== jdtls WARM run ==="
	-@time $(GO) test -run 'Java' ./test/integration/... -count=1
```

**Suggested shape for new targets (planner finalizes wording):**
```make
bench: ## Run the bench suite once and print results to stdout
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...

bench-baseline: ## Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee test/bench/baselines/local.txt
```

Notes for the planner:
- The `$$` is required in Make to escape `$` for the shell — the deleted `bench.yml` used the same `-run=^$` pattern.
- Add both targets to the `.PHONY` line at the top of the Makefile (currently `clean-jdtls-cache bench-jdtls-warm`).
- The `tee` form is one option; a redirect form (`> test/bench/baselines/local.txt`) is equally valid and avoids the optional `tee` dependency. Planner picks one.

### Anti-Patterns to Avoid
- **Re-introducing benchgate-shaped logic in Make.** The whole point of D-03 is to delete custom comparison code. Do not write a `bench-compare` target that wraps `benchstat` with thresholds — that recreates `benchgate` in shell.
- **Auto-detect platform suffix in baseline filename.** D-08 explicitly forbids `uname`-derived names. Single fixed path, end of story.
- **Pin gopls in `internal/langregistry/languages.go`.** D-01 is doc-only. Editing the LSEntry would create a code change that contradicts the locked decision.
- **Editing historical `.planning/` artifacts** that mention `benchgate` (e.g. `RETROSPECTIVE.md`, `milestones/v1.2-*.md`). Those are immutable historical records — only in-tree code/config/docs change.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Compare two benchmark runs | A new Make target / shell script | Direct invocation of `benchstat A.txt B.txt` (contributor installs locally) | benchstat is the canonical upstream tool, parses benchfmt natively, computes Welch's t-test, normalizes units. Hand-rolling reduces to re-implementing benchgate, which D-03 deletes. |
| Detect "compatible gopls version" in code | A `min_version` field on LSEntry, version probe at activation | Doc-only guidance in CONTRIBUTING.md (D-01) | The Serena binary should not embed gopls version policy — that policy changes faster than Serena releases. |
| Capture baseline with milestone tagging | A NAME=v1.X parameter or workflow input | Single fixed path `test/bench/baselines/local.txt` (D-08) | Local baselines are personal scratch; the moment a contributor needs more they can `cp local.txt my-experiment.txt` manually. Make surface stays minimal. |

**Key insight:** This phase's architectural value is *removing* code, not adding it. Any new tooling beyond two trivial Make targets is scope creep.

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — verified by grep for `benchgate`, `bench-baseline`, `local.txt` across the repo. The bench suite stores no persistent state across runs. | None |
| Live service config | GitHub Actions workflows `bench.yml` and `capture-baseline.yml` are committed YAML, not live external config. Deleting them removes them from the next push/PR. No external service (e.g. Codecov, external bench dashboard) consumes these workflow outputs — the artifacts are uploaded only to GitHub Actions ephemeral storage. | Delete the YAML files; no external coordination needed. |
| OS-registered state | None — no Task Scheduler, launchd, systemd, pm2 entries reference `benchgate` or any phase artifact. | None |
| Secrets/env vars | None — `bench.yml`/`capture-baseline.yml` use only `GOMAXPROCS` and `GOPLS_VERSION` (literal values, not secrets). No `secrets.*` references. No env vars consumed by `benchgate` source. | None |
| Build artifacts / installed packages | The deleted `test/bench/cmd/benchgate` is `package main` with `go run` invocation — never compiled into a persistent binary in the repo. `go install`-class artifacts in contributors' `$GOPATH/bin` may include a stale `benchgate` only if a contributor previously ran `go install ./test/bench/cmd/benchgate`; this is harmless. CONTRIBUTING.md may optionally note "if you previously installed benchgate, you can `rm $(go env GOPATH)/bin/benchgate`." Planner discretion (low value). | None blocking. |

**The canonical question:** *After every file in the repo is updated, what runtime systems still have the old string cached, stored, or registered?* — Answer: nothing. This is the cleanest possible deletion: no databases, no live services, no OS state, no secrets.

## Common Pitfalls

### Pitfall 1: Deleting workflows but leaving stale references
**What goes wrong:** A doc still says "see `.github/workflows/bench.yml`" or "the benchgate gate" after the files are gone, breaking links and confusing readers.
**Why it happens:** CONTRIBUTING.md currently has a "Benchmark CI Gate" section (lines 178-184) and a "Running Benchmarks" section (lines 130-152) that reference both the workflow file and the in-tree `benchgate`. PROJECT.md line 139 references CI ubuntu-latest baselines. USAGE.md line 515 references `capture-baseline.yml`.
**How to avoid:** Treat the deletion + doc-edit as one atomic plan. Run a final `grep -rn "benchgate\|bench.yml\|capture-baseline\|github-hosted" --include="*.md" --include="*.go" --include="Makefile"` excluding `.planning/` history dirs as a verification step before merge.
**Warning signs:** A `grep` hit anywhere outside `.planning/{milestones,phases/_old,RETROSPECTIVE.md}` after the phase is "done."

### Pitfall 2: Forgetting `.PHONY`
**What goes wrong:** `make bench` silently does nothing if a file/dir named `bench` ever appears in repo root.
**Why it happens:** The current `.PHONY` line lists `clean-jdtls-cache bench-jdtls-warm` but not the new targets.
**How to avoid:** Add `bench bench-baseline` to the `.PHONY` line (Makefile line 1).
**Warning signs:** `make bench` prints `make: 'bench' is up to date.`

### Pitfall 3: Gitignoring the wrong path
**What goes wrong:** `.gitignore` adds a pattern that also ignores the new `test/bench/baselines/README.md` (e.g. ignoring the whole `baselines/` directory) — losing the rewritten doc.
**Why it happens:** Easy to write `test/bench/baselines/` instead of `test/bench/baselines/local.txt`.
**How to avoid:** Use the exact filename, not a directory prefix. Verify with `git status` after creating a dummy `local.txt`.
**Warning signs:** README.md vanishing from `git ls-files`.

### Pitfall 4: gopls release-version drift in CONTRIBUTING.md
**What goes wrong:** Hard-coding "v0.21.1" in CONTRIBUTING.md ages immediately. Later contributors may install a much newer version.
**Why it happens:** Tempting to mirror the deleted `bench.yml`'s pinned `GOPLS_VERSION: v0.21.1`.
**How to avoid:** Phrase the doc as "gopls v0.17.1 had a Go 1.25 incompatibility resolved in upstream gopls v0.21 and later. Run `go install golang.org/x/tools/gopls@latest` to pick up the fix." Mention `>=v0.21` as a floor, not as a pin.
**Warning signs:** A version string with no "or later" qualifier.

### Pitfall 5: Editing PROJECT.md line 139 incorrectly
**What goes wrong:** Removing too much (e.g. dropping the rust-analyzer or jdtls sentences that are still active tech debt) or too little (leaving the bench fragment).
**Why it happens:** Line 139 is a single dense paragraph with four comma-separated sentences. Surgical edit required.
**How to avoid:** The exact remove targets per D-10/D-11:
1. **Remove (D-10):** `Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest due to gopls v0.17.1 incompatibility with Go 1.25 on linux/amd64.`
2. **Remove (D-11):** `3 redundant GrammarRegistry instances (functionally correct).`
3. **Keep:** `rust-analyzer v1.90 \`textDocument/rename\` returns "No references found at position" in temp workspaces despite hover/references/search working at the same position — upstream LS bug, all other Rust operations work (see USAGE.md Troubleshooting).` AND `jdtls cold-start indexing exceeds 2min in temp workspaces — Java integration tests gated behind \`-short=false\`.`
**Warning signs:** The "Known tech debt:" paragraph either still mentions baselines/ubuntu-latest or has lost the rust-analyzer/jdtls sentences.

## Code Examples

### `make bench-baseline` redirect form
```make
bench-baseline: ## Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... > test/bench/baselines/local.txt
	@echo "captured baseline at test/bench/baselines/local.txt"
```

### `benchstat` install one-liner for CONTRIBUTING.md
```bash
# Source: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
go install golang.org/x/perf/cmd/benchstat@latest
```

### Suggested CONTRIBUTING.md "Running Benchmarks" replacement skeleton (planner finalizes prose)
```markdown
## Running Benchmarks

The benchmark suite lives in `test/bench/`. **Benchmarks run locally only** — the project does not run benchmarks on CI runners because shared GitHub-hosted runners produce noisy, untrustable baselines.

Run the suite once and print to stdout:

```sh
make bench
```

Capture a local baseline (overwrites `test/bench/baselines/local.txt`, which is gitignored):

```sh
make bench-baseline
```

To compare two captures, install `benchstat` and run it directly:

```sh
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

Key details:

- Benchmarks use `testing.B.Loop` (Go 1.24+).
- `test/bench/baselines/` is the conventional capture location; everything you save there is gitignored.
- If your changes are likely to affect bench numbers, mention the local before/after deltas in your PR description.
```

### Suggested USAGE.md line 515 replacement skeleton
```markdown
### gopls version compatibility

**Symptom:** Build failures or unexpected behavior from the Go integration tests after upgrading Go.

**Cause:** Older gopls releases (notably v0.17.1) predate Go 1.25 and exhibit incompatibilities on linux/amd64. The fix shipped upstream in gopls v0.21 and later.

**Fix:** Update gopls after every Go upgrade:

```bash
go install golang.org/x/tools/gopls@latest
```

Serena does not pin a gopls version — `internal/langregistry` invokes whatever `gopls` is on `$PATH`.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Custom `benchgate` CLI with tiered thresholds, p-value gating, blocking PR mode | Local `benchstat` invocation (manual) | Phase 50 (this phase) | Removes ~750 LOC of in-tree comparison code; defers regression detection to contributor judgment per project rule. |
| `bench.yml` + `capture-baseline.yml` running on `ubuntu-latest` with pinned gopls | No CI bench at all; `make bench`/`make bench-baseline` for local runs | Phase 50 | Aligns with project-wide "no CI benches" rule. Frees CI minutes; eliminates noisy baselines. |
| Pinned `GOPLS_VERSION: v0.21.1` in deleted bench.yml | `gopls@latest` documented in CONTRIBUTING.md | Phase 50 (and pre-existing in `internal/langregistry`) | Doc reflects existing reality of the registry; no version-bump churn in code. |

**Deprecated/outdated:**
- `test/bench/cmd/benchgate` — superseded by upstream `benchstat`.
- `test/bench/baselines/v1.*-github-hosted.txt` — captured against a CI runner class the project no longer uses for bench gating.
- `.github/workflows/{bench,capture-baseline}.yml` — superseded by Make targets per the no-CI-bench rule.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | gopls v0.21.x is the floor where the Go 1.25 linux/amd64 incompatibility is resolved upstream. Inferred from (a) v0.21.0 release notes explicitly referencing Go 1.25 stdlib features (`strings.Cut` analyzer), (b) v0.21.1 being the version the project's now-deleted `bench.yml` self-pinned to, (c) gopls support policy of tracking the most recent two Go major releases. | gopls strategy / CONTRIBUTING.md wording | LOW. Even if the precise floor is v0.20.x rather than v0.21, the doc-only strategy says "use `gopls@latest`" — the `>=v0.21` qualifier is a sanity guide, not a load-bearing pin. If a sharper version is needed, change the prose; no code changes downstream. |
| A2 | Deleting `bench.yml` and `capture-baseline.yml` does not break any other workflow. `go-test.yml` is independent (no `needs:` reference to bench), and no external service (e.g. badge endpoint) reads bench artifacts. | Workflow deletion plan | LOW. Verified by reading `go-test.yml` (no bench dependency) and confirming no `needs: bench` strings exist in other workflow files. README.md does not embed a bench-status badge. |
| A3 | `pytest.yml` (legacy Python) is unaffected by this phase. It installs `gopls@latest` for its own purposes (legacy Python tests use gopls fixtures), independent of the Go bench/test path. | Phase scope | LOW. CONTEXT.md canonical_refs flags it for review only; the install line stays. |

**If this table is empty:** N/A — three assumptions tagged. All are LOW risk and easily corrected by doc edits if reality differs.

## Open Questions

1. **Exact filename for the local baseline.**
   - What we know: D-08 mandates a single fixed path; D-09 mandates it is gitignored.
   - What's unclear: Whether `test/bench/baselines/local.txt` is the right name, vs. `local-baseline.txt`, `current.txt`, or `latest.txt`.
   - Recommendation: `test/bench/baselines/local.txt` — matches the README's existing "local-only flow" framing and is the shortest unambiguous name. Planner can choose otherwise within `test/bench/baselines/`.

2. **One plan or three?**
   - What we know: Phase is small (~10 file deletions, ~5 file edits, 2 Make target additions, 1 .gitignore line).
   - What's unclear: Whether to bundle into a single execution plan or split by concern (CI deletions, build/Make additions, doc edits).
   - Recommendation: Three small plans is the natural shape and parallelizable for execution-agent review, but a single plan is defensible. Planner decides per `.planning/config.json` `granularity: coarse`.

3. **Optional: stale `benchgate` cleanup hint in CONTRIBUTING.md.**
   - What we know: Contributors who previously `go install ./test/bench/cmd/benchgate` have a stale binary in `$GOPATH/bin/`.
   - What's unclear: Whether to mention this in CONTRIBUTING.md (one extra sentence) or leave it for individual cleanup.
   - Recommendation: Skip — extremely low audience size (only those who explicitly ran `go install` against an in-tree CLI tool).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | `make bench`, CI workflows | ✓ | 1.25.x (per `go-test.yml` and project `go.mod`) | — |
| GNU make | new Makefile targets | ✓ | system default (already used by `bench-jdtls-warm`) | — |
| gopls | bench suite (loads Go fixtures via LS) | ✓ | runtime install via `internal/langregistry` (no Serena-build dep) | — |
| benchstat | optional local comparisons (CONTRIBUTING.md) | n/a | contributor installs on demand via `go install golang.org/x/perf/cmd/benchstat@latest` | document install path; not bundled |
| GitHub Actions ubuntu-latest runner | `go-test.yml` (criterion 1) | ✓ | provided by GitHub | — |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** `benchstat` is not present on a fresh contributor machine — but the install one-liner is documented and it's an optional convenience, not a requirement to run `make bench`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go's built-in `testing` (no third-party harness) |
| Config file | None — `go test` is self-configuring |
| Quick run command | `go vet ./... && go test ./... -count=1 -short` |
| Full suite command | `go test ./... -count=1` (mirrors `go-test.yml`) |

### Phase Requirements → Test Map
Because this phase is doc-and-config, "tests" here are mostly grep / diff assertions and CI-job outcomes rather than Go unit tests.

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TOOL-01 | CI build/vet/test green on ubuntu-latest with Go 1.25 | smoke (CI) | `gh run list --workflow=go-test.yml --branch=main --limit=1 --json conclusion -q '.[0].conclusion'` should equal `success` after merge | ✅ go-test.yml (existing) |
| TOOL-01 | Local Go vet passes | unit | `go vet ./...` | ✅ existing |
| TOOL-01 | Local Go test passes (no new code, sanity) | unit | `go test ./... -count=1 -short` | ✅ existing |
| TOOL-02 | No `bench.yml` or `capture-baseline.yml` workflows remain | grep | `! ls .github/workflows/bench.yml .github/workflows/capture-baseline.yml 2>/dev/null` | n/a (deletion check) |
| TOOL-02 | No `benchgate` references in code or docs (excluding `.planning/` history) | grep | `! grep -rn benchgate --include='*.go' --include='*.md' --include='*.yml' --include='Makefile' . 2>/dev/null \| grep -v '\.planning/'` | n/a (deletion check) |
| TOOL-02 | `make bench` target exists and runs (smoke) | smoke | `make -n bench` (dry-run; planner may choose `make bench` against a tiny `-benchtime=1x` invocation) | ❌ Wave 0 — Makefile addition |
| TOOL-02 | `make bench-baseline` target exists, writes to gitignored path | smoke + grep | `make -n bench-baseline` and `git check-ignore test/bench/baselines/local.txt` | ❌ Wave 0 — Makefile + .gitignore additions |
| TOOL-02 | `*-github-hosted.txt` baselines are gone | grep | `! ls test/bench/baselines/v1.*-github-hosted.txt 2>/dev/null` | n/a (deletion check) |
| TOOL-02 | `test/bench/baselines/README.md` documents local-only flow | grep | `grep -q 'local' test/bench/baselines/README.md && grep -q 'gitignored' test/bench/baselines/README.md && ! grep -q 'benchgate' test/bench/baselines/README.md` | ❌ Wave 0 — rewrite |
| TOOL-02 | CONTRIBUTING.md has a gopls subsection mentioning v0.21+ floor | grep | `grep -q 'v0.21' CONTRIBUTING.md && grep -q 'gopls' CONTRIBUTING.md` | ❌ Wave 0 — doc edit |
| TOOL-02 | USAGE.md line ~515 no longer says "active bug" / no longer references `capture-baseline.yml` | grep | `! grep -q 'capture-baseline' USAGE.md` | ❌ Wave 0 — doc edit |
| TOOL-02 | PROJECT.md tech-debt note no longer mentions GrammarRegistry or CI ubuntu-latest baselines | grep | `! grep -q 'GrammarRegistry instances' .planning/PROJECT.md && ! grep -q 'CI ubuntu-latest' .planning/PROJECT.md` | ❌ Wave 0 — doc edit |
| TOOL-02 | rust-analyzer + jdtls tech-debt sentences preserved | grep | `grep -q 'rust-analyzer' .planning/PROJECT.md && grep -q 'jdtls cold-start' .planning/PROJECT.md` | ❌ Wave 0 — doc edit (negative-protection check) |
| TOOL-02 | `.gitignore` ignores the canonical local baseline path | unit | `git check-ignore test/bench/baselines/local.txt` (exit 0) | ❌ Wave 0 — .gitignore edit |

### Sampling Rate
- **Per task commit:** `go vet ./... && go test ./... -count=1 -short` (≤30s on this codebase). For doc-only commits, run the relevant grep block from the table above.
- **Per wave merge:** Full grep matrix above + `go test ./... -count=1` (no `-short`).
- **Phase gate:** Push to a PR branch, observe `go-test.yml` go green on `ubuntu-latest`. CI green on the merge commit is the criterion-1 acceptance signal.

### Wave 0 Gaps
- [ ] `Makefile` — `bench` and `bench-baseline` targets do not yet exist
- [ ] `.gitignore` — no entry for the canonical local-baseline path
- [ ] `test/bench/baselines/README.md` — currently documents the deleted CI flow
- [ ] `CONTRIBUTING.md` — currently lacks a gopls compatibility subsection; "Running Benchmarks" + "Benchmark CI Gate" sections describe the deleted flow
- [ ] `USAGE.md` line 515 — currently describes the bug as active and references the deleted workflow
- [ ] `.planning/PROJECT.md` line 139 — currently contains the bench sentence (D-10) and GrammarRegistry sentence (D-11) to be removed
- [ ] No new test files needed — verification is grep + CI outcome, not unit tests

## Project Constraints (from CLAUDE.md)

These directives from `./CLAUDE.md` constrain this phase. The planner MUST verify any plan does not contradict them.

| Directive | Source line in CLAUDE.md | Relevance to Phase 50 |
|-----------|--------------------------|------------------------|
| `Always run go vet and go test before completing any Go task.` | "Go Development Commands" section | Quick-run gate per task commit; embedded in Validation Architecture sampling. |
| `Single Go binary, no Python, Docker, or runtime dependencies` | Project section | Forbids any new tool-binary dependency. `benchstat` is contributor-installed, NOT bundled. ✅ |
| `cmd/serena/main.go ... single entrypoint` | Architecture section | This phase touches no `cmd/` code. ✅ |
| `LSP only: No JetBrains or proprietary backends` | Constraints | Not relevant — phase touches no LS adapters. ✅ |
| GSD workflow enforcement: edits go through GSD commands | "GSD Workflow Enforcement" | Already satisfied by being in `/gsd-research-phase` → `/gsd-plan-phase`. |
| **Project rule (HARD): Benchmarks are local-only — never on CI** | User memory `feedback_no_ci_benchmarks.md` (referenced in CLAUDE.md preamble) | This is the entire reason for Phase 50 (B). Plans MUST NOT propose any CI bench step or self-hosted runner. |

## Sources

### Primary (HIGH confidence)
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.planning/phases/50-toolchain-go1.25-bench-local/50-CONTEXT.md` — locked decisions D-01..D-11
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.planning/REQUIREMENTS.md` — TOOL-01, TOOL-02 wording (revised 2026-04-28)
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.github/workflows/go-test.yml` — confirms criterion-1 surface already exists (ubuntu-latest, Go 1.25.x, vet+test, no gopls install — runtime install via langregistry)
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.github/workflows/bench.yml` — confirms exact bench invocation (`go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...`) and pinned `GOPLS_VERSION: v0.21.1`
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/Makefile` — `bench-jdtls-warm` self-doc style, lines 37-42; `.PHONY` line 1
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/langregistry/languages.go:158` — Go LSEntry has `Command: "gopls"` with no version field
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.planning/PROJECT.md:139` — full text of tech-debt sentence to surgically edit
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/USAGE.md:511-529` — full text of gopls troubleshooting section to rewrite
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/test/bench/baselines/README.md` — current README content (134 lines, all to be replaced)
- `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/CONTRIBUTING.md:130-184` — current "Running Benchmarks" + "Benchmark CI Gate" sections to rewrite
- [pkg.go.dev — benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) — canonical install path

### Secondary (MEDIUM confidence)
- [github.com/golang/tools releases](https://github.com/golang/tools/releases) — gopls release timeline (v0.17.1 Dec 2024 → v0.21.0 Dec 2025)
- [go.dev/gopls/release/v0.21.0](https://go.dev/gopls/release/v0.21.0) — confirms v0.21.0 mentions Go 1.25 features (`strings.Cut` analyzer)
- [GitHub issue golang/go#76367 — x/tools/gopls: release version v0.21.0](https://github.com/golang/go/issues/76367) — release tracking checklist for v0.21.0

### Tertiary (LOW confidence)
- None — every load-bearing claim is anchored either in the in-tree files cited above or in upstream Go documentation. The "v0.21 is the floor" claim is corroborated by the project's own deleted `bench.yml` self-pinning to v0.21.1.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every tool referenced is either already in-tree (Go 1.25, gopls via langregistry) or has a one-line install (benchstat).
- Architecture: HIGH — phase has no novel architecture; just deletes + adds Make targets + edits docs. Pre/post diagram is exhaustive.
- Pitfalls: HIGH — pitfalls are mechanical (gitignore syntax, .PHONY, leftover references) and easily caught by the grep matrix in Validation Architecture.
- gopls floor version (v0.21+): MEDIUM — corroborated by deleted `bench.yml` pinning v0.21.1 + v0.21.0 release notes mentioning Go 1.25 features. The doc-only strategy makes this non-load-bearing.

**Research date:** 2026-04-28
**Valid until:** 2026-05-28 (30 days — stable infra/doc surface, no fast-moving APIs)
