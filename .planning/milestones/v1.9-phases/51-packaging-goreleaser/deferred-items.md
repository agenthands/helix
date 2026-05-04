# Deferred Items - Phase 51: packaging-goreleaser

## DEF-51-01: CGO_ENABLED=0 build fails on Swift / R tree-sitter bindings

**Discovered during:** Plan 51-01 Task 2 (`.goreleaser.yaml` -- local snapshot smoke test)

**Symptom:** `goreleaser release --snapshot --clean --skip=sign` fails immediately during the build matrix:

```
build failed: exit status 1: package github.com/postfix/serena/cmd/serena
  imports github.com/postfix/serena/internal/cli
  imports github.com/postfix/serena/internal/daemon
  imports github.com/postfix/serena/internal/kernel/edit
  imports github.com/postfix/serena/internal/treesitter
  imports github.com/postfix/serena/internal/treesitter/bindings/r:
    build constraints exclude all Go files in internal/treesitter/bindings/r
  imports github.com/postfix/serena/internal/treesitter/bindings/swift:
    build constraints exclude all Go files in internal/treesitter/bindings/swift
```

**Root cause:** `internal/treesitter/bindings/r/` and `internal/treesitter/bindings/swift/` use CGO-gated build constraints. The goreleaser config sets `CGO_ENABLED=0` (per RESEARCH Pattern 1, Project constraint "single static binary, no Python/Docker/runtime deps") which excludes these source files from compilation. The packages have no non-CGO fallback, so the import graph cannot resolve.

**Why deferred (not fixed in Plan 51-01):**
- Out of scope for this plan (`<files_modified>` is `.goreleaser.yaml`, `release.yml`, `minisign.pub`, `publish.yml` only).
- Touching `internal/treesitter/bindings/{r,swift}/` is a Go source change with a much larger blast radius (architectural).
- The plan's acceptance criteria for Task 2 is `goreleaser check` exits 0 (which it does after the `archives.builds`->`archives.ids` fix). Behavior 1 (six archive matrix) is sampled in Plan 02 / phase gate (per RESEARCH §"Validation Architecture / Sampling Rate").
- CLAUDE.md states "Serena ships as a single Go binary with no CGO dependencies." This deferred item documents that the current main branch violates that invariant for tree-sitter bindings r + swift.

**Resolution paths (for a future phase):**
1. Make r + swift bindings CGO-optional via build tags (e.g. add `//go:build !nocgo` and provide stub fallbacks under `//go:build nocgo`).
2. Vendor pure-Go tree-sitter parsers for r and swift (likely not available upstream).
3. Re-enable CGO in the goreleaser builds: env (`CGO_ENABLED=1`) and accept cross-compile complexity (different CGO toolchains per target OS/arch). This contradicts the "no runtime deps" invariant.

**Recommendation:** Add a Phase 52+ task to make r and swift bindings CGO-optional (option 1) so `CGO_ENABLED=0` cross-compilation succeeds. Until then, the goreleaser pipeline's first real tag push will fail at CI and the maintainer must resolve before Phase 51 success criterion 1 can be observed end-to-end.

**Severity:** HIGH (Plan 51-01 cannot be smoke-tested locally; the first CI run on a real `v*` tag will fail until this is resolved). The goreleaser config itself is correct and lints clean.

**Status:** PARTIALLY RESOLVED -- The R/Swift-specific build-constraint errors are fixed by Plan 51-03 (build tags + non-CGO stubs + nil-guarded registry); see Resolution section below. However, during 51-03 execution it was discovered that **all 19 upstream tree-sitter Go bindings are also CGO-only** -- not just R and Swift. Full CGO_ENABLED=0 build of `./cmd/serena` therefore still fails. The remaining (much larger) scope is now tracked as DEF-51-02. SC-1 ("6 archives via goreleaser") remains structurally blocked until DEF-51-02 is also closed.

## Resolution (DEF-51-01 R/Swift-specific portion)

