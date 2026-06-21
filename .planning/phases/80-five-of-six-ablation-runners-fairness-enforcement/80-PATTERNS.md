# Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement - Pattern Map

**Mapped:** 2026-06-19
**Files analyzed:** 14 (6 new, 8 modified/extended)
**Analogs found:** 14 / 14 (every file has an in-repo analog — this phase is pure growth)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/runners/baseline_plain/MODE.md` (NEW) | config | transform | `bench/runners/your_agent_full/MODE.md` | exact |
| `bench/runners/no_lsp/MODE.md` (NEW) | config | transform | `bench/runners/your_agent_full/MODE.md` | exact |
| `bench/runners/no_structured_edit/MODE.md` (NEW) | config | transform | `bench/runners/your_agent_full/MODE.md` | exact |
| `bench/runners/your_agent_no_semantic/MODE.md` (NEW) | config | transform | `bench/runners/your_agent_full/MODE.md` | exact |
| `bench/runners/baseline_rag/MODE.md` (NEW) | config | transform | `bench/runners/your_agent_full/MODE.md` | exact (+ stub-marker variant) |
| `bench/runners/contract_test.go` (NEW) | test | request-response | `bench/runners/fairness_contract_test.go` | exact (same package, same style) |
| `bench/runtime/cell.go` (MOD) | service | request-response | itself (`RunCell` steps 1/9) | self-extend |
| `bench/runtime/result.go` (MOD) | model | transform | itself (`ResultInput`/`resultDoc`/`BuildResult`) | self-extend |
| `bench/runtime/matrix.go` (MOD) | service | batch | itself (`RunMatrix`/`dispatch`/`Summary`) | self-extend |
| `bench/runners/deltas.go` (NEW, helper) | utility | transform | `bench/runtime/result.go` `BuildResult` (pure transform) | role-match |
| `bench/schema/result.v2.schema.json` (MOD) | config | — | existing `mode` + `fairness.overrides[]` defs | self-extend |
| `bench/BENCH.md` (MOD) | doc | — | n/a (prose) | n/a |
| `bench/runners/mode_resolver_test.go` (EXT) | test | request-response | `TestModeResolverYourAgentFull` (existing) | self-extend |
| `bench/runtime/cell_test.go` (EXT) | test | request-response | `fairness_contract_test.go` table-test style | role-match |

If Claude's-discretion adds a frontmatter field (e.g. `disabled_subsystem`), `bench/runners/mode_resolver.go` also changes (one struct-field add at lines 35-38) — see Shared Pattern "Strict frontmatter struct".

## Pattern Assignments

### `bench/runners/<mode>/MODE.md` ×5 (config, transform)

**Analog:** `bench/runners/your_agent_full/MODE.md` (the P77 seed; full file, 16 lines).

**Exact frontmatter to mirror** (`your_agent_full/MODE.md:1-4`):
```markdown
---
mode: your_agent_full
profile: bench-full
---
```
Two keys ONLY — `mode` and `profile`. These are the only keys `modeFrontmatter` (`mode_resolver.go:35-38`) declares, and the parser runs `dec.KnownFields(true)` (`mode_resolver.go:119`), so any third key is a hard parse error unless the struct is extended first.

**Per-mode frontmatter values** (profile names verified present in `internal/profile/profiles/`):
| MODE.md dir | `mode:` | `profile:` |
|-------------|---------|-----------|
| `baseline_plain/` | `baseline_plain` | `baseline` (D-01; NOT a new `bench-*` yaml — ABLATE-03 forbids one) |
| `no_lsp/` | `no_lsp` | `bench-no-lsp` |
| `no_structured_edit/` | `no_structured_edit` | `bench-no-structured-edit` |
| `your_agent_no_semantic/` | `your_agent_no_semantic` | `bench-no-semantic` (prose: deferred to Phase 81) |
| `baseline_rag/` | `baseline_rag` | (stub — see fail-closed pattern below) |

**Prose body pattern** (mirror `your_agent_full/MODE.md:6-16` — H1 == mode name, then a paragraph naming the resolved profile + the subsystem it ablates). For `your_agent_no_semantic` the prose MUST document the deferred-guarantee (D-03 belt-and-suspenders; Pitfall 4): the row is partial until Phase 81's `disable_semantic_subsystem` kernel flag lands.

**Constraint — `validateModeName` (`mode_resolver.go:54-62`):** new mode dir names must pass `filepath.Clean(mode)==mode`, no `/` or `\`, no leading dot. All five proposed names satisfy this.

**baseline_rag stub-marker variant (D-02):** prefer a `MODE.md`-detectable stub signal over a hard-coded `"baseline_rag"` string in `cell.go` (Open Q3 recommendation). If a stub flag is added to frontmatter, it MUST be added to `modeFrontmatter` (see Shared Pattern below). `baseline_rag/MODE.md` prose documents "real RAG arm deferred to Phase 83 (ABLATE-04)".

---

### `bench/runtime/result.go` (model, transform) — additive `ablation_status`

**Analog:** itself — copy how `model_id` / `mode` are projected.

**`ResultInput` field add** (mirror the open-provenance fields at `result.go:53-58`):
```go
// In ResultInput (result.go:28-69):
AblationStatus string // "" for honest modes; "guarantee_pending_phase_81" for no_semantic (D-03)
```

**`resultDoc` field add** (mirror the open-prop fields `Outcome`/`TraceRef`/`ModelID` at `result.go:104-106`, but use `omitempty` so honest modes emit nothing):
```go
// In resultDoc (result.go:90-114):
AblationStatus string `json:"ablation_status,omitempty"`
```

**`BuildResult` wiring** (mirror `result.go:141-155` doc-literal assignment — add one line):
```go
doc := resultDoc{
    // ... existing fields ...
    ModelID:        in.Fairness.ModelID, // result.go:152 — the projected effective config
    AblationStatus: in.AblationStatus,   // NEW (D-03)
    // ...
}
```
Schema is open at the top level (no major bump — see Shared Pattern "Additive schema field"). The existing `Validate(resultBytes)` gate (`result.go:185+`) passes unchanged.

---

### `bench/runtime/cell.go` (service, request-response) — fairness gate + fail-close + ablation_status

**Analog:** itself — the `RunCell` numbered steps. Insert immediately after step (1) profile resolution at `cell.go:247-256`.

**(D-04) startup fairness gate** — insert right after `cell.go:256` (after `profileName` resolved, before sandbox at 258):
```go
// [NEW D-04] fail-closed fairness gate — pure, unconditional, CI-cheap.
if verr := runners.DefaultContract.Validate(); verr != nil {
    return res, fmt.Errorf("bench/runtime: fairness contract invalid: %w", verr)
}
```
`runners` is already imported (`result.go:16`; `cell.go` calls `runners.ResolveProfile` at 250-252 and `runners.DefaultContract` at 527). Recommendation (Open Q1): call unconditionally; `cfg.Agent` is available if gating on `--agent=claude` is preferred.

**(D-02) baseline_rag fail-close (NO row)** — insert before sandbox (`cell.go:258`), after the gate. Return a `CellResult` carrying a new `Deferred`/`SkippedReason` signal, BEFORE `benchsandbox.New`, `StartDaemon`, `BuildResult`, or `writeDurable`:
```go
// [NEW D-02] baseline_rag is a registered fail-closed stub — NO result.v2 row.
if isDeferredStub(cfg.Mode /* or a MODE.md stub marker */) {
    res.Deferred = true        // NEW CellResult field (cell.go:114-133)
    res.DeferredReason = "baseline_rag: real RAG arm deferred to Phase 83 (ABLATE-04)"
    return res, nil            // no sandbox, no daemon, no row
}
```
New `CellResult` fields mirror the existing bool/string signal fields at `cell.go:123-133` (`ResultValid bool`, `ResultPath string`). `runOneCell` success today is `err==nil && res.VerifyExitCode==0 && res.ResultValid` (`matrix.go:272`) — a deferred stub naturally fails that (ResultValid stays false); add a distinct outcome flag so it is neither success nor infra-error (Pitfall 2).

**(D-03) ablation_status on no_semantic row** — at step (9), where `BuildResult` is called (`cell.go:~517-537`, `Fairness: runners.DefaultContract` is set at `cell.go:527`, `TraceRef: res.MergedTracePath` at 526):
```go
resultBytes, err := BuildResult(ResultInput{
    // ... existing fields (cell.go:519-535) ...
    Fairness:       runners.DefaultContract, // cell.go:527 (existing)
    AblationStatus: ablationStatusFor(cfg.Mode), // NEW: "guarantee_pending_phase_81" iff your_agent_no_semantic
})
```

---

### `bench/runtime/matrix.go` (service, batch) — post-run 3-delta pass

**Analog:** `RunMatrix`/`dispatch`/`Summary` (`matrix.go:160-221`). The delta pass runs AFTER `wg.Wait()` (`matrix.go:212`) — the matrix is the ONLY tier that sees all modes for a task (the cell sees one mode). Either a new post-`RunMatrix` step in `cmd/helix-bench`'s `runBench`, or a new function invoked after `dispatch` returns.

**Read the rows back via `Summary.Outcomes[].Result.ResultPath`** (`matrix.go:78-82` `Summary{Total, Succeeded, Outcomes}`; `CellOutcome.Result.ResultPath` is set at `cell.go:242`). Group by `oc.Cell.Task`; for each task with all 4 real-mode rows present, compute exactly 3 deltas:
- `your_agent_full − baseline_plain`
- `your_agent_full − no_lsp`
- `your_agent_full − no_structured_edit`

**Skip tasks missing any of the 4 modes** (log + continue — Pitfall 3). Write deltas back into each per-mode row under a new open property (e.g. `ablation_deltas`) and re-validate via `runtime.Validate` (Open Q2); optionally also a sibling `<OutDir>/<task>/deltas.json`. Read rows with `encoding/json` (already imported `matrix.go:18` / `result.go:12`).

**Constraint:** single-run, 3-fixed-deltas only. Do NOT grow into the Phase 82 aggregator (BCa/pass@k/variance) — D-05, Anti-Pattern.

---

### `bench/runners/contract_test.go` (test, request-response) — NEW unconditional D-04 contract test

**Analog:** `bench/runners/fairness_contract_test.go` (same package `runners`, same `testing` style).

**Mirror `TestDefaultContractValidates`** (`fairness_contract_test.go:152-156`) for the unconditional gate:
```go
func TestDefaultContractValidates(t *testing.T) {
    if err := DefaultContract.Validate(); err != nil {
        t.Fatalf("committed DefaultContract fails Validate(): %v", err)
    }
}
```

**Mirror the table-test of `TestEmptyWaiverReasonFatal`** (`fairness_contract_test.go:16-75`) for per-runner effective-config assertions. For the projected config (Open Q1 scope A — recommended), assert `model_id == DefaultContract.ModelID` (the value `BuildResult` projects at `result.go:152`) for every registered mode, plus `Validate()` clean. Document in the test that live-agent enforcement of temperature/max_tokens/system_prompt is future (Pitfall 5: `internal/eval/agent/claude.go:76-89` `buildArgv` sets none of them today).

**Reuse `TestSystemPromptHashMatches`** (`fairness_contract_test.go:138-148`) as the existing drift gate — do not duplicate it; the new test sits alongside.

---

### Test extensions

**`bench/runners/mode_resolver_test.go`** — mirror `TestModeResolverYourAgentFull` (asserts `ResolveProfile("your_agent_full") == "bench-full"`): add one assertion per new mode (`baseline_plain→baseline`, `no_lsp→bench-no-lsp`, `no_structured_edit→bench-no-structured-edit`, `your_agent_no_semantic→bench-no-semantic`). The strict-frontmatter test `TestModeResolverFromRootStrictFrontmatter` already guards unknown-key rejection.

**`bench/runtime/cell_test.go`** — table-test style from `fairness_contract_test.go`: (a) fairness gate fatal on a bad-override contract (mirror `TestEmptyWaiverReasonFatal` cases); (b) `baseline_rag` produces no `result.v2.json` + a deferred outcome; (c) `your_agent_no_semantic` row carries `ablation_status: guarantee_pending_phase_81` and still passes `runtime.Validate`.

**`bench/runtime/` delta test** — feed 4 synthetic mode rows; assert the 3 deltas' arithmetic and that they land in each per-mode row.

## Shared Patterns

### Strict frontmatter struct (`KnownFields(true)`)
**Source:** `bench/runners/mode_resolver.go:35-38` (`modeFrontmatter`) + `:119` (`dec.KnownFields(true)`).
**Apply to:** all 5 new `MODE.md` files; `mode_resolver.go` IFF a discretion field is added.
```go
type modeFrontmatter struct {
    Mode    string `yaml:"mode"`
    Profile string `yaml:"profile"`
    // Any new key (e.g. DisabledSubsystem `yaml:"disabled_subsystem"`) MUST be
    // added HERE first and kept optional — else KnownFields(true) hard-fails parse.
}
```
**Rule:** keep the 5 new dirs to `mode`+`profile` for the "zero Go change" path; the moment any extra frontmatter key is introduced, add the optional struct field and re-run `go test ./bench/runners/`.

### Path-segment validation before FS join
**Source:** `bench/runners/mode_resolver.go:54-62` (`validateModeName`); `bench/runtime/matrix.go:92-94` (`validateMatrixID`).
**Apply to:** every new mode dir name. All five proposed names pass; no code change needed — the gate already runs in `ResolveProfileFromRoot` (`mode_resolver.go:85`) and `ExpandMatrix` (`matrix.go:128-132`).

### Fairness contract single source
**Source:** `bench/runners/fairness_contract.go:84-100` (`DefaultContract`) + `:107-114` (`Validate()`).
**Apply to:** the cell startup gate (D-04) and the CI contract test. `Validate()` returns non-nil iff any override has an empty/whitespace `WaiverReason`; `DefaultContract` has none, so the gate never fatals under the committed contract (`TestDefaultContractValidates`).

### Additive schema field (no major bump)
**Source:** `bench/schema/result.v2.schema.json:6-8` ($comment: additive-only is MINOR; top-level `additionalProperties` is OPEN — verified no `additionalProperties: false` at the root object on line 7). Existing `mode` (`:18-21`) and `fairness.overrides[]` (`:51-76`) are the field-definition templates.
**Apply to:** `ablation_status` (D-03) and `ablation_deltas` (D-05). Both land as OPTIONAL top-level props with NO `schema_version` bump. Documenting `ablation_status` in `properties` is optional and still additive.

### Result projection / open-provenance props
**Source:** `bench/runtime/result.go:53-58` (`ResultInput` open props), `:104-106` (`resultDoc` open props with snake_case json tags), `:141-155` (`BuildResult` doc literal).
**Apply to:** the `AblationStatus`/`ablation_deltas` additions — mirror the `Outcome`/`TraceRef`/`ModelID` pattern (typed field, snake_case json tag, set in the `BuildResult` literal).

## No Analog Found

None. Every file in this phase has an in-repo analog or extends an existing file. This phase is "growth, not invention" — the danger is re-inventing substrate, not the absence of patterns. (`bench/runners/deltas.go` is genuinely new code but follows the pure-transform style of `BuildResult` and the single-run 3-delta scope of D-05.)

## Metadata

**Analog search scope:** `bench/runners/`, `bench/runtime/`, `bench/schema/`, `internal/profile/profiles/`, `internal/eval/agent/`.
**Files scanned (read at file:line this session):** `your_agent_full/MODE.md`, `mode_resolver.go`, `fairness_contract.go`, `fairness_contract_test.go`, `result.go` (1-165), `cell.go` (114-138, 240-350), `matrix.go` (60-150, 160-274), `result.v2.schema.json` (1-80).
**Pattern extraction date:** 2026-06-19
