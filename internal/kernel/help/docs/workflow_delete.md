Canonical Helix workflow for symbol and file deletion.

# Definition of Done — Delete

**Satisfies:** G-002 (delete-without-refs)

## Checklist

- [ ] **Step 1a** — For `safe_delete_symbol`: call `find_references` AND/OR `analyze_blast_radius` on the symbol.
  - `find_references` is sufficient for private symbols with `refcount == 0`.
  - `analyze_blast_radius` is required if the symbol is public/exported or entry-point-reachable.
- [ ] **Step 1b** — For `delete_file` with exported symbols: call `analyze_blast_radius` for each exported symbol defined in the file. All returned `_receipt_id` values must be included in the `receipts` array.
- [ ] **Step 1c** — For `delete_file` on a fully-private file (no exported symbols): `find_references` (or `get_context`) is sufficient.
- [ ] **Step 2** — Confirm the response shows zero callers or all callers are in files you will update.
- [ ] **Step 3** — Call the delete operation with `receipts: ["<id1>", "<id2>", ...]`.
- [ ] **Step 4** — Call `verify_edit` after the deletion.

## Tool Sequence (delete private symbol)

```json
{ "method": "tools/call", "params": { "name": "find_references", "arguments": { "symbol": "legacyHelper", "file_path": "src/util/helpers.go" } } }
// Response: { "ref_count": 0, "_receipt_id": "rcpt_CC0003DDDDDDDDD000CCCC0000" }

{ "method": "tools/call", "params": { "name": "safe_delete_symbol", "arguments": { "symbol": "legacyHelper", "file_path": "src/util/helpers.go", "receipts": ["rcpt_CC0003DDDDDDDDD000CCCC0000"] } } }

{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/util/helpers.go" } } }
```

## Tool Sequence (delete file with exported symbols)

```json
{ "method": "tools/call", "params": { "name": "analyze_blast_radius", "arguments": { "symbol": "Router", "file_path": "src/routing/router.go" } } }
// Response: { "_receipt_id": "rcpt_DD0004EEEEEEEEE000DDDD0000" }

{ "method": "tools/call", "params": { "name": "delete_file", "arguments": { "file_path": "src/routing/router.go", "receipts": ["rcpt_DD0004EEEEEEEEE000DDDD0000"] } } }

{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/routing/" } } }
```
