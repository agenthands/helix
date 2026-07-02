# Technology Stack

**Analysis Date:** 2026-07-01

Helix is a **single Go binary** with no runtime Python, no Docker dependency, and
no external runtime services. Module `github.com/agenthands/helix`, Go 1.25.1,
built `CGO_ENABLED=1`. All versions below are read from the real `go.mod` at HEAD.

## Languages

**Primary:**
- Go 1.25.1 - the entire shipped product: MCP runtime, code-intelligence kernel,
  semantic index, skills, CLI. Single binary, native concurrency.

**Analyzed (not implementation) languages:**
- 52 languages served via the embedded language-server registry
  (`internal/langregistry/`) over LSP; 23 have in-process tree-sitter grammars
  (`internal/treesitter/registry.go`): go, python, typescript, tsx, rust, java, c,
  cpp, c_sharp, ruby, php, javascript, kotlin, scala, bash, haskell, julia, ocaml,
  lua, zig, hcl, plus locally vendored swift and r. 11 have full semantic
  extraction testdata (go, ts, java, csharp, kotlin, php, python, ruby, rust, c, cpp).

**Dev-time only (NOT shipped, not in the binary):**
- Python - the DSPy tuning harness under `tools/dspy-tune/`; run via `uv`/`uvx`
  in a git-ignored `.venv/`, import-isolated from runtime by `vet-tools-quarantine`.
- C/C++ toolchain - required at build time only (CGO=1) for duckdb and vendored
  tree-sitter bindings.

**Supporting:**
- YAML - profile, config, and language-registry override files.
- Protobuf - gRPC IPC schema (`api/proto/serena/v1/ipc.proto`).
- JSON - LSP 3.17 metaModel (`protocol/metaModel.json`), tool schemas.

## Runtime

**Environment:**
- Self-contained Go binary; no interpreter, no VM, no bundled language runtime.
- Runs as a **persistent daemon** (`internal/daemon/`) keeping language servers
  warm between agent sessions; agent-facing surface is the `helix <verb>` CLI.
- Language servers are external subprocesses, resolved three-tier (PATH lookup >
  managed download > helpful error) by `internal/langregistry/installer.go`.

**IPC:**
- gRPC bidirectional `StreamMCP` over a unix socket / named pipe by default
  (opt-in loopback-gated gRPC TCP via `HELIX_GRPC_ADDR`/`--grpc-addr`). The stdio
  MCP forwarder head and the Streamable-HTTP `/mcp` head were removed in Phase 94;
  only the internal gRPC `StreamMCP` wire remains.

**Containers (bench/eval only):**
- Podman in this dev environment; `bench/container` is engine-agnostic and
  auto-detects docker-or-podman. Not a product runtime dependency.

**Package Manager:**
- Go modules (`go.mod` / `go.sum`) for the product.
- `uv`/`uvx` for the dev-time `tools/dspy-tune/` Python harness only.

## Frameworks

**Core:**
- `github.com/modelcontextprotocol/go-sdk v1.5.0` - official MCP Go SDK; the
  server (`internal/mcp/SerenaMCPServer`) wraps it, middleware composes via
  `AddReceivingMiddleware` (LIFO).
- `github.com/spf13/cobra v1.10.2` - CLI command tree (`internal/cli/root.go`);
  the `helix <verb>` surface agents drive via Bash.
- `github.com/knadh/koanf/v2 v2.3.4` (+ `parsers/yaml v1.1.0`,
  `providers/file v1.2.1`, `providers/confmap v1.0.0`) - 4-layer config.
- `google.golang.org/grpc v1.80.0` + `google.golang.org/protobuf v1.36.11` -
  forwarder↔daemon IPC.

**Code intelligence:**
- `github.com/tree-sitter/go-tree-sitter v0.25.0` + 23 grammar modules - body
  extraction for symbol editing and RepoMap tag extraction. Grammar modules:
  tree-sitter-{go,python,typescript,rust,java,c,cpp,c-sharp,ruby,php,javascript,
  bash,haskell,julia,ocaml,scala} and tree-sitter-grammars/{kotlin,hcl,lua,zig},
  plus locally vendored swift and r bindings (`internal/treesitter/bindings/`).
