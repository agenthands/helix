# Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags - Research

**Researched:** 2026-06-16
**Domain:** Go daemon composition-root wiring, profile/tool-filter YAML, go/analysis import-boundary linting, OTel span suppression
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01 (Hybrid coupling):** Each bench profile YAML *declares* its kernel disable flags as first-class fields (e.g. `disable_lsp_subsystem: true`, `disable_structured_edit_subsystem: true`), AND the daemon accepts a CLI override for ad-hoc runs without editing a profile.
- **D-02 (Precedence):** CLI override > profile YAML > default-off. Default state (no flag set) is the subsystem ENABLED — flags are opt-in disables.
- **D-03 (Resolution at daemon composition root):** Resolution happens in `internal/daemon/daemon.go` where the profile is already resolved via `skill.ResolveTools(...)`. The daemon reads the resolved profile's disable fields (and any CLI override) and configures the kernel from there. Exact kernel config-struct threading is planner/researcher discretion.
- **D-04 (Defense-in-depth — filter AND guard):** `bench-no-structured-edit.yaml` **excludes** `replace_symbol_body`, `fuzzy_edit`, `insert_before_symbol`, `insert_after_symbol` from `tools/list`. Independently, the kernel `disable_structured_edit_subsystem` flag makes those tool paths **return a typed `Unsupported` error with a documented kind** if reached via any back-channel.
- **D-05:** Under `disable_structured_edit_subsystem`, `replace_in_file` **falls through to exact-match only** (no fuzzy cascade).
- **D-06 (Claude's discretion):** The exact `Unsupported` error-kind string/constant and its documentation location. Suggested: a stable, greppable kind such as `subsystem_disabled`.
- **D-07 (Import-boundary analyzer):** `vet-ablation-leakage` modelled on `internal/lint/noduckdb` + `nokernel2semantic`/`nosemantic2kernel`. Fails the build if an ablation-gated tool package imports a disabled-subsystem package it must not reach. Ships with a deliberate violating import in `testdata/`. Wired into `make vet` alongside the existing `-vettool=` analyzers.
- **D-08 (Honest scope):** The analyzer catches **compile-time import leakage**, not every runtime path. Runtime path precision is explicitly OUT of scope. The kernel `Unsupported` guard (D-04) is the runtime backstop.
- **D-09 (Null-object injection at daemon init):** When `disable_lsp_subsystem` is set, the daemon wires a **no-op `EditNotifier`**, **skips `SetEnrichFn`**, and does **not inject the LS pool** so `lspProbeForEdges`, RepoMap enrichment, and `OnEdit` hooks all become no-ops by construction.
- **D-10:** Planner must **audit the pool-nil / notifier-nil degrade-gracefully paths** so the no-op wiring doesn't crash tools that expect a live pool handle.
- **D-11 (`bench-no-semantic.yaml` is tool-filter-only this phase):** It excludes the semantic-store-backed tools at the profile level. The kernel `disable_semantic_subsystem` back-channel guard (ABLATE-06) is added in Phase 81.
- **D-12:** Planner must NOT wire `disable_semantic_subsystem` in Phase 76 — only LSP and structured-edit kernel flags land here. The exact semantic-store-backed exclude-list is researcher/planner work.

### Claude's Discretion
- The `Unsupported` error-kind constant name + where it's documented (D-06).
- CLI override flag naming/shape for the kernel disables (D-01).
- Internal kernel config-struct threading for the two flags (D-03).
- Exact semantic-store-backed tool list excluded by `bench-no-semantic.yaml` (D-12).
- Golden-test structure and how the 4 bench YAMLs share common config (DRY vs explicit).

### Deferred Ideas (OUT OF SCOPE)
- **`disable_semantic_subsystem` kernel flag (ABLATE-06)** — Phase 81. Only the `bench-no-semantic.yaml` *profile* (tool filtering) ships in Phase 76.
- **Call-site guard verification in `vet-ablation-leakage`** — out of scope; import-boundary check is the v1.
- **`baseline_plain` (ABLATE-03, Phase 80) and `baseline_rag` (ABLATE-04, Phase 83)** — other phases, not Phase 76 scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ABLATE-02 | 4 mode YAML profiles ship as `internal/profile/profiles/bench-*.yaml`; golden tests cover each; loader rejects an unknown mode name. | Profile struct + loader (`internal/profile/profile.go`, `loader.go`); EMPTY-LISTS filter mechanism (`baseline.yaml`); `resolveAllowedToolsForMode` flow; unknown-mode-rejection lives in loader or a validation pass (§Architecture Pattern 4). |
| ABLATE-05 | Kernel `disable_lsp_subsystem` prevents any back-channel LSP call; zero `lsp.*` OTel spans. | Null-object injection at daemon init (D-09); span emitted as `lspool.lsp.*` at `jsonrpc/conn.go:102`; null-LSP achieved by not injecting the pool / skipping `SetEnrichFn` / no-op `EditNotifier` (§no_lsp Architecture). |
| ABLATE-07 | Kernel `disable_structured_edit_subsystem` forces fall-through to plain unified-diff; structured-edit tools return `unsupported` with a documented kind. | `serr.Unsupported` kind already exists (`internal/errors/kinds.go:17`); guard insertion points in `internal/kernel/edit/tools.go`; `replace_in_file` fuzzy cascade entry at `fileops/tools.go:415-453` (§structured-edit Architecture). |
| ABLATE-08 | `vet-ablation-leakage` static analyzer fails the build if any mode-restricted tool path reaches a disabled subsystem; wired into `make vet`; a deliberate test-case violation makes `make vet` fail. | `internal/lint/noduckdb` copy-shape; `analysistest` already vendored (`go.mod:53`); `make vet` `-vettool=` chaining (Makefile 33-50) (§vet-ablation-leakage Architecture). |
</phase_requirements>

## Summary

Phase 76 is the only invasive-daemon phase in v1.12. It lands three independent deliverables, all of which have strong existing in-repo precedent — there is no novel external technology and **no new third-party packages**. The work is: (1) two boolean flags threaded from a resolved profile (plus a CLI override) at the daemon composition root into the kernel; (2) four new tool-filter profile YAMLs that reuse the Phase 67 EMPTY-LISTS mechanism `baseline.yaml` proved; (3) a go/analysis import-boundary analyzer that is a near-clone of `internal/lint/noduckdb`.

The single most important architectural insight is that **no_lsp is achieved structurally by what the daemon does NOT wire, not by scattered runtime flag-checks.** The LSP-emitting span (`lspool.lsp.*`, emitted in `internal/kernel/jsonrpc/conn.go:102`) only fires when a live LS worker is reached through the pool. If the daemon, under `disable_lsp_subsystem`, (a) wires a no-op `EditNotifier`, (b) skips `SetEnrichFn`, and (c) prevents symbol/diag tools from acquiring a pool lease (by excluding them at the profile layer **and** gating the lease path), then zero `lspool.lsp.*` spans can be emitted because there is no live LSP to emit them. This satisfies roadmap criterion #2's hard-fail by construction (D-09).

The structured-edit disable is defense-in-depth (D-04): the profile excludes the four structured-edit tools from `tools/list`, AND the kernel handlers return `serr.ErrUnsupported` if reached via a back-channel; `replace_in_file` skips its fuzzy fallback block under the flag (D-05). The `Unsupported` error kind **already exists** in `internal/errors/kinds.go` — no new error-taxonomy work is needed beyond a documented sentinel/message convention.

**Primary recommendation:** Land all three deliverables as TDD plans modeled on the cited in-repo precedents. Add `DisableLSPSubsystem bool` and `DisableStructuredEditSubsystem bool` to `kernel.KernelConfig`, populate them at `daemon.go` step 5 from a resolved-profile read + CLI override, add `disable_lsp_subsystem`/`disable_structured_edit_subsystem` fields to `profile.Profile`, add two persistent bool flags to `internal/cli/root.go` (`--disable-lsp-subsystem`, `--disable-structured-edit-subsystem`), and copy `internal/lint/noduckdb` to `internal/lint/ablationleakage` + `cmd/vet-ablation-leakage`. Include a Validation Architecture section (Nyquist enabled) — the OTel-span hard-fail and the green→red vet flip are both first-class testable gates.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Bench profile YAML (tool surface declaration) | Agent Profiles (`internal/profile/`) | — | Profiles own the tool-filter contract (Layer 3); reuses Phase 67 `baseline.yaml` EMPTY-LISTS mechanism. |
| Kernel disable-flag definition | Code Intelligence Kernel (`internal/kernel/`) | — | Flags live on `KernelConfig`; the kernel owns subsystem reachability (Layer 1). |
| Flag resolution (profile + CLI → kernel) | MCP Runtime / Daemon (`internal/daemon/daemon.go`) | Config (`internal/config/`) | Composition root owns wiring; CLI flag binds via koanf precedence (D-03). |
| no_lsp null-object injection | Daemon composition root | Kernel (consumes nil pool / no-op notifier) | Structural suppression at wiring time, not runtime checks (D-09). |
| Structured-edit `Unsupported` guard | Kernel edit tools (`internal/kernel/edit/`, `fileops/`) | — | Runtime backstop owned by the tool handlers (D-04). |
| Import-boundary enforcement | Lint (`internal/lint/ablationleakage/`) + `make vet` | — | Compile-time structural enforcement; house pattern (D-07). |
| Unknown-mode-name rejection | Profile loader (`internal/profile/loader.go`) | — | Loader is the single ingest point for mode/profile YAML. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/tools/go/analysis` | v0.43.0 (already in `go.mod:53`) | `analysis.Analyzer` for `vet-ablation-leakage` | Already the basis of all 4 existing `internal/lint/*` analyzers. [VERIFIED: go.mod] |
| `golang.org/x/tools/go/analysis/singlechecker` | v0.43.0 | `singlechecker.Main(Analyzer)` for `cmd/vet-ablation-leakage` | Exact pattern of `cmd/vet-noduckdb/main.go`. [VERIFIED: codebase] |
| `golang.org/x/tools/go/analysis/analysistest` | v0.43.0 | testdata-driven analyzer tests with `// want` comments | Used by `noduckdb`/`nokernel2semantic` tests. [VERIFIED: codebase] |
| `gopkg.in/yaml.v3` | (in `go.mod`) | profile YAML decode | Existing `internal/profile/loader.go` decoder. [VERIFIED: codebase] |
| `github.com/knadh/koanf/v2` | (in `go.mod`) | CLI→config precedence for the two override flags | Existing 4-layer config (`internal/config/loader.go`). [VERIFIED: codebase] |
| `go.opentelemetry.io/otel/trace` | (in `go.mod`) | span family being suppressed (not added) | `lspool.lsp.*` span emitted at `jsonrpc/conn.go:102`. [VERIFIED: codebase] |

### Supporting
None — this phase adds no new dependencies.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `KernelConfig` bool fields | A separate `kernel.AblationConfig` struct | More ceremony; D-03 says threading is discretion, and `KernelConfig` already exists and is passed to `NewKernel`. Recommend the simpler bool fields. |
| Runtime flag-check in each LSP call site | Null-object injection at daemon init (D-09) | Per-call-site checks are error-prone and the very leakage `vet-ablation-leakage` exists to prevent. Structural suppression is the locked decision. |
| New `subsystem_disabled` error kind | Reuse existing `serr.Unsupported` kind | ABLATE-07 literally says "return `unsupported`"; the `Unsupported` kind already exists. Recommend reusing it (see Open Questions Q1 for the D-06 nuance). |

**Installation:**
```bash
# No new packages. All dependencies already present in go.mod.
# Build the new vet tool the same way the others are built:
go install ./cmd/vet-ablation-leakage
```

**Version verification:**
```bash
grep "golang.org/x/tools" go.mod   # -> golang.org/x/tools v0.43.0  [VERIFIED: go.mod]
```

## Package Legitimacy Audit

No external packages are installed by this phase. All libraries used (`golang.org/x/tools`, `gopkg.in/yaml.v3`, `koanf/v2`, `otel`) are pre-existing `go.mod` dependencies already exercised by sibling code in the repo.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none added) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                        ┌─────────────────────────────────────────────┐
   CLI flags            │  internal/cli/root.go                        │
   --profile=bench-no-lsp ──┐                                          │
   --disable-lsp-subsystem ─┼──> koanf bind ──> config.SerenaConfig    │
                          └──┼──> (new) DisableLSPSubsystem etc.        │
                             └──────────────────┬─────────────────────┘
                                                │ (D-02 precedence:
                                                │  CLI > profile YAML > default-off)
                                                ▼
       internal/profile/profiles/bench-*.yaml  │
       (disable_lsp_subsystem: true, +          │
        EMPTY/exclude tools list)  ──> ResolveProfile()
                                                │
                                                ▼
            ┌───────────────── internal/daemon/daemon.go (composition root) ───────────────┐
            │  step 5:  effDisableLSP := cliOverride OR resolvedProfile.DisableLSPSubsystem  │
            │           k := NewKernel(..., KernelConfig{ Pool, DisableLSPSubsystem,         │
            │                                             DisableStructuredEditSubsystem })  │
            │                                                                                 │
            │  IF DisableLSPSubsystem:                                                         │
            │     - SetEditNotifier(noopNotifier{})   ◄── instead of live bundle             │
            │     - SKIP rs.SetEnrichFn(...)          ◄── daemon.go:661                       │
            │     - do NOT register pool-backed symbol/diag tools (profile excludes them)     │
            │                                                                                 │
            │  step 12:  initialAllowedTools = resolveAllowedToolsForMode(...)  (tool filter) │
            └────────────────────────────────────────────────┬────────────────────────────┘
                                                              │
                  ┌───────────────────────────────────────────┼──────────────────────────────┐
                  ▼                                            ▼                              ▼
   internal/kernel/symbols/                  internal/kernel/edit/ +            internal/kernel/jsonrpc/conn.go
   lspProbeForEdges (no live pool             fileops/ tools                     tracer.Start("lspool.lsp."+method)
   lease => no LSP call => no span)           IF DisableStructuredEdit:          ── ONLY fires if a live LS
                                              return serr.ErrUnsupported         worker is reached. Under no_lsp
   get_repo_map / get_context                replace_in_file: skip fuzzy        the pool is never leased => 0 spans
   (SetEnrichFn skipped => tree-sitter        block (D-05)
   only, no LSP enrichment)

   ─────────────────────── compile-time guard (independent of runtime) ───────────────────────
   internal/lint/ablationleakage/Analyzer  ──>  cmd/vet-ablation-leakage  ──>  make vet
   fails if an ablation-gated tool pkg imports a forbidden disabled-subsystem pkg
```

### Recommended Project Structure
```
internal/profile/profiles/
├── bench-full.yaml              # all bench skills, no disable flags (control arm)
├── bench-no-lsp.yaml            # disable_lsp_subsystem: true + excludes LSP-only tools
├── bench-no-semantic.yaml       # excludes the 10 semantic-store-backed tools (tool-filter only this phase)
└── bench-no-structured-edit.yaml# disable_structured_edit_subsystem: true + excludes 4 structured-edit tools

internal/profile/
├── profile.go                   # + DisableLSPSubsystem / DisableStructuredEditSubsystem fields
├── loader.go                    # + unknown-mode-name rejection
└── bench_profiles_test.go       # NEW golden tests (one per bench YAML)

internal/kernel/kernel.go        # + 2 bool fields on KernelConfig + accessors
internal/daemon/daemon.go        # composition-root: read flags, conditional wiring
internal/cli/root.go             # + 2 persistent bool override flags
internal/errors/kinds.go         # (reuse Unsupported; document subsystem-disabled convention)

internal/lint/ablationleakage/
├── analyzer.go                  # near-clone of noduckdb/analyzer.go
├── analyzer_test.go             # analysistest with want-comment fixtures
└── testdata/src/
    ├── <gated-pkg>/imports.go   # deliberate forbidden import + // want
    └── <good-pkg>/imports.go    # clean control fixture
cmd/vet-ablation-leakage/main.go # singlechecker.Main(ablationleakage.Analyzer)
Makefile                         # + VETTOOL_ABLATION_LEAKAGE + go vet -vettool= line
```

### Pattern 1: Kernel disable flag on `KernelConfig`
**What:** Add the two booleans to the existing `kernel.KernelConfig` (`internal/kernel/kernel.go:19-21`) and store them on `Kernel`. Expose accessors (`func (k *Kernel) LSPSubsystemDisabled() bool`) so edit/symbol handlers can guard.
**When to use:** D-03 — kernel-config concern owned by the daemon, threaded through the existing constructor.
**Example:**
```go
// Source: internal/kernel/kernel.go (existing struct, extended)
type KernelConfig struct {
    Pool lspool.PoolConfig
    DisableLSPSubsystem            bool // Phase 76 ABLATE-05
    DisableStructuredEditSubsystem bool // Phase 76 ABLATE-07
}
// NewKernel already takes cfg KernelConfig; store cfg on k.config (already done at kernel.go:55).
func (k *Kernel) StructuredEditDisabled() bool { return k.config.DisableStructuredEditSubsystem }
func (k *Kernel) LSPSubsystemDisabled() bool   { return k.config.DisableLSPSubsystem }
```

### Pattern 2: Composition-root resolution with CLI > profile > default precedence (D-02)
**What:** At `daemon.go` step 5 (kernel construction, currently `daemon.go:274`), compute effective flags before `NewKernel`.
**Example:**
```go
// Source: pattern derived from daemon.go step 5 + resolveAllowedToolsForMode
// activeProfile is already resolved (config.ResolveProfile). cfg carries CLI overrides via koanf.
effDisableLSP := cfg.DisableLSPSubsystem || activeProfile.DisableLSPSubsystem
effDisableSE  := cfg.DisableStructuredEditSubsystem || activeProfile.DisableStructuredEditSubsystem
// NOTE: a plain OR makes the CLI a one-way "force-disable". Because default is OFF and the
// flag is an opt-in disable (D-02), OR is the correct precedence: CLI true OR profile true => disabled.
k := kernel.NewKernel(workspaces, langReg, installer,
    kernel.KernelConfig{Pool: poolCfg, DisableLSPSubsystem: effDisableLSP,
        DisableStructuredEditSubsystem: effDisableSE},
    pressure, logger, observability.Metrics(), observability.Tracer())
```

### Pattern 3: no_lsp null-object injection (D-09)
**What:** Under `effDisableLSP`, the daemon does NOT wire the live LSP-touching seams:
- Instead of `buildLiveBundle(...)` → `SetEditNotifier(live)`, install a no-op notifier (or skip SetEditNotifier entirely — `EditNotifier()` returns nil, and all 4 call sites at `fileops/tools.go:263,447,463,519` already nil-check before invoking).
- Skip the `rs.SetEnrichFn(...)` block at `daemon.go:661` — repomap then falls back to tree-sitter / no LSP enrichment.
- Exclude pool-backed symbol/diag tools at the profile layer (`bench-no-lsp.yaml`), so `acquireLease`→`rt.AcquireSession`→LSP-function call sites (`symbols/tools.go:349-583`) are unreachable, hence `lspool.lsp.*` spans (`jsonrpc/conn.go:102`) cannot fire.
**Why it satisfies the hard-fail:** The span name `lspool.lsp.<method>` is only emitted on a real `conn.Call`/`Notify` to a live LS worker. No lease → no Call → no span.

### Pattern 4: Unknown-mode-name rejection (ABLATE-02)
**What:** `resolveAllowedToolsForMode` (`daemon.go:78`) currently returns `nil` silently when `store.Mode(modeName)` misses (line 82-85). The loader (`internal/profile/loader.go`) silently accepts any YAML. ABLATE-02 requires the loader to **reject** an unknown mode name. Add a validation pass: after loading, for each profile, verify its `default_mode` and every key in `allowed_mode_transitions` resolves to a loaded mode; return an error otherwise. The cleanest seam is a new `func (s *ProfileStore) Validate() error` invoked in `LoadEmbedded` (after both profiles+modes load).
**When to use:** the golden-test "loader rejects an unknown mode name" criterion.

### Pattern 5: Import-boundary analyzer (clone of noduckdb) — D-07
**What:** `internal/lint/ablationleakage/analyzer.go` follows `noduckdb/analyzer.go` exactly: a `checkedPkgPrefix` (the ablation-gated tool package) and a `forbiddenImportPrefix` (the disabled-subsystem package). See §Open Questions Q2 for the concrete forbidden edge to pin.
**Example:**
```go
// Source: internal/lint/noduckdb/analyzer.go (copy-shape)
const checkedPkgPrefix = "github.com/agenthands/helix/internal/kernel/fileops" // gated tool pkg
const forbiddenImportPrefix = "github.com/agenthands/helix/internal/fuzzy"      // disabled subsystem
var Analyzer = &analysis.Analyzer{
    Name: "ablationleakage",
    Doc:  "fails if an ablation-gated tool package imports a disabled-subsystem package",
    Run: func(pass *analysis.Pass) (interface{}, error) {
        if !strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) { return nil, nil }
        for _, file := range pass.Files {
            for _, imp := range file.Imports {
                path := strings.Trim(imp.Path.Value, `"`)
                if path == forbiddenImportPrefix || strings.HasPrefix(path, forbiddenImportPrefix+"/") {
                    pass.Reportf(imp.Pos(), "ablation-gated %s must not import %s (got %s)",
                        checkedPkgPrefix, forbiddenImportPrefix, path)
                }
            }
        }
        return nil, nil
    },
}
```

### Anti-Patterns to Avoid
- **Per-call-site `if k.LSPSubsystemDisabled()` checks scattered across symbol/diag tools for no_lsp:** D-09 mandates structural suppression at the wiring root. Scattered checks are exactly the leakage surface `vet-ablation-leakage` exists to catch.
- **Inventing a new error kind for structured-edit disable:** `serr.Unsupported` already exists (`kinds.go:17`). ABLATE-07 says "return `unsupported`." Do not add a redundant kind (see Q1).
- **Bare `strings.HasPrefix` without slash-boundary in the analyzer:** `nokernel2semantic` documents the `integ`/`integ_evil` collision (A6). Match exact-OR-slash-suffix (the `noduckdb` form already does this).
- **Editing `eval/` or any pre-milestone runtime code:** out of scope; Phase 76 only adds new files + extends 5 named files.
- **Wiring `disable_semantic_subsystem` (D-12):** explicitly Phase 81. `bench-no-semantic.yaml` is tool-filter-only here.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Import-boundary detection | A custom AST walker / regex over source files | `golang.org/x/tools/go/analysis` + `singlechecker` | Exact sibling pattern (`noduckdb`); package-resolution + `// want` testing for free. |
| Analyzer testing | Hand-rolled diff assertions | `analysistest.Run` with `// want` comments | All 4 existing analyzers use it; `analysistest` already vendored. |
| Tool filtering | New filter middleware | EMPTY-LISTS / `exclude_tools` via `ResolveTools` | `ProfileFilterMiddleware` + `baseline.yaml` already prove this. |
| Nil-safe notifier | New conditional in each edit tool | The existing nil-check-before-`OnEdit` contract (`notifier.go:48-51`) + a no-op or unset notifier | All 4 call sites already nil-check; injecting nil/no-op is sufficient. |
| Typed error for disabled tool | New error struct | `serr.New(serr.Unsupported, "...").WithTool(...)` | Taxonomy + `errors.Is` matching already exist. |
| CLI→config precedence | Manual flag parsing | koanf posflag bind in `root.go` | 4-layer precedence already implemented. |

**Key insight:** Every deliverable in Phase 76 has a complete, tested in-repo precedent. The risk is not novelty — it is *missing a call site* (the D-10 audit) or *picking the wrong forbidden import edge* for the analyzer (Q2).

## Runtime State Inventory

> Phase 76 is a code/config-only feature phase (new flags, new YAMLs, new analyzer). It does NOT rename, migrate, or rewrite existing stored state. Section retained for completeness.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no datastore keys/collections change. Verified: no DB schema or key rename in scope. | none |
| Live service config | None — daemon flags are read at boot from profile YAML + CLI; no external service config embeds them. | none |
| OS-registered state | None — no Task Scheduler / systemd / pm2 registrations involve these flags. | none |
| Secrets/env vars | None — no new secret keys. CLI flags bind via koanf, no env-var rename. (Optional: koanf may map `--disable-lsp-subsystem` to env, but no existing env var is renamed.) | none |
| Build artifacts | New binary `cmd/vet-ablation-leakage` is `go install`ed by `make vet` (mirrors `vet-noduckdb`). No stale artifact from a rename. | `go install ./cmd/vet-ablation-leakage` (Make target handles it) |

**Nothing found requiring data migration** — verified by scoping against the four deliverables (flags, YAMLs, analyzer, loader validation), none of which touch persisted state.

## Common Pitfalls

### Pitfall 1: Incomplete no_lsp wiring → a live span leaks
**What goes wrong:** A symbol or diag tool the profile forgot to exclude acquires a pool lease under no_lsp and emits a `lspool.lsp.*` span, failing roadmap criterion #2.
**Why it happens:** There are ~13 pool-leasing call sites in `symbols/tools.go` (`acquireLease`+`AcquireSession` at lines 349,376,424,453,479,509,536,570,…) plus diag's `leaseFn` (`daemon.go:610`) and the repomap fallback `AcquireLease` (`daemon.go:679`).
**How to avoid (D-10 audit):** Enumerate every `k.Pool()` / `AcquireLease` / `AcquireSession` reachable from a tool. Under no_lsp either (a) exclude the owning tool in `bench-no-lsp.yaml`, or (b) guard the lease path to return `Unsupported`. The repomap fallback extractor (`daemon.go:671-686`) and diag `leaseFn` (`daemon.go:608-612`) are NOT tool-gated by the profile, so they need the SetEnrichFn-skip (repomap) and an explicit guard or no-op (diag) — confirm both during planning.
**Warning signs:** Any non-zero `lspool.lsp.*` span in the no_lsp trace tap.

### Pitfall 2: `replace_in_file` still does fuzzy under structured-edit-disable
**What goes wrong:** The fuzzy fallback block (`fileops/tools.go:415-453`) runs even when `disable_structured_edit_subsystem` is set, violating D-05.
**Why it happens:** The fuzzy block is an inline `if count == 0 && !args.IsRegex` branch, not a separate function.
**How to avoid:** Add `&& !k.StructuredEditDisabled()` to the fuzzy-fallback guard at line 415, so a no-match under the flag returns the plain `"0 replacement(s) made"` path (line 426 already exists). Cover with a test that asserts no fuzzy match occurs and outcome classification is correct.
**Warning signs:** A `match_strategy:` line in `replace_in_file` output under the no_structured_edit arm.

### Pitfall 3: Analyzer testdata not isolated → `go vet ./...` fails on the deliberate violation
**What goes wrong:** A deliberate forbidden import placed in a real package (not under `testdata/`) makes the whole repo's `go vet`/`make vet` fail permanently.
**Why it happens:** Misunderstanding that the violation must live in `internal/lint/ablationleakage/testdata/src/...`, which `go build`/`go vet ./...` ignore (testdata is excluded by the go tool).
**How to avoid:** Mirror `noduckdb/testdata/src/badpkg/imports.go` — the violation lives only in testdata with a `// want \`...\`` comment, exercised solely by `analysistest.Run`. The green→red flip is demonstrated by the test fixture, never by polluting the real tree.
**Warning signs:** `make vet` failing on a normal build (not the analyzer's own test).

### Pitfall 4: Unknown-mode rejection breaks existing profiles
**What goes wrong:** Adding strict mode-name validation rejects an existing profile whose `default_mode`/transitions reference a mode that isn't in `internal/profile/modes/`.
**Why it happens:** Existing profiles use `read`/`edit`/`review`/`admin`; if the modes dir doesn't define all four as loadable `Mode` entries, validation over-fires.
**How to avoid:** Before adding validation, enumerate `internal/profile/modes/*.yaml` and assert every mode referenced by existing profiles (incl. `baseline.yaml`, `full.yaml`) resolves. Scope the rejection to the *bench* loader path if a global one regresses existing profiles, OR fix the modes dir. Test both the accept (all 4 bench YAMLs load) and reject (a synthetic `default_mode: nonsense`) cases.
**Warning signs:** `LoadEmbedded` returning an error for `claude-code`/`full` after the change.

### Pitfall 5: Choosing a too-broad forbidden import edge for the analyzer
**What goes wrong:** Pinning `internal/kernel` → `internal/fuzzy` as forbidden would break `replace_in_file`/`fuzzy_edit` which legitimately import `internal/fuzzy` in the *enabled* build. The analyzer is compile-time and cannot see the runtime flag (D-08).
**Why it happens:** The structured-edit tools always import `internal/fuzzy`; the ablation is a runtime flag, not a compile-time partition. So the forbidden edge cannot be "edit tools must not import fuzzy."
**How to avoid:** The analyzer must pin an edge that is genuinely architectural, not flag-conditional. Candidates: (a) a *new bench-runner* package (lands Phase 80) must not import `internal/kernel/lspool` or `internal/semantic/store`; (b) the `baseline_rag` standalone (`cmd/helix-bench-rag`, Phase 83) must not import `internal/kernel`/`internal/semantic` (ABLATE-04 already specifies this vet test). For Phase 76's deliverable, pin the edge that is BOTH true today AND meaningful — see Open Questions Q2; the safest concrete edge is documented there.

## Code Examples

### Bench profile YAML (no_lsp arm) — reuses EMPTY-LISTS + adds disable flag
```yaml
# Source: derived from internal/profile/profiles/baseline.yaml + full.yaml
name: bench-no-lsp
description: >
  Bench ablation arm: LSP subsystem disabled. Tool surface excludes the
  9 pool-backed LSP symbol tools + diagnostics; structured edits and
  tree-sitter repomap remain. Pairs with kernel disable_lsp_subsystem.
prompt: |
  You are the no_lsp ablation arm. LSP-backed code intelligence is unavailable.
skills:
  - file-ops
  - memory
  - workflow
  - repomap        # tree-sitter only under no_lsp (SetEnrichFn skipped)
  - help
  # NOTE: symbol-retrieval + diagnostics OMITTED (LSP-backed) — D-09
tools: []
exclude_tools: []
disable_lsp_subsystem: true          # Phase 76 new first-class field (D-01)
tool_description_overrides: {}
default_mode: edit
single_project: false
guardrails:
  enforcement: off
allowed_mode_transitions:
  read: [edit, review]
  edit: [read, review]
  review: [edit, read]
  admin: [read, edit, review]
```

### Structured-edit guard in a kernel edit handler (D-04)
```go
// Source: pattern at internal/kernel/edit/tools.go:307+ (replace_symbol_body handler)
// Insert near the top of the handler, after arg validation:
if k.StructuredEditDisabled() {
    return errorResult(serr.New(serr.Unsupported,
        "replace_symbol_body is disabled (structured-edit subsystem off); use replace_in_file").
        WithTool("replace_symbol_body").Error()), nil, nil
}
```

### `replace_in_file` fuzzy-fallback guard (D-05)
```go
// Source: internal/kernel/fileops/tools.go:415 (existing guard, extended)
// Fuzzy fallback: when literal match returns 0 hits and not regex (FUZZ-06)
if count == 0 && !args.IsRegex && !k.StructuredEditDisabled() {  // <-- add the flag check
    // ... existing fuzzy.Match block ...
}
// When StructuredEditDisabled and count==0, falls through to the plain
// "0 replacement(s) made" return (exact-match-only behavior, D-05).
```

### `make vet` wiring (Makefile, mirrors lines 33-50)
```makefile
# Source: Makefile lines 20-50 (existing -vettool= chain)
VETTOOL_ABLATION_LEAKAGE=$(shell go env GOPATH)/bin/vet-ablation-leakage

vet: $(VETTOOL) $(VETTOOL_NOKERNEL2SEMANTIC) $(VETTOOL_NOSEMANTIC2KERNEL) $(VETTOOL_COMPACT_USES_STORE) $(VETTOOL_ABLATION_LEAKAGE)
	$(GO) vet ./...
	$(GO) vet -vettool=$(VETTOOL) ./...
	$(GO) vet -vettool=$(VETTOOL_NOKERNEL2SEMANTIC) ./...
	$(GO) vet -vettool=$(VETTOOL_NOSEMANTIC2KERNEL) ./...
	$(GO) vet -vettool=$(VETTOOL_COMPACT_USES_STORE) ./...
	$(GO) vet -vettool=$(VETTOOL_ABLATION_LEAKAGE) ./...

$(VETTOOL_ABLATION_LEAKAGE): cmd/vet-ablation-leakage/main.go internal/lint/ablationleakage/*.go
	$(GO) install ./cmd/vet-ablation-leakage
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CGO=0 stub apparatus / `//go:build cgo` constraints | single-mode `CGO_ENABLED=1` | Phase 59.1 | No build-tag gymnastics needed; the analyzer + flags compile in the one build mode. |
| `disable_all_tools` flag (proposed Phase 67) | EMPTY-LISTS via `ResolveTools` + `AllowedTools != nil` | Phase 67 (A1 VERIFIED) | bench YAMLs use empty/exclude lists, not a disable-all flag (`baseline.yaml` header). |
| n/a | `serr.Unsupported` kind + sentinel | exists pre-76 | No new error taxonomy; reuse. |

**Deprecated/outdated:** none relevant to this phase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The 9 symbol-retrieval tools + `get_diagnostics`/`get_code_actions`/`format_file` are the LSP-backed surface to exclude from `bench-no-lsp.yaml`. (`repomap`, `memory`, `file-ops`, `workflow` are not LSP-backed.) | no_lsp Architecture / Q3 | If a "non-LSP" tool transitively leases the pool, a span leaks. Mitigated by the D-10 audit + trace-tap test. [ASSUMED — based on reading skill.go tool lists + lease call sites] |
| A2 | The 10 semantic skill tools are the correct `bench-no-semantic.yaml` exclude-list; `find_references`/`analyze_blast_radius`/`get_repo_map`/`get_context` are NOT excluded (they degrade to LSP/tree-sitter, not duckdb-only). | Q4 / D-12 | Over- or under-excluding changes the ablation's meaning. Confirm with user in discuss/plan. [ASSUMED — semantic skill = the duckdb-store consumers; symbol/repomap tools have non-store fallbacks per strangler design] |
| A3 | Reusing `serr.Unsupported` (rather than a new `subsystem_disabled` kind) satisfies D-06/ABLATE-07. | Q1 | If graders expect a distinct greppable kind, a message-level convention may be required instead. Low risk — ABLATE-07 says "unsupported". [ASSUMED] |
| A4 | The CLI override flags should be `--disable-lsp-subsystem` / `--disable-structured-edit-subsystem` (persistent bool on rootCmd), bound via koanf like `--profile`. | Q1 | Naming is discretion (D-01); a different convention is cosmetic. [ASSUMED] |
| A5 | The forbidden import edge for `vet-ablation-leakage` v1 is best pinned at a genuinely-architectural boundary (not the flag-conditional `edit→fuzzy` edge). | Pitfall 5 / Q2 | A wrong edge either breaks the enabled build or catches nothing. Confirm the exact edge in planning. [ASSUMED] |

## Open Questions

1. **D-06: reuse `serr.Unsupported` vs. add `subsystem_disabled`?**
   - What we know: `serr.Unsupported` (= `"unsupported"`) already exists with a sentinel; ABLATE-07 literally says return `unsupported`.
   - What's unclear: whether graders want a *distinct* greppable kind to disambiguate "disabled by ablation" from "unsupported language/feature."
   - Recommendation: **Reuse `serr.Unsupported`**, and standardize a greppable message prefix (e.g. `"subsystem_disabled: <tool> requires the structured-edit subsystem"`). Document the convention in a doc-comment on `kinds.go` near `Unsupported`. This satisfies ABLATE-07's literal text and gives a greppable token without a redundant kind. (D-06 is Claude's discretion — confirm in discuss-phase.)

2. **D-07/Q2: which exact forbidden import edge does the v1 analyzer pin?**
   - What we know: It must be a true-today, architectural (not flag-conditional) edge. The structured-edit tools legitimately import `internal/fuzzy` in the enabled build, so `kernel/*`→`fuzzy` is NOT a valid forbidden edge (Pitfall 5).
   - What's unclear: The downstream ablation-runner packages (`bench/runners/...`, `cmd/helix-bench-rag`) that the analyzer is ultimately meant to police land in Phases 80/83 — they don't exist yet in Phase 76.
   - Recommendation: Pin the edge that is both architectural and present today, with testdata proving the green→red flip even if the real gated package is a stub. Strongest candidate: **the `bench-*` profile/runner namespace must not import `internal/kernel/lspool` or `internal/semantic/store`** (the two disabled subsystems). If no real bench-runner package exists yet, the analyzer's `checkedPkgPrefix` can target the future `bench/runners/` path; its correctness is proven entirely by the `testdata/` fixtures (which the go tool ignores), so the green→red flip is demonstrable without a real consumer. Lock the exact `checkedPkgPrefix`/`forbiddenImportPrefix` pair in planning.

3. **Q3: which tools exactly are LSP-backed (no_lsp exclude-list)?**
   - What we know (from `skill.go` files + lease call sites): symbol-retrieval skill = 9 tools all use `acquireLease`→`AcquireSession`→LSP fns; diagnostics skill (`get_diagnostics`, `get_code_actions`, `format_file`) uses a pool `leaseFn`.
   - What's unclear: whether `analyze_blast_radius` (which uses the SemanticLookup strangler + an LSP Pass-2 probe) should be in the no_lsp arm at all, or kept with its LSP-probe portion no-op'd.
   - Recommendation: For no_lsp, exclude the full `symbol-retrieval` + `diagnostics` skills in `bench-no-lsp.yaml`. Keep `repomap` (tree-sitter), `file-ops`, `memory`, `workflow`, `help`, and the semantic skill (duckdb-backed, not LSP). Confirm in discuss-phase.

4. **Q4: exact `bench-no-semantic.yaml` exclude-list?**
   - What we know: The 10 semantic-skill tools (`index_semantic_graph`, `refresh_semantic_graph`, `get_semantic_graph_status`, `get_semantic_context`, `explain_symbol_deep`, `find_related_symbols`, `validate_graph_edge`, `get_cluster_map`, `explain_cluster`, `get_change_impact_graph`) are the duckdb-store consumers.
   - What's unclear: whether `get_repo_map`/`get_context` (repomap skill, which *can* be semantic-enriched via the Phase 65 strangler but fall back to tree-sitter) should also be excluded or kept with `source == tree_sitter`.
   - Recommendation: Exclude the 10 semantic-skill tools (cleanest, fully tool-filterable now). Keep `get_repo_map`/`get_context` — they already degrade to tree-sitter via the strangler and the *kernel* `disable_semantic_subsystem` (Phase 81) is what forces `source == tree_sitter`. This matches D-11's "profile filter now, kernel guard in Phase 81." Confirm in discuss-phase.

## Environment Availability

> No external runtime dependencies. The phase is pure Go source + YAML + Make wiring.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain (`go vet`, `go install`) | analyzer build + vet wiring | ✓ | per CLAUDE.md (`go build ./cmd/helix`) | — |
| `golang.org/x/tools` | analyzer | ✓ (go.mod) | v0.43.0 | — |

**Missing dependencies with no fallback:** none.

## Validation Architecture

> Nyquist validation is enabled (no `workflow.nyquist_validation: false` found). This phase has two hard-fail gates (zero-span OTel + green→red vet) that are first-class automatable tests.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` + `golang.org/x/tools/go/analysis/analysistest` for the analyzer |
| Config file | none (Go convention) |
| Quick run command | `go test ./internal/lint/ablationleakage/... ./internal/profile/... ./internal/kernel/... -count=1` |
| Full suite command | `make test` (runs `make vet` then `go test ./...`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ABLATE-02 | 4 bench YAMLs load + golden tool-surface per arm | unit | `go test ./internal/profile/ -run TestBenchProfiles -count=1` | ❌ Wave 0 |
| ABLATE-02 | loader rejects unknown mode name | unit | `go test ./internal/profile/ -run TestLoaderRejectsUnknownMode -count=1` | ❌ Wave 0 |
| ABLATE-05 | no_lsp emits zero `lspool.lsp.*` spans | integration (trace-tap) | `go test ./internal/daemon/ -run TestNoLSPZeroSpans -count=1` | ❌ Wave 0 (model on `jsonrpc/conn_trace_test.go`) |
| ABLATE-05 | `SetEnrichFn` skipped + no-op notifier under flag | unit | `go test ./internal/daemon/ -run TestNoLSPWiring -count=1` | ❌ Wave 0 |
| ABLATE-07 | structured-edit tools return `Unsupported` under flag | unit | `go test ./internal/kernel/edit/ -run TestStructuredEditDisabled -count=1` | ❌ Wave 0 |
| ABLATE-07 | `replace_in_file` exact-match-only under flag | unit | `go test ./internal/kernel/fileops/ -run TestReplaceInFileNoFuzzyWhenDisabled -count=1` | ❌ Wave 0 |
| ABLATE-08 | analyzer fires on deliberate testdata violation | unit | `go test ./internal/lint/ablationleakage/ -count=1` | ❌ Wave 0 |
| ABLATE-08 | `make vet` green→red flip demonstrable | gate | `make vet` (with a temporarily-injected violation in a scratch test) | n/a (proven via analysistest fixture) |

### Sampling Rate
- **Per task commit:** `go vet ./... && go test ./internal/lint/ablationleakage/... ./internal/profile/... -count=1`
- **Per wave merge:** `make vet && go test ./internal/kernel/... ./internal/daemon/... -count=1`
- **Phase gate:** `make test` fully green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/profile/bench_profiles_test.go` — golden tool-surface per bench arm (ABLATE-02)
- [ ] `internal/profile/loader_test.go` — extend with unknown-mode-rejection case (ABLATE-02)
- [ ] `internal/daemon/no_lsp_wiring_test.go` — zero-span trace-tap + SetEnrichFn-skip (ABLATE-05); model on `internal/kernel/jsonrpc/conn_trace_test.go` (uses an in-memory span recorder)
- [ ] `internal/kernel/edit/structured_edit_disabled_test.go` — Unsupported guard (ABLATE-07)
- [ ] `internal/kernel/fileops/replace_in_file_disabled_test.go` — exact-match-only (ABLATE-07/D-05)
- [ ] `internal/lint/ablationleakage/analyzer_test.go` + `testdata/src/...` fixtures — green→red (ABLATE-08)

## Security Domain

> `security_enforcement` not explicitly disabled; included per default-on. This phase adds no auth/session/crypto surface; the relevant category is input handling on the new CLI flags + YAML fields.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | n/a — no auth surface added |
| V3 Session Management | no | n/a |
| V4 Access Control | yes (indirect) | The disable flags REDUCE the tool surface; failure mode is a tool being available when it should be filtered. Defense-in-depth (D-04 filter+guard) is the control. |
| V5 Input Validation | yes | YAML fields decoded by `yaml.v3` (typed bool); CLI flags typed bool via koanf; unknown-mode rejection (ABLATE-02) IS an input-validation control. |
| V6 Cryptography | no | n/a |

### Known Threat Patterns for Go daemon + profile YAML
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| A disabled tool remains callable via a back-channel (e.g. switch_mode, direct MCP call) | Elevation of Privilege (ablation integrity) | D-04 kernel `Unsupported` guard + `vet-ablation-leakage` compile-time boundary |
| Malformed/unknown mode name in an override YAML silently no-ops the filter | Tampering | ABLATE-02 loader rejection (fail-closed) |
| LSP span leaks under no_lsp, corrupting the ablation measurement | Information Disclosure (measurement validity) | D-09 null-object injection (structural) + trace-tap hard-fail test |

## Sources

### Primary (HIGH confidence)
- `internal/profile/profiles/baseline.yaml`, `full.yaml` — EMPTY-LISTS filter mechanism, profile YAML shape [VERIFIED: codebase]
- `internal/profile/profile.go`, `loader.go` — `Profile` struct, `ProfileStore`, embedded loader [VERIFIED: codebase]
- `internal/lint/noduckdb/analyzer.go` + `analyzer_test.go` + `testdata/` — analyzer copy-shape + `// want` testing [VERIFIED: codebase]
- `internal/lint/nokernel2semantic/analyzer.go` + test — slash-boundary discipline (A6) [VERIFIED: codebase]
- `cmd/vet-noduckdb/main.go` + `Makefile:20-50` — singlechecker + `-vettool=` wiring [VERIFIED: codebase]
- `internal/kernel/kernel.go:18-59` — `KernelConfig`, `NewKernel`, `k.config` storage [VERIFIED: codebase]
- `internal/kernel/notifier.go:30-63` — `EditNotifier` interface + nil-check contract [VERIFIED: codebase]
- `internal/kernel/fileops/tools.go:263,415-468` — `OnEdit` call sites + fuzzy fallback entry [VERIFIED: codebase]
- `internal/kernel/edit/tools.go:305-395` — structured-edit handler shape [VERIFIED: codebase]
- `internal/kernel/symbols/tools.go:159-583` — `acquireLease`/`AcquireSession` pool-lease sites [VERIFIED: codebase]
- `internal/kernel/symbols/blast_radius_strangler.go:151-199` — `lspProbeForEdges` [VERIFIED: codebase]
- `internal/kernel/jsonrpc/conn.go:102,160` — `lspool.lsp.*` span emission [VERIFIED: codebase]
- `internal/errors/kinds.go:13-38` — `Unsupported` kind + sentinel [VERIFIED: codebase]
- `internal/daemon/daemon.go:78-111,274,580-686` — `resolveAllowedToolsForMode`, kernel construction, SetEnrichFn/notifier wiring [VERIFIED: codebase]
- `internal/skill/semantic/skill.go:331-390` + `register.go` — 10 semantic-store-backed tool names [VERIFIED: codebase]
- `internal/skill/semantic/skill.go`, `kernel/symbols/skill.go`, `kernel/fileops/skill.go`, `kernel/edit/skill.go` — per-skill tool inventories [VERIFIED: codebase]
- `internal/cli/root.go:56-73` — daemon CLI flag pattern (`--profile`, etc.) [VERIFIED: codebase]
- `go.mod:53` — `golang.org/x/tools v0.43.0` present [VERIFIED: go.mod]
- `.planning/REQUIREMENTS.md` ABLATE-02/05/07/08 [CITED]
- `.planning/milestones/v1.12-ROADMAP.md` Phase 76 §, 4 success criteria [CITED]
- `.planning/phases/76-.../76-CONTEXT.md` D-01..D-12 [CITED]

### Secondary (MEDIUM confidence)
- None — all findings are codebase-verified.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new deps; all libraries pre-existing and exercised by siblings.
- Architecture: HIGH — every deliverable has a tested in-repo precedent; line numbers verified this session (CONTEXT.md anchors confirmed: `notifier.go` nil-check at 48-51, `fileops/tools.go` OnEdit at 263/447/463/519, `daemon.go` SetEnrichFn at 661, `jsonrpc/conn.go` span at 102).
- Pitfalls: HIGH — derived from concrete call-site enumeration (the D-10 audit) and the analyzer compile-time/runtime-scope distinction (D-08).
- Open questions: MEDIUM — Q1/Q2/Q3/Q4 are bounded discretion decisions (D-06/D-07/D-12) with recommended answers; they need user confirmation in discuss-phase but do not block planning.

**Research date:** 2026-06-16
**Valid until:** 2026-07-16 (stable — internal Go codebase, no fast-moving external deps)
