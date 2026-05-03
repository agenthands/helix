# Phase 50: toolchain-go1.25-gopls-ci - Pattern Map

**Mapped:** 2026-04-25
**Files analyzed:** 4 (1 CI workflow modified, 1 CI workflow triggered-only, 2 docs modified, 1 baseline file produced by workflow)
**Analogs found:** 4 / 4 (all in-tree analogs; no external pattern needed)

## Scope Note

Phase 50 is a **docs + CI baseline-swap phase**. There are no Go source files, no MCP tools, no skills, no LSP wiring, no tests to author. All "patterns" here are file-internal — the analog for each modified file is the file itself (existing surrounding lines are the pattern). The planner should treat this as a copy-the-existing-line-style exercise, not a new-component build.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `.github/workflows/bench.yml` | config (CI workflow) | event-driven (PR trigger) | self (existing baseline-path lines 79-81, 88-90) | exact (in-file edit) |
| `.github/workflows/capture-baseline.yml` | config (CI workflow) | event-driven (workflow_dispatch) | self (no edits — invoked only) | n/a (read-only invocation) |
| `test/bench/baselines/v1.9-github-hosted.txt` | data (bench baseline artifact) | batch (capture output) | `test/bench/baselines/v1.1-github-hosted.txt` | exact (same producer, same flags) |
| `CONTRIBUTING.md` | docs (contributor guide) | n/a | self §`## Benchmark CI Gate` (lines 178-184) | exact (in-file insertion) |
| `.planning/PROJECT.md` | docs (project status) | n/a | self §`Known tech debt` paragraph (line 139) | exact (in-file deletion) |

## Pattern Assignments

### `.github/workflows/bench.yml` (config, event-driven)

**Analog:** itself — two existing lines reference the v1.1 baseline path. Per D-02, both must be swapped to v1.9. No structural change.

**Existing baseline-path pattern — Location 1, benchstat summary** (`.github/workflows/bench.yml:75-81`):

```yaml
      - name: Print benchstat summary
        # Human-readable diff for reviewers; benchgate does the actual
        # gating below using the benchfmt library path (Pitfall 9).
        run: |
          benchstat -alpha 0.05 \
            test/bench/baselines/v1.1-github-hosted.txt \
            /tmp/bench-new.txt
```

**Existing baseline-path pattern — Location 2, benchgate flag** (`.github/workflows/bench.yml:86-90`):

```yaml
      - name: Run benchgate
        run: |
          go run ./test/bench/cmd/benchgate \
            --baseline test/bench/baselines/v1.1-github-hosted.txt \
            --new /tmp/bench-new.txt
```

**Edit pattern:** Replace `v1.1-github-hosted.txt` with `v1.9-github-hosted.txt` in BOTH locations. Preserve indentation, line breaks, and the trailing path on a continuation line. Do not touch the surrounding `name:`, comments, or `--alpha` / `--new` flags.

**Header-comment pattern (lines 1-15)** — currently references "v1.1 baseline" in the file header:

```yaml
# Serena benchmark regression gate (Phase 9, D-01 PR tier).
#
# Runs the full bench suite on every PR against main, plus manual
# `workflow_dispatch`, and invokes benchgate to compare results against
# the committed v1.1 baseline. See test/bench/baselines/README.md for
# the three-step rollout (now in blocking mode).
```

The header text "the committed v1.1 baseline" should also be updated to "v1.9" for consistency (cosmetic — not strictly required by D-02 but matches the pattern of keeping comments accurate).

---

### `.github/workflows/capture-baseline.yml` (config, event-driven)

**Analog:** self — read-only this phase. Invoked once via:

```sh
gh workflow run capture-baseline.yml -f milestone=v1.9 --ref <branch>
```

**Existing flag-parity pattern (lines 29-33, 53-57)** — captures with the SAME flags `bench.yml` enforces (Pitfall 1):

```yaml
    env:
      # Must match bench.yml exactly (Pitfall 1).
      GOMAXPROCS: "4"
      GOPLS_VERSION: v0.21.1
      MILESTONE: ${{ inputs.milestone }}
```

```yaml
      - name: Run benchmark suite
        run: |
          set -euo pipefail
          go test -short -bench=. -benchmem -count=10 -run=^$ \
            ./test/bench/... | tee test/bench/baselines/${MILESTONE}-github-hosted.txt
```

