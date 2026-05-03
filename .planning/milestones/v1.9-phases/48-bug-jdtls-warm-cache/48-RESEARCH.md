# Phase 48: bug-jdtls-warm-cache - Research

**Researched:** 2026-04-24
**Domain:** Go integration test caching; jdtls workspace persistence; CI cache wiring
**Confidence:** HIGH (all key claims verified against repo source and live jdtls binary)

## Summary

Phase 48 makes `test/integration/java_test.go` runnable under default `go test ./...`
by persisting jdtls's `-data` workspace across test invocations, keyed on fixture
content + jdtls binary identity. The existing `JdtlsAdapter.ExtraArgs` hook
(`internal/kernel/lspool/quirks.go:333-337`) already constructs the `-data` flag
from `workDir + ".jdtls-data"`. The warm-cache design plugs in at exactly that
seam — either by redirecting `-data` when a sentinel env var/config is set, or by
pre-creating `workDir/.jdtls-data` as a symlink / hardlink to a persistent cache
directory resolved from `os.UserCacheDir()`.

The two non-obvious risks uncovered during research are: (1) **every file** in
`test/integration/` has `//go:build integration` (22 files, not just
`java_test.go`), so D-08's literal "drop the tag from java_test.go" does not
compile unless the harness files' tag is also addressed; and (2) `jdtls` the CLI
wrapper (e.g. Homebrew 1.57.0) has **no `--version` flag**, so D-05's "jdtls
version hash" must derive from binary path + file stat/content hash, not a
version subcommand.

