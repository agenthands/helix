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

**Status:** OPEN -- deferred to a separate decision/phase per Plan 51-01 scope boundary.
