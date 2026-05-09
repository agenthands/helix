Canonical Helix workflow for symbol rename.

# Definition of Done — Rename

**Satisfies:** G-001 (rename-by-grep), G-003 (public API edit if exported)

## Checklist

- [ ] **Step 1** — Call `find_references` on the symbol being renamed.
  - Confirm the response includes a `_receipt_id` field (e.g., `"rcpt_AAAA..."`).
  - If the symbol is public/exported, call `analyze_blast_radius` instead to satisfy G-003.
- [ ] **Step 2** — Record the `_receipt_id` value from the response.
- [ ] **Step 3** — Call `rename_symbol` (preferred — LSP-native, G-001 exempt) OR `fuzzy_edit`/`replace_in_file` with `receipts: ["<receipt_id>"]`.
- [ ] **Step 4** — Call `verify_edit` to confirm the rename applied cleanly.

## Preferred Tool Sequence (private symbol rename)

```json
{ "method": "tools/call", "params": { "name": "find_references", "arguments": { "symbol": "oldName", "file_path": "src/auth/handler.go" } } }
// Response includes: { "_receipt_id": "rcpt_AA0001BBBBBBBBB000AAAA0000" }

{ "method": "tools/call", "params": { "name": "rename_symbol", "arguments": { "file_path": "src/auth/handler.go", "line": 12, "column": 5, "new_name": "newName", "receipts": ["rcpt_AA0001BBBBBBBBB000AAAA0000"] } } }

{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/auth/handler.go" } } }
```

## Preferred Tool Sequence (public/exported symbol rename)

```json
{ "method": "tools/call", "params": { "name": "analyze_blast_radius", "arguments": { "symbol": "ExportedHandler", "file_path": "src/api/handler.go" } } }
// Response includes: { "_receipt_id": "rcpt_BB0002CCCCCCCCC000BBBB0000" }

{ "method": "tools/call", "params": { "name": "rename_symbol", "arguments": { "file_path": "src/api/handler.go", "line": 20, "column": 6, "new_name": "ExportedHandlerV2", "receipts": ["rcpt_BB0002CCCCCCCCC000BBBB0000"] } } }

{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/api/handler.go" } } }
```

## Notes

- `rename_symbol` is preferred over `fuzzy_edit`/`replace_in_file` because it is LSP-native and handles cross-file propagation automatically.
- `rename_symbol` is **exempt** from G-001 (rename-by-grep). Only `fuzzy_edit`/`replace_in_file` require a receipt for renaming an identifier.
- For public/exported symbols, use `analyze_blast_radius` (not `find_references`) to obtain an `impact_checked` receipt, which is required by G-003.
