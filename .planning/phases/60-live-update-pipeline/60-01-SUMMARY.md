---
phase: 60
plan: 01
subsystem: lint
tags: [lint, analyzer, kernel-semantic-boundary, live-07]
wave: 0
status: complete
requires:
  - "internal/lint/noduckdb/* (template / shape reference)"
  - "golang.org/x/tools/go/analysis (already in go.mod via noduckdb)"
provides:
  - "go/analysis Analyzer enforcing LIVE-07 invariant #1"
  - "cmd/vet-nokernel2semantic singlechecker"
  - "make vet runs the new analyzer alongside vet-noduckdb"
affects:
  - "Makefile vet target"
tech_stack:
  added: []
  patterns: [singlechecker analyzer, analysistest fixture pair, build-tag-gated integration test]
key_files:
  created:
    - internal/lint/nokernel2semantic/analyzer.go
    - internal/lint/nokernel2semantic/analyzer_test.go
    - internal/lint/nokernel2semantic/realtree_integration_test.go
    - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/badpkg/imports.go
    - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/goodpkg/imports.go
    - internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/semantic/foo/foo.go
    - cmd/vet-nokernel2semantic/main.go
  modified:
    - Makefile
decisions:
  - "Mirror noduckdb structure exactly (analyzer.go, singlechecker, analysistest fixture pair) — no novel scaffolding."
  - "Bad fixture lives at testdata/src/github.com/agenthands/helix/internal/kernel/badpkg/ so pass.Pkg.Path() begins with the kernel prefix and the analyzer's early-return guard is genuinely exercised."
  - "Real-tree regression is gated behind the `integration` build tag to keep the unit-test path fast (~0.5s vs ~2.3s)."
  - "Did NOT modify .github/workflows/go-test.yml — plan explicitly forbade adding workflow files in 60-01. CI today still runs plain `go vet ./...` (no -vettool); CI wiring is a follow-up step explicitly noted in the analyzer's package doc."
metrics:
  duration_minutes: 7
  tasks_completed: 2
  completed_date: 2026-05-05
---

# Phase 60 Plan 01: nokernel2semantic Analyzer Summary

Wave 0 ships the `nokernel2semantic` go/analysis analyzer + `vet-nokernel2semantic` singlechecker + Makefile vet wiring, mechanically enforcing LIVE-07 invariant #1 ("`internal/kernel/` does not import `internal/semantic/...`") before any import-boundary-touching code lands in plans 03 and 04.

## Wave 0 Status

