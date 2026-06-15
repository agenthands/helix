package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Shared staleness/date constants for both validators (D-16).
const (
	// dateLayout is the Go reference layout for all YYYY-MM-DD dates in the
	// cost table and PROVIDERS frontmatter.
	dateLayout = "2006-01-02"
	// stalenessWindowDays is the maximum allowed age (in days) of a
	// last_verified / attested_on date before the validators HARD-FAIL.
	// This is the deliberate inversion of the warn-only eval-attestation-check
	// (180d, continue-on-error): here it is 90d and blocking (D-16).
	stalenessWindowDays = 90
)

// CostRow is one pinned pricing + staleness row (D-13). Dates are YYYY-MM-DD
// strings parsed with dateLayout; floats are per-MTok USD prices.
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

// validateCostTable strict-decodes the cost table at path and HARD-FAILS
// (returns a non-nil error) on: an unknown/extra YAML key, an unparseable
// date, a past valid_until, or a last_verified more than stalenessWindowDays
// old — all relative to the INJECTED today (never time.Now() inside, D-11).
// It returns the first error encountered.
func validateCostTable(today time.Time, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cost-table: cannot open %s: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true) // unknown key → error → exit non-zero (D-16)
	var ct CostTable
	if err := dec.Decode(&ct); err != nil {
		return fmt.Errorf("cost-table: strict decode of %s failed: %w", path, err)
	}
	if len(ct.Rows) == 0 {
		return fmt.Errorf("cost-table: %s has no rows", path)
	}

	today = today.UTC().Truncate(24 * time.Hour)
	staleCutoff := today.AddDate(0, 0, -stalenessWindowDays)

	for i, row := range ct.Rows {
		validUntil, err := time.Parse(dateLayout, row.ValidUntil)
		if err != nil {
			return fmt.Errorf("cost-table: row %d (%s): cannot parse valid_until %q: %w", i, row.ModelID, row.ValidUntil, err)
		}
		if validUntil.Before(today) {
			return fmt.Errorf("cost-table: row %d (%s): valid_until %s is in the past (today %s)", i, row.ModelID, row.ValidUntil, today.Format(dateLayout))
		}

		lastVerified, err := time.Parse(dateLayout, row.LastVerified)
		if err != nil {
			return fmt.Errorf("cost-table: row %d (%s): cannot parse last_verified %q: %w", i, row.ModelID, row.LastVerified, err)
		}
		if lastVerified.Before(staleCutoff) {
			return fmt.Errorf("cost-table: row %d (%s): last_verified %s is more than %dd stale (cutoff %s)", i, row.ModelID, row.LastVerified, stalenessWindowDays, staleCutoff.Format(dateLayout))
		}

		if _, err := time.Parse(dateLayout, row.DeprecationAt); err != nil {
			return fmt.Errorf("cost-table: row %d (%s): cannot parse deprecation_at %q: %w", i, row.ModelID, row.DeprecationAt, err)
		}
	}
	return nil
}

// newValidateCostTableCmd returns the 'validate-cost-table' subcommand. It
// passes time.Now().UTC() to validateCostTable and returns the error via RunE
// so the process exits non-zero on failure — there is NO os.Exit deep inside,
// and NO continue-on-error (D-16).
func newValidateCostTableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate-cost-table [path]",
		Short: "Strict HARD-FAIL validator for the cost-table pricing+staleness contract",
		Long: `validate-cost-table strict-decodes a cost-table YAML (default
bench/datasets/cost-table.yaml) and exits NON-ZERO on an unknown key, an
unparseable date, a past valid_until, or a last_verified more than 90 days old.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "bench/datasets/cost-table.yaml"
			if len(args) == 1 {
				path = args[0]
			}
			if err := validateCostTable(time.Now().UTC(), path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "validate-cost-table: %s OK\n", path)
			return nil
		},
	}
}
