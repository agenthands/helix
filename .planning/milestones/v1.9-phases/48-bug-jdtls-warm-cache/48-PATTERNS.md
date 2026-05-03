# Phase 48: bug-jdtls-warm-cache - Pattern Map

**Mapped:** 2026-04-24
**Files analyzed:** 10 (new + modified)
**Analogs found:** 10 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `test/integration/jdtlscache/cache.go` (NEW) | utility (test helper, pure stdlib) | file-I/O + transform (hash) | `test/integration/harness.go` `PrepareFixture` (SHA-absent) + `internal/memory/index.go` (sha256 use) | role-match |
| `test/integration/jdtlscache/cache_test.go` (NEW) | test (unit, tag-free) | request-response | `internal/kernel/lspool/quirks_test.go` | role-match |
| `internal/kernel/lspool/quirks.go` (MODIFIED) | adapter/quirk (ArgsModifier) | request-response (process launch) | self — same file, lines 303-337 `JdtlsAdapter` | exact |
| `test/integration/harness.go` (MODIFIED: drop tag + add `Options.JdtlsDataDir` + threading) | test harness | request-response | self (existing `Options` struct) | exact |
| `test/integration/java_test.go` (MODIFIED: drop tag + `Short()` skip + resolve warm dir) | test (integration) | request-response | self + `test/integration/python_test.go` | exact |
| `test/integration/helpers.go` (MODIFIED: drop tag only) | test helper | request-response | self | exact |
| `test/integration/harness_test.go` (MODIFIED: drop tag, verify fast) | test (unit) | request-response | self | exact |
| `test/integration/a_doc.go` (MODIFIED: drop tag, amend doc) | package doc | — | self | exact |
| `Makefile` (MODIFIED: add `clean-jdtls-cache`, `bench-jdtls-warm`) | build orchestration | batch | self (`test`, `vet`, `docs` targets) | exact |
| `.github/workflows/go-test.yml` (NEW) | CI pipeline | event-driven | `.github/workflows/bench.yml` + `.github/workflows/pytest.yml` (cache blocks) | role-match |

## Pattern Assignments

### `test/integration/jdtlscache/cache.go` (utility, file-I/O + transform)

**Analog:** `test/integration/harness.go` (for `filepath.WalkDir` pattern + `projectRoot` via `runtime.Caller`); `internal/memory/index.go` for `crypto/sha256` usage.

**No build tag.** Pure stdlib. Package lives under `test/integration/jdtlscache/` and must compile under default `go test ./...`.

**Walk + read pattern** — copy from `harness.go:258-287` `PrepareFixture`:
```go
err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
    if err != nil {
        return err
    }
    rel, err := filepath.Rel(srcDir, path)
    if err != nil {
        return err
    }
    // ... read os.ReadFile(path)
})
```
Sort the collected paths explicitly after walk (Pitfall 6 — do not rely on Walk order for cross-OS determinism).

**Core API surface (required by plumbing):**
```go
// ResolveDataDir returns an absolute path under os.UserCacheDir() keyed on
// fixture content hash + jdtls binary identity hash. Creates the dir.
func ResolveDataDir(fixtureName, fixtureRoot, jdtlsPath string) (string, error)
```
Reference implementation: RESEARCH.md §"Code Examples → Resolve the warm -data directory" (lines 344-409) — copy verbatim, no modifications needed.

**Error wrapping convention** — copy from `internal/langregistry/installer.go:64`:
```go
return "", nil, fmt.Errorf("language server %q not found for %q; install with: %s", ...)
```
Use `fmt.Errorf("fixture hash: %w", err)` / `fmt.Errorf("jdtls hash: %w", err)` / `fmt.Errorf("user cache dir: %w", err)`.

---

### `test/integration/jdtlscache/cache_test.go` (test, tag-free unit)

**Analog:** `internal/kernel/lspool/quirks_test.go` lines 55-93.

**No build tag.** Must run under default `go test ./...`. Use stdlib `testing` + existing `github.com/stretchr/testify/assert` (already a transitive dep).

