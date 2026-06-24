# Phase 104: Reference Generator Per-Verb Correctness - Research

**Researched:** 2026-06-24
**Domain:** Go code-generation (helix-refgen), deterministic markdown rendering, anti-vacuity test design
**Confidence:** HIGH (every claim grounded in the actual generator source, read this session)

## Summary

The "group collapse" root cause is precisely located and small. The reference generator (`cmd/helix-refgen/render.go`) emits each verb's **Output** line via `outputShape(d.GroupID)` and its **Use this, not that** line via `useThisNotThat(d.GroupID, d.Verb)` — both keyed *only* on `GroupID`. There are 6 group IDs, but the upstream catalog generator (`cmd/helix-cligen/render.go`) folds **five distinct ToolProvider categories** (`memory`, `workflow`, `health`, `help`, `profile`) into the single `groupMemory` ("memory") group ID via `categoryToGroup`. Consequently every non-memory verb in that bucket (`switch-mode`, `get-token-budget`, `onboard-project`, `prepare-for-new-conversation`, `get-health`, `get-tool-help`) inherits memory-query prose — confirmed live: `helix switch-mode` currently reads `**Output:** the memory body or a ranked FTS5 search result set` and `**Use this, not that:** Use 'helix switch-mode' for durable project/session memory instead of ad-hoc scratch notes.` That is wrong for a mode switch. The mutating memory verbs (`write-memory`, `edit-memory`, `delete-memory`, `rename-memory`) share the query-flavored memory prose too.

**Primary recommendation:** Fix at the generator root cause in `cmd/helix-refgen/render.go` by introducing a **per-verb override map** keyed on the kebab verb name, consulted *before* the group-default fallback in BOTH `outputShape` and `useThisNotThat` (change their signatures to take `(verb, group string)`). Keep the existing `switch group` blocks as the fallback. Do NOT change `cmd/helix-cligen`'s `categoryToGroup` — the cobra display group "memory" is intentionally titled "Memory & Workflow:" and is a CLI-help concern, not a doc-prose concern; splitting it would churn `verbs_gen.go` and the cobra help layout for no benefit. Then regenerate `reference.md` (`go run ./cmd/helix-refgen`), commit it, and verify `--check` is byte-clean.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REFGEN-01 | Every verb's "use this, not that" and "Output" lines in generated `reference.md` are correct for that verb's semantics; fix the `cmd/helix-cligen`/`cmd/helix-refgen` group-collapse via a per-verb override in the generator (not hand-edits); regenerated `reference.md` committed and passes `helix-refgen --check`. | Root cause located in `cmd/helix-refgen/render.go` `outputShape`/`useThisNotThat` (group-keyed) + `cmd/helix-cligen/render.go` `categoryToGroup` (5-category→memory collapse). Override-map design, regen/commit flow, and `--check`/contract/parity gates all mapped below. |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Per-verb Output/Use-this prose | Generator (`cmd/helix-refgen`) | — | Success criterion #1 mandates the fix lives in the generator, not the committed artifact |
| Verb→group catalog | Generator (`cmd/helix-cligen` → `internal/cli/verbs_gen.go`) | — | `GroupID` is generated; the collapse originates here but is NOT the fix site (see Decision below) |
| Committed reference artifact | `internal/cli/skills/helix/reference.md` | — | Regenerated output, never hand-edited; byte-gated by `--check` |
| Completeness contract | `internal/cli/reference_contract_test.go` | — | Asserts `reference ⊇ VerbToolNames()` (exact-count==50), already hardened in Phase 103 |
| Drift gate | `make verify-reference` / CI `go-test.yml` (REF-03) | — | `go run ./cmd/helix-refgen --check` byte-reproducibility |

## Standard Stack

This is a pure in-repo Go codegen change. **No external packages.** Verified there are no new dependencies needed.

