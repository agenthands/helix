---
phase: 75-schema-fairness-contract-tree-skeleton
reviewed: 2026-06-15T00:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - bench/runners/fairness_contract.go
  - bench/runners/fairness_contract_test.go
  - bench/schema/result.v2.schema.json
  - bench/schema/result.v2_test.go
  - cmd/helix-bench/main.go
  - cmd/helix-bench/main_test.go
  - cmd/helix-bench/validate_cost_table.go
  - cmd/helix-bench/verify_tos.go
  - cmd/helix-bench/validate_test.go
  - bench/datasets/cost-table.yaml
  - bench/PROVIDERS.md
  - bench/BENCH.md
  - Makefile
  - internal/semantic/store/bench_fts_probe.go
findings:
  critical: 1
  warning: 5
  info: 3
  total: 9
status: issues_found
---

# Phase 75: Code Review Report

**Reviewed:** 2026-06-15T00:00:00Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

Phase 75 stands up the bench CONTRACTS-and-SKELETON foundation: a JSON Schema, a
compile-time Go fairness contract, a cobra CLI skeleton, two HARD-FAIL validators
(`validate-cost-table`, `verify-tos`), and cost-table / TOS data. The fairness
contract, deprecation gate (injected clock), schema, and cost-table validator are
largely sound: clocks are injected, YAML is strict-decoded with `KnownFields(true)`,
errors propagate through cobra `RunE` to a single `os.Exit` site, and the schema's
additive-only / required-`schema_version` contract is correctly modeled and tested.

However, the `verify-tos` validator — a HARD-FAIL compliance gate — contains a real
correctness bug: its frontmatter parser **mis-pairs `---` delimiters** because of the
standalone Markdown horizontal-rule `---` lines in `bench/PROVIDERS.md`. As a result the
**Anthropic attestation block (the primary provider) is never validated** — a stale or
malformed-key Anthropic block passes the gate silently. I reproduced this directly. The
remaining findings are gaps in the cost-table validator's field-integrity checks (empty
required fields and zero/negative prices pass silently) and a few robustness / doc-drift
items.

## Critical Issues

### CR-01: verify-tos silently skips the Anthropic attestation block (delimiter mis-pairing)

**File:** `cmd/helix-bench/verify_tos.go:79-100` (parser) and `:42-48` (consumer)
**Issue:**
`extractFrontmatterBlocks` treats *every* line that is exactly `---` as a toggle
between "outside block" and "inside block", alternating on each occurrence. But
`bench/PROVIDERS.md` contains a standalone Markdown horizontal-rule `---` (line 46,
closing the "Display-only safety note" section) **before** the first real YAML
attestation block. That extra delimiter shifts the open/close pairing by one, so the
toggling is misaligned for the rest of the file.

Concretely, running the real parser over the committed `bench/PROVIDERS.md` yields 5
blocks, of which only **three** contain `provider:` (OpenAI, Google, Local). The
Anthropic YAML block (lines 51-56) is captured as the *closing prose* of a non-provider
block and is therefore never strict-decoded or freshness-checked. The first two
extracted blocks are the `## Anthropic API` section header and the Anthropic prose
paragraph.

I confirmed the false-negative by corrupting the Anthropic block in a copy of the file
(set `attested_on: 2020-01-01` and added `bogus_unknown_key: nope`) and running
`verifyTOS(2026-06-15, path)` — it returned `nil` (PASS) despite a stale date **and** an
unknown key that strict-decode is supposed to reject. The HARD-FAIL gate (D-16) has a
hole for the single most important provider, and the existing tests miss it because
`TestVerifyTOSGoodPasses` only asserts the file passes (the 3 surviving blocks are all
valid) and never asserts the *count* of validated attestations or that Anthropic
specifically was checked.

**Fix:**
Make block extraction unambiguous instead of relying on naive toggling. Two viable
approaches:

