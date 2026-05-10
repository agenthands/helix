// Command eval-attestation-check is a warn-only date-staleness check for
// eval/EVAL.md's Provider Retention Attestation.
//
// Behavior (default — warn-only):
//   - age <= 180 days  : exit 0 silently
//   - age >  180 days  : print WARNING to stderr, exit 0 (still warn-only)
//   - parse error / no date line / file missing : print ERROR to stderr,
//     exit 0 (warn-only — never blocks the build)
//
// With --strict (local use only — CI never passes --strict):
//   - parse error / stale date : exit 2
//
// Per project rule "benchmarks local-only", this check is hygiene, not a
// gate. It is wired into .github/workflows/go-test.yml as a step with
// continue-on-error: true.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

const stalenessThresholdDays = 180

// verifiedAtRe matches the line:
//
//	**Verified at:** YYYY-MM-DD ...
//
// Anchored at the start of a line (multi-line mode); allows trailing text
// (parenthetical phase notes) after the date.
var verifiedAtRe = regexp.MustCompile(`(?m)^\*\*Verified at:\*\*\s+(\d{4}-\d{2}-\d{2})`)

func main() {
	args := os.Args[1:]
	os.Exit(run(os.Stderr, args))
}

// run is the testable entry point. Writes diagnostics to errOut; returns the
// exit code.
func run(errOut io.Writer, args []string) int {
	strict := false
	path := "eval/EVAL.md"
	for _, a := range args {
		switch {
		case a == "--strict":
			strict = true
		case a == "-h" || a == "--help":
			fmt.Fprintln(errOut, "usage: eval-attestation-check [--strict] [path-to-EVAL.md]")
			return 0
		default:
			path = a
		}
	}

	exitOnError := func(msg string) int {
		fmt.Fprintf(errOut, "ERROR: %s\n", msg)
		if strict {
			return 2
		}
		return 0
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return exitOnError(fmt.Sprintf("cannot read %s: %v", path, err))
	}

	m := verifiedAtRe.FindStringSubmatch(string(data))
	if m == nil {
		return exitOnError(fmt.Sprintf("no '**Verified at:**' line found in %s", path))
	}

	verified, err := time.Parse("2006-01-02", m[1])
	if err != nil {
		return exitOnError(fmt.Sprintf("cannot parse date %q in %s: %v", m[1], path, err))
	}

	ageDays := int(time.Since(verified).Hours() / 24)
	if ageDays > stalenessThresholdDays {
		fmt.Fprintf(errOut, "WARNING: %s attestation is %d days old (>%d); refresh per docs/EVAL-cadence\n",
			path, ageDays, stalenessThresholdDays)
		if strict {
			return 2
		}
	}
	return 0
}
