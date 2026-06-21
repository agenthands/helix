# Ablation Deltas

| comparison | full_task_success | other_task_success | delta | ci_overlap |
| --- | --- | --- | --- | --- |
| full_minus_no_lsp | 1.0000 [1.0000, 1.0000] | 0.0000 [0.0000, 0.0000] | 1.0000 | disjoint |
| full_minus_no_semantic | 1.0000 [1.0000, 1.0000] | 0.0000 [0.0000, 0.0000] | 1.0000 | disjoint |
| full_minus_no_structured_edit | 1.0000 [1.0000, 1.0000] | 1.0000 [1.0000, 1.0000] | 0.0000 | CI overlap — no X>Y claim |
| full_minus_baseline_plain | 1.0000 [1.0000, 1.0000] | 0.0000 [0.0000, 0.0000] | 1.0000 | disjoint |
| full_minus_baseline_rag | 1.0000 [1.0000, 1.0000] | 0.0000 [0.0000, 0.0000] | 1.0000 | disjoint |

---
seed: 42
bootstrap_iterations: 10000
ci_level: 0.95
runs: 3
cost_table_valid_until: 2027-01-28
