# Phase 104: Reference Generator Per-Verb Correctness - Pattern Map

**Mapped:** 2026-06-24
**Files analyzed:** 4 (2 modified, 1 new test, 1 regenerated artifact)
**Analogs found:** 4 / 4 (all in-repo, all read this session)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/helix-refgen/render.go` (MODIFY: `outputShape`, `useThisNotThat` + new override maps) | generator/transform | transform (catalog → markdown) | self (existing `outputShape`/`useThisNotThat` group switch) | exact (in-file refactor) |
| `cmd/helix-cligen/render.go` (`categoryToGroup`) | generator/transform | transform | self | NOT a fix site — read-only reference (see Anti-Pattern) |
| `cmd/helix-refgen/render_test.go` (NEW) | test | transform-assertion | `internal/cli/reference_contract_test.go` + `cmd/helix-cligen/render_test.go` | exact (mirror anti-vacuity templates) |
| `internal/cli/skills/helix/reference.md` (REGENERATE) | config/artifact | embedded doc | self (byte-gated by `--check`) | n/a — regenerated, never hand-edited |

## Pattern Assignments

### `cmd/helix-refgen/render.go` (generator, transform)

**Analog:** self — the existing group-keyed switch is the structure to refactor.

**Current call sites in `renderVerb`** (`cmd/helix-refgen/render.go:79-91`) — both keyed ONLY on `d.GroupID`; this is the bug surface. Update both to pass `d.Verb` as well:
```go
sb.WriteString("**Output:** ")
sb.WriteString(outputShape(d.GroupID))            // -> outputShape(d.Verb, d.GroupID)
...
sb.WriteString("**Use this, not that:** ")
sb.WriteString(useThisNotThat(d.GroupID, d.Verb)) // -> useThisNotThat(d.Verb, d.GroupID)
```

**Existing group switch to factor into a pure default helper** (`render.go:202-242`). The full group default sets (the OLD prose that overrides must differ from for Guard B) are:
- `outputShape` cases: `navigation`, `edit`, `fileops`, `diagnostics`, `memory` (`"the memory body or a ranked FTS5 search result set, one entry per line."`), `repomap`, `default`.
- `useThisNotThat` builds `cmd := "`helix " + verb + "`"` then switches; `memory` case returns `"Use " + cmd + " for durable project/session memory instead of ad-hoc scratch notes."` — the wrong prose 10 verbs inherit.

**Recommended refactor shape** (per RESEARCH Pattern 1 + Validation refactor note) — extract `groupOutputDefault(group string)` and `groupUseDefault(group, verb string)` as pure helpers holding the existing switch bodies verbatim, then:
```go
func outputShape(verb, group string) string {
    if s, ok := outputShapeOverrides[verb]; ok { return s }
    return groupOutputDefault(group)
}
func useThisNotThat(verb, group string) string {
    if s, ok := useThisNotThatOverrides[verb]; ok { return s }
    return groupUseDefault(group, verb)
}
```
Override maps are package-level `map[string]string` keyed on kebab verb, consulted by KEYED lookup (`m[verb]`), never ranged — preserves `TestRenderDeterministic` (`main_test.go:32`). Override set (10 verbs per RESEARCH root-cause table): `write-memory`, `edit-memory`, `delete-memory`, `rename-memory`, `switch-mode`, `get-token-budget`, `onboard-project`, `prepare-for-new-conversation`, `get-health`, `get-tool-help`.

**Determinism / escaping constraint:** the section loop already iterates `cli.VerbSpecsForDocs()` (sorted). Existing pipe-escape (`render.go:26,73`) applies to tool-derived text; override values are author constants — keep any literal `|` escaped if ever placed in a table cell (current Output/Use-this lines are not tabular).

---

### `cmd/helix-refgen/render_test.go` (test, transform-assertion) — NEW FILE

This package currently has only `main_test.go`. New file mirrors two established anti-vacuity templates below.

**Template A — fabricated-token discriminator** (from `internal/cli/reference_contract_test.go:85-98`, `TestReferenceContractDiscriminatesAbsentVerb`). Mirror this for **Guard A** (`TestOverrideKeysAreRealVerbs`): every override-map key must be in the kebab set of `cli.VerbToolNames()`; the discriminator injects `"totally-not-a-verb"` into a COPY of the map and asserts the key-validity helper flags it.
```go
const fabricated = "totally-not-a-verb"
require.NotContains(t, ref, referenceSectionHeader(fabricated),
    "precondition: the fabricated verb must have no section")
