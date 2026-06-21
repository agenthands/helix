package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/bench/runners"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// injectedToday is the fixed clock used across the validator tests so the date
// logic is deterministic and reproducible (D-11). It matches the freshness of
// the committed good fixtures (cost-table.yaml last_verified 2026-06-15;
// PROVIDERS.md attested_on 2026-06-15).
var injectedToday = time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

const (
	goodCostTable           = "../../bench/datasets/cost-table.yaml"
	staleCostTable          = "../../bench/datasets/testdata/cost-table-stale.yaml"
	goodProviders           = "../../bench/PROVIDERS.md"
	staleProviders          = "../../bench/PROVIDERS-stale.testdata.md"
	staleAnthropicProviders = "../../bench/datasets/testdata/providers-stale-anthropic.testdata.md"
)

func TestValidateCostTableGoodPasses(t *testing.T) {
	require.NoError(t, validateCostTable(injectedToday, goodCostTable),
		"the committed good cost-table must pass at the injected clock")
}

func TestValidateCostTablePastValidUntilFails(t *testing.T) {
	// The stale fixture has valid_until 2026-01-01 (< injected today) AND a
	// last_verified > 90 days stale — either one must hard-fail.
	require.Error(t, validateCostTable(injectedToday, staleCostTable),
		"a past valid_until / stale last_verified must hard-fail")
}

func TestValidateCostTableStaleLastVerifiedFails(t *testing.T) {
	// last_verified > 90 days before injected today, valid_until still in future.
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.yaml")
	body := `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: 3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.30
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-01-01
    deprecation_at: 2027-01-28
`
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	require.Error(t, validateCostTable(injectedToday, p),
		"last_verified > 90 days stale must hard-fail")
}

func TestValidateCostTableMalformedRowFails(t *testing.T) {
	// Unknown/extra YAML key → strict KnownFields(true) decode must error.
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.yaml")
	body := `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: 3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.30
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
    bogus_unknown_key: nope
`
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	require.Error(t, validateCostTable(injectedToday, p),
		"an unknown YAML key must hard-fail under strict decode")
}

func TestValidateCostTableEmptyRequiredFieldFails(t *testing.T) {
	// WR-01: a row with an empty provider/model_id/currency must hard-fail.
	cases := map[string]string{
		"empty model_id": `rows:
  -
    provider: Anthropic
    model_id: ""
    input_per_mtok: 3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.30
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`,
		"empty provider": `rows:
  -
    provider: ""
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: 3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.30
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`,
		"empty currency": `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: 3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.30
    currency: ""
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "ct.yaml")
			require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
			require.Error(t, validateCostTable(injectedToday, p),
				"an empty required string field must hard-fail")
		})
	}
}

func TestValidateCostTableNonPositivePriceFails(t *testing.T) {
	// WR-02: a zero or negative input/output price must hard-fail; a missing
	// price decodes to 0.0 which is exactly this case.
	cases := map[string]string{
		"missing input price": `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.30
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`,
		"zero output price": `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: 3.0
    output_per_mtok: 0.0
    cached_input_per_mtok: 0.30
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`,
		"negative input price": `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: -3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.30
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`,
		"negative cached price": `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: 3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: -0.30
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "ct.yaml")
			require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
			require.Error(t, validateCostTable(injectedToday, p),
				"a non-positive price must hard-fail")
		})
	}
}

func TestValidateCostTableZeroCachedPricePasses(t *testing.T) {
	// WR-02: cached_input_per_mtok == 0 is legitimate (providers without prompt
	// caching) and must NOT hard-fail.
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.yaml")
	body := `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: 3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.0
    currency: USD
    valid_until: 2027-01-28
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	require.NoError(t, validateCostTable(injectedToday, p),
		"cached_input_per_mtok == 0 must be accepted")
}

func TestCostTableMatchesDefaultContractModelID(t *testing.T) {
	// WR-03 drift-guard: cost-table.yaml documents that `model_id` MUST match
	// bench/runners DefaultContract.ModelID. Enforce it so re-pinning the
	// contract without updating the cost table (or vice versa) hard-fails CI,
	// mirroring the TestSystemPromptHashMatches pattern. bench/runners is a
	// sibling package in the same module — no import cycle.
	data, err := os.ReadFile(goodCostTable)
	require.NoError(t, err)

	var ct CostTable
	require.NoError(t, yaml.Unmarshal(data, &ct))

	want := runners.DefaultContract.ModelID
	require.NotEmpty(t, want, "DefaultContract.ModelID must be set")

	found := false
	for _, row := range ct.Rows {
		if row.ModelID == want {
			found = true
			break
		}
	}
	require.True(t, found,
		"cost-table.yaml must contain a row with model_id == DefaultContract.ModelID (%q)", want)
}

func TestVerifyTOSGoodPasses(t *testing.T) {
	require.NoError(t, verifyTOS(injectedToday, goodProviders),
		"the committed good PROVIDERS.md must pass at the injected clock")
}

func TestVerifyTOSValidatesAllFourProviders(t *testing.T) {
	// CR-01 regression: the parser must find and validate EVERY provider
	// attestation in the real PROVIDERS.md — Anthropic, OpenAI, Google, Local —
	// despite the stray Markdown horizontal rule. Asserting the COUNT (not just
	// "the file passes") is the guard that the Anthropic block is no longer
	// silently dropped by delimiter mis-pairing.
	n, err := verifyTOSCount(injectedToday, goodProviders)
	require.NoError(t, err, "the committed good PROVIDERS.md must pass")
	require.GreaterOrEqual(t, n, 4,
		"verify-tos must validate >= 4 provider attestations (Anthropic, OpenAI, Google, Local)")
}

func TestVerifyTOSStaleAnthropicFails(t *testing.T) {
	// CR-01 regression: a PROVIDERS.md whose ANTHROPIC block is stale must
	// hard-fail. The fixture reproduces the original bug shape (a leading
	// Markdown horizontal rule before the first attestation) to prove the
	// Anthropic block is now actually validated rather than skipped.
	require.Error(t, verifyTOS(injectedToday, staleAnthropicProviders),
		"a stale attested_on in the Anthropic block must hard-fail")
}

func TestVerifyTOSStaleAttestationFails(t *testing.T) {
	require.Error(t, verifyTOS(injectedToday, staleProviders),
		"an attested_on > 90 days stale must hard-fail")
}

func TestVerifyTOSMalformedFrontmatterFails(t *testing.T) {
	// Unknown key in a frontmatter block → strict decode must error.
	dir := t.TempDir()
	p := filepath.Join(dir, "PROVIDERS.md")
	body := `# malformed

---
provider: Anthropic
tos_url: https://example.com/tos
attested_by: helix-maintainers
attested_on: 2026-06-15
benchmarking_permitted: true
publish_permitted: true
bogus_unknown_key: nope
---

trailing prose
`
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	require.Error(t, verifyTOS(injectedToday, p),
		"an unknown frontmatter key must hard-fail under strict decode")
}
