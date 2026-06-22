---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
plan: 03
subsystem: lint/static-analysis
tags: [ablation, vet, call-site-gate, narrow-ast, ABLATE-06, D-06]
requires:
  - "internal/semantic/integ.ChooseSource + SemanticLookup interface (Phase 65)"
  - "internal/lint/ablationleakage import-boundary Analyzer (Phase 76 ABLATE-08)"
provides:
  - "vet-ablation-leakage call-site gate check (D-06): flags direct ExpandFrom/RankFiles/ValidateCriticalEdges reads outside the gate allowlist not routed through integ.ChooseSource"
  - "badgate (RED) / goodgate (GREEN) analysistest fixtures + two test funcs"
  - "gate allowlist (Plan 04 production wiring must stay inside it)"
affects:
  - "internal/daemon (Plan 04 production read wiring must remain inside gateAllowedPkgPrefixes OR route through ChooseSource)"
  - "make vet (the existing vettool rebuild rule now enforces the call-site check)"
tech-stack:
  added: []
  patterns:
    - "Narrow-AST call-site check (RESEARCH Pitfall 6 / A3) — name-based selector-call match, NOT SSA"
    - "Import-gating collision guard: only inspect files importing internal/semantic/integ"
    - "Syntactic ChooseSource-in-file => reads gated (green path approximation)"
    - "_test-suffix-stripped allowlist match so external test packages are exempted like the package they test"
    - "Slash-boundary discipline preserved on the allowlist match (no bare HasPrefix over-flagging)"
key-files:
  created:
    - internal/lint/ablationleakage/testdata/src/github.com/agenthands/helix/internal/semantic/integ/integ.go
    - internal/lint/ablationleakage/testdata/src/github.com/agenthands/helix/badgate/imports.go
    - internal/lint/ablationleakage/testdata/src/github.com/agenthands/helix/goodgate/imports.go
  modified:
    - internal/lint/ablationleakage/analyzer.go
    - internal/lint/ablationleakage/analyzer_test.go
decisions:
  - "Flagged SemanticLookup read method set = {ExpandFrom, RankFiles, ValidateCriticalEdges} (the data-bearing reads named in ABLATE-06); lifecycle/status methods (Available, Status, SymbolID, LocateSymbol, Visibility, IsEntrypointReachable, RankFromSeeds) are NOT flagged"
  - "Gate allowlist (gateAllowedPkgPrefixes): internal/semantic, internal/skill/semantic, internal/daemon, internal/kernel/symbols, internal/kernel/health — Plan 04 wiring MUST stay inside this set or route through ChooseSource"
  - "Narrow-AST over SSA (A3): name-based match gated on the file importing internal/semantic/integ; SSA receiver-typed call-graph proof documented as deferred future precision (D-08 / Pitfall 6)"
  - "No Makefile change — the existing $(VETTOOL_ABLATION_LEAKAGE) rebuild rule (Makefile:60-61) rebuilds the vettool from source on every make vet"
metrics:
  duration: ~25m
  completed: 2026-06-20
---

# Phase 81 Plan 03: vet-ablation-leakage Call-Site Gate Check Summary

Extended the Phase 76 `vet-ablation-leakage` analyzer from an import-boundary check into a dual-check analyzer that ALSO enforces the D-06 call-site gate: a direct `SemanticLookup` read (`ExpandFrom` / `RankFiles` / `ValidateCriticalEdges`) made outside the gate allowlist and not routed through `integ.ChooseSource` is flagged at `make vet`. Shipped a deliberate green→red `badgate`/`goodgate` testdata pair proving the check fires (not a rubber-stamp). The import-boundary check (ABLATE-08) is preserved verbatim, and `make vet` stays green on the real tree.

## What Was Built

### Task 1 RED — green→red call-site fixtures (commit 55d17f72)
- `testdata/.../internal/semantic/integ/integ.go`: stub `integ` package mirroring just the surface the analyzer resolves — a trimmed `SemanticLookup` (RankFiles/ExpandFrom/ValidateCriticalEdges), `ConfigGate`, and `ChooseSource`.
- `testdata/.../badgate/imports.go`: RED fixture — a direct `lookup.ExpandFrom(...)` read with NO `ChooseSource` call in the file, carrying the matching `// want` directive.
- `testdata/.../goodgate/imports.go`: GREEN fixture — the same `.ExpandFrom(` read routed through `integ.ChooseSource` first; no expectation directive (analysistest fails if any diagnostic fires).
- `analyzer_test.go`: two analysistest funcs (`TestAnalyzer_RejectsUngatedSemanticRead`, `TestAnalyzer_AllowsSemanticReadBehindChooseSource`) mirroring the badrunner/goodrunner driver. RED confirmed: badgate not flagged (no call-site check yet).

### Task 1 GREEN — narrow-AST call-site check (commit b48ab11e)
- `analyzer.go`: added Check 2 to the Analyzer Run func. Walks call expressions; for a `SelectorExpr` call whose `Sel.Name` is in `semanticReadMethods`, reports `"semantic read X must route through integ.ChooseSource (ABLATE-06 gate bypass)"` UNLESS the package is allowlisted OR the file syntactically contains a `ChooseSource` call. The Phase 76 import-boundary check (Check 1) is preserved verbatim, including its slash-boundary discipline. Method set and allowlist extracted to package vars; no separate REFACTOR commit needed.