**Test name convention** (match existing `TestXxxAdapter_Yyy` style from quirks_test.go):
- `TestFixtureHashStable` — same tree hashes identically on repeat
- `TestFixtureHashInvalidation` — edit a file → hash changes
- `TestJdtlsHashInvalidation` — binary content change → hash changes
- `TestResolveDataDir_CreatesUnderUserCache` — path prefix assertion

**Assertion style** (quirks_test.go:86):
```go
assert.Equal(t, map[string]any{"foo": "bar"}, opts)
```
Use `assert.Equal` for hash strings, `assert.NotEqual` for invalidation cases, `require.NoError` for setup.

**Temp dir pattern** (already idiomatic in repo): `tb.TempDir()` for fixture roots; inject an `os.Setenv("XDG_CACHE_HOME", t.TempDir())` to avoid polluting real user cache during unit tests.

---

### `internal/kernel/lspool/quirks.go` (adapter, request-response)

**Analog:** self — modify the existing `JdtlsAdapter` (lines 303-337).

**Current code** (lines 303-337):
```go
type JdtlsAdapter struct {
    Entry langregistry.LSEntry
}

// ExtraArgs injects -data <dir> so jdtls has a workspace-specific data directory.
func (j *JdtlsAdapter) ExtraArgs(workDir string, args []string) []string {
    dataDir := filepath.Join(workDir, ".jdtls-data")
    _ = os.MkdirAll(dataDir, 0o755)
    return append([]string{"-data", dataDir}, args...)
}
```

**Extension pattern** — add a field, branch on empty:
```go
type JdtlsAdapter struct {
    Entry           langregistry.LSEntry
    OverrideDataDir string // if non-empty, used verbatim as jdtls -data (test-only injection)
}

func (j *JdtlsAdapter) ExtraArgs(workDir string, args []string) []string {
    dataDir := j.OverrideDataDir
    if dataDir == "" {
        dataDir = filepath.Join(workDir, ".jdtls-data")
    }
    _ = os.MkdirAll(dataDir, 0o755)
    return append([]string{"-data", dataDir}, args...)
}
```

**Plumbing:** `GetQuirkAdapter` (called at `pool.go:263`) constructs `JdtlsAdapter` with `Entry` only today. Planner must decide the injection surface — either:
- (a) a package-level `var jdtlsOverrideDataDir string` set via a setter function, OR
- (b) thread a field through `config.SerenaConfig` → `pool.go` → `GetQuirkAdapter`, OR
- (c) env var `SERENA_TEST_JDTLS_DATA_DIR` read inside `ExtraArgs` (simplest, test-only).

Recommend option (c) — smallest surface, test-only, zero API churn on the pool types. Document the env var in the helper package godoc.

---

### `test/integration/harness.go` (test harness)

**Actions:**
1. **Drop** `//go:build integration` (line 1).
2. **Add** `JdtlsDataDir string` field to `Options` struct (line 35).
3. **Thread** it into daemon/pool startup (env-var set on the child daemon process OR via `cfg.*` — depends on the injection choice above).

**Existing `Options` pattern** (lines 34-52) — new field follows same doc-comment convention:
```go
type Options struct {
    // WorkspaceDir is the project path to activate after startup.
    WorkspaceDir string
    // ...
    // JdtlsDataDir, if non-empty, overrides the default workDir/.jdtls-data
    // path passed to jdtls via -data. Used by Java integration tests to
    // share a warm workspace across runs (Phase 48).
    JdtlsDataDir string
}
```

**Fixture path resolution** — reuse as-is (lines 256-287 `PrepareFixture` + line 261 `filepath.Join(projectRoot(), "testdata", "fixtures", lang)`). `jdtlscache.ResolveDataDir` takes the **source** fixture path (not the ephemeral copy) so the content hash is stable across runs.

**Imports that must survive tag drop:** this file already uses `runtime`, `path/filepath`, `os`, `testing` — all stdlib, no tag dependency.

---

### `test/integration/java_test.go` (integration test)

**Actions:**
1. **Drop** `//go:build integration` (line 1).
2. **Delete** lines 18-20 (`if testing.Short() { t.Skip(...) }`) — per D-09.
3. **Wire** warm data dir before `StartTestDaemon`:

