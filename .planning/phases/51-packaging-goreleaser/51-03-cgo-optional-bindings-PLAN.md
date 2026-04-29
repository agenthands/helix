---
phase: 51-packaging-goreleaser
plan: 03
type: execute
wave: 3
depends_on: [51-01, 51-02]
files_modified:
  - internal/treesitter/bindings/r/binding.go
  - internal/treesitter/bindings/r/binding_nocgo.go
  - internal/treesitter/bindings/swift/binding.go
  - internal/treesitter/bindings/swift/binding_nocgo.go
  - internal/treesitter/registry.go
  - .planning/phases/51-packaging-goreleaser/deferred-items.md
autonomous: true
gap_closure: true
requirements: [PKG-01]
requirements_addressed: [PKG-01]
tags: [packaging, cgo, treesitter, build-tags, gap-closure]
must_haves:
  truths:
    - "CLOSES VERIFICATION gap 'Tagging a release ... uploads 6 platform/arch binaries' -- DEF-51-01 root cause resolved"
    - "CGO_ENABLED=0 go build ./cmd/serena succeeds without 'build constraints exclude all Go files in internal/treesitter/bindings/{r,swift}'"
    - "make release-snapshot (committed in plan 51-02) produces 6 .tar.gz archives in dist/ on a clean checkout"
    - "CGO_ENABLED=1 go test ./... continues to pass -- the CGO-on path is the original behavior, untouched (R and Swift tree-sitter parsers still register and work)"
    - "CGO_ENABLED=0 go test ./... passes -- the no-CGO stub path returns nil from Language() and registry.go skips registration of r and swift WITHOUT panic"
    - "GrammarRegistry.GetLanguage(r) returns (nil, false) under CGO_ENABLED=0 and (non-nil, true) under CGO_ENABLED=1"
    - "GrammarRegistry.GetLanguage(swift) returns (nil, false) under CGO_ENABLED=0 and (non-nil, true) under CGO_ENABLED=1"
    - "Project invariant from CLAUDE.md (single Go binary with no CGO dependencies) is restored for the goreleaser-built artifacts; CGO-only language coverage degrades gracefully rather than failing the build"
    - "deferred-items.md DEF-51-01 status flipped from OPEN to RESOLVED with a pointer to this plan"
  artifacts:
    - path: "internal/treesitter/bindings/r/binding.go"
      provides: "CGO-only R tree-sitter binding, gated by //go:build cgo"
      contains: ["//go:build cgo", "package tree_sitter_r", "#cgo CFLAGS"]
    - path: "internal/treesitter/bindings/r/binding_nocgo.go"
      provides: "Non-CGO stub for R bindings; Language() returns nil so registry can skip registration"
      contains: ["//go:build !cgo", "package tree_sitter_r", "func Language() unsafe.Pointer", "return nil"]
    - path: "internal/treesitter/bindings/swift/binding.go"
      provides: "CGO-only Swift tree-sitter binding, gated by //go:build cgo"
      contains: ["//go:build cgo", "package tree_sitter_swift", "#cgo CFLAGS"]
    - path: "internal/treesitter/bindings/swift/binding_nocgo.go"
      provides: "Non-CGO stub for Swift bindings; Language() returns nil so registry can skip registration"
      contains: ["//go:build !cgo", "package tree_sitter_swift", "func Language() unsafe.Pointer", "return nil"]
    - path: "internal/treesitter/registry.go"
      provides: "Updated registry that nil-checks tree_sitter_r_local.Language() and tree_sitter_swift_local.Language() before NewLanguage; skips registration on nil"
      contains: ["if ptr := tree_sitter_r_local.Language(); ptr != nil", "if ptr := tree_sitter_swift_local.Language(); ptr != nil"]
    - path: ".planning/phases/51-packaging-goreleaser/deferred-items.md"
      provides: "DEF-51-01 marked RESOLVED with pointer to plan 51-03"
      contains: ["**Status:** RESOLVED", "Resolved by Plan 51-03"]
  key_links:
    - from: "internal/treesitter/registry.go (line 85-86)"
      to: "internal/treesitter/bindings/{r,swift}/binding{,_nocgo}.go"
      via: "Language() function call wrapped in nil-check before tree_sitter.NewLanguage"
      pattern: "if ptr := tree_sitter_r_local.Language\\(\\); ptr != nil"
    - from: ".goreleaser.yaml builds.env CGO_ENABLED=0"
      to: "binding_nocgo.go stubs"
      via: "Go toolchain selects binding_nocgo.go via the !cgo build tag when CGO_ENABLED=0"
      pattern: "//go:build !cgo"
---

<objective>
Resolve the foundational blocker (DEF-51-01 / VERIFICATION Concern A) that prevents CGO_ENABLED=0 cross-compile from succeeding. Without this fix, the goreleaser pipeline committed in 51-01 cannot produce a single archive -- the build matrix fails before any binary is emitted, and ROADMAP success criterion 1 (6 archives published) cannot be observed end-to-end on a real v* tag push.

