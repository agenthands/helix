# Polyglot-Edit Committed Baseline (aider_edit)

This is the **committed, byte-reproducible** polyglot-edit baseline (BASELINE-01).
It is produced by a **deterministic scripted agent** that applies the exercism
reference solution (`.meta/example.go`) to the solution stub through the helix
`replace_in_file` EDIT verb against the warm daemon, then grades with the dataset's
native `go test`. It carries **deterministic metrics only** — no live latency, no
timestamp, no absolute path, no live tokens. Regenerate locally with
`make bench-aider-edit`.

## Cell

| field | value |
| --- | --- |
| schema_version | v2 |
| benchmark | aider-polyglot |
| task_id | wordy |
| mode | aider_edit |
| language | go |

## Deterministic Metrics

| metric | value |
| --- | --- |
| edit_format_applied | true |
| outcome | success |
| task_success | true |
| verified_correctness | true |
