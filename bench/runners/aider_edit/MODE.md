---
mode: aider_edit
profile: bench-full
---

# aider_edit

The **polyglot-edit benchmark mode** (EDITBENCH-02). This mode drives the
Aider-Polyglot exercism fixtures through the Helix daemon's **full EDIT-verb
surface** (`replace_in_file`, `fuzzy_edit`, `replace_symbol_body`,
`insert_before_symbol`, `insert_after_symbol`, …) and grades the result with the
dataset's **native** per-language test command (the upstream
`benchmark.py run_unit_tests` table — pytest / cargo / go / gradlew / …), NOT the
TOOLBENCH runner argv.

It resolves to the `bench-full` Helix profile — the same full EDIT-verb surface a
real coding agent touches — so the measured edit format is the production
surface, not a stripped subset.

The `profile: bench-full` frontmatter is **structurally required** (the resolver
is strict two-key with `KnownFields(true)` — any extra frontmatter key fails the
parse). The arm is detected **BY MODE NAME** in `RunCell` (like `baseline_rag`)
so the cell branches to `runAiderEditCell` (Plan 02); the MODE.md still validates
the two-key frontmatter as a side effect. `mode_resolver.go` needs **zero** Go
change — adding this 7th mode dir grows the filesystem-as-table, not the resolver
code (`ResolveProfileFromRoot` reads `<root>/<mode>/MODE.md`).

## Deterministic baseline driver

The committed-baseline driver (Plan 01 substrate) is **deterministic**: for each
`files.solution` stub it applies the exercise's reference solution
(`files.example`, the `.meta/example.<ext>` body) through the `replace_in_file`
EDIT verb against the warm daemon, then records `edit_format_applied` on the
result row. The graded **test** file is restored pristine before each run
(WR-01), so a do-nothing or tampering edit cannot force a spurious green.
