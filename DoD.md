# Definition of Done — Helix Agent Checklists

Canonical per-task-class checklists for agents using Helix MCP tools. Each section defines the exact tool sequence required to satisfy guardrail rules (G-001..G-005) for that task class. Load this document as system context so your agent knows exactly which tools to call, in which order, with which receipt IDs.

For operator configuration and rule semantics, see `GUARDRAILS.md`. For runtime access, call `get_tool_help` with `topic: "dod"` or the individual workflow topics.

---

## Definition of Done — Rename

**Satisfies:** G-001 (rename-by-grep), G-003 (public API edit if exported)

### Checklist

- [ ] **Step 1** — Call `find_references` on the symbol being renamed.
  - Confirm the response includes a `_receipt_id` field (e.g., `"rcpt_AAAA..."`).
  - If the symbol is public/exported, call `analyze_blast_radius` instead to satisfy G-003.
- [ ] **Step 2** — Record the `_receipt_id` value from the response.
- [ ] **Step 3** — Call `rename_symbol` (preferred — LSP-native, G-001 exempt) OR `fuzzy_edit`/`replace_in_file` with `receipts: ["<receipt_id>"]`.
- [ ] **Step 4** — Call `verify_edit` to confirm the rename applied cleanly.

### Preferred Tool Sequence (private symbol rename)

```json
// Turn 1: gather references
{ "method": "tools/call", "params": { "name": "find_references", "arguments": { "symbol": "oldName", "file_path": "src/auth/handler.go" } } }
// Response includes: { "_receipt_id": "rcpt_AA0001BBBBBBBBB000AAAA0000" }

// Turn 2: rename via LSP (G-001 exempt)
{ "method": "tools/call", "params": { "name": "rename_symbol", "arguments": { "file_path": "src/auth/handler.go", "line": 12, "column": 5, "new_name": "newName", "receipts": ["rcpt_AA0001BBBBBBBBB000AAAA0000"] } } }

// Turn 3: verify
{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/auth/handler.go" } } }
```

### Preferred Tool Sequence (public/exported symbol rename)

```json
// Turn 1: blast-radius check (required for G-003)
{ "method": "tools/call", "params": { "name": "analyze_blast_radius", "arguments": { "symbol": "ExportedHandler", "file_path": "src/api/handler.go" } } }
// Response includes: { "_receipt_id": "rcpt_BB0002CCCCCCCCC000BBBB0000" }

// Turn 2: rename with impact_checked receipt
{ "method": "tools/call", "params": { "name": "rename_symbol", "arguments": { "file_path": "src/api/handler.go", "line": 20, "column": 6, "new_name": "ExportedHandlerV2", "receipts": ["rcpt_BB0002CCCCCCCCC000BBBB0000"] } } }

// Turn 3: verify
{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/api/handler.go" } } }
```

---

## Definition of Done — Delete

**Satisfies:** G-002 (delete-without-refs)

### Checklist

- [ ] **Step 1a** — For `safe_delete_symbol`: call `find_references` AND/OR `analyze_blast_radius` on the symbol.
  - `find_references` is sufficient for private symbols with `refcount == 0`.
  - `analyze_blast_radius` is required if the symbol is public/exported or entry-point-reachable.
- [ ] **Step 1b** — For `delete_file` with exported symbols: call `analyze_blast_radius` for each exported symbol defined in the file. All returned `_receipt_id` values must be included in the `receipts` array.
- [ ] **Step 1c** — For `delete_file` on a fully-private file (no exported symbols): `find_references` (or `get_context`) is sufficient.
- [ ] **Step 2** — Confirm the response shows zero callers or all callers are in files you will update.
- [ ] **Step 3** — Call the delete operation with `receipts: ["<id1>", "<id2>", ...]`.
- [ ] **Step 4** — Call `verify_edit` after the deletion.

### Tool Sequence (delete private symbol)

