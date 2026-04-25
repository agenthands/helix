# Phase 50: toolchain-go1.25-gopls-ci - Research

**Researched:** 2026-04-25
**Domain:** CI / Go toolchain / benchmark gating
**Confidence:** HIGH

## Summary

Phase 50 is a **documentation-and-baseline phase, not a code change phase.** The technical work (Go 1.25 adoption, gopls upgrade to v0.21.1, benchgate in blocking mode) is already done. CONTEXT.md locks every meaningful design decision; the phase exists to (a) capture a real `v1.9` ubuntu-latest baseline file, (b) flip `bench.yml` to read it, (c) write a short gopls subsection in CONTRIBUTING.md, (d) delete one sentence from PROJECT.md. The PR's own `go-test.yml` and `bench.yml` runs are the success-criteria evidence (D-10).

There is **no library research, no architecture, no API discovery** to perform. The risk surface is pure CI mechanics: baseline-capture parameter parity, file path swaps, doc edits. All risks are catalogued in `test/bench/baselines/README.md` and the in-source Pitfall comments already wired into `bench.yml`.

**Primary recommendation:** Plan as a single small wave: (1) trigger `capture-baseline.yml` with `milestone=v1.9` to produce `test/bench/baselines/v1.9-github-hosted.txt`, (2) author one PR that swaps the two `bench.yml` references from `v1.1-github-hosted.txt` → `v1.9-github-hosted.txt`, edits CONTRIBUTING.md, and deletes the tech-debt sentence from PROJECT.md. The PR's own green CI is the verification.

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Baseline strategy**
- **D-01:** Capture a fresh `v1.9` ubuntu-latest baseline by running `.github/workflows/capture-baseline.yml` with `milestone=v1.9`. Output file: `test/bench/baselines/v1.9-github-hosted.txt`.
- **D-02:** Switch `.github/workflows/bench.yml` `--baseline` flag and the `benchstat` summary step to compare against `test/bench/baselines/v1.9-github-hosted.txt` (replacing the `v1.1-github-hosted.txt` reference).
- **D-03:** Keep the existing darwin/arm64 placeholder baselines (`v1.1-github-hosted.txt`, `v1.2-phase10/11/12-github-hosted.txt`) on disk as historical reference. Do not delete.
- **D-04:** v1.9 baseline must be captured with the **same flags as the PR gate**: `-short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...` (Pitfall 1 parity). `BenchmarkFullRepoSmoke` stays excluded.

**gopls strategy documentation**
- **D-05:** Add a dedicated subsection in `CONTRIBUTING.md` covering: (a) current pin `v0.21.1`, (b) why pinned (skips broken v0.17.1 on linux/amd64; pin also stabilizes cold-start/warm-reuse bench metrics — Pitfall 4), (c) where the pin is set (`GOPLS_VERSION` env in `bench.yml` and `capture-baseline.yml`), (d) bump policy: any version change requires a re-baseline PR per `test/bench/baselines/README.md` refresh policy.
- **D-06:** Keep `GOPLS_VERSION` declared independently in `bench.yml` and `capture-baseline.yml`. Do NOT introduce a reusable workflow or shared versions file. The CONTRIBUTING.md note must explicitly state the two values MUST stay in sync and reference Pitfall 1.

**PR threshold tuning**
- **D-07:** Keep PR-tier thresholds at the current `15%` time / `25%` allocs at `p<0.05`. Do not pre-emptively loosen or tighten.
- **D-08:** No pre-launch noise sampling required. Observe real PR behavior post-launch.
- **D-09:** No in-workflow retry / auto-retry logic. Flake escape hatch is the GitHub Actions "Re-run failed jobs" button — manual only. No CONTRIBUTING.md "best-of-3" prescription.

**Verification scope**
- **D-10:** Single **verification PR**: bundle the v1.9 baseline file + bench.yml `--baseline` switch + CONTRIBUTING.md gopls section + PROJECT.md tech-debt removal. PR's own `go-test.yml` and `bench.yml` runs on ubuntu-latest are the success-criteria evidence.
- **D-11:** No `workflow_dispatch` dry-run before opening the PR.
- **D-12:** Out of scope for Phase 50: auditing/fixing `docker.yml`, `junie.yml`, `pytest.yml`, `docs.yaml`, `publish.yml`, `codespell.yml`. Only `go-test.yml` and `bench.yml` (plus the `capture-baseline.yml` invocation) are in scope.

