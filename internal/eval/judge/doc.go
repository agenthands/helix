// Package judge runs the informational LLM judge pass for Phase 67 evaluation.
// It sends each (task, mode) trace to the Anthropic API (default: claude-sonnet-4-6)
// with the family-specific rubric from eval/judge/prompts/rubric.md and records
// per-axis scores into tool_behavior_judge.json. The judge is NEVER a CI gate
// per EVAL-07: a judge failure does not affect make eval's exit code. Real
// implementation (including --no-judge fallback) lands in Wave 3+.
package judge
