---
phase: 02-code-intelligence-kernel
plan: 01
subsystem: protocol
tags: [lsp, codegen, json-rpc, metamodel, go-generate]

requires:
  - phase: 01-foundation
    provides: Go module structure, go.mod, cmd/ and internal/ layout

provides:
  - LSP 3.17 protocol types generated from metaModel.json (324 structs, 37 enums)
  - Or_X_Y union types with typed JSON marshal/unmarshal and accessors
  - Client/Server interfaces with all LSP request and notification methods
  - Content-Length framed JSON-RPC 2.0 codec for LS stdio communication
  - Bidirectional Conn with Call, Notify, Listen, and session-prefixed IDs

affects: [02-02, 02-03, 02-04, 02-05, 02-06]

tech-stack:
  added: [metaModel.json (LSP 3.17.0), go/format, text generation]
  patterns: [go:generate codegen from JSON schema, Content-Length framed codec, session-prefixed request IDs]

key-files:
  created:
    - cmd/lspgen/main.go
    - cmd/lspgen/tables.go
    - cmd/lspgen/typenames.go
    - cmd/lspgen/output.go
    - protocol/metaModel.json
    - protocol/generate.go
    - protocol/gen/tsprotocol.go
    - protocol/gen/tsclient.go
    - protocol/gen/tsserver.go
    - protocol/gen/tsjson.go
    - protocol/patch/rename_params.go
    - protocol/patch/compatibility.go
    - internal/kernel/jsonrpc/message.go
    - internal/kernel/jsonrpc/codec.go
    - internal/kernel/jsonrpc/conn.go
    - internal/kernel/jsonrpc/codec_test.go
  modified: []

key-decisions:
  - "Used named type (not alias) for array-based type aliases like DocumentSelector since Go type aliases require identifiers"
  - "Simplified X|null unions to direct Go type (no Or_ wrapper) since Go pointers handle nullable semantics"
  - "Deduplicated literal types in unions since multiple anonymous literals resolve to map[string]interface{}"
  - "Custom thin JSON-RPC (~300 lines) over unmaintained go.lsp.dev/jsonrpc2 for daemon multiplexing needs"
  - "Session-prefixed IDs (sess-abc:42) to avoid collision between sessions sharing LS workers"

patterns-established:
  - "go:generate codegen: metaModel.json -> cmd/lspgen -> protocol/gen/ (4 files)"
  - "Or_X_Y union pattern: Value interface{} + typed As/Set accessors + custom JSON marshal"
  - "Content-Length framed codec: ReadMessage/WriteMessage for LSP stdio"
  - "Session-prefixed JSON-RPC IDs for multi-session LS worker sharing"

requirements-completed: [LNG-04]

duration: 7min
completed: 2026-04-07
---

# Phase 2 Plan 01: LSP Protocol Types and JSON-RPC Codec Summary

**Full LSP 3.17 type generation from metaModel.json (324 structs, 216 union types) plus Content-Length framed JSON-RPC 2.0 codec with session-prefixed ID routing**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-07T20:04:29Z
- **Completed:** 2026-04-07T20:12:14Z
- **Tasks:** 2
- **Files created:** 16

## Accomplishments
- Built cmd/lspgen/ code generator that reads the official LSP 3.17 metaModel.json and produces 4 Go files totaling 9,032 lines
- Generated all 324 LSP structures, 37 enumerations, 67 request definitions, 26 notification definitions, and 21 type aliases
- 216 Or_X_Y union types with custom MarshalJSON/UnmarshalJSON and typed accessors (AsTextEdit, SetTextEdit, etc.)
- Built thin JSON-RPC 2.0 codec with Content-Length framing, bidirectional Conn with Call/Notify/Listen, and session-prefixed IDs
- 13 passing tests for the JSON-RPC codec covering all message types and connection behaviors

## Task Commits

Each task was committed atomically:

1. **Task 1: LSP metamodel codegen** - `3380a555` (feat)
2. **Task 2: JSON-RPC 2.0 codec** - `31da6d5f` (feat)

