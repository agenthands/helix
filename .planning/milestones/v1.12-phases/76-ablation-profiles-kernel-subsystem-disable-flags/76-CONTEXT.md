# Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags - Context

**Gathered:** 2026-06-16
**Status:** Ready for planning

<domain>
## Phase Boundary

The only **invasive** code paths inside the daemon in milestone v1.12. This phase lands:
- 2 kernel-level subsystem-disable flags — `disable_lsp_subsystem`, `disable_structured_edit_subsystem`
  (the `disable_semantic_subsystem` flag, ABLATE-06, is **intentionally deferred to Phase 81** — it
  depends on un-wiring the v1.10 Phase 65 SemanticLookup seam cleanly).
- 4 new bench profile YAMLs under `internal/profile/profiles/`:
  `bench-full.yaml`, `bench-no-lsp.yaml`, `bench-no-semantic.yaml`, `bench-no-structured-edit.yaml`,
  with profile-filter golden tests covering each and a loader that rejects unknown mode names.
- A `vet-ablation-leakage` static analyzer in `internal/lint/`, wired into `make vet`.

These must be stable BEFORE any downstream phase consumes them (Phase 77 launches the daemon
subprocess with `--profile=bench-full`; ablation runners land in Phase 80; `no_semantic` is consumed
in Phase 81+). This phase delivers ablation **infrastructure**, not benchmark adapters or runners.

**Requirements locked here:** ABLATE-02, ABLATE-05, ABLATE-07, ABLATE-08 (see `.planning/REQUIREMENTS.md`).
WHAT/WHY are locked upstream in REQUIREMENTS.md and v1.12-ROADMAP.md; the decisions below are the HOW.
</domain>

<decisions>
## Implementation Decisions

### Profile ↔ kernel-flag coupling (ABLATE-02, ABLATE-05, ABLATE-07)
- **D-01:** **Hybrid coupling.** Each bench profile YAML *declares* its kernel disable flags as
  first-class fields (e.g. `disable_lsp_subsystem: true`, `disable_structured_edit_subsystem: true`),
  AND the daemon accepts a CLI override for ad-hoc runs without editing a profile. The mode YAML is
  the single reviewable artifact that fully describes an ablation arm (tool surface + subsystem
  flags); the golden tests can therefore cover the whole arm from one source. This mirrors how
  `baseline.yaml` fully describes its arm via empty `skills`/`tools` lists.
- **D-02:** **Precedence: CLI override > profile YAML > default-off.** Matches the project's existing
  4-layer config precedence (CLI flags > project `.helix/` > user `~/.helix/` > profile defaults,
  per CLAUDE.md). Default state (no flag set) is the subsystem ENABLED — flags are opt-in disables.
- **D-03:** Resolution happens at the **daemon composition root** (`internal/daemon/daemon.go`, where
  the profile is already resolved via `skill.ResolveTools(...)`). The daemon reads the resolved
  profile's disable fields (and any CLI override) and configures the kernel from there — keeping the
  kernel-config concern owned by the daemon, not leaked into `internal/profile/`. Exact kernel
  config-struct threading is planner/researcher discretion.

