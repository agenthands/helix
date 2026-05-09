---
phase: 66
plan: "02"
subsystem: guardrails-seams
tags: [guardrails, semantic-lookup, config, profile, visibility]
dependency_graph:
  requires: []
  provides:
    - integ.SemanticLookup.Visibility
    - integ.SemanticLookup.IsEntrypointReachable
    - integ.Visibility closed enum + ParseVisibility
    - guardrails.Visibility type alias
    - GuardrailsConfig D-22 shape (Rules/Tools/G004/G005)
    - D-22 defaults in config/defaults.go
    - Profile.Guardrails YAML decoding
  affects:
    - internal/semantic/integ (interface extension)
    - internal/daemon/semantic_wiring (production adapter stubs)
    - internal/config/defaults (new D-22 keys)
    - internal/profile (Profile struct + 5 YAMLs)
tech_stack:
  added: []
  patterns:
    - UnsupportedSemanticLookup mixin for zero-cost interface evolution
    - Closed-enum typed visibility with ParseVisibility translator
    - type alias for cross-package re-export without wrapping
    - koanf nested-map decoding for per-rule/per-tool override maps
key_files:
  created:
    - internal/semantic/integ/visibility.go
    - internal/semantic/integ/visibility_test.go
    - internal/semantic/integ/lookup_stub.go
    - internal/guardrails/visibility.go
    - internal/guardrails/visibility_test.go
    - internal/semantic/config_test.go
  modified:
    - internal/semantic/integ/lookup.go
    - internal/semantic/integ/noop.go
    - internal/daemon/semantic_wiring.go
    - internal/kernel/health/tools_semantic_test.go
    - internal/kernel/symbols/blast_radius_strangler_test.go
    - internal/semantic/integ/source_select_test.go
    - internal/skill/repomap/strangler_test.go
    - internal/skill/semantic/integration_test.go
    - internal/semantic/config.go
    - internal/config/defaults.go
    - internal/profile/profile.go
    - internal/profile/profiles/ci-bot.yaml
    - internal/profile/profiles/claude-code.yaml
    - internal/profile/profiles/codex.yaml
    - internal/profile/profiles/ide-assistant.yaml
    - internal/profile/profiles/full.yaml
decisions:
  - "Production Visibility/IsEntrypointReachable return ErrUnsupported (Wave-3 seam placeholder): Wave-2 rule predicates call these and fall back to D-19 conservative-warn on ErrUnsupported; store methods wired in Wave 3"
  - "UnsupportedSemanticLookup mixin chosen over surgical per-fake edits: 4 test fakes embed it via struct embedding; 4 others are in-package files that needed direct method adds"
  - "ParseVisibility is case-sensitive by design (T-66-10): extractor strings are code-generated constants, not user input; mixed-case maps to VisUnknown"
  - "ParseVisibility exported as var not func in guardrails package: allows var reassignment in tests if needed; identical to integ.ParseVisibility at runtime"
metrics:
  duration: "~35 minutes"
  completed: "2026-05-09"
  tasks_completed: 2
  tasks_total: 2
  files_created: 6
  files_modified: 16
---

# Phase 66 Plan 02: Semantic Seams Extension (Visibility + Config D-22) Summary

SemanticLookup gains Visibility and IsEntrypointReachable methods; closed-enum Visibility ships with ParseVisibility and IsPublicLike; GuardrailsConfig extended with D-22 per-rule/per-tool maps and G004/G005 substructs; all five profiles carry guardrails enforcement defaults.

## What Was Built

### Task 1: SemanticLookup Extension + Visibility Closed Enum

**`internal/semantic/integ/visibility.go`** — 6-value closed enum `Visibility` string type with:
- Constants: `VisPublic`, `VisExported`, `VisProtected`, `VisPackage`, `VisPrivate`, `VisUnknown`
- `ParseVisibility(raw string) Visibility` — translates extractor strings to typed enum; "" or unknown → `VisUnknown` (T-66-10 mitigation)
- `(v Visibility).IsPublicLike() bool` — true for Public/Exported/Protected; false for Package/Private/Unknown

**`internal/semantic/integ/lookup.go`** — SemanticLookup interface extended with:
```go
Visibility(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID) (Visibility, error)
IsEntrypointReachable(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID) (bool, error)
```

