---
phase: 48
plan: 05
subsystem: ci-docs
tags: [ci, github-actions, docs, jdtls, warm-cache]
requires: [48-03, 48-04]
provides:
  - "CI workflow `.github/workflows/go-test.yml` running go vet + go test on ubuntu-latest with warm jdtls cache"
  - "USAGE.md `## Development` section documenting warm-cache workflow, env-var contract, concurrent-run caveat"
affects:
  - .github/workflows/go-test.yml
  - USAGE.md
tech-stack:
  added:
    - "GitHub Actions: actions/setup-go@v5, actions/setup-java@v4, actions/cache@v3"
    - "pipx-installed jdtls 1.57.0 (pinned via JDTLS_VERSION env)"
  patterns:
    - "Cache key combines runner.os + hashFiles('testdata/fixtures/java/**') + JDTLS_VERSION; restore-keys fallback prefix"
    - "Job-scoped `permissions: contents: read` (matches bench.yml)"
    - "Test-only env-var documentation pattern (explicit production-warning sentence)"
key-files:
  created:
    - .github/workflows/go-test.yml
  modified:
    - USAGE.md
decisions:
  - "Used pipx install jdtls (matches plan template) — no deviation needed"
  - "Inserted USAGE.md content as a new top-level `## Development` section at end-of-file (no existing Development/Testing top-level section); subsection heading: `### Java integration tests (warm jdtls cache)`"
  - "CI workflow not yet triggered (commit pushed to main worktree, not yet to remote in this execution)"
  - "Pre-existing Java test failures (3 sub-cases) are documented in 48-03-SUMMARY.md as out-of-scope fixture/jdtls-indexing issues; Plan 05 modifies no Go code, so these are unaffected"
metrics:
  tasks_completed: 2
  files_created: 1
  files_modified: 1
  completed_date: "2026-04-25"
---

# Phase 48 Plan 05: CI Workflow + USAGE.md Warm-Cache Docs Summary

Closing wave of Phase 48: ships `.github/workflows/go-test.yml` (Go 1.25 + Temurin 17 + pinned jdtls 1.57.0 + actions/cache@v3 keyed on Java fixture hash + jdtls version) and a new `## Development` section in USAGE.md covering cache layout, Makefile commands (from Plan 04), the `SERENA_TEST_JDTLS_DATA_DIR` test-only env-var contract (with explicit production warning), and the deferred concurrent-run caveat (Pitfall 4).

## Objective Achieved

- [x] `.github/workflows/go-test.yml` exists, runs on ubuntu-latest with Go 1.25.x + Temurin 17, invokes `go vet ./...` + `go test ./... -count=1`
- [x] Workflow uses `actions/cache@v3` with `path: ~/.cache/serena-test/jdtls` and key encoding fixture tree hash + pinned jdtls version
- [x] jdtls installed via `pipx install "jdtls==${JDTLS_VERSION}"` with version pinned to `1.57.0`
- [x] USAGE.md documents `make clean-jdtls-cache`, `make bench-jdtls-warm`, and `SERENA_TEST_JDTLS_DATA_DIR`
- [x] USAGE.md documents warm-cache directory layout (Linux/macOS/Windows) and concurrent-run caveat (Pitfall 4 deferred)
- [x] USAGE.md contains explicit "Do not set this variable in production" warning (T-48-05-07 mitigation)

## Tasks Completed

| Task | Name                                                       | Commit   | Files                            |
| ---- | ---------------------------------------------------------- | -------- | -------------------------------- |
| 1    | Create go-test.yml workflow with warm jdtls cache          | 350a480d | .github/workflows/go-test.yml    |
| 2    | Document warm cache operations in USAGE.md                 | bee06b20 | USAGE.md                         |

## Verification

### Workflow file (Task 1)

