// Package report writes the five EVAL-04 report artifacts for each Phase 67
// evaluation run: eval_report.json, eval_report.md, cost_summary.json,
// tool_behavior.json, and safety_compliance.json. It reads per-task result.json
// files from the run output directory and aggregates them into the run-level
// reports under eval/reports/<run-id>/. Real implementation lands in Wave 3+.
package report
