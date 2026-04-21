package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/postfix/serena/internal/kernel/lspool"
)

// StatusPrinter handles colored terminal output for the status command.
// All output goes to Writer (defaults to os.Stderr).
// Respects NO_COLOR env automatically via fatih/color.
type StatusPrinter struct {
	// Writer is the output destination. Defaults to os.Stderr if nil.
	Writer io.Writer
}

func (p *StatusPrinter) writer() io.Writer {
	if p.Writer != nil {
		return p.Writer
	}
	return os.Stderr
}

// Success prints a green checkmark + formatted message.
func (p *StatusPrinter) Success(format string, args ...any) {
	green := color.New(color.FgGreen)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(p.writer(), "%s %s\n", green.Sprint("\u2713"), msg)
}

// Failure prints a red cross + formatted message.
func (p *StatusPrinter) Failure(format string, args ...any) {
	red := color.New(color.FgRed)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(p.writer(), "%s %s\n", red.Sprint("\u2717"), msg)
}

// Warning prints a yellow exclamation mark + formatted message.
func (p *StatusPrinter) Warning(format string, args ...any) {
	yellow := color.New(color.FgYellow)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(p.writer(), "%s %s\n", yellow.Sprint("!"), msg)
}

// Info prints a blue arrow + formatted message.
func (p *StatusPrinter) Info(format string, args ...any) {
	blue := color.New(color.FgBlue)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(p.writer(), "%s %s\n", blue.Sprint("->"), msg)
}

// PrintReport formats a HealthReport for terminal display.
// In default mode (verbose=false), only unhealthy items are shown.
// If all healthy, the Summary field provides a single-line message per D-11.
func (p *StatusPrinter) PrintReport(report *lspool.HealthReport, verbose bool) {
	// If summary is set (all healthy in default mode per D-11), show it and return.
	if report.Summary != "" && !verbose {
		p.Info("%s", report.Summary)
		return
	}

	for _, ws := range report.Workspaces {
		fmt.Fprintf(p.writer(), "\nWorkspace: %s\n", ws.Root)

		if verbose {
			fmt.Fprintf(p.writer(), "  Languages: %s\n", strings.Join(ws.Languages, ", "))
		}

		for _, w := range ws.Workers {
			capStr := strings.Join(w.Capabilities, ", ")

			switch {
			case strings.HasPrefix(w.State, "healthy"):
				p.Success("%s (%s) %s", w.Language, w.Command, capStr)
			case w.State == "degraded":
				p.Warning("%s (%s) degraded", w.Language, w.Command)
			case w.State == "failed":
				p.Failure("%s (%s) failed", w.Language, w.Command)
			}
		}

		for _, c := range ws.Circuits {
			if c.State != "closed" {
				p.Failure("%s circuit breaker %s (%d failures)", c.Language, c.State, c.Failures)
			}
		}
	}
}

// HasFailures returns true if any worker is in failed state or any circuit
// is open. Used to set exit code 1 per D-07.
func (p *StatusPrinter) HasFailures(report *lspool.HealthReport) bool {
	for _, ws := range report.Workspaces {
		for _, w := range ws.Workers {
			if w.State == "failed" {
				return true
			}
		}
		for _, c := range ws.Circuits {
			if c.State == "open" {
				return true
			}
		}
	}
	return false
}
