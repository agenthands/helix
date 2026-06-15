package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// injectedToday is the fixed clock used across the validator tests so the date
// logic is deterministic and reproducible (D-11). It matches the freshness of
// the committed good fixtures (cost-table.yaml last_verified 2026-06-15;
// PROVIDERS.md attested_on 2026-06-15).
var injectedToday = time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

const (
	goodCostTable  = "../../bench/datasets/cost-table.yaml"
	staleCostTable = "../../bench/datasets/testdata/cost-table-stale.yaml"
	goodProviders  = "../../bench/PROVIDERS.md"
	staleProviders = "../../bench/PROVIDERS-stale.testdata.md"
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
    model_id: claude-sonnet-4-5-20260128
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
    model_id: claude-sonnet-4-5-20260128
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

func TestVerifyTOSGoodPasses(t *testing.T) {
	require.NoError(t, verifyTOS(injectedToday, goodProviders),
		"the committed good PROVIDERS.md must pass at the injected clock")
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
