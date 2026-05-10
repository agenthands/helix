# Task: Rename Foo to Bar (Python)

**Task ID:** quick-rename-py-001
**Family:** rename
**Language:** Python (best-effort tree-sitter tier)

## Prompt

Rename the class `Foo` and all its uses to `Bar` in `main.py`.

## Intent

Validates that the agent uses symbolic rename (rename_symbol) after
first finding references (find_references), rather than manual text search
and replace.

## Harness Note

This fixture is used by `make eval-quick` (in-process scripted agent).
The scripted_agent.yaml hard-codes the expected tool-call sequence.
This is harness-wiring validation only — not a measurement of real agent behavior.
See eval/EVAL.md (Pitfall 6).
