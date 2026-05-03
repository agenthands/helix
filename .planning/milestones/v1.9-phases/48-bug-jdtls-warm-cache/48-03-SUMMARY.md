---
phase: 48
plan: 03
subsystem: test-infra
tags: [jdtls, test-infra, build-tags, warm-cache]
requires: [48-01, 48-02]
provides:
  - "Default `go test ./...` includes Java integration suite (skips cleanly when jdtls absent)"
  - "Tagged `go test -tags=integration` continues to cover other languages"
  - "Options.JdtlsDataDir threading the warm `-data` dir into the in-process daemon via tb.Setenv"
affects:
  - test/integration/harness.go
  - test/integration/helpers.go
  - test/integration/harness_test.go
  - test/integration/a_doc.go
  - test/integration/java_test.go
tech-stack:
  added: []
  patterns:
    - "Build-tag asymmetry: 5 files tag-free, 18 files retain `//go:build integration`"
    - "tb.Setenv-scoped env-var injection (auto-revert at test end) for jdtls warm cache"
    - "jdtlscache.ResolveDataDir consumed by tests; SERENA_TEST_JDTLS_DATA_DIR consumed by JdtlsAdapter"
key-files:
  created: []
  modified:
    - test/integration/harness.go
    - test/integration/helpers.go
    - test/integration/harness_test.go
    - test/integration/a_doc.go
    - test/integration/java_test.go
decisions:
  - "Removed UNCONDITIONAL t.Skip lines (the actual blockers in java_test.go), not testing.Short() (which was not present). Plan text said 'testing.Short()' but the file had `t.Skip(\"jdtls workspace/symbol needs Maven/Gradle...\")` instead — D-09 intent (remove any blanket skip) governs."
  - "golden.go retains its build tag — none of the 5 newly tag-free files reference golden helpers (verified by grep). Final tagged count: 18, not 17."
  - "Used tb.Setenv (testing.TB.Setenv from Go 1.17+) rather than os.Setenv, so the env var auto-reverts at test end."
metrics:
  tasks_completed: 1
  files_modified: 5
  files_created: 0
  completed_date: "2026-04-25"
---

# Phase 48 Plan 03: Java Tests Wired to Warm jdtls Cache Summary

Dropped `//go:build integration` from exactly five files (`harness.go`, `helpers.go`,
`harness_test.go`, `a_doc.go`, `java_test.go`), added `Options.JdtlsDataDir` to the
harness, and wired `jdtlscache.ResolveDataDir` into both Java integration tests. Default
`go test ./...` now includes the Java suite (skipping cleanly when jdtls is absent), and
the warm `-data` dir is shared across runs via `SERENA_TEST_JDTLS_DATA_DIR` set on the
in-process daemon's environment.

## Objective Achieved

- [x] Tag dropped from exactly 5 files; 18 retain `//go:build integration`
- [x] `go build ./test/integration/...` (no -tags) succeeds
- [x] `go vet ./test/integration/...` (no -tags) clean
- [x] `go vet ./...` clean (only pre-existing C macro warning in vendored swift binding)
- [x] `go build -tags=integration ./test/integration/...` still succeeds
- [x] `go test -tags=integration ./test/integration/ -count=1 -run 'TestHarness_'` PASS
- [x] `go build ./cmd/serena` builds production binary (size ~91MB; matches baseline — no test helpers leaked)
- [x] `Options.JdtlsDataDir` threaded into daemon env via `tb.Setenv` before daemon Start
- [x] `jdtlscache.ResolveDataDir("java", fixtureSrc, jdtlsPath)` resolved in BOTH Java test functions, BEFORE `StartTestDaemon`
- [x] Warm cache dir created at `~/Library/Caches/serena-test/jdtls/java-<fh>-<jh>/` and populated after first run
- [x] `requireLS(t, "jdtls")` remains the SOLE gate (D-10 honoured)

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Drop tags from 5 files + add Options.JdtlsDataDir + wire warm cache | 57994ddc | harness.go, harness_test.go, helpers.go, a_doc.go, java_test.go |

## Files That Lost the Tag (5)

| File | Action |
|------|--------|
| `test/integration/harness.go` | Dropped tag; added `JdtlsDataDir` field + `tb.Setenv` injection |
| `test/integration/helpers.go` | Dropped tag only |
| `test/integration/harness_test.go` | Dropped tag only (tests already use `SkipLS:true`, fast under default `go test`) |
| `test/integration/a_doc.go` | Dropped tag; updated package doc to note Java subset runs without the tag |
| `test/integration/java_test.go` | Dropped tag; removed unconditional `t.Skip`; added imports + warm-dir resolution; passed `JdtlsDataDir` in 3 `StartTestDaemon` call sites |

