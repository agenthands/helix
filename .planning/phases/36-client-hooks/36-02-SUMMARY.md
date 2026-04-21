---
phase: 36-client-hooks
plan: "02"
subsystem: cli-hooks
tags: [grpc, cli, lifecycle, proto, daemon]
dependency_graph:
  requires: ["36-01"]
  provides: ["activate-command", "deactivate-command", "grpc-activate-rpc", "grpc-deactivate-rpc", "exported-connect-or-start-daemon"]
  affects: ["internal/daemon/daemon.go", "api/proto/serena/v1/ipc.proto", "internal/forwarder/dial.go", "internal/cli/root.go"]
tech_stack:
  added: []
  patterns: ["gRPC RPC extension", "CLI subcommand with daemon auto-start", "silent failure tolerance"]
key_files:
  created:
    - internal/cli/activate.go
    - internal/cli/deactivate.go
    - internal/cli/activate_test.go
    - internal/cli/deactivate_test.go
  modified:
    - api/proto/serena/v1/ipc.proto
    - api/proto/serena/v1/ipc.pb.go
    - api/proto/serena/v1/ipc_grpc.pb.go
    - internal/daemon/daemon.go
    - internal/forwarder/dial.go
    - internal/forwarder/forwarder.go
    - internal/cli/root.go
decisions:
  - "Export ConnectOrStartDaemon rather than duplicating auto-start logic in CLI"
  - "Deactivate uses best-effort daemon notification with silent failure on missing daemon (D-15)"
  - "30s timeout for activate covers cold-start LS installation; 10s for deactivate is sufficient"
metrics:
  duration: 242s
  completed: "2026-04-21"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 11
---

# Phase 36 Plan 02: Activate/Deactivate CLI Commands Summary

gRPC protocol extension with ActivateWorkspace/DeactivateWorkspace RPCs, daemon handlers with path validation, exported ConnectOrStartDaemon helper, and activate/deactivate CLI commands for Claude Code session lifecycle hooks.

## Task Results

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Proto extension, generated code, daemon handler, exported dial helper | 8afe8ff2 | ipc.proto, ipc.pb.go, ipc_grpc.pb.go, daemon.go, dial.go, forwarder.go |
| 2 | Activate and deactivate CLI commands, root wiring, tests | 91a62766 | activate.go, deactivate.go, activate_test.go, deactivate_test.go, root.go |

## What Was Built

1. **Proto extension**: Added `ActivateWorkspace` and `DeactivateWorkspace` RPCs to `ForwarderService` with `ActivateRequest/Response` and `DeactivateRequest/Response` message types. Regenerated Go code via protoc.

2. **Daemon handlers**: `ActivateWorkspace` handler validates path (filepath.Abs + os.Stat + IsDir) then delegates to kernel.ActivateWorkspace. `DeactivateWorkspace` handler acknowledges deactivation without tearing down kernel workspace (other sessions may be using it).

3. **Exported ConnectOrStartDaemon**: Renamed from unexported `connectOrStartDaemon` in forwarder/dial.go for reuse by CLI commands. Updated call site in forwarder.go.

4. **serena activate**: CLI command that auto-starts daemon via ConnectOrStartDaemon, resolves workspace path to absolute, calls ActivateWorkspace RPC with 30s timeout, prints activation status to stdout.

5. **serena deactivate**: CLI command that cleans up `.serena/session-stats.json`, then tries best-effort daemon notification. Silently succeeds if daemon is not running (D-15 requirement).

6. **Root wiring**: activate and deactivate registered as subcommands in root.go (nudge was already registered).

7. **Tests**: 5 tests covering command structure, flags, session file cleanup, and missing-daemon silent success pattern.

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

- `go build ./...` -- passes (only pre-existing swift binding warning)
- `go vet ./...` -- passes
- `go test ./internal/cli/... -run "TestNewActivate|TestNewDeactivate|TestDeactivate_"` -- 5/5 pass
- `go test ./...` -- all pass except pre-existing `TestBenchToolsManifestMatchesRegistry` (tool count 42 vs expected 41, predates this plan)

## Known Stubs

None.

## Threat Mitigations Applied

| Threat ID | Mitigation | Location |
|-----------|-----------|----------|
| T-36-06 | filepath.Abs in activate.go + filepath.Abs + os.Stat + IsDir in daemon handler | internal/cli/activate.go, internal/daemon/daemon.go |
| T-36-07 | filepath.Abs in deactivate.go before constructing statsPath | internal/cli/deactivate.go |
| T-36-09 | os.Stat + IsDir validation in daemon handler before kernel call | internal/daemon/daemon.go |
