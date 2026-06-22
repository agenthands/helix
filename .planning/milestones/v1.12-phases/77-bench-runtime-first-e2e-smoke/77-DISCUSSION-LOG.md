# Phase 77: Bench Runtime & First E2E Smoke - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-16
**Phase:** 77-bench-runtime-first-e2e-smoke
**Areas discussed:** Agent driver for the smoke, Smoke task source, Mode→profile mapping, Daemon transport & cell isolation

---

## Agent driver for the smoke

### Q1 — What drives the agent leg for the first E2E smoke AND `make bench-quick` in CI?

| Option | Description | Selected |
|--------|-------------|----------|
| Scripted gates CI; claude wired | bench-quick / first smoke uses the in-process scripted-agent (deterministic, no API key, hermetic, ≤30s/≤90s) as the CI gate; real `claude` CLI subprocess path fully wired and locally runnable but NOT gating CI. | ✓ |
| Real claude CLI for the smoke | First smoke spawns real `claude` CLI; genuine agent-tap but CI needs API-key secret, network, slower/flakier vs ≤90s. | |
| Scripted only this phase | Only the scripted agent; defer ALL real-claude wiring to a later phase; leaves criterion #3 subprocess shape unproven. | |

**User's choice:** Scripted gates CI; claude wired
**Notes:** Matches eval-quick's scripted/real split; criterion #3's "(eventually) claude CLI" wording supports deferring the real-claude gate.

### Q2 — How is the agent-tap (CCTapResult) leg produced so criterion #4's merged trace exists in hermetic CI?

| Option | Description | Selected |
|--------|-------------|----------|
| Synthesize CCTap from scripted | Scripted agent emits a CCTapResult-shaped agent-tap from its executed tool-call sequence; bench-quick produces+asserts a real 2-leg merged trace every PR (0 orphans, daemon-tap PID-gated). Real claude yields the same shape from its transcript. | ✓ |
| Criterion #4 = real-claude run | CI scripted smoke asserts daemon-tap only; full daemon+agent-tap merged trace graded against a real-claude run local/nightly. | |

**User's choice:** Synthesize CCTap from scripted
**Notes:** Makes criterion #4 a CI gate rather than a manual artifact; exercises `trace.Merge` zero-orphan / PID-gating invariants every PR without an API key.

---

## Smoke task source

### Q1 — Where does Phase 77's single Go smoke task come from, given the real ToolBench corpus is Phase 78?

| Option | Description | Selected |
|--------|-------------|----------|
| One real fixture under toolbench-go | Ship exactly ONE minimal-but-real Go task at `bench/datasets/toolbench-go/<task>/` (tiny module + prompt + `go test` verify); scripted agent replays a real MCP edit. Seed dir Phase 78 grows. | ✓ |
| Reuse an internal/eval corpus task | Point the smoke at an existing eval corpus task; fastest but conflicts with `toolbench-go` naming and blurs eval↔bench separation. | |
| Trivial no-grade stub | A stub task checking only that the daemon answered; under-proves the runtime (no verify/scoring). | |

**User's choice:** One real fixture under toolbench-go
**Notes:** Honors criterion #1's `--benchmarks=toolbench-go` naming and respects P75 D-15/INFRA-03 eval↔bench separation. Scope-guarded to ONE task (corpus is Phase 78).

### Q2 — How much of result.v2.json does Phase 77 populate (evaluators are Phase 79)?

| Option | Description | Selected |
|--------|-------------|----------|
| Outcome + provenance, schema-valid | pass/fail outcome from verify.sh exit (reuse merge.go VerifyExitCode) + fairness + token/cost + trace ref + schema_version; rich metrics null/absent for Phase 79. Validates against result.v2.schema.json today. | ✓ |
| Minimal: just enough to validate | Only schema_version + outcome; defer fairness/token/trace; risks rework for P82 aggregation. | |
| Attempt richer metrics now | Compute files_modified/wall_time now; pulls Phase 79 evaluator scope forward. | |

**User's choice:** Outcome + provenance, schema-valid
**Notes:** Provenance-complete but metric-sparse; keeps the evaluator/metrics contract owned by Phase 79.

---

## Mode→profile mapping

