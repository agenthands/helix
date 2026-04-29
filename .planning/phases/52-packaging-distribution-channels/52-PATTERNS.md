# Phase 52: packaging-distribution-channels - Pattern Map

**Mapped:** 2026-04-29
**Files analyzed:** 21 (9 new, 12 modified by mechanical rename)
**Analogs found:** 18 / 21 (3 have no analog and rely on RESEARCH.md library guidance)

This phase has three workstreams: (1) mechanical rename `serena` → `helix` with module path flip, (2) new `internal/upgrade/` package, (3) docs/manifest edits. The rename's "analogs" are the *existing* files (the new versions are textually-flipped copies); pattern extraction focuses on the upgrade package.

## File Classification

### Workstream 1 — Rename (modifications, mechanical)

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/helix/main.go` (move from `cmd/serena/main.go`) | binary entrypoint | request-response | `cmd/serena/main.go` (the file itself, post-rename) | exact (self-rename) |
| `go.mod` (module rename) | build config | n/a | n/a | n/a — single-line `go mod edit` |
| `.goreleaser.yaml` | build config | batch | existing file, post-edit | exact (self-rename) |
| `Makefile` | build config | n/a | existing file, post-edit | exact (self-rename) |
| `internal/cli/root.go` (`Use: "serena"` → `"helix"` etc.) | controller (cobra root) | request-response | self | exact (self-rename) |
| `internal/cli/setup_clients.go` (registration name `"serena"` → `"helix"`) | controller (per-client registrar) | request-response | self | exact (self-rename) |
| `internal/mcp/server.go` (`Implementation.Name`) | service init | n/a | self | exact (self-rename) |
| `internal/config/defaults.go` (`.serena` → `.helix`, `service_name`) | config | n/a | self | exact (self-rename) |
| All other `*.go` with `github.com/postfix/serena` imports (392 lines) | various | various | self | exact (self-rename) — driven by `go mod edit` + project-wide find/replace |
| `INSTALL.md`, `README.md`, `USAGE.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `CLAUDE.md` | docs | n/a | self | exact (self-rename) |

### Workstream 2 — `internal/upgrade/` (new package)

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `internal/upgrade/upgrade.go` (orchestrator) | service | request-response (one-shot CLI) | `internal/langregistry/installer.go` | role-match (download + verify + place pattern) |
| `internal/upgrade/github.go` (GitHub Releases API client) | service (HTTP client) | request-response | `internal/langregistry/installer.go` lines 184-195 (binary download) | role-match (HTTP GET + status check + body) |
| `internal/upgrade/verify.go` (minisign wrapper) | service | transform | `internal/langregistry/installer.go` lines 203-217 (sha256 verify) | partial (verify-on-download flow shape; crypto primitive differs) |
| `internal/upgrade/archive.go` (tar.gz extract) | utility | transform | none in code; stdlib `archive/tar` from RESEARCH.md | no analog → use RESEARCH.md Pattern 7 |
| `internal/upgrade/semver.go` (compare + prerelease filter) | utility | transform | none in code; `golang.org/x/mod/semver` from RESEARCH.md | no analog → use RESEARCH.md Pattern 6 |
| `internal/upgrade/permission.go` (writability probe + sudo hint) | utility | transform | `internal/langregistry/installer.go` lines 197-201 (CreateTemp probe) | partial (CreateTemp pattern reusable) |
| `internal/upgrade/stage.go` (stage-dir lifecycle) | utility | file-I/O | `internal/langregistry/installer.go` lines 197-209 (CreateTemp + cleanup) | role-match |
| `internal/upgrade/swap_unix.go` (`//go:build !windows`) | utility (platform-split) | file-I/O | `internal/forwarder/dial_unix.go` | exact (build-tag pair) |
| `internal/upgrade/swap_windows.go` (`//go:build windows`) | utility (platform-split) | file-I/O | `internal/forwarder/dial_windows.go` | exact (build-tag pair) |
| `internal/upgrade/daemon_detect.go` (env-var probe) | utility | request-response | `test/integration/harness.go` lines 112, 129 (env-var probe pattern) | partial |
| `internal/upgrade/pubkey.go` (`//go:embed minisign.pub`) | utility (embed) | n/a | `internal/profile/embed.go` | exact (single-file embed pattern) |
| `internal/upgrade/minisign.pub` (build-time-synced copy) | data | n/a | n/a — build artifact | n/a |
| `internal/upgrade/upgrade_test.go` (httptest stubs) | test | n/a | `internal/daemon/telemetry_test.go` lines 50-77 (httptest pattern) | role-match |
| `internal/cli/update.go` (cobra subcommand) | controller (cobra) | request-response | `internal/cli/status.go` lines 20-36 | exact (subcommand pattern) |
| `internal/cli/upgrade.go` (cobra subcommand) | controller (cobra) | request-response | `internal/cli/setup.go` lines 16-39 | exact (subcommand with flags) |