Resolved by Plan 51-03 on 2026-04-29. The fix:
- Added `//go:build cgo` tag to `internal/treesitter/bindings/{r,swift}/binding.go`.
- Added `internal/treesitter/bindings/{r,swift}/binding_nocgo.go` stubs (`//go:build !cgo`) returning nil from `Language()`.
- Updated `internal/treesitter/registry.go` to nil-check `Language()` before `NewLanguage`; skips registration for r and swift under CGO_ENABLED=0.

Smoke test result (Plan 51-03 Task 4):
- `CGO_ENABLED=0 go build ./cmd/serena` -- **STILL FAILS**, but for a different reason: 19 upstream tree-sitter bindings (go, python, rust, typescript, java, c, cpp, c-sharp, ruby, php, javascript, kotlin, scala, bash, haskell, julia, ocaml, lua, zig, hcl) all hit `build constraints exclude all Go files`. The R/Swift contribution to that error list is gone (locally vendored bindings now compile under CGO_ENABLED=0 -- they just register no language). The remaining 19 errors are upstream and out of 51-03 scope.
- `CGO_ENABLED=1 go build ./cmd/serena` -- exit 0, byte-identical CGO behavior preserved.
- `CGO_ENABLED=1 go test ./...` -- exit 0 (all 38 packages pass; treesitter and dependents continue to register and use R/Swift parsers).
- `CGO_ENABLED=0 go test ./...` -- fails because `internal/treesitter` and downstream packages (repomap, integration tests) cannot compile when 19 upstream bindings are excluded. Same root cause as the build failure; documented in DEF-51-02.
- `make release-snapshot` -- not run; would fail at the same upstream-binding wall as `CGO_ENABLED=0 go build`.

Net effect of the R/Swift portion of the fix: when (and only when) DEF-51-02 lands a workable strategy for the upstream bindings, the R/Swift portion of the fix lets the CGO_ENABLED=0 binary support 21 tree-sitter languages (R and Swift omitted, gracefully). CGO_ENABLED=1 binaries continue to support all 23 languages. The R/Swift code is no longer an obstacle to CGO_ENABLED=0 cross-compilation.

---

## DEF-51-02: Upstream tree-sitter Go bindings are CGO-only across the board

**Discovered during:** Plan 51-03 Task 4 Step A (CGO_ENABLED=0 go build smoke).

**Symptom:** `CGO_ENABLED=0 go build ./cmd/serena` fails with 19 errors of the form:

```
github.com/tree-sitter/tree-sitter-go/bindings/go: build constraints exclude all Go files in /Users/.../tree-sitter-go@v0.25.0/bindings/go
github.com/tree-sitter/tree-sitter-python/bindings/go: build constraints exclude all Go files in ...
github.com/tree-sitter/tree-sitter-rust/bindings/go: build constraints exclude all Go files in ...
github.com/tree-sitter/tree-sitter-typescript/bindings/go: ...
github.com/tree-sitter/tree-sitter-java/bindings/go: ...
github.com/tree-sitter/tree-sitter-c/bindings/go: ...
github.com/tree-sitter/tree-sitter-cpp/bindings/go: ...
github.com/tree-sitter/tree-sitter-c-sharp/bindings/go: ...
github.com/tree-sitter/tree-sitter-ruby/bindings/go: ...
github.com/tree-sitter/tree-sitter-php/bindings/go: ...
github.com/tree-sitter/tree-sitter-javascript/bindings/go: ...
github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go: ...
github.com/tree-sitter/tree-sitter-scala/bindings/go: ...
github.com/tree-sitter/tree-sitter-bash/bindings/go: ...
github.com/tree-sitter/tree-sitter-haskell/bindings/go: ...
github.com/tree-sitter/tree-sitter-julia/bindings/go: ...
github.com/tree-sitter/tree-sitter-ocaml/bindings/go: ...
github.com/tree-sitter-grammars/tree-sitter-lua/bindings/go: ...
github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go: ...
github.com/tree-sitter-grammars/tree-sitter-hcl/bindings/go: ...
```

(Total: 19 distinct upstream packages -- 20 import-line errors because tree-sitter-typescript exposes both `LanguageTypescript` and `LanguageTSX` via the same package. R/Swift errors are GONE post-51-03.)