- **Wave:** 0 (gates Wave 1+)
- **Status:** Complete (LIVE-07 invariant #1 is now machine-checked by `make vet` / `make test`)
- **Real-tree run:** exit 0 — invariant holds today

## Analyzer Behavior

```text
checkedPkgPrefix     = "github.com/agenthands/helix/internal/kernel"
forbiddenImportPrefix = "github.com/agenthands/helix/internal/semantic"
```

For each Go package the analyzer is invoked on:
1. If `pass.Pkg.Path()` does NOT start with `checkedPkgPrefix` → return immediately (no-op).
2. Else, scan every `import` declaration in every file of the package; if the import path starts with `forbiddenImportPrefix`, emit:
   ```
   internal/kernel/* must not import internal/semantic/* (got import "<path>" in <pkg>)
   ```

The rule is **one-directional**: `semantic→kernel` is allowed (and is the architecture — semantic depends on kernel for typed IDs, fileops, etc.). The analyzer never inspects packages outside the kernel prefix, so semantic packages can import kernel freely without triggering it.

## Fixture Layout

Mirrors `internal/lint/noduckdb/testdata/` exactly:

```
internal/lint/nokernel2semantic/testdata/
└── src/
    └── github.com/agenthands/helix/internal/
        ├── kernel/
        │   ├── badpkg/imports.go    # imports semantic/foo + `// want` directive — analyzer MUST flag
        │   └── goodpkg/imports.go   # imports only fmt — analyzer MUST stay silent
        └── semantic/
            └── foo/foo.go           # empty stub package, imported by badpkg
```

Fixture-driven tests (`analyzer_test.go`):
- `TestAnalyzer_RejectsKernelImportingSemantic` — runs analyzer on badpkg, expects the `// want` directive to fire.
- `TestAnalyzer_AllowsKernelWithoutSemanticImport` — runs analyzer on goodpkg, expects zero diagnostics.

Real-tree regression (`realtree_integration_test.go`, build tag `integration`):
- `TestAnalyzer_RealTreeIsClean` — `go install ./cmd/vet-nokernel2semantic`, then shell out to `go vet -vettool=$(GOPATH)/bin/vet-nokernel2semantic ./internal/kernel/...` and assert exit-zero. Run with `go test -tags=integration ./internal/lint/nokernel2semantic/...`.

## Makefile Diff

```diff
 VETTOOL=$(shell go env GOPATH)/bin/vet-noduckdb
+VETTOOL_NOKERNEL2SEMANTIC=$(shell go env GOPATH)/bin/vet-nokernel2semantic

-vet: $(VETTOOL)
+vet: $(VETTOOL) $(VETTOOL_NOKERNEL2SEMANTIC)
 	$(GO) vet ./...
 	$(GO) vet -vettool=$(VETTOOL) ./...
+	$(GO) vet -vettool=$(VETTOOL_NOKERNEL2SEMANTIC) ./...

 $(VETTOOL): cmd/vet-noduckdb/main.go internal/lint/noduckdb/*.go
 	$(GO) install ./cmd/vet-noduckdb

+$(VETTOOL_NOKERNEL2SEMANTIC): cmd/vet-nokernel2semantic/main.go internal/lint/nokernel2semantic/*.go
+	$(GO) install ./cmd/vet-nokernel2semantic
+
 fmt:
 	gofmt -w .
```

`make vet` (and transitively `make test`, since `test: vet`) now installs and runs both `vet-noduckdb` and `vet-nokernel2semantic`.

## LIVE-07 Invariant #1 Trace

| Source | Detail |
|---|---|
| Requirement ID | LIVE-07 (60-CONTEXT.md acceptance criterion #1) |
| Statement | "`internal/kernel/` does not import `internal/semantic/...`" |
| Enforcement | `internal/lint/nokernel2semantic/analyzer.go` (Analyzer.Run, prefix string-match) |
| CI/local entry point | `make vet` → `cmd/vet-nokernel2semantic` singlechecker → `go vet -vettool=...` |
| Regression net (live tree) | `realtree_integration_test.go` (build tag `integration`) |
| Fixture suite | `analyzer_test.go` (badpkg flagged, goodpkg silent) |
| Today's status on real tree | exit 0 — invariant holds |

## TDD Gate Compliance

- **RED:** `15eb5c73 test(60-01): add failing analysistest fixtures for nokernel2semantic` — build failed ("no non-test Go files") because analyzer.go did not exist yet.
- **GREEN:** `654ed5c9 feat(60-01): implement nokernel2semantic analyzer + singlechecker + vet wiring` — both fixture tests pass; real-tree run exit-zero.
- **REFACTOR:** Not needed; the analyzer is a 25-line near-verbatim port of the noduckdb template.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] analysistest comment-scan parser tripped on literal `// want` inside a backtick comment**
- **Found during:** Task 1 GREEN — `TestAnalyzer_AllowsKernelWithoutSemanticImport` failed with `literal not terminated` because the goodpkg fixture's package doc contained the literal text "`// want`" (in backticks), which analysistest scans regardless of context.
- **Fix:** Reworded the comment to drop the literal `// want` token; backticks left out where they could be reintroduced into directive parsing.
- **Files modified:** `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/goodpkg/imports.go`
- **Commit:** `654ed5c9` (rolled into the GREEN commit since it was the same edit cycle)

### Architectural notes (not deviations — explicitly per plan)

The plan explicitly forbade adding a new workflow file in 60-01. CI today (`.github/workflows/go-test.yml` line 94) runs `go vet ./...` without `-vettool=`, so the singlechecker fires on local `make vet`/`make test` only. This is documented as a CI-wiring follow-up in the analyzer's package doc; the regression is caught by the integration test if anyone runs `make test` (which depends on `vet`).

## Verification Log

| Check | Command | Result |
|---|---|---|
| Unit tests (fixtures) | `go test ./internal/lint/nokernel2semantic/... -count=1` | PASS (~0.55s) |
| Integration test (real tree) | `go test -tags=integration ./internal/lint/nokernel2semantic/... -count=1 -run TestAnalyzer_RealTreeIsClean` | PASS (~2.3s) |
| Singlechecker against real tree | `go install ./cmd/vet-nokernel2semantic && go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./...` | exit 0 |
| Makefile vet token count | `grep -c 'vet-nokernel2semantic' Makefile` | 3 (variable, dep, invocation; install target is on a separate token line) |
| Makefile token line count | `grep -E 'nokernel2semantic' Makefile \| wc -l` | 5 |
| analyzer.go LIVE-07 cite count | `grep -c 'LIVE-07' internal/lint/nokernel2semantic/analyzer.go` | 4 |
| `go vet ./internal/lint/nokernel2semantic/... ./cmd/vet-nokernel2semantic/...` | — | exit 0 |
| `gofmt -l internal/lint/nokernel2semantic/ cmd/vet-nokernel2semantic/` | — | clean |

## Commits

| Phase | Hash | Message |
|---|---|---|
| RED   | `15eb5c73` | test(60-01): add failing analysistest fixtures for nokernel2semantic |
| GREEN | `654ed5c9` | feat(60-01): implement nokernel2semantic analyzer + singlechecker + vet wiring |
| Task 2 | `f12e042f` | test(60-01): add real-tree integration regression + CI wiring note |

## Self-Check: PASSED

- FOUND: `internal/lint/nokernel2semantic/analyzer.go`
- FOUND: `internal/lint/nokernel2semantic/analyzer_test.go`
- FOUND: `internal/lint/nokernel2semantic/realtree_integration_test.go`
- FOUND: `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/badpkg/imports.go`
- FOUND: `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/kernel/goodpkg/imports.go`
- FOUND: `internal/lint/nokernel2semantic/testdata/src/github.com/agenthands/helix/internal/semantic/foo/foo.go`
- FOUND: `cmd/vet-nokernel2semantic/main.go`
- Makefile modified (vet target installs and runs vet-nokernel2semantic)
- Commit `15eb5c73`: present in `git log`
- Commit `654ed5c9`: present in `git log`
- Commit `f12e042f`: present in `git log`
