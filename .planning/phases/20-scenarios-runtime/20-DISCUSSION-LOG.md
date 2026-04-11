# Phase 20: Scenarios & Runtime - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 20-scenarios-runtime
**Areas discussed:** Fixture matrix scope, Scenario workflow depth, Runtime stress approach, Degraded mode injection

---

## Fixture Matrix Scope

### Polyglot Monorepo

| Option | Description | Selected |
|--------|-------------|----------|
| Composite of existing | Combine Go + Python + TS subdirs into one fixture | ✓ |
| Purpose-built monorepo | New fixture with intentional cross-language imports | |
| You decide | Claude picks | |

**User's choice:** Composite of existing (Recommended)
**Notes:** Reuses proven code, minimal new files.

### Unsupported Language

| Option | Description | Selected |
|--------|-------------|----------|
| Plain text / Markdown files | No LS for .md/.txt | |
| Exotic real language (Haskell) | Real language, no LS installed | |
| Fake .xyz extension | Invented extension, guaranteed unsupported | ✓ |

**User's choice:** Fake `.xyz` extension
**Notes:** User provided detailed rationale: upstream Serena explicitly supports Markdown, so .md is unsafe. .xyz is deterministic, environment-independent, cannot accidentally become supported. Added rule: extension must not be present in shipped Serena-Go language manifest.

### Name Collisions

| Option | Description | Selected |
|--------|-------------|----------|
| Same symbol names across languages | Config, Handler, Parse in Go/Python/TS | |
| Same filenames, different content | Multiple main.go, main.py, main.ts in subdirs | |
| Both patterns combined | Same symbols AND same filenames | ✓ |

**User's choice:** Both patterns combined
**Notes:** User specified structure: `backend/config/main.go`, `worker/config/main.py`, `web/config/main.ts` each defining Config, Handler, Parse. Tests both symbol cross-contamination and path disambiguation. Keep fixture intentionally minimal.

---

## Scenario Workflow Depth

| Option | Description | Selected |
|--------|-------------|----------|
| Full cycle | activate → search → read → edit → verify | ✓ |
| Read-only workflows | activate → search → read → verify (no edits) | |
| Tiered by fixture | Full for Go, read-only for others | |

**User's choice:** Full cycle (Recommended)
**Notes:** Each fixture gets one representative full-cycle scenario.

### Profile/Mode Fixtures

| Option | Description | Selected |
|--------|-------------|----------|
| Single Go fixture | Profile behavior is language-independent | |
| Go + one other language | Go + Python for language-specific mode bugs | ✓ |
| You decide | Claude picks | |

**User's choice:** Go + one other language
**Notes:** Two-language coverage to catch language-specific mode filtering bugs.

---

## Runtime Stress Approach

### Load Driver

| Option | Description | Selected |
|--------|-------------|----------|
| Real concurrent goroutines | Fan out N goroutines against real daemon | |
| testing/synctest deterministic | Deterministic scheduling for unit tests | |
| Tiered: both layers | synctest for unit, goroutines for integration | ✓ |

**User's choice:** Tiered: both layers
**Notes:** synctest for circuit breaker/TTL logic, real goroutines for integration-level pool stress.

### Clean Shutdown

| Option | Description | Selected |
|--------|-------------|----------|
| In-flight work + signal | Start calls, SIGTERM mid-flight, assert drain | ✓ |
| Idle shutdown only | Clean shutdown when no work in-flight | |
| You decide | Claude picks | |

**User's choice:** In-flight work + signal (Recommended)
**Notes:** Check runtime.NumGoroutine and os.Process for stuck LS subprocesses.

---

## Degraded Mode Injection

### Injection Method

| Option | Description | Selected |
|--------|-------------|----------|
| Interface-level test doubles | Failing implementations at daemon construction | ✓ |
| Environment-based fault injection | Bad config/paths cause real failures | |
| You decide | Claude picks | |

**User's choice:** Interface-level test doubles (Recommended)
**Notes:** Clean, no process-level hacks. Tests degraded startup path directly.

### Subsystems

| Option | Description | Selected |
|--------|-------------|----------|
| Language server (LS) | LS installer error, symbol tools report honestly | ✓ |
| Memory store (SQLite) | Memory init fails, memory tools error | ✓ |
| Skill init failure | Init() error, tools unavailable | ✓ |
| All three | Each tested independently | ✓ |

**User's choice:** All three (Recommended)
**Notes:** Three separate test cases, each subsystem tested independently.

---

## Claude's Discretion

- Test file organization, runtime test location, concurrency parameters, test double structure, synctest placement

## Deferred Ideas

None