### Task 2 — real-tree false-positive resolution + make vet smoke (commit e783f56e)
The first `make vet` run flagged 5 real-tree sites — all false positives:
- 3× `internal/repomap` + `internal/skill/repomap`: `repomap.FileGraph.RankFiles(damping float64, ...)` — a PageRank method that collides by NAME with `SemanticLookup.RankFiles`; repomap does not even import `integ`.
- 1× `internal/skill/semantic_test`: an external test package of an allowlisted package, missed because go/analysis reports the path with a `_test` suffix.

Fixes (both narrow-AST, no SSA):
- **Import-gating collision guard:** only inspect a file that imports `internal/semantic/integ` (the package owning `SemanticLookup`). A file that never imports integ cannot be making a SemanticLookup read → the repomap collision is eliminated.
- **`_test`-suffix allowlist match:** strip a trailing `_test` from the package path before allowlist matching, so an external test of an allowlisted package is exempted exactly like the package it tests.

After the fix, `make vet` exits 0 on the real tree while `badgate` stays flagged. No Makefile change (the existing rebuild rule covers the extension).

## Key Outputs for Plan 04 (READ THIS BEFORE WIRING)

- **Flagged read method set:** `ExpandFrom`, `RankFiles`, `ValidateCriticalEdges`.
- **Gate allowlist (`gateAllowedPkgPrefixes`):** `internal/semantic`, `internal/skill/semantic`, `internal/daemon`, `internal/kernel/symbols`, `internal/kernel/health` (exact-OR-slash-boundary; `_test` siblings included).
- Plan 04's production read wiring MUST either live inside one of those package prefixes OR route the read through `integ.ChooseSource` in the same file — otherwise `make vet` will flag it. Any new read site outside these prefixes that legitimately gates elsewhere needs an allowlist widen documented as a deviation.
- **Collision guard caveat (narrow-AST limitation):** the check only inspects files importing `internal/semantic/integ`. A future read site that calls a `SemanticLookup` method via a type alias re-exported WITHOUT importing `integ` directly would slip the static check — the Plan 05 runtime read-counter is the complementary dynamic guard (T-81-03-03 disposition = accept; SSA is the documented upgrade).

## Verification

- `go test ./internal/lint/ablationleakage/ -count=1` — green (badgate flagged, goodgate silent, badrunner/goodrunner/siblingrunner import-boundary tests all still pass)
- `go build ./...` — succeeds
- `go vet ./...` — clean
- `make vet` — exits 0 on the real tree (all 5 vettools pass, including the rebuilt vet-ablation-leakage)
- `gofmt -l` on analyzer.go — clean
- `grep -c ChooseSource analyzer.go` ≥ 1; `grep -c want badgate/imports.go` ≥ 1 — satisfied
- git log: `test(81-03):` (55d17f72) precedes `feat(81-03):` (b48ab11e) — RED→GREEN gate intact

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Eliminated `repomap.RankFiles` name-collision false positive**
- **Found during:** Task 2 (`make vet` smoke)
- **Issue:** The pure name-based check flagged `repomap.FileGraph.RankFiles(0.85, nil)` (a PageRank method, unrelated to the semantic store) at 3 real-tree sites; repomap does not import `integ`.
- **Fix:** Gate the call-site check on the file importing `internal/semantic/integ` (`fileImportsInteg`). This is the documented narrow-AST collision guard (RESEARCH Pitfall 6) and is more precise than blanket-widening the allowlist.
- **Files modified:** internal/lint/ablationleakage/analyzer.go
- **Commit:** e783f56e

**2. [Rule 1 - Bug] Exempted external test packages of allowlisted packages**
- **Found during:** Task 2 (`make vet` smoke)
- **Issue:** `internal/skill/semantic`'s external test package (`internal/skill/semantic_test`) tripped the check because go/analysis reports the path with a `_test` suffix that the allowlist's slash-boundary match did not cover.
- **Fix:** `strings.TrimSuffix(pkgPath, "_test")` before allowlist matching, so an external test of an allowlisted package is exempted like the package it tests.
- **Files modified:** internal/lint/ablationleakage/analyzer.go
- **Commit:** e783f56e

The plan's Task 2 explicitly anticipated real-tree false positives ("if make vet flags a legitimate production read, widen the allowlist") — these two fixes are the precise (rather than blanket) realization of that instruction.

## Self-Check: PASSED
- internal/lint/ablationleakage/analyzer.go — FOUND (ChooseSource + semanticReadMethods + gateAllowedPkgPrefixes present)
- internal/lint/ablationleakage/analyzer_test.go — FOUND (two new test funcs present)
- internal/lint/ablationleakage/testdata/.../badgate/imports.go — FOUND (// want present)
- internal/lint/ablationleakage/testdata/.../goodgate/imports.go — FOUND (ChooseSource-routed read present)
- internal/lint/ablationleakage/testdata/.../internal/semantic/integ/integ.go — FOUND (stub ChooseSource present)
- Commit 55d17f72 (test) — FOUND
- Commit b48ab11e (feat) — FOUND
- Commit e783f56e (fix) — FOUND