The fix is surgical: split internal/treesitter/bindings/{r,swift}/binding.go into two build-tagged files each. The CGO version (existing C-based parser) is preserved verbatim under //go:build cgo. A new no-CGO stub under //go:build !cgo provides a Language() function returning nil. registry.go is updated with a nil-check around the two registrations so the no-CGO build registers 21 tree-sitter languages instead of 23, gracefully degrading without panicking.

Purpose: Closes VERIFICATION gap "Tagging a release ... uploads 6 platform/arch binaries to GitHub Releases" by removing the build-matrix block. Restores the project invariant from CLAUDE.md ("single Go binary with no CGO dependencies"). Unblocks SC-1, which in turn unblocks the end-to-end UAT for SC-2/SC-3/SC-4.

Architectural risk and rationale for inclusion in Phase 51 (NOT split to Phase 52):
The original phase explicitly deferred this work to Phase 52+, but VERIFICATION.md correctly identifies it as the structural blocker that prevents observing any of the four success criteria end-to-end. Without it, every other gap-closure plan in this wave produces work that cannot be UAT-validated. Scope is bounded:
- 2 packages, 4 files (2 existing edits, 2 new files), and a small registry.go edit
- No new types, no API surface changes, no data migration
- The CGO-on path is byte-identical to today's behavior (the existing binding.go gets one build-tag header line added)
- Smoke-testable in seconds: CGO_ENABLED=0 go build ./cmd/serena either succeeds or fails

If unexpected scope surfaces during execution (e.g., a third package also has CGO-only files we missed, or tree_sitter.NewLanguage(nil) panics in a way that requires upstream library changes), the executor SHOULD STOP and document the discovery in deferred-items.md as DEF-51-02 and surface it via the SUMMARY. Do NOT silently expand scope.

Output: A repo state where CGO_ENABLED=0 go build ./cmd/serena succeeds, CGO_ENABLED=0 make release-snapshot produces 6 archives, and CGO_ENABLED=1 go test ./... continues to pass with full R/Swift parser coverage.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/51-packaging-goreleaser/51-CONTEXT.md
@.planning/phases/51-packaging-goreleaser/51-RESEARCH.md
@.planning/phases/51-packaging-goreleaser/51-VERIFICATION.md
@.planning/phases/51-packaging-goreleaser/deferred-items.md
@./CLAUDE.md
@internal/treesitter/registry.go
@internal/treesitter/bindings/r/binding.go
@internal/treesitter/bindings/swift/binding.go
@.goreleaser.yaml

<interfaces>
The two CGO bindings expose a single function each. Both new stub files MUST
preserve the same signature and unsafe.Pointer return type so registry.go
imports unchanged on the package-level import line.

From internal/treesitter/bindings/r/binding.go (current):
  package tree_sitter_r
  func Language() unsafe.Pointer

From internal/treesitter/bindings/swift/binding.go (current):
  package tree_sitter_swift
  func Language() unsafe.Pointer

From internal/treesitter/registry.go (current, lines 85-86):
  r.languages["r"] = tree_sitter.NewLanguage(tree_sitter_r_local.Language())
  r.languages["swift"] = tree_sitter.NewLanguage(tree_sitter_swift_local.Language())

After this plan, both CGO and no-CGO Language() functions still return
unsafe.Pointer. The no-CGO version returns nil. registry.go nil-checks
before NewLanguage.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Add //go:build cgo tag to existing R and Swift binding.go files</name>
  <files>internal/treesitter/bindings/r/binding.go, internal/treesitter/bindings/swift/binding.go</files>
  <read_first>
    - internal/treesitter/bindings/r/binding.go (full read; current 13 lines, no build tag)
    - internal/treesitter/bindings/swift/binding.go (full read; current 13 lines, no build tag, structurally identical to r/binding.go)
    - .planning/phases/51-packaging-goreleaser/deferred-items.md DEF-51-01 (resolution path 1: build tags + stub fallbacks)
    - ./CLAUDE.md "Constraints" section (reaffirms the no-CGO invariant)
  </read_first>
  <action>
Add a //go:build cgo constraint at the very top of each existing binding.go so the CGO-on toolchain compiles them and the CGO-off toolchain skips them.

The build tag MUST be the very first non-blank line in the file, followed by a single blank line, then the existing package declaration. Go toolchain (1.17+) requires this exact placement; if it is below the package line or below an import block, the constraint is silently ignored.

Edit 1: internal/treesitter/bindings/r/binding.go

Replace the existing file content with this exact content (preserve the existing CGO directives byte-for-byte, only prepending the build tag and updating the doc comment):

  //go:build cgo

  package tree_sitter_r

  // #cgo CFLAGS: -std=c11 -fPIC -Isrc
  // #include "src/parser.c"
  // #include "src/scanner.c"
  import "C"

  import "unsafe"

  // Language returns the tree-sitter Language for R.
  //
  // This file is compiled only when CGO is enabled. The non-CGO build picks up
  // binding_nocgo.go which returns nil; registry.go skips registering "r" in
  // that case. See .planning/phases/51-packaging-goreleaser/deferred-items.md
  // DEF-51-01 for the rationale (CGO_ENABLED=0 cross-compile for goreleaser).
  func Language() unsafe.Pointer {
  	return unsafe.Pointer(C.tree_sitter_r())
  }

