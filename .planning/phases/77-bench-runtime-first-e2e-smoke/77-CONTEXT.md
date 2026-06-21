# Phase 77: Bench Runtime & First E2E Smoke - Context

**Gathered:** 2026-06-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Turn the `notYetImplemented("run")` stub in `cmd/helix-bench` (`cmd/helix-bench/main.go:87`)
into a **working end-to-end orchestrator** — on exactly **one** Go task, in **one** mode
(`your_agent_full`), **no container, no per-language sprawl** — so the runtime shape
(subprocess daemon spawn → agent → schema-valid `result.v2.json` → merged OTel trace) is forced
into existence and proven before evaluators (Phase 79) and ablation runners (Phase 80) land on top.

The phase is overwhelmingly **reuse, not invention**: it thin-wraps and wires the Phase 67
`internal/eval/` machinery (sandbox, per-`(task,mode)` daemon spawn, daemon-tap, trace merge,
scripted-agent and real-`claude` agent paths) into the new `bench/runtime/` surface.

**Requirements locked here:** BENCH-04, BENCH-05 (see `.planning/REQUIREMENTS.md` lines 20–21).
WHAT/WHY are locked upstream in REQUIREMENTS.md and v1.12-ROADMAP.md (4 success criteria); the
decisions below are the HOW.

**Graded against (v1.12-ROADMAP Phase 77 success criteria):**
1. `helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=<one>` ≤ 30 s,
   writes schema-valid `result.v2.json`; one daemon subprocess per `(task × mode)`; no port
   collisions on `--parallel=4`.
2. `make bench-quick` exits 0 with ≥ 1 task succeeding in CI ≤ 90 s; `make bench` / `make
   bench-<suite>` invoke `cmd/helix-bench run --benchmarks=…`.
3. `bench/runtime/sandbox/` thin-wraps `internal/eval/sandbox/`; `bench/runtime/subprocess/`
   spawns `helix daemon` and (eventually) `claude` CLI with PID-gated per-cell JSONL paths.
4. First E2E smoke produces a merged OTel trace (daemon-tap + agent-tap) with zero orphan spans
   and zero PID cross-talk.
</domain>

<decisions>
## Implementation Decisions

### Agent driver for the first smoke (criteria #2, #3, #4; BENCH-05)
- **D-01:** **Scripted agent gates CI; real `claude` CLI path wired but NOT gating.** `make
  bench-quick` and the first E2E smoke run the in-process **scripted agent**
  (`internal/eval/runner/scripted_agent.go`, driven by a `scripted_agent.yaml`) — deterministic,
  hermetic, no `$ANTHROPIC_API_KEY`, trivially inside the ≤ 30 s / ≤ 90 s budgets. The real
  `claude` CLI subprocess path (`internal/eval/agent/claude.go` + `mcpconfig.go`) is **fully
  wired and locally runnable** (e.g. `helix-bench run --agent=claude …`) but is NOT a CI gate
  this phase. Mirrors eval-quick's scripted/real split and satisfies criterion #3's "(eventually)
  claude CLI" wording.
- **D-02:** **The agent-tap leg is synthesized as a `CCTapResult` from the scripted agent's
  executed tool-call sequence.** So even the hermetic `bench-quick` produces and **asserts a real
  2-leg merged trace** (`trace.Merge` over `Daemon` + `CC` streams) with zero orphan spans and a
  PID-gated daemon-tap (`trace.TapDaemonLog(daemonPID)`) — making criterion #4 a **CI gate, not a
  manual artifact**. The real `claude` path produces the **same `CCTapResult` shape** from its
  transcript when run. (`CCTapResult` is the existing agent-side evidence stream in
  `internal/eval/trace/merge.go`.)

### Smoke task source — chicken-and-egg with Phase 78 ToolBench (criterion #1; BENCH-04)
- **D-03:** **Ship exactly ONE minimal-but-real Go task** at
  `bench/datasets/toolbench-go/<task>/` (tiny Go module with one failing test + `task.json`
  prompt/metadata + a `verify` step that runs `go test`). The scripted agent replays a **real MCP
  edit** (e.g. via `replace_in_file` / a structured-edit tool) to make the test pass. This honors
  criterion #1's `--benchmarks=toolbench-go --tasks=<one>` naming AND respects the eval↔bench
  separation (P75 D-15 / INFRA-03). **This is the seed task** — Phase 78 grows the full ToolBench
  Go corpus in the same directory. Scope guard: ONE task only; the Go capability matrix is Phase 78.
