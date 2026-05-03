# Phase 51: packaging-goreleaser - Pattern Map

**Mapped:** 2026-04-28
**Files analyzed:** 7 (3 new, 3 modified, 1 deleted)
**Analogs found:** 4 / 7 (3 files have NO direct analog — see "No Analog Found" section)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `.goreleaser.yaml` (NEW) | config (release pipeline) | batch (build → archive → sign → publish) | none in repo | **no analog** — see RESEARCH.md Patterns 1–5 |
| `.github/workflows/release.yml` (NEW) | CI workflow (tag-triggered) | event-driven (`push: tags: ['v*']`) | `.github/workflows/go-test.yml` | role-match (CI shape, checkout/setup-go reuse) |
| `minisign.pub` (NEW) | static config (cryptographic key) | request-response (user fetches via raw URL) | none in repo | **no analog** — file format dictated by minisign upstream |
| `INSTALL.md` (MODIFY) | documentation (user-facing install guide) | request-response (reader → terminal commands) | `INSTALL.md` (existing top section, lines 1–29) | self-analog (restructure of existing content) |
| `Makefile` (MODIFY — add `release-snapshot`) | build automation | request-response (developer invokes target) | `Makefile` lines 44–48 (`bench`, `bench-baseline`) | exact (Phase 50 D-07 self-doc style) |
| `CONTRIBUTING.md` (MODIFY — add "Releasing" subsection) | documentation (maintainer-facing) | request-response (reader → ceremony) | `CONTRIBUTING.md` lines 130–157 ("Running Benchmarks") | role-match (closest H2 ceremony block in current file) |
| `.github/workflows/publish.yml` (DELETE) | legacy Python CI workflow | n/a — being removed | — | n/a |

## Pattern Assignments

### `.github/workflows/release.yml` (NEW — CI workflow)

**Analog:** `.github/workflows/go-test.yml`
**Match:** role-match — same CI shape (`ubuntu-latest`, single job, official actions), same `actions/checkout@v4` and `actions/setup-go@v5` step blocks. The release workflow MUST mirror the Go version pin (`'1.25.x'`) and `cache: true` exactly.

**Workflow header pattern** (`go-test.yml` lines 1–11):
```yaml
name: go-test
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
  workflow_dispatch:

permissions:
  contents: read
```

**For `release.yml`, change to** (per RESEARCH Pattern 5):
- `name: release`
- `on: push: tags: ['v*']` only (NOT pull_request, NOT workflow_dispatch — D-06 locks tag-triggered auto-publish; planner MUST NOT add a `workflow_dispatch:` gate)
- `permissions: contents: write` (release.yml needs write to create the GitHub Release; go-test.yml uses read-only — bump it)

**Job/checkout pattern** (`go-test.yml` lines 12–23):
```yaml
jobs:
  test:
    name: go test (ubuntu-latest)
    runs-on: ubuntu-latest
    timeout-minutes: 20
    env:
      JDTLS_VERSION: "1.57.0"
      SERENA_TEST_LS_TIMEOUT: "2m"
    steps:
      - uses: actions/checkout@v4
```