- LSP 3.17 - generated Go types in `protocol/gen/` (324 structs, 216 union types)
  produced by `cmd/lspgen` from `protocol/metaModel.json`.

**Storage / retrieval:**
- `modernc.org/sqlite v1.48.1` - CGO-free SQLite; FTS5 memory search
  (`internal/memory/`) and RepoMap tag cache.
- `github.com/duckdb/duckdb-go/v2 v2.10502.0` - the semantic graph store
  (`internal/semantic/store/`); the only platform-conditional build tag in the
  tree gates `store/duckdb.go` (`!(windows && arm64)`).
- `github.com/philippgille/chromem-go v0.7.0` - in-process embedding store.
- `github.com/blevesearch/bleve/v2` (indirect) - full-text retrieval engine behind
  `internal/semantic/retrieval/`.

**Observability:**
- `github.com/prometheus/client_golang v1.23.2` (+ `client_model v0.6.2`,
  `prometheus v0.311.3`) - RED metrics via `TelemetryMiddleware`.
- `go.opentelemetry.io/otel v1.43.0` (+ `sdk`, `trace`, otlptrace/grpc exporter,
  otelgrpc contrib) - distributed tracing across the gRPC wire.

**Supply chain / release:**
- `github.com/sigstore/sigstore-go v1.1.4` - cosign keyless (Sigstore) attestation
  over release archives.
- `github.com/google/go-containerregistry v0.20.7` - bench container mirror pulls.

**Data / bench:**
- `github.com/apache/arrow-go/v18 v18.5.1` - columnar data interchange for
  bench/eval result tables.

## Key Dependencies

**Critical (runtime path):**
- `modelcontextprotocol/go-sdk v1.5.0` - MCP protocol core.
- `spf13/cobra v1.10.2` + `spf13/pflag` (indirect) - CLI.
- `knadh/koanf/v2 v2.3.4` - configuration.
- `tree-sitter/go-tree-sitter v0.25.0` (+ 23 grammars) - AST parsing.
- `modernc.org/sqlite v1.48.1` - FTS5 + tag cache (CGO-free).
- `duckdb/duckdb-go/v2 v2.10502.0` - semantic store (CGO).
- `google.golang.org/grpc v1.80.0` + `protobuf v1.36.11` - IPC.

**Infrastructure:**
- `github.com/fsnotify/fsnotify v1.9.0` - live-index and memory file watching.
- `github.com/gofrs/flock v0.13.0` - cross-process file locking (daemon single-instance).
- `github.com/fatih/color v1.19.0` - terminal color output.
- `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2` - tool argument schema validation.
- `golang.org/x/sync v0.20.0` - errgroup for daemon orchestration.
- `golang.org/x/mod v0.35.0`, `golang.org/x/tools v0.43.0`, `golang.org/x/sys v0.42.0`.

**Dev-time LLM SDKs (bench/tools ONLY — never on the product runtime path):**
- `github.com/anthropics/anthropic-sdk-go v1.35.0`
- `github.com/openai/openai-go v1.12.0`
  Both are used by the bench/eval harnesses and `tools/dspy-tune/`; the shipped
  `helix` binary makes no LLM calls of its own.

## Configuration

**4-layer koanf precedence** (`internal/config/`), highest wins:
1. CLI flags (cobra).
2. Project config: `.helix/project.yml`.
3. User config: `~/.helix/helix_config.yml`.
4. Profile defaults: embedded profile YAMLs (`internal/profile/`).
- `config.ResolveProfile()` bridges the config profile name to the ProfileStore.
- 5 profiles (claude-code, codex, ide-assistant, ci-bot, full) × 4 modes
  (read/edit/review/admin).

