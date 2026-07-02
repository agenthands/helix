# External Integrations

**Analysis Date:** 2026-07-01

## APIs & External Services

**Language Servers (primary external integration):**
- 52-language embedded registry (`internal/langregistry/languages.go`
  `defaultEntries`); each entry declares command, args, file extensions, and an
  optional `InstallInfo`.
- Three-tier installer (`internal/langregistry/installer.go` `Installer.Resolve`):
  PATH lookup → managed download (npm / pip / cargo / gem / dotnet / binary) →
  helpful error. Examples: `gopls serve` (Go), `pyright-langserver --stdio` (Python,
  pip), `typescript-language-server --stdio` (npm), `rust-analyzer` (Rust), `jdtls`
  (Java, binary), `clangd --background-index` (C/C++).

**MCP protocol (internal plumbing, not agent-facing):**
- MCP server on the official Go SDK, `github.com/modelcontextprotocol/go-sdk v1.5.0`
  (`internal/mcp/server.go`; server type `SerenaMCPServer` — Go identifier retained
  for internal-API stability, Phase 52-03). The user-facing MCP
  `Implementation.Name` is `helix`.
- Transport: gRPC `StreamMCP` bidirectional stream over a unix-domain socket by
  default (`api/proto/serena/v1/ipc.proto`, `google.golang.org/grpc v1.80.0`). The
  stdio MCP head and Streamable-HTTP `/mcp` head were removed in Phase 94.

**LLM provider SDKs (DEV-TIME bench/tools only — NOT in the shipped agent path):**
- `github.com/anthropics/anthropic-sdk-go v1.35.0` and
  `github.com/openai/openai-go v1.12.0` appear only in the `bench/` harness and
  dev-time tooling; the runtime `helix` daemon makes zero LLM API calls. The
  bench scripted-agent path is local-only, no-network, no-API-key (`Makefile`
  eval/bench targets, D-01).

**Container engines (bench harness only):**
- `bench/container` drives a local engine purely via `os/exec` (`engine.go`
  `Detect`), probing `docker` then `podman` (docker wins when both present;
  `TestDetectPrefersDocker`). It NEVER imports the Docker Go SDK — the ban is
  enforced by the `verify-no-docker-sdk` gate in `make vet`. This dev environment
  uses Podman.

**Container registry (bench image mirror):**
- `github.com/google/go-containerregistry v0.20.7` (`bench/container/pull.go`)
  performs daemon-free, digest-pinned registry round-trips for bench image mirroring.

## Data Storage

**Semantic graph store — DuckDB:**
- `github.com/duckdb/duckdb-go/v2 v2.10502.0` (CGO). SOLE owner is
  `internal/semantic/store/duckdb.go` (D-12); the import is confined by the
  `vet-noduckdb` gate. Per-workspace file at `<workspace>/.helix/semantic.duckdb`.
- Three-tier open (`internal/semantic/store/doc.go`): open-clean / quarantine+rebuild
  (`<path>.corrupt.<unix-ts>`) / hard-fail. Effective reads = snapshot ⊕ overlay −
  tombstones. DuckDB holds its own file lock → single-daemon-per-workspace.
- Platform-conditional build tag: `duckdb.go` carries `//go:build !(windows && arm64)`
  (the only build-tag constraint in the source tree).

**Memory / knowledge store — SQLite FTS5:**
- `modernc.org/sqlite v1.48.1` (CGO-free). `internal/memory/index.go` opens the
  `sqlite` driver and `schema.go` creates a `memories` table plus a `memories_fts`
  FTS5 virtual table; search orders by FTS5 rank. Backs the 7 memory tools.

**RepoMap tag cache — SQLite:**
- `internal/repomap/` persists a mtime-invalidated tag cache in SQLite for PageRank
  and token-budgeted repo overview (`TagCache`, wired in `internal/daemon/daemon.go`).

**Embeddings:**
- `github.com/philippgille/chromem-go v0.7.0` — embedded vector store used by the
  semantic retrieval path.

**Columnar / bench data:**
- `github.com/apache/arrow-go/v18` — Arrow tables in the bench evaluation stack.

**File storage (local filesystem only):**
- Project config/data: `.helix/` (project) and `~/.helix/` (user).
- Semantic DB: `<workspace>/.helix/semantic.duckdb`. Memory: SQLite index under the
  helix data dir. No cloud storage; no runtime network dependency.

## Authentication & Identity

**No agent-facing auth:** the `helix` daemon exposes no authenticated network
service. The default transport is a unix-domain socket (filesystem-permissioned);
gRPC TCP is opt-in and loopback-gated.

**Supply-chain identity — sigstore keyless attestation:**
- `github.com/sigstore/sigstore-go v1.1.4` (`internal/upgrade/verify.go`). Self-upgrade
  verifies the release archive's sigstore bundle against a pinned GitHub Actions OIDC
  issuer (`https://token.actions.githubusercontent.com`) and a pinned certificate SAN
  regex for the `agenthands/helix` `release.yml` workflow on a canonical semver tag.
  Every failure branch returns the identical `signature verification FAILED` message
  (Pitfall 4) so failure modes are indistinguishable to a probing attacker; the
  Rekor-unreachable branch is the sole exception. Trust root: `internal/upgrade/trusted_root.json`.

## Monitoring & Observability

**Metrics — Prometheus:**
- `github.com/prometheus/client_golang v1.23.2`, confined to `internal/obs/`
  (`metrics.go`). RED metrics on `tools/call` via `TelemetryMiddleware`; exposed on
  the admin listener `/metrics` when `observability.admin_addr` is set.

