# Phase 57: Semantic Store Foundation + Pipeline DAG Library — Pattern Map

**Mapped:** 2026-05-03
**Files analyzed:** 26 new + 2 modified across 4 plans
**Analogs found:** 24 / 26 (two greenfield: `phasegraph` package + `vet-noduckdb` analyzer have no in-repo analog beyond stdlib singlechecker)

---

## File Classification

### P01 — `internal/phasegraph/` library

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `internal/phasegraph/phase.go` | library (types) | transform | *(none — greenfield, mirror SPEC §39.2)* | greenfield |
| `internal/phasegraph/dag.go` | library (algorithm) | transform | *(none — Kahn's algorithm from SPEC §39.3)* | greenfield |
| `internal/phasegraph/validate.go` | library (entrypoint) | transform | `internal/langregistry/installer.go:43-66` (multi-tier checked sequence) | partial — same "validate-then-act" shape |
| `internal/phasegraph/run.go` | library (executor) | transform | *(none — Kahn execution loop)* | greenfield |
| `internal/phasegraph/shutdown.go` | library (helper) | transform | *(none — reverse-topo)* | greenfield |
| `internal/phasegraph/dot.go` | library (writer) | file-I/O | `cmd/docgen/main.go` (text writer) | weak |
| `internal/phasegraph/pipelines/semantic.go` | shape declaration | static-data | *(none — typed ID consts + Requires/Provides shape)* | greenfield |
| `internal/phasegraph/pipelines/live.go` | shape declaration | static-data | *(see semantic.go above)* | greenfield |
| `internal/phasegraph/pipelines/eval.go` | shape declaration | static-data | *(see semantic.go above)* | greenfield |
| `internal/phasegraph/phasegraph_test.go` | unit test | testing | `internal/langregistry/installer_test.go` | role-match |

### P02 — `internal/semantic/{config,store,types}` skeleton

| New / Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------------|------|-----------|----------------|---------------|
| `internal/semantic/config.go` | typed config mirror | static-data | `internal/config/config.go:39-57` (`ObservabilityConfig`) | exact |
| `internal/semantic/types.go` | type aliases | static-data | `internal/langregistry/entry.go` | role-match |
| `internal/semantic/store/duckdb.go` (`//go:build cgo`) | service (open/close) | request-response + file-I/O | `internal/langregistry/installer.go:30-66` (three-tier) + diag store ctor | role-match |
| `internal/semantic/store/duckdb_nocgo.go` (`//go:build !cgo`) | CGO=0 stub | transform | `internal/repomap/extractor_nocgo.go` (verbatim mirror) | exact |
| `internal/semantic/store/migrations.go` | migration registry | static-data + transform | *(none — first migration registry in repo)* | greenfield |
| `internal/semantic/store/snapshot.go` | service (CRUD) | CRUD | `internal/kernel/diag/actions.go` (lease+session shape) | partial |
| `internal/semantic/store/overlay.go` | service skeleton | CRUD | `internal/kernel/diag/actions.go` | partial |
| `internal/semantic/store/effective.go` | service (read API) | request-response | SPEC §10 verbatim — no in-repo analog | spec-direct |
| `internal/semantic/store/store_test.go` | integration test | testing | `internal/langregistry/installer_test.go` | role-match |
| `internal/semantic/store/README.md` | docs | docs | `internal/repomap/README.md` (if present) / package doc.go | role-match |
| `internal/daemon/daemon.go` (modified) | bootstrap step 6b | event-driven | step 6a CGO refusal (lines 193-203) + step 6 diag store ctor (lines 205-209) | exact (insertion site) |

### P03 — `semantic_index.*` config keys

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------|------|-----------|----------------|---------------|
| `internal/config/defaults.go` (modified) | static-data | static-data | `defaults.go:20-24` (existing `observability.*` block) | exact |
| `internal/config/config.go` (modified — add `SemanticIndex` field + nested struct) | static-data | static-data | `config.go:5-23` (`SerenaConfig` + nested `Observability ObservabilityConfig`) | exact |
| `internal/config/loader_test.go` (modified — `TestLoad_SemanticIndexDefaults` + `TestLoad_SemanticIndexPrecedence`) | unit test | testing | `loader_test.go:92-120` (`TestLoad_ObservabilityDefaults` + `TestLoad_ObservabilityOverride`) | exact |

### P04 — `cmd/vet-noduckdb/` analyzer

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `cmd/vet-noduckdb/main.go` | binary entrypoint | request-response | `cmd/docgen/main.go` (single-purpose tool binary) | role-match |
| `internal/lint/noduckdb/analyzer.go` | analyzer (AST walk) | transform | *(none — first custom analyzer in repo; use stdlib `analysis.Analyzer` template)* | stdlib-direct |
| `internal/lint/noduckdb/analyzer_test.go` | unit test | testing | `internal/repomap/extractor_test.go` style — `analysistest.Run` is stdlib | stdlib-direct |
| `internal/lint/noduckdb/testdata/src/badpkg/imports.go` | test fixture | static-data | *(none — `analysistest` fixture convention)* | stdlib-direct |
| `Makefile` (modified — `vet` target + `$(VETTOOL)` rule) | build glue | build | `Makefile:21-22` (existing `vet:` target — replace) | exact (replacement site) |

---

## Pattern Assignments

### `internal/semantic/store/duckdb_nocgo.go` (CGO=0 stub) — P02

**Analog:** `internal/repomap/extractor_nocgo.go` (verbatim canonical template per CONTEXT.md D-CGO).

**Build-tag header pattern** (lines 1-3):
```go
//go:build !cgo

package repomap
```

**Stub struct + constructor pattern** (lines 11-21):
```go
// TagExtractor under !cgo is a stub; the daemon refuses to start before any
// instance is exercised at runtime (see internal/daemon/daemon.go).
type TagExtractor struct{}

// NewTagExtractor under !cgo returns a non-nil empty stub to mirror sibling
// stubs (NewBodyExtractor, NewElisionRenderer). Loud failure is delivered by
// the daemon refusal hook before any caller is exercised; methods on this
// stub still return errors as defense-in-depth if the refusal is bypassed.
func NewTagExtractor(_ *treesitter.GrammarRegistry) (*TagExtractor, error) {
    return &TagExtractor{}, nil
}
```

**Method-stub error sentinel pattern** (lines 23-27):
```go
// Extract is unreachable under !cgo at runtime (daemon refuses). Returns an
// error sentinel for safety if the stub is ever exercised directly.
func (*TagExtractor) Extract(_ []byte, _ string, _ string) ([]Tag, error) {
    return nil, fmt.Errorf("repomap.TagExtractor.Extract: tree-sitter unavailable (CGO_ENABLED=0)")
}
```

**Apply to `duckdb_nocgo.go` verbatim, with one substitution per CONTEXT.md D-CGO:** swap `fmt.Errorf("...")` for `serr.ErrUnsupported` (the typed sentinel from `internal/errors/kinds.go:30`). This is the only documented divergence from the Phase 51.1 template — use the typed error because Phase 57 RESEARCH explicitly leans on `serr.Unsupported` for STORE-02.

**Cross-check second instance:** `internal/kernel/edit/treesitter_nocgo.go:13-28` — same shape, includes a second helper method (`SupportsLanguage` returns false) so callers that branch on capability flags also short-circuit. Mirror: any `Store` capability accessor (e.g., `func (*Store) Available() bool`) should return `false` in the stub.

---

### `internal/semantic/store/duckdb.go` (CGO=1 open / quarantine / rebuild) — P02

**Analog:** `internal/langregistry/installer.go` — three-tier resolution body.

**Three-tier sequenced fallback pattern** (lines 43-66):
```go
// Resolve finds or installs the language server for the given entry.
// It returns the resolved command path and arguments.
func (i *Installer) Resolve(ctx context.Context, entry LSEntry) (command string, args []string, err error) {
    // Tier 1: Check PATH.
    if path, lookErr := exec.LookPath(entry.Command); lookErr == nil {
        i.logger.Debug("language server found in PATH", "language", entry.Language, "path", path)
        return path, entry.Args, nil
    }

    // Tier 2: Managed download if enabled and install info is available.
    if i.config.AutoInstall && entry.Install != nil {
        i.logger.Info("attempting managed install", "language", entry.Language, "type", entry.Install.Type)
        installed, installErr := i.download(ctx, entry)
        if installErr == nil {
            return installed, entry.Args, nil
        }
        i.logger.Warn("managed install failed", "language", entry.Language, "error", installErr)
        // Fall through to Tier 3.
    }

    // Tier 3: Helpful error.
    return "", nil, fmt.Errorf("language server %q not found for %q; install with: %s",
        entry.Command, entry.Language, entry.InstallHint())
}
```

**Mirror for `OpenSemanticStore`:**
- **Tier 1 (open existing+clean):** `sql.OpenDB` + `SELECT 1` + read `semantic_schema_version`. Success → migrate-in-place if backward-compatible.
- **Tier 2 (quarantine + rebuild):** classify error per RESEARCH Pitfall 1 (lock-vs-corruption taxonomy); rename to `<path>.corrupt.<unix-ts>`; emit slog event + counter increment (D-06/D-07); recreate fresh DB.
- **Tier 3 (hard fail):** rebuild also failed → return error so daemon refuses to start.

**Constructor + logger pattern** (lines 30-41):
```go
type Installer struct {
    config InstallerConfig
    logger *slog.Logger
}

// NewInstaller creates an Installer with the given configuration.
func NewInstaller(cfg InstallerConfig, logger *slog.Logger) *Installer {
    if logger == nil {
        logger = slog.Default()
    }
    return &Installer{config: cfg, logger: logger}
}
```

**Apply to `Store`:** same `Options{...}` value type, slog defaulting, no global state.

---

### Daemon bootstrap step 6b insertion — P02

**Analog:** `internal/daemon/daemon.go:193-209` — adjacent steps 6a (CGO refusal) and 6 (diag store).

**Step 6a — CGO=0 refusal pattern** (lines 193-203):
```go
// 6a. Refuse to start under CGO_ENABLED=0 (DEF-51-02 / Phase 51.1 / D-02).
//     The CGO=0 binary is a goreleaser archive-count placeholder; tree-sitter
//     parsing, RepoMap tag extraction, and replace_symbol_body all depend on
//     CGO bindings. Surface the unavailability loudly rather than starting in
//     a permanently degraded state. Under CGO=1, treesitter.Available is a
//     compile-time const true and the Go compiler eliminates this branch.
if !treesitter.Available {
    return nil, fmt.Errorf("tree-sitter is unavailable in this build " +
        "(CGO_ENABLED=0): rebuild with CGO_ENABLED=1 or download the " +
        "CGO=1 release binary (see CONTRIBUTING.md > Releasing)")
}
```

**Step 6 — diag store + body extractor pattern** (lines 205-209):
```go
// 6. Create diagnostic store and body extractor.
// NOTE: step numbering preserved from original New() for git-blame continuity.
diagStore := diag.NewDiagnosticStore()
grammarRegistry := treesitter.NewGrammarRegistry()
bodyExtractor := edit.NewBodyExtractor(grammarRegistry)
```

**Apply to step 6b (insert immediately between line 203 and line 205):**
```go
// 6b. Open semantic fact store when enabled (Phase 57, STORE-01..05).
//     Fail-fast core subsystem; on corruption auto-quarantines to
//     `<path>.corrupt.<ts>` and rebuilds fresh (SPEC §29.1, STORE-01).
//     When semantic_index.enabled=false, semanticStore is nil; consumers
//     must guard. Under CGO=0 the call is unreachable: step 6a returned.
var semanticStore *semanticstore.Store
if cfg.SemanticIndex.Enabled {
    /* OpenSemanticStore(...) per pattern above */
}
```

**TODO marker placement** (CONTEXT.md "Claude's Discretion"): insert at top of `newDaemon` (current line 145), as leading comment block:
```go
// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases) (DAG-04).
// The numbered imperative steps below are the future PhaseSpec set.
```

---

### `internal/semantic/config.go` (typed mirror) — P02

**Analog:** `internal/config/config.go:39-57` — `ObservabilityConfig` struct.

**Nested-config struct + koanf tag pattern** (lines 39-57):
```go
// ObservabilityConfig holds admin listener, pprof gating, and tracing settings.
// AdminAddr empty = disabled. Must be loopback (127.0.0.1/localhost/::1) — v1.3 adds auth.
type ObservabilityConfig struct {
    // AdminAddr is the loopback bind address for the admin listener.
    // Empty string disables the listener (D-02, D-05).
    AdminAddr string `koanf:"admin_addr"`
    // EnablePprof registers /debug/pprof/* handlers on the admin listener when true (D-10).
    // Default false: zero attack surface when disabled (D-12).
    EnablePprof bool `koanf:"enable_pprof"`

    // Phase 12: Tracing
    // TracingEndpoint is the OTLP/gRPC collector endpoint.
    // Empty string disables tracing entirely (D-11).
    TracingEndpoint string `koanf:"tracing_endpoint"`
    // TracingSampleRatio is the TraceIDRatioBased fraction.
    // 0.0 = off (default), 1.0 = sample everything (D-11).
    TracingSampleRatio float64 `koanf:"tracing_sample_ratio"`
    // ServiceName is the OTel resource service.name attribute.
    // Default: "helix" (D-11).
    ServiceName string `koanf:"service_name"`
}
```

**Apply to `semantic.Config`:** one-comment-per-field giving SPEC §25 default + the ID range that introduced it. Nest under `SerenaConfig.SemanticIndex` per CONTEXT.md `<code_context>` "Integration Points".

---

### `internal/config/defaults.go` (modified) — P03

**Analog:** the existing `observability.*` block in `defaults.go:20-24`.

**Default-key declaration pattern** (lines 20-24):
```go
"observability.admin_addr":           "",         // empty = admin listener disabled (D-02, D-05)
"observability.enable_pprof":         false,      // zero attack surface by default (D-12)
"observability.tracing_endpoint":     "",         // empty = tracing disabled (D-11)
"observability.tracing_sample_ratio": float64(0), // 0.0 = off by default (D-11)
"observability.service_name":         "helix",    // OTel resource service.name (D-11)
```

**Apply for every SPEC §25 `semantic_index.*` key** — column-aligned, end-of-line comment cites the SPEC default justification. Float defaults must be wrapped `float64(...)` exactly as observability does (koanf gotcha: untyped Go literals decode as `int`).

---

### `internal/config/loader_test.go` (modified) — P03

**Analog:** `loader_test.go:92-120` — `TestLoad_ObservabilityDefaults` + `TestLoad_ObservabilityOverride`.

**Per-feature defaults assertion pattern** (lines 92-103):
```go
func TestLoad_ObservabilityDefaults(t *testing.T) {
    cfg, err := Load("/nonexistent/global.yml", "", nil)
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }
    if cfg.Observability.AdminAddr != "" {
        t.Errorf("expected empty default admin_addr, got %q", cfg.Observability.AdminAddr)
    }
    if cfg.Observability.EnablePprof {
        t.Errorf("expected default enable_pprof=false, got true")
    }
}
```

**Per-feature override assertion pattern** (lines 105-120):
```go
func TestLoad_ObservabilityOverride(t *testing.T) {
    overrides := map[string]interface{}{
        "observability.admin_addr":   "127.0.0.1:9090",
        "observability.enable_pprof": true,
    }
    cfg, err := Load("/nonexistent/global.yml", "", overrides)
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }
    if cfg.Observability.AdminAddr != "127.0.0.1:9090" {
        t.Errorf("expected admin_addr 127.0.0.1:9090, got %q", cfg.Observability.AdminAddr)
    }
    if !cfg.Observability.EnablePprof {
        t.Errorf("expected enable_pprof=true, got false")
    }
}
```

**Apply for `TestLoad_SemanticIndexDefaults`:** assert every SPEC §25 default. **Apply for `TestLoad_SemanticIndexPrecedence`:** D-10 says cover "3-4 representative keys, one per layer per key" — pattern-mirror the existing `TestLoad_ProjectConfigOverridesGlobal` (lines 53-77) for the project-vs-global layer, plus the override map for CLI.

---

### `helix_semantic_store_quarantine_total` counter — P02

**Analog:** `internal/obs/metrics.go:109-117` — `LSPoolEvictions` (closed-enum `reason` carve-out, exact Phase 57 D-06/D-07 shape).

**Vector declaration pattern** (lines 109-117, inside `newMetrics()`):
```go
LSPoolEvictions: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "helix_lspool_evictions_total",
        Help: "LS worker evictions by reason (idle/pressure/crash/shutdown).",
    },
    // CONTEXT.md D-13: "reason" is a closed 4-value enum enforced at
    // emission sites. It is NOT in the D-04 RED allowlist; the CI label
    // lint carves it out explicitly for this family.
    []string{"language", "reason"},
),
```

**Helper-method emission pattern** (lines 216-220):
```go
// LSPoolEviction increments the eviction counter for a language + reason.
// reason must be one of {idle, pressure, crash, shutdown} (D-13).
func (m *Metrics) LSPoolEviction(language, reason string) {
    m.LSPoolEvictions.WithLabelValues(language, reason).Inc()
}
```

**Carve-out registration pattern** (`metrics_labels_test.go:22-25`):
```go
var carveOuts = map[string]map[string]bool{
    "helix_lspool_evictions_total": {"reason": true},
    ...
}
```

**Apply to `helix_semantic_store_quarantine_total`:**
- Add `SemanticStoreQuarantine *prometheus.CounterVec` field to `Metrics` struct (after `EditOutcome` at line 75).
- Construct in `newMetrics()` with labels `[]string{"workspace_label", "reason"}` per D-06.
- Add helper method `(m *Metrics) SemanticStoreQuarantine(workspaceLabel, reason string)` mirroring `LSPoolEviction`.
- Add carve-out: `"helix_semantic_store_quarantine_total": {"workspace_label": true, "reason": true}`.
- Per D-08, ship in P02 alongside the store, not as a separate plan.
- Per "Specific Ideas" line in CONTEXT.md, also add a sibling `helix_semantic_store_open_total{outcome=opened|quarantined|created}` counter — same pattern, second carve-out for `outcome`.

---

### `cmd/vet-noduckdb/main.go` (binary entrypoint) — P04

**Analog:** `cmd/docgen/main.go` is the closest in-repo "single-purpose tool binary". For singlechecker invocation, no in-repo analog exists — use the stdlib idiom (RESEARCH lines 376-386).

**Single-purpose tool entrypoint shape** (from `cmd/docgen/main.go` — read for `package main` + minimal-imports convention).

**Singlechecker idiom** (stdlib):
```go
package main

import (
    "github.com/agenthands/helix/internal/lint/noduckdb"
    "golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(noduckdb.Analyzer) }
```

---

### `internal/lint/noduckdb/analyzer.go` — P04

**Analog:** none in repo (first custom analyzer). Use stdlib `*analysis.Analyzer` template (RESEARCH lines 388-422).

**AST-walk analyzer body** (research-supplied):
```go
package noduckdb

import (
    "strings"

    "golang.org/x/tools/go/analysis"
)

const allowedPkgPrefix = "github.com/agenthands/helix/internal/semantic/store"
const forbiddenImport = "github.com/duckdb/duckdb-go"

var Analyzer = &analysis.Analyzer{
    Name: "noduckdb",
    Doc:  "fails if duckdb-go is imported outside internal/semantic/store/",
    Run: func(pass *analysis.Pass) (interface{}, error) {
        if strings.HasPrefix(pass.Pkg.Path(), allowedPkgPrefix) {
            return nil, nil
        }
        for _, file := range pass.Files {
            for _, imp := range file.Imports {
                path := strings.Trim(imp.Path.Value, `"`)
                if strings.HasPrefix(path, forbiddenImport) {
                    pass.Reportf(imp.Pos(),
                        "duckdb-go may only be imported from %s (got %s)",
                        allowedPkgPrefix, pass.Pkg.Path())
                }
            }
        }
        return nil, nil
    },
}
```

**Test pattern** (stdlib `analysistest`): `internal/lint/noduckdb/testdata/src/badpkg/imports.go` contains an import line `import _ "github.com/duckdb/duckdb-go/v2"  // want \`duckdb-go may only be imported from .*\`` and `analyzer_test.go` calls `analysistest.Run(t, analysistest.TestData(), noduckdb.Analyzer, "badpkg")`.

