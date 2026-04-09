# Phase 8: Advanced Testing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-04-09
**Phase:** 08-advanced-testing
**Areas discussed:** Profile source of truth, Concurrency test design, Error path coverage

---

## Profile Source of Truth

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcode expected lists | Write the expected tool names directly in the test. Catches unintended profile changes but needs manual updates. | |
| Load from YAML dynamically | Read .serena/profiles/*.yaml at test time. Self-updates but can mask bugs where profile YAML is wrong. | |
| Hybrid: hardcode counts, dynamic names | Assert total count is correct (hardcoded) and tool names match YAML. | |
| Golden files (user refinement) | Checked-in testdata/profiles/*.golden with sorted tool lists. -update flag for explicit changes. | ✓ |

**User's choice:** Golden files
**Notes:** User refined option 1 — implement as checked-in golden expectations, not inline string slices. Golden files provide independent oracles from production YAML, so bad YAML changes can't self-approve. Separate YAML loader unit tests use dynamic loading. Contract tests (TestReadProfileExposesExpectedTools etc.) use golden files. OWASP integrity testing alignment.

---

## Concurrency Test Design

| Option | Description | Selected |
|--------|-------------|----------|
| t.Parallel() subtests | Many parallel subtests calling different tools on shared daemon. Realistic usage pattern. | |
| Goroutine fan-out | Explicit N goroutines each calling same tool. Targeted stress. Easier to reproduce. | |
| Both layered | t.Parallel() subtests + targeted goroutine fan-out + testing/synctest for deterministic units | ✓ |

**User's choice:** Both layered (three tiers)
**Notes:** Scenario stress (t.Parallel() for mixed realistic usage), hot-path fan-out (targeted acquire/release, queue saturation, shutdown mid-work), testing/synctest for small scheduler-sensitive units. All under -race. CI matrix: baseline race, heavy stress, optional synctest. Loop-variable capture caveat noted.

---

## Error Path Coverage

| Option | Description | Selected |
|--------|-------------|----------|
| Exhaustive per tool | Every tool × every error class. ~150 tests. High maintenance. | |
| Representative per category | One tool per category tests error classes. ~30 tests. | |
| Category + critical tools | Representative per category + exhaustive on high-risk tools (edit, safe_delete) | ✓ |

**User's choice:** Category + critical tools (three bands)
**Notes:** Band 1: representative per category for common framework errors (no workspace, invalid args, missing file, symbol not found, permission denied, timeout, cancellation, provider unavailable). Band 2: exhaustive per tool for destructive/mutating (edit, rename, safe_delete, write_memory). Band 3: thin smoke for low-risk read-only. Go table-driven style. Assert on error type/code, not message text. Google testing guidance on risk/cost tradeoff. OWASP integrity negative testing alignment.

---

## Claude's Discretion

- Error matrix table structure, golden file format, fan-out goroutine counts, make test-stress target

## Deferred Ideas

None
