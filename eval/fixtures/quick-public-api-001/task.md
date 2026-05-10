# Task: Update Public API Return Type

**Task ID:** quick-public-api-001
**Family:** public_api
**Language:** Go (first-class LSP tier)

## Prompt

The method `Greeter.Hello` currently returns `string`.
Change its return type to `(string, error)` so callers can handle errors.
Check the blast radius before making the change.

## Intent

Validates that the agent checks `analyze_blast_radius` before editing a
public method, then uses `replace_symbol_body` to make the change — rather
than blindly editing with text replacement.

## Harness Note

This fixture is used by `make eval-quick` (in-process scripted agent).
The scripted_agent.yaml hard-codes the expected tool-call sequence.
This is harness-wiring validation only — not a measurement of real agent behavior.
See eval/EVAL.md (Pitfall 6).