```go
func TestSymbols_JavaFixture(t *testing.T) {
    requireLS(t, "jdtls")  // D-10: sole gate

    jdtlsPath, err := exec.LookPath("jdtls")
    require.NoError(t, err)
    fixtureSrc := filepath.Join(projectRoot(), "testdata", "fixtures", "java")
    warmDir, err := jdtlscache.ResolveDataDir("java", fixtureSrc, jdtlsPath)
    require.NoError(t, err)

    fixture := PrepareFixture(t, "java")
    td := StartTestDaemon(t, Options{
        WorkspaceDir: fixture,
        JdtlsDataDir: warmDir,
        LSTimeout:    120 * time.Second,
    })
    // ... subtests unchanged
}
```

**Do NOT** remove `LSTimeout: 120 * time.Second` — cold first-run still needs the generous budget.

---

### `test/integration/helpers.go`, `harness_test.go`, `a_doc.go` (MODIFIED: drop tag)

Each file: delete line 1 (`//go:build integration`) and line 2 (blank). Nothing else changes in `helpers.go`. For `a_doc.go`, update the package doc to note that the Java subset runs without the tag:

**Before** (`a_doc.go`):
```go
//go:build integration

// Package integration_test provides end-to-end integration tests ...
// Run with: go test -tags integration ./test/integration/...
package integration_test
```

**After:**
```go
// Package integration_test provides end-to-end integration tests ...
// Java tests run under default `go test ./...` (warm jdtls cache, Phase 48).
// Other language tests still gate on `-tags integration`.
package integration_test
```

For `harness_test.go` — verify the two tests (`TestHarness_StartAndCallTool`, `TestHarness_FixtureActivateAndReadFile`) use `SkipLS: true` (they do, lines 11 and 20) so they remain fast under default `go test`. Pitfall 2 from RESEARCH.md.

**Files to LEAVE tagged** (per D-11 audit, 16 files): `concurrency_test.go`, `diag_test.go`, `edit_test.go`, `errors_test.go`, `fileops_test.go`, `golden.go`, `memory_test.go`, `mode_golden_test.go`, `profile_golden_test.go`, `profile_test.go`, `python_test.go`, `rust_test.go`, `smoke_http_test.go`, `symbols_test.go`, `trace_propagation_test.go`, `trace_shutdown_test.go`, `typescript_test.go`, `workflow_test.go`.

---

### `Makefile` (add `clean-jdtls-cache` + `bench-jdtls-warm`)

**Analog:** self — existing structure (full file, 28 lines).

**Existing pattern** (lines 1-27):
```makefile
.PHONY: build clean proto test vet fmt docs

BINARY=serena
GO=go

test:
	$(GO) test ./...

docs: ## Regenerate tool and language tables in README.md
	$(GO) run ./cmd/docgen
```

**New targets to append** — follow the `target: ## description` help-text convention established by `docs`:
```makefile
.PHONY: clean-jdtls-cache bench-jdtls-warm

clean-jdtls-cache: ## Wipe warm jdtls workspaces under $XDG_CACHE_HOME/serena-test/jdtls/
	@rm -rf "$${XDG_CACHE_HOME:-$$HOME/.cache}/serena-test/jdtls"
	@echo "cleared jdtls warm cache"

bench-jdtls-warm: ## Run Java integration suite cold then warm; print both wall-clocks
	@$(MAKE) clean-jdtls-cache
	@echo "=== jdtls COLD run ==="
	@time $(GO) test -run 'Java' ./test/integration/... -count=1
	@echo "=== jdtls WARM run ==="
	@time $(GO) test -run 'Java' ./test/integration/... -count=1
```

**Update `.PHONY`** on line 1: add `clean-jdtls-cache bench-jdtls-warm`.

**Platform note:** the `$${XDG_CACHE_HOME:-$$HOME/.cache}` expansion is POSIX; matches D-01's Linux/macOS path. Windows users run tests via Git Bash or run the Go helper directly — the Makefile target is POSIX-only, documented in USAGE.md.

---

### `.github/workflows/go-test.yml` (NEW CI workflow)

**Analog:** `.github/workflows/bench.yml` (structure, Go setup, least-privilege) + `.github/workflows/pytest.yml` lines 58-82, 134-138 (cache usage + `setup-java@v4`).