```json
// Turn 1: check references
{ "method": "tools/call", "params": { "name": "find_references", "arguments": { "symbol": "legacyHelper", "file_path": "src/util/helpers.go" } } }
// Response: { "ref_count": 0, "_receipt_id": "rcpt_CC0003DDDDDDDDD000CCCC0000" }

// Turn 2: delete
{ "method": "tools/call", "params": { "name": "safe_delete_symbol", "arguments": { "symbol": "legacyHelper", "file_path": "src/util/helpers.go", "receipts": ["rcpt_CC0003DDDDDDDDD000CCCC0000"] } } }

// Turn 3: verify
{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/util/helpers.go" } } }
```

### Tool Sequence (delete file with exported symbols)

```json
// Turn 1: blast-radius for each exported symbol
{ "method": "tools/call", "params": { "name": "analyze_blast_radius", "arguments": { "symbol": "Router", "file_path": "src/routing/router.go" } } }
// Response: { "_receipt_id": "rcpt_DD0004EEEEEEEEE000DDDD0000" }

// Turn 2: delete file with all receipts
{ "method": "tools/call", "params": { "name": "delete_file", "arguments": { "file_path": "src/routing/router.go", "receipts": ["rcpt_DD0004EEEEEEEEE000DDDD0000"] } } }

// Turn 3: verify
{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/routing/" } } }
```

---

## Definition of Done — Public API Change

**Satisfies:** G-003 (public API edit without blast-radius check)

A "public API change" is any modification to a symbol that is public/exported, entry-point-reachable, part of an interface/type contract, has external-package references, or changes its signature with callers.

### Checklist

- [ ] **Step 1** — Call `analyze_blast_radius` on the symbol (NOT just `find_references` — G-003 requires `impact_checked`, not `references_checked`).
- [ ] **Step 2** — Review the `blast_nodes` in the response to understand downstream impact.
- [ ] **Step 3** — Issue the destructive call (`replace_symbol_body`, `rename_symbol`, `fuzzy_edit`, or `replace_in_file`) with `receipts: ["<impact_checked_receipt_id>"]`.
- [ ] **Step 4** — Call `verify_edit` to confirm correctness.

### Tool Sequence

```json
// Turn 1: blast-radius analysis (issues impact_checked receipt)
{ "method": "tools/call", "params": { "name": "analyze_blast_radius", "arguments": { "symbol": "ProcessPayment", "file_path": "src/billing/service.go" } } }
// Response: { "blast_nodes": [...], "_receipt_id": "rcpt_EE0005FFFFFFFFF000EEEE0000" }

// Turn 2: replace symbol body with impact_checked receipt
{ "method": "tools/call", "params": { "name": "replace_symbol_body", "arguments": { "symbol": "ProcessPayment", "file_path": "src/billing/service.go", "new_body": "func ProcessPayment(...) {...}", "receipts": ["rcpt_EE0005FFFFFFFFF000EEEE0000"] } } }

// Turn 3: verify
{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/billing/service.go" } } }
```

### Why `analyze_blast_radius` and not `find_references`?

`find_references` issues a `references_checked` receipt; G-003 requires an `impact_checked` receipt. These are different classes with different scope structs. Using `find_references` then attempting a public API change will produce:

```json
{ "error": { "kind": "guardrail_violation", "rule": "G-003", "message": "impact_checked receipt required for public API edit; references_checked is insufficient" } }
```

---

## Definition of Done — Large Edit

**Satisfies:** G-004 (large fuzzy edit without context)

A "large edit" is any edit where `changed_lines (inserted + deleted) > 50` OR `touched_files > 1`. The secondary trigger `changed_lines / file_line_count > 0.30` is also active by default.

### Checklist

- [ ] **Step 1** — For single-file large edits: call `get_context` on the file being edited. For multi-file or module-wide edits: call `get_repo_map` on the relevant directory.
- [ ] **Step 2** — Confirm the response includes a `_receipt_id`.
- [ ] **Step 3** — Issue the destructive call with `receipts: ["<context_or_overview_receipt_id>"]`.
- [ ] **Step 4** — Call `verify_edit` after completion.

