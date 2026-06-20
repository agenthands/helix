// Package cost is the single importable home for the bench cost-table contract:
// the pinned per-(provider, model_id) USD pricing + staleness rows, a strict
// loader, the fail-closed freshness gate, and a model_id lookup. It was lifted
// verbatim out of cmd/helix-bench's package main (Phase 82 Open Q1 — MOVE, not
// duplicate) so BOTH the `validate-cost-table` CLI and the multi-run aggregator
// (bench/aggregator) share ONE parser and ONE freshness gate.
//
// All date logic takes an INJECTED today (never time.Now() inside) so the gate
// is deterministic and reproducible (D-11/D-13).
package cost

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Shared staleness/date constants for the cost-table + TOS validators (D-16).
const (
	// DateLayout is the Go reference layout for all YYYY-MM-DD dates in the
	// cost table and PROVIDERS frontmatter.
	DateLayout = "2006-01-02"
	// StalenessWindowDays is the maximum allowed age (in days) of a
	// last_verified / attested_on date before the validators HARD-FAIL.
	// This is the deliberate inversion of the warn-only eval-attestation-check
	// (180d, continue-on-error): here it is 90d and blocking (D-16).
	StalenessWindowDays = 90
)

// CostRow is one pinned pricing + staleness row (D-13). Dates are YYYY-MM-DD
// strings parsed with DateLayout; floats are per-MTok USD prices.
type CostRow struct {
	Provider           string  `yaml:"provider"`
	ModelID            string  `yaml:"model_id"`
	InputPerMtok       float64 `yaml:"input_per_mtok"`
	OutputPerMtok      float64 `yaml:"output_per_mtok"`
	CachedInputPerMtok float64 `yaml:"cached_input_per_mtok"`
	Currency           string  `yaml:"currency"`
	ValidUntil         string  `yaml:"valid_until"`
	LastVerified       string  `yaml:"last_verified"`
	DeprecationAt      string  `yaml:"deprecation_at"`
}

// CostTable is the top-level cost-table.yaml document.
type CostTable struct {
	Rows []CostRow `yaml:"rows"`
}

