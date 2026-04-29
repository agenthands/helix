---
phase: 51-packaging-goreleaser
plan: 03
subsystem: packaging
tags: [packaging, cgo, treesitter, build-tags, gap-closure, partial-resolution]
requires: [51-01, 51-02]
provides: [cgo-optional-r-swift-bindings, def-51-01-r-swift-portion-resolved, def-51-02-upstream-bindings-discovered]
affects: [internal/treesitter/registry.go, .planning/phases/51-packaging-goreleaser/deferred-items.md]
tech-stack:
  added: []
  patterns: [build-tag-gated-cgo-with-nocgo-stub, nil-guarded-registration]
key-files:
  created:
    - internal/treesitter/bindings/r/binding_nocgo.go
    - internal/treesitter/bindings/swift/binding_nocgo.go
  modified:
    - internal/treesitter/bindings/r/binding.go
    - internal/treesitter/bindings/swift/binding.go
    - internal/treesitter/registry.go
    - .planning/phases/51-packaging-goreleaser/deferred-items.md
decisions:
  - "Did NOT silently expand 51-03 scope to fix the 19 upstream tree-sitter Go bindings discovered during Task 4. Recorded as DEF-51-02 per plan line 74 directive."
  - "DEF-51-01 status flipped to PARTIALLY RESOLVED rather than RESOLVED. The R/Swift-specific build-constraint errors are gone, but full CGO=0 build still fails on 19 upstream bindings."
metrics:
  completed: 2026-04-29
  duration: ~25min
  tasks: 4
---

# Phase 51 Plan 03: CGO-Optional Bindings Summary

Split the locally-vendored R and Swift tree-sitter bindings into CGO-on (`binding.go` with `//go:build cgo`) and CGO-off (`binding_nocgo.go` with `//go:build !cgo`) variants, and nil-guarded their registration in `internal/treesitter/registry.go` so a CGO_ENABLED=0 build registers 21 of 23 grammars instead of failing -- but discovered during smoke testing that 19 upstream tree-sitter Go bindings are ALSO CGO-only, so the goreleaser pipeline remains structurally blocked (now tracked as DEF-51-02).

## Files Modified / Created

| File | Description |
|------|-------------|
| `internal/treesitter/bindings/r/binding.go` | Prepended `//go:build cgo` tag; added DEF-51-01 reference in doc comment. CGO-on body is byte-identical to before. |
| `internal/treesitter/bindings/r/binding_nocgo.go` (NEW) | `//go:build !cgo` stub returning nil from `Language()`. |
| `internal/treesitter/bindings/swift/binding.go` | Prepended `//go:build cgo` tag; added DEF-51-01 reference in doc comment. |
| `internal/treesitter/bindings/swift/binding_nocgo.go` (NEW) | `//go:build !cgo` stub returning nil from `Language()`. |
| `internal/treesitter/registry.go` | Replaced unconditional `r.languages["r"] = NewLanguage(...)` and `r.languages["swift"] = NewLanguage(...)` with `if ptr := ...; ptr != nil` nil-guards. gofmt also reordered import lines alphabetically within the existing import blocks (cosmetic). |
| `.planning/phases/51-packaging-goreleaser/deferred-items.md` | Flipped DEF-51-01 status OPEN -> PARTIALLY RESOLVED with Resolution subsection; appended DEF-51-02 documenting the 19 upstream tree-sitter bindings that are also CGO-only. |

## Commits

| Task | Hash | Message |
|------|------|---------|
| 1 | `18a13e41` | feat(51-03): add //go:build cgo tag to R and Swift tree-sitter bindings |
| 2 | `7161dc97` | feat(51-03): add no-CGO stub bindings for R and Swift tree-sitter |
| 3 | `ebeaf8f6` | feat(51-03): nil-guard R and Swift language registration in registry |
| 4 | `42f7b186` | docs(51-03): mark DEF-51-01 partially resolved; add DEF-51-02 for upstream tree-sitter bindings |