### Tool Sequence (single-file large edit)

```json
// Turn 1: gather file context (issues context_gathered receipt)
{ "method": "tools/call", "params": { "name": "get_context", "arguments": { "file_path": "src/parser/grammar.go", "max_tokens": 4000 } } }
// Response includes: { "_receipt_id": "rcpt_FF0006GGGGGGGGG000FFFF0000" }

// Turn 2: large edit with context receipt
{ "method": "tools/call", "params": { "name": "replace_in_file", "arguments": { "file_path": "src/parser/grammar.go", "find": "...", "replace": "...", "receipts": ["rcpt_FF0006GGGGGGGGG000FFFF0000"] } } }

// Turn 3: verify
{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/parser/grammar.go" } } }
```

### Tool Sequence (multi-file / module-wide edit)

```json
// Turn 1: get repo map (issues structural_overview receipt)
{ "method": "tools/call", "params": { "name": "get_repo_map", "arguments": { "path": "src/parser/", "max_tokens": 8000 } } }
// Response includes: { "_receipt_id": "rcpt_GG0007HHHHHHHHH000GGGG0000" }

// Turn 2: edit across multiple files with structural_overview receipt
{ "method": "tools/call", "params": { "name": "fuzzy_edit", "arguments": { "file_path": "src/parser/grammar.go", "find": "...", "replace": "...", "receipts": ["rcpt_GG0007HHHHHHHHH000GGGG0000"] } } }

// Turn 3: verify
{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/parser/" } } }
```

---

## Definition of Done — Security-Sensitive Edit

**Satisfies:** G-005 (security-sensitive edit)

A "security-sensitive edit" is any edit to a file that matches security path globs (`**/auth/**`, `**/crypto/**`, `**/middleware/**`, etc.), touches security-sensitive identifiers (`*Password*`, `*Token*`, `*Auth*`, etc.), or imports security-sensitive packages (see `GUARDRAILS.md` for the full catalog).

### Checklist

- [ ] **Step 1** — Call `get_context` covering **all** files you intend to edit. If editing multiple security-sensitive files, issue one `get_context` call per file (or use a multi-file context call if supported).
- [ ] **Step 2** — Record the `_receipt_id` from the response.
- [ ] **Step 3** — Issue the destructive call with `receipts: ["<context_gathered_receipt_id>"]`.
- [ ] **Step 4** — Call `verify_edit`. On success, the response includes a `diagnostics_clean` receipt with `error_count == 0`.
- [ ] **Step 5** — Confirm `error_count == 0` in the `verify_edit` response. Report this in any session handoff.

### Tool Sequence

```json
// Turn 1: get context of security-sensitive file
{ "method": "tools/call", "params": { "name": "get_context", "arguments": { "file_path": "src/auth/jwt.go", "max_tokens": 4000 } } }
// Response: { "_receipt_id": "rcpt_HH0008IIIIIIIII000HHHH0000" }

// Turn 2: edit with context receipt
{ "method": "tools/call", "params": { "name": "replace_symbol_body", "arguments": { "symbol": "generateToken", "file_path": "src/auth/jwt.go", "new_body": "func generateToken(...) {...}", "receipts": ["rcpt_HH0008IIIIIIIII000HHHH0000"] } } }

// Turn 3: verify (auto-issues diagnostics_clean receipt)
{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/auth/jwt.go" } } }
// Response: { "error_count": 0, "_receipt_id": "rcpt_II0009JJJJJJJJJ000IIII0000" }

// Step 5: confirm error_count == 0 before proceeding
```

### Why `diagnostics_clean` matters

After a security-sensitive edit, confirming `error_count == 0` is critical: a compile error or type error in auth/crypto code can silently degrade security (e.g., a broken validation path that a linter would catch but which passes without complaint). The `diagnostics_clean` receipt is your evidence that the post-edit state is clean.
