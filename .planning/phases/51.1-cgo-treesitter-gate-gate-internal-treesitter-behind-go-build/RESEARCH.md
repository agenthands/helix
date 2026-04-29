# Phase 51.1: cgo-treesitter-gate - Research

**Researched:** 2026-04-29
**Domain:** Go build-tag gating across CGO boundary; daemon bootstrap refusal hook
**Confidence:** HIGH (all findings verified by direct file reads + grep + git inspection)

## Summary

Phase 51.1 gates the entire `internal/treesitter` package behind `//go:build cgo`
with a `//go:build !cgo` stub, plus a daemon-level refusal hook so the runtime exits
non-zero under `CGO_ENABLED=0` instead of silently returning an empty grammar set.

**Critical discovery (signature-level conflict — see §4):** `internal/treesitter/registry.go`
is **not** the only CGO-tainted file in the project. Three callers
(`internal/repomap/extractor.go`, `internal/repomap/elide.go`,
`internal/kernel/edit/treesitter.go`) directly import
`tree_sitter "github.com/tree-sitter/go-tree-sitter"` — itself a CGO package
(`/Users/Janis_Vizulis/go/pkg/mod/github.com/tree-sitter/go-tree-sitter@v0.24.0/language.go`
line 7: `import "C"`). They will **not** compile under `CGO_ENABLED=0` even if
`internal/treesitter` is gated. Same applies for the public type
`*tree_sitter.Language` returned by `GrammarRegistry.GetLanguage(...)` — a `!cgo`
stub cannot return that type without itself depending on the CGO module.

**Primary recommendation:** Apply build-tag splits at THREE package boundaries
(`internal/treesitter`, `internal/repomap`, `internal/kernel/edit`), not just one.
For each, gate every `.go` file that names `tree_sitter.*` symbols. The stub for
`internal/treesitter` re-exposes `GrammarRegistry` as an opaque struct with the
same method set but a **`bool ok` always `false`** return shape — preserving the
`(lang, ok)` idiom callers use today. Daemon refusal fires via a constant
`treesitter.Available` from a build-tagged `available_cgo.go` / `available_nocgo.go`
pair, checked at `internal/daemon/daemon.go:196`.

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** `!cgo` stub preserves public API surface but every public method returns
  an error / sentinel ("tree-sitter unavailable in this build"). No silent empty
  registry. CGO=0 binaries are placeholders for goreleaser archive count, not
  functional builds.
- **D-02:** Daemon **refuses to start** under `CGO_ENABLED=0`. Detection at
  `internal/daemon/daemon.go:196` (call site of `treesitter.NewGrammarRegistry()`),
  exit non-zero with a message naming (a) cause ("tree-sitter / CGO"),
  (b) remediation ("use the CGO=1 release binary" / "rebuild with `CGO_ENABLED=1`").
  Build still succeeds (acceptance criterion 1 is about the *build*, not runtime).
- **D-03:** Tag downstream tree-sitter-exercising tests `//go:build cgo`. Do NOT
  add `!cgo` "asserts empty registry" tests.
- **D-04:** CGO=0 build smoke gate is the existing release-snapshot dry-run path
  (currently flagged "expected to fail until DEF-51-02" in **CONTRIBUTING.md** —
  not in any CI workflow file). Flip to expected-pass.
- **D-05:** Update `.planning/phases/51-packaging-goreleaser/deferred-items.md`:
  flip DEF-51-02 status `OPEN` → `RESOLVED` with pointer to phase 51.1.
- **D-06:** Whether to publish CGO=0 archives on real `v*` tags is deferred to
  Phase 52 / DEF-51-03. Capture is in scope; resolution is not.

### Claude's Discretion
- Exact file split shape (single `_cgo.go`/`_nocgo.go` swap vs per-file pairs).
- Exact mechanism for daemon `!cgo` detection.
- Exact wording of refusal message (subject to D-02 must-have content).
- Exact set of test files to tag.
- Whether `internal/treesitter/bindings/{r,swift}/binding_nocgo.go` survive,
  fold away, or are removed.
- Whether to file follow-up DEF-51-03 entry now or defer to Phase 52.