### Disabled structured-edit tool behavior (ABLATE-07, roadmap criterion #3)
- **D-04:** **Defense-in-depth — filter AND guard.** The `bench-no-structured-edit.yaml` profile
  **excludes** `replace_symbol_body`, `fuzzy_edit`, `insert_before_symbol`, `insert_after_symbol`
  from `tools/list` (satisfies ABLATE-07: "tool inventory excludes structured edits; agent receives
  plain `replace_in_file` only"). Independently, the kernel `disable_structured_edit_subsystem` flag
  makes those tool paths **return a typed `Unsupported` error with a documented kind** if reached via
  any back-channel (satisfies roadmap criterion #3 and gives `vet-ablation-leakage` a concrete
  enforcement floor to point at). The two layers reconcile the apparent ABLATE-07 ↔ roadmap-#3
  tension rather than satisfying either vacuously.
- **D-05:** Under `disable_structured_edit_subsystem`, `replace_in_file` **falls through to
  exact-match only** (no fuzzy cascade) — per roadmap criterion #3.
- **D-06:** The exact `Unsupported` error-kind string/constant and its documentation location are
  **Claude's discretion** (suggest a stable, greppable kind such as `subsystem_disabled`).

### vet-ablation-leakage analyzer (ABLATE-08)
- **D-07:** **Import-boundary analyzer**, modelled on `internal/lint/noduckdb` and
  `nokernel2semantic`/`nosemantic2kernel`: it fails the build if an ablation-gated tool package
  imports a disabled-subsystem package it must not reach. Ships with a **deliberate violating import
  in `testdata/`** so a regression makes `make vet` fail with a clear diagnostic. Wired into the
  `make vet` target alongside the existing `-vettool=` analyzers.
- **D-08:** **Honest scope (recorded, not hidden):** the analyzer catches **compile-time import
  leakage**, not every runtime path. This is the deliberate v1 scope — consistent with Phase 75
  D-08's philosophy of shipping a `vet-*` analyzer scoped to what it can concretely catch. Runtime
  path precision (call-site guard verification) is explicitly out of scope for Phase 76 and is the
  fallback if a real runtime leak later appears. The kernel `Unsupported` guard (D-04) is the
  runtime backstop the analyzer does not cover.

### no_lsp subsystem no-op enforcement (ABLATE-05, roadmap criterion #2)
- **D-09:** **Null-object injection at daemon init.** When `disable_lsp_subsystem` is set, the daemon
  (at the composition root) wires a **no-op `EditNotifier`**, **skips `SetEnrichFn`**
  (`daemon.go:661`), and does **not inject the LS pool** — so `lspProbeForEdges`
  (`internal/kernel/symbols/`), RepoMap enrichment, and the `OnEdit` live-update hooks
  (`internal/kernel/fileops/tools.go`) all become no-ops by construction. This structurally
  guarantees the roadmap criterion #2 hard-fail ("zero `lsp.*` OTel spans") because there is no live
  LSP to emit spans, rather than relying on scattered per-callsite flag checks.
- **D-10:** Planner must **audit the pool-nil / notifier-nil degrade-gracefully paths** so the
  no-op wiring doesn't crash tools that expect a live pool handle (the `notifier.go` contract
  already mandates a nil-check before `OnEdit` — confirm the analogous LS-pool paths).

### no_semantic profile content in Phase 76 (ABLATE-02, boundary with ABLATE-06/Phase 81)
- **D-11:** **`bench-no-semantic.yaml` is a real tool-filter arm now; kernel flag in Phase 81.**
  In Phase 76 it **excludes the semantic-store-backed tools** (the v1.11 P1 tools that read the
  duckdb store — `find_references`/`get_reachability`/`get_neighborhood`/etc.) at the profile level,
  so it is a meaningful, golden-testable ablation arm immediately. The kernel
  `disable_semantic_subsystem` back-channel guard (ABLATE-06) is added in Phase 81. This matches the
  "profile filter + kernel guard" shape of the other arms, just split across two phases by the
  Phase 65 un-wiring dependency.
- **D-12:** Planner must NOT attempt to wire `disable_semantic_subsystem` in Phase 76 — only the
  LSP and structured-edit kernel flags land here. The exact list of "semantic-store-backed" tools to
  exclude in `bench-no-semantic.yaml` is researcher/planner work (cross-check against the v1.11 P1
  tool inventory and which tools actually read the duckdb store vs. tree-sitter).

