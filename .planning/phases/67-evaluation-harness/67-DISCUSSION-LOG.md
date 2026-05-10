# Phase 67: Evaluation Harness — Discussion Log

**Date:** 2026-05-10
**Mode:** discuss (default)

## Areas Selected

- Agent runner choice
- Task corpus shape
- Tool-behavior scoring
- Cost & token tracking

## Q&A Trail

### Area 1: Agent runner

**Q: Which agent drives the daemon under test?**
Options: minimal in-house Go agent / Claude Code CLI subprocess / both / pluggable interface
**A: Claude Code CLI subprocess** — closest-to-real-world; primary client; smallest custom code surface.

**Q: How to capture per-task evidence?**
Options: daemon-side only / CC --output-format=json / both merged / defer
**A: Both — daemon trace + CC json merged.** Daemon = source of truth for tool calls/tokens; CC json adds reasoning/messages.

**Q: How to pin Claude Code version?**
Options: pin in EVAL.md / vendor in CI / record version, no pin
**A: Don't pin — record version in report.** Cross-run comparisons filter by version.

### Area 2: Task corpus shape

**Q: What format does a synthetic eval task take?**
Options: hand-authored YAML / per-task directory (SWE-bench-style) / programmatic generators / YAML + generators later
**A: Per-task directory + programmatic generators.** Canonical = directory; generators emit into the same shape for combinatorial coverage.

**Q: Corpus size for quick vs full?**
Options: 5/20-30 / 10/50-100 / 3/10-15
**A: Quick 10 / Full 50-100.** Quick <30s in-process; full ~30 min across 4 modes nightly.

### Area 3: Tool-behavior scoring

**Q: How to judge "right tool used"?**
Options: heuristic rules / LLM judge / heuristic primary + LLM informational / heuristic only, defer judge
**A: Heuristic primary + LLM informational.** `tool_behavior.json` (CI-actionable) + `tool_behavior_judge.json` (informational, never gates merges).

### Area 4: Cost & token tracking

**Q: Token + dollar cost source?**
Options: in-band + price table / billing API / hybrid
**A: Tokens only this phase — no $ conversion.** Token counts from in-band Anthropic API usage fields via CC json. Dollar pricing deferred.

**Q: Per-task budget cap to prevent runaway?**
Options: hard caps (token+wall+tool-count) / wall+token only / soft warnings
**A: Hard caps on tokens + wall-time + tool-call count.** Sane defaults, per-task overrides via `budget.yaml`. Breach = `failed-with-cause: budget_<axis>`.

## Scope Boundary Held

No scope creep raised by user. All four discussion areas stayed within EVAL-01..EVAL-07 boundary. Deferred items captured in CONTEXT.md `<deferred>`.

## Next Step

`/gsd-plan-phase 67` — researcher reads CONTEXT.md (and the canonical refs it lists) to investigate the open_questions; planner produces PLAN.md from research + locked decisions.
