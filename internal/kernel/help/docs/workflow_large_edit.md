Canonical Helix workflow for large fuzzy edits.

# Definition of Done — Large Edit

**Satisfies:** G-004 (large fuzzy edit without context)

A "large edit" is any edit where `changed_lines (inserted + deleted) > 50` OR `touched_files > 1`. The secondary trigger `changed_lines / file_line_count > 0.30` is also active by default.

## Checklist

- [ ] **Step 1** — For single-file large edits: call `get_context` on the file being edited. For multi-file or module-wide edits: call `get_repo_map` on the relevant directory.
- [ ] **Step 2** — Confirm the response includes a `_receipt_id`.
- [ ] **Step 3** — Issue the destructive call with `receipts: ["<context_or_overview_receipt_id>"]`.
- [ ] **Step 4** — Call `verify_edit` after completion.

## Tool Sequence (single-file large edit)

```json
{ "method": "tools/call", "params": { "name": "get_context", "arguments": { "file_path": "src/parser/grammar.go", "max_tokens": 4000 } } }
// Response includes: { "_receipt_id": "rcpt_FF0006GGGGGGGGG000FFFF0000" }

{ "method": "tools/call", "params": { "name": "replace_in_file", "arguments": { "file_path": "src/parser/grammar.go", "find": "...", "replace": "...", "receipts": ["rcpt_FF0006GGGGGGGGG000FFFF0000"] } } }

{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/parser/grammar.go" } } }
```

## Tool Sequence (multi-file / module-wide edit)

```json
{ "method": "tools/call", "params": { "name": "get_repo_map", "arguments": { "path": "src/parser/", "max_tokens": 8000 } } }
// Response includes: { "_receipt_id": "rcpt_GG0007HHHHHHHHH000GGGG0000" }

{ "method": "tools/call", "params": { "name": "fuzzy_edit", "arguments": { "file_path": "src/parser/grammar.go", "find": "...", "replace": "...", "receipts": ["rcpt_GG0007HHHHHHHHH000GGGG0000"] } } }

{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/parser/" } } }
```

## Notes

- `get_context` issues a `context_gathered` receipt; `get_repo_map` issues a `structural_overview` receipt.
- For very broad rewrites spanning many files AND many lines, include both a `structural_overview` receipt and a `context_gathered` receipt.
- The G-004 thresholds are configurable via `guardrails.G-004.max_changed_lines` and `guardrails.G-004.max_files`.
