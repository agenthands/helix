package main

import (
	"fmt"
	"os"

	"github.com/agenthands/helix/internal/cli"
	"github.com/agenthands/helix/internal/mcp"
)

// version is set via -ldflags="-X main.version=$VERSION" by goreleaser; see .goreleaser.yaml.
var version = "dev"

func main() {
	cli.SetVersion(version)
	mcp.SetVersion(version) // thread same value as MCP Implementation.Version (RESEARCH.md A6)
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		// Phase 92-02 (OUT-05): exit with the per-kind code instead of a blanket
		// os.Exit(1). The error string already carries the typed `<kind>: msg`
		// (runVerb returns the daemon's typed message verbatim), so writing it to
		// stderr surfaces the stable prefix the agent branches on, and
		// cli.ExitCodeForError maps the parsed kind to its frozen exit code
		// (falling back to 1 when no kind parses — e.g. arg-validation errors).
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCodeForError(err))
	}
}