### Claude's Discretion
- The `Unsupported` error-kind constant name + where it's documented (D-06).
- CLI override flag naming/shape for the kernel disables (D-01).
- Internal kernel config-struct threading for the two flags (D-03).
- Exact semantic-store-backed tool list excluded by `bench-no-semantic.yaml` (D-12).
- Golden-test structure and how the 4 bench YAMLs share common config (DRY vs explicit).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & requirements (locked WHAT/WHY — do not re-derive)
- `.planning/REQUIREMENTS.md` — ABLATE-02, ABLATE-05, ABLATE-07, ABLATE-08 acceptance criteria
  (lines 46, 49, 51, 52). Note the broader ABLATE-03/04/06 rows for boundary awareness:
  ABLATE-06 (`disable_semantic_subsystem`) is Phase 81; ABLATE-03/04 are Phases 80/83.
- `.planning/milestones/v1.12-ROADMAP.md` §"Phase 76" (lines 60–70) — Goal, Depends-on,
  Requirements, and the **4 Success Criteria** this phase is graded against.
- `.planning/ROADMAP.md` §"Phase 76" — abbreviated active-roadmap entry (promoted from the
  milestone roadmap on 2026-06-16 to unblock discuss/plan tooling).
- `.planning/PROJECT.md` (v1.12 milestone section) — milestone scoping.

### Reused-pattern source (Depends-on)
- `internal/profile/profiles/baseline.yaml` — the Phase 67 control-arm profile; documents the
  EMPTY-LISTS mechanism (`ProfileFilterMiddleware` checks `AllowedTools != nil`) that the 4 bench
  YAMLs build on for tool filtering. The closest analog for the new profiles.
- `internal/lint/noduckdb/analyzer.go` — the import-boundary analyzer pattern (`pass.Pkg.Path()`
  allowlist + forbidden-import check) that `vet-ablation-leakage` should mirror (D-07).
- `internal/lint/{nokernel2semantic,nosemantic2kernel,compactusesstore}` — sibling boundary
  analyzers; the `make vet` `-vettool=` wiring precedent (Makefile lines 33–38).
- `internal/kernel/notifier.go` — the `EditNotifier.OnEdit` contract + the nil-check-before-invoke
  rule (lines 48–51); the no-op injection (D-09) builds on this.
- `internal/kernel/symbols/blast_radius_strangler.go:151` + `internal/kernel/symbols/tools.go:719`
  — `lspProbeForEdges`, the kernel-side LSP probe that must become a no-op under `no_lsp`.
- `internal/daemon/daemon.go:601,661` — composition-root wiring of the `EditNotifier.OnEdit` hook
  and `SetEnrichFn`; where D-03/D-09 daemon-side configuration happens.
- `internal/kernel/fileops/tools.go:264,448,464,520` — the `OnEdit` call sites on edit-tool
  success paths (no-op under `no_lsp`).

### Milestone research (architectural verdicts)
- `.planning/research/SUMMARY.md`, `ARCHITECTURE.md`, `PITFALLS.md` — milestone-level verdicts and
  known failure modes to design the ablation flags around.

