---
phase: 57
plan: 04
subsystem: lint/build-gate
tags: [lint, vet, analyzer, build-gate, tdd, store-06]
type: tdd
wave: 2
requirements: [STORE-04, STORE-06]
requirements_addressed: [STORE-06]
depends_on: [57-02]
dependency_graph:
  requires:
    - "internal/semantic/store/ allowlist prefix (no on-disk dependency; the analyzer matches by import-path string)"
    - "CONTEXT.md D-12 — duckdb-go module path lock (prefix 'github.com/duckdb/duckdb-go', major-version-agnostic)"
  provides:
    - "noduckdb.Analyzer — reusable go/analysis Analyzer enforcing STORE-06 import boundary"
    - "cmd/vet-noduckdb — singlechecker binary wrapping the analyzer"
    - "Makefile vet target that builds the analyzer then runs go vet -vettool=... ./..."
    - "test→vet Makefile dependency so the boundary is enforced on every test run"
  affects:
    - "internal/semantic/store/ — sole legitimate consumer of duckdb-go (P02)"
    - "Future v1.10 phases that may otherwise be tempted to import duckdb-go directly (P59 watcher, P60 enrich worker, etc.) — analyzer rejects at build time"
tech_stack:
  added: []
  patterns:
    - "go/analysis singlechecker pattern (golang.org/x/tools/go/analysis/{analysis,singlechecker,analysistest})"
    - "analysistest fixture layout under testdata/src/<derived-pkg-path>/"
    - "Make file-target rule (`$(VETTOOL): deps`) so the analyzer rebuilds whenever its sources change"
key_files:
  created:
    - "cmd/vet-noduckdb/main.go"
    - "internal/lint/noduckdb/analyzer.go"
    - "internal/lint/noduckdb/analyzer_test.go"
    - "internal/lint/noduckdb/testdata/src/badpkg/imports.go"
    - "internal/lint/noduckdb/testdata/src/github.com/agenthands/helix/internal/semantic/store/goodpkg/imports.go"
    - "internal/lint/noduckdb/testdata/src/github.com/duckdb/duckdb-go/v2/duckdb.go"
  modified:
    - "Makefile"
decisions:
  - "Used D-12 prefix form `github.com/duckdb/duckdb-go` for forbiddenImport (omits /v2) so future major-version bumps still match without revision."
  - "Created a stub `testdata/src/github.com/duckdb/duckdb-go/v2/duckdb.go` package because analysistest performs full type-checking and chokes on unresolvable imports (Rule 3 fix — plan optimistically claimed analysistest skips type-check; in reality it does not)."
  - "Makefile `vet` target runs `go vet ./...` first (stdlib analyzers) then `go vet -vettool=$(VETTOOL) ./...` (noduckdb gate) — preserves existing stdlib-vet pass, adds the boundary gate as a second pass."
metrics:
  duration_minutes: ~10
  completed_date: "2026-05-03"
  tasks_completed: 3
  files_changed: 7
  commits: 3
---

# Phase 57 Plan 04: cmd/vet-noduckdb (Build-Gate Analyzer for STORE-06) Summary

**One-liner:** Standalone `cmd/vet-noduckdb` singlechecker binary + `noduckdb.Analyzer` in `internal/lint/noduckdb/`, wired into `make vet` via `-vettool=` and chained from `make test`, that fails the build if any package outside `internal/semantic/store/` imports `github.com/duckdb/duckdb-go` — closes the boundary side of STORE-06.

## What Shipped

### `internal/lint/noduckdb/analyzer.go`
- `var Analyzer = &analysis.Analyzer{Name: "noduckdb", ...}` — go/analysis Analyzer.
- **`allowedPkgPrefix = "github.com/agenthands/helix/internal/semantic/store"`** — verified against the helix module name (`go.mod` line 1: `module github.com/agenthands/helix`).
- **`forbiddenImport = "github.com/duckdb/duckdb-go"`** — D-12 lock; **prefix form**, deliberately omits `/v2` so future major-version bumps still match without changing this constant.
- Run logic: short-circuit return if `pass.Pkg.Path()` starts with the allowlisted prefix; otherwise scan every file's imports and `pass.Reportf` on any path with the forbidden prefix.

### `cmd/vet-noduckdb/main.go`
- Three-line standard singlechecker idiom: `func main() { singlechecker.Main(noduckdb.Analyzer) }`.
- No new module dependencies — `golang.org/x/tools v0.43.0` was already in `go.mod` (transitive).

### `internal/lint/noduckdb/analyzer_test.go`
- `TestAnalyzer_RejectsImportFromBadpkg` — `analysistest.Run(t, ..., "badpkg")` confirms the analyzer fires on a non-store package.
- `TestAnalyzer_AllowsImportFromGoodpkg` — `analysistest.Run(t, ..., "github.com/agenthands/helix/internal/semantic/store/goodpkg")` confirms the analyzer is silent for store-prefixed packages.