**Auto-commit pattern (lines 68-72)** — produces the baseline file on the triggering branch:

```yaml
      - name: Commit baseline
        uses: stefanzweifel/git-auto-commit-action@v5
        with:
          commit_message: "bench: capture ${{ inputs.milestone }} baseline from ubuntu-latest CI"
          file_pattern: 'test/bench/baselines/*.txt'
```

**No edits.** Planner should call out this file as the *producer* of the new baseline, not a target of code changes.

---

### `test/bench/baselines/v1.9-github-hosted.txt` (data, batch)

**Analog:** `test/bench/baselines/v1.1-github-hosted.txt` (existing baseline, same producer workflow, same flags per Pitfall 1 parity verification in RESEARCH.md §Common Pitfalls).

**Pattern:** A `go test -bench` text output capture. Format is `golang.org/x/perf/benchfmt` lines beginning with `Benchmark...` plus `goos:`, `goarch:`, `pkg:` headers. Verified by `capture-baseline.yml` step 6 ("Verify capture", lines 59-66) which requires ≥5 `^Benchmark` lines.

**Do not hand-author this file.** It is produced by `capture-baseline.yml` and committed by `stefanzweifel/git-auto-commit-action@v5`. Planner action: trigger the workflow, do not write the file content.

---

### `CONTRIBUTING.md` (docs)

**Analog:** self §`## Benchmark CI Gate` (lines 178-184), the existing terse-bullet style of this file.

**Existing section pattern (lines 178-184)** — short, bulleted, links to file paths in backticks:

```markdown
## Benchmark CI Gate

The benchmark CI gate prevents performance regressions from landing:

- `.github/workflows/bench.yml` runs on PRs, comparing PR benchmarks against committed baselines via `benchstat`.
- Uses tiered thresholds: PR tier (>15% time / >25% allocs) and release tier (>10% time / >20% allocs).
- `.github/workflows/capture-baseline.yml` captures new baselines (manual dispatch on GitHub Actions).
```

**Heading-style pattern across the file** — top-level `##` for major topics, `###` for nested subsections (note: file currently has no `###` subsections, but markdown convention allows them). Per D-05 + RESEARCH Open Question #2, recommended placement is a new `### Gopls Pin` nested **under** `## Benchmark CI Gate`.

**New subsection skeleton to insert (per RESEARCH.md §Code Examples, recommended placement after line 184, before `## Legacy Python` at line 186):**

```markdown
### Gopls Pin

The benchmark CI gate pins `gopls` to an explicit version because:

1. **Compatibility:** `v0.17.1` is incompatible with Go 1.25 on `linux/amd64`. The current pin `v0.21.1` resolves this.
2. **Bench stability:** Cold-start and warm-reuse metrics (`BenchmarkLSPIndex_Cold`, `BenchmarkLSPIndex_Warm`) are sensitive to gopls indexing changes (Pitfall 4).

**Where the pin is set (two locations — must stay in sync, Pitfall 1):**
- `.github/workflows/bench.yml` — `GOPLS_VERSION` env
- `.github/workflows/capture-baseline.yml` — `GOPLS_VERSION` env

**Bump policy:** Any change to `GOPLS_VERSION` requires a re-baseline PR per the refresh policy in `test/bench/baselines/README.md`. Do not bump in a PR that does not also re-capture the baseline — the PR will fail bench gate against the old baseline.
```

**Style invariants from existing CONTRIBUTING.md:**
- Backticks around all file paths (`.github/workflows/bench.yml`).
- Backticks around env-var names (`GOPLS_VERSION`) and benchmark identifiers (`BenchmarkLSPIndex_Cold`).
- Numbered lists for ordered rationale; bullet lists for unordered enumerations.
- No emoji, no horizontal rules between subsections within a section.

---

### `.planning/PROJECT.md` (docs)

**Analog:** self — paragraph at line 139 starting `**Known tech debt:**`.

**Existing tech-debt paragraph pattern (line 139)** — single paragraph, sentences separated by periods, items independently scoped:

```
**Known tech debt:** Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest due to gopls v0.17.1 incompatibility with Go 1.25 on linux/amd64. 3 redundant GrammarRegistry instances (functionally correct). rust-analyzer v1.90 `textDocument/rename` returns "No references found at position" in temp workspaces despite hover/references/search working at the same position — upstream LS bug, all other Rust operations work (see USAGE.md Troubleshooting). jdtls cold-start indexing exceeds 2min in temp workspaces — Java integration tests gated behind `-short=false`.
```

