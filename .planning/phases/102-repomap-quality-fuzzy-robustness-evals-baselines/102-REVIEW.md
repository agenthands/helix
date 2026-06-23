---
phase: 102-repomap-quality-fuzzy-robustness-evals-baselines
reviewed: 2026-06-23T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - bench/evaluators/repomapeval/repomapeval.go
  - bench/evaluators/repomapeval/rankers.go
  - bench/evaluators/repomapeval/corpus.go
  - bench/evaluators/fuzzyrobust/fuzzyrobust.go
  - bench/evaluators/fuzzyrobust/perturb.go
  - bench/evaluators/fuzzyrobust/corpus.go
  - bench/runtime/repomap_eval_capture_regen.go
  - bench/runtime/fuzzy_robust_capture_regen.go
  - bench/aggregator/repomap_eval_baseline.go
  - bench/aggregator/fuzzy_robust_baseline.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 102: Code Review Report

**Reviewed:** 2026-06-23
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Reviewed the two stdlib-only leaf evaluators (`repomapeval`, `fuzzyrobust`), their
two `//go:build ignore` HELIX_BIN-gated capture regenerators, and the two
deterministic aggregator baseline renderers, at standard depth with focus on
metric correctness, discriminator soundness, fuzzy refusal handling, and
determinism.

The **shipped package code is solid**. The four IR metrics (`RecallAtK`, `MRR`,
`NDCGAtK`, `BudgetFitRatio`) are total and division-safe on empty/nil/negative-k
inputs; the nDCG gain/discount math (`1/log2(i+2)`, IDCG over `min(len(gold),k)`,
`nDCG=0` when `IDCG==0`) is correct and matches the hand-computed test vectors.
The discriminator uses seeded PCG (`rand.NewPCG`) for byte-identical shuffles, the
margin gate plus the strict `rev<fwd`/`rnd<fwd` bite assertions genuinely cut, and
the fuzzy `ErrAmbiguous→ambiguous_match` vs `ErrNoMatch→no_match` distinction is
preserved end-to-end (corpus, regen mapping, `ScoreCase`, refusal gate). All
aggregator map ranges sort before emit (`strategyRows`, `metricRows`, `langs`),
both renderers fail-closed on the load-bearing metric, and both leaf loaders
validate the language path segment before `filepath.Join` and fail-closed on an
empty corpus. `go test ./bench/...` is green and `go vet` is clean.

The findings below are confined to a **contract/documentation gap between the
`make bench-*` targets + regenerators and the committed baseline reports**, and a
**heuristic fragility in the build-excluded tree parser**. No correctness,
security, or determinism defect was found in the shipped (compiled) code.

## Warnings

### WR-01: `make bench-*` cannot regenerate the committed baseline reports its goldens assert against

**File:** `bench/runtime/repomap_eval_capture_regen.go:64-131`, `bench/runtime/fuzzy_robust_capture_regen.go:90-186`, `Makefile:174-194`

**Issue:** The aggregator goldens
(`bench/aggregator/repomap_eval_baseline_test.go`,
`bench/aggregator/fuzzy_robust_baseline_test.go`) treat
`bench/reports/{repomap-eval,fuzzy-robust}-baseline/result.v2.json` and
`BENCH-RESULTS.md` as the committed CI contract and byte-match against them. The
`Makefile` targets claim the regenerators produce exactly those bytes:

> "The committed bytes under the force-tracked `bench/reports/repomap-eval-baseline/` dir are the CI contract; re-running this regenerates them byte-identically." (`Makefile:180-181`, mirrored at `:190-192`)

But neither regenerator writes anything under `bench/reports/`. `repomap_eval_capture_regen.go` writes only `testdata/captured/<lang>.json` (`:115-116`); `fuzzy_robust_capture_regen.go` writes only `testdata/captured/<lang>.json` (`:172-173`). A repo-wide grep confirms `result.v2.json` / `BENCH-RESULTS.md` under `bench/reports/...` has **no in-tree writer**. Consequently `make bench-repomap-eval` / `make bench-fuzzy-robust` regenerate the captured corpus but leave the committed baseline reports — the artifacts the aggregator tests actually gate — stale/hand-authored. If a metric or render-format change lands, the documented "re-run to regenerate byte-identically" workflow silently does not refresh the reports, so a contributor must hand-edit them and the byte-reproducibility claim is only enforced for the render step, never for the upstream result.v2.json.