## Smoke Test Results

| Test | Result | Notes |
|------|--------|-------|
| `CGO_ENABLED=1 go build ./cmd/serena` | exit 0 | CGO-on path byte-identical; binary builds cleanly. |
| `CGO_ENABLED=1 go vet ./...` | exit 0 | Whole-repo vet passes (one pre-existing C macro warning in vendored swift scanner.c, unrelated). |
| `CGO_ENABLED=1 go test ./...` | exit 0 | All 38 packages pass (cli, daemon, kernel/*, mcp, treesitter, repomap, integration tests, etc.). R and Swift parsers continue to register and work. |
| `CGO_ENABLED=1 go test ./internal/treesitter/...` | exit 0 | Treesitter-specific subset passes. |
| `CGO_ENABLED=0 go build ./cmd/serena` | **FAIL** | But for a different reason than DEF-51-01 documented: the R/Swift errors are GONE; the failure is now `build constraints exclude all Go files` on 19 upstream tree-sitter packages (go, python, rust, typescript (LanguageTypescript + LanguageTSX), java, c, cpp, c-sharp, ruby, php, javascript, kotlin, scala, bash, haskell, julia, ocaml, lua, zig, hcl). See DEF-51-02. |
| `CGO_ENABLED=0 go test ./...` | FAIL | Same root cause: `internal/treesitter` and downstream packages (repomap, skill/repomap, integration tests, test/bench) cannot compile when 19 upstream bindings are excluded. Also surfaced an unrelated `internal/obs` test failure (`TestMetrics_RegisteredFamilies`: missing "process_resident_memory_bytes" metric family) -- this is platform-specific (likely a macOS-vs-Linux Prometheus collector difference) and is NOT introduced by this plan. Recorded as out-of-scope; will be re-examined when DEF-51-02 unblocks the rest of the suite. |
| `make release-snapshot` | NOT RUN | Goreleaser is locally installed (`/opt/homebrew/bin/goreleaser`), but running `make release-snapshot` would fail at the same upstream-binding wall. The R/Swift fix is necessary but not sufficient; running goreleaser before DEF-51-02 closes would only re-confirm the failure mode already documented. Saved a few minutes of execution time and ~zero new information. |

No test was deleted, skipped, or build-tagged to make the suites pass. The CGO_ENABLED=0 test failure is a pre-existing structural issue (DEF-51-02), not a regression introduced by this plan.

## Deviations from Plan

### Auto-fixed Issues

None -- the three source-code edits (Tasks 1, 2, 3) executed exactly as the plan prescribed. `gofmt` reordered the import lines in `registry.go` alphabetically within their existing comment-grouped blocks; this is a cosmetic side effect of running gofmt on the file (mandated by the plan) and is documented in the Task 3 commit body.

### Stopped per Plan Directive (Task 4)

**[Plan line 74] Unexpected scope: 19 upstream tree-sitter bindings are also CGO-only.**

- **Found during:** Task 4 Step A (`CGO_ENABLED=0 go build ./cmd/serena`).
- **Issue:** After the R/Swift fix landed, the CGO_ENABLED=0 build no longer fails on `internal/treesitter/bindings/r` or `.../swift` -- but it now fails with 19 distinct `build constraints exclude all Go files` errors against upstream packages (`tree-sitter-go`, `tree-sitter-python`, ..., 19 in total). Investigation of one upstream binding (`/Users/.../tree-sitter-go@v0.25.0/bindings/go/binding.go`) confirmed: every official tree-sitter-* Go binding upstream uses `import "C"` with no non-CGO fallback, identical in shape to the pre-fix R/Swift bindings. They cannot be edited from this repo.
- **Decision:** Did NOT expand 51-03 scope to vendor + stub all 19 upstream packages. Per Plan 51-03 line 74 ("If unexpected scope surfaces during execution ... the executor SHOULD STOP and document the discovery in deferred-items.md as DEF-51-02 ... Do NOT silently expand scope"), the finding is recorded as DEF-51-02 with four explicit resolution paths (vendor-and-stub all 19, gate the whole `internal/treesitter` package behind `//go:build cgo`, re-enable CGO in goreleaser, or wait for upstream pure-Go bindings).
- **Plan-level impact:** Plan 51-03 success_criteria #1 ("CGO_ENABLED=0 go build ./cmd/serena exits 0") and #3 ("CGO_ENABLED=0 go test ./... exits 0") are NOT met. The plan's premise that R and Swift were the only CGO blockers turned out to be wrong. The R/Swift portion of the fix (success_criteria #5, #6, #7) IS complete. CGO_ENABLED=1 path (success_criteria #2, #4) is verified intact.
- **Status update:** DEF-51-01 flipped from OPEN to PARTIALLY RESOLVED rather than RESOLVED, with a clear pointer to DEF-51-02 for the residual blocker. ROADMAP success criterion 1 ("6 archives") remains structurally blocked until DEF-51-02 closes; Plans 51-04 / 51-05 / 51-06 build on archive existence and their end-to-end UATs are still blocked.

## Authentication Gates

None.

## Threat Flags

No new threat surface introduced. The R/Swift portion of this plan is byte-identical CGO behavior on the CGO=1 path and a graceful "language not registered" degradation on the CGO=0 path -- which already had no exposed surface to degrade because the goreleaser pipeline cannot produce a CGO=0 binary at all (DEF-51-02). All STRIDE threats T-51-15 through T-51-20 listed in the plan's threat register were mitigated as designed (build tags placed correctly on line 1, stub package names match, nil-check guards prevent NewLanguage(nil), CGO=1 path proven unaffected, etc.).

## Cross-Reference

- Phase 51 success criterion 1 ("6 archives via goreleaser") is **NOT** unblocked by this plan in isolation. It will only unblock once DEF-51-02 closes (architectural decision required: vendor-and-stub all 19 upstream bindings, gate whole `internal/treesitter` package behind cgo, or re-enable CGO in goreleaser).
- Plans 51-04 (signing ceremony), 51-05 (reproducibility doc), and 51-06 (release.yml hardening) all build on archive existence. Their end-to-end UATs remain blocked until DEF-51-02 is resolved. Their plan documents and code edits can still be authored and committed in this phase, but they cannot be observed end-to-end on a real v* tag push.
- VERIFICATION.md gap "Tagging a release ... uploads 6 platform/arch binaries to GitHub Releases" -- partially de-risked (R/Swift portion eliminated), but not closed.

## Self-Check: PASSED

Verified existence of all created files and commits before writing this section.

- `internal/treesitter/bindings/r/binding.go` -- FOUND (modified, line 1 = `//go:build cgo`)
- `internal/treesitter/bindings/r/binding_nocgo.go` -- FOUND (new, line 1 = `//go:build !cgo`)
- `internal/treesitter/bindings/swift/binding.go` -- FOUND (modified, line 1 = `//go:build cgo`)
- `internal/treesitter/bindings/swift/binding_nocgo.go` -- FOUND (new, line 1 = `//go:build !cgo`)
- `internal/treesitter/registry.go` -- FOUND (modified; contains `if ptr := tree_sitter_r_local.Language(); ptr != nil` and `if ptr := tree_sitter_swift_local.Language(); ptr != nil`)
- `.planning/phases/51-packaging-goreleaser/deferred-items.md` -- FOUND (DEF-51-01 status = PARTIALLY RESOLVED, DEF-51-02 added)
- Commit `18a13e41` -- FOUND in git log (Task 1)
- Commit `7161dc97` -- FOUND in git log (Task 2)
- Commit `ebeaf8f6` -- FOUND in git log (Task 3)
- Commit `42f7b186` -- FOUND in git log (Task 4)
