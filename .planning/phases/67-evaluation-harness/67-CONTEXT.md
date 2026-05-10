# Phase 67: Evaluation Harness — Context

**Gathered:** 2026-05-10
**Status:** Ready for research / planning

<domain>
## Phase Boundary

Phase 67 ships an **out-of-process evaluation harness** that proves Helix
improves agent success / cost / safety by running the same task corpus under
four modes and emitting per-task evidence — without leaking proprietary code
into commercial LLM training pipelines.

**Phase 67 ships:**

1. **`eval/` top-level workspace** — corpus (per-task directories,
   SWE-bench-style), generators, prices/budgets config, fixture seeds.
2. **`cmd/helix-eval/`** — runner binary that, per task × per mode, spawns
   an isolated daemon subprocess, launches a `claude` CLI subprocess as the
   agent, captures merged trace evidence, and writes per-task artifacts.
3. **New `baseline` profile** (`internal/profile/profiles/baseline.yaml`) —
   strips Helix tools entirely so baseline mode runs the agent against a
   daemon that exposes nothing beyond the bare MCP server (control case).
4. **Four mode wrappers** — `baseline / native / semantic /
   semantic_guarded`. Each mode = one daemon subprocess invocation with a
   distinct `--profile` and config layer overrides; agent connects via
   stdio forwarder per EVAL-02.
5. **`make eval`** (full corpus, subprocess) and **`make eval-quick`**
   (small fixture suite, in-process) targets.
6. **Reports** — `eval_report.json`, `eval_report.md`, `cost_summary.json`,
   `tool_behavior.json`, `tool_behavior_judge.json` (informational),
   `safety_compliance.json`, plus per-task traces and patches.
7. **Heuristic tool-behavior scorer** + **informational LLM judge** running
   on the same trace. Heuristic is CI-actionable; judge never gates merges.
8. **Provider TOS attestation** in `EVAL.md` — retention-zero verified for
   every commercial provider used; recorded at planning time per EVAL-06.

**Out of scope (deferred):**

- Dollar-cost conversion / static price table (Phase 68+ or backlog) —
  Phase 67 reports tokens only.
- Pluggable agent runners other than `claude` CLI subprocess (no
  Anthropic-SDK-direct, OpenAI Agents SDK, Codex, Gemini CLI in this
  phase). Future: behind a `--agent=` flag if needed.
- LLM judge as CI gate (per EVAL-07, judge stays informational).
- SWE-bench / public benchmark integration — synthetic corpus only at v1.10.
- Cross-version reproducibility infra beyond recording `claude --version`.
</domain>

<canonical_refs>
## Canonical References

Every downstream agent (researcher, planner) MUST read these:

- `.planning/REQUIREMENTS.md` — EVAL-01..EVAL-07 (locked acceptance)
- `.planning/milestones/v1.10-ROADMAP.md` — Phase 67 goal and success criteria
- `.planning/PROJECT.md` — overall product framing
- `.planning/phases/65-existing-tool-integration-strangler-fig/65-CONTEXT.md`
  — strangler-fig source-field semantics; eval baseline must observe these
- `.planning/phases/66-agent-guardrails/66-CONTEXT.md` — receipt store,
  guardrail telemetry classes (`guardrail_warned/_blocked`); eval
  `safety_compliance.json` reports against these.
- `internal/profile/` — existing 5 profiles + loader; baseline.yaml lands here.
- `internal/forwarder/` — stdio→gRPC plumbing reused by subprocess agent.
- `internal/mcp/middleware.go` — TelemetryMiddleware; daemon-side trace source of truth.
- `internal/daemon/daemon.go` — bootstrap reference for eval-quick in-process path.
- `.planning/codebase/TESTING.md` — overall testing patterns (mostly Python-legacy;
  Go test patterns inferred from existing `_test.go` files).

No prior `EVAL.md` exists — Phase 67 creates it.
</canonical_refs>

<prior_decisions>
## Carrying Forward From Earlier Phases

- **5 existing profiles** (`ci-bot, claude-code, codex, full, ide-assistant`)
  — `baseline` is profile #6, lands as `internal/profile/profiles/baseline.yaml`.
- **TelemetryMiddleware outcome taxonomy** (Phase 9 + Phase 66 extension):
  `success / timeout / circuit_open / internal / guardrail_warned /
  guardrail_blocked`. Eval rides on this taxonomy directly — no new outcome
  classes added.
- **Receipt store** (Phase 66) — `safety_compliance.json` reports
  receipt-issuance and guardrail-block counts pulled from the receipt
  store and Phase 66 metrics.
- **Stdio forwarder** (`internal/forwarder/`) — reused as the agent⇄daemon
  transport per EVAL-02; no new transport.
- **PhaseGraph** (Phase 57) — eval pipeline declares its DAG via
  `internal/phasegraph/`; the existing eval pipeline shape introduced in
  57 is the starting point.
</prior_decisions>

<decisions>
## Implementation Decisions From This Discussion

### D-01 — Agent runner: Claude Code CLI subprocess (LOCKED)