**`internal/semantic/integ/lookup_stub.go`** — `UnsupportedSemanticLookup` zero-value struct implementing both new methods returning `(VisUnknown, ErrUnsupported)` and `(false, ErrUnsupported)` respectively.

**`internal/semantic/integ/noop.go`** — NoopLookup extended with both new methods (same ErrUnsupported semantics).

**`internal/daemon/semantic_wiring.go`** — Production adapter `integSemanticLookup` gains both methods as Wave-3 seam placeholders returning ErrUnsupported (store method wiring deferred to Wave 3).

**Test fakes updated** (5 files): `source_select_test.go`, `strangler_test.go` (repomap), `blast_radius_strangler_test.go`, `integration_test.go` (semantic), `tools_semantic_test.go` (health).

**`internal/guardrails/visibility.go`** — type alias `type Visibility = integ.Visibility` re-exports all 6 constants + ParseVisibility + per-language coverage table comment.

**Tests:**
- `internal/semantic/integ/visibility_test.go`: `TestParseVisibility` (10 rows), `TestIsPublicLike` (6 rows), `TestVisibility_RoundTrip`
- `internal/guardrails/visibility_test.go`: `TestVisibility_AliasParity`, `TestVisibility_ParseVisibilityParity`, `TestVisibility_IsPublicLike_ThroughAlias`

### Task 2: GuardrailsConfig D-22 Extension + Defaults + Profile YAMLs

**`internal/semantic/config.go`** — GuardrailsConfig extended with:
```go
ReceiptTTL  time.Duration          `koanf:"receipt_ttl"`
Rules       map[string]RuleConfig  `koanf:"rules"`
Tools       map[string]ToolConfig  `koanf:"tools"`
G004        G004Config             `koanf:"G-004"`
G005        G005Config             `koanf:"G-005"`
```
New types: `RuleConfig`, `ToolConfig`, `G004Config`, `G005Config`.

**`internal/config/defaults.go`** — D-22 defaults added:
- `receipt_ttl = "5m"`
- Per-rule: G-001=warn, G-002=enforce, G-003=enforce, G-004=warn, G-005=enforce
- Per-tool: rename_symbol/safe_delete_symbol/replace_symbol_body=enforce, fuzzy_edit/replace_in_file=warn
- G-004 thresholds: max_changed_lines=50, max_files=1, max_file_change_ratio=0.30, enable_ratio_trigger=true

**`internal/profile/profile.go`** — Profile struct extended with `Guardrails ProfileGuardrailsConfig` field; `ProfileGuardrailsConfig` type defined.

**Profile YAMLs:**
- `ci-bot.yaml`: `guardrails: { enforcement: enforce }` (D-11 OI-01)
- `claude-code.yaml`, `codex.yaml`, `ide-assistant.yaml`, `full.yaml`: `guardrails: { enforcement: warn }`

**`internal/semantic/config_test.go`** — D-22 YAML fixture decode test: asserts rules["G-002"].Enforcement=="enforce", tools["fuzzy_edit"].Enforcement=="warn", G004.MaxChangedLines==50, G005.ImportPatterns["go"] populated.

## Verification

```
go test ./internal/semantic/integ/ ./internal/guardrails/ ./internal/semantic/ ./internal/profile/ ./internal/config/ -count=1 -race
```
All packages: PASS

```
go build ./...  # exits 0 (only pre-existing swift binding warning)
go vet ./...    # exits 0 (same)
```

## Acceptance Criteria — Verified

