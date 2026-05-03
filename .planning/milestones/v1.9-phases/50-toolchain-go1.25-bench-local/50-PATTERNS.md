# Phase 50: toolchain-go1.25-bench-local - Pattern Map

**Mapped:** 2026-04-28
**Files analyzed:** 6 created/modified, 7 deleted
**Analogs found:** 6 / 6 (100%)

This phase has no new Go source files; only Make targets, doc rewrites, and a `.gitignore` addition. All analogs are in-tree.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `Makefile` (new targets `bench`, `bench-baseline`) | build-tooling | command-execution | `Makefile:bench-jdtls-warm` (lines 37-42) + `.PHONY` (line 1) | exact (same file, same target style) |
| `test/bench/baselines/README.md` (rewrite) | docs | reference | `test/bench/baselines/README.md` (current, 134 lines) — used as "before" excerpt | self (full rewrite) |
| `CONTRIBUTING.md` — gopls subsection (new) | docs | reference | `CONTRIBUTING.md` "Adding Language Support" (lines 172-176) — short tooling subsection style | structural-match (same file's section idiom) |
| `CONTRIBUTING.md` — "Running Benchmarks" rewrite | docs | reference | `CONTRIBUTING.md` "Running Integration Tests" (lines 80-101) — same fenced-`sh` + "Key details:" idiom | structural-match (sibling section) |
| `CONTRIBUTING.md` — "Benchmark CI Gate" delete | docs | reference | `CONTRIBUTING.md` lines 178-184 (current, to be removed) | self (deletion) |
| `USAGE.md` line 511-529 — gopls troubleshooting rewrite | docs | reference | `USAGE.md` lines 511-529 (current, to be replaced) | self (in-place edit) |
| `.planning/PROJECT.md` line 139 — surgical removal of two sentences | docs | reference | `.planning/PROJECT.md` line 139 (current paragraph) | self (in-place edit) |
| `.gitignore` (add `test/bench/baselines/local.txt`) | repo-config | ignore-pattern | `.gitignore` lines 274-279 (`# Go` section) | role-match (same file, sensible group placement) |

## Pattern Assignments

### `Makefile` — new `bench` and `bench-baseline` targets

**Analog:** `Makefile` itself — `bench-jdtls-warm` target is the canonical self-doc style.

**`.PHONY` pattern** (line 1 verbatim):
```make
.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm
```
**Edit instruction:** Append `bench bench-baseline` to this list, e.g.:
```make
.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench bench-baseline
```

**Self-doc target pattern** (lines 26-27, 29-35, 37-42 verbatim):
```make
docs: ## Regenerate tool and language tables in README.md
	$(GO) run ./cmd/docgen

clean-jdtls-cache: ## Wipe warm jdtls workspaces under the platform user cache dir
	@case "$$(uname -s)" in \
	  Darwin) DIR="$$HOME/Library/Caches/serena-test/jdtls" ;; \
	  *)      DIR="$${XDG_CACHE_HOME:-$$HOME/.cache}/serena-test/jdtls" ;; \
	esac; \
	rm -rf "$$DIR"; \
	echo "cleared jdtls warm cache at $$DIR"

bench-jdtls-warm: ## Run Java integration suite cold then warm; print both wall-clocks
	@$(MAKE) clean-jdtls-cache
	@echo "=== jdtls COLD run ==="
	-@time $(GO) test -run 'Java' ./test/integration/... -count=1
	@echo "=== jdtls WARM run ==="
	-@time $(GO) test -run 'Java' ./test/integration/... -count=1
```

**Concrete patterns to copy:**
1. `target: ## help-text` form — the `## ` after the colon is the inline help annotation (used by `docs`, `clean-jdtls-cache`, `bench-jdtls-warm`).
2. Use `$(GO)` (defined at line 4 as `GO=go`), never bare `go`, for consistency with existing targets.
3. Escape `$` as `$$` in shell expansion — the deleted `bench.yml` used `-run=^$`, so the Makefile body must be `-run=^$$`.
4. New target body invokes the same command the deleted `bench.yml` used (per RESEARCH §Standard Stack and CONTEXT D-07):
   ```make
   $(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...
   ```
5. For `bench-baseline`, append a redirect or `tee` to the canonical gitignored path. RESEARCH §"Code Examples" supplies a `tee`-form skeleton:
   ```make
   bench-baseline: ## Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)
   	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee test/bench/baselines/local.txt
   ```
   Or the redirect form:
   ```make
   bench-baseline: ## Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)
   	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... > test/bench/baselines/local.txt
   	@echo "captured baseline at test/bench/baselines/local.txt"
   ```
   Planner picks one form; both match the existing Makefile idiom.

**Anti-patterns** (per CONTEXT and RESEARCH):
- Do NOT add a third "compare" target wrapping `benchstat` — that re-creates the deleted `benchgate`.
- Do NOT auto-derive a uname suffix on the baseline filename (D-08).

---

### `test/bench/baselines/README.md` — full rewrite

**Analog:** the file's own current content (the "before" excerpt below). The rewrite is a wholesale replacement — there is no other in-tree README that closely matches the new shape (a one-screen "what this dir is for + how to capture locally" doc).

**"Before" excerpt** (current `test/bench/baselines/README.md` lines 1-17 — the framing the rewrite must reverse):
```markdown
# `test/bench/baselines/` — committed v1.1 reference baselines

This directory holds the **immutable performance reference** against which
every v1.2+ phase publishes its benchstat / benchgate delta. Per
`.planning/phases/09-benchmark-harness-v1-1-baseline/09-CONTEXT.md` D-01,
the goal is a tiered regression gate that prevents silent drift during
the v1.2 Performance & Production Hardening milestone.

## Files

| File | Purpose |
|------|---------|
| `v1.1-github-hosted.txt` | PR-tier baseline captured on GitHub-hosted `ubuntu-latest`. Consumed by `.github/workflows/bench.yml` on every PR via `benchgate`. |
```

**Required rewrite contents** (per CONTEXT D-05 and RESEARCH §Test Map "TOOL-02 README" row — must contain "local" + "gitignored" tokens; must NOT contain "benchgate"):
- Single-screen explanation of the directory purpose under the local-only flow.
- How to capture: `make bench-baseline`.
- Note that the captured `local.txt` is gitignored and personal (not committed).
- Optional: pointer to `benchstat` install one-liner from CONTRIBUTING.md (do not duplicate the install instructions; cross-link).

**Style reference for terse in-tree READMEs:** none in `internal/` matches the doc-only README shape needed here (most `internal/` packages have no README). The file's own structural skeleton (heading + short prose + fenced commands) is the right shape — just inverted from "committed CI baselines" to "personal local baselines."

---

### `CONTRIBUTING.md` — gopls compatibility subsection (new)

**Analog:** `CONTRIBUTING.md` "Adding Language Support" (lines 172-176) — the existing short tooling subsection style.

**Existing pattern excerpt** (lines 172-176 verbatim):
```markdown
## Adding Language Support

- Language definitions live in `internal/langregistry/languages.yaml`.
- See the memory guide: [`.serena/memories/adding_new_language_support_guide.md`](.serena/memories/adding_new_language_support_guide.md).
- After adding a language, run `make docs` to regenerate the README language table.
```

**Patterns to mirror:**
- `## Section Heading` (h2) for the subsection.
- Short bullet list, no preamble paragraph.
- Cross-references to in-repo files use backticks + relative path; external paths use Markdown link syntax.

**Required content** (per CONTEXT D-01 + RESEARCH §Code Examples — phrasing must include `gopls` and `v0.21` tokens per the Test Map):
- gopls is installed at runtime by `internal/langregistry`, not pinned in code.
- Project tests against `gopls@latest`.
- gopls v0.17.1 had a Go 1.25 linux/amd64 incompatibility resolved upstream in gopls v0.21 and later. State `>=v0.21` as a floor, not as a pin (per RESEARCH Pitfall 4).
- `go install golang.org/x/tools/gopls@latest` to update.

---

### `CONTRIBUTING.md` — "Running Benchmarks" section rewrite

**Analog:** `CONTRIBUTING.md` "Running Integration Tests" (lines 80-101) — the sibling section that establishes the prose + fenced-`sh` + "Key details:" idiom.

**Existing pattern excerpt** (lines 80-101 verbatim — the structural template to mirror):
```markdown
## Running Integration Tests

The integration test harness lives in `test/integration/`. It starts a real Serena daemon with a Go fixture project and exercises MCP round-trips via stdio transport against live language servers.

Run all integration tests:

```sh
go test ./test/integration/ -v -timeout 120s
```

Run a specific test:

```sh
go test ./test/integration/ -run TestSymbolRetrieval -v
```

Key details:

- The harness uses the `testing.TB` interface, shared by both integration tests and benchmarks.
- Tests exercise MCP round-trips against live language servers.
- Go fixtures are in `test/integration/`, with additional language fixtures available for Python, TypeScript, Java, and Rust.
- **Requirement:** Integration tests require `gopls` installed. Tests for other languages require their respective language servers.
```

**Patterns to mirror:**
- `## Section Heading` → one-paragraph what-and-where → fenced `sh` blocks for primary commands → "Key details:" bullet list.
- Use `make bench` / `make bench-baseline` as the primary commands (not raw `go test` invocations) — driven by D-07 making Make the documented surface.
- Bullet list mentions: testing.B.Loop (Go 1.24+), `test/bench/baselines/` is the gitignored capture location, `benchstat` install one-liner for optional comparisons, bench-impact PR responsibility note (D-06).

**Current "Running Benchmarks" content to delete** (lines 130-152, must be fully replaced):
```markdown
## Running Benchmarks

The benchmark suite lives in `test/bench/`. It measures tool response times, LSP indexing throughput, and memory profiles.

Run all benchmarks:

```sh
go test -bench=. ./test/bench/ -timeout 300s
```
... (lines 130-152 entire section)
```

**Current "Benchmark CI Gate" section to fully delete** (lines 178-184):
```markdown
## Benchmark CI Gate

The benchmark CI gate prevents performance regressions from landing:

- `.github/workflows/bench.yml` runs on PRs, comparing PR benchmarks against committed baselines via `benchstat`.
- Uses tiered thresholds: PR tier (>15% time / >25% allocs) and release tier (>10% time / >20% allocs).
- `.github/workflows/capture-baseline.yml` captures new baselines (manual dispatch on GitHub Actions).
```
This section must be removed entirely (no replacement — the local-only flow is documented in "Running Benchmarks" and the gopls subsection above).

---

### `USAGE.md` — gopls troubleshooting rewrite (lines 511-529)

**Analog:** the section's own current content (in-place rewrite). Sibling troubleshooting sections in USAGE.md (e.g., the jdtls section at lines 501-509 just above) supply the structural template.

**Existing pattern excerpt — sibling jdtls Troubleshooting section above** (lines 501-509):
```markdown
1. Increase the indexing timeout for Java projects in `.serena/project.yml`:
   ```yaml
   degradation:
     timeout_index: 300  # 5 minutes for jdtls cold-start
   ```

2. Wait for indexing to complete before issuing symbol queries. Serena creates a workspace-specific data directory (`.jdtls-data`) to avoid cross-workspace conflicts.

3. Subsequent sessions reuse the cached index, so cold-start delay is only on first activation per workspace.
```

**Section to rewrite — current lines 511-529 verbatim**:
```markdown
### gopls version incompatibility with Go 1.25

**Symptom:** Build failures or unexpected behavior when running Serena's benchmark suite (`test/bench/`) or when gopls returns errors after a Go version upgrade.

**Cause:** gopls v0.17.1 has a known incompatibility with Go 1.25 on linux/amd64. Additionally, the benchmark suite uses `testing.B.Loop` which requires Go 1.24 or later.

**Fix:**

1. Ensure your gopls version is compatible with your Go version. After upgrading Go, update gopls:
   ```bash
   go install golang.org/x/tools/gopls@latest
   ```

2. For benchmarks, verify you are running Go 1.24 or later:
   ```bash
   go version  # Must be 1.24+
   ```

3. After Go version changes, re-baseline benchmarks using the `capture-baseline.yml` CI workflow to avoid false regression alerts from benchstat comparisons.
```

**Patterns to preserve in the rewrite:**
- `### Heading` (h3) — matches the surrounding troubleshooting subsection level.
- `**Symptom:** ... **Cause:** ... **Fix:**` triad — established structure across this Troubleshooting section.
- Numbered fix steps with fenced `bash` code blocks.

**Required content changes** (per CONTEXT D-01 + RESEARCH §Code Examples):
- Acknowledge the bug as resolved upstream rather than as an active issue.
- State the fix shipped in gopls v0.21+; floor, not pin.
- DELETE step 3 (the `capture-baseline.yml` reference) — that workflow is gone (D-02). RESEARCH Test Map asserts `! grep -q 'capture-baseline' USAGE.md` post-edit.
- Note: `internal/langregistry` invokes whatever `gopls` is on `$PATH`.

A concrete RESEARCH-supplied skeleton (planner finalizes prose):
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

---

### `.planning/PROJECT.md` line 139 — surgical edit

**Analog:** the line itself (in-place edit). The "Known tech debt:" paragraph is a single dense line of comma-separated sentences.

**Current line 139 verbatim**:
```markdown
**Known tech debt:** Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest due to gopls v0.17.1 incompatibility with Go 1.25 on linux/amd64. 3 redundant GrammarRegistry instances (functionally correct). rust-analyzer v1.90 `textDocument/rename` returns "No references found at position" in temp workspaces despite hover/references/search working at the same position — upstream LS bug, all other Rust operations work (see USAGE.md Troubleshooting). jdtls cold-start indexing exceeds 2min in temp workspaces — Java integration tests gated behind `-short=false`.
```

**Surgical edit instructions** (per CONTEXT D-10, D-11 and RESEARCH Pitfall 5):
1. **REMOVE (D-10):** `Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest due to gopls v0.17.1 incompatibility with Go 1.25 on linux/amd64.`
2. **REMOVE (D-11):** `3 redundant GrammarRegistry instances (functionally correct).`
3. **PRESERVE verbatim:**
   - `rust-analyzer v1.90 \`textDocument/rename\` returns "No references found at position" in temp workspaces despite hover/references/search working at the same position — upstream LS bug, all other Rust operations work (see USAGE.md Troubleshooting).`
   - `jdtls cold-start indexing exceeds 2min in temp workspaces — Java integration tests gated behind \`-short=false\`.`

**Resulting line 139 shape:**
```markdown
**Known tech debt:** rust-analyzer v1.90 `textDocument/rename` returns "No references found at position" in temp workspaces despite hover/references/search working at the same position — upstream LS bug, all other Rust operations work (see USAGE.md Troubleshooting). jdtls cold-start indexing exceeds 2min in temp workspaces — Java integration tests gated behind `-short=false`.
```

**Verification per RESEARCH Test Map:**
- `! grep -q 'GrammarRegistry instances' .planning/PROJECT.md`
- `! grep -q 'CI ubuntu-latest' .planning/PROJECT.md`
- `grep -q 'rust-analyzer' .planning/PROJECT.md`
- `grep -q 'jdtls cold-start' .planning/PROJECT.md`

---

### `.gitignore` — add canonical local-baseline path

**Analog:** `.gitignore` lines 274-279 (the `# Go` section) — the existing repo-root grouping pattern.

**Existing pattern excerpt** (lines 274-279 verbatim):
```
# Go
/serena
*.exe
/bin/
/api/proto/**/*.pb.go
```

**Patterns to mirror:**
- `# <Group>` comment header introduces a group.
- Each entry is a single line, no quotes, paths relative to repo root.
- `.gitignore` already uses both leading-`/`-rooted patterns (`/serena`, `/bin/`) and unrooted globs (`*.exe`). For a single specific file, a relative path without leading `/` is sufficient and matches at any depth, but a precise path is clearer here.

**Recommended addition** (per CONTEXT D-09 and RESEARCH Pitfall 3):
- Use the exact filename, NOT a directory prefix — ignoring `test/bench/baselines/` would also hide the rewritten `README.md`.
- Place the entry under the existing `# Go` group at the bottom (it's a Go-toolchain-produced artifact), or introduce a new `# Benchmarks` group right after.

Example placement under existing `# Go` group:
```
# Go
/serena
*.exe
/bin/
/api/proto/**/*.pb.go
test/bench/baselines/local.txt
```

Or as a dedicated group (slightly more discoverable):
```
# Benchmarks (local-only — see test/bench/baselines/README.md)
test/bench/baselines/local.txt
```

**Verification per RESEARCH Test Map:** `git check-ignore test/bench/baselines/local.txt` must exit 0.

## Shared Patterns

### Doc-edit minimality
**Source:** project convention reflected in CONTEXT D-10/D-11 and PROJECT.md line 139 idiom.
**Apply to:** All doc edits in this phase.
- Edit only what the decision requires; preserve surrounding sentences/sections verbatim.
- Avoid drift in tone or unrelated reformatting.

### Self-doc Make targets
**Source:** `Makefile` lines 26, 29, 37 (`docs`, `clean-jdtls-cache`, `bench-jdtls-warm`).
**Apply to:** Both new Make targets.
- `target: ## description` form is mandatory; without it the target is invisible to any future help-printing pass.

### Floor versions, not pins, in docs
**Source:** RESEARCH Pitfall 4 + CONTEXT D-01.
**Apply to:** CONTRIBUTING.md gopls subsection and USAGE.md troubleshooting rewrite.
- Phrase upstream version constraints as ">=" floors with "or later" qualifier; never hard-pin a version that will age out.

### Verification grep matrix
**Source:** RESEARCH §Validation Architecture "Phase Requirements → Test Map".
**Apply to:** Every plan completion check.
- Each doc edit has a corresponding grep assertion (positive: token must exist; negative: token must NOT exist). Plans should embed the relevant grep(s) in their per-task verification.

## Deletions (no analog)

These items are pure removals — no analog needed because no replacement file is created.

| Path | Type | Decision Source | Notes |
|------|------|-----------------|-------|
| `.github/workflows/bench.yml` | CI workflow | D-02 | Verify no other workflow has `needs: bench` (RESEARCH A2 confirms none). |
| `.github/workflows/capture-baseline.yml` | CI workflow | D-02 | No external service reads its outputs. |
| `test/bench/cmd/benchgate/` (entire directory) | Go package + tests | D-03 | `package main`; never compiled into a persistent binary in the repo. |
| `test/bench/baselines/v1.1-github-hosted.txt` | Captured baseline | D-04 | Belongs to deleted CI flow. |
| `test/bench/baselines/v1.2-phase10-github-hosted.txt` | Captured baseline | D-04 | Same. |
| `test/bench/baselines/v1.2-phase11-github-hosted.txt` | Captured baseline | D-04 | Same. |
| `test/bench/baselines/v1.2-phase12-github-hosted.txt` | Captured baseline | D-04 | Same. |

**Deletion verification (per RESEARCH Test Map):**
- `! ls .github/workflows/bench.yml .github/workflows/capture-baseline.yml 2>/dev/null`
- `! ls test/bench/baselines/v1.*-github-hosted.txt 2>/dev/null`
- `! grep -rn benchgate --include='*.go' --include='*.md' --include='*.yml' --include='Makefile' . 2>/dev/null | grep -v '\.planning/'`

## No Analog Found

None. Every new/edited file has a direct in-tree analog (often the file itself for in-place edits, or the same Makefile / same CONTRIBUTING.md sibling section for new content).

## Metadata

**Analog search scope:** `Makefile`, `.gitignore`, `CONTRIBUTING.md`, `USAGE.md`, `.planning/PROJECT.md`, `test/bench/baselines/README.md` (all in-place edits or single-file additions; no cross-package search needed for this doc-and-config phase).
**Files scanned:** 6
**Pattern extraction date:** 2026-04-28