### Prior-phase context
- `.planning/phases/75-schema-fairness-contract-tree-skeleton/75-CONTEXT.md` — D-08 lint philosophy
  (ship `vet-*` analyzers only when they catch something concrete) carried forward into D-07/D-08.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/profile/profiles/baseline.yaml` — template for the 4 new bench YAMLs; its header
  documents the empty-list filtering mechanism and the `default_mode`/`guardrails` fields.
- `internal/lint/noduckdb/` (`analyzer.go`, `analyzer_test.go`, `testdata/`) — copy-shape source
  for `vet-ablation-leakage`: an `analysis.Analyzer` with an allowlist-prefix + forbidden-import
  check, plus a `testdata/` package carrying a deliberate violation.

### Established Patterns
- **Import-boundary lint** (`nokernel2semantic`, `nosemantic2kernel`, `noduckdb`, `compactusesstore`)
  — the house pattern for structural enforcement; `vet-ablation-leakage` joins it (D-07).
- **`make vet` `-vettool=` chaining** (Makefile 33–38) — add a `VETTOOL_ABLATION_LEAKAGE` and a new
  `$(GO) vet -vettool=...` line.
- **Profile resolution at daemon init** — `resolved := skill.ResolveTools(profile.Skills,
  profile.Tools, profile.ExcludeTools)` then `session.SetAllowedTools(...)` (per baseline.yaml
  header); the disable-flag read (D-03) hooks in alongside this.
- **Nil-checked fire-and-forget notifier** (`notifier.go` 48–51) — the no-op `EditNotifier`
  injection (D-09) is consistent with this contract.

### Integration Points
- `internal/profile/profiles/` — 4 new YAMLs land here; loader gains unknown-mode-name rejection.
- `internal/daemon/daemon.go` — composition root reads disable flags and configures the kernel
  (null-object wiring for `no_lsp`; structured-edit guard plumbing).
- `internal/kernel/` — the two flags live on a kernel config/options struct (exact shape TBD);
  `symbols/` (lspProbe) and the structured-edit tools (`replace_symbol_body`, `fuzzy_edit`,
  `insert_*`) read the structured-edit flag to return `Unsupported`.
- `internal/lint/ablationleakage/` (new) + root `Makefile` `vet` target.

</code_context>

<specifics>
## Specific Ideas

- The 4 bench YAMLs are named `bench-full.yaml`, `bench-no-lsp.yaml`, `bench-no-semantic.yaml`,
  `bench-no-structured-edit.yaml` (roadmap criterion #1 — exact filenames).
- Default subsystem state is ENABLED; flags are opt-in disables (D-02) — `bench-full.yaml` sets none.
- `vet-ablation-leakage` testdata should carry a deliberate forbidden import so a green→red flip is
  demonstrable (roadmap criterion #4 / ABLATE-08 acceptance).
</specifics>

<deferred>
## Deferred Ideas

- **`disable_semantic_subsystem` kernel flag (ABLATE-06)** — Phase 81. Depends on un-wiring the
  v1.10 Phase 65 SemanticLookup seam. Only the `bench-no-semantic.yaml` *profile* (tool filtering)
  ships in Phase 76 (D-11/D-12).
- **Call-site guard verification in `vet-ablation-leakage`** — out of scope for Phase 76; the
  import-boundary check (D-07) is the v1. Revisit only if a real runtime (non-import) leak appears
  past the kernel `Unsupported` guard (D-08).
- **`baseline_plain` (ABLATE-03, Phase 80) and `baseline_rag` (ABLATE-04, Phase 83)** — not bench
  Helix profiles in the same sense; explicitly other phases. Not in Phase 76 scope.

None of the above expand Phase 76 scope — discussion stayed within the ablation-infrastructure boundary.

### Planner notes (apply during plan-phase)
1. **Phase 76 lands exactly TWO kernel flags** (`disable_lsp_subsystem`,
   `disable_structured_edit_subsystem`) — NOT `disable_semantic_subsystem` (Phase 81).
2. **Four bench YAMLs ship + golden tests + unknown-mode-name rejection** (ABLATE-02). `bench-full`
   sets no disable flags; `bench-no-semantic` is tool-filter-only this phase (D-11).
3. **vet-ablation-leakage** must be wired into `make vet` with a deliberate testdata violation
   (ABLATE-08 / roadmap #4) — model on `internal/lint/noduckdb`.
4. **no_lsp = null-object injection at daemon init** (D-09); prove zero `lsp.*` spans via the
   trace-tap (roadmap #2 hard-fail). Audit pool-nil/notifier-nil degrade paths (D-10).
5. **Structured-edit disable = filter + guard** (D-04/D-05); `replace_in_file` exact-match-only
   under the flag.
</deferred>

---

*Phase: 76-ablation-profiles-kernel-subsystem-disable-flags*
*Context gathered: 2026-06-16*