### Fixtures (analysistest `testdata/src/<pkg-path>/` convention)
- `testdata/src/badpkg/imports.go` — `package badpkg; import _ "github.com/duckdb/duckdb-go/v2" // want \`duckdb-go may only be imported from .*\``. The simple `badpkg` path means `pass.Pkg.Path() == "badpkg"`, which does NOT match the allowed prefix → diagnostic fires (negative path).
- `testdata/src/github.com/agenthands/helix/internal/semantic/store/goodpkg/imports.go` — deep path required so analysistest derives `pass.Pkg.Path() == "github.com/agenthands/helix/internal/semantic/store/goodpkg"`, which DOES match the allowlisted prefix → analyzer must be silent (positive path). No `// want` directive.
- `testdata/src/github.com/duckdb/duckdb-go/v2/duckdb.go` — minimal stub package (`package duckdb`) **added under Rule 3** because `analysistest.Run` performs full type-checking on fixtures and was failing with "could not import github.com/duckdb/duckdb-go/v2 (invalid package name: \"\")". The plan's `<action>` step 2 optimistically claimed "the testdata import does not need to actually resolve at compile time" — that is wrong in current `golang.org/x/tools v0.43.0`. The stub resolves the import for type-checking; the actual analyzer logic only inspects the import-path string, so the stub being a no-op package has no effect on what the analyzer sees.

### `Makefile` diff (lines 17-21 of pre-existing file → lines 17-27 of new file)
**Before** (lines 17-21):
```make
test:
	$(GO) test ./...

vet:
	$(GO) vet ./...
```

**After** (lines 17-27):
```make
test: vet
	$(GO) test ./...

VETTOOL=$(shell go env GOPATH)/bin/vet-noduckdb

vet: $(VETTOOL)
	$(GO) vet ./...
	$(GO) vet -vettool=$(VETTOOL) ./...

$(VETTOOL): cmd/vet-noduckdb/main.go internal/lint/noduckdb/*.go
	$(GO) install ./cmd/vet-noduckdb
```

- **Single `vet:` target** in the Makefile after the change (`grep -c '^vet:' Makefile` == 1) — replacement, not duplication.
- `test: vet` makes the boundary check a hard prerequisite of every test run.
- `$(VETTOOL)` is a real file target (not phony) — Make rebuilds the analyzer whenever `cmd/vet-noduckdb/main.go` or any `internal/lint/noduckdb/*.go` source changes.
- The pre-existing `$(GO) vet ./...` line is preserved as the first command so stdlib analyzers continue to run; the new `$(GO) vet -vettool=$(VETTOOL) ./...` line adds the noduckdb gate as a second pass.

## Commits

| Task | Type     | Commit   | Description                                                       |
| ---- | -------- | -------- | ----------------------------------------------------------------- |
| 1    | test     | 9974f707 | add failing analysistest fixtures for noduckdb analyzer (RED)     |
| 2    | feat     | 450b3ee7 | add noduckdb analyzer + cmd/vet-noduckdb singlechecker binary (GREEN) |
| 3    | build    | 7cd0ffa6 | replace Makefile vet target with build-then-vet sequence; chain test to vet |

## Verification

### `make vet` (clean state, analyzer binary not yet installed)
```
$ rm -f $(go env GOPATH)/bin/vet-noduckdb
$ make vet
go install ./cmd/vet-noduckdb         # implicit via $(VETTOOL) prerequisite
go vet ./...                          # stdlib analyzers — clean
go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./...   # noduckdb gate — clean
$ echo $?
0
```

### `make test` (full chain — vet then test)
```
$ make test
go vet ./...                          # via test→vet dependency
go vet -vettool=.../vet-noduckdb ./...
go test ./...
ok  github.com/agenthands/helix/internal/lint/noduckdb  4.034s   # both analysistest cases GREEN
ok  github.com/agenthands/helix/internal/phasegraph     3.063s   # P01 (wave-1) confirmed green
ok  github.com/agenthands/helix/internal/semantic/store 2.156s   # P02 (wave-1) confirmed green
... (29 packages OK, 0 FAIL, 0 unexpected)
$ echo $?
0
```

### Analyzer self-test (negative path proven via analysistest fixture)
The badpkg fixture imports duckdb-go; the analyzer's `pass.Reportf` matches the `// want` directive: `"duckdb-go may only be imported from github.com/agenthands/helix/internal/semantic/store (got badpkg)"`. The directive uses Go regex syntax `\`duckdb-go may only be imported from .*\`` which matches that error format string.

