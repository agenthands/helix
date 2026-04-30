# Phase 52: EMBED-AUDIT.md

**Created:** 2026-04-30
**Status:** living artifact — update on any new disk-read added in future phases
**Scope:** every runtime asset read or embedded by the `helix` binary's production code paths
**Authority:** CONTEXT.md D-14 (audit produces a written manifest) + D-15 (LS binaries are external-by-design)

This manifest classifies every runtime asset the shipped `helix` binary touches as one of:

- **Embedded** — shipped inside the binary via `//go:embed` (the asset travels with `helix`).
- **External-by-design** — read from disk at runtime by deliberate policy (LS binaries, user config, project workspace files, OS interfaces).
- **Gap** — assets that *should* have been embedded but aren't. Closed in this plan; the manifest ships with zero open gaps.

Future contributors adding a new `os.ReadFile` / `os.Open` site MUST consult this document and update it.

## Audit Method

Greps run from repo root (per CONTEXT.md D-14 recipe):

```bash
grep -rn "//go:embed"        --include="*.go" --exclude-dir=legacy --exclude-dir=testdata .
grep -rn "os\.ReadFile\|os\.Open" --include="*.go" --exclude-dir=legacy --exclude-dir=testdata .
grep -rn "filepath\.Join"    --include="*.go" --exclude-dir=legacy --exclude-dir=testdata .
```

Scope filtering applied to the raw hits:

- **Excluded:** `legacy/`, `testdata/`, `*_test.go` (test fixtures and pytest-port reference are not runtime assets of the shipped binary).
- **Excluded:** `cmd/docgen/` and `cmd/lspgen/` (build-time dev tools — not part of the `helix` binary; their disk reads target `README.md` and `protocol/metaModel.json` at *generation time*, not at user runtime).
- **Excluded:** `test/` directory (integration / oracle / harness — not shipped).
- **Out-of-scope finding:** `internal/kernel/edit/queries/*.scm` (22 files). Tracked in repo as Phase 31 reference documentation for `internal/kernel/edit/treesitter.go`'s `langConfig` map; **never read by any production code path** (verified: zero references to `edit/queries` in any Go file). Not a runtime asset; not a gap. Possible Phase-53+ cleanup (delete the dead reference dir) — out of scope for this audit per the executor's `<scope_boundary>` rule.

Raw counts:
- `//go:embed` directive surface: **26 hits across 3 files** (counts include 2 doc-comment hits in `internal/upgrade/pubkey.go`; 24 effective directives).
- `os.ReadFile` / `os.Open` surface: **92 raw hits** → **51 in production code** after filtering tests → **38 classified entries** after grouping callsites by package responsibility (multiple hits in one file with the same disposition collapse to one row).
- `filepath.Join` surface: 364 raw hits, sampled informationally — every join is either workspace-relative (`<project>/...`), home-relative (`$HOME/.helix/...` or `$HOME/.gemini/...`), or `os.Executable()`-relative (forwarder/upgrade swap) — none read an asset that should ship in the binary.

Total entries classified: **38** (24 embedded + 14 external-by-design groups; 0 gaps).

## Embedded

Assets shipped inside the binary via `//go:embed`. Adding a new entry here means it does NOT need a runtime download path or a user-managed fallback.