**Tech-debt cleanup**
- **D-13:** SC #4 — remove the "Benchmark baselines captured locally..." sentence from `.planning/PROJECT.md` Context section. Other tech-debt items in that paragraph stay.
- **D-14:** SC #3 — the gopls writeup in CONTRIBUTING.md (D-05) IS the documented "upgrade" decision (chosen over patch / replacement).

### Claude's Discretion
- Exact placement / heading wording of the gopls subsection in `CONTRIBUTING.md` (top-level vs. nested under "CI" / "Benchmarks").
- Whether the v1.9 baseline capture is committed via the existing `stefanzweifel/git-auto-commit-action@v5` flow in `capture-baseline.yml` or pulled into the verification PR by hand.
- Whether the PROJECT.md edit also adjusts surrounding sentences for flow vs. deletion-only.

### Deferred Ideas (OUT OF SCOPE)
- Single source of truth for `GOPLS_VERSION` (reusable workflow or `.github/versions.env`).
- Audit of `docker.yml`, `junie.yml`, `pytest.yml`, `publish.yml`, `docs.yaml`, `codespell.yml` for Go 1.25 / ubuntu-latest correctness.
- Pre-launch noise sampling to data-derive PR-tier thresholds.
- Bench retry / auto-retry logic in bench.yml.
- Release-tier baseline (full bench, no `-short`) including `BenchmarkFullRepoSmoke`.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TOOL-01 | Serena builds, tests, benches green on `ubuntu-latest` with Go 1.25; gopls v0.17.1 root cause resolved | Already implemented in `go-test.yml` (Go 1.25.x) and bench workflows (`GOPLS_VERSION: v0.21.1`). Phase 50 ratifies + documents in CONTRIBUTING.md (D-05, D-14). |
| TOOL-02 | CI bench gate runs on `ubuntu-latest` enforcing PR-tier thresholds; closes benchmarks tech-debt note | `bench.yml` already runs blocking benchgate on ubuntu-latest with `15%/25%` thresholds. Phase 50 swaps the placeholder v1.1 darwin baseline for a real ubuntu-latest v1.9 baseline (D-01, D-02) and removes tech-debt note (D-13). |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Bench baseline capture | CI / GitHub Actions | — | `capture-baseline.yml` workflow_dispatch; auto-commit via `stefanzweifel/git-auto-commit-action@v5` |
| Bench gate enforcement | CI / GitHub Actions | benchgate Go binary | `bench.yml` runs `go run ./test/bench/cmd/benchgate` blocking PR merge |
| Gopls strategy documentation | Repo docs (CONTRIBUTING.md) | — | Human-facing policy doc, no code |
| Tech-debt status | Repo docs (PROJECT.md) | — | Remove single sentence from Context section |

## Standard Stack

This phase introduces **zero new dependencies**. All required infrastructure is already wired:

| Component | Version | Purpose | Location |
|-----------|---------|---------|----------|
| Go | `1.25.x` | Toolchain target | `go.mod` (1.25.1), `bench.yml`, `capture-baseline.yml`, `go-test.yml` `actions/setup-go@v5` |
| gopls | `v0.21.1` | LSP for warm-reuse benches | `GOPLS_VERSION` env in `bench.yml:41` and `capture-baseline.yml:32` |
| benchstat | `latest` | Human-readable diff summary | `golang.org/x/perf/cmd/benchstat` (installed in workflow) |
| benchgate | local | Blocking regression gate | `test/bench/cmd/benchgate/main.go` — uses `golang.org/x/perf/benchfmt` and `benchmath.AssumeNormal.Compare` |
| `actions/setup-go` | `v5` | Go toolchain in CI | All 3 Go workflows |
| `actions/checkout` | `v4` | Repo checkout | All 3 Go workflows |
| `stefanzweifel/git-auto-commit-action` | `v5` | Auto-commit baseline file | `capture-baseline.yml:69` |
| `actions/upload-artifact` | `v4` | Bench artifact upload | `bench.yml:98` |

**No `npm install` / `go get` step required.** All packages are already in `go.mod` (`golang.org/x/perf v0.0.0-20260312031701-16a31bc5fbd0`).