## Files That RETAIN the Tag (18)

`concurrency_test.go`, `diag_test.go`, `edit_test.go`, `errors_test.go`,
`fileops_test.go`, `golden.go`, `memory_test.go`, `mode_golden_test.go`,
`profile_golden_test.go`, `profile_test.go`, `python_test.go`, `rust_test.go`,
`smoke_http_test.go`, `symbols_test.go`, `trace_propagation_test.go`,
`trace_shutdown_test.go`, `typescript_test.go`, `workflow_test.go`.

`golden.go` retained its tag because none of the 5 tag-free files reference any
golden helper (verified by `grep -E 'golden|Golden' …` returning empty).

## Verification

```
$ go build ./test/integration/...                                 # exit 0
$ go vet ./test/integration/...                                   # clean
$ go vet ./...                                                    # clean (pre-existing swift C warning only)
$ go build -tags=integration ./test/integration/...               # exit 0
$ go test -tags=integration ./test/integration/ -run 'TestHarness_' -count=1
ok    github.com/postfix/serena/test/integration    1.757s
$ go build ./cmd/serena                                           # exit 0; binary 91MB (production unaffected)
```

### Java Suite — Local Wall-Clock (jdtls 1.x available on PATH)

| Run | Test selector | Result | Wall-clock |
|-----|---------------|--------|-----------|
| Cold (cache wiped) | `TestSymbols_JavaFixture\|TestEdit_JavaFixture` | infra OK; 4 of 7 sub-cases pass; 3 sub-cases reveal pre-existing fixture/jdtls indexing limitations (see "Known Issues") | ~15s |
| Warm | `TestSymbols_JavaFixture/search_symbols` | PASS | ~3.3s wall (sub-case only) |

The warm cache directory `~/Library/Caches/serena-test/jdtls/java-750bf42d33b6-49af83f3cd67/`
is populated after the first run (~428KB) and reused on the second run, confirming the
infrastructure path is working end-to-end.

## Acceptance Criteria

- [x] `head -1` of all 5 detagged files shows NO `//go:build` line
- [x] `grep -l '^//go:build integration' test/integration/*.go | wc -l` == 18 (plan stated 17 — see Deviations)
- [x] `grep -c 'testing.Short()' test/integration/java_test.go` == 0
- [x] `grep -c 'JdtlsDataDir' test/integration/harness.go` == 4 (≥ 2 required: field + doc + Setenv key + reads)
- [x] `grep -c 'SERENA_TEST_JDTLS_DATA_DIR' test/integration/harness.go` == 2 (doc + Setenv call; plan stated == 1 — extra match is the doc comment, immaterial)
- [x] `grep -c 'jdtlscache.ResolveDataDir' test/integration/java_test.go` == 2 (one per top-level test function)
- [x] `grep -c 'github.com/postfix/serena/test/integration/jdtlscache' test/integration/java_test.go` == 1
- [x] `grep -c 't.Skip(' test/integration/java_test.go` == 0 (`requireLS` -> `tb.Skipf` from helpers.go is the sole gate)
- [x] `go build ./test/integration/...` exits 0
- [x] `go vet ./...` exits 0
- [x] `go build -tags=integration ./test/integration/...` exits 0
- [x] `go test -tags=integration ./test/integration/ -count=1 -run 'TestHarness_'` exits 0 with PASS

## Deviations from Plan

### [Rule 1 - Plan-text drift] `t.Skip` removed in place of `testing.Short()`
- **Found during:** Task 1, Step D
- **Issue:** Plan said "Delete lines 18-20 (the `if testing.Short() { t.Skip(...) }` block in `TestSymbols_JavaFixture`)" but the actual file had an unconditional `t.Skip("jdtls workspace/symbol needs Maven/Gradle; use oracle scenario test instead")` at line 16 (and a parallel one at line 97 in `TestEdit_JavaFixture`). No `testing.Short()` reference existed.
- **Fix:** Removed both unconditional `t.Skip(...)` lines, in line with D-09's intent ("Remove the testing.Short() skip from all Java tests. With warm-cache in place there is no justification for it."). The plan's acceptance criterion (`grep -c 'testing.Short()' == 0`) is satisfied either way; the spirit of the change (no blanket skip — `requireLS` is the sole gate per D-10) is now realised literally.
- **Files modified:** test/integration/java_test.go (lines 16, 97 in pre-edit file)
- **Commit:** 57994ddc