| Asset | File (directive site) | Directive | Purpose |
|-------|-----------------------|-----------|---------|
| `internal/upgrade/minisign.pub` | `internal/upgrade/pubkey.go:22` | `//go:embed minisign.pub` | Verifies next release archive at upgrade time. Build-time-synced from repo-root `minisign.pub` to `internal/upgrade/minisign.pub` via `make embed-pubkey` (Plan 04 / Plan 01). |
| Profile YAMLs (5 files: claude-code, codex, ide-assistant, ci-bot, full) | `internal/profile/embed.go:5` | `//go:embed profiles/*.yaml` | Agent profiles consumed by `profile.LoadEmbedded()` |
| Mode YAMLs (4 files: read, edit, review, admin) | `internal/profile/embed.go:8` | `//go:embed modes/*.yaml` | Operational modes consumed by `profile.LoadEmbedded()` |
| `queries/go_tags.scm` | `internal/repomap/extractor.go:18` | `//go:embed queries/go_tags.scm` | RepoMap tag extraction for Go |
| `queries/python_tags.scm` | `internal/repomap/extractor.go:21` | `//go:embed queries/python_tags.scm` | RepoMap tag extraction for Python |
| `queries/typescript_tags.scm` | `internal/repomap/extractor.go:24` | `//go:embed queries/typescript_tags.scm` | RepoMap tag extraction for TypeScript |
| `queries/rust_tags.scm` | `internal/repomap/extractor.go:27` | `//go:embed queries/rust_tags.scm` | RepoMap tag extraction for Rust |
| `queries/java_tags.scm` | `internal/repomap/extractor.go:30` | `//go:embed queries/java_tags.scm` | RepoMap tag extraction for Java |
| `queries/c_tags.scm` | `internal/repomap/extractor.go:33` | `//go:embed queries/c_tags.scm` | RepoMap tag extraction for C |
| `queries/cpp_tags.scm` | `internal/repomap/extractor.go:36` | `//go:embed queries/cpp_tags.scm` | RepoMap tag extraction for C++ |
| `queries/csharp_tags.scm` | `internal/repomap/extractor.go:39` | `//go:embed queries/csharp_tags.scm` | RepoMap tag extraction for C# |
| `queries/ruby_tags.scm` | `internal/repomap/extractor.go:42` | `//go:embed queries/ruby_tags.scm` | RepoMap tag extraction for Ruby |
| `queries/php_tags.scm` | `internal/repomap/extractor.go:45` | `//go:embed queries/php_tags.scm` | RepoMap tag extraction for PHP |
| `queries/javascript_tags.scm` | `internal/repomap/extractor.go:48` | `//go:embed queries/javascript_tags.scm` | RepoMap tag extraction for JavaScript |
| `queries/kotlin_tags.scm` | `internal/repomap/extractor.go:51` | `//go:embed queries/kotlin_tags.scm` | RepoMap tag extraction for Kotlin |
| `queries/scala_tags.scm` | `internal/repomap/extractor.go:54` | `//go:embed queries/scala_tags.scm` | RepoMap tag extraction for Scala |
| `queries/bash_tags.scm` | `internal/repomap/extractor.go:57` | `//go:embed queries/bash_tags.scm` | RepoMap tag extraction for Bash |
| `queries/haskell_tags.scm` | `internal/repomap/extractor.go:60` | `//go:embed queries/haskell_tags.scm` | RepoMap tag extraction for Haskell |
| `queries/julia_tags.scm` | `internal/repomap/extractor.go:63` | `//go:embed queries/julia_tags.scm` | RepoMap tag extraction for Julia |
| `queries/ocaml_tags.scm` | `internal/repomap/extractor.go:66` | `//go:embed queries/ocaml_tags.scm` | RepoMap tag extraction for OCaml |
| `queries/lua_tags.scm` | `internal/repomap/extractor.go:69` | `//go:embed queries/lua_tags.scm` | RepoMap tag extraction for Lua |
| `queries/zig_tags.scm` | `internal/repomap/extractor.go:72` | `//go:embed queries/zig_tags.scm` | RepoMap tag extraction for Zig |
| `queries/hcl_tags.scm` | `internal/repomap/extractor.go:75` | `//go:embed queries/hcl_tags.scm` | RepoMap tag extraction for HCL/Terraform |
| `queries/r_tags.scm` | `internal/repomap/extractor.go:78` | `//go:embed queries/r_tags.scm` | RepoMap tag extraction for R |
| `queries/swift_tags.scm` | `internal/repomap/extractor.go:81` | `//go:embed queries/swift_tags.scm` | RepoMap tag extraction for Swift |

