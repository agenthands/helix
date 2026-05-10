---
phase: 67
plan: "02"
subsystem: eval/sandbox, eval/agent, eval/budget
tags: [eval, sandbox, isolation, subprocess, budget, watchdog, tdd]
dependency_graph:
  requires: [67-01]
  provides: [sandbox-primitive, agent-subprocess-wrapper, budget-watchdog]
  affects: [67-03, 67-05, 67-06]
tech_stack:
  added: []
  patterns:
    - per-(task,mode) tmpdir under /tmp for short UDS paths (macOS 104-byte limit)
    - os.Lstat symlink check instead of filepath.EvalSymlinks comparison (T-67-01)
    - yaml.v3 KnownFields(true) strict YAML parsing
    - context.WithCancel + timer goroutine for seconds-axis budget enforcement
    - sync/atomic counters + mutex first-breach wins for token/tool-call axes
key_files:
  created:
    - internal/eval/sandbox/sandbox.go
    - internal/eval/sandbox/sandbox_test.go
    - internal/eval/agent/claude.go
    - internal/eval/agent/claude_test.go
    - internal/eval/agent/mcpconfig.go
    - internal/eval/agent/mcpconfig_test.go
    - internal/eval/budget/budget.go
    - internal/eval/budget/budget_test.go
  modified: []
decisions:
  - "/tmp instead of os.TempDir() for sandbox root (macOS UDS 104-byte path limit)"
  - "os.Lstat for symlink check (not filepath.EvalSymlinks) avoids /var -> /private/var false positive"
  - "context.WithCancel + timer goroutine for seconds axis (not context.WithTimeout) to ensure breach reason is set before Done fires"
  - "yaml.v3 KnownFields(true) for budget.yaml strict parsing — unknown keys surface loudly"
metrics:
  duration: "14m"
  completed: "2026-05-10T11:43:26Z"
  tasks_completed: 3
  tasks_total: 3
  files_created: 8
  files_modified: 0
---

# Phase 67 Plan 02: Sandbox + Agent + Budget Watchdog Summary

**One-liner:** Per-(task,mode) tmpdir isolation with UDS daemon spawn, claude CLI subprocess wrapper with strict env allowlist, and four-axis D-08 budget watchdog using context cancellation and atomic counters.

## What Was Built

### Task 1: Sandbox primitive (7 tests GREEN, race-clean)

`internal/eval/sandbox/sandbox.go` implements the per-(task,mode) filesystem and process isolation:

- `NewSandbox(runID, helixBin)` creates a root tmpdir under `/tmp` (not `os.TempDir()` — see Deviations) with mode 0700; rejects symlinked root via `os.Lstat` check (T-67-01 mitigation).
- `ModeDir / HomeFor / RepoFor / SocketFor / McpConfigPath` return consistent paths under `<root>/<task>/<mode>/`.
- `Prepare` creates `home/.helix/` and `repo/` subdirs at 0700.
- `CloneRepo` recursively copies corpus repo files; skips symlinks (no escape from srcRepo).
- `StartDaemon` spawns helix via `exec.CommandContext` with env allowlist (`PATH`, `HOME=HomeFor`, `HELIX_LOG_LEVEL=info`); polls socket with 50ms ticker up to 10s; redirects daemon stderr to `<modeDir>/daemon.log`.
- `DaemonHandle.Kill` sends SIGKILL and waits up to 5s.
- `Cleanup` kills all held handles and calls `os.RemoveAll(root)`.
- `waitSocket` mirrors the `internal/forwarder/dial.go` socket-poll pattern.

### Task 2: Agent subprocess wrapper (7 tests GREEN, race-clean)

`internal/eval/agent/mcpconfig.go` writes the CC `--mcp-config` JSON:
- `WriteMCPConfig` requires absolute `helixBin` path (T-67-03 mitigation); writes `{"mcpServers":{"helix":{"type":"stdio","command":...,"args":["--socket",sockPath],"env":{"HOME":homePath}}}}` at mode 0600.

`internal/eval/agent/claude.go` wraps the claude CLI subprocess:
- `cleanEnv` allowlist: `PATH`, `HOME` (= `sandbox.HomeFor`), `ANTHROPIC_API_KEY`, `HELIX_LOG_LEVEL` — all other host env vars are stripped (T-67-Pitfall-1 mitigation).
- `buildArgv` produces fixed order: `--print --bare --strict-mcp-config --mcp-config <path> --output-format stream-json --verbose --include-partial-messages --max-turns <N> <prompt>`.
- `Run` resolves `claude` via `exec.LookPath` → `ErrClaudeNotFound` if absent; captures stdout/stderr to `<modeDir>/claude.stdout` and `claude.stderr` (mode 0600); returns `ctx.Err()` on context cancellation.

### Task 3: Budget watchdog (6 tests GREEN, race-clean)

