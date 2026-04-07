---
phase: 02-code-intelligence-kernel
plan: 05
subsystem: diagnostics
tags: [diagnostics, code-actions, formatting, lsp, mcp-tools]
dependency_graph:
  requires: [02-03]
  provides: [diag-store, code-actions, format-code]
  affects: [02-06]
tech_stack:
  added: []
  patterns: [notification-subscription, bottom-up-edit-application, lease-provider-abstraction]
key_files:
  created:
    - internal/kernel/diag/subscriber.go
    - internal/kernel/diag/actions.go
    - internal/kernel/diag/format.go
    - internal/kernel/diag/tools.go
    - internal/kernel/diag/diag_test.go
  modified: []
decisions:
  - LeaseProvider func type to decouple tools from kernel dependency
  - HandleNotification on DiagnosticStore for JSON-RPC callback registration
  - Atomic file write via temp+rename for format edits
metrics:
  duration: 3min
  completed: "2026-04-07T20:33:25Z"
---

# Phase 02 Plan 05: Diagnostics Tools Summary

Diagnostics subscription, code action forwarding, and LSP formatting with 3 MCP tools and 9 passing tests.

## What Was Built

### DiagnosticStore (subscriber.go)
- Thread-safe store keyed by document URI
- `HandlePublishDiagnostics` replaces diagnostics per URI and wakes waiters
- `HandleNotification` JSON-RPC callback dispatches publishDiagnostics
- `WaitForDiagnostics` blocks with timeout using channels (essential for post-edit verification)
- `GetDiagnostics` / `GetAllDiagnostics` / `Clear` for retrieval and management

### Code Actions (actions.go)
- `GetCodeActions` sends textDocument/codeAction with position/range
- `ApplyCodeAction` applies workspace edits via workspace/applyEdit
- `CodeActionResult` simplified view with Title, Kind, IsPreferred, Edit

### Formatting (format.go)
- `FormatDocument` sends textDocument/formatting via worker lease
- `ApplyFormatEdits` applies edits bottom-up (reverse order) to preserve line numbers
- Atomic file write via temp file + rename

### MCP Tools (tools.go)
- `get_diagnostics` (DGN-01): Returns severity:line:col: message formatted output
- `get_code_actions` (DGN-02): Returns available actions for position/range
- `format_code` (DGN-03): Formats file via LSP and writes result atomically
- `LeaseProvider` func type decouples tools from kernel for testability

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 2a5c1a3d | Diagnostics subscriber, code actions, and formatting |
| 2 | cc3ba067 | MCP tool registration for diagnostics |

## Test Results

9 tests, all passing:
- TestDiagnosticStore_StoreAndRetrieve
- TestDiagnosticStore_GetAll
- TestDiagnosticStore_Clear
- TestDiagnosticStore_WaitForDiagnostics
- TestDiagnosticStore_WaitForDiagnostics_Timeout
- TestDiagnosticStore_WaitForDiagnostics_AlreadyPresent
- TestApplyFormatEdits_ReverseOrder
- TestCodeActionResult_Parsing
- TestHandleNotification_Dispatch

## Deviations from Plan

### Minor Adjustments

**1. [Rule 2] Added LeaseProvider abstraction**
- Plan specified `RegisterTools(kernel *Kernel, diagStore *DiagnosticStore, registry *mcp.ToolRegistry)` signature
- Changed to `RegisterTools(server *mcp.SerenaMCPServer, store *DiagnosticStore, workspaceRoot func() string, leaseFn LeaseProvider)` to match the established fileops pattern and decouple from kernel type
- This avoids circular import (diag importing kernel) and follows existing convention

**2. [Rule 2] Added HandleNotification method on DiagnosticStore**
- Plan mentioned notification handler registration but did not specify the exact callback signature
- Added `HandleNotification(method string, params json.RawMessage)` matching jsonrpc.NotificationFunc for direct use as Conn.OnNotification callback

## Known Stubs

None - all data paths are fully wired.
