---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
plan: 05
subsystem: bench/runtime + daemon/shutdown + obs
tags: [ablation, semantic, no_semantic, ABLATE-06, D-05, runtime-verification, tdd, MODE.md]
requires:
  - "helix_semantic_store_reads_total counter + read chokepoint (Plan 81-01)"
  - "effSemanticDisabled kernel gate forcing NoopLookup on the no_semantic arm (Plan 81-04)"
  - "bench cell RunCell + daemon-tap + result.v2 builder (Phase 77)"
  - "trace.TapDaemonLog daemon-log JSONL msg== dispatch (Phase 67)"
provides:
  - "scrapeSemanticReadsTotal: parses the daemon shutdown log line for the read-counter value (Task 0 path A)"
  - "assertNoSemanticReads: fail-closed predicate — hard-fails the no_semantic cell on any non-zero read (D-05)"
  - "RunCell wiring: scrape + assert after daemon-tap; CellResult.SemanticStoreReads + SemanticReadViolation"
  - "obs.Metrics.SemanticStoreReadsValue(): integer getter for the labelless counter"
  - "daemon shutdown emits msg=\"semantic store reads total\" count=N (the parseable signal)"
  - "TestNoSemanticZeroReads (pass + hard-fail + scope + scrape cases)"
  - "your_agent_no_semantic/MODE.md rewritten — gate key + 8-consumer enumeration (criterion #4)"
affects:
  - "bench/runtime (the no_semantic arm now fails the cell hard if a semantic read survived the kernel gate)"
  - "Phase 82 aggregator (the no_semantic row no longer carries the guarantee_pending_phase_81 partial marker)"
tech-stack:
  added: []
  patterns:
    - "Counter exposure via daemon-log shutdown line (Task 0 path A) — HTTP-disabled Unix-socket sandbox makes the Prometheus /metrics scrape unreachable"
    - "Fail-closed runtime gate (mirror the Phase 80 fairness gate): violation -> hard cell failure + preserve scratch, NOT a warn"
    - "Pure testable seam: assertNoSemanticReads / scrapeSemanticReadsTotal decoupled from the live daemon so RED/GREEN drives the count deterministically"
key-files:
  created:
    - bench/runtime/no_semantic_zero_reads_test.go
  modified:
    - bench/runtime/cell.go
    - bench/runtime/cell_test.go
    - bench/runners/your_agent_no_semantic/MODE.md
    - internal/obs/metrics.go
    - internal/daemon/shutdown.go
decisions:
  - "Task 0: chose path A (daemon-log shutdown line). The bench daemon runs --http-addr= (HTTP off) over a Unix socket (D-06), so the Prometheus /metrics scrape is unreachable. The daemon's stderr is captured to <modeDir>/daemon.log as JSONL (--json), which trace.TapDaemonLog already reads by msg== dispatch. The daemon emits ONE line msg=\"semantic store reads total\" count=N in shutdown.go; the cell's scrapeSemanticReadsTotal parses it (last occurrence wins). Exact signal: msg string == \"semantic store reads total\", integer field == \"count\"."
  - "Path A daemon emission landed (pre-declared contingency for internal/daemon/daemon.go in files_modified). Realized in internal/daemon/shutdown.go (the d.shutdown() path reached after g.Wait()), NOT daemon.go directly — shutdown.go IS d.shutdown(), so the contingency's intent (single shutdown-path log line) is satisfied without editing daemon.go. No deviation flag needed (pre-declared)."
  - "Added obs.Metrics.SemanticStoreReadsValue() (reads the labelless counter via dto.Metric.Write) so the daemon can emit the integer value; nil-safe, returns 0. client_model/go (dto) is already a go.mod dependency (transitive of prometheus client_golang) — zero new packages."
  - "Hard-fail shape: a violation routes through RunCell's existing preserve(...) path (returns non-nil error, sets CellResult.SemanticReadViolation, preserves scratch). This is a CORRECTNESS failure not infra, but reusing preserve gives the fail-closed + debuggable behavior the plan requires (mirror the fairness gate's fatal shape)."
  - "Deferral marker removal: deleted ablationStatusFor entirely (it only existed to emit guarantee_pending_phase_81 for the no_semantic arm) and forced AblationStatus: \"\" for every mode at the BuildResult call. Rewrote the orphaned TestAblationStatus as TestAblationStatusMarkerRemoved (asserts ablation_status omitted for no_semantic too)."
  - "MODE.md frontmatter set to mode: no_semantic per the plan's criterion #4 (the no_lsp analog uses the short name). The resolver only reads the `profile` field — `mode` is never compared to the directory name — so the short name is safe; bench/runners tests stay green."
