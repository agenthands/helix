# Task: Update Public API Return Type (Python)

**Task ID:** quick-public-api-py-001
**Family:** public_api
**Language:** Python (best-effort tree-sitter tier)

## Prompt

The function `compute` currently returns `int`.
Change its return type annotation to `tuple[int, str]` so callers receive
both the result and a status message. Check the blast radius before making the change.

## Intent

Validates that the agent checks `analyze_blast_radius` before editing a
public function, then uses `replace_symbol_body` to make the change — rather
than blindly editing with text replacement.

## Harness Note

This fixture is used by `make eval-quick` (in-process scripted agent).
The scripted_agent.yaml hard-codes the expected tool-call sequence.
This is harness-wiring validation only — not a measurement of real agent behavior.
See eval/EVAL.md (Pitfall 6).