**For `release.yml`, copy verbatim** but:
- Job name `release` / `name: goreleaser`
- `timeout-minutes: 30` (RESEARCH Pattern 5 — three goreleaser passes need more headroom)
- Drop the `JDTLS_VERSION` / `SERENA_TEST_LS_TIMEOUT` env vars (release pipeline doesn't run Java tests)
- **Critical:** add `with: fetch-depth: 0` to the checkout step — goreleaser's changelog generator needs full git history (RESEARCH Pitfall 1 / Pattern 5)

**setup-go step pattern** (`go-test.yml` lines 25–29) — copy verbatim:
```yaml
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true
```

This block reproduces unchanged in `release.yml`. Same Go version, same `cache: true`. Mirrors `go-test.yml` exactly per RESEARCH Standard Stack ("matches go-test.yml").

**vet/test step pattern** (`go-test.yml` lines 93–97) — DO NOT copy:
```yaml
      - name: go vet
        run: go vet ./...

      - name: go test
        run: go test ./... -count=1
```

`release.yml` does NOT re-run tests (RESEARCH §"Project Constraints" + Open Question 3 — go-test.yml runs on `push: branches: [main]`, release.yml runs on `push: tags`, and the planner MUST NOT add a cross-workflow `needs:`). The release-pipeline-specific steps replace these (install minisign → snapshot pass 1 → diff → real release; full block in RESEARCH Pattern 5).

---

### `Makefile` (MODIFY — add `release-snapshot` target)

**Analog:** `Makefile` lines 44–48 — `bench` and `bench-baseline` targets
**Match:** exact — Phase 50 D-07 self-doc style locks the conventions: single-line `## help` annotation, single fixed-path output, no `NAME=` parameter.

**Verbatim analog excerpt** (`Makefile` lines 44–48):
```makefile
bench: ## Run the bench suite once and print results to stdout
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...

bench-baseline: ## Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee test/bench/baselines/local.txt
```

**Style rules to copy** (extracted from `bench-baseline`):
1. Single `## help` annotation on the target line — no separate help system
2. Annotation explicitly names the **fixed output path** (`test/bench/baselines/local.txt`) and **gitignored** status — `release-snapshot` must do the same for `dist/`
3. Annotation is one short sentence, imperative voice
4. No `NAME=` or other parameters — single fixed-path output
5. The recipe is a single command line (no `@` prefix, no multi-step orchestration)

**`.PHONY` declaration** (`Makefile` line 1):
```makefile
.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench bench-baseline
```

**Required edit:** add `release-snapshot` to this `.PHONY` list (alphabetical placement is not enforced — append at end is fine; matches how `bench` / `bench-baseline` were appended).

**Target shape to write** (per RESEARCH Pattern 6, following `bench-baseline` style verbatim):
```makefile
release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
	goreleaser release --snapshot --clean --skip=sign
```

Annotation matches `bench-baseline` cadence: "writes archives to dist/ (overwrites; gitignored)" mirrors "Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)".

---

### `INSTALL.md` (MODIFY — restructure per D-05)

**Analog:** `INSTALL.md` lines 1–29 (current top section being demoted)
**Match:** self-analog — D-05 restructures the existing content; the planner needs to see what's being moved.

**Verbatim current top section** (`INSTALL.md` lines 1–29) — this is what gets DEMOTED to a new "Build from source" subsection:

```markdown
# Install Guide

Serena is a single Go binary. Install it, point your coding agent at it, and you are ready to go.

## Prerequisites

**Install the binary:**

```bash
go install github.com/postfix/serena/cmd/serena@latest
```

Requires Go 1.25 or later.

Or build from source:

```bash
git clone https://github.com/postfix/serena.git
cd serena
go build ./cmd/serena
```

Verify the binary is in your PATH:

```bash
serena --help
```

If `serena` is not found, ensure `$(go env GOPATH)/bin` is in your PATH.
```

**Restructure plan** (per CONTEXT D-04 + D-05):
1. Keep the H1 `# Install Guide` and the one-line tagline ("Serena is a single Go binary…").
2. **NEW lead H2** (planner picks title — recommend `## Install (pre-built binary)` per RESEARCH Discretion): contains the D-04 verification block.
3. Insert the D-04 verification block — single fenced bash block, copy-paste ready, with placeholders resolved to `agenthands/helix` (RESEARCH Pitfall 2). Final form is in RESEARCH §"Code Examples — Verifying a release on Linux (D-04 verify block — final form)".
4. Add the macOS one-line note immediately after the block: "On macOS use `shasum -a 256 -c` instead of `sha256sum -c`. Install minisign with `brew install minisign`." (RESEARCH §"Code Examples — macOS verification one-line note").
5. **Demote** the existing "Prerequisites" content (lines 5–29 above) to a new H2 `## Build from source` subsection placed AFTER the new pre-built section but BEFORE `## Quick Start`.
6. **Repo identity fix** (RESEARCH Pitfall 2 — MANDATORY): in the demoted "Build from source" block, change line 18 `git clone https://github.com/postfix/serena.git` → `git clone https://github.com/agenthands/helix.git`. Line 10 (`go install github.com/postfix/serena/cmd/serena@latest`) is a separate decision: per RESEARCH Open Question 1, **do NOT silently rename `go.mod` in this phase**. Either delete that `go install` line or leave it as a known-broken reference flagged with a "TODO: pending go.mod rename" comment — planner picks during plan-checking with the user.

**Quick Start / HTTP Mode / Verify Installation / Next Steps / Legacy Python sections** (lines 31–247): UNCHANGED. The restructure is surgical — only the top ~29 lines move.

---

### `CONTRIBUTING.md` (MODIFY — add "Releasing" subsection)

**Analog:** `CONTRIBUTING.md` lines 130–157 ("Running Benchmarks" subsection)
**Match:** role-match — closest existing H2 ceremony block. There is NO existing "Releasing" or "Maintainer" section in `CONTRIBUTING.md` (verified by reading the full file); the planner is creating a new H2.

**Verbatim analog excerpt** (`CONTRIBUTING.md` lines 130–157) — this shows house style for an H2 ceremony block:

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

To compare two captures, install upstream `benchstat` and run it directly:

```sh
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

Key details:

- Benchmarks use `testing.B.Loop` (Go 1.24+) to prevent compiler elision.
- `test/bench/baselines/` is the conventional capture location; everything you save there is gitignored.
- If your changes are likely to affect bench numbers, mention the local before/after deltas in your PR description. (Honor system — there is no PR-time gate.)
```

**Style rules to copy:**
1. Single H2, one-paragraph intro that names the scope and any non-obvious constraint up front (here: "Benchmarks run locally only" + rationale).
2. Numbered/bulleted ceremony steps presented as tagged code fences (` ```sh ` not ` ```bash ` — house preference per file).
3. Inline rationale in italics or normal prose between fences (NOT inside the fence).
4. "Key details:" trailing bulleted list for caveats / cross-references that don't fit the linear flow.
5. Use of `make <target>` references rather than open-coded shell — points readers at the Makefile as single source of truth.
6. No emoji, no "Note:" callouts, no admonition syntax — plain prose only.

**For the new `## Releasing` subsection, mirror this shape**:
- Placement: after `## Running Benchmarks` (line 157) and before `## Adding a New MCP Tool` (line 159) — keeps maintainer ceremony together, before the "how to extend Serena" sections.
- Cover (per CONTEXT decisions): cutting a release (push a `v*` tag — D-06), GitHub Actions secret names (`MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD` — RESEARCH Discretion / D-02a), local dry-run command (`make release-snapshot` — Phase 51 Pattern 6), key rotation procedure (edit `minisign.pub` in a normal commit — D-02), one-time keypair setup (`minisign -G -s minisign.key -p minisign.pub` then upload secret via `gh secret set` — RESEARCH §"Maintainer prerequisite").
- Use ` ```sh ` for shell fences (matches house style — every fence in the analog uses `sh`, NOT `bash`).
- Use the same "Key details:" bulleted trailer for caveats (e.g. "auto-generated release notes from conventional-commit prefixes — see `.goreleaser.yaml` `changelog:` block", "tags MUST come from a green-CI commit — release.yml does not re-run tests").

---

### `.github/workflows/publish.yml` (DELETE)

**Action:** outright deletion. RESEARCH §"Pitfall 5" + CONTEXT canonical_refs both confirm zero cross-references. Verbatim analog content (`.github/workflows/publish.yml`, all 55 lines) is preserved here for the planner's reference — it shows the legacy Python `uv build` → PyPI flow that goreleaser supersedes:

```yaml
name: Publish Python Package

on:
  release:
    types: [created]

  workflow_dispatch:
    inputs:
      tag:
        description: 'Tag name for the release (e.g., v0.1.0)'
        required: true
        default: 'v0.1.0'

env:
  PUBLISH_TO_PYPI: true

jobs:
  publish:
    name: Publish the serena-agent package
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: write
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      - name: Install the latest version of uv
        uses: astral-sh/setup-uv@v6
        with:
          version: "latest"
      - name: Build package
        run: uv build
      - name: Upload artifacts to GitHub Release
        if: env.PUBLISH_TO_PYPI == 'true'
        uses: softprops/action-gh-release@v2
        with:
          tag_name: ${{ github.event.inputs.tag || github.ref_name }}
          files: |
            dist/*.tar.gz
            dist/*.whl
      - name: Publish to TestPyPI
        run: uv publish --index testpypi
      - name: Publish to PyPI (conditional)
        if: env.PUBLISH_TO_PYPI == 'true'
        run: uv publish
```

**Key reason for deletion** (beyond "obsolete"): trigger is `on: release: types: [created]`. When goreleaser-action publishes a release in the new `release.yml`, GitHub fires `release: created`, which would trigger this workflow against a Go-only working tree → `uv build` fails confusingly. RESEARCH Pitfall 5 documents this in detail.

---

## Shared Patterns

### Repo identity (postfix → agenthands)

**Source:** RESEARCH §"Pitfall 2: Repo identity drift" — `git remote` is `agenthands/helix`; `INSTALL.md` and `README.md` still say `postfix/serena`.
**Apply to:** `.goreleaser.yaml` (release.github.owner/name), `.github/workflows/release.yml` (any URL refs), `INSTALL.md` (line 18 `git clone` URL + all D-04 placeholders), `CONTRIBUTING.md` (any URL refs in the new "Releasing" section).
**Concrete substitution:** every `<owner>/<repo>` placeholder and every literal `postfix/serena` becomes `agenthands/helix`.
**Out of scope for Phase 51:** renaming `go.mod` module path — see RESEARCH Open Question 1; planner asks user during plan-checking.

### Go toolchain version pin

**Source:** `.github/workflows/go-test.yml` lines 26–29
**Apply to:** `.github/workflows/release.yml` setup-go step (must match exactly)
```yaml
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true
```
Mirroring this exactly is mandatory per RESEARCH §"Standard Stack" ("matches go-test.yml exactly"). Any drift between `go-test.yml` and `release.yml` Go versions is a smell.

### Makefile self-doc convention

**Source:** `Makefile` lines 44–48 (`bench`, `bench-baseline`)
**Apply to:** new `release-snapshot` target
**Rules:** single-line `## help` annotation; annotation names fixed output path AND gitignored status when applicable; no `NAME=`/parameter args; single command line; target name added to `.PHONY` line 1.

### `dist/` is already gitignored

**Source:** `.gitignore` line 77 — `dist/` (under "Distribution / packaging" header, lines 73–92)
**Apply to:** `.goreleaser.yaml` output, `make release-snapshot` output, CI workflow snapshot artifacts
**Status:** **NO ACTION REQUIRED** — RESEARCH Pitfall 4 confirms this is already handled. The planner does NOT need to edit `.gitignore` in this phase. (Note: `dist/` in `.gitignore` is shared with the legacy Python `dist/` convention — same line covers both ecosystems by coincidence.)

### Documentation house style

**Source:** `CONTRIBUTING.md` lines 130–157 ("Running Benchmarks")
**Apply to:** new `## Releasing` subsection in `CONTRIBUTING.md`; structure of the new lead H2 in `INSTALL.md`
**Rules:** ` ```sh ` fences (not bash); plain prose, no emoji/admonitions; "Key details:" trailing bullet list; reference Makefile targets rather than open-coded shell; one-paragraph intro that calls out non-obvious constraints upfront.

---

## No Analog Found

These files have no direct codebase analog. The planner uses RESEARCH.md's quoted upstream snippets as the source pattern:

| File | Role | Data Flow | Pattern Source |
|------|------|-----------|----------------|
| `.goreleaser.yaml` | release-pipeline config | batch | RESEARCH Pattern 1 (`builds:` + reproducibility flags), Pattern 2 (`archives:` + `checksum:` override), Pattern 3 (`signs:` minisign), Pattern 4 (`changelog:` conventional-commits), Pattern 5 (`release:` block) — all six patterns are quoted in full and cite goreleaser docs as VERIFIED 2026-04-28. The planner concatenates Patterns 1–5 to produce the file. |
| `minisign.pub` | static cryptographic key | request-response | Format dictated by minisign upstream — file is the literal output of `minisign -G -s minisign.key -p minisign.pub`. Two-line ASCII content (untrusted comment + base64 key). RESEARCH §"Maintainer prerequisite" documents the generation command. No project analog because no other crypto-key file lives in this repo. |
| `.github/workflows/release.yml` (full body) | tag-triggered CI workflow | event-driven | Header/checkout/setup-go pattern is from `go-test.yml` (covered above). The release-specific body — install minisign, snapshot pass 1, capture sha256s, snapshot pass 2, diff gate, real release pass — has NO codebase analog and comes from RESEARCH Pattern 5 (full ~80-line YAML block, quoted verbatim with all `--skip=sign`, `mv dist dist-pass-1`, `diff -u` mechanics). |

---

## Metadata

**Analog search scope:**
- `.github/workflows/` — all 7 workflow files surveyed; only `go-test.yml` is a structural analog
- `Makefile` — single file, full read
- `INSTALL.md` — full read for "what's being demoted" content
- `CONTRIBUTING.md` — full read; confirmed no existing "Releasing" or "Maintainer" H2 exists
- `.gitignore` — full read; line 77 `dist/` confirmed
- `.github/workflows/publish.yml` — full read; preserved verbatim above for reference before deletion

**Files scanned:** 6 analog candidates read in full (no partial reads — all files ≤ 250 lines)

**Pattern extraction date:** 2026-04-28

**Cross-reference quality:** The RESEARCH document is unusually prescriptive (CONTEXT decisions are all locked, alternatives rejected) — for the three files with no codebase analog, the planner can write them prescriptively from RESEARCH alone. For the four files with analogs, the analogs are tight matches (especially `bench-baseline` → `release-snapshot` and `go-test.yml` → `release.yml` checkout/setup-go shape), so the planner copies blocks verbatim with the documented substitutions.
