# Phase 48: bug-jdtls-warm-cache - Context

**Gathered:** 2026-04-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the Java integration suite (`test/integration/java_test.go`) run as part of the default `go test ./...` by reusing a warm jdtls workspace on disk across runs. Eliminate the `//go:build integration` tag and the `testing.Short()` skip on java tests. Record CI wall-clock before/after and verify a second consecutive run reuses the cache measurably faster than the first.

Out of scope: upstream jdtls tuning, multi-JDK matrix, and other jdtls performance work (deferred to BUG-DEFER-01).

</domain>

<decisions>
## Implementation Decisions

### Warm Workspace Location & Scope
- **D-01:** Warm jdtls `-data` directory lives under `$XDG_CACHE_HOME/serena-test/jdtls/` (fallback `~/.cache/serena-test/jdtls/` on POSIX, `%LOCALAPPDATA%\serena-test\jdtls\` on Windows). Outside the repo so it survives `git clean -fdx` and keys cleanly into CI cache.
- **D-02:** Keyed per-fixture: one warm `-data` dir per Java fixture, matching jdtls's one-workspace-per-project mental model. Directory name embeds the fixture content hash + jdtls version hash (see D-04/D-05) so old and new coexist safely.

### Cache Invalidation & Lifecycle
- **D-03:** Warm workspaces are invalidated by two inputs, combined into the cache key:
  - **D-04:** Fixture content hash — SHA-256 over sorted file list + contents under `testdata/fixtures/java/<fixture>/`. Mandatory for correctness.
  - **D-05:** jdtls identity hash — hash of the resolved jdtls binary path plus its reported version. Prevents cryptic index-corruption crashes after a jdtls upgrade.
- **D-06:** Keyed-path coexistence: a new hash produces a new directory; stale siblings are **not** deleted during a test run (avoids races with concurrent `go test` invocations on the same machine).
- **D-07:** A `make clean-jdtls-cache` target wipes `$XDG_CACHE_HOME/serena-test/jdtls/` for manual cleanup. Document in USAGE.md and Makefile help text.

### Skip-Gate & Build-Tag Policy
- **D-08:** Remove the `//go:build integration` tag from `test/integration/java_test.go` so bare `go test ./...` includes it. Satisfies success criterion #1 literally.
- **D-09:** Remove the `testing.Short()` skip from all Java tests. With warm-cache in place there is no justification for it.
- **D-10:** `requireLS(t, "jdtls")` stays as the sole gate. Contributors without jdtls on PATH still see a clean skip, not a failure.
- **D-11:** Planner must audit whether removing the `integration` tag affects other helpers in `test/integration/` that share the same build-tag assumption (fixture helpers, `StartTestDaemon`, etc.). If any helper relies on the tag to gate itself, scope the tag-drop to the Java file only or split helpers appropriately.

### Speed Measurement & CI Wall-Clock
- **D-12:** Add a `make bench-jdtls-warm` Makefile target that runs the Java suite twice (cold then warm) and prints both wall-clocks. Output pasted into the phase review satisfies success criteria #3 and #4 without introducing new bench infrastructure.
- **D-13:** CI reuses the warm cache via `actions/cache`, keyed on the same inputs as the local key (fixture content hash + jdtls version). Cache miss falls back to cold start — no custom logic required.
- **D-14:** The phase review must record: (a) local first-run wall-clock, (b) local second-run wall-clock, (c) CI wall-clock with cold cache, (d) CI wall-clock with warm cache.

### Claude's Discretion
- Implementation module placement for the hash/key helpers (likely a new helper under `test/integration/` or a small `jdtlscache` package) — planner decides.
- Exact hash encoding (hex-truncated vs base32, prefix length) — planner/executor decides, constrained by "fits in a directory name on all three OSes."
- Whether to set jdtls `-configuration` alongside `-data` under the same keyed dir — planner decides based on how `StartTestDaemon`/langregistry invoke jdtls today.
- Concurrent-test lock strategy (e.g. a `flock`-style sentinel so two parallel `go test` invocations don't corrupt the same warm dir) — planner to evaluate; call it out as a follow-up if deferred.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Scope & Requirements
- `.planning/ROADMAP.md` §"Phase 48: bug-jdtls-warm-cache" — goal, success criteria, dependency on BUG-03.
- `.planning/REQUIREMENTS.md` §"BUG-03" — requirement text and cross-phase mapping.
- `.planning/PROJECT.md` — project-wide constraints (Go-only, no CGO, single-binary distribution).

### Existing Test Infrastructure (must read before changing)
- `test/integration/java_test.go` — the file being modified; current build tag and skip live here.
- `test/integration/*.go` (fixture helpers, `StartTestDaemon`, `requireLS`, `PrepareFixture`) — shared integration-test utilities whose behaviour the warm-cache change must preserve.
- `testdata/fixtures/java/` — the fixture contents that feed the content-hash key.

### Language Server Wiring
- `internal/langregistry/` — where jdtls is registered, installed, and resolved. Source of the jdtls path/version used for D-05.
- `internal/kernel/lspool/` — LS worker pool; planner must confirm how `-data` is passed through today before deciding where the warm path plugs in.

### CI
- `.github/workflows/*.yml` — where `actions/cache` wiring for D-13 will land.

No external specs or ADRs are referenced for this phase — all inputs are in-repo.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `PrepareFixture(t, "java")` in `test/integration/` — currently copies the fixture to a fresh temp dir per test. Hook point for redirecting the `-data` path to the warm cache (the fixture copy itself may stay ephemeral; only the jdtls workspace needs to persist).
- `StartTestDaemon(t, Options{...})` — `Options` likely needs a new field (or an env-var pathway) to carry the warm `-data` directory into the jdtls launch.
- `requireLS(t, "jdtls")` — remains the single gate after the build tag and `testing.Short()` are removed.

### Established Patterns
- Test fixtures under `testdata/fixtures/<lang>/` — Java fixture already exists; the content-hash input is a stable sorted walk over this tree.
- `langregistry` three-tier LS resolution (PATH > managed > error) already surfaces both the binary path and install state, making D-05's "path + version" hash trivial to compute.

### Integration Points
- Java test entry points: `TestSymbols_JavaFixture`, `TestEdit_JavaFixture` in `test/integration/java_test.go`.
- jdtls invocation point inside the LS pool (where `-data <dir>` is constructed) — confirm exact call site during research.
- CI workflow file(s) under `.github/workflows/` — add `actions/cache` step keyed on the same inputs as the local warm-cache dir.

</code_context>

<specifics>
## Specific Ideas

- Cache directory name pattern: `$XDG_CACHE_HOME/serena-test/jdtls/<fixture-name>-<fixture-hash[:12]>-<jdtls-hash[:12]>/`. Human-readable enough to eyeball in `ls` output, short enough to stay under Windows path limits.
- `make bench-jdtls-warm` should run the Java suite once with the cache pre-nuked (cold) and once immediately after (warm), printing both wall-clocks in a diff-friendly format for the phase review.

</specifics>

<deferred>
## Deferred Ideas

- Upstream jdtls tuning (JVM flags, worker counts, project-import hints) — already deferred to BUG-DEFER-01 per ROADMAP.
- Multi-JDK matrix testing — out of scope.
- Concurrent-test advisory lock for the warm dir — planner to evaluate; if judged non-trivial, split out as a follow-up phase rather than expanding Phase 48 scope.
- Benchmark gate integration for jdtls warm/cold timing — `make` target suffices for v1.9.

</deferred>

---

*Phase: 48-bug-jdtls-warm-cache*
*Context gathered: 2026-04-24*