---

### Makefile `vet` target — P04

**Analog:** Makefile lines 21-22 (existing `vet:` target) — replace with the new build-the-tool-then-vet sequence.

**Existing pattern** (lines 21-22):
```make
vet:
	$(GO) vet ./...
```

**Apply replacement** (research lines 425-432):
```make
VETTOOL=$(shell go env GOPATH)/bin/vet-noduckdb

vet: $(VETTOOL)
	$(GO) vet ./...
	$(GO) vet -vettool=$(VETTOOL) ./...

$(VETTOOL): cmd/vet-noduckdb/main.go internal/lint/noduckdb/*.go
	$(GO) install ./cmd/vet-noduckdb
```

Per CONTEXT.md "Specifics", add `vet` as a prerequisite of `test` so the boundary is enforced from day 1: `test: vet` (or `ci: vet test`).

---

### `internal/phasegraph/` library — P01 (greenfield, stdlib-only)

No in-repo analog: the phasegraph library is the first DAG-orchestrator in the codebase. **Three pattern sources to honor:**

1. **API signatures** are pinned by SPEC §39.2-§39.3 (verbatim — RESEARCH lines 332-369). Do not invent new signatures.
2. **File layout convention** mirrors `internal/langregistry/` (single-package, multiple files split by responsibility — `entry.go` types, `registry.go` lookups, `installer.go` algorithms, `*_test.go` per file). Apply: `phase.go` types, `dag.go` graph build + cycle/dup/missing detection, `validate.go` entry point, `run.go` Kahn execution, `shutdown.go` reverse-topo, `dot.go` debug writer.
3. **Test-fixture-inline convention** per CONTEXT.md "Specifics" — "no `testdata/` directory; fixtures inline in `phasegraph_test.go`". Mirror `internal/langregistry/installer_test.go` (inline LSEntry literals).

