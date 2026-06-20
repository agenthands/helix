# Contributing to Helix

Thank you for your interest in contributing to Helix! We welcome contributions that improve and extend the project.

## Scope of Contributions

The following types of contributions can be submitted directly via pull requests:

- Isolated additions that extend Helix along existing lines (e.g., adding support for a new language server)
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
| `go build ./cmd/helix` | Build the helix binary |
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

Helix uses a 4-layer architecture shipping as a single Go binary:

- `cmd/helix/` -- CLI entry point
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
- `api/proto/serena/v1/` -- gRPC IPC definitions (proto package directory retained as a wire-format lineage artifact from the v1.8 Serena lineage; see CHANGELOG.md v1.9)
- `protocol/gen/` -- Generated LSP 3.17 types
- `test/harness/` -- Test harness: Runner, tool helpers, golden file comparison, fixtures
- `test/oracle/` -- Oracle test suite (6 layers: protocol, contract, runtime, scenario, llm, judge)
- `test/integration/` -- MCP round-trip integration tests
- `test/bench/` -- Benchmark suite with baselines

## Running Integration Tests

The integration test harness lives in `test/integration/`. It starts a real Helix daemon with a Go fixture project and exercises MCP round-trips via stdio transport against live language servers.

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

The benchmark suite lives in `test/bench/`. **Benchmarks run locally only** — the project does not run benchmarks on CI runners because shared GitHub-hosted runners produce noisy, untrustable baselines.

Run the suite once and print to stdout:

```sh
make bench
```

Capture a local baseline (overwrites `test/bench/baselines/local.txt`, which is gitignored):

```sh
make bench-baseline
```

To compare two captures, install upstream `benchstat` and run it directly:

```sh
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

Key details:

- Benchmarks use `testing.B.Loop` (Go 1.24+) to prevent compiler elision.
- `test/bench/baselines/` is the conventional capture location; everything you save there is gitignored.
- If your changes are likely to affect bench numbers, mention the local before/after deltas in your PR description. (Honor system — there is no PR-time gate.)

## Releasing

Releases ship as multi-arch signed binaries via a [GoReleaser](https://goreleaser.com/) (OSS v2.15.4) pipeline (see `.goreleaser.yaml` and `.github/workflows/release.yml`). A `v*` git tag triggers the release workflow automatically — there is no manual draft step.

Phase 59.1 introduced a **split-runner architecture** because GoReleaser's native partial-by-target / split-release / merge-continue mechanic is **GoReleaser Pro exclusive** (verified directly against https://goreleaser.com/customization/partial/). Helix runs the OSS distribution. The OSS-supported equivalent is the **FALLBACK-B-MULTI-BUILD-ID** pattern: `.goreleaser.yaml` declares two `builds:` entries (`helix-non-darwin`, `helix-darwin`), each runner invokes `goreleaser build --id <runner-id>`, and a merge job runs `goreleaser release --skip=build` to assemble archives + checksums + signatures from the pre-built binaries. (See `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-02-SPLIT-PROBE-NOTES.md` for the GoReleaser Pro lock-in evidence and the Pro-exclusive directive names.)

### Local snapshot builds (per-host partial matrix)

The full 6-archive matrix only assembles in CI. Locally, you can build the subset your host can natively compile.

**darwin host** (e.g. an Apple Silicon Mac) — Apple clang produces darwin/{amd64,arm64} (2 binaries):

```sh
goreleaser build --snapshot --clean --id helix-darwin
```

Output: `dist/helix-darwin_darwin_amd64_v1/helix` and `dist/helix-darwin_darwin_arm64_v8.0/helix`.

**linux host** (with `zig` on PATH, version 0.14.1+) — zig cc cross-compiles linux/{amd64,arm64} + windows/{amd64,arm64} (4 binaries):

```sh
goreleaser build --snapshot --clean --id helix-non-darwin
```

Output: `dist/helix-non-darwin_linux_amd64_v1/helix`, `dist/helix-non-darwin_linux_arm64_v8.0/helix`, `dist/helix-non-darwin_windows_amd64_v1/helix.exe`, `dist/helix-non-darwin_windows_arm64_v8.0/helix.exe`.

### `make release-snapshot` (convenience target)

`make release-snapshot` invokes `goreleaser release --snapshot --clean --skip=sign`, which attempts to build BOTH `helix-non-darwin` AND `helix-darwin` entries in a single invocation. Behavior depends on host:

- **On a linux host (with zig 0.14.1+):** builds 4 linux+windows binaries successfully; darwin entries fail (a linux host cannot natively produce darwin binaries because zig cc cannot reach the Apple SDK headers `mach/mach_vm.h` — the Wave 0 probe blocker).
- **On a darwin host:** builds 2 darwin binaries successfully; the `helix-non-darwin/windows_amd64_v1` zig cc link step then fails with C++ stdlib undefined symbols (`std::ostream`, `std::basic_streambuf`, `std::rethrow_exception`, …) because `libduckdb_static.a` was built against **libstdc++** but zig defaults to **libc++** — lld-link cannot resolve cross-stdlib symbols. **This is a darwin-host-local limitation, NOT a config defect** — the CI ubuntu runner does not hit it because it invokes `goreleaser build --id helix-non-darwin` (constrained to the non-darwin entry) and never collides both stdlibs in a single link step.

The recommended local workflow is to use the per-`--id` invocations above instead of `make release-snapshot` if you only need to validate the subset your host can build. CI is the authoritative full-matrix path.

### CI release flow

A real release is cut by pushing a `vX.Y.Z` tag. The `.github/workflows/release.yml` workflow runs three jobs:

1. **`release-linux`** (`ubuntu-22.04`): SHA-pinned `mlugg/setup-zig` action installs zig 0.14.1; runs Pass-1 then Pass-2 of `goreleaser build --id helix-non-darwin`; asserts per-target byte-identical sha256 (D-17); uploads partial dist as `dist-partial-linux`.
2. **`release-darwin`** (`macos-14`, tag-gated per D-16): selects Xcode 15.x; records `xcodebuild -version` + `clang --version` provenance; runs Pass-1 then Pass-2 of `goreleaser build --id helix-darwin`; asserts per-target byte-identical sha256; uploads partial dist as `dist-partial-darwin`.
3. **`release-merge`** (`ubuntu-22.04`, `needs:` both): downloads both partial dists into a unified `dist/` tree; runs `goreleaser release --clean --skip=build`; cosign keyless attestation runs uniformly across all 6 archives; publishes the GitHub Release.

The user-facing artifact-name shape (`helix_v<version>_<os>_<arch>.tar.gz`) and the `.sigstore.json` bundle layout are preserved verbatim; the Phase 58 REL-01 contract holds. Existing `helix upgrade` consumers see no change.

To cut a release, push a version tag from a green-CI commit on `main`:

```sh
git tag v1.10.0
git push origin v1.10.0
```

Pre-release tags (`v1.10.0-rc1`, `v1.10.0-beta1`, `v1.10.0-alpha1`) are auto-detected by goreleaser and marked as Pre-release on the GitHub Releases page. Production tags (`v1.10.0`) publish as a regular release.

### Reproducibility gate

The reproducibility gate is **per-target within-runner Pass-1 ≡ Pass-2**. Each runner rebuilds its own targets twice in the same workflow job and fails-loud if the sha256s diverge. **Cross-runner byte-equality is NOT asserted** (different machines, different SDKs, different clang versions — that comparison was never meaningful). On Pass-1 ≢ Pass-2 divergence, a per-runner WR-07 forensic dump (per-file sha256, tar entry metadata, gzip header bytes, Mach-O LC_UUID + codesign timestamp on darwin, `go version -m` diff) drops into the workflow logs.

The gate does not compare against the real-release artifacts that ship to users — a non-determinism source that lives only behind the real-release code path (e.g. tag-only build constants, changelog generation) would not be caught. If you suspect a real-release-only non-determinism, do a local build of the same tag with `goreleaser build --snapshot --clean --id <runner-id>` after the tag and diff against the published `dist/` from CI. The trade-off is intentional — adding a real-artifact comparison job would require regenerating expected hashes per release, which the v1.9 milestone audit explicitly judged not worth the ongoing maintainer toil.

### Snapshot dispatch (rehearsal)

To exercise the full pipeline against a draft branch without cutting a real tag (e.g. for first-CI-repro evidence per Wave 5), use the `workflow_dispatch` trigger:

```sh
gh workflow run release.yml --ref <draft-branch>
```

The dispatch path runs `goreleaser release --snapshot --clean --skip=build,sign` in the merge job: archives + checksums are produced, but cosign signing and GitHub release publishing are skipped.

### Toolchain prerequisites (local)

| Tool | Required for | Install hint |
|------|--------------|--------------|
| Go 1.25.x | `make build`, `go test ./...` | https://go.dev/dl/ |
| goreleaser v2.15.4 | `make release-snapshot`, local `goreleaser build --id <id>` | `brew install goreleaser` (macOS) or [release page](https://github.com/goreleaser/goreleaser/releases/tag/v2.15.4) |
| zig 0.14.1+ | `make release-snapshot` (linux+windows portion); local `--id helix-non-darwin` invocation | `brew install zig` (macOS, picks up latest), `apt install zig` (Ubuntu 22.04+), or https://ziglang.org/download/ |
| cosign 2.4.1+ | local sign verification only (CI uses keyless OIDC, no local cosign needed for the build itself) | `brew install cosign` |

### Repository secrets

The release workflow signs archives with sigstore cosign keyless via the GitHub Actions OIDC token (Phase 58 D-02). No long-lived secrets are required — the workflow declares `id-token: write` permission, exchanges the OIDC token with Fulcio for a short-lived signing certificate, and submits the signature to Rekor for transparency-log inclusion. There is no signing key to generate, no password to manage, and no secret rotation.

### Trust root refresh

The verifier embeds `internal/upgrade/trusted_root.json` — a snapshot of the upstream public-good Sigstore TUF trust root. Sigstore rotates the public-good Fulcio CA infrequently (multi-year cadence), but a stale embedded root could in principle reject post-rotation signatures. Refresh the snapshot before each minor release:

```sh
make update-trust-root
git add internal/upgrade/trusted_root.json
git commit -m "chore(release): refresh sigstore trust root"
```

The Make target downloads the file from `raw.githubusercontent.com/sigstore/sigstore-go/main/examples/trusted-root-public-good.json` and overwrites the in-tree copy. There is no separate verify-the-embed gate — the bytes that ship in the binary are the bytes you committed.

Key details:

- The release workflow does NOT re-run `go test` or `go vet` -- tags are assumed to be cut from a commit that has already passed `go-test.yml` on `main`. If you tag a commit that has not been through CI, the release may publish a binary built from broken code (the reproducibility gate cannot catch logic bugs, only build determinism).
- Release notes are auto-generated from the git log between tags using conventional-commit prefix grouping (`feat:`, `fix:`, `docs:`, `refactor:`). `CHANGELOG.md` stays hand-curated separately for human-readable narrative.
- The local dry-run skips signing (`--skip=sign`) because cosign keyless signing requires a GitHub Actions OIDC token that is only available to the release runner -- signing is exercised in CI only. The dry-run still validates the build matrix, archive packaging, and checksums.txt generation.
- A typo'd tag publishes a release immediately; there is no draft step. The reproducibility gate is the safety net against non-deterministic artifacts, not against typo'd tags. If a release is published in error, delete it via the GitHub Releases UI and re-tag with a corrected version.

## Tracing

Helix emits OpenTelemetry spans across the stdio→forwarder→daemon→kernel call
chain. By default both the forwarder and the daemon use a no-op TracerProvider
(no spans exported, no overhead).

To enable real trace export, set `OTEL_EXPORTER_OTLP_ENDPOINT` to an OTLP/gRPC
collector URL (for example `http://localhost:4317`):

