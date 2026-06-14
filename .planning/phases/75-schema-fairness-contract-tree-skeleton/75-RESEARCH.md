# Phase 75: Schema, Fairness Contract & Tree Skeleton - Research

**Researched:** 2026-06-14
**Domain:** Go contracts/skeletons — JSON Schema authoring + validation, compile-time config pin, YAML validation gates, `git mv` refactor, cobra CLI skeleton
**Confidence:** HIGH

## Summary

Phase 75 lays the contracts and skeletons for the v1.12 bench stack. The single most
important research finding is that **every library this phase needs is already vendored** in
`go.mod` — no new dependency is required, satisfying the single-binary/CGO constraint
trivially:

- `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2` (direct) — JSON Schema validation, already
  used at `test/oracle/contract/schema_test.go` with Draft 2020-12.
- `gopkg.in/yaml.v3 v3.0.1` (direct) — cost-table.yaml + PROVIDERS.md frontmatter parsing.
- `github.com/spf13/cobra v1.10.2` (direct) — CLI skeleton (CLAUDE.md says v1.9.1; that is
  **stale** — actual vendored version is 1.10.2 [VERIFIED: go.mod]).

The second key finding is that **near-exact templates already exist in the tree** for almost
every artifact this phase produces: `cmd/helix-eval/main.go` is the cobra skeleton template;
`cmd/eval-attestation-check/main.go` is the date-staleness gate template (but warn-only — D-16
requires hard-fail, the one deliberate divergence); `eval/EVAL.md` is the BENCH.md / PROVIDERS.md
prose template; `internal/eval/report/cost_summary.go` is the `schema_version`-stamped result
template; `test/oracle/contract/schema_test.go` is the golden-fixture-validates-against-schema
template; and the four `internal/lint/` analyzers + their `cmd/vet-*` wrappers are the precedent
D-08 cites for *not* shipping a `vet-noeval2bench` analyzer yet.

The third finding is a concrete **Wave 0 collision hazard**: `make bench` already exists (Phase 64
microbench at `./test/bench/...`), and the relocated `bench/semantic_bench_*.go` files carry
cross-references from `internal/semantic/store/bench_fts_probe.go` and the fixture README that
must be updated in the same atomic `git mv` commit or `git grep` will show dangling pointers.

