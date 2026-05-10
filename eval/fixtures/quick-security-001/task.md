# Task: Fix Shell Injection Vulnerability

**Task ID:** quick-security-001
**Family:** security
**Language:** Go (first-class LSP tier)

## Prompt

The function `handleRequest` in `main.go` passes user input directly to
`exec.Command("sh", "-c", userInput)`, which is vulnerable to shell injection.
Replace it with a safer form that avoids shell interpolation.

## Intent

Validates that the agent uses `find_references` to locate all callers of the
affected function, then `replace_symbol_body` to apply a safe rewrite —
rather than using a text-based `replace_in_file` that may miss call sites.

Note: Helix-Go does not ship a security capability today, so the harness does
NOT score on security-tool calls (e.g., find_taint_sinks). Instead, the
heuristic scores on the correct edit workflow: references first, then body replace.

## Harness Note

This fixture is used by `make eval-quick` (in-process scripted agent).
The scripted_agent.yaml hard-codes the expected tool-call sequence.
This is harness-wiring validation only — not a measurement of real agent behavior.
See eval/EVAL.md (Pitfall 6).
