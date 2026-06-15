package main

import "github.com/spf13/cobra"

// newValidateCostTableCmd returns the 'validate-cost-table' subcommand.
// (Task 1 thin stub; wired to the real hard-fail validator in Task 2.)
func newValidateCostTableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate-cost-table [path]",
		Short: "Strict HARD-FAIL validator for the cost-table pricing+staleness contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			return notYetImplemented("validate-cost-table")(cmd, args)
		},
	}
}
