# Phase 52: packaging-distribution-channels - Research

**Researched:** 2026-04-29
**Domain:** Go binary rename + in-binary self-upgrade (GitHub Releases + minisign)
**Confidence:** HIGH on rename mechanics, HIGH on goreleaser/embed/minisign, MEDIUM on Windows-specific atomic-swap edge cases (locked-binary semantics differ across Windows versions)

## Summary

Phase 52 has been rescoped during discuss-phase: the original PKG-02/03/04 (brew/scoop/native-Linux) channels are deferred. The actual work is (a) a hard-cut binary + product rename `serena` → `helix` at v1.9 with no migration shim, (b) an in-binary self-upgrade subcommand pair (`helix update` read-only check, `helix upgrade` install), (c) an embed audit producing `EMBED-AUDIT.md`, (d) embedding `minisign.pub`, and (e) bookkeeping edits to ROADMAP.md and REQUIREMENTS.md.

Three findings change the planner's mental model from CONTEXT.md:

1. **`go.mod` is `github.com/postfix/serena`, not `github.com/agenthands/helix`** [VERIFIED: head go.mod]. CONTEXT.md D-01 asserts the module path is unchanged — that is incorrect. There are 392 import-line references to `github.com/postfix/serena` across the codebase that must be rewritten. This is the single largest mechanical surface in the phase.
2. **`//go:embed ../../minisign.pub` is not valid Go** [VERIFIED: pkg.go.dev/embed; golang/go#46056]. The embed directive forbids `..` and forbids escaping the package directory. CONTEXT.md D-13's "or absolute-from-module-root equivalent" doesn't exist either. The canonical fixes: (a) keep `minisign.pub` only at repo root and embed from `cmd/helix/main.go` (or a top-level `internal/release` package adjacent to it), or (b) check in a copy at `internal/upgrade/minisign.pub` kept in sync via `go generate` or Makefile. Recommendation: option (b) — explicit per-package copy with a Makefile pre-build step that fails the build on drift, so the single source of truth at repo root continues to satisfy INSTALL.md while the embed lives next to the verifier.
3. **Symbol counts**: 392 `github.com/postfix/serena` import lines, 21 `SERENA_` env-var references in Go, 54 `.serena` path references in Go, plus all docs. The rename is a multi-wave operation, not a single-commit sed.

**Primary recommendation:** Stage the rename across **three sequential waves** (1: module-path + cmd-dir + goreleaser; 2: env-vars + config dirs + MCP server name; 3: docs + CHANGELOG), then a **fourth parallel wave** for the upgrade subcommand + embed audit + ROADMAP/REQUIREMENTS edits. Use `gopls rename` or `go-mod-edit` + `gofmt` for the import path flip, not freeform sed, because every Go file's import block participates.

## User Constraints (from CONTEXT.md)

### Locked Decisions

D-01: Binary renamed `serena` → `helix`. Hard cut at v1.9. Archive naming flips. `cmd/serena/main.go` → `cmd/helix/main.go`. Go module path is **stated as unchanged** in CONTEXT.md but is in fact `github.com/postfix/serena` — the planner must surface this mismatch and confirm with the user before locking the wave plan (see Open Questions).

D-02: Full env-var + config-path rename, no transparent migration. `SERENA_*` → `HELIX_*`. `~/.serena/` → `~/.helix/`; `.serena/project.yml` → `.helix/project.yml`; `.serena/memories/` → `.helix/memories/`. No fallback. Existing users re-onboard.

D-03: Product name is "Helix" in PROJECT.md, README.md, USAGE.md, INSTALL.md, CHANGELOG.md, log lines, and `helix --version` output.

D-04: No backwards-compat symlink or wrapper. Archive contains only the `helix` binary.

D-05: MCP server registration name flips to `helix`. Setup CLI may detect the old `serena` registration and print a removal hint (planner discretion whether to ship in v1.9).

D-06: Two distinct verbs. `helix update` is read-only (GitHub API check + release notes); `helix upgrade` is install. They are not aliases.

D-07: Upgrade flow: GitHub API → download → minisign verify → extract → atomic swap → `os.Exec` same-args. Inode-replace on Linux/macOS; rename-current-to-`.old` on Windows. After successful swap, re-launch is mandatory.

D-08: Restart via `os.Exec` same args (minus the `upgrade` verb). Daemon-aware: if invoked from inside the daemon process, do NOT exec — print "restart the daemon manually" and exit cleanly.

D-09: Permission probe BEFORE network I/O. If install path unwritable, print sudo hint with exact re-invocation and exit. No internal sudo prompt.

D-10: Hard-refuse downgrades, no `--force-downgrade` flag. Exit 0 with "already up to date" message on `latest <= current`.

D-11: Default upgrade target = latest stable only. `v*-rc*` / `v*-beta*` / `v*-alpha*` skipped unless `--prerelease` is set.

D-12: Flags shipped in v1.9: `--prerelease`, `--version vX.Y.Z`, `--check`, `--dry-run`. `--version` always wins over `--prerelease`.

D-13: Embed `minisign.pub` via `//go:embed`. Single Go file. Build fails if file is missing. Same file at repo root continues to be served for INSTALL.md curl-fetch. Hardcoded `const` strings explicitly NOT used. **Note:** CONTEXT.md suggests `//go:embed ../../minisign.pub` — that exact path is invalid Go (see Common Pitfalls below). Recommendation: build-time copy into `internal/upgrade/minisign.pub` with a Makefile guard. Final location is Claude's discretion per CONTEXT.md.

D-14: Embed audit produces `EMBED-AUDIT.md`. Manifest classifies every runtime asset as **embedded** / **external-by-design** / **gap**. Gaps closed in this phase.

D-15: LS binaries stay runtime-downloaded by the three-tier installer. EMBED-AUDIT.md classifies every LS as **external-by-design**.

### Claude's Discretion

- Stage-dir path (`os.TempDir()` vs. `os.UserCacheDir()/helix/upgrade-stage/` vs. next to running binary).
- Exact GitHub Releases endpoint pattern.
- Daemon-detection heuristic for the "do not exec" branch.
- Wording of error messages and the sudo hint.
- Exact location of `//go:embed minisign.pub`.
- Exit codes for `--dry-run`.
- Whether `helix update` and `helix upgrade --check` print identical output.
- Whether to do the rename in one atomic commit or stage across plans.
- "Previous serena MCP registration detected" nudge — ship in v1.9 or defer.
- Whether `cmd/helix/main.go` is bit-identical to `cmd/serena/main.go` minus paths.

### Deferred Ideas (OUT OF SCOPE)

- PKG-02 Homebrew tap, PKG-03 Scoop bucket, PKG-04 native Linux package — explicitly deferred.
- PKG-DEFER-02 Docker / container images — already deferred project-wide.
- Curl-pipe-sh installer script — explicitly rejected.
- `~/.serena/` migration tool / fallback shim — explicitly rejected; hard cut.
- Embedding LS binaries — rejected for archive-size reasons.
- Auto-rotation of `minisign.pub` via embedded fingerprint check — deferred.
- MCP-side automatic re-registration removal — Claude's discretion.
- `serena` shell-completion fallback — defer to planner discretion.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PKG-02 | Homebrew tap | DEFERRED — invert in REQUIREMENTS.md (see ROADMAP.md edits below). Research support: REQUIREMENTS.md table edit pattern (existing rows show "Phase 52" → swap to "Deferred" with a 1-line rationale). |
| PKG-03 | Scoop bucket | DEFERRED — same posture. |
| PKG-04 | Native Linux package | DEFERRED — same posture. |

The phase's *actual* deliverables (rename + self-upgrade + embed audit) cover requirements that don't exist in REQUIREMENTS.md yet. The planner must add new REQ-IDs in REQUIREMENTS.md to make the rename + self-upgrade traceable. Suggested IDs:

| Suggested ID | Description | Phase |
|--------------|-------------|-------|
| `PKG-05` | Binary + product renamed `serena` → `helix` at v1.9 | Phase 52 |
| `PKG-06` | In-binary self-upgrade (`helix update` / `helix upgrade`) with minisign verification, atomic swap, downgrade refusal | Phase 52 |
| `PKG-07` | `EMBED-AUDIT.md` manifest produced; `minisign.pub` embedded via `//go:embed` | Phase 52 |

The "Auto-update mechanism inside the binary | Out of scope" row in REQUIREMENTS.md is to be **inverted**: removed from Out-of-Scope, added as PKG-06 above. The rationale "Package managers (brew/scoop/apt) own updates" no longer holds because brew/scoop/apt are themselves out of v1.9 scope.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Binary entrypoint (cobra root) | Client / CLI | — | `cmd/helix/main.go` is the single Go entrypoint; cobra root mounted from `internal/cli/`. |
| Module-path rename | Build / Toolchain | — | `go.mod` + every import statement; cross-cuts every layer but lives in build-toolchain concerns. |
| Env-var / config-path read | Config layer | — | `internal/config/`, `internal/skill/`, `internal/memory/`. Read once at boot; not user-tier. |
| MCP server name registration | Client setup CLI | — | `internal/cli/setup_clients.go` — invokes `claude mcp add-json`, etc. |
| GitHub Releases API client | New `internal/upgrade/` | — | Backend HTTP client; no LS interaction. |
| Minisign signature verification | New `internal/upgrade/` | — | Pure-Go crypto using `golang.org/x/crypto/ed25519` via `jedisct1/go-minisign`. |
| Archive download + extraction | New `internal/upgrade/` | — | `archive/tar` + `compress/gzip` from stdlib. |
| Atomic binary swap | New `internal/upgrade/` (platform-split) | OS / Filesystem | Uses kernel rename(2) on Unix, "rename to .old + write new" on Windows. |
| `os.Exec` re-launch | New `internal/upgrade/` (platform-split) | OS | `syscall.Exec` on Linux/macOS replaces process image; `os.StartProcess` + `os.Exit(0)` on Windows. |
| Daemon-detect heuristic | Daemon supervisor + new `internal/upgrade/` | Env | Env var set by `internal/daemon/daemon.go` when spawning workers; upgrade subcommand reads it. |
| `//go:embed minisign.pub` | New `internal/upgrade/` | — | Single source of truth at repo root, copied/synced into `internal/upgrade/` at build time. |
| Embed audit manifest | Phase artifact (`EMBED-AUDIT.md`) | — | Living markdown file; not code. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/jedisct1/go-minisign` | latest (verify before locking; min Go 1.23.4) | Pure-Go minisign signature verification | Upstream library by minisign's author; pure Go (no CGO); only deps are `golang.org/x/crypto` + `golang.org/x/sys`. [VERIFIED: pkg.go.dev/github.com/jedisct1/go-minisign + their go.mod] |
| `net/http` (stdlib) | Go 1.25.1 (project's pinned toolchain) | GitHub Releases API + asset download | We need exactly 1-2 endpoints. Pulling `google/go-github` is overkill and adds ~MB of unused codegen. [ASSUMED: project preference for minimal deps] |
| `archive/tar` + `compress/gzip` (stdlib) | Go 1.25.1 | Extract `.tar.gz` archives on Linux/macOS | Standard library. |
| `archive/zip` (stdlib) | Go 1.25.1 | Extract Windows release archives **if goreleaser produces .zip on Windows** | The current `.goreleaser.yaml` produces `.tar.gz` for all platforms (line 35: `formats: ["tar.gz"]`). Windows users get tar.gz too — INSTALL.md confirms "Modern Windows (10 1803+) ships tar in System32". Archive extraction is uniform across platforms. [VERIFIED: `.goreleaser.yaml`:35 + INSTALL.md:54] |
| `golang.org/x/mod/semver` | latest indirect (already in many Go projects) | Semver parse + compare for downgrade refusal | Stdlib's `runtime.Version()` is for the Go toolchain, not for app versions. `golang.org/x/mod/semver` is the canonical lightweight semver comparer; `Masterminds/semver` is heavier and not needed. [CITED: pkg.go.dev/golang.org/x/mod/semver] |
| `github.com/spf13/cobra` v1.9.1 | already in tree | Subcommand wiring | Project standard; `update` and `upgrade` register exactly like `setup`, `status`. [VERIFIED: internal/cli/root.go imports + project version pin in CLAUDE.md] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os/exec` (stdlib) | Go 1.25.1 | Subprocess for `claude mcp remove serena` cleanup hint (D-05) | Only if D-05's "previous serena registration detected" nudge is shipped. |
| `syscall` (stdlib) | Go 1.25.1 | `syscall.Exec` on Linux/macOS for in-place re-launch | Linux/macOS implementation file. |
| `net/http/httptest` (stdlib) | Go 1.25.1 | Test fixture: stub GitHub Releases API server | All upgrade-subcommand tests. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `jedisct1/go-minisign` | shell out to `minisign -V` binary | Requires user to have minisign installed at upgrade time. Phase 51's signing **does** shell to the binary (CI-side), but on user machines this would mean every `helix upgrade` requires `minisign` in PATH — defeats "single binary" promise. [VERIFIED: .goreleaser.yaml:51-63 signs.cmd: minisign — that's CI-only.] |
| `inconshreveable/go-update` | hand-rolled atomic-swap | `go-update` is feature-rich (binary patching, etc.) but the original repo is unmaintained (last commits 2018-ish). `minio/selfupdate` is the maintained fork; `openstor/selfupdate` is a further fork. [VERIFIED: search results — minio/selfupdate is "based on original work at github.com/inconshreveable/go-update, modified for the needs within the MinIO project"] We need ~100 LOC of swap logic; reading a maintained library's source for the Windows quirk is the cheapest approach (recommend: hand-roll using `minio/selfupdate`'s `apply.go` as the reference, NOT depend on it — extra dep for a small surface adds churn). |
| `google/go-github` API client | hand-rolled `net/http` | go-github bundles the entire GitHub API (~10MB of generated code) for ~2 endpoints. Hand-rolled saves the dependency cost. [ASSUMED: dep-cost preference matches the project's "single Go binary" stance] |
| `Masterminds/semver` | `golang.org/x/mod/semver` | x/mod is already an indirect dep in many Go projects via toolchain bits; Masterminds adds a separate dependency. [CITED: pkg.go.dev/golang.org/x/mod/semver] |

**Installation:**

```bash
go get github.com/jedisct1/go-minisign@latest
go get golang.org/x/mod/semver@latest
```

**Version verification:** Before locking, run on a fresh checkout:

```bash
go list -m -versions github.com/jedisct1/go-minisign | tail -5
go list -m -versions golang.org/x/mod | tail -5
```

The planner records the exact pinned versions in PLAN.md. The upstream `jedisct1/go-minisign` go.mod requires Go ≥ 1.23.4 [VERIFIED: master/go.mod] which is satisfied by the project's Go 1.25.1.

## Architecture Patterns

### System Architecture Diagram

```
                          ┌────────────────────────────┐
                          │   user runs `helix upgrade`│
                          └──────────────┬─────────────┘
                                         │
                                         ▼
                  ┌──────────────────────────────────────┐
                  │ 1. Daemon-detect (env-var probe)     │
                  │    HELIX_RUNNING_AS_DAEMON==1 ?      │
                  └──────────────┬───────────┬───────────┘
                              YES│           │NO
                                 ▼           │
                     ┌────────────────────┐  │
                     │ Print "restart     │  │
                     │ daemon manually"   │  │
                     │ exit 0             │  │
                     └────────────────────┘  │
                                             ▼
                  ┌────────────────────────────────────┐
                  │ 2. Permission probe (D-09)         │
                  │    can_write(os.Executable()) ?    │
                  └──────────────┬───────────┬─────────┘
                              NO │           │ YES
                                 ▼           │
                ┌─────────────────────────┐  │
                │ Print sudo hint, exit 1 │  │
                └─────────────────────────┘  │
                                             ▼
                  ┌────────────────────────────────────┐
                  │ 3. GitHub Releases API check       │
                  │    (--version vX → /releases/tags/ │
                  │     /vX, else /latest, else        │
                  │     filter /releases for prerelease)│
                  └──────────────┬─────────────────────┘
                                 ▼
                  ┌────────────────────────────────────┐
                  │ 4. Semver compare (D-10)           │
                  │    latest <= current ?             │
                  └──────────────┬───────────┬─────────┘
                              YES│           │ NO
                                 ▼           │
                ┌─────────────────────────┐  │
                │ Print "up to date",     │  │
                │ exit 0                  │  │
                └─────────────────────────┘  │
                                             ▼
                  ┌────────────────────────────────────┐
                  │ 5. Download archive + .minisig +   │
                  │    checksums.txt to stage dir      │
                  │    (recommend: os.TempDir())       │
                  └──────────────┬─────────────────────┘
                                 ▼
                  ┌────────────────────────────────────┐
                  │ 6. minisign verify (embedded pub)  │
                  │    - verify checksums.txt          │
                  │    - verify archive .minisig       │
                  │    - cross-check sha256 in         │
                  │      checksums.txt (defense in     │
                  │      depth, mirrors INSTALL.md)    │
                  └──────────────┬─────────────────────┘
                                 │
                              FAIL│   PASS │
                                 ▼          ▼
                ┌──────────────────────┐  ┌──────────────────┐
                │ exit 1, leave stage  │  │ 7. Extract tar.gz│
                │ dir for inspection   │  │    to stage/bin  │
                └──────────────────────┘  └────────┬─────────┘
                                                   ▼
                          ┌────────────────────────────────────┐
                          │ 8. --dry-run? exit 0 here          │
                          └──────────────┬─────────────────────┘
                                         ▼
                          ┌────────────────────────────────────┐
                          │ 9. Atomic swap (platform-split)    │
                          │  Linux/macOS: os.Rename inode-     │
                          │   replace; running process keeps   │
                          │   the old inode via open FD until  │
                          │   exit                             │
                          │  Windows: rename current to .old;  │
                          │   write new to original path       │
                          └──────────────┬─────────────────────┘
                                         ▼
                          ┌────────────────────────────────────┐
                          │ 10. os.Exec same args minus "upgrade"│
                          │  Linux/macOS: syscall.Exec replaces│
                          │   process image                    │
                          │  Windows: os.StartProcess + os.Exit(0)│
                          └────────────────────────────────────┘
```

### Recommended Project Structure

```
cmd/
└── helix/                           # renamed from cmd/serena/
    └── main.go                      # cobra root invocation; minimal change
internal/
└── upgrade/                         # NEW package
    ├── upgrade.go                   # public Upgrade(opts) + Update(opts) entrypoints
    ├── github.go                    # GitHub Releases API client (net/http; ~150 LOC)
    ├── verify.go                    # minisign verifier wrapper around go-minisign
    ├── archive.go                   # tar.gz extract helpers
    ├── semver.go                    # version compare + prerelease filter
    ├── permission.go                # writability probe + sudo-hint formatter
    ├── stage.go                     # stage-dir lifecycle (create/cleanup)
    ├── swap_unix.go                 # //go:build !windows — inode-replace + syscall.Exec
    ├── swap_windows.go              # //go:build windows — rename-to-.old + StartProcess
    ├── daemon_detect.go             # env-var probe; daemon supervisor sets it
    ├── pubkey.go                    # //go:embed minisign.pub  (file copied at build time)
    ├── minisign.pub                 # build artifact, kept in sync with repo-root by Makefile
    ├── upgrade_test.go              # unit tests w/ httptest stub + test keypair
    └── testdata/
        ├── test_minisign.pub        # test-only keypair
        ├── test_minisign.key        # test-only signing key (passwordless OK in tests)
        └── fixtures/                # synthetic archives + signatures
```

Adding `update` and `upgrade` to `internal/cli/root.go`:

```go
// in NewRootCommand()
rootCmd.AddCommand(newUpdateCommand())   // NEW
rootCmd.AddCommand(newUpgradeCommand())  // NEW
```

### Pattern 1: GitHub Releases API client (hand-rolled)

**What:** Three endpoints, plain `net/http`, no third-party client.
**When to use:** Always — only 2-3 endpoints needed; vendored go-github is overkill.

```go
// Source: docs.github.com/en/rest/releases/releases (HIGH confidence)
const (
    apiBase   = "https://api.github.com"
    userAgent = "helix-upgrade/" + Version  // GitHub recommends a UA; without one, anonymous requests are rate-limited harder
    accept    = "application/vnd.github+json"
    apiVer    = "2026-03-10" // X-GitHub-Api-Version header
)

type Release struct {
    TagName     string  `json:"tag_name"`
    Name        string  `json:"name"`
    Body        string  `json:"body"`        // raw markdown release notes
    Prerelease  bool    `json:"prerelease"`
    Draft       bool    `json:"draft"`
    Assets      []Asset `json:"assets"`
}

type Asset struct {
    Name               string `json:"name"`
    BrowserDownloadURL string `json:"browser_download_url"`
}

func fetchLatest(ctx context.Context, owner, repo string) (*Release, error) {
    return fetchJSON[Release](ctx, fmt.Sprintf("%s/repos/%s/%s/releases/latest", apiBase, owner, repo))
}

func fetchAll(ctx context.Context, owner, repo string) ([]Release, error) {
    // /releases endpoint includes prereleases (and drafts for users with push access)
    // Single page is fine for v1.9 — pagination support is a v1.10 polish item.
    return fetchJSON[[]Release](ctx, fmt.Sprintf("%s/repos/%s/%s/releases?per_page=30", apiBase, owner, repo))
}
```

Rate limit: 60 req/hr/IP unauthenticated [VERIFIED: docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api]. `helix update` makes 1 request; safe. `helix upgrade --prerelease` makes 1 request to `/releases`. The User-Agent header is recommended (and the docs strongly suggest it for tracking abuse). Inspect `X-RateLimit-Remaining` and surface a friendly message if exhausted.

### Pattern 2: minisign verification with go-minisign

**What:** Verify the embedded `minisign.pub` against the downloaded `.minisig`.
**When to use:** Always — every archive download is verified before extraction; double-verify against `checksums.txt` mirroring the INSTALL.md ceremony.

```go
// Source: pkg.go.dev/github.com/jedisct1/go-minisign (HIGH confidence)
import "github.com/jedisct1/go-minisign"

//go:embed minisign.pub
var pubKeyBytes []byte

func VerifyArchive(archivePath, sigPath string) error {
    pub, err := minisign.DecodePublicKey(string(pubKeyBytes))
    if err != nil {
        return fmt.Errorf("decoding embedded minisign.pub: %w", err)
    }
    sig, err := minisign.NewSignatureFromFile(sigPath)
    if err != nil {
        return fmt.Errorf("parsing %s: %w", sigPath, err)
    }
    ok, err := pub.VerifyFromFile(archivePath, sig)
    if err != nil || !ok {
        return fmt.Errorf("signature verification FAILED: %w", err)
    }
    return nil
}
```

Note the API: `DecodePublicKey` for a multi-line formatted key (matches `minisign.pub` format with `untrusted comment:` line); `NewSignatureFromFile` for the `.minisig` file; `(*PublicKey).VerifyFromFile(path, sig)` returns `(bool, error)`. **Always check both** the boolean and the error per Pitfall 4 below.

### Pattern 3: Atomic swap on Linux/macOS via inode-replace

**What:** `os.Rename` on the same filesystem performs an atomic POSIX `rename(2)` syscall; the running process keeps the old inode alive via its open executable file descriptor until exit.
**When to use:** Default path on `!windows`.

```go
//go:build !windows
package upgrade

import (
    "os"
    "syscall"
)

func swap(currentPath, newPath string) error {
    // os.Rename is atomic on the same filesystem (POSIX rename(2)).
    // The running process keeps its old inode through its exec handle.
    return os.Rename(newPath, currentPath)
}

func relaunch(binPath string, args, env []string) error {
    // Replace the current process image entirely. No fork.
    return syscall.Exec(binPath, args, env)
}
```

Critical: the new binary must be on the SAME filesystem as the install path or `os.Rename` returns `EXDEV` and falls back to copy-then-unlink (NOT atomic). The stage dir choice (`os.TempDir()` is often `/tmp` which is *not* on the same fs as `/usr/local/bin` on many distros) matters here. Recommendation: stage to a sibling temp dir of the install path (`filepath.Join(filepath.Dir(installPath), ".helix-upgrade-stage")`) so rename is always cross-fs-safe. The permission probe in D-09 already requires write to that directory.

### Pattern 4: Atomic swap on Windows via "rename to .old"

**What:** Windows refuses to rename a running executable. The canonical workaround is `MoveFileEx(current → current.old)` then write new to `current`, then `os.Exec` (no inode-magic; the .old file is leaked but harmless, removable on next launch).
**When to use:** `windows` only.

```go
//go:build windows
package upgrade

import (
    "os"
    "os/exec"
    "path/filepath"
)

func swap(currentPath, newPath string) error {
    oldPath := currentPath + ".old"
    // Best-effort cleanup of any prior leak from a previous upgrade
    _ = os.Remove(oldPath)
    if err := os.Rename(currentPath, oldPath); err != nil {
        return fmt.Errorf("renaming current to .old: %w", err)
    }
    if err := os.Rename(newPath, currentPath); err != nil {
        // Roll back the .old rename so the user isn't left without a binary
        _ = os.Rename(oldPath, currentPath)
        return fmt.Errorf("moving new binary into place: %w", err)
    }
    return nil
}

func relaunch(binPath string, args, env []string) error {
    // Windows has no exec-without-fork. Spawn + exit.
    cmd := exec.Command(binPath, args[1:]...)
    cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
    cmd.Env = env
    if err := cmd.Start(); err != nil {
        return err
    }
    os.Exit(0)
    return nil // unreachable
}
```

The .old leak is a known property of Windows self-update — `inconshreveable/go-update` "makes the old file hidden instead" [VERIFIED: search result on inconshreveable/go-update Windows section]. We do not need that polish in v1.9; an explicit "leftover .old file from previous upgrade can be deleted manually" line in CHANGELOG is sufficient. A future v1.10 polish phase can mark the file hidden via `attrib +H` or the syscall equivalent.

### Pattern 5: Daemon-detect via env var

**What:** The daemon supervisor sets `HELIX_RUNNING_AS_DAEMON=1` when spawning child processes. The upgrade subcommand checks for it.
**When to use:** D-08's "do not exec, print manual hint" branch.

```go
// In internal/daemon/daemon.go where worker subprocesses are spawned:
cmd.Env = append(os.Environ(), "HELIX_RUNNING_AS_DAEMON=1")

// In internal/upgrade/daemon_detect.go:
func RunningInDaemon() bool {
    return os.Getenv("HELIX_RUNNING_AS_DAEMON") == "1"
}
```

Alternatives considered:
- `/proc/self/status` — Linux only, not portable. Reject.
- Parent-PID match against a recorded pidfile — fragile; pidfile may be stale, parent may have been re-parented to init, daemon may not have written one yet. Reject.
- A dedicated `--from-daemon` flag — explicit but couples the daemon's spawn logic to the upgrade flag set forever. Env-var is cleaner.

### Pattern 6: Semver compare + prerelease filter

```go
// Source: pkg.go.dev/golang.org/x/mod/semver (HIGH confidence)
import "golang.org/x/mod/semver"

// canonicalize accepts "v1.9.0" or "1.9.0"; semver requires the leading 'v'.
func canonical(v string) string {
    if !strings.HasPrefix(v, "v") {
        return "v" + v
    }
    return v
}

// IsDowngrade returns true if target <= current.
func IsDowngrade(current, target string) bool {
    return semver.Compare(canonical(target), canonical(current)) <= 0
}

// IsPrerelease returns true if the tag has a prerelease segment (e.g. "v1.9.0-rc1").
func IsPrerelease(tag string) bool {
    return semver.Prerelease(canonical(tag)) != ""
}
```

The function is cheap, has no transitive deps beyond what `golang.org/x/mod` already pulls, and `semver.Compare` returns -1 / 0 / +1 — making the `<= 0` check unambiguous.

### Anti-Patterns to Avoid

- **Hand-rolled regex for version parsing** — `golang.org/x/mod/semver` exists for exactly this; do not parse `v1.2.3-rc1` with regex.
- **Shelling out to `minisign -V`** — defeats the "single binary" promise; users would need minisign installed.
- **Vendoring `google/go-github`** — adds ~MB of generated code for 2-3 endpoints.
- **Embedding `minisign.pub` via const string** — explicitly rejected by D-13 (manual sync = drift bug waiting to happen).
- **Fallback `~/.serena/` reads** — explicitly rejected by D-02 (clean break).
- **Single 5000-line commit for the rename** — unbisectable; stage across sequential waves.
- **`os.Rename` from `/tmp` to `/usr/local/bin`** — different filesystems; falls back to non-atomic copy. Stage to a sibling dir of the install path.
- **Forgetting to set `cmd.Stdin/Stdout/Stderr` on Windows relaunch** — the new process loses I/O and appears "broken".

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Minisign verification | Pure-Go ed25519 + minisign-format parser | `github.com/jedisct1/go-minisign` | Author of minisign maintains the canonical Go verifier; getting the format details right is fiddly (trusted-comment global signature). |
| Semver compare | Regex + custom parser | `golang.org/x/mod/semver` | Edge cases: prerelease tags, build metadata, leading 'v', `+build.123` suffixes. |
| GitHub Releases response parsing | Reflection-based JSON parser | `encoding/json` + a typed struct | We need 4 fields per release; standard library is enough. |
| Tar/gzip extraction | Custom decompressor | `archive/tar` + `compress/gzip` | Stdlib. |
| Atomic file rename | `os.Open` + `io.Copy` + `os.Remove` | `os.Rename` | Loses POSIX atomicity guarantees; intermediate state is visible. |
| Process replacement | `os.StartProcess` everywhere | `syscall.Exec` on Unix, `os.StartProcess + os.Exit(0)` on Windows | Linux/macOS users lose the "same shell, new version" UX if you fork. |

**Key insight:** All the hard parts of self-upgrade are well-trodden. The places where careful research pays off are: (1) minisign's trusted-comment global signature semantics, (2) the same-filesystem rule for atomic rename, (3) Windows's "rename a running .exe" prohibition. Use the libraries and patterns named above; do not reinvent.

## Runtime State Inventory

This is a **rename phase**. The CONTEXT.md hard-cut policy (D-02: "no fallback") means no data migration is required, but the runtime-state inventory is still meaningful for "what breaks visibly when users upgrade".

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **MCP server registrations**: Existing `serena` entries in client config files (`.mcp.json`, `mcp.json`, `claude_desktop_config.json`, OpenCode `opencode.json`, Cursor `.cursor/mcp.json`, JetBrains `.junie/mcp/mcp.json`) — registered by previous `serena setup`. **Memory store**: SQLite FTS5 index lives at `~/.serena/memories/` and `<project>/.serena/memories/` (per `internal/memory/store.go:24-25`). **Logs**: `~/.serena/logs/` (per `internal/config/defaults.go:17`). **Tagcache**: SQLite tag cache (mtime-keyed) — path discovery still in `~/.serena/...`. **Bin**: managed LS installs at `~/.serena/bin/` (per `internal/langregistry/installer.go:22`). | Per D-02: NO migration. After rename, the new binary reads from `~/.helix/...` exclusively. The old `~/.serena/` tree is orphaned (and harmless — users may delete manually). CHANGELOG must explicitly state: "to free disk, `rm -rf ~/.serena` after first successful `helix setup`". |
| Live service config | **None.** No external services are externally configured by name (no SaaS dashboards, no Datadog tags, no Cloudflare Tunnel that the project owns). Per-client MCP registrations (above) are the closest equivalent and are addressed by re-running `helix setup`. | None — verified by inspection of `internal/` (no external-service SDK registrations). |
| OS-registered state | **Claude Code hooks**: installed by `internal/cli/setup_hooks.go` during `serena setup claude-code`. Hook commands invoke the `serena` binary by name. | Re-running `helix setup claude-code` regenerates hooks with the new `helix` binary path. Users who never re-setup will see hooks fail silently with "command not found: serena". CHANGELOG entry must call this out. |
| Secrets / env vars | `SERENA_TEST_*` env vars in test code only (test/integration/harness.go, test/oracle/*). User-facing env vars: review `internal/config/` to enumerate `SERENA_*` reads — 21 total Go-source matches found. **No GitHub Actions secrets are renamed**: `MINISIGN_PRIVATE_KEY` and `MINISIGN_PASSWORD` are minisign-tooling names, not project-scoped. | Code edit: rename every `SERENA_*` reference to `HELIX_*`. CHANGELOG documents the env-var rename. The Phase 51 `MINISIGN_*` Action secrets stay unchanged. |
| Build artifacts / installed packages | **`dist/`**: goreleaser output dir. Old archives still in CI artifact storage carry `serena_v*` names — these are immutable historical artifacts and stay as-is (released tags don't get retroactively renamed). `make build` produces a `serena` binary today (`Makefile:6`); after rename, produces `helix`. Users who built locally must re-run `make clean && make build`. | Code edit: rename `BINARY=serena` → `BINARY=helix` in Makefile. `make clean` already does `rm -f $(BINARY)` so this works idempotently. |

**Canonical question answered:** After every file in the repo is updated, what runtime systems still have the old string cached, stored, or registered?

- **MCP client configs** — must re-run `helix setup <client>`.
- **Claude Code hooks** — re-run `helix setup claude-code` regenerates them.
- **`~/.serena/`** — orphaned; no migration; manual `rm -rf` per CHANGELOG.
- **Local PATH symlinks** users may have created (`ln -s /usr/local/bin/serena ~/bin/serena`) — out of our control; CHANGELOG note.
- **GitHub Actions cached deps + module cache** — first post-rename CI run will rebuild from scratch (the module path change forces this).

## Common Pitfalls

### Pitfall 1: The Go module path is `github.com/postfix/serena`, not `github.com/agenthands/helix`

**What goes wrong:** CONTEXT.md D-01 states "The Go module path (`github.com/agenthands/helix`) is unchanged — no module rename." This is factually wrong. `head go.mod` shows `module github.com/postfix/serena`. There are 392 `github.com/postfix/serena` import lines across the codebase.
**Why it happens:** The repo was migrated from `postfix/serena` to `agenthands/helix` at the GitHub level, but the Go module identifier was not updated. The release pipeline already targets `agenthands/helix` for GitHub Releases (`.goreleaser.yaml:91-93`).
**How to avoid:** The planner MUST surface this to the user before locking the wave plan. The choices are: (a) **leave the module path as `github.com/postfix/serena`** for v1.9 (rename binary + product but not the import path — defensible but produces a permanent identity mismatch), or (b) **rename the module to `github.com/agenthands/helix`** (clean — but a 392-line mechanical edit on top of everything else). Recommendation: do (b) in Wave 1 because the rename's whole rationale is "clean break, no permanent residual mismatch", and a `postfix/serena` module path inside a `helix` binary is exactly the kind of residual the user explicitly wanted to avoid (D-04). Tooling: `go mod edit -module github.com/agenthands/helix` followed by a project-wide find-and-replace of `github.com/postfix/serena` → `github.com/agenthands/helix`, then `gofmt -w .` and `go build ./...` (will catch any missed references at compile time).
**Warning signs:** post-rename build error `package github.com/postfix/serena/internal/foo is not in std`.

### Pitfall 2: `//go:embed ../../minisign.pub` is invalid — embed cannot escape the package directory

**What goes wrong:** CONTEXT.md D-13 suggests `//go:embed ../../minisign.pub` (or "absolute-from-module-root equivalent"). Neither exists. The Go compiler rejects `..` in embed patterns.
**Why it happens:** Embed paths must be (a) relative to the source file and (b) within the same module. `embed.FS` "cannot provide access to files with names beginning with .." [VERIFIED: golang/go#46056].
**How to avoid:** Two viable patterns:
1. **Build-time copy (recommended):** Add a Makefile pre-build step `cp minisign.pub internal/upgrade/minisign.pub`. Add a `go generate ./internal/upgrade/...` directive in `pubkey.go` that does the same. Add a `.gitignore` entry for `internal/upgrade/minisign.pub` so the canonical source stays at repo root. `go generate` runs during local dev; `make build` and goreleaser CI run the copy step before `go build`.
2. **Embed at repo root (alternative):** Define `pubkey.go` at repo root in package `main` with `//go:embed minisign.pub`, then export the bytes via a public function. Keeps the file at exactly one location but couples the `main` package to the upgrade-package's verification logic. Cleaner mechanically but uglier architecturally.
Recommendation: pattern (1). The Makefile already has `make build` as the contributor-facing entry point.
**Warning signs:** `pattern ../../minisign.pub: invalid pattern` at compile time.

### Pitfall 3: `os.Rename` cross-filesystem fallback is NOT atomic

**What goes wrong:** Stage download to `os.TempDir()` (often `/tmp`, mounted as tmpfs) then `os.Rename(stage/helix, /usr/local/bin/helix)` — different filesystems. Go falls back to copy + unlink, which has a visible intermediate state where `/usr/local/bin/helix` exists but is incomplete.
**Why it happens:** POSIX `rename(2)` requires source and destination on the same filesystem.
**How to avoid:** Stage to a sibling dir of the install path. Compute `installDir := filepath.Dir(execPath); stageDir := filepath.Join(installDir, ".helix-upgrade-stage")`. The permission probe (D-09) already requires write access to `installDir`, so the stage dir is automatically writable. Clean up the stage dir on success (or on failure, leave for inspection per Pitfall 4 below).
**Warning signs:** non-atomic visible "half-replaced" binary on slow disks; the new binary appears truncated for 200ms during the copy.

### Pitfall 4: minisign returns `(false, nil)` on signature mismatch — check the bool, not just the error

**What goes wrong:** `pub.Verify(bytes, sig)` returns `(bool, error)`. A *malformed* signature produces `(false, error)`. A *valid-format-but-wrong-key* signature produces `(false, nil)`. Code that only checks `err != nil` accepts wrong-key signatures.
**Why it happens:** go-minisign distinguishes "couldn't parse" from "parsed but didn't verify".
**How to avoid:** Always `if err != nil || !ok { return fmt.Errorf("verification failed: %v", err) }`. Make the error message identical for both branches so an attacker probing the failure mode can't distinguish them.
**Warning signs:** unit test passes with a deliberately-tampered signature; `--dry-run` against a corrupted archive returns 0 instead of failing.

### Pitfall 5: Windows `os.Exec` is not the same as Linux `syscall.Exec`

**What goes wrong:** On Linux, `syscall.Exec` replaces the current process image. On Windows, there's no equivalent; `os.StartProcess` spawns a child. Naive cross-platform code uses `os.StartProcess` everywhere and breaks the "same shell session, new version" UX on Linux/macOS where users *expect* to see the new banner inline.
**Why it happens:** Windows lacks the POSIX exec model.
**How to avoid:** Platform-split the `relaunch()` function. Linux/macOS: `syscall.Exec(path, args, env)` — never returns on success. Windows: `cmd.Start()` then `os.Exit(0)`. Always set `cmd.Stdin/Stdout/Stderr` so the child inherits the terminal.
**Warning signs:** Windows shows two banners (parent + child); Linux loses output between `os.Exec` invocation and process replacement.

### Pitfall 6: GitHub anonymous rate limit is 60/hr/IP — multiple `helix update` from a CI pipeline blow the budget

**What goes wrong:** A CI pipeline runs `helix update` on every job for change-detection; quickly exhausts the unauthenticated rate limit; user gets cryptic 403s.
**Why it happens:** 60 req/hr/IP is shared across all unauthenticated GitHub API consumers from that IP — `git`, `helix`, other tools, etc.
**How to avoid:** (a) Require a User-Agent header on all requests (already in Pattern 1 above). (b) On 403/429 with `X-RateLimit-Remaining: 0`, surface a clear "rate-limited; retry after X minutes; if running in CI, set GITHUB_TOKEN to authenticate" message. (c) Honor a `GITHUB_TOKEN` env var if set (auth lifts to 5000/hr). (d) Document this in INSTALL.md "Upgrading" section.
**Warning signs:** "API rate limit exceeded" without an actionable path forward.

### Pitfall 7: The release archive name template flips silently break the upgrade subcommand

**What goes wrong:** Phase 51's `.goreleaser.yaml` produces `serena_v1.9.0_linux_amd64.tar.gz`. After Phase 52 rename, it produces `helix_v1.10.0_linux_amd64.tar.gz`. A `helix upgrade` running on a v1.9.0 binary tries to fetch `helix_v1.10.0_linux_amd64.tar.gz` from the v1.10.0 release — works. But `helix upgrade` on a v1.8.x serena binary doesn't exist (the subcommand is new in v1.9), so this isn't a backwards-compat issue *per se*; the issue is that the asset-name template MUST match what `helix update` expects literally. Mismatch produces a confusing 404.
**Why it happens:** The asset name is constructed client-side as `helix_${VERSION}_${OS}_${ARCH}.tar.gz` and looked up in the release's `assets[]` list. Any drift breaks asset discovery.
**How to avoid:** Make the asset-name template a **shared constant** between `.goreleaser.yaml` and the `internal/upgrade/` package. Recommend: a `goreleaser_assets.go` test that parses `.goreleaser.yaml`'s `archives[0].name_template`, reproduces the same template in Go, and asserts equality on a sample input. Catches drift at PR-review time.
**Warning signs:** post-release manual test of `helix upgrade` returns "asset not found in release v1.10.0".

### Pitfall 8: The Phase 51 reproducibility CI gate compares archives by name

**What goes wrong:** Phase 51's reproducibility job builds twice and diffs `dist/`. After the rename, the diff still works because both builds produce `helix_*` archives. BUT the **first** post-rename release runs the diff against archives whose names changed mid-pipeline if the workflow somehow caches goreleaser metadata across runs. Less of a "will break" and more of a "verify this won't break".
**Why it happens:** Goreleaser doesn't cache anything keyed on `project_name` between runs [VERIFIED: goreleaser.com/customization/archive — no cache mentioned]. The `dist/` directory is ephemeral per CI run. So this is *probably* fine, but the planner should (a) read `.github/workflows/release.yml` carefully for any `project_name` literal that needs flipping and (b) run the local `make release-snapshot` before pushing the rename PR to confirm.
**How to avoid:** Local pre-flight: `make release-snapshot` after rename, manually inspect `dist/` for `helix_v*` archives and absent `serena_v*` archives. Then push.
**Warning signs:** CI release.yml fails on "archive sha256 mismatch" between pass-1 and pass-2 — would be a goreleaser bug, but worth a sanity check.

## Code Examples

### `helix update` end-to-end

```go
// Source: docs.github.com/en/rest/releases (HIGH confidence) + project conventions
// internal/cli/update.go
func newUpdateCommand() *cobra.Command {
    var prerelease bool
    cmd := &cobra.Command{
        Use:   "update",
        Short: "Check for a newer Helix release without installing",
        Long:  "Read-only check against the GitHub Releases API. Prints current and latest versions plus release notes.",
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            current := canonical(Version) // ldflag-injected
            release, err := upgrade.FetchReleaseInfo(ctx, prerelease)
            if err != nil {
                return err
            }
            fmt.Printf("current: %s\n", current)
            fmt.Printf("latest:  %s\n", release.TagName)
            if upgrade.IsDowngrade(current, release.TagName) {
                fmt.Println("status:  up to date")
                return nil
            }
            fmt.Printf("status:  upgrade available\n\n--- release notes ---\n%s\n", release.Body)
            return nil
        },
    }
    cmd.Flags().BoolVar(&prerelease, "prerelease", false, "Include pre-release versions")
    return cmd
}
```

### `helix upgrade` skeleton

```go
// internal/cli/upgrade.go
func newUpgradeCommand() *cobra.Command {
    var (
        prerelease bool
        version    string
        check      bool
        dryRun     bool
    )
    cmd := &cobra.Command{
        Use:   "upgrade",
        Short: "Download, verify, and install a newer Helix release",
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()

            // D-08: refuse if running inside daemon
            if upgrade.RunningInDaemon() {
                fmt.Println("helix is running as a daemon child process; restart the daemon manually after upgrading from a non-daemon shell")
                return nil
            }

            // --check is functionally identical to `helix update` (D-12)
            if check {
                return runUpdate(ctx, prerelease)
            }

            opts := upgrade.Options{
                Prerelease: prerelease,
                Version:    version, // empty = latest
                DryRun:     dryRun,
                Current:    Version,
            }
            return upgrade.Upgrade(ctx, opts)
        },
    }
    cmd.Flags().BoolVar(&prerelease, "prerelease", false, "Include pre-release versions")
    cmd.Flags().StringVar(&version, "version", "", "Pin to specific version (overrides --prerelease)")
    cmd.Flags().BoolVar(&check, "check", false, "Check for updates without installing (alias for `helix update`)")
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Download + verify but do not swap or restart")
    return cmd
}
```

### Permission probe (D-09)

```go
// internal/upgrade/permission.go
func ProbeWritable(installPath string) error {
    dir := filepath.Dir(installPath)
    f, err := os.CreateTemp(dir, ".helix-perm-probe-*")
    if err != nil {
        return err
    }
    name := f.Name()
    f.Close()
    os.Remove(name)
    return nil
}

func SudoHint(installPath string) string {
    return fmt.Sprintf("helix is installed at %s and your user cannot write there.\nRe-run with elevated privileges, e.g.:\n  sudo helix upgrade %s",
        installPath, strings.Join(os.Args[2:], " "))
}
```

### Test stub for GitHub API

```go
// internal/upgrade/upgrade_test.go
func TestFetchLatest_HappyPath(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        require.Equal(t, "/repos/agenthands/helix/releases/latest", r.URL.Path)
        require.Contains(t, r.Header.Get("User-Agent"), "helix-upgrade/")
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(Release{
            TagName: "v1.10.0",
            Body:    "## Changes\n- thing 1\n- thing 2\n",
            Assets: []Asset{{
                Name:               "helix_v1.10.0_linux_amd64.tar.gz",
                BrowserDownloadURL: server.URL + "/download/helix_v1.10.0_linux_amd64.tar.gz",
            }},
        })
    }))
    defer server.Close()

    r, err := fetchLatestFromBase(context.Background(), server.URL, "agenthands", "helix")
    require.NoError(t, err)
    require.Equal(t, "v1.10.0", r.TagName)
}
```

The same pattern (httptest server returning canned responses + asset bytes) covers `--prerelease`, `--version`, downgrade refusal, signature failure (server returns deliberately corrupted .minisig), and rate-limit handling.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `inconshreveable/go-update` | `minio/selfupdate` (maintained fork) | ~2018 (original abandoned) | Plugin replaced; for our use, *neither* is needed — we hand-roll using minio's apply.go as reference. |
| Curl-pipe-sh installer scripts | Signed archives + verification ceremony | shell-injection vulnerabilities + supply-chain awareness | Phase 51 already established this; Phase 52 reaffirms via in-binary upgrade. |
| Trusting-on-first-use (TOFU) public keys | Embedded pub key shipped with binary | Modern supply-chain hygiene | TOFU works for SSH known_hosts; binaries warrant stronger guarantees. |
| Vendoring `google/go-github` for any GitHub API call | `net/http` for narrow integrations | Build-cost awareness | Hand-rolling 3 endpoints is cheaper than ~10MB of generated client code. |
| Hardcoded version constants | `-ldflags="-X main.version=$VERSION"` | Build reproducibility | Phase 51's `.goreleaser.yaml` already does this; Phase 52 reuses. |

**Deprecated/outdated:**
- `inconshreveable/go-update` (last meaningful release 2018) — replaced by `minio/selfupdate`.
- `gobinary` upgrade tools that fork-and-exec without explicit version pinning — replaced by version-pinned, signature-verified flows.

## Project Constraints (from CLAUDE.md)

- **Build commands:** `go build ./cmd/serena` (post-rename: `./cmd/helix`); `go test ./...`; `go vet ./...`; `gofmt -w .`; `make build`; `make test`. Run go vet and go test before completing any Go task — non-optional.
- **Single Go binary, native concurrency:** Phase 52 must NOT introduce CGO transitive deps. The chosen libraries (`go-minisign`, `golang.org/x/mod/semver`, stdlib) are all CGO-free. [VERIFIED: jedisct1/go-minisign go.mod has only `golang.org/x/crypto` + `golang.org/x/sys` — both pure Go.]
- **MCP only:** No JetBrains or proprietary backends. Upgrade subcommand is a CLI verb; not exposed as an MCP tool.
- **No CGO dependencies invariant:** Phase 51.1 gates `internal/treesitter` behind `//go:build cgo`. None of the new `internal/upgrade/` code may add a CGO dep.
- **Repo:** Same repo, Python in `legacy/`. Phase 52 does not touch `legacy/`.
- **GSD workflow:** Edits go through `/gsd:execute-phase` once a plan exists.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `go test` (Go 1.25.1) + `testify/require` (already in tree) |
| Config file | None (Go's built-in test discovery) |
| Quick run command | `go test ./internal/upgrade/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PKG-05 (rename) | `cmd/helix/main.go` builds CGO=0 | smoke | `CGO_ENABLED=0 go build ./cmd/helix` | ❌ Wave 1 |
| PKG-05 | No `github.com/postfix/serena` imports remain | grep gate | `! grep -r "github.com/postfix/serena" --include="*.go" .` | ❌ Wave 1 |
| PKG-05 | No `SERENA_` env-var references in production code | grep gate | `! grep -r "SERENA_" --include="*.go" --exclude-dir=legacy --exclude-dir=test .` | ❌ Wave 2 |
| PKG-05 | No `~/.serena` / `.serena/` path strings | grep gate | `! grep -rn '"\.serena' --include="*.go" --exclude-dir=legacy .` | ❌ Wave 2 |
| PKG-05 | MCP server registers as `helix` | unit | `go test ./internal/cli/... -run TestSetupClients_RegistersHelix` | ❌ Wave 2 |
| PKG-05 | `helix --version` prints "Helix" | unit | `go test ./internal/cli/... -run TestRoot_Version` | ❌ Wave 2 |
| PKG-06 (semver compare) | downgrade detection | unit | `go test ./internal/upgrade/ -run TestSemver` | ❌ Wave 4 |
| PKG-06 (permission probe) | unwritable path returns hint, no network call | unit | `go test ./internal/upgrade/ -run TestProbeWritable` | ❌ Wave 4 |
| PKG-06 (daemon detect) | env-var probe true/false | unit | `go test ./internal/upgrade/ -run TestRunningInDaemon` | ❌ Wave 4 |
| PKG-06 (archive name) | template matches goreleaser config | unit | `go test ./internal/upgrade/ -run TestArchiveNameTemplate` | ❌ Wave 4 |
| PKG-06 (`helix update`) | API stub → prints current + latest + body | integration | `go test ./internal/upgrade/ -run TestUpdate_HappyPath` | ❌ Wave 4 |
| PKG-06 (`helix upgrade --dry-run`) | download + verify, no swap | integration | `go test ./internal/upgrade/ -run TestUpgradeDryRun` | ❌ Wave 4 |
| PKG-06 (`helix upgrade --version` pin) | targets specific tag, refuses if older | integration | `go test ./internal/upgrade/ -run TestUpgradeVersionPin` | ❌ Wave 4 |
| PKG-06 (corrupt signature → fail closed) | tampered .minisig rejected | unit (negative) | `go test ./internal/upgrade/ -run TestVerify_TamperedSig` | ❌ Wave 4 |
| PKG-06 (Windows .old swap) | swap_windows.go path tested | unit (platform) | `GOOS=windows go test ./internal/upgrade/...` (or CI matrix) | ❌ Wave 4 |
| PKG-06 (Linux inode-replace) | `swap_unix.go` path tested | unit | `go test ./internal/upgrade/ -run TestSwap_Unix` | ❌ Wave 4 |
| PKG-07 (embed audit) | `EMBED-AUDIT.md` exists with all 5 categories | doc-existence + manual review | `test -s .planning/phases/52-packaging-distribution-channels/EMBED-AUDIT.md` | ❌ Wave 4 |
| PKG-07 (minisign.pub embedded) | binary contains key bytes | unit | `go test ./internal/upgrade/ -run TestEmbeddedPubKey` | ❌ Wave 4 |
| PKG-07 (build fails if pubkey missing) | rm internal/upgrade/minisign.pub → build fails | manual / smoke | `mv internal/upgrade/minisign.pub /tmp/x; go build ./cmd/helix; mv /tmp/x internal/upgrade/minisign.pub` | manual |

### Sampling Rate

- **Per task commit:** `go test ./internal/upgrade/... -count=1 -race` + `go vet ./...` (≤30s).
- **Per wave merge:** `go test ./... -count=1 -race` (full suite, ≤5min).
- **Phase gate:** `make release-snapshot` (goreleaser dry-run produces 6 archives with `helix_v*` names) + full grep matrix for residual `serena` / `postfix` / `SERENA_` / `.serena` references + manual run of `helix update` against the project's actual GitHub API + `go test ./... -count=1 -race`.

### Wave 0 Gaps

- [ ] `internal/upgrade/upgrade_test.go` — covers PKG-06 happy paths
- [ ] `internal/upgrade/verify_test.go` — covers minisign happy + tampered + wrong-key
- [ ] `internal/upgrade/semver_test.go` — covers downgrade refusal + prerelease filter
- [ ] `internal/upgrade/permission_test.go` — covers writable + unwritable
- [ ] `internal/upgrade/daemon_detect_test.go` — covers env-var on/off
- [ ] `internal/upgrade/swap_unix_test.go` — covers same-fs and cross-fs paths (skip on Windows)
- [ ] `internal/upgrade/swap_windows_test.go` — covers .old rename (skip on non-Windows)
- [ ] `internal/upgrade/testdata/` — test minisign keypair + sample release archives
- [ ] `internal/upgrade/archive_test.go` — covers tar.gz extraction
- [ ] Test-only minisign keypair generation: a `make gen-test-keypair` target that runs `minisign -G -p test/testdata/minisign.pub -s test/testdata/minisign.key -W` (passwordless) — pubkey checked in, signing key checked in (test-only, clearly marked).
- [ ] `internal/cli/upgrade_test.go` — cobra-level test that asserts flag parsing + daemon-detect short-circuit.
- [ ] Framework install: nothing new — `testify/require` already in `go.sum`.

### Nyquist Dimensions

| Dimension | Coverage | Tests |
|-----------|----------|-------|
| 1. Boundary | Empty release list, no assets, malformed JSON, 0-byte archive, missing `.minisig`, missing `checksums.txt`, asset name not in archive list | `TestFetchLatest_EmptyReleases`, `TestExtract_ZeroByte`, `TestVerify_MissingSig`, `TestArchiveLookup_AssetMissing` |
| 2. Functional | Happy paths for `update`, `upgrade`, `--check`, `--dry-run`, `--version`, `--prerelease` | `TestUpdate_HappyPath`, `TestUpgrade_HappyPath`, `TestUpgrade_VersionPin`, `TestUpgrade_PrereleaseFlag` |
| 3. Adversarial | Corrupted signature, wrong-key signature, downgrade attempt, sudo-escalation refusal | `TestVerify_TamperedSig`, `TestVerify_WrongKey`, `TestUpgrade_DowngradeRefused`, `TestUpgrade_RootRequired` |
| 4. Performance | Stage-dir fs-scope (same-fs rename is O(1)), single GitHub API call (60/hr budget) | manual + `TestUpgrade_NoUnnecessaryAPICalls` |
| 5. Concurrency | Two `helix upgrade` invocations racing — second fails with file-locked or rename-mid-flight error | `TestUpgrade_ConcurrentInvocation` (best-effort; file-lock the stage dir) |
| 6. State | Stage-dir cleanup on success, retention on failure, .old leak on Windows | `TestStage_CleanupOnSuccess`, `TestStage_RetainedOnFailure`, `TestSwap_Windows_OldFileExists` |
| 7. Compatibility | All 6 GOOS×GOARCH combos build CGO=0; `helix update` reads release built by Phase 51 pipeline | `TestArchiveNameTemplate_MatchesGoreleaser`, CI matrix runs `make release-snapshot` |
| 8. Integration | End-to-end against project's real GitHub API (manual UAT); httptest stub for CI | `TestUpgrade_E2E_HTTPTest` + manual UAT step in VERIFICATION.md |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build | ✓ | 1.25.1 (per go.mod) | — |
| `make` | build entrypoint | ✓ | system | direct `go build` if no make |
| `goreleaser` | release pipeline | ✓ in CI; optional locally | — | Local dev: `make release-snapshot` short-circuits with helpful error if missing (already shipped per Phase 51 51-05) |
| `minisign` (CLI) | CI signing only | CI: yes | latest | None — CI-side only; users do not need it |
| `git` | source control | ✓ | system | — |
| `curl` / `tar` | INSTALL.md verification ceremony | ✓ on user's machine | system | — |
| GitHub Releases API access | `helix update` / `helix upgrade` | network-dependent | — | If offline: print "no network" error and exit 1; honor `GITHUB_TOKEN` to lift rate limit |

**Missing dependencies with no fallback:**
- None for Phase 52's in-scope work. The CI-side minisign tooling is already wired in Phase 51.

**Missing dependencies with fallback:**
- `goreleaser` for local development — already handled by Phase 51 51-05 (Makefile guard with helpful error).

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | upgrade subcommand has no user authentication; signature *is* the trust anchor |
| V3 Session Management | no | stateless one-shot CLI; no session |
| V4 Access Control | yes | OS-level write permission probe BEFORE network I/O (D-09) |
| V5 Input Validation | yes | semver parsing (use `golang.org/x/mod/semver`, never custom regex); JSON response validation; archive-name template comparison |
| V6 Cryptography | yes | minisign Ed25519 verification via `jedisct1/go-minisign`; never hand-roll signature math |
| V7 Error Handling | yes | fail-closed on signature failure; identical error message for "wrong key" and "tampered signature" so attackers can't probe failure modes |
| V8 Data Protection | yes | embed `minisign.pub` rather than fetching at runtime (avoids TOFU + MITM risk on the key itself) |
| V11 Business Logic | yes | hard-refuse downgrade prevents rollback attacks (D-10); no `--force-downgrade` escape hatch |
| V12 Files and Resources | yes | atomic swap prevents half-replaced binary; .old leak on Windows is a known acceptable property |

### Known Threat Patterns for self-upgrading binaries

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Substituted release archive | Tampering | Minisign signature with embedded pubkey (single source of truth, no TOFU) |
| Substituted checksums.txt | Tampering | `checksums.txt` is itself signed (Phase 51 D-01a / 51-05); both archive *and* checksums verified independently |
| Rollback to vulnerable older version | Tampering / Repudiation | Hard-refuse downgrade (D-10); no override flag |
| MITM on key file fetch | Tampering | `minisign.pub` embedded at build time, NOT fetched at runtime |
| Public-key replacement at upgrade time | Tampering | The new binary contains the *next* `minisign.pub`; if the maintainer rotates keys, current users (running embedded *old* key) cannot verify the new release and fail closed — this is the correct behavior for emergency key rotation. CHANGELOG must announce the rotation so users do a manual verified install. |
| Half-installed binary on swap failure | DoS | Atomic rename on Unix; Windows rolls back .old → current on swap failure (Pattern 4 above) |
| Privilege escalation surprise | Elevation of Privilege | D-09 permission probe BEFORE download — no internal sudo prompt; user retypes the command with sudo |
| Cached old credentials in `~/.serena` | Information Disclosure | Hard cut per D-02; users re-onboard; CHANGELOG documents the cleanup |
| Log injection via release notes | Tampering / Injection | Release notes printed to terminal; sanitize ANSI escape sequences before printing the body to prevent terminal injection |
| Rate-limit-induced failure mode | DoS | Honor `GITHUB_TOKEN`; surface clear error message; don't retry-loop |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The project prefers minimal third-party dependencies over feature-rich libraries (e.g. hand-roll the GitHub API client over vendoring `google/go-github`). | Standard Stack > Alternatives | Low — if the user prefers a vendored client, swap during planning. No locked decision. |
| A2 | The maintained `minio/selfupdate` is suitable as a *reference* for our hand-rolled atomic swap, but we do not depend on it. | Standard Stack > Alternatives, Pattern 4 | Low — if planner chooses to depend on `minio/selfupdate` instead, ~80 LOC of swap code disappears in exchange for a dependency. Cleaner for the planner; not a security regression since signature verification is independent of the swap mechanism. |
| A3 | Stage-dir choice of "sibling of install path" is preferable to `os.TempDir()` because the rename is then guaranteed same-filesystem. | Pitfall 3, Code Examples | Medium — if the install path is on a read-only fs (e.g. immutable distro like NixOS), the sibling-stage approach fails the permission probe earlier than expected. The alternative (TempDir + cross-fs copy fallback) is non-atomic and we explicitly reject it. The sibling approach surfaces "this fs is read-only" cleanly. |
| A4 | The Phase 51 reproducibility CI gate compares archives by name and is robust to a one-time `serena_v*` → `helix_v*` rename. | Pitfall 8 | Low — Pitfall 8 itself flags this as a check-before-merge item. Verified during pre-merge `make release-snapshot`. |
| A5 | `golang.org/x/mod/semver` handles all the project's expected version strings (including pre-release suffixes like `v1.9.0-rc1`, `v1.9.0-beta.1`). | Pattern 6 | Low — `x/mod/semver` is the canonical Go semver library and handles all SemVer 2.0.0 forms including build metadata. |
| A6 | The MCP server's `Implementation.Name` field (currently `"serena"` in `internal/mcp/server.go:51`) is the value that propagates to clients as the server identity. Changing it to `"helix"` is the right semantic — distinct from the MCP **registration name** in `setup_clients.go` (also `"serena"`, also flips). | Code Examples, Pitfall 7 | Medium — both must flip together. If they desync (e.g. registration says "helix" but server identifies as "serena"), MCP clients see a mismatched identity. Verified via `TestSetupClients_RegistersHelix` test. |
| A7 | The project version `2.0.0-dev` (currently hardcoded in `internal/cli/root.go:62`) gets replaced by the ldflag-injected `Version` variable in this phase. | Code Examples > `helix update` | Medium — if the planner doesn't introduce a `var Version = "dev"` at the top of `internal/cli/root.go` (or wherever) wired to `-X main.version=...`, then `helix update` reports a meaningless version. The current hardcoded "2.0.0-dev" is a legacy of the Go-rewrite naming; the ldflag is already wired in `.goreleaser.yaml:26` (`-X main.version={{.Version}}`) but does NOT have a `main.version` package-level variable to bind to. **The planner must add this**. |
| A8 | The user-chosen v1.9 version bump is acceptable for a binary-name + env-var + config-dir break, even though semver-strictly this is v2.0 territory. | Open Questions, CHANGELOG | Low — the project is already at "2.0.0-dev" internally. The user picked v1.9 for the public version; the planner respects this without proposing v2.0. CHANGELOG calls out the breaking changes prominently regardless of the version number. |

## Open Questions (RESOLVED)

1. **Module path mismatch** (the most important question)
   - What we know: `go.mod` says `module github.com/postfix/serena`. CONTEXT.md asserts the module path is `github.com/agenthands/helix` and won't change. These contradict. 392 import lines across the codebase reference `github.com/postfix/serena`.
   - What's unclear: did the user mean "leave the import path" (in which case the rebrand has a permanent residual) or "the path matches the repo" (in which case a 392-line rename is required)?
   - Recommendation: **The planner MUST re-confirm with the user before locking the wave plan.** Recommended answer: rename the module to `github.com/agenthands/helix` in Wave 1 — `go mod edit -module github.com/agenthands/helix` + project-wide find-replace + `gofmt -w .` + `go build ./...`. This is a noisy diff but it's deterministic and the build catches misses.
   - **RESOLVED:** via CONTEXT.md D-01 (locked 2026-04-29) — module renames `github.com/postfix/serena` → `github.com/agenthands/helix`. Implemented in Plan 02 Task 1 via `go mod edit` + portable `perl -i -pe` find-replace + `gofmt -w .` + `go build ./cmd/helix`.

2. **Version variable wiring**
   - What we know: `.goreleaser.yaml:26` injects `-X main.version=...`. `internal/cli/root.go:62` hardcodes `"serena version 2.0.0-dev"`. `cmd/serena/main.go` has no `var version` — the ldflag has nothing to bind to.
   - What's unclear: where should the `main.version` var live? CONTEXT.md's "ldflags-embedded version" comment in <specifics> assumes it already exists.
   - Recommendation: declare `var version = "dev"` in `cmd/helix/main.go` (the ldflag's `main.` prefix already targets package main); thread it into `internal/cli/` via `cli.SetVersion(version)`.
   - **RESOLVED:** declare `var version = "dev"` in `cmd/helix/main.go` (Plan 02 Task 1); thread via `cli.SetVersion(version)` exported by `internal/cli/root.go`.

3. **EMBED-AUDIT.md scope**
   - What we know: D-14 specifies the audit produces a written manifest classifying every runtime asset. D-15 says LSes are external-by-design.
   - What's unclear: how much depth does the audit go to? Every `os.Open`, every `embed.FS`, every `filepath.Join(home, ".serena", ...)`? Or only the "could be embedded" gray-area cases?
   - Recommendation: the planner produces the audit by `grep -rn "os.ReadFile\|os.Open\|embed:" --include="*.go"` and classifies each finding. ~50-100 entries expected. Sufficient depth.
   - **RESOLVED:** Plan 05 produces the full manifest; classification per file (embedded / external-by-design / gap) per D-14. Gaps closed in-phase; manifest persists as living artifact.

4. **CHANGELOG convention check**
   - What we know: CHANGELOG.md uses a "v1.X — Title (date)" header with subsections like `### Setup CLI`, `### Health & Status`. Not strictly Keep-a-Changelog ("Added/Changed/Deprecated/Removed/Fixed") but a project-specific narrative format.
   - What's unclear: should the v1.9 entry use a "BREAKING" tag or callout?
   - Recommendation: the v1.9 entry's first subsection should be `### Breaking Changes (v1.8 → v1.9)` with a 4-bullet list (binary name, env vars, config dir, MCP registration). The narrative subsections follow.
   - **RESOLVED:** Plan 06 Task 2 adds a `### Breaking Changes (v1.8 → v1.9)` subsection per Keep-a-Changelog with the four user-visible breaks enumerated.

5. **Pre-existing `2.0.0-dev` versus user-chosen `v1.9`**
   - What we know: `internal/cli/root.go:62` and `internal/mcp/server.go:51` both say `2.0.0-dev`. CONTEXT.md and the user choose v1.9 for the rename release.
   - What's unclear: is the `2.0.0-dev` placeholder intended to be replaced by ldflag injection at release time, or is it a stale legacy from the Python-to-Go transition?
   - Recommendation: ldflag-inject. Default fallback `"dev"`. The two strings flip together via the `cli.SetVersion()` thread.
   - **RESOLVED:** irrelevant once `-X main.version=...` ldflag binds (Plan 02 Task 1 introduces `var version` in `cmd/helix/main.go`); the `2.0.0-dev` literal is removed in Plan 02/03 in favor of the ldflag-injected value threaded via `cli.SetVersion()`.

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/github.com/jedisct1/go-minisign](https://pkg.go.dev/github.com/jedisct1/go-minisign) — verified API surface; pure Go; no CGO
- [pkg.go.dev/embed](https://pkg.go.dev/embed) — `..` forbidden in embed patterns
- [docs.github.com/en/rest/releases/releases](https://docs.github.com/en/rest/releases/releases) — `/releases/latest` filters drafts/prereleases; `/releases` includes them
- [docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) — 60 req/hr/IP unauthenticated
- [pkg.go.dev/golang.org/x/mod/semver](https://pkg.go.dev/golang.org/x/mod/semver) — canonical Go semver compare
- [goreleaser.com/customization/archive](https://goreleaser.com/customization/archive/) — name_template variables (`.ProjectName`, `.Binary`, `.Os`, `.Arch`, `.Version`); no caching keyed on project_name
- Project source: `go.mod`, `.goreleaser.yaml`, `Makefile`, `cmd/serena/main.go`, `internal/cli/root.go`, `internal/cli/setup_clients.go`, `internal/memory/store.go`, `internal/config/defaults.go`, `internal/mcp/server.go`, `INSTALL.md`, `CHANGELOG.md`, `minisign.pub`

### Secondary (MEDIUM confidence)
- [github.com/inconshreveable/go-update](https://github.com/inconshreveable/go-update) — atomic swap reference (unmaintained)
- [github.com/minio/selfupdate](https://github.com/minio/selfupdate) — maintained fork with current Windows quirks documented
- [github.com/jedisct1/go-minisign](https://github.com/jedisct1/go-minisign) — repo (latest tag count visible; specific version verified via go.mod)
- [github.com/golang/go#46056](https://github.com/golang/go/issues/46056) — `..` in embed paths is intentionally forbidden
- [github.com/golang/go#58519](https://github.com/golang/go/issues/58519) — proposal for parent-dir embeds (not accepted)

### Tertiary (LOW confidence — flagged for validation if locked into PLAN)
- Specific Windows behavior across Win10 → Win11 versions for `MoveFileEx` on running executables — verified pattern works on Win10 1803+, but the planner's CI matrix should include both Windows runners.
- Whether `golang.org/x/mod/semver` correctly handles `+build.metadata` per SemVer 2.0.0 — assumed yes based on its provenance, but the planner's unit test should include a `v1.9.0+build.42` case.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every recommended library was verified against pkg.go.dev or its repo; no `[ASSUMED]` claims about API surfaces.
- Architecture (rename + upgrade flow): HIGH on the conceptual flow; MEDIUM on the precise file split inside `internal/upgrade/` (planner has discretion).
- Pitfalls: HIGH on the embed-path pitfall (verified across 3 sources including the official Go issue); HIGH on cross-filesystem rename (well-documented POSIX behavior); MEDIUM on Windows .exe locking (behavior varies by Windows version + admin elevation).
- CONTEXT.md correctness: TWO issues found and surfaced (module path D-01 mismatch; embed path D-13 invalid syntax). Both flagged as Open Questions and Pitfalls.

**Research date:** 2026-04-29
**Valid until:** 2026-05-29 (stable libraries; goreleaser and GitHub API are unlikely to change in 30 days)