**Primary recommendation:** Introduce a small `test/integration/jdtlscache` helper
package (pure stdlib — `crypto/sha256`, `os.UserCacheDir`, `filepath.WalkDir`)
that resolves the cache dir, computes both hashes, and returns the path. Wire
it into `StartTestDaemon` via a new `Options.JdtlsDataDir` field; the jdtls
quirk adapter reads that field (via a new pool-level knob) and substitutes it
for the default `workDir/.jdtls-data`. Drop the `//go:build integration` tag
from `java_test.go`, `harness.go`, `harness_test.go`, `helpers.go`, `a_doc.go`,
and `golden.go` together (the minimum set transitively required to compile the
Java tests); leave the tag on the other 16 integration files. `requireLS` keeps
the contributors-without-jdtls path clean.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Cache-key computation (fixture + jdtls hash) | Test helper (`test/integration/jdtlscache`) | — | Pure test-side concern; must not leak into production daemon code |
| Warm `-data` path injection into jdtls process | Test harness (`harness.go` Options → lspool) | `internal/kernel/lspool/quirks.go` JdtlsAdapter | The process launch is owned by lspool; tests need a narrow injection pathway |
| jdtls binary resolution + path surfacing | `internal/langregistry` (already exists) | — | `Installer.Resolve` returns the absolute path — reuse, do not reinvent |
| XDG cache dir resolution | `os.UserCacheDir()` stdlib | — | Stdlib already maps the three-platform layout per D-01 |
| CI cache restore/save | `.github/workflows/*.yml` + `actions/cache@v3` | — | Standard GitHub Actions pattern already used in `pytest.yml` |
| Makefile orchestration | Root `Makefile` | — | One-off target alongside `test`, `vet`, `build` |

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Warm Workspace Location & Scope**
- **D-01:** Warm jdtls `-data` directory lives under `$XDG_CACHE_HOME/serena-test/jdtls/` (fallback `~/.cache/serena-test/jdtls/` on POSIX, `%LOCALAPPDATA%\serena-test\jdtls\` on Windows). Outside the repo so it survives `git clean -fdx` and keys cleanly into CI cache.
- **D-02:** Keyed per-fixture: one warm `-data` dir per Java fixture, matching jdtls's one-workspace-per-project mental model. Directory name embeds the fixture content hash + jdtls version hash so old and new coexist safely.

**Cache Invalidation & Lifecycle**
- **D-03:** Warm workspaces are invalidated by two inputs, combined into the cache key.
- **D-04:** Fixture content hash — SHA-256 over sorted file list + contents under `testdata/fixtures/java/<fixture>/`. Mandatory for correctness.
- **D-05:** jdtls identity hash — hash of the resolved jdtls binary path plus its reported version. Prevents cryptic index-corruption crashes after a jdtls upgrade.
- **D-06:** Keyed-path coexistence: a new hash produces a new directory; stale siblings are **not** deleted during a test run (avoids races with concurrent `go test` invocations on the same machine).
- **D-07:** A `make clean-jdtls-cache` target wipes `$XDG_CACHE_HOME/serena-test/jdtls/` for manual cleanup. Document in USAGE.md and Makefile help text.

**Skip-Gate & Build-Tag Policy**
- **D-08:** Remove the `//go:build integration` tag from `test/integration/java_test.go` so bare `go test ./...` includes it.
- **D-09:** Remove the `testing.Short()` skip from all Java tests.
- **D-10:** `requireLS(t, "jdtls")` stays as the sole gate.
- **D-11:** Planner must audit whether removing the `integration` tag affects other helpers in `test/integration/` that share the same build-tag assumption. If any helper relies on the tag to gate itself, scope the tag-drop to the Java file only or split helpers appropriately.

**Speed Measurement & CI Wall-Clock**
- **D-12:** Add a `make bench-jdtls-warm` Makefile target that runs the Java suite twice (cold then warm) and prints both wall-clocks.
- **D-13:** CI reuses the warm cache via `actions/cache`, keyed on the same inputs as the local warm-cache dir. Cache miss falls back to cold start.
- **D-14:** The phase review must record: (a) local first-run wall-clock, (b) local second-run wall-clock, (c) CI wall-clock with cold cache, (d) CI wall-clock with warm cache.

### Claude's Discretion
- Implementation module placement for the hash/key helpers.
- Exact hash encoding (hex-truncated vs base32, prefix length).
- Whether to set jdtls `-configuration` alongside `-data` under the same keyed dir.
- Concurrent-test lock strategy (e.g. a `flock`-style sentinel).

### Deferred Ideas (OUT OF SCOPE)
- Upstream jdtls tuning (JVM flags, worker counts, project-import hints) — BUG-DEFER-01.
- Multi-JDK matrix testing.
- Concurrent-test advisory lock for the warm dir — planner to evaluate; if non-trivial, split out as follow-up.
- Benchmark gate integration for jdtls warm/cold timing.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BUG-03 | Java integration tests run without `-short=false` by reusing a warm jdtls workspace across test runs (target: the Java integration suite is green in default `go test ./...`) | `JdtlsAdapter.ExtraArgs` hook (`quirks.go:333`) is the single injection seam for `-data`; `Installer.Resolve` (`installer.go:45`) surfaces the jdtls path; all 22 integration files carry `//go:build integration` so D-11's audit is mandatory |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Go only, no CGO.** The repo pins `modernc.org/sqlite` specifically to avoid CGO — any hashing/cache helper MUST use pure stdlib or existing deps.
- **Single Go binary.** Helpers live under `test/integration/` (not shipped with the binary).
- **Must pass `go vet ./...` and `go test ./...` before task completion.**
- **GSD workflow enforcement:** file edits happen through a GSD command.
- **Legacy Python** in `legacy/` is read-only reference — **not** a target of this phase even though `pytest.yml` workflow exists.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `crypto/sha256` | stdlib | Fixture + jdtls binary hashing | Already used in `internal/memory/index.go:303` for content hashing; zero deps |
| `os.UserCacheDir` | stdlib (Go 1.11+) | Resolves platform cache dir per D-01 | Returns `$XDG_CACHE_HOME` (Linux), `~/Library/Caches` (darwin), `%LocalAppData%` (Windows) — **matches D-01 verbatim** [VERIFIED: Go stdlib docs] |
| `filepath.WalkDir` | stdlib (Go 1.16+) | Sorted fixture walk for content hash | Already used in `harness.go:264` (`PrepareFixture`) — reuse the pattern |
| `encoding/hex` | stdlib | Hash encoding for directory name | Shorter than base32 for a 12-char prefix; Windows-safe characters |
| `github.com/stretchr/testify/assert` | (existing) | Java test assertions | Already in `java_test.go` — no new deps |
| `actions/cache@v3` | GitHub Actions | CI warm-cache persistence | Already used 3× in `.github/workflows/pytest.yml` (lines 58, 82, 556) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `runtime` (`runtime.Caller`) | stdlib | Resolve repo root for fixture path | Already used in `harness.go:292` — keep pattern identical |
| `os/exec` (`exec.LookPath`) | stdlib | Resolve jdtls binary path (already done in `langregistry/installer.go:47`) | Reuse `Installer.Resolve`; do not re-invoke `LookPath` in test code |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Binary content hash | `jdtls --version` | jdtls CLI **has no `--version` flag** — verified by running `jdtls -h` [VERIFIED: local execution]. Content-hash of the wrapper script + resolved path is the only portable identity signal. |
| New `jdtlscache` package | Inline helpers in `harness.go` | A dedicated package keeps the hashing/path logic unit-testable without the `integration` tag (the tests can run under default `go test ./...` themselves). |
| `flock`-based lock | Sentinel file + rename-atomic swap | Go stdlib has no `flock` wrapper; would need `golang.org/x/sys/unix`. D-06 already eliminates the need (coexisting directories, no deletion during run). Recommend: **defer lock**, document as follow-up per CONTEXT Deferred. |
| `-configuration` colocation | Default jdtls-managed config dir | The Homebrew wrapper script resolves `-configuration` internally (`/opt/homebrew/Cellar/jdtls/<ver>/libexec/configuration/...`) [VERIFIED: observed in jdtls stderr]. `-data` alone is sufficient; do NOT set `-configuration`. |

**Installation:** No new Go dependencies. All helpers use stdlib.

**Version verification:** stdlib — no npm/registry lookup needed.

## Architecture Patterns

### System Architecture Diagram

```
 [ go test ./... ]
        │
        ▼
 [ java_test.go ] ──requireLS("jdtls")──► (skip if absent)
        │
        ├─► PrepareFixture(t, "java")  ──► ephemeral copy of testdata/fixtures/java/
        │                                  (still per-test, still a tmp dir)
        │
        ├─► jdtlscache.ResolveDataDir(fixtureSrc, jdtlsPath)
        │     │
        │     ├─► sha256(sorted walk of testdata/fixtures/java/)   ── fixture-hash[:12]
        │     ├─► sha256(jdtlsPath || file-content || size)        ── jdtls-hash[:12]
        │     └─► returns  $UserCacheDir/serena-test/jdtls/
        │                    java-<fixture-hash>-<jdtls-hash>/
        │
        ▼
 [ StartTestDaemon(Options{ WorkspaceDir:fixture, JdtlsDataDir:<cache path> }) ]
        │
        ▼
 [ daemon → kernel → lspool.Worker.Start ]
        │
        ▼
 [ JdtlsAdapter.ExtraArgs(workDir, args) ]
        │                           ▲
        │                           └── NEW: if warm path is set via a
        │                                    pool/quirk-level config, emit
        │                                    "-data <warm path>" instead of
        │                                    "-data <workDir>/.jdtls-data"
        ▼
 [ jdtls -data <persistent path> ]
        │
        └─► first run: cold index (slow)
            second run: reuse index (fast)   ◄── D-14 measurement
```

### Recommended Project Structure
```
test/integration/
├── jdtlscache/              # NEW: pure-Go helper package
│   ├── cache.go             # ResolveDataDir, fixture hash, jdtls hash
│   └── cache_test.go        # unit tests (no build tag — runs in default ./...)
├── java_test.go             # NO build tag (per D-08)
├── harness.go               # NO build tag (required by java_test.go)
├── harness_test.go          # NO build tag (same package, must compile)
├── helpers.go               # NO build tag (provides requireLS, callTool)
├── a_doc.go                 # NO build tag (package doc)
├── golden.go                # KEEP tag unless java tests import it (audit)
└── *.go (other *_test.go)   # KEEP //go:build integration
```

### Pattern 1: Quirk-level Injection Point
**What:** `JdtlsAdapter.ExtraArgs` already builds `-data <dir>`; override the dir source.
**When to use:** Always — this is the one and only place the `-data` path is constructed.
**Current code (`internal/kernel/lspool/quirks.go:333-337`):**
```go
func (j *JdtlsAdapter) ExtraArgs(workDir string, args []string) []string {
    dataDir := filepath.Join(workDir, ".jdtls-data")
    _ = os.MkdirAll(dataDir, 0o755)
    return append([]string{"-data", dataDir}, args...)
}
```
**Proposed extension (conceptual — planner refines):**
```go
func (j *JdtlsAdapter) ExtraArgs(workDir string, args []string) []string {
    dataDir := j.overrideDataDir  // set by pool/worker config; empty in prod
    if dataDir == "" {
        dataDir = filepath.Join(workDir, ".jdtls-data")
    }
    _ = os.MkdirAll(dataDir, 0o755)
    return append([]string{"-data", dataDir}, args...)
}
```
The override is propagated via a new `Options.JdtlsDataDir` in the test harness,
threaded through `config.SerenaConfig` or a per-workspace knob into the pool.
Planner confirms the exact plumbing; the hook itself is stable.

### Pattern 2: Content Hash via Sorted Walk
**What:** SHA-256 over (sorted relative paths) || (file contents) for reproducibility.
**When to use:** D-04 fixture hash.
**Example:**
```go
// Pure stdlib. No new deps.
func fixtureHash(root string) (string, error) {
    h := sha256.New()
    var paths []string
    err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        if !d.IsDir() { paths = append(paths, p) }
        return nil
    })
    if err != nil { return "", err }
    sort.Strings(paths)
    for _, p := range paths {
        rel, _ := filepath.Rel(root, p)
        h.Write([]byte(rel))
        h.Write([]byte{0})
        data, err := os.ReadFile(p)
        if err != nil { return "", err }
        h.Write(data)
        h.Write([]byte{0})
    }
    return hex.EncodeToString(h.Sum(nil))[:12], nil
}
```

### Pattern 3: jdtls Identity Hash (no `--version` available)
**What:** D-05 combines resolved path + binary file stat/content.
**Rationale:** `jdtls -h` output (verified locally) shows no `--version` option. The
Homebrew wrapper path itself encodes the version (`/opt/homebrew/Cellar/jdtls/1.57.0/...`),
but a version-free derivation is more robust:
```go
func jdtlsHash(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil { return "", err }
    defer f.Close()
    h := sha256.New()
    h.Write([]byte(path))
    h.Write([]byte{0})
    if _, err := io.Copy(h, f); err != nil { return "", err }
    return hex.EncodeToString(h.Sum(nil))[:12], nil
}
```
For brew-managed jdtls, the wrapper is a small Python script — hashing it
captures upgrades. If the planner wants version extraction, parse `jdtls -h`
output (fragile) or stat the launched-jar path — recommend **NOT** doing this;
binary-content hash is strictly more conservative.

### Anti-Patterns to Avoid
- **Writing the warm dir inside the repo.** Violates D-01 and gets wiped by `git clean -fdx`; also pollutes CI cache keys with repo-path noise.
- **Reusing `workDir/.jdtls-data`.** This is the current prod behavior and the root cause of cold starts — every test gets a fresh `workDir` via `PrepareFixture`.
- **Parsing `jdtls --version` output.** It does not exist. [VERIFIED: `jdtls -h` shows no such flag.]
- **Setting `-configuration`.** The jdtls wrapper manages it internally; adding it risks breaking Homebrew's relocation logic.
- **Deleting stale cache siblings mid-run.** D-06 explicitly forbids this — concurrent `go test` runs would race.
- **Hand-rolling an flock.** Go stdlib has no portable flock; `golang.org/x/sys/unix` is CGO-free but adds surface area. Defer per Claude's Discretion + Deferred list.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Platform cache-dir resolution | Custom `XDG_CACHE_HOME` parsing | `os.UserCacheDir()` | stdlib handles all three platforms per D-01 exactly |
| jdtls binary lookup | `exec.LookPath("jdtls")` in test | `langregistry.Installer.Resolve(ctx, entry)` | Already implemented; returns absolute path + respects managed installs |
| Fixture copy | Custom copy routine | Existing `PrepareFixture` (`harness.go:258`) | Keep the per-test ephemeral copy; only **-data** persists |
| CI cache wiring | Manual tar+upload | `actions/cache@v3` | Already used 3× in `pytest.yml`; same key convention |
| Content hashing | Tree-sitter or repomap invalidation | `crypto/sha256` + sorted walk | Repomap's cache uses mtime — inappropriate for CI-portable keys |

**Key insight:** Every primitive needed here already exists in stdlib or repo.
The phase is pure wiring; new **code volume** should be ≤150 LOC of helper +
one Make target + one CI step.

## Runtime State Inventory

> This is a test-infra change, not a rename/refactor, but the same question
> applies: what persistent state survives outside git?

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **NEW** `$UserCacheDir/serena-test/jdtls/java-<h1>-<h2>/` — jdtls index data. First occurrence. | Wiped by `make clean-jdtls-cache` (D-07); not git-tracked. |
| Live service config | None — no running external services. | — |
| OS-registered state | None — no pm2/systemd/launchd/scheduled tasks. | — |
| Secrets/env vars | None introduced. Tests do not read secrets. | — |
| Build artifacts | No new artifacts. Existing `.jdtls-data` inside ephemeral workDir becomes vestigial for Java tests (but still used elsewhere — keep). | — |

**Nothing found in category:** Live services, OS registrations, secrets, build artifacts — verified by `grep`-ing the repo for `systemd|launchd|pm2|SOPS|.env` in prior audits (none of these patterns exist in the codebase beyond unrelated contexts).

## Common Pitfalls

### Pitfall 1: Build-tag asymmetry breaks `go test ./...`
**What goes wrong:** D-08 says "drop the `integration` tag from `java_test.go`". Done literally, the file won't compile in default builds because it references `PrepareFixture`, `StartTestDaemon`, `Options`, `requireLS`, `callTool`, `textContent` — all defined in files tagged `//go:build integration`.
**Why it happens:** Every file in `test/integration/` currently has the tag [VERIFIED: `head -1 test/integration/*.go`]. No subset is tag-free.
**How to avoid:** Drop the tag from the transitive closure of symbols referenced by `java_test.go`: `harness.go` (Options, StartTestDaemon, PrepareFixture, WaitForLS, defaultTestConfig, projectRoot, TestDaemon, requireGopls), `helpers.go` (requireLS, callTool, textContent, listSessionTools, callToolExpectError), `a_doc.go` (package doc), and `harness_test.go` (same package, must compile together). `golden.go` only if java tests reference golden helpers — they currently don't, so leave tagged.
**Warning signs:** `go build ./test/integration/...` without `-tags=integration` failing with "undefined: StartTestDaemon". CI must run **without** `-tags=integration` to catch this.

### Pitfall 2: Dropping the `integration` tag pulls other tests into default `go test`
**What goes wrong:** The other 16 *_test.go files (`concurrency_test.go`, `python_test.go`, etc.) are in `package integration_test`. They KEEP their tag; because Go's build-tag gating is per-file, the non-Java tests remain gated. However, harness_test.go has its own tests (`TestHarness_StartAndCallTool`, `TestHarness_FixtureActivateAndReadFile`) that will now run in default builds.
**Why it happens:** Removing the tag from `harness_test.go` exposes its tests.
**How to avoid:** Audit `harness_test.go`'s tests — they use `SkipLS: true` or a Go fixture, so they should be safe. If they're too slow for default runs, move them to a new `harness_integration_test.go` that keeps the tag, and leave `harness.go` (non-test file) without the tag.
**Warning signs:** CI wall-clock for `go test ./...` jumps unexpectedly on non-Java tests.

### Pitfall 3: Persistent `-data` dir across jdtls version bumps
**What goes wrong:** Upgrading jdtls (brew upgrade jdtls) against a workspace indexed by an older version produces cryptic crashes ("workspace incompatible").
**Why it happens:** jdtls index format is not guaranteed forward-compatible.
**How to avoid:** D-05 jdtls-hash component — any binary change produces a new directory.
**Warning signs:** `NullPointerException` or "workspace corrupted" in jdtls stderr after upgrade.

### Pitfall 4: Two concurrent `go test` runs on same fixture
**What goes wrong:** Both resolve to the same warm `-data` dir; jdtls requires exclusive lock on its workspace and will fail on the second.
**Why it happens:** No advisory lock; D-06 forbids deletion mid-run but doesn't prevent contention.
**How to avoid:** CONTEXT's Claude's Discretion allows deferring this. Recommend **defer** — document in phase review. Mitigation: contributors running parallel suites should `make clean-jdtls-cache` between runs.
**Warning signs:** "OSGi framework failed to start" or ".metadata locked" in test logs.

### Pitfall 5: CI cache-key mismatch between local and remote
**What goes wrong:** Local key = `sha256(fixture)[:12]-sha256(binary)[:12]`; CI's key = `hashFiles('testdata/fixtures/java/**')-<jdtls-version-from-install-step>`. They compute differently; CI never hits local cache format (and vice versa — which is fine, they are separate machines).
**Why it happens:** `actions/cache` uses its own hashing.
**How to avoid:** The CI key does NOT need to match the local key byte-for-byte. CI only needs the cache `path:` to be the UserCacheDir, and the key to encode the same *inputs* (fixture tree + jdtls version). Use `hashFiles('testdata/fixtures/java/**')` + the installed jdtls's path or a pinned apt/brew version string.
**Warning signs:** CI reports 100% cache-miss even on unchanged PRs.

### Pitfall 6: Fixture hash non-determinism from filesystem ordering
**What goes wrong:** `filepath.WalkDir` claims lexical ordering but some filesystems (case-insensitive macOS HFS+) can return mixed case; hash differs between macOS and Linux.
**Why it happens:** Underlying `readdir` order.
**How to avoid:** Sort `paths` explicitly after walk (shown in Pattern 2). Don't rely on Walk order.
**Warning signs:** First CI run after a Darwin-local run always misses cache.

## Code Examples

### Resolve the warm -data directory
```go
// Source: this phase — composed from stdlib patterns
package jdtlscache

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "io/fs"
    "os"
    "path/filepath"
    "sort"
)

// ResolveDataDir returns the absolute path to the warm jdtls -data dir for a
// given fixture and resolved jdtls binary. It creates the directory if missing.
func ResolveDataDir(fixtureName, fixtureRoot, jdtlsPath string) (string, error) {
    fh, err := hashTree(fixtureRoot)
    if err != nil { return "", fmt.Errorf("fixture hash: %w", err) }
    jh, err := hashBinary(jdtlsPath)
    if err != nil { return "", fmt.Errorf("jdtls hash: %w", err) }

    cache, err := os.UserCacheDir()
    if err != nil { return "", fmt.Errorf("user cache dir: %w", err) }

    dir := filepath.Join(cache, "serena-test", "jdtls",
        fmt.Sprintf("%s-%s-%s", fixtureName, fh, jh))
    if err := os.MkdirAll(dir, 0o755); err != nil { return "", err }
    return dir, nil
}

func hashTree(root string) (string, error) {
    var paths []string
    err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        if !d.IsDir() { paths = append(paths, p) }
        return nil
    })
    if err != nil { return "", err }
    sort.Strings(paths)

    h := sha256.New()
    for _, p := range paths {
        rel, _ := filepath.Rel(root, p)
        h.Write([]byte(rel))
        h.Write([]byte{0})
        data, err := os.ReadFile(p)
        if err != nil { return "", err }
        h.Write(data)
        h.Write([]byte{0})
    }
    return hex.EncodeToString(h.Sum(nil))[:12], nil
}

func hashBinary(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil { return "", err }
    defer f.Close()
    h := sha256.New()
    h.Write([]byte(path))
    h.Write([]byte{0})
    if _, err := io.Copy(h, f); err != nil { return "", err }
    return hex.EncodeToString(h.Sum(nil))[:12], nil
}
```

### Makefile targets (D-07 and D-12)
```makefile
# Source: this phase; conforms to existing Makefile conventions.
.PHONY: clean-jdtls-cache bench-jdtls-warm

clean-jdtls-cache: ## Wipe warm jdtls workspaces under $UserCacheDir/serena-test/jdtls/
	@$(GO) run ./cmd/serena --print-cache-dir 2>/dev/null || true
	@rm -rf "$${XDG_CACHE_HOME:-$$HOME/.cache}/serena-test/jdtls"
	@echo "cleared jdtls warm cache"

bench-jdtls-warm: ## Run Java integration suite cold then warm; print both wall-clocks.
	@$(MAKE) clean-jdtls-cache
	@echo "=== jdtls COLD run ==="
	@time $(GO) test -run 'Java' ./test/integration/... -count=1
	@echo "=== jdtls WARM run ==="
	@time $(GO) test -run 'Java' ./test/integration/... -count=1
```

### CI wiring (addition to `.github/workflows/pytest.yml` or a new Go-test workflow)
```yaml
# Source: pattern already used in .github/workflows/pytest.yml:58-82
- name: Cache warm jdtls workspace
  uses: actions/cache@v3
  with:
    path: ~/.cache/serena-test/jdtls
    key: jdtls-warm-${{ runner.os }}-${{ hashFiles('testdata/fixtures/java/**') }}-jdtls-${{ env.JDTLS_VERSION }}
    restore-keys: |
      jdtls-warm-${{ runner.os }}-
```
**Note:** The repo currently has **no GitHub Actions workflow that runs `go test ./...`** — `pytest.yml` is for Python legacy, `bench.yml` runs `go test -bench`. The planner must decide whether to (a) add the Java integration step to `bench.yml`, (b) add a new `go-test.yml` workflow, or (c) extend `pytest.yml` (less clean). Recommend **(b)** — new workflow — cleanest. This is an in-scope planning decision, not out-of-scope Phase 50 work (Phase 50 addresses gopls/Go 1.25, not the missing go-test workflow).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `testing.Short()` skip + `//go:build integration` tag | Warm `-data` dir keyed on fixture + jdtls identity | This phase | Java suite joins default `go test ./...` |
| `workDir + ".jdtls-data"` (ephemeral) | `UserCacheDir/serena-test/jdtls/<key>/` (persistent) | This phase | Second run hits index cache, not cold start |

**Deprecated/outdated:** None — this is net-new test infra.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Homebrew jdtls wrapper binary-content hash is stable enough to represent "identity" for cache invalidation | Pattern 3 | Low — if stale, hash changes when brew upgrades so cache refreshes; worst case is over-invalidation (safe) |
| A2 | `harness_test.go` tests are fast enough to run in default `go test` once the tag is dropped | Pitfall 2 | Medium — if slow, move tests to `harness_integration_test.go` keeping the tag; verify during execution |
| A3 | No other integration test file transitively imports types from the Java-side tag-dropped files in ways that would re-tag them | Pitfall 1 | Low — the shared package is `integration_test`; file-level tags are independent in Go |
| A4 | CI does not currently have a `go test ./...` workflow (only bench) | CI note | Low — verified by `grep "go test" .github/workflows/*.yml`; only `-bench=` matches exist |
| A5 | Deferring the concurrent-run lock is acceptable to the user per CONTEXT Discretion | Pitfall 4 / Alternatives | Low — flagged explicitly; user can overrule in plan-check |

## Open Questions

1. **Should `-configuration` be colocated?**
   - What we know: Homebrew jdtls wrapper manages its own `-configuration` internally; the CLI does not expose it.
   - What's unclear: Other jdtls distributions (pip `jdtls` package, manual download) may behave differently.
   - Recommendation: Do not set `-configuration`. Document that assumption in the phase review; revisit if the managed-install tier (Tier 2 in `Installer.Resolve`) ever ships jdtls.

2. **Which CI workflow gets the Go integration step?**
   - What we know: No `go test ./...` workflow exists today.
   - What's unclear: Whether Phase 50 (`toolchain-go1.25-gopls-ci`) plans to introduce one.
   - Recommendation: Add a minimal `.github/workflows/go-test.yml` in this phase that runs `go test ./...` + the `actions/cache` step for the jdtls dir. Keep it small — Phase 50 can expand scope.

3. **Fixture hash scope: `.classpath` and `.project` inclusion?**
   - What we know: The Java fixture contains `.project`, `.classpath`, `pom.xml`, `Main.java`, `Greeter.java`. All feed jdtls's project model.
   - What's unclear: Whether a pure hash over the full fixture directory is desired, or whether some files (e.g. editor-generated `.project`) should be excluded.
   - Recommendation: Hash the entire fixture directory. The fixture is small (5 files); over-invalidation is cheap and under-invalidation risks stale index crashes.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `jdtls` (local dev) | Running Java integration test | ✓ | 1.57.0 (Homebrew, verified) | `requireLS(t, "jdtls")` skips cleanly if missing — D-10 |
| `jdtls` (CI) | CI Java integration step | ✗ (no Go-test workflow runs jdtls today) | — | Install step must be added: `brew install jdtls` on macOS, manual install on Linux (ubuntu-latest). See Pitfall 5. |
| Java 17+ (JDK) | jdtls runtime | ✓ (local), ✓ CI (`actions/setup-java@v4` in `pytest.yml:134-138`) | Temurin 17 | Already wired for pytest; reuse for go-test workflow |
| `go` (≥1.21) | Build + test | ✓ | repo's go.mod pins 1.25 | — |
| `make` | Orchestration | ✓ | — | — |

**Missing dependencies with no fallback:**
- CI install of `jdtls` on Linux runners — Linux has no `brew install jdtls` that works reliably without Homebrew-for-Linux. Planner must choose: pip install `jdtls` (`pip install jdtls`), or download a released tarball from Eclipse. Recommend `pipx install jdtls` in the workflow (standard, fast).

**Missing dependencies with fallback:**
- None.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/stretchr/testify/assert` (existing) |
| Config file | None — Go convention; tests live beside source |
| Quick run command | `go test -run 'TestSymbols_JavaFixture' ./test/integration/... -count=1` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BUG-03 | Fixture hash is deterministic across OS | unit | `go test ./test/integration/jdtlscache/... -run TestFixtureHashStable -count=1` | ❌ Wave 0 |
| BUG-03 | Fixture hash changes on file edit | unit | `go test ./test/integration/jdtlscache/... -run TestFixtureHashInvalidation -count=1` | ❌ Wave 0 |
| BUG-03 | jdtls hash changes on binary replacement | unit | `go test ./test/integration/jdtlscache/... -run TestJdtlsHashInvalidation -count=1` | ❌ Wave 0 |
| BUG-03 | `ResolveDataDir` creates directory under `UserCacheDir` | unit | `go test ./test/integration/jdtlscache/... -run TestResolveDataDir -count=1` | ❌ Wave 0 |
| BUG-03 | Cold-vs-warm wall-clock delta is positive | manual/make | `make bench-jdtls-warm` | ❌ Wave 0 (Makefile target missing) |
| BUG-03 | Java integration suite passes in default `go test ./...` | integration | `go test -run 'Java' ./test/integration/... -count=1` (jdtls must be installed) | ✅ (file exists; will run after tag drop) |
| BUG-03 | Tag drop does not break other integration tests | build | `go build ./test/integration/...` (no `-tags=integration`) | ✅ (just needs to compile) |
| BUG-03 | `make clean-jdtls-cache` removes the cache dir | integration | `make clean-jdtls-cache && test ! -d $HOME/.cache/serena-test/jdtls` | ❌ Wave 0 (target missing) |
| BUG-03 | Cache-miss fallback: empty/missing cache dir falls back to cold start cleanly | integration | `rm -rf $HOME/.cache/serena-test/jdtls && make bench-jdtls-warm` (first leg = cold; must complete without error) | ❌ Wave 0 |
| BUG-03 | CI cache-hit verification | CI inspection | Check `actions/cache` step logs in PR — `Cache restored successfully` on second run | ❌ Wave 0 (workflow missing) |
| BUG-03 | Concurrent-run safety | manual note | Deferred per Claude's Discretion — documented as follow-up in phase review; no test required | N/A (deferred) |

### Sampling Rate
- **Per task commit:** `go test ./test/integration/jdtlscache/... -count=1 && go vet ./...`
- **Per wave merge:** `go test ./...` (will now include Java if jdtls is on PATH)
- **Phase gate:** `make bench-jdtls-warm` manually; record wall-clocks in phase review per D-14

### Wave 0 Gaps
- [ ] `test/integration/jdtlscache/cache.go` — helper package (new)
- [ ] `test/integration/jdtlscache/cache_test.go` — unit tests for hash + ResolveDataDir (new)
- [ ] `Makefile` — add `clean-jdtls-cache` and `bench-jdtls-warm` targets
- [ ] `.github/workflows/go-test.yml` — new workflow (no existing go-test workflow detected)
- [ ] Tag removal from `java_test.go`, `harness.go`, `harness_test.go`, `helpers.go`, `a_doc.go` (and audit `golden.go`)
- [ ] `Options.JdtlsDataDir` field + plumbing from harness → pool → quirk adapter
- [ ] `JdtlsAdapter` override field (or pool-level config) for data-dir override

## Sources

### Primary (HIGH confidence)
- **Repo source (verified via Read):** `internal/kernel/lspool/quirks.go:303-337` — JdtlsAdapter + ExtraArgs
- **Repo source:** `internal/kernel/lspool/worker.go:166-175` — quirks.ExtraArgs call site
- **Repo source:** `internal/langregistry/installer.go:43-66` — Resolve returns path
- **Repo source:** `internal/langregistry/languages.go:255-259` — Java LSEntry
- **Repo source:** `test/integration/harness.go:1,258-287` — PrepareFixture, build tag
- **Repo source:** `test/integration/java_test.go:1,18-23,102-105` — tag + Short() skip
- **Repo source:** `test/integration/helpers.go:1,15-20` — requireLS
- **Local execution:** `jdtls -h` output — confirms no `--version` flag
- **Local execution:** `jdtls --version` observed banner — confirms 1.57.0 via Homebrew path
- **Repo source:** `.github/workflows/pytest.yml:58,82,556` — `actions/cache@v3` usage pattern
- **Repo source:** `.github/workflows/pytest.yml:134-138` — `actions/setup-java@v4` already present
- **Repo source:** `Makefile` (full file) — existing `test`, `vet`, `build`, `fmt`, `docs` targets

### Secondary (MEDIUM confidence)
- Go stdlib docs for `os.UserCacheDir()` — platform mapping behavior

### Tertiary (LOW confidence)
- jdtls multi-process workspace-lock behavior — inferred from Eclipse's historical `.metadata/.lock` pattern; has not been tested in this phase. Basis for Pitfall 4 recommendation to defer the lock.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, all verified against repo
- Architecture: HIGH — injection point located (`quirks.go:333`), shape of change clear
- Pitfalls: HIGH — build-tag asymmetry verified by `head -1` audit; jdtls version-flag absence verified by local run
- CI wiring: MEDIUM — the absence of a Go-test workflow is a real gap, not a bug in research
- Concurrency/lock: LOW — not exercised in this phase, recommendation is to defer

**Research date:** 2026-04-24
**Valid until:** 2026-05-24 (30 days — stable codebase, no fast-moving upstream dependency)
