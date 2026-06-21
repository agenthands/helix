---
mode: no_structured_edit
profile: bench-no-structured-edit
---

# no_structured_edit

The structured-edit-disabled ablation arm. Resolves to the
`bench-no-structured-edit` profile
(internal/profile/profiles/bench-no-structured-edit.yaml). This arm runs Helix
with its tree-sitter structured-edit subsystem turned off
(`disable_structured_edit_subsystem`, Phase 76 ABLATE-07) so the matrix can
isolate the contribution of symbol-body surgery (`replace_symbol_body`,
`insert_before_symbol`, and the other structured editing tools) versus the
text-level fuzzy-edit fallback.

The `bench-no-structured-edit` profile keeps the full skill set and uses
`exclude_tools` to drop the structured-edit tools (Phase 76 D-09); the
agent retains `replace_in_file` / `fuzzy_edit` for text-level edits.