1. Treat only blocks that *start with* `provider:` as attestation blocks and require
   each attestation block to be a properly fenced `---\nprovider: ...\n---` unit. Detect
   an attestation block by its opening line rather than alternating across unrelated
   horizontal rules. For example, scan for an opening `---` that is *immediately
   followed* by a `provider:` line:

```go
func extractAttestationBlocks(s string) []string {
	var blocks []string
	lines := strings.Split(s, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "---" {
			continue
		}
		// Look ahead: is the next non-empty line a provider: key?
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[j]), "provider:") {
			continue // horizontal rule or non-attestation fence
		}
		// Collect until the closing ---
		var cur []string
		k := j
		for ; k < len(lines) && strings.TrimSpace(lines[k]) != "---"; k++ {
			cur = append(cur, lines[k])
		}
		if k >= len(lines) {
			// unterminated attestation block — surface as an error upstream
			blocks = append(blocks, strings.Join(cur, "\n")+"\n__UNTERMINATED__")
		} else {
			blocks = append(blocks, strings.Join(cur, "\n"))
		}
		i = k
	}
	return blocks
}
```

2. Alternatively, drop the Markdown horizontal rules from `bench/PROVIDERS.md` so the
   delimiters pair cleanly — but that is fragile (any future `---` rule re-breaks the
   gate). Prefer fixing the parser.

Additionally, add a regression test that asserts **exactly 4** attestation blocks are
validated and that a deliberately stale Anthropic block fails:

```go
func TestVerifyTOSValidatesAllFourProviders(t *testing.T) {
	// corrupt only the Anthropic block; verifyTOS must now error.
	...
	require.Error(t, verifyTOS(injectedToday, corruptedPath))
}
```

## Warnings

### WR-01: cost-table validator accepts empty required string fields

**File:** `cmd/helix-bench/validate_cost_table.go:68-88`
**Issue:**
`validateCostTable` validates dates but never checks that `provider`, `model_id`, or
`currency` are non-empty. A row that omits `model_id` (or sets it to `""`) passes the
HARD-FAIL validator. The error messages even interpolate `row.ModelID` to identify the
offending row, which would be blank and unhelpful for exactly such a malformed row. For
a contract validator this is a silent acceptance of malformed data.

**Fix:**
```go
if strings.TrimSpace(row.Provider) == "" || strings.TrimSpace(row.ModelID) == "" || strings.TrimSpace(row.Currency) == "" {
	return fmt.Errorf("cost-table: row %d: provider/model_id/currency must be non-empty", i)
}
```

### WR-02: cost-table validator accepts zero/negative prices

**File:** `cmd/helix-bench/validate_cost_table.go:26-36, 68-88`
**Issue:**
The `*_per_mtok` price fields are `float64` with no validation. A missing price field
decodes to the Go zero value `0.0`, indistinguishable from an intentional `0`, and a
negative price is accepted. Since this cost table feeds fairness cost accounting
downstream (FAIR-03 substrate), a zero or negative price is a data-integrity defect that
would corrupt cost numbers without any signal.

**Fix:**
```go
if row.InputPerMtok <= 0 || row.OutputPerMtok <= 0 || row.CachedInputPerMtok < 0 {
	return fmt.Errorf("cost-table: row %d (%s): non-positive price (input=%v output=%v cached=%v)",
		i, row.ModelID, row.InputPerMtok, row.OutputPerMtok, row.CachedInputPerMtok)
}
```
(cached read may legitimately be 0 for providers without cache, hence `< 0` there.)

### WR-03: model_id ↔ DefaultContract.ModelID consistency is asserted in prose but never enforced

**File:** `bench/datasets/cost-table.yaml:15` and `cmd/helix-bench/validate_cost_table.go`
**Issue:**
The cost-table file comment states: "`model_id` MUST match
bench/runners/fairness_contract.go DefaultContract.ModelID." Nothing enforces this. If
the contract's `ModelID` is re-pinned to a new dated snapshot and the cost table is not
updated (or vice versa), fairness accounting silently uses a model whose pricing/
deprecation row is absent or mismatched — exactly the drift FAIR-02 is meant to prevent.
The phase ships both halves; the cross-link is the natural place to close the loop.

