# Contributing to Serena

Thank you for your interest in contributing to Serena! We welcome contributions that improve and extend the project.

## Scope of Contributions

The following types of contributions can be submitted directly via pull requests:

- Isolated additions that extend Serena along existing lines (e.g., adding support for a new language server)
- Small bug fixes
- Documentation improvements

For larger changes, please open an issue first to discuss your ideas with the maintainers.

Every PR should cover a single logical change or a set of closely related changes.

## Prerequisites

- **Go 1.25+** (see `go.mod` for the exact version)
- **Language servers** for testing: at minimum, `gopls` for Go fixture tests. Tests for other languages require their respective language servers (e.g., `pyright` for Python, `typescript-language-server` for TypeScript).
- **protoc + protoc-gen-go + protoc-gen-go-grpc** (only if modifying gRPC protos in `api/proto/`)

## Development Commands

| Command | Description |
|---------|-------------|
| `go build ./cmd/serena` | Build the serena binary |
| `go test ./...` | Run all tests |
| `go vet ./...` | Run static analysis |
| `gofmt -w .` | Format code |
| `make build` | Build via Makefile |
| `make test` | Run tests via Makefile |
| `make vet` | Run vet via Makefile |
| `make fmt` | Format via Makefile |
| `make docs` | Regenerate tool and language tables in README.md |
| `make proto` | Regenerate gRPC protobuf code (only if modifying `api/proto/`) |

**Always run `go vet` and `go test` before submitting a PR.**

## Project Structure

Serena uses a 4-layer architecture shipping as a single Go binary:

- `cmd/serena/` -- CLI entry point
- `internal/mcp/` -- MCP runtime, smart error suggestions, lazy workspace init (Layer 0)
- `internal/daemon/` -- Persistent supervisor daemon (Layer 0)
- `internal/forwarder/` -- Stdio-to-gRPC proxy (Layer 0)
- `internal/cli/` -- CLI commands: setup, status, activate, deactivate, nudge
- `internal/kernel/` -- Code intelligence kernel (Layer 1)
- `internal/kernel/lspool/` -- LS worker pool (share-until-dirty, adaptive TTL, circuit breaking)
- `internal/kernel/symbols/` -- 9 symbol retrieval tools
- `internal/kernel/edit/` -- 6 symbol editing tools (tree-sitter body surgery)
- `internal/kernel/fileops/` -- 7 file operation tools (includes fuzzy_edit)
- `internal/kernel/diag/` -- 3 diagnostic tools
- `internal/kernel/health/` -- get_health tool
- `internal/kernel/help/` -- get_tool_help tool
- `internal/kernel/jsonrpc/` -- Custom JSON-RPC 2.0 codec for LS communication
- `internal/fuzzy/` -- Fuzzy editing strategies (whitespace-normalized, indentation-flexible)
- `internal/repomap/` -- RepoMap subsystem (PageRank-based context selection)
- `internal/treesitter/` -- Tree-sitter grammar integration
- `internal/skill/` -- Skill plugin system, Caddy-style init() registration (Layer 2)
- `internal/skill/memory/` -- 7 memory tools
- `internal/skill/repomap/` -- 2 RepoMap tools (get_repo_map, get_context)
- `internal/skill/workflow/` -- 2 workflow tools (onboard_project, prepare_for_new_conversation)
- `internal/langregistry/` -- 52-language registry with YAML override (Layer 2)
- `internal/memory/` -- Memory store, FTS5 index, fsnotify watcher
- `internal/profile/` -- 5 agent profiles (Layer 3)
- `internal/config/` -- 4-layer configuration (Layer 3)
- `internal/errors/` -- Structured error types
- `internal/obs/` -- Observability (metrics, tracing, admin listener)
- `internal/degrade/` -- Graceful degradation
- `internal/workspace/` -- Workspace key and state
- `api/proto/serena/v1/` -- gRPC IPC definitions
- `protocol/gen/` -- Generated LSP 3.17 types
- `test/harness/` -- Test harness: Runner, tool helpers, golden file comparison, fixtures
- `test/oracle/` -- Oracle test suite (6 layers: protocol, contract, runtime, scenario, llm, judge)
- `test/integration/` -- MCP round-trip integration tests
- `test/bench/` -- Benchmark suite with baselines

## Running Integration Tests