```
$ go run gopkg.in/yaml.v3 ... → YAML OK (parsed via custom yaml.v3 program — python3 lacks PyYAML on this host)
$ grep -c '^name: go-test' .github/workflows/go-test.yml          # 1
$ grep -c 'runs-on: ubuntu-latest' .github/workflows/go-test.yml  # 1
$ grep -c "go-version: '1.25.x'" .github/workflows/go-test.yml    # 1
$ grep -c 'actions/setup-java@v4' .github/workflows/go-test.yml   # 1
$ grep -c 'actions/cache@v3' .github/workflows/go-test.yml        # 1
$ grep -c 'path: ~/.cache/serena-test/jdtls' .github/workflows/go-test.yml  # 1
$ grep -c "hashFiles('testdata/fixtures/java/\*\*')" .github/workflows/go-test.yml  # 1
$ grep -c 'JDTLS_VERSION' .github/workflows/go-test.yml           # 3 (env decl + key interp + install line)
$ grep -c 'go test' .github/workflows/go-test.yml                 # 3 (heading + name + run line)
$ grep -c 'go vet' .github/workflows/go-test.yml                  # 3
$ grep -Pn '^\t' .github/workflows/go-test.yml                    # (no output — no tabs)
$ grep -l '^name: go-test' .github/workflows/*.yml                # only the new file
```