**Skeleton** — copy bench.yml's header + permissions + `actions/setup-go@v5` block:
```yaml
name: go-test
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
permissions:
  contents: read

jobs:
  test:
    name: go test (ubuntu-latest)
    runs-on: ubuntu-latest
    timeout-minutes: 20
    env:
      JDTLS_VERSION: "1.57.0"  # pin for cache key stability
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true

      - name: Setup Java (for jdtls)
        uses: actions/setup-java@v4
        with:
          distribution: 'temurin'
          java-version: '17'

      - name: Install jdtls
        run: pipx install "jdtls==${JDTLS_VERSION}"

      - name: Cache warm jdtls workspace
        uses: actions/cache@v3
        with:
          path: ~/.cache/serena-test/jdtls
          key: jdtls-warm-${{ runner.os }}-${{ hashFiles('testdata/fixtures/java/**') }}-jdtls-${{ env.JDTLS_VERSION }}
          restore-keys: |
            jdtls-warm-${{ runner.os }}-

      - name: go vet
        run: go vet ./...

      - name: go test
        run: go test ./...
```

**Cache block verbatim source:** `.github/workflows/pytest.yml:56-61`:
```yaml
- name: Cache uv virtualenv
  id: cache-uv
  uses: actions/cache@v3
  with:
    path: .venv
    key: uv-venv-${{ runner.os }}-${{ matrix.python-version }}-lock-${{ hashFiles('uv.lock') }}
```

**Setup-java block verbatim source:** `.github/workflows/pytest.yml:134-138`:
```yaml
- name: Setup Java (for JVM based languages)
  uses: actions/setup-java@v4
  with:
    distribution: 'temurin'
    java-version: '17'
```

## Shared Patterns

### `crypto/sha256` + `encoding/hex` content hashing
**Source:** `internal/memory/index.go:303` (existing SHA-256 use for memory content hashing).
**Apply to:** `jdtlscache/cache.go` only.
```go
h := sha256.New()
h.Write(data)
return hex.EncodeToString(h.Sum(nil))[:12]
```

### `filepath.WalkDir` with sorted post-processing
**Source:** `test/integration/harness.go:264-281` (`PrepareFixture` walk), plus Pitfall 6 correction (explicit sort).
**Apply to:** `jdtlscache/cache.go` (`hashTree`).
```go
var paths []string
err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
    if err != nil { return err }
    if !d.IsDir() { paths = append(paths, p) }
    return nil
})
sort.Strings(paths) // required — Walk order is not cross-OS stable
```

### Error wrapping with `%w`
**Source:** `internal/langregistry/installer.go:64` + idiomatic repo-wide.
**Apply to:** `jdtlscache/cache.go`.
```go
return "", fmt.Errorf("fixture hash: %w", err)
```

### `requireLS` gate + `exec.LookPath`
**Source:** `test/integration/helpers.go:14-20`.
**Apply to:** `java_test.go` (already present — preserved, not modified).
```go
func requireLS(tb testing.TB, binary string) {
    tb.Helper()
    if _, err := exec.LookPath(binary); err != nil {
        tb.Skipf("%s not installed, skipping integration test", binary)
    }
}
```

### `actions/cache@v3` block
**Source:** `.github/workflows/pytest.yml:58-61, 82-87`.
**Apply to:** `.github/workflows/go-test.yml`.

### `QuirkAdapter` / `ArgsModifier` interface contract
**Source:** `internal/kernel/lspool/worker.go:169-175` (dispatch site) + `quirks.go:331-337` (implementation).
**Apply to:** quirks.go modification — the `ArgsModifier` interface is structural, so adding a field on the concrete struct does not break it.

## No Analog Found

None. Every new file has at least one role-match analog in-repo.

## Metadata

**Analog search scope:** `test/integration/`, `internal/kernel/lspool/`, `internal/langregistry/`, `internal/memory/`, `.github/workflows/`, `Makefile`.
**Files scanned:** ~30 read directly; grep coverage over full repo for `QuirkAdapter`, `ArgsModifier`, `actions/cache`, `sha256`, build-tag headers.
**Pattern extraction date:** 2026-04-24