Edit 2: internal/treesitter/bindings/swift/binding.go

Apply the same transformation with these substitutions:
- package tree_sitter_swift (instead of tree_sitter_r)
- C.tree_sitter_swift() (instead of C.tree_sitter_r())
- doc comment says "Swift" and "swift" instead of "R" and "r"

Style non-negotiables:
1. The //go:build cgo line is the FIRST line of the file, followed by ONE blank line, followed by the package declaration. Do NOT use the legacy `// +build cgo` form. Do NOT add both forms.
2. Do NOT modify the #cgo CFLAGS, #include, import "C", or function body. The CGO behavior is byte-identical to today.
3. Do NOT add a copyright header -- the rest of the bindings/ tree does not have one.
4. The doc comment on Language() should reference DEF-51-01 so a future maintainer reading this file understands why the build tag exists.
5. After the edit, gofmt -d on each file MUST produce no diff (already-formatted state).
  </action>
  <verify>
    <automated>head -1 internal/treesitter/bindings/r/binding.go | grep -q '^//go:build cgo$' && head -1 internal/treesitter/bindings/swift/binding.go | grep -q '^//go:build cgo$' && sed -n '2p' internal/treesitter/bindings/r/binding.go | grep -qE '^$' && sed -n '3p' internal/treesitter/bindings/r/binding.go | grep -q '^package tree_sitter_r$' && sed -n '3p' internal/treesitter/bindings/swift/binding.go | grep -q '^package tree_sitter_swift$' && gofmt -l internal/treesitter/bindings/r/binding.go internal/treesitter/bindings/swift/binding.go | wc -l | grep -qE '^[[:space:]]*0$' && CGO_ENABLED=1 go vet ./internal/treesitter/bindings/r/... ./internal/treesitter/bindings/swift/...</automated>
  </verify>
  <acceptance_criteria>
    - internal/treesitter/bindings/r/binding.go line 1 is exactly `//go:build cgo`
    - internal/treesitter/bindings/r/binding.go line 2 is blank (Go toolchain requirement)
    - internal/treesitter/bindings/r/binding.go line 3 is exactly `package tree_sitter_r`
    - internal/treesitter/bindings/swift/binding.go line 1 is exactly `//go:build cgo`
    - internal/treesitter/bindings/swift/binding.go line 2 is blank
    - internal/treesitter/bindings/swift/binding.go line 3 is exactly `package tree_sitter_swift`
    - Both files retain `#cgo CFLAGS: -std=c11 -fPIC -Isrc`, `#include "src/parser.c"`, `#include "src/scanner.c"`, `import "C"` lines verbatim
    - Both files retain `func Language() unsafe.Pointer` returning `unsafe.Pointer(C.tree_sitter_<lang>())`
    - `gofmt -l` on both files produces NO output (gofmt-clean)
    - `CGO_ENABLED=1 go vet ./internal/treesitter/bindings/r/... ./internal/treesitter/bindings/swift/...` exits 0
    - `grep -c "DEF-51-01" internal/treesitter/bindings/r/binding.go internal/treesitter/bindings/swift/binding.go` shows at least 1 match per file
  </acceptance_criteria>
  <done>Both existing binding.go files carry //go:build cgo so they are compiled only when CGO is enabled. The CGO-on behavior is byte-identical to before; the CGO-off toolchain now silently excludes them (and would still fail to build until Task 2 lands the stubs).</done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: Create no-CGO stub files for R and Swift bindings</name>
  <files>internal/treesitter/bindings/r/binding_nocgo.go, internal/treesitter/bindings/swift/binding_nocgo.go</files>
  <read_first>
    - internal/treesitter/bindings/r/binding.go (just-edited; confirm package name tree_sitter_r and signature func Language() unsafe.Pointer)
    - internal/treesitter/bindings/swift/binding.go (just-edited; confirm package name tree_sitter_swift)
    - internal/treesitter/registry.go lines 39-40 (import aliases tree_sitter_r_local and tree_sitter_swift_local; the stub package names MUST match exactly)
  </read_first>
  <action>
Create two new Go source files providing no-CGO fallbacks for the R and Swift bindings. Each stub returns nil from Language() and is gated by //go:build !cgo. Together with the //go:build cgo files from Task 1, exactly one of the two files in each package compiles for any given build.