### Workstream 3 — Docs / planning artifacts

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `.planning/phases/52-.../EMBED-AUDIT.md` (NEW) | docs | n/a | n/a — phase artifact | no analog (free-form manifest) |
| `.planning/REQUIREMENTS.md` edits | docs | n/a | self | exact |
| `.planning/ROADMAP.md` edits | docs | n/a | self | exact |

## Pattern Assignments

### `internal/cli/update.go` (controller, cobra subcommand)

**Analog:** `internal/cli/status.go`

**Imports + cobra root pattern** (`internal/cli/status.go` lines 1-36):
```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	// ... project imports
)

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show workspace health and language server status",
		Long:  `...multi-line description...`,
		RunE:          runStatus,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().Bool("json", false, "Output as JSON ...")
	cmd.Flags().BoolP("verbose", "v", false, "Show all language servers ...")
	return cmd
}
```

**Wiring into `internal/cli/root.go`** (existing pattern, lines 49-54):
```go
rootCmd.AddCommand(newSetupCommand())
rootCmd.AddCommand(newStatusCommand())
rootCmd.AddCommand(newActivateCommand())
rootCmd.AddCommand(newDeactivateCommand())
rootCmd.AddCommand(newNudgeCommand())
// ADD:
rootCmd.AddCommand(newUpdateCommand())
rootCmd.AddCommand(newUpgradeCommand())
```

---

### `internal/cli/upgrade.go` (controller, cobra subcommand with flags)

**Analog:** `internal/cli/setup.go`

**Multi-flag command pattern** (`internal/cli/setup.go` lines 16-39):
```go
func newSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup [client]",
		Short: "Register Serena as MCP server for a coding agent",
		Long:  `...`,
		ValidArgs: []string{...},
		Args:      cobra.MaximumNArgs(1),
		RunE:      runSetup,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().Bool("global", false, "...")
	cmd.Flags().Bool("uninstall", false, "...")
	cmd.Flags().Bool("dry-run", false, "Show what would happen without making changes")
	return cmd
}
```

For `upgrade` follow this verbatim with flags `--prerelease`, `--version`, `--check`, `--dry-run` (per D-12).

---

### `internal/upgrade/upgrade.go` + `github.go` (service, HTTP client)

**Analog:** `internal/langregistry/installer.go`

**HTTP download + status-check + body-read pattern** (lines 184-209):
```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
if err != nil {
    return "", fmt.Errorf("creating request: %w", err)
}
resp, err := http.DefaultClient.Do(req)
if err != nil {
    return "", fmt.Errorf("downloading %s: %w", entry.Command, err)
}
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK {
    return "", fmt.Errorf("download failed: HTTP %d for %s", resp.StatusCode, url)
}

tmpFile, err := os.CreateTemp(i.config.BinDir, entry.Command+"-*")
if err != nil {
    return "", fmt.Errorf("creating temp file: %w", err)
}
defer func() { _ = os.Remove(tmpFile.Name()) }()

hasher := sha256.New()
writer := io.MultiWriter(tmpFile, hasher)
if _, err := io.Copy(writer, resp.Body); err != nil {
    _ = tmpFile.Close()
    return "", fmt.Errorf("writing download: %w", err)
}
```

**Adaptations for `internal/upgrade/`:**
- Add `User-Agent: helix-upgrade/<version>` header (RESEARCH.md Pitfall 6) — the langregistry installer does not set one; the upgrade subcommand must.
- Add `Accept: application/vnd.github+json` and `X-GitHub-Api-Version: 2026-03-10` headers per RESEARCH.md Pattern 1.
- Replace `sha256` MultiWriter with the `minisign.Verify*` API at the equivalent integrity-check point (see `verify.go` below).
- Replace `i.config.BinDir` stage location with a sibling-of-install-path stage dir (RESEARCH.md Pitfall 3 — `os.TempDir()` cross-fs causes non-atomic rename).
- Use `errgroup` only if downloading archive + signature in parallel; otherwise sequential is fine.