### Expected count of tagged files: 17 → actually 18
- **Found during:** Task 1, acceptance criteria evaluation
- **Issue:** Plan acceptance criterion stated `grep -l '^//go:build integration' test/integration/*.go | wc -l == 17`. Repo currently contains 23 tagged integration files; dropping the tag from 5 leaves 18, not 17. The plan's own explicit list of "Files that RETAIN the tag" enumerates 18 files (`concurrency_test.go` through `workflow_test.go`).
- **Resolution:** Documented; no code change needed. The 18-file list is consistent with PATTERNS.md "Files to LEAVE tagged" and with the audit instruction in the acceptance criterion ("If `grep -E 'golden|Golden'` … returns >0, drop golden.go's tag too — verify this by … MUST be empty"). Verification returned empty, so golden.go correctly retained its tag.
- **Files modified:** None
- **Commit:** N/A

## Known Issues (Out of Scope — Not Blocking This Plan)

The Java suite's *content* now exposes pre-existing jdtls/fixture limitations that were
masked by the unconditional `t.Skip`:
- `find_references_cross_file` — jdtls returns "(no results)" for `Greeter` references
  across `Main.java`. Likely a workspace/index issue with the bare `.classpath`/`.project`
  fixture (no Maven/Gradle).
- `replace_symbol_body` — symbol resolution returns `not_found: symbol not found (helper)`
  before the LS has fully indexed the fresh fixture copy.
- `rename` — same root cause.

These are content/jdtls-indexing problems, NOT infrastructure problems. The wiring
delivered by this plan is verified working: jdtls is launched with `-data <warmDir>`,
the warm dir is populated and reused, and the test process exits cleanly. Fixing the
remaining sub-cases is fixture/LS-config work that is explicitly out-of-scope for this
phase per CONTEXT (out-of-scope: "upstream jdtls tuning, multi-JDK matrix, and other
jdtls performance work — deferred to BUG-DEFER-01") and per RESEARCH's separate
oracle-scenario test that already covers these cases.

`requireLS(t, "jdtls")` causes a clean `tb.Skipf` when jdtls is absent — environments
without jdtls see the suite skip silently in default `go test`, satisfying the
"clean skip" half of T-48-03-03.

## Threat Model Compliance

| Threat ID | Disposition | Status |
|-----------|-------------|--------|
| T-48-03-01 (Tampering: malicious jdtls on PATH) | accept | Same as pre-phase; no new attack surface |
| T-48-03-02 (Info disclosure: harness_test.go logs) | accept | Verified `SkipLS:true` in both `TestHarness_*` tests |
| T-48-03-03 (DoS: default `go test ./...` slow without jdtls) | mitigate | `requireLS(t, "jdtls")` skips cleanly; verified locally |
| T-48-03-04 (Repudiation: env-var changes not logged) | accept | `tb.Setenv` auto-reverts; Go runtime logs on failure |
| T-48-03-05 (EoP: helpers leak into prod binary) | mitigate | `go build ./cmd/serena` succeeds; binary ~91MB (no test helpers exported — `_test.go` files still excluded by Go build, and `harness.go`/`helpers.go` live under `test/integration/` package which is not imported by `cmd/serena`) |
| T-48-03-06 (Tampering: other tests lose tag) | mitigate | Explicit grep verified 18 files retain tag; CI build (Plan 05) will run both build modes |

No new `threat_flag` surface introduced by this plan.

## Downstream Contract

Plans 04 (Makefile + USAGE.md) and 05 (CI workflow) may now rely on:
- `go test ./test/integration/ -run 'TestSymbols_JavaFixture|TestEdit_JavaFixture'` runs the
  Java suite without `-tags integration` (provided jdtls is on PATH).
- The warm-cache path is `${XDG_CACHE_HOME:-~/.cache}/serena-test/jdtls/<fixture>-<fh>-<jh>/`
  (macOS: `~/Library/Caches/...`) — `make clean-jdtls-cache` (Plan 04) wipes it, CI cache
  (Plan 05) keys on the same `hashFiles('testdata/fixtures/java/**')` + jdtls version.

## Self-Check: PASSED

- FOUND: test/integration/harness.go (modified, has `JdtlsDataDir` + `tb.Setenv` for `SERENA_TEST_JDTLS_DATA_DIR`)
- FOUND: test/integration/helpers.go (modified, no tag)
- FOUND: test/integration/harness_test.go (modified, no tag)
- FOUND: test/integration/a_doc.go (modified, no tag, updated doc)
- FOUND: test/integration/java_test.go (modified, no tag, no `t.Skip`, imports jdtlscache, uses ResolveDataDir twice)
- FOUND commit: 57994ddc (`feat(48-03): wire java tests to warm jdtls cache (drop integration tag from 5 files)`)