## Files Created/Modified
- `cmd/lspgen/main.go` - Generator entry point: parse metaModel.json, drive 4-file output
- `cmd/lspgen/tables.go` - Type name fixups, field name overrides, base type mappings
- `cmd/lspgen/typenames.go` - Go type resolution, Or_ naming heuristics, deduplication
- `cmd/lspgen/output.go` - go/format formatting and file writing with debug output
- `protocol/metaModel.json` - Official LSP 3.17 metamodel (pinned)
- `protocol/generate.go` - go:generate directive for the codegen pipeline
- `protocol/gen/tsprotocol.go` - 5,180 lines: all LSP struct types, enums, type aliases, Or_ union declarations
- `protocol/gen/tsclient.go` - Client/server-to-client interfaces with 59+ method signatures
- `protocol/gen/tsserver.go` - Server interface, notification handler, dispatch table
- `protocol/gen/tsjson.go` - 3,095 lines: MarshalJSON/UnmarshalJSON for 216 union types
- `protocol/patch/rename_params.go` - Documents RenameParams.newName required-vs-optional quirk
- `protocol/patch/compatibility.go` - Compatibility notes for metamodel-to-reality discrepancies
- `internal/kernel/jsonrpc/message.go` - Request, Response, Notification, ResponseError types
- `internal/kernel/jsonrpc/codec.go` - Content-Length framed ReadMessage/WriteMessage
- `internal/kernel/jsonrpc/conn.go` - Bidirectional Conn with Call, Notify, Listen, session-prefixed IDs
- `internal/kernel/jsonrpc/codec_test.go` - 13 tests covering codec, messages, connection behavior

## Decisions Made
- Used named types (not aliases) for array-based type aliases (DocumentSelector = []DocumentFilter) since Go type aliases require identifiers on both sides
- Simplified X|null union types to direct Go types rather than Or_ wrappers, leveraging Go pointer semantics for nullable fields
- Deduplicated literal types within unions since multiple anonymous literal types all resolve to map[string]interface{}
- Built custom JSON-RPC implementation (~300 lines) instead of using unmaintained go.lsp.dev/jsonrpc2 (last updated 2022)
- Session-prefixed IDs (e.g., "sess-abc:42") to avoid collision when multiple sessions share an LS worker

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Invalid Go type alias targets**
- **Found during:** Task 1 (generator output)
- **Issue:** LSPArray ([]interface{}), LSPAny (interface{}), LSPObject (map[string]interface{}) produced invalid Go type alias declarations
- **Fix:** Added isValidAliasTarget check; non-identifier targets emitted as named types instead of aliases
- **Files modified:** cmd/lspgen/main.go, cmd/lspgen/typenames.go
- **Verification:** go generate + go build succeed

**2. [Rule 1 - Bug] Or_ type names with invalid characters**
- **Found during:** Task 1 (generator output)
- **Issue:** Or type names contained [], {}, map[] characters from Go type strings when array/map/interface types appeared in unions
- **Fix:** Rewrote typeShortName to produce clean identifier fragments (e.g., "Array" not "[]interface{}")
- **Files modified:** cmd/lspgen/typenames.go
- **Verification:** All Or_ names are valid Go identifiers

**3. [Rule 1 - Bug] Duplicate accessor methods on union types**
- **Found during:** Task 1 (compilation)
- **Issue:** Multiple literal types in a union all resolved to map[string]interface{}, causing duplicate AsMapStringAny/SetMapStringAny methods
- **Fix:** Added deduplication of Go types before generating unmarshal attempts and accessor methods
- **Files modified:** cmd/lspgen/main.go, cmd/lspgen/typenames.go
- **Verification:** go build succeeds with no redeclaration errors

---

**Total deviations:** 3 auto-fixed (3 Rule 1 bugs)
**Impact on plan:** All auto-fixes necessary for generated code to compile. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all generated types are complete from metaModel.json and codec is fully functional.

## Next Phase Readiness
- Protocol types ready for LS worker pool (Plan 03) to use for request/response typing
- JSON-RPC codec ready for LS child process stdio communication
- `go generate ./protocol/...` can be re-run whenever metaModel.json is updated

---
*Phase: 02-code-intelligence-kernel*
*Completed: 2026-04-07*