`internal/eval/budget/budget.go` implements D-08 four-axis enforcement:
- `Default = Budget{MaxInputTokens:200000, MaxOutputTokens:32000, MaxSeconds:300, MaxToolCalls:100}`.
- `LoadFromFile` uses `yaml.v3` with `KnownFields(true)` strict mode; merges over `Default` for unset fields; empty file returns `Default`.
- `NewWatchdog` uses `context.WithCancel` + a timer goroutine (not `context.WithTimeout`) so the breach reason is set atomically before the done channel fires.
- `RecordToolCall` / `RecordTokens` use `sync/atomic` counters; first breach wins via mutex.
- `Breach()` returns the `*BreachReason` from Plan 01 (`types.go`) — not redeclared.
- `Done()` proxies the inner context done channel.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] /tmp vs os.TempDir() for sandbox root**

- **Found during:** Task 1 (TestSandboxStartDaemon FAIL)
- **Issue:** macOS Unix domain socket paths have a 104-byte limit (`sys/un.h`). `os.TempDir()` returns `/var/folders/ym/<long-path>/T/` which, combined with `<task>/<mode>/daemon.sock`, exceeds 104 bytes. The `net.Listen("unix", sockPath)` call fails with `bind: invalid argument` inside the sandbox subprocess.
- **Fix:** Changed `os.MkdirTemp("", ...)` to `os.MkdirTemp("/tmp", ...)` and applied `filepath.EvalSymlinks` to resolve `/tmp -> /private/tmp` on macOS. `/private/tmp/helix-eval-<runID>-XXXXXXXX/<task>/<mode>/daemon.sock` is ~65 chars.
- **Files modified:** `internal/eval/sandbox/sandbox.go`
- **Commit:** 36066318

**2. [Rule 1 - Bug] checkNotSymlink uses os.Lstat not filepath.EvalSymlinks**

- **Found during:** Task 1 (TestSandboxLayout/Isolation/CloneRepo/etc. ALL FAIL)
- **Issue:** Original `checkNotSymlink` used `filepath.EvalSymlinks` and compared resolved path to input. On macOS, `/var/folders` IS a symlink to `/private/var/folders` — every test sandbox path triggered the check and NewSandbox always returned an error.
- **Fix:** Changed to `os.Lstat(path)` and checking `info.Mode()&os.ModeSymlink != 0`. This checks only whether the path node itself is a symlink, not whether parent directories contain symlinks. The T-67-01 threat (attacker places symlink at predictable tmpdir name) is still blocked; OS-level filesystem symlinks in parent paths are correctly ignored.
- **Files modified:** `internal/eval/sandbox/sandbox.go`
- **Commit:** 36066318

**3. [Rule 1 - Bug] Seconds watchdog needed context.WithCancel + goroutine instead of context.WithTimeout**

- **Found during:** Task 3 (TestWatchdogSecondsBreach FAIL — `Breach()` returned nil)
- **Issue:** Initial implementation used `context.WithTimeout` and a goroutine that set `w.breach` only AFTER `innerCtx.Done()` fired. Race condition: the test called `Breach()` immediately after receiving from `Done()`, but the goroutine had not yet set `w.breach`. `Breach()` returned nil.
- **Fix:** Changed to `context.WithCancel` + a timer goroutine. The goroutine now: (1) sets `w.breach` first, (2) calls `innerCancel()` second. This ensures `Breach()` is non-nil by the time `Done()` closes. The timer goroutine exits cleanly if the inner context is cancelled by another axis first.
- **Files modified:** `internal/eval/budget/budget.go`
- **Commit:** 9964fa4a

**4. [Rule 1 - Bug] YAML reader returned fmt.Errorf("EOF") instead of io.EOF**

- **Found during:** Task 3 (TestBudgetParseYAML FAIL — "yaml: input error: EOF")
- **Issue:** Custom `yamlReader` returned `fmt.Errorf("EOF")` as the error; yaml.v3 Decode did not treat this as a clean EOF and surfaced it as an error.
- **Fix:** Replaced custom `yamlReader` with `bytes.NewReader(data)` (stdlib). Added explicit empty-file check before decode: `if len(bytes.TrimSpace(data)) == 0 { return Default, nil }`.
- **Files modified:** `internal/eval/budget/budget.go`
- **Commit:** 9964fa4a

## Threat Mitigations Verified by Tests

| Threat | Test | Mitigation |
|--------|------|-----------|
| T-67-01: Symlink at tmpdir path | `TestSandboxRefusesSymlinkedTmpdir` | `checkNotSymlink` via `os.Lstat` |
| T-67-02: claude writes outside repo/ | `TestAgentBuildsArgv` (cmd.Dir = RepoFor) | `cmd.Dir` set in `Run` |
| T-67-03: mcp-config path injection | `TestMCPConfigRejectsRelativeBin` | `filepath.IsAbs` in `WriteMCPConfig` |
| T-67-Pitfall-1: env leakage | `TestAgentCleanEnv` | strict allowlist in `cleanEnv` |

## Known Stubs

None — all exported behavior is implemented and tested.

## Threat Flags

None — no new network endpoints or trust boundaries introduced. All network access is UDS within the per-mode tmpdir.

## Self-Check: PASSED

All 8 files exist. All 6 commit hashes verified in git log.
