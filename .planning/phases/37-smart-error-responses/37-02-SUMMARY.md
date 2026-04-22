---
phase: 37-smart-error-responses
plan: "02"
subsystem: mcp-runtime
tags: [middleware, error-enrichment, daemon-wiring]
dependency_graph:
  requires: [37-01]
  provides: [suggestion-middleware-active]
  affects: [internal/mcp/server.go, internal/daemon/daemon.go]
tech_stack:
  added: []
  patterns: [tool-schema-collection, middleware-chain-ordering]
key_files:
  created: []
  modified:
    - internal/mcp/server.go
    - internal/daemon/daemon.go
decisions:
  - Store *mcpsdk.Tool pointers directly (SDK mutates InputSchema in place via jsonschema.ForType)
  - SuggestionMiddleware installed after TelemetryMiddleware so errors are enriched before telemetry classifies
metrics:
  duration: 2m30s
  completed: "2026-04-22T17:55:05Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
---

# Phase 37 Plan 02: Wire Suggestion Middleware Summary

Wire suggestion middleware into daemon startup for production error enrichment with did-you-mean parameter corrections.

## One-liner

SuggestionMiddleware wired into daemon step 14b with schema map built from all registered tool pointers collected at startup.

## What was done

### Task 1: Add tool schema collection to SerenaMCPServer (72bf0f7f)

Added `toolSchemas []*mcpsdk.Tool` field to `SerenaMCPServer` struct and modified all tool registration paths to store tool pointers:

- `AddTool()` -- dynamic runtime registration
- `AddSkillTool()` -- skill-provided tools (memory, workflow)
- `registerPingTool()`, `registerEchoTool()`, `registerActivateProjectTool()` -- constructor tools

Added `CollectToolSchemas()` method that returns the stored slice for schema introspection at daemon startup.

### Task 2: Wire SuggestionMiddleware in daemon startup (6a78ebd3)

Added step 14b in `daemon.go` after the existing `InstallMiddleware` call:
1. `BuildToolSchemaMap(mcpServer.CollectToolSchemas())` -- builds schema map from all registered tools
2. `InstallSuggestionMiddleware(mcpServer.SDK(), suggestionSchemaMap, logger)` -- installs middleware

Ordering ensures schema map is built after all tools are registered (steps 10-11) and SuggestionMiddleware sits between the tool handler and TelemetryMiddleware in the response path.

## Deviations from Plan

None -- plan executed exactly as written.

## Verification

- `go build ./cmd/serena` -- binary compiles (exit 0)
- `go vet ./...` -- clean (exit 0)
- `go test ./... -count=1 -timeout=300s` -- all packages pass (exit 0)

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | 72bf0f7f | feat(37-02): add tool schema collection to SerenaMCPServer |
| 2 | 6a78ebd3 | feat(37-02): wire SuggestionMiddleware in daemon startup |
