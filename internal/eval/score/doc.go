// Package score implements the heuristic tool-behavior scorer for Phase 67.
// It loads a task's expected_tools.yaml rule file (the DSL defined in
// 67-RESEARCH.md §"Heuristic Rule DSL"), applies expect_sequence, expect_set,
// forbid_sequence, and forbid_set matchers against the merged trace.json, and
// emits per-task +1/-1 deltas into tool_behavior.json. The heuristic scorer is
// CI-actionable and never LLM-based per D-06 / EVAL-05. Real implementation
// lands in Wave 2+.
package score