**Implicit / structural embeds** (not via `//go:embed` directives but compiled-in by virtue of being Go source/data):

| Asset | File | Form | Purpose |
|-------|------|------|---------|
| Language registry defaults | `internal/langregistry/languages.go` | `defaultEntries map[string]LSEntry` literal in Go source | 52-language LS metadata (command, args, file_exts, install info). Compiled into the binary; user `~/.helix/languages.yaml` overrides applied at registry construction time per `internal/langregistry/registry.go`. |
| Tree-sitter grammar bindings (23 langs) | `internal/treesitter/bindings/<lang>/` (CGO) and `internal/treesitter/grammars.go` (CGO=0 stub) | Linked C source + Go bindings | Tag extraction grammars used by RepoMap. Shipped with the binary via the CGO build path; the CGO=0 path falls back to LSP `documentSymbol` per `internal/repomap/lspfallback.go`. |
| Generated LSP 3.17 types | `protocol/gen/*.go` | Generated Go source (commit-time, not runtime) | 324 structs + 216 union types compiled into the binary. The generator (`cmd/lspgen/`) reads `protocol/metaModel.json` at *generation time* (not runtime); the produced Go files are in-binary. |

## External-by-Design

Assets the shipped `helix` binary reads from disk at runtime by deliberate policy. Each row cites the locked decision (or the canonical-by-design class) that scopes it as out-of-binary.