## Architecture Patterns

### Baseline Lifecycle (current state, post-phase)

```
                  ┌─────────────────────────────┐
  manual trigger  │ capture-baseline.yml        │
  milestone=v1.9 ─►   - setup Go 1.25.x         │
                  │   - install gopls v0.21.1   │
                  │   - run bench (D-04 flags)  │
                  │   - verify ≥5 Benchmark lns │
                  │   - auto-commit to branch   │
                  └──────────────┬──────────────┘
                                 ▼
                  test/bench/baselines/
                    v1.9-github-hosted.txt   ◄── new (D-01)
                    v1.1-github-hosted.txt   ◄── kept as historical (D-03)
                    v1.2-phase10/11/12-...   ◄── kept as historical (D-03)
                                 │
                                 ▼
  PR opened ──────► bench.yml (PR gate)
                      - setup Go 1.25.x
                      - install gopls v0.21.1
                      - run bench (-short -count=10)
                      - benchstat -alpha 0.05  ──► reads v1.9 baseline (D-02)
                      - benchgate --baseline   ──► reads v1.9 baseline (D-02)
                                                   exit 1 if 15%/25% breach
                                                   AND p<0.05
```

### Verification PR Flow (D-10)

```
  branch: gsd/phase-50-toolchain-go1.25-gopls-ci
    │
    ├── test/bench/baselines/v1.9-github-hosted.txt   (from capture-baseline)
    ├── .github/workflows/bench.yml                    (2 path swaps)
    ├── CONTRIBUTING.md                                (new gopls subsection)
    └── .planning/PROJECT.md                           (delete 1 sentence)
    │
    ▼ open PR
    │
    ├── go-test.yml runs ─► green = SC #1 evidence
    └── bench.yml runs    ─► green = SC #2 evidence
                              (compares PR HEAD against new v1.9 baseline)
```

### Existing Project Structure (relevant subset)

