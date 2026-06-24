# Phase 106: Exploratory DSPy Offline Tuning Harness (spike) - Pattern Map

**Mapped:** 2026-06-24
**Files analyzed:** 13 new + 2 modified
**Analogs found:** 6 with strong Go analogs / 13 (the 6 Python files have no in-repo analog by design — they mirror Go truth or are net-new DSPy)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/lint/toolsquarantine/analyzer.go` | linter (go/analysis) | transform (AST import inspection) | `internal/lint/ablationleakage/analyzer.go` | exact (import-boundary leg only) |
| `internal/lint/toolsquarantine/analyzer_test.go` | test (analysistest) | request-response | `internal/lint/ablationleakage/analyzer_test.go` | exact |
| `internal/lint/toolsquarantine/testdata/src/.../leakyruntime/imports.go` | test fixture (RED) | — | `.../bench/runners/badrunner/imports.go` | exact |
| `internal/lint/toolsquarantine/testdata/src/.../goodruntime/imports.go` | test fixture (GREEN) | — | `.../bench/runners/goodrunner/imports.go` | exact |
| `internal/lint/toolsquarantine/testdata/src/.../lookalike/imports.go` | test fixture (lookalike) | — | `.../bench/runners/siblingrunner/imports.go` | exact |
| `cmd/vet-tools-quarantine/main.go` | cmd (singlechecker) | — | `cmd/vet-noduckdb/main.go` / `cmd/vet-ablation-leakage/main.go` | exact (verbatim copy, swap analyzer) |
| `test/oracle/adopt/parity_test.go` | test (Go) | file-I/O (reads golden JSON) | `test/oracle/adopt/scorecard_test.go` | exact (same package, sibling file) |
| `tools/dspy-tune/scorer.py` | service (Python re-impl) | transform (string classify) | `test/oracle/adopt/scorecard.go` (`FirstCommand`/`ClassifyChoice`) | mirror (cross-language — port exactly) |
| `tools/dspy-tune/test_parity.py` | test (pytest) | file-I/O | `test/oracle/adopt/scorecard_test.go` (`TestFirstCommandNotSubstring`) | mirror |
| `tools/dspy-tune/test_split.py` | test (pytest) | — | (no analog — overfit guard) | none |
| `tools/dspy-tune/test_degenerate.py` | test (pytest) | — | (no analog — metric-gaming guard) | none |
| `tools/dspy-tune/optimize.py` | service (DSPy harness) | event-driven (optimizer loop) | (no Go analog — net-new DSPy) | none (use RESEARCH §Pattern 1) |
| `tools/dspy-tune/golden/parity_cases.json` | fixture (shared corpus) | — | `test/oracle/adopt/testdata/transcripts/*.json` | role-match |
| `tools/dspy-tune/requirements.txt` | config | — | (no analog — net-new) | none |
| Makefile (MODIFIED) | config | — | existing `vet:` target + VETTOOL vars (lines 20-72) | exact |
| `.gitignore` (MODIFIED) | config | — | existing root `.gitignore` Python patterns | exact |

## Pattern Assignments

### `internal/lint/toolsquarantine/analyzer.go` (linter, AST import inspection)

**Analog:** `internal/lint/ablationleakage/analyzer.go` — use ONLY the import-boundary leg (Check 1). Drop the entire ABLATE-06 call-site/`ChooseSource`/`semanticReadMethods` machinery (lines 47-147, 180-212); the quarantine analyzer needs only the import edge.

**Package-doc + imports pattern** (`ablationleakage/analyzer.go:27-45`):
```go
package toolsquarantine // (mirror ablationleakage's doc block: state forbidden edge, slash-boundary discipline, "testdata-only violation keeps real tree green")

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

const toolsPrefix = "github.com/agenthands/helix/tools"
```
Note: the quarantine check needs NO `go/ast` import unless you keep the (deferred/optional) shell-out scan — the import edge is read straight off `file.Imports`.

**Self-import exemption + import-inspection `run` func** — model exactly on the slash-boundary discipline at `ablationleakage/analyzer.go:165-178`. The quarantine inversion: ablationleakage checks `HasPrefix(pass.Pkg.Path(), checkedPkgPrefix)` to *opt a narrow namespace IN*; the quarantine analyzer opts the `tools/` namespace OUT (it may self-import) and checks everything else:
```go
var Analyzer = &analysis.Analyzer{
	Name: "toolsquarantine",
	Doc:  "fails if a runtime/cmd package imports the dev-time tools/ tree",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if strings.HasPrefix(pass.Pkg.Path(), toolsPrefix) {
			return nil, nil // tools/ may self-import
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == toolsPrefix || strings.HasPrefix(path, toolsPrefix+"/") {
					pass.Reportf(imp.Pos(),
						"runtime package %s must not import dev-time %s (got %s)",
						pass.Pkg.Path(), toolsPrefix, path)
				}
			}
		}
		return nil, nil
	},
}
```

**Slash-boundary guard (load-bearing, copy verbatim)** — the `path == forbidden || strings.HasPrefix(path, forbidden+"/")` form at `ablationleakage/analyzer.go:170`. A bare `HasPrefix(path, toolsPrefix)` would over-flag a lookalike sibling like `internal/toolsupport`. This is the exact false-positive guard tested by `TestAnalyzer_AllowsSlashBoundaryLookalike`.

**False-positive trap to AVOID (RESEARCH Pitfall 2):** do NOT add a blanket `exec.Command("pip"...)` ban — `internal/langregistry/installer.go:119,129` legitimately shells `pip`/`pipx` to install language servers. Keep the analyzer to the import-boundary leg only (recommended), or scope any shell-out scan to DSPy/optimizer literals.

---

### `internal/lint/toolsquarantine/analyzer_test.go` (test, analysistest)

**Analog:** `internal/lint/ablationleakage/analyzer_test.go` (whole file). Three test functions mirror the three fixtures: RED (rejects), GREEN (allows), lookalike (allows).

**Harness pattern** (`ablationleakage/analyzer_test.go:1-35`):
```go
package toolsquarantine_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/agenthands/helix/internal/lint/toolsquarantine"
)

func TestAnalyzer_RejectsRuntimeImportingTools(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), toolsquarantine.Analyzer,
		"github.com/agenthands/helix/internal/leakyruntime")
}

func TestAnalyzer_AllowsCleanRuntime(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), toolsquarantine.Analyzer,
		"github.com/agenthands/helix/internal/goodruntime")
}

func TestAnalyzer_AllowsSlashBoundaryLookalike(t *testing.T) { // internal/toolsupport-style lookalike
	analysistest.Run(t, analysistest.TestData(), toolsquarantine.Analyzer,
		"github.com/agenthands/helix/internal/toolsupport")
}
```
Test-name convention to match the RESEARCH→Test map: `RejectsRuntimeImportingTools` and `Allows*` (the per-wave gate greps `-run RejectsRuntimeImportingTools` and `-run Allows`).

---

### Leakage testdata fixtures (RED / GREEN / lookalike)

**Analogs (copy structure + the `// want` discipline verbatim):**

**RED** — `.../bench/runners/badrunner/imports.go`:
```go
// Deliberate violation lives ONLY under testdata/ (the go tool ignores it), so
// `make vet` on the real tree stays green.
package leakyruntime

import _ "github.com/agenthands/helix/tools/dspytune" // want `runtime package .* must not import .*`
```
Place this under a runtime-rooted path (e.g. `testdata/src/github.com/agenthands/helix/internal/leakyruntime/imports.go`) so `pass.Pkg.Path()` does NOT start with `tools` and the analyzer inspects it. You must also create a stub `testdata/src/.../tools/dspytune/dspytune.go` package for the import to resolve (analysistest builds the fixture GOPATH).

**GREEN** — `.../bench/runners/goodrunner/imports.go` (no `// want` directive → analysistest fails on any spurious diagnostic):
```go
package goodruntime

import _ "os/exec"
```

**Lookalike** — `.../bench/runners/siblingrunner/imports.go` (no `// want`; proves slash boundary):
```go
package toolsupport

import _ "github.com/agenthands/helix/internal/leakyruntime" // a bare-prefix lookalike of `tools` — MUST stay silent
```
Pick a lookalike whose path shares the bare `tools` prefix without the slash boundary (e.g. package `toolsupport`, or an import path containing the literal substring `tools` but not `tools/`).

---

### `cmd/vet-tools-quarantine/main.go` (cmd, singlechecker)

**Analog:** `cmd/vet-noduckdb/main.go` / `cmd/vet-ablation-leakage/main.go` — copy verbatim, swap the analyzer import:
```go
// Command vet-tools-quarantine is a singlechecker binary wrapping the
// toolsquarantine Analyzer. It is wired into `make vet` via
// `go vet -vettool=...` to enforce the runtime → dev-time tools/ import
// boundary on every test run.
package main

import (
	"github.com/agenthands/helix/internal/lint/toolsquarantine"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(toolsquarantine.Analyzer) }
```

---

### `test/oracle/adopt/parity_test.go` (test, Go — pins golden corpus to Go truth)

**Analog:** `test/oracle/adopt/scorecard_test.go` (same package `adopt`, sibling file). Reuse its `encoding/json` + `os.ReadFile` + `testify/require` idiom and its anti-vacuity ethos.

**Imports + load pattern** (mirror `scorecard_test.go:1-12, 25-44`):
```go
package adopt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)
```

**Corpus assertion** — for each case assert BOTH `ClassifyChoice` AND `FirstCommand` (the latter pins the substring-trap discriminator). Mirror the `require.GreaterOrEqual(len, MinTasks/8)` floor from `loadFixtureBucket` (`scorecard_test.go:30-32`) so a future corpus shrink turns RED:
```go
func TestPythonGoParityCorpus(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "tools", "dspy-tune", "golden", "parity_cases.json"))
	require.NoError(t, err)
	var cases []struct {
		Response     string `json:"response"`
		FirstCommand string `json:"first_command"`
		Chose        bool   `json:"chose"`
		FellBack     bool   `json:"fell_back"`
	}
	require.NoError(t, json.Unmarshal(data, &cases))
	require.GreaterOrEqual(t, len(cases), 8, "corpus must cover every classifier branch")
	for _, c := range cases {
		require.Equalf(t, c.FirstCommand, FirstCommand(c.Response), "FirstCommand mismatch on %q", c.Response)
		chose, fell := ClassifyChoice(c.Response)
		require.Equalf(t, c.Chose, chose, "choice mismatch on %q", c.Response)
		require.Equalf(t, c.FellBack, fell, "fallback mismatch on %q", c.Response)
	}
}
```
NOTE (RESEARCH Open Q3): confirm the relative-path depth `../../../tools/...` at plan time; the canonical corpus must be ONE committed file read by both sides — never duplicated.

**Anti-vacuity discipline to mirror** — the substring trap from `scorecard_test.go:46-62` (`TestFirstCommandNotSubstring`): a prose line mentioning "helix" then a grep must classify `(false,false)` then `(false,true)`. Ensure the golden corpus contains that exact discriminator case so the parity test is non-vacuous.

---

### `tools/dspy-tune/scorer.py` (service, Python re-impl — EXACT mirror of Go)

**Analog (port target):** `test/oracle/adopt/scorecard.go` lines 59-95. Port `FirstCommand` and `ClassifyChoice` line-for-line. Load-bearing details from the Go source:

- `FirstCommand` (`scorecard.go:59-72`): split on `"\n"`; per line `TrimSpace`; skip empty OR `HasPrefix("```")`; then `Trim(line, "\`")` → `TrimSpace` → `TrimPrefix("$ ")` → `TrimPrefix("> ")`; return `TrimSpace`. Go's `strings.Trim(line, "\`")` strips backticks from BOTH ends (Python `line.strip("\`")` matches). `TrimPrefix` is applied to `$ ` AND `> ` sequentially (Go applies both `TrimPrefix` calls; only one fires per line in practice).
- `fallbackPrefixes` (`scorecard.go:77`): `{"grep ", "sed ", "cat ", "find ", "rg ", "ls "}` — trailing space load-bearing (`"lsp"` is NOT a fallback).
- `ClassifyChoice` (`scorecard.go:85-95`): `cmd := ToLower(FirstCommand(...))`; `chose = HasPrefix(cmd, "helix ")`; `fellBack = any prefix in fallbackPrefixes`. Lowercase ONCE on the extracted command, NOT the whole response.

The RESEARCH §Code Examples Python block (lines 346-366) is the correct port; use it verbatim. Do NOT use `"helix" in response` (the substring trap — RESEARCH Pitfall 1 / Anti-Pattern).

---

### `tools/dspy-tune/test_parity.py` + `test_split.py` + `test_degenerate.py` (pytest)

**Analog for `test_parity.py`:** mirror `scorecard_test.go`'s anti-vacuity tests. Two required tests:
1. Parity over the same `golden/parity_cases.json` — assert `scorer.classify_choice` / `scorer.first_command` match each case's expected fields.
2. `test_broken_classifier_diverges` (the planted-divergence guard, RESEARCH §Parity Contract Anti-vacuity): a `_broken_classify` using `"helix" in response` substring MUST produce a DIFFERENT result on the substring-trap case — proving the corpus discriminates a real classifier from a broken one. This mirrors `TestFirstCommandNotSubstring` (`scorecard_test.go:46-62`).

`test_split.py` (overfit guard) and `test_degenerate.py` (metric-gaming guard) have NO in-repo analog — see "No Analog Found". They follow the RESEARCH §Corpus Split design: `test_split.py` asserts TEST ⊄ trainset∪valset; `test_degenerate.py` flags always-`helix` steering text.

---

### `tools/dspy-tune/optimize.py` (service, DSPy harness)

**No Go analog (by design).** Use RESEARCH §Pattern 1 (lines 250-274) verbatim as the starting skeleton: `dspy.configure(lm=...)` from `OPENAI_API_KEY` (dev env ONLY), `adopt_metric` wrapping `scorer.score_choice_rate`, `GEPA(metric=..., auto="light", track_stats=True, reflection_lm=...)`, `optimizer.compile(student, trainset, valset)` (TEST excluded), `optimized.save("output/optimized.json")` (git-ignored). Re-confirm the GEPA API against Context7 `/websites/dspy_ai` at plan time (DSPy fast-moving; valid-until 2026-07-01).

---

### `tools/dspy-tune/golden/parity_cases.json` (shared fixture)

**Analog (role):** `test/oracle/adopt/testdata/transcripts/*.json` (each has a `response` field). The corpus adds `first_command`, `chose`, `fell_back` expected fields. Per RESEARCH §Parity Contract, MUST include ≥8 branch-covering cases: a helix-choice, one per the 6 fallback prefixes, an unclassified-prose case, a fence-wrapped case, a `$ `/`> ` prompt case, the substring trap, and the `lsp`/`ls `-lookalike (`"lsp"` must classify `(false,false)`, NOT a fallback).

---

## Shared Patterns

### Slash-boundary import matching (anti-false-positive)
**Source:** `internal/lint/ablationleakage/analyzer.go:170` (and `pkgInAllowlist` at :101-109)
**Apply to:** `toolsquarantine/analyzer.go` import check
```go
if path == forbidden || strings.HasPrefix(path, forbidden+"/") { ... }
```
A bare `HasPrefix(path, forbidden)` over-flags lookalike siblings. Every existing vet-* analyzer (noduckdb, nokernel2semantic, ablationleakage) uses this exact form; the new analyzer must not regress it. Tested by a no-`// want` lookalike fixture.

### Anti-vacuity / planted-break discipline (the named CR-01 class)
**Source:** `test/oracle/adopt/scorecard_test.go:46-88` (`TestFirstCommandNotSubstring`, `TestSabotagedSkillRevertAndFail`, `TestEmptyBucketRejected`) + the `badrunner` RED fixture (`// want` directive)
**Apply to:** EVERY gate this phase adds — the leakage analyzer (planted leak → RED fixture trips it), the parity check (planted broken classifier → diverges), the overfit guard (held-out TEST that the optimizer provably never sees), the degenerate-steering inspection (always-`helix` text → flagged). A green-only gate is presumed broken.

### singlechecker cmd wrapper
**Source:** `cmd/vet-noduckdb/main.go`, `cmd/vet-ablation-leakage/main.go` (4-line bodies)
**Apply to:** `cmd/vet-tools-quarantine/main.go` — `func main() { singlechecker.Main(<analyzer>.Analyzer) }`

### Makefile vet: wiring (3 edit sites)
**Source:** Makefile lines 20-25 (VETTOOL var defs), 47-54 (`vet:` deps + `go vet -vettool=` lines), 56-72 (`$(VETTOOL_*):` install rules)
**Apply to:** Makefile — add three things mirroring the `ABLATION_LEAKAGE` entry exactly:
1. Var (after line 25): `VETTOOL_TOOLS_QUARANTINE=$(shell go env GOPATH)/bin/vet-tools-quarantine`
2. Add `$(VETTOOL_TOOLS_QUARANTINE)` to the `vet:` prereq list (line 47) and a `$(GO) vet -vettool=$(VETTOOL_TOOLS_QUARANTINE) ./...` recipe line (after line 54)
3. Install rule: `$(VETTOOL_TOOLS_QUARANTINE): cmd/vet-tools-quarantine/main.go internal/lint/toolsquarantine/*.go` → `$(GO) install ./cmd/vet-tools-quarantine`

### Artifact re-entry gate (existing, untouched)
**Source:** Makefile line 90 `verify-reference` (`go run ./cmd/helix-refgen --check`); `internal/cli/skill.go:21` `//go:embed`; `internal/cli/skill_test.go:121` `cap1536=1536` / `TestSkillIdleCostBound`
**Apply to:** any adopted optimizer output — it re-enters ONLY via a human SKILL.md edit (≤1536, `## Decision matrix` anchor preserved) or a refgen override; `optimized.json` is git-ignored and never pasted in. No new code here; this phase preserves these gates.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `tools/dspy-tune/optimize.py` | service | event-driven | No DSPy/optimizer code exists in-repo; net-new. Use RESEARCH §Pattern 1. |
| `tools/dspy-tune/test_split.py` | test (pytest) | — | Overfit/held-out-split guard; no in-repo pattern. Use RESEARCH §Corpus Split. |
| `tools/dspy-tune/test_degenerate.py` | test (pytest) | — | Metric-gaming/degenerate-steering inspection; no in-repo pattern. Use RESEARCH Pitfall 4. |
| `tools/dspy-tune/requirements.txt` | config | — | First Python tree in the repo; pin `dspy==3.1.3`, `pytest`. |
| `tools/dspy-tune/README.md` | doc | — | Net-new; document "no-ship is OK" + dev-env-only `OPENAI_API_KEY`. |

`.gitignore` (MODIFIED): root already covers `.venv/`, `__pycache__/`, `*.py[cod]`, `.pytest_cache/` (RESEARCH verified). Add an explicit ignore for `tools/dspy-tune/output/` (DSPy `program.save()` JSON must not be committed raw).

## Metadata

**Analog search scope:** `internal/lint/ablationleakage/`, `cmd/vet-*/`, `test/oracle/adopt/`, Makefile `vet:` target, `.gitignore`
**Files scanned (first-hand):** `scorecard.go`, `scorecard_test.go`, `ablationleakage/analyzer.go`, `ablationleakage/analyzer_test.go`, 3 ablationleakage testdata fixtures, `cmd/vet-noduckdb/main.go`, `cmd/vet-ablation-leakage/main.go`, Makefile lines 20-95
**Pattern extraction date:** 2026-06-24