### Core (already in tree)
| Component | Location | Purpose |
|-----------|----------|---------|
| `helix-refgen` | `cmd/helix-refgen/{main.go,render.go,main_test.go}` | Emits `internal/cli/skills/helix/reference.md`; owns `outputShape`/`useThisNotThat` (the fix site) |
| `helix-cligen` | `cmd/helix-cligen/{main.go,render.go,scan.go,*_test.go}` | Emits `internal/cli/verbs_gen.go` (the verb→group catalog); owns `categoryToGroup` (root of the collapse, but NOT the fix site) |
| `cli.VerbSpecsForDocs()` | `internal/cli/verb.go:142` | Read-only `[]VerbDoc` (Verb, ToolName, GroupID, Short, Flags), sorted by kebab verb — refgen's input |
| `cli.VerbToolNames()` | `internal/cli/verb.go:81` | The 50-verb frozen authority (sorted tool names) — the completeness contract source of truth |
| `skill.ToolProviders()` | `internal/skill` | Live registry walk; refgen reads it for the synopsis prose |

### Package Legitimacy Audit

> Not applicable — this phase installs **zero** external packages. All work is in already-vendored, in-repo Go. (REQUIREMENTS.md "Out of Scope": "New Go module dependencies — the rewrite/allowlist work uses only already-vendored packages.")

## The Root Cause (verified, exact)

### Where the prose is chosen — `cmd/helix-refgen/render.go`
```go
// renderVerb (lines 79-92):
sb.WriteString("**Output:** ")
sb.WriteString(outputShape(d.GroupID))          // <-- keyed ONLY on GroupID
...
sb.WriteString("**Use this, not that:** ")
sb.WriteString(useThisNotThat(d.GroupID, d.Verb)) // <-- group decides; verb only fills the cmd token
```
`outputShape(group string)` (render.go:203) and `useThisNotThat(group, verb string)` (render.go:224) are `switch group { case "navigation": ... case "memory": ... default: ... }`. The `verb` arg in `useThisNotThat` is used ONLY to build the `` `helix <verb>` `` token, never to vary the guidance.

### Why every odd verb lands in "memory" — `cmd/helix-cligen/render.go:60`
```go
var categoryToGroup = map[string]string{
    "symbol-retrieval": groupNavigation,
    "symbols":          groupNavigation,
    "symbol-editing":   groupEdit,
    "file-ops":         groupFileops,
    "diagnostics":      groupDiagnostics,
    "repomap":          groupRepomap,
    "semantic":         groupRepomap,
    "memory":           groupMemory,
    "workflow":         groupMemory,  // <-- collapse
    "health":           groupMemory,  // <-- collapse
    "help":             groupMemory,  // <-- collapse
    "profile":          groupMemory,  // <-- collapse
}
```
Plus an unknown-category fallback (`if group == "" { group = groupMemory }`, render.go:133-135). The ToolProvider category names are real and verified via `tp.Name()`: `workflow` (`internal/skill/workflow/skill.go:29`), `health` (`internal/kernel/health/skill_adapter.go:17`), `help` (`internal/kernel/help/skill_adapter.go:17`), `profile` (`internal/profile/skill.go:38`), `memory` (`internal/skill/memory/skill.go:29`).

### Exact list of the 13 verbs currently in groupID "memory" (verified by grep on `verbs_gen.go`)
| Verb | True ToolProvider category | Owner file | Currently-wrong prose? |
|------|---------------------------|-----------|------------------------|
| `read-memory` | memory (query) | `internal/skill/memory` | Correct (genuine memory query) |
| `search-memories` | memory (query) | `internal/skill/memory` | Correct |
| `list-memories` | memory (query) | `internal/skill/memory` | Correct |
| `write-memory` | memory (**mutate**) | `internal/skill/memory` | **WRONG** — "ranked FTS5 search result" / "durable memory instead of scratch notes" describes a query, not a write |
| `edit-memory` | memory (**mutate**) | `internal/skill/memory` | **WRONG** (same) |
| `delete-memory` | memory (**mutate**) | `internal/skill/memory` | **WRONG** (same) |
| `rename-memory` | memory (**mutate**) | `internal/skill/memory` | **WRONG** (same) |
| `switch-mode` | profile | `internal/profile/skill.go:70` | **WRONG** — switches profile mode, not memory |
| `get-token-budget` | profile | `internal/profile/skill.go:75` | **WRONG** — reports the per-tool token budget |
| `onboard-project` | workflow | `internal/skill/workflow/skill.go:91` | **WRONG** — session onboarding workflow |
| `prepare-for-new-conversation` | workflow | `internal/skill/workflow/skill.go:104` | **WRONG** — session handoff workflow |
| `get-health` | health | `internal/kernel/health` | **WRONG** — per-workspace LS status/capabilities |
| `get-tool-help` | help | `internal/kernel/help` | **WRONG** — on-demand tool documentation |

