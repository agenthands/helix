# Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags - Pattern Map

**Mapped:** 2026-06-16
**Files analyzed:** 14 (8 new, 6 modified)
**Analogs found:** 14 / 14 (every deliverable has a tested in-repo precedent)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/profile/profiles/bench-full.yaml` (NEW) | config | n/a (declarative) | `internal/profile/profiles/baseline.yaml` | exact |
| `internal/profile/profiles/bench-no-lsp.yaml` (NEW) | config | n/a | `baseline.yaml` + `full.yaml` | exact |
| `internal/profile/profiles/bench-no-semantic.yaml` (NEW) | config | n/a | `baseline.yaml` | exact |
| `internal/profile/profiles/bench-no-structured-edit.yaml` (NEW) | config | n/a | `baseline.yaml` | exact |
| `internal/profile/profile.go` (MODIFY) | model | transform | existing `Profile` struct (same file) | exact |
| `internal/profile/loader.go` (MODIFY) | loader | transform | `LoadEmbedded` (same file) | exact |
| `internal/profile/bench_profiles_test.go` (NEW) | test | n/a | existing profile tests | role-match |
| `internal/lint/ablationleakage/analyzer.go` (NEW) | utility | transform (AST pass) | `internal/lint/noduckdb/analyzer.go` | exact (copy-shape) |
| `internal/lint/ablationleakage/analyzer_test.go` (NEW) | test | n/a | `internal/lint/noduckdb/analyzer_test.go` | exact |
| `internal/lint/ablationleakage/testdata/src/...` (NEW) | test fixture | n/a | `noduckdb/testdata/src/...` | exact |
| `cmd/vet-ablation-leakage/main.go` (NEW) | utility | n/a (singlechecker) | `cmd/vet-noduckdb/main.go` | exact |
| `Makefile` `vet` target (MODIFY) | config | n/a | existing `-vettool=` chain (lines 20-50) | exact |
| `internal/kernel/kernel.go` (MODIFY) | model/config | transform | existing `KernelConfig` (same file) | exact |
| `internal/daemon/daemon.go` (MODIFY) | provider (composition root) | event-driven wiring | `resolveAllowedToolsForMode` + SetEnrichFn/notifier wiring (same file) | exact |
| `internal/kernel/edit/tools.go` (MODIFY) | controller (tool handler) | request-response | `replace_symbol_body` handler (same file) | exact |
| `internal/kernel/fileops/tools.go` (MODIFY) | controller (tool handler) | request-response | `replace_in_file` fuzzy-fallback block (same file) | exact |
| `internal/cli/root.go` (MODIFY) | config | n/a | existing `--profile` flag (same file) | exact |

## Pattern Assignments

### `internal/profile/profiles/bench-*.yaml` (config, declarative)

**Analog:** `internal/profile/profiles/baseline.yaml` (full file read; EMPTY-LISTS mechanism is documented in its header lines 1-16).

**Mechanism to copy** — `ProfileFilterMiddleware` filters when `AllowedTools != nil`; an empty non-nil `skills:[] tools:[]` yields zero tools (baseline.yaml:3-16). The daemon resolves via `skill.ResolveTools(profile.Skills, profile.Tools, profile.ExcludeTools)` then `session.SetAllowedTools(...)` (baseline.yaml:6-8).

**Skeleton to copy** (baseline.yaml:17-55) — `name`, `description`, `prompt`, `skills`, `tools`, `exclude_tools`, `tool_description_overrides: {}`, `default_mode`, `single_project`, `guardrails.enforcement`, `allowed_mode_transitions` (all four modes read/edit/review/admin). Use the standard transition block verbatim from baseline.yaml:41-54.

**New first-class fields (D-01)** — add `disable_lsp_subsystem: true` (bench-no-lsp) and `disable_structured_edit_subsystem: true` (bench-no-structured-edit). `bench-full.yaml` sets NEITHER (default-off, D-02). `bench-no-semantic.yaml` sets neither flag this phase (D-11/D-12) — it is tool-filter-only, excluding the 10 semantic-store-backed tools.

**Per-arm content (from RESEARCH.md Q3/Q4 recommendations):**
- `bench-no-lsp`: exclude `symbol-retrieval` + `diagnostics` skills; keep `file-ops`, `memory`, `workflow`, `repomap`, `help`. Full YAML example at RESEARCH.md:326-356.
- `bench-no-semantic`: exclude the 10 semantic-skill tools (`index_semantic_graph`, `refresh_semantic_graph`, `get_semantic_graph_status`, `get_semantic_context`, `explain_symbol_deep`, `find_related_symbols`, `validate_graph_edge`, `get_cluster_map`, `explain_cluster`, `get_change_impact_graph` — RESEARCH.md:435). Keep `get_repo_map`/`get_context`.
- `bench-no-structured-edit`: exclude `replace_symbol_body`, `fuzzy_edit`, `insert_before_symbol`, `insert_after_symbol` (D-04).

---

### `internal/profile/profile.go` (model, struct extension)

**Analog:** the existing `Profile` struct (profile.go:27-53), read in full.

**Pattern to copy** — add two `yaml`-tagged bool fields alongside `SingleProject` (profile.go:43-44), mirroring its tag style:
```go
// DisableLSPSubsystem requests the daemon disable the LSP subsystem (Phase 76 ABLATE-05).
DisableLSPSubsystem bool `yaml:"disable_lsp_subsystem"`
// DisableStructuredEditSubsystem requests the daemon disable structured edits (Phase 76 ABLATE-07).
DisableStructuredEditSubsystem bool `yaml:"disable_structured_edit_subsystem"`
```
Default zero-value `false` gives the default-ENABLED / opt-in-disable semantics (D-02) for free. `yaml.v3` decode of typed bool is already the loader's mechanism (loader.go:57-60).

---

### `internal/profile/loader.go` (loader, unknown-mode-name rejection)

**Analog:** `LoadEmbedded` (loader.go:13-26) and `loadModesFromFS` (loader.go:66-88), read in full.

**Current gap** — `loadProfilesFromFS`/`loadModesFromFS` silently accept any YAML; `resolveAllowedToolsForMode` (daemon.go:82-85) returns nil on a missed `store.Mode(modeName)`. ABLATE-02 wants a fail-closed reject.

**Pattern to add (RESEARCH.md Pattern 4)** — a `func (s *ProfileStore) Validate() error` invoked at the end of `LoadEmbedded` (after both profiles+modes load, loader.go:24). For each profile, verify `DefaultMode` and every key+value in `AllowedModeTransitions` resolves via `store.Mode(...)`; return an `fmt.Errorf` otherwise. Reuse the existing `fmt.Errorf("...: %w", err)` wrap style (loader.go:19-23).

**Pitfall 4 guard** — first enumerate `internal/profile/modes/*.yaml` and confirm every mode referenced by existing `baseline.yaml`/`full.yaml`/etc. resolves, or scope the rejection so existing profiles don't regress.

---

### `internal/lint/ablationleakage/analyzer.go` (utility, AST import-boundary pass)

**Analog:** `internal/lint/noduckdb/analyzer.go` (full file read, 42 lines — near-clone target).

**Imports + Analyzer shape to copy verbatim** (noduckdb/analyzer.go:11-41):
```go
import (
	"strings"
	"golang.org/x/tools/go/analysis"
)

const checkedPkgPrefix = "..."     // ablation-gated tool package
const forbiddenImportPrefix = "..." // disabled-subsystem package

var Analyzer = &analysis.Analyzer{
	Name: "ablationleakage",
	Doc:  "fails if an ablation-gated tool package imports a disabled-subsystem package",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if !strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) {
			return nil, nil
		}
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

**Slash-boundary discipline (anti-pattern, noduckdb:32)** — match exact-OR-`prefix+"/"`, never bare `HasPrefix` (avoids the `integ`/`integ_evil` collision documented in `nokernel2semantic`).

**Forbidden-edge selection (Q2 / Pitfall 5 — DO NOT pin `kernel/*`→`internal/fuzzy`; structured-edit tools legitimately import fuzzy in the enabled build).** Pin a genuinely-architectural edge that is true today and proven by testdata: recommended `checkedPkgPrefix` = a future `bench/runners/` namespace, `forbiddenImportPrefix` = `internal/kernel/lspool` or `internal/semantic/store`. Correctness is proven entirely by the testdata fixtures (the go tool ignores `testdata/`), so the green→red flip is demonstrable without a real consumer. Lock the exact pair in planning.

---

### `cmd/vet-ablation-leakage/main.go` (utility, singlechecker)

**Analog:** `cmd/vet-noduckdb/main.go` (full file, 11 lines — copy verbatim, swap the package):
```go
package main

import (
	"github.com/agenthands/helix/internal/lint/ablationleakage"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(ablationleakage.Analyzer) }
```

---

### `internal/lint/ablationleakage/{analyzer_test.go,testdata/...}` (test + fixtures)

**Analog:** `internal/lint/noduckdb/analyzer_test.go` + `testdata/`. Use `analysistest.Run` with `// want \`...\`` comment fixtures (already vendored, go.mod:53). One `testdata/src/<bad-pkg>/imports.go` carries the deliberate forbidden import + `// want`, plus a clean control fixture (Pitfall 3: the violation MUST live only under `testdata/` so `go vet ./...` ignores it).

---

### `Makefile` `vet` target (config, -vettool chaining)

**Analog:** existing chain (Makefile:20-50), read in full.

**Pattern to add** — a `VETTOOL_ABLATION_LEAKAGE=$(shell go env GOPATH)/bin/vet-ablation-leakage` var (mirror line 20-23); append it to the `vet:` prereq list (line 33) and add `$(GO) vet -vettool=$(VETTOOL_ABLATION_LEAKAGE) ./...` (mirror lines 35-38); add the install rule (mirror lines 40-50):
```makefile
$(VETTOOL_ABLATION_LEAKAGE): cmd/vet-ablation-leakage/main.go internal/lint/ablationleakage/*.go
	$(GO) install ./cmd/vet-ablation-leakage
```

---

### `internal/kernel/kernel.go` (model/config, two disable flags)

**Analog:** existing `KernelConfig` (kernel.go:18-21) + `k.config` storage (kernel.go:55), read in full.

**Pattern to copy (RESEARCH.md Pattern 1)** — extend the existing struct (no new struct; D-03 discretion favors simplicity):
```go
type KernelConfig struct {
	Pool                           lspool.PoolConfig
	DisableLSPSubsystem            bool // Phase 76 ABLATE-05
	DisableStructuredEditSubsystem bool // Phase 76 ABLATE-07
}
```
`NewKernel` already stores `config: cfg` (kernel.go:55) — no constructor signature change. Add accessors next to `Tracer()` (kernel.go:63):
```go
func (k *Kernel) StructuredEditDisabled() bool { return k.config.DisableStructuredEditSubsystem }
func (k *Kernel) LSPSubsystemDisabled() bool   { return k.config.DisableLSPSubsystem }
```

---

### `internal/daemon/daemon.go` (composition root, conditional wiring)

**Analog:** kernel construction at daemon.go:274; SetEnrichFn block at daemon.go:658-670; repomap fallback `AcquireLease` at daemon.go:673-686; `resolveAllowedToolsForMode` at daemon.go:78-111. All read.

**Current kernel construction** (daemon.go:274):
```go
k := kernel.NewKernel(workspaces, langReg, installer, kernel.KernelConfig{Pool: poolCfg}, pressure, logger, observability.Metrics(), observability.Tracer())
```

**Pattern to apply (RESEARCH.md Pattern 2, D-02 precedence CLI||profile)** — compute effective flags before line 274 from the resolved `activeProfile` (already available) OR'd with the CLI overrides (read from `config.SerenaConfig`), then pass into the extended `KernelConfig`:
```go
effDisableLSP := cfg.DisableLSPSubsystem || activeProfile.DisableLSPSubsystem
effDisableSE  := cfg.DisableStructuredEditSubsystem || activeProfile.DisableStructuredEditSubsystem
// plain OR is correct: default-OFF, opt-in disable — CLI is a one-way force-disable.
```

**no_lsp null-object wiring (D-09, RESEARCH.md Pattern 3)** — when `effDisableLSP`:
- skip `SetEditNotifier(live)` (or install a no-op) — `EditNotifier()` returns nil and all 4 call sites already nil-check (notifier.go:47-63; fileops/tools.go:447,463).
- skip the `rs.SetEnrichFn(...)` block at daemon.go:658-670 (repomap falls back to tree-sitter).
- the profile excludes pool-backed symbol/diag tools, so symbol-tool lease paths are unreachable → no `lspool.lsp.*` span.

**D-10 audit (Pitfall 1)** — the repomap fallback `AcquireLease` (daemon.go:673-686) and the diag `leaseFn` are NOT tool-gated by the profile; under no_lsp they need the SetEnrichFn-skip (repomap) and an explicit guard/no-op (diag). Enumerate every `k.Pool()`/`AcquireLease`/`AcquireSession` reachable from a tool and confirm each is either excluded or guarded.

---

### `internal/kernel/edit/tools.go` (controller, structured-edit Unsupported guard)

**Analog:** the `replace_symbol_body` handler (edit/tools.go:303-369), read; `serr.Unsupported` kind (errors/kinds.go:17,32), read.

**Existing error-return shape to mirror** (edit/tools.go:317-318) — the handlers already return `errorResult(serr.New(serr.<Kind>, "...").WithTool("...").Error())`.

**Pattern to add (D-04, RESEARCH.md Code Example)** — near the top of each structured-edit handler (`replace_symbol_body`, `fuzzy_edit`, `insert_before_symbol`, `insert_after_symbol`), after arg validation:
```go
if k.StructuredEditDisabled() {
	return errorResult(serr.New(serr.Unsupported,
		"subsystem_disabled: replace_symbol_body requires the structured-edit subsystem; use replace_in_file").
		WithTool("replace_symbol_body").Error()), nil, nil
}
```
Reuse the existing `serr.Unsupported` kind (do NOT invent a new kind — anti-pattern). D-06 discretion: standardize a greppable `subsystem_disabled:` message prefix and document it on `kinds.go` near `Unsupported`.

---

### `internal/kernel/fileops/tools.go` (controller, replace_in_file exact-match-only)

**Analog:** the `replace_in_file` fuzzy-fallback block itself (fileops/tools.go:414-453), read.

**Existing fuzzy-fallback guard** (fileops/tools.go:415):
```go
if count == 0 && !args.IsRegex {
	// ... fuzzy.Match block ...
}
```

**Pattern to apply (D-05, Pitfall 2)** — add `&& !k.StructuredEditDisabled()` to the guard so that under the flag a no-match falls through to the existing plain `"0 replacement(s) made"` return (the path at fileops/tools.go:426 already exists):
```go
if count == 0 && !args.IsRegex && !k.StructuredEditDisabled() {
```
Test asserts no `match_strategy:` line appears in output under the flag, and outcome classification stays correct.

---

### `internal/cli/root.go` (config, CLI override flags)

**Analog:** the `--profile` persistent flag (root.go:67-68), read.

**Pattern to copy (D-01, A4)** — add two bool flags mirroring the `rootCmd.Flags().Bool(...)` style at root.go:58:
```go
rootCmd.Flags().Bool("disable-lsp-subsystem", false, "Force-disable the LSP subsystem (ablation override)")
rootCmd.Flags().Bool("disable-structured-edit-subsystem", false, "Force-disable structured edits (ablation override)")
```
Bind via koanf posflag into `config.SerenaConfig` (same path the existing flags use) so D-02 precedence holds. Add the corresponding `DisableLSPSubsystem`/`DisableStructuredEditSubsystem` fields to `config.SerenaConfig`.

---

## Shared Patterns

### Typed error taxonomy (`serr`)
**Source:** `internal/errors/kinds.go:13-38` (the `Unsupported` kind + `ErrUnsupported` sentinel already exist).
**Apply to:** all structured-edit handlers in `edit/tools.go`.
```go
return errorResult(serr.New(serr.Unsupported, "subsystem_disabled: ...").WithTool("<tool>").Error()), nil, nil
```
Do NOT add a new kind; reuse `serr.Unsupported` (ABLATE-07 literally says "return unsupported").

### Nil-checked fire-and-forget notifier
**Source:** `internal/kernel/notifier.go:47-63` (the `EditNotifier()` accessor + nil-check contract).
**Apply to:** the no_lsp wiring in `daemon.go` and all 4 `OnEdit` call sites (already nil-check at fileops/tools.go:447,463). Skipping `SetEditNotifier` is sufficient to no-op `OnEdit` everywhere.

### Import-boundary lint (house pattern)
**Source:** `internal/lint/noduckdb/analyzer.go` + `cmd/vet-noduckdb/main.go` + Makefile:20-50.
**Apply to:** the entire `vet-ablation-leakage` deliverable (analyzer, singlechecker, Make wiring, testdata).

### EMPTY-LISTS tool filtering
**Source:** `internal/profile/profiles/baseline.yaml:1-16` header + `skill.ResolveTools(...)` flow.
**Apply to:** all 4 bench YAMLs; reuse `exclude_tools` and `skills` lists rather than any disable-all flag.

## No Analog Found

None — every Phase 76 deliverable has a complete, tested in-repo precedent (RESEARCH.md "Don't Hand-Roll" + Sources). There is no novel external technology and no new third-party packages.

## Metadata

**Analog search scope:** `internal/lint/`, `cmd/vet-*`, `internal/profile/`, `internal/kernel/`, `internal/daemon/`, `internal/errors/`, `internal/cli/`, `Makefile`.
**Files scanned:** baseline.yaml, profile.go, loader.go, kernel.go, notifier.go, errors/kinds.go, noduckdb/analyzer.go, cmd/vet-noduckdb/main.go, Makefile, edit/tools.go, fileops/tools.go, daemon.go, cli/root.go.
**Line anchors:** taken from RESEARCH.md §Sources (codebase-VERIFIED this session) and re-confirmed by direct read.
**Pattern extraction date:** 2026-06-16
