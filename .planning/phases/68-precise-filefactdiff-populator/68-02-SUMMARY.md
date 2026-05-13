---
phase: 68-precise-filefactdiff-populator
plan: 02
subsystem: semantic-extract
tags: [extract, provider, golang, typescript, python, interface-widening]
requires: []
provides: [ExtractFile method on ExtractionPipeline]
affects: [DIFF-01 enabler for plan 68-04 live handler wiring]
tech_added: []
patterns: [partialFile-on-failure, file-read+delegate]
key_files:
  created:
    - internal/semantic/extract/golang/provider_extract_file_test.go
    - internal/semantic/extract/typescript/provider_extract_file_test.go
  modified:
    - internal/semantic/extract/provider.go
    - internal/semantic/extract/golang/provider.go
    - internal/semantic/extract/typescript/provider.go
    - internal/semantic/extract/python/provider.go
    - internal/semantic/extract/registry_test.go
decisions:
  - "ExtractFile landed on ExtractionPipeline (the existing per-file extraction surface) — Provider composes it transitively. No new wider interface introduced."
  - "Python provider was also widened with an ExtractFile shim (not only Go and TS), because its concrete *pyextract.Provider is exercised via the extract.Provider interface in provider_polymorphic_test.go and would otherwise break the build."
  - "fakeProvider in registry_test.go gets a minimal (nil, nil) ExtractFile stub — registry construction tests do not invoke extraction."
metrics:
  tasks_completed: 3
  files_created: 2
  files_modified: 5
  commits: 2
---

# Phase 68 Plan 02: Per-language ExtractFile shims Summary

Widen `extract.ExtractionPipeline` with `ExtractFile(ctx, repoID, path)` and ship Go, TypeScript, and Python implementations that read the file from disk and delegate to the existing `Extract` method, with I/O failures surfacing as partial-status `ExtractedFile` per the established `partialFile` convention.

## What landed

### Interface widening — internal/semantic/extract/provider.go

Final signature added to `ExtractionPipeline`:

```go
ExtractFile(ctx context.Context, repoID, path string) (*ExtractedFile, error)
```

The `repoID` argument is part of the stable Phase 68 contract for the live handler call site (so plan 68-04 can pass through a stable repo identifier without further interface churn), but is unused by all three current providers — owner / package / module context is derived from the file path. The argument is documented as `_repoID` in each implementation.

### Per-language implementations

Each provider implements a 7-line shim:

```go
func (p *Provider) ExtractFile(ctx context.Context, _repoID, path string) (*extract.ExtractedFile, error) {
    source, err := os.ReadFile(path)
    if err != nil {
        return partialFile(
            extract.SourceFile{Path: path, Language: "<lang>"},
            extract.PartialReasonPermissionDenied, err), nil
    }
    return p.Extract(ctx, source, extract.SourceFile{Path: path, Language: "<lang>"})
}
```

- `internal/semantic/extract/golang/provider.go` — Language: `"go"`
- `internal/semantic/extract/typescript/provider.go` — Language: `"typescript"`
- `internal/semantic/extract/python/provider.go` — Language: `"python"`

`os` was added to each provider's import list. No LSP / fsnotify / network surface introduced (Pitfall 5 honored).

### Provider sweep

The plan called out a sweep with `grep -lR 'func (.*) Extract(ctx context.Context, source \[\]byte,' internal/semantic/extract/`. Concrete providers found: `golang`, `typescript`, `python`. The Python provider was widened with a real `ExtractFile` shim (not a minimal `ExtractionStatusUnsupported` stub) — `python/Provider` is part of the live `extract.Provider` interface satisfaction in `provider_polymorphic_test.go:29-32` and a missing method would fail to compile.

The `fakeProvider` in `internal/semantic/extract/registry_test.go` is a test-only stub for registry-construction tests; it received a minimal `(nil, nil)` `ExtractFile` so the test file still compiles. Comments on `fakeProvider` already note the analogous treatment from Phase 65 D-06.

### Tests