metrics:
  duration: ~35m
  completed: 2026-06-20
  tasks: 3
  files: 6
---

# Phase 81 Plan 05: no_semantic Zero-Reads Runtime Verification + MODE.md Rewrite Summary

Landed the independent runtime verification (D-05, criterion #2): the bench cell
now asserts `helix_semantic_store_reads_total == 0` after a `no_semantic` run and
FAILS the cell HARD (not a warn) on any non-zero read — the dynamic complement to
Plan 04's structural build-but-block gate. Because the DuckDB store is still
BUILT (D-04) yet the counter reads zero, the assertion is non-vacuous: it proves
a store that exists and could be queried was NOT. Removed the
`guarantee_pending_phase_81` deferral marker (the kernel guarantee lands this
phase) and rewrote `your_agent_no_semantic/MODE.md` to document the gate key plus
the full 8-consumer strangler-fig enumeration (criterion #4).

## Task 0 (Wave 0) — Counter-Exposure Path: PATH A (daemon-log shutdown line)

**Chosen mechanism:** a single parseable daemon-log line emitted at shutdown.

**Why:** the bench daemon is spawned `helix --serve --socket=<cell>/daemon.sock
--http-addr= --json` (internal/eval/sandbox/sandbox.go:270) — `--http-addr=` is
empty, so there is NO TCP listener and the Prometheus `/metrics` HTTP scrape is
unreachable in the sandbox (RESEARCH Open Q2, P77 D-06/D-07). The daemon's stderr
is redirected to `<modeDir>/daemon.log` (sandbox.go:238,308) as JSONL (`--json`),
and `trace.TapDaemonLog` already parses that file by dispatching on the `msg`
field (tap.go:69 `switch raw.Msg`). Path A reuses that exact substrate.

**Exact signal (documented for Task 1's parser):**
- `msg` string: `"semantic store reads total"`
- integer field: `"count"`
- Emitted in `internal/daemon/shutdown.go` `d.shutdown()` (Phase 1.6, before the
  listener close) via `d.logger.Info("semantic store reads total", "count",
  d.obs.Metrics().SemanticStoreReadsValue())`. `d.obs.Metrics()` is never nil
  (Noop-default invariant).

**Reachability confirmed:** the line lands in the same daemon.log the cell already
taps; `scrapeSemanticReadsTotal` reads it after `h.Kill()` (the daemon flushes its
shutdown line on graceful exit). A daemon that never ran the semantic subsystem
emits no line → scraper returns 0 (treated as zero reads, not an error).

**Scope contingency realized:** the path-A daemon emission was the pre-declared
contingency (files_modified marked `internal/daemon/daemon.go` "CONTINGENT on path
A"). It landed in `internal/daemon/shutdown.go` — which IS the body of
`d.shutdown()` reached after `g.Wait()` — so the contingency's intent (one
shutdown-path log line) is satisfied; `daemon.go` itself was not edited. No
deviation flag required (pre-declared).

## Task 1 (RED→GREEN) — counter==0 cell assertion + hard-fail + marker removal

- **RED (a50ec2cb):** `bench/runtime/no_semantic_zero_reads_test.go` with
  `TestNoSemanticZeroReads` — pass case (no_semantic, count 0 → no error),
  hard-fail case (no_semantic, count N>0 → error naming the counter + the count),
  scope guard (other 4 modes with count 42 → no error), and three
  `scrapeSemanticReadsTotal` cases (count 0 line, non-zero line, missing line →
  0). RED was a compile failure (both helpers undefined) — the correct
  pre-implementation state.
- **GREEN (4d5d1cb2):**
  - `internal/obs/metrics.go`: `SemanticStoreReadsValue()` getter (dto.Metric
    read; nil-safe).
  - `internal/daemon/shutdown.go`: the path-A shutdown emission.
  - `bench/runtime/cell.go`: `scrapeSemanticReadsTotal` (daemon-log parse) +
    `assertNoSemanticReads` (fail-closed predicate, no-op off the no_semantic
    arm), wired into `RunCell` after the daemon-tap (step 5b) with
    `CellResult.SemanticStoreReads` + `SemanticReadViolation`; a violation routes
    through `preserve(...)` (hard error + scratch preserved). Removed
    `ablationStatusFor` and forced `AblationStatus: ""` for every mode.
  - `bench/runtime/cell_test.go`: rewrote `TestAblationStatus` →
    `TestAblationStatusMarkerRemoved` (the no_semantic row now also omits
    `ablation_status`).
- **REFACTOR:** none needed — the two helpers are already minimal and pure.

## Task 2 — your_agent_no_semantic/MODE.md rewrite (criterion #4)

Rewrote the file mirroring `no_lsp/MODE.md`: frontmatter (`mode: no_semantic` /
`profile: bench-no-semantic`), a one-paragraph arm description, and gate
documentation that names (a) the koanf gate key `semantic_index.bench_disabled`
and its precedence (`CLI --disable-semantic-subsystem > profile YAML
disable_semantic_subsystem > default-off`), (b) the build-but-block behavior
(store still built, reads forced to `NoopLookup` + disabled `ConfigGate`, source
== `tree_sitter`) plus the `helix_semantic_store_reads_total == 0` runtime proof,
and (c) the full 8-consumer strangler-fig enumeration (`get_repo_map`,
`get_context`, `find_related_symbols`, `explain_symbol_deep`,
`validate_graph_edge`, `analyze_blast_radius`, `RankFiles`, `ExpandFrom`). Deleted
the `ablation_status: guarantee_pending_phase_81` deferred-guarantee section.

## Verification

- `go test ./bench/runtime/ -run TestNoSemanticZeroReads -count=1` — PASS (pass +
  hard-fail + scope + scrape cases)
- `go test ./bench/runtime/... ./internal/obs/... ./internal/daemon/... -count=1`
  — PASS
- `go test ./...` — ALL PASS (0 FAIL)
- `go vet ./...` — clean
- `make vet` — exit 0 (incl. the Plan 03 `vet-ablation-leakage` gate and all
  five custom vet tools)
- `gofmt` — clean on all touched files
- AC greps: `grep -c "helix_semantic_store_reads_total" bench/runtime/cell.go` = 3
  (≥1); `grep -c "guarantee_pending_phase_81" bench/runtime/cell.go` = 0; MODE.md
  criterion-#4 gate (all 8 consumers + `semantic_index.bench_disabled` present,
  `guarantee_pending_phase_81` absent) — satisfied
- `git log` shows `test(81-05)` (a50ec2cb) preceding `feat(81-05)` (4d5d1cb2) —
  RED→GREEN gate

## TDD Gate Compliance

- RED commit `a50ec2cb test(81-05): add failing no_semantic zero-reads cell
  assertion` — compile failure (helpers undefined), the correct pre-implementation
  RED state.
- GREEN commit `4d5d1cb2 feat(81-05): ...` — `TestNoSemanticZeroReads` passes.
- No REFACTOR commit (helpers already in consolidated form).

## Deviations from Plan

None — plan executed as written. The path-A daemon emission was a pre-declared
contingency (Task 0 acceptance criteria + files_modified), not a deviation; it
landed in `internal/daemon/shutdown.go` (the `d.shutdown()` body) rather than
`daemon.go` itself, which satisfies the contingency's single-shutdown-line intent.
The one orphaned-test update (`TestAblationStatus` → `TestAblationStatusMarkerRemoved`)
is a required consequence of the plan's mandated deferral-marker removal, not a
scope expansion.

## Commits

- `a50ec2cb` test(81-05): add failing no_semantic zero-reads cell assertion
- `4d5d1cb2` feat(81-05): assert helix_semantic_store_reads_total == 0 on no_semantic cell + remove deferral marker
- `ec26e3e7` docs(81-05): rewrite no_semantic MODE.md — gate key + 8-consumer enumeration (criterion #4)

## Self-Check: PASSED

- bench/runtime/no_semantic_zero_reads_test.go — FOUND
- bench/runtime/cell.go — FOUND (scrapeSemanticReadsTotal + assertNoSemanticReads + RunCell wiring + marker removed)
- bench/runtime/cell_test.go — FOUND (TestAblationStatusMarkerRemoved)
- bench/runners/your_agent_no_semantic/MODE.md — FOUND (rewritten, 8 consumers + gate key)
- internal/obs/metrics.go — FOUND (SemanticStoreReadsValue)
- internal/daemon/shutdown.go — FOUND (path-A shutdown emission)
- Commit a50ec2cb (test) — FOUND
- Commit 4d5d1cb2 (feat) — FOUND
- Commit ec26e3e7 (docs) — FOUND