**Root cause:** Every official tree-sitter-* Go binding upstream is structured as a single `binding.go` with `import "C"` and a CGO `#include` of the parser sources, e.g.:

```go
package tree_sitter_go

// #cgo CFLAGS: -std=c11 -fPIC
// #include "../../src/parser.c"
// #if __has_include("../../src/scanner.c")
// #include "../../src/scanner.c"
// #endif
import "C"

import "unsafe"

func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_go())
}
```

When the `import "C"` directive is present in any source file, the Go toolchain treats the whole file as cgo-implicit and applies an implicit `//go:build cgo` constraint. Under `CGO_ENABLED=0`, the file is excluded -- and because no other Go source exists in those binding packages, the toolchain reports "build constraints exclude all Go files."

This is the same structural problem that DEF-51-01 documented for R and Swift -- but DEF-51-01's R/Swift fix only addressed the two locally-vendored bindings. The 19 upstream packages cannot be edited from this repo.

**Why the 51-03 plan missed this:**
- Plan 51-03 was scoped to the two locally-vendored packages where the project has direct write access. The plan's "<must_haves>" assumed the only CGO blockers were R and Swift (because those were the only two error lines visible in DEF-51-01's quoted output).
- DEF-51-01's quoted goreleaser output stops at the R/Swift errors because Go's import-cycle / build-constraint reporter short-circuits on the first failing import in the dependency graph -- it does NOT enumerate every CGO-gated package. If R and Swift are removed from the graph, the next 19 errors surface. This was not visible until 51-03 actually built a CGO=0 binary post-stub-creation.
- Per Plan 51-03 line 74 ("STOP and document the discovery in deferred-items.md as DEF-51-02 ... Do NOT silently expand scope"), this finding is recorded here rather than fixed in 51-03.

**Resolution paths (for a future phase):**
1. **Vendor + stub all 19 bindings (mirror the R/Swift pattern).** For each upstream binding, copy the CGO source into `internal/treesitter/bindings/<lang>/binding.go` with `//go:build cgo`, write a `binding_nocgo.go` returning nil, and update `registry.go` with 19 more nil-guarded calls. Pros: same proven pattern; CGO=1 binaries still parse 23 languages; CGO=0 binaries register 0 tree-sitter languages but still build and run with LSP-only RepoMap. Cons: ~19 packages * ~3 files * vendored source -- significant repo footprint; ongoing maintenance burden to track upstream parser updates; CGO=0 binaries lose ALL tree-sitter coverage which materially degrades RepoMap quality on user machines that get the no-CGO build.
2. **Move tree-sitter behind a CGO-required interface.** Restructure `internal/treesitter/` so the package itself has a `//go:build cgo` constraint, with a `//go:build !cgo` stub that returns "tree-sitter unavailable" from every public method. Callers (RepoMap, edit subsystem) must already handle the not-registered case via the (lang, ok) idiom. Pros: minimal new code; clean separation; no per-binding vendoring. Cons: CGO=0 binaries have NO tree-sitter capability, only LSP `documentSymbol` fallback; harder to opt into "21 of 23 languages" partial coverage.
3. **Re-enable CGO in goreleaser and accept cross-compile complexity.** Set `CGO_ENABLED=1` in `.goreleaser.yaml`, install a per-target C toolchain (e.g. `zig cc`, `xx`, or platform-specific cross-compilers) in the release CI image. Pros: keeps full 23-language coverage in release binaries; no source restructuring. Cons: directly contradicts the CLAUDE.md invariant "single Go binary with no CGO dependencies"; goreleaser CGO cross-compile is fragile (different toolchain per OS/arch); release CI image becomes much heavier.
4. **Wait for upstream pure-Go tree-sitter bindings.** Tree-sitter has an experimental WASM runtime with pure-Go execution paths, but it is not API-compatible with the current `unsafe.Pointer` Language() interface. Watch upstream; revisit in 6-12 months.

**Recommendation:** Defer to a dedicated phase (Phase 52 or later). The decision between paths 1, 2, and 3 is architectural and requires explicit sign-off on the trade-off between binary release surface (CGO=0 invariant) and runtime feature coverage (21+ tree-sitter parsers). Path 2 has the smallest 51-related footprint; path 3 keeps the strongest user feature parity but breaks the no-CGO invariant; path 1 is a middle ground but materially expands the repo.

