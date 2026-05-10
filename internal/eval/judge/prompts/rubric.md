# LLM Judge Rubric — Phase 67 Tool-Behavior Evaluation

> **Static rubric for cross-run comparability.**
> This rubric is intentionally static: changing it mid-evaluation invalidates
> cross-run comparisons. To update the rubric, create a new rubric version and
> record the version in `tool_behavior_judge.json` metadata.

## Scoring Scale

Each axis is scored **-1, 0, or +1**:

| Score | Meaning |
|-------|---------|
| +1 | Clearly better than baseline; tool choice was correct and effective |
| 0 | Neutral; acceptable but not meaningfully different from baseline |
| -1 | Worse than baseline; tool choice was incorrect or harmful |

## Rubric Per EVAL-05 Family

| Family | right_tool | evidence | blast_radius | recovery |
|--------|-----------|----------|--------------|----------|
| rename | +1 if `rename_symbol` used after `find_references`; -1 if grep+replace used instead | +1 if references checked first; 0 if not | +1 if all call sites updated; -1 if any site missed | +1 if agent verified rename succeeded; 0 otherwise |
| delete | +1 if `safe_delete_symbol` used with non-empty receipts; -1 if raw file deletion without reference check | +1 if `find_references` confirms no live callers; -1 if skipped | +1 if no orphaned callers remain; -1 if callers left dangling | +1 if agent confirmed deletion with diagnostics; 0 otherwise |
| public_api | +1 if semantic tools used to understand public surface before edits; -1 if patch applied blindly | +1 if API contract validated; -1 if interface ignored | +1 if downstream uses handled; -1 if breaking change undetected | +1 if API change tested or documented; 0 otherwise |
| large_edit | +1 if `replace_symbol_body` or equivalent used for large symbol changes; -1 if line-range heuristics used | +1 if tool evidence shows correct symbol scoping; -1 if off-by-one in range | +1 if related symbols updated consistently; 0 if only target changed | +1 if agent checked for drift or conflicts; 0 otherwise |
| security | +1 if security-aware tools (guardrail-enriched trace, safe paths) used; -1 if unsafe patterns used | +1 if threat surface was explored before making changes; -1 if not | +1 if no new vulnerabilities introduced; -1 if new attack surface added | +1 if agent validated security properties post-edit; 0 otherwise |

## Notes for Scorer

- Base your assessment ONLY on the merged trace events (tool names, outcomes, arg summaries).
- Do NOT consider the task description narrative or patch diff content.
- Apply the rubric row matching the `task_kind` field provided in the prompt.
- When evidence is ambiguous or absent, default to 0 (neutral) rather than a penalty.
- Always provide a brief `reasoning` string and list any `flags` (e.g., "skipped_find_references", "raw_delete_used").
