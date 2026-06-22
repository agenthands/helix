# Phase 75: Schema, Fairness Contract & Tree Skeleton - Pattern Map

**Mapped:** 2026-06-14
**Files analyzed:** 14 new/modified artifacts
**Analogs found:** 11 / 14 (3 are data/doc files with prose-only analogs)

This phase produces *contracts and skeletons*. RESEARCH.md correctly identified near-exact
in-tree templates for almost every artifact. The analogs below were read and verified live;
line numbers are load-bearing for the planner.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/schema/result.v2.schema.json` | config (JSON Schema contract) | transform/validation | `test/oracle/contract/schema_test.go` (validator), Draft 2020-12 meta-schema | role-match (no in-tree .schema.json authored; analog is the *validation* pattern) |
| `bench/schema/result.v2_test.go` (golden validate) | test | validation | `test/oracle/contract/schema_test.go` | exact |
| `bench/schema/testdata/result.v2.golden.json` | test fixture | static | `eval/fixtures/*` testdata idiom | role-match |
| `bench/runners/fairness_contract.go` | config (compile-time pin) + loader | transform | `internal/eval/report/cost_summary.go` (struct+stamp); RESEARCH Pattern 2 (idiom) | partial (no exact analog; Go literal + Validate idiom) |
| `bench/runners/fairness_contract_test.go` | test | validation | `cmd/eval-attestation-check/run_test`-style injected-clock test | role-match |
| `bench/datasets/cost-table.yaml` | config (data) | static/CRUD | none (eval has no YAML pricing file; D-15 keeps separate) | no analog |
| `bench/PROVIDERS.md` | doc + config (YAML frontmatter) | static | `eval/EVAL.md` (prose + attestation table) | role-match |
| `bench/BENCH.md` | doc | static | `eval/EVAL.md` | exact |
| `bench/LICENSES.md` | doc | static | `eval/EVAL.md` (table prose) | partial |
| `cmd/helix-bench/main.go` | CLI (cobra) | request-response | `cmd/helix-eval/main.go` | exact |
| `cmd/helix-bench/main_test.go` | test | validation | `cmd/helix-eval/main_test.go` | exact |
| `make verify-tos` + `validate-cost-table` (Makefile) + validator code | build tooling | validation/batch | `cmd/eval-attestation-check/main.go` (date gate) + `Makefile:237` (warn-only — INVERT to hard-fail) | role-match, deliberate divergence |
| Wave 0 `git mv` bench/* → internal/semantic/bench/ + cross-ref edits | build/VCS | — | (mechanical; cross-refs in `bench_fts_probe.go:4`, `fixtures/.../README.md:41,49`) | n/a |
| `.planning/REQUIREMENTS.md` BENCH-03 line 19 (`v1`→`v2`) | doc edit | static | n/a (one-line doc fix) | n/a |

## Pattern Assignments

### `cmd/helix-bench/main.go` (CLI, cobra) — EXACT analog

**Analog:** `cmd/helix-eval/main.go`

**Package doc + main() + root tree** (lines 1-56):
```go
package main

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd constructs the cobra command tree. Exported for testing.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "helix-eval",
		Short: "Helix evaluation harness",
		Long:  `...`,
	}
	root.AddCommand(newRunCmd())
	root.AddCommand(newValidateRulesCmd())
	return root
}
```
For helix-bench: `Use: "helix-bench"`, register the 5 BENCH-02 subcommands
(`newRunCmd`, `newFetchDatasetsCmd`, `newDoctorCmd`, `newReportCmd`, `newValidateCostTableCmd`).

**Subcommand factory pattern** (lines 59-96): each subcommand is a `func newXxxCmd() *cobra.Command`
returning a `*cobra.Command` with `RunE:` (NOT `Run:` — so errors propagate to `os.Exit(1)` and
deferred cleanup is not bypassed; see the WR-02 note at `cmd/helix-eval/main.go:246-247`). Flags
bound via `cmd.Flags().StringVar(...)` over closure-captured locals (lines 60-93). Skeleton
subcommands return a clear "not yet implemented (Phase NN)" error or no-op; `doctor` must exit 0
on a clean host (success criterion #2).

**Exit-code contract** (lines 245-252): return an error from `RunE`, never `os.Exit` deep inside,
so cobra owns the exit and cleanup runs.

---

### `cmd/helix-bench/main_test.go` (test) — EXACT analog

**Analog:** `cmd/helix-eval/main_test.go`

**In-process --help capture** (lines 16-31):
```go
func TestHelixEvalCommandHelp(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	require.NoError(t, err) // cobra exits 0 for --help
	out := buf.String()
	assert.Contains(t, out, "run", "help output must mention 'run' subcommand")
}
```
For helix-bench: assert all 5 subcommands appear in `--help` output (BENCH-02 acceptance) by
asserting `Contains` for each, or by iterating `root.Commands()` and checking `len == 5`. Uses
`bytes.Buffer` + `SetOut/SetErr/SetArgs` — no subprocess, no PATH dependence.

---

### `bench/schema/result.v2_test.go` (test) — EXACT analog (golden validates)

**Analog:** `test/oracle/contract/schema_test.go`

**Compiler setup, Draft 2020-12, offline** (lines 20-27):
```go
c := jsonschema.NewCompiler()
c.DefaultDraft(jsonschema.Draft2020)
sch, err := c.Compile("https://json-schema.org/draft/2020-12/schema") // meta-schema, no network
require.NoError(t, err)
```

**Validation call** (lines 52, 69): `err := sch.Validate(<instance>)` then `require.NoError`.

**CRITICAL v6 gotcha (from RESEARCH Pitfall 2, not visible in this analog):** the schema_test.go
analog validates `tool.InputSchema` (already a `map[string]any` from the SDK). For Phase 75 the
instance is read from a JSON file, so it MUST be decoded with `jsonschema.UnmarshalJSON(reader)`
(preserves `json.Number`), NOT `encoding/json` into a struct/map. Full golden-test shape is in
RESEARCH.md "Code Examples > Golden-fixture-validates-against-schema test" (lines 445-465):
read schema bytes → `UnmarshalJSON` → `AddResource("result.v2.schema.json", doc)` →
`Compile(...)` → `UnmarshalJSON` the fixture → `sch.Validate(inst)`.

**Testify idiom:** `github.com/stretchr/testify/require` with `t.Helper()` on shared setup
(line 21). `t.Run(subname, ...)` for per-case subtests (lines 50, 64).

---

### `bench/schema/result.v2.schema.json` (JSON Schema contract) — role-match

**Analog:** validated *by* `test/oracle/contract/schema_test.go`; no authored `.schema.json` exists in-tree.

**Authoring constraints (from CONTEXT D-03/D-04 + RESEARCH Pitfall 5):**
- `"$schema": "https://json-schema.org/draft/2020-12/schema"` (repo standard — schema_test.go:24).
- Top-level required field `schema_version` (string, const/enum `"v2"`) — D-04, success criterion #3.
- Keep `required` MINIMAL; new fields land OPTIONAL to honor additive-only=minor (D-03).
- Include FAIR-03 cached-input columns as fields (`tokens_input_cached_read`,
  `tokens_input_cache_write`) and a `fairness.overrides[]` array (D-10/D-12).
- Document the additive-only stance in a `$comment` (Open Question #2).

---

### `bench/runners/fairness_contract.go` (compile-time config pin + loader) — partial

**Analog (struct + version stamping idiom):** `internal/eval/report/cost_summary.go`

**Versioned struct + stamp pattern** (cost_summary.go lines 9-35):
```go
// SchemaVersion is always "1".
type CostSummary struct {
	SchemaVersion string `json:"schema_version"`
	// ...
}
func WriteCostSummary(path string, results []EvalResult) error {
	cs := CostSummary{ SchemaVersion: "1", ... }   // stamp the version literal
	data, err := json.MarshalIndent(cs, "", "  ")
	...
	os.WriteFile(path, data, 0600)  // note: 0600 perms idiom
}
```
For fairness_contract.go: bump to `"v2"` for the result stamp; the contract itself is a
package-level `var DefaultContract = FairnessContract{...}` literal (D-09). Full struct shape +
`Validate()` (log.Fatal on empty WaiverReason, D-10) + injected-clock `DeprecationGate(today, ...)`
(D-11) are in RESEARCH.md Pattern 2 (lines 209-255) and Pattern 3 (lines 257-273).

**Anti-pattern to enforce (RESEARCH lines 322-327):** `ModelID` MUST be a DATED snapshot
(e.g. `claude-sonnet-4-5-20260128`), NEVER an alias. The eval judge uses the alias
`claude-sonnet-4-6` (`cmd/helix-eval/main.go:90` flag default) — that is exactly what FAIR-02
forbids here.

---

### `make verify-tos` / `make validate-cost-table` + validator (build tooling) — role-match, INVERT exit policy

**Analog:** `cmd/eval-attestation-check/main.go` (date-staleness gate) + Makefile target `eval-attestation-check` (lines 237-238)

**Date parse + staleness math** (eval-attestation-check/main.go lines 26-89):
```go
const stalenessThresholdDays = 180
var verifiedAtRe = regexp.MustCompile(`(?m)^\*\*Verified at:\*\*\s+(\d{4}-\d{2}-\d{2})`)

verified, err := time.Parse("2006-01-02", m[1])
ageDays := int(time.Since(verified).Hours() / 24)
if ageDays > stalenessThresholdDays { /* WARNING */ }
```

**Testable entry point** (line 43): `func run(errOut io.Writer, args []string) int` — `main`
calls `os.Exit(run(...))`. Reuse this shape so the gate is unit-testable with good/stale fixtures.

**DELIBERATE DIVERGENCE (CONTEXT D-16, RESEARCH Pitfall 3):** this analog is WARN-ONLY (exit 0
even on stale/parse-error; wired `continue-on-error: true`). Phase 75 needs HARD-FAIL: `verify-tos`
and `validate-cost-table` MUST exit non-zero on missing/expired(>90d) attestation, past
`valid_until`, or unparseable rows. Threshold is 90 days here (not 180). RESEARCH recommends
implementing the validators inside `cmd/helix-bench` (`go run ./cmd/helix-bench validate-cost-table`)
rather than a separate `cmd/`; the Makefile target shells to it.

**Makefile target shape** (analog Makefile lines 237-238):
```make
eval-attestation-check:
	go run ./cmd/eval-attestation-check eval/EVAL.md