**Language registry override:**
- Embedded 52-language registry with YAML override
  (`internal/langregistry/languages.go`, `registry.go`).

**Environment / opt-ins:**
- `HELIX_GRPC_ADDR` / `--grpc-addr` - loopback-gated gRPC TCP endpoint (default off;
  unix socket / named pipe otherwise).
- `HELIX_BIN` - real-binary path for E2E tests.

**Build config:**
- `go.mod` / `go.sum` - deterministic module resolution.
- `Makefile` - `make build`, `make test` (= `make vet` + `go test`), `make vet`
  (go vet + 7 `vet-*` gate tools + verify-no-docker-sdk), `make release-snapshot`.
- `.goreleaser.yaml` - release matrix + `-ldflags` version injection.
- `.github/workflows/` - `go-test.yml`, `bench.yml`, `bench-mirror.yml`,
  `release.yml`, `codeql.yml`, `codespell.yml`.

## Platform Requirements

**Development:**
- Go 1.25.1, a host C compiler (CGO=1). zig is NOT required for local `make build`.
- `zig` (pinned 0.14.1) required only for `make release-snapshot` cross-compiles.
- Git (history mining for `cochange`, and general VCS ops).
- Podman (this env) or Docker for container-backed benches; not needed for the
  product itself.

**Build pipeline (CGO=1, split-runner per Phase 59.1):**
- **Source tree:** single-mode `CGO_ENABLED=1`; no `//go:build cgo` constraints
  except the platform-conditional `internal/semantic/store/duckdb.go`
  (`!(windows && arm64)`, D-14).
- **Local:** `make build` uses the host CC.
- **CI split-runner (D-15):**
  - `ubuntu-22.04` builds linux/{amd64,arm64} + windows/{amd64,arm64} (4 archives)
    via `CC="zig cc -target <triple>"`, zig pinned 0.14.1 (SHA-pinned setup-zig).
  - `macos-14` builds darwin/{amd64,arm64} (2 archives) natively via Apple clang.
  - Merge job (`ubuntu-22.04`) stitches both runners' artifacts and runs cosign
    keyless attestation uniformly across all 6 archives.
- **Reproducibility:** each runner rebuilds its own targets twice in the same job
  and asserts byte-identical sha256 (per-target within-runner Pass-1 ≡ Pass-2).
  Cross-runner byte-equality is not asserted.

**Runtime (production):**
- Single self-contained binary per target OS/arch (6-archive matrix).
- Language servers downloaded on demand by the three-tier installer; no bundled LS.
- No runtime Python, no Docker, no external services.

## Language Server Architecture

Language servers run as external subprocesses managed by the kernel's LS worker
pool (`internal/kernel/lspool/`): share-until-dirty reuse (clean sessions share a
warm worker), adaptive TTL with reuse scoring, circuit breaking with exponential
backoff for crashy workers, and platform-aware memory-pressure eviction (Linux
cgroups, macOS vm_stat). LS communication uses a custom JSON-RPC 2.0 codec
(`internal/kernel/jsonrpc/`).

Each supported language has:
- An entry in the 52-language embedded registry (`internal/langregistry/`, YAML
  override supported) declaring its language server and install strategy.
- Three-tier resolution (`internal/langregistry/installer.go`): PATH lookup >
  managed download > helpful error.
- Optionally an in-process tree-sitter grammar (23 registered in
  `internal/treesitter/registry.go`) used for RepoMap tag extraction and
  tree-sitter body surgery in the edit tools, independent of the LSP path.

The kernel exposes LSP-backed operations (go-to-definition, find-references, hover,
implementations, call/type hierarchy) through `internal/kernel/symbols/`, with a
tree-sitter `documentSymbol` fallback in `internal/repomap/` when no LSP is
available for a language.

Representative language servers resolved by the registry: gopls (Go), pyright
(Python), rust-analyzer (Rust), typescript-language-server (TS/JS), Eclipse JDT
(Java), OmniSharp (C#), plus 40+ additional entries.
