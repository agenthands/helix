---
phase: 50
slug: toolchain-go1-25-bench-local
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-28
---

# Phase 50 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
>
> Phase 50 is a doc/config phase. Verification is grep-based filesystem checks plus the existing `go vet` + `go test` gate per CLAUDE.md. No new test files are needed — Wave 0 is intentionally empty.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (existing); shell `grep` / `test -f` for filesystem assertions |
| **Config file** | none — uses repo `go.mod` and existing `.github/workflows/go-test.yml` |
| **Quick run command** | `go vet ./... && go test ./... -short -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -count=1` |
| **CI gate** | `gh run list --workflow=go-test.yml --branch=$(git rev-parse --abbrev-ref HEAD) --limit=1 --json conclusion` (criterion 1) |
| **Estimated runtime** | ~30s quick (short tests), ~2–4 min full |

---

## Sampling Rate

- **After every task commit:** Run the per-task `<verify><automated>` block from the PLAN (most are `test -f` / `! grep` filesystem checks completing in <1s).
- **After every plan wave:** Run `go vet ./... && go test ./... -short -count=1` (CLAUDE.md project rule).
- **Before `/gsd-verify-work`:** Full suite (`go vet ./... && go test ./... -count=1`) must be green AND the latest `go-test.yml` run on `ubuntu-latest` Go 1.25.x for the current branch must conclude `success` (covers TOOL-01).
- **Max feedback latency:** ~30s for quick run; doc-edit tasks verify in <1s.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Test Type | Automated Command | Status |
|---------|------|------|-------------|------------|-----------|-------------------|--------|
| 50-01-01 | 01 | 1 | TOOL-02 | accept (deletion reduces surface) | filesystem | `! test -f .github/workflows/bench.yml && ! test -f .github/workflows/capture-baseline.yml` | ⬜ pending |
| 50-01-02 | 01 | 1 | TOOL-02 | accept | filesystem | `! test -d test/bench/cmd/benchgate && ! ls test/bench/baselines/v1.*-github-hosted.txt 2>/dev/null` | ⬜ pending |
| 50-01-03 | 01 | 1 | TOOL-02 | accept | grep sweep | `! grep -rn 'benchgate' --include='*.go' --include='*.md' --include='*.yml' --include='*.yaml' --include='Makefile' --exclude-dir=_superseded --exclude-dir=.planning --exclude-dir=legacy .` | ⬜ pending |
| 50-02-01 | 02 | 2 | TOOL-02 | accept | grep + smoke | `grep -E '^bench: ## ' Makefile && grep -E '^bench-baseline: ## ' Makefile && make -n bench >/dev/null` | ⬜ pending |
| 50-02-02 | 02 | 2 | TOOL-02 | mitigate (gitignore-shape correctness — Pitfall 3) | gitignore behavior | `git check-ignore -q test/bench/baselines/local.txt && ! git check-ignore -q test/bench/baselines/README.md` | ⬜ pending |
| 50-02-03 | 02 | 2 | TOOL-02 | accept | grep | `grep -q 'make bench' test/bench/baselines/README.md && ! grep -q 'benchgate' test/bench/baselines/README.md && ! grep -qE 'v1\\.[0-9]+-github-hosted' test/bench/baselines/README.md` | ⬜ pending |
| 50-03-01 | 03 | 2 | TOOL-01 | accept | grep | `grep -q 'gopls' CONTRIBUTING.md && grep -q '>=v0.21' CONTRIBUTING.md && grep -q 'benchmark' CONTRIBUTING.md` | ⬜ pending |
| 50-03-02 | 03 | 2 | TOOL-01 | accept | grep | `grep -q 'gopls version compatibility' USAGE.md && ! grep -q 'gopls version incompatibility' USAGE.md && grep -q 'gopls v0.21' USAGE.md` | ⬜ pending |
| 50-03-03 | 03 | 2 | TOOL-02 | mitigate (surgical edit — preserve siblings) | grep | `! grep -q 'GrammarRegistry instances' .planning/PROJECT.md && ! grep -q 'CI ubuntu-latest' .planning/PROJECT.md && grep -q 'rust-analyzer' .planning/PROJECT.md && grep -q 'jdtls cold-start' .planning/PROJECT.md` | ⬜ pending |
| 50-04-01 | 04 | 3 | TOOL-01 | accept (CLAUDE.md mandatory gate) | go test | `go vet ./... && go test ./... -count=1` | ⬜ pending |
| 50-04-02 | 04 | 3 | TOOL-02 | accept | grep sweep | `! grep -rn 'benchgate' --include='*.go' --include='*.md' --include='*.yml' --include='Makefile' --exclude-dir=_superseded --exclude-dir=.planning --exclude-dir=legacy .` | ⬜ pending |
| 50-04-03 | 04 | 3 | TOOL-01 | accept | gh CLI checkpoint | `gh run list --workflow=go-test.yml --branch="$(git rev-parse --abbrev-ref HEAD)" --limit=1 --json conclusion -q '.[0].conclusion' \| grep -q success` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.* No new test files are needed: this phase is verification-by-deletion and verification-by-grep over existing source. The only "test" surface is the existing `go test ./...` suite which must remain green per CLAUDE.md and per Phase 50 criterion 1.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `make bench` executes the benchmark suite locally and prints results to stdout | TOOL-02 | Bench execution is local-only by project rule and is not automated in CI; a contributor must observe it works | Run `make bench` on a developer machine; expect benchstat-formatted output; expect non-zero runtime; expect exit 0 |
| `make bench-baseline` writes to `test/bench/baselines/local.txt` and the file is ignored by git | TOOL-02 | Same as above — local execution surface | Run `make bench-baseline`; verify `test -f test/bench/baselines/local.txt` and `git status` does not list the file |
| `.github/workflows/go-test.yml` concludes `success` on `ubuntu-latest` Go 1.25.x post-merge | TOOL-01 | CI conclusion is observed via GitHub, not produced locally | Wait for the workflow run on the merge commit; confirm conclusion `success` and runner image `ubuntu-latest` with `setup-go` version `^1.25` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (Wave 0 is empty by design — no test stubs needed for a doc/config phase)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (every task has a `<verify><automated>` block)
- [x] Wave 0 covers all MISSING references (none — verification is grep + existing test suite)
- [x] No watch-mode flags
- [x] Feedback latency < 50s for per-task verification (filesystem checks <1s; quick `go test` ~30s; full ~2–4 min)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-28