The integration test harness lives in `test/integration/`. It starts a real Serena daemon with a Go fixture project and exercises MCP round-trips via stdio transport against live language servers.

Run all integration tests:

```sh
go test ./test/integration/ -v -timeout 120s
```

Run a specific test:

```sh
go test ./test/integration/ -run TestSymbolRetrieval -v
```

Key details:

- The harness uses the `testing.TB` interface, shared by both integration tests and benchmarks.
- Tests exercise MCP round-trips against live language servers.
- Go fixtures are in `test/integration/`, with additional language fixtures available for Python, TypeScript, Java, and Rust.
- **Requirement:** Integration tests require `gopls` installed. Tests for other languages require their respective language servers.

## Oracle Test Suite

The oracle test suite in `test/oracle/` provides structured verification across 6 layers:

| Layer | Package | Purpose |
|-------|---------|---------|
| protocol | `test/oracle/protocol/` | MCP handshake, reconnect, session isolation, smoke tests, tool listing |
| contract | `test/oracle/contract/` | Schema validation, golden output comparison, error contracts, selectability |
| runtime | `test/oracle/runtime/` | Degraded start, pool stress, shutdown ordering, deferred errors |
| scenario | `test/oracle/scenario/` | Multi-language end-to-end scenarios (Go, Python, TS, Java, Rust, PHP, C++, JS, Swift, and more) |
| llm | `test/oracle/llm/` | LLM-driven disambiguation, interpretation, selection |
| judge | `test/oracle/judge/` | Automated scoring with rubrics and aggregation |

Run all oracle tests:

```sh
go test ./test/oracle/... -v -timeout 300s
```

Run a specific layer:

```sh
go test ./test/oracle/scenario/ -v -timeout 120s
```

The test harness (`test/harness/`) provides shared infrastructure: `Runner` (starts daemon + exercises MCP round-trips), tool call helpers, golden file comparison, and fixture management.

## Running Benchmarks

The benchmark suite lives in `test/bench/`. It measures tool response times, LSP indexing throughput, and memory profiles.

Run all benchmarks:

```sh
go test -bench=. ./test/bench/ -timeout 300s
```

Run a specific benchmark:

```sh
go test -bench=BenchmarkTools ./test/bench/ -timeout 300s
```

Key details:

- Benchmarks use `testing.B.Loop` (Go 1.24+) to prevent compiler elision.
- Baselines are committed in `test/bench/baselines/`.
- CI runs the benchstat regression gate via `.github/workflows/bench.yml`.
- PR thresholds: >15% time regression or >25% allocs regression blocks merge.
- Release thresholds: >10% time or >20% allocs.

## Adding a New MCP Tool

1. **Choose the right layer:**
   - Kernel tools go in `internal/kernel/{category}/` (symbols, edit, fileops, diag, health, help)
   - Skill tools go in `internal/skill/{name}/`

2. **Implement the tool:**
   - For kernel tools: implement the tool function, register via `RegisterTools(server *mcp.SerenaMCPServer, ...)` using `mcpsdk.AddTool`
   - For skill tools: implement the `ToolProvider` interface with `Tools() []*mcp.ToolDef`, register the skill via an `init()` function calling `skill.Register(&MySkill{})`

3. **Add to profiles:** Add the tool to the appropriate profile YAML files in `internal/profile/profiles/`.

4. **Regenerate docs:** Run `make docs` to regenerate the README tool table.

5. **Add test coverage:** Add integration tests in `test/integration/` and oracle scenario tests in `test/oracle/scenario/` for end-to-end verification.

6. **Validate:** Run `go test ./...` and `go vet ./...`.

## Adding Language Support

- Language definitions live in `internal/langregistry/languages.yaml`.
- See the memory guide: [`.serena/memories/adding_new_language_support_guide.md`](.serena/memories/adding_new_language_support_guide.md).
- After adding a language, run `make docs` to regenerate the README language table.

## Benchmark CI Gate

The benchmark CI gate prevents performance regressions from landing:

- `.github/workflows/bench.yml` runs on PRs, comparing PR benchmarks against committed baselines via `benchstat`.
- Uses tiered thresholds: PR tier (>15% time / >25% allocs) and release tier (>10% time / >20% allocs).
- `.github/workflows/capture-baseline.yml` captures new baselines (manual dispatch on GitHub Actions).

## Legacy Python

The `legacy/` directory contains the original Python Serena for reference only. It is not actively developed.