authorityPlus := append(append([]string{}, VerbToolNames()...), fabricated)
missing := referenceMissingVerbs(ref, authorityPlus)
require.Contains(t, missing, fabricated, "...keys on the real frozen authority, not ∅ ⊇ ∅")
```
Kebab authority construction (reuse `cli.VerbToolNames()`, `verb.go:81`):
```go
verb := strings.ReplaceAll(toolName, "_", "-")
```

**Template B — revert-and-fail / break-the-invariant** (from `reference_contract_test.go:106-128`, `TestReferenceCompletenessRevertFails`). Mirror this for **Guard B** (`TestOverrideDiffersFromGroupDefault`): for each overridden verb, assert `outputShape(verb, group) != groupOutputDefault(group)` and `useThisNotThat(verb, group) != groupUseDefault(group, verb)` — call the extracted default helper directly to bypass the override. A "fix" that copied the group default verbatim turns this RED. The template's discipline: establish baseline (intact passes), mutate the input to violate the invariant, assert the SAME helper goes RED, and assert exact cardinality (`require.Len(..., 1, ...)`).

**Template C — hard-fail generation assertion** (from `cmd/helix-cligen/render_test.go:141-156,179-193`, `TestRender_DuplicateFlagNameFailsGeneration` / `TestRender_ReservedSubcommandFailsGeneration`). Use this shape for any negative assertion that a malformed override should be rejected (`err == nil { t.Fatalf }` + `strings.Contains(err.Error(), ...)`).

**Template D — golden per-verb content** (from `cmd/helix-refgen/main_test.go:43-58`, `TestRenderSectionContent`; and `cmd/helix-cligen/render_test.go:91-101` for the `for _, want := range []string{...}` substring loop). Use for `TestRenderOverride`: render once, assert each of the 10 overridden verbs' corrected Output/Use-this prose is present and the OLD memory prose (`"ranked FTS5 search result"`, `"durable project/session memory"`) is absent from those verbs' sections.

---

### `internal/cli/skills/helix/reference.md` (artifact) — REGENERATE

**Pattern:** generated, never hand-edited. After editing `render.go`: `go run ./cmd/helix-refgen` then commit `reference.md` in the SAME commit. Verify byte-clean with `go run ./cmd/helix-refgen --check` (→ "reference.md is up to date.") and `git diff --exit-code internal/cli/skills/helix/reference.md`. Round-trip logic is `referenceStale()` (gated by `main_test.go:62` `TestCheckRoundTrip`).

## Shared Patterns

### Authority sourcing (anti-vacuity foundation)
**Source:** `internal/cli/verb.go:81` (`VerbToolNames()`), `verb.go:142` (`VerbSpecsForDocs()`)
**Apply to:** Guard A key-validity check; all completeness assertions.
Authority is ALWAYS `cli.VerbToolNames()` (50 frozen, sorted, fresh-copy) — NEVER the generator's own output (set-compared-to-itself tautology). Kebab conversion: `strings.ReplaceAll(toolName, "_", "-")`.

### Break-the-invariant discriminator (the SC #4 mandate)
**Source:** `reference_contract_test.go:85-128` (fabricated-token + revert-fail), `cmd/helix-cligen/render_test.go:141-156` (hard-fail)
**Apply to:** Every new guard in `render_test.go`. STATE constraint: "A gate with only a green-path test is presumed broken." Each guard ships a sibling that mutates the input to violate the invariant and asserts the SAME helper goes RED, with exact cardinality (`require.Len`).

### Determinism
**Source:** `cmd/helix-refgen/main_test.go:32` (`TestRenderDeterministic`), `cmd/helix-cligen/render_test.go:31` (`TestRender_Deterministic`)
**Apply to:** Override maps — keyed `m[verb]` lookup only, never ranged. Verified by the existing determinism test surviving the change.

### Testing framework
**Source:** `internal/cli/reference_contract_test.go:7-8`
**Apply to:** `render_test.go` — `testify` `assert`/`require` already vendored; `package main` (refgen tests are in-package, see `main_test.go:1`).

## No Analog Found

None. Every file has an exact in-repo analog. The phase builds ONE new gate (override-map vacuity guard) by mirroring two existing anti-vacuity templates — no new infrastructure.

## Do NOT Modify (read-only reference)

| File | Reason |
|------|--------|
| `cmd/helix-cligen/render.go` `categoryToGroup` (lines 60-74) | Root of the collapse but NOT the fix site — re-splitting churns `verbs_gen.go` GroupIDs and the cobra "Memory & Workflow:" help layout (`internal/cli/root.go:106`). The doc-prose fix belongs in refgen. (RESEARCH Anti-Pattern, Assumption A1.) |
| Blank-import block `cmd/helix-refgen/main.go:34-44`, `cmd/helix-cligen/main.go:26-36` | Parity drift risk (v1.12 docgen lesson). Touch nothing here; re-verify via `--check` + `reference_contract_test.go` (==50) after the change. |

## Metadata

**Analog search scope:** `cmd/helix-refgen/`, `cmd/helix-cligen/`, `internal/cli/`
**Files scanned:** 5 (render.go, main_test.go, reference_contract_test.go, cligen render_test.go, verb.go)
**Pattern extraction date:** 2026-06-24
