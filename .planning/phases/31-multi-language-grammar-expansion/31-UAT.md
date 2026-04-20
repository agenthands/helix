---
status: complete
phase: 31-multi-language-grammar-expansion
source: [31-01-SUMMARY.md, 31-02-SUMMARY.md, 31-03-SUMMARY.md, 31-04-SUMMARY.md]
started: 2026-04-20T08:00:00Z
updated: 2026-04-20T08:05:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Grammar Registry — 23 Languages
expected: GrammarRegistry.SupportedLanguages() returns 23 entries in sorted order including all Wave 1 (java, c, cpp, c_sharp, ruby, php, javascript, kotlin), Wave 2a (scala, bash, haskell, julia, ocaml), Wave 2b (lua, zig, hcl), and gap closure (r, swift) plus original 5 (go, python, typescript, tsx, rust)
result: pass
verified: `go test ./internal/treesitter/... -run TestSupportedLanguages -v` — PASS

### 2. Tag Extraction — All 23 Languages
expected: TagExtractor.Extract() produces definition tags for all 23 supported languages with correct @name and @definition captures
result: pass
verified: `go test ./internal/repomap/... -run TestExtract_ -v` — 29 subtests, all PASS (Go, Python, TypeScript, Rust, Java, C, C++, C#, Ruby, PHP, JavaScript, Kotlin, Scala, Bash, Haskell, Julia, OCaml, Lua, Zig, HCL, R, Swift)

### 3. Body Extraction — 20 Languages (3 Graceful Skips)
expected: BodyExtractor.ExtractBody() works for languages with standard body fields. Haskell, OCaml (equation-based) and R (name on parent node) gracefully skip. HCL has no body config (block-based).
result: pass
verified: `go test ./internal/kernel/edit/... -run TestExtractBody_ -v` — 20 PASS, 1 SKIP (R), Haskell/OCaml/HCL have no body config entries (expected)

### 4. File Extension Mapping
expected: LangFromExt correctly maps .r, .R -> "r", .swift -> "swift", plus all previously supported extensions
result: pass
verified: `go test ./internal/repomap/... -run TestLangFromExt -v` — all 15 subtests PASS including .r, .R, .swift

### 5. Binary Build
expected: `go build ./cmd/serena` exits 0 with no linker errors for any grammar binding. Only expected warning: TOKEN_COUNT macro redefined in Swift scanner.c
result: pass
verified: `go build ./cmd/serena` — exit 0

### 6. Full Test Suite Regression
expected: `go test ./...` passes all packages with no regressions from grammar expansion
result: pass
verified: `go test ./... -count=1` — all 28 packages PASS, 0 failures

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none]
