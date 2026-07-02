# Coding Conventions

**Analysis Date:** 2026-07-01

## Naming Patterns

**Packages:**
- Short, lowercase, single-word, no underscores: `daemon`, `kernel`, `mcp`,
  `semantic`, `langregistry`, `repomap`, `lspool`, `obs`, `errors`.
- Package name matches the directory; subsystems nest by concern
  (`internal/semantic/store`, `internal/kernel/symbols`).

**Files:**
- lowercase, `snake_case` where multi-word: `daemon.go`, `middleware.go`,
  `lazy_init.go`, `profile_enforce.go`, `edge_kind_surface.go`.
- Tests are `*_test.go`; real-binary end-to-end tests are `*_e2e_test.go`
  (`internal/cli/*_e2e_test.go`, driven by `HELIX_BIN`).
- Platform-conditional files use the Go suffix convention: `pressure_linux.go`,
  `pressure_darwin.go`, `swap_windows.go`; build-tag conditionals are rare
  (`internal/semantic/store/duckdb.go` `//go:build !(windows && arm64)`).

**Exported symbols:**
- PascalCase for exported identifiers: `Daemon`, `SerenaMCPServer`, `GrammarRegistry`,
  `SemanticLookup`, `InstallMiddleware`. `SerenaMCPServer` is a deliberately retained
  lineage identifier (Phase 52-03), not a stale name.
- Interfaces are named for behavior, often `-er`: `ToolProvider`, `WorkflowProvider`,
  `LeaseProvider`, `SkillToolExecutor`, `ClientRegistrar`.
- Argument structs for MCP tools use a `<Tool>Args` suffix: `GoToDefinitionArgs`,
  `FindReferencesArgs` (`internal/kernel/symbols/tools.go`).

**Unexported symbols:**
- camelCase: `newDaemon`, `resolveAllowedToolsForMode`, `defaultSessionRunner`,
  `getSessionFn`. Struct fields unexported when internal (`slogHandler`, `metrics`,
  `tracerProvider` in `obs.Provider`).

**Constants / enums:**
- Closed enums are typed string aliases with grouped `const` blocks:
  `Kind` (`internal/errors/kinds.go`), `Freshness` / `ChangeSource`
  (`internal/semantic/`). Sentinel errors are `Err<Kind>` vars
  (`ErrNotFound`, `ErrUnsupported`).

## Code Style

**Formatting:**
- `gofmt` is the sole formatter (`make fmt` / `gofmt -w .`); tabs for indentation,
  no configurable line length. No third-party formatter.

**Vetting:**
- `go vet ./...` plus 7 custom `cmd/vet-*` singlechecker analyzers, all run by
  `make vet` (`Makefile` `vet` target):
  `vet-noduckdb` (confine the duckdb-go import to `internal/semantic/store`),
  `vet-nokernel2semantic` and `vet-nosemantic2kernel` (pin the kernel↔semantic import
  boundary in both directions; kernel may import only `internal/semantic/integ`),
  `vet-compact-uses-store` (compact must go through the store, not duckdb directly),
  `vet-ablation-leakage` and `vet-bench-rag-leakage` (bench arms must not import
  disabled subsystems), and `vet-tools-quarantine` (nothing outside `tools/` may
  import the dev-time `tools/` tree). `verify-no-docker-sdk` bans the Docker Go SDK.

**Testing:**
- `make test` = `make vet` + `go test`; `testify` assertions; `testdata/` fixtures per
  package; real-binary E2E tests gated on `HELIX_BIN`.

## Import Organization

**Grouping (gofmt/goimports order):**
1. Standard library.
2. Third-party modules.
3. Local `github.com/agenthands/helix/...` packages.

Groups separated by blank lines; alphabetical within each group. Example from
`internal/kernel/symbols/tools.go`:

```go
import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)
```

**Aliases:**
- `serr` for `internal/errors` (avoids shadowing stdlib `errors`; convention pinned in
  `kinds.go`). `mcpsdk` for the upstream MCP SDK to distinguish it from the local
  `internal/mcp`. Blank imports for driver/init side effects
  (`_ "modernc.org/sqlite"`, `_ "github.com/duckdb/duckdb-go/v2"`, and the daemon's
  `imports.go` skill blank-imports).

## Error Handling

**Typed error taxonomy (`internal/errors/`):**
- `Error{Kind, Message, Tool, Detail, cause}` with a closed `Kind` enum. Construct
  with `serr.New(kind, msg)` / `serr.Wrap(kind, msg, cause)`; builder chaining via
  `WithTool` / `WithDetail`.
- Match by kind, not string: `errors.Is(err, serr.ErrNotFound)`. `Error.Is` compares
  `Kind`; `Unwrap` preserves the chain; `MarshalJSON` renders errors onto the MCP wire.
- Never return a typed nil `*Error` as `error` — return `nil` directly (documented
  pitfall in `errors.go`).
- Wrapping with `fmt.Errorf("...: %w", err)` at package boundaries where a bare cause
  suffices (e.g. `internal/config/loader.go`).
- Domain extension: `NewGuardrailViolation` / `AsGuardrailViolation` carry a typed
  `GuardrailViolationDetail`; disabled subsystems reuse `Unsupported` with a greppable
  `subsystem_disabled:` message prefix.

## Logging

**Framework:** `log/slog` (standard library) — no third-party logging framework.
- Loggers are dependency-injected (`*slog.Logger` passed into `New`, middleware
  installers, workers), not package globals.