File 1: internal/treesitter/bindings/r/binding_nocgo.go (create with this exact content)

  //go:build !cgo

  package tree_sitter_r

  import "unsafe"

  // Language returns nil when the binary is built with CGO_ENABLED=0.
  //
  // The CGO version of this binding (binding.go) wraps tree-sitter-r's C parser,
  // which cannot cross-compile under CGO_ENABLED=0. The goreleaser pipeline
  // (.goreleaser.yaml, Phase 51) builds with CGO_ENABLED=0 to keep the project
  // invariant of a single static Go binary. Returning nil here is paired with a
  // nil-check in internal/treesitter/registry.go that skips registering "r" when
  // the no-CGO build is in use.
  //
  // Net effect: CGO_ENABLED=0 binaries support 21 tree-sitter languages (R and
  // Swift omitted); CGO_ENABLED=1 binaries support all 23. See
  // .planning/phases/51-packaging-goreleaser/deferred-items.md DEF-51-01 for the
  // full rationale and resolution history.
  func Language() unsafe.Pointer {
  	return nil
  }

File 2: internal/treesitter/bindings/swift/binding_nocgo.go

Same shape, with package tree_sitter_swift and the prose updated to say "Swift" instead of "R" and "tree-sitter-swift's C parser" instead of "tree-sitter-r's". Body is identical (return nil).

Style non-negotiables:
1. Build tag form is //go:build !cgo (Go 1.17+ canonical). Do NOT use `// +build !cgo`. Do NOT use `//go:build nocgo` (the project does not define a custom nocgo tag -- Go's built-in cgo tag with ! negation is the correct mechanism).
2. The package name MUST be tree_sitter_r (or tree_sitter_swift) to match the existing CGO file's package declaration. Different package names in the same directory are a compile error.
3. The function signature MUST be func Language() unsafe.Pointer -- same name, same return type. Returning nil from an unsafe.Pointer is allowed (the zero value is nil).
4. The import "unsafe" is required -- unsafe.Pointer is a built-in type but the package name must be in scope.
5. Do NOT import "C" here -- that pulls in cgo machinery and defeats the build tag.
6. After creation, gofmt -d on each file MUST produce no diff.
7. After creation, CGO_ENABLED=0 go vet ./internal/treesitter/bindings/r/... ./internal/treesitter/bindings/swift/... MUST exit 0.
  </action>
  <verify>
    <automated>test -f internal/treesitter/bindings/r/binding_nocgo.go && test -f internal/treesitter/bindings/swift/binding_nocgo.go && head -1 internal/treesitter/bindings/r/binding_nocgo.go | grep -q '^//go:build !cgo$' && head -1 internal/treesitter/bindings/swift/binding_nocgo.go | grep -q '^//go:build !cgo$' && grep -q '^package tree_sitter_r$' internal/treesitter/bindings/r/binding_nocgo.go && grep -q '^package tree_sitter_swift$' internal/treesitter/bindings/swift/binding_nocgo.go && grep -q 'return nil' internal/treesitter/bindings/r/binding_nocgo.go && grep -q 'return nil' internal/treesitter/bindings/swift/binding_nocgo.go && ! grep -q 'import "C"' internal/treesitter/bindings/r/binding_nocgo.go && ! grep -q 'import "C"' internal/treesitter/bindings/swift/binding_nocgo.go && gofmt -l internal/treesitter/bindings/r/binding_nocgo.go internal/treesitter/bindings/swift/binding_nocgo.go | wc -l | grep -qE '^[[:space:]]*0$' && CGO_ENABLED=0 go vet ./internal/treesitter/bindings/r/... ./internal/treesitter/bindings/swift/...</automated>
  </verify>
  <acceptance_criteria>
    - internal/treesitter/bindings/r/binding_nocgo.go exists with //go:build !cgo on line 1, blank line 2, package tree_sitter_r on line 3
    - internal/treesitter/bindings/swift/binding_nocgo.go exists with //go:build !cgo on line 1, blank line 2, package tree_sitter_swift on line 3
    - Each stub contains import "unsafe" (and ONLY unsafe; NOT import "C")
    - Each stub contains func Language() unsafe.Pointer with body return nil
    - Each stub references DEF-51-01 in the doc comment
    - gofmt -l on both new files produces NO output
    - CGO_ENABLED=0 go vet ./internal/treesitter/bindings/r/... ./internal/treesitter/bindings/swift/... exits 0
    - CGO_ENABLED=1 go vet ./internal/treesitter/bindings/r/... ./internal/treesitter/bindings/swift/... STILL exits 0
  </acceptance_criteria>
  <done>Both packages now have a CGO-on path (binding.go) and a CGO-off path (binding_nocgo.go). The no-CGO build resolves the import graph without compile errors. registry.go still calls Language() and gets nil under CGO_ENABLED=0 -- Task 3 adds the nil-check.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: Add nil-check guards in registry.go for r and swift language registration</name>
  <files>internal/treesitter/registry.go</files>
  <read_first>
    - internal/treesitter/registry.go full file (the relevant edit is at lines 85-86 inside the constructor; lines 1-50 contain the imports including tree_sitter_r_local and tree_sitter_swift_local)
    - internal/treesitter/bindings/r/binding_nocgo.go (just-created; confirms Language() returns nil under no-CGO)
    - internal/treesitter/bindings/swift/binding_nocgo.go (just-created; same)
  </read_first>
  <action>