---

### `internal/upgrade/verify.go` (service, signature verification)

**No close codebase analog.** The langregistry installer verifies by `sha256` only (lines 211-217); minisign Ed25519 verification is new in this codebase. Use RESEARCH.md Pattern 2 verbatim:

```go
import "github.com/jedisct1/go-minisign"

//go:embed minisign.pub
var pubKeyBytes []byte

func VerifyArchive(archivePath, sigPath string) error {
    pub, err := minisign.DecodePublicKey(string(pubKeyBytes))
    if err != nil { return fmt.Errorf("decoding embedded minisign.pub: %w", err) }
    sig, err := minisign.NewSignatureFromFile(sigPath)
    if err != nil { return fmt.Errorf("parsing %s: %w", sigPath, err) }
    ok, err := pub.VerifyFromFile(archivePath, sig)
    if err != nil || !ok {
        return fmt.Errorf("signature verification FAILED: %w", err)  // single error msg per Pitfall 4
    }
    return nil
}
```

---

### `internal/upgrade/permission.go` (utility, writability probe)

**Analog:** `internal/langregistry/installer.go` lines 197-201

**CreateTemp-as-probe pattern** (existing):
```go
tmpFile, err := os.CreateTemp(i.config.BinDir, entry.Command+"-*")
if err != nil {
    return "", fmt.Errorf("creating temp file: %w", err)
}
defer func() { _ = os.Remove(tmpFile.Name()) }()
```

**Adapt for `ProbeWritable`** (per RESEARCH.md Code Examples):
```go
func ProbeWritable(installPath string) error {
    dir := filepath.Dir(installPath)
    f, err := os.CreateTemp(dir, ".helix-perm-probe-*")
    if err != nil { return err }
    name := f.Name()
    f.Close()
    os.Remove(name)
    return nil
}
```

---

### `internal/upgrade/swap_unix.go` + `swap_windows.go` (utility, platform-split)

**Analog:** `internal/forwarder/dial_unix.go` + `internal/forwarder/dial_windows.go`

**Build-tag header pattern** (`internal/forwarder/dial_unix.go` lines 1-8):
```go
//go:build !windows

package forwarder

import (
    "os/exec"
    "syscall"
)

// detachFromProcessGroup configures cmd to start in its own process group ...
func detachFromProcessGroup(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
```

**Windows counterpart** (`internal/forwarder/dial_windows.go` lines 1-27):
```go
//go:build windows

package forwarder

import (
    "os/exec"
    "syscall"
    "golang.org/x/sys/windows"
)

func detachFromProcessGroup(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{
        CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
    }
}
```

**Apply to `internal/upgrade/swap_*.go`:**
- Same `//go:build` header convention, same package, same exported function-pair signatures across both files.
- Include compile-time signature assertions per Phase 51.1 WR-03 precedent (the codebase already enforces cgo/!cgo stubs match — extend this discipline to upgrade swap functions).
- Use RESEARCH.md Patterns 3 + 4 for the function body.

---

### `internal/upgrade/pubkey.go` (`//go:embed minisign.pub`)

**Analog:** `internal/profile/embed.go`

**Single-file embed pattern** (`internal/profile/embed.go` complete file):
```go
package profile

import "embed"

//go:embed profiles/*.yaml
var embeddedProfiles embed.FS

//go:embed modes/*.yaml
var embeddedModes embed.FS
```

**Adapt for upgrade** (D-13: file MUST be sibling, not `..` traversal):
```go
package upgrade

import _ "embed"

//go:embed minisign.pub
var pubKeyBytes []byte
```

The build-time copy `cp minisign.pub internal/upgrade/minisign.pub` is performed by:
- `make build` pre-step (Makefile target `embed-pubkey`)
- `go generate ./internal/upgrade/...` directive in `pubkey.go`
- CI gate `verify-embed-pubkey` (cmp byte-for-byte; fail if drift)
- `internal/upgrade/minisign.pub` is git-ignored (single source of truth at repo root) **OR** checked-in with the verify-embed-pubkey gate enforcing sync — planner picks per D-13 wording.

---

### `internal/upgrade/daemon_detect.go` (utility, env-var probe)

**Analog:** `test/integration/harness.go` lines 112, 129

**Env-var probe pattern** (existing test code):
```go
tb.Setenv("SERENA_TEST_JDTLS_DATA_DIR", opts.JdtlsDataDir)
// ...
if os.Getenv("SERENA_TEST_LS_DEBUG") != "" {
```

