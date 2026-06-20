package cost

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// injectedToday is the fixed clock used across the cost tests so the date logic
// is deterministic and reproducible (D-11). It matches the freshness of the
// committed good fixture (cost-table.yaml last_verified 2026-06-15).
var injectedToday = time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

const goodCostTable = "../../bench/datasets/cost-table.yaml"

// freshRow is a single valid_until-future, last_verified-fresh row at the
// injected clock — the building block for the PriceFor positive cases.
const freshRow = `rows:
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
`

func writeTable(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "ct.yaml")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

func TestValidateCostTableGoodPasses(t *testing.T) {
	require.NoError(t, ValidateCostTable(injectedToday, goodCostTable),
		"the committed good cost-table must pass at the injected clock")
}

func TestValidateCostTableStaleLastVerifiedFails(t *testing.T) {
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
	require.Error(t, ValidateCostTable(injectedToday, writeTable(t, body)),
		"last_verified > 90 days stale must hard-fail")
}

func TestValidateCostTableMalformedRowFails(t *testing.T) {
	body := freshRow + "    bogus_unknown_key: nope\n"
	require.Error(t, ValidateCostTable(injectedToday, writeTable(t, body)),
		"an unknown YAML key must hard-fail under strict decode")
}

func TestLoadCostTableStrictDecode(t *testing.T) {
	ct, err := LoadCostTable(writeTable(t, freshRow))
	require.NoError(t, err)
	require.Len(t, ct.Rows, 1)
	require.Equal(t, "claude-sonnet-4-5-20250929", ct.Rows[0].ModelID)
	require.Equal(t, 3.0, ct.Rows[0].InputPerMtok)

	_, err = LoadCostTable(writeTable(t, "rows: []\n"))
	require.Error(t, err, "an empty rows list must hard-fail")

	_, err = LoadCostTable("does-not-exist.yaml")
	require.Error(t, err, "a missing file must hard-fail")
}

func TestPriceForMatchReturnsRow(t *testing.T) {
	ct, err := LoadCostTable(writeTable(t, freshRow))
	require.NoError(t, err)

	row, err := PriceFor(ct, "claude-sonnet-4-5-20250929", injectedToday)
	require.NoError(t, err)
	require.Equal(t, 3.0, row.InputPerMtok)
	require.Equal(t, 15.0, row.OutputPerMtok)
	require.Equal(t, 0.30, row.CachedInputPerMtok)
}

func TestPriceForUnknownModelIsHardError(t *testing.T) {
	ct, err := LoadCostTable(writeTable(t, freshRow))
	require.NoError(t, err)

	_, err = PriceFor(ct, "no-such-model", injectedToday)
	require.Error(t, err, "an unknown model_id must be a hard error (cannot price)")
	require.Contains(t, err.Error(), "no row for model_id")
}

func TestPriceForPastValidUntilFailsClosed(t *testing.T) {
	body := `rows:
  -
    provider: Anthropic
    model_id: claude-sonnet-4-5-20250929
    input_per_mtok: 3.0
    output_per_mtok: 15.0
    cached_input_per_mtok: 0.30
    currency: USD
    valid_until: 2026-01-01
    last_verified: 2026-06-15
    deprecation_at: 2027-01-28
`
	ct, err := LoadCostTable(writeTable(t, body))
	require.NoError(t, err)

	_, err = PriceFor(ct, "claude-sonnet-4-5-20250929", injectedToday)
	require.Error(t, err, "a past valid_until must fail closed on lookup")
}

func TestPriceForStaleLastVerifiedFailsClosed(t *testing.T) {
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
	ct, err := LoadCostTable(writeTable(t, body))
	require.NoError(t, err)

	_, err = PriceFor(ct, "claude-sonnet-4-5-20250929", injectedToday)
	require.Error(t, err, "a last_verified > 90 days stale must fail closed on lookup")
}