```sh
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
helix daemon
```

Trace propagation between the forwarder client and the daemon gRPC server
uses W3C TraceContext via `otelgrpc`'s `WithPropagators(propagation.TraceContext{})`
option, wired per-handler in `internal/obs/grpc.go`. There is no global
`otel.SetTextMapPropagator` call anywhere in the codebase — the no-global
OTel rule (see `internal/obs/tracing.go` D-01) keeps tracing setup explicit.

The end-to-end trace-continuity contract is enforced by
`test/integration/trace_continuity_test.go`: a single TraceID must cover
the `forwarder.tools.call` client span and the
`serena.v1.ForwarderService/StreamMCP` server span. If you change handler
wiring, make sure that test still passes.

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
- See the memory guide: [`.helix/memories/adding_new_language_support_guide.md`](.helix/memories/adding_new_language_support_guide.md).
- After adding a language, run `make docs` to regenerate the README language table.

## gopls Compatibility

- `gopls` is installed at runtime by `internal/langregistry`; the project does not pin a gopls version in `go.sum` or `go.mod`.
- The project tests against `gopls@latest`. Run `go install golang.org/x/tools/gopls@latest` after every Go upgrade.
- gopls v0.17.1 had a Go 1.25 incompatibility on linux/amd64. The fix shipped upstream in gopls v0.21 and later — use `>=v0.21` as a floor, not a pin.