Replace the unconditional r.languages[...] = tree_sitter.NewLanguage(...) calls for "r" and "swift" with nil-guarded versions so a no-CGO build registers 21 languages instead of panicking on tree_sitter.NewLanguage(nil).

Current lines 84-86 of internal/treesitter/registry.go (verbatim):

  	// Wave 2b gap closure: local vendored bindings
  	r.languages["r"] = tree_sitter.NewLanguage(tree_sitter_r_local.Language())
  	r.languages["swift"] = tree_sitter.NewLanguage(tree_sitter_swift_local.Language())

Replace with (preserving the existing TAB indentation -- the surrounding file uses one TAB for body indent):

  	// Wave 2b gap closure: local vendored bindings.
  	// CGO-only: under CGO_ENABLED=0, Language() returns nil from the binding_nocgo.go
  	// stubs and we skip registration so a release-build binary supports 21 languages
  	// instead of panicking. See .planning/phases/51-packaging-goreleaser/deferred-items.md
  	// DEF-51-01 (resolved by Plan 51-03).
  	if ptr := tree_sitter_r_local.Language(); ptr != nil {
  		r.languages["r"] = tree_sitter.NewLanguage(ptr)
  	}
  	if ptr := tree_sitter_swift_local.Language(); ptr != nil {
  		r.languages["swift"] = tree_sitter.NewLanguage(ptr)
  	}

Style non-negotiables:
1. The if ptr := ...; ptr != nil short-variable-declaration form is idiomatic Go. The local variable name ptr is short because the scope is one statement; verify it does not collide with anything else in the constructor (a quick grep on the file confirms `ptr` is not used elsewhere; if it is, rename to `langPtr`).
2. Indentation MUST be a single TAB for the if-block lines and two TABs for the inner assignment -- match the surrounding file exactly. Mixed tabs/spaces are a Go compile error.
3. The inline comment block MUST reference DEF-51-01 and Plan 51-03 so a future grep `grep -rn DEF-51-01 .` lands on this file.
4. Do NOT touch any other line in registry.go. The other 21 language registrations stay byte-identical.
5. After the edit, gofmt -d internal/treesitter/registry.go MUST produce no diff.

Why nil-check at the registry rather than make the stub return a sentinel "empty" Language:
- tree_sitter.NewLanguage(nil) is undefined behavior in the upstream go-tree-sitter API and would likely panic on first parse attempt.
- Skipping registration is the cleanest "this language is not available" signal -- GetLanguage("r") returns (nil, false), and existing call sites (RepoMap tag extractor, edit subsystem) already handle the not-registered case gracefully via the ok boolean.
- A future re-enable of CGO (or a pure-Go tree-sitter-r upstream) is a one-line revert of this guard.
  </action>
  <verify>
    <automated>grep -q 'if ptr := tree_sitter_r_local.Language(); ptr != nil' internal/treesitter/registry.go && grep -q 'if ptr := tree_sitter_swift_local.Language(); ptr != nil' internal/treesitter/registry.go && grep -q 'DEF-51-01 (resolved by Plan 51-03)' internal/treesitter/registry.go && ! grep -qE 'r\.languages\["r"\] = tree_sitter\.NewLanguage\(tree_sitter_r_local\.Language\(\)\)' internal/treesitter/registry.go && ! grep -qE 'r\.languages\["swift"\] = tree_sitter\.NewLanguage\(tree_sitter_swift_local\.Language\(\)\)' internal/treesitter/registry.go && gofmt -l internal/treesitter/registry.go | wc -l | grep -qE '^[[:space:]]*0$' && CGO_ENABLED=1 go vet ./internal/treesitter/... && CGO_ENABLED=0 go vet ./internal/treesitter/...</automated>
  </verify>
  <acceptance_criteria>
    - internal/treesitter/registry.go contains the literal substring `if ptr := tree_sitter_r_local.Language(); ptr != nil`
    - internal/treesitter/registry.go contains the literal substring `if ptr := tree_sitter_swift_local.Language(); ptr != nil`
    - internal/treesitter/registry.go contains the substring `DEF-51-01 (resolved by Plan 51-03)`
    - The OLD unconditional lines `r.languages["r"] = tree_sitter.NewLanguage(tree_sitter_r_local.Language())` and `r.languages["swift"] = tree_sitter.NewLanguage(tree_sitter_swift_local.Language())` are GONE (`grep -c` returns 0 for each)
    - `gofmt -l internal/treesitter/registry.go` produces NO output
    - `CGO_ENABLED=1 go vet ./internal/treesitter/...` exits 0
    - `CGO_ENABLED=0 go vet ./internal/treesitter/...` exits 0
    - The other 21 language registrations are byte-identical (`git diff internal/treesitter/registry.go` shows ONLY the lines around 84-86 changed)
  </acceptance_criteria>
  <done>The registry constructor nil-checks the R and Swift Language() return values and skips registration when nil. CGO-on builds register 23 languages; CGO-off builds register 21. No panic in either path.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 4: End-to-end smoke test -- CGO_ENABLED=0 build + make release-snapshot produces 6 archives</name>
  <files>(no file modifications -- pure verification task; updates deferred-items.md status)</files>
  <read_first>
    - .planning/phases/51-packaging-goreleaser/deferred-items.md (current state -- DEF-51-01 marked OPEN; this task flips it to RESOLVED)
    - .goreleaser.yaml (committed in 51-01; confirms CGO_ENABLED=0 in the env block)
    - Makefile (committed in 51-02; confirms `release-snapshot` target exists at line 50)
  </read_first>
  <action>