### Deferred Ideas (OUT OF SCOPE)
- Vendor + stub all 19 upstream tree-sitter Go bindings (DEF-51-02 Path 1).
- Re-enable CGO in goreleaser with cross-toolchain (Path 3) — separate DEF-51-03 candidate.
- Pure-Go tree-sitter via WASM (Path 4).
- Homebrew/Scoop/native packages (Phase 52).
- Restoring R/Swift to `!cgo` registry (already moot under D-01).

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| AC-1 | `CGO_ENABLED=0 go build ./cmd/serena` exits 0 | §1 file split + §4 caller gating provides compile path |
| AC-2 | CGO=1 build byte-identical, all 23 languages registered | §1 `_cgo.go` is verbatim copy of current `registry.go` |
| AC-3 | `goreleaser snapshot` produces 6 archives | §6 release.yml runs `release --snapshot` reproducibility gate |
| AC-4 | CGO=0 build smoke gated in CI | §6 — release.yml's two snapshot-pass gate IS the smoke gate; no separate workflow needed |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Tree-sitter grammar registry | `internal/treesitter` | — | Single source of truth; CGO entry point |
| Tree-sitter parsing for symbol body extraction | `internal/kernel/edit` | `internal/treesitter` (consumer) | Edit subsystem owns body surgery; gates on registry presence |
| Tree-sitter tag extraction for RepoMap | `internal/repomap` | `internal/treesitter` (consumer) | Repomap owns tag/elide/render; gates on registry presence |
| Daemon refusal under `!cgo` | `internal/daemon` | `internal/treesitter` (probe source) | Daemon owns startup lifecycle; reads `treesitter.Available` |

---

## §1 — File Inventory under `internal/treesitter/`

**Non-test `.go` files in `internal/treesitter/` (excluding bindings subdir):**

| File | Imports CGO bindings? | Action |
|------|------------------------|--------|
| `internal/treesitter/registry.go` | YES — 19 upstream `tree-sitter-*` packages + `tree_sitter` core (lines 10-40) | Split into `registry_cgo.go` (verbatim copy with `//go:build cgo`) + `registry_nocgo.go` (stub) |

There is **only one** non-test Go file in the package. The cleanest split shape is
**a straight rename**:

- **Rename** `internal/treesitter/registry.go` → `internal/treesitter/registry_cgo.go`
  and add `//go:build cgo` as line 1 (blank line 2 required by go vet).
- **Create new** `internal/treesitter/registry_nocgo.go` with `//go:build !cgo`
  exposing the **same public type and method set** but with no CGO imports
  and `(lang, ok)` always returning `(nil, false)`. See §4 for the signature
  question.

**Recommended `registry_nocgo.go` skeleton:**

```go
//go:build !cgo

package treesitter

import "sort"

// GrammarRegistry under !cgo carries no grammars. Methods preserve the public
// signature so callers (RepoMap, edit) compile, but every lookup returns (nil, false).
// Callers MUST check the bool. The daemon refuses to start before any caller is
// invoked (see internal/daemon/daemon.go step 6).
type GrammarRegistry struct{}

func NewGrammarRegistry() *GrammarRegistry          { return &GrammarRegistry{} }
func (r *GrammarRegistry) SupportsLanguage(string) bool   { return false }
func (r *GrammarRegistry) SupportedLanguages() []string   { s := []string{}; sort.Strings(s); return s }
// GetLanguage's return type is the source of the signature conflict — see §4.
```

**Also add** `internal/treesitter/available_cgo.go` (line 1: `//go:build cgo`,
`const Available = true`) and `internal/treesitter/available_nocgo.go`
(line 1: `//go:build !cgo`, `const Available = false`) for the daemon probe (§3).

## §2 — Test File Inventory (need `//go:build cgo`)

Every `_test.go` file that imports `internal/treesitter` or any
`github.com/tree-sitter/...` package needs `//go:build cgo` as line 1.

