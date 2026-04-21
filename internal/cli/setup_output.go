package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

// SetupPrinter handles colored terminal output for setup command.
// All output goes to os.Stderr (stdout reserved for MCP JSON-RPC and generic client JSON output).
// Respects NO_COLOR env automatically via fatih/color.
type SetupPrinter struct {
	DryRun bool
}

// Success prints a green checkmark + formatted message to stderr.
func (p *SetupPrinter) Success(format string, args ...any) {
	green := color.New(color.FgGreen)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", green.Sprint("\u2713"), msg)
}

// Failure prints a red cross + formatted message to stderr.
func (p *SetupPrinter) Failure(format string, args ...any) {
	red := color.New(color.FgRed)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", red.Sprint("\u2717"), msg)
}

// Info prints a blue arrow + formatted message to stderr.
func (p *SetupPrinter) Info(format string, args ...any) {
	blue := color.New(color.FgBlue)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", blue.Sprint("->"), msg)
}

// DryRunAction prints a yellow "[dry-run]" prefix + formatted message to stderr.
// Only prints when DryRun is true.
func (p *SetupPrinter) DryRunAction(format string, args ...any) {
	if !p.DryRun {
		return
	}
	yellow := color.New(color.FgYellow)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", yellow.Sprint("[dry-run]"), msg)
}
