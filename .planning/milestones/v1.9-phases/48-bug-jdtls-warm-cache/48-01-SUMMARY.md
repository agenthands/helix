---
phase: 48
plan: 01
subsystem: test-infra
tags: [jdtls, cache, test-infra, stdlib]
requires: []
provides:
  - "jdtlscache.ResolveDataDir(fixtureName, fixtureRoot, jdtlsPath) (string, error)"
  - "jdtlscache.hashTree (unexported, white-box tested)"
  - "jdtlscache.hashBinary (unexported, white-box tested)"
affects:
  - test/integration/jdtlscache/
tech-stack:
  added: []
  patterns:
    - "stdlib-only sha256 content hashing (matches internal/memory/index.go pattern)"
    - "os.UserCacheDir() for cross-platform cache root resolution (D-01)"
    - "sorted filepath.WalkDir for deterministic content hash (D-04, Pattern 2)"
key-files:
  created:
    - test/integration/jdtlscache/cache.go
    - test/integration/jdtlscache/cache_test.go
  modified: []
decisions:
  - "Hash prefix length: 12 hex chars (per D-02 / RESEARCH recommendation, Windows-path-safe)"
  - "White-box test package (`package jdtlscache`) so unexported hashTree/hashBinary are unit-testable per behavior list"
  - "Implementation copied verbatim from RESEARCH.md §Resolve the warm -data directory (lines 344-409); no modifications"
  - "No //go:build tag — package + tests run under default `go test ./...` (D-08 prerequisite)"
metrics:
  duration_seconds: 85
  tasks_completed: 1
  files_created: 2
  files_modified: 0
  completed_date: "2026-04-25"
---

# Phase 48 Plan 01: jdtlscache Helper Summary

Pure-stdlib helper package that resolves the warm jdtls `-data` directory under `$UserCacheDir/serena-test/jdtls/`, keyed on (fixture content hash, jdtls binary identity hash). Establishes the single stable API (`ResolveDataDir`) that downstream plans 02 (LS pool injection), 03 (test-infra wiring) and 04 (CI cache wiring) will consume.

## Objective Achieved

- [x] Package `test/integration/jdtlscache` exists with exported `ResolveDataDir(fixtureName, fixtureRoot, jdtlsPath) (string, error)`
- [x] Implementation copied verbatim from RESEARCH.md §"Resolve the warm -data directory"
- [x] All five unit tests pass under default `go test ./...` (no build tag)
- [x] No new module dependencies (`git diff go.mod go.sum` empty)
- [x] `go vet ./test/integration/jdtlscache/...` clean

## Tasks Completed

| Task | Name                                              | Commit   | Files                                                                |
| ---- | ------------------------------------------------- | -------- | -------------------------------------------------------------------- |
| 1-RED   | Failing tests for hash + ResolveDataDir         | 2628d208 | test/integration/jdtlscache/cache_test.go                            |
| 1-GREEN | Helper implementation (copied from RESEARCH.md) | 5b37ce63 | test/integration/jdtlscache/cache.go                                 |

REFACTOR phase: no changes required — implementation was already minimal and idiomatic per RESEARCH guidance.

## Verification

```
$ go vet ./test/integration/jdtlscache/...
(clean — no diagnostics)

$ go test ./test/integration/jdtlscache/... -count=1 -run 'TestFixtureHashStable|TestFixtureHashInvalidation|TestJdtlsHashInvalidation|TestResolveDataDir_CreatesUnderUserCache|TestResolveDataDir_DirectoryExists' -v
=== RUN   TestFixtureHashStable
--- PASS: TestFixtureHashStable (0.00s)
=== RUN   TestFixtureHashInvalidation
--- PASS: TestFixtureHashInvalidation (0.00s)
=== RUN   TestJdtlsHashInvalidation
--- PASS: TestJdtlsHashInvalidation (0.00s)
=== RUN   TestResolveDataDir_CreatesUnderUserCache
--- PASS: TestResolveDataDir_CreatesUnderUserCache (0.00s)
=== RUN   TestResolveDataDir_DirectoryExists
--- PASS: TestResolveDataDir_DirectoryExists (0.00s)
PASS
ok  	github.com/postfix/serena/test/integration/jdtlscache	0.482s
```

### Acceptance Criteria

- [x] `test/integration/jdtlscache/cache.go` exists, package line `package jdtlscache`, no `//go:build` directive
- [x] `grep -c "func ResolveDataDir(" test/integration/jdtlscache/cache.go` == 1
- [x] `grep -c "func hashTree(" test/integration/jdtlscache/cache.go` == 1
- [x] `grep -c "func hashBinary(" test/integration/jdtlscache/cache.go` == 1
- [x] `grep -c "os.UserCacheDir" test/integration/jdtlscache/cache.go` == 2 (≥ 1 required)
- [x] `grep -c "sha256.New" test/integration/jdtlscache/cache.go` == 2 (≥ 1 required)
- [x] All five named test functions present and pass
- [x] `go test ./test/integration/jdtlscache/... -count=1` exits 0 with `PASS`
- [x] `go vet ./test/integration/jdtlscache/...` exits 0 with no diagnostics
- [x] No new module dependencies (`git diff go.mod go.sum` empty)

## Deviations from Plan

None — plan executed exactly as written. Implementation copied verbatim from RESEARCH.md §"Resolve the warm -data directory" lines 344-409 as instructed.

The repo-level `go vet ./...` emits a pre-existing CGO `-Wmacro-redefined` warning in `internal/treesitter/bindings/swift` (TOKEN_COUNT macro). This is **out of scope** — not introduced by this plan, originates from vendored tree-sitter Swift bindings. Logged as a deferred item; no action taken per scope-boundary rule.

## Threat Model Compliance

All five threats in the plan's `<threat_model>` are either `accept` (justified — test-only, repo-local, hash output not reversible) or `mitigate` (T-48-01-04 → `os.MkdirAll(dir, 0o755)` used; no world-write). The implementation satisfies the single `mitigate` disposition exactly as planned.

## TDD Gate Compliance

- RED: commit `2628d208` — `test(48-01): add failing tests for jdtlscache helper` (verified failing with "undefined: hashTree / hashBinary / ResolveDataDir")
- GREEN: commit `5b37ce63` — `feat(48-01): implement jdtlscache helper for warm jdtls -data dir` (all 5 tests pass)
- REFACTOR: not required — implementation was already minimal.

## Downstream Contract

Plans 02, 03, 04 may now `import "github.com/postfix/serena/test/integration/jdtlscache"` and call:

```go
dataDir, err := jdtlscache.ResolveDataDir("java", fixtureRoot, jdtlsPath)
// dataDir is e.g. /Users/<user>/Library/Caches/serena-test/jdtls/java-<fh>-<jh>/
// dataDir is created on disk; pass directly to jdtls -data flag.
```

The directory layout, hash inputs, and prefix length are now frozen — downstream plans should not re-derive them.

## Self-Check: PASSED

- FOUND: test/integration/jdtlscache/cache.go
- FOUND: test/integration/jdtlscache/cache_test.go
- FOUND commit: 2628d208 (test RED)
- FOUND commit: 5b37ce63 (feat GREEN)