**Edit pattern (per D-13):** Delete the first sentence ("Benchmark baselines captured locally...incompatibility with Go 1.25 on linux/amd64."). Keep the bold lead-in `**Known tech debt:**` and the remaining three sentences (GrammarRegistry, rust-analyzer rename, jdtls cold-start). Discretionary light flow cleanup of the joining sentence is acceptable.

**Resulting pattern (after edit, target shape):**

```
**Known tech debt:** 3 redundant GrammarRegistry instances (functionally correct). rust-analyzer v1.90 `textDocument/rename` returns "No references found at position" in temp workspaces despite hover/references/search working at the same position — upstream LS bug, all other Rust operations work (see USAGE.md Troubleshooting). jdtls cold-start indexing exceeds 2min in temp workspaces — Java integration tests gated behind `-short=false`.
```

**Style invariants:** Bold lead-in label, period-separated independent clauses, file/symbol references in backticks, em-dash for clarifying parentheticals.

---

## Shared Patterns

### Parameter Parity Between bench.yml and capture-baseline.yml (Pitfall 1)

**Source:** `.github/workflows/bench.yml:33-41` and `.github/workflows/capture-baseline.yml:29-33`.

**Apply to:** Any future edit to either workflow. The CONTRIBUTING.md `### Gopls Pin` subsection MUST cite this invariant (per D-06).

**Pattern (env-block parity):**

```yaml
# bench.yml (lines 33-41)
    env:
      GOMAXPROCS: "4"
      GOPLS_VERSION: v0.21.1
```

```yaml
# capture-baseline.yml (lines 29-32)
    env:
      # Must match bench.yml exactly (Pitfall 1).
      GOMAXPROCS: "4"
      GOPLS_VERSION: v0.21.1
```

**Pattern (bench-flag parity):** Both files use the identical `go test` invocation:

```
go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...
```

If editing either file, the planner must diff both env blocks and both `go test` invocations against each other.

### Pinned Action Versions

**Source:** all three Go workflows.

**Apply to:** any new step added to either bench workflow.

```yaml
- uses: actions/checkout@v4
- uses: actions/setup-go@v5
  with:
    go-version: '1.25.x'
    cache: true
- uses: actions/upload-artifact@v4
- uses: stefanzweifel/git-auto-commit-action@v5
```

No `@latest`, no floating `@v5.x`. Phase 50 introduces no new actions, but if any step is added, follow this pattern.

### Pitfall-Annotated Comments

**Source:** `.github/workflows/bench.yml:8-15`, `.github/workflows/bench.yml:34-40`, `.github/workflows/capture-baseline.yml:6, 30`.

**Apply to:** any comment added during this phase that touches gopls / GOMAXPROCS / bench flags. Reference the Pitfall number from `test/bench/baselines/README.md` and `bench.yml` header.

```yaml
# Pitfall 4 mitigation: pin gopls to an explicit version so
# cold-start/warm-reuse metrics are not tainted by upstream drift.
# Bump deliberately in a re-baseline PR per
# test/bench/baselines/README.md refresh policy.
GOPLS_VERSION: v0.21.1
```

The CONTRIBUTING.md `### Gopls Pin` subsection mirrors this annotation style in prose form (per D-05's reference to Pitfall 1 and Pitfall 4).

## No Analog Found

None. Every modified file has either an in-file analog (existing surrounding lines) or a sibling analog (existing v1.1 baseline file). No new file class is introduced this phase.

## Metadata

**Analog search scope:**
- `.github/workflows/` (9 files listed; only `bench.yml`, `capture-baseline.yml`, `go-test.yml` in scope per D-12)
- `CONTRIBUTING.md` (188 lines, fully read for heading/style patterns)
- `.planning/PROJECT.md` (line 139 area read for tech-debt paragraph pattern)
- `test/bench/baselines/` (existing baseline file naming + producer wiring)

**Files scanned:** 5 read in full or in targeted ranges (bench.yml, capture-baseline.yml, CONTRIBUTING.md §170-188, PROJECT.md §135-145, plus directory listing of `.github/workflows/`).

**Pattern extraction date:** 2026-04-25
