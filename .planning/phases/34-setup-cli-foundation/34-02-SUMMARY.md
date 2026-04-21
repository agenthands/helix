---
phase: 34-setup-cli-foundation
plan: 02
subsystem: cli
tags: [setup, language-detection, health-check, tests, langregistry]
dependency_graph:
  requires: [setup-command, client-registrars, setup-output]
  provides: [language-detection, ls-preinstall, health-check, setup-tests]
  affects: [internal/cli/setup.go]
tech_stack:
  added: []
  patterns: [filepath-walkdir-detection, three-tier-installer, exec-lookpath-health]
key_files:
  created:
    - internal/cli/setup_detect.go
    - internal/cli/setup_health.go
    - internal/cli/setup_test.go
  modified:
    - internal/cli/setup.go
decisions:
  - "Lightweight health check via exec.LookPath (binary existence) rather than full daemon LSP initialize -- full health check deferred to Phase 35 HLTH-01 through HLTH-04"
  - "Skip-install passes entries through to health check for binary lookup verification"
  - "Dry-run returns all entries as-if successful for downstream display consistency"
metrics:
  duration: 287s
  completed: "2026-04-21"
  tasks_completed: 2
  tasks_total: 2
  files_created: 3
  files_modified: 1
---

# Phase 34 Plan 02: Language Detection, LS Pre-Install, Health Check & Tests Summary

Language detection via filepath.WalkDir + Registry.ByExtension for 52-language coverage, LS pre-installation via Installer.Resolve three-tier strategy, binary-existence health check, and 22 unit tests covering all setup components.

## Commits

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Implement language detection, LS pre-installation, and health check | `e1f44d84` | setup_detect.go, setup_health.go, setup.go |
| 2 | Comprehensive unit tests for all setup components | `daf0f332` | setup_test.go |

## What Was Built

### Language Detection (`internal/cli/setup_detect.go`)
- `detectLanguages()` walks project directory using `filepath.WalkDir`
- Skips hidden directories (`.` prefix), `node_modules`, `vendor`, `__pycache__`, `.git` per T-34-07
- Matches file extensions via `langregistry.Registry.ByExtension()` for 52-language coverage
- Deduplicates by language name, returns sorted entries
- Prints detected languages via SetupPrinter

### LS Pre-Installation (`internal/cli/setup_detect.go`)
- `preInstallLanguageServers()` calls `langregistry.Installer.Resolve()` per detected language
- Three-tier strategy: PATH lookup > managed download > helpful error
- Dry-run mode prints planned actions without executing
- Returns successfully resolved entries for downstream health check

### Health Check (`internal/cli/setup_health.go`)
- `runHealthCheck()` verifies LS binary availability via `exec.LookPath`
- Informational output to stderr via SetupPrinter (T-34-09), never blocking
- Dry-run shows planned verification count
- Empty entries handled gracefully

### Setup Orchestration Wiring (`internal/cli/setup.go`)
- After client registration: creates `langregistry.NewRegistry()`, calls `detectLanguages()`
- If `--skip-install` is false: creates Installer with `AutoInstall: true`, calls `preInstallLanguageServers()`
- Calls `runHealthCheck()` on installed entries
- Registry errors are non-fatal (prints warning, skips detection)
- Logger at warn level to avoid noise

### Unit Tests (`internal/cli/setup_test.go` -- 508 lines, 22 tests)
- **mergeJSONConfig**: new file creation, existing file merge preservation, VS Code `servers` key, non-MCP key preservation
- **removeFromJSONConfig**: entry removal, missing file handling
- **Registrars**: ClaudeCode dry-run, GeminiCLI dry-run, VSCode file write verification, JetBrains file write verification, ClaudeDesktop dry-run, Generic file output, Generic stdout capture
- **detectLanguages**: real registry against temp dirs with .go/.py/.ts files, hidden dir skipping, node_modules skipping
- **runHealthCheck**: dry-run, no-entries, known-binary (go)
- **resolveBinaryPath**: non-empty path, file existence
- **Setup command**: client listing (stderr capture), invalid client error, dry-run execution

## Threat Mitigations Applied

- T-34-07: Skip hidden dirs, node_modules, vendor, __pycache__ to avoid walking massive dependency trees. WalkDir does not follow symlinks by default.
- T-34-09: Health check output goes to stderr via SetupPrinter, no secrets in LS probe commands.

## Deviations from Plan

None -- plan executed exactly as written.

## Decisions Made

1. **Lightweight health check**: Used `exec.LookPath` for binary existence verification rather than attempting full LSP initialization. Full daemon-based health check is complex and deferred to Phase 35.
2. **Skip-install passes entries through**: When `--skip-install` is set, entries still pass to health check so users can see which LS binaries are available.
3. **Non-fatal registry errors**: If `langregistry.NewRegistry()` fails, setup prints a warning and returns success (registration still completed).

## Verification Results

1. `go build ./cmd/serena` -- PASS
2. `go vet ./internal/cli/...` -- PASS
3. `go vet ./...` -- PASS
4. `go test ./internal/cli/... -count=1` -- PASS (22 tests)
5. `go test ./... -count=1` -- PASS (full suite green)
6. `./serena setup --dry-run generic` -- shows detect + register + install + health steps
7. `./serena setup --dry-run --skip-install generic` -- shows detect + register, skips install, runs health

## Self-Check: PASSED

- [x] `internal/cli/setup_detect.go` exists
- [x] `internal/cli/setup_health.go` exists
- [x] `internal/cli/setup_test.go` exists (508 lines, > 200 minimum)
- [x] `internal/cli/setup.go` updated with langregistry wiring
- [x] Commit `e1f44d84` exists
- [x] Commit `daf0f332` exists