```
New targets follow the same `go run ./cmd/...` form but WITHOUT `continue-on-error` in CI.

---

### `bench/BENCH.md` / `bench/PROVIDERS.md` / `bench/LICENSES.md` (docs) — role-match

**Analog:** `eval/EVAL.md`

**Attestation table + dated verification line** (EVAL.md lines 5-19):
```markdown
## Provider Retention Attestation
**Verified at:** 2026-05-10 (Phase 67 planning)
**Re-verification cadence:** Every minor Helix release (quarterly minimum)

### Anthropic API ...
| Attribute | Value | Source |
|-----------|-------|--------|
| Default API log retention | 7 days | https://... |
```
For PROVIDERS.md (D-14): use a `---`-delimited YAML frontmatter block per provider
(`provider, tos_url, attested_by, attested_on, benchmarking_permitted, publish_permitted`)
parsed by `verify-tos` (strict decode, RESEARCH Pattern 5), followed by EVAL.md-style prose tables.
For BENCH.md (D-07/D-08): document the six-dir runtime-vs-contract distinction, the INFRA-03
eval↔bench separation note, and FLAG the `make bench` collision (RESEARCH Pitfall 1). A reciprocal
INFRA-03 pointer paragraph must be added to `eval/EVAL.md` in the same wave.

---

### `bench/datasets/cost-table.yaml` (config data) — NO analog

Authored fresh per CONTEXT D-13. Strict `yaml.v3` decode struct + per-row date validation in
RESEARCH.md "Code Examples > Strict cost-table decode" (lines 467-489). Use
`yaml.NewDecoder(f)` + `dec.KnownFields(true)` so unknown keys hard-fail (D-16).

---

## Shared Patterns

### Versioned-struct schema_version stamping
**Source:** `internal/eval/report/cost_summary.go:11-12,33`
**Apply to:** `fairness_contract.go` result emission, `result.v2.schema.json` design
A `SchemaVersion string` field stamped with a literal at marshal time; greppable flat string
(D-04 `"v2"`). Note the `0600` file-perm idiom (cost_summary.go:61).

### Cobra RunE error-propagation (no deep os.Exit)
**Source:** `cmd/helix-eval/main.go:31-36,245-252`
**Apply to:** all `cmd/helix-bench` subcommands
`main` is the ONLY `os.Exit` site; subcommands return errors via `RunE`.

### Offline Draft 2020-12 validation (santhosh-tekuri v6)
**Source:** `test/oracle/contract/schema_test.go:20-27`
**Apply to:** the golden-validate test and any downstream producer self-check
`NewCompiler()` → `DefaultDraft(jsonschema.Draft2020)` → `Compile`. Decode INSTANCES with
`jsonschema.UnmarshalJSON` (Pitfall 2).

### Injected-clock date gate (testability + reproducibility)
**Source:** `cmd/eval-attestation-check/main.go:43,76-82` (testable `run()`); RESEARCH Pattern 3
**Apply to:** FAIR-02 deprecation gate, `validate-cost-table` valid_until/last_verified checks
Pass `today time.Time` as a parameter; CLI passes `time.Now().UTC()`, tests pass a fixed date.
`time.Parse("2006-01-02", ...)` is the date layout.

### Strict YAML decode for hard-fail (D-16)
**Source:** RESEARCH.md Pattern 5 + Code Examples (no in-tree analog uses yaml.v3 strict decode)
**Apply to:** `cost-table.yaml`, `PROVIDERS.md` frontmatter
`yaml.NewDecoder(r)` + `dec.KnownFields(true)` → unknown/malformed → error → exit non-zero.

### Lint-analyzer restraint (prose-only INFRA-03)
**Source:** `internal/lint/` (`nokernel2semantic`, `nosemantic2kernel`, `noduckdb`) precedent
**Apply to:** D-08 — DO NOT add a `vet-noeval2bench` analyzer this phase; INFRA-03 is prose-only.

## Wave 0 Cross-Reference Edits (atomic with the git mv)

Per RESEARCH Runtime State Inventory (lines 358-364), these two cross-refs must update in the
SAME commit as the `git mv`, or `git grep "bench/semantic_bench"` shows dangling pointers:
- `internal/semantic/store/bench_fts_probe.go:4` — comment `bench/semantic_bench_test.go` →
  `internal/semantic/bench/semantic_bench_test.go`
- `bench/fixtures/synthetic_50k_go/README.md:41,49` — `bench/semantic_bench_test.go` and
  `./bench/...` → `internal/semantic/bench/...`

`make bench` (Makefile:84-85) runs `./test/bench/...` — it does NOT reference the moved `bench/`
dir, so the move does not break it. But the NAME collides with future BENCH-05 (Phase 77); BENCH.md
must flag this (RESEARCH Pitfall 1). The microbench files keep `package bench`; only import is the
absolute module path `internal/semantic/store` — unaffected by the move.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `bench/datasets/cost-table.yaml` | config data | static | eval has no YAML pricing file; D-15 keeps bench independent. Use RESEARCH struct (lines 467-489). |
| `bench/runners/fairness_contract.go` (the literal + waiver loader) | config pin | transform | No compile-time-pin precedent in-tree; pure-Go idiom in RESEARCH Pattern 2/3. cost_summary.go covers only the versioned-struct half. |
| `bench/schema/result.v2.schema.json` (the schema document itself) | contract | — | No authored `.schema.json` exists; in-tree pattern is the *validator*, not an authored schema. Author per D-03/D-04 + Pitfall 5. |

## Metadata

**Analog search scope:** `cmd/helix-eval/`, `cmd/eval-attestation-check/`, `internal/eval/report/`,
`test/oracle/contract/`, `eval/`, `bench/`, `internal/semantic/store/`, `internal/lint/`, `Makefile`
**Files scanned:** 8 analogs read in full + bench/ listing + Makefile target grep + cross-ref grep
**Pattern extraction date:** 2026-06-14
