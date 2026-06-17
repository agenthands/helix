# Phase 78: Internal ToolBench — Go First + LanguageRunner Interface - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-17
**Phase:** 78-internal-toolbench-go-first-languagerunner-interface
**Areas discussed:** Semantic/LSP per cell, Capability test shape, Corpus layout & naming, LanguageRunner contract

---

## Semantic/LSP per cell

User initially asked to discuss the pros and cons before choosing; a full tradeoff analysis was
provided (cwd-fix cost grounded in `subprocess.StartDaemon` delegating into eval's `StartDaemon`,
which doesn't expose `cmd.Dir`; semantic store rejects absolute paths per T-57-02-01).

| Option | Description | Selected |
|--------|-------------|----------|
| A — Enable per-cell (cwd fix everywhere) | Apply the eval StartDaemon working-dir fix to all cells; semantic index enabled uniformly. Cleanest end state, no per-capability knob, removes the P77 stopgap — at the cost of per-cell DuckDB/indexing latency on all 10 capabilities. | |
| B — Targeted enablement | Disabled by default (fast live-LSP path); store-dependent fixtures opt in via a per-cell config knob using the eval StartDaemon working-dir extension. Contains DuckDB cost, protects ≤90s budget, still proves the differentiator. | ✓ |
| C — Live-LSP only, defer store | Keep semantic disabled globally; all fixtures on live-LSP/tree-sitter/RepoMap; incremental update gets a store-less proxy or is deferred. Smallest diff, but under-tests the semantic differentiator. | |

**User's choice:** B — Targeted enablement.
**Notes:** A and B cost the same hard part (the eval `StartDaemon` working-dir extension); B contains
the DuckDB/indexing cost to fixtures that genuinely need it. C was advised against (hollows out the
"incremental update" capability).

### Follow-up: store scope

| Option | Description | Selected |
|--------|-------------|----------|
| Incremental update only | Only the incremental-update fixture(s) enable the store (the one capability that structurally needs the Phase 70 overlay-drain refresh); everything else uses live LSP / RepoMap / tree-sitter. | ✓ |
| Incremental + graph caps | Also store-back the call-graph + dependency-graph fixtures. | |
| You decide per capability | Determine during planning; rule: store-on only when un-testable without it. | |

**User's choice:** Incremental update only (with the planning-time rule that any other capability
found un-testable without the store may opt in under the same knob).

---

## Capability test shape

| Option | Description | Selected |
|--------|-------------|----------|
| Uniform solve-then-go-test | Every capability is a task the scripted agent solves; `go test` verifies the resulting code state (reuse P77 outcome path). Read/query caps engineered so correct completion requires consuming the tool's full output; scripted_agent.yaml calls the capability's tool by construction. One fixture shape, zero new verify infra. | ✓ |
| Two fixture types (golden for reads) | Mutating caps use solve-then-`go test`; pure-read caps assert tool OUTPUT against a checked-in golden via a new verify mechanism. More direct, but a second verification path + golden-output maintenance. | |
| You decide per capability | Determine per capability during planning. | |

**User's choice:** Uniform solve-then-go-test.

### Follow-up: capability tagging

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit field in task.json | `capability` enum field in task.json; runner aggregates coverage; CAPABILITIES.md is the human doc; `IT-go-<capability>-<n>` ID mirrors it. Explicit, greppable, decouples ID format from coverage logic. | ✓ |
| Derive from task ID | Parse capability from the `IT-go-<capability>-<n>` ID; no separate field. DRY but couples ID format to coverage logic. | |
| Central manifest | A capabilities.yaml/CAPABILITIES.md table is the single source the runner reads. One list, but a second place to keep in sync. | |

**User's choice:** Explicit field in task.json.

---

## Corpus layout & naming

| Option | Description | Selected |
|--------|-------------|----------|
| Add language axis | Layout `internal-toolbench/<lang>/<task>`; benchmark=`internal-toolbench`; matrix Cell gains a Language field + `--languages` flag (default `go`); seed-dir join `<benchmark>/<lang>/<task>`. Honors criterion #2's literal path AND is the seam Phase 85 needs. | ✓ |
| Language-as-benchmark | benchmark=`internal-toolbench-go` (matches LICENSES.md id), flat join unchanged. Minimal matrix change, but on-disk path misses criterion #2's nested `internal-toolbench/go/`. | |
| You decide | Pick during planning. | |

**User's choice:** Add language axis.

### Follow-up: seed migration + defaults cutover

| Option | Description | Selected |
|--------|-------------|----------|
| Migrate + flip defaults | `git mv` the seed into `internal-toolbench/go/<task>` (preserves history), assign IT-go ID + capability tag, flip `toolbench-go`→`internal-toolbench` defaults (Makefile/main.go/BENCH.md). Hard constraint: `make bench-quick` stays green. Single clean cutover. | ✓ |
| Build alongside, defer cleanup | Leave `toolbench-go/sum-doubler` + defaults; build new corpus alongside; defer removal. Lowest risk, but two parallel corpora + stale defaults. | |
| You decide | Decide mechanics during planning. | |

**User's choice:** Migrate + flip defaults.

---

## LanguageRunner contract

| Option | Description | Selected |
|--------|-------------|----------|
| RunTests = outcome, verify.sh = fallback | Cell dispatches by language to a registered LanguageRunner; RunTests runs `go test ./... -json`, returns structured pass/fail + per-capability results (feeds Phase 79). verify.sh retired for internal-toolbench, kept as fallback for runner-less benchmarks. | ✓ |
| verify.sh stays sole outcome; runner is coverage-only | Leave the P77 spine untouched; LanguageRunner is a separate corpus-coverage surface. Lowest risk, but two verification mechanisms that can drift. | |
| You decide | Decide during planning. | |

**User's choice:** RunTests = outcome, verify.sh = fallback.

### Follow-up: Capabilities() semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Static per-language declaration | Capabilities() returns the set the runner is DESIGNED to support (Go = 10; Phase 85: Python/Rust/TS = 8, C#/C++ = 6). Coverage report cross-references declared vs fixtures-present; gives the per-language thresholds a denominator + gap detection. | ✓ |
| Derived from fixtures on disk | Informational only; coverage = union of task.json capability fields. Simpler but loses the declared-but-missing gap signal. | |
| You decide | Pick during planning. | |

**User's choice:** Static per-language declaration.

---

## Claude's Discretion

- Exact `LanguageRunner` method signatures, `RunTests` return type / `Capabilities` value type, and
  `bench/languages/` package layout (semantics fixed in D-10/D-11; Go syntax is discretion).
- The 10 per-capability Go fixture designs (modules + scripted_agent.yaml sequences + `go test`
  assertions) within the D-04/D-05 shape.
- The seed task's exact `capability` assignment + `IT-go-*` index.
- `PHASE67_CROSSWALK.md` scope (inspiration-mapping doc only; no `T-67-*` code migration).
- Whether the store opt-in knob is an explicit `task.json` field or derived from the capability value.
- The cleanest additive seam for the eval `StartDaemon` working-dir extension.

## Deferred Ideas

- Other 7 LanguageRunners + fixtures — Phase 85.
- Evaluators + 17 result-schema metrics — Phase 79.
- Golden-output assertion mechanism — explicitly not built (D-04).
- Container runtime / per-language images / GHCR mirror — Phases 84–88.
- Multi-run / pass@k path threading — Phase 79/82.
- Relaxing the semantic store's absolute-path rejection (T-57-02-01) — out of scope; D-03 uses the
  per-cell working-dir route instead.