```
.github/workflows/
├── bench.yml                  # PR gate — D-02 edits 2 paths here
├── capture-baseline.yml       # baseline producer — D-01 invokes; no edits needed
├── go-test.yml                # build/vet/test — already green on Go 1.25.x
└── (others)                   # OUT OF SCOPE per D-12

test/bench/
├── baselines/
│   ├── README.md              # refresh policy referenced in D-05 doc text
│   ├── v1.1-github-hosted.txt           # KEEP (D-03)
│   ├── v1.2-phase10-github-hosted.txt   # KEEP (D-03)
│   ├── v1.2-phase11-github-hosted.txt   # KEEP (D-03)
│   ├── v1.2-phase12-github-hosted.txt   # KEEP (D-03)
│   └── v1.9-github-hosted.txt           # NEW (D-01)
└── cmd/benchgate/             # blocking gate — already in v0.05/15%/25% mode

CONTRIBUTING.md                # D-05 inserts subsection (location is discretion)
.planning/PROJECT.md           # D-13 deletes 1 sentence in Known tech debt para
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Capture baseline to disk | Custom shell script in PR | `capture-baseline.yml` `workflow_dispatch` | Already wired with parity guarantees (Pitfall 1), auto-commits via `git-auto-commit-action@v5` |
| Compare bench output | Hand-parse benchstat text | `benchgate` reads `golang.org/x/perf/benchfmt` | Pitfall 9 — benchstat is human output, not API |
| Bench statistical significance | Custom t-test code | `benchmath.AssumeNormal.Compare` (already in benchgate) | Already implemented |
| Centralize `GOPLS_VERSION` | New `.github/versions.env` or composite workflow | Documented duplication + Pitfall 1 reminder in CONTRIBUTING.md | D-06 explicitly defers — duplication kept intentionally |
| Bench flake retry | `nick-fields/retry@v3` wrapper | Manual "Re-run failed jobs" | D-09 explicitly defers |

## Common Pitfalls

### Pitfall 1: Bench / Capture Parameter Drift (HIGH risk for this phase)
**What goes wrong:** `bench.yml` and `capture-baseline.yml` use different `GOMAXPROCS`, `GOPLS_VERSION`, Go version, or bench flags. Result: PR runs compare apples to oranges, every PR fails the gate.
**Why it happens:** The two files duplicate the env block (D-06 keeps this). A change to one without the other breaks parity.
**How to avoid:** When editing either file, diff the env blocks and the `go test -bench` invocation by hand. CONTRIBUTING.md gopls subsection (D-05) MUST call this out so future contributors notice.
**Warning signs:** Suddenly every PR shows large bench deltas in the same direction, or benchgate fails on benchmarks that did not change.

**Current parity state (verified):**
- `bench.yml`: `GOMAXPROCS=4`, `GOPLS_VERSION=v0.21.1`, Go `1.25.x`, flags `-short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...`
- `capture-baseline.yml`: `GOMAXPROCS=4`, `GOPLS_VERSION=v0.21.1`, Go `1.25.x`, flags `-short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...`
- ✅ Match.

### Pitfall 4: Gopls Version Drift Tainting Bench Metrics
**What goes wrong:** Cold-start and warm-reuse benchmarks (`BenchmarkLSPIndex_Cold`, `BenchmarkLSPIndex_Warm`) are sensitive to gopls internal indexing changes. An unpinned `gopls@latest` introduces noise on every PR.
**Why it happens:** gopls is upstream Go-tools project, releases happen multiple times per year.
**How to avoid:** Pin `GOPLS_VERSION` and bump deliberately via re-baseline PR. CONTRIBUTING.md must document this. (D-05.)

### Pitfall 5: Runner Core-Count Variance
**What goes wrong:** `ubuntu-latest` runners ship with different core counts depending on GitHub's hardware rollout. Bench numbers shift without code changes.
**Mitigation already in place:** `GOMAXPROCS: "4"` pinned in both bench workflows.

### Pitfall 8: Bench Artifact Bloat
**What goes wrong:** Heap snapshots committed per-PR balloon repo size.
**Mitigation already in place:** `bench.yml` uploads via `actions/upload-artifact@v4` only; never committed.

### Pitfall 9: Parsing Benchstat Text
**What goes wrong:** benchstat output format is human-oriented and changes without notice.
**Mitigation already in place:** `benchgate` parses `golang.org/x/perf/benchfmt` library output directly.

### Pitfall 11: Full-Repo Smoke in PR Gate
**What goes wrong:** `BenchmarkFullRepoSmoke` is too slow for PR gating (~10-15min).
**Mitigation already in place:** `-short` flag in both bench.yml and capture-baseline.yml skips it.

### Phase-50-specific gotcha: Verification PR self-bootstrap
**What goes wrong:** The verification PR includes `bench.yml` swap to v1.9 AND the new v1.9 baseline file. If the file is missing or the swap path typo'd, the PR's own `bench.yml` run fails with "no such file" — masking real bench results.
**How to avoid:** Before opening the verification PR, confirm:
1. `test/bench/baselines/v1.9-github-hosted.txt` exists, has ≥5 `Benchmark` lines (matches `capture-baseline.yml` verify step).
2. `bench.yml` references the exact path `test/bench/baselines/v1.9-github-hosted.txt` (no typos) in BOTH the `benchstat` step and the `benchgate --baseline` flag.

## Runtime State Inventory

> Phase 50 is a CI-config + docs phase. No databases, no live services, no OS state, no secrets renamed.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — bench baselines are tracked git files, not runtime data | None |
| Live service config | GitHub Actions caches (`actions/setup-go@v5 cache: true`, `actions/cache@v3` for jdtls) — opaque, key-driven, will repopulate on first run | None |
| OS-registered state | None | None |
| Secrets/env vars | `GOPLS_VERSION` (env var only — public value, not a secret); `JDTLS_VERSION`, `SERENA_TEST_LS_TIMEOUT` (unchanged) | None |
| Build artifacts | None — phase does not change `go.mod`, `go.sum`, or any built binary | None |

**Nothing in any category requires migration.** This is the simplest possible state surface.

## Code Examples

### Triggering the baseline capture (D-01)

```sh
# Via gh CLI
gh workflow run capture-baseline.yml -f milestone=v1.9 --ref <branch>