- `internal/cli/root.go` `newLogger` builds a text or JSON handler to stderr and wraps
  it with `obs.NewContextHandler` so traced requests emit `trace_id`/`span_id`.
- Structured key/value attributes; level via `slog.HandlerOptions{Level: ...}`.

## Comments

**Doc comments:**
- Standard Go doc comments: every exported identifier has a `//` comment beginning with
  the identifier name (`// Daemon is the persistent supervisor...`,
  `// InstallMiddleware wires...`).
- Package docs live in `doc.go` (or atop the primary file) and state invariants,
  allowed/forbidden imports, and design rules (`internal/semantic/store/doc.go`,
  `internal/semantic/scheduler/doc.go`, `internal/obs/obs.go`).
- Inline comments explain non-obvious invariants and cite the governing phase/decision
  (e.g. `// LazyInit MUST run first ...`, `D-12`, `Phase 91 SEC-01`), and flag
  concurrency/pitfall hazards.

## Function Design

**Signatures:**
- `context.Context` is the first parameter on any call that does I/O or can be
  cancelled (`Resolve(ctx, entry)`, `Probe(ctx)`, `StreamMCP(stream)`).
- Multiple returns with a trailing `error`; named returns only when they aid clarity
  (`Resolve(...) (command string, args []string, err error)`).
- Options passed as structs (`InstallerConfig`, `SkillDeps`, `RegistrationConfig`,
  `MiddlewareDeps`) rather than long positional lists.

**Seams for testing:**
- Overridable function-typed package vars are the test seam (`runDaemonFn = runDaemon`
  in `root.go`, `sessionRunner` in `daemon.go`) so tests substitute fakes without
  spawning real processes.

## Module Design

**Boundaries:**
- `internal/` is the private surface; `api/proto/` and `protocol/` hold generated
  wire/LSP types; `cmd/` holds binary entrypoints; `bench/` and `tools/` are dev-time
  and quarantined from the shipped binary (`vet-tools-quarantine`,
  `vet-*-leakage`).
- Cross-subsystem coupling flows through narrow, types-only seams: the kernel reaches
  the semantic engine only via `internal/semantic/integ` (`SemanticLookup`), never
  through the store or duckdb.
- Third-party-dependency blast radius is contained by package ownership: `internal/obs`
  is the sole home of prometheus/otel; `internal/semantic/store` is the sole home of
  duckdb-go. Vet gates enforce both.

**Registration:**
- Caddy-style plugin registration: a skill package's `init()` calls
  `skill.Register(&MySkill{})` (`internal/skill/memory/skill.go:24`,
  `internal/skill/workflow/skill.go:24`, `internal/skill/semantic/skill.go:72`); the
  daemon blank-imports skill packages (`imports.go`) and calls `skill.InitAll(deps)`,
  then `skill.ToolProviders()` / `skill.WorkflowProviders()`.

## Struct Usage

*(This section covers Go structs — the analogous value-carrier shape.)*

**Pattern (MCP tool arg struct, `internal/kernel/symbols/tools.go`):**
```go
// GoToDefinitionArgs is the input schema for the go_to_definition tool.
type GoToDefinitionArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (1-indexed, matches editor display)"`
	Col  int    `json:"column" jsonschema:"Column number (1-indexed, matches editor display)"`
}
```

- Exported fields carry `json:` tags (wire field names) and `jsonschema:` tags (the
  agent-facing parameter descriptions that drive the tool schema); `omitempty` for
  optional inputs.
- Config structs mirror koanf keys one tag per leaf (`koanf:"enabled"` in
  `internal/semantic/config.go`), so a SPEC key rename must change the tag in lockstep.
- Structs (never interfaces) are used for extensible value carriers like `obs.Provider`
  so methods can be added without breaking call sites; typed IDs are single-field
  aliases (`type SnapshotID uint64`, `internal/semantic/types.go`).
- Compile-time interface guards: `var _ health.SemanticIndexAccessor =
  (*daemonSemIndexAccessor)(nil)` (`internal/daemon/daemon.go`).

## Code Organization Examples

**Middleware installer (`internal/mcp/`):** each middleware is a
`func Install<Name>Middleware(server *mcpsdk.Server, ...)` that calls
`server.AddReceivingMiddleware(...)`. Because the SDK composes middleware LIFO, the
daemon installs in the order Telemetry+ProfileFilter → Suggestion → Guardrail →
ProfileEnforce → LazyInit so execution runs LazyInit-first; install-order comments
document the invariant (`lazy_init.go:106-112`, `profile_enforce.go:16-23`).

**Generated-code discipline:** generated files carry a `// Code generated by ...; DO
NOT EDIT.` header and are regenerated, never hand-edited:
- `internal/cli/verbs_gen.go` (`helix-cligen`) — the frozen verb→tool catalog; the
  `verify-cligen` drift gate hard-fails if it diverges from the live registry.
- `protocol/gen/*.go` (`cmd/lspgen` from LSP metaModel.json 3.17.0).
- `api/proto/serena/v1/ipc.pb.go` / `ipc_grpc.pb.go` (`protoc-gen-go`; `make proto`).
- Companion drift gates: `verify-docs` (README tool table) and `verify-reference`
  (`internal/cli/skills/helix/reference.md`) keep hand-written docs in lockstep with
  the registry.

**Test structure:** `testify` (`assert`/`require`), table-driven subtests, `testdata/`
fixtures per package, and real-binary E2E tests (`*_e2e_test.go`, `HELIX_BIN`) for the
CLI surface.

---

*Convention analysis: 2026-07-01*