- **D-04:** **`result.v2.json` populated with outcome + provenance, schema-valid.** Fields:
  pass/fail **outcome from the verify exit code** (reuse `trace.Merge`'s `VerifyExitCode`
  resolution), the **fairness block** (from `bench/runners/fairness_contract.go`), **token/cost
  columns** available from the run, the **merged-trace reference**, and `schema_version: "v2"`.
  Rich metrics (`edit_locality`, `regression_rate`, pass@k) are **left null/absent** and filled by
  Phase 79. The result MUST validate against `bench/schema/result.v2.schema.json` today.

### Mode → profile resolution (criterion #1; boundary with ABLATE-01/Phase 80)
- **D-05:** **Seed the ABLATE-01-shaped convention for ONLY the mode Phase 77 needs.** Create
  `bench/runners/your_agent_full/MODE.md` whose frontmatter declares `profile: bench-full`, plus a
  **small resolver** that maps a canonical mode name → profile name by reading that MODE.md. The
  runner translates `--modes=your_agent_full` → `profileName=bench-full`, then calls the (existing)
  `StartDaemon(ctx, taskID, mode, profileName, cfgPath)` — note `StartDaemon` **already separates**
  the `mode` label (used for the per-cell dir/socket) from the `profileName` (passed as
  `--profile=`). **Phase 80 adds the other 5 mode dirs and extends the same resolver — it grows,
  it does not refactor.** Do NOT rename the Phase 76 profile files (their filenames are locked by
  P76 roadmap criterion #1 + golden tests).

### Daemon transport & per-cell isolation (criteria #1, #3, #4; BENCH-04)
- **D-06:** **Per-cell Unix domain socket; HTTP disabled; no TCP ports.** Reuse eval's proven path
  verbatim: each `(task × mode)` cell spawns `helix --serve --socket=<per-cell-path> --http-addr=
  --json` (HTTP off), and the agent reaches the daemon via the `helix --mode=stdio --socket=…`
  **forwarder**. "No port collisions on `--parallel=4`" holds **by construction** — there are no
  TCP ports to collide.
- **D-07:** **`bench/runtime/sandbox/` EMBEDS `internal/eval/sandbox.Sandbox`** (thin wrapper that
  adds only bench-specific paths such as the `result.v2.json` and synthesized-JSONL locations) —
  **does not fork/copy** the eval sandbox code (upholds P75 D-08/D-15 reuse-don't-fork and
  criterion #3's "thin-wraps" wording). `bench/runtime/subprocess/` owns the `helix daemon` (and
  eventual `claude`) subprocess lifecycle.
- **D-08:** **Cell key = `<run_id>/<task>/<mode>`; durable artifacts vs ephemeral scratch are
  split.** Durable artifacts (`result.v2.json`, merged-trace JSON, `daemon.log`) land under an
  **out dir** (default `bench/reports/<run_id>/<task>/<mode>/`, overridable via `--out`). Ephemeral
  scratch (repo working copy, daemon `HOME`, the Unix socket) lives in an **OS temp sandbox**,
  **deleted on success, PRESERVED on failure** for debugging. The daemon-tap is **PID-gated** to
  the spawned daemon's PID (`trace.TapDaemonLog(daemonPID)`), giving criterion #4's "zero PID
  cross-talk." `run_id` reuses eval's shape (ISO-8601 + git SHA, per `runner.go` `RunID`).

### Claude's Discretion
- The `MODE.md` frontmatter schema beyond the `mode` + `profile` keys (D-05) — keep minimal;
  Phase 80 owns the full ABLATE-01 convention.
- `run_id` exact string format (reuse eval's ISO-8601+gitSHA; exact layout is discretion) (D-08).
- Exact `make bench` / `bench-quick` / `bench-<suite>` target wording and default args — fixed
  semantics are: `bench-quick` runs the hermetic scripted `your_agent_full` smoke on the one
  `toolbench-go` task ≤ 90 s; `bench-<suite>` parameterizes `--benchmarks=<suite>`; all invoke
  `cmd/helix-bench run --benchmarks=…` (BENCH-05).
- Whether `bench/reports/<run_id>/` output is git-ignored (ops detail; `reports/` is a runtime dir
  per P75 D-07).
- The `scripted_agent.yaml` step sequence for the seed task and exact MCP edit tool used (D-03).
- Internal package layout of `bench/runtime/` (`sandbox/`, `subprocess/`, runner glue) and the
  `helix-bench run` flag plumbing (`--benchmarks`, `--modes`, `--tasks`, `--parallel`, `--out`,
  `--agent`).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & requirements (locked WHAT/WHY — do not re-derive)
- `.planning/milestones/v1.12-ROADMAP.md` §"Phase 77" — Goal, Depends-on, Requirements, and the
  **4 Success Criteria** this phase is graded against.
- `.planning/REQUIREMENTS.md` — **BENCH-04** (line 20: reuse eval sandbox/subprocess, one daemon
  per `(task×mode)`, ≤ 30 s, no port collisions) and **BENCH-05** (line 21: `make
  bench`/`bench-quick`/`bench-<suite>` targets). Boundary rows: **ABLATE-01** (line 45 → **Phase
  80**: 6 modes + `bench/runners/<mode>/MODE.md` convention), **METRIC-06** (line 61: trace
  merging, reused Phase 67 trace-tap), **BENCH-03/06** (lines 19/22: schema + `BENCH.md`, done).
- `.planning/ROADMAP.md` §"Phase 77" — active-roadmap entry (promoted from the milestone roadmap
  on 2026-06-16 to unblock discuss/plan tooling).
- `.planning/research/PITFALLS.md` — milestone failure modes; relevant here: daemon-tap PID
  cross-talk (F-07), trace-merge single-host clock assumption, fairness-contract drift,
  `tokens_to_model` vs `tokens_through_daemon` counting, contamination canary (for later phases).

### Reused-pattern source (Depends-on — Phase 67 `internal/eval/`)
- `internal/eval/sandbox/sandbox.go` — `StartDaemon(ctx, taskID, mode, profileName, cfgPath)`
  (spawns `--serve --socket=… --http-addr= --json`), `ModeDir`/`SocketFor`/`HomeFor`/`RepoFor`,
  and the path-traversal + lstat-symlink hardening that criterion #3 says carries forward. **The
  thin-wrap target for `bench/runtime/sandbox/`.**
- `internal/eval/runner/runner.go` + `internal/eval/runner/daemon_tap_integration_test.go` —
  per-`(task,mode)` daemon spawn, the `helix --mode=stdio --socket=` forwarder drive path, the
  `daemon.log` tap, and the `RunID` shape. **The closest analog for the bench runner loop.**
- `internal/eval/runner/scripted_agent.go` — the in-process scripted-agent path (D-01); the
  `scripted_agent.yaml` step format the seed task drives.
- `internal/eval/agent/claude.go` + `internal/eval/agent/mcpconfig.go` — the real `claude` CLI
  subprocess path + per-mode MCP config writer + `ErrClaudeNotFound` (D-01, wired-not-gating).
- `internal/eval/trace/merge.go`, `tap.go`, `schema.go` — `trace.Merge(MergeInput{Daemon, CC, …})`,
  `DaemonTapResult` (PID-gated via `TapDaemonLog`), `CCTapResult` (the agent-tap stream D-02
  synthesizes), and `MergedTrace` (zero-orphan / PID-gating invariants for criterion #4).

### Contracts produced by prior phases (write into these — do not redefine)
- `bench/schema/result.v2.schema.json` — the Phase 75 result schema the smoke MUST validate
  against (D-04); `schema_version: "v2"`.
- `bench/runners/fairness_contract.go` — Phase 75 compile-time model-config pin; the smoke's
  `result.v2.json` fairness block sources from `DefaultContract` (D-04).
- `internal/profile/profiles/bench-full.yaml` (+ `bench-no-lsp` / `bench-no-semantic` /
  `bench-no-structured-edit`) — Phase 76 ablation profiles; `your_agent_full` resolves to
  `bench-full` (D-05).
- `cmd/helix-bench/main.go` — the cobra root with the `notYetImplemented("run")` stub (line 87–89)
  that this phase implements; `internal/cli/root.go:68` defines the daemon `--profile` flag.
- `bench/BENCH.md` — operator-side contract / eval↔bench separation note (Phase 75 BENCH-06/INFRA-03).

### Prior-phase context
- `.planning/phases/75-schema-fairness-contract-tree-skeleton/75-CONTEXT.md` — schema (D-01/02),
  `bench/` six-dir skeleton (D-07: `reports/` is a runtime dir), reuse-don't-fork + eval↔bench
  separation (D-08/D-15) carried into D-04/D-06/D-07.
- `.planning/phases/76-ablation-profiles-kernel-subsystem-disable-flags/76-CONTEXT.md` — the 4
  `bench-*.yaml` profiles + locked filenames feeding D-05; `--profile` precedence (D-02).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/eval/sandbox.Sandbox` — per-`(task,mode)` isolation (private HOME, socket, repo,
  config) with `StartDaemon`. `bench/runtime/sandbox/` **embeds** it (D-07).
- `internal/eval/runner/` — the (task,mode) dispatch loop, scripted-agent path, and daemon-tap
  integration test are near-drop-in references for the bench runner.
- `internal/eval/trace/` — daemon-tap + CC-tap + merge already exist; D-02 reuses `CCTapResult`
  and `MergedTrace` so no new trace schema is invented this phase.
- `internal/eval/agent/` — real `claude` CLI driver + MCP config writer; wired-not-gating (D-01).

### Established Patterns
- **One daemon per `(task,mode)` over a per-cell Unix socket, HTTP disabled** (`StartDaemon` args
  `--socket=… --http-addr=`) — the no-TCP design that makes criterion #1's "no port collisions"
  hold by construction (D-06).
- **PID-gated daemon-tap** (`TapDaemonLog(preKillPid)`, `RejectedForeignPid` counter) — the F-07
  mitigation that delivers criterion #4's "zero PID cross-talk" (D-08).
- **`--mode=stdio --socket=` forwarder** — how an agent (scripted drive or `claude`'s MCP config)
  reaches the per-cell daemon.
- **Reuse-don't-fork `vet-*`/separation discipline** (P75 D-08/D-15) — `bench/runtime/sandbox/`
  wraps, never copies, `internal/eval/sandbox/` (D-07).

### Integration Points
- `cmd/helix-bench/main.go` — implement the `run` subcommand (replace `notYetImplemented("run")`),
  add `--benchmarks` / `--modes` / `--tasks` / `--parallel` / `--out` / `--agent` flags.
- `bench/runtime/` (new) — `sandbox/` (embeds eval sandbox), `subprocess/` (daemon + claude
  lifecycle), and the runner glue that orchestrates a cell end-to-end.
- `bench/datasets/toolbench-go/<task>/` (new) — the single seed Go fixture task (D-03).
- `bench/runners/your_agent_full/MODE.md` (new) + resolver — mode→profile mapping (D-05).
- Root `Makefile` — new `bench`, `bench-quick`, `bench-<suite>` targets invoking `cmd/helix-bench
  run` (D-01 budgets; BENCH-05).
- `bench/reports/<run_id>/<task>/<mode>/` — durable artifact out dir (D-08).

</code_context>

<specifics>
## Specific Ideas

- The smoke command shape is fixed by criterion #1:
  `helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=<one>` ≤ 30 s.
- `bench-quick` is the **hermetic scripted** gate (no API key, ≤ 90 s, ≥ 1 task succeeds) — the
  real-`claude` path is opt-in (e.g. `--agent=claude`), never the CI default (D-01).
- The seed task is **one** Go task only (failing test → scripted MCP edit → `go test` passes);
  it is deliberately the Phase 78 corpus seed, not the corpus (D-03).
- `result.v2.json` is **provenance-complete but metric-sparse** this phase — outcome + fairness +
  tokens + trace ref, schema-valid; metrics are Phase 79 (D-04).
</specifics>

<deferred>
## Deferred Ideas

- **Real `claude` CLI as a CI gate** — needs an API-key secret + network; local/nightly only this
  phase (the path is wired, just not gating) (D-01).
- **Full ToolBench Go corpus + capability matrix** — Phase 78. Phase 77 ships only the seed task
  in `bench/datasets/toolbench-go/` (D-03).
- **Rich result metrics** (`edit_locality`, `regression_rate`, pass@k, multi-run aggregation) —
  Phase 79 (evaluators/metrics) and Phase 82 (aggregator/bootstrap). Phase 77 leaves them
  null/absent (D-04).
- **Remaining 5 modes + the full `bench/runners/<mode>/MODE.md` convention** (ABLATE-01) — Phase
  80. Phase 77 seeds only `your_agent_full/MODE.md` and a resolver that Phase 80 extends (D-05).
- **Container runtime / per-language runners / GHCR mirror** — Phases 84–88; explicitly out of the
  "no container, no per-language sprawl" Phase 77 boundary.

None of the above expand Phase 77 scope — discussion stayed within the runtime-smoke boundary.

### Planner notes (apply during plan-phase)
1. **Phase 77 implements ONE thing end-to-end**: `helix-bench run` on a single `toolbench-go` task
   in `your_agent_full` mode, scripted-agent gated. Resist building the corpus (P78), evaluators
   (P79), or other modes (P80).
2. **Reuse, don't reinvent**: `bench/runtime/sandbox/` embeds `internal/eval/sandbox/`; the runner
   loop, daemon-tap, and trace merge come straight from `internal/eval/runner/` + `…/trace/`.
3. **Unix-socket-per-cell, HTTP disabled** is the no-port-collision mechanism — do not add TCP
   ports (D-06).
4. **The agent-tap is synthesized from the scripted tool-call log** (`CCTapResult`) so the 2-leg
   merged trace (criterion #4) is asserted in hermetic CI — not deferred to a real-`claude` run
   (D-02).
5. **`result.v2.json` MUST validate** against `bench/schema/result.v2.schema.json`; outcome from
   verify exit code, fairness from `fairness_contract.go`; metrics stay empty for Phase 79 (D-04).
6. **Seed `your_agent_full/MODE.md` only**; the resolver must be extensible so Phase 80 adds the
   other 5 modes without a rewrite (D-05).
</deferred>

---

*Phase: 77-bench-runtime-first-e2e-smoke*
*Context gathered: 2026-06-16*