YAML validation: parsed cleanly via `gopkg.in/yaml.v3` (the host's `python3` lacks PyYAML; equivalent validator used).

### USAGE.md (Task 2)

```
$ grep -c 'warm jdtls cache' USAGE.md                       # 1 (subsection heading)
$ grep -c 'make clean-jdtls-cache' USAGE.md                 # 2
$ grep -c 'make bench-jdtls-warm' USAGE.md                  # 1
$ grep -c 'SERENA_TEST_JDTLS_DATA_DIR' USAGE.md             # 1
$ grep -c 'Do not set this variable in production' USAGE.md # 1 (T-48-05-07)
$ grep -c 'concurrent' USAGE.md                             # 5 (new content + pre-existing 'consecutively' stems)
$ grep -c 'serena-test/jdtls' USAGE.md                      # 3 (Linux/macOS/Windows lines)
$ wc -l USAGE.md                                            # 877 (was 835; +42 lines)
```

USAGE.md section title: `### Java integration tests (warm jdtls cache)` under a new top-level `## Development` heading. Inserted at file end (line 838 onward), after `### Restart Budget`.

### Build & vet

```
$ go vet ./...     # clean (only pre-existing swift C-binding macro warning)
$ go build ./...   # exit 0 (same pre-existing swift warning)
```

### Tests

`go test ./... -count=1 -short` — all packages pass except the pre-existing Java fixture failures already documented in 48-03-SUMMARY.md ("Known Issues — Out of Scope") as content/jdtls-indexing limitations: `find_references_cross_file`, `replace_symbol_body`, `get_hover_info`, `rename`. Plan 05 modifies **no Go code** (only YAML + Markdown), so these failures are not introduced by this plan and are explicitly out-of-scope per CONTEXT (deferred to BUG-DEFER-01).

## CI Wall-Clock (D-14)

CI was **not** triggered as part of this plan (the workflow file is committed to a worktree branch; merge to `main` will produce the first run). Plan-level acceptance criteria explicitly note the workflow is "valid as a spec" even before a green CI run, since Phase 50 owns the holistic Go 1.25 / gopls CI work. Local cold/warm wall-clocks are recorded in 48-03-SUMMARY.md (~15s cold, ~3.3s warm sub-case).

To capture CI numbers post-merge: open the first `go-test` run on the `main` branch, record total job time with cache miss, then re-run a subsequent commit and record cache-hit time.

## Acceptance Criteria

### Task 1 (workflow)

- [x] `.github/workflows/go-test.yml` exists
- [x] File parses as valid YAML
- [x] `grep -c '^name: go-test'` == 1
- [x] `grep -c 'runs-on: ubuntu-latest'` == 1
- [x] `grep -c "go-version: '1.25.x'"` == 1
- [x] `grep -c 'actions/setup-java@v4'` == 1
- [x] `grep -c 'actions/cache@v3'` == 1
- [x] `grep -c 'path: ~/.cache/serena-test/jdtls'` == 1
- [x] `grep -c "hashFiles('testdata/fixtures/java/\*\*')"` == 1
- [x] `grep -c 'JDTLS_VERSION'` >= 2 (got 3)
- [x] `grep -c 'go test'` >= 1 (got 3)
- [x] `grep -c 'go vet'` >= 1 (got 3)
- [x] `grep -Pn '^\t'` returns no output
- [x] No duplicate workflow `name:` (only the new file matches)

### Task 2 (USAGE.md)

- [x] `grep -c 'warm jdtls cache'` >= 1
- [x] `grep -c 'make clean-jdtls-cache'` >= 1 (got 2)
- [x] `grep -c 'make bench-jdtls-warm'` >= 1
- [x] `grep -c 'SERENA_TEST_JDTLS_DATA_DIR'` >= 1
- [x] `grep -c 'Do not set this variable in production'` == 1
- [x] `grep -c 'concurrent'` >= 1 in jdtls warm context
- [x] `grep -c 'serena-test/jdtls'` >= 1 (got 3 — three OS lines)
- [x] Line count increased by ≥ 25 (+42 actual)

## Deviations from Plan

None — plan executed exactly as written.

Notes:

- Local YAML validation used a one-shot `gopkg.in/yaml.v3` Go program because `python3 -c 'import yaml'` failed on this host (no PyYAML). Equivalent semantic check (full parse to map) — workflow YAML structure validates.
- CI run not yet triggered (acceptance criteria do not require it; Phase 50 owns CI greenness). Workflow validity proven structurally + by grep checklist.

## Threat Model Compliance

| Threat ID  | Disposition | Status                                                                                                              |
| ---------- | ----------- | ------------------------------------------------------------------------------------------------------------------- |
| T-48-05-01 | mitigate    | jdtls pinned via `JDTLS_VERSION: "1.57.0"` env at workflow level; cannot silently upgrade.                          |
| T-48-05-02 | mitigate    | Cache scoped to `~/.cache/serena-test/jdtls`; key includes `hashFiles('testdata/fixtures/java/**')` for invalidation. |
| T-48-05-03 | accept      | Public-info logs only.                                                                                              |
| T-48-05-04 | accept      | GitHub 10 GB cache limit; reviewed in PRs.                                                                          |
| T-48-05-05 | mitigate    | `permissions: contents: read` set at job level; no write scope.                                                     |
| T-48-05-06 | accept      | Standard CI logs sufficient for v1.9.                                                                               |
| T-48-05-07 | mitigate    | USAGE.md contains literal "Do not set this variable in production" sentence (grep-verified == 1).                    |

No new `threat_flag` surface introduced by this plan.

## Downstream Contract

Phase 48 closure surface:

- CI gate: `.github/workflows/go-test.yml` runs `go vet ./... && go test ./... -count=1` on every PR + push to main, with a warm jdtls workspace restored from the previous run's cache.
- Developer documentation: USAGE.md `## Development` → `### Java integration tests (warm jdtls cache)` is the canonical reference for the warm-cache workflow, env-var contract, and known limitations.

Subsequent phases that touch CI (Phase 50 Go 1.25 / gopls work) inherit this workflow as the baseline; Phase 50 may bump `go-version`, adjust the test invocation, or split the workflow into matrix jobs without changing the warm-cache wiring.

## Self-Check: PASSED

- FOUND: .github/workflows/go-test.yml (created, 51 lines, YAML-valid)
- FOUND: USAGE.md (modified, +42 lines, all grep terms present)
- FOUND commit: 350a480d (`feat(48-05): add go-test CI workflow with warm jdtls cache`)
- FOUND commit: bee06b20 (`docs(48-05): document warm jdtls cache workflow in USAGE.md`)