| Asset | Read Site | Rationale | Decision Reference |
|-------|-----------|-----------|--------------------|
| **52 Language Server binaries** (gopls, jdtls, rust-analyzer, pyright-langserver, typescript-language-server, clangd, ccls, csharp-ls, OmniSharp, kotlin-language-server, metals, pwsh+PowerShellEditorServices, fsautocomplete, elixir-ls, haskell-language-server-wrapper, lua-language-server, julia, clojure-lsp, terraform-ls, groovy-language-server, regal, vue-language-server, jedi-language-server, ty, bash-language-server, ruby-lsp, solargraph, intelephense, phpactor, perl, dart, erlang_ls, fortls, yaml-language-server, taplo, marksman, zls, nixd, ocamllsp, lean, luau-lsp, solidity-ls, verible-verilog-ls, ansible-language-server, elm-language-server, pasls, R, al-language-server, sourcekit-lsp, shader_language_server, matlab-language-server) | `internal/langregistry/installer.go` (download → cache; PATH lookup; managed bin-dir resolution) | LS binaries average tens-to-hundreds of MB; jdtls ~200MB, rust-analyzer ~30MB. Embedding all 52 would push archives to GB-class which goreleaser-CI cannot realistically build, sign, and reproducibility-verify on tag in reasonable time. The three-tier installer (PATH > managed download > error) is the documented external-download surface. | **D-15** |
| `~/.helix/serena_config.yml` (user serena_config / globally-resolvable koanf layer) | `internal/config/` (loaded via koanf 4-layer precedence) | User-mutable global config; survives across helix versions | CONTEXT.md general principle (post-`serena→helix` rename); D-02 |
| `<project>/.helix/project.yml` | `internal/config/` (project-marker resolution) | Per-project marker; users version-control or gitignore at their discretion | **D-02** |
| `<project>/.helix/memories/*.md` | `internal/memory/store.go:92,128` (`MemoryStore.Read`) | User-mutable memory store; FTS5-indexed via `internal/memory/index.go:217` | by design (`<domain>` of CONTEXT.md) |
| Memory FTS5 index file (`memories.db` or equivalent SQLite) | `internal/memory/index.go` (sqlite open in store dir) | Regeneratable cache of `.md` memories; mtime-keyed | by design |
| Memory `.md` files via fsnotify watcher | `internal/memory/watcher.go:142` | Watcher re-reads on user edit; same files as MemoryStore.Read | by design |
| RepoMap SQLite tag cache | `internal/repomap/` cache (mtime-keyed, regenerated on demand) | Cache of `.scm`-extracted tags keyed by file mtime — disposable | by design |
| `~/.gemini/mcp-server-enablement.json` (Gemini CLI registrar) | `internal/cli/setup_clients.go:334,357` | User's Gemini-CLI client config; helix only sets the `helix:enabled` key | **D-05** (per-client setup) |
| `<project>/.mcp.json`, `<homedir>/.config/<client>/mcp.json` (MCP client config) | `internal/cli/setup_clients.go:64,98` | User's MCP client registration target (Claude Code, VS Code, JetBrains, OpenCode, Gemini, Claude Desktop, generic) | **D-05** |
| `~/.claude/settings.json` (Claude Code hooks) | `internal/cli/setup_hooks.go:79,130` | User's Claude Code hooks file; helix appends `helix_managed: true` matchers | **D-05** (hooks installer) |
| `~/.helix/sessions/<session>.json` (session stats — nudge tool) | `internal/cli/nudge.go:112` | Per-session counter file mutated each tool call | by design (nudge is a session-scoped affordance) |
| Workspace project files (every file under the activated workspace) | `internal/kernel/fileops/read.go:36,56`, `internal/kernel/fileops/search.go:113,129`, `internal/kernel/edit/{rename,delete,insert,replace}.go`, `internal/kernel/diag/format.go:66`, `internal/repomap/render.go:119`, `internal/skill/repomap/skill.go:369` | Workspace source files are the *primary* user-supplied input to MCP tools (read_file, search_in_files, replace_symbol_body, find_definition, …). The whole product purpose is to read these. | by design (core MCP tool surface) |
| Workspace files for LS `didOpen` priming | `internal/kernel/lspool/quirks.go:526,559,588,618` | Some LSes (tsserver, pyright, jdtls, rust-analyzer) require a `didOpen` notification to create their internal "project" before workspace/symbol works | by design (LS protocol requirement) |
| `/proc/pressure/memory`, `/proc/<pid>/status` | `internal/kernel/lspool/pressure_linux.go:22,62` | Linux PSI / VmRSS readings for the platform-aware memory pressure eviction policy | by design (kernel API) |
| `os.Executable()` self-path probe | `internal/forwarder/dial.go:84`, `internal/cli/setup.go:163`, `internal/upgrade/upgrade.go:122` | Used to relaunch helix as daemon (forwarder), record install location for setup output, and target the in-place atomic swap (upgrade). NOT reading a data asset — reading the *binary's own location* on disk. | by design |
| Upgrade-time download artifacts: archive + signature + (optional) checksums | `internal/upgrade/upgrade.go:312,335`, `internal/upgrade/archive.go:28,81` | Network-fetched, sibling-of-install stage dir, retained on signature failure for postmortem; cleaned otherwise | **D-07** |
| User-mutable language-registry override YAMLs | `internal/langregistry/registry.go:51` (silently skipped if missing) | Allows users to override `defaultEntries` (command path, args, install URL/SHA) without recompiling | by design (extensibility) |
| User-mutable profile / mode override YAML directories | `internal/profile/loader.go:106,147` (silently skipped if missing) | `LoadOverrides(profileDir, modeDir)` lets users layer their own profiles on top of the embedded set | by design (extensibility) |
| Skill context / mode YAML overrides | `internal/skill/spec.go:44,57` (callable as `LoadContextSpecs(path)` / `LoadModeSpecs(path)`) | Configurable extension points for context/mode definitions read from a user-supplied path; only exercised by tests today, but the API is part of the skill-system extensibility surface | by design (extensibility; same posture as `internal/profile/loader.go`) |

## Gaps Closed

Items the audit found that should have been embedded but weren't. Each was closed in this plan; the file edits are listed for traceability.

**No gaps found** — every runtime asset the shipped `helix` binary reads was already either embedded via `//go:embed` (24 directives + 3 implicit/structural embeds) or external-by-design (14 grouped categories with locked-decision citations).