**Pipelines sub-package shape declarations** are simple `[]PhaseSpec` literals with empty `Run` bodies. Pattern: declare typed phase ID consts (`const PhaseStore PhaseID = "store"`), then `var SemanticIndexPhases = []PhaseSpec{{ID: PhaseStore, Requires: nil, Provides: []string{"snapshot_id"}, Run: noopRun}, ...}`. P60/P67 will replace `noopRun` with real bodies.

---

## Shared Patterns

### CGO=0 stub pattern (Phase 51.1)

**Source:** `internal/repomap/extractor_nocgo.go` (canonical) + `internal/kernel/edit/treesitter_nocgo.go` (cross-check).

**Apply to:** `internal/semantic/store/duckdb_nocgo.go`.

**Invariant:** every CGO=1 source file in `internal/semantic/store/` MUST have a `//go:build cgo` header; every method declared in those files MUST have a stub counterpart in `duckdb_nocgo.go` returning `serr.ErrUnsupported`. Verified by `CGO_ENABLED=0 go build ./internal/semantic/store/...` succeeding.

### Bounded-label metric registration

**Source:** `internal/obs/metrics.go:109-117` (vector declaration) + `:216-220` (helper method) + `internal/obs/metrics_labels_test.go:22-25` (carve-out).

**Apply to:** `helix_semantic_store_quarantine_total` AND `helix_semantic_store_open_total` (per CONTEXT.md "Specifics" — two metrics, both new in P57).