**Tracing — OpenTelemetry:**
- `go.opentelemetry.io/otel v1.43.0` + `otlptrace/otlptracegrpc` +
  `contrib/.../otelgrpc`. `obs.Provider` holds the `TracerProvider` (noop by default;
  OTLP/gRPC exporter via `WithTracing`); spans across daemon, kernel, and semantic
  ops. `ShutdownTracing` flushes on shutdown.

**Logging:**
- `log/slog` to stderr (text or JSON), wrapped in `obs.NewContextHandler` so traced
  requests carry `trace_id`/`span_id` (`internal/cli/root.go` `newLogger`).

## CI/CD & Deployment

**Distribution:** single Go binary; no container or compose deployment. Self-upgrade
via `internal/upgrade/` pulls signed release archives from GitHub Releases.

**Build (CGO=1, split-runner per Phase 59.1):**
- Local: `make build` uses the host CC (no zig required). `make release-snapshot`
  needs `zig` for cross-compilation (host-platform partial matrix only).
- CI: `ubuntu-22.04` builds linux + windows (4 archives) via `zig cc`; `macos-14`
  builds darwin (2 archives) natively; the merge job runs cosign keyless attestation
  over all 6 archives. Per-target within-runner reproducibility (Pass-1 ≡ Pass-2
  byte-identical sha256).

**CI workflows (`.github/workflows/`):**
- `go-test.yml` — `make test` (= `make vet` + `go test`).
- `release.yml` — split-runner build + cosign attestation + GitHub Release.
- `bench.yml`, `bench-mirror.yml` — bench harness + image mirror.
- `codeql.yml` — CodeQL static analysis. `codespell.yml` — spelling.

**Test gates (`make vet`):** `go vet` + 7 custom `cmd/vet-*` architectural gates +
`verify-no-docker-sdk`; drift gates `verify-cligen` / `verify-docs` /
`verify-reference` keep generated catalogs in lockstep.

## Environment Configuration

**Config precedence (`internal/config/loader.go` `Load`, koanf v2):**
CLI flags > project `.helix/project.yml` > user `~/.helix/helix_config.yml` >
built-in defaults. Providers: `koanf/parsers/yaml`, `providers/file`, `providers/confmap`.

**Key CLI flags (`internal/cli/root.go` `runDaemon`):**
- `--serve`, `--socket` (`daemon.socket_path`), `--config`, `--profile`,
  `--admin-addr` (`observability.admin_addr`), `--grpc-addr` (`daemon.grpc_addr`;
  empty = unix-socket only), `--json` (JSON logs), and subsystem-disable ablation
  flags `--disable-lsp-subsystem` / `--disable-structured-edit-subsystem` /
  `--disable-semantic-subsystem`.

**Semantic index config (`internal/semantic/config.go`, koanf `semantic_index.*`):**
`semantic_index.enabled` gates the whole subsystem; `semantic_index.bench_disabled`
is the distinct ablation gate. Defaults live in `internal/config/defaults.go`.

**Version injection:** `main.version` set via `-ldflags "-X main.version=$VERSION"`
by goreleaser; threaded into both `helix --version` and the MCP `Implementation.Version`.

## Webhooks & Callbacks

**Client hooks (`helix setup <client>`):** for Claude-family clients,
`internal/cli/setup_hooks.go` installs Claude Code hooks — SessionStart (activate
workspace), PreToolUse (nudge toward symbolic tools), Stop (cleanup); `--no-hooks`
opts out. Non-skill clients receive MCP-teardown only.

**Daemon post-init callbacks (`internal/daemon/daemon.go`):** internal wiring
callbacks rather than network webhooks — `SetEnrichFn` (RepoMap LSP enrichment on
graph rebuild), the workspace activation callback (`ActivateWorkspace` /
`DeactivateWorkspace` gRPC RPCs invoked by client hooks), and the kernel
`EditNotifier` bridge that feeds edits into the semantic live overlay.

*(The prior web-dashboard HTTP/WebSocket endpoints and proprietary IDE plugin
callbacks no longer exist — Helix ships no dashboard and no proprietary IDE backend.
Non-Claude setup clients receive MCP-teardown only.)*

## Language Server Communication

**Protocol:** LSP 3.17 over JSON-RPC 2.0 on the language server's stdin/stdout.
`internal/kernel/jsonrpc/` provides the JSON-RPC 2.0 codec (`codec.go`, `conn.go`);
generated LSP types live in `protocol/gen/` (324 structs, 216 union types from the
official metaModel.json).

**Process management:** language servers run as external subprocesses managed by the
LS worker pool (`internal/kernel/lspool/`): `process.go` (spawn/lifecycle),
`worker.go` (per-worker LSP session), `circuit.go` (circuit breaking with backoff),
`pressure_linux.go` / `pressure_darwin.go` (memory-pressure eviction). Warm workers
are shared across clean sessions (share-until-dirty) and kept alive between agent
sessions by the persistent daemon.

**Key external language servers (from `internal/langregistry/languages.go`):**
- `gopls` (Go), `pyright-langserver` (Python), `typescript-language-server`
  (TypeScript), `rust-analyzer` (Rust), `jdtls` (Java, Eclipse JDT LS), `clangd`
  (C/C++), `csharp-ls` (C#), `kotlin-language-server` (Kotlin), plus ~44 more entries
  (Ruby, PHP, Scala, Swift, Lua, Haskell, Julia, OCaml, Zig, Terraform, and others).

---

*Integration audit: 2026-07-01*