| Criterion | Status |
|-----------|--------|
| `grep -E "Visibility\(ctx context.Context..."` matches | PASS |
| `grep -E "IsEntrypointReachable\(ctx context.Context..."` matches | PASS |
| All 6 VisXxx constants in visibility.go | PASS |
| `type Visibility = integ.Visibility` in guardrails/visibility.go | PASS |
| `go build ./...` exits 0 | PASS |
| TestParseVisibility passes | PASS |
| `go vet ./...` exits 0 | PASS |
| `grep -E "Rules\s+map\[string\]RuleConfig"` matches | PASS |
| `grep -E "G005\s+G005Config"` matches | PASS |
| `grep -E "type G004Config struct"` matches | PASS |
| `grep -E "type RuleConfig struct"` matches | PASS |
| `grep -E "guardrails.rules.G-002.enforcement"` in defaults.go | PASS |
| `grep -E "guardrails.tools.rename_symbol.enforcement"` in defaults.go | PASS |
| `grep -E "guardrails.G-004.max_changed_lines"` in defaults.go | PASS |
| ci-bot.yaml: `guardrails: enforcement: enforce` | PASS |
| claude-code.yaml: `guardrails: enforcement: warn` | PASS |
| codex.yaml: `guardrails: enforcement: warn` | PASS |
| ide-assistant.yaml: `guardrails: enforcement: warn` | PASS |
| full.yaml: `guardrails: enforcement: warn` | PASS |
| `grep -E "Guardrails\s+ProfileGuardrailsConfig"` in profile.go | PASS |
| `go test ./internal/semantic/ ./internal/profile/ -count=1 -race` exits 0 | PASS |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] UnsupportedSemanticLookup embedding requires direct fake updates for in-package test files**
- **Found during:** Task 1
- **Issue:** `source_select_test.go` (in `integ` package) can't embed `UnsupportedSemanticLookup` using `integ.UnsupportedSemanticLookup` (it's in the same package); similarly, 4 other test fakes in external packages already have full interface implementations
- **Fix:** Added the two new methods directly to each of the 5 test fakes; `UnsupportedSemanticLookup` remains available for future fakes in external packages
- **Files modified:** 5 test files
- **Commit:** 17734a1e

**2. [Rule 3 - Blocking] rawbytes koanf provider not in go.mod**
- **Found during:** Task 2, config_test.go creation
- **Issue:** `github.com/knadh/koanf/providers/rawbytes` is not in go.mod; only `confmap` and `file` providers are available
- **Fix:** Used `file.Provider` with `t.TempDir()` fixture file instead; semantically equivalent
- **Files modified:** internal/semantic/config_test.go
- **Commit:** 5e35af25

**3. [Rule 3 - Blocking] Production adapter returns ErrUnsupported (Wave-3 seam placeholder)**
- **Found during:** Task 1
- **Issue:** `Store.QuerySymbolVisibility` and `Store.QueryEntrypointReachability` don't exist yet (Wave-3 deliverable); production adapter in `semantic_wiring.go` cannot call non-existent store methods
- **Fix:** Production adapter returns `ErrUnsupported` with `// Wave-3 TODO` comments; Wave-2 rule predicates handle ErrUnsupported via D-19 conservative-warn fallback — correct behavior per plan intent
- **Files modified:** internal/daemon/semantic_wiring.go
- **Commit:** 17734a1e

## Known Stubs

| File | Nature | Future plan |
|------|--------|-------------|
| `internal/daemon/semantic_wiring.go` line ~1232 `Visibility()` | Returns `(VisUnknown, ErrUnsupported)` — store method not wired | Wave-3 (Phase 66 plans after 02) |
| `internal/daemon/semantic_wiring.go` line ~1256 `IsEntrypointReachable()` | Returns `(false, ErrUnsupported)` — entry-point graph not wired | Wave-3 (Phase 66 plans after 02) |

These stubs are intentional per the plan: "Plans 03/04/05 must be able to call `lookup.Visibility(...)` ... without re-shaping anything mid-implementation." The interface seam is established; Wave-2 predicates fall through to D-19 conservative-warn on ErrUnsupported.

## Threat Flags

No new network endpoints, auth paths, or schema changes introduced. Config/profile changes are operator-controlled (T-66-08 accepted). T-66-10 mitigation confirmed: `ParseVisibility("") → VisUnknown; VisUnknown.IsPublicLike() == false`.

## Self-Check: PASSED

All created files verified to exist. Both task commits verified in git log.

| Check | Result |
|-------|--------|
| `internal/semantic/integ/visibility.go` exists | FOUND |
| `internal/semantic/integ/lookup_stub.go` exists | FOUND |
| `internal/guardrails/visibility.go` exists | FOUND |
| `internal/semantic/config_test.go` exists | FOUND |
| Commit `17734a1e` (Task 1) exists | FOUND |
| Commit `5e35af25` (Task 2) exists | FOUND |