### Threat model coverage (STRIDE)
- **T-57-04-01 (blank-import bypass)** — mitigated. Analyzer matches by `imp.Path.Value` string, not binding mode; the negative fixture uses `import _ "github.com/duckdb/duckdb-go/v2"` and the analyzer fires correctly.
- **T-57-04-02 (dot-import bypass)** — mitigated by the same path-string check; binding mode is irrelevant.
- **T-57-04-04 (skipping vet from make test)** — mitigated. The `test: vet` prerequisite is committed atomically with the analyzer in this plan; CI changes that drop `make test` would be a separate review.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocker] analysistest type-checks fixture imports — added stub `duckdb-go/v2` package**
- **Found during:** Task 2 verification (`go test ./internal/lint/noduckdb/... -count=1`)
- **Issue:** Plan's `<action>` step 2 stated "the testdata import does not need to actually resolve at compile time because `analysistest` uses its own GOPATH; what matters is the import string the analyzer sees." This is incorrect in `golang.org/x/tools v0.43.0`: `analysistest.Run` performs full type-checking on the fixture, and unresolvable imports produce errors like `cannot find package "github.com/duckdb/duckdb-go/v2"` followed by `analysis skipped due to errors in package` — the analyzer never gets to run, so both tests fail.
- **Fix:** Created `internal/lint/noduckdb/testdata/src/github.com/duckdb/duckdb-go/v2/duckdb.go` as a minimal stub:
  ```go
  // Stub package used only by analysistest fixtures.
  package duckdb
  ```
  The stub has no symbols — the fixtures use blank-import (`import _`), so the import resolves and type-checking succeeds without the stub needing to provide any API surface. The real analyzer logic only inspects the import-path string, so the stub being a no-op package has zero effect on what the analyzer sees.
- **Files added:** `internal/lint/noduckdb/testdata/src/github.com/duckdb/duckdb-go/v2/duckdb.go`
- **Commit:** 450b3ee7 (Task 2 GREEN — included with the analyzer commit since the analyzer cannot be proven green without the stub)
- **Why this isn't STORE-06 leakage:** The stub lives entirely under `internal/lint/noduckdb/testdata/`, which is excluded from `go vet ./...` (testdata is by convention skipped by Go tooling). The real `internal/semantic/store/` package will use the actual `github.com/duckdb/duckdb-go/v2` driver — completely separate from this fixture stub.

### Documentation-only verify-command defect (NOT auto-fixed; cosmetic)
- The plan's Task 1 `<verify><automated>` is `! go build ./internal/lint/noduckdb/... 2>/dev/null`, asserting the build fails (RED gate). In Go, `go build ./...` on a package containing only `_test.go` files exits 0 (test files aren't built by `go build`). The accurate RED-gate command is `go test ./internal/lint/noduckdb/` which exits 1 (`no non-test Go files`).
- **Impact:** None on correctness — the RED→GREEN transition is proven by Task 2's `go test` exit going from 1 to 0. The plan's verify command is a literal-text defect that doesn't affect what the tasks build or test.
- **Not auto-fixed in plan source** because the plan was already authored and the per-task commit is what matters for the build chain. Flagged here for future plan revisions.

## Configuration Snapshot

| Constant            | Value                                                       | Source        |
| ------------------- | ----------------------------------------------------------- | ------------- |
| `allowedPkgPrefix`  | `github.com/agenthands/helix/internal/semantic/store`       | matches `go.mod` module name + SPEC §6 layout |
| `forbiddenImport`   | `github.com/duckdb/duckdb-go`                               | CONTEXT.md D-12 prefix form (omits /v2 for future-major-version compat) |
| Fixture import path | `github.com/duckdb/duckdb-go/v2`                            | CONTEXT.md D-12 verbatim     |

## Self-Check: PASSED

**Files exist:**
- `internal/lint/noduckdb/analyzer.go` — FOUND
- `internal/lint/noduckdb/analyzer_test.go` — FOUND
- `internal/lint/noduckdb/testdata/src/badpkg/imports.go` — FOUND
- `internal/lint/noduckdb/testdata/src/github.com/agenthands/helix/internal/semantic/store/goodpkg/imports.go` — FOUND
- `internal/lint/noduckdb/testdata/src/github.com/duckdb/duckdb-go/v2/duckdb.go` — FOUND
- `cmd/vet-noduckdb/main.go` — FOUND
- `Makefile` (modified) — FOUND

**Commits exist:**
- 9974f707 (test) — FOUND
- 450b3ee7 (feat) — FOUND
- 7cd0ffa6 (build) — FOUND

**Functional checks:**
- `go test ./internal/lint/noduckdb/... -count=1` exits 0 — PASSED
- `go install ./cmd/vet-noduckdb` exits 0 — PASSED
- `go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./...` exits 0 — PASSED
- `make vet && make test` from clean state exits 0 — PASSED
- `grep -c '^vet:' Makefile` == 1 (single target, replacement confirmed) — PASSED
- `grep -q '^test: vet' Makefile` — PASSED

## TDD Gate Compliance

- **RED gate:** commit 9974f707 (`test(57-04): add failing analysistest fixtures for noduckdb analyzer`) — test file references undefined `noduckdb.Analyzer`; `go test` exits 1.
- **GREEN gate:** commit 450b3ee7 (`feat(57-04): add noduckdb analyzer + cmd/vet-noduckdb singlechecker binary`) — analyzer added, both analysistest cases pass; `go test` exits 0.
- **REFACTOR gate:** none required — analyzer is exactly the RESEARCH-supplied template; no clean-up phase needed.
- Sequence verified: 9974f707 (test) precedes 450b3ee7 (feat) precedes 7cd0ffa6 (build). All three commits are in the correct order.