The harness shells out to `claude --print --mcp-config=<mode>.json` (or
equivalent) per task per mode. Closest-to-real-world client, smallest
custom code surface, but accepts non-determinism cost.

**Why:** Claude Code is Helix's primary client; eval results should reflect
actual usage. A custom in-house Go agent would diverge from real behavior.

**How to apply:** No `cmd/eval-agent/` — the agent is `claude`. The
runner's job is process orchestration + trace capture, not tool-calling
loop logic. Future agents (Codex, Gemini CLI) can be added behind
`--agent=` later; not in this phase.

### D-02 — Trace capture: daemon telemetry + CC json merged (LOCKED)

**Daemon-side telemetry is source of truth** for tool calls, tokens, and
guardrail outcomes. **Claude Code `--output-format=json`** is parsed in
parallel for assistant messages / reasoning / final patch. Reports merge
both streams into the per-task evidence.

**Why:** Daemon trace is stable across CC versions; CC json adds reasoning
context that daemon can't see. Baseline mode (Helix tools stripped) still
gets daemon-side stdio-forwarder logs for tool-call accounting (which will
be empty/minimal for baseline — that's the point).

**How to apply:** Per-task artifact directory contains
`trace_daemon.jsonl` + `trace_cc.json` + merged `trace.json`. Schema
reconciliation lives in eval reporter.

### D-03 — CC version handling: record, don't pin (LOCKED)

Each `eval_report.json` records `claude --version` output. No hard
version gate; cross-run comparisons must filter by version.

**Why:** Pinning a CLI binary in CI is high-friction; recording the
version preserves comparability without blocking upgrades.

**How to apply:** Reporter calls `claude --version` once per run and
embeds the string in report metadata.

### D-04 — Corpus format: per-task directory + programmatic generators (LOCKED)

Canonical task shape is a directory:

```
eval/corpus/<task-id>/
  task.md            # prompt + intent
  repo/              # seeded repo fixture
  verify.sh          # exit 0 = pass; assertions on diff/tests/grep
  expected_tools.yaml # heuristic tool-behavior patterns
  budget.yaml        # optional per-task overrides for D-07 caps
```

Programmatic generators (`eval/gen/*.go`) emit into the same format for
combinatorial coverage of rename / delete / public-API / etc.

**Why:** SWE-bench-shaped per-task dirs are flexible (any verify
strategy: tests, grep, diff). Generators amortize hand-authoring while
keeping the canonical format hand-readable.

**How to apply:** Hand-author seed corpus in Phase 67; generators ship
as scaffolding. Don't generate the entire corpus from generators in
this phase — small hand-authored seed proves the heuristic scorer first.

### D-05 — Corpus size: quick 10 / full 50-100 (LOCKED)

`make eval-quick`: ~10 in-process tasks, target <30s wall-time, in CI on
every PR.
`make eval`: 50-100 subprocess tasks, target single-digit minutes per
mode (so ~30 min total across 4 modes), nightly or pre-release CI.

**Why:** 10 quick tasks gives enough signal for fast feedback; 50-100
full gives statistical power without becoming a multi-hour run.

**How to apply:** Planner sizes the seed corpus accordingly. Generators
can grow the full corpus toward 100 in later phases without re-discussing.

### D-06 — Tool-behavior scoring: heuristic primary + LLM informational (LOCKED)

Heuristic scorer reads `expected_tools.yaml` (sequences/sets of tool
calls) and the merged trace, emits `tool_behavior.json` with +1/-1
deltas per task. Examples:
- `+1 rename_symbol after find_references`
- `-1 grep+manual_replace_in_file for rename`
- `+1 safe_delete_symbol with non-empty receipts after find_references`
- `-1 delete_file on a symbol-bearing file`

LLM judge runs on the same trace and writes `tool_behavior_judge.json` —
**informational only, never gates CI** per EVAL-07.

**Why:** Heuristics are deterministic, debuggable, CI-safe. LLM judge
adds qualitative depth and catches novel patterns but is too flaky to
gate merges.

**How to apply:** Planner scopes the rule schema and a starter rule set
covering the rename / delete / public-API task families EVAL-05 calls
out. Judge model selection is a planner-level detail (default: Sonnet).

### D-07 — Cost tracking: tokens only this phase (LOCKED)

`cost_summary.json` reports input/output token totals per task per
mode. **No dollar conversion this phase.** Tokens come from in-band
Anthropic API response usage fields surfaced via CC json.

**Why:** Static price table goes stale and doesn't add information that
tokens don't already carry. Dollar reporting is a downstream concern;
Phase 67 ships the measurement, not the pricing.

**How to apply:** Reporter sums `usage.input_tokens` /
`usage.output_tokens` from `trace_cc.json` per task. No prices.yaml.

### D-08 — Budget caps: hard, three-axis (LOCKED)

Each task can declare in `budget.yaml`:
- `max_input_tokens` (default ~200k)
- `max_output_tokens` (default ~32k)
- `max_seconds` (default 300)
- `max_tool_calls` (default 100)

Harness aborts the task on any breach and marks it
`failed-with-cause: budget_<axis>`. Breach data is recorded as a
first-class outcome, not lost.

**Why:** A pathological loop can burn $hundreds before a wall-time
timeout; counting tool calls catches loop-on-tool patterns. Hard caps
protect CI cost and surface bad behavior as data instead of incidents.

**How to apply:** Defaults set in eval config; per-task `budget.yaml`
overrides. Reporter aggregates `failed-with-cause: budget_*` counts
into `eval_report.md`.
</decisions>

<deferred>
## Deferred Ideas (out of scope this phase)

- **Pluggable agent runners** (Anthropic SDK direct, OpenAI Agents SDK,
  Codex, Gemini CLI) behind `--agent=` flag. Capture as backlog item if
  motivated by user demand.
- **Static price table + `cost_summary.json` $ conversion.** Defer to
  Phase 68+ or backlog; tokens-only is sufficient to prove improvement.
- **LLM judge as gating signal** — explicitly forbidden by EVAL-07.
  Stays informational permanently unless requirement changes.
- **SWE-bench / public benchmark integration** — synthetic-only at v1.10
  per EVAL-06. Public-corpus runs would require separate retention review.
- **Provider billing API reconciliation** — would give ground-truth $;
  not worth the complexity until tokens-only reporting reveals a gap.
- **Cross-version CC comparison infra** — beyond recording `--version` in
  the report, no special tooling for "compare Phase 67 results across CC
  versions". Add later if drift becomes a real problem.
- **Generator-driven full corpus** — Phase 67 ships hand-authored seed +
  generator scaffold. Scaling to 100% generator-emitted corpus is later work.
</deferred>

<open_questions>
## For Researcher / Planner to Resolve

These are technical/research questions the user does not need to decide:

- **Languages in seed corpus** — Go, TypeScript, Python at minimum?
  Researcher proposes coverage based on Helix's first-class LSP tier
  (Java/Go/Rust/TS/JS).
- **Subprocess isolation mechanics** — separate `HOME` per mode? Temp
  config dir per mode? Researcher inspects `internal/config/` precedence
  and writes the cleanest isolation strategy.
- **`baseline` profile contents** — empty tools list vs. only-non-Helix
  tools vs. MCP server with zero registrations. Researcher aligns with
  `internal/profile/loader.go` semantics.
- **In-process `eval-quick` plumbing** — how does it skip the subprocess
  spawn while still exercising the same modes? Probably reuses
  `internal/daemon/` Server directly with profile overlays in-memory.
- **CC `--mcp-config` format** — confirm current schema for stdio MCP
  servers; document any version-dependence.
- **Trace merge schema** — define `trace.json` shape in PLAN.md;
  reconcile event ordering between daemon telemetry (gRPC-side) and CC
  json (CLI-side). Wall-clock alignment likely sufficient.
- **Heuristic rule schema** — DSL design for `expected_tools.yaml`
  patterns (sequence vs. set vs. regex on tool name + args).
- **LLM judge model + prompt** — Sonnet 4.6 default? Prompt template
  lives in `eval/judge/prompts/`; rubric per EVAL-05 task families.
- **Provider retention TOS evidence format** — what does the
  attestation block in `EVAL.md` look like (link + retrieval date +
  setting verified)? Per EVAL-06.
- **PhaseGraph integration** — eval pipeline DAG declaration: where
  exactly in `internal/phasegraph/`?
- **Top-level `eval/` vs `internal/eval/`** — placement: top-level for
  user-visible artifacts (corpus, reports), `internal/eval/` for runner
  internals? Researcher recommends.
</open_questions>

<acceptance_criteria>
## Phase Done When (from REQUIREMENTS.md, locked)

- [ ] **EVAL-01:** `EvalResult` per task captures success, patch applies,
      tests pass, diagnostics clean, duration, tokens, edit count,
      guardrail compliance, context precision/recall.
- [ ] **EVAL-02:** Each mode runs in out-of-process subprocess with
      isolated config dir; `baseline` mode uses `--profile=baseline` to
      strip Helix tools; agent connects via stdio forwarder.
- [ ] **EVAL-03:** `make eval-quick` runs an in-process small fixture
      suite without subprocess overhead.
- [ ] **EVAL-04:** Reports emitted: `eval_report.json`, `eval_report.md`,
      `cost_summary.json`, `tool_behavior.json`, `safety_compliance.json`,
      plus per-task traces and patches.
- [ ] **EVAL-05:** Tool-behavior scoring records right-tool usage for
      rename / delete / public-API tasks; +1/-1 heuristic per pattern.
- [ ] **EVAL-06:** Synthetic-only corpus by default (Helix OSS as
      permissible secondary source); retention-zero on commercial
      providers where supported; TOS attestation in `EVAL.md`.
- [ ] **EVAL-07:** Cross-model judging informational; flaky judge does
      not block merges.

Any of these failing = phase not done; planner derives plans to deliver
all seven.
</acceptance_criteria>