# Or via UI: Actions > capture-baseline > Run workflow > milestone=v1.9
```

The workflow auto-commits `test/bench/baselines/v1.9-github-hosted.txt` to the triggering branch via `stefanzweifel/git-auto-commit-action@v5`.

### bench.yml path swap (D-02)

Two locations in `.github/workflows/bench.yml` need the path swap:

**Location 1 — benchstat summary step (currently line ~80):**
```yaml
- name: Print benchstat summary
  run: |
    benchstat -alpha 0.05 \
      test/bench/baselines/v1.9-github-hosted.txt \
      /tmp/bench-new.txt
```

**Location 2 — benchgate step (currently line ~88):**
```yaml
- name: Run benchgate
  run: |
    go run ./test/bench/cmd/benchgate \
      --baseline test/bench/baselines/v1.9-github-hosted.txt \
      --new /tmp/bench-new.txt
```

### CONTRIBUTING.md gopls subsection skeleton (D-05) — recommended placement: nested under existing `## Benchmark CI Gate` section

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

### PROJECT.md sentence to delete (D-13)

Current text in `.planning/PROJECT.md` line 139, "Known tech debt" paragraph:

```
Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest due to gopls v0.17.1 incompatibility with Go 1.25 on linux/amd64.
```

Delete this sentence. Keep the rest of the paragraph (3 redundant GrammarRegistry instances was Phase 49 [done]; rust-analyzer rename = Phase 47 [done]; jdtls cold-start = Phase 48/56). Per discretion, light flow cleanup of surrounding sentences is acceptable.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Bench baseline captured on darwin/arm64 (placeholder) | Captured on `ubuntu-latest` via `capture-baseline.yml` | Phase 50 (this) | PR gate compares apples-to-apples |
| Gopls `v0.17.1` (broken on Go 1.25 linux/amd64) | Gopls `v0.21.1` pinned | Already in workflows | Build/test/bench unblocked |
| Benchgate `--warn-only` | Benchgate blocking | Phase 15 | Real PR enforcement |
| Hand re-baseline (download + commit) | `capture-baseline.yml` `workflow_dispatch` + auto-commit | Phase 9-15 | One-click refresh |

**Deprecated/outdated:**
- `v1.1-github-hosted.txt` as the active PR baseline — kept on disk per D-03 as historical reference, but no longer referenced by `bench.yml` after this phase.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + `golang.org/x/perf/benchfmt` (benchgate) |
| Config file | `bench.yml` (gate config); thresholds compiled into `test/bench/cmd/benchgate/main.go` |
| Quick run command | `go vet ./...` |
| Full suite command | `go test ./... -count=1` |
| Bench command | `go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TOOL-01 | `go build/vet/test` green on ubuntu-latest + Go 1.25 | CI integration | `go-test.yml` workflow run on PR | ✅ |
| TOOL-01 | gopls strategy documented | Manual review | grep CONTRIBUTING.md for gopls section | ❌ Wave 0 (writing the section IS the test) |
| TOOL-02 | Bench gate fails PR on threshold breach | CI integration | `bench.yml` workflow run on PR | ✅ |
| TOOL-02 | Tech-debt note removed | Manual review | grep PROJECT.md absent of v0.17.1 sentence | ❌ Wave 0 (deletion IS the test) |

### Sampling Rate
- **Per task commit:** `go vet ./...` (already runs locally per CLAUDE.md)
- **Per wave merge:** N/A (single-wave phase)
- **Phase gate:** Verification PR's `go-test.yml` AND `bench.yml` both green (D-10)

### Wave 0 Gaps
- None — existing test infrastructure (`go-test.yml`, `bench.yml`, benchgate) covers all phase requirements. The doc edits (CONTRIBUTING.md, PROJECT.md) are validated by visual review on the PR, not by an automated test.

## Security Domain

> No code changes that touch authentication, authorization, cryptography, input validation, or session handling. Workflow permissions stay as-is (`bench.yml: contents: read`; `capture-baseline.yml: contents: write` for auto-commit). No new third-party actions added.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2-V11 | no | — |
| V14 Configuration | yes | Pinned action versions (`@v4`, `@v5`); pinned Go version (`1.25.x`); pinned `GOPLS_VERSION` |

**Note:** `benchstat@latest` is unpinned (a known TODO in `bench.yml:62`). Out of scope for Phase 50 (no CONTEXT.md decision); flag as a future hardening item if pursued separately.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| GitHub Actions ubuntu-latest | bench.yml, capture-baseline.yml, go-test.yml | ✓ | runner-managed | — |
| Go 1.25.x | All Go workflows | ✓ | provided by `actions/setup-go@v5` | — |
| gopls v0.21.1 | bench workflows | ✓ | `go install golang.org/x/tools/gopls@v0.21.1` | — |
| `gh` CLI (local, for triggering capture) | D-01 | ✓ (assumed available locally) | — | UI workflow_dispatch |

**No missing dependencies.** Phase is fully runnable.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `capture-baseline.yml` auto-commit path will succeed on a feature branch (not just main). The action targets the triggering branch by default — verified by reading the workflow (no explicit `branch:` arg). | Architecture / D-01 flow | If branch protection rules block the auto-commit, fall back to manual download (CI artifact) and hand-commit. Discretion item already covers this. |
| A2 | The bench gate's "blocking" status was successfully flipped in Phase 15 (per `test/bench/baselines/README.md` "three-step rollout COMPLETED"). | State of the Art table | Verified by inspection: `bench.yml` no longer carries `--warn-only`. ✅ |

**No claims tagged `[ASSUMED]` are load-bearing decisions** — both items above are verified or have clear fallbacks.

## Open Questions

1. **Should `benchstat@latest` be pinned at the same time as documenting gopls?**
   - What we know: `bench.yml:62` has a TODO comment to pin to a commit SHA.
   - What's unclear: User did not raise this in discussion; not in CONTEXT.md.
   - Recommendation: Out of scope for Phase 50. Note in phase review as a follow-up todo so it doesn't get lost.

2. **Should the gopls subsection live under `## Benchmark CI Gate` (existing) or as a new `## CI Tooling` top-level section?**
   - This is a Claude's discretion item. Recommendation: nest under `## Benchmark CI Gate` because the pin's rationale is bench-stability-driven, and that section already exists. Top-level placement implies a broader CI tooling story that does not yet exist.

