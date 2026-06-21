# Cost vs Quality

| mode | benchmark | cost_per_solved_task |
| --- | --- | --- |
| full | internal-toolbench | 0.0045 [0.0045, 0.0045] |
| no_lsp | internal-toolbench | 0.0150 [0.0150, 0.0150] |

## FAIR-03 between-run variance warnings (CV > 0.05)

- task-3 / no_lsp: per-run USD coefficient of variation 0.6000 exceeds 0.05

## Cost vs verified_correctness (scatter)

```
y = verified_correctness (1.0 top .. 0.0 bottom), x = cost_per_solved (min left .. max right)
(no plottable points: every point has a null cost or verified_correctness)
```

---
seed: 42
bootstrap_iterations: 10000
ci_level: 0.95
runs: 3
cost_table_valid_until: 2027-01-28
