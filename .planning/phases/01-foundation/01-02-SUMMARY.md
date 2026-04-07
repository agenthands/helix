---
phase: 01-foundation
plan: 02
subsystem: daemon-core
tags: [daemon, config, workspace, socket, shutdown, lifecycle]
dependency_graph:
  requires: [01-01]
  provides: [daemon-lifecycle, config-system, workspace-registry, session-state]
  affects: [01-03]
tech_stack:
  added: [koanf/v2, golang.org/x/sync, stretchr/testify]
  patterns: [errgroup-subsystem-orchestration, layered-config-loading, signal-first-shutdown]
key_files:
  created:
    - internal/config/config.go
    - internal/config/defaults.go
    - internal/config/loader.go
    - internal/config/loader_test.go
    - internal/workspace/key.go
    - internal/workspace/workspace.go
    - internal/daemon/daemon.go
    - internal/daemon/socket.go
    - internal/daemon/shutdown.go
    - internal/daemon/daemon_test.go
  modified:
    - internal/cli/root.go
    - go.mod
    - go.sum
decisions:
  - Used koanf v2 for layered config loading (4 layers: defaults, global, project, CLI)
  - Used errgroup for daemon subsystem orchestration
  - Used stretchr/testify for daemon test assertions
  - Short socket paths in tests to avoid macOS 104-char Unix socket limit
metrics:
  duration: 3min
  completed: "2026-04-07T13:19:21Z"
---

# Phase 01 Plan 02: Daemon Skeleton with Config, Workspace Registry, and Socket Listener

Persistent daemon with errgroup lifecycle, koanf 4-layer config, workspace registry keyed by repo+language+toolchain, Unix socket listener with stale cleanup, and two-phase graceful shutdown on SIGTERM/SIGINT.

## What Was Built

### Config System (internal/config/)
- **config.go**: SerenaConfig with DaemonConfig (socket, HTTP, shutdown timeout), LoggingConfig (format, level, dir), ProjectDefaults (contexts, modes)
- **defaults.go**: Built-in defaults including auto-computed socket path from UID
- **loader.go**: koanf-based 4-layer loading: defaults -> global ~/.serena/serena_config.yml -> project .serena/project.yml -> CLI overrides
- **loader_test.go**: Tests for defaults, CLI overrides, and project-overrides-global precedence

### Workspace Registry (internal/workspace/)
- **key.go**: WorkspaceKey (RepoRoot+Language+Toolchain) with SHA256-based hash for map keying
- **workspace.go**: Thread-safe Registry with ActivateWorkspace, GetWorkspace, RegisterSession, RemoveSession; SessionState tracks session ID, workspace key, mode, and profile

### Daemon Lifecycle (internal/daemon/)
- **daemon.go**: Daemon struct with config, logger, workspace registry, socket listener; Run() registers signal handlers FIRST (Pitfall 3), then starts errgroup subsystems
- **socket.go**: ensureSocket() probes existing sockets via net.DialTimeout to distinguish stale vs active; createSocketDir() with 0700 permissions
- **shutdown.go**: Two-phase shutdown: close listeners + clean socket file (Phase 2 will add LS worker drain)
- **daemon_test.go**: Tests for no-file, stale-file, active-daemon socket scenarios, plus full start/stop lifecycle

### CLI Integration (internal/cli/root.go)
- runDaemon() wired to --serve flag and mode=http
- slog JSON/text handler selection from --json flag
- Config Load() called with CLI socket/http-addr overrides

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | d9385a42 | Config system with layered loading and workspace/session types |
| 2 | 8e220c83 | Daemon lifecycle with socket listener and graceful shutdown |

## Verification Results

- `go build ./...` passes
- `go test ./internal/config/... -count=1` passes (3 tests)
- `go test ./internal/daemon/... -count=1` passes (4 tests)
- Binary compiles with daemon wired into CLI

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Unix socket path length on macOS**
- **Found during:** Task 2 test execution
- **Issue:** macOS has a 104-character limit for Unix socket paths; t.TempDir() paths exceeded this limit causing "bind: invalid argument"
- **Fix:** Added shortSocketPath() helper using /tmp/serena-test-* for short paths with cleanup
- **Files modified:** internal/daemon/daemon_test.go
- **Commit:** 8e220c83

## Known Stubs

None -- all code is functional, not stubbed. Phase 2 items are documented as comments (LS workers, dirty buffers) but do not affect current plan goals.

## Self-Check: PASSED
