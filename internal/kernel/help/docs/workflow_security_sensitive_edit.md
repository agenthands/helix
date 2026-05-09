Canonical Helix workflow for security-sensitive edits.

# Definition of Done — Security-Sensitive Edit

**Satisfies:** G-005 (security-sensitive edit)

A "security-sensitive edit" is any edit to a file that matches security path globs (`**/auth/**`, `**/crypto/**`, `**/middleware/**`, etc.), touches security-sensitive identifiers (`*Password*`, `*Token*`, `*Auth*`, etc.), or imports security-sensitive packages (Go: `crypto/*`, `net/http`; TypeScript/JavaScript: `jsonwebtoken`, `bcrypt`, `passport`; Python: `cryptography`, `jwt`, `hashlib`).

## Checklist

- [ ] **Step 1** — Call `get_context` covering **all** files you intend to edit. If editing multiple security-sensitive files, issue one `get_context` call per file.
- [ ] **Step 2** — Record the `_receipt_id` from the response.
- [ ] **Step 3** — Issue the destructive call with `receipts: ["<context_gathered_receipt_id>"]`.
- [ ] **Step 4** — Call `verify_edit`. On success, the response includes a `diagnostics_clean` receipt with `error_count == 0`.
- [ ] **Step 5** — Confirm `error_count == 0` in the `verify_edit` response. Report this in any session handoff.

## Tool Sequence

```json
{ "method": "tools/call", "params": { "name": "get_context", "arguments": { "file_path": "src/auth/jwt.go", "max_tokens": 4000 } } }
// Response: { "_receipt_id": "rcpt_HH0008IIIIIIIII000HHHH0000" }

{ "method": "tools/call", "params": { "name": "replace_symbol_body", "arguments": { "symbol": "generateToken", "file_path": "src/auth/jwt.go", "new_body": "func generateToken(...) {...}", "receipts": ["rcpt_HH0008IIIIIIIII000HHHH0000"] } } }

{ "method": "tools/call", "params": { "name": "verify_edit", "arguments": { "file_path": "src/auth/jwt.go" } } }
// Response: { "error_count": 0, "_receipt_id": "rcpt_II0009JJJJJJJJJ000IIII0000" }
```

## Why `diagnostics_clean` matters

After a security-sensitive edit, confirming `error_count == 0` is critical: a compile error or type error in auth/crypto code can silently degrade security (e.g., a broken validation path that a linter would catch but which passes without complaint). The `diagnostics_clean` receipt is your evidence that the post-edit state is clean.

## Notes

- The G-005 path globs, identifier patterns, and import patterns are configurable via `guardrails.G-005.*` config keys.
- Default catalogs ship for Go, TypeScript, JavaScript, and Python. Java/Rust/C# catalogs are deferred.
- If the edit triggers both G-003 (public API) and G-005 (security-sensitive), both receipts are required: an `impact_checked` receipt from `analyze_blast_radius` AND a `context_gathered` receipt from `get_context`.
