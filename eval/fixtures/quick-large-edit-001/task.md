# Task: Rewrite Large Function Body

**Task ID:** quick-large-edit-001
**Family:** large_edit
**Language:** Go (first-class LSP tier)

## Prompt

The function `Process` in `main.go` has a verbose implementation that computes
a result through several redundant intermediate steps.
Rewrite its body to be a single, direct computation: `return x * 2`.

## Intent

Validates that the agent uses `replace_symbol_body` (or `fuzzy_edit`) to
replace a large function body — rather than a regex-based `replace_in_file`
that treats the function as a text blob.

## Harness Note

This fixture is used by `make eval-quick` (in-process scripted agent).
The scripted_agent.yaml hard-codes the expected tool-call sequence.
This is harness-wiring validation only — not a measurement of real agent behavior.
See eval/EVAL.md (Pitfall 6).