**Go** (`internal/semantic/extract/golang/provider_extract_file_test.go`):
- `TestExtractFile_HappyPath_Go` — writes `package x; func Hello() string {...}` to a `t.TempDir()`, asserts Ready status and `Hello` symbol present.
- `TestExtractFile_FileNotFound_Go` — points at a nonexistent path, asserts partial status with `PartialReasonPermissionDenied` and non-empty `ErrorMessage`.
- `TestExtractFile_RepoIDIgnoredByGo` — runs with `""` and `"anything"` repoID, asserts identical ExtractionStatus and symbol count.

**TypeScript** (`internal/semantic/extract/typescript/provider_extract_file_test.go`):
- `TestExtractFile_HappyPath_TS` — `export function hello(): string { return "v1"; }`.
- `TestExtractFile_FileNotFound_TS` — partial path, expects partial + PermissionDenied.

All tests `t.Parallel()`; passed under `-race`.

## Verification

- `go test -race -run 'TestExtractFile' ./internal/semantic/extract/golang/... ./internal/semantic/extract/typescript/... -count=1` — PASS
- `go test -race ./internal/semantic/extract/... -count=1` — PASS for `extract`, `golang`, `python`, `typescript`
- `go vet ./...` — clean
- `go build ./...` — clean (one pre-existing tree-sitter swift macro-redefinition warning, unrelated)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Python provider also required ExtractFile shim**
- **Found during:** Task 2 GREEN (interface widening)
- **Issue:** The plan named only Go and TS as in-scope providers, but `internal/semantic/extract/python/provider.go` also implements the `extract.Provider` interface and is explicitly asserted in `provider_polymorphic_test.go:32` (`_ extract.Provider = (*pyextract.Provider)(nil)`). Widening the interface without adding an ExtractFile method to Python would break the build.
- **Fix:** Added an identical 7-line file-read+delegate shim to Python provider (Language: `"python"`).
- **Files modified:** `internal/semantic/extract/python/provider.go`
- **Commit:** `3b26645c`

**2. [Rule 3 — Blocking] fakeProvider in registry_test.go**
- **Found during:** Task 2 GREEN
- **Issue:** `registry_test.go::fakeProvider` is asserted against the widened interface in `NewExtractorRegistry`; missing `ExtractFile` would break the test build.
- **Fix:** Added `(nil, nil)` stub matching the existing `Extract` stub pattern (comment already documents the Phase 65 D-06 precedent).
- **Files modified:** `internal/semantic/extract/registry_test.go`
- **Commit:** `3b26645c`

## Commits

| Hash       | Type | Description                                                          |
|------------|------|----------------------------------------------------------------------|
| `c91a9d92` | test | add failing ExtractFile tests for Go and TS providers (RED)          |
| `3b26645c` | feat | widen ExtractionPipeline with ExtractFile + Go/TS/Py shims (GREEN)   |

## TDD Gate Compliance

- RED gate: `c91a9d92` (test commit, compile failure confirmed: `p.ExtractFile undefined`)
- GREEN gate: `3b26645c` (feat commit, all new tests pass `-race`)
- REFACTOR gate: not required (implementations are 7-line shims; no cleanup pass needed)

## Self-Check: PASSED

Files exist:
- FOUND: internal/semantic/extract/golang/provider_extract_file_test.go
- FOUND: internal/semantic/extract/typescript/provider_extract_file_test.go
- FOUND (modified): internal/semantic/extract/provider.go (interface widened)
- FOUND (modified): internal/semantic/extract/golang/provider.go (ExtractFile added)
- FOUND (modified): internal/semantic/extract/typescript/provider.go (ExtractFile added)
- FOUND (modified): internal/semantic/extract/python/provider.go (ExtractFile added)
- FOUND (modified): internal/semantic/extract/registry_test.go (fakeProvider extended)

Commits exist:
- FOUND: c91a9d92 (test RED)
- FOUND: 3b26645c (feat GREEN)

Acceptance criteria met:
- `grep -c 'ExtractFile(ctx context.Context, repoID, path string)' internal/semantic/extract/provider.go` ≥ 1
- `grep -c 'func (p \*Provider) ExtractFile' internal/semantic/extract/{golang,typescript,python}/provider.go` == 1 each
- `go test -race -run 'TestExtractFile' ...` exits 0
- `go build ./...` exits 0
- No `lspclient` / `fsnotify` references introduced