**Fix:** Either (a) extend each regenerator to also compute and write the
`bench/reports/.../result.v2.json` (and re-render `BENCH-RESULTS.md` via the
aggregator renderer) so the Makefile claim holds, or (b) correct the Makefile
comments and the regenerator package docs to state explicitly that only
`testdata/captured/*.json` is regenerated and the `bench/reports/...` baseline is
authored/refreshed by a separate, named step — and point to that step. Today the
prose and the code disagree.

### WR-02: `parseTreeToIDs` directory reconstruction is unsound for sibling/nested directories

**File:** `bench/runtime/repomap_eval_capture_regen.go:282-320`

**Issue:** Directory depth is derived purely from leading-space count
(`depth := indent / 2`, `:300`) and the `dirStack` is truncated only when a `/`-
suffixed line is seen with `depth < len(dirStack)` (`:302-305`). The file-vs-content
discriminator is `repomap.LangFromExt(trimmed) != ""` (`:307`). Two problems if the
captured tree ever nests: (1) a content/def line whose trimmed text ends in a known
extension token (e.g. a Go line ending in a string literal like `"x.go"`, or any
def line whose last field has a recognized suffix) would be misclassified as a file
leaf, resetting `curFile` and silently dropping subsequent symbols; (2) the depth
truncation assumes a strict DFS where every directory line monotonically tracks
`indent/2`, but the renderer sorts directories-first then files
(`internal/repomap/render.go:182-190`), so a file at a shallower depth following a
deeper directory subtree relies on the next directory line to fix the stack — there
is no per-file depth reset, so a stale `dirStack` can prepend the wrong parent path
to a file's IDs.

This is **latent only**: the committed corpus exercises are flat single-directory
fixtures, so the heuristic produces correct IDs today and the goldens pass. The file
is `//go:build ignore` (excluded from `go build ./...`), so it is local-tooling, not
shipped code.

**Fix:** Anchor the parser to the renderer's exact structure rather than a
spaces/2 + extension heuristic: track depth from the `indent` of `/`-suffixed lines
and reset `curFile`/recompute the dir prefix from each file line's own indent
(content lines are always `fileIndent + 4` per `render.go:200`), and gate the
"file leaf" branch on the line being at a directory-or-root indent rather than on
`LangFromExt` of arbitrary trimmed text. At minimum, add a regen-time assertion that
re-derives each emitted relpath against the on-disk file set so a misparse fails-not-
skips instead of committing wrong IDs.

## Info

### IN-01: `firstText` JSON shape uses a capitalized `"Content"` tag that relies on case-insensitive matching

**File:** `bench/runtime/repomap_eval_capture_regen.go:253-274`

**Issue:** `firstText` unmarshals the marshaled `*mcp.CallToolResult` into a local
struct tagged `json:"Content"` (capitalized, `:263`), but the SDK type marshals the
field as lowercase `"content"` (`CallToolResult.Content []Content
\`json:"content"\``, go-sdk v1.5.0 `protocol.go:81`). This works today only because
Go's `encoding/json` unmarshal performs a case-insensitive fallback match, so
`"content"` still binds to `json:"Content"`. It is functionally correct but
brittle/misleading — a future reader could "fix" the casing or a stricter decoder
would break it, and on a true miss the function returns `""` which (correctly)
fail-not-skips via `callTree`'s empty-content error (`:240-242`). Low risk; the file
is build-excluded local tooling.

**Fix:** Change the tag to `json:"content"` to match the wire shape exactly (and
likewise rely on the documented lowercase `"text"` tag, which already matches).

---

_Reviewed: 2026-06-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