### Q1 — How does the runner resolve `--modes=your_agent_full` to `--profile=bench-full`, given ABLATE-01's full 6-mode convention is Phase 80?

| Option | Description | Selected |
|--------|-------------|----------|
| Seed one MODE.md + resolver | Create `bench/runners/your_agent_full/MODE.md` (frontmatter `profile: bench-full`) + a small resolver reading it; Phase 80 adds the other 5 mode dirs and extends the resolver (grows, not refactors). | ✓ |
| Tiny hardcoded map, defer convention | In-code map (your_agent_full→bench-full) for P77; defer the MODE.md convention to Phase 80. Smallest surface but likely replaced. | |
| Rename P76 profiles to mode names | Rename bench-full.yaml→your_agent_full.yaml so mode==profile; churns P76 locked filenames + golden tests; baseline_plain/rag still unprofiled. | |

**User's choice:** Seed one MODE.md + resolver
**Notes:** Forward-compatible with ABLATE-01's stated location (`bench/runners/<mode>/MODE.md`). `StartDaemon(ctx,taskID,mode,profileName,cfgPath)` already separates mode label from profile name — the seam already exists.

---

## Daemon transport & cell isolation

### Q1 — What transport does each per-cell daemon use, and how thin is bench/runtime/sandbox over internal/eval/sandbox?

| Option | Description | Selected |
|--------|-------------|----------|
| Unix socket, embed eval sandbox | Per-cell Unix domain socket, `--http-addr=` empty (no TCP), agent via `helix --mode=stdio --socket=` forwarder; "no port collisions on --parallel=4" by construction. bench/runtime/sandbox EMBEDS eval's Sandbox. | ✓ |
| TCP ephemeral port per cell | Each daemon binds :0 and reads back the port; reintroduces the port-collision class and diverges from the proven eval path. | |
| Copy eval sandbox into bench | Fork eval sandbox code into bench; violates reuse-don't-fork (P75 D-08/D-15) and the "thin-wraps" wording. | |

**User's choice:** Unix socket, embed eval sandbox
**Notes:** Reuses eval's `StartDaemon` path verbatim (`--socket=… --http-addr=`); no TCP ports exist to collide.

### Q2 — How are per-cell artifacts laid out (PID-gated JSONL, result, trace) and what's the cleanup policy?

| Option | Description | Selected |
|--------|-------------|----------|
| Durable run dir + ephemeral scratch | Cell key `<run_id>/<task>/<mode>`; durable artifacts under `bench/reports/<run_id>/<task>/<mode>/` (--out overridable); ephemeral scratch (repo/home/socket) in OS temp, deleted on success, PRESERVED on failure; daemon-tap PID-gated via TapDaemonLog. | ✓ |
| Everything under one sandbox root | Scratch + results under one eval-style root, copy result/merged out at the end; extra copy-out plumbing. | |
| All under reports, keep everything | Scratch + results together under bench/reports/<run_id>/, clean nothing; pollutes reports, grows unbounded. | |

**User's choice:** Durable run dir + ephemeral scratch
**Notes:** `run_id` reuses eval's ISO-8601+gitSHA shape; keep-on-failure aids debugging the smoke.

---

## Claude's Discretion

- `MODE.md` frontmatter schema beyond `mode` + `profile` keys (Phase 80 owns the full convention).
- `run_id` exact string format (reuse eval's ISO-8601+gitSHA).
- Exact `make bench` / `bench-quick` / `bench-<suite>` target wording and default args (semantics fixed; wording open).
- Whether `bench/reports/<run_id>/` output is git-ignored.
- The `scripted_agent.yaml` step sequence for the seed task and the exact MCP edit tool used.
- Internal package layout of `bench/runtime/` and the `helix-bench run` flag plumbing.

## Deferred Ideas

- Real `claude` CLI as a CI gate (needs API-key secret; local/nightly only this phase).
- Full ToolBench Go corpus + capability matrix (Phase 78).
- Rich result metrics — edit_locality, regression_rate, pass@k, multi-run aggregation (Phases 79/82).
- Remaining 5 modes + full `bench/runners/<mode>/MODE.md` convention (Phase 80, ABLATE-01).
- Container runtime / per-language runners / GHCR mirror (Phases 84–88).