| File | Imports treesitter pkg | Imports tree_sitter core | Action |
|------|------------------------|--------------------------|--------|
| `internal/treesitter/registry_test.go` | (same package) calls `NewGrammarRegistry`, `SupportsLanguage`, `GetLanguage` | indirect via registry | Add `//go:build cgo` |
| `internal/repomap/extractor_test.go` | YES (line 6) | indirect | Add `//go:build cgo` |
| `internal/repomap/elide_test.go` | YES (line 6) | indirect | Add `//go:build cgo` |
| `internal/repomap/polyglot_rank_test.go` | YES (line 11) | indirect | Add `//go:build cgo` |
| `internal/repomap/render_test.go` | YES (line 12) | indirect | Add `//go:build cgo` |
| `internal/skill/repomap/skill_test.go` | YES (line 18) | indirect | Add `//go:build cgo` |
| `internal/skill/repomap/cache_persistence_test.go` | YES (line 14) | indirect | Add `//go:build cgo` |
| `internal/skill/repomap/skill_integration_test.go` | YES (line 14) | indirect | Add `//go:build cgo` |
| `internal/kernel/edit/treesitter_test.go` | YES (line 7) | indirect | Add `//go:build cgo` |

**Total: 9 test files.** All identified by:
```
grep -rn "internal/treesitter\|tree-sitter/go-tree-sitter\|tree-sitter/tree-sitter-\|tree-sitter-grammars/" --include="*.go" -l
```
plus inspecting each `_test.go` hit.

**Verification step for the planner:** after adding tags, run
`CGO_ENABLED=0 go vet ./...` and `CGO_ENABLED=0 go test ./...` — both must succeed.
Any other test file the planner discovers compiling against gated symbols gets the
same tag.

## §3 — Daemon Refusal Hook

**Call site** — `internal/daemon/daemon.go`:

```
line  41:  "github.com/postfix/serena/internal/treesitter"
line 193:  // 6. Create diagnostic store and body extractor.
line 194:  // NOTE: step numbering preserved from original New() for git-blame continuity.
line 195:  diagStore := diag.NewDiagnosticStore()
line 196:  grammarRegistry := treesitter.NewGrammarRegistry()
line 197:  bodyExtractor := edit.NewBodyExtractor(grammarRegistry)
```

**Recommended mechanism — build-tagged `const Available bool`:**

Insert refusal **immediately before** line 196 (the `NewGrammarRegistry()` call),
so the abort fires before any optional/degraded mode logic runs (CONTEXT
`code_context`: "fail fast"):

```go
// 6a. Refuse to start under CGO_ENABLED=0 (DEF-51-02 / Phase 51.1).
//     The CGO=0 binary is a goreleaser archive-count placeholder; tree-sitter
//     parsing, RepoMap tag extraction, and replace_symbol_body all depend on
//     CGO bindings. Surface the unavailability loudly rather than starting in
//     a permanently degraded state.
if !treesitter.Available {
    return nil, fmt.Errorf("tree-sitter is unavailable in this build (CGO_ENABLED=0): " +
        "rebuild with CGO_ENABLED=1 or download the CGO=1 release binary " +
        "(see CONTRIBUTING.md > Releasing)")
}

// 6. Create diagnostic store and body extractor.
diagStore := diag.NewDiagnosticStore()
grammarRegistry := treesitter.NewGrammarRegistry()
```

**Rationale for `const Available` over alternatives:**

| Option | Pro | Con |
|--------|-----|-----|
| **`const Available bool` (recommended)** | Compile-time eliminated under cgo; trivially testable; symmetric with proven `binding_nocgo.go` pattern from 51-03 | None significant |
| Sentinel error from `NewGrammarRegistry()` | API-symmetric | Forces signature change `(*GrammarRegistry, error)` — caller diff in 4+ test files and skill wiring |
| `treesitter.IsAvailable()` probe | Works | More code than needed; same effect |

`fmt.Errorf` returns through daemon `New()` — `cmd/serena/main.go` already exits
non-zero on daemon construction error (existing pattern; planner verifies).

**Note on imports:** `daemon.go` already imports `"fmt"` (used elsewhere in the
file at lines 206, 220, etc.). No new import needed.

## §4 — Caller Compile Verification under `!cgo` (CRITICAL)