**Invariant:** every new label name not in `AllowedLabels` (line 28: `tool_name, profile, mode, language, outcome`) MUST be added to `carveOuts` with a comment explaining the closed-enum value set (D-06 says reason ∈ {`corrupt_file`, `schema_forward_incompat`, `schema_unreadable`, `unknown`}).

### Typed-error sentinel reuse

**Source:** `internal/errors/kinds.go:30` — `ErrUnsupported = &Error{Kind: Unsupported}`.

**Apply to:** every CGO=0 stub method in `internal/semantic/store/duckdb_nocgo.go` AND every disabled-path method on the real `Store` when `cfg.SemanticIndex.Enabled == false` (defense-in-depth — daemon already returns nil store, but if a caller forgets to guard the nil, the method should return `serr.ErrUnsupported` not panic).

### Daemon imperative-bootstrap step convention

**Source:** `internal/daemon/daemon.go:160-209` — numbered comment block per step.

**Apply to:** the new step 6b. Numbering convention requires the format `// 6b. <one-line summary>.` followed by a blank-line-separated rationale paragraph (matches step 6a's style at lines 193-198).

### Per-feature `TestLoad_*Defaults` + `TestLoad_*Override`

**Source:** `internal/config/loader_test.go:92-120`.

**Apply to:** `TestLoad_SemanticIndexDefaults` (D-09) AND `TestLoad_SemanticIndexPrecedence` (D-10). **Do NOT generalize to a reflection-walking matrix** — CONTEXT.md "Deferred Ideas" rejects this for v1.10.

### Three-tier resolution sequence

**Source:** `internal/langregistry/installer.go:43-66` — `Resolve()` body.

**Apply to:** `OpenSemanticStore` per CONTEXT.md `<code_context>` "Established Patterns": existing+clean → quarantine+rebuild → hard fail. Maintain the explicit `Tier 1 / Tier 2 / Tier 3` comment ladder so the analog is grep-able.

---

## No Analog Found

Files where no in-repo close match exists; planner should rely on SPEC + RESEARCH directly:

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/phasegraph/phase.go` | library types | transform | First DAG-orchestrator types in repo; SPEC §39.2 is the source. |
| `internal/phasegraph/dag.go` | Kahn algorithm | transform | First topological sort in repo; SPEC §39.3 is the source. |
| `internal/phasegraph/run.go` | DAG executor | transform | First DAG executor; SPEC §39.3. |
| `internal/phasegraph/shutdown.go` | reverse-topo | transform | First reverse-topo helper; trivial slice reverse. |
| `internal/phasegraph/dot.go` | DOT writer | file-I/O | First analyzer/debug DOT emission in repo; SPEC §39.9 names path convention `.helix/debug/phasegraph-*.dot`. |
| `internal/phasegraph/pipelines/{semantic,live,eval}.go` | shape decls | static-data | Phase ID lists from SPEC §39.5-§39.7 verbatim. |
| `internal/semantic/store/migrations.go` | migration registry | static-data | First explicit-migration registry; CONTEXT.md D-02 specifies the `Migration{From, To, Kind}` shape. |
| `internal/semantic/store/effective.go` | snapshot⊕overlay read API | request-response | SPEC §10 verbatim pseudocode is the source. |
| `internal/lint/noduckdb/analyzer.go` | custom analyzer | transform | First custom analyzer; stdlib `*analysis.Analyzer` template. |
| `internal/lint/noduckdb/analyzer_test.go` | analysistest fixture | testing | First `analysistest.Run` fixture in repo. |

For these files, the planner should reference the SPEC sections + RESEARCH code blocks cited in each row rather than searching for non-existent in-repo analogs.

---

## Metadata

**Analog search scope:** `internal/repomap/`, `internal/kernel/edit/`, `internal/kernel/diag/`, `internal/langregistry/`, `internal/config/`, `internal/obs/`, `internal/daemon/`, `internal/errors/`, `cmd/`, `Makefile`.

**Files scanned:** 11 source files read in full or by targeted ranges; 4 directories listed for shape inventory.

**Pattern extraction date:** 2026-05-03.
