# Phase 48: bug-jdtls-warm-cache - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-24
**Phase:** 48-bug-jdtls-warm-cache
**Areas discussed:** Warm workspace location, Cache invalidation & lifecycle, Skip-gate & build-tag policy, Speed measurement & CI wall-clock

---

## Gray Area Selection

| Option | Description | Selected |
|--------|-------------|----------|
| Warm workspace location | Where the reused jdtls -data dir lives and whether per-fixture or shared | ✓ |
| Cache invalidation / lifecycle | When to nuke the warm workspace and how cleanup happens | ✓ |
| Skip-gate & build-tag policy | Build tag + testing.Short() skip on Java tests | ✓ |
| Speed measurement & CI wall-clock | How success criteria #3 and #4 are captured | ✓ |

**User's choice:** All four areas.

---

## Warm workspace location

### Q1: Where should the reusable jdtls -data directory live?

| Option | Description | Selected |
|--------|-------------|----------|
| $XDG_CACHE_HOME/serena-test/jdtls (Recommended) | Outside repo, XDG convention, survives `git clean`, CI can actions/cache it | ✓ |
| testdata/.jdtls-cache (repo-local, gitignored) | Inside repo tree; dies on `git clean -fdx` | |
| Stable OS temp path | Uses os.TempDir with a stable subdirectory name; may be wiped on reboot | |
| Workspace-copy pattern | Keep PrepareFixture copying and put -data under the fixture root; likely defeats warming | |

**User's choice:** XDG cache path.

### Q2: Per-fixture vs shared warm dir?

| Option | Description | Selected |
|--------|-------------|----------|
| Per-fixture keyed by fixture path + content hash (Recommended) | Each fixture gets its own warm -data dir; cleanest isolation | ✓ |
| Single shared warm dir | One -data for all Java tests; cross-test interference risk | |
| Per-test-function | Defeats warming when tests share a fixture | |

**User's choice:** Per-fixture.

---

## Cache invalidation / lifecycle

### Q1: What invalidates a warm workspace?

| Option | Description | Selected |
|--------|-------------|----------|
| Fixture content hash (Recommended) | SHA of all files under the fixture; mandatory for correctness | ✓ |
| jdtls binary version/path hash (Recommended) | Prevents index corruption after jdtls upgrades | ✓ |
| Serena version / commit SHA | Too aggressive — effectively disables warming during active development | |
| Explicit TTL | Extra dimension; only useful if workspaces are known to rot | |

**User's choice:** Content hash + jdtls version hash.

### Q2: How is the cache cleaned?

| Option | Description | Selected |
|--------|-------------|----------|
| Keyed path coexistence + periodic manual clean (Recommended) | New hash → new dir; manual `make clean-jdtls-cache` target; no in-run deletion | ✓ |
| Auto-delete stale entries on startup | Races between parallel `go test` invocations | |
| Never prune | Accepts unbounded growth | |

**User's choice:** Keyed coexistence with manual Makefile clean target.

---

## Skip-gate & build-tag policy

### Q1: Drop the //go:build integration tag?

| Option | Description | Selected |
|--------|-------------|----------|
| Drop the tag — tests run under bare `go test ./...` (Recommended) | Satisfies criterion #1 literally; requireLS still gates absent jdtls | ✓ |
| Keep //go:build integration | Bare `go test ./...` excludes them — violates criterion #1 | |
| Move to a different tag (e.g. slow/lsp) | Same net effect as keeping the tag | |

**User's choice:** Drop the tag.

### Q2: What happens to testing.Short() skip?

| Option | Description | Selected |
|--------|-------------|----------|
| Remove entirely (Recommended) | Warm cache makes it unnecessary; requireLS remains the only gate | ✓ |
| Keep as -short escape hatch | Preserves an opt-out at minor cost | |
| First-run guard | Skip only if cache cold AND -short; extra logic for narrow case | |

**User's choice:** Remove entirely.

---

## Speed measurement & CI wall-clock

### Q1: How is success criterion #3 captured?

| Option | Description | Selected |
|--------|-------------|----------|
| Makefile `make bench-jdtls-warm` target (Recommended) | Runs suite twice, prints both wall-clocks; pasted into phase review | ✓ |
| Dedicated testing.B benchmark | Overkill; LS-driven benches are slow and flaky in CI | |
| One-off manual timing in RCA/review only | No repeatable artifact | |

**User's choice:** Makefile target.

### Q2: Should CI reuse the warm cache?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — actions/cache keyed on fixture hash + jdtls version (Recommended) | Directly satisfies criterion #4 | ✓ |
| No — CI always cold-starts | Java suite stays a ~2min tax on every CI run | |
| Best-effort cache | Same behaviour as (1) — noise | |

**User's choice:** actions/cache with the same keys as local.

---

## Claude's Discretion

- Hash/key helper module placement.
- Hash encoding (hex length, base32) within OS path-length limits.
- Whether jdtls `-configuration` co-locates with the keyed `-data` dir.
- Concurrent-test advisory lock strategy (or deferring it).

## Deferred Ideas

- Upstream jdtls tuning (BUG-DEFER-01) — already on roadmap.
- Multi-JDK matrix testing.
- Concurrent-test advisory lock (planner call).
- Benchmark-gate integration for jdtls timing.