This is the planner's hardest decision point. Three caller files import
`tree_sitter "github.com/tree-sitter/go-tree-sitter"` directly — and that import
is itself CGO-only (verified at `~/go/pkg/mod/github.com/tree-sitter/go-tree-sitter@v0.24.0/language.go:7: import "C"`).

### Files affected and the tree_sitter symbols they use

| File | tree_sitter imports | Treesitter pkg API used |
|------|---------------------|--------------------------|
| `internal/repomap/extractor.go` | `tree_sitter.Query`, `tree_sitter.Node`, `tree_sitter.NewQuery`, `tree_sitter.NewParser`, `tree_sitter.NewQueryCursor` | `*treesitter.GrammarRegistry`, `GetLanguage(lang) (*tree_sitter.Language, bool)` (line 91, et al.) |
| `internal/repomap/elide.go` | `tree_sitter.Language`, `tree_sitter.Node`, `tree_sitter.NewParser` | `*treesitter.GrammarRegistry`, `GetLanguage` |
| `internal/kernel/edit/treesitter.go` | `tree_sitter.NewParser`, `tree_sitter.Node` | `*treesitter.GrammarRegistry`, `GetLanguage` |
| `internal/skill/repomap/skill.go` | (none — only the `treesitter` package) | `*treesitter.GrammarRegistry`, `SetRegistry` (line 120), `NewTagExtractor` |

### The signature-level conflict

`GrammarRegistry.GetLanguage` returns `(*tree_sitter.Language, bool)`. The
**`!cgo` stub cannot name `*tree_sitter.Language`** — that type lives inside the
CGO-only `github.com/tree-sitter/go-tree-sitter` module. The stub only works if
**the caller files are also gated**, OR if the public return type is changed to
`(any, bool)` / `(unsafe.Pointer, bool)` (which would force CGO-side caller
changes).

**Recommended resolution — gate the caller files too.**

Because `extractor.go`, `elide.go`, and `kernel/edit/treesitter.go` all directly
name `tree_sitter.*` types in function signatures and locals, they cannot compile
under `!cgo` regardless of what `internal/treesitter`'s stub looks like. The
minimum-diff fix is:

1. Add `//go:build cgo` as line 1 of each of the three caller files.
2. Create `!cgo` stub siblings:
   - `internal/repomap/extractor_nocgo.go` — exports a stub `TagExtractor` whose
     `NewTagExtractor` returns `(*TagExtractor, error)` with a non-nil error,
     and whose extraction methods are unreachable (daemon refused at step 6a).
   - `internal/repomap/elide_nocgo.go` — exports a stub `ElisionRenderer` whose
     methods all `panic("unreachable: tree-sitter unavailable")` or return
     empty strings; in practice the daemon refusal in §3 means these are
     never invoked.
   - `internal/kernel/edit/treesitter_nocgo.go` — exports a stub `BodyExtractor`
     with `NewBodyExtractor(*treesitter.GrammarRegistry) *BodyExtractor` and
     `ExtractBody` returning `(0, 0, error)`.
3. **Keep the `(lang, ok)` shape** — under `!cgo`, change `GetLanguage`'s return
   type to `(any, bool)` and document that callers use it only after the CGO-tag
   gate. Wait — see refinement below.

### Refinement: types in stubs

Because callers under `!cgo` are themselves stubs (they only need to *compile*,
not function — daemon refused at step 6a), the simplest scheme is to redefine
`GrammarRegistry`'s no-cgo stub with **only the methods that callers reference
across the file boundary**, and let internal-only methods (anything taking a
`*tree_sitter.Language`) live exclusively in the `_cgo.go` half. Concretely:

- `internal/treesitter/registry_nocgo.go`:
  ```go
  //go:build !cgo
  package treesitter
  type GrammarRegistry struct{}
  func NewGrammarRegistry() *GrammarRegistry         { return &GrammarRegistry{} }
  func (r *GrammarRegistry) SupportsLanguage(string) bool { return false }
  func (r *GrammarRegistry) SupportedLanguages() []string { return nil }
  // GetLanguage NOT defined here — only the _cgo.go file declares it,
  // because callers that USE the *tree_sitter.Language return are themselves
  // gated `//go:build cgo` (see extractor.go/elide.go/kernel/edit/treesitter.go).
  ```
- `internal/skill/repomap/skill.go` references `*treesitter.GrammarRegistry`
  and `treesitter.NewTagExtractor` (NOT `GetLanguage`). Verify by reading
  skill.go: it has `registry *treesitter.GrammarRegistry` (line 43) and calls
  `repomap.NewTagExtractor(registry)` (around line 130) — **only the type name
  and the `NewTagExtractor` constructor are reached from skill.go**. Both can
  live in the stub. **Skill.go itself does NOT need `//go:build cgo`** — but
  `repomap.NewTagExtractor` does, so skill.go ends up calling a stub that
  returns an error.

**Conclusion:** Three `//go:build cgo` files PLUS a small stub each in
`internal/repomap/` (2 files) and `internal/kernel/edit/` (1 file). Skill files
remain ungated. The `internal/treesitter` stub omits `GetLanguage` entirely.

**Planner ACTION ITEM:** verify `internal/skill/repomap/skill.go` does not
transitively reference `*tree_sitter.Language` after the gating proposal lands.
A `CGO_ENABLED=0 go build ./...` is the canonical check.

## §5 — R/Swift Binding Stubs from 51-03

| File | Status under D-01 |
|------|-------------------|
| `internal/treesitter/bindings/r/binding.go` (`//go:build cgo`) | Survives — still loaded by `registry_cgo.go` |
| `internal/treesitter/bindings/r/binding_nocgo.go` (`//go:build !cgo`) | **Dead code under D-01** — `registry_nocgo.go` does not import the bindings package. Recommend **delete** for minimum diff drift. |
| `internal/treesitter/bindings/swift/binding.go` | Survives — same reason |
| `internal/treesitter/bindings/swift/binding_nocgo.go` | **Dead code** — recommend delete. |

**Recommendation:** Delete the two `binding_nocgo.go` files. They were correct
under DEF-51-01 (where the rest of the package compiled under `!cgo`); under
51.1's D-01, the entire package is gated, so the `!cgo` half of the
bindings is unreachable. Keeping them adds noise without value.

**Rationale on `src/` directories:** The `bindings/{r,swift}/src/` directories
hold the C parser sources used by the `cgo` binding files. They stay.

## §6 — Release-Snapshot Dry-Run Gate (D-04)

**There is no separate dry-run CI workflow.** Verified by:
- `git show 1a1ce93a --stat` — only `CONTRIBUTING.md` was edited, +3 −1 lines.
- `grep -rn "release-snapshot\|snapshot" .github/workflows/` — only matches are
  in `release.yml` (the real release workflow's reproducibility gate at lines
  82-113), which only runs on `push: tags: ['v*']`.

**The "expected to fail until DEF-51-02" marker is in `CONTRIBUTING.md`**, not in
CI. Specific lines to flip:

- **`CONTRIBUTING.md:169`** — add the "Known issue (phase 51, DEF-51-02)"
  callout block. **Action:** delete the entire callout (the `>` blockquote
  block introduced by 1a1ce93a).
- **`CONTRIBUTING.md:215`** — currently reads:
  > "Once DEF-51-02 closes, the dry-run will validate the build matrix,
  > archive packaging, and checksums.txt generation; until then it fails at
  > the build step (see the 'Known issue' callout above)."
  **Action:** restore to pre-1a1ce93a wording:
  > "The dry-run still validates the build matrix, archive packaging, and
  > checksums.txt generation."

**The actual CGO=0 smoke gate** is the existing two-pass reproducibility gate
in `.github/workflows/release.yml` lines 82-113 (`Reproducibility gate -- snapshot
pass 1` and `pass 2`). That job already invokes `goreleaser release --snapshot
--clean --skip=sign` which goreleaser runs under `CGO_ENABLED=0` per
`.goreleaser.yaml`. **Today it fails at archive build for the 19 import-constraint
errors enumerated in DEF-51-02.** After 51.1 lands, the same job becomes
expected-pass — no workflow YAML edits needed.