**Fix:**
Add a test (in `cmd/helix-bench` or `bench/runners`) asserting that
`DefaultContract.ModelID` appears as a `model_id` in `bench/datasets/cost-table.yaml`,
mirroring the `TestSystemPromptHashMatches` drift-guard pattern. Cross-package import is
fine here since both live in the same module.

### WR-04: unterminated frontmatter block is silently dropped

**File:** `cmd/helix-bench/verify_tos.go:79-100`
**Issue:**
If a file ends while `inBlock` is true (an opening `---` with no closing `---`), the
accumulated `cur` lines are never appended to `blocks` and the partial block vanishes
with no error. For a HARD-FAIL validator, a truncated final attestation should be a hard
error, not a silent omission. (This is a secondary instance of the CR-01 root cause:
delimiter handling is too permissive.)

**Fix:**
Track unterminated state and return an error, e.g. surface an `__UNTERMINATED__` sentinel
(see CR-01 fix) or `if inBlock { return error }` after the loop, so a malformed file
fails closed.

### WR-05: verify-tos attestation detection via substring `"provider:"` is over-broad

**File:** `cmd/helix-bench/verify_tos.go:45`
**Issue:**
`strings.Contains(block, "provider:")` decides whether a block is an attestation. Any
prose line inside a block that happens to contain the substring `provider:` (e.g. a note
like "the provider: chosen above") would flip a prose block into an attestation block and
then fail strict YAML decode — or, combined with CR-01's mis-pairing, mis-classify
content. The intent is "this YAML document has a top-level `provider` key," which a
substring check does not reliably capture.

**Fix:**
After fixing CR-01 to fence blocks by their opening `provider:` line, classify on the
first non-empty line beginning with `provider:` rather than a free substring match, or
decode first and treat `att.Provider != ""` as the attestation signal.

## Info

### IN-01: DeprecationGate doc comment says "within 30 days ... is rejected" but the boundary is exclusive

**File:** `bench/runners/fairness_contract.go:13, 38-39, 128`
**Issue:**
`dep.Sub(today) < deprecationWindow` rejects strictly-less-than 30 days; exactly 30 days
out passes (confirmed by `TestDeprecationGate` "exactly 30 days out passes"). The doc
comments ("within 30 days of EOL", "A pin within this window ... is rejected") read as
inclusive. The code is internally consistent and tested; only the prose is slightly
imprecise.

**Fix:** Reword the comments to "within fewer than 30 days" / "strictly inside the
window" to match the exclusive boundary.

### IN-02: validate-cost-table / verify-tos read arbitrary file paths from argv

**File:** `cmd/helix-bench/validate_cost_table.go:104-114`, `cmd/helix-bench/verify_tos.go:117-127`
**Issue:**
Both subcommands accept a path arg and `os.Open`/`os.ReadFile` it directly. There is no
path-traversal *vulnerability* here in any meaningful sense — these are developer-facing
build-gate CLIs run locally against repo-relative paths, the file content is only
parsed/validated (never executed), and there is no privilege boundary being crossed. Noting
for completeness against the review checklist: no action required. The free-text
`waiver_reason` / `attested_by` / `tos_url` fields are likewise display-only and are never
shelled or used as a sink (documented and correct, T-75-09 / T-75-04).

### IN-03: verify-tos error on a wrongly-classified prose block could be confusing

**File:** `cmd/helix-bench/verify_tos.go:53-54`
**Issue:**
Once CR-01/WR-05 are addressed, this is moot, but today a prose block that trips the
`provider:` substring check would produce a "strict decode ... failed" error that does
not make clear the offending block was prose, not a malformed attestation. Lower
priority; fixing CR-01 removes the ambiguity.

---

_Reviewed: 2026-06-15T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
