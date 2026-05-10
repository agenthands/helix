# Task: Delete Unused Symbol

**Task ID:** quick-delete-001
**Family:** delete
**Language:** Go (first-class LSP tier)

## Prompt

The function `unusedHelper` in `main.go` is no longer referenced anywhere.
Delete it safely using safe_delete_symbol after verifying there are no callers.

## Intent

Validates that the agent uses symbolic safe delete (safe_delete_symbol) after
finding references (find_references confirms zero callers), rather than manually
editing the file with text search.

## Harness Note

This fixture is used by `make eval-quick` (in-process scripted agent).
The scripted_agent.yaml hard-codes the expected tool-call sequence.
This is harness-wiring validation only — not a measurement of real agent behavior.
See eval/EVAL.md (Pitfall 6).