**Optional CI nicety (planner discretion):** Add a `CGO_ENABLED=0 go vet ./...`
step to `.github/workflows/go-test.yml` between lines 93 and 96 so PR-time CI
catches `!cgo` regressions before tag time. Not required by any acceptance
criterion; the snapshot pass on real tags is the contract gate per D-04.

**Verification command** the planner should add to validation steps:
```
CGO_ENABLED=0 go build ./cmd/serena
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
make release-snapshot
```

## §7 — `deferred-items.md` Update for DEF-51-02 (D-05)

**File:** `.planning/phases/51-packaging-goreleaser/deferred-items.md`

**Section to rewrite — line 127:**
```
**Status:** OPEN -- requires architectural decision in a separate phase (Phase 52+ recommendation).
```

**Replace with:**
```
**Status:** RESOLVED via Phase 51.1 (Path 2). See `.planning/phases/51.1-cgo-treesitter-gate-gate-internal-treesitter-behind-go-build/` for PLAN, RESEARCH, and SUMMARY. The `internal/treesitter` package is now gated behind `//go:build cgo`; the daemon refuses to start under `CGO_ENABLED=0` with a clear remediation message. CGO=0 archives build but are placeholders — DEF-51-03 (re-enable CGO in goreleaser, Path 3) tracks the follow-up to make them functional.
```

**Optional sibling update (D-06 capture):** add `DEF-51-03` entry below
DEF-51-02 noting the publish-or-not policy for placeholder CGO=0 archives.
Planner discretion per CONTEXT line 67.

**Recommendation lines (118-123)** — `Path 2` paragraph (line 119) is the path
chosen. No edit needed; the **Status** flip is sufficient.

## Standard Stack

No new libraries are introduced by this phase. The pattern reused is the
**Go build constraint** (`//go:build cgo` / `//go:build !cgo`), already proven
in `internal/treesitter/bindings/{r,swift}/binding{_nocgo,}.go` from Phase 51-03.

| Mechanism | Source | Confidence |
|-----------|--------|------------|
| `//go:build cgo` and `//go:build !cgo` constraints | Go toolchain (built-in); 51-03 reference impl | HIGH (verified in repo) |
| Build-tagged `const Available bool` | Standard Go pattern | HIGH |
| `fmt.Errorf` for daemon construction failure | Existing pattern in `daemon.go` (lines 206, 220) | HIGH |

## Don't Hand-Roll

| Problem | Don't Build | Use Instead |
|---------|-------------|-------------|
| Runtime CGO detection | `runtime.GOROOT` parsing, `cgo.Available` from runtime | Build-tagged `const Available` (compile-time, zero overhead) |
| Stub registry that "looks alive" | Empty-but-functional `GrammarRegistry` | Daemon refuses at step 6a — stubs only need to compile |

## Common Pitfalls

### Pitfall 1: Forgetting tree_sitter core is itself CGO
**What goes wrong:** Author gates `internal/treesitter/registry.go` only, then
`CGO_ENABLED=0 go build ./...` still fails on `internal/repomap/extractor.go`
because that file imports `tree_sitter "github.com/tree-sitter/go-tree-sitter"`
directly.
**How to avoid:** Run `grep -rn "tree-sitter/go-tree-sitter" --include="*.go"`
and gate every file that hits. See §4 inventory.

### Pitfall 2: Stub method signature mismatch
**What goes wrong:** The `!cgo` stub for `GrammarRegistry.GetLanguage` cannot
return `*tree_sitter.Language` because `tree_sitter` is the CGO module.
**How to avoid:** Don't define `GetLanguage` in the `!cgo` stub at all. Gate
every caller of `GetLanguage` with `//go:build cgo` (these are all in `repomap`
and `kernel/edit`). The stub only needs the methods skill-layer code reaches.

### Pitfall 3: `//go:build` constraint syntax
**What goes wrong:** Constraint must be on **line 1** with a **blank line 2**
before the `package` declaration. Otherwise Go silently treats it as a regular
comment.
**How to avoid:** Always:
```
//go:build cgo

package treesitter
```

### Pitfall 4: Forgetting refusal must precede `NewGrammarRegistry()`
**What goes wrong:** Author calls `NewGrammarRegistry()` first, then checks
`Available`. Under `!cgo` the stub returns a valid empty struct so this works,
but it's a latent foot-gun: any future side effect added to the stub
constructor would run.
**How to avoid:** Refusal block (§3) goes **before** line 196.