This is the desired outcome: the embed surface produced by Phase 31 (RepoMap + Edit), Phase 02-06 (early kernel), Phase 22-24 (typed errors / generated LSP types), Phase 51 (signing pipeline / minisign at repo root), and Phase 52 Plans 01–04 (binary rename, `cmd/helix`, embedded `minisign.pub`, upgrade pipeline) was already comprehensive enough that no production-code edits were needed in Plan 05 to fulfill D-14's invariant.

| Item | Pre-state | Post-state | Edit |
|------|-----------|------------|------|
| (none) | — | — | — |

## Deferred Gaps

Items the audit identified as gaps but explicitly deferred to a future phase, with the locked-decision-or-constraint that justifies deferral per the planner-authority-limits.

**No deferred gaps.** Every entry in the surface is fully classified.

| Item | Why Deferred | Follow-up Phase |
|------|--------------|-----------------|
| (none) | — | — |

## Out-of-Scope Findings (Tracked for Possible Future Cleanup)

Findings encountered during the audit that are **not** runtime asset reads and **not** gaps — but are worth recording for completeness:

| Finding | File(s) | Why Out-of-Scope | Possible Cleanup Phase |
|---------|---------|------------------|------------------------|
| `internal/kernel/edit/queries/*.scm` (22 files) checked into the repo as Phase-31-era reference for `treesitter.go::NewBodyExtractor`'s `langConfig` map | `internal/kernel/edit/queries/{bash,c,cpp,csharp,go,haskell,hcl,java,javascript,julia,kotlin,lua,ocaml,php,python,r,ruby,rust,scala,swift,typescript,zig}.scm` | Verified zero references to `edit/queries` from any Go file; never read at runtime. They duplicate (in `.scm` syntax) what `langConfig` declares programmatically. Not a gap because no `os.ReadFile` consumes them; not embedded because nothing needs to embed them. | Phase 53+ housekeeping — delete the dead reference directory or convert `treesitter.go` to consume them via `//go:embed`, which would unify the body-extraction pattern with the RepoMap one. Not urgent; not in scope per the executor's `<scope_boundary>` rule on out-of-task discoveries. |

## Future-Audit Trigger

A future phase **MUST** update this manifest if it:

- Adds a new `os.ReadFile` / `os.Open` site in production code (non-test, non-legacy, non-`cmd/docgen`-non-`cmd/lspgen`).
- Adds a new `//go:embed` directive (so future contributors can confirm it's accounted for).
- Adds a new external runtime download path beyond the three-tier LS installer and the upgrade subcommand pair.
- Adds a new `embed.FS` walk consumer (e.g., a future skill that ships its own embedded markdown corpus).
- Adds a new package whose primary responsibility is reading a kind of asset (e.g., a hypothetical `internal/templates/` skill).

The CHANGELOG / CONTRIBUTING entries that Plan 06 will produce cross-link to this manifest so PR reviewers consult it before merging any new disk-read.

## Threat Model Coverage (cross-reference)

| Threat ID | Coverage |
|-----------|----------|
| **T-52-05-01** (silent disk-dependency growth) | Future-Audit Trigger section above + Plan 06 CHANGELOG/CONTRIBUTING cross-links |
| **T-52-05-02** (secrets in embed) | The single non-trivial new embed in Phase 52 is `minisign.pub` (a *public* key — by name and by content). Every other embedded asset in the manifest above is YAML / `.scm` query / generated Go code — none contain secrets. A future PR adding a `.key`/`.pem`/`token` embed would surface in a manifest update and trigger manual review. |
| **T-52-05-03** (audit silently misses a category) | Three independent grep commands (`//go:embed`, `os.ReadFile`/`os.Open`, `filepath.Join`) used; counts and overlap reasoning recorded in the **Audit Method** section. The 38 classified groups + the explicit out-of-scope discussion of `edit/queries` and `cmd/docgen`/`cmd/lspgen` show the audit considered every grep hit. |