## Project Constraints (from CLAUDE.md)

The planner MUST honor these:

- **Always run `go vet ./...` and `go test ./...` before completing any Go task.** (No Go code changes in this phase, but local validation of edited workflow YAML via `actionlint` would be analogous; not strictly required.)
- **Single Go binary, no JetBrains/proprietary backends.** N/A — this phase touches CI and docs only.
- **MCP is the primary protocol.** N/A.
- **Same repo, Python lives in `legacy/`.** N/A.
- **GSD workflow enforcement:** Edits go through this phase's plan tasks, not direct repo edits.

## Sources

### Primary (HIGH confidence)
- `.planning/phases/50-toolchain-go1.25-gopls-ci/50-CONTEXT.md` — locked decisions
- `.planning/phases/50-toolchain-go1.25-gopls-ci/50-DISCUSSION-LOG.md` — alternatives considered
- `.planning/REQUIREMENTS.md` §TOOL-01, §TOOL-02
- `.planning/ROADMAP.md` Phase 50 entry — success criteria
- `.planning/PROJECT.md` line 139 — tech-debt sentence to delete
- `.github/workflows/bench.yml` — current PR gate (verified line numbers and env)
- `.github/workflows/capture-baseline.yml` — baseline producer (verified parity)
- `.github/workflows/go-test.yml` — already on Go 1.25.x + ubuntu-latest
- `test/bench/baselines/README.md` — refresh policy referenced in D-05
- `test/bench/cmd/benchgate/main.go` — gate logic (already blocking, 15%/25% defaults)
- `CONTRIBUTING.md` lines 178-185 — existing `## Benchmark CI Gate` section (target for D-05 nesting)
- `go.mod` line 3 — Go 1.25.1 confirmed
- `go.mod` line 45 — `golang.org/x/perf` already a direct dep

### Secondary (MEDIUM confidence)
- None needed — no external research required.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Locked decisions: HIGH — every meaningful question is answered in CONTEXT.md
- CI mechanics: HIGH — all workflows read directly, parity verified by inspection
- Pitfalls: HIGH — already documented in `bench.yml` header comments and `test/bench/baselines/README.md`
- Doc placement: MEDIUM — discretion item, recommendation given but final call belongs to executor

**Research date:** 2026-04-25
**Valid until:** 2026-05-25 (CI config can drift if other workflows are touched in parallel; verify `bench.yml` and `capture-baseline.yml` parity at execution time)