**Adapt for daemon detect** (RESEARCH.md Pattern 5):
```go
func RunningInDaemon() bool {
    return os.Getenv("HELIX_RUNNING_AS_DAEMON") == "1"
}
```

The setter side requires editing `internal/daemon/daemon.go` (or wherever child processes are spawned) to set `cmd.Env = append(os.Environ(), "HELIX_RUNNING_AS_DAEMON=1")`.

---

### `internal/upgrade/upgrade_test.go` (test, httptest stub)

**Analog:** `internal/daemon/telemetry_test.go` lines 50-77

**httptest pattern** (existing):
```go
import (
    "net/http"
    "net/http/httptest"
    // ...
)

func TestHandleHealthz(t *testing.T) {
    d := minimalDaemon(config.ObservabilityConfig{})
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rec := httptest.NewRecorder()
    d.handleHealthz(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("status = %d, want 200", rec.Code) }
    // ...
}
```

**Adapt to a server stub** (the daemon test uses `NewRequest`/`NewRecorder` — for upgrade we need `httptest.NewServer` returning canned release JSON). Use RESEARCH.md Code Examples §"Test stub for GitHub API":
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    require.Equal(t, "/repos/agenthands/helix/releases/latest", r.URL.Path)
    require.Contains(t, r.Header.Get("User-Agent"), "helix-upgrade/")
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(Release{ /* canned response */ })
}))
defer server.Close()
```

---

### `cmd/helix/main.go` (rename from `cmd/serena/main.go`)

**Analog:** `cmd/serena/main.go` (the file itself)

**Existing content** (preserve verbatim minus paths) — `cmd/serena/main.go` lines 1-17:
```go
package main

import (
    "fmt"
    "os"

    "github.com/postfix/serena/internal/cli"
)