The success criteria explicitly name `switch-mode`, `get-token-budget`, `onboard-project`, `get-health`, `get-tool-help`, and "mutating memory verbs" — all confirmed present and wrong above. `prepare-for-new-conversation` is the unnamed sixth non-memory verb in the same bucket (also wrong); recommend including it in the override set.

## Architecture Patterns

### System Architecture (data flow)
```
ToolProvider registry (skill.ToolProviders, via blank imports)
        │  tp.Name() = category, tp.Tools() = ToolDef{Name, Description}
        ▼
cmd/helix-cligen ──categoryToGroup──▶ internal/cli/verbs_gen.go (verbSpecs: verb→{toolName, groupID, flags})
        │                                         │
        │                                         ▼
        │                              cli.VerbSpecsForDocs() ──▶ []VerbDoc{Verb, GroupID, ...}
        │                                         │
        ▼                                         ▼
   (synopsis prose                     cmd/helix-refgen/render.go renderVerb()
    from tool.Description)  ──────────▶   ├─ outputShape(GroupID)        ◀── FIX: add per-verb override
                                          └─ useThisNotThat(GroupID,Verb)◀── FIX: add per-verb override
                                                   │
                                                   ▼
                                  internal/cli/skills/helix/reference.md  (committed)
                                                   │
                              ┌────────────────────┴───────────────────┐
                              ▼                                         ▼
                  make verify-reference                    reference_contract_test.go
                  (refgen --check, byte gate, REF-03)      (reference ⊇ VerbToolNames, ==50)
```

### Pattern 1: Per-verb override map with group-default fallback (RECOMMENDED design)
**What:** Two package-level `map[string]string` literals in `cmd/helix-refgen/render.go`, keyed on the kebab verb name; consulted by keyed lookup (NOT ranged) before the group switch.
**When to use:** Any verb whose collapsed `GroupID` produces semantically wrong prose.
**Determinism:** keyed `m[verb]` lookup is order-independent; the file is still emitted in `VerbSpecsForDocs()` sorted order, so output stays byte-identical across runs (preserves `TestRenderDeterministic`).

**Example (illustrative shape — final prose at planner/executor discretion):**
```go
// Source: derived from cmd/helix-refgen/render.go outputShape/useThisNotThat
// Per-verb Output overrides for verbs whose collapsed GroupID gives wrong prose.
var outputShapeOverrides = map[string]string{
    "write-memory":   "a confirmation that the named memory was written/updated (durable markdown + FTS5 index).",
    "edit-memory":    "a confirmation that the named memory body was updated.",
    "delete-memory":  "a confirmation that the named memory was removed.",
    "rename-memory":  "a confirmation that the memory was renamed.",
    "switch-mode":    "the active profile/mode after the switch (and the tools it gates).",
    "get-token-budget": "the per-tool token budget for the active profile/mode.",
    "onboard-project":  "an onboarding summary for the workspace (languages, entry points, suggested memories).",
    "prepare-for-new-conversation": "a session-handoff summary to seed the next conversation.",
    "get-health":     "per-workspace language-server status, capabilities, and indexing progress.",
    "get-tool-help":  "comprehensive on-demand documentation for the named tool.",
}

func outputShape(verb, group string) string {
    if s, ok := outputShapeOverrides[verb]; ok {
        return s
    }
    switch group { /* unchanged existing cases + default */ }
}
```
Mirror the same override-then-switch structure for `useThisNotThatOverrides` / `useThisNotThat(verb, group string)`. Update the two call sites in `renderVerb` to pass `d.Verb` as well.

### Pattern 2: Keep `useThisNotThat` cmd-token construction
The existing `cmd := "`helix " + verb + "`"` prefix is still wanted in overrides — write override values as full sentences that reference the verb, OR keep building the cmd token and store only the predicate. Either is deterministic; pick one consistently.