## Code Examples

### Example: build-tagged constant pair (proven pattern from 51-03)

`internal/treesitter/available_cgo.go`:
```go
//go:build cgo

package treesitter

const Available = true
```

`internal/treesitter/available_nocgo.go`:
```go
//go:build !cgo

package treesitter

const Available = false
```

### Example: daemon refusal (recommended insert at daemon.go:195)

```go
// 6a. Refuse to start under CGO_ENABLED=0 (DEF-51-02 / Phase 51.1).
if !treesitter.Available {
    return nil, fmt.Errorf("tree-sitter is unavailable in this build " +
        "(CGO_ENABLED=0): rebuild with CGO_ENABLED=1 or download the " +
        "CGO=1 release binary (see CONTRIBUTING.md > Releasing)")
}
```

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (`github.com/stretchr/testify`) |
| Config file | none (Go convention) |
| Quick run command | `go test ./internal/treesitter/...` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AC-1 | CGO=0 build exits 0 | smoke | `CGO_ENABLED=0 go build ./cmd/serena` | ✅ (toolchain) |
| AC-2 | CGO=1 byte-identical, 23 langs | unit | `go test ./internal/treesitter/... -count=1` | ✅ `registry_test.go` (after `cgo` tag) |
| AC-3 | goreleaser produces 6 archives | smoke | `make release-snapshot` then `ls dist/*.tar.gz \| wc -l` | ✅ `Makefile:50` |
| AC-4 | CI smoke gates CGO=0 | integration | release.yml reproducibility gate (already wired) | ✅ `.github/workflows/release.yml:82-113` |
| AC-1b | CGO=0 vet clean | smoke | `CGO_ENABLED=0 go vet ./...` | ✅ (toolchain) |
| AC-1c | CGO=0 test compiles | smoke | `CGO_ENABLED=0 go test ./... -count=1` | ✅ (toolchain — passes once tagged tests are excluded) |

### Sampling Rate
- **Per task commit:** `CGO_ENABLED=0 go vet ./... && go test ./internal/treesitter/...`
- **Per wave merge:** `CGO_ENABLED=0 go build ./cmd/serena && go test ./... -count=1 && CGO_ENABLED=1 go test ./... -count=1`
- **Phase gate:** `make release-snapshot` exits 0 with 6 archives in `dist/`.

### Wave 0 Gaps
- None — all required test files exist; no framework install needed.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All build/test | ✓ | go.mod requires 1.25.x (per `release.yml:41`) | — |
| C compiler (cc) | CGO=1 verification | ✓ (macOS xcode tools) | — | — |
| goreleaser | `make release-snapshot` | ✓ (per CONTRIBUTING.md) | `~> v2` (per release.yml:86) | — |

No missing dependencies.

## Architecture Patterns

### Recommended File Layout (post-phase)

```
internal/
├── treesitter/
│   ├── registry_cgo.go         # NEW — was registry.go, +//go:build cgo
│   ├── registry_nocgo.go       # NEW — !cgo stub (no GetLanguage method)
│   ├── available_cgo.go        # NEW — const Available = true
│   ├── available_nocgo.go      # NEW — const Available = false
│   ├── registry_test.go        # +//go:build cgo
│   └── bindings/
│       ├── r/
│       │   ├── binding.go              # unchanged
│       │   ├── binding_nocgo.go        # DELETE (dead under D-01)
│       │   └── src/                    # unchanged
│       └── swift/
│           ├── binding.go              # unchanged
│           ├── binding_nocgo.go        # DELETE
│           └── src/                    # unchanged
├── repomap/
│   ├── extractor.go            # +//go:build cgo
│   ├── extractor_nocgo.go      # NEW — stub TagExtractor (constructor returns error)
│   ├── elide.go                # +//go:build cgo
│   ├── elide_nocgo.go          # NEW — stub ElisionRenderer
│   ├── extractor_test.go       # +//go:build cgo
│   ├── elide_test.go           # +//go:build cgo
│   ├── polyglot_rank_test.go   # +//go:build cgo
│   └── render_test.go          # +//go:build cgo
├── kernel/edit/
│   ├── treesitter.go           # +//go:build cgo
│   ├── treesitter_nocgo.go     # NEW — stub BodyExtractor
│   └── treesitter_test.go      # +//go:build cgo
├── skill/repomap/
│   ├── skill.go                # UNCHANGED (only references type name + NewTagExtractor)
│   ├── skill_test.go                    # +//go:build cgo
│   ├── cache_persistence_test.go        # +//go:build cgo
│   └── skill_integration_test.go        # +//go:build cgo
└── daemon/
    └── daemon.go               # +5 lines at line 195 (refusal block)
```