**Severity:** HIGH -- blocks Phase 51 success criterion 1 (the goreleaser pipeline produces 6 platform/arch archives). Plan 51-01's release matrix cannot run end-to-end on a real v* tag push until this is resolved. Plans 51-04 (signing), 51-05 (reproducibility doc), and 51-06 (release.yml hardening) build on archive existence -- their UATs are also blocked until DEF-51-02 closes.

**Status:** RESOLVED via Phase 51.1 (Path 2). See `.planning/phases/51.1-cgo-treesitter-gate-gate-internal-treesitter-behind-go-build/` for PLAN, RESEARCH, and SUMMARY. The `internal/treesitter` package is now gated behind `//go:build cgo`; the daemon refuses to start under `CGO_ENABLED=0` with a clear remediation message. CGO=0 archives build but are placeholders -- DEF-51-03 (re-enable CGO in goreleaser, Path 3) tracks the follow-up to make them functional.

---

## DEF-51-03: re-enable CGO in goreleaser (Path 3 closure tracking)

**Originally referenced in:** Phase 51.1 SUMMARY discussion of "DEF-51-03 candidate" (Path 3 from DEF-51-02 resolution paths).

**Status:** RESOLVED via Phase 59.1 (FALLBACK-B-MULTI-BUILD-ID).

**Resolution:**

Phase 59.1 closes DEF-51-03 by inverting Phase 51.1 D-02 (the daemon's `treesitter.Available` refusal under CGO=0) into single-mode CGO=1. See:

- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-CONTEXT.md` — locked decisions D-01..D-20 (D-15..D-19 are the split-runner contingency; D-20 is the GoReleaser Pro lock-in / FALLBACK-B canonicalization)
- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-01-SUMMARY.md` — Wave 1: source-tree CGO=1 flatten (12 `_nocgo.go` deletions; 24 `//go:build cgo` strips; daemon step-6a deletion; D-14 Path-3 windows-arm64 platform stub)
- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-02-SUMMARY.md` — Wave 2: `.goreleaser.yaml` flipped to CGO=1 with two `builds:` entries (`helix-non-darwin` zig cc + `helix-darwin` Apple clang); Makefile zig-presence guard
- `.planning/phases/59.1-drop-cgo-0-single-mode-cgo-1-build-release/59.1-02-SPLIT-PROBE-NOTES.md` — Wave 2 prerequisite probe: GoReleaser Pro lock-in evidence (the `partial.by_target` / split-release / merge-continue mechanic is Pro-exclusive at every v2.x patch level; see linked probe notes for the verbatim directive names)
- `.github/workflows/release.yml` (Wave 3) — split-runner pipeline: `release-linux` (ubuntu-22.04, zig cc) + `release-darwin` (macos-14, Apple clang, tag-gated) + `release-merge` (uniform cosign across 6 archives)

**Mechanism note (FALLBACK-B-MULTI-BUILD-ID):**

The original DEF-51-02 Path 3 sketch assumed cross-compile complexity would be solved by GoReleaser's native split-by-target mechanic. Wave 2 prerequisite probe (linked above) discovered that mechanic is **GoReleaser Pro exclusive** (verified directly against https://goreleaser.com/customization/partial/). Helix runs on the OSS distribution and FALLBACK-A (bumping the goreleaser pin) does not unlock the feature because GoReleaser Pro is a paid commercial license, not a different OSS tag. The OSS-supported equivalent is FALLBACK-B-MULTI-BUILD-ID: each runner invokes `goreleaser build --id <runner-id>` with `--snapshot --clean`; the merge job runs `goreleaser release --skip=build` to assemble archives + checksums + signatures from the pre-built binaries. Architectural intent (split topology, per-runner reproducibility, uniform cosign attestation across 6 archives) is preserved entirely; only the syntactic mechanism differs from the original sketch.

**`helix upgrade` artifact-name and `.sigstore.json` bundle layout preserved.** Phase 58 REL-01 contract holds.