### Anti-Patterns to Avoid
- **Hand-editing `reference.md`** — fails `--check` (REQUIREMENTS Out of Scope; STATE v2.2 constraint). All corrections go through the generator.
- **Ranged map iteration to pick prose** — non-deterministic; would break `TestRenderDeterministic` and `--check`. Use keyed lookup.
- **Re-splitting `categoryToGroup` in cligen** — would churn `verbs_gen.go` `GroupID` values AND the cobra help group layout (the "memory" cobra group is deliberately titled "Memory & Workflow:" at `internal/cli/root.go:106`). Out of scope and risk-multiplying. The doc-prose fix belongs in refgen.
- **Adding override keys for verbs that are already correct** — a non-vacuity guard (SC #4) will flag an override whose value equals the group default it "replaced."

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Byte-reproducibility check | A custom diff script | Existing `referenceStale()` + `make verify-reference` (`refgen --check`) | Already wired into CI (REF-03) |
| Completeness (reference ⊇ verbs) | A new test | Existing `reference_contract_test.go` (Phase 103, exact-count==50) | Already non-vacuous with revert-fail proof |
| Verb→group/flag catalog | Hand table | `cli.VerbSpecsForDocs()` | Generated, sorted, fresh-copy; refgen already consumes it |
| Verb authority list | Hardcoded list | `cli.VerbToolNames()` | The single frozen-50 source of truth |

**Key insight:** Every gate this phase needs already exists from Phases 97/103. The phase adds ONE new gate (the override-map vacuity guard, SC #4) and corrects prose — it does not build new infrastructure.

## Runtime State Inventory

> Not a rename/refactor/migration of stored or live state — this is a code-generation correctness fix touching generator source + the regenerated committed artifact. No databases, services, OS registrations, secrets, or installed packages embed the changed strings as keys.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — verified: the change is to generated doc prose, not to any datastore key/collection/id. | none |
| Live service config | None — `reference.md` ships embedded in the binary and is rewritten by `helix setup`; no external service holds it. | none |
| OS-registered state | None. | none |
| Secrets/env vars | None. | none |
| Build artifacts | The committed `internal/cli/skills/helix/reference.md` IS the artifact and is regenerated by `go run ./cmd/helix-refgen`; it is also `go:embed`-ed into the binary (`embeddedSkillFS`), so a rebuild re-embeds it. | Regenerate + commit `reference.md`; rebuild picks it up automatically. |

## Common Pitfalls

### Pitfall 1: Forgetting to regenerate after editing override maps
**What goes wrong:** Override map edited in `render.go`, but `reference.md` not regenerated → `--check` RED in CI.
**How to avoid:** `go run ./cmd/helix-refgen` then commit the changed `reference.md` in the SAME commit. Verify with `go run ./cmd/helix-refgen --check` (must print "reference.md is up to date.") and `git diff --exit-code internal/cli/skills/helix/reference.md`.
**Warning sign:** `make verify-reference` exits 1.

### Pitfall 2: Vacuous override map (the SC #4 trap)
**What goes wrong:** An override key is a non-verb (typo, stale name) → silently never matches → dead entry. Or an override value equals the group default it "replaced" → no-op masquerading as a fix.
**How to avoid:** Add the two vacuity guards (see Validation Architecture). Guard A: every override key ∈ `cli.VerbToolNames()` kebab set. Guard B: for each overridden verb, the rendered Output/Use-this line ≠ the old group-default line for that verb's GroupID (a deliberate "break the invariant" assertion).
**Warning sign:** A guard that's only green-path — STATE constraint: "A gate with only a green-path test is presumed broken."

### Pitfall 3: Blank-import parity drift between generators and daemon
**What goes wrong:** Editing refgen and accidentally adding/removing a blank import diverges the generated tool SET from the daemon's, silently dropping/adding verbs (the v1.12 docgen-drift lesson).
**How to avoid:** Do NOT touch the blank-import block in `cmd/helix-refgen/main.go:34-44` or `cmd/helix-cligen/main.go:26-36`. The reciprocal-note rule (refgen main.go:23-33, daemon `internal/daemon/imports.go:6-13`) states literal import-list equality is NOT required (health/help are non-blank in the daemon by design, guardrails contributes zero verbs), but the tool SET must match. The `refgen --check` gate (REF-03) is the real protection. Re-verify by running the full suite + `--check` after the change — if the verb set drifted, `reference_contract_test.go` (==50) and `verify-cligen` go RED.
**Warning sign:** verb count ≠ 50, or `verify-cligen`/`verify-reference` RED.

### Pitfall 4: Breaking determinism with map iteration
**What goes wrong:** Iterating an override map to build output → random order → `--check` flaps.
**How to avoid:** Keyed lookup only (`m[verb]`). The section loop already iterates `VerbSpecsForDocs()` (sorted). Confirmed by `TestRenderDeterministic` in `cmd/helix-refgen/main_test.go:32`.

## Code Examples

### Verifying the fix end-to-end (the regen + gate loop)
```bash
# 1. Edit cmd/helix-refgen/render.go (override maps + signatures + call sites).
# 2. Regenerate the committed artifact.
go run ./cmd/helix-refgen
# 3. Byte-reproducibility gate (must be clean).
go run ./cmd/helix-refgen --check       # -> "reference.md is up to date."
git diff --exit-code internal/cli/skills/helix/reference.md   # (after staging the regen, should be empty on a 2nd regen)
# 4. Catalog + docs drift gates stay green (parity re-verify).
go run ./cmd/helix-cligen --check        # verbs_gen.go unchanged (we did NOT touch categoryToGroup)
go run ./cmd/docgen --check
# 5. Full suite (contract + generator + vacuity guards).
go vet ./... && go test ./...
```

### Confirming the wrong prose today (the RED baseline)
```bash
# Source: live reference.md, verified this session
awk '/^## `helix switch-mode`/{f=1} f{print} /^\*\*Use this/{if(f)exit}' \
  internal/cli/skills/helix/reference.md | grep -E '^\*\*(Output|Use this)'
# -> **Output:** the memory body or a ranked FTS5 search result set, one entry per line.
# -> **Use this, not that:** Use `helix switch-mode` for durable project/session memory instead of ad-hoc scratch notes.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Group-only prose selection in refgen | Per-verb override + group fallback | This phase (104) | Corrects 10 verbs' Output/Use-this prose |
| Reference completeness gate vacuous-prone | Exact-count==50 + revert-fail discriminator | Phase 103 (BUNDLE-02) | Gate cannot pass `∅ ⊇ ∅` through the churn |

**Deprecated/outdated:** none for this phase.

## Validation Architecture

> Nyquist validation is ENABLED for this phase. This section is mandatory.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` stdlib + `testify` (`assert`/`require`) — already used in `reference_contract_test.go` and `cmd/helix-*/*_test.go` |
| Config file | none — `go test` |
| Quick run command | `go test ./cmd/helix-refgen/... ./internal/cli/...` |
| Full suite command | `go vet ./... && go test ./...` |

### Phase Requirements → Test Map
| Req | Behavior | Test Type | Automated Command | File Exists? |
|-----|----------|-----------|-------------------|-------------|
| REFGEN-01 (correct prose) | Each overridden verb's rendered Output/Use-this line is its corrected prose (golden per-verb assertions for `switch-mode`, `get-token-budget`, `onboard-project`, `prepare-for-new-conversation`, `get-health`, `get-tool-help`, the 4 mutating memory verbs) | unit/golden | `go test ./cmd/helix-refgen/ -run TestRenderOverride` | ❌ Wave 0 |
| REFGEN-01 (root-cause, not hand-edit) | `outputShape`/`useThisNotThat` consult the override map before the group switch | unit | `go test ./cmd/helix-refgen/ -run TestRenderOverride` | ❌ Wave 0 |
| REFGEN-01 (byte-reproducible) | `reference.md` round-trips through `--check` | contract | `go run ./cmd/helix-refgen --check` (+ existing `TestCheckRoundTrip`) | ✅ `cmd/helix-refgen/main_test.go` |
| REFGEN-01 (completeness stays green) | reference ⊇ VerbToolNames(), ==50 | contract | `go test ./internal/cli/ -run TestReferenceCoversEveryVerb` | ✅ `reference_contract_test.go` |
| REFGEN-01 (drift gate green) | CI REF-03 byte gate | contract | `make verify-reference` | ✅ Makefile + `go-test.yml` |
| REFGEN-01 (parity re-verified) | verb SET unchanged (cligen --check clean) | contract | `make verify-cligen` | ✅ Makefile + `go-test.yml` |
| SC #4 Guard A (no non-verb key) | every override map key ∈ kebab(`VerbToolNames()`) | unit + RED-discriminator | `go test ./cmd/helix-refgen/ -run TestOverrideKeysAreRealVerbs` | ❌ Wave 0 |
| SC #4 Guard B (override differs from group default) | each overridden line ≠ the old group-default line for that GroupID (deliberate break-the-invariant) | unit + RED-discriminator | `go test ./cmd/helix-refgen/ -run TestOverrideDiffersFromGroupDefault` | ❌ Wave 0 |

### Anti-Vacuity (SC #4 — the deliberate break-the-invariant pattern)
The repo's established pattern (see `reference_contract_test.go` `TestReferenceContractDiscriminatesAbsentVerb` + `TestReferenceCompletenessRevertFails`, and `cmd/helix-cligen/render_test.go` `TestRender_DuplicateFlagNameFailsGeneration`): a gate ships a sibling that mutates the input to violate the invariant and asserts the SAME helper goes RED. For this phase:

- **Guard A discriminator:** inject a fabricated key (e.g. `"totally-not-a-verb"`) into a copy of the override map and assert the key-validity helper reports it as a non-verb. (Mirrors `TestReferenceContractDiscriminatesAbsentVerb`'s fabricated-token approach.)
- **Guard B discriminator:** for an overridden verb, assert its rendered Output line ≠ the value `outputShape` would return for its GroupID via the *fallback path* (call the group switch directly with the override map bypassed). A "fix" that copied the group default verbatim must turn this RED.

Refactor note: factor the group-default switch into a pure helper (e.g. `groupOutputDefault(group)` / `groupUseDefault(group, verb)`) so Guard B can call the fallback in isolation, and the override functions become `if v, ok := overrides[verb]; ok { return v }; return groupDefault(...)`. This makes both the override and the default independently testable (and keeps `outputShape`/`useThisNotThat` thin).

### Sampling Rate
- **Per task commit:** `go test ./cmd/helix-refgen/... ./internal/cli/...`
- **Per wave merge:** `go vet ./... && go test ./...` + `make verify-refgen verify-cligen` (note: target is `verify-reference`)
- **Phase gate:** full suite green + `git diff --exit-code internal/cli/skills/helix/reference.md` empty after fresh regen, before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `cmd/helix-refgen/render_test.go` (NEW file — `cmd/helix-refgen` currently has only `main_test.go`) covering: per-verb override golden assertions (REFGEN-01), Guard A (`TestOverrideKeysAreRealVerbs` + fabricated-key discriminator), Guard B (`TestOverrideDiffersFromGroupDefault` + copied-default discriminator).
- [ ] No new fixtures or framework install needed — testify already vendored.

## Environment Availability

> Skip-eligible (pure in-repo Go), but the build toolchain is the only dependency:

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build + `go run ./cmd/helix-refgen`, `go test` | ✓ (project builds today; `go run ./cmd/helix-refgen --check` ran clean this session) | repo `go.mod` | — |

No missing dependencies.

## Security Domain

> `security_enforcement` is not disabled, but this phase changes only generated documentation prose. ASVS surface is minimal.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes (existing) | refgen already pipe-escapes synopsis/help (`strings.ReplaceAll(desc, "|", "\\|")`, render.go:25,73) for markdown-injection safety; override-map values are author-controlled constants (no user input), so no new injection surface. Keep any literal `|` in override prose escaped if a table cell could contain it (current Output/Use-this lines are not in tables). |
| Others (V2/V3/V4/V6) | no | No auth, session, access-control, or crypto surface in doc generation. |

| Pattern | STRIDE | Mitigation |
|---------|--------|-----------|
| Markdown/markup injection via generated prose | Tampering | Override values are compile-time string constants; no dynamic/user data flows into them. Existing pipe-escape covers tool-derived text. |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The intended fix site is `cmd/helix-refgen` (doc prose), and `cmd/helix-cligen`'s `categoryToGroup` should NOT be re-split. | Standard Stack / Anti-Patterns | LOW — SC #1 names refgen first and says "per-verb override map"; splitting cligen groups would churn cobra help layout. If the planner prefers re-splitting groups, the override approach is still the safer, lower-blast-radius choice and matches "per-verb override." |
| A2 | `prepare-for-new-conversation` should be included in the override set (it is the unnamed sixth non-memory verb in the memory bucket). | Root Cause table | LOW — it is verifiably workflow-category and currently shows memory prose; omitting it leaves one verb wrong. |
| A3 | Exact corrected prose wording is at executor discretion (the example strings above are illustrative). | Pattern 1 example | LOW — only the *correctness/semantics* is contractually required; wording is style. Guard B only requires it differ from the group default. |
| A4 | The 4 mutating memory verbs (`write/edit/delete/rename-memory`) are in scope as "mutating memory verbs" per SC #1. | Root Cause table | LOW — SC #1 names them explicitly; their current query-flavored prose is wrong for a mutation. |

**All four assumptions are LOW risk and consistent with the success criteria; no user confirmation strictly required, but A1 (refgen-not-cligen fix site) is the one design choice the planner should affirm.**

## Open Questions

1. **Should `read-memory`/`search-memories`/`list-memories` keep the group default or also get tightened overrides?**
   - What we know: these are genuine memory queries; the current group prose is *roughly* correct for them.
   - What's unclear: whether "the memory body or a ranked FTS5 search result set" is precise enough per-verb (`list-memories` returns names, not bodies; `read-memory` returns one body, not a result set).
   - Recommendation: optional polish. Add overrides for the three query verbs too if pursuing per-verb precision, but they are not in the SC #1 named-wrong set, so they are not required. Guard B would require each such override to differ from the group default.

## Sources

### Primary (HIGH confidence — read this session)
- `cmd/helix-refgen/render.go` — `outputShape`/`useThisNotThat` (group-keyed), `renderVerb`, determinism contract.
- `cmd/helix-refgen/main.go` + `main_test.go` — `--check`/`referenceStale`, blank-import parity note, `TestRenderDeterministic`/`TestCheckRoundTrip`.
- `cmd/helix-cligen/render.go` — `categoryToGroup` (the 5-category→memory collapse), group constants.
- `cmd/helix-cligen/main.go` — `liveRegistryNames` (tp.Name() = category), blank imports.
- `cmd/helix-cligen/render_test.go` — anti-vacuity hard-fail test patterns (duplicate-flag/reserved-subcommand).
- `internal/cli/verb.go` — `VerbDoc`, `VerbSpecsForDocs()`, `VerbToolNames()`.
- `internal/cli/verbs_gen.go` — actual per-verb `groupID` assignments (the 13 memory-group verbs).
- `internal/cli/reference_contract_test.go` — completeness contract (==50, revert-fail, discriminator).
- `internal/cli/root.go` — cobra group "memory" titled "Memory & Workflow:".
- ToolProvider `Name()` methods (workflow/health/help/profile/memory/semantic skill files).
- `Makefile` (`reference`/`verify-reference`/`verify-cligen`/`verify-docs`) + `.github/workflows/go-test.yml` (REF-03/VERB-02/DOCS-02 gates).
- Live `reference.md` grep — confirmed wrong prose for the 6 named verbs.

### Secondary / Tertiary
- None — no external sources needed; everything verified in-repo.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — in-repo only, no deps, all files read.
- Architecture / root cause: HIGH — exact lines and the 13-verb collapse list verified by source + live grep.
- Pitfalls: HIGH — drawn from the repo's own gate wiring and STATE constraints.
- Validation: HIGH — existing test patterns (`reference_contract_test.go`, cligen render_test) are the templates.

**Research date:** 2026-06-24
**Valid until:** stable (~30 days) — generator source is slow-moving; the only invalidator is a verb-set change (would trip the ==50 gate).