// LoadCostTable strict-decodes the cost table at path (yaml.NewDecoder +
// KnownFields(true)) and returns the parsed CostTable. It HARD-FAILS on a
// missing file, an unknown/extra YAML key, or an empty rows list. It does NOT
// apply the freshness gate — that is ValidateCostTable (whole-table) or
// PriceFor (single-row, on lookup). This lets a caller load once and price
// many (the aggregator path) without re-reading the file per model.
func LoadCostTable(path string) (CostTable, error) {
	f, err := os.Open(path)
	if err != nil {
		return CostTable{}, fmt.Errorf("cost-table: cannot open %s: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true) // unknown key → error → exit non-zero (D-16)
	var ct CostTable
	if err := dec.Decode(&ct); err != nil {
		return CostTable{}, fmt.Errorf("cost-table: strict decode of %s failed: %w", path, err)
	}
	if len(ct.Rows) == 0 {
		return CostTable{}, fmt.Errorf("cost-table: %s has no rows", path)
	}
	return ct, nil
}

// ValidateCostTable strict-decodes the cost table at path and HARD-FAILS
// (returns a non-nil error) on: an unknown/extra YAML key, an unparseable
// date, a past valid_until, or a last_verified more than StalenessWindowDays
// old — all relative to the INJECTED today (never time.Now() inside, D-11).
// It returns the first error encountered. This is the exported, package-moved
// rename of the former package-main validateCostTable (body unchanged).
func ValidateCostTable(today time.Time, path string) error {
	ct, err := LoadCostTable(path)
	if err != nil {
		return err
	}

	today = today.UTC().Truncate(24 * time.Hour)
	staleCutoff := today.AddDate(0, 0, -StalenessWindowDays)

	for i, row := range ct.Rows {
		// Required string fields must be non-empty (WR-01). A blank model_id
		// would also make the date/price error messages below unidentifiable.
		if strings.TrimSpace(row.Provider) == "" || strings.TrimSpace(row.ModelID) == "" || strings.TrimSpace(row.Currency) == "" {
			return fmt.Errorf("cost-table: row %d: provider/model_id/currency must be non-empty", i)
		}

		// Prices must be sane (WR-02). A missing input/output price decodes to
		// the Go zero value 0.0, indistinguishable from an intentional 0, so a
		// non-positive price is a data-integrity defect. cached_input_per_mtok
		// may legitimately be 0 for providers without prompt caching, so it is
		// only required to be non-negative.
		if row.InputPerMtok <= 0 || row.OutputPerMtok <= 0 || row.CachedInputPerMtok < 0 {
			return fmt.Errorf("cost-table: row %d (%s): non-positive price (input=%v output=%v cached=%v)",
				i, row.ModelID, row.InputPerMtok, row.OutputPerMtok, row.CachedInputPerMtok)
		}

		validUntil, err := time.Parse(DateLayout, row.ValidUntil)
		if err != nil {
			return fmt.Errorf("cost-table: row %d (%s): cannot parse valid_until %q: %w", i, row.ModelID, row.ValidUntil, err)
		}
		if validUntil.Before(today) {
			return fmt.Errorf("cost-table: row %d (%s): valid_until %s is in the past (today %s)", i, row.ModelID, row.ValidUntil, today.Format(DateLayout))
		}

		lastVerified, err := time.Parse(DateLayout, row.LastVerified)
		if err != nil {
			return fmt.Errorf("cost-table: row %d (%s): cannot parse last_verified %q: %w", i, row.ModelID, row.LastVerified, err)
		}
		if lastVerified.Before(staleCutoff) {
			return fmt.Errorf("cost-table: row %d (%s): last_verified %s is more than %dd stale (cutoff %s)", i, row.ModelID, row.LastVerified, StalenessWindowDays, staleCutoff.Format(DateLayout))
		}

		if _, err := time.Parse(DateLayout, row.DeprecationAt); err != nil {
			return fmt.Errorf("cost-table: row %d (%s): cannot parse deprecation_at %q: %w", i, row.ModelID, row.DeprecationAt, err)
		}
	}
	return nil
}

// PriceFor looks up the cost row for modelID and applies the per-row D-13
// freshness gate against the INJECTED today, returning the row or a hard error.
// It is the per-lookup gate the aggregator (Plan 04) reuses: an unknown model
// is a HARD error (cannot price), a past valid_until is a hard error, and a
// last_verified more than StalenessWindowDays old is a hard error. All three
// fail closed — there is no "warn and continue" path.
func PriceFor(ct CostTable, modelID string, today time.Time) (CostRow, error) {
	today = today.UTC().Truncate(24 * time.Hour)
	staleCutoff := today.AddDate(0, 0, -StalenessWindowDays)

	for _, r := range ct.Rows {
		if r.ModelID != modelID {
			continue
		}
		validUntil, err := time.Parse(DateLayout, r.ValidUntil)
		if err != nil {
			return CostRow{}, fmt.Errorf("cost: model_id %q: cannot parse valid_until %q: %w", modelID, r.ValidUntil, err)
		}
		if validUntil.Before(today) {
			return CostRow{}, fmt.Errorf("cost: model_id %q: valid_until %s is in the past (today %s)", modelID, r.ValidUntil, today.Format(DateLayout))
		}
		lastVerified, err := time.Parse(DateLayout, r.LastVerified)
		if err != nil {
			return CostRow{}, fmt.Errorf("cost: model_id %q: cannot parse last_verified %q: %w", modelID, r.LastVerified, err)
		}
		if lastVerified.Before(staleCutoff) {
			return CostRow{}, fmt.Errorf("cost: model_id %q: last_verified %s is more than %dd stale (cutoff %s)", modelID, r.LastVerified, StalenessWindowDays, staleCutoff.Format(DateLayout))
		}
		return r, nil
	}
	return CostRow{}, fmt.Errorf("cost: no row for model_id %q", modelID)
}