Run the end-to-end smoke test that DEF-51-01 documented as failing, and update the deferred-items.md status. This task does not modify any source code -- it executes the verification chain that Plan 51-01 / 51-02 could not run end-to-end and updates the planning record.

Step A: CGO_ENABLED=0 build smoke

Run:
  CGO_ENABLED=0 go build ./cmd/serena

Expected: exit 0, produces a `serena` binary at the repo root. If this fails, STOP and document the failure in deferred-items.md as DEF-51-02 (do NOT silently expand scope).

Step B: CGO_ENABLED=1 sanity (unchanged behavior)

Run (whole repo per success_criteria #4):
  CGO_ENABLED=1 go test ./... -count=1 -timeout 10m

Expected: exit 0, all tests pass (R and Swift parser tests included). The whole-repo scope matches the success criterion; the narrower `./internal/treesitter/...` is a useful subset to run first if iterating.

Step C: CGO_ENABLED=0 sanity

Run (whole repo per success_criteria #3):
  CGO_ENABLED=0 go test ./... -count=1 -timeout 10m

Expected: exit 0. Tests that depend on R/Swift specifically (anywhere in the repo, NOT just under internal/treesitter/) MUST either be CGO-tagged themselves OR check `GetLanguage("r")` for `(_, ok)` and skip when not registered. The whole-repo scope is intentional: a test outside internal/treesitter that calls `GetLanguage("swift")` and asserts ok=true would pass a narrow `./internal/treesitter/...` run but fail success_criteria #3. If a test fails here that was passing under CGO_ENABLED=1, that is a Task 4 surprise:
- If the test is CGO-only by nature (parses real R/Swift source and expects a non-nil Language), add `//go:build cgo` to its test file. Document the addition in the SUMMARY.
- If the test should be CGO-agnostic, fix it to use the (lang, ok) idiom. Document the fix in the SUMMARY.
Do NOT skip this step -- a test that fails CGO_ENABLED=0 is a real regression.

Step D: make release-snapshot end-to-end (requires goreleaser locally)

If `command -v goreleaser` returns 0 (the executor's machine has goreleaser installed):
  make release-snapshot

Expected:
- exit 0
- ls dist/*.tar.gz | wc -l returns 6 (the cartesian product of 3 OS x 2 ARCH)
- ls dist/checksums.txt exists
- The archive names match the pattern `serena_<version>_<os>_<arch>.tar.gz`

If goreleaser is NOT installed locally, document this in the SUMMARY as "deferred to first CI tag push" and proceed. The CI workflow committed in 51-01 will exercise this end-to-end on the next v* tag.

Step E: Update deferred-items.md

Edit `.planning/phases/51-packaging-goreleaser/deferred-items.md` to mark DEF-51-01 as RESOLVED. Append a new "Resolution" subsection at the end of the DEF-51-01 entry (do NOT delete the existing OPEN narrative -- preserve the history, just update the **Status** marker and add the resolution record):

Change the line:
  **Status:** OPEN -- deferred to a separate decision/phase per Plan 51-01 scope boundary.

To:
  **Status:** RESOLVED -- Resolved by Plan 51-03 (build tags + non-CGO stubs).

Then append at the end of the DEF-51-01 entry (before any subsequent items):

  ## Resolution
  
  Resolved by Plan 51-03 on <DATE>. The fix:
  - Added //go:build cgo tag to internal/treesitter/bindings/{r,swift}/binding.go.
  - Added internal/treesitter/bindings/{r,swift}/binding_nocgo.go stubs (//go:build !cgo) returning nil from Language().
  - Updated internal/treesitter/registry.go to nil-check Language() before NewLanguage; skips registration for r and swift under CGO_ENABLED=0.
  
  Smoke test result: <PASTE OUTPUT OF Step D, or "deferred to CI" if goreleaser not installed locally>.
  
  Net effect: CGO_ENABLED=0 builds support 21 tree-sitter languages (R and Swift omitted); CGO_ENABLED=1 builds support all 23. The goreleaser pipeline now produces 6 binaries on a real v* tag push.

Replace `<DATE>` with today's date (YYYY-MM-DD format) and `<PASTE OUTPUT...>` with either the actual command output or "deferred to CI" depending on Step D.

Style non-negotiables:
1. Do NOT delete the existing OPEN narrative -- it is the historical record of what was discovered.
2. The Status line is the only line that flips OPEN -> RESOLVED; the rest of the file is preserved.
3. The Resolution subsection uses ## (H2) so it is visible at the same level as the discovery sections.
  </action>
  <verify>
    <automated>CGO_ENABLED=0 go build -o /tmp/serena-nocgo-smoke ./cmd/serena && rm -f /tmp/serena-nocgo-smoke serena && CGO_ENABLED=0 go test ./... -count=1 -timeout 10m && CGO_ENABLED=1 go test ./... -count=1 -timeout 10m && grep -q '\*\*Status:\*\* RESOLVED' .planning/phases/51-packaging-goreleaser/deferred-items.md && grep -q 'Resolved by Plan 51-03' .planning/phases/51-packaging-goreleaser/deferred-items.md</automated>
  </verify>
  <acceptance_criteria>
    - `CGO_ENABLED=0 go build ./cmd/serena` exits 0 (the build matrix block is gone)
    - `CGO_ENABLED=1 go test ./internal/treesitter/...` exits 0 (CGO-on path unchanged)
    - `CGO_ENABLED=0 go test ./internal/treesitter/...` exits 0 (CGO-off path passes; any tests that needed R/Swift specifically were updated to skip-when-not-registered or build-tagged in this task)
    - `CGO_ENABLED=0 go test ./... -count=1 -timeout 10m` exits 0 (whole-repo CGO-off run; matches success_criteria #3)
    - `CGO_ENABLED=1 go test ./... -count=1 -timeout 10m` exits 0 (whole-repo CGO-on run; matches success_criteria #4)
    - If goreleaser is locally installed: `ls dist/*.tar.gz | wc -l` returns 6 after `make release-snapshot`
    - If goreleaser is locally installed: `ls dist/checksums.txt` exists
    - .planning/phases/51-packaging-goreleaser/deferred-items.md DEF-51-01 entry: Status line says `**Status:** RESOLVED -- Resolved by Plan 51-03`
    - .planning/phases/51-packaging-goreleaser/deferred-items.md contains a `## Resolution` subsection with the date, the three-bullet fix summary, and the smoke-test result line
    - The original OPEN narrative in DEF-51-01 is PRESERVED (not deleted) -- `grep -c "Symptom:" .planning/phases/51-packaging-goreleaser/deferred-items.md` still returns 1 (or whatever the original count was)
    - `git status` shows only the planned files modified: 4 source files + 1 registry.go + deferred-items.md (6 files total across all 4 tasks of this plan)
    - `go vet ./...` exits 0 under both CGO_ENABLED=0 and CGO_ENABLED=1
  </acceptance_criteria>
  <done>The CGO-optional build is proven end-to-end. CGO_ENABLED=0 produces a working binary. The deferred-items.md record is updated so a future maintainer reading it understands when and how DEF-51-01 was closed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Go toolchain build constraint evaluation -> compiled binary | Build tags decide which file is in or out of the compilation unit. Misplaced tags (below package line, wrong directive form) are silently ignored, producing surprising binaries. Mitigated by acceptance criteria that grep for the exact placement (line 1, blank line 2). |
| Goreleaser CGO_ENABLED=0 cross-compile -> import graph | The pipeline's per-target builds run in parallel. If any target fails, the whole release fails closed. Mitigated by Task 4's local CGO_ENABLED=0 build smoke. |
| Runtime feature surface -> end user | A no-CGO build silently lacks R/Swift parsers. Users targeting those languages on a release binary see "language not registered" rather than a parser failure. Mitigated by GetLanguage's existing (lang, ok) idiom, which is honored throughout the codebase (RepoMap, edit subsystem). |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-51-15 (T-stub-misregistration) | Spoofing / Tampering | binding_nocgo.go stubs | mitigate | Stub files use the SAME package name as the CGO version (tree_sitter_r / tree_sitter_swift). Different package names in the same directory would be a compile error caught by Task 2's `go vet` step. The function signature is identical so registry.go imports unchanged. |
| T-51-16 (T-build-tag-silent-ignored) | Tampering | //go:build directive placement | mitigate | Acceptance criteria verify the exact placement: line 1 = build tag, line 2 = blank, line 3 = package. A misplaced build tag below the package declaration is silently ignored by the Go toolchain (a known Go pitfall); the grep gates on `head -1` and `sed -n '2p'` make this caught at verify time. |
| T-51-17 (T-nil-deref-in-registry) | Denial of Service | registry.go calling NewLanguage on nil | mitigate | Task 3 adds explicit `if ptr := ...; ptr != nil` guards before tree_sitter.NewLanguage. Without this, a no-CGO build would panic at startup on the first NewLanguage(nil) call. The acceptance criterion `CGO_ENABLED=0 go test ./internal/treesitter/...` exercises the registry constructor end-to-end. |
| T-51-18 (T-cgo-on-regression) | Tampering (silent regression) | CGO_ENABLED=1 path | mitigate | Task 1's edits ONLY add a build-tag header line + doc comment. The function body is byte-identical. Acceptance criterion `CGO_ENABLED=1 go test ./internal/treesitter/...` proves the CGO path is unaffected. |
| T-51-19 (T-test-cgo-coupling) | Confusion | Tests that implicitly require R/Swift parsers | mitigate | Task 4 Step C runs `CGO_ENABLED=0 go test ./internal/treesitter/...`. Any test that fails there is either CGO-only by nature (must be tagged) or buggy (must be fixed to handle the 21-language case). Either way the failure is caught and addressed in this plan, not deferred. |
| T-51-20 (T-deferred-items-stale) | Confusion | deferred-items.md DEF-51-01 record | mitigate | Task 4 Step E flips the Status line and appends a Resolution subsection. A future maintainer grep'ing the planning tree for OPEN deferred items sees DEF-51-01 as RESOLVED with a pointer to this plan and a smoke-test result. |

**Severity assessment:** No HIGH-severity unmitigated threats. The plan does NOT introduce new attack surface; it removes a pre-existing build-time failure mode that was preventing the release pipeline from running.
</threat_model>

<verification>
Plan-level verification (after all 4 tasks complete):

1. Both binding.go files have //go:build cgo on line 1, blank line 2, package on line 3 (Task 1 acceptance).
2. Both binding_nocgo.go files exist with //go:build !cgo, return nil from Language() (Task 2 acceptance).
3. registry.go nil-checks the two CGO-only Language() calls with the documented if-statement form (Task 3 acceptance).
4. End-to-end CGO_ENABLED=0 build of cmd/serena succeeds (Task 4 Step A).
5. Both CGO_ENABLED=0 and CGO_ENABLED=1 test suites pass for the WHOLE REPO (`go test ./...`), not just `./internal/treesitter/...` (Task 4 Steps B+C broadened to match success_criteria #3 and #4).
6. If goreleaser is locally installed: `make release-snapshot` produces 6 archives + checksums.txt (Task 4 Step D).
7. deferred-items.md DEF-51-01 marked RESOLVED with date and resolution record (Task 4 Step E).
8. `go vet ./...` exits 0 under both CGO modes (entire repo, not just treesitter).
9. `git status` shows exactly 6 modified files (4 binding-related Go files + registry.go + deferred-items.md); no incidental edits.

End-to-end UAT (deferred to /gsd-verify-work, sampled at the phase gate): a real v* tag push to agenthands/helix triggers release.yml; the goreleaser build matrix produces 6 archives without the DEF-51-01 build-constraints error; the reproducibility gate runs against artifacts that actually exist. This UAT is now achievable for the first time post-51-03 -- it was structurally blocked before this plan.
</verification>

<success_criteria>
Plan 51-03 succeeds when ALL of the following are true:

1. `CGO_ENABLED=0 go build ./cmd/serena` exits 0 (DEF-51-01 root cause resolved).
2. `CGO_ENABLED=1 go build ./cmd/serena` continues to exit 0 (no regression on the CGO-on path).
3. `CGO_ENABLED=0 go test ./...` exits 0 (no test depends on R/Swift parsers being registered without a build tag).
4. `CGO_ENABLED=1 go test ./...` exits 0 (the CGO-on path with full 23-language coverage passes).
5. The 4 binding files (2 existing + 2 new) carry the correct build tags on line 1.
6. registry.go has the nil-guarded if-statement form for r and swift.
7. deferred-items.md DEF-51-01 marked RESOLVED with date and three-bullet fix record.
8. If goreleaser is locally installed: `make release-snapshot` produces 6 archives + checksums.txt; otherwise this verification is deferred to the first CI tag push.
9. Every threat in the STRIDE register has a mitigation tied to a specific file change in this plan.
10. `git diff` shows exactly the planned scope: 4 binding files + registry.go + deferred-items.md, nothing else.

Once this plan ships, VERIFICATION.md gap "Tagging a release ... uploads 6 platform/arch binaries" is structurally achievable. Plans 51-04 (signing ceremony), 51-05 (reproducibility doc), 51-06 (release.yml hardening) build on this foundation.
</success_criteria>

<output>
After completion, create `.planning/phases/51-packaging-goreleaser/51-03-SUMMARY.md` capturing:
- The 4 binding-related files modified/created with one-line description each.
- Result of `CGO_ENABLED=0 go build ./cmd/serena` and `CGO_ENABLED=1 go test ./...` (exit codes + brief output).
- Result of `make release-snapshot` if goreleaser was locally installed (archive count, checksums.txt presence) or "deferred to CI" otherwise.
- Confirmation that no test was deleted or skipped to make CGO_ENABLED=0 pass; if any test was build-tagged or fixed, document the rationale per file.
- DEF-51-01 status flip from OPEN to RESOLVED in deferred-items.md (cite the date and one-line resolution).
- Cross-reference: Phases 51-04 and 51-05 / 51-06 / 51-07 can now proceed because the build matrix unblocks SC-1; their UATs are achievable for the first time post-51-03.
- Any unexpected scope discovered during execution (e.g., a third package with CGO-only code that ALSO needs the same fix) -- record as DEF-51-02 in deferred-items.md if it surfaced; do NOT silently expand 51-03's scope.
</output>