**Primary recommendation:** Reuse the existing in-tree templates verbatim (helix-eval cobra
tree, eval-attestation-check date gate, cost_summary.go schema-version stamping, schema_test.go
golden validation, EVAL.md prose). Target JSON Schema **Draft 2020-12** with
`santhosh-tekuri/jsonschema/v6`. Use `gopkg.in/yaml.v3` for both cost-table and PROVIDERS
frontmatter. Add **zero** new dependencies. Resolve the `make bench` name collision before
Wave 0 lands.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `result.v2.schema.json` authoring | Contract (`bench/schema/`) | — | D-02: dedicated contract dir; consumed by all downstream phases as a producer/validator contract |
| Schema validation (Go-side) | Backend / build tooling | Test | Producers validate output; golden fixture test validates the schema itself (mirrors `schema_test.go`) |
| `fairness_contract.go` config pin | Backend (`bench/runners/`) | Build (compile-time) | D-09: pure Go literal — compile-time pin, no runtime back-channel |
| Waiver loader (`log.Fatal` on empty reason) | Backend | — | D-10: loader is a runtime startup gate |
| `cost-table.yaml` / `PROVIDERS.md` validation | Build tooling (`cmd/helix-bench`, Make) | CI | D-16: `make` targets that hard-fail CI |
| `helix-bench` CLI skeleton | CLI (`cmd/`) | — | BENCH-02: cobra entrypoint, sibling of `cmd/helix-eval` |
| Wave 0 `git mv` relocation | Build / VCS | — | D-05/D-06: package move, import-path update, atomic commit |
| eval↔bench separation | Docs (prose) | — | D-08: prose-only this phase, no analyzer |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | Validate `result.v2.json` against `result.v2.schema.json`; validate the schema itself against the Draft 2020-12 meta-schema | Already vendored & in active use at `test/oracle/contract/schema_test.go`; offline meta-schema; full Draft 2020-12 support [VERIFIED: go.mod + codebase grep] |
| `gopkg.in/yaml.v3` | v3.0.1 | Parse `cost-table.yaml` rows and `PROVIDERS.md` YAML frontmatter for `make validate-cost-table` / `make verify-tos` | Already vendored; canonical Go YAML; strict-decode support via `KnownFields(true)` for malformed-row detection [VERIFIED: go.mod] |
| `github.com/spf13/cobra` | v1.10.2 | `cmd/helix-bench` subcommand tree (`run`, `fetch-datasets`, `doctor`, `report`, `validate-cost-table`) | Already vendored; `cmd/helix-eval` is the in-tree template [VERIFIED: go.mod] |
| `encoding/json` + `time` (stdlib) | go1.25.1 | Result marshalling, `schema_version` stamping, deprecation-calendar date math | Stdlib; mirrors `cost_summary.go` and `eval-attestation-check/main.go` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `regexp` (stdlib) | go1.25.1 | Extract `**Verified at:**`-style dates / frontmatter delimiters from `PROVIDERS.md` | Mirrors `eval-attestation-check/main.go:34` `verifiedAtRe` |
| `github.com/stretchr/testify/require` | (vendored) | Golden-fixture validation test assertions | Already the repo's test idiom (`schema_test.go`) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `santhosh-tekuri/jsonschema/v6` | `xeipuuv/gojsonschema` | gojsonschema is NOT vendored, is effectively unmaintained, and only supports up to Draft-07 — would add a dependency for a strictly worse feature set. Reject. |
| `santhosh-tekuri/jsonschema/v6` | `google/jsonschema-go` (indirect dep, v0.4.2) | Pulled in transitively by the MCP SDK; it is a v0.x API geared to MCP tool schemas, not a general-purpose validator. The repo already chose santhosh-tekuri for general validation in `schema_test.go`. Stay consistent. |
| `gopkg.in/yaml.v3` for validation | `yq`/`grep` shell in the Makefile | D-16 only fixes the hard-fail semantics and parseable formats, not the implementation (Claude's Discretion). A Go validator inside `cmd/helix-bench validate-cost-table` is testable, has no external-binary prereq (cleaner `helix-bench doctor`), and gives precise error messages. **Recommend Go**; `make` target shells to `go run ./cmd/helix-bench validate-cost-table`. |
| Draft 2020-12 | Draft-07 | Draft-07 is older; the repo already standardized on 2020-12 (`schema_test.go:23` `DefaultDraft(jsonschema.Draft2020)`). Use 2020-12 for consistency. |

**Installation:**
```bash
# No installation required — all dependencies already in go.mod.
# Verify the binary builds (BENCH-02 acceptance):
go build ./cmd/helix-bench
```

**Version verification:**
- `santhosh-tekuri/jsonschema/v6` v6.0.2 — `go list -m` confirms; `go doc` confirms API
  surface (`NewCompiler`, `AddResource`, `Compile`, `Schema.Validate`, `UnmarshalJSON`)
  [VERIFIED: go doc].
- `spf13/cobra` v1.10.2 — `go list -m github.com/spf13/cobra` → `v1.10.2` [VERIFIED: go list].
  **CLAUDE.md's "cobra v1.9.1" is stale.**
- `gopkg.in/yaml.v3` v3.0.1 [VERIFIED: go.mod].

## Package Legitimacy Audit

> All packages this phase uses are **already vendored direct dependencies** that ship in the
> production `helix` binary today. No new package is introduced. No registry fetch occurs.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/santhosh-tekuri/jsonschema/v6` | Go modules | mature (v6 line) | high (widely used) | github.com/santhosh-tekuri/jsonschema | OK | Approved — already vendored & in use |
| `gopkg.in/yaml.v3` | Go modules | mature | very high | github.com/go-yaml/yaml | OK | Approved — already vendored & in use |
| `github.com/spf13/cobra` | Go modules | mature | very high | github.com/spf13/cobra | OK | Approved — already vendored & in use |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*No WebSearch-discovered or training-data-only packages are recommended. Every library named
here was confirmed present in `go.mod` and exercised by existing code in this repository.*

## Architecture Patterns

### System Architecture Diagram

```
                          ┌─────────────────────────────────────────────┐
                          │           bench/ (six dirs, D-07)            │
                          │  datasets/ runners/ languages/ evaluators/   │
                          │  reports/        schema/ (contract dir)      │
                          └─────────────────────────────────────────────┘
                                    │                        │
      downstream producer          │                        │  validates against
      (Phase 79 evaluator) ───────►│ result.v2.json ───────►│ result.v2.schema.json
                                    │  (schema_version:"v2") │   (Draft 2020-12)
                                    ▼                        ▼
                          ┌──────────────────┐    ┌──────────────────────────┐
                          │ golden fixture   │    │ santhosh-tekuri/jsonschema│
                          │ (this phase)     │───►│ Compiler.Compile + Validate│ ──► PASS/FAIL
                          └──────────────────┘    └──────────────────────────┘

   bench/runners/fairness_contract.go
     var DefaultContract = FairnessContract{ model_id (DATED), temperature, max_tokens,
                                             system_prompt_hash, retry_policy, cache_policy }
     map[mode]ModeOverride{ WaiverReason, ApprovedBy }
          │
          │ Load() ──► if any override has empty WaiverReason ──► log.Fatal  (D-10)
          │ DeprecationGate(today, cost-table) ──► fail if deprecation_at − today < 30d (D-11)
          ▼
   every bench runner (Phase 80+) reads effective config from DefaultContract — never re-declares

   cmd/helix-bench (cobra, sibling of cmd/helix-eval)
     run · fetch-datasets · doctor · report · validate-cost-table
                                                      │
                                                      ▼
                          bench/datasets/cost-table.yaml ──► yaml.v3 strict decode
                          bench/PROVIDERS.md (YAML frontmatter) ──► verify-tos
                                                      │
   Makefile: make validate-cost-table / make verify-tos ──► hard-fail (exit≠0) on
             past valid_until, expired (>90d) attestation, or unparseable rows (D-16)
```

### Recommended Project Structure
```
bench/
├── schema/
│   └── result.v2.schema.json     # D-02 contract dir; Draft 2020-12
├── datasets/
│   └── cost-table.yaml           # D-13 pricing+staleness rows
├── runners/
│   └── fairness_contract.go      # D-09 pure-Go literal + loader + ModeOverride
├── languages/                    # (empty skeleton this phase)
├── evaluators/                   # (empty skeleton this phase)
├── reports/                      # (empty skeleton this phase)
├── BENCH.md                      # D-07/D-08/INFRA-03 prose; mirrors eval/EVAL.md
├── PROVIDERS.md                  # D-14 YAML frontmatter + prose (INFRA-01)
└── LICENSES.md                   # INFRA-02 dataset license audit (success criterion #5)

cmd/helix-bench/                  # BENCH-02 cobra skeleton; mirror cmd/helix-eval/
├── main.go
└── main_test.go

internal/semantic/bench/          # D-05 Wave 0 destination for Phase 64 microbench
├── semantic_bench_test.go
├── semantic_bench_fts_test.go
├── semantic_bench_REPORT.md
├── _blevprobe/
└── fixtures/
```

Note: a `result.v2/` schema golden fixture should live wherever the validation test reads it
(e.g. `bench/schema/testdata/result.v2.golden.json`), following `schema_test.go`'s testdata
idiom.

### Pattern 1: Offline JSON Schema validation (santhosh-tekuri v6)
**What:** Compile the schema, decode the instance with `jsonschema.UnmarshalJSON` (NOT
`encoding/json` into a struct), then `Validate`.
**When to use:** The golden-fixture-validates test and any downstream producer self-check.
**Example:**
```go
// Source: santhosh-tekuri/jsonschema/v6 (go doc) + repo precedent test/oracle/contract/schema_test.go
import "github.com/santhosh-tekuri/jsonschema/v6"

c := jsonschema.NewCompiler()
c.DefaultDraft(jsonschema.Draft2020)            // [VERIFIED: schema_test.go:23]

// Add the schema as a resource (offline; no network fetch).
schemaDoc, _ := jsonschema.UnmarshalJSON(strings.NewReader(schemaBytes))
_ = c.AddResource("result.v2.schema.json", schemaDoc)
sch, err := c.Compile("result.v2.schema.json")   // returns *jsonschema.Schema

// CRITICAL v6 gotcha: decode the INSTANCE with jsonschema.UnmarshalJSON,
// which preserves number precision (json.Number). Passing a struct or a map
// produced by encoding/json can mis-validate numeric/format keywords.
inst, _ := jsonschema.UnmarshalJSON(bytes.NewReader(resultJSON))
if err := sch.Validate(inst); err != nil {       // *jsonschema.ValidationError on failure
    // err.Error() is a structured, multi-line, human-readable report
}
```

### Pattern 2: Compile-time config pin with waiver loader (D-09/D-10)
**What:** A package-level `var DefaultContract` literal plus a `Load`/validate function that
`log.Fatal`s on any override with an empty `WaiverReason`.
**When to use:** `bench/runners/fairness_contract.go`.
**Example:**
```go
// Source: D-09/D-10 + Go idiom (no external pattern needed)
package runners

type RetryPolicy struct { MaxRetries int; BackoffMs int }
type CachePolicy  struct { Mode string } // e.g. "ephemeral_5m"

type ModeOverride struct {
    MaxTokens    *int    // pointer = "override present"; nil = inherit
    Temperature  *float64
    WaiverReason string  // free-text (D-10) — emergent reasons, no closed enum
    ApprovedBy   string  // maintainer signoff
}

type FairnessContract struct {
    ModelID          string        // DATED snapshot, e.g. "claude-sonnet-4-5-20260128" (FAIR-02)
    Temperature      float64
    MaxTokens        int
    SystemPromptHash string        // sha256 hex of the canonical system prompt bytes
    Retry            RetryPolicy
    Cache            CachePolicy
    Overrides        map[string]ModeOverride // keyed by mode name
}

var DefaultContract = FairnessContract{ /* pinned literal */ }

// Validate fatals on any override lacking a WaiverReason (D-10).
func (c FairnessContract) Validate() {
    for mode, ov := range c.Overrides {
        if strings.TrimSpace(ov.WaiverReason) == "" {
            log.Fatalf("fairness_contract: mode %q overrides without WaiverReason — refusing to start", mode)
        }
    }
}
```

**`system_prompt_hash` derivation (Claude's Discretion within D-09):** Pin the *hash*, not the
prompt text, in the Go literal. Compute it as `sha256.Sum256([]byte(canonicalSystemPrompt))`
hex-encoded, where the canonical prompt lives in a committed file (e.g.
`bench/runners/system_prompt.txt`). A unit test recomputes the hash from the file and asserts
equality with the pinned literal — so a prompt edit that isn't re-pinned fails CI. This keeps
the contract greppable while making drift a hard error.

### Pattern 3: Deprecation-calendar gate (D-11, static, reproducible)
**What:** Pure date math against `deprecation_at` in `cost-table.yaml` — no live provider API.
**When to use:** FAIR-02 gate inside the loader / `validate-cost-table`.
**Example:**
```go
// Source: D-11 + time stdlib; date-parse idiom from eval-attestation-check/main.go:76
const dateLayout = "2006-01-02"
dep, err := time.Parse(dateLayout, row.DeprecationAt)
// "today" is injected (not time.Now() directly) so tests are deterministic and
// runs reproduce from --run-id. The CLI passes time.Now().UTC(); tests pass a fixed date.
if dep.Sub(today) < 30*24*time.Hour {
    return fmt.Errorf("fairness gate: model %s/%s within 30d of deprecation (%s)", row.Provider, row.ModelID, row.DeprecationAt)
}
```
> Inject `today` as a parameter (do not call `time.Now()` deep in the gate). FAIR-02's
> acceptance test ("fails with a pin within 30 days of a known deprecation") requires
> deterministic control of the clock, and D-11 demands reproducibility from `--run-id`.

### Pattern 4: cobra skeleton (mirror cmd/helix-eval)
**What:** `newRootCmd()` returning a `*cobra.Command` with `AddCommand` for each subcommand;
`main()` calls `Execute()` and `os.Exit(1)` on error.
**When to use:** `cmd/helix-bench/main.go`.
**Example:**
```go
// Source: cmd/helix-eval/main.go:31-56 (in-tree template)
func main() {
    if err := newRootCmd().Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
func newRootCmd() *cobra.Command {
    root := &cobra.Command{Use: "helix-bench", Short: "Helix benchmark harness"}
    root.AddCommand(newRunCmd(), newFetchDatasetsCmd(), newDoctorCmd(), newReportCmd(), newValidateCostTableCmd())
    return root
}
// Skeleton subcommands return a clear "not yet implemented (Phase NN)" error or no-op
// so `helix-bench --help` lists all 5 (BENCH-02) and `helix-bench doctor` exits 0 on a
// clean Linux host (success criterion #2).
```
> Export `newRootCmd` so a `main_test.go` can assert the 5 subcommands are registered
> (mirrors `cmd/helix-eval/main_test.go` + `run_cmd_test.go`).

### Pattern 5: YAML frontmatter parse for PROVIDERS.md (D-14)
**What:** Split the leading `---`-delimited block, decode it with `yaml.v3`, keep the prose
that follows.
**Example:**
```go
// Source: D-14 + gopkg.in/yaml.v3 idiom
// PROVIDERS.md may hold one frontmatter block per provider (multi-doc) OR a single
// front-matter + prose. Recommend a per-provider markdown section, each opening with a
// fenced ```yaml ... ``` or a "---\n...\n---" block.
type ProviderAttestation struct {
    Provider             string `yaml:"provider"`
    TOSURL               string `yaml:"tos_url"`
    AttestedBy           string `yaml:"attested_by"`
    AttestedOn           string `yaml:"attested_on"`            // YYYY-MM-DD
    BenchmarkingPermitted bool  `yaml:"benchmarking_permitted"`
    PublishPermitted     bool   `yaml:"publish_permitted"`
}
dec := yaml.NewDecoder(bytes.NewReader(frontmatter))
dec.KnownFields(true) // strict: unknown keys → error → hard-fail (D-16)
```

### Anti-Patterns to Avoid
- **Aliased model snapshot in the contract.** The repo's judge uses the alias
  `claude-sonnet-4-6` (`internal/eval/judge/judge.go:22`). That is exactly what FAIR-02
  forbids for the fairness contract — `ModelID` MUST be a **dated** snapshot
  (e.g. `claude-sonnet-4-5-20260128`), no runtime alias resolution.
- **`time.Now()` buried in the deprecation gate.** Breaks reproducibility (D-11) and the
  FAIR-02 acceptance test. Inject the clock.
- **Validating the instance via `encoding/json` into a struct, then passing it to v6.**
  v6 expects `jsonschema.UnmarshalJSON` output (preserves `json.Number`). Mixing decoders
  silently weakens numeric/format checks.
- **Adding a `vet-noeval2bench` analyzer this phase.** D-08 explicitly defers it; INFRA-03 is
  prose-only (matches the `internal/lint/` precedent of shipping analyzers only with a concrete
  leak to catch).
- **Letting `make bench` (BENCH-05, future) silently shadow the existing Phase 64 `make bench`.**
  See Pitfall 1.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON Schema validation | A field-by-field Go validator | `santhosh-tekuri/jsonschema/v6` | Already vendored; handles `$ref`, enums, `additionalProperties:false`, format, draft semantics correctly |
| Schema-is-valid-schema check | Hand-checking the `.schema.json` | Compile against the Draft 2020-12 **meta-schema** (offline) | `schema_test.go:20-27` already does exactly this for tool schemas |
| YAML parsing / strict-row detection | regex/`grep` field extraction | `yaml.v3` with `Decoder.KnownFields(true)` | Strict decode catches malformed/unknown rows for D-16 hard-fail; greppable error messages |
| Cobra subcommand wiring | Manual `os.Args` switch | `spf13/cobra` (mirror `cmd/helix-eval`) | Consistent `--help`, flag parsing, testable `newRootCmd()` |
| Date staleness math | Custom calendar code | `time.Parse(...)` + subtraction | `eval-attestation-check/main.go` is the template |

**Key insight:** This phase's entire value is *contracts that don't drift*. Hand-rolled
validators drift from the schema they're supposed to enforce; the whole point of a JSON Schema
file + a real validator is that the contract is declarative and machine-checked. Likewise the
`system_prompt_hash` recompute test and `KnownFields(true)` strict decode are how the contracts
stay self-enforcing.

## Runtime State Inventory

> Phase 75 is partly a relocation (Wave 0, D-05/D-06 `git mv`). This inventory covers what the
> move touches beyond the files themselves.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — the Phase 64 microbench reads a generated fixture (`bench/fixtures/synthetic_50k_go/`, gitignored output) and writes nothing persistent. Verified by reading `semantic_bench_test.go`. | none |
| Live service config | None — no daemon/service references `bench/` paths. Verified by `git grep` (only test files + fixture README + `bench_fts_probe.go` comment). | none |
| OS-registered state | None — no Task Scheduler / systemd / pm2 entries reference bench paths (Go-only repo, no such registrations). | none |
| Secrets/env vars | None — the microbench uses no env vars or secrets. | none |
| Build artifacts / source cross-refs | (1) `internal/semantic/store/bench_fts_probe.go:1-4` comment points at `bench/semantic_bench_test.go`. (2) `bench/fixtures/synthetic_50k_go/README.md:41,49` references `bench/semantic_bench_test.go` and `./bench/...`. (3) `Makefile` has **no** target pointing at `./bench/...` (the `benchfts` run is documented in file comments only, invoked manually). (4) The fixture `gen.go` carries `//go:build ignore` and uses relative paths from its own dir — unaffected by the move. | Update comment in `bench_fts_probe.go` and the two README cross-refs to `internal/semantic/bench/...` **in the same Wave 0 commit** so `git grep "bench/semantic_bench"` is clean afterward. |

**Import-path fallout (D-05):** The files keep `package bench`. Their only internal import is
`github.com/agenthands/helix/internal/semantic/store` (an *absolute* module path) — unaffected
by the directory move. The `benchfts` build tag and `//go:build` constraints are unaffected.
The invocation command changes from `go test -tags=benchfts ... ./bench/...` to
`./internal/semantic/bench/...`. No `go.mod`/`replace` changes needed (single module).

**Nothing found in categories 1–4:** Confirmed by `git grep` for `bench/_blevprobe`,
`bench/fixtures`, `bench/semantic_bench` across `*.go`, `Makefile`, `*.yml`, `*.md` (excluding
`legacy/` and `.planning/`).

## Common Pitfalls

### Pitfall 1: `make bench` name collision
**What goes wrong:** `make bench` already exists (`Makefile:84`, runs the Phase 64 microbench
suite `go test -bench=. ./test/bench/...`). BENCH-05 (a *future* phase, 77) introduces a
`make bench` that invokes `cmd/helix-bench run`. Two targets with the same name → the later
`.PHONY`/recipe silently wins, breaking one of them.
**Why it happens:** The roadmap names overlap; the existing target predates the milestone.
**How to avoid:** This phase does NOT own `make bench` (BENCH-05 is Phase 77), but the planner
should **flag the collision now** and decide the rename: either rename the Phase 64 target
(e.g. `make microbench`) or namespace the new one. Phase 75 *should* at minimum note this in
`bench/BENCH.md` so Phase 77 doesn't trip on it.
**Warning signs:** `grep -n "^bench:" Makefile` returning two matches; `make bench` running the
wrong suite.

### Pitfall 2: santhosh-tekuri v6 instance decoding
**What goes wrong:** Passing a Go struct or an `encoding/json`-decoded `map[string]any` to
`Schema.Validate` causes numeric/`format` keywords to mis-evaluate (floats vs `json.Number`).
**Why it happens:** v6 was redesigned around `json.Number`-preserving decoding; the old v5 habit
of passing arbitrary maps no longer holds.
**How to avoid:** Always decode the instance with `jsonschema.UnmarshalJSON` before `Validate`.
**Warning signs:** A golden fixture that "should" pass fails only on integer/number fields.

### Pitfall 3: warn-only vs hard-fail divergence
**What goes wrong:** Copying `eval-attestation-check` (which is **warn-only**, exit 0 even on
stale dates, wired with `continue-on-error: true`) into `verify-tos` would silently violate
D-16's hard-fail requirement.
**Why it happens:** The template is 90% right but its exit-code policy is the opposite of what
D-16 demands.
**How to avoid:** `make verify-tos` and `make validate-cost-table` MUST exit non-zero (return
an error from cobra `RunE`, or `os.Exit(1)`) on: missing/expired (>90d) attestation, past
`valid_until`, unparseable rows. No `continue-on-error` on these CI steps.
**Warning signs:** CI green despite a deliberately-stale `attested_on` in a test fixture.

### Pitfall 4: `git mv` continuity broken by a non-atomic move
**What goes wrong:** Splitting the relocation across commits (or `rm` + add) loses
`git log --follow` continuity for the Phase 64 microbench history.
**Why it happens:** Git only tracks renames heuristically; a single-commit `git mv` of each
file preserves the rename detection cleanly.
**How to avoid:** D-06: one atomic commit, `git mv` each path, update the three cross-refs in the
same commit. Verify with `git log --follow internal/semantic/bench/semantic_bench_test.go`.
**Warning signs:** `git log --follow` on the moved file stops at the move commit.

### Pitfall 5: `schema_version` required-but-additive tension (D-03/D-04)
**What goes wrong:** Marking too many fields `required` in `result.v2.schema.json` makes the
"additive-only = minor" contract impossible — adding a new required field is a breaking (major)
change, but the temptation is to require everything.
**Why it happens:** Schema authors default to `required` for completeness.
**How to avoid:** Keep `required` minimal (at least `schema_version`, per success criterion #3).
New fields land as **optional** to stay minor (D-03). `schema_version` is a single top-level
string `"v2"` (D-04). Consider `additionalProperties: false` only if you accept that *any* new
field is then a schema change — D-03's additive policy is easier with
`additionalProperties: true` or unspecified, OR with `false` plus disciplined optional-field
additions. Document the chosen stance in a schema `$comment`/sidebar.
**Warning signs:** A downstream phase needing a new metric forces a v3 bump for a purely
additive change.

### Pitfall 6: six dirs vs BENCH-01's literal five
**What goes wrong:** BENCH-01's acceptance text lists five dirs
(`datasets,runners,languages,evaluators,reports`); D-07 adds a sixth (`schema/`). A
`tree -d -L 2 bench/` assertion copied verbatim from BENCH-01 will fail.
**Why it happens:** The requirement predates the schema-dir decision (planner note #2).
**How to avoid:** Assert **six** dirs; `bench/BENCH.md` marks `schema/` as contract-only vs the
five runtime dirs. Also apply planner note #1 (REQUIREMENTS.md BENCH-03 line 19 `v1`→`v2`).
**Warning signs:** Acceptance test or BENCH.md listing only five dirs.

## Code Examples

### Golden-fixture-validates-against-schema test
```go
// Source: adapted from test/oracle/contract/schema_test.go (in-tree precedent)
func TestResultV2GoldenValidates(t *testing.T) {
    schemaBytes, err := os.ReadFile("result.v2.schema.json")
    require.NoError(t, err)
    fixture, err := os.ReadFile("testdata/result.v2.golden.json")
    require.NoError(t, err)

    c := jsonschema.NewCompiler()
    c.DefaultDraft(jsonschema.Draft2020)
    doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
    require.NoError(t, err)
    require.NoError(t, c.AddResource("result.v2.schema.json", doc))
    sch, err := c.Compile("result.v2.schema.json")
    require.NoError(t, err)

    inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(fixture))
    require.NoError(t, err)
    require.NoError(t, sch.Validate(inst), "golden result.v2.json must validate")
}
```

### Strict cost-table decode (hard-fail on malformed rows)
```go
// Source: gopkg.in/yaml.v3 + D-13/D-16
type CostRow struct {
    Provider          string `yaml:"provider"`
    ModelID           string `yaml:"model_id"`
    InputPerMtok      float64 `yaml:"input_per_mtok"`
    OutputPerMtok     float64 `yaml:"output_per_mtok"`
    CachedInputPerMtok float64 `yaml:"cached_input_per_mtok"`
    Currency          string `yaml:"currency"`
    ValidUntil        string `yaml:"valid_until"`     // YYYY-MM-DD
    LastVerified      string `yaml:"last_verified"`   // YYYY-MM-DD
    DeprecationAt     string `yaml:"deprecation_at"`  // YYYY-MM-DD
}
type CostTable struct{ Rows []CostRow `yaml:"rows"` }

dec := yaml.NewDecoder(f)
dec.KnownFields(true)                 // unknown key → error → exit non-zero (D-16)
var ct CostTable
if err := dec.Decode(&ct); err != nil { /* hard-fail */ }
// then: for each row parse the three dates; fail if valid_until < today,
// or (today - last_verified) > 90d, or any date unparseable (D-16).
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `xeipuuv/gojsonschema` (Draft-07, stagnant) | `santhosh-tekuri/jsonschema/v6` (Draft 2020-12) | repo already on v6 | Use v6; do not introduce gojsonschema |
| JSON Schema Draft-07 | Draft 2020-12 | repo standard (`schema_test.go`) | Author `result.v2.schema.json` with `"$schema": "https://json-schema.org/draft/2020-12/schema"` |
| Model alias at runtime (`claude-sonnet-4-6`, judge) | Dated snapshot pin (FAIR-02) | this milestone | Fairness contract pins a dated snapshot, never an alias |

**Deprecated/outdated:**
- CLAUDE.md "cobra v1.9.1": stale — actual `v1.10.2`. (Doc-only nit; behavior compatible.)
- `xeipuuv/gojsonschema`: do not add; effectively unmaintained and Draft-07-only.

## Project Constraints (from CLAUDE.md)

- **Go single binary, CGO=1.** No Python/Docker/runtime deps in this phase's deliverables (the
  Python/Docker prereqs documented in `bench/BENCH.md` are *operator-side*, BENCH-06, not build
  deps). [Satisfied: zero new deps.]
- **Run `go vet ./...` and `go test ./...` before completing any Go task.** Plans MUST include
  these. Also `make vet` (runs the four `vet-*` analyzers).
- **`modernc.org/sqlite` (CGO-free) for FTS5** — not touched by this phase.
- **GSD workflow enforcement** — all edits via a GSD command.
- **SMTC-first tool routing** for code-aware lookups — used during this research.
- **CLAUDE.md "cobra v1.9.1" is stale** — actual vendored version is v1.10.2; planner may note
  a doc-fix but it is not load-bearing for this phase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `claude-sonnet-4-5-20260128` is the intended FAIR-01 dated snapshot to pin | Anti-Patterns / Pattern 2 | Low — it is FAIR-01's own example string; the exact pinned snapshot is a maintainer decision at write time. The *pattern* (dated, not aliased) is what's load-bearing. [ASSUMED] |
| A2 | `system_prompt_hash` = sha256 hex of a committed canonical prompt file, with a recompute test | Pattern 2 | Low — D-09 only fixes that the hash is pinned in the literal; derivation is Claude's Discretion. This is a recommendation, not a locked requirement. [ASSUMED] |
| A3 | PROVIDERS.md uses a per-provider frontmatter block (vs one global block) | Pattern 5 | Low — D-14 fixes the field set and "YAML frontmatter per provider" wording; exact multi-block vs single layout is a formatting choice. [ASSUMED] |
| A4 | A Go validator (not yq/grep) implements `validate-cost-table`/`verify-tos` | Stack / Alternatives | Low — explicitly Claude's Discretion (D-16 fixes only hard-fail semantics). Recommendation, not requirement. [ASSUMED] |

**All other claims are [VERIFIED] against go.mod / go doc / codebase grep or [CITED] from
in-tree files.**

## Open Questions (RESOLVED)

> All three open questions are substantively settled. Q1 is deferred to Phase 77 (documented,
> not silently dropped); Q2 and Q3 fall within Claude's Discretion (CONTEXT.md) and are locked
> by the choices the plans actually took (75-02, 75-03).

1. **`make bench` collision ownership. (RESOLVED)**
   - What we know: `make bench` already exists (Phase 64 microbench); BENCH-05 (Phase 77)
     wants the name.
   - What's unclear: whether Phase 75 renames the existing target or just documents the conflict.
   - RESOLVED: **Deferred to Phase 77.** Phase 75 does NOT rename the existing `make bench`
     target — Plan 75-01 explicitly leaves the Makefile `make bench` target untouched. Plan
     75-02 (Task 1, `bench/BENCH.md`) records the collision as a flagged note so Phase 77
     (BENCH-05 owner) resolves the rename/namespacing. No Wave-0 rename is performed.

2. **`additionalProperties` stance in `result.v2.schema.json`. (RESOLVED)**
   - What we know: D-03 wants additive-only = minor; D-04 wants `schema_version` required.
   - What's unclear: whether the schema sets `additionalProperties: false` (stricter, but every
     new field is a change) or leaves it open.
   - RESOLVED: **`additionalProperties` is left OPEN** (not set to `false`) at the top level so
     additive optional fields stay minor (D-03), with the additive-only=minor / breaking=v3
     policy recorded in a schema `$comment`. Locked by Plan 75-03 and verified by its
     `TestResultV2AdditiveFieldStaysValid` (an extra unknown key must keep the instance valid).
     Within Claude's Discretion (CONTEXT.md).

3. **Where the golden `result.v2.json` fixture lives. (RESOLVED)**
   - What we know: success criterion #3 needs a golden fixture that validates.
   - What's unclear: exact path.
   - RESOLVED: the golden fixture lives at **`bench/schema/testdata/result.v2.golden.json`**
     (co-located with the schema and its test, following the repo's `testdata/` convention).
     Locked by Plan 75-03's `files_modified` and `TestResultV2GoldenValidates`.

## Environment Availability

> Phase 75 is contracts/skeletons only — its *build* has no external dependencies beyond the Go
> toolchain (all libraries vendored). The operator-side prereqs (Python 3.11+, Docker) that
> BENCH-06 requires `bench/BENCH.md` to *document* are not needed to build or test this phase.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Go builds/tests | ✓ | go1.26.0 (module targets 1.25.1) | — |
| `santhosh-tekuri/jsonschema/v6` | schema validation | ✓ (vendored) | v6.0.2 | — |
| `gopkg.in/yaml.v3` | cost-table/PROVIDERS parse | ✓ (vendored) | v3.0.1 | — |
| `spf13/cobra` | helix-bench CLI | ✓ (vendored) | v1.10.2 | — |
| `git` | Wave 0 `git mv` | ✓ | (repo is a git repo) | — |
| `tree` | BENCH-01 `tree -d -L 2 bench/` acceptance | unknown on CI | — | acceptance can use `find bench -maxdepth 2 -type d` if `tree` absent |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** `tree` (acceptance-only; substitute `find` if needed).

## Validation Architecture

> `workflow.nyquist_validation` not checked as explicitly false — section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/stretchr/testify/require` |
| Config file | none (Go convention) |
| Quick run command | `go test ./cmd/helix-bench/... ./bench/...` |
| Full suite command | `go test ./...` then `make vet` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BENCH-01 | six-dir `bench/` tree; `eval/` byte-identical | unit/script | `find bench -maxdepth 2 -type d` + git diff on `eval/` | ❌ Wave 0 |
| BENCH-02 | `go build ./cmd/helix-bench`; 5 subcommands listed | unit | `go test ./cmd/helix-bench/...` (assert `newRootCmd` registers 5) | ❌ Wave 0 |
| BENCH-03 | golden `result.v2.json` validates; `schema_version` required | unit | `go test ./bench/schema/...` (Pattern: golden validates) | ❌ Wave 0 |
| BENCH-06 | `bench/BENCH.md` renders; `make verify-tos` stale → non-zero | unit | `go test ./cmd/helix-bench/...` (verify-tos fixture) | ❌ Wave 0 |
| FAIR-01 | every runner's effective config == DefaultContract | unit | `go test ./bench/runners/...` | ❌ Wave 0 |
| FAIR-02 | dated pin OK; pin within 30d of EOL fails | unit | `go test ./bench/runners/...` (injected clock) | ❌ Wave 0 |
| FAIR-03 | (variance gate; Phase 82/89) — schema-substrate field presence here | unit | golden fixture includes cached-input columns | ❌ Wave 0 |
| COST-01 | cost-table validates; past `valid_until` / >90d → fail | unit | `go test ./cmd/helix-bench/...` (validate-cost-table fixture) | ❌ Wave 0 |
| INFRA-01 | PROVIDERS.md attestation parsed; stale → fail | unit | verify-tos fixture test | ❌ Wave 0 |
| INFRA-02 | `bench/LICENSES.md` per-dataset license present | doc/render | presence check | ❌ Wave 0 |
| INFRA-03 | eval↔bench note in BENCH.md + cross-link in EVAL.md | doc/render | presence/grep check | ❌ Wave 0 |
| (Wave 0) | `git mv` continuity + cross-refs updated | script | `go test -tags=benchfts ./internal/semantic/bench/...` + `git grep "bench/semantic_bench"` empty | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./<touched-package>/...`
- **Per wave merge:** `go test ./...` + `make vet`
- **Phase gate:** full suite green + `make verify-tos` + `make validate-cost-table` exit 0 on
  good fixtures and non-zero on the deliberately-stale fixtures.

### Wave 0 Gaps
- [ ] `cmd/helix-bench/main_test.go` — asserts 5 subcommands (BENCH-02); mirror `cmd/helix-eval/main_test.go`
- [ ] `bench/schema/result.v2.schema.json` + `testdata/result.v2.golden.json` + validation test (BENCH-03)
- [ ] `bench/runners/fairness_contract_test.go` — empty-WaiverReason fatal + 30d gate (FAIR-01/02)
- [ ] cost-table + PROVIDERS validator tests with good/stale fixtures (COST-01/INFRA-01)
- [ ] Wave 0 relocation test: `go test -tags=benchfts ./internal/semantic/bench/...` passes; `git grep "bench/semantic_bench"` clean
- [ ] Framework install: none needed (all vendored)

## Security Domain

> `security_enforcement` not explicitly false. This phase introduces no auth, no network input,
> no crypto beyond a content hash. ASVS surface is minimal.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | `yaml.v3` strict decode (`KnownFields(true)`) for cost-table/PROVIDERS; `santhosh-tekuri/jsonschema/v6` for result JSON. Both reject malformed input → hard-fail (D-16). |
| V6 Cryptography | minimal | `crypto/sha256` (stdlib) for `system_prompt_hash` — a content-integrity digest, not a secret. Never hand-roll a hash. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed/oversized cost-table or schema fixture | DoS / Tampering | Strict YAML decode + schema validation; files are repo-committed, not untrusted external input |
| Stale TOS attestation / expired pricing slipping through | Repudiation / compliance | D-16 hard-fail gates (`verify-tos`, `validate-cost-table`) in CI |
| Prompt drift defeating the fairness pin | Tampering (fairness integrity) | `system_prompt_hash` recompute test; dated model snapshot (no alias) |

## Sources

### Primary (HIGH confidence)
- `cmd/helix-eval/main.go`, `cmd/helix-eval/main_test.go` — cobra skeleton template
- `cmd/eval-attestation-check/main.go` — date-staleness gate template (warn-only; invert for D-16)
- `internal/eval/report/cost_summary.go`, `eval_result.go` — `schema_version` stamping + result struct
- `test/oracle/contract/schema_test.go` — santhosh-tekuri/jsonschema v6 Draft 2020-12 offline validation
- `eval/EVAL.md` — BENCH.md / PROVIDERS.md prose + attestation template
- `Makefile` (lines 1, 17–50, 84–88, 219–238) — `make bench` collision, vet/eval target patterns
- `go.mod` — vendored versions (jsonschema/v6 v6.0.2, yaml.v3 v3.0.1, cobra v1.10.2)
- `go doc github.com/santhosh-tekuri/jsonschema/v6` — Compiler/Schema/UnmarshalJSON API
- `bench/semantic_bench_test.go`, `bench/semantic_bench_fts_test.go`, `internal/semantic/store/bench_fts_probe.go`, `bench/fixtures/synthetic_50k_go/README.md` — Wave 0 move scope + cross-refs
- `.planning/phases/75-.../75-CONTEXT.md` (D-01..D-16), `.planning/REQUIREMENTS.md`, `.planning/milestones/v1.12-ROADMAP.md`

### Secondary (MEDIUM confidence)
- (none — all findings grounded in repo files or `go doc`)

### Tertiary (LOW confidence)
- A1–A4 in the Assumptions Log (discretionary recommendations within CONTEXT.md's locked decisions)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library verified present in `go.mod` and exercised by existing code
- Architecture: HIGH — near-exact in-tree templates exist for every artifact
- Pitfalls: HIGH — collision and cross-ref hazards found by direct grep, not inferred

**Research date:** 2026-06-14
**Valid until:** 2026-07-14 (stable — internal contracts + vendored deps; low churn)
</content>
</invoke>
