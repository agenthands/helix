# Few-Shot Examples for LLM Judge

These examples illustrate how to apply the rubric to trace events. Use these as calibration references.

---

## Example 1: Rename Family (+1 outcome)

**Task kind:** rename
**Task title:** Rename function `parseConfig` to `parseOptions`

**Trace events:**
- [10:00:01.000] find_references | symbol=parseConfig | outcome: success
- [10:00:02.500] rename_symbol | old=parseConfig new=parseOptions | outcome: success
- [10:00:03.100] get_diagnostics | path=. | outcome: success

**Expected response:**
```json
{"scores":{"right_tool":1,"evidence":1,"blast_radius":1,"recovery":1},"reasoning":"Agent correctly used find_references before rename_symbol, ensuring all call sites were identified. Diagnostics check post-rename confirms verification.","flags":[]}
```

---

## Example 2: Delete Family (-1 outcome)

**Task kind:** delete
**Task title:** Remove deprecated function `legacyParse`

**Trace events:**
- [10:00:01.000] replace_in_file | path=parser.go content=... | outcome: success

**Expected response:**
```json
{"scores":{"right_tool":-1,"evidence":-1,"blast_radius":-1,"recovery":0},"reasoning":"Agent used replace_in_file for a deletion task without checking references first. safe_delete_symbol was not used; dangling callers are a risk. No blast-radius analysis performed.","flags":["skipped_find_references","raw_file_edit_for_delete","safe_delete_not_used"]}
```
