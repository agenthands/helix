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

Releases ship as multi-arch signed binaries via a goreleaser pipeline (see `.goreleaser.yaml` and `.github/workflows/release.yml`). A `v*` git tag triggers the release workflow automatically -- there is no manual draft step. The CI-enforced reproducibility gate runs two consecutive snapshot builds with identical inputs and refuses to publish if their archive sha256s differ, catching most build-environment non-determinism (toolchain drift, mod_timestamp, trimpath, GOFLAGS) before publication. The gate does not, however, compare against the real-release artifacts that ship to users -- a non-determinism source that lives only behind the real-release code path (e.g. tag-only build constants, changelog generation) would not be caught. If you suspect a real-release-only non-determinism, do a local build of the same tag with `goreleaser release --snapshot --clean --skip=sign` after your tag and diff against the published `dist/` from CI.

To dry-run the build matrix locally (signs are skipped because the secret key lives only in CI):

```sh
make release-snapshot
```

Output goes to `dist/` (gitignored, overwrites). On a clean checkout you should see 6 archives (`serena_<version>_<os>_<arch>.tar.gz`) and a `checksums.txt` file. The local dry-run requires `goreleaser` on `$PATH`; install with `brew install goreleaser` on macOS, or download a release tarball from `github.com/goreleaser/goreleaser/releases` on Linux.

To cut a release, push a version tag from a green-CI commit on `main`:

```sh
git tag v1.9.0
git push origin v1.9.0
```

Pre-release tags (`v1.9.0-rc1`, `v1.9.0-beta1`, `v1.9.0-alpha1`) are auto-detected by goreleaser and marked as Pre-release on the GitHub Releases page. Production tags (`v1.9.0`) publish as a regular release.

### Repository secrets

The release workflow signs archives with the project's minisign keypair. Two GitHub Actions secrets must exist on the repo BEFORE the first `v*` tag is pushed:

- `MINISIGN_PRIVATE_KEY` -- contents of the locally generated `minisign.key` file (NOT the file path; the actual file contents pasted into the secret)
- `MINISIGN_PASSWORD` -- the password chosen during keypair generation

These are uploaded under repo Settings > Secrets and variables > Actions.

### One-time keypair setup

The maintainer generates the keypair once on a trusted local machine, commits the public half to the repo, and uploads the private half plus password as repo secrets:

```sh
minisign -G -p minisign.pub -s minisign.key
# minisign prompts for a password; choose a strong one and store it securely
gh secret set MINISIGN_PRIVATE_KEY < minisign.key
gh secret set MINISIGN_PASSWORD   # paste the password when prompted
git add minisign.pub
git commit -m "feat(release): commit minisign public key"
git push
```

After this is done, delete the local `minisign.key` (the secret is now in GitHub's secret store; the local copy is no longer needed and should not linger on disk). The password should be stored in a password manager.

### Key rotation

To rotate the minisign keypair, repeat the one-time setup with a new pair, commit the new `minisign.pub`, and update both repo secrets. Users who have already downloaded the old `minisign.pub` will need to re-fetch it from `https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub` -- INSTALL.md tells them to re-fetch the key as part of the verification recipe, so users on the latest INSTALL.md instructions pick up the rotated key automatically.

Key details:

- The release workflow does NOT re-run `go test` or `go vet` -- tags are assumed to be cut from a commit that has already passed `go-test.yml` on `main`. If you tag a commit that has not been through CI, the release may publish a binary built from broken code (the reproducibility gate cannot catch logic bugs, only build determinism).
- Release notes are auto-generated from the git log between tags using conventional-commit prefix grouping (`feat:`, `fix:`, `docs:`, `refactor:`). `CHANGELOG.md` stays hand-curated separately for human-readable narrative.
- The local dry-run skips signing (`--skip=sign`) because contributors do not have access to the project's minisign secret key -- signing is exercised in CI only. The dry-run still validates the build matrix, archive packaging, and checksums.txt generation.
- A typo'd tag publishes a release immediately; there is no draft step. The reproducibility gate is the safety net against non-deterministic artifacts, not against typo'd tags. If a release is published in error, delete it via the GitHub Releases UI and re-tag with a corrected version.

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

## gopls Compatibility

- `gopls` is installed at runtime by `internal/langregistry`; the project does not pin a gopls version in `go.sum` or `go.mod`.
- The project tests against `gopls@latest`. Run `go install golang.org/x/tools/gopls@latest` after every Go upgrade.
- gopls v0.17.1 had a Go 1.25 incompatibility on linux/amd64. The fix shipped upstream in gopls v0.21 and later — use `>=v0.21` as a floor, not a pin.

## Legacy Python

The `legacy/` directory contains the original Python Serena for reference only. It is not actively developed.
