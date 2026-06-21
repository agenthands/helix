---
phase: 76-ablation-profiles-kernel-subsystem-disable-flags
verified: 2026-06-16T17:10:18Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
---

# Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags Verification Report

**Phase Goal:** Land the only invasive code paths inside the daemon (kernel-level `disable_lsp_subsystem` / `disable_structured_edit_subsystem` flags) plus the 4 new bench profile YAMLs, with a static `vet-ablation-leakage` analyzer so they are stable before any downstream phase consumes them. `no_semantic` flag is intentionally deferred to Phase 81.
**Verified:** 2026-06-16T17:10:18Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

The 4 ROADMAP Success Criteria are the graded contract. Each maps to ABLATE requirement IDs and was verified by running the named tests and reading the actual source — not from SUMMARY claims.

| # | Truth (ROADMAP SC) | Status | Evidence |
| --- | --- | --- | --- |
| 1 | 4 bench YAMLs ship under `internal/profile/profiles/`; profile-filter golden tests cover each; loader rejects unknown mode names (ABLATE-02) | ✓ VERIFIED | `ls` shows all 4 YAMLs (`bench-full/no-lsp/no-semantic/no-structured-edit`). `go test ./internal/profile/ -count=1` → `ok`. `TestBenchProfiles` (bench_profiles_test.go:81) asserts each arm's resolved tool surface substantively (bench-no-semantic excludes all 10 semantic-store tools; bench-no-structured-edit excludes the 4 structured-edit tools + keeps replace_in_file; bench-no-lsp excludes LSP-backed tools). `TestLoaderRejectsUnknownMode` (loader_test.go:125) + `ProfileStore.Validate()` (loader.go:44) invoked from `LoadEmbedded` (loader.go:28). |
| 2 | With `disable_lsp_subsystem: true`, no_lsp emits ZERO `lspool.lsp.*` OTel spans (hard fail); `SetEnrichFn`, OnEdit hooks, lspProbe are no-ops (ABLATE-05) | ✓ VERIFIED | `go test ./internal/daemon/ -run 'TestNoLSP' -count=1` → all 3 PASS. `TestNoLSPZeroSpans` (no_lsp_wiring_test.go:86) iterates recorded spans and fails on any `lspool.lsp.` prefix. `TestNoLSPWiring` asserts `EditNotifier()==nil`, `HasEnrichFn()==false`, `HasFallbackDeps()==false` under the flag. Daemon wiring confirmed at daemon.go:287-301 (effective `cfg||profile` flags into KernelConfig), :380/:707-710 (SetEnrichFn(nil) under effDisableLSP), :731-734 (SetFallbackDeps(nil) — D-10 audit). `TestNoLSPDefaultArmUnchanged` proves the default arm still wires enrichment (additive). |
| 3 | With `disable_structured_edit_subsystem: true`, the 4 structured-edit tools return `Unsupported` with a documented kind; `replace_in_file` falls through to exact-match only (ABLATE-07) | ✓ VERIFIED | `go test ./internal/kernel/edit/ ./internal/kernel/fileops/ -count=1` → `ok`. 4 `subsystem_disabled:` guards present: edit/tools.go:337 (replace_symbol_body), :437 (insert_before_symbol), :515 (insert_after_symbol), fileops/tools.go:509 (fuzzy_edit). `replace_in_file` exact-match-only guard at fileops/tools.go:418 (`count==0 && !args.IsRegex && !k.StructuredEditDisabled()`). `subsystem_disabled:` convention documented on `serr.Unsupported` (kinds.go, D-06 — no new kind). `TestStructuredEditDisabled` + `TestReplaceInFileNoFuzzyWhenDisabled` + `TestFuzzyEditDisabled` all present and pass. |
| 4 | `vet-ablation-leakage` analyzer in `internal/lint/` wired into `make vet`; a deliberate test-case violation makes it fail with a clear diagnostic (ABLATE-08) | ✓ VERIFIED | `make vet` → green, runs 5 vettools including `vet-ablation-leakage`. `go test ./internal/lint/ablationleakage/ -count=1` → 3 cases PASS (badrunner fires, goodrunner silent, siblingrunner slash-boundary control). Forbidden edge correctly pinned `bench/runners → {internal/kernel/lspool, internal/semantic/store}` (analyzer.go:37,41-43); `internal/fuzzy` appears ONLY in the doc comment, NOT as a forbidden prefix (Pitfall 5 respected, D-07/D-08). Makefile VETTOOL_ABLATION_LEAKAGE var + prereq + vettool line + install rule (Makefile:24,40,46,60). `testdata` violation carries the `// want` comment and lives only under `testdata/` (real tree `go vet` clean). |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/kernel/kernel.go` | 2 disable-flag fields + 2 accessors | ✓ VERIFIED | Fields at :24,:29; accessors `StructuredEditDisabled()`:77, `LSPSubsystemDisabled()`:82 |
| `internal/kernel/edit/tools.go` | Unsupported guards on 3 edit tools | ✓ VERIFIED | Guards at :337,:437,:515 with `subsystem_disabled:` prefix |
| `internal/kernel/fileops/tools.go` | fuzzy_edit guard + replace_in_file exact-match-only | ✓ VERIFIED | :509 guard, :418 fuzzy-fallback gate |
| `internal/errors/kinds.go` | `subsystem_disabled:` convention on Unsupported | ✓ VERIFIED | Documented; reuses existing kind (D-06) |
| `internal/profile/profiles/bench-*.yaml` (4) | bench arms w/ correct flags + exclusions | ✓ VERIFIED | All 4 present; bench-no-lsp sets disable_lsp_subsystem:true; bench-no-structured-edit sets disable_structured_edit_subsystem:true; bench-full + bench-no-semantic set NEITHER (D-02/D-11) |
| `internal/profile/profile.go` | 2 yaml-tagged Profile fields | ✓ VERIFIED | Decoded + asserted by TestBenchProfiles |
| `internal/profile/loader.go` | Validate() + LoadEmbedded call | ✓ VERIFIED | Validate():44, called :28 |
| `internal/profile/bench_profiles_test.go` | golden per-arm surface tests | ✓ VERIFIED | TestBenchProfiles:81, substantive assertions |
| `internal/lint/ablationleakage/analyzer.go` | import-boundary analyzer | ✓ VERIFIED | Correct architectural edge; passes analysistest |
| `cmd/vet-ablation-leakage/main.go` | singlechecker entrypoint | ✓ VERIFIED | Builds; runs under make vet |
| `internal/cli/root.go` | 2 CLI override flags | ✓ VERIFIED | :75-76 flags, :151-152 read, only-when-set override entries |
| `internal/config/config.go` | 2 koanf SerenaConfig fields | ✓ VERIFIED | :35,:42 |
| `internal/daemon/daemon.go` | effective-flag computation + null-object wiring | ✓ VERIFIED | :287-301,:380,:707-734 |
| `internal/daemon/no_lsp_wiring_test.go` | TestNoLSPWiring + TestNoLSPZeroSpans | ✓ VERIFIED | :60,:86,:100 — all pass |
| `Makefile` | VETTOOL_ABLATION_LEAKAGE wiring | ✓ VERIFIED | :24,:40,:46,:60 |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| daemon.go | kernel.KernelConfig | effective `(cfg||profile)` flags at NewKernel | ✓ WIRED | daemon.go:287-301 |
| daemon.go | rs.SetEnrichFn | conditional skip/clear under effDisableLSP | ✓ WIRED | daemon.go:707-710 (SetEnrichFn(nil)) |
| daemon.go | rs.SetFallbackDeps | D-10 audit neutralization | ✓ WIRED | daemon.go:731-734 |
| cli/root.go | config.SerenaConfig | overrides map (only-when-set) | ✓ WIRED | root.go:151-152 |
| loader.go | ProfileStore.Validate() | LoadEmbedded validation call | ✓ WIRED | loader.go:28 |
| cmd/vet-ablation-leakage | ablationleakage.Analyzer | singlechecker.Main | ✓ WIRED | main.go |
| Makefile vet | vet-ablation-leakage binary | go vet -vettool | ✓ WIRED | Makefile:46 |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Module builds | `go build ./...` | exit 0 | ✓ PASS |
| Profile golden + loader tests | `go test ./internal/profile/ -count=1` | ok | ✓ PASS |
| Kernel/edit/fileops guards | `go test ./internal/kernel/ ./internal/kernel/edit/ ./internal/kernel/fileops/ -count=1` | ok (3 pkgs) | ✓ PASS |
| no_lsp zero-span trace-tap | `go test ./internal/daemon/ -run 'TestNoLSP' -count=1` | 3/3 PASS | ✓ PASS |
| Analyzer green→red fixture | `go test ./internal/lint/ablationleakage/ -count=1` | 3/3 PASS | ✓ PASS |
| make vet (5 analyzers green) | `make vet` | exit 0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| ABLATE-02 | 76-02 | 4 bench mode YAMLs ship; golden tests; loader rejects unknown mode | ✓ SATISFIED | SC #1 verified |
| ABLATE-05 | 76-04 | Kernel `disable_lsp_subsystem` prevents all back-channel LSP; zero `lsp.*` spans | ✓ SATISFIED | SC #2 verified |
| ABLATE-07 | 76-01 | Kernel `disable_structured_edit_subsystem`; tools return Unsupported; replace_in_file exact-only | ✓ SATISFIED | SC #3 verified |
| ABLATE-08 | 76-03 | `vet-ablation-leakage` wired into make vet; deliberate violation fails | ✓ SATISFIED | SC #4 verified |
| ABLATE-06 | — (Phase 81) | `disable_semantic_subsystem` kernel flag | ⏭ DEFERRED | Intentionally out of scope (D-11/D-12, CONTEXT line 11). bench-no-semantic ships as tool-filter-only arm this phase. Not a Phase 76 gap. |

All 4 Phase 76 requirement IDs from PLAN frontmatter (ABLATE-02/05/07/08) are accounted for, each claimed exactly once, each satisfied. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| — | — | — | — | None. No TBD/FIXME/XXX in modified source. The diag leaseFn `serr.Unsupported` stub and SetEnrichFn(nil)/SetFallbackDeps(nil) are deliberate null-object ablation behavior (documented D-09/D-10), not stubs. The `bench/runners` package not existing yet is intentional (Phase 80). |

### Deferred / Out-of-Scope (correctly handled, not gaps)

- **ABLATE-06 / `disable_semantic_subsystem` kernel flag** — Phase 81 (D-11/D-12). `bench-no-semantic.yaml` ships as a tool-filter-only arm with NO kernel flag, verified.
- **`test/bench` failures** (`TestBenchToolsManifestMatchesRegistry`, `TestToolDescriptionsGoldenFile`, 53-vs-47 tool drift) — PRE-EXISTING. `test/bench` last touched by Phase 66 commit `3c80abab`; no Phase 76 commit touches it; Phase 76 adds zero MCP tools. Logged in `.planning/deferred-items.md`. NOT a Phase 76 regression.
- **Analyzer compile-time-only scope** (D-08) — intentional v1 scope; kernel `Unsupported` guard is the runtime backstop.

### Human Verification Required

None. All phase behaviors have automated verification (per 76-VALIDATION.md, the Manual-Only table is empty). The green→red `make vet` flip is proven non-manually via the `analysistest` `// want` fixture; the zero-span hard fail is proven via the in-memory trace-tap exporter.

### Gaps Summary

No gaps. All 4 ROADMAP Success Criteria and both hard-fail gates (zero `lspool.lsp.*` spans under no_lsp; green→red `make vet` flip) are verified against the actual codebase via real command execution and source reading. All locked CONTEXT decisions honored:
- TWO kernel flags only (LSP + structured-edit), NOT three — confirmed (no `disable_semantic_subsystem` field).
- Analyzer forbidden edge is `bench/runners → lspool|store`, NOT `kernel → fuzzy` — confirmed (fuzzy only in doc comment).
- structured-edit = filter (profile excludes 4 tools) + guard (kernel Unsupported) — both confirmed.
- no_lsp = null-object injection at daemon init — confirmed (SetEnrichFn(nil), SetFallbackDeps(nil), skipped buildLiveBundle, neutralized diag leaseFn).

The phase goal — landing the invasive daemon code paths + 4 bench YAMLs + static analyzer, stable before downstream consumption — is achieved.

---

_Verified: 2026-06-16T17:10:18Z_
_Verifier: Claude (gsd-verifier)_