func main() {
    cmd := cli.NewRootCommand()
    if err := cmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

**Required edits per RESEARCH.md Open Question 2 + A7:** add `var version = "dev"` so the goreleaser ldflag `-X main.version=...` has a target:
```go
package main

import (
    "fmt"
    "os"

    "github.com/agenthands/helix/internal/cli"  // module path flip
)

// version is set via -ldflags="-X main.version=$VERSION" by goreleaser; see .goreleaser.yaml line 26.
var version = "dev"

func main() {
    cli.SetVersion(version)  // thread the ldflag-injected value into the cli package
    cmd := cli.NewRootCommand()
    if err := cmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

The corresponding `cli.SetVersion()` API needs to be added to `internal/cli/root.go`; the existing `fmt.Println("serena version 2.0.0-dev")` (line 62) gets replaced with `fmt.Printf("helix version %s\n", currentVersion)`.

---

### `internal/mcp/server.go` (Implementation.Name flip)

**Existing line 51:**
```go
&mcpsdk.Implementation{Name: "serena", Version: "2.0.0-dev"},
```

**After rename** — must flip to `"helix"`. The `Version` literal also flips to the ldflag-injected value (RESEARCH.md A6 — this MUST stay in lockstep with the cobra `--version` output).

---

### `.goreleaser.yaml` (rename)

**Existing template** (lines 1-44, four lines flip):
```yaml
project_name: serena            # → helix
builds:
  - id: serena                  # → helix
    main: ./cmd/serena          # → ./cmd/helix
    binary: serena              # → helix
    # ...
archives:
  - id: serena                  # → helix
    ids: [serena]               # → [helix]
    name_template: "serena_v{{ .Version }}_{{ .Os }}_{{ .Arch }}"  # → "helix_v..."
```

The signing config (lines 50-63) and changelog config (lines 65-88) remain unchanged. The `release.github.owner: agenthands` / `name: helix` (lines 90-93) is already correct.

---

### `Makefile` (rename + add `embed-pubkey` targets)

**Existing self-doc style** (lines 26, 29, 37, 44, 47, 50):
```makefile
docs: ## Regenerate tool and language tables in README.md
	$(GO) run ./cmd/docgen

bench: ## Run the bench suite once and print results to stdout
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...

release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
	@command -v goreleaser >/dev/null 2>&1 || { ...; exit 1; }
	goreleaser release --snapshot --clean --skip=sign
```

**Existing rename rows** (lines 1-7):
```makefile
.PHONY: build clean ...

BINARY=serena                   # → helix
GO=go

build:
	$(GO) build -o $(BINARY) ./cmd/serena    # → ./cmd/helix

clean:
	rm -f $(BINARY)
```

**New targets** (D-13 + Phase 50 D-07 self-doc style):
```makefile
embed-pubkey: ## Sync repo-root minisign.pub into internal/upgrade/minisign.pub before build
	@cp minisign.pub internal/upgrade/minisign.pub

verify-embed-pubkey: ## CI gate: assert internal/upgrade/minisign.pub matches repo-root copy byte-for-byte
	@cmp -s minisign.pub internal/upgrade/minisign.pub || { \
	  echo "internal/upgrade/minisign.pub drift; run: make embed-pubkey"; exit 1; }

build: embed-pubkey            # add the dependency so `make build` always re-syncs first
	$(GO) build -o $(BINARY) ./cmd/helix
```

---

## Shared Patterns

### Typed error taxonomy

**Source:** `internal/errors/kinds.go` lines 14-21
**Apply to:** All public-facing error returns in `internal/upgrade/`, especially the `Upgrade()` and `Update()` entrypoints

```go
// Existing taxonomy:
const (
    NotFound    Kind = "not_found"
    InvalidArgs Kind = "invalid_args"
    NoWorkspace Kind = "no_workspace"
    Unsupported Kind = "unsupported"
    Internal    Kind = "internal"
    CircuitOpen Kind = "circuit_open"
    Timeout     Kind = "timeout"
)
```

**Use in upgrade:**
- `serr.New(serr.InvalidArgs, "version flag must be a valid semver tag")` for malformed `--version`
- `serr.New(serr.NotFound, "no GitHub release matching constraints").WithDetail("...")` for empty release list
- `serr.Wrap(serr.Internal, "minisign verification failed", err)` for signature failure (per RESEARCH.md Pitfall 4 — single message regardless of cause)
- `serr.New(serr.Timeout, "github API rate limited").WithDetail("retry after Xm; or set GITHUB_TOKEN")` for 429 (RESEARCH.md Pitfall 6)
- `serr.New(serr.Unsupported, "no release archive for platform XYZ")` for asset-name miss

**Import alias** (from package doc comment lines 3-5 of `internal/errors/kinds.go`):
```go
import serr "github.com/agenthands/helix/internal/errors"
```

### Module-path import alias

**Source:** existing package doc convention (`internal/errors/kinds.go` lines 1-6)

After Wave 1 module rename, every import line shifts:
```go
// before
import "github.com/postfix/serena/internal/cli"
// after
import "github.com/agenthands/helix/internal/cli"
```

Driver: `go mod edit -module github.com/agenthands/helix && find . -name '*.go' -exec sed -i '' 's|github.com/postfix/serena|github.com/agenthands/helix|g' {} + && gofmt -w . && go build ./...`. Per RESEARCH.md Open Question 1, the planner MUST surface this 392-line edit and confirm before locking the wave plan.

### Cobra subcommand registration

**Source:** `internal/cli/root.go` lines 49-54
**Apply to:** `update`, `upgrade` subcommands

```go
// existing pattern in NewRootCommand():
rootCmd.AddCommand(newSetupCommand())
rootCmd.AddCommand(newStatusCommand())
rootCmd.AddCommand(newActivateCommand())
rootCmd.AddCommand(newDeactivateCommand())
rootCmd.AddCommand(newNudgeCommand())
```

Append `newUpdateCommand()` and `newUpgradeCommand()` after `newNudgeCommand()`.

### `SilenceUsage`/`SilenceErrors` on cobra commands

**Source:** `internal/cli/setup.go` lines 27-28, `internal/cli/status.go` lines 28-29

Every existing subcommand sets both. Apply to `update`/`upgrade` as well — keeps cobra from spamming the usage block on a runtime error (RESEARCH.md Pitfall 6 in `internal/cli/root.go` references this convention).

### Path string rename surface

**Source:** `internal/config/defaults.go` line 17, line 24

```go
"logging.dir":               filepath.Join(homeDir, ".serena", "logs"),  // → ".helix"
"observability.service_name": "serena",                                   // → "helix"
```

There are 54 `.serena` path references in Go source (RESEARCH.md Symbol Counts). Apply via `find . -name '*.go' -exec sed -i '' 's|"\.serena|".helix|g; s|"~/\.serena|"~/.helix|g' {} +` then manual review for false positives (e.g., the `legacy/` Python references must not be touched per CLAUDE.md "Repo: Same repo, Python in legacy/").

### Build-tag pair signature assertion (Phase 51.1 WR-03 precedent)

**Source:** Phase 51.1 commit `48550030 fix(51.1): WR-03 add compile-time signature assertions for cgo/!cgo stubs`

**Apply to:** `internal/upgrade/swap_unix.go` and `swap_windows.go` — add a compile-time assertion in a build-tag-free file (e.g., `internal/upgrade/swap_assert.go`) that takes the function signatures of both `swap` and `relaunch` to ensure the cgo/!cgo and unix/windows variants stay in lockstep. The exact pattern from 51.1 is:

```go
// (no build tag; compiles on all platforms)
package upgrade

// Compile-time check: both swap_unix.go and swap_windows.go must export the same `swap` signature.
var _ = swap   // function reference forces compile-time signature equality across variants
var _ = relaunch
```

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/upgrade/archive.go` | utility (tar.gz extract) | transform | The codebase has no archive-extraction code. Use stdlib `archive/tar` + `compress/gzip` per RESEARCH.md Pattern 7 (no project pattern exists). |
| `internal/upgrade/semver.go` | utility (version compare) | transform | The codebase has no semver-aware code (the existing `2.0.0-dev` is a literal string compare). Use `golang.org/x/mod/semver` per RESEARCH.md Pattern 6. |
| `EMBED-AUDIT.md` | docs (manifest) | n/a | Free-form artifact; no template exists. Structure follows D-14: classify each finding as embedded / external-by-design / gap. |

## Metadata

**Analog search scope:** `cmd/`, `internal/cli/`, `internal/langregistry/`, `internal/forwarder/`, `internal/profile/`, `internal/errors/`, `internal/mcp/`, `internal/config/`, `internal/daemon/`, `test/`, `Makefile`, `.goreleaser.yaml`, `go.mod`
**Files scanned:** ~60 (focused on the upgrade-relevant subtrees and the rename mechanical surface)
**Pattern extraction date:** 2026-04-29

## PATTERN MAPPING COMPLETE

**Phase:** 52 - packaging-distribution-channels
**Files classified:** 21 (9 new in `internal/upgrade/`, 2 new cobra subcommands, ~10 modified by rename)
**Analogs found:** 18 / 21

### Coverage
- Files with exact analog: 13 (most are self-rename; 5 use sibling subcommand or build-tag-pair patterns)
- Files with role-match analog: 5 (HTTP client, stage dir, httptest, permission probe, daemon detect)
- Files with no analog: 3 (archive extract, semver, EMBED-AUDIT.md)

### Key Patterns Identified
- **Build-tag pair convention:** `internal/forwarder/dial_{unix,windows}.go` is the exact template for `internal/upgrade/swap_{unix,windows}.go` — same headers, same package, same imports of `syscall` + `golang.org/x/sys/windows`.
- **Single-file embed:** `internal/profile/embed.go` shows the `import "embed"` + `//go:embed` directive style for `internal/upgrade/pubkey.go`. The package-local file (per D-13's build-time copy) avoids the invalid `..` traversal pitfall.
- **Cobra subcommand symmetry:** `internal/cli/{setup,status,activate,deactivate,nudge}.go` are five concrete templates for `update`/`upgrade` — `Use`, `Short`, `Long`, `RunE`, `SilenceUsage`, `SilenceErrors`, then `cmd.Flags().Bool(...)`.
- **HTTP download flow:** `internal/langregistry/installer.go` lines 184-209 is the closest analog for the upgrade GitHub-API + archive-download flow; differences are User-Agent header, GitHub-specific Accept header, minisign-instead-of-sha256 verification, and sibling-of-install-path stage dir.
- **Typed error taxonomy:** `internal/errors/kinds.go` defines the exact 7-kind set (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout) referenced in CONTEXT.md — the upgrade subcommand returns these kinds at every error boundary using the `serr` alias convention.
- **Makefile self-doc:** the `## help` annotation style (Phase 50 D-07) is established by `docs:`, `bench:`, `release-snapshot:` — apply to `embed-pubkey:` and `verify-embed-pubkey:`.

### File Created
`.planning/phases/52-packaging-distribution-channels/52-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. Planner can reference analog file paths and concrete excerpts directly in PLAN.md actions; all upgrade-package files have either an exact analog (build-tag pair, embed, cobra subcommand) or a partial analog (HTTP client, permission probe, httptest) plus the RESEARCH.md library-level pattern for the three no-analog cases (archive, semver, EMBED-AUDIT.md).