**Total non-test file changes:**
- 1 rename + 3 new in `internal/treesitter/` (and 2 deletions in `bindings/`)
- 2 file-tag + 2 new in `internal/repomap/`
- 1 file-tag + 1 new in `internal/kernel/edit/`
- 1 small block in `internal/daemon/daemon.go`
- 9 test files tagged
- `CONTRIBUTING.md` + `deferred-items.md` documentation updates

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `cmd/serena/main.go` exits non-zero when `daemon.New()` returns an error | §3 | LOW — universal Go pattern; planner verifies during plan-checking. If wrong, the planner replaces `return nil, err` with explicit `os.Exit(1)` after logging. |
| A2 | `goreleaser release --snapshot` invokes `CGO_ENABLED=0` per project `.goreleaser.yaml` | §6 | LOW — DEF-51-02's existence proves it. Planner can spot-check `.goreleaser.yaml` for `env: [CGO_ENABLED=0]`. |
| A3 | `internal/skill/repomap/skill.go` does not transitively name any `tree_sitter.*` type at file scope | §4 | MEDIUM — verified by grep at line 19 (`"github.com/postfix/serena/internal/treesitter"`) being the only match; no `tree_sitter` (underscore) import. Planner re-runs `CGO_ENABLED=0 go vet ./internal/skill/repomap/...` post-implementation. |
| A4 | `*tree_sitter.Language` cannot be aliased / re-exported from a non-CGO file | §4 | LOW — the type's methods involve unsafe pointers wired to C symbols; even an alias `type Language = tree_sitter.Language` requires importing the CGO module. |

## Sources

### Primary (HIGH confidence — direct file reads)
- `internal/treesitter/registry.go` — lines 1-127 read in full
- `internal/treesitter/registry_test.go` — lines 1-151 read in full
- `internal/treesitter/bindings/r/binding.go` and `binding_nocgo.go` — read in full
- `internal/daemon/daemon.go` — lines 35-44, 180-310 read; `treesitter.NewGrammarRegistry()` confirmed at line 196
- `.github/workflows/release.yml` — read in full (no separate snapshot dry-run job exists)
- `.github/workflows/go-test.yml` — read in full
- `.planning/phases/51-packaging-goreleaser/deferred-items.md` lines 55-127 — DEF-51-02 read in full
- `git show 1a1ce93a` — confirmed CONTRIBUTING.md is the only file touched
- `~/go/pkg/mod/github.com/tree-sitter/go-tree-sitter@v0.24.0/language.go:7` — `import "C"` confirmed (CGO dependency)

### Secondary (HIGH confidence — grep-verified)
- File enumeration via `grep -rn "internal/treesitter\|tree-sitter/go-tree-sitter\|tree-sitter/tree-sitter-\|tree-sitter-grammars/" --include="*.go" -l` — 17 hits, 15 caller files identified

## Metadata

**Confidence breakdown:**
- File inventory: HIGH — direct reads
- Test inventory: HIGH — grep + read
- Daemon hook: HIGH — exact line numbers cited
- Caller signature analysis: MEDIUM — A3 (skill.go) flagged for planner re-verify
- Bindings cleanup: HIGH — proven dead under D-01
- CI workflow gate: HIGH — confirmed no separate workflow exists
- deferred-items.md update: HIGH — exact line cited

**Research date:** 2026-04-29
**Valid until:** 2026-05-29 (30 days; `internal/treesitter` is stable post-51-03, no churn expected)
