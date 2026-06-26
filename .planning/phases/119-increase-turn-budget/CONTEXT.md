# Phase 119: Increase Turn Budget - Context

**Gathered:** 2026-06-26
**Status:** Executing

<domain>
## Phase Boundary

Increase the agent's turn budget from 8 to 20 and re-measure the steering delta. This is a single-phase milestone.

**In scope:**
- BUDGET-01: Change default max_turns from 8 to 20
- BUDGET-02: Verify environment override still works
- BUDGET-03: Re-run attribution with new budget
- BUDGET-04: Update REPORT.md with honest verdict

**Out of scope:**
- Corpus filtering (Phase 120, conditional)
- SWE-bench scale-up (TUNE-FUT-05)
- Significance test (future enhancement)

</domain>

<decisions>
## Implementation Decisions

### D-01: max_turns=20 (user-selected)

The user selected 20 as the turn budget (recommended option). This gives the agent 2.5x more time than v2.5's 8 turns, which should be sufficient for simpler Exercism problems.

### D-02: Keep environment override

`AGENT_MAX_TURNS` environment variable still overrides the default. No code change needed — the default change automatically affects the attribution pipeline.

### D-03: Cost guard

Abort if total cost exceeds $2.00. The v2.5 run cost $0.20; with 2.5x more turns, we expect $0.50-1.00.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core Attribution Files
- `tools/dspy-tune/run_real.py:73` — attribution entry point (default max_turns)
- `tools/dspy-tune/optimize.py:255` — GEPA max_turns default
- `tools/dspy-tune/agent/react.py:74` — _DEFAULT_MAX_TURNS (12)

### Requirements and Roadmap
- `.planning/REQUIREMENTS.md` — BUDGET-01/02/03/04 requirements
- `.planning/ROADMAP.md` — Phase 119 success criteria

### Previous Results
- `tools/dspy-tune/output/attribution.json` — v2.5 results (delta=0.0000)
- `tools/dspy-tune/REPORT.md` — v2.5 verdict (NO-SHIP)

</canonical_refs>

<code_context>
## Existing Code Insights

### Attribution Pipeline (run_real.py)

```python
def attribution(max_turns=20):  # Changed from 8
    # Uses AGENT_MAX_TURNS env var if set
    mt = int(os.environ.get("AGENT_MAX_TURNS") or 20)
    return attribution(max_turns=mt)
```

### GEPA Runs (optimize.py)

```python
_gepa_max_turns = int(os.environ.get("AGENT_MAX_TURNS") or 20)  # Changed from 8
```

### Agent Default (react.py)

```python
_DEFAULT_MAX_TURNS = 12  # Unchanged — attribution overrides this
```

</code_context>

<specifics>
## Specific Ideas

- Change two defaults (run_real.py, optimize.py) from 8 to 20
- Re-run attribution
- Compare delta with v2.5
- Document honest verdict

</specifics>

<deferred>
## Deferred Ideas

None — this is a single-phase milestone.

</deferred>

---

*Phase: 119-Increase-Turn-Budget*
*Context gathered: 2026-06-26*